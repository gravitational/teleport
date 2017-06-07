# Teleport Enterprise

This repository keeps the proprietary bits of Teleport enterprise. It is not
meant to exist independently, it is meant to be cloned into `e` subdirectory 
of Teleport repo as a git submodule.

The filesystem layout of Teleport source code:

```
$GOHOME/gravitational/teleport  <-- open source public Teleport repo
   |
   |- lib
   |- tool
   |- web
   |- e                         <-- this repo cloned into 'e'
```

## Getting Started

By default, when you clone OSS Teleport, the submodules are not cloned:

```
$ git@github.com:gravitational/teleport.git
```

This will create a regular OSS Teleport repo, so any Github user can clone,
compile, modify and send pull requests.

If you want to start working on Enterprise, you need to initialize the
enterprise submodule:

```
$ git@github.com:gravitational/teleport.git
$ git submodule init e
$ git submodule update --remote
```

This will populate `e` subdirectory.

## Submodules

Read these two articles and you'll be fine:

* [Introduction to Submodules](https://git-scm.com/book/en/v2/Git-Tools-Submodules)
* [Submodules Reference](https://git-scm.com/docs/git-submodule)

## Branching

The parent OSS repository always "knows" which branch of the `e` submodule is
current.  Say, you want to introduce a new feature which spans across both
repositories, so ideally you'd like to keep it in your own "safe place". So how
do you create such "safe place"? Usually you would just create a branch and
work there, how would that work with two repositories when one of them is a
submodule of another?

Lets use "Ev wants to add feature foo" scenario as an example. 
Here's the sequence of steps:

1. Create branch `ev/foo` in OSS Teleport.
2. Make OSS changes in that branch (maybe edit README.md file)
3. Create branch `ev/foo` in `e` submodule via `cd e; git checkout -b ev/foo`.
4. Work in `e` implementing the enterprise side of "foo".

Now, if you type `git status` in the Teleport root directory, you will see
something like this:

```
$ git status
    modified:   README.md
    modified:   e (modified content)
```

It is telling you that you've changed README.md in the OSS branch and something
else inside of `e` submodule.
   
