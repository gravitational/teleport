const activeSession = [ ['tlpt_active_terminal'], (activeSession) => {
    if(!activeSession){
      return null;
    }

    return activeSession.toJS();
  }
];

export default {
  activeSession
}
