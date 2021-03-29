---
authors: Trent Clarke (trent@goteleport.com)
state: draft
---

# RFD 22 - SSH key agent forwarding

## What

Issue [#1517](https://github.com/gravitational/teleport/issues/1517) draws 
attention to the behaviour of `tsh ssh` differing from the behaviour of the
stock OpenSSH `ssh` client when forwarding a Key Agent to the remote machine. 

This is a proposal to change `tsh ssh`'s agent-forwarding behaviour to match
that of the OpenSSH client, while also allowing the legacy `tsh` behaviour if
necessary.

## Why

### Background: What is Key Agent Forwarding?

An SSH Key Agent is a service that brokers access to keys, used by an SSH client
when authenticating against a remote machine.

The OpenSSH remote login client allows a user to _forward_ their key agent to 
the remote server via a secondary channel in the ssh protocol. This makes the Key 
Agent on the user's local machine available on the remote server, and allowing the 
user to connect to a _third_ machine from the remote server and authenticate 
with keys stored on the user's local machine.

```
             Overly Simplified Authentication Flow with Forwarded Key Agent
                                                                                           
   ┌─────────┐          ┌─────────────┐          ┌──────────────┐          ┌─────────────┐ 
   │Key Agent│          │Local Machine│          │Remote Machine│          │Third Machine│ 
   └────┬────┘          └──────┬──────┘          └──────┬───────┘          └──────┬──────┘ 
        │                      │      SSH Connect       │                         │        
        │                      │───────────────────────>│                         │        
        │                      │    Auth challenge      │                         │        
        │                      │<───────────────────────│                         │        
        │   Auth Challenge     │                        │                         │        
        │<─────────────────────│                        │                         │        
        │   Signed Response    │                        │                         │        
        │─────────────────────>│                        │                         │        
        │                      │    Signed Response     │                         │        
        │                      │───────────────────────>│                         │        
        │              ╔═══════╧════════════════════════╧════════╗                │        
        │              ║       Authentication established        ║                │        
        │              ╚═══════╤════════════════════════╤════════╝                │        
        │                      │               ╔════════╧═════════╗               │        
        │                      │               ║ User connects to ║               │        
        │                      │               ║ "Third Machine"  ║               │        
        │                      │               ╚════════╤═════════╝               │        
        │                      │                        │       SSH Connect       │        
        │                      │                        │ ───────────────────────>│        
        │                      │                        │     Auth challenge      │        
        │                      │                        │ <───────────────────────│        
        │  Auth Challenge (via forwarded connection)    │                         │        
        │<──────────────────────────────────────────────│                         │        
        │  Signed Response (via forwarded connection)   │                         │        
        │──────────────────────────────────────────────>│                         │        
        │                      │                        │     Signed Response     │        
        │                      │                        │ ───────────────────────>│        
        │                      │                ╔═══════╧═════════════════════════╧═══════╗
        │                      │                ║       Authentication established        ║
        |                      |                ╚═════════════════════════════════════════╝
   ┌────┴────┐          ┌──────┴──────┐          ┌──────┴───────┐          ┌──────┴──────┐ 
   │Key Agent│          │Local Machine│          │Remote Machine│          │Third Machine│ 
   └─────────┘          └─────────────┘          └──────────────┘          └─────────────┘ 
```

The above diagram leaves out a _lot_ of detail, but the overall process should be clear 
enough for this discussion.

### What's the issue?

An OpenSSH user can forward their Key Agent to the remote machine using the `-A` flag on 
the `ssh` command line. This allows the user to use arbitrary keys stored on their _local_ 
machine to authenticate against a "Third Machine" from inside their session on the Remote
Machine.

When running `tsh ssh`, a user may have _two_ independent key agents running: their own 
(which we will call the _User Key Agent_), and an ephemeral one inside the `tsh` process 
itself (the _Teleport Key Agent_). The Teleport Key Agent is populated with the contents 
of the user's `.tsh` directory.

The `tsh ssh` client _also_ offers a `-A` option to forward a Key Agent to the Remote 
Machine, but it will only ever forward the Teleport Key Agent, not the User Key Agent.
This is surprising behaviour to a user accustomed to OpenSSH. The user finds that keys 
they expected to be available to them in the session on the Remote Machine are not 
available.

The tricky part is that _both_ of these behaviours are sensible, depending on the user's 
expectations. Any solution will have to accommodate both behaviours.

## Details

### Proposed Solution

Behind the scenes, both the OpenSSH client & `tsh ssh` treat the `-A` flag as shorthand 
for setting the SSH config value `ForwardAgent` to `yes`.

Looking at the `man` page for `ssh_config(5)`, we can see the definitive list of values 
that `ForwardAgent` can take, and their meaning:

> **ForwardAgent**
>
>   Specifies whether the connection to the authentication agent (if any) will be forwarded 
>   to the remote machine.  The argument may be yes, no (the default), an explicit path to
>   an agent socket or the name of an environment variable (beginning with ‘$’) in which to 
>   find the path.

The `tsh ssh` client already supports the `yes` and `no` options, where `yes` means 
forwarding the Teleport Key Agent. To allow the user to forward their own Key Agent, _and_ 
to bring `tsh ssh`'s behaviour more in line with the OpenSSH client, I propose the following
values be allowed for the `ForwardAgent`option:


| Value          | Interpretation                                          |
|----------------|---------------------------------------------------------|
| `no` (default) | `tsh ssh` will not forward _any_ Key Agent              |
| `yes`          | `tsh ssh` will forward the User Key Agent (if present)  |
| `local`        | `tsh ssh` will forward the Teleport Key Agent           |


* The value `yes` is redefined to refer to the User Key Agent instead of 
  the Teleport Key Agent.

* The value `local` is a `tsh`-specific extension that will activate the 
  backwards-compatible behaviour. 

The effect of this change is that the `tsh ssh` option `-A` will automatically acquire 
semantics in line with the OpenSSH client, while the backwards compatible behaviour can be 
activated with something like:

```bash
$ tsh ssh -o "ForwardAgent local" root@example.com
```

### Precedence

If both `-A` and `-o "ForwardAgent $VALUE"` are specified together on the same command line, `-A`
will take precedence. This is consistent with the existing behaviour of `tsh ssh`, where a 
command line like... 
```bash
$ tsh ssh -A -o "ForwardAgent no" root@example.com
```
...would result in the Key Agent being forwarded.


### No User Key Agent

If no User Key Agent is running and the user specifies `-A` and/or `-o "ForwardAgent yes"`, then 
`tsh ssh` will NOT forward an Key Agent to the remote machine, consistent with the behaviour of
OpenSSH.

### Security Concerns

Forwarding a Key Agent is inherently a security risk, as it allows anyone with sufficient 
privileges on the remote machine (i.e. `rw` on the unix domain socket used by the Key Agent) to
perform operations with the User's keys on the local machine. 

The local keys are vulnerable for the duration of any ssh connection (with a forwarded Key Agent) 
to a remote machine.

This change will also potentially expose _more_ keys to danger than the existing behaviour. The 
existing `tsh ssh` Key Agent forwarding system exposes only the keys in the `~/.tsh` directory.
Allowing the user to forward their own Key Agent changes that risk to _all_ keys managed by that 
agent by that. Depending on how the user invoked `tsh login`, this may also include the teleport
keys as well.

We also have to understand that by making this change we are implicitly encouraging users to work
in a way that exposes them to a higher risk of compromise, however slight. This is something 
we should intentionally decide on, rather than have it happen as a side effect.

My (admittedly naive) opinion is that given forwarding the User Key Agent

 1. exposes nothing worse than OpenSSH already allows,
 2. allows the user some extra utility they would miss from OpenSSH,
 3. is a time-limited vulnerability, in that the vulnerability only lasts as long as the SSH
    connection, and
 4. is opt-in behaviour

...it's _probably_ OK, but I welcome points to the contrary.

## Out of scope

Adding support for indicating an arbitrary Key Agent to forward (by passing the path to a unix 
domain socket, or an environment variable containing the same) as per `man ssh_config(5)` is
not being considered here. My only concern in that direction has been to avoid making it harder
to implement later, if it's ever required.

## Appendices

### PlantUML source

```PlantUML
@startuml

title "Overly Simplified Authentication Flow"

participant "Key Agent"
participant "Local Machine"
participant "Remote Machine"
participant "Third Machine"

"Local Machine" -> "Remote Machine" : SSH Connect
"Remote Machine" -> "Local Machine" : Auth challenge
"Local Machine" -> "Key Agent" : Sign Challenge
"Key Agent" -> "Local Machine": Signed Response
"Local Machine" -> "Remote Machine" : Signed Response

note over "Local Machine", "Remote Machine": Authentication established

note over "Remote Machine": User connects to\n"Third Machine"
"Remote Machine" -> "Third Machine" : SSH Connect
"Third Machine" -> "Remote Machine" : Auth challenge
"Remote Machine" -> "Key Agent" : Sign Challenge (via forwarded connection)
"Key Agent" -> "Remote Machine": Signed Response (via forwarded connection)
"Remote Machine" -> "Third Machine" : Signed Response

note over "Remote Machine", "Third Machine": Authentication established
@enduml
```