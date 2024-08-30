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
import { render, screen, fireEvent } from 'design/utils/testing';
import userEvent from '@testing-library/user-event';
import { within } from '@testing-library/react';

import { createTeleportContext } from 'teleport/mocks/contexts';
import { ContextProvider } from 'teleport';
import makeJoinToken from 'teleport/services/joinToken/makeJoinToken';

import { JoinTokens } from './JoinTokens';

describe('JoinTokens', () => {
  test('create dialog opens', async () => {
    render(<Component />);
    await userEvent.click(
      screen.getByRole('button', { name: /create new token/i })
    );

    expect(screen.getByText(/create a new join token/i)).toBeInTheDocument();
  });

  test('edit dialog opens with values', async () => {
    const token = tokens[0];
    render(<Component />);
    const optionButtons = await screen.findAllByText(/options/i);
    await userEvent.click(optionButtons[0]);
    const editButtons = await screen.findAllByText(/view\/edit/i);
    await userEvent.click(editButtons[0]);
    expect(screen.getByText(/edit token/i)).toBeInTheDocument();

    expect(screen.getByDisplayValue(token.id)).toBeInTheDocument();
    expect(
      screen.getByDisplayValue(token.allow[0].aws_account)
    ).toBeInTheDocument();
  });

  test('create form fails if roles arent selected', async () => {
    render(<Component />);
    await userEvent.click(
      screen.getByRole('button', { name: /create new token/i })
    );

    fireEvent.change(screen.getByPlaceholderText('iam-token-name'), {
      target: { value: 'the_token' },
    });

    fireEvent.click(screen.getByRole('button', { name: /create join token/i }));
    expect(
      screen.getByText('At least one role is required')
    ).toBeInTheDocument();
  });

  test('successful create adds token to the table', async () => {
    render(<Component />);
    await userEvent.click(
      screen.getByRole('button', { name: /create new token/i })
    );

    fireEvent.change(screen.getByPlaceholderText('iam-token-name'), {
      target: { value: 'the_token' },
    });

    const inputEl = within(screen.getByTestId('role_select')).getByRole(
      'textbox'
    );
    fireEvent.change(inputEl, { target: { value: 'Node' } });
    fireEvent.focus(inputEl);
    fireEvent.keyDown(inputEl, { key: 'Enter', keyCode: 13 });

    fireEvent.click(screen.getByRole('button', { name: /create join token/i }));
    expect(
      screen.queryByText('At least one role is required')
    ).not.toBeInTheDocument();
    fireEvent.change(screen.getByPlaceholderText('AWS Account ID'), {
      target: { value: '123123123' },
    });

    await userEvent.click(
      screen.getByRole('button', { name: /create join token/i })
    );

    expect(
      screen.queryByText(/create a new join token/i)
    ).not.toBeInTheDocument();
    expect(screen.getByText('the_token')).toBeInTheDocument();
  });

  test('a rule cannot be deleted if it is the only rule', async () => {
    render(<Component />);
    await userEvent.click(
      screen.getByRole('button', { name: /create new token/i })
    );

    const buttons = screen.queryAllByTestId('delete_rule');
    expect(buttons).toHaveLength(0);
  });

  test('a rule can be deleted more than one rule exists', async () => {
    render(<Component />);
    await userEvent.click(
      screen.getByRole('button', { name: /create new token/i })
    );

    fireEvent.click(screen.getByText('Add another AWS Rule'));

    const buttons = screen.queryAllByTestId('delete_rule');
    expect(buttons).toHaveLength(2);
  });
});

const Component = () => {
  const ctx = createTeleportContext();
  jest
    .spyOn(ctx.joinTokenService, 'fetchJoinTokens')
    .mockResolvedValue({ items: tokens.map(makeJoinToken) });

  jest.spyOn(ctx.joinTokenService, 'createJoinToken').mockResolvedValue(
    makeJoinToken({
      id: 'the_token',
      safeName: 'the_token',
      bot_name: '',
      expiry: '3024-07-26T11:52:48.320045Z',
      roles: ['Node'],
      isStatic: false,
      method: 'iam',
      allow: [
        {
          aws_account: '1234444',
          aws_arn: 'asdf',
        },
      ],
      content: 'fake content',
    })
  );

  return (
    <ContextProvider ctx={ctx}>
      <JoinTokens />
    </ContextProvider>
  );
};

const tokens = [
  {
    id: '123123ffff',
    safeName: '123123ffff',
    bot_name: '',
    expiry: '3024-07-26T11:52:48.320045Z',
    roles: ['Node'],
    isStatic: false,
    method: 'iam',
    allow: [
      {
        aws_account: '1234444',
        aws_arn: 'asdf',
      },
    ],
    content: 'fake content',
  },
  {
    id: 'rrrrr',
    safeName: 'rrrrr',
    bot_name: '7777777',
    expiry: '3024-07-26T12:05:48.08241Z',
    roles: ['Bot', 'Node'],
    isStatic: false,
    method: 'iam',
    allow: [
      {
        aws_account: '445555444',
      },
    ],
    content: 'fake content',
  },
];
