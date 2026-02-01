# worktree-dev

Git worktree manager with isolated Docker environments. Run multiple branches simultaneously without port conflicts.

## Installation

```bash
# Clone the repo
git clone https://github.com/yourusername/worktree-dev.git ~/source/worktree-dev

# Add to PATH (add to your ~/.zshrc or ~/.bashrc)
export PATH="$HOME/source/worktree-dev:$PATH"

# Or symlink to a directory already in PATH
ln -s ~/source/worktree-dev/worktree-dev /usr/local/bin/worktree-dev
```

## Usage

```bash
# Create a worktree for a branch
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

The script auto-detects these patterns and assigns unique ports per worktree.

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

## License

MIT
