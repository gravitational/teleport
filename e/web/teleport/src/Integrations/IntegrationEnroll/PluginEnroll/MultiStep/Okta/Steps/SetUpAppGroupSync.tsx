import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
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
import { getErrMessage } from 'shared/utils/errorType';

import type { UserOption } from 'e-teleport/AccessListManagement/Shared/Shared';
import cfg from 'e-teleport/config';
import {
  AppTable,
  UserGroupsTable,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/ImportUserGroupsAndApps/FilterTable';
import { FormDataFilterField } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/ImportUserGroupsAndApps/types';
import { useOktaIntegrationSetUpContext } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/SetUpContext';
import {
  APP_GROUP_SYNC_CONFIG,
  OktaIntegrationStepType,
  OktaSetupStepComplete,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Shared';
import type { OktaIntegrationStepFormProps } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/props';
import { FormDataField } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/types';
import { Header } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Shared';
import { StyledBox } from 'e-teleport/Integrations/Shared';
import {
  CreateFilters,
  FilterOption,
} from 'e-teleport/Integrations/shared/CreateFilters';
import { pluginsService } from 'e-teleport/services/plugins';
import { createFetchPluginQueryKey } from 'e-teleport/services/plugins/hooks';
import { Redirect } from 'teleport/components/Router';
import { ApiError } from 'teleport/services/api/parseError';
import userService, { User } from 'teleport/services/user';
import { withUnsupportedOktaPluginUpdateErrorConversion } from 'teleport/services/version/unsupported';
import useTeleport from 'teleport/useTeleport';

export const SetUpAppGroupSync = () => {
  const { completedStepTypes, getPreviousStep, plugin, startFrom } =
    useOktaIntegrationSetUpContext();

  const previousStep = getPreviousStep(OktaIntegrationStepType.UserSync);
  const previousStepType = completedStepTypes.includes(previousStep?.type)
    ? undefined
    : previousStep?.type;

  if (startFrom === OktaIntegrationStepType.AppGroupSync) {
    return <Redirect to={cfg.oss.getIntegrationEnrollRoute('okta')} />;
  }

  return (
    <AppGroupSyncForm plugin={plugin} previousStepType={previousStepType} />
  );
};

export const AppGroupSyncForm = ({
  plugin,
  isEditing,
  previousStepType,
}: OktaIntegrationStepFormProps) => {
  const queryClient = useQueryClient();
  const ctx = useTeleport();
  const userAccess = ctx.storeUser.getUserAccess();
  const canReadListUsers = userAccess.list && userAccess.read;

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

  const getFetchAppsGroupsFormData = useCallback(() => {
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
  }, [
    appFilters,
    groupFilters,
    plugin.spec.orgUrl,
    syncAllApps,
    syncAllGroups,
  ]);

  const [selectedOwners, setSelectedOwners] = useState<UserOption[]>([]);

  const users = useQuery({
    queryKey: ['users', 'fetch'],
    queryFn: async () => {
      const users = await userService.fetchUsers();

      return users.map(u => ({
        value: u,
        label: u.name,
      }));
    },
    enabled: canReadListUsers,
  });

  useEffect(() => {
    if (users.isSuccess && plugin?.spec?.defaultOwners?.length) {
      const selectedOwners = plugin.spec.defaultOwners
        .map(o => users.data.find(u => u.label === o))
        .filter(Boolean);

      setSelectedOwners(selectedOwners);
    }
  }, [plugin.spec.defaultOwners, users.data, users.isSuccess]);

  const formData = getFetchAppsGroupsFormData();

  const apps = useQuery({
    queryKey: ['okta', 'apps'],
    queryFn: () => pluginsService.getPluginConfigOktaApps(formData),
  });

  const groups = useQuery({
    queryKey: ['okta', 'groups'],
    queryFn: () => pluginsService.getPluginConfigOktaGroups(formData),
  });

  const updatePlugin = useMutation({
    mutationFn: () =>
      pluginsService
        .updatePlugin({
          plugin: 'okta',
          okta: {
            enableAppGroupSync: true,
            enableAccessListSync: true,
            enableUserSync: true,
            assignDefaultRoles: !!plugin?.spec?.assignDefaultRoles,
            enableSystemLogExport: !!plugin?.spec?.enableSystemLogExport,
            defaultOwners: selectedOwners.map(o => o.label),
            enableBidirectionalSync: !!plugin?.spec?.enableBidirectionalSync,
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
    onSuccess: data =>
      queryClient.setQueryData(createFetchPluginQueryKey('okta'), data),
  });

  const onSubmit = useCallback(
    (validator: Validator) => {
      if (
        updatePlugin.isPending ||
        apps.isPending ||
        groups.isPending ||
        !validator.validate()
      ) {
        return;
      }

      updatePlugin.mutate();
    },
    [apps.isPending, groups.isPending, updatePlugin]
  );

  const [invalidAppFilters, setInvalidAppFilters] = useState(false);
  const [invalidGroupFilters, setInvalidGroupFilters] = useState(false);

  const updateFilters = useCallback(
    async ({
      applyFilters,
      queryKey,
      setHasInvalidFilters,
      setFilter,
      formDataFilterField,
      validator,
      updatedFilters = [],
    }: {
      applyFilters: (formData: FormData) => Promise<unknown>;
      queryKey: string[];
      setHasInvalidFilters: (hasInvalidFilters: boolean) => void;
      setFilter(f: FilterOption[]): void;
      formDataFilterField: FormDataFilterField;
      validator: Validator;
      updatedFilters: FilterOption[];
    }) => {
      const filters = updatedFilters.map(o => o.value);

      setFilter(updatedFilters);
      setHasInvalidFilters(false);

      const formData = getFetchAppsGroupsFormData();

      formData.set(formDataFilterField, JSON.stringify(filters));

      try {
        await queryClient.fetchQuery({
          queryKey,
          queryFn: async () => {
            try {
              return await applyFilters(formData);
            } catch (e) {
              if (!(e instanceof ApiError) || e.response.status !== 400) {
                throw e;
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
                setHasInvalidFilters(true);
                // Highlight invalid filters.
                validator.validate();
              }

              throw e;
            }
          },
        });
      } catch {
        // no need to handle the error here
      }
    },
    [getFetchAppsGroupsFormData, queryClient]
  );

  const handleAppFilterChange = useCallback(
    (opts: FilterOption[], validator: Validator) => {
      return updateFilters({
        applyFilters: pluginsService.getPluginConfigOktaApps,
        queryKey: ['okta', 'apps'],
        setHasInvalidFilters: setInvalidAppFilters,
        setFilter: setAppFilters,
        formDataFilterField: FormDataField.AppFilters,
        validator,
        updatedFilters: opts,
      });
    },
    [updateFilters]
  );

  const handleGroupFilterChange = useCallback(
    (opts: FilterOption[], validator: Validator) => {
      return updateFilters({
        applyFilters: pluginsService.getPluginConfigOktaGroups,
        queryKey: ['okta', 'groups'],
        setHasInvalidFilters: setInvalidGroupFilters,
        setFilter: setGroupFilters,
        formDataFilterField: FormDataField.GroupFilters,
        validator,
        updatedFilters: opts,
      });
    },
    [updateFilters]
  );

  if (updatePlugin.isSuccess) {
    if (isEditing) {
      return (
        <Redirect to={cfg.oss.getIntegrationStatusRoute('okta', 'okta')} />
      );
    }

    return <OktaSetupStepComplete config={APP_GROUP_SYNC_CONFIG} />;
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
            <StyledBox header="Step 1: Set Default List Owner(s) for Access Lists">
              <Text>
                List Owners are responsible for periodically reviewing
                membership to each Access List. You must assign at least 1
                default owner to your imported access lists. You can edit owners
                on individual lists, later.
              </Text>
              {users.isError && (
                <Alert
                  kind="danger"
                  primaryAction={{
                    content: 'Retry',
                    onClick: () => void users.refetch(),
                  }}
                >
                  {getErrMessage(users.error)}
                </Alert>
              )}
              {!users.isPending ? (
                <Box width="540px">
                  <FieldSelectCreatable
                    autoFocus={true}
                    placeholder="Type a username and press enter"
                    isMulti
                    isClearable
                    isSearchable
                    options={users.data ?? []}
                    isDisabled={updatePlugin.isPending}
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
            <StyledBox
              header={
                <Flex
                  flexDirection="row"
                  alignItems="center"
                  justifyContent="space-between"
                  width="100%"
                >
                  <H2>Step 2: Sync Okta User Groups as Access Lists</H2>
                  <Flex flexDirection="row" alignItems="center" gap={3}>
                    <Toggle
                      isToggled={syncAllGroups}
                      onToggle={() => setSyncAllGroups(s => !s)}
                      size="large"
                    />
                    <Text typography="body1">Sync All User Groups</Text>
                  </Flex>
                </Flex>
              }
            >
              {groups.isError && (
                <Alert
                  kind="danger"
                  primaryAction={
                    invalidGroupFilters
                      ? undefined
                      : {
                          content: 'Retry',
                          onClick: () => void groups.refetch(),
                        }
                  }
                >
                  {getErrMessage(groups.error)}
                </Alert>
              )}
              {!syncAllGroups && (
                <CreateFilters
                  filters={groupFilters}
                  validator={validator}
                  label="Filter by Group Name(s) - Regex and glob supported"
                  onFilterChange={handleGroupFilterChange}
                  isDisabled={updatePlugin.isPending || groups.isPending}
                />
              )}
              <UserGroupsTable
                userGroups={groups.data ?? []}
                loading={groups.isPending}
              />
            </StyledBox>
            <StyledBox
              header={
                <Flex
                  flexDirection="row"
                  alignItems="center"
                  justifyContent="space-between"
                  width="100%"
                >
                  <H2>Step 3: Sync Direct Assignments as Access Lists</H2>
                  <Flex flexDirection="row" alignItems="center" gap={3}>
                    <Toggle
                      isToggled={syncAllApps}
                      onToggle={() => setSyncAllApps(s => !s)}
                      size="large"
                    />
                    <Text typography="body1">Sync All Apps</Text>
                  </Flex>
                </Flex>
              }
            >
              <Text>
                Teleport will import and sync any applications with direct
                assignments to Teleport Access Lists. You will not see an Access
                List if there are no direct assignments.
              </Text>
              {apps.isError && (
                <Alert
                  kind="danger"
                  primaryAction={
                    invalidAppFilters
                      ? undefined
                      : {
                          content: 'Retry',
                          onClick: () => void apps.refetch(),
                        }
                  }
                >
                  {getErrMessage(apps.error)}
                </Alert>
              )}
              {!syncAllApps && (
                <CreateFilters
                  filters={appFilters}
                  validator={validator}
                  label="Filter by App Name(s) - Regex and glob supported"
                  onFilterChange={handleAppFilterChange}
                  isDisabled={updatePlugin.isPending || apps.isPending}
                />
              )}
              <AppTable apps={apps.data ?? []} loading={apps.isPending} />
            </StyledBox>
            {updatePlugin.isError && (
              <Alert kind="outline-danger" mb={0}>
                {getErrMessage(updatePlugin.error)}
              </Alert>
            )}
            <Flex flexDirection="row" alignItems="center" gap={3}>
              <ButtonPrimary
                onClick={() => onSubmit(validator)}
                disabled={
                  updatePlugin.isPending ||
                  apps.isPending ||
                  groups.isPending ||
                  !selectedOwners?.length ||
                  invalidAppFilters ||
                  invalidGroupFilters
                }
              >
                {isEditing ? 'Save Changes' : 'Continue'}
              </ButtonPrimary>
              <ButtonSecondary
                as={Link}
                to={
                  isEditing
                    ? cfg.oss.getIntegrationStatusRoute('okta', 'okta')
                    : cfg.oss.getIntegrationEnrollRoute(
                        'okta',
                        previousStepType
                      )
                }
                disabled={updatePlugin.isPending}
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
