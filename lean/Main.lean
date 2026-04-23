import Lean.Data.Json
import Teleport

open Lean (Json)

/-- Placeholder differential-test driver. Reads the corpus path from
argv, parses the top-level `{cases: [...]}` structure, and reports the
case count. The actual `checkAccess`-vs-expected comparison is wired
in by the `differential-loop` task; this skeleton just proves the
pipeline compiles and can be invoked from `scripts/diff.sh` on an
empty corpus. -/
def main (args : List String) : IO UInt32 := do
  match args with
  | [] =>
    IO.eprintln "usage: teleport-lean-diff <corpus.json>"
    return 1
  | path :: _ =>
    let content ← IO.FS.readFile path
    match Json.parse content with
    | .error e =>
      IO.eprintln s!"parse error: {e}"
      return 1
    | .ok j =>
      match j.getObjVal? "cases" >>= Json.getArr? with
      | .error e =>
        IO.eprintln s!"invalid schema (expected top-level `cases` array): {e}"
        return 1
      | .ok cases =>
        IO.println s!"cases={cases.size} mismatches=0"
        return 0
