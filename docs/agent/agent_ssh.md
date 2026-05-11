<!-- SPDX-Licence-Identifier: EUPL-1.2 -->

# agent_ssh.go — SSH transport for remote checkpoint discovery

**Package**: `dappco.re/go/ml`
**File**: `go/agent_ssh.go`

## What this is

`SSHTransport` — the **SSH-based RemoteTransport** for the agent loop. Walks a remote directory, lists files newer than a watermark, pulls bytes. The production path for "scan the homelab M3 for new checkpoints".

## Config

```go
type SSHConfig struct {
    Host       string         // "homelab.lthn.sh"
    Port       int            // 22
    User       string
    PrivateKey []byte         // PEM bytes (or path read in beforehand)
    KnownHosts string         // path to known_hosts file
    Timeout    time.Duration
}

transport, err := ml.NewSSHTransport(cfg)
```

## RemoteTransport implementation

```go
transport.ListNew(ctx, "/checkpoints/vi", lastSeen) ([]string, error)
transport.Read(ctx, "/checkpoints/vi/step-1000.npz") ([]byte, error)
transport.Stat(ctx, path) (FileInfo, error)
```

`ListNew` walks the directory (one shallow listing — doesn't recurse by default) and filters by mtime > `lastSeen`. The agent loop calls this each poll with the watermark advanced from the previous round.

## Authentication

PEM-decoded private key only. No password fallback. No SSH agent forwarding (deliberate — the agent runs unattended; SSH agent prompts would block forever).

`KnownHosts` enforced for host-key verification. Setting it to `""` disables verification (test only; never production).

## Pooling

One persistent SSH session per remote host, multiplexed across `ListNew` / `Read` / `Stat` calls. Reconnects on connection drop with exponential backoff. Closes cleanly when the agent stops.

## Why SSH not HTTP

Three reasons:

1. **The homelab forge stores checkpoints on the M3's local FS, not in object storage.** SSH is the natural read path.
2. **Auth via key.** No need to set up HTTP auth + TLS for cross-machine pulls inside the homelab Tailnet.
3. **Streamable reads.** SSH SFTP supports range reads natively; HTTP would require a wrapping server.

When checkpoints live in S3 / R2 / Backblaze, swap to a different RemoteTransport. The interface is stable.

## Error model

Transport errors are typed:

- `ErrSSHAuth` — bad credentials / host-key mismatch
- `ErrSSHConn` — connection refused / timeout
- `ErrSSHFile` — path not found / permission denied
- `ErrSSHTransient` — recoverable; retry with backoff

Agents handle these distinctly — auth/conn errors halt the loop with operator alert; transient errors retry.

## Used by

- `Agent.Run()` — the orchestrator's transport field
- Vi training pipeline — pulls Vi checkpoints from `vi-trainer-m3.lthn.sh`
- LARQL inspection — pulls weights for vindex extraction

## Related

- [agent.md](agent.md) — orchestrator that owns the transport
- [agent_eval.md](agent_eval.md) — consumer of pulled checkpoint bytes
- [agent_execute.md](agent_execute.md) — one-shot execution helpers
