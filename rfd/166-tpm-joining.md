---
authors: @strideynet (noah@goteleport.com)
state: draft
---

# RFD 0166 - TPM Joining

## Required Approvers
 
* Engineering: (@zmb33 || @codingllama)
* Security: (@reedloden || @jentfoo)
* Product: (@xinding33 || @klizhentas)

# What

Introduce a new join method that allows a Bot or Agent running on a host with a
TPM to securely join a Teleport cluster without using a shared secret.

# Why

Bootstrapping trust with a newly provisioned host in an on-premises environment
is challenging. In many environments, this is done by transferring an initial
shared secret to the host, which can then be used for authenticating and
receiving other identities. However, this method is difficult to complete
securely - especially at scale. This is due to the risk of impersonation, an
attacker who has sufficiently compromised the network can impersonate a newly
provisioned host and receive access to this secret. In addition, this shared 
secret is liable to exfiltration.

A TPM provides a secure, unique and persistent initial identity ideal for
bootstrapping trust with a host. Even across reboots, or reconfigurations, the
TPM identity remains the same. The guarantees provided by a compliant TPM mean
that this identity cannot be exfiltrated. This makes it a strong candidate for
bootstrapping trust.

# Implementation

## Overview of TPM functionality and terminology

- Trusted Platform Module (TPM): a hardware module that provides a secure root
  of trust for a host. It can securely generate and store keys, and perform
  cryptographic operations for the host without exposing the keys.
- Endorsement Key (EK): A key pair that is "burned in" to the TPM at the time of
  manufacture. The endorsement key cannot be used to perform signing operations,
  cannot be exported from the TPM, but can be used to decrypt data.
- Public Endorsement Key (EKPub): The public part of the EK.
- Endorsement Key Certificate (EKCert): A certificate containing the EKPub,
  signed by the TPM manufacturer's CA, that is "burned in" to many TPMs. This 
  allows the authenticity of the TPM to be verified.
- Attestation Key (AK): A key pair created for the TPM which can be used to
  sign data.

A compliant TPM provides certain guarantees that will be relevant to TPM
joining:

- Certain keys cannot be "exported" from the TPM (e.g available to the host)
- Certain keys can only be used to perform certain operations (e.g the EK cannot
  be used to sign or decrypt arbitrary data)
- The Credential Activation ceremony can be used to prove that a key exists in
  the TPM with another key, and that the key is configured in a certain way
  (e.g cannot be exported).

It is important to note that these guarantees do not exist if the TPM is not
compliant or the host has been compromised in such a way that some malicious
software is intercepting commands intended for the TPM. This is mitigated by
validating the EKPub and EKCert against the manufacturers CA. Unless

Credential Activation is a TPM ceremony that allows a third party to verify a
TPM's possession of 

## Authentication Process

## API Changes

## Security Considerations

### TPM Compromise

