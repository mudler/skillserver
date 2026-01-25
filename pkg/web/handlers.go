package web

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v5"

	"github.com/mudler/skillserver/pkg/domain"
)

// SkillResponse represents a skill in API responses
type SkillResponse struct {
	Name          string            `json:"name"`
	Content       string            `json:"content"`
	Description   string            `json:"description,omitempty"`
	License       string            `json:"license,omitempty"`
	Compatibility string            `json:"compatibility,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	AllowedTools  string            `json:"allowed-tools,omitempty"`
	ReadOnly      bool              `json:"readOnly"`
}

// CreateSkillRequest represents a request to create a skill
type CreateSkillRequest struct {
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Content       string            `json:"content"`
	License       string            `json:"license,omitempty"`
	Compatibility string            `json:"compatibility,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	AllowedTools  string            `json:"allowed-tools,omitempty"`
}

// UpdateSkillRequest represents a request to update a skill
type UpdateSkillRequest struct {
	Description   string            `json:"description"`
	Content       string            `json:"content"`
	License       string            `json:"license,omitempty"`
	Compatibility string            `json:"compatibility,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	AllowedTools  string            `json:"allowed-tools,omitempty"`
}

// listSkills lists all skills
func (s *Server) listSkills(c *echo.Context) error {
	skills, err := s.skillManager.ListSkills()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	responses := make([]SkillResponse, len(skills))
	for i, skill := range skills {
		responses[i] = SkillResponse{
			Name:     skill.Name,
			Content:  skill.Content,
			ReadOnly: skill.ReadOnly,
		}
		if skill.Metadata != nil {
			responses[i].Description = skill.Metadata.Description
			responses[i].License = skill.Metadata.License
			responses[i].Compatibility = skill.Metadata.Compatibility
			responses[i].Metadata = skill.Metadata.Metadata
			responses[i].AllowedTools = skill.Metadata.AllowedTools
		}
	}

	return c.JSON(http.StatusOK, responses)
}

// getSkill gets a single skill by name
func (s *Server) getSkill(c *echo.Context) error {
	name := c.Param("name")
	skill, err := s.skillManager.ReadSkill(name)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "skill not found",
		})
	}

	response := SkillResponse{
		Name:     skill.Name,
		Content:  skill.Content,
		ReadOnly: skill.ReadOnly,
	}
	if skill.Metadata != nil {
		response.Description = skill.Metadata.Description
		response.License = skill.Metadata.License
		response.Compatibility = skill.Metadata.Compatibility
		response.Metadata = skill.Metadata.Metadata
		response.AllowedTools = skill.Metadata.AllowedTools
	}

	return c.JSON(http.StatusOK, response)
}

// createSkill creates a new skill
func (s *Server) createSkill(c *echo.Context) error {
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

	// Validate name according to Agent Skills spec
	if err := domain.ValidateSkillName(req.Name); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	// Validate description
	if req.Description == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "description is required",
		})
	}
	if len(req.Description) > 1024 {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "description must be 1-1024 characters",
		})
	}

	// Validate compatibility if provided
	if req.Compatibility != "" && len(req.Compatibility) > 500 {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "compatibility must be max 500 characters",
		})
	}

	// Get the skills directory from the manager
	fsManager, ok := s.skillManager.(*domain.FileSystemManager)
	if !ok {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "unsupported manager type",
		})
	}

	// Create skill directory
	skillsDir := fsManager.GetSkillsDir()
	skillDir := filepath.Join(skillsDir, req.Name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to create skill directory: %v", err),
		})
	}

	// Build frontmatter
	frontmatter := fmt.Sprintf("---\nname: %s\ndescription: %s\n", req.Name, req.Description)
	if req.License != "" {
		frontmatter += fmt.Sprintf("license: %s\n", req.License)
	}
	if req.Compatibility != "" {
		frontmatter += fmt.Sprintf("compatibility: %s\n", req.Compatibility)
	}
	if len(req.Metadata) > 0 {
		frontmatter += "metadata:\n"
		for k, v := range req.Metadata {
			frontmatter += fmt.Sprintf("  %s: %s\n", k, v)
		}
	}
	if req.AllowedTools != "" {
		frontmatter += fmt.Sprintf("allowed-tools: %s\n", req.AllowedTools)
	}
	frontmatter += "---\n\n"

	// Write SKILL.md file
	skillMdPath := filepath.Join(skillDir, "SKILL.md")
	fullContent := frontmatter + req.Content
	if err := writeFile(skillMdPath, fullContent); err != nil {
		os.RemoveAll(skillDir) // Clean up on error
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

	skill, err := s.skillManager.ReadSkill(req.Name)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to read created skill",
		})
	}

	response := SkillResponse{
		Name:     skill.Name,
		Content:  skill.Content,
		ReadOnly: skill.ReadOnly,
	}
	if skill.Metadata != nil {
		response.Description = skill.Metadata.Description
		response.License = skill.Metadata.License
		response.Compatibility = skill.Metadata.Compatibility
		response.Metadata = skill.Metadata.Metadata
		response.AllowedTools = skill.Metadata.AllowedTools
	}

	return c.JSON(http.StatusCreated, response)
}

// updateSkill updates an existing skill
func (s *Server) updateSkill(c *echo.Context) error {
	name := c.Param("name")

	// Check if skill exists and is read-only
	existingSkill, err := s.skillManager.ReadSkill(name)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "skill not found",
		})
	}
	if existingSkill.ReadOnly {
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "cannot update read-only skill from git repository",
		})
	}

	var req UpdateSkillRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request",
		})
	}

	// Validate description
	if req.Description == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "description is required",
		})
	}
	if len(req.Description) > 1024 {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "description must be 1-1024 characters",
		})
	}

	// Validate compatibility if provided
	if req.Compatibility != "" && len(req.Compatibility) > 500 {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "compatibility must be max 500 characters",
		})
	}

	// Get the skills directory from the manager
	fsManager, ok := s.skillManager.(*domain.FileSystemManager)
	if !ok {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "unsupported manager type",
		})
	}

	// Build frontmatter (name must match directory name)
	skillDir := filepath.Join(fsManager.GetSkillsDir(), name)
	frontmatter := fmt.Sprintf("---\nname: %s\ndescription: %s\n", name, req.Description)
	if req.License != "" {
		frontmatter += fmt.Sprintf("license: %s\n", req.License)
	}
	if req.Compatibility != "" {
		frontmatter += fmt.Sprintf("compatibility: %s\n", req.Compatibility)
	}
	if len(req.Metadata) > 0 {
		frontmatter += "metadata:\n"
		for k, v := range req.Metadata {
			frontmatter += fmt.Sprintf("  %s: %s\n", k, v)
		}
	}
	if req.AllowedTools != "" {
		frontmatter += fmt.Sprintf("allowed-tools: %s\n", req.AllowedTools)
	}
	frontmatter += "---\n\n"

	// Write SKILL.md file
	skillMdPath := filepath.Join(skillDir, "SKILL.md")
	fullContent := frontmatter + req.Content
	if err := writeFile(skillMdPath, fullContent); err != nil {
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
		Name:     skill.Name,
		Content:  skill.Content,
		ReadOnly: skill.ReadOnly,
	}
	if skill.Metadata != nil {
		response.Description = skill.Metadata.Description
		response.License = skill.Metadata.License
		response.Compatibility = skill.Metadata.Compatibility
		response.Metadata = skill.Metadata.Metadata
		response.AllowedTools = skill.Metadata.AllowedTools
	}

	return c.JSON(http.StatusOK, response)
}

// deleteSkill deletes a skill
func (s *Server) deleteSkill(c *echo.Context) error {
	name := c.Param("name")

	// Check if skill exists and is read-only
	existingSkill, err := s.skillManager.ReadSkill(name)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "skill not found",
		})
	}
	if existingSkill.ReadOnly {
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "cannot delete read-only skill from git repository",
		})
	}

	// Get the skills directory from the manager
	fsManager, ok := s.skillManager.(*domain.FileSystemManager)
	if !ok {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "unsupported manager type",
		})
	}

	// Delete the skill directory
	skillsDir := fsManager.GetSkillsDir()
	skillDir := filepath.Join(skillsDir, name)
	if err := os.RemoveAll(skillDir); err != nil {
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
func (s *Server) searchSkills(c *echo.Context) error {
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
			Name:     skill.Name,
			Content:  skill.Content,
			ReadOnly: skill.ReadOnly,
		}
		if skill.Metadata != nil {
			responses[i].Description = skill.Metadata.Description
			responses[i].License = skill.Metadata.License
			responses[i].Compatibility = skill.Metadata.Compatibility
			responses[i].Metadata = skill.Metadata.Metadata
			responses[i].AllowedTools = skill.Metadata.AllowedTools
		}
	}

	return c.JSON(http.StatusOK, responses)
}

// Helper functions

func writeFile(filename, content string) error {
	return os.WriteFile(filename, []byte(content), 0644)
}

func deleteFile(filename string) error {
	return os.Remove(filename)
}
