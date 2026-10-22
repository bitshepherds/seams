package main

import (
	"fmt"
	"os"

	"github.com/bitshepherds/sheepdog-seams/internal/registry"
)

func importFrom(c *cli, args []string) int {
	fs := c.flags("import")
	file := fs.String("file", "catalog-info.yaml", "the Backstage catalog file")
	if !c.parse(fs, args, 1) {
		return 2
	}
	if c.args[0] != "catalog-info" {
		return c.fail(fmt.Errorf("seams imports from catalog-info, not %q", c.args[0]))
	}
	data, err := os.ReadFile(*file) //nolint:gosec // the catalog's path is the user's to choose
	if err != nil {
		return c.fail(err)
	}
	r, err := c.load()
	if err != nil {
		return c.fail(err)
	}
	changes, err := r.ImportCatalog(data)
	if err != nil {
		return c.fail(err)
	}
	if err := r.Save(c.registry); err != nil {
		return c.fail(err)
	}
	if c.json {
		if changes == nil {
			changes = []registry.Change{}
		}
		c.emit("import", map[string]any{"changes": changes})
		return 0
	}
	for _, ch := range changes {
		_, _ = fmt.Fprintf(c.out, "%s %s: %s\n", ch.Action, ch.Seam, ch.Message)
	}
	if len(changes) == 0 {
		_, _ = fmt.Fprintln(c.out, "catalog-info names no APIs")
	}
	return 0
}
