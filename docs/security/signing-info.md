## Public Certificates & Encryption Keys

We use the following certificates and public keys to sign our software. Many of
these keys and certificates use our legal business name “Gravitational Inc.” and
our former domain “gravitational.com”. Don’t worry – [Gravitational is
Teleport](https://goteleport.com/blog/gravitational-is-teleport/).

### APT, YUM, & Zypper Signing Keys

We sign our [APT, YUM and Zypper
repositories](https://goteleport.com/docs/installation/#package-repositories)
with the following PGP key:

* ID `C87ED53A6282C411`
* Fingerprint `0C5E 8BA5 658E 320D 1B03 1179 C87E D53A 6282 C411`

The key is available for download at:
* [https://apt.releases.teleport.dev/gpg](https://apt.releases.teleport.dev/gpg)
* [https://yum.releases.teleport.dev/gpg](https://yum.releases.teleport.dev/gpg)
* [https://zypper.releases.teleport.dev/gpg](https://zypper.releases.teleport.dev/gpg)

### Apple Signing Certificates

Our Apple packages and binaries are [code
signed](https://developer.apple.com/support/code-signing/) by "Developer ID
QH8AA5B8UP Gravitational Inc." with the following certificate:
* SHA256 Fingerprint: `78 2F E1 18 5F A1 AD 68 AD 25 0B A9 4D 21 DC BB 0D 8E 47
  C6 E4 1D FE FB AB 05 41 33 4C 33 1D 43`
* SHA1 Fingerprint: `82 B6 25 AD 32 7C 24 1B 37 8A 54 B4 B2 54 BB 08 CE 71 B5 DF`

Packages published prior to September 14, 2021 are signed with an older certificate for the same Developer ID (QH8AA5B8UP):
* SHA256 Fingerprint: `78 05 14 69 20 59 21 D1 EE 96 42 01 5A 28 35 FB E1 D4 38 5E 2A 23 5D 62 73 A4 D1 27 8A 33 BA 34`
* SHA1 Fingerprint: `D2 70 EA 0C F2 0E CB 17 28 B2 21 E1 D5 B6 7C FE 50 FF AB 62`

Verify the Developer ID and fingerprint match on package downloads with the
pkgutil tool:
```console
$ pkgutil --check-signature teleport-15.0.2.pkg
Package "teleport-15.0.2.pkg":
   Status: signed by a developer certificate issued by Apple for distribution
   Notarization: trusted by the Apple notary service
   Signed with a trusted timestamp on: 2024-02-16 21:42:52 +0000
   Certificate Chain:
    1. Developer ID Installer: Gravitational Inc. (QH8AA5B8UP)
       Expires: 2026-07-27 18:27:29 +0000
       SHA256 Fingerprint:
           78 2F E1 18 5F A1 AD 68 AD 25 0B A9 4D 21 DC BB 0D 8E 47 C6 E4 1D
           FE FB AB 05 41 33 4C 33 1D 43
       ------------------------------------------------------------------------
    2. Developer ID Certification Authority
       Expires: 2027-02-01 22:12:15 +0000
       SHA256 Fingerprint:
           7A FC 9D 01 A6 2F 03 A2 DE 96 37 93 6D 4A FE 68 09 0D 2D E1 8D 03
           F2 9C 88 CF B0 B1 BA 63 58 7F
       ------------------------------------------------------------------------
    3. Apple Root CA
       Expires: 2035-02-09 21:40:36 +0000
       SHA256 Fingerprint:
           B0 B1 73 0E CB C7 FF 45 05 14 2C 49 F1 29 5E 6E DA 6B CA ED 7E 2C
           68 C5 BE 91 B5 A1 10 01 F0 24
```

The codesign tool can be used to perform the verification on individual
binaries:
```console
$ codesign --verify -d --verbose=2 /usr/local/bin/tsh
...
Authority=Developer ID Application: Gravitational Inc. (QH8AA5B8UP)
Authority=Developer ID Certification Authority
Authority=Apple Root CA
Timestamp=Jun 29, 2024 at 11:02:15 PM
Info.plist=not bound
TeamIdentifier=QH8AA5B8UP
...
```

The Teleport package in Homebrew is not maintained or signed by Teleport. We
recommend the use of [our Teleport packages](https://goteleport.com/download/).

### Windows Signing Certificates

Our Windows binaries are signed with the following certificate:
* Issued to: Gravitational Inc.
* Thumbprint: C644BAB07912F5BD09BDB3C2D9AE6A724F9B2391

Verify the binary using the following PowerShell command:
```console
Get-AuthenticodeSignature -FilePath .\tsh.exe

    Directory: C:\Users\ExampleUser

SignerCertificate                         Status   Path
-----------------                         ------   ----
C644BAB07912F5BD09BDB3C2D9AE6A724F9B2391  Valid    tsh.exe
```

Ensure that the `SignerCertificate` matches the thumbprint shown above, and that
the `Status` field is `Valid`. 

To further inspect the certificate, run the following PowerShell command:
```console
(Get-AuthenticodeSignature -FilePath.\tsh.exe).SignerCertificate | Format-List

Subject      : CN="Gravitational, Inc.", O="Gravitational, Inc.", L=Oakland,
               S=California, C=US, SERIALNUMBER=5720258,
               OID.2.5.4.15=Private Organization,
               OID.1.3.6.1.4.1.311.60.2.1.2=Delaware,
               OID.1.3.6.1.4.1.311.60.2.1.3=US
Issuer       : CN=DigiCert Trusted G4 Code Signing RSA4096 SHA384 2021 CA1,
               O="DigiCert, Inc.", C=US
Thumbprint   : C644BAB07912F5BD09BDB3C2D9AE6A724F9B2391
FriendlyName :
NotBefore    : 11/2/2023 12:00:00 AM
NotAfter     : 10/16/2026 11:59:59 PM
Extensions   : {System.Security.Cryptography.Oid, 
                System.Security.Cryptography.Oid,
                System.Security.Cryptography.Oid,
                System.Security.Cryptography.Oid...}
```

Alternatively, Windows binaries may be inspected graphically via the Windows
Explorer with the following steps:
1. Right click on the binary in question, for example `tsh.exe`.
2. Select “Properties”.
3. On the resulting “tsh.exe Properties” dialog, select the “Digital Signatures”
   tab.
4. Select the “Gravitational Inc.” signer from the list.
5. Select the “Details” button.
5. On the resulting “Digital Signature Details” dialog, ensure that the header
   states “This digital signature is OK.”
6. Select the “View Certificate” button.
7. On the resulting “Certificate” dialog, select the “Details” tab.
8. Select the “Thumbprint” item from the list, and compare its value to the
   thumbprint listed above.

### OCI Container Images
All of our distroless OCI container images are signed with `cosign`. The public
key is:
```
-----BEGIN PUBLIC KEY-----
MIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAx+9UZboMl9ibwu/IWqbX
+wEJeKJqVpaLEsy1ODRpzIgcgaMh2n3BWtFEIoEszR3ZNlGdfqoPmb0nNnWx/qSf
eEsoSXievXa63M/gAUBB+jecbGEJH+SNaJPMVuvjabPqKtoMT2Spw3cacqpINzq1
rkWU8IawY333gXbwzgsuK7izT7ymgOLPO9qPuX7Q3EBaGw3EvY7u6UKtqhvSGdyr
MirEErOERQ8EP8TrkCcJk0UfPAukzIcj91uHlXaqYBD/IyNYiC70EOlSLoN5/EeA
I4jQnGRfaKF6H6K+WieX9tP9k8/02S+1EVJW592pdQZhJZEq1B/dMc8UR3IjPMMC
qCT2xT6TsinaVzDaAbaRf0hvp311GxwrckNofGm/OSLn1+HqM6q4/A7qHubeRXGO
byabRr93CHSLegZ7OBMswHqqnu6/DuXjc6gOsQkH09dVTFeh34rQy4GKrvnpmOwj
Er1ccxzKcF/pw+lxi07hkpihR/uHUPxFboA/Wl7H2Jub21MFwIFQrDJv7z8yQgxJ
EuIXJJox2oAL7NzdSi9VIUYnEnx+2EtkU/spAFRR6i1BnT6aoIy3521B76wnmRr9
atCSKjt6MdRxgj4htCjBWWJAGM9Z/avF4CYFmK7qiVxgpdrSM8Esbt2Ta+Lu3QMJ
T8LjqFu3u3dxVOo9RuLk+BkCAwEAAQ==
-----END PUBLIC KEY-----
```

Signatures can be validated against the Teleport OCI image signing key:
```console
$ cosign verify --key teleport-oci-key.pub \
    public.ecr.aws/gravitational/teleport-distroless-debug:15.4.6

Verification for public.ecr.aws/gravitational/teleport-distroless-debug:15.4.6 --
The following checks were performed on each of these signatures:
  - The cosign claims were validated
  - The signatures were verified against the specified public key

[
    {
        "critical": {
            "identity": {
                "docker-reference": "public.ecr.aws/gravitational/teleport-distroless-debug"
            },
            "image": {
                "docker-manifest-digest": "sha256:02093593bf129dc304b79854b01b0b911674e9bd6b9049cac14b6e1b116c58e5"
            },
            "type": "cosign container image signature"
        },
        "optional": ...
    }
]
```
