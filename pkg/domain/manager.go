package domain

import (
	"encoding/base64"
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

	// Resource management methods
	ListSkillResources(skillID string) ([]SkillResource, error)
	ReadSkillResource(skillID, resourcePath string) (*ResourceContent, error)
	GetSkillResourceInfo(skillID, resourcePath string) (*SkillResource, error)
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

		// Check if this is from a git repo
		relPath, err := filepath.Rel(m.skillsDir, skillPath)
		if err != nil {
			continue
		}
		parts := strings.Split(relPath, string(filepath.Separator))
		isFromGitRepo := len(parts) > 0 && parts[0] != skillDir

		// If it's from a git repo, check if that repo is enabled
		if isFromGitRepo {
			repoName := parts[0]
			repoEnabled := false
			for _, enabledRepoName := range m.gitRepos {
				if enabledRepoName == repoName {
					repoEnabled = true
					break
				}
			}
			// Skip skills from disabled repos
			if !repoEnabled {
				continue
			}
		}

		isReadOnly := m.isGitRepoPath(skillPath)

		var skillName string
		if isReadOnly {
			// For git repo skills, use repoName/directoryName format
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

// UpdateGitRepos updates the list of git repository names for read-only detection
func (m *FileSystemManager) UpdateGitRepos(gitRepoNames []string) {
	m.gitRepos = gitRepoNames
}

// getSkillPath returns the full path to a skill directory given its ID
func (m *FileSystemManager) getSkillPath(skillID string) (string, error) {
	// Check if this is a git repo skill (format: repoName/skillName)
	if strings.Contains(skillID, "/") {
		parts := strings.Split(skillID, "/")
		if len(parts) == 2 {
			repoName := parts[0]
			skillDirName := parts[1]
			repoPath := filepath.Join(m.skillsDir, repoName)

			// Recursively search for the skill directory within the repo
			skillPath, err := m.findSkillDirByName(repoPath, skillDirName)
			if err != nil {
				return "", fmt.Errorf("skill not found: %s", skillID)
			}
			return skillPath, nil
		}
	}

	// Local skill
	skillPath := filepath.Join(m.skillsDir, skillID)
	skillMdPath := filepath.Join(skillPath, "SKILL.md")
	if _, err := os.Stat(skillMdPath); err != nil {
		return "", fmt.Errorf("skill not found: %s", skillID)
	}
	return skillPath, nil
}

// ListSkillResources lists all resources in a skill's optional directories
func (m *FileSystemManager) ListSkillResources(skillID string) ([]SkillResource, error) {
	skillPath, err := m.getSkillPath(skillID)
	if err != nil {
		return nil, err
	}

	var resources []SkillResource
	resourceDirs := []string{"scripts", "references", "assets"}

	for _, dir := range resourceDirs {
		dirPath := filepath.Join(skillPath, dir)
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			// Directory doesn't exist, skip
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				// Recursively list subdirectories
				subResources, err := m.listResourcesInDir(skillPath, filepath.Join(dir, entry.Name()))
				if err == nil {
					resources = append(resources, subResources...)
				}
				continue
			}

			resourcePath := filepath.Join(dir, entry.Name())
			fullPath := filepath.Join(skillPath, resourcePath)

			info, err := entry.Info()
			if err != nil {
				continue
			}

			// Read file to detect MIME type
			content, err := os.ReadFile(fullPath)
			if err != nil {
				continue
			}

			mimeType := DetectMimeType(entry.Name(), content)
			readable := IsTextFile(mimeType)

			resources = append(resources, SkillResource{
				Type:     GetResourceType(resourcePath),
				Path:     filepath.ToSlash(resourcePath), // Use forward slashes for consistency
				Name:     entry.Name(),
				Size:     info.Size(),
				MimeType: mimeType,
				Readable: readable,
				Modified: info.ModTime(),
			})
		}
	}

	return resources, nil
}

// listResourcesInDir recursively lists resources in a subdirectory
func (m *FileSystemManager) listResourcesInDir(skillPath, relPath string) ([]SkillResource, error) {
	var resources []SkillResource
	fullPath := filepath.Join(skillPath, relPath)

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Recursively list subdirectories
			subResources, err := m.listResourcesInDir(skillPath, filepath.Join(relPath, entry.Name()))
			if err == nil {
				resources = append(resources, subResources...)
			}
			continue
		}

		resourcePath := filepath.Join(relPath, entry.Name())
		fullResourcePath := filepath.Join(skillPath, resourcePath)

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Read file to detect MIME type
		content, err := os.ReadFile(fullResourcePath)
		if err != nil {
			continue
		}

		mimeType := DetectMimeType(entry.Name(), content)
		readable := IsTextFile(mimeType)

		resources = append(resources, SkillResource{
			Type:     GetResourceType(resourcePath),
			Path:     filepath.ToSlash(resourcePath),
			Name:     entry.Name(),
			Size:     info.Size(),
			MimeType: mimeType,
			Readable: readable,
			Modified: info.ModTime(),
		})
	}

	return resources, nil
}

// ReadSkillResource reads the content of a skill resource file
func (m *FileSystemManager) ReadSkillResource(skillID, resourcePath string) (*ResourceContent, error) {
	// Validate path
	if err := ValidateResourcePath(resourcePath); err != nil {
		return nil, err
	}

	skillPath, err := m.getSkillPath(skillID)
	if err != nil {
		return nil, err
	}

	fullPath := filepath.Join(skillPath, resourcePath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read resource: %w", err)
	}

	mimeType := DetectMimeType(filepath.Base(resourcePath), content)
	readable := IsTextFile(mimeType)

	var encoding string
	var contentStr string

	if readable {
		// Try to decode as UTF-8
		contentStr = string(content)
		encoding = "utf-8"
	} else {
		// Encode as base64 for binary files
		contentStr = base64.StdEncoding.EncodeToString(content)
		encoding = "base64"
	}

	return &ResourceContent{
		Content:  contentStr,
		Encoding: encoding,
		MimeType: mimeType,
		Size:     int64(len(content)),
	}, nil
}

// GetSkillResourceInfo gets metadata about a specific resource without reading content
func (m *FileSystemManager) GetSkillResourceInfo(skillID, resourcePath string) (*SkillResource, error) {
	// Validate path
	if err := ValidateResourcePath(resourcePath); err != nil {
		return nil, err
	}

	skillPath, err := m.getSkillPath(skillID)
	if err != nil {
		return nil, err
	}

	fullPath := filepath.Join(skillPath, resourcePath)
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("resource not found: %w", err)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("resource path points to a directory, not a file")
	}

	// Read a small portion to detect MIME type
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open resource: %w", err)
	}
	defer file.Close()

	buffer := make([]byte, 512)
	n, _ := file.Read(buffer)
	mimeType := DetectMimeType(filepath.Base(resourcePath), buffer[:n])
	readable := IsTextFile(mimeType)

	return &SkillResource{
		Type:     GetResourceType(resourcePath),
		Path:     filepath.ToSlash(resourcePath),
		Name:     filepath.Base(resourcePath),
		Size:     info.Size(),
		MimeType: mimeType,
		Readable: readable,
		Modified: info.ModTime(),
	}, nil
}
