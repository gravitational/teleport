## Athena Teleport Backend  IaC Setup
This directory contains the IaC setup for the Athena Teleport Backend and Athena Access Monitoring.

#### Configuration for Athena Audit Events Teleport Backend

```bash
cat > variables.auto.tfvars << EOF
aws_region            = "eu-central-1"
sns_topic_name        = "example-sns_topic"
sqs_queue_name        = "example-sns_queue"
sqs_dlq_name          = "example-sns_dlq"
kms_key_alias         = "example-kms_key"
long_term_bucket_name = "example-long-term-bucket"
transient_bucket_name = "example-transient-bucket"
database_name         = "example_db"
table_name            = "example_table"
workgroup             = "example_workgroup"
EOF
```

#### Configuration for Teleport Audit Event Backend and Athena Access Monitoring

```bash
cat > variables.auto.tfvars << EOF
aws_region            = "eu-central-1"
sns_topic_name        = "example-sns_topic"
sqs_queue_name        = "example-sns_queue"
sqs_dlq_name          = "example-sns_dlq"
kms_key_alias         = "example-kms_key"
long_term_bucket_name = "example-long-term-bucket"
transient_bucket_name = "example-transient-bucket"
database_name         = "example_db"
table_name            = "example_table"
workgroup             = "example_workgroup"

access_monitoring                               = true
access_monitoring_prefix                        = "example_"
access_monitoring_trusted_relationship_role_arn = "arn:aws:iam::123456789012:role/example-teleport-role"
EOF
```


where `access_monitoring_trusted_relationship_role_arn` can be omitted. Terraform Access Monitoring setup will use the current caller identity role arn as the trusted relationship role arn.
