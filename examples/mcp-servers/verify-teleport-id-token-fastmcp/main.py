# Copyright 2025 Gravitational, Inc
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import os

from fastmcp import FastMCP
from fastmcp.server.dependencies import get_access_token, AccessToken
from fastmcp.server.auth.providers.jwt import JWTVerifier
from fastmcp.server.auth.oidc_proxy import OIDCConfiguration

# Teleport Proxy URL. If Proxy uses self-signed certificate for web address:
# export SSL_CERT_FILE=/path/to/teleport-proxy-web-cert.pem
TELEPORT_PROXY_URL = os.getenv("TELEPORT_PROXY_URL", "https://teleport.example.com")

# Optional configs.
HOST = os.getenv("MCP_HOST", "127.0.0.1")
PORT = int(os.getenv("MCP_PORT", "8000"))
TELEPORT_MCP_APP_URI = os.getenv("TELEPORT_MCP_APP_URI", f"mcp+http://{HOST}:{PORT}/mcp")

async def teleport_user_info_from_id_token() -> dict:
    "Read Teleport user info from verified ID token"
    token: AccessToken | None = get_access_token()
    if token is None:
        return {"authenticated": False}

    return {
        "authenticated": True,
        "teleport_user_name": token.claims.get("username"),
        "teleport_roles": token.claims.get("roles"),
    }

if __name__ == "__main__":
    # strict=False: Teleport's OIDC discovery only advertises ID-token issuance
    # (no authorization_endpoint / token_endpoint), so the strict validator
    # would reject it.
    oidc = OIDCConfiguration.get_oidc_configuration(
        config_url=f"{TELEPORT_PROXY_URL}/.well-known/openid-configuration",
        strict=False,
        timeout_seconds=5,
    )
    # JWTVerifier pins a single accepted algorithm rather than reading it from
    # JWKS, so pick one from the issuer's advertised list.
    algo = (oidc.id_token_signing_alg_values_supported or ["RS256"])[0]
    print(f"☕ Issuer: {oidc.issuer}")
    print(f"☕ JWKS URI: {oidc.jwks_uri}")
    print(f"☕ ID token algo: {algo}")

    print(f"""🚀 Teleport app service example:
app_service:
  enabled: "yes"
  apps:
  - name: "verify-teleport-id-token"
    uri: "mcp+http://127.0.0.1:{PORT}/mcp"
    labels:
      env: dev
    rewrite:
      headers:
      - "Authorization: Bearer {{{{internal.id_token}}}}"
""")

    verifier = JWTVerifier(
        jwks_uri=str(oidc.jwks_uri),
        issuer=str(oidc.issuer),       # https://[cluster-name]
        audience=TELEPORT_MCP_APP_URI, # MCP app URI
        algorithm=algo,
    )
    mcp = FastMCP("Verify Teleport ID Token", auth=verifier)
    mcp.tool(teleport_user_info_from_id_token)
    mcp.run(transport="http", host=HOST, port=PORT, log_level="debug")
