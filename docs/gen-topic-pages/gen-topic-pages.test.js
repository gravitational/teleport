import { Volume, createFsFromVolume } from 'memfs';
import { TopicContentsFragment } from './gen-topic-pages.js';

describe('generate a menu page', () => {
  const testFilesTwoSections = {
    '/docs.mdx': `---
title: "Documentation Home"
description: "Guides to setting up the product."
---

Guides to setting up the product.

{/*TOPICS*/}
`,
    '/docs/database-access.mdx': `---
title: "Database Access"
description: "Guides related to Database Access."
---

Guides related to Database Access.

{/*TOPICS*/}
`,
    '/docs/database-access/page1.mdx': `---
title: "Database Access Page 1"
description: "Protecting DB 1 with Teleport"
---`,
    '/docs/database-access/page2.mdx': `---
title: "Database Access Page 2"
description: "Protecting DB 2 with Teleport"
---`,
    '/docs/application-access.mdx': `---
title: "Application Access"
description: "Guides related to Application Access"
---

Guides related to Application Access.

{/*TOPICS*/}
`,
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
title: "Database Access"
description: "Guides related to Database Access."
---

Guides related to Database Access.

{/*TOPICS*/}

- [Database Access Page 1](database-access/page1.mdx): Protecting DB 1 with Teleport
- [Database Access Page 2](database-access/page2.mdx): Protecting DB 2 with Teleport
`;

    const vol = Volume.fromJSON(testFilesTwoSections);
    const fs = createFsFromVolume(vol);
    const frag = new TopicContentsFragment(fs, '/docs/database-access');
    const actual = frag.makeTopicPage();
    expect(actual).toBe(expected);
  });

  test('one link to a directory', () => {
    const expected = `---
title: Documentation Home
description: Guides for setting up the product.
---

Guides for setting up the product.

{/*TOPICS*/}

## Application Access

Guides related to Application Access ([more info](docs/application-access.mdx))

- [Application Access Page 1](docs/application-access/page1.mdx): Protecting App 1 with Teleport
- [Application Access Page 2](docs/application-access/page2.mdx): Protecting App 2 with Teleport
`;

    const vol = Volume.fromJSON({
      '/docs.mdx': `---
title: Documentation Home
description: Guides for setting up the product.
---

Guides for setting up the product.

{/*TOPICS*/}
`,
      '/docs/application-access.mdx': `---
title: "Application Access"
description: "Guides related to Application Access"
---

{/*TOPICS*/}
`,
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
    const frag = new TopicContentsFragment(fs, '/docs');
    const actual = frag.makeTopicPage();
    expect(actual).toBe(expected);
  });

  test('multiple links to directories', () => {
    const expected = `---
title: "Documentation Home"
description: "Guides to setting up the product."
---

Guides to setting up the product.

{/*TOPICS*/}

## Application Access

Guides related to Application Access ([more info](docs/application-access.mdx))

- [Application Access Page 1](docs/application-access/page1.mdx): Protecting App 1 with Teleport
- [Application Access Page 2](docs/application-access/page2.mdx): Protecting App 2 with Teleport

## Database Access

Guides related to Database Access. ([more info](docs/database-access.mdx))

- [Database Access Page 1](docs/database-access/page1.mdx): Protecting DB 1 with Teleport
- [Database Access Page 2](docs/database-access/page2.mdx): Protecting DB 2 with Teleport
`;

    const vol = Volume.fromJSON(testFilesTwoSections);
    const fs = createFsFromVolume(vol);
    const frag = new TopicContentsFragment(fs, '/docs');
    const actual = frag.makeTopicPage();
    expect(actual).toBe(expected);
  });

  test('limits menus to one directory level', () => {
    const expected = `---
title: Documentation Home
description: Guides to setting up the product.
---

Guides to setting up the product.

{/*TOPICS*/}

## Application Access

Guides related to Application Access ([more info](docs/application-access.mdx))

- [Application Access Page 1](docs/application-access/page1.mdx): Protecting App 1 with Teleport
- [Application Access Page 2](docs/application-access/page2.mdx): Protecting App 2 with Teleport
- [JWT Guides (section)](docs/application-access/jwt.mdx): Guides related to JWTs
`;

    const vol = Volume.fromJSON({
      '/docs.mdx': `---
title: Documentation Home
description: Guides to setting up the product.
---

Guides to setting up the product.

{/*TOPICS*/}
`,
      '/docs/application-access.mdx': `---
title: "Application Access"
description: "Guides related to Application Access"
---

{/*TOPICS*/}
`,
      '/docs/application-access/page1.mdx': `---
title: "Application Access Page 1"
description: "Protecting App 1 with Teleport"
---`,
      '/docs/application-access/page2.mdx': `---
title: "Application Access Page 2"
description: "Protecting App 2 with Teleport"
---`,
      '/docs/application-access/jwt.mdx': `---
title: "JWT Guides"
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
    const frag = new TopicContentsFragment(fs, '/docs');
    const actual = frag.makeTopicPage();
    expect(actual).toBe(expected);
  });

  test(`throws an error if a root menu page does not have "TOPICS" delimiter`, () => {
    const vol = Volume.fromJSON({
      '/docs.mdx': `---
title: "Documentation Home"
description: "Guides to setting up the product."
`,
      '/docs/application-access.mdx': `---
title: "Application Access"
description: "Guides related to Application Access"
---`,
    });

    const fs = createFsFromVolume(vol);
    expect(() => {
      const frag = new TopicContentsFragment(fs, '/docs');
      frag.makeTopicPage();
    }).toThrow('TOPICS');
  });

  test(`throws an error on a generated menu page that does not correspond to a subdirectory`, () => {
    const vol = Volume.fromJSON({
      '/docs.mdx': `---
title: "Documentation Home"
description: "Guides to setting up the product."
---

{/*TOPICS*/}
`,
      '/docs/application-access.mdx': `---
title: "Application Access"
description: "Guides related to Application Access"
---

{/*TOPICS*/}
`,
      '/docs/application-access/page1.mdx': `---
title: "Application Access Page 1"
description: "Protecting App 1 with Teleport"
---`,
      '/docs/jwt.mdx': `---
title: "JWT guides"
description: "Guides related to JWTs"
---

{/*TOPICS*/}
`,
    });

    const fs = createFsFromVolume(vol);
    expect(() => {
      const frag = new TopicContentsFragment(fs, '/docs');
      frag.makeTopicPage();
    }).toThrow('jwt.mdx');
  });

  test('overwrites topics rather than append to them', () => {
    const expected = `---
title: "Database Access"
description: "Guides related to Database Access."
---

Guides related to Database Access.

{/*TOPICS*/}

- [Database Access Page 1](database-access/page1.mdx): Protecting DB 1 with Teleport
- [Database Access Page 2](database-access/page2.mdx): Protecting DB 2 with Teleport
`;

    const vol = Volume.fromJSON(testFilesTwoSections);
    const fs = createFsFromVolume(vol);
    const frag = new TopicContentsFragment(fs, '/docs/database-access');
    fs.writeFileSync('/docs/database-access.mdx', frag.makeTopicPage());
    const actual = frag.makeTopicPage();
    expect(actual).toBe(expected);
  });

  test('orders sections correctly', () => {
    const expected = `---
title: Documentation Home
description: Guides to setting up the product.
---

Guides to setting up the product.

{/*TOPICS*/}

- [API Usage](docs/api.mdx): Using the API.
- [Initial Setup](docs/initial-setup.mdx): How to set up the product for the first time.
- [Kubernetes](docs/kubernetes.mdx): A guide related to Kubernetes.

## Application Access

Guides related to Application Access ([more info](docs/application-access.mdx))

- [Application Access Page 1](docs/application-access/page1.mdx): Protecting App 1 with Teleport

## Desktop Access

Guides related to Desktop Access ([more info](docs/desktop-access.mdx))

- [Get Started](docs/desktop-access/get-started.mdx): Get started with desktop access.
`;

    const vol = Volume.fromJSON({
      '/docs.mdx': `---
title: Documentation Home
description: Guides to setting up the product.
---

Guides to setting up the product.

{/*TOPICS*/}
`,
      '/docs/desktop-access.mdx': `---
title: "Desktop Access"
description: "Guides related to Desktop Access"
---

{/*TOPICS*/}
`,

      '/docs/application-access.mdx': `---
title: "Application Access"
description: "Guides related to Application Access"
---

{/*TOPICS*/}
`,
      '/docs/desktop-access/get-started.mdx': `---
title: "Get Started"
description: "Get started with desktop access."
---`,
      '/docs/application-access/page1.mdx': `---
title: "Application Access Page 1"
description: "Protecting App 1 with Teleport"
---`,
      '/docs/kubernetes.mdx': `---
title: "Kubernetes"
description: "A guide related to Kubernetes."
---`,

      '/docs/initial-setup.mdx': `---
title: "Initial Setup"
description: "How to set up the product for the first time."
---`,
      '/docs/api.mdx': `---
title: "API Usage"
description: "Using the API."
---`,
    });
    const fs = createFsFromVolume(vol);
    const frag = new TopicContentsFragment(fs, '/docs');
    const actual = frag.makeTopicPage();
    expect(actual).toBe(expected);
  });
});
