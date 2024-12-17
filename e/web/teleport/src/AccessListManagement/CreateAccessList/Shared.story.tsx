import { useState } from 'react';

import Validation from 'shared/components/Validation';
import { Option } from 'shared/components/Select';

import { EligibilityOrGrantRolesFieldSelectAndCreate } from './Shared';

export default {
  title: 'TeleportE/AccessLists/Create/Roles Select',
};

export function Loaded() {
  const [options, setOptions] = useState<Option[]>([]);
  return (
    <Validation>
      <EligibilityOrGrantRolesFieldSelectAndCreate
        loadOptions={async function (): Promise<Option[]> {
          return [
            { label: 'foo', value: 'foo' },
            { label: 'bar', value: 'bar' },
          ];
        }}
        isDisabled={false}
        onChange={setOptions}
        selected={options}
        editKind={'Member'}
      />
    </Validation>
  );
}

export function LoadError() {
  const [options, setOptions] = useState<Option[]>([]);
  return (
    <Validation>
      <EligibilityOrGrantRolesFieldSelectAndCreate
        loadOptions={async function (): Promise<Option[]> {
          throw new Error('Server error');
        }}
        isDisabled={false}
        onChange={setOptions}
        selected={options}
        editKind={'Member'}
      />
    </Validation>
  );
}

export function Loading() {
  const [options, setOptions] = useState<Option[]>([]);
  return (
    <Validation>
      <EligibilityOrGrantRolesFieldSelectAndCreate
        loadOptions={async function (): Promise<Option[]> {
          return new Promise(() => {});
        }}
        isDisabled={false}
        onChange={setOptions}
        selected={options}
        editKind={'Member'}
      />
    </Validation>
  );
}
