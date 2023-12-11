---
authors: Sakshyam Shah (sshah@goteleport.com)
state: draft
---

# RFD 18E - SAML IdP User Attribute Mapping

## Required approvers

- Engineering: (@r0mant && @mdwn)
- Security: (@reed || @jentfoo)
- Product: (@xinding33 || @klizhentas)

## What

Mapping user traits to Service Provider requested attribute names using predicate expressions.

For example:

- Relay user groups retrieved from an upstream Identity Provider (like Okta) to downstream Service Provider.
- Assert user’s first name (`user.spec.traits.firstname`) as an outgoing SAML attribute `givenname` or `given_name`.
- Filtering out “prod-db” role in SAML assertion using predicate expression `user.spec.roles.remove("prod-db")`.

## Why

Currently, the only attributes that are always asserted by Teleport IdP are `uid` - Teleport username and `eduPersonAffiliate` - Teleport user roles. Additionally, the following attributes are also asserted if they are explicitly requested in the SSO request:

https://github.com/gravitational/teleport.e/blob/11900c2d95e7c9e7527df32ccf933fec78bcb119/lib/idp/saml/assertion.go#L99-L117

Although these attributes represent “well known” identifiers, they are not guaranteed to be supported by Service Providers. The SAML specification does not make these attributes mandatory either. Service Providers can request attributes as required by their application.

Further, besides general attribute mapping, administrators may want to filter and transform user attributes before asserting to the Service Provider.

Ultimately, with custom attribute mapping, we hope to make it easier for Teleport administrators to enroll SAML supported applications to Teleport IdP, increasing its adoption.

## Details

The scope of the work can be broken down into two categories:

1. Attribute mapping configuration.
2. Evaluating mapped attributes.

For readers new to Teleport SAML IdP, please refer to [RFD 4E - Teleport as a SAML IdP](https://github.com/gravitational/teleport.e/blob/master/rfd/0004e-teleport-idp.md).

## 1. Attribute Mapping Configuration

### Defining attributes

A new field named `AttributeMapping` of type `SAMLAttributeMapping` will be added to [`SAMLIdPServiceProviderSpecV1`](https://github.com/gravitational/teleport/blob/464edfa2cdc7acc4e8d2b711fef3e9740e21d4d5/api/proto/teleport/legacy/types/types.proto#L5750).

```diff
// SAMLIdPServiceProviderSpecV1 is the SAMLIdPServiceProviderV1 resource spec.
message SAMLIdPServiceProviderSpecV1 {
 // EntityDescriptor is the entity descriptor for the service provider
 string EntityDescriptor = 1 [(gogoproto.jsontag) = "entity_descriptor"];
 string EntityID = 2 [(gogoproto.jsontag) = "entity_id"];
+ repeated SAMLIdPAttributeMapping AttributeMapping = 3 [(gogoproto.jsontag) = "attribute_mapping"];
}

+ message SAMLIdPAttributeMapping {
+  string Name = 1 [(gogoproto.jsontag) = "name"];
+  string NameFormat = 2 [(gogoproto.jsontag) = "name_format"];
+  string Value = 3 [(gogoproto.jsontag) = "value"];
+ }
```

`SAMLIdPAttributeMapping` defines three fields:

- `name`: Name of the outgoing attribute. Required. Name should be unique across attribute mapping.
- `value`: Value defined using predicate expression, which can be mapped to Teleport user name, roles or traits. Required.
- `name_format`: SAML attribute name format. Optional. `"urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified"` will be used as default.

Attribute mapping will be configurable via the YAML resource representing a SAML service provider as shown below:

```yml
kind: saml_idp_service_provider
version: v1
metadata:
  name: internalapp
spec:
  entity_id: https://internalapp/saml/metadata
  sso_url: https://internalapp/saml/acs
  attribute_mapping:
    - name: displayname
      name_format: uri
      value: user.spec.traits.displayname
    - name: firstName
      value: user.spec.traits.firstname
    - name: lastName
      value: user.spec.traits.lastname
    - name: groups
      # include both roles and groups
      value: union(user.spec.roles, user.spec.traits.groups)
```

Attribute mapping will be available in the Web UI as well. Web UI will also feature predefined predicate expressions in a drop down menu for common user traits to help ease configuration.

### Storing attributes

While the attribute mapping details are stored in the backend as part of the [`SAMLIdPServiceProviderSpecV1`](https://github.com/gravitational/teleport/blob/464edfa2cdc7acc4e8d2b711fef3e9740e21d4d5/api/proto/teleport/legacy/types/types.proto#L5750) spec, we will also embed these values into the Service Provider metadata (Entity Descriptor) as `RequestedAttribute` element.

For example, the following attribute statement:

```yml
- name: groups
  name_format: unspecified
  value: user.spec.traits.groups
```

Will be converted to SAML object:

```
saml.RequestedAttribute{
		Attribute: saml.Attribute{
			FriendlyName: groups,
			Name:         groups,
			NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified",
			Values:       []saml.AttributeValue{{Value: user.spec.traits.groups}},
	},
}
```

Which will be marshalled and added as a new XML element to Service Provider metadata:

```xml
<SPSSODescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" protocolSupportEnumeration="">
             <AttributeConsumingService index="0">
                 <RequestedAttribute FriendlyName="groups" Name="groups" NameFormat="urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified">
                     <AttributeValue xmlns:_XMLSchema-instance="http://www.w3.org/2001/XMLSchema-instance" _XMLSchema-instance:type="">user.spec.traits.groups</AttributeValue>
                 </RequestedAttribute>
             </AttributeConsumingService>
</SPSSODescriptor>
```

[`Element <RequestedAttribute>`](http://docs.oasis-open.org/security/saml/v2.0/saml-metadata-2.0-os.pdf) (§2.4.4.2) is part of the Service Provider metadata specification.

By embedding attribute into metadata, these values will be retrieved using existing `GetServiceProvider` method, with no additional changes to IdP or an extra trip to the backend.

```
// ServiceProviderProvider is an interface used by IdentityProvider to look up
// service provider metadata for a request.
type ServiceProviderProvider interface {
   GetServiceProvider(r *http.Request, serviceProviderID string) (*EntityDescriptor, error)
}
```

Embedding will happen once during SAML Service Provider create or update flow.

#### Supported `name_format`

User can configure one of the following three name formats (both name and full `urn` value is supported):

- `unspecified`: value equals to `urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified`. Used as a default value.
- `uri`: value equals to `urn:oasis:names:tc:SAML:2.0:attrname-format:uri`.
- `basic`: value equals to `urn:oasis:names:tc:SAML:2.0:attrname-format:basic`.

### Testing attribute mapping

A utility test command will be introduced to verify attribute mapping.

```
$ tctl idp saml test_attribute_mapping \
  --user (user spec file or username)
  --sp sp.yaml
{"uid": "user@example.com", "displayName": "test user", "groups": ["admin", "staging"]}
```

## 2. Attribute evaluation

Attribute values are authored using Predicate expression. During SSO request, the expressions will be parsed from the SP metadata and passed to predicate parser for evaluation. The resulting values are asserted in the SAML response as per the requested attribute name.

#### Evaluation context

The following user attributes will be made available for mapping between Teleport IdP and Service Providers:

| Attributes | Description                                                      | Syntax                                                       |
| ---------- | ---------------------------------------------------------------- | ------------------------------------------------------------ |
| Username   | User name.                                                       | `uid` or `user.metadata.name`.                               |
| Roles      | User roles.                                                      | `eduPersonAffiliate` or `user.spec.roles`.                   |
| Traits     | All values under user traits will be made available for mapping. | `user.spec.traits.firstname`, `user.spec.traits.groups` etc. |

#### Helper functions and methods

All the helper functions and methods supported by [Login Rules](https://github.com/gravitational/teleport/blob/master/rfd/0078-login-rules.md#predicate-helper-functions) for `traits_map` will be available for attribute mapping.

### Non-existent attribute value

Given a correct and supported predicate expression, attributes will be mapped as long as the requested attributes are present in Teleport.

Attribute with non-existent value will not be included in SAML assertion. It is the current behavior and will remain unchanged.

## Backward compatibility

Introducing attribute mapping will not disrupt existing SAML IdP and SP configurations. Existing user attributes `uid` and `eduPersonAffiliate`, as well as the supported attributes that are asserted when requested through SSO request will remain unchanged to ensure existing access configuration does not break.

## Audit events

Attribute mapping will add mapped attribute name and values to Service Provider create and udpate events.

```diff
// SAMLIdPServiceProviderMetadata contains common metadata for SAML IdP service provider
// events.
message SAMLIdPServiceProviderMetadata {
  // ServiceProviderEntityID is the entity ID of the service provider.
  string ServiceProviderEntityID = 1 [(gogoproto.jsontag) = "service_provider_entity_id,omitempty"];
  // ServiceProviderShortcut is the shortcut name of a service provider.
  string ServiceProviderShortcut = 2 [(gogoproto.jsontag) = "service_provider_shortcut,omitempty"];
+  map<string, string> AttributeMapping = 3 [(gogoproto.jsontag) = "attribute_mapping,omitempty"];
}
```

## Examples

1. Transform firstname to lower.

```
attribute_mapping:
  - name: firstname
    value: strings.lower(user.spec.traits.firstname)
```

2. Merge groups and roles to attribute name "groupnames".

```
attribute_mapping:
  - name: groupnames
    value: union(user.spec.traits.groups, user.spec.roles)
```

3. Include `user.traits.roles` if one of the role is "dev". Else, return `user.spec.traits.groups`.

```
attribute_mapping:
  - name: groupnames
    value: ifelse(user.spec.traits.roles("dev"), user.spec.roles, user.spec.traits.groups)
```

4. Include all groups except "prod-ssh".

```
attribute_mapping:
  - name: groups
    value: user.spec.traits.groups.remove("prod-ssh")
```
