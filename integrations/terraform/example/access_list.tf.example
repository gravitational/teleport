resource "teleport_access_list" "crane-operation" {
  header = {
    metadata = {
      name = "crane-operation"
      labels = {
        example = "yes"
      }
    }
  }
  spec = {
    description = "Used to grant access to the crane."
    owners = [
      {
        name = "gru"
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
        key = "allowed-machines"
        values = ["crane", "forklift"]
      }]
    }
    title = "Crane operation"
    audit = {
      recurrence = {
        frequency = 3 # audit every 3 months
        day_of_month = 15 # audit happen 15's day of the month. Possible values are 1, 15, and 31.
      }
    }
  }
}
