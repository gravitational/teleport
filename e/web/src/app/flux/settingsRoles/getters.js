import { requestStatus } from 'app/flux/restApi/getters';
import * as AT from 'app/flux/restApi/constants';

export default {  
  saveAttempt: requestStatus(AT.TRYING_TO_SAVE_ROLE),
  store: ['tlp_settings_role']  
}
