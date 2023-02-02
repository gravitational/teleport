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

import TtyPlayer from 'teleport/lib/term/ttyPlayer';
import TtyPlayerEventProvider from 'teleport/lib/term/ttyPlayerEventProvider';

import { PlayerNext } from './PlayerNext';

export default {
  title: 'Teleport/PlayerNext',
};

export const Vim = () => {
  const mocked = useMockedEvents(
    import('./fixtures/vim').then(vim => vim.default)
  );

  return renderMocked(mocked);
};

export const Npm = () => {
  const mocked = useMockedEvents(
    import('./fixtures/npm').then(vim => vim.default)
  );

  return renderMocked(mocked);
};

function useMockedEvents(loader) {
  const [data, setData] = React.useState(null);
  loader.then(data => {
    setData(data);
  });

  if (!data) {
    return null;
  }

  const eventProvider = new TtyPlayerEventProvider({ url: 'localhost' });
  const tty = new TtyPlayer(eventProvider);
  const events = tty._eventProvider._createEvents(data.ttyEvents);

  eventProvider._fetchEvents = () => Promise.resolve(events);
  eventProvider._fetchContent = () => Promise.resolve(data.ttyStream);

  return {
    events,
    tty,
    auditEvents: data.auditEvents,
  };
}

function renderMocked(mocked) {
  return (
    <Box>
      {mocked && <PlayerNext tty={mocked.tty} bpfEvents={mocked.auditEvents} />}
    </Box>
  );
}

const Box = styled.div`
  left: 0;
  right: 0;
  top: 0;
  bottom: 0;
  position: absolute;
`;
