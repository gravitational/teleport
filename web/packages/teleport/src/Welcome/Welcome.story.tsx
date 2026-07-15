/**
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

import { useEffect, useState } from 'react';
import { MemoryRouter } from 'react-router';

import { WelcomeWrapper } from 'teleport/components/Onboard';
import cfg from 'teleport/config';
import { NewCredentials } from 'teleport/Welcome/NewCredentials';

import { CardWelcome } from './CardWelcome';
import { Welcome } from './Welcome';

export default { title: 'Teleport/Welcome' };

export const WelcomeCustom = () => (
  <WelcomeWrapper>
    <CardWelcome
      title="Some Title"
      subTitle="some small subtitle"
      btnText="Button Text"
      onClick={() => null}
    />
  </WelcomeWrapper>
);

export const WelcomeInvite = () => (
  <MemoryRouter initialEntries={['/web/invite/1234']}>
    <Welcome NewCredentials={NewCredentials} />
  </MemoryRouter>
);

export const WelcomeReset = () => (
  <MemoryRouter initialEntries={['/web/reset/1234']}>
    <Welcome NewCredentials={NewCredentials} />
  </MemoryRouter>
);

export const WelcomeInviteBeams = () => {
  const [key, setKey] = useState(0);

  useEffect(() => {
    const previous = cfg.beamsUi;
    cfg.beamsUi = true;
    setKey(k => k + 1);
    return () => {
      cfg.beamsUi = previous;
    };
  }, []);

  return (
    <MemoryRouter initialEntries={['/web/invite/1234']} key={key}>
      <Welcome NewCredentials={NewCredentials} />
    </MemoryRouter>
  );
};
