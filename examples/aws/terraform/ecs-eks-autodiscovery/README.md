## Auto Discover and access EKS clusters

This is a terraform example to get you started on EKS Access using only AWS ECS.

## How does it work?

It creates the required AWS resources:
- IAM Role with the required permission for accessing EKS APIs
- IAM Role to allow log stream of the teleport agent into CloudWatch
- ECS Task Definition which runs a Teleport Agent with a Discovery and a Kubernetes Service
- ECS Cluster and an ECS Service which runs the Task Definition above

## Requirements
The following set up is required:
- install [`terraform`](https://developer.hashicorp.com/terraform/install)
- [configure AWS credentials](https://registry.terraform.io/providers/hashicorp/aws/latest/docs#authentication-and-configuration)
- EKS Clusters running in your AWS account with discoverable tags
- An IAM Join Token in teleport which allows [Discovery and Kube roles](https://goteleport.com/docs/enroll-resources/auto-discovery/kubernetes/aws/#get-a-join-token)
- The Teleport service must be able to connect to the EKS API

## Instructions

1. Create a `my.tfvars` file using the following example and replace the variable values with your Teleport cluster and AWS information.

            hcl
            // Your Teleport Cluster Address
            teleport_proxy_server = "tenant.teleport.sh:443"
            
            // Create a new IAM Join Token that allows Discovery and Kubernetes roles.
            teleport_iam_token_name = "iam-join-token"
            
            // This information indicates where the Teleport Discovery and Kubernetes services will run and its network access.
            aws_region = "eu-south-2"
            teleport_agent_subnets = [ "subnet-1111" ]
            teleport_agent_security_groups = [ "sg-2222" ]
            // The name for the ECS Cluster that will be created.
            ecs_cluster = "my-cluster"
            
            // Update this value to match the version that your cluster is running.
            teleport_image = "public.ecr.aws/gravitational/teleport-ent-distroless:17.5.2"
            
            // Default tags to add to AWS resources when creating them.
            default_tags = {
                "DeployedBy" = "TerraformTeleport"
            }
            
            // The following allows you to filter the EKS Clusters to proxy.
            // Only the matching EKS clusters will be enrolled.
            discover_eks_tags = {
                "TeleportAutoDiscover" = ["please"]
            }

2. Save the my.tfvars you created in the previous step.
3. Run the following command using Terraform:
   
        bash
        $ terraform apply -var-file my.tfvars

After deploying, you should see your discovered EKS clusters in your Teleport tenant.

