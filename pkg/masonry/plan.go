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
	DirPath                string
	SourceScripts          []Script
	Phase                  string
	MergedScript           string
	PostRunOnSuccessScript string
	PostRunOnFailureScript string

	blueprint Blueprint
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
			script, err := ScriptFromFile(filePath)
			if err != nil {
				return nil, fmt.Errorf("failed to parse script from file %s: %w", filePath, err)
			}

			plan.SourceScripts = append(plan.SourceScripts, *script)
		}
	}

	err = plan.computeFinalScripts()
	if err != nil {
		return nil, fmt.Errorf("failed to compute final scripts: %w", err)
	}

	return &plan, nil
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
		if script.Phase == phase || script.Phase == "" {
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

	err := filteredPlan.computeFinalScripts()
	if err != nil {
		return nil, fmt.Errorf("failed to compute final scripts for phase %s: %w", phase, err)
	}
	return filteredPlan, nil
}

func (p *Plan) computeFinalScripts() error {
	var mainScripts []Script
	for _, script := range p.SourceScripts {
		if script.PostRun == "" {
			mainScripts = append(mainScripts, script)
		}
	}
	p.MergedScript = ""
	if len(mainScripts) > 0 {
		mainScript, err := mergeScripts(mainScripts)
		if err != nil {
			return fmt.Errorf("failed to merge main scripts: %w", err)
		}
		p.MergedScript = "#!/usr/bin/env dagger\n\n"
		if p.Phase != "" {
			p.MergedScript += fmt.Sprintf("# Phase: %s\n\n", p.Phase)
		}
		p.MergedScript += mainScript
	}

	var postRunOnSuccessScripts []Script
	for _, script := range p.SourceScripts {
		switch script.PostRun {
		case PostRunOnSuccess, PostRunAlways:
			postRunOnSuccessScripts = append(postRunOnSuccessScripts, script)
		}
	}
	p.PostRunOnSuccessScript = ""
	if len(postRunOnSuccessScripts) > 0 {
		postRunOnSuccessScripts = append(postRunOnSuccessScripts, p.postRunInitScript())
		postRunOnSuccessScript, err := mergeScripts(postRunOnSuccessScripts)
		if err != nil {
			return fmt.Errorf("failed to merge post-run on-success scripts: %w", err)
		}
		p.PostRunOnSuccessScript = "#!/usr/bin/env dagger\n\n"
		p.PostRunOnSuccessScript += "# Post run on-success script"
		if p.Phase != "" {
			p.PostRunOnSuccessScript += fmt.Sprintf(" for phase %s", p.Phase)
		}
		p.PostRunOnSuccessScript += "\n\n" + postRunOnSuccessScript
	}

	var postRunOnFailureScripts []Script
	for _, script := range p.SourceScripts {
		switch script.PostRun {
		case PostRunOnFailure, PostRunAlways:
			postRunOnFailureScripts = append(postRunOnFailureScripts, script)
		}
	}
	p.PostRunOnFailureScript = ""
	if len(postRunOnFailureScripts) > 0 {
		postRunOnFailureScripts = append(postRunOnFailureScripts, p.postRunInitScript())
		postRunOnFailureScript, err := mergeScripts(postRunOnFailureScripts)
		if err != nil {
			return fmt.Errorf("failed to merge post-run on-failure scripts: %w", err)
		}
		p.PostRunOnFailureScript = "#!/usr/bin/env dagger\n\n"
		p.PostRunOnFailureScript += "# Post run on-failure script"
		if p.Phase != "" {
			p.PostRunOnFailureScript += fmt.Sprintf(" for phase %s", p.Phase)
		}
		p.PostRunOnFailureScript += "\n\n" + postRunOnFailureScript
	}

	return nil
}

func (p Plan) Run() error {
	p.blueprint.workspace.mason.EventBus.Publish(partybus.Event{
		Type:   EventTypeApplyPlan,
		Source: map[string]string{"phase": p.Phase},
	})

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

	logFile, err := os.Create(p.logFilePath())
	if err != nil {
		return fmt.Errorf("failed to create log file %q: %w", p.logFilePath(), err)
	}
	defer func() {
		err := logFile.Close()
		if err != nil {
			p.logger().WithFields("path", p.logFilePath()).
				Errorf("Failed to close log file: %s", err)
		}
	}()

	p.logger().WithFields("script", planFilePath).Info("Applying plan with Dagger")
	var daggerOutWriter bytes.Buffer
	runErr := dagger.ExecScript(dagger.ExecScriptOpts{
		BinaryPath:    p.blueprint.workspace.mason.DaggerBinary,
		Logger:        p.logger(),
		ScriptPath:    planFilePath,
		Env:           p.blueprint.workspace.mason.DaggerEnv,
		Args:          p.blueprint.workspace.mason.DaggerArgs,
		DisableOutput: p.blueprint.workspace.mason.DaggerOutputDisabled,
		Stdout:        &daggerOutWriter,
		Stderr:        logFile,
	})

	// parse/write the dagger output before handling the error
	// to make sure we don't lose it before returning
	output := strings.TrimSpace(daggerOutWriter.String())
	p.logger().Infof("Dagger output:\n%+v\n",
		color.Success.Sprint(indent.String("  ", output)),
	)
	if output != "" {
		p.blueprint.workspace.mason.EventBus.Publish(partybus.Event{
			Type:   EventTypeDaggerOutput,
			Source: map[string]string{"phase": p.Phase},
			Value:  output,
		})
	}

	postRun := PostRunOnSuccess
	if runErr != nil {
		postRun = PostRunOnFailure
	}
	postRunErr := p.runPostScript(postRun)
	if postRunErr != nil {
		if runErr != nil {
			runErr = errors.Join(runErr, postRunErr)
		} else {
			runErr = postRunErr
		}
	}

	if runErr != nil {
		return fmt.Errorf("failed to run plan: %w", runErr)
	}
	return nil
}

func (p Plan) runPostScript(postRun PostRun) error {
	var script string
	switch postRun {
	case PostRunOnSuccess:
		script = p.PostRunOnSuccessScript
	case PostRunOnFailure:
		script = p.PostRunOnFailureScript
	default:
		return fmt.Errorf("unsupported post-run type: %s", postRun)
	}

	if script == "" {
		p.logger().WithFields("post-run", postRun).Debug("No post-run script to run")
		return nil
	}

	p.blueprint.workspace.mason.EventBus.Publish(partybus.Event{
		Type:   EventTypeApplyPlan,
		Source: map[string]string{"phase": p.Phase, "postRun": string(postRun)},
	})

	planFileName := fmt.Sprintf("plan_%s_postrun_%s.dagger", p.Phase, postRun)
	planFilePath := filepath.Join(p.DirPath, planFileName)
	p.logger().WithFields("path", planFilePath).
		Tracef("Writing Dagger post-run script to disk:\n%+v\n",
			color.Note.Sprint(indent.String("  ", script)),
		)
	err := os.WriteFile(planFilePath, []byte(script), 0644)
	if err != nil {
		return fmt.Errorf("failed to write post-run plan file %q: %w", planFilePath, err)
	}

	logFileName := fmt.Sprintf("dagger_%s_postrun_%s.log", p.Phase, postRun)
	logFilePath := filepath.Join(p.DirPath, logFileName)
	logFile, err := os.Create(logFilePath)
	if err != nil {
		return fmt.Errorf("failed to create post-run log file %q: %w", logFilePath, err)
	}
	defer func() {
		err := logFile.Close()
		if err != nil {
			p.logger().WithFields("path", logFilePath).
				Errorf("Failed to close log file: %s", err)
		}
	}()

	p.logger().WithFields("script", planFilePath).Info("Applying post-run plan with Dagger")
	var daggerOutWriter bytes.Buffer
	runErr := dagger.ExecScript(dagger.ExecScriptOpts{
		BinaryPath:    p.blueprint.workspace.mason.DaggerBinary,
		Logger:        p.logger(),
		ScriptPath:    planFilePath,
		Env:           p.blueprint.workspace.mason.DaggerEnv,
		Args:          p.blueprint.workspace.mason.DaggerArgs,
		DisableOutput: p.blueprint.workspace.mason.DaggerOutputDisabled,
		Stdout:        &daggerOutWriter,
		Stderr:        logFile,
	})

	// parse/write the dagger output before handling the error
	// to make sure we don't lose it before returning
	output := strings.TrimSpace(daggerOutWriter.String())
	p.logger().Infof("Dagger post-run output:\n%+v\n",
		color.Success.Sprint(indent.String("  ", output)),
	)
	if output != "" {
		p.blueprint.workspace.mason.EventBus.Publish(partybus.Event{
			Type:   EventTypeDaggerOutput,
			Source: map[string]string{"phase": p.Phase, "postRun": string(postRun)},
			Value:  output,
		})
	}

	if runErr != nil {
		return fmt.Errorf("failed to run post-run plan: %w", runErr)
	}
	return nil
}

func (p Plan) postRunInitScript() Script {
	relativeLogFilePath, _ := filepath.Rel(p.blueprint.workspace.Dir(), p.logFilePath())
	if relativeLogFilePath == "" {
		relativeLogFilePath = p.logFilePath()
	}
	return Script{
		Name:       "post-run-init",
		PostRun:    PostRunAlways,
		Phase:      p.Phase,
		ModuleName: "mason-internal",
		Content: dagger.Script(fmt.Sprintf(`
log_file_path=$(.echo -n "%s")
		`, relativeLogFilePath)),
	}
}

func (p Plan) logger() logger.Logger {
	relDirPath, _ := filepath.Rel(p.blueprint.workspace.WorkDir(), p.DirPath)
	if relDirPath == "" {
		relDirPath = p.DirPath
	}
	return p.blueprint.logger().Nested("plan", relDirPath)
}

func (p Plan) logFilePath() string {
	logFileName := fmt.Sprintf("dagger_%s.log", p.Phase)
	return filepath.Join(p.DirPath, logFileName)
}

// dagVisitorFunc implements the Visitor interface for a function.
type dagVisitorFunc func(dag.Vertexer)

func (f dagVisitorFunc) Visit(v dag.Vertexer) {
	f(v)
}

func mergeScripts(scripts []Script) (string, error) {
	if len(scripts) == 0 {
		return "", nil
	}

	var (
		variablesDAG         = dag.NewDAG()
		variablesDefinitions = make(map[string]Script)
		variablesUsages      = make(map[string][]Script)
	)
	for _, script := range scripts {
		err := variablesDAG.AddVertexByID(string(script.Content), script)
		if err != nil {
			if errors.As(err, &dag.IDDuplicateError{}) {
				continue // can happen if the same script is used in multiple phases...
			}
			return "", fmt.Errorf("failed to add script from %q to DAG: %w", script.Name, err)
		}

		for varName := range script.Content.ExtractDefinedVariables() {
			if existingScript, ok := variablesDefinitions[varName]; ok {
				return "", fmt.Errorf("variable %q is defined twice: by %q and %q", varName, existingScript.Name, script.Name)
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
			// this can happen if the variable references an environment variable
			continue
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
				return "", fmt.Errorf("failed to add edge for variable %q from %q to %q: %w", varName, varDefinitionScript.Name, script.Name, err)
			}
		}
	}

	var (
		mergedScript string
		err          error
	)
	variablesDAG.DFSWalk(dagVisitorFunc(func(v dag.Vertexer) {
		id, val := v.Vertex()
		script, ok := val.(Script)
		if !ok {
			err = errors.Join(err, fmt.Errorf("failed to cast vertex %q to Script", id))
			return
		}
		mergedScript += fmt.Sprintf("# %s\n", script.Name)
		mergedScript += strings.TrimSpace(string(script.Content)) + "\n"
		mergedScript += ".echo\n\n" // we echo an empty line to separate scripts output
	}))
	mergedScript = strings.TrimSpace(mergedScript)

	return mergedScript, err
}
