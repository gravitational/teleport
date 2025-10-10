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
import requests

from fastmcp import FastMCP
from fastmcp.server.dependencies import get_access_token, AccessToken
from fastmcp.server.auth.providers.jwt import JWTVerifier

# Teleport Proxy URL. If Proxy uses self-signed certificate for web address:
# export REQUESTS_CA_BUNDLE=/path/to/teleport-cert.pem
# export SSL_CERT_FILE=/path/to/teleport-cert.pem
TELEPORT_PROXY_URL = os.getenv("TELEPORT_PROXY_URL", "https://teleport.example.com")

# Optional configs.
HOST = os.getenv("MCP_HOST", "127.0.0.1")
PORT = int(os.getenv("MCP_PORT", "8000"))
TELEPORT_MCP_APP_URI = os.getenv("TELEPORT_MCP_APP_URL", f"mcp+http://{HOST}:{PORT}/mcp")

def get_teleport_cluster_name() -> str:
    return requests.get(f"{TELEPORT_PROXY_URL}/webapi/find").json().get("cluster_name")

def get_jwt_algo(jwks_uri: str) -> str:
    keys = requests.get(jwks_uri).json().get("keys") or []
    if len(keys) == 0:
        raise ValueError("JWKS keys not found")
    return keys[0].get("alg") or "ES256"

async def teleport_user_info_from_jwt() -> dict:
    "Read Teleport user info from verified JWT"
    token: AccessToken | None = get_access_token()
    if token is None:
        return {"authenticated": False}

    return {
        "authenticated": True,
        "teleport_user_name": token.claims.get("username"),
        "teleport_roles": token.claims.get("roles"),
    }

if __name__ == "__main__":
    teleport_cluster_name = get_teleport_cluster_name()
    print(f"☕ Teleport cluster: {teleport_cluster_name}")

    jwks_uri = f"{TELEPORT_PROXY_URL}/.well-known/jwks.json"
    algo = get_jwt_algo(jwks_uri)
    print(f"☕ JWT algo: {algo}")

    print(f"""🚀 Teleport app service example:
app_service:
  enabled: "yes"
  apps:
  - name: "verify-teleport-jwt"
    uri: "mcp+http://127.0.0.1:8000/mcp"
    labels:
      env: dev
    rewrite:
      headers:
      - "Authorization: Bearer {{{{internal.jwt}}}}"
""")

    verifier = JWTVerifier(
        jwks_uri=jwks_uri,
        issuer=teleport_cluster_name,  # Teleport cluster name
        audience=TELEPORT_MCP_APP_URI, # MCP app URI
        algorithm=algo,
    )
    mcp = FastMCP("Verify Teleport JWT", auth=verifier)
    mcp.tool(teleport_user_info_from_jwt)
    mcp.run(transport="http", host=HOST, port=PORT, log_level="debug")
