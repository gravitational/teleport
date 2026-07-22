# `config-doc-checker`

`config-doc-check` is a Go program that ensures that the Teleport configuration reference documentation examples cover all of the YAML fields defined by the corresponding Go structs. It reports any fields missing from the examples and fields that no longer exist in the source configuration.

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
`example_path`: Path to the example YAML file, relative to `source_path`.
`section_key`: Top-level YAML key containing the service configuration.
`type_name`: The name of the struct type declaration that represents the configuration section, e.g. `JamfService`.
