import { requiredField } from 'shared/components/Validation/rules';
import { Option } from 'shared/components/Select';

import {
  FieldSelectCreatable,
  FieldSelectCreatableAsync,
} from 'shared/components/FieldSelect/FieldSelectCreatable';

import {
  EditKind,
  HybridUserOption,
} from 'e-teleport/AccessListManagement/Shared/Shared';

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

export function EligibleUsersFieldSelectAndCreate<T = HybridUserOption>({
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
    noOptionsMsg = 'Start typing a username and press enter';
  }
  return (
    <FieldSelectCreatable
      label={label}
      value={selected}
      rule={requiredErrMsg ? requiredField(requiredErrMsg) : undefined}
      menuPosition="fixed"
      autoFocus={autoFocus}
      classNamePrefix="react-select"
      placeholder="Start typing a username and press enter"
      isMulti={true}
      isClearable={true}
      isDisabled={isDisabled}
      onChange={onChange}
      options={options}
      noOptionsMessage={() => noOptionsMsg}
    />
  );
}
