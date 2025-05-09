resource "teleport_health_check_config" "test" {
  metadata = {
    name        = "test"
    description = "Example health check config"
    labels = {
      foo = "bar"
    }
  }
  version = "v1"
  spec = {
    interval            = "60s"
    timeout             = "5s"
    healthy_threshold   = 3
    unhealthy_threshold = 2
    match = {
      db_labels = [{
        name = "inEnv"
        values = [
          "foo",
          "bar",
        ]
      }]
      db_labels_expression = "labels.foo == `bar`"
    }
  }
}
