// preview-links.test.js
//
// Run with:  node --test
// (No test framework to install — this uses Node's built-in `node:test`.)

const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const {
  START_MARKER,
  END_MARKER,
  pageUrl,
  getChangedPaths,
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
} = require('./preview-links.js');

const { listMdxPages } = require('./post-preview-links.js');

const HOST = 'https://branch.d123.amplifyapp.com';

// --- pageUrl ---------------------------------------------------------------

test('pageUrl: a normal nested page', () => {
  assert.equal(
    pageUrl(HOST, 'docs/pages/admin-guides/foo.mdx'),
    `${HOST}/docs/admin-guides/foo/`
  );
});

test('pageUrl: top-level index maps to /docs/', () => {
  assert.equal(pageUrl(HOST, 'docs/pages/index.mdx'), `${HOST}/docs/`);
});

test('pageUrl: nested index maps to its parent directory', () => {
  assert.equal(
    pageUrl(HOST, 'docs/pages/admin-guides/index.mdx'),
    `${HOST}/docs/admin-guides/`
  );
});

test('pageUrl: deeply nested page', () => {
  assert.equal(
    pageUrl(HOST, 'docs/pages/a/b/c/d.mdx'),
    `${HOST}/docs/a/b/c/d/`
  );
});

test('pageUrl: section index page (dir/dir.mdx) maps to the directory URL', () => {
  assert.equal(
    pageUrl(HOST, 'docs/pages/enroll-resources/enroll-resources.mdx'),
    `${HOST}/docs/enroll-resources/`
  );
});

test('pageUrl: nested section index page maps to its directory URL', () => {
  assert.equal(
    pageUrl(HOST, 'docs/pages/admin-guides/management/management.mdx'),
    `${HOST}/docs/admin-guides/management/`
  );
});

test('pageUrl: a normal page is not mistaken for a section index', () => {
  // Parent dir and file name differ, so no collapsing should happen.
  assert.equal(
    pageUrl(HOST, 'docs/pages/enroll-resources/server-access.mdx'),
    `${HOST}/docs/enroll-resources/server-access/`
  );
});

// --- getChangedPaths -------------------------------------------------------

test('getChangedPaths: drops removed files, keeps the rest', () => {
  const files = [
    { status: 'modified', filename: 'docs/pages/a.mdx' },
    { status: 'added', filename: 'docs/pages/b.mdx' },
    { status: 'removed', filename: 'docs/pages/gone.mdx' },
    { status: 'renamed', filename: 'docs/pages/c.mdx' },
  ];
  assert.deepEqual(getChangedPaths(files), [
    'docs/pages/a.mdx',
    'docs/pages/b.mdx',
    'docs/pages/c.mdx',
  ]);
});

// --- categorizeChangedPaths ------------------------------------------------

test('categorizeChangedPaths: separates pages, images, and partials', () => {
  const changedPaths = [
    'docs/pages/foo.mdx', // page
    'docs/pages/includes/snippet.mdx', // partial (under includes)
    'docs/pages/guide/index.mdx', // page
    'docs/img/headless/approval.png', // image
    'docs/img/diagram.svg', // image
    'docs/img/notes.txt', // not an image
    'docs/config.yaml', // ignored entirely
  ];

  const { directlyChangedPages, changedImages, changedPartials } =
    categorizeChangedPaths(changedPaths);

  assert.deepEqual(
    [...directlyChangedPages].sort(),
    ['docs/pages/foo.mdx', 'docs/pages/guide/index.mdx']
  );

  assert.deepEqual(changedPartials, ['docs/pages/includes/snippet.mdx']);

  assert.deepEqual(changedImages, [
    {
      path: 'docs/img/headless/approval.png',
      basename: 'approval.png',
      matchSuffix: 'img/headless/approval.png',
    },
    {
      path: 'docs/img/diagram.svg',
      basename: 'diagram.svg',
      matchSuffix: 'img/diagram.svg',
    },
  ]);
});

test('categorizeChangedPaths: image extension match is case-insensitive', () => {
  const { changedImages } = categorizeChangedPaths(['docs/img/X.PNG']);
  assert.equal(changedImages.length, 1);
  assert.equal(changedImages[0].basename, 'X.PNG');
});

// --- matchedImagesInContent ------------------------------------------------

test('matchedImagesInContent: returns only images referenced in the content', () => {
  const changedImages = [
    { path: 'docs/img/a.png', basename: 'a.png', matchSuffix: 'img/a.png' },
    { path: 'docs/img/b.png', basename: 'b.png', matchSuffix: 'img/b.png' },
  ];
  const content = '![alt](../../img/a.png) and some prose';
  const matched = matchedImagesInContent(content, changedImages);
  assert.deepEqual(matched.map((i) => i.basename), ['a.png']);
});

test('matchedImagesInContent: empty when nothing matches', () => {
  const changedImages = [
    { path: 'docs/img/a.png', basename: 'a.png', matchSuffix: 'img/a.png' },
  ];
  assert.deepEqual(matchedImagesInContent('no images here', changedImages), []);
});

// --- computeOrphanedImages -------------------------------------------------

test('computeOrphanedImages: returns sorted images not referenced anywhere', () => {
  const changedImages = [
    { path: 'docs/img/z.png' },
    { path: 'docs/img/a.png' },
    { path: 'docs/img/m.png' },
  ];
  const referenced = new Set(['docs/img/m.png']);
  assert.deepEqual(computeOrphanedImages(changedImages, referenced), [
    'docs/img/a.png',
    'docs/img/z.png',
  ]);
});

// --- extractIncludePaths ---------------------------------------------------

test('extractIncludePaths: pulls .mdx partial paths from include directives', () => {
  const content = [
    'Some prose.',
    '(!docs/pages/includes/database-access/connection-timeout-troubleshooting.mdx!)',
    'More prose, then a code snippet include we ignore:',
    '(!examples/foo/main.go!)',
    '(! docs/pages/includes/spaced.mdx !)', // tolerate surrounding whitespace
  ].join('\n');

  assert.deepEqual(extractIncludePaths(content), [
    'docs/pages/includes/database-access/connection-timeout-troubleshooting.mdx',
    'docs/pages/includes/spaced.mdx',
  ]);
});

test('extractIncludePaths: handles parameterized includes (props after the path)', () => {
  const content =
    '(!docs/pages/includes/aws-credentials.mdx service="the Teleport Auth Service"!)';
  assert.deepEqual(extractIncludePaths(content), [
    'docs/pages/includes/aws-credentials.mdx',
  ]);
});

test('extractIncludePaths: handles multiple props on one include', () => {
  const content = '(!docs/pages/includes/foo.mdx service="x" region="us-east-1"!)';
  assert.deepEqual(extractIncludePaths(content), ['docs/pages/includes/foo.mdx']);
});

test('extractIncludePaths: normalizes a leading slash (absolute form)', () => {
  // Both forms appear in the docs and must resolve to the same repo-root path.
  const content = [
    '(!/docs/pages/includes/tctl.mdx!)',
    '(!docs/pages/includes/edition-prereqs-tabs.mdx!)',
  ].join('\n');
  assert.deepEqual(extractIncludePaths(content), [
    'docs/pages/includes/tctl.mdx',
    'docs/pages/includes/edition-prereqs-tabs.mdx',
  ]);
});

test('extractIncludePaths: returns empty array when there are no includes', () => {
  assert.deepEqual(extractIncludePaths('no includes here'), []);
});

// --- resolveAffectedPages --------------------------------------------------

test('resolveAffectedPages: maps a changed partial to the page that includes it', () => {
  const changedPartials = ['docs/pages/includes/db/timeout.mdx'];
  const includedBy = new Map([
    ['docs/pages/includes/db/timeout.mdx', new Set(['docs/pages/db/troubleshooting.mdx'])],
  ]);
  const affected = resolveAffectedPages(changedPartials, includedBy);
  assert.deepEqual(
    [...affected.keys()],
    ['docs/pages/db/troubleshooting.mdx']
  );
  assert.deepEqual(
    [...affected.get('docs/pages/db/troubleshooting.mdx')],
    ['docs/pages/includes/db/timeout.mdx']
  );
});

test('resolveAffectedPages: resolves through a nested partial (partial includes partial)', () => {
  const changedPartials = ['docs/pages/includes/inner.mdx'];
  const includedBy = new Map([
    // inner partial is included by an outer partial...
    ['docs/pages/includes/inner.mdx', new Set(['docs/pages/includes/outer.mdx'])],
    // ...and the outer partial is included by a real page.
    ['docs/pages/includes/outer.mdx', new Set(['docs/pages/guide/setup.mdx'])],
  ]);
  const affected = resolveAffectedPages(changedPartials, includedBy);
  assert.deepEqual([...affected.keys()], ['docs/pages/guide/setup.mdx']);
});

test('resolveAffectedPages: a partial included by nothing yields no pages', () => {
  const affected = resolveAffectedPages(
    ['docs/pages/includes/orphan.mdx'],
    new Map()
  );
  assert.equal(affected.size, 0);
});

test('resolveAffectedPages: guards against include cycles', () => {
  // a -> b -> a, with neither being a rendered page; should terminate.
  const includedBy = new Map([
    ['docs/pages/includes/a.mdx', new Set(['docs/pages/includes/b.mdx'])],
    ['docs/pages/includes/b.mdx', new Set(['docs/pages/includes/a.mdx'])],
  ]);
  const affected = resolveAffectedPages(['docs/pages/includes/a.mdx'], includedBy);
  assert.equal(affected.size, 0);
});

// --- computeOrphanedPartials -----------------------------------------------

test('computeOrphanedPartials: returns sorted partials that reach no page', () => {
  const changedPartials = [
    'docs/pages/includes/used.mdx',
    'docs/pages/includes/z-orphan.mdx',
    'docs/pages/includes/a-orphan.mdx',
  ];
  const affectedPages = new Map([
    ['docs/pages/guide.mdx', new Set(['docs/pages/includes/used.mdx'])],
  ]);
  assert.deepEqual(computeOrphanedPartials(changedPartials, affectedPages), [
    'docs/pages/includes/a-orphan.mdx',
    'docs/pages/includes/z-orphan.mdx',
  ]);
});

// --- buildAllPagePaths -----------------------------------------------------

test('buildAllPagePaths: merges direct, image, and partial pages, deduped and sorted', () => {
  const direct = new Set(['docs/pages/b.mdx', 'docs/pages/a.mdx']);
  const imageRefMap = new Map([
    ['docs/pages/a.mdx', new Set(['x.png'])], // also in `direct` -> deduped
    ['docs/pages/c.mdx', new Set(['y.png'])],
  ]);
  const partialRefMap = new Map([
    ['docs/pages/c.mdx', new Set(['p.mdx'])], // also via image -> deduped
    ['docs/pages/d.mdx', new Set(['q.mdx'])],
  ]);
  assert.deepEqual(buildAllPagePaths(direct, imageRefMap, partialRefMap), [
    'docs/pages/a.mdx',
    'docs/pages/b.mdx',
    'docs/pages/c.mdx',
    'docs/pages/d.mdx',
  ]);
});

// --- buildPageEntries ------------------------------------------------------

test('buildPageEntries: plain page with no annotation', () => {
  const entries = buildPageEntries(['docs/pages/foo.mdx'], HOST);
  assert.deepEqual(entries, [
    `- [\`docs/pages/foo.mdx\`](${HOST}/docs/foo/)`,
  ]);
});

test('buildPageEntries: single image change annotation', () => {
  const imageRefMap = new Map([['docs/pages/foo.mdx', new Set(['a.png'])]]);
  const entries = buildPageEntries(['docs/pages/foo.mdx'], HOST, { imageRefMap });
  assert.equal(
    entries[0],
    `- [\`docs/pages/foo.mdx\`](${HOST}/docs/foo/) — image change: \`a.png\``
  );
});

test('buildPageEntries: multiple image changes are sorted and pluralized', () => {
  const imageRefMap = new Map([
    ['docs/pages/foo.mdx', new Set(['b.png', 'a.png'])],
  ]);
  const entries = buildPageEntries(['docs/pages/foo.mdx'], HOST, { imageRefMap });
  assert.equal(
    entries[0],
    `- [\`docs/pages/foo.mdx\`](${HOST}/docs/foo/) — image changes: \`a.png\`, \`b.png\``
  );
});

test('buildPageEntries: affected-partial annotation', () => {
  const partialRefMap = new Map([['docs/pages/foo.mdx', new Set(['timeout.mdx'])]]);
  const entries = buildPageEntries(['docs/pages/foo.mdx'], HOST, { partialRefMap });
  assert.equal(
    entries[0],
    `- [\`docs/pages/foo.mdx\`](${HOST}/docs/foo/) — affected partial: \`timeout.mdx\``
  );
});

test('buildPageEntries: combined image and partial annotations', () => {
  const imageRefMap = new Map([['docs/pages/foo.mdx', new Set(['a.png'])]]);
  const partialRefMap = new Map([['docs/pages/foo.mdx', new Set(['p.mdx'])]]);
  const entries = buildPageEntries(['docs/pages/foo.mdx'], HOST, {
    imageRefMap,
    partialRefMap,
  });
  assert.equal(
    entries[0],
    `- [\`docs/pages/foo.mdx\`](${HOST}/docs/foo/) — image change: \`a.png\`; affected partial: \`p.mdx\``
  );
});

// --- composeLinksSection ---------------------------------------------------

test('composeLinksSection: wraps entries in markers and header', () => {
  const section = composeLinksSection(['- entry one'], []);
  assert.ok(section.startsWith(START_MARKER));
  assert.ok(section.endsWith(END_MARKER));
  assert.ok(section.includes('### Preview links for changed docs pages'));
  assert.ok(section.includes('- entry one'));
});

test('composeLinksSection: empty page list shows the "no pages" message', () => {
  const section = composeLinksSection([], []);
  assert.ok(section.includes('No docs pages changed in this PR.'));
});

test('composeLinksSection: includes an orphaned-images subsection when present', () => {
  const section = composeLinksSection(['- entry'], ['docs/img/orphan.png']);
  assert.ok(section.includes('#### Changed images with no associated page'));
  assert.ok(section.includes('- `docs/img/orphan.png`'));
});

test('composeLinksSection: includes an orphaned-partials subsection when present', () => {
  const section = composeLinksSection(['- entry'], [], ['docs/pages/includes/orphan.mdx']);
  assert.ok(section.includes('#### Changed partials not included by any page'));
  assert.ok(section.includes('- `docs/pages/includes/orphan.mdx`'));
});

// --- upsertLinksSection ----------------------------------------------------

test('upsertLinksSection: appends after a separator when no section exists', () => {
  const body = 'Amplify deployment status\nhttps://x.amplifyapp.com';
  const section = composeLinksSection(['- entry'], []);
  const result = upsertLinksSection(body, section);
  assert.ok(result.startsWith(body));
  assert.ok(result.includes('---'));
  assert.ok(result.includes(section));
});

test('upsertLinksSection: replaces an existing section in place (no duplication)', () => {
  const first = composeLinksSection(['- old entry'], []);
  const body = `Amplify deployment status\n\n---\n\n${first}`;
  const second = composeLinksSection(['- new entry'], []);
  const result = upsertLinksSection(body, second);

  assert.ok(result.includes('- new entry'));
  assert.ok(!result.includes('- old entry'));
  // Only one section should remain.
  const occurrences = result.split(START_MARKER).length - 1;
  assert.equal(occurrences, 1);
});

// --- listMdxPages (filesystem) ---------------------------------------------

test('listMdxPages: finds .mdx files recursively and skips "includes" dirs', async () => {
  const root = await fs.promises.mkdtemp(path.join(os.tmpdir(), 'mdx-test-'));
  try {
    await fs.promises.mkdir(path.join(root, 'guide'), { recursive: true });
    await fs.promises.mkdir(path.join(root, 'includes'), { recursive: true });

    await fs.promises.writeFile(path.join(root, 'top.mdx'), '# top');
    await fs.promises.writeFile(path.join(root, 'guide', 'nested.mdx'), '# nested');
    await fs.promises.writeFile(path.join(root, 'guide', 'notes.txt'), 'ignore me');
    await fs.promises.writeFile(path.join(root, 'includes', 'snippet.mdx'), '# excluded');

    const found = (await listMdxPages(root)).map((p) => path.relative(root, p)).sort();
    assert.deepEqual(found, [path.join('guide', 'nested.mdx'), 'top.mdx']);
  } finally {
    await fs.promises.rm(root, { recursive: true, force: true });
  }
});

test('listMdxPages: with skipIncludes=false, also lists partials under includes/', async () => {
  const root = await fs.promises.mkdtemp(path.join(os.tmpdir(), 'mdx-test-'));
  try {
    await fs.promises.mkdir(path.join(root, 'includes'), { recursive: true });
    await fs.promises.writeFile(path.join(root, 'top.mdx'), '# top');
    await fs.promises.writeFile(path.join(root, 'includes', 'snippet.mdx'), '# partial');

    const found = (await listMdxPages(root, { skipIncludes: false }))
      .map((p) => path.relative(root, p))
      .sort();
    assert.deepEqual(found, [path.join('includes', 'snippet.mdx'), 'top.mdx']);
  } finally {
    await fs.promises.rm(root, { recursive: true, force: true });
  }
});
