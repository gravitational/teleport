namespace Teleport

/-- Kubernetes verbs recognized by Teleport RBAC. `other` is a fallthrough
for any verb string that isn't a well-known constant. -/
inductive Verb where
  | get
  | list
  | watch
  | create
  | update
  | delete
  | patch
  | deleteCollection
  | wildcard
  | other (s : String)
  deriving Repr, Inhabited, BEq, DecidableEq

/-- A single Kubernetes resource matcher entry. `ns` is the Kubernetes
namespace field (named `Namespace` in Go; renamed here because `namespace`
is a Lean keyword). -/
structure KubeResource where
  kind : String
  ns : String
  name : String
  apiGroup : String
  verbs : List Verb
  deriving Repr, Inhabited, BEq, DecidableEq

/-- Label selector: a list of `(key, allowedValues)` pairs. A value of
`"*"` inside `allowedValues` is a wildcard. -/
abbrev LabelSelector := List (String × List String)

structure RoleCondition where
  namespaces : List String
  kubernetesLabels : LabelSelector
  kubernetesResources : List KubeResource
  deriving Repr, Inhabited, BEq, DecidableEq

structure Role where
  name : String
  allow : RoleCondition
  deny : RoleCondition
  deriving Repr, Inhabited, BEq, DecidableEq

abbrev RoleSet := List Role

/-- A Kubernetes cluster with its labels. `teleportNs` corresponds to
`apidefaults.Namespace` in Go (typically `"default"`). -/
structure Cluster where
  name : String
  labels : List (String × String)
  teleportNs : String
  deriving Repr, Inhabited, BEq, DecidableEq

/-- A pending access request. `resource` is `none` for cluster-level
access checks (the v0 scope); it carries a resource descriptor for
pod/resource-level checks if those are later added. -/
structure Request where
  resource : Option KubeResource
  verb : Verb
  isClusterWide : Bool
  deriving Repr, Inhabited, BEq, DecidableEq

inductive Decision where
  | allow
  | deny
  deriving Repr, Inhabited, BEq, DecidableEq

end Teleport
