var { Store, toImmutable } = require('nuclear-js');
var {weekRange} = require('app/common/dateUtils');
var {
  TLPT_STORED_SESSINS_FILTER_SET_RANGE,
  TLPT_STORED_SESSINS_FILTER_SET_STATUS } = require('./actionTypes');

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
    this.on(TLPT_STORED_SESSINS_FILTER_SET_RANGE, setRange);
    this.on(TLPT_STORED_SESSINS_FILTER_SET_STATUS, setStatus);
  }
})

function setStatus(state, status){
  return state.mergeIn(['status'], status);
}

function setRange(state, {start, end}){
  return state.set('start', start)
       .set('end', end)
       .set('hasMore', false);
}
