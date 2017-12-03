import $ from 'jQuery';
import reactor from 'app/reactor';
import { isObject } from 'lodash';
import withFeature from './components/withFeature';
import api from 'app/services/api';
import { RestRespCodeEnum } from 'app/services/enums';
import { makeStatus } from 'app/flux/status/actions';
import { makeGetter } from 'app/flux/status/getters';

let _featureId = 0;

const ensureActionType = actionType => {
  if (!actionType) {
    ++_featureId;
    return `TRYING_TO_INIT_FEATURE_${_featureId}`;
  }

  return actionType;
}

export default class FeatureBase {
      
  constructor(actionType) {
    actionType = ensureActionType(actionType);
    this.initStatus = makeStatus(ensureActionType(actionType));
    this.initAttemptGetter = makeGetter(actionType);
  }
      
  preload() {
    return $.Deferred().resolve();
  }
  
  onload() { }
      
  startProcessing() {
    this.initStatus.start();    
  }

  stopProcessing() {
    this.initStatus.success();    
  }
    
  isReady() {
    return this._getInitAttempt().isSuccess;
  }

  isProcessing() {
    return this._getInitAttempt().isProcessing;
  }

  isFailed() {
    return this._getInitAttempt().isFailed;
  }

  wasInitialized() {
    const attempt = this._getInitAttempt();
    return attempt.isFailed || attempt.isProcessing || attempt.isSuccess;
  }

  componentDidMount(){ }

  getErrorText() {
    const { message } = this._getInitAttempt();
    return isObject(message) ? message.text : message;          
  }

  getErrorCode(){
    const { message } = this._getInitAttempt();
    return isObject(message) ? message.code : null;
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
    
    this.initStatus.fail(message);    
  }
    
  withMe(component) {
    return withFeature(this)(component);
  }
    
  _getInitAttempt(){
    return reactor.evaluate(this.initAttemptGetter);
  }
}
