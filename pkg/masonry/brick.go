package masonry

type Brick struct {
	Kind      string        `json:"kind"`
	ModuleRef ModuleRef     `json:"moduleRef"`
	Metadata  BrickMetadata `json:"metadata"`
	Spec      any           `json:"spec"`
}

func (b Brick) IsValid() bool {
	return b.Kind != "" && b.ModuleRef != "" && b.Metadata.Name != ""
}

type BrickMetadata struct {
	Name        string            `json:"name"`
	Labels      map[string]string `json:"labels"`
	ExtraPhases []string          `json:"extraPhases"`
	PostRun     PostRun           `json:"postRun"`
}

type PostRun string

const (
	PostRunAlways    PostRun = "always"
	PostRunOnSuccess PostRun = "on_success"
	PostRunOnFailure PostRun = "on_failure"
	PostRunNever     PostRun = ""
)
