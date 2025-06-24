/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

/* eslint-disable no-console */

/*

This script uses the SVG files in the `design/Icon/assets` directory to generate Icon React components to be used in the app.
It also generates the `index.ts` file which re-exports all the icons, as well as the `Icons.story.tsx` file containing a story
with all the icons so they can be viewed in Storybook.

Everything that is generated is based on the SVG files in the `assets` directory. If you wish to add, remove, or edit any icons,
only do so in that directory and then run the script. Do not edit any of the generated files.
*/

const fs = require('fs');
const path = require('path');

const basePath = 'web/packages/design/src/Icon';

const svgAssetsPath = basePath + '/optimized';
const iconComponentTemplate = basePath + '/script/IconTemplate.txt';
const storybookTemplatePath = basePath + '/script/StoryTemplate.txt';

// Creates an Icon React component from an SVG and outputs it to a `.tsx` file in `./Icon`.
// The name of the SVG file will be used as the icon name.
function createIconComponent(svgFilePath) {
  const svgContent = fs.readFileSync(svgFilePath, 'utf-8');
  const templateContent = fs.readFileSync(iconComponentTemplate, 'utf-8');

  // Get `path` elements.
  const pathElements = svgContent.match(/<path[^>]*\/>/g);

  // Remove `fill` attributes from the path elements, and convert `fill-rule` and `clip-rule` to JSX attributes.
  const processedPathElements = pathElements.map(path =>
    path
      .replace(/fill="[^"]*"/g, '')
      .replace(/fill-rule/g, 'fillRule')
      .replace(/clip-rule/g, 'clipRule')
  );

  const iconName = path.basename(svgFilePath, '.svg');
  const output = templateContent
    .replace('{PATHS}', processedPathElements.join('\n'))
    .replace('{ICON_NAME}', iconName)
    .replace('{CLASS_NAME}', `icon-${iconName.toLowerCase()}`);

  // Write output to `.tsx` file.
  const outputFilePath = path.join(basePath, '/Icons/', `${iconName}.tsx`);
  fs.writeFileSync(outputFilePath, output);
}

// Iterates through all the SVG files in the `assets` directory and creates an Icon component for each.
function processSVGFiles() {
  // Get all SVG files.
  const svgFiles = fs
    .readdirSync(svgAssetsPath)
    .filter(file => file.endsWith('.svg'));

  if (svgFiles.length === 0) {
    console.error(`No SVG files found in ${svgAssetsPath}.`);
    return;
  }

  // Before generating Icon components, delete all the existing ones. This is to prevent Icon components for now-deleted SVG files from persisting.
  const iconComponentsDir = path.join(basePath, '/Icons/');
  const files = fs.readdirSync(iconComponentsDir);
  for (const file of files) {
    const filePath = path.join(iconComponentsDir, file);
    fs.unlinkSync(filePath);
  }

  svgFiles.forEach(svgFile => {
    const svgFilePath = path.join(svgAssetsPath, svgFile);
    createIconComponent(svgFilePath);
  });

  console.log('Generated Icons.');
}

// Generates the `index.ts` file which re-exports all the Icon components.
function generateIconExportsFile() {
  const exportTemplate = "export { {ICON_NAME} } from './Icons/{ICON_NAME}';";

  const svgFiles = fs
    .readdirSync(svgAssetsPath)
    .filter(file => file.endsWith('.svg'));

  if (svgFiles.length === 0) {
    console.error(`No SVG files found in ${svgAssetsPath}.`);
    return;
  }

  const lines = svgFiles.map(fileName => {
    const iconName = path.parse(fileName).name;
    return exportTemplate.replace(/{ICON_NAME}/g, iconName);
  });

  const output = `/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

/*

THIS FILE IS GENERATED. DO NOT EDIT.

*/

export { Icon } from './Icon';

${lines.join('\n')}
`;

  // Write output to `Icons.tsx` file.
  fs.writeFileSync(path.join(basePath, 'index.ts'), output);
  console.log('Generated index.ts file.');
}

// Generates the `Icons.story.tsx` with a story that contains all the icons to be viewed in Storybook.
function generateStories() {
  // Read the template file
  let content = fs.readFileSync(storybookTemplatePath, 'utf-8');

  const lineTemplate =
    '<IconBox IconCmpt={Icon.{ICON_NAME}} text="{ICON_NAME}" />';

  // Get all SVG files in the directory
  const svgFiles = fs
    .readdirSync(svgAssetsPath)
    .filter(file => file.toLowerCase().endsWith('.svg'));

  if (svgFiles.length === 0) {
    console.error(`No SVG files found in the assets directory.`);
    return;
  }

  svgFiles.forEach((svgFile, index) => {
    const iconName = path.parse(svgFile).name;

    // Whether this icon is the last in the list.
    const isLast = index === svgFiles.length - 1;

    // Generate the new line. This works by creating the `IconBox` component using the template, replacing `{ICON_NAME}` with the correct icon name,
    // and then pushing the template line one line down. The next iteration will then do the same. If the icon is the last in the list, it won't
    // add the template in the next line.
    let newLine;
    if (!isLast) {
      newLine = `${lineTemplate.replace(/{ICON_NAME}/g, iconName)}
    ${lineTemplate}`;
    } else {
      newLine = `${lineTemplate.replace(/{ICON_NAME}/g, iconName)}`;
    }

    // Replace the template line with the lines created above.
    const updatedContent = content.replace(lineTemplate, newLine);

    // Update content before the next iteration.
    content = updatedContent;
  });

  // Write the content to `Icons.story.tsx` file.
  const outputFilePath = path.join(basePath, '/Icons.story.tsx');
  fs.writeFileSync(outputFilePath, content, 'utf-8');

  console.log('Generated stories.');
}

processSVGFiles();
generateIconExportsFile();
generateStories();
