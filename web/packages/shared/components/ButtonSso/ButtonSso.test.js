/**
 * Copyright 2020 Gravitational, Inc.
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
import ButtonSso, { TypeEnum, pickSsoIcon } from '.';
import { render } from 'design/utils/testing';

test('renders default type to unknown', () => {
  const { color, type } = pickSsoIcon('unknown');
  const { container, getByTestId, getByText } = render(<ButtonSso />);

  expect(container.firstChild).toHaveStyle({ 'background-color': color });
  expect(getByTestId('icon')).toHaveClass('icon-openid');
  expect(getByText(/unknown sso/i)).toHaveTextContent(type);
});

test.each`
  ssoType               | expectedIcon
  ${TypeEnum.MICROSOFT} | ${'icon-windows'}
  ${TypeEnum.GITHUB}    | ${'icon-github'}
  ${TypeEnum.BITBUCKET} | ${'icon-bitbucket'}
  ${TypeEnum.GOOGLE}    | ${'icon-google-plus'}
`('prop ssoType set to $ssoType is respected', ({ ssoType, expectedIcon }) => {
  const { color, type } = pickSsoIcon(ssoType);
  const { getByTestId, getByText, container } = render(
    <ButtonSso ssoType={type} />
  );

  expect(container.firstChild).toHaveStyle({ 'background-color': color });
  expect(getByTestId('icon')).toHaveClass(expectedIcon);
  expect(getByText(type)).toHaveTextContent(type);
});
