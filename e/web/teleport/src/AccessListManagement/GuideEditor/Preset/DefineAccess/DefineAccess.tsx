import { JSX, useCallback, useState } from 'react';
import styled from 'styled-components';

import { Box, Flex, H1 } from 'design';
import {
  UnifiedResourcesQueryParams,
  useUnifiedResourcesFetch,
} from 'shared/components/UnifiedResources';
import Validation from 'shared/components/Validation';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import { URLResourceFilter } from 'teleport/components/hooks/useUrlFiltering/useUrlFiltering';
import cfg from 'teleport/config';
import { App } from 'teleport/services/apps';
import ResourceService from 'teleport/services/resources';

import { GuideContent, StepButtons } from '../../Shared';
import { StandardRoleConditions } from '../role/conditions';
import {
  definableResourceAccessFields,
  DefinableResourceAccessFields,
} from '../role/listaccess';
import { getGitHubOrgs } from '../role/resources/github';
import { wildcard } from '../role/role';
import { AwsIcSection } from './AwsIc/AwsIcSection';
import { GitServerSection } from './GitServerSection';
import { ListResourceSection } from './ListResourceSection/ListResourceSection';
import { NoAccessDefinedDialog } from './NoAccessDefinedDialog';
import { PreviewingResourceAlert } from './PreviewingResourceAlert';
import { RemoveAccessDialog } from './RemoveAccessDialog';
import { ResourceTabs } from './ResourceTabs';
import {
  getPredicateExpression,
  getUnifiedResourceKind,
} from './unifiedResource';

/**
 * Allows the user to define access to resources:
 *  - By typing or clicking on resource labels for resources controlled
 *    by labels e.g. app, db, kube, server, windows. As user defines labels
 *    the listing of resources will update with the selected label filter
 *    applied.
 *  - By selecting from a list of available git_servers from the cluster.
 *    Git server's are not controlled by labels but by GitHub org name.
 *  - By selecting from a list of available AWS IC applications and then
 *    selecting shared permission sets between the selected apps.
 *
 * The resources that users can "preview/list" is based on the their current
 * RBAC. If by chance the current user has limited RBAC, they can only define
 * access to limited resources. The preview then may also not be an accurate
 * depiction. There may be more resources listed for the end user then the
 * user defining access can see (e.g. if they choose to use wildcard). The
 * user is made aware of this by rendering an alert explaining this constraint.
 */
export function DefineAccess() {
  const { guideEditor } = useAccessListManagementContext();
  const {
    nextStep,
    definedAccessInAnyRoleCondition,
    currentStep,
    definedAccess,
    preset,
    standardRoleState,
    awsIcRoleState,
    isEditing,
  } = guideEditor;

  // Defaults tab selection to the first access type.
  const [selectedResourceTab, setSelectedResourceTab] =
    useState<DefinableResourceAccessFields>(
      () => definableResourceAccessFields[0]
    );

  /**
   * "no-access-defined": Gives feedback to user that no accesses were defined
   * but can be defined later.
   *
   * "remove-access": Prevent user from going next or going to a new resource
   * tab if a user has defined some access but no resources resulted from the
   * query. This prevents adding random access and gives feedback to user
   * that there were no results.
   */
  const [activeDialog, setActiveDialog] = useState<
    'no-access-defined' | 'remove-access' | ''
  >('');

  const [resourceFilters, setResourceFilters] = useState<URLResourceFilter>(
    () =>
      initResourceFilters(selectedResourceTab, standardRoleState.roleConditions)
  );

  const markRequiredAppIdentities = useCallback(
    (fetchedApps: App[], startKey: string) => {
      if (!standardRoleState.definedAccess('app_labels')) {
        standardRoleState.clearRequiredAppIdentities();
        return;
      }

      if (!fetchedApps.length) {
        standardRoleState.clearRequiredAppIdentities();
        return;
      }

      standardRoleState.markRequiredAppIdentities(
        fetchedApps,
        !startKey /* all pages fetched */
      );
    },
    [standardRoleState]
  );

  const fetchedResources = useUnifiedResourcesFetch({
    fetchFunc: useCallback(
      async (paginationParams, signal) => {
        const resourceService = new ResourceService();
        const results = await resourceService.fetchUnifiedResources(
          cfg.proxyCluster,
          {
            search: resourceFilters.search,
            query: resourceFilters.query,
            sort: resourceFilters.sort,
            kinds: resourceFilters.kinds,
            limit: paginationParams.limit,
            startKey: paginationParams.startKey,
          },
          signal
        );

        if (selectedResourceTab === 'app_labels') {
          markRequiredAppIdentities(results.agents as App[], results.startKey);
        }

        return results;
      },
      [resourceFilters, selectedResourceTab, markRequiredAppIdentities]
    ),
  });

  // This state is used to recognize when the `filter` value has changed,
  // and reset the overall state of `useUnifiedResourcesFetch` hook.
  const [prevFilters, setPrevFilters] = useState(resourceFilters);
  if (prevFilters !== resourceFilters) {
    setPrevFilters(resourceFilters);
    fetchedResources.clear();
  }

  // When navigating to a different resource tab, trigger the reset of the
  // useUnifiedResourcesFetch hook to query for the correct resource.
  const [prevSelectedResourceTab, setPrevSelectedResourceTab] =
    useState(selectedResourceTab);
  if (prevSelectedResourceTab !== selectedResourceTab) {
    setPrevSelectedResourceTab(selectedResourceTab);
    setResourceFilters(
      initResourceFilters(selectedResourceTab, standardRoleState.roleConditions)
    );
  }

  function hasResourceResults() {
    if (definedAccess(selectedResourceTab)) {
      if (selectedResourceTab === 'awsIc') {
        return awsIcRoleState.fetchedApps.data?.list.length > 0;
      }
      if (selectedResourceTab === 'github_permissions') {
        const orgs = getGitHubOrgs(
          standardRoleState.roleConditions.github_permissions
        );
        if (!orgs.includes(wildcard)) {
          return orgs.length > 0;
        }
      }
      return fetchedResources.resources.length > 0;
    }
    return true;
  }

  function handleNextStep() {
    // Block going next until user removes the access.
    if (!hasResourceResults()) {
      setActiveDialog('remove-access');
      return;
    }
    if (!definedAccessInAnyRoleCondition()) {
      setActiveDialog('no-access-defined');
      return;
    }

    nextStep();
  }

  function handleSelectTab(accessField: DefinableResourceAccessFields) {
    if (!hasResourceResults()) {
      // Block going to selected tab until user removes the access.
      setActiveDialog('remove-access');
      return;
    }
    setSelectedResourceTab(accessField);
  }

  function updateResourceFilters(newFilters: UnifiedResourcesQueryParams) {
    setResourceFilters({ ...resourceFilters, ...newFilters });
  }

  let defineAccessContent: JSX.Element;

  switch (selectedResourceTab) {
    case 'awsIc': {
      defineAccessContent = <AwsIcSection />;
      break;
    }

    case 'app_labels':
    case 'db_labels':
    case 'kubernetes_labels':
    case 'node_labels':
    case 'windows_desktop_labels': {
      defineAccessContent = (
        <ListResourceSection
          key={selectedResourceTab}
          selectedResourceTab={selectedResourceTab}
          resourceFilters={resourceFilters}
          updateResourceFilters={updateResourceFilters}
          fetchedResources={fetchedResources}
        />
      );
      break;
    }

    case 'github_permissions': {
      defineAccessContent = (
        <GitServerSection updateResourceFilters={updateResourceFilters} />
      );

      const selectedWildcard = getGitHubOrgs(
        standardRoleState.roleConditions.github_permissions
      ).includes(wildcard);

      if (selectedWildcard) {
        // Unified resource table is not necessary since
        // it's just repeat info (there is nothing for the user
        // to interact with the list since git_server access
        // is not controlled by labels).
        //
        // But if user selected a wildcard we render the table to
        // scale better in terms of paging and better visibility of
        // all available servers in the cluster.
        defineAccessContent = (
          <>
            {defineAccessContent}
            <ListResourceSection
              key={selectedResourceTab}
              selectedResourceTab={selectedResourceTab}
              resourceFilters={resourceFilters}
              updateResourceFilters={updateResourceFilters}
              fetchedResources={fetchedResources}
            />
          </>
        );
      }
      break;
    }

    default:
      selectedResourceTab satisfies never;
  }

  return (
    <Box>
      <Validation>
        <GuideContent>
          <H1 mb={1}>
            Step {currentStep + 1}:{' '}
            {preset === 'long-term'
              ? 'What resources will the members have long-term access to?'
              : 'What resources can members request access to?'}
          </H1>

          <PreviewingResourceAlert />

          <Flex
            mb={4}
            css={`
              --guide-section-height: 70vh;
            `}
          >
            <ResourceTabs
              selectedTab={selectedResourceTab}
              onTabSelect={handleSelectTab}
            />
            <DefineAccessContainer>{defineAccessContent}</DefineAccessContainer>
          </Flex>
        </GuideContent>

        <StepButtons
          onNext={handleNextStep}
          disableNext={fetchedResources.attempt.status === 'processing'}
          // When entering a editing mode (versus creating mode), the editor
          // opens up like a full screen dialog. Prev button is hidden
          // since this "dialog" like editor can be exited with the classical
          // upper left "x" button.
          hidePrevBtn={isEditing}
        />
      </Validation>
      {activeDialog === 'no-access-defined' && (
        <NoAccessDefinedDialog
          onCancel={() => setActiveDialog('')}
          onNext={() => nextStep()}
          isEditing={isEditing}
        />
      )}
      {activeDialog === 'remove-access' && (
        <RemoveAccessDialog
          field={selectedResourceTab}
          onClose={() => setActiveDialog('')}
        />
      )}
    </Box>
  );
}

function initResourceFilters(
  accessField: DefinableResourceAccessFields,
  roleConditions: StandardRoleConditions
): URLResourceFilter {
  let query = '';
  switch (accessField) {
    case 'awsIc':
      query = getPredicateExpression({ accessField: 'awsIc' });
      break;
    case 'app_labels':
    case 'db_labels':
    case 'github_permissions':
    case 'kubernetes_labels':
    case 'node_labels':
    case 'windows_desktop_labels':
      query = getPredicateExpression({ accessField, roleConditions });
      break;
    default:
      accessField satisfies never;
  }

  return {
    query,
    kinds: [getUnifiedResourceKind(accessField)],
    sort: {
      fieldName: 'name',
      dir: 'ASC',
    },
  };
}

const DefineAccessContainer = styled(Box)`
  width: 100%;
  padding: ${p => p.theme.space[3]}px;
  padding-left: ${p => p.theme.space[4]}px;
  padding-top: ${p => p.theme.space[2]}px;
  border: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[0]};
  border-top-right-radius: ${p => p.theme.radii[2]}px;
  border-bottom-right-radius: ${p => p.theme.radii[2]}px;
  box-shadow: -3px 2px 5px 2px #00000024;
  overflow: scroll;
  height: var(--guide-section-height);
`;
