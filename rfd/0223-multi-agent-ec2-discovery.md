---
authors: Marco Dinis (marco.dinis@goteleport.com)
state: draft
---

# RFD 223 - Multiple Agents in EC2 Auto Discovery

## Required Approvers

* Engineering: @r0mant && @hugo
* Product: @r0mant

## What

Enroll the same AWS EC2 instance into multiple Teleport clusters.

## Why

Teleport auto discovers EC2 instances and installs teleport into them.
Instances are configured to join the cluster, and Teleport users can access them.

Some deployments require high availability, where two Teleport Clusters, with different versions, must be able to access the same EC2 instance.
This is also known as blue-green deployment.

Having two clusters discovering the same EC2 instance doesn't work currently.
After installing Teleport from one Cluster, the second attempt (from the other Cluster) fails because it conflicts with the existing installation: teleport installation, configuration and data are global.

## Details

### UX

#### User stories

**New installation: Alice wants to access EC2 instances from Cluster Blue and from Cluster Green**

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
        update_group: prod-blue # this is a new field
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
        update_group: prod-green # this is a new field
# ...
```

Only the `discovery_service.aws[].install.suffix` has to be defined, when compared to the current guide.

The value at `discovery_service.aws[].install.update_group` can be defined in order to set the update group in the `teleport-update` configuration.

**Alice already has access to EC2 instances from Cluster Green and wants to access them from Cluster Blue**

Alice installed Teleport using the old method: no suffix was configured.

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
        update_group: prod-blue # this is a new field
# ...
```

Only the `discovery_service.aws[].install.suffix` has to be defined, when compared to the current guide.

The value at `discovery_service.aws[].install.update_group` can be defined in order to set the update group in the `teleport-update` configuration.

This installs Teleport using a suffix, and two agents are running:
- one using the global installation (ie, configuration at `/etc/teleport.yaml` and binaries at `/usr/local/bin/teleport`), which is connecting to Cluster Green
- another, suffixed, installation which is connecting to Cluster Blue

The agents do not collide with each other and are able to follow different teleport versions.

**Alice already has access to EC2 from Cluster Green and adds an installation suffix**

Alice installed Teleport using the old method: no suffix was used and the old SSM Document exists.

After reading the guide again, she notices the `install.suffix` param and, in preparation for a blue-green deployment, sets it.

This change was effective, but now new EC2 instances are no longer being added.

_Using the AWS OIDC Integration as source of credentials_: there's now an User Task explaining that the SSM Document must be updated as well

_Using ambient (IAM Profile Role) credentials_: there's logs explaining what the error was and how the user can fix them by updating the SSM Document.

### Auto Discover EC2 instances

To allow multiple agents to run in the same host, Teleport has to use non-global locations for binaries, configuration and data.

As described in [RFD 184](./0184-agent-auto-updates.md), `teleport-update` supports installing and configuring teleport agents in non standard locations.
This is defined using the `--install-suffix` flag or `TELEPORT_INSTALL_SUFFIX` environment variable and allows multiple agents to co-exist in the same instance.

Different binary versions are supported and each installation follows an independent cluster Auto Update configuration.

**New SSM Document**

The SSM Document must be updated to allow for any environment variable to be set, by the Discovery Service, before starting the installation and configuration process.

This will be accomplished by a new SSM Document Parameter and a change in the `aws:runShellScript` action:
```yaml
schemaVersion: '2.2'
description: aws:runShellScript
parameters:
  token:
    type: String
    description: "(Required) The Teleport invite token to use when joining the cluster."
  scriptName:
    type: String
    description: "(Required) The Teleport installer script to use when joining the cluster."
  env:
    type: String
    description: "Environment variables exported to the script. Format 'ENV=var FOO=bar'"
    default: "X=$X"
mainSteps:
- action: aws:downloadContent
  name: downloadContent
  inputs:
    sourceType: "HTTP"
    destinationPath: "/tmp/installTeleport.sh"
    sourceInfo:
      url: "https://teleport.example.com:443/webapi/scripts/installer/{{ scriptName }}"
- action: aws:runShellScript
  name: runShellScript
  inputs:
    timeoutSeconds: '300'
    runCommand:
      - export {{ env }}; /bin/sh /tmp/installTeleport.sh "{{ token }}"
```
A default value ensures it continues working if no `env` parameter is sent by the Discovery Service.

**Discovery Service must send the new parameter**

When the EC2 Matcher (whether is comes from the `teleport.yaml/discovery_service.aws[]`, or from `discovery_config.aws[]`) has the `install.suffix` value,
it will inject the following value into the `env` parameter:
```go
var envVars []string

if install.suffix != "" {
  envVars = append(envVars, "TELEPORT_INSTALL_SUFFIX="+install.suffix)
}

if install.updateGroup != "" {
  envVars = append(envVars, "TELEPORT_UPDATE_GROUP="+install.updateGroup)
}

params["env"] = strings.Join(envVars, " ")

// ....
output, err := req.SSM.SendCommand(ctx, &ssm.SendCommandInput{
  DocumentName: aws.String(req.DocumentName),
  InstanceIds:  validInstanceIDs,
  Parameters:   params,
})
```

**Installer script**

The installer script is downloaded by the SSM Agent (see step `downloadContent` in the SSM Document), and then executed:

```bash
#!/usr/bin/env sh
set -eu


INSTALL_SCRIPT_URL="https://teleport.example.com:443/scripts/install.sh"

echo "Offloading the installation part to the generic Teleport install script hosted at: $INSTALL_SCRIPT_URL"

TEMP_INSTALLER_SCRIPT="$(mktemp)"
curl -sSf "$INSTALL_SCRIPT_URL" -o "$TEMP_INSTALLER_SCRIPT"

chmod +x "$TEMP_INSTALLER_SCRIPT"

sudo "$TEMP_INSTALLER_SCRIPT" || (echo "The install script ($TEMP_INSTALLER_SCRIPT) returned a non-zero exit code" && exit 1)
rm "$TEMP_INSTALLER_SCRIPT"


echo "Configuring the Teleport agent"

set +x
sudo /usr/local/bin/teleport install autodiscover-node --public-proxy-addr=platform.teleport.sh:443 --teleport-package=teleport-ent --repo-channel=stable/cloud --auto-upgrade=true --azure-client-id= $@
```

The `scripts/install.sh` script will call the `teleport-update` binary with the following arguments:
```code
$ teleport-update enable --proxy teleport.example.com:443
```

After the changes in the SSM Document, the `TELEPORT_INSTALL_SUFFIX` and `TELEPORT_UPDATE_GROUP` environment variables are set.
The above command is equivalent to:
```code
$ teleport-update enable --proxy teleport.example.com:443 --install-suffix=example-suffix --group=example-group
```

This configures the following:
- teleport binaries are stored in `/opt/teleport/example-suffix/bin`
- systemd unit stored as `teleport_example-suffix`

The last statement needs to be changed because, for non-global installations because there is no binary at `/usr/local/bin/teleport`:
```bash
TELEPORT_BINARY=/usr/local/bin/teleport
if [[ -v TELEPORT_INSTALL_SUFFIX ]]; then
  TELEPORT_BINARY=/opt/teleport/${TELEPORT_INSTALL_SUFFIX}/bin/teleport
fi

sudo $TELEPORT_BINARY install autodiscover-node --public-proxy-addr=platform.teleport.sh:443 --teleport-package=teleport-ent --repo-channel=stable/cloud --auto-upgrade=true --azure-client-id= $@
```

As we can see above, after installing teleport, the script will execute the `install autodiscover-node` command.
This flow must be changed because it assumes a global installation of teleport (which is no longer true when using a suffix).
The following checks must be changed to accommodate a non-global installation:

The suffix must be passed into the `teleport install autodiscover-node` so that it knows which non-global teleport installation to look for.

This can be accomplished by looking up the `TELEPORT_INSTALL_SUFFIX` environment variable.

**`teleport install autodiscover-node`: install phase**

Currently, the `teleport install autodiscover-node` does not install teleport and only configures it: https://github.com/gravitational/teleport/blob/c20095e9d696b007866414ed4b929917814fe047/lib/srv/server/installer/autodiscover.go#L235

No check will happen if the suffix was set.

**`teleport install autodiscover-node`: configure phase**

After installing, the flow continues to the configuration steps.
For this, `teleport node configure --output=file:///etc/teleport.yaml ...` will be called.

https://github.com/gravitational/teleport/blob/c20095e9d696b007866414ed4b929917814fe047/lib/srv/server/installer/autodiscover.go#L306-L308

This must be changed to take into account the suffix when setting the output flag value: `/etc/teleport_example-suffix.yaml`.

The `--data-dir` must also be passed in order to have a different directory for this non-global installation: `/var/lib/teleport_example-suffix/`.

**`teleport install autodiscover-node`: service management (systemd)**

After configuring, teleport is started using `systemd`.
This must also be update to account for the `TELEPORT_INSTALL_SUFFIX` because the systemd unit is named after the suffix: `teleport_example-suffix`.

https://github.com/gravitational/teleport/blob/c20095e9d696b007866414ed4b929917814fe047/lib/srv/server/installer/autodiscover.go#L266-L277

### Security
The `install.suffix` parameter must be a valid `install-suffix` flag value, in the `teleport-update` binary.

Only `a-zA-Z0-9-` chars are acceptable there: https://github.com/gravitational/teleport/blob/c6010eb0db32b37f283481bed14be690c2a52d91/lib/autoupdate/agent/setup.go#L182

Validation is done when reading the `teleport.yaml/discovery_service` configuration or when writing to `discovery_config` resource.

Given this, there's no need for additional measures (like shell escaping) because the only special symbol is `-` which can't be used to inject any command.

The `install.updater_group` parameter will also be validated against the same rule: `a-zA-Z0-9-`.
The validation used in `teleport-update` or in the backend resource that stores the group is not that strict, but opting for this validation should help us prevent any shell injection.

### Proto Specifications
Add the suffix parameter

```proto
// InstallParams sets join method to use on discovered nodes
message InstallerParams {
  // ...

  // Suffix indicates the installation suffix for the teleport installation.
  // Set this value if you want multiple installations of Teleport.
  // Note: only supported for AWS EC2.
  string Suffix = 9;
}
```

### Backwards Compatibility

**SSM Document is old and `install.suffix` is not set**

Existing installations will continue working as they work now:
- SSM Document does not have the `env` parameter
- `InstallerParams` do not have the `Suffix` value

This means that Discovery Service will not send any `env` parameter in the SSM Run Command API Call.

**SSM Document is new but `install.suffix` is not set**

When the SSM Document is updated and no `install.suffix` is configured, the process will continue to work because the `env` parameter has a default (no-op) value.

**SSM Document is old but `install.suffix` is set**

It might happen that a user configures the `Suffix` param but is still using the old SSM Document.
In that case, the SSM Run Command API call will fail with:
```
Original Error: *smithy.OperationError operation error SSM: SendCommand, https response error StatusCode: 400, RequestID: <uuid>, InvalidParameters: Parameters provided in document are invalid or not supported.
```

In that situation, Discovery Service must handle the error and provide an error indicating that the user must update their SSM Document according to the docs.
The message must have a link to the docs and a link to their Amazon SSM Document.

This error will surface in:
- application logs
- User Tasks, when using an Integration as source of credentials

### Audit Events

The `ssm.run` audit event must include the `install.suffix` param.

https://github.com/gravitational/teleport/blob/a12def583af069cf0896d26e14528d499027dbfd/lib/srv/server/ssm_install.go#L511-L525

### Test Plan
Include a new testing item in EC2 Discovery section
- Join the instance into two different Teleport clusters using the `install.suffix` configuration