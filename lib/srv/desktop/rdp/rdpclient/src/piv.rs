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

use crate::rdpdr::TeleportRdpdrBackendError;
use ironrdp_pdu::{pdu_other_err, PduResult};
use iso7816::aid::Aid;
use iso7816::command::instruction::Instruction;
use iso7816::command::Command;
use iso7816::response::Status;
use iso7816_tlv::ber::{Tag, Tlv, Value};
use log::{debug, warn};
use rsa::pkcs1::DecodeRsaPrivateKey;
use rsa::traits::{PrivateKeyParts, PublicKeyParts};
use rsa::{BigUint, RsaPrivateKey};
use std::convert::TryFrom;
use std::fmt::Write as _;
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

// Card implements a PIV-compatible smartcard, per:
// https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-73-4.pdf
#[derive(Debug, PartialEq, Eq)]
pub struct Card<const S: usize> {
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
}

impl<const S: usize> Card<S> {
    pub fn new(uuid: Uuid, cert_der: &[u8], key_der: &[u8], pin: String) -> PduResult<Self> {
        let piv_auth_key = RsaPrivateKey::from_pkcs1_der(key_der)
            .map_err(|_e| pdu_other_err!("failed to parse private key from DER"))?;

        Ok(Self {
            chuid: Self::build_chuid(uuid),
            piv_auth_cert: Self::build_piv_auth_cert(cert_der),
            piv_auth_key,
            pin,
            pending_command: None,
            pending_response: None,
        })
    }

    pub fn handle(&mut self, cmd: Command<S>) -> PduResult<Response> {
        debug!("got command: {:?}", cmd);
        debug!("command data: {}", hex_data(&cmd));

        // Handle chained commands.
        let cmd = match self.pending_command.as_mut() {
            None => cmd,
            Some(pending) => {
                pending.extend_from_command(&cmd).map_err(|e| {
                    pdu_other_err!("", source: TeleportRdpdrBackendError(format!(
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
        debug!("response data: {}", to_hex(&resp.encode()));
        Ok(resp)
    }

    fn handle_select(&mut self, cmd: Command<S>) -> PduResult<Response> {
        // For our use case, we only allow selecting the PIV application on the smartcard.
        //
        // P1=04 and P2=00 means selection of DF (usually) application by name. Everything else not
        // supported.
        if cmd.p1 != 0x04 && cmd.p2 != 0x00 {
            return Ok(Response::new(Status::NotFound));
        }
        if !PIV_AID.matches(cmd.data()) {
            return Ok(Response::new(Status::NotFound));
        }

        // See https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-73-4.pdf section
        // 3.1.1
        let resp = tlv(
            TLV_TAG_PIV_APPLICATION_PROPERTY_TEMPLATE,
            Value::Constructed(vec![
                tlv(
                    TLV_TAG_AID,
                    Value::Primitive(vec![0x00, 0x00, 0x10, 0x00, 0x01, 0x00]),
                )?,
                tlv(
                    TLV_TAG_COEXISTENT_TAG_ALLOCATION_AUTHORITY,
                    Value::Constructed(vec![tlv(
                        TLV_TAG_AID,
                        Value::Primitive(PIV_AID.truncated().to_vec()),
                    )?]),
                )?,
            ]),
        )?;
        Ok(Response::with_data(Status::Success, resp.to_vec()))
    }

    fn handle_verify(&mut self, cmd: Command<S>) -> PduResult<Response> {
        if cmd.data() == self.pin.as_bytes() {
            Ok(Response::new(Status::Success))
        } else {
            warn!("PIN mismatch, want {}, got {:?}", self.pin, cmd.data());
            Ok(Response::new(Status::VerificationFailed))
        }
    }

    fn handle_get_data(&mut self, cmd: Command<S>) -> PduResult<Response> {
        // See https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-73-4.pdf section
        // 3.1.2.
        if cmd.p1 != 0x3F && cmd.p2 != 0xFF {
            return Ok(Response::new(Status::NotFound));
        }
        let request_tlv = Tlv::from_bytes(cmd.data()).map_err(
            |e| pdu_other_err!("", source:TeleportRdpdrBackendError(format!("TLV invalid: {e:?}"))),
        )?;
        if *request_tlv.tag() != tlv_tag(0x5C)? {
            return Ok(Response::new(Status::NotFound));
        }
        match request_tlv.value() {
            Value::Primitive(tag) => match to_hex(tag).as_str() {
                // Card Holder Unique Identifier.
                "5FC102" => Ok(Response::with_data(Status::Success, self.chuid.clone())),
                // X.509 Certificate for PIV Authentication
                "5FC105" => {
                    self.pending_response = Some(Cursor::new(self.piv_auth_cert.clone()));
                    self.handle_get_response(cmd)
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
        // See section 3.2.4 and example in Appending A.3 from
        // https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-73-4.pdf

        // P1='07' means 2048-bit RSA.
        //
        // TODO(zmb3): compare algorithm against the private key using consts from
        // https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-78-4.pdf
        // TODO(zmb3): support non-RSA keys, if needed.
        if cmd.p1 != 0x07 {
            return Err(pdu_other_err!("", source:TeleportRdpdrBackendError(format!(
                "unsupported algorithm identifier P1:{:#X} in general authenticate command",
                cmd.p1
            ))));
        }
        // P2='9A' means PIV Authentication Key (matches our cert '5FC105' in handle_get_data).
        if cmd.p2 != 0x9A {
            return Err(pdu_other_err!("", source:TeleportRdpdrBackendError(format!(
                "unsupported key reference P2:{:#X} in general authenticate command",
                cmd.p2
            ))));
        }

        let request_tlv = Tlv::from_bytes(cmd.data()).map_err(
            |e| pdu_other_err!("", source:TeleportRdpdrBackendError(format!("TLV invalid: {e:?}"))),
        )?;
        if *request_tlv.tag() != tlv_tag(TLV_TAG_DYNAMIC_AUTHENTICATION_TEMPLATE)? {
            return Err(pdu_other_err!("", source:TeleportRdpdrBackendError(format!(
                "general authenticate command TLV invalid: {request_tlv:?}"
            ))));
        }

        // Extract the challenge field.
        let request_tlvs = match request_tlv.value() {
            Value::Primitive(_) => {
                return Err(pdu_other_err!("", source:TeleportRdpdrBackendError(format!(
                    "general authenticate command TLV invalid: {request_tlv:?}"
                ))));
            }
            Value::Constructed(tlvs) => tlvs,
        };
        let mut challenge = None;
        for data in request_tlvs {
            if *data.tag() != tlv_tag(TLV_TAG_CHALLENGE)? {
                continue;
            }
            challenge = match data.value() {
                Value::Primitive(chal) => Some(chal),
                Value::Constructed(_) => {
                    return Err(pdu_other_err!("", source:TeleportRdpdrBackendError(format!(
                        "general authenticate command TLV invalid: {request_tlv:?}"
                    ))));
                }
            };
        }
        let challenge = challenge.ok_or_else(|| {
            pdu_other_err!("", source:TeleportRdpdrBackendError(format!(
                "general authenticate command TLV invalid: {request_tlv:?}, missing challenge data"
            )))
        })?;

        // TODO(zmb3): support non-RSA keys, if needed.
        let signed_challenge = self.sign_auth_challenge(challenge);

        // Return signed challenge.
        let resp = tlv(
            TLV_TAG_DYNAMIC_AUTHENTICATION_TEMPLATE,
            Value::Constructed(vec![tlv(
                TLV_TAG_RESPONSE,
                Value::Primitive(signed_challenge),
            )?]),
        )?
        .to_vec();
        self.pending_response = Some(Cursor::new(resp));
        self.handle_get_response(cmd)
    }

    fn build_chuid(uuid: Uuid) -> Vec<u8> {
        // This is gross: the response is a BER-TLV value, but it has nested SIMPLE-TLV
        // values. None of the TLV encoding libraries out there support this, they fail
        // when checking the tag of nested TLVs.
        //
        // So, construct the TLV by hand from raw bytes. Hopefully the comments will be
        // enough to explain the structure.
        //
        // https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-73-4.pdf
        // table 9 has the explanation of fields.
        //
        // Start with a top-level BER-TLV tag and length:
        let mut resp = vec![TLV_TAG_DATA_FIELD, 0x3B];
        // TLV tag and length for FASC-N.
        resp.extend_from_slice(&[TLV_TAG_FASC_N, 0x19]);
        // FASC-N value containing S9999F9999F999999F0F1F0000000000300001E, with a
        // weird encoding from section 6 of:
        // https://www.idmanagement.gov/docs/pacs-tig-scepacs.pdf
        resp.extend_from_slice(&[
            0xd4, 0xe7, 0x39, 0xda, 0x73, 0x9c, 0xed, 0x39, 0xce, 0x73, 0x9d, 0x83, 0x68, 0x58,
            0x21, 0x08, 0x42, 0x10, 0x84, 0x21, 0xc8, 0x42, 0x10, 0xc3, 0xeb,
        ]);
        // TLV for user UUID.
        resp.extend_from_slice(&[TLV_TAG_GUID, 0x10]);
        resp.extend_from_slice(uuid.as_bytes());
        // TLV for expiration date (YYYYMMDD).
        resp.extend_from_slice(&[TLV_TAG_EXPIRATION_DATE, 0x08]);
        // TODO(awly): generate this from current time.
        resp.extend_from_slice("20300101".as_bytes());
        // TLV for signature (empty).
        resp.extend_from_slice(&[TLV_TAG_ISSUER_ASYMMETRIC_SIGNATURE, 0x00]);
        // TLV for error detection code.
        resp.extend_from_slice(&[TLV_TAG_ERROR_DETECTION_CODE, 0x00]);
        resp
    }

    fn build_piv_auth_cert(cert_der: &[u8]) -> Vec<u8> {
        // Same as above, tags in this BER-TLV value are not compatible with the spec
        // and existing libraries. Marshal by hand.
        //
        // Certificate TLV tag and length.
        let mut resp = vec![TLV_TAG_CERTIFICATE];
        resp.extend_from_slice(&len_to_vec(cert_der.len()));
        resp.extend_from_slice(cert_der);
        // CertInfo TLV (0x00 indicates uncompressed cert).
        resp.extend_from_slice(&[TLV_TAG_CERTINFO, 0x01, 0x00]);
        // TLV for error detection code.
        resp.extend_from_slice(&[TLV_TAG_ERROR_DETECTION_CODE, 0x00]);

        // Wrap with top-level TLV tag and length.
        let mut resp_outer = vec![TLV_TAG_DATA_FIELD];
        resp_outer.extend_from_slice(&len_to_vec(resp.len()));
        resp_outer.extend_from_slice(&resp);
        resp_outer
    }
}

#[derive(Debug)]
pub struct Response {
    data: Option<Vec<u8>>,
    status: Status,
}

impl Response {
    fn new(status: Status) -> Self {
        Self { data: None, status }
    }
    fn with_data(status: Status, data: Vec<u8>) -> Self {
        Self {
            data: Some(data),
            status,
        }
    }

    pub fn encode(&self) -> Vec<u8> {
        let mut buf = Vec::new();
        if let Some(data) = &self.data {
            buf.extend_from_slice(data);
        }
        let status: [u8; 2] = self.status.into();
        buf.extend_from_slice(&status);
        buf
    }
}

// SELECT command tags.
const TLV_TAG_PIV_APPLICATION_PROPERTY_TEMPLATE: u8 = 0x61;
const TLV_TAG_AID: u8 = 0x4F;
const TLV_TAG_COEXISTENT_TAG_ALLOCATION_AUTHORITY: u8 = 0x79;
const TLV_TAG_DATA_FIELD: u8 = 0x53;
const TLV_TAG_FASC_N: u8 = 0x30;
const TLV_TAG_GUID: u8 = 0x34;
const TLV_TAG_EXPIRATION_DATE: u8 = 0x35;
const TLV_TAG_ISSUER_ASYMMETRIC_SIGNATURE: u8 = 0x3E;
const TLV_TAG_ERROR_DETECTION_CODE: u8 = 0xFE;
const TLV_TAG_CERTIFICATE: u8 = 0x70;
const TLV_TAG_CERTINFO: u8 = 0x71;
// GENERAL AUTHENTICATE command tags.
const TLV_TAG_DYNAMIC_AUTHENTICATION_TEMPLATE: u8 = 0x7C;
const TLV_TAG_CHALLENGE: u8 = 0x81;
const TLV_TAG_RESPONSE: u8 = 0x82;

fn tlv(tag: u8, value: Value) -> PduResult<Tlv> {
    Tlv::new(tlv_tag(tag)?, value).map_err(|e| {
        pdu_other_err!("", source:TeleportRdpdrBackendError(format!(
            "TLV with tag {tag:#X} invalid: {e:?}"
        )))
    })
}

fn tlv_tag(val: u8) -> PduResult<Tag> {
    Tag::try_from(val).map_err(|e| {
        pdu_other_err!("", source:TeleportRdpdrBackendError(format!(
            "TLV tag {val:#X} invalid: {e:?}"
        )))
    })
}

fn hex_data<const S: usize>(cmd: &Command<S>) -> String {
    to_hex(cmd.data())
}

fn to_hex(bytes: &[u8]) -> String {
    let mut s = String::new();
    for b in bytes {
        // https://rust-lang.github.io/rust-clippy/master/index.html#format_push_string
        let _ = write!(s, "{b:02X}");
    }
    s
}

#[allow(clippy::cast_possible_truncation)]
fn len_to_vec(len: usize) -> Vec<u8> {
    if len < 0x7f {
        vec![len as u8]
    } else {
        let mut ret: Vec<u8> = len
            .to_be_bytes()
            .iter()
            .skip_while(|&x| *x == 0)
            .cloned()
            .collect();
        ret.insert(0, 0x80 | ret.len() as u8);
        ret
    }
}
