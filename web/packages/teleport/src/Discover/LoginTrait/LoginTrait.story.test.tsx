/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { render } from 'design/utils/testing';

import * as stories from './LoginTrait.story';

test('no logins and perms', () => {
  const { container } = render(<stories.NoLoginsAndPerm />);
  expect(container.firstChild).toMatchSnapshot();
});

test('no logins, perms, and is a SSO user', () => {
  const { container } = render(<stories.NoLoginsAndPermAndSsoUser />);
  expect(container.firstChild).toMatchSnapshot();
});

test('logins with no perms', () => {
  const { container } = render(<stories.NoPerm />);
  expect(container.firstChild).toMatchSnapshot();
});

test('logins and is  SSO user', () => {
  const { container } = render(<stories.SsoUser />);
  expect(container.firstChild).toMatchSnapshot();
});

test('static and dynamic logins with perms`', () => {
  const { container } = render(<stories.StaticAndDynamic />);
  expect(container.firstChild).toMatchSnapshot();
});

test('dynamic only logins with perms', () => {
  const { container } = render(<stories.DynamicOnly />);
  expect(container.firstChild).toMatchSnapshot();
});

test('static only logins with perms', () => {
  const { container } = render(<stories.StaticOnly />);
  expect(container.firstChild).toMatchSnapshot();
});

test('no logins with perms', () => {
  const { container } = render(<stories.NoLogins />);
  expect(container.firstChild).toMatchSnapshot();
});

test('failed', () => {
  const { container } = render(<stories.Failed />);
  expect(container.firstChild).toMatchSnapshot();
});
