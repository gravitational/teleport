# Teleport load-test resources

## Introduction

This directory contains:

- [the `node-agent` helm chart](./node-agent) deploying Teleport ssh node load-test agents
- [the `tsh-bench-agent` helm chart](./tsh-bench-agent) deploying tsh bench session agents
- instructions to deploy a test Teleport cluster on EKS (in this README)

Those charts and instructions are for Teleport internal development,
they are not part of the product and no support will be provided.

## How to load-test Teleport deployed via the `teleport-cluster` Helm chart

### Install tested cluster

Start by creating a working cluster:

- Create EKS cluster with the correct policies
  [according to our EKS guide](https://goteleport.com/docs/admin-guides/deploy-a-cluster/helm-deployments/aws/)
- Make sure EBS CSI addon is deployed
- Make sure the policy `AmazonEBSCSIDriverPolicy` is granted to the instance
  role associated with the EKS nodegroups which are running your Kubernetes nodes.
- install cert-manager and create an issuer as instructed in the EKS guide

Install the monitoring stack:

```shell
# Add repos if you don't have them yet
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# Install the stack
helm install monitoring -n monitoring --create-namespace prometheus-community/kube-prometheus-stack -f values/kube-prometheus-stack.yaml
```

Generate a secret token

```bash
TOKEN=$(pwgen -n 30)
```
Edit `values/teleport.yaml` (replace <your-name>), then install Teleport using the chart

```shell
helm install teleport -n teleport --create-namespace <path/to/chart> --values values/teleport.yaml --set auth.teleportConfig.auth_service.tokens[0]="node:$TOKEN"
```

For v11 and below:
- edit the `teleport` configmap to add a static token and set `routing_strategy: most_recent`
  ```yaml
    auth_service:
      routing_strategy: 'most_recent'
      tokens:
        - "node:$TOKEN"  # Replace $TOKEN with your join token
  ```

In the AWS Console, [change dynamoDB provision settings for "onDemand"](https://aws.amazon.com/blogs/aws/amazon-dynamodb-on-demand-no-capacity-planning-and-pay-per-request-pricing/).

### Run test

#### Run node agents

To deploy 5000 ssh nodes, run the following command. A node is a teleport instance running only the `ssh_service`.

```
helm upgrade --install node-agents -n agents --create-namespace node-agent/ --values values/node-agents.yaml --set replicaCount=250 --set agentsPerPod=20 --set proxyServer=<your-name>-lt.teleportdemo.net:443 --set joinParams.token_name=$TOKEN
```

This will deploy 250 pods running 20 Teleport SSH instances each, the instances are packed by pod because ENIs are limited on EKS and Kubernetes also limits the amount of pods per node.

#### Run tsh-bench agents

Create a user and get an identity (by default the identity is valid for 24 hours, make sure to refresh it or increase the TTL):

Note: by default the user is named `joe`, you can change this by editing `user.yaml`.

```bash
POD="$(kubectl get pods -n teleport -l app=teleport -o name | head -n 1 | sed 's@^pod/@@')"
kubectl exec -i -n teleport "$POD" -- tctl create -f < fixtures/user.yaml
kubectl exec -it -n teleport "$POD" -- tctl auth sign --user joe -o identity.pem
kubectl cp -n teleport "$POD:/identity.pem" ./fixtures/identity.pem
kubectl create -n agents secret generic tsh-bench-agents --from-file=identity.pem=./fixtures/identity.pem
```

Deploy the agent:

```shell
helm upgrade --install tsh-bench-agents tsh-bench-agent/ -n agents --values values/tsh-bench-agents.yaml --set proxyServer=<your-name>-lt.teleportdemo.net:443 --set joinParams.token_name=$TOKEN
```
