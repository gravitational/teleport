/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package aws

import (
	"encoding/xml"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/private/protocol"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/stretchr/testify/require"
)

func TestIsXMLOfLocalName(t *testing.T) {
	data := `<MyXMLName xmlns="my-space"><MyNode><Value>5</Value></MyNode></MyXMLName>`
	require.True(t, IsXMLOfLocalName([]byte(data), "MyXMLName"))
	require.False(t, IsXMLOfLocalName([]byte(data), "SomeOtherName"))
	require.False(t, IsXMLOfLocalName([]byte("<bad-xml"+data), "MyXMLName"))
}

func TestUnmarshalXMLChildNode(t *testing.T) {
	want := sts.AssumeRoleOutput{
		AssumedRoleUser: &sts.AssumedRoleUser{
			Arn: aws.String("some-arn"),
		},
		Credentials: &sts.Credentials{
			AccessKeyId:     aws.String("some-access-key-id"),
			SecretAccessKey: aws.String("some-secret-access-key"),
			SessionToken:    aws.String("some-session-token"),
			Expiration:      aws.Time(time.Unix(1234567890, 0).UTC()),
		},
	}

	body := []byte(`<AssumeRoleResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <AssumeRoleResult>
    <Credentials>
      <SecretAccessKey>some-secret-access-key</SecretAccessKey>
      <SessionToken>some-session-token</SessionToken>
      <AccessKeyId>some-access-key-id</AccessKeyId>
      <Expiration>2009-02-13T23:31:30Z</Expiration>
    </Credentials>
    <AssumedRoleUser>
      <Arn>some-arn</Arn>
    </AssumedRoleUser>
  </AssumeRoleResult>
  <ResponseMetadata>
    <StatusCode>200</StatusCode>
    <RequestID>some-request-id</RequestID>
  </ResponseMetadata>
</AssumeRoleResponse>`)

	var actual sts.AssumeRoleOutput
	require.NoError(t, UnmarshalXMLChildNode(&actual, body, "AssumeRoleResult"))
	require.Equal(t, want, actual)
}

func TestMarshalXMLIndent(t *testing.T) {
	data, err := MarshalXML(
		xml.Name{
			Local: "AssumeRoleResponse",
			Space: "https://sts.amazonaws.com/doc/2011-06-15/",
		},
		map[string]any{
			"AssumeRoleResult": sts.AssumeRoleOutput{
				AssumedRoleUser: &sts.AssumedRoleUser{
					Arn: aws.String("some-arn"),
				},
				Credentials: &sts.Credentials{
					AccessKeyId:     aws.String("some-access-key-id"),
					SecretAccessKey: aws.String("some-secret-access-key"),
					SessionToken:    aws.String("some-session-token"),
					Expiration:      aws.Time(time.Unix(1234567890, 0).UTC()),
				},
			},
			"ResponseMetadata": protocol.ResponseMetadata{
				RequestID:  "some-request-id",
				StatusCode: http.StatusOK,
			},
		},
	)
	require.NoError(t, err)

	// Nodes are not sorted. Use ElementsMatch to ensure each line is present.
	require.ElementsMatch(t, []string{
		`<AssumeRoleResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">`,
		`  <AssumeRoleResult>`,
		`    <Credentials>`,
		`      <SecretAccessKey>some-secret-access-key</SecretAccessKey>`,
		`      <SessionToken>some-session-token</SessionToken>`,
		`      <AccessKeyId>some-access-key-id</AccessKeyId>`,
		`      <Expiration>2009-02-13T23:31:30Z</Expiration>`,
		`    </Credentials>`,
		`    <AssumedRoleUser>`,
		`      <Arn>some-arn</Arn>`,
		`    </AssumedRoleUser>`,
		`  </AssumeRoleResult>`,
		`  <ResponseMetadata>`,
		`    <StatusCode>200</StatusCode>`,
		`    <RequestID>some-request-id</RequestID>`,
		`  </ResponseMetadata>`,
		`</AssumeRoleResponse>`,
	}, strings.Split(string(data), "\n"))
}
