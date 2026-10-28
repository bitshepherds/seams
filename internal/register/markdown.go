package register

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bitshepherds/seams/internal/registry"
)

// DefaultDoc is where seams render writes the register.
const DefaultDoc = "docs/SEAMS.md"

// Guide is the plain-language guide to reading the register. It's on main,
// not pinned to a release, so the link works in any repo that uses seams, and
// stays the same from one release to the next: the rungs' meanings are stable
// under the status schema's v1.
const Guide = "https://github.com/bitshepherds/sheepdog-seams/blob/main/docs/READING-THE-REGISTER.md"

// Markdown is the register for the team to read: a link to the guide to
// reading it, a diagram of the components and the seams between them, the
// internal seams apart from those that cross out of the system, and the
// history of the count. rated says a status file was given, so each seam's
// rung is shown.
func Markdown(entries []Entry, h *History, rated bool) string {
	var b strings.Builder
	internal, out := split(entries)
	b.WriteString("# Seams\n\n")
	b.WriteString("<!-- Written by seams render, from the registry. Run it again rather than editing this file. -->\n\n")
	fmt.Fprintf(&b, "%d seam(s): %d internal, %d crossing out of the system.\n", len(entries), len(internal), len(out))
	fmt.Fprintf(&b, "\nNew to the register? See [how to read it](%s).\n", Guide)
	if len(entries) > 0 {
		b.WriteString("\n## Components and seams\n\n")
		diagram(&b, entries)
	}
	table(&b, "Internal seams", internal, rated)
	table(&b, "Seams that cross out of the system", out, rated)
	if h != nil && len(h.Counts) > 0 {
		history(&b, h)
	}
	return b.String()
}

func split(entries []Entry) (internal, out []Entry) {
	for _, e := range entries {
		if e.CrossesOut {
			out = append(out, e)
		} else {
			internal = append(internal, e)
		}
	}
	return internal, out
}

// diagram draws the components as nodes, those inside the system in a
// subgraph, and each seam as an arrow from its provider to each consumer. A
// seam with no single owner is a node of its own, which its consumers use.
func diagram(b *strings.Builder, entries []Entry) {
	external := map[string]bool{}
	var parties []string
	for _, e := range entries {
		for _, p := range sides(e.Seam) {
			if !external[p.Name] && !contains(parties, p.Name) {
				parties = append(parties, p.Name)
			}
			external[p.Name] = external[p.Name] || p.External
		}
	}
	node := func(name string) string { return "p" + strconv.Itoa(index(parties, name)+1) }
	var inside, outside, edges []string
	for _, name := range parties {
		decl := fmt.Sprintf("%s[%s]", node(name), label(name))
		if external[name] {
			outside = append(outside, decl)
		} else {
			inside = append(inside, decl)
		}
	}
	for i, e := range entries {
		from := ""
		if e.Seam.Provider != nil {
			from = node(e.Seam.Provider.Name) + " -->|" + label(e.Seam.ID) + "| "
		} else {
			id := "s" + strconv.Itoa(i+1)
			decl := fmt.Sprintf("%s{{%s}}", id, label(e.Seam.ID))
			if e.CrossesOut {
				outside = append(outside, decl)
			} else {
				inside = append(inside, decl)
			}
			from = id + " --> "
		}
		for _, c := range e.Seam.Consumers {
			edges = append(edges, from+node(c.Name))
		}
	}
	b.WriteString("```mermaid\nflowchart LR\n")
	if len(inside) > 0 {
		b.WriteString("  subgraph system[\"The system\"]\n")
		for _, d := range inside {
			b.WriteString("    " + d + "\n")
		}
		b.WriteString("  end\n")
	}
	for _, line := range append(outside, edges...) {
		b.WriteString("  " + line + "\n")
	}
	b.WriteString("```\n")
}

// sides are the seam's provider, if it has one, and its consumers.
func sides(s registry.Seam) []registry.Party {
	ps := append([]registry.Party(nil), s.Consumers...)
	if s.Provider != nil {
		ps = append([]registry.Party{*s.Provider}, ps...)
	}
	return ps
}

func contains(names []string, name string) bool { return index(names, name) >= 0 }

func index(names []string, name string) int {
	for i, n := range names {
		if n == name {
			return i
		}
	}
	return -1
}

// label is Mermaid's quoted text, on one line, with its quotes escaped.
func label(s string) string { return `"` + strings.ReplaceAll(oneLine(s), `"`, "#quot;") + `"` }

// oneLine is the text with its line breaks as spaces.
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func table(b *strings.Builder, title string, entries []Entry, rated bool) {
	fmt.Fprintf(b, "\n## %s\n\n", title)
	if len(entries) == 0 {
		b.WriteString("None.\n")
		return
	}
	head := []string{"Seam", "Kind", "Provider", "Consumers", "Audience", "Contract", "Deploys independently", "Test world"}
	if rated {
		head = append(head, "Rung", "Gaps")
	}
	row(b, head)
	row(b, strings.Split(strings.Repeat("---,", len(head)-1)+"---", ","))
	for _, e := range entries {
		cells := []string{"`" + e.Seam.ID + "`", e.Kind, ProviderName(e.Seam), ConsumerNames(e.Seam), e.Seam.Audience,
			ContractOf(e.Seam), Independence(e.Seam), strings.Join(TestWorld(e.Seam), "; ")}
		if rated {
			cells = append(cells, RungOf(e), strings.Join(e.Gaps, "; "))
		}
		row(b, cells)
	}
}

// row writes a table row, an empty cell as a dash, so it isn't misread, and
// each cell on one line.
func row(b *strings.Builder, cells []string) {
	for i, c := range cells {
		if c == "" {
			c = "—"
		}
		cells[i] = strings.ReplaceAll(oneLine(c), "|", `\|`)
	}
	b.WriteString("| " + strings.Join(cells, " | ") + " |\n")
}

func history(b *strings.Builder, h *History) {
	b.WriteString("\n## History\n\n")
	head := []string{"Date", "Seams"}
	for r := 0; r <= Top; r++ {
		head = append(head, "Rung "+strconv.Itoa(r))
	}
	head = append(head, "Not rated")
	row(b, head)
	row(b, strings.Split(strings.Repeat("---,", len(head)-1)+"---", ","))
	for _, c := range h.Counts {
		cells := []string{c.Date, strconv.Itoa(c.Seams)}
		for r := 0; r <= Top; r++ {
			cells = append(cells, rungCell(c, r))
		}
		if c.Rungs == nil {
			cells = append(cells, "")
		} else {
			cells = append(cells, strconv.Itoa(*c.Unrated))
		}
		row(b, cells)
	}
}

func rungCell(c Count, r int) string {
	if c.Rungs == nil {
		return ""
	}
	return strconv.Itoa(c.Rungs[r])
}

// ProviderName is the seam's provider, marked, or that it has no single owner.
func ProviderName(s registry.Seam) string {
	if s.Provider == nil {
		return "no single owner"
	}
	return PartyName(*s.Provider)
}

// ConsumerNames are the seam's consumers, each marked.
func ConsumerNames(s registry.Seam) string {
	ns := make([]string, len(s.Consumers))
	for i, p := range s.Consumers {
		ns[i] = PartyName(p)
	}
	return strings.Join(ns, ", ")
}

// PartyName is the party's name, marked when it's external or in this repo.
func PartyName(p registry.Party) string {
	var marks []string
	if p.External {
		marks = append(marks, "external")
	}
	if p.IsInRepo() {
		marks = append(marks, "in this repo")
	}
	if len(marks) == 0 {
		return p.Name
	}
	return p.Name + " (" + strings.Join(marks, ", ") + ")"
}

// ContractOf is where the seam's contract lives, and who owns it.
func ContractOf(s registry.Seam) string {
	if s.Contract == nil {
		return "none"
	}
	if s.Contract.Owner == "" {
		return s.Contract.Location
	}
	return s.Contract.Location + " (owned by " + s.Contract.Owner + ")"
}

// Independence says whether the seam's sides deploy independently.
func Independence(s registry.Seam) string {
	switch {
	case s.DeploysIndependently == nil:
		return "not known"
	case *s.DeploysIndependently:
		return "yes"
	default:
		return "no"
	}
}

// TestWorld is each of the seam's contract tests: its side, the test, its
// tier, and the tool that runs it.
func TestWorld(s registry.Seam) []string {
	var tests []string
	for _, t := range s.TestWorld {
		line := t.Side + ": " + t.Test + ", in the " + t.Tier
		if t.Tool != "" {
			line += ", with " + t.Tool
		}
		tests = append(tests, line)
	}
	return tests
}

// RungOf is the entry's rung of the seam's top, or that it isn't rated. A
// rung above the top is shown as given: the rating isn't seams' to judge.
func RungOf(e Entry) string {
	if e.Rung == nil {
		return "not rated"
	}
	return fmt.Sprintf("%d of %d", *e.Rung, e.Top)
}
