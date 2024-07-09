#!/user/bin/env node
const { RedirectChecker } = require('./check-redirects.js');
const yargs = require('yargs/yargs');
const { hideBin } = require('yargs/helpers');
const process = require('node:process');
const fs = require('node:fs');
const path = require('node:path');

const args = yargs(hideBin(process.argv))
  .option('in', {
    describe: 'root directory path in which to check for necessary redirects.',
  })
  .option('config', {
    describe: 'path to a docs configuration file with a "redirects" key',
  })
  .option('docs', {
    describe: 'path to the root of a gravitational/teleport repo',
  })
  .option('exclude', {
    describe:
      'comma-separated list of file extensions not to check, e.g., ".md" or ".test.tsx"',
  })
  .option('name', {
    describe:
      'name of the directory tree we are checking for docs URLs (for display only)',
  })
  .demandOption(['in', 'config', 'docs', 'name'])
  .help()
  .parse();

const conf = fs.readFileSync(args.config);
const redirects = JSON.parse(conf).redirects;
let exclude;
if (args.exclude != undefined) {
  exclude = args.exclude.split(',');
}
const checker = new RedirectChecker(fs, args.in, args.docs, redirects, exclude);
const results = checker.check();

if (!!results && results.length > 0) {
  const message =
    `Found Teleport docs URLs in ${args.name} that do not correspond to a docs
page or redirect. Either add a redirect for these or edit ${args.name}.
  - ` + results.join('\n  - ');
  console.error(message);
  process.exit(1);
}
