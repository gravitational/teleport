resource "teleport_workload_cluster" "example" {
  version = "v1"
  metadata = {
    name = "example"
  }
  spec = {
    regions = [
      {
        name = "us-west-2"
      },
    ]

    bot = {
      name = "onboarding"
    }

    token = {
      join_method = "iam"

      allow = [
        {
          aws_account = "333333333333"
          aws_arn     = "arn:aws:sts::333333333333:assumed-role/my-role-name/my-role-session-name"
        },
      ]
    }
  }
}

