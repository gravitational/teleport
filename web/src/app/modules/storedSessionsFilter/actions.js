var reactor = require('app/reactor');
var sessionModule = require('./../sessions');
var {filter} = require('./getters');
var {maxSessionLoadSize} = require('app/config');
var moment = require('moment');

const {
  TLPT_STORED_SESSINS_FILTER_SET_RANGE,
  TLPT_STORED_SESSINS_FILTER_SET_STATUS }  = require('./actionTypes');

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
  sessionModule.actions.fetchSessions({sid, before, limit: maxSessionLoadSize})
    .done((json) => {
      let {start} = reactor.evaluate(filter);
      let {sessions } = json;

      let status = {
        hasMore: false,
        isLoading: false
      }

      if (sessions.length === maxSessionLoadSize) {
        let {id, created} = sessions[sessions.length-1];
        status.sid = id;
        status.nextBefore = new Date(created);
        status.hasMore = moment(start).isBefore(status.nextBefore)
      }

      reactor.dispatch(TLPT_STORED_SESSINS_FILTER_SET_STATUS, status);
    });
}

export default actions;
