import { useState } from 'react';
import selectEvent from 'react-select-event';

import { render, screen, userEvent } from 'design/utils/testing';
import type { Option } from 'shared/components/Select';
import Validation from 'shared/components/Validation';

import {
  AccessListMemberKind,
  AccessListOrigin,
} from 'e-teleport/services/accessmanagement';

import type { HybridUserOption, MemberSelection } from '../Shared/Shared';
import { EligibleUsersFieldSelect } from './Shared';

function renderSelect({
  loadOptions = async () => [],
  selected = [],
  disableCreate = false,
  userKind,
}: {
  loadOptions?: (input: string) => Promise<HybridUserOption[]>;
  selected?: Option<MemberSelection>[];
  disableCreate?: boolean;
  userKind?: 'nested-access-list';
} = {}) {
  function Harness() {
    const [selectedOptions, setSelectedOptions] =
      useState<Option<MemberSelection>[]>(selected);

    return (
      <Validation>
        <EligibleUsersFieldSelect
          selected={selectedOptions}
          isDisabled={false}
          onChange={setSelectedOptions}
          loadOptions={loadOptions}
          label="Add Members"
          disableCreate={disableCreate}
          userKind={userKind}
        />
      </Validation>
    );
  }

  render(<Harness />);
}

test('renders display names for user options and selected chips', async () => {
  const user = userEvent.setup();
  const value: MemberSelection = {
    membershipKind: AccessListMemberKind.User,
    name: 'alice',
    displayPrimary: 'Alice Liddell',
    displaySecondary: 'alice@example.com',
  };
  renderSelect({
    loadOptions: async () => [
      {
        label: 'alice',
        value,
      },
    ],
  });

  selectEvent.openMenu(screen.getByRole('combobox', { name: 'Add Members' }));

  expect(await screen.findByText('Alice Liddell')).toBeVisible();
  expect(screen.getByText('alice')).toBeVisible();
  expect(screen.getByText('alice@example.com')).toBeVisible();

  await user.click(screen.getByText('Alice Liddell'));

  expect(screen.getByText('Alice Liddell')).toBeVisible();
  expect(screen.getByText('alice')).toBeVisible();
  expect(screen.queryByText('alice@example.com')).not.toBeInTheDocument();
});

test('renders username-only and free-text selected chips as usernames', async () => {
  renderSelect({
    selected: [
      {
        label: 'bob',
        value: {
          membershipKind: AccessListMemberKind.User,
          name: 'bob',
        },
      },
      {
        label: 'future-user',
        value: {
          membershipKind: AccessListMemberKind.User,
          name: 'future-user',
        },
      },
    ],
  });

  selectEvent.openMenu(screen.getByRole('combobox', { name: 'Add Members' }));
  // Let the async default-options load settle before asserting selected chips.
  await screen.findByText('No users found');

  expect(screen.getByText('bob')).toBeVisible();
  expect(screen.getByText('future-user')).toBeVisible();
});

test('keeps nested access list options rendering as list titles with origin badges', async () => {
  renderSelect({
    disableCreate: true,
    userKind: 'nested-access-list',
    loadOptions: async () => [
      {
        label: 'Interns',
        value: {
          membershipKind: AccessListMemberKind.List,
          name: 'interns',
          origin: AccessListOrigin.Okta,
        },
      },
    ],
  });

  selectEvent.openMenu(screen.getByRole('combobox', { name: /Add Members/ }));

  expect(await screen.findByRole('option', { name: /Interns/ })).toBeVisible();
  expect(screen.getByTestId('badge-okta')).toBeVisible();
});
