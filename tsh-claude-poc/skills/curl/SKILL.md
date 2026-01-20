---
name: curl
description: run curl command against Teleport applications
user-invocable: true
allowed-tools: mcp__plugin_tsh_tsh__curl
---

## When to use this skill

Use this skill when you need to make HTTP requests to Teleport applications.

## Instructions

When making HTTP/REST API calls to Teleport apps:

1. **ALWAYS use the `curl` tool from the tsh MCP server**
2. **NEVER use plain `curl` or Bash commands**

The MCP curl tool provides authenticated access to Teleport applications with built-in policy enforcement.

## MCP Tool Parameters

- **app_name** (required): The Teleport application name (e.g., "grafana-staging-onprem", "cloud-api-dev")
- **url_path** (optional): The URL path starting with / (e.g., "/api/v1/users", "/health"). Omit to curl the root domain.
- **curl_args** (optional): Array of curl arguments (e.g., ["-X", "POST", "-H", "Content-Type: application/json", "-d", "{...}"])

## Examples

### Simple GET request
```
Tool: mcp__plugin_tsh_tsh__curl
Parameters:
{
  "app_name": "grafana-staging-onprem",
  "url_path": "/api/health"
}
```

### POST with JSON data
```
Tool: mcp__plugin_tsh_tsh__curl
Parameters:
{
  "app_name": "cloud-api-dev",
  "url_path": "/api/v1/users",
  "curl_args": ["-X", "POST", "-H", "Content-Type: application/json", "-d", "{\"name\":\"test\"}"]
}
```

### GET with headers
```
Tool: mcp__plugin_tsh_tsh__curl
Parameters:
{
  "app_name": "grafana-staging-onprem",
  "url_path": "/api/datasources",
  "curl_args": ["-H", "Accept: application/json", "-s"]
}
```

### PUT request
```
Tool: mcp__plugin_tsh_tsh__curl
Parameters:
{
  "app_name": "cloud-api-dev",
  "url_path": "/api/v1/users/123",
  "curl_args": ["-X", "PUT", "-H", "Content-Type: application/json", "-d", "{\"status\":\"active\"}"]
}
```

### DELETE request
```
Tool: mcp__plugin_tsh_tsh__curl
Parameters:
{
  "app_name": "cloud-api-dev",
  "url_path": "/api/v1/users/123",
  "curl_args": ["-X", "DELETE"]
}
```

## Policy Enforcement

A pre-tool-use hook (`tsh claude-hook`) enforces access policies before the MCP curl tool executes. The hook evaluates each curl request and returns one of three decisions:

- **allow**: Request executes immediately without user prompt
- **prompt** (ask): User approval required before execution
- **deny**: Request is blocked and will not execute

The actual policy (which apps are allowed/require approval/denied) is determined by the `tsh claude-hook` implementation.

**Example policies (for illustration):**
- `grafana-staging-onprem`: Auto-approved (executes immediately)
- `cloud-api-prod`: Requires manual approval (user prompted to confirm)
- `unknown-app`: Denied (access not permitted)

All access attempts are logged to `~/.tsh/claude-hook-audit.log` for audit purposes.

## Benefits

- **Authentication handled automatically**: tsh manages Teleport authentication
- **Policy enforcement**: Hook validates app access before execution
- **Security**: Commands are validated and logged before execution
- **Audit trail**: All curl operations are logged with timestamps

## Important Notes

- Always use the MCP curl tool, never plain curl or Bash commands
- Authentication is handled automatically by the tsh MCP server
- The preToolUse hook validates app access before the MCP tool executes
- Policy enforcement happens transparently - you'll be prompted if approval is needed
- All requests are audited and logged for compliance
