var { Store, toImmutable } = require('nuclear-js');

var { TLPT_DIALOG_SELECT_NODE_SHOW, TLPT_DIALOG_SELECT_NODE_CLOSE } = require('./actionTypes');

export default Store({

  getInitialState() {
    return toImmutable({
      isSelectNodeDialogOpen: false
    });
  },

  initialize() {
    this.on(TLPT_DIALOG_SELECT_NODE_SHOW, showSelectNodeDialog);
    this.on(TLPT_DIALOG_SELECT_NODE_CLOSE, closeSelectNodeDialog);
  }
})

function showSelectNodeDialog(state){
  return state.set('isSelectNodeDialogOpen', true);
}

function closeSelectNodeDialog(state){
  return state.set('isSelectNodeDialogOpen', false);
}
