# Vale style tests

This directory contains the test suite for our vale styles.

`test.json` contains a mapping of file paths, relative to `.github/vale-tests`,
to expected error substrings within each path:

```json
   "file_path.md": [
,
    "unexpected heading",
    "\"Auth Server\" is incorrect",
    "mispelled word: \"seerver\"
]
```

Run `run-vale-tests.sh` to run the test suite. This runs `vale` against each
test MD file in `.github/vale-tests`, and checks the errors it receives against
each substring in `expected_error_substrings`.
