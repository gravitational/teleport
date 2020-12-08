// Proxy instance profile and roles
resource "aws_iam_role" "monitor" {
  name = "${var.cluster_name}-monitor"

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

// Monitor fetches certificates obtained by auth servers from encrypted S3 bucket.
// Monitors do not setup certificates, to keep privileged operations happening
// only on auth servers.
resource "aws_iam_instance_profile" "monitor" {
  name       = "${var.cluster_name}-monitor"
  role       = aws_iam_role.monitor.name
  depends_on = [aws_iam_role_policy.monitor_s3]
}

resource "aws_iam_role_policy" "monitor_s3" {
  name = "${var.cluster_name}-monitor-s3"
  role = aws_iam_role.monitor.id

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

// Fetch and setup default grafana adminpass
resource "aws_iam_role_policy" "monitor_ssm" {
  name = "${var.cluster_name}-monitor-ssm"
  role = aws_iam_role.monitor.id

  policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "ssm:GetParameters",
                "ssm:GetParametersByPath",
                "ssm:GetParameter",
                "ssm:PutParameter"
            ],
            "Resource": "arn:aws:ssm:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:parameter/teleport/${var.cluster_name}/grafana_pass"
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

