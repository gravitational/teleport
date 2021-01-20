package common

const (
	usageNotes = `Notes:
  --roles=node,proxy,auth,app

  This flag tells Teleport which services to run. By default it runs auth,
  proxy, and node. In a production environment you may want to separate them.

  --token=xyz

  This token is needed to connect a node or web app to an auth server. Get it
  by running "tctl tokens add --type=node" or "tctl tokens add --type=app" to
  join an SSH server or web app to your cluster respectively. It's used once
  and ignored afterwards.
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
  to using that label in addition to its name.

> teleport start --roles=app --token=xyz --auth-server=proxy.example.com:3080 \
    --app-name="example-app" \
    --app-uri="http://localhost:8080"
  Starts an app server that proxies the application "example-app" running at
  http://localhost:8080.

> teleport start --roles=app --token=xyz --auth-server=proxy.example.com:3080 \
    --app-name="example-app" \
    --app-uri="http://localhost:8080" \
    --labels=group:dev
  Same as the above, but the app server runs with "group=dev" label which only
  allows access to users with the role "group=dev".
  
> teleport start --roles=database --token=xyz --auth-server=proxy.example.com:3080 \
    --db-name="example-db" \
    --db-protocol="postgres" \
    --db-uri="localhost:5432"
  Starts a database server that proxies PostgreSQL database "example-db" running
  at localhost:5432. The database must be configured with Teleport CA and key
  pair issued by "tctl auth sign --format=db".
`

	sampleConfComment = `#
# Sample Teleport configuration file
# Creates a single proxy, auth and node server.
#
# Things to update:
#  1. ca_pin: Obtain the CA pin hash for joining more nodes by running 'tctl status'
#     on the auth server once Teleport is running.
#  2. license-if-using-teleport-enterprise.pem: If you are an Enterprise customer,
#     obtain this from https://dashboard.gravitational.com/web/
#`
)
