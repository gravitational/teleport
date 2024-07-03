import { resolve } from 'node:path';

import tsconfigPaths from 'vite-tsconfig-paths';

const rootDirectory = resolve(import.meta.dirname, '../../../..');

export function tsconfigPathsPlugin() {
  return tsconfigPaths({
    // Asking vite to crawl the root directory (by defining the `root` object, rather than `projects`) causes vite builds to fail
    // with a:
    //
    // "Error: ENOTDIR: not a directory, scandir '/go/src/github.com/gravitational/teleport/docker/ansible/rdir/rdir/rdir'""
    //
    // on a Debian GNU/Linux 10 (buster) (buildbox-node) Docker image running on an arm64 Macbook macOS 14.1.2. It's not clear why
    // this happens, however defining the tsconfig file directly works around the issue.
    projects: [resolve(rootDirectory, 'tsconfig.json')],
  });
}
