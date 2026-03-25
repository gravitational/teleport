# Deepcopy Patch Workflow

This repository vendors `deepcopy.go` from [goderive](https://github.com/awalterschulze/goderive) and applies a custom patch to support `*time.Time` deep copy.

## Workflow

1. **Fetch upstream `deepcopy.go`**
```bash
$ GODERIVE_VERSION=$(go list -m -f '{{.Version}}' github.com/awalterschulze/goderive)
$ curl -fsSL "https://raw.githubusercontent.com/awalterschulze/goderive/refs/tags/${GODERIVE_VERSION}/plugin/deepcopy/deepcopy.go" -o deepcopy.go
```

2. **Apply custom patch**
```bash
$ patch deepcopy.go < 0001-add-time-deepcopy-support.patch
```
