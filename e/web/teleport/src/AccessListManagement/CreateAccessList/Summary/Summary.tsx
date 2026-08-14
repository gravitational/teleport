import { useState } from 'react';

import { Box, ButtonSecondary, Flex, H1, Text } from 'design';
import { ChevronDown, ChevronRight } from 'design/Icon';

import {
  AwsIcRoleConditions,
  StandardRoleConditions,
} from 'e-teleport/AccessListManagement/GuideEditor/Preset/role/conditions';
import { definableResourceAccessFields } from 'e-teleport/AccessListManagement/GuideEditor/Preset/role/listaccess';
import { AccessListPreset } from 'e-teleport/services/accessmanagement/preset';

import { Members, Owners, Spec } from '../types';
import { AccessSection } from './AccessSection';
import { BasicInfoSection } from './BasicInfoSection';
import { SelectedUsersSection } from './SelectedUsersSection';

export function Summary({
  preset,
  spec,
  members,
  owners,
  definedAccessFields,
  awsIcRoleConditions,
  standardRoleConditions,
}: {
  preset: AccessListPreset;
  spec: Spec;
  members: Members;
  owners: Owners;
  definedAccessFields: typeof definableResourceAccessFields;
  awsIcRoleConditions: AwsIcRoleConditions;
  standardRoleConditions: StandardRoleConditions;
}) {
  const [summaryExpanded, setSummaryExpanded] = useState(false);
  const ArrowIcon = summaryExpanded ? ChevronDown : ChevronRight;

  const hasAccessDefined = definedAccessFields.length > 0;
  return (
    <Box width="620px">
      <H1 mb={3}>Summary</H1>

      <Flex gap={3} flexDirection={'column'}>
        <BasicInfoSection
          preset={preset}
          spec={spec}
          hideBottomBorder={!summaryExpanded}
        />

        <Flex justifyContent={'center'}>
          <ButtonSecondary
            onClick={() => setSummaryExpanded(e => !e)}
            pl={3}
            pr={2}
          >
            <Flex alignItems={'center'} gap={2}>
              <Text bold>
                {summaryExpanded ? 'Hide Summary' : 'Expand Summary'}
              </Text>
              <ArrowIcon size={16} />
            </Flex>
          </ButtonSecondary>
        </Flex>

        {summaryExpanded && (
          <>
            <SelectedUsersSection
              title="Owners"
              kind="owners"
              selectedUsers={owners.selectedOwners}
              requiredRoles={owners.selectedRolesRequired}
              requiredTraits={owners.traitLabels}
            />
            <SelectedUsersSection
              title="Members"
              kind="members"
              selectedUsers={members.selectedMembers}
              requiredRoles={members.selectedRolesRequired}
              requiredTraits={members.traitLabels}
            />
          </>
        )}

        {summaryExpanded && hasAccessDefined && (
          <AccessSection
            definedAccessFields={definedAccessFields}
            awsIcRoleConditions={awsIcRoleConditions}
            standardRoleConditions={standardRoleConditions}
          />
        )}
      </Flex>
    </Box>
  );
}
