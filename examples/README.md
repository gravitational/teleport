# Examples

## Daemon Configuration

* [systemd](https://github.com/gravitational/teleport/tree/master/examples/systemd) : Service file for systemd
* [upstart](https://github.com/gravitational/teleport/tree/master/examples/upstart) : Start-up script for [upstart](https://en.wikipedia.org/wiki/Upstart_(software))

## AWS examples

* [AWS: Simple cluster with Terraform](https://github.com/gravitational/teleport/tree/master/examples/aws/terraform/starter-cluster#teleport-terraform-aws-ami-simple-example)
* [AWS: High Availability cluster with Terraform](https://github.com/gravitational/teleport/tree/master/examples/aws/terraform/ha-autoscale-cluster#terraform-based-provisioning-example-amazon-single-ami)

## Kubernetes - Helm Charts

* [Helm Chart - Teleport cluster](https://github.com/gravitational/teleport/tree/master/examples/chart/teleport-cluster#teleport-cluster)
* [Helm Chart - Teleport agent](https://github.com/gravitational/teleport/tree/master/examples/chart/teleport-kube-agent#teleport-agent-chart)

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
