package masonry

import (
	"github.com/wagoodman/go-partybus"
)

const (
	EventTypeDaggerOutput = partybus.EventType("dagger.output")
	EventTypeDaggerError  = partybus.EventType("dagger.error")
	EventTypeRenderPlan   = partybus.EventType("plan.render")
	EventTypeApplyPlan    = partybus.EventType("plan.apply")
)
