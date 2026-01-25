package domain

import (
	"fmt"
	"mime"
	"path/filepath"
	"strings"
	"time"
)

// ResourceType represents the type of resource directory
type ResourceType string

const (
	ResourceTypeScript    ResourceType = "script"
	ResourceTypeReference ResourceType = "reference"
	ResourceTypeAsset     ResourceType = "asset"
)

// SkillResource represents a resource file in a skill
type SkillResource struct {
	Type     ResourceType
	Path     string    // Relative path from skill root (e.g., "scripts/script.py")
	Name     string    // Filename only
	Size     int64     // File size in bytes
	MimeType string    // MIME type
	Readable bool      // true if text file, false if binary
	Modified time.Time // Last modification time
}

// ResourceContent represents the content of a resource
type ResourceContent struct {
	Content  string // UTF-8 for text, base64 for binary
	Encoding string // "utf-8" or "base64"
	MimeType string
	Size     int64
}

// ValidateResourcePath validates a resource path
func ValidateResourcePath(path string) error {
	// Normalize path separators
	path = filepath.ToSlash(path)

	// Must start with one of the resource directory names
	validPrefixes := []string{"scripts/", "references/", "assets/"}
	valid := false
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(path, prefix) {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("resource path must start with scripts/, references/, or assets/")
	}

	// Check for path traversal
	if strings.Contains(path, "..") {
		return fmt.Errorf("resource path cannot contain '..'")
	}

	// Check for absolute paths
	if filepath.IsAbs(path) {
		return fmt.Errorf("resource path must be relative")
	}

	return nil
}

// GetResourceType determines the resource type from a path
func GetResourceType(path string) ResourceType {
	path = filepath.ToSlash(path)
	if strings.HasPrefix(path, "scripts/") {
		return ResourceTypeScript
	}
	if strings.HasPrefix(path, "references/") {
		return ResourceTypeReference
	}
	if strings.HasPrefix(path, "assets/") {
		return ResourceTypeAsset
	}
	return ResourceTypeAsset // Default fallback
}

// IsTextFile determines if a file is text-based based on MIME type
func IsTextFile(mimeType string) bool {
	if strings.HasPrefix(mimeType, "text/") {
		return true
	}
	// Common text-based MIME types
	textMimeTypes := []string{
		"application/json",
		"application/xml",
		"application/javascript",
		"application/x-sh",
		"application/x-python",
		"application/x-yaml",
		"application/x-toml",
	}
	for _, t := range textMimeTypes {
		if mimeType == t {
			return true
		}
	}
	return false
}

// DetectMimeType detects MIME type from file extension and content
func DetectMimeType(filename string, content []byte) string {
	ext := filepath.Ext(filename)
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		// Try to detect from content
		if len(content) > 0 {
			// Check for text file signatures
			if isTextContent(content) {
				// Default to text/plain for unknown text files
				return "text/plain"
			}
		}
		return "application/octet-stream"
	}
	return mimeType
}

// isTextContent checks if content appears to be text
func isTextContent(content []byte) bool {
	// Check first 512 bytes for null bytes
	checkLen := len(content)
	if checkLen > 512 {
		checkLen = 512
	}
	for i := 0; i < checkLen; i++ {
		if content[i] == 0 {
			return false
		}
	}
	return true
}
