# worktree-dev

Git worktree manager with isolated Docker environments. Run multiple branches simultaneously without port conflicts.

## Installation

### Requirements
- Go 1.21+
- Git
- Docker & docker-compose

### Build from source

```bash
# Clone and build
git clone https://github.com/dylan/worktree-dev
cd worktree-dev

# Install dependencies and build
make deps
make install
```

This installs `worktree-dev` to `~/.local/bin`. Make sure this is in your PATH:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

## Usage

```bash
# Interactive mode - choose between new or existing branch
worktree-dev create

# Create worktree for a specific branch
worktree-dev create feature/new-api

# List all worktrees
worktree-dev list

# Remove a worktree (stops containers, removes volumes)
worktree-dev remove feature/new-api

# Preview ports for a branch
worktree-dev ports feature/new-api
```

## What it does

1. Creates a git worktree at `.worktrees/<branch-name>/`
2. Copies your `.env` file
3. Creates `.env.local` with:
   - `COMPOSE_PROJECT_NAME` - isolates containers, networks, volumes
   - Port overrides detected from your `docker-compose.yml`
4. Creates a `./dev` helper script for easy commands

## Requirements

Your `docker-compose.yml` must use environment variables for ports:

```yaml
services:
  web:
    ports:
      - "${WEB_PORT:-8000}:8000"

  db:
    ports:
      - "${DB_PORT:-5432}:5432"
```

The tool auto-detects these patterns and assigns unique ports per worktree.

## Worktree Commands

Each worktree includes a `./dev` helper:

```bash
cd .worktrees/feature-new-api

./dev up              # Start services
./dev down            # Stop services
./dev logs            # View logs
./dev ps              # Show containers
./dev exec web bash   # Shell into container
```

## How Isolation Works

- **COMPOSE_PROJECT_NAME**: Docker prefixes all resources with this, so `myapp-feature_web` won't conflict with `myapp-hotfix_web`
- **Port offsets**: Each branch gets a deterministic offset (1-99) based on its name hash
- **Separate volumes**: Each project gets its own named volumes (fresh database)

## Shell integration (optional)

Add to your `.zshrc` or `.bashrc` to auto-cd into created worktrees:

```bash
wt() {
    local output
    output=$(worktree-dev "$@")
    echo "$output"

    # Extract worktree path if present
    local path=$(echo "$output" | grep "^WORKTREE_PATH:" | cut -d: -f2)
    if [ -n "$path" ] && [ -d "$path" ]; then
        cd "$path"
    fi
}
```

Then use `wt create feature/foo` to create and cd in one command.

## Development

```bash
make build    # Build binary to ./bin/
make test     # Run tests
make clean    # Remove build artifacts
```

## License

MIT
