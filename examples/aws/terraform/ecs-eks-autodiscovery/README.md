## Auto Discover and access EKS clusters

This is a terraform example to get you started on EKS Access using only AWS ECS.

## How does it work?

It creates the required AWS resources:
- IAM Role with the required permission for accessing EKS APIs
- IAM Role to allow log stream of the teleport agent into CloudWatch
- ECS Task Definition which runs a Teleport Agent with a Discovery and a Kubernetes Service
- ECS Cluster and an ECS Service which runs the Task Definition above

## Instructions

Create a `my.tfvars` file with the following content, and replace to match your teleport cluster and aws information.
```hcl
teleport_proxy_server = "proxy.example:443"

// Create a new IAM Join Token in Teleport.
teleport_iam_token_name = "iam-join-token"

aws_region = "eu-south-2"
ecs_cluster = "my-cluster"
teleport_agent_subnets = [ "subnet-1111" ]
teleport_agent_security_groups = [ "sg-2222" ]

// Default tags to add to AWS resources when creating them.
default_tags = {
    "teleport.dev/creator" = "em@i.l"
}

// The following allows you to filter the EKS Clusters to proxy.
// Only the matching EKS clusters will be enrolled.
discover_eks_tags = {
    "RunDiscover" = ["yes-please"]
}
```

Save this as a file and then run:
```bash
$ terraform apply -var-file my.tfvars
```
