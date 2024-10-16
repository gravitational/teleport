import { Volume, createFsFromVolume } from 'memfs';
import { RedirectChecker } from './check-redirects.js';

describe('check files for links to missing Teleport docs', () => {
  const files = {
    '/blog/content1.mdx': `---
title: "Sample Page 1"
---

This is a link to a [documentation page](https://goteleport.com/docs/page1).

This is a link to the [index page](https://goteleport.com/docs/).

This link has a [trailing slash](https://goteleport.com/docs/desktop-access/getting-started/).

This link has a [fragment](https://goteleport.com/docs/page1#introduction).

`,
    '/blog/content2.mdx': `---
title: "Sample Page 2"
---

This is a link to a [documentation page](https://goteleport.com/docs/subdirectory/page2).

Here is a link to a [missing page](https://goteleport.com/docs/page3).

Here is a link to a [missing page](https://goteleport.com/docs/page3).

`,
    '/docs/content/1.x/docs/pages/page1.mdx': `---
title: "Sample Page 1"
---
`,
    '/docs/content/1.x/docs/pages/subdirectory/page2.mdx': `---
title: "Sample Page 2"
---
`,
    '/docs/content/1.x/docs/pages/index.mdx': `---
title: "Index page"
---
`,
    '/docs/content/1.x/docs/pages/desktop-access/getting-started.mdx': `---
title: "Desktop Access Getting Started"
---`,
  };

  test(`throws an error if there is no redirect for a missing docs page`, () => {
    const vol = Volume.fromJSON(files);
    const fs = createFsFromVolume(vol);
    const checker = new RedirectChecker(fs, '/blog', '/docs/content/1.x', []);
    const results = checker.check();
    expect(results).toEqual(['https://goteleport.com/docs/page3']);
  });

  test(`handles URL fragments`, () => {
    const vol = Volume.fromJSON(files);
    const fs = createFsFromVolume(vol);
    const checker = new RedirectChecker(fs, '/blog', '/docs/content/1.x', []);
    const results = checker.check();
    expect(results).toEqual(['https://goteleport.com/docs/page3']);
  });

  test(`handles trailing slashes in URLs`, () => {
    const vol = Volume.fromJSON(files);
    const fs = createFsFromVolume(vol);
    const checker = new RedirectChecker(fs, '/blog', '/docs/content/1.x', []);
    const results = checker.check();
    expect(results).toEqual(['https://goteleport.com/docs/page3']);
  });

  test(`allows missing docs pages if there is a redirect`, () => {
    const vol = Volume.fromJSON(files);
    const fs = createFsFromVolume(vol);
    const checker = new RedirectChecker(fs, '/blog', '/docs/content/1.x', [
      {
        source: '/page3/',
        destination: '/page1/',
        permanent: true,
      },
    ]);
    const results = checker.check();
    expect(results).toEqual([]);
  });

  test(`allows missing docs pages for links with fragments if there is a redirect`, () => {
    const vol = Volume.fromJSON({
      '/blog/content2.mdx': `---
title: "Sample Page 2"
---

Here is a link to a [missing page](https://goteleport.com/docs/page3/#my-fragment).

`,
      '/docs/content/1.x/docs/pages/page1.mdx': `---
title: "Sample Page 1"
---
`,
    });
    const fs = createFsFromVolume(vol);
    const checker = new RedirectChecker(fs, '/blog', '/docs/content/1.x', [
      {
        source: '/page3/',
        destination: '/page1/',
        permanent: true,
      },
    ]);
    const results = checker.check();
    expect(results).toEqual([]);
  });

  test(`catches missing redirects for URLs with query strings`, () => {
    const vol = Volume.fromJSON({
      '/blog/content2.mdx': `---
title: "Sample Page 2"
---

Here is a link to a [missing page](https://goteleport.com/docs/access-controls/device-trust/guide/?scope=enterprise#step-12-register-a-trusted-device").

`,
      '/docs/content/1.x/docs/pages/page1.mdx': `---
title: "Sample Page 1"
---
`,
    });
    const fs = createFsFromVolume(vol);
    const checker = new RedirectChecker(fs, '/blog', '/docs/content/1.x', []);
    const results = checker.check();
    expect(results).toEqual(['https://goteleport.com/docs/access-controls/device-trust/guide/']);
  });

  test(`allows a docs URL with a query string if there is a redirect`, () => {
    const vol = Volume.fromJSON({
      '/blog/content2.mdx': `---
title: "Sample Page 2"
---

Here is a link to a [missing page](https://goteleport.com/docs/access-controls/device-trust/guide/?scope=enterprise#step-12-register-a-trusted-device").

`,
      '/docs/content/1.x/docs/pages/page1.mdx': `---
title: "Sample Page 1"
---
`,
    });
    const fs = createFsFromVolume(vol);
    const checker = new RedirectChecker(fs, '/blog', '/docs/content/1.x', [
      {
        source: '/access-controls/device-trust/guide/',
        destination: '/access-controls/device-trust/',
        permanent: true,
      },
    ]);
    const results = checker.check();
    expect(results).toEqual([]);
  });

  test(`excluding file extensions`, () => {
    const vol = Volume.fromJSON({
      '/web/content1.mdx': `---
title: "Sample Page 1"
---

This is a link to a [documentation page](https://goteleport.com/docs/page1).

`,
      '/web/file.story.test.tsx.snap': `

https://goteleport.com/docs/page2

`,
      '/docs/content/1.x/docs/pages/page1.mdx': `---
title: "Sample Page 1"
---
`,
    });
    const fs = createFsFromVolume(vol);
    const checker = new RedirectChecker(
      fs,
      '/web',
      '/docs/content/1.x',
      [],
      ['.story.test.tsx.snap']
    );
    const results = checker.check();
    expect(results).toEqual([]);
  });

  test(`URL in sentence`, () => {
    const vol = Volume.fromJSON({
      '/web/content1.mdx': `---
title: "Sample Page 1"
---

 Learn [how Teleport works](https://goteleport.com/docs/page1/) and get started with Teleport today -https://goteleport.com/docs/.
`,
      '/docs/content/1.x/docs/pages/page1.mdx': `---
title: "Sample Page 1"
---
`,
      '/docs/content/1.x/docs/pages/index.mdx': `---
title: Docs Home"
---`,
    });
    const fs = createFsFromVolume(vol);
    const checker = new RedirectChecker(
      fs,
      '/web',
      '/docs/content/1.x',
      [],
      []
    );
    const results = checker.check();
    expect(results).toEqual([]);
  });
});
