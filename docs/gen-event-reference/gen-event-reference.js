// createEventSection takes a JSON document that defines an audit event test
// fixture and returns a string that contains an H2-level section to describe
// the event.
//
// See web/packages/teleport/src/Audit/fixtures/index.ts for the
// structure of an audit event test fixture.
export function createEventSection(event) {
  return `## ${event.raw.event}
${event.codeDesc == 'Unknown' ? '' : '\n' + event.codeDesc + '\n'}
Example:

\`\`\`json
${JSON.stringify(event.raw, null, 2)}
\`\`\`
`;
}

// createMultipleEventsSection takes an array of JSON documents that define an
// audit event test fixture and returns a string that contains an H2-level
// section to describe the event. It includes a separate H3 section for each
// event code.
//
// See web/packages/teleport/src/Audit/fixtures/index.ts for the structure of an
// audit event test fixture.
export function createMultipleEventsSection(events) {
  return events.reduce(
    (accum, event) => {
      return (
        accum +
        '\n' +
        `### ${event.raw.code}
${event.codeDesc == 'Unknown' ? '' : '\n' + event.codeDesc + '\n'}
Example:

\`\`\`json
${JSON.stringify(event.raw, null, 2)}
\`\`\`
`
      );
    },
    `## ${events[0].raw.event}

There are multiple events with the \`${events[0].raw.event}\` type.
`
  );
}

// createReferencePage takes an array of JSON documents that define an audit
// event test fixture and returns a string that contains the text of an audit
// event reference guide.
//
// introParagraph contains the text of the introductory paragraph to include in
// the guide.
//
// See web/packages/teleport/src/Audit/fixtures/index.ts for the structure of an
// audit event test fixture.
export function createReferencePage(jsonEvents, introParagraph) {
  const codeSet = new Set();
  let result = jsonEvents;
  result.sort((a, b) => {
    if (a.raw.event < b.raw.event) {
      return -1;
    } else {
      return 1;
    }
  });
  const events = new Map();
  result.forEach(e => {
    if (codeSet.has(e.raw.code)) {
      return;
    }
    const codeData = events.get(e.raw.event);
    codeSet.add(e.raw.code);
    if (!codeData) {
      events.set(e.raw.event, [e]);
      return;
    }
    codeData.push(e);
  });

  return events.keys().reduce(
    (accum, current) => {
      const codes = events.get(current);
      if (codes.length == 1) {
        return accum + '\n' + createEventSection(codes[0]);
      }
      return accum + '\n' + createMultipleEventsSection(codes);
    },
    `---
title: "Audit Event Reference"
description: "Provides a comprehensive list of Teleport audit events and their fields."
---
{/* Generated file. Do not edit. */}
{/* To regenerate, navigate to docs/gen-event-reference and run pnpm gen-docs */}

${introParagraph}
`
  );
}
