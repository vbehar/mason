package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/anchore/clio"
	"github.com/coding-hui/common/labels"
)

var masonConfig = &MasonConfig{
	RootPath:    ".",
	IgnoredDirs: []string{".git"},
	KeepWorkDir: false,
	Dagger: DaggerConfig{
		Binary: "dagger",
		Env: []string{
			"DAGGER_ALLOW_LLM=all",
		},
	},
}

var _ interface {
	clio.FlagAdder
	clio.PostLoader
	clio.FieldDescriber
} = (*MasonConfig)(nil)

type MasonConfig struct {
	RootPath    string   `mapstructure:"root-path"`
	IgnoredDirs []string `mapstructure:"ignored-dirs"`
	KeepWorkDir bool     `mapstructure:"keep-work-dir"`

	BrickLabelSelector string `mapstructure:"label-selector"`
	labelSelector      labels.Selector

	Aliases map[string][]AliasConfig `mapstructure:"aliases"`

	Dagger DaggerConfig `mapstructure:"dagger"`

	state *clio.State `mapstructure:"-"`
}

func (c *MasonConfig) AddFlags(flags clio.FlagSet) {
	flags.StringVarP(&c.RootPath, "root-path", "", "Root path of the workspace")
	flags.StringArrayVarP(&c.IgnoredDirs, "ignored-dirs", "", "Directories to ignore")
	flags.BoolVarP(&c.KeepWorkDir, "keep-work-dir", "", "Keep the work directory after execution")
	flags.StringVarP(&c.BrickLabelSelector, "selector", "l", "Label selector for bricks, similar to Kubernetes Label selector syntax. "+
		"Note that the brick kind and name can be used as labels.")
}

func (c *MasonConfig) DescribeFields(d clio.FieldDescriptionSet) {
	d.Add(&c.Aliases, "Aliases for phases. Each alias is a list of labels that will be used to select bricks for the phase.")
}

func (c *MasonConfig) PostLoad() error {
	var err error
	c.labelSelector, err = labels.Parse(c.BrickLabelSelector)
	if err != nil {
		return fmt.Errorf("failed to parse label selector %q: %w", c.BrickLabelSelector, err)
	}
	for alias, cfgs := range c.Aliases {
		for i := range cfgs {
			cfg := &cfgs[i]
			if err = cfg.PostLoad(); err != nil {
				return fmt.Errorf("failed to post load cfg %d (%s) for alias %q: %w", i, cfg.Phase, alias, err)
			}
		}
	}

	// configure Dagger in quiet mode (Reduce verbosity - show progress, but clean up at the end)
	// but only if the user didn't specify a quiet flag, and if we're using our default UI
	hasQuietFlag := slices.ContainsFunc(c.Dagger.Args, func(arg string) bool {
		if strings.HasPrefix(arg, "-q") ||
			strings.HasPrefix(arg, "--quiet") {
			return true
		}
		return false
	})
	if !hasQuietFlag && c.state != nil && c.state.Config.Log.Verbosity == 0 {
		c.Dagger.Args = append(c.Dagger.Args, "-q=1")
	}

	// now that our config is loaded, we can use it
	mason.RootPath = c.RootPath
	mason.IgnoredDirs = c.IgnoredDirs
	mason.DaggerArgs = c.Dagger.Args
	mason.DaggerEnv = c.Dagger.Env
	mason.DaggerBinary = c.Dagger.Binary
	return nil
}

var _ interface {
	clio.FlagAdder
} = (*DaggerConfig)(nil)

type DaggerConfig struct {
	Binary string   `mapstructure:"binary"`
	Env    []string `mapstructure:"env"`
	Args   []string `mapstructure:"args"`
}

func (c *DaggerConfig) AddFlags(flags clio.FlagSet) {
	flags.StringVarP(&c.Binary, "dagger-binary", "", "Path to the dagger binary")
	flags.StringArrayVarP(&c.Env, "dagger-env", "", "Environment variables to pass to the dagger command")
	flags.StringArrayVarP(&c.Args, "dagger-args", "", "Arguments (flags) to pass to the dagger command")
}

var _ interface {
	clio.FieldDescriber
	clio.PostLoader
} = (*MasonConfig)(nil)

type AliasConfig struct {
	Phase              string `mapstructure:"phase"`
	BrickLabelSelector string `mapstructure:"selector"`
	labelSelector      labels.Selector
}

func (c *AliasConfig) DescribeFields(d clio.FieldDescriptionSet) {
	d.Add(&c.Phase, "Phase name")
	d.Add(&c.BrickLabelSelector, "Label selector for bricks, similar to Kubernetes Label selector syntax. "+
		"Note that the brick kind and name can be used as labels.")
}

func (c *AliasConfig) PostLoad() error {
	var err error
	c.labelSelector, err = labels.Parse(c.BrickLabelSelector)
	if err != nil {
		return fmt.Errorf("failed to parse label selector %q: %w", c.BrickLabelSelector, err)
	}
	return nil
}
