package main

import (
	"regexp"
	"strings"
)

// TODO: use the standard libray in place of this
// function* readAllFilesFromDirectory(dir) {
//   const files = fs.readdirSync(dir, { withFileTypes: true });
//
//   for (const file of files) {
//     if (file.isDirectory()) {
//       yield* readAllFilesFromDirectory(path.join(dir, file.name));
//     } else {
//       yield path.join(dir, file.name);
//     }
//   }
// }

// TODO: use the standard library in place of this
// function writeFile(path, contents) {
//   fs.mkdir(getDirName(path), { recursive: true}, function (err) {
//     if (err) return;
//
//     fs.writeFileSync(path, contents, 'utf8');
//   });
// }

func findFrontmatterEndIndex(markdown string) int {
	frontmatterPattern := regexp.MustCompile(`^---\s*[\s\S]*?---\s*`)
	loc := frontmatterPattern.FindStringIndex(markdown)
	if loc == nil {
		return 0
	}
	return loc[1]
}

func toPascalCase(in string) string {
	sep := regexp.MustCompile("[-_]+")
	w := regexp.MustCompile(`\s+(.)`)
	s := strings.ToLower(in)
	s = sep.ReplaceAllString(s, " ")
	s = strings.TrimLeft(s, `\w\s`)
	return w.ReplaceAllStringFunc(s, strings.ToUpper)
}
