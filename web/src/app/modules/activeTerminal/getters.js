const activeSession = [
['tlpt_active_terminal'], ['tlpt_sessions'],
(activeTerm, sessions) => {
    if(!activeTerm){
      return null;
    }

    let view = {
      isNewSession: activeTerm.get('isNewSession'),
      notFound: activeTerm.get('notFound'),
      addr: activeTerm.get('addr'),
      serverId: activeTerm.get('serverId'),
      login: activeTerm.get('login'),
      sid: activeTerm.get('sid'),
      cols: undefined,
      rows: undefined
    };

    let session = sessions.get(view.sid);

    if(session){
      view.active = session.get('active'),
      view.cols = session.getIn(['terminal_params', 'w']);
      view.rows = session.getIn(['terminal_params', 'h']);
    }

    return view;

  }
];

export default {
  activeSession
}
