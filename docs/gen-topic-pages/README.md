# Topic page generator

The topic page generator creates an "All Topics" partial that lists all docs
guides in a user-specified directory tree.

## Usage examples

Create a partial called `docs/pages/includes/topic-pages/management.mdx` based
on the directory tree rooted at `docs/pages/management`:

```
$ node docs/gen-topic-pages/index.js --in docs/pages/management --out docs/pages/includes/topic-pages
```

We can then include this partial in `docs/pages/management/all-topics.mdx`.

## Flags

To use the script, run the script at `index.js` with two flags:

|Flag|Description|
|---|---|
|`in`|Comma-separated list of root directory paths from which to generate topic page partials. We expect each root directory to include the output in a page called `all-topics.mdx`|
|`out`|Relative path to a directory in which to place topic page partials, which are named after their corresponding root input directories. For example, use `docs/pages/includes/topic-pages.`|

The script generates a partial for each directory tree passed to the `in` flag.
Each partial lists the MDX files in each directory level as table rows under a
heading within the table of contents.

## Assumptions

The script makes the following assumptions about the docs directory tree:

- Each MDX page includes frontmatter with keys `title` and `description`. The
  script uses this information to populate the table of contents.
- Each subdirectory of the docs has a menu page, named after the subdirectory,
  that provides information about the subdirectory. For example, a subdirectory
  called `guides` would have a menu page called `guides.mdx` at the same
  directory level.
- A page called `all-topics.mdx` at each root directory includes the output of
  the script. The script does _not_ create links to pages called
  `all-topics.mdx`.
