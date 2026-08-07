# Library Helm Charts

This directory contains Helm Library charts used by multiple Application (i.e.
"normal") charts. Library charts are kept here to distinguish them from those
meant to be installed by end users.

## What is a library chart?

A [Library Chart](https://helm.sh/docs/topics/library_charts/) is one that has
`type: library` in `Chart.yaml`. Such charts behave differently in two ways:

1. Helm will actively refuse to install them.

   ```
   $ helm install my-release .
   Error: INSTALLATION FAILED: library charts are not installable
   ```

2. They render no templates

   Helm still registers the chart's named templates (`{{ define ... }}`) into
   the **parent chart's** global scope, but it does not render the library
   chart's own manifest templates. Files that don't start with `_` are ignored.

## Interface Contract

Library charts defined here should define
[Named Templates](https://helm.sh/docs/chart_template_guide/named_templates/)
designed to take a copy of the calling chart's root context.

```gotmpl
{{- include "my-lib-chart.my-template" . -}}
```

This follows patterns established by:

- Helm docs'
  [Common Helper Chart Example](https://helm.sh/docs/topics/library_charts/#the-common-helm-helper-chart)
- Other common open source repos
  ([1](https://github.com/bitnami/charts/tree/main/bitnami/common),
  [2](https://github.com/bjw-s-labs/helm-charts/tree/main/charts/library/common)).

This means:

- Parent charts should define top-level values the library chart's templates
  consume

  ```yaml
  # parent/values.yaml

  # Correct!
  myLibChartValue: foo

  ## WRONG! Don't Copy ME
  my-lib-chart:
    myLibChartValue: bar
  ```

- Library chart templates should consume them at:
  - `.Values.myLibChartValue`

  **_NOT_**:
  - `.myLibChartValue` # WRONG, DON'T COPY
  - `.Values.my-lib-chart.myLibChartValue` # WRONG, DON'T COPY

### Value Defaults / Schema Validation

Library charts' default values and schema validation **are** processed just like
subchart dependencies. This means that _if_ a library chart has a `values.yaml`
and/or `values.schema.json`, these will apply to the parent chart at
`.Values.<subkey>.*`, **not** at the root level. This doesn't follow the pattern
above, nor does it allow the parent chart to deviate from the dependent library
chart when it needs different validation (i.e. if the parent chart manipulates
the context passed to the library chart's named templates, thereby making the
library chart's validation invalid). Library charts should therefore generally
**not** define values or schema.

This does mean that multiple charts that depend on the same library chart will
likely duplicate values and schema for the library chart. Helm has no great way
to move that responsibility to library charts without limiting how the parent
chart can define its required values.

Library charts can instead validate their inputs at render-time by having all
public named templates call an internal validation template.

```gotmpl
{{- define "my-lib-chart.my-template" -}}
{{- include "my-lib-chart.internal.validate" . -}}
...
{{- end -}}

{{- define "my-lib-chart.internal.validate" -}}
{{- if not (and (hasKey .Values "myRequiredValue") (kindIs "string" .Values.myRequiredValue)) -}}
{{- fail "'myRequiredValue' is required" -}}
{{- end -}}
...
{{- end -}}
```

#### Value Merging

It's possible to merge library chart values from the parent chart's root values
and the library chart's subkey with an init helper called by library chart
templates
([example](https://github.com/bjw-s-labs/helm-charts/blob/main/charts/library/common/templates/values/_init.tpl))
to define values at multiple levels. However, including values and schema in the
library chart still only applies defaults and validation to values defined under
the library chart's subkey (`.Values.my-lib-chart`), so something like
`.Values.myLibChartValue` still needs independent validation by the parent's
schema.

## Documentation

A library chart's public interface is its public (non-`internal`) named
templates and the `.Values` keys they read. Document that interface in a
`README.md` at the **chart root** (e.g. `lib/teleport-proxy-lib/README.md`). It
should list each public template, how to call it, and the values it requires —
plus any values the consuming chart must compute and inject.

Do **not** rely on `values.yaml` or `values.schema.json` for this. As described
under "Value Defaults / Schema Validation" above, Helm applies a library chart's
`values.yaml`/schema at `.Values.<subkey>`, not where the templates read, so
they cannot serve as the interface contract.

## Testing

Because they don't render templates, library charts can't be tested using helm
unittest. To work around this, library charts should define an application chart
as a test fixture/helper in the `test-chart/` directory under their root. This
chart should include the library chart with a symlink
(`my-lib-chart/test-chart/charts/my-lib-chart` -> `../../`). It should define
templates and unit tests that call the library chart templates. Don't forget to
add **both** the library chart **AND** test-chart to Helm Janitor
([teleport/build.assets/tooling/cmd/helm-janitor/main.go](https://github.com/gravitational/teleport/blob/ae0e1cc4785cd9246c61d154c706433a12460f86/build.assets/tooling/cmd/helm-janitor/main.go#L51)).

## Scoping & Naming

All named templates defined in a library chart are exported into the parent's
global namespace. All named templates should therefore be prefixed with the
library chart's name to avoid naming collisions:

```gotmpl
{{- define "my-lib-chart.my-template" -}}
```

### Kebab-case vs. camelCase

Use **kebab-case for chart names and named-template names**, and **camelCase for
values/field keys**. The two never collide:

- Named-template names are always quoted strings in `define`/`include`
  (`{{ include "my-lib-chart.my-template" . }}`), so a `-` is just a character
  in a string — it is never parsed as an identifier.
- Value keys are read with dot notation (`.Values.myValue`). Go templates parse
  `-` as subtraction, so a dashed key like `.Values.my-value` won't work — you'd
  have to write `index .Values "my-value"`. camelCase keeps dot access working.

### Abstraction (if you must)

Helm **does** allow naming collisions, in which case the parent chart's version
will be rendered. This can be used as a form of abstraction:

```gotmpl
# my-lib-chart/templates/_helpers.tpl
{{- define "my-lib-chart.my-template" -}}
{{ fail "REDEFINE ME IN THE CALLING CHART PLEASE." }}
{{- end -}}
```

```gotmpl
# parent/templates/_helpers.tpl
{{- define "my-lib-chart.my-template" -}}
A concrete implementation of my-template in parent chart.
{{- end -}}
```

This is acceptable, but should be well documented.

### Internal / Private Named Templates

Helm has no way to protect / privatize some subset of a library chart's
templates. To distinguish the library chart's public interface from templates
not intended for consumption, "private" templates should have an `.internal`
component to their name:

```gotmpl
{{- define "my-lib-chart.internal.my-internal-template" -}}
```
