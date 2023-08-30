import React from 'react';
import { Box, Flex } from 'design';
import FieldInput from 'shared/components/FieldInput';
import FieldSelect from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import { requiredField } from 'shared/components/Validation/rules';

import { CalendarDateSelect, auditFrequencyOpts } from '../Shared';

import { EligibilityOrGrantRolesFieldSelectAndCreate } from './Shared';

type Props = {
  spec: Spec;
  setSpec(s: Spec): void;
  isDisabled: boolean;
  roleOptions: Option[];
};

export type Spec = {
  title: string;
  description: string;
  auditFrequency: Option;
  auditStartDate: Date;
  rolesToGrant: Option[];
};

export const SpecSection = ({
  spec,
  setSpec,
  isDisabled,
  roleOptions,
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
      <EligibilityOrGrantRolesFieldSelectAndCreate
        options={roleOptions}
        isDisabled={isDisabled}
        onChange={(roles: Option[]) =>
          setSpec({ ...spec, rolesToGrant: roles || [] })
        }
        selected={spec.rolesToGrant}
        editKind="Grants"
      />
    </>
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
