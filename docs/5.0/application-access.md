---
title: Application Access Guide
description: How to set up and configure Teleport for Application access with SSO and RBAC
---

# Teleport Application Access

## Introduction
Application Access has been designed to provide secure access to internal dashboards and applications. Building on Teleports strong foundations of security and identity. You can now put applications onto the internet safely and securely.

Our team has put extra effort into support older and newer applications.

**Example Legacy App**</br>
A device such as a load balancer might come with a control panel, but it doesn't have any auth and can only be access via a privileged network. These applications are supported and can extend access beyond your network.

**Example Modern App**</br>
Teleport Application Access supports all modern applications, these could be built in-house or off-the-shelf software such as jenkins, Kubernetes Dashboard and Jupyter workbooks.

#### Hardware Requirements
We recommend reviewing our [How it works](#) page to get a good overview of how Teleport works. You'll need a small VM or even a Raspberry Pi to run Teleport (Auth and Proxy).  This will be the brains and gateway to your applications.

#### Networking Requirements

!!! note "Why do I see SSH Ports for Teleport Application Access?"

    Below is a list of ports required for Teleport Application Access to work but why SSH ports? The reason for this is Teleport uses reverse tunnels to obtain access to these applications.

|Port      | Service    | Description
|----------|------------|-------------------------------------------
|3023      | Proxy      | SSH port clients connect to used for the reverse tunnel.
|3024      | Proxy      | SSH port used to create "reverse SSH tunnels" from behind-firewall environments into a trusted proxy server.
|3025      | Auth       | SSH port used by the Auth Service to serve its API to other nodes in a cluster.
|3080      | Proxy      | HTTPS connection to authenticate `tsh` users and web users into the cluster. The same connection is used to serve a Web UI.

#### TLS Requirements

## Example Applications

As outlined in our introduction Teleport Application Access has been designed to support two types of applications.

**Example Legacy App**</br>
A device such as an load balancer might come with a control panel, but it doesn't' have any auth and can only be access via a privileged network. These applications are supported and can extend access beyond your network.

Other example legacy apps.

+ An internal admin tool
+ Control panel for networking devices

**Example Modern App**</br>
Teleport Application Access supports all modern applications, these could be built in-house or off-the-shelf software such as jenkins, Kubernetes Dashboard and Jupyter workbooks.

+ Kubernetes Internal Dashboard
+ Grafana
+ Jupyter notebooks
+ In-house Single Page Apps (SPA)
+ SPA with custom Json Web Token support (JWT)

Teleport Application Service Setup

Teleport

```yaml
teleport:
  # nodename of machine running process.
  nodename: wellington
  # The Auth token `tctl tokens add --type=app` or the static token. 4c7e15
  auth_token: "4c7e15"
  # This is the location of the Teleport Auth Server or Public Proxy
  auth_servers:
    - teleport.asteroid.earth:3080
auth_service:
  enabled: no
proxy_service:
  enabled: no
ssh_service:
   enabled: no
# The app_service is new
app_service:
   enabled: yes
   apps:
   - name: "jwt"
     # URI and Port of Application. If Teleport is installed
     uri: "http://10.0.1.27:8000"
     # This version requires a public_addr for all Apps, these
     #  applications should have a certificate and DNS setup
     public_addr: "jwt.asteroid.earth"
     # Optional Labels
     labels:
        name: "jwt"
     # Optional Dynamic Labels
     commands:
     - name: "os"
       command: ["/usr/bin/uname"]
       period: "5s"
   - name: "arris"
     uri: "http://localhost:3001"
     public_addr: "arris.asteroid.earth"
   # Teleport Application Access can be used to proxy any HTTP Endpoint
   # Note: Name can't include any spaces
   - name: "hackernews"
     uri: "https://news.ycombinator.com"
     public_addr: "hn.asteroid.earth
```

## View Applications in Teleport

`https://[cluster-url]:3080/web/cluster/[cluster-name]/apps`


## Integrating with JWTs

### Introduction to JWTs
JSON Web Token (JWT) is an open standard that defines a secure way to transfer information bewteen parties as a JSON Object.

For a in-depth explanation please visit [https://jwt.io/introduction/](https://jwt.io/introduction/)

Teleports JSON Web Token (JWT) includes three sections:

+ Header
+ Payload
+ Signature

### Header

**Example Header**
```json
{
  "alg": "RS256",
  "typ": "JWT"
}
```

### Payload

**Example Payload**

```json
{
  "aud": [
    "http://127.0.0.1:34679"
  ],
  "exp": 1603943800,
  "iss": "aws",
  "nbf": 1603835795,
  "roles": [
    "admin"
  ],
  "sub": "benarent",
  "username": "benarent"
}
```
**TODO Add info about claims etc.

Teleport will

`Teleport-Jwt-Assertion`

**Example Teleport JWT Assertion**
```json
eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJhdWQiOlsiaHR0cDovLzEyNy4wLjAuMTozNDY3OSJdLCJleHAiOjE2MDM5NDM4MDAsImlzcyI6ImF3cyIsIm5iZiI6MTYwMzgzNTc5NSwicm9sZXMiOlsiYWRtaW4iXSwic3ViIjoiYmVuYXJlbnQiLCJ1c2VybmFtZSI6ImJlbmFyZW50In0.PZGUyFfhEWl22EDniWRLmKAjb3fL0D4cTmkxEfb-Q30hVMzVhka5WB8AUsPsLPVhTzsQ6Nkk1DnXHdz6oxrqDDfumuRrDnpJpjiXj_l0D3bExrchN61enzBHxSD13VkRIqP1V6l4i8yt8kXDIBWc-QejLTodA_GtczkDfnnpuAfaxIbD7jEwF27KI4kZu7uES9LMu2iCLdV9ZqarA-6HeDhXPA37OJ3P6eVQzYpgaOBYro5brEiVpuJLr1yA0gncmR4FqmhCpCj-KmHi2vmjmJAuuHId6HZoEZJjC9IAsNlrSA4GHH9j82o7FF1F4J2s38bRy3wZv46MT8X8-QBSpg
```
## Validate JWT with JSON Web Key Set (JWKS)

Teleport provides a jwks endpoint to verify that the JWT can be trusted. This endpoint is `https://[cluster-name]:3080/.well-known/jwks.json`

This will output
```json
{
  "keys": [
    {
      "kty": "rsa",
      "n": "xk-0VSVZY76QGqeN9TD-FJp32s8jZrpsalnRoFwlZ_JwPbbd5-_bPKcz8o2tv1eJS0Ll6ePxRCyK68Jz2UC4V4RiYaqJCRq_qVpDQMB1sQ7p9M-8qvT82FJ-Rv-W4RNe3xRmBSFDYdXaFm51Uk8OIYfv-oZ0kGptKpkNY390aJOzjHPH2MqSvhk9Xn8GwM8kEbpSllavdJCRPCeNVGJXiSCsWrOA_wsv_jqBP6g3UOA9GnI8R6HR14OxV3C184vb3NxIqxtrW0C4W6UtSbMDcKcNCgajq2l56pHO8In5GoPCrHqlo379LE5QqpXeeHj8uqcjeGdxXTuPrRq1AuBpvQ",
      "e": "AQAB",
      "alg": "RS256"
    }
  ]
}
```

### How to validate the signature.

TODO