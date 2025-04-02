package app

import (
	"testing"

	gogotypes "github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
)

const (
	requestExample = `{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05",
    "capabilities": {
      "roots": {
        "listChanged": true
      },
      "sampling": {}
    },
    "clientInfo": {
      "name": "ExampleClient",
      "version": "1.0.0"
    }
  }
}`

	notificationExample = `{
  "jsonrpc": "2.0",
  "method": "notifications/initialized"
}`
)

func TestMCPMessageToEvent(t *testing.T) {
	tests := []struct {
		desc      string
		input     string
		wantEvent apievents.AuditEvent
	}{
		{
			desc:  "request",
			input: requestExample,
			wantEvent: &apievents.AppSessionMCPRequest{
				Metadata: apievents.Metadata{
					Type: events.AppSessionMCPRequestEvent,
					Code: events.AppSessionMCPRequestCode,
				},
				JSONRPC:   "2.0",
				RPCMethod: "initialize",
				RPCID:     "1",
				RPCParams: &apievents.Struct{
					Struct: gogotypes.Struct{
						Fields: map[string]*gogotypes.Value{
							"protocolVersion": {
								Kind: &gogotypes.Value_StringValue{
									StringValue: "2024-11-05",
								},
							},
							"capabilities": {
								Kind: &gogotypes.Value_StructValue{
									StructValue: &gogotypes.Struct{
										Fields: map[string]*gogotypes.Value{
											"roots": {
												Kind: &gogotypes.Value_StructValue{
													StructValue: &gogotypes.Struct{
														Fields: map[string]*gogotypes.Value{
															"listChanged": {
																Kind: &gogotypes.Value_BoolValue{
																	BoolValue: true,
																},
															},
														},
													},
												},
											},
											"sampling": {
												Kind: &gogotypes.Value_StructValue{
													StructValue: &gogotypes.Struct{
														Fields: map[string]*gogotypes.Value{},
													},
												},
											},
										},
									},
								},
							},
							"clientInfo": {
								Kind: &gogotypes.Value_StructValue{
									StructValue: &gogotypes.Struct{
										Fields: map[string]*gogotypes.Value{
											"name": {
												Kind: &gogotypes.Value_StringValue{
													StringValue: "ExampleClient",
												},
											},
											"version": {
												Kind: &gogotypes.Value_StringValue{
													StringValue: "1.0.0",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			desc:  "notification",
			input: notificationExample,
			wantEvent: &apievents.AppSessionMCPNotification{
				Metadata: apievents.Metadata{
					Type: events.AppSessionMCPNotificationEvent,
					Code: events.AppSessionMCPNotificationCode,
				},
				JSONRPC:   "2.0",
				RPCMethod: "notifications/initialized",
				RPCParams: nil,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			event, _, err := mcpMessageToEvent(test.input, apievents.UserMetadata{}, apievents.SessionMetadata{})
			require.NoError(t, err)
			switch wantEvent := test.wantEvent.(type) {
			case *apievents.AppSessionMCPRequest:
				require.IsType(t, wantEvent, event)
				gotRequest := event.(*apievents.AppSessionMCPRequest)
				require.Equal(t, wantEvent.JSONRPC, gotRequest.JSONRPC)
				require.Equal(t, wantEvent.RPCID, gotRequest.RPCID)
				require.Equal(t, wantEvent.RPCMethod, gotRequest.RPCMethod)
				require.Equal(t, wantEvent.RPCParams, gotRequest.RPCParams)
			case *apievents.AppSessionMCPNotification:
				require.IsType(t, wantEvent, event)
				gotNotification := event.(*apievents.AppSessionMCPNotification)
				require.Equal(t, wantEvent.JSONRPC, gotNotification.JSONRPC)
				require.Equal(t, wantEvent.RPCMethod, gotNotification.RPCMethod)
				require.Equal(t, wantEvent.RPCParams, gotNotification.RPCParams)
			default:
				require.FailNow(t, "unexpected audit event type %T", wantEvent)
			}
		})
	}
}
