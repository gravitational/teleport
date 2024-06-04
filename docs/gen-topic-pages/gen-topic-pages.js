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
    const rootConfig = path.join(parts.dir, parts.name + '.mdx');
    const text = this.fs.readFileSync(rootConfig, 'utf-8');
    const lines = text.split('\n');
    if (!lines.includes(generationLine)) {
      throw new Error(
        `Found a root menu page that was not automatically generated (${this.root + '.mdx'}). Add "${generationLine}" where you want to include a list of section topics and run the menu generator again..`
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
    const rootName = path.parse(this.root).name;
    return path.join(rootName, filepath.slice(this.root.length));
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

    const dirs = files.reduce((accum, current) => {
      const stats = this.fs.statSync(path.join(dirPath, current));
      if (!stats.isDirectory()) {
        return accum;
      }
      accum[path.join(dirPath, current)] = true;
      return accum;
    }, {});

    let firstLevelGuides = {};
    let menuPages = {};

    // Sort files into menu pages and non-menu pages so we can format
    // them accordingly.
    files.forEach(current => {
      if (!current.endsWith('.mdx')) {
        return;
      }

      const pathParts = path.parse(path.join(dirPath, current));
      const asDir = path.join(pathParts.dir, pathParts.name);
      if (!!dirs[asDir]) {
        menuPages[path.join(dirPath, current)] = true;
        return;
      }

      firstLevelGuides[path.join(dirPath, current)] = true;
    });

    // Preserve the non-generated part of the page while deleting the rest so we
    // can regenerate it.
    newText = newText.split(generationLine)[0] + generationLine + '\n\n';

    // Add rows to the menu page for non-menu pages.
    let entries = [];
    Object.keys(firstLevelGuides).forEach((f, idx) => {
      const text = this.fs.readFileSync(f, 'utf8');
      const lines = text.split('\n');

      if (lines.includes(generationLine)) {
        throw new Error(
          'Found a menu page that no longer corresponds to a subdirectory: ' + f
        );
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
    if (
      Object.keys(firstLevelGuides).length > 0 &&
      Object.keys(menuPages).length > 0
    ) {
      newText += `
`;
    }

    // Add rows to the menu page for first-level child menu pages
    let menuEntries = [];
    Object.keys(menuPages).forEach((f, idx) => {
      const text = this.fs.readFileSync(f, 'utf8');
      const lines = text.split('\n');
      const correspondingDir = f.replace(/\.mdx$/, '');

      if (!lines.includes(generationLine)) {
        throw new Error(
          `Found a menu page that was not automatically generated (${f}). Add "${generationLine}" where you want to include a list of section topics and run the menu generator again..`
        );
      }

      let relPath = this.relativePathToFile(f);
      const fm = this.getFrontmatter(text);

      let newEntry = `## ${fm.title}

${fm.description} ([more info](${relPath}))

`;

      const childFiles = this.fs.readdirSync(correspondingDir, 'utf8');
      let childDirs = {};
      childFiles.forEach(fp => {
        const stats = this.fs.statSync(path.join(correspondingDir, fp));
        if (stats.isDirectory()) {
          childDirs[path.join(correspondingDir, fp)] = true;
        }
      });
      let childEntries = [];
      childFiles.forEach(fp => {
        const stats = this.fs.statSync(path.join(correspondingDir, fp));
        if (stats.isDirectory()) {
          return;
        }
        const absChildPath = path.join(correspondingDir, fp);
        const relChildPath = this.relativePathToFile(absChildPath);
        const childText = this.fs.readFileSync(absChildPath, 'utf8');
        const childFM = this.getFrontmatter(childText);
        if (
          !!childDirs[
            path.join(correspondingDir, fp.slice(0, fp.length - '.mdx'.length))
          ]
        ) {
          childEntries.push(`- [${childFM.title} (section)](${relChildPath}): ${childFM.description}
`);
          return;
        }
        childEntries.push(`- [${childFM.title}](${relChildPath}): ${childFM.description}
`);
      });
      childEntries.sort();
      newEntry += childEntries.join('');
      menuEntries.push(newEntry);
    });
    menuEntries.sort();
    newText += menuEntries.join('\n');
    return newText;
  }
}
module.exports.TopicContentsFragment = TopicContentsFragment;
