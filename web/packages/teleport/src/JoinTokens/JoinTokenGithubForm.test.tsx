/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import userEvent from '@testing-library/user-event';
import { PropsWithChildren } from 'react';

import { fireEvent, render, screen, within } from 'design/utils/testing';
import Validation, { useValidation } from 'shared/components/Validation';

import { ThemeProvider } from 'teleport/ThemeProvider';

import { JoinTokenGithubForm } from './JoinTokenGithubForm';
import { NewJoinTokenState } from './UpsertJoinTokenDialog';

const populateRuleFieldTest =
  (
    field: keyof NewJoinTokenState['github']['rules'][number],
    placeholer: string,
    value: string
  ) =>
  async () => {
    const state = baseState();
    const onUpdate = jest.fn();

    render(
      <JoinTokenGithubForm
        tokenState={state}
        onUpdateState={onUpdate}
        readonly={false}
      />,
      { wrapper: Wrapper }
    );

    fireEvent.change(screen.getByPlaceholderText(placeholer), {
      target: { value },
    });

    expect(onUpdate).toHaveBeenCalledTimes(1);
    expect(onUpdate).toHaveBeenLastCalledWith(
      baseState({
        rules: [
          {
            ...state.github.rules[0],
            [field]: value,
          },
        ],
      })
    );
  };

const populateFieldTest =
  ({
    field,
    placeholer,
    value,
  }: {
    field: keyof NewJoinTokenState['github'];
    placeholer: string;
    value: string;
  }) =>
  async () => {
    const state = baseState();
    const onUpdate = jest.fn();

    render(
      <JoinTokenGithubForm
        tokenState={state}
        onUpdateState={onUpdate}
        readonly={false}
      />,
      { wrapper: Wrapper }
    );

    fireEvent.change(screen.getByPlaceholderText(placeholer), {
      target: { value },
    });

    expect(onUpdate).toHaveBeenCalledTimes(1);
    expect(onUpdate).toHaveBeenLastCalledWith(
      baseState({
        [field]: value,
      })
    );
  };

describe('GithubJoinTokenForm', () => {
  it('a rule can be added', async () => {
    const state = baseState();
    const onUpdate = jest.fn();

    render(
      <JoinTokenGithubForm
        tokenState={state}
        onUpdateState={onUpdate}
        readonly={false}
      />,
      { wrapper: Wrapper }
    );

    await userEvent.click(
      screen.getByRole('button', { name: /Add another GitHub rule/i })
    );

    expect(onUpdate).toHaveBeenCalledTimes(1);
    expect(onUpdate).toHaveBeenLastCalledWith(
      baseState({
        rules: [
          ...state.github.rules,
          {
            ref_type: 'any',
          },
        ],
      })
    );
  });

  it('delete button is hidden when only one rule exists', async () => {
    const state = baseState();

    render(
      <JoinTokenGithubForm
        tokenState={state}
        onUpdateState={jest.fn()}
        readonly={false}
      />,
      { wrapper: Wrapper }
    );

    expect(screen.queryByTestId('delete_rule')).not.toBeInTheDocument();
  });

  it('delete button is visible when more than one rule exists', async () => {
    const state = baseState({
      rules: [{}, {}],
    });

    render(
      <JoinTokenGithubForm
        tokenState={state}
        onUpdateState={jest.fn()}
        readonly={false}
      />,
      { wrapper: Wrapper }
    );

    expect(screen.queryAllByTestId('delete_rule').length).toBe(2);
  });

  it('a rule can be deleted', async () => {
    const state = baseState({
      rules: [{}, {}],
    });
    const onUpdate = jest.fn();

    render(
      <JoinTokenGithubForm
        tokenState={state}
        onUpdateState={onUpdate}
        readonly={false}
      />,
      { wrapper: Wrapper }
    );

    const rule0 = screen.getByTestId('rule_0');
    const deleteButton0 = within(rule0).getByTestId('delete_rule');

    await userEvent.click(deleteButton0);

    expect(onUpdate).toHaveBeenCalledTimes(1);
    expect(onUpdate).toHaveBeenLastCalledWith(
      baseState({
        rules: [state.github.rules[0]],
      })
    );
  });

  // eslint-disable-next-line jest/expect-expect
  it(
    'repository field can be populated',
    populateRuleFieldTest(
      'repository',
      'gravitational/teleport',
      'gravitational/teleport'
    )
  );

  it('repository field shows a validation message', async () => {
    const state = baseState();

    render(
      <JoinTokenGithubForm
        tokenState={state}
        onUpdateState={jest.fn()}
        readonly={false}
      />,
      { wrapper: Wrapper }
    );

    await userEvent.click(screen.getByTestId('submit'));

    expect(
      screen.getByText('Either repository name or owner is required')
    ).toBeInTheDocument();
  });

  // eslint-disable-next-line jest/expect-expect
  it(
    'repository owner field can be populated',
    populateRuleFieldTest('repository_owner', 'gravitational', 'gravitational')
  );

  it('repository owner field shows a validation message', async () => {
    const state = baseState();

    render(
      <JoinTokenGithubForm
        tokenState={state}
        onUpdateState={jest.fn()}
        readonly={false}
      />,
      { wrapper: Wrapper }
    );

    await userEvent.click(screen.getByTestId('submit'));

    expect(
      screen.getByText('Either repository owner or name is required')
    ).toBeInTheDocument();
  });

  // eslint-disable-next-line jest/expect-expect
  it(
    'workflow field can be populated',
    populateRuleFieldTest('workflow', 'my-workflow', 'my-workflow')
  );

  // eslint-disable-next-line jest/expect-expect
  it(
    'environment field can be populated',
    populateRuleFieldTest('environment', 'production', 'production')
  );

  // eslint-disable-next-line jest/expect-expect
  it(
    'ref field can be populated',
    populateRuleFieldTest('ref', 'ref/heads/main', 'ref/heads/main')
  );

  it('ref type is disabled when ref is not populated', async () => {
    const state = baseState();

    render(
      <JoinTokenGithubForm
        tokenState={state}
        onUpdateState={jest.fn()}
        readonly={false}
      />,
      { wrapper: Wrapper }
    );

    expect(screen.getByLabelText('Ref type')).toBeDisabled();
  });

  it('ref type can be selected', async () => {
    const state = baseState({
      rules: [
        {
          ref: 'ref/heads/main',
          ref_type: 'any',
        },
      ],
    });
    const onUpdate = jest.fn();

    render(
      <JoinTokenGithubForm
        tokenState={state}
        onUpdateState={onUpdate}
        readonly={false}
      />,
      { wrapper: Wrapper }
    );

    const selectElement = screen.getByLabelText('Ref type');
    expect(selectElement).toBeEnabled();

    // Seems to be the only way to interact with react-select component
    fireEvent.keyDown(selectElement, { key: 'ArrowDown' });
    const existingItem = await screen.findByText('Branch');
    fireEvent.click(existingItem);

    expect(onUpdate).toHaveBeenCalledTimes(1);
    expect(onUpdate).toHaveBeenLastCalledWith(
      baseState({
        rules: [
          {
            ...state.github.rules[0],
            ref_type: 'branch',
          },
        ],
      })
    );
  });

  // eslint-disable-next-line jest/expect-expect
  it(
    'server host field can be populated',
    populateFieldTest({
      field: 'server_host',
      placeholer: 'github.example.com',
      value: 'github.example.com',
    })
  );

  // eslint-disable-next-line jest/expect-expect
  it(
    'slug field can be populated',
    populateFieldTest({
      field: 'enterprise_slug',
      placeholer: 'octo-enterprise',
      value: 'octo-enterprise',
    })
  );

  // eslint-disable-next-line jest/expect-expect
  it(
    'jwks field can be populated',
    populateFieldTest({
      field: 'static_jwks',
      placeholer: '{"keys":[--snip--]}',
      value: '{"keys":[]}',
    })
  );
});

const Wrapper = ({ children }: PropsWithChildren) => {
  return (
    <ThemeProvider>
      <Validation>
        <SubmitWrapper>{children}</SubmitWrapper>
      </Validation>
    </ThemeProvider>
  );
};

const SubmitWrapper = ({ children }: PropsWithChildren) => {
  const validation = useValidation();

  return (
    <>
      {children}
      <button data-testid="submit" onClick={() => validation.validate()} />
    </>
  );
};

const baseState = (
  github: Partial<NewJoinTokenState['github']> = {}
): NewJoinTokenState => ({
  name: 'test-name',
  method: { label: 'github', value: 'github' },
  roles: [{ label: 'Bot', value: 'Bot' }],
  bot_name: 'test-bot-name',
  github: {
    ...github,
    rules: github.rules ?? [
      {
        ref_type: 'any',
      },
    ],
  },
  iam: [],
  gcp: [],
  oracle: [],
});
