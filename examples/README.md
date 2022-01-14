# Examples

## Configuration Examples

* [local-cluster](https://github.com/gravitational/teleport/tree/master/examples/local-cluster) : Sample configuration of a 3-node Teleport cluster using just a single machine

## Daemon Configuration

* [systemd](https://github.com/gravitational/teleport/tree/master/examples/systemd) : Service file for systemd
* [upstart](https://github.com/gravitational/teleport/tree/master/examples/upstart) : Start-up script for [upstart](https://en.wikipedia.org/wiki/Upstart_(software))

## AWS examples

* [AWS: CloudFormation](https://github.com/gravitational/teleport/tree/master/examples/aws/cloudformation#aws-cloudformation-based-provisioning-example) : CloudFormation templates as an example of how to setup HA Teleport in AWS using our AMIs.
* [AWS: Terraform](https://github.com/gravitational/teleport/tree/master/examples/aws/terraform#terraform-based-provisioning-example-amazon-single-ami) : Terraform specifies example provisioning script for Teleport auth, proxy and nodes in HA mode. 
* [AWS: EKS. External Link](https://aws.amazon.com/blogs/opensource/authenticating-eks-github-credentials-teleport/)

## Kubernetes - Helm Charts

* [Helm Chart - Teleport Enterprise](https://github.com/gravitational/teleport/tree/master/examples/chart/teleport) : For deploying into Kubernetes using Helm 
* [Helm Chart - Teleport Demo](https://github.com/gravitational/teleport/tree/master/examples/chart/teleport-demo) : An internal demo app showing Teleport components deployed into Kubernetes using Helm Charts. 


## SSO Connector Examples and Trusted Cluster Examples
### SSO Resources
* [Active Directory - YAML Resource](https://github.com/gravitational/teleport/blob/master/examples/resources/adfs-connector.yaml)
* [OIDC Connector, like "keycloak". - YAML Resource](https://github.com/gravitational/teleport/blob/master/examples/resources/oidc-connector.yaml)
* [SAML Connector, like "Okta". - YAML Resource](https://github.com/gravitational/teleport/blob/master/examples/resources/saml-connector.yaml)


### Role
* [Example Role](https://github.com/gravitational/teleport/blob/master/examples/resources/role.yaml)

### Trusted Cluster
* [Trusted Cluster Resource](https://github.com/gravitational/teleport/blob/master/examples/resources/trusted_cluster.yaml)
* [Trusted Cluster Resource - With RBAC (Enterprise Only)](https://github.com/gravitational/teleport/blob/master/examples/resources/trusted_cluster_enterprise.yaml)