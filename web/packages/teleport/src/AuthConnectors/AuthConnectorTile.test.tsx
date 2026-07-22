/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { screen } from '@testing-library/react';

import { fireEvent, render } from 'design/utils/testing';
import { AuthType } from 'shared/services';

import { AuthConnectorTile } from './AuthConnectorTile';
import getSsoIcon from './ssoIcons/getSsoIcon';

test('default, real connector, renders properly', () => {
  render(<AuthConnectorTile {...props} />);

  expect(screen.getByText('Okta')).toBeInTheDocument();
  expect(screen.queryByText('Default')).not.toBeInTheDocument();

  const optionsButton = screen.getByTestId('button');
  fireEvent.click(optionsButton);

  expect(screen.getByText('Set as default')).toBeInTheDocument();
  expect(screen.getByText('Edit')).toBeInTheDocument();
  expect(screen.getByText('Delete')).toBeInTheDocument();
});

test('non-default, real connector, renders properly', () => {
  render(<AuthConnectorTile {...props} isDefault={true} />);

  expect(screen.getByText('Okta')).toBeInTheDocument();
  expect(screen.getByText('Default')).toBeInTheDocument();

  const optionsButton = screen.getByTestId('button');
  fireEvent.click(optionsButton);

  expect(screen.queryByText('Set as default')).not.toBeInTheDocument();
  expect(screen.getByText('Edit')).toBeInTheDocument();
  expect(screen.getByText('Delete')).toBeInTheDocument();
});

// "local" connector for has no edit/delete functionality, only set as default
test('non-default, real connector, with no edit/delete functionality renders properly', () => {
  render(<AuthConnectorTile {...props} onDelete={null} onEdit={null} />);

  expect(screen.getByText('Okta')).toBeInTheDocument();
  expect(screen.queryByText('Default')).not.toBeInTheDocument();

  const optionsButton = screen.getByTestId('button');
  fireEvent.click(optionsButton);

  expect(screen.getByText('Set as default')).toBeInTheDocument();
  expect(screen.queryByText('Edit')).not.toBeInTheDocument();
  expect(screen.queryByText('Delete')).not.toBeInTheDocument();
});

test('default, real connector, with no edit/delete functionality renders properly', () => {
  render(
    <AuthConnectorTile
      {...props}
      isDefault={true}
      onDelete={null}
      onEdit={null}
    />
  );

  expect(screen.getByText('Okta')).toBeInTheDocument();
  expect(screen.getByText('Default')).toBeInTheDocument();

  expect(screen.queryByTestId('button')).not.toBeInTheDocument();
});

test('placeholder renders properly', () => {
  render(<AuthConnectorTile {...props} isPlaceholder={true} />);

  expect(screen.getByText('Okta')).toBeInTheDocument();
  expect(screen.queryByText('Default')).not.toBeInTheDocument();

  expect(screen.getByText('Set Up')).toBeInTheDocument();

  expect(screen.queryByTestId('button')).not.toBeInTheDocument();
});

const props = {
  name: 'Okta',
  id: 'okta-connector',
  kind: 'saml' as AuthType,
  Icon: getSsoIcon('saml', 'okta'),
  isDefault: false,
  isPlaceholder: false,
  onSetup: () => null,
  onEdit: () => null,
  onDelete: () => null,
  onSetAsDefault: () => null,
};
