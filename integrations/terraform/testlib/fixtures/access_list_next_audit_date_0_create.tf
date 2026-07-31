resource "teleport_access_list" "test" {
  header = {
    version = "v1"
    metadata = {
      name = "test-next-audit-date"
      labels = {
        example = "yes"
      }
    }
  }
  spec = {
    description = "next audit date test"
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
    title = "Next Audit Date"
    audit = {
      recurrence = {
        frequency = 3
      }
    }
  }
}
