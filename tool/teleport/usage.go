package main

const (
	usageNotes = `Notes:
  --roles=node,proxy,auth

  This flag tells Teleport which services to run. By default it runs all three. 
  In a production environment you may want to separate them.

  --token=xyz

  This token is needed to connect a node to an auth server. Obtain it by running 
  "tctl nodes add" on the auth server. It's used once and ignored afterwards.
`

	usageExamples = `
Examples:

> teleport start
      By default without any configuration, teleport starts running with all roles 
      enabled It's the equivalent of running with --roles=node,proxy,auth 

> teleport start --roles=node --auth-server=10.5.0.2 --token=xyz --name=database
      Starts this SSH node named 'database' running in SSH server mode and 
      authenticating connections via the auth server running on 10.5.0.2`

	sampleConfComment = `#
# Sample Teleport configuration file.
#`
)
