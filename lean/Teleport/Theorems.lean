import Teleport.CheckAccess

namespace Teleport

/-! ## Foundational theorems

Each theorem is expected to be a one- or two-line `simp` proof given the
deliberately simple shape of `checkAccess`. If a proof grows beyond a
handful of tactic lines, that is a signal to revisit the definitions. -/

/-- **T1** — Empty role set always denies. -/
theorem empty_denies (c : Cluster) (r : Request) :
    checkAccess [] c r = .deny := by
  simp [checkAccess]

/-- **T2** — If any role's deny condition matches, the decision is deny,
regardless of allow rules. -/
theorem deny_dominates (rs : RoleSet) (c : Cluster) (r : Request)
    (h : rs.any (fun role => denyMatches role c r) = true) :
    checkAccess rs c r = .deny := by
  simp [checkAccess, h]

/-- **T3** — Prepending a role whose deny matches forces deny, regardless
of the tail. Follows from T2. -/
theorem adding_matching_deny_denies (rs : RoleSet) (role : Role)
    (c : Cluster) (r : Request) (h : denyMatches role c r = true) :
    checkAccess (role :: rs) c r = .deny := by
  simp [checkAccess, List.any_cons, h]

/-- **T9** — The proposition `checkAccess … = .allow` is decidable.
Automatic from `Decision`'s `DecidableEq` instance. -/
example (rs : RoleSet) (c : Cluster) (r : Request) :
    Decidable (checkAccess rs c r = .allow) :=
  inferInstance

/-! ## Allow/deny interaction theorems

Helper characterization: `checkAccess` returns `.allow` iff no deny
matches and some allow matches. This is the workhorse lemma that makes
T4, T5, T7 one-liners. -/

private theorem checkAccess_eq_allow_iff (rs : RoleSet) (c : Cluster) (r : Request) :
    checkAccess rs c r = .allow ↔
    rs.any (fun role => denyMatches role c r) = false ∧
    rs.any (fun role => allowMatches role c r) = true := by
  unfold checkAccess
  cases h1 : rs.any (fun role => denyMatches role c r) <;>
    cases h2 : rs.any (fun role => allowMatches role c r) <;>
    simp

private theorem checkAccess_eq_deny_iff (rs : RoleSet) (c : Cluster) (r : Request) :
    checkAccess rs c r = .deny ↔
    rs.any (fun role => denyMatches role c r) = true ∨
    rs.any (fun role => allowMatches role c r) = false := by
  unfold checkAccess
  cases h1 : rs.any (fun role => denyMatches role c r) <;>
    cases h2 : rs.any (fun role => allowMatches role c r) <;>
    simp

/-- **T4** — Adding a role whose deny does not match preserves any
existing `.allow` verdict. -/
theorem allow_preserved_when_new_role_has_no_deny_match
    (rs : RoleSet) (newRole : Role) (c : Cluster) (r : Request)
    (hPrev : checkAccess rs c r = .allow)
    (hNoDeny : denyMatches newRole c r = false) :
    checkAccess (newRole :: rs) c r = .allow := by
  rw [checkAccess_eq_allow_iff] at hPrev ⊢
  refine ⟨?_, ?_⟩
  · simp [List.any_cons, hNoDeny, hPrev.1]
  · simp [List.any_cons, hPrev.2]

/-- **T5** — Removing a role that did not contribute a deny cannot turn
a `.deny` verdict into `.allow`. Dual of T4. -/
theorem removing_allow_cannot_grant
    (rs : RoleSet) (role : Role) (c : Cluster) (r : Request)
    (hPrev : checkAccess (role :: rs) c r = .deny)
    (hNoDeny : denyMatches role c r = false) :
    checkAccess rs c r = .deny := by
  rw [checkAccess_eq_deny_iff] at hPrev ⊢
  rw [List.any_cons, hNoDeny, Bool.false_or, List.any_cons] at hPrev
  rcases hPrev with hD | hA
  · exact Or.inl hD
  · rw [Bool.or_eq_false_iff] at hA
    exact Or.inr hA.2

/-- **T7** — A role whose allow labels are empty never grants access
via the allow branch. Guards against the "empty = wildcard" bug. -/
theorem empty_allow_labels_never_grants
    (role : Role) (c : Cluster) (r : Request)
    (h : role.allow.kubernetesLabels = []) :
    allowMatches role c r = false := by
  simp [allowMatches, effectiveLabels, h, labelMatch]

/-- **T8** — The implicit-wildcard injection in `effectiveLabels` fires
exactly when deny labels are empty and deny's resource list is non-empty. -/
theorem implicit_wildcard_deny_encoded
    (cond : RoleCondition)
    (hLabels : cond.kubernetesLabels = [])
    (hRes : cond.kubernetesResources ≠ []) :
    effectiveLabels cond true = [("*", ["*"])] := by
  unfold effectiveLabels
  rw [hLabels]
  cases hRes' : cond.kubernetesResources with
  | nil => exact absurd hRes' hRes
  | cons _ _ => simp

/-- **T10** — Duplicate roles in the set do not change the decision.
Follows from the fact that `any` is idempotent on duplicates. -/
theorem duplicate_roles_idempotent (role : Role) (rs : RoleSet)
    (c : Cluster) (r : Request) :
    checkAccess (role :: role :: rs) c r = checkAccess (role :: rs) c r := by
  simp [checkAccess, List.any_cons]

-- T6 (wildcard_allow_grants) is deferred: existential setup (role-in-roleset)
-- plus matcher unfolding makes it meaningfully harder than T4/T5. The
-- essential intuition is that a wildcard-allow role makes allowMatches
-- true and the any-over-roleset is therefore true. Revisit in v1 once
-- auxiliary lemmas about List.any membership are in place.

end Teleport
