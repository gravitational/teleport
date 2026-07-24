resource "teleport_access_list" "member_test" {
  header = {
    version = "v1"
    metadata = {
      name = "test-member-list"
    }
  }
  spec = {
    type        = "static"
    title       = "Member test"
    description = "Access list member Terraform test"
    owners = [
      {
        name = "gru"
      }
    ]
    grants = {
      roles = ["crane-operator"]
    }
  }
}

resource "teleport_access_list_member" "fighter" {
  header = {
    version = "v1"
    metadata = {
      name = "fighter"
      labels = {
        class = "veteran"
      }
    }
  }
  spec = {
    access_list     = teleport_access_list.member_test.id
    membership_kind = 1
    reason          = "joined the party"
    expires         = "2038-02-01T00:00:00Z"
  }
}
