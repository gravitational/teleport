---
author: Grzegorz Zdunek (grzegorz.zdunek@goteleport.com)
state: draft
---

# RFD 177 - Upgrade JS package manager 

## Required Approvers

- Engineering: @zmb3 && (@ravicious || @ryanclark)

## What
Currently (June 2024), we use Yarn 1.x to manage JS dependencies. We should 
upgrade it to Yarn >= 2.x or switch to a different manager.

## Why
Yarn 1.x entered maintenance mode in January 2020 and will eventually reach 
end-of-life in terms of support. 
It also has many unfixed issues, with a one that affects us the most: 
[incorrect dependency hoisting when using workspaces](https://github.com/Yarnpkg/Yarn/issues/7572).

## Details
### What is the problem with dependency hoisting in Yarn 1.x?
Dependency hoisting is a feature designed to optimize the layout of the 
`node_modules` directory by moving dependencies as high in the project directory 
tree as possible. This approach reduces duplication and simplifies the 
dependency tree, leading to less disk space usage.
However, this does not work correctly in Yarn 1.x and hoists too deep 
dependencies.
For example, if `packages/build/package.json` depends on `@babel/core ^7.23.2` 
and `@storybook/builder-webpack5 ^6.5.16` depends on `@babel/core ^7.12.10`, 
Yarn will hoist the `@babel/core` version resolved from that storybook 
dependency, rather than the direct `@babel/core` dep in package.json.
This makes managing dependencies difficult: we don't really know what version of 
a package is being used.

We experienced this issue many times, and the workaround was to move such 
dependency to the root `package.json`.
The Yarn team confirmed that it won't be fixed in the 1.x branch.

Theoretically, upgrading the package manager to Yarn >= 2.0.0 (with 
`nodeLinker: node-modules` option that hoists dependencies in a similar way to 
Yarn 1.x) would solve this particular problem, but we should first take a closer 
look at the dependency hoisting problem.

### What’s the problem with dependency hoisting in general?
Traditionally, packages mangers (npm, Yarn 1.x) maintain a flat dependency list 
in `node_modules` by hoisting all dependencies there.
For example, when the developer installs package `fuu`, which depends on 
`bar` and `baz`, the package manager creates three directories, 
`fuu`, `bar` and `baz` in `node_modules`.
The developer can then import all of these packages in code, although only one 
of them is listed in the `package.json` file.
It is possible because of how the Node.js module resolution algorithm works. 
The `package.json` file is not needed at all; when Node.js resolves an import, 
it traverses `node_modules` folders all the way to the user's root directory, 
looking for that dependency.

The main problem with using undeclared dependencies is that we have no control 
over them. The next (even a patch) version of the `fuu` package may update `bar` 
to a version that makes our code relying on `bar` stop working properly (read 
more: [pnpm's strictness helps to avoid silly bugs](https://www.kochan.io/nodejs/pnpms-strictness-helps-to-avoid-silly-bugs.html)).
We can see such dependencies in our codebase: we import `prop-types` in a few 
components, but we don't specify in any workspace’s `package.json` 
(it's taken from `react-select`).

To address that problem, the new package manager should use non-flat (or 
sometimes called isolated) `node_modules` structure, which contains only 
direct dependencies, declared in `package.json`.
Let's go through installation options of the most popular package managers:
* npm has `install-strategy`:
    * hoisted (default): install non-duplicated in top-level, and duplicated as 
  necessary within directory structure.
    * nested: install in place, no hoisting. Note: this does not seem to work 
  correctly https://github.com/npm/cli/issues/6537.
    * shallow: only install direct deps at top-level. Note: it creates 
  `node_modules` directory only in the project's root, so it does not change 
  much when it comes to workspaces, the dependencies are still hoisted.
    * linked: (experimental) install in node_modules/.store, link in place, 
  unhoisted.
  Note: works like pnpm. See an [RFD](https://github.com/npm/rfcs/blob/main/accepted/0042-isolated-mode.md).
* Yarn has `nodeLinker`: 
    * pnp: no node_modules, unhoisted. Note: does not work with Electron.
    * pnpm: install in `node_modules/.store`, link in place, unhoisted. 
  Note: It should work with the latest alpha electron-builder and Electron 
  versions.
    * node_modules: flat, hoisted. Same as the node_modules created by npm or 
  Yarn Classic.
* pnpm has `node-linker`:
    * isolated: dependencies are symlinked from a virtual store at
  `node_modules/.pnpm`. Note: I was able to build Teleport Connect with the 
  latest electron-builder and Electron versions.
    * hoisted: a flat node_modules without symlinks is created. 
  Same as the node_modules created by npm or Yarn Classic.
    * pnp: no node_modules. Same as pnp in Yarn.

The installation option that is most strict and works with Electron is
npm's `linked`, Yarn's `pnpm` and pnpm's `isolated`.

### So, what package manager should we migrate to?
Modern package managers offer similar features, so I think the strictness and 
performance of the manager are the factors that differentiate them the most.
Here pnpm appears to be the winner. It supports installing dependencies as 
linked by default, which makes it the most battle-tested implementation.
The speed is also great, it's consistently one of the fastest tools in 
[benchmarks](https://p.datadoghq.eu/sb/d2wdprp9uki7gfks-c562c42f4dfd0ade4885690fa719c818?fromUser=false&refresh_mode=sliding&tpl_var_npm%5B0%5D=%2A&tpl_var_pnpm%5B0%5D=%2A&tpl_var_yarn-classic%5B0%5D=%2A&tpl_var_yarn-modern%5B0%5D=%2A&tpl_var_yarn-nm%5B0%5D=%2A&tpl_var_yarn-pnpm%5B0%5D=no&from_ts=1711447153431&to_ts=1719223153431&live=true).

The npm support for `linked` is really interesting, but as long as it is 
experimental, we should not use it in a production environment.

When it comes to Yarn, the main advantage is that we already use it, so we could 
still type `yarn start-teleport`. 
However, if we are going to use pnpm installation method, why not switch to pnpm 
entirely? 
Yarn also shows a warning `The pnpm linker doesn't support providing different 
versions to workspaces' peer dependencies` during the installation, but I'm not 
sure what significance this has.

To sum up: I recommend migrating to pnpm.

## Migration process
### Establishing correct relationships between workspaces
After switching the package manager to pnpm, the Web UI/Connect apps won’t run, 
due to TS/JS code in the workspaces not being able to resolve their imports 
(like `react`).
Why? This is a continuation of the hoisting problem — our workspaces rely 
on dependencies of other workspaces, without declaring them in their 
`package.json`.
It worked in Yarn 1.x, since adding a dependency to a one workspace made it 
available in any other workspace (because of hoisting to the root 
`node_modules`).
We can see this in `packages/build`: the `@opentelemetry/*` dependencies are 
declared there but used only in `packages/teleport`.

This contradicts the idea of workspaces: they are designed to encapsulate 
project dependencies and code. Sharing dependencies between unrelated workspaces 
breaks this encapsulation, leading to potential conflicts and contamination 
where changes in one workspace inadvertently affect another.

To allow the projects to compile again, the dependencies need to be moved to the
correct workspaces.
I see three scenarios for how they should be moved over:
1. Dependencies that are used only in a single workspace should be kept there.
Example: `electron-builder` that builds Connect - it’s declared in 
`packages/build`, but it should be in `packages/teleterm`.
2. Project-wide dependencies imported everywhere, should be kept in the root 
`package.json`.
Example: `react` - declared in `build/package.json`, but imported in every other 
workspace.
3. Project-wide tools used in the root `package.json` should be kept in 
`build/package.json`, if possible. 
Example: `jest` or `storybook` can be installed in `build/package.json` 
which will expose them through scripts to the root `package.json`.

There is also an edge case for teleport.e. Its dependencies are declared in 
`packages/e-imports`, allowing them to be installed when only the OSS teleport 
repository is cloned. This makes the lockfile always look the same, regardless 
of teleport.e being cloned or not. 
After our workspaces become “isolated”, code in teleport.e will no longer be 
able to import these dependencies. 
This problem could be solved by re-exporting them from `packages/e-imports`.
For example, `import Highlight from ‘react-highlight’` would become 
`import Highlight from '@gravitational/e-imports/react-highlight'`.
However, the ergonomics of this solution is not ideal, as importing packages 
from `@gravitational/e-imports` would be too cumbersome for developers.
Instead, we will move these dependencies to the root `package.json`, making
them explicitly available everywhere. We don't have too many packages used 
exclusively by teleport.e, so this should be acceptable.
The `e-imports` workspace will be removed.

This part should be done before the actual package manager upgrade.

### Upgrading the manager
In the actual PR, we will need to:
1. Set `"packageManager": "pnpm@x.y.z”` in the root package.json (the exact 
version is required).
   - It will install the correct package manager automatically if `corepack` is 
   enabled.
   - Calling `yarn run ...` will return a clear error that the project uses a 
   different package manager. 
2. Convert the lockfile to a new format (`pnpm import`).
3. Update CI, Makefile and package.json scripts.

The dev teams will be notified of the transition via Slack, along with examples 
of the updated commands. 
For example: 
>`yarn start-teleport` is now `pnpm run start-teleport`

### Backports
For the best developer experience, we should change the manager in all release 
branches. However, this may be really time-consuming, since all branches have 
different dependencies.
Alternatively, we can add `"packageManager": "yarn@1.22.22”`to other release 
branches to indicate these still use Yarn 1.x.