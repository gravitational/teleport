import React from 'react';
import { useFluxStore } from 'gravity/components/nuclear';
import ResourceEditor from 'gravity/components/ResourceEditor';
import userAclGetters from 'gravity/flux/userAcl/getters';
import { FeatureBox, FeatureHeader, FeatureHeaderTitle } from 'gravity/cluster/components/Layout';
import { Flex, Box, ButtonPrimary, Text } from 'design';
import { useState, withState } from 'shared/hooks';
import * as roleFlux from 'e-gravity/cluster/flux/roles';
import * as actions from 'e-gravity/cluster/flux/roles/actions';
import sampleRole from './template';
import RoleList from './RoleList';
import DeleteRoleDialog from './DeleteRole';

export function Roles(props){
  const { store, userAclStore, saveRole } = props;

  // state
  const [ resource, setResource ] = useState(null);
  const [ roleToDelete, setRoleToDelete ] = useState(null);

  const onSave = resource => {
    return saveRole(resource.content, resource.isNew)
  }

  const onCreate = () => {
    setResource({
      isNew: true,
      content: sampleRole
    })
  }

  const onEditor = id => {
    const roleRec = props.store.findItem(id);
    const { content, name } = roleRec;
    setResource({ content, name });
  }

  const onDelete = id => {
    const roleRec = props.store.findItem(id);
    setRoleToDelete(roleRec);
  }

  const roles = store.getItems().toJS();
  const access = userAclStore.getRoleAccess();
  const canCreate = access.create;

  const isNewRole = resource && resource.isNew;
  const title = isNewRole ? 'Create a new role' : 'Edit role';

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle>
          Roles
        </FeatureHeaderTitle>
        <ButtonPrimary ml="auto" width="240px" onClick={onCreate}>
          CREATE NEW ROLE
        </ButtonPrimary>
      </FeatureHeader>
      <Flex>
        <Box width="100%" mr="6" mb="4">
          <RoleList
            canCreate={canCreate}
            roles={roles}
            onDelete={onDelete}
            onCreate={onCreate}
            onEdit={onEditor}
          />
        </Box>
        <Box ml="auto" width="240px" color="text.primary" style={{flexShrink: 0}}>
          <Text typography="h6" mb={3} caps>
            Role based access control
          </Text>
          <Text typography="subtitle1" mb={3}>
            Kuberntes and SSH authentication in one place. A Gravity role can be assigned based on user identity when used with single sign-on (SSO).
          </Text>
          <Text>
            Learn more in <Text as="a" color="light" target="_blank" href="https://gravitational.com/gravity/docs/cluster/#rbac">cluster management (RBAC)</Text> section of online documentation.
          </Text>
        </Box >
      </Flex>
      <ResourceEditor
        onSave={onSave}
        title={title}
        onClose={() => setResource(null)}
        resource={resource}
        directions={<Directions/>}
        docsURL="https://gravitational.com/gravity/docs/cluster/#rbac"
      />
      <DeleteRoleDialog
        role={roleToDelete}
        onClose={() => setRoleToDelete(null)}
      />
    </FeatureBox>
  )
}

function Directions(){
  return (
    <>
      WARNING Roles are defined using <Text as="a" color="light" target="_blank" href="https://en.wikipedia.org/wiki/YAML">YAML format</Text>.
      YAML is sensitive to white space, please be careful.
    </>
  )
}

export default withState(() => {
  const userAclStore = useFluxStore(userAclGetters.userAcl);
  const store = useFluxStore(roleFlux.getters.store);
  return {
    userAclStore,
    store,
    saveRole: actions.saveRole,
  }
})(Roles);

