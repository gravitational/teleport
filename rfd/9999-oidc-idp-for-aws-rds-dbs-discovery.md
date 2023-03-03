---
authors: Marco Dinis (marco.dinis@goteleport.com)
state: draft
---

# RFD 999 - OIDC IdP for AWS RDS DBs discovery

## Required Approvals

* Engineering: @ryanclark && @r0mant
* Product: @xinding33 || @klizhentas
* Security: @reedloden

## What

Discover AWS RDS Databases without manual configuration.

#### Goals

Easily discover AWS RDS Databases of an account.

#### Non-goals

Having a real OIDC IdP is out of scope.
That initiative can be tracked [here](https://github.com/gravitational/teleport/issues/20967).

Automatically onboard AWS RDS Databases into Teleport.

Discovering other AWS resources is out of scope.

## Terminology

* OIDC: OpenID Connect is a protocol that works on top of OAuth2 and provides identity information.
* IdP: Identity Provider is a system that manages identities.
* JWT: JSON Web Token is a json encoded token that contains a set of claims signed by an entity.
* JWKS: JSON Web Key Set is a set of public keys that IdP consumers must use to validate a signed JWT.

## Why

Our current discovery process for AWS resources requires multiple steps before the user gets to the "aha moment".
Some of those steps are a little complicated and require the user to know some Teleport specific internals.

We can reduce the number of steps and remove most of the Teleport specific configuration by creating an OIDC IdP in Teleport.

## How

AWS allows the [set up](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_providers_create_oidc.html) of an IdP using OIDC providers.
Each provider has a set of roles, which limit their access.

Turning Teleport into an OIDC IdP, and asking the user to create a new OIDC IdP that consumes the Teleport OIDC endpoints, allows us to easily call AWS API endpoints to, for instance, list RDS instances and their details.

When configuring the provider, we'll need an AWS Role which Teleport uses to issue API Calls.

To store the configuration above, we'll create a new resource Kind: `Integration`.
It'll leverage the `subkind` prop to distinguish future integrations.

```yaml
kind: integration
subkind: aws-oidc
version: v1
metadata:
	name: some-name
spec:
	aws_role: <aws role>
```

### High Level Flow
Simplified flow of interactions between User, Teleport and AWS:

```
                                                               ┌─────────┐
                                                               │         │
                ┌─────────────────────────────────────────────►│  AWS    │
               2│                                              │         │
                │                                              └──┬───▲──┘
User────────────┤                                                 │   │
                │       ┌───────────────────────┐                 │   │
               1│       │       Teleport        │                 │   │
                │       │                       │                 │   │
               5│    ┌──┼──────┐       ┌────────┼──┐              │   │
                └────►  │ Web  │       │ OIDC   │  │       3      │   │
                     └──┼──┬───┘       │ IdP    │  │◄─────────────┘   │
                        │  │           └┬───────┼──┘                  │
                        │  │            │       │                     │
                        │  │           4│       │                     │
                        │  │            │       │                     │
                        │  │  ┌─────────▼┐      │                     │
                        │  │  │ CA KeySet│      │                     │
                        │  │  │ +RSA Key │      │                     │
                        │  │  └─────────▲┘      │                     │
                        │  │6           │7      │                     │
                        │  │     ┌──────┴────┐  │                     │
                        │  │     │ Discover  │  │           8         │
                        │  └────►│ Service   ├──┼─────────────────────┘
                        │        │ [ RDS ]   │  │
                        │        └───────────┘  │
                        │                       │
                        └───────────────────────┘
```

#### User's Point of View
When the user adds a new resource, using the Discover Wizard, they'll be asked where the resource lives.
Choosing an RDS DB, the user is presented with the following flow:

1. User is prompted to setup an OIDC Identity Provider pointing to its Teleport instance's url.
2. User opens AWS, selects IAM and then, under `Access Management`, selects `Identity providers`.
3. Configures the Teleport instance as an OIDC IdP:
   1. Clicks `Add provider` of type `OpenID Connect`.
   2. Fills in the Provider URL as provided by the Discover Wizard.
   3. Fills in the Audience as provided by the Discover Wizard `sts.amazonaws.com`.
4. The user is then asked to enter the Thumbprint in Teleport, to ensure the user entered the correct `Provider URL`.
5. Configures the role associated with that IdP
   1. The user is asked to create or assign an existing role.
   2. The role's `Policies` must be filled with the required Policies (described down below).
   3. The role's `Trust relationships` must target the Identity Provider they just created (described down below).
6. The user will be greeted with a message in Teleport saying that the configuration was successful.

To ease the necessary manual steps, the Discover wizard will provide a small video demoing what the user must do for the previous steps.

At this point, the integration is configured and the user is able to discover RDS DBs without leaving Teleport.

#### System's Point of View
The first interaction between AWS and Teleport happens when the user clicks on `Get Thumbprint` (step 4 of the User's Point of View flow).
This will trigger a request started by AWS to Teleport hitting the OpenID Configuration endpoint and subsequently the JSON Web Key Set endpoint.

After this step, when the user tries to list RDS DBs, Teleport generates a token and issues the API call.

To generate a token, Teleport creates a JWT with the claims described above and the configured role, and signs it with the private key (the public is provided in the public JWKS endpoint).

AWS receives the request and validates the token against the public key provided by OIDC IdP (ie, Teleport) at the `jwks_uri`.
If authenticated and authorized for the api call, AWS returns the response to the API call.

### Implementation Details

#### Signing Key
One of the requirements to be an OIDC provider is to provide the public key in a known HTTP endpoint and sign a JSON object (with claims) with the private key.

We'll create a new RSA 2048 Key type and add it to the CertAuthority.
This will be very similar to the JWTSigner key type used for App access.

A single signing key will be used for this flow even if multiple AWS integrations are created (eg, multiple regions).

#### HTTP Endpoints
The following endpoints will be used during OIDC IdP set up:

##### OpenID Configuration
According to the [spec](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderConfig) the Identity Provider must provide an endpoint at `<providerURL>/.well-known/openid-configuration` that returns the provider's configuration.

So, a new endpoint at `<teleportProviderURL>/.well-known/openid-configuration` will be created that returns the following JSON:
```
200 OK

{
  "issuer": "<teleportProviderURL>", 
  "jwks_uri": "<teleportProviderURL>/.well-known/jwks-oidc",
  "claims": ["iss", "sub", "aud", "jti", "iat", "exp", "nbf"],
  "id_token_signing_alg_values_supported": ["RS256"],
  "response_types_supported": ["id_token"],
  "scopes_supported": ["openid"],
  "subject_types_supported": ["public", "pairwise"]
}
```

- Issuer: the provider's URL
- JWKS URI: the endpoint where the provider returns the public keys.
- Claims Supported: a list of supported claims that the OpenID Provider (ie, Teleport) MAY be able to supply values for when issuing a token:
  - `iss`: the issuer identity, on our case it will be the same as the `issuer` key.
  - `sub`: identifies the subject of the token, on our case it will contain the user's name.
  - `aud`: the audience for the token, on our case it will be the same as the `audience` defined when configuring the OIDC IdP in AWS (step 3.3 for the User's Point of View).
  - `jti`: is the token id.
  - `iat`: unix epoch time of when the token was issued.
  - `exp`: unix epoch time of when the token is no longer valid (expired).
  - `nbf`: unix epoch time which indicates when the token began to be valid.
- ID Token Signing Algorithm Values Supported: a list of supported signing algorithms to assist during token validation.
- Scopes Supported: a list of scopes to be used, on our case it will only contain the required one: `openid`.
- Subject Types Supported: a list of supported subject types:
  - `public`: we can either return clear text version of the subject (user names) or
  - `pairwise`: we can return a hash of the subject's identity (to hide their email, for instance)

##### JSON Web Key Sets
This endpoint returns a list of public keys (usually only one key, except for periods of key rotation).

This endpoint URI must be equal to `jwks_uri` defined in the previous section.

As an example, `<teleportProviderURL>/.well-known/jwks-oidc`.

It should return the following:

```
200 OK

{
  "keys": [
    {
      "kty": "RSA",
      "alg": "RS256",
      "n": "<public key value>",
      "e": "AQAB"
    }
  ]
}
```

This must respect the RFC7517 (eg of a key can be found at https://www.rfc-editor.org/rfc/rfc7517#appendix-A.1).

Obtaining this key should be possible using
```golang
GetCertAuthority(
	ctx,
	types.CertAuthID{
		Type:       types.OIDCSigner, // new key type
		DomainName: clusterName,
	},
	false /*loadKeys*/,
)
```

#### AWS Role for Teleport OIDC IdP

The user will create or associate an AWS Role to the Teleport OIDC IdP.
This role must have access to list RDS Databases and have a policy that trusts Teleport as an Identity Provider.

At least a policy allowing the following must be part of the role:
```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "rds:DescribeDBInstances",
            "Resource": "*"
        }
    ]
}
```

The AWS Role must trust Teleport as an IdP.
To do so, the user must add the following Trusted relationship:
```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "Federated": "arn:aws:iam::<account id>:oidc-provider/<teleportProviderURL>"
            },
            "Action": "sts:AssumeRoleWithWebIdentity",
            "Condition": {
                "StringEquals": {
                    "<teleportProviderURL>:aud": "sts.amazonaws.com"
                }
            }
        }
    ]
}
```

### Security

#### Rotation
JWT Signing keys can be rotated using `tctl auth rotate --type=oidc`.

JWKS endpoint returns a list of public keys, so the old and the new key are provided for a seamless migration.

#### MITM between AWS and Teleport

The only configuration AWS has about the provider is its URL.
If that DNS is controlled by some evil part, AWS might accept requests from that evil part.

However, assuming the DNS is controlled by an evil part, then the users that are logging in to Teleport are also providing their credentials to that part.

We don't think this is a scenario we should focus on this RFD.

#### AWS Calls from unauthorized Teleport Users
We'll be able to call `DescribeDBInstances` AWS endpoint, however we must ensure that this is protected by RBAC.

In order to do so, we'll re-use the `db` resource with the `list` verb to allow listing RDS Databases.

```yaml
kind: role
metadata:
  name: editor
spec:
  allow:
    rules:
    - resources:
      - db
      verbs:
      - list
```

## Proof of Concept

<details>
<summary>View code</summary>

Check the `TODO`s within the source code.

```golang
package main

import (
	"crypto/rsa"
	"crypto/x509"
	_ "embed"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/gravitational/trace"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/olekukonko/tablewriter"
)

type DiscoveryConfiguration struct {
	// Issuer is the identifier of the OP and is used in the tokens as `iss` claim.
	Issuer string `json:"issuer,omitempty"`

	// JwksURI is the URL of the JSON Web Key Set. This site contains the signing keys that RPs can use to validate the signature.
	// It may also contain the OP's encryption keys that RPs can use to encrypt request to the OP.
	JwksURI string `json:"jwks_uri,omitempty"`

	// ClaimsSupported contains a list of Claim Names the OP may be able to supply values for. This list might not be exhaustive.
	ClaimsSupported []string `json:"claims_supported,omitempty"`

	// IDTokenSigningAlgValuesSupported contains a list of JWS signing algorithms (alg values) supported by the OP for the ID Token.
	IDTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported,omitempty"`

	// ResponseTypesSupported contains a list of the OAuth 2.0 response_type values that the OP supports (code, id_token, token id_token, ...).
	ResponseTypesSupported []string `json:"response_types_supported,omitempty"`

	// ScopesSupported lists an array of supported scopes. This list must not include every supported scope by the OP.
	ScopesSupported []string `json:"scopes_supported,omitempty"`

	// SubjectTypesSupported contains a list of Subject Identifier types that the OP supports (pairwise, public).
	SubjectTypesSupported []string `json:"subject_types_supported,omitempty"`
}

// $ openssl genrsa -out keypair.pem 2048
//
//go:embed keypair.pem
var privateKey string

// TODO replace with a public HTTPS server which serves this server (port 1323)
var URL = "<public address>/v1"
// TODO add account id and role to be associated to the IdP
var roleARN = "arn:aws:iam::<account id>:role/<role>"

type IdentityToken string

// GetIdentityToken retrieves the JWT token from the file and returns the contents as a []byte
func (j IdentityToken) GetIdentityToken() ([]byte, error) {
	return []byte(j), nil
}

func main() {
	e := echo.New()
	e.Debug = true
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "method=${method}, uri=${uri}, status=${status}\n",
	}))

	block, _ := pem.Decode([]byte(privateKey))
	parseResult, _ := x509.ParsePKCS8PrivateKey(block.Bytes)
	key := parseResult.(*rsa.PrivateKey)

	e.GET("/v1/.well-known/openid-configuration", func(c echo.Context) error {
		discovery := &DiscoveryConfiguration{
			Issuer:                           "https://" + URL,
			JwksURI:                          "https://" + URL + "/.well-known/jwks",
			ClaimsSupported:                  []string{"sub", "aud", "exp", "iat", "iss", "jti", "nbf", "ref"},
			IDTokenSigningAlgValuesSupported: []string{"RS256"},
			ResponseTypesSupported:           []string{"id_token"},
			ScopesSupported:                  []string{"openid"},
			SubjectTypesSupported:            []string{"public", "pairwise"},
		}

		return c.JSON(http.StatusOK, discovery)
	})

	e.GET("/v1/.well-known/jwks", func(c echo.Context) error {
		keyset := &jose.JSONWebKeySet{
			Keys: []jose.JSONWebKey{
				{
					KeyID:     "id",
					Algorithm: "RS256",
					Use:       "sig",
					Key:       &key.PublicKey,
				},
			},
		}

		return c.JSON(http.StatusOK, keyset)
	})

	e.GET("/aws", func(c echo.Context) error {
		ctx := c.Request().Context()

		key := jose.SigningKey{Algorithm: jose.RS256, Key: key}

		// create a Square.jose RSA signer, used to sign the JWT
		var signerOpts = jose.SignerOptions{}
		signerOpts.WithType("JWT")
		rsaSigner, err := jose.NewSigner(key, &signerOpts)
		if err != nil {
			return trace.Wrap(err)
		}

		builder := jwt.Signed(rsaSigner)

		pubClaims := jwt.Claims{
			Issuer:   "https://" + URL,
			Subject:  "some-subject",
			ID:       "id1",
			Audience: jwt.Audience{"sts.amazonaws.com"},
			IssuedAt: jwt.NewNumericDate(time.Now()),
			Expiry:   jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		}

		builder = builder.Claims(pubClaims)
		rawJWT, err := builder.CompactSerialize()
		if err != nil {
			return trace.Wrap(err)
		}

		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Region = "us-east-1"

		p := stscreds.NewWebIdentityRoleProvider(
			sts.NewFromConfig(cfg),
			roleARN,
			IdentityToken(rawJWT),
		)

		cfg2, err := config.LoadDefaultConfig(ctx, config.WithCredentialsProvider(p))
		if err != nil {
			return trace.Wrap(err)
		}
		cfg2.Region = "us-east-1"

		// RDS List DBs
		rdsClient := rds.NewFromConfig(cfg2)
		rdsDBs, err := rdsClient.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{})
		if err != nil {
			return trace.Wrap(err)
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Status", "Name", "IAMAuth", "Engine", "EngineVersion", "MasterUserName", "DBName", "Tags", "ARN", "Addr", "Port"})

		for _, db := range rdsDBs.DBInstances {
			table.Append([]string{
				*db.DBInstanceStatus,
				*db.DBInstanceIdentifier,
				fmt.Sprintf("%t", db.IAMDatabaseAuthenticationEnabled),
				*db.Engine,
				*db.EngineVersion,
				*db.MasterUsername,
				stringOrNil(db.DBName),
				fmt.Sprintf("%v", db.TagList),
				*db.DBInstanceArn,
				*db.Endpoint.Address,
				fmt.Sprintf("%d", db.Endpoint.Port),
			})
		}
		table.Render() // Send output

		return c.String(http.StatusOK, "hello!")
	})

	e.Logger.Fatal(e.Start(":1323"))
}

func stringOrNil(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}
```
</details>
