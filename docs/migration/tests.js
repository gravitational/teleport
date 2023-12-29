const { expectsToBe } = require('./lib/utility');
const { migrateFigures, migrateTabs, migrateTipAdmonitions, migrateNoteAdmonitions,
  migrateWarningAdmonitions, migrateTipNotices, migrateWarningNotices, migrateDetails,
  migrateVariables, migrateSnippets } = require('./lib/pages');

console.log('Testing component migrations...');

expectsToBe(
  'Figures',
  migrateFigures(`<Figure width="700">![Architecture of the setup you will complete in this guide](../img/linux-server-diagram.png)</Figure>`),
  `![Architecture of the setup you will complete in this guide](../img/linux-server-diagram.png)`
);

expectsToBe(
  'Tabs',
  migrateTabs(`<Tabs>
  <TabItem label="Public internet deployment with Let's Encrypt">
    Let's Encrypt verifies that you control the domain name of your Teleport cluster by communicating with the HTTPS server listening on port 443 of your Teleport Proxy Service.
  </TabItem>
  <TabItem label="Private network deployment">
    On your Teleport host, place a valid private key and a certificate chain in \`/var/lib/teleport/privkey.pem\` and \`/var/lib/teleport/fullchain.pem\` respectively.
  </TabItem>
</Tabs>`),
  `<Tabs>
  <Tab title="Public internet deployment with Let's Encrypt">
    Let's Encrypt verifies that you control the domain name of your Teleport cluster by communicating with the HTTPS server listening on port 443 of your Teleport Proxy Service.
  </Tab>
  <Tab title="Private network deployment">
    On your Teleport host, place a valid private key and a certificate chain in \`/var/lib/teleport/privkey.pem\` and \`/var/lib/teleport/fullchain.pem\` respectively.
  </Tab>
</Tabs>`
);

expectsToBe(
  'Tip Admonitions',
  migrateTipAdmonitions(`<Admonition type="tip" title="OS User Mappings">The users that you specify in the \`logins\` flag (e.g., \`root\`, \`ubuntu\` and \`ec2-user\` in our examples) must exist on your Linux host. Otherwise, you will get authentication errors later in this tutorial.</Admonition>`),
  `<Tip>The users that you specify in the \`logins\` flag (e.g., \`root\`, \`ubuntu\` and \`ec2-user\` in our examples) must exist on your Linux host. Otherwise, you will get authentication errors later in this tutorial.</Tip>`
)

expectsToBe(
  'Note Admonitions',
  migrateNoteAdmonitions(`<Admonition type="note">\`apt\`, \`yum\`, and \`zypper\` repos don't expose packages for all distribution variants. When following installation instructions, you might need to replace \`ID\` with \`ID_LIKE\` to install packages of the closest supported distribution.</Admonition>`),
  `<Note>\`apt\`, \`yum\`, and \`zypper\` repos don't expose packages for all distribution variants. When following installation instructions, you might need to replace \`ID\` with \`ID_LIKE\` to install packages of the closest supported distribution.</Note>`
)

expectsToBe(
  'Warning Admonitions',
  migrateWarningAdmonitions(`<Admonition type="warning" title="Preview">Login Rules are currently in Preview mode.</Admonition>`),
  `<Warning>Login Rules are currently in Preview mode.</Warning>`
)

expectsToBe(
  'Tip Notices',
  migrateTipNotices(`<Notice type="tip">lorem ipsum</Notice>`),
  `<Tip>lorem ipsum</Tip>`
)

expectsToBe(
  'Warning Notices',
  migrateWarningNotices(`<Notice type="warning">warning lorem ipsum</Notice>`),
  `<Warning>warning lorem ipsum</Warning>`
)

expectsToBe(
  'Details',
  migrateDetails(`<Details title="Logging in via the CLI">

  In addition to Teleport's Web UI, you can access resources in your
  infrastructure via the \`tsh\` client tool.
  
  Install \`tsh\` on your local workstation:
  
  Log in to receive short-lived certificates from Teleport:
  
  </Details>`),
  `<Accordion title="Logging in via the CLI">

  In addition to Teleport's Web UI, you can access resources in your
  infrastructure via the \`tsh\` client tool.
  
  Install \`tsh\` on your local workstation:
  
  Log in to receive short-lived certificates from Teleport:
  
  </Accordion>`
)

console.log('');
console.log('Testing variables migrations...');

expectsToBe(
  'Variables on page',
  migrateVariables(`---
title: "Page Title"
description: "Page Description"
---

## Header

The cluster name is (=clusterDefaults.clusterName=)`),
  `---
title: "Page Title"
description: "Page Description"
---

import { clusterDefaults } from "/snippets/variables.mdx";

## Header

The cluster name is {clusterDefaults.clusterName}`
)

console.log('');
console.log('Testing snippets migrations...');

expectsToBe(
  'Snippets on page',
  migrateSnippets(`---
title: "Page Title"
description: "Page Description"
---

## Header

(!docs/pages/includes/page.mdx!)`),
  `---
title: "Page Title"
description: "Page Description"
---

import Page from "/snippets/includes/page.mdx";

## Header

<Page />`
)

// Pages
// expectsToBeTest('Accordion', 1, 2);


console.log('');
console.log('💯 All unit tests passed!');