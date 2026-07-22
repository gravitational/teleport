# gobuildverify

gobuildverify is a tool that verifies that a Go binary has required build
settings. The output of `go version -m -json` has a section named `Settings`
that lists all the build settings that were set for a binary. These are
available programmatically via the `debug/buildinfo` package.

The build setting values embedded in a Go binary are strings. In some cases,
the strings are comma-separated lists. `gobuildverify` expressions can match
against individual strings or against individual elements of a comma-separated
list. A match may be an exact string match or a RE2 regular expression match.

For example:
```
gobuildverify binary '-tags=(fips)' 'DefaultGODEBUG=(/fips140=on(ly)?/)' 'GOFIPS140=/^v1\.0\.0/'
```

This will verify that `binary` has:
* a build setting named `-tags` that is a comma-separated list and one of the elements
  has the value `fips`,
* a build setting named `DefaultGODEBUG` that is a comma-separated list and one
  of the elements matches the regular expression `fips140=on(ly)?`,
* a build setting named `GOFIPS140` that is a string that matches the regular
  expression `^v1\.0\.0`.

If all three expressions match, then `gobuildverify` will exit with success (0).
Otherwise it will print an appropriate error message saying what did not match
and exit with failure (1).

Each argument is an expression of the form `A[=B]`. `B` can be absent, a
string, a string surrounded by slashes, or a string surrounded by parentheses.
These four forms are interpreted as follows:
* Presence: Of the form `A`, checks for the presence of the build setting `A`,
* Exact match: Of the form `A=B`, checks that the setting `A` has the exact
  value `B`,
* Regexp match: Of the form `A=/B/`, treats `B` as a regular expression (RE2)
  and checks that the setting `A` has a value that matches the regexp `B` with
  `regexp.MatchString` (which is an unanchored match),
* List contains match: Of the form `A=(B)`, treats the value of the `A` setting
  as a comma-separated list and matches `B` against each element. The contents
  inside the parens is itself a match expression evaluated by the rules above.
  For example, `A=(/B/)` does a regexp match of `B` against each list element
  of the `A` setting. `A=(B)` does an exact match against each of the list
  elements.

If a setting is not present, the error message will say:

    Build setting not present: <setting>

If the expression has a match value that is a string or a regex and it does not
match the build setting in the binary, the error message will say:

    Build setting value does not match: <setting>: <actual> != <want>

If the expression is a list match and none of the elements match, the error
message will say:

    Build setting does not contain value: <setting>: <want> not in <list>

