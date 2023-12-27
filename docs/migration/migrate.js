const { migratePages } = require('./lib/pages');
const { migrateConfig } = require('./lib/config');

function main() {
  migratePages();
  migrateConfig();
}

main();
