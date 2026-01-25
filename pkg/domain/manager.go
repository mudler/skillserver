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
	gitRepos  []string // List of git repo directory names (for read-only detection)
}

// NewFileSystemManager creates a new FileSystemManager
func NewFileSystemManager(skillsDir string, gitRepos []string) (*FileSystemManager, error) {
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
		gitRepos:  gitRepos,
	}

	// Initial index build
	if err := manager.RebuildIndex(); err != nil {
		return nil, fmt.Errorf("failed to build initial index: %w", err)
	}

	return manager, nil
}

// isGitRepoPath checks if a path is within a git repository directory
func (m *FileSystemManager) isGitRepoPath(path string) bool {
	relPath, err := filepath.Rel(m.skillsDir, path)
	if err != nil {
		return false
	}

	// Check if path starts with any git repo name
	parts := strings.Split(relPath, string(filepath.Separator))
	if len(parts) > 0 {
		for _, repoName := range m.gitRepos {
			if parts[0] == repoName {
				return true
			}
		}
	}
	return false
}

// findSkillDirs recursively finds all directories containing SKILL.md files
func (m *FileSystemManager) findSkillDirs(root string, basePath string) ([]string, error) {
	var skillDirs []string

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		entryPath := filepath.Join(root, entry.Name())

		// Check if this directory contains SKILL.md
		skillMdPath := filepath.Join(entryPath, "SKILL.md")
		if _, err := os.Stat(skillMdPath); err == nil {
			// Found a skill directory
			relPath, _ := filepath.Rel(basePath, entryPath)
			skillDirs = append(skillDirs, relPath)
		}

		// Recursively search subdirectories (for git repos)
		subDirs, err := m.findSkillDirs(entryPath, basePath)
		if err == nil {
			skillDirs = append(skillDirs, subDirs...)
		}
	}

	return skillDirs, nil
}

// ListSkills returns all skills (local and from git repos)
func (m *FileSystemManager) ListSkills() ([]Skill, error) {
	var skills []Skill

	// Find all directories containing SKILL.md
	skillDirs, err := m.findSkillDirs(m.skillsDir, m.skillsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find skill directories: %w", err)
	}

	for _, skillDir := range skillDirs {
		// Determine skill name and read-only status
		skillPath := filepath.Join(m.skillsDir, skillDir)
		isReadOnly := m.isGitRepoPath(skillPath)

		var skillName string
		if isReadOnly {
			// For git repo skills, use repoName/directoryName format
			parts := strings.Split(skillDir, string(filepath.Separator))
			if len(parts) >= 2 {
				// Extract repo name and skill directory name
				repoName := parts[0]
				skillDirName := parts[len(parts)-1]
				skillName = fmt.Sprintf("%s/%s", repoName, skillDirName)
			} else {
				skillName = skillDir
			}
		} else {
			// For local skills, use directory name
			skillName = filepath.Base(skillDir)
		}

		skill, err := m.readSkillFromPath(skillPath, skillName, isReadOnly)
		if err != nil {
			// Skip skills that can't be read
			continue
		}
		skills = append(skills, *skill)
	}

	return skills, nil
}

// readSkillFromPath reads a skill from a directory path
func (m *FileSystemManager) readSkillFromPath(skillPath, skillName string, isReadOnly bool) (*Skill, error) {
	skillMdPath := filepath.Join(skillPath, "SKILL.md")
	content, err := os.ReadFile(skillMdPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read SKILL.md: %w", err)
	}

	metadata, contentStr, err := ParseFrontmatter(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	// Validate that name in frontmatter matches directory name
	dirName := filepath.Base(skillPath)
	if metadata.Name != dirName {
		return nil, fmt.Errorf("skill name in frontmatter (%s) does not match directory name (%s)", metadata.Name, dirName)
	}

	return &Skill{
		Name:       skillName,
		ID:         skillName, // ID is the same as Name - the identifier to use when reading
		Content:    contentStr,
		Metadata:   metadata,
		SourcePath: skillPath,
		ReadOnly:   isReadOnly,
	}, nil
}

// findSkillDirByName recursively finds a skill directory by name within a base path
func (m *FileSystemManager) findSkillDirByName(basePath, targetName string) (string, error) {
	var foundPath string
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}
		if !info.IsDir() {
			return nil
		}
		// Check if this directory contains SKILL.md and matches the target name
		skillMdPath := filepath.Join(path, "SKILL.md")
		if _, err := os.Stat(skillMdPath); err == nil {
			dirName := filepath.Base(path)
			if dirName == targetName {
				foundPath = path
				return filepath.SkipAll // Found it, stop walking
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if foundPath == "" {
		return "", fmt.Errorf("skill directory not found: %s", targetName)
	}
	return foundPath, nil
}

// ReadSkill reads a skill by name (supports both local skills and git repo skills with repoName/skillName format)
func (m *FileSystemManager) ReadSkill(name string) (*Skill, error) {
	// Check if this is a git repo skill (format: repoName/skillName)
	if strings.Contains(name, "/") {
		parts := strings.Split(name, "/")
		if len(parts) == 2 {
			repoName := parts[0]
			skillDirName := parts[1]
			repoPath := filepath.Join(m.skillsDir, repoName)

			// Check if repo directory exists
			if _, err := os.Stat(repoPath); err != nil {
				return nil, fmt.Errorf("skill not found: %s", name)
			}

			// Recursively search for the skill directory within the repo
			skillPath, err := m.findSkillDirByName(repoPath, skillDirName)
			if err != nil {
				return nil, fmt.Errorf("skill not found: %s", name)
			}

			return m.readSkillFromPath(skillPath, name, true)
		}
	}

	// Local skill - look for directory with this name
	skillPath := filepath.Join(m.skillsDir, name)
	skillMdPath := filepath.Join(skillPath, "SKILL.md")

	if _, err := os.Stat(skillMdPath); err != nil {
		return nil, fmt.Errorf("skill not found: %s", name)
	}

	return m.readSkillFromPath(skillPath, name, false)
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
