---
authors: Marco Dinis (marco.dinis@goteleport.com)
state: draft
---

# RFD 223 - Multiple Agents in EC2 Auto Discovery

## Required Approvers

* Engineering: @r0mant && @hugo
* Product: @r0mant

## What

Allow the installation of multiple Teleport Agents in the same EC2 instance.

## Why

Teleport can auto discover EC2 instances and install teleport into them.
They will be configured to join the cluster, so that Teleport users can access them.

Some deployments require high availability, where two Teleport Clusters, with different versions, must be able access to the same EC2 instance.
This is also known as blue-green deployment.

Having two clusters discovering the same EC2 instance doesn't work.
After installing Teleport from one Cluster, the second attempt (from the other Cluster) fails because it conflicts with the existing installation: teleport installation, configuration and data are unique per instance.

## Details

As described in [RFD 184](./0184-agent-auto-updates.md), `teleport-update` supports suffixing the installation.

This allows multiple teleport agents to run in the same instance, without conflicting with each other.

Different versions are supported and each installation follows the Cluster Auto Update configurations.

### UX

#### User stories

**Alice wants to access EC2 from Cluster Blue and from Cluster Green, new installation**

Alice logs in to Cluster Blue, and follows the EC2 Auto Discover guide.

After completing the guide, a new SSM Document is created with the following contents:
```yaml
schemaVersion: '2.2'
description: aws:runShellScript
parameters:
  # ...
  env:
    type: String
    description: "Environment variables exported to the script. Format 'ENV=var FOO=bar'"
    default: "X=$X"
mainSteps:
# ...
- action: aws:runShellScript
  name: runShellScript
  inputs:
    runCommand:
      - 'export {{ env }}; /bin/sh /tmp/installTeleport.sh "{{ token }}"'
```

And a new Teleport Agent is running the Discovery Service with the following configuration:
```yaml
version: v3
discovery_service:
  enabled: true
  aws:
   - types: ["ec2"]
     install:
        suffix: cluster-blue # this is a new field
# ...
```

Only the `discovery_service.aws[].install.suffix` has to be defined, when compared to the current guide.

Alice now follows the exact same guide, but this time for Cluster Green.

This time the configuration of the Discovery Service looks like this:
```yaml
version: v3
discovery_service:
  enabled: true
  aws:
   - types: ["ec2"]
     install:
        suffix: cluster-green # this is a new field
# ...
```

Again, only the `discovery_service.aws[].install.suffix` has to be defined, when compared to the current guide.


**Alice already has access to EC2 from Cluster Green and wants to access Cluster Blue**

Alice installed Teleport using the old method: no suffix was used.

Now, she is planning for high availability and wants to access the same EC2 instance from their new cluster: Cluster Blue.

Alice follows the EC2 Auto Discover guide and creates the following configuration:
```yaml
version: v3
discovery_service:
  enabled: true
  aws:
   - types: ["ec2"]
     install:
        suffix: cluster-blue # this is a new field
# ...
```

Only the `discovery_service.aws[].install.suffix` has to be defined, when compared to the current guide.

This installs Teleport using a suffix, and runs two agents:
- one using the global installation (ie, configuration at `/etc/teleport.yaml` and binaries at `/usr/local/bin/teleport`), which is connecting to Cluster Green
- another, suffixed, installation which is connecting to Cluster Blue

The agents do not collide with each other and are able to follow different teleport versions.
