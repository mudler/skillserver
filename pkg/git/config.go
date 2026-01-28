package git

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GitRepoConfig represents a git repository configuration
type GitRepoConfig struct {
	ID      string `json:"id"`
	URL     string `json:"url"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// ConfigManager manages git repository configurations
type ConfigManager struct {
	configPath string
}

// NewConfigManager creates a new ConfigManager
func NewConfigManager(skillsDir string) *ConfigManager {
	return &ConfigManager{
		configPath: filepath.Join(skillsDir, ".git-repos.json"),
	}
}

// LoadConfig loads git repository configurations from the config file
func (cm *ConfigManager) LoadConfig() ([]GitRepoConfig, error) {
	// If config file doesn't exist, return empty slice
	if _, err := os.Stat(cm.configPath); os.IsNotExist(err) {
		return []GitRepoConfig{}, nil
	}

	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var repos []GitRepoConfig
	if len(data) == 0 {
		return []GitRepoConfig{}, nil
	}

	if err := json.Unmarshal(data, &repos); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return repos, nil
}

// SaveConfig saves git repository configurations to the config file
func (cm *ConfigManager) SaveConfig(repos []GitRepoConfig) error {
	// Ensure directory exists
	dir := filepath.Dir(cm.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(repos, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(cm.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// ExtractRepoName extracts a repository name from a URL
func ExtractRepoName(repoURL string) string {
	// Remove protocol and .git suffix
	name := strings.TrimSuffix(repoURL, ".git")

	// Extract last part of path
	parts := strings.Split(name, "/")
	if len(parts) > 0 {
		name = parts[len(parts)-1]
	}

	// Remove protocol prefix if present
	if strings.Contains(name, "://") {
		parts = strings.Split(name, "://")
		if len(parts) > 1 {
			parts = strings.Split(parts[1], "/")
			if len(parts) > 0 {
				name = parts[len(parts)-1]
			}
		}
	}

	return name
}

// GenerateID generates a unique ID for a git repo config
func GenerateID(repoURL string) string {
	// Use a simple hash-like approach: take first 8 chars of URL hash
	// In a real implementation, you might want to use a proper hash function
	name := ExtractRepoName(repoURL)
	return strings.ToLower(strings.ReplaceAll(name, "-", ""))
}
