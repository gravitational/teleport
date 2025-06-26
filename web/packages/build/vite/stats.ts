/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import type { Plugin } from 'vite';
import { gzipSync } from 'node:zlib';
import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'path';
import { join, relative } from 'node:path'
import type { OutputAsset, OutputChunk } from 'rollup';

type OutputBundle =
  | ({ type: 'chunk' } & OutputChunk)
  | ({ type: 'asset' } & OutputAsset);

interface JSONStatsReport {
  bundleSizes: Record<string, number>;
  bundleGzipSizes: Record<string, number>;
  fileSizes: Record<string, number>;
  fileGzipSizes: Record<string, number>;
  moduleSizes: Record<string, number>;
  moduleGzipSizes: Record<string, number>;
  totalSize: number;
  totalGzipSize: number;
}

export function statsPlugin(rootDirectory: string, outputDirectory: string): Plugin {
  return {
    name: 'teleport-stats-plugin',
    async writeBundle(options, output) {
      const bundleSizes = new Map<string, number>();
      const bundleGzipSizes = new Map<string, number>();
      const fileSizes = new Map<string, number>();
      const fileGzipSizes = new Map<string, number>();
      const moduleSizes = new Map<string, number>();
      const moduleGzipSizes = new Map<string, number>();

      for (const [bundleId, bundle] of Object.entries(output)) {
        bundleSizes.set(bundleId, getBundleSize(bundle));
        bundleGzipSizes.set(bundleId, getGzippedSize(bundle));

        if (bundle.type === 'asset') {
          continue;
        }

        for (const [moduleId, module] of Object.entries(bundle.modules)) {
          if (/\/node_modules\//.test(moduleId)) {
            const packageName = getPackageName(moduleId, rootDirectory);

            const packageSize = moduleSizes.get(packageName) ?? 0;
            const packageGzipSize = moduleGzipSizes.get(packageName) ?? 0;

            moduleSizes.set(packageName, packageSize + module.renderedLength);

            const moduleCode = module.code ?? '';
            const gzippedLength = gzipSync(moduleCode).length;

            moduleGzipSizes.set(packageName, packageGzipSize + gzippedLength);

            continue;
          }

          // eslint-disable-next-line no-control-regex
          if (/\u0000/g.test(moduleId)) {
            // This is a Vite virtual module, skip it
            continue;
          }

          const relativeModuleId = makeRelative(moduleId, rootDirectory);

          const fileSize = fileSizes.get(relativeModuleId) ?? 0;
          const fileGzipSize = fileGzipSizes.get(relativeModuleId) ?? 0;

          fileSizes.set(relativeModuleId, fileSize + module.renderedLength);

          const moduleCode = module.code ?? '';
          const gzippedLength = gzipSync(moduleCode).length;

          fileGzipSizes.set(relativeModuleId, fileGzipSize + gzippedLength);
        }
      }

      const totalSize = Array.from(bundleSizes.values()).reduce(
        (sum, size) => sum + size,
        0
      );

      const totalGzipSize = Array.from(bundleGzipSizes.values()).reduce(
        (sum, size) => sum + size,
        0
      );

      const report: JSONStatsReport = {
        bundleSizes: sortedObjectFromMap(bundleSizes),
        bundleGzipSizes: sortedObjectFromMap(bundleGzipSizes),
        fileSizes: sortedObjectFromMap(fileSizes),
        fileGzipSizes: sortedObjectFromMap(fileGzipSizes),
        moduleSizes: sortedObjectFromMap(moduleSizes),
        moduleGzipSizes: sortedObjectFromMap(moduleGzipSizes),
        totalSize,
        totalGzipSize,
      };

      writeFileSync(
        resolve(outputDirectory, 'stats.json'),
        JSON.stringify(report, null, 2)
      );
    },
  };
}
function getBundleSize(bundle: OutputBundle) {
  switch (bundle.type) {
    case 'chunk':
      return Buffer.byteLength(bundle.code);

    case 'asset':
      if (typeof bundle.source === 'string') {
        return Buffer.byteLength(bundle.source);
      }

      return bundle.source.length;
  }
}

function getGzippedSize(bundle: OutputBundle) {
  switch (bundle.type) {
    case 'chunk':
      return gzipSync(bundle.code).length;

    case 'asset':
      if (typeof bundle.source === 'string') {
        return gzipSync(bundle.source).length;
      }

      return gzipSync(bundle.source).length;
  }
}

const packageNameCache = new Map<string, string>();

function findPackageName(path: string, rootDirectory: string): string | null {
  const resolvedRoot = resolve(rootDirectory);
  const parts = path.split('/');

  for (let i = parts.length - 1; i >= 0; i--) {
    const currentPath = parts.slice(0, i + 1).join('/');

    if (!currentPath.startsWith(resolvedRoot)) {
      break;
    }

    const packageJsonPath = join(currentPath, 'package.json');

    if (existsSync(packageJsonPath)) {
      const packageJson = JSON.parse(readFileSync(packageJsonPath, 'utf-8'));

      if (packageJson.name) {
        return packageJson.name;
      }
    }
  }

  return null;
}

function getPackageName(_path: string, rootDirectory: string) {
  // eslint-disable-next-line no-control-regex
  const path = dirname(_path).replace(/^\x00/, ''); // remove Vite's virtual module prefix

  if (packageNameCache.has(path)) {
    return packageNameCache.get(path)!;
  }

  const packageName = findPackageName(path, rootDirectory);

  packageNameCache.set(path, packageName);

  return packageName;
}

function makeRelative(path: string, rootDirectory: string): string {
  const resolvedRoot = resolve(rootDirectory);
  const resolvedPath = resolve(path);

  if (resolvedPath.startsWith(resolvedRoot)) {
    return relative(resolvedRoot, resolvedPath);
  }

  return path;
}

function sortedObjectFromMap(map: Map<string, number>) {
  return Object.fromEntries(
    Array.from(map.entries()).sort((a, b) => b[1] - a[1])
  );
}