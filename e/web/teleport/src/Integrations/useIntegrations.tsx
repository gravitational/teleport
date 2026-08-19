import { useEffect, useState } from 'react';

import useAttempt from 'shared/hooks/useAttemptNext';

import useTeleport from 'e-teleport/useTeleportE';
import cfg from 'teleport/config';
import { DeleteRequestOptions } from 'teleport/Integrations/Operations/IntegrationOperations';
import {
  EditableIntegrationFields,
  ExternalAuditStorageOpType,
  OperationType,
  useIntegrationOperation,
} from 'teleport/Integrations/Operations/useIntegrationOperation';
import {
  IntegrationKind,
  integrationService,
  IntegrationStatusCode,
  type ExternalAuditStorage,
  type ExternalAuditStorageIntegration,
  type Integration,
  type IntegrationListResponse,
  type Plugin,
} from 'teleport/services/integrations';

type OperationE = {
  type: OperationType;
  item?: Plugin | Integration | { name: ExternalAuditStorageOpType };
};

type GitHubIntegrationEditableFields = {
  kind: IntegrationKind.GitHub;
  secret: string;
};

export type EditableIntegrationFieldsE =
  | EditableIntegrationFields
  | GitHubIntegrationEditableFields;

export function useIntegrations() {
  const ctx = useTeleport();
  const integrationOps = useIntegrationOperation();
  const [items, setItems] = useState<
    (Plugin | Integration | ExternalAuditStorageIntegration)[]
  >([]);
  const { attempt, run, setAttempt } = useAttempt('processing');
  const { attempt: auditStorageAttempt, run: auditStorageRun } = useAttempt('');
  // warning is used when a user has permissions to list both the
  // "integration" and "plugin" resource, but when fetching
  // only one resolved. This lets the user know why the listing
  // may not be complete.
  const [warning, setWarning] = useState('');
  const [pluginOps, setPluginOps] = useState<OperationE>({
    type: 'none',
  });

  const [externalAuditStorageOps, setExternalAuditStorageOps] =
    useState<OperationE>({
      type: 'none',
    });

  useEffect(() => {
    // At least one of these access flag will be true since
    // either access will render the nav item that renders
    // the integrations screen. This means we will always
    // be fetching at least one resource.
    const hasPluginAccess = ctx.getFeatureFlags().plugins;
    const hasIntegrationAccess = ctx.getFeatureFlags().integrations;
    const hasExternalAuditStorageAccess =
      cfg.isCloud &&
      cfg.entitlements.ExternalAuditStorage.enabled &&
      ctx.getFeatureFlags().externalAuditStorage;

    // There can be two failure points:
    //   1) network error
    //   2) access error: user is missing a read access in one or more resources.
    // If all failed to fetch, we will render an error instead.
    setAttempt({ status: 'processing' });
    Promise.allSettled([
      hasPluginAccess ? ctx.pluginsService.fetchPlugins() : null,
      hasIntegrationAccess ? integrationService.fetchIntegrations(true) : null,
      hasExternalAuditStorageAccess
        ? ctx.externalAuditStorageService.getCluster()
        : null,
      hasExternalAuditStorageAccess
        ? ctx.externalAuditStorageService.getDraft()
        : null,
    ]).then(responses => {
      // TODO(lisa): handle paginating as a follow up polish.
      // Default fetch is 1k of integrations, which is plenty for beginning.
      // Currently only integration resource has pagination, check up on
      // plugins.
      const plugins = responses[0];
      const integrations = responses[1];
      const clusterExternalAuditStorage = responses[2];
      const draftExternalAuditStorage = responses[3];
      let fetchedItems = [];

      if (
        plugins.status === 'fulfilled' &&
        integrations.status === 'fulfilled' &&
        clusterExternalAuditStorage.status === 'fulfilled' &&
        draftExternalAuditStorage.status === 'fulfilled'
      ) {
        // Merge the responses into one

        // If the user doesn't have permission, instead of the request we make a null promise.
        // In that case, the promise will be fulfilled and the value is undefined.
        if (plugins.value) {
          fetchedItems.push(...plugins.value);
        }
        if (integrations.value) {
          fetchedItems.push(...integrations.value.items);
        }
        const audit = makeClusterExternalAuditStorageIntegration(
          clusterExternalAuditStorage?.value
        );
        if (audit) {
          fetchedItems.push(audit);
        }

        const draftAudit = makeDraftExternalAuditStorageIntegration(
          draftExternalAuditStorage?.value
        );
        if (draftAudit) {
          fetchedItems.push(draftAudit);
        }
      } else {
        const warning = getWarningMessage(
          plugins,
          integrations,
          clusterExternalAuditStorage
          // TODO draft
        );
        if (
          plugins.status === 'rejected' &&
          integrations.status === 'rejected' &&
          clusterExternalAuditStorage.status === 'rejected'
        ) {
          // all failed
          setAttempt({
            status: 'failed',
            statusText: warning,
          });
          return;
        } else {
          // some failed
          setWarning(warning);
          if (plugins.status === 'fulfilled' && plugins.value) {
            fetchedItems.push(...plugins.value);
          }
          if (integrations.status === 'fulfilled' && integrations.value) {
            fetchedItems.push(...integrations.value.items);
          }
          if (
            clusterExternalAuditStorage.status === 'fulfilled' &&
            clusterExternalAuditStorage.value
          ) {
            const audit = makeClusterExternalAuditStorageIntegration(
              clusterExternalAuditStorage.value
            );
            if (audit) {
              fetchedItems.push(audit);
            }
          }
          if (
            draftExternalAuditStorage.status === 'fulfilled' &&
            draftExternalAuditStorage.value
          ) {
            const draft = makeDraftExternalAuditStorageIntegration(
              draftExternalAuditStorage.value
            );
            if (draft) {
              fetchedItems.push(draft);
            }
          }
        }
      }
      if (fetchedItems) {
        setAttempt({ status: 'success' });
        setItems(fetchedItems);
      }
    });
  }, []);

  function onCancelDelete() {
    setPluginOps({ type: 'none' });
  }

  function onDelete(plugin: Plugin) {
    return ctx.pluginsService.deletePlugin(plugin.name).then(() => {
      const updatedItems = items.filter(
        i => i.resourceType === 'integration' || i.name !== plugin.name
      );
      setItems(updatedItems);
    });
  }

  function onStartDelete(plugin: Plugin) {
    setPluginOps({ type: 'delete', item: plugin });
  }

  function removeIntegration(opts: DeleteRequestOptions = {}) {
    return integrationOps.remove(opts).then(() => {
      const updatedItems = items.filter(
        i => i.resourceType === 'plugin' || i.name !== integrationOps.item.name
      );
      setItems(updatedItems);
      integrationOps.clear();
    });
  }

  async function editIntegration(req: EditableIntegrationFieldsE) {
    let updatedIntegration: Integration;
    if (req.kind === IntegrationKind.GitHub) {
      updatedIntegration =
        await integrationService.updateIntegrationOAuthSecret(
          integrationOps.item.name,
          req.secret
        );
    } else {
      updatedIntegration = await integrationOps.edit(req);
    }
    const updatedItems = items.map(item => {
      if (
        item.resourceType === 'integration' &&
        item.name == integrationOps.item.name
      ) {
        return updatedIntegration;
      }
      return item;
    });
    setItems(updatedItems);
    integrationOps.clear();
  }

  function onCancelDeleteExternalAuditStorage() {
    setExternalAuditStorageOps({ type: 'none' });
  }

  function onStartDeleteExternalAuditStorage(
    opType: ExternalAuditStorageOpType
  ) {
    setExternalAuditStorageOps({ type: 'delete', item: { name: opType } });
  }

  function onDeleteExternalAuditStorage() {
    if (externalAuditStorageOps.item.name === 'cluster') {
      return auditStorageRun(() =>
        ctx.externalAuditStorageService.deleteCluster().then(() => {
          setItems(
            items.filter(
              item =>
                item.kind !== IntegrationKind.ExternalAuditStorage ||
                item.statusCode === IntegrationStatusCode.Draft
            )
          );
        })
      );
    }
    return auditStorageRun(ctx.externalAuditStorageService.deleteDraft).then(
      () => {
        setItems(
          items.filter(
            item =>
              item.kind !== IntegrationKind.ExternalAuditStorage ||
              item.statusCode !== IntegrationStatusCode.Draft
          )
        );
      }
    );
  }

  const requiredCreatePermissions = [
    {
      value: ctx.storeUser.getPluginsAccess().create,
      label: 'plugin.create',
    },
    {
      value: ctx.storeUser.getIntegrationsAccess().create,
      label: 'integration.create',
    },
  ];

  return {
    items,
    attempt,
    run,
    pluginOps: {
      ...pluginOps,
      onCancelDelete,
      onDelete,
      onStartDelete,
    },
    integrationOps: {
      ...integrationOps,
      removeIntegration,
      editIntegration,
    },
    externalAuditStorageOps: {
      ...externalAuditStorageOps,
      onCancelDeleteExternalAuditStorage,
      onDeleteExternalAuditStorage,
      onStartDeleteExternalAuditStorage,
    },
    auditStorageAttempt,
    warning,
    requiredCreatePermissions,
  };
}

export type State = ReturnType<typeof useIntegrations>;

export function getWarningMessage(
  plugins: PromiseSettledResult<Plugin[]>,
  integrations: PromiseSettledResult<IntegrationListResponse>,
  externalAuditStorage: PromiseSettledResult<ExternalAuditStorage>
): string {
  if (
    plugins.status === 'fulfilled' &&
    integrations.status === 'fulfilled' &&
    externalAuditStorage.status === 'fulfilled'
  ) {
    return '';
  }

  if (
    plugins.status === 'rejected' &&
    integrations.status === 'rejected' &&
    externalAuditStorage.status === 'rejected'
  ) {
    const pluginsErr = plugins.reason;
    const integegrationsErr = integrations.reason;
    const externalAuditStorageErr = externalAuditStorage.reason;
    return `An error has occurred. PLUGINS: ${pluginsErr}, INTEGRATIONS: ${integegrationsErr}, EXTERNAL AUDIT: ${externalAuditStorageErr}`;
  } else {
    let warning = 'Failed to fetch ';
    let helpMsg = 'try refreshing browser or check your ';
    let errMsg = '';
    let errAmount = 0;

    if (externalAuditStorage.status === 'rejected') {
      warning += `external audit integration`;
      helpMsg += `"external_audit_storage"`;
      errMsg += externalAuditStorage.reason;
      errAmount += 1;
    }

    if (plugins.status === 'rejected') {
      let and = errAmount > 0 ? ' and ' : '';
      warning += `${and}plugin integrations`;
      helpMsg += `${and}"plugin"`;
      errMsg += `${and}${plugins.reason}`;
      errAmount += 1;
    }

    if (integrations.status === 'rejected') {
      let and = errAmount > 0 ? ' and ' : '';
      warning += `${and}rest of integrations`;
      helpMsg += `${and}"integration"`;
      errMsg += `${and}${integrations.reason}`;
      errAmount += 1;
    }

    helpMsg += '';

    return `${warning} (${helpMsg} access): ${errMsg}`;
  }
}

function makeExternalAuditStorageIntegration(
  externalAuditStorage: ExternalAuditStorage | null,
  isDraft: boolean,
  details: string
): ExternalAuditStorageIntegration | null {
  if (!externalAuditStorage) {
    return null;
  }
  return {
    kind: IntegrationKind.ExternalAuditStorage,
    resourceType: 'external-audit-storage',
    name: `External Audit Storage${isDraft ? ' (Draft) ' : ''}`,
    statusCode: isDraft
      ? IntegrationStatusCode.Draft
      : IntegrationStatusCode.Running,
    spec: externalAuditStorage,
    details,
  };
}

function makeClusterExternalAuditStorageIntegration(
  externalAuditStorage: ExternalAuditStorage | null
): ExternalAuditStorageIntegration | null {
  return makeExternalAuditStorageIntegration(
    externalAuditStorage,
    false,
    `Audit Log Events and Session Recordings Storage in AWS Integration ${externalAuditStorage?.integrationName}`
  );
}

function makeDraftExternalAuditStorageIntegration(
  externalAuditStorage: ExternalAuditStorage | null
): ExternalAuditStorageIntegration | null {
  return makeExternalAuditStorageIntegration(
    externalAuditStorage,
    true,
    `In-progress configuration for Audit Log and Session Recording storage with AWS Integration ${externalAuditStorage?.integrationName}`
  );
}
