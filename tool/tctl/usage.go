package main

const (
	GlobalHelpString = "CLI Admin tool for the Teleport Auth service. Runs on a host where Teleport Auth is running."
	AddUserHelp      = `Notes:

  1. tctl will generate a signup token and give you a URL to share with a user.
     He will have to configure the mandatory 2nd facto auth and select a password.

  2. A Teleport user account is not the same as a local UNIX users on SSH nodes.
     You must assign a list of allowed local users for every Teleport login.

Examples:

  > tctl users add joe admin,nginx

  This creates a Teleport identity 'joe' who can login as 'admin' or 'nginx' 
  to any SSH node connected to this auth server.

  > tctl users add joe

  If the list of local users is not given, 'joe' will only be able to connect
  as 'joe' to SSH nodes.
`
	AddNodeHelp = `Notes:
  This command generates and prints an invitation token another node can use to 
  join the cluster. 

Examples:

  > tctl nodes add 

  Generates a token when can be used to add a regular SSH node to the cluster.
  The token will be valid for 15 minutes.

  > tctl nodes add --roles=node,proxy --ttl=1h

  Generates a token when can be used to add an SSH node to the cluster which
  will also be a proxy node. The token can be used multiple times within an 
  hour.
`
	ListNodesHelp = `Notes:
  SSH nodes send periodic heartbeat to the Auth service. This command prints
  the list of current online nodes.
`
)
