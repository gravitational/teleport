// Migrate all content inside /pages and config to the Mintlify supported syntax
const { migratePages, migrateConfig } = require('./utility');

function main() {
  migratePages();
  migrateConfig();
}

main();
