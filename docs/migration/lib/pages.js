const fs = require('fs');
const { readAllFilesFromDirectory, writeFile, findFrontmatterEndIndex, toPascalCase } = require('./utility');

// Location of the current documentation pages
const PAGES_DIRECTORY = '../pages'

const defaultSnippetTemplateBindingRegex = /^{{.+=.+}}\n/;
const templateBindingRegex = /{{\s*(\S+)\s*}}/g;
const variablesRegex = /\(=\s*(.*?)\s*=\)/g;
const snippetsRegex = /\(!\/*docs\/pages\/(.*\/(.*)\.mdx)(.*)!\)/g;

function migrateFigures(page) {
  return page.replace(/<Figure[^>]*>([\s\S]*?)<\/Figure>/g, '$1')
}

function migrateTabs(page) {
  return page
    .replace(/<TabItem[\S\s]*?label=/g, '<Tab title=')
    .replace(/<\/TabItem>/g, '</Tab>');
}

function migrateTipAdmonitions(page) {
  return page.replace(
    /<Admonition\s+type="tip"[^>]*?>([\s\S]*?)<\/Admonition>/g,
    '<Tip>$1</Tip>'
  )
}

function migrateNoteAdmonitions(page) {
  return page.replace(
    /<Admonition\s+type="note"[^>]*?>([\s\S]*?)<\/Admonition>/g,
    '<Note>$1</Note>'
  )
}

function migrateWarningAdmonitions(page) {
  return page.replace(
    /<Admonition\s+type="warning"[^>]*?>([\s\S]*?)<\/Admonition>/g,
    '<Warning>$1</Warning>'
  )
}

function migrateTipNotices(page) {
  return page.replace(
    /<Notice\s+type="tip"[^>]*?>([\s\S]*?)<\/Notice>/g,
    '<Tip>$1</Tip>'
  )
}

function migrateWarningNotices(page) {
  return page.replace(
    /<Notice\s+type="warning"[^>]*?>([\s\S]*?)<\/Notice>/g,
    '<Warning>$1</Warning>'
  )
}

function migrateDetails(page) {
  return page
    .replace(/<Details([^>]+)>/g, '<Accordion$1>')
    .replace(/<\/Details>/g, '</Accordion>')
}

function migrateVarComponent(page) {
  return page
    .replace(/<Var[\s\S]*?name="(.*?)"[\s\S]*?\/>/g, '$1')
}

function migrateSnippetTemplateBinding(snippetPage) {
  const defaultValues = snippetPage.match(defaultSnippetTemplateBindingRegex);

  if (defaultValues == null) {
    return snippetPage
  }

  const defaultValuesMap = JSON.parse(defaultValues[0].trim().replaceAll('\'', '').replaceAll(/(\S+)=/g, '"$1":').replaceAll(/"\s+"/g, '", "').slice(1,-1));

  let newPage = snippetPage.replace(defaultSnippetTemplateBindingRegex, '');
  newPage = newPage.replace(templateBindingRegex, (_match, variableName) => {
    return `{ ${variableName} || "${defaultValuesMap[variableName]}" }`
  })

  return newPage;
}

function migrateVariables(page) {
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

  const importStatement = `import { ${uniqueVariables.join(', ')} } from "/snippets/variables.mdx";\n\n`
  const frontmatterEndIndex = findFrontmatterEndIndex(page);
  newPage = page.slice(0, frontmatterEndIndex) + importStatement + page.slice(frontmatterEndIndex)

  return newPage.replace(variablesRegex, '{$1}');
}

function migrateSnippets(page) {
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
  
  return newPage.replace(snippetsRegex, (_, _path, filename, props) => {
    return `<${toPascalCase(filename)}${props} />`
  });
}

const migrationFunctions = {
  migrateFigures,
  migrateTabs,
  migrateTipAdmonitions,
  migrateNoteAdmonitions,
  migrateWarningAdmonitions,
  migrateTipNotices,
  migrateWarningNotices,
  migrateDetails,
  migrateVarComponent,
  migrateSnippetTemplateBinding,
  migrateVariables,
  migrateSnippets,
};

function migratePages() {
  // Build global variables page
  for (const pagePath of readAllFilesFromDirectory(PAGES_DIRECTORY)) {
    const pageContent = fs.readFileSync(pagePath, 'utf8');

    let migratedPage = pageContent;

    for (migrationFunction of Object.values(migrationFunctions)) {
      migratedPage = migrationFunction(migratedPage);
      migratedPage = migratedPage.trim();
    }

    const relativePagePath = pagePath.replace(PAGES_DIRECTORY, '');
    const isSnippet = relativePagePath.includes('/includes/');
    const outputPagePath = isSnippet ? `./output/snippets${relativePagePath}` : `./output${relativePagePath}`;

    writeFile(outputPagePath, migratedPage);
  }
}

module.exports = { ...migrationFunctions, migratePages };
