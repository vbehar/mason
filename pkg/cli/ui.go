package cli

import (
	"fmt"
	"io"

	"github.com/charmbracelet/lipgloss"
	"github.com/vbehar/mason/pkg/masonry"
	"github.com/wagoodman/go-partybus"
)

type UI struct {
	Output io.Writer
}

func (ui *UI) Setup(subscription partybus.Unsubscribable) error {
	return nil
}

func (ui *UI) Handle(event partybus.Event) error {
	switch event.Type {
	case masonry.EventTypeRenderPlan:
		phase := event.Source.(map[string]string)["phase"]
		ui.print(phaseStyle.Render(phase))
		ui.println(descriptionStyle.Render("Rendering plan..."))
	case masonry.EventTypeApplyPlan:
		phase := event.Source.(map[string]string)["phase"]
		postRun := event.Source.(map[string]string)["postRun"]
		ui.print(phaseStyle.Render(phase))
		switch masonry.PostRun(postRun) {
		case masonry.PostRunOnSuccess:
			ui.println(postRunOnSuccessStyle.Render("Post run on success..."))
		case masonry.PostRunOnFailure:
			ui.println(postRunOnFailureStyle.Render("Post run on error..."))
		default:
			ui.println(descriptionStyle.Render("Applying plan..."))
		}
	case masonry.EventTypeDaggerOutput:
		phase := event.Source.(map[string]string)["phase"]
		postRun := event.Source.(map[string]string)["postRun"]
		ui.print(phaseStyle.Render(phase))
		switch masonry.PostRun(postRun) {
		case masonry.PostRunOnSuccess:
			ui.println(postRunOnSuccessStyle.Render("Post run on success Dagger output:"))
		case masonry.PostRunOnFailure:
			ui.println(postRunOnFailureStyle.Render("Post run on error Dagger output:"))
		default:
			ui.println(descriptionStyle.Render("Dagger output:"))
		}
		ui.println(event.Value.(string))
	}
	return nil
}

func (ui *UI) Teardown(force bool) error {
	return nil
}

func (ui *UI) print(a ...any) {
	fmt.Fprint(ui.Output, a...) //nolint:errcheck // don't care
}

func (ui *UI) println(a ...any) {
	fmt.Fprintln(ui.Output, a...) //nolint:errcheck // don't care
}

var (
	phaseStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), false, true).
			BorderForeground(lipgloss.Color("#874BFD")).
			Foreground(lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}).
			Margin(0, 1, 0, 0)
	postRunOnSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#43BF6D")).
				Background(lipgloss.Color("#1E1E2D")).
				Border(lipgloss.RoundedBorder(), false, true).
				BorderForeground(lipgloss.Color("#874BFD")).
				Margin(0, 1, 0, 0)
	postRunOnFailureStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F00")).
				Background(lipgloss.Color("#1E1E2D")).
				Border(lipgloss.RoundedBorder(), false, true).
				BorderForeground(lipgloss.Color("#874BFD")).
				Margin(0, 1, 0, 0)
	descriptionStyle = lipgloss.NewStyle()
)
