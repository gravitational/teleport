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
  user
}
