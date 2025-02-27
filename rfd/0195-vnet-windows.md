---
authors: Nic Klaassen (nic@goteleport.com)
state: draft
---

# RFD 195 - Windows VNet

## Required Approvers

* Engineering: @ravicious && (@zmb3 || @rosstimothy)
* Security: doyensec

## What

This document outlines the design for [VNet](0163-vnet.md) on Windows clients.

## Why

The design for Windows differs significantly from VNet on MacOS.
On MacOS we use a Launch Daemon running as root to create a virtual TUN network
interface, and then pass the TUN to a client process (Connect or tsh) over a pipe.
The client process then manages all networking and the daemon handles OS
configuration.

On Windows, there is no way to pass the TUN interface over a pipe, so all
networking is handled in a Windows Service, with IPC to the user process which
handles all Teleport client methods.
The client process retains control of all Teleport client methods and user keys
so that it can easily perform MFA prompts and hardware key signatures using
existing code.

## Details

### UX

The goal is to keep the VNet UX identical on Windows and MacOS.
Teleport Connect will have support for VNet with the best UX for details like
ease of installation, MFA prompts, and error reporting.
`tsh vnet` will be supported on Windows just as it is on MacOS, but it is
inherently more difficult to build a first-class CLI UX that includes MFA
prompts and this will not be a goal, Connect will be the recommended client.

In keeping with the MacOS experience, this design avoids administrator (UAC)
prompts each time VNet is started, the goal is to only require admin rights
during installation.
Errors and any user prompts will be identical on each OS, with a goal of reusing
as much code between operating systems as possible so that this happens by
default.

### Virtual Network Interface

VNet requires a virtual network interface to be created on the host OS,
the specific kind of interface we use is typically called a TUN interface.
VNet configures IP routes on the host so that IP traffic for a set of configured
CIDR ranges are bound to the TUN.
VNet then reads and writes IP packets to and from the TUN interface to handle
TCP connections to Teleport apps and UDP connections to VNet's internal DNS server.

Because Windows does not natively support TUN interfaces the same way that MacOS
and Linux do, we will leverage an open-source driver wintun.dll to provide the
TUN interface.
Wintun is available for download from https://www.wintun.net/ as a signed DLL.
It has a custom license that permits commercial use as long as we use their
signed DLL via the provided API.
Another open-source library that we already use on MacOS provides a common Go
interface for the TUN, this is `golang.zx2c4.com/wireguard/tun`.

The signed `wintun.dll` file will be distributed with Connect and installed in
the same directory as the `tsh` Windows service.

### Windows Service

Creating the TUN interface requires administrator rights on Windows.
To create the TUN without requiring the user to run Connect as administrator or
requiring a UAC prompt each time VNet starts, we will install and run VNet as a
Windows Service.
This is a similar concept to the Launch Daemon that we use on MacOS.

The actual exe that runs the service will be `tsh`, it will be started with a
specific argument to run the VNet Windows service.
Windows Services are installed and controlled by the Service Control Manager (SCM).
In the Connect installer (built with electron-builder), we will include a
script to install the service with `sc.exe` (this is the SCM binary).

We will install the Service with a security descriptor that allows all
authenticated users to launch the service without elevated privileges.
Each time VNet starts, it will start the service with a request to the SCM.
The service will handle Stop, Shutdown, and Interrogate requests from the SCM.

When installing a Windows service you configure a path to the .exe that
implements the service. This *must not* be a path the non-admin users can write
to.
If a user has the ability to write a different .exe to this path, they would be
allowed to launch that .exe as a service with elevated privileges, this could be
used for a privilege escalation attack.
Currently when Connect is installed on Windows by a specific user `tsh.exe` is
installed to `C:\Users\<username>\AppData\Local\Programs\teleport connect\resources\bin\tsh.exe`
Since this is user-writable, we will switch the Connect installer to do a
per-machine install which writes all binaries to
`C:\Program Files\Teleport Connect`, which is not user-writable.

### Inter-process Communication (IPC)

On MacOS we were able to create the TUN interface in the admin process, and then
pass the file descriptor over a unix socket to the user process, where the user
process then handled all networking and most VNet code except for OS
configuration that required root privileges.
Because Windows does support cloning file handles for use in other processes, I
had hoped a similar approach would work.
However, on Windows the TUN interface is not a file, and it seems impossible for
an unprivileged process to interact with the TUN interface provided by
wintun.dll.
This means that all networking code will have to run in the Windows Service.

This leaves a decision to make about how much code we can or should move into
the Windows Service, or leave in the user process.
Because Connect's `tshd` has special caching of Teleport clients and handling
for hardware keys, I am opting to keep everything that deals with a Teleport
client or a user private key in the user process, and to have the Windows
Service handle all networking and OS configuration.

VNet's networking stack is already blind to Teleport app specifics, and accepts an
interface `tcpHandlerResolver` that resolves fully-qualified domain names to TCP
connection handlers.
This is currently implemented by `vnet.tcpAppResolver` which itself accepts an
interface `AppProvider` that is implemented for both Connect and `tsh`.
The combination of `tcpAppResolver` and `AppProvider` handles all Teleport-app
specific code for listing clusters and apps, logging into apps and re-issuing
certificates, and proxying TCP connections.
My plan is to restructure this `AppProvider` interface so that it can be
implemented locally in-process on MacOS as it is today, OR implemented by a gRPC
client that dials to a gRPC server running in the user process that exposes the
local AppProvider implementation as a service.

The gRPC service will run in the user process (`tshd` or `tsh`) and accept mTLS
connections from the Windows service.
The user process will listen on any free TCP port on localhost, and pass that
address to the Windows Service as an argument when it launches the service.
The connection will use mTLS in a similar way to how the Connect UI process sets
up mTLS for connections to the `tshd` gRPC service.
Each time the user process starts it will:
1. Create a self-signed x509 CA with a new ECDSA key generated in-memory.
1. Use the CA to issue a server certificate for itself and a client certificate
   for the Windows service, both with unique generated ECDSA keys.
1. Write the CA certificate, the client certificate, and the client key to a
   path which is only readable by privileged users (the Windows Service can read
   this).
1. Listen on a free TCP port on localhost.
1. Start the Windows service and pass the listen address and mTLS credential paths as
   arguments.
1. Configure the gRPC server to use the server key/cert, and only accept mTLS
   connections from client certificates signed by the CA.

### Security

When the user process starts the Windows service, it trusts that the service was
installed by an administrative user, as all services must be.
The service binary is installed to a path under `C:\Program Files\` that can
only be written to by administrative users.
It also trusts that incoming gRPC connections are coming from a process with
administrative rights, because it was able to read the certificate and key from
the filesystem where they were configured to only be readable by admin users.

We don't want any user process on the machine to be able to start the Windows
Service and influence the host networking configuration, so the first thing the
Windows Service will do before starting any networking or configuring the OS in
any way, is call an `AuthenticateProcess` RPC which will be used to authenticate
the user process to the Windows Service.

When calling the `AuthenticateProcess` RPC, the Windows service will:
1. Create a Windows named pipe and give the user process permission to open the pipe.
1. Pass the name of the pipe (via the RPC) to the user process.
1. Wait for the user process to dial the named pipe.
1. Use the Windows API `GetNamedPipeClientProcessId` to get the pipe client
   process handle.
1. Once it has the user process handle, the Windows service can confirm that
   it's own exe is identical to the exe of the cleint application connecting to
   it.

### Privacy

There are no new privacy considerations on Windows.

### Proto Specification

```protobuf
// ClientApplicationService is a service the VNet client applications provide to
// the VNet admin process to facilate app queries, certificate issuance,
// metrics, error reporting, and signatures.
service ClientApplicationService {
  // AuthenticateProcess mutually authenticates client applicates to the admin
  // service.
  rpc AuthenticateProcess(AuthenticateProcessRequest) returns (AuthenticateProcessResponse);
  // Ping is used by the admin process to regularly poll that the client
  // application is still running.
  rpc Ping(PingRequest) returns (PingResponse);
  // ResolveAppInfo returns info for the given app fqdn, or an error if the app
  // is not present in any logged-in cluster.
  rpc ResolveAppInfo(ResolveAppInfoRequest) returns (ResolveAppInfoResponse);
  // ReissueAppCert issues a new app cert.
  rpc ReissueAppCert(ReissueAppCertRequest) returns (ReissueAppCertResponse);
  // SignForApp issues a signature with the private key associated with an x509
  // certificate previously issued for a requested app.
  rpc SignForApp(SignForAppRequest) returns (SignForAppResponse);
  // OnNewConnection gets called whenever a new connection is about to be
  // established through VNet for observability.
  rpc OnNewConnection(OnNewConnectionRequest) returns (OnNewConnectionResponse);
  // OnInvalidLocalPort gets called before VNet refuses to handle a connection
  // to a multi-port TCP app because the provided port does not match any of the
  // TCP ports in the app spec.
  rpc OnInvalidLocalPort(OnInvalidLocalPortRequest) returns (OnInvalidLocalPortResponse);
}

// AuthenticateProcessRequest is a request for AuthenticateProcess.
message AuthenticateProcessRequest {
  // version is the admin process version.
  string version = 1;
  // pipe_path is the path to a named pipe used for process authentication.
  string pipe_path = 2;
}

// AuthenticateProcessResponse is a response for AuthenticateProcess.
message AuthenticateProcessResponse {
  // version is the client application version.
  string version = 1;
}

// PingRequest is a request for the Ping rpc.
message PingRequest {}

// PingResponse is a response for the Ping rpc.
message PingResponse {}

// ResolveAppInfoRequest is a request for ResolveAppInfo.
message ResolveAppInfoRequest {
  // fqdn is the fully-qualified domain name of the app.
  string fqdn = 1;
}

// ResolveAppInfoResponse is a response for ResolveAppInfo.
message ResolveAppInfoResponse {
  // app_info holds all necessary info for making connections to the resolved app.
  AppInfo app_info = 1;
}

// AppInfo holds all necessary info for making connections to VNet TCP apps.
message AppInfo {
  // app_key uniquely identifies a TCP app (and optionally a port for multi-port
  // TCP apps).
  AppKey app_key = 1;
  // cluster is the name of the cluster in which the app is found.
  // Iff the app is in a leaf cluster, this will match app_key.leaf_cluster.
  string cluster = 2;
  // app is the app spec.
  types.AppV3 app = 3;
  // ipv4_cidr_range is the CIDR range from which an IPv4 address should be
  // assigned to the app.
  string ipv4_cidr_range = 4;
  // dial_options holds options that should be used when dialing the root cluster
  // of the app.
  DialOptions dial_options = 5;
}

// AppKey uniquely identifies a TCP app in a specific profile and cluster.
message AppKey {
  // profile is the profile in which the app is found.
  string profile = 1;
  // leaf_cluster is the leaf cluster in which the app is found. If empty, the
  // app is in the root cluster for the profile.
  string leaf_cluster = 2;
  // name is the name of the app.
  string name = 3;
}

// DialOptions holds ALPN dial options for dialing apps.
message DialOptions {
  // web_proxy_addr is the address to dial.
  string web_proxy_addr = 1;
  // alpn_conn_upgrade_required specifies if ALPN connection upgrade is required.
  bool alpn_conn_upgrade_required = 2;
  // sni is a ServerName value set for upstream TLS connection.
  string sni = 3;
  // insecure_skip_verify turns off verification for x509 upstream ALPN proxy service certificate.
  bool insecure_skip_verify = 4;
  // root_cluster_ca_cert_pool overrides the x509 certificate pool used to verify the server.
  bytes root_cluster_ca_cert_pool = 5;
}

// ReissueAppCertRequest is a request for ReissueAppCert.
message ReissueAppCertRequest {
  // app_info contains info about the app, every ReissueAppCertRequest must
  // include an app_info as returned from ResolveAppInfo.
  AppInfo app_info = 1;
  // target_port is the TCP port to issue the cert for.
  uint32 target_port = 2;
}

// ReissueAppCertResponse is a response for ReissueAppCert.
message ReissueAppCertResponse {
  // cert is the issued app certificate in x509 DER format.
  bytes cert = 1;
}

// SignForAppRequest is a request to sign data with a private key that the
// server has cached for the (app_key, target_port) pair. The (app_key,
// target_port) pair here must match a previous successful call to
// ReissueAppCert. The private key used for the signature will match the subject
// public key of the issued x509 certificate.
message SignForAppRequest {
  // app_key uniquely identifies a TCP app, it must match the key of an app from
  // a previous successful call to ReissueAppCert.
  AppKey app_key = 1;
  // target_port identifies the TCP port of the app, it must match the
  // target_port of a previous successful call to ReissueAppCert for an app
  // matching AppKey.
  uint32 target_port = 2;
  // digest is the bytes to sign.
  bytes digest = 3;
  // hash is the hash function used to compute digest.
  Hash hash = 4;
}

// Hash specifies a cryptographic hash function.
enum Hash {
  HASH_UNSPECIFIED = 0;
  HASH_NONE = 1;
  HASH_SHA256 = 2;
}

// SignForAppResponse is a response for SignForApp.
message SignForAppResponse {
  // signature is the signature.
  bytes signature = 1;
}

// OnNewConnectionRequest is a request for OnNewConnection.
message OnNewConnectionRequest {
  // app_key identifies the app the connection is being made for.
  AppKey app_key = 1;
}

// OnNewConnectionRequest is a response for OnNewConnection.
message OnNewConnectionResponse {}

// OnInvalidLocalPortRequest is a request for OnInvalidLocalPort.
message OnInvalidLocalPortRequest {
  // app_info identifies the app the request was made for. AppInfo is used
  // instaed of AppKey so that the application spec is included, which includes
  // the TCP port ranges allowed for the app, which are ultimately included in
  // the user error message.
  AppInfo app_info = 1;
  // target_port is the invalid port the request was made for.
  uint32 target_port = 2;
}

// OnInvalidLocalPortResponse is a response for OnInvalidLocalPort.
message OnInvalidLocalPortResponse {}
```

### Backward Compatibility

The Windows Service will be updated in lockstep with the client
(Connect/tshd/tsh) by the Connect installer, so there are not really any
backward compatibility concerns, even with the gRPC API.
During the `AuthenticateProcess` rpc we will double-check that each binary is
running the same version of Teleport, and fail to start VNet is a user-visible
error if they are somehow on different versions.
This should not occur, but if it does, we will recommend the user to re-install
Connect.

### Audit Events

There are no new audit events needed on Windows.

### Observability

We will use the same observability methods for VNet on all platforms.

### Product Usage

Connect already reports usage metrics for VNet tagged with the host OS.

### Test Plan

Windows testing will be added to the VNet test plan.
