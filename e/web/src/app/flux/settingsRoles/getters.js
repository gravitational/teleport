import { requestStatus } from 'app/flux/restApi/getters';
import { TRYING_TO_DELETE_ROLE } from 'app/flux/restApi/constants';

const roles = [['tlp_settings_role', 'allRoles'], allRoles => allRoles.toJS()]
const roleStore = [['tlp_settings_role'], store => store.toJS()];
const roleNames = [['tlp_settings_role', 'allRoles'], allRoles => {
  return allRoles.map(i => i.get('name')).toJS()
}];  

const roleLabels = [['tlp_settings_role', 'allRoles'], allRoles => {
  return allRoles.map(r => ({
    value: r.name,
    label: r.displayName
  })).toJS();  
}]

export default {  
  deleteRoleAttemp: requestStatus(TRYING_TO_DELETE_ROLE),
  roleStore,
  roleNames,
  roleLabels,
  roles
}
