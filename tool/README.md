# Teleport Binaries

Entry points for Teleport binaries. Core implementation lives in `lib/`.

- `tbot` - Machine & Workload Identity agent - see [architecture](https://goteleport.com/docs/reference/architecture/machine-id-architecture/)
- `tctl` - Admin CLI for managing cluster resources
- `teleport` - The Teleport daemon (runs Auth, Proxy, and agent services) - see [architecture](https://goteleport.com/docs/reference/architecture/agents/)
- `teleport-update` - Auto-updater for Teleport agents - see [architecture](https://goteleport.com/docs/reference/architecture/agent-update-management/)
- `tsh` - User-facing CLI for accessing resources
