/*eslint no-undef: 0,  no-unused-vars: 0, no-debugger:0*/

var {TRYING_TO_SIGN_UP} = require('app/modules/restApi/constants');

const invite = [ ['tlpt_invite'], (invite) => {
  return invite;
 }
];

const attemp = [ ['tlpt_rest_api', TRYING_TO_SIGN_UP], (attemp) => {
  var defaultObj = {
    isProcessing: false,
    isError: false,
    isSuccess: false,
    message: ''
  }

  return attemp ? attemp.toJS() : defaultObj;
  
 }
];

export default {
  invite,
  attemp
}
