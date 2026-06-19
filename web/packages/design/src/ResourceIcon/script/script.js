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

/* oxlint-disable no-console */

/**
 * This script processes the SVGs in `design/ResourceIcon/assets`.
 *
 * A `<name>-dark.svg` / `<name>-light.svg` pair whose only difference is a single
 * flipping color ("monochrome") is COLLAPSED into one `currentColor` `<name>.svg`
 * and turned into a generated React component under `./MonoIcons`, themed purely
 * via the `html.dark` / `html.light` class. The flipping colors are recorded in
 * `icons.json` (defaulting to black/white, so only brand colors are stored).
 *
 * Pairs whose artworks genuinely differ are left untouched and continue to render
 * through the hand-authored `resourceIconSpecs.ts` (as a themed image swap).
 *
 * Run via `pnpm process-icons`. Do not edit anything under `./MonoIcons` by hand.
 */

const fs = require('fs');
const path = require('path');

const { optimize } = require('svgo');

const basePath = process.env.RI_BASE ?? 'web/packages/design/src/ResourceIcon';

const assetsPath = path.join(basePath, 'assets');
const iconsDir = path.join(basePath, 'MonoIcons');
const iconsTsPath = path.join(basePath, 'icons.ts');
const colorsConfigPath = path.join(basePath, 'icons.json');

const WHITE = new Set(['#fff', '#ffffff', 'white']);
const BLACK = new Set(['#000', '#000000', 'black']);

const isWhite = c => WHITE.has(c);
const isBlack = c => BLACK.has(c);

const LICENSE = `/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
 */`;

const GENERATED_NOTICE = `/*

THIS FILE IS GENERATED. DO NOT EDIT.

*/`;

function run() {
  const colors = loadColors();

  const files = fs.readdirSync(assetsPath).filter(f => f.endsWith('.svg'));
  const fileSet = new Set(files);

  // Find collapsible pairs and collapse them.
  const monoBases = new Set();

  for (const file of files) {
    if (!isDarkFile(file)) {
      continue;
    }

    const base = file.slice(0, -'-dark.svg'.length);
    const lightFile = `${base}-light.svg`;

    if (!fileSet.has(lightFile)) {
      continue;
    }

    const darkSvg = fs.readFileSync(path.join(assetsPath, file), 'utf-8');
    const lightSvg = fs.readFileSync(path.join(assetsPath, lightFile), 'utf-8');

    const verdict = classifyPair(darkSvg, lightSvg);

    if (verdict.kind !== 'mono') {
      continue;
    }

    const svg = collapseSvg(darkSvg);

    fs.writeFileSync(path.join(assetsPath, `${base}.svg`), svg);
    fs.unlinkSync(path.join(assetsPath, file));
    fs.unlinkSync(path.join(assetsPath, lightFile));

    fileSet.delete(file);
    fileSet.delete(lightFile);
    fileSet.add(`${base}.svg`);

    monoBases.add(base);

    const isDefault = isWhite(verdict.dark) && isBlack(verdict.light);

    if (isDefault) {
      delete colors[base];
    } else {
      colors[base] = { dark: verdict.dark, light: verdict.light };
    }

    console.log(
      `Collapsed ${base} (dark ${verdict.dark} / light ${verdict.light}).`
    );
  }

  // Any single SVG whose only paint is `currentColor` is also a mono icon (e.g. authored directly, or collapsed on a previous run).
  for (const file of fileSet) {
    if (isDarkFile(file) || isLightFile(file)) {
      continue;
    }

    const base = file.slice(0, -'.svg'.length);
    if (monoBases.has(base)) {
      continue;
    }

    const svg = fs.readFileSync(path.join(assetsPath, file), 'utf-8');
    if (/currentColor/i.test(svg) && foregroundColors(svg).size === 0) {
      monoBases.add(base);
    }
  }

  // Regenerate the component directory.
  fs.rmSync(iconsDir, { recursive: true, force: true });
  fs.mkdirSync(iconsDir, { recursive: true });

  const sorted = [...monoBases].sort();

  for (const base of sorted) {
    const svg = fs.readFileSync(path.join(assetsPath, `${base}.svg`), 'utf-8');

    const { data } = optimize(svg, { multipass: true });

    const viewBox = (data.match(/viewBox="([^"]+)"/) || [])[1] || '0 0 24 24';
    const inner = stripGroups(stripDefs(stripSvgTags(data))).trim();
    const name = toComponentName(base);

    fs.writeFileSync(
      path.join(iconsDir, `${name}.tsx`),
      componentSource(base, viewBox, inner, colors[base])
    );
  }

  console.log(`Generated ${sorted.length} monochrome icon components.`);

  // Barrel.
  const exports = sorted
    .map(
      b => `export { ${toComponentName(b)} } from './${toComponentName(b)}';`
    )
    .join('\n');

  fs.writeFileSync(
    path.join(iconsDir, 'index.ts'),
    `${LICENSE}\n\n${GENERATED_NOTICE}\n\n${exports}\n`
  );

  // Drop now-dead url exports (the collapsed pairs) from icons.ts, leaving the hand-curated static/themed exports untouched.
  pruneIconsTs();

  // Persist extracted brand colors.
  const orderedColors = {};
  for (const base of Object.keys(colors).sort()) {
    orderedColors[base] = colors[base];
  }

  fs.writeFileSync(
    colorsConfigPath,
    JSON.stringify({ colors: orderedColors }, null, 2) + '\n'
  );

  console.log('Wrote icons.json.');

  warnUnreferencedIcons(sorted);
}

// Warn about generated monochrome components that no `resourceIconSpecs.ts` entry references yet.
// The component name is deterministic, so this reliably catches "added an icon but forgot to wire it up" without the
// generator having to edit the curated, hand-authored registry.
function warnUnreferencedIcons(monoBases) {
  const specPath = path.join(basePath, 'resourceIconSpecs.ts');
  if (!fs.existsSync(specPath)) {
    return;
  }

  const spec = fs.readFileSync(specPath, 'utf-8');
  const missing = monoBases.filter(
    base => !new RegExp(`Icons\\.${toComponentName(base)}\\b`).test(spec)
  );

  if (missing.length === 0) {
    return;
  }

  console.warn(
    `\n⚠ ${missing.length} generated icon(s) are not referenced in resourceIconSpecs.ts.`
  );
  console.warn(
    '  Add an entry for each so it is available via <ResourceIcon name="…" /> ' +
      '(the key may be curated):'
  );

  for (const base of missing) {
    console.warn(`    ${base}: mono(Icons.${toComponentName(base)}),`);
  }

  console.warn('');
}

function pruneIconsTs() {
  if (!fs.existsSync(iconsTsPath)) {
    return;
  }

  const lines = fs.readFileSync(iconsTsPath, 'utf-8').split('\n');
  const kept = lines.filter(line => {
    const m = line.match(/from '\.\/assets\/([^']+)'/);
    if (!m) {
      return true;
    }

    return fs.existsSync(path.join(assetsPath, m[1]));
  });

  fs.writeFileSync(iconsTsPath, kept.join('\n'));

  console.log(
    `Pruned icons.ts (${lines.length - kept.length} dead exports removed).`
  );
}

run();

// Build a clean, single-color `currentColor` SVG from a variant: drop defs, unwrap groups (their clip-path/fill is
// inert here), strip fills so geometry inherits the root `currentColor`.
function collapseSvg(variantSvg) {
  const { data } = optimize(variantSvg, { multipass: true });
  const viewBox = (data.match(/viewBox="([^"]+)"/) || [])[1] || '0 0 24 24';

  const inner = stripCommonSvgAttrs(
    stripGroups(stripDefs(stripSvgTags(data)))
  ).trim();

  const collapsed = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="${viewBox}" fill="currentColor">${inner}</svg>`;

  // Re-run through SVGO so the written file is byte-identical to what `optimize-resource-icons` (the SVGO CLI) would
  // produce, otherwise the next `pnpm process-icons` would re-optimize it and `make icons-up-to-date` would fail.
  return optimize(collapsed, { multipass: true }).data;
}

function loadColors() {
  if (!fs.existsSync(colorsConfigPath)) {
    return {};
  }

  try {
    return JSON.parse(fs.readFileSync(colorsConfigPath, 'utf-8')).colors || {};
  } catch {
    return {};
  }
}

function toComponentName(base) {
  return base
    .split(/[^a-zA-Z0-9]+/)
    .filter(Boolean)
    .map(p => p[0].toUpperCase() + p.slice(1))
    .join('');
}

function jsxify(inner) {
  return stripCommonSvgAttrs(inner)
    .replace(/fill-rule/g, 'fillRule')
    .replace(/clip-rule/g, 'clipRule')
    .replace(/stroke-linecap/g, 'strokeLinecap')
    .replace(/stroke-linejoin/g, 'strokeLinejoin')
    .replace(/stroke-width/g, 'strokeWidth');
}

function colorProps(colors) {
  const props = [];
  if (colors && !isWhite(colors.dark)) {
    props.push(`darkColor="${colors.dark}"`);
  }
  if (colors && !isBlack(colors.light)) {
    props.push(`lightColor="${colors.light}"`);
  }
  return props.join('\n      ');
}

function componentSource(base, viewBox, inner, colors) {
  const name = toComponentName(base);
  const colorLines = colorProps(colors);
  return `${LICENSE}

import { MonoIcon, type MonoIconProps } from '../MonoIcon';

${GENERATED_NOTICE}

export const ${name} = (props: MonoIconProps) => (
  <MonoIcon
    className="resource-icon resource-icon-${base.toLowerCase().replace(/[^a-z0-9]+/g, '-')}"
    viewBox="${viewBox}"
    ${colorLines ? colorLines + '\n    ' : ''}{...props}
  >
    ${jsxify(inner)}
  </MonoIcon>
);
`;
}

function stripSvgTags(svg) {
  return svg.replace(/^[\s\S]*?<svg[^>]*>/, '').replace(/<\/svg>\s*$/, '');
}

function stripDefs(svg) {
  return svg.replace(/<defs>[\s\S]*?<\/defs>/g, '');
}

function stripCommonSvgAttrs(svg) {
  return svg.replace(/\s(?:fill|clip-path|style)="[^"]*"/g, '');
}

function stripGroups(svg) {
  return svg.replace(/<g[^>]*>/g, '').replace(/<\/g>/g, '');
}

// Geometry signature: everything except ids, color references and fills, so two variants that differ only by color
// compare equal.
function geometrySignature(svg) {
  return stripDefs(svg)
    .replace(/\sid="[^"]*"/g, '')
    .replace(/url\(#[^)]*\)/g, 'url(#X)')
    .replace(/fill="[^"]*"/g, '')
    .replace(/fill:\s*[^;"\s]+/g, '')
    .replace(/\s+/g, ' ')
    .trim();
}

// Real foreground colors used in the body (excludes `none`, `currentColor`, gradient/pattern refs and anything inside <defs>).
function foregroundColors(svg) {
  const body = stripDefs(svg);
  const colors = new Set();

  let m;
  const attr = /fill="([^"]+)"/g;
  while ((m = attr.exec(body))) {
    addColor(colors, m[1]);
  }

  const style = /fill:\s*([^;"\s]+)/g;
  while ((m = style.exec(body))) {
    addColor(colors, m[1]);
  }

  return colors;
}

function addColor(set, raw) {
  const c = raw.trim().toLowerCase();
  if (c === 'none' || c === 'currentcolor') {
    return;
  }

  set.add(c);
}

// Classify a dark/light pair.
// Returns { kind: 'mono', dark, light } or { kind: 'themed' }.
function classifyPair(darkSvg, lightSvg) {
  if (geometrySignature(darkSvg) !== geometrySignature(lightSvg)) {
    return { kind: 'themed' };
  }

  // Transforms can't be safely flattened by the naive collapse below.
  if (/transform=/.test(stripDefs(darkSvg))) {
    return { kind: 'themed' };
  }

  const d = foregroundColors(darkSvg);
  const l = foregroundColors(lightSvg);
  const all = [...d, ...l];

  if (all.some(c => c.startsWith('url('))) {
    return { kind: 'themed' };
  }

  if (d.size !== 1 || l.size !== 1) {
    return { kind: 'themed' };
  }

  const dark = [...d][0];
  const light = [...l][0];

  if (dark === light) {
    return { kind: 'themed' };
  }

  return { kind: 'mono', dark, light };
}

function isDarkFile(file) {
  return file.endsWith('-dark.svg');
}

function isLightFile(file) {
  return file.endsWith('-light.svg');
}
