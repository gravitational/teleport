import React from 'react';
import { useFluxStore } from 'gravity/components/nuclear';
import Users from 'gravity/cluster/components/Users';
import { getters } from 'e-gravity/cluster/flux/roles';

export default function EnterpriseUsers(props) {
  const roles = useFluxStore(getters.store);
  return <Users roles={roles} {...props} />
}