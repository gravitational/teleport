var reactor = require('app/reactor');
var { TLPT_NOTIFICATIONS_ADD }  = require('./actionTypes');

export default {

  showError(text, title='ERROR'){
    dispatch({isError: true, text: text, title});
  },

  showSuccess(text, title='SUCCESS'){
    dispatch({isSuccess:true, text: text, title});
  },

  showInfo(text, title='INFO'){
    dispatch({isInfo:true, text: text, title});
  },

  showWarning(text, title='WARNING'){
    dispatch({isWarning: true, text: text, title});
  }

}

function dispatch(msg){
  reactor.dispatch(TLPT_NOTIFICATIONS_ADD, msg);
}
