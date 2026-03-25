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

import fs from 'node:fs';

import { events } from 'teleport/Audit/fixtures';

import { formatters } from '../makeEvent';
import {
  createReferencePage,
  eventsWithoutExamples,
  removeUnknowns,
} from './gen-event-reference.js';

const introParagraph = `{/*cSpell:disable*/}

{/* Formatted event examples sometimes include different capitalization than
what we standardize on in the docs*/}
{/* vale messaging.capitalization = NO */}

Teleport components emit audit events to record activity within the cluster. 

Audit event payloads have an \`event\` field that describes the event, which is
often an operation performed against a dynamic resource (e.g.,
\`access_list.create\` for the creation of an Access List) or some other user
behavior, such as a local user login (\`user.login\`). The \`code\` field
includes a string with pattern \`[A-Z0-9]{6}\` that is unique to an audit event,
such as \`TAP03I\` for the creation of an application resource.

In some cases, an audit event describes both a success state and a failure
state, while the \`event\` field is the same for both states. In this case, the
\`code\` field differs between states. For example, \`access_list.create\`
describes both successful and failed Access List creations, while the success
event has code \`TAL001I\` and the failure has code \`TAL001E\`. For other
events, like \`db.session.query.failed\` and \`db.session.query\`, the event
type describes only the success or failure state.

You can set up Teleport to export audit events to third-party services for
storage, visualization, and analysis. For more information, read [Exporting
Teleport Audit Events](
../zero-trust-access/export-audit-events/export-audit-events.mdx).`;

if (process.argv.length !== 3) {
  console.error(
    'The argument of the script must be the path of the audit event reference page.'
  );
  process.exit(1);
}

console.log('Writing an audit event reference page to ', process.argv[2]);

const noExampleEvents = eventsWithoutExamples(events, formatters);
noExampleEvents.forEach(e => {
  console.error(
    `Warning: adding an entry for ${e.code} (${e.raw.event}) with no example. Add a test fixture to web/packages/teleport/src/Audit/fixtures/index.ts`
  );
});

fs.writeFileSync(
  process.argv[2],
  createReferencePage(
    removeUnknowns(events, formatters).concat(noExampleEvents),
    introParagraph
  )
);
