// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// the crepe! macro expands to code that triggers a linter warning.
// supressing the warning on the offending line breaks the macro,
// so we just disable it for the entire file
#![allow(clippy::collapsible_if)]

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
pub struct Output {
    access: *mut u8,
    length: size_t,
    error: i32,
}

fn create_error_ptr(e: String) -> Output {
    let mut err_bytes = e.into_bytes().into_boxed_slice();
    let err_ptr = err_bytes.as_mut_ptr();
    let err_len = err_bytes.len();
    std::mem::forget(err_bytes);

    Output {
        access: err_ptr,
        length: err_len,
        error: -1,
    }
}

/// # Safety
///
/// This function should not be called if input is invalid
#[no_mangle]
pub unsafe extern "C" fn process_access(input: *mut c_uchar, input_len: size_t) -> *mut Output {
    let mut runtime = Crepe::new();
    let b = slice::from_raw_parts_mut(input, input_len);
    let r = match types::Facts::decode(BytesMut::from(&b[..])) {
        Ok(b) => b,
        Err(e) => return Box::into_raw(Box::new(create_error_ptr(e.to_string()))),
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
    predicates.extend(deny_accesses.into_iter().map(|DenyAccess(a, b, c, d)| {
        types::facts::Predicate {
            name: types::facts::PredicateType::DenyAccess as i32,
            atoms: vec![a, b, c, d],
        }
    }));
    predicates.extend(
        deny_logins
            .into_iter()
            .map(|DenyLogins(a, b, c)| types::facts::Predicate {
                name: types::facts::PredicateType::DenyLogins as i32,
                atoms: vec![a, b, c],
            }),
    );

    let idb = types::Facts { predicates };

    let mut buf = Vec::with_capacity(idb.encoded_len());
    if let Err(e) = idb.encode(&mut buf) {
        return Box::into_raw(Box::new(create_error_ptr(e.to_string())));
    }

    let mut ret = buf.into_boxed_slice();
    let access_ptr = ret.as_mut_ptr();
    let access_len = ret.len();
    std::mem::forget(ret);

    Box::into_raw(Box::new(Output {
        access: access_ptr,
        length: access_len,
        error: 0,
    }))
}

/// # Safety
///
/// This function should not be called if output is invalid
#[no_mangle]
pub unsafe extern "C" fn output_access(output: *mut Output) -> *mut u8 {
    if let Some(db) = output.as_ref() {
        return db.access;
    }
    ptr::null_mut()
}

/// # Safety
///
/// This function should not be called if output is invalid
#[no_mangle]
pub unsafe extern "C" fn output_length(output: *mut Output) -> size_t {
    if let Some(db) = output.as_ref() {
        return db.length;
    }
    0
}

/// # Safety
///
/// This function should not be called if output is invalid
#[no_mangle]
pub unsafe extern "C" fn output_error(output: *mut Output) -> i32 {
    // We've checked that the pointer is not null.
    if let Some(db) = output.as_ref() {
        return db.error;
    }
    0
}

/// # Safety
///
/// This function should not be called if output is invalid
#[no_mangle]
pub unsafe extern "C" fn drop_output_struct(output: *mut Output) {
    let db = match output.as_ref() {
        Some(s) => s,
        None => return,
    };
    // Drop access buf
    if db.length > 0 {
        let s = std::slice::from_raw_parts_mut(db.access, db.length);
        let s = s.as_mut_ptr();
        Box::from_raw(s);
    }
    // Drop struct
    Box::from_raw(output);
}
