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

package mocks

import (
	"bytes"
	"html/template"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	awstime "github.com/aws/smithy-go/time"
	"github.com/gravitational/trace"
)

// AssumedRoleARN is a sample ARN of an assumed role.
const AssumedRoleARN = "arn:aws:sts::123456789012:assumed-role/role-name/role-session-name"

// AssumeRoleXMLResponse returns the AssumeRole response in XML format.
func AssumeRoleXMLResponse(output *sts.AssumeRoleOutput) ([]byte, error) {
	// AWS SDK uses private functions to serialize/deserialize API structs.
	// Instead of mimicking the SDK, this output was captured from AWS CLI with
	// debug flag.
	tmpl := template.Must(template.New("sts:AssumeRole").Funcs(awsTemplateFuncs).Parse(`
<AssumeRoleResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <AssumeRoleResult>
    <AssumedRoleUser>
      <AssumedRoleId>{{ formatString .AssumedRoleUser.AssumedRoleId }}</AssumedRoleId>
      <Arn>{{ formatString .AssumedRoleUser.Arn }}</Arn>
    </AssumedRoleUser>
    <Credentials>
      <AccessKeyId>{{ formatString .Credentials.AccessKeyId }}</AccessKeyId>
      <SecretAccessKey>{{ formatString .Credentials.SecretAccessKey }}</SecretAccessKey>
      <SessionToken>{{ formatString .Credentials.SessionToken }} </SessionToken>
      <Expiration>{{ formatTime .Credentials.Expiration }}</Expiration>
    </Credentials>
  </AssumeRoleResult>
  <ResponseMetadata>
    <RequestId>22222222-2222-2222-2222-222222222222</RequestId>
  </ResponseMetadata>
</AssumeRoleResponse>
`))

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, output); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

// GetCallerIdentityXMLResponse returns the GetCalllerIdentity response in XML format.
func GetCallerIdentityXMLResponse(output *sts.GetCallerIdentityOutput) ([]byte, error) {
	tmpl := template.Must(template.New("sts:GetCalllerIdentity").Funcs(awsTemplateFuncs).Parse(`
<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <GetCallerIdentityResult>
    <Arn>{{ formatString .Arn }}</Arn>
    <UserId>{{ formatString .UserId }}</UserId>
    <Account>{{ formatString .Account }}</Account>
  </GetCallerIdentityResult>
  <ResponseMetadata>
    <RequestId>22222222-3333-3333-3333-333333333333</RequestId>
  </ResponseMetadata>
</GetCallerIdentityResponse>
`))

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, output); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

var awsTemplateFuncs = template.FuncMap{
	"formatString": aws.ToString,
	"formatTime": func(t *time.Time) string {
		return awstime.FormatDateTime(aws.ToTime(t))
	},
}
