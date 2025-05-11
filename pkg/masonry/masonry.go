package masonry

const (
	MasonDirName       = ".mason"
	WorkDirPrefix      = ".work"
	BlueprintDirPrefix = "blueprint"
	PlanDirPrefix      = "plan"
)

var Phases = map[string]string{
	"test":    "Run tests",
	"lint":    "Run linters",
	"package": "Package artifacts",
	"publish": "Publish artifacts",
	"run":     "Run the application",
}
