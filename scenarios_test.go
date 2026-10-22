package sheepdogseams

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-bdd/gobdd"
)

// The version the scenarios' build of seams is stamped with.
const scenarioVersion = "0.0.0-scenario"

// TestScenarios runs the Gherkin scenarios in features/ against a real build
// of seams, driving it from outside, as a developer would.
func TestScenarios(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "seams")
	build := exec.Command("go", "build", "-o", bin,
		"-ldflags", "-X main.version="+scenarioVersion, "./cmd/seams")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building seams: %v\n%s", err, out)
	}

	suite := gobdd.NewSuite(t)
	suite.AddStep(`seams is installed`, func(t gobdd.StepTest, ctx gobdd.Context) {
		ctx.Set("bin", bin)
	})
	suite.AddStep(`a developer runs seams (.+)`, func(t gobdd.StepTest, ctx gobdd.Context, args string) {
		path, err := ctx.GetString("bin")
		if err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command(path, strings.Fields(args)...).Output()
		if err != nil {
			t.Fatalf("seams %s: %v", args, err)
		}
		ctx.Set("stdout", string(out))
	})
	suite.AddStep(`seams prints its version`, func(t gobdd.StepTest, ctx gobdd.Context) {
		if out, _ := ctx.GetString("stdout"); out != "seams "+scenarioVersion+"\n" {
			t.Errorf("seams printed %q", out)
		}
	})
	suite.Run()
}
