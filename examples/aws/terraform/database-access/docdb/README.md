## Teleport Database Access for DocumentDB example with Terraform

This is a simple Terraform example to deploy an Amazon DocumentDB cluster. You
can optionally create IAM roles for Teleport to use or deploy dynamic Teleport
resources using this example. Further manual steps are also provided in this
README to test out the deployment.

It is assumed that you already have a working Teleport cluster and a running
Database Service. 

This example uses two providers: AWS and
[Teleport](https://goteleport.com/docs/management/dynamic-resources/terraform-provider/).
The Teleport provider is only required when deploy Teleport resources like a
dynamic database.

### Step 1. Create the resources using terraform.

Here is a sample using this module:
```
module "steve-docdb" {
  source = "./path/to/this/example/docdb"

  identifier         = "steve-docdb"
  subnet_ids         = module.vpc.private_subnets
  security_group_ids = [module.vpc.default_security_group_id]
  num_instances      = 2
  tags               = local.tags

  create_teleport_databases                 = true
  create_database_user_iam_role             = true
  databaase_user_iam_role_trusted_role_arns = [module.iam_teleport.iam_role_arn]
}
```

In this sample, a DocumentDB cluster is created with two instances. An IAM role
is created for the database user that will access the DocumentDB cluster and a
AWS Lambda function is executed to provision the database user with IAM
authentication in the created DocumentDB cluster. In addition, several dynamic
resources representing the endpoints are added to the Teleport cluster.

See `variables.tf` for more details.

### Step2. Connect through `tsh`

Once above steps are successful:
```
$ tsh db ls --search docdb
Name                   Description                                             Allowed Users Labels              Connect 
---------------------- ------------------------------------------------------- ------------- ------------------- ------- 
steve-docdb-cluster    Cluster endpoint for DocumentDB steve-docdb             [*]           Env=dev,Owner=STeve         
steve-docdb-instance-0 Instance endpoint for DocumentDB steve-docdb instance 0 [*]           Env=dev,Owner=STeve         
steve-docdb-instance-1 Instance endpoint for DocumentDB steve-docdb instance 1 [*]           Env=dev,Owner=STeve         
steve-docdb-reader     Reader endpoint for DocumentDB steve-docdb              [*]           Env=dev,Owner=STeve   

$ tsh db connect steve-docdb-cluster --db-user role/steve-docdb-teleport-admin-user --db-name test
...
```
