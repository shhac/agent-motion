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
		target := string(link[1])
		// A link may name a heading inside the file. Checking only the file
		// would let a link rot silently the moment a section is renamed, which
		// is the same failure this guard exists to prevent one level up.
		file, anchor, hasAnchor := strings.Cut(target, "#")
		path := filepath.Join(filepath.Dir(skill), file)
		body, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("SKILL.md links to %s, which does not exist", target)
			continue
		}
		if hasAnchor && !hasHeading(string(body), anchor) {
			t.Errorf("SKILL.md links to %s, but %s has no heading anchored at #%s", target, file, anchor)
		}
	}
}

// TestEveryReferenceIsReachable guards the other direction. SKILL.md is loaded
// every time the skill triggers; the reference files are loaded only when it
// points at them. One that nothing links to is never read, which is the same
// silent failure as a broken link and harder to notice.
func TestEveryReferenceIsReachable(t *testing.T) {
	const skill = "../../skills/agent-motion/SKILL.md"
	data, err := os.ReadFile(skill)
	if err != nil {
		t.Fatal(err)
	}
	linked := map[string]bool{}
	for _, link := range regexp.MustCompile(`\]\((references/[^)#]+)`).FindAllSubmatch(data, -1) {
		linked[filepath.Base(string(link[1]))] = true
	}
	entries, err := os.ReadDir(filepath.Join(filepath.Dir(skill), "references"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if !linked[entry.Name()] {
			t.Errorf("references/%s is not linked from SKILL.md, so nothing will ever read it", entry.Name())
		}
	}
}

// hasHeading reports whether a Markdown body carries a heading whose GitHub
// anchor is the one given.
func hasHeading(body, anchor string) bool {
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, "#") {
			continue
		}
		if headingAnchor(strings.TrimLeft(line, "# ")) == anchor {
			return true
		}
	}
	return false
}

func headingAnchor(heading string) string {
	var out strings.Builder
	for _, r := range strings.ToLower(heading) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			out.WriteRune(r)
		case r == ' ':
			out.WriteRune('-')
		}
	}
	return out.String()
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
