package masonry

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/anchore/go-logger"
	"github.com/coding-hui/common/labels"
	"github.com/gookit/color"
	"github.com/pborman/indent"
	"github.com/rs/xid"
	"github.com/vbehar/mason/pkg/dagger"
	"github.com/wagoodman/go-partybus"
)

type Blueprint struct {
	Bricks []Brick

	workspace Workspace
}

func (b Blueprint) logger() logger.Logger {
	return b.workspace.logger()
}

func (b Blueprint) Filter(selector labels.Selector) Blueprint {
	b.logger().WithFields("selector", selector.String()).Debug("Filtering blueprint")
	var filteredBricks []Brick
	for _, brick := range b.Bricks {
		brickLabels := labels.Set(maps.Clone(brick.Metadata.Labels))
		if brickLabels == nil {
			brickLabels = make(labels.Set)
		}
		brickLabels["module"] = string(brick.ModuleRef)
		brickLabels["kind"] = brick.Kind
		brickLabels["name"] = brick.Metadata.Name
		if selector.Matches(brickLabels) {
			b.logger().WithFields("name", brick.Metadata.Name, "kind", brick.Kind).
				Trace("Brick matches selector")
			filteredBricks = append(filteredBricks, brick)
		}
	}
	if len(filteredBricks) != len(b.Bricks) {
		b.logger().WithFields("kept", len(filteredBricks), "discarded", len(b.Bricks)-len(filteredBricks)).
			Info("Filtered blueprint")
	}
	return Blueprint{
		Bricks:    filteredBricks,
		workspace: b.workspace,
	}
}

func (b Blueprint) RenderPlan() (*Plan, error) {
	planName := xid.New().String()
	b.logger().WithFields("path", filepath.Join(b.workspace.WorkDir(), planName)).
		Debug("Preparing plan")

	b.logger().WithFields("path", filepath.Join(b.workspace.WorkDir(), planName, BlueprintDirPrefix)).
		Trace("Writing blueprint bricks to disk")
	modulesDirByRef, err := b.dumpBricksToDiskByModule(planName)
	if err != nil {
		return nil, fmt.Errorf("failed to dump blueprint to disk: %w", err)
	}

	planDir := filepath.Join(b.workspace.WorkDir(), planName, PlanDirPrefix)
	err = os.MkdirAll(planDir, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", planDir, err)
	}

	daggerScript := "#!/usr/bin/env dagger\n\n"
	daggerScript += "directory |\n"
	for moduleRef, moduleDir := range modulesDirByRef {
		relativeModuleDir, err := filepath.Rel(b.workspace.Dir(), moduleDir)
		if err != nil {
			return nil, fmt.Errorf("failed to get relative path of %q: %w", moduleDir, err)
		}
		moduleName := moduleRef.SanitizedName()
		daggerScript += fmt.Sprintf("with-directory %[1]s $(%[2]s | render-plan %[3]s) |\n",
			moduleName, moduleRef, relativeModuleDir)
	}
	daggerScript += "export " + planDir + "\n"

	daggerScriptFilePath := filepath.Join(planDir, "render-plan.dagger")
	b.logger().WithFields("path", daggerScriptFilePath).
		Tracef("Writing Dagger script to disk:\n%+v",
			color.Note.Sprint(indent.String("  ", daggerScript)),
		)
	err = os.WriteFile(daggerScriptFilePath, []byte(daggerScript), 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write file %q: %w", daggerScriptFilePath, err)
	}

	b.logger().WithFields("script", daggerScriptFilePath).Info("Rendering plan with Dagger")
	var output bytes.Buffer
	err = dagger.ExecScript(dagger.ExecScriptOpts{
		ScriptPath: daggerScriptFilePath,
		Env:        b.workspace.mason.DaggerEnv,
		Args:       b.workspace.mason.DaggerArgs,
		Stderr:     b.workspace.mason.DaggerOut,
		Stdout:     &output,
	})
	if err != nil {
		return nil, err
	}

	b.logger().Infof("Dagger output:\n%+v\n",
		color.Success.Sprint(indent.String("  ", output.String())),
	)
	b.workspace.mason.EventBus.Publish(partybus.Event{
		Type:   EventTypeDaggerOutput,
		Source: b,
		Value:  output.String(),
	})

	b.logger().WithFields("path", planDir).Debug("Parsing generated plan")
	plan, err := ParsePlanFromDir(planDir)
	if err != nil {
		return nil, fmt.Errorf("failed to parse plan from directory %s: %w", planDir, err)
	}
	plan.blueprint = b

	return plan, nil
}

func (b Blueprint) dumpBricksToDiskByModule(planName string) (modulesDirByRef map[ModuleRef]string, err error) {
	modulesDirByRef = make(map[ModuleRef]string)
	for moduleRef, blueprint := range b.splitByModuleRef() {
		moduleName := moduleRef.SanitizedName()
		moduleDir := filepath.Join(b.workspace.WorkDir(), planName, BlueprintDirPrefix, moduleName)
		modulesDirByRef[moduleRef] = moduleDir

		err = os.MkdirAll(moduleDir, os.ModePerm)
		if err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", moduleDir, err)
		}

		for _, brick := range blueprint.Bricks {
			brickFileName := strings.ToLower(brick.Kind) + "_" + strings.ToLower(brick.Metadata.Name) + ".json"
			brickFilePath := filepath.Join(moduleDir, brickFileName)
			brickFile, err := os.Create(brickFilePath)
			if err != nil {
				return nil, fmt.Errorf("failed to create file %s: %w", brickFilePath, err)
			}
			defer func() {
				if CloseErr := brickFile.Close(); CloseErr != nil {
					err = errors.Join(err, fmt.Errorf("failed to close file %s: %w", brickFilePath, err))
				}
			}()
			encoder := json.NewEncoder(brickFile)
			encoder.SetIndent("", "  ")
			err = encoder.Encode(brick)
			if err != nil {
				return nil, fmt.Errorf("failed to encode brick %s: %w", brickFilePath, err)
			}
		}
	}
	return modulesDirByRef, nil
}

func (b Blueprint) splitByModuleRef() map[ModuleRef]Blueprint {
	blueprintByModule := make(map[ModuleRef]Blueprint)
	for _, brick := range b.Bricks {
		if !brick.IsValid() {
			continue
		}
		moduleRef := brick.ModuleRef
		if _, ok := blueprintByModule[moduleRef]; !ok {
			blueprintByModule[moduleRef] = Blueprint{
				workspace: b.workspace,
			}
		}
		blueprint := blueprintByModule[moduleRef]
		blueprint.Bricks = append(blueprint.Bricks, brick)
		blueprintByModule[moduleRef] = blueprint
	}
	return blueprintByModule
}
