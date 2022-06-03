|Event Type|Description|
|---|---|
|**access_request.create**|`access_request.create` is emitted when a new Access Request is created. |
|**access_request.delete**|`access_request.delete` is emitted when a new Access Request is deleted. |
|**access_request.review**|`access_request.review` is emitted when a review is applied to an Access Request. |
|**access_request.search**|`access_request.search` is emitted when a user searches for resources as part of a search-based Access Request. |
|**access_request.update**|`access_request.update` is emitted when an Access Request's state is updated. |
|**app.create**|`app.create` is emitted when an application resource is created. |
|**app.delete**|`app.delete` is emitted when an application resource is deleted. |
|**app.session.chunk**|`app.session.chunk` is emitted at the start of a 5 minute chunk on each Application Service instance. This chunk is used to buffer 5 minutes of audit events at a time for applications. |
|**app.session.end**|`app.session.end` is emitted when a user connects to a TCP application. |
|**app.session.request**|`app.session.request` is an HTTP request and response. |
|**app.session.start**|`app.session.start` is emitted when a user is issued an application certificate. |
|**app.update**|`app.update` is emitted when an application resource is updated. |
|**auth**|`auth` is an authentication attempt that either succeeded or failed based on the event's status. |
|**cert.create**|`cert.create` is emitted when a certificate is issued. |
|**cert.generation_mismatch**|`cert.generation_mismatch` is emitted when a renewable certificate's generation counter is invalid. |
|**client.disconnect**|`client.disconnect` is emitted when client is disconnected by the server due to inactivity or any other reason. |
|**db.create**|`db.create` is emitted when a database resource is created. |
|**db.delete**|`db.delete` is emitted when a database resource is deleted. |
|**db.session.query**|`db.session.query` is emitted when a database client executes a query. |
|**db.session.sqlserver.rpc_request**|`db.session.sqlserver.rpc_request` is emitted when a SQL Server client sends an RPC request command. |
|**db.update**|`db.update` is emitted when a database resource is updated. |
|**desktop.clipboard.receive**|`desktop.clipboard.receive` is emitted when Teleport receives clipboard data from a remote desktop. |
|**desktop.clipboard.send**|`desktop.clipboard.send` is emitted when local clipboard data is sent to Teleport. |
|**desktop.recording**|`desktop.recording` is emitted as a Desktop Access session is recorded. |
|**exec**|`exec` is an exec command executed by script or user on the server side. |
|**github.created**|`github.created` fires when a GitHub connector is created/updated. |
|**github.deleted**|`github.deleted` fires when a GitHub connector is deleted. |
|**kube.create**|`kube.create` is emitted when a Kubernetes cluster resource is created. |
|**kube.delete**|`kube.delete` is emitted when a Kubernetes cluster resource is deleted. |
|**kube.request**|`kube.request` fires when a Kubernetes Service instance handles a generic Kubernetes request. |
|**kube.update**|`kube.update` is emitted when a Kubernetes cluster resource is updated. |
|**lock.created**|`lock.created` fires when a lock is created/updated. |
|**lock.deleted**|`lock.deleted` fires when a lock is deleted. |
|**mfa.add**|`mfa.add` is an event type for users adding MFA devices. |
|**mfa.delete**|`mfa.delete` is an event type for users deleting MFA devices. |
|**oidc.created**|`oidc.created` fires when an OIDC connector is created/updated. |
|**oidc.deleted**|`oidc.deleted` fires when an OIDC connector is deleted. |
|**port**|Port forwarding event. |
|**privilege_token.create**|`privilege_token.create` is emitted when a new user privilege token is created. |
|**recovery_code.generated**|`recovery_code.generated` is an event type for generating a user's recovery tokens. |
|**recovery_code.used**|`recovery_code.used` is an event type when a recovery token was used. |
|**recovery_token.create**|`recovery_token.create` is emitted when a new recovery token is created. |
|**reset_password_token.create**|`reset_password_token.create` is emitted when a new reset password token is created. |
|**resize**|`resize` means that some user resized the PTY on their client. |
|**role.created**|`role.created` fires when a role is created/updated. |
|**role.deleted**|`role.deleted` fires when a role is deleted. |
|**saml.created**|`saml.created` fires when a SAML connector is created/updated. |
|**saml.deleted**|`saml.deleted` fires when a SAML connector is deleted. |
|**scp**|`scp` means that a data transfer occurred on the server. |
|**session.command**|`session.command` is emitted when an executable is run within a session. |
|**session.connect**|`session.connect` is emitted when any SSH connection is made. |
|**session.data**|`session.data` reports how much data was transmitted and received over a connection to a Teleport SSH or Kubernetes Service instance. Reported when the connection is closed. |
|**session.disk**|`session.disk` is emitted when a file is opened within an session. |
|**session.end**|`session.end` indicates that a session has ended. |
|**session.join**|`session.join` indicates that someone joined a session. |
|**session.leave**|`session.leave` indicates that someone left a session. |
|**session.network**|`session.network` is emitted when a network connection is initiated with a session. |
|**session.recording.access**|`session.recording.access` is emitted when a session recording is accessed. |
|**session.rejected**|`session.rejected` fires when a user's attempt to create an authenticated session has been rejected due to exceeding a session control limit. |
|**session.start**|`session.start` indicates that session has been initiated or updated by a joining party on the server |
|**session.upload**|`session.upload` indicates that session has been uploaded to the external storage backend. |
|**sftp**|`sftp` means that a user attempted a file operation. |
|**ssm.run**|`ssm.run` is emitted when a run of an install script completes on a discovered EC2 instance. |
|**subsystem**|`subsystem` is the result of the execution of a subsystem. |
|**trusted_cluster.create**|`trusted_cluster.create` is the event for creating a Trusted Cluster. |
|**trusted_cluster.delete**|`trusted_cluster.delete` is the event for removing a Trusted Cluster. |
|**trusted_cluster_token.create**|`trusted_cluster_token.create` is the event for creating new join token for a Trusted Cluster. |
|**user.create**|`user.create` is emitted when a user is created. |
|**user.delete**|`user.delete` is emitted when a user is deleted. |
|**user.login**|`user.login` indicates that a user logged in via the Web UI or `tsh`. |
|**user.password_change**|`user.password_change` is when the user changes their own password. |
|**user.update**|`user.update` is emitted when the user is updated. |
|**windows.desktop.session.end**|`windows.desktop.session.end` is emitted when a user disconnects from a desktop. |
|**windows.desktop.session.start**|`windows.desktop.session.start` is emitted when a user attempts to connect to a desktop. |
|**x11-forward**|X11 forwarding event. |
