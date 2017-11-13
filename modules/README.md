# Terraform Modules

Modules is a managed collection of Teleport modules
useful for AWS deployments of Teleport. These modules provide supported scenarios to
deploy Teleport on cloud providers.

To use the modules, include them in the terraform script:

```
provider "aws" {
}

module dynamoautoscale {
 source = "github.com/gravitational/teleport//modules/dynamodbautoscale"
 table_name = "table-to-auto-scale"
}
```

* [dynamoautoscale](dynamoautoscale) Enables DynamoDB table autoscaling


