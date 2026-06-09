use std::collections::HashSet;
use std::mem::size_of;
use std::ptr;
use std::ptr::{addr_of_mut, null_mut, slice_from_raw_parts};

use anyhow::{Context, Result};
use log::warn;
use rand::RngCore;
use serde::Deserialize;
use windows::{core::*, Win32::Foundation::*, Win32::Security::Cryptography::*};

use crate::crypto::LicenseType::{Enterprise, Unknown, OSS};
use crate::crypto::UserCreation::{No, Yes};

#[derive(Debug, Eq, PartialEq)]
pub enum LicenseType {
    OSS,
    Enterprise,
    Unknown,
}

#[derive(Deserialize)]
struct CreateUserOptions {
    #[serde(rename = "createUser")]
    create_user: bool,
    #[serde(rename = "groups")]
    groups: Option<HashSet<String>>,
}

#[derive(Default)]
pub struct CryptContext {
    handle: usize,
    cert: Option<*mut CERT_CONTEXT>,
}

const PROVIDER: PCWSTR = w!("Microsoft Base Smart Card Crypto Provider");
const CONTAINER: PCWSTR = w!("");

// OID constants have to be kept in sync with teleport (tls/ca.go)
const SMART_CARD_USAGE_OID: PCSTR = s!("1.3.6.1.4.1.311.20.2.2");
const CREATE_USER_EXT_OID: PCSTR = s!("1.3.9999.2.16");
const LICENSE_EXT_OID: PCSTR = s!("1.3.9999.2.14");
const DESKTOPS_COUNT_EXT_OID: PCSTR = s!("1.3.9999.2.17");
const AD_EXT_OID: PCSTR = s!("1.3.9999.2.22");

pub enum UserCreation {
    Yes(HashSet<String>),
    No,
}

impl CryptContext {
    pub fn new() -> Result<CryptContext> {
        let mut ctx = CryptContext::default();
        unsafe {
            CryptAcquireContextW(
                &mut ctx.handle,
                CONTAINER,
                PROVIDER,
                AT_KEYEXCHANGE.0,
                CRYPT_SILENT,
            )
            .context("Can't acquire context")?;
        }
        Ok(ctx)
    }

    fn init_cert(&mut self) -> Result<*mut CERT_CONTEXT> {
        if let Some(ctx) = self.cert {
            return Ok(ctx);
        }
        let bytes = certificate_bytes(self.handle)?;
        let encoding = X509_ASN_ENCODING | PKCS_7_ASN_ENCODING;
        let context = unsafe { CertCreateCertificateContext(encoding, &bytes) };
        if context.is_null() {
            return Err(Error::from_thread()).context("Can't create certificate context");
        }
        self.cert = Some(context);
        Ok(context)
    }

    pub fn validate(&mut self) -> Result<()> {
        let cert = self.init_cert()?;
        let mut chain_ctx: *mut CERT_CHAIN_CONTEXT = null_mut();
        let mut smart_card_usage = PSTR::from_raw(SMART_CARD_USAGE_OID.as_ptr() as *mut u8);
        let chain_para = CERT_CHAIN_PARA {
            cbSize: size_of::<CERT_CHAIN_PARA>() as u32,
            RequestedUsage: CERT_USAGE_MATCH {
                dwType: USAGE_MATCH_TYPE_AND,
                Usage: CTL_USAGE {
                    // we match single extKeyUsage - smart card
                    cUsageIdentifier: 1,
                    rgpszUsageIdentifier: &mut smart_card_usage,
                },
            },
            ..Default::default()
        };
        unsafe {
            CertGetCertificateChain(None, cert, None, None, &chain_para, 0, None, &mut chain_ctx)
                .context("Can't create certificate chain")?;
        };
        let status: Result<()> = match unsafe { *chain_ctx }.TrustStatus.dwErrorStatus {
            0 => Ok(()),
            CERT_TRUST_IS_UNTRUSTED_ROOT => Err(Error::from(STATUS_ISSUING_CA_UNTRUSTED).into()),
            CERT_TRUST_IS_NOT_TIME_VALID => Err(Error::from(STATUS_SMARTCARD_CERT_EXPIRED).into()),
            CERT_TRUST_IS_REVOKED => Err(Error::from(STATUS_SMARTCARD_CERT_REVOKED).into()),
            other => {
                warn!("Certificate chain validation failed with error status: {:#010x}", other);
                Err(Error::from(STATUS_PKINIT_CLIENT_FAILURE).into())
            }
        };
        unsafe { CertFreeCertificateChain(chain_ctx) };
        status.context("Certificate is invalid")
    }

    pub fn should_create_user(&mut self) -> Result<UserCreation> {
        let cert = self.init_cert()?;
        if let Some(data) = Self::get_extension(cert, CREATE_USER_EXT_OID) {
            let options = serde_json::from_slice::<CreateUserOptions>(&data).context(format!(
                "Create user extension was invalid: {}",
                String::from_utf8_lossy(&data)
            ))?;
            if options.create_user {
                return Ok(Yes(options.groups.unwrap_or_default()));
            }
        }
        Ok(No)
    }

    pub fn ad_desktop(&mut self) -> Result<bool> {
        let cert = self.init_cert()?;
        Ok(Self::get_extension(cert, AD_EXT_OID).is_some())
    }

    pub fn get_license(&mut self) -> Result<LicenseType> {
        let cert = self.init_cert()?;
        Ok(match Self::get_extension(cert, LICENSE_EXT_OID) {
            None => Unknown,
            Some(data) => {
                if data.eq("ent".as_bytes()) {
                    Enterprise
                } else {
                    OSS
                }
            }
        })
    }

    pub fn desktops_limit_exceeded(&mut self) -> Result<bool> {
        let cert = self.init_cert()?;
        match Self::get_extension(cert, DESKTOPS_COUNT_EXT_OID) {
            None => Err(Error::from(STATUS_CTX_LICENSE_NOT_AVAILABLE))
                .context("Can't get number of desktops"),
            Some(data) => Ok(data.ne("false".as_bytes())),
        }
    }

    fn get_extension(cert: *mut CERT_CONTEXT, oid: PCSTR) -> Option<Vec<u8>> {
        let info = unsafe { *(*cert).pCertInfo };
        let parts_mut = slice_from_raw_parts(info.rgExtension, info.cExtension as usize);
        let ext = unsafe { CertFindExtension(oid, &*parts_mut) };
        if ext.is_null() {
            return None;
        }
        let value = unsafe { (*ext).Value };
        let mut buf = vec![0u8; value.cbData as usize];
        unsafe { ptr::copy(value.pbData, buf.as_mut_ptr(), value.cbData as usize) };
        Some(buf)
    }

    pub fn username_from_certificate(&mut self) -> Result<String> {
        let cert = self.init_cert()?;
        let username_length =
            unsafe { CertGetNameStringA(cert, CERT_NAME_SIMPLE_DISPLAY_TYPE, 0, None, None) };
        let mut buf = vec![0u8; username_length as usize];
        let size = unsafe {
            CertGetNameStringA(cert, CERT_NAME_SIMPLE_DISPLAY_TYPE, 0, None, Some(&mut buf))
        };
        // strip terminating \0
        buf.truncate((size - 1) as usize);
        let name = String::from_utf8(buf)?;
        Ok(name)
    }

    fn set_pin(&self, pin: *const u8) -> windows::core::Result<()> {
        unsafe { CryptSetProvParam(self.handle, PP_KEYEXCHANGE_PIN, pin, 0) }
    }

    // sign_and_verify is used to verify that we have valid certificate and private key on smart card presented.
    // It generates random data, signs it and verify that signature using public key
    pub fn sign_and_verify(&mut self, pin: *const u8) -> Result<()> {
        let cert = self.init_cert()?;
        self.set_pin(pin).context("Can't set PIN")?;
        let mut h = Hash(0);
        let data = generate_random_data();
        let mut key = Key(0);
        unsafe {
            // create hash object
            CryptCreateHash(self.handle, CALG_SHA_256, 0, 0, &mut h.0)
                .context("Can't create hash")?;

            // feed data to the smart card CSP, which computes the digest
            CryptHashData(h.0, &data, 0).context("Can't hash data")?;

            // get signature length
            let mut size = 0u32;
            CryptSignHashA(h.0, AT_KEYEXCHANGE.0, None, 0, None, &mut size)
                .context("Can't sign data")?;

            // sign data
            let mut data = vec![0u8; size as usize];
            CryptSignHashA(
                h.0,
                AT_KEYEXCHANGE.0,
                None,
                0,
                Some(data.as_mut_ptr()),
                &mut size,
            )
            .context("Can't sign data")?;

            // set public key to verify signature, it's the one stored on SmartCard
            CryptImportPublicKeyInfo(
                self.handle,
                X509_ASN_ENCODING | PKCS_7_ASN_ENCODING,
                addr_of_mut!((*(*cert).pCertInfo).SubjectPublicKeyInfo),
                &mut key.0,
            )
            .context("Can't import public key")?;

            // verify signature
            CryptVerifySignatureA(h.0, &data, key.0, None, 0).context("Can't verify signature")?;
        }
        Ok(())
    }
}

fn generate_random_data() -> [u8; 256] {
    let mut data = [0u8; 256];
    rand::rngs::OsRng.fill_bytes(&mut data);
    data
}

fn certificate_bytes(hprov: usize) -> Result<Vec<u8>> {
    let mut u = UserKey(0);
    let mut size = 0u32;
    unsafe {
        CryptGetUserKey(hprov, AT_KEYEXCHANGE.0, &mut u.0).context("Can't get user key")?;
        CryptGetKeyParam(u.0, KP_CERTIFICATE, None, &mut size, 0)
            .context("Can't get certificate size")?;
    }
    let mut data = vec![0u8; size as usize];

    unsafe {
        CryptGetKeyParam(u.0, KP_CERTIFICATE, Some(data.as_mut_ptr()), &mut size, 0)
            .context("Can't get certificate")?;
    }

    Ok(data)
}

impl Drop for CryptContext {
    fn drop(&mut self) {
        if self.handle != 0 {
            if let Err(e) = unsafe { CryptReleaseContext(self.handle, 0) } {
                warn!("Can't release context: {}", e)
            }
        }
        if let Some(cert) = self.cert {
            if let Err(e) = unsafe { checked(CertFreeCertificateContext(Some(cert))) } {
                warn!("Can't free certificate context: {}", e)
            }
        }
    }
}

macro_rules! crypt_handle {
    ($name:ident, $destroy:path, $desc:literal) => {
        struct $name(usize);

        impl Drop for $name {
            fn drop(&mut self) {
                if self.0 == 0 {
                    return;
                }
                if let Err(e) = unsafe { $destroy(self.0) } {
                    warn!("Can't destroy {}: {}", $desc, e)
                }
            }
        }
    };
}

crypt_handle!(UserKey, CryptDestroyKey, "user key");
crypt_handle!(Hash, CryptDestroyHash, "hash");
crypt_handle!(Key, CryptDestroyKey, "key");

/// If res is FALSE this function will return last error by calling GetLastError. It only makes sense
/// if the call to checked is right after function call returning BOOL so last error is not lost.
pub fn checked(res: BOOL) -> Result<()> {
    if res == TRUE {
        Ok(())
    } else {
        Err(Error::from_thread().into())
    }
}
