import $ from 'jQuery';
import reactor from 'app/reactor';
import api from 'app/services/api'
import { fetchConnectors } from './../settingsAuth/actions';
import { fetchRoles } from './../settingsRoles/actions';
import { fetchTrustedClusters } from './../settingsClusters/actions';
import { SETTINGS_INIT } from './actionTypes';
import apiActions from './../restApi/actions';
import { TRYING_TO_INIT_SETTINGS } from './../restApi/constants';
import { RestRespCodeEnum } from 'app/services/enums';

const actions = {
  initSettings() {        
    apiActions.start(TRYING_TO_INIT_SETTINGS)    
    $.when(fetchRoles(), fetchConnectors(), fetchTrustedClusters())
      .done(() => {                
        apiActions.success(TRYING_TO_INIT_SETTINGS);
        reactor.dispatch(SETTINGS_INIT, {});        
      })  
      .fail(err => {
        let msg = api.getErrorText(err);                             
        if (err.status === RestRespCodeEnum.FORBIDDEN) {          
          msg = {
            code: RestRespCodeEnum.FORBIDDEN,
            text: msg
          }
        } else {
          msg = api.getErrorText(err);            
        }         
        
        apiActions.fail(TRYING_TO_INIT_SETTINGS, msg);                  
      });              
  }         
}

export default actions;
