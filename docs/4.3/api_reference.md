# Teleport API Reference

Teleport is currently working on documenting our API.

!!! warning

        We are currently working on this project. If you have an API suggestion, [please complete our survey](https://docs.google.com/forms/d/1HPQu5Asg3lR0cu5crnLDhlvovGpFVIIbDMRvqclPhQg/edit).

## Authentication
In order to interact with the Access Request API, you will need to provision appropriate
TLS certificates. In order to provision certificates, you will need to create a
user with appropriate permissions:

```bash
$ cat > rscs.yaml <<EOF
kind: user
metadata:
  name: access-plugin
spec:
  roles: ['access-plugin']
version: v2
---
kind: role
metadata:
  name: access-plugin
spec:
  allow:
    rules:
      - resources: ['access_request']
        verbs: ['list','read','update']
    # teleport currently refuses to issue certs for a user with 0 logins,
    # this restriction may be lifted in future versions.
    logins: ['access-plugin']
version: v3
EOF
# ...
$ tctl create rscs.yaml
# ...
$ tctl auth sign --format=tls --user=access-plugin --out=auth
# ...
```

The above sequence should result in three PEM encoded files being generated:
`auth.crt`, `auth.key`, and `auth.cas` (certificate, private key, and CA certs respectively).

Note: by default, tctl auth sign produces certificates with a relatively short lifetime.
For production deployments, the --ttl flag can be used to ensure a more practical
certificate lifetime.

# gRPC APIs

## Audit Events API
Coming Soon

## Certificate Generation API
Coming Soon

## Tokens API
Coming Soon

## Workflow API
Coming Soon