/-
  Teleport Access List Formal Verification - Theorems and Proofs

  This file contains theorems about the `user_meets_requirements` function
  that is auto-translated from Rust via Aeneas. Until the Aeneas translation
  is generated, we define an equivalent specification here and prove properties
  against it.

  After running Aeneas translation, replace the definitions below with imports
  from the generated file and adjust proofs accordingly.
-/

-- ==========================================================================
-- Data types (mirrors Rust structs / Go types)
-- ==========================================================================

/-- A trait entry: a key mapped to a list of values. -/
structure TraitEntry where
  key : String
  values : List String
  deriving Repr, BEq, DecidableEq

/-- Access list membership requirements. -/
structure Requires where
  roles : List String
  traits : List TraitEntry
  deriving Repr, BEq, DecidableEq

/-- User information for requirements checking. -/
structure UserInfo where
  roles : List String
  traits : List TraitEntry
  deriving Repr, BEq, DecidableEq

-- ==========================================================================
-- Helper functions (mirrors Rust implementation)
-- ==========================================================================

/-- Check if requirements are empty. -/
def requiresIsEmpty (req : Requires) : Bool :=
  req.roles.isEmpty && req.traits.isEmpty

/-- Check if a string is in a list. -/
def listContains (haystack : List String) (needle : String) : Bool :=
  match haystack with
  | [] => false
  | h :: t => if h == needle then true else listContains t needle

/-- Find values for a given key in a trait list. -/
def findTraitValues (traits : List TraitEntry) (key : String) : Option (List String) :=
  match traits with
  | [] => none
  | entry :: rest =>
    if entry.key == key then some entry.values
    else findTraitValues rest key

/-- Check that all elements of `needles` are in `haystack`. -/
def allContained (haystack : List String) (needles : List String) : Bool :=
  match needles with
  | [] => true
  | n :: rest => listContains haystack n && allContained haystack rest

/-- Check that all required roles are present in the user's roles. -/
def checkRoles (userRoles : List String) (requiredRoles : List String) : Bool :=
  allContained userRoles requiredRoles

/-- Check that all required traits are satisfied by the user's traits. -/
def checkTraits (userTraits : List TraitEntry) (requiredTraits : List TraitEntry) : Bool :=
  match requiredTraits with
  | [] => true
  | reqEntry :: rest =>
    match findTraitValues userTraits reqEntry.key with
    | none => false
    | some userValues =>
      allContained userValues reqEntry.values && checkTraits userTraits rest

/-- The core function: check if a user meets access list requirements.
    This mirrors `user_meets_requirements` in Rust / `UserMeetsRequirements` in Go. -/
def userMeetsRequirements (user : UserInfo) (requires_ : Requires) : Bool :=
  if requiresIsEmpty requires_ then true
  else checkRoles user.roles requires_.roles && checkTraits user.traits requires_.traits

-- ==========================================================================
-- Theorem 1: Empty requirements always pass
-- ==========================================================================

theorem empty_requires_always_passes (user : UserInfo) :
    userMeetsRequirements user ⟨[], []⟩ = true := by
  simp [userMeetsRequirements, requiresIsEmpty]

-- ==========================================================================
-- Helper lemmas
-- ==========================================================================

theorem listContains_not_found (needle : String) :
    listContains [] needle = false := by
  rfl

theorem allContained_nil (haystack : List String) :
    allContained haystack [] = true := by
  rfl

theorem allContained_cons (haystack : List String) (n : String) (rest : List String) :
    allContained haystack (n :: rest) = (listContains haystack n && allContained haystack rest) := by
  rfl

theorem checkRoles_nil (userRoles : List String) :
    checkRoles userRoles [] = true := by
  rfl

-- ==========================================================================
-- Theorem 2: If a role is missing, checkRoles fails
-- ==========================================================================

theorem allContained_false_if_missing
    (haystack : List String) (needles : List String) (needle : String)
    (h_in : needle ∈ needles)
    (h_missing : listContains haystack needle = false) :
    allContained haystack needles = false := by
  induction needles with
  | nil => contradiction
  | cons head tail ih =>
    simp [allContained]
    cases h_in with
    | head => simp [h_missing]
    | tail _ h_tail =>
      cases listContains haystack head <;> simp
      exact ih h_tail

theorem missing_role_means_check_fails
    (userRoles : List String) (role : String) (requiredRoles : List String)
    (h_in : role ∈ requiredRoles)
    (h_missing : listContains userRoles role = false) :
    checkRoles userRoles requiredRoles = false := by
  exact allContained_false_if_missing userRoles requiredRoles role h_in h_missing

-- ==========================================================================
-- Theorem 3: allContained is true when all elements are present
-- ==========================================================================

theorem allContained_true_if_all_present
    (haystack : List String) (needles : List String)
    (h : ∀ n, n ∈ needles → listContains haystack n = true) :
    allContained haystack needles = true := by
  induction needles with
  | nil => rfl
  | cons head tail ih =>
    simp [allContained]
    constructor
    · exact h head (List.Mem.head _)
    · exact ih (fun n hn => h n (List.Mem.tail _ hn))

-- ==========================================================================
-- Theorem 4: If user has all required roles and no traits required, they pass
-- ==========================================================================

theorem roles_only_pass
    (user : UserInfo) (requiredRoles : List String)
    (h_roles : ∀ r, r ∈ requiredRoles → listContains user.roles r = true) :
    userMeetsRequirements user ⟨requiredRoles, []⟩ = true := by
  simp [userMeetsRequirements, requiresIsEmpty]
  cases requiredRoles with
  | nil => left; rfl
  | cons head tail =>
    right
    simp [checkRoles, checkTraits]
    exact allContained_true_if_all_present user.roles (head :: tail) h_roles

-- ==========================================================================
-- Theorem 5: Determinism - same inputs always produce same output
-- This is trivially true for pure functions in Lean4 but worth stating
-- ==========================================================================

theorem deterministic (user : UserInfo) (req : Requires) :
    userMeetsRequirements user req = userMeetsRequirements user req := by
  rfl

-- ==========================================================================
-- Theorem 6: Monotonicity - adding a role to the user preserves listContains
-- ==========================================================================

theorem listContains_cons_preserved (haystack : List String) (newItem needle : String) :
    listContains haystack needle = true →
    listContains (newItem :: haystack) needle = true := by
  intro h
  unfold listContains
  split
  · rfl
  · exact h

-- ==========================================================================
-- Executable tests (sanity checks via #eval)
-- ==========================================================================

-- Test 1: Empty requirements
#eval userMeetsRequirements
  ⟨["admin"], [⟨"team", ["infra"]⟩]⟩
  ⟨[], []⟩
-- Expected: true

-- Test 2: User has required role
#eval userMeetsRequirements
  ⟨["admin", "editor"], []⟩
  ⟨["admin"], []⟩
-- Expected: true

-- Test 3: User missing required role
#eval userMeetsRequirements
  ⟨["viewer"], []⟩
  ⟨["admin"], []⟩
-- Expected: false

-- Test 4: User has required traits
#eval userMeetsRequirements
  ⟨[], [⟨"team", ["infra", "platform"]⟩]⟩
  ⟨[], [⟨"team", ["infra"]⟩]⟩
-- Expected: true

-- Test 5: User missing trait value
#eval userMeetsRequirements
  ⟨[], [⟨"team", ["infra"]⟩]⟩
  ⟨[], [⟨"team", ["platform"]⟩]⟩
-- Expected: false

-- Test 6: Both roles and traits
#eval userMeetsRequirements
  ⟨["admin"], [⟨"team", ["infra"]⟩]⟩
  ⟨["admin"], [⟨"team", ["infra"]⟩]⟩
-- Expected: true
