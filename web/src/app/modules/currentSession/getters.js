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

var {createView} = require('app/modules/sessions/getters');

const currentSession = [ ['tlpt_current_session'], ['tlpt_sessions'],
(current, sessions) => {
    if(!current){
      return null;
    }

    /*
    * active session needs to have its own view as an actual session might not
    * exist at this point. For example, upon creating a new session we need to know
    * login and serverId. It will be simplified once server API gets extended.
    */
    let curSessionView = {
      isNewSession: current.get('isNewSession'),
      serverId: current.get('serverId'),
      login: current.get('login'),
      sid: current.get('sid'),
      cols: undefined,
      rows: undefined
    };

    /*
    * in case if session already exists, get its view data (for example, when joining an existing session)
    */
    if(sessions.has(curSessionView.sid)){
      let existing = createView(sessions.get(curSessionView.sid));

      curSessionView.parties = existing.parties;
      curSessionView.serverId = existing.serverId;
      curSessionView.active = existing.active;
      curSessionView.cols = existing.cols;
      curSessionView.rows = existing.rows;
    }

    return curSessionView;
  }
];

export default {
  currentSession
}
