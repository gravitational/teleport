var {TRYING_TO_SIGN_UP} = require('app/modules/restApi/constants');

const invite = [ ['tlpt_invite'], (invite) => {
  return invite;
 }
];

const attemp = [
  [['tmpl_rest_api', TRYING_TO_SIGN_UP]],
  (attemp) => {
    return attemp;
 }
];

export default {
  invite,
  attemp
}
