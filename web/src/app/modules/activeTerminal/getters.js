const terminal = [ ['tlpt_active_terminal'], (settings) => {
    if(!settings){
      return null;
    }

    var {addr, login } = settings.toJS();
    return {addr, login }
  }
];

export default {
  terminal
}
