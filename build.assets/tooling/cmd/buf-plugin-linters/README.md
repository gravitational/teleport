# `buf-plugin-linters`

`buf-plugin-linters` is a `buf`[1] plugin for linting `.proto` files for APIs that should
confirm to Teleport resource guidelines for pagination[2].

## Usage

To enable the plugin in `buf.yaml`:

```yaml
version: v2

lint:
  use:
    - PAGINATION_REQUIRED
plugins:
  - plugin:
      - env
      - GOWORK=off
      - go
      - -C
      - ./build.assets/tooling
      - run
      - ./cmd/buf-plugin-linters
```

See `default.go` for the default configuration. 

[1]: http://buf.build/
[2]: https://github.com/gravitational/teleport/blob/master/rfd/0153-resource-guidelines.md#list
