// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

import { useState } from 'react';
import styled from 'styled-components';

import Flex from 'design/Flex/Flex';
import { SlideTabs } from 'design/SlideTabs';
import { Markdown } from 'shared/components/Markdown/Markdown';
import { parse } from 'shared/utils/semVer';

import { FeatureBox } from 'teleport/components/Layout/Layout';
import { useNoMinWidth } from 'teleport/Main';
import useTeleport from 'teleport/useTeleport';

export function BeamsQuickstart() {
  const [activeScenario, setActiveScenario] = useState<Scenario>(Scenario.run);

  const { storeUser } = useTeleport();

  useNoMinWidth();

  const infoContent = interpolateContent(
    introContent,
    storeUser.getClusterPublicUrl(),
    storeUser.getClusterAuthVersion(),
    storeUser.getUsername()
  );

  const scenario = makeContent(
    activeScenario,
    storeUser.getClusterPublicUrl(),
    storeUser.getClusterAuthVersion(),
    storeUser.getUsername()
  );

  return (
    <FeatureBox>
      <Flex
        flexDirection={'column'}
        maxWidth={800}
        marginX={'auto'}
        width="100%"
        py={5}
        gap={3}
      >
        {infoContent.map(c => (
          <Wrapper key={c}>
            <Markdown text={c} enableLinks />
          </Wrapper>
        ))}

        <Wrapper>
          <Markdown
            text={
              '### Step 3: Check out these scenarios to help you explore Beams:'
            }
          />
          <SlideTabs
            tabs={scenarios}
            activeIndex={scenarios.findIndex(c => c.key === activeScenario)}
            onChange={index => setActiveScenario(scenarios[index]!.key)}
          />
        </Wrapper>

        {scenario.map(c => (
          <Wrapper key={c}>
            <Markdown text={c} enableLinks />
          </Wrapper>
        ))}
      </Flex>
    </FeatureBox>
  );
}

function makeContent(
  activeScenario: Scenario,
  clusterPublicUrl: string,
  clusterVersion: string,
  username: string
) {
  switch (activeScenario) {
    case Scenario.run:
      return interpolateContent(
        runContent,
        clusterPublicUrl,
        clusterVersion,
        username
      );
    case Scenario.vibe:
      return interpolateContent(
        vibeContent,
        clusterPublicUrl,
        clusterVersion,
        username
      );
    case Scenario.examples:
      return interpolateContent(
        examplesContent,
        clusterPublicUrl,
        clusterVersion,
        username
      );
  }
}

function interpolateContent(
  content: string[],
  clusterPublicUrl: string,
  clusterVersion: string,
  username: string
) {
  const semver = parse(clusterVersion);
  // Strip the build and prerelease parts
  const version = `${semver?.major}.${semver?.minor}.${semver?.patch}`;

  const subs: Record<string, string> = {
    ':cluster_public_url': clusterPublicUrl,
    ':cluster_version': version,
    ':username': username,
  };

  return content.map(c =>
    c.replace(
      /:cluster_public_url|:cluster_version|:username/g,
      match => subs[match]
    )
  );
}

enum Scenario {
  run,
  vibe,
  examples,
}

const scenarios = [
  {
    title: 'Run a command',
    key: Scenario.run,
  },
  {
    title: 'Vibe-coding session',
    key: Scenario.vibe,
  },
  {
    title: 'Browse the examples',
    key: Scenario.examples,
  },
];

const introContent = [
  `
# Welcome to Beams

**Let's get you set up!**

### Step 1: Check you have the Teleport client tools installed

\`\`\`
tsh version
\`\`\`

💡 **Note:** Install \`v:cluster_version\` or newer for an optimal experience.

<details>
<summary>Need to install the Teleport client tools?</summary>

Run the install command for your OS:

**macOS**

Download the signed macOS .pkg installer for Teleport, which includes \`tsh\`. In Finder double-click the pkg file to begin installation:

\`\`\`
curl -O https://cdn.teleport.dev/teleport-:cluster_version.pkg
\`\`\`

**Windows**

Unzip the archive and move tsh.exe to your %PATH%

\`\`\`
curl.exe -O https://cdn.teleport.dev/teleport-v:cluster_version-windows-amd64-bin.zip
\`\`\`

**Linux**

\`\`\`
curl -O https://cdn.teleport.dev/teleport-v:cluster_version-linux-amd64-bin.tar.gz
\`\`\`

\`\`\`
tar -xzf teleport-v:cluster_version-linux-amd64-bin.tar.gz && cd teleport
\`\`\`

\`\`\`
sudo ./install
\`\`\`

💡 For more information read the docs page: [Using the \`tsh\` Command Line Tool](https://goteleport.com/docs/connect-your-client/teleport-clients/tsh/).

</details>
`,
  `
### Step 2: Authenticate with the cluster

Choose a command below depending on how you login to your cluster.

**Password + MFA**

\`\`\`
tsh login \\
    --proxy=:cluster_public_url \\
    --user=:username
\`\`\`

**Passwordless**

Add the \`--auth=passwordless\` flag

\`\`\`
tsh login \\
    --proxy=:cluster_public_url \\
    --user=:username \\
    --auth=passwordless
\`\`\`
`,
];

const runContent = [
  `## Run a command

In this guide we’ll execute a command in a Beam instance and inspect the output.

### Step 1: Create a Beam

\`\`\`
tsh beams add --no-console
\`\`\`

‼️ **Take note of the Beam's identifier, you'll use this in the following steps.**

💡 \`--no-console\` disables auto-opening an interactive shell on the Beam.
`,
  `
### Step 2: Run a command and view its output

\`\`\`
tsh beams exec <beam-id> 'echo "Describe the environment and its capabilities" | codex e --yolo'
\`\`\`

💡 This command submits a simple prompt to codex and disables approvals and sandboxing.

<details>
<summary>Want to run a task in the background?</summary>
#### Use a \`tmux\` session

\`\`\`
tsh beams exec <beam-id> 'tmux new -d -s web-server -- python3 -m http.server 8080'
\`\`\`

💡 This command creates a new, detached \`tmux\` session called "web-server" in which the command is executed.

#### Later, attach to the session to view output or end the running task

\`\`\`
tsh beams ssh <beam-id>
\`\`\`

From the beam's shell

\`\`\`
tmux attach
\`\`\`
</details>
`,
  `
### Step 3: When you're done, delete the Beam

\`\`\`
tsh beams rm <beam-id>
\`\`\`

♻️ Beams are automatically garbage collected after 24hrs.
`,
  `
## That's it! 🙌
- You've used a Beam to run a command in an isolated environment.
- Your roles and permissions were delegated to the Beam's own identity while it was running.
- Certificates, filesystem changes and internal state were erased when you deleted the Beam.

Read the [docs and reference](https://goteleport.com/docs/ver/19.x/beams/) for more details.
`,
];

const vibeContent = [
  `## Vibe-code

In this guide we’ll co-create a basic web app using Claude Code in an isolated environment, then access the app from your local browser.

### Step 1: Create a Beam

\`\`\`
tsh beams add --no-console
\`\`\`

‼️ **Take note of the Beam's identifier, you'll use this in the following steps.**
`,
  `
### Step 2: Setup secure access to the Beam from your browser

\`\`\`
tsh beams publish <beam-id>
\`\`\`

‼️ **Take note of the Beam's web URL, you'll use this to access the app later.**

💡 This command exposes port 8080 as an HTTP app. It's accessible to authenticated Teleport users only.
`,
  `
### Step 3: Co-create a basic web application

\`\`\`
tsh beams ssh <beam-id>
\`\`\`

4. Co-create a basic web application;

\`\`\`
claude
\`\`\`

✨ There's no need to install, configure or authenticate Claude Code, it works out of the box.

An example prompt

\`\`\`
Create a simple calculator app. Use Vite, TypeScript and React. Add an NPM script to start the app in dev mode on port 8080 with network access.
\`\`\`

Start the development server if Claude hasn't done it already

\`\`\`
npm run dev
\`\`\`
`,
  `
### Step 4: View the app in your browser

To find the app's URL, use the beams list as a reference

\`\`\`
tsh beams ls
\`\`\`
`,
  `
### Step 5: To end your vibe-coding session, delete the Beam

\`\`\`
tsh beams rm <beam-id>
\`\`\`

♻️ Beams are automatically garbage collected after 24hrs.

<details>
<summary>Want to keep the generated code?</summary>
Copy the app folder from the beam to your local filesystem;

\`\`\`
tsh beams scp --recursive <beam-id>:/home/beams/calc-app ./calc-app
\`\`\`
</details>
`,
  `
## That's it! 🙌
- You've used a Beam as an interactive environment.
- Claude Code was used to generate a basic application which was served over port 8080 and accessible from your local browser.
- You optionally copied files from the Beam to your local filesystem.
- Your roles and permissions were delegated to the Beam's own identity while it was running.
- Certificates, filesystem changes and internal state were erased when you deleted the Beam.

Read the [docs and reference](https://goteleport.com/docs/ver/19.x/beams/) for more details.
`,
];

const examplesContent = [
  `## Browse the examples

In this guide we’ll explore the examples included with a beam.

### Step 1: Create a Beam

\`\`\`
tsh beams add
\`\`\`

‼️ **Take note of the Beam's identifier, you'll use this in the following steps.**
`,
  `
### Step 2: Browse the examples directory

\`\`\`
ls -l examples/
\`\`\`
`,
  `
### Step 3: Run an example

\`\`\`
node examples/anthropic_hello.js
\`\`\`
`,
  `
### Step 4: When you're done, delete the Beam

\`\`\`
tsh beams rm <beam-id>
\`\`\`

♻️ Beams are automatically garbage collected after 24hrs.
`,
  `
## That's it! 🙌
- You browsed the examples included in a beam and ran one.
- Your roles and permissions were delegated to the Beam's own identity while it was running.
- Certificates, filesystem changes and internal state were erased when you deleted the Beam.

Read the [docs and reference](https://goteleport.com/docs/ver/19.x/beams/) for more details.
`,
];

const Wrapper = styled.div`
  padding: ${({ theme }) => theme.space[6]}px;
  border: 1px solid ${({ theme }) => theme.colors.interactive.tonal.neutral[2]};
  border-radius: ${({ theme }) => theme.radii[3]}px;
  background-color: ${({ theme }) => theme.colors.levels.surface};
`;
