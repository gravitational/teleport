use std::cell::Cell;
use std::mem::size_of;
use std::slice;

use log::{debug, error};
use windows::{core::*, Win32::Foundation::*, Win32::System::Com::*, Win32::UI::Shell::*};

use crate::credential::Credential;

#[implement(ICredentialProvider)]
pub struct Provider {
    pin: Cell<Option<[u8; 8]>>,
}

impl Provider {
    pub fn new() -> Self {
        Provider {
            pin: Cell::new(None),
        }
    }
}

impl Default for Provider {
    fn default() -> Self {
        Self::new()
    }
}

#[allow(non_snake_case)]
impl ICredentialProvider_Impl for Provider_Impl {
    fn SetUsageScenario(
        &self,
        cpus: CREDENTIAL_PROVIDER_USAGE_SCENARIO,
        _dwflags: u32,
    ) -> Result<()> {
        debug!("Provider::SetUsageScenario");
        match cpus {
            CPUS_LOGON => Ok(()),
            _ => Err(E_NOTIMPL.into()),
        }
    }

    // Windows promises that all pointers below are valid
    #[allow(clippy::not_unsafe_ptr_arg_deref)]
    fn SetSerialization(
        &self,
        pcpcs: *const CREDENTIAL_PROVIDER_CREDENTIAL_SERIALIZATION,
    ) -> Result<()> {
        debug!("Provider::SetSerialization");
        if pcpcs.is_null() {
            error!("Pointer is invalid");
            return Err(E_INVALIDARG.into());
        }
        unsafe {
            let cpcs = *pcpcs;
            let slice: &[u8] =
                slice::from_raw_parts(cpcs.rgbSerialization, cpcs.cbSerialization as usize);
            debug!("Serialization: {:?}", slice);
            if cpcs.cbSerialization != 8 {
                return Err(E_UNEXPECTED.into());
            }
            self.pin.set(Some(
                slice.try_into().map_err(|_| Error::from(E_UNEXPECTED))?,
            ));
        }
        Ok(())
    }

    fn Advise(&self, _pcpe: Ref<ICredentialProviderEvents>, _upadvisecontext: usize) -> Result<()> {
        Ok(())
    }

    fn UnAdvise(&self) -> Result<()> {
        Ok(())
    }

    fn GetFieldDescriptorCount(&self) -> Result<u32> {
        // only 1 field - logo
        Ok(1)
    }

    #[allow(clippy::not_unsafe_ptr_arg_deref)]
    fn GetFieldDescriptorAt(
        &self,
        dwindex: u32,
    ) -> Result<*mut CREDENTIAL_PROVIDER_FIELD_DESCRIPTOR> {
        debug!("Provider::GetFieldDescriptorAt {}", dwindex);
        type Descriptor = CREDENTIAL_PROVIDER_FIELD_DESCRIPTOR;
        let ptr = unsafe { CoTaskMemAlloc(size_of::<Descriptor>()).cast::<Descriptor>() };
        if ptr.is_null() {
            error!("Can't allocate field descriptor");
            return Err(E_OUTOFMEMORY.into());
        }
        unsafe {
            *ptr = CREDENTIAL_PROVIDER_FIELD_DESCRIPTOR {
                dwFieldID: dwindex,
                cpft: CPFT_TILE_IMAGE,
                guidFieldType: GUID::default(),
                pszLabel: PWSTR::null(),
            };
            debug!("Set up field descriptor {:?}", *ptr);
        }
        Ok(ptr)
    }

    #[allow(clippy::not_unsafe_ptr_arg_deref)]
    fn GetCredentialCount(
        &self,
        credentials_count: *mut u32,
        default_credential_index: *mut u32,
        auto_login: *mut BOOL,
    ) -> Result<()> {
        if credentials_count.is_null() || default_credential_index.is_null() || auto_login.is_null()
        {
            error!("Pointer is invalid");
            return Err(E_INVALIDARG.into());
        }
        unsafe {
            // we have only 1 credential and we want to login automatically with it, without
            // user clicking in Windows login UI
            *credentials_count = 1;
            *default_credential_index = 0;
            *auto_login = TRUE;
        };
        Ok(())
    }

    fn GetCredentialAt(&self, _dwindex: u32) -> Result<ICredentialProviderCredential> {
        debug!("Provider::GetCredentialAt");
        match self.pin.get() {
            Some(pin) => Ok(Credential { pin }.into()),
            None => Err(E_INVALIDARG.into()),
        }
    }
}
