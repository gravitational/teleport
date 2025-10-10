import { delay, http, HttpResponse } from 'msw';
import { useState } from 'react';

import { Option } from 'shared/components/Select';
import Validation from 'shared/components/Validation';

import { TeleportProviderBasicE } from 'e-teleport/mocks/providers';
import cfg from 'teleport/config';

import { EligibilityOrGrantRolesFieldSelectAndCreate } from './Shared';

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
Loaded.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getRoleUrl({ action: 'list' }), async () => {
        await delay(1000);
        return HttpResponse.json({
          items: rawRoles,
        });
      }),
    ],
  },
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
LoadError.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getRoleUrl({ action: 'list' }), () => {
        return HttpResponse.json(
          {
            error: { message: 'Whoops, some error' },
          },
          { status: 404 }
        );
      }),
    ],
  },
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
Loading.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getRoleUrl({ action: 'list' }), async () => {
        return delay('infinite');
      }),
    ],
  },
};
