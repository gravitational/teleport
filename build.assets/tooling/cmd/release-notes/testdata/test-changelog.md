# Changelog

## 16.0.1 (06/17/24)

* `tctl` now ignores any configuration file if the auth_service section is disabled, and prefer loading credentials from a given identity file or tsh profile instead. [#43115](https://github.com/gravitational/teleport/pull/43115)
* Skip `jamf_service` validation when the service is not enabled. [#43095](https://github.com/gravitational/teleport/pull/43095)
* Fix v16.0.0 amd64 Teleport plugin images using arm64 binaries. [#43084](https://github.com/gravitational/teleport/pull/43084)
* Add ability to edit user traits from the Web UI. [#43067](https://github.com/gravitational/teleport/pull/43067)
* Enforce limits when reading events from Firestore for large time windows to prevent OOM events. [#42966](https://github.com/gravitational/teleport/pull/42966)
* Allow all authenticated users to read the cluster `vnet_config`. [#42957](https://github.com/gravitational/teleport/pull/42957)
* Improve search and predicate/label based dialing performance in large clusters under very high load. [#42943](https://github.com/gravitational/teleport/pull/42943)

## 16.0.0 (06/13/24)

Teleport 16 brings the following new features and improvements:

- Teleport VNet
- Device Trust for the Web UI
- Increased support for per-session MFA
- Web UI notification system
- Access requests from the resources view
- `tctl` for Windows
- Teleport plugins improvements
