import reactor from 'app/reactor';
import cfg from 'app/config';
import api from 'app/services/api';
import { USERACL_RECEIVE } from './actionTypes';

export default {
  fetchAcl(){    
    return api.get(cfg.api.userAclPath)
      .then(json => {
        reactor.dispatch(USERACL_RECEIVE, json)
      })       
  }  
}
