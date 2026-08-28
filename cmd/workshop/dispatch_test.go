package main

import (
	"sort"
	"testing"
)

func TestRootHasAllSubcommands(t *testing.T) {
	root := newRootCmd()
	var names []string
	for _, c := range root.Commands() {
		names = append(names, c.Name())
	}
	sort.Strings(names)
	want := map[string]bool{"serve": false, "service": false, "compile": false, "setup": false, "run": false}
	for _, n := range names {
		if _, ok := want[n]; ok {
			want[n] = true
		}
	}
	for n, seen := range want {
		if !seen {
			t.Errorf("missing subcommand %q (have %v)", n, names)
		}
	}
}
