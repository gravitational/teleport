## Demo

To run this program:

**Setup TLS**

Alongisde a running teleport auth server, run:

```
tctl auth export --type=tls > /var/lib/teleport/ca.cert
```

**Execute**


```bash
go get github.com/gravitational/teleport/lib/auth
go run main.go
```

