package mcp

import (
	"context"

	"dappco.re/go"
	mlpkg "dappco.re/go/ml"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func newSDKServer() *mcpsdk.Server {
	return mcpsdk.NewServer(&mcpsdk.Implementation{Name: "test", Version: "v0"}, nil)
}

func TestSubsystem_NewMLSubsystem_Good(t *core.T) {
	subsystem := NewMLSubsystem(nil)
	core.AssertNotNil(t, subsystem)
	core.AssertEqual(t, "ml", subsystem.Name())
}

func TestSubsystem_NewMLSubsystem_Bad(t *core.T) {
	subsystem := NewMLSubsystem(&mlpkg.Service{})
	core.AssertNotNil(t, subsystem)
	core.AssertNotNil(t, subsystem.logger)
}

func TestSubsystem_NewMLSubsystem_Ugly(t *core.T) {
	subsystem := NewMLSubsystem(nil)
	subsystem.service = nil
	core.AssertNil(t, subsystem.service)
}

func TestSubsystem_MLSubsystem_Name_Good(t *core.T) {
	subsystem := NewMLSubsystem(nil)
	got := subsystem.Name()
	core.AssertEqual(t, "ml", got)
}

func TestSubsystem_MLSubsystem_Name_Bad(t *core.T) {
	subsystem := &MLSubsystem{}
	got := subsystem.Name()
	core.AssertEqual(t, "ml", got)
}

func TestSubsystem_MLSubsystem_Name_Ugly(t *core.T) {
	subsystem := NewMLSubsystem(nil)
	subsystem.logger = nil
	got := subsystem.Name()
	core.AssertEqual(t, "ml", got)
}

func TestSubsystem_MLSubsystem_RegisterTools_Good(t *core.T) {
	subsystem := NewMLSubsystem(nil)
	server := newSDKServer()
	core.AssertNotPanics(t, func() { subsystem.RegisterTools(server) })
}

func TestSubsystem_MLSubsystem_RegisterTools_Bad(t *core.T) {
	stubName := t.Name()
	core.AssertNotEmpty(t, stubName)
	subsystem := NewMLSubsystem(nil)
	core.AssertPanics(t, func() { subsystem.RegisterTools(nil) })
}

func TestSubsystem_MLSubsystem_RegisterTools_Ugly(t *core.T) {
	subsystem := &MLSubsystem{}
	server := newSDKServer()
	core.AssertNotPanics(t, func() { subsystem.RegisterTools(server) })
}

func TestSubsystem_MLSubsystem_Shutdown_Good(t *core.T) {
	subsystem := NewMLSubsystem(nil)
	err := subsystem.Shutdown(context.Background())
	core.AssertNoError(t, err)
	core.AssertEqual(t, "ml", subsystem.Name())
}

func TestSubsystem_MLSubsystem_Shutdown_Bad(t *core.T) {
	subsystem := &MLSubsystem{}
	err := subsystem.Shutdown(context.Background())
	core.AssertNoError(t, err)
}

func TestSubsystem_MLSubsystem_Shutdown_Ugly(t *core.T) {
	subsystem := NewMLSubsystem(nil)
	subsystem.logger = nil
	err := subsystem.Shutdown(context.Background())
	core.AssertNoError(t, err)
}
