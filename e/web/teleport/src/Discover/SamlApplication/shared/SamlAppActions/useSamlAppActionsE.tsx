import { useMemo, useState, useCallback } from 'react';
import { ResourceSpec } from 'teleport/Discover/SelectResource';
import useTeleport from 'teleport/useTeleport';
import { useAsync, makeEmptyAttempt } from 'shared/hooks/useAsync';
import { SamlMeta } from 'teleport/Discover/useDiscover';
import {
  SamlIdpServiceProvider,
  SamlServiceProviderPreset,
} from 'teleport/services/samlidp/types';
import { SamlAppActionContext } from 'teleport/SamlApplications/useSamlAppActions';

import useTeleportE from 'e-teleport/useTeleportE';

import type {
  SamlAppAction,
  SamlAppActionMode,
} from 'teleport/SamlApplications/useSamlAppActions';
import type { SamlAppToDelete } from 'teleport/services/samlidp/types';

export function SamlAppActionProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const ctx = useTeleport();
  const userSamlIdPPerm = ctx.storeUser.getSamlIdPServiceProviderAccess();

  const [currentAction, setCurrentAction] = useState<SamlAppActionMode>(null);
  const [resourceSpec, setResourceSpec] = useState<ResourceSpec>(null);

  const { idpService } = useTeleportE();
  const [fetchSamlResourceAttempt, fetchSamlResource] = useAsync(
    async (name: string) => {
      const resp = await idpService.getSamlIdpServiceProvider(name);
      return samlResponseToAgentMeta(resp);
    }
  );

  // samlAppToDelete holds deletion state to remove
  // saml app item from the unified resources view.
  const [samlAppToDelete, SetSamlAppToDelete] = useState<SamlAppToDelete>();

  const [deleteSamlAppAttempt, deleteSamlApp, setDeleteSamlAppAttempt] =
    useAsync(async (name: string) => {
      await idpService.deleteSamlIdpServiceProvider(name);
    });

  const onDelete = async () => {
    const [, err] = await deleteSamlApp(resourceSpec.name);
    if (err) {
      return;
    }
    SetSamlAppToDelete({
      name: resourceSpec.name,
      backendDeleted: true,
    });
    setCurrentAction(null);
  };

  const actions = useMemo(
    () => ({
      startEdit: (resourceSpec: ResourceSpec) => {
        if (!userSamlIdPPerm.edit) {
          return;
        }
        setCurrentAction('edit');
        fetchSamlResource(resourceSpec.name);
        setResourceSpec(resourceSpec);
      },
      startDelete: (resourceSpec: ResourceSpec) => {
        if (!userSamlIdPPerm.remove) {
          return;
        }
        setResourceSpec(resourceSpec);
        setCurrentAction('delete');
      },
      showActions: true,
    }),
    [userSamlIdPPerm.edit, userSamlIdPPerm.remove]
  );

  const clearAction = useCallback(() => {
    setCurrentAction(null);
    setResourceSpec(null);
    setDeleteSamlAppAttempt(makeEmptyAttempt);
  }, []);

  const value: SamlAppAction = {
    actions,
    currentAction,
    fetchSamlResourceAttempt,
    resourceSpec,
    onDelete,
    deleteSamlAppAttempt,
    samlAppToDelete,
    clearAction,
    userSamlIdPPerm,
  };

  return (
    <SamlAppActionContext.Provider value={value}>
      {children}
    </SamlAppActionContext.Provider>
  );
}

function samlResponseToAgentMeta(resp: SamlIdpServiceProvider) {
  let samlMeta: SamlMeta = {
    samlGeneric: resp,
  };
  if (resp.spec.preset === SamlServiceProviderPreset.GcpWorkforce) {
    samlMeta.samlGcpWorkforce = {
      isAutoConfig: true,
      orgId: '' /* we do not store the organization Id */,
      poolName: poolNameFromEntityId(resp.spec.entity_id),
      poolProviderName: resp.metadata.name,
    };
  }
  return samlMeta;
}

function poolNameFromEntityId(entityId: string): string {
  try {
    // Expected format of the entityId:
    // https://iam.googleapis.com/locations/global/workforcePools/pool_name/providers/pool_provider_name
    return new URL(entityId).pathname.split('/')[4];
  } catch {
    // While a URL is expected, user may have misconfigured
    // the field so we'll just swallow an error here and return
    // an empty poolName string.
    return '';
  }
}
