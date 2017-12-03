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

import React from 'react';
import reactor from '../reactor';
import Indicator from './indicator';
import { RestRespCodeEnum } from '../services/enums';
import * as Messages from './msgPage.jsx';
import Logger from '../lib/logger';

const logger = Logger.create('components/withFeature');

const withFeature = feature => component => {
  
  return class WithFeatureWrapper extends React.Component{
      
    static displayName = `withFeatureWrapper`

    constructor(props, context) {
      super(props, context)            
      this._unsubscribeFn = null;
    }
                    
    componentDidMount() {
      try{
        this._unsubscribeFn = reactor.observe(feature.initAttemptGetter, ()=>{        
          this.setState({})
        })

        reactor.batch(() => {
          feature.componentDidMount();
        })      
                
      }catch(err){
        logger.error('failed to initialize a feature', err);        
      }      
    }
    
    componentWillUnmount() {
      this._unsubscribeFn();
    }
             
    render() {      
      if (feature.isProcessing()) {
        return <Indicator delay="long" type="bounce" />;  
      }

      if (feature.isFailed()) {
        const errorText = feature.getErrorText();
        if (feature.getErrorCode() === RestRespCodeEnum.FORBIDDEN) {
          return <Messages.AccessDenied message={errorText}/>  
        }
        return <Messages.Failed message={errorText}/>
      }
      
      if (!feature.wasInitialized()) {
        return null;
      }

      const props = this.props;
      return React.createElement(component, {
        ...props,
        feature
      });      
    }
  }
}

export default withFeature;