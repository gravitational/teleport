/*
Copyright 2021-2022 Gravitational, Inc.

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
import { MemoryRouter } from 'react-router-dom';

import Welcome from './Welcome';
import { CardWelcome } from './CardWelcome';

/**
 *
 * @remarks
 * This component is duplicated in Enterprise for Enterprise onboarding. If you are making edits to this file, check to see if the
 * equivalent change should be applied in Enterprise
 *
 */
export default { title: 'Teleport/Welcome' };

export const WelcomeCustom = () => (
  <CardWelcome
    title="Some Title"
    subTitle="some small subtitle"
    btnText="Button Text"
    onClick={() => null}
  />
);

export const WelcomeInvite = () => (
  <MemoryRouter initialEntries={['/web/invite/1234']}>
    <Welcome />
  </MemoryRouter>
);

export const WelcomeReset = () => (
  <MemoryRouter initialEntries={['/web/reset/1234']}>
    <Welcome />
  </MemoryRouter>
);
