package masonry

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anchore/go-logger"
	"github.com/gookit/color"
	"github.com/heimdalr/dag"
	"github.com/pborman/indent"
	"github.com/vbehar/mason/pkg/dagger"
	"github.com/wagoodman/go-partybus"
)

type Plan struct {
	DirPath       string
	SourceScripts []Script
	Phase         string
	MergedScript  string

	blueprint Blueprint
}

type Script struct {
	ModuleName string
	Phase      string
	Name       string
	Content    dagger.Script
}

func (s Script) Equals(other Script) bool {
	return s.ModuleName == other.ModuleName &&
		s.Phase == other.Phase &&
		s.Name == other.Name &&
		s.Content == other.Content
}

func ParsePlanFromDir(dirPath string) (*Plan, error) {
	plan := Plan{
		DirPath: dirPath,
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		subDirPath := filepath.Join(dirPath, entry.Name())
		files, err := os.ReadDir(subDirPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read directory %s: %w", subDirPath, err)
		}
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			if filepath.Ext(file.Name()) != ".dagger" {
				continue
			}

			filePath := filepath.Join(dirPath, entry.Name(), file.Name())
			fileData, err := os.ReadFile(filePath)
			if err != nil {
				return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
			}

			fileName := strings.TrimSuffix(file.Name(), ".dagger")
			phase, name, _ := strings.Cut(fileName, "_")

			plan.SourceScripts = append(plan.SourceScripts, Script{
				ModuleName: entry.Name(),
				Phase:      phase,
				Name:       name,
				Content:    dagger.Script(fileData),
			})
		}
	}

	err = plan.computeMergedScript()
	if err != nil {
		return nil, fmt.Errorf("failed to compute merged script: %w", err)
	}

	return &plan, nil
}

func (p Plan) logger() logger.Logger {
	relPlanDir, _ := filepath.Rel(p.blueprint.workspace.WorkDir(), p.DirPath)
	if relPlanDir == "" {
		relPlanDir = p.DirPath
	}
	return p.blueprint.logger().Nested("plan", relPlanDir)
}

func (p Plan) IsEmpty() bool {
	return len(p.SourceScripts) == 0 || p.MergedScript == ""
}

func (p Plan) FilterForPhase(phase string) (*Plan, error) {
	p.logger().WithFields("phase", phase).Debug("Filtering plan")

	filteredPlan := &Plan{
		DirPath:   p.DirPath,
		blueprint: p.blueprint,
		Phase:     phase,
	}

	for _, script := range p.SourceScripts {
		if script.Phase == phase {
			filteredPlan.SourceScripts = append(filteredPlan.SourceScripts, script)
		}
	}
	if len(filteredPlan.SourceScripts) != len(p.SourceScripts) {
		p.logger().WithFields(
			"phase", phase,
			"kept", len(filteredPlan.SourceScripts),
			"discarded", len(p.SourceScripts)-len(filteredPlan.SourceScripts),
		).Debug("Filtered plan")
	}

	err := filteredPlan.computeMergedScript()
	if err != nil {
		return nil, fmt.Errorf("failed to compute merged script for phase %s: %w", phase, err)
	}
	return filteredPlan, nil
}

func (p *Plan) computeMergedScript() error {
	if len(p.SourceScripts) == 0 {
		return nil
	}

	var (
		variablesDAG         = dag.NewDAG()
		variablesDefinitions = make(map[string]Script)
		variablesUsages      = make(map[string][]Script)
	)
	for _, script := range p.SourceScripts {
		err := variablesDAG.AddVertexByID(string(script.Content), script)
		if err != nil {
			if errors.As(err, &dag.IDDuplicateError{}) {
				continue // can happen if the same script is used in multiple phases...
			}
			return fmt.Errorf("failed to add script from %q to DAG: %w", script.Name, err)
		}

		for varName := range script.Content.ExtractDefinedVariables() {
			if existingScript, ok := variablesDefinitions[varName]; ok {
				return fmt.Errorf("variable %q is defined twice: by %q and %q", varName, existingScript.Name, script.Name)
			}
			variablesDefinitions[varName] = script
		}

		for varName := range script.Content.ExtractUsedVariables() {
			variablesUsages[varName] = append(variablesUsages[varName], script)
		}
	}

	for varName, scripts := range variablesUsages {
		varDefinitionScript, ok := variablesDefinitions[varName]
		if !ok {
			var scriptsNames []string
			for _, script := range scripts {
				scriptsNames = append(scriptsNames, script.Name)
			}
			return fmt.Errorf("variable %q is used but not defined. Used by %v", varName, scriptsNames)
		}

		for _, script := range scripts {
			if script.Equals(varDefinitionScript) {
				continue
			}
			err := variablesDAG.AddEdge(string(varDefinitionScript.Content), string(script.Content))
			if err != nil {
				if errors.As(err, &dag.EdgeDuplicateError{}) {
					continue
				}
				return fmt.Errorf("failed to add edge for variable %q from %q to %q: %w", varName, varDefinitionScript.Name, script.Name, err)
			}
		}
	}

	var err error
	p.MergedScript = "#!/usr/bin/env dagger\n\n"
	if p.Phase != "" {
		p.MergedScript += fmt.Sprintf("# Phase: %s\n\n", p.Phase)
	}
	variablesDAG.DFSWalk(dagVisitorFunc(func(v dag.Vertexer) {
		id, val := v.Vertex()
		script, ok := val.(Script)
		if !ok {
			err = errors.Join(err, fmt.Errorf("failed to cast vertex %q to Script", id))
			return
		}
		p.MergedScript += fmt.Sprintf("# %s\n", script.Name)
		p.MergedScript += strings.TrimSpace(string(script.Content)) + "\n"
		p.MergedScript += ".echo\n\n" // we echo an empty line to separate scripts output
	}))
	p.MergedScript = strings.TrimSpace(p.MergedScript)

	return err
}

func (p Plan) Run() error {
	planFileName := fmt.Sprintf("plan_%s.dagger", p.Phase)
	planFilePath := filepath.Join(p.DirPath, planFileName)
	p.logger().WithFields("path", planFilePath).
		Tracef("Writing Dagger script to disk:\n%+v\n",
			color.Note.Sprint(indent.String("  ", p.MergedScript)),
		)
	err := os.WriteFile(planFilePath, []byte(p.MergedScript), 0644)
	if err != nil {
		return fmt.Errorf("failed to write plan file %q: %w", planFilePath, err)
	}

	p.logger().WithFields("script", planFilePath).Info("Applying plan with Dagger")
	var daggerOutWriter bytes.Buffer
	err = dagger.ExecScript(dagger.ExecScriptOpts{
		ScriptPath: planFilePath,
		Env:        p.blueprint.workspace.mason.DaggerEnv,
		Args:       p.blueprint.workspace.mason.DaggerArgs,
		Stderr:     p.blueprint.workspace.mason.DaggerOut,
		Stdout:     &daggerOutWriter,
	})
	if err != nil {
		return fmt.Errorf("failed to run plan: %w", err)
	}

	output := strings.TrimSpace(daggerOutWriter.String())

	p.logger().Infof("Dagger output:\n%+v\n",
		color.Success.Sprint(indent.String("  ", output)),
	)
	p.blueprint.workspace.mason.EventBus.Publish(partybus.Event{
		Type:   EventTypeDaggerOutput,
		Source: p,
		Value:  output,
	})

	return nil
}

// dagVisitorFunc implements the Visitor interface for a function.
type dagVisitorFunc func(dag.Vertexer)

func (f dagVisitorFunc) Visit(v dag.Vertexer) {
	f(v)
}
