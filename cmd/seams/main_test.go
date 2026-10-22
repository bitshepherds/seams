package main

import (
	"strings"
	"testing"
)

func TestVersionPrintsTheStampedVersion(t *testing.T) {
	defer func(v string) { version = v }(version)
	version = "1.2.3"
	var out, errs strings.Builder
	if code := run([]string{"--version"}, &out, &errs); code != 0 || out.String() != "seams 1.2.3\n" {
		t.Fatalf("run(--version) = %d, %q, %q", code, out.String(), errs.String())
	}
}

func TestSeamsRefusesAnUnknownArgument(t *testing.T) {
	var out, errs strings.Builder
	if code := run([]string{"--nope"}, &out, &errs); code != 2 || !strings.Contains(errs.String(), "usage") {
		t.Fatalf("run(--nope) = %d, %q", code, errs.String())
	}
}
