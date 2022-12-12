# teleport-stats

This tool displays GitHub API stats of `gravitational/teleport` repo.

## Build

```
go build
```

## Display checks stats

Tool iterates over last n merged pull requests, gets checks for each merge commit and displays the result.

To get stats on latest 100 checks, and display 10 latest failures by each check:

```
GITHUB_TOKEN=<token> ./stats 100 -f 10
```

### Update generated GraphQL go code

```
go run github.com/Khan/genqlient
```