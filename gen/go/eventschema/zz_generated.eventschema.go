/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package eventschema

// Code generated from protobuf, DO NOT EDIT.
// To re-generate the file, go into build.assets/ and run "make generate-eventschema".

type Event struct {
	Description string
	Fields      []*EventField
}

type EventField struct {
	Name        string
	Type        string
	Description string
	Fields      []*EventField
	Items       *EventField
}

// Events is a map containing the description and schema for all Teleport events
var events = map[string]*Event{
	"AccessListCreate": {
		Description: "is emitted when an access list is created",
		Fields: []*EventField{
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "error",
				Description: "includes system error message for the failed attempt",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "message",
				Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "success",
				Description: "indicates the success or failure of the operation",
				Type:        "boolean",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
		},
	},
	"AccessListDelete": {
		Description: "is emitted when an access list is deleted",
		Fields: []*EventField{
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "error",
				Description: "includes system error message for the failed attempt",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "message",
				Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "success",
				Description: "indicates the success or failure of the operation",
				Type:        "boolean",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
		},
	},
	"AccessListMemberCreate": {
		Description: "is emitted when an access list member is created",
		Fields: []*EventField{
			{
				Name:        "access_list_name",
				Description: "is the name of the access list the members are being added to or removed from",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "error",
				Description: "includes system error message for the failed attempt",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "members",
				Description: "are all members affected by the access list membership change",
				Type:        "array",
				Items: &EventField{
					Type: "object",
					Fields: []*EventField{
						{
							Name:        "joined_on",
							Description: "is the date that the member joined",
							Type:        "string",
						},
						{
							Name:        "member_name",
							Description: "is the name of the member",
							Type:        "string",
						},
						{
							Name:        "reason",
							Description: "is the reason that the member was added, modified, or removed",
							Type:        "string",
						},
						{
							Name:        "removed_on",
							Description: "is the date that the access list member was removed. Will only be populated for deletion",
							Type:        "string",
						},
					},
				},
			},
			{
				Name:        "message",
				Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "success",
				Description: "indicates the success or failure of the operation",
				Type:        "boolean",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
		},
	},
	"AccessListMemberDelete": {
		Description: "is emitted when an access list member is deleted",
		Fields: []*EventField{
			{
				Name:        "access_list_name",
				Description: "is the name of the access list the members are being added to or removed from",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "error",
				Description: "includes system error message for the failed attempt",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "members",
				Description: "are all members affected by the access list membership change",
				Type:        "array",
				Items: &EventField{
					Type: "object",
					Fields: []*EventField{
						{
							Name:        "joined_on",
							Description: "is the date that the member joined",
							Type:        "string",
						},
						{
							Name:        "member_name",
							Description: "is the name of the member",
							Type:        "string",
						},
						{
							Name:        "reason",
							Description: "is the reason that the member was added, modified, or removed",
							Type:        "string",
						},
						{
							Name:        "removed_on",
							Description: "is the date that the access list member was removed. Will only be populated for deletion",
							Type:        "string",
						},
					},
				},
			},
			{
				Name:        "message",
				Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "success",
				Description: "indicates the success or failure of the operation",
				Type:        "boolean",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
		},
	},
	"AccessListMemberDeleteAllForAccessList": {
		Description: "is emitted when all members are deleted for an access list",
		Fields: []*EventField{
			{
				Name:        "access_list_name",
				Description: "is the name of the access list the members are being added to or removed from",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "error",
				Description: "includes system error message for the failed attempt",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "members",
				Description: "are all members affected by the access list membership change",
				Type:        "array",
				Items: &EventField{
					Type: "object",
					Fields: []*EventField{
						{
							Name:        "joined_on",
							Description: "is the date that the member joined",
							Type:        "string",
						},
						{
							Name:        "member_name",
							Description: "is the name of the member",
							Type:        "string",
						},
						{
							Name:        "reason",
							Description: "is the reason that the member was added, modified, or removed",
							Type:        "string",
						},
						{
							Name:        "removed_on",
							Description: "is the date that the access list member was removed. Will only be populated for deletion",
							Type:        "string",
						},
					},
				},
			},
			{
				Name:        "message",
				Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "success",
				Description: "indicates the success or failure of the operation",
				Type:        "boolean",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
		},
	},
	"AccessListMemberUpdate": {
		Description: "is emitted when an access list member is updated",
		Fields: []*EventField{
			{
				Name:        "access_list_name",
				Description: "is the name of the access list the members are being added to or removed from",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "error",
				Description: "includes system error message for the failed attempt",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "members",
				Description: "are all members affected by the access list membership change",
				Type:        "array",
				Items: &EventField{
					Type: "object",
					Fields: []*EventField{
						{
							Name:        "joined_on",
							Description: "is the date that the member joined",
							Type:        "string",
						},
						{
							Name:        "member_name",
							Description: "is the name of the member",
							Type:        "string",
						},
						{
							Name:        "reason",
							Description: "is the reason that the member was added, modified, or removed",
							Type:        "string",
						},
						{
							Name:        "removed_on",
							Description: "is the date that the access list member was removed. Will only be populated for deletion",
							Type:        "string",
						},
					},
				},
			},
			{
				Name:        "message",
				Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "success",
				Description: "indicates the success or failure of the operation",
				Type:        "boolean",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
		},
	},
	"AccessListReview": {
		Description: "is emitted when an access list is reviewed",
		Fields: []*EventField{
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "error",
				Description: "includes system error message for the failed attempt",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "membership_requirements_changed",
				Description: "is populated if the memrship requirements have changed",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "roles",
						Description: "are the roles that changed as part of a review",
						Type:        "array",
						Items: &EventField{
							Type: "string",
						},
					},
					{
						Name:        "traits",
						Description: "are the traits that changed as part of a review",
						Type:        "object",
						Fields: []*EventField{
							{
								Name: "key",
								Type: "string",
							},
							{
								Name: "value",
								Type: "string",
							},
						},
					},
				},
			},
			{
				Name:        "message",
				Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "removed_members",
				Description: "are the members that were removed as part of the review",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "review_day_of_month_changed",
				Description: "is populated if the review day of month has changed",
				Type:        "string",
			},
			{
				Name:        "review_frequency_changed",
				Description: "is populated if the review frequency has changed",
				Type:        "string",
			},
			{
				Name:        "review_id",
				Description: "is the ID of the review",
				Type:        "string",
			},
			{
				Name:        "success",
				Description: "indicates the success or failure of the operation",
				Type:        "boolean",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
		},
	},
	"AccessListUpdate": {
		Description: "is emitted when an access list is updated",
		Fields: []*EventField{
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "error",
				Description: "includes system error message for the failed attempt",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "message",
				Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "success",
				Description: "indicates the success or failure of the operation",
				Type:        "boolean",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
		},
	},
	"AccessRequestCreate": {
		Description: "is emitted when access request has been created or updated",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "annotations",
				Description: "is an optional set of attributes supplied by a plugin during approval/denial of the request",
				Type:        "object",
			},
			{
				Name:        "assume_start_time",
				Description: "is the time the requested roles can be assumed",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "delegator",
				Description: "is used by teleport plugins to indicate the identity which caused them to update state",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "id",
				Description: "is access request ID",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "max_duration",
				Description: "indicates how long the access should be granted for",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "promoted_access_list_name",
				Description: "is the name of the access list that this request was promoted to. This field is only populated when the request is in the PROMOTED state",
				Type:        "string",
			},
			{
				Name:        "proposed_state",
				Description: "is the state proposed by a review (only used in the access_request.review event variant)",
				Type:        "string",
			},
			{
				Name:        "reason",
				Description: "is an optional description of why the request is being created or updated",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "resource_ids",
				Description: "is the set of resources to which access is being requested",
				Type:        "array",
				Items: &EventField{
					Type: "object",
					Fields: []*EventField{
						{
							Name:        "cluster",
							Description: "is the name of the cluster the resource is in",
							Type:        "string",
						},
						{
							Name:        "kind",
							Description: "is the resource kind",
							Type:        "string",
						},
						{
							Name:        "name",
							Description: "is the name of the specific resource",
							Type:        "string",
						},
						{
							Name:        "sub_resource",
							Description: "is the resource belonging to resource identified by \"Name\" that the user is allowed to access to. When granting access to a subresource, access to other resources is limited. Currently it just supports resources of Kind=pod and the format is the following \"<kube_namespace>/<kube_pod>\"",
							Type:        "string",
						},
					},
				},
			},
			{
				Name:        "reviewer",
				Description: "is the author of the review (only used in the access_request.review event variant)",
				Type:        "string",
			},
			{
				Name:        "roles",
				Description: "is a list of roles for the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "state",
				Description: "is access request state (in the access_request.review variant of the event this represents the post-review state of the request)",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"AccessRequestDelete": {
		Description: "is emitted when an access request has been deleted",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "id",
				Description: "is access request ID",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"AccessRequestResourceSearch": {
		Description: "is emitted when a user searches for resources as part of a search-based access request",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "labels",
				Description: "is the label-based matcher used for the search",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "namespace",
				Description: "is the namespace of resources",
				Type:        "string",
			},
			{
				Name:        "predicate_expression",
				Description: "is the list of boolean conditions that were used for the search",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "resource_type",
				Description: "is the type of resource being searched for",
				Type:        "string",
			},
			{
				Name:        "search_as_roles",
				Description: "is the list of roles the search was performed as",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "search_keywords",
				Description: "is the list of search keywords used to match against resource field values",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"AppCreate": {
		Description: "is emitted when a new application resource is created",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "app_labels",
				Description: "are the configured application labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "app_name",
				Description: "is the configured application name",
				Type:        "string",
			},
			{
				Name:        "app_public_addr",
				Description: "is the configured application public address",
				Type:        "string",
			},
			{
				Name:        "app_uri",
				Description: "is the application endpoint",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"AppDelete": {
		Description: "is emitted when an application resource is deleted",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"AppSessionChunk": {
		Description: "is emitted at the start of a 5 minute chunk on each proxy. This chunk is used to buffer 5 minutes of audit events at a time for applications",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "app_labels",
				Description: "are the configured application labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "app_name",
				Description: "is the configured application name",
				Type:        "string",
			},
			{
				Name:        "app_public_addr",
				Description: "is the configured application public address",
				Type:        "string",
			},
			{
				Name:        "app_uri",
				Description: "is the application endpoint",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "forwarded_by",
				Description: "tells us if the metadata was sent by the node itself or by another node in it's place. We can't verify emit permissions fully for these events so care should be taken with them",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "namespace",
				Description: "is a namespace of the server event",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "server_addr",
				Description: "is the address of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_hostname",
				Description: "is the hostname of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_id",
				Description: "is the UUID of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_labels",
				Description: "are the labels (static and dynamic) of the server the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "server_sub_kind",
				Description: "is the sub kind of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "session_chunk_id",
				Description: "is the ID of the session that was created for this 5 minute application log chunk",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"AppSessionDynamoDBRequest": {
		Description: "is emitted when a user executes a DynamoDB request via app access",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "app_labels",
				Description: "are the configured application labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "app_name",
				Description: "is the configured application name",
				Type:        "string",
			},
			{
				Name:        "app_public_addr",
				Description: "is the configured application public address",
				Type:        "string",
			},
			{
				Name:        "app_uri",
				Description: "is the application endpoint",
				Type:        "string",
			},
			{
				Name:        "aws_assumed_role",
				Description: "is the assumed role that signed this request",
				Type:        "string",
			},
			{
				Name:        "aws_host",
				Description: "is the requested host of the AWS service",
				Type:        "string",
			},
			{
				Name:        "aws_region",
				Description: "is the requested AWS region",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "aws_service",
				Description: "is the requested AWS service name",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "body",
				Description: "is the HTTP request json body. The Struct type is a wrapper around protobuf/types.Struct and is used to marshal the JSON body correctly",
				Type:        "object",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "method",
				Description: "is the request HTTP method, like GET/POST/DELETE/etc",
				Type:        "string",
			},
			{
				Name:        "path",
				Description: "is relative path in the URL",
				Type:        "string",
			},
			{
				Name:        "raw_query",
				Description: "are the encoded query values",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "session_chunk_id",
				Description: "is the ID of the app session chunk that this request belongs to. This is more appropriate to include than the app session id, since it is the chunk id that is needed to play back the session chunk with tsh. The session chunk event already includes the app session id",
				Type:        "string",
			},
			{
				Name:        "status_code",
				Description: "the HTTP response code for the request",
				Type:        "integer",
			},
			{
				Name:        "target",
				Description: "is the API target in the X-Amz-Target header",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"AppSessionEnd": {
		Description: "is emitted when an application session ends",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "app_labels",
				Description: "are the configured application labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "app_name",
				Description: "is the configured application name",
				Type:        "string",
			},
			{
				Name:        "app_public_addr",
				Description: "is the configured application public address",
				Type:        "string",
			},
			{
				Name:        "app_uri",
				Description: "is the application endpoint",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "forwarded_by",
				Description: "tells us if the metadata was sent by the node itself or by another node in it's place. We can't verify emit permissions fully for these events so care should be taken with them",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "namespace",
				Description: "is a namespace of the server event",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "server_addr",
				Description: "is the address of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_hostname",
				Description: "is the hostname of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_id",
				Description: "is the UUID of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_labels",
				Description: "are the labels (static and dynamic) of the server the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "server_sub_kind",
				Description: "is the sub kind of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"AppSessionRequest": {
		Description: "is an HTTP request and response",
		Fields: []*EventField{
			{
				Name:        "app_labels",
				Description: "are the configured application labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "app_name",
				Description: "is the configured application name",
				Type:        "string",
			},
			{
				Name:        "app_public_addr",
				Description: "is the configured application public address",
				Type:        "string",
			},
			{
				Name:        "app_uri",
				Description: "is the application endpoint",
				Type:        "string",
			},
			{
				Name:        "aws_assumed_role",
				Description: "is the assumed role that signed this request",
				Type:        "string",
			},
			{
				Name:        "aws_host",
				Description: "is the requested host of the AWS service",
				Type:        "string",
			},
			{
				Name:        "aws_region",
				Description: "is the requested AWS region",
				Type:        "string",
			},
			{
				Name:        "aws_service",
				Description: "is the requested AWS service name",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "method",
				Description: "is the request HTTP method, like GET/POST/DELETE/etc",
				Type:        "string",
			},
			{
				Name:        "path",
				Description: "is relative path in the URL",
				Type:        "string",
			},
			{
				Name:        "raw_query",
				Description: "are the encoded query values",
				Type:        "string",
			},
			{
				Name:        "status_code",
				Description: "the HTTP response code for the request",
				Type:        "integer",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
		},
	},
	"AppSessionStart": {
		Description: "is emitted when a user is issued an application certificate",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "app_labels",
				Description: "are the configured application labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "app_name",
				Description: "is the configured application name",
				Type:        "string",
			},
			{
				Name:        "app_public_addr",
				Description: "is the configured application public address",
				Type:        "string",
			},
			{
				Name:        "app_uri",
				Description: "is the application endpoint",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "forwarded_by",
				Description: "tells us if the metadata was sent by the node itself or by another node in it's place. We can't verify emit permissions fully for these events so care should be taken with them",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "namespace",
				Description: "is a namespace of the server event",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "public_addr",
				Description: "is the public address of the application being requested. DELETE IN 10.0: this information is also present on the AppMetadata",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "server_addr",
				Description: "is the address of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_hostname",
				Description: "is the hostname of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_id",
				Description: "is the UUID of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_labels",
				Description: "are the labels (static and dynamic) of the server the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "server_sub_kind",
				Description: "is the sub kind of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"AppUpdate": {
		Description: "is emitted when an existing application resource is updated",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "app_labels",
				Description: "are the configured application labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "app_name",
				Description: "is the configured application name",
				Type:        "string",
			},
			{
				Name:        "app_public_addr",
				Description: "is the configured application public address",
				Type:        "string",
			},
			{
				Name:        "app_uri",
				Description: "is the application endpoint",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"AuditQueryRun": {
		Description: "is emitted when a user runs an audit query",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "data_scanned_in_bytes",
				Description: "is the amount of data scanned by the query",
				Type:        "integer",
			},
			{
				Name:        "days",
				Description: "is the number of days time range for the query",
				Type:        "integer",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "error",
				Description: "includes system error message for the failed attempt",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "message",
				Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is the name of the query",
				Type:        "string",
			},
			{
				Name:        "query",
				Description: "is the query that was run",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "success",
				Description: "indicates the success or failure of the operation",
				Type:        "boolean",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "total_execution_time_in_millis",
				Description: "is the total execution time of the query",
				Type:        "integer",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"AuthAttempt": {
		Description: "is emitted upon a failed or successfull authentication attempt",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "error",
				Description: "includes system error message for the failed attempt",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "message",
				Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
				Type:        "string",
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "success",
				Description: "indicates the success or failure of the operation",
				Type:        "boolean",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"BillingCardCreate": {
		Description: "is emitted when a user creates or updates a credit card",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"BillingCardDelete": {
		Description: "is emitted when a user deletes a credit card",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"BillingInformationUpdate": {
		Description: "is emitted when a user updates the billing information",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"BotJoin": {
		Description: "records a bot join event",
		Fields: []*EventField{
			{
				Name:        "attributes",
				Description: "is a map of attributes received from the join method provider",
				Type:        "object",
			},
			{
				Name:        "bot_name",
				Description: "is the name of the bot which has joined",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "error",
				Description: "includes system error message for the failed attempt",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "message",
				Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
				Type:        "string",
			},
			{
				Name:        "method",
				Description: "is the event field indicating what join method was used",
				Type:        "string",
			},
			{
				Name:        "success",
				Description: "indicates the success or failure of the operation",
				Type:        "boolean",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "token_name",
				Description: "is the name of the provision token used to join",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
		},
	},
	"CassandraBatch": {
		Description: "is emitted when a Cassandra client executes a batch of CQL statements",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "batch_type",
				Description: "is the type of batch",
				Type:        "string",
			},
			{
				Name:        "children",
				Description: "is batch children statements",
				Type:        "array",
				Items: &EventField{
					Type: "object",
					Fields: []*EventField{
						{
							Name: "id",
							Type: "string",
						},
						{
							Name: "query",
							Type: "string",
						},
						{
							Name: "values",
							Type: "array",
							Items: &EventField{
								Type: "object",
								Fields: []*EventField{
									{
										Name: "type",
										Type: "integer",
									},
								},
							},
						},
					},
				},
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "consistency",
				Description: "is the consistency level to use",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "keyspace",
				Description: "is the keyspace the statement is in",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"CassandraExecute": {
		Description: "is emitted when a Cassandra client executes a CQL statement",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "query_id",
				Description: "is the prepared query id to execute",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"CassandraPrepare": {
		Description: "is emitted when a Cassandra client sends the prepare a CQL statement",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "keyspace",
				Description: "is the keyspace the statement is in",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "query",
				Description: "is the CQL statement",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"CassandraRegister": {
		Description: "is emitted when a Cassandra client request to register for the specified event types",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "event_types",
				Description: "is the list of event types to register for",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"CertificateCreate": {
		Description: "is emitted when a certificate is issued",
		Fields: []*EventField{
			{
				Name:        "cert_type",
				Description: "is the type of certificate that was just issued",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "identity",
				Description: "is the identity associated with the certificate, as interpreted by Teleport",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "access_requests",
						Description: "is a list of UUIDs of active requests for this Identity",
						Type:        "array",
						Items: &EventField{
							Type: "string",
						},
					},
					{
						Name:        "allowed_resource_ids",
						Description: "is the list of resources which the identity will be allowed to access. An empty list indicates that no resource-specific restrictions will be applied",
						Type:        "array",
						Items: &EventField{
							Type: "object",
							Fields: []*EventField{
								{
									Name:        "cluster",
									Description: "is the name of the cluster the resource is in",
									Type:        "string",
								},
								{
									Name:        "kind",
									Description: "is the resource kind",
									Type:        "string",
								},
								{
									Name:        "name",
									Description: "is the name of the specific resource",
									Type:        "string",
								},
								{
									Name:        "sub_resource",
									Description: "is the resource belonging to resource identified by \"Name\" that the user is allowed to access to. When granting access to a subresource, access to other resources is limited. Currently it just supports resources of Kind=pod and the format is the following \"<kube_namespace>/<kube_pod>\"",
									Type:        "string",
								},
							},
						},
					},
					{
						Name:        "aws_role_arns",
						Description: "is a list of allowed AWS role ARNs user can assume",
						Type:        "array",
						Items: &EventField{
							Type: "string",
						},
					},
					{
						Name:        "azure_identities",
						Description: "is a list of allowed Azure identities user can assume",
						Type:        "array",
						Items: &EventField{
							Type: "string",
						},
					},
					{
						Name:        "client_ip",
						Description: "is an observed IP of the client that this Identity represents",
						Type:        "string",
					},
					{
						Name:        "database_names",
						Description: "is a list of allowed database names",
						Type:        "array",
						Items: &EventField{
							Type: "string",
						},
					},
					{
						Name:        "database_users",
						Description: "is a list of allowed database users",
						Type:        "array",
						Items: &EventField{
							Type: "string",
						},
					},
					{
						Name:        "disallow_reissue",
						Description: "is a flag that, if set, instructs the auth server to deny any attempts to reissue new certificates while authenticated with this certificate",
						Type:        "boolean",
					},
					{
						Name:        "expires",
						Description: "specifies whenever the session will expire",
						Type:        "string",
					},
					{
						Name:        "gcp_service_accounts",
						Description: "is a list of allowed GCP service accounts user can assume",
						Type:        "array",
						Items: &EventField{
							Type: "string",
						},
					},
					{
						Name:        "impersonator",
						Description: "is a username of a user impersonating this user",
						Type:        "string",
					},
					{
						Name:        "kubernetes_cluster",
						Description: "specifies the target kubernetes cluster for TLS identities. This can be empty on older Teleport clients",
						Type:        "string",
					},
					{
						Name:        "kubernetes_groups",
						Description: "is a list of Kubernetes groups allowed",
						Type:        "array",
						Items: &EventField{
							Type: "string",
						},
					},
					{
						Name:        "kubernetes_users",
						Description: "is a list of Kubernetes users allowed",
						Type:        "array",
						Items: &EventField{
							Type: "string",
						},
					},
					{
						Name:        "logins",
						Description: "is a list of Unix logins allowed",
						Type:        "array",
						Items: &EventField{
							Type: "string",
						},
					},
					{
						Name:        "mfa_device_uuid",
						Description: "is the UUID of an MFA device when this Identity was confirmed immediately after an MFA check",
						Type:        "string",
					},
					{
						Name:        "prev_identity_expires",
						Description: "is the expiry time of the identity/cert that this identity/cert was derived from. It is used to determine a session's hard deadline in cases where both require_session_mfa and disconnect_expired_cert are enabled. See https://github.com/gravitational/teleport/issues/18544",
						Type:        "string",
					},
					{
						Name:        "private_key_policy",
						Description: "is the private key policy of the user's private key",
						Type:        "string",
					},
					{
						Name:        "roles",
						Description: "is a list of groups (Teleport roles) encoded in the identity",
						Type:        "array",
						Items: &EventField{
							Type: "string",
						},
					},
					{
						Name:        "route_to_app",
						Description: "holds routing information for applications. Routing metadata allows Teleport web proxy to route HTTP requests to the appropriate cluster and Teleport application proxy within the cluster",
						Type:        "object",
						Fields: []*EventField{
							{
								Name:        "aws_role_arn",
								Description: "is the AWS role to assume when accessing AWS API",
								Type:        "string",
							},
							{
								Name:        "azure_identity",
								Description: "is the Azure identity ot assume when accessing Azure API",
								Type:        "string",
							},
							{
								Name:        "cluster_name",
								Description: "is the cluster where the application resides",
								Type:        "string",
							},
							{
								Name:        "gcp_service_account",
								Description: "is the GCP service account to assume when accessing GCP API",
								Type:        "string",
							},
							{
								Name:        "name",
								Description: "is the application name certificate is being requested for",
								Type:        "string",
							},
							{
								Name:        "public_addr",
								Description: "is the application public address",
								Type:        "string",
							},
							{
								Name:        "session_id",
								Description: "is the ID of the application session",
								Type:        "string",
							},
						},
					},
					{
						Name:        "route_to_cluster",
						Description: "specifies the target cluster if present in the session",
						Type:        "string",
					},
					{
						Name:        "route_to_database",
						Description: "contains routing information for databases",
						Type:        "object",
						Fields: []*EventField{
							{
								Name:        "database",
								Description: "is an optional database name to embed",
								Type:        "string",
							},
							{
								Name:        "protocol",
								Description: "is the type of the database the cert is for",
								Type:        "string",
							},
							{
								Name:        "service_name",
								Description: "is the Teleport database proxy service name the cert is for",
								Type:        "string",
							},
							{
								Name:        "username",
								Description: "is an optional database username to embed",
								Type:        "string",
							},
						},
					},
					{
						Name:        "teleport_cluster",
						Description: "is the name of the teleport cluster that this identity originated from. For TLS certs this may not be the same as cert issuer, in case of multi-hop requests that originate from a remote cluster",
						Type:        "string",
					},
					{
						Name:        "traits",
						Description: "hold claim data used to populate a role at runtime",
						Type:        "object",
					},
					{
						Name:        "usage",
						Description: "is a list of usage restrictions encoded in the identity",
						Type:        "array",
						Items: &EventField{
							Type: "string",
						},
					},
					{
						Name:        "user",
						Description: "is a username or name of the node connection",
						Type:        "string",
					},
				},
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
		},
	},
	"ClientDisconnect": {
		Description: "is emitted when client is disconnected by the server due to inactivity or any other reason",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "forwarded_by",
				Description: "tells us if the metadata was sent by the node itself or by another node in it's place. We can't verify emit permissions fully for these events so care should be taken with them",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "namespace",
				Description: "is a namespace of the server event",
				Type:        "string",
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "reason",
				Description: "is a field that specifies reason for event, e.g. in disconnect event it explains why server disconnected the client",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "server_addr",
				Description: "is the address of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_hostname",
				Description: "is the hostname of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_id",
				Description: "is the UUID of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_labels",
				Description: "are the labels (static and dynamic) of the server the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "server_sub_kind",
				Description: "is the sub kind of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"DatabaseCreate": {
		Description: "is emitted when a new database resource is created",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"DatabaseDelete": {
		Description: "is emitted when a database resource is deleted",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"DatabaseSessionEnd": {
		Description: "is emitted when a user ends the database session",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"DatabaseSessionMalformedPacket": {
		Description: "is emitted when a database sends a malformed packet",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"DatabaseSessionQuery": {
		Description: "is emitted when a user executes a database query",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_query",
				Description: "is the executed query string",
				Type:        "string",
			},
			{
				Name:        "db_query_parameters",
				Description: "are the query parameters for prepared statements",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "error",
				Description: "includes system error message for the failed attempt",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "message",
				Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "success",
				Description: "indicates the success or failure of the operation",
				Type:        "boolean",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"DatabaseSessionStart": {
		Description: "is emitted when a user connects to a database",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "error",
				Description: "includes system error message for the failed attempt",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "forwarded_by",
				Description: "tells us if the metadata was sent by the node itself or by another node in it's place. We can't verify emit permissions fully for these events so care should be taken with them",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "message",
				Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
				Type:        "string",
			},
			{
				Name:        "namespace",
				Description: "is a namespace of the server event",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "server_addr",
				Description: "is the address of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_hostname",
				Description: "is the hostname of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_id",
				Description: "is the UUID of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_labels",
				Description: "are the labels (static and dynamic) of the server the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "server_sub_kind",
				Description: "is the sub kind of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "success",
				Description: "indicates the success or failure of the operation",
				Type:        "boolean",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"DatabaseUpdate": {
		Description: "is emitted when an existing database resource is updated",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"DesktopClipboardReceive": {
		Description: "is emitted when Teleport receives clipboard data from a remote desktop",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "desktop_addr",
				Description: "is the address of the desktop being accessed",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "length",
				Description: "is the number of bytes of data received from the remote clipboard",
				Type:        "integer",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"DesktopClipboardSend": {
		Description: "is emitted when clipboard data is sent from a user's workstation to Teleport",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "desktop_addr",
				Description: "is the address of the desktop being accessed",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "length",
				Description: "is the number of bytes of data sent",
				Type:        "integer",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"DesktopRecording": {
		Description: "happens when a Teleport Desktop Protocol message is captured during a Desktop Access Session",
		Fields: []*EventField{
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "ms",
				Description: "is the delay in milliseconds from the start of the session",
				Type:        "integer",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
		},
	},
	"DesktopSharedDirectoryRead": {
		Description: "is emitted when Teleport attempts to read from a file in a shared directory at the behest of the remote desktop",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "desktop_addr",
				Description: "is the address of the desktop being accessed",
				Type:        "string",
			},
			{
				Name:        "directory_id",
				Description: "is the ID of the directory being shared (unique to the Windows Desktop Session)",
				Type:        "integer",
			},
			{
				Name:        "directory_name",
				Description: "is the name of the directory being shared",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "error",
				Description: "includes system error message for the failed attempt",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "file_path",
				Description: "is the path within the shared directory where the file is located",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "length",
				Description: "is the number of bytes read",
				Type:        "integer",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "message",
				Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
				Type:        "string",
			},
			{
				Name:        "offset",
				Description: "is the offset the bytes were read from",
				Type:        "integer",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "success",
				Description: "indicates the success or failure of the operation",
				Type:        "boolean",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"DesktopSharedDirectoryStart": {
		Description: "is emitted when Teleport successfully begins sharing a new directory to a remote desktop",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "desktop_addr",
				Description: "is the address of the desktop being accessed",
				Type:        "string",
			},
			{
				Name:        "directory_id",
				Description: "is the ID of the directory being shared (unique to the Windows Desktop Session)",
				Type:        "integer",
			},
			{
				Name:        "directory_name",
				Description: "is the name of the directory being shared",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "error",
				Description: "includes system error message for the failed attempt",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "message",
				Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "success",
				Description: "indicates the success or failure of the operation",
				Type:        "boolean",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"DesktopSharedDirectoryWrite": {
		Description: "is emitted when Teleport attempts to write to a file in a shared directory at the behest of the remote desktop",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "desktop_addr",
				Description: "is the address of the desktop being accessed",
				Type:        "string",
			},
			{
				Name:        "directory_id",
				Description: "is the ID of the directory being shared (unique to the Windows Desktop Session)",
				Type:        "integer",
			},
			{
				Name:        "directory_name",
				Description: "is the name of the directory being shared",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "error",
				Description: "includes system error message for the failed attempt",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "file_path",
				Description: "is the path within the shared directory where the file is located",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "length",
				Description: "is the number of bytes written",
				Type:        "integer",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "message",
				Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
				Type:        "string",
			},
			{
				Name:        "offset",
				Description: "is the offset the bytes were written to",
				Type:        "integer",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "success",
				Description: "indicates the success or failure of the operation",
				Type:        "boolean",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"DeviceEvent": {
		Description: "is a device-related event. The event type (Metadata.Type) for device events is always \"device\". See the event code (Metadata.Code) for its meaning. Deprecated: Use DeviceEvent2 instead",
		Fields: []*EventField{
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "device",
				Description: "holds metadata about the user device",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "status",
				Description: "indicates the outcome of the event",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "error",
						Description: "includes system error message for the failed attempt",
						Type:        "string",
					},
					{
						Name:        "message",
						Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
						Type:        "string",
					},
					{
						Name:        "success",
						Description: "indicates the success or failure of the operation",
						Type:        "boolean",
					},
				},
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "holds metadata about the user behind the event",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "access_requests",
						Description: "are the IDs of access requests created by the user",
						Type:        "array",
						Items: &EventField{
							Type: "string",
						},
					},
					{
						Name:        "aws_role_arn",
						Description: "is AWS IAM role user assumes when accessing AWS console",
						Type:        "string",
					},
					{
						Name:        "azure_identity",
						Description: "is the Azure identity user assumes when accessing Azure API",
						Type:        "string",
					},
					{
						Name:        "gcp_service_account",
						Description: "is the GCP service account user assumes when accessing GCP API",
						Type:        "string",
					},
					{
						Name:        "impersonator",
						Description: "is a user acting on behalf of another user",
						Type:        "string",
					},
					{
						Name:        "login",
						Description: "is OS login",
						Type:        "string",
					},
					{
						Name:        "required_private_key_policy",
						Description: "is the private key policy enforced for this login",
						Type:        "string",
					},
					{
						Name:        "trusted_device",
						Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
						Type:        "object",
						Fields: []*EventField{
							{
								Name:        "asset_tag",
								Description: "inventory identifier",
								Type:        "string",
							},
							{
								Name:        "credential_id",
								Description: "credential identifier",
								Type:        "string",
							},
							{
								Name:        "device_id",
								Description: "of the device",
								Type:        "string",
							},
							{
								Name:        "device_origin",
								Description: "origin",
								Type:        "integer",
							},
							{
								Name:        "os_type",
								Description: "of the device",
								Type:        "integer",
							},
						},
					},
					{
						Name:        "user",
						Description: "is teleport user name",
						Type:        "string",
					},
				},
			},
		},
	},
	"DeviceEvent2": {
		Description: "is a device-related event. See the \"lib/events.Device*Event\" and \"lib/events.Device*Code\" for the various event types and codes, respectively. Replaces the previous [DeviceEvent] proto, presenting a more standard event interface with various embeds",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "device",
				Description: "holds metadata about the user device",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "error",
				Description: "includes system error message for the failed attempt",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "message",
				Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "success",
				Description: "indicates the success or failure of the operation",
				Type:        "boolean",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"DynamoDBRequest": {
		Description: "is emitted when a user executes a DynamoDB request via database-access",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "body",
				Description: "is the HTTP request json body. The Struct type is a wrapper around protobuf/types.Struct and is used to marshal the JSON body correctly",
				Type:        "object",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "method",
				Description: "is the request HTTP method, like GET/POST/DELETE/etc",
				Type:        "string",
			},
			{
				Name:        "path",
				Description: "is relative path in the URL",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "raw_query",
				Description: "are the encoded query values",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name: "status_code",
				Type: "integer",
			},
			{
				Name:        "target",
				Description: "is the API target in the X-Amz-Target header",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"ElasticsearchRequest": {
		Description: "is emitted when user executes an Elasticsearch request, which isn't covered by API-specific events",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "category",
				Description: "represents the category if API being accessed in a given request",
				Type:        "integer",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "headers",
				Description: "are the HTTP request headers",
				Type:        "object",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "method",
				Description: "is the request HTTP method, like GET/POST/DELETE/etc",
				Type:        "string",
			},
			{
				Name:        "path",
				Description: "is relative path in the URL",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "query",
				Description: "is an optional text of query (e.g. an SQL select statement for _sql API), if a request includes it",
				Type:        "string",
			},
			{
				Name:        "raw_query",
				Description: "are the encoded query values",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "status_code",
				Description: "is optional status code returned from the call to database",
				Type:        "integer",
			},
			{
				Name:        "target",
				Description: "is an optional field indicating the target index or set of indices used as a subject of request",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"Exec": {
		Description: "specifies command exec event",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "command",
				Description: "is the executed command name",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "exitCode",
				Description: "specifies command exit code",
				Type:        "string",
			},
			{
				Name:        "exitError",
				Description: "is an optional exit error, set if command has failed",
				Type:        "string",
			},
			{
				Name:        "forwarded_by",
				Description: "tells us if the metadata was sent by the node itself or by another node in it's place. We can't verify emit permissions fully for these events so care should be taken with them",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "kubernetes_cluster",
				Description: "is a kubernetes cluster name",
				Type:        "string",
			},
			{
				Name:        "kubernetes_container_image",
				Description: "is the image of the container within the pod",
				Type:        "string",
			},
			{
				Name:        "kubernetes_container_name",
				Description: "is the name of the container within the pod",
				Type:        "string",
			},
			{
				Name:        "kubernetes_groups",
				Description: "is a list of kubernetes groups for the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "kubernetes_labels",
				Description: "are the labels (static and dynamic) of the kubernetes cluster the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "kubernetes_node_name",
				Description: "is the node that runs the pod",
				Type:        "string",
			},
			{
				Name:        "kubernetes_pod_name",
				Description: "is the name of the pod",
				Type:        "string",
			},
			{
				Name:        "kubernetes_pod_namespace",
				Description: "is the namespace of the pod",
				Type:        "string",
			},
			{
				Name:        "kubernetes_users",
				Description: "is a list of kubernetes usernames for the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "namespace",
				Description: "is a namespace of the server event",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "server_addr",
				Description: "is the address of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_hostname",
				Description: "is the hostname of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_id",
				Description: "is the UUID of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_labels",
				Description: "are the labels (static and dynamic) of the server the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "server_sub_kind",
				Description: "is the sub kind of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"ExternalAuditStorageDisable": {
		Description: "is emitted when External Audit Storage is disabled",
		Fields: []*EventField{
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "details",
				Description: "holds details about the External Audit Storage configuration that was disabled",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "athenaResultsUri",
						Description: "is the S3 path used to store temporary results generated by Athena",
						Type:        "string",
					},
					{
						Name:        "athenaWorkgroup",
						Description: "is the workgroup used for Athena audit log queries",
						Type:        "string",
					},
					{
						Name:        "auditEventsLongTermUri",
						Description: "is the S3 path used to store batched parquet files with audit events, partitioned by event date",
						Type:        "string",
					},
					{
						Name:        "glueDatabase",
						Description: "is the database used for Athena audit log queries",
						Type:        "string",
					},
					{
						Name:        "glueTable",
						Description: "is the table used for Athena audit log queries",
						Type:        "string",
					},
					{
						Name:        "integrationName",
						Description: "is the name of the AWS OIDC integration used",
						Type:        "string",
					},
					{
						Name:        "policyName",
						Description: "is the name of the IAM policy attached to the OIDC integration role",
						Type:        "string",
					},
					{
						Name:        "region",
						Description: "is the AWS region where the infrastructure is hosted",
						Type:        "string",
					},
					{
						Name:        "sessionRecordingsUri",
						Description: "is the S3 path used to store session recordings",
						Type:        "string",
					},
				},
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
		},
	},
	"ExternalAuditStorageEnable": {
		Description: "is emitted when External Audit Storage is enabled",
		Fields: []*EventField{
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "details",
				Description: "holds details about the External Audit Storage configuration that was enabled",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "athenaResultsUri",
						Description: "is the S3 path used to store temporary results generated by Athena",
						Type:        "string",
					},
					{
						Name:        "athenaWorkgroup",
						Description: "is the workgroup used for Athena audit log queries",
						Type:        "string",
					},
					{
						Name:        "auditEventsLongTermUri",
						Description: "is the S3 path used to store batched parquet files with audit events, partitioned by event date",
						Type:        "string",
					},
					{
						Name:        "glueDatabase",
						Description: "is the database used for Athena audit log queries",
						Type:        "string",
					},
					{
						Name:        "glueTable",
						Description: "is the table used for Athena audit log queries",
						Type:        "string",
					},
					{
						Name:        "integrationName",
						Description: "is the name of the AWS OIDC integration used",
						Type:        "string",
					},
					{
						Name:        "policyName",
						Description: "is the name of the IAM policy attached to the OIDC integration role",
						Type:        "string",
					},
					{
						Name:        "region",
						Description: "is the AWS region where the infrastructure is hosted",
						Type:        "string",
					},
					{
						Name:        "sessionRecordingsUri",
						Description: "is the S3 path used to store session recordings",
						Type:        "string",
					},
				},
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
		},
	},
	"GithubConnectorCreate": {
		Description: "fires when a Github connector is created",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"GithubConnectorDelete": {
		Description: "fires when a Github connector is deleted",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"GithubConnectorUpdate": {
		Description: "fires when a Github connector is updated",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"InstanceJoin": {
		Description: "records an instance join event",
		Fields: []*EventField{
			{
				Name:        "attributes",
				Description: "is a map of attributes received from the join method provider",
				Type:        "object",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "error",
				Description: "includes system error message for the failed attempt",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "host_id",
				Description: "is the unique host ID of the instance which attempted to join",
				Type:        "string",
			},
			{
				Name:        "message",
				Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
				Type:        "string",
			},
			{
				Name:        "method",
				Description: "is the event field indicating what join method was used",
				Type:        "string",
			},
			{
				Name:        "node_name",
				Description: "is the name of the instance which attempted to join",
				Type:        "string",
			},
			{
				Name:        "role",
				Description: "is the role that the node requested when attempting to join",
				Type:        "string",
			},
			{
				Name:        "success",
				Description: "indicates the success or failure of the operation",
				Type:        "boolean",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "token_expires",
				Description: "contain information about token expiration time. In case of static token the TokenExpiration time is to the Unix epoch start time",
				Type:        "string",
			},
			{
				Name:        "token_name",
				Description: "is the name of the token used to join. This will be omitted for the 'token' join method where the token name is a secret value",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
		},
	},
	"KubeRequest": {
		Description: "specifies a Kubernetes API request event",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "forwarded_by",
				Description: "tells us if the metadata was sent by the node itself or by another node in it's place. We can't verify emit permissions fully for these events so care should be taken with them",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "kubernetes_cluster",
				Description: "is a kubernetes cluster name",
				Type:        "string",
			},
			{
				Name:        "kubernetes_groups",
				Description: "is a list of kubernetes groups for the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "kubernetes_labels",
				Description: "are the labels (static and dynamic) of the kubernetes cluster the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "kubernetes_users",
				Description: "is a list of kubernetes usernames for the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "namespace",
				Description: "is a namespace of the server event",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "request_path",
				Description: "is the raw request URL path",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "resource_api_group",
				Description: "is the resource API group",
				Type:        "string",
			},
			{
				Name:        "resource_kind",
				Description: "is the API resource kind (e.g. \"pod\", \"service\", etc)",
				Type:        "string",
			},
			{
				Name:        "resource_name",
				Description: "is the API resource name",
				Type:        "string",
			},
			{
				Name:        "resource_namespace",
				Description: "is the resource namespace",
				Type:        "string",
			},
			{
				Name:        "response_code",
				Description: "is the HTTP response code for this request",
				Type:        "integer",
			},
			{
				Name:        "server_addr",
				Description: "is the address of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_hostname",
				Description: "is the hostname of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_id",
				Description: "is the UUID of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_labels",
				Description: "are the labels (static and dynamic) of the server the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "server_sub_kind",
				Description: "is the sub kind of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "verb",
				Description: "is the HTTP verb used for this request (e.g. GET, POST, etc)",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"KubernetesClusterCreate": {
		Description: "is emitted when a new kubernetes cluster resource is created",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "kube_labels",
				Description: "are the configured cluster labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"KubernetesClusterDelete": {
		Description: "is emitted when a kubernetes cluster resource is deleted",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"KubernetesClusterUpdate": {
		Description: "is emitted when an existing kubernetes cluster resource is updated",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "kube_labels",
				Description: "are the configured cluster labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"LockCreate": {
		Description: "is emitted when a lock is created/updated. Locks are used to restrict access to a Teleport environment by disabling interactions involving a user, an RBAC role, a node, etc. See rfd/0009-locking.md for more details",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "target",
				Description: "describes the set of interactions that the lock applies to",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "access_request",
						Description: "specifies the UUID of an access request",
						Type:        "string",
					},
					{
						Name:        "device",
						Description: "is the device ID of a trusted device. Requires Teleport Enterprise",
						Type:        "string",
					},
					{
						Name:        "login",
						Description: "specifies the name of a local UNIX user",
						Type:        "string",
					},
					{
						Name:        "mfa_device",
						Description: "specifies the UUID of a user MFA device",
						Type:        "string",
					},
					{
						Name:        "node",
						Description: "specifies the UUID of a Teleport node. A matching node is also prevented from heartbeating to the auth server. DEPRECATED: use ServerID instead",
						Type:        "string",
					},
					{
						Name:        "role",
						Description: "specifies the name of an RBAC role known to the root cluster. In remote clusters, this constraint is evaluated before translating to local roles",
						Type:        "string",
					},
					{
						Name:        "server_id",
						Description: "is the host id of the Teleport instance",
						Type:        "string",
					},
					{
						Name:        "user",
						Description: "specifies the name of a Teleport user",
						Type:        "string",
					},
					{
						Name:        "windows_desktop",
						Description: "specifies the name of a Windows desktop",
						Type:        "string",
					},
				},
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"LockDelete": {
		Description: "is emitted when a lock is deleted",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"LoginRuleCreate": {
		Description: "is emitted when a login rule is created or updated",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"LoginRuleDelete": {
		Description: "is emitted when a login rule is deleted",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"MFADeviceAdd": {
		Description: "is emitted when a user adds an MFA device",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "mfa_device_name",
				Description: "is the user-specified name of the MFA device",
				Type:        "string",
			},
			{
				Name:        "mfa_device_type",
				Description: "is the type of this MFA device",
				Type:        "string",
			},
			{
				Name:        "mfa_device_uuid",
				Description: "is the UUID of the MFA device generated by Teleport",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"MFADeviceDelete": {
		Description: "is emitted when a user deletes an MFA device",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "mfa_device_name",
				Description: "is the user-specified name of the MFA device",
				Type:        "string",
			},
			{
				Name:        "mfa_device_type",
				Description: "is the type of this MFA device",
				Type:        "string",
			},
			{
				Name:        "mfa_device_uuid",
				Description: "is the UUID of the MFA device generated by Teleport",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"MySQLCreateDB": {
		Description: "is emitted when a MySQL client creates a schema",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "schema_name",
				Description: "is the name of the schema to create",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"MySQLDebug": {
		Description: "is emitted when a MySQL client asks the server to dump internal debug info to stdout",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"MySQLDropDB": {
		Description: "is emitted when a MySQL client drops a schema",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "schema_name",
				Description: "is the name of the schema to drop",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"MySQLInitDB": {
		Description: "is emitted when a MySQL client changes the default schema for the connection",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "schema_name",
				Description: "is the name of the schema to use",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"MySQLProcessKill": {
		Description: "is emitted when a MySQL client asks the server to terminate a connection",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "process_id",
				Description: "is the process ID of a connection",
				Type:        "integer",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"MySQLRefresh": {
		Description: "is emitted when a MySQL client sends refresh commands",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "subcommand",
				Description: "is the string representation of the subcommand",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"MySQLShutDown": {
		Description: "is emitted when a MySQL client asks the server to shut down",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"MySQLStatementBulkExecute": {
		Description: "is emitted when a MySQL client executes a bulk insert of a prepared statement using the prepared statement protocol",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "parameters",
				Description: "are the parameters used to execute the prepared statement",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "statement_id",
				Description: "is the identifier of the prepared statement",
				Type:        "integer",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"MySQLStatementClose": {
		Description: "is emitted when a MySQL client deallocates a prepared statement using the prepared statement protocol",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "statement_id",
				Description: "is the identifier of the prepared statement",
				Type:        "integer",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"MySQLStatementExecute": {
		Description: "is emitted when a MySQL client executes a prepared statement using the prepared statement protocol",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "parameters",
				Description: "are the parameters used to execute the prepared statement",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "statement_id",
				Description: "is the identifier of the prepared statement",
				Type:        "integer",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"MySQLStatementFetch": {
		Description: "is emitted when a MySQL client fetches rows from a prepared statement using the prepared statement protocol",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "rows_count",
				Description: "is the number of rows to fetch",
				Type:        "integer",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "statement_id",
				Description: "is the identifier of the prepared statement",
				Type:        "integer",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"MySQLStatementPrepare": {
		Description: "is emitted when a MySQL client creates a prepared statement using the prepared statement protocol",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "query",
				Description: "is the prepared statement query",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"MySQLStatementReset": {
		Description: "is emitted when a MySQL client resets the data of a prepared statement using the prepared statement protocol",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "statement_id",
				Description: "is the identifier of the prepared statement",
				Type:        "integer",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"MySQLStatementSendLongData": {
		Description: "is emitted when a MySQL client sends long bytes stream using the prepared statement protocol",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "data_size",
				Description: "is the size of the data",
				Type:        "integer",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "parameter_id",
				Description: "is the identifier of the parameter",
				Type:        "integer",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "statement_id",
				Description: "is the identifier of the prepared statement",
				Type:        "integer",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"OIDCConnectorCreate": {
		Description: "fires when OIDC connector is created",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"OIDCConnectorDelete": {
		Description: "fires when OIDC connector is deleted",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"OIDCConnectorUpdate": {
		Description: "fires when OIDC connector is updated",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"OktaAssignmentResult": {
		Description: "is emitted when an Okta assignment processing or cleanup was attempted",
		Fields: []*EventField{
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "ending_status",
				Description: "is the ending status of the assignment",
				Type:        "string",
			},
			{
				Name:        "error",
				Description: "includes system error message for the failed attempt",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "forwarded_by",
				Description: "tells us if the metadata was sent by the node itself or by another node in it's place. We can't verify emit permissions fully for these events so care should be taken with them",
				Type:        "string",
			},
			{
				Name:        "message",
				Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "namespace",
				Description: "is a namespace of the server event",
				Type:        "string",
			},
			{
				Name:        "server_addr",
				Description: "is the address of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_hostname",
				Description: "is the hostname of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_id",
				Description: "is the UUID of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_labels",
				Description: "are the labels (static and dynamic) of the server the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "server_sub_kind",
				Description: "is the sub kind of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "source",
				Description: "is the source of the Okta assignment",
				Type:        "string",
			},
			{
				Name:        "starting_status",
				Description: "is the starting status of the assignment",
				Type:        "string",
			},
			{
				Name:        "success",
				Description: "indicates the success or failure of the operation",
				Type:        "boolean",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is the user the Okta assignment is for",
				Type:        "string",
			},
		},
	},
	"OktaResourcesUpdate": {
		Description: "is emitted when Okta related resources have been updated",
		Fields: []*EventField{
			{
				Name:        "added",
				Description: "is the number of resources added",
				Type:        "integer",
			},
			{
				Name:        "added_resources",
				Description: "is a list of the actual resources that were added",
				Type:        "array",
				Items: &EventField{
					Type: "object",
					Fields: []*EventField{
						{
							Name:        "description",
							Description: "is the description of the Okta resource",
							Type:        "string",
						},
						{
							Name:        "id",
							Description: "is the identifier of the Okta resource",
							Type:        "string",
						},
					},
				},
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "deleted",
				Description: "is the number of resources deleted",
				Type:        "integer",
			},
			{
				Name:        "deleted_resources",
				Description: "is a list of the actual resources that were deleted",
				Type:        "array",
				Items: &EventField{
					Type: "object",
					Fields: []*EventField{
						{
							Name:        "description",
							Description: "is the description of the Okta resource",
							Type:        "string",
						},
						{
							Name:        "id",
							Description: "is the identifier of the Okta resource",
							Type:        "string",
						},
					},
				},
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "forwarded_by",
				Description: "tells us if the metadata was sent by the node itself or by another node in it's place. We can't verify emit permissions fully for these events so care should be taken with them",
				Type:        "string",
			},
			{
				Name:        "namespace",
				Description: "is a namespace of the server event",
				Type:        "string",
			},
			{
				Name:        "server_addr",
				Description: "is the address of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_hostname",
				Description: "is the hostname of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_id",
				Description: "is the UUID of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_labels",
				Description: "are the labels (static and dynamic) of the server the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "server_sub_kind",
				Description: "is the sub kind of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated",
				Description: "is the number of resources updated",
				Type:        "integer",
			},
			{
				Name:        "updated_resources",
				Description: "is a list of the actual resources that were updated",
				Type:        "array",
				Items: &EventField{
					Type: "object",
					Fields: []*EventField{
						{
							Name:        "description",
							Description: "is the description of the Okta resource",
							Type:        "string",
						},
						{
							Name:        "id",
							Description: "is the identifier of the Okta resource",
							Type:        "string",
						},
					},
				},
			},
		},
	},
	"OktaSyncFailure": {
		Description: "is emitted when an Okta synchronization attempt has failed",
		Fields: []*EventField{
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "error",
				Description: "includes system error message for the failed attempt",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "forwarded_by",
				Description: "tells us if the metadata was sent by the node itself or by another node in it's place. We can't verify emit permissions fully for these events so care should be taken with them",
				Type:        "string",
			},
			{
				Name:        "message",
				Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
				Type:        "string",
			},
			{
				Name:        "namespace",
				Description: "is a namespace of the server event",
				Type:        "string",
			},
			{
				Name:        "server_addr",
				Description: "is the address of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_hostname",
				Description: "is the hostname of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_id",
				Description: "is the UUID of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_labels",
				Description: "are the labels (static and dynamic) of the server the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "server_sub_kind",
				Description: "is the sub kind of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "success",
				Description: "indicates the success or failure of the operation",
				Type:        "boolean",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
		},
	},
	"OpenSearchRequest": {
		Description: "is emitted when a user executes a OpenSearch request via database-access",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "category",
				Description: "represents the category if API being accessed in a given request",
				Type:        "integer",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "headers",
				Description: "are the HTTP request headers",
				Type:        "object",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "method",
				Description: "is the request HTTP method, like GET/POST/DELETE/etc",
				Type:        "string",
			},
			{
				Name:        "path",
				Description: "is relative path in the URL",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "query",
				Description: "is an optional text of query (e.g. an SQL select statement for _sql API), if a request includes it",
				Type:        "string",
			},
			{
				Name:        "raw_query",
				Description: "are the encoded query values",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "status_code",
				Description: "is optional status code returned from the call to database",
				Type:        "integer",
			},
			{
				Name:        "target",
				Description: "is an optional field indicating the target index or set of indices used as a subject of request",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"PortForward": {
		Description: "is emitted when a user requests port forwarding",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "addr",
				Description: "is a target port forwarding address",
				Type:        "string",
			},
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "error",
				Description: "includes system error message for the failed attempt",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "message",
				Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
				Type:        "string",
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "success",
				Description: "indicates the success or failure of the operation",
				Type:        "boolean",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"PostgresBind": {
		Description: "is emitted when a Postgres client readies a prepared statement for execution and binds it to parameters",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "parameters",
				Description: "are the query bind parameters",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "portal_name",
				Description: "is the destination portal name that binds statement to parameters",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "statement_name",
				Description: "is the name of prepared statement that's being bound to parameters",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"PostgresClose": {
		Description: "is emitted when a Postgres client closes an existing prepared statement",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "portal_name",
				Description: "is the name of destination portal that's being closed",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "statement_name",
				Description: "is the name of prepared statement that's being closed",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"PostgresExecute": {
		Description: "is emitted when a Postgres client executes a previously bound prepared statement",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "portal_name",
				Description: "is the name of destination portal that's being executed",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"PostgresFunctionCall": {
		Description: "is emitted when a Postgres client calls internal database function",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "function_args",
				Description: "contains formatted function arguments",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "function_oid",
				Description: "is the Postgres object ID of the called function",
				Type:        "integer",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"PostgresParse": {
		Description: "is emitted when a Postgres client creates a prepared statement using extended query protocol",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "query",
				Description: "is the prepared statement query",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "statement_name",
				Description: "is the prepared statement name",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"ProvisionTokenCreate": {
		Description: "event is emitted when a provisioning token (a.k.a. join token) of any role is created",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name: "join_method",
				Type: "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name: "roles",
				Type: "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"RecoveryCodeGenerate": {
		Description: "is emitted when a user's new recovery codes are generated and updated",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"RecoveryCodeUsed": {
		Description: "is emitted when a user's recovery code was used successfully or unsuccessfully",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "error",
				Description: "includes system error message for the failed attempt",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "message",
				Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "success",
				Description: "indicates the success or failure of the operation",
				Type:        "boolean",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"RenewableCertificateGenerationMismatch": {
		Description: "is emitted when a renewable certificate's generation counter fails to validate, possibly indicating a stolen certificate and an invalid renewal attempt",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"Resize": {
		Description: "means that some user resized PTY on the client",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "forwarded_by",
				Description: "tells us if the metadata was sent by the node itself or by another node in it's place. We can't verify emit permissions fully for these events so care should be taken with them",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "kubernetes_cluster",
				Description: "is a kubernetes cluster name",
				Type:        "string",
			},
			{
				Name:        "kubernetes_container_image",
				Description: "is the image of the container within the pod",
				Type:        "string",
			},
			{
				Name:        "kubernetes_container_name",
				Description: "is the name of the container within the pod",
				Type:        "string",
			},
			{
				Name:        "kubernetes_groups",
				Description: "is a list of kubernetes groups for the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "kubernetes_labels",
				Description: "are the labels (static and dynamic) of the kubernetes cluster the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "kubernetes_node_name",
				Description: "is the node that runs the pod",
				Type:        "string",
			},
			{
				Name:        "kubernetes_pod_name",
				Description: "is the name of the pod",
				Type:        "string",
			},
			{
				Name:        "kubernetes_pod_namespace",
				Description: "is the namespace of the pod",
				Type:        "string",
			},
			{
				Name:        "kubernetes_users",
				Description: "is a list of kubernetes usernames for the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "namespace",
				Description: "is a namespace of the server event",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "server_addr",
				Description: "is the address of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_hostname",
				Description: "is the hostname of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_id",
				Description: "is the UUID of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_labels",
				Description: "are the labels (static and dynamic) of the server the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "server_sub_kind",
				Description: "is the sub kind of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "size",
				Description: "is expressed as 'W:H'",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"RoleCreate": {
		Description: "is emitted when a role is created/updated",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"RoleDelete": {
		Description: "is emitted when a role is deleted",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"RoleUpdate": {
		Description: "is emitted when a role is updated",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"SAMLConnectorCreate": {
		Description: "fires when SAML connector is created/updated",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"SAMLConnectorDelete": {
		Description: "fires when SAML connector is deleted",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"SAMLConnectorUpdate": {
		Description: "fires when SAML connector is updated",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"SAMLIdPAuthAttempt": {
		Description: "is emitted when a user has attempted to authorize against the SAML IdP",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "error",
				Description: "includes system error message for the failed attempt",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "message",
				Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "service_provider_entity_id",
				Description: "is the entity ID of the service provider",
				Type:        "string",
			},
			{
				Name:        "service_provider_shortcut",
				Description: "is the shortcut name of a service provider",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "success",
				Description: "indicates the success or failure of the operation",
				Type:        "boolean",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"SAMLIdPServiceProviderCreate": {
		Description: "is emitted when a service provider has been added",
		Fields: []*EventField{
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "service_provider_entity_id",
				Description: "is the entity ID of the service provider",
				Type:        "string",
			},
			{
				Name:        "service_provider_shortcut",
				Description: "is the shortcut name of a service provider",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
		},
	},
	"SAMLIdPServiceProviderDelete": {
		Description: "is emitted when a service provider has been deleted",
		Fields: []*EventField{
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "service_provider_entity_id",
				Description: "is the entity ID of the service provider",
				Type:        "string",
			},
			{
				Name:        "service_provider_shortcut",
				Description: "is the shortcut name of a service provider",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
		},
	},
	"SAMLIdPServiceProviderDeleteAll": {
		Description: "is emitted when all service providers have been deleted",
		Fields: []*EventField{
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
		},
	},
	"SAMLIdPServiceProviderUpdate": {
		Description: "is emitted when a service provider has been updated",
		Fields: []*EventField{
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "service_provider_entity_id",
				Description: "is the entity ID of the service provider",
				Type:        "string",
			},
			{
				Name:        "service_provider_shortcut",
				Description: "is the shortcut name of a service provider",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
		},
	},
	"SCP": {
		Description: "is emitted when data transfer has occurred between server and client",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "action",
				Description: "is upload or download",
				Type:        "string",
			},
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "command",
				Description: "is the executed command name",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "exitCode",
				Description: "specifies command exit code",
				Type:        "string",
			},
			{
				Name:        "exitError",
				Description: "is an optional exit error, set if command has failed",
				Type:        "string",
			},
			{
				Name:        "forwarded_by",
				Description: "tells us if the metadata was sent by the node itself or by another node in it's place. We can't verify emit permissions fully for these events so care should be taken with them",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "namespace",
				Description: "is a namespace of the server event",
				Type:        "string",
			},
			{
				Name:        "path",
				Description: "is a copy path",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "server_addr",
				Description: "is the address of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_hostname",
				Description: "is the hostname of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_id",
				Description: "is the UUID of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_labels",
				Description: "are the labels (static and dynamic) of the server the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "server_sub_kind",
				Description: "is the sub kind of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"SFTP": {
		Description: "is emitted when file operations have occurred between server and client",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "action",
				Description: "is what kind of file operation",
				Type:        "integer",
			},
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "attributes",
				Description: "is file metadata that the user requested to be changed",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "access_time",
						Description: "is when the file was last read",
						Type:        "string",
					},
					{
						Name:        "file_size",
						Description: "is file size",
						Type:        "object",
						Fields: []*EventField{
							{
								Name:        "value",
								Description: "uint64 value",
								Type:        "integer",
							},
						},
					},
					{
						Name:        "gid",
						Description: "is the group owner of the file",
						Type:        "object",
						Fields: []*EventField{
							{
								Name:        "value",
								Description: "uint32 value",
								Type:        "integer",
							},
						},
					},
					{
						Name:        "modification_time",
						Description: "was when the file was last changed",
						Type:        "string",
					},
					{
						Name:        "permissions",
						Description: "is the file permissions",
						Type:        "object",
						Fields: []*EventField{
							{
								Name:        "value",
								Description: "uint32 value",
								Type:        "integer",
							},
						},
					},
					{
						Name:        "uid",
						Description: "is the user owner of a file",
						Type:        "object",
						Fields: []*EventField{
							{
								Name:        "value",
								Description: "uint32 value",
								Type:        "integer",
							},
						},
					},
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "error",
				Description: "is the optional error that may have occurred",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "flags",
				Description: "is options that were passed that affect file creation events",
				Type:        "integer",
			},
			{
				Name:        "forwarded_by",
				Description: "tells us if the metadata was sent by the node itself or by another node in it's place. We can't verify emit permissions fully for these events so care should be taken with them",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "namespace",
				Description: "is a namespace of the server event",
				Type:        "string",
			},
			{
				Name:        "path",
				Description: "is the filepath that was operated on. It is the exact path that was sent by the client, so it may be relative or absolute",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "server_addr",
				Description: "is the address of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_hostname",
				Description: "is the hostname of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_id",
				Description: "is the UUID of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_labels",
				Description: "are the labels (static and dynamic) of the server the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "server_sub_kind",
				Description: "is the sub kind of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "target_path",
				Description: "is the new path in file renames, or the path of the symlink when creating symlinks. It is the exact path that wassent by the client, so it may be relative or absolute",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
			{
				Name:        "working_directory",
				Description: "is the current directory the SFTP server is in",
				Type:        "string",
			},
		},
	},
	"SQLServerRPCRequest": {
		Description: "is emitted when a user executes a MSSQL Server RPC command",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "db_aws_redshift_cluster_id",
				Description: "is cluster ID for Redshift databases",
				Type:        "string",
			},
			{
				Name:        "db_aws_region",
				Description: "is AWS regions for AWS hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_instance_id",
				Description: "is instance ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_gcp_project_id",
				Description: "is project ID for GCP hosted databases",
				Type:        "string",
			},
			{
				Name:        "db_labels",
				Description: "is the database resource labels",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "db_name",
				Description: "is the name of the database a user is connecting to",
				Type:        "string",
			},
			{
				Name:        "db_origin",
				Description: "is the database origin source",
				Type:        "string",
			},
			{
				Name:        "db_protocol",
				Description: "is the database type, e.g. postgres or mysql",
				Type:        "string",
			},
			{
				Name:        "db_roles",
				Description: "is a list of database roles for auto-provisioned users",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "db_service",
				Description: "is the name of the database service proxying the database",
				Type:        "string",
			},
			{
				Name:        "db_type",
				Description: "is the database type",
				Type:        "string",
			},
			{
				Name:        "db_uri",
				Description: "is the database URI to connect to",
				Type:        "string",
			},
			{
				Name:        "db_user",
				Description: "is the database username used to connect",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "parameters",
				Description: "are the RPC parameters used to execute RPC Procedure",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "proc_name",
				Description: "is the RPC SQL Server procedure name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"SSMRun": {
		Description: "is emitted after an AWS SSM document completes execution",
		Fields: []*EventField{
			{
				Name:        "account_id",
				Description: "is the id of the AWS account that ran the command",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "command_id",
				Description: "is the id of the SSM command that was run",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "exit_code",
				Description: "is the exit code resulting from the script run",
				Type:        "integer",
			},
			{
				Name:        "instance_id",
				Description: "is the id of the EC2 instance the command was run on",
				Type:        "string",
			},
			{
				Name:        "region",
				Description: "is the AWS region the command was ran in",
				Type:        "string",
			},
			{
				Name:        "status",
				Description: "represents the success or failure status of a script run",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
		},
	},
	"SecurityReportRun": {
		Description: "is emitted when a user runs an audit query",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "audit_queries",
				Description: "is the list of audit queries that were run",
				Type:        "array",
				Items: &EventField{
					Type: "object",
					Fields: []*EventField{
						{
							Name:        "data_scanned_in_bytes",
							Description: "is the amount of data scanned by the query",
							Type:        "integer",
						},
						{
							Name:        "days",
							Description: "is the number of days time range for the query",
							Type:        "integer",
						},
						{
							Name:        "name",
							Description: "is the name of the query",
							Type:        "string",
						},
						{
							Name:        "query",
							Description: "is the query that was run",
							Type:        "string",
						},
						{
							Name:        "total_execution_time_in_millis",
							Description: "is the total execution time of the query",
							Type:        "integer",
						},
					},
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "error",
				Description: "includes system error message for the failed attempt",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "message",
				Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is the query that was run",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "success",
				Description: "indicates the success or failure of the operation",
				Type:        "boolean",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "total_data_scanned_in_bytes",
				Description: "is the amount of data scanned by the query",
				Type:        "integer",
			},
			{
				Name:        "total_execution_time_in_millis",
				Description: "is the total execution time of the query",
				Type:        "integer",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "version",
				Description: "is the version of security report",
				Type:        "string",
			},
		},
	},
	"SessionCommand": {
		Description: "is a session command event",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "argv",
				Description: "is the list of arguments to the program. Note, the first element does not contain the name of the process",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cgroup_id",
				Description: "is the internal cgroupv2 ID of the event",
				Type:        "integer",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "forwarded_by",
				Description: "tells us if the metadata was sent by the node itself or by another node in it's place. We can't verify emit permissions fully for these events so care should be taken with them",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "namespace",
				Description: "is a namespace of the server event",
				Type:        "string",
			},
			{
				Name:        "path",
				Description: "is the full path to the executable",
				Type:        "string",
			},
			{
				Name:        "pid",
				Description: "is the ID of the process",
				Type:        "integer",
			},
			{
				Name:        "ppid",
				Description: "is the PID of the parent process",
				Type:        "integer",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "program",
				Description: "is name of the executable",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "return_code",
				Description: "is the return code of execve",
				Type:        "integer",
			},
			{
				Name:        "server_addr",
				Description: "is the address of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_hostname",
				Description: "is the hostname of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_id",
				Description: "is the UUID of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_labels",
				Description: "are the labels (static and dynamic) of the server the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "server_sub_kind",
				Description: "is the sub kind of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"SessionConnect": {
		Description: "is emitted when a non-Teleport connection is made over net.Dial",
		Fields: []*EventField{
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "forwarded_by",
				Description: "tells us if the metadata was sent by the node itself or by another node in it's place. We can't verify emit permissions fully for these events so care should be taken with them",
				Type:        "string",
			},
			{
				Name:        "namespace",
				Description: "is a namespace of the server event",
				Type:        "string",
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "server_addr",
				Description: "is the address of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_hostname",
				Description: "is the hostname of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_id",
				Description: "is the UUID of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_labels",
				Description: "are the labels (static and dynamic) of the server the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "server_sub_kind",
				Description: "is the sub kind of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
		},
	},
	"SessionData": {
		Description: "is emitted to report session data usage",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "forwarded_by",
				Description: "tells us if the metadata was sent by the node itself or by another node in it's place. We can't verify emit permissions fully for these events so care should be taken with them",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "namespace",
				Description: "is a namespace of the server event",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "rx",
				Description: "is the amount of bytes received",
				Type:        "integer",
			},
			{
				Name:        "server_addr",
				Description: "is the address of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_hostname",
				Description: "is the hostname of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_id",
				Description: "is the UUID of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_labels",
				Description: "are the labels (static and dynamic) of the server the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "server_sub_kind",
				Description: "is the sub kind of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "tx",
				Description: "is the amount of bytes transmitted",
				Type:        "integer",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"SessionDisk": {
		Description: "is a session disk access event",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cgroup_id",
				Description: "is the internal cgroupv2 ID of the event",
				Type:        "integer",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "flags",
				Description: "are the flags passed to open",
				Type:        "integer",
			},
			{
				Name:        "forwarded_by",
				Description: "tells us if the metadata was sent by the node itself or by another node in it's place. We can't verify emit permissions fully for these events so care should be taken with them",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "namespace",
				Description: "is a namespace of the server event",
				Type:        "string",
			},
			{
				Name:        "path",
				Description: "is the full path to the executable",
				Type:        "string",
			},
			{
				Name:        "pid",
				Description: "is the ID of the process",
				Type:        "integer",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "program",
				Description: "is name of the executable",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "return_code",
				Description: "is the return code of disk open",
				Type:        "integer",
			},
			{
				Name:        "server_addr",
				Description: "is the address of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_hostname",
				Description: "is the hostname of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_id",
				Description: "is the UUID of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_labels",
				Description: "are the labels (static and dynamic) of the server the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "server_sub_kind",
				Description: "is the sub kind of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"SessionEnd": {
		Description: "is a session end event",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "enhanced_recording",
				Description: "is used to indicate if the recording was an enhanced recording or not",
				Type:        "boolean",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "forwarded_by",
				Description: "tells us if the metadata was sent by the node itself or by another node in it's place. We can't verify emit permissions fully for these events so care should be taken with them",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "initial_command",
				Description: "is the command used to start this session",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "interactive",
				Description: "is used to indicate if the session was interactive (has PTY attached) or not (exec session)",
				Type:        "boolean",
			},
			{
				Name:        "kubernetes_cluster",
				Description: "is a kubernetes cluster name",
				Type:        "string",
			},
			{
				Name:        "kubernetes_container_image",
				Description: "is the image of the container within the pod",
				Type:        "string",
			},
			{
				Name:        "kubernetes_container_name",
				Description: "is the name of the container within the pod",
				Type:        "string",
			},
			{
				Name:        "kubernetes_groups",
				Description: "is a list of kubernetes groups for the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "kubernetes_labels",
				Description: "are the labels (static and dynamic) of the kubernetes cluster the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "kubernetes_node_name",
				Description: "is the node that runs the pod",
				Type:        "string",
			},
			{
				Name:        "kubernetes_pod_name",
				Description: "is the name of the pod",
				Type:        "string",
			},
			{
				Name:        "kubernetes_pod_namespace",
				Description: "is the namespace of the pod",
				Type:        "string",
			},
			{
				Name:        "kubernetes_users",
				Description: "is a list of kubernetes usernames for the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "namespace",
				Description: "is a namespace of the server event",
				Type:        "string",
			},
			{
				Name:        "participants",
				Description: "is a list of participants in the session",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "server_addr",
				Description: "is the address of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_hostname",
				Description: "is the hostname of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_id",
				Description: "is the UUID of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_labels",
				Description: "are the labels (static and dynamic) of the server the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "server_sub_kind",
				Description: "is the sub kind of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "session_recording",
				Description: "is the type of session recording",
				Type:        "string",
			},
			{
				Name:        "session_start",
				Description: "is the timestamp at which the session began",
				Type:        "string",
			},
			{
				Name:        "session_stop",
				Description: "is the timestamp at which the session ended",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"SessionJoin": {
		Description: "emitted when another user joins a session",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "forwarded_by",
				Description: "tells us if the metadata was sent by the node itself or by another node in it's place. We can't verify emit permissions fully for these events so care should be taken with them",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "kubernetes_cluster",
				Description: "is a kubernetes cluster name",
				Type:        "string",
			},
			{
				Name:        "kubernetes_groups",
				Description: "is a list of kubernetes groups for the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "kubernetes_labels",
				Description: "are the labels (static and dynamic) of the kubernetes cluster the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "kubernetes_users",
				Description: "is a list of kubernetes usernames for the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "namespace",
				Description: "is a namespace of the server event",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "server_addr",
				Description: "is the address of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_hostname",
				Description: "is the hostname of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_id",
				Description: "is the UUID of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_labels",
				Description: "are the labels (static and dynamic) of the server the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "server_sub_kind",
				Description: "is the sub kind of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"SessionLeave": {
		Description: "is emitted to report that a user left the session",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "forwarded_by",
				Description: "tells us if the metadata was sent by the node itself or by another node in it's place. We can't verify emit permissions fully for these events so care should be taken with them",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "namespace",
				Description: "is a namespace of the server event",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "server_addr",
				Description: "is the address of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_hostname",
				Description: "is the hostname of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_id",
				Description: "is the UUID of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_labels",
				Description: "are the labels (static and dynamic) of the server the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "server_sub_kind",
				Description: "is the sub kind of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"SessionNetwork": {
		Description: "is a network event",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "action",
				Description: "denotes what happened in response to the event",
				Type:        "integer",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cgroup_id",
				Description: "is the internal cgroupv2 ID of the event",
				Type:        "integer",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "dst_addr",
				Description: "is the destination IP address of the connection",
				Type:        "string",
			},
			{
				Name:        "dst_port",
				Description: "is the destination port of the connection",
				Type:        "integer",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "forwarded_by",
				Description: "tells us if the metadata was sent by the node itself or by another node in it's place. We can't verify emit permissions fully for these events so care should be taken with them",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "namespace",
				Description: "is a namespace of the server event",
				Type:        "string",
			},
			{
				Name:        "operation",
				Description: "denotes what network operation was performed (e.g. connect)",
				Type:        "integer",
			},
			{
				Name:        "pid",
				Description: "is the ID of the process",
				Type:        "integer",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "program",
				Description: "is name of the executable",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "server_addr",
				Description: "is the address of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_hostname",
				Description: "is the hostname of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_id",
				Description: "is the UUID of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_labels",
				Description: "are the labels (static and dynamic) of the server the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "server_sub_kind",
				Description: "is the sub kind of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "src_addr",
				Description: "is the source IP address of the connection",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "version",
				Description: "is the version of TCP (4 or 6)",
				Type:        "integer",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"SessionPrint": {
		Description: "event happens every time a write occurs to terminal I/O during a session",
		Fields: []*EventField{
			{
				Name:        "bytes",
				Description: "says how many bytes have been written into the session during \"print\" event",
				Type:        "integer",
			},
			{
				Name:        "ci",
				Description: "is a monotonically incremented index for ordering print events",
				Type:        "integer",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "ms",
				Description: "is the delay in milliseconds from the start of the session",
				Type:        "integer",
			},
			{
				Name:        "offset",
				Description: "is the offset in bytes in the session file",
				Type:        "integer",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
		},
	},
	"SessionRecordingAccess": {
		Description: "is emitted when a session recording is accessed, allowing session views to be included in the audit log",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is the ID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"SessionReject": {
		Description: "event happens when a user hits a session control restriction",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "forwarded_by",
				Description: "tells us if the metadata was sent by the node itself or by another node in it's place. We can't verify emit permissions fully for these events so care should be taken with them",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "max",
				Description: "is an event field specifying a maximal value (e.g. the value of `max_connections` for a `session.rejected` event)",
				Type:        "integer",
			},
			{
				Name:        "namespace",
				Description: "is a namespace of the server event",
				Type:        "string",
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "reason",
				Description: "is a field that specifies reason for event, e.g. in disconnect event it explains why server disconnected the client",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "server_addr",
				Description: "is the address of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_hostname",
				Description: "is the hostname of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_id",
				Description: "is the UUID of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_labels",
				Description: "are the labels (static and dynamic) of the server the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "server_sub_kind",
				Description: "is the sub kind of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"SessionStart": {
		Description: "is a session start event",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "forwarded_by",
				Description: "tells us if the metadata was sent by the node itself or by another node in it's place. We can't verify emit permissions fully for these events so care should be taken with them",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "initial_command",
				Description: "is the command used to start this session",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "kubernetes_cluster",
				Description: "is a kubernetes cluster name",
				Type:        "string",
			},
			{
				Name:        "kubernetes_container_image",
				Description: "is the image of the container within the pod",
				Type:        "string",
			},
			{
				Name:        "kubernetes_container_name",
				Description: "is the name of the container within the pod",
				Type:        "string",
			},
			{
				Name:        "kubernetes_groups",
				Description: "is a list of kubernetes groups for the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "kubernetes_labels",
				Description: "are the labels (static and dynamic) of the kubernetes cluster the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "kubernetes_node_name",
				Description: "is the node that runs the pod",
				Type:        "string",
			},
			{
				Name:        "kubernetes_pod_name",
				Description: "is the name of the pod",
				Type:        "string",
			},
			{
				Name:        "kubernetes_pod_namespace",
				Description: "is the namespace of the pod",
				Type:        "string",
			},
			{
				Name:        "kubernetes_users",
				Description: "is a list of kubernetes usernames for the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "namespace",
				Description: "is a namespace of the server event",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "server_addr",
				Description: "is the address of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_hostname",
				Description: "is the hostname of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_id",
				Description: "is the UUID of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "server_labels",
				Description: "are the labels (static and dynamic) of the server the session occurred on",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "server_sub_kind",
				Description: "is the sub kind of the server the session occurred on",
				Type:        "string",
			},
			{
				Name:        "session_recording",
				Description: "is the type of session recording",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "size",
				Description: "is expressed as 'W:H'",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"SessionUpload": {
		Description: "is a session upload",
		Fields: []*EventField{
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "url",
				Description: "is where the url the session event data upload is at",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"Subsystem": {
		Description: "is emitted when a user requests a new subsystem",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "exitError",
				Description: "contains error in case of unsucessfull attempt",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a subsystem name",
				Type:        "string",
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"TrustedClusterCreate": {
		Description: "is the event for creating a trusted cluster",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"TrustedClusterDelete": {
		Description: "is the event for removing a trusted cluster",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"TrustedClusterTokenCreate": {
		Description: "event is emitted (in addition to ProvisionTokenCreate) when a token of a \"Trusted_cluster\" role is created.  Deprecated: redundant, since we also emit ProvisionTokenCreate",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"Unknown": {
		Description: "is a fallback event used when we don't recognize an event from the backend",
		Fields: []*EventField{
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "data",
				Description: "is the serialized JSON data of the unknown event",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "unknown_code",
				Description: "is the event code extracted from the unknown event",
				Type:        "string",
			},
			{
				Name:        "unknown_event",
				Description: "is the event type extracted from the unknown event",
				Type:        "string",
			},
		},
	},
	"UpgradeWindowStartUpdate": {
		Description: "is emitted when a user updates the cloud upgrade window start time",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "upgrade_window_start",
				Description: "is the upgrade window time",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"UserCreate": {
		Description: "is emitted when the user is created or upserted",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "connector",
				Description: "is the connector used to create the user",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "roles",
				Description: "is a list of roles for the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"UserDelete": {
		Description: "is emitted when a user gets deleted",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"UserLogin": {
		Description: "records a successfully or failed user login event",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "applied_login_rules",
				Description: "stores the name of each login rule that was applied during the login",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "attributes",
				Description: "is a map of user attributes received from identity provider",
				Type:        "object",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "error",
				Description: "includes system error message for the failed attempt",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "message",
				Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
				Type:        "string",
			},
			{
				Name:        "method",
				Description: "is the event field indicating how the login was performed",
				Type:        "string",
			},
			{
				Name:        "mfa_device",
				Description: "is the MFA device used during the login",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "mfa_device_name",
						Description: "is the user-specified name of the MFA device",
						Type:        "string",
					},
					{
						Name:        "mfa_device_type",
						Description: "is the type of this MFA device",
						Type:        "string",
					},
					{
						Name:        "mfa_device_uuid",
						Description: "is the UUID of the MFA device generated by Teleport",
						Type:        "string",
					},
				},
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "success",
				Description: "indicates the success or failure of the operation",
				Type:        "boolean",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "user_agent",
				Description: "identifies the type of client that attempted the event",
				Type:        "string",
			},
		},
	},
	"UserPasswordChange": {
		Description: "is emitted when the user changes their own password",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"UserTokenCreate": {
		Description: "is emitted when a user token is created",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"UserUpdate": {
		Description: "is emitted when the user is updated",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "connector",
				Description: "is the connector used to create the user",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "expires",
				Description: "is set if resource expires",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "name",
				Description: "is a resource name",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "roles",
				Description: "is a list of roles for the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "ttl",
				Description: "is a TTL of reset password token represented as duration, e.g. \"10m\" used for compatibility purposes for some events, Expires should be used instead as it's more useful (contains exact expiration date/time)",
				Type:        "string",
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "updated_by",
				Description: "if set indicates the user who modified the resource",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
	"WindowsDesktopSessionEnd": {
		Description: "is emitted when a user ends a Windows desktop session",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "desktop_addr",
				Description: "is the address of the desktop being accessed",
				Type:        "string",
			},
			{
				Name:        "desktop_labels",
				Description: "are the labels on the desktop resource",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "desktop_name",
				Description: "is the name of the desktop resource",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "participants",
				Description: "is a list of participants in the session",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "recorded",
				Description: "is true if the session was recorded, false otherwise",
				Type:        "boolean",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "session_start",
				Description: "is the timestamp at which the session began",
				Type:        "string",
			},
			{
				Name:        "session_stop",
				Description: "is the timestamp at which the session ended",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "windows_desktop_service",
				Description: "is the name of the service proxying the RDP session",
				Type:        "string",
			},
			{
				Name:        "windows_domain",
				Description: "is the Active Directory domain of the desktop being accessed",
				Type:        "string",
			},
			{
				Name:        "windows_user",
				Description: "is the Windows username used to connect",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"WindowsDesktopSessionStart": {
		Description: "is emitted when a user connects to a desktop",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "allow_user_creation",
				Description: "indicates whether automatic local user creation is allowed for this session",
				Type:        "boolean",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "desktop_addr",
				Description: "is the address of the desktop being accessed",
				Type:        "string",
			},
			{
				Name:        "desktop_labels",
				Description: "are the labels on the desktop resource",
				Type:        "object",
				Fields: []*EventField{
					{
						Name: "key",
						Type: "string",
					},
					{
						Name: "value",
						Type: "string",
					},
				},
			},
			{
				Name:        "desktop_name",
				Description: "is the name of the desktop resource",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "error",
				Description: "includes system error message for the failed attempt",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "message",
				Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
				Type:        "string",
			},
			{
				Name:        "private_key_policy",
				Description: "is the private key policy of the private key used to start this session",
				Type:        "string",
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "sid",
				Description: "is a unique UUID of the session",
				Type:        "string",
			},
			{
				Name:        "success",
				Description: "indicates the success or failure of the operation",
				Type:        "boolean",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
			{
				Name:        "windows_desktop_service",
				Description: "is the name of the service proxying the RDP session",
				Type:        "string",
			},
			{
				Name:        "windows_domain",
				Description: "is the Active Directory domain of the desktop being accessed",
				Type:        "string",
			},
			{
				Name:        "windows_user",
				Description: "is the Windows username used to connect",
				Type:        "string",
			},
			{
				Name:        "with_mfa",
				Description: "is a UUID of an MFA device used to start this session",
				Type:        "string",
			},
		},
	},
	"X11Forward": {
		Description: "is emitted when a user requests X11 protocol forwarding",
		Fields: []*EventField{
			{
				Name:        "access_requests",
				Description: "are the IDs of access requests created by the user",
				Type:        "array",
				Items: &EventField{
					Type: "string",
				},
			},
			{
				Name:        "addr.local",
				Description: "is a target address on the host",
				Type:        "string",
			},
			{
				Name:        "addr.remote",
				Description: "is a client (user's) address",
				Type:        "string",
			},
			{
				Name:        "aws_role_arn",
				Description: "is AWS IAM role user assumes when accessing AWS console",
				Type:        "string",
			},
			{
				Name:        "azure_identity",
				Description: "is the Azure identity user assumes when accessing Azure API",
				Type:        "string",
			},
			{
				Name:        "cluster_name",
				Description: "identifies the originating teleport cluster",
				Type:        "string",
			},
			{
				Name:        "code",
				Description: "is a unique event code",
				Type:        "string",
			},
			{
				Name:        "ei",
				Description: "is a monotonically incremented index in the event sequence",
				Type:        "integer",
			},
			{
				Name:        "error",
				Description: "includes system error message for the failed attempt",
				Type:        "string",
			},
			{
				Name:        "event",
				Description: "is the event type",
				Type:        "string",
			},
			{
				Name:        "gcp_service_account",
				Description: "is the GCP service account user assumes when accessing GCP API",
				Type:        "string",
			},
			{
				Name:        "impersonator",
				Description: "is a user acting on behalf of another user",
				Type:        "string",
			},
			{
				Name:        "login",
				Description: "is OS login",
				Type:        "string",
			},
			{
				Name:        "message",
				Description: "is a user-friendly message for successfull or unsuccessfull auth attempt",
				Type:        "string",
			},
			{
				Name:        "proto",
				Description: "specifies protocol that was captured",
				Type:        "string",
			},
			{
				Name:        "required_private_key_policy",
				Description: "is the private key policy enforced for this login",
				Type:        "string",
			},
			{
				Name:        "success",
				Description: "indicates the success or failure of the operation",
				Type:        "boolean",
			},
			{
				Name:        "time",
				Description: "is event time",
				Type:        "string",
			},
			{
				Name:        "trusted_device",
				Description: "contains information about the users' trusted device. Requires a registered and enrolled device to be used during authentication",
				Type:        "object",
				Fields: []*EventField{
					{
						Name:        "asset_tag",
						Description: "inventory identifier",
						Type:        "string",
					},
					{
						Name:        "credential_id",
						Description: "credential identifier",
						Type:        "string",
					},
					{
						Name:        "device_id",
						Description: "of the device",
						Type:        "string",
					},
					{
						Name:        "device_origin",
						Description: "origin",
						Type:        "integer",
					},
					{
						Name:        "os_type",
						Description: "of the device",
						Type:        "integer",
					},
				},
			},
			{
				Name:        "uid",
				Description: "is a unique event identifier",
				Type:        "string",
			},
			{
				Name:        "user",
				Description: "is teleport user name",
				Type:        "string",
			},
		},
	},
}
