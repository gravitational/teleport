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
    description = "updated next audit date test"
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
      // Terraform should not clear or recompute the server-maintained next audit date during update.
      next_audit_date = null
      recurrence = {
        frequency    = 3
        day_of_month = 15
      }
    }
  }
}
