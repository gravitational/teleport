Pretty Printing for Go
======================

[![godebug build status][ciimg]][ci]

Have you ever wanted to get a pretty-printed version of a Go data structure,
complete with indentation?  I have found this especially useful in unit tests
and in debugging my code, and thus godebug was born!

[ciimg]: https://travis-ci.org/kylelemons/godebug.svg?branch=master
[ci]:    https://travis-ci.org/kylelemons/godebug

Quick Examples
--------------

By default, pretty will write out a very compact representation of a data structure.
From the [Print example][printex]:

```
{Name:     "Spaceship Heart of Gold",
 Crew:     {Arthur Dent:       "Along for the Ride",
            Ford Prefect:      "A Hoopy Frood",
            Trillian:          "Human",
            Zaphod Beeblebrox: "Galactic President"},
 Androids: 1,
 Stolen:   true}
```

It can also produce a much more verbose, one-item-per-line representation suitable for
[computing diffs][diffex].  See the documentation for more examples and customization.

[printex]: https://godoc.org/github.com/kylelemons/godebug/pretty#example-Print
[diffex]:  https://godoc.org/github.com/kylelemons/godebug/pretty#example-Compare

Documentation
-------------

Documentation for this package is available at [godoc.org][doc]:

 * Pretty: [![godoc for godebug/pretty][prettyimg]][prettydoc]
 * Diff:   [![godoc for godebug/diff][diffimg]][diffdoc]

[doc]:       https://godoc.org/
[prettyimg]: https://godoc.org/github.com/kylelemons/godebug/pretty?status.png
[prettydoc]: https://godoc.org/github.com/kylelemons/godebug/pretty
[diffimg]:   https://godoc.org/github.com/kylelemons/godebug/diff?status.png
[diffdoc]:   https://godoc.org/github.com/kylelemons/godebug/diff

Installation
------------

These packages are available via `go get`:

```bash
$ go get -u github.com/kylelemons/godebug/{pretty,diff}
```

Other Packages
--------------

If `godebug/pretty` is not granular enough, I highly recommend
checking out [go-spew][spew].

[spew]: http://godoc.org/github.com/davecgh/go-spew/spew
