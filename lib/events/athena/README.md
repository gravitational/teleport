# Athena audit log backend

## Running integration tests

The integration tests run against real AWS resources in the **teleport-dev** account, region **`eu-central-1`**.

### Prerequisites

- Access to the `teleport-dev` AWS account
- The following resources already exist in `eu-central-1`:
  - S3 bucket `auditlogs-integrationtests-locking` (object locking enabled, 1-day governance retention)
  - S3 bucket `auditlogs-integrationtests` (1-day lifecycle expiration)
  - Glue database `auditlogs_integrationtests`
  - Athena workgroup `primary`

The tests create and clean up per-run resources automatically (SNS topic, SQS queue, Athena table). The S3 buckets and Glue database are shared and long-lived.

### Run

```bash
AWS_PROFILE=teleport-dev AWS_REGION=eu-central-1 TEST_AWS=true \
  go test ./lib/events/athena/ -run TestIntegrationAthena -v
```

`AWS_REGION` must be `eu-central-1` — the S3 buckets live in that region and the AWS SDK will return `301 PermanentRedirect` if a different region is used.