package sheepdogseams

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
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
	registerSteps(suite)
	topSteps(suite, bin)
	inRepoSteps(suite, bin)
	contractSteps(suite)
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
	suite.AddStep(`the fixture system`, func(t gobdd.StepTest, ctx gobdd.Context) {
		copyFixture(t, ctx, "system")
	})
	suite.AddStep(`a registry that lacks a seam the code has, and lists one the code no longer has`, func(t gobdd.StepTest, ctx gobdd.Context) {
		copyFixture(t, ctx, "signals/messaging") // its code has event:checkout/orders
		writeRegistry(t, ctx, `{"id": "http:legacy/api", "provider": {"name": "legacy"}, "consumers": [{"name": "checkout"}], "audience": "internal"}`)
	})
}

// registerSteps are the register's: seams show and render.
func registerSteps(suite *gobdd.Suite) {
	suite.AddStep(`a registry of three seams, and a status file that puts one of them at no coverage`, func(t gobdd.StepTest, ctx gobdd.Context) {
		writeRegistry(t, ctx, registerSeams)
		status := `{"schema": "` + schemas.ID("status") + `", "seams": {"event:backend/quote-accepted": {"rung": 4},
			"http:stripe/payments": {"rung": 0, "gaps": ["no contract"]}, "http:backend/quotes": {"rung": 2}}}`
		if err := os.WriteFile(filepath.Join(get(t, ctx, "dir"), "status.json"), []byte(status), 0o600); err != nil {
			t.Fatal(err)
		}
	})
	suite.AddStep(`a registry with internal seams and one that crosses out of the system`, func(t gobdd.StepTest, ctx gobdd.Context) {
		writeRegistry(t, ctx, registerSeams)
	})
	suite.AddStep(`the uncovered seam is listed first, and each seam shows its rung`, func(t gobdd.StepTest, ctx gobdd.Context) {
		out := get(t, ctx, "stdout")
		if !strings.HasPrefix(out, "http:stripe/payments\nrung:                  0 of 5\n") {
			t.Errorf("seams show printed\n%s", out)
		}
		if strings.Count(out, "\nrung: ") != 3 {
			t.Errorf("not every seam shows its rung:\n%s", out)
		}
	})
	suite.AddStep(`docs/SEAMS.md lists the internal seams apart from the external one, under a diagram of the components and their seams`, func(t gobdd.StepTest, ctx gobdd.Context) {
		data, err := os.ReadFile(filepath.Join(get(t, ctx, "dir"), "docs", "SEAMS.md"))
		if err != nil {
			t.Fatal(err)
		}
		doc := string(data)
		diagram, internal, out := strings.Index(doc, "```mermaid"), strings.Index(doc, "## Internal seams"), strings.Index(doc, "## Seams that cross out of the system")
		under := func(id string, from, to int) bool {
			i := strings.Index(doc, "`"+id+"`")
			return i > from && (to < 0 || i < to)
		}
		if diagram < 0 || internal < diagram || out < internal || !under("http:stripe/payments", out, -1) ||
			!under("event:backend/quote-accepted", internal, out) || !under("http:backend/quotes", internal, out) {
			t.Errorf("seams render wrote\n%s", doc)
		}
	})
	suite.AddStep(`docs/SEAMS.md links to the guide to reading it, under its summary line`, func(t gobdd.StepTest, ctx gobdd.Context) {
		data, err := os.ReadFile(filepath.Join(get(t, ctx, "dir"), "docs", "SEAMS.md"))
		if err != nil {
			t.Fatal(err)
		}
		doc := string(data)
		summary, link := strings.Index(doc, "crossing out of the system.\n"), strings.Index(doc, "(https://github.com/bitshepherds/sheepdog-seams/blob/main/docs/READING-THE-REGISTER.md)")
		if summary < 0 || link < summary || strings.Contains(doc[summary:link], "## ") {
			t.Errorf("seams render wrote\n%s", doc)
		}
	})
}

// topSteps are for each seam's top rung, which whether its sides deploy
// apart, and whether its provider is an outside vendor, set.
func topSteps(suite *gobdd.Suite, bin string) {
	suite.AddStep(`a registry whose seam (\S+) deploys independently: (true|false|not known), and a status file that rates it (\d)`, func(t gobdd.StepTest, ctx gobdd.Context, id, independently, rung string) {
		writeRatedSeam(t, ctx, id, "", independently, rung)
	})
	suite.AddStep(`a registry whose seam (\S+) has an external (provider|consumer), deploys independently: (true|false|not known), and a status file that rates it (\d)`, func(t gobdd.StepTest, ctx gobdd.Context, id, external, independently, rung string) {
		writeRatedSeam(t, ctx, id, external, independently, rung)
	})
	suite.AddStep(`docs/SEAMS.md shows (\S+) at (\d of \d)`, func(t gobdd.StepTest, ctx gobdd.Context, id, rung string) {
		if code, _ := ctx.GetInt("code"); code != 0 {
			t.Fatalf("seams failed: %s", get(t, ctx, "stderr"))
		}
		data, err := os.ReadFile(filepath.Join(get(t, ctx, "dir"), "docs", "SEAMS.md"))
		if err != nil {
			t.Fatal(err)
		}
		row := regexp.MustCompile("(?m)^\\| `" + regexp.QuoteMeta(id) + "` \\|.*$").Find(data)
		if !strings.Contains(string(row), "| "+rung+" |") {
			t.Errorf("seams render wrote %s at\n%s", id, data)
		}
	})
	suite.AddStep(`seams show --json gives (\S+) a top of (\d)`, func(t gobdd.StepTest, ctx gobdd.Context, id, top string) {
		for _, args := range [][]string{{"show", "--json", "--status", "status.json"}, {"show", "--json", "--status", "status.json", id}} {
			runSeams(t, ctx, bin, args...)
			out := []byte(get(t, ctx, "stdout"))
			var doc struct {
				Seams []struct {
					Top *int `json:"top"`
				} `json:"seams"`
				Top *int `json:"top"`
			}
			if err := json.Unmarshal(out, &doc); err != nil {
				t.Fatalf("%v\n%s", err, out)
			}
			got := doc.Top
			if len(doc.Seams) == 1 {
				got = doc.Seams[0].Top
			}
			if got == nil || strconv.Itoa(*got) != top {
				t.Errorf("seams %s gave\n%s", strings.Join(args, " "), out)
			}
		}
	})
}

// writeRatedSeam writes a registry of one seam, with its external party
// ("provider", "consumer" or none) and whether its sides deploy apart, and a
// status file that rates it.
func writeRatedSeam(t gobdd.StepTest, ctx gobdd.Context, id, external, independently, rung string) {
	provider, consumer := `{"name": "backend"}`, `{"name": "web"}`
	switch external {
	case "provider":
		provider = `{"name": "stripe", "external": true}`
	case "consumer":
		consumer = `{"name": "bank", "external": true}`
	}
	deploys := ""
	if independently != "not known" {
		deploys = `, "deploysIndependently": ` + independently
	}
	writeRegistry(t, ctx, `{"id": "`+id+`", "provider": `+provider+`, "consumers": [`+consumer+`], "audience": "internal"`+deploys+`}`)
	status := `{"schema": "` + schemas.ID("status") + `", "seams": {"` + id + `": {"rung": ` + rung + `}}}`
	if err := os.WriteFile(filepath.Join(get(t, ctx, "dir"), "status.json"), []byte(status), 0o600); err != nil {
		t.Fatal(err)
	}
}

// registerSeams are two internal seams and a vendor's, which crosses out.
const registerSeams = `{"id": "event:backend/quote-accepted", "provider": {"name": "backend"}, "consumers": [{"name": "billing"}], "audience": "internal"},
	{"id": "http:stripe/payments", "provider": {"name": "stripe", "external": true}, "consumers": [{"name": "backend"}], "audience": "public"},
	{"id": "http:backend/quotes", "provider": {"name": "backend"}, "consumers": [{"name": "web"}], "audience": "internal"}`

// inRepoSteps are for parties recorded as in this repo, or as not in it.
func inRepoSteps(suite *gobdd.Suite, bin string) {
	suite.AddStep(`a registry whose seam (\S+) has a provider (not in|in) this repo`, func(t gobdd.StepTest, ctx gobdd.Context, id, where string) {
		provider := strings.SplitN(strings.SplitN(id, ":", 2)[1], "/", 2)[0]
		in := strconv.FormatBool(where == "in")
		writeRegistry(t, ctx, `{"id": "`+id+`", "provider": {"name": "`+provider+`", "inRepo": `+in+`}, "consumers": [{"name": "app"}], "audience": "internal"}`)
	})
	suite.AddStep(`the registry records (\S+) on (\S+) as (not in|in) this repo`, func(t gobdd.StepTest, ctx gobdd.Context, party, id, where string) {
		if code, _ := ctx.GetInt("code"); code != 0 {
			t.Fatalf("seams failed: %s", get(t, ctx, "stderr"))
		}
		data, err := os.ReadFile(registryPath(t, ctx))
		if err != nil {
			t.Fatal(err)
		}
		checkInRepo(t, data, id, party, where == "in")
	})
	suite.AddStep(`seams list --json reports (\S+) on (\S+) as (not in|in) this repo`, func(t gobdd.StepTest, ctx gobdd.Context, party, id, where string) {
		runSeams(t, ctx, bin, "list", "--json")
		checkInRepo(t, []byte(get(t, ctx, "stdout")), id, party, where == "in")
	})
	suite.AddStep(`it gets the list as JSON matching the published schema, with (\S+) not in this repo`, func(t gobdd.StepTest, ctx gobdd.Context, party string) {
		out := []byte(get(t, ctx, "stdout"))
		if err := schemas.Validate("list", out); err != nil {
			t.Errorf("%v\n%s", err, out)
		}
		if !bytes.Contains(out, []byte(`"inRepo": false`)) {
			t.Errorf("the list doesn't say %s isn't in this repo:\n%s", party, out)
		}
	})
	refused := func(t gobdd.StepTest, ctx gobdd.Context, want string) {
		if code, _ := ctx.GetInt("code"); code == 0 {
			t.Fatal("seams succeeded")
		}
		if errs := get(t, ctx, "stderr"); !strings.Contains(errs, want) {
			t.Errorf("seams said %q, without %q", errs, want)
		}
		if data, err := os.ReadFile(registryPath(t, ctx)); err != nil || string(data) != get(t, ctx, "registry") {
			t.Errorf("the registry changed: %v\n%s", err, data)
		}
	}
	suite.AddStep(`it fails, saying (\S+) can't be both in this repo and not, and the registry is unchanged`, func(t gobdd.StepTest, ctx gobdd.Context, party string) {
		refused(t, ctx, party+" can't be both in this repo and not")
	})
	suite.AddStep(`it fails, naming (\S+), and the registry is unchanged`, func(t gobdd.StepTest, ctx gobdd.Context, party string) {
		refused(t, ctx, party)
	})
}

// contractSteps are for a seam's contract: where it lives, and who owns it.
func contractSteps(suite *gobdd.Suite) {
	suite.AddStep(`a registry whose seam (\S+) has the contract (\S+), owned by (\S+)`, func(t gobdd.StepTest, ctx gobdd.Context, id, location, owner string) {
		provider := strings.SplitN(strings.SplitN(id, ":", 2)[1], "/", 2)[0]
		writeRegistry(t, ctx, `{"id": "`+id+`", "provider": {"name": "`+provider+`"}, "consumers": [{"name": "app"}], "audience": "internal",
			"contract": {"location": "`+location+`", "owner": "`+owner+`"}}`)
	})
	suite.AddStep(`the registry records the contract of (\S+) at (\S+), owned by (\S+)`, func(t gobdd.StepTest, ctx gobdd.Context, id, location, owner string) {
		if code, _ := ctx.GetInt("code"); code != 0 {
			t.Fatalf("seams failed: %s", get(t, ctx, "stderr"))
		}
		data, err := os.ReadFile(registryPath(t, ctx))
		if err != nil {
			t.Fatal(err)
		}
		type contract struct{ Location, Owner string }
		var doc struct {
			Seams []struct {
				ID       string    `json:"id"`
				Contract *contract `json:"contract"`
			} `json:"seams"`
		}
		if err := json.Unmarshal(data, &doc); err != nil {
			t.Fatalf("%v\n%s", err, data)
		}
		want := contract{location, owner}
		for _, s := range doc.Seams {
			if s.ID == id {
				if s.Contract == nil || *s.Contract != want {
					t.Errorf("the contract of %s is %+v, not %+v", id, s.Contract, want)
				}
				return
			}
		}
		t.Errorf("no seam %s:\n%s", id, data)
	})
}

// checkInRepo checks that a document's seams, the registry's or a list's,
// record the party on the seam as in this repo or not: its inRepo is there,
// and true or false, not left out.
func checkInRepo(t gobdd.StepTest, data []byte, id, party string, want bool) {
	var doc struct {
		Seams []struct {
			ID        string            `json:"id"`
			Provider  json.RawMessage   `json:"provider"`
			Consumers []json.RawMessage `json:"consumers"`
		} `json:"seams"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("%v\n%s", err, data)
	}
	for _, s := range doc.Seams {
		if s.ID != id {
			continue
		}
		for _, raw := range append([]json.RawMessage{s.Provider}, s.Consumers...) {
			var p struct {
				Name   string `json:"name"`
				InRepo *bool  `json:"inRepo"`
			}
			_ = json.Unmarshal(raw, &p) // a null provider is no party
			if p.Name != party {
				continue
			}
			if p.InRepo == nil || *p.InRepo != want {
				t.Errorf("%s on %s is %s, not inRepo %t", party, id, raw, want)
			}
			return
		}
	}
	t.Errorf("no party %s on %s:\n%s", party, id, data)
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
	suite.AddStep(`each candidate is listed with its kind, provider, consumers, evidence by file and line, and confidence`, func(t gobdd.StepTest, ctx gobdd.Context) {
		checkCandidates(t, get(t, ctx, "stdout"))
	})
	suite.AddStep(`both are named, each with why`, func(t gobdd.StepTest, ctx gobdd.Context) {
		out := get(t, ctx, "stdout")
		if code, _ := ctx.GetInt("code"); code != 1 {
			t.Errorf("seams detect --drift exited %d", code)
		}
		for _, want := range []string{"missing: event:checkout/orders  the code has it", "stale: http:legacy/api  registered, but nothing"} {
			if !strings.Contains(out, want) {
				t.Errorf("seams printed %q, without %q", out, want)
			}
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

var (
	candidateLine = regexp.MustCompile(`^(?:http|grpc|graphql|event|queue|file|config|table|lib|cli):\S+/\S+  \S.* → \S.*  \((?:low|medium|high), (?:internal|public)\)$`)
	evidenceLine  = regexp.MustCompile(`^    (?:provides|consumes|owns|supports) +\S+:\d+  \S+  `)
)

// checkCandidates checks seams detect's listing: each candidate on a line
// of its own, with its id, provider, consumers and confidence, followed by
// its evidence, each by file and line.
func checkCandidates(t gobdd.StepTest, out string) {
	candidates, evidence := 0, 0
	for _, l := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		switch {
		case candidateLine.MatchString(l):
			if candidates > 0 && evidence == 0 {
				t.Errorf("a candidate before %q has no evidence", l)
			}
			candidates, evidence = candidates+1, 0
		case evidenceLine.MatchString(l):
			evidence++
		case strings.HasPrefix(l, "    note: "), strings.HasSuffix(l, " component(s)"):
		default:
			t.Errorf("seams detect printed the line %q", l)
		}
	}
	if candidates < 8 || evidence == 0 {
		t.Errorf("seams detect listed %d candidate(s):\n%s", candidates, out)
	}
}

// copyFixture copies a fixture repo from testdata/ into the scenario's
// directory.
func copyFixture(t gobdd.StepTest, ctx gobdd.Context, name string) {
	if err := os.CopyFS(get(t, ctx, "dir"), os.DirFS(filepath.Join("testdata", name))); err != nil {
		t.Fatal(err)
	}
}

// writeRegistry writes the scenario's registry, at the default path, with
// the seams given as JSON.
func writeRegistry(t gobdd.StepTest, ctx gobdd.Context, seams string) {
	path := registryPath(t, ctx)
	doc := `{"schema": "` + schemas.ID("registry") + `", "seams": [` + seams + `]}`
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx.Set("registry", doc)
}

// registryPath is the scenario's registry, at the default path.
func registryPath(t gobdd.StepTest, ctx gobdd.Context) string {
	return filepath.Join(get(t, ctx, "dir"), ".sheepdog", "seams", "registry.json")
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
