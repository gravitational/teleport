# Enhanced Session Recording

Teleport standard session recordings only capture what is echoed to a terminal. 
This has inherent advantages, for example because no input is captured, Teleport
session recordings typically do not contain passwords that were ended into a terminal.

The disadvantages is that session recordings can by bypassed using several techniques.

- **Obfuscation**. For example, even though the command ` echo Y3VybCBodHRwOi8vd3d3LmV4YW1wbGUuY29tCg== | base64 --decode | sh` does not contain 
`curl http://www.example.com`, when decoded, that is what is run.
- **Shell scripts**. For example if a user uploads and executes a script, the commands 
run within the script are not captured, simply the output.
- **Terminal controls**. Terminals support a wide variety of controls including the 
ability for users to disable terminal echo. This is frequently used when requesting
 credentials. Disabling terminal echo allows commands to be run without being captured.

Furthermore, due to their unstructured nature, session recordings are difficult to 
ingest and perform monitoring/alerting on.

!!! Note:

    Enhanced Session Recording requires all parts of the Teleport system to be running
    4.2+. 

# Requirements:

## 1. Check / Patch Kernel.
Teleport 4.2 with Enhanced Session Recording requires Linux kernel 4.18 (or above) as 
well as kernel headers. 

You can check your kernel version using the `uname` command. The output should look 
something like the following.

```
$ uname -a
Linux ip-172-31-43-104.ec2.internal 4.19.72-25.58.amzn2.x86_64 x86_64 x86_64 x86_64 GNU/Linux
```
### CentOS
|               | Kernel Version         |
|---------------|------------------------|
| 8.0-1905	    |            4.18.0.80 ✅  |

!!! Note
    At release we've only production tested Enhanced Session recording with CentOS 
    7 and 8. We welcome feedback for other Operating Systems, and simply require a
    Linux kernel 4.18 (or above). Please send feedback to [ben@gravitational.com](mailto:ben@gravitational.com)

### Ubuntu     

|       |                   | Kernel Version        |
|-------|-------------------|-----------------------|
| 18.10 | Cosmic Cuttlefish | 4.18 [Patch Kernel](http://www.theubuntumaniac.com/2018/11/update-install-kernel-4191-stable-on.html)  |
| 19.04 | Disco Dingo       | 5.0 ✅ 🚧 Feature in Alpha      |
| 19.10 | Eoan Ermine       | 5.3 ✅ 🚧 Feature in Alpha      |

### Debian

|     |                     | Kernel Version            |
|-----|---------------------|---------------------------|
| 9   | Debian Stretch      | 4.9.0-6 [Patch Kernel](https://wiki.debian.org/HowToUpgradeKernel) |
| 10  | Buster              | 4.19 ✅ 🚧 Feature in Alpha       |



### Red Hat 
|                     | Kernel Version         |
|---------------------|------------------------|
| Enterprise Linux 8  |          4.18.0-147 ✅ 🚧 Feature in Alpha  |

### Amazon Linux 
We recommend using `Amazon Linux 2` to install and use Linux kernel 4.19 using
`sudo amazon-linux-extras install kernel-ng` and rebooting your instance. 

### archlinux 
|                     | Kernel Version         |
|---------------------|------------------------|
| 2019.12.01          |  5.3.13 ✅🚧 Feature in Alpha  |

## 2. Install BCC Tools 

Run the following script to download the prerequisites to build BCC tools, building LLVM and Clang targeting BPF byte code, and then building and installing BCC tools.

!!! note

    We plan to soon support installing bcc-tools from packages instead of compiling them yourself to make taking advantage of enhanced session recording easier.

**Example Script to install relevant bcc packages for CentOS**
Follow [bcc documentation](https://github.com/iovisor/bcc/blob/master/INSTALL.md#debian---source) on how to install the relevant tooling for other operating systems. 


```sh
#!/bin/bash

set -e

if [[ $EUID -ne 0 ]]; then
   echo "Please run this script as root or sudo." 
   exit 1
fi

# Create a temporary to build tooling in.
BUILD_DIR=$(mktemp -d)
cd $BUILD_DIR
echo "Building in $BUILD_DIR."

# Install Extra Packages for Enterprise Linux (EPEL)
yum install -y epel-release
yum update -y

# Install development tools.
yum groupinstall -y "Development tools"
yum install -y elfutils-libelf-devel cmake3 git bison flex ncurses-devel

# Download and install LLVM and Clang. Build them with BPF target.
curl  -LO  http://releases.llvm.org/7.0.1/llvm-7.0.1.src.tar.xz
curl  -LO  http://releases.llvm.org/7.0.1/cfe-7.0.1.src.tar.xz
tar -xf cfe-7.0.1.src.tar.xz
tar -xf llvm-7.0.1.src.tar.xz

mkdir clang-build
mkdir llvm-build

cd llvm-build
cmake3 -G "Unix Makefiles" -DLLVM_TARGETS_TO_BUILD="BPF;X86" \
  -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX=/usr ../llvm-7.0.1.src
make
make install
cd ..

cd clang-build
cmake3 -G "Unix Makefiles" -DLLVM_TARGETS_TO_BUILD="BPF;X86" \
  -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX=/usr ../cfe-7.0.1.src
make
make install
cd ..

# Install BCC.
git clone https://github.com/iovisor/bcc.git
cd bcc && git checkout v0.11.0
mkdir bcc/build; cd bcc/build
cmake3 .. -DCMAKE_INSTALL_PREFIX=/usr
make
make install

# Install is done.
rm -fr $BUILD_DIR
echo "Install is complete, try running /usr/share/bcc/tools/execsnoop to verify install."
```

## 3. Install & Configure Teleport Node 

Follow our [installation instructions](../installation.md) to install Teleport Auth, Proxy 
and Nodes. 

Setup the Teleport node with this `etc/teleport.yaml` see our [configuration file setup](../admin-guide.md#configuration) for more instructions. 


```yaml
# Example Config to be saved as etc/teleport.yaml
teleport:
  nodename: graviton-node
  auth_token: exampletoken
  auth_servers:
  - 127.0.0.1:5000
  data_dir: /var/lib/teleport
proxy_service:
  enabled: no
auth_service:
  enabled: no
ssh_service:
  enabled: yes
    enhanced_recording:
       # Enable or disable enhanced auditing for this node. Default value: false.
       enabled: true
    
       # Optional: command_buffer_size is optional with a default value of 8 pages. 
       command_buffer_size: 8

       # Optional: disk_buffer_size is optional with default value of 128 pages.
       disk_buffer_size: 128

       # Optional: network_buffer_size is optional with default value of 8 pages.
       network_buffer_size: 8

       # Optional: Controls where cgroupv2 hierarchy is mounted. Default value: 
       # /cgroup2.
       cgroup_path: /cgroup2
```

## 4. Test by logging into node via Teleport.

**Session wih Enhanced Session Recording will be marked as 'true' in the logs.**

```json
{
  "code": "T2004I",
  "ei": 23,
  "enhanced_recording": true,
  "event": "session.end",
  "interactive": true,
  "namespace": "default",
  "participants": [
    "benarent"
  ],
  "server_id": "585fc225-5cf9-4e9f-8ff6-1b0fd6885b09",
  "sid": "ca82b98d-1d30-11ea-8244-cafde5327a6c",
  "time": "2019-12-12T22:44:46.218Z",
  "uid": "83e67464-a93a-4c7c-8ce6-5a3d8802c3b2",
  "user": "benarent"
}
```

## 5. Inspect Logs
[Show new file structure]
