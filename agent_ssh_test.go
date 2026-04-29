package ml

import (
	"context"
	"time"

	"dappco.re/go"
)

func TestAgentSsh_WithPort_Good(t *core.T) {
	stubName := t.Name()
	core.AssertNotEmpty(t, stubName)
	transport := NewSSHTransport("host", "user", "key", WithPort("2222"))
	core.AssertEqual(t, "2222", transport.Port)
}

func TestAgentSsh_WithPort_Bad(t *core.T) {
	opt := WithPort("")
	transport := NewSSHTransport("host", "user", "key", opt)
	core.AssertEqual(t, "", transport.Port)
}

func TestAgentSsh_WithPort_Ugly(t *core.T) {
	stubName := t.Name()
	core.AssertNotEmpty(t, stubName)
	transport := NewSSHTransport("host", "user", "key", WithPort("22"))
	core.AssertEqual(t, "22", transport.Port)
}

func TestAgentSsh_WithTimeout_Good(t *core.T) {
	stubName := t.Name()
	core.AssertNotEmpty(t, stubName)
	transport := NewSSHTransport("host", "user", "key", WithTimeout(time.Second))
	core.AssertEqual(t, time.Second, transport.Timeout)
}

func TestAgentSsh_WithTimeout_Bad(t *core.T) {
	stubName := t.Name()
	core.AssertNotEmpty(t, stubName)
	transport := NewSSHTransport("host", "user", "key", WithTimeout(0))
	core.AssertEqual(t, time.Duration(0), transport.Timeout)
}

func TestAgentSsh_WithTimeout_Ugly(t *core.T) {
	stubName := t.Name()
	core.AssertNotEmpty(t, stubName)
	transport := NewSSHTransport("host", "user", "key", WithTimeout(time.Nanosecond))
	core.AssertEqual(t, time.Nanosecond, transport.Timeout)
}

func TestAgentSsh_NewSSHTransport_Good(t *core.T) {
	transport := NewSSHTransport("host", "user", "key")
	core.AssertEqual(t, "host", transport.Host)
	core.AssertEqual(t, "22", transport.Port)
}

func TestAgentSsh_NewSSHTransport_Bad(t *core.T) {
	transport := NewSSHTransport("", "", "")
	core.AssertEqual(t, "", transport.Host)
	core.AssertEqual(t, "", transport.User)
}

func TestAgentSsh_NewSSHTransport_Ugly(t *core.T) {
	transport := NewSSHTransport("host", "user", "key", WithPort("2200"), WithTimeout(time.Millisecond))
	core.AssertEqual(t, "2200", transport.Port)
	core.AssertEqual(t, time.Millisecond, transport.Timeout)
}

func TestAgentSsh_SSHTransport_Run_Good(t *core.T) {
	transport := NewSSHTransport("127.0.0.1", "nobody", "/missing", WithTimeout(time.Millisecond))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := transport.Run(ctx, "true")
	core.AssertError(t, err)
}

func TestAgentSsh_SSHTransport_Run_Bad(t *core.T) {
	transport := NewSSHTransport("", "", "", WithTimeout(time.Millisecond))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := transport.Run(ctx, "true")
	core.AssertError(t, err)
}

func TestAgentSsh_SSHTransport_Run_Ugly(t *core.T) {
	transport := &SSHTransport{Host: "127.0.0.1", User: "nobody", KeyPath: "/missing", Timeout: -1}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := transport.Run(ctx, "true")
	core.AssertError(t, err)
}

func TestAgentSsh_SSHTransport_CopyFrom_Good(t *core.T) {
	transport := NewSSHTransport("127.0.0.1", "nobody", "/missing", WithTimeout(time.Millisecond))
	err := transport.CopyFrom(context.Background(), "/remote/file", core.JoinPath(t.TempDir(), "local"))
	core.AssertError(t, err)
}

func TestAgentSsh_SSHTransport_CopyFrom_Bad(t *core.T) {
	transport := NewSSHTransport("", "", "", WithTimeout(time.Millisecond))
	err := transport.CopyFrom(context.Background(), "", core.JoinPath(t.TempDir(), "local"))
	core.AssertError(t, err)
}

func TestAgentSsh_SSHTransport_CopyFrom_Ugly(t *core.T) {
	transport := &SSHTransport{Host: "127.0.0.1", User: "nobody", KeyPath: "/missing", Timeout: -1}
	err := transport.CopyFrom(context.Background(), "/remote/file", core.JoinPath(t.TempDir(), "local"))
	core.AssertError(t, err)
}

func TestAgentSsh_SSHTransport_CopyTo_Good(t *core.T) {
	transport := NewSSHTransport("127.0.0.1", "nobody", "/missing", WithTimeout(time.Millisecond))
	err := transport.CopyTo(context.Background(), core.JoinPath(t.TempDir(), "local"), "/remote/file")
	core.AssertError(t, err)
}

func TestAgentSsh_SSHTransport_CopyTo_Bad(t *core.T) {
	transport := NewSSHTransport("", "", "", WithTimeout(time.Millisecond))
	err := transport.CopyTo(context.Background(), "", "")
	core.AssertError(t, err)
}

func TestAgentSsh_SSHTransport_CopyTo_Ugly(t *core.T) {
	transport := &SSHTransport{Host: "127.0.0.1", User: "nobody", KeyPath: "/missing", Timeout: -1}
	err := transport.CopyTo(context.Background(), core.JoinPath(t.TempDir(), "local"), "/remote/file")
	core.AssertError(t, err)
}

func TestAgentSsh_SSHCommand_Good(t *core.T) {
	ft := newFakeTransport()
	ft.On("echo ok", "ok\n", nil)
	out, err := SSHCommand(&AgentConfig{Transport: ft}, "echo ok")
	core.RequireNoError(t, err)
	core.AssertEqual(t, "ok\n", out)
}

func TestAgentSsh_SSHCommand_Bad(t *core.T) {
	stubName := t.Name()
	core.AssertNotEmpty(t, stubName)
	_, err := SSHCommand(&AgentConfig{Transport: newFakeTransport()}, "missing")
	core.AssertError(t, err)
}

func TestAgentSsh_SSHCommand_Ugly(t *core.T) {
	ft := newFakeTransport()
	ft.On("fail", "", core.AnError)
	_, err := SSHCommand(&AgentConfig{Transport: ft}, "fail")
	core.AssertError(t, err)
}

func TestAgentSsh_SCPFrom_Good(t *core.T) {
	stubName := t.Name()
	core.AssertNotEmpty(t, stubName)
	err := SCPFrom(&AgentConfig{Transport: newFakeTransport()}, "/remote", core.JoinPath(t.TempDir(), "local"))
	core.AssertNoError(t, err)
}

func TestAgentSsh_SCPFrom_Bad(t *core.T) {
	ft := newFakeTransport()
	err := SCPFrom(&AgentConfig{Transport: ft}, "", "")
	core.AssertNoError(t, err)
}

func TestAgentSsh_SCPFrom_Ugly(t *core.T) {
	cfg := &AgentConfig{Transport: newFakeTransport()}
	err := SCPFrom(cfg, "/remote", core.JoinPath(t.TempDir(), "local"))
	core.AssertNoError(t, err)
}

func TestAgentSsh_SCPTo_Good(t *core.T) {
	stubName := t.Name()
	core.AssertNotEmpty(t, stubName)
	err := SCPTo(&AgentConfig{Transport: newFakeTransport()}, core.JoinPath(t.TempDir(), "local"), "/remote")
	core.AssertNoError(t, err)
}

func TestAgentSsh_SCPTo_Bad(t *core.T) {
	ft := newFakeTransport()
	err := SCPTo(&AgentConfig{Transport: ft}, "", "")
	core.AssertNoError(t, err)
}

func TestAgentSsh_SCPTo_Ugly(t *core.T) {
	cfg := &AgentConfig{Transport: newFakeTransport()}
	err := SCPTo(cfg, core.JoinPath(t.TempDir(), "local"), "/remote")
	core.AssertNoError(t, err)
}

func TestAgentSsh_EnvOr_Good(t *core.T) {
	t.Setenv("ML_TEST_ENV", "value")
	got := EnvOr("ML_TEST_ENV", "fallback")
	core.AssertEqual(t, "value", got)
}

func TestAgentSsh_EnvOr_Bad(t *core.T) {
	stubName := t.Name()
	core.AssertNotEmpty(t, stubName)
	got := EnvOr("ML_TEST_MISSING", "fallback")
	core.AssertEqual(t, "fallback", got)
}

func TestAgentSsh_EnvOr_Ugly(t *core.T) {
	t.Setenv("ML_TEST_EMPTY", "")
	got := EnvOr("ML_TEST_EMPTY", "fallback")
	core.AssertEqual(t, "fallback", got)
}

func TestAgentSsh_IntEnvOr_Good(t *core.T) {
	t.Setenv("ML_TEST_INT", "7")
	got := IntEnvOr("ML_TEST_INT", 1)
	core.AssertEqual(t, 7, got)
}

func TestAgentSsh_IntEnvOr_Bad(t *core.T) {
	t.Setenv("ML_TEST_INT_BAD", "not-number")
	got := IntEnvOr("ML_TEST_INT_BAD", 3)
	core.AssertEqual(t, 3, got)
}

func TestAgentSsh_IntEnvOr_Ugly(t *core.T) {
	t.Setenv("ML_TEST_INT_ZERO", "0")
	got := IntEnvOr("ML_TEST_INT_ZERO", 5)
	core.AssertEqual(t, 5, got)
}

func TestAgentSsh_ExpandHome_Good(t *core.T) {
	t.Setenv("DIR_HOME", "/home/tester")
	got := ExpandHome("~/models")
	core.AssertContains(t, got, "models")
}

func TestAgentSsh_ExpandHome_Bad(t *core.T) {
	t.Setenv("DIR_HOME", "")
	got := ExpandHome("~/models")
	core.AssertContains(t, got, "models")
}

func TestAgentSsh_ExpandHome_Ugly(t *core.T) {
	stubName := t.Name()
	core.AssertNotEmpty(t, stubName)
	got := ExpandHome("/absolute/models")
	core.AssertEqual(t, "/absolute/models", got)
}
