resource "teleport_db_object_import_rule" "test" {
  version = "v1"
  metadata = {
    name = "my_custom_rule"
  }

  spec = {
    priority = 124
    database_labels = [
      {
        name   = "env"
        values = ["test", "staging"]
      },
      {
        name   = "dept"
        values = ["*"]
      }
    ]
    mappings = [
      {
        add_labels = {
          database              = "{{obj.database}}"
          object_kind           = "{{obj.object_kind}}"
          name                  = "{{obj.name}}"
          protocol              = "{{obj.protocol}}"
          schema                = "{{obj.schema}}"
          database_service_name = "{{obj.database_service_name}}"
          fixed                 = "const_value"
          template              = "foo-{{obj.name}}"
        }
        match = {
          table_names = [
            "fixed_table_name",
            "partial_wildcard_*"
          ]
        }
        scope = {
          database_names = ["Widget*"]
          schema_names   = ["public", "secret"]
        }
      },
    ]
  }
}