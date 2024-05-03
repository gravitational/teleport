import { Volume, createFsFromVolume } from 'memfs';
import { TopicContentsFragment } from './gen-topic-pages.js';

describe('generate All Topics page', () => {
  const testFilesTwoSections = {
    '/docs/database-access.mdx': `title: "Database Access"
description: "Guides related to Database Access"
---`,
    '/docs/database-access/page1.mdx': `title: "Database Access Page 1"
description: "Protecting DB 1 with Teleport"
---`,
    '/docs/database-access/page2.mdx': `title: "Database Access Page 2"
description: "Protecting DB 2 with Teleport"
---`,
    '/docs/application-access.mdx': `title: "Application Access"
description: "Guides related to Application Access"
---`,
    '/docs/application-access/page1.mdx': `title: "Application Access Page 1"
description: "Protecting App 1 with Teleport"
---`,
    '/docs/application-access/page2.mdx': `title: "Application Access Page 2"
description: "Protecting App 2 with Teleport"
---`,
  };

  test('lists the contents of a directory', () => {
    const expected = `{/*GENERATED PARTIAL. DO NOT EDIT. RECREATE WITH THIS COMMAND:
sample-command*/}

|Topic|Description|
|---|---|
|[Database Access Page 1](../page1.mdx)|Protecting DB 1 with Teleport|
|[Database Access Page 2](../page2.mdx)|Protecting DB 2 with Teleport|
`;

    const vol = Volume.fromJSON(testFilesTwoSections);
    const fs = createFsFromVolume(vol);
    const frag = new TopicContentsFragment(
      'sample-command',
      fs,
      '/docs/database-access',
      '..'
    );
    const actual = frag.makeTopicTree();
    expect(actual).toBe(expected);
  });

  test('handles frontmatter document separators', () => {
    const expected = `{/*GENERATED PARTIAL. DO NOT EDIT. RECREATE WITH THIS COMMAND:
sample-command*/}

|Topic|Description|
|---|---|
|[Database Access Page 1](../page1.mdx)|Protecting DB 1 with Teleport|
|[Database Access Page 2](../page2.mdx)|Protecting DB 2 with Teleport|
`;

    const vol = Volume.fromJSON({
      '/docs/database-access.mdx': `title: "Database Access"
description: "Guides related to Database Access"
---`,
      '/docs/database-access/page1.mdx': `---
title: "Database Access Page 1"
description: "Protecting DB 1 with Teleport"
---`,
      '/docs/database-access/page2.mdx': `title: "Database Access Page 2"
description: "Protecting DB 2 with Teleport"
---`,
    });
    const fs = createFsFromVolume(vol);
    const frag = new TopicContentsFragment(
      'sample-command',
      fs,
      '/docs/database-access',
      '..'
    );
    const actual = frag.makeTopicTree();
    expect(actual).toBe(expected);
  });

  test('ignores files called "all-topics.mdx"', () => {
    const expected = `{/*GENERATED PARTIAL. DO NOT EDIT. RECREATE WITH THIS COMMAND:
sample-command*/}

|Topic|Description|
|---|---|
|[Introduction](../introduction.mdx)|Protecting databases with Teleport|
|[Database Access Page 1](../page1.mdx)|Protecting DB 1 with Teleport|
|[Database Access Page 2](../page2.mdx)|Protecting DB 2 with Teleport|
`;

    const vol = Volume.fromJSON({
      '/docs/database-access.mdx': `title: "Database Access"
description: "Guides related to Database Access"
---`,
      '/docs/database-access/introduction.mdx': `title: "Introduction"
description: Protecting databases with Teleport
---`,
      '/docs/database-access/all-topics.mdx': `title: All Topics
description: "The All Topics page"
---`,
      '/docs/database-access/page1.mdx': `title: "Database Access Page 1"
description: "Protecting DB 1 with Teleport"
---`,
      '/docs/database-access/page2.mdx': `title: "Database Access Page 2"
description: "Protecting DB 2 with Teleport"
---`,
    });
    const fs = createFsFromVolume(vol);
    const frag = new TopicContentsFragment(
      'sample-command',
      fs,
      '/docs/database-access',
      '..'
    );
    const actual = frag.makeTopicTree();
    expect(actual).toBe(expected);
  });

  test('adds section headings if the root includes directories (single)', () => {
    const expected = `{/*GENERATED PARTIAL. DO NOT EDIT. RECREATE WITH THIS COMMAND:
sample-command*/}

## Application Access

Guides related to Application Access ([more info](../application-access.mdx))

|Topic|Description|
|---|---|
|[Application Access Page 1](../application-access/page1.mdx)|Protecting App 1 with Teleport|
|[Application Access Page 2](../application-access/page2.mdx)|Protecting App 2 with Teleport|
`;

    const vol = Volume.fromJSON({
      '/docs/application-access.mdx': `title: "Application Access"
description: "Guides related to Application Access"
---`,
      '/docs/application-access/page1.mdx': `title: "Application Access Page 1"
description: "Protecting App 1 with Teleport"
---`,
      '/docs/application-access/page2.mdx': `title: "Application Access Page 2"
description: "Protecting App 2 with Teleport"
---`,
    });
    const fs = createFsFromVolume(vol);
    const frag = new TopicContentsFragment('sample-command', fs, '/docs', '..');
    const actual = frag.makeTopicTree();
    expect(actual).toBe(expected);
  });

  test('adds section headings if the root includes directories (multiple)', () => {
    const expected = `{/*GENERATED PARTIAL. DO NOT EDIT. RECREATE WITH THIS COMMAND:
sample-command*/}

## Application Access

Guides related to Application Access ([more info](../application-access.mdx))

|Topic|Description|
|---|---|
|[Application Access Page 1](../application-access/page1.mdx)|Protecting App 1 with Teleport|
|[Application Access Page 2](../application-access/page2.mdx)|Protecting App 2 with Teleport|

## Database Access

Guides related to Database Access ([more info](../database-access.mdx))

|Topic|Description|
|---|---|
|[Database Access Page 1](../database-access/page1.mdx)|Protecting DB 1 with Teleport|
|[Database Access Page 2](../database-access/page2.mdx)|Protecting DB 2 with Teleport|
`;

    const vol = Volume.fromJSON(testFilesTwoSections);
    const fs = createFsFromVolume(vol);
    const frag = new TopicContentsFragment('sample-command', fs, '/docs', '..');
    const actual = frag.makeTopicTree();
    expect(actual).toBe(expected);
  });

  test('nested directories', () => {
    const expected = `{/*GENERATED PARTIAL. DO NOT EDIT. RECREATE WITH THIS COMMAND:
sample-command*/}

## Application Access

Guides related to Application Access ([more info](../application-access.mdx))

|Topic|Description|
|---|---|
|[Application Access Page 1](../application-access/page1.mdx)|Protecting App 1 with Teleport|
|[Application Access Page 2](../application-access/page2.mdx)|Protecting App 2 with Teleport|

### JWT guides

Guides related to JWTs ([more info](../application-access/jwt.mdx))

|Topic|Description|
|---|---|
|[JWT Page 1](../application-access/jwt/page1.mdx)|Protecting JWT App 1 with Teleport|
|[JWT Page 2](../application-access/jwt/page2.mdx)|Protecting JWT App 2 with Teleport|
`;

    const vol = Volume.fromJSON({
      '/docs/application-access.mdx': `title: "Application Access"
description: "Guides related to Application Access"
---`,
      '/docs/application-access/page1.mdx': `title: "Application Access Page 1"
description: "Protecting App 1 with Teleport"
---`,
      '/docs/application-access/page2.mdx': `title: "Application Access Page 2"
description: "Protecting App 2 with Teleport"
---`,
      '/docs/application-access/jwt.mdx': `title: "JWT guides"
description: "Guides related to JWTs"
---`,
      '/docs/application-access/jwt/page1.mdx': `title: "JWT Page 1"
description: "Protecting JWT App 1 with Teleport"
---`,
      '/docs/application-access/jwt/page2.mdx': `title: "JWT Page 2"
description: "Protecting JWT App 2 with Teleport"
---`,
    });
    const fs = createFsFromVolume(vol);
    const frag = new TopicContentsFragment('sample-command', fs, '/docs', '..');
    const actual = frag.makeTopicTree();
    expect(actual).toBe(expected);
  });
});
