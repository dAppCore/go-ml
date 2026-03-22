package ml

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	coreio "dappco.re/go/core/io"
	coreerr "dappco.re/go/core/log"
)

// RemoteTransport abstracts remote command execution and file transfer.
// Implementations may use SSH/SCP, Docker exec, or in-memory fakes for testing.
type RemoteTransport interface {
	// Run executes a command on the remote host and returns combined output.
	Run(ctx context.Context, cmd string) (string, error)

	// CopyFrom copies a file from the remote host to a local path.
	CopyFrom(ctx context.Context, remote, local string) error

	// CopyTo copies a local file to the remote host.
	CopyTo(ctx context.Context, local, remote string) error
}

// SSHTransport implements RemoteTransport using the ssh and scp binaries.
type SSHTransport struct {
	Host    string
	User    string
	KeyPath string
	Port    string
	Timeout time.Duration
}

// SSHOption configures an SSHTransport.
type SSHOption func(*SSHTransport)

// WithPort sets a non-default SSH port.
func WithPort(port string) SSHOption {
	return func(t *SSHTransport) {
		t.Port = port
	}
}

// WithTimeout sets the SSH connection timeout.
func WithTimeout(d time.Duration) SSHOption {
	return func(t *SSHTransport) {
		t.Timeout = d
	}
}

// NewSSHTransport creates an SSHTransport with the given credentials and options.
func NewSSHTransport(host, user, keyPath string, opts ...SSHOption) *SSHTransport {
	t := &SSHTransport{
		Host:    host,
		User:    user,
		KeyPath: keyPath,
		Port:    "22",
		Timeout: 10 * time.Second,
	}
	for _, o := range opts {
		o(t)
	}
	return t
}

// commonArgs returns the shared SSH options for both ssh and scp.
func (t *SSHTransport) commonArgs() []string {
	timeout := int(t.Timeout.Seconds())
	if timeout < 1 {
		timeout = 10
	}
	args := []string{
		"-o", fmt.Sprintf("ConnectTimeout=%d", timeout),
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-i", t.KeyPath,
	}
	if t.Port != "" && t.Port != "22" {
		args = append(args, "-P", t.Port)
	}
	return args
}

// sshPortArgs returns the port flag for ssh (uses -p, not -P).
func (t *SSHTransport) sshPortArgs() []string {
	timeout := int(t.Timeout.Seconds())
	if timeout < 1 {
		timeout = 10
	}
	args := []string{
		"-o", fmt.Sprintf("ConnectTimeout=%d", timeout),
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-i", t.KeyPath,
	}
	if t.Port != "" && t.Port != "22" {
		args = append(args, "-p", t.Port)
	}
	return args
}

// Run executes a command on the remote host via ssh.
func (t *SSHTransport) Run(ctx context.Context, cmd string) (string, error) {
	args := t.sshPortArgs()
	args = append(args, fmt.Sprintf("%s@%s", t.User, t.Host), cmd)

	c := exec.CommandContext(ctx, "ssh", args...)
	result, err := c.CombinedOutput()
	if err != nil {
		return "", coreerr.E("ml.SSHTransport.Run", fmt.Sprintf("ssh %q: %s", cmd, strings.TrimSpace(string(result))), err)
	}
	return string(result), nil
}

// CopyFrom copies a file from the remote host to a local path via scp.
func (t *SSHTransport) CopyFrom(ctx context.Context, remote, local string) error {
	coreio.Local.EnsureDir(filepath.Dir(local))
	args := t.commonArgs()
	args = append(args, fmt.Sprintf("%s@%s:%s", t.User, t.Host, remote), local)

	c := exec.CommandContext(ctx, "scp", args...)
	result, err := c.CombinedOutput()
	if err != nil {
		return coreerr.E("ml.SSHTransport.CopyFrom", fmt.Sprintf("scp %s: %s", remote, strings.TrimSpace(string(result))), err)
	}
	return nil
}

// CopyTo copies a local file to the remote host via scp.
func (t *SSHTransport) CopyTo(ctx context.Context, local, remote string) error {
	args := t.commonArgs()
	args = append(args, local, fmt.Sprintf("%s@%s:%s", t.User, t.Host, remote))

	c := exec.CommandContext(ctx, "scp", args...)
	result, err := c.CombinedOutput()
	if err != nil {
		return coreerr.E("ml.SSHTransport.CopyTo", fmt.Sprintf("scp to %s: %s", remote, strings.TrimSpace(string(result))), err)
	}
	return nil
}

// SSHCommand executes a command on M3 via SSH.
// Deprecated: Use AgentConfig.Transport.Run() instead.
func SSHCommand(cfg *AgentConfig, cmd string) (string, error) {
	return cfg.transport().Run(context.Background(), cmd)
}

// SCPFrom copies a file from M3 to a local path.
// Deprecated: Use AgentConfig.Transport.CopyFrom() instead.
func SCPFrom(cfg *AgentConfig, remotePath, localPath string) error {
	return cfg.transport().CopyFrom(context.Background(), remotePath, localPath)
}

// SCPTo copies a local file to M3.
// Deprecated: Use AgentConfig.Transport.CopyTo() instead.
func SCPTo(cfg *AgentConfig, localPath, remotePath string) error {
	return cfg.transport().CopyTo(context.Background(), localPath, remotePath)
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
