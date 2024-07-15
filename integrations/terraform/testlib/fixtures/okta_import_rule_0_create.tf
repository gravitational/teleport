resource "teleport_okta_import_rule" "test" {
  version = "v1"
  metadata = {
    name        = "example"
    description = "Test Okta Import Rule"
    labels = {
      example               = "yes"
      "teleport.dev/origin" = "okta"
    }
  }

  spec = {
    priority = 100
    mappings = [
      {
        add_labels = {
          "label1" : "value1",
        }
        match = [
          {
            app_ids = ["1", "2", "3"]
          },
        ],
      },
      {
        add_labels = {
          "label2" : "value2",
        }
        match = [
          {
            group_ids = ["1", "2", "3"]
          },
        ],
      },
      {
        add_labels = {
          "label3" : "value3",
        }
        match = [
          {
            group_name_regexes = ["^.*$"]
          },
        ],
      },
      {
        add_labels = {
          "label4" : "value4",
        }
        match = [
          {
            app_name_regexes = ["^.*$"]
          },
        ],
      }
    ]
  }
}
