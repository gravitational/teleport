resource "teleport_role" "myrole" {
  metadata = {
    name = "test"
  }

  spec = {
    allow = {
      logins = ["anonymous"]
    }
  }

  version = "v6"
}


resource "teleport_github_connector" "test" {
  version = "v3"
  metadata = {
    name    = "test"
    expires = "2032-10-12T07:20:50Z"
    labels = {
      example = "yes"
    }
  }

  spec = {
    client_id     = "Iv1.3386eee92ff932a4"
    client_secret = "secret"

    teams_to_roles = [{
      organization = "evilmartians"
      team         = "devs"
      roles        = ["myrole"]
    }]
  }
}
