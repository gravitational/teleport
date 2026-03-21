use std::sync::atomic::{AtomicI64, Ordering};
use std::{ffi, mem, ptr};

use ::log::debug;
use windows::{core::*, Win32::Foundation::*, Win32::System::Com::*, Win32::UI::Shell::*};

pub use credential_provider::*;

use crate::filter::Filter;

pub mod auth;
pub mod credential;
pub mod credential_provider;
pub mod crypto;
pub mod filter;
mod log;
mod utf16;

static DLL_REF_COUNT: AtomicI64 = AtomicI64::new(0);

/// CLSID is GUID identifying this Credential Provider during COM registration
/// See https://github.com/gravitational/teleport.e/blob/rfd/0002-non-ad-desktop-access/rfd/0002-non-ad-desktop-access.md#credential-provider
///
/// Note: this must be kept in sync with the same value in installer/main.go
pub const CLSID: GUID = GUID::from_u128(0xFF285315_5335_4F69_A9A2_9CC5F8419D55);

#[no_mangle]
unsafe extern "system" fn DllGetClassObject(
    rclsid: *const GUID,
    riid: *const GUID,
    ppv: *mut *mut ffi::c_void,
) -> HRESULT {
    if log::initialize("C:\\Windows\\Logs\\teleport.cp.txt").is_err() {
        return E_FAIL;
    }

    debug!("DllGetClassObject");

    // Validate arguments
    if ppv.is_null() {
        return E_POINTER;
    }
    *ppv = ptr::null_mut();
    if rclsid.is_null() || riid.is_null() {
        return E_INVALIDARG;
    }

    let rclsid = *rclsid;
    let riid = *riid;

    if rclsid != CLSID || riid != IClassFactory::IID {
        return CLASS_E_CLASSNOTAVAILABLE;
    }

    // Construct the factory object and return its `IClassFactory` interface
    let factory: IClassFactory = ProviderFactory.into();
    *ppv = mem::transmute::<windows::Win32::System::Com::IClassFactory, *mut std::ffi::c_void>(
        factory,
    );
    S_OK
}

#[no_mangle]
extern "system" fn DllAddRef() {
    DLL_REF_COUNT.fetch_add(1, Ordering::AcqRel);
}

#[no_mangle]
extern "system" fn DllRelease() {
    DLL_REF_COUNT.fetch_add(-1, Ordering::AcqRel);
}

#[no_mangle]
extern "system" fn DllCanUnloadNow() -> HRESULT {
    let ref_count = DLL_REF_COUNT.load(Ordering::Acquire);
    debug!("DllCanUnloadNow, ref count: {}", ref_count);
    if ref_count > 0 {
        S_FALSE
    } else {
        S_OK
    }
}

#[implement(IClassFactory)]
struct ProviderFactory;

#[allow(non_snake_case)]
impl IClassFactory_Impl for ProviderFactory_Impl {
    fn CreateInstance(
        &self,
        punkouter: Ref<IUnknown>,
        riid: *const GUID,
        ppvobject: *mut *mut ffi::c_void,
    ) -> Result<()> {
        if ppvobject.is_null() {
            return Err(E_POINTER.into());
        }
        unsafe { *ppvobject = ptr::null_mut() };
        if riid.is_null() {
            return Err(E_INVALIDARG.into());
        }
        let riid = unsafe { *riid };
        if punkouter.is_some() {
            return Err(CLASS_E_NOAGGREGATION.into());
        }

        if riid == ICredentialProvider::IID {
            let provider: ICredentialProvider = Provider::new().into();
            unsafe {
                *ppvobject = mem::transmute::<
                    windows::Win32::UI::Shell::ICredentialProvider,
                    *mut std::ffi::c_void,
                >(provider)
            };
            Ok(())
        } else if riid == ICredentialProviderFilter::IID {
            let filter: ICredentialProviderFilter = Filter {}.into();
            unsafe {
                *ppvobject = mem::transmute::<
                    windows::Win32::UI::Shell::ICredentialProviderFilter,
                    *mut std::ffi::c_void,
                >(filter)
            };
            Ok(())
        } else {
            Err(E_NOINTERFACE.into())
        }
    }

    fn LockServer(&self, _flock: BOOL) -> Result<()> {
        Ok(())
    }
}
