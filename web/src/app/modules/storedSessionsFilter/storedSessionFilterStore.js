var { Store, toImmutable } = require('nuclear-js');
var {weekRange} = require('app/common/dateUtils');
var {maxSessionLoadSize} = require('app/config');
var moment = require('moment');

var {
  TLPT_STORED_SESSINS_FILTER_SET_RANGE,
  TLPT_STORED_SESSINS_FILTER_SET_STATUS,
  TLPT_STORED_SESSINS_FILTER_RECEIVE_MORE } = require('./actionTypes');

export default Store({
  getInitialState() {
    let [start, end] = weekRange(new Date());
    let state = {
      start,
      end,
      status: {
        isLoading: false,
        hasMore: false
      }
    }

    return toImmutable(state);
  },

  initialize() {
    this.on(TLPT_STORED_SESSINS_FILTER_RECEIVE_MORE, receiveMore)
    this.on(TLPT_STORED_SESSINS_FILTER_SET_RANGE, setRange);
    this.on(TLPT_STORED_SESSINS_FILTER_SET_STATUS, setStatus);
  }
})

function receiveMore(state, sessions){
  let status = {
    hasMore: false,
    isLoading: false
  }

  if (sessions.length === maxSessionLoadSize) {
    let {id, created} = sessions[sessions.length-1];
    status.sid = id;
    status.nextBefore = new Date(created);
    status.hasMore = moment(state.get('start')).isBefore(status.nextBefore)
  }

  return setStatus(state, status);
}

function setStatus(state, status){
  return state.mergeIn(['status'], status);
}

function setRange(state, {start, end}){
  return state.set('start', start)
       .set('end', end)
       .set('hasMore', false);
}
