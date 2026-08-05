# Teleport Beams BYOI Setup

## Provider Requirements

Let `<major>` be `cluster_version`'s major version. Set `<provider_version>` to
`>= 18.8.0, < 19.0.0` when `<major>` is 18, else `~> <major>.0`.

Declare the provider requirements and configuration:

```hcl
terraform {
  required_version = ">= 1.5.7"
  required_providers {
    teleport = {
      source  = "terraform.releases.teleport.dev/gravitational/teleport"
      version = "<provider_version>"
    }
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
    tls = {
      source  = "hashicorp/tls"
      version = ">= 4.0"
    }
  }
}

provider "teleport" {
  addr = "<proxy_addr>"
}
```

## AWS IAM OIDC Provider

```hcl
resource "aws_iam_openid_connect_provider" "teleport" {
  url = "<proxy_addr>"
  client_id_list = "discover.teleport"
  thumbprint_list = [data.tls_certificate.teleport_proxy[0].certificates[0].sha1_fingerprint]
  tags = {
    "teleport.dev/cluster" = "<cluster_name>"
    "teleport.dev/integration" = "<integration_name>"
    "teleport.dev/iac-tool" = "terraform"
  }
}

data "tls_certificate" "teleport_proxy" {
  url = "<proxy_addr>"
}
```

## AWS IAM Role

Let `<provider_name>` be `cluster_name`.

```hcl
resource "aws_iam_role" "teleport" {
  name = "<proxy_addr>"
  assume_role_policy = data.aws_iam_policy_document.teleport_iam_role_trust.json
  tags = {
    "teleport.dev/cluster" = "<cluster_name>"
    "teleport.dev/integration" = "<integration_name>"
    "teleport.dev/iac-tool" = "terraform"
  }
  max_session_duration = 3600
}

data "aws_iam_policy_document" "teleport_iam_role_trust" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [aws_iam_openid_connect_provider.teleport[0].arn]
    }

    condition {
      test     = "StringEquals"
      variable = "<provider_name>:aud"
      values   = ["discover.teleport"]
    }
  }

  statement {
    actions = ["bedrock-mantle:CreateInference"]
    # customize resource for alternate Bedrock Mantle project
    resource = "arn:aws:bedrock-mantle:*:<aws_account_id>:project/default"
  }

  statement {
      # This statement is required for Mantle to automatically create
      # subscriptions when models are first used.

      actions = [
        "aws-marketplace:Subscribe",
        "aws-marketplace:ViewSubscriptions",
      ]
      resources = ["*"]

      condition {
        test     = "StringEquals"
        variable = "aws:CalledViaLast"
        values   = ["bedrock-mantle.amazonaws.com"]
      }
    }
}
```

## Teleport Integration

```hcl
resource "teleport_integration" "<integration_name>" {
  metadata = {
    name = "<integration_name>"
  }
  spec = {
    aws_oidc = {
      role_arn = one(aws_iam_role.teleport[*].arn)
    }
  }
  sub_kind = "aws-oidc"
  version  = "v1"
}
```

## Teleport App Servers

Let `<host>` be `cluster_name`. Each configured slot gets a Teleport app
server. Don't create if the `app_name` is exactly "anthropic" or "openai", as
these are default, internal apps.

```hcl
resource "teleport_app_server" "<app_name>" {
  version = "v1"
  metadata = {
    name = "<app_name>"
  }

  spec = {
    host_id = uuid()
    app = {
      kind = "app"
      sub_kind = "llm"
      version = "v3"
      metadata = {
        name = "<app_name>"
      }
      spec = {
        public_addr  = "<app_name>.<host>"
        integration  = teleport_integration.bedrock.metadata.name
        inference = {
          format   = "anthropic"
          provider = "bedrock"
          models = [ "<anthropic_models>" ]
          fallback_model = "<anthropic_fallback_model>"
        }
      }
    }
  }
}
```

Each configured model is a mapping from client name to provider name. For
example:

```hcl
{ name = "claude-opus-4-8", provider_name = "anthropic.claude-opus-4-8" }
```

For the OpenAI slot, use `format = "openai"` and Bedrock model identifiers. Do
not add `api_key_secret_ref`, a public OpenAI base URL, or another provider.

`fallback_model`, when set, must equal one of the app's configured model names.

## Teleport Beams Configuration

```hcl
resource "teleport_beams_config" "this" {
  version = "v1"
  metadata = {
    labels = {
      "teleport.dev/origin" = "dynamic"
    }
  }

  spec = {
    llm = {
      anthropic = {
        app_name = "<app_name>"
      }
      openai = {
        app_name = "<app_name>"
      }
    }
  }
}
```

Import an existing singleton before managing it:

```text
terraform import teleport_beams_config.this beams-config
```
