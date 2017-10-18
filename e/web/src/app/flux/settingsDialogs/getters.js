import * as API from 'app/flux/restApi/constants';
import { requestStatus } from 'app/flux/restApi/getters';

export default {    
  dialogsStore: ['tlp_settings_dialogs'],  
  deleteAttempt: requestStatus(API.TRYING_TO_DELETE_RESOURCE)
}
