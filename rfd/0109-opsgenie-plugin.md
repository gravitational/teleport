# OPSGenie Integration RFD
## Required approvers

Engineering:
Product:
Security:

## What

This RFD proposes a plugin that allows Teleport to integrate with OpsGenie. This plugin will differ from existing plugins by being part of the Teleport binary directly.

## Success criteria
Users are able to configure teleport to automatically create alerts in OpsGenie from access requests.
Alerts were chosen over incidents in OpsGenie for high priority alerts that indicate a service interruption.
Users are also able to configure auto approval flows to be met under certain conditions. E.g when a requester is on-call 

## Configuration UX

The plugin will be configured using a toml file containing the required to interact with both Teleport access and the OpsGenie API.

```
[teleport]
addr = "example.com:3025" # Teleport Auth Server GRPC API address
client_key = "/var/lib/teleport/plugins/opsgenie/auth.key" # Teleport GRPC client secret key
client_crt = "/var/lib/teleport/plugins/opsgenie/auth.crt" # Teleport GRPC client certificate
root_cas = "/var/lib/teleport/plugins/opsgenie/auth.cas" # Teleport cluster CA certs

[opsgenie]
api_key = "key" # Opsgenie API Key

[log]
output = "stderr" # Logger output. Could be "stdout", "stderr" or "/var/lib/teleport/opsgenie.log"
severity = "INFO" # Logger severity. Could be "INFO", "ERROR", "DEBUG" or "WARN".
```

### Getting an OpsGenie API key

In the OpsGenie web UI go to Settings -> App settings -> API key management. Create key with Read, Create and Update access.

### Executing

The plugin will be started using a command of the form

```
Teleport opsgenie start –config <config file location>
```

## UX

Once an access request has been created, the OpsGenie plugin will create an alert in service specified in the config file using the OpsGenie Alert API Create endpoint. 

The appropriate on call responder can then click into the provided link and approve or deny the access request.

For auto approval of certain access requests the access request will be auto approved if the requesting user is on-call in one of the services provided in request annotation.

Once an access request has been approved or denied the plugin will add a note to the alert and close the relevant alert tied to that access request.

## Implementation details
In this section we will take a look at how the plugin will interact with the OpsGenie API.

### Authorization

The plugin will use the API key provided in the OpsGenie config file when interacting with the OpsGenie API. This API key will be included in the headers of the requests made.

```
Header Key: Authorization
Header Value: GenieKey $apiKey
```

### Creating alerts
When the OpsGenie plugin creates alerts for incoming access requests the Create alert request will be of the form

```
{
	"message": "Access request from <someuser>",
	"description":"<someuser> requested permissions for roles <someroles> on Teleport at <someTime>.
 	Reason: <someReason>
 	To approve or deny the request, proceed to <link to the access request>",
	"responders":[
    	....
	],
	"tags": ["TeleportAccessRequest"],
	"priority":"<somePriority>"
}
```

When the access request has been approved or denied the alert created in OpsGenie will have a note added to it using the ‘Add note to alert’ endpoint. Then the alert will be closed using the ‘Close alert’ endpoint.

```
<Reviewer> reviewed the request at <someTime>.
Resolution: <StateEmojiAsUsedByExistingPLugins> <State>.
Reason: <Reason>
```

### Auto approval

To check if the requesting user of a request is currently on-call the ‘Who is on call API’s ‘Get on calls’ endpoint will be used. 'https://<configured-opsgenie-address>/v2/schedules/<SheduleName>/on-calls?scheduleIdentifierType=name'

Similar to the existing Pagerduty plugin for auto-approval to work, the user creating an Access Request must have a Teleport username that is also the email address associated with an OpsGenie account.

Access requests will be mapped to OpsGenie alerts by including the Access request ID in the tags field of the note. 

Shared code between the teleport-plugins found in lib is not too extensive and the simplest method to handle this when adding the OpsGenie plugin would be to simply duplicate what is needed for now.

## Security considerations

Potential for users to get access requests auto approved if they can get themselves onto the current on call rotation.
Since Teleport usernames are assumed to match the OpsGenie email address when checking on call there is potential for access requests to be auto approved unintentionally.

