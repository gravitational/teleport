import { useId, useState } from 'react';
import styled from 'styled-components';

import { Box, Text } from 'design';
import { SlideTabs } from 'design/SlideTabs';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import { resourceAccessSections } from 'teleport/Roles/RoleEditor/StandardEditor/Resources';
import {
  ResourceAccess,
  StandardEditorModel,
} from 'teleport/Roles/RoleEditor/StandardEditor/standardmodel';
import {
  ActionType,
  StandardModelDispatcher,
} from 'teleport/Roles/RoleEditor/StandardEditor/useStandardModel';
import { AccessListStepStatusEvent } from 'teleport/services/userEvent/accessListEvents';

import {
  getResourceAccessTabSpecs,
  getRoleSectionInputFieldConfig,
} from '../../tabs';
import { AccessRoleEditor } from '../../ViewAndEditAccessRoles/types';
import { labelBasedResourceAccessFields } from '../role/listaccess';
import { AppIdentities, appIdentityFieldNames } from '../role/resources/app';
import { DbIdentities, dbIdentities } from '../role/resources/db';
import {
  DesktopIdentities,
  desktopIdentities,
} from '../role/resources/desktop';
import { KubeIdentities, kubeIdentities } from '../role/resources/kube';
import { ServerIdentities, serverIdentities } from '../role/resources/server';
import { IdentityStepButtons, IdentityTabContainer } from './Shared';
import { UpdateAccessRolesDialog } from './UpdateAccessRolesDialog';

export function IdentityTabsAndSection({
  role,
  dispatchRole,
  hasAccessGraphEnabled,
  accessRoleEditor,
}: {
  role: StandardEditorModel;
  dispatchRole: StandardModelDispatcher;
  hasAccessGraphEnabled: boolean;
  accessRoleEditor?: AccessRoleEditor;
}) {
  const { guideEditor } = useAccessListManagementContext();
  const {
    standardRoleState,
    awsIcRoleState,
    nextStep,
    prevStep,
    isEditing,
    emitEvent,
  } = guideEditor;

  const idPrefix = useId();

  const [currentTab, setCurrentTab] = useState(0);

  const [showUpdateDialog, setShowUpdateDialog] = useState(false);

  function handleOnChange(roleModelVal: ResourceAccess) {
    dispatchRole({
      type: ActionType.SetResourceAccess,
      payload: roleModelVal,
    });

    // Also update the root state.
    const resourceKind = roleModelVal.kind;
    switch (resourceKind) {
      case 'app': {
        const appIdentitiesToUpdate: AppIdentities = {
          aws_role_arns: standardRoleState.roleConditions.aws_role_arns,
          azure_identities: standardRoleState.roleConditions.azure_identities,
          gcp_service_accounts:
            standardRoleState.roleConditions.gcp_service_accounts,
          mcp: { tools: standardRoleState.roleConditions.mcp.tools },
        };
        appIdentityFieldNames.forEach(field => {
          if (!standardRoleState.requiredAppIdentities[field]) {
            return;
          }
          switch (field) {
            case 'aws_role_arns':
              appIdentitiesToUpdate.aws_role_arns = roleModelVal.awsRoleARNs;
              return;
            case 'azure_identities':
              appIdentitiesToUpdate.azure_identities =
                roleModelVal.azureIdentities;
              return;
            case 'gcp_service_accounts':
              appIdentitiesToUpdate.gcp_service_accounts =
                roleModelVal.gcpServiceAccounts;
              return;
            case 'mcp':
              appIdentitiesToUpdate.mcp = { tools: roleModelVal.mcpTools };
              return;
            default:
              field satisfies never;
          }
        });
        standardRoleState.updateIdentity(appIdentitiesToUpdate);
        break;
      }

      case 'db': {
        const dbIdentitiesToUpdate: DbIdentities = {
          db_names: standardRoleState.roleConditions.db_names,
          db_users: standardRoleState.roleConditions.db_users,
        };
        dbIdentities.forEach(field => {
          switch (field) {
            case 'db_names':
              dbIdentitiesToUpdate.db_names = roleModelVal.names.map(
                opt => opt.value
              );
              return;
            case 'db_users':
              dbIdentitiesToUpdate.db_users = roleModelVal.users.map(
                opt => opt.value
              );
              return;
            default:
              field satisfies never;
          }
        });
        standardRoleState.updateIdentity(dbIdentitiesToUpdate);

        break;
      }

      case 'kube_cluster': {
        const kubeIdentitiesToUpdate: KubeIdentities = {
          kubernetes_groups: standardRoleState.roleConditions.kubernetes_groups,
          kubernetes_users: standardRoleState.roleConditions.kubernetes_users,
          kubernetes_resources:
            standardRoleState.roleConditions.kubernetes_resources,
        };
        kubeIdentities.forEach(field => {
          switch (field) {
            case 'kubernetes_groups':
              kubeIdentitiesToUpdate.kubernetes_groups =
                roleModelVal.groups.map(opt => opt.value);
              return;
            case 'kubernetes_users':
              kubeIdentitiesToUpdate.kubernetes_users = roleModelVal.users.map(
                opt => opt.value
              );
              return;
            case 'kubernetes_resources':
              kubeIdentitiesToUpdate.kubernetes_resources =
                roleModelVal.resources.map(r => ({
                  kind: r.kind.value,
                  name: r.name,
                  namespace: r.namespace,
                  verbs: r.verbs.map(opt => opt.value),
                  api_group: r.apiGroup,
                }));
              return;
            default:
              field satisfies never;
          }
        });
        standardRoleState.updateIdentity(kubeIdentitiesToUpdate);
        break;
      }

      case 'node': {
        const serverIdentitiesToUpdate: ServerIdentities = {
          logins: standardRoleState.roleConditions.logins,
        };
        serverIdentities.forEach(field => {
          switch (field) {
            case 'logins':
              serverIdentitiesToUpdate.logins = roleModelVal.logins.map(
                opt => opt.value
              );
              return;
            default:
              field satisfies never;
          }
        });
        standardRoleState.updateIdentity(serverIdentitiesToUpdate);
        break;
      }

      case 'windows_desktop': {
        const desktopIdentitiesToUpdate: DesktopIdentities = {
          windows_desktop_logins:
            standardRoleState.roleConditions.windows_desktop_logins,
        };
        desktopIdentities.forEach(field => {
          switch (field) {
            case 'windows_desktop_logins':
              desktopIdentitiesToUpdate.windows_desktop_logins =
                roleModelVal.logins.map(opt => opt.value);
              return;
            default:
              field satisfies never;
          }
        });
        standardRoleState.updateIdentity(desktopIdentitiesToUpdate);
        break;
      }

      case 'git_server':
        // git server is edited in GitServerSection.tsx instead.
        break;

      default:
        resourceKind satisfies never;
    }
  }

  const tabSpecs = getResourceAccessTabSpecs({
    idPrefix,
    accessKind: 'identities',
    accessFields: labelBasedResourceAccessFields.filter(field => {
      const definedAccess = standardRoleState.definedAccess(field);
      if (definedAccess && field === 'app_labels') {
        return standardRoleState.hasRequiredAppIdentities();
      }
      return definedAccess;
    }),
  });

  const selectedTab = tabSpecs[currentTab];

  const section = resourceAccessSections[tabSpecs[currentTab]?.kind];

  const currentResourceIndex = role.roleModel.resources.findIndex(
    r => r.kind == selectedTab?.kind
  );

  const sectionValue = role.roleModel.resources[currentResourceIndex];
  const sectionValidation =
    role.validationResult.resources[currentResourceIndex];

  const hasNextTabSection = tabSpecs[currentTab + 1];

  const modifiedOriginalRole =
    isEditing &&
    (standardRoleState.roleEditState.isDirty ||
      awsIcRoleState.roleEditState.isDirty);

  let nextBtnText = hasNextTabSection
    ? `Next: ${hasNextTabSection.btnTitle}`
    : 'Next';
  if (!hasNextTabSection && isEditing) {
    nextBtnText = 'Save Changes';
  }

  function handleNext() {
    if (!hasNextTabSection) {
      if (isEditing) {
        setShowUpdateDialog(true);
        return;
      }
      if (standardRoleState.hasAnyIdentitiesDefined()) {
        emitEvent({
          stepStatus: AccessListStepStatusEvent.Success,
        });
      } else {
        emitEvent({
          stepStatus: AccessListStepStatusEvent.Skipped,
        });
      }

      nextStep();
    } else {
      setCurrentTab(currentTab + 1);
    }
  }

  function handlePrev() {
    const hasPrevSection = tabSpecs[currentTab - 1];
    if (!hasPrevSection) {
      prevStep();
    } else {
      setCurrentTab(currentTab - 1);
    }
  }

  const requiresIdentities = !standardRoleState.canSkipDefiningIdentities();

  return (
    <IdentityTabContainer>
      {requiresIdentities ? (
        <Box>
          <StickyTabs px={2} pt={2}>
            <SlideTabs
              appearance="round"
              size="medium"
              hideStatusIconOnActiveTab
              tabs={tabSpecs}
              activeIndex={currentTab}
              onChange={setCurrentTab}
            />
          </StickyTabs>
          <Box p={3} mt={1}>
            <Text bold fontSize={3} mb={3}>
              {selectedTab.sectionTitle}
            </Text>
            <section.component
              visibleInputFields={getRoleSectionInputFieldConfig({
                kind: sectionValue.kind,
                requiredAppIdentities: standardRoleState.requiredAppIdentities,
                withLabels: false,
              })}
              value={sectionValue}
              isProcessing={false}
              validation={sectionValidation}
              onChange={(val: ResourceAccess) => handleOnChange(val)}
            />
          </Box>
        </Box>
      ) : (
        <Box p={3} mt={1}>
          <Text mb={3}>No identities are required.</Text>
          {hasAccessGraphEnabled && (
            <Text>
              The access graph on the right shows how members&apos; access to
              resources can look like. To make any changes, go back.
            </Text>
          )}
        </Box>
      )}
      <IdentityStepButtons
        nextBtnText={nextBtnText}
        onNext={handleNext}
        onPrev={handlePrev}
        nextBtnDisabled={
          isEditing && !hasNextTabSection && !modifiedOriginalRole
        }
        nextBtnTooltipTxt={
          isEditing && !hasNextTabSection && !modifiedOriginalRole
            ? 'No changes made'
            : undefined
        }
      />
      {showUpdateDialog && accessRoleEditor && (
        <UpdateAccessRolesDialog
          onCancelUpdate={() => setShowUpdateDialog(false)}
          accessRoleEditor={accessRoleEditor}
        />
      )}
    </IdentityTabContainer>
  );
}

const StickyTabs = styled(Box)`
  position: sticky;
  top: 0;
  background-color: ${props => props.theme.colors.levels.elevated};
  z-index: 1;
`;
