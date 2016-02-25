var { Store, toImmutable } = require('nuclear-js');
var {
  TLPT_REST_API_START,
  TLPT_REST_API_SUCCESS,
  TLPT_REST_API_FAIL } = require('./actionTypes');

export default Store({
  getInitialState() {
    return toImmutable({});
  },

  initialize() {
    this.on(TLPT_REST_API_START, start);
    this.on(TLPT_REST_API_FAIL, fail);
    this.on(TLPT_REST_API_SUCCESS, success);
  }
})

function start(state, request){
  return state.set(request.type, toImmutable({isProcessing: true}));
}

function fail(state, request){
  return state.set(request.type, toImmutable({isFailed: true, message: request.message}));
}

function success(state, request){
  return state.set(request.type, toImmutable({isSuccess: true}));
}
