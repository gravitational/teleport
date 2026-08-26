resource "teleport_access_list" "test" {
  header = {
    version = "v1"
    metadata = {
      name = "test"
      labels = {
        example = "yes"
      }
    }
  }
  spec = {
    description = "test description"
    owners = [
      {
        name        = "gru"
        description = "The supervillain."
      }
    ]
    membership_requires = {
      roles = ["minion"]
    }
    ownership_requires = {
      roles = ["supervillain"]
    }
    grants = {
      roles = ["crane-operator"]
    }
    title = "Hello"
    audit = {
      recurrence = {
        frequency = 3
      }
    }
  }
}
