import Lean.Data.Json
import Teleport.Types

open Lean (Json)

namespace Teleport

/-! ## JSON decoders for the differential-test corpus

The corpus schema is produced by `tool/kubeaccess-corpus/main.go`.
`FromJson`/`ToJson` deriving isn't in core Lean — these decoders are
hand-written (see workspace/research.md §2 for the rationale). -/

private def decodeList {α : Type} (f : Json → Except String α) :
    Json → Except String (List α)
  | j => do
    let arr ← j.getArr?
    arr.toList.mapM f

private def getStrField (j : Json) (key : String) : Except String String := do
  let v ← j.getObjVal? key
  v.getStr?

private def getBoolField (j : Json) (key : String) : Except String Bool := do
  let v ← j.getObjVal? key
  v.getBool?

def Verb.fromJson? : Json → Except String Verb
  | .str "get" => .ok .get
  | .str "list" => .ok .list
  | .str "watch" => .ok .watch
  | .str "create" => .ok .create
  | .str "update" => .ok .update
  | .str "delete" => .ok .delete
  | .str "patch" => .ok .patch
  | .str "deleteCollection" => .ok .deleteCollection
  | .str "*" => .ok .wildcard
  | .str s => .ok (.other s)
  | _ => .error "Verb: expected string"

def KubeResource.fromJson? (j : Json) : Except String KubeResource := do
  let kind ← getStrField j "kind"
  let ns ← getStrField j "ns"
  let name ← getStrField j "name"
  let apiGroup ← getStrField j "apiGroup"
  let verbsJ ← j.getObjVal? "verbs"
  let verbs ← decodeList Verb.fromJson? verbsJ
  return { kind, ns, name, apiGroup, verbs }

private def decodeLabelEntry (j : Json) : Except String (String × List String) := do
  let k ← getStrField j "key"
  let vsJ ← j.getObjVal? "values"
  let vs ← decodeList Json.getStr? vsJ
  return (k, vs)

private def decodeClusterLabel (j : Json) : Except String (String × String) := do
  let k ← getStrField j "key"
  let v ← getStrField j "value"
  return (k, v)

def RoleCondition.fromJson? (j : Json) : Except String RoleCondition := do
  let nsJ ← j.getObjVal? "namespaces"
  let namespaces ← decodeList Json.getStr? nsJ
  let klJ ← j.getObjVal? "kubernetesLabels"
  let kubernetesLabels ← decodeList decodeLabelEntry klJ
  let krJ ← j.getObjVal? "kubernetesResources"
  let kubernetesResources ← decodeList KubeResource.fromJson? krJ
  return { namespaces, kubernetesLabels, kubernetesResources }

def Role.fromJson? (j : Json) : Except String Role := do
  let name ← getStrField j "name"
  let allowJ ← j.getObjVal? "allow"
  let allow ← RoleCondition.fromJson? allowJ
  let denyJ ← j.getObjVal? "deny"
  let deny ← RoleCondition.fromJson? denyJ
  return { name, allow, deny }

def Cluster.fromJson? (j : Json) : Except String Cluster := do
  let name ← getStrField j "name"
  let labelsJ ← j.getObjVal? "labels"
  let labels ← decodeList decodeClusterLabel labelsJ
  let teleportNs ← getStrField j "teleportNs"
  return { name, labels, teleportNs }

def Request.fromJson? (j : Json) : Except String Request := do
  let resJ ← j.getObjVal? "resource"
  let resource ← match resJ with
    | .null => Except.ok none
    | r => (KubeResource.fromJson? r).map some
  let verbJ ← j.getObjVal? "verb"
  let verb ← Verb.fromJson? verbJ
  let isClusterWide ← getBoolField j "isClusterWide"
  return { resource, verb, isClusterWide }

def Decision.fromJson? : Json → Except String Decision
  | .str "allow" => .ok .allow
  | .str "deny" => .ok .deny
  | _ => .error "Decision: expected \"allow\" or \"deny\""

/-- A single corpus entry. `request` is `none` for cluster-level access
checks (the v0 scope). -/
structure TestCase where
  name : String
  source : String
  roles : List Role
  cluster : Cluster
  request : Option Request
  expected : Decision
  deriving Repr, Inhabited

def TestCase.fromJson? (j : Json) : Except String TestCase := do
  let name ← getStrField j "name"
  let source ← getStrField j "source"
  let rolesJ ← j.getObjVal? "roles"
  let roles ← decodeList Role.fromJson? rolesJ
  let clusterJ ← j.getObjVal? "cluster"
  let cluster ← Cluster.fromJson? clusterJ
  let reqJ ← j.getObjVal? "request"
  let request ← match reqJ with
    | .null => Except.ok none
    | r => (Request.fromJson? r).map some
  let expectedJ ← j.getObjVal? "expected"
  let expected ← Decision.fromJson? expectedJ
  return { name, source, roles, cluster, request, expected }

/-- Decode a full corpus `{cases: [...]}` into a list of `TestCase`. -/
def decodeCorpus (input : String) : Except String (List TestCase) := do
  let j ← Json.parse input
  let casesJ ← j.getObjVal? "cases"
  decodeList TestCase.fromJson? casesJ

end Teleport

section Tests
open Teleport

-- Smoke: decode an empty corpus.
#guard (Teleport.decodeCorpus "{\"cases\":[]}").isOk

-- Smoke: decode a single minimal case.
private def tinyCorpus : String :=
  "{\"cases\":[{\"name\":\"t\",\"source\":\"synthetic\",\"roles\":[],\"cluster\":{\"name\":\"c\",\"labels\":[],\"teleportNs\":\"default\"},\"request\":null,\"expected\":\"deny\"}]}"

#guard (decodeCorpus tinyCorpus).isOk

-- Decoding a malformed string surfaces an error, not a panic.
#guard (decodeCorpus "{").isOk == false

end Tests
