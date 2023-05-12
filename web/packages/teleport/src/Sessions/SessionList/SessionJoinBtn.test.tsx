/*
Copyright 2023 Gravitational, Inc.

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
import { render, screen, fireEvent } from 'design/utils/testing';

import { ContextProvider } from 'teleport';

import { createTeleportContext } from 'teleport/mocks/contexts';

import { SessionJoinBtn } from './SessionJoinBtn';

test('all participant modes are properly listed and in the correct order', () => {
  const ctx = createTeleportContext();

  render(
    <ContextProvider ctx={ctx}>
      <SessionJoinBtn
        sid={'4b038eda-ddca-5533-9a49-3a34f133b5f4'}
        clusterId={'test-cluster'}
        participantModes={['moderator', 'peer', 'observer']}
        showCTA={false}
      />
    </ContextProvider>
  );

  const joinBtn = screen.queryByText(/Join/i);
  expect(joinBtn).toBeInTheDocument();

  fireEvent.click(joinBtn);

  // Make sure that the join URL is correct.
  const moderatorJoinUrl = screen
    .queryByText('moderator')
    .closest('a')
    .getAttribute('href');

  expect(moderatorJoinUrl).toBe(
    '/web/cluster/test-cluster/console/session/4b038eda-ddca-5533-9a49-3a34f133b5f4?mode=moderator'
  );

  // Make sure that the menu items are in the order of observer -> moderator -> peer.
  const menuItems = screen.queryAllByRole<HTMLAnchorElement>('link');
  expect(menuItems).toHaveLength(3);
  expect(menuItems[0].innerHTML).toBe('observer');
  expect(menuItems[1].innerHTML).toBe('moderator');
  expect(menuItems[2].innerHTML).toBe('peer');
});

test('all possible participant modes are properly listed in the CTA without join links', () => {
  const ctx = createTeleportContext();
  render(
    <ContextProvider ctx={ctx}>
      <SessionJoinBtn
        sid={'4b038eda-ddca-5533-9a49-3a34f133b5f4'}
        clusterId={'test-cluster'}
        participantModes={['moderator', 'peer', 'observer']}
        showCTA={true}
      />
    </ContextProvider>
  );

  const joinBtn = screen.queryByText(/Join/i);
  expect(joinBtn).toBeInTheDocument();

  fireEvent.click(joinBtn);

  // Make sure that no link to join session is available when showCTA = true.
  const menuItems = screen.queryByRole<HTMLAnchorElement>('link');

  expect(menuItems.getAttribute('href')).not.toMatch(/.*console\/session.*/);

  // Make sure the CTAs are rendered
  expect(menuItems).toHaveTextContent(
    'Join Active Sessions with Teleport Enterprise'
  );

  const cta = screen.queryByText(
    'Join Active Sessions with Teleport Enterprise'
  );
  expect(cta).toBeInTheDocument();
});
