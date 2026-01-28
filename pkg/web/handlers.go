package web

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/mudler/skillserver/pkg/domain"
	"github.com/mudler/skillserver/pkg/git"
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

// Resource management handlers

// listSkillResources lists all resources in a skill
func (s *Server) listSkillResources(c *echo.Context) error {
	skillName := c.Param("name")

	// Check if skill exists
	skill, err := s.skillManager.ReadSkill(skillName)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "skill not found",
		})
	}

	resources, err := s.skillManager.ListSkillResources(skill.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	// Group resources by type
	scripts := []map[string]any{}
	references := []map[string]any{}
	assets := []map[string]any{}

	for _, res := range resources {
		resourceMap := map[string]any{
			"path":      res.Path,
			"name":      res.Name,
			"size":      res.Size,
			"mime_type": res.MimeType,
			"readable":  res.Readable,
			"modified":  res.Modified.Format(time.RFC3339),
		}

		switch res.Type {
		case domain.ResourceTypeScript:
			scripts = append(scripts, resourceMap)
		case domain.ResourceTypeReference:
			references = append(references, resourceMap)
		case domain.ResourceTypeAsset:
			assets = append(assets, resourceMap)
		}
	}

	return c.JSON(http.StatusOK, map[string]any{
		"scripts":    scripts,
		"references": references,
		"assets":     assets,
		"readOnly":   skill.ReadOnly,
	})
}

// getSkillResource gets a specific resource file
func (s *Server) getSkillResource(c *echo.Context) error {
	skillName := c.Param("name")
	resourcePath := c.Param("*")

	if resourcePath == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "resource path is required",
		})
	}

	// Check if skill exists
	skill, err := s.skillManager.ReadSkill(skillName)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "skill not found",
		})
	}

	// Get resource info first
	info, err := s.skillManager.GetSkillResourceInfo(skill.ID, resourcePath)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "resource not found",
		})
	}

	// Read resource content
	content, err := s.skillManager.ReadSkillResource(skill.ID, resourcePath)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	// Check if client wants base64 encoding
	encoding := c.QueryParam("encoding")
	if encoding == "base64" || !info.Readable {
		return c.JSON(http.StatusOK, map[string]any{
			"content":   content.Content,
			"encoding":  content.Encoding,
			"mime_type": content.MimeType,
			"size":      content.Size,
		})
	}

	// For text files, return as plain text
	c.Response().Header().Set("Content-Type", content.MimeType)
	return c.String(http.StatusOK, content.Content)
}

// createSkillResource creates/uploads a new resource
func (s *Server) createSkillResource(c *echo.Context) error {
	skillName := c.Param("name")

	// Check if skill exists and is not read-only
	skill, err := s.skillManager.ReadSkill(skillName)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "skill not found",
		})
	}
	if skill.ReadOnly {
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "cannot create resources in read-only skill from git repository",
		})
	}

	fsManager, ok := s.skillManager.(*domain.FileSystemManager)
	if !ok {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "unsupported manager type",
		})
	}

	// Check Content-Type to determine if it's multipart/form-data or JSON
	contentType := c.Request().Header.Get("Content-Type")

	var resourcePath string
	var fileContent []byte

	if strings.HasPrefix(contentType, "multipart/form-data") {
		// Handle file upload
		file, err := c.FormFile("file")
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "file is required",
			})
		}

		resourceType := c.FormValue("type")
		if resourceType == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "type is required (script, reference, or asset)",
			})
		}

		pathValue := c.FormValue("path")
		if pathValue != "" {
			resourcePath = pathValue
		} else {
			resourcePath = resourceType + "s/" + file.Filename
		}

		// Validate path starts with correct directory
		if !strings.HasPrefix(resourcePath, resourceType+"s/") {
			resourcePath = resourceType + "s/" + file.Filename
		}

		src, err := file.Open()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to open uploaded file",
			})
		}
		defer src.Close()

		fileContent, err = io.ReadAll(src)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to read uploaded file",
			})
		}
	} else {
		// Handle JSON request for text files
		var req struct {
			Type    string `json:"type"`
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "invalid request",
			})
		}

		resourcePath = req.Path
		if req.Type != "" && !strings.HasPrefix(resourcePath, req.Type+"/") {
			resourcePath = req.Type + "/" + resourcePath
		}
		fileContent = []byte(req.Content)
	}

	// Validate path
	if err := domain.ValidateResourcePath(resourcePath); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	// Check size limit (10MB)
	const maxFileSize = 10 * 1024 * 1024
	if len(fileContent) > maxFileSize {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("file too large (max %d bytes)", maxFileSize),
		})
	}

	// Write file
	skillsDir := fsManager.GetSkillsDir()
	skillDir := filepath.Join(skillsDir, skillName)
	fullPath := filepath.Join(skillDir, resourcePath)

	// Create parent directories if needed
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to create directory: %v", err),
		})
	}

	if err := os.WriteFile(fullPath, fileContent, 0644); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	// Return resource info
	info, err := s.skillManager.GetSkillResourceInfo(skill.ID, resourcePath)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to read created resource",
		})
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"path":      info.Path,
		"name":      info.Name,
		"size":      info.Size,
		"mime_type": info.MimeType,
		"readable":  info.Readable,
		"modified":  info.Modified.Format(time.RFC3339),
	})
}

// updateSkillResource updates an existing resource
func (s *Server) updateSkillResource(c *echo.Context) error {
	skillName := c.Param("name")
	resourcePath := c.Param("*")

	if resourcePath == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "resource path is required",
		})
	}

	// Check if skill exists and is not read-only
	skill, err := s.skillManager.ReadSkill(skillName)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "skill not found",
		})
	}
	if skill.ReadOnly {
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "cannot update resources in read-only skill from git repository",
		})
	}

	// Validate path
	if err := domain.ValidateResourcePath(resourcePath); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	// Read request body
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "failed to read request body",
		})
	}

	// Check size limit
	const maxFileSize = 10 * 1024 * 1024
	if len(body) > maxFileSize {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("file too large (max %d bytes)", maxFileSize),
		})
	}

	fsManager, ok := s.skillManager.(*domain.FileSystemManager)
	if !ok {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "unsupported manager type",
		})
	}

	// Write file
	skillsDir := fsManager.GetSkillsDir()
	skillDir := filepath.Join(skillsDir, skillName)
	fullPath := filepath.Join(skillDir, resourcePath)

	if err := os.WriteFile(fullPath, body, 0644); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	// Return resource info
	info, err := s.skillManager.GetSkillResourceInfo(skill.ID, resourcePath)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to read updated resource",
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"path":      info.Path,
		"name":      info.Name,
		"size":      info.Size,
		"mime_type": info.MimeType,
		"readable":  info.Readable,
		"modified":  info.Modified.Format(time.RFC3339),
	})
}

// deleteSkillResource deletes a resource
func (s *Server) deleteSkillResource(c *echo.Context) error {
	skillName := c.Param("name")
	resourcePath := c.Param("*")

	if resourcePath == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "resource path is required",
		})
	}

	// Check if skill exists and is not read-only
	skill, err := s.skillManager.ReadSkill(skillName)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "skill not found",
		})
	}
	if skill.ReadOnly {
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "cannot delete resources from read-only skill from git repository",
		})
	}

	// Validate path
	if err := domain.ValidateResourcePath(resourcePath); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	fsManager, ok := s.skillManager.(*domain.FileSystemManager)
	if !ok {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "unsupported manager type",
		})
	}

	// Delete file
	skillsDir := fsManager.GetSkillsDir()
	skillDir := filepath.Join(skillsDir, skillName)
	fullPath := filepath.Join(skillDir, resourcePath)

	if err := os.Remove(fullPath); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	return c.NoContent(http.StatusNoContent)
}

// exportSkill exports a skill as a compressed archive
func (s *Server) exportSkill(c *echo.Context) error {
	// Get skill name from wildcard path (handles names with slashes like "repoName/skillName")
	// The route is /skills/export/*, so * captures the skill name
	name := c.Param("*")
	// Remove leading slash if present
	name = strings.TrimPrefix(name, "/")
	// URL decode the name (Echo should do this automatically, but be explicit)
	if decoded, err := url.PathUnescape(name); err == nil {
		name = decoded
	}

	// Check if skill exists
	skill, err := s.skillManager.ReadSkill(name)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "skill not found",
		})
	}

	// Get the skills directory from the manager
	fsManager, ok := s.skillManager.(*domain.FileSystemManager)
	if !ok {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "unsupported manager type",
		})
	}

	// Create archive
	archiveData, err := domain.ExportSkill(skill.ID, fsManager.GetSkillsDir())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to create archive: %v", err),
		})
	}

	// Set headers for file download
	c.Response().Header().Set("Content-Type", "application/gzip")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.tar.gz\"", name))
	c.Response().Header().Set("Content-Length", fmt.Sprintf("%d", len(archiveData)))

	return c.Blob(http.StatusOK, "application/gzip", archiveData)
}

// importSkill imports a skill from a compressed archive
func (s *Server) importSkill(c *echo.Context) error {
	// Get uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "file is required",
		})
	}

	// Open file
	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "failed to open uploaded file",
		})
	}
	defer src.Close()

	// Read file content
	const maxArchiveSize = 50 * 1024 * 1024 // 50MB limit
	archiveData := make([]byte, file.Size)
	if file.Size > maxArchiveSize {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("archive too large (max %d bytes)", maxArchiveSize),
		})
	}

	n, err := io.ReadFull(src, archiveData)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "failed to read uploaded file",
		})
	}
	archiveData = archiveData[:n]

	// Get the skills directory from the manager
	fsManager, ok := s.skillManager.(*domain.FileSystemManager)
	if !ok {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "unsupported manager type",
		})
	}

	// Import skill
	skillName, err := domain.ImportSkill(archiveData, fsManager.GetSkillsDir())
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	// Rebuild index
	if err := s.skillManager.RebuildIndex(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to rebuild index",
		})
	}

	// Read the imported skill
	skill, err := s.skillManager.ReadSkill(skillName)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to read imported skill",
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

// Git repository management handlers

// GitRepoResponse represents a git repository in API responses
type GitRepoResponse struct {
	ID      string `json:"id"`
	URL     string `json:"url"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// AddGitRepoRequest represents a request to add a git repository
type AddGitRepoRequest struct {
	URL string `json:"url"`
}

// UpdateGitRepoRequest represents a request to update a git repository
type UpdateGitRepoRequest struct {
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
}

// listGitRepos lists all configured git repositories
func (s *Server) listGitRepos(c *echo.Context) error {
	if s.gitSyncer == nil {
		return c.JSON(http.StatusOK, []GitRepoResponse{})
	}

	// Get repos from syncer
	repoURLs := s.gitSyncer.GetRepos()

	// Convert to response format
	repos := make([]GitRepoResponse, len(repoURLs))
	for i, url := range repoURLs {
		repos[i] = GitRepoResponse{
			ID:      git.GenerateID(url),
			URL:     url,
			Name:    git.ExtractRepoName(url),
			Enabled: true, // All repos in syncer are enabled
		}
	}

	return c.JSON(http.StatusOK, repos)
}

// addGitRepo adds a new git repository
func (s *Server) addGitRepo(c *echo.Context) error {
	if s.gitSyncer == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "git syncer not available",
		})
	}

	var req AddGitRepoRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request",
		})
	}

	if req.URL == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "URL is required",
		})
	}

	// Validate URL format (basic check)
	if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") && !strings.HasPrefix(req.URL, "git@") {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid URL format",
		})
	}

	// Add repo to syncer
	if err := s.gitSyncer.AddRepo(req.URL); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	// Update FileSystemManager's git repos list for read-only detection
	if s.fsManager != nil {
		repos := s.gitSyncer.GetRepos()
		gitRepoNames := make([]string, len(repos))
		for i, url := range repos {
			gitRepoNames[i] = git.ExtractRepoName(url)
		}
		s.fsManager.UpdateGitRepos(gitRepoNames)
	}

	// Save config
	if s.configManager != nil {
		repos := s.gitSyncer.GetRepos()
		configs := make([]git.GitRepoConfig, len(repos))
		for i, url := range repos {
			configs[i] = git.GitRepoConfig{
				ID:      git.GenerateID(url),
				URL:     url,
				Name:    git.ExtractRepoName(url),
				Enabled: true,
			}
		}
		if err := s.configManager.SaveConfig(configs); err != nil {
			// Log error but don't fail the request
			fmt.Printf("Warning: failed to save config: %v\n", err)
		}
	}

	response := GitRepoResponse{
		ID:      git.GenerateID(req.URL),
		URL:     req.URL,
		Name:    git.ExtractRepoName(req.URL),
		Enabled: true,
	}

	return c.JSON(http.StatusCreated, response)
}

// updateGitRepo updates a git repository
func (s *Server) updateGitRepo(c *echo.Context) error {
	if s.gitSyncer == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "git syncer not available",
		})
	}

	id := c.Param("id")

	var req UpdateGitRepoRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request",
		})
	}

	// Find repo by ID
	repos := s.gitSyncer.GetRepos()
	var foundURL string
	for _, url := range repos {
		if git.GenerateID(url) == id {
			foundURL = url
			break
		}
	}

	if foundURL == "" {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "repository not found",
		})
	}

	// If URL changed, remove old and add new
	if req.URL != "" && req.URL != foundURL {
		if err := s.gitSyncer.RemoveRepo(foundURL); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": err.Error(),
			})
		}
		if err := s.gitSyncer.AddRepo(req.URL); err != nil {
			// Try to restore old repo on error
			s.gitSyncer.AddRepo(foundURL)
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
		}
		foundURL = req.URL
	}

	// Update FileSystemManager's git repos list for read-only detection
	if s.fsManager != nil {
		repos := s.gitSyncer.GetRepos()
		gitRepoNames := make([]string, len(repos))
		for i, url := range repos {
			gitRepoNames[i] = git.ExtractRepoName(url)
		}
		s.fsManager.UpdateGitRepos(gitRepoNames)
	}

	// Save config
	if s.configManager != nil {
		repos := s.gitSyncer.GetRepos()
		configs := make([]git.GitRepoConfig, len(repos))
		for i, url := range repos {
			configs[i] = git.GitRepoConfig{
				ID:      git.GenerateID(url),
				URL:     url,
				Name:    git.ExtractRepoName(url),
				Enabled: true,
			}
		}
		if err := s.configManager.SaveConfig(configs); err != nil {
			fmt.Printf("Warning: failed to save config: %v\n", err)
		}
	}

	response := GitRepoResponse{
		ID:      git.GenerateID(foundURL),
		URL:     foundURL,
		Name:    git.ExtractRepoName(foundURL),
		Enabled: req.Enabled,
	}

	return c.JSON(http.StatusOK, response)
}

// deleteGitRepo deletes a git repository
func (s *Server) deleteGitRepo(c *echo.Context) error {
	if s.gitSyncer == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "git syncer not available",
		})
	}

	id := c.Param("id")

	// Find repo by ID
	repos := s.gitSyncer.GetRepos()
	var foundURL string
	for _, url := range repos {
		if git.GenerateID(url) == id {
			foundURL = url
			break
		}
	}

	if foundURL == "" {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "repository not found",
		})
	}

	// Get repo name to delete the directory
	repoName := git.ExtractRepoName(foundURL)

	// Remove repo from syncer
	if err := s.gitSyncer.RemoveRepo(foundURL); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	// Delete the repository directory and all its contents
	skillsDir := s.gitSyncer.GetSkillsDir()
	repoDir := filepath.Join(skillsDir, repoName)
	if err := os.RemoveAll(repoDir); err != nil {
		// Log error but don't fail the request - repo is already removed from config
		fmt.Printf("Warning: failed to delete repository directory %s: %v\n", repoDir, err)
	}

	// Update FileSystemManager's git repos list for read-only detection
	if s.fsManager != nil {
		repos := s.gitSyncer.GetRepos()
		gitRepoNames := make([]string, len(repos))
		for i, url := range repos {
			gitRepoNames[i] = git.ExtractRepoName(url)
		}
		s.fsManager.UpdateGitRepos(gitRepoNames)
	}

	// Save config
	if s.configManager != nil {
		repos := s.gitSyncer.GetRepos()
		configs := make([]git.GitRepoConfig, len(repos))
		for i, url := range repos {
			configs[i] = git.GitRepoConfig{
				ID:      git.GenerateID(url),
				URL:     url,
				Name:    git.ExtractRepoName(url),
				Enabled: true,
			}
		}
		if err := s.configManager.SaveConfig(configs); err != nil {
			fmt.Printf("Warning: failed to save config: %v\n", err)
		}
	}

	// Trigger re-indexing
	if err := s.skillManager.RebuildIndex(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to rebuild index",
		})
	}

	return c.NoContent(http.StatusNoContent)
}

// syncGitRepo manually syncs a git repository
func (s *Server) syncGitRepo(c *echo.Context) error {
	if s.gitSyncer == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "git syncer not available",
		})
	}

	id := c.Param("id")

	// Find repo by ID
	repos := s.gitSyncer.GetRepos()
	var foundURL string
	for _, url := range repos {
		if git.GenerateID(url) == id {
			foundURL = url
			break
		}
	}

	if foundURL == "" {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "repository not found",
		})
	}

	// Sync the repo
	if err := s.gitSyncer.SyncRepo(foundURL); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	response := GitRepoResponse{
		ID:      git.GenerateID(foundURL),
		URL:     foundURL,
		Name:    git.ExtractRepoName(foundURL),
		Enabled: true,
	}

	return c.JSON(http.StatusOK, response)
}

// Helper functions

func writeFile(filename, content string) error {
	return os.WriteFile(filename, []byte(content), 0644)
}

func deleteFile(filename string) error {
	return os.Remove(filename)
}
