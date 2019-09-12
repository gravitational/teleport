import React from 'react';
import PropTypes from 'prop-types';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'design/Layout';
import Indicator from 'design/Indicator';
import { Danger } from 'design/Alert';
import { Flex, Box, ButtonPrimary, Text } from 'design';
import ResourceEditor from 'e-shared/components/ResourceEditor';
import sampleRole from './template';
import RoleList from './RoleList';
import DeleteRoleDialog from './DeleteRole';

export default function Roles(props) {
  const { attempt, roles, canCreate } = props;
  const [selected, selectedActions] = useSelection(roles);
  const { message, isProcessing, isFailed } = attempt;
  const title = selected.isCreating ? 'Create a new rolw' : 'Edit role';

  function onDelete() {
    return props.onDelete(selected.role);
  }

  function onSave(content) {
    return props.onSave(content, selected.isCreating);
  }

  if (isProcessing) {
    return (
      <Flex justifyContent="center">
        <Indicator />
      </Flex>
    );
  }

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle>Roles</FeatureHeaderTitle>
        {canCreate && (
          <ButtonPrimary
            ml="auto"
            width="240px"
            onClick={selectedActions.onCreate}
          >
            CREATE NEW ROLE
          </ButtonPrimary>
        )}
      </FeatureHeader>
      {isFailed && <Danger>{message} </Danger>}
      <Flex>
        <Box width="100%" mr="6" mb="4">
          <RoleList
            items={roles}
            onEdit={selectedActions.onEdit}
            onDelete={selectedActions.onDelete}
          />
        </Box>
        <Box
          ml="auto"
          width="240px"
          color="text.primary"
          style={{ flexShrink: 0 }}
        >
          <Text typography="h6" mb={3} caps>
            Role based access control
          </Text>
          <Text typography="subtitle1" mb={3}>
            Kuberntes and SSH authentication in one place. A Gravity role can be
            assigned based on user identity when used with single sign-on (SSO).
          </Text>
          <Text>
            Learn more in{' '}
            <Text
              as="a"
              color="light"
              target="_blank"
              href="https://gravitational.com/gravity/docs/cluster/#rbac"
            >
              cluster management (RBAC)
            </Text>{' '}
            section of online documentation.
          </Text>
        </Box>
      </Flex>
      {(selected.isCreating || selected.isEditing) && (
        <ResourceEditor
          onSave={onSave}
          title={title}
          onClose={selectedActions.onCancel}
          text={selected.role.content}
          name={selected.role.name}
          isNew={selected.isCreating}
          docsURL="https://gravitational.com/gravity/docs/cluster/#rbac"
          directions={<Directions />}
        />
      )}
      {selected.isDeleting && (
        <DeleteRoleDialog
          name={selected.role.name}
          onClose={selectedActions.onCancel}
          onDelete={onDelete}
        />
      )}
    </FeatureBox>
  );
}

function Directions() {
  return (
    <>
      WARNING Roles are defined using{' '}
      <Text
        as="a"
        color="light"
        target="_blank"
        href="https://en.wikipedia.org/wiki/YAML"
      >
        YAML format
      </Text>
      . YAML is sensitive to white space, please be careful.
    </>
  );
}

const defaultState = {
  isCreating: false,
  isEditing: false,
  isDeleting: false,
  role: null,
};

function useSelection(roles) {
  const [state, setState] = React.useState({
    ...defaultState,
  });

  const onCreate = () => {
    setState({
      ...defaultState,
      isCreating: true,
      role: {
        content: sampleRole,
      },
    });
  };

  const onCancel = () => {
    setState({
      ...defaultState,
      role: null,
    });
  };

  const onEdit = id => {
    const role = roles.find(c => c.id === id);
    setState({
      ...defaultState,
      isEditing: true,
      role,
    });
  };

  const onDelete = id => {
    const role = roles.find(c => c.id === id);
    setState({
      ...defaultState,
      isDeleting: true,
      role,
    });
  };

  return [state, { onCreate, onEdit, onCancel, onDelete }];
}

Roles.propTypes = {
  roles: PropTypes.array.isRequired,
  canCreate: PropTypes.bool.isRequired,
  onSave: PropTypes.func.isRequired,
  onDelete: PropTypes.func.isRequired,
};
