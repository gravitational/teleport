# Beams Sandbox - Local Testing Environment

A local Docker-based environment for testing Teleport Beams UX before deploying to Firecracker-based cloud infrastructure.

## What's Included

- **Claude Code CLI** - AI-powered coding assistant
- **Python 3** with pip
- **Node.js 20** (LTS) with npm
- **Developer Tools**: vim, jq, git, curl, wget

## Quick Start

```bash
# Build and run interactively
make run

# Just build the image
make build

# Run in background
make run-detached

# Connect to background instance
make shell

# Stop background instance
make stop

# See all options
make help
```

## Authentication

Export your Claude Code OAuth token before running:

```bash
export CLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-...
make run
```

## Workspace

The `workspace/` directory is mounted into the sandbox at `/workspace`, allowing you to persist files between sessions.

## Architecture Notes

### Local Testing (Current)
- Uses Docker containers for quick iteration
- Simulates the sandbox environment

### Production Deployment (Future)
- Will use Firecracker microVMs for isolation
- Convert Docker image to Firecracker-compatible rootfs:
  ```bash
  docker export <container> > rootfs.tar
  # Convert to ext4 for Firecracker
  ```
- Or use `firecracker-containerd` for OCI image support

## Development

- **Dockerfile**: Contains the sandbox image definition
- **Makefile**: Build and run automation
- **workspace/**: Persistent storage (git-ignored)
