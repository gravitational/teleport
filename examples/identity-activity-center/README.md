## Identity Activity Center IaC Setup

This directory contains the IaC setup for the Identity Activity Center infrastructure.

#### Configuration

```bash
cat > variables.auto.tfvars << EOF
aws_region            = "eu-central-1"
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

