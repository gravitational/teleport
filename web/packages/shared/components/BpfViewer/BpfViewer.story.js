/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
