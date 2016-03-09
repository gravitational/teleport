var {createView} = require('app/modules/sessions/getters');

const activeSession = [
['tlpt_active_terminal'], ['tlpt_sessions'],
(activeTerm, sessions) => {
    if(!activeTerm){
      return null;
    }

    /*
    * active session needs to have its own view as an actual session might not
    * exist at this point. For example, upon creating a new session we need to know
    * login and serverId. It will be simplified once server API gets extended.
    */
    let asView = {
      isNewSession: activeTerm.get('isNewSession'),
      notFound: activeTerm.get('notFound'),
      addr: activeTerm.get('addr'),
      serverId: activeTerm.get('serverId'),
      serverIp: undefined,
      login: activeTerm.get('login'),
      sid: activeTerm.get('sid'),
      cols: undefined,
      rows: undefined
    };

    // in case if session already exists, get the data from there
    // (for example, when joining an existing session)
    if(sessions.has(asView.sid)){
      let sView = createView(sessions.get(asView.sid));

      asView.parties = sView.parties;
      asView.serverIp = sView.serverIp;
      asView.serverId = sView.serverId;
      asView.active = sView.active;
      asView.cols = sView.cols;
      asView.rows = sView.rows;
    }

    return asView;

  }
];

export default {
  activeSession
}
