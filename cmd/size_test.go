package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLintSizeGateFlagAndCeilings checks the wiring of --max-lines and the registry's
// line_ceilings into lint; the gate's own cases are in internal/lint.
func TestLintSizeGateFlagAndCeilings(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "skilla")
	writeSkill(t, dir) // validSkill is 6 lines
	registry := filepath.Join(t.TempDir(), "registry.json")
	if err := os.WriteFile(registry, []byte(`{"line_ceilings":{"skilla":6}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		args    []string
		wantErr bool
		wantSub string
	}{
		"default limit passes a short skill": {args: []string{"lint", dir}},
		"a lower limit fails it": {
			args: []string{"lint", "--max-lines", "3", dir}, wantErr: true, wantSub: "6 lines > 3",
		},
		"a recorded ceiling excepts it": {
			args:    []string{"lint", "--max-lines", "3", "--registry", registry, dir},
			wantSub: "known exception at its ceiling",
		},
		"zero turns the gate off": {args: []string{"lint", "--max-lines", "0", dir}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			out, err := run(t, tc.args...)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v\n%s", err, tc.wantErr, out)
			}
			if !strings.Contains(out, tc.wantSub) {
				t.Errorf("want %q in:\n%s", tc.wantSub, out)
			}
		})
	}
}
