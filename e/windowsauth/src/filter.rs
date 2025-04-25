use std::ptr;

use log::{debug, error, info};
use windows::{
    core::*, Win32::Foundation::*, Win32::Security::Authentication::Identity::*,
    Win32::System::Com::*, Win32::UI::Shell::*,
};

use crate::crypto::{CryptContext, LicenseType};
use crate::CLSID;

#[implement(ICredentialProviderFilter)]
pub struct Filter {}

#[allow(non_snake_case)]
impl ICredentialProviderFilter_Impl for Filter_Impl {
    fn Filter(
        &self,
        _cpus: CREDENTIAL_PROVIDER_USAGE_SCENARIO,
        _dwflags: u32,
        _rgclsidproviders: *const GUID,
        _rgballow: *mut BOOL,
        _cproviders: u32,
    ) -> Result<()> {
        Ok(())
    }

    /// UpdateRemoteCredential is used to redirect login information (extracted PIN) and login process
    /// itself to our credential provider.
    /// Windows promises that all pointers below are valid.
    #[allow(clippy::not_unsafe_ptr_arg_deref)]
    fn UpdateRemoteCredential(
        &self,
        pcpcsin: *const CREDENTIAL_PROVIDER_CREDENTIAL_SERIALIZATION,
        pcpcsout: *mut CREDENTIAL_PROVIDER_CREDENTIAL_SERIALIZATION,
    ) -> Result<()> {
        if pcpcsin.is_null() || pcpcsout.is_null() {
            error!("Pointer is invalid");
            return Err(E_INVALIDARG.into());
        }
        unsafe {
            let cpcsin = *pcpcsin;
            if cpcsin.cbSerialization == 0 {
                debug!("No serialization, skipping redirection to Teleport Authentication Package");
                *pcpcsout = *pcpcsin;
                return Err(E_FAIL.into());
            }
            let logon: KERB_CERTIFICATE_LOGON =
                *cpcsin.rgbSerialization.cast::<KERB_CERTIFICATE_LOGON>();
            if logon.MessageType != KerbCertificateLogon {
                debug!("Unknown message type {}, skipping redirection to Teleport Authentication Package", logon.MessageType.0);
                *pcpcsout = *pcpcsin;
                return Err(E_FAIL.into());
            }
            // If we can tell that this certificate was issued for an AD login,
            // then we let Windows handle the login instead of attempting to
            // process it.
            let mut ctx = CryptContext::new().map_err(|_| E_FAIL)?;
            if ctx.ad_desktop().map_err(|e| {
                error!("Could not determine login type: {:?}", e);
                E_FAIL
            })? {
                debug!("AD request, skipping redirection to Teleport Authentication Package");
                *pcpcsout = *pcpcsin;
                return Err(E_FAIL.into());
            }
            let license = ctx.get_license().map_err(|e| {
                error!("Could not determine license type: {:?}", e);
                E_FAIL
            })?;
            if license == LicenseType::Unknown {
                debug!("Unknown license, skipping redirection to Teleport Authentication Package");
                *pcpcsout = *pcpcsin;
                return Err(E_FAIL.into());
            }

            // logon.Pin.Buffer is marked as pointer, but it really is offset from the start of the
            // cpcsin.rgbSerialization structure, we have to calculate real pointer by hand
            let pin: *const u8 =
                ptr::addr_of!(*cpcsin.rgbSerialization).add(logon.Pin.Buffer.as_ptr() as _);
            let pin = PWSTR::from_raw(pin as _).to_string()?;

            debug!("PIN {:?} found", pin);
            let ptr = CoTaskMemAlloc(pin.len()) as *mut u8;
            if ptr.is_null() {
                *pcpcsout = *pcpcsin;
                return Err(E_OUTOFMEMORY.into());
            }
            let pin = pin.as_bytes();
            ptr::copy(pin.as_ptr(), ptr, pin.len());

            *pcpcsout = CREDENTIAL_PROVIDER_CREDENTIAL_SERIALIZATION {
                ulAuthenticationPackage: 0,
                rgbSerialization: ptr,
                cbSerialization: pin.len() as u32,
                clsidCredentialProvider: CLSID,
            };
        }
        info!("Found smart card login data, redirecting to Teleport Authentication Package");
        Ok(())
    }
}
