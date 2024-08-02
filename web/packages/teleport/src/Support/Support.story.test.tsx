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

import React from 'react';
import { MemoryRouter } from 'react-router';

import { render } from 'design/utils/testing';

import cfg from 'teleport/config';

import {
  SupportOSS,
  SupportOSSWithCTA,
  SupportCloud,
  SupportEnterprise,
  SupportEnterpriseWithCTA,
} from './Support.story';

test('support OSS', () => {
  const { container } = render(
    <MemoryRouter>
      <SupportOSS />
    </MemoryRouter>
  );
  expect(container.firstChild).toMatchSnapshot();
});

test('support OSSWithCTA', () => {
  const { container } = render(
    <MemoryRouter>
      <SupportOSSWithCTA />
    </MemoryRouter>
  );
  expect(container.firstChild).toMatchSnapshot();
});

test('support Cloud', () => {
  const { container } = render(
    <MemoryRouter>
      <SupportCloud />
    </MemoryRouter>
  );
  expect(container.firstChild).toMatchSnapshot();
});

test('support Enterprise', () => {
  const { container } = render(
    <MemoryRouter>
      <SupportEnterprise />
    </MemoryRouter>
  );
  expect(container.firstChild).toMatchSnapshot();
});

test('support Enterprise with CTA', () => {
  cfg.isEnterprise = true;
  const { container } = render(
    <MemoryRouter>
      <SupportEnterpriseWithCTA />
    </MemoryRouter>
  );
  expect(container.firstChild).toMatchSnapshot();
});
