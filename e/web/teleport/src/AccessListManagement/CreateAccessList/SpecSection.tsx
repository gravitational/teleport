import React from 'react';
import { Box, Flex } from 'design';
import FieldInput from 'shared/components/FieldInput';
import FieldSelect from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import { requiredField } from 'shared/components/Validation/rules';

import { CalendarDateSelect, RoleOption } from '../Shared';

type Props = {
  spec: Spec;
  setSpec(s: Spec): void;
  isDisabled: boolean;
  fetchedRoleOpts: RoleOption[];
};

export type Spec = {
  title: string;
  description: string;
  auditFrequency: Option;
  auditStartDate: Date;
  rolesToGrant: RoleOption[];
};

export const auditFrequencyOpts: Option[] = [
  { label: '1 month', value: '730h' },
  { label: '2 month', value: '1460h' },
  { label: '3 month', value: '2190h' },
  { label: '4 month', value: '2920h' },
  { label: '5 month', value: '3650h' },
  { label: '6 month', value: '4380h' },
];

export const SpecSection = ({
  spec,
  setSpec,
  isDisabled,
  fetchedRoleOpts,
}: Props) => {
  return (
    <>
      <FieldInput
        label="Title"
        rule={requiredField('Title is required')}
        placeholder="Tile"
        autoFocus
        value={spec.title}
        onChange={e => setSpec({ ...spec, title: e.target.value })}
      />
      <FieldInput
        label="Description (Optional)"
        placeholder="Description"
        value={spec.description}
        onChange={e => setSpec({ ...spec, description: e.target.value })}
      />
      <Flex>
        <Box width="50%" mr={2}>
          <AuditReviewFrequencySelectField
            isDisabled={isDisabled}
            onChangeFrequency={(o: Option) =>
              setSpec({ ...spec, auditFrequency: o })
            }
            selectedFrequency={spec.auditFrequency}
          />
        </Box>
        <Box width="50%" ml={2}>
          <CalendarDateSelect
            date={spec.auditStartDate}
            onChange={(newDate: Date) =>
              setSpec({ ...spec, auditStartDate: newDate })
            }
            rule={requiredField('Review deadline required')}
            label="Deadline for First Review"
          />
        </Box>
      </Flex>
      <RolesGrantedFieldSelect
        options={fetchedRoleOpts}
        isDisabled={isDisabled}
        onChange={(roles: RoleOption[]) =>
          setSpec({ ...spec, rolesToGrant: roles || [] })
        }
        selected={spec.rolesToGrant}
      />
    </>
  );
};

export const RolesGrantedFieldSelect = ({
  options,
  isDisabled,
  onChange,
  selected,
  autoFocus = false,
}: {
  options: RoleOption[];
  isDisabled: boolean;
  onChange(roles: RoleOption[]): void;
  selected: RoleOption[];
  autoFocus?: boolean;
}) => {
  return (
    <FieldSelect
      autoFocus={autoFocus}
      label="Roles Granted"
      rule={requiredField('Roles granted is required')}
      isMulti={true}
      isSearchable={true}
      options={options}
      isDisabled={isDisabled}
      onChange={onChange}
      value={selected}
    />
  );
};

export const AuditReviewFrequencySelectField = ({
  isDisabled,
  onChangeFrequency,
  selectedFrequency,
}: {
  isDisabled: boolean;
  onChangeFrequency(o: Option): void;
  selectedFrequency: Option;
}) => {
  return (
    <FieldSelect
      label="Review Frequency"
      isSearchable={true}
      options={auditFrequencyOpts}
      isDisabled={isDisabled}
      onChange={onChangeFrequency}
      value={selectedFrequency}
    />
  );
};
