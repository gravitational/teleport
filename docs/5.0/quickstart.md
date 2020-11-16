---
title: Teleport Quick Start
description: The quick start guide for how to set up modern SSH access to cloud or edge infrastructure.
---

# Teleport Quick Start

This tutorial will guide you through the steps needed to install and run
Teleport on Linux machine(s).

### Prerequisites

* A Linux Machine. With Ports `3023-3025` and `3080` open.
* A Domain Name, DNS and TLS Certs. We'll provide examples using Let'sEncrypt
* 25 minutes to complete. 15min is waiting for DNS propagation and TLS certificates.

## Step 1: Install Teleport on a Linux Host

There are several ways to install Teleport.
Take a look at the [Teleport Installation](installation.md) page to pick the most convenient for you.

=== "yum repo / AWS Linux 2"

    ```bash
    yum-config-manager --add-repo https://rpm.releases.teleport.dev/teleport.repo
    yum install teleport

    # Optional:  Using DNF on newer distributions
    # dnf config-manager --add-repo https://rpm.releases.teleport.dev/teleport.repo
    # dnf install teleport
    ```

=== "ARM"

    === "ARMv7 (32-bit)"

        ```bash
        curl -O https://get.gravitational.com/teleport-v{{ teleport.version }}-linux-arm-bin.tar.gz
        tar -xzf teleport-v{{ teleport.version }}-linux-arm-bin.tar.gz
        cd teleport
        ./install
        ```

    === "ARM64/ARMv8 (64-bit)"

        ```bash
        curl -O https://get.gravitational.com/teleport-v{{ teleport.version }}-linux-arm64-bin.tar.gz
        tar -xzf teleport-v{{ teleport.version }}-linux-arm64-bin.tar.gz
        cd teleport
        ./install
        ```

=== "Linux Tarball"

    ```bash
    curl -O https://get.gravitational.com/teleport-v{{ teleport.version }}-linux-amd64-bin.tar.gz
    tar -xzf teleport-v{{ teleport.version }}-linux-amd64-bin.tar.gz
    cd teleport
    ./install
    ```

## Step 1b: Configure Teleport

When setting up Teleport, we recommend running it with Teleports YAML configuration file.

```bash
# Concatenate teleport.yaml using a basic demo config.
$ cat > teleport.yaml <<EOF
teleport:
    data_dir: /var/lib/teleport
auth_service:
    enabled: "yes"
    cluster_name: "teleport-quickstart"
    listen_addr: 0.0.0.0:3025
    tokens:
    - proxy,node,app:f7adb7ccdf04037bcd2b52ec6010fd6f0caec94ba190b765
ssh_service:
    enabled: "yes"
    labels:
        env: staging
proxy_service:
    enabled: "yes"
    listen_addr: 0.0.0.0:3023
    web_listen_addr: 0.0.0.0:3080
    tunnel_listen_addr: 0.0.0.0:3024
    https_keypairs:
        - key_file:
        - cert_file:
app_service:
    enabled: "yes"
    debug_app: true
EOF

# Move teleport.yaml to /etc/teleport.yaml
$  mv teleport.yaml /etc
```


## Step 1c: Configure Domain Name & Obtain TLS Certs using Let's Encrypt

Teleport requires a secure public endpoint for the Teleport UI and for end users to connect to. A domain name and TLS are required for Teleport. We'll use Let's Encrypt to obtain a free TLS certificate.

DNS Setup:<br>
For this setup, we'll simply use a `A` or `CNAME` record pointing to the IP/FQDN of the machine with Teleport installed.

TLS Setup:<br>
If you already have TLS certs you can use those certificates, or if using a new domain
we recommend using Certbot; which is free and simple to setup. Follow [certbot instructions](https://certbot.eff.org/) for how to obtain a certificate for your distro.

!!! tip "Using Certbot to obtain Wildcard Certs"

    Let's Encrypt provides free wildcard certificates. If using [certbot](https://certbot.eff.org/)
    with DNS challenge the below script will make setup easy. Replace with your email
    _foo@example.com_ and URL for Teleport _teleport.example.com_


      ```sh
      certbot certonly --manual \
        --preferred-challenges=dns \
        --email foo@example.com \
        --server https://acme-v02.api.letsencrypt.org/directory \
        --agree-tos \
        --manual-public-ip-logging-ok \
        -d "teleport.example.com, *.teleport.example.com"
      ```

**Update `teleport.yaml`<br>**
Once you've obtain the certificates from LetsEncrypt.  The below command will add
update Teleport `public_addr` and update the location of the LetsEncrypt key pairs.

Replace `teleport.example.com` with the location of your proxy.

```bash
# Replace `teleport.example.com` with your domain name.
export TELEPORT_PUBLIC_DNS_NAME="teleport.example.com"
cat >> /etc/teleport.yaml <<EOL
  public_addr: $TELEPORT_PUBLIC_DNS_NAME:3080
  https_keypairs:
    - key_file: /etc/letsencrypt/live/$TELEPORT_PUBLIC_DNS_NAME/privkey.pem
    - cert_file: /etc/letsencrypt/live/$TELEPORT_PUBLIC_DNS_NAME/fullchain.pem
EOL
```

Visit: `https://teleport.example.com:3080/`

!!! success

    Teleport is now up and running


## Step 2: Create User & Setup 2FA

Create a new user `teleport-admin`, with the Principles `root, ubuntu, ec2-user`

```
tctl users add teleport-admin root,ubuntu, ec2-user
```

Teleport will always enforces Two-Factor Authentication and support OTP and Hardware Tokens (U2F).The quickstart has been setup with OTP. For setup you'll need an OTP app.

A selection of Two-Factor Authentication apps are.

 - [Authy](https://authy.com/download/)
 - [Google Authenticator](https://www.google.com/landing/2step/)
 - [Microsoft Authenticator](https://www.microsoft.com/en-us/account/authenticator)

![Teleport User Registration](img/quickstart/login.png)

!!! info "OS User Mappings"

    The OS user `root, ubuntu, ec2-user` must exist! On Linux, if it
    does not already exist, create it with `adduser teleport`. If you do not have
    the permission to create new users on the Linux Host, run `tctl users add teleport
    <your-username> ` to explicitly map ` teleport` to an existing OS user. If you
    do not map to a real OS user you will get authentication errors later on in
    this tutorial!

![Teleport UI Dashboard](img/quickstart/teleport-nodes.png)

## Step 2a: Install Teleport Locally

=== "Mac - Homebrew"

    ```bash
    brew install teleport
    ```

=== "Windows - Powershell"

    ```bash
    curl -O teleport-v{{ teleport.version }}-windows-amd64-bin.zip https://get.gravitational.com/teleport-v{{ teleport.version }}-windows-amd64-bin.zip
    # Move `tsh` to your %PATH%
    ```

=== "Linux"

    For more options please see our [installation page](installation.md).

    ```bash
     $ curl -O https://get.gravitational.com/teleport-v{{ teleport.version }}-linux-amd64-bin.tar.gz
    $ tar -xzf teleport-v{{ teleport.version }}-linux-amd64-bin.tar.gz
    $ cd teleport
    $ ./install
    ```

## Step 3: Login Using `tsh`

`tsh` is equivalent to `ssh`, and after a while you'll be wonder where it's been all of your DevOps days.

Prior to launch you must authenticate.

=== "Local Cluster - tsh"

    ```
    tsh login --proxy=teleport.example.com:3080 --user=teleport-admin
    ```

## Step 4: Have Fun with Teleport!

### View Status

=== "tsh status"

    ```bash
    tsh status
    ```

### SSH into a node

=== "tsh ssh"

    ```
    tsh ssh root@node-name
    ```

### Add a Node to the Cluster

When you setup Teleport earlier we setup a strong static token this is `auth_token`
and the auth server will be the IP of that box. You can find this via `ifconfig -a`.

```bash
$ cat /etc/teleport.yaml
# example output
# teleport:
#   data_dir: /var/lib/teleport
#   auth_token: f7adb7ccdf04037bcd2b52ec6010fd6f0caec94ba190b765
```
Armed with these details, we'll bootstrap a new host using

=== "DEB"

    ```bash
    $ curl -O https://get.gravitational.com/teleport_5.0.0-beta.4_amd64.deb
    $ dpkg -i teleport_5.0.0-beta.4_amd64.deb
    $ which teleport
    /usr/local/bin/teleport
    ```

=== "cloud-config"

    ```ini
    #cloud-config

    package_upgrade: true

    write_files:
    - path: /etc/teleport.yaml
        content: |
            teleport:
                auth_token: "f7adb7ccdf04037bcd2b52ec6010fd6f0caec94ba190b765"
                auth_servers:
                    - "46.101-REPLACE-WITH-YOUR_IP:3025"
            auth_service:
                enabled: "false"
            ssh_service:
                enabled: "true"
                labels:
                    host: test-machine
            proxy_service:
                enabled: "false"

    runcmd:
    - 'mkdir -p /run/teleport'
    - 'cd /run/teleport && curl -O https://get.gravitational.com/teleport_5.0.0-beta.4_amd64.deb'
    - 'dpkg -i /run/teleport/teleport_5.0.0-beta.4_amd64.deb'
    - 'systemctl enable teleport.service'
    - 'systemctl start teleport.service'
    ```

## Next Steps

Congratulations! You've completed the Teleport Quickstart.

In this guide you've learned how to install Teleport on a single-node and seen a
few of the most practical features in action. When you're ready to learn how to set
up Teleport for your team, we recommend that you read our [Admin Guide](admin-guide.md)
to get all the important details. This guide will lay out everything you need to
safely run Teleport in production, including SSL certificates, security considerations,
and YAML configuration.

### Guides

If you like to learn by doing, check out our collection of step-by-step guides for
common Teleport tasks.

* [Install Teleport](installation.md)
* [Share Sessions](user-manual.md#sharing-sessions)
* [Manage Users](admin-guide.md#adding-and-deleting-users)
* [Label Nodes](admin-guide.md#labeling-nodes)
* [Teleport with OpenSSH](admin-guide.md#using-teleport-with-openssh)
