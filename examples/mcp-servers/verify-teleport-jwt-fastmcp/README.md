# verify-teleport-jwt-fastmcp

Teleport sends a JWT token signed with Teleport's authority with each request
to a target MCP server over the streamable HTTP or SSE transport.

This example demonstrates a MCP server built with `fastmcp` that verifies the
JWT issued by Teleport and extracts the Teleport's identity information.

To start the server:
```bash
# export TELEPORT_PROXY_URL=https://teleport.example.com
$ uv run main.py
☕ Teleport cluster: teleport.example.com
☕ JWT algo: ES256
🚀 example Teleport MCP app definition:
app_service:
  enabled: "yes"
  apps:
  - name: "verify-teleport-jwt"
    uri: "mcp+http://127.0.0.1:8000/mcp"
    labels:
      env: dev
    rewrite:
      headers:
      - "Authorization: Bearer {{internal.jwt}}"

...
Starting MCP server 'Verify Teleport JWT' with transport 'http' on http://127.0.0.1:8000/mcp  
...
```

Sample response from calling the `teleport_user_info_from_jwt` tool via
Teleport:
```json
{
  "authenticated": true,
  "teleport_user_name": "admin",
  "teleport_roles": ["access", "editor"]
}
```

References:
- [Enroll MCP servers](https://goteleport.com/docs/enroll-resources/mcp-access/)
- [Use JWT Tokens With MCP Access](https://goteleport.com/docs/enroll-resources/application-access/jwt/introduction/)
- [Configure MCP clients](https://goteleport.com/docs/connect-your-client/model-context-protocol/mcp-access/)
