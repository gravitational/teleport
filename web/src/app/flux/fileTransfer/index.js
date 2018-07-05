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
import store from './store';

const STORE_NAME = 'tlpt_files';

export function getFileTransfer() {
  return reactor.evaluate([STORE_NAME])
}

export const register = reactor => {
  reactor.registerStores({
    [STORE_NAME]: store
  })
}

export const getters = {
  store: [STORE_NAME]
}