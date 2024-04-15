resource "teleport_access_list" "test" {
  header = {
    version = "v1"
    metadata = {
      name = "test"
      labels = {
        example = "yes"
      }
      expires = "2038-01-01T00:00:00Z"
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
      traits = [{
        key    = "allowed-machines"
        values = ["crane", "forklift"]
      }]
    }
    title = "Hello"
    audit = {
      recurrence = {
        frequency    = 3
        day_of_month = 15
      }
    }
  }
}
