resource "teleport_provision_token" "token" {
  version = "v2"
  metadata = {
    name        = "gitlab-test-terraform"
    description = ""
  }

  spec = {
    roles       = ["Bot"]
    join_method = "gitlab"
    bot_name    = "gitlab-bot"
    gitlab = {
      domain = "bug.report"
      allow = [
        {
          project_path = "my-repo"
        }
      ]
    }
  }
}
