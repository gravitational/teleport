import { useId, useMemo, useState } from 'react';
import styled from 'styled-components';

import { Box, Text } from 'design';
import { SlideTabs } from 'design/SlideTabs';

import { resourceAccessSections } from 'teleport/Roles/RoleEditor/StandardEditor/Resources';
import { useStandardModel } from 'teleport/Roles/RoleEditor/StandardEditor/useStandardModel';

import { useAccessListManagementContext } from '../../../AccessListManagementContext';
import { definableResourceAccessFields } from '../../Preset/role/listaccess';
import {
  getResourceAccessTabSpecs,
  getRoleSectionInputFieldConfig,
} from '../../tabs';
import { ReadAwsIcAccessSection } from './ReadAwsIcAccessSection';

export function ReadRoleAccessTabsAndSection() {
  const { guideEditor } = useAccessListManagementContext();
  const { standardRoleState, awsIcRoleState, definedAccess } = guideEditor;

  const initStandardRoleModel = useMemo(() => getInitRoleForAccessGraph(), []);
  const [role] = useStandardModel(initStandardRoleModel);

  const idPrefix = useId();
  const [currentTab, setCurrentTab] = useState(0);

  function getInitRoleForAccessGraph() {
    if (standardRoleState.roleEditState?.original) {
      return standardRoleState.roleEditState.original;
    }

    if (awsIcRoleState.roleEditState?.original) {
      return awsIcRoleState.roleEditState.original;
    }
  }

  const tabSpecs = getResourceAccessTabSpecs({
    idPrefix,
    accessKind: 'access',
    accessFields: definableResourceAccessFields.filter(field =>
      definedAccess(field)
    ),
  });

  const selectedTab = tabSpecs[currentTab];
  const selectedTabKind = selectedTab.kind;

  let Section: React.ReactNode;
  switch (selectedTabKind) {
    case 'awsIc':
      Section = <ReadAwsIcAccessSection />;
      break;

    case 'app':
    case 'db':
    case 'git_server':
    case 'kube_cluster':
    case 'node':
    case 'windows_desktop':
      const section = resourceAccessSections[selectedTabKind];
      const currentResourceIndex = role.roleModel.resources.findIndex(
        r => r.kind == selectedTabKind
      );
      const sectionValue = role.roleModel.resources[currentResourceIndex];
      const sectionValidation =
        role.validationResult.resources[currentResourceIndex];

      Section = (
        <section.component
          visibleInputFields={getRoleSectionInputFieldConfig({
            kind: sectionValue.kind,
            requiredAppIdentities: standardRoleState.requiredAppIdentities,
            withLabels: true,
          })}
          value={sectionValue}
          isProcessing={false}
          validation={sectionValidation}
          onChange={() => null}
          readOnly={true}
        />
      );
  }

  return (
    <Box
      backgroundColor="levels.surface"
      borderRadius={3}
      width={'380px'}
      css={`
        overflow: scroll;
      `}
    >
      <StickyTabs px={2} pt={2}>
        <SlideTabs
          intent="neutral"
          appearance="round"
          size="medium"
          hideStatusIconOnActiveTab
          tabs={tabSpecs}
          activeIndex={currentTab}
          onChange={setCurrentTab}
        />
      </StickyTabs>
      <Box p={2}>
        <Box p={2} mt={1}>
          <Text bold fontSize={3} mb={3}>
            {selectedTab.sectionTitle}
          </Text>
          {Section}
        </Box>
      </Box>
    </Box>
  );
}

const StickyTabs = styled(Box)`
  position: sticky;
  top: 0;
  background-color: ${props => props.theme.colors.levels.surface};
  z-index: 1;
`;
