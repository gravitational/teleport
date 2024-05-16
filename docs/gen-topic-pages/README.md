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

Each directory in the tree has a corresponding `yaml` file with the same name as
the directory that configures the menu page for that directory. For example,
`docs/pages/management` would have a menu page called
`docs/pages/management.yaml`. 

The `yaml` file has the following fields:

- `title`: The title of the menu page.
- `description`: The description of the menu page, which the generator uses as
  the page's introductory paragraph.

The script assumes that each MDX page includes frontmatter with keys `title` and
`description`. The script uses this information to populate each menu of links.
directory level.
