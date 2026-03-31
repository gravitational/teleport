The Teleport API allows providing an XML `entity_descriptor`, an `entity_id`, an
`acs_url`. However, the `entity_descriptor` contains both the `entity_id` and
`acs_url` values. The API disallows mutations if `entity_id` doesn't match the
copy in `entity_descriptor`. It does not check `acs_url`, but will use the copy
in `entity_descriptor` if they disagree. We therefore recommend using either
`entity_id` and `acs_url` or `entity_descriptor`, but not both. However, the
Terraform provider doesn't explicitly block using all 3 in order to match the
underlying API.

The API also rewrites `entity_descriptor` with values from `attribute_mapping`.
If they differ, this will cause the Terraform resource to be recreated on the
next apply. To avoid this, either use `attribute_mapping` with
`entity_id`+`acs_url` rather than `entity_descriptor`.

To prevent similar idempotency issues, the Terraform provider also requires full
URNs for the attribute mapping `name_format` fields.
