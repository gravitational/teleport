const activeSession = [
['tlpt_active_terminal'], ['tlpt_sessions'],
(activeTerm, sessions) => {
    if(!activeTerm){
      return null;
    }

    let view = {
      isNew: activeTerm.get('isNew'),
      addr: activeTerm.get('addr'),
      login: activeTerm.get('login'),
      sid: activeTerm.get('sid'),
      cols: undefined,
      rows: undefined
    };

    if(sessions.has(view.sid)){
      view.cols = sessions.getIn([view.sid, 'terminal_params', 'H']);
      view.rows = sessions.getIn([view.sid, 'terminal_params', 'W']);
    }

    return view;

  }
];

export default {
  activeSession
}
