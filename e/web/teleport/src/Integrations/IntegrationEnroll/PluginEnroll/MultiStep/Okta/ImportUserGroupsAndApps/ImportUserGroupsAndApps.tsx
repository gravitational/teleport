import { useEffect, useState } from 'react';
import styled from 'styled-components';

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
import { Option } from 'shared/components/Select';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import useAttempt, { State as AttemptState } from 'shared/hooks/useAttemptNext';
import { assertUnreachable } from 'shared/utils/assertUnreachable';

import { pluginsService } from 'e-teleport/services/plugins';
import {
  PluginConfigOktaApp,
  PluginConfigOktaGroup,
} from 'e-teleport/services/plugins/types';
import { ApiError } from 'teleport/services/api/parseError';
import { PluginOktaSpec } from 'teleport/services/integrations';
import userService, { User } from 'teleport/services/user';
import useTeleport from 'teleport/useTeleport';

import { Header } from '../../Shared';
import { usePlugin } from '../../usePlugin';
import { FormDataField } from '../types';
import { CreateFilters } from './CreateFilters';
import { FailedAttempt } from './FailedAttempt';
import { AppTable, UserGroupsTable } from './FilterTable';
import { FilterOption, FormDataFilterField } from './types';

type UserOption = Option<User>;

export function ImportUserGroupsAndApps() {
  const ctx = useTeleport();
  const userAccess = ctx.storeUser.getUserAccess();
  const canReadListUsers = userAccess.list && userAccess.read;

  const { nextStep, prevStep, formData, setInstalledPlugin } =
    usePlugin<PluginOktaSpec>();
  const [importAllUserGroups, setImportAllUserGroups] = useState(true);
  const [importAllApps, setImportAllApps] = useState(true);
  const [userGroups, setUserGroups] = useState<PluginConfigOktaGroup[]>([]);
  const [apps, setApps] = useState<PluginConfigOktaApp[]>([]);
  const [userOptions, setUserOptions] = useState<UserOption[]>([]);
  const [selectedOwners, setSelectedOwners] = useState<UserOption[]>([]);
  const [groupFilters, setGroupFilters] = useState<FilterOption[]>([]);
  const [appFilters, setAppFilters] = useState<FilterOption[]>([]);

  const { attempt: importAttempt, run: importRun } = useAttempt();
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

  useEffect(() => {
    filterAppsRun(() =>
      pluginsService.getPluginConfigOktaApps(formData).then(setApps)
    );

    filterGroupsRun(() =>
      pluginsService.getPluginConfigOktaGroups(formData).then(setUserGroups)
    );

    if (canReadListUsers) {
      fetchUsers();
    }
  }, []);

  function fetchUsers() {
    fetchUsersRun(() =>
      userService.fetchUsers().then(fetchedUsers => {
        setUserOptions(fetchedUsers.map(u => ({ value: u, label: u.name })));
      })
    );
  }

  function handleSkipStep() {
    formData.delete(FormDataField.GroupFilters);
    formData.delete(FormDataField.AppFilters);
    formData.delete(FormDataField.DefaultOwners);

    importRun(() =>
      pluginsService.createPlugin(formData).then(resp => {
        setInstalledPlugin(resp);
        nextStep();
      })
    );
  }

  function handleImport(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    if (!importAllUserGroups) {
      const filters = groupFilters.map(o => o.value);
      formData.set(FormDataField.GroupFilters, JSON.stringify(filters));
    } else {
      // Passing in an empty field is equivalent to asteriks.
      formData.delete(FormDataField.GroupFilters);
    }

    if (!importAllApps) {
      const filters = appFilters.map(o => o.value);
      formData.set(FormDataField.AppFilters, JSON.stringify(filters));
    } else {
      // Passing in an empty field is equivalent to asteriks.
      formData.delete(FormDataField.AppFilters);
    }

    const defaultOwners = selectedOwners.map(o => o.label);
    formData.set(FormDataField.DefaultOwners, JSON.stringify(defaultOwners));

    importRun(() =>
      pluginsService.createPlugin(formData).then(resp => {
        setInstalledPlugin(resp);
        nextStep();
      })
    );
  }

  function handleImportToggle(filterKind: FormDataFilterField) {
    switch (filterKind) {
      case FormDataField.AppFilters:
        if (!importAllApps) {
          formData.delete(filterKind);
          // re-fetch list of all apps
          filterAppsRun(() =>
            pluginsService.getPluginConfigOktaApps(formData).then(setApps)
          );
        }
        setImportAllApps(b => !b);
        setAppFilters([]);
        break;

      case FormDataField.GroupFilters:
        if (!importAllUserGroups) {
          formData.delete(filterKind);
          // re-fetch list of all user groups
          filterGroupsRun(() =>
            pluginsService
              .getPluginConfigOktaGroups(formData)
              .then(setUserGroups)
          );
        }
        setImportAllUserGroups(b => !b);
        setGroupFilters([]);
        break;
      default:
        assertUnreachable(filterKind);
    }
  }

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

  function updateFilters<T>({
    applyFilters,
    setAppliedFiltersResp,
    setFilterAttempt,
    setFilter,
    formDataFilterField,
    validator,
    updatedFilters,
  }: {
    applyFilters(formData: FormData): Promise<T>;
    setAppliedFiltersResp(results: T): void;
    setFilterAttempt: AttemptState['setAttempt'];
    setFilter(f: FilterOption[]): void;
    formDataFilterField: FormDataFilterField;
    validator: Validator;
    updatedFilters: FilterOption[];
  }) {
    updatedFilters = updatedFilters || [];

    const filters = updatedFilters.map(o => o.value);
    setFilter(updatedFilters);
    formData.set(formDataFilterField, JSON.stringify(filters));

    setFilterAttempt({ status: 'processing' });

    // Get new filtered results.
    applyFilters(formData)
      .then(resp => {
        setFilterAttempt({ status: 'success' });
        setAppliedFiltersResp(resp);
      })
      .catch(async (e: Error) => {
        if (e instanceof ApiError && e.response.status === 400) {
          try {
            // Check if a filter is mentioned in the error message.
            // Any offending filter will be returned as part of
            // error message.
            let foundInvalidFilter = false;
            const errMessages = e.messages.join(' ');
            const markedFilters = updatedFilters.map(f => {
              if (
                e.message.includes(f.value) ||
                errMessages.includes(f.value)
              ) {
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
          } catch {
            // let it fall under other errors.
          }
        }
        // All other errors, fail as normal.
        setFilterAttempt({ status: 'failed', statusText: e.message });
      });
  }

  return (
    <Box width="980px">
      <Validation>
        {({ validator }) => (
          <>
            <Box mb={3}>
              <Header header="Configure Syncing of User Groups and Direct Assignments" />
              <Text>
                Sync user groups and direct assignments with Teleport Access
                Lists
              </Text>
            </Box>
            <Flex mb={1} mt={4} flexDirection="column" gap={4} width="100%">
              <StyledBox>
                <H2 mb={1}>
                  Step 1: Set Default List Owner(s) for Access Lists
                </H2>
                <Text>
                  List Owners are responsible for periodically reviewing
                  membership to each Access List. You must assign at least 1
                  default owner to your imported access lists. You can edit
                  owners on individual lists, later.
                </Text>
                {fetchUserAttempt.status === 'failed' && (
                  <Box mt={2}>
                    <FailedAttempt
                      attempt={fetchUserAttempt}
                      retry={() => fetchUsers()}
                    />
                  </Box>
                )}
                {fetchUserAttempt.status !== 'processing' ? (
                  <Box width="540px" mt={2}>
                    <FieldSelectCreatable
                      autoFocus={true}
                      placeholder="Type a username and press enter"
                      isMulti
                      isClearable
                      isSearchable
                      options={userOptions}
                      isDisabled={importAttempt.status === 'processing'}
                      onChange={(opts: Option<User>[]) =>
                        setSelectedOwners(opts)
                      }
                      value={selectedOwners || []}
                      noOptionsMessage={() => 'Type a username and press enter'}
                      label="Add Default List Owner(s)"
                      rule={requiredField(
                        'At least 1 default owner is required'
                      )}
                    />
                  </Box>
                ) : (
                  <Box m={4} textAlign="center">
                    <Indicator delay="none" />
                  </Box>
                )}
              </StyledBox>

              {/* User groups */}
              <StyledBox>
                <Flex justifyContent="space-between" alignItems="center" mb={2}>
                  <H2 mb={1}>
                    Step 2: Import User Groups with Direct Assignments as Access
                    Lists
                  </H2>
                  <Toggle
                    isToggled={importAllUserGroups}
                    onToggle={() =>
                      handleImportToggle(FormDataField.GroupFilters)
                    }
                    disabled={
                      importAttempt.status === 'processing' ||
                      filterGroupsAttempt.status === 'processing'
                    }
                  >
                    <Box ml={2}>
                      Import All User Groups with Direct Assignments
                    </Box>
                  </Toggle>
                </Flex>
                <Text mb={2}>
                  Teleport will import and sync any Okta user groups with direct
                  assignments to Teleport Access Lists.
                  <br />
                  <b>Note:</b> You will not see a user group if there are no
                  direct assignments.
                </Text>
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
                {!importAllUserGroups && (
                  <CreateFilters
                    filters={groupFilters}
                    validator={validator}
                    filterKind={FormDataField.GroupFilters}
                    onFilterChange={handleFilterOnChange}
                    importAttempt={importAttempt}
                    filterAttempt={filterGroupsAttempt}
                  />
                )}
                <UserGroupsTable
                  userGroups={userGroups}
                  loading={filterGroupsAttempt.status === 'processing'}
                />
              </StyledBox>

              {/* Apps */}
              <StyledBox>
                <Flex justifyContent="space-between" alignItems="center">
                  <H2 mb={1}>
                    Step 3: Import Apps with Direct Assignments as Access Lists
                  </H2>
                  <Toggle
                    isToggled={importAllApps}
                    onToggle={() =>
                      handleImportToggle(FormDataField.AppFilters)
                    }
                    disabled={
                      importAttempt.status === 'processing' ||
                      filterAppsAttempt.status === 'processing'
                    }
                  >
                    <Box ml={2}>Import All Apps with Direct Assignments</Box>
                  </Toggle>
                </Flex>
                <Text mb={2}>
                  Teleport will import and sync any Okta applications with
                  direct assignments to Teleport Access Lists.
                  <br />
                  <b>Note:</b> You will not see an application if there are no
                  direct assignments.
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
                {!importAllApps && (
                  <CreateFilters
                    filters={appFilters}
                    validator={validator}
                    filterKind={FormDataField.AppFilters}
                    onFilterChange={handleFilterOnChange}
                    importAttempt={importAttempt}
                    filterAttempt={filterAppsAttempt}
                  />
                )}
                <AppTable
                  apps={apps}
                  loading={filterAppsAttempt.status === 'processing'}
                />
              </StyledBox>
            </Flex>
            {importAttempt.status === 'failed' && (
              <Alert mt={3} kind="danger" children={importAttempt.statusText} />
            )}
            <Flex mt={4} mb={5} justifyContent="space-between">
              <Flex gap={2}>
                <ButtonPrimary
                  onClick={() => handleImport(validator)}
                  disabled={importAttempt.status === 'processing'}
                >
                  Next
                </ButtonPrimary>

                <ButtonSecondary onClick={() => prevStep()}>
                  Back
                </ButtonSecondary>
              </Flex>
              <ButtonSecondary
                onClick={() => handleSkipStep()}
                disabled={importAttempt.status === 'processing'}
              >
                Skip
              </ButtonSecondary>
            </Flex>
          </>
        )}
      </Validation>
    </Box>
  );
}

export const StyledBox = styled(Box).attrs({
  p: 4,
  borderRadius: 3,
  maxWidth: '1121px',
})`
  background-color: ${props => props.theme.colors.levels.elevated};
  box-shadow: 0px 3px 1px -2px #00000033;
`;
