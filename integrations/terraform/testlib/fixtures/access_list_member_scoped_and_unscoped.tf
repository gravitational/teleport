resource "teleport_access_list" "unscoped" {
  header = {
    version = "v1"
    metadata = {
      name = "test-unscoped-member-list"
    }
  }
  spec = {
    type        = "static"
    title       = "Unscoped member list"
    description = "unscoped access list member test"
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

resource "teleport_access_list" "scoped" {
  depends_on = [teleport_access_list.unscoped]

  header = {
    version = "v1"
    metadata = {
      name = "test-scoped-member-list"
    }
  }
  scope = "/foo/bar"
  spec = {
    type        = "static"
    title       = "Scoped member list"
    description = "scoped access list member test"
    owners = [
      {
        name = "gru"
      }
    ]
  }
}

resource "teleport_access_list" "scoped_child" {
  depends_on = [teleport_access_list.scoped]

  header = {
    version = "v1"
    metadata = {
      name = "test-scoped-child-member-list"
    }
  }
  scope = "/foo/bar"
  spec = {
    type        = "static"
    title       = "Scoped child member list"
    description = "scoped child access list member test"
    owners = [
      {
        name = "gru"
      }
    ]
  }
}

resource "teleport_access_list_member" "unscoped" {
  depends_on = [teleport_access_list.scoped_child]

  header = {
    version = "v1"
    metadata = {
      name = "unscoped-fighter"
    }
  }
  spec = {
    access_list     = teleport_access_list.unscoped.id
    membership_kind = 1
    reason          = "unscoped member"
    expires         = "2038-03-01T00:00:00Z"
  }
}

resource "teleport_access_list_member" "scoped" {
  depends_on = [teleport_access_list_member.unscoped]

  header = {
    version = "v1"
    metadata = {
      name = "scoped-fighter"
    }
  }
  scope = "/foo/bar"
  spec = {
    access_list     = teleport_access_list.scoped.id
    membership_kind = 1
    reason          = "scoped member"
    expires         = "2038-03-01T00:00:00Z"
  }
}

resource "teleport_access_list_member" "scoped_list" {
  depends_on = [teleport_access_list_member.scoped]

  header = {
    version = "v1"
    metadata = {
      name = teleport_access_list.scoped_child.id
    }
  }
  scope = "/foo/bar"
  spec = {
    access_list     = teleport_access_list.scoped.id
    membership_kind = 3
    reason          = "scoped list member"
    expires         = "2038-03-01T00:00:00Z"
  }
}
