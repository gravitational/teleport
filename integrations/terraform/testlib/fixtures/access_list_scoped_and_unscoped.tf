resource "teleport_access_list" "unscoped" {
  header = {
    version = "v1"
    metadata = {
      name = "test-unscoped"
    }
  }
  spec = {
    description = "unscoped access list"
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
    title = "Unscoped"
    audit = {
      recurrence = {
        frequency = 3
      }
    }
  }
}

resource "teleport_access_list" "scoped" {
  depends_on = [teleport_access_list.unscoped]

  header = {
    version = "v1"
    metadata = {
      name = "test-scoped"
    }
  }
  scope = "/foo/bar"
  spec = {
    description = "scoped access list"
    owners = [
      {
        name        = "gru"
        description = "The supervillain."
      }
    ]
    title = "Scoped"
    audit = {
      recurrence = {
        frequency = 3
      }
    }
  }
}
