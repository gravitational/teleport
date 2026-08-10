import { delay, http, HttpResponse } from 'msw';
import { useState } from 'react';

import { Option } from 'shared/components/Select';
import Validation from 'shared/components/Validation';

import { TeleportProviderBasicE } from 'e-teleport/mocks/providers';
import {
  AccessListMemberKind,
  AccessListOrigin,
} from 'e-teleport/services/accessmanagement';
import cfg from 'teleport/config';

import type { MemberSelection } from '../Shared/Shared';
import {
  EligibilityOrGrantRolesFieldSelectAndCreate,
  EligibleUsersFieldSelect,
} from './Shared';

export default {
  title: 'TeleportE/AccessLists/Create/RolesSelect',
};

const rawRoles = [
  { id: 'id1', kind: 'role', name: 'access', content: '' },
  { id: 'id2', kind: 'role', name: 'admin', content: '' },
  { id: 'id3', kind: 'role', name: 'editor', content: '' },
  { id: 'id4', kind: 'role', name: 'foo', content: '' },
  { id: 'id5', kind: 'role', name: 'apple', content: '' },
  { id: 'id6', kind: 'role', name: 'banana', content: '' },
];

const displayUserOptions: Option<MemberSelection>[] = [
  {
    label: 'alice',
    value: {
      membershipKind: AccessListMemberKind.User,
      name: 'alice',
      displayPrimary: 'Alice Liddell',
      displaySecondary: 'alice@example.com',
    },
  },
  {
    label: 'grace',
    value: {
      membershipKind: AccessListMemberKind.User,
      name: 'grace',
      displayPrimary: 'Grace Hopper',
    },
  },
  {
    label: 'charlie',
    value: {
      membershipKind: AccessListMemberKind.User,
      name: 'charlie',
    },
  },
  {
    label: 'long-name',
    value: {
      membershipKind: AccessListMemberKind.User,
      name: 'long-name',
      displayPrimary:
        'Alexandria Montgomery-Fitzwilliam With An Exceptionally Long Display Name',
      displaySecondary:
        'alexandria.montgomery-fitzwilliam@example-very-long-domain.test',
    },
  },
];

const nestedListOptions: Option<MemberSelection>[] = [
  {
    label: 'Engineering Access Review',
    value: {
      membershipKind: AccessListMemberKind.List,
      name: 'engineering-access-review',
      origin: AccessListOrigin.Okta,
    },
  },
];

export function Loaded() {
  const [options, setOptions] = useState<Option[]>([]);
  return (
    <Validation>
      <TeleportProviderBasicE>
        <EligibilityOrGrantRolesFieldSelectAndCreate
          isDisabled={false}
          onChange={setOptions}
          selected={options}
          userKind={'Members'}
          rolesSelectedFor="grants"
        />
        <EligibilityOrGrantRolesFieldSelectAndCreate
          isDisabled={false}
          onChange={setOptions}
          selected={options}
          userKind={'Owners'}
          rolesSelectedFor="grants"
        />
        <EligibilityOrGrantRolesFieldSelectAndCreate
          isDisabled={false}
          onChange={setOptions}
          selected={options}
          userKind={'Members'}
          rolesSelectedFor="eligibility"
        />
        <EligibilityOrGrantRolesFieldSelectAndCreate
          isDisabled={false}
          onChange={setOptions}
          selected={options}
          userKind={'Owners'}
          rolesSelectedFor="eligibility"
        />
      </TeleportProviderBasicE>
    </Validation>
  );
}
Loaded.beforeEach = ({ msw }) => {
  msw.use(
    http.get(cfg.getRoleUrl({ action: 'list' }), async () => {
      await delay(1000);
      return HttpResponse.json({
        items: rawRoles,
      });
    })
  );
};

export function LoadError() {
  const [options, setOptions] = useState<Option[]>([]);
  return (
    <Validation>
      <TeleportProviderBasicE>
        <EligibilityOrGrantRolesFieldSelectAndCreate
          isDisabled={false}
          onChange={setOptions}
          selected={options}
          userKind={'Members'}
          rolesSelectedFor="grants"
        />
      </TeleportProviderBasicE>
    </Validation>
  );
}
LoadError.beforeEach = ({ msw }) => {
  msw.use(
    http.get(cfg.getRoleUrl({ action: 'list' }), () => {
      return HttpResponse.json(
        {
          error: { message: 'Whoops, some error' },
        },
        { status: 404 }
      );
    })
  );
};

export function Loading() {
  const [options, setOptions] = useState<Option[]>([]);
  return (
    <Validation>
      <TeleportProviderBasicE>
        <EligibilityOrGrantRolesFieldSelectAndCreate
          isDisabled={false}
          onChange={setOptions}
          selected={options}
          userKind={'Members'}
          rolesSelectedFor="grants"
        />
      </TeleportProviderBasicE>
    </Validation>
  );
}
Loading.beforeEach = ({ msw }) => {
  msw.use(
    http.get(cfg.getRoleUrl({ action: 'list' }), async () => {
      return delay('infinite');
    })
  );
};

export function UserPickerDisplayNames() {
  const [options, setOptions] = useState<Option<MemberSelection>[]>([
    displayUserOptions[0],
    {
      label: 'future-user@example.com',
      value: {
        membershipKind: AccessListMemberKind.User,
        name: 'future-user@example.com',
      },
    },
  ]);

  return (
    <Validation>
      <TeleportProviderBasicE>
        <EligibleUsersFieldSelect
          label="Add Members"
          isDisabled={false}
          onChange={setOptions}
          selected={options}
          loadOptions={async () => displayUserOptions}
          placeholder="Search for a user…"
        />
      </TeleportProviderBasicE>
    </Validation>
  );
}

export function NestedListPicker() {
  const [options, setOptions] = useState<Option<MemberSelection>[]>([
    nestedListOptions[0],
  ]);

  return (
    <Validation>
      <TeleportProviderBasicE>
        <EligibleUsersFieldSelect
          label="Add Access Lists as Members"
          isDisabled={false}
          onChange={setOptions}
          selected={options}
          loadOptions={async () => nestedListOptions}
          placeholder="Search for an access list…"
          disableCreate
          userKind="nested-access-list"
        />
      </TeleportProviderBasicE>
    </Validation>
  );
}
