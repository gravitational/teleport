#!/user/bin/env node
const { TopicContentsFragment } = require('./gen-topic-pages.js');
const yargs = require('yargs/yargs');
const { hideBin } = require('yargs/helpers');
const process = require('node:process');
const fs = require('node:fs');
const path = require('node:path');

const args = yargs(hideBin(process.argv))
  .option('in', {
    describe:
      'Comma-separated list of root directory paths from which to generate topic page partials. We expect each root directory to include the output in a page called "all-topics.mdx"',
  })
  .demandOption(['in'])
  .help()
  .parse();

const addTopicsForDir = (dirPath, command) => {
  const frag = new TopicContentsFragment(fs, dirPath, '');
  const parts = path.parse(dirPath);
  const newPath = path.join(parts.dir, parts.name + '.mdx');
  fs.writeFileSync(newPath, frag.makeTopicPage());

  fs.readdirSync(dirPath).forEach(filePath => {
    const fullPath = path.join(dirPath, filePath);
    const stats = fs.statSync(fullPath);
    if (stats.isDirectory()) {
      addTopicsForDir(fullPath);
    }
  });
};

args.in.split(',').forEach(p => {
  addTopicsForDir(p);
});
