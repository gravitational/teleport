# Teleport Event Handler Plugin

This chart sets up and configures a Deployment for the Event Handler plugin.

## Installation

See the [Access Requests with Slack guide](https://goteleport.com/docs/access-controls/access-request-plugins/ssh-approval-slack/).

## Settings

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
    <td><code>eventHandler.storagePath</code></td>
    <td>Path to the directory where <code>event-handler</code>'s state is stored</td>
    <td>string</td>
    <td><code>"/var/lib/teleport/plugins/event-handler/storage"</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>eventHandler.timeout</code></td>
    <td>Maximum time to wait for incoming events before sending them to fluentd.</td>
    <td>string</td>
    <td><code>"10s"</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>eventHandler.batch</code></td>
    <td>Maximum number of events fetched from Teleport in one request</td>
    <td>string</td>
    <td><code>20</code></td>
    <td>no</td>
  </tr>

  <tr>
    <td><code>fluentd.url</code></td>
    <td>URL of fluentd where the event logs will be sent to.</td>
    <td>string</td>
    <td><code>""</code></td>
    <td>yes</td>
  </tr>
  <tr>
    <td><code>fluentd.sessionUrl</code></td>
    <td>URL of fluentd where the session logs will be sent to.</td>
    <td>string</td>
    <td><code>""</code></td>
    <td>yes</td>
  </tr>
  <tr>
    <td><code>fluentd.secretName</code></td>
    <td>
      Name of the secret where credentials for the connection is stored.
      It must contain the client's private key, certificate and fluentd's
      CA certificate. See the default paths below.
    </td>
    <td>string</td>
    <td><code>""</code></td>
    <td>yes</td>
  </tr>
  <tr>
    <td><code>fluentd.caPath</code></td>
    <td>Path of the CA certificate in the secret described by <code>fluentd.secretName</code>.</td>
    <td>string</td>
    <td><code>"ca.crt"</code></td>
  </tr>
  <tr>
    <td><code>fluentd.certPath</code></td>
    <td>Path of the client's certificate in the secret described by <code>fluentd.secretName</code>.</td>
    <td>string</td>
    <td><code>"client.crt"</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>fluentd.keyPath</code></td>
    <td>Path of the client private key in the secret described by <code>fluentd.secretName</code>.</td>
    <td>string</td>
    <td><code>"client.key"</code></td>
    <td>no</td>
  </tr>

  <tr>
    <td><code>persistentVolumeClaim.enabled</code></td>
    <td>
      Instructs the Helm chart to include a PersistentVolumeClaim for the storage. This storage
      will be mounted to the path specified by <code>eventHandler.storagePath</code>.
    </td>
    <td>boolean</td>
    <td><code>false</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>persistentVolumeClaim.size</code></td>
    <td>Sets the size of the created PersistentVolumeClaim. Don't forget to append the proper suffix!</td>
    <td>string</td>
    <td><code>"1Gi"</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>persistentVolumeClaim.storageClassName</code></td>
    <td>
      Sets the storage class name of the created PersistentVolumeClaim. Kubernetes will use the default
      one when omitted.
    </td>
    <td>string</td>
    <td><code>""</code></td>
    <td>no</td>
  </tr>
  <tr>
    <td><code>persistentVolumeClaim.existingClaim</code></td>
    <td>
      Specifies an already existing PersistentVolumeClaim which should be mounted to the path specified
      by <code>eventHandler.storagePath</code>. <code>persistentVolumeClaim.enabled</code> must be set to false for this
      option to take precedence. Ignored when <code>persistentVolumeClaim.enabled</code> is true.
    </td>
    <td>string</td>
    <td><code>""</code></td>
    <td>no</td>
  </tr>
</table>
