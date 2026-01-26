import { useId, useState } from 'react';

import { Box, Text } from 'design';
import {
  Application,
  Database,
  Kubernetes,
  Server,
  Windows,
} from 'design/Icon';
import { SlideTabs } from 'design/SlideTabs';
import { TabSpec } from 'design/SlideTabs/SlideTabs';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import { resourceAccessSections } from 'teleport/Roles/RoleEditor/StandardEditor/Resources';
import {
  AppAccessInputFields,
  DatabaseAccessInputFields,
  GitHubOrganizationAccessInputFields,
  KubernetesAccessInputFields,
  ResourceAccess,
  ResourceAccessKind,
  ServerAccessInputFields,
  StandardEditorModel,
  WindowsDesktopAccessInputFields,
} from 'teleport/Roles/RoleEditor/StandardEditor/standardmodel';
import {
  ActionType,
  StandardModelDispatcher,
} from 'teleport/Roles/RoleEditor/StandardEditor/useStandardModel';

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

export function IdentityTabsAndSection({
  role,
  dispatchRole,
}: {
  role: StandardEditorModel;
  dispatchRole: StandardModelDispatcher;
}) {
  const { guideEditor } = useAccessListManagementContext();
  const { standardRoleState, nextStep, prevStep } = guideEditor;

  const idPrefix = useId();

  const [currentTab, setCurrentTab] = useState(0);

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

  function makeRoleSectionTabs(): (TabSpec & {
    kind: ResourceAccessKind;
    btnTitle: string;
    sectionTitle: string;
  })[] {
    const identityTxt = 'Identities';
    return labelBasedResourceAccessFields
      .filter(field => standardRoleState.definedAccess(field))
      .map(field => {
        switch (field) {
          case 'app_labels': {
            const sectionTitle = `Application ${identityTxt}`;
            return {
              key: 'app',
              kind: 'app',
              ariaLabel: 'Go to Application tab',
              btnTitle: 'Application',
              icon: Application,
              controls: `${idPrefix}-app`,
              sectionTitle,
              tooltip: {
                content: sectionTitle,
              },
            };
          }
          case 'db_labels': {
            const sectionTitle = `Database ${identityTxt}`;
            return {
              key: 'db',
              ariaLabel: 'Go to Database tab',
              kind: 'db',
              btnTitle: 'Database',
              icon: Database,
              controls: `${idPrefix}-db`,
              sectionTitle,
              tooltip: {
                content: sectionTitle,
              },
            };
          }
          case 'kubernetes_labels': {
            const sectionTitle = `Kubernetes ${identityTxt}`;
            return {
              key: 'kube_cluster',
              ariaLabel: `"Go to Kubernetes Cluster tab"`,
              kind: 'kube_cluster',
              btnTitle: 'Kubernetes',
              icon: Kubernetes,
              controls: `${idPrefix}-kube_cluster`,
              sectionTitle,
              tooltip: {
                content: sectionTitle,
              },
            };
          }
          case 'node_labels': {
            const sectionTitle = `Server ${identityTxt}`;
            return {
              key: 'node',
              ariaLabel: 'Go to Server tab',
              kind: 'node',
              icon: Server,
              btnTitle: 'Server',
              controls: `${idPrefix}-node`,
              sectionTitle,
              tooltip: {
                content: sectionTitle,
              },
            };
          }
          case 'windows_desktop_labels': {
            const sectionTitle = `Windows Desktop ${identityTxt}`;
            return {
              key: 'windows_desktop',
              ariaLabel: 'Go to Windows Desktop tab',
              kind: 'windows_desktop',
              icon: Windows,
              btnTitle: 'Desktops',
              controls: `${idPrefix}-windows_desktop`,
              sectionTitle,
              tooltip: {
                content: sectionTitle,
              },
            };
          }
          default:
            field satisfies never;
        }
      });
  }

  function getRoleSectionInputFieldConfig(kind: ResourceAccessKind) {
    const requiredAppIdentities = standardRoleState.requiredAppIdentities;
    switch (kind) {
      case 'app': {
        const show: AppAccessInputFields = {
          labels: false,
          awsRoleARNs: !!requiredAppIdentities.aws_role_arns,
          azureIdentities: !!requiredAppIdentities.azure_identities,
          gcpServiceAccounts: !!requiredAppIdentities.gcp_service_accounts,
          mcpTools: !!requiredAppIdentities.mcp?.tools,
        };
        return show;
      }

      case 'db': {
        const show: DatabaseAccessInputFields = {
          labels: false,
          dbServiceLabels: false,
          roles: false,
          names: true,
          users: true,
        };
        return show;
      }

      case 'kube_cluster': {
        const show: KubernetesAccessInputFields = {
          labels: false,
          resources: true,
          users: true,
          groups: true,
        };
        return show;
      }
      case 'node': {
        const show: ServerAccessInputFields = {
          labels: false,
          logins: true,
        };
        return show;
      }
      case 'windows_desktop': {
        const show: WindowsDesktopAccessInputFields = {
          labels: false,
          logins: true,
        };
        return show;
      }
      case 'git_server':
        const show: GitHubOrganizationAccessInputFields = {
          organizations: true,
        };
        return show;
      default:
        kind satisfies never;
    }
  }

  const tabSpecs = makeRoleSectionTabs();

  const selectedTab = tabSpecs[currentTab];

  const { component: Section } =
    resourceAccessSections[tabSpecs[currentTab].kind];

  const currentResourceIndex = role.roleModel.resources.findIndex(
    r => r.kind == selectedTab.kind
  );

  const sectionValue = role.roleModel.resources[currentResourceIndex];
  const sectionValidation =
    role.validationResult.resources[currentResourceIndex];

  const hasNextTabSection = tabSpecs[currentTab + 1];
  const nextBtnTxt = hasNextTabSection
    ? `Next: ${hasNextTabSection.btnTitle}`
    : 'Next';

  function handleNext() {
    if (!hasNextTabSection) {
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

  return (
    <IdentityTabContainer>
      <Box>
        <SlideTabs
          appearance="round"
          size="medium"
          hideStatusIconOnActiveTab
          tabs={tabSpecs}
          activeIndex={currentTab}
          onChange={setCurrentTab}
        />
        <Box p={2} mt={1}>
          <Text bold fontSize={3} mb={3}>
            {selectedTab.sectionTitle}
          </Text>
          <Section
            visibleInputFields={getRoleSectionInputFieldConfig(
              sectionValue.kind
            )}
            value={sectionValue}
            isProcessing={false}
            validation={sectionValidation}
            onChange={(val: ResourceAccess) => handleOnChange(val)}
          />
        </Box>
      </Box>
      <IdentityStepButtons
        nextBtnText={nextBtnTxt}
        onNext={handleNext}
        onPrev={handlePrev}
      />
    </IdentityTabContainer>
  );
}
