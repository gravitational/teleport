var reactor = require('app/reactor');
var { TLPT_DIALOG_SELECT_NODE_SHOW, TLPT_DIALOG_SELECT_NODE_CLOSE } = require('./actionTypes');

var actions = {
  showSelectNodeDialog(){
    reactor.dispatch(TLPT_DIALOG_SELECT_NODE_SHOW);
  },

  closeSelectNodeDialog(){
    reactor.dispatch(TLPT_DIALOG_SELECT_NODE_CLOSE);
  }
}

export default actions;
