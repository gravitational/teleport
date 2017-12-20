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
import { storage } from 'app/flux/misc/store';

const withStorage = component => {
  
  return class WithTmpStorageWrapper extends React.Component{
      
    static displayName = `withTmpStorageWrapper`

    constructor(props, context) {
      super(props, context)                  
    }
                                 
    render() {            
      const props = this.props;
      return React.createElement(component, {
        ...props,
        storage
      });      
    }
  }
}

export default withStorage;