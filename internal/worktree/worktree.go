package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Styles for output
var (
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))  // Green
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))  // Red
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))  // Yellow
	infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))  // Blue
	cyanStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))  // Cyan
	boldStyle    = lipgloss.NewStyle().Bold(true)
	dimStyle     = lipgloss.NewStyle().Faint(true)
)

// Repo represents a git repository with worktree management
type Repo struct {
	Root         string
	Name         string
	WorktreesDir string
}

// NewRepo creates a new Repo from the current directory
func NewRepo() (*Repo, error) {
	root, err := gitRoot()
	if err != nil {
		return nil, fmt.Errorf("not inside a git repository")
	}

	return &Repo{
		Root:         root,
		Name:         filepath.Base(root),
		WorktreesDir: filepath.Join(root, ".worktrees"),
	}, nil
}

// CreateWorktree creates a new worktree for the given branch
func (r *Repo) CreateWorktree(branch string) error {
	safeName := sanitizeName(branch)
	worktreePath := filepath.Join(r.WorktreesDir, safeName)
	offset := getPortOffset(safeName)
	prefix := getProjectPrefix(r.Name)

	fmt.Println(infoStyle.Render("Creating worktree for branch:"), warnStyle.Render(branch))
	fmt.Println(infoStyle.Render("Repository:"), r.Name)
	fmt.Println(infoStyle.Render("Location:"), worktreePath)
	fmt.Println()

	// Create worktrees directory
	if err := os.MkdirAll(r.WorktreesDir, 0755); err != nil {
		return fmt.Errorf("failed to create worktrees directory: %w", err)
	}

	// Add .worktrees to .gitignore
	if err := r.ensureGitignore(); err != nil {
		fmt.Println(warnStyle.Render("Warning: could not update .gitignore:"), err)
	}

	// Check if worktree already exists
	if _, err := os.Stat(worktreePath); err == nil {
		return fmt.Errorf("worktree already exists at %s\nUse 'worktree-dev remove %s' first if you want to recreate it", worktreePath, branch)
	}

	// Check if branch is currently checked out
	currentBranch, _ := r.currentBranch()
	if currentBranch == branch {
		return fmt.Errorf("'%s' is currently checked out in the main repo\nSwitch to a different branch first, or create a worktree for a different branch", branch)
	}

	// Check if branch exists, create if not
	if !r.branchExists(branch) {
		if r.remoteBranchExists(branch) {
			fmt.Println(infoStyle.Render("Branch exists on remote, will track origin/" + branch))
		} else {
			fmt.Println(warnStyle.Render("Branch '" + branch + "' doesn't exist. Creating new branch from current HEAD..."))
			if err := r.git("branch", branch); err != nil {
				return fmt.Errorf("failed to create branch: %w", err)
			}
		}
	}

	// Create the worktree
	fmt.Println(infoStyle.Render("Creating git worktree..."))
	if err := r.git("worktree", "add", worktreePath, branch); err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}

	// Copy .env files
	r.copyEnvFiles(worktreePath)

	// Detect ports and create config
	ports := r.detectPorts()
	projectName := fmt.Sprintf("%s-%s", prefix, safeName)

	// Create .env.local with isolated configuration
	if err := r.createEnvLocal(worktreePath, branch, projectName, offset, ports); err != nil {
		return fmt.Errorf("failed to create .env.local: %w", err)
	}

	// Create the dev helper script
	if err := r.createDevScript(worktreePath, projectName, offset, ports); err != nil {
		return fmt.Errorf("failed to create dev script: %w", err)
	}

	// Print success
	fmt.Println()
	fmt.Println(successStyle.Render("========================================"))
	fmt.Println(successStyle.Render("Worktree created successfully!"))
	fmt.Println(successStyle.Render("========================================"))
	fmt.Println()
	fmt.Println(infoStyle.Render("Location: "), worktreePath)
	fmt.Println(infoStyle.Render("Branch:   "), branch)
	fmt.Println(infoStyle.Render("Project:  "), projectName)
	fmt.Println()

	if len(ports) > 0 {
		fmt.Println(warnStyle.Render(fmt.Sprintf("Ports allocated (offset +%d):", offset)))
		for _, p := range ports {
			fmt.Printf("  %s: %d\n", p.VarName, p.Default+offset)
		}
		fmt.Println()
	}

	fmt.Println(warnStyle.Render("Commands:"))
	fmt.Println("  ./dev up              # Start services")
	fmt.Println("  ./dev logs            # View logs")
	fmt.Println("  ./dev down            # Stop services")
	fmt.Println()
	fmt.Println("WORKTREE_PATH:" + worktreePath)

	return nil
}

// ListWorktrees lists all worktrees with their status
func (r *Repo) ListWorktrees() error {
	fmt.Println(infoStyle.Render("Worktrees for"), cyanStyle.Render(r.Name)+":")
	fmt.Println()

	worktrees, err := r.getWorktrees()
	if err != nil {
		return err
	}

	found := false
	for _, wt := range worktrees {
		if strings.Contains(wt.Path, ".worktrees") {
			found = true
			safeName := filepath.Base(wt.Path)
			prefix := getProjectPrefix(r.Name)
			project := fmt.Sprintf("%s-%s", prefix, safeName)

			running := r.countRunningContainers(project)

			if running > 0 {
				fmt.Printf("  %s %s\n", successStyle.Render("●"), wt.Branch)
				fmt.Printf("    Path: %s\n", wt.Path)
				fmt.Printf("    Project: %s (%d containers running)\n", project, running)
			} else {
				fmt.Printf("  %s %s\n", warnStyle.Render("○"), wt.Branch)
				fmt.Printf("    Path: %s\n", wt.Path)
				fmt.Printf("    Project: %s (stopped)\n", project)
			}
			fmt.Println()
		} else if wt.Path == r.Root {
			fmt.Printf("  %s %s %s\n", infoStyle.Render("◆"), wt.Branch, cyanStyle.Render("(main repo)"))
			fmt.Printf("    Path: %s\n", wt.Path)
			fmt.Println()
		}
	}

	if !found {
		fmt.Println(warnStyle.Render("  No worktrees created yet."))
		fmt.Println("  Run: worktree-dev create <branch-name>")
		fmt.Println()
	}

	return nil
}

// RemoveWorktree removes a worktree and cleans up Docker resources
func (r *Repo) RemoveWorktree(branch string) error {
	safeName := sanitizeName(branch)
	worktreePath := filepath.Join(r.WorktreesDir, safeName)
	prefix := getProjectPrefix(r.Name)
	project := fmt.Sprintf("%s-%s", prefix, safeName)

	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		return fmt.Errorf("worktree not found at %s", worktreePath)
	}

	fmt.Println(warnStyle.Render("Removing worktree:"), branch)

	// Stop and remove Docker containers and volumes
	fmt.Println(infoStyle.Render("Stopping Docker containers and removing volumes..."))
	r.dockerComposeDown(worktreePath, project)

	// Remove any remaining containers
	r.removeContainers(project)

	// Remove worktree
	fmt.Println(infoStyle.Render("Removing git worktree..."))
	_ = r.git("worktree", "remove", worktreePath, "--force")

	// If that didn't work, force remove the directory
	if _, err := os.Stat(worktreePath); err == nil {
		os.RemoveAll(worktreePath)
	}

	// Prune worktree references
	r.git("worktree", "prune")

	fmt.Println(successStyle.Render("Worktree '" + branch + "' removed successfully!"))
	return nil
}

// ShowPorts shows the ports that would be allocated for a branch
func (r *Repo) ShowPorts(branch string) error {
	safeName := sanitizeName(branch)
	offset := getPortOffset(safeName)

	fmt.Println(infoStyle.Render("Ports for branch:"), warnStyle.Render(branch), fmt.Sprintf("(offset +%d)", offset))
	fmt.Println()

	ports := r.detectPorts()
	if len(ports) == 0 {
		fmt.Println(warnStyle.Render("No docker-compose.yml found"))
		return nil
	}

	for _, p := range ports {
		fmt.Printf("  %s: %d\n", p.VarName, p.Default+offset)
	}

	return nil
}

// GetBranches returns all available branches (local and remote)
func (r *Repo) GetBranches() (local []string, remote []string, err error) {
	currentBranch, _ := r.currentBranch()

	// Get local branches
	out, err := exec.Command("git", "-C", r.Root, "branch", "--format=%(refname:short)").Output()
	if err != nil {
		return nil, nil, err
	}
	for _, b := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if b != "" && b != currentBranch {
			local = append(local, b)
		}
	}

	// Get remote branches
	out, err = exec.Command("git", "-C", r.Root, "branch", "-r", "--format=%(refname:short)").Output()
	if err == nil {
		localMap := make(map[string]bool)
		for _, b := range local {
			localMap[b] = true
		}
		localMap[currentBranch] = true

		for _, b := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			b = strings.TrimPrefix(b, "origin/")
			if b != "" && b != "HEAD" && !localMap[b] {
				remote = append(remote, b)
			}
		}
	}

	return local, remote, nil
}

// Helper methods

func (r *Repo) git(args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", r.Root}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *Repo) currentBranch() (string, error) {
	out, err := exec.Command("git", "-C", r.Root, "branch", "--show-current").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (r *Repo) branchExists(branch string) bool {
	err := exec.Command("git", "-C", r.Root, "show-ref", "--verify", "--quiet", "refs/heads/"+branch).Run()
	return err == nil
}

func (r *Repo) remoteBranchExists(branch string) bool {
	err := exec.Command("git", "-C", r.Root, "show-ref", "--verify", "--quiet", "refs/remotes/origin/"+branch).Run()
	return err == nil
}

func (r *Repo) ensureGitignore() error {
	gitignorePath := filepath.Join(r.Root, ".gitignore")
	content, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if strings.Contains(string(content), ".worktrees") {
		return nil
	}

	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Println(infoStyle.Render("Adding .worktrees to .gitignore..."))
	_, err = f.WriteString("\n# Git worktrees with isolated Docker environments\n.worktrees\n")
	return err
}

func (r *Repo) copyEnvFiles(worktreePath string) {
	// Copy .env if exists
	envPath := filepath.Join(r.Root, ".env")
	if _, err := os.Stat(envPath); err == nil {
		fmt.Println(infoStyle.Render("Copying .env..."))
		copyFile(envPath, filepath.Join(worktreePath, ".env"))
	} else {
		// Try .env.example
		examplePath := filepath.Join(r.Root, ".env.example")
		if _, err := os.Stat(examplePath); err == nil {
			fmt.Println(warnStyle.Render("No .env found, copying .env.example..."))
			copyFile(examplePath, filepath.Join(worktreePath, ".env"))
		}
	}
}

func (r *Repo) createEnvLocal(worktreePath, branch, projectName string, offset int, ports []PortVar) error {
	fmt.Println(infoStyle.Render("Creating .env.local with isolated configuration..."))

	var b strings.Builder
	b.WriteString("# Auto-generated by worktree-dev\n")
	b.WriteString(fmt.Sprintf("# Repository: %s\n", r.Name))
	b.WriteString(fmt.Sprintf("# Worktree: %s\n", branch))
	b.WriteString(fmt.Sprintf("# Created: %s\n\n", time.Now().Format(time.RFC3339)))
	b.WriteString("# Docker Compose project name (isolates containers, networks, and volumes)\n")
	b.WriteString(fmt.Sprintf("COMPOSE_PROJECT_NAME=%s\n\n", projectName))
	b.WriteString(fmt.Sprintf("# Port mappings (offset by %d from defaults)\n", offset))

	for _, p := range ports {
		b.WriteString(fmt.Sprintf("%s=%d\n", p.VarName, p.Default+offset))
	}

	return os.WriteFile(filepath.Join(worktreePath, ".env.local"), []byte(b.String()), 0644)
}

func (r *Repo) createDevScript(worktreePath, projectName string, offset int, ports []PortVar) error {
	var portsDisplay strings.Builder
	for _, p := range ports {
		portsDisplay.WriteString(fmt.Sprintf("    echo \"  %s: %d\"\n", p.VarName, p.Default+offset))
	}

	script := fmt.Sprintf(`#!/bin/bash
# Convenience script for this worktree
# Loads .env.local and runs docker-compose with proper isolation

set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Load environment
if [ -f "$SCRIPT_DIR/.env.local" ]; then
    set -a
    source "$SCRIPT_DIR/.env.local"
    set +a
fi

# Show help
show_help() {
    echo "Worktree dev helper for: $COMPOSE_PROJECT_NAME"
    echo ""
    echo "Commands:"
    echo "  up [services...]     Start services (default: all)"
    echo "  down                 Stop services"
    echo "  logs [service]       View logs (follows)"
    echo "  ps                   Show running containers"
    echo "  exec <svc> <cmd>     Execute command in service"
    echo "  run <svc> <cmd>      Run one-off command"
    echo "  build                Rebuild containers"
    echo "  restart [service]    Restart services"
    echo "  <any>                Passed to docker-compose"
    echo ""
    echo "Ports:"
%s}

CMD="${1:-help}"
shift 2>/dev/null || true

case "$CMD" in
    up)
        echo "Starting $COMPOSE_PROJECT_NAME..."
        docker-compose up -d "$@"
        echo ""
        echo "Services started. Ports:"
%s        ;;
    down)
        echo "Stopping $COMPOSE_PROJECT_NAME..."
        docker-compose down "$@"
        ;;
    logs)
        docker-compose logs -f "$@"
        ;;
    ps)
        docker-compose ps "$@"
        ;;
    exec)
        docker-compose exec "$@"
        ;;
    run)
        docker-compose run --rm "$@"
        ;;
    build)
        docker-compose build "$@"
        ;;
    restart)
        docker-compose restart "$@"
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        docker-compose "$CMD" "$@"
        ;;
esac
`, portsDisplay.String(), portsDisplay.String())

	scriptPath := filepath.Join(worktreePath, "dev")
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return err
	}
	return nil
}

type WorktreeInfo struct {
	Path   string
	Branch string
}

func (r *Repo) getWorktrees() ([]WorktreeInfo, error) {
	out, err := exec.Command("git", "-C", r.Root, "worktree", "list").Output()
	if err != nil {
		return nil, err
	}

	var worktrees []WorktreeInfo
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			branch := strings.Trim(parts[2], "[]")
			worktrees = append(worktrees, WorktreeInfo{
				Path:   parts[0],
				Branch: branch,
			})
		}
	}
	return worktrees, nil
}

func (r *Repo) countRunningContainers(project string) int {
	out, _ := exec.Command("docker", "ps", "--filter", "name="+project, "--format", "{{.Names}}").Output()
	if len(out) == 0 {
		return 0
	}
	return len(strings.Split(strings.TrimSpace(string(out)), "\n"))
}

func (r *Repo) dockerComposeDown(worktreePath, project string) {
	cmd := exec.Command("docker-compose", "down", "-v")
	cmd.Dir = worktreePath
	cmd.Env = append(os.Environ(), "COMPOSE_PROJECT_NAME="+project)
	cmd.Run()
}

func (r *Repo) removeContainers(project string) {
	out, _ := exec.Command("docker", "ps", "-a", "--filter", "name="+project, "--format", "{{.ID}}").Output()
	if len(out) > 0 {
		for _, id := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if id != "" {
				exec.Command("docker", "rm", "-f", id).Run()
			}
		}
	}
}

func gitRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
