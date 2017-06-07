import { requestStatus } from 'app/flux/restApi/getters';
import { TRYING_TO_SAVE_CLUSTER } from 'app/flux/restApi/constants';

export default {  
  store: [['tlp_settings_cluster'], store => store.toJS() ],    
  saveClusterAttemp: requestStatus(TRYING_TO_SAVE_CLUSTER)
}