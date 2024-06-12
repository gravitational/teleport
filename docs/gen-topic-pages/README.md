# Topic page generator

The topic page generator automatically generates menu pages at each level of a
chosen directory tree. Each menu page has the same name as its corresponding
directory, and lists the contents of that directory within the docs. 

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

- `docs/pages/management.mdx`
- `docs/pages/security.mdx`
- `docs/pages/management/authentication.mdx`
- `docs/pages/management/authorization.mdx`

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
