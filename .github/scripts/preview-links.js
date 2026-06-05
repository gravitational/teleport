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

// Build the public preview URL for a docs/pages/*.mdx file.
//   previewHost: e.g. "https://my-branch.d123.amplifyapp.com"
//   p:           e.g. "docs/pages/foo/bar.mdx"
function pageUrl(previewHost, p) {
  let urlPath = p.replace(/^docs\/pages\//, '').replace(/\.mdx$/, '');
  // index pages map to their parent directory; handle both
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

// Split the changed paths into the two categories we care about:
//   - directlyChangedPages: a Set of docs/pages/**/*.mdx (excluding includes)
//   - changedImages:        an array of {path, basename, matchSuffix} objects
//                           for docs/img/**/*.{png,svg}
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

  return { directlyChangedPages, changedImages };
}

// Given one MDX file's text content and the list of changed images, return
// just the changed images whose reference appears in that content.
function matchedImagesInContent(content, changedImages) {
  return changedImages.filter((img) => content.includes(img.matchSuffix));
}

// Changed images that no page references at all, sorted for stable output.
// referencedImagePaths is a Set of image paths that were matched somewhere.
function computeOrphanedImages(changedImages, referencedImagePaths) {
  return changedImages
    .filter((img) => !referencedImagePaths.has(img.path))
    .map((img) => img.path)
    .sort();
}

// The unified, de-duplicated, sorted list of page paths to link:
// directly-changed pages plus any page that references a changed image.
// imageRefMap is a Map of mdxPath -> Set<basename>.
function buildAllPagePaths(directlyChangedPages, imageRefMap) {
  return [
    ...new Set([...directlyChangedPages, ...imageRefMap.keys()]),
  ].sort();
}

// Build the Markdown list items, one per page, with an optional annotation
// noting which changed image(s) the page references.
function buildPageEntries(allPagePaths, imageRefMap, previewHost) {
  return allPagePaths.map((p) => {
    const url = pageUrl(previewHost, p);
    const imageRefs = imageRefMap.get(p);
    let annotation = '';
    if (imageRefs && imageRefs.size > 0) {
      const sorted = [...imageRefs].sort();
      const label = sorted.length === 1 ? 'image change' : 'image changes';
      annotation = ` — ${label}: ${sorted.map((b) => '`' + b + '`').join(', ')}`;
    }
    return `- [\`${p}\`](${url})${annotation}`;
  });
}

// Compose the full Markdown section (including the start/end markers) from
// the page entries and any orphaned images.
function composeLinksSection(pageEntries, orphanedImages) {
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
  pageUrl,
  getChangedPaths,
  categorizeChangedPaths,
  matchedImagesInContent,
  computeOrphanedImages,
  buildAllPagePaths,
  buildPageEntries,
  composeLinksSection,
  upsertLinksSection,
};
