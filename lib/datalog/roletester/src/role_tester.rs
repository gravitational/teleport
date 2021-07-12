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
    struct HasAllowRole(u32, u32, u32);
    struct HasDenyRole(u32, u32);
    struct HasDeniedLogin(u32, u32);

    // Output for IDB
    @output
    struct HasAccess(u32, u32, u32);
    @output
    struct AllowRoles(u32, u32, u32, u32);
    @output
    struct DenyRoles(u32, u32, u32, u32);

    // Intermediate rules to help determine access
    HasAllowNodeLabel(role, node, key, value) <- RoleAllowsNodeLabel(role, key, value), NodeHasLabel(node, key, value);
    HasDenyNodeLabel(role, node, key, value) <- RoleDeniesNodeLabel(role, key, value), NodeHasLabel(node, key, value);
    HasAllowRole(user, login, node) <- HasRole(user, role), HasAllowNodeLabel(role, node, _, _), RoleAllowsLogin(role, login),
        !RoleDeniesLogin(role, login);
    HasAllowRole(user, login, node) <- HasRole(user, role), HasAllowNodeLabel(role, node, _, _), HasTrait(user, LOGIN_TRAIT_HASH, login),
        !RoleDeniesLogin(role, login);
    HasDenyRole(user, node) <- HasRole(user, role), HasDenyNodeLabel(role, node, _, _);
    HasDeniedLogin(user, login) <- HasRole(user, role), RoleDeniesLogin(role, login);

    // HasAccess rule determines each access for a specified user, login and node
    HasAccess(user, login, node) <- HasAllowRole(user, login, node), !HasDenyRole(user, node), !HasDeniedLogin(user, login);

    // AllowRoles rule determines each role that allows a user access with a given login to node
    AllowRoles(user, login, node, role) <- HasRole(user, role), HasAllowNodeLabel(role, node, _, _), RoleAllowsLogin(role, login),
    !RoleDeniesLogin(role, login);
    AllowRoles(user, login, node, role) <- HasRole(user, role), HasAllowNodeLabel(role, node, _, _), HasTrait(user, LOGIN_TRAIT_HASH, login),
    !RoleDeniesLogin(role, login);

    // DenyRoles rule determines each denied access given a user, login, and node. The role that denies the access is also provided
    DenyRoles(user, login, node, role) <- HasRole(user, role), HasDenyNodeLabel(role, node, _, _), RoleAllowsLogin(role, login);
    DenyRoles(user, login, node, role) <- HasRole(user, role), HasDenyNodeLabel(role, node, _, _), RoleDeniesLogin(role, login);
    DenyRoles(user, login, node, role) <- HasRole(user, role), HasDenyNodeLabel(role, node, _, _), HasTrait(user, LOGIN_TRAIT_HASH, login);
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
    allow_roles: Vec<Predicate>,
    deny_roles: Vec<Predicate>
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
    
    // Create each predicate
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
    let (accesses, allow_roles, deny_roles) = runtime.run();
    let mut accesses = accesses.into_iter().collect::<Vec<_>>();
    accesses.sort_by(|HasAccess(a, _, _), HasAccess(b, _, _)| a.cmp(b));

    let mut allow_roles = allow_roles.into_iter().collect::<Vec<_>>();
    allow_roles.sort_by(|AllowRoles(_, _, _, a), AllowRoles(_, _, _, b)| a.cmp(b));

    let mut deny_roles = deny_roles.into_iter().collect::<Vec<_>>();
    deny_roles.sort_by(|DenyRoles(a, _, _, _), DenyRoles(b, _, _, _)| a.cmp(b));

    let idb = IDB {
        accesses: accesses.into_iter().map(|HasAccess(a, b, c)| Predicate{atoms: vec![a, b, c]}).collect(),
        allow_roles: allow_roles.into_iter().map(|AllowRoles(a, b, c, d)| Predicate{atoms: vec![a, b, c, d]}).collect(),
        deny_roles: deny_roles.into_iter().map(|DenyRoles(a, b, c, d)| Predicate{atoms: vec![a, b, c, d]}).collect()
    };

    let c_str_song = CString::new(serde_json::to_string(&idb).unwrap()).unwrap();
    c_str_song.into_raw()
}
