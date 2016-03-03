var $ = require("jQuery");
var session = require('app/session');

const api = {

  put(path, data, withToken){
    return api.ajax({url: path, data: JSON.stringify(data), type: 'PUT'}, withToken);
  },

  post(path, data, withToken){
    return api.ajax({url: path, data: JSON.stringify(data), type: 'POST'}, withToken);
  },

  get(path){
    return api.ajax({url: path});
  },

  ajax(cfg, withToken = true){
    var defaultCfg = {
      type: "GET",
      dataType: "json",
      beforeSend: function(xhr) {
        if(withToken){
          var { token } = session.getUserData();
          xhr.setRequestHeader('Authorization','Bearer ' + token);
        }
       }
    }

    return $.ajax($.extend({}, defaultCfg, cfg));
  }
}

module.exports = api;
