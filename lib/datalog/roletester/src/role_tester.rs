use bytes::BytesMut;
use crepe::crepe;
use libc::{c_uchar, size_t};
use prost::Message;
use std::{ptr, slice};

pub mod types {
    include!(concat!(env!("OUT_DIR"), "/datalog.rs"));
}

// Login trait hash is the value for all login traits, equal to the Go library's definition.
const LOGIN_TRAIT_HASH: u32 = 0;

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

#[repr(C)]
pub struct Status {
    output_length: size_t,
    error: i32
}

#[no_mangle]
pub extern "C" fn process_access(
    input: *mut c_uchar,
    output: *mut c_uchar,
    input_len: size_t,
    output_len: size_t
) -> *mut Status {
    let mut runtime = Crepe::new();
    let b = unsafe { slice::from_raw_parts_mut(input, input_len) };
    let r = match types::Facts::decode(BytesMut::from(&b[..])) {
        Ok(b) => b,
        Err(e) => {
            let err_bytes = e.to_string().into_bytes();
            unsafe {
                ptr::copy(&(*err_bytes)[0], output, err_bytes.len());
            }
            return Box::into_raw(Box::new(Status{
                output_length: err_bytes.len(),
                error: -1,
            }))
        },
    };

    for pred in &r.predicates {
        if pred.name == types::facts::PredicateType::HasRole as i32 {
            runtime.extend(&[HasRole(pred.atoms[0], pred.atoms[1])]);
        } else if pred.name == types::facts::PredicateType::HasTrait as i32 {
            runtime.extend(&[HasTrait(pred.atoms[0], pred.atoms[1], pred.atoms[2])]);
        } else if pred.name == types::facts::PredicateType::RoleAllowsLogin as i32 {
            runtime.extend(&[RoleAllowsLogin(pred.atoms[0], pred.atoms[1])]);
        } else if pred.name == types::facts::PredicateType::RoleDeniesLogin as i32 {
            runtime.extend(&[RoleDeniesLogin(pred.atoms[0], pred.atoms[1])]);
        } else if pred.name == types::facts::PredicateType::RoleAllowsNodeLabel as i32 {
            runtime.extend(&[RoleAllowsNodeLabel(
                pred.atoms[0],
                pred.atoms[1],
                pred.atoms[2],
            )]);
        } else if pred.name == types::facts::PredicateType::RoleDeniesNodeLabel as i32 {
            runtime.extend(&[RoleDeniesNodeLabel(
                pred.atoms[0],
                pred.atoms[1],
                pred.atoms[2],
            )]);
        } else if pred.name == types::facts::PredicateType::NodeHasLabel as i32 {
            runtime.extend(&[NodeHasLabel(pred.atoms[0], pred.atoms[1], pred.atoms[2])]);
        }
    }

    let (accesses, deny_accesses, deny_logins) = runtime.run();
    let mut predicates = vec![];
    predicates.extend(
        accesses
            .into_iter()
            .map(|HasAccess(a, b, c, d)| types::facts::Predicate {
                name: types::facts::PredicateType::HasAccess as i32,
                atoms: vec![a, b, c, d],
            }),
    );
    predicates.extend(
        deny_accesses
            .into_iter()
            .map(|DenyAccess(a, b, c, d)| types::facts::Predicate {
                name: types::facts::PredicateType::DenyAccess as i32,
                atoms: vec![a, b, c, d],
            })
    );
    predicates.extend(
        deny_logins
            .into_iter()
            .map(|DenyLogins(a, b, c)| types::facts::Predicate {
                name: types::facts::PredicateType::DenyLogins as i32,
                atoms: vec![a, b, c],
            })
    );

    let idb = types::Facts {
        predicates,
    };

    let mut buf = Vec::with_capacity(idb.encoded_len());
        if let Err(e) = idb.encode(&mut buf) {
            let err_bytes = e.to_string().into_bytes();
            unsafe {
                ptr::copy(&(*err_bytes)[0], output, err_bytes.len());
            }
            return Box::into_raw(Box::new(Status{
                output_length: err_bytes.len(),
                error: -1,
            }))
        }
    if buf.is_empty() || buf.len() > output_len {
        return Box::into_raw(Box::new(Status{
            output_length: buf.len(),
            error: 0,
        }))
    }
    // We can't guarantee the memory regions are non-overlapping, but we only need to copy the data to output so
    // it is able to be presented on the Go end. The necessary checks for length is done before this call.
    //
    // buf is valid for reads of len * size_of::<T>() bytes.
    // output is valid for writes of count * size_of::<T>() bytes.
    // Both buf and output are properly aligned.
    unsafe {
        ptr::copy(&(*buf)[0], output, buf.len());
    }

    Box::into_raw(Box::new(Status{
        output_length: buf.len(),
        error: 0
    }))
}

#[no_mangle]
pub extern "C" fn status_output_length(
    status: *mut Status
) -> size_t {
    if status.is_null() {
        return 0
    }
    // We've checked that the pointer is not null.
    let db = unsafe {
        &*status
    };
    db.output_length
}

#[no_mangle]
pub extern "C" fn status_error(
    status: *mut Status
) -> i32 {
    if status.is_null() {
        return 0
    }
    // We've checked that the pointer is not null.
    let db = unsafe {
        &*status
    };
    db.error
}

#[no_mangle]
extern "C" fn drop_status_struct(status: *mut Status) {
    if status.is_null() {
        return;
    }
    unsafe {
        Box::from_raw(status);
    }
}
