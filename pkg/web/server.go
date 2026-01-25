package web

import (
	"context"
	"embed"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"github.com/mudler/skillserver/pkg/domain"
)

//go:embed ui
var uiFiles embed.FS

// Server wraps the Echo server
type Server struct {
	echo         *echo.Echo
	httpServer   *http.Server
	skillManager domain.SkillManager
	gitRepos     []string
}

// NewServer creates a new web server
func NewServer(skillManager domain.SkillManager, gitRepos []string, enableLogging bool) *Server {
	e := echo.New()

	// Middleware
	// Only enable request logging if explicitly enabled (to avoid interfering with MCP stdio)
	if enableLogging {
		e.Use(middleware.RequestLogger())
	} else {
		e.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	e.Use(middleware.Recover())
	//e.Use(middleware.CORS())

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

	// Resource management routes
	api.GET("/skills/:name/resources", server.listSkillResources)
	api.GET("/skills/:name/resources/*", server.getSkillResource)
	api.POST("/skills/:name/resources", server.createSkillResource)
	api.PUT("/skills/:name/resources/*", server.updateSkillResource)
	api.DELETE("/skills/:name/resources/*", server.deleteSkillResource)

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
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.echo,
	}
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	if s.httpServer == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}
