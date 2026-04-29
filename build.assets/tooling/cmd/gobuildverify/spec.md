# gobuildverify

gobuildverify is a tool that verifies that a Go binary has required build
settings set on it. The output of `go version -m -json` has a section named
`Settings` that lists all the build settings that were set for a binary. These
are availble programmatically via the `debug/buildinfo` package.

```
gobuildverify -- '-tags=(fips)' 'DefaultGODEBUG=(/fips140=on(ly)?/)' 'GOFIPS140=/^v1\.0\.0/'
```

Each argument is an expression of the form `A[=B]`. `B` can be absent, a string,
a string surrounded by slashes, or a string surrounded by parentheses, which are
interpreted as follows:
* Presence: Of the form `A`, the expression checks for the presence of the build
  setting `A`.
* Exact match: Of the form `A=B` Checks that the setting `A` has the exact value
  `B`,
* Regexp match: Of the form `A=/B/`. Treats `B` as a regular expression (RE2)
  checks that the setting `A` has a value that matches the regexp `B` with
  `regexp.MatchString`,
* List matc: Of the form `A=(B)`. The value of the `A` setting is treated as a
  comma-separated list and `B` is matched against each element. The part of `B`
  inside the parens is a match string that matches using these rules. e.g.
  `A=(/B/)` does a regexp match of `B` against each list element of the `A`
  setting. `A=(B)` does an exact match against each of the list elements.

The standard `--` argument means end-of-flags, and is needed as some of the
build settings start with a hyphen and would otherwise be interpreted as flags
to `gobuildverify`. Any argument after `--` on the command line is treated as a
non-flag argument. `--` is optional but required if any expressions start with a
hyphen. Such expressions must appear after `--` on the command line. Currently
there are no supported flags but there may be in future.

If any of the expressions fail to match, `gobuildverify` prints an error message
for each expression that does not match on a separate line and exits with a
value of 1. If all expressions match, `gobuildverify` exits with a value of 0
and prints nothing.

If a setting is not present, the error message will say:

    Build setting not present: <setting>

If the expression has a match value that is a string or a regex and it does not
match the build setting in the binary, the error message will say:

    Build setting value does not match: <setting>: <actual> != <want>

If the expression is a list match and none of the elements match, the error
message will say:

    Build setting does not contain value: <setting>=<list> : <want>
