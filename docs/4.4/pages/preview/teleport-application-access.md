---
title: Teleport Application Access Preview
---

Teleport is currently beta testing an application access proxy. Teleport Application Access has been designed to secure internal web applications, letting you provide secure access while improving both visibility and control for access.

Here are a few things you might want to secure with Teleport Application Access:

- Internal Control Panels.
- Wikis / Tooling that's only available on the VPN.
- Access to the Kubernetes Dashboard.
- Developer tools. Such as Jenkins, or the Atlassian stack.

**Example Teleport Application Usage**

This diagram shows Teleport deployed into an AWS VPC. Teleport Application Access is now providing access to Jenkins and an internal dashboard. Another Teleport Application Service is running in another datacenter and dials back to the Teleport cluster. This enables users to access that other dashboard.
![Example App Access Usage](../../img/aap.svg)

## Teleport Setup

Teleport Application Access requires two processes to be run. One is the dedicated Teleport bastion host (auth/proxy service) and the other App service will proxy the applications. The App service can be run on the host of the app, or it can be put in front of it.

### Install Teleport

<Admonition type="danger">
This is currently a very early alpha build of Teleport 5.0. **DO NOT USE FOR PRODUCTION**
</Admonition>

Download Teleport Community Version

<Tabs>
<TabItem label="Teleport Community Edition">
| OPERATING SYSTEM | CHECKSUM | DOWNLOAD LINK |
|-|-|-|
| Linux 32-bit | [SHA256](https://get.gravitational.com/teleport-v5.0.0-beta.10-linux-386-bin.tar.gz.sha265) | [teleport-v5.0.0-beta.10-linux-386-bin.tar.gz](https://get.gravitational.com/teleport-v5.0.0-beta.10-linux-386-bin.tar.gz) |
| Linux 64-bit | [SHA256](https://get.gravitational.com/teleport-v5.0.0-beta.10-linux-amd64-bin.tar.gz.sha256) | [teleport-v5.0.0-beta.10-linux-amd64-bin.tar.gz](https://get.gravitational.com/teleport-v5.0.0-beta.10-linux-amd64-bin.tar.gz) |
| Linux 64-bit (RHEL/CentOS 6.x compatible) | [SHA256](https://get.gravitational.com/teleport-v5.0.0-beta.10-linux-amd64-bin.tar.gz.sha256) | [teleport-v5.0.0-beta.10-linux-amd64-bin.tar.gz](https://get.gravitational.com/teleport-v5.0.0-beta.10-linux-amd64-bin.tar.gz) |
| Linux ARMv7 | [SHA256](https://get.gravitational.com/teleport-v5.0.0-beta.10-linux-arm-bin.tar.gz.sha256) | [teleport-v5.0.0-beta.10-linux-arm-bin.tar.gz](https://get.gravitational.com/teleport-v5.0.0-beta.10-linux-arm-bin.tar.gz) |
| Linux 64-bit DEB | [SHA256](https://get.gravitational.com/teleport_5.0.0-beta.10_amd64.deb.sha256) | [teleport_v5.0.0-beta.10_amd64.deb](https://get.gravitational.com/teleport_5.0.0-beta.10_amd64.deb) |
| Linux 32-bit DEB | [SHA256](https://get.gravitational.com/teleport_5.0.0-beta.10_i386.deb.sha256) | [teleport_v5.0.0-beta.10_i386.deb](https://get.gravitational.com/teleport_5.0.0-beta.10_amd64.deb) |
| Linux 64-bit RPM | [SHA256](https://get.gravitational.com/teleport-5.0.0-beta.10-1.x86_64.rpm.sha256) | [teleport-5.0.0-beta.10-1.x86_64.rpm](https://get.gravitational.com/teleport-5.0.0-beta.10-1.x86_64.rpm)
| Docker Image | [SHA256](https://quay.io/repository/gravitational/teleport?tab=tags) | `docker pull quay.io/gravitational/teleport:5.0.0-beta.10` |
</TabItem>
<TabItem label="Teleport Enterprise Edition">
| OPERATING SYSTEM | CHECKSUM | DOWNLOAD LINK |
|-|-|-|
| Linux 32-bit | [SHA256](https://get.gravitational.com/teleport-ent-v5.0.0-beta.10-linux-386-bin.tar.gz.sha265) | [teleport-ent-v5.0.0-beta.10-linux-386-bin.tar.gz](https://get.gravitational.com/teleport-ent-v5.0.0-beta.10-linux-386-bin.tar.gz) |
| Linux 64-bit | [SHA256](https://get.gravitational.com/teleport-ent-v5.0.0-beta.10-linux-amd64-bin.tar.gz.sha256) | [teleport-ent-v5.0.0-beta.10-linux-amd64-bin.tar.gz](https://get.gravitational.com/teleport-ent-v5.0.0-beta.10-linux-amd64-bin.tar.gz) |
| Linux 64-bit (RHEL/CentOS 6.x compatible) | [SHA256](https://get.gravitational.com/teleport-ent-v5.0.0-beta.10-linux-amd64-bin.tar.gz.sha256) | [teleport-ent-v5.0.0-beta.10-linux-amd64-bin.tar.gz](https://get.gravitational.com/teleport-ent-v5.0.0-beta.10-linux-amd64-bin.tar.gz) |
| Linux ARMv7 | [SHA256](https://get.gravitational.com/teleport-ent-v5.0.0-beta.10-linux-arm-bin.tar.gz.sha256) | [teleport-ent-v5.0.0-beta.10-linux-arm-bin.tar.gz](https://get.gravitational.com/teleport-ent-v5.0.0-beta.10-linux-arm-bin.tar.gz) |
| Linux 64-bit DEB | [SHA256](https://get.gravitational.com/teleport-ent_5.0.0-beta.10_amd64.deb.sha256) | [teleport-ent_v5.0.0-beta.10_amd64.deb](https://get.gravitational.com/teleport-ent_5.0.0-beta.10_amd64.deb) |
| Linux 32-bit DEB | [SHA256](https://get.gravitational.com/teleport-ent_5.0.0-beta.10_i386.deb.sha256) | [teleport-ent_v5.0.0-beta.10_i386.deb](https://get.gravitational.com/teleport-ent_5.0.0-beta.10_i386.deb) |
| Linux 64-bit RPM | | [teleport-ent-5.0.0-beta.10-1.x86_64.rpm](https://get.gravitational.com/teleport-ent-5.0.0-beta.10-1.x86_64.rpm)
| Docker Image | [SHA256](https://quay.io/repository/gravitational/teleport-ent?tab=tags) | `docker pull quay.io/gravitational/teleport-ent:5.0.0-beta.10` |
</TabItem>
</Tabs>

Follow our standard [installation procedure](https://gravitational.com/teleport/docs/installation/).


**Define `/etc/teleport.yaml`**

| Variable to replace | Description  |
|-|-|
| `nodename` | Name of node Teleport is running on |
| `auth_token` | Static Join Token |
| `public_addr` | Public URL and Port for Teleport |
| `https_key_file` | LetsEncrypt Key File ( Wildcard Cert )  |
| `https_cert_file` | LetsEncrypt Cert File ( Wildcard Cert ) |

`teleport.yaml` is a configuration file used by Teleport. For this first example, Teleport has been set up using local storage. After this proxy service is running, we'll connect the Teleport Application Access service back to it.

```yaml
teleport:
  nodename: i-083e63d0daecd1315
  data_dir: /var/lib/teleport
  auth_token: Jl1u0rwar6bqzlg79ou0kYaWHErUqPsr
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
  public_addr: teleport.example.com:3080
  ## Example using a wildcard cert.
  https_keypairs:
  - key_file: /etc/letsencrypt/live/example.com/privkey.pem
  - cert_file: /etc/letsencrypt/live/example.com/fullchain.pem
```

#### Start Teleport

```
$ teleport start -d --config=/etc/teleport.yaml
```

#### [ Optional ] Obtain a new App Token

In the above example we've hard coded a join token `Jl1u0rwar6bqzlg79ou0kYaWHErUqPsr`. You can use this to join apps or create a dynamic token using the tctl command below.

```
$ tctl tokens add --type=app
```

## Teleport Application Service Setup

Teleport


| Variable to replace | Description  |
|-|-|
| `auth_servers` | Address of Auth or Proxy Service setup above |
| `auth_token` | Token used to connect to other Teleport processes  |

```yaml
teleport:
  # nodename of machine running process.
  nodename: wellington
  # The Auth token `tctl tokens add --type=app` or the static token configured above (Jl1u0rwar6bqzlg79ou0kYaWHErUqPsr)
  auth_token: "Jl1u0rwar6bqzlg79ou0kYaWHErUqPsr"
  # This is the location of the Teleport Auth Server or Public Proxy
  auth_servers:
    - teleport.example.com:3080
auth_service:
  enabled: no
proxy_service:
  enabled: no
ssh_service:
   enabled: no
# The app_service is a new Teleport service used to access web applications.
app_service:
   enabled: yes
   apps:
   - name: "internal-dashboard"
     # URI and Port of Application. If Teleport is installed on the host
     # this could be a loopback address
     uri: "http://10.0.1.27:8000"
     # This version requires a public_addr for all Apps, these
     # applications should have a certificate and DNS setup
     public_addr: "internal-dashboard.acme.com"
     # Optional Labels
     labels:
        name: "acme-dashboard"
     # Optional Dynamic Labels
     commands:
     - name: "os"
       command: ["/usr/bin/uname"]
       period: "5s"
   - name: "arris"
     uri: "http://localhost:3001"
     public_addr: "arris.example.com"
   # Teleport Application Access can be used to proxy any HTTP endpoint
   # Note: Name can't include any spaces
   - name: "hackernews"
     uri: "https://news.ycombinator.com"
     public_addr: "hn.example.com
```

#### Start Teleport

```
$ teleport start -d --config=/etc/teleport.yaml
```

#### Update DNS

In the above config example, I've configured a range of applications with `public_addr`. Each of these needs
to be set up in DNS using an `A`, `CNAME` or `AAAA` record pointing to the IP of the Teleport Proxy server.

For the beta, we would recommend using a wildcard cert for TLS. You can also use an individual certificate on the Teleport Main process using the new `https_keypairs` option.

```yaml
proxy_service:
  #... Example Snippet for new https_keypairs.
  https_keypairs:
  - key_file: /etc/letsencrypt/live/jwt.example.com/privkey.pem
  - cert_file: /etc/letsencrypt/live/jwt.example.com/fullchain.pem
  - key_file: /etc/letsencrypt/live/hn.example.com/privkey.pem
  - cert_file: /etc/letsencrypt/live/hn.example.com/fullchain.pem
  - key_file: /etc/certs/privkey.pem
  - cert_file: /etc/certs/fullchain.pem
```

## View Applications in Teleport

`https://[cluster-url]:3080/web/cluster/[cluster-name]/apps`

{/* <video autoPlay loop muted playsInline>
  <source src="/video/app.mp4" type="video/mp4" />
  <source src="/video/app.webm" type="video/webm" />
Your browser does not support the video tag.
</video> */}

## Product Feedback

We really value early feedback for our products. [You can easily pick a time here to have a chat on your feedback.](https://calendly.com/c/AEBVTDEDCQ2Z2YSB?month=2020-11)

### Found a bug?

Please create a [Github Issue](https://github.com/gravitational/teleport/issues/new/choose).
