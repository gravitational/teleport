# Teleport Access Request Datadog Incident Management Plugin

This chart sets up and configures a Deployment for the Access Request Datadog Incident Management plugin.

## Installation

See the [Access Requests with Datadog Incident Management guide](https://goteleport.com/docs/access-controls/access-request-plugins/datadog-hosted/).

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
    <td>Host/port combination of the teleport Auth Service</td>
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
    <td><code>datadog.apiEndpoint</code></td>
    <td>Datadog API Endpoint. See documentation for supported Datadog Sites: https://docs.datadoghq.com/getting_started/site/#access-the-datadog-site</td>
    <td>string</td>
    <td><code>"https://api.datadoghq.com"</code></td>
    <td>yes</td>
  </tr>
  <tr>
    <td><code>datadog.apiKey</code></td>
    <td>Datadog API Key</td>
    <td>string</td>
    <td><code></code></td>
    <td>yes</td>
  </tr>
  <tr>
    <td><code>datadog.apiKeyFromSecret</code></td>
    <td>Kubernetes secret to read the api key from instead of <code>datadog.apiKey</code></td>
    <td>string</td>
    <td><code>""</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>datadog.apiKeySecretPath</code></td>
    <td>The path of the api key in the secret described by <code>datadog.apiKeyFromSecret</code></td>
    <td>string</td>
    <td><code>"datadogApiKey"</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>datadog.applicationKey</code></td>
    <td>Datadog Application Key</td>
    <td>string</td>
    <td><code></code></td>
    <td>yes</td>
  </tr>
  <tr>
    <td><code>datadog.applicationKeyFromSecret</code></td>
    <td>Kubernetes secret to read the application key from instead of <code>datadog.applicationKey</code></td>
    <td>string</td>
    <td><code>""</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>datadog.applicationKeySecretPath</code></td>
    <td>The path of the application key in the secret described by <code>datadog.applicationKeyFromSecret</code></td>
    <td>string</td>
    <td><code>"datadogApplicationKey"</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>datadog.fallbackRecipient</code></td>
    <td>The default recipient for Access Request notifications. Accepts a Datadog user email or team handle.</td>
    <td>string</td>
    <td><code>""</code></td>
    <td>yes</td>
  </tr>
  <tr>
    <td><code>datadog.severity</code></td>
    <td>Datadog Incident Severity</td>
    <td>string</td>
    <td><code>"SEV-3"</code></td>
    <td>no</td>
  </tr>

  <tr>
    <td><code>log.output</code></td>
    <td>
      Logger output. Could be <code>"stdout"</code>, <code>"stderr"</code> or a file name,
      eg. <code>"/var/lib/teleport/datadog.log"</code>
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
