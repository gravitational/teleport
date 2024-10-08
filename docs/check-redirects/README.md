# Redirect checker

This is a tool to check for out-of-date URL paths to pages on the Teleport docs
site. You can use it to identify 404ing links in the Teleport Web UI source,
`gravitational/blog`, and `gravitational/next` repositories. The tool identifies
URLs in the target directory that do not correspond to docs page files or
redirects in the `gravitational/teleport` repository.

## Usage

The following example checks for `https://goteleport.com/docs` URLs in a
`gravitational/blog` clone. The `--in` flag points to the directory that
contains blog pages (the clone itself is at `~/Documents/blog`). Our
`gravitational/teleport` clone is at `~/Documents/docs/content/16.x`.
The commands are run from `docs/check-redirects`.

```bash
pnpm install && \
pnpm check-redirects --in ~/Documents/blog/pages --docs ~/Documents/docs/content/16.x --name "the Teleport blog" --config ~/Documents/docs/content/16.x/docs/config.json
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
