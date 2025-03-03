# Teleport Access Request Email Plugin

This chart sets up and configures a Deployment for the Access Request Email plugin.

## Installation

See the [Access Requests with Email guide](https://goteleport.com/docs/admin-guides/access-controls/access-request-plugins/ssh-approval-email/).

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
    <td>no</td>
  </tr>

  <tr>
    <td><code>mailgun.enabled</code></td>
    <td>
      Specifies if the Mailgun integration should be enabled. Mutually exclusive with <code>smtp.enabled</code>.
      In the case of both values are set to true, <code>mailgun.enabled</code> will take precedence.
    </td>
    <td>boolean</td>
    <td><code>false</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>mailgun.domain</code></td>
    <td>Domain name of the Mailgun instance</td>
    <td>string</td>
    <td><code>""</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>mailgun.privateKey</code></td>
    <td>Private key for accessing the Mailgun instance</td>
    <td>string</td>
    <td><code>""</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>mailgun.privateKeyFromSecret</code></td>
    <td>Kubernetes secret to read the private key from instead of using <code>mailgun.privateKey</code></td>
    <td>string</td>
    <td><code>""</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>mailgun.privateKeySecretPath</code></td>
    <td>The path of the private key in the secret described by <code>mailgun.privateKeyFromSecret</code></td>
    <td>string</td>
    <td><code>"mailgunPrivateKey"</code></td>
    <td>no</td>
  </tr>

  <tr>
    <td><code>smtp.enabled</code></td>
    <td>
      Specifies if the MailSMTPgun integration should be enabled. Mutually exclusive with <code>mailgun.enabled</code>.
      In the case of both values are set to true, <code>mailgun.enabled</code> will take precedence.
    </td>
    <td>boolean</td>
    <td><code>false</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>smtp.host</code></td>
    <td>SMTP host.</td>
    <td>string</td>
    <td><code>""</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>smtp.port</code></td>
    <td>Port of the SMTP server.</td>
    <td>integer</td>
    <td><code>587</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>smtp.username</code></td>
    <td>Username to be used with the SMTP server.</td>
    <td>string</td>
    <td><code>""</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>smtp.password</code></td>
    <td>Password to be used with the SMTP server. Mutually exclusive with <code>smtp.passwordFile</code>.</td>
    <td>string</td>
    <td><code>""</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>smtp.passwordFromSecret</code></td>
    <td>Kubernetes secret to read the SMTP password from instead of using <code>smtp.password</code></td>
    <td>string</td>
    <td><code>""</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>smtp.passwordSecretPath</code></td>
    <td>The path of the SMTP password in the secret described by <code>smtp.passwordFromSecret</code></td>
    <td>string</td>
    <td><code>"smtpPassword"</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>smtp.starttlsPolicy</code></td>
    <td>Which policy to use for secure communications: mandatory, opportunistic or disabled.</td>
    <td>string</td>
    <td><code>"mandatory"</code></td>
    <td>no</td>
  </tr>

  <tr>
    <td><code>delivery.sender</code></td>
    <td>Email address to be used in the <code>From</code> field of the emails.</td>
    <td>string</td>
    <td><code>""</code></td>
    <td>yes</td>
  </tr>
  <tr>
    <td><code>delivery.recipients</code></td>
    <td>Array of the recipients the plugin should send emails.</td>
    <td>array</td>
    <td><code>[]</code></td>
    <td>no</td>
  </tr>

  <tr>
    <td><code>roleToRecipients</code></td>
    <td>
      Mapping of roles to a list of emails. <br />
      Example:
      <pre>
"dev" = ["developers@example.com", "user@example.com"]
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
      eg. <code>"/var/lib/teleport/email.log"</code>
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
