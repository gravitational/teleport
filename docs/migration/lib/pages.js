const fs = require('fs');
const { readAllFilesFromDirectory, writeFile, findFrontmatterEndIndex, toPascalCase } = require('./utility');

// Location of the current documentation pages
const PAGES_DIRECTORY = '../pages'

const variablesRegex = /\(=\s*(.*?)\s*=\)/g;
const snippetsRegex = /\(!docs\/pages\/(.*\/(.*)\.mdx)!\)/g;

// TODO: Add moving all includes folders into snippets
// TODO: Consider for cases where snippets and variables are inside code blocks or other MDX syntax
// TODO: Add setup for properties
function migrateReusableSnippets(page) {
  const matches = page.matchAll(snippetsRegex);
  
  const snippetsMap = {};
  for (const match of matches) {
    const snippet = match[1];
    snippetsMap[snippet] = toPascalCase(match[2]);
  }

  const uniqueSnippets = Object.entries(snippetsMap);

  if (uniqueSnippets.length === 0) {
    return page;
  }

  let newPage = page;

  const importStatement = `${uniqueSnippets.reduce((acc, [path, component]) => acc + `import ${component} from "/snippets/${path}";\n`, '')}\n`
  const frontmatterEndIndex = findFrontmatterEndIndex(page);
  newPage = page.slice(0, frontmatterEndIndex) + importStatement + page.slice(frontmatterEndIndex)
  
  return newPage.replace(snippetsRegex, (_, _path, filename) => {
    return `<${toPascalCase(filename)} />`
  });
}

function migrateReusableVariables(page) {
  const matches = page.matchAll(variablesRegex);
  
  const variablesMap = {};
  for (const match of matches) {
    const variable = match[1];
    const variableParent = variable.substr(0, variable.indexOf('.'));
    variablesMap[variableParent] = true;
  }

  const uniqueVariables = Object.keys(variablesMap);

  if (uniqueVariables.length === 0) {
    return page;
  }

  let newPage = page;

  const importStatement = `import { ${uniqueVariables.join(', ')} } from '/snippets/variables.mdx'\n\n`
  const frontmatterEndIndex = findFrontmatterEndIndex(page);
  newPage = page.slice(0, frontmatterEndIndex) + importStatement + page.slice(frontmatterEndIndex)

  return newPage.replace(variablesRegex, '{$1}');
}

const migrationFunctions = {
  figure: (page) => page.replace(/<Figure[^>]*>([\s\S]*?)<\/Figure>/g, '$1'),
  relativeLink: (page) => page.replace(/\(.\//, '(').replace(/\.mdx\)/g, ')'),
  tab: (page) =>
    page
      .replace(/<TabItem[\S\s]*label=/g, '<Tab title=')
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
  variable: migrateReusableVariables,
  snippet: migrateReusableSnippets,
};

function migratePages() {
  // Build global variables page
  for (const pagePath of readAllFilesFromDirectory(PAGES_DIRECTORY)) {
    const pageContent = fs.readFileSync(pagePath, 'utf8');

    let migratedPage = pageContent;

    for (migrationFunction of Object.values(migrationFunctions)) {
      migratedPage = migrationFunction(migratedPage);
    }

    const relativePagePath = pagePath.replace(PAGES_DIRECTORY, '');
    const isSnippet = relativePagePath.includes('/includes/');
    const outputPagePath = isSnippet ? `./output/snippets${relativePagePath}` : `./output${relativePagePath}`;

    writeFile(outputPagePath, migratedPage);
  }
}

module.exports = {
  migratePages,
};
