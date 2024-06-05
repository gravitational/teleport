# Teleport Access Request Jira Plugin

This chart sets up and configures a Deployment for the Access Request Jira plugin.

## Installation

See the [Access Requests with JIRA guide](https://goteleport.com/docs/access-controls/access-request-plugins/ssh-approval-jira/).

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
    <td><code>chartMode</code></td>
    <td>
      When set to <code>"aws"</code>, it'll add the proper annotations to the created service
      to ensure the AWS LoadBalancer is set up properly. Additional annotations can be added
      using <code>serviceAnnotations</code>.
    </td>
    <td>string</td>
    <td><code>""</code></td>
    <td>no</td>
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
    <td><code>jira.url</code></td>
    <td>URL of the Jira server</td>
    <td>string</td>
    <td><code>""</code></td>
    <td>yes</td>
  </tr>
  <tr>
    <td><code>jira.username</code></td>
    <td>Username of the bot user in Jira to use for creating issues.</td>
    <td>string</td>
    <td><code>""</code></td>
    <td>yes</td>
  </tr>
  <tr>
    <td><code>jira.apiToken</code></td>
    <td>API token of the bot user.</td>
    <td>string</td>
    <td><code>""</code></td>
    <td>yes</td>
  </tr>
  <tr>
    <td><code>jira.project</code></td>
    <td>Short code of the project in Jira in which issues will be created</td>
    <td>string</td>
    <td><code>""</code></td>
    <td>yes</td>
  </tr>
  <tr>
    <td><code>jira.issueType</code></td>
    <td>Type of the issues to be created on access requests (eg. Bug, Task)</td>
    <td>string</td>
    <td><code>"Task"</code></td>
    <td>no</td>
  </tr>

  <tr>
    <td><code>http.publicAddress</code></td>
    <td>The domain name which will be assigned to the service</td>
    <td>string</td>
    <td><code>""</code></td>
    <td>yes</td>
  </tr>
  <tr>
    <td><code>http.tlsFromSecret</code></td>
    <td>Name of the Kubernetes secret where the TLS key and certificate will be mounted</td>
    <td>string</td>
    <td><code>""</code></td>
    <td>yes</td>
  </tr>
  <tr>
    <td><code>http.tlsKeySecretPath</code></td>
    <td>Path of the TLS key in the secret specified by <code>http.tlsFromSecret</code></td>
    <td>string</td>
    <td><code>""</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>http.tlsCertSecretPath</code></td>
    <td>Path of the TLS certificate in the secret specified by <code>http.tlsFromSecret</code></td>
    <td>string</td>
    <td><code>""</code></td>
    <td>no</td>
  </tr>

  <tr>
    <td><code>http.basicAuth.username</code></td>
    <td>Username for the basic authentication. The plugin will require a m atching `Authorization` header in case both the username and the password are specified.</td>
    <td>string</td>
    <td><code>""</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>http.basicAuth.password</code></td>
    <td>Password for the basic authentication. The plugin will require a m atching `Authorization` header in case both the username and the password are specified.</td>
    <td>string</td>
    <td><code>""</code></td>
    <td>no</td>
  </tr>

  <tr>
    <td><code>log.output</code></td>
    <td>
      Logger output. Could be <code>"stdout"</code>, <code>"stderr"</code> or a file name,
      eg. <code>"/var/lib/teleport/jira.log"</code>
    </td>
    <td>string</td>
    <td><code>"stdout"</code></td>
  </tr>
  <tr>
    <td><code>log.severity</code></td>
    <td>
      Logger severity. Possible values are <code>"INFO"</code>, <code>"ERROR"</code>,
      <code>"DEBUG"</code> or <code>"WARN"</code>.
    </td>
    <td>string</td>
    <td><code>"INFO"</code></td>
  </tr>
</table>
