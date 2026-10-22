package sheepdogseams

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-bdd/gobdd"

	"github.com/bitshepherds/sheepdog-seams/schemas"
)

// The version the scenarios' build of seams is stamped with.
const scenarioVersion = "0.0.0-scenario"

// TestScenarios runs the Gherkin scenarios in features/ against a real build
// of seams, driving it from outside, as a developer or a tool would, each in
// a working directory of its own.
func TestScenarios(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "seams")
	build := exec.Command("go", "build", "-o", bin,
		"-ldflags", "-X main.version="+scenarioVersion, "./cmd/seams")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building seams: %v\n%s", err, out)
	}

	suite := gobdd.NewSuite(t, gobdd.WithBeforeScenario(func(ctx gobdd.Context) {
		dir, err := os.MkdirTemp("", "seams-scenario-")
		if err != nil {
			panic(err)
		}
		ctx.Set("dir", dir)
	}), gobdd.WithAfterScenario(func(ctx gobdd.Context) {
		if dir, err := ctx.GetString("dir"); err == nil {
			_ = os.RemoveAll(dir)
		}
	}))
	givenSteps(suite)
	whenSteps(suite, bin)
	thenSteps(suite, bin)
	suite.Run()
}

func givenSteps(suite *gobdd.Suite) {
	suite.AddStep(`seams is installed`, func(gobdd.StepTest, gobdd.Context) {})
	suite.AddStep(`an empty registry`, func(gobdd.StepTest, gobdd.Context) {})
	suite.AddStep(`a registry with a seam whose id is (\S+)`, func(t gobdd.StepTest, ctx gobdd.Context, id string) {
		writeRegistry(t, ctx, `{"id": "`+id+`", "provider": {"name": "backend"}, "consumers": [{"name": "billing"}], "audience": "internal"}`)
	})
	suite.AddStep(`a registry with the seam (\S+)`, func(t gobdd.StepTest, ctx gobdd.Context, id string) {
		provider := strings.SplitN(strings.SplitN(id, ":", 2)[1], "/", 2)[0]
		writeRegistry(t, ctx, `{"id": "`+id+`", "provider": {"name": "`+provider+`"}, "consumers": [{"name": "checkout"}], "audience": "internal"}`)
	})
}

func whenSteps(suite *gobdd.Suite, bin string) {
	suite.AddStep(`a developer adds the seam (\S+), provided by (\S+) and consumed by (\S+)`, func(t gobdd.StepTest, ctx gobdd.Context, id, provider, consumer string) {
		runSeams(t, ctx, bin, "add", id, "--provider", provider, "--consumer", consumer)
		if code, _ := ctx.GetInt("code"); code != 0 {
			t.Fatalf("seams add failed: %s", get(t, ctx, "stderr"))
		}
	})
	suite.AddStep(`(?:a developer|a tool) runs seams (.+)`, func(t gobdd.StepTest, ctx gobdd.Context, args string) {
		runSeams(t, ctx, bin, strings.Fields(args)...)
	})
}

func thenSteps(suite *gobdd.Suite, bin string) {
	suite.AddStep(`seams prints its version`, func(t gobdd.StepTest, ctx gobdd.Context) {
		if out := get(t, ctx, "stdout"); out != "seams "+scenarioVersion+"\n" {
			t.Errorf("seams printed %q", out)
		}
	})
	suite.AddStep(`it fails, naming (\S+) and the form (\S+)`, func(t gobdd.StepTest, ctx gobdd.Context, seam, form string) {
		if code, _ := ctx.GetInt("code"); code == 0 {
			t.Fatal("seams succeeded")
		}
		if out := get(t, ctx, "stdout") + get(t, ctx, "stderr"); !strings.Contains(out, seam) || !strings.Contains(out, form) {
			t.Errorf("seams said %q", out)
		}
	})
	suite.AddStep(`seams list shows (\S+), with (\S+) as its provider and (\S+) as its consumer`, func(t gobdd.StepTest, ctx gobdd.Context, id, provider, consumer string) {
		runSeams(t, ctx, bin, "list")
		if out := get(t, ctx, "stdout"); !strings.Contains(out, id+"  "+provider+" → "+consumer) {
			t.Errorf("seams list printed %q", out)
		}
	})
	suite.AddStep(`it gets the seam as JSON matching the published schema`, func(t gobdd.StepTest, ctx gobdd.Context) {
		if err := schemas.Validate("seam", []byte(get(t, ctx, "stdout"))); err != nil {
			t.Errorf("%v\n%s", err, get(t, ctx, "stdout"))
		}
	})
}

func get(t gobdd.StepTest, ctx gobdd.Context, key string) string {
	v, err := ctx.GetString(key)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// writeRegistry writes the scenario's registry, at the default path, with
// the seams given as JSON.
func writeRegistry(t gobdd.StepTest, ctx gobdd.Context, seams string) {
	path := filepath.Join(get(t, ctx, "dir"), ".sheepdog", "seams.json")
	doc := `{"schema": "` + schemas.ID("registry") + `", "seams": [` + seams + `]}`
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
}

// runSeams runs seams in the scenario's directory, keeping its exit code and
// what it wrote.
func runSeams(t gobdd.StepTest, ctx gobdd.Context, bin string, args ...string) {
	var out, errs bytes.Buffer
	cmd := exec.Command(bin, args...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = get(t, ctx, "dir"), &out, &errs
	err := cmd.Run()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		t.Fatalf("running seams: %v", err)
	}
	ctx.Set("code", cmd.ProcessState.ExitCode())
	ctx.Set("stdout", out.String())
	ctx.Set("stderr", errs.String())
}
