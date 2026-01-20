# Domain Validation Hook Reference

This document describes the PreToolUse hook for domain validation in the tsh plugin.

## Hook Configuration

The hook is configured in `plugin.json`:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "mcp__tsh__curl",
        "hooks": [{
          "type": "command",
          "command": "tsh claude-hook"
        }]
      }
    ]
  }
}
```

## Input Format (stdin)

The `tsh claude-hook` command receives JSON via stdin:

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

## Output Format (stdout)

### Case 1: Auto-approve

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "allow",
    "permissionDecisionReason": "cloud-api is in the auto-approved application list"
  }
}
```

Exit code: `0`

### Case 2: Require manual approval

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "ask",
    "permissionDecisionReason": "staging-api requires manual approval per security policy"
  }
}
```

Exit code: `0`

### Case 3: Deny

**Option A - Structured denial:**

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": "unknown-app is not in the allowed application list"
  }
}
```

Exit code: `0`

**Option B - Block with stderr:**

```
Application unknown-app is not allowed
```

Exit code: `2` (or any non-zero)

### Case 4: Modify input (optional)

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "allow",
    "permissionDecisionReason": "Adding required headers for cloud-api",
    "updatedInput": {
      "app_name": "cloud-api",
      "curl_args": "-X POST -H 'Content-Type: application/json' -H 'X-Request-ID: uuid-here'",
      "url_path": "/api/v1/users"
    }
  }
}
```

Exit code: `0`

## Implementation in tsh

Implement the `tsh claude-hook` subcommand to:

1. Read JSON from stdin
2. Extract `app_name` from `tool_input`
3. Look up the actual domain for the application
4. Check against your allow/ask/deny lists
5. Output the decision JSON to stdout
6. Exit with code 0 for allow/ask/deny decisions, or non-zero to block

## Testing

Create test input:

```bash
cat > test-input.json << 'EOF'
{
  "tool_name": "curl",
  "tool_input": {
    "app_name": "cloud-api",
    "curl_args": "-X GET",
    "url_path": "/api/v1/health"
  }
}
EOF
```

Test the hook:

```bash
cat test-input.json | tsh claude-hook
```

Expected output:
```json
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow","permissionDecisionReason":"cloud-api is in the auto-approved application list"}}
```

## Permission Decision Logic

Suggested logic in `tsh claude-hook`:

1. **Extract app_name** from tool_input
2. **Resolve domain** from app_name (internal mapping)
3. **Check domain against policy:**
   - If in auto-approve list → return `"allow"`
   - If in require-approval list → return `"ask"`
   - If blocked or unknown → return `"deny"` or exit with error
4. **Output decision** as JSON to stdout
5. **Exit 0** for successful processing

## Example Domain Policy

```go
var domainPolicy = map[string]string{
    "cloud-api":       "allow",      // aaa.example.com
    "staging-api":     "ask",        // bbb.example.com
    "production-api":  "ask",        // ccc.example.com
    "internal-dev":    "allow",      // dev.internal.com
}
```

## Hook Execution Flow

1. Claude decides to use `curl` tool from tsh MCP server
2. **Before** permission system, PreToolUse hook runs
3. `tsh claude-hook` command executes
4. Command receives tool input via stdin
5. Command outputs decision to stdout
6. Hook processes decision:
   - `"allow"` → Auto-approves, bypasses permission system
   - `"ask"` → Prompts user for approval
   - `"deny"` or non-zero exit → Blocks execution
7. If approved, MCP tool executes

## Key Points

- Hook runs **before** Claude Code's permission system
- Decision is final - bypasses other permission rules
- You can modify tool input before execution via `updatedInput`
- Exit code 0 with JSON = structured decision
- Non-zero exit code = block with stderr message
- The hook sees the **app_name**, not the final URL
