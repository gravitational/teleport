# Quickstart

Welcome to the Teleport Quickstart!

This tutorial will guide you through the steps needed to install and run Teleport on a single node which could be your local machine.

[TOC]

### Prerequisites
- In this tutorial you will start a web UI which must be accessible via a web browser. If you run this tutorial on a remote machine without a GUI, first make sure that this machine's IP can be reached over the your network and that it accept incoming traffic on port `3080`.
- We recommend that you read [Teleport Basics](./concept-basics) before working through this tutorial. If you'd like to dive right in though this is the best place to start!

This guide is only meant to demonstrate how to run teleport in a sandbox or demo environment and showcase a few basic tasks you can do with Teleport. **You should not follow this guide if you want to set up Teleport in production. Instead follow the [Production Guide]("")**



## Step 1: Install Teleport

This guide installs teleport v4.1.0 on the CLI. Previous versions are documented in [Release History](https://gravitational.com/teleport/releases/)

You can download pre-built binaries from our [Downloads](https://gravitational.com/teleport/download/) page.
or you can [build it from source](https://gravitational.com/teleport/docs/admin-guide/#installing-from-source).

You can also download `.deb`, `.rpm`, and `.pkg` files from [Downloads](https://gravitational.com/teleport/download/)

```bash
$ export version=v4.1.0
$ export os=linux # 'darwin' 'linux' or 'windows'
$ export arch=amd64 # '386' 'arm' on linux or 'amd64' for all distros
# Automated way to retrieve the checksum, just append .sha256
$ curl https://get.gravitational.com/teleport-$version-$os-$arch-bin.tar.gz.sha256
[Checksum output]
$ curl -O https://get.gravitational.com/teleport-$version-$os-$arch-bin.tar.gz
$ shasum -a 256 teleport-$version-$os-$arch-bin.tar.gz 
# ensure the checksum matches the shaa256 checksum on the download page!
$ tar -xzf teleport-$version-$os-$arch-bin.tar.gz
$ cd teleport
$ ./install
# this copies teleport binaries to /usr/local/bin
$ which teleport
/usr/local/bin/teleport
```

## Step 2: Start Teleport

First, create a directory for Teleport
to keep its data. By default it's `/var/lib/teleport`. Then start the `teleport` daemon:

```bash
$ mkdir -p /var/lib/teleport
```

Now we are ready to start Teleport. 

_**Tip**: Avoid suspending your current shell session by running the process in the background like so: `teleport start > teleport.log 2>&1 &`. Access the process logs with `less teleport.log`._

```bash
$ teleport start # if you are not `root` you may need `sudo`
```

By default Teleport services bind to 0.0.0.0. If you ran teleport without any configuration or flags you should see this output in your console or logfile
```
[AUTH]  Auth service is starting on 0.0.0.0:3025
[PROXY] Reverse tunnel service is starting on 0.0.0.0:3024
[PROXY] Web proxy service is starting on 0.0.0.0:3080
[PROXY] SSH proxy service is starting on 0.0.0.0:3023
[SSH]   Service is starting on 0.0.0.0:3022
```

Congratulations - you are now running Teleport!

## Step 3: Create a User Signup Token

We've got Teleport running but there are no users recognized by Teleport Auth yet. Let's create one for your OS user. In this example the OS user is `teleport` and the hostname of the node is `grav-00`

```bash
[teleport@grav-00 ~]$ tctl users add teleport
Signup token has been created and is valid for 1 hours. Share this URL with the user:
https://grav-00:3080/web/newuser/3a8e9fb6a5093a47b547c0f32e3a98d4

NOTE: Make sure grav-00:3080 points at a Teleport proxy which users can access.
```

By default a new Teleport user will be assigned a mapping to an OS user of the same name. We now have a mapping between Teleport User `teleport` and OS-user `teleport`. If you want to map to a different OS user, `electric` for instance, you can specify another argument like so: `tctl users add teleport electric`

You now have a signup token for the Teleport User `teleport` and will need to open this URL in a web browser to complete the registration process.

## Step 4: Register a User

- If the machine where you ran these commands has a web browser installed you should be able to open the URL and connect to Teleport Proxy right away. 
- If the are working on a remote machine you may need to access the Teleport Proxy via the host machine and port `3080` in a web browser. One simple way to do this is to temporarily append `[HOST_IP] grav-00` to `/etc/hosts`

!!! warning "Warning":
    We haven't provisioned any SSL certs for Teleport yet. Your browser will
    throw a warning **Your connection is not private**. Click Advanced, and **Proceed to [HOST_IP] (unsafe)** to preview the Teleport UI.   

<!-- Link to networking/production guide -->

![Teleport User Registration](/img/login.png?style=grv-image-center-md)

Teleport enforces two-factor authentication by default <!-- Link to Configuration -->. If you do not already have [Google Authenticator](https://en.wikipedia.org/wiki/Google_Authenticator), [Authy](https://www.authy.com/) or another 2FA client installed, you will need to install it on your smart phone. Then you can scan the bar code on the Teleport login web page, pick a password and enter the two-factor token.

After completing registration you will be logged in automatically

![Teleport UI Dashboard](/img/ui-dashboard.png?style=grv-image-center-md)

Try out some of the dashboard features
- Login to the node via the web browser with "Login as"
- Playback the Session

## Step 5: Log in through the CLI

Let's login using the `tsh` command line tool. Just as in user the previous step, you will need to be able to resolve the **hostname** of the cluster to a network accessible IP.

!!! warning "Warning":
    For the purposes of this quickstart we are using the `--insecure` flag which allows us to skip configuring the HTTP/TLS certificate for Teleport proxy.

    Never use `--insecure` in production unless you terminate SSL at a load balancer. You must configure a HTTP/TLS certificate for the Proxy.

```bash
[teleport@grav-00 ~]$ tsh --proxy=grav-00 --insecure login
WARNING: You are using insecure connection to SSH proxy https://grav-00:3080
Enter password for Teleport user teleport:
Enter your OTP token:
XXXXXX
WARNING: You are using insecure connection to SSH proxy https://grav-00:3080
> Profile URL:  https://grav-00:3080
  Logged in as: teleport
  Cluster:      grav-00
  Roles:        admin*
  Logins:       teleport
  Valid until:  2019-10-05 02:01:36 +0000 UTC [valid for 12h0m0s]
  Extensions:   permit-agent-forwarding, permit-port-forwarding, permit-pty


* RBAC is only available in Teleport Enterprise
  https://gravitational.com/teleport/docs/enterprise
```

## Step 6: Start A Recorded Session

At this point you have authenticated with Teleport Auth and can now start a recorded SSH session.

```bash
$ tsh ssh --proxy=grav-00 grav-00
```
_Note: The `tsh` client always requires the `--proxy` flag_

You command prompt may not look different, but you are now in a new SSH session which has been authenticated by Teleport!

Try a few things to get familiar with recorded sessions:

- Navigate to `https://[HOSTNAME]:3080/web/sessions` in your web browser to see the list of sessions.
- After you end the session you can replay them here
- Try joining the session in the web browser by clicking the orange "join" button

<!-- TODO: Video showing shared session between browser/CLI -->

## Step 7: Join a Session on the CLI





This will apply to most cloud providers (AWS, GCP and Azure).

This process has been made easier with Let's Encrypt. [See instructions here](https://gravitational.com/blog/letsencrypt-teleport-ssh/).


