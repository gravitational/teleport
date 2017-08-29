import * as API from 'app/flux/restApi/constants';
import { requestStatus } from 'app/flux/restApi/getters';

export default {  
  store: ['tlp_settings'],
  dialogsStore: ['tlp_settings_dialogs'],
  initAttempt: requestStatus(API.TRYING_TO_INIT_SETTINGS),
  deleteAttempt: requestStatus(API.TRYING_TO_DELETE_RESOURCE)
}
