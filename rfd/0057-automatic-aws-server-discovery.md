---
authors: Alex McGrath (alex.mcgrath@goteleport.com)
state: draft
---

# RFD 57 - Automatic discovery and enrollment of AWS servers

## What

Proposes a way by which an SSH service might automatically discover and register AWS
EC2 instances.

## Why

Currently when adding a new AWS server, it's required that Teleport be installed
after the server has been provisioned which may be a slow process for organizations
with large numbers of servers as it needs to be installed and then added to the
teleport cluster

With the changes described in this document, Teleport will be able to resolve the
issues with adding AWS servers to Teleport clusters automatically.


## Discovery

A new service will be introduced for general purpose cloud resource discovery:
`discovery_service`. Initially, it will only support EC2 discovery.

Discovery will use a matcher similar to the `db_service/aws` matcher, however EC2
discovery will have an optional install command, set of join parameters and script to
use when joining:

```yaml
discovery_service:
  enabled: "yes"
  aws:
  aws:
   - types: ["ec2"]
     regions: ["eu-central-1"]
     tags:
       "teleport": "yes"
     install:
       join_params:
         token_name:  "aws-discovery-iam-token" # default value
       script_name: "default-installer" # default value
     ssm:
       document: "TeleportDiscoveryInstaller" # default value
```

The agent will use EC2's `DescribeInstances` API in order to list instances[1]. This
will require the teleport SSH agent to include `ec2:DescribeInstances` as part of
it's IAM permissions.

As with AWS database discover, new EC2 nodes will be discovered periodically on a 60
second timer, as new nodes are found they will be added to the teleport cluster.

In order to avoid attempting to reinstall teleport on top of an instance where it is
already present the generated teleport config will match against the node name using
the AWS account id and instance id.

Example:
```json
{
  "kind": "node",
  "version": "v2",
  "metadata": {
    "name": "${AWS_ACCOUNT_ID}-${AWS_INSTANCE_ID}",
    "labels": {
      "env": "example"
      "teleport.dev/discovered-node": "yes"
    },
  },
  "spec": {
    "public_addr": "ec2-54-194-252-215.us-west-1.compute.amazonaws.com",
    "hostname": "awsxyz"
  }
}
```

## Agent installation

In order to install the Teleport agent on EC2 instances, Teleport will serve an
install script at `/webapi/scripts/{installer-resource-name}`. Installer scripts will
be editable as a resource.

Example resource script:
```yaml
kind: installer
metadata:
  name: "installer" # default value
spec:
  # shell script that will be downloaded an run by the EC2 node
  script: |
    #!/bin/sh
    curl https://.../teleport-pubkey.asc ...
    echo "deb [signed-by=... stable main" | tee ... > /dev/null
    apt-get update
    apt-get install teleport
    teleport node configure --auth-agent=...  --join-method=iam --token-name=iam-
  # Any resource in Teleport can automatically expire.
  expires: 0001-01-01T00:00:00Z
```

Unless overridden by a user, a default teleport installer command will be
generated that is appropriate for the current running version and operating
system initially supporting DEB and RPM based distros that Teleport already
provides packages for.

The user must create a custom SSM Command document that will be used to
execute the served command. The instance of Teleport doing discovery will
attempt to automatically create the SSM document.

Example SSM aws:runCommand document:
```yaml
# name: installTeleport
---
schemaVersion: '2.2'
description: aws:runShellScript
parameters:
  token:
    types: String
    description: "(Required) The Teleport invite token to use when joining the cluster."
mainSteps:
- action: aws:downloadContent
  name: downloadContent
  inputs:
    sourceType: "HTTP"
    destinationPath: "/tmp/installTeleport.sh"
    sourceInfo:
      url: "https://teleportcluster.xyz/webapi/scripts/installer"
- action: aws:runShellScript
  name: runShellScript
  inputs:
    timeoutSeconds: '300'
    runCommand:
      - /bin/sh /tmp/installTeleport.sh "{{ token }}"
```

In order to run the new SSM document the AWS user will need IAM permissions to run
SSM commands[3] for example:

```js
{
    "Statement": [
        {
            "Action": "ssm:SendCommand",
            "Effect": "Allow",
            "Resource": [
                # Allow running commands on all us-west-2 instances
                "arn:aws:ssm:us-west-2:*:instance/*",
                 # Allows running the installTeleport docuemnt on the allowed instances
                "arn:aws:ssm:us-east-2:aws-account-ID:document/installTeleport"
            ]
        },
		// "CreateDocument" and "GetDocument" permissions are required
		// to automatically create the document
        {
            "Action": "ssm:CreateDocument",
            "Effect": "Allow",
            "Resource": [ "*" ]
        },
		{
            "Action": "ssm:GetDocument",
            "Effect": "Allow",
            "Resource": [ "*" ]
        }
    ]
}
```

The machines being discovered will need to allow recieving `ec2messages` in
order to recieve the SSM commands:

```js
{
	"Statement": [
		{
			"Action": "ec2messages:GetMessages"
			"Effect": "Allow"
		}
	]
}
```

On AWS, Amazon Linux and Ubuntu LTS (16.04, 18.04, 20.04) come with the SSM agent
preinstalled[4].

In order to allow nodes to create tokens for the purposes of sending invites to EC2
instances a new system role will be added -- `RoleNodeDiscovery`, that will have
permissions to create tokens.

Each EC2 instance that is to be discovered will also require that they have an IAM
role attached, in order to be able to send and recieve messages for the SSM agent.

Example:

```json
{
    "Statement": [
        {
            "Action": "ec2messages:*",
            "Effect": "Allow",
            "Resource": [
                # Allow running commands on all us-west-2 instances
                "*"
            ]
        }
    ]
}
```


## teleport.yaml generation

The `teleport node configure` subcommand will be used to generate a
new /etc/teleport.yaml file:
```sh
teleport node configure
    --auth-server=auth-server.example.com [auth server that is being connected to]
    --token="$1" # passed via parameter from SSM document
    --labels="teleport.dev/instance-id=${INSTANCE_ID},teleport.dev/account-id=${ACCOUNT_ID}"
```
This will create generate a file with the following contents:

```yaml
teleport:
  nodename: "$accountID-$instanceID"
  auth_servers:
    - "auth-server.example.com:3025"
  join_params:
    token_name: token
discovery_service:
  enabled: yes
  labels:
    teleport.dev/origin: "cloud"
```


### Agentless installation

In addition to supporting automatic Teleport agent installation, an agentless option
will also be supported. This mode will update the OpenSSH CA to use the Teleport CA
without installing the full Teleport Agent.

The teleport ca and host keys will be generated and then uploaded to
the the AWS Secret store where, for each node discovered a new secret
will be created, each secret will have the following contents

```json
{
	"ca_key": "...",
	"host_key": "...",
	"host_cert": "...",
}
```

This will require changes to Teleport in order to support RBAC of OpenSSH based
nodes and to support showing the nodes in tsh/web ui.

This mode can be enabled by setting `agentless: true` in the matcher. When the
matcher includes this, a predefined script for agentless installation will be used for
the endpoint.

Example agentless config:

```yaml
discovery_service:
  enabled: "yes"
  aws:
  - types: ["ec2"]
    regions: ["us-west-1"]
    tags:
      "teleport": "yes" # aws tags to match
    install:
      agentless: true
      # default to this as a result of agentless: true
      script_name: "default-agentless-installer"
      sshd_config: "/etc/ssh/sshd_config" # default path
    ssm:
      # default to this as a result of agentless: true
      document_name: "TeleportAgentlessDiscoveryInstaller"
```


An agentless specific SSM document will be required. The `teleport discovery bootstrap`
command will need to be updated to create SSM documents appropriate for agentless discovery.

Example SSM document:
```yaml
# name: TeleportAgentlessDiscoveryInstaller
---
schemaVersion: '2.2'
description: aws:runShellScript
parameters:
  caKey:
    types: String
    description: "(Required) The Teleport CA to sshd will trust."
  hostCert:
    types: String
    description: "(Required) The Teleport host cert to use."
  sshdConfigPath:
    types: String
    description: "(Required) The path to the sshd config file."
mainSteps:
- action: aws:downloadContent
  name: downloadContent
  inputs:
    sourceType: "HTTP"
    destinationPath: "/tmp/installTeleport.sh"
    sourceInfo:
      url: "https://teleportcluster.xyz/webapi/scripts/default-agentless-installer"
- action: aws:runShellScript
  name: runShellScript
  inputs:
    timeoutSeconds: '300'
    runCommand:
      - export CERT_SECRETS='{{ secretName }}'
      - export SSHD_CONFIG='{{ sshdConfigPath }}'
      - /bin/sh /tmp/installTeleport.sh
```

Agentless mode will serve a different install script resource named
`default-agentless-installer`. Which will be used to update and restart the sshd
configuration.

Possible agentless installer script:

```bash
(
  flock -n 9 || exit 1

  if grep -q 'TrustedUserCAKeys /etc/ssh/teleport_user_ca.pub' "$SSHD_CONFIG"; then
	exit 0
  fi

  if [ "$distro_id" = "debian" ] || [ "$distro_id" = "ubuntu" ]; then
	sudo apt-get install -y awscli jq
  elif [ "$distro_id" = "amzn" ] || [ "$distro_id" = "rhel" ]; then
    sudo yum install -y awscli jq
  fi

  CA_KEY=$(aws secretsmanager get-secret-value --output=json --secret-id="$CERT_SECRETS" | jq -r '.["SecretString"] | fromjson."ca_key"')
  HOST_KEY=$(aws secretsmanager get-secret-value --output=json --secret-id="$CERT_SECRETS" | jq -r '.["SecretString"] | fromjson."host_key"')
  HOST_CERT=$(aws secretsmanager get-secret-value --output=json --secret-id="$CERT_SECRETS" | jq -r '.["SecretString"] | fromjson."host_cert"')

  echo "$CA_KEY" > /etc/ssh/teleport_user_ca.pub
  echo 'TrustedUserCAKeys /etc/ssh/teleport_user_ca.pub' >> $SSHD_CONFIG

  echo "$HOST_KEY" > /etc/ssh/teleport
  chmod 0600 /etc/ssh/teleport
  echo "$HOST_CERT" > /etc/ssh/teleport-cert.pub
  chmod 0600 /etc/ssh/teleport-cert.pub

  echo 'HostKey /etc/ssh/teleport' >> $SSHD_CONFIG
  echo 'HostCertificate /etc/ssh/teleport-cert.pub' >> $SSHD_CONFIG

  if ! sshd -t; then
	echo "sshd_config is in a bad state, exiting without reloading"
	exit 1
  fi
  systemctl restart sshd

) 9>/var/lock/teleport_install.lock
```

The discovery agent will need CRUD permissions on `host_cert` resources as well as
permission to create Node resources.


### Including AWS Tags as Teleport labels

The AWS tags on discovered EC2 instances will be included as Teleport labels on the
discovered Nodes.

In order to achieve this a helper resource named `DiscoveredServer` will be
introduced with will store metadata about discovered nodes that was retrieved via the
AWS API.

When Teleport is installed and registers the EC2 instance, the Auth server will check
for a corresponding `DIscoveredServer` resource by matching on `instance-id` and
`account-id` labels. If there is a matching `DiscoveredServer`, it will create a
`Server` resource using the metadata from the `DIscoveredServer` and ignore labels
sent via heartbeat from the node.

## UX

### User has 1 account to discover servers on

#### Teleport config

Discovery server:
```yaml
teleport:
  ...
auth_service:
  enabled: "yes"
discovery_service:
  enabled: "yes"
  aws:
   - types: ["ec2"]
     regions: ["eu-central-1"]
     tags:
       "teleport": "yes"
     install:
       join_params:
         token_name:  aws-discovery-iam-token # default value
     ssm:
       document: "TeleportDiscoveryInstaller" # default value
```

#### AWS configuration and IAM permissions

An SSM document must be created to download and run the teleport install script.
The script will be generated using a configuration appropriate for the system
running Teleport.

```yaml
# name: installTeleport
---
schemaVersion: '2.2'
description: aws:runShellScript
parameters:
  token:
    types: String
    description: "(Required) The Teleport invite token to use when joining the cluster."
mainSteps:
- action: aws:downloadContent
  name: downloadContent
  inputs:
    sourceType: "HTTP"
    destinationPath: "/tmp/installTeleport.sh"
    sourceInfo:
      url: "https://teleportcluster.xyz/webapi/scripts/installer"
- action: aws:runShellScript
  name: runShellScript
  inputs:
    timeoutSeconds: '300'
    runCommand:
      - /bin/sh /tmp/installTeleport.sh "{{ token }}"
```

The discovery node should have IAM permissions to call ec2:SendCommand and then
limit it to the `installTeleport` document:

```js
{
    "Statement": [
        {
            "Action": "ssm:SendCommand",
            "Effect": "Allow",
            "Resource": [
                # Allow running commands on all instances
                "*",
                # allow running the installTeleport document
                "arn:aws:ssm:*:aws-account-ID:document/installTeleport"
            ]
        }
    ]
}
```

The SSH discovery node should have permission to call `ec2:DescribeInstances`
```js
{
    "Statement": [
        {
            "Action": [
                "ec2:DescribeInstances",
            ],
            "Effect": "Allow",
            "Resource": [
                "*", # for example, allow on all ec2 instance with SSM availablea
            ]
        }
    ]
}
```

Nodes being discovered will need permission to `GetMessages`
```json
{
	"Statement": [
		{
			"Action": "ec2messages:GetMessages"
			"Effect": "Allow"
		}
	]
}
```

## Security Considerations


## Future work

### Assume roles

In the future the option to include a list of IAM roles to assume for
different accounts may be included:

```yaml
discovery_service:
  enabled: "yes"
  aws:
  - types: ["ec2"]
    regions: ["us-west-1"]
    tags:
      "teleport": "yes"
    ssm_command_document: ssm_command_document_name
    roles: # list of ARNs for IAM roles to assume
      - "arn:aws:iam::222222222222:role/teleport-DescribeInstancesInstall-role"
```

## Refs:
[1]: https://goteleport.com/docs/setup/guides/joining-nodes-aws-iam/
[2]: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstances.html
[3]: https://docs.aws.amazon.com/systems-manager/latest/userguide/security_iam_id-based-policy-examples.html
[4]: https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-restrict-command-access.html
[5]: https://docs.aws.amazon.com/systems-manager/latest/userguide/ssm-agent.html
