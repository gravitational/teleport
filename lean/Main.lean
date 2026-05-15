import Lean.Data.Json
import Teleport

open Teleport

private def decisionStr : Decision → String
  | .allow => "allow"
  | .deny => "deny"

/-- A placeholder request used for cluster-level cases where the corpus
sets `request: null`. The v0 matchers ignore the request, so this is
purely a syntactic stand-in. -/
private def clusterWideRequest : Request :=
  { resource := none, verb := .get, isClusterWide := true }

private def runCase (tc : Teleport.TestCase) : Option String :=
  let req := tc.request.getD clusterWideRequest
  -- Dispatch to the production decision function for cases that used the
  -- NewKubernetesClusterLabelMatcher path in Go; everything else uses the
  -- core RBAC decision.
  let actual :=
    if tc.source = "production" then
      checkAccessProduction tc.roles tc.cluster req
    else
      checkAccess tc.roles tc.cluster req
  if actual != tc.expected then
    some s!"{tc.name} | expected={decisionStr tc.expected} actual={decisionStr actual}"
  else
    none

def main (args : List String) : IO UInt32 := do
  match args with
  | [] =>
    IO.eprintln "usage: teleport-lean-diff <corpus.json>"
    return 1
  | path :: _ =>
    let content ← IO.FS.readFile path
    match decodeCorpus content with
    | .error e =>
      IO.eprintln s!"decode error: {e}"
      return 1
    | .ok cases =>
      let mismatches := cases.filterMap runCase
      for m in mismatches do
        IO.println m
      IO.println s!"cases={cases.length} mismatches={mismatches.length}"
      return if mismatches.isEmpty then 0 else 1
