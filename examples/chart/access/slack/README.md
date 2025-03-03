# Teleport Access Request Slack Plugin

This chart sets up and configures a Deployment for the Access Request Slack plugin.

## Installation

See the [Access Requests with Slack guide](https://goteleport.com/docs/access-controls/access-request-plugins/ssh-approval-slack/).

## Values

The following values can be set for the Helm chart:

<table>
  <tr>
    <th>Name</th>
    <th>Description</th>
    <th>Type</th>
    <th>Default</th>
    <th>Required</th>
  </tr>

  <tr>
    <td><code>teleport.address</code></td>
    <td>Host/port combination of the teleport auth server</td>
    <td>string</td>
    <td><code>""</code></td>
    <td>yes</td>
  </tr>
  <tr>
    <td><code>teleport.identitySecretName</code></td>
    <td>Name of the Kubernetes secret that contains the credentials for the connection</td>
    <td>string</td>
    <td><code>""</code></td>
    <td>yes</td>
  </tr>
  <tr>
    <td><code>teleport.identitySecretPath</code></td>
    <td>Key of the field in the secret specified by <code>teleport.identitySecretName</code></td>
    <td>string</td>
    <td><code>"auth_id"</code></td>
    <td>yes</td>
  </tr>

  <tr>
    <td><code>slack.token</code></td>
    <td>Slack API token</td>
    <td>string</td>
    <td><code>""</code></td>
    <td>yes</td>
  </tr>
  <tr>
    <td><code>slack.tokenFromSecret</code></td>
    <td>Kubernetes secret to read the token from instead of <code>slack.token</code></td>
    <td>string</td>
    <td><code>""</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>slack.tokenSecretPath</code></td>
    <td>The path of the token in the secret described by <code>slack.tokenFromSecret</code></td>
    <td>string</td>
    <td><code>"slackToken"</code></td>
    <td>no</td>
  </tr>

  <tr>
    <td><code>roleToRecipients</code></td>
    <td>
      Mapping of roles to a list of channels and Slack emails. <br />
      Example:
      <pre>
"dev" = ["dev-access-requests", "user@example.com"]
"*" = ["access-requests"]</pre>
    </td>
    <td>map</td>
    <td><code>{}</code></td>
    <td>yes</td>
  </tr>

  <tr>
    <td><code>log.output</code></td>
    <td>
      Logger output. Could be <code>"stdout"</code>, <code>"stderr"</code> or a file name,
      eg. <code>"/var/lib/teleport/slack.log"</code>
    </td>
    <td>string</td>
    <td><code>"stdout"</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>log.severity</code></td>
    <td>
      Logger severity. Possible values are <code>"INFO"</code>, <code>"ERROR"</code>,
      <code>"DEBUG"</code> or <code>"WARN"</code>.
    </td>
    <td>string</td>
    <td><code>"INFO"</code></td>
    <td>no</td>
  </tr>

  <tr>
    <td><code>annotations.config</code></td>
    <td>
      Annotations to add to the configmap.
    </td>
    <td>map</td>
    <td><code>{}</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>annotations.deployment</code></td>
    <td>
      Annotations to add to the deployment.
    </td>
    <td>map</td>
    <td><code>{}</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>annotations.pod</code></td>
    <td>
      Annotations to add to every pod created by the deployment.
    </td>
    <td>map</td>
    <td><code>{}</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>annotations.secret</code></td>
    <td>
      Annotations to add to the secret.
    </td>
    <td>map</td>
    <td><code>{}</code></td>
    <td>no</td>
  </tr>
</table>
