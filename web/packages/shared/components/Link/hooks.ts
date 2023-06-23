/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { resolve } from "url";

import { useRouter } from "next/router";

import {
  normalizePath,
  splitPath,
  isHash,
  isExternalLink,
  isLocalAssetFile,
} from "shared/utils/url";

/*
 * This hook should return current href with resolved rewrites
 */

export const useCurrentHref = () => {
  const { asPath } = useRouter();

  return normalizePath(asPath);
};

/*
 * This hook should return absolute site paths, with resolved rewrites and correct scopes
 */

export const useNormalizedHref = (href: string) => {
  const { asPath, basePath } = useRouter();

  const noBaseHref = href.startsWith(basePath)
    ? href.substring(basePath.length)
    : href;

  // This needs to be added because all strings of "/docs/" are being stripped down to
  // "/" in noBaseHref. This is called below useContext because of the rule of hooks
  // in which hooks are not able to be called conditionally
  if (href === `${basePath}/`) {
    return href;
  }

  if (
    isHash(noBaseHref) ||
    isExternalLink(noBaseHref) ||
    isLocalAssetFile(noBaseHref)
  ) {
    return noBaseHref;
  }

  const replaceUrl = basePath === '/docs' ? `/ver/${process.env.DOCS_LATEST_VERSION}` : '';

  const currentHref = normalizePath(asPath, replaceUrl);

  let fullHref = resolve(splitPath(currentHref).path, noBaseHref);

  return fullHref;
};

