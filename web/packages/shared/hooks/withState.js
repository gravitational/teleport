/*
Copyright 2019 Gravitational, Inc.

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

import { createElement } from 'react';
import hoistStatics from 'hoist-non-react-statics';

export default function withState(mapState) {
  const hooks = hooks ? hooks : () => ({});
  return function wrapWithHook(WrappedComponent) {
    function WithState(props, context){
      const propsFromMapState = mapState(props, context);
      return createElement(WrappedComponent, {
        ...props,
        ...propsFromMapState,
      })
    }

    WithState.displayName = `WithState(${getDisplayName(WrappedComponent)})`
    WithState.WrappedComponent = WrappedComponent
    return hoistStatics(WithState, WrappedComponent)
  }
}

function getDisplayName(WrappedComponent) {
  return WrappedComponent.displayName || WrappedComponent.name || 'Component'
}