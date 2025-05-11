package cli

import (
	"fmt"

	"github.com/anchore/clio"
	"github.com/anchore/fangs"
	"github.com/spf13/cobra"
	"github.com/vbehar/mason/pkg/masonry"
	"github.com/wagoodman/go-partybus"
)

func rootCommand(appID clio.Identification) *cobra.Command {
	return &cobra.Command{
		Use:   appID.Name + " [phases]",
		Short: "Mason is a declarative build tool leveraging Dagger.",
		Long: `Mason is a declarative build tool that simplifies complex build workflows by providing
a consistent interface to Dagger pipelines organized into phases.

Mason uses a workspace model where build configurations are stored in a .mason directory.
Each workspace contains a blueprint made up of "bricks" - reusable components that define
parts of your build process. These bricks are executed in phases, allowing for a modular
and extensible build process.

The execution happens in 2 steps:
1. Rendering a plan.
   Mason calls a set of Dagger modules to generate a plan in the form of Dagger scripts.
2. Applying the plan.
   Mason executes the generated Dagger scripts to perform the actual build.

Common phases include:
  test     Run tests
  lint     Run linters
  package  Package artifacts
  publish  Publish artifacts
  run      Run the application`,
		Example: `  # Run a single phase
  mason package

  # Run multiple phases in sequence
  mason test lint package

  # Use a predefined alias (set of phases with optional filters)
  mason ci

  # Run a phase with a specific label selector, to filter the bricks
  mason package -l os=darwin`,
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: phasesValidArgsFunction,
		RunE:              run,
	}
}

func run(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}

	workspaces, err := mason.DetectWorkspaces()
	if err != nil {
		return err
	}
	if len(workspaces) == 0 {
		return fmt.Errorf("no .mason directory found")
	}
	if len(workspaces) > 1 {
		return fmt.Errorf("found %d workspaces: %v. Multi-workspace support is not implemented yet. mason ignored-dirs: %v", len(workspaces), workspaces, mason.IgnoredDirs)
	}

	workspace := workspaces[0]
	if workspace.RelativePath != "." {
		return fmt.Errorf("the .mason directory must be in the current working directory")
	}

	blueprint, err := workspace.LoadBlueprint()
	if err != nil {
		return err
	}

	for _, phaseCfg := range parsePhasesAndSelectors(args) {
		filteredBlueprint := blueprint.Filter(phaseCfg.labelSelector)

		mason.EventBus.Publish(partybus.Event{
			Type:   EventTypeRenderPlan,
			Source: map[string]string{"phase": phaseCfg.Phase},
		})
		plan, err := filteredBlueprint.RenderPlan()
		if err != nil {
			return err
		}

		plan, err = plan.FilterForPhase(phaseCfg.Phase)
		if err != nil {
			return err
		}

		if plan.IsEmpty() {
			mason.Logger.WithFields("phase", phaseCfg.Phase).Warn("No scripts found, skipping phase")
			continue
		}

		mason.EventBus.Publish(partybus.Event{
			Type:   EventTypeApplyPlan,
			Source: map[string]string{"phase": phaseCfg.Phase},
		})
		err = plan.Run()
		if err != nil {
			return err
		}
	}
	return nil
}

func parsePhasesAndSelectors(phasesOrAliases []string) []AliasConfig {
	var allPhases []AliasConfig
	for _, phaseOrAlias := range phasesOrAliases {
		if aliasCfgs, ok := masonConfig.Aliases[phaseOrAlias]; ok {
			for _, cfg := range aliasCfgs {
				globalRequirements, _ := masonConfig.labelSelector.Requirements()
				selector := cfg.labelSelector.DeepCopySelector().Add(globalRequirements...)
				allPhases = append(allPhases, AliasConfig{
					Phase:              cfg.Phase,
					BrickLabelSelector: selector.String(),
					labelSelector:      selector,
				})
			}
		} else {
			allPhases = append(allPhases, AliasConfig{
				Phase:              phaseOrAlias,
				BrickLabelSelector: masonConfig.BrickLabelSelector,
				labelSelector:      masonConfig.labelSelector,
			})
		}
	}
	return allPhases
}

func phasesValidArgsFunction(cmd *cobra.Command, args []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
	// hack to load the config, to get the aliases...
	_ = fangs.Load(clioSetupConfig(clio.Identification{
		Name: "mason",
	}).FangsConfig, cmd, masonConfig)

	alreadyUsedPhases := make(map[string]struct{})
	for _, arg := range args {
		alreadyUsedPhases[arg] = struct{}{}
	}

	var completions []cobra.Completion

	for phase, description := range masonry.Phases {
		if _, ok := alreadyUsedPhases[phase]; ok {
			continue
		}
		completions = append(completions, cobra.CompletionWithDesc(phase, description))
	}

	for alias, configs := range masonConfig.Aliases {
		if _, ok := alreadyUsedPhases[alias]; ok {
			continue
		}
		var phases []string
		for _, cfg := range configs {
			phases = append(phases, cfg.Phase)
		}
		completions = append(completions, cobra.CompletionWithDesc(alias, fmt.Sprintf("Alias for %s", phases)))
	}

	return completions, cobra.ShellCompDirectiveDefault | cobra.ShellCompDirectiveNoFileComp
}
