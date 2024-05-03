const yaml = require('yaml');
const path = require('path');

// TopicContentsFragment contains data necessary to generate a table of contents
// page for the subdirectories of a user-specified root directory.
// @param fs - The filesystem to use. Either memfs or the NodeJS fs package.
// @param {string} prefix - prefix to append before any links in the topic tree.
// This is a sequence of relative path characters, e.g., "../../", and enables
// users to place topic trees within directories for partials.
// @param {string} root - file path in fs in which to place the table of
// contents partial. While TopicContentsFragment is not responsible for writing
// the output file, it builds the table of contents based on the assumption that
// it will be a partial in the specified directory.
class TopicContentsFragment {
  constructor(command, fs, root, prefix) {
    this.command = command;
    this.fs = fs;
    this.prefix = prefix;
    this.root = root;
  }
  // makeTopicTree constructs an index of the files in the directory, returning
  // it as a string.
  //
  // makeTopicTree expects files in the directory to be MDX files with two
  // frontmatter keys, title and description. For each subdirectory, there must
  // be a menu page in the same directory as the subdirectory with the same name
  // as the subdirectory. For example, if there is a subdirectory called
  // "guides", we expect a menu page called "guides.mdx". We use this to
  // provide context for the subdirectory in the table of contents.
  makeTopicTree() {
    const initial = `{/*GENERATED PARTIAL. DO NOT EDIT. RECREATE WITH THIS COMMAND:
${this.command}*/}

`;
    return this.addTopicsFromDir(this.root, initial, 2);
  }

  // getFrontmatter returns the frontmatter YAML of an MDX page as an object
  // containing keys "title" and "description".
  getFrontmatter(fullPath) {
    let text = this.fs.readFileSync(fullPath, 'utf8');
    // Remove any initial YAML document separator
    if (text.startsWith('---\n')) {
      text = text.slice('---\n'.length);
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
    // Remove the root and replace it with the prefix. Add any
    // path segments to get us from the prefix to any inner
    // child directories.
    const additionalpathsegments = filepath.slice(this.root.length);
    return path.join(this.prefix, additionalpathsegments);
  }

  // addTopicsFromDir takes the path at dirPath and recursively adds any topic
  // listings to the string sofar. Returns the new string.
  // @param dirPath {string} - path to the directory to use for generating part
  // of the table of contents. Handles one directory level per call.
  // @param sofar {string} - the table of contents fragment generated prior to
  // this iteration.
  // @ param level {number} is the heading level at
  // which to put any topic headings.
  addTopicsFromDir(dirPath, sofar, level) {
    let newText = sofar;
    const files = this.fs.readdirSync(dirPath, 'utf8');

    // List all MDX files in the current directory
    const mdxFiles = files.reduce((accum, current) => {
      const filename = path.parse(current).base;
      if (
        !current.endsWith('.mdx') ||
        // We assume that each All Topics partial is included in a page called
        // "all-topics.mdx".
        filename == 'all-topics.mdx'
      ) {
        return accum;
      }
      accum[path.join(dirPath, current)] = true;
      return accum;
    }, {});

    // List all subdirectories of the current directory
    const dirs = files.reduce((accum, current) => {
      const stats = this.fs.statSync(path.join(dirPath, current));
      if (stats.isDirectory()) {
        accum[path.join(dirPath, current)] = true;
      }

      return accum;
    }, {});

    // List all MDX files in the subdirectory with the same name as a
    // subdirectory. By convention, these are menu pages for the subdirectory.
    const menuPages = Object.keys(mdxFiles).reduce((accum, current) => {
      const parts = path.parse(current);
      const asDir = path.join(parts.dir, parts.name);
      if (dirs[asDir]) {
        accum[current] = true;
        // Exclude the menu page from the map of regular MDX pages. We
        // treat these separately.
        delete mdxFiles[current];
      }
      return accum;
    }, {});

    // Add a table of MDX pages within the directory.
    if (Object.keys(mdxFiles).length > 0) {
      newText =
        newText +
        `|Topic|Description|
|---|---|
`;
    }

    // Add rows to the table.
    Object.keys(mdxFiles).forEach(f => {
      const relPath = this.relativePathToFile(f);
      const fm = this.getFrontmatter(f);
      newText = newText + `|[${fm.title}](${relPath})|${fm.description}|\n`;
    });

    // Add another section of the topic for each subdirectory.
    Object.keys(dirs).forEach(p => {
      if (!menuPages[p + '.mdx']) {
        throw new Error(
          `expecting a menu page for ${p} called ${p + '.mdx'}, but there is none`
        );
      }
      const fm = this.getFrontmatter(p + '.mdx');
      let heading = '';
      for (let i = 0; i < level; i++) {
        heading += '#';
      }

      // Make sure there is a blank line before the heading
      if (!newText.endsWith('\n\n')) {
        newText = newText + '\n';
      }
      newText =
        newText +
        `${heading} ${fm.title}

${fm.description} ([more info](${this.relativePathToFile(p) + '.mdx'}))

`;

      newText = this.addTopicsFromDir(p, newText, level + 1);
    });

    return newText;
  }
}
module.exports.TopicContentsFragment = TopicContentsFragment;
