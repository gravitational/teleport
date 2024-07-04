# RFD 109: Opsgenie Integration

## Required approvers

Engineering: @r0mant, @marcoandredinis, @hugoShaka
Product: @klizhentas, @xinding33
Security: @reedloden, @jentfoo

## What

This RFD proposes a plugin that allows Teleport to integrate with Opsgenie, allowing access requests to show up as alerts in Opsgenie. This plugin will differ from existing plugins by being part of the Teleport binary directly.
Doing so by creating a new Teleport service 'plugin_service' with the opsgenie plugin (and any future ones) being configurable resources. This will allow the plugins to be run either by themselves or alongside other Teleport services.

## Scope

The plugin will be able to run standalone or alongside other services as part of the new plugin_service.
The plugin will support auto-approval similar to PagerDuty plugin.

## Success criteria

### Plugin service
The creation of a Teleport service where plugins can be created as configurable resources similar to the existing 'app_service' and 'db_service'.

Plugins managed through this service should have the ability to be enabled and disabled dynamically via 'tctl'.

### Opsgenie plugin
Users are able to configure teleport to automatically create alerts in Opsgenie from access requests.
Alerts were chosen over incidents in Opsgenie as incidents are intended to be used for high priority alerts that indicate a service interruption.
Users are also able to configure auto approval flows to be met under certain conditions. E.g when a requester is on-call.

## Configuration UX

The plugin service will be configured using the existing Teleport YAML file in a section called 'plugin_service'.

```
plugin_service:
    enabled: true
    plugins:
    - "type": "opsgenie"
    - "type": "pagerduty"
    opsgenie:
        api_key: "path/to/key.txt" # Path to a file containing the Opsgenie API key
```
This example would only match plugin resources with the labels 'type:opsgenie', or 'type:slack'.

The Opsgenie plugin (and any others created) can then be configured using resources.
Example plugin.yaml for Opsgenie that would match with this.
```
kind: plugin
metadata:
  name: opsgenie-plugin
  labels:
    type: opsgenie
spec:
  opsgenie:
    addr: "example.app.opsgenie.com" # Address of Opsgenie
    priority: "2" # Priority to create Opsgenie alerts with
    alert_tags: ["example-tag"] # List of tags to be added to alerts created in Opsgenie
    default_schedules: ["schedule1"] # Default on-call schedules to check if none are provided in the access request annotations
```

Given the above example teleport.yaml the following plugin resource will not match.
```
kind: plugin
metadata:
  name: slack-plugin
  labels:
    type: slack
spec:
  slack:
    addr: "example.slack.com"
```

The logging configuration will be shared with the main Teleport process.

### Getting an Opsgenie API key

In the Opsgenie web UI go to Settings -> App settings -> API key management. Create key with Read, Create and Update access.

### Executing

The plugin will be started using a command of the form given the appropriate 'plugin_service' is enabled in the config.

```
teleport start --config /etc/teleport.yaml
```

## UX


### Opsgenie plugin
Once an access request has been created, the Opsgenie plugin will create an alert in the service specified in the request annotation opsgenie_notify_services using the Opsgenie Alert API [Create](https://docs.opsgenie.com/docs/alert-api#create-alert) endpoint.

The appropriate on-call responder can then click the provided link to the access request and approve or deny it.

For auto approval of certain access requests the access request will be auto approved if the requesting user is on-call in one of the schedules provided in access request annotations.

Once an access request has been approved or denied the plugin will add a note to the alert and close the relevant alert tied to that access request.

Example role with services in the annotation that indicate that users with this roll can be on-call for those services.

```
kind: role
metadata:
  name: someRole
spec:
  allow:
    request:
      roles: [someOtherRole]
      annotations:
        teleport.dev/notify-services: ["schedule1", "schedule2"] # These are the Opsgenie schedules alerts will be created under
        teleport.dev/schedules: ["schedule1", "schedule2"] # These are the Opsgenie schedules checked during auto approval
```

## Implementation details

### Plugin service
The 'plugin_service' when enabled will watch for 'plugin' resources matching the labels specified in teleport.yaml.
When a plugin resource matching these labels is created the appropriate plugin will be started.
In the case of the Opsgenie plugin, the plugin startup will fail in the event the API key file field was not provided in the plugin_service configuration.

### Opsgenie plugin
In this section we will take a look at how the plugin will interact with the Opsgenie API.

### Authorization

The plugin will use the API key provided in the teleport.yaml config file when interacting with the Opsgenie API. This API key will be included in the headers of the requests made.

```
Authorization: GenieKey $apiKey
```

### Creating alerts
When the Opsgenie plugin creates alerts for incoming access requests the [Create](https://docs.opsgenie.com/docs/alert-api#create-alert) alert request will be of the form

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

On every review of the access request the alert created in Opsgenie will have a note added to it using the ‘[Add note to alert](https://docs.opsgenie.com/docs/alert-api#add-note-to-alert)’ endpoint. Then the alert will be closed using the ‘[Close alert](https://docs.opsgenie.com/docs/alert-api#close-alert)’ endpoint once the alert has either been approved or denied.

```
<Reviewer> reviewed the request at <someTime>.
Resolution: <StateEmojiAsUsedByExistingPLugins> <State>.
Reason: <Reason>
```

### Auto approval

To check if the requesting user of a request is currently on-call the ‘Who is on call API’s ‘[Get on calls](https://docs.opsgenie.com/docs/who-is-on-call-api#get-on-calls)’ endpoint will be used. 'https://<configured-opsgenie-address>/v2/schedules/<ScheduleName>/on-calls?scheduleIdentifierType=name'

Similar to the existing Pagerduty plugin for auto-approval to work, the user creating an Access Request must have a Teleport username that is also the email address associated with an Opsgenie account.

Access requests will be mapped to Opsgenie alerts by including the Access request ID in the tags field of the note.

### IaC

The Helm charts will be updated to support the new plugin-service.

## Security considerations

Potential for users to get access requests auto approved if they can get themselves onto the current on call rotation.
Since Teleport usernames are assumed to match the Opsgenie email address when checking on call there is potential for access requests to be auto approved unintentionally.
