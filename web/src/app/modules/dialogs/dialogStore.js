var { Store, toImmutable } = require('nuclear-js');

var { TLPT_DIALOG_SHOW_SELECT_NODE, TLPT_DIALOG_CLOSE_SELECT_NODE } = require('./actionTypes');

export default Store({

  getInitialState() {
    return toImmutable({
      isSelectNodeDialogOpen: false
    });
  },

  initialize() {
    this.on(TLPT_DIALOG_SHOW_SELECT_NODE, showSelectNodeDialog);
    this.on(TLPT_DIALOG_CLOSE_SELECT_NODE, closeSelectNodeDialog);
  }
})

function showSelectNodeDialog(state){
  return state.set('isSelectNodeDialogOpen', true);
}

function closeSelectNodeDialog(state){
  return state.set('isSelectNodeDialogOpen', false);
}
