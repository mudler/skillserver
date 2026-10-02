package domain

import (
	"fmt"
	"os"
	"path"
	"strings"
)

// ExcludeSkillsEnv is the environment variable holding a comma-separated list
// of skill name patterns to exclude (e.g. "superpowers/using-superpowers").
const ExcludeSkillsEnv = "SKILLSERVER_EXCLUDE_SKILLS"

// Option configures a FileSystemManager.
type Option func(*managerConfig)

type managerConfig struct {
	excludePatterns []string
}

// WithExcludedSkills hides skills whose name matches one of the given glob
// patterns (path.Match syntax, matched against the skill name as returned by
// ListSkills, e.g. "my-skill" or "repoName/skillName"). Note that "*" does not
// cross "/": "superpowers/*" excludes every skill of the superpowers repo,
// "*/using-*" excludes matching skills of any repo.
//
// Excluded skills are hidden from ListSkills, SearchSkills, ReadSkill and the
// resource methods. Their files are not modified, so git syncs keep working.
// This option overrides the SKILLSERVER_EXCLUDE_SKILLS environment variable;
// pass it with no patterns to disable exclusion explicitly.
func WithExcludedSkills(patterns ...string) Option {
	return func(c *managerConfig) {
		c.excludePatterns = normalizePatterns(patterns)
	}
}

// ParseExcludedSkills splits a comma-separated pattern list, trimming
// whitespace and dropping empty entries.
func ParseExcludedSkills(value string) []string {
	return normalizePatterns(strings.Split(value, ","))
}

// ExcludedSkillsFromEnv returns the patterns configured in
// SKILLSERVER_EXCLUDE_SKILLS (nil when unset or empty).
func ExcludedSkillsFromEnv() []string {
	return ParseExcludedSkills(os.Getenv(ExcludeSkillsEnv))
}

func normalizePatterns(patterns []string) []string {
	var out []string
	for _, p := range patterns {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func validateExcludePatterns(patterns []string) error {
	for _, p := range patterns {
		if _, err := path.Match(p, ""); err != nil {
			return fmt.Errorf("invalid skill exclude pattern %q: %w", p, err)
		}
	}
	return nil
}

// IsExcluded reports whether the skill with the given name is excluded.
func (m *FileSystemManager) IsExcluded(name string) bool {
	for _, p := range m.excludePatterns {
		if ok, _ := path.Match(p, name); ok {
			return true
		}
	}
	return false
}

// ExcludedSkills returns the configured exclude patterns.
func (m *FileSystemManager) ExcludedSkills() []string {
	return append([]string(nil), m.excludePatterns...)
}
