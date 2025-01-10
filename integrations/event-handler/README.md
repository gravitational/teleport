# Teleport-event-handler

This plugin is used to export Audit Log events to Fluentd service.

## Usage

See the [Export Events with FluentD Guide](https://goteleport.com/docs/management/export-audit-events/fluentd/).

## How it works

* `teleport-event-handler` takes the Audit Log event stream from Teleport. It loads events in batches of 20 by default. Every event gets sent to fluentd.
* Once event is successfully received by fluentd, its ID is saved to the `teleport-event-handler` state. In case `teleport-event-handler` crashes, it will pick the stream up from a latest successful event.
* Once all events are sent, `teleport-event-handler` starts polling for new evetns. It happens every 5 seconds by default.
* If storage directory gets lost, you may specify latest event id value. `teleport-event-handler` will pick streaming up from the next event after it.

## Configuration options

You may specify configuration options via command line arguments, environment variables or TOML file.

| CLI arg name              | Description                                                                                           | Env var name                    |
|---------------------------|-------------------------------------------------------------------------------------------------------|---------------------------------|
| teleport-addr             | Teleport host and port                                                                                | FDFWD_TELEPORT_ADDR             |
| teleport-ca               | Teleport TLS CA file                                                                                  | FDFWD_TELEPORT_CA               |
| teleport-cert             | Teleport TLS certificate file                                                                         | FDWRD_TELEPORT_CERT             |
| teleport-key              | Teleport TLS key file                                                                                 | FDFWD_TELEPORT_KEY              |
| teleport-identity         | Teleport identity file                                                                                | FDFWD_TELEPORT_IDENTITY         |
| teleport-refresh-enabled  | Controls if the identity file should be reloaded from disk after the initial start on interval.       | FDFWD_TELEPORT_REFRESH_ENABLED  |
| teleport-refresh-interval | How often to load the identity file from disk when teleport-refresh-enabled is specified. Default: 1m | FDFWD_TELEPORT_REFRESH_INTERVAL |
| fluentd-url               | Fluentd URL                                                                                           | FDFWD_FLUENTD_URL               |
| fluentd-session-url       | Fluentd session URL                                                                                   | FDFWD_FLUENTD_SESSION_URL       |
| fluentd-ca                | fluentd TLS CA file                                                                                   | FDFWD_FLUENTD_CA                |
| fluentd-cert              | Fluentd TLS certificate file                                                                          | FDFWD_FLUENTD_CERT              |
| fluentd-key               | Fluentd TLS key file                                                                                  | FDFWD_FLUENTD_KEY               |
| storage                   | Storage directory                                                                                     | FDFWD_STORAGE                   |
| batch                     | Fetch batch size                                                                                      | FDFWD_BATCH                     |
| types                     | Comma-separated list of event types to forward                                                        | FDFWD_TYPES                     |
| skip-event-types              | Comma-separated list of event types to skip                                                           | FDFWD_SKIP_EVENT_TYPES              |
| skip-session-types        | Comma-separated list of session event types to skip                                                   | FDFWD_SKIP_SESSION_TYPES        |
| start-time                | Minimum event time (RFC3339 format)                                                                   | FDFWD_START_TIME                |
| timeout                   | Polling timeout                                                                                       | FDFWD_TIMEOUT                   |
| cursor                    | Start cursor value                                                                                    | FDFWD_CURSOR                    |
| debug                     | Debug logging                                                                                         | FDFWD_DEBUG                     |

TOML configuration keys are the same as CLI args. Teleport and Fluentd variables can be grouped into sections. See [example TOML](example/config.toml). You can specify TOML file location using `--config` CLI flag.

You could use `--dry-run` argument if you want event handler to simulate event export (it will not connect to Fluentd). `--exit-on-last-event` can be used to terminate service after the last event is processed.

`--skip-session-types` is `['print']` by default. Please note that if you enable forwarding of print events (`--skip-session-types=''`) the `Data` field would also be sent.

## Advanced topics

### Generate mTLS certificates using OpenSSL/LibreSSL

For the purpose of security, we require mTLS to be enabled on the fluentd side. You are going to need [OpenSSL configuration file](example/ssl.conf). Put the following contents to `ssl.conf`:

```sh
[req]
default_bits        = 4096
distinguished_name  = req_distinguished_name
string_mask         = utf8only
default_md          = sha256
x509_extensions     = v3_ca

[req_distinguished_name]
countryName                     = Country Name (2 letter code)
stateOrProvinceName             = State or Province Name
localityName                    = Locality Name
0.organizationName              = Organization Name
organizationalUnitName          = Organizational Unit Name
commonName                      = Common Name
emailAddress                    = Email Address

countryName_default             = US
stateOrProvinceName_default     = USA
localityName_default            =
0.organizationName_default      = Teleport
commonName_default              = localhost

[v3_ca]
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid:always,issuer
basicConstraints = critical, CA:true, pathlen: 0
keyUsage = critical, cRLSign, keyCertSign

[client_cert]
basicConstraints = CA:FALSE
nsCertType = client, email
nsComment = "OpenSSL Generated Client Certificate"
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid,issuer
keyUsage = critical, nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth, emailProtection

[server_cert]
basicConstraints = CA:FALSE
nsCertType = server
nsComment = "OpenSSL Generated Server Certificate"
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid,issuer:always
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = DNS:localhost,IP:127.0.0.1

[crl_ext]
authorityKeyIdentifier=keyid:always

[ocsp]
basicConstraints = CA:FALSE
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid,issuer
keyUsage = critical, digitalSignature
extendedKeyUsage = critical, OCSPSigning
```

Generate certificates using the following commands:

```sh
openssl genrsa -out ca.key 4096
chmod 444 ca.key
openssl req -config ssl.conf -key ca.key -new -x509 -days 7300 -sha256 -extensions v3_ca -subj "/CN=ca" -out ca.crt

openssl genrsa -aes256 -out server.key 4096
chmod 444 server.key
openssl req -config ssl.conf -subj "/CN=server" -key server.key -new -out server.csr
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -days 365 -out server.crt -extfile ssl.conf -extensions server_cert

openssl genrsa -out client.key 4096
chmod 444 client.key
openssl req -config ssl.conf -subj "/CN=client" -key client.key -new -out client.csr
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key -CAcreateserial -days 365 -out client.crt -extfile ssl.conf -extensions client_cert
```

You will be requested to enter key password. Remember this password since it will be required later, in fluentd configuration. Note that for the testing purposes we encrypt only `server.key` (which is fluentd instance key). It is strongly recommended by the Fluentd. Plugin does not yet support client key encryption.

Alternatively, you can run: `PASS=12345678 KEYLEN=4096 make gen-example-mtls` from the plugin source folder. Keys will be generated and put to `example/keys` folder.
