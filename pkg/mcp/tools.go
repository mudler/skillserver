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
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// ReadSkillInput is the input for read_skill tool
type ReadSkillInput struct {
	Filename string `json:"filename" jsonschema:"The name of the skill file (without .md extension)"`
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
	Name    string `json:"name"`
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
			Name: skill.Name,
		}
		if skill.Metadata != nil {
			skillInfos[i].Description = skill.Metadata.Description
			skillInfos[i].Tags = skill.Metadata.Tags
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
	skill, err := manager.ReadSkill(input.Filename)
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
			Name:    skill.Name,
			Content: skill.Content,
			Snippet: snippet,
		}
	}

	return nil, SearchSkillsOutput{Results: results}, nil
}
