resource "teleport_access_list" "characters" {
  header = {
    version = "v1"
    metadata = {
      name = "characters"
    }
  }
  spec = {
    type        = "static" # the access list must be of type "static" to manage its members with Terraform
    title       = "Characters"
    description = "The list of game characters."
    owners = [
      { name = "dungeon_master" },
    ]
    grants = {
      roles = ["dungeon_access"]
    }
  }
}

# User member:

resource "teleport_access_list_member" "fighter" {
  header = {
    version = "v1"
    metadata = {
      name = "fighter" # Teleport user name
    }
  }
  spec = {
    access_list     = teleport_access_list.characters.id
    membership_kind = 1 # 1 for "MEMBERSHIP_KIND_USER", 2 for "MEMBERSHIP_KIND_LIST"
  }
}

# Nested Access List member:

resource "teleport_access_list" "npcs" {
  header = {
    version = "v1"
    metadata = {
      name = "npcs"
    }
  }
  spec = {
    title       = "NPCs"
    description = "Non-player characters."
    owners = [
      { name = "dungeon_master" }
    ]
    grants = {
      roles = ["dungeon_access"]
    }
    audit = {
      recurrence = {
        frequency    = 3
        day_of_month = 15
      }
    }
  }
}

resource "teleport_access_list_member" "npcs" {
  header = {
    version = "v1"
    metadata = {
      name = teleport_access_list.npcs.id
    }
  }
  spec = {
    access_list     = teleport_access_list.characters.id
    membership_kind = 2 # 1 for "MEMBERSHIP_KIND_USER", 2 for "MEMBERSHIP_KIND_LIST"
  }
}
