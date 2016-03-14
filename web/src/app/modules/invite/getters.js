var {TRYING_TO_SIGN_UP, FETCHING_INVITE} = require('app/modules/restApi/constants');
var {requestStatus} = require('app/modules/restApi/getters');

const invite = [ ['tlpt_invite'], (invite) => invite ];

export default {
  invite,
  attemp: requestStatus(TRYING_TO_SIGN_UP),
  fetchingInvite: requestStatus(FETCHING_INVITE)
}
