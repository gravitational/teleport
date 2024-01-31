const fs = require('fs');
const getDirName = require('path').dirname;
const path = require('path');

function* readAllFilesFromDirectory(dir) {
  const files = fs.readdirSync(dir, { withFileTypes: true });

  for (const file of files) {
    if (file.isDirectory()) {
      yield* readAllFilesFromDirectory(path.join(dir, file.name));
    } else {
      yield path.join(dir, file.name);
    }
  }
}

function writeFile(path, contents) {
  fs.mkdir(getDirName(path), { recursive: true}, function (err) {
    if (err) return;

    fs.writeFileSync(path, contents, 'utf8');
  });
}

function findFrontmatterEndIndex(markdown) {
  const frontmatterPattern = /^---\s*[\s\S]*?---\s*/m;
  const match = frontmatterPattern.exec(markdown);
  if (match) {
    return match.index + match[0].length;
  }
  return 0;
}

function toPascalCase(string) {
  return `${string}`
    .toLowerCase()
    .replace(new RegExp(/[-_]+/, 'g'), ' ')
    .replace(new RegExp(/[^\w\s]/, 'g'), '')
    .replace(
      new RegExp(/\s+(.)(\w*)/, 'g'),
      ($1, $2, $3) => `${$2.toUpperCase() + $3}`
    )
    .replace(new RegExp(/\w/), s => s.toUpperCase());
}

function test(name, value, expectation) {
  if (value !== expectation) {
    throw new Error(`❌ ${name} failed. Expecting:\n${expectation} but instead got:\n${value}`)
  }

  console.log(`✅ ${name} test passed`)
}

module.exports = {
  readAllFilesFromDirectory,
  writeFile,
  findFrontmatterEndIndex,
  toPascalCase,
  test
};