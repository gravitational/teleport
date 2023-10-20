---
authors: Nic Klaassen (nic@goteleport.com)
state: draft
---

# RFD 136 - Modern Signature Algorithms

## Required Approvers

* Engineering: (@jakule || @espadolini)
* Security: (@reedloden || @jentfoo) && Doyensec
* Product: @klizhentas

## What

This RFD proposes the adoption of a new set of asymmetric key types and
signature algorithms across all Teleport protocols that surpass the 2048-bit RSA
keys currently used in terms of security and performance.

## Why

Currently Teleport uses 2048-bit RSA keypairs for all internal Certificate
Authorities and all user and host keys.

More modern algorithms based on Elliptic Curve Cryptography offer better
security properties with smaller keys that offer better performance for keypair
generation and signing operations, with comparable performance for signature
verification.  Some of the more restrictive security policies are starting to
reject RSA2048 (e.g.  [RHEL 8's FUTURE policy](https://access.redhat.com/articles/3642912)).

## Details

### Summary

We will introduce the concept of signature algorithm "suites" that will
configure the key types and signature algorithms used across Teleport.
This will be a cluster-wide configuration available in both the `teleport.yaml`
config of Teleport Auth servers (so that initial CA keys generated at first
startup can be configured) and in the `cluster_auth_preference` (so that Cloud
users can optionally change the suite used in their cluster).

The initial set of suites we will introduce is:

* `legacy`
* `balanced-v1`
* `fips-v1`
* `hsm-v1`

The `legacy` suite exists for compatibility and it will use the exact same
key types and algorithms Teleport uses today (2048-bit RSA everywhere).

The `balanced-v1` suite will use a modern set of algorithms selected to
balance security, compatibility, and performance.
The proposed selection for this suite is to use Ed25519 as the default choice,
and ECDSA with the NIST P-256 curve in cases where Teleport is likely to
interact with third-party software that does not support Ed25519.
The exception is the database CA, where it looks like it will be necessary to
continue using RSA in order to continue supporting our current Snowflake
integration.
We will continue using 2048-bit keys wherever RSA is used, to avoid the
performance penalty of using larger keys.

The `fips-v1` suite will only use key types and signature algorithms that are
approved by FIPS 186-5 *and* are supported in Go's `GOEXPERIMENT=boringcrypto`
mode.
FIPS 186-5 actually approves all algorithms in the `balanced-v1` suite (and
even the `legacy` suite), but Go's BoringCrypto integration does NOT support
Ed25519.
This suite will be based off of the `balanced-v1`, but all uses of Ed25519 will
be replaced by ECDSA with the NIST P-256 curve.
Teleport will fail to start in FIPS mode unless the `fips-v1` or `legacy` suite
is selected.
The `fips-v1` algorithm suite will be available without FIPS mode, to enable
migration to FIPS mode.

The `hsm-v1` suite will be based off of the `balanced-v1` suite, but all uses of
Ed25519 *for CA keys only* will be replaced by ECDSA with the NIST P-256 curve.
This is necessary and sufficient for all of the HSMs and KMSs we currently
support and test for (YubiHSM2, AWS CloudHSM, and GCP KMS).
This is very similar to the `fips-v1` suite, but "subjects" (users, hosts, etc)
will still be allowed to use Ed25519 keys, it is only the CAs that are limited
to the algorithms supported by the HSM.

### Default suite

For all Teleport clusters where the Auth server's teleport.yaml config file has
`version: v3` (the current latest version), the default suite will be `legacy`.
Changing the default will be a breaking change, so we will introduce a config v4
to change the default.

In config v4 the default suite will be:

* `fips-v1` if the Auth server was started in FIPS mode
* `hsm-v1` if the Auth server has any HSM or KMS configured (and is not in FIPS mode)
* `balanced-v1` otherwise

If the suite is explicitly set in the teleport.yaml or
`cluster_auth_preference`, it will be honoured and no default will be necessary.

### Configuration

The key types and signature algorithms used cluster-wide will be configurable
via `cluster_auth_preference` and `teleport.yaml`

We want it configurable via `teleport.yaml` so that you can start a new cluster
and the CA keys will be automatically generated at first start with the correct
algorithms, so you don't have to immediately edit the `cap` and then rotate all
of your brand-new CAs.

We want it configurable via `cluster_auth_preference` as well so that it can be
configurable for Cloud users.

If a value is set in the `cluster_auth_preference`, it will completely override
the setting from `teleport.yaml`

```yaml
# teleport.yaml
version: v3
teleport:
  auth_service:
    enabled: true

    # supported values are balanced-v1, fips-v1, hsm-v1, and legacy
    signature_algorithm_suite: balanced-v1
```

```yaml
kind: cluster_auth_preference
metadata:
  name: cluster-auth-preference
spec:
  # supported values are balanced-v1, fips-v1, hsm-v1, and legacy
  signature_algorithm_suite: balanced-v1
```

### `balanced-v1` suite

The following key types will be used when the configured algorithm suite is
`balanced-v1`.

* CA key types
  * user CA
    * SSH: Ed25519
    * TLS: ECDSA with NIST P-256
  * host CA
    * SSH: Ed25519
    * TLS: ECDSA with NIST P-256
  * database CA
    * TLS: 2048-bit RSA
      * it's necessary to use RSA for our Snowflake integration, maybe others
  * OpenSSH CA
    * SSH: Ed25519
  * JWT CA
    * JWT: ECDSA with NIST P-256
  * OIDC IdP CA
    * JWT: ECDSA with NIST P-256
  * SAML IdP CA
    * TLS: ECDSA with NIST P-256
* Subject key types
  * users via `tsh login`
    * SSH: Ed25519 (SSH cert signed by user CA)
      * a different algorithm may be used by a hardware key (currently ECDSA with NIST P-256)
    * TLS: ECDSA with NIST P-256 (X.509 cert signed by user CA)
  * user web sessions
    * SSH: Ed25519 (SSH cert signed by user CA)
    * TLS: ECDSA with NIST P-256 (X.509 cert signed by user CA)
  * Teleport hosts
    * SSH: Ed25519 (SSH cert signed by host CA)
    * TLS: ECDSA with NIST P-256 (X.509 cert signed by host CA)
  * OpenSSH hosts
    * SSH: Ed25519 (SSH cert signed by host CA)
  * User certs for "Agentless" SSH connections
    * SSH: Ed25519 (SSH certs signed by OpenSSH CA)
  * proxy -> database agent
    * 2048-bit RSA (X.509 cert signed by Database CA)
    * this may be forwarded directly to the database
  * database agent -> self-hosted database
    * 2048-bit RSA (X.509 cert signed by Database CA)
  * self-hosted database
    * 2048-bit RSA (X.509 cert signed by Database CA)
    * this may be a JWT for Snowflake access
  * windows desktop service -> RDP server
    * 2048-bit RSA (X.509 cert signed by user CA)
    * this is a current limitation of our rdpclient implementation, we could
      change this with some effort, and I don't think it would be a breaking
      change (we could do it within `balanced-v1`).

This suite will *not* be compatible with clusters running in FIPS mode and/or
configured to use an HSM or KMS for CAs.

### `fips-v1` suite

Identical to `balanced-v1` suite except:

* all instances of Ed25519 are replaced with ECDSA with NIST P-256

FIPS 186-5 only added approval for Ed25519 relatively recently (February 2023)
and there is some nuance to how the algorithm can be used.
An even more relevant reason for us to avoid it is that our FIPS builds are
compiled with `GOEXPERIMENT=boringcrypto`, which has no support for Ed25519
(yet, at least).

This suite will be compatible with clusters running in FIPS mode and/or
configured to use an HSM or KMS for CAs.

Clusters running with a non-FIPS compatible suite will be able to migrate to
FIPS mode by first change with configured suite to `fips-v1`, completing all
necessary CA rotations, and then switching to FIPS binaries running in FIPS
mode.

### `hsm-v1` suite

Identical to `balanced-v1` suite except:

* instances of Ed25519 *for CA key types only* are replaced with ECDSA with NIST P-256

Subjects may still use Ed25519 keys, the HSM is only used for CA keys so the
algorithms are limited to what the HSM can support.

We claim to support and test with YubiHSM2, AWS CloudHSM, and GCP KMS.
All of these support ECDSA with NIST P-256, none support Ed25519.

This suite will *not* be compatible with clusters running in FIPS mode, because
subjects may use Ed25519.
In case both FIPS mode and an HSM or KMS are desired, the `fips-v1` suite should
be used instead, it is also compatible with all HSMs and KMS we support.

### `legacy` suite

2048-bit RSA keys are used by all existing CAs and subjects.

If new CAs or subjects are added, they should use the same choice as
`fips-v1` where possible, because the `legacy` suite may be used in clusters
running in FIPS mode or with an HSM.

### CA details

Each Teleport CA holds 1 or more of the following:

* SSH public and private key
* TLS certificate and private key
* JWT public and private key

Each CA key may be a software key stored in the Teleport backend, an HSM key
held in an HSM connected to an Auth server via a PKCS#11 interface, or a KMS key
held in GCP KMS.
In the future we will likely support more KMS services.

Teleport currently has these CAs:

#### User CA

keys: ssh, tls

uses: user ssh cert signing, user tls cert signing, ssh hosts trust this CA

* current/`legacy` SSH key type: 2048-bit RSA
* proposed `balanced-v1` key type: Ed25519
* proposed `fips-v1` key type: ECDSA with NIST P-256
* proposed `hsm-v1` key type: ECDSA with NIST P-256
* reasoning:
  * Ed25519 is currently considered by multiple sources to be the best
    algorithm for SSH
  * ECDSA with P-256 has Go BoringCrypto support, Ed25519 doesn't
  * Ed25519 support was added to OpenSSH 6.5 in January 2014, *before* SHA-2
    hash support was added for RSA keys in OpenSSH 7.2
  * Some SSH clients other than OpenSSH have very limited support for newer
    protocols - we will need to make sure this is very visible as a possibly
    breaking change.

* current/`legacy` TLS key type: 2048-bit RSA
* proposed `balanced-v1` key type: ECDSA with NIST P-256
* proposed `fips-v1` key type: ECDSA with NIST P-256
* proposed `hsm-v1` key type: ECDSA with NIST P-256
* reasoning:
  * external tools may need to load user X.509 certificates signed by this CA,
    e.g. for application access, and Ed25519 support is generally spotty

#### Host CA

keys: ssh, tls

uses: host ssh cert signing, host tls cert signing, ssh clients trust this CA

* current/`legacy` SSH key type: 2048-bit RSA
* proposed `balanced-v1` key type: Ed25519
* proposed `fips-v1` key type: ECDSA with NIST P-256
* proposed `hsm-v1` key type: ECDSA with NIST P-256
* reasoning:
  * Ed25519 is currently considered by multiple sources to be the best
    algorithm for SSH
  * Ed25519 support was added to OpenSSH 6.5 in January 2014, *before* SHA-2
    hash support was added for RSA keys in OpenSSH 7.2
  * ECDSA with P-256 has Go BoringCrypto support, Ed25519 doesn't
  * ECDSA is supported by HSMs, Ed25519 isn't

* current/`legacy` TLS key type: 2048-bit RSA
* proposed `balanced-v1` key type: ECDSA with NIST P-256
* proposed `fips-v1` key type: ECDSA with NIST P-256
* proposed `hsm-v1` key type: ECDSA with NIST P-256
* reasoning:
  * it's possible we may want an external tool to trust host certs, and support
    for Ed25519 X.509 certs is generally spotty

#### Database CA

keys: tls

uses:

* signs (often) long-lived db cert used to authenticate db to database service
* signs short-lived proxy cert used to authenticate proxy to database service
* signs snowflake JWTs
* self-hosted databases (and Snowflake) trust this CA

* current/`legacy` TLS key type: 2048-bit RSA
* proposed `balanced-v1` key type: 2048-bit RSA
* proposed `fips-v1` key type: 2048-bit RSA
* proposed `hsm-v1` key type: 2048-bit RSA
* reasoning:
  * some database protocols still require RSA, reduce friction by keeping it as
    the default

#### OpenSSH CA

keys: ssh

uses: signs user certs to authenticate to registered OpenSSH nodes, registered
OpenSSH hosts trust this CA.

* current/`legacy` SSH key type: 2048-bit RSA
* proposed `balanced-v1` key type: Ed25519
* proposed `fips-v1` key type: ECDSA with NIST P-256
* proposed `hsm-v1` key type: ECDSA with NIST P-256
* reasoning:
  * Ed25519 is currently considered by multiple sources to be the best
    algorithm for SSH
  * Ed25519 support was added to OpenSSH 6.5 in January 2014, *before* SHA-2
    hash support was added for RSA keys in OpenSSH 7.2
  * ECDSA with P-256 has Go BoringCrypto support, Ed25519 doesn't
  * ECDSA is supported by HSMs, Ed25519 isn't

#### JWT CA

keys: jwt

uses: user jwt cert signing, exposed at `/.well-known/jwks.json`, applications
that verify user JWTs trust this CA

* current/`legacy` TLS key type: `RS256` (2048-bit RSA with PKCS#1 v1.5 and SHA256)
* proposed `balanced-v1` key type: `ES256` (ECDSA with NIST P-256 and SHA256)
* proposed `fips-v1` key type: `ES256` (ECDSA with NIST P-256 and SHA256)
* reasoning:
  * `ES256` is `Recommended+` for JWS implementations by RFC 7518,
    this is stronger than `RS256` which is only `Recommended`
    * <https://datatracker.ietf.org/doc/html/rfc7518#section-3>
  * `Ed25519` is not mentioned in RFC 7518

#### OIDC IdP CA

keys: jwt

uses: signing JWTs as an OIDC provider.

* current/`legacy` key type: `RS256` (2048-bit RSA with PKCS#1 v1.5 and SHA256)
* proposed `balanced-v1` key type: `ES256` (ECDSA with NIST P-256 and SHA256)
* proposed `fips-v1` key type: `ES256` (ECDSA with NIST P-256 and SHA256)
* reasoning:
  * `ES256` is `Recommended+` for JWS implementations by RFC 7518,
    this is stronger than `RS256` which is only `Recommended`
    * <https://datatracker.ietf.org/doc/html/rfc7518#section-3>
  * `Ed25519` is not mentioned in RFC 7518

#### SAML IdP CA

keys: tls

uses: signing SAML assertions as a SAML provider.

* current/`legacy` TLS key type: 2048-bit RSA
* proposed `balanced-v1` key type: ECDSA with NIST P-256
* proposed `fips-v1` key type: ECDSA with NIST P-256
* proposed `hsm-v1` key type: ECDSA with NIST P-256
* reasoning:
  * external software needs to trust certs signed by this CA, and support for
    Ed25519 X.509 certs is generally spotty

### Splitting user SSH and TLS private keys

In order to support the use of different key types for SSH and TLS, as well as
possible a different algorithm for some protocols (DB access will use RSA)
we will begin to use a brand new key for each certificate generated instead of
reusing a single private key for all.

This will change the disk layout of the ~/.tsh directory:

```diff
  ~/.tsh/                             --> default base directory
  ├── current-profile                 --> file containing the name of the currently active profile
  ├── one.example.com.yaml            --> file containing profile details for proxy "one.example.com"
  ├── two.example.com.yaml            --> file containing profile details for proxy "two.example.com"
  ├── known_hosts                     --> trusted certificate authorities (their keys) in a format similar to known_hosts
  └── keys                            --> session keys directory
     ├── one.example.com              --> Proxy hostname
     │   ├── certs.pem                --> TLS CA certs for the Teleport CA
-    │   ├── foo                      --> Private Key for user "foo"
-    │   ├── foo.pub                  --> Public Key
+    │   ├── foo                      --> SSH Private Key for user "foo"
+    │   ├── foo.pub                  --> SSH Public Key
     │   ├── foo.ppk                  --> PuTTY PPK-formatted keypair for user "foo"
     │   ├── kube_credentials.lock    --> Kube credential lockfile, used to prevent excessive relogin attempts
-    │   ├── foo-x509.pem             --> TLS client certificate for Auth Server
+    │   ├── foo-privkey.pem          --> TLS client private key
+    │   ├── foo-cert.pem             --> TLS client certificate for Auth Server
     │   ├── foo-ssh                  --> SSH certs for user "foo"
     │   │   ├── root-cert.pub        --> SSH cert for Teleport cluster "root"
     │   │   └── leaf-cert.pub        --> SSH cert for Teleport cluster "leaf"
     │   ├── foo-app                  --> App access certs for user "foo"
     │   │   ├── root                 --> App access certs for cluster "root"
-    │   │   │   ├── appA-x509.pem    --> TLS cert for app service "appA"
-    │   │   │   ├── appB-x509.pem    --> TLS cert for app service "appB"
+    │   │   │   ├── appA-privkey.pem --> TLS private key for app service "appA"
+    │   │   │   ├── appA-cert.pem    --> TLS cert for app service "appA"
+    │   │   │   ├── appB-privkey.pem --> TLS private key for app service "appB"
+    │   │   │   └── appB-cert.pem    --> TLS cert for app service "appB"
     │   │   └── leaf                 --> App access certs for cluster "leaf"
-    │   │       ├── appC-x509.pem    --> TLS cert for app service "appC"
+    │   │       ├── appC-privkey.pem --> TLS private key for app service "appC"
+    │   │       └── appC-cert.pem    --> TLS cert for app service "appC"
     │   ├── foo-db                   --> Database access certs for user "foo"
     │   │   ├── root                 --> Database access certs for cluster "root"
-    │   │   │   ├── dbA-x509.pem     --> TLS cert for database service "dbA"
-    │   │   │   ├── dbB-x509.pem     --> TLS cert for database service "dbB"
+    │   │   │   ├── dbA-privkey.pem  --> TLS private key for database service "dbA"
+    │   │   │   ├── dbA-cert.pem     --> TLS cert for database service "dbA"
+    │   │   │   ├── dbB-privkey.pem  --> TLS private key for database service "dbB"
+    │   │   │   ├── dbB-cert.pem     --> TLS cert for database service "dbA"
     │   │   │   └── dbC-wallet       --> Oracle Client wallet Configuration directory.
     │   │   ├── leaf                 --> Database access certs for cluster "leaf"
-    │   │   │   ├── dbC-x509.pem     --> TLS cert for database service "dbC"
+    │   │   │   ├── dbC-privkey.pem  --> TLS private key for database service "dbC"
+    │   │   │   └── dbC-cert.pem     --> TLS cert for database service "dbC"
     │   │   └── proxy-localca.pem    --> Self-signed TLS Routing local proxy CA
     │   ├── foo-kube                 --> Kubernetes certs for user "foo"
     │   |    ├── root                 --> Kubernetes certs for Teleport cluster "root"
     │   |    │   ├── kubeA-kubeconfig --> standalone kubeconfig for Kubernetes cluster "kubeA"
-    │   |    │   ├── kubeA-x509.pem   --> TLS cert for Kubernetes cluster "kubeA"
+    │   |    │   ├── kubeA-privkey.pem --> TLS private key for Kubernetes cluster "kubeA"
+    │   |    │   ├── kubeA-cert.pem   --> TLS cert for Kubernetes cluster "kubeA"
     │   |    │   └── localca.pem      --> Self-signed localhost CA cert for Teleport cluster "root"
     │   |    └── leaf                 --> Kubernetes certs for Teleport cluster "leaf"
     │   |        ├── kubeC-kubeconfig --> standalone kubeconfig for Kubernetes cluster "kubeC"
-    │   |        └── kubeC-x509.pem   --> TLS cert for Kubernetes cluster "kubeC"
+    │   |        └── kubeC-privkey.pem --> TLS cert for Kubernetes cluster "kubeC"
+    │   |        └── kubeC-cert.pem --> TLS cert for Kubernetes cluster "kubeC"
     |   └── cas                       --> Trusted clusters certificates
     |        ├── root.pem             --> TLS CA for teleport cluster "root"
     |        ├── leaf1.pem            --> TLS CA for teleport cluster "leaf1"
     |        └── leaf2.pem            --> TLS CA for teleport cluster "leaf2"
     └── two.example.com               --> Additional proxy host entries follow the same format
                ...
```

`foo` above is a stand-in for the username of the logged-in user.

Currently the user's private key for both SSH and TLS is held in
`~/.tsh/keys/one.example.com/foo`, with the SSH pubkey held in `foo.pub` and
the TLS cert held in `foo-x509.pem`.
App, Database, and K8s certs use the same private key and are held under the
`foo-app`, `foo-db`, and `foo-kube` directories.

To avoid breaking user OpenSSH config files that rely on the location of the ssh
keys, they will not be moved.
The TLS keys and certs are often used automatically by `tsh` e.g. in `tsh proxy
app`, `tsh db connect`, `tsh kube login`, so it is less likely for users to have
to make a manual fix if we move the TLS files.

The main TLS private key will now be held in
`~/.tsh/keys/one.example.com/foo-privkey.pem`

All TLS x509 cert files will be renamed from `<name>-x509.pem` to
`<name>-cert.pem` so that any software trying to use the old `<name>-x509.pem`
along with the outdated private key location will fail to load both files,
instead of succesfully opening the files but failing with some confusing error
when the private key does not match the certificate.

RPCs such as `GenerateUserCerts` will also need to change to support passing
both the SSH and TLS public keys along, possibly along with attestation
statements if hardware keys are being used.
These will remain backward compatible by continuing to use the single public key
for both protocols if both are not passed.

### HSMs and KMS

Admins should configure the `hsm-v1` suite when using any HSM or KMS.

When the default suite is changed in config v4, it will default to `hsm-v1` if
any HSM or KMS is configured.
If a specific PKCS#11 HSM does not support one of the algorithms, we will do our
best to return an informative error message to the user and block the CA
rotation before the misconfigured algorithm could take effect.

If a cluster has an HSM and an admin explicitly configures the `balanced-v1`
suite, we can warn but try to support it in case their PKCS#11 HSM supports
Ed25519, and fail early with explicit logs if there's an error.

#### Cloud

Cloud will be able to select their preferred default suite by configuring it in
the `teleport.yaml`.
Cloud users will be able to change the CA algorithms by modifying the
`cluster_auth_preference` and performing the necessary CA rotations.
We should update the default to `balanced-v1` when config v4 is released.

### Backward Compatibility

* Will the change impact older clients? (tsh, tctl)

By default, Auth servers with non-default algorithms configured should continue
to sign certificates for clients on older Teleport versions using RSA2048 keys.
In a future major version we can disable this and enforce that the configured
algorithm suite is followed by all clients.

* Are there any backend migrations required?

A CA rotation will effectively act as the backend migration when changing
algorithms.

### Remote Clusters

Remote clusters may all need to be updated to a version with support for the new
algorithms before any cluster can begin using a new algorithm.
This will be tested and documented.
We will attempt to warn users or prevent them from creating an incompatible
config when remote clusters are on older Teleport versions.

### Security

This entire RFD is about improving Teleport's security.

We are not introducing any new endpoints, CLI commands, or APIs.

All supported algorithms will be reviewed by our internal security team and
external security auditors.

We will only use Go standard library implementations of crypto algorithms (or
BoringCrypto if compiled in FIPS mode).

### Signature Algorithms

These signature algorithms are being considered for support:

#### RSA

Private key sizes: 2048

Signature algorithms: PKCS#1 v1.5

Digest/hash algorithms: SHA512 for SSH, SHA256 for TLS

Considerations:

* RSA2048 is the current default and deviating from it by default may break
  compatibility with third-party components and protocols
* RSA has the most widespread support among all protocols
* Certain database protocols only support RSA client certs
  * <https://docs.snowflake.com/en/user-guide/key-pair-auth#step-2-generate-a-public-key>
  * Our [Cassandra docs](https://goteleport.com/docs/database-access/guides/cassandra-self-hosted/?scope=enterprise#step-45-configure-cassandrascylla)
    explicitly include `TLS_RSA_WITH_AES_256_CBC_SHA` cipher suite, not sure if
    necessary, but some changes would need to be made if we deviate from RSA
* Some apps may only support RSA signed JWTs
* golang.org/x/crypto/ssh uses SHA512 hash by default with all RSA public keys
  (but this can be overridden)
  <https://github.com/golang/crypto/blob/0ff60057bbafb685e9f9a97af5261f484f8283d1/ssh/certs.go#L443-L445>
* crypto/x509 uses SHA256 hash by default with all RSA public keys
  (but this can be overridden)
  <https://github.com/golang/go/blob/dbf9bf2c39116f1330002ebba8f8870b96645d87/src/crypto/x509/x509.go#L1411-L1414>
* ssh only supports the PKCS#1 v1.5 signature scheme with RSA keys
  <https://datatracker.ietf.org/doc/html/rfc8332>
* FIPS 186-5 approves RSA with all the options listed here
* BoringCrypto supports all listed options
* We could consider PSS signatures instead of PKCS#1 v1.5 for TLS and JWT
  signatures, but SSH does not support it.
* We could consider supporting larger RSA key sizes, but we should prefer to
  push people towards the newer, better algorithms

#### ECDSA

Curves: P-256

Digest/hash algorithms: SHA256

Considerations:

* ECDSA has good support across SSH and TLS protocols for both client and CA
  certs.
* ECDSA certs are supported by web browsers.
* ECDSA key generation is *much* faster than RSA key generation.
* ECDSA signatures are faster than RSA signatures.
* FIPS 186-5 approves it
* BoringCrypto supports it
* The P-256 curve is the most common, it is considered to be secure, and it has
  the broadest support among external tools.
* We could consider supporting the P-384 and P-521 curves for CAs but this would
  have worse performance without much tangible benefit.

#### EdDSA

Curves: Ed25519

Digest/hash algorithms: none (the full message is signed without hashing)

Considerations:

* There is widespread support for Ed25519 SSH certs.
* Go libraries support Ed25519 for TLS
* Support for Ed25519 is *not* widespread in the TLS ecosystem.
  * Neither Chrome nor Firefox support EdDSA signatures
  * The [CA Baseline Requirements](https://cabforum.org/baseline-requirements-documents/)
    do not allow EdDSA signatures
  * LibreSSL only added Ed25519 support in v3.7.0 (2022-12-12), the default
    version of `curl` on my fully updated Macbook (Ventura 13.4.1) uses
    LibreSSL 3.3.6 (2022-03-15)
* YubiHSM and GCP KMS do *not* support Ed25519 keys.
* YubiKey 5 does *not* support Ed25519 keys.
* Ed25519 is considered by some to be the fastest, most secure, most modern
  option for SSH certs.
* Ed25519 key generation is *much* faster than RSA key generation.
* Ed25519 signatures are faster than RSA signatures.
* FIPS 186-5 approves Ed25519
* Go BoringCrypto does not support Ed25519
  <https://github.com/golang/go/blob/cd6676126b7e663e6202e98e2f235fff20d5e858/src/crypto/tls/boring.go#L78-L90>
* Ed25519 is the only EdDSA curve supported in the Go standard library.

#### Algorithms Summary

* We are probably forced to continue unconditionally using RSA for database
  certs, I'm assuming this would apply to both client and CA.
* Ed25519 is a modern favourite for SSH, but TLS (and HSM, KMS) support is lacking.
* Teleport CAs use separate keypairs for SSH and TLS, they do not need to use
  the same algorithm.
* Teleport derives client SSH and TLS certs from the same client keypair,
  supporting different algorithms for each will require larger changes.
* It seems it is time to split client SSH and TLS keys to support the popular
  and secure Ed25519 algorithm for SSH and the widely-supported
  `ECDSA_P256_SHA256` algorithm for TLS. This will also allows to evolve the
  algorithms used for each protocol independently in the future.

### UX

The algorithm suite will be configurable in `teleport.yaml` and
`cluster_auth_preference` as described above.

We will add visibility of the current and configured CA algorithms to `tctl status`.

```
$ tctl status
Cluster      cluster-one
Version      14.0.0-dev

Host CA pins:
sha256:d2c825ede608da97891b3eaf1dac27e7e1085be5ff71e48a97ff8eb53360e362

Host CA
expires:         Jun 21 2033 00:03:19 UTC
last rotated:    Jun 21 2023 00:03:19 UTC
rotation state:  standby
SSH algorithm:   Ed25519
TLS algorithm:   ECDSA_P256_SHA256

User CA
expires:         Jun 21 2033 00:03:19 UTC
last rotated:    never
rotation state:  standby
SSH algorithm:   RSA2048_PKCS1_SHA512 (balanced-v1 algorithm Ed25519 will take effect during next manual CA rotation)
TLS algorithm:   RSA2048_PKCS1_SHA256 (balanced-v1 algorithm ECDSA_P256_SHA256 will take effect during next manual CA rotation)

Database CA
expires:         Jun 21 2033 00:03:19 UTC
last rotated:    never
rotation state:  rotating clients (mode: manual, started: Jun 30 2023 00:03:19 UTC, ending: Jul 1 2023 06:03:19 UTC)
TLS algorithm:   RSA3072_PKCS1_SHA256

...
```

`tctl auth rotate` will inform users when a rotation is going to trigger an
algorithm change.

```
$ tctl auth rotate --manual --type user --phase init
INFO: Rotation will update the key types for this CA to match the balanced-v1 suite:
Protocol  Before                After
SSH       RSA2048_PKCS1_SHA512  Ed25519
TLS       RSA2048_PKCS1_SHA256  ECDSA_P256_SHA256

Updated rotation phase to "init". To check status use 'tctl status'
```

If users realize that the new algorithms have broken compatibility with some of
their systems, they can either roll back the rotation if it has not yet
completed, or switch their algorithms suite back to `legacy` and rotate again.

### Audit Events

User login and existing certificate generation events will be supplemented with
info on the algorithms used by the subject and the CA.

### Observability

Log messages will be emitted whenever CA keys/certs are generated or rotated,
including the algorithms used for all new keys.

Audit events will also include the algorithms used.

When the configuration has been updated but the certs have not been rotated yet,
this will be logged on Auth startup and visible in the output of `tctl status`
(see UX section).

When the `legacy` suite is used by default and hasn't been explicitly
configured, we will log during Auth server startup and display in the output of
`tctl status` a hint that the newer suites are available and suggest upgrading
with a link to the docs.

### Product Usage

We will not add telemetry or usage events, logs and audit events will indicate
if this feature is being used.

### Test Plan

TODO:

Include any changes or additions that will need to be made to
the [Test Plan](../.github/ISSUE_TEMPLATE/testplan.md) to appropriately
test the changes in your design doc and prevent any regressions from
happening in the future.

## Rejected alternatives

### Configurable algorithms per protocol

Introduce a new config to `teleport.yaml` and `cluster_auth_preference`
to control the key types and signature algorithms used by Teleport CAs and all
clients and hosts which have certificates issued by those CAs.

This config will default to a `recommended` set of algorithms for each protocol
chosen by us to balance security, compatibility, and performance.
We will reserve the right to change this set of `recommended` algorithms when
either:

* the major version of the auth server's teleport.yaml config changes, or
* in a major version release of Teleport.

Most Teleport administrators will never need to see or interact with this config
because they can trust that we will select a vetted set of standards-compliant
algorithms that are trusted to be secure, and we will not break compatibility
with internal Teleport components or third-party software unless deemed
absolutely necessary for security reasons.

Teleport administrators will be able to deviate from the `recommended`
algorithms when they have a compliance need (they must use a particular
algorithm) or a compatibility need (one of our selected algorithms is not
supported by an external software that interacts with Teleport in their
deployment).

Here is what the config will look like in its default state:

```yaml
ca_key_params:
  user:
    ssh:
      algorithm: recommended
      allowed_subject_algorithms: [recommended]
    tls:
      algorithm: recommended
      allowed_subject_algorithms: [recommended]
  host:
    ssh:
      algorithm: recommended
      allowed_subject_algorithms: [recommended]
    tls:
      algorithm: recommended
      allowed_subject_algorithms: [recommended]
  db:
    tls:
      algorithm: recommended
      allowed_subject_algorithms: [recommended]
  openssh:
    ssh:
      algorithm: recommended
      allowed_subject_algorithms: [recommended]
  jwt:
    jwt:
      algorithm: recommended
      allowed_subject_algorithms: [recommended]
  saml_idp:
    tls:
      algorithm: recommended
      allowed_subject_algorithms: [recommended]
  oidc_idp:
    jwt:
      algorithm: recommended
      allowed_subject_algorithms: [recommended]
```

You can imagine that if we had this config today, `algorithm: recommended` would
expand to `RSA2048_PKCS1_SHA(256|512)` for all protocols.
For `allowed_subject_algorithms`, technically this is not enforced at all today,
but all users and hosts also use RSA2048 keypairs.

When we are ready to update the defaults (in a major config version or a major
release) we will update the `recommended` rules to default to the following:

(Note: there will be no actual change to the configuration resource which will
still show `recommended`, the actual values will be computed within Teleport)

```yaml
ca_key_params:
  user:
    ssh:
      algorithm: Ed25519
      # RSA2048 will initially be allowed for older `tsh` clients that don't
      # know how to generate Ed25519 certs, and removed in a future major version
      allowed_subject_algorithms: [Ed25519, RSA2048_PKCS1_SHA512]
    tls:
      algorithm: ECDSA_P256_SHA256
      # RSA2048 will initially be allowed for older `tsh` clients that don't
      # know how to generate Ed25519 certs, and removed in a future major version
      allowed_subject_algorithms: [ECDSA_P256_SHA256, RSA2048_PKCS1_SHA256]
  host:
    ssh:
      algorithm: Ed25519
      # RSA2048 will initially be allowed for older hosts that don't know how to
      # generate Ed25519 certs, and removed in a future major version
      allowed_subject_algorithms: [Ed25519, RSA2048_PKCS1_SHA512]
    tls:
      algorithm: ECDSA_P256_SHA256
      # RSA2048 will initially be allowed for older hosts that don't know how to
      # generate Ed25519 certs, and removed in a future major version
      allowed_subject_algorithms: [ECDSA_P256_SHA256, RSA2048_PKCS1_SHA256]
  db:
    tls:
      # multiple DBs only support RSA so it will remain the default for now
      algorithm: RSA3072_PKCS1_SHA256
      # db certs are often fairly long-lived so we should prefer a larger key
      # size for them.
      # We will allow Ed25519 for connections the Proxy makes to Teleport
      # database services because they are short lived, generated often, and
      # only used internally within Teleport components.
      allowed_subject_algorithms: [RSA3072_PKCS1_SHA256, RSA2048_PKCS1_SHA256, Ed25519]
  openssh:
    ssh:
      algorithm: Ed25519
      # RSA2048 will initially be allowed for older hosts that don't know how to
      # generate Ed25519 certs, and removed in a future major version
      allowed_subject_algorithms: [Ed25519, RSA2048_PKCS1_SHA512]
  jwt:
    jwt:
      algorithm: ECDSA_P256_SHA256
      allowed_subject_algorithms: [ECDSA_P256_SHA256]
  saml_idp:
    tls:
      algorithm: ECDSA_P256_SHA256
      allowed_subject_algorithms: [ECDSA_P256_SHA256]
  oidc_idp:
    jwt:
      algorithm: ECDSA_P256_SHA256
      allowed_subject_algorithms: [ECDSA_P256_SHA256]
```

For backward-compatibility, all certs already signed by trusted CAs will
continue to be trusted, `allowed_subject_algorithms` can be modified at any time
without breaking connectivity, and only controls the allowed algorithms used for
new certificates signed by the CA.

Changing CA `algorithm` values in this config will take effect for:

* new Teleport clusters
* existing Teleport clusters only after a CA rotation.
