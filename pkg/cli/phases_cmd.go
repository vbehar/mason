package cli

import (
	"fmt"

	"github.com/charmbracelet/lipgloss/tree"
	"github.com/coding-hui/common/labels"
	"github.com/spf13/cobra"
)

func phasesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "phases",
		Short: "List all the custom phases defined in the configuration",
		Args:  cobra.NoArgs,
		RunE:  printPhases,
	}
}

func printPhases(_ *cobra.Command, _ []string) error {
	root := tree.Root("Phases:")
	for alias, cfgs := range masonConfig.Aliases {
		child := tree.Root(alias)
		for _, cfg := range cfgs {
			grandChild := tree.Root(cfg.Phase)
			if cfg.BrickLabelSelector != "" {
				reqs, err := labels.ParseToRequirements(cfg.BrickLabelSelector)
				if err != nil {
					return fmt.Errorf("failed to parse label selector %s: %w", cfg.BrickLabelSelector, err)
				}
				for _, req := range reqs {
					grandChild.Child(req.String())
				}
			}
			child.Child(grandChild)
		}
		root.Child(child)
	}

	fmt.Println(root.Enumerator(tree.RoundedEnumerator))
	return nil
}
