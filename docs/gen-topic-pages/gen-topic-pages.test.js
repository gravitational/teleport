import { Volume, createFsFromVolume } from 'memfs';
import { TopicContentsFragment } from './gen-topic-pages.js';

describe('generate a menu page', () => {
  const testFilesTwoSections = {
    '/docs.yaml': `---
title: "Documentation Home"
description: "Guides to setting up the product."
`,
    '/docs/database-access.yaml': `---
title: "Database Access"
description: "Guides related to Database Access."
---`,
    '/docs/database-access/page1.mdx': `---
title: "Database Access Page 1"
description: "Protecting DB 1 with Teleport"
---`,
    '/docs/database-access/page2.mdx': `---
title: "Database Access Page 2"
description: "Protecting DB 2 with Teleport"
---`,
    '/docs/application-access.yaml': `---
title: "Application Access"
description: "Guides related to Application Access"
---`,
    '/docs/application-access/page1.mdx': `---
title: "Application Access Page 1"
description: "Protecting App 1 with Teleport"
---`,
    '/docs/application-access/page2.mdx': `---
title: "Application Access Page 2"
description: "Protecting App 2 with Teleport"
---`,
  };

  test('lists the contents of a directory', () => {
    const expected = `---
title: Database Access
description: Guides related to Database Access.
---

{/*GENERATED MENU PAGE. DO NOT EDIT. RECREATE WITH THIS COMMAND:
sample-command*/}

Guides related to Database Access.

- [Database Access Page 1](database-access/page1.mdx): Protecting DB 1 with Teleport
- [Database Access Page 2](database-access/page2.mdx): Protecting DB 2 with Teleport
`;

    const vol = Volume.fromJSON(testFilesTwoSections);
    const fs = createFsFromVolume(vol);
    const frag = new TopicContentsFragment(
      'sample-command',
      fs,
      '/docs/database-access'
    );
    const actual = frag.makeTopicPage();
    expect(actual).toBe(expected);
  });

  test('handles frontmatter document separators', () => {
    const expected = `---
title: Database Access
description: Guides related to Database Access.
---

{/*GENERATED MENU PAGE. DO NOT EDIT. RECREATE WITH THIS COMMAND:
sample-command*/}

Guides related to Database Access.

- [Database Access Page 1](database-access/page1.mdx): Protecting DB 1 with Teleport
- [Database Access Page 2](database-access/page2.mdx): Protecting DB 2 with Teleport
`;

    const vol = Volume.fromJSON({
      '/docs/database-access.yaml': `---
title: Database Access
description: Guides related to Database Access.
---`,
      '/docs/database-access/page1.mdx': `---
title: "Database Access Page 1"
description: "Protecting DB 1 with Teleport"
---`,
      '/docs/database-access/page2.mdx': `---
title: "Database Access Page 2"
description: "Protecting DB 2 with Teleport"
---`,
    });
    const fs = createFsFromVolume(vol);
    const frag = new TopicContentsFragment(
      'sample-command',
      fs,
      '/docs/database-access'
    );
    const actual = frag.makeTopicPage();
    expect(actual).toBe(expected);
  });

  test('treats links to directories as regular links (single)', () => {
    const expected = `---
title: Documentation Home
description: Guides for setting up the product.
---

{/*GENERATED MENU PAGE. DO NOT EDIT. RECREATE WITH THIS COMMAND:
sample-command*/}

Guides for setting up the product.

- [Application Access](docs/application-access.mdx): Guides related to Application Access
`;

    const vol = Volume.fromJSON({
      '/docs.yaml': `---
title: Documentation Home
description: Guides for setting up the product.
---`,
      '/docs/application-access.yaml': `---
title: "Application Access"
description: "Guides related to Application Access"
---`,
      '/docs/application-access/page1.mdx': `---
title: "Application Access Page 1"
description: "Protecting App 1 with Teleport"
---`,
      '/docs/application-access/page2.mdx': `---
title: "Application Access Page 2"
description: "Protecting App 2 with Teleport"
---`,
    });
    const fs = createFsFromVolume(vol);
    const frag = new TopicContentsFragment('sample-command', fs, '/docs');
    const actual = frag.makeTopicPage();
    expect(actual).toBe(expected);
  });

  test('treats links to directories as regular links (multiple)', () => {
    const expected = `---
title: Documentation Home
description: Guides to setting up the product.
---

{/*GENERATED MENU PAGE. DO NOT EDIT. RECREATE WITH THIS COMMAND:
sample-command*/}

Guides to setting up the product.

- [Application Access](docs/application-access.mdx): Guides related to Application Access
- [Database Access](docs/database-access.mdx): Guides related to Database Access.
`;

    const vol = Volume.fromJSON(testFilesTwoSections);
    const fs = createFsFromVolume(vol);
    const frag = new TopicContentsFragment('sample-command', fs, '/docs');
    const actual = frag.makeTopicPage();
    expect(actual).toBe(expected);
  });

  test('limits menus to one directory level', () => {
    const expected = `---
title: Documentation Home
description: Guides to setting up the product.
---

{/*GENERATED MENU PAGE. DO NOT EDIT. RECREATE WITH THIS COMMAND:
sample-command*/}

Guides to setting up the product.

- [Application Access](docs/application-access.mdx): Guides related to Application Access
`;

    const vol = Volume.fromJSON({
      '/docs.yaml': `---
title: "Documentation Home"
description: "Guides to setting up the product."
`,
      '/docs/application-access.yaml': `---
title: "Application Access"
description: "Guides related to Application Access"
---`,
      '/docs/application-access/page1.mdx': `---
title: "Application Access Page 1"
description: "Protecting App 1 with Teleport"
---`,
      '/docs/application-access/page2.mdx': `---
title: "Application Access Page 2"
description: "Protecting App 2 with Teleport"
---`,
      '/docs/application-access/jwt.yaml': `---
title: "JWT guides"
description: "Guides related to JWTs"
---`,
      '/docs/application-access/jwt/page1.mdx': `---
title: "JWT Page 1"
description: "Protecting JWT App 1 with Teleport"
---`,
      '/docs/application-access/jwt/page2.mdx': `---
title: "JWT Page 2"
description: "Protecting JWT App 2 with Teleport"
---`,
    });
    const fs = createFsFromVolume(vol);
    const frag = new TopicContentsFragment('sample-command', fs, '/docs');
    const actual = frag.makeTopicPage();
    expect(actual).toBe(expected);
  });

  test(`throws an error for YAML configs that don't correspond to subdirectory names`, () => {
    const vol = Volume.fromJSON({
      '/docs.yaml': `---
title: "Documentation Home"
description: "Guides to setting up the product."
`,
      '/docs/application-access.yaml': `---
title: "Application Access"
description: "Guides related to Application Access"
---`,
      '/docs/application-access/page1.mdx': `---
title: "Application Access Page 1"
description: "Protecting App 1 with Teleport"
---`,
      '/docs/jwt.yaml': `---
title: "JWT guides"
description: "Guides related to JWTs"
---`,
    });

    const fs = createFsFromVolume(vol);
    expect(() => {
      const frag = new TopicContentsFragment('sample-command', fs, '/docs');
      frag.makeTopicPage();
    }).toThrow('jwt.yaml');
  });

  test(`throws an error on a generated menu page that does not correspond to a subdirectory`, () => {
    const vol = Volume.fromJSON({
      '/docs.yaml': `---
title: "Documentation Home"
description: "Guides to setting up the product."
`,
      '/docs/application-access.yaml': `---
title: "Application Access"
description: "Guides related to Application Access"
---`,
      '/docs/application-access/page1.mdx': `---
title: "Application Access Page 1"
description: "Protecting App 1 with Teleport"
---`,
      '/docs/jwt.mdx': `---
title: "JWT guides"
description: "Guides related to JWTs"
---
{/*GENERATED MENU PAGE. DO NOT EDIT. RECREATE WITH THIS COMMAND:
sample-command*/}
`,
    });

    const fs = createFsFromVolume(vol);
    expect(() => {
      const frag = new TopicContentsFragment('sample-command', fs, '/docs');
      frag.makeTopicPage();
    }).toThrow('jwt.mdx');
  });

  test(`throws an error on a menu page that was not auto-generated`, () => {
    const vol = Volume.fromJSON({
      '/docs.yaml': `---
title: "Documentation Home"
description: "Guides to setting up the product."
`,
      '/docs/application-access.yaml': `---
title: "Application Access"
description: "Guides related to Application Access"
---`,
      '/docs/application-access/page1.mdx': `---
title: "Application Access Page 1"
description: "Protecting App 1 with Teleport"
---`,
      '/docs/application-access.mdx': `---
title: "JWT guides"
description: "Guides related to JWTs"
---
This menu page was written manually.
`,
    });

    const fs = createFsFromVolume(vol);
    expect(() => {
      const frag = new TopicContentsFragment('sample-command', fs, '/docs');
      frag.makeTopicPage();
    }).toThrow('application-access.mdx');
  });
});
