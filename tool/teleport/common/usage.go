package common

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
  By default without any configuration, teleport starts running as a single-node
  cluster. It's the equivalent of running with --roles=node,proxy,auth 

> teleport start --roles=node --auth-server=10.1.0.1 --token=xyz --nodename=db
  Starts a node named 'db' running in strictly SSH mode role, joining the cluster 
  serviced by the auth server running on 10.1.0.1

> teleport start --roles=node --auth-server=10.1.0.1 --labels=db=master
  Same as the above, but the node runs with db=master label and can be connected
  to using that label in addition to its name.`

	sampleConfComment = `#
# Sample Teleport configuration file
# Creates a single proxy, auth and node server.
#
# Things to update:
#  1. ca_pin: Obtain the CA pin hash for joining more nodes by running 'tctl status'
#     on the auth server once Teleport is running.
#  2. cluster-join-token: Update to a more secure static token. For more details,
#     see https://gravitational.com/teleport/docs/admin-guide/#adding-nodes-to-the-cluster
#  3. license-if-using-teleport-enterprise.pem: If you are an Enterprise customer,
#     obtain this from https://dashboard.gravitational.com/web/
#`
)
