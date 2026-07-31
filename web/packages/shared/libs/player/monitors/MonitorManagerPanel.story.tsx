/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { useEffect, useState } from 'react';

import { Meta } from '@storybook/react-vite';

import { MonitorManagerPanel } from './MonitorManagerPanel';
import {
  createMockMonitorSession,
  createMockTopology,
  mockDisplay,
} from './mockMonitorSession';
import type { ModelMonitor } from './monitorModel';
import type { MonitorSession } from './monitorSession';
import type { ScreenTopology } from './useScreenTopology';

const meta: Meta = {
  title: 'Shared/Player/MonitorManagerPanel',
};
export default meta;

function Harness({
  session,
  topology,
}: {
  session: MonitorSession;
  topology: ScreenTopology;
}) {
  const [state, setState] = useState(session.getState());
  useEffect(() => session.subscribe(setState), [session]);
  return (
    <MonitorManagerPanel
      open
      onClose={() => undefined}
      session={session}
      state={state}
      topology={topology}
    />
  );
}

const mainMonitor: ModelMonitor = {
  id: 0,
  role: 'main',
  cssWidth: 1920,
  cssHeight: 1080,
  isPrimary: true,
  physical: { left: 0, top: 0 },
  status: 'active',
};

const twoDisplays = [
  mockDisplay({ id: 'builtin', label: 'Built-in Retina', left: 0, top: 0, isPrimary: true, isInternal: true, devicePixelRatio: 2 }),
  mockDisplay({ id: 'dell', label: 'DELL U2720Q', left: 1920, top: 0, width: 2560, height: 1440 }),
];

export const SingleDisplay = () => {
  const [session] = useState(() => createMockMonitorSession([mainMonitor]));
  const [topology] = useState(() =>
    createMockTopology([twoDisplays[0]], 'granted')
  );
  return <Harness session={session} topology={topology} />;
};

export const TwoDisplays = () => {
  const [session] = useState(() =>
    createMockMonitorSession([
      mainMonitor,
      {
        id: 1,
        role: 'popup',
        cssWidth: 2560,
        cssHeight: 1440,
        isPrimary: false,
        physical: { left: 1920, top: 0 },
        displayId: 'dell',
        status: 'active',
      },
    ])
  );
  const [topology] = useState(() => createMockTopology(twoDisplays, 'granted'));
  return <Harness session={session} topology={topology} />;
};

export const PermissionPrompt = () => {
  const [session] = useState(() => createMockMonitorSession([mainMonitor]));
  const [topology] = useState(() => createMockTopology(twoDisplays, 'prompt'));
  return <Harness session={session} topology={topology} />;
};

export const Unsupported = () => {
  const [session] = useState(() => createMockMonitorSession([mainMonitor]));
  const [topology] = useState(() =>
    createMockTopology(twoDisplays, 'unsupported')
  );
  return <Harness session={session} topology={topology} />;
};
