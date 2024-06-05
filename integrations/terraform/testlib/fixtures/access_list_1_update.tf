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
      traits = [{
        key    = "allowed-machines"
        values = ["crane", "forklift"]
      }]
    }
    title = "Hello"
    audit = {
      recurrence = {
        frequency = 3
        // changing day of the month should not change the next audit date
        // it should take effect after the next review
        day_of_month = 15
      }
    }
  }
}
