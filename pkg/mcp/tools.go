package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/mudler/skillserver/pkg/domain"
)

// ListSkillsInput is the input for list_skills tool
type ListSkillsInput struct{}

// ListSkillsOutput is the output for list_skills tool
type ListSkillsOutput struct {
	Skills []SkillInfo `json:"skills"`
}

// SkillInfo represents basic information about a skill
type SkillInfo struct {
	ID          string `json:"id"`   // Unique identifier to use when reading the skill (repoName/skillName or skillName)
	Name        string `json:"name"` // Display name
	Description string `json:"description,omitempty"`
}

// ReadSkillInput is the input for read_skill tool
type ReadSkillInput struct {
	ID string `json:"id" jsonschema:"The skill ID returned by list_skills or search_skills (format: 'skill-name' for local skills, or 'repoName/skill-name' for git repo skills)"`
}

// ReadSkillOutput is the output for read_skill tool
type ReadSkillOutput struct {
	Content string `json:"content"`
}

// SearchSkillsInput is the input for search_skills tool
type SearchSkillsInput struct {
	Query string `json:"query" jsonschema:"The search query"`
}

// SearchSkillsOutput is the output for search_skills tool
type SearchSkillsOutput struct {
	Results []SearchResult `json:"results"`
}

// SearchResult represents a search result
type SearchResult struct {
	ID      string `json:"id"`   // Unique identifier to use when reading the skill (repoName/skillName or skillName)
	Name    string `json:"name"` // Display name
	Content string `json:"content"`
	Snippet string `json:"snippet,omitempty"`
}

// listSkills lists all available skills
func listSkills(ctx context.Context, req *mcp.CallToolRequest, input ListSkillsInput, manager domain.SkillManager) (
	*mcp.CallToolResult,
	ListSkillsOutput,
	error,
) {
	skills, err := manager.ListSkills()
	if err != nil {
		return nil, ListSkillsOutput{}, fmt.Errorf("failed to list skills: %w", err)
	}

	skillInfos := make([]SkillInfo, len(skills))
	for i, skill := range skills {
		skillInfos[i] = SkillInfo{
			ID: skill.ID,
			//	Name: skill.Name,
		}
		if skill.Metadata != nil {
			skillInfos[i].Description = skill.Metadata.Description
		}
	}

	return nil, ListSkillsOutput{Skills: skillInfos}, nil
}

// readSkill reads the full content of a skill
func readSkill(ctx context.Context, req *mcp.CallToolRequest, input ReadSkillInput, manager domain.SkillManager) (
	*mcp.CallToolResult,
	ReadSkillOutput,
	error,
) {
	skill, err := manager.ReadSkill(input.ID)
	if err != nil {
		return nil, ReadSkillOutput{}, fmt.Errorf("failed to read skill: %w", err)
	}

	return nil, ReadSkillOutput{Content: skill.Content}, nil
}

// searchSkills searches for skills matching the query
func searchSkills(ctx context.Context, req *mcp.CallToolRequest, input SearchSkillsInput, manager domain.SkillManager) (
	*mcp.CallToolResult,
	SearchSkillsOutput,
	error,
) {
	skills, err := manager.SearchSkills(input.Query)
	if err != nil {
		return nil, SearchSkillsOutput{}, fmt.Errorf("failed to search skills: %w", err)
	}

	results := make([]SearchResult, len(skills))
	for i, skill := range skills {
		// Create a snippet (first 200 characters)
		snippet := skill.Content
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}

		results[i] = SearchResult{
			ID:      skill.ID,
			Name:    skill.Name,
			Content: skill.Content,
			Snippet: snippet,
		}
	}

	return nil, SearchSkillsOutput{Results: results}, nil
}

// ListSkillResourcesInput is the input for list_skill_resources tool
type ListSkillResourcesInput struct {
	SkillID string `json:"skill_id" jsonschema:"The skill ID returned by list_skills or search_skills (format: 'skill-name' for local skills, or 'repoName/skill-name' for git repo skills)"`
}

// ListSkillResourcesOutput is the output for list_skill_resources tool
type ListSkillResourcesOutput struct {
	Resources []SkillResourceInfo `json:"resources"`
}

// SkillResourceInfo represents resource information in MCP responses
type SkillResourceInfo struct {
	Type     string `json:"type"`      // "script", "reference", or "asset"
	Path     string `json:"path"`      // Relative path from skill root
	Name     string `json:"name"`      // Filename only
	Size     int64  `json:"size"`      // File size in bytes
	MimeType string `json:"mime_type"` // MIME type
	Readable bool   `json:"readable"`  // true if text file, false if binary
}

// ReadSkillResourceInput is the input for read_skill_resource tool
type ReadSkillResourceInput struct {
	SkillID      string `json:"skill_id"`      // The skill ID
	ResourcePath string `json:"resource_path"` // Relative path from skill root (e.g., "scripts/script.py")
}

// ReadSkillResourceOutput is the output for read_skill_resource tool
type ReadSkillResourceOutput struct {
	Content  string `json:"content"`  // UTF-8 for text, base64 for binary
	Encoding string `json:"encoding"` // "utf-8" or "base64"
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
}

// GetSkillResourceInfoInput is the input for get_skill_resource_info tool
type GetSkillResourceInfoInput struct {
	SkillID      string `json:"skill_id"`
	ResourcePath string `json:"resource_path"`
}

// GetSkillResourceInfoOutput is the output for get_skill_resource_info tool
type GetSkillResourceInfoOutput struct {
	Exists   bool   `json:"exists"`
	Type     string `json:"type"`
	Path     string `json:"path"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	MimeType string `json:"mime_type"`
	Readable bool   `json:"readable"`
}

// listSkillResources lists all resources in a skill's optional directories
func listSkillResources(ctx context.Context, req *mcp.CallToolRequest, input ListSkillResourcesInput, manager domain.SkillManager) (
	*mcp.CallToolResult,
	ListSkillResourcesOutput,
	error,
) {
	resources, err := manager.ListSkillResources(input.SkillID)
	if err != nil {
		return nil, ListSkillResourcesOutput{}, fmt.Errorf("failed to list skill resources: %w", err)
	}

	resourceInfos := make([]SkillResourceInfo, len(resources))
	for i, res := range resources {
		resourceInfos[i] = SkillResourceInfo{
			Type:     string(res.Type),
			Path:     res.Path,
			Name:     res.Name,
			Size:     res.Size,
			MimeType: res.MimeType,
			Readable: res.Readable,
		}
	}

	return nil, ListSkillResourcesOutput{Resources: resourceInfos}, nil
}

// readSkillResource reads the content of a skill resource file
func readSkillResource(ctx context.Context, req *mcp.CallToolRequest, input ReadSkillResourceInput, manager domain.SkillManager) (
	*mcp.CallToolResult,
	ReadSkillResourceOutput,
	error,
) {
	// Check file size limit (1MB for MCP)
	info, err := manager.GetSkillResourceInfo(input.SkillID, input.ResourcePath)
	if err != nil {
		return nil, ReadSkillResourceOutput{}, fmt.Errorf("failed to get resource info: %w", err)
	}

	const maxMCPFileSize = 1024 * 1024 // 1MB
	if info.Size > maxMCPFileSize {
		return nil, ReadSkillResourceOutput{}, fmt.Errorf("file too large (%d bytes, max %d). Use web UI to download", info.Size, maxMCPFileSize)
	}

	content, err := manager.ReadSkillResource(input.SkillID, input.ResourcePath)
	if err != nil {
		return nil, ReadSkillResourceOutput{}, fmt.Errorf("failed to read resource: %w", err)
	}

	return nil, ReadSkillResourceOutput{
		Content:  content.Content,
		Encoding: content.Encoding,
		MimeType: content.MimeType,
		Size:     content.Size,
	}, nil
}

// getSkillResourceInfo gets metadata about a specific resource without reading content
func getSkillResourceInfo(ctx context.Context, req *mcp.CallToolRequest, input GetSkillResourceInfoInput, manager domain.SkillManager) (
	*mcp.CallToolResult,
	GetSkillResourceInfoOutput,
	error,
) {
	info, err := manager.GetSkillResourceInfo(input.SkillID, input.ResourcePath)
	if err != nil {
		// Resource doesn't exist
		return nil, GetSkillResourceInfoOutput{
			Exists: false,
		}, nil
	}

	return nil, GetSkillResourceInfoOutput{
		Exists:   true,
		Type:     string(info.Type),
		Path:     info.Path,
		Name:     info.Name,
		Size:     info.Size,
		MimeType: info.MimeType,
		Readable: info.Readable,
	}, nil
}
