resource "teleport_access_list" "member_defaults" {
  header = {
    version = "v1"
    metadata = {
      name = "member_defaults"
    }
  }
  spec = {
    type  = "static"
    title = "Hello"
    owners = [
      {
        name = "gru"
      }
    ]
  }
}

resource "teleport_access_list_member" "member_defaults" {
  header = {
    version = "v1"
    metadata = {
      name = "member_defaults"
    }
  }
  spec = {
    access_list     = teleport_access_list.member_defaults.id
    membership_kind = 1
    reason          = "example reason"
    expires         = "2026-07-11T20:00:00Z"
  }
}
