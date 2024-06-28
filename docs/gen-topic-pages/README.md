# Topic page generator

The topic page generator automatically generates table of contents pages at each
level of a chosen directory tree. Each menu page has the same name as its parent
directory, and lists the contents of that directory within the docs.

The generator does not create a table of contents for the first level of the
chosen directory tree, since the resulting page is usually too large to be
useful.

## Usage examples

Create two menu pages for each subdirectory within the directory trees rooted at
`docs/pages/management` and `docs/pages/security`:

```bash
$ node docs/gen-topic-pages/index.js --in docs/pages/management,docs/pages/security
```

Let's assume that `docs/pages/management` contains the following subdirectories:

- `docs/pages/management/authentication`
- `docs/pages/management/authorization`

In this case, the script creates the following menu pages:

- `docs/pages/management/management.mdx`
- `docs/pages/security/security.mdx`
- `docs/pages/management/authentication/authentication.mdx`
- `docs/pages/management/authorization/authorization.mdx`

## Configuration

The generator assumes that each menu page has a comment with the following
value:

```
{/*TOPICS*/}
```

When the generator runs, it automatically adds a table of contents below the
comment. This allows authors to include introductory text before the list of
links. The generator throws an error if there is a table of contents page
without this comment.

The script assumes that each MDX page includes frontmatter with keys `title` and
`description`. The script uses this information to populate each menu of links.

## Available flags

```
Options:
  --version  Show version number                                       [boolean]
  --in       Comma-separated list of root directory paths from which to generate
             topic pages. We expect each root directory to include the output in
             a page, within the directory, that has the directory's name"
                                                                      [required]
  --ignore   Comma-separated list of directory paths to skip when generating
             topic pages. The generator will not place a topic page within that
             directory or its children.
  --help     Show help                                                 [boolean]
```
