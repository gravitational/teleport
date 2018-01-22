// EFS is NFS from Amazon, used by Auth servers
// to store session replays and audit logs.
resource "aws_efs_file_system" "auth" {
  creation_token = "${var.cluster_name}"
  performance_mode = "${var.performance_mode}"

  tags {
    TeleportCluster = "${var.cluster_name}"
  }
}

// Security group rule allows access to NFS port
resource "aws_security_group" "efs" {
  name   = "${var.cluster_name}-efs"
  vpc_id = "${local.vpc_id}"
  tags {
    TeleportCluster = "${var.cluster_name}"
  }
}

resource "aws_security_group_rule" "ingress_allow_internal_nfs" {
  type              = "ingress"
  from_port         = 0
  to_port           = 2049
  protocol          = "-1"
  security_group_id = "${aws_security_group.efs.id}"
  // allow auth servers to mount EFS volumes
  source_security_group_id = "${aws_security_group.auth.id}"
}

resource "aws_efs_mount_target" "auth" {
  count             = "${length(local.azs)}"
  file_system_id = "${aws_efs_file_system.auth.id}"
  subnet_id      = "${element(aws_subnet.auth.*.id, count.index)}"
  security_groups = ["${aws_security_group.efs.id}"]
}

variable "performance_mode" {
  type = "string"
  default = "generalPurpose"
}

