import Teleport.Types
import Teleport.Match

namespace Teleport

/-- A deny condition matches iff its namespace and label selectors match
the cluster. Resource-level matching is out of v0 scope (request
resources are always `none` in the corpus).

Note: production Kubernetes access (`lib/kube/proxy/forwarder.go:1014`)
layers `NewKubernetesClusterLabelMatcher` on top, which injects `{*:*}`
when deny labels are empty (`lib/services/role.go:2665`). That matcher
is not part of the pure RBAC core exercised by `TestCheckAccessToKubernetes`
and therefore not modeled here. -/
def denyMatches (role : Role) (c : Cluster) (_r : Request) : Bool :=
  namespaceMatch role.deny.namespaces c.teleportNs &&
  labelMatch role.deny.kubernetesLabels c.labels

/-- An allow condition matches iff its namespace and label selectors match. -/
def allowMatches (role : Role) (c : Cluster) (_r : Request) : Bool :=
  namespaceMatch role.allow.namespaces c.teleportNs &&
  labelMatch role.allow.kubernetesLabels c.labels

/-- Kubernetes access decision: deny dominates, then any allow grants,
else deny. Mirrors `RoleSet.checkAccess` in `lib/services/role.go:2681-2905`. -/
def checkAccess (rs : RoleSet) (c : Cluster) (r : Request) : Decision :=
  if rs.any (fun role => denyMatches role c r) then .deny
  else if rs.any (fun role => allowMatches role c r) then .allow
  else .deny

end Teleport

section Tests
open Teleport

private def emptyCond : RoleCondition :=
  { namespaces := ["default"], kubernetesLabels := [], kubernetesResources := [] }

private def wildcardAllow : RoleCondition :=
  { namespaces := ["default"], kubernetesLabels := [("*", ["*"])], kubernetesResources := [] }

private def envProdLabels : RoleCondition :=
  { namespaces := ["default"], kubernetesLabels := [("env", ["prod"])], kubernetesResources := [] }

private def otherLabels : RoleCondition :=
  { namespaces := ["default"], kubernetesLabels := [("env", ["staging"])], kubernetesResources := [] }

private def mkRole (name : String) (allow deny : RoleCondition) : Role :=
  { name, allow, deny }

private def prodCluster : Cluster :=
  { name := "prod", labels := [("env", "prod")], teleportNs := "default" }

private def req : Request :=
  { resource := none, verb := Verb.get, isClusterWide := true }

-- Empty roleset denies
#guard checkAccess [] prodCluster req == Decision.deny

-- Single wildcard allow, empty deny → allow
#guard checkAccess [mkRole "w" wildcardAllow emptyCond] prodCluster req == Decision.allow

-- Wildcard allow + matching deny → deny dominates
#guard checkAccess [mkRole "w" wildcardAllow emptyCond, mkRole "d" emptyCond envProdLabels]
  prodCluster req == Decision.deny

-- Mismatched allow → deny (no match)
#guard checkAccess [mkRole "s" otherLabels emptyCond] prodCluster req == Decision.deny

-- Single matching allow (env=prod label) → allow
#guard checkAccess [mkRole "p" envProdLabels emptyCond] prodCluster req == Decision.allow

-- Order doesn't matter: deny in first or second slot both win
#guard checkAccess [mkRole "d" emptyCond envProdLabels, mkRole "w" wildcardAllow emptyCond]
  prodCluster req == Decision.deny

end Tests
