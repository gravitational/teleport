# Packages

This directory contains Gravitational Web UI packages. A package can be
a stand-alone web application or library referenced by other packages.

## Description

|Package   | Description  |
|---|---|
|`teleport`| Open-source version of Gravitational Teleport Web UI |
|`gravity`|  Open-source version of Gravitational Gravity Web UI   |
|`build`| Collection of webpack and build scripts used to build Gravitational packages |
|`design`| Gravitational Design-System  |
|`shared`| Shared code |

## Working with the packages
The packages are managed by yarn via yarn workspaces.

To learn more about workspaces, check these links:

- [Workspaces in Yarn](https://yarnpkg.com/blog/2017/08/02/introducing-workspaces)
- [Workspaces](https://yarnpkg.com/en/docs/workspaces)

##### `yarn workspace <workspace_name> <command>` <a class="toc" id="toc-yarn-workspace" href="#toc-yarn-workspace"></a>

This will run the chosen Yarn command in the selected workspace.

Example:

```sh
yarn workspace awesome-package add react react-dom --dev
```

This will add `react` and `react-dom` as `devDependencies` in your `packages/awesome-package/package.json`.

If you want to remove a package:

```sh
yarn workspace web-project remove some-package
```

The example above would remove `some-package` from `packages/web-project/package.json`.

