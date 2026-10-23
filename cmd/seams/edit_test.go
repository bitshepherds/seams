package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitshepherds/sheepdog-seams/internal/registry"
)

// emptyRegistry is the path of a registry not written yet.
func emptyRegistry(t *testing.T) string {
	return filepath.Join(t.TempDir(), ".sheepdog", "seams", "registry.json")
}

func mustLoad(t *testing.T, path string) *registry.Registry {
	t.Helper()
	r, err := registry.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func mustSeams(t *testing.T, args ...string) string {
	t.Helper()
	code, out, errs := seams(args...)
	if code != 0 {
		t.Fatalf("seams %s = %d: %s", strings.Join(args, " "), code, errs)
	}
	return out
}

func TestAddRegistersASeamWithItsIDsProviderByDefault(t *testing.T) {
	path := emptyRegistry(t)
	out := mustSeams(t, "add", "--registry", path, "event:backend/quote-accepted", "--consumer", "billing,mailer",
		"--contract", "events/quote-accepted.json", "--owner", "backend-team", "--deploys-independently", "true",
		"--in-repo", "backend", "--audience", "internal")
	if out != "added event:backend/quote-accepted\n" {
		t.Errorf("add printed %q", out)
	}
	s, _ := mustLoad(t, path).Find("event:backend/quote-accepted")
	if s.Provider.Name != "backend" || !s.Provider.InRepo || len(s.Consumers) != 2 || s.Contract.Owner != "backend-team" || !*s.DeploysIndependently {
		t.Fatalf("registered %+v", s)
	}
}

func TestCommandQueueIsRegisteredWithItsReceiverAsProvider(t *testing.T) {
	path := emptyRegistry(t)
	mustSeams(t, "add", "--registry", path, "queue:worker/resize-image", "--sender", "api", "--sender", "admin", "--receiver", "worker")
	s, _ := mustLoad(t, path).Find("queue:worker/resize-image")
	if s.Provider.Name != "worker" || len(s.Consumers) != 2 || s.Consumers[0].Name != "api" {
		t.Fatalf("registered %+v", s)
	}
	// Named for the sender, the id doesn't name the queue's provider.
	code, _, errs := seams("add", "--registry", path, "queue:api/resize-thumbnail", "--sender", "api", "--receiver", "worker")
	if code != 1 || !strings.Contains(errs, "provided by worker, but its id names api") {
		t.Fatalf("a queue named for its sender: %d, %q", code, errs)
	}
}

func TestAnEventIsRegisteredWithItsSenderAsProvider(t *testing.T) {
	path := emptyRegistry(t)
	mustSeams(t, "add", "--registry", path, "event:backend/quote-accepted", "--sender", "backend", "--receiver", "billing")
	if s, _ := mustLoad(t, path).Find("event:backend/quote-accepted"); s.Provider.Name != "backend" || s.Consumers[0].Name != "billing" {
		t.Fatalf("registered %+v", s)
	}
}

func TestAddRefusesWhatCannotBeASeam(t *testing.T) {
	path := emptyRegistry(t)
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"quotes", "--consumer", "web"}, "isn't an id"},
		{[]string{"http:backend/quotes"}, "at least one consumer"},
		{[]string{"http:backend/quotes", "--sender", "web"}, "for event and queue seams"},
		{[]string{"event:backend/a", "--sender", "x", "--sender", "y", "--receiver", "z"}, "give one --sender"},
		{[]string{"queue:w/a", "--sender", "x"}, "give one --receiver"},
		{[]string{"event:backend/a", "--provider", "backend", "--sender", "backend", "--receiver", "z"}, "not both"},
		{[]string{"http:backend/quotes", "--consumer", "web", "--audience", "everyone"}, "internal or public"},
		{[]string{"http:backend/quotes", "--consumer", "web", "--owner", "team"}, "--owner needs a contract"},
		{[]string{"http:backend/quotes", "--consumer", "web", "--deploys-independently", "often"}, "true or false"},
		{[]string{"http:backend/quotes", "--consumer", "web", "--test", "consumer:web/quotes_test.go"}, "<provider|consumer>:<test>:<tier>"},
		{[]string{"http:backend/quotes", "--consumer", "web", "--test", "both:web/quotes_test.go:gate"}, "provider or consumer"},
	} {
		code, _, errs := seams(append([]string{"add", "--registry", path}, tc.args...)...)
		if code != 1 || !strings.Contains(errs, tc.want) {
			t.Errorf("add %v = %d, %q; want %q", tc.args, code, errs, tc.want)
		}
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("a refused seam was saved")
	}
	if code, _, _ := seams("add", "--registry", path); code != 2 {
		t.Errorf("add with no id gave %d", code)
	}
}

func TestAddRegistersASeamWithNoSingleOwnerAndAnExternalProvider(t *testing.T) {
	path := emptyRegistry(t)
	mustSeams(t, "add", "--registry", path, "table:_/quotes", "--consumer", "backend,reports")
	mustSeams(t, "add", "--registry", path, "http:stripe/payments", "--consumer", "checkout", "--external", "stripe", "--audience", "public")
	r := mustLoad(t, path)
	if s, _ := r.Find("table:_/quotes"); s.Provider != nil {
		t.Errorf("the ownerless seam has provider %+v", s.Provider)
	}
	if s, _ := r.Find("http:stripe/payments"); !s.Provider.External || s.Audience != registry.Public {
		t.Errorf("the vendor seam is %+v", s)
	}
}

func TestAddAndUpdateRecordTheSeamsTestWorld(t *testing.T) {
	path := emptyRegistry(t)
	mustSeams(t, "add", "--registry", path, "http:backend/quotes", "--consumer", "web", "--test", "provider:backend/quotes_test.go:gate")
	mustSeams(t, "update", "--registry", path, "http:backend/quotes", "--test", "consumer:pact:web/quotes:gate")
	s, _ := mustLoad(t, path).Find("http:backend/quotes")
	if len(s.TestWorld) != 2 || s.TestWorld[1].Test != "pact:web/quotes" || s.TestWorld[1].Side != "consumer" || s.TestWorld[0].Tier != "gate" {
		t.Fatalf("test world %+v", s.TestWorld)
	}
}

func TestUpdateRenamesAndChangesASeam(t *testing.T) {
	path := emptyRegistry(t)
	mustSeams(t, "add", "--registry", path, "http:backend/prices", "--consumer", "web,mobile")
	out := mustSeams(t, "update", "--registry", path, "http:backend/prices", "--rename", "http:backend/quotes",
		"--add-consumer", "partner", "--remove-consumer", "mobile", "--external", "partner", "--audience", "public")
	if out != "updated http:backend/quotes\n" {
		t.Errorf("update printed %q", out)
	}
	s, _ := mustLoad(t, path).Find("http:backend/prices")
	if s.ID != "http:backend/quotes" || len(s.Consumers) != 2 || s.Consumers[1].Name != "partner" || !s.Consumers[1].External || s.Audience != registry.Public {
		t.Fatalf("updated to %+v", s)
	}
}

func TestUpdateRefusesAChangeWithProblemsAndKeepsTheRegistry(t *testing.T) {
	path := emptyRegistry(t)
	mustSeams(t, "add", "--registry", path, "http:backend/quotes", "--consumer", "web")
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"http:backend/quotes", "--add-consumer", "backend"}, "both provider and consumer"},
		{[]string{"http:backend/quotes", "--audience", "everyone"}, "internal or public"},
		{[]string{"http:backend/nothing", "--audience", "public"}, "no seam"},
	} {
		code, _, errs := seams(append([]string{"update", "--registry", path}, tc.args...)...)
		if code != 1 || !strings.Contains(errs, tc.want) {
			t.Errorf("update %v = %d, %q; want %q", tc.args, code, errs, tc.want)
		}
	}
	if s, _ := mustLoad(t, path).Find("http:backend/quotes"); len(s.Consumers) != 1 || s.Audience != registry.Internal {
		t.Errorf("a refused change was saved: %+v", s)
	}
	if code, _, _ := seams("update", "--registry", path); code != 2 {
		t.Errorf("update with no id gave %d", code)
	}
}

func TestRejectRemembersACandidateOnce(t *testing.T) {
	path := emptyRegistry(t)
	args := []string{"reject", "--registry", path, "http:backend/health", "--signal", "route", "--file", "backend/main.go", "--reason", "a probe"}
	if out := mustSeams(t, args...); out != "rejected http:backend/health\n" {
		t.Errorf("reject printed %q", out)
	}
	if out := mustSeams(t, args...); out != "rejected already: http:backend/health\n" {
		t.Errorf("rejecting again printed %q", out)
	}
	if r := mustLoad(t, path); len(r.Rejected) != 1 || r.Rejected[0].Reason != "a probe" {
		t.Fatalf("rejected %+v", r.Rejected)
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"http:backend/ready"}, "give --signal and --file"},
		{[]string{"ready", "--signal", "route", "--file", "main.go"}, "isn't an id"},
	} {
		code, _, errs := seams(append([]string{"reject", "--registry", path}, tc.args...)...)
		if code != 1 || !strings.Contains(errs, tc.want) {
			t.Errorf("reject %v = %d, %q; want %q", tc.args, code, errs, tc.want)
		}
	}
	if code, _, _ := seams("reject", "--registry", path); code != 2 {
		t.Errorf("reject with no candidate gave %d", code)
	}
}

func TestEditingCommandsFailOnARegistryTheyCannotReadOrWrite(t *testing.T) {
	broken := filepath.Join(t.TempDir(), "seams.json")
	if err := os.WriteFile(broken, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A link to nowhere reads as no registry yet, but can't be written through.
	unwritable := filepath.Join(t.TempDir(), "seams.json")
	if err := os.Symlink(filepath.Join(t.TempDir(), "gone", "seams.json"), unwritable); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "--registry", broken, "http:backend/quotes", "--consumer", "web"},
		{"add", "--registry", unwritable, "http:backend/quotes", "--consumer", "web"},
		{"reject", "--registry", broken, "http:backend/health", "--signal", "route", "--file", "main.go"},
		{"reject", "--registry", unwritable, "http:backend/health", "--signal", "route", "--file", "main.go"},
		{"list", "--registry", broken},
		{"show", "--registry", broken, "http:backend/quotes"},
	} {
		if code, _, _ := seams(args...); code != 1 {
			t.Errorf("seams %v = %d", args, code)
		}
	}
}
