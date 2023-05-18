# RFD 01xx - Teleport Command

## Required Approvers

- Engineering: @r0mant
- Security: @reed || @jentfoo
- Product: @xinding33 || @klizhentas

## What

Implement Teleport Command, a non-interactive local host management agent, in
our infrastructure. The implementation of Teleport Command will provide a safer
interface for localhost debugging and management, reducing the reliance on
interactive shells for emergency debugging and recovery.

## Why

Our infrastructure often requires emergency debugging and management,
traditionally done through an interactive shell. This approach,
while effective, presents potential security risks as interactive shells can be
prone to exploitation. Implementing Teleport Command allows for stronger
authentication, authorization, and auditing of server management across
our infrastructure.

Example use cases include:

1. Debugging server issues: Teleport Command offers a safer interface for debugging
   server issues, reducing the risk of exploitation during emergency debugging.
2. Server management: With Teleport Command, we can better manage servers of any type,
   with a stronger focus on authentication, authorization, and auditing.

## Details

### Security

#### Teleport Command implementation design principles

The implementation of SansShell should fulfil the following design principles
to ensure maximum security and efficiency:

1. **Non-interactive:** SansShell is primarily a non-interactive agent,
   meaning it should not prompt for user input during its operation.
   This reduces the chances of exploitation through user input.

2. **Per command MFA:** Teleport Command will allow to make access even
    more granular by requiring MFA for each command. 

3. **Per command request access:** Teleport Command will allow to request
    access to a command. This will allow to implement a workflow where
    a user can request access to run a command on a particular set of nodes.

4. **Safer interface for debugging:** The implementation should replace
   interactive shells for debugging, providing a safer interface for these operations.

5. **Auditing capabilities:** The output of a command execution will be
   stored in the session recordings. This will allow for better monitoring and control
   over server management activities.

While these principles will guide the implementation, further details will need
to be established as we delve into the specific requirements of our infrastructure
and the capabilities of SansShell.

### Implementation

The implementation will re-use the existing Teleport SSH and Assist infrastructure.
Teleport Proxy will be used as the main command execution engine.


#### Command resource

We will introduce a new resource type `command` that will allow to define
commands that can be executed by Teleport Command. The resource will have
the following fields:

```yaml
#
# Example resource for a Command
#
kind: command
version: v1
metadata:
  # The name of the command. It must be unique.
  name: cpu-usage
  # Human-readable description
  description: Show top 10 CPU usage
spec:
  # Node labels where the command can run.  
  labels: 
    os: linux
    env: dev
  interpreter: "/bin/bash"
  command: |
    ps -eo pid,ppid,cmd,%mem,%cpu --sort=-%cpu | head
```

Interpreter is an optional field. It should allow to specify the interpreter
allowing to not only execute a shell command but also a short Python or JS script.

## Roles

Role will be extended to allow to define access to commands. The following
example shows how to allow to execute `cpu-usage` command on all nodes

```yaml
kind: role
version: v5
metadata:
  name: developer
spec:
  allow:
    # Labels selector for command execution.
    command_execution_labels:
      environment: ["dev"]
    # Labels selector for commands resource access.
    command_labels:
     environment: ["dev"]
  deny:
    command_labels:
     environment: ["prod"]
```

## tsh extensions

`tsh` will learn two new commands

1. `tsh command ls` - allowing to list all available commands.
2. `tsh command exec` - allowing to execute commands on one or multiple nodes.


### Display output from multiple nodes

```shell
$ tsh command exec cat-log
node1: Detected long output. Redirecting to node1.log file
node2: Detected long output. Redirecting to node2.log file
```