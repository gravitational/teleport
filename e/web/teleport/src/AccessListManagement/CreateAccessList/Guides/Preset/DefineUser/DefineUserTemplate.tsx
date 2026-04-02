import { useQuery } from '@tanstack/react-query';
import { useMemo, useState } from 'react';

import { Alert, Box, ButtonIcon, Flex, H1, Indicator, Text } from 'design';
import { ChevronDown, ChevronRight } from 'design/Icon';
import { FieldSelect } from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import Validation, { Validator } from 'shared/components/Validation';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import {
  GuideContent,
  StepButtons,
} from 'e-teleport/AccessListManagement/GuideEditor/Shared';
import { MemberSelection } from 'e-teleport/AccessListManagement/Shared/Shared';
import {
  convertTraitLabelsToAllUserTraits,
  TraitLabel,
  TraitsCreator,
} from 'e-teleport/AccessListManagement/Traits';
import { useFetch } from 'e-teleport/AccessListManagement/useFetch';
import {
  AccessListMemberKind,
  AccessListOrigin,
  accessManagementService,
} from 'e-teleport/services/accessmanagement';
import cfg from 'teleport/config';
import {
  AccessListEvent,
  AccessListIntegrateEvent,
  AccessListStepStatusEvent,
} from 'teleport/services/userEvent/accessListEvents';

import { useCreateAccessList } from '../../../CreateAccessListContextProvider';
import {
  EligibilityOrGrantRolesFieldSelectAndCreate,
  EligibleUsersFieldSelect,
} from '../../../Shared';
import {
  EligibleUsersFieldSelectProps,
  UserTypeOption,
  userTypeOptions,
} from '../../../types';
import { CollapsibleAccessListTypeInfo } from '../../Shared/CollapsibleAccessListTypeInfo';

export function DefineUserTemplate({
  userKind,
  isLastStep,
}: {
  isLastStep: boolean;
  userKind: 'member' | 'owner';
}) {
  const { fetchUsersOptions, fetchAccessListsOptions } = useFetch();

  const {
    onCreate,
    createAttempt,
    owners,
    setOwners,
    members,
    setMembers,
    getResumableAccessListState,
  } = useCreateAccessList();

  const { guideEditor, oktaPluginAttempt } = useAccessListManagementContext();
  const {
    nextStep,
    preset,
    currentStep,
    originatedFromOkta,
    removeLocationState,
    emitEvent,
  } = guideEditor;

  const [userType, setUserType] = useState<UserTypeOption>(() => {
    return userTypeOptions.find(m => m.value === 'access-lists');
  });

  /**
   * When unique, triggers the react select async component to
   * re-load it's initial options.
   */
  const [rerenderKey, setRerenderKey] = useState('');

  const [expanded, setExpanded] = useState(false);
  const ArrowIcon = expanded ? ChevronDown : ChevronRight;

  function updateSelectedUsers(
    vals: Option<MemberSelection>[],
    membershipKind: AccessListMemberKind
  ) {
    // if we "create" a user that doesnt exist, the value comes back as a
    // string rather than the formatted value we need. we can update it here.
    const formattedVals = vals.map(val =>
      typeof val.value === 'string' &&
      membershipKind === AccessListMemberKind.User
        ? { label: val.label, value: { membershipKind, name: val.value } }
        : val
    );

    switch (userKind) {
      case 'member':
        setMembers({
          ...members,
          selectedMembers: [...formattedVals],
        });
        break;

      case 'owner':
        setOwners({
          ...owners,
          selectedOwners: [...formattedVals],
        });
        break;
    }
  }

  const requiredErrMsg = useMemo(() => {
    let selectedUsers: Option<MemberSelection>[] = [];
    switch (userKind) {
      case 'member':
        selectedUsers = members.selectedMembers;
        break;
      case 'owner':
        selectedUsers = owners.selectedOwners;
        break;
    }

    if (!selectedUsers.length) {
      return 'Please select at least one user or access list.';
    }

    return '';
  }, [owners.selectedOwners, members.selectedMembers, userKind]);

  let capitalizedUserKind = '';
  let selectedUsers: Option<MemberSelection>[] = [];
  let selectedRolesRequired: Option[] = [];
  let traitLabels: TraitLabel[] = [];
  let headerText = '';
  let HeaderDesc;

  switch (userKind) {
    case 'member':
      capitalizedUserKind = 'Member';
      selectedUsers = members.selectedMembers;
      selectedRolesRequired = members.selectedRolesRequired;
      traitLabels = members.traitLabels;

      headerText =
        preset === 'short-term'
          ? 'Who should be required to request access?'
          : 'Who are you setting up access for?';

      HeaderDesc = (
        <Box mb={5}>
          {preset === 'short-term' ? (
            <>
              <Text mb={2}>
                Members defined here will be made required to request for access
                to resources defined by this guide.
              </Text>
              <Text>
                Upon approval of their requests, access duration depends on the
                duration requested by member in which maximum duration is capped
                to their remaining web session TTL.
              </Text>
            </>
          ) : (
            <>
              Members defined here will be given access to Teleport resources
              defined by this guide.
            </>
          )}
        </Box>
      );
      break;

    case 'owner':
      capitalizedUserKind = 'Owner';
      selectedUsers = owners.selectedOwners;
      selectedRolesRequired = owners.selectedRolesRequired;
      traitLabels = owners.traitLabels;

      headerText =
        preset === 'short-term'
          ? 'Who should review access requests?'
          : 'Who should periodically review and audit memberships?';

      HeaderDesc = (
        <Text mb={4}>
          List Owners are also responsible for managing members and membership
          requirements for this access list, and must conduct periodic access
          reviews.
        </Text>
      );
      break;
  }

  let eligibleUsersProps: EligibleUsersFieldSelectProps;
  if (userType.value === 'access-lists') {
    eligibleUsersProps = {
      userKind: 'nested-access-list',
      disableCreate: true,
      selected: selectedUsers || [],
      isDisabled: false,
      onChange: vals =>
        updateSelectedUsers(vals || [], AccessListMemberKind.List),

      loadOptions: fetchAccessListsOptions,
      placeholder: 'Search for an access list…',
      noOptionsMsg: 'No access lists found.',
      label: `Add Access Lists as ${capitalizedUserKind}s`,
      requiredErrMsg: requiredErrMsg,
    };
  } else if (userType.value === 'users') {
    eligibleUsersProps = {
      selected: selectedUsers || [],
      isDisabled: false,
      onChange: vals =>
        updateSelectedUsers(vals || [], AccessListMemberKind.User),
      loadOptions: fetchUsersOptions,
      placeholder: 'Search for a user…',
      label: `Add ${capitalizedUserKind}s`,
      requiredErrMsg: requiredErrMsg,
    };
  }

  function onEligibilityChange(option: Option[]) {
    switch (userKind) {
      case 'member':
        setMembers({
          ...members,
          selectedRolesRequired: option || [],
        });
        break;
      case 'owner':
        setOwners({
          ...owners,
          selectedRolesRequired: option || [],
        });
        break;
      default:
        userKind satisfies never;
    }
  }

  function onTraitChange(traitLabels: TraitLabel[]) {
    switch (userKind) {
      case 'member':
        setMembers({
          ...members,
          traitLabels,
          traitLookup: convertTraitLabelsToAllUserTraits(traitLabels),
        });
        break;
      case 'owner':
        setOwners({
          ...owners,
          traitLabels,
          traitLookup: convertTraitLabelsToAllUserTraits(traitLabels),
        });
        break;
      default:
        userKind satisfies never;
    }
  }

  async function handleNext(validator: Validator) {
    if (isLastStep) {
      const created = await onCreate(validator);
      if (created) {
        nextStep();
        emitEvent({
          stepStatus: AccessListStepStatusEvent.Success,
        });
      }
      return;
    }

    if (!validator.validate()) {
      return;
    }
    nextStep();
    emitEvent({
      stepStatus: AccessListStepStatusEvent.Success,
    });
  }

  const oktaOriginatedAccessLists = useQuery({
    queryKey: ['get', 'okta', 'access-lists'],
    queryFn: async () => {
      const resp = await accessManagementService.fetchAccessListsV2({
        origin: 'okta',
        /**
         * TODO(kimlisa): Remove "search: okta" field by v20. The origin
         * filter param was added to proxy v18.7 and should be the standard
         * when querying for okta originated access lists.
         */
        search: 'okta',
        /**
         * TODO(kimlisa): Remove limit by v20. Limit was increased to
         * increase the chance that the access lists returned includes okta
         * originated access lists from proxy version <18.7 (where origin is
         * not supported). This is an attempt to handle the edge case where an
         * access list can have "search keywords" that includes okta but is NOT
         * originated from okta (unlikely that all 100 lists will end up not
         * okta origin)
         */
        limit: 100,
      });
      const gotAccessLists = resp.agents;
      /**
       * TODO(kimlisa): Remove "if" by v20. Unnecessary check since by v20 the
       * origin query filter has full support.
       */
      if (gotAccessLists.some(al => al.origin === AccessListOrigin.Okta)) {
        setRerenderKey(crypto.randomUUID());
        removeLocationState();
        return gotAccessLists;
      }
      return [];
    },
    gcTime: 0,
    enabled: originatedFromOkta,
  });

  // Render loader only on the first query.
  if (originatedFromOkta && oktaOriginatedAccessLists.isLoading) {
    return (
      <GuideContent withMaxWidth>
        <H1>
          Step {currentStep + 1}: {headerText}
        </H1>
        {HeaderDesc}
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      </GuideContent>
    );
  }

  return (
    <Validation>
      {({ validator }) => (
        <>
          <GuideContent withMaxWidth>
            <H1>
              Step {currentStep + 1}: {headerText}
            </H1>

            {HeaderDesc}

            {originatedFromOkta &&
              oktaOriginatedAccessLists.isSuccess &&
              !oktaOriginatedAccessLists.data?.length && (
                <Alert
                  mb={6}
                  kind="info"
                  wrapContents
                  secondaryAction={{
                    content: 'Go back to Okta Status Page',
                    linkTo: cfg.getIntegrationStatusRoute('okta', 'okta'),
                  }}
                  primaryAction={{
                    content: 'Reload List',
                    onClick: () => oktaOriginatedAccessLists.refetch(),
                  }}
                >
                  Either your Okta integration has not finished syncing Apps and
                  User Groups as access lists or there was nothing to sync. The
                  sync can take about a minute.
                </Alert>
              )}

            {oktaOriginatedAccessLists.error && (
              <Alert
                primaryAction={{
                  content: 'Retry',
                  onClick: () => oktaOriginatedAccessLists.refetch(),
                }}
              >
                {oktaOriginatedAccessLists.error.message}
              </Alert>
            )}

            {createAttempt.status === 'failed' && (
              <Alert>{createAttempt.statusText}</Alert>
            )}

            <Flex gap={2}>
              <FieldSelect
                width="29%"
                label={`${capitalizedUserKind} Type`}
                value={userType}
                onChange={o => setUserType(o)}
                options={userTypeOptions}
                placeholder={`${capitalizedUserKind} Type`}
                mb={3}
              />
              <Box width="71%">
                <EligibleUsersFieldSelect
                  key={rerenderKey}
                  {...eligibleUsersProps}
                />
              </Box>
            </Flex>

            <CollapsibleAccessListTypeInfo
              userTypeOption={userType}
              userCategory={userKind}
              resumableState={{
                ...guideEditor.getResumableState(),
                ...getResumableAccessListState(),
              }}
              okta={{
                hasPlugin: Boolean(oktaPluginAttempt?.data),
                hasAppGroupSyncEnabled:
                  oktaPluginAttempt?.data?.spec?.enableAccessListSync,
                hasConfiguredOauthCredentials:
                  oktaPluginAttempt?.data?.spec?.credentialsInfo
                    ?.hasConfiguredOauthCredentials,
                emitEvent: () =>
                  emitEvent({
                    event: AccessListEvent.Integrate,
                    integrate: AccessListIntegrateEvent.Okta,
                    stepStatus: AccessListStepStatusEvent.Success,
                  }),
              }}
            />

            <Box mt={8} mb={6}>
              <Flex
                width="240px"
                flexWrap="wrap"
                gap={2}
                alignItems="center"
                onClick={() => setExpanded(e => !e)}
                css={`
                  cursor: pointer;
                `}
              >
                <Text bold color="text.slightlyMuted">
                  Optional Advanced Settings
                </Text>
                <ButtonIcon>
                  <ArrowIcon size={16} />
                </ButtonIcon>
              </Flex>
              {expanded && (
                <Box>
                  <Text mb={3} mt={2}>
                    If a Teleport user is assigned as a {userKind} but does not
                    have all required roles and traits defined in this section,{' '}
                    {userKind}ship will have no effect. They will not be granted
                    any additional roles or traits by this list.
                  </Text>
                  <EligibilityOrGrantRolesFieldSelectAndCreate
                    userKind={userKind === 'member' ? 'Members' : 'Owners'}
                    rolesSelectedFor="eligibility"
                    optional={true}
                    isDisabled={false}
                    onChange={onEligibilityChange}
                    selected={selectedRolesRequired}
                  />
                  <Box mb={3}>
                    <TraitsCreator
                      kind={userKind === 'member' ? 'Member' : 'Owner'}
                      traitLabels={traitLabels}
                      isDisabled={false}
                      updateTraitLabels={onTraitChange}
                    />
                  </Box>
                </Box>
              )}
            </Box>
          </GuideContent>
          <StepButtons
            onNext={() => handleNext(validator)}
            disabled={createAttempt.status === 'processing'}
          />
        </>
      )}
    </Validation>
  );
}
