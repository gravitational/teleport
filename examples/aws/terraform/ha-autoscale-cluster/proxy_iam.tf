// Proxy instance profile and roles
resource "aws_iam_role" "proxy" {
  name = "${var.cluster_name}-proxy"

  assume_role_policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {"Service": "ec2.amazonaws.com"},
            "Action": "sts:AssumeRole"
        }
    ]
}
EOF

}

// Proxy fetches certificates obtained by auth servers from encrypted S3 bucket.
// Proxies do not setup certificates, to keep privileged operations happening
// only on auth servers.
resource "aws_iam_instance_profile" "proxy" {
  name       = "${var.cluster_name}-proxy"
  role       = aws_iam_role.proxy.name
  depends_on = [aws_iam_role_policy.proxy_ssm]
}

resource "aws_iam_role_policy" "proxy_s3" {
  name = "${var.cluster_name}-proxy-s3"
  role = aws_iam_role.proxy.id

  policy = <<EOF
{
   "Version": "2012-10-17",
   "Statement": [
     {
       "Effect": "Allow",
       "Action": ["s3:ListBucket"],
       "Resource": ["arn:aws:s3:::${aws_s3_bucket.certs.bucket}"]
     },
     {
       "Effect": "Allow",
       "Action": [
         "s3:GetObject"
       ],
       "Resource": ["arn:aws:s3:::${aws_s3_bucket.certs.bucket}/*"]
     }
   ]
 }

EOF

}

// Proxies fetch join tokens from SSM parameter store. Tokens are rotated
// and published by auth servers on an hourly basis.
resource "aws_iam_role_policy" "proxy_ssm" {
  name = "${var.cluster_name}-proxy-ssm"
  role = aws_iam_role.proxy.id

  policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "ssm:GetParameters",
                "ssm:GetParametersByPath",
                "ssm:GetParameter"
            ],
            "Resource": "arn:aws:ssm:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:parameter/teleport/${var.cluster_name}/tokens/proxy"
        },
        {
            "Effect": "Allow",
            "Action": [
                "ssm:GetParameters",
                "ssm:GetParametersByPath",
                "ssm:GetParameter"
            ],
            "Resource": "arn:aws:ssm:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:parameter/teleport/${var.cluster_name}/ca-pin-hash"
        },
        {
         "Effect":"Allow",
         "Action":[
            "kms:Decrypt"
         ],
         "Resource":[
            "arn:aws:kms:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:key/${data.aws_kms_alias.ssm.target_key_id}"
         ]
      }
    ]
}
EOF

}

