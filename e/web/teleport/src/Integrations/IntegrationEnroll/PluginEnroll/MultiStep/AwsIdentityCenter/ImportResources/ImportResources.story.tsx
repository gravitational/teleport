import { PropsWithChildren, useState } from 'react';
import { MemoryRouter } from 'react-router';

import { Box, ButtonSecondary, Flex, Text } from 'design';
import Validation from 'shared/components/Validation';

import {
  accounts,
  groupsWithPermissionAssignment,
  permissionSets,
  users,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/AwsIdentityCenter/shared/fixture';
import {
  PluginProvider,
  usePlugin,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/usePlugin';
import { pluginMap } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/plugins';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import type { CloudHostablePlugin } from 'e-teleport/services/plugins';
import { PluginConfigAwsIc } from 'e-teleport/services/plugins/types';
import { ContextProvider } from 'teleport';
import { User } from 'teleport/services/user';

import {
  Account,
  AwsIcImportResources,
  GroupsWithAssignment as GroupsWithAssignmentComponent,
  PermissionSets as PermissionSetsComponent,
} from './ImportResources';

export default {
  title: 'TeleportE/Integrations/Enroll/AWSIdentityCenter/ImportResources',
};

const awsIdentityCenterPlugin = pluginMap[
  PluginConfigAwsIc.PluginName
] as CloudHostablePlugin;

export const ImportResources = () => {
  const ctx = createTeleportContextE();
  ctx.userService.fetchUsersV2 = () =>
    Promise.resolve({ items: users as User[], startKey: '' });
  ctx.pluginsService.getAwsIcAccounts = () => Promise.resolve(accounts);
  ctx.pluginsService.getAwsIcGroupsWithPermissionAssignments = () =>
    Promise.resolve(groupsWithPermissionAssignment);
  ctx.pluginsService.getAwsIcPermissionSets = () =>
    Promise.resolve(permissionSets);
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <PluginProvider selectedPlugin={awsIdentityCenterPlugin}>
          <PluginContextWrapper>
            <AwsIcImportResources />
          </PluginContextWrapper>
        </PluginProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};

const PluginContextWrapper: React.FC<PropsWithChildren> = ({ children }) => {
  const { setFormData } = usePlugin();
  const formData = new FormData();
  formData.set('region', 'ca-central-1');
  formData.set('arn', 'arn:aws:sso:::instance/ssoins-8004ee88884dd26a');
  formData.set(PluginConfigAwsIc.OidcIntegrationName, 'test-integration');
  setFormData(formData);

  return <>{children}</>;
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

const mockLoadOptions = async () =>
  users.map(u => ({ value: u, label: u.name }));

export const GroupsWithAssignment = () => {
  const [showTable, setShowTable] = useState(false);
  const [selectedOwners, setSelectedOwners] = useState([]);
  const props = {
    groupsWithPermissionAssignment: groupsWithPermissionAssignment,
    showTable: showTable,
    setShowTable: setShowTable,
    loading: false,
    loadOptions: mockLoadOptions,
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
        loadOptions={() => new Promise(() => {})}
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
        loadOptions={() => Promise.reject('Failed to fetch users')}
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
