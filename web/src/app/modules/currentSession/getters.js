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

const currentSession = [ ['tlpt_current_session'], ['tlpt_sessions_active'],
(current, sessions) => {
    if(!current){
      return null;
    }
    
    let curSession = current.toJS();
    
    // get the list of participants     
    if(sessions.has(curSession.sid)){
      let activeSessionRec = sessions.get(curSession.sid);        
      curSession.parties = activeSessionRec.parties.toJS();                      
    }

    return curSession;
  }
];

export default {
  currentSession
}
