/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import fs from "node:fs";
import config from "./config.json";
import { events } from "teleport/Audit/fixtures";

import { formatters } from "../makeEvent";
import {
  createReferencePages,
  eventsWithoutExamples,
  fixtureTypeMismatches,
  removeUnknowns,
} from './gen-event-reference.js';

const fixturePath = 'web/packages/teleport/src/Audit/fixtures/index.ts';
const formatterPath = 'web/packages/teleport/src/services/audit/makeEvent.ts';

if (process.argv.length !== 3) {
  console.error(
    'The argument of the script must be the index of the audit event reference pages.'
  );
  process.exit(1);
}

const auditEventsDir = process.argv[2].split('/').slice(0, -1).join('/');

console.log('Writing audit event reference pages to ', auditEventsDir);

const noExampleEvents = eventsWithoutExamples(events, formatters);
noExampleEvents.forEach(e => {
  console.error(
    `Warning: adding an entry for ${e.code} (${e.raw.event}) with no example. Add a test fixture to web/packages/teleport/src/Audit/fixtures/index.ts`
  );
});

const mismatches = fixtureTypeMismatches(events, formatters);
if (mismatches.length > 0) {
  mismatches.forEach(m => {
    console.error(
      `Fatal: event formatter code ${m.code} has type ${m.formatterType}, but its corresponding fixture has type ${m.fixtureType}. Ensure the formatter at ${formatterPath} matches the fixture at ${fixturePath}`
    );
  });
  process.exit(1);
}

const referencePages = createReferencePages(
  removeUnknowns(events, formatters).concat(noExampleEvents),
  config,
);

referencePages.forEach((page) => {
  const filePath = `${auditEventsDir}/${page.id}.mdx`;
  fs.writeFileSync(filePath, page.content);
});
