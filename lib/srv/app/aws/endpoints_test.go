/*
Copyright 2021 Gravitational, Inc.

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
	"bytes"
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/stretchr/testify/require"
)

// serviceSigningNames is a list of signing names used by AWS services.
//
// This list is gathered by running (in aws-sdk-go-v2 source repo):
// find service -name "endpoints.go" | xargs awk '/signingName = /{print $3}' | sort -u | tr '\n' ','
var serviceSigningNames = []string{
	"a4b", "access-analyzer", "account", "acm", "acm-pca", "airflow",
	"amplify", "amplifybackend", "amplifyuibuilder", "apigateway",
	"app-integrations", "appconfig", "appconfigdata", "appflow",
	"application-autoscaling", "application-cost-profiler",
	"applicationinsights", "appmesh", "apprunner", "appstream", "appsync",
	"aps", "athena", "auditmanager", "autoscaling", "autoscaling-plans",
	"aws-marketplace", "awsiottwinmaker",
	"awsmigrationhubstrategyrecommendation", "awsmobilehubservice",
	"awsproton20200720", "awsssooidc", "awsssoportal", "backup",
	"backup-gateway", "batch", "braket", "budgets", "ce", "chime", "cloud9",
	"cloudcontrolapi", "clouddirectory", "cloudformation", "cloudfront",
	"cloudhsm", "cloudsearch", "cloudtrail", "codeartifact", "codebuild",
	"codecommit", "codedeploy", "codeguru-profiler", "codeguru-reviewer",
	"codepipeline", "codestar", "codestar-connections",
	"codestar-notifications", "cognito-identity", "cognito-idp",
	"cognito-sync", "comprehend", "comprehendmedical", "compute-optimizer",
	"config", "connect", "cur", "databrew", "dataexchange", "datapipeline",
	"datasync", "dax", "detective", "devicefarm", "devops-guru",
	"directconnect", "discovery", "dlm", "dms", "drs", "ds", "dynamodb", "ebs",
	"ec2", "ec2-instance-connect", "ecr", "ecr-public", "ecs", "eks",
	"elastic-inference", "elasticache", "elasticbeanstalk",
	"elasticfilesystem", "elasticloadbalancing", "elasticmapreduce",
	"elastictranscoder", "emr-containers", "es", "events", "evidently",
	"finspace", "finspace-api", "firehose", "fis", "fms", "forecast",
	"frauddetector", "fsx", "gamelift", "geo", "glacier", "globalaccelerator",
	"glue", "grafana", "greengrass", "groundstation", "guardduty", "health",
	"healthlake", "honeycode", "iam", "identitystore", "imagebuilder",
	"inspector", "inspector2", "iot", "iot-jobs-data", "iot1click",
	"iotanalytics", "iotdata", "iotdeviceadvisor", "iotevents",
	"ioteventsdata", "iotfleethub", "iotsecuredtunneling", "iotsitewise",
	"iotthingsgraph", "iotwireless", "ivs", "kafka", "kafkaconnect", "kendra",
	"kinesis", "kinesisanalytics", "kinesisvideo", "kms", "lakeformation",
	"lambda", "lex", "license-manager", "lightsail", "logs",
	"lookoutequipment", "lookoutmetrics", "lookoutvision", "machinelearning",
	"macie", "macie2", "managedblockchain", "marketplacecommerceanalytics",
	"mediaconnect", "mediaconvert", "medialive", "mediapackage",
	"mediapackage-vod", "mediastore", "mediatailor", "memorydb", "mgh", "mgn",
	"mobiletargeting", "monitoring", "mq", "mturk-requester",
	"network-firewall", "networkmanager", "nimble", "opsworks", "opsworks-cm",
	"organizations", "outposts", "panorama", "personalize", "pi", "polly",
	"pricing", "profile", "qldb", "quicksight", "ram", "rbin", "rds",
	"rds-data", "redshift", "redshift-data", "refactor-spaces", "rekognition",
	"resiliencehub", "resource-groups", "robomaker", "route53",
	"route53-recovery-cluster", "route53-recovery-control-config",
	"route53-recovery-readiness", "route53domains", "route53resolver", "rum",
	"s3", "s3-outposts", "sagemaker", "savingsplans", "schemas",
	"secretsmanager", "securityhub", "serverlessrepo", "servicecatalog",
	"servicediscovery", "servicequotas", "ses", "shield", "signer", "sms",
	"sms-voice", "snow-device-management", "snowball", "sns", "sqs", "ssm",
	"ssm-contacts", "ssm-incidents", "sso", "states", "storagegateway", "sts",
	"support", "swf", "synthetics", "tagging", "textract", "timestream",
	"transcribe", "transfer", "translate", "voiceid", "waf", "waf-regional",
	"wafv2", "wellarchitected", "wisdom", "workdocs", "worklink", "workmail",
	"workmailmessageflow", "workspaces", "workspaces-web", "xray",
}

func TestResolveEndpoint(t *testing.T) {
	signer := v4.NewSigner(credentials.NewStaticCredentials("fakeClientKeyID", "fakeClientSecret", ""))
	now := time.Now()

	for _, serviceSignName := range serviceSigningNames {
		serviceSignName := serviceSignName

		t.Run(serviceSignName, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequest("GET", "http://localhost", nil)
			require.NoError(t, err)

			_, err = signer.Sign(req, bytes.NewReader(nil), serviceSignName, "us-east-1", now)
			require.NoError(t, err)

			endpoint, err := resolveEndpoint(req)
			require.NoError(t, err)
			require.Equal(t, serviceSignName, endpoint.SigningName)
		})
	}
}
