var reactor = require('app/reactor');
var { TLPT_DIALOG_SHOW_SELECT_NODE, TLPT_DIALOG_CLOSE_SELECT_NODE } = require('./actionTypes');

var actions = {
  showSelectNodeDialog(){
    reactor.dispatch(TLPT_DIALOG_SHOW_SELECT_NODE);
  },

  closeSelectNodeDialog(){
    reactor.dispatch(TLPT_DIALOG_CLOSE_SELECT_NODE);
  }
}

export default actions;
