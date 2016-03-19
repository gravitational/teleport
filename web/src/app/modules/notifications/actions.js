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

var reactor = require('app/reactor');
var { TLPT_NOTIFICATIONS_ADD }  = require('./actionTypes');

export default {

  showError(text, title='ERROR'){
    dispatch({isError: true, text: text, title});
  },

  showSuccess(text, title='SUCCESS'){
    dispatch({isSuccess:true, text: text, title});
  },

  showInfo(text, title='INFO'){
    dispatch({isInfo:true, text: text, title});
  },

  showWarning(text, title='WARNING'){
    dispatch({isWarning: true, text: text, title});
  }

}

function dispatch(msg){
  reactor.dispatch(TLPT_NOTIFICATIONS_ADD, msg);
}
