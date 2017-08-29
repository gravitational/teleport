import { requestStatus } from 'app/flux/restApi/getters';
import { TRYING_TO_SAVE_AUTH_PROVIDER } from 'app/flux/restApi/constants';

export default {  
  saveAttempt: requestStatus(TRYING_TO_SAVE_AUTH_PROVIDER),
  store: ['tlp_settings_auth'],
  errors: ['tlp_settings_auth_errors'] 
}
