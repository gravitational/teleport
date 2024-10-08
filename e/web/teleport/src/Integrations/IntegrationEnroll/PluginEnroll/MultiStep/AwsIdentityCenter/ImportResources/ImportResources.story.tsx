import { useState } from 'react';
import { Box, ButtonSecondary, Flex, Text } from 'design';
import { MemoryRouter } from 'react-router';
import { ContextProvider } from 'teleport';
import { User } from 'teleport/services/user';
import Validation from 'shared/components/Validation';
import { createTeleportContext } from 'teleport/mocks/contexts';

import {
  Account,
  DirectAssignments as DirectAssignmentsComponent,
  GroupsWithAssignment as GroupsWithAssignmentComponent,
  AwsIcImportResources,
  PermissionSets as PermissionSetsComponent,
} from './ImportResources';

import type {
  PluginConfigAwsIcAccounts,
  PluginConfigAwsIcPermissionSetsTable,
  PluginConfigAwsIcUserDirectAssignment,
  PluginConfigAwsIcUserGroupsWithAssignment,
} from 'e-teleport/services/plugins/types';

export default {
  title: 'TeleportE/Integrations/Enroll/AWSIdentityCenter/ImportResources',
};

export const ImportResources = () => {
  const ctx = createTeleportContext();
  ctx.userService.fetchUsers = () => Promise.resolve<User[]>(users);
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <AwsIcImportResources
          accounts={accounts}
          groupsWithPermissionAssignment={groupsWithPermissionAssignment}
          userDirectPermissionAssignment={userDirectPermissionAssignment}
          permissionSets={permissionSets}
        />
      </ContextProvider>
    </MemoryRouter>
  );
};

export const Accounts = () => {
  const [showTable, setShowTable] = useState(false);
  const accountProps = {
    accounts: accounts,
    showTable: showTable,
    setShowTable: setShowTable,
    loading: false,
  };
  return (
    <Flex flexDirection="column" gap={3}>
      <Box>
        <Text ml={1} bold>
          Accounts
        </Text>
        <Account
          {...accountProps}
          showTable={showTable}
          setShowTable={setShowTable}
        />
      </Box>

      <Box>
        <Text ml={1} bold>
          Empty accounts
        </Text>
        <AccountsEmpty />
      </Box>

      <Box>
        <Text ml={1} bold>
          Accounts loading
        </Text>

        <AccountsLoading />
      </Box>
    </Flex>
  );
};

const AccountsEmpty = () => {
  const [showTable, setShowTable] = useState(true);
  return (
    <Account
      accounts={[]}
      showTable={showTable}
      setShowTable={setShowTable}
      loading={false}
    />
  );
};

const AccountsLoading = () => {
  const [showTable, setShowTable] = useState(true);
  return (
    <Account
      accounts={[]}
      showTable={showTable}
      setShowTable={setShowTable}
      loading={true}
    />
  );
};

export const GroupsWithAssignment = () => {
  const [showTable, setShowTable] = useState(false);
  const [selectedOwners, setSelectedOwners] = useState([]);
  const props = {
    groupsWithPermissionAssignment: groupsWithPermissionAssignment,
    showTable: showTable,
    setShowTable: setShowTable,
    loading: false,
    fetchUsersAttempt: {
      status: 'success' as any,
      data: users.map(u => ({ value: u, label: u.name })),
      statusText: '',
    },
    selectedOwners: selectedOwners,
    setSelectedOwners: setSelectedOwners,
  };
  return (
    <Flex flexDirection="column" gap={3}>
      <Box>
        <Text ml={1} bold>
          Valid
        </Text>
        <Validation>
          <GroupsWithAssignmentComponent {...props} />
        </Validation>
      </Box>

      <Box>
        <Text ml={1} bold>
          Loading
        </Text>
        <GroupsLoading {...props} />
      </Box>

      <Box>
        <Text ml={1} bold>
          Empty groups
        </Text>
        <GroupsEmpty {...props} />
      </Box>

      <Box>
        <Text ml={1} bold>
          Default owner loading
        </Text>
        <GroupsWithDefaultOwnerLoading {...props} />
      </Box>

      <Box>
        <Text ml={1} bold>
          Default owner error
        </Text>
        <GroupsWithDefaultOwnerError {...props} />
      </Box>

      <Box>
        <Text ml={1} bold>
          Default owner validation
        </Text>
        <GroupsWithDefaultOwnerValidation {...props} />
      </Box>
    </Flex>
  );
};

const GroupsLoading = props => {
  const [showTable, setShowTable] = useState(true);
  return (
    <Validation>
      <GroupsWithAssignmentComponent
        {...props}
        showTable={showTable}
        setShowTable={setShowTable}
        groupsWithPermissionAssignment={[]}
        loading={true}
      />
    </Validation>
  );
};

const GroupsEmpty = props => {
  const [showTable, setShowTable] = useState(true);
  return (
    <Validation>
      <GroupsWithAssignmentComponent
        {...props}
        showTable={showTable}
        setShowTable={setShowTable}
        groupsWithPermissionAssignment={[]}
      />
    </Validation>
  );
};

const GroupsWithDefaultOwnerLoading = props => {
  const [showTable, setShowTable] = useState(true);
  return (
    <Validation>
      <GroupsWithAssignmentComponent
        {...props}
        showTable={showTable}
        setShowTable={setShowTable}
        fetchUsersAttempt={{ status: 'processing', data: null, statusText: '' }}
        loading={false}
      />
    </Validation>
  );
};

const GroupsWithDefaultOwnerError = props => {
  const [showTable, setShowTable] = useState(true);
  return (
    <Validation>
      <GroupsWithAssignmentComponent
        {...props}
        showTable={showTable}
        setShowTable={setShowTable}
        fetchUsersAttempt={{
          status: 'error',
          data: null,
          statusText: 'Failed to fetch users',
        }}
        loading={false}
      />
    </Validation>
  );
};

const GroupsWithDefaultOwnerValidation = props => {
  const [showTable, setShowTable] = useState(true);
  return (
    <Validation>
      {({ validator }) => (
        <>
          <GroupsWithAssignmentComponent
            {...props}
            selectedOwners={[]}
            showTable={showTable}
            setShowTable={setShowTable}
            loading={false}
          />
          <ButtonSecondary onClick={() => validator.validate()} mt={2}>
            Click to validate
          </ButtonSecondary>
        </>
      )}
    </Validation>
  );
};

export const DirectAssignments = () => {
  const [showTable, setShowTable] = useState(false);
  const props = {
    userDirectPermissionAssignment: userDirectPermissionAssignment,
    showTable: showTable,
    setShowTable: setShowTable,
    loading: false,
  };
  return (
    <Flex flexDirection="column" gap={3}>
      <Box>
        <Text ml={1} bold>
          Valid
        </Text>
        <DirectAssignmentsComponent {...props} />
      </Box>

      <Box>
        <Text ml={1} bold>
          Loading
        </Text>
        <DirectAssignmentLoading {...props} />
      </Box>

      <Box>
        <Text ml={1} bold>
          Empty direct assignments
        </Text>
        <DirectAssignmentEmpty {...props} />
      </Box>
    </Flex>
  );
};

const DirectAssignmentEmpty = props => {
  const [showTable, setShowTable] = useState(true);
  return (
    <DirectAssignmentsComponent
      {...props}
      showTable={showTable}
      setShowTable={setShowTable}
      userDirectPermissionAssignment={[]}
    />
  );
};

const DirectAssignmentLoading = props => {
  const [showTable, setShowTable] = useState(true);
  return (
    <DirectAssignmentsComponent
      {...props}
      loading={true}
      showTable={showTable}
      setShowTable={setShowTable}
    />
  );
};

export const PermissionSets = () => {
  const [showTable, setShowTable] = useState(false);
  const props = {
    permissionSets: permissionSets,
    showTable: showTable,
    setShowTable: setShowTable,
    loading: false,
  };
  return (
    <Flex flexDirection="column" gap={3}>
      <Box>
        <Text ml={1} bold>
          Valid
        </Text>
        <PermissionSetsComponent {...props} />
      </Box>

      <Box>
        <Text ml={1} bold>
          Loading
        </Text>
        <PermissionSetsLoading {...props} />
      </Box>

      <Box>
        <Text ml={1} bold>
          Empty groups
        </Text>
        <PermissionSetsEmpty {...props} />
      </Box>
    </Flex>
  );
};

const PermissionSetsLoading = props => {
  const [showTable, setShowTable] = useState(true);
  return (
    <PermissionSetsComponent
      {...props}
      loading={true}
      showTable={showTable}
      setShowTable={setShowTable}
    />
  );
};

const PermissionSetsEmpty = props => {
  const [showTable, setShowTable] = useState(true);
  return (
    <PermissionSetsComponent
      {...props}
      showTable={showTable}
      setShowTable={setShowTable}
      permissionSets={[]}
    />
  );
};

const users = [
  { name: 'access-user', roles: ['access'], authType: 'local' },
  { name: 'editor-user', roles: ['editor'], authType: 'local' },
  { name: 'auditor-user', roles: ['auditor'], authType: 'local' },
];

const accounts: PluginConfigAwsIcAccounts[] = [
  {
    name: 'dev-account',
    arn: 'arn:aws"organizations::1234567890:account/o-u0adfj/123456789',
    id: '719283048592',
  },
  {
    name: 'prod-account',
    arn: 'arn:aws"organizations::1234567890:account/o-u0adfj/123456789',
    id: '719283098752',
  },
  {
    name: 'stage-account',
    arn: 'arn:aws"organizations::1234567890:account/o-u0adfj/123456789',
    id: '719283041526',
  },
];

const groupsWithPermissionAssignment: PluginConfigAwsIcUserGroupsWithAssignment[] =
  [
    {
      groupname: 'group1',
      assignments: [
        {
          account_name: 'dev-account',
          permission_set_name: 'NetworkAdministrator',
        },
      ],
    },
    {
      groupname: 'group2',
      assignments: [
        { account_name: 'dev-account', permission_set_name: 'DevOps' },
      ],
    },
    {
      groupname: 'group3',
      assignments: [
        {
          account_name: 'stage-account',
          permission_set_name: 'SecurityAdministrator',
        },
      ],
    },
  ];

const userDirectPermissionAssignment: PluginConfigAwsIcUserDirectAssignment[] =
  [
    {
      username: 'user1',
      assignments: [
        {
          account_name: 'dev-account',
          permission_set_name: 'NetworkAdministrator',
        },
      ],
    },
    {
      username: 'user2',
      assignments: [
        { account_name: 'prod-account', permission_set_name: 'SuperAdmin' },
      ],
    },
    {
      username: 'user3',
      assignments: [
        { account_name: 'dev-account', permission_set_name: 'DevAccess' },
      ],
    },
  ];

const permissionSets: PluginConfigAwsIcPermissionSetsTable[] = [
  {
    name: 'AIOps',
    description: '',
    arn: 'arn:aws:sso:::permissionSet/ssoins-88345234523dfdd26a/ps-93cddc3fasdf99',
  },
  {
    name: 'NetworkAdministrator',
    description: 'Network admin role',
    arn: 'arn:aws:sso:::permissionSet/ssoins-88345234523dfdd26a/ps-93cs09dc3fasdf99',
  },
  {
    name: 'SuperAdmin',
    description: 'Super admin role',
    arn: 'arn:aws:sso:::permissionSet/ssoins-88345234523dfdd26a/ps-93cddc3faasd2299',
  },
];
