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

var { Store, toImmutable } = require('nuclear-js');

var { TLPT_DIALOG_SELECT_NODE_SHOW, TLPT_DIALOG_SELECT_NODE_CLOSE } = require('./actionTypes');

export default Store({

  getInitialState() {
    return toImmutable({
      isSelectNodeDialogOpen: false
    });
  },

  initialize() {
    this.on(TLPT_DIALOG_SELECT_NODE_SHOW, showSelectNodeDialog);
    this.on(TLPT_DIALOG_SELECT_NODE_CLOSE, closeSelectNodeDialog);
  }
})

function showSelectNodeDialog(state){
  return state.set('isSelectNodeDialogOpen', true);
}

function closeSelectNodeDialog(state){
  return state.set('isSelectNodeDialogOpen', false);
}
