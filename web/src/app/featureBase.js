import $ from 'jQuery';
import reactor from 'app/reactor';
import { isObject } from 'lodash';
import withFeature from './components/withFeature';
import api from 'app/services/api';
import { RestRespCodeEnum } from 'app/services/enums';
import restApiActions from 'app/flux/restApi/actions';
import { requestStatus } from 'app/flux/restApi/getters';

let _featureId = 0;

const ensureActionType = actionType => {
  if (!actionType) {
    ++_featureId;
    return `TRYING_TO_INIT_FEATURE_${_featureId}`;
  }

  return actionType;
}

export default class FeatureBase {

  initAttemptActionType = '';
    
  constructor(actionType) {
    this.initAttemptActionType = ensureActionType(actionType);
  } 
      
  preload(){
    return $.Deferred().resolve();    
  }
  
  onload() {}
      
  startProcessing() {
    restApiActions.start(this.initAttemptActionType);        
  }

  stopProcessing() {    
    restApiActions.success(this.initAttemptActionType);
  }
    
  isReady(){
    return this._getInitAttemptFromStore().isSuccess;
  }

  isProcessing() {
    return this._getInitAttemptFromStore().isProcessing;
  }

  isFailed() {
    return this._getInitAttemptFromStore().isFailed;
  }

  wasInitialized(){
    const attempt = this._getInitAttemptFromStore();        
    return attempt.isFailed || attempt.isProcessing || attempt.isSuccess;
  }

  getIndexRoute(){
    throw Error('not implemented');
  }

  getErrorText() {
    const { message } = this._getInitAttemptFromStore();
    return isObject(message) ? message.text : message;          
  }

  getErrorCode(){
    const { message } = this._getInitAttemptFromStore();
    return isObject(message) ? message.code : null;
  }

  getIndexComponent(){
    return null;
  }

  handleAccesDenied() {
    this.handleError(new Error('Access Denied'));
  }

  handleError(err) {        
    let message = api.getErrorText(err);                
    if (err.status === RestRespCodeEnum.FORBIDDEN) {          
      message = {
        code: RestRespCodeEnum.FORBIDDEN,
        text: message
      }
    }      
    
    restApiActions.fail(this.initAttemptActionType, message);        
  }
    
  withMe(component) {
    return withFeature(this)(component);
  }

  initAttemptGetter(){    
    return requestStatus(this.initAttemptActionType);      
  }
  
  _getInitAttemptFromStore(){
    return reactor.evaluate(this.initAttemptGetter());
  }
}
