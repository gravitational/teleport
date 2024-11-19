#!/bin/bash

set -euo pipefail

source vars.env

if [[ "$TELEPORT_BACKEND" == "firestore" ]]; then
  exit 0
fi


dynamo_policy="$STATE_DIR/dynamo-iam-policy"

s3_policy="$STATE_DIR/s3-iam-policy"

route53_policy="$STATE_DIR/route53-policy"

mkdir -p "$STATE_DIR"

cat > "$dynamo_policy" <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "ClusterStateStorage",
            "Effect": "Allow",
            "Action": [
                "dynamodb:BatchWriteItem",
                "dynamodb:UpdateTimeToLive",
                "dynamodb:PutItem",
                "dynamodb:DeleteItem",
                "dynamodb:Scan",
                "dynamodb:Query",
                "dynamodb:DescribeStream",
                "dynamodb:UpdateItem",
                "dynamodb:DescribeTimeToLive",
                "dynamodb:CreateTable",
                "dynamodb:DescribeTable",
                "dynamodb:GetShardIterator",
                "dynamodb:GetItem",
                "dynamodb:UpdateTable",
                "dynamodb:GetRecords",
                "dynamodb:UpdateContinuousBackups"
            ],
            "Resource": [
                "arn:aws:dynamodb:${REGION}:${ACCOUNT_ID}:table/${CLUSTER_NAME}-backend",
                "arn:aws:dynamodb:${REGION}:${ACCOUNT_ID}:table/${CLUSTER_NAME}-backend/stream/*"
            ]
        },
        {
            "Sid": "ClusterEventsStorage",
            "Effect": "Allow",
            "Action": [
                "dynamodb:CreateTable",
                "dynamodb:BatchWriteItem",
                "dynamodb:UpdateTimeToLive",
                "dynamodb:PutItem",
                "dynamodb:DescribeTable",
                "dynamodb:DeleteItem",
                "dynamodb:GetItem",
                "dynamodb:Scan",

                "dynamodb:Query",
                "dynamodb:UpdateItem",
                "dynamodb:DescribeTimeToLive",
                "dynamodb:UpdateTable",
                "dynamodb:UpdateContinuousBackups"
            ],
            "Resource": [
                "arn:aws:dynamodb:${REGION}:${ACCOUNT_ID}:table/${CLUSTER_NAME}-events",
                "arn:aws:dynamodb:${REGION}:${ACCOUNT_ID}:table/${CLUSTER_NAME}-events/index/*"
            ]
        }
    ]
}
EOF

cat > "$s3_policy" <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "BucketActions",
            "Effect": "Allow",
            "Action": [
                "s3:PutEncryptionConfiguration",
                "s3:PutBucketVersioning",
                "s3:ListBucketVersions",
                "s3:ListBucketMultipartUploads",
                "s3:ListBucket",
                "s3:GetEncryptionConfiguration",
                "s3:GetBucketVersioning",
                "s3:CreateBucket"
            ],
            "Resource": "arn:aws:s3:::${SESSION_BUCKET}"
        },
        {
            "Sid": "ObjectActions",
            "Effect": "Allow",
            "Action": [
                "s3:GetObjectVersion",
                "s3:GetObjectRetention",
                "s3:*Object",
                "s3:ListMultipartUploadParts",
                "s3:AbortMultipartUpload"
            ],
            "Resource": "arn:aws:s3:::${SESSION_BUCKET}/*"
        }
    ]
}
EOF


cat > "$route53_policy" <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "route53:GetChange",
            "Resource": "arn:aws:route53:::change/*"
        },
        {
            "Effect": "Allow",
            "Action": [
                "route53:ChangeResourceRecordSets",
                "route53:ListResourceRecordSets"
            ],
            "Resource": "arn:aws:route53:::hostedzone/${ROUTE53_ZONE_ID}"
        }
    ]
}
EOF
