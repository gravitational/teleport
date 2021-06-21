extern crate libc;
use crepe::crepe;
use libc::c_char;
use std::ffi::CString;
use std::ffi::CStr;
use serde::{Deserialize, Serialize};

crepe! {
    @input
    struct HasRole(u32, u32);
    @input
    struct HasLoginTrait(u32);
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

    struct HasAllowNodeLabel(u32, u32, u32, u32);
    struct HasDenyNodeLabel(u32, u32, u32, u32);

    struct HasAllowRole(u32, u32, u32);
    struct HasDenyRole(u32, u32);
    @output
    struct HasAccess(u32, u32, u32);

    @output
    struct AllowRoles(u32, u32, u32);
    @output
    struct DenyRoles(u32, u32, u32);

    HasAllowNodeLabel(role, node, key, value) <- RoleAllowsNodeLabel(role, key, value), NodeHasLabel(node, key, value);
    HasDenyNodeLabel(role, node, key, value) <- RoleDeniesNodeLabel(role, key, value), NodeHasLabel(node, key, value);

    HasAllowRole(user, login, node) <- HasRole(user, role), HasAllowNodeLabel(role, node, _, _), RoleAllowsLogin(role, login),
        !RoleDeniesLogin(role, login), !HasLoginTrait(user);
    HasAllowRole(user, login, node) <- HasRole(user, role), HasAllowNodeLabel(role, node, _, _), HasTrait(user, 0, login),
        !RoleDeniesLogin(role, login);
    HasDenyRole(user, node) <- HasRole(user, role), HasDenyNodeLabel(role, node, _, _);
    HasAccess(user, login, node) <- HasAllowRole(user, login, node), !HasDenyRole(user, node);

    AllowRoles(user, role, node) <- HasRole(user, role), HasAllowNodeLabel(role, node, _, _);
    DenyRoles(user, role, node) <- HasRole(user, role), HasDenyNodeLabel(role, node, _, _);
}

#[derive(Serialize, Deserialize)]
struct Predicate {
    Atoms: Vec<u32>
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
    
    match edb.has_role {
        Some(x) => {
            x.into_iter()
             .for_each(|Predicate{Atoms}| runtime.extend(&[HasRole(Atoms[0], Atoms[1])]));
        },
        None => {}
    }

    match edb.has_trait {
        Some(x) => {
            x.into_iter()
             .for_each(|Predicate{Atoms}| runtime.extend(&[HasTrait(Atoms[0], Atoms[1], Atoms[2])]));
        },
        None => {}
    }
    
    match edb.has_login_trait {
        Some(x) => {
            x.into_iter()
             .for_each(|Predicate{Atoms}| runtime.extend(&[HasLoginTrait(Atoms[0])]));
        },
        None => {}
    }

    match edb.role_allows_login {
        Some(x) => {
            x.into_iter()
             .for_each(|Predicate{Atoms}| runtime.extend(&[RoleAllowsLogin(Atoms[0], Atoms[1])]));
        },
        None => {}
    }

    match edb.role_denies_login {
        Some(x) => {
            x.into_iter()
             .for_each(|Predicate{Atoms}| runtime.extend(&[RoleDeniesLogin(Atoms[0], Atoms[1])]));
        },
        None => {}
    }

    match edb.role_allows_node_label {
        Some(x) => {
            x.into_iter()
             .for_each(|Predicate{Atoms}| runtime.extend(&[RoleAllowsNodeLabel(Atoms[0], Atoms[1], Atoms[2])]));
        },
        None => {}
    }

    match edb.role_denies_node_label {
        Some(x) => {
            x.into_iter()
             .for_each(|Predicate{Atoms}| runtime.extend(&[RoleDeniesNodeLabel(Atoms[0], Atoms[1], Atoms[2])]));
        },
        None => {}
    }

    match edb.node_has_label {
        Some(x) => {
            x.into_iter()
             .for_each(|Predicate{Atoms}| runtime.extend(&[NodeHasLabel(Atoms[0], Atoms[1], Atoms[2])]));
        },
        None => {}
    }

    let (accesses, allow_roles, deny_roles) = runtime.run();
    let mut accesses = accesses.into_iter().collect::<Vec<_>>();
    accesses.sort_by(|HasAccess(a, _, _), HasAccess(b, _, _)| a.cmp(b));

    let mut allow_roles = allow_roles.into_iter().collect::<Vec<_>>();
    allow_roles.sort_by(|AllowRoles(a, _, _), AllowRoles(b, _, _)| a.cmp(b));

    let mut deny_roles = deny_roles.into_iter().collect::<Vec<_>>();
    deny_roles.sort_by(|DenyRoles(a, _, _), DenyRoles(b, _, _)| a.cmp(b));

    let idb = IDB {
        accesses: accesses.into_iter().map(|HasAccess(a, b, c)| Predicate{Atoms: vec![a, b, c]}).collect(),
        allow_roles: allow_roles.into_iter().map(|AllowRoles(a, b, c)| Predicate{Atoms: vec![a, b, c]}).collect(),
        deny_roles: deny_roles.into_iter().map(|DenyRoles(a, b, c)| Predicate{Atoms: vec![a, b, c]}).collect()
    };

    let c_str_song = CString::new(serde_json::to_string(&idb).unwrap()).unwrap();
    c_str_song.into_raw()
}
