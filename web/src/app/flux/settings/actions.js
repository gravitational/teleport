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

import reactor from 'app/reactor';
import getters from './getters';
import * as AT from './actionTypes';
import { initSettingsStatus } from './../status/actions';

export function addNavItem(navItem){
  reactor.dispatch(AT.ADD_NAV_ITEM, navItem)
}

export function initSettings(featureActivator) {                    
  // init only once
  let store = reactor.evaluate(getters.store)
  if (store.isReady()){
    return;
  }
  
  featureActivator.onload();         
  reactor.dispatch(AT.INIT, {});            
  initSettingsStatus.success();  
}         
