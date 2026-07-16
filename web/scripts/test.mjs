#!/usr/bin/env node
/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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
import { spawnSync } from 'node:child_process';

// Routes `pnpm test <args>` during the jest -> vitest migration. Vitest owns *.vitest.{ts,tsx} files. Everything
// else (*.test.* files, name patterns, flags) goes to Jest, matching the pre-wrapper `jest <args>` behavior. Naming
// both kinds of file runs both runners. With no args, both full suites run.
//
// For advanced flag combos across a mixture (e.g. `-t <pattern>` alongside both file kinds), call `pnpm test-jest`
// or `pnpm test-vitest` directly, since a flag can only be forwarded verbatim to each selected runner.

const args = process.argv.slice(2);
const isVitestFile = arg => arg.includes('.vitest.');

const flags = args.filter(arg => arg.startsWith('-'));
const positionals = args.filter(arg => !arg.startsWith('-'));
const vitestFiles = positionals.filter(isVitestFile);
const jestFiles = positionals.filter(arg => !isVitestFile(arg));

const commands = [];
if (args.length === 0) {
  // Bare `pnpm test`: run both full suites, unchanged from before the wrapper existed.
  commands.push(['jest', []], ['vitest', ['run']]);
} else {
  // Run Jest for any non-vitest arg, or when only flags were passed so `pnpm test --watch` still starts Jest.
  if (jestFiles.length > 0 || vitestFiles.length === 0) {
    commands.push(['jest', [...flags, ...jestFiles]]);
  }
  if (vitestFiles.length > 0) {
    commands.push(['vitest', ['run', ...flags, ...vitestFiles]]);
  }
}

let exitCode = 0;
for (const [command, commandArgs] of commands) {
  const { status, error } = spawnSync(command, commandArgs, {
    stdio: 'inherit',
  });
  if (error) {
    console.error(error);
    process.exit(1);
  }
  if (status !== 0 && exitCode === 0) {
    exitCode = status ?? 1;
  }
}

process.exit(exitCode);
