# Redirect checker

This is a tool to check for URL paths in the Teleport docs site
(`https://goteleport.com/docs/`) that do not correspond to actual docs pages or
redirects. These URLs are most likely links that will 404 if a user follows
them.

## Usage:

Example of running the program from the root of a `gravitational/teleport` clone
to check for mentions of docs URLs in `gravitational/blog`:

```bash
$ node docs/check-redirects/index.js --in ~/Documents/blog/pages --docs . --name "gravitational/blog" --config docs/config.json
```

## Command-line flags

```
  --version  Show version number                                       [boolean]
  --in       root directory path in which to check for necessary redirects.
                                                                      [required]
  --config   path to a docs configuration file with a "redirects" key [required]
  --docs     path to the root of a gravitational/teleport repo        [required]
  --exclude  comma-separated list of file extensions not to check, e.g., ".md"
             or ".test.tsx"
  --name     name of the directory tree we are checking for docs URLs (for
             display only)                                            [required]
  --help     Show help                                                 [boolean]
```
