module github.com/gravitational/teleport-ami-update

go 1.23

require (
	github.com/alecthomas/kingpin/v2 v2.3.2 // replaced
	github.com/aws/aws-sdk-go-v2 v1.39.5
	github.com/aws/aws-sdk-go-v2/config v1.31.16
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.260.0
)

require (
	github.com/alecthomas/units v0.0.0-20211218093645-b94a6e3cc137 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.18.20 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.12 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.12 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.12 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.12 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.39.0 // indirect
	github.com/aws/smithy-go v1.23.1 // indirect
	github.com/xhit/go-str2duration/v2 v2.1.0 // indirect
)

replace github.com/alecthomas/kingpin/v2 => github.com/gravitational/kingpin/v2 v2.1.11-0.20230515143221-4ec6b70ecd33
