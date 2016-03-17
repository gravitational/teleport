var reactor = require('app/reactor');
var {fetchSessions} = require('./../sessions/actions');
var {filter} = require('./getters');
var {maxSessionLoadSize} = require('app/config');

const {
  TLPT_STORED_SESSINS_FILTER_SET_RANGE,
  TLPT_STORED_SESSINS_FILTER_SET_STATUS,
  TLPT_STORED_SESSINS_FILTER_RECEIVE_MORE }  = require('./actionTypes');

const actions = {

  fetch(){
    let { end } = reactor.evaluate(filter);
    _fetch(end);
  },

  fetchMore(){
    let {status } = reactor.evaluate(filter);
    if(status.hasMore === true && !status.isLoading){
      let {sid, nextBefore} = status;
      _fetch(nextBefore, sid);
    }
  },

  setTimeRange(start, end){
    reactor.batch(()=>{
      reactor.dispatch(TLPT_STORED_SESSINS_FILTER_SET_RANGE, {start, end});
      _fetch(end);
    });
  }
}

function _fetch(before, sid){
  reactor.dispatch(TLPT_STORED_SESSINS_FILTER_SET_STATUS, {isLoading: true, hasMore: false});
  fetchSessions({sid, before, limit: maxSessionLoadSize})
    .done((json) => {
      let {sessions } = json;
      reactor.dispatch(TLPT_STORED_SESSINS_FILTER_RECEIVE_MORE, sessions);
    });
}

export default actions;
