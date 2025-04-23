const tsConfigBase = require('./tsconfig.json');

const ourPackages = new Set(
  Object.keys(tsConfigBase.compilerOptions.paths).map(
    // Remove extra '/*' if present in the package name.
    packageName => packageName.split('/')[0]
  )
);
const appPackages = ['teleport', 'e-teleport', 'teleterm'].flatMap(pkg => [
  pkg,
  `@gravitational/${pkg}`,
]);
const libraryPackages = [...ourPackages]
  .filter(pkg => !appPackages.includes(pkg))
  .flatMap(pkg => [pkg, `@gravitational/${pkg}`]);

module.exports = {
  arrowParens: 'avoid',
  printWidth: 80,
  bracketSpacing: true,
  plugins: ['@ianvs/prettier-plugin-sort-imports'],
  importOrder: [
    '<BUILTIN_MODULES>',
    '',
    '<THIRD_PARTY_MODULES>',
    '',
    `^(${libraryPackages.join('|')})`,
    '',
    `^(${appPackages.join('|')})`,
    '',
    '^[./]',
  ],
  importOrderParserPlugins: ['typescript', 'jsx', 'decorators-legacy'],
  importOrderTypeScriptVersion: '5.0.0',
  semi: true,
  singleQuote: true,
  tabWidth: 2,
  trailingComma: 'es5',
};
