import path from 'node:path';

const teleportDocsURL = 'https://goteleport.com/docs';

/**
 * RedirectChecker checks for Teleport docs site domains and paths within a
 * given file tree and determines whether a given docs configuration requires
 * redirects.
 * @param fs - The filesystem to use. Either memfs or the NodeJS fs package.
 * @param {string} otherRepoRoot - directory path in fs in which to check for
 * required redirects.
 * @param {string} docsRoot - directory path in fs in which to check for present
 * or missing docs files based on URL paths found in the directory tree at
 * otherRepoRoot.
 * @param {Array<object>} redirects - array of objects with keys "source",
 * "destination", and "permanent".
 * @param {Array<string>} exclude - array of file extensions not to check.
 */
export class RedirectChecker {
  constructor(fs, otherRepoRoot, docsRoot, redirects, exclude) {
    this.fs = fs;
    this.otherRepoRoot = otherRepoRoot;
    this.docsRoot = docsRoot;
    this.redirectSet = new Set();

    if (!exclude) {
      this.exclude = [];
    } else {
      this.exclude = exclude;
    }

    if (redirects) {
      redirects.forEach(r => {
        this.redirectSet.add(r.source);
      });
    }
  }

  // check walks the user-configured directory tree and identifies Teleport docs
  // URLs that do not correspond to an existing docs page or redirect. It
  // returns a list of problematic URLs.
  check() {
    return this.checkDir(this.otherRepoRoot);
  }

  // checkDir recursively checks for docs URLs with missing docs paths or
  // redirects at dirPath. It returns an array of missing URLs.
  checkDir(dirPath) {
    const files = this.fs.readdirSync(dirPath, 'utf8');
    let result = new Set();
    files.forEach(f => {
      for (let e = 0; e < this.exclude.length; e++) {
        if (f.endsWith(this.exclude[e])) {
          return;
        }
      }
      const fullPath = path.join(dirPath, f);
      const info = this.fs.statSync(fullPath);
      if (!info.isDirectory()) {
        for (const r of this.checkFile(fullPath)) {
          result.add(r);
        }
        return;
      }
      for (const r of this.checkDir(fullPath)) {
        result.add(r);
      }
    });
    return [...result];
  }

  // checkFile determines whether docs URLs found in the file
  // match either an actual docs file path or a redirect source.
  // Returns an array of URLs with missing files or redirects.
  checkFile(filePath) {
    const docsPattern = new RegExp(
      /https:\/\/goteleport.com\/docs(\/ver\/[0-9]+\.x)?(\/[\w\/_-]+)\/?#?/,
      'gm'
    );
    const text = this.fs.readFileSync(filePath, 'utf8');
    const docsURLs = [...text.matchAll(docsPattern)];
    if (docsURLs.length === 0) {
      return [];
    }
    let result = [];
    docsURLs.forEach(url => {
      const docsPath = this.urlToDocsPath(url[0]);
      const missingEntry = !this.fs.statSync(docsPath, {
        throwIfNoEntry: false,
      });

      if (!missingEntry) {
        return [];
      }

      let pathPart = url[2];
      if (pathPart[pathPart.length - 1] !== '/') {
        pathPart += '/';
      }

      if (!this.redirectSet.has(pathPart)) {
        result.push(url[0]);
      }
    });
    return result;
  }

  urlToDocsPath(url) {
    let nofrag = url.split('#')[0]; // Remove the fragment
    let rest = nofrag.slice(teleportDocsURL.length);
    if (rest.length === 0) {
      return path.join(this.docsRoot, 'docs', 'pages', 'index.mdx');
    }
    if (rest[rest.length - 1] === '/') {
      rest = rest.slice(0, rest.length - 1);
    }
    return path.join(this.docsRoot, 'docs', 'pages', rest + '.mdx');
  }
}
