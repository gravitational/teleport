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
  getChangedPaths,
  categorizeChangedPaths,
  matchedImagesInContent,
  computeOrphanedImages,
  buildAllPagePaths,
  buildPageEntries,
  composeLinksSection,
  upsertLinksSection,
  START_MARKER,
} = require('./preview-links.js');

// Recursively list every .mdx file under `dir`, skipping any subdirectory
// named "includes". Filesystem I/O, so it lives here rather than in the
// pure module, but it has no GitHub dependency and is covered by a test.
async function listMdxPages(dir) {
  const results = [];
  async function walk(d) {
    const entries = await fs.promises.readdir(d, { withFileTypes: true });
    for (const entry of entries) {
      const full = path.join(d, entry.name);
      if (entry.isDirectory()) {
        if (entry.name === 'includes') continue;
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
  const previewBranch = process.env.PREVIEW_BRANCH;
  if (!previewBranch) {
    core.warning(
      'No preview branch name available from amplify-preview step; skipping per-page comment.'
    );
    return;
  }

  const previewAppId = process.env.PREVIEW_APP_ID;
  if (!previewAppId) {
    core.warning(
      'No preview app ID available from amplify-preview step; skipping per-page comment.'
    );
    return;
  }

  const previewHost = `https://${previewBranch}.${previewAppId}.amplifyapp.com`;

  // --- Gather changed files from the PR (GitHub API) ---
  const { data: files } = await github.rest.pulls.listFiles({
    owner: context.repo.owner,
    repo: context.repo.repo,
    pull_number: context.issue.number,
    per_page: 100,
  });

  const changedPaths = getChangedPaths(files);
  const { directlyChangedPages, changedImages } = categorizeChangedPaths(changedPaths);

  // --- Map changed images to the pages that reference them (filesystem) ---
  const imageRefMap = new Map(); // mdx path -> Set<basename>
  const referencedImagePaths = new Set();

  if (changedImages.length > 0) {
    const allMdxPages = await listMdxPages('docs/pages');
    for (const mdxPath of allMdxPages) {
      const content = await fs.promises.readFile(mdxPath, 'utf8');
      for (const img of matchedImagesInContent(content, changedImages)) {
        if (!imageRefMap.has(mdxPath)) {
          imageRefMap.set(mdxPath, new Set());
        }
        imageRefMap.get(mdxPath).add(img.basename);
        referencedImagePaths.add(img.path);
      }
    }
  }

  const orphanedImages = computeOrphanedImages(changedImages, referencedImagePaths);
  const allPagePaths = buildAllPagePaths(directlyChangedPages, imageRefMap);
  const pageEntries = buildPageEntries(allPagePaths, imageRefMap, previewHost);
  const linksSection = composeLinksSection(pageEntries, orphanedImages);

  // --- Find the deployment-status comment to append to (GitHub API) ---
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
