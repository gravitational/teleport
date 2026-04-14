// Teleport
// Copyright (C) 2026  Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

//! Formally verified access list membership checking logic.
//!
//! This module implements the core `user_meets_requirements` function in a subset
//! of Rust compatible with Aeneas (https://github.com/AeneasVerif/aeneas), which
//! can auto-translate it to Lean4 for formal verification.
//!
//! The equivalent Go function is `UserMeetsRequirements` in
//! `lib/accesslists/hierarchy.go`.

#[cfg(feature = "ffi")]
mod ffi;

/// A trait entry: a key mapped to a list of values.
/// Uses Vec instead of HashMap for Aeneas compatibility.
#[derive(Clone, Debug, PartialEq)]
#[cfg_attr(feature = "ffi", derive(serde::Deserialize, serde::Serialize))]
pub struct TraitEntry {
    pub key: String,
    pub values: Vec<String>,
}

/// Membership/ownership requirements for an access list.
/// Mirrors `accesslist.Requires` in Go.
#[derive(Clone, Debug, PartialEq)]
#[cfg_attr(feature = "ffi", derive(serde::Deserialize, serde::Serialize))]
pub struct Requires {
    pub roles: Vec<String>,
    pub traits: Vec<TraitEntry>,
}

/// User information needed for requirements checking.
/// Mirrors the subset of `types.User` used by `UserMeetsRequirements`.
#[derive(Clone, Debug, PartialEq)]
#[cfg_attr(feature = "ffi", derive(serde::Deserialize, serde::Serialize))]
pub struct UserInfo {
    pub roles: Vec<String>,
    pub traits: Vec<TraitEntry>,
}

/// Returns true if the requirements are empty (no roles and no traits required).
pub fn requires_is_empty(req: &Requires) -> bool {
    req.roles.is_empty() && req.traits.is_empty()
}

/// Returns true if `needle` is found in `haystack`.
/// Uses a while-loop for Aeneas compatibility (no iterators).
pub fn vec_contains(haystack: &[String], needle: &str) -> bool {
    let mut i: usize = 0;
    while i < haystack.len() {
        if haystack[i] == needle {
            return true;
        }
        i += 1;
    }
    false
}

/// Finds the values associated with `key` in the trait list.
/// Returns None if the key is not found.
pub fn find_trait_values<'a>(traits: &'a [TraitEntry], key: &str) -> Option<&'a Vec<String>> {
    let mut i: usize = 0;
    while i < traits.len() {
        if traits[i].key == key {
            return Some(&traits[i].values);
        }
        i += 1;
    }
    None
}

/// Returns true if all `needles` are found in `haystack`.
/// Extracted as a separate function to avoid nested loops (Aeneas limitation).
pub fn all_contained(haystack: &[String], needles: &[String]) -> bool {
    let mut i: usize = 0;
    while i < needles.len() {
        if !vec_contains(haystack, &needles[i]) {
            return false;
        }
        i += 1;
    }
    true
}

/// Returns true if the user's roles contain all required roles.
pub fn check_roles(user_roles: &[String], required_roles: &[String]) -> bool {
    all_contained(user_roles, required_roles)
}

/// Returns true if a single trait requirement is satisfied by the user's traits.
pub fn check_single_trait(user_traits: &[TraitEntry], req_entry: &TraitEntry) -> bool {
    match find_trait_values(user_traits, &req_entry.key) {
        None => false,
        Some(user_values) => all_contained(user_values, &req_entry.values),
    }
}

/// Returns true if all trait requirements are satisfied by the user's traits.
pub fn check_traits(user_traits: &[TraitEntry], required_traits: &[TraitEntry]) -> bool {
    let mut i: usize = 0;
    while i < required_traits.len() {
        if !check_single_trait(user_traits, &required_traits[i]) {
            return false;
        }
        i += 1;
    }
    true
}

/// Checks whether the user meets the given access list requirements.
///
/// A user meets the requirements if and only if:
/// - The requirements are empty, OR
/// - The user has ALL required roles AND ALL required trait key-value pairs.
///
/// This is a pure function with no side effects, suitable for formal verification.
/// It mirrors the Go function `UserMeetsRequirements` in `lib/accesslists/hierarchy.go`.
pub fn user_meets_requirements(user: &UserInfo, requires: &Requires) -> bool {
    if requires_is_empty(requires) {
        return true;
    }

    check_roles(&user.roles, &requires.roles) && check_traits(&user.traits, &requires.traits)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_user(roles: &[&str], traits: &[(&str, &[&str])]) -> UserInfo {
        UserInfo {
            roles: roles.iter().map(|r| r.to_string()).collect(),
            traits: traits
                .iter()
                .map(|(k, vs)| TraitEntry {
                    key: k.to_string(),
                    values: vs.iter().map(|v| v.to_string()).collect(),
                })
                .collect(),
        }
    }

    fn make_requires(roles: &[&str], traits: &[(&str, &[&str])]) -> Requires {
        Requires {
            roles: roles.iter().map(|r| r.to_string()).collect(),
            traits: traits
                .iter()
                .map(|(k, vs)| TraitEntry {
                    key: k.to_string(),
                    values: vs.iter().map(|v| v.to_string()).collect(),
                })
                .collect(),
        }
    }

    #[test]
    fn test_empty_requirements_always_pass() {
        let user = make_user(&["admin"], &[("team", &["infra"])]);
        let requires = make_requires(&[], &[]);
        assert!(user_meets_requirements(&user, &requires));
    }

    #[test]
    fn test_empty_user_with_empty_requirements() {
        let user = make_user(&[], &[]);
        let requires = make_requires(&[], &[]);
        assert!(user_meets_requirements(&user, &requires));
    }

    #[test]
    fn test_user_has_all_required_roles() {
        let user = make_user(&["admin", "editor", "viewer"], &[]);
        let requires = make_requires(&["admin", "editor"], &[]);
        assert!(user_meets_requirements(&user, &requires));
    }

    #[test]
    fn test_user_missing_one_required_role() {
        let user = make_user(&["editor", "viewer"], &[]);
        let requires = make_requires(&["admin", "editor"], &[]);
        assert!(!user_meets_requirements(&user, &requires));
    }

    #[test]
    fn test_user_has_all_required_traits() {
        let user = make_user(&[], &[("team", &["infra", "platform"]), ("org", &["eng"])]);
        let requires = make_requires(&[], &[("team", &["infra"]), ("org", &["eng"])]);
        assert!(user_meets_requirements(&user, &requires));
    }

    #[test]
    fn test_user_missing_trait_key() {
        let user = make_user(&[], &[("team", &["infra"])]);
        let requires = make_requires(&[], &[("org", &["eng"])]);
        assert!(!user_meets_requirements(&user, &requires));
    }

    #[test]
    fn test_user_missing_trait_value() {
        let user = make_user(&[], &[("team", &["infra"])]);
        let requires = make_requires(&[], &[("team", &["platform"])]);
        assert!(!user_meets_requirements(&user, &requires));
    }

    #[test]
    fn test_roles_and_traits_both_required() {
        let user = make_user(&["admin"], &[("team", &["infra"])]);
        let requires = make_requires(&["admin"], &[("team", &["infra"])]);
        assert!(user_meets_requirements(&user, &requires));
    }

    #[test]
    fn test_roles_pass_but_traits_fail() {
        let user = make_user(&["admin"], &[("team", &["infra"])]);
        let requires = make_requires(&["admin"], &[("team", &["platform"])]);
        assert!(!user_meets_requirements(&user, &requires));
    }

    #[test]
    fn test_traits_pass_but_roles_fail() {
        let user = make_user(&["viewer"], &[("team", &["infra"])]);
        let requires = make_requires(&["admin"], &[("team", &["infra"])]);
        assert!(!user_meets_requirements(&user, &requires));
    }
}
