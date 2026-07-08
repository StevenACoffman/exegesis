package book2skill_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
)

func TestAppendCustomSections(t *testing.T) {
	t.Parallel()
	existing := "# Index\n\nintro\n\n## Skills\n\n- old\n\n" +
		"## Notes\n\nhand-authored note.\n\n## More Notes\n\nkept too.\n"
	generated := "# Index\n\nintro\n\n## Skills\n\n- fresh\n"

	out := book2skill.AppendCustomSections(generated, existing)
	if !strings.Contains(out, "- fresh") || strings.Contains(out, "- old") {
		t.Errorf("owned section should be regenerated, not carried over:\n%s", out)
	}
	if !strings.Contains(out, "## Notes") || !strings.Contains(out, "hand-authored note.") {
		t.Errorf("custom section not preserved:\n%s", out)
	}
	if !strings.Contains(out, "## More Notes") || !strings.Contains(out, "kept too.") {
		t.Errorf("second custom section not preserved:\n%s", out)
	}

	// Idempotent: re-applying against its own output is a fixed point.
	if again := book2skill.AppendCustomSections(generated, out); again != out {
		t.Errorf(
			"AppendCustomSections not idempotent:\n--- once ---\n%s\n--- twice ---\n%s",
			out,
			again,
		)
	}

	// Case-insensitive ownership: a title-cased owned heading is not duplicated.
	titled := book2skill.AppendCustomSections(
		"# I\n\n## Skills\n\n- x\n",
		"# I\n\n## skills\n\n- y\n",
	)
	if strings.Count(titled, "Skills") != 1 || strings.Contains(titled, "- y") {
		t.Errorf("case-insensitive owned heading mishandled:\n%s", titled)
	}

	// No custom sections: unchanged.
	if got := book2skill.AppendCustomSections(generated, generated); got != generated {
		t.Errorf("expected unchanged output when there is nothing custom")
	}
}
