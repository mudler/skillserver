package web

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/mudler/skillserver/pkg/domain"
)

// SkillResponse represents a skill in API responses
type SkillResponse struct {
	Name        string   `json:"name"`
	Content     string   `json:"content"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// CreateSkillRequest represents a request to create a skill
type CreateSkillRequest struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// UpdateSkillRequest represents a request to update a skill
type UpdateSkillRequest struct {
	Content string `json:"content"`
}

// listSkills lists all skills
func (s *Server) listSkills(c echo.Context) error {
	skills, err := s.skillManager.ListSkills()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	responses := make([]SkillResponse, len(skills))
	for i, skill := range skills {
		responses[i] = SkillResponse{
			Name:    skill.Name,
			Content: skill.Content,
		}
		if skill.Metadata != nil {
			responses[i].Description = skill.Metadata.Description
			responses[i].Tags = skill.Metadata.Tags
		}
	}

	return c.JSON(http.StatusOK, responses)
}

// getSkill gets a single skill by name
func (s *Server) getSkill(c echo.Context) error {
	name := c.Param("name")
	skill, err := s.skillManager.ReadSkill(name)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "skill not found",
		})
	}

	response := SkillResponse{
		Name:    skill.Name,
		Content: skill.Content,
	}
	if skill.Metadata != nil {
		response.Description = skill.Metadata.Description
		response.Tags = skill.Metadata.Tags
	}

	return c.JSON(http.StatusOK, response)
}

// createSkill creates a new skill
func (s *Server) createSkill(c echo.Context) error {
	var req CreateSkillRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request",
		})
	}

	// Validate name
	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "name is required",
		})
	}

	// Sanitize name (remove .md extension if present, remove path separators)
	name := strings.TrimSuffix(req.Name, ".md")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")

	// Get the skills directory from the manager
	// We need to access the underlying FileSystemManager
	fsManager, ok := s.skillManager.(*domain.FileSystemManager)
	if !ok {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "unsupported manager type",
		})
	}

	// Write the file
	skillsDir := fsManager.GetSkillsDir()
	filename := filepath.Join(skillsDir, name+".md")
	if err := writeFile(filename, req.Content); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	// Rebuild index
	if err := s.skillManager.RebuildIndex(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to rebuild index",
		})
	}

	skill, err := s.skillManager.ReadSkill(name)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to read created skill",
		})
	}

	response := SkillResponse{
		Name:    skill.Name,
		Content: skill.Content,
	}
	if skill.Metadata != nil {
		response.Description = skill.Metadata.Description
		response.Tags = skill.Metadata.Tags
	}

	return c.JSON(http.StatusCreated, response)
}

// updateSkill updates an existing skill
func (s *Server) updateSkill(c echo.Context) error {
	name := c.Param("name")
	var req UpdateSkillRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request",
		})
	}

	// Get the skills directory from the manager
	fsManager, ok := s.skillManager.(*domain.FileSystemManager)
	if !ok {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "unsupported manager type",
		})
	}

	// Write the file
	skillsDir := fsManager.GetSkillsDir()
	filename := filepath.Join(skillsDir, name+".md")
	if err := writeFile(filename, req.Content); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	// Rebuild index
	if err := s.skillManager.RebuildIndex(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to rebuild index",
		})
	}

	skill, err := s.skillManager.ReadSkill(name)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to read updated skill",
		})
	}

	response := SkillResponse{
		Name:    skill.Name,
		Content: skill.Content,
	}
	if skill.Metadata != nil {
		response.Description = skill.Metadata.Description
		response.Tags = skill.Metadata.Tags
	}

	return c.JSON(http.StatusOK, response)
}

// deleteSkill deletes a skill
func (s *Server) deleteSkill(c echo.Context) error {
	name := c.Param("name")

	// Get the skills directory from the manager
	fsManager, ok := s.skillManager.(*domain.FileSystemManager)
	if !ok {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "unsupported manager type",
		})
	}

	// Delete the file
	skillsDir := fsManager.GetSkillsDir()
	filename := filepath.Join(skillsDir, name+".md")
	if err := deleteFile(filename); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	// Rebuild index
	if err := s.skillManager.RebuildIndex(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to rebuild index",
		})
	}

	return c.NoContent(http.StatusNoContent)
}

// searchSkills searches for skills
func (s *Server) searchSkills(c echo.Context) error {
	query := c.QueryParam("q")
	if query == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "query parameter 'q' is required",
		})
	}

	skills, err := s.skillManager.SearchSkills(query)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	responses := make([]SkillResponse, len(skills))
	for i, skill := range skills {
		responses[i] = SkillResponse{
			Name:    skill.Name,
			Content: skill.Content,
		}
		if skill.Metadata != nil {
			responses[i].Description = skill.Metadata.Description
			responses[i].Tags = skill.Metadata.Tags
		}
	}

	return c.JSON(http.StatusOK, responses)
}

// getShareURL generates a URL for sharing a skill on Git providers
func (s *Server) getShareURL(c echo.Context) error {
	name := c.Param("name")

	// Try to determine the Git provider from the first repo URL
	if len(s.gitRepos) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "no git repositories configured",
		})
	}

	repoURL := s.gitRepos[0]
	shareURL := generateShareURL(repoURL, name)

	return c.JSON(http.StatusOK, map[string]string{
		"url": shareURL,
	})
}

// Helper functions

func writeFile(filename, content string) error {
	return os.WriteFile(filename, []byte(content), 0644)
}

func deleteFile(filename string) error {
	return os.Remove(filename)
}

func generateShareURL(repoURL, skillName string) string {
	// Parse the repo URL to determine provider
	u, err := url.Parse(repoURL)
	if err != nil {
		// If parsing fails, return a generic GitHub URL
		return "https://github.com"
	}

	// Extract owner and repo from path
	path := strings.TrimPrefix(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		return "https://github.com"
	}

	owner := parts[0]
	repo := parts[1]

	// Generate URL based on provider
	switch u.Hostname() {
	case "github.com":
		// GitHub: create new file URL
		return fmt.Sprintf("https://github.com/%s/%s/new/main?filename=skills/%s.md", owner, repo, skillName)
	case "gitlab.com":
		// GitLab: create new file URL
		return fmt.Sprintf("https://gitlab.com/%s/%s/-/new/main?file_name=skills/%s.md", owner, repo, skillName)
	default:
		// Generic fallback
		return fmt.Sprintf("%s://%s/%s/%s", u.Scheme, u.Hostname(), owner, repo)
	}
}
