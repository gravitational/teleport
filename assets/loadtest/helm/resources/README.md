# resources 

This Helm chart can be used to programmatically spawn an arbitary number of resources in a Teleport cluster given a resource manifest.

It assumes a pre-existing k8s Teleport cluster and correctly authenticated kubectl client.

*note:* This is an experimental tool used for internal testing, do not use in production.

## Testing dynamic applications

All commands run from `assets/loadtest/`

Create join token for the app service:
```sh
make create-token TYPE=node,app

# Note the output and export it:
export TOKEN=<token value>

# Export proxy address:
export PROXY_SERVER=example.teleport-test.com:443

# Set the version
export TELEPORT_VERSION="18.1.8"
```

Deploy application service agent:
```sh
make deploy-app-service
```

Prepare a tbot and the correct token:
```sh
make create-tbot-resources
```

Deploy tbot and create applications:
```sh
# This will spawn batch jobs using tbot credentials to run backend commands on the cluster.
# By default each app job will spawn 10 applications for a total of 10*$COUNT items in the backend.
make create-app-resources COUNT=1000 PARALLELISM=10
```

In the `teleport-kube-agent` logs you should see:
```
2025-09-02T08:57:10.170Z INFO [APP:SERVI] New resource matches, creating kind:app name:app6-7.example services/reconciler.go:175
```
And the app resources should show up in the web UI after logging in, or alternatively using `tctl`:
```sh
kubectl --namespace teleport exec -i deploy/teleport-auth -- tctl apps ls
```

To clean up:
```sh
make delete-app-resources delete-app-service
```


