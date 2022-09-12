---
authors: Krzysztof Skrzętnicki <krzysztof.skrzetnicki@goteleport.com>
state: implemented
---

# RFD 71 - `tctl sso test` command

## What

This RFD proposes new subcommand for the `tctl` tool: `sso test`. The purpose of this command is to perform validation
of auth connector in SSO flow prior to creating the connector resource with `tctl create ...` command.

To accomplish that the definition of auth connector is read from file and attached to the auth request being made. The
Teleport server uses the attached definition to proceed with the flow, instead of using any of the stored auth connector
definitions.

The login flow proceeds as usual, with some exceptions:

- Embedded definition of auth connector will be used whenever applicable.
- The client key is not signed at the end of successful flow, so no actual login will be performed.
- During the flow the diagnostic information is captured and stored in the backend, where it can be retrieved by using
  the auth request ID as the key.

The following kinds of auth connectors will be supported:

- SAML (Enterprise only)
- OIDC (Enterprise only)
- Github

## Why

Currently, Teleport offers no mechanism for testing the SSO flows prior to creating the connector, at which point the
connector is immediately accessible for everyone. Having a dedicated testing flow using single console terminal for
initiating the test and seeing the results would improve the usability and speed at which the changes to connectors can
be iterated. Decreased SSO configuration time contributes to improved "time-to-first-value" metric of Teleport.

The testing capabilities would be especially useful for Teleport Cloud, as currently the administrator is running a risk
of locking themselves out of Teleport cluster.

## Details

### UX

The user initiates the flow by issuing command such as `tctl --proxy=<proxy.addr> sso test <auth_connector.yaml>`. The
resource is loaded, and it's kind is determined (SAML/OIDC/GitHub). If the connector kind is supported, the
browser-based SSO flow is initiated.

Once the flow is finished, either successfully or not, the tool notifies the user of this fact. This is a change from
current behaviour of `tsh login`, where only successful flow are terminated in non-timeout manner. After the flow is
finished, the user is provided with a wealth of diagnostic information. Depending on the scenario a different levels of
verbosity are applied; the highest level is available by using the `--debug` flag.

The flow is carried out mostly via web browser,

In the same manner as `tsh login --auth=<sso_auth>` opens the browser to perform the login, the user is redirected to
the browser as well. Once the flow is finished in any way, the user is notified of that fact along with any debugging
information that has been passed by the server (e.g. claims, mapped roles, ...).

### Example runs

- Successful test:

```
$ tctl sso test connector-good.yaml

If browser window does not open automatically, open it by clicking on the link:
 http://127.0.0.1:65228/ef343c31-cc9f-4105-a2c7-c490463f1b96
SSO flow succeeded! Logged in as: example@gravitational.io
--------------------------------------------------------------------------------
Authentication details:
- username: example@gravitational.io
- roles: [editor auditor access]
- traits: map[groups:[Everyone okta-admin okta-dev] username:[example@gravitational.io]]

--------------------------------------------------------------------------------
[SAML] Attributes to roles:
key: SAML.attributesToRoles
value:
    - name: groups
      roles:
        - editor
        - auditor
      value: okta-admin
    - name: groups
      roles:
        - access
      value: okta-dev

--------------------------------------------------------------------------------
[SAML] Attributes statements:
key: SAML.attributeStatements
value:
    groups:
        - Everyone
        - okta-admin
        - okta-dev
    username:
        - example@gravitational.io

--------------------------------------------------------------------------------
For more details repeat the command with --debug flag.
```

- Mapping to a role which does not exist:

```
> tctl sso test connector-bad-mapping.yaml

If browser window does not open automatically, open it by clicking on the link:
 http://127.0.0.1:65239/9da0c11c-824f-473a-8c3f-16c4543e7845
SSO flow failed! Login error: sso flow failed, error: role XXX-DOES_NOT_EXIST is not found
--------------------------------------------------------------------------------
Error details: Failed to calculate user attributes.

Details: [role XXX-DOES_NOT_EXIST is not found]
--------------------------------------------------------------------------------
[SAML] Attributes to roles:
key: SAML.attributesToRoles
value:
    - name: groups
      roles:
        - access
        - XXX-DOES_NOT_EXIST
      value: okta-admin

--------------------------------------------------------------------------------
[SAML] Attributes statements:
key: SAML.attributeStatements
value:
    groups:
        - Everyone
        - okta-admin
        - okta-dev
    username:
        - example@gravitational.io

--------------------------------------------------------------------------------
For more details repeat the command with --debug flag.
```

- Bad connector: 

```
> tctl sso test connector-no-roles.yaml

ERROR: Unable to load SAML connector. Correct the definition and try again. Details: attributes_to_roles is empty, authorization with connector would never give any roles.
```

```
> tctl sso test connector-malformed-entity-desc.yaml

ERROR: Unable to load SAML connector. Correct the definition and try again. Details: no SSO set either explicitly or via entity_descriptor spec.
```

```
> tctl sso test connector-missing-entity-desc.yaml
------------------------------------------------------------------------------

ERROR: Unable to load SAML connector. Correct the definition and try again. Details: no entity_descriptor set, either provide entity_descriptor or entity_descriptor_url in spec.
```

- Bad ACS

Note: this error is less likely to occur if `tctl sso configure` command is used, as it sets the ACS automatically.

```
> tctl sso test connector-bad-acs.yaml

If browser window does not open automatically, open it by clicking on the link:
 http://127.0.0.1:65288/6c5fd054-9055-4f53-a8fe-c5d49f64f77e
SSO flow failed! Login error: sso flow failed, error: received response with incorrect or missing attribute statements, please check the identity provider configuration to make sure that mappings for claims/attribute statements are set up correctly. <See: https://goteleport.com/teleport/docs/enterprise/sso/ssh-sso/>, failed to retrieve SAML assertion info from response: error validating response: Unrecognized Destination value, Expected: https://teleport.example.com:8080/v1/webapi/saml/acs, Actual: https://teleport.example.com:3080/v1/webapi/saml/acs.
--------------------------------------------------------------------------------
Error details: Failed to retrieve assertion info. This may indicate IdP configuration error.

Details: [received response with incorrect or missing attribute statements, please check the identity provider configuration to make sure that mappings for claims/attribute statements are set up correctly. <See: https://goteleport.com/teleport/docs/enterprise/sso/ssh-sso/>, failed to retrieve SAML assertion info from response: error validating response: Unrecognized Destination value, Expected: https://teleport.example.com:8080/v1/webapi/saml/acs, Actual: https://teleport.example.com:3080/v1/webapi/saml/acs.]
--------------------------------------------------------------------------------
For more details repeat the command with --debug flag.
```

- Missing IdP certificate

```
> tctl sso test connector-no-cert.yaml

ERROR: Failed to create auth request. Check the auth connector definition for errors. Error: no identity provider certificate provided, either set certificate as a parameter or via entity_descriptor
```

- Incorrect group mapping on IdP side results in no attributes being passed in a claim. 

Note: This is an error on IdP configuration side which we cannot see directly. 

```
> tctl sso test connector-bad-idp-group-config.yaml

If browser window does not open automatically, open it by clicking on the link:
 http://127.0.0.1:65325/d87df69d-d5d6-427a-bb9e-056ee20b4b90
SSO flow failed! Login error: sso flow failed, error: received response with incorrect or missing attribute statements, please check the identity provider configuration to make sure that mappings for claims/attribute statements are set up correctly. <See: https://goteleport.com/teleport/docs/enterprise/sso/ssh-sso/>, failed to retrieve SAML assertion info from response: missing AttributeStatement element.
--------------------------------------------------------------------------------
Error details: Failed to retrieve assertion info. This may indicate IdP configuration error.

Details: [received response with incorrect or missing attribute statements, please check the identity provider configuration to make sure that mappings for claims/attribute statements are set up correctly. <See: https://goteleport.com/teleport/docs/enterprise/sso/ssh-sso/>, failed to retrieve SAML assertion info from response: missing AttributeStatement element.]
--------------------------------------------------------------------------------
For more details repeat the command with --debug flag.
```

- The IdP is expecting a different certificate than the one we have in auth connector spec.


Note: This is an error on IdP configuration side which we cannot see directly.

```
> tctl sso test connector-bad-idp-cert-config.yaml
------------------------------------------------------------------------------

If browser window does not open automatically, open it by clicking on the link:
http://127.0.0.1:65333/3c9ac37a-1fb5-4160-8740-e04e4b18ea46
SSO flow failed! Login error: sso flow failed, error: received response with incorrect or missing attribute statements, please check the identity provider configuration to make sure that mappings for claims/attribute statements are set up correctly. <See: https://goteleport.com/teleport/docs/enterprise/sso/ssh-sso/>, failed to retrieve SAML assertion info from response: error validating response: unable to decrypt encrypted assertion: cannot decrypt, error retrieving private key: key decryption attempted with mismatched cert, SP cert(11:35:70:c1), assertion cert(f1:e3:25:c6).
--------------------------------------------------------------------------------
Error details: Failed to retrieve assertion info. This may indicate IdP configuration error.

Details: [received response with incorrect or missing attribute statements, please check the identity provider configuration to make sure that mappings for claims/attribute statements are set up correctly. <See: https://goteleport.com/teleport/docs/enterprise/sso/ssh-sso/>, failed to retrieve SAML assertion info from response: error validating response: unable to decrypt encrypted assertion: cannot decrypt, error retrieving private key: key decryption attempted with mismatched cert, SP cert(11:35:70:c1), assertion cert(f1:e3:25:c6).]
--------------------------------------------------------------------------------
For more details repeat the command with --debug flag.
```

### Passing details to SSO callback on failed login

Currently, in case of SSO error, the console client is not informed of failure. Unless interrupted (Ctrl-C/SIGINT) the
request will simply time out after few minutes:

```
If browser window does not open automatically, open it by clicking on the link:
http://127.0.0.1:59544/452e0ecc-797d-488e-a9be-ffc23b21fcf8
DEBU [CLIENT]    Timed out waiting for callback after 3m0s. client/weblogin.go:324
```

The user can tell the SSO flow has failed because they should see an error page in the browser. They still have to
manually interrupt the `tsh login` command, which is suboptimal and may confuse users.

We want to change this behaviour for **ALL** SSO flows (both testing ones and normal ones) so that client callback is
always called. In case of failure the callback will omit the login information, but we may include the error
information. The updated `tsh` clients will correctly receive that error and will be able to display it to the user. The
old `tsh` clients will also terminate the flow, but will lack the ability to display a detailed error message.

### Implementation details

There are several conceptual pieces to the implementation:

1. Extending auth requests with embedded connector details. The auth requests (`SAMLAuthRequest`,`OIDCAuthRequest`
   ,`GithubAuthRequest`) will gain two new fields: boolean `TestFlow` flag and optional auth connector spec.
2. Creating the extended auth requests. Currently, the auth requests are created by calling unauthenticated endpoints (
   one per auth kind): `/webapi/{oidc,saml,github}/login/console`. These endpoints will *not*
   change. For security reasons, we don't want unauthenticated users to initiate SSO test flows. Instead, the requests
   will be created using authenticated API call such as `CreateSAMLAuthRequest`.
3. Making the backend aware of the testing flow. Right now there is no concept of "dry run" login flow. All logins are
   deemed "real" and assumed to be made against existing connectors. We want to avoid issuing actual certificates,
   adjust the audit log events and make use of embedded connector spec.
4. Adding the API for collecting the SSO diagnostic info, keyed by particular auth request IDs. This is functionality
   potentially useful outside of test flows, e.g. by extending the web ui with diagnostic panel for recent SSO logins.
   This API will be queried for information after the test flow terminates.
5. Calling the `ClientRedirectURL` in the case of SSO flow failure, but with an error message.

The implementation of this RFD for different kinds of connectors should be largely independent. As such, the first
iteration will implement this functionality for SAML, while the lessons learned will help shape the implementations for
OIDC and GitHub.

### Security

We consider the following potential security problems arising from the implementation of this feature.

| Problem                                                 | Mitigation                                                                                                                                                                                                                                                                     |
|---------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| 1. Unauthorized user initiating the test flow           | Creating auth request with TestFlow flag will require elevated permissions, equivalent to those required to create the auth connector directly.                                                                                                                                |
| 2. Malicious definition of auth connector.              | The admin must be able to create real connectors already. The test ones are strictly less powerful.                                                                                                                                                                            |
| 3. Auth connector secrets leak via logs.                | Careful review of logging to ensure we don't log auth connector spec directly or when embedded in auth request.                                                                                                                                                                |
| 4. Auth connector secrets leak via access request.      | Auth connector secrets are embedded in auth request. Any user capable of accessing this auth request will be able to read those secrets. We already secure the access to auth requests with RBAC. Additionally, the auth requests are short-lived and their ID is random UUID. |
| 5. Other info leak in the logs.                         | Logging code review to ensure sensitive information is not logged.                                                                                                                                                                                                             |
| 6. Logic flaw resulting in test flow issuing real auth. | Code review to ensure the logic is not flawed.                                                                                                                                                                                                                                 |
