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

end Teleport
