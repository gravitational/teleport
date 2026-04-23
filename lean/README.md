# Lean formal model of Teleport Kubernetes RBAC (v0)

A small Lean 4 model of `RoleSet.checkAccess` for cluster-level
Kubernetes access, kept in sync with Go via a JSON corpus.

Two artifacts:

1. **Spec.** `checkAccess` and helpers encode the decision semantics.
   Proven theorems in `Teleport/Theorems.lean` state invariants.
2. **Drift detector.** `scripts/diff.sh` regenerates a corpus from Go,
   runs Lean over it, and fails on divergence. Go is always the oracle.

## Modeled

Cluster-level `CheckAccess` as exercised by `TestCheckAccessToKubernetes`
(`lib/services/role_test.go:6828`). The 8 organic cases appear as
`organic/*` entries; a synthetic generator adds ~96 combinatorial cases
(7 role templates × 6 cluster label sets, plus 2- and 3-role sets).

## Not modeled (silent on regressions in these paths)

- Trait interpolation — `lib/services/role.go:497-650`
- Label expressions — `lib/services/role.go:9712`
- MFA / device trust (`AccessState`) — `lib/services/role.go:2850`
- `kubernetes_users` / `kubernetes_groups` (identity mapping)
- Regex beyond glob — `lib/utils/replace.go:405-431`
- Read-only namespace branch — `lib/utils/replace.go:199-228`
- Role versions other than V6
- Pod/resource-level requests (`request.resource != null`)
- **Implicit-wildcard deny injection** in `NewKubernetesClusterLabelMatcher`
  (`lib/services/role.go:2665`, wired via `lib/kube/proxy/forwarder.go:1014`).
  This matcher is applied on top of `CheckAccess` in production but not
  by the v0 oracle (`TestCheckAccessToKubernetes` calls `CheckAccess`
  without matchers). Model the pure RBAC core; add the matcher layer
  separately when extending scope.

## Proved invariants

All in `Teleport/Theorems.lean`, no `sorry`.

- **T1** empty role set denies
- **T2** deny dominates
- **T3** prepending a matching-deny role forces deny
- **T4** allow is preserved when new role has no matching deny
- **T5** removing a no-deny role cannot grant access
- **T7** empty allow labels never grant
- **T9** `checkAccess = .allow` is decidable
- **T10** duplicate roles are idempotent
- **T6** (wildcard grants) — deferred
- **T8** (implicit-wildcard deny injection) — removed; see below

## Run

```sh
./scripts/diff.sh
```

Expected: `cases=104 mismatches=0`, exit 0. On mismatch, one line per
divergent case is printed and exit is 1.

Requires Go toolchain (for the corpus generator at
`tool/kubeaccess-corpus`) and Lean via `elan` (toolchain pinned in
`lean-toolchain`). The corpus JSON is `.gitignore`d — regenerated each
run.

## Maintenance

When Go semantics change: rerun `diff.sh`, inspect mismatches, update
Lean matchers — or file a Go bug if Lean turns out to be right. Extend
corpus coverage by editing `tool/kubeaccess-corpus/main.go`.

## Structure

```
lean/
  Teleport.lean                  -- library root
  Teleport/
    Types.lean                   -- Role, RoleSet, Cluster, Request, Decision
    Match.lean                   -- glob, label, namespace, resource matchers
    CheckAccess.lean             -- the decision function
    Theorems.lean                -- proven invariants
    Corpus.lean                  -- JSON decoders
  Main.lean                      -- CLI entrypoint
  scripts/diff.sh                -- regenerate + build + compare
  corpus/                        -- (gitignored) runtime artifacts
```

## Status

v0 research branch. Not merge-ready. No team commitments.
