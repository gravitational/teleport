import React from 'react';
import ReactSelectCreatable from 'react-select/creatable';
import FieldSelect from 'shared/components/FieldSelect';
import { requiredField } from 'shared/components/Validation/rules';

import {
  FieldSelectAndCreatableWrapper,
  RoleOption,
  UserOption,
} from '../Shared';

export function EligibilityRolesFieldSelect({
  options,
  isDisabled,
  onChange,
  selected,
  requiredErrMsg = '',
}: {
  options: RoleOption[];
  isDisabled: boolean;
  onChange(opts: RoleOption[]): void;
  selected: RoleOption[];
  requiredErrMsg?: string;
}) {
  return (
    <FieldSelect
      label="Eligibility: Roles Required"
      isMulti={true}
      isSearchable={true}
      options={options}
      isDisabled={isDisabled}
      rule={requiredErrMsg ? requiredField(requiredErrMsg) : undefined}
      onChange={onChange}
      value={selected}
    />
  );
}

export function EligibleUsersFieldSelectAndCreate({
  selected,
  isDisabled,
  onChange,
  options,
  label,
  requiredErrMsg = '',
}: {
  selected: UserOption[];
  isDisabled: boolean;
  onChange(opts: UserOption[]): void;
  options: UserOption[];
  label: string;
  requiredErrMsg?: string;
}) {
  return (
    <FieldSelectAndCreatableWrapper<UserOption>
      label={label}
      value={selected}
      rule={requiredErrMsg ? requiredField(requiredErrMsg) : undefined}
    >
      <ReactSelectCreatable
        classNamePrefix="react-select"
        placeholder="Start typing a username and press enter"
        isMulti={true}
        isClearable={true}
        isDisabled={isDisabled}
        value={selected || []}
        onChange={onChange}
        options={options}
        noOptionsMessage={() => 'No eligible users found'}
      />
    </FieldSelectAndCreatableWrapper>
  );
}
