# Teleport tsh Plugin for Claude Code

Access Teleport resources backed by tsh.

## Features

- **Authenticated API calls**: Make HTTP requests through Teleport-authenticated applications
- **Domain-based access control**: Automatic approval for trusted domains, manual approval for others
- **Seamless integration**: Claude automatically uses the tsh MCP `curl` tool instead of bash curl
- **Audit trail**: All API requests go through the controlled MCP server

## Structure

```
tsh-claude-poc/
├── plugin.json                          # Plugin manifest
├── mcp.json                             # MCP server configuration (localhost:23456)
├── skills/
│   └── curl.md                          # Skill: Use tsh curl tool instead of bash curl
├── hooks/
│   └── validate-domain-reference.md     # Hook reference documentation
└── README.md                            # This file
```

## Prerequisites

1. **MCP Server Running**: Ensure your tsh MCP server is running at `http://localhost:23456`
2. **tsh CLI with hook support**: Implement the `tsh claude-hook` subcommand

## Installation

Load the plugin when starting Claude Code:

```bash
claude --plugin-dir tsh-claude-poc
```

Or with absolute path:

```bash
claude --plugin-dir /Users/stevehuang/go/github.com/gravitational/cloud/tsh-claude-poc
```

## Configuration

### MCP Server

The MCP server at `http://localhost:23456` should expose the `curl` tool:

**curl tool** (required):
- `app_name`: Teleport application name
- `curl_args`: curl arguments (without command and URL)
- `url_path`: URL path or full URL (domain replaced internally)

Optional tools:
- `login`
- `status`
- `list_apps`

### Domain Validation Hook

Implement the `tsh claude-hook` subcommand in your tsh CLI.

#### Input (stdin):
```json
{
  "tool_name": "curl",
  "tool_input": {
    "app_name": "cloud-api",
    "curl_args": "-X POST -H 'Content-Type: application/json'",
    "url_path": "/api/v1/users"
  }
}
```

#### Output (stdout):
```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "allow",
    "permissionDecisionReason": "cloud-api is auto-approved"
  }
}
```

Permission decisions:
- `"allow"`: Auto-approve (bypasses permission system)
- `"ask"`: Require manual user approval
- `"deny"`: Block the request

## Usage

Start Claude with the plugin:

```bash
claude --plugin-dir tsh-claude-poc
```

Claude will automatically:
1. Use the tsh MCP `curl` tool for API calls instead of bash curl
2. Validate domains via PreToolUse hook before execution
3. Prompt for approval when accessing applications that require it

### Example Interaction

**User**: "Fetch users from cloud-api"

**Claude**: *Uses curl tool from tsh MCP server with:*
- `app_name: "cloud-api"`
- `url_path: "/api/v1/users"`

*Hook validates: cloud-api → auto-approved → executes*

**User**: "Check staging-api health"

**Claude**: *Uses curl tool from tsh MCP server with:*
- `app_name: "staging-api"`
- `url_path: "/health"`

*Hook validates: staging-api → requires approval → prompts user*

## Access Control Policy

Configure your domain policy in the `tsh claude-hook` implementation:

```go
// Example policy
var appPolicy = map[string]string{
    "cloud-api":      "allow",    // Auto-approve
    "internal-api":   "allow",    // Auto-approve
    "staging-api":    "ask",      // Require approval
    "production-api": "ask",      // Require approval
}
```

## Testing

### Test the MCP Server

```bash
curl http://localhost:23456/health
curl http://localhost:23456/tools
```

### Test the Hook

```bash
echo '{
  "tool_name": "curl",
  "tool_input": {
    "app_name": "cloud-api",
    "url_path": "/api/v1/health"
  }
}' | tsh claude-hook
```

Expected output:
```json
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow","permissionDecisionReason":"cloud-api is auto-approved"}}
```

### Test in Claude Code

```bash
cd /Users/stevehuang/go/github.com/gravitational/cloud
claude --plugin-dir tsh-claude-poc
```

Ask Claude: "Fetch /api/v1/health from cloud-api"

Claude should automatically use the `curl` tool from the tsh MCP server.

## Troubleshooting

### Plugin not loading
- Check `tsh-claude-poc/plugin.json` exists and has valid JSON
- Verify `--plugin-dir` path is correct (relative or absolute)
- Restart Claude Code with the flag

### MCP server connection fails
- Ensure server is running at `http://localhost:23456`
- Check `mcp.json` configuration
- Test with `curl http://localhost:23456/tools`

### Hook not executing
- Verify `tsh claude-hook` command exists and is executable
- Test manually: `echo '{"tool_name":"curl","tool_input":{"app_name":"test"}}' | tsh claude-hook`
- Check hook returns valid JSON and exits with 0
- Ensure matcher in `plugin.json` is correct: `"mcp__tsh__curl"`

### Hook always blocks
- Check exit code (should be 0 for allow/ask/deny)
- Verify JSON output format matches spec
- Add debug logging to `tsh claude-hook`

## Security Notes

- PreToolUse hooks run **before** permission prompts
- Hook decisions bypass the standard permission system
- Always validate input in the hook implementation
- Log all hook invocations for audit purposes
- Consider rate limiting in the hook for additional protection

## Example: Starting Claude with this Plugin

```bash
# Navigate to your cloud repository
cd /Users/stevehuang/go/github.com/gravitational/cloud

# Start Claude with the tsh plugin
claude --plugin-dir tsh-claude-poc

# Or with absolute path
claude --plugin-dir /Users/stevehuang/go/github.com/gravitational/cloud/tsh-claude-poc

# With additional settings
claude --plugin-dir tsh-claude-poc --settings ./my-settings.json
```

## References

- [Claude Code MCP Documentation](https://code.claude.com/docs/en/mcp)
- [Claude Code Hooks Documentation](https://code.claude.com/docs/en/hooks)
- [Claude Code Plugins Documentation](https://code.claude.com/docs/en/plugins)
