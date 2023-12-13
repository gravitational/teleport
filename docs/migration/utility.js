const fs = require('fs');
var getDirName = require('path').dirname;
const path = require('path');

// Location of the current documentation pages
const PAGES_DIRECTORY = '../pages'

function writeFile(path, contents) {
  fs.mkdir(getDirName(path), { recursive: true}, function (err) {
    if (err) return;

    fs.writeFileSync(path, contents, 'utf8');
  });
}

function* readAllFiles(dir) {
  const files = fs.readdirSync(dir, { withFileTypes: true });

  for (const file of files) {
    if (file.isDirectory()) {
      yield* readAllFiles(path.join(dir, file.name));
    } else {
      yield path.join(dir, file.name);
    }
  }
}

const teleportMintConfig = {
  $schema: 'https://mintlify.com/schema.json',
  name: 'Teleport',
  logo: {
    light: '/logo/light.png',
    dark: '/logo/dark.png',
  },
  favicon: '/favicon.png',
  colors: {
    primary: '#512FC9',
    light: '#7956F5',
    dark: '#512FC9',
  },
  topAnchor: {
    name: 'OpenSource',
    icon: 'code',
  },
  anchors: [
    {
      name: "Manage Access",
      icon: "shield-halved",
      url: "access-controls"
    },
    {
      name: "Management",
      icon: "people-group",
      url: "management"
    }
  ],
  feedback: {
    thumbsRating: true,
  },
  topbarCtaButton: {
    name: 'Get Started',
    url: 'https://goteleport.com/pricing/',
  },
  topbarLinks: [
    {
      name: 'Sign In',
      url: 'https://teleport.sh/',
    },
  ],
};

/*
  Snippets with Mintlify works like React components.
  
  1. Move the file inside the /snippets folder.
  2. Add an import statement to the top of the file. Example: import File from '/snippets/name.mdx'
  3. Add the code the same way you would a component. Example: <File />

  
  The migration script is intentionally not provided as choosing the name for snippet components should be decided by the writers for usability.
*/
function migrateReusableSnippet(page) {
  return page;
}

/*
  Variables with Mintlify also works like JS/React exports
  
  1. Create a new file called /snippets/variables.mdx
  2. Add the variable inside the file the same way you would export a variable. Example: export const name = 'John Smith'
  3. Import the variable. Example: import { name } from '/snippets/variables.mdx';
  4. Use the variable inside curly brackets. Example {name}

  The migration script is also intentionally not provided as the variable names should be decided by the writers for usability.
*/
function migrateReusableVariable(page) {
  return page;
}

const migrationFunctions = {
  figure: (page) => page.replace(/<Figure[^>]*>([\s\S]*?)<\/Figure>/g, '$1'),
  relativeLink: (page) => page.replace(/\(.\//, '(').replace(/\.mdx\)/g, ')'),
  tab: (page) =>
    page
      .replace(/<TabItem label=/g, '<Tab title=')
      .replace(/<\/TabItem>/g, '</Tab>'),
  tipAdmonition: (page) =>
    page.replace(
      /<Admonition\s+type="tip"[^>]*>([\s\S]*?)<\/Admonition>/g,
      '<Tip>$1</Tip>'
    ),
  noteAdmonition: (page) =>
    page.replace(
      /<Admonition\s+type="note"[^>]*>([\s\S]*?)<\/Admonition>/g,
      '<Note>$1</Note>'
    ),
  warningAdmonition: (page) =>
    page.replace(
      /<Admonition\s+type="warning"[^>]*>([\s\S]*?)<\/Admonition>/g,
      '<Warning>$1</Warning>'
    ),
  tipNotice: (page) =>
    page.replace(
      /<Notice\s+type="tip"[^>]*>([\s\S]*?)<\/Notice>/g,
      '<Tip>$1</Tip>'
    ),
  warningNotice: (page) =>
    page.replace(
      /<Notice\s+type="warning"[^>]*>([\s\S]*?)<\/Notice>/g,
      '<Warning>$1</Warning>'
    ),
  detail: (page) =>
    page
      .replace(/<Details([^>]+)>/g, '<Accordion$1>')
      .replace(/<\/Details>/g, '</Accordion>'),
  // TODO
  snippet: migrateReusableSnippet,
  variable: migrateReusableVariable,
};



function migratePages() {
  for (const pagePath of readAllFiles(PAGES_DIRECTORY)) {
    const pageContent = fs.readFileSync(pagePath, 'utf8');

    let migratedPage = pageContent;

    for (migrationFunction of Object.values(migrationFunctions)) {
      migratedPage = migrationFunction(migratedPage);
    }

    const outputPagePath = `./output${pagePath.replace(PAGES_DIRECTORY, '')}`;

    writeFile(outputPagePath, migratedPage);
  }
}

function migrateConfigNavigation(navigation) {
  return navigation.map((nav) => {
    if (nav.slug) {
      if (nav.slug === '/') {
        return 'index';
      }
      // Remove leading and trailing slash
      return nav.slug.replace(/^\/+/g, '').replace(/\/\s*$/, '');
    }

    let migratedNav = {
      group: nav.title,
    };

    if (nav.entries) {
      migratedNav.pages = migrateConfigNavigation(nav.entries);
    }

    return migratedNav;
  });
}

function migrateConfigRedirects(redirects) {
  // Preserve unique values
  const redirectsMap = {};
  redirects.forEach((redirect) => {
    redirectsMap[redirect.source] = redirect.destination
  });

  return Object.entries(redirectsMap).map(([source, destination]) => {
    return {
      source,
      destination,
    };
  })
}

function migrateConfig() {
  const configContent = fs.readFileSync(`../config.json`, 'utf8');
  const { navigation, redirects } = JSON.parse(configContent);

  const migratedConfig = {
    ...teleportMintConfig,
    navigation: migrateConfigNavigation(navigation),
    redirects: migrateConfigRedirects(redirects),
  };

  writeFile(
    `./output/mint.json`,
    JSON.stringify(migratedConfig, null, 2),
  );
}

module.exports = {
  migratePages,
  migrateConfig,
};
