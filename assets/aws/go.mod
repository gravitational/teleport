module github.com/gravitational/teleport-ami-update

go 1.19

require (
	github.com/alecthomas/kingpin/v2 v2.3.2 // replaced
	github.com/aws/aws-sdk-go-v2 v1.20.1
	github.com/aws/aws-sdk-go-v2/config v1.18.33
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.111.0
)

require (
	github.com/alecthomas/units v0.0.0-20211218093645-b94a6e3cc137 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.13.32 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.13.8 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.38 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.4.32 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.39 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.9.32 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.13.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.15.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.21.2 // indirect
	github.com/aws/smithy-go v1.14.1 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/xhit/go-str2duration/v2 v2.1.0 // indirect
)

replace github.com/alecthomas/kingpin/v2 => github.com/gravitational/kingpin/v2 v2.1.11-0.20230515143221-4ec6b70ecd33
