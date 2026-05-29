resource "teleport_role" "with_options" {
  version = "v8"
  metadata = {
    name = "with_options"
  }

  spec = {
    options = {
      record_session = {
        default = "strict"
        desktop = false
        ssh     = "strict"
      }
    }
  }
}
