import $ from 'jQuery';
import Logger from 'app/lib/logger';
const logger = Logger.create('featureActivator');

/**
 * Invokes methods on a group of registered features. 
 * 
 */
class FeactureActivator {
  
  constructor() {
    this._features = [];
  }

  register(feature) {
    if (!feature) {
      throw Error('Feature is undefined');
    }

    this._features.push(feature);    
  }

  /**
   * to be called during app initialization. Becomes useful if feature wants to be
   * part of app initialization flow. 
   */
  preload(context) {
    let promises = this._features.map(f => {
      let featurePromise = $.Deferred();
      // feature should handle failed promises thus always resolve.
      f.init(context).always(() => {
        featurePromise.resolve()
      })
      
      return featurePromise;
    });
                          
    return $.when(...promises);      
  }
    
  onload(context) {
    this._features.forEach(f => {      
      this._invokeOnload(f, context);
    });
  }
  
  getFirstAvailable(){
    return this._features.find( f => !f.isFailed() );
  }

  _invokeOnload(f, ...props) {
    try {
      f.onload(...props);
    } catch(err) {
      logger.error('failed to invoke feature onload()', err);
    }          
  }

}

export default FeactureActivator;