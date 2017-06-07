import { requestStatus } from 'app/flux/restApi/getters';
import { TRYING_TO_DELETE_AUTH_PROVIDER } from 'app/flux/restApi/constants';

const authStore = [['tlp_settings_auth'], providers => providers.toJS()];  

export default {
  deleteConnectorAttemp: requestStatus(TRYING_TO_DELETE_AUTH_PROVIDER),
  store: authStore  
}
