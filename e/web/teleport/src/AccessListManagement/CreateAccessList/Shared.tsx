import styled from 'styled-components';

import Link from 'design/Link';
import { FieldSelectAsync } from 'shared/components/FieldSelect';
import { FieldSelectCreatableAsync } from 'shared/components/FieldSelect/FieldSelectCreatable';
import { Option } from 'shared/components/Select';
import { requiredField } from 'shared/components/Validation/rules';

import ResourceService from 'teleport/services/resources';

import {
  ReactSelectAccessListMultiValue,
  ReactSelectAccessListOption,
} from '../Shared/Shared';
import { RolesSelectedFor, UserKind } from '../Shared/types';
import { EligibleUsersFieldSelectProps } from './types';

// EligibilityOrGrantRolesFieldSelectAndCreate is used to define
// eligibility (what roles are required to be eligible members/owners)
// or access grants (what additional roles are granted to members/owners).
export function EligibilityOrGrantRolesFieldSelectAndCreate({
  isDisabled,
  onChange,
  selected,
  autoFocus = false,
  rolesSelectedFor,
  optional = false,
  userKind,
}: {
  isDisabled: boolean;
  onChange(opts: Option[]): void;
  selected: Option[];
  autoFocus?: boolean;
  rolesSelectedFor: RolesSelectedFor;
  optional?: boolean;
  userKind: UserKind;
}) {
  let requiredErrMsg = 'Roles granted are required';
  let label = `Roles Granted to ${userKind}`;

  if (rolesSelectedFor === 'eligibility') {
    requiredErrMsg = `Eligibility roles are required`;
    label = `Required Roles (Optional)`;
  }

  async function fetchRoleOptions(search: string): Promise<Option[]> {
    const resourceSvc = new ResourceService();
    const roles = await resourceSvc.fetchRoles({ search, limit: 50 });
    return roles.items.map(r => ({ value: r.name, label: r.name }));
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
      loadOptions={fetchRoleOptions}
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

  .react-select__input-container {
    margin: 0;
    padding: 0 calc(${p => p.theme.space[1]}px / 2);
  }
`;

export function EligibleUsersFieldSelect({
  selected,
  isDisabled,
  onChange,
  loadOptions,
  label,
  placeholder = 'Start typing a username or an Access List name and press enter',
  disableCreate = false,
  requiredErrMsg = '',
  autoFocus = false,
  noOptionsMsg = 'No users found',
  userKind,
  key,
}: EligibleUsersFieldSelectProps) {
  const Component = disableCreate
    ? FieldSelectAsync
    : FieldSelectCreatableAsync;

  return (
    <FieldSelectCreatableWrapper>
      <Component
        key={key}
        label={label}
        value={selected}
        rule={requiredErrMsg ? requiredField(requiredErrMsg) : undefined}
        menuPosition="fixed"
        autoFocus={autoFocus}
        classNamePrefix="react-select"
        placeholder={placeholder}
        isMulti={true}
        isSearchable={true}
        isClearable={false}
        isDisabled={isDisabled}
        onChange={onChange}
        loadOptions={loadOptions}
        defaultOptions={true}
        noOptionsMessage={() => noOptionsMsg}
        components={{
          Option: ReactSelectAccessListOption,
          MultiValue: ReactSelectAccessListMultiValue,
        }}
        {...(userKind === 'nested-access-list' && {
          toolTipContent: (
            <>
              Learn more about{' '}
              <Link
                target="_blank"
                href="https://goteleport.com/docs/identity-governance/access-lists/nested-access-lists/#how-it-works"
              >
                Nested Access Lists
              </Link>
            </>
          ),
          tooltipSticky: true,
        })}
      />
    </FieldSelectCreatableWrapper>
  );
}
