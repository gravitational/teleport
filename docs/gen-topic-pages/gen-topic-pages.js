const yaml = require('yaml');
const path = require('path');

const generationLine = `{/*TOPICS*/}`;

// TopicContentsFragment contains data necessary to generate a table of contents
// page for the subdirectories of a user-specified root directory.
// @param fs - The filesystem to use. Either memfs or the NodeJS fs package.
// @param {string} root - file path in fs in which to place the table of
// contents partial. While TopicContentsFragment is not responsible for writing
// the output file, it builds the table of contents based on the assumption that
// it will be a partial in the specified directory.
class TopicContentsFragment {
  constructor(fs, root) {
    this.fs = fs;
    this.root = root;
  }
  // makeTopicPage constructs an index of the files in the directory, returning
  // it as a string.
  //
  // makeTopicPage expects files in the directory to be MDX files with two
  // frontmatter keys, title and description. For each subdirectory, there must
  // be a menu page in the same directory as the subdirectory with the same name
  // as the subdirectory. For example, if there is a subdirectory called
  // "guides", we expect a menu page called "guides.mdx". We use this to
  // provide context for the subdirectory in the table of contents.
  makeTopicPage() {
    const parts = path.parse(this.root);
    const rootTOC = path.join(parts.dir, parts.name, parts.name + '.mdx');
    if (!this.fs.existsSync(rootTOC)){
    	throw (`There must be a page called ${rootTOC} that includes a line consisting of "${generationLine}".`);
    }
    const text = this.fs.readFileSync(rootTOC, 'utf-8');
    const lines = text.split('\n');
    if (!lines.includes(generationLine)) {
      throw new Error(
        `Found a root menu page that was not automatically generated (${rootTOC}). Add "${generationLine}" where you want to include a list of section topics and run the menu generator again..`
      );
    }

    return this.addTopicsFromDir(this.root, text + '\n', 0);
  }

  // getFrontmatter returns the frontmatter YAML of an MDX page as an object
  // containing keys "title" and "description".
  getFrontmatter(pageContent) {
    let text = pageContent;
    // Remove any initial YAML document separator
    if (pageContent.startsWith('---')) {
      text = text.slice('---'.length);
    }
    const docs = text.split('---');
    if (docs.length === 0) {
      throw new Error(`no frontmatter in ${fullPath}`);
    }

    const fm = yaml.parse(docs[0]);
    if (!fm || !fm.hasOwnProperty('title')) {
      throw new Error(`page ${fullPath} does not have a title`);
    }

    return fm;
  }

  // relativePathToFile takes a filepath and returns a path we can use in links
  // to the file in a table of contents page. The link path is a relative path
  // to the directory where we are placing the table of contents page.
  // @param filepath {string} - the path from which to generate a link path.
  relativePathToFile(filepath) {
    // Return the filepath without the first segment, removing the first
    // slash. This is because the TOC file we are generating is located at
    // this.root.
    return filepath.slice(this.root.length).replace(/^\//, '');
  }

  // addTopicsFromDir takes the path at dirPath and adds any topic listings to
  // the string sofar. Returns the new string.
  // @param dirPath {string} - path to the directory to use for generating part
  // of the table of contents. Handles one directory level per call.
  // @param sofar {string} - the table of contents fragment generated prior to
  // this iteration.
  // @ param level {number} is the heading level at
  // which to put any topic headings.
  addTopicsFromDir(dirPath, sofar, level) {
    let newText = sofar;
    const files = this.fs.readdirSync(dirPath, 'utf8');

    let mdxFiles = new Set();
    const dirs = files.reduce((accum, current) => {
      const stats = this.fs.statSync(path.join(dirPath, current));
      if (!stats.isDirectory()) {
        mdxFiles.add(path.join(dirPath, current));
        return accum;
      }
      accum.add(path.join(dirPath, current));
      return accum;
    }, new Set());

    // Preserve the non-generated part of the page while deleting the rest so we
    // can regenerate it.
    newText = newText.split(generationLine)[0] + generationLine + '\n\n';

    // Add rows to the menu page for non-menu pages.
    let entries = [];
    mdxFiles.forEach((f, idx) => {
      const text = this.fs.readFileSync(f, 'utf8');
      const lines = text.split('\n');

      // The file includes a table of contents but does not have the same name
      // as its parent directory, i.e., isn't a TOC page.
      if (
        lines.includes(generationLine) &&
        !path.dirname(f).endsWith(path.parse(f).name)
      ) {
        throw new Error(
          'Found a menu page that no longer corresponds to a subdirectory: ' + f
        );
      }

      // Skip TOC pages (with the same name as the parent directory) since we
      // process these elswhere.
      if (path.dirname(f).endsWith(path.parse(f).name)) {
        mdxFiles.delete(f);
        return;
      }

      let relPath = this.relativePathToFile(f);
      const fm = this.getFrontmatter(text);

      entries.push(`- [${fm.title}](${relPath}): ${fm.description}
`);
    });
    entries.sort();
    newText += entries.join('');

    // Add a blank line between the list of first-level guides and first-level
    // menu pages.
    if (mdxFiles.size > 0 && dirs.size > 0) {
      newText += `
`;
    }

    // Add rows to the menu page for first-level child menu pages
    let menuEntries = [];
    dirs.forEach((f, idx) => {
      const menuPath = path.join(f, path.parse(f).base + '.mdx');
      if (!this.fs.existsSync(menuPath)) {
        throw new Error(
          `there must be a page called ${menuPath} that includes a line that consists of ${generationLine}`
        );
      }
      const text = this.fs.readFileSync(menuPath, 'utf8');
      const lines = text.split('\n');

      if (!lines.includes(generationLine)) {
        throw new Error(
          `Found a menu page that was not automatically generated (${menuPath}). Add "${generationLine}" where you want to include a list of section topics and run the menu generator again..`
        );
      }

      let relPath = this.relativePathToFile(menuPath);
      const fm = this.getFrontmatter(text);

      let newEntry = `## ${fm.title}

${fm.description} ([more info](${relPath}))

`;

      const childFiles = this.fs.readdirSync(f, 'utf8');
      let childEntries = [];
      childFiles.forEach(fp => {
        let absChildPath = path.join(f, fp);
        const childName = path.parse(absChildPath).name;
        // Skip TOC pages (with the same name as the parent directory) since we
        // process these elswhere.
        if (path.dirname(absChildPath).endsWith(childName)) {
          return;
        }

        const stats = this.fs.statSync(path.join(f, fp));
        if (stats.isDirectory()) {
          // The file is a directory, so add a link to its TOC page
          absChildPath = path.join(absChildPath, childName + '.mdx');
          if (!this.fs.existsSync(absChildPath)) {
            throw `expected a table of contents page called ${absChildPath} that includes a line consisting of "${generationLine}"`;
          }
        }

        const relChildPath = this.relativePathToFile(absChildPath);
        const childText = this.fs.readFileSync(absChildPath, 'utf8');
        const childFM = this.getFrontmatter(childText);
        if (stats.isDirectory()) {
          childEntries.push(`- [${childFM.title} (section)](${relChildPath}): ${childFM.description}
`);
          return;
        }
        childEntries.push(`- [${childFM.title}](${relChildPath}): ${childFM.description}
`);
      });
      const sortedEntries = [...childEntries].sort();
      newEntry += sortedEntries.join('');
      menuEntries.push(newEntry);
    });
    menuEntries.sort();
    newText += menuEntries.join('\n');
    return newText;
  }
}
module.exports.TopicContentsFragment = TopicContentsFragment;
