// Tests for composeLinksSection's size caps. These can live in this separate
// file or be merged into test-preview-links.js — `node --test` picks up both.
// Named test-*.js (not *.test.js) so Jest's default testMatch skips it; see
// the note in preview-links.js.

const test = require('node:test');
const assert = require('node:assert');

const {
  composeLinksSection,
  buildPageEntries,
  upsertLinksSection,
  END_MARKER,
  MAX_SECTION_LENGTH,
} = require('./preview-links.js');

const HOST = 'https://my-branch.d123abc456.amplifyapp.com';
const GITHUB_COMMENT_LIMIT = 65536;

// Synthetic page paths long enough to reproduce the docs-sweep scenario
// (hundreds of rendered pages -> a section far larger than the comment limit
// without the cap).
function makePagePaths(n) {
  return Array.from(
    { length: n },
    (_, i) =>
      'docs/pages/enroll-resources/database-access/enroll-managed-databases/' +
      `some-fairly-long-guide-name-${String(i).padStart(4, '0')}.mdx`
  );
}

test('caps a full-corpus sweep below the GitHub comment limit', () => {
  const entries = buildPageEntries(makePagePaths(800), HOST);
  // Sanity-check the premise: uncapped, this would exceed the limit.
  assert.ok(entries.join('\n').length > GITHUB_COMMENT_LIMIT);

  const section = composeLinksSection(entries, [], [], { previewHost: HOST });

  assert.ok(
    section.length <= MAX_SECTION_LENGTH,
    `section is ${section.length} chars, over the ${MAX_SECTION_LENGTH} cap`
  );
  assert.match(section, /…and \d+ more changed pages not listed here/);
  assert.ok(section.includes(`${HOST}/docs/`), 'summary should link to the preview root');
  assert.ok(section.trimEnd().endsWith(END_MARKER), 'section must stay well-formed');
});

test('caps oversized orphan lists too', () => {
  const orphanedImages = Array.from(
    { length: 500 },
    (_, i) => `docs/img/some/deeply/nested/dir/screenshot-${String(i).padStart(4, '0')}.png`
  );
  const orphanedPartials = Array.from(
    { length: 500 },
    (_, i) => `docs/pages/includes/database-access/partial-${String(i).padStart(4, '0')}.mdx`
  );

  const section = composeLinksSection([], orphanedImages, orphanedPartials);

  assert.ok(section.length <= MAX_SECTION_LENGTH);
  assert.match(section, /…and \d+ more_/);
  assert.ok(section.trimEnd().endsWith(END_MARKER));
});

test('upserting a capped worst-case section keeps the whole comment under the limit', () => {
  const deploymentCommentBody =
    'Amplify deployment status\n\n' +
    `| App | Status |\n|---|---|\n| [${HOST}](${HOST}) | SUCCEED |\n` +
    'x'.repeat(2000); // generous stand-in for the rest of the Amplify comment

  const entries = buildPageEntries(makePagePaths(800), HOST);
  const section = composeLinksSection(entries, [], [], { previewHost: HOST });
  const newBody = upsertLinksSection(deploymentCommentBody, section);

  assert.ok(
    newBody.length < GITHUB_COMMENT_LIMIT,
    `combined comment is ${newBody.length} chars, over GitHub's ${GITHUB_COMMENT_LIMIT} limit`
  );
});

test('leaves small lists untouched (no summary line, no behavior change)', () => {
  const paths = makePagePaths(5);
  const entries = buildPageEntries(paths, HOST);
  const section = composeLinksSection(entries, [], [], { previewHost: HOST });

  for (const p of paths) {
    assert.ok(section.includes(p), `expected ${p} in section`);
  }
  assert.doesNotMatch(section, /more changed page/);
});

test('backward compatible when called without options', () => {
  const entries = buildPageEntries(makePagePaths(3), HOST);
  const section = composeLinksSection(entries);
  assert.ok(section.trimEnd().endsWith(END_MARKER));
  assert.doesNotMatch(section, /more changed page/);
});
