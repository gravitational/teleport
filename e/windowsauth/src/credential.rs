use std::ffi::c_void;
use std::ptr;

use anyhow::Context;
use include_flate::flate;
use log::{debug, error, info};
use windows::{
    core::*, Win32::Foundation::*, Win32::Graphics::Gdi::*,
    Win32::Security::Authentication::Identity::*, Win32::System::Com::*, Win32::UI::Shell::*,
};

use crate::CLSID;

const BITMAP_HEADER_SIZE: usize = std::mem::size_of::<BITMAPFILEHEADER>();
const BITMAP_INFO_SIZE: usize = std::mem::size_of::<BITMAPINFO>();

// Tile icon showed during login, file must be valid 24-bit bitmap
flate!(static TILE_IMAGE: [u8] from "icon.bmp");

#[implement(ICredentialProviderCredential)]
pub struct Credential {
    pub pin: [u8; 8],
}

#[allow(non_snake_case)]
impl ICredentialProviderCredential_Impl for Credential_Impl {
    fn Advise(&self, _pcpce: Ref<ICredentialProviderCredentialEvents>) -> Result<()> {
        Ok(())
    }

    fn UnAdvise(&self) -> Result<()> {
        Ok(())
    }

    fn SetSelected(&self) -> Result<BOOL> {
        debug!("Credential::SetSelected");
        Ok(TRUE)
    }

    fn SetDeselected(&self) -> Result<()> {
        debug!("Credential::SetDeselected");
        Ok(())
    }

    // Windows promises that all pointers below are valid
    #[allow(clippy::not_unsafe_ptr_arg_deref)]
    fn GetFieldState(
        &self,
        _dwfieldid: u32,
        pcpfs: *mut CREDENTIAL_PROVIDER_FIELD_STATE,
        pcpfis: *mut CREDENTIAL_PROVIDER_FIELD_INTERACTIVE_STATE,
    ) -> Result<()> {
        debug!("Credential::GetFieldState");
        if pcpfis.is_null() || pcpfs.is_null() {
            error!("Pointer is invalid");
            return Err(E_INVALIDARG.into());
        }
        unsafe {
            *pcpfs = CPFS_DISPLAY_IN_BOTH;
            *pcpfis = CPFIS_NONE;
        }
        Ok(())
    }

    fn GetStringValue(&self, _dwfieldid: u32) -> Result<PWSTR> {
        debug!("Credential::GetStringValue");
        Err(E_NOTIMPL.into())
    }

    fn GetBitmapValue(&self, _dwfieldid: u32) -> Result<HBITMAP> {
        debug!("Credential::GetBitmapValue");
        match unsafe { get_tile_image() } {
            Err(e) => {
                error!("Can't load bitmap data: {:#}", e);
                Err(E_FAIL.into())
            }
            Ok(bitmap) => Ok(bitmap),
        }
    }

    fn GetCheckboxValue(
        &self,
        _dwfieldid: u32,
        _pbchecked: *mut BOOL,
        _ppszlabel: *mut PWSTR,
    ) -> Result<()> {
        debug!("Credential::GetCheckboxValue");
        Err(E_NOTIMPL.into())
    }

    fn GetSubmitButtonValue(&self, _dwfieldid: u32) -> Result<u32> {
        debug!("Credential::GetSubmitButtonValue");
        Err(E_NOTIMPL.into())
    }

    fn GetComboBoxValueCount(
        &self,
        _dwfieldid: u32,
        _pcitems: *mut u32,
        _pdwselecteditem: *mut u32,
    ) -> Result<()> {
        debug!("Credential::GetComboBoxValueCount");
        Err(E_NOTIMPL.into())
    }

    fn GetComboBoxValueAt(&self, _dwfieldid: u32, _dwitem: u32) -> Result<PWSTR> {
        debug!("Credential::GetComboBoxValueAt");
        Err(E_NOTIMPL.into())
    }

    fn SetStringValue(&self, _dwfieldid: u32, _psz: &PCWSTR) -> Result<()> {
        Err(E_NOTIMPL.into())
    }

    fn SetCheckboxValue(&self, _dwfieldid: u32, _bchecked: BOOL) -> Result<()> {
        Err(E_NOTIMPL.into())
    }

    fn SetComboBoxSelectedValue(&self, _dwfieldid: u32, _dwselecteditem: u32) -> Result<()> {
        Err(E_NOTIMPL.into())
    }

    fn CommandLinkClicked(&self, _dwfieldid: u32) -> Result<()> {
        Err(E_NOTIMPL.into())
    }

    /// This is main link between credential provider and authentication package.
    /// We redirect login to our authentication package (with ID obtained with get_auth_package_id)
    /// and send pin to the smart card received from Teleport side.
    ///
    /// Windows promises that all pointers below are valid.
    #[allow(clippy::not_unsafe_ptr_arg_deref)]
    fn GetSerialization(
        &self,
        pcpgsr: *mut CREDENTIAL_PROVIDER_GET_SERIALIZATION_RESPONSE,
        pcpcs: *mut CREDENTIAL_PROVIDER_CREDENTIAL_SERIALIZATION,
        ppszoptionalstatustext: *mut PWSTR,
        pcpsioptionalstatusicon: *mut CREDENTIAL_PROVIDER_STATUS_ICON,
    ) -> Result<()> {
        debug!("Credential::GetSerialization {:?}", self.pin);
        let pin_length = self.pin.len();
        if pcpgsr.is_null()
            || pcpcs.is_null()
            || ppszoptionalstatustext.is_null()
            || pcpsioptionalstatusicon.is_null()
        {
            error!("Pointer is invalid");
            return Err(E_INVALIDARG.into());
        }
        unsafe {
            *pcpgsr = CPGSR_RETURN_CREDENTIAL_FINISHED;
            *ppszoptionalstatustext = PWSTR::null();
            *pcpsioptionalstatusicon = CPSI_NONE;
            *pcpcs = CREDENTIAL_PROVIDER_CREDENTIAL_SERIALIZATION {
                ulAuthenticationPackage: get_auth_package_id().map_err(|e| {
                    error!("Can't get authentication package ID: {:#}", e);
                    E_FAIL
                })?,
                clsidCredentialProvider: CLSID,
                cbSerialization: pin_length as u32,
                rgbSerialization: CoTaskMemAlloc(pin_length).cast::<u8>(),
            };
            ptr::copy(self.pin.as_ptr(), (*pcpcs).rgbSerialization, pin_length);
        }
        Ok(())
    }

    /// ReportResult converts an error code to a human-readable
    /// error message that is displayed to the user at the login screen.
    ///
    /// See: https://learn.microsoft.com/en-us/windows/win32/api/credentialprovider/nf-credentialprovider-icredentialprovidercredential-reportresult
    ///
    /// Windows promises that all pointers below are valid.
    #[allow(clippy::not_unsafe_ptr_arg_deref)]
    fn ReportResult(
        &self,
        ntsstatus: NTSTATUS,
        _ntssubstatus: NTSTATUS,
        ppszoptionalstatustext: *mut PWSTR,
        pcpsioptionalstatusicon: *mut CREDENTIAL_PROVIDER_STATUS_ICON,
    ) -> Result<()> {
        if ppszoptionalstatustext.is_null() || pcpsioptionalstatusicon.is_null() {
            error!("Pointer is invalid");
            return Err(E_INVALIDARG.into());
        }
        // conversion from NTSTATUS to Error sets additional bit, we have to clear it to match
        // defined constants
        let ntsstatus = NTSTATUS(ntsstatus.0 & (!0x1000_0000));
        unsafe {
            match ntsstatus {
                STATUS_CTX_CLIENT_LICENSE_NOT_SET => {
                    *pcpsioptionalstatusicon = CPSI_ERROR;
                    *ppszoptionalstatustext = SHStrDupW(w!("Unknown license"))?;
                }
                STATUS_IMPLEMENTATION_LIMIT => {
                    *pcpsioptionalstatusicon = CPSI_ERROR;
                    *ppszoptionalstatustext =
                        SHStrDupW(w!("Desktops limit exceeded for Teleport Community"))?;
                }
                s => {
                    info!("Status: {:?}", s)
                }
            }
        }
        Ok(())
    }
}

fn get_auth_package_id() -> anyhow::Result<u32> {
    let mut hlsa = HANDLE::default();
    let mut auth_package = 0u32;
    let mut lsa_string = LSA_STRING::default();
    lsa_string.Buffer = PSTR::from_raw(s!("Teleport").as_ptr() as _);
    lsa_string.Length = "Teleport".len() as u16;
    lsa_string.MaximumLength = lsa_string.Length;
    unsafe {
        LsaConnectUntrusted(&mut hlsa)
            .ok()
            .context("Can't connect to LSA")?;
        LsaLookupAuthenticationPackage(hlsa, &lsa_string, &mut auth_package)
            .ok()
            .context("Can't lookup authentication package")?;
        LsaDeregisterLogonProcess(hlsa)
            .ok()
            .context("Can't deregister logon process")?;
    }
    debug!("Package id: {}", auth_package);
    Ok(auth_package)
}

/// This function creates HBITMAP from BMP file included in TILE_IMAGE.
///
/// Safety: data in TILE_IMAGE must be valid bitmap (max 24bit)!
unsafe fn get_tile_image() -> anyhow::Result<HBITMAP> {
    // we have to have at least header and bitmap info
    if TILE_IMAGE.len() < BITMAP_HEADER_SIZE + BITMAP_INFO_SIZE {
        return Err(Error::from(E_INVALIDARG)).context("Tile image doesn't contain bitmap header");
    }
    let bmpfh = *(TILE_IMAGE.as_ptr() as *const BITMAPFILEHEADER);
    //bitmap info structure is right after header
    let bmpi = *(TILE_IMAGE[BITMAP_HEADER_SIZE..].as_ptr() as *const BITMAPINFO);
    let image_data = &TILE_IMAGE[bmpfh.bfOffBits as usize..] as *const _ as *const c_void;
    let dib_section = CreateDIBSection(None, &bmpi, DIB_RGB_COLORS, ptr::null_mut(), None, 0)?;
    let lines = SetDIBits(
        None,
        dib_section,
        0,
        bmpi.bmiHeader.biHeight as u32,
        image_data,
        &bmpi,
        DIB_RGB_COLORS,
    );
    if lines > 0 {
        Ok(dib_section)
    } else {
        Err(Error::from_thread()).context("No lines written to bitmap")
    }
}
