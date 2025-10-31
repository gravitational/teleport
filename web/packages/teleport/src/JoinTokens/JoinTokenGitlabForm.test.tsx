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
import selectEvent from 'react-select-event';

import { render, screen, within } from 'design/utils/testing';
import Validation, { useValidation } from 'shared/components/Validation';

import { ThemeProvider } from 'teleport/ThemeProvider';

import { JoinTokenGitlabForm } from './JoinTokenGitlabForm';
import {
  NewJoinTokenGitlabStateRule,
  NewJoinTokenState,
} from './UpsertJoinTokenDialog';

const populateRuleFieldTest =
  (
    field: keyof NewJoinTokenGitlabStateRule,
    placeholer: string,
    value: string
  ) =>
  async () => {
    const { user, onUpdate } = renderComponent();

    const input = screen.getByPlaceholderText(placeholer);
    expect(input).toBeEnabled();

    await user.click(input);
    await user.paste(value);

    expect(onUpdate).toHaveBeenCalledTimes(1);
    expect(onUpdate).toHaveBeenLastCalledWith(
      baseState({
        rules: [
          {
            ref_type: 'any',
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
    field: keyof NonNullable<NewJoinTokenState['gitlab']>;
    placeholer: string;
    value: string;
  }) =>
  async () => {
    const { user, onUpdate } = renderComponent();

    const input = screen.getByPlaceholderText(placeholer);
    expect(input).toBeEnabled();

    await user.click(input);
    await user.paste(value);

    expect(onUpdate).toHaveBeenCalledTimes(1);
    expect(onUpdate).toHaveBeenLastCalledWith(
      baseState({
        [field]: value,
      })
    );
  };

describe('GitlabJoinTokenForm', () => {
  it('a rule can be added', async () => {
    const { user, onUpdate } = renderComponent();

    await user.click(
      screen.getByRole('button', { name: /Add another GitLab rule/i })
    );

    expect(onUpdate).toHaveBeenCalledTimes(1);
    expect(onUpdate).toHaveBeenLastCalledWith(
      baseState({
        rules: [
          {
            ref_type: 'any',
          },
          {
            ref_type: 'any',
          },
        ],
      })
    );
  });

  it('delete button is hidden when only one rule exists', async () => {
    renderComponent();

    expect(screen.queryByTestId('delete_rule')).not.toBeInTheDocument();
  });

  it('delete button is visible when more than one rule exists', async () => {
    const state = baseState({
      rules: [{}, {}],
    });

    renderComponent({ state });

    expect(screen.queryAllByTestId('delete_rule').length).toBe(2);
  });

  it('a rule can be deleted', async () => {
    const state = baseState({
      rules: [{}, {}],
    });

    const { user, onUpdate } = renderComponent({ state });

    const rule0 = screen.getByTestId('rule_0');
    const deleteButton0 = within(rule0).getByTestId('delete_rule');

    await user.click(deleteButton0);

    expect(onUpdate).toHaveBeenCalledTimes(1);
    expect(onUpdate).toHaveBeenLastCalledWith(
      baseState({
        rules: state.gitlab?.rules ? [state.gitlab.rules[0]] : [],
      })
    );
  });

  // eslint-disable-next-line jest/expect-expect
  it(
    'project path field can be populated',
    populateRuleFieldTest(
      'project_path',
      'my-user/my-project',
      'gravitational/teleport'
    )
  );

  it('project path field shows a validation message', async () => {
    const { user } = renderComponent();

    await user.click(screen.getByTestId('submit'));

    expect(
      screen.getByText('Either project path or namespace path is required')
    ).toBeInTheDocument();
  });

  // eslint-disable-next-line jest/expect-expect
  it(
    'namespace path field can be populated',
    populateRuleFieldTest('namespace_path', 'my-user', 'gravitational')
  );

  it('namespace path field shows a validation message', async () => {
    const { user } = renderComponent();

    await user.click(screen.getByTestId('submit'));

    expect(
      screen.getByText('Either namespace path or project path is required')
    ).toBeInTheDocument();
  });

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
    renderComponent();
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

    const { onUpdate } = renderComponent({ state });

    const selectElement = screen.getByLabelText('Ref type');
    expect(selectElement).toBeEnabled();

    await selectEvent.select(selectElement, ['Branch']);

    expect(onUpdate).toHaveBeenCalledTimes(1);
    expect(onUpdate).toHaveBeenLastCalledWith(
      baseState({
        rules: state.gitlab?.rules
          ? [
              {
                ...state.gitlab.rules[0],
                ref_type: 'branch',
              },
            ]
          : [],
      })
    );
  });

  // eslint-disable-next-line jest/expect-expect
  it(
    'domain field can be populated',
    populateFieldTest({
      field: 'domain',
      placeholer: 'gitlab.example.com',
      value: 'gitlab.example.com',
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

function renderComponent(options?: { state?: NewJoinTokenState }) {
  const { state = baseState() } = options ?? {};
  const onUpdate = jest.fn();
  const user = userEvent.setup();
  return {
    ...render(
      <JoinTokenGitlabForm
        tokenState={state}
        onUpdateState={onUpdate}
        readonly={false}
      />,
      { wrapper: Wrapper }
    ),
    onUpdate,
    user,
  };
}

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
  gitlab: Partial<NewJoinTokenState['gitlab']> = {}
): NewJoinTokenState => ({
  name: 'test-name',
  method: { label: 'gitlab', value: 'gitlab' },
  roles: [{ label: 'Bot', value: 'Bot' }],
  bot_name: 'test-bot-name',
  gitlab: {
    ...gitlab,
    rules: gitlab.rules ?? [
      {
        ref_type: 'any',
      },
    ],
  },
});
