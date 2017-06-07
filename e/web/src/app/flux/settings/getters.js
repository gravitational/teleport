import { TRYING_TO_INIT_SETTINGS } from 'app/flux/restApi/constants';
import { requestStatus } from 'app/flux/restApi/getters';

export default {  
  store: ['tlp_settings'],
  initAttemp: requestStatus(TRYING_TO_INIT_SETTINGS)
}
