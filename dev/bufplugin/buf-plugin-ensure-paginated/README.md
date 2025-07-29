# `buf-plugin-ensure-paginated`

`buf-plugin-ensure-paginated` is a `buf`[1] plugin for linting `.proto` files for APIs that should
confirm to Teleport resource guidelines for pagination[2].

## Usage

To enable the plugin in `buf.yaml`:

```yaml
version: v2

lint:
  use:
    - PAGINATION_REQUIRED
plugins:
  - plugin: [./dev/bufplugin/run.sh, buf-plugin-ensure-paginated]
    options:
      prefixes: ['List', 'Search']
      size_names: ['page_size', 'limit']
      token_names: ['page_token', 'next_token']
      next_names: ['next_page_token', 'next_token', 'cursor']
```

## Options

| Name             | Default               | Function                                                                        |
| ---------------- | --------------------- | ------------------------------------------------------------------------------- |
| `prefixes`       | `["List", "Search"]`  | All RPCs prefixed with any of the listed strings will expect pagination fields. |
| `size_names`     | `["page_size"]`       | Valid request field names for page sizes.                                       |
| `token_names`    | `["page_token"]`      | Valid request field names for token.                                            |
| `next_names`     | `["next_page_token"]` | Valid response field name for next page token.                                  |
| `check_repeated` | `false`               | If set to true will check any RPC returning a message with `repeated` field.    |

[1]: http://buf.build/
[2]: https://github.com/gravitational/teleport/blob/master/rfd/0153-resource-guidelines.md#list
