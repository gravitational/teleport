const user = [ ['tlpt_user'], (currentUser) => {
    if(!currentUser){
      return null;
    }
    
    return {
      name: currentUser.get('name'),
      logins: currentUser.get('allowed_logins').toJS()
    }
  }
];

export default {
  user
}
