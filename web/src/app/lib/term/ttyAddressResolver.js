/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import cfg from 'app/config';

export default class AddressResolver {
  _params = {
    login: null,
    target: () => {
      throw Error('target method is not provided');
    },
    sid: null,
    clusterName: null,
    ttyUrl: null,
    ttyEventUrl: null,
    ttyResizeUrl: null
  }

  constructor(params){
    this._params = {
      ...params
    }    
  }

  getConnStr(w, h){
    const { getTarget, ttyUrl, login, sid} = this._params;
    const params = JSON.stringify({
      ...getTarget(),
      login,
      sid,      
      term: { h, w }
    });
    
    const encoded = window.encodeURI(params);    
    return this.format(ttyUrl).replace(':params', encoded);    
  }

  getEventProviderConnStr(){
    return this.format(this._params.ttyEventUrl);      
  }

  getResizeReqUrl(){        
    return this.format(this._params.ttyResizeUrl);      
  }

  format(url){
    return url
      .replace(':fqdm', cfg.getWsHostName()) 
      .replace(':token', this._params.token)
      .replace(':cluster', this._params.cluster) 
      .replace(':sid', this._params.sid);    
  }
}