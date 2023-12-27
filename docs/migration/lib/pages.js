const fs = require('fs');
const { readAllFilesFromDirectory, writeFile } = require('./utility');

// Location of the current documentation pages
const PAGES_DIRECTORY = '../pages'

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
  snippet: migrateReusableSnippet,
  variable: migrateReusableVariable,
};



function migratePages() {
  // Build global variables page
  for (const pagePath of readAllFilesFromDirectory(PAGES_DIRECTORY)) {
    const pageContent = fs.readFileSync(pagePath, 'utf8');

    let migratedPage = pageContent;

    for (migrationFunction of Object.values(migrationFunctions)) {
      migratedPage = migrationFunction(migratedPage);
    }

    const outputPagePath = `./output${pagePath.replace(PAGES_DIRECTORY, '')}`;

    writeFile(outputPagePath, migratedPage);
  }
}

module.exports = {
  migratePages,
};
