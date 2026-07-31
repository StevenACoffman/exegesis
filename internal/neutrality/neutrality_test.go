package neutrality_test

import (
	"testing"

	"github.com/StevenACoffman/exegesis/internal/neutrality"
)

func TestScan(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		content  string
		wantHits int
		wantLine int
	}{
		{
			name:     "clean neutral skill",
			content:  "# Skill\nWorks in any agent runtime.\n",
			wantHits: 0,
		},
		{
			name:     "claude-code binding phrase",
			content:  "This is a Claude Code skill for your terminal.\n",
			wantHits: 1,
			wantLine: 1,
		},
		{
			name:     "hard-coded runtime path",
			content:  "line one\ncp it to ~/.claude/skills/foo\n",
			wantHits: 1,
			wantLine: 2,
		},
		{
			name:     "plugin install command",
			content:  "run /plugin install here\n",
			wantHits: 1,
			wantLine: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hits := neutrality.Scan([]neutrality.NamedFile{{Name: "SKILL.md", Content: tc.content}})
			if len(hits) != tc.wantHits {
				t.Fatalf("Scan hits = %d, want %d (%v)", len(hits), tc.wantHits, hits)
			}
			if tc.wantHits > 0 && hits[0].Line != tc.wantLine {
				t.Errorf("hit line = %d, want %d", hits[0].Line, tc.wantLine)
			}
		})
	}
}

func TestScanOrdersByFileThenLine(t *testing.T) {
	t.Parallel()
	files := []neutrality.NamedFile{
		{Name: "SKILL.md", Content: "ok\nClaude Code skill\n"},
		{Name: "README.md", Content: "Cursor only\n"},
	}
	hits := neutrality.Scan(files)
	if len(hits) != 2 {
		t.Fatalf("want 2 hits, got %d", len(hits))
	}
	if hits[0].File != "SKILL.md" || hits[1].File != "README.md" {
		t.Errorf("hits not ordered by input file: %v", hits)
	}
}
