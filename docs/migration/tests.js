const { expectsToBeTest } = require('./lib/utility');
const { migrateFigures, migrateTabs } = require('./lib/pages');

/**
 * Testing Components
 **/

expectsToBeTest(
  'Component: Figures',
  migrateFigures(`<Figure width="700">![Architecture of the setup you will complete in this guide](../img/linux-server-diagram.png)</Figure>`),
  `![Architecture of the setup you will complete in this guide](../img/linux-server-diagram.png)`
);

expectsToBeTest(
  'Component: Tabs',
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

// Snippets

// Variables

// Pages
// expectsToBeTest('Accordion', 1, 2);


console.log('✅ All unit tests passed');