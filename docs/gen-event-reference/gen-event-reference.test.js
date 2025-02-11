import {
  createEventSection,
  createReferencePage,
  eventsWithoutExamples,
  removeUnknowns,
} from './gen-event-reference.js';

describe('eventsWithoutExamples', () => {
  const testCases = [
    {
      description: 'formatters with no fixture',
      events: [
        {
          codeDesc: 'App created',
          code: 'ABC123',
          raw: {
            event: 'app.create',
          },
        },
      ],
      formatters: {
        ABC456: {
          type: 'billing.create_card',
          desc: 'Card created',
        },
      },
      expected: [
        {
          codeDesc: 'Card created',
          code: 'ABC456',
          raw: {
            event: 'billing.create_card',
          },
        },
      ],
    },
  ];

  test.each(testCases)('$description', testCase => {
    expect(eventsWithoutExamples(testCase.events, testCase.formatters)).toEqual(
      testCase.expected
    );
  });
});

describe('removeUnknowns', () => {
  const testCases = [
    {
      description: 'event code not present in the formatters array',
      events: [
        {
          codeDesc: 'Unknown',
          code: 'ABC123',
          raw: {
            event: 'billing.delete_card',
          },
        },
      ],
      formatters: {
        ABC456: {
          type: 'billing.create_card',
          desc: 'Card created',
        },
      },
      expected: [],
    },
  ];

  test.each(testCases)('$description', testCase => {
    expect(removeUnknowns(testCase.events, testCase.formatters)).toEqual(
      testCase.expected
    );
  });
});

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
      description: 'Event with only the raw.event field',
      event: {
        codeDesc: 'Credit Card Deleted',
        code: 'TBL01I',
        raw: {
          event: 'billing.delete_card',
        },
      },
      expected: `## billing.delete_card

Credit Card Deleted

Code: \`TBL01I\`

Event: \`billing.delete_card\`
`,
    },
    {
      description: 'description is a function',
      event: {
        codeDesc: ({ code, event }) => {
          const eventName = 'Port forwarding';

          switch (code) {
            case 'ABC123':
              return `${eventName} Start`;
            case 'DEF123':
              return `${eventName} Stop`;
            case 'GHI123':
              return `${eventName} Failure`;
          }
        },
        code: 'ABC123',
        raw: {
          event: 'port',
          user: 'myuser',
        },
      },
      expected: `## port

Port forwarding Start

Example:

\`\`\`json
{
  "event": "port",
  "user": "myuser"
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
        code: 'GHI123',
        raw: {
          event: 'event.c',
          code: 'GHI123',
        },
      },
      {
        codeDesc: 'Event A',
        code: 'ABC123',
        raw: {
          event: 'event.a',
          code: 'ABC123',
        },
      },
      {
        codeDesc: 'Event B',
        code: 'DEF123',
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
        code: 'ABC123',
        raw: {
          event: 'event.a',
          code: 'ABC123',
        },
      },
      {
        codeDesc: 'Event A failed',
        code: 'ABC456',
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

  test('displays multiple events with only one raw field', () => {
    const events = [
      {
        codeDesc: 'Access Request Reviewed',
        code: 'T5002I',
        raw: { event: 'access_request.review' },
      },
      {
        codeDesc: 'Stable UNIX user created',
        code: 'TSUU001I',
        raw: { event: 'stable_unix_user.create' },
      },
    ];

    const expected = `---
title: "Audit Event Reference"
description: "Provides a comprehensive list of Teleport audit events and their fields."
---
{/* Generated file. Do not edit. */}
{/* To regenerate, navigate to docs/gen-event-reference and run pnpm gen-docs */}

This is an intro paragraph.

## access_request.review

Access Request Reviewed

Code: \`T5002I\`

Event: \`access_request.review\`

## stable_unix_user.create

Stable UNIX user created

Code: \`TSUU001I\`

Event: \`stable_unix_user.create\`
`;
    const actual = createReferencePage(events, introParagraph);
    expect(actual).toEqual(expected);
  });

  test('includes H3 sections for event codes with duplicate types and partial fields', () => {
    const events = [
      {
        codeDesc: 'Event A',
        code: 'ABC123',
        raw: {
          event: 'event.a',
        },
      },
      {
        codeDesc: 'Event A failed',
        code: 'ABC456',
        raw: {
          event: 'event.a',
          code: 'ABC456',
          user: 'myuser',
        },
      },
      {
        codeDesc: 'Event A starting',
        code: 'ABC789',
        raw: {
          event: 'event.a',
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

Code: \`ABC123\`

Event: \`event.a\`

### ABC456

Event A failed

Example:

\`\`\`json
{
  "event": "event.a",
  "code": "ABC456",
  "user": "myuser"
}
\`\`\`

### ABC789

Event A starting

Code: \`ABC789\`

Event: \`event.a\`
`;
    const actual = createReferencePage(events, introParagraph);
    expect(actual).toEqual(expected);
  });
});
