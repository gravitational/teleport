import {
  createEventSection,
  createReferencePage,
} from './gen-event-reference.js';

describe('createEventSection', () => {
  const testCases = [
    {
      description: 'Example event with full information',
      event: {
        codeDesc: 'Credit Card Deleted',
        message: 'User [root] has deleted a credit card',
        id: '056517e0-f7e1-4286-b437-c75f3a865af4',
        code: 'TBL01I',
        user: 'root',
        time: new Date('2021-03-18T16:28:51.219Z'),
        raw: {
          cluster_name: 'some-name',
          code: 'TBL01I',
          ei: 0,
          event: 'billing.delete_card',
          time: '2021-03-18T16:28:51.219Z',
          uid: '056517e0-f7e1-4286-b437-c75f3a865af4',
          user: 'root',
        },
      },
      expected: `## billing.delete_card

Credit Card Deleted

Example:

\`\`\`json
{
  "cluster_name": "some-name",
  "code": "TBL01I",
  "ei": 0,
  "event": "billing.delete_card",
  "time": "2021-03-18T16:28:51.219Z",
  "uid": "056517e0-f7e1-4286-b437-c75f3a865af4",
  "user": "root"
}
\`\`\`
`,
    },
    {
      description: 'Event description of Unknown',
      event: {
        codeDesc: 'Unknown',
        raw: {
          event: 'billing.delete_card',
        },
      },
      expected: `## billing.delete_card

Example:

\`\`\`json
{
  "event": "billing.delete_card"
}
\`\`\`
`,
    },
  ];

  test.each(testCases)('$description', testCase => {
    expect(createEventSection(testCase.event)).toEqual(testCase.expected);
  });
});

describe('createReferencePage', () => {
  const introParagraph = 'This is an intro paragraph.';

  test('formats a list of events as expected', () => {
    const events = [
      {
        codeDesc: 'Kubernetes Created',
        message:
          'User [05ff66c9-a948-42f4-af0e-a1b6ba62561e.root] created Kubernetes cluster [kube-local]',
        id: '9d37514f-aef5-426f-9fda-31fd35d070f5',
        code: 'T3010I',
        user: '05ff66c9-a948-42f4-af0e-a1b6ba62561e.root',
        time: new Date('2022-09-08T15:42:36.005Z'),
        raw: {
          cluster_name: 'root',
          code: 'T3010I',
          kube_labels: [Object],
          ei: 0,
          event: 'kube.create',
          expires: '0001-01-01T00:00:00Z',
          name: 'kube-local',
          time: '2022-09-08T15:42:36.005Z',
          uid: '9d37514f-aef5-426f-9fda-31fd35d070f5',
          user: '05ff66c9-a948-42f4-af0e-a1b6ba62561e.root',
        },
      },
      {
        codeDesc: 'Kubernetes Updated',
        message:
          'User [05ff66c9-a948-42f4-af0e-a1b6ba62561e.root] updated Kubernetes cluster [kube-local]',
        id: 'fe631a5a-6418-49d6-99e7-5280654663ec',
        code: 'T3011I',
        user: '05ff66c9-a948-42f4-af0e-a1b6ba62561e.root',
        time: new Date('2022-09-08T15:42:36.005Z'),
        raw: {
          cluster_name: 'root',
          code: 'T3011I',
          kube_labels: [Object],
          ei: 0,
          event: 'kube.update',
          expires: '0001-01-01T00:00:00Z',
          name: 'kube-local',
          time: '2022-09-08T15:42:36.005Z',
          uid: 'fe631a5a-6418-49d6-99e7-5280654663ec',
          user: '05ff66c9-a948-42f4-af0e-a1b6ba62561e.root',
        },
      },
    ];

    const expected = `---
title: "Audit Event Reference"
description: "Provides a comprehensive list of Teleport audit events and their fields."
---
{/* Generated file. Do not edit. */}
{/* To regenerate, navigate to docs/gen-event-reference and run pnpm gen-docs */}

This is an intro paragraph.

## kube.create

Kubernetes Created

Example:

\`\`\`json
{
  "cluster_name": "root",
  "code": "T3010I",
  "kube_labels": [
    null
  ],
  "ei": 0,
  "event": "kube.create",
  "expires": "0001-01-01T00:00:00Z",
  "name": "kube-local",
  "time": "2022-09-08T15:42:36.005Z",
  "uid": "9d37514f-aef5-426f-9fda-31fd35d070f5",
  "user": "05ff66c9-a948-42f4-af0e-a1b6ba62561e.root"
}
\`\`\`

## kube.update

Kubernetes Updated

Example:

\`\`\`json
{
  "cluster_name": "root",
  "code": "T3011I",
  "kube_labels": [
    null
  ],
  "ei": 0,
  "event": "kube.update",
  "expires": "0001-01-01T00:00:00Z",
  "name": "kube-local",
  "time": "2022-09-08T15:42:36.005Z",
  "uid": "fe631a5a-6418-49d6-99e7-5280654663ec",
  "user": "05ff66c9-a948-42f4-af0e-a1b6ba62561e.root"
}
\`\`\`
`;
    const actual = createReferencePage(events, introParagraph);
    expect(actual).toEqual(expected);
  });

  test('orders event sections by H2', () => {
    const events = [
      {
        codeDesc: 'Event C',
        raw: {
          event: 'event.c',
          code: 'GHI123',
        },
      },
      {
        codeDesc: 'Event A',
        raw: {
          event: 'event.a',
          code: 'ABC123',
        },
      },
      {
        codeDesc: 'Event B',
        raw: {
          event: 'event.b',
          code: 'DEF123',
        },
      },
    ];

    const expected = `---
title: "Audit Event Reference"
description: "Provides a comprehensive list of Teleport audit events and their fields."
---
{/* Generated file. Do not edit. */}
{/* To regenerate, navigate to docs/gen-event-reference and run pnpm gen-docs */}

This is an intro paragraph.

## event.a

Event A

Example:

\`\`\`json
{
  "event": "event.a",
  "code": "ABC123"
}
\`\`\`

## event.b

Event B

Example:

\`\`\`json
{
  "event": "event.b",
  "code": "DEF123"
}
\`\`\`

## event.c

Event C

Example:

\`\`\`json
{
  "event": "event.c",
  "code": "GHI123"
}
\`\`\`
`;
    const actual = createReferencePage(events, introParagraph);
    expect(actual).toEqual(expected);
  });

  test('includes H3 sections for event codes if there are duplicate types', () => {
    const events = [
      {
        codeDesc: 'Event A',
        raw: {
          event: 'event.a',
          code: 'ABC123',
        },
      },
      {
        codeDesc: 'Event A failed',
        raw: {
          event: 'event.a',
          code: 'ABC456',
        },
      },
    ];

    const expected = `---
title: "Audit Event Reference"
description: "Provides a comprehensive list of Teleport audit events and their fields."
---
{/* Generated file. Do not edit. */}
{/* To regenerate, navigate to docs/gen-event-reference and run pnpm gen-docs */}

This is an intro paragraph.

## event.a

There are multiple events with the \`event.a\` type.

### ABC123

Event A

Example:

\`\`\`json
{
  "event": "event.a",
  "code": "ABC123"
}
\`\`\`

### ABC456

Event A failed

Example:

\`\`\`json
{
  "event": "event.a",
  "code": "ABC456"
}
\`\`\`
`;
    const actual = createReferencePage(events, introParagraph);
    expect(actual).toEqual(expected);
  });

  test('deduplicates event codes', () => {
    const events = [
      {
        codeDesc: 'Event A',
        raw: {
          event: 'event.a',
          code: 'ABC123',
        },
      },
      {
        codeDesc: 'Event A',
        raw: {
          event: 'event.a',
          code: 'ABC123',
        },
      },
    ];

    const expected = `---
title: "Audit Event Reference"
description: "Provides a comprehensive list of Teleport audit events and their fields."
---
{/* Generated file. Do not edit. */}
{/* To regenerate, navigate to docs/gen-event-reference and run pnpm gen-docs */}

This is an intro paragraph.

## event.a

Event A

Example:

\`\`\`json
{
  "event": "event.a",
  "code": "ABC123"
}
\`\`\`
`;
    const actual = createReferencePage(events, introParagraph);
    expect(actual).toEqual(expected);
  });
});
