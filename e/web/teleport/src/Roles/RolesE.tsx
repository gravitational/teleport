import { RolesContainer as Roles } from 'teleport/Roles';

import { useRoleWithAccessGraph } from './useRoleWithAccessGraph';

export const RolesE = () => {
  const roleDiffProps = useRoleWithAccessGraph();
  return <Roles roleDiffProps={roleDiffProps} />;
};
