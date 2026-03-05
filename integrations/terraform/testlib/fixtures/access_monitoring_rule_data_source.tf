data "teleport_access_monitoring_rule" "test" {
  kind    = "access_monitoring_rule"
  version = "v1"
  metadata = {
    name = "test"
  }
  spec = {
    subjects = []
  }
}
