import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useCallback, useState } from 'react';
import { useNavigate } from 'react-router';
import styled from 'styled-components';

import {
  Alert,
  Box,
  ButtonPrimary,
  ButtonSecondary,
  ButtonWarning,
  Flex,
  Text,
} from 'design';
import Dialog, {
  DialogContent,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import { Trash } from 'design/Icon';
import { getErrMessage } from 'shared/utils/errorType';

import { OktaIntegrationStepType } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Shared';
import { AppGroupSyncForm } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/SetUpAppGroupSync';
import { SetupIdentitySecuritySyncForm } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/SetupIdentitySecuritySync';
import { ScimForm } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/SetUpScim';
import { UserSyncForm } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/SetUpUserSync';
import { IdentitySecuritySyncDetails } from 'e-teleport/Integrations/IntegrationStatus/OktaStatusDetails/IdentitySecuritySyncDetails';
import {
  oktaPluginUpdate,
  pluginsService,
  PluginUpdateRequest,
} from 'e-teleport/services/plugins';
import { createFetchPluginQueryKey } from 'e-teleport/services/plugins/hooks';
import { useTeleport } from 'teleport';
import { Route, Switch } from 'teleport/components/Router';
import cfg from 'teleport/config';
import { Plugin, PluginOktaSpec } from 'teleport/services/integrations';
import { PluginStatusOkta } from 'teleport/services/integrations/oktaStatusTypes';
import { storageService } from 'teleport/services/storageService';
import { withUnsupportedOktaPluginUpdateErrorConversion } from 'teleport/services/version/unsupported';

import { AppGroupSyncDetails } from './AppGroupSyncDetails';
import { ScimDetails } from './ScimDetails';
import { SetupAccessCta } from './SetupAccessCta';
import { FlexWrap } from './Shared';
import { SsoDetails } from './SsoDetails';
import { UserSyncDetails } from './UserSyncDetails';

type UpdateState = 'enable' | 'disable';

enum UpdateSetting {
  SCIM = 'SCIM',
  UserSync = 'UserSync',
  // App/Group Sync is not used, as the web UI should only ever toggle
  // it alongside AccessListSync. It's just included here for completeness.
  AppGroupSync = 'AppGroupSync',
  AccessListSync = 'AccessListSync',
  IdentitySecuritySync = 'IdentitySecuritySync',
}

type UpdateType = `${UpdateState}${UpdateSetting}`;

const handleUpdateType = (
  updateType: UpdateType,
  opts: NonNullable<PluginUpdateRequest['okta']>
): NonNullable<PluginUpdateRequest['okta']> => {
  console.log(opts, updateType);
  switch (updateType) {
    case 'enableUserSync':
      opts.enableUserSync = true;
      break;
    case 'enableIdentitySecuritySync':
      opts.enableSystemLogExport = true;
      break;
    case 'disableIdentitySecuritySync':
      opts.enableSystemLogExport = false;
      break;
    case 'disableUserSync':
      opts.enableAccessListSync = false;
      opts.enableAppGroupSync = false;
      opts.enableUserSync = false;
      break;
    case 'enableAccessListSync':
    case 'enableAppGroupSync':
      opts.enableAccessListSync = true;
      opts.enableAppGroupSync = true;
      opts.enableUserSync = true;
      break;
    case 'disableAccessListSync':
    case 'disableAppGroupSync':
      opts.enableAccessListSync = false;
      opts.enableAppGroupSync = false;
      break;
  }
  return opts;
};

type ConfirmModalState = {
  updateState: UpdateState;
  updateSetting:
    | UpdateSetting.AccessListSync
    | UpdateSetting.UserSync
    | UpdateSetting.IdentitySecuritySync;
  onConfirm: () => void | Promise<void>;
};

function updateSettingToTitle(updateSetting: UpdateSetting) {
  switch (updateSetting) {
    case UpdateSetting.UserSync:
      return 'User Sync';
    case UpdateSetting.AccessListSync:
      return 'App and Group Sync';
    case UpdateSetting.IdentitySecuritySync:
      return 'Teleport Identity Security Sync';
  }
}

function updateSettingToText(
  updateState: string,
  updateSetting: UpdateSetting,
  suffix: string | undefined = ''
) {
  if (updateState === 'enable') {
    switch (updateSetting) {
      case UpdateSetting.AccessListSync:
        return (
          'Enabling App and Group Sync requires that User Sync be enabled. ' +
          suffix
        );
      case UpdateSetting.IdentitySecuritySync:
        return (
          'Enabling Teleport Identity Security Sync requires that User Sync be enabled. ' +
          suffix
        );
    }
  }

  if (updateState === 'disable') {
    switch (updateSetting) {
      case UpdateSetting.UserSync:
        return 'This will also disable App and Group Sync. ' + suffix;
    }
  }
}

const ConfirmationModal = ({
  modal,
  setModal,
  disabled,
}: {
  modal: ConfirmModalState;
  setModal: (state: ConfirmModalState | undefined) => void;
  disabled: boolean;
}) => {
  const action = modal.updateState === 'disable' ? 'Disable' : 'Enable';
  const setting = updateSettingToTitle(modal.updateSetting);
  const text = updateSettingToText(
    modal.updateState,
    modal.updateSetting,
    modal.updateState === 'disable'
      ? `Your current configuration will be saved, and ${setting} may be re-enabled later.`
      : undefined
  );

  return (
    <Dialog open={true}>
      <DialogHeader mb={3}>
        <DialogTitle>
          {action} {setting}?
        </DialogTitle>
      </DialogHeader>
      <DialogContent flexDirection="column" gap={4} mb={0} maxWidth="400px">
        <Text>{text}</Text>
        <Flex flexDirection="row" gap={3}>
          <ButtonSecondary onClick={() => setModal(undefined)}>
            Cancel
          </ButtonSecondary>
          <ButtonPrimary
            intent={modal.updateState === 'disable' ? 'danger' : 'primary'}
            disabled={disabled}
            onClick={() => {
              const res = modal.onConfirm();
              if (res && res.finally) {
                res.finally(() => setModal(undefined));
              } else {
                setModal(undefined);
              }
            }}
          >
            {action}
          </ButtonPrimary>
        </Flex>
      </DialogContent>
    </Dialog>
  );
};

export function OktaStatusDetails({
  plugin,
  deletePlugin,
}: {
  plugin: Plugin<PluginOktaSpec, PluginStatusOkta>;
  deletePlugin(): void;
}) {
  const ctx = useTeleport();
  const accessGraphEnabled =
    storageService.getAccessGraphEnabled() && ctx.getFeatureFlags().accessGraph;

  return (
    <Switch>
      {accessGraphEnabled && !cfg.isCloud && (
        <Route exact path={OktaIntegrationStepType.IdentitySecuritySync}>
          <SetupIdentitySecuritySyncForm plugin={plugin} isEditing />
        </Route>
      )}
      {cfg.entitlements.Identity.enabled
        ? [
            <Route
              exact
              key={OktaIntegrationStepType.Scim}
              path={OktaIntegrationStepType.Scim}
            >
              <ScimForm plugin={plugin} isEditing />
            </Route>,
            <Route
              exact
              key={OktaIntegrationStepType.UserSync}
              path={OktaIntegrationStepType.UserSync}
            >
              <UserSyncForm plugin={plugin} isEditing />
            </Route>,
            <Route
              exact
              key={OktaIntegrationStepType.AppGroupSync}
              path={OktaIntegrationStepType.AppGroupSync}
            >
              <AppGroupSyncForm plugin={plugin} isEditing />
            </Route>,
          ]
        : []}
      <Route path="*">
        <StatusDetails
          accessGraphEnabled={accessGraphEnabled}
          plugin={plugin}
          deletePlugin={deletePlugin}
        />
      </Route>
    </Switch>
  );
}

const StatusDetails = ({
  accessGraphEnabled,
  plugin,
  deletePlugin,
}: {
  accessGraphEnabled: boolean;
  plugin: Plugin<PluginOktaSpec, PluginStatusOkta>;
  deletePlugin: () => void;
}) => {
  const navigate = useNavigate();
  const [confirmModal, setConfirmModal] = useState<
    ConfirmModalState | undefined
  >();
  const [localSettings, setLocalSettings] = useState<oktaPluginUpdate>({
    enableAccessListSync: plugin.spec?.enableAccessListSync,
    enableAppGroupSync: plugin.spec?.enableAppGroupSync,
    enableUserSync: plugin.spec?.enableUserSync,
    assignDefaultRoles: plugin.spec?.assignDefaultRoles,
    enableBidirectionalSync: plugin.spec?.enableBidirectionalSync,
    enableSystemLogExport: plugin.spec?.enableSystemLogExport,
    defaultOwners: plugin.spec?.defaultOwners ?? [],
    appFilters:
      plugin?.status?.details?.accessListsSyncDetails?.appFilters ?? [],
    groupFilters:
      plugin?.status?.details?.accessListsSyncDetails?.groupFilters ?? [],
  });

  const queryClient = useQueryClient();

  const update = useMutation({
    mutationFn: (opts: NonNullable<PluginUpdateRequest['okta']>) =>
      pluginsService
        .updatePlugin({
          plugin: 'okta',
          okta: opts,
        })
        .catch(withUnsupportedOktaPluginUpdateErrorConversion),
    onSuccess: data =>
      queryClient.setQueryData(createFetchPluginQueryKey('okta'), data),
  });

  const updatePlugin = useCallback(
    async (opts: NonNullable<PluginUpdateRequest['okta']>) => {
      if (update.isPending) {
        return;
      }

      await update.mutateAsync(opts);

      setLocalSettings(current => ({ ...current, ...opts }));
    },
    [update]
  );

  const handleToggleFeature = useCallback(
    (
      updateState: UpdateState,
      updateSetting: Exclude<UpdateSetting, UpdateSetting.AppGroupSync>
    ) => {
      // SCIM can only be set up, not disabled.
      if (updateSetting === UpdateSetting.SCIM) {
        navigate(
          cfg.getIntegrationStatusRoute(
            'okta',
            'okta',
            OktaIntegrationStepType.Scim
          )
        );
        return;
      }

      // Confirm before disabling User Sync or App/Group Sync.
      if (updateState === 'disable') {
        setConfirmModal({
          updateState,
          updateSetting,
          onConfirm: () =>
            updatePlugin(
              handleUpdateType(`disable${updateSetting}`, localSettings)
            ),
        });
        return;
      }

      // User Sync and App/Group Sync require OAuth creds. If not present, prompt before redirecting to
      // the UserSync setup page to add them.
      if (!plugin.spec?.credentialsInfo?.hasConfiguredOauthCredentials) {
        const goToSetup = () =>
          navigate(
            cfg.getIntegrationStatusRoute(
              'okta',
              'okta',
              OktaIntegrationStepType.UserSync
            )
          );
        // Confirm before navigating to the User Sync setup to avoid any confusion
        if (
          updateSetting === UpdateSetting.AccessListSync ||
          updateSetting === UpdateSetting.IdentitySecuritySync
        ) {
          setConfirmModal({
            updateState,
            updateSetting,
            onConfirm: goToSetup,
          });
        } else {
          goToSetup();
        }
        return;
      }

      // If defaultOwners or app/group filters are missing, we need to set up App/Group Sync first.
      if (
        updateSetting === UpdateSetting.AccessListSync &&
        (!plugin.spec?.defaultOwners?.length ||
          !!plugin.status?.details?.accessListsSyncDetails?.appFilters ||
          !!plugin.status?.details?.accessListsSyncDetails?.groupFilters)
      ) {
        navigate(
          cfg.getIntegrationStatusRoute(
            'okta',
            'okta',
            OktaIntegrationStepType.AppGroupSync
          )
        );
        return;
      }

      // Go to the setup page for Identity Security Sync so we can show the additional scopes required.
      if (updateSetting === UpdateSetting.IdentitySecuritySync) {
        navigate(
          cfg.getIntegrationStatusRoute(
            'okta',
            'okta',
            OktaIntegrationStepType.IdentitySecuritySync
          )
        );
        return;
      }

      return updatePlugin(
        handleUpdateType(`enable${updateSetting}`, localSettings)
      );
    },
    [
      localSettings,
      updatePlugin,
      navigate,
      plugin.spec?.credentialsInfo?.hasConfiguredOauthCredentials,
      plugin.spec?.defaultOwners,
      plugin.status?.details?.accessListsSyncDetails?.appFilters,
      plugin.status?.details?.accessListsSyncDetails?.groupFilters,
    ]
  );

  const enabledAppGroupSync = plugin.spec.enableAppGroupSync;

  return (
    <Box>
      {confirmModal && (
        <ConfirmationModal
          modal={confirmModal}
          setModal={setConfirmModal}
          disabled={update.isPending}
        />
      )}
      {update.isError && (
        <Alert kind="outline-danger">{getErrMessage(update.error)}</Alert>
      )}
      <FlexSection>
        <SsoDetails
          spec={plugin.status.details?.ssoDetails}
          orgUrl={plugin.spec.orgUrl}
        />
        <ScimDetails
          spec={plugin.status.details?.ssoDetails}
          orgUrl={plugin.spec.orgUrl}
          toggled={
            plugin.spec.credentialsInfo?.hasSCIMToken ||
            plugin.status.details?.scimDetails?.enabled
          }
          disabled={update.isPending}
          // SCIM can't be disabled from Teleport – however, the user can re-save the SCIM settings
          // to generate and save a new bearer token.
          onToggle={() => handleToggleFeature('enable', UpdateSetting.SCIM)}
        />
        <UserSyncDetails
          spec={plugin.status.details?.usersSyncDetails}
          bidirectionalSync={!!plugin.spec?.enableBidirectionalSync}
          disabled={update.isPending}
          toggled={localSettings.enableUserSync}
          onToggle={() =>
            handleToggleFeature(
              localSettings.enableUserSync ? 'disable' : 'enable',
              UpdateSetting.UserSync
            )
          }
        />
      </FlexSection>
      <FlexSection>
        <AppGroupSyncDetails
          appGroupSpec={plugin.status.details?.appGroupSyncDetails}
          accessListSpec={plugin.status.details?.accessListsSyncDetails}
          defaultOwners={plugin.spec.defaultOwners}
          disabled={update.isPending}
          toggled={localSettings.enableAccessListSync}
          onToggle={() =>
            handleToggleFeature(
              localSettings.enableAccessListSync ? 'disable' : 'enable',
              UpdateSetting.AccessListSync
            )
          }
        />
      </FlexSection>
      <FlexSection>
        {!cfg.isCloud && (
          <IdentitySecuritySyncDetails
            syncEnabled={plugin.spec?.enableSystemLogExport}
            accessGraphEnabled={accessGraphEnabled}
            onToggle={() =>
              handleToggleFeature(
                localSettings.enableSystemLogExport ? 'disable' : 'enable',
                UpdateSetting.IdentitySecuritySync
              )
            }
          />
        )}

        <SetupAccessCta
          enabledAppGroupSync={enabledAppGroupSync}
          oktaOrgUrl={plugin.spec.orgUrl}
        />
      </FlexSection>
      <ButtonWarning size="large" onClick={deletePlugin} mt={2}>
        <Trash mr={2} />
        <Text>Delete Integration</Text>
      </ButtonWarning>
    </Box>
  );
};

const FlexSection = styled(FlexWrap)`
  gap: ${p => p.theme.space[3]}px;
  margin-bottom: ${p => p.theme.space[3]}px;
  @media screen and (max-width: ${p => p.theme.breakpoints.tablet}) {
    gap: ${p => p.theme.space[4]}px;
    margin-bottom: ${p => p.theme.space[4]}px;
  }
`;
