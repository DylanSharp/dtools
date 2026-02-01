package worktree

import (
	"hash/crc32"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// PortVar represents a port variable found in docker-compose.yml
type PortVar struct {
	VarName string
	Default int
}

// detectPorts finds port variables in docker-compose.yml
// Looks for patterns like ${DJANGO_PORT:-8000}
func (r *Repo) detectPorts() []PortVar {
	composePath := filepath.Join(r.Root, "docker-compose.yml")
	content, err := os.ReadFile(composePath)
	if err != nil {
		return nil
	}

	// Match patterns like ${VAR_NAME:-default}
	re := regexp.MustCompile(`\$\{([A-Z_]+_PORT):-(\d+)\}`)
	matches := re.FindAllStringSubmatch(string(content), -1)

	seen := make(map[string]bool)
	var ports []PortVar

	for _, match := range matches {
		if len(match) >= 3 && !seen[match[1]] {
			seen[match[1]] = true
			defaultPort, _ := strconv.Atoi(match[2])
			ports = append(ports, PortVar{
				VarName: match[1],
				Default: defaultPort,
			})
		}
	}

	return ports
}

// getPortOffset calculates a stable port offset (1-99) from a branch name
func getPortOffset(branch string) int {
	hash := crc32.ChecksumIEEE([]byte(branch))
	return int(hash%99) + 1
}

// sanitizeName converts a branch name to a safe Docker project name
func sanitizeName(name string) string {
	// Replace / with -
	name = strings.ReplaceAll(name, "/", "-")
	// Lowercase
	name = strings.ToLower(name)
	// Remove any character that isn't alphanumeric or hyphen
	re := regexp.MustCompile(`[^a-z0-9-]`)
	name = re.ReplaceAllString(name, "")
	return name
}

// getProjectPrefix creates a short prefix from the repo name
// Takes first 2 chars of each word, max 6 chars total
func getProjectPrefix(repoName string) string {
	words := strings.Split(strings.ReplaceAll(repoName, "-", " "), " ")
	var prefix strings.Builder

	for _, word := range words {
		if word == "" {
			continue
		}
		if len(word) >= 2 {
			prefix.WriteString(strings.ToLower(word[:2]))
		} else {
			prefix.WriteString(strings.ToLower(word))
		}
		if prefix.Len() >= 6 {
			break
		}
	}

	result := prefix.String()
	if len(result) > 6 {
		result = result[:6]
	}
	return result
}
