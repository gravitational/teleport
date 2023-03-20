Tested with a `tbot` producing intentionally short-lived certificates to trigger
aggressive expiry.

```
build/tctl bots add expiry-test --roles=root,noahstride --roles access
```

```
build/tbot start \
   --destination-dir=./scratch/tbot-user \
   --data-dir=./scratch/tbot-data \
   --token=$BOT_TOKEN \
   --auth-server=root.tele.ottr.sh:443 \
   --certificate-ttl=3m \
   --renewal-interval=1m
```

```
go run ./examples/go-client
```

Need to test with auto disconnect expired certs or inject network failures.