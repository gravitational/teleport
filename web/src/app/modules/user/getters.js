var {TRYING_TO_LOGIN} = require('app/modules/restApi/constants');
var {requestStatus} = require('app/modules/restApi/getters');

const user = [ ['tlpt_user'], (currentUser) => {
    if(!currentUser){
      return null;
    }

    var name = currentUser.get('name') || '';
    var shortDisplayName = name[0] || '';

    return {
      name,
      shortDisplayName,
      logins: currentUser.get('allowed_logins').toJS()
    }
  }
];

export default {
  user,
  loginAttemp: requestStatus(TRYING_TO_LOGIN)
}
