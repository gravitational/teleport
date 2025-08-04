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

import { Event, Formatters } from './types';

// eventsWithoutExamples returns an array of event objects based on the
// elements in formatters that do not have corresponding examples in fixtures.
export function eventsWithoutExamples(
  fixtures: Event[],
  formatters: Formatters
): Event[] {
  const fixtureCodes = new Set(fixtures.map(fixture => fixture.code));
  return (Object.keys(formatters) as Array<keyof Formatters>).reduce(
    (accum, current) => {
      if (fixtureCodes.has(current)) {
        return accum;
      }
      accum.push({
        codeDesc: formatters[current].desc,
        code: current,
        raw: {
          event: formatters[current].type,
        },
      });
      return accum;
    },
    [] as Event[]
  );
}

// codeDesc returns the description of the given event, depending on whether the
// description is a function or a string.
function codeDesc(event: Event): string {
  if (typeof event.codeDesc == 'function') {
    return event.codeDesc({ code: event.code, event: event.raw.event });
  }
  return event.codeDesc;
}

// removeUnknowns removes any event fixtures in the fixtures array that do not
// have a formatter.
export function removeUnknowns(
  fixtures: Event[],
  formatters: Formatters
): Event[] {
  return fixtures.filter(r => r.code in formatters);
}

// exampleOrAttributes returns a string to include in a reference entry for an
// audit event that describes the event's attributes.
//
// The generator expects all event objects to include a raw.event attribute, and
// events with full examples include additional fields in the raw object. If
// there is an example available for the event, we include the example,
// formatted as JSON. Otherwise, we print only the event code and type.
export function exampleOrAttributes(event: Event): string {
  if (Object.keys(event.raw).length > 1) {
    return `Example:

\`\`\`json
${JSON.stringify(event.raw, null, 2)}
\`\`\``;
  }
  return `Code: \`${event.code}\`

Event: \`${event.raw.event}\``;
}

// createEventSection takes a JSON document that defines an audit event test
// fixture and returns a string that contains an H2-level section to describe
// the event.
export function createEventSection(event: Event): string {
  return `## ${event.raw.event}

${codeDesc(event) + '\n'}
${exampleOrAttributes(event)}
`;
}

// createMultipleEventsSection takes an array of JSON documents that define an
// audit event test fixture and returns a string that contains an H2-level
// section to describe the event. It includes a separate H3 section for each
// event code.
//
// See web/packages/teleport/src/Audit/fixtures/index.ts for the structure of an
// audit event test fixture.
export function createMultipleEventsSection(events: Event[]): string {
  return events.reduce(
    (accum, event) => {
      return (
        accum +
        '\n' +
        `### ${event.code}

${codeDesc(event) + '\n'}
${exampleOrAttributes(event)}
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
export function createReferencePage(
  jsonEvents: Event[],
  introParagraph: string
): string {
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
    if (codeSet.has(e.code)) {
      return;
    }
    const codeData = events.get(e.raw.event);
    codeSet.add(e.code);
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
