package main

// Location of the current documentation pages
const PAGES_DIRECTORY = "../pages"

const defaultSnippetTemplateBindingRegex = regexp.MustCompile(`^{{.+=.+}}\n`)
const templateBindingRegex = regexp.MustCompile(`{{\s*(\S+)\s*}}`);
const variablesRegex = regexp.MustCompile(`\(=\s*(.*?)\s*=\)`)
const snippetsRegex = regexp.MustCompile(`\(!\/*docs\/pages\/(.*\/(.*)\.mdx)(.*)!\)`)

func migrateFigures(page string) string {
    re := regexp.MustCompile(`<Figure[^>]*>([\s\S]*?)<\/Figure>`)
  return re.ReplaceAllString(page, "$1")
}

func migrateTabs(page string) string {
    begin := regexp.MustCompile(`<TabItem[\S\s]*?label=`)
    end := regexp.MustCompile(`<\/TabItem>`)
    s :=    begin.ReplaceAllString("<Tab title=")
    return end.ReplaceAllString(s,  "</Tab>")
}

func migrateTipAdmonitions(page string) string {
    re := regexp.MustCompile(`/<Admonition\s+type="tip"[^>]*?>([\s\S]*?)<\/Admonition>/`)
  return re.ReplaceAllString(page, "<Tip>$1</Tip>")
}

func migrateNoteAdmonitions(page string) string {
    re := regexp.MustCompile(`<Admonition\s+type="note"[^>]*?>([\s\S]*?)<\/Admonition>`)
  return re.ReplaceAllString(page, 
    "<Note>$1</Note>")
}

func migrateWarningAdmonitions(page string) string {
    re := regexp.MustCompile(`<Admonition\s+type="warning"[^>]*?>([\s\S]*?)<\/Admonition>`)
  return re.ReplaceAllString(
      page,
    "<Warning>$1</Warning>",
  )
}

func migrateTipNotices(page string) string {
    re := regexp.MustCompile(`<Notice\s+type="tip"[^>]*?>([\s\S]*?)<\/Notice>`)
  return re.ReplaceAllString(page,     "<Tip>$1</Tip>")
}

func migrateWarningNotices(page string) string {
    re := regexp.MustCompile(`<Notice\s+type="warning"[^>]*?>([\s\S]*?)<\/Notice>`)
    return re.ReplaceAllString(page, 
    "<Warning>$1</Warning>")
}

func migrateDetails(page string)string {
    begin := regexp.MustCompile(`<Details([^>]+)>`)
    end := regexp.MustCompile(`<\/Details>`)
    s := begin.ReplaceAllString(page, "<Accordion$1>")
return end.ReplaceAllString(s,   "</Accordion>")
}

func migrateVarComponent(page string) string {
    re := regexp.MustCompile(`<Var[\s\S]*?name="(.*?)"[\s\S]*?\`)
return    re.ReplaceAllString(page, "$1")
}

// TODO: Need to finish migrating this function to Go
func migrateSnippetTemplateBinding(snippetPage string) string {
    defaultValues :=defaultSnippetTemplateBindingRegex.MatchAllString(snippetPage)

if len(defaultValues) == 0{
    return snippetPage
  }

  // TODO: Create a map from the default values
  const defaultValuesMap = JSON.parse(defaultValues[0].trim().replaceAll(`'`, "").replaceAll(/(\S+)=/g, `"$1":`).replaceAll(/`\s+`/g, `", "`).slice(1,-1));

  newPage := defaultSnippetTemplateBindingRegex.ReplaceAllString(snippetPage, "")

  // TODO: Replace variables with their default values
  newPage = newPage.replace(templateBindingRegex, (_match, variableName) => {
    return `{ ${variableName} || "${defaultValuesMap[variableName]}" }`
  })

  return newPage;
}

// TODO: Finish migrating this function to Go
func migrateVariables(page string) string {
    matches :=variablesRegex.MatchAllString(page);
  
    variablesMap := make(map[string]struct{})
    for _, variable := range matches {
	variableParent := variable[0:strings.Index(variable,".")]
        variablesMap[variableParent] = struct{}{}
    }

  if len(variablesMap) == 0 {
    return page;
  }

  newPage := page;

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
