package domain

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// SkillMetadata represents optional YAML frontmatter metadata
type SkillMetadata struct {
	Tags        []string `yaml:"tags"`
	Description string   `yaml:"description"`
}

// Skill represents a markdown skill file
type Skill struct {
	Name     string
	Content  string
	Metadata *SkillMetadata
}

// ParseFrontmatter extracts YAML frontmatter from markdown content
// Returns the metadata (if present) and the remaining content
func ParseFrontmatter(content string) (*SkillMetadata, string, error) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return nil, content, nil
	}

	// Find the end of frontmatter
	endIdx := strings.Index(content[3:], "---")
	if endIdx == -1 {
		return nil, content, nil // Malformed frontmatter, treat as regular content
	}

	frontmatter := content[3 : endIdx+3]
	remaining := strings.TrimSpace(content[endIdx+6:])

	var metadata SkillMetadata
	if err := yaml.Unmarshal([]byte(frontmatter), &metadata); err != nil {
		return nil, content, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	return &metadata, remaining, nil
}
