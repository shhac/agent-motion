// Package skillmeta holds repo-level checks on the skill files. The skill is
// how an agent discovers this tool at all, so a malformed one fails silently:
// the tool simply never gets used, with nothing to indicate why.
package skillmeta

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// maxDescriptionLength is the harness limit on a skill description.
const maxDescriptionLength = 1024

func TestSkillFrontmatterWithinHarnessLimits(t *testing.T) {
	data, err := os.ReadFile("../../skills/agent-motion/SKILL.md")
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	match := regexp.MustCompile(`(?s)^---\n(.*?)\n---\n`).FindSubmatch(data)
	if match == nil {
		t.Fatal("SKILL.md has no YAML frontmatter, so the skill cannot be discovered")
	}
	fields := parseFrontmatter(string(match[1]))

	if name := unquote(fields["name"]); name != "agent-motion" {
		t.Errorf("frontmatter name = %q, want %q", name, "agent-motion")
	}
	description := scalar(fields["description"])
	if description == "" {
		t.Error("frontmatter description is missing; it is what routing matches against")
	}
	if len(description) > maxDescriptionLength {
		t.Errorf("frontmatter description is %d chars, over the %d limit",
			len(description), maxDescriptionLength)
	}
	allowed := fields["allowed-tools"]
	if !strings.Contains(allowed, "Bash(agent-motion") {
		t.Errorf("allowed-tools = %q, want it to permit the binary", allowed)
	}
	if !strings.Contains(allowed, "Read") {
		t.Error("allowed-tools must permit Read, or the skill's images cannot be looked at")
	}
}

// TestSkillReferencesResolve guards the links out of SKILL.md, which are how an
// agent reaches the flag and field reference at all.
func TestSkillReferencesResolve(t *testing.T) {
	const skill = "../../skills/agent-motion/SKILL.md"
	data, err := os.ReadFile(skill)
	if err != nil {
		t.Fatal(err)
	}
	links := regexp.MustCompile(`\]\((references/[^)]+)\)`).FindAllSubmatch(data, -1)
	if len(links) == 0 {
		t.Fatal("SKILL.md links to no reference files")
	}
	for _, link := range links {
		path := filepath.Join(filepath.Dir(skill), string(link[1]))
		if _, err := os.Stat(path); err != nil {
			t.Errorf("SKILL.md links to %s, which does not exist", link[1])
		}
	}
}

func parseFrontmatter(frontmatter string) map[string]string {
	fields := map[string]string{}
	var current string
	for _, line := range strings.Split(frontmatter, "\n") {
		if key, value, found := strings.Cut(line, ":"); found && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "-") {
			current = strings.TrimSpace(key)
			fields[current] = strings.TrimSpace(value)
			continue
		}
		if current != "" {
			fields[current] += " " + strings.TrimSpace(line)
		}
	}
	return fields
}

func unquote(v string) string { return strings.TrimSpace(strings.Trim(strings.TrimSpace(v), `"'`)) }

// scalar strips the block-scalar marker so the body is measured, not the marker.
func scalar(v string) string {
	v = strings.TrimSpace(v)
	for _, marker := range []string{"|-", ">-", "|", ">"} {
		if strings.HasPrefix(v, marker) {
			return strings.TrimSpace(strings.TrimPrefix(v, marker))
		}
	}
	return v
}
