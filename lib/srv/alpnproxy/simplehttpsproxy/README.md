# Simple HTTPS proxy

A simple forward proxy that can be used as `HTTPS_PROXY` for testing.

Usage:
```
go run . -h
```

Listen localhost:
```
go run . -l localhost:18888
```

Route requests to a private ip:
```
go run . -r example.com:443=10.0.0.20:443
```
