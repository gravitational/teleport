use std::collections::HashSet;
use std::ffi::c_void;
use std::str::FromStr;
use std::sync::atomic::{AtomicPtr, Ordering};
use std::sync::Mutex;
use std::{mem, ptr, slice};

use anyhow::{anyhow, ensure, Context, Result};
use itertools::Itertools;
use log::{debug, error, info, warn};
use rand::distributions::{Alphanumeric, DistString};
use rand::rngs::OsRng;
use windows::{
    core::*, Win32::Foundation::*, Win32::NetworkManagement::NetManagement::*,
    Win32::Security::Authentication::Identity::*, Win32::Security::Authorization::*,
    Win32::Security::Credentials::*, Win32::Security::*, Win32::System::Kernel::*,
    Win32::System::SystemServices::*, Win32::System::WindowsProgramming::*, *,
};

use crate::crypto::LicenseType;
use crate::crypto::{CryptContext, UserCreation};
use crate::utf16::UTF16;

static DISPATCH_TABLE: AtomicPtr<LSA_SECPKG_FUNCTION_TABLE> = AtomicPtr::new(ptr::null_mut());

static GROUPS_LOCK: Mutex<()> = Mutex::new(());

fn dispatch_table() -> LSA_SECPKG_FUNCTION_TABLE {
    unsafe { *DISPATCH_TABLE.load(Ordering::SeqCst) }
}

/// allocate_lsa_heap_size allocates memory using [`AllocateLsaHeap`] function.
/// It's used in most places where we need to return data to LSA. If used in such context
/// the LSA will be responsible for freeing memory, and we must not use FreeLsaHeap.
///
/// [`AllocateLsaHeap`]: https://learn.microsoft.com/en-us/windows/win32/api/ntsecpkg/nc-ntsecpkg-lsa_allocate_lsa_heap
fn allocate_lsa_heap_size<T>(size: u32) -> Result<*mut T> {
    let data = unsafe {
        dispatch_table()
            .AllocateLsaHeap
            .context("AllocateLsaHeap missing")?(size)
    };
    if data.is_null() {
        Err(core::Error::from(E_OUTOFMEMORY)).context("Can't allocate LSA heap")
    } else {
        Ok(data as _)
    }
}

/// Convenience method for calling [allocate_lsa_heap_size] with size calculated by the compiler.
/// Size of T must be smaller than 2^32 bytes
fn allocate_lsa_heap<T>() -> Result<*mut T> {
    allocate_lsa_heap_size(mem::size_of::<T>().try_into()?)
}

/// allocate_private_heap allocates memory using AllocatePrivateHeap function. It's used for groups
/// data stored in the user token.
///
/// See: https://learn.microsoft.com/en-us/windows/win32/api/ntsecpkg/nc-ntsecpkg-lsa_allocate_private_heap
fn allocate_private_heap<T>(size: usize) -> Result<*mut T> {
    let data = unsafe {
        dispatch_table()
            .AllocatePrivateHeap
            .context("AllocatePrivateHeap missing")?(size)
    };
    if data.is_null() {
        Err(core::Error::from(E_OUTOFMEMORY)).context("Can't allocate private heap")
    } else {
        Ok(data as _)
    }
}

unsafe fn create_logon_session(luid: *mut LUID) -> Result<()> {
    map_nt_status(dispatch_table()
        .CreateLogonSession
        .context("CreateLogonSession missing")?(luid))
}

unsafe fn impersonate_client() -> Result<Impersonation> {
    map_nt_status(dispatch_table()
        .ImpersonateClient
        .context("ImpersonateClient missing")?())
    .map(|_| Impersonation {})
}

unsafe fn allocate_client_buffer(
    client_request: *const *const c_void,
    size: u32,
    target: *mut *mut c_void,
) -> Result<()> {
    let allocate = dispatch_table()
        .AllocateClientBuffer
        .context("AllocateClientBuffer missing")?;
    map_nt_status(allocate(client_request, size, target))
}

unsafe fn copy_to_client_buffer(
    client_request: *const *const c_void,
    data: Vec<u8>,
    target: *mut c_void,
) -> Result<()> {
    let copy = dispatch_table()
        .CopyToClientBuffer
        .context("CopyToClientBuffer missing")?;
    map_nt_status(copy(
        client_request,
        data.len() as u32,
        target,
        data.as_ptr() as _,
    ))
}

fn map_nt_status(status: NTSTATUS) -> Result<()> {
    match status {
        STATUS_SUCCESS => Ok(()),
        err => Err(core::Error::from(err).into()),
    }
}

/// LsaApInitializePackage is called once by LSA during system startup to provide package chance to initialize itself.
/// It also provides table with useful functions.
///
/// See: https://learn.microsoft.com/en-us/windows/win32/api/ntsecpkg/nc-ntsecpkg-lsa_ap_initialize_package
///
/// Windows promises that all pointers provided are valid, we do additional check to avoid null pointers.
#[no_mangle]
unsafe extern "system" fn LsaApInitializePackage(
    authentication_package_id: u32,
    dispatch_table: *mut LSA_SECPKG_FUNCTION_TABLE,
    _database: *const STRING,
    _confidentiality: *const STRING,
    authentication_package_name: *mut *mut STRING,
) -> NTSTATUS {
    if dispatch_table.is_null() || authentication_package_name.is_null() {
        return STATUS_INVALID_PARAMETER;
    }
    if crate::log::initialize("C:\\Windows\\Logs\\teleport.ap.txt").is_err() {
        return STATUS_FILE_INVALID;
    }

    debug!("Authentication package id: {}", authentication_package_id);
    let table = unsafe { *dispatch_table };
    if let Err(e) = verify_dispatch_table(&table) {
        error!("Required function missing in LSA dispatch table: {:#}", e);
        return STATUS_INVALID_PARAMETER;
    }
    DISPATCH_TABLE.store(dispatch_table, Ordering::SeqCst);
    debug!("LSA dispatch table stored");
    let ptr = allocate_lsa_heap();
    if let Err(e) = ptr {
        error!("Can't initialize authentication package: {:#}", e);
        return STATUS_NO_MEMORY;
    }
    let ptr = ptr.unwrap();
    let pcstr = s!("Teleport");
    RtlInitString(ptr, pcstr.as_ptr() as *mut i8);
    *authentication_package_name = ptr;

    STATUS_SUCCESS
}

fn verify_dispatch_table(table: &LSA_SECPKG_FUNCTION_TABLE) -> Result<()> {
    table.AllocateLsaHeap.context("AllocateLsaHeap")?;
    table.AllocatePrivateHeap.context("AllocatePrivateHeap")?;
    table.CreateLogonSession.context("CreateLogonSession")?;
    table.ImpersonateClient.context("ImpersonateClient")?;
    table.AllocateClientBuffer.context("AllocateClientBuffer")?;
    table.CopyToClientBuffer.context("CopyToClientBuffer")?;
    Ok(())
}

fn create_group(name: &str) -> Result<()> {
    let mut uname = UTF16::from(name);
    let comment = w!("This group is managed by Teleport");
    let group = GROUP_INFO_1 {
        grpi1_name: uname.pwstr(),
        grpi1_comment: PWSTR::from_raw(comment.as_ptr() as _),
    };
    let pgroup: *const GROUP_INFO_1 = &group;
    let res = unsafe { NetLocalGroupAdd(None, 1, pgroup as _, None) };
    const ERR_GROUP_EXISTS: u32 = ERROR_ALIAS_EXISTS.0;
    #[allow(non_upper_case_globals)]
    match res {
        NERR_Success => Ok(()),
        NERR_GroupExists => Ok(()),
        ERR_GROUP_EXISTS => Ok(()),
        e => Err(Error::from(WIN32_ERROR(e))).context(format!("Can't create group {}", name)),
    }
}

const TELEPORT_USERS_GROUP: &str = "Teleport Users";

fn ensure_user(name: &str) -> Result<()> {
    let group = TELEPORT_USERS_GROUP.to_string();
    create_group(&group)?;
    let mut uname = UTF16::from(name);
    let comment = w!("This user is managed by Teleport");
    let user = USER_INFO_1 {
        usri1_name: uname.pwstr(),
        usri1_priv: USER_PRIV_USER,
        usri1_comment: PWSTR::from_raw(comment.as_ptr() as _),
        usri1_flags: UF_SCRIPT
            | UF_ACCOUNTDISABLE
            | UF_PASSWD_NOTREQD
            | UF_PASSWD_CANT_CHANGE
            | UF_DONT_EXPIRE_PASSWD
            | UF_SMARTCARD_REQUIRED,
        ..Default::default()
    };
    let puser: *const USER_INFO_1 = &user;
    let res = unsafe { NetUserAdd(None, 1, puser as _, None) };
    #[allow(non_upper_case_globals)]
    match res {
        NERR_Success => {
            // User created, let's add it to Teleport Users group
            let user = lookup_account(name, &[SidTypeUser])?;
            let ugroup = UTF16::from(&group);
            let members_info = &LOCALGROUP_MEMBERS_INFO_0 {
                lgrmi0_sid: user.psid(),
            };
            let pmi: *const LOCALGROUP_MEMBERS_INFO_0 = members_info;
            match unsafe { NetLocalGroupAddMembers(None, ugroup.pcwstr(), 0, pmi as _, 1) } {
                NERR_Success => Ok(()),
                e => Err(Error::from(WIN32_ERROR(e)))
                    .context(format!("Can't add user {} to group {}", name, group)),
            }
        }
        // User already exists, nothing to do here
        NERR_UserExists => Ok(()),
        e => Err(Error::from(WIN32_ERROR(e))).context(format!("Can't create user {}", name)),
    }
}

/// LsaAPLogonUserEx2 is the main function responsible for validating a user's login and creating a user token.
///
/// See: https://learn.microsoft.com/en-us/windows/win32/api/ntsecpkg/nc-ntsecpkg-lsa_ap_logon_user_ex2
///
/// Windows promises that all pointers provided are valid, we do additional check to avoid null pointers.
#[no_mangle]
unsafe extern "system" fn LsaApLogonUserEx2(
    client_request: *const *const c_void,
    logon_type: SECURITY_LOGON_TYPE,
    authentication_information: *const u8,
    _client_authentication_base: *const c_void,
    authentication_information_length: u32,
    profile_buffer: *mut *mut c_void,
    profile_buffer_length: *mut u32,
    logon_id: *mut LUID,
    _substatus: *mut i32,
    token_information_type: *mut LSA_TOKEN_INFORMATION_TYPE,
    token_information: *mut *mut LSA_TOKEN_INFORMATION_V1,
    account_name: *mut *mut UNICODE_STRING,
    authenticating_authority: *mut *mut UNICODE_STRING,
    _machine_name: *mut *mut UNICODE_STRING,
    primary_credential: *mut SECPKG_PRIMARY_CRED,
    _supplemental_credentials: *mut *mut SECPKG_SUPPLEMENTAL_CRED_ARRAY,
) -> NTSTATUS {
    debug!("LsaApLogonUserEx2");
    if logon_type != SECURITY_LOGON_TYPE::RemoteInteractive {
        debug!("Invalid logon type {:?}", logon_type);
        return STATUS_INVALID_LOGON_TYPE;
    }
    match lsa_ap_logon_user(
        client_request,
        authentication_information,
        authentication_information_length,
        profile_buffer,
        profile_buffer_length,
        logon_id,
        token_information_type,
        token_information,
        account_name,
        authenticating_authority,
        primary_credential,
    ) {
        Ok(_) => STATUS_SUCCESS,
        Err(e) => {
            let status = match e.downcast_ref::<Error>() {
                Some(e) => NTSTATUS(e.code().0),
                None => STATUS_LOGON_FAILURE,
            };
            error!("Logon failed: {:#}", e);
            status
        }
    }
}

#[allow(clippy::too_many_arguments)]
unsafe fn lsa_ap_logon_user(
    client_request: *const *const c_void,
    authentication_information: *const u8,
    authentication_information_length: u32,
    profile_buffer: *mut *mut c_void,
    profile_buffer_length: *mut u32,
    logon_id: *mut LUID,
    token_information_type: *mut LSA_TOKEN_INFORMATION_TYPE,
    token_information: *mut *mut LSA_TOKEN_INFORMATION_V1,
    account_name: *mut *mut UNICODE_STRING,
    authenticating_authority: *mut *mut UNICODE_STRING,
    primary_credential: *mut SECPKG_PRIMARY_CRED,
) -> Result<()> {
    // Windows promises all pointers here are valid, we do additional check to avoid null pointers
    if authentication_information.is_null()
        || profile_buffer.is_null()
        || profile_buffer_length.is_null()
        || logon_id.is_null()
        || token_information.is_null()
        || token_information_type.is_null()
        || account_name.is_null()
        || authenticating_authority.is_null()
    {
        return Err(Error::from(STATUS_INVALID_PARAMETER)).context("Pointer is invalid");
    }
    let pin = slice::from_raw_parts(
        authentication_information,
        authentication_information_length as usize,
    );
    let (name, should_create_user) = process_certificate(pin)?;

    *account_name = lsa_string(&name).context("Can't create account name")?;

    if let UserCreation::Yes(_) = should_create_user {
        ensure_user(&name)?;
    }

    fill_profile_buffer(client_request, profile_buffer, profile_buffer_length)
        .context("Can't fill profile buffer")?;

    AllocateLocallyUniqueId(logon_id).context("Can't allocate logon id")?;
    create_logon_session(logon_id).context("Can't create logon session")?;

    let user = lookup_account(&name, &[SidTypeUser])?;
    *authenticating_authority =
        lsa_string(&user.domain).context("Can't create authenticating authority")?;

    // If we get here, all checks have passed and the user should
    // be able to log in. Generate a token with information about
    // the user, their groups, etc.
    let token = allocate_lsa_heap::<LSA_TOKEN_INFORMATION_V1>().context("Can't allocate token")?;
    ptr::write(
        token,
        LSA_TOKEN_INFORMATION_V1 {
            ExpirationTime: i64::MAX,
            ..Default::default()
        },
    );
    *token_information = token;
    *token_information_type = LsaTokenInformationV1;
    let token = &mut *token;

    copy_user_to_token(token, &user).context("Can't copy user to token")?;
    copy_primary_group_to_token(&name, token, &user)
        .context("Can't copy primary group to token")?;

    // take lock before syncing groups so login token returned will have consistent view of
    // group membership
    let _lock = GROUPS_LOCK.lock().unwrap();
    let (groups, managed_by) = sync_groups(&name, should_create_user, &user)?;
    copy_groups_to_token(token, groups).context("Can't copy groups to token")?;

    if managed_by == ManagedBy::Teleport {
        // We have to have password equivalent that is unique and secret for each user and is consistent
        // between logins. We retrieve it from LSA secrets and if it's missing we generate new random one
        // and store it there.
        // Consistent password is required for Credential Manager to work.
        // https://learn.microsoft.com/en-us/windows/win32/api/ntsecapi/nf-ntsecapi-lsaretrieveprivatedata
        // https://learn.microsoft.com/en-us/windows/win32/api/ntsecapi/nf-ntsecapi-lsastoreprivatedata
        // https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-lsad/483f1b6e-7b14-4341-9ab2-9b99c01f896e
        let mut policy = LsaPolicy::new()?;
        // L$ means that it's local secret, available only on this machine
        let private_data_key = &format!("L$TELEPORT_{}", name);
        let data = policy.retrieve_private_data(private_data_key)?;
        let key = if let Some(key) = data {
            key
        } else {
            let new_key: String = Alphanumeric.sample_string(&mut OsRng, 50);
            policy.store_private_data(private_data_key, &new_key)?;
            new_key
        };

        (*primary_credential).LogonId = *logon_id;
        (*primary_credential).Flags = PRIMARY_CRED_CLEAR_PASSWORD;
        (*primary_credential).DownlevelName = to_lsa_unicode_string(&name)?;
        (*primary_credential).Password = to_lsa_unicode_string(&key)?;
        (*primary_credential).UserSid = PSID(allocate_lsa_heap_size(user.sid_length()?)?);
        CopySid(
            user.sid.len() as _,
            (*primary_credential).UserSid,
            user.psid(),
        )
        .context("Can't copy user SID")?;
    }

    info!("User {} logged in successfully", name);
    Ok(())
}

unsafe fn to_lsa_unicode_string(s: &str) -> Result<LSA_UNICODE_STRING> {
    let s = *lsa_string(s)?;
    Ok(LSA_UNICODE_STRING {
        Length: s.Length,
        MaximumLength: s.MaximumLength,
        Buffer: s.Buffer,
    })
}

unsafe fn process_certificate(pin: &[u8]) -> Result<(String, UserCreation)> {
    // we must use impersonate client in order to have permission to access smart cards,
    // we'll revert to LSA context when _impersonation is dropped
    let _impersonation = impersonate_client()
        .context("Can't impersonate client which is required for certificate verification")?;
    let mut ctx = CryptContext::new()?;

    let name = ctx
        .username_from_certificate()
        .context("Can't get user name")?;

    ctx.validate()?;
    match ctx.get_license().context("Can't check license")? {
        LicenseType::Enterprise => {}
        LicenseType::OSS => {
            if ctx.desktops_limit_exceeded()? {
                return Err(Error::from(STATUS_IMPLEMENTATION_LIMIT))
                    .context("Desktops limit exceeded");
            }
        }
        _ => return Err(Error::from(STATUS_CTX_CLIENT_LICENSE_NOT_SET)).context("Unknown license"),
    };

    ctx.sign_and_verify(pin.as_ptr())
        .context("Can't verify certificate")?;

    let should_create_user = ctx
        .should_create_user()
        .context("Can't check if certificate allows user creation")?;

    Ok((name, should_create_user))
}

unsafe fn copy_primary_group_to_token(
    name: &str,
    token: &mut LSA_TOKEN_INFORMATION_V1,
    user: &Account,
) -> Result<()> {
    let primary_group =
        lookup_primary_group(name, &user.domain).context("Can't lookup primary group")?;
    let sid_length = primary_group.sid_length()?;
    token.PrimaryGroup.PrimaryGroup = PSID(allocate_lsa_heap_size(sid_length)?);
    CopySid(
        sid_length,
        token.PrimaryGroup.PrimaryGroup,
        primary_group.psid(),
    )
    .context("Can't copy primary group SID")
}

unsafe fn copy_user_to_token(token: &mut LSA_TOKEN_INFORMATION_V1, user: &Account) -> Result<()> {
    token.User.User.Sid = PSID(allocate_lsa_heap_size(user.sid_length()?)?);
    CopySid(user.sid.len() as _, token.User.User.Sid, user.psid()).context("Can't copy user SID")
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
enum ManagedBy {
    Windows,
    Teleport,
}

/// sync_groups will return groups that should be included in the token returned to LSA.
/// If user is managed by Teleport it will also add user to requested groups and remove it from
/// all other groups.
fn sync_groups(
    name: &str,
    should_create_user: UserCreation,
    user: &Account,
) -> Result<(HashSet<String>, ManagedBy)> {
    let mut groups = lookup_groups(name)?;

    let mut managed_by = ManagedBy::Windows;

    // auto user creation was requested and user is managed by Teleport, return groups requested in the certificate
    if let UserCreation::Yes(requested_groups) = should_create_user {
        if groups.contains(TELEPORT_USERS_GROUP) {
            debug!(
                "User managed by Teleport, creating requested groups: {}",
                requested_groups.iter().format(", "),
            );
            managed_by = ManagedBy::Teleport;
            for group in requested_groups.difference(&groups) {
                create_group(group)?;
                let ugroup = UTF16::from(group);
                let members_info = LOCALGROUP_MEMBERS_INFO_0 { lgrmi0_sid: user.psid() };
                match unsafe {
                    NetLocalGroupAddMembers(None, ugroup.pcwstr(), 0, &members_info as *const LOCALGROUP_MEMBERS_INFO_0 as _, 1)
                } {
                    NERR_Success => {}
                    e => return Err(Error::from(WIN32_ERROR(e)))
                        .context(format!("Can't add user {} to group {}", name, group)),
                }
            }

            groups.remove(TELEPORT_USERS_GROUP);
            for group in groups.difference(&requested_groups) {
                let ugroup = UTF16::from(group);
                let members_info = LOCALGROUP_MEMBERS_INFO_0 { lgrmi0_sid: user.psid() };
                match unsafe {
                    NetLocalGroupDelMembers(None, ugroup.pcwstr(), 0, &members_info as *const LOCALGROUP_MEMBERS_INFO_0 as _, 1)
                } {
                    NERR_Success => {}
                    e => return Err(Error::from(WIN32_ERROR(e)))
                        .context(format!("Can't remove user {} from group {}", name, group)),
                }
            }
            groups = requested_groups;
        }
    }
    Ok((groups, managed_by))
}

/// REMOTE_DESKTOP_USERS_SID is [well-known SID] for users that can connect through RDP.
///
/// [well-known SID]: https://learn.microsoft.com/en-us/windows/win32/secauthz/well-known-sids
const REMOTE_DESKTOP_USERS_SID: &str = "S-1-5-32-555";

unsafe fn copy_groups_to_token(
    token: &mut LSA_TOKEN_INFORMATION_V1,
    groups: HashSet<String>,
) -> Result<()> {
    debug!("Groups added to token: {}", groups.iter().format(", "));

    // Space for the TOKEN_GROUPS struct, which includes space for 1 SID_AND_ATTRIBUTES.
    // We fill this 1 "free" SID_AND_ATTRIBUTES with REMOTE_DESKTOP_USERS group.
    let token_groups_and_remote_desktop_users_sid_size = mem::size_of::<TOKEN_GROUPS>();
    // Addtional space for an SID_AND_ATTRIBUTES for each group in groups
    let rest_sid_size = mem::size_of::<SID_AND_ATTRIBUTES>() * groups.len();
    // Total size of the TOKEN_GROUPS struct
    let size = token_groups_and_remote_desktop_users_sid_size + rest_sid_size;
    token.Groups = allocate_private_heap(size)?;
    ptr::write(
        token.Groups,
        TOKEN_GROUPS {
            GroupCount: groups.len() as u32 + 1,
            Groups: Default::default(),
        },
    );
    let token_groups = unsafe { (*token.Groups).Groups.as_mut_ptr() };

    // put REMOTE_DESKTOP_USERS_SID as first element in array
    let mut remote_desktop_users_sid = to_sid(REMOTE_DESKTOP_USERS_SID)?;
    copy_sid(
        token_groups,
        0,
        remote_desktop_users_sid.len() as _,
        PSID(remote_desktop_users_sid.as_mut_ptr() as _),
    )?;

    // put all requested groups' SIDs in array starting at index 1
    for (i, group) in groups.iter().enumerate() {
        let group = lookup_account(
            group,
            // group can be represented by multiple different types,
            // depending on if the group is built-in or created by user
            &[SidTypeGroup, SidTypeWellKnownGroup, SidTypeAlias],
        )
        .context(format!("Can't lookup SID for group {}", group))?;
        copy_sid(token_groups, i + 1, group.sid_length()?, group.psid())?;
    }

    Ok(())
}

/// copy_sid will put requested SID in array at specified index. Each SID is copied to private heap.
unsafe fn copy_sid(
    token_groups: *mut SID_AND_ATTRIBUTES,
    index: usize,
    sid_length: u32,
    source_sid: PSID,
) -> Result<()> {
    let psid = PSID(allocate_private_heap(sid_length as usize)?);
    ptr::write(
        token_groups.add(index),
        SID_AND_ATTRIBUTES {
            Sid: psid,
            Attributes: (SE_GROUP_MANDATORY | SE_GROUP_ENABLED | SE_GROUP_ENABLED_BY_DEFAULT)
                as u32,
        },
    );
    CopySid(sid_length, psid, source_sid)?;
    Ok(())
}

fn lookup_groups(name: &str) -> Result<HashSet<String>> {
    let mut entries_read = 0u32;
    let mut entries_total = 0u32;
    let mut data: *mut LOCALGROUP_USERS_INFO_0 = ptr::null_mut();
    let pdata: *mut *mut LOCALGROUP_USERS_INFO_0 = &mut data;
    let uname = UTF16::from(name);
    let res = unsafe {
        NetUserGetLocalGroups(
            None,
            uname.pcwstr(),
            0,
            LG_INCLUDE_INDIRECT,
            pdata as _,
            MAX_PREFERRED_LENGTH,
            &mut entries_read,
            &mut entries_total,
        )
    };
    if res != NERR_Success {
        return Err(Error::from(WIN32_ERROR(res))).context("Can't lookup groups");
    }
    let groups = unsafe { slice::from_raw_parts_mut(data, entries_read as _) };
    let groups = groups
        .iter()
        .map(|g| unsafe { g.lgrui0_name.to_string().context("Can't convert string") })
        .collect();
    unsafe { NetApiBufferFree(Some(data as _)) };
    groups
}

unsafe fn fill_profile_buffer(
    client_request: *const *const c_void,
    profile_buffer: *mut *mut c_void,
    profile_buffer_length: *mut u32,
) -> Result<()> {
    // by trial and error we determined that Windows does not accept
    // the login unless we write some data to the profile buffer
    let size = 256;
    *profile_buffer_length = size;
    allocate_client_buffer(client_request, size, profile_buffer)
        .context("Can't allocate client buffer")?;
    copy_to_client_buffer(client_request, vec![0u8; size as usize], *profile_buffer)
        .context("Can't clear client buffer")
}

#[derive(Default, Debug)]
struct Account {
    sid: Vec<u8>,
    domain: String,
}

impl Account {
    fn psid(&self) -> PSID {
        PSID(self.sid.as_ptr() as _)
    }

    fn sid_length(&self) -> Result<u32> {
        self.sid.len().try_into().context("SID too long")
    }
}

#[allow(non_upper_case_globals)]
fn is_domain_joined() -> Result<bool> {
    let mut domain = PWSTR::null();
    let mut join_status = NetSetupUnknownStatus;
    let status = unsafe { NetGetJoinInformation(None, &mut domain, &mut join_status) };
    match status {
        NERR_Success => Ok(join_status == NetSetupDomainName),
        e => Err(Error::from(WIN32_ERROR(e)).into()),
    }
}

/// lookup_primary_group obtains the primary group SID for a user using this method:
/// https://support.microsoft.com/en-us/help/297951/how-to-use-the-primarygroupid-attribute-to-find-the-primary-group-for
/// The method follows this formula: domainRID + "-" + primaryGroupRID
unsafe fn lookup_primary_group(name: &str, domain: &str) -> Result<Account> {
    let mut domain_acc = lookup_account(domain, &[SidTypeDomain])
        .context(format!("Can't lookup domain {}", domain))?;
    let mut domain_rid = to_string(domain_acc.psid())?;

    let joined = is_domain_joined().context("Can't check domain join status")?;
    if joined {
        // If the user has joined a domain use the RID of the default primary group
        // called "Domain Users":
        // https://support.microsoft.com/en-us/help/243330/well-known-security-identifiers-in-windows-operating-systems
        // SID: S-1-5-21domain-513
        domain_rid += "-513";
    } else {
        // For non-domain users call NetUserGetInfo() with level 4, which
        // in this case would not have any network overhead.
        // The primary group should not change from RID 513 here either
        // but the group will be called "None" instead:
        // https://www.adampalmer.me/iodigitalsec/2013/08/10/windows-null-session-enumeration/
        // "Group 'None' (RID: 513)"
        let uname = UTF16::from(name);
        let udomain = UTF16::from(domain);
        let mut info: *mut u8 = ptr::null_mut();
        let status = NetUserGetInfo(udomain.pcwstr(), uname.pcwstr(), 4, &mut info);
        if status != NERR_Success {
            return Err(Error::from(WIN32_ERROR(status))).context("Can't get user info");
        }
        let user_info = unsafe { *(info as *mut USER_INFO_4) };
        domain_rid = format!("{}-{}", domain_rid, user_info.usri4_primary_group_id);
        NetApiBufferFree(Some(info as _));
    }
    domain_acc.sid = to_sid(&domain_rid)?;
    Ok(domain_acc)
}

/// lookup_account gets account information for given name and checks if account is of correct type.
/// Note: Windows treats many things as an account - user, group, domain, computer etc.
/// See: https://learn.microsoft.com/en-us/windows/win32/api/winnt/ne-winnt-sid_name_use
fn lookup_account(name: &str, sid_types: &[SID_NAME_USE]) -> Result<Account> {
    let mut cb = 0u32;
    let mut cd = 0u32;
    let mut name_use = SidTypeInvalid;
    let account_name = UTF16::from(name);
    let res = unsafe {
        LookupAccountNameW(
            None,
            account_name.pcwstr(),
            None,
            &mut cb,
            None,
            &mut cd,
            &mut name_use,
        )
    };
    if let Err(err) = res {
        if err != ERROR_INSUFFICIENT_BUFFER.into() {
            Err(err).with_context(|| format!("Can't lookup account '{}' name length", name))?
        }
    }
    let mut account = Account {
        sid: vec![0u8; cb as _],
        ..Default::default()
    };
    let mut domain_buf = vec![0u16; cd as usize];
    let domain = PWSTR::from_raw(domain_buf.as_mut_ptr());
    unsafe {
        LookupAccountNameW(
            None,
            account_name.pcwstr(),
            Some(account.psid()),
            &mut cb,
            Some(domain),
            &mut cd,
            &mut name_use,
        )
        .context(format!("Can't lookup account name {}", name))?;
        account.domain = domain.to_string()?;
    }
    // in case we have user/group with the same name as computer LookupAccountNameW will return domain SID,
    // we have to try again with DOMAIN\USER format
    if name_use == SidTypeDomain && !sid_types.contains(&SidTypeDomain) {
        let name = format!("{}\\{}", account.domain, name);
        return lookup_account(&name, sid_types);
    }
    if !sid_types.contains(&name_use) {
        return Err(anyhow!("Invalid SID type: {}", name_use.0));
    }
    Ok(account)
}

unsafe fn lsa_string(name: &str) -> Result<*mut UNICODE_STRING> {
    let uname = allocate_lsa_heap::<UNICODE_STRING>()?;
    let utf: Vec<u16> = name.encode_utf16().collect();
    let buffer = allocate_lsa_heap_size((utf.len() * 2).try_into()?)?;
    ptr::copy(utf.as_ptr(), buffer, utf.len());
    let size = (utf.len() * 2) as u16;
    ptr::write(
        uname,
        UNICODE_STRING {
            Buffer: PWSTR::from_raw(buffer),
            Length: size,
            MaximumLength: size,
        },
    );
    Ok(uname)
}

// LsaApCallPackage* functions are used in non-interactive scenarios, we don't support them here, but they have to be present in every AP
#[no_mangle]
extern "system" fn LsaApCallPackage(
    _client_request: *const *const c_void,
    _protocol_submit_buffer: *const c_void,
    _client_buffer_base: *const c_void,
    _submit_buffer_length: u32,
    _protocol_return_buffer: *mut *mut c_void,
    _return_buffer_length: *mut u32,
    _protocol_status: *mut i32,
) -> NTSTATUS {
    STATUS_UNSUCCESSFUL
}

#[no_mangle]
extern "system" fn LsaApCallPackageUntrusted(
    _client_request: *const *const c_void,
    _protocol_submit_buffer: *const c_void,
    _client_buffer_base: *const c_void,
    _submit_buffer_length: u32,
    _protocol_return_buffer: *mut *mut c_void,
    _return_buffer_length: *mut u32,
    _protocol_status: *mut i32,
) -> NTSTATUS {
    STATUS_UNSUCCESSFUL
}

#[no_mangle]
extern "system" fn LsaApCallPackagePassthrough(
    _client_request: *const *const c_void,
    _protocol_submit_buffer: *const c_void,
    _client_buffer_base: *const c_void,
    _submit_buffer_length: u32,
    _protocol_return_buffer: *mut *mut c_void,
    _return_buffer_length: *mut u32,
    _protocol_status: *mut i32,
) -> NTSTATUS {
    STATUS_UNSUCCESSFUL
}

/// LsaApLogonTerminated is called when logon session started by this package ends i.e. user logs out.
///
/// See: https://learn.microsoft.com/en-us/windows/win32/api/ntsecpkg/nc-ntsecpkg-lsa_ap_logon_terminated
#[no_mangle]
extern "system" fn LsaApLogonTerminated(_logon_id: *const LUID) {}

unsafe fn to_string(psid: PSID) -> Result<String> {
    let mut s = PWSTR::null();
    ConvertSidToStringSidW(psid, &mut s).context("Can't convert SID to string")?;
    let converted = s.to_string().context("Can't convert to string");
    let _ = LocalFree(Some(HLOCAL(s.as_ptr() as _)));
    converted
}

struct Impersonation;

impl Drop for Impersonation {
    fn drop(&mut self) {
        // we're done using the smart card provider, so revert to original permissions
        if let Err(e) = unsafe { RevertToSelf() } {
            error!("Can't revert to LSA context: {}", e);
        }
    }
}

/// to_sid converts a security identifier from its
/// string representation to binary form.
/// See: https://learn.microsoft.com/en-us/windows-server/identity/ad-ds/manage/understand-security-identifiers
fn to_sid(s: &str) -> Result<Vec<u8>> {
    // parse the SID manually instead of using ConvertStringSidToSidA,
    // which sporadically fails with an invalid parameter error
    let split: Vec<&str> = s.split('-').collect();
    ensure!(
        split.len() > 2,
        "SID must be in standard string representation S-R-I-S..."
    );
    // SID standard string representation is S-<revision>-<authority>-<subauthorities>
    let revision = u8::from_str(split[1])?;
    let authority = u64::from_str(split[2])?;
    let subauthorities: Result<Vec<u32>, _> =
        split.iter().skip(3).map(|s| u32::from_str(s)).collect();
    let subauthorities = subauthorities.context(format!("Can't convert SID {}", s))?;
    let mut res = vec![0u8; 2 /*header*/ + 6 /*top authority*/ + 4 * subauthorities.len()];
    res[0] = revision;
    res[1] = subauthorities.len() as u8;
    // top authority is stored as 6 bytes big endian
    res[2..8].copy_from_slice(&authority.to_be_bytes()[2..]);
    let mut start = 8;
    for subauthority in subauthorities {
        // each subauthority is stored as 4 bytes little endian
        res[start..start + 4].copy_from_slice(&subauthority.to_le_bytes());
        start += 4;
    }
    Ok(res)
}

/// Wrapper for handle returned by [`LsaOpenPolicy`] that will close is on drop
///
/// ['LsaOpenPolicy`]: https://learn.microsoft.com/en-us/windows/win32/api/ntsecapi/nf-ntsecapi-lsaopenpolicy
struct LsaPolicy {
    handle: LSA_HANDLE,
}

impl LsaPolicy {
    /// Creates new handle to LSA policy with read and write access
    fn new() -> Result<Self> {
        let mut policy = LsaPolicy {
            handle: LSA_HANDLE::default(),
        };
        let desired_access = GENERIC_WRITE | GENERIC_READ;
        let object_attributes = LSA_OBJECT_ATTRIBUTES::default();
        unsafe {
            LsaOpenPolicy(
                None,
                &object_attributes,
                desired_access.0,
                &mut policy.handle,
            )
        }
        .ok()?;
        Ok(policy)
    }

    /// Wrapper for [`LsaStorePrivateData`] that stores private `data` indexed by `key`.
    ///
    /// [`LsaStorePrivateData`]: https://learn.microsoft.com/en-us/windows/win32/api/ntsecapi/nf-ntsecapi-lsastoreprivatedata
    fn store_private_data(&self, key: &str, data: &str) -> Result<()> {
        let mut key = UTF16::from(key);
        let mut data = UTF16::from(data);
        unsafe {
            LsaStorePrivateData(
                self.handle,
                &key.lsa_unicode_string(),
                Some(&data.lsa_unicode_string()),
            )
        }
        .ok()
        .context("storing private data")
    }

    /// Wrapper for [`LsaRetrievePrivateData`] that returns private data indexed by `key` as a string.
    ///
    /// [`LsaRetrievePrivateData`]: https://learn.microsoft.com/en-us/windows/win32/api/ntsecapi/nf-ntsecapi-lsaretrieveprivatedata
    fn retrieve_private_data(&mut self, key: &str) -> Result<Option<String>> {
        let mut key = UTF16::from(key);
        let mut data: *mut LSA_UNICODE_STRING = ptr::null_mut();
        unsafe {
            let status = LsaRetrievePrivateData(self.handle, &key.lsa_unicode_string(), &mut data);
            if status == STATUS_OBJECT_NAME_NOT_FOUND {
                return Ok(None);
            }
            status.ok()?;
            let len = ((*data).Length as usize) / size_of::<u16>();

            let s = String::from_utf16(slice::from_raw_parts((*data).Buffer.as_ptr(), len));
            let _ = LsaFreeMemory(Some(data as *mut c_void));
            Ok(Some(s?))
        }
    }
}

impl Drop for LsaPolicy {
    /// Closes policy handle returned by [`LsaOpenPolicy`] using [`LsaClose`]
    ///
    /// [`LsaOpenPolicy`]: https://learn.microsoft.com/en-us/windows/win32/api/ntsecapi/nf-ntsecapi-lsaopenpolicy
    /// [`LsaClose`]: https://learn.microsoft.com/en-us/windows/win32/api/ntsecapi/nf-ntsecapi-lsaclose
    fn drop(&mut self) {
        if !self.handle.is_invalid() {
            if let Err(e) = unsafe { LsaClose(self.handle) }.ok() {
                warn!("Can't close LSA policy: {}", e);
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use anyhow::Result;

    use crate::auth::to_sid;

    #[test]
    fn to_sid_ok() -> Result<()> {
        let sid = to_sid("S-1-5-21-1686530393-9139194-3084028869-513")?;
        assert_eq!(
            sid,
            [
                1, 5, 0, 0, 0, 0, 0, 5, 21, 0, 0, 0, 89, 105, 134, 100, 250, 115, 139, 0, 197, 139,
                210, 183, 1, 2, 0, 0
            ]
        );
        let sid = to_sid("S-1-5")?; // no subauthorities
        assert_eq!(sid, [1, 0, 0, 0, 0, 0, 0, 5]);
        Ok(())
    }

    #[test]
    fn to_sid_err() {
        assert!(to_sid("S-1").is_err());
        assert!(to_sid("").is_err());
        assert!(to_sid("S-1-5-g").is_err());
    }
}
