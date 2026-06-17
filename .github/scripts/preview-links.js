// preview-links.js
//
// Pure helper functions for the "Docs Preview" workflow. None of these
// touch the GitHub API or the Actions `context`/`core` objects, so they can
// be unit tested in isolation (see preview-links.test.js). The workflow's
// orchestration lives in post-preview-links.js, which calls into here.

const path = require('path');

// HTML markers that bracket our section inside the deployment-status comment.
// They let us find and replace our own section on re-runs without disturbing
// the rest of the comment.
const START_MARKER = '<!-- amplify-per-page-preview:start -->';
const END_MARKER = '<!-- amplify-per-page-preview:end -->';

// True if a path is a rendered docs page (not a reusable partial under
// an includes/ directory).
function isRenderedPage(p) {
  return (
    p.startsWith('docs/pages/') &&
    p.endsWith('.mdx') &&
    !p.includes('/includes/')
  );
}

// Build the public preview URL for a docs/pages/*.mdx file.
//   previewHost: e.g. "https://my-branch.d123.amplifyapp.com"
//   p:           e.g. "docs/pages/foo/bar.mdx"
function pageUrl(previewHost, p) {
  let urlPath = p.replace(/^docs\/pages\//, '').replace(/\.mdx$/, '');
  // Section index pages are named after their parent directory
  // (e.g. enroll-resources/enroll-resources.mdx) and render at the directory
  // URL (/docs/enroll-resources/), not /docs/enroll-resources/enroll-resources/.
  // Collapse a trailing foo/bar/bar -> foo/bar.
  urlPath = urlPath.replace(/(^|\/)([^/]+)\/\2$/, '$1$2');
  // index pages also map to their parent directory; handle both
  // top-level (docs/pages/index.mdx) and nested (foo/bar/index.mdx).
  urlPath = urlPath.replace(/(^|\/)index$/, '');
  // Trim any trailing slash the index strip may have left
  // (foo/index -> foo/) to avoid producing double-slash URLs.
  urlPath = urlPath.replace(/\/$/, '');
  return urlPath === ''
    ? `${previewHost}/docs/`
    : `${previewHost}/docs/${urlPath}/`;
}

// Turn the raw file list from the GitHub API into a plain list of changed
// paths, dropping deleted files. `files` is the array returned by
// pulls.listFiles (each item has `status` and `filename`).
function getChangedPaths(files) {
  return files
    .filter((f) => f.status !== 'removed')
    .map((f) => f.filename);
}

// Split the changed paths into the categories we care about:
//   - directlyChangedPages: a Set of docs/pages/**/*.mdx (excluding includes)
//   - changedImages:        an array of {path, basename, matchSuffix} objects
//                           for docs/img/**/*.{png,svg}
//   - changedPartials:      an array of docs/pages/**/includes/**/*.mdx paths
// matchSuffix is the portion of the path that MDX references contain
// (e.g. "img/headless/approval.png"), used for substring matching later.
function categorizeChangedPaths(changedPaths) {
  const directlyChangedPages = new Set(
    changedPaths
      .filter((p) => p.startsWith('docs/pages/'))
      .filter((p) => p.endsWith('.mdx'))
      .filter((p) => !p.includes('/includes/'))
  );

  const changedImages = changedPaths
    .filter((p) => p.startsWith('docs/img/'))
    .filter((p) => /\.(png|svg)$/i.test(p))
    .map((p) => ({
      path: p,
      basename: path.basename(p),
      matchSuffix: p.replace(/^docs\//, ''),
    }));

  const changedPartials = changedPaths
    .filter((p) => p.startsWith('docs/pages/'))
    .filter((p) => p.endsWith('.mdx'))
    .filter((p) => p.includes('/includes/'));

  return { directlyChangedPages, changedImages, changedPartials };
}

// Given one MDX file's text content and the list of changed images, return
// just the changed images whose reference appears in that content.
function matchedImagesInContent(content, changedImages) {
  return changedImages.filter((img) => content.includes(img.matchSuffix));
}

// Extract the partial (.mdx) paths referenced by Teleport include directives
// in the given content. Directives look like (!path!) and may carry props
// after the path, e.g. (!docs/pages/includes/foo.mdx service="x" region="y"!).
// We capture the leading .mdx path token and ignore any trailing props before
// the closing !). Optional surrounding whitespace is tolerated. A leading
// slash is stripped so absolute forms (!/docs/pages/includes/foo.mdx!) match
// the repo-root-relative paths in the changed-file list. Non-.mdx includes
// (e.g. code snippets) are ignored, since they are not rendered pages.
function extractIncludePaths(content) {
  const paths = [];
  const re = /\(!\s*\/?([^\s!]+\.mdx)(?:\s+[^!]*)?!\)/g;
  let m;
  while ((m = re.exec(content)) !== null) {
    paths.push(m[1]);
  }
  return paths;
}

// Resolve changed partials to the rendered pages that embed them.
//   changedPartials: array of partial paths that changed in the PR.
//   includedBy:      Map of partialPath -> Set<includerPath>, where an
//                    includer is any file that contains (!partialPath!).
// Returns a Map of renderedPagePath -> Set<changedPartialPath>. Partials can
// be included by other partials, so this walks the "included-by" graph upward
// until it reaches rendered pages, guarding against cycles.
function resolveAffectedPages(changedPartials, includedBy) {
  const affectedPages = new Map();
  for (const partial of changedPartials) {
    const seen = new Set();
    const stack = [partial];
    while (stack.length > 0) {
      const current = stack.pop();
      const includers = includedBy.get(current);
      if (!includers) continue;
      for (const includer of includers) {
        if (seen.has(includer)) continue;
        seen.add(includer);
        if (isRenderedPage(includer)) {
          if (!affectedPages.has(includer)) {
            affectedPages.set(includer, new Set());
          }
          affectedPages.get(includer).add(partial);
        } else {
          // The includer is itself a partial; keep climbing toward pages.
          stack.push(includer);
        }
      }
    }
  }
  return affectedPages;
}

// Changed images that no page references at all, sorted for stable output.
// referencedImagePaths is a Set of image paths that were matched somewhere.
function computeOrphanedImages(changedImages, referencedImagePaths) {
  return changedImages
    .filter((img) => !referencedImagePaths.has(img.path))
    .map((img) => img.path)
    .sort();
}

// Changed partials that no rendered page embeds (directly or transitively),
// sorted for stable output. affectedPages is the Map from resolveAffectedPages.
function computeOrphanedPartials(changedPartials, affectedPages) {
  const reached = new Set();
  for (const partials of affectedPages.values()) {
    for (const p of partials) reached.add(p);
  }
  return changedPartials.filter((p) => !reached.has(p)).sort();
}

// The unified, de-duplicated, sorted list of page paths to link: directly
// changed pages, pages referencing a changed image, and pages embedding a
// changed partial. imageRefMap/partialRefMap are Maps keyed by page path.
function buildAllPagePaths(directlyChangedPages, imageRefMap, partialRefMap) {
  return [
    ...new Set([
      ...directlyChangedPages,
      ...(imageRefMap ? imageRefMap.keys() : []),
      ...(partialRefMap ? partialRefMap.keys() : []),
    ]),
  ].sort();
}

// Build the parenthetical annotation for a page, noting any changed images it
// references and any changed partials it embeds. imageRefMap/partialRefMap are
// Maps of pagePath -> Set<basename>.
function buildAnnotation(p, imageRefMap, partialRefMap) {
  const segments = [];

  const imageRefs = imageRefMap && imageRefMap.get(p);
  if (imageRefs && imageRefs.size > 0) {
    const sorted = [...imageRefs].sort();
    segments.push(
      `${sorted.length === 1 ? 'image change' : 'image changes'}: ` +
        sorted.map((b) => '`' + b + '`').join(', ')
    );
  }

  const partialRefs = partialRefMap && partialRefMap.get(p);
  if (partialRefs && partialRefs.size > 0) {
    const sorted = [...partialRefs].sort();
    segments.push(
      `${sorted.length === 1 ? 'affected partial' : 'affected partials'}: ` +
        sorted.map((b) => '`' + b + '`').join(', ')
    );
  }

  return segments.length > 0 ? ` — ${segments.join('; ')}` : '';
}

// Build the Markdown list items, one per page, with optional annotations.
//   refMaps: { imageRefMap, partialRefMap }, both Maps of page -> Set<basename>
function buildPageEntries(allPagePaths, previewHost, refMaps = {}) {
  const { imageRefMap, partialRefMap } = refMaps;
  return allPagePaths.map((p) => {
    const url = pageUrl(previewHost, p);
    return `- [\`${p}\`](${url})${buildAnnotation(p, imageRefMap, partialRefMap)}`;
  });
}

// Compose the full Markdown section (including the start/end markers) from the
// page entries and any orphaned images/partials.
function composeLinksSection(pageEntries, orphanedImages = [], orphanedPartials = []) {
  const sectionLines = [START_MARKER, '', '### Preview links for changed docs pages', ''];

  if (pageEntries.length > 0) {
    sectionLines.push(...pageEntries);
  } else {
    sectionLines.push('No docs pages changed in this PR.');
  }

  if (orphanedImages.length > 0) {
    sectionLines.push('', '#### Changed images with no associated page', '');
    sectionLines.push(...orphanedImages.map((p) => `- \`${p}\``));
  }

  if (orphanedPartials.length > 0) {
    sectionLines.push('', '#### Changed partials not included by any page', '');
    sectionLines.push(...orphanedPartials.map((p) => `- \`${p}\``));
  }

  sectionLines.push('', END_MARKER);
  return sectionLines.join('\n');
}

// Insert or replace our section inside an existing comment body. If a section
// from a prior run is already present (between the markers), replace it in
// place; otherwise append it after a horizontal rule.
function upsertLinksSection(body, linksSection) {
  const existingSectionRegex = new RegExp(
    `\\n*${START_MARKER}[\\s\\S]*?${END_MARKER}\\n*`,
    'g'
  );
  if (existingSectionRegex.test(body)) {
    return body.replace(existingSectionRegex, `\n\n${linksSection}\n`);
  }
  return `${body}\n\n---\n\n${linksSection}`;
}

module.exports = {
  START_MARKER,
  END_MARKER,
  isRenderedPage,
  pageUrl,
  getChangedPaths,
  categorizeChangedPaths,
  matchedImagesInContent,
  extractIncludePaths,
  resolveAffectedPages,
  computeOrphanedImages,
  computeOrphanedPartials,
  buildAllPagePaths,
  buildAnnotation,
  buildPageEntries,
  composeLinksSection,
  upsertLinksSection,
};
