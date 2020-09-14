---
authors:  Ben Arent (ben@gravitational.com), Alexander Klizhentas (sasha@gravitational.com),
state: draft
---

# RFD 4 - Device Attestation

## What?

Device attestation acts as a form of contextual access where the device must pass
an inspection before the user is granted access, even if they have the correct
Teleport Login.

Teleport uses SSO to verify customer identity and set RBAC rules to set access,
we’ve companies that not only want to verify if it’s the user but also that it’s
coming from an approved endpoint.

## Why?

Teleport customers want to only allow `tsh` to work on corporate supplied laptops
that are patched and have the up-to-date corporate policies on them, through a
EPP, MDM, SCEP or internal service that’ll only allow issued devices ( laptops
/ workstations ) to access Teleport.

## Device Setup

The IT Department will update a device via MDM management, and shall provide a
certificate to end user devices.

Trusted device issued a long lived X509 certificate by a separate certificate authority
used for all devices. The certificate metadata includes the device ID, owner and
any other information.

For this RFD, we'll assume they'll be an out of band host scanning and certificate
distribution.

[Certificate Scanning](diagrams/0004/certificates-out-of-band.png)

## Login Experience

On Login tsh will contact the auth server using the long lived device certificate,
and then pass the state through the web flow back, which ties the long lived cert
to the issued short lived cert.

Once verified the Teleport experience should be the same for tsh CLI and Web UI.

## Revocation

Since IT departments have full machine access they are able to lock down access to
certificates.

### Appendix

[1] Info on Mac Deployments https://www.apple.com/business/docs/site/Mac_Deployment_Overview.pdf

[2] List of Hardware Devices with TPMs
T2 Chip on iMac https://support.apple.com/en-us/HT208862

[3] Startup Versions for MDM
https://www.rippling.com/mobile-device-management/

[4] Device Enrollment from x509 certs https://docs.microsoft.com/en-us/azure/iot-dps/concepts-service#individual-enrollment

[5] ExtensibleSingleSignOnKerberos - Device Management
https://developer.apple.com/videos/play/tech-talks/301/
https://developer.apple.com/documentation/devicemanagement/extensiblesinglesignonkerberos
