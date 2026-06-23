import Teleport.Types

namespace Teleport

/-- Glob-match with `*` as the only metacharacter. Anchored (matches the
entire string). Mirrors Go's `GlobToRegexp` followed by an anchored
`MatchString`: every character except `*` is literal (including `.`, `+`,
`?`, `$`, etc.). Pattern `"*"` matches any string including empty. -/
def globMatch (pat : String) (s : String) : Bool :=
  go pat.toList s.toList
where
  go : List Char → List Char → Bool
    | [], [] => true
    | [], _ :: _ => false
    | '*' :: ps, [] => go ps []
    | '*' :: ps, sc :: ss =>
        go ps (sc :: ss) || go ('*' :: ps) ss
    | _ :: _, [] => false
    | pc :: ps, sc :: ss => pc == sc && go ps ss
  termination_by p s => p.length + s.length

/-- Assoc-list lookup by string key. Returns the value bound to the
first matching key (if any). -/
def assocLookup (key : String) : List (String × α) → Option α
  | [] => none
  | (k, v) :: rest => if k == key then some v else assocLookup key rest

/-- Label-selector match. Mirrors `MatchLabelGetter` in
`lib/services/role.go:1221-1281`:
- empty selector matches nothing;
- selector `[("*", ["*"])]` matches anything;
- otherwise, every selector key must be present in the target with a
  value that either appears literally in the selector's value list,
  is glob-matched by some pattern in that list, or the value list
  contains `"*"`. -/
def labelMatch (sel : LabelSelector) (target : List (String × String)) : Bool :=
  if sel.isEmpty then
    false
  else if sel == [("*", ["*"])] then
    true
  else
    sel.all fun entry =>
      let (k, vs) := entry
      match assocLookup k target with
      | none => false
      | some tv => vs.contains "*" || vs.any (fun v => globMatch v tv)

/-- True iff any pattern in `pats` glob-matches `ns`. -/
def namespaceMatch (pats : List String) (ns : String) : Bool :=
  pats.any (fun p => globMatch p ns)

/-- True iff the verb list contains `wildcard` or the exact verb `v`. -/
def verbAllowed (verbs : List Verb) (v : Verb) : Bool :=
  verbs.contains Verb.wildcard || verbs.contains v

/-- Match a resource request against a list of resource matchers.
Mirrors `KubeResourceMatchesRegex` in `lib/utils/replace.go:162-257`
with the read-only-namespace branch intentionally omitted (dead in
the v0 oracle per workspace/research.md §5). `isDeny` is retained
for API stability; currently unused. -/
def kubeResourceMatch (req : KubeResource) (isDeny : Bool)
    (resources : List KubeResource) : Bool :=
  let _ := isDeny
  resources.any fun r =>
    globMatch r.kind req.kind &&
    globMatch r.ns req.ns &&
    globMatch r.name req.name &&
    globMatch r.apiGroup req.apiGroup &&
    req.verbs.all (fun v => verbAllowed r.verbs v)

end Teleport

section Tests
open Teleport

-- globMatch edge cases
#guard globMatch "*" ""
#guard globMatch "*" "anything"
#guard globMatch "*.lean" "Main.lean"
#guard globMatch "1.2.*" "1.2.3"
#guard !globMatch "1.2.*" "1.2"           -- literal `.` required after "1.2"
#guard globMatch "foo" "foo"
#guard !globMatch "foo" "bar"
#guard !globMatch "foo" "foox"
#guard globMatch "foo*" "foox"
#guard globMatch "foo*" "foo"
#guard !globMatch "" "x"
#guard globMatch "" ""
#guard globMatch "*bar*" "foobarbaz"

-- labelMatch
#guard !labelMatch [] []
#guard !labelMatch [] [("k", "v")]
#guard labelMatch [("*", ["*"])] []
#guard labelMatch [("*", ["*"])] [("k", "v")]
#guard labelMatch [("k", ["v"])] [("k", "v")]
#guard !labelMatch [("k", ["v"])] [("k", "w")]
#guard !labelMatch [("k", ["v"])] [("other", "v")]
#guard labelMatch [("k", ["*"])] [("k", "anything")]
#guard labelMatch [("k", ["v", "w"])] [("k", "w")]
#guard labelMatch [("k", ["pre*"])] [("k", "prefix")]
#guard !labelMatch [("k", ["pre*"])] [("k", "other")]

-- multi-key selector: ALL keys must be present
#guard labelMatch [("a", ["1"]), ("b", ["2"])] [("a", "1"), ("b", "2")]
#guard !labelMatch [("a", ["1"]), ("b", ["2"])] [("a", "1")]

-- namespaceMatch
#guard namespaceMatch ["*"] "default"
#guard namespaceMatch ["default"] "default"
#guard !namespaceMatch ["prod"] "default"
#guard !namespaceMatch [] "default"
#guard namespaceMatch ["prod", "*"] "default"

-- verbAllowed
#guard verbAllowed [Verb.wildcard] Verb.get
#guard verbAllowed [Verb.wildcard] Verb.delete
#guard verbAllowed [Verb.get, Verb.list] Verb.list
#guard !verbAllowed [Verb.get] Verb.delete
#guard !verbAllowed [] Verb.get

end Tests
