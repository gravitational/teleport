## Identity Activity Center IaC Setup

This directory contains the IaC setup for the Identity Activity Center infrastructure.

#### Configuration

```bash
cat > variables.auto.tfvars << EOF
aws_region            = "eu-central-1"
iac_sqs_queue_name        = "example-sns_queue"
iac_sqs_dlq_name          = "example-sns_dlq"
iac_kms_key_alias         = "example-kms_key"
iac_long_term_bucket_name = "example-long-term-bucket"
iac_transient_bucket_name = "example-transient-bucket"
iac_database_name         = "example_db"
iac_table_name            = "example_table"
iac_workgroup             = "example_iac_workgroup"
EOF
```

