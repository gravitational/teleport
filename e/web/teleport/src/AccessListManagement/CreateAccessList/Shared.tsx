import styled from 'styled-components';

import {
  FieldSelectCreatable,
  FieldSelectCreatableAsync,
} from 'shared/components/FieldSelect/FieldSelectCreatable';
import { Option } from 'shared/components/Select';
import { requiredField } from 'shared/components/Validation/rules';

import {
  AccessListMemberKind,
  type AccessList,
} from 'e-teleport/services/accessmanagement';

import {
  ReactSelectAccessListMultiValue,
  ReactSelectAccessListOption,
  type EditKind,
  type HybridUserOption,
} from '../Shared/Shared';

export function convertAccessListsToUserOptions(
  acls: AccessList[],
  existingUsers: HybridUserOption[]
) {
  return acls
    .filter(l => existingUsers.every(m => m.value.name !== l.id))
    .map(
      x =>
        ({
          label: x.title,
          value: {
            name: x.id,
            membershipKind: AccessListMemberKind.List,
            roles: [],
          },
        }) satisfies HybridUserOption
    );
}

export function EligibilityOrGrantRolesFieldSelectAndCreate({
  loadOptions,
  isDisabled,
  onChange,
  selected,
  autoFocus = false,
  editKind,
  optional = false,
}: {
  loadOptions(input: string): Promise<Option[]>;
  isDisabled: boolean;
  onChange(opts: Option[]): void;
  selected: Option[];
  autoFocus?: boolean;
  editKind: EditKind;
  optional?: boolean;
}) {
  let requiredErrMsg = 'Roles granted are required';
  let label = 'Roles Granted';

  if (editKind !== 'Grants') {
    requiredErrMsg = `Eligibility roles are required`;
    label = `Required Roles (Optional)`;
  }

  return (
    <FieldSelectCreatableAsync
      label={label}
      value={selected}
      rule={optional ? undefined : requiredField(requiredErrMsg)}
      menuPosition="fixed"
      autoFocus={autoFocus}
      classNamePrefix="react-select"
      placeholder="Start typing a role name and press enter"
      isMulti={true}
      isClearable={true}
      loadOptions={loadOptions}
      defaultOptions={true}
      isDisabled={isDisabled}
      onChange={onChange}
      noOptionsMessage={() => 'Start typing a role name and press enter'}
    />
  );
}

// Used for consistent spacing between selected items and the container border
// when line-wrapping occurs.
const FieldSelectCreatableWrapper = styled.div`
  display: contents;

  .react-select__value-container {
    margin: ${p => p.theme.space[1]}px 0;
  }
  .react-select__input-container {
    margin: 0;
    padding: 0 calc(${p => p.theme.space[1]}px / 2);
  }
`;

export function EligibleUsersFieldSelectAndCreate<T extends HybridUserOption>({
  selected,
  isDisabled,
  onChange,
  options,
  label,
  requiredErrMsg = '',
  autoFocus = false,
  noEligibleUsersFromNoAccess = false,
}: {
  selected: T[];
  isDisabled: boolean;
  onChange(opts: T[]): void;
  options: T[];
  label: string;
  requiredErrMsg?: string;
  autoFocus?: boolean;
  noEligibleUsersFromNoAccess?: boolean;
}) {
  let noOptionsMsg = 'No eligible users found';

  // If a user had no access to list users,
  // then we can't calculate eligible users.
  if (noEligibleUsersFromNoAccess) {
    noOptionsMsg =
      'Start typing a username or an Access List name and press enter';
  }
  return (
    <FieldSelectCreatableWrapper>
      <FieldSelectCreatable
        label={label}
        value={selected}
        rule={requiredErrMsg ? requiredField(requiredErrMsg) : undefined}
        menuPosition="fixed"
        autoFocus={autoFocus}
        classNamePrefix="react-select"
        placeholder="Start typing a username or an Access List name and press enter"
        isMulti={true}
        isClearable={true}
        isDisabled={isDisabled}
        onChange={onChange}
        options={options}
        noOptionsMessage={() => noOptionsMsg}
        components={{
          Option: ReactSelectAccessListOption,
          MultiValue: ReactSelectAccessListMultiValue,
        }}
      />
    </FieldSelectCreatableWrapper>
  );
}
