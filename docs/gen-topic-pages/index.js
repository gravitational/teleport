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
  .option('out', {
    describe:
      'Relative path to a directory in which to place topic page partials, which are named after their corresponding root input directories. For example, use "docs/pages/includes/topic-pages.',
  })
  .demandOption(['in', 'out'])
  .help()
  .parse();

args.in.split(',').forEach(p => {
  const command =
    'node docs/gen-topic-pages/index.js ' + hideBin(process.argv).join(' ');
  const prefix = path.relative(args.out, p);
  const frag = new TopicContentsFragment(command, fs, p, prefix);
  const parts = path.parse(p);
  const newPath = path.join(args.out, parts.name + '.mdx');
  fs.writeFileSync(newPath, frag.makeTopicTree());
});
