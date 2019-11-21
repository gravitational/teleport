/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import styled from 'styled-components';
import { storiesOf } from '@storybook/react';
import { Player } from './Player';
import TtyPlayer from 'teleport/lib/term/ttyPlayer';
import vim from './fixtures/vim';
import troubleshot from './fixtures/troubleshot';
import npm from './fixtures/npm';

storiesOf('TeleportAudit/Player', module)
  .add('vim', () => {
    const tty = new TtyPlayer('url');
    const events = tty._eventProvider._createEvents(vim.ttyEvents);
    tty._eventProvider._fetchEvents = () => Promise.resolve(events);
    tty._eventProvider._fetchContent = () => Promise.resolve(vim.ttyStream);
    return (
      <Box>
        <Player
          sid="4014408407"
          tty={tty}
          bpfEvents={vim.auditEvents.filter(e => e.event === 'session.exec')}
        />
      </Box>
    );
  })
  .add('troubleshot', () => {
    const tty = new TtyPlayer('url');
    const events = tty._eventProvider._createEvents(troubleshot.ttyEvents);
    tty._eventProvider._fetchEvents = () => Promise.resolve(events);
    tty._eventProvider._fetchContent = () =>
      Promise.resolve(troubleshot.ttyStream);

    return (
      <Box>
        <Player
          sid="515511762"
          tty={tty}
          bpfEvents={troubleshot.auditEvents.filter(
            e => e.event === 'session.exec'
          )}
        />
      </Box>
    );
  })
  .add('npm', () => {
    const tty = new TtyPlayer('url');
    const events = tty._eventProvider._createEvents(npm.ttyEvents);
    tty._eventProvider._fetchEvents = () => Promise.resolve(events);
    tty._eventProvider._fetchContent = () => Promise.resolve(npm.ttyStream);

    return (
      <Box>
        <Player sid="3301964015" tty={tty} bpfEvents={npm.auditEvents} />
      </Box>
    );
  });

const Box = styled.div`
  left: 0;
  right: 0;
  top: 0;
  bottom: 0;
  position: absolute;
`;
