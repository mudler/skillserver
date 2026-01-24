package web

import (
	"context"
	"embed"
	"io/fs"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/mudler/skillserver/pkg/domain"
)

//go:embed ui
var uiFiles embed.FS

// Server wraps the Echo server
type Server struct {
	echo         *echo.Echo
	skillManager domain.SkillManager
	gitRepos     []string
}

// NewServer creates a new web server
func NewServer(skillManager domain.SkillManager, gitRepos []string) *Server {
	e := echo.New()

	// Middleware
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	server := &Server{
		echo:         e,
		skillManager: skillManager,
		gitRepos:     gitRepos,
	}

	// API routes
	api := e.Group("/api")
	api.GET("/skills", server.listSkills)
	api.GET("/skills/:name", server.getSkill)
	api.POST("/skills", server.createSkill)
	api.PUT("/skills/:name", server.updateSkill)
	api.DELETE("/skills/:name", server.deleteSkill)
	api.GET("/skills/search", server.searchSkills)
	api.GET("/share/:name", server.getShareURL)

	// Serve UI
	uiFS, err := fs.Sub(uiFiles, "ui")
	if err != nil {
		panic(err)
	}
	e.GET("/*", echo.WrapHandler(http.FileServer(http.FS(uiFS))))

	return server
}

// Start starts the web server
func (s *Server) Start(addr string) error {
	return s.echo.Start(addr)
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	return s.echo.Shutdown(context.Background())
}
