#!/user/bin/env node
const { TopicContentsFragment } = require('./gen-topic-pages.js');
const yargs = require('yargs/yargs');
const { hideBin } = require('yargs/helpers');
const process = require('node:process');
const fs = require('node:fs');
const path = require('node:path');

const args = yargs(hideBin(process.argv))
  .option('in', {
    describe: `Comma-separated list of root directory paths from which to generate topic pages. We expect each root directory to include the output in a page, within the directory, that has the directory's name"`,
  })
  .option('ignore', {
    describe: `Comma-separated list of directory paths to skip when generating topic pages. The generator will not place a topic page within that directory or its children.`,
  })
  .demandOption(['in'])
  .help()
  .parse();

// addTopicsForDir generates a single topics page for the given dirPath, then
// recursively descends dirPath to write topic pages to its subdirectories.
// @param dirPath {string} - relative path to the working directory of the
// script in which to place a topic page.
// @param ignore {Set} - set of relative paths. addTopicsForDir will
// not write topic pages to this directory or its children.
const addTopicsForDir = (dirPath, ignore, lvl) => {
  // Skip ignored directories and their children
  if (ignore.has(dirPath)) {
    return;
  }

  let current = lvl;
  if (!current) {
    current = 0;
  }
  // Only add table of contents pages for subdirectories of the root
  // directory. Topic pages for the root directory are too long.
  if (lvl > 0) {
    const frag = new TopicContentsFragment(fs, dirPath, '');
    const parts = path.parse(dirPath);
    const newPath = path.join(parts.dir, parts.name, parts.name + '.mdx');

    fs.writeFileSync(newPath, frag.makeTopicPage());
  }

  fs.readdirSync(dirPath).forEach(filePath => {
    const fullPath = path.join(dirPath, filePath);
    const stats = fs.statSync(fullPath);
    if (stats.isDirectory()) {
      addTopicsForDir(fullPath, ignore, current + 1);
    }
  });
};

let ignore;
if (!args.ignore) {
  ignore = new Set();
} else {
  ignore = new Set(args.ignore.split(','));
}

args.in.split(',').forEach(p => {
  try {
    addTopicsForDir(p, ignore);
  } catch (err) {
    console.error(`Problem creating table of contents pages: ${err}.`);
    process.exit(1);
  }
});
