import { ReactNode } from 'react';
import { Link } from 'react-router';
import styled from 'styled-components';

import {
  ArrowSquareIn,
  Beams,
  BookOpenText,
  Package,
  Question,
  Terminal,
  Trash,
} from 'design/Icon';
import { Platform } from 'design/platform';
import { P2 } from 'design/Text';
import { parse } from 'shared/utils/semVer';

import cfg from 'teleport/config';
import { PasswordState } from 'teleport/services/user';
import StoreUserContext from 'teleport/stores/storeUserContext';

import { CodeBlock, InlineCode } from './components/CodeBlock';
import { CardDef, SectionDef } from './types';

export type AuthMethod = 'mfa' | 'passwordless' | 'sso';

export function pickDefaultAuth(
  storeUser: StoreUserContext
): AuthMethod | undefined {
  if (storeUser.isSso()) {
    return 'sso';
  }
  if (storeUser.getPasswordState() === PasswordState.PASSWORD_STATE_UNSET) {
    return 'passwordless';
  }
  if (storeUser.state.authType === 'passwordless') {
    return 'passwordless';
  }
  return 'mfa';
}

const DOCS_URL = 'https://goteleport.com/docs/beams/';

const VIBE_PROMPT =
  'Create a new directory called calc-app and inside it build a simple ' +
  'calculator app. Use Vite, TypeScript and React. Add an NPM script to ' +
  'start the app in dev mode on port 8080 with network access.';

function installCommand(platform: Platform, version: string): string {
  if (platform === Platform.Windows) {
    return [
      `curl.exe -O https://cdn.teleport.dev/teleport-v${version}-windows-amd64-bin.zip`,
      '# Unzip the archive and move tsh.exe to your %PATH%',
    ].join('\n');
  }
  if (platform === Platform.Linux) {
    return [
      `curl -O https://cdn.teleport.dev/teleport-v${version}-linux-amd64-bin.tar.gz`,
      `tar -xzf teleport-v${version}-linux-amd64-bin.tar.gz && cd teleport`,
      'sudo ./install',
    ].join('\n');
  }
  return [
    `curl -O https://cdn.teleport.dev/teleport-tools-${version}.pkg`,
    `# Double click the package to begin installation.`,
  ].join('\n');
}

const PLATFORM_LABEL: Record<Platform, string> = {
  [Platform.macOS]: 'macOS',
  [Platform.Linux]: 'Linux',
  [Platform.Windows]: 'Windows',
};

// Short bits of copy reused across several cards.
const ROLES_DELEGATED =
  'Your roles and permissions get delegated to the Beam’s own identity as it runs.';

const BEAM_ID_HINT: ReactNode = (
  <>
    The returned value is your <InlineCode>Beam ID</InlineCode>. You’ll need
    this for the next step.
  </>
);

// The cleanup card is the same in all three tutorials.
const cleanupCard: CardDef = {
  id: 'cleanup',
  icon: Trash,
  steps: [
    {
      eyebrow: 'Clean up',
      title: 'When you’re ready, delete your Beam.',
      blocks: [{ kind: 'code', cmd: 'tsh beams rm <beam-id>' }],
    },
  ],
  trailing: [
    {
      kind: 'text',
      body: 'If not deleted, Beams expire and are garbage collected in 24 hours. The deletion or expiration of a Beam erases its certificates, file system changes, and internal states.',
    },
  ],
};

function displayVersion(raw: string): string {
  const semver = parse(raw);
  return semver ? `${semver.major}.${semver.minor}.${semver.patch}` : raw;
}

export function buildSections(args: {
  clusterPublicUrl: string;
  clusterVersion: string;
  username: string;
  defaultAuth?: AuthMethod;
  platform: Platform;
}): SectionDef[] {
  const { clusterPublicUrl, clusterVersion, username, defaultAuth, platform } =
    args;
  const proxyVersion = displayVersion(clusterVersion);
  const loginBase = `tsh login --proxy=${clusterPublicUrl} --user=${username}`;
  return [
    {
      id: 'welcome',
      group: 'Client Installation',
      title: 'Check your client setup',
      cards: [
        {
          icon: Package,
          steps: [
            {
              eyebrow: 'Step 1',
              title:
                'Check that you’ve installed the latest version of the Teleport client.',
              blocks: [{ kind: 'code', cmd: 'tsh version' }],
            },
          ],
          trailing: [
            {
              kind: 'reveal',
              hint: (
                <>
                  We detected you’re on{' '}
                  <strong>{PLATFORM_LABEL[platform]}</strong>. If{' '}
                  <InlineCode>tsh</InlineCode> isn’t installed yet, run our
                  install command to get set up.
                </>
              ),
              cmd: installCommand(platform, proxyVersion),
            },
          ],
        },
        {
          icon: Terminal,
          steps: [
            {
              eyebrow: 'Step 2',
              title:
                'Select your authentication setup. Run the command to log into your cluster in your terminal.',
              blocks: [
                {
                  kind: 'radios',
                  name: 'beams-quickstart-auth',
                  defaultValue: defaultAuth,
                  options: [
                    {
                      value: 'mfa',
                      label: 'I have a password & MFA set up for my account.',
                      cmd: loginBase,
                    },
                    {
                      value: 'passwordless',
                      label:
                        'I only have passwordless (passkeys) set up for my account.',
                      cmd: `tsh login --proxy=${clusterPublicUrl} --auth=passwordless`,
                    },
                    {
                      value: 'sso',
                      label: 'I sign in with SSO (e.g. Okta, GitHub, SAML).',
                      cmd: `tsh login --proxy=${clusterPublicUrl}`,
                    },
                  ],
                },
              ],
            },
          ],
          trailing: [
            {
              kind: 'text',
              body: defaultAuth ? (
                <>
                  We’ve pre-selected the option that matches your current login.
                  Change it if you’d like to use a different method, or see your
                  authentication setup in{' '}
                  <AccountLink to={cfg.routes.account}>
                    Account Settings.
                  </AccountLink>
                </>
              ) : (
                <>
                  If you don’t remember, check out your authentication setup in{' '}
                  <AccountLink to={cfg.routes.account}>
                    Account Settings.
                  </AccountLink>
                </>
              ),
            },
          ],
        },
        {
          icon: BookOpenText,
          steps: [
            {
              eyebrow: 'Step 3',
              title: 'Now that you’re in, take a look around.',
              blocks: [
                {
                  kind: 'text',
                  body: 'If you need guidance, scroll down for our tutorials.',
                },
              ],
            },
          ],
        },
      ],
    },
    {
      id: 'tutorial-run',
      group: 'Tutorials',
      title: 'Run commands in the background',
      cards: [
        {
          icon: Beams,
          steps: [
            {
              eyebrow: 'Step 1',
              title: 'Create a Beam.',
              blocks: [
                { kind: 'text', body: ROLES_DELEGATED },
                { kind: 'code', cmd: 'tsh beams add --no-console' },
              ],
            },
          ],
          trailing: [
            {
              kind: 'tips',
              items: [
                {
                  name: 'Tip 1',
                  body: (
                    <>
                      <InlineCode>tsh beam</InlineCode> works interchangeably
                      with <InlineCode>tsh beams</InlineCode>.
                    </>
                  ),
                },
                {
                  name: 'Tip 2',
                  body: (
                    <>
                      <InlineCode>--no-console</InlineCode> creates the beam
                      without dropping you into an interactive session.
                    </>
                  ),
                },
              ],
            },
          ],
        },
        {
          icon: Terminal,
          steps: [
            {
              eyebrow: 'Step 2',
              title: 'Run a command and view its output.',
              blocks: [
                {
                  kind: 'code',
                  cmd: `tsh beams exec <beam-id> 'echo "Describe the environment and its configuration" | codex e --yolo'`,
                },
              ],
            },
          ],
          trailing: [
            {
              kind: 'text',
              body: 'This command submits a simple prompt to Codex in YOLO mode — which is safe to do in your Beam.',
            },
          ],
        },
        {
          icon: Terminal,
          steps: [
            {
              eyebrow: 'Step 3',
              title: 'Run a task in the background using tmux.',
              blocks: [
                {
                  kind: 'code',
                  cmd: `tsh beams exec <beam-id> 'tmux new -d -s web-server -- python3 -m http.server 8080'`,
                },
              ],
            },
          ],
          trailing: [
            {
              kind: 'text',
              body: (
                <>
                  This creates a new, detached <InlineCode>tmux</InlineCode>{' '}
                  session called “web-server” in which the command is executed.
                </>
              ),
            },
          ],
        },
        {
          icon: Terminal,
          steps: [
            {
              eyebrow: 'Step 4 (optional)',
              title:
                'To view output or end the task, open a shell in your Beam to attach to the session.',
              blocks: [
                { kind: 'text', body: 'Open a shell in your Beam:' },
                { kind: 'code', cmd: 'tsh beams console <beam-id>' },
                {
                  kind: 'text',
                  body: (
                    <>
                      From within your Beam, attach to your running{' '}
                      <InlineCode>tmux</InlineCode> session:
                    </>
                  ),
                },
                { kind: 'code', cmd: 'tmux attach' },
              ],
            },
          ],
        },
        cleanupCard,
      ],
    },
    {
      id: 'tutorial-vibe',
      group: 'Tutorials',
      title: 'Vibe code & publish an app',
      cards: [
        {
          icon: Beams,
          steps: [
            {
              eyebrow: 'Step 1',
              title: 'Create a Beam and ssh into it in one command.',
              blocks: [
                { kind: 'text', body: ROLES_DELEGATED },
                { kind: 'code', cmd: 'tsh beams add' },
              ],
            },
          ],
          trailing: [{ kind: 'text', body: BEAM_ID_HINT }],
        },
        {
          icon: Terminal,
          steps: [
            {
              eyebrow: 'Step 2',
              title: 'Publish an app in your Beam.',
              blocks: [{ kind: 'code', cmd: 'tsh beams publish <beam-id>' }],
            },
            {
              blocks: [
                {
                  kind: 'text',
                  body: 'You’ll see an http app link in your output. This command exposes port 8080 as an HTTP app that you can use to securely access your Beam from your browser.',
                },
                {
                  kind: 'tips',
                  items: [
                    {
                      name: 'Tip',
                      body: (
                        <>
                          <InlineCode>tsh beams</InlineCode> commands also work
                          from within a beam.
                        </>
                      ),
                    },
                  ],
                },
              ],
            },
          ],
        },
        {
          icon: Terminal,
          steps: [
            {
              eyebrow: 'Step 3',
              title: 'Create a calculator web app.',
              blocks: [
                { kind: 'text', body: 'Start Claude Code:' },
                { kind: 'code', cmd: 'claude' },
                {
                  kind: 'text',
                  body: 'Use an example prompt or write your own:',
                },
                { kind: 'code', cmd: VIBE_PROMPT, prompt: '' },
                {
                  kind: 'text',
                  body: 'Start the development server if Claude hasn’t done so already:',
                },
                { kind: 'code', cmd: 'cd calc-app && npm run dev' },
              ],
            },
          ],
          trailing: [
            {
              kind: 'tips',
              items: [
                {
                  name: 'Tip',
                  body: 'Your Beam comes with Claude Code and Codex, with no need to install, configure, or authenticate as you work in your terminal. Use Teleport’s inference provider as you begin (subject to rate limits). Connecting your own inference providers will be supported later this summer.',
                },
              ],
            },
          ],
        },
        {
          icon: Terminal,
          steps: [
            {
              eyebrow: 'Step 4',
              title: 'View the app in your browser.',
              blocks: [
                {
                  kind: 'text',
                  body: (
                    <>
                      Run <InlineCode>tsh beams ls</InlineCode> to find the URL
                      for your Beam, then open it in your browser to view the
                      app.
                    </>
                  ),
                },
                { kind: 'code', cmd: 'tsh beams ls' },
              ],
            },
          ],
        },
        {
          icon: Terminal,
          steps: [
            {
              eyebrow: 'Step 5 (optional)',
              title:
                'Copy the app folder from the Beam to your local filesystem.',
              blocks: [
                {
                  kind: 'code',
                  cmd: 'tsh beams cp --recursive <beam-id>:/home/beams/calc-app ./calc-app',
                },
              ],
            },
          ],
        },
        cleanupCard,
      ],
    },
    {
      id: 'tutorial-examples',
      group: 'Tutorials',
      title: 'Find more examples in the CLI',
      cards: [
        {
          icon: Beams,
          steps: [
            {
              eyebrow: 'Step 1',
              title: 'Create a Beam and ssh into it in one command.',
              blocks: [
                { kind: 'text', body: ROLES_DELEGATED },
                { kind: 'code', cmd: 'tsh beams add' },
              ],
            },
          ],
          trailing: [{ kind: 'text', body: BEAM_ID_HINT }],
        },
        {
          icon: Terminal,
          steps: [
            {
              eyebrow: 'Step 2',
              title: 'In your Beam, explore the examples directory.',
              blocks: [{ kind: 'code', cmd: 'ls -l examples/' }],
            },
          ],
        },
        {
          icon: Terminal,
          steps: [
            {
              eyebrow: 'Step 3',
              title: 'Pick an example and give it a run.',
              blocks: [
                {
                  kind: 'text',
                  body: 'For example, you can run this example found in the examples directory:',
                },
                { kind: 'code', cmd: 'node examples/anthropic_hello.js' },
              ],
            },
          ],
        },
        cleanupCard,
      ],
    },
    {
      id: 'resources',
      group: 'Resources',
      title: 'Docs & FAQs',
      cards: [
        {
          icon: BookOpenText,
          steps: [
            {
              eyebrow: 'Docs',
              title: 'Visit our docs for more information.',
              blocks: [
                {
                  kind: 'link',
                  label: 'Visit Beams Documentation',
                  href: DOCS_URL,
                  icon: ArrowSquareIn,
                },
              ],
            },
          ],
        },
        {
          icon: Question,
          steps: [{ eyebrow: 'FAQs', title: 'Frequently Asked Questions' }],
          trailing: [{ kind: 'faqs', items: FAQS }],
        },
      ],
    },
  ];
}

const FAQS: Array<{ q: string; a: ReactNode }> = [
  {
    q: 'How do I find my Beam’s ID?',
    a: (
      <>
        <P2>
          The ID is printed when you run <InlineCode>tsh beams add</InlineCode>.
          You can also list all your Beams at any time:
        </P2>
        <CodeBlock command="tsh beams ls" />
      </>
    ),
  },
  {
    q: 'How long does a Beam live?',
    a: (
      <P2>
        Beams are ephemeral. They’re automatically garbage-collected after 24
        hours, or whenever you run{' '}
        <InlineCode>{'tsh beams rm <beam-id>'}</InlineCode>. Certificates,
        filesystem changes, and internal state are erased on delete.
      </P2>
    ),
  },
  {
    q: 'Can I access internal services from a Beam?',
    a: (
      <P2>
        Yes. Your Teleport identity is forwarded automatically, so the Beam can
        reach databases, Kubernetes clusters, and other internal services using
        your roles and permissions — no additional keys or secrets required.
      </P2>
    ),
  },
  {
    q: 'What’s preinstalled in a Beam?',
    a: (
      <P2>
        The Teleport client (<InlineCode>tsh</InlineCode>), common developer
        tools (<InlineCode>git</InlineCode>, <InlineCode>curl</InlineCode>,{' '}
        <InlineCode>python3</InlineCode>, <InlineCode>node</InlineCode>,{' '}
        <InlineCode>npm</InlineCode>), and AI agents (
        <InlineCode>claude</InlineCode>, <InlineCode>codex</InlineCode>) — all
        ready to use without additional auth.
      </P2>
    ),
  },
  {
    q: 'Can I copy files between my machine and a Beam?',
    a: (
      <>
        <P2>
          Yes, <InlineCode>tsh beams cp</InlineCode> works in both directions.
          To pull generated code back to your machine before deleting the Beam:
        </P2>
        <CodeBlock command="tsh beams cp --recursive <beam-id>:/home/beams/app ./app" />
      </>
    ),
  },
  {
    q: 'How do I reconnect to a Beam?',
    a: (
      <>
        <P2>Use the CLI to open a shell in a running Beam by its ID:</P2>
        <CodeBlock command="tsh beams console <beam-id>" />
      </>
    ),
  },
  {
    q: 'What if I want to keep my work?',
    a: (
      <P2>
        Everything inside a Beam is wiped on delete or after the 24-hour TTL. If
        you want to keep generated code, copy it out with{' '}
        <InlineCode>tsh beams cp</InlineCode> before running{' '}
        <InlineCode>tsh beams rm</InlineCode>.
      </P2>
    ),
  },
];

const AccountLink = styled(Link)`
  color: ${p => p.theme.colors.brand};
  font-weight: ${p => p.theme.fontWeights.bold};
  text-decoration: none;
  &:hover {
    text-decoration: underline;
  }
`;
