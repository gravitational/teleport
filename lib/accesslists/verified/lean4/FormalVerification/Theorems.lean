/-
  Teleport Access List Formal Verification - Theorems and Proofs

  This file contains theorems about the `user_meets_requirements` function
  that was auto-translated from Rust via Aeneas. The function definitions
  live in AccesslistVerify.lean (generated, do not edit).
-/
import FormalVerification.AccesslistVerify

open Aeneas Aeneas.Std Result ControlFlow Error
open accesslist_verify

-- ==========================================================================
-- Theorem 1: Empty requirements always pass
--
-- If both roles and traits vectors are empty, user_meets_requirements
-- returns ok true for any user.
-- ==========================================================================

-- We need an axiom about Vec.is_empty on empty vectors.
-- Aeneas models Vec.is_empty as an opaque axiom, so we state the
-- expected behavior for empty vectors.
axiom vec_is_empty_nil {T : Type} (A : Type) :
  alloc.vec.Vec.is_empty A (alloc.vec.Vec.new T) = ok true

axiom vec_is_empty_push {T : Type} (A : Type) (v : alloc.vec.Vec T) (x : T) :
  ∃ v', alloc.vec.Vec.push v x = ok v' ∧
  alloc.vec.Vec.is_empty A v' = ok false

theorem empty_requires_always_passes (user : UserInfo) :
    user_meets_requirements user ⟨alloc.vec.Vec.new String, alloc.vec.Vec.new TraitEntry⟩ = ok true := by
  simp [user_meets_requirements, requires_is_empty]
  simp [vec_is_empty_nil]

-- ==========================================================================
-- Theorem 2: Determinism - same inputs always produce same output
--
-- This is trivially true for pure functions in Lean4 but worth stating
-- explicitly: the access list check is deterministic.
-- ==========================================================================

theorem deterministic (user : UserInfo) (req : Requires) :
    user_meets_requirements user req = user_meets_requirements user req := by
  rfl

-- ==========================================================================
-- Theorem 3: If requires_is_empty returns true, user_meets_requirements
-- returns true regardless of user
-- ==========================================================================

theorem empty_check_implies_pass (user : UserInfo) (req : Requires)
    (h : requires_is_empty req = ok true) :
    user_meets_requirements user req = ok true := by
  simp [user_meets_requirements, h]

-- ==========================================================================
-- Theorem 4: If role check fails, the whole check fails
--
-- When requires_is_empty returns false and check_roles returns false,
-- user_meets_requirements returns false.
-- ==========================================================================

theorem role_check_fails_means_overall_fails
    (user : UserInfo) (req : Requires)
    (h_nonempty : requires_is_empty req = ok false)
    (h_roles : check_roles (alloc.vec.Vec.deref user.roles) (alloc.vec.Vec.deref req.roles) = ok false) :
    user_meets_requirements user req = ok false := by
  simp [user_meets_requirements, h_nonempty, h_roles]

-- ==========================================================================
-- Theorem 5: If role check passes but trait check fails, overall fails
-- ==========================================================================

theorem trait_check_fails_means_overall_fails
    (user : UserInfo) (req : Requires)
    (h_nonempty : requires_is_empty req = ok false)
    (h_roles : check_roles (alloc.vec.Vec.deref user.roles) (alloc.vec.Vec.deref req.roles) = ok true)
    (h_traits : check_traits (alloc.vec.Vec.deref user.traits) (alloc.vec.Vec.deref req.traits) = ok false) :
    user_meets_requirements user req = ok false := by
  simp [user_meets_requirements, h_nonempty, h_roles, h_traits]

-- ==========================================================================
-- Theorem 6: If both role and trait checks pass, overall passes
-- ==========================================================================

theorem both_checks_pass_means_overall_passes
    (user : UserInfo) (req : Requires)
    (h_nonempty : requires_is_empty req = ok false)
    (h_roles : check_roles (alloc.vec.Vec.deref user.roles) (alloc.vec.Vec.deref req.roles) = ok true)
    (h_traits : check_traits (alloc.vec.Vec.deref user.traits) (alloc.vec.Vec.deref req.traits) = ok true) :
    user_meets_requirements user req = ok true := by
  simp [user_meets_requirements, h_nonempty, h_roles, h_traits]

-- ==========================================================================
-- Theorem 7: user_meets_requirements is fully characterized by the three
-- sub-checks (requires_is_empty, check_roles, check_traits)
--
-- This is the key compositionality theorem: the overall result is
-- determined entirely by the results of the sub-checks.
-- ==========================================================================

theorem user_meets_requirements_decomposition
    (user : UserInfo) (req : Requires)
    (b_empty : Bool) (h_empty : requires_is_empty req = ok b_empty) :
    (b_empty = true → user_meets_requirements user req = ok true) ∧
    (b_empty = false →
      ∀ (b_roles : Bool),
        check_roles (alloc.vec.Vec.deref user.roles) (alloc.vec.Vec.deref req.roles) = ok b_roles →
        (b_roles = false → user_meets_requirements user req = ok false) ∧
        (b_roles = true →
          ∀ (b_traits : Bool),
            check_traits (alloc.vec.Vec.deref user.traits) (alloc.vec.Vec.deref req.traits) = ok b_traits →
            user_meets_requirements user req = ok b_traits)) := by
  constructor
  · intro h_true
    subst h_true
    simp [user_meets_requirements, h_empty]
  · intro h_false
    subst h_false
    intro b_roles h_roles
    constructor
    · intro h_rf
      subst h_rf
      simp [user_meets_requirements, h_empty, h_roles]
    · intro h_rt
      subst h_rt
      intro b_traits h_traits
      simp [user_meets_requirements, h_empty, h_roles, h_traits]
