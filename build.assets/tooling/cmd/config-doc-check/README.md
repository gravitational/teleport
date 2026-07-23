# `config-doc-checker`

`config-doc-check` is a Go program that checks for coverage gaps in the Teleport configuration reference documentation examples: the YAML objects are compared to their corresponding Go structs. It reports any fields missing from the examples as well as fields that no longer exist in the source.

## Usage

From the root of your `gravitational/teleport` clone:

```
$ make docs-check-config-coverage
```

## Configuration

`config-doc-check` reads a YAML configuration file specified with the `-config` flag.

### Main configuration

- `source_path` (string): Path to the root of the Teleport source repository.
- `service_sections` (array of service section configuration objects): The configuration reference sections that should be checked.

### Service section configuration

- `name`: Human-readable name displayed in the checker output.
- `example_path`: Path to the example YAML file, relative to `source_path`.
- `key_type_pairs`: Each object contains a `section_key` top-level YAML key (e.g. `jamf_service`) and its corresponding `type_name` struct declaration, (e.g. `JamfService`).
- `dismissed_keys`: Exact YAML key paths that should be dismissed by the checker. A leaf path such as `teleport.storage.type` dismisses only that leaf. A parent path such as `db_service.resources` dismisses that node and its descendants. For example, deprecated fields that exist in the struct only for backward compatibility should be dismissed.

Example:

```yaml
service_sections:
  - name: Test Service
    example_path: docs/pages/includes/config-reference/test_service.yaml
    key_type_pairs:
      - section_key: test_service
        type_name: TestService
      - section_key: teleport
        type_name: Global
    dismissed_keys:
      - test_service.storage.type # deprecated
```
