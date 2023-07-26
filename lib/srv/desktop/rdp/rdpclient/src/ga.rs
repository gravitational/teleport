// Copyright 2023 Gravitational, Inc
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

//! This module provides a wrapper around Arc built such that the underlying
//! allocation can be passed to and from Go safely without violating Rust
//! compiler guarantees.
//!
//! The [`GoArc`] (created by [`GoArc_new`]) is a raw pointer that is
//! expected to be passed into Go and held on to for later use; in a sense
//! it's like an [`Arc`] that's owned by Go. The only way for Rust to safely make use of
//! the [`GoArc`] is to clone it into a [`Arc`], which is done by calling
//! [`GoArc_clone`]. Finally, when Go is done with the [`GoArc`], it must drop it
//! explicitly with [`GoArc_drop`].
//!
//! # Memory Management
//!
//! While being used in Rust as an [`Arc`], the underlying memory is managed
//! by standard Rust [`Arc`] semantics.
//!
//! On the other hand, the [`GoArc`] itself is owned by Go, and Go doesn't have
//! any understanding of Rust's ownership semantics, which means that it Go's
//! responsibility to explicitly drop the [`GoArc`] using [`GoArc_drop`] when it's
//! no longer needed.
//!
//! ## Memory Safety
//!
//! It's important that [`GoArc_drop`] is only called once per [`GoArc`]. Calling
//! it multiple times will result in a double free.
//!
//! Once the [`GoArc`] is dropped, it's no longer safe to call [`GoArc_clone`] on
//! it. Doing so could result in a use-after-free.
//!
//! # Thread Safety: [`Send`] + [`Sync`]
//!
//! [`GoArc_new`] enforces that the underlying type T is [`Send`] + [`Sync`].
//!
//! This constraint is enforced because the underlying memory is expected to
//! be passed back into Rust from Go via FFI compatible functions, which might
//! be called from any arbitrary thread selected by Go's runtime. By enforcing
//! [`Send`] + [`Sync`], we can guarantee that the underlying memory is always
//! safe to send to and access from any thread.
//!
//! # Example
//!
//! ```no_run
//! use ga::{GoArc_new, GoArc_clone, GoArc_drop};
//!
//! /// Imagine [`create_go_arc`] being called by CGO via the FFI.
//! /// Go holds on to the [`GoArc`] returned to it by that call,
//! /// uses it in any arbitrary goroutine(s) via [`use_go_arc`],
//! /// and then drops it when it's done with it via [`drop_go_arc`].
//!
//! /// Creates a new [`GoArc`] containing the value 42.
//! #[no_mangle]
//! pub unsafe extern "C" fn create_go_arc() -> GoArc<u32> {
//!     GoArc_new(42)
//! }
//!
//! /// Uses the given [`GoArc`] by cloning it into a [`Arc`].
//! ///
//! /// This can be called from any arbitrary goroutine(s) in Go,
//! /// so long as [`drop_go_arc`] hasn't been called yet.
//! #[no_mangle]
//! pub unsafe extern "C" fn use_go_arc(go_arc: GoArc<u32>) -> GoArc<u32> {
//!     // Clone the [`GoArc`] into a [`Arc`] so that we can
//!     // safely access the value inside.
//!     let rust_arc = GoArc_clone(go_arc);
//!
//!     // We can use the [`Arc`] just like a normal [`Arc`] here.
//!     // For example, we can directly call any non-mutating methods on
//!     // the underlying u32 via the [`Deref`] trait.
//!     let is_lt = rust_arc.lt(32);
//!     if is_lt {
//!        println!("{} is less than {}", 32, rust_arc);
//!     } else {
//!         println!("{} is not less than {}", 32, rust_arc);
//!     }
//!
//!     // We can even clone it again if we want to, pass it around
//!     // to other threads, etc.
//!     let rust_arc_clone = rust_arc.clone();
//!     std::thread::spawn(move || {
//!        println!("Hello from another thread! {}", rust_arc_clone);
//!     });
//! }
//!
//! /// Drops the given [`GoArc`].
//! ///
//! /// This must be called once and only once by Go when it's done
//! /// with the [`GoArc`]. After this is called, [`use_go_arc`] can
//! /// no longer be called.
//! #[no_mangle]
//! pub unsafe extern "C" fn drop_go_arc(go_arc: GoArc<u32>) {
//!     // Drop the [`GoArc`] explicitly.
//!     GoArc_drop(go_arc);
//! }
//!
//!
//! let go_arc = GoArc_new(42);
//! ```
use std::sync::Arc;

/// A raw pointer to an [`Arc`] that is owned by Go.
/// See the module level documentation for more details.
pub type GoArc<T> = *mut Arc<T>;

/// Creates a new [`GoArc`] containing the given value.
#[allow(non_snake_case)]
pub fn GoArc_new<T>(inner: T) -> GoArc<T>
where
    T: Send + Sync,
{
    // Create a new Send + Sync Arc
    let arc = Arc::new(inner);
    // Move it to the heap
    let boxed_arc = Box::new(arc);
    // Convert it to a raw pointer, preventing it from being dropped.
    // The Arc's count is now at 1, and will remain at at least 1
    // until GoArc_drop is called on the returned pointer.
    Box::into_raw(boxed_arc)
}

/// Clones the given [`GoArc`] into a [`Arc`].
///
/// # Safety
///
/// The given [`GoArc`] must not have been dropped yet
/// (i.e. [`GoArc_drop`] must not have been called on it).
#[allow(non_snake_case)]
pub unsafe fn GoArc_clone<T>(raw: GoArc<T>) -> Arc<T>
where
    T: Send + Sync,
{
    // Take a reference to the Arc
    let arc = &*raw;
    // Return a clone of the Arc
    arc.clone()
}

/// Drops the given [`GoArc`].
///
/// # Safety
///
/// This must be called once and only once per [`GoArc`].
/// Calling it multiple times will result in a double free.
#[allow(non_snake_case)]
pub unsafe fn GoArc_drop<T: Send + Sync>(raw: GoArc<T>) {
    // The raw pointer was created via Box::into_raw in GoArc_new,
    // so we can reconstruct it here with Box::from_raw.
    let boxed_arc = Box::from_raw(raw);
    // Drop the Box, which will drop the Arc.
    drop(boxed_arc);
}
