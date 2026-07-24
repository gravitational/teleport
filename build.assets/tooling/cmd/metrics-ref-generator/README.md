# Metrics reference generator

The metrics reference generator is a Go program that produces a
comprehensive reference guide for all available Teleport metrics.
It uses the Teleport source as the basis for the guide.

## Usage

From the root of your `gravitational/teleport` clone:

```
$ make gen-metrics-docs
```

## How it works

The metrics reference generator works by:

1. Identifying Prometheus metric registrations from the Teleport source.
1. Grouping the identified metrics into sections based on the provided config file.

## Configuration

The generator uses a YAML configuration file with the following fields.

### Main config

- `source` (string): the path to the root of a Go project directory.

- `destination` (string): output file path of `metrics.mdx`.

- `introduction` (string): optional markdown rendered at the top of the page, before the generated metrics reference.

- `components` (array of component configuration objects): optional ordered mappings from metric name prefixes to component names. The first matching mapping is used. Used to populate the "Component" column for the field tables.

- `sections` (array of section configuration objects): optional groupings for the generated page. Metrics that do not match any section's filters are placed in an implicit "Other" section.

### Component configuration

- `name` (string): the name of the component.
- `filters` (array of strings): Prefixes matched against metric full names. For example, a `teleport_test` filter matches `teleport_test_example_metric`. A metric can appear in more than one section when their filters overlap.

### Section configuration

Each item in the `sections` array has the following object structure:

- `title` (string): The human-readable name of the section. Used as the section heading in the output.
- `description` (string): Optional text rendered below the section heading.
- `component` (string): Optional component applied to generated metrics in this section. This overrides global component mappings and allows the same metric to have context-specific components in different sections.
- `filters` (array of strings): Prefixes matched against metric full names.
- `metrics` (array of metric configuration objects): Optional explicit metric rows for metrics that are not declared in the scanned Go source. Each row has `name`, `type`, `component`, and `description` fields. A filtered source metric takes precedence when it has the same name.
- `sections` (array of section configuration objects): Optional nested sections. A section with nested sections and no filters acts as a grouping section and does not include a metrics table of its own.

Components are resolved in this order: an explicit component on a configured metric row, the section component, the first matching global component mapping, and finally the Prometheus subsystem or namespace.

### Example

```yaml
source: "../../../../lib"
destination: "../../../../docs/pages/reference/deployment/monitoring/metrics.mdx"
introduction: |
  Teleport metrics are intended for performance monitoring.
components:
  - name: Teleport Auth
    filters:
      - auth_
      - teleport_registered_
  - name: Teleport Proxy
    filters:
      - proxy_
sections:
  - title: Auth Service
    component: Teleport Auth
    filters:
      - auth_
      - teleport_registered_
    metrics:
      - name: grpc_server_started_total
        type: counter
        component: Teleport Auth
        description: Total number of RPCs started on the server.
  - title: Proxy Service
    filters:
      - proxy_
```
