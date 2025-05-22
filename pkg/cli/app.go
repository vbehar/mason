package cli

import (
	"io"
	"os"

	"github.com/anchore/clio"
	"github.com/vbehar/mason/pkg/dagger"
	"github.com/vbehar/mason/pkg/masonry"
)

var mason = masonry.NewMason()

func Application(id clio.Identification) clio.Application {
	app := clio.New(*clioSetupConfig(id))

	rootCmd := app.SetupRootCommand(rootCommand(id), masonConfig)
	rootCmd.AddCommand(
		app.SetupCommand(phasesCommand(), masonConfig),
		clio.VersionCommand(id, daggerVersion),
		clio.ConfigCommand(app, &clio.ConfigCommandConfig{
			IncludeLocationsSubcommand: true,
			LoadConfig:                 true,
			ReplaceHomeDirWithTilde:    true,
		}),
	)

	return app
}

func clioSetupConfig(id clio.Identification) *clio.SetupConfig {
	return clio.NewSetupConfig(id).
		WithGlobalConfigFlag().
		WithGlobalLoggingFlags().
		WithConfigInRootHelp().
		WithUIConstructor(
			func(cfg clio.Config) (*clio.UICollection, error) {
				var output io.Writer
				if cfg.Log.Verbosity > 0 {
					// in case of verbose output, we'll use the logs instead of the UI
					output = io.Discard
				} else {
					output = os.Stdout
				}
				return clio.NewUICollection(&UI{
					Output: output,
				}), nil
			},
		).
		WithInitializers(func(state *clio.State) error {
			// at this point, the state is ready, but out masonConfig is not yet loaded
			masonConfig.state = state
			mason.EventBus = state.Bus
			mason.Logger = state.Logger
			if state.Config.Log.Quiet {
				mason.DaggerOutputDisabled = true
			}
			return nil
		}).
		WithPostRuns(func(state *clio.State, err error) {
			if err == nil && !masonConfig.KeepWorkDir {
				cleanErr := mason.CleanWorkDirs()
				if cleanErr != nil {
					state.Logger.Warn(cleanErr)
				}
			}
			if err != nil {
				state.Logger.
					WithFields("workdirs", mason.WorkDirs()).
					Infof("Keeping workdirs")
			}
		})
}

func daggerVersion() (string, any) {
	v, err := dagger.Version()
	if err != nil {
		return "Dagger", err
	}
	return "Dagger", v
}
