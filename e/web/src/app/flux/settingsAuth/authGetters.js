import { requestStatus } from 'app/flux/restApi/getters';
import { TRYING_TO_DELETE_AUTH_PROVIDER, TRYING_TO_SAVE_AUTH_PROVIDER } from 'app/flux/restApi/constants';

export default {
  deleteAttempt: requestStatus(TRYING_TO_DELETE_AUTH_PROVIDER),
  saveAttempt: requestStatus(TRYING_TO_SAVE_AUTH_PROVIDER),
  store: ['tlp_settings_auth']  
}
