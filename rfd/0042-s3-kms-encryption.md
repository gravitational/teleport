# ---
authors: Joel Wejdenstål (jwejdenstal@goteleport.com)
state: implemented
---

# RFD 42 - S3 KMS Encryption

## What

Allow users to configure a custom AWS KMS Customer Managed Key for encrypting
objects Teleport stores in S3.

## Why

Encrypting objects in S3 with a custom CMK allows for additional security as it
can be used to restrict access to sensitive data like session recordings
to those with read privileges to S3 buckets storing said sensitive data.

## Details

As per the current scheme for configuring storage backends like S3 an optional URL
parameter will be added to the S3 storage url with the name of `sse_kms_key`.
This URL parameter can be specified with the value of the KMS ID of the desired
custom key. When configured all objects uploaded will be encrypted with that key.

Example:
```
"s3://teleport-demo-bucket/records?sse_kms_key=1234abcd-12ab-34cd-56ef-1234567890ab"
```

The configured KMS CMK needs to have a standard spec configuration and must be symmetric.

Below template KMS policies are provided for restricting access to
KMS keys and the needed permissions. The policies are to be filled in with
the IAM user the authentication nodes authenticate as.

### Encryption/Decryption

This policy allows an IAM user to encrypt and decrypt objects.
This allows a cluster auth to write and play back session recordings.

Replace `[iam-key-admin-arn]` with the IAM ARN of the user(s) that should have
administrative key access and `[auth-node-iam-arn]` with the IAM ARN
of the user the Teleport auth nodes are using.

```json
{
  "Id": "Teleport Encryption and Decryption",
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "Teleport CMK Admin",
      "Effect": "Allow",
      "Principal": {
        "AWS": "[iam-key-admin-arn]"
      },
      "Action": "kms:*",
      "Resource": "*"
    },
    {
      "Sid": "Teleport CMK Auth",
      "Effect": "Allow",
      "Principal": {
        "AWS": "[auth-node-iam-arn]"
      },
      "Action": [
        "kms:Encrypt",
        "kms:Decrypt",
        "kms:ReEncrypt*",
        "kms:GenerateDataKey*",
        "kms:DescribeKey"
      ],
      "Resource": "*"
    }
  ]
}
```

### Key Access Permissions

IAM users do by default not have access to administrate and use encryption keys not created by the user
themselves. This does not apply to the AWS root user which has unrestricted permissions.
IAM administrators with the `AdministratorAccess` that should not have access to session recordings
should have a deny clause within the KMS policy to prevent their access. See example below.

```json
{
  "Id": "Teleport Encryption and Decryption",
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "Teleport CMK Admin",
      "Effect": "Allow",
      "Principal": {
        "AWS": "[iam-key-admin-arn]"
      },
      "Action": "kms:*",
      "Resource": "*"
    },
    {
      "Sid": "Teleport CMK Auth",
      "Effect": "Allow",
      "Principal": {
        "AWS": "[auth-node-iam-arn]"
      },
      "Action": [
        "kms:Encrypt",
        "kms:Decrypt",
        "kms:ReEncrypt*",
        "kms:GenerateDataKey*",
        "kms:DescribeKey"
      ],
      "Resource": "*"
    },
    {
      "Sid": "Deny AdministratorAccess users.",
      "Effect": "Deny",
      "Principal": {
        "AWS": "[iam-administrator-user-arn]"
      },
      "Action": "kms:*",
      "Resource": "*"
    }
  ]
}
```

In addition, all IAM users with `kms:*` permissions granted for all KMS keys will also need to be
explicitly denied access as per the above example.

Teleport auth nodes should use a dedicated IAM account and KMS permissions can be granted by filling in the IAM ARN
at the correct places in the template policies as per above instructions.

### Further limiting access to session recordings

When correctly implemented, AWS administrators will not have the
ability to view session recordings since only the key administrator and Teleport auth
IAM user can use the KMS key to decrypt the session recordings.

Teleport administrators will still be able to read the session recordings however.
Access to session recordings can be restricted by limiting access to the `read` verb
for the `session` session resource in roles and only giving permission
to administrative or auditor users that can be trusted with sensitive information.

More information on the role configuration can be found [here](https://goteleport.com/docs/setup/reference/resources/).
