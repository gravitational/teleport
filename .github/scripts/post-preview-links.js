// post-preview-links.js
//
// Orchestration for the "Append per-page preview links" workflow step.
// This file holds everything that DOES talk to the GitHub API, the Actions
// `context`, or the filesystem. All the testable string/path logic lives in
// preview-links.js and is imported here.
//
// The workflow invokes this via actions/github-script:
//
//   const { run } = require('./.github/scripts/post-preview-links.js');
//   await run({ github, context, core });

const fs = require('fs');
const path = require('path');

const {
  pageUrl, // re-exported for convenience/tests
  categorizeChangedPaths,
  matchedImagesInContent,
  extractIncludePaths,
  resolveAffectedPages,
  computeOrphanedImages,
  computeOrphanedPartials,
  buildAllPagePaths,
  buildPageEntries,
  composeLinksSection,
  upsertLinksSection,
  extractPreviewHost,
  sanitizeBranchForDomain,
  START_MARKER,
} = require('./preview-links.js');

// Recursively list every .mdx file under `dir`. By default (skipIncludes:
// true) it skips any subdirectory named "includes", matching the set of
// rendered pages. Pass { skipIncludes: false } to also list partials, which
// is needed to build the include graph. Filesystem I/O, so it lives here
// rather than in the pure module, but it has no GitHub dependency and is
// covered by a test.
async function listMdxPages(dir, { skipIncludes = true } = {}) {
  const results = [];
  async function walk(d) {
    const entries = await fs.promises.readdir(d, { withFileTypes: true });
    for (const entry of entries) {
      const full = path.join(d, entry.name);
      if (entry.isDirectory()) {
        if (skipIncludes && entry.name === 'includes') continue;
        await walk(full);
      } else if (entry.isFile() && full.endsWith('.mdx')) {
        results.push(full);
      }
    }
  }
  await walk(dir);
  return results;
}

async function run({ github, context, core }) {
  // --- Changed files come from the dorny/paths-filter step as a JSON array
  // (passed via CHANGED_FILES). That action handles diffing and pagination,
  // and the added|modified filter already excludes deleted files, so there is
  // no API call or status filtering to do here. ---
  let changedPaths = [];
  const changedFilesRaw = process.env.CHANGED_FILES;
  if (changedFilesRaw) {
    try {
      changedPaths = JSON.parse(changedFilesRaw);
    } catch (err) {
      core.warning(`Could not parse CHANGED_FILES as JSON; skipping per-page comment: ${err.message}`);
      return;
    }
  }

  const { directlyChangedPages, changedImages, changedPartials } =
    categorizeChangedPaths(changedPaths);

  // --- Single filesystem pass: map changed images to the pages that
  // reference them, and build the include graph used to map changed partials
  // back to the pages that embed them. We only walk the tree when there is
  // indirect work to do (an image or partial changed). ---
  const imageRefMap = new Map(); // page path -> Set<image basename>
  const referencedImagePaths = new Set();
  const includedBy = new Map(); // partial path -> Set<includer path>

  if (changedImages.length > 0 || changedPartials.length > 0) {
    // Partials can include other partials, so build the graph from every mdx
    // file, not just rendered pages.
    const allMdxFiles = await listMdxPages('docs/pages', { skipIncludes: false });
    for (const mdxPath of allMdxFiles) {
      const content = await fs.promises.readFile(mdxPath, 'utf8');

      // Image references only matter on rendered pages (partials are not
      // linked directly), preserving the existing image behavior.
      if (changedImages.length > 0 && !mdxPath.includes('/includes/')) {
        for (const img of matchedImagesInContent(content, changedImages)) {
          if (!imageRefMap.has(mdxPath)) imageRefMap.set(mdxPath, new Set());
          imageRefMap.get(mdxPath).add(img.basename);
          referencedImagePaths.add(img.path);
        }
      }

      // Record include edges: this file includes each extracted partial path.
      if (changedPartials.length > 0) {
        for (const included of extractIncludePaths(content)) {
          if (!includedBy.has(included)) includedBy.set(included, new Set());
          includedBy.get(included).add(mdxPath);
        }
      }
    }
  }

  // Resolve changed partials to the rendered pages that embed them, then
  // reduce to a per-page set of partial basenames for display.
  const affectedPages = resolveAffectedPages(changedPartials, includedBy);
  const partialRefMap = new Map(); // page path -> Set<partial basename>
  for (const [pagePath, partials] of affectedPages) {
    partialRefMap.set(pagePath, new Set([...partials].map((p) => path.basename(p))));
  }

  const orphanedImages = computeOrphanedImages(changedImages, referencedImagePaths);
  const orphanedPartials = computeOrphanedPartials(changedPartials, affectedPages);
  const allPagePaths = buildAllPagePaths(directlyChangedPages, imageRefMap, partialRefMap);

  // --- Find the Amplify deployment-status comment. We both append our links
  // to it and read the canonical preview host from it: Amplify builds that URL
  // from the branch display name, which differs from the raw branch name for
  // names containing characters it sanitizes (e.g. "feature/foo"). ---
  const { data: comments } = await github.rest.issues.listComments({
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number: context.issue.number,
  });

  const deploymentComment = comments.find(
    (c) =>
      c.user?.login === 'github-actions[bot]' &&
      c.body?.includes('Amplify deployment status') &&
      c.body?.includes('amplifyapp.com')
  );

  // Prefer the host from the deployment comment (built from the display name).
  // Fall back to a sanitized branch name only if the comment is missing.
  const previewAppId = process.env.PREVIEW_APP_ID;
  let previewHost = deploymentComment
    ? extractPreviewHost(deploymentComment.body, previewAppId)
    : null;
  if (!previewHost) {
    const previewBranch = process.env.PREVIEW_BRANCH;
    if (previewBranch && previewAppId) {
      previewHost = `https://${sanitizeBranchForDomain(previewBranch)}.${previewAppId}.amplifyapp.com`;
    }
  }
  if (!previewHost) {
    core.warning(
      'Could not determine the Amplify preview host from the deployment comment ' +
        'or environment; skipping per-page comment.'
    );
    return;
  }

  const pageEntries = buildPageEntries(allPagePaths, previewHost, { imageRefMap, partialRefMap });
  const linksSection = composeLinksSection(pageEntries, orphanedImages, orphanedPartials);

  if (!deploymentComment) {
    core.warning(
      'Could not find the Amplify deployment status comment to append ' +
        'per-page preview links to. Posting per-page links as a separate comment.'
    );

    // Update an existing fallback comment if present, else create one, so
    // reruns do not accumulate duplicate comments.
    const existingFallback = comments.find(
      (c) => c.user?.login === 'github-actions[bot]' && c.body?.includes(START_MARKER)
    );

    if (existingFallback) {
      await github.rest.issues.updateComment({
        owner: context.repo.owner,
        repo: context.repo.repo,
        comment_id: existingFallback.id,
        body: linksSection,
      });
    } else {
      await github.rest.issues.createComment({
        owner: context.repo.owner,
        repo: context.repo.repo,
        issue_number: context.issue.number,
        body: linksSection,
      });
    }
    return;
  }

  const newBody = upsertLinksSection(deploymentComment.body, linksSection);

  await github.rest.issues.updateComment({
    owner: context.repo.owner,
    repo: context.repo.repo,
    comment_id: deploymentComment.id,
    body: newBody,
  });
}

module.exports = { run, listMdxPages };
