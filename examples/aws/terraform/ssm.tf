// SSM parameters are populated by default, and
// are here to make sure they will get deleted after cluster
// is destroyed, cluster will overwrite them with real values

resource "aws_ssm_parameter" "license" {
  name      = "/teleport/${var.cluster_name}/license"
  type      = "SecureString"
  value     = "${file(var.license_path)}"
  overwrite = true
}
