import { useCallback, useEffect, useState } from 'react';
import {
  Box,
  ButtonPrimary,
  ButtonSecondary,
  ButtonText,
  Flex,
  Indicator,
  Text,
} from 'design';
import * as Icons from 'design/Icon';
import { FieldSelectCreatable } from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import { useAsync, Attempt } from 'shared/hooks/useAsync';
import { StyledBox } from 'teleport/Discover/Shared';
import { Danger } from 'design/Alert';
import userService, { User } from 'teleport/services/user';
import useTeleport from 'teleport/useTeleport';

import { pluginsService } from 'e-teleport/services/plugins';
import { usePlugin } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/usePlugin';
import { Header } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Shared';
import {
  PluginConfigAwsIc,
  AwsIcResourceTypes,
} from 'e-teleport/services/plugins/types';

import {
  AccountsTable,
  GroupsWithAssignmentTable,
  PermissionSetsTable,
} from './ResourceTable';

import type {
  AwsIcAccounts,
  AwsIcPermissionSets,
  AwsIcGroupsWithAssignment,
} from 'e-teleport/services/plugins/types';

export function AwsIcImportResources() {
  const ctx = useTeleport();
  const userAccess = ctx.storeUser.getUserAccess();
  const canReadListUsers = userAccess.list && userAccess.read;
  const { formData, nextStep, prevStep } = usePlugin();
  const integrationName = formData
    .get(PluginConfigAwsIc.OidcIntegrationName)
    ?.toString();
  const arn = formData.get(PluginConfigAwsIc.InstanceArn)?.toString();
  const region = formData.get(PluginConfigAwsIc.InstanceRegion)?.toString();
  const {
    fetchAccountsAttempt,
    fetchGroupsWithPermAssignmentsAttempt,
    fetchPermissionSetsAttempt,
    fetchResources,
  } = useImportResources({ integrationName, arn, region });

  const [showTable, setShowTable] = useState<tableType>(defaultTableStates);
  const [selectedOwners, setSelectedOwners] = useState<userOption[]>([]);

  const [fetchUsersAttempt, runFetchUsers] = useAsync(
    useCallback(async () => {
      const resp = await userService.fetchUsers();
      return resp.map(u => ({ value: u, label: u.name }));
    }, [])
  );
  useEffect(() => {
    if (canReadListUsers && fetchUsersAttempt.status === '') {
      runFetchUsers();
    }
  }, [runFetchUsers, fetchUsersAttempt, canReadListUsers]);

  function handleNext(v: Validator) {
    if (!v.validate()) {
      return;
    }

    const defaultOwners = selectedOwners.map(o => o.label);
    formData.append(
      PluginConfigAwsIc.AccessListDefaultOwners,
      JSON.stringify(defaultOwners)
    );
    nextStep();
  }

  return (
    <Box width="800px">
      <Validation>
        {({ validator }) => (
          <>
            <Box mb={3}>
              <Header header={headerText} />
              <Text> {subheaderText} </Text>
            </Box>
            <Flex mb={1} mt={4} flexDirection="column" gap={4} width="100%">
              <Account
                accounts={fetchAccountsAttempt.data || []}
                showTable={showTable.accounts}
                setShowTable={state => {
                  if (!showTable.accounts) {
                    fetchResources(AwsIcResourceTypes.Accounts);
                  }
                  setShowTable({ ...showTable, accounts: state });
                }}
                loading={fetchAccountsAttempt.status === 'processing'}
              />

              <GroupsWithAssignment
                groupsWithPermissionAssignment={
                  fetchGroupsWithPermAssignmentsAttempt.data || []
                }
                showTable={showTable.groups}
                setShowTable={state => {
                  if (!showTable.groups) {
                    fetchResources(AwsIcResourceTypes.GroupsWithAssignments);
                  }
                  setShowTable({ ...showTable, groups: state });
                }}
                selectedOwners={selectedOwners}
                setSelectedOwners={setSelectedOwners}
                fetchUsersAttempt={fetchUsersAttempt}
                loading={
                  fetchGroupsWithPermAssignmentsAttempt.status === 'processing'
                }
              />

              <PermissionSets
                permissionSets={fetchPermissionSetsAttempt.data || []}
                showTable={showTable.permissionSets}
                setShowTable={state => {
                  if (!showTable.permissionSets) {
                    fetchResources(AwsIcResourceTypes.PermissionSets);
                  }
                  setShowTable({ ...showTable, permissionSets: state });
                }}
                loading={fetchPermissionSetsAttempt.status === 'processing'}
              />
            </Flex>
            <Flex mt={5} mb={5} gap={3}>
              <ButtonPrimary
                onClick={() => handleNext(validator)}
                disabled={selectedOwners.length === 0}
              >
                Next
              </ButtonPrimary>

              {/* TODO(sshah): delete previously created integration on back? */}
              <ButtonSecondary onClick={prevStep}>Back</ButtonSecondary>
            </Flex>
          </>
        )}
      </Validation>
    </Box>
  );
}

const headerText =
  'Preview AWS IAM Identity Center resources that will be imported to Teleport';
const subheaderText = `Configure default access list owner and preview accounts, user
                       groups and permission sets and permission assignments that will
                       be imported to Teleport.`;

export const Account = ({
  accounts,
  showTable,
  setShowTable,
  loading = false,
}: {
  accounts: AwsIcAccounts[];
  showTable: boolean;
  setShowTable: (boolean) => void;
  loading: boolean;
}) => (
  <StyledBox>
    <Flex justifyContent="space-between" alignItems="center">
      <Flex
        flexDirection={'column'}
        justifyContent="flex-start"
        alignItems="flext-start"
        maxWidth={'520px'}
      >
        <Text bold mb={1}>
          Accounts
        </Text>
        <Text mb={2}>
          All accounts will be imported and maintained in Teleport as an AWS
          application.
        </Text>
      </Flex>

      <ButtonText inputAlignment onClick={() => setShowTable(!showTable)}>
        {showTable ? 'Hide' : 'Show'} Accounts
        {showTable ? (
          <Icons.ChevronUp ml={2} size={'small'} />
        ) : (
          <Icons.ChevronDown ml={2} size={'small'} />
        )}
      </ButtonText>
    </Flex>

    {showTable && <AccountsTable accounts={accounts} loading={loading} />}
  </StyledBox>
);

const GroupDescription = () => (
  <>
    <Text bold mb={1}>
      Groups
    </Text>
    <Text mb={2}>
      All groups will be imported and maintained in Teleport as an Access List.
      Each account assignment with respective permission sets will be maintained
      as a role and assigned to the Access List.
    </Text>
  </>
);

const AccessListDescription = () => (
  <>
    <Text bold mb={1}>
      Default Access List Owner(s)
    </Text>
    <Text>
      List Owners are responsible for periodically reviewing membership to each
      Access List.
      <br />
      You must select at least one default owner. You may add or remove
      additional owners
      <br />
      on individual lists, later.
    </Text>
  </>
);

export const GroupsWithAssignment = ({
  groupsWithPermissionAssignment,
  showTable,
  setShowTable,
  fetchUsersAttempt,
  selectedOwners,
  setSelectedOwners,
  loading,
}: {
  groupsWithPermissionAssignment: AwsIcGroupsWithAssignment[];
  showTable: boolean;
  setShowTable: (boolean) => void;
  fetchUsersAttempt: Attempt<Option<User>[]>;
  selectedOwners: userOption[];
  setSelectedOwners: (string) => void;
  loading: boolean;
}) => (
  <StyledBox>
    <Flex justifyContent="space-between" alignItems="center">
      <Flex
        flexDirection={'column'}
        justifyContent="flex-start"
        alignItems="flext-start"
        maxWidth={'520px'}
      >
        <GroupDescription />
      </Flex>

      <ButtonText inputAlignment onClick={() => setShowTable(!showTable)}>
        {showTable ? 'Hide' : 'Show'} Groups
        {showTable ? (
          <Icons.ChevronUp ml={2} size={'small'} />
        ) : (
          <Icons.ChevronDown ml={2} size={'small'} />
        )}
      </ButtonText>
    </Flex>

    {showTable && (
      <GroupsWithAssignmentTable
        userGroups={groupsWithPermissionAssignment}
        loading={loading}
      />
    )}

    <Box mt={4}>
      <AccessListDescription />
      {fetchUsersAttempt.status === 'error' && (
        <Box mt={2}>
          <Danger>{fetchUsersAttempt.statusText}</Danger>
        </Box>
      )}
      {fetchUsersAttempt.status === 'processing' ? (
        <Box m={4} textAlign="center">
          <Indicator delay="none" />
        </Box>
      ) : (
        <Box width="540px" mt={2}>
          <FieldSelectCreatable
            autoFocus={true}
            placeholder="Type a username and press enter"
            isMulti
            isClearable
            isSearchable
            options={fetchUsersAttempt.data}
            onChange={(opts: Option<User>[]) => setSelectedOwners(opts)}
            value={selectedOwners || []}
            noOptionsMessage={() => 'Type a username and press enter'}
            label="Access List Owner(s)*"
            rule={requiredField('At least 1 default owner is required')}
            isDisabled={fetchUsersAttempt.status === 'error'}
          />
        </Box>
      )}
    </Box>
  </StyledBox>
);

const PermisionSetDescription = () => (
  <>
    <Text bold mb={1}>
      Permission Set
    </Text>
    <Text mb={2}>
      All permission sets will be imported and maintained in Teleport as a role.
    </Text>
  </>
);

export const PermissionSets = ({
  permissionSets,
  showTable,
  setShowTable,
  loading,
}: {
  permissionSets: AwsIcPermissionSets[];
  showTable: boolean;
  setShowTable: (boolean) => void;
  loading: boolean;
}) => (
  <StyledBox>
    <Flex justifyContent="space-between" alignItems="center">
      <Flex
        flexDirection={'column'}
        justifyContent="flex-start"
        alignItems="flext-start"
        maxWidth={'520px'}
      >
        <PermisionSetDescription />
      </Flex>

      <ButtonText inputAlignment onClick={() => setShowTable(!showTable)}>
        {showTable ? 'Hide' : 'Show'} Permission Sets
        {showTable ? (
          <Icons.ChevronUp ml={2} size={'small'} />
        ) : (
          <Icons.ChevronDown ml={2} size={'small'} />
        )}
      </ButtonText>
    </Flex>

    {showTable && (
      <PermissionSetsTable permissionSets={permissionSets} loading={loading} />
    )}
  </StyledBox>
);

export type userOption = Option<User>;

export type tableType = {
  accounts: boolean;
  groups: boolean;
  directAssignments: boolean;
  permissionSets: boolean;
};

export const defaultTableStates = {
  accounts: false,
  groups: false,
  directAssignments: false,
  permissionSets: false,
};

export function useImportResources({
  integrationName,
  arn,
  region,
}: {
  integrationName: string;
  arn: string;
  region: string;
}) {
  const [fetchAccountsAttempt, runFetchAccounts] = useAsync(async () => {
    return await pluginsService.getAwsIcAccounts({
      integrationName,
      arn,
      region,
    });
  });

  const [
    fetchGroupsWithPermAssignmentsAttempt,
    runFetchGroupsWithPermAssignments,
  ] = useAsync(async () => {
    return await pluginsService.getAwsIcGroupsWithPermissionAssignments({
      integrationName,
      arn,
      region,
    });
  });

  const [fetchPermissionSetsAttempt, runFetchPermissionSets] = useAsync(
    async () => {
      return await pluginsService.getAwsIcPermissionSets({
        integrationName,
        arn,
        region,
      });
    }
  );

  function fetchResources(resourceType: string) {
    switch (resourceType) {
      case 'accounts':
        if (fetchAccountsAttempt.status === '') {
          runFetchAccounts();
        }
        break;
      case 'groupsWithAssignments':
        if (fetchGroupsWithPermAssignmentsAttempt.status === '') {
          runFetchGroupsWithPermAssignments();
        }
        break;
      case 'permissionSets':
        if (fetchPermissionSetsAttempt.status === '') {
          runFetchPermissionSets();
        }
        break;
      default:
        return;
    }
  }

  return {
    fetchAccountsAttempt,
    runFetchAccounts,
    fetchGroupsWithPermAssignmentsAttempt,
    runFetchGroupsWithPermAssignments,
    fetchPermissionSetsAttempt,
    runFetchPermissionSets,
    fetchResources,
  };
}
