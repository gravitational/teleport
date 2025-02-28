import { useCallback, useEffect, useState } from 'react';
import { Link } from 'react-router-dom';

import {
  Alert,
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  H2,
  Indicator,
  Text,
  Toggle,
} from 'design';
import { FieldSelectCreatable } from 'shared/components/FieldSelect';
import type { Option } from 'shared/components/Select';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import { useAsync } from 'shared/hooks/useAsync';
import useAttempt, { State as AttemptState } from 'shared/hooks/useAttemptNext';
import { assertUnreachable } from 'shared/utils/assertUnreachable';
import { getErrMessage } from 'shared/utils/errorType';

import type { UserOption } from 'e-teleport/AccessListManagement/Shared/Shared';
import cfg from 'e-teleport/config';
import { CreateFilters } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/ImportUserGroupsAndApps/CreateFilters';
import { FailedAttempt } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/ImportUserGroupsAndApps/FailedAttempt';
import {
  AppTable,
  UserGroupsTable,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/ImportUserGroupsAndApps/FilterTable';
import {
  FilterOption,
  FormDataFilterField,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/ImportUserGroupsAndApps/types';
import { useOktaIntegrationSetUpContext } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/SetUpContext';
import {
  OktaIntegrationLevel,
  OktaSetupStepComplete,
  StyledBox,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Shared';
import { FormDataField } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/types';
import { Header } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Shared';
import {
  PluginConfigOktaApp,
  PluginConfigOktaGroup,
  pluginsService,
} from 'e-teleport/services/plugins';
import { Redirect } from 'teleport/components/Router';
import { ApiError } from 'teleport/services/api/parseError';
import type { Plugin, PluginOktaSpec } from 'teleport/services/integrations';
import { PluginStatusOkta } from 'teleport/services/integrations/oktaStatusTypes';
import userService, { User } from 'teleport/services/user';
import { withUnsupportedOktaPluginUpdateErrorConversion } from 'teleport/services/version/unsupported';
import useTeleport from 'teleport/useTeleport';

export const SetUpAppGroupSync = () => {
  const { plugin, setPlugin, startFrom } = useOktaIntegrationSetUpContext();

  if (startFrom === OktaIntegrationLevel.APP_GROUP_SYNC) {
    return <Redirect to={cfg.oss.getIntegrationEnrollRoute('okta')} />;
  }

  return <AppGroupSyncForm plugin={plugin} setPlugin={setPlugin} />;
};

export const AppGroupSyncForm = ({
  plugin,
  setPlugin,
  isEditing,
}: {
  plugin: Plugin<PluginOktaSpec, PluginStatusOkta>;
  setPlugin: (plugin: Plugin<PluginOktaSpec, PluginStatusOkta>) => void;
  isEditing?: boolean;
}) => {
  const ctx = useTeleport();
  const userAccess = ctx.storeUser.getUserAccess();
  const canReadListUsers = userAccess.list && userAccess.read;

  const { attempt: fetchUserAttempt, run: fetchUsersRun } = useAttempt(
    canReadListUsers ? 'processing' : ''
  );
  const {
    attempt: filterGroupsAttempt,
    setAttempt: setFilterGroupsAttempt,
    run: filterGroupsRun,
  } = useAttempt('processing');

  const {
    attempt: filterAppsAttempt,
    setAttempt: setFilterAppsAttempt,
    run: filterAppsRun,
  } = useAttempt('processing');

  const [appFilters, setAppFilters] = useState<FilterOption[]>(
    (plugin?.status?.details?.accessListsSyncDetails?.appFilters || []).map(
      s => ({ value: s, label: s, invalid: false })
    )
  );
  const [groupFilters, setGroupFilters] = useState<FilterOption[]>(
    (plugin?.status?.details?.accessListsSyncDetails?.groupFilters || []).map(
      s => ({ value: s, label: s, invalid: false })
    )
  );
  const [syncAllApps, setSyncAllApps] = useState(
    (plugin?.status?.details?.accessListsSyncDetails?.appFilters?.length ??
      0) === 0
  );
  const [syncAllGroups, setSyncAllGroups] = useState(
    (plugin?.status?.details?.accessListsSyncDetails?.groupFilters?.length ??
      0) === 0
  );
  const [apps, setApps] = useState<PluginConfigOktaApp[]>([]);
  const [userGroups, setUserGroups] = useState<PluginConfigOktaGroup[]>([]);
  const [userOptions, setUserOptions] = useState<UserOption[]>([]);
  const [selectedOwners, setSelectedOwners] = useState<UserOption[]>([]);

  const getFetchAppsGroupsFormData = () => {
    const data = new FormData();
    data.set(FormDataField.OrgUrl, plugin.spec.orgUrl);
    data.set(
      FormDataField.GroupFilters,
      syncAllGroups ? '[]' : JSON.stringify(groupFilters.map(o => o.value))
    );
    data.set(
      FormDataField.AppFilters,
      syncAllApps ? '[]' : JSON.stringify(appFilters.map(o => o.value))
    );
    return data;
  };

  useEffect(() => {
    fetchApps();
    fetchGroups();
    canReadListUsers && fetchUsers();
  }, []);

  const fetchUsers = () =>
    fetchUsersRun(() =>
      userService.fetchUsers().then(fetchedUsers => {
        const userOptions = fetchedUsers.map(u => ({
          value: u,
          label: u.name,
        }));
        setUserOptions(userOptions);
        if (plugin?.spec?.defaultOwners?.length) {
          const selectedOwners = plugin.spec.defaultOwners
            .map(o => userOptions.find(u => u.label === o))
            .filter(Boolean);
          setSelectedOwners(selectedOwners);
        }
      })
    );

  const fetchApps = () =>
    filterAppsRun(() =>
      pluginsService
        .getPluginConfigOktaApps(getFetchAppsGroupsFormData())
        .then(setApps)
    );

  const fetchGroups = () =>
    filterGroupsRun(() =>
      pluginsService
        .getPluginConfigOktaGroups(getFetchAppsGroupsFormData())
        .then(setUserGroups)
    );

  const [updatePluginAttempt, updatePlugin] = useAsync(
    useCallback(
      () =>
        pluginsService
          .updatePlugin({
            plugin: 'okta',
            okta: {
              enableAppGroupSync: true,
              enableAccessListSync: true,
              enableUserSync: true,
              defaultOwners: selectedOwners.map(o => o.label),
              // Providing an empty array is equivalent to an asterisk
              [FormDataField.GroupFilters]: syncAllGroups
                ? []
                : groupFilters.map(o => o.value),
              [FormDataField.AppFilters]: syncAllApps
                ? []
                : appFilters.map(o => o.value),
            },
          })
          .catch(withUnsupportedOktaPluginUpdateErrorConversion),
      [appFilters, groupFilters, selectedOwners, syncAllApps, syncAllGroups]
    )
  );

  const onSubmit = async (validator: Validator) => {
    if (
      updatePluginAttempt.status === 'processing' ||
      filterAppsAttempt.status === 'processing' ||
      filterGroupsAttempt.status === 'processing' ||
      !validator.validate() ||
      !!filterAppsAttempt.statusText ||
      !!filterGroupsAttempt.statusText
    ) {
      return;
    }

    const [resp, err] = await updatePlugin();
    if (err) {
      return;
    }
    setPlugin(resp);
  };

  const updateFilters = async ({
    applyFilters,
    setAppliedFiltersResp,
    setFilterAttempt,
    setFilter,
    formDataFilterField,
    validator,
    updatedFilters = [],
  }: {
    applyFilters(formData: FormData): Promise<unknown>;
    setAppliedFiltersResp(results: unknown): void;
    setFilterAttempt: AttemptState['setAttempt'];
    setFilter(f: FilterOption[]): void;
    formDataFilterField: FormDataFilterField;
    validator: Validator;
    updatedFilters: FilterOption[];
  }) => {
    const filters = updatedFilters.map(o => o.value);
    setFilter(updatedFilters);

    const formData = getFetchAppsGroupsFormData();
    formData.set(formDataFilterField, JSON.stringify(filters));
    formData.set(FormDataField.OrgUrl, plugin.spec.orgUrl);

    setFilterAttempt({ status: 'processing' });

    try {
      const resp = await applyFilters(formData);
      setAppliedFiltersResp(resp);
      setFilterAttempt({ status: 'success' });
    } catch (e) {
      if (!(e instanceof ApiError) || e.response.status !== 400) {
        setFilterAttempt({ status: 'failed', statusText: e.message });
        return;
      }

      // Check if a filter is mentioned in the error message.
      // Any offending filter will be returned.
      let foundInvalidFilter = false;
      const messages = e.messages.join(' ');
      const markedFilters = updatedFilters.map(f => {
        if (e.message.includes(f.value) || messages.includes(f.value)) {
          foundInvalidFilter = true;
          return { ...f, invalid: true };
        }
        return f;
      });
      if (foundInvalidFilter) {
        setFilter(markedFilters);
        // We will show the error on the created filters.
        setFilterAttempt({ status: '', statusText: e.message });
        // Highlight invalid filters.
        validator.validate();
        return;
      }

      setFilterAttempt({ status: 'failed', statusText: e.message });
    }
  };

  const handleFilterOnChange = (
    opts: FilterOption[],
    validator: Validator,
    formDataFilterField: FormDataFilterField
  ) => {
    switch (formDataFilterField) {
      case FormDataField.AppFilters:
        return updateFilters({
          applyFilters: pluginsService.getPluginConfigOktaApps,
          setAppliedFiltersResp: setApps,
          setFilterAttempt: setFilterAppsAttempt,
          setFilter: setAppFilters,
          formDataFilterField,
          validator,
          updatedFilters: opts,
        });
      case FormDataField.GroupFilters:
        return updateFilters({
          applyFilters: pluginsService.getPluginConfigOktaGroups,
          setAppliedFiltersResp: setUserGroups,
          setFilterAttempt: setFilterGroupsAttempt,
          setFilter: setGroupFilters,
          formDataFilterField,
          validator,
          updatedFilters: opts,
        });
      default:
        assertUnreachable(formDataFilterField);
    }
  };

  if (updatePluginAttempt.status === 'success') {
    return isEditing ? (
      <Redirect to={cfg.oss.getIntegrationStatusRoute('okta', 'okta')} />
    ) : (
      <OktaSetupStepComplete step={OktaIntegrationLevel.APP_GROUP_SYNC} />
    );
  }

  return (
    <Flex flexDirection="column" gap={5} maxWidth={900}>
      <Box>
        <Header header="Sync User Groups and App Assignments" />
        <Text>
          For each user group and app with direct user assignments, Teleport
          will generate corresponding Access Lists that grant the same access.
          If you don’t want to sync all of your groups or apps, just toggle off
          the “Sync All” option and add regex or glob patterns to filter for the
          lists you do want.
        </Text>
      </Box>
      <Validation>
        {({ validator }) => (
          <Flex flexDirection="column" gap={4} width="100%">
            <StyledBox>
              <H2 mb={1}>Step 1: Set Default List Owner(s) for Access Lists</H2>
              <Text>
                List Owners are responsible for periodically reviewing
                membership to each Access List. You must assign at least 1
                default owner to your imported access lists. You can edit owners
                on individual lists, later.
              </Text>
              {fetchUserAttempt.status === 'failed' && (
                <FailedAttempt
                  attempt={fetchUserAttempt}
                  retry={() => fetchUsers()}
                />
              )}
              {fetchUserAttempt.status !== 'processing' ? (
                <Box width="540px">
                  <FieldSelectCreatable
                    autoFocus={true}
                    placeholder="Type a username and press enter"
                    isMulti
                    isClearable
                    isSearchable
                    options={userOptions}
                    isDisabled={updatePluginAttempt.status === 'processing'}
                    onChange={(opts: Option<User>[]) => setSelectedOwners(opts)}
                    value={selectedOwners || []}
                    noOptionsMessage={() => 'Type a username and press enter'}
                    label="Add Default List Owner(s)"
                    rule={requiredField('At least 1 default owner is required')}
                  />
                </Box>
              ) : (
                <Box textAlign="center">
                  <Indicator delay="none" />
                </Box>
              )}
            </StyledBox>
            <StyledBox>
              <Flex
                flexDirection="row"
                alignItems="center"
                justifyContent="space-between"
                width="100%"
                mb={1}
              >
                <H2>Step 2: Sync Okta User Groups as Access Lists</H2>
                <Flex flexDirection="row" alignItems="center" gap={2}>
                  <Toggle
                    isToggled={syncAllGroups}
                    onToggle={() => setSyncAllGroups(s => !s)}
                    size="large"
                  />
                  <Text typography="body1">Sync All User Groups</Text>
                </Flex>
              </Flex>
              {filterGroupsAttempt.statusText && (
                <FailedAttempt
                  attempt={filterGroupsAttempt}
                  retry={
                    filterGroupsAttempt.status === 'failed'
                      ? () =>
                          handleFilterOnChange(
                            groupFilters,
                            validator,
                            FormDataField.GroupFilters
                          )
                      : undefined
                  }
                />
              )}
              {!syncAllGroups && (
                <CreateFilters
                  filters={groupFilters}
                  validator={validator}
                  filterKind={FormDataField.GroupFilters}
                  onFilterChange={handleFilterOnChange}
                  importAttempt={{
                    statusText: updatePluginAttempt.statusText,
                    status:
                      updatePluginAttempt.status === 'error'
                        ? 'failed'
                        : updatePluginAttempt.status,
                  }}
                  filterAttempt={filterGroupsAttempt}
                />
              )}
              <UserGroupsTable
                userGroups={userGroups}
                loading={filterGroupsAttempt.status === 'processing'}
              />
            </StyledBox>
            <StyledBox>
              <Flex
                flexDirection="row"
                alignItems="center"
                justifyContent="space-between"
                width="100%"
                mb={1}
              >
                <H2>Step 3: Sync Direct Assignments as Access Lists</H2>
                <Flex flexDirection="row" alignItems="center" gap={2}>
                  <Toggle
                    isToggled={syncAllApps}
                    onToggle={() => setSyncAllApps(s => !s)}
                    size="large"
                  />
                  <Text typography="body1">Sync All Apps</Text>
                </Flex>
              </Flex>
              <Text>
                Teleport will import and sync any applications with direct
                assignments to Teleport Access Lists. You will not see an Access
                List if there are no direct assignments.
              </Text>
              {filterAppsAttempt.statusText && (
                <FailedAttempt
                  attempt={filterAppsAttempt}
                  retry={
                    filterAppsAttempt.status === 'failed'
                      ? () =>
                          handleFilterOnChange(
                            appFilters,
                            validator,
                            FormDataField.AppFilters
                          )
                      : undefined
                  }
                />
              )}
              {!syncAllApps && (
                <CreateFilters
                  filters={appFilters}
                  validator={validator}
                  filterKind={FormDataField.AppFilters}
                  onFilterChange={handleFilterOnChange}
                  importAttempt={{
                    statusText: updatePluginAttempt.statusText,
                    status:
                      updatePluginAttempt.status === 'error'
                        ? 'failed'
                        : updatePluginAttempt.status,
                  }}
                  filterAttempt={filterAppsAttempt}
                />
              )}
              <AppTable
                apps={apps}
                loading={filterAppsAttempt.status === 'processing'}
              />
            </StyledBox>
            {updatePluginAttempt.status === 'error' && (
              <Alert kind="outline-danger" mb={0}>
                {getErrMessage(updatePluginAttempt.error)}
              </Alert>
            )}
            <Flex flexDirection="row" alignItems="center" gap={3}>
              <ButtonPrimary
                onClick={() => onSubmit(validator)}
                disabled={
                  updatePluginAttempt.status === 'processing' ||
                  filterAppsAttempt.status === 'processing' ||
                  filterGroupsAttempt.status === 'processing' ||
                  !selectedOwners?.length ||
                  !!filterAppsAttempt.statusText ||
                  !!filterGroupsAttempt.statusText
                }
              >
                {isEditing ? 'Update' : 'Submit Configuration'}
              </ButtonPrimary>
              <ButtonSecondary
                as={Link}
                to={
                  isEditing
                    ? cfg.oss.getIntegrationStatusRoute('okta', 'okta')
                    : cfg.oss.getIntegrationEnrollRoute('okta')
                }
                disabled={updatePluginAttempt.status === 'processing'}
              >
                {isEditing ? 'Cancel' : 'Back'}
              </ButtonSecondary>
            </Flex>
          </Flex>
        )}
      </Validation>
    </Flex>
  );
};
