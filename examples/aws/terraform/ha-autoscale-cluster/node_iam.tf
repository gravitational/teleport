// Node isntance profile and roles
resource "aws_iam_role" "node" {
  name = "${var.cluster_name}-node"

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

// Nodes use SSM to fetch join tokens to join the cluster.
// Join tokens are security tokens published and rotated by auth server nodes.
// Note that nodes are only allowed to read node SSM path.
resource "aws_iam_instance_profile" "node" {
  name       = "${var.cluster_name}-node"
  role       = aws_iam_role.node.name
  depends_on = [aws_iam_role_policy.node_ssm]
}

resource "aws_iam_role_policy" "node_ssm" {
  name = "${var.cluster_name}-node-ssm"
  role = aws_iam_role.node.id

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
            "Resource": "arn:aws:ssm:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:parameter/teleport/${var.cluster_name}/tokens/node"
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

