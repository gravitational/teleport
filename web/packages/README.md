# Packages

This directory contains Gravitational npm packages. A package can be
a stand-alone web application or library referenced by other packages.

## Description

|Package   | Description  |
|---|---|
|`teleport`| Open-source version of Gravitational Teleport Web UI |
|`gravity`|  Open-source version of Gravitational Gravity Web UI   |
|`build`| Collection of webpack and build scripts used to build Gravitational packages |
|`design`| Gravitational Design-System  |
|`shared`| Shared code |

## Adding a new package

If you want to add a new project or package into this directory,
please read this article about
[yarn workspaces](https://yarnpkg.com/blog/2017/08/02/introducing-workspaces/)

## Building

You can run build scripts from this repository root folder

```
$ cd webapps/
$ yarn workspace @gravitational/your_package_name run build
```

or if you are working directly on the package, you can run it inside the
package folder

```
$ cd webapps/packages/mypackage
$ yarn run build
```