package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/mudler/skillserver/pkg/domain"
)

// Server wraps the MCP server and provides access to the skill manager
type Server struct {
	mcpServer    *mcp.Server
	skillManager domain.SkillManager
}

// NewServer creates a new MCP server for skills
func NewServer(skillManager domain.SkillManager) *Server {
	impl := &mcp.Implementation{
		Name:    "skillserver",
		Version: "v1.0.0",
	}

	mcpServer := mcp.NewServer(impl, nil)

	// Register tools with closures that capture the skill manager
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "list_skills",
		Description: "List all available skills",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ListSkillsInput) (
		*mcp.CallToolResult,
		ListSkillsOutput,
		error,
	) {
		return listSkills(ctx, req, input, skillManager)
	})

	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "read_skill",
		Description: "Read the full content of a skill by its ID (use the 'id' field returned by list_skills or search_skills)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ReadSkillInput) (
		*mcp.CallToolResult,
		ReadSkillOutput,
		error,
	) {
		return readSkill(ctx, req, input, skillManager)
	})

	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "search_skills",
		Description: "Search for skills by query string",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input SearchSkillsInput) (
		*mcp.CallToolResult,
		SearchSkillsOutput,
		error,
	) {
		return searchSkills(ctx, req, input, skillManager)
	})

	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "list_skill_resources",
		Description: "List all resources (scripts, references, assets) in a skill",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ListSkillResourcesInput) (
		*mcp.CallToolResult,
		ListSkillResourcesOutput,
		error,
	) {
		return listSkillResources(ctx, req, input, skillManager)
	})

	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "read_skill_resource",
		Description: "Read the content of a skill resource file (scripts, references, or assets). Text files are returned as UTF-8, binary files as base64. Files larger than 1MB cannot be read via MCP.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ReadSkillResourceInput) (
		*mcp.CallToolResult,
		ReadSkillResourceOutput,
		error,
	) {
		return readSkillResource(ctx, req, input, skillManager)
	})

	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "get_skill_resource_info",
		Description: "Get metadata about a specific skill resource without reading its content",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetSkillResourceInfoInput) (
		*mcp.CallToolResult,
		GetSkillResourceInfoOutput,
		error,
	) {
		return getSkillResourceInfo(ctx, req, input, skillManager)
	})

	return &Server{
		mcpServer:    mcpServer,
		skillManager: skillManager,
	}
}

// Run starts the MCP server with stdio transport
func (s *Server) Run(ctx context.Context) error {
	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}
