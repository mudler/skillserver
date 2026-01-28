package domain

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExportSkill creates a tar.gz archive containing the skill directory
// Returns the archive data as bytes
func ExportSkill(skillID string, skillsDir string) ([]byte, error) {
	// Get the skill path
	var skillPath string
	if strings.Contains(skillID, "/") {
		// Git repo skill
		parts := strings.Split(skillID, "/")
		if len(parts) == 2 {
			repoName := parts[0]
			skillDirName := parts[1]
			repoPath := filepath.Join(skillsDir, repoName)
			
			// Find skill directory within repo
			var err error
			skillPath, err = findSkillDirByName(repoPath, skillDirName)
			if err != nil {
				return nil, fmt.Errorf("skill not found: %s", skillID)
			}
		} else {
			return nil, fmt.Errorf("invalid skill ID format: %s", skillID)
		}
	} else {
		// Local skill
		skillPath = filepath.Join(skillsDir, skillID)
		skillMdPath := filepath.Join(skillPath, "SKILL.md")
		if _, err := os.Stat(skillMdPath); err != nil {
			return nil, fmt.Errorf("skill not found: %s", skillID)
		}
	}

	// Get skill name (directory name)
	skillName := filepath.Base(skillPath)

	// Create a buffer to write the archive
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	// Walk the skill directory and add files to archive
	err := filepath.Walk(skillPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the directory itself, but include its contents
		if path == skillPath {
			return nil
		}

		// Get relative path from skill directory
		relPath, err := filepath.Rel(skillPath, path)
		if err != nil {
			return err
		}

		// Create archive path: skill-name/relative-path
		archivePath := filepath.Join(skillName, relPath)
		// Normalize path separators for tar format
		archivePath = filepath.ToSlash(archivePath)

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = archivePath

		// Write header
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// If it's a directory, we're done
		if info.IsDir() {
			return nil
		}

		// Write file content
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(tw, file)
		return err
	})

	if err != nil {
		tw.Close()
		gzw.Close()
		return nil, fmt.Errorf("failed to create archive: %w", err)
	}

	// Close tar writer
	if err := tw.Close(); err != nil {
		gzw.Close()
		return nil, fmt.Errorf("failed to close tar writer: %w", err)
	}

	// Close gzip writer
	if err := gzw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// ImportSkill extracts a tar.gz archive and imports the skill
// Returns the skill name if successful
func ImportSkill(archiveData []byte, skillsDir string) (string, error) {
	// Create a reader from the archive data
	r := bytes.NewReader(archiveData)
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tarReader := tar.NewReader(gzr)

	var skillName string
	var skillDir string
	var hasSkillMd bool

	// First pass: validate archive structure and find skill name
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read tar header: %w", err)
		}

		// Extract skill name from first entry
		if skillName == "" {
			parts := strings.Split(header.Name, "/")
			if len(parts) > 0 {
				skillName = parts[0]
				// Validate skill name
				if err := ValidateSkillName(skillName); err != nil {
					return "", fmt.Errorf("invalid skill name in archive: %w", err)
				}
				skillDir = filepath.Join(skillsDir, skillName)
			}
		}

		// Check for SKILL.md
		if strings.HasSuffix(header.Name, "SKILL.md") {
			hasSkillMd = true
		}

		// Validate path to prevent directory traversal
		if strings.Contains(header.Name, "..") {
			return "", fmt.Errorf("invalid path in archive: %s", header.Name)
		}
	}

	if skillName == "" {
		return "", fmt.Errorf("archive does not contain a skill directory")
	}

	if !hasSkillMd {
		return "", fmt.Errorf("archive does not contain SKILL.md file")
	}

	// Check if skill already exists
	existingSkillPath := filepath.Join(skillsDir, skillName)
	if _, err := os.Stat(existingSkillPath); err == nil {
		return "", fmt.Errorf("skill '%s' already exists", skillName)
	}

	// Reset reader for second pass
	r = bytes.NewReader(archiveData)
	gzr, err = gzip.NewReader(r)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tarReader = tar.NewReader(gzr)

	// Second pass: extract files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read tar header: %w", err)
		}

		// Get relative path from skill name
		parts := strings.Split(header.Name, "/")
		if len(parts) < 2 {
			continue // Skip root directory entry
		}
		relPath := strings.Join(parts[1:], string(filepath.Separator))
		targetPath := filepath.Join(skillsDir, skillName, relPath)

		// Validate path to prevent directory traversal
		if strings.Contains(relPath, "..") {
			return "", fmt.Errorf("invalid path in archive: %s", header.Name)
		}

		// Ensure target is within skills directory
		absTarget, err := filepath.Abs(targetPath)
		if err != nil {
			return "", fmt.Errorf("failed to get absolute path: %w", err)
		}
		absSkillsDir, err := filepath.Abs(skillsDir)
		if err != nil {
			return "", fmt.Errorf("failed to get absolute skills dir: %w", err)
		}
		if !strings.HasPrefix(absTarget, absSkillsDir) {
			return "", fmt.Errorf("invalid path: outside skills directory")
		}

		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return "", fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			// Create parent directories
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return "", fmt.Errorf("failed to create parent directory: %w", err)
			}

			// Create file
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return "", fmt.Errorf("failed to create file: %w", err)
			}

			// Copy file content
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return "", fmt.Errorf("failed to write file: %w", err)
			}
			outFile.Close()
		}
	}

	// Validate the imported skill
	skillMdPath := filepath.Join(skillDir, "SKILL.md")
	content, err := os.ReadFile(skillMdPath)
	if err != nil {
		os.RemoveAll(skillDir) // Clean up on error
		return "", fmt.Errorf("failed to read SKILL.md: %w", err)
	}

	metadata, _, err := ParseFrontmatter(string(content))
	if err != nil {
		os.RemoveAll(skillDir) // Clean up on error
		return "", fmt.Errorf("failed to parse SKILL.md: %w", err)
	}

	// Validate that name in frontmatter matches directory name
	if metadata.Name != skillName {
		os.RemoveAll(skillDir) // Clean up on error
		return "", fmt.Errorf("skill name in frontmatter (%s) does not match directory name (%s)", metadata.Name, skillName)
	}

	return skillName, nil
}

// findSkillDirByName recursively finds a skill directory by name within a base path
func findSkillDirByName(basePath, targetName string) (string, error) {
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
