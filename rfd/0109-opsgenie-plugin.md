# Opsgenie Integration RFD
## Required approvers

Engineering:
Product:
Security:

## What

This RFD proposes a plugin that allows Teleport to integrate with Opsgenie, allowing access requests to show up as alerts in Opsgenie. This plugin will differ from existing plugins by being part of the Teleport binary directly.

## Success criteria
Users are able to configure teleport to automatically create alerts in Opsgenie from access requests.
Alerts were chosen over incidents in Opsgenie as incidents are intended to be used for high priority alerts that indicate a service interruption.
Users are also able to configure auto approval flows to be met under certain conditions. E.g when a requester is on-call 

## Configuration UX

The plugin will be configured in Teleport's config yaml file. The required fields will be added to a 'plugins' section containing the required information to interact with both Teleport access and the Opsgenie API.

```
plugins:
    opsgenie:
        api_key: "path/to/key" # File containing Opsgenie API Key
        opsgenie_addr: "example.app.opsgenie.com" # Address of Opsgenie
        priority: "2" # Priority to create Opsgenie alerts with
        alert_tags: ["example-tag"] # List of tags to be added to alerts created in Opsgenie
        auto_approval: true # Whether or not to enable auto approval
```

The logging configuration will be shared with the main Teleport process.

### Getting an Opsgenie API key

In the Opsgenie web UI go to Settings -> App settings -> API key management. Create key with Read, Create and Update access.

### Executing

The plugin will be started using a command of the form

```
teleport start --config /etc/teleport.yaml
```

## UX

Once an access request has been created, the Opsgenie plugin will create an alert in the service specified in the request annotation using the Opsgenie Alert API Create endpoint. 

The appropriate on call responder can then click into the provided link and approve or deny the access request.

For auto approval of certain access requests the access request will be auto approved if the requesting user is on-call in one of the services provided in request annotation. (Provided auto approval is enabled in the config)

Once an access request has been approved or denied the plugin will add a note to the alert and close the relevant alert tied to that access request.

## Implementation details
In this section we will take a look at how the plugin will interact with the Opsgenie API.

### Authorization

The plugin will use the API key provided in the Opsgenie config file when interacting with the Opsgenie API. This API key will be included in the headers of the requests made.

```
Header Key: Authorization
Header Value: genieKey $apiKey
```

### Creating alerts
When the Opsgenie plugin creates alerts for incoming access requests the Create alert request will be of the form

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

On every review of the access request the alert created in Opsgenie will have a note added to it using the ‘Add note to alert’ endpoint. Then the alert will be closed using the ‘Close alert’ endpoint once the alert has either been approved or denied.

```
<Reviewer> reviewed the request at <someTime>.
Resolution: <StateEmojiAsUsedByExistingPLugins> <State>.
Reason: <Reason>
```

### Auto approval

To check if the requesting user of a request is currently on-call the ‘Who is on call API’s ‘Get on calls’ endpoint will be used. 'https://<configured-opsgenie-address>/v2/schedules/<SheduleName>/on-calls?scheduleIdentifierType=name'

Similar to the existing Pagerduty plugin for auto-approval to work, the user creating an Access Request must have a Teleport username that is also the email address associated with an Opsgenie account.

Access requests will be mapped to Opsgenie alerts by including the Access request ID in the tags field of the note. 

Shared code between the teleport-plugins found in lib is not too extensive and the simplest method to handle this when adding the Opsgenie plugin would be to simply duplicate what is needed for now.

### Note

Opsgenie will only ever read Teleport state and never write it.

## Security considerations

Potential for users to get access requests auto approved if they can get themselves onto the current on call rotation.
Since Teleport usernames are assumed to match the Opsgenie email address when checking on call there is potential for access requests to be auto approved unintentionally.

