# verify-teleport-id-token-fastmcp

Teleport can sign each request to a target MCP server with an OIDC ID token
issued by Teleport's OIDC IdP CA. The signing key, issuer, and supported
algorithms are advertised through Teleport's OIDC discovery document at
`/.well-known/openid-configuration`.

This example demonstrates an MCP server built with `fastmcp` that verifies the
ID token issued by Teleport and extracts the Teleport's identity information.

To start the server:
```bash
# export TELEPORT_PROXY_URL=https://teleport.example.com
$ uv sync
$ uv run main.py
☕ Issuer: https://teleport.example.com
☕ JWKS URI: https://teleport.example.com/.well-known/jwks-oidc
☕ ID token algo: RS256
🚀 Teleport app service example:
app_service:
  enabled: "yes"
  apps:
  - name: "verify-teleport-id-token"
    uri: "mcp+http://127.0.0.1:8000/mcp"
    labels:
      env: dev
    rewrite:
      headers:
      - "Authorization: Bearer {{internal.id_token}}"

...
Starting MCP server 'Verify Teleport ID Token' with transport 'http' on http://127.0.0.1:8000/mcp
...
```

Sample response from calling the `teleport_user_info_from_id_token` tool via
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
- [Egress JWT Authentication to MCP Servers](https://goteleport.com/docs/enroll-resources/mcp-access/jwt/)
- [Configure MCP clients](https://goteleport.com/docs/connect-your-client/model-context-protocol/mcp-access/)
