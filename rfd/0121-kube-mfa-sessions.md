---
authors: Anton Miniailo (anton@goteleport.com)
state: implemented (v13.0)
---

# RFD 0121 - Kubernetes MFA sessions

## Required Approvers

- Engineering: `@r0mant` && `@tigrato`
- Product: `@klizhentas` || `@xinding33`
- Security: `@reedloden`


## What
Use local proxy for Kubernetes access to extend lifetime of Kubernetes MFA sessions, so that users don't have to use MFA device for every command they run when per-session-MFA is enabled.

## Why
Kubernetes commands currently are treated as separate MFA sessions, so each time user runs a kubectl command they have to do a MFA check. This creates a burdensome user experience.
By using local Kubernetes proxy we can extend MFA session to be limited not by running single command but by lifetime of the local proxy. We should also remove the 1 minute restriction 
on TTL of Kubernetes certificates for local proxy to improve overall UX.

## Details

This proposal builds heavily upon [RFD 90 "Database MFA Sessions"](https://github.com/gravitational/teleport/blob/master/rfd/0090-db-mfa-sessions.md). 
It is an extension of the principles established in RFD 90 and it follows the same structure and
approach since we're extending same idea to provide very similar functionality, but for Kubernetes access.

Local proxy for Kubernetes access will handle issuing MFA certificates and will keep them in memory while the proxy process is running. 
As with DB MFA sessions, certificates TTL will be limited to the `max_session_ttl` time instead of 1 minute.
Local proxy will also reissue certificates and ask for MFA check/credentials if required.

Unlike DB MFA sessions, Kubernetes local proxy will only have one mode of operation (DB access has tunnelled and not tunnelled mode). 
Since the pattern of usage for Kubernetes is mostly short-lived commands, analogue of tunneled mode for the local proxy makes the most sense.

Kubernetes local proxy is already being worked on as part of adding support of L7 load balancers in RFD_NNN (placeholder, not merged yet), 
which makes usage of local proxy mandatory if Teleport is behind L7 load balancer, such as AWS ALB.

Local proxy will generate ephemeral kubeconfig that kubectl will use and then based on SNI proxy will select real user credentials
to access remote Kubernetes service. Existing user's kubeconfig (including entries from `tsh kube login`) will be integrated into
this ephemeral kubeconfig for better UX.

Chart from RFD_0123 demonstrating local Kubernetes proxy flow:
```
 ┌───────┐                                        ┌─────────┐                         ┌───────────────┐
 │kubectl│                                        │tsh local│                         │Teleport Proxy/│
 │       │                                        │  proxy  │                         │ Load Balancer │
 └┬──────┘                                        └─┬───────┘                         └────────────┬──┘
  │                                                 │                                              │
  │                                                 ├──┐                                           │
  │                                                 │  │generate:                                  │
  │                                                 │  │local CA                                   │
  │                                                 │  │local credentials                          │
  │                                                 │◄─┘ephemeral KUBECONFIG                       │
  │                                                 │                                              │
  │ server https://localhost:8888                   │                                              │
  │ sni my-kube.kube-teleport-local.my-teleport.com │                                              │
  │ local credentials                               │                                              │
  ├────────────────────────────────────────────────►│ server https://my-teleport.com:443           │
  │                                                 │ sni kube-teleport-proxy-alpn.my-teleport.com │
  │                                                 │ "real" user credentials                      │
  │                                                 ├─────────────────────────────────────────────►│
  │                                                 │                                              │
  │                                                 │◄─────────────────────────────────────────────┤
  │◄────────────────────────────────────────────────┤                                              │
  │                                                 │                                              │
```

User experience will look like this:

```bash
$ tsh proxy kube test-kube-cluster -p 8888
Preparing the following Teleport Kubernetes clusters:
Teleport Cluster Name Kube Cluster Name
--------------------- ----------------------
my-teleport.com       test-kube-cluster

Enter an OTP code from a device:
Started local proxy for Kubernetes on 127.0.0.1:8888

Use the following config for your Kubernetes applications. For example:
export KUBECONFIG=/Users/teleuser/.tsh/keys/my-teleport.com/teleuser-kube/my-teleport.com/localproxy-8888-kubeconfig
kubectl version
```
If user already ran `tsh kube login`, then they don't have to provide Kubernetes cluster name in the `tsh proxy kube` command,
 it will be automatically picked up.
In addition users can utilize aliases to make it even simpler to invoke Kubernetes commands with running local proxy. 

In the case of relogin user might not notice immediately that running proxy requires input. To improve the situation we'll add 
a timeout for the request that triggered the relogin operation and return an error to kubectl if the login didn't happen until the timeout occurs, so they can understand that the local proxy
requires attention.

### Preserving Prior Behavior

There are no required changes to user workflow if per-session-MFA is not enabled, although local proxy will work in that case as well. 
We are preserving capability to still use regular flow of `tsh kube login` -> `kubectl` when per-session-MFA is enabled if user doesn't want
to use local proxy. In that case there's also no changes and user will have to pass MFA check on each command. 
Although, as mentioned above, if Teleport setup includes L7 load balancer, usage of the Kubernetes local proxy becomes a requirement
whether per-session-MFA is enabled or not.

### API

We should change the auth service to issue Kubernetes access certs without capping the
cert TTL to 1 minute for per-session-MFA when the certs are requested specifically for a 
local Kubernetes proxy, which will only keep the certs in-memory.

### Teleport Connect
Similar to Database MFA session, new Kubernetes MFA sessions can be seamlessly integrated into Connect workflow, improving user UX.
Connect can start local proxy and when MFA is needed to reissue certificates; the app window can be brought to the top to prompt the user for
MFA. This can be even better UX compared to a cli-based prompt, which may not raise any indicator that an MFA tap is required
when the cli is not visible on the user's screen.

The full details of how Teleport Connect will implement such MFA prompt are outside the
scope of this RFD, but building upon DB MFA sessions, there's sufficient infrastructure to enable this capability.

## Security
Same security considerations as with DB MFA sessions can be applied for this proposal.
Extending lifetime of MFA certificates might be seen as decrease of security, but we should balance it on the usability,
as with all things security. Too strict rules creating burdensome user experience might lead to decrease of overall security,
 because people will try to find workarounds or simply disable per-session-MFA. By extending MFA session to `max_session_ttl` 
we give admins ability to decide for themselves what time they find most appropriate, on per resource basis. They can 
set it to 1 minute, mimicking old behaviour, so we're just improving agility of the product.
Or reserve 1 minute TTL for seldom used high importance resources and make TTL higher for the rest.

By keeping certificates in memory we're making exfiltration harder and protect from some "accidental" certificate sharing events - like
stealing of a computer/hard drive. It doesn't prevent it completely and attacker with high degree of access to the 
user's machine can exfiltrate certificates from memory, or simply connect to the local proxy while it's running. Attacker still needs user to
initiate MFA session, the difference is only in the time window for action. Certificates with 1 minute TTL don't fundamentally
close this opportunity, just make it harder. But in any case if attacker has such a high degree of access, user's machine
is compromised regardless of MFA certificate TTL, so this is already an existing concern.

In addition, we can get some security improvement by integrating with PIV hardware private keys. Detailed overview can be 
found in [dedicated section of RFD 90](https://github.com/gravitational/teleport/blob/master/rfd/0090-db-mfa-sessions.md#integrating-with-piv-hardware-private-keys-for-security-improvements)
, but in summary an attacker capable of hijacking local connections on a user's machine
cannot be *fully* mitigated; this is simply unavoidable. But with PIV we can prevent private key exfiltration and narrow the attack surface area
to only active local proxy tunnel connections to specific resources.

## Future development
To improve UX in a situation when local proxy requires relogin and user running kubectl in another terminal not noticing it, we could 
add ability to tsh to communicate with the running local proxy and transfer request for the input to the kubectl. And also this could eliminate
requirement for the ephemeral kubeconfig, since credentials for kubectl could be received the usual way through `tsh`. In that case user
will see that relogin is required immediately, though this adds more complexity to the system (`tsh` -> `local proxy` communication).

## Alternatives 
Here are some alternative considerations or improvements we could do
1. We can leave 1 minute TTL, forcing user to perform MFA check to reissue certs every minute. This decreases the risk, but still does not eliminate it, 
 only leaving shorter time window for an attack. But this is bad user experience and may lead to MFA sessions just being disabled for often used resources.
2. We could add a new parameter for the config that will allow clients specifically control length of MFA sessions, 
independent of `max_session_ttl`. It can be cluster-level or role-level setting, and also can be single option for all MFA sessions,
 or split by resource types (e.g. `max_kube_mfa_session`, `max_db_mfa_session`). This might make it easier for admins to control it.


