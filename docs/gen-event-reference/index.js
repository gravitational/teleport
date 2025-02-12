import { default as fs } from 'node:fs';

import { events } from './dist/fixtures.js';
import { formatters } from './dist/formatters.js';
import {
  createReferencePage,
  eventsWithoutExamples,
  removeUnknowns,
} from './gen-event-reference.js';

const introParagraph = `{/*cSpell:disable*/}

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
../admin-guides/management/export-audit-events/export-audit-events.mdx).`;

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
