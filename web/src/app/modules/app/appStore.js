var { Store, toImmutable } = require('nuclear-js');

var { TLPT_APP_INIT, TLPT_APP_FAILED, TLPT_APP_READY } = require('./actionTypes');

var initState = toImmutable({
  isReady: false,
  isInitializing: false,
  isFailed: false
});

export default Store({

  getInitialState() {
    return initState.set('isInitializing', true);
  },

  initialize() {
    this.on(TLPT_APP_INIT, ()=> initState.set('isInitializing', true));
    this.on(TLPT_APP_READY,()=> initState.set('isReady', true));
    this.on(TLPT_APP_FAILED,()=> initState.set('isFailed', true));
  }
})
