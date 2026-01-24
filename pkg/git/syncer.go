package git

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

// GitSyncer handles synchronization with Git repositories
type GitSyncer struct {
	skillsDir string
	repos     []string
	ctx       context.Context
	cancel    context.CancelFunc
	onUpdate  func() error // Callback to trigger re-indexing
	progress  io.Writer    // Writer for git progress output (nil = disabled)
	logger    io.Writer    // Writer for log messages (nil = disabled)
}

// NewGitSyncer creates a new GitSyncer
func NewGitSyncer(skillsDir string, repos []string, onUpdate func() error) *GitSyncer {
	ctx, cancel := context.WithCancel(context.Background())
	return &GitSyncer{
		skillsDir: skillsDir,
		repos:     repos,
		ctx:       ctx,
		cancel:    cancel,
		onUpdate:  onUpdate,
		progress:  nil, // Default to no progress output (to avoid interfering with MCP stdio)
		logger:    nil, // Default to no logging
	}
}

// SetProgressWriter sets the writer for git progress output
func (g *GitSyncer) SetProgressWriter(w io.Writer) {
	g.progress = w
}

// SetLogger sets the writer for log messages
func (g *GitSyncer) SetLogger(w io.Writer) {
	g.logger = w
}

// Start begins the Git synchronization process
func (g *GitSyncer) Start() error {
	// Initial sync
	if err := g.syncAll(); err != nil {
		return fmt.Errorf("initial sync failed: %w", err)
	}

	// Start periodic sync in background
	go g.periodicSync()

	return nil
}

// Stop stops the Git synchronization
func (g *GitSyncer) Stop() {
	g.cancel()
}

// syncAll syncs all configured repositories
func (g *GitSyncer) syncAll() error {
	for _, repoURL := range g.repos {
		if err := g.syncRepo(repoURL); err != nil {
			// Log error but continue with other repos (only if logger is set)
			if g.logger != nil {
				fmt.Fprintf(g.logger, "Warning: failed to sync repo %s: %v\n", repoURL, err)
			}
		}
	}

	// Trigger re-indexing if callback is set
	if g.onUpdate != nil {
		if err := g.onUpdate(); err != nil {
			return fmt.Errorf("failed to trigger re-indexing: %w", err)
		}
	}

	return nil
}

// syncRepo syncs a single repository
func (g *GitSyncer) syncRepo(repoURL string) error {
	// Extract repo name from URL
	repoName := g.extractRepoName(repoURL)
	targetDir := filepath.Join(g.skillsDir, repoName)

	// Check if directory exists
	_, err := os.Stat(targetDir)
	if os.IsNotExist(err) {
		// Clone the repository
		return g.cloneRepo(repoURL, targetDir)
	} else if err != nil {
		return fmt.Errorf("failed to check directory: %w", err)
	}

	// Pull updates
	return g.pullRepo(targetDir)
}

// cloneRepo clones a repository
func (g *GitSyncer) cloneRepo(repoURL, targetDir string) error {
	_, err := git.PlainClone(targetDir, false, &git.CloneOptions{
		URL:      repoURL,
		Progress: g.progress, // Use progress writer (nil = no output)
	})
	if err != nil {
		// Handle authentication errors gracefully
		if err == transport.ErrAuthenticationRequired {
			return fmt.Errorf("authentication required for %s", repoURL)
		}
		return fmt.Errorf("failed to clone repository: %w", err)
	}
	return nil
}

// pullRepo pulls updates from a repository
func (g *GitSyncer) pullRepo(repoDir string) error {
	r, err := git.PlainOpen(repoDir)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	w, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	err = w.Pull(&git.PullOptions{
		Progress: g.progress, // Use progress writer (nil = no output)
	})
	if err != nil {
		if err == git.NoErrAlreadyUpToDate {
			// Not an error, just already up to date
			return nil
		}
		if err == transport.ErrAuthenticationRequired {
			return fmt.Errorf("authentication required")
		}
		return fmt.Errorf("failed to pull: %w", err)
	}

	return nil
}

// extractRepoName extracts a repository name from a URL
func (g *GitSyncer) extractRepoName(repoURL string) string {
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

// periodicSync runs periodic synchronization every 5 minutes
func (g *GitSyncer) periodicSync() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			if err := g.syncAll(); err != nil {
				fmt.Printf("Warning: periodic sync failed: %v\n", err)
			}
		}
	}
}
