# The life cycle of a Teleport instance

The top-level `teleport` application acts as a scheduler that manages the 
execution of various registered "services", and an event bus that 
communicates between them.

The services can run briefly and exit, or live as long as the 
process, depending on what they need to do. They can also be 
tagged as "critical" services, meaning that if the service exits
with error it will bring down the whole process. 

Most of this is encapsulated by the `./lib/service.TeleportProcess` type, and 
services usually interact with it via the `./lib/service.Supervisor` interface.

## TeleportProcess

`main`
  - configure & parse command line (`/tool/teleport/common`)
  - load config file & merge config (`config.Configure`)
  - handle command routing for subcommands (e.g. re-exec, scp server side, etc)
`service.Run()`
`NewTeleport()`
    - sets up a bunch of services (see `initSSH()` below)
`TeleportProcess.Start()` (starts supervisor)
    - kicks off all of the registered services
wait for signals
exit, potentially forking replacement depending on signal.

## Reverse Tunnel

A node can be configured to use a tunnel, where a node will initiate a 
connection with a proxy that the proxy will route commands over. 

## An SSH Node Lifecycle

`initSSH()`: 

RegisterWithAuthServer() starts the `register.node` critical service.
 Asynchronously:
 - (re)connects to auth server & creates `auth.Client`
 - registers a process-exit handler that closes the client gracefully
 - Broadcasts the `SSHIdentity` event, with the new client in the payload

Calls `WaitForEvent` to act as a message pump that will route the payload 
from a `SSHIdentity` event in the broadcast system into a locally-created 
channel.

Registers the `ssh.node` service with the supervisor:
  Asynchronously:
  - Waits on the `SSHIdentity` event via the channel created above
  - Inits BPF, if necessary
  - Configures a `regular.Server` SSH server
    - If _not_ using a tunnel to talk to the auth server:
        - Creates (or imports an existing, if present) a listener for the 
          configured SSH port
        - Asynchronously starts serving SSH on the configured listener
    - Else (i.e. _is_ using a tunnel)
         - Starts the SSH server
         - Creates an agent pool to supply new connections to the SSH server
    - Broadcasts the `NodeSSHReady` event
  - Waits for ssh service to exit

Registers a process exit handler `ssh.shutdown` that will close the SSH 
server on process shutdown.

All this happens _before_ the `Supervisor` is started. Once the supervisor 
kicks off, the actual process loos something like this:

```
        ┌────┐                                                                       
        │main│                                                                       
        └────┘                                                                       
  ╔═══════╧══════════╗                                                               
  ║ Start supervisor ║
  ╚═══════╤══════════╝
          │    start    ┌─────────────┐
          │ ───────────>│register.node│
          │             └─────────────┘
          │                    │     start            ┌────────┐
          │ ─────────────────────────────────────────>│node.ssh│
          │                    │                      └────────┘
          │         ╔══════════╧═══════════╗   ╔══════════╧════════════╗
          │         ║ Establish connection ║   ║  Wait for SSHIdentity ║
          │         ║ to auth              ║   ║         Event         ║ 
          │         ╚══════════╤═══════════╝   ╚══════════╤════════════╝
          │                    │       SSHIdentity        │                          
          │                    │─────────────────────────>│                          
          │                    │                          │                          
          │            ┌───────────────┐                  │                          
          │            | register.node |                  │                          
          │            |     exits     |                  │                          
          │            └───────────────┘                  │                          
          │                                               │                                    
          │                      ╔══════╤═════════════════╪═════════════════════════╗
          │                      ║ ALT  │  direct connection to auth                ║
          │                      ╟──────┘                 │                         ║
          │                      ║            ╔═══════════╧═════════════╗           ║
          │                      ║            ║ Create listener for SSH ║           ║
          │                      ║            ╚═══════════╤═════════════╝           ║
          │                      ║      ╔═════════════════╧═════════════════╗       ║
          │                      ║      ║ Create SSH server around listener ║       ║
          │                      ║      ╚═════════════════╧═════════════════╝       ║
          │                      ╠══════════════════════════════════════════════════╣
          │                      ║ [using tunnel]         │                         ║
          │                      ║          ╔═════════════╧═══════════════╗         ║
          │                      ║          ║ Create AgentPool for        ║         ║
          │                      ║          ║ managing tunnel connections ║         ║
          │                      ║          ╚═════════════╤═══════════════╝         ║
          │                      ║      ╔═════════════════╧═══════════════════╗     ║
          │                      ║      ║ Create SSH server around agent pool ║     ║
          │                      ║      ╚═════════════════════════════════════╝     ║
          │                      ╚══════════════════════════════════════════════════╝          
          │                                               │                          
          │                                      ╔════════╧═════════╗                
          │                                      ║ Start SSH server ║                
          │                                      ╚════════╤═════════╝                
          │                                 ╔═════════════╧═══════════════╗          
          │                                 ║ Wait for SSH server to exit ║          
          │                                 ╚═════════════════════════════╝
          │                                               |
        ┌────┐                                        ┌────────┐  
        │main│                                        │node.ssh│                     
        └────┘                                        └────────┘                     
```

## The proxy discovery protocol

When a node uses a proxied tunnel to connect to a cluster's auth server, it is only visible 
to the proxy that provides the tunnel. If there are any other proxies in the cluster, by 
default they cannot reach the node. Teleport uses a discovery protocol to let nodes and 
proxies find each other.

Imagine the following cluster:
 * 3 Internet-facing proxies (`Huey`, `Dewey` and `Louie`) behind a load balancer.
 * A single auth server (`auth`)
 * An SSH node (`ssh`) that connects to `auth` server via a tunnel
 
Say that node `ssh` starts up and tries to connect to the cluster. It's initial attempts to find 
the tunnel addresses are routed to `Dewey` the load balancer. `Dewey` responds with a tunnel 
address, and after the requisite amount of handshaking, `ssh` has connection to `auth`, tunnelled through `Dewey`.

```
                     ┌───────┐
                     │       │
                     │  Huey │
                     │       │
                     └───────┘
 ┌────────┐          ┌───────┐           ┌──────┐
 │        ├──────────┼───────┼───────────►      │
 │  node  │          │ Dewey │           │ auth │
 │        │          │       │           │      │
 └────────┘          └───────┘           └──────┘
                     ┌───────┐
                     │       │
                     │ Louie │
                     │       │
                     └───────┘
```

So far, so good. Except that if a client tries to connect to `node`
via `Huey`, the connection will fail, because `Huey` has know knowledge 
of `node`'s existence. In order to get around this problem, `auth` will
issue a discovery message to `node`, telling it about the other proxies 
in the cluster.

The `AgentPool` on `node` will then attempt to establish tunnelled 
connections via each proxy in the cluster until they all know that 
`node` exists.

```
                     ┌───────┐
                     │       │
                     │  Huey │
         ┌───────────┼───────┼─────────────┐
         │           └───────┘             │
 ┌───────┴┐          ┌───────┐           ┌─▼────┐
 │        ├──────────┼───────┼───────────►      │
 │  node  │          │ Dewey │           │ auth │
 │        │          │       │           │      │
 └───────┬┘          └───────┘           └─▲────┘
         │           ┌───────┐             │
         └───────────┼───────┼─────────────┘
                     │ Louie │
                     │       │
                     └───────┘
```

## The AgentPool

The `AgentPool` exists to manage the node-side execution of the discovery protocol. 
Discovery is initiated by the auth server, which sends a discovery message to an 
`Agent` once the `Agent` has established a tunnel. This discovery message contains 
a list of all known proxies in the cluster.

Once a new proxy is discovered, the `AgentPool` spawns a new agent to create and 
manage a new tunnel to the auth server via the new proxy. The Agent handles things 
like reconnection of dropouts, backoff during connection attempts and so on.

The agent pool has a work throttling mechanism (see `WorkPool`) built in to stop 
an `Agent` from hammering to hard on an unresponisve proxy.

Very roughly, the process looks a bit like this:
```
                        ┌─────────┐          ┌─────┐               ┌────┐   
                        │AgentPool│          │Dewey│               │auth│   
                        └────┬────┘          └──┬──┘               └─┬──┘   
┌──────────────────────────────────────────────────────────────────────────┐
│                        Tunnel address found for Dewey                    │
└──────────────────────────────────────────────────────────────────────────┘
                              |                  |                    |
                              |                  |                    |
┌────────────┐ NewAgent(Dewey)│                  │                    │      
│Agent(Dewey)│ <───────────────                  │                    │      
└────────────┘                │                  │                    │      
      │                  connect                 │                    │      
      │ ─────────────────────────────────────────>                    │      
      │                       │                  │                    │      
      │                       │                  │ authentication, etc│      
      │                       │                  │ ───────────────────>      
      │                       │                  │                    │      
┌──────────────────────────────────────────────────────────────────────────┐
│                       Tunnel established via Dewey                       │
└──────────────────────────────────────────────────────────────────────────┘
      │                     OpenChannel(Discovery)                    │      
      │ <──────────────────────────────────────────────────────────────      
      │                       │                  │                    │      
      │                  Discover(Huey, Dewey, Louie)                 │      
      │ <──────────────────────────────────────────────────────────────      
      │                       │                  │                    │      
      │      Track(Huey)      │                  │                    │      
      │ ──────────────────────>                  │                    │      
      │                       │                  │                    │      
      │              ┌───────────────────┐       │                    │      
      │              │ New proxy "Huey", │       │                    │      
      │              │  Spawn new agent  │       │                    │      
      │              └───────────────────┘       │                    │      
      │      Track(Louie)     │                  │                    │      
      │ ──────────────────────>                  │                    │      
      │                       │                  │                    │      
      │             ┌────────────────────┐       │                    │      
      │             │ New proxy "Louie", │       │                    │      
      │             │  Spawn new agent   │       │                    │      
      │             └────────────────────┘       │                    │      
      │      Track(Dewey)     │                  │                    │      
      │ ──────────────────────>                  │                    │      
      │                       │                  │                    │      
      │               ┌─────────────────┐        │                    │      
      │               │  Dewey already  │        │                    │      
      │               │ tracked, ignore │        │                    │      
      |               └─────────────────┘        |                    |
┌────────────┐           ┌────┴────┐          ┌──┴──┐               ┌─┴──┐
│Agent(Dewey)│           │AgentPool│          │Dewey│               │auth│   
└────────────┘           └─────────┘          └─────┘               └────┘   
```


# Appendices

## PlantUML source

### SSH startup diagram
```
@startuml

control "main"
control "register.node" as register
control "node.ssh" as ssh

note over main: Start supervisor
create register
main -> register: start

create ssh
main -> ssh: start

note over ssh: Wait for SSH identity
/ note over register: Establish connection\nto auth

register -> ssh: SSHIdentity
note over register: Service exits

alt direct connection to auth
    note over ssh: Create listener for SSH 
    note over ssh: Create SSH server around listerner
else using tunnel
    note over ssh: Create AgentPool for\nmanaging tunnel conenctions
    note over ssh: Create SSH server around agent pool
end

note over ssh: Start SSH server
note over ssh: Wait for SSH server to exit
@enduml
```

### Discovery protocol diagram

```
@startuml

control "Agent(Dewey)" as agentd
control "AgentPool" as pool
participant "Dewey" as dewey
participant "auth"

rnote across: Tunnel address found for Dewey

create agentd
pool -> agentd : NewAgent(Dewey)
agentd -> dewey : connect
dewey -> auth : authentication, etc

rnote across: Tunnel established via Dewey

auth -> agentd : OpenChannel(Discovery)
auth -> agentd : Discover(Huey, Dewey, Louie)

agentd -> pool : Track(Huey)
rnote over pool: New proxy "Huey",\nSpawn new agent

agentd -> pool : Track(Louie)
rnote over pool: New proxy "Louie",\nSpawn new agent

agentd -> pool : Track(Dewey)
rnote over pool: Dewey already\ntracked, ignore
@enduml
```