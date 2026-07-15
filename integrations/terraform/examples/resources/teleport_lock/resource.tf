# Generate a random UUID to use as the lock name.
resource "random_uuid" "my_lock" {}

resource "teleport_lock" "my_lock" {
  version = "v2"
  metadata = {
    name        = random_uuid.my_lock.result
    description = "Ongoing incident investigation."
  }

  spec = {
    target = {
      user = "john"
    }
  }
}
