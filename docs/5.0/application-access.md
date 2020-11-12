---
title: Application Access Guide
description: How to set up and configure Teleport for Application access with SSO and RBAC
---

# Teleport Application Access

## Introduction
Teleport Application Access has been designed to provide secure access to internal dashboards and applications. Building on Teleports strong foundations of security and identity. You can now put applications onto the internet safely and securely.

You can secure any web application using application access:

* Internal control panels
* Wikis / Tooling that's only available on the VPN.
* Infra dashboards - Kubernetes, Grafana
* Developer tools, such as Jenkins, Gitlab or Ops genie


#### Hardware Requirements
[How it works](https://gravitational.com/teleport/how-it-works/) page will give you a good overview of the setup.

#### Networking Requirements

!!! note "Why do I see SSH Ports for Application Access?"

    Teleport uses SSH reverse tunnels to connect applications to the proxy. This is why configuration below mentions SSH ports.

|Port      | Service    | Description
|----------|------------|-------------------------------------------
|3023      | Proxy      | SSH port for clients who need tsh login
|3024      | Proxy      | SSH port used to create reverse SSH tunnels from behind-firewall environments into a trusted proxy server.
|3025      | Auth       | TLS port used by the Auth Service to serve its API to other nodes in a cluster.
|443       | Proxy      | HTTPS connection to authenticate `tsh` users and web users into the cluster. The same connection is used to serve a Teleport UI.

#### TLS Requirements

TLS is required to secure Teleports Unified Access Plane and any connected applications. When setting up Teleport the minium requiremtn is a certificate for the proxy and a wild card certifcate for it's sub-domain. This is where everyone will login to Teleport.

Example: `teleport.example.com` will host the Access Plane. `*.teleport.example.com` will host all of the applications. e.g. `jenkins.teleport.example.com`. Teleport supports accessing these applications on other domains if required. Both DNS and the correct certificates need to be obtained for this to work.

!!! tip "Using Certbot to obtain Wildcard Certs"

    ```sh
    certbot certonly --manual \
      --preferred-challenges=dns \
      --email [EMAIL] \
      --server https://acme-v02.api.letsencrypt.org/directory \
      --agree-tos \
      --manual-public-ip-logging-ok \
      -d "teleport.example.com, *.teleport.example.com"
    ```

## Example Applications

As outlined in our introduction Application Access has been designed to support two types of applications.

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

## Teleport Application Service Setup

Teleport
**Define `/etc/teleport.yaml`**

| Variable to replace | Description  |
|-|-|
| `nodename` | Name of node Teleport is running on |
| `auth_token` | Static Join Token |
| `public_addr` | Public URL and Port for Teleport |
| `https_key_file` | LetsEncrypt Key File ( Wildcard Cert )  |
| `https_cert_file` | LetsEncrypt Key File ( Wildcard Cert ) |


```
teleport:
  nodename: i-083e63d0daecd1315
  data_dir: /var/lib/teleport
  auth_token: 4c7e15
  auth_servers:
  - 127.0.0.1:3025
auth_service:
  enabled: "yes"
  listen_addr: 0.0.0.0:3025
  tokens:
  - proxy,node,app:4c7e15
ssh_service:
  enabled: "false"
proxy_service:
  enabled: "yes"
  listen_addr: 0.0.0.0:3023
  web_listen_addr: 0.0.0.0:3080
  tunnel_listen_addr: 0.0.0.0:3024
  public_addr: teleport.asteroid.earth:3080
  ## Example using a wildcard cert.
  https_keypairs:
  https_keypairs:
  - key_file: /etc/letsencrypt/live/teleport.example.com/privkey.pem
  - cert_file: /etc/letsencrypt/live/teleport.example.com/fullchain.pem
  - key_file: /etc/letsencrypt/live/*.teleport.example.com/privkey.pem
  - cert_file: /etc/letsencrypt/live/*.teleport.example.com/fullchain.pem
```
#### [ Optional ] Obtain a new App Token
In the above example we've hard coded a join token 4c7e15. You can use this to join apps or create a dynamic token using the tctl command below.
```bash
$ tctl tokens add --type=app
```

| Variable to replace | Description  |
|-|-|
| `auth_servers` | Address of Auth or Proxy Service setup above |
| `auth_token` | Token used to connect to other Teleport processes  |

```yaml
teleport:
  # nodename of machine running process.
  nodename: wellington
  # The Auth token `tctl tokens add --type=app` or the static token. 4c7e15
  auth_token: "4c7e15"
  # This is the location of the Teleport Auth Server or Public Proxy
  auth_servers:
    - teleport.example.com:3080
auth_service:
  enabled: no
proxy_service:
  enabled: no
ssh_service:
   enabled: no
# The app_service is new
app_service:
   enabled: yes
    debug_app: true
   apps:
   - name: "internal-app"
     uri: "http://10.0.1.27:8000"
   - name: "kubernetes-dashboard"
     # This version requires a public_addr for all Apps, these
     #  applications should have a certificate and DNS setup
     public_addr: "example.com"
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
     public_addr: "arris.example.com"
   # Teleport Application Access can be used to proxy any HTTP Endpoint
   # Note: Name can't include any spaces
   - name: "hackernews"
     uri: "https://news.ycombinator.com"
     public_addr: "hn.example.com"
```

## Advanced Options

### Customize Public Address

```yaml
   - name: "jira"
     uri: "https://localhost:8001"
     public_addr: "jira.example.com"
```

### Deeplink to Subdirectory
Some Applications are available on a Subdirectory, examples include Kubernetes Dashboard.

```yaml
   - name: "k8s"
     uri: "http://10.0.1.60:8001/api/v1/namespaces/kube-system/services/https:kubernetes-dashboard:/proxy/#/overview?namespace=default"
     public_addr: "k8s.example.com"
```
### Rewrite
We provide simple rewrites. This is helpful for applications

```yaml
   - name: "jira"
     uri: "https://localhost:8001"
     public_addr: "jira.example.com"
     rewrite:
        # Rewrite the "Location" header on redirect responses replacing the
        # host with the public address of this application.
        redirect:
           - "localhost"
           - "jenkins.internal.dev"
```

## View Applications in Teleport

`https://[cluster-url]:3080/web/cluster/[cluster-name]/apps`

## Logging out of Applications




## Integrating with JWTs

### Introduction to JWTs
JSON Web Token (JWT) is an open standard that defines a secure way to transfer information between parties as a JSON Object.

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