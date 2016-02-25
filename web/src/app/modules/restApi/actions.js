var reactor = require('app/reactor');

var {
  TLPT_REST_API_START,
  TLPT_REST_API_SUCCESS,
  TLPT_REST_API_FAIL } = require('./actionTypes');

export default {

  start(reqType){
    reactor.dispatch(TLPT_REST_API_START, {type: reqType});
  },

  fail(reqType, message){
    reactor.dispatch(TLPT_REST_API_FAIL,  {type: reqType, message});
  },

  success(reqType){
    reactor.dispatch(TLPT_REST_API_SUCCESS, {type: reqType});
  }

}
