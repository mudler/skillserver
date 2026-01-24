package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/mudler/skillserver/pkg/domain"
	"github.com/mudler/skillserver/pkg/git"
	"github.com/mudler/skillserver/pkg/mcp"
	"github.com/mudler/skillserver/pkg/web"
)

// getEnvOrDefault returns the environment variable value or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvOrEmpty returns the environment variable value or empty string
func getEnvOrEmpty(key string) string {
	return os.Getenv(key)
}

// getEnvBool returns the environment variable as a boolean, or default value
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// setupLogger configures logging based on the enable flag
// When disabled, all logs go to io.Discard to avoid interfering with stdio MCP protocol
func setupLogger(enable bool) *log.Logger {
	var writer io.Writer
	if enable {
		writer = os.Stderr // Use stderr for logs when enabled (doesn't interfere with MCP stdio)
	} else {
		writer = io.Discard // Discard all logs when disabled
	}
	return log.New(writer, "", log.LstdFlags)
}

func main() {
	// Get default values from environment variables
	defaultDir := getEnvOrDefault("SKILLSERVER_DIR", getEnvOrDefault("SKILLS_DIR", "./skills"))
	defaultPort := getEnvOrDefault("SKILLSERVER_PORT", getEnvOrDefault("PORT", "8080"))
	defaultGitRepos := getEnvOrEmpty("SKILLSERVER_GIT_REPOS")
	if defaultGitRepos == "" {
		defaultGitRepos = getEnvOrEmpty("GIT_REPOS")
	}
	// Logging defaults to false (disabled) to avoid interfering with MCP stdio
	defaultEnableLogging := getEnvBool("SKILLSERVER_ENABLE_LOGGING", false)

	// Parse command line flags (flags override environment variables)
	skillsDir := flag.String("dir", defaultDir, "Directory to store skills (env: SKILLSERVER_DIR or SKILLS_DIR)")
	port := flag.String("port", defaultPort, "Port for the web server (env: SKILLSERVER_PORT or PORT)")
	gitReposFlag := flag.String("git-repos", defaultGitRepos, "Comma-separated list of Git repository URLs to sync (env: SKILLSERVER_GIT_REPOS or GIT_REPOS)")
	enableLogging := flag.Bool("enable-logging", defaultEnableLogging, "Enable logging to stderr (env: SKILLSERVER_ENABLE_LOGGING). Default: false (disabled to avoid interfering with MCP stdio)")
	flag.Parse()

	// Setup logger based on flag
	logger := setupLogger(*enableLogging)
	log.SetOutput(logger.Writer())
	log.SetFlags(logger.Flags())

	// Get final values (flags take precedence over env vars)
	finalDir := *skillsDir
	finalPort := *port
	finalGitRepos := *gitReposFlag

	// Parse git repos
	var gitRepos []string
	if finalGitRepos != "" {
		gitRepos = strings.Split(finalGitRepos, ",")
		for i := range gitRepos {
			gitRepos[i] = strings.TrimSpace(gitRepos[i])
		}
	}

	// Initialize skill manager
	skillManager, err := domain.NewFileSystemManager(finalDir)
	if err != nil {
		log.Fatalf("Failed to initialize skill manager: %v", err)
	}

	// Initialize Git syncer if repos are provided
	var gitSyncer *git.GitSyncer
	if len(gitRepos) > 0 {
		gitSyncer = git.NewGitSyncer(finalDir, gitRepos, func() error {
			return skillManager.RebuildIndex()
		})
		// Configure git syncer output based on logging flag
		if *enableLogging {
			gitSyncer.SetProgressWriter(os.Stderr) // Use stderr for git progress
			gitSyncer.SetLogger(os.Stderr)         // Use stderr for log messages
		}
		if err := gitSyncer.Start(); err != nil {
			log.Printf("Warning: Failed to start Git syncer: %v", err)
		} else if *enableLogging {
			log.Println("Git syncer started")
		}
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start web server in a goroutine (non-blocking)
	webServer := web.NewServer(skillManager, gitRepos, *enableLogging)
	go func() {
		addr := fmt.Sprintf(":%s", finalPort)
		if *enableLogging {
			log.Printf("Starting web server on %s", addr)
		}
		if err := webServer.Start(addr); err != nil {
			log.Printf("Web server error: %v", err)
		}
	}()

	// Start MCP server on main thread (blocking, stdio)
	mcpServer := mcp.NewServer(skillManager)

	// Handle shutdown in a goroutine
	go func() {
		<-sigChan
		if *enableLogging {
			log.Println("Shutting down...")
		}

		// Stop Git syncer
		if gitSyncer != nil {
			gitSyncer.Stop()
		}

		// Shutdown web server
		if err := webServer.Shutdown(); err != nil {
			log.Printf("Error shutting down web server: %v", err)
		}

		cancel()
		if *enableLogging {
			log.Println("Shutdown complete")
		}
	}()

	// Run MCP server (blocks main thread)
	// Note: No logging here to avoid interfering with stdio protocol
	if err := mcpServer.Run(ctx); err != nil {
		// Only log errors if logging is enabled
		if *enableLogging {
			log.Printf("MCP server error: %v", err)
		}
	}
}
