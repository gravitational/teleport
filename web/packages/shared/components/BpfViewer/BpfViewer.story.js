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

import BpfViewer, { formatEvents } from './BpfViewer';

export default {
  title: 'Shared/BpfViewer',
};

export const Vim = () => {
  const events = useMockedEvents(
    import('./fixtures/vim').then(vim => vim.default)
  );

  if (events.length === 0) {
    return null;
  }

  return (
    <Box>
      <Viewer events={events} />
    </Box>
  );
};

export const Npm = () => {
  const events = useMockedEvents(
    import('./fixtures/npm').then(vim => vim.default)
  );

  if (events.length === 0) {
    return null;
  }

  return (
    <Box>
      <Viewer events={events} />
    </Box>
  );
};

function useMockedEvents(loader) {
  const [events, setEvents] = React.useState([]);
  loader.then(data => {
    setEvents(data);
  });

  return events.filter(e => e.event === 'session.exec');
}

function Viewer({ events, mode = 'tree' }) {
  const ref = React.useRef();
  React.useEffect(() => {
    const content = formatEvents(events, mode).join('\n');
    ref.current.editor.insert(content);
    ref.current.session.foldAll();
  }, []);

  return <BpfViewer ref={ref} />;
}

const Box = styled.div`
  left: 0;
  right: 0;
  top: 0;
  bottom: 0;
  position: absolute;
  display: flex;
`;
