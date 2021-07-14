extern crate libc;
use crepe::crepe;
use libc::c_char;
use std::ffi::CString;
use std::ffi::CStr;
use serde::{Deserialize, Serialize};

static LOGIN_TRAIT_HASH: u32 = 0;

crepe! {
    // Input from EDB
    @input
    struct HasRole(u32, u32);
    @input
    struct HasTrait(u32, u32, u32);
    @input
    struct NodeHasLabel(u32, u32, u32);
    @input
    struct RoleAllowsNodeLabel(u32, u32, u32);
    @input
    struct RoleDeniesNodeLabel(u32, u32, u32);
    @input
    struct RoleAllowsLogin(u32, u32);
    @input
    struct RoleDeniesLogin(u32, u32);

    // Intermediate rules
    struct HasAllowNodeLabel(u32, u32, u32, u32);
    struct HasDenyNodeLabel(u32, u32, u32, u32);
    struct HasAllowRole(u32, u32, u32, u32);
    struct HasDenyRole(u32, u32, u32);
    struct HasDeniedLogin(u32, u32, u32);

    // Output for IDB
    @output
    struct HasAccess(u32, u32, u32, u32);
    @output
    struct DenyAccess(u32, u32, u32, u32);
    @output
    struct DenyLogins(u32, u32, u32);

    // Intermediate rules to help determine access
    HasAllowNodeLabel(role, node, key, value) <- RoleAllowsNodeLabel(role, key, value), NodeHasLabel(node, key, value);
    HasDenyNodeLabel(role, node, key, value) <- RoleDeniesNodeLabel(role, key, value), NodeHasLabel(node, key, value);
    HasAllowRole(user, login, node, role) <- HasRole(user, role), HasAllowNodeLabel(role, node, _, _), RoleAllowsLogin(role, login),
        !RoleDeniesLogin(role, login);
    HasAllowRole(user, login, node, role) <- HasRole(user, role), HasAllowNodeLabel(role, node, _, _), HasTrait(user, LOGIN_TRAIT_HASH, login),
        !RoleDeniesLogin(role, login), !RoleDeniesLogin(role, LOGIN_TRAIT_HASH);
    HasDenyRole(user, node, role) <- HasRole(user, role), HasDenyNodeLabel(role, node, _, _);
    HasDeniedLogin(user, login, role) <- HasRole(user, role), RoleDeniesLogin(role, login);
    HasDeniedLogin(user, login, role) <- HasRole(user, role), HasTrait(user, LOGIN_TRAIT_HASH, login), RoleDeniesLogin(role, LOGIN_TRAIT_HASH);

    // HasAccess rule determines each access for a specified user, login and node
    HasAccess(user, login, node, role) <- HasAllowRole(user, login, node, role), !HasDenyRole(user, node, _), !HasDeniedLogin(user, login, _);
    DenyAccess(user, login, node, role) <- HasDenyRole(user, node, role), HasTrait(user, LOGIN_TRAIT_HASH, login);
    DenyAccess(user, login, node, role) <- HasDenyRole(user, node, role), HasAllowRole(user, login, node, _);

    DenyLogins(user, login, role) <- HasDeniedLogin(user, login, role);
}

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
struct Predicate {
    atoms: Vec<u32>
}

#[derive(Serialize, Deserialize)]
struct EDB {
    has_role: Option<Vec<Predicate>>,
	has_trait: Option<Vec<Predicate>>,
	has_login_trait: Option<Vec<Predicate>>,
	role_allows_login: Option<Vec<Predicate>>,
	role_denies_login: Option<Vec<Predicate>>,
	role_allows_node_label: Option<Vec<Predicate>>,
	role_denies_node_label: Option<Vec<Predicate>>,
	node_has_label: Option<Vec<Predicate>>,
}

#[derive(Serialize, Deserialize)]
struct IDB {
    accesses: Vec<Predicate>,
    deny_accesses: Vec<Predicate>,
    deny_logins: Vec<Predicate>
}

#[no_mangle]
pub extern "C" fn process_access(s: *const c_char) -> *mut c_char {
    let mut runtime = Crepe::new();
    let c_str = unsafe {
        assert!(!s.is_null());
        CStr::from_ptr(s)
    };
    let r_str = c_str.to_str().unwrap();
    let edb: EDB = serde_json::from_str(r_str).unwrap();
    
    match edb.has_role {
        Some(x) => {
            x.into_iter()
             .for_each(|Predicate{atoms}| runtime.extend(&[HasRole(atoms[0], atoms[1])]));
        },
        None => {}
    }

    match edb.has_trait {
        Some(x) => {
            x.into_iter()
             .for_each(|Predicate{atoms}| runtime.extend(&[HasTrait(atoms[0], atoms[1], atoms[2])]));
        },
        None => {}
    }
    
    match edb.role_allows_login {
        Some(x) => {
            x.into_iter()
             .for_each(|Predicate{atoms}| runtime.extend(&[RoleAllowsLogin(atoms[0], atoms[1])]));
        },
        None => {}
    }

    match edb.role_denies_login {
        Some(x) => {
            x.into_iter()
             .for_each(|Predicate{atoms}| runtime.extend(&[RoleDeniesLogin(atoms[0], atoms[1])]));
        },
        None => {}
    }

    match edb.role_allows_node_label {
        Some(x) => {
            x.into_iter()
             .for_each(|Predicate{atoms}| runtime.extend(&[RoleAllowsNodeLabel(atoms[0], atoms[1], atoms[2])]));
        },
        None => {}
    }

    match edb.role_denies_node_label {
        Some(x) => {
            x.into_iter()
             .for_each(|Predicate{atoms}| runtime.extend(&[RoleDeniesNodeLabel(atoms[0], atoms[1], atoms[2])]));
        },
        None => {}
    }

    match edb.node_has_label {
        Some(x) => {
            x.into_iter()
             .for_each(|Predicate{atoms}| runtime.extend(&[NodeHasLabel(atoms[0], atoms[1], atoms[2])]));
        },
        None => {}
    }

    // Format output into JSON
    let (accesses, deny_accesses, deny_logins) = runtime.run();
    let idb = IDB {
        accesses: accesses.into_iter().map(|HasAccess(a, b, c, d)| Predicate{atoms: vec![a, b, c, d]}).collect(),
        deny_accesses: deny_accesses.into_iter().map(|DenyAccess(a, b, c, d)| Predicate{atoms: vec![a, b, c, d]}).collect(),
        deny_logins: deny_logins.into_iter().map(|DenyLogins(a, b, c)| Predicate{atoms: vec![a, b, c]}).collect()
    };

    let c_str_song = CString::new(serde_json::to_string(&idb).unwrap()).unwrap();
    c_str_song.into_raw()
}
