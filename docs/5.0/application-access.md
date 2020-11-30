---
title: Application Access Guide
description: How to set up and configure Teleport for Application access with SSO and RBAC
---

# Teleport Application Access

## Introduction
Teleport Application Access has been designed to provide secure access to internal dashboards and applications, building on Teleport's strong foundations of security and identity. You can now put applications onto the internet safely and securely.

You can secure any web application using application access:

* Internal control panels
* Wikis / Tooling that's only available on the VPN.
* Infra dashboards - Kubernetes, Grafana
* Developer tools, such as Jenkins, Gitlab or Opsgenie

## Demo

<video autoplay loop muted playsinline controls style="width:100%">
  <source src="https://goteleport.com/teleport/videos/k8s-application-access/k8s-taa.mp4" type="video/mp4">
  <source src="https://goteleport.com/teleport/videos/k8s-application-access/k8s-taa.webm" type="video/webm">
Your browser does not support the video tag.
</video>


#### Hardware Requirements
[How it works](https://gravitational.com/teleport/how-it-works/) page will give you a good overview of the setup.

#### Networking Requirements

!!! note "Why do I see SSH Ports for Application Access?"

    Teleport uses SSH reverse tunnels to connect applications to the proxy. This is why the configuration below mentions SSH ports.

|Port      .| Service    | Description
|----------|------------|-------------------------------------------
| 3023      | Proxy      | SSH port for clients who need tsh login
| 3024      | Proxy      | SSH port used to create reverse SSH tunnels from behind-firewall environments into a trusted proxy server.
| 3025      | Auth       | TLS port used by the Auth Service to serve its API to other nodes in a cluster.
| 3080      | Proxy      | HTTPS connection to authenticate `tsh` users and web users into the cluster. The same connection is used to serve a Teleport UI.

#### TLS Requirements

TLS is required to secure Teleport's Unified Access Plane and any connected applications. When setting up Teleport, the minimum requirement is a certificate for the proxy and a wildcard certificate for its sub-domain. This is where everyone will log into Teleport.

Example: `teleport.example.com` will host the Access Plane. `*.teleport.example.com` will host all of the applications e.g. `jenkins.teleport.example.com`.

Teleport supports accessing these applications on other domains if required. DNS entries must be configured and the correct certificates need to be obtained for this to work.

```yaml
proxy_service:
  enabled: "yes"
  listen_addr: 0.0.0.0:3023
  tunnel_listen_addr: 0.0.0.0:3024
  public_addr: teleport.example.com:443
  ## Example of two https keypairs.
  https_keypairs:
  - key_file: /etc/letsencrypt/live/teleport.example.com/privkey.pem
    cert_file: /etc/letsencrypt/live/teleport.example.com/fullchain.pem
  - key_file: /etc/letsencrypt/live/*.teleport.example.com/privkey.pem
    cert_file: /etc/letsencrypt/live/*.teleport.example.com/fullchain.pem
```

!!! tip "Using Certbot to obtain Wildcard Certs"

    Let's Encrypt provides free wildcard certificates. If using [certbot](https://certbot.eff.org/)
    with DNS challenge, the below script will make setup easy. Update [EMAIL] and [DOMAIN]

      ```sh
      certbot certonly --manual \
        --preferred-challenges=dns \
        --email [EMAIL] \
        --agree-tos \
        --manual-public-ip-logging-ok \
        -d "teleport.example.com, *.teleport.example.com"
      ```


## Teleport Application Service Setup

There are two options for running Teleport, once installed it can be set up with
inline options using `teleport start` or using a `teleport.yaml` config file.

### Starting Teleport

| Variable to replace | Description  |
|-|-|
| `--roles=app` | The 'app' role will set up the reverse proxy for applications |
| `--token` | A dynamic or static `app` token obtained from the root cluster |
| `--auth-server` | URL of the root cluster auth server or public proxy address |
| `--app-name` | Application name |
| `--app-uri` |  URI and Port of Application  |

```sh
teleport start --roles=app --token=xyz --auth-server=proxy.example.com:3080 \
    --app-name="example-app" \
    --app-uri="http://localhost:8080"
```

### Application Name
When picking an application name, it's important to make sure the name will make a valid sub-domain (<=63 characters, no spaces, only `A-Z a-z 0-9 _ -` allowed).

After Teleport is running, the application will be accessible at `app-name.proxy_public_addr.com` e.g. `jenkins.teleport.example.com`.  Teleport also provides the ability to override `public_addr`. e.g `jenkins.acme.com` if you configure the appropriate DNS entry to point to the Teleport proxy server.

### Starting Teleport with config file

**Define `/etc/teleport.yaml`**

| Variable to replace | Description  |
|-|-|
| `proxy_service: public_addr` | Public URL and Port for Teleport |
| `https_keypairs` | LetsEncrypt Key File ( Wildcard Cert )  |
| `-key_file` | LetsEncrypt Key File ( Wildcard Cert ) |
| `-cert_file` | LetsEncrypt Key File ( Wildcard Cert ) |
| `app_service: enabled: yes` | LetsEncrypt Key File ( Wildcard Cert ) |
| `apps: name` | LetsEncrypt Key File ( Wildcard Cert ) |

```yaml
teleport:
  data_dir: /var/lib/teleport
  auth_token: a3aff444e182cf4ee5c2f78ad2a4cc47d8a30c95747a08f8
auth_service:
  enabled: "yes"
  listen_addr: 0.0.0.0:3025
  tokens:
  - proxy,node,app:a3aff444e182cf4ee5c2f78ad2a4cc47d8a30c95747a08f8
ssh_service:
  enabled: "false"
app_service:
   enabled: yes
   # We've a small debug app that can be used to make sure application
   # Access is working correctly. It'll output JWTs so it can be useful
   # for when extending your application.
   debug_app: true
   apps:
   - name: "kubernetes-dashboard"
     # URI and port of application.
     uri: "https://localhost:3040"
     # Optional Public Addr
     public_addr: "example.com"
     # Optional Label: These can be used in combination with RBAC rules
     # to limit access to applications
     labels:
        env: "prod"
     # Optional Dynamic Labels
     commands:
     - name: "os"
       command: ["/usr/bin/uname"]
       period: "5s"
proxy_service:
  enabled: "yes"
  listen_addr: 0.0.0.0:3023
  # Public address defaults to port 3080 which doesn't
  # require Teleport starting as root
  public_addr: teleport.example.com:443
  # Example using a wildcard cert
  https_keypairs:
  - key_file: /etc/letsencrypt/live/teleport.example.com/privkey.pem
    cert_file: /etc/letsencrypt/live/teleport.example.com/fullchain.pem
  - key_file: /etc/letsencrypt/live/*.teleport.example.com/privkey.pem
    cert_file: /etc/letsencrypt/live/*.teleport.example.com/fullchain.pem
```
#### [ Optional ] Obtain a new App Token
In the above example we've hard coded a join token `a3aff444e182cf4ee5c2f78ad2a4cc47d8a30c95747a08f8`. You can use this to join apps or create a dynamic token using the tctl command below.

```bash
$ tctl tokens add --type=app
```
## Advanced Options

### Running Debug Application

For testing and debuging purposes we provide an inbuilt debugging app. This can be turned on use `debug_app: true`.

```yaml
app_service:
   enabled: yes
   debug_app: true
```
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

`https://[cluster-url:cluster-port]/web/cluster/[cluster-name]/apps`

## Logging out of Applications

When you log into an application, you'll get a certificate and login session per your defined RBAC. If you want to force logout before this period you can force a log out by hitting the `/teleport-logout` endpoint:

https://internal-app.teleport.example.com/teleport-logout


## Integrating with JWTs

### Introduction to JWTs
JSON Web Token (JWT) is an open standard that defines a secure way to transfer information between parties as a JSON Object.

For a in-depth explanation please visit [https://jwt.io/introduction/](https://jwt.io/introduction/)

Teleport JSON Web Tokens (JWT) include three sections:

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
The JWT will be sent with the header: `Teleport-Jwt-Assertion`

**Example Teleport JWT Assertion**
```json
eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJhdWQiOlsiaHR0cDovLzEyNy4wLjAuMTozNDY3OSJdLCJleHAiOjE2MDM5NDM4MDAsImlzcyI6ImF3cyIsIm5iZiI6MTYwMzgzNTc5NSwicm9sZXMiOlsiYWRtaW4iXSwic3ViIjoiYmVuYXJlbnQiLCJ1c2VybmFtZSI6ImJlbmFyZW50In0.PZGUyFfhEWl22EDniWRLmKAjb3fL0D4cTmkxEfb-Q30hVMzVhka5WB8AUsPsLPVhTzsQ6Nkk1DnXHdz6oxrqDDfumuRrDnpJpjiXj_l0D3bExrchN61enzBHxSD13VkRIqP1V6l4i8yt8kXDIBWc-QejLTodA_GtczkDfnnpuAfaxIbD7jEwF27KI4kZu7uES9LMu2iCLdV9ZqarA-6HeDhXPA37OJ3P6eVQzYpgaOBYro5brEiVpuJLr1yA0gncmR4FqmhCpCj-KmHi2vmjmJAuuHId6HZoEZJjC9IAsNlrSA4GHH9j82o7FF1F4J2s38bRy3wZv46MT8X8-QBSpg
```
## Validate JWT with JSON Web Key Set (JWKS)

Teleport provides a jwks endpoint to verify that the JWT can be trusted. This endpoint is `https://[cluster-name]:3080/.well-known/jwks.json`

_Example jwks.json_

```json
{
  "keys": [
    {
      "kty": "RSA",
      "n": "xk-0VSVZY76QGqeN9TD-FJp32s8jZrpsalnRoFwlZ_JwPbbd5-_bPKcz8o2tv1eJS0Ll6ePxRCyK68Jz2UC4V4RiYaqJCRq_qVpDQMB1sQ7p9M-8qvT82FJ-Rv-W4RNe3xRmBSFDYdXaFm51Uk8OIYfv-oZ0kGptKpkNY390aJOzjHPH2MqSvhk9Xn8GwM8kEbpSllavdJCRPCeNVGJXiSCsWrOA_wsv_jqBP6g3UOA9GnI8R6HR14OxV3C184vb3NxIqxtrW0C4W6UtSbMDcKcNCgajq2l56pHO8In5GoPCrHqlo379LE5QqpXeeHj8uqcjeGdxXTuPrRq1AuBpvQ",
      "e": "AQAB",
      "alg": "RS256"
    }
  ]
}
```


## Example Applications

As outlined in our introduction, Application Access has been designed to support two types of applications.

**Example Legacy App**</br>
A device such as a load balancer might come with a control panel, but it doesn't have any authentication and can only be access via a privileged network. These applications are supported and can extend access beyond your network.

Other example legacy apps:

+ An internal admin tool
+ Control panel for networking devices

**Example Modern App**</br>
Teleport Application Access supports all modern applications, these could be built in-house or off-the-shelf software such as Jenkins, Kubernetes Dashboard and Jupyter workbooks.

+ Kubernetes Internal Dashboard
+ Grafana
+ Jupyter notebooks
+ In-house Single Page Apps (SPA)
+ SPA with custom Json Web Token support (JWT)
