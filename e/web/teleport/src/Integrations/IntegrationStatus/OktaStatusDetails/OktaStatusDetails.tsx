import { useCallback, useState } from 'react';
import { useHistory } from 'react-router-dom';

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
import { Attempt, useAsync } from 'shared/hooks/useAsync';
import { getErrMessage } from 'shared/utils/errorType';

import { OktaIntegrationLevel } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Shared';
import { AppGroupSyncForm } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/SetUpAppGroupSync';
import { ScimForm } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/SetUpScim';
import { UserSyncForm } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/SetUpUserSync';
import {
  pluginsService,
  PluginUpdateRequest,
} from 'e-teleport/services/plugins';
import { Route, Switch } from 'teleport/components/Router';
import cfg from 'teleport/config';
import { Plugin, PluginOktaSpec } from 'teleport/services/integrations';
import { PluginStatusOkta } from 'teleport/services/integrations/oktaStatusTypes';
import { withUnsupportedOktaPluginUpdateErrorConversion } from 'teleport/services/version/unsupported';

import { AppGroupSyncDetails } from './AppGroupSyncDetails';
import { ScimDetails } from './ScimDetails';
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
}

type UpdateType = `${UpdateState}${UpdateSetting}`;

const handleUpdateType = (
  updateType: UpdateType,
  opts: NonNullable<PluginUpdateRequest['okta']>
): NonNullable<PluginUpdateRequest['okta']> => {
  switch (updateType) {
    case 'enableUserSync':
      opts.enableUserSync = true;
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
  updateSetting: UpdateSetting.AccessListSync | UpdateSetting.UserSync;
  onConfirm: () => void | Promise<void>;
};

const ConfirmationModal = ({
  modal,
  setModal,
  attempt,
}: {
  modal: ConfirmModalState;
  setModal: (state: ConfirmModalState | undefined) => void;
  attempt: Attempt<unknown>;
}) => {
  const action = modal.updateState === 'disable' ? 'Disable' : 'Enable';
  const setting =
    modal.updateSetting === UpdateSetting.UserSync
      ? 'User Sync'
      : 'App and Group Sync';
  let textContent = '';
  if (modal.updateState === 'disable') {
    if (modal.updateSetting === UpdateSetting.UserSync) {
      textContent = 'This will also disable App and Group Sync. ';
    }
    textContent += `Your current configuration will be saved, and ${setting} may be re-enabled later.`;
  }
  if (modal.updateState === 'enable') {
    if (modal.updateSetting === UpdateSetting.AccessListSync) {
      textContent =
        'Enabling App and Group Sync requires that User Sync be enabled.';
    }
  }

  return (
    <Dialog open={true}>
      <DialogHeader mb={3}>
        <DialogTitle>
          {action} {setting}?
        </DialogTitle>
      </DialogHeader>
      <DialogContent flexDirection="column" gap={4} mb={0} maxWidth="400px">
        <Text>{textContent}</Text>
        <Flex flexDirection="row" gap={3}>
          <ButtonSecondary onClick={() => setModal(undefined)}>
            Cancel
          </ButtonSecondary>
          <ButtonPrimary
            intent={modal.updateState === 'disable' ? 'danger' : 'primary'}
            disabled={attempt.status === 'processing'}
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
  setPlugin,
  deletePlugin,
}: {
  plugin: Plugin<PluginOktaSpec, PluginStatusOkta>;
  setPlugin: (plugin: Plugin<PluginOktaSpec, PluginStatusOkta>) => void;
  deletePlugin(): void;
}) {
  return (
    <Switch>
      ...
      {cfg.entitlements.Identity.enabled
        ? [
            <Route
              exact
              key={OktaIntegrationLevel.SCIM}
              path={cfg.getIntegrationStatusRoute(
                'okta',
                'okta',
                OktaIntegrationLevel.SCIM
              )}
            >
              <ScimForm plugin={plugin} setPlugin={setPlugin} isEditing />
            </Route>,
            <Route
              exact
              key={OktaIntegrationLevel.USER_SYNC}
              path={cfg.getIntegrationStatusRoute(
                'okta',
                'okta',
                OktaIntegrationLevel.USER_SYNC
              )}
            >
              <UserSyncForm plugin={plugin} setPlugin={setPlugin} isEditing />
            </Route>,
            <Route
              exact
              key={OktaIntegrationLevel.APP_GROUP_SYNC}
              path={cfg.getIntegrationStatusRoute(
                'okta',
                'okta',
                OktaIntegrationLevel.APP_GROUP_SYNC
              )}
            >
              <AppGroupSyncForm
                plugin={plugin}
                setPlugin={setPlugin}
                isEditing
              />
            </Route>,
          ]
        : []}
      <Route path={cfg.getIntegrationStatusRoute('okta', 'okta')}>
        <StatusDetails
          plugin={plugin}
          setPlugin={setPlugin}
          deletePlugin={deletePlugin}
        />
      </Route>
    </Switch>
  );
}

const StatusDetails = ({
  plugin,
  setPlugin,
  deletePlugin,
}: {
  plugin: Plugin<PluginOktaSpec, PluginStatusOkta>;
  setPlugin: (plugin: Plugin<PluginOktaSpec, PluginStatusOkta>) => void;
  deletePlugin: () => void;
}) => {
  const history = useHistory();
  const [confirmModal, setConfirmModal] = useState<
    ConfirmModalState | undefined
  >();
  const [localSettings, setLocalSettings] = useState({
    enableAccessListSync: plugin.spec?.enableAccessListSync,
    enableAppGroupSync: plugin.spec?.enableAppGroupSync,
    enableUserSync: plugin.spec?.enableUserSync,
    defaultOwners: plugin.spec?.defaultOwners ?? [],
    appFilters:
      plugin?.status?.details?.accessListsSyncDetails?.appFilters ?? [],
    groupFilters:
      plugin?.status?.details?.accessListsSyncDetails?.groupFilters ?? [],
  });
  const [updatePluginAttempt, runUpdatePlugin] = useAsync(
    useCallback(
      (opts: NonNullable<PluginUpdateRequest['okta']>) =>
        pluginsService
          .updatePlugin({
            plugin: 'okta',
            okta: opts,
          })
          .catch(withUnsupportedOktaPluginUpdateErrorConversion),
      []
    )
  );

  const updatePlugin = async (
    opts: NonNullable<PluginUpdateRequest['okta']>
  ) => {
    if (updatePluginAttempt.status === 'processing') {
      return;
    }

    const [resp, err] = await runUpdatePlugin(opts);
    if (err) {
      return;
    }
    setPlugin(resp);
    setLocalSettings(current => ({ ...current, ...opts }));
  };

  const handleToggleFeature = (
    updateState: UpdateState,
    updateSetting: Exclude<UpdateSetting, UpdateSetting.AppGroupSync>
  ) => {
    // SCIM can only be set up, not disabled.
    if (updateSetting === UpdateSetting.SCIM) {
      history.push(
        cfg.getIntegrationStatusRoute('okta', 'okta', OktaIntegrationLevel.SCIM)
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
        history.push(
          cfg.getIntegrationStatusRoute(
            'okta',
            'okta',
            OktaIntegrationLevel.USER_SYNC
          )
        );
      // Confirm before navigating to the User Sync setup to avoid any confusion
      if (updateSetting === UpdateSetting.AccessListSync) {
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
      history.push(
        cfg.getIntegrationStatusRoute(
          'okta',
          'okta',
          OktaIntegrationLevel.APP_GROUP_SYNC
        )
      );
      return;
    }

    return updatePlugin(
      handleUpdateType(`enable${updateSetting}`, localSettings)
    );
  };

  return (
    <Box>
      {confirmModal && (
        <ConfirmationModal
          modal={confirmModal}
          setModal={setConfirmModal}
          attempt={updatePluginAttempt}
        />
      )}
      {updatePluginAttempt.status === 'error' && (
        <Alert kind="outline-danger">
          {getErrMessage(updatePluginAttempt.statusText)}
        </Alert>
      )}
      <FlexWrap
        css={`
          gap: ${p => p.theme.space[3]}px;
          margin-bottom: ${p => p.theme.space[3]}px;
          @media screen and (max-width: ${p => p.theme.breakpoints.tablet}) {
            gap: ${p => p.theme.space[4]}px;
            margin-bottom: ${p => p.theme.space[4]}px;
          }
        `}
      >
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
          disabled={updatePluginAttempt.status === 'processing'}
          // SCIM can't be disabled from Teleport – however, the user can re-save the SCIM settings
          // to generate and save a new bearer token.
          onToggle={() => handleToggleFeature('enable', UpdateSetting.SCIM)}
        />
        <UserSyncDetails
          spec={plugin.status.details?.usersSyncDetails}
          disabled={updatePluginAttempt.status === 'processing'}
          toggled={localSettings.enableUserSync}
          onToggle={() =>
            handleToggleFeature(
              localSettings.enableUserSync ? 'disable' : 'enable',
              UpdateSetting.UserSync
            )
          }
        />
      </FlexWrap>
      <Flex gap={3} flexWrap="wrap" mb={3}>
        <AppGroupSyncDetails
          appGroupSpec={plugin.status.details?.appGroupSyncDetails}
          accessListSpec={plugin.status.details?.accessListsSyncDetails}
          defaultOwners={plugin.spec.defaultOwners}
          disabled={updatePluginAttempt.status === 'processing'}
          toggled={localSettings.enableAccessListSync}
          onToggle={() =>
            handleToggleFeature(
              localSettings.enableAccessListSync ? 'disable' : 'enable',
              UpdateSetting.AccessListSync
            )
          }
        />
      </Flex>
      <ButtonWarning size="large" onClick={deletePlugin} mt={2}>
        <Trash mr={2} />
        <Text>Delete Integration</Text>
      </ButtonWarning>
    </Box>
  );
};
