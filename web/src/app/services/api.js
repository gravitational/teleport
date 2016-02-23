var $ = require("jQuery");
var session = require('./../session');

const BASE_URL = window.location.origin;

const api = {
  login(username, password){
    var options = {
      url: BASE_URL + "/portal/v1/sessions",
      type: "POST",
      dataType: "json",
      beforeSend: function(xhr) {
        if(!!username && !!password){
          xhr.setRequestHeader ("Authorization", "Basic " + btoa(username + ":" + password));
        }else{
          var { token } = session.getUserData();
          xhr.setRequestHeader('Authorization','Bearer ' + token);
        }
      }
    }

    return $.ajax(options).then(json => {
      return {
        user: {
          name: json.user.name,
          accountId: json.user.account_id,
          siteId: json.user.site_id
        },
        token: json.token
      }
    })
  }
}

module.exports = api;
