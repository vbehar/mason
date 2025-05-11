package cli

import (
	"fmt"
	"io"

	"github.com/charmbracelet/lipgloss"
	"github.com/vbehar/mason/pkg/masonry"
	"github.com/wagoodman/go-partybus"
)

const (
	EventTypeRenderPlan = partybus.EventType("plan.render")
	EventTypeApplyPlan  = partybus.EventType("plan.apply")
)

type UI struct {
	Output io.Writer
}

func (ui *UI) Setup(subscription partybus.Unsubscribable) error {
	return nil
}

func (ui *UI) Handle(event partybus.Event) error {
	switch event.Type {
	case EventTypeRenderPlan:
		phase := event.Source.(map[string]string)["phase"]
		ui.print(phaseStyle.Render(phase))
		ui.println(descriptionStyle.Render("Rendering plan..."))
	case EventTypeApplyPlan:
		phase := event.Source.(map[string]string)["phase"]
		ui.print(phaseStyle.Render(phase))
		ui.println(descriptionStyle.Render("Applying plan..."))
	case masonry.EventTypeDaggerOutput:
		switch src := event.Source.(type) {
		case masonry.Blueprint:
			// "render plan" output
		case masonry.Plan:
			// "apply plan" output
			ui.print(phaseStyle.Render(src.Phase))
			ui.println(descriptionStyle.Render("Dagger output:"))
			ui.println(event.Value.(string))
		}
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
	descriptionStyle = lipgloss.NewStyle()
)
