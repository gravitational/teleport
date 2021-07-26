use libc::{size_t, c_uchar};
use crepe::crepe;
use prost::Message;
use bytes::BytesMut;
use std::{ptr, slice};

pub mod types {
    include!(concat!(env!("OUT_DIR"), "/types.rs"));
}

// Login trait hash is the value for all login traits, equal to the Go library's definition.
const LOGIN_TRAIT_HASH: u32 = 0;
enum Errors {
    InvalidInputError = -1,
    InvalidOutputError = -2,
}

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

#[no_mangle]
pub extern "C" fn process_access(input: *mut c_uchar, output: *mut c_uchar, input_len: size_t, output_len: size_t) -> i32 {
    let mut runtime = Crepe::new();
    let b = unsafe { slice::from_raw_parts_mut(input, input_len) };
    let r = match types::Facts::decode(BytesMut::from(&b[..])) {
        Ok(b) => b,
        Err(_e) => return Errors::InvalidInputError as i32
    };

    for pred in &r.predicates {
        if pred.name == types::facts::PredicateType::Hasrole as i32 {
            runtime.extend(&[HasRole(pred.atoms[0], pred.atoms[1])]);
        } else if pred.name == types::facts::PredicateType::Hastrait as i32 {
            runtime.extend(&[HasTrait(pred.atoms[0], pred.atoms[1], pred.atoms[2])]);
        } else if pred.name == types::facts::PredicateType::Roleallowslogin as i32 {
            runtime.extend(&[RoleAllowsLogin(pred.atoms[0], pred.atoms[1])]);
        } else if pred.name == types::facts::PredicateType::Roledenieslogin as i32 {
            runtime.extend(&[RoleDeniesLogin(pred.atoms[0], pred.atoms[1])]);
        } else if pred.name == types::facts::PredicateType::Roleallowsnodelabel as i32 {
            runtime.extend(&[RoleAllowsNodeLabel(pred.atoms[0], pred.atoms[1], pred.atoms[2])]);
        } else if pred.name == types::facts::PredicateType::Roledeniesnodelabel as i32 {
            runtime.extend(&[RoleDeniesNodeLabel(pred.atoms[0], pred.atoms[1], pred.atoms[2])]);
        } else if pred.name == types::facts::PredicateType::Nodehaslabel as i32 {
            runtime.extend(&[NodeHasLabel(pred.atoms[0], pred.atoms[1], pred.atoms[2])]);
        } else {
            return Errors::InvalidInputError as i32;
        }
    }

    let (accesses, deny_accesses, deny_logins) = runtime.run();
    let mut predicates = vec![];
    predicates.extend::<Vec<_>>(accesses.into_iter().map(|HasAccess(a, b, c, d)| types::facts::Predicate{
        name: types::facts::PredicateType::Hasaccess as i32,
        atoms: vec![a, b, c, d],
    }).collect());
    predicates.extend::<Vec<_>>(deny_accesses.into_iter().map(|DenyAccess(a, b, c, d)| types::facts::Predicate{
        name: types::facts::PredicateType::Denyaccess as i32,
        atoms: vec![a, b, c, d],
    }).collect());
    predicates.extend::<Vec<_>>(deny_logins.into_iter().map(|DenyLogins(a, b, c)| types::facts::Predicate{
        name: types::facts::PredicateType::Denylogins as i32,
        atoms: vec![a, b, c],
    }).collect());

    let idb = types::Facts {
        predicates: predicates
    };

    let mut buf = Vec::with_capacity(idb.encoded_len());
    match idb.encode(&mut buf) {
        Ok(_) => (),
        Err(_e) => return Errors::InvalidOutputError as i32,
    };
    unsafe {
        if buf.len() == 0 || buf.len() > output_len {
            return buf.len() as i32
        }
        ptr::copy_nonoverlapping(&(*buf)[0], output, buf.len());
    }

    buf.len() as i32
}

#[no_mangle]
extern "C" fn deallocate_rust_buffer(ptr: *mut c_uchar, len: size_t) {
    let len = len as usize;
    unsafe { drop(slice::from_raw_parts(ptr, len)) };
}
