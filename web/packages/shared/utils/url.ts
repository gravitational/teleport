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

interface URLParts {
  anchor?: string;
  path: string;
  query: Record<string, string>;
}

export const splitPath = (fullPath: string): URLParts => {
  const [rest, anchor] = fullPath.split("#");

  if (rest === undefined) {
    throw new Error("Expected there to be a URL, but there was not.");
  }

  const [path, search] = rest.split("?");

  if (path === undefined) {
    throw new Error("Expected there to be a URL, but there was not.");
  }

  const query: Record<string, string> = !search
    ? {}
    : search.split("&").reduce<{ [key: string]: string }>((result, segment) => {
        const [key, value] = segment.split("=");

        if (key === undefined) {
          throw new Error("Expected there to be a segment, but there was not.");
        }

        if (value) {
          result[key] = value;
        }

        return result;
      }, {});

  return { anchor, path, query };
};

export const isExternalLink = (href: string): boolean =>
  href.startsWith("//") || href.startsWith("mailto:") || href.includes("://");

export const isHash = (href: string): boolean => href.startsWith("#");

interface IsLocalAssetFileProps {
  extWhiteList?: string[];
  extBlackList?: string[];
}

export const getExtension = (href: string): string | undefined => {
  const parts = href.split("/");
  const filename = parts[parts.length - 1];

  if (filename && filename.indexOf(".") !== -1) {
    // should catch double extensions like `.tag.gz` and `.gitignore`
    const regex = /[^.]*\.(.+)/.exec(filename);
    if (!regex) {
      throw new Error("Unable to find file extension");
    }

    return regex[1];
  }

  return;
};

export const isLocalAssetFile = (
  href: unknown,
  options: IsLocalAssetFileProps = {}
) => {
  if (typeof href !== "string") {
    return false;
  }

  const { extWhiteList = [], extBlackList = [] } = options;

  const { path } = splitPath(href);
  const ext = getExtension(path);

  return (
    !isExternalLink(path) &&
    !path.startsWith("/") &&
    !!ext &&
    (extBlackList.length ? !extBlackList.includes(ext) : false) &&
    (extWhiteList.length ? extWhiteList.includes(ext) : true)
  );
};

export const buildPath = (parts: URLParts): string => {
  let result = parts.path;

  const search = Object.entries(parts.query)
    .map(([key, value]) => `${key}=${value}`)
    .join("&");

  if (search) {
    result = `${result}?${search}`;
  }

  if (parts.anchor) {
    result = `${result}#${parts.anchor}`;
  }

  return result;
};

// router.asPath returns original path before rewrites, so to match
// browser url with url in asPath we need to first apply rewrites to it
// manually
export const normalizePath = (path: string, replace = "") => {
  const parts = splitPath(path);

  parts.path = parts.path.replace(replace, "");

  // In SSR mode next ignores trailingSlsh option in asPath
  if (!parts.path.endsWith("/")) {
    parts.path = `${parts.path}/`;
  }

  return buildPath(parts);
};
