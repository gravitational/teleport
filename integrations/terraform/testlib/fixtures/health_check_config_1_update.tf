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
    interval            = "45s"
    timeout             = "7s"
    healthy_threshold   = 2
    unhealthy_threshold = 1
    match = {
      db_labels = [{
        name = "env"
        values = [
          "prod",
        ]
      }]
      db_labels_expression = "labels.foo == `baz`"
    }
  }
}
