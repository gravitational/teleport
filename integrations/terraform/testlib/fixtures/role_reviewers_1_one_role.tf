resource "teleport_role" "test_decrease_reviewers" {
  metadata = {
    name = "test_decrease_reviewers"
  }

  spec = {
    allow = {
      logins = ["anonymous"]
      review_requests = {
        roles = ["roleb"]
      }
    }
  }

  version = "v6"
}
