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

use crepe::crepe;
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

type Output = Result<Vec<u8>, String>;

/// # Safety
///
/// `input` should point to a buffer of size at least `input_len` bytes.
#[no_mangle]
pub unsafe extern "C" fn process_access(input: *const u8, input_len: usize) -> Box<Output> {
    let input = slice::from_raw_parts(input, input_len);

    fn inner(input: &[u8]) -> Output {
        let facts = types::Facts::decode(input).map_err(|e| e.to_string())?;

        Ok(process_facts(facts).encode_to_vec())
    }

    Box::new(inner(input))
}

fn process_facts(facts: types::Facts) -> types::Facts {
    let mut runtime = Crepe::new();

    for pred in &facts.predicates {
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

    types::Facts { predicates }
}

/// Get a pointer to the data from `output`.
#[no_mangle]
pub extern "C" fn output_data(output: Option<&Output>) -> *const u8 {
    match output {
        Some(Ok(b)) => b.as_ptr(),
        Some(Err(e)) => e.as_ptr(),
        None => ptr::null(),
    }
}

/// Get the length of the data from `output`.
#[no_mangle]
pub extern "C" fn output_len(output: Option<&Output>) -> usize {
    match output {
        Some(Ok(b)) => b.len(),
        Some(Err(e)) => e.len(),
        None => 0,
    }
}

/// Returns 0 if `output` is `Ok`, -1 if `output` is `Err`.
#[no_mangle]
pub extern "C" fn output_err(output: Option<&Output>) -> i32 {
    match output {
        Some(Ok(_)) => 0,
        Some(Err(_)) => -1,
        None => -1,
    }
}

/// Drops the input value.
#[no_mangle]
pub extern "C" fn output_drop(_: Option<Box<Output>>) {}
