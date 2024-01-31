const fs = require('fs');
const { writeFile } = require('./utility');

// Directory of the primary config file
const CONFIG_DIRECTORY = '../config.json'

const initialMintConfig = {
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

function addVariablesSnippetsFile(variables) {
  const variablesSnippets = `${Object.entries(variables)
    .reduce((acc, [variable, value]) => acc + `export const ${variable} = ${JSON.stringify(value, null, 2)};\n\n`, '')}`;
  
  writeFile(
    `./output/snippets/variables.mdx`,
    variablesSnippets,
  );
}

function migrateConfig() {
  const configContent = fs.readFileSync(CONFIG_DIRECTORY, 'utf8');
  const { navigation, redirects, variables } = JSON.parse(configContent);

  const migratedConfig = {
    ...initialMintConfig,
    navigation: migrateConfigNavigation(navigation),
    redirects: migrateConfigRedirects(redirects),
  };

  addVariablesSnippetsFile(variables);

  writeFile(
    `./output/mint.json`,
    JSON.stringify(migratedConfig, null, 2),
  );
}

module.exports = {
  migrateConfig
};
