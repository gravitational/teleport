## render-helm-ref

`render-helm-ref` reads a Helm chart `values.yaml` and generates a chart reference.
This reference can then be included in our user documentation.

### How to use

```shell
go run ./cmd/render-helm-ref/ \
  -chart ../../examples/chart/teleport-cluster/charts/teleport-operator \
  -output ../../docs/pages/reference/helm-reference/zz_generated.teleport-operator.mdx
```

### `values.yaml` syntax

See [the test data `values.yaml`](./testdata/values.yaml) for an example input,
and [its expected output](./testdata/values.yaml)

### Why?

- to stop maintaining twice the documentation (and because everyone forgets to
  update either the values or the docs)
- to reduce the time-to-merge of Helm PRs (we need at least 1 round-trip to have
  folks fix the docs)
- to make sure all informnation is available everywhere. Some users only read
  the docs, other read the values. Without an automated sync, both are not
  providing the same information.

### What about existing tools?

- [helm-docs](https://github.com/norwoodj/helm-docs) almost did the trick, but
  its only output is a Markdown table. Its `pkg` is not reusable because it
  relies to heavily on viper. We would also need to specify `@raw` everywhere,
  which is cumbersome.
- [bitnami/readme-generator-for-helm](https://github.com/bitnami/readme-generator-for-helm)
  would need to be extended to support non-table outputs. However, it is developed
  in NodeJS while all our existing tooling is in go.
