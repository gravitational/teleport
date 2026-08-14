// Teleport
// Copyright (C) 2023  Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

use crate::scard::piv::auth_cert::build_piv_auth_cert;
use crate::scard::piv::chuid::build_chuid;
use crate::scard::piv::{tlv_tags, utils};
use crate::scard::Response;
use ironrdp_pdu::{pdu_other_err, PduResult};
use iso7816::aid::Aid;
use iso7816::command::instruction::Instruction;
use iso7816::command::Command;
use iso7816::response::Status;
use iso7816_tlv::ber::{Tag, Tlv, Value};
use log::{debug, warn};
use rand::{RngCore, TryRngCore};
use rsa::pkcs1::DecodeRsaPrivateKey;
use rsa::traits::{PrivateKeyParts, PublicKeyParts};
use rsa::{BigUint, RsaPrivateKey};
use std::convert::TryFrom;
use std::io::{Cursor, Read};
use uuid::Uuid;

// AID (Application ID) of PIV application, per:
// https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-73-4.pdf
const PIV_AID: Aid = Aid::new_truncatable(
    &[
        0xA0, 0x00, 0x00, 0x03, 0x08, 0x00, 0x00, 0x10, 0x00, 0x01, 0x00,
    ],
    5, // usually truncates to first 5 bytes
);

// See 'Table 3' at: https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-73-4.pdf#page=31
const CARD_CAPABILITY_CONTAINER_TAG: &[u8] = &[0x5F, 0xC1, 0x07];
const CHUID_TAG: &[u8] = &[0x5F, 0xC1, 0x02];
const PIV_AUTH_CERT_TAG: &[u8] = &[0x5F, 0xC1, 0x05];

// Card implements a PIV-compatible smartcard, per:
// https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-73-4.pdf
#[derive(Debug, PartialEq, Eq)]
pub struct Card<const S: usize> {
    // Card Capability Container.
    ccc: Vec<u8>,
    // Card-holder user ID (CHUID). In federal agencies, this value would be unique per employee
    // and encodes some agency information. In our case it's static.
    chuid: Vec<u8>,
    piv_auth_cert: Vec<u8>,
    piv_auth_key: RsaPrivateKey,
    pin: String,
    // Pending command and response to receive/send over multiple messages when
    // they don't fit into one.
    pending_command: Option<Command<S>>,
    pending_response: Option<Cursor<Vec<u8>>>,
    security_status: SecurityStatus,
}

// A simplified security status as defined in '2.4.2. Security Status': https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-73-4.pdf#page=80
// Its full definition is established by ISO7816, Part 4.
// Here, it consists of a single security status indicator related to the status of the PIV Card Application PIN.
// According to '2.3.1 Default Selected Card Application', we can assume that the PIV Card Application is always chosen.
#[derive(Debug, Clone, PartialEq, Eq)]
enum SecurityStatus {
    PinNotVerified,
    PinVerified,
}

impl<const S: usize> Card<S> {
    pub(in crate::scard) fn new(
        uuid: Uuid,
        cert_der: &[u8],
        key_der: &[u8],
        pin: String,
    ) -> PduResult<Self> {
        let piv_auth_key = RsaPrivateKey::from_pkcs1_der(key_der)
            .map_err(|_e| pdu_other_err!("failed to parse private key from DER"))?;

        Ok(Self {
            ccc: Self::build_ccc(),
            chuid: build_chuid(uuid),
            piv_auth_cert: build_piv_auth_cert(cert_der),
            piv_auth_key,
            pin,
            pending_command: None,
            pending_response: None,
            security_status: SecurityStatus::PinNotVerified,
        })
    }

    pub(in crate::scard) fn handle(&mut self, cmd: Command<S>) -> PduResult<Response> {
        debug!("got command: {:?}", cmd);
        debug!("command data: {}", utils::hex_data(&cmd));

        // Handle chained commands.
        let cmd = match self.pending_command.as_mut() {
            None => cmd,
            Some(pending) => {
                pending.extend_from_command(&cmd).map_err(|e| {
                    pdu_other_err!("", source: PivCardError(format!(
                        "could not build chained command: {e:?}"
                    )))
                })?;

                pending.clone()
            }
        };
        if cmd.class().chain().not_the_last() {
            self.pending_command = Some(cmd);
            return Ok(Response::new(Status::Success));
        } else {
            self.pending_command = None;
        }

        let resp = match cmd.instruction() {
            Instruction::Select => self.handle_select(cmd),
            Instruction::Verify => self.handle_verify(cmd),
            Instruction::GetData => self.handle_get_data(cmd),
            Instruction::GetResponse => self.handle_get_response(cmd),
            Instruction::GeneralAuthenticate => self.handle_general_authenticate(cmd),
            _ => {
                warn!("unimplemented instruction {:?}", cmd.instruction());
                Ok(Response::new(Status::InstructionNotSupportedOrInvalid))
            }
        }?;
        debug!("send response: {:?}", resp);
        debug!("response data: {}", utils::to_hex(&resp.encode()));
        Ok(resp)
    }

    fn handle_select(&mut self, cmd: Command<S>) -> PduResult<Response> {
        // For our use case, we only allow selecting the PIV application on the smartcard.
        //
        // P1=04 and P2=00 means selection of DF (usually) application by name. Everything else not
        // supported.
        if cmd.p1 != 0x04 || cmd.p2 != 0x00 {
            return Ok(Response::new(Status::NotFound));
        }
        if !PIV_AID.matches(cmd.data()) {
            return Ok(Response::new(Status::NotFound));
        }

        // See https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-73-4.pdf section
        // 3.1.1
        let resp = tlv(
            tlv_tags::PIV_APPLICATION_PROPERTY_TEMPLATE,
            Value::Constructed(vec![
                tlv(
                    tlv_tags::AID,
                    Value::Primitive(vec![0x00, 0x00, 0x10, 0x00, 0x01, 0x00]),
                )?,
                tlv(
                    tlv_tags::COEXISTENT_TAG_ALLOCATION_AUTHORITY,
                    Value::Constructed(vec![tlv(
                        tlv_tags::AID,
                        Value::Primitive(PIV_AID.truncated().to_vec()),
                    )?]),
                )?,
            ]),
        )?;
        Ok(Response::with_data(Status::Success, resp.to_vec()))
    }

    fn handle_verify(&mut self, cmd: Command<S>) -> PduResult<Response> {
        // See subsection '3.2.1 VERIFY Card Command' at: https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-73-4.pdf#page=87
        // + '3.2.1.1 PIV Card Application PIN and Global PIN'
        const VERIFY_P1_DEFAULT: u8 = 0x00;
        const VERIFY_P1_RESET_SECURITY_INFO: u8 = 0xFF;
        // P2 contains the key reference. We only support PIV Card Application.
        const VERIFY_P2_PIV_CARD_APPLICATION: u8 = 0x80; // 'Specific reference data' from ISO/IEC 7816-4, Section 7.5.1, Table 65

        const PIN_LEN_MIN: usize = 6;
        const PIN_LEN_MAX: usize = 8;

        if cmd.p2 != VERIFY_P2_PIV_CARD_APPLICATION {
            return Ok(Response::new(Status::KeyReferenceNotFound));
        }

        match cmd.p1 {
            VERIFY_P1_DEFAULT => {
                if self.security_status == SecurityStatus::PinVerified {
                    return Ok(Response::new(Status::Success));
                }

                if !cmd.data().is_empty() {
                    if !(PIN_LEN_MIN..=PIN_LEN_MAX).contains(&cmd.data().len()) {
                        // Incorrect PIN formatting.
                        return Ok(Response::new(Status::IncorrectDataParameter));
                    }

                    if cmd.data() == self.pin.as_bytes() {
                        self.security_status = SecurityStatus::PinVerified;
                        return Ok(Response::new(Status::Success));
                    }
                }

                // Always return max remaining retries.
                return Ok(Response::new(Status::RemainingRetries(0xF)));
            }
            VERIFY_P1_RESET_SECURITY_INFO => {
                // The standard does not specify what should happen when
                // the command data is not empty with this P1. Ignore it.
                self.security_status = SecurityStatus::PinNotVerified;
            }
            _ => return Ok(Response::new(Status::IncorrectP1OrP2Parameter)),
        };
        Ok(Response::new(Status::Success))
    }

    fn handle_get_data(&mut self, cmd: Command<S>) -> PduResult<Response> {
        // See subsection '3.1.2 GET DATA Card Command' at: https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-73-4.pdf#page=86
        // See ISO 7816-4 for other possible P1-P2 pairs.
        const GET_DATA_P1_CURRENT_DF: u8 = 0x3F;
        const GET_DATA_P2_CURRENT_DF: u8 = 0xFF;

        const TAG_LIST: u8 = 0x5C;

        if cmd.p1 != GET_DATA_P1_CURRENT_DF || cmd.p2 != GET_DATA_P2_CURRENT_DF {
            return Ok(Response::new(Status::NotFound));
        }
        let request_tlv = Tlv::from_bytes(cmd.data())
            .map_err(|e| pdu_other_err!("", source:PivCardError(format!("TLV invalid: {e:?}"))))?;
        if *request_tlv.tag() != tlv_tag(TAG_LIST)? {
            return Ok(Response::new(Status::NotFound));
        }
        match request_tlv.value() {
            Value::Primitive(tag) => match tag.as_slice() {
                CHUID_TAG => Ok(Response::with_data(Status::Success, self.chuid.clone())),
                PIV_AUTH_CERT_TAG => {
                    self.pending_response = Some(Cursor::new(self.piv_auth_cert.clone()));
                    self.handle_get_response(cmd)
                }
                CARD_CAPABILITY_CONTAINER_TAG => {
                    Ok(Response::with_data(Status::Success, self.ccc.clone()))
                }
                _ => {
                    // Some other unimplemented data object.
                    Ok(Response::new(Status::NotFound))
                }
            },
            Value::Constructed(_) => Ok(Response::new(Status::NotFound)),
        }
    }

    fn handle_get_response(&mut self, _cmd: Command<S>) -> PduResult<Response> {
        // CHUNK_SIZE is the max response data size in bytes, without resorting to "extended"
        // messages.
        const CHUNK_SIZE: usize = 256;
        match &mut self.pending_response {
            None => Ok(Response::new(Status::NotFound)),
            Some(cursor) => {
                let mut chunk = [0; CHUNK_SIZE];
                let n = cursor
                    .read(&mut chunk)
                    .map_err(|e| pdu_other_err!("", source:e))?;
                let mut chunk = chunk.to_vec();
                chunk.truncate(n);
                let remaining = cursor.get_ref().len() as u64 - cursor.position();
                let status = if remaining == 0 {
                    Status::Success
                } else if remaining < CHUNK_SIZE as u64 {
                    Status::MoreAvailable(remaining as u8)
                } else {
                    Status::MoreAvailable(0)
                };
                Ok(Response::with_data(status, chunk))
            }
        }
    }

    /// Sign the challenge.
    ///
    /// Note: for signatures, typically you'd use a signer that hashes the input data, adds padding
    /// according to some scheme (like PKCS1v15 or PSS) and then "decrypts" this data with the key.
    /// The decrypted blob is the signature.
    ///
    /// In our case, the RDP server does the hashing and padding, and only gives us a finished blob
    /// to decrypt. Most crypto libraries don't directly expose RSA decryption without padding, as
    /// it's easy to build insecure crypto systems. Thankfully for us, this decryption is just a single
    /// modpow operation which is suppored by RustCrypto.
    fn sign_auth_challenge(&self, challenge: &[u8]) -> Vec<u8> {
        let c = BigUint::from_bytes_be(challenge);
        let plain_text = c
            .modpow(self.piv_auth_key.d(), self.piv_auth_key.n())
            .to_bytes_be();

        let mut result = vec![0u8; self.piv_auth_key.size()];
        let start = result.len() - plain_text.len();
        result[start..].copy_from_slice(&plain_text);
        result
    }

    fn handle_general_authenticate(&mut self, cmd: Command<S>) -> PduResult<Response> {
        // See subsection '3.2.4 GENERAL AUTHENTICATE Card Command' at: https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-73-4.pdf#page=93
        // + 'A.3 Authentication of PIV Cardholder' at: https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-73-4.pdf#page=116
        if self.security_status != SecurityStatus::PinVerified {
            return Ok(Response::new(Status::SecurityStatusNotSatisfied));
        }

        // P1='07' means 2048-bit RSA.
        //
        // TODO(zmb3): compare algorithm against the private key using consts from
        // https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-78-4.pdf
        // TODO(zmb3): support non-RSA keys, if needed.
        if cmd.p1 != 0x07 {
            return Err(pdu_other_err!("", source: PivCardError(format!(
                "unsupported algorithm identifier P1:{:#X} in general authenticate command",
                cmd.p1
            ))));
        }
        // P2='9A' means PIV Authentication Key (matches our cert '5FC105' in handle_get_data).
        if cmd.p2 != 0x9A {
            return Err(pdu_other_err!("", source: PivCardError(format!(
                "unsupported key reference P2:{:#X} in general authenticate command",
                cmd.p2
            ))));
        }

        let request_tlv = Tlv::from_bytes(cmd.data())
            .map_err(|e| pdu_other_err!("", source: PivCardError(format!("TLV invalid: {e:?}"))))?;
        if *request_tlv.tag() != tlv_tag(tlv_tags::DYNAMIC_AUTHENTICATION_TEMPLATE)? {
            return Err(pdu_other_err!("", source: PivCardError(format!(
                "general authenticate command TLV invalid: {request_tlv:?}"
            ))));
        }

        // Extract the challenge field.
        let request_tlvs = match request_tlv.value() {
            Value::Primitive(_) => {
                return Err(pdu_other_err!("", source: PivCardError(format!(
                    "general authenticate command TLV invalid: {request_tlv:?}"
                ))));
            }
            Value::Constructed(tlvs) => tlvs,
        };
        let mut challenge = None;
        for data in request_tlvs {
            if *data.tag() != tlv_tag(tlv_tags::CHALLENGE)? {
                continue;
            }
            challenge = match data.value() {
                Value::Primitive(chal) => Some(chal),
                Value::Constructed(_) => {
                    return Err(pdu_other_err!("", source: PivCardError(format!(
                        "general authenticate command TLV invalid: {request_tlv:?}"
                    ))));
                }
            };
        }
        let challenge = challenge.ok_or_else(|| {
            pdu_other_err!("", source: PivCardError(format!(
                "general authenticate command TLV invalid: {request_tlv:?}, missing challenge data"
            )))
        })?;

        // TODO(zmb3): support non-RSA keys, if needed.
        let signed_challenge = self.sign_auth_challenge(challenge);

        // Return signed challenge.
        let resp = tlv(
            tlv_tags::DYNAMIC_AUTHENTICATION_TEMPLATE,
            Value::Constructed(vec![tlv(
                tlv_tags::RESPONSE,
                Value::Primitive(signed_challenge),
            )?]),
        )?
        .to_vec();
        self.pending_response = Some(Cursor::new(resp));
        self.handle_get_response(cmd)
    }

    fn build_ccc() -> Vec<u8> {
        // Card Capability Container, described in subsection 3.1.1. here: https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-73-4.pdf#page=18
        // Its structure is defined in Table 8 here: https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-73-4.pdf#page=40
        // The description of each field can be found here: https://nvlpubs.nist.gov/nistpubs/Legacy/IR/nistir6887e2003.pdf#page=117
        // The CCC is comprised of SIMPLE-TLV data objects.
        let mut ccc = Vec::new();

        const TLV_L_IDX: usize = 1;
        const TLV_TL_SIZE: usize = 2;

        ccc.extend_from_slice(&[
            tlv_tags::DATA_FIELD,
            0x00, // Length - fill this field dynamically at the end.
        ]);

        let mut card_identifier: Vec<u8> = vec![
            tlv_tags::CARD_IDENTIFIER,
            0x00, // Length - fill this field dynamically.
        ];
        // GSC-RID of GSA - TFCS; US.
        card_identifier.extend_from_slice(&[0xa0, 0x00, 0x00, 0x01, 0x16]);
        card_identifier.extend_from_slice(&[
            0xff, // Manufacturer ID - e.g., Yubikey uses 0xff.
            0x02, // Card Type - Java Card.
        ]);
        // CardID - random 14 bytes to fill out the rest of the Card Identifier (21 bytes).
        // Generating a unique one per CCC is more future-proof than hardcoding.
        card_identifier.extend_from_slice(&{
            let mut bytes = [0u8; 14];
            rand::rngs::OsRng.unwrap_err().fill_bytes(&mut bytes);
            bytes
        });
        card_identifier[TLV_L_IDX] = (card_identifier.len() - TLV_TL_SIZE) as u8;

        ccc.extend_from_slice(&card_identifier);
        // Capability Container Version Number (ver. 2.1)
        ccc.extend_from_slice(&[tlv_tags::CAPABILITY_CONTAINER_VERSION_NUMBER, 0x01, 0x21]);
        // Capability Grammar Version Number (ver. 2.1)
        ccc.extend_from_slice(&[tlv_tags::CAPABILITY_GRAMMAR_VERSION_NUMBER, 0x01, 0x21]);
        // Applications CardURL (none)
        ccc.extend_from_slice(&[tlv_tags::APPLICATIONS_CARDURL, 0x00]);
        // PKCS#15 (not supported)
        ccc.extend_from_slice(&[tlv_tags::PKCS15, 0x00]);
        // Registered Data Model Number (GSC-IS, see '8. Data Model': https://nvlpubs.nist.gov/nistpubs/Legacy/IR/nistir6887e2003.pdf#page=135)
        ccc.extend_from_slice(&[tlv_tags::REGISTERED_DATA_MODEL, 0x01, 0x01]);
        // Access Control Rule Table (none)
        ccc.extend_from_slice(&[tlv_tags::ACCESS_CONTROL_RULE_TABLE, 0x00]);
        // Card APDUs (none)
        ccc.extend_from_slice(&[tlv_tags::CARD_APDUS, 0x00]);
        // Redirection Tag (none)
        ccc.extend_from_slice(&[tlv_tags::REDIRECTION_TAG, 0x00]);
        // Capability Tuples (CTs) (none)
        ccc.extend_from_slice(&[tlv_tags::CAPABILITY_TUPLES, 0x00]);
        // Status Tuples (STs) (none)
        ccc.extend_from_slice(&[tlv_tags::STATUS_TUPLES, 0x00]);
        // Next CCC (none)
        ccc.extend_from_slice(&[tlv_tags::NEXT_CCC, 0x00]);
        // Error Detection Code (mandated by GSC-IS)
        ccc.extend_from_slice(&[tlv_tags::ERROR_DETECTION_CODE, 0x00]);

        ccc[TLV_L_IDX] = (ccc.len() - TLV_TL_SIZE) as u8;
        ccc
    }
}

fn tlv(tag: u8, value: Value) -> PduResult<Tlv> {
    Tlv::new(tlv_tag(tag)?, value).map_err(|e| {
        pdu_other_err!("", source: PivCardError(format!(
            "TLV with tag {tag:#X} invalid: {e:?}"
        )))
    })
}

fn tlv_tag(val: u8) -> PduResult<Tag> {
    Tag::try_from(val).map_err(|e| {
        pdu_other_err!("", source: PivCardError(format!(
            "TLV tag {val:#X} invalid: {e:?}"
        )))
    })
}

/// A generic error type for the [`PivCard`] that can contain any arbitrary error message.
#[derive(Debug)]
struct PivCardError(String);

impl std::fmt::Display for PivCardError {
    fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
        write!(f, "PivCardError({})", self.0)
    }
}

impl std::error::Error for PivCardError {}

#[cfg(test)]
mod tests {
    extern crate std;

    use base64::{engine::general_purpose, Engine as _};
    use picky::key::PrivateKey;

    use super::*;

    const MAX_APDU_SIZE: usize = 1024;

    fn new_test_card() -> Card<MAX_APDU_SIZE> {
        let uuid = Uuid::new_v4();
        let cert_der_stub = vec![0xff; 1024];
        // Random RSA 2048 private key, encoded with Base64 for obfuscation.
        // Hardcoded because generating it with `picky` is quite slow in debug builds.
        let key_pem_base64 = "LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcGdJQkFBS0NBUUVBelI3SFNYSEUvdHZHditCZ1A1amwxSGZtcTMxOGwvOEp2Q3lGdVo4MmRGNHN1OTlGCmdYU2hxS3NsaloxczNnZStuZlhzdVJ6TGN4N3ZGWDA5ajdXR254SmNsUENnMGFvUWJkSWxsMUlYWXprUDdwQlkKbkIwS09JalhNL2hEdXdjQ1VENldkOUtzWEVqQzYvS3lrSk5RbzhIWi9QU20zQm9CTFhTUUgwVUk1Z3N6Znk3egpoQU1NZksrRTJtbjlUVkRDMU1wekJ1aXZhaXZIQk5LV1BaYjJrYUV1STF0N0h0V01peGdycFJhQ1czdzNCVnFYCi9DQ3lxTzhBazFWdTh5NFc5ZlEwMWNMakZFN1BIK0lieElNblVwYWtNMmhsRDhVUTdydXN2ZzQydXR4OXN6ZVoKVW1seEJqTVRJS2xYNGZxQ2VSUHpJR2pTVFVmZUpPOEJOY2toNndJREFRQUJBb0lCQVFDOGRDNjh0NTQ2V1VtOQpPRFMxdVpCUEtPbnhYUlYvd0gzOU9ScVRkRWZmbWphWFZZYlNiWW1wSWJVYTZ5dit3amJMZ2dxLytFaWp1Q1FKCkprdk5JUVpTVjloZnJzVFNYT0ZEUlBQc2x5dU5xZnVOUDJscDVQUmpCTFpUdFNEbFVJYzdtb1U4Q1g3Nk9vOTcKb0R1V3dRSXhzZ1RKZHkxbXd5Mlp5YUl3V0lqWWNJUDVqMDIwMTJHaUVIUTFDRFBpT0tBSU9WWHd5ZG9FWGZyRgpVMVlXK2lncnE0Wno2ZmpCbk54Z2VGYUlsWTBodUUxb044V3U1RGtwWkw5VzhwVG9IQTIrL1RUUGhLTDFEY1hYCkF3RWk0a0l2bDNRdFQ5NDBEVE5mY2luV3R2MzY2WmNJSzg1eTVhckVQWm5FRk1yM3JCdUVSb1FYR0t4OVRhUjQKeXNISGZEM2hBb0dCQVBZQW1HTXlsWGhUcWZaNjhNaVJUSGRKWkplMXFLN29jdlBQRXliTnZwNnluQ25QeHkwWApsblNVUkJsYzZvZFByekV4QVloWGVnWm1jVytNRDRDd05BSnBDdUhmVENPWUREM1dsVTNwRWRjdytQeU02OW43Ckh4SkhzRkM1K1c1R28vUGZpd09hb2FEdVEybVk2WGM2NVUrRWkva2ZhYUdZdTNkNVFKTWJaT3JWQW9HQkFOVjAKMkxBOUluTS9FZ01tbWdxemJTR05PcERXS0VRUWZnREJxMEJBTldnbjNDelNUMzVMYy9WbXNkTU1MQmVuZlcrSwpSM3c0Qlo3UlVoUGNmbE04T0o5aEVxcjhWNHdpcGphNUZLWFZLM1pqWStzbGJHT2IxaE1raDRUY3JHVEpqOHAyCkcrRzV6Y1pzOTY1STgwcmZyMFBiclFoSVZnL0ZxajEwRGw3Sm9ybS9Bb0dCQVBKSlJjMDFqZGRUOTJybVRPNE4KaFJWYmVMS2UzVU5mZDVBL20rbzA2NUJiODhpT2R1b25lQ3pidG5LUWZBREc3NUp3Wk1VRyt3MEFxcXFsZE1OWApSL0l6eU44TDBXNmhHelZ3ZWQ2aE5jd08xTHZRZzU1T1laemNkSUFkbXRnTXhQKzFaTElwQXhXQWRXNjBod1RDClFnVmVVNG9LY1R3U05Ga0lXQnhLOThyOUFvR0JBSVNWamw1eHFxdFEycHhRWnRBTXdOVmRScXBlQ3lhejQ4QU8KaTVOZURvNUNhL1QvTU5jdWdMbEY3MkE2cUV5TkFWRzkzMGNkK1FlNzFySjFlNVd4eXkzYit0OXYyK1UwUkcrcgpLRk1WQkdrRnRUT0N6RDlXdFhLd2R1aWt0UVBwV3NJVCtKK05iRzQ2a3VHVGVHTGlhNWZIcEVPSHdzVUxMd0g2CnkwNC9DaTg3QW9HQkFMRmtKQWp4V3llRms1Q0lXNUpMRHhHTWZXRllDNm0zUnhXWFJmSGVEYWJWb08xSEk4RGkKcm85Wi9WZzFtN2xtM1VFSlZXTldTS0N1NkVoUnZMTUJlaHk5WC9CWkVJQ0JsSkxTamtHcGhvM1RhM1dLUjFTUApBb1dOTWw5ODJPeGlISzhXY1pMQ0h6ZlBIMHpyajJ2OHhKV3FsbjN6cEdyTEVpeGFBekl1ZXpHSAotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQ==";
        let key_pem_bytes = general_purpose::STANDARD.decode(key_pem_base64).unwrap();
        let key_pem = String::from_utf8(key_pem_bytes).unwrap();
        let key_der = PrivateKey::from_pem_str(&key_pem)
            .unwrap()
            .to_pkcs1()
            .unwrap();
        let pin = "12345678".to_string();
        Card::new(uuid, cert_der_stub.as_slice(), key_der.as_slice(), pin).unwrap()
    }

    // Helper function for handling the `MoreAvailable` status by invoking the GET RESPONSE
    // handler repeatedly until no more readable data is available.
    fn handle_more_available_status(
        mut response: Response,
        scard: &mut Card<MAX_APDU_SIZE>,
    ) -> Vec<u8> {
        let mut complete_response = vec![];

        // Keep sending GET RESPONSE until a success status.
        while let Status::MoreAvailable(bytes_left) = response.status {
            complete_response
                .extend_from_slice(&response.data.expect("Should be more data available"));
            let get_response_cmd =
                Command::<MAX_APDU_SIZE>::try_from(&[0x00, 0xC0, 0x00, 0x00, bytes_left]).unwrap();
            response = scard
                .handle(get_response_cmd)
                .expect("Should have retrieved more available data");
        }
        assert_eq!(response.status, Status::Success);

        complete_response.extend_from_slice(
            &response
                .data
                .expect("Should have retrieved the last of available data"),
        );
        complete_response
    }

    // TODO(apri-dh): the rest of the command tests.

    #[test]
    fn get_data_valid_commands() {
        // Test the GET DATA handler's CHUID and PIV auth cert retrieval capabilities.
        let mut scard = new_test_card();

        // Get the Card Holder Unique Identifier (CHUID).
        let get_data_chuid = Command::<MAX_APDU_SIZE>::try_from(&[
            0x00, 0xCB, 0x3F, 0xFF, 0x05, 0x5C, 0x03, 0x5F, 0xC1, 0x02,
        ])
        .unwrap();
        let response = scard.handle(get_data_chuid);
        assert!(response.is_ok_and(|resp| resp.status == Status::Success
            && resp.data.expect("Should have retrieved CHUID") == scard.chuid));

        // Get the Card Capability Container (CCC).
        let get_data_ccc = Command::<MAX_APDU_SIZE>::try_from(&[
            0x00, 0xCB, 0x3F, 0xFF, 0x05, 0x5C, 0x03, 0x5F, 0xC1, 0x07,
        ])
        .unwrap();
        let response = scard.handle(get_data_ccc);
        assert!(response.is_ok_and(|resp| resp.status == Status::Success
            && resp.data.expect("Should have retrieved CCC") == scard.ccc));

        // Get the PIV authentication certificate.
        let get_data_piv_auth_cert = Command::<MAX_APDU_SIZE>::try_from(&[
            0x00, 0xCB, 0x3F, 0xFF, 0x05, 0x5C, 0x03, 0x5F, 0xC1, 0x05,
        ])
        .unwrap();
        let response = scard.handle(get_data_piv_auth_cert);
        assert!(response.is_ok_and(|resp| {
            // The certificate won't fit in just one APDU, so a `MoreAvailable` status will be returned
            // repeatedly to retrieve the whole certificate with GET RESPONSE commands.
            let piv_auth_cert = handle_more_available_status(resp, &mut scard);
            piv_auth_cert == scard.piv_auth_cert
        }));
    }

    #[test]
    fn get_data_invalid_commands() {
        // Test the GET DATA handler's APDU error handling capabilities.
        let mut scard = new_test_card();

        // Unsupported identifier in P1-P2 (DEAD instead of 3FFF for current DF).
        let get_data_unsupported_p1p2 =
            Command::<MAX_APDU_SIZE>::try_from(&[0x00, 0xCB, 0xDE, 0xAD, 0x00]).unwrap();
        let response = scard.handle(get_data_unsupported_p1p2);
        assert!(response.is_ok_and(|resp| resp.status == Status::NotFound));

        // Invalid tag (BADBAD instead of e.g., 5FC102 for CHUID)
        let get_data_invalid_tag = Command::<MAX_APDU_SIZE>::try_from(&[
            0x00, 0xCB, 0x3F, 0xFF, 0x05, 0x5C, 0x03, 0xBA, 0xDB, 0xAD,
        ])
        .unwrap();
        let response = scard.handle(get_data_invalid_tag);
        assert!(response.is_ok_and(|resp| resp.status == Status::NotFound));
    }

    #[test]
    fn verify_valid_commands() {
        // Test the VERIFY handler's PIN verification capabilities.
        let mut scard = new_test_card();
        assert!(scard.security_status == SecurityStatus::PinNotVerified);

        // Query the number of remaining tries; it should be non-zero.
        let verify_empty =
            Command::<MAX_APDU_SIZE>::try_from(&[0x00, 0x20, 0x00, 0x80, 0x00]).unwrap();
        let response = scard.handle(verify_empty);
        assert!(
            response.is_ok_and(|resp| matches!(resp.status, Status::RemainingRetries(n) if n > 0))
        );

        // Try to pass an invalid but correctly formatted PIN code.
        let mut verify_invalid_pin =
            Command::<MAX_APDU_SIZE>::try_from(&[0x00, 0x20, 0x00, 0x80, 0x08]).unwrap();
        verify_invalid_pin
            .data_mut()
            .extend_from_slice(&[0; 8])
            .unwrap();
        let response = scard.handle(verify_invalid_pin);
        assert!(response.is_ok_and(|resp| matches!(resp.status, Status::RemainingRetries(_))));

        // Try to pass an incorrectly formatted PIN code (too short).
        let mut verify_too_short_pin =
            Command::<MAX_APDU_SIZE>::try_from(&[0x00, 0x20, 0x00, 0x80, 0x03]).unwrap();
        verify_too_short_pin
            .data_mut()
            .extend_from_slice(&[0; 3])
            .unwrap();
        let response = scard.handle(verify_too_short_pin);
        assert!(response.is_ok_and(|resp| matches!(resp.status, Status::IncorrectDataParameter)));

        // Try to pass the correct PIN.
        let mut verify_correct_pin =
            Command::<MAX_APDU_SIZE>::try_from(&[0x00, 0x20, 0x00, 0x80, 0x08]).unwrap();
        verify_correct_pin
            .data_mut()
            .extend_from_slice(scard.pin.as_bytes())
            .unwrap();
        let response = scard.handle(verify_correct_pin);
        assert!(response.is_ok_and(|resp| resp.status == Status::Success));
        assert!(scard.security_status == SecurityStatus::PinVerified);

        // Reset the security status.
        let verify_reset =
            Command::<MAX_APDU_SIZE>::try_from(&[0x00, 0x20, 0xFF, 0x80, 0x00]).unwrap();
        let response = scard.handle(verify_reset);
        assert!(response.is_ok_and(|resp| resp.status == Status::Success));
        assert!(scard.security_status == SecurityStatus::PinNotVerified);
    }

    #[test]
    fn verify_invalid_commands() {
        // Test the VERIFY handler's APDU error handling capabilities.
        let mut scard = new_test_card();

        // Unsupported P2.
        let verify_invalid_p2_key_reference =
            Command::<MAX_APDU_SIZE>::try_from(&[0x00, 0x20, 0x00, 0x42, 0x00]).unwrap();
        let response = scard.handle(verify_invalid_p2_key_reference);
        assert!(response.is_ok_and(|resp| resp.status == Status::KeyReferenceNotFound));

        // Unsupported P1.
        let verify_invalid_p1 =
            Command::<MAX_APDU_SIZE>::try_from(&[0x00, 0x20, 0x42, 0x80, 0x00]).unwrap();
        let response = scard.handle(verify_invalid_p1);
        assert!(response.is_ok_and(|resp| resp.status == Status::IncorrectP1OrP2Parameter));
    }

    #[test]
    fn general_authenticate_pre_verify() {
        // Test the GENERAL AUTHENTICATE handler's adherence to the pre-PIN verification behavior
        // from 'A.3 Authentication of PIV Cardholder' at: https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-73-4.pdf#page=116
        let mut scard = new_test_card();
        assert!(scard.security_status == SecurityStatus::PinNotVerified);

        let gen_auth = Command::<MAX_APDU_SIZE>::try_from(&[0x00, 0x87, 0x07, 0x9A, 0x00]).unwrap();
        let response = scard.handle(gen_auth);
        assert!(response.is_ok_and(|resp| resp.status == Status::SecurityStatusNotSatisfied));
    }
}
