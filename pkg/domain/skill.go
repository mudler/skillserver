package domain

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// SkillMetadata represents YAML frontmatter metadata per Agent Skills specification
type SkillMetadata struct {
	Name          string            `yaml:"name"`        // Required, 1-64 chars, lowercase alphanumeric + hyphens
	Description   string            `yaml:"description"` // Required, 1-1024 chars
	License       string            `yaml:"license,omitempty"`
	Compatibility string            `yaml:"compatibility,omitempty"` // Max 500 chars
	Metadata      map[string]string `yaml:"metadata,omitempty"`
	AllowedTools  string            `yaml:"allowed-tools,omitempty"` // Space-delimited
}

// Skill represents a skill directory with SKILL.md file
type Skill struct {
	Name       string // Display name (same as ID for local skills, repoName/skillName for git repo skills)
	ID         string // Unique identifier to use when reading the skill (repoName/skillName or skillName)
	Content    string
	Metadata   *SkillMetadata
	SourcePath string // Full path to the skill directory
	ReadOnly   bool   // True if skill is from a git repository
}

var (
	// Valid skill name pattern: lowercase letters, numbers, hyphens, 1-64 chars, no leading/trailing hyphens, no consecutive hyphens
	skillNamePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
)

// ValidateSkillName validates a skill name according to Agent Skills specification
func ValidateSkillName(name string) error {
	if len(name) == 0 {
		return fmt.Errorf("skill name is required")
	}
	if len(name) > 64 {
		return fmt.Errorf("skill name must be 1-64 characters, got %d", len(name))
	}
	if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") {
		return fmt.Errorf("skill name cannot start or end with a hyphen")
	}
	if strings.Contains(name, "--") {
		return fmt.Errorf("skill name cannot contain consecutive hyphens")
	}
	if !skillNamePattern.MatchString(name) {
		return fmt.Errorf("skill name may only contain lowercase letters, numbers, and hyphens")
	}
	return nil
}

// ParseFrontmatter extracts YAML frontmatter from markdown content
// Returns the metadata (if present) and the remaining content
func ParseFrontmatter(content string) (*SkillMetadata, string, error) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return nil, content, fmt.Errorf("frontmatter is required (must start with ---)")
	}

	// Find the end of frontmatter
	endIdx := strings.Index(content[3:], "---")
	if endIdx == -1 {
		return nil, content, fmt.Errorf("malformed frontmatter (missing closing ---)")
	}

	frontmatter := content[3 : endIdx+3]
	remaining := strings.TrimSpace(content[endIdx+6:])

	var metadata SkillMetadata
	if err := yaml.Unmarshal([]byte(frontmatter), &metadata); err != nil {
		return nil, content, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	// Validate required fields
	if metadata.Name == "" {
		return nil, content, fmt.Errorf("frontmatter 'name' field is required")
	}
	if err := ValidateSkillName(metadata.Name); err != nil {
		return nil, content, fmt.Errorf("invalid skill name: %w", err)
	}
	if metadata.Description == "" {
		return nil, content, fmt.Errorf("frontmatter 'description' field is required")
	}
	if len(metadata.Description) > 1024 {
		return nil, content, fmt.Errorf("description must be 1-1024 characters, got %d", len(metadata.Description))
	}
	if metadata.Compatibility != "" && len(metadata.Compatibility) > 500 {
		return nil, content, fmt.Errorf("compatibility must be max 500 characters, got %d", len(metadata.Compatibility))
	}

	return &metadata, remaining, nil
}
