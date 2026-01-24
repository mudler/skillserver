# SkillServer

A local Golang application serving as a centralized skills database for AI Agents. It manages "Skills" (Markdown files) stored in a local directory.

## Features

- **MCP Server**: Provides tools for AI agents to list, search, and read skills
- **Web Interface**: Local web UI for creating, editing, and organizing skills
- **Git Synchronization**: Automatically syncs with Git repositories
- **Full-Text Search**: Powered by Bleve for fast skill searching

## Installation

### From Source

```bash
git clone <repository-url>
cd SkillsServer
make build
```

### Using Docker

```bash
docker pull ghcr.io/mudler/skillserver:latest
```

## Configuration

SkillServer supports both **environment variables** and **command-line flags**. Flags take precedence over environment variables.

### Environment Variables

| Variable | Alternative | Default | Description |
|----------|-------------|---------|-------------|
| `SKILLSERVER_DIR` | `SKILLS_DIR` | `./skills` | Directory to store skills |
| `SKILLSERVER_PORT` | `PORT` | `8080` | Port for the web server |
| `SKILLSERVER_GIT_REPOS` | `GIT_REPOS` | (empty) | Comma-separated Git repository URLs |

### Command-Line Flags

| Flag | Description |
|------|-------------|
| `--dir` | Directory to store skills (overrides `SKILLSERVER_DIR` or `SKILLS_DIR`) |
| `--port` | Port for the web server (overrides `SKILLSERVER_PORT` or `PORT`) |
| `--git-repos` | Comma-separated list of Git repository URLs (overrides `SKILLSERVER_GIT_REPOS` or `GIT_REPOS`) |

## Usage

### Basic Usage

```bash
# Using defaults
./skillserver

# Using environment variables
export SKILLSERVER_DIR=/path/to/skills
export SKILLSERVER_PORT=9090
./skillserver

# Using command-line flags
./skillserver --dir /path/to/skills --port 9090

# Using both (flags override env vars)
export SKILLSERVER_PORT=8080
./skillserver --port 9090  # Will use 9090
```

### With Git Synchronization

```bash
# Using environment variable
export SKILLSERVER_GIT_REPOS="https://github.com/user/repo1.git,https://github.com/user/repo2.git"
./skillserver

# Using command-line flag
./skillserver --git-repos "https://github.com/user/repo1.git,https://github.com/user/repo2.git"
```

### Docker Usage

```bash
# Using environment variables
docker run -p 8080:8080 \
  -e SKILLSERVER_DIR=/app/skills \
  -e SKILLSERVER_PORT=8080 \
  -e SKILLSERVER_GIT_REPOS="https://github.com/user/repo.git" \
  -v $(pwd)/skills:/app/skills \
  ghcr.io/<owner>/skillserver:latest

# Using command-line flags
docker run -p 8080:8080 \
  -v $(pwd)/skills:/app/skills \
  ghcr.io/<owner>/skillserver:latest \
  --dir /app/skills --port 8080 --git-repos "https://github.com/user/repo.git"
```

## API Endpoints

### REST API

- `GET /api/skills` - List all skills
- `GET /api/skills/:name` - Get skill content
- `POST /api/skills` - Create new skill
- `PUT /api/skills/:name` - Update skill
- `DELETE /api/skills/:name` - Delete skill
- `GET /api/skills/search?q=query` - Search skills
- `GET /api/share/:name` - Generate Git provider URL for sharing

### MCP Tools

- `list_skills` - List all available skills
- `read_skill` - Read the full content of a skill by filename
- `search_skills` - Search for skills by query string

## Development

### Building

```bash
make build
```

### Testing

```bash
make test
```

### Running

```bash
make run
```

### Docker Build

```bash
make docker-build
```

## License

[Add your license here]
