# Teleport Access Request PagerDuty Plugin

This chart sets up and configures a Deployment for the Access Request PagerDuty plugin.

## Installation

See the [Access Requests with PagerDuty guide](https://goteleport.com/docs/access-controls/access-request-plugins/ssh-approval-pagerduty/).

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
    <td><code>pagerduty.apiKey</code></td>
    <td>PagerDuty API Key</td>
    <td>string</td>
    <td><code></code></td>
    <td>yes</td>
  </tr>
  <tr>
    <td><code>pagerduty.apiKeyFromSecret</code></td>
    <td>Kubernetes secret to read the api key from instead of <code>pagerduty.apiKey</code></td>
    <td>string</td>
    <td><code>""</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>pagerduty.apiKeySecretPath</code></td>
    <td>The path of the api key in the secret described by <code>pagerduty.apiKeyFromSecret</code></td>
    <td>string</td>
    <td><code>"pagerdutyApiKey"</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>pagerduty.userEmail</code></td>
    <td>PagerDuty bot user email</td>
    <td>string</td>
    <td><code>""</code></td>
    <td>yes</td>
  </tr>

  <tr>
    <td><code>log.output</code></td>
    <td>
      Logger output. Could be <code>"stdout"</code>, <code>"stderr"</code> or a file name,
      eg. <code>"/var/lib/teleport/pagerduty.log"</code>
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
</table>
