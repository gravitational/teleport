// Copyright 2025 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	jwtmiddleware "github.com/auth0/go-jwt-middleware/v2"
	"github.com/auth0/go-jwt-middleware/v2/validator"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"gopkg.in/go-jose/go-jose.v2"
)

type teleportCustomClaims struct {
	UserName string   `json:"username"`
	Roles    []string `json:"roles"`
}

func (c *teleportCustomClaims) Validate(context.Context) error {
	if c.UserName == "" {
		return fmt.Errorf("missing username")
	}
	return nil
}

func main() {
	teleportProxyURL := cmp.Or(os.Getenv("TELEPORT_PROXY_URL"), "https://teleport.example.com")

	// Optional configs.
	host := cmp.Or(os.Getenv("MCP_HOST"), "127.0.0.1")
	port := cmp.Or(os.Getenv("MCP_PORT"), "8000")
	mcpAppURI := cmp.Or(os.Getenv("MCP_APP_URI"), fmt.Sprintf("mcp+http://%s:%s/mcp", host, port))

	jwtValidator, err := makeJWTValidator(teleportProxyURL, mcpAppURI)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf(`🚀 Teleport app service example:
app_service:
  enabled: "yes"
  apps:
  - name: "verify-teleport-jwt"
    uri: "%s"
    labels:
      env: dev
    rewrite:
      headers:
	  - "Authorization: Bearer {{internal.jwt}}"

`, mcpAppURI)
	fmt.Printf("🏁 Starting MCP server 'Verify Teleport JWT' with transport 'http' on http://%s:%s/mcp\n", host, port)

	mcpServerHandler := mcpserver.NewStreamableHTTPServer(makeMCPServer())
	httpServer := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", host, port),
		Handler: jwtmiddleware.New(jwtValidator.ValidateToken).CheckJWT(mcpServerHandler),
	}
	if err := httpServer.ListenAndServe(); err != nil {
		log.Fatalf("failed to start the HTTP server: %v", err)
	}
}

func makeMCPServer() *mcpserver.MCPServer {
	server := mcpserver.NewMCPServer("Verify Teleport JWT", "1.0.0")
	server.AddTool(
		mcp.NewTool(
			"teleport_user_info_from_jwt",
			mcp.WithDescription("Read Teleport user info from verified JWT"),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Find jwt claim.
			claims, ok := ctx.Value(jwtmiddleware.ContextKey{}).(*validator.ValidatedClaims)
			if !ok {
				return &mcp.CallToolResult{
					Content: []mcp.Content{mcp.NewTextContent(`{"authenticated":false}`)},
				}, nil
			}
			teleportClaims := claims.CustomClaims.(*teleportCustomClaims)

			// Prepare result.
			result := struct {
				Authenticated    bool     `json:"authenticated"`
				TeleportUsername string   `json:"teleport_username"`
				TeleportRoles    []string `json:"teleport_roles"`
			}{
				TeleportUsername: teleportClaims.UserName,
				TeleportRoles:    teleportClaims.Roles,
				Authenticated:    true,
			}
			teleportClaimsInJSON, err := json.Marshal(result)
			if err != nil {
				return nil, err
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.NewTextContent(string(teleportClaimsInJSON))},
			}, nil
		},
	)
	return server
}

func makeJWTValidator(teleportProxyURL, mcpAppURI string) (*validator.Validator, error) {
	teleportClusterName, err := getTeleportClusterName(teleportProxyURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get Teleport cluster name: %v", err)
	}
	fmt.Println("☕ Teleport cluster:", teleportClusterName)

	keySet, err := getJWKS(teleportProxyURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get JWKS: %v", err)
	}
	fmt.Println("☕ JWT algo:", getJWTAlgorithm(keySet))

	jwtValidator, err := validator.New(
		func(context.Context) (interface{}, error) {
			return keySet, nil
		},
		getJWTAlgorithm(keySet),
		teleportClusterName, // Issuer is Teleport cluster name.
		[]string{mcpAppURI}, // Audience is MCP app URI.
		validator.WithCustomClaims(func() validator.CustomClaims {
			return &teleportCustomClaims{}
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set up the validator: %v", err)
	}
	return jwtValidator, nil
}

func getJSONFromURL(url string, target any) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func getTeleportClusterName(teleportProxyURL string) (string, error) {
	var find struct {
		ClusterName string `json:"cluster_name"`
	}
	err := getJSONFromURL(teleportProxyURL+"/webapi/find", &find)
	return find.ClusterName, err
}

func getJWTAlgorithm(keySet *jose.JSONWebKeySet) validator.SignatureAlgorithm {
	for _, key := range keySet.Keys {
		return validator.SignatureAlgorithm(key.Algorithm)
	}
	return validator.ES256
}

func getJWKS(teleportProxyURL string) (*jose.JSONWebKeySet, error) {
	var jwks jose.JSONWebKeySet
	if err := getJSONFromURL(teleportProxyURL+"/.well-known/jwks.json", &jwks); err != nil {
		return nil, fmt.Errorf("failed to get JWKS: %v", err)
	}
	if len(jwks.Keys) == 0 {
		return nil, fmt.Errorf("no keys found")
	}
	return &jwks, nil
}
