package domain

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SkillManager defines the interface for managing skills
type SkillManager interface {
	ListSkills() ([]Skill, error)
	ReadSkill(name string) (*Skill, error)
	SearchSkills(query string) ([]Skill, error)
	RebuildIndex() error
}

// FileSystemManager implements SkillManager using the file system
type FileSystemManager struct {
	skillsDir string
	searcher  *Searcher
}

// NewFileSystemManager creates a new FileSystemManager
func NewFileSystemManager(skillsDir string) (*FileSystemManager, error) {
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create skills directory: %w", err)
	}

	searcher, err := NewSearcher(skillsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create searcher: %w", err)
	}

	manager := &FileSystemManager{
		skillsDir: skillsDir,
		searcher:  searcher,
	}

	// Initial index build
	if err := manager.RebuildIndex(); err != nil {
		return nil, fmt.Errorf("failed to build initial index: %w", err)
	}

	return manager, nil
}

// ListSkills returns all skills in the directory
func (m *FileSystemManager) ListSkills() ([]Skill, error) {
	entries, err := os.ReadDir(m.skillsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read skills directory: %w", err)
	}

	var skills []Skill
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".md")
		skill, err := m.ReadSkill(name)
		if err != nil {
			// Skip files that can't be read
			continue
		}
		skills = append(skills, *skill)
	}

	return skills, nil
}

// ReadSkill reads a skill by name (without .md extension)
func (m *FileSystemManager) ReadSkill(name string) (*Skill, error) {
	filename := filepath.Join(m.skillsDir, name+".md")
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read skill file: %w", err)
	}

	metadata, contentStr, err := ParseFrontmatter(string(content))
	if err != nil {
		return nil, err
	}

	return &Skill{
		Name:     name,
		Content:  contentStr,
		Metadata: metadata,
	}, nil
}

// SearchSkills searches for skills matching the query
func (m *FileSystemManager) SearchSkills(query string) ([]Skill, error) {
	results, err := m.searcher.Search(query)
	if err != nil {
		return nil, err
	}

	// Read full skill content for each result
	var skills []Skill
	for _, result := range results {
		skill, err := m.ReadSkill(result.Name)
		if err != nil {
			// Skip skills that can't be read
			continue
		}
		skills = append(skills, *skill)
	}

	return skills, nil
}

// RebuildIndex rebuilds the search index
func (m *FileSystemManager) RebuildIndex() error {
	skills, err := m.ListSkills()
	if err != nil {
		return err
	}

	return m.searcher.IndexSkills(skills)
}

// GetSkillsDir returns the skills directory path
func (m *FileSystemManager) GetSkillsDir() string {
	return m.skillsDir
}
