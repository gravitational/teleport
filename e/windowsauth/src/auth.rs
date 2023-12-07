use std::ffi::c_void;
use std::sync::atomic::{AtomicPtr, Ordering};
use std::{mem, ptr, slice};

use anyhow::{Context, Result};
use log::{debug, error, info};
use windows::{
    core::*, Win32::Foundation::*, Win32::NetworkManagement::NetManagement::*,
    Win32::Security::Authentication::Identity::*, Win32::Security::Authorization::*,
    Win32::Security::Credentials::*, Win32::Security::*, Win32::System::Kernel::*,
    Win32::System::SystemServices::*, Win32::System::WindowsProgramming::*, *,
};

use crate::crypto::LicenseType;
use crate::crypto::{CryptContext, UserCreation};

static DISPATCH_TABLE: AtomicPtr<LSA_SECPKG_FUNCTION_TABLE> = AtomicPtr::new(ptr::null_mut());

fn dispatch_table() -> LSA_SECPKG_FUNCTION_TABLE {
    unsafe { *DISPATCH_TABLE.load(Ordering::SeqCst) }
}

/// allocate_lsa_heap_size allocates memory using AllocateLsaHeap function. It's used in most places
/// where we need to return data to LSA.
///
/// See: https://learn.microsoft.com/en-us/windows/win32/api/ntsecpkg/nc-ntsecpkg-lsa_allocate_lsa_heap
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

/// Convenience method for calling allocate_lsa_heap_size with size calculated by the compiler.
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
        grpi1_name: PWSTR::from_raw(uname.0.as_mut_ptr() as _),
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
            let user = lookup_name(name)?;
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

/// LsaAPLogonUser is the main function responsible for validating a user's login and creating a user token.
///
/// See: https://learn.microsoft.com/en-us/windows/win32/api/ntsecapi/nf-ntsecapi-lsalogonuser
///
/// Windows promises that all pointers provided are valid, we do additional check to avoid null pointers.
#[no_mangle]
unsafe extern "system" fn LsaApLogonUser(
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
) -> NTSTATUS {
    debug!("LsaApLogonUser");
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

    let user = lookup_name(&name)?;
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

    let groups = select_groups(&name, should_create_user)?;
    copy_groups_to_token(token, groups).context("Can't copy groups to token")?;
    copy_user_to_token(token, &user).context("Can't copy user to token")?;
    copy_primary_group_to_token(&name, token, &user)
        .context("Can't copy primary group to token")?;

    info!("User {} logged in successfully", name);
    Ok(())
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

/// select_groups will return groups that should be included in the token returned to LSA.
fn select_groups(name: &str, should_create_user: UserCreation) -> Result<Vec<String>> {
    let mut groups = lookup_groups(name)?;

    // auto user creation was requested and user is managed by Teleport, return groups requested in the certificate
    if let UserCreation::Yes(requested_groups) = should_create_user {
        if groups.contains(&TELEPORT_USERS_GROUP.to_string()) {
            debug!(
                "User managed by Teleport, creating requested groups {}",
                requested_groups.join(", ")
            );
            for group in &requested_groups {
                create_group(group)?;
            }
            groups = requested_groups;
        }
    }
    Ok(groups)
}

/// REMOTE_DESKTOP_USERS_SID is [well-known SID] for users that can connect through RDP.
///
/// [well-known SID]: https://learn.microsoft.com/en-us/windows/win32/secauthz/well-known-sids
const REMOTE_DESKTOP_USERS_SID: &str = "S-1-5-32-555";

unsafe fn copy_groups_to_token(
    token: &mut LSA_TOKEN_INFORMATION_V1,
    groups: Vec<String>,
) -> Result<()> {
    debug!("Groups added to token: {}", groups.join(", "));

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
            GroupCount: groups.len() as u32,
            Groups: Default::default(),
        },
    );
    let token_groups = unsafe { (*token.Groups).Groups.as_mut_ptr() };

    // put REMOTE_DESKTOP_USERS_SID as first element in array
    let remote_desktop_users_sid = LocalSID::from(REMOTE_DESKTOP_USERS_SID)?;
    copy_sid(
        token_groups,
        0,
        remote_desktop_users_sid.length,
        remote_desktop_users_sid.psid,
    )?;

    // put all requested groups' SIDs in array starting at index 1
    for (i, group) in groups.iter().enumerate() {
        let group = lookup_name(group).context(format!("Can't lookup SID for group {}", group))?;
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

fn lookup_groups(name: &str) -> Result<Vec<String>> {
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
    name_use: SID_NAME_USE,
}

impl Account {
    fn psid(&self) -> PSID {
        PSID(self.sid.as_ptr() as _)
    }

    fn sid_length(&self) -> Result<u32> {
        self.sid.len().try_into().context("SID to long")
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
    let mut domain_acc = lookup_name(domain).context(format!("Can't lookup domain {}", domain))?;
    if domain_acc.name_use != SidTypeDomain {
        return Err(Error::from(ERROR_INVALID_NAME)).context(format!(
            "Lookup returned wrong SID type: {}",
            domain_acc.name_use.0
        ));
    }
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
    let sid = LocalSID::from(&domain_rid)?;
    let psid = sid.into();
    let len = GetLengthSid(psid);
    let mut buf = vec![0u8; len as _];
    CopySid(len, PSID(buf.as_mut_ptr() as _), psid).context("Can't copy SID")?;
    domain_acc.sid = buf;
    domain_acc.name_use = SidTypeGroup;
    Ok(domain_acc)
}

fn lookup_name(name: &str) -> Result<Account> {
    let mut cb = 0u32;
    let mut cd = 0u32;
    let mut name_use = SidTypeInvalid;
    let account_name = UTF16::from(name);
    let res = unsafe {
        LookupAccountNameW(
            None,
            account_name.pcwstr(),
            PSID::default(),
            &mut cb,
            PWSTR::null(),
            &mut cd,
            &mut name_use,
        )
    };
    if let Err(err) = res {
        if err != ERROR_INSUFFICIENT_BUFFER.into() {
            return Err(err).context(format!("Can't lookup account '{}' name length", name))?;
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
            account.psid(),
            &mut cb,
            domain,
            &mut cd,
            &mut name_use,
        )
        .context(format!("Can't lookup account name {}", name))?;
        account.domain = domain.to_string()?;
    }
    account.name_use = name_use;
    Ok(account)
}

unsafe fn lsa_string(name: &str) -> Result<*mut UNICODE_STRING> {
    let uname = allocate_lsa_heap::<UNICODE_STRING>()?;
    let utf: Vec<u16> = name.encode_utf16().collect();
    let buffer = allocate_lsa_heap_size((utf.len() * mem::size_of::<u16>()).try_into()?)?;
    ptr::copy(utf.as_ptr(), buffer, utf.len());
    let size = (utf.len() * mem::size_of::<u16>()) as u16;
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
    let _ = LocalFree(HLOCAL(s.as_ptr() as _));
    converted
}

struct LocalSID {
    psid: PSID,
    length: u32,
}

impl LocalSID {
    unsafe fn from(s: &str) -> Result<LocalSID> {
        let mut sid = PSID::default();
        let s = s.to_owned() + "\0";
        ConvertStringSidToSidA(PCSTR::from_raw(s.as_ptr()), &mut sid)
            .context(format!("Can't convert {} to SID", s))?;
        Ok(LocalSID {
            psid: sid,
            length: GetLengthSid(sid),
        })
    }
}

impl From<LocalSID> for PSID {
    fn from(val: LocalSID) -> Self {
        val.psid
    }
}

impl Drop for LocalSID {
    fn drop(&mut self) {
        if let Err(e) = unsafe { LocalFree(HLOCAL(self.psid.0 as _)) } {
            if e.code() != S_OK {
                error!("Can't free SID memory {}", e);
            }
        }
    }
}

#[derive(Clone, Debug)]
struct UTF16(Vec<u16>);

impl UTF16 {
    pub fn from(s: &str) -> UTF16 {
        let mut vec: Vec<u16> = s.encode_utf16().collect();
        vec.push(0);
        UTF16(vec)
    }

    pub fn pcwstr(&self) -> PCWSTR {
        PCWSTR::from_raw(self.0.as_ptr())
    }

    pub fn pwstr(&mut self) -> PWSTR {
        PWSTR::from_raw(self.0.as_mut_ptr())
    }
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
