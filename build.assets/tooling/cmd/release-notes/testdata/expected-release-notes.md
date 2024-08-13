## Description

* `tctl` now ignores any configuration file if the auth_service section is disabled, and prefer loading credentials from a given identity file or tsh profile instead. [#43115](https://github.com/gravitational/teleport/pull/43115)
* Skip `jamf_service` validation when the service is not enabled. [#43095](https://github.com/gravitational/teleport/pull/43095)
* Fix v16.0.0 amd64 Teleport plugin images using arm64 binaries. [#43084](https://github.com/gravitational/teleport/pull/43084)
* Add ability to edit user traits from the Web UI. [#43067](https://github.com/gravitational/teleport/pull/43067)
* Enforce limits when reading events from Firestore for large time windows to prevent OOM events. [#42966](https://github.com/gravitational/teleport/pull/42966)
* Allow all authenticated users to read the cluster `vnet_config`. [#42957](https://github.com/gravitational/teleport/pull/42957)
* Improve search and predicate/label based dialing performance in large clusters under very high load. [#42943](https://github.com/gravitational/teleport/pull/42943)

## Download

Download the current and previous releases of Teleport at https://goteleport.com/download.

## Plugins

Download the current release of Teleport plugins from the links below.
* Slack ([Linux amd64](https://cdn.teleport.dev/teleport-access-slack-v16.0.1-linux-amd64-bin.tar.gz))
* Mattermost ([Linux amd64](https://cdn.teleport.dev/teleport-access-mattermost-v16.0.1-linux-amd64-bin.tar.gz))
* Discord ([Linux amd64](https://cdn.teleport.dev/teleport-access-discord-v16.0.1-linux-amd64-bin.tar.gz))
* Terraform Provider ([Linux amd64](https://cdn.teleport.dev/terraform-provider-teleport-v16.0.1-linux-amd64-bin.tar.gz) | [Linux arm64](https://cdn.teleport.dev/terraform-provider-teleport-v16.0.1-linux-arm64-bin.tar.gz) | [macOS amd64](https://cdn.teleport.dev/terraform-provider-teleport-v16.0.1-darwin-amd64-bin.tar.gz) | [macOS arm64](https://cdn.teleport.dev/terraform-provider-teleport-v16.0.1-darwin-arm64-bin.tar.gz) | [macOS universal](https://cdn.teleport.dev/terraform-provider-teleport-v16.0.1-darwin-universal-bin.tar.gz))
* Event Handler ([Linux amd64](https://cdn.teleport.dev/teleport-event-handler-v16.0.1-linux-amd64-bin.tar.gz) | [macOS amd64](https://cdn.teleport.dev/teleport-event-handler-v16.0.1-darwin-amd64-bin.tar.gz))
* PagerDuty ([Linux amd64](https://cdn.teleport.dev/teleport-access-pagerduty-v16.0.1-linux-amd64-bin.tar.gz))
* Jira ([Linux amd64](https://cdn.teleport.dev/teleport-access-jira-v16.0.1-linux-amd64-bin.tar.gz))
* Email ([Linux amd64](https://cdn.teleport.dev/teleport-access-email-v16.0.1-linux-amd64-bin.tar.gz))
* Microsoft Teams ([Linux amd64](https://cdn.teleport.dev/teleport-access-msteams-v16.0.1-linux-amd64-bin.tar.gz))
