import { useEffect } from 'react';

import { Box, H1, Indicator } from 'design';
import Validation from 'shared/components/Validation';
import useAttempt from 'shared/hooks/useAttemptNext';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import cfg from 'teleport/config';
import useTeleport from 'teleport/useTeleport';

import { AccessRoleEditor } from '../../ViewAndEditAccessRoles/types';
import { getPredicateExpression } from '../DefineAccess/unifiedResource';
import { AppIdentities, appIdentityFieldNames } from '../role/resources/app';
import { IdentityView } from './IdentityView';

/**
 * This component allows users to define identities for the resources
 * they defined access to from the previous step.
 *
 * Resource identities (or principals) allow a user to connect to the
 * resources ((e.g. db_names, db_users) they can list (e.g. db_labels).
 *
 * If access for apps are defined, each applicable app subkind will be
 * queried to see if that subkind identity is required. This provides better
 * UX in that it does not render unnecessary application identities input
 * fields.
 */
export function DefineIdentities({
  accessRoleEditor,
}: {
  /**
   * Only defined if user is "updating" existing access roles
   * when viewing an access list. Will be undefined if user is
   * using this step as part of "creating" an access list.
   */
  accessRoleEditor?: AccessRoleEditor;
}) {
  const { guideEditor } = useAccessListManagementContext();
  const { standardRoleState, definedAccess } = guideEditor;

  const teleCtx = useTeleport();
  const {
    attempt: queryAppAttempt,
    setAttempt: setQueryAppAttempt,
    run,
  } = useAttempt('processing');

  async function appExists(appKindQuery: string) {
    const accessQuery = getPredicateExpression({
      accessField: 'app_labels',
      roleConditions: standardRoleState.roleConditions,
    });
    const response = await teleCtx.resourceService.fetchUnifiedResources(
      cfg.proxyCluster,
      {
        query: `(${accessQuery}) && ${appKindQuery}`,
        kinds: ['app'],
        limit: 1,
      }
    );
    return response.agents?.length > 0;
  }

  async function queryForRequiredApplicationIdentities() {
    const requiredApps = standardRoleState.requiredAppIdentities;

    const queries: {
      field: keyof AppIdentities;
      query: string;
    }[] = [];

    for (const field of appIdentityFieldNames) {
      if (
        (field === 'mcp' && requiredApps.mcp?.tools != null) ||
        (field !== 'mcp' && requiredApps[field] != null)
      ) {
        // Means it was determined from the previous step.
        continue;
      }

      switch (field) {
        case 'aws_role_arns':
          queries.push({ field, query: `resource.spec.cloud == "AWS"` });
          break;
        case 'azure_identities':
          queries.push({ field, query: `resource.spec.cloud == "Azure"` });
          break;
        case 'gcp_service_accounts':
          queries.push({ field, query: `resource.spec.cloud == "GCP"` });
          break;
        case 'mcp':
          queries.push({ field, query: `resource.sub_kind == "mcp"` });
          break;
        default:
          field satisfies never;
      }
    }

    const results = await Promise.allSettled(
      queries.map(async ({ field, query }) => {
        const exists = await appExists(query);
        return { field, exists };
      })
    );

    // Process results and collect fields that should be marked as required.
    const fieldsToMarkRequired: (keyof AppIdentities)[] = [];

    for (let i = 0; i < results.length; i++) {
      const result = results[i];
      if (result.status === 'fulfilled') {
        const { field, exists } = result.value;
        if (exists) {
          fieldsToMarkRequired.push(field);
        }
      } else {
        // Ignore error, but mark identity as required to be safe.
        const query = queries[i];
        if (query) {
          fieldsToMarkRequired.push(query.field);
        }
      }
    }

    standardRoleState.markAppIdentityFieldsAsRequired(
      fieldsToMarkRequired,
      true /* nothing more to fetch */
    );
  }

  useEffect(() => {
    // If this block is true, there is no need to
    // query for app resources to determine which
    // identities are required of the user.
    if (
      !definedAccess('app_labels') ||
      !standardRoleState.requiredAppIdentities ||
      standardRoleState.requiredAppIdentities.allPagesFetched ||
      Object.keys(standardRoleState.requiredAppIdentities).every(
        p => standardRoleState.requiredAppIdentities[p] != null
      )
    ) {
      setQueryAppAttempt({ status: 'success' });
      return;
    }

    run(() => queryForRequiredApplicationIdentities());
  }, []);

  return (
    <Box>
      <Validation>
        <H1 mb={3}>
          Step {guideEditor.currentStep + 1}: Define what identities users can
          assume when connecting to resources
        </H1>

        {queryAppAttempt.status === 'processing' && (
          <Box textAlign="center" m={10}>
            <Indicator />
          </Box>
        )}

        {queryAppAttempt.status === 'success' && (
          <IdentityView accessRoleEditor={accessRoleEditor} />
        )}
      </Validation>
    </Box>
  );
}
