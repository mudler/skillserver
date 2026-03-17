package domain

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// FlexibleStringList is a type that can unmarshal from either a YAML string
// (space-delimited) or a YAML list of strings, storing the result as a
// space-delimited string for backward compatibility.
type FlexibleStringList string

func (f FlexibleStringList) String() string {
	return string(f)
}

// UnmarshalYAML implements custom YAML unmarshaling for FlexibleStringList
func (f *FlexibleStringList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		// Plain string: "Bash Read Write"
		*f = FlexibleStringList(value.Value)
		return nil
	case yaml.SequenceNode:
		// List: ["Bash", "Read", "Write"]
		var items []string
		for _, item := range value.Content {
			if item.Kind == yaml.ScalarNode {
				items = append(items, item.Value)
			}
		}
		*f = FlexibleStringList(strings.Join(items, " "))
		return nil
	default:
		return fmt.Errorf("allowed-tools must be a string or list of strings")
	}
}

// SkillMetadata represents YAML frontmatter metadata per Agent Skills specification
type SkillMetadata struct {
	Name          string             `yaml:"name"`        // Required, 1-64 chars, lowercase alphanumeric + hyphens
	Description   string             `yaml:"description"` // Required
	License       string             `yaml:"license,omitempty"`
	Compatibility string             `yaml:"compatibility,omitempty"` // Max 500 chars
	Metadata      map[string]string  `yaml:"metadata,omitempty"`
	AllowedTools  FlexibleStringList `yaml:"allowed-tools,omitempty"` // Space-delimited string or YAML list
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

	// First try strict parsing into SkillMetadata
	var metadata SkillMetadata
	if err := yaml.Unmarshal([]byte(frontmatter), &metadata); err != nil {
		// If strict parsing fails, try a lenient approach:
		// parse only the known fields we need (name, description) from a generic map
		var raw map[string]interface{}
		if yamlErr := yaml.Unmarshal([]byte(frontmatter), &raw); yamlErr != nil {
			return nil, content, fmt.Errorf("failed to parse frontmatter: %w", yamlErr)
		}
		if name, ok := raw["name"].(string); ok {
			metadata.Name = name
		}
		if desc, ok := raw["description"].(string); ok {
			metadata.Description = desc
		}
		if license, ok := raw["license"].(string); ok {
			metadata.License = license
		}
		if compat, ok := raw["compatibility"].(string); ok {
			metadata.Compatibility = compat
		}
		// allowed-tools handled best-effort: skip if format is unrecognized
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

	return &metadata, remaining, nil
}
