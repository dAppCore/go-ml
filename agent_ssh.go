package ml

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SSHCommand executes a command on M3 via SSH.
func SSHCommand(cfg *AgentConfig, cmd string) (string, error) {
	sshArgs := []string{
		"-o", "ConnectTimeout=10",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-i", cfg.M3SSHKey,
		fmt.Sprintf("%s@%s", cfg.M3User, cfg.M3Host),
		cmd,
	}
	result, err := exec.Command("ssh", sshArgs...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ssh %q: %w: %s", cmd, err, strings.TrimSpace(string(result)))
	}
	return string(result), nil
}

// SCPFrom copies a file from M3 to a local path.
func SCPFrom(cfg *AgentConfig, remotePath, localPath string) error {
	os.MkdirAll(filepath.Dir(localPath), 0755)
	scpArgs := []string{
		"-o", "ConnectTimeout=10",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-i", cfg.M3SSHKey,
		fmt.Sprintf("%s@%s:%s", cfg.M3User, cfg.M3Host, remotePath),
		localPath,
	}
	result, err := exec.Command("scp", scpArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("scp %s: %w: %s", remotePath, err, strings.TrimSpace(string(result)))
	}
	return nil
}

// SCPTo copies a local file to M3.
func SCPTo(cfg *AgentConfig, localPath, remotePath string) error {
	scpArgs := []string{
		"-o", "ConnectTimeout=10",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-i", cfg.M3SSHKey,
		localPath,
		fmt.Sprintf("%s@%s:%s", cfg.M3User, cfg.M3Host, remotePath),
	}
	result, err := exec.Command("scp", scpArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("scp to %s: %w: %s", remotePath, err, strings.TrimSpace(string(result)))
	}
	return nil
}

// fileBase returns the last component of a path.
func fileBase(path string) string {
	if i := strings.LastIndexAny(path, "/\\"); i >= 0 {
		return path[i+1:]
	}
	return path
}

// EnvOr returns the environment variable value or a fallback.
func EnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// IntEnvOr returns the integer environment variable value or a fallback.
func IntEnvOr(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	var n int
	fmt.Sscanf(v, "%d", &n)
	if n == 0 {
		return fallback
	}
	return n
}

// ExpandHome expands ~ to the user's home directory.
func ExpandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
