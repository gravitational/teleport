webpackJsonp([1],[
/* 0 */
/***/ function(module, exports, __webpack_require__) {

	module.exports = __webpack_require__(198);


/***/ },
/* 1 */,
/* 2 */,
/* 3 */,
/* 4 */,
/* 5 */,
/* 6 */,
/* 7 */,
/* 8 */,
/* 9 */,
/* 10 */,
/* 11 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _nuclearJs = __webpack_require__(18);
	
	var reactor = new _nuclearJs.Reactor({
	  debug: true
	});
	
	window.reactor = reactor;
	
	exports['default'] = reactor;
	module.exports = exports['default'];

/***/ },
/* 12 */,
/* 13 */,
/* 14 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(148);
	
	var formatPattern = _require.formatPattern;
	
	var cfg = {
	
	  baseUrl: window.location.origin,
	
	  api: {
	    renewTokenPath: '/v1/webapi/sessions/renew',
	    nodesPath: '/v1/webapi/sites/-current-/nodes',
	    sessionPath: '/v1/webapi/sessions',
	    invitePath: '/v1/webapi/users/invites/:inviteToken',
	    createUserPath: '/v1/webapi/users',
	    getInviteUrl: function getInviteUrl(inviteToken) {
	      return formatPattern(cfg.api.invitePath, { inviteToken: inviteToken });
	    },
	
	    getEventStreamerConnStr: function getEventStreamerConnStr(token, sid) {
	      var hostname = getWsHostName();
	      return hostname + '/v1/webapi/sites/-current-/sessions/' + sid + '/events/stream?access_token=' + token;
	    },
	
	    getSessionConnStr: function getSessionConnStr(token, params) {
	      var json = JSON.stringify(params);
	      var jsonEncoded = window.encodeURI(json);
	      var hostname = getWsHostName();
	      return hostname + '/v1/webapi/sites/-current-/connect?access_token=' + token + '&params=' + jsonEncoded;
	    }
	  },
	
	  routes: {
	    app: '/web',
	    logout: '/web/logout',
	    login: '/web/login',
	    nodes: '/web/nodes',
	    newUser: '/web/newuser/:inviteToken',
	    sessions: '/web/sessions'
	  }
	
	};
	
	exports['default'] = cfg;
	
	function getWsHostName() {
	  var prefix = location.protocol == "https:" ? "wss://" : "ws://";
	  var hostport = location.hostname + (location.port ? ':' + location.port : '');
	  return '' + prefix + hostport;
	}
	module.exports = exports['default'];

/***/ },
/* 15 */,
/* 16 */,
/* 17 */,
/* 18 */,
/* 19 */,
/* 20 */,
/* 21 */,
/* 22 */,
/* 23 */,
/* 24 */,
/* 25 */
/***/ function(module, exports) {

	/**
	 * Copyright 2013-2014 Facebook, Inc.
	 *
	 * Licensed under the Apache License, Version 2.0 (the "License");
	 * you may not use this file except in compliance with the License.
	 * You may obtain a copy of the License at
	 *
	 * http://www.apache.org/licenses/LICENSE-2.0
	 *
	 * Unless required by applicable law or agreed to in writing, software
	 * distributed under the License is distributed on an "AS IS" BASIS,
	 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	 * See the License for the specific language governing permissions and
	 * limitations under the License.
	 *
	 */
	
	"use strict";
	
	/**
	 * Constructs an enumeration with keys equal to their value.
	 *
	 * For example:
	 *
	 *   var COLORS = keyMirror({blue: null, red: null});
	 *   var myColor = COLORS.blue;
	 *   var isColorValid = !!COLORS[myColor];
	 *
	 * The last line could not be performed if the values of the generated enum were
	 * not equal to their keys.
	 *
	 *   Input:  {key1: val1, key2: val2}
	 *   Output: {key1: key1, key2: key2}
	 *
	 * @param {object} obj
	 * @return {object}
	 */
	var keyMirror = function(obj) {
	  var ret = {};
	  var key;
	  if (!(obj instanceof Object && !Array.isArray(obj))) {
	    throw new Error('keyMirror(...): Argument must be an object.');
	  }
	  for (key in obj) {
	    if (!obj.hasOwnProperty(key)) {
	      continue;
	    }
	    ret[key] = key;
	  }
	  return ret;
	};
	
	module.exports = keyMirror;


/***/ },
/* 26 */,
/* 27 */,
/* 28 */,
/* 29 */,
/* 30 */,
/* 31 */,
/* 32 */,
/* 33 */,
/* 34 */,
/* 35 */,
/* 36 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var _require = __webpack_require__(43);
	
	var browserHistory = _require.browserHistory;
	
	var AUTH_KEY_DATA = 'authData';
	
	var _history = null;
	
	var session = {
	
	  init: function init() {
	    var history = arguments.length <= 0 || arguments[0] === undefined ? browserHistory : arguments[0];
	
	    _history = history;
	  },
	
	  getHistory: function getHistory() {
	    return _history;
	  },
	
	  setUserData: function setUserData(userData) {
	    localStorage.setItem(AUTH_KEY_DATA, JSON.stringify(userData));
	  },
	
	  getUserData: function getUserData() {
	    var item = localStorage.getItem(AUTH_KEY_DATA);
	    if (item) {
	      return JSON.parse(item);
	    }
	
	    return {};
	  },
	
	  clear: function clear() {
	    localStorage.clear();
	  }
	
	};
	
	module.exports = session;

/***/ },
/* 37 */,
/* 38 */,
/* 39 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var $ = __webpack_require__(52);
	var session = __webpack_require__(36);
	
	var api = {
	
	  post: function post(path, data, withToken) {
	    return api.ajax({ url: path, data: JSON.stringify(data), type: 'POST' }, withToken);
	  },
	
	  get: function get(path) {
	    return api.ajax({ url: path });
	  },
	
	  ajax: function ajax(cfg) {
	    var withToken = arguments.length <= 1 || arguments[1] === undefined ? true : arguments[1];
	
	    var defaultCfg = {
	      type: "GET",
	      dataType: "json",
	      beforeSend: function beforeSend(xhr) {
	        if (withToken) {
	          var _session$getUserData = session.getUserData();
	
	          var token = _session$getUserData.token;
	
	          xhr.setRequestHeader('Authorization', 'Bearer ' + token);
	        }
	      }
	    };
	
	    return $.ajax($.extend({}, defaultCfg, cfg));
	  }
	};
	
	module.exports = api;

/***/ },
/* 40 */,
/* 41 */,
/* 42 */,
/* 43 */,
/* 44 */,
/* 45 */,
/* 46 */,
/* 47 */,
/* 48 */,
/* 49 */,
/* 50 */,
/* 51 */,
/* 52 */
/***/ function(module, exports) {

	module.exports = jQuery;

/***/ },
/* 53 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(25);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_NODES_RECEIVE: null
	});
	module.exports = exports['default'];

/***/ },
/* 54 */,
/* 55 */,
/* 56 */,
/* 57 */,
/* 58 */,
/* 59 */,
/* 60 */,
/* 61 */,
/* 62 */,
/* 63 */,
/* 64 */,
/* 65 */,
/* 66 */,
/* 67 */,
/* 68 */,
/* 69 */,
/* 70 */,
/* 71 */,
/* 72 */,
/* 73 */,
/* 74 */,
/* 75 */,
/* 76 */,
/* 77 */,
/* 78 */,
/* 79 */,
/* 80 */,
/* 81 */,
/* 82 */,
/* 83 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var api = __webpack_require__(39);
	var session = __webpack_require__(36);
	var cfg = __webpack_require__(14);
	var $ = __webpack_require__(52);
	
	var refreshRate = 60000 * 5; // 1 min
	
	var refreshTokenTimerId = null;
	
	var auth = {
	
	  signUp: function signUp(name, password, token, inviteToken) {
	    var data = { user: name, pass: password, second_factor_token: token, invite_token: inviteToken };
	    return api.post(cfg.api.createUserPath, data).then(function (user) {
	      session.setUserData(user);
	      auth._startTokenRefresher();
	      return user;
	    });
	  },
	
	  login: function login(name, password, token) {
	    auth._stopTokenRefresher();
	    return auth._login(name, password, token).done(auth._startTokenRefresher);
	  },
	
	  ensureUser: function ensureUser() {
	    var userData = session.getUserData();
	    if (userData.token) {
	      // refresh timer will not be set in case of browser refresh event
	      if (auth._getRefreshTokenTimerId() === null) {
	        return auth._refreshToken().done(auth._startTokenRefresher);
	      }
	
	      return $.Deferred().resolve(userData);
	    }
	
	    return $.Deferred().reject();
	  },
	
	  logout: function logout() {
	    auth._stopTokenRefresher();
	    return session.clear();
	  },
	
	  _startTokenRefresher: function _startTokenRefresher() {
	    refreshTokenTimerId = setInterval(auth._refreshToken, refreshRate);
	  },
	
	  _stopTokenRefresher: function _stopTokenRefresher() {
	    clearInterval(refreshTokenTimerId);
	    refreshTokenTimerId = null;
	  },
	
	  _getRefreshTokenTimerId: function _getRefreshTokenTimerId() {
	    return refreshTokenTimerId;
	  },
	
	  _refreshToken: function _refreshToken() {
	    return api.post(cfg.api.renewTokenPath).then(function (data) {
	      session.setUserData(data);
	      return data;
	    }).fail(function () {
	      auth.logout();
	      window.location.reload();
	    });
	  },
	
	  _login: function _login(name, password, token) {
	    var data = {
	      user: name,
	      pass: password,
	      second_factor_token: token
	    };
	
	    return api.post(cfg.api.sessionPath, data, false).then(function (data) {
	      session.setUserData(data);
	      return data;
	    });
	  }
	};
	
	module.exports = auth;

/***/ },
/* 84 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(25);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_TERM_OPEN: null,
	  TLPT_TERM_CLOSE: null,
	  TLPT_TERM_CONNECTED: null,
	  TLPT_TERM_RECEIVE_PARTIES: null
	});
	module.exports = exports['default'];

/***/ },
/* 85 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(11);
	
	var _require = __webpack_require__(163);
	
	var uuid = _require.uuid;
	
	var _require2 = __webpack_require__(84);
	
	var TLPT_TERM_OPEN = _require2.TLPT_TERM_OPEN;
	var TLPT_TERM_CLOSE = _require2.TLPT_TERM_CLOSE;
	var TLPT_TERM_CONNECTED = _require2.TLPT_TERM_CONNECTED;
	var TLPT_TERM_RECEIVE_PARTIES = _require2.TLPT_TERM_RECEIVE_PARTIES;
	exports['default'] = {
	
	  close: function close() {
	    reactor.dispatch(TLPT_TERM_CLOSE);
	  },
	
	  connected: function connected() {
	    reactor.dispatch(TLPT_TERM_CONNECTED);
	  },
	
	  receiveParties: function receiveParties(json) {
	    var parties = json.map(function (item) {
	      return {
	        user: item.user,
	        lastActive: new Date(item.last_active)
	      };
	    });
	
	    reactor.dispatch(TLPT_TERM_RECEIVE_PARTIES, parties);
	  },
	
	  open: function open(addr, login) {
	    var sid = arguments.length <= 2 || arguments[2] === undefined ? uuid() : arguments[2];
	
	    /*
	    *   {
	    *   "addr": "127.0.0.1:5000",
	    *   "login": "admin",
	    *   "term": {"h": 120, "w": 100},
	    *   "sid": "123"
	    *  }
	    */
	    reactor.dispatch(TLPT_TERM_OPEN, { addr: addr, login: login, sid: sid });
	  }
	};
	module.exports = exports['default'];

/***/ },
/* 86 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };
	
	var _require = __webpack_require__(18);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(84);
	
	var TLPT_TERM_OPEN = _require2.TLPT_TERM_OPEN;
	var TLPT_TERM_CLOSE = _require2.TLPT_TERM_CLOSE;
	var TLPT_TERM_CONNECTED = _require2.TLPT_TERM_CONNECTED;
	var TLPT_TERM_RECEIVE_PARTIES = _require2.TLPT_TERM_RECEIVE_PARTIES;
	exports['default'] = Store({
	  getInitialState: function getInitialState() {
	    return toImmutable(null);
	  },
	
	  initialize: function initialize() {
	    this.on(TLPT_TERM_CONNECTED, connected);
	    this.on(TLPT_TERM_OPEN, setActiveTerminal);
	    this.on(TLPT_TERM_CLOSE, close);
	    this.on(TLPT_TERM_RECEIVE_PARTIES, receiveParties);
	  }
	
	});
	
	function close() {
	  return toImmutable(null);
	}
	
	function receiveParties(state, parties) {
	  return state.set('parties', toImmutable(parties));
	}
	
	function setActiveTerminal(state, settings) {
	  return toImmutable(_extends({
	    parties: [],
	    isConnecting: true
	  }, settings));
	}
	
	function connected(state) {
	  return state.set('isConnected', true).set('isConnecting', false);
	}
	module.exports = exports['default'];

/***/ },
/* 87 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(25);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_RECEIVE_USER_INVITE: null
	});
	module.exports = exports['default'];

/***/ },
/* 88 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(18);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(87);
	
	var TLPT_RECEIVE_USER_INVITE = _require2.TLPT_RECEIVE_USER_INVITE;
	exports['default'] = Store({
	  getInitialState: function getInitialState() {
	    return toImmutable(null);
	  },
	
	  initialize: function initialize() {
	    this.on(TLPT_RECEIVE_USER_INVITE, receiveInvite);
	  }
	});
	
	function receiveInvite(state, invite) {
	  return toImmutable(invite);
	}
	module.exports = exports['default'];

/***/ },
/* 89 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(18);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(53);
	
	var TLPT_NODES_RECEIVE = _require2.TLPT_NODES_RECEIVE;
	exports['default'] = Store({
	  getInitialState: function getInitialState() {
	    return toImmutable([]);
	  },
	
	  initialize: function initialize() {
	    this.on(TLPT_NODES_RECEIVE, receiveNodes);
	  }
	});
	
	function receiveNodes(state, nodeArray) {
	  return toImmutable(nodeArray);
	}
	module.exports = exports['default'];

/***/ },
/* 90 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(25);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_REST_API_START: null,
	  TLPT_REST_API_SUCCESS: null,
	  TLPT_REST_API_FAIL: null
	});
	module.exports = exports['default'];

/***/ },
/* 91 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(25);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TRYING_TO_SIGN_UP: null
	});
	module.exports = exports['default'];

/***/ },
/* 92 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(25);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_SESSINS_RECEIVE: null,
	  TLPT_SESSINS_UPDATE: null
	});
	module.exports = exports['default'];

/***/ },
/* 93 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(25);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_RECEIVE_USER: null
	});
	module.exports = exports['default'];

/***/ },
/* 94 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(11);
	
	var _require = __webpack_require__(93);
	
	var TLPT_RECEIVE_USER = _require.TLPT_RECEIVE_USER;
	
	var _require2 = __webpack_require__(91);
	
	var TRYING_TO_SIGN_UP = _require2.TRYING_TO_SIGN_UP;
	
	var restApiActions = __webpack_require__(159);
	var auth = __webpack_require__(83);
	var session = __webpack_require__(36);
	var cfg = __webpack_require__(14);
	
	exports['default'] = {
	
	  ensureUser: function ensureUser(nextState, replace, cb) {
	    auth.ensureUser().done(function (userData) {
	      reactor.dispatch(TLPT_RECEIVE_USER, userData.user);
	      cb();
	    }).fail(function () {
	      replace({ redirectTo: nextState.location.pathname }, cfg.routes.login);
	      cb();
	    });
	  },
	
	  signUp: function signUp(_ref) {
	    var name = _ref.name;
	    var psw = _ref.psw;
	    var token = _ref.token;
	    var inviteToken = _ref.inviteToken;
	
	    restApiActions.start(TRYING_TO_SIGN_UP);
	    auth.signUp(name, psw, token, inviteToken).done(function (sessionData) {
	      reactor.dispatch(TLPT_RECEIVE_USER, sessionData.user);
	      restApiActions.success(TRYING_TO_SIGN_UP);
	      session.getHistory().push({ pathname: cfg.routes.app });
	    }).fail(function () {
	      restApiActions.fail(TRYING_TO_SIGN_UP, 'failed to sing up');
	    });
	  },
	
	  login: function login(_ref2, redirect) {
	    var user = _ref2.user;
	    var password = _ref2.password;
	    var token = _ref2.token;
	
	    auth.login(user, password, token).done(function (sessionData) {
	      reactor.dispatch(TLPT_RECEIVE_USER, sessionData.user);
	      session.getHistory().push({ pathname: redirect });
	    }).fail(function () {});
	  }
	};
	module.exports = exports['default'];

/***/ },
/* 95 */
/***/ function(module, exports) {

	'use strict';
	
	exports.__esModule = true;
	var user = [['tlpt_user'], function (currentUser) {
	  if (!currentUser) {
	    return null;
	  }
	
	  return {
	    name: currentUser.get('name'),
	    logins: currentUser.get('allowed_logins').toJS()
	  };
	}];
	
	exports['default'] = {
	  user: user
	};
	module.exports = exports['default'];

/***/ },
/* 96 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(95);
	module.exports.actions = __webpack_require__(94);
	module.exports.nodeStore = __webpack_require__(97);

/***/ },
/* 97 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(18);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(93);
	
	var TLPT_RECEIVE_USER = _require2.TLPT_RECEIVE_USER;
	exports['default'] = Store({
	  getInitialState: function getInitialState() {
	    return toImmutable(null);
	  },
	
	  initialize: function initialize() {
	    this.on(TLPT_RECEIVE_USER, receiveUser);
	  }
	
	});
	
	function receiveUser(state, user) {
	  return toImmutable(user);
	}
	module.exports = exports['default'];

/***/ },
/* 98 */,
/* 99 */,
/* 100 */,
/* 101 */,
/* 102 */,
/* 103 */,
/* 104 */,
/* 105 */,
/* 106 */,
/* 107 */,
/* 108 */,
/* 109 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	"use strict";
	
	var React = __webpack_require__(5);
	
	var GoogleAuthInfo = React.createClass({
	  displayName: "GoogleAuthInfo",
	
	  render: function render() {
	    return React.createElement(
	      "div",
	      { className: "grv-google-auth" },
	      React.createElement("div", { className: "grv-google-auth-icon" }),
	      React.createElement(
	        "strong",
	        null,
	        "Google Authenticator"
	      ),
	      React.createElement(
	        "div",
	        null,
	        "Download ",
	        React.createElement(
	          "a",
	          { href: "https://support.google.com/accounts/answer/1066447?hl=en" },
	          "Google Authenticator"
	        ),
	        " on your phone to access your two factory token"
	      )
	    );
	  }
	});
	
	module.exports = GoogleAuthInfo;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "googleAuth.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 110 */,
/* 111 */,
/* 112 */,
/* 113 */,
/* 114 */,
/* 115 */,
/* 116 */,
/* 117 */,
/* 118 */,
/* 119 */,
/* 120 */,
/* 121 */,
/* 122 */,
/* 123 */,
/* 124 */,
/* 125 */,
/* 126 */,
/* 127 */,
/* 128 */,
/* 129 */,
/* 130 */,
/* 131 */,
/* 132 */,
/* 133 */,
/* 134 */,
/* 135 */,
/* 136 */,
/* 137 */,
/* 138 */,
/* 139 */,
/* 140 */,
/* 141 */,
/* 142 */,
/* 143 */,
/* 144 */,
/* 145 */,
/* 146 */,
/* 147 */,
/* 148 */
/***/ function(module, exports, __webpack_require__) {

	/*
	 *  The MIT License (MIT)
	 *  Copyright (c) 2015 Ryan Florence, Michael Jackson
	 *  Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:
	 *  The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.
	 *  THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	exports.compilePattern = compilePattern;
	exports.matchPattern = matchPattern;
	exports.getParamNames = getParamNames;
	exports.getParams = getParams;
	exports.formatPattern = formatPattern;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _invariant = __webpack_require__(10);
	
	var _invariant2 = _interopRequireDefault(_invariant);
	
	function escapeRegExp(string) {
	  return string.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
	}
	
	function escapeSource(string) {
	  return escapeRegExp(string).replace(/\/+/g, '/+');
	}
	
	function _compilePattern(pattern) {
	  var regexpSource = '';
	  var paramNames = [];
	  var tokens = [];
	
	  var match = undefined,
	      lastIndex = 0,
	      matcher = /:([a-zA-Z_$][a-zA-Z0-9_$]*)|\*\*|\*|\(|\)/g;
	  /*eslint no-cond-assign: 0*/
	  while (match = matcher.exec(pattern)) {
	    if (match.index !== lastIndex) {
	      tokens.push(pattern.slice(lastIndex, match.index));
	      regexpSource += escapeSource(pattern.slice(lastIndex, match.index));
	    }
	
	    if (match[1]) {
	      regexpSource += '([^/?#]+)';
	      paramNames.push(match[1]);
	    } else if (match[0] === '**') {
	      regexpSource += '([\\s\\S]*)';
	      paramNames.push('splat');
	    } else if (match[0] === '*') {
	      regexpSource += '([\\s\\S]*?)';
	      paramNames.push('splat');
	    } else if (match[0] === '(') {
	      regexpSource += '(?:';
	    } else if (match[0] === ')') {
	      regexpSource += ')?';
	    }
	
	    tokens.push(match[0]);
	
	    lastIndex = matcher.lastIndex;
	  }
	
	  if (lastIndex !== pattern.length) {
	    tokens.push(pattern.slice(lastIndex, pattern.length));
	    regexpSource += escapeSource(pattern.slice(lastIndex, pattern.length));
	  }
	
	  return {
	    pattern: pattern,
	    regexpSource: regexpSource,
	    paramNames: paramNames,
	    tokens: tokens
	  };
	}
	
	var CompiledPatternsCache = {};
	
	function compilePattern(pattern) {
	  if (!(pattern in CompiledPatternsCache)) CompiledPatternsCache[pattern] = _compilePattern(pattern);
	
	  return CompiledPatternsCache[pattern];
	}
	
	/**
	 * Attempts to match a pattern on the given pathname. Patterns may use
	 * the following special characters:
	 *
	 * - :paramName     Matches a URL segment up to the next /, ?, or #. The
	 *                  captured string is considered a "param"
	 * - ()             Wraps a segment of the URL that is optional
	 * - *              Consumes (non-greedy) all characters up to the next
	 *                  character in the pattern, or to the end of the URL if
	 *                  there is none
	 * - **             Consumes (greedy) all characters up to the next character
	 *                  in the pattern, or to the end of the URL if there is none
	 *
	 * The return value is an object with the following properties:
	 *
	 * - remainingPathname
	 * - paramNames
	 * - paramValues
	 */
	
	function matchPattern(pattern, pathname) {
	  // Make leading slashes consistent between pattern and pathname.
	  if (pattern.charAt(0) !== '/') {
	    pattern = '/' + pattern;
	  }
	  if (pathname.charAt(0) !== '/') {
	    pathname = '/' + pathname;
	  }
	
	  var _compilePattern2 = compilePattern(pattern);
	
	  var regexpSource = _compilePattern2.regexpSource;
	  var paramNames = _compilePattern2.paramNames;
	  var tokens = _compilePattern2.tokens;
	
	  regexpSource += '/*'; // Capture path separators
	
	  // Special-case patterns like '*' for catch-all routes.
	  var captureRemaining = tokens[tokens.length - 1] !== '*';
	
	  if (captureRemaining) {
	    // This will match newlines in the remaining path.
	    regexpSource += '([\\s\\S]*?)';
	  }
	
	  var match = pathname.match(new RegExp('^' + regexpSource + '$', 'i'));
	
	  var remainingPathname = undefined,
	      paramValues = undefined;
	  if (match != null) {
	    if (captureRemaining) {
	      remainingPathname = match.pop();
	      var matchedPath = match[0].substr(0, match[0].length - remainingPathname.length);
	
	      // If we didn't match the entire pathname, then make sure that the match
	      // we did get ends at a path separator (potentially the one we added
	      // above at the beginning of the path, if the actual match was empty).
	      if (remainingPathname && matchedPath.charAt(matchedPath.length - 1) !== '/') {
	        return {
	          remainingPathname: null,
	          paramNames: paramNames,
	          paramValues: null
	        };
	      }
	    } else {
	      // If this matched at all, then the match was the entire pathname.
	      remainingPathname = '';
	    }
	
	    paramValues = match.slice(1).map(function (v) {
	      return v != null ? decodeURIComponent(v) : v;
	    });
	  } else {
	    remainingPathname = paramValues = null;
	  }
	
	  return {
	    remainingPathname: remainingPathname,
	    paramNames: paramNames,
	    paramValues: paramValues
	  };
	}
	
	function getParamNames(pattern) {
	  return compilePattern(pattern).paramNames;
	}
	
	function getParams(pattern, pathname) {
	  var _matchPattern = matchPattern(pattern, pathname);
	
	  var paramNames = _matchPattern.paramNames;
	  var paramValues = _matchPattern.paramValues;
	
	  if (paramValues != null) {
	    return paramNames.reduce(function (memo, paramName, index) {
	      memo[paramName] = paramValues[index];
	      return memo;
	    }, {});
	  }
	
	  return null;
	}
	
	/**
	 * Returns a version of the given pattern with params interpolated. Throws
	 * if there is a dynamic segment of the pattern for which there is no param.
	 */
	
	function formatPattern(pattern, params) {
	  params = params || {};
	
	  var _compilePattern3 = compilePattern(pattern);
	
	  var tokens = _compilePattern3.tokens;
	
	  var parenCount = 0,
	      pathname = '',
	      splatIndex = 0;
	
	  var token = undefined,
	      paramName = undefined,
	      paramValue = undefined;
	  for (var i = 0, len = tokens.length; i < len; ++i) {
	    token = tokens[i];
	
	    if (token === '*' || token === '**') {
	      paramValue = Array.isArray(params.splat) ? params.splat[splatIndex++] : params.splat;
	
	      _invariant2['default'](paramValue != null || parenCount > 0, 'Missing splat #%s for path "%s"', splatIndex, pattern);
	
	      if (paramValue != null) pathname += encodeURI(paramValue);
	    } else if (token === '(') {
	      parenCount += 1;
	    } else if (token === ')') {
	      parenCount -= 1;
	    } else if (token.charAt(0) === ':') {
	      paramName = token.substring(1);
	      paramValue = params[paramName];
	
	      _invariant2['default'](paramValue != null || parenCount > 0, 'Missing "%s" parameter for path "%s"', paramName, pattern);
	
	      if (paramValue != null) pathname += encodeURIComponent(paramValue);
	    } else {
	      pathname += token;
	    }
	  }
	
	  return pathname.replace(/\/+/g, '/');
	}

/***/ },
/* 149 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(11);
	var api = __webpack_require__(39);
	var cfg = __webpack_require__(14);
	
	var _require = __webpack_require__(92);
	
	var TLPT_SESSINS_RECEIVE = _require.TLPT_SESSINS_RECEIVE;
	
	var _require2 = __webpack_require__(53);
	
	var TLPT_NODES_RECEIVE = _require2.TLPT_NODES_RECEIVE;
	exports['default'] = {
	  fetchNodesAndSessions: function fetchNodesAndSessions() {
	    api.get(cfg.api.nodesPath).done(function (json) {
	      var nodeArray = [];
	      var sessions = {};
	
	      json.nodes.forEach(function (item) {
	        nodeArray.push(item.node);
	        if (item.sessions) {
	          item.sessions.forEach(function (item2) {
	            sessions[item2.id] = item2;
	          });
	        }
	      });
	
	      reactor.batch(function () {
	        reactor.dispatch(TLPT_NODES_RECEIVE, nodeArray);
	        reactor.dispatch(TLPT_SESSINS_RECEIVE, sessions);
	      });
	    });
	  }
	};
	module.exports = exports['default'];

/***/ },
/* 150 */
/***/ function(module, exports) {

	'use strict';
	
	exports.__esModule = true;
	var activeSession = [['tlpt_active_terminal'], function (activeSession) {
	  if (!activeSession) {
	    return null;
	  }
	
	  return activeSession.toJS();
	}];
	
	exports['default'] = {
	  activeSession: activeSession
	};
	module.exports = exports['default'];

/***/ },
/* 151 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(150);
	module.exports.actions = __webpack_require__(85);
	module.exports.activeTermStore = __webpack_require__(86);

/***/ },
/* 152 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var reactor = __webpack_require__(11);
	reactor.registerStores({
	  'tlpt_active_terminal': __webpack_require__(86),
	  'tlpt_user': __webpack_require__(97),
	  'tlpt_nodes': __webpack_require__(89),
	  'tlpt_invite': __webpack_require__(88),
	  'tlpt_rest_api': __webpack_require__(160),
	  'tlpt_sessions': __webpack_require__(162)
	});

/***/ },
/* 153 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(11);
	
	var _require = __webpack_require__(87);
	
	var TLPT_RECEIVE_USER_INVITE = _require.TLPT_RECEIVE_USER_INVITE;
	
	var api = __webpack_require__(39);
	var cfg = __webpack_require__(14);
	
	exports['default'] = {
	  fetchInvite: function fetchInvite(inviteToken) {
	    var path = cfg.api.getInviteUrl(inviteToken);
	    api.get(path).done(function (invite) {
	      reactor.dispatch(TLPT_RECEIVE_USER_INVITE, invite);
	    });
	  }
	};
	module.exports = exports['default'];

/***/ },
/* 154 */
/***/ function(module, exports, __webpack_require__) {

	/*eslint no-undef: 0,  no-unused-vars: 0, no-debugger:0*/
	
	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(91);
	
	var TRYING_TO_SIGN_UP = _require.TRYING_TO_SIGN_UP;
	
	var invite = [['tlpt_invite'], function (invite) {
	  return invite;
	}];
	
	var attemp = [['tlpt_rest_api', TRYING_TO_SIGN_UP], function (attemp) {
	  var defaultObj = {
	    isProcessing: false,
	    isError: false,
	    isSuccess: false,
	    message: ''
	  };
	
	  return attemp ? attemp.toJS() : defaultObj;
	}];
	
	exports['default'] = {
	  invite: invite,
	  attemp: attemp
	};
	module.exports = exports['default'];

/***/ },
/* 155 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(154);
	module.exports.actions = __webpack_require__(153);
	module.exports.nodeStore = __webpack_require__(88);

/***/ },
/* 156 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(11);
	
	var _require = __webpack_require__(53);
	
	var TLPT_NODES_RECEIVE = _require.TLPT_NODES_RECEIVE;
	
	var api = __webpack_require__(39);
	var cfg = __webpack_require__(14);
	
	exports['default'] = {
	  fetchNodes: function fetchNodes() {
	    api.get(cfg.api.nodesPath).done(function (data) {
	      reactor.dispatch(TLPT_NODES_RECEIVE, data.nodes);
	    });
	  }
	};
	module.exports = exports['default'];

/***/ },
/* 157 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(11);
	
	var _require = __webpack_require__(161);
	
	var sessionsByServer = _require.sessionsByServer;
	
	var nodeListView = [['tlpt_nodes'], function (nodes) {
	  return nodes.map(function (item) {
	    var addr = item.get('addr');
	    var sessions = reactor.evaluate(sessionsByServer(addr));
	    return {
	      tags: getTags(item),
	      addr: addr,
	      sessionCount: sessions.size
	    };
	  }).toJS();
	}];
	
	function getTags(node) {
	  var allLabels = [];
	  var labels = node.get('labels');
	
	  if (labels) {
	    labels.entrySeq().toArray().forEach(function (item) {
	      allLabels.push({
	        role: item[0],
	        value: item[1]
	      });
	    });
	  }
	
	  labels = node.get('cmd_labels');
	
	  if (labels) {
	    labels.entrySeq().toArray().forEach(function (item) {
	      allLabels.push({
	        role: item[0],
	        value: item[1].get('result'),
	        tooltip: item[1].get('command')
	      });
	    });
	  }
	
	  return allLabels;
	}
	
	exports['default'] = {
	  nodeListView: nodeListView
	};
	module.exports = exports['default'];

/***/ },
/* 158 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(157);
	module.exports.actions = __webpack_require__(156);
	module.exports.nodeStore = __webpack_require__(89);
	
	// nodes: [{"id":"x220","addr":"0.0.0.0:3022","hostname":"x220","labels":null,"cmd_labels":null}]

	// sessions: [{"id":"07630636-bb3d-40e1-b086-60b2cae21ac4","parties":[{"id":"89f762a3-7429-4c7a-a913-766493fe7c8a","site":"127.0.0.1:37514","user":"akontsevoy","server_addr":"0.0.0.0:3022","last_active":"2016-02-22T14:39:20.93120535-05:00"}]}]

	/*
	let TodoRecord = Immutable.Record({
	    id: 0,
	    description: "",
	    completed: false
	});
	*/

/***/ },
/* 159 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(11);
	
	var _require = __webpack_require__(90);
	
	var TLPT_REST_API_START = _require.TLPT_REST_API_START;
	var TLPT_REST_API_SUCCESS = _require.TLPT_REST_API_SUCCESS;
	var TLPT_REST_API_FAIL = _require.TLPT_REST_API_FAIL;
	exports['default'] = {
	
	  start: function start(reqType) {
	    reactor.dispatch(TLPT_REST_API_START, { type: reqType });
	  },
	
	  fail: function fail(reqType, message) {
	    reactor.dispatch(TLPT_REST_API_FAIL, { type: reqType, message: message });
	  },
	
	  success: function success(reqType) {
	    reactor.dispatch(TLPT_REST_API_SUCCESS, { type: reqType });
	  }
	
	};
	module.exports = exports['default'];

/***/ },
/* 160 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(18);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(90);
	
	var TLPT_REST_API_START = _require2.TLPT_REST_API_START;
	var TLPT_REST_API_SUCCESS = _require2.TLPT_REST_API_SUCCESS;
	var TLPT_REST_API_FAIL = _require2.TLPT_REST_API_FAIL;
	exports['default'] = Store({
	  getInitialState: function getInitialState() {
	    return toImmutable({});
	  },
	
	  initialize: function initialize() {
	    this.on(TLPT_REST_API_START, start);
	    this.on(TLPT_REST_API_FAIL, fail);
	    this.on(TLPT_REST_API_SUCCESS, success);
	  }
	});
	
	function start(state, request) {
	  return state.set(request.type, toImmutable({ isProcessing: true }));
	}
	
	function fail(state, request) {
	  return state.set(request.type, toImmutable({ isFailed: true, message: request.message }));
	}
	
	function success(state, request) {
	  return state.set(request.type, toImmutable({ isSuccess: true }));
	}
	module.exports = exports['default'];

/***/ },
/* 161 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(18);
	
	var toImmutable = _require.toImmutable;
	
	var sessionsByServer = function sessionsByServer(addr) {
	  return [['tlpt_sessions'], function (sessions) {
	    return sessions.valueSeq().filter(function (item) {
	      var parties = item.get('parties') || toImmutable([]);
	      var hasServer = parties.find(function (item2) {
	        return item2.get('server_addr') === addr;
	      });
	      return hasServer;
	    }).toList();
	  }];
	};
	
	var partiesBySessionId = function partiesBySessionId(sid) {
	  return [['tlpt_sessions', sid, 'parties'], function (parties) {
	
	    if (!parties) {
	      return [];
	    }
	
	    var lastActiveUsrName = getLastActiveUser(parties);
	
	    return parties.map(function (item) {
	      var user = item.get('user');
	      return {
	        user: item.get('user'),
	        isActive: lastActiveUsrName === user
	      };
	    }).toJS();
	  }];
	};
	
	function getLastActiveUser(parties) {
	  return parties.sortBy(function (item) {
	    return new Date(item.get('lastActive'));
	  }).first().get('user');
	}
	
	exports['default'] = {
	  partiesBySessionId: partiesBySessionId,
	  sessionsByServer: sessionsByServer
	};
	module.exports = exports['default'];

/***/ },
/* 162 */
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(18);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(92);
	
	var TLPT_SESSINS_RECEIVE = _require2.TLPT_SESSINS_RECEIVE;
	var TLPT_SESSINS_UPDATE = _require2.TLPT_SESSINS_UPDATE;
	exports['default'] = Store({
	  getInitialState: function getInitialState() {
	    return toImmutable({});
	  },
	
	  initialize: function initialize() {
	    this.on(TLPT_SESSINS_RECEIVE, receiveSessions);
	    this.on(TLPT_SESSINS_UPDATE, updateSession);
	  }
	});
	
	function updateSession(state, json) {
	  return state.set(json.id, toImmutable(json));
	}
	
	function receiveSessions(state, json) {
	  return toImmutable(json);
	}
	module.exports = exports['default'];

/***/ },
/* 163 */
/***/ function(module, exports) {

	'use strict';
	
	var utils = {
	
	  uuid: function uuid() {
	    // never use it in production
	    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
	      var r = Math.random() * 16 | 0,
	          v = c == 'x' ? r : r & 0x3 | 0x8;
	      return v.toString(16);
	    });
	  },
	
	  displayDate: function displayDate(date) {
	    try {
	      return date.toLocaleDateString() + ' ' + date.toLocaleTimeString();
	    } catch (err) {
	      console.error(err);
	      return 'undefined';
	    }
	  },
	
	  formatString: function formatString(format) {
	    var args = Array.prototype.slice.call(arguments, 1);
	    return format.replace(new RegExp('\\{(\\d+)\\}', 'g'), function (match, number) {
	      return !(args[number] === null || args[number] === undefined) ? args[number] : '';
	    });
	  }
	};
	
	module.exports = utils;

/***/ },
/* 164 */,
/* 165 */,
/* 166 */,
/* 167 */,
/* 168 */,
/* 169 */,
/* 170 */,
/* 171 */,
/* 172 */,
/* 173 */,
/* 174 */,
/* 175 */,
/* 176 */,
/* 177 */,
/* 178 */,
/* 179 */,
/* 180 */,
/* 181 */,
/* 182 */,
/* 183 */,
/* 184 */,
/* 185 */,
/* 186 */,
/* 187 */,
/* 188 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	var NavLeftBar = __webpack_require__(192);
	var cfg = __webpack_require__(14);
	
	var _require = __webpack_require__(197);
	
	var TerminalHost = _require.TerminalHost;
	
	var actions = __webpack_require__(149);
	
	var App = React.createClass({
	  displayName: 'App',
	
	  componentDidMount: function componentDidMount() {
	    actions.fetchNodesAndSessions();
	  },
	
	  render: function render() {
	    return React.createElement(
	      'div',
	      { className: 'grv-tlpt' },
	      React.createElement(TerminalHost, null),
	      React.createElement(NavLeftBar, null),
	      React.createElement(
	        'div',
	        { className: 'row' },
	        React.createElement(
	          'nav',
	          { className: '', role: 'navigation', style: { marginBottom: 0 } },
	          React.createElement(
	            'ul',
	            { className: 'nav navbar-top-links navbar-right' },
	            React.createElement(
	              'li',
	              null,
	              React.createElement(
	                'a',
	                { href: cfg.routes.logout },
	                React.createElement('i', { className: 'fa fa-sign-out' }),
	                'Log out'
	              )
	            )
	          )
	        )
	      ),
	      React.createElement(
	        'div',
	        { className: 'grv-page' },
	        this.props.children
	      )
	    );
	  }
	});
	
	module.exports = App;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "app.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 189 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	exports.__esModule = true;
	var cfg = __webpack_require__(14);
	var React = __webpack_require__(5);
	
	var EventStreamer = React.createClass({
	  displayName: 'EventStreamer',
	
	  componentDidMount: function componentDidMount() {
	    var _props = this.props;
	    var token = _props.token;
	    var sid = _props.sid;
	
	    var connStr = cfg.api.getEventStreamerConnStr(token, sid);
	
	    this.socket = new WebSocket(connStr, "proto");
	    this.socket.onmessage = function () {};
	    this.socket.onclose = function () {};
	  },
	
	  componentWillUnmount: function componentWillUnmount() {
	    this.socket.close();
	  },
	
	  shouldComponentUpdate: function shouldComponentUpdate() {
	    return false;
	  },
	
	  render: function render() {
	    return null;
	  }
	});
	
	exports['default'] = EventStreamer;
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "eventStreamer.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 190 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	module.exports.App = __webpack_require__(188);
	module.exports.Login = __webpack_require__(191);
	module.exports.NewUser = __webpack_require__(193);
	module.exports.Nodes = __webpack_require__(194);
	module.exports.Sessions = __webpack_require__(195);
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "index.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 191 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };
	
	var React = __webpack_require__(5);
	var $ = __webpack_require__(52);
	var reactor = __webpack_require__(11);
	var LinkedStateMixin = __webpack_require__(58);
	
	var _require = __webpack_require__(96);
	
	var actions = _require.actions;
	
	var GoogleAuthInfo = __webpack_require__(109);
	var LoginInputForm = React.createClass({
	  displayName: 'LoginInputForm',
	
	  mixins: [LinkedStateMixin],
	
	  getInitialState: function getInitialState() {
	    return {
	      user: '',
	      password: '',
	      token: ''
	    };
	  },
	
	  onClick: function onClick(e) {
	    e.preventDefault();
	    if (this.isValid()) {
	      actions.login(_extends({}, this.state), '/web');
	    }
	  },
	
	  isValid: function isValid() {
	    var $form = $(this.refs.form);
	    return $form.length === 0 || $form.valid();
	  },
	
	  render: function render() {
	    return React.createElement(
	      'form',
	      { ref: 'form', className: 'grv-login-input-form' },
	      React.createElement(
	        'h3',
	        null,
	        ' Welcome to Teleport '
	      ),
	      React.createElement(
	        'div',
	        { className: '' },
	        React.createElement(
	          'div',
	          { className: 'form-group' },
	          React.createElement('input', { valueLink: this.linkState('user'), className: 'form-control required', placeholder: 'User name', name: 'userName' })
	        ),
	        React.createElement(
	          'div',
	          { className: 'form-group' },
	          React.createElement('input', { valueLink: this.linkState('password'), type: 'password', name: 'password', className: 'form-control required', placeholder: 'Password' })
	        ),
	        React.createElement(
	          'div',
	          { className: 'form-group' },
	          React.createElement('input', { valueLink: this.linkState('token'), className: 'form-control required', name: 'token', placeholder: 'Two factor token (Google Authenticator)' })
	        ),
	        React.createElement(
	          'button',
	          { type: 'submit', className: 'btn btn-primary block full-width m-b', onClick: this.onClick },
	          'Login'
	        )
	      )
	    );
	  }
	});
	
	var Login = React.createClass({
	  displayName: 'Login',
	
	  mixins: [reactor.ReactMixin],
	
	  getDataBindings: function getDataBindings() {
	    return {
	      //    userRequest: getters.userRequest
	    };
	  },
	
	  render: function render() {
	    var isProcessing = false; //this.state.userRequest.get('isLoading');
	    var isError = false; //this.state.userRequest.get('isError');
	
	    return React.createElement(
	      'div',
	      { className: 'grv-login text-center' },
	      React.createElement('div', { className: 'grv-logo-tprt' }),
	      React.createElement(
	        'div',
	        { className: 'grv-content grv-flex' },
	        React.createElement(
	          'div',
	          { className: 'grv-flex-column' },
	          React.createElement(LoginInputForm, null),
	          React.createElement(GoogleAuthInfo, null),
	          React.createElement(
	            'div',
	            { className: 'grv-login-info' },
	            React.createElement('i', { className: 'fa fa-question' }),
	            React.createElement(
	              'strong',
	              null,
	              'New Account or forgot password?'
	            ),
	            React.createElement(
	              'div',
	              null,
	              'Ask for assistance from your Company administrator'
	            )
	          )
	        )
	      )
	    );
	  }
	});
	
	module.exports = Login;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "login.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 192 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	
	var _require = __webpack_require__(43);
	
	var Router = _require.Router;
	var IndexLink = _require.IndexLink;
	var History = _require.History;
	
	var cfg = __webpack_require__(14);
	
	var menuItems = [{ icon: 'fa fa-cogs', to: cfg.routes.nodes, title: 'Nodes' }, { icon: 'fa fa-sitemap', to: cfg.routes.sessions, title: 'Sessions' }, { icon: 'fa fa-question', to: cfg.routes.sessions, title: 'Sessions' }];
	
	var NavLeftBar = React.createClass({
	  displayName: 'NavLeftBar',
	
	  render: function render() {
	    var _this = this;
	
	    var items = menuItems.map(function (i, index) {
	      var className = _this.context.router.isActive(i.to) ? 'active' : '';
	      return React.createElement(
	        'li',
	        { key: index, className: className },
	        React.createElement(
	          IndexLink,
	          { to: i.to },
	          React.createElement('i', { className: i.icon, title: i.title })
	        )
	      );
	    });
	
	    return React.createElement(
	      'nav',
	      { className: '', role: 'navigation', style: { width: '60px', float: 'left', position: 'absolute' } },
	      React.createElement(
	        'div',
	        { className: '' },
	        React.createElement(
	          'ul',
	          { className: 'nav 1metismenu', id: 'side-menu' },
	          items
	        )
	      )
	    );
	  }
	});
	
	NavLeftBar.contextTypes = {
	  router: React.PropTypes.object.isRequired
	};
	
	module.exports = NavLeftBar;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "navLeftBar.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 193 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	var $ = __webpack_require__(52);
	var reactor = __webpack_require__(11);
	
	var _require = __webpack_require__(155);
	
	var actions = _require.actions;
	var getters = _require.getters;
	
	var userModule = __webpack_require__(96);
	var LinkedStateMixin = __webpack_require__(58);
	var GoogleAuthInfo = __webpack_require__(109);
	
	var InviteInputForm = React.createClass({
	  displayName: 'InviteInputForm',
	
	  mixins: [LinkedStateMixin],
	
	  componentDidMount: function componentDidMount() {
	    $(this.refs.form).validate({
	      rules: {
	        password: {
	          minlength: 5,
	          required: true
	        },
	        passwordConfirmed: {
	          required: true,
	          equalTo: this.refs.password
	        }
	      },
	
	      messages: {
	        passwordConfirmed: {
	          minlength: $.validator.format('Enter at least {0} characters'),
	          equalTo: 'Enter the same password as above'
	        }
	      }
	    });
	  },
	
	  getInitialState: function getInitialState() {
	    return {
	      name: this.props.invite.user,
	      psw: '',
	      pswConfirmed: '',
	      token: ''
	    };
	  },
	
	  onClick: function onClick(e) {
	    e.preventDefault();
	    if (this.isValid()) {
	      userModule.actions.signUp({
	        name: this.state.name,
	        psw: this.state.psw,
	        token: this.state.token,
	        inviteToken: this.props.invite.invite_token });
	    }
	  },
	
	  isValid: function isValid() {
	    var $form = $(this.refs.form);
	    return $form.length === 0 || $form.valid();
	  },
	
	  render: function render() {
	    return React.createElement(
	      'form',
	      { ref: 'form', className: 'grv-invite-input-form' },
	      React.createElement(
	        'h3',
	        null,
	        ' Get started with Teleport '
	      ),
	      React.createElement(
	        'div',
	        { className: '' },
	        React.createElement(
	          'div',
	          { className: 'form-group' },
	          React.createElement('input', {
	            valueLink: this.linkState('name'),
	            name: 'userName',
	            className: 'form-control required',
	            placeholder: 'User name' })
	        ),
	        React.createElement(
	          'div',
	          { className: 'form-group' },
	          React.createElement('input', {
	            valueLink: this.linkState('psw'),
	            ref: 'password',
	            type: 'password',
	            name: 'password',
	            className: 'form-control',
	            placeholder: 'Password' })
	        ),
	        React.createElement(
	          'div',
	          { className: 'form-group grv-' },
	          React.createElement('input', {
	            valueLink: this.linkState('pswConfirmed'),
	            type: 'password',
	            name: 'passwordConfirmed',
	            className: 'form-control',
	            placeholder: 'Password confirm' })
	        ),
	        React.createElement(
	          'div',
	          { className: 'form-group' },
	          React.createElement('input', {
	            name: 'token',
	            valueLink: this.linkState('token'),
	            className: 'form-control required',
	            placeholder: 'Two factor token (Google Authenticator)' })
	        ),
	        React.createElement(
	          'button',
	          { type: 'submit', disabled: this.props.attemp.isProcessing, className: 'btn btn-primary block full-width m-b', onClick: this.onClick },
	          'Sign up'
	        )
	      )
	    );
	  }
	});
	
	var Invite = React.createClass({
	  displayName: 'Invite',
	
	  mixins: [reactor.ReactMixin],
	
	  getDataBindings: function getDataBindings() {
	    return {
	      invite: getters.invite,
	      attemp: getters.attemp
	    };
	  },
	
	  componentDidMount: function componentDidMount() {
	    actions.fetchInvite(this.props.params.inviteToken);
	  },
	
	  render: function render() {
	    if (!this.state.invite) {
	      return null;
	    }
	
	    return React.createElement(
	      'div',
	      { className: 'grv-invite text-center' },
	      React.createElement('div', { className: 'grv-logo-tprt' }),
	      React.createElement(
	        'div',
	        { className: 'grv-content grv-flex' },
	        React.createElement(
	          'div',
	          { className: 'grv-flex-column' },
	          React.createElement(InviteInputForm, { attemp: this.state.attemp, invite: this.state.invite.toJS() }),
	          React.createElement(GoogleAuthInfo, null)
	        ),
	        React.createElement(
	          'div',
	          { className: 'grv-flex-column' },
	          React.createElement(
	            'h4',
	            null,
	            'Scan bar code for auth token ',
	            React.createElement('br', null),
	            ' ',
	            React.createElement(
	              'small',
	              null,
	              'Scan below to generate your two factor token'
	            )
	          ),
	          React.createElement('img', { className: 'img-thumbnail', src: 'data:image/png;base64,' + this.state.invite.get('qr') })
	        )
	      )
	    );
	  }
	});
	
	module.exports = Invite;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "newUser.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 194 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }
	
	var React = __webpack_require__(5);
	var reactor = __webpack_require__(11);
	
	var _require = __webpack_require__(158);
	
	var getters = _require.getters;
	var actions = _require.actions;
	
	var userGetters = __webpack_require__(95);
	
	var _require2 = __webpack_require__(196);
	
	var Table = _require2.Table;
	var Column = _require2.Column;
	var Cell = _require2.Cell;
	
	var _require3 = __webpack_require__(85);
	
	var open = _require3.open;
	
	var TextCell = function TextCell(_ref) {
	  var rowIndex = _ref.rowIndex;
	  var data = _ref.data;
	  var columnKey = _ref.columnKey;
	
	  var props = _objectWithoutProperties(_ref, ['rowIndex', 'data', 'columnKey']);
	
	  return React.createElement(
	    Cell,
	    props,
	    data[rowIndex][columnKey]
	  );
	};
	
	var TagCell = function TagCell(_ref2) {
	  var rowIndex = _ref2.rowIndex;
	  var data = _ref2.data;
	  var columnKey = _ref2.columnKey;
	
	  var props = _objectWithoutProperties(_ref2, ['rowIndex', 'data', 'columnKey']);
	
	  return React.createElement(
	    Cell,
	    props,
	    data[rowIndex].tags.map(function (item, index) {
	      return React.createElement(
	        'span',
	        { key: index, className: 'label label-default' },
	        item.role,
	        ' ',
	        React.createElement('li', { className: 'fa fa-long-arrow-right' }),
	        item.value
	      );
	    })
	  );
	};
	
	var LoginCell = function LoginCell(_ref3) {
	  var user = _ref3.user;
	  var rowIndex = _ref3.rowIndex;
	  var data = _ref3.data;
	
	  var props = _objectWithoutProperties(_ref3, ['user', 'rowIndex', 'data']);
	
	  if (!user || user.logins.length === 0) {
	    return React.createElement(Cell, props);
	  }
	
	  var $lis = [];
	
	  for (var i = 0; i < user.logins.length; i++) {
	    $lis.push(React.createElement(
	      'li',
	      { key: i },
	      React.createElement(
	        'a',
	        { href: '#', target: '_blank', onClick: open.bind(null, data[rowIndex].addr, user.logins[i], undefined) },
	        user.logins[i]
	      )
	    ));
	  }
	
	  return React.createElement(
	    Cell,
	    props,
	    React.createElement(
	      'div',
	      { className: 'btn-group' },
	      React.createElement(
	        'button',
	        { type: 'button', onClick: open.bind(null, data[rowIndex].addr, user.logins[0], undefined), className: 'btn btn-sm btn-primary' },
	        user.logins[0]
	      ),
	      $lis.length > 1 ? React.createElement(
	        'div',
	        { className: 'btn-group' },
	        React.createElement(
	          'button',
	          { 'data-toggle': 'dropdown', className: 'btn btn-default btn-sm dropdown-toggle', 'aria-expanded': 'true' },
	          React.createElement('span', { className: 'caret' })
	        ),
	        React.createElement(
	          'ul',
	          { className: 'dropdown-menu' },
	          React.createElement(
	            'li',
	            null,
	            React.createElement(
	              'a',
	              { href: '#', target: '_blank' },
	              'Logs'
	            )
	          ),
	          React.createElement(
	            'li',
	            null,
	            React.createElement(
	              'a',
	              { href: '#', target: '_blank' },
	              'Logs'
	            )
	          )
	        )
	      ) : null
	    )
	  );
	};
	
	var Nodes = React.createClass({
	  displayName: 'Nodes',
	
	  mixins: [reactor.ReactMixin],
	
	  getDataBindings: function getDataBindings() {
	    return {
	      nodeRecords: getters.nodeListView,
	      user: userGetters.user
	    };
	  },
	
	  render: function render() {
	    var data = this.state.nodeRecords;
	    return React.createElement(
	      'div',
	      { className: 'grv-nodes' },
	      React.createElement(
	        'h1',
	        null,
	        ' Nodes '
	      ),
	      React.createElement(
	        'div',
	        { className: '' },
	        React.createElement(
	          'div',
	          { className: '' },
	          React.createElement(
	            'div',
	            { className: '' },
	            React.createElement(
	              Table,
	              { rowCount: data.length, className: 'table-stripped grv-nodes-table' },
	              React.createElement(Column, {
	                columnKey: 'sessionCount',
	                header: React.createElement(
	                  Cell,
	                  null,
	                  ' Sessions '
	                ),
	                cell: React.createElement(TextCell, { data: data })
	              }),
	              React.createElement(Column, {
	                columnKey: 'addr',
	                header: React.createElement(
	                  Cell,
	                  null,
	                  ' Node '
	                ),
	                cell: React.createElement(TextCell, { data: data })
	              }),
	              React.createElement(Column, {
	                columnKey: 'tags',
	                header: React.createElement(Cell, null),
	                cell: React.createElement(TagCell, { data: data })
	              }),
	              React.createElement(Column, {
	                columnKey: 'roles',
	                header: React.createElement(
	                  Cell,
	                  null,
	                  'Login as'
	                ),
	                cell: React.createElement(LoginCell, { data: data, user: this.state.user })
	              })
	            )
	          )
	        )
	      )
	    );
	  }
	});
	
	module.exports = Nodes;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "main.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 195 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	var reactor = __webpack_require__(11);
	var Nodes = React.createClass({
	  displayName: 'Nodes',
	
	  render: function render() {
	    return React.createElement(
	      'div',
	      null,
	      React.createElement(
	        'h1',
	        null,
	        ' Sessions!'
	      ),
	      React.createElement(
	        'div',
	        { className: '' },
	        React.createElement(
	          'div',
	          { className: '' },
	          React.createElement(
	            'div',
	            { className: '' },
	            React.createElement(
	              'table',
	              { className: 'table table-striped' },
	              React.createElement(
	                'thead',
	                null,
	                React.createElement(
	                  'tr',
	                  null,
	                  React.createElement(
	                    'th',
	                    null,
	                    'Node'
	                  ),
	                  React.createElement(
	                    'th',
	                    null,
	                    'Status'
	                  ),
	                  React.createElement(
	                    'th',
	                    null,
	                    'Labels'
	                  ),
	                  React.createElement(
	                    'th',
	                    null,
	                    'CPU'
	                  ),
	                  React.createElement(
	                    'th',
	                    null,
	                    'RAM'
	                  ),
	                  React.createElement(
	                    'th',
	                    null,
	                    'OS'
	                  ),
	                  React.createElement(
	                    'th',
	                    null,
	                    ' Last Heartbeat '
	                  )
	                )
	              ),
	              React.createElement('tbody', null)
	            )
	          )
	        )
	      )
	    );
	  }
	});
	
	module.exports = Nodes;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "main.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 196 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	exports.__esModule = true;
	
	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };
	
	var React = __webpack_require__(5);
	
	var GrvTableCell = React.createClass({
	  displayName: 'GrvTableCell',
	
	  render: function render() {
	    var props = this.props;
	    return props.isHeader ? React.createElement(
	      'th',
	      { key: props.key },
	      props.children
	    ) : React.createElement(
	      'td',
	      { key: props.key },
	      props.children
	    );
	  }
	});
	
	var GrvTable = React.createClass({
	  displayName: 'GrvTable',
	
	  renderHeader: function renderHeader(children) {
	    var _this = this;
	
	    var cells = children.map(function (item, index) {
	      return _this.renderCell(item.props.header, _extends({ index: index, key: index, isHeader: true }, item.props));
	    });
	
	    return React.createElement(
	      'thead',
	      null,
	      React.createElement(
	        'tr',
	        null,
	        cells
	      )
	    );
	  },
	
	  renderBody: function renderBody(children) {
	    var _this2 = this;
	
	    var count = this.props.rowCount;
	    var rows = [];
	    for (var i = 0; i < count; i++) {
	      var cells = children.map(function (item, index) {
	        return _this2.renderCell(item.props.cell, _extends({ rowIndex: i, key: index, isHeader: false }, item.props));
	      });
	
	      rows.push(React.createElement(
	        'tr',
	        { key: i },
	        cells
	      ));
	    }
	
	    return React.createElement(
	      'tbody',
	      null,
	      rows
	    );
	  },
	
	  renderCell: function renderCell(cell, cellProps) {
	    var content = null;
	    if (React.isValidElement(cell)) {
	      content = React.cloneElement(cell, cellProps);
	    } else if (typeof props.cell === 'function') {
	      content = cell(cellProps);
	    }
	
	    return content;
	  },
	
	  render: function render() {
	    var children = [];
	    React.Children.forEach(this.props.children, function (child, index) {
	      if (child == null) {
	        return;
	      }
	
	      if (child.type.displayName !== 'GrvTableColumn') {
	        throw 'Should be GrvTableColumn';
	      }
	
	      children.push(child);
	    });
	
	    var tableClass = 'table ' + this.props.className;
	
	    return React.createElement(
	      'table',
	      { className: tableClass },
	      this.renderHeader(children),
	      this.renderBody(children)
	    );
	  }
	});
	
	var GrvTableColumn = React.createClass({
	  displayName: 'GrvTableColumn',
	
	  render: function render() {
	    throw new Error('Component <GrvTableColumn /> should never render');
	  }
	});
	
	exports['default'] = GrvTable;
	exports.Column = GrvTableColumn;
	exports.Table = GrvTable;
	exports.Cell = GrvTableCell;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "table.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 197 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	exports.__esModule = true;
	
	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }
	
	var session = __webpack_require__(36);
	var cfg = __webpack_require__(14);
	var React = __webpack_require__(5);
	
	var _require = __webpack_require__(151);
	
	var getters = _require.getters;
	var actions = _require.actions;
	
	var EventStreamer = __webpack_require__(189);
	
	var TerminalHost = React.createClass({
	  displayName: 'TerminalHost',
	
	  mixins: [reactor.ReactMixin],
	
	  onOpen: function onOpen() {
	    actions.connected();
	  },
	
	  getDataBindings: function getDataBindings() {
	    return {
	      activeSession: getters.activeSession
	    };
	  },
	
	  render: function render() {
	    if (!this.state.activeSession) {
	      return null;
	    }
	
	    var _state$activeSession = this.state.activeSession;
	    var isConnected = _state$activeSession.isConnected;
	
	    var settings = _objectWithoutProperties(_state$activeSession, ['isConnected']);
	
	    var _session$getUserData = session.getUserData();
	
	    var token = _session$getUserData.token;
	
	    return React.createElement(
	      'div',
	      { className: 'grv-terminal-host' },
	      React.createElement(
	        'div',
	        { className: 'grv-terminal-participans' },
	        React.createElement(
	          'ul',
	          { className: 'nav' },
	          React.createElement(
	            'li',
	            null,
	            React.createElement(
	              'button',
	              { className: 'btn btn-primary btn-circle', type: 'button' },
	              ' ',
	              React.createElement(
	                'strong',
	                null,
	                'A'
	              )
	            )
	          ),
	          React.createElement(
	            'li',
	            null,
	            React.createElement(
	              'button',
	              { className: 'btn btn-primary btn-circle', type: 'button' },
	              ' B '
	            )
	          ),
	          React.createElement(
	            'li',
	            null,
	            React.createElement(
	              'button',
	              { className: 'btn btn-primary btn-circle', type: 'button' },
	              ' C '
	            )
	          ),
	          React.createElement(
	            'li',
	            null,
	            React.createElement(
	              'button',
	              { onClick: actions.close, className: 'btn btn-danger btn-circle', type: 'button' },
	              React.createElement('i', { className: 'fa fa-times' })
	            )
	          )
	        )
	      ),
	      React.createElement(
	        'div',
	        null,
	        React.createElement(
	          'div',
	          { className: 'btn-group' },
	          React.createElement(
	            'span',
	            { className: 'btn btn-xs btn-primary' },
	            '128.0.0.1:8888'
	          ),
	          React.createElement(
	            'div',
	            { className: 'btn-group' },
	            React.createElement(
	              'button',
	              { 'data-toggle': 'dropdown', className: 'btn btn-default btn-xs dropdown-toggle', 'aria-expanded': 'true' },
	              React.createElement('span', { className: 'caret' })
	            ),
	            React.createElement(
	              'ul',
	              { className: 'dropdown-menu' },
	              React.createElement(
	                'li',
	                null,
	                React.createElement(
	                  'a',
	                  { href: '#', target: '_blank' },
	                  'Logs'
	                )
	              ),
	              React.createElement(
	                'li',
	                null,
	                React.createElement(
	                  'a',
	                  { href: '#', target: '_blank' },
	                  'Logs'
	                )
	              )
	            )
	          )
	        )
	      ),
	      isConnected ? React.createElement(EventStreamer, { token: token, sid: settings.sid }) : null,
	      React.createElement(TerminalBox, { settings: settings, token: token, onOpen: actions.connected })
	    );
	  }
	});
	
	var TerminalBox = React.createClass({
	  displayName: 'TerminalBox',
	
	  renderTerminal: function renderTerminal() {
	    var _this = this;
	
	    var isNew;
	    var parent = document.getElementById("terminal-box");
	
	    var _props = this.props;
	    var settings = _props.settings;
	    var token = _props.token;
	    var sid = _props.sid;
	
	    //settings.sid = 5555;
	    settings.term = {
	      h: 120,
	      w: 100
	    };
	
	    var connectionStr = cfg.api.getSessionConnStr(token, settings);
	
	    this.term = new Terminal({
	      cols: 180,
	      rows: 50,
	      useStyle: true,
	      screenKeys: true,
	      cursorBlink: false
	    });
	
	    this.term.open(parent);
	    this.socket = new WebSocket(connectionStr, "proto");
	    this.term.write('\x1b[94mconnecting to "pod"\x1b[m\r\n');
	
	    this.socket.onopen = function () {
	      _this.props.onOpen();
	      _this.term.on('data', function (data) {
	        _this.socket.send(data);
	      });
	
	      _this.socket.onmessage = function (e) {
	        _this.term.write(e.data);
	      };
	
	      _this.socket.onclose = function () {
	        _this.term.write('\x1b[31mdisconnected\x1b[m\r\n');
	      };
	    };
	  },
	
	  componentDidMount: function componentDidMount() {
	    this.renderTerminal();
	  },
	
	  componentWillUnmount: function componentWillUnmount() {
	    this.socket.close();
	    this.term.destroy();
	  },
	
	  shouldComponentUpdate: function shouldComponentUpdate() {
	    return false;
	  },
	
	  componentWillReceiveProps: function componentWillReceiveProps(props) {},
	
	  render: function render() {
	    return React.createElement('div', { className: 'grv-wiz-terminal', id: 'terminal-box' });
	  }
	});
	
	exports['default'] = TerminalHost;
	exports.TerminalBox = TerminalBox;
	exports.TerminalHost = TerminalHost;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "terminalHost.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 198 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	var render = __webpack_require__(108).render;
	
	var _require = __webpack_require__(43);
	
	var Router = _require.Router;
	var Route = _require.Route;
	var Redirect = _require.Redirect;
	var IndexRoute = _require.IndexRoute;
	var browserHistory = _require.browserHistory;
	
	var _require2 = __webpack_require__(190);
	
	var App = _require2.App;
	var Login = _require2.Login;
	var Nodes = _require2.Nodes;
	var Sessions = _require2.Sessions;
	var NewUser = _require2.NewUser;
	
	var _require3 = __webpack_require__(94);
	
	var ensureUser = _require3.ensureUser;
	
	var auth = __webpack_require__(83);
	var session = __webpack_require__(36);
	var cfg = __webpack_require__(14);
	
	__webpack_require__(152);
	
	// init session
	session.init();
	
	function handleLogout(nextState, replace, cb) {
	  auth.logout();
	  // going back will hit requireAuth handler which will redirect it to the login page
	  session.getHistory().goBack();
	}
	
	render(React.createElement(
	  Router,
	  { history: session.getHistory() },
	  React.createElement(Route, { path: cfg.routes.login, component: Login }),
	  React.createElement(Route, { path: cfg.routes.logout, onEnter: handleLogout }),
	  React.createElement(Route, { path: cfg.routes.newUser, component: NewUser }),
	  React.createElement(
	    Route,
	    { path: cfg.routes.app, component: App, onEnter: ensureUser },
	    React.createElement(IndexRoute, { component: Nodes }),
	    React.createElement(Route, { path: cfg.routes.nodes, component: Nodes }),
	    React.createElement(Route, { path: cfg.routes.sessions, component: Sessions })
	  )
	), document.getElementById("app"));
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "index.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ }
]);
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJzb3VyY2VzIjpbIndlYnBhY2s6Ly8vLi9zcmMvYXBwL3JlYWN0b3IuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb25maWcuanMiLCJ3ZWJwYWNrOi8vLy4vfi9rZXltaXJyb3IvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9zZXNzaW9uLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvc2VydmljZXMvYXBpLmpzIiwid2VicGFjazovLy9leHRlcm5hbCBcImpRdWVyeVwiIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2F1dGguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGlvblR5cGVzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3RpdmVUZXJtU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2ludml0ZVN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9ub2RlU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvYWN0aW9uVHlwZXMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvY29uc3RhbnRzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9zZXNzaW9ucy9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvdXNlci9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvdXNlci9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy91c2VyL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvdXNlclN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9nb29nbGVBdXRoLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbW1vbi9wYXR0ZXJuVXRpbHMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9nZXR0ZXJzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvcmVzdEFwaVN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9zZXNzaW9ucy9nZXR0ZXJzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9zZXNzaW9ucy9zZXNzaW9uU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC91dGlscy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvYXBwLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvZXZlbnRTdHJlYW1lci5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2luZGV4LmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvbG9naW4uanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9uYXZMZWZ0QmFyLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvbmV3VXNlci5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL25vZGVzL21haW4uanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9zZXNzaW9ucy9tYWluLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvdGFibGUuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy90ZXJtaW5hbEhvc3QuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvaW5kZXguanN4Il0sIm5hbWVzIjpbXSwibWFwcGluZ3MiOiI7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7c0NBQXdCLEVBQVk7O0FBRXBDLEtBQU0sT0FBTyxHQUFHLHVCQUFZO0FBQzFCLFFBQUssRUFBRSxJQUFJO0VBQ1osQ0FBQzs7QUFFRixPQUFNLENBQUMsT0FBTyxHQUFHLE9BQU8sQ0FBQzs7c0JBRVYsT0FBTzs7Ozs7Ozs7Ozs7OztnQkNSQSxtQkFBTyxDQUFDLEdBQXlCLENBQUM7O0tBQW5ELGFBQWEsWUFBYixhQUFhOztBQUVsQixLQUFJLEdBQUcsR0FBRzs7QUFFUixVQUFPLEVBQUUsTUFBTSxDQUFDLFFBQVEsQ0FBQyxNQUFNOztBQUUvQixNQUFHLEVBQUU7QUFDSCxtQkFB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QU8sQ0FBQyxJQUFJLEVBQUUsQ0FBQzs7QUFFZixVQUFTLFlBQVksQ0FBQyxTQUFTLEVBQUUsT0FBTyxFQUFFLEVBQUUsRUFBQztBQUMzQyxPQUFJLENBQUMsTUFBTSxFQUFFLENBQUM7O0FBRWQsVUFBTyxDQUFDLFVBQVUsRUFBRSxDQUFDLE1BQU0sRUFBRSxDQUFDO0VBQy9COztBQUVELE9BQU0sQ0FDSjtBQUFDLFNBQU07S0FBQyxPQUFPLEVBQUUsT0FBTyxDQUFDLFVBQVUsRUFBRztHQUNwQyxvQkFBQyxLQUFLLElBQUMsSUFBSSxFQUFFLEdBQUcsQ0FBQyxNQUFNLENBQUMsS0FBTSxFQUFDLFNBQVMsRUFBRSxLQUFNLEdBQUU7R0FDbEQsb0JBQUMsS0FBSyxJQUFDLElBQUksRUFBRSxHQUFHLENBQUMsTUFBTSxDQUFDLE1BQU8sRUFBQyxPQUFPLEVBQUUsWUFBYSxHQUFFO0dBQ3hELG9CQUFDLEtBQUssSUFBQyxJQUFJLEVBQUUsR0FBRyxDQUFDLE1BQU0sQ0FBQyxPQUFRLEVBQUMsU0FBUyxFQUFFLE9BQVEsR0FBRTtHQUN0RDtBQUFDLFVBQUs7T0FBQyxJQUFJLEVBQUUsR0FBRyxDQUFDLE1BQU0sQ0FBQyxHQUFJLEVBQUMsU0FBUyxFQUFFLEdBQUksRUFBQyxPQUFPLEVBQUUsVUFBVztLQUMvRCxvQkFBQyxVQUFVLElBQUMsU0FBUyxFQUFFLEtBQU0sR0FBRTtLQUMvQixvQkFBQyxLQUFLLElBQUMsSUFBSSxFQUFFLEdBQUcsQ0FBQyxNQUFNLENBQUMsS0FBTSxFQUFDLFNBQVMsRUFBRSxLQUFNLEdBQUU7S0FDbEQsb0JBQUMsS0FBSyxJQUFDLElBQUksRUFBRSxHQUFHLENBQUMsTUFBTSxDQUFDLFFBQVMsRUFBQyxTQUFTLEVBQUUsUUFBUyxHQUFFO0lBQ2xEO0VBQ0QsRUFDUixRQUFRLENBQUMsY0FBYyxDQUFDLEtBQUssQ0FBQyxDQUFDLEMiLCJmaWxlIjoiYXBwLmpzIiwic291cmNlc0NvbnRlbnQiOlsiaW1wb3J0IHsgUmVhY3RvciB9IGZyb20gJ251Y2xlYXItanMnXG5cbmNvbnN0IHJlYWN0b3IgPSBuZXcgUmVhY3Rvcih7XG4gIGRlYnVnOiB0cnVlXG59KVxuXG53aW5kb3cucmVhY3RvciA9IHJlYWN0b3I7XG5cbmV4cG9ydCBkZWZhdWx0IHJlYWN0b3JcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9yZWFjdG9yLmpzXG4gKiovIiwibGV0IHtmb3JtYXRQYXR0ZXJufSA9IHJlcXVpcmUoJ2FwcC9jb21tb24vcGF0dGVyblV0aWxzJyk7XG5cbmxldCBjZmcgPSB7XG5cbiAgYmFzZVVybDogd2luZG93LmxvY2F0aW9uLm9yaWdpbixcblxuICBhcGk6IHtcbiAgICByZW5ld1Rva2VuUGF0aDonL3YxL3dlYmFwaS9zZXNzaW9ucy9yZW5ldycsXG4gICAgbm9kZXNQYXRoOiAnL3YxL3dlYmFwaS9zaXRlcy8tY3VycmVudC0vbm9kZXMnLFxuICAgIHNlc3Npb25QYXRoOiAnL3YxL3dlYmFwaS9zZXNzaW9ucycsXG4gICAgaW52aXRlUGF0aDogJy92MS93ZWJhcGkvdXNlcnMvaW52aXRlcy86aW52aXRlVG9rZW4nLFxuICAgIGNyZWF0ZVVzZXJQYXRoOiAnL3YxL3dlYmFwaS91c2VycycsXG4gICAgZ2V0SW52aXRlVXJsOiAoaW52aXRlVG9rZW4pID0+IHtcbiAgICAgIHJldHVybiBmb3JtYXRQYXR0ZXJuKGNmZy5hcGkuaW52aXRlUGF0aCwge2ludml0ZVRva2VufSk7XG4gICAgfSxcblxuICAgIGdldEV2ZW50U3RyZWFtZXJDb25uU3RyOiAodG9rZW4sIHNpZCkgPT4ge1xuICAgICAgdmFyIGhvc3RuYW1lID0gZ2V0V3NIb3N0TmFtZSgpO1xuICAgICAgcmV0dXJuIGAke2hvc3RuYW1lfS92MS93ZWJhcGkvc2l0ZXMvLWN1cnJlbnQtL3Nlc3Npb25zLyR7c2lkfS9ldmVudHMvc3RyZWFtP2FjY2Vzc190b2tlbj0ke3Rva2VufWA7XG4gICAgfSxcblxuICAgIGdldFNlc3Npb25Db25uU3RyOiAodG9rZW4sIHBhcmFtcykgPT4ge1xuICAgICAgdmFyIGpzb24gPSBKU09OLnN0cmluZ2lmeShwYXJhbXMpO1xuICAgICAgdmFyIGpzb25FbmNvZGVkID0gd2luZG93LmVuY29kZVVSSShqc29uKTtcbiAgICAgIHZhciBob3N0bmFtZSA9IGdldFdzSG9zdE5hbWUoKTtcbiAgICAgIHJldHVybiBgJHtob3N0bmFtZX0vdjEvd2ViYXBpL3NpdGVzLy1jdXJyZW50LS9jb25uZWN0P2FjY2Vzc190b2tlbj0ke3Rva2VufSZwYXJhbXM9JHtqc29uRW5jb2RlZH1gO1xuICAgIH1cbiAgfSxcblxuICByb3V0ZXM6IHtcbiAgICBhcHA6ICcvd2ViJyxcbiAgICBsb2dvdXQ6ICcvd2ViL2xvZ291dCcsXG4gICAgbG9naW46ICcvd2ViL2xvZ2luJyxcbiAgICBub2RlczogJy93ZWIvbm9kZXMnLFxuICAgIG5ld1VzZXI6ICcvd2ViL25ld3VzZXIvOmludml0ZVRva2VuJyxcbiAgICBzZXNzaW9uczogJy93ZWIvc2Vzc2lvbnMnXG4gIH1cblxufVxuXG5leHBvcnQgZGVmYXVsdCBjZmc7XG5cbmZ1bmN0aW9uIGdldFdzSG9zdE5hbWUoKXtcbiAgdmFyIHByZWZpeCA9IGxvY2F0aW9uLnByb3RvY29sID09IFwiaHR0cHM6XCI/XCJ3c3M6Ly9cIjpcIndzOi8vXCI7XG4gIHZhciBob3N0cG9ydCA9IGxvY2F0aW9uLmhvc3RuYW1lKyhsb2NhdGlvbi5wb3J0ID8gJzonK2xvY2F0aW9uLnBvcnQ6ICcnKTtcbiAgcmV0dXJuIGAke3ByZWZpeH0ke2hvc3Rwb3J0fWA7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29uZmlnLmpzXG4gKiovIiwiLyoqXG4gKiBDb3B5cmlnaHQgMjAxMy0yMDE0IEZhY2Vib29rLCBJbmMuXG4gKlxuICogTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbiAqIHlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbiAqIFlvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuICpcbiAqIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuICpcbiAqIFVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbiAqIGRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbiAqIFdJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuICogU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxuICogbGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4gKlxuICovXG5cblwidXNlIHN0cmljdFwiO1xuXG4vKipcbiAqIENvbnN0cnVjdHMgYW4gZW51bWVyYXRpb24gd2l0aCBrZXlzIGVxdWFsIHRvIHRoZWlyIHZhbHVlLlxuICpcbiAqIEZvciBleGFtcGxlOlxuICpcbiAqICAgdmFyIENPTE9SUyA9IGtleU1pcnJvcih7Ymx1ZTogbnVsbCwgcmVkOiBudWxsfSk7XG4gKiAgIHZhciBteUNvbG9yID0gQ09MT1JTLmJsdWU7XG4gKiAgIHZhciBpc0NvbG9yVmFsaWQgPSAhIUNPTE9SU1tteUNvbG9yXTtcbiAqXG4gKiBUaGUgbGFzdCBsaW5lIGNvdWxkIG5vdCBiZSBwZXJmb3JtZWQgaWYgdGhlIHZhbHVlcyBvZiB0aGUgZ2VuZXJhdGVkIGVudW0gd2VyZVxuICogbm90IGVxdWFsIHRvIHRoZWlyIGtleXMuXG4gKlxuICogICBJbnB1dDogIHtrZXkxOiB2YWwxLCBrZXkyOiB2YWwyfVxuICogICBPdXRwdXQ6IHtrZXkxOiBrZXkxLCBrZXkyOiBrZXkyfVxuICpcbiAqIEBwYXJhbSB7b2JqZWN0fSBvYmpcbiAqIEByZXR1cm4ge29iamVjdH1cbiAqL1xudmFyIGtleU1pcnJvciA9IGZ1bmN0aW9uKG9iaikge1xuICB2YXIgcmV0ID0ge307XG4gIHZhciBrZXk7XG4gIGlmICghKG9iaiBpbnN0YW5jZW9mIE9iamVjdCAmJiAhQXJyYXkuaXNBcnJheShvYmopKSkge1xuICAgIHRocm93IG5ldyBFcnJvcigna2V5TWlycm9yKC4uLik6IEFyZ3VtZW50IG11c3QgYmUgYW4gb2JqZWN0LicpO1xuICB9XG4gIGZvciAoa2V5IGluIG9iaikge1xuICAgIGlmICghb2JqLmhhc093blByb3BlcnR5KGtleSkpIHtcbiAgICAgIGNvbnRpbnVlO1xuICAgIH1cbiAgICByZXRba2V5XSA9IGtleTtcbiAgfVxuICByZXR1cm4gcmV0O1xufTtcblxubW9kdWxlLmV4cG9ydHMgPSBrZXlNaXJyb3I7XG5cblxuXG4vKioqKioqKioqKioqKioqKipcbiAqKiBXRUJQQUNLIEZPT1RFUlxuICoqIC4vfi9rZXltaXJyb3IvaW5kZXguanNcbiAqKiBtb2R1bGUgaWQgPSAyNVxuICoqIG1vZHVsZSBjaHVua3MgPSAxXG4gKiovIiwidmFyIHsgYnJvd3Nlckhpc3RvcnkgfSA9IHJlcXVpcmUoJ3JlYWN0LXJvdXRlcicpO1xuXG5jb25zdCBBVVRIX0tFWV9EQVRBID0gJ2F1dGhEYXRhJztcblxudmFyIF9oaXN0b3J5ID0gbnVsbDtcblxudmFyIHNlc3Npb24gPSB7XG5cbiAgaW5pdChoaXN0b3J5PWJyb3dzZXJIaXN0b3J5KXtcbiAgICBfaGlzdG9yeSA9IGhpc3Rvcnk7XG4gIH0sXG5cbiAgZ2V0SGlzdG9yeSgpe1xuICAgIHJldHVybiBfaGlzdG9yeTtcbiAgfSxcblxuICBzZXRVc2VyRGF0YSh1c2VyRGF0YSl7XG4gICAgbG9jYWxTdG9yYWdlLnNldEl0ZW0oQVVUSF9LRVlfREFUQSwgSlNPTi5zdHJpbmdpZnkodXNlckRhdGEpKTtcbiAgfSxcblxuICBnZXRVc2VyRGF0YSgpe1xuICAgIHZhciBpdGVtID0gbG9jYWxTdG9yYWdlLmdldEl0ZW0oQVVUSF9LRVlfREFUQSk7XG4gICAgaWYoaXRlbSl7XG4gICAgICByZXR1cm4gSlNPTi5wYXJzZShpdGVtKTtcbiAgICB9XG5cbiAgICByZXR1cm4ge307XG4gIH0sXG5cbiAgY2xlYXIoKXtcbiAgICBsb2NhbFN0b3JhZ2UuY2xlYXIoKVxuICB9XG5cbn1cblxubW9kdWxlLmV4cG9ydHMgPSBzZXNzaW9uO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL3Nlc3Npb24uanNcbiAqKi8iLCJ2YXIgJCA9IHJlcXVpcmUoXCJqUXVlcnlcIik7XG52YXIgc2Vzc2lvbiA9IHJlcXVpcmUoJ2FwcC9zZXNzaW9uJyk7XG5cbmNvbnN0IGFwaSA9IHtcblxuICBwb3N0KHBhdGgsIGRhdGEsIHdpdGhUb2tlbil7XG4gICAgcmV0dXJuIGFwaS5hamF4KHt1cmw6IHBhdGgsIGRhdGE6IEpTT04uc3RyaW5naWZ5KGRhdGEpLCB0eXBlOiAnUE9TVCd9LCB3aXRoVG9rZW4pO1xuICB9LFxuXG4gIGdldChwYXRoKXtcbiAgICByZXR1cm4gYXBpLmFqYXgoe3VybDogcGF0aH0pO1xuICB9LFxuXG4gIGFqYXgoY2ZnLCB3aXRoVG9rZW4gPSB0cnVlKXtcbiAgICB2YXIgZGVmYXVsdENmZyA9IHtcbiAgICAgIHR5cGU6IFwiR0VUXCIsXG4gICAgICBkYXRhVHlwZTogXCJqc29uXCIsXG4gICAgICBiZWZvcmVTZW5kOiBmdW5jdGlvbih4aHIpIHtcbiAgICAgICAgaWYod2l0aFRva2VuKXtcbiAgICAgICAgICB2YXIgeyB0b2tlbiB9ID0gc2Vzc2lvbi5nZXRVc2VyRGF0YSgpO1xuICAgICAgICAgIHhoci5zZXRSZXF1ZXN0SGVhZGVyKCdBdXRob3JpemF0aW9uJywnQmVhcmVyICcgKyB0b2tlbik7XG4gICAgICAgIH1cbiAgICAgICB9XG4gICAgfVxuXG4gICAgcmV0dXJuICQuYWpheCgkLmV4dGVuZCh7fSwgZGVmYXVsdENmZywgY2ZnKSk7XG4gIH1cbn1cblxubW9kdWxlLmV4cG9ydHMgPSBhcGk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvc2VydmljZXMvYXBpLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMgPSBqUXVlcnk7XG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiBleHRlcm5hbCBcImpRdWVyeVwiXG4gKiogbW9kdWxlIGlkID0gNTJcbiAqKiBtb2R1bGUgY2h1bmtzID0gMVxuICoqLyIsImltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX05PREVTX1JFQ0VJVkU6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9hY3Rpb25UeXBlcy5qc1xuICoqLyIsInZhciBhcGkgPSByZXF1aXJlKCcuL3NlcnZpY2VzL2FwaScpO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCcuL3Nlc3Npb24nKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG52YXIgJCA9IHJlcXVpcmUoJ2pRdWVyeScpO1xuXG5jb25zdCByZWZyZXNoUmF0ZSA9IDYwMDAwICogNTsgLy8gMSBtaW5cblxudmFyIHJlZnJlc2hUb2tlblRpbWVySWQgPSBudWxsO1xuXG52YXIgYXV0aCA9IHtcblxuICBzaWduVXAobmFtZSwgcGFzc3dvcmQsIHRva2VuLCBpbnZpdGVUb2tlbil7XG4gICAgdmFyIGRhdGEgPSB7dXNlcjogbmFtZSwgcGFzczogcGFzc3dvcmQsIHNlY29uZF9mYWN0b3JfdG9rZW46IHRva2VuLCBpbnZpdGVfdG9rZW46IGludml0ZVRva2VufTtcbiAgICByZXR1cm4gYXBpLnBvc3QoY2ZnLmFwaS5jcmVhdGVVc2VyUGF0aCwgZGF0YSlcbiAgICAgIC50aGVuKCh1c2VyKT0+e1xuICAgICAgICBzZXNzaW9uLnNldFVzZXJEYXRhKHVzZXIpO1xuICAgICAgICBhdXRoLl9zdGFydFRva2VuUmVmcmVzaGVyKCk7XG4gICAgICAgIHJldHVybiB1c2VyO1xuICAgICAgfSk7XG4gIH0sXG5cbiAgbG9naW4obmFtZSwgcGFzc3dvcmQsIHRva2VuKXtcbiAgICBhdXRoLl9zdG9wVG9rZW5SZWZyZXNoZXIoKTtcbiAgICByZXR1cm4gYXV0aC5fbG9naW4obmFtZSwgcGFzc3dvcmQsIHRva2VuKS5kb25lKGF1dGguX3N0YXJ0VG9rZW5SZWZyZXNoZXIpO1xuICB9LFxuXG4gIGVuc3VyZVVzZXIoKXsgICAgXG4gICAgdmFyIHVzZXJEYXRhID0gc2Vzc2lvbi5nZXRVc2VyRGF0YSgpO1xuICAgIGlmKHVzZXJEYXRhLnRva2VuKXtcbiAgICAgIC8vIHJlZnJlc2ggdGltZXIgd2lsbCBub3QgYmUgc2V0IGluIGNhc2Ugb2YgYnJvd3NlciByZWZyZXNoIGV2ZW50XG4gICAgICBpZihhdXRoLl9nZXRSZWZyZXNoVG9rZW5UaW1lcklkKCkgPT09IG51bGwpe1xuICAgICAgICByZXR1cm4gYXV0aC5fcmVmcmVzaFRva2VuKCkuZG9uZShhdXRoLl9zdGFydFRva2VuUmVmcmVzaGVyKTtcbiAgICAgIH1cblxuICAgICAgcmV0dXJuICQuRGVmZXJyZWQoKS5yZXNvbHZlKHVzZXJEYXRhKTtcbiAgICB9XG5cbiAgICByZXR1cm4gJC5EZWZlcnJlZCgpLnJlamVjdCgpO1xuICB9LFxuXG4gIGxvZ291dCgpe1xuICAgIGF1dGguX3N0b3BUb2tlblJlZnJlc2hlcigpO1xuICAgIHJldHVybiBzZXNzaW9uLmNsZWFyKCk7XG4gIH0sXG5cbiAgX3N0YXJ0VG9rZW5SZWZyZXNoZXIoKXtcbiAgICByZWZyZXNoVG9rZW5UaW1lcklkID0gc2V0SW50ZXJ2YWwoYXV0aC5fcmVmcmVzaFRva2VuLCByZWZyZXNoUmF0ZSk7XG4gIH0sXG5cbiAgX3N0b3BUb2tlblJlZnJlc2hlcigpe1xuICAgIGNsZWFySW50ZXJ2YWwocmVmcmVzaFRva2VuVGltZXJJZCk7XG4gICAgcmVmcmVzaFRva2VuVGltZXJJZCA9IG51bGw7XG4gIH0sXG5cbiAgX2dldFJlZnJlc2hUb2tlblRpbWVySWQoKXtcbiAgICByZXR1cm4gcmVmcmVzaFRva2VuVGltZXJJZDtcbiAgfSxcblxuICBfcmVmcmVzaFRva2VuKCl7XG4gICAgcmV0dXJuIGFwaS5wb3N0KGNmZy5hcGkucmVuZXdUb2tlblBhdGgpLnRoZW4oZGF0YT0+e1xuICAgICAgc2Vzc2lvbi5zZXRVc2VyRGF0YShkYXRhKTtcbiAgICAgIHJldHVybiBkYXRhO1xuICAgIH0pLmZhaWwoKCk9PntcbiAgICAgIGF1dGgubG9nb3V0KCk7XG4gICAgICB3aW5kb3cubG9jYXRpb24ucmVsb2FkKCk7XG4gICAgfSk7XG4gIH0sXG5cbiAgX2xvZ2luKG5hbWUsIHBhc3N3b3JkLCB0b2tlbil7XG4gICAgdmFyIGRhdGEgPSB7XG4gICAgICB1c2VyOiBuYW1lLFxuICAgICAgcGFzczogcGFzc3dvcmQsXG4gICAgICBzZWNvbmRfZmFjdG9yX3Rva2VuOiB0b2tlblxuICAgIH07XG5cbiAgICByZXR1cm4gYXBpLnBvc3QoY2ZnLmFwaS5zZXNzaW9uUGF0aCwgZGF0YSwgZmFsc2UpLnRoZW4oZGF0YT0+e1xuICAgICAgc2Vzc2lvbi5zZXRVc2VyRGF0YShkYXRhKTtcbiAgICAgIHJldHVybiBkYXRhO1xuICAgIH0pO1xuICB9XG59XG5cbm1vZHVsZS5leHBvcnRzID0gYXV0aDtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9hdXRoLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfVEVSTV9PUEVOOiBudWxsLFxuICBUTFBUX1RFUk1fQ0xPU0U6IG51bGwsXG4gIFRMUFRfVEVSTV9DT05ORUNURUQ6IG51bGwsXG4gIFRMUFRfVEVSTV9SRUNFSVZFX1BBUlRJRVM6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3Rpb25UeXBlcy5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7dXVpZH0gPSByZXF1aXJlKCdhcHAvdXRpbHMnKTtcbnZhciB7IFRMUFRfVEVSTV9PUEVOLCBUTFBUX1RFUk1fQ0xPU0UsIFRMUFRfVEVSTV9DT05ORUNURUQsIFRMUFRfVEVSTV9SRUNFSVZFX1BBUlRJRVMgfSAgPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IHtcblxuICBjbG9zZSgpe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9URVJNX0NMT1NFKTtcbiAgfSxcblxuICBjb25uZWN0ZWQoKXtcbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfVEVSTV9DT05ORUNURUQpO1xuICB9LFxuXG4gIHJlY2VpdmVQYXJ0aWVzKGpzb24pe1xuICAgIHZhciBwYXJ0aWVzID0ganNvbi5tYXAoaXRlbT0+e1xuICAgICAgcmV0dXJuIHtcbiAgICAgICAgdXNlcjogaXRlbS51c2VyLFxuICAgICAgICBsYXN0QWN0aXZlOiBuZXcgRGF0ZShpdGVtLmxhc3RfYWN0aXZlKVxuICAgICAgfVxuICAgIH0pXG5cbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfVEVSTV9SRUNFSVZFX1BBUlRJRVMsIHBhcnRpZXMpO1xuICB9LFxuXG4gIG9wZW4oYWRkciwgbG9naW4sIHNpZD11dWlkKCkpe1xuICAgIC8qXG4gICAgKiAgIHtcbiAgICAqICAgXCJhZGRyXCI6IFwiMTI3LjAuMC4xOjUwMDBcIixcbiAgICAqICAgXCJsb2dpblwiOiBcImFkbWluXCIsXG4gICAgKiAgIFwidGVybVwiOiB7XCJoXCI6IDEyMCwgXCJ3XCI6IDEwMH0sXG4gICAgKiAgIFwic2lkXCI6IFwiMTIzXCJcbiAgICAqICB9XG4gICAgKi9cbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfVEVSTV9PUEVOLCB7YWRkciwgbG9naW4sIHNpZH0gKTtcbiAgfVxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvYWN0aW9ucy5qc1xuICoqLyIsInZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyIHsgVExQVF9URVJNX09QRU4sIFRMUFRfVEVSTV9DTE9TRSwgVExQVF9URVJNX0NPTk5FQ1RFRCwgVExQVF9URVJNX1JFQ0VJVkVfUEFSVElFUyB9ICA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKG51bGwpO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX1RFUk1fQ09OTkVDVEVELCBjb25uZWN0ZWQpO1xuICAgIHRoaXMub24oVExQVF9URVJNX09QRU4sIHNldEFjdGl2ZVRlcm1pbmFsKTtcbiAgICB0aGlzLm9uKFRMUFRfVEVSTV9DTE9TRSwgY2xvc2UpO1xuICAgIHRoaXMub24oVExQVF9URVJNX1JFQ0VJVkVfUEFSVElFUywgcmVjZWl2ZVBhcnRpZXMpO1xuICB9XG5cbn0pXG5cbmZ1bmN0aW9uIGNsb3NlKCl7XG4gIHJldHVybiB0b0ltbXV0YWJsZShudWxsKTtcbn1cblxuZnVuY3Rpb24gcmVjZWl2ZVBhcnRpZXMoc3RhdGUsIHBhcnRpZXMpe1xuICByZXR1cm4gc3RhdGUuc2V0KCdwYXJ0aWVzJywgdG9JbW11dGFibGUocGFydGllcykpO1xufVxuXG5mdW5jdGlvbiBzZXRBY3RpdmVUZXJtaW5hbChzdGF0ZSwgc2V0dGluZ3Mpe1xuICByZXR1cm4gdG9JbW11dGFibGUoe1xuICAgICAgcGFydGllczogW10sXG4gICAgICBpc0Nvbm5lY3Rpbmc6IHRydWUsXG4gICAgICAuLi5zZXR0aW5nc1xuICB9KTtcbn1cblxuZnVuY3Rpb24gY29ubmVjdGVkKHN0YXRlKXtcbiAgcmV0dXJuIHN0YXRlLnNldCgnaXNDb25uZWN0ZWQnLCB0cnVlKVxuICAgICAgICAgICAgICAuc2V0KCdpc0Nvbm5lY3RpbmcnLCBmYWxzZSk7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3RpdmVUZXJtU3RvcmUuanNcbiAqKi8iLCJpbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVExQVF9SRUNFSVZFX1VTRVJfSU5WSVRFOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2FjdGlvblR5cGVzLmpzXG4gKiovIiwidmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG52YXIgIHsgVExQVF9SRUNFSVZFX1VTRVJfSU5WSVRFIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IFN0b3JlKHtcbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB0b0ltbXV0YWJsZShudWxsKTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9SRUNFSVZFX1VTRVJfSU5WSVRFLCByZWNlaXZlSW52aXRlKVxuICB9XG59KVxuXG5mdW5jdGlvbiByZWNlaXZlSW52aXRlKHN0YXRlLCBpbnZpdGUpe1xuICByZXR1cm4gdG9JbW11dGFibGUoaW52aXRlKTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9pbnZpdGVTdG9yZS5qc1xuICoqLyIsInZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyICB7IFRMUFRfTk9ERVNfUkVDRUlWRSB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gdG9JbW11dGFibGUoW10pO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX05PREVTX1JFQ0VJVkUsIHJlY2VpdmVOb2RlcylcbiAgfVxufSlcblxuZnVuY3Rpb24gcmVjZWl2ZU5vZGVzKHN0YXRlLCBub2RlQXJyYXkpe1xuICByZXR1cm4gdG9JbW11dGFibGUobm9kZUFycmF5KTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL25vZGVTdG9yZS5qc1xuICoqLyIsImltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX1JFU1RfQVBJX1NUQVJUOiBudWxsLFxuICBUTFBUX1JFU1RfQVBJX1NVQ0NFU1M6IG51bGwsXG4gIFRMUFRfUkVTVF9BUElfRkFJTDogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvYWN0aW9uVHlwZXMuanNcbiAqKi8iLCJpbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVFJZSU5HX1RPX1NJR05fVVA6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2NvbnN0YW50cy5qc1xuICoqLyIsImltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX1NFU1NJTlNfUkVDRUlWRTogbnVsbCxcbiAgVExQVF9TRVNTSU5TX1VQREFURTogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2FjdGlvblR5cGVzLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfUkVDRUlWRV9VU0VSOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvdXNlci9hY3Rpb25UeXBlcy5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7IFRMUFRfUkVDRUlWRV9VU0VSIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG52YXIgeyBUUllJTkdfVE9fU0lHTl9VUH0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9yZXN0QXBpL2NvbnN0YW50cycpO1xudmFyIHJlc3RBcGlBY3Rpb25zID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvcmVzdEFwaS9hY3Rpb25zJyk7XG52YXIgYXV0aCA9IHJlcXVpcmUoJ2FwcC9hdXRoJyk7XG52YXIgc2Vzc2lvbiA9IHJlcXVpcmUoJ2FwcC9zZXNzaW9uJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xuXG5leHBvcnQgZGVmYXVsdCB7XG5cbiAgZW5zdXJlVXNlcihuZXh0U3RhdGUsIHJlcGxhY2UsIGNiKXtcbiAgICBhdXRoLmVuc3VyZVVzZXIoKVxuICAgICAgLmRvbmUoKHVzZXJEYXRhKT0+IHtcbiAgICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFQ0VJVkVfVVNFUiwgdXNlckRhdGEudXNlcik7XG4gICAgICAgIGNiKCk7XG4gICAgICB9KVxuICAgICAgLmZhaWwoKCk9PntcbiAgICAgICAgcmVwbGFjZSh7cmVkaXJlY3RUbzogbmV4dFN0YXRlLmxvY2F0aW9uLnBhdGhuYW1lIH0sIGNmZy5yb3V0ZXMubG9naW4pO1xuICAgICAgICBjYigpO1xuICAgICAgfSk7XG4gIH0sXG5cbiAgc2lnblVwKHtuYW1lLCBwc3csIHRva2VuLCBpbnZpdGVUb2tlbn0pe1xuICAgIHJlc3RBcGlBY3Rpb25zLnN0YXJ0KFRSWUlOR19UT19TSUdOX1VQKTtcbiAgICBhdXRoLnNpZ25VcChuYW1lLCBwc3csIHRva2VuLCBpbnZpdGVUb2tlbilcbiAgICAgIC5kb25lKChzZXNzaW9uRGF0YSk9PntcbiAgICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFQ0VJVkVfVVNFUiwgc2Vzc2lvbkRhdGEudXNlcik7XG4gICAgICAgIHJlc3RBcGlBY3Rpb25zLnN1Y2Nlc3MoVFJZSU5HX1RPX1NJR05fVVApO1xuICAgICAgICBzZXNzaW9uLmdldEhpc3RvcnkoKS5wdXNoKHtwYXRobmFtZTogY2ZnLnJvdXRlcy5hcHB9KTtcbiAgICAgIH0pXG4gICAgICAuZmFpbCgoKT0+e1xuICAgICAgICByZXN0QXBpQWN0aW9ucy5mYWlsKFRSWUlOR19UT19TSUdOX1VQLCAnZmFpbGVkIHRvIHNpbmcgdXAnKTtcbiAgICAgIH0pO1xuICB9LFxuXG4gIGxvZ2luKHt1c2VyLCBwYXNzd29yZCwgdG9rZW59LCByZWRpcmVjdCl7XG4gICAgICBhdXRoLmxvZ2luKHVzZXIsIHBhc3N3b3JkLCB0b2tlbilcbiAgICAgICAgLmRvbmUoKHNlc3Npb25EYXRhKT0+e1xuICAgICAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRUNFSVZFX1VTRVIsIHNlc3Npb25EYXRhLnVzZXIpO1xuICAgICAgICAgIHNlc3Npb24uZ2V0SGlzdG9yeSgpLnB1c2goe3BhdGhuYW1lOiByZWRpcmVjdH0pO1xuICAgICAgICB9KVxuICAgICAgICAuZmFpbCgoKT0+e1xuICAgICAgICB9KVxuICAgIH1cbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvYWN0aW9ucy5qc1xuICoqLyIsImNvbnN0IHVzZXIgPSBbIFsndGxwdF91c2VyJ10sIChjdXJyZW50VXNlcikgPT4ge1xuICAgIGlmKCFjdXJyZW50VXNlcil7XG4gICAgICByZXR1cm4gbnVsbDtcbiAgICB9XG4gICAgXG4gICAgcmV0dXJuIHtcbiAgICAgIG5hbWU6IGN1cnJlbnRVc2VyLmdldCgnbmFtZScpLFxuICAgICAgbG9naW5zOiBjdXJyZW50VXNlci5nZXQoJ2FsbG93ZWRfbG9naW5zJykudG9KUygpXG4gICAgfVxuICB9XG5dO1xuXG5leHBvcnQgZGVmYXVsdCB7XG4gIHVzZXJcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvZ2V0dGVycy5qc1xuICoqLyIsIm1vZHVsZS5leHBvcnRzLmdldHRlcnMgPSByZXF1aXJlKCcuL2dldHRlcnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGlvbnMgPSByZXF1aXJlKCcuL2FjdGlvbnMnKTtcbm1vZHVsZS5leHBvcnRzLm5vZGVTdG9yZSA9IHJlcXVpcmUoJy4vdXNlclN0b3JlJyk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy91c2VyL2luZGV4LmpzXG4gKiovIiwidmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG52YXIgIHsgVExQVF9SRUNFSVZFX1VTRVIgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKG51bGwpO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX1JFQ0VJVkVfVVNFUiwgcmVjZWl2ZVVzZXIpXG4gIH1cblxufSlcblxuZnVuY3Rpb24gcmVjZWl2ZVVzZXIoc3RhdGUsIHVzZXIpe1xuICByZXR1cm4gdG9JbW11dGFibGUodXNlcik7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy91c2VyL3VzZXJTdG9yZS5qc1xuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG5cbnZhciBHb29nbGVBdXRoSW5mbyA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgcmVuZGVyKCkge1xuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1nb29nbGUtYXV0aFwiPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1nb29nbGUtYXV0aC1pY29uXCI+PC9kaXY+XG4gICAgICAgIDxzdHJvbmc+R29vZ2xlIEF1dGhlbnRpY2F0b3I8L3N0cm9uZz5cbiAgICAgICAgPGRpdj5Eb3dubG9hZCA8YSBocmVmPVwiaHR0cHM6Ly9zdXBwb3J0Lmdvb2dsZS5jb20vYWNjb3VudHMvYW5zd2VyLzEwNjY0NDc/aGw9ZW5cIj5Hb29nbGUgQXV0aGVudGljYXRvcjwvYT4gb24geW91ciBwaG9uZSB0byBhY2Nlc3MgeW91ciB0d28gZmFjdG9yeSB0b2tlbjwvZGl2PlxuICAgICAgPC9kaXY+XG4gICAgKTtcbiAgfVxufSlcblxubW9kdWxlLmV4cG9ydHMgPSBHb29nbGVBdXRoSW5mbztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2dvb2dsZUF1dGguanN4XG4gKiovIiwiLypcbiAqICBUaGUgTUlUIExpY2Vuc2UgKE1JVClcbiAqICBDb3B5cmlnaHQgKGMpIDIwMTUgUnlhbiBGbG9yZW5jZSwgTWljaGFlbCBKYWNrc29uXG4gKiAgUGVybWlzc2lvbiBpcyBoZXJlYnkgZ3JhbnRlZCwgZnJlZSBvZiBjaGFyZ2UsIHRvIGFueSBwZXJzb24gb2J0YWluaW5nIGEgY29weSBvZiB0aGlzIHNvZnR3YXJlIGFuZCBhc3NvY2lhdGVkIGRvY3VtZW50YXRpb24gZmlsZXMgKHRoZSBcIlNvZnR3YXJlXCIpLCB0byBkZWFsIGluIHRoZSBTb2Z0d2FyZSB3aXRob3V0IHJlc3RyaWN0aW9uLCBpbmNsdWRpbmcgd2l0aG91dCBsaW1pdGF0aW9uIHRoZSByaWdodHMgdG8gdXNlLCBjb3B5LCBtb2RpZnksIG1lcmdlLCBwdWJsaXNoLCBkaXN0cmlidXRlLCBzdWJsaWNlbnNlLCBhbmQvb3Igc2VsbCBjb3BpZXMgb2YgdGhlIFNvZnR3YXJlLCBhbmQgdG8gcGVybWl0IHBlcnNvbnMgdG8gd2hvbSB0aGUgU29mdHdhcmUgaXMgZnVybmlzaGVkIHRvIGRvIHNvLCBzdWJqZWN0IHRvIHRoZSBmb2xsb3dpbmcgY29uZGl0aW9uczpcbiAqICBUaGUgYWJvdmUgY29weXJpZ2h0IG5vdGljZSBhbmQgdGhpcyBwZXJtaXNzaW9uIG5vdGljZSBzaGFsbCBiZSBpbmNsdWRlZCBpbiBhbGwgY29waWVzIG9yIHN1YnN0YW50aWFsIHBvcnRpb25zIG9mIHRoZSBTb2Z0d2FyZS5cbiAqICBUSEUgU09GVFdBUkUgSVMgUFJPVklERUQgXCJBUyBJU1wiLCBXSVRIT1VUIFdBUlJBTlRZIE9GIEFOWSBLSU5ELCBFWFBSRVNTIE9SIElNUExJRUQsIElOQ0xVRElORyBCVVQgTk9UIExJTUlURUQgVE8gVEhFIFdBUlJBTlRJRVMgT0YgTUVSQ0hBTlRBQklMSVRZLCBGSVRORVNTIEZPUiBBIFBBUlRJQ1VMQVIgUFVSUE9TRSBBTkQgTk9OSU5GUklOR0VNRU5ULiBJTiBOTyBFVkVOVCBTSEFMTCBUSEUgQVVUSE9SUyBPUiBDT1BZUklHSFQgSE9MREVSUyBCRSBMSUFCTEUgRk9SIEFOWSBDTEFJTSwgREFNQUdFUyBPUiBPVEhFUiBMSUFCSUxJVFksIFdIRVRIRVIgSU4gQU4gQUNUSU9OIE9GIENPTlRSQUNULCBUT1JUIE9SIE9USEVSV0lTRSwgQVJJU0lORyBGUk9NLCBPVVQgT0YgT1IgSU4gQ09OTkVDVElPTiBXSVRIIFRIRSBTT0ZUV0FSRSBPUiBUSEUgVVNFIE9SIE9USEVSIERFQUxJTkdTIElOIFRIRSBTT0ZUV0FSRS5cbiovXG5cbmltcG9ydCBpbnZhcmlhbnQgZnJvbSAnaW52YXJpYW50J1xuXG5mdW5jdGlvbiBlc2NhcGVSZWdFeHAoc3RyaW5nKSB7XG4gIHJldHVybiBzdHJpbmcucmVwbGFjZSgvWy4qKz9eJHt9KCl8W1xcXVxcXFxdL2csICdcXFxcJCYnKVxufVxuXG5mdW5jdGlvbiBlc2NhcGVTb3VyY2Uoc3RyaW5nKSB7XG4gIHJldHVybiBlc2NhcGVSZWdFeHAoc3RyaW5nKS5yZXBsYWNlKC9cXC8rL2csICcvKycpXG59XG5cbmZ1bmN0aW9uIF9jb21waWxlUGF0dGVybihwYXR0ZXJuKSB7XG4gIGxldCByZWdleHBTb3VyY2UgPSAnJztcbiAgY29uc3QgcGFyYW1OYW1lcyA9IFtdO1xuICBjb25zdCB0b2tlbnMgPSBbXTtcblxuICBsZXQgbWF0Y2gsIGxhc3RJbmRleCA9IDAsIG1hdGNoZXIgPSAvOihbYS16QS1aXyRdW2EtekEtWjAtOV8kXSopfFxcKlxcKnxcXCp8XFwofFxcKS9nXG4gIC8qZXNsaW50IG5vLWNvbmQtYXNzaWduOiAwKi9cbiAgd2hpbGUgKChtYXRjaCA9IG1hdGNoZXIuZXhlYyhwYXR0ZXJuKSkpIHtcbiAgICBpZiAobWF0Y2guaW5kZXggIT09IGxhc3RJbmRleCkge1xuICAgICAgdG9rZW5zLnB1c2gocGF0dGVybi5zbGljZShsYXN0SW5kZXgsIG1hdGNoLmluZGV4KSlcbiAgICAgIHJlZ2V4cFNvdXJjZSArPSBlc2NhcGVTb3VyY2UocGF0dGVybi5zbGljZShsYXN0SW5kZXgsIG1hdGNoLmluZGV4KSlcbiAgICB9XG5cbiAgICBpZiAobWF0Y2hbMV0pIHtcbiAgICAgIHJlZ2V4cFNvdXJjZSArPSAnKFteLz8jXSspJztcbiAgICAgIHBhcmFtTmFtZXMucHVzaChtYXRjaFsxXSk7XG4gICAgfSBlbHNlIGlmIChtYXRjaFswXSA9PT0gJyoqJykge1xuICAgICAgcmVnZXhwU291cmNlICs9ICcoW1xcXFxzXFxcXFNdKiknXG4gICAgICBwYXJhbU5hbWVzLnB1c2goJ3NwbGF0Jyk7XG4gICAgfSBlbHNlIGlmIChtYXRjaFswXSA9PT0gJyonKSB7XG4gICAgICByZWdleHBTb3VyY2UgKz0gJyhbXFxcXHNcXFxcU10qPyknXG4gICAgICBwYXJhbU5hbWVzLnB1c2goJ3NwbGF0Jyk7XG4gICAgfSBlbHNlIGlmIChtYXRjaFswXSA9PT0gJygnKSB7XG4gICAgICByZWdleHBTb3VyY2UgKz0gJyg/Oic7XG4gICAgfSBlbHNlIGlmIChtYXRjaFswXSA9PT0gJyknKSB7XG4gICAgICByZWdleHBTb3VyY2UgKz0gJyk/JztcbiAgICB9XG5cbiAgICB0b2tlbnMucHVzaChtYXRjaFswXSk7XG5cbiAgICBsYXN0SW5kZXggPSBtYXRjaGVyLmxhc3RJbmRleDtcbiAgfVxuXG4gIGlmIChsYXN0SW5kZXggIT09IHBhdHRlcm4ubGVuZ3RoKSB7XG4gICAgdG9rZW5zLnB1c2gocGF0dGVybi5zbGljZShsYXN0SW5kZXgsIHBhdHRlcm4ubGVuZ3RoKSlcbiAgICByZWdleHBTb3VyY2UgKz0gZXNjYXBlU291cmNlKHBhdHRlcm4uc2xpY2UobGFzdEluZGV4LCBwYXR0ZXJuLmxlbmd0aCkpXG4gIH1cblxuICByZXR1cm4ge1xuICAgIHBhdHRlcm4sXG4gICAgcmVnZXhwU291cmNlLFxuICAgIHBhcmFtTmFtZXMsXG4gICAgdG9rZW5zXG4gIH1cbn1cblxuY29uc3QgQ29tcGlsZWRQYXR0ZXJuc0NhY2hlID0ge31cblxuZXhwb3J0IGZ1bmN0aW9uIGNvbXBpbGVQYXR0ZXJuKHBhdHRlcm4pIHtcbiAgaWYgKCEocGF0dGVybiBpbiBDb21waWxlZFBhdHRlcm5zQ2FjaGUpKVxuICAgIENvbXBpbGVkUGF0dGVybnNDYWNoZVtwYXR0ZXJuXSA9IF9jb21waWxlUGF0dGVybihwYXR0ZXJuKVxuXG4gIHJldHVybiBDb21waWxlZFBhdHRlcm5zQ2FjaGVbcGF0dGVybl1cbn1cblxuLyoqXG4gKiBBdHRlbXB0cyB0byBtYXRjaCBhIHBhdHRlcm4gb24gdGhlIGdpdmVuIHBhdGhuYW1lLiBQYXR0ZXJucyBtYXkgdXNlXG4gKiB0aGUgZm9sbG93aW5nIHNwZWNpYWwgY2hhcmFjdGVyczpcbiAqXG4gKiAtIDpwYXJhbU5hbWUgICAgIE1hdGNoZXMgYSBVUkwgc2VnbWVudCB1cCB0byB0aGUgbmV4dCAvLCA/LCBvciAjLiBUaGVcbiAqICAgICAgICAgICAgICAgICAgY2FwdHVyZWQgc3RyaW5nIGlzIGNvbnNpZGVyZWQgYSBcInBhcmFtXCJcbiAqIC0gKCkgICAgICAgICAgICAgV3JhcHMgYSBzZWdtZW50IG9mIHRoZSBVUkwgdGhhdCBpcyBvcHRpb25hbFxuICogLSAqICAgICAgICAgICAgICBDb25zdW1lcyAobm9uLWdyZWVkeSkgYWxsIGNoYXJhY3RlcnMgdXAgdG8gdGhlIG5leHRcbiAqICAgICAgICAgICAgICAgICAgY2hhcmFjdGVyIGluIHRoZSBwYXR0ZXJuLCBvciB0byB0aGUgZW5kIG9mIHRoZSBVUkwgaWZcbiAqICAgICAgICAgICAgICAgICAgdGhlcmUgaXMgbm9uZVxuICogLSAqKiAgICAgICAgICAgICBDb25zdW1lcyAoZ3JlZWR5KSBhbGwgY2hhcmFjdGVycyB1cCB0byB0aGUgbmV4dCBjaGFyYWN0ZXJcbiAqICAgICAgICAgICAgICAgICAgaW4gdGhlIHBhdHRlcm4sIG9yIHRvIHRoZSBlbmQgb2YgdGhlIFVSTCBpZiB0aGVyZSBpcyBub25lXG4gKlxuICogVGhlIHJldHVybiB2YWx1ZSBpcyBhbiBvYmplY3Qgd2l0aCB0aGUgZm9sbG93aW5nIHByb3BlcnRpZXM6XG4gKlxuICogLSByZW1haW5pbmdQYXRobmFtZVxuICogLSBwYXJhbU5hbWVzXG4gKiAtIHBhcmFtVmFsdWVzXG4gKi9cbmV4cG9ydCBmdW5jdGlvbiBtYXRjaFBhdHRlcm4ocGF0dGVybiwgcGF0aG5hbWUpIHtcbiAgLy8gTWFrZSBsZWFkaW5nIHNsYXNoZXMgY29uc2lzdGVudCBiZXR3ZWVuIHBhdHRlcm4gYW5kIHBhdGhuYW1lLlxuICBpZiAocGF0dGVybi5jaGFyQXQoMCkgIT09ICcvJykge1xuICAgIHBhdHRlcm4gPSBgLyR7cGF0dGVybn1gXG4gIH1cbiAgaWYgKHBhdGhuYW1lLmNoYXJBdCgwKSAhPT0gJy8nKSB7XG4gICAgcGF0aG5hbWUgPSBgLyR7cGF0aG5hbWV9YFxuICB9XG5cbiAgbGV0IHsgcmVnZXhwU291cmNlLCBwYXJhbU5hbWVzLCB0b2tlbnMgfSA9IGNvbXBpbGVQYXR0ZXJuKHBhdHRlcm4pXG5cbiAgcmVnZXhwU291cmNlICs9ICcvKicgLy8gQ2FwdHVyZSBwYXRoIHNlcGFyYXRvcnNcblxuICAvLyBTcGVjaWFsLWNhc2UgcGF0dGVybnMgbGlrZSAnKicgZm9yIGNhdGNoLWFsbCByb3V0ZXMuXG4gIGNvbnN0IGNhcHR1cmVSZW1haW5pbmcgPSB0b2tlbnNbdG9rZW5zLmxlbmd0aCAtIDFdICE9PSAnKidcblxuICBpZiAoY2FwdHVyZVJlbWFpbmluZykge1xuICAgIC8vIFRoaXMgd2lsbCBtYXRjaCBuZXdsaW5lcyBpbiB0aGUgcmVtYWluaW5nIHBhdGguXG4gICAgcmVnZXhwU291cmNlICs9ICcoW1xcXFxzXFxcXFNdKj8pJ1xuICB9XG5cbiAgY29uc3QgbWF0Y2ggPSBwYXRobmFtZS5tYXRjaChuZXcgUmVnRXhwKCdeJyArIHJlZ2V4cFNvdXJjZSArICckJywgJ2knKSlcblxuICBsZXQgcmVtYWluaW5nUGF0aG5hbWUsIHBhcmFtVmFsdWVzXG4gIGlmIChtYXRjaCAhPSBudWxsKSB7XG4gICAgaWYgKGNhcHR1cmVSZW1haW5pbmcpIHtcbiAgICAgIHJlbWFpbmluZ1BhdGhuYW1lID0gbWF0Y2gucG9wKClcbiAgICAgIGNvbnN0IG1hdGNoZWRQYXRoID1cbiAgICAgICAgbWF0Y2hbMF0uc3Vic3RyKDAsIG1hdGNoWzBdLmxlbmd0aCAtIHJlbWFpbmluZ1BhdGhuYW1lLmxlbmd0aClcblxuICAgICAgLy8gSWYgd2UgZGlkbid0IG1hdGNoIHRoZSBlbnRpcmUgcGF0aG5hbWUsIHRoZW4gbWFrZSBzdXJlIHRoYXQgdGhlIG1hdGNoXG4gICAgICAvLyB3ZSBkaWQgZ2V0IGVuZHMgYXQgYSBwYXRoIHNlcGFyYXRvciAocG90ZW50aWFsbHkgdGhlIG9uZSB3ZSBhZGRlZFxuICAgICAgLy8gYWJvdmUgYXQgdGhlIGJlZ2lubmluZyBvZiB0aGUgcGF0aCwgaWYgdGhlIGFjdHVhbCBtYXRjaCB3YXMgZW1wdHkpLlxuICAgICAgaWYgKFxuICAgICAgICByZW1haW5pbmdQYXRobmFtZSAmJlxuICAgICAgICBtYXRjaGVkUGF0aC5jaGFyQXQobWF0Y2hlZFBhdGgubGVuZ3RoIC0gMSkgIT09ICcvJ1xuICAgICAgKSB7XG4gICAgICAgIHJldHVybiB7XG4gICAgICAgICAgcmVtYWluaW5nUGF0aG5hbWU6IG51bGwsXG4gICAgICAgICAgcGFyYW1OYW1lcyxcbiAgICAgICAgICBwYXJhbVZhbHVlczogbnVsbFxuICAgICAgICB9XG4gICAgICB9XG4gICAgfSBlbHNlIHtcbiAgICAgIC8vIElmIHRoaXMgbWF0Y2hlZCBhdCBhbGwsIHRoZW4gdGhlIG1hdGNoIHdhcyB0aGUgZW50aXJlIHBhdGhuYW1lLlxuICAgICAgcmVtYWluaW5nUGF0aG5hbWUgPSAnJ1xuICAgIH1cblxuICAgIHBhcmFtVmFsdWVzID0gbWF0Y2guc2xpY2UoMSkubWFwKFxuICAgICAgdiA9PiB2ICE9IG51bGwgPyBkZWNvZGVVUklDb21wb25lbnQodikgOiB2XG4gICAgKVxuICB9IGVsc2Uge1xuICAgIHJlbWFpbmluZ1BhdGhuYW1lID0gcGFyYW1WYWx1ZXMgPSBudWxsXG4gIH1cblxuICByZXR1cm4ge1xuICAgIHJlbWFpbmluZ1BhdGhuYW1lLFxuICAgIHBhcmFtTmFtZXMsXG4gICAgcGFyYW1WYWx1ZXNcbiAgfVxufVxuXG5leHBvcnQgZnVuY3Rpb24gZ2V0UGFyYW1OYW1lcyhwYXR0ZXJuKSB7XG4gIHJldHVybiBjb21waWxlUGF0dGVybihwYXR0ZXJuKS5wYXJhbU5hbWVzXG59XG5cbmV4cG9ydCBmdW5jdGlvbiBnZXRQYXJhbXMocGF0dGVybiwgcGF0aG5hbWUpIHtcbiAgY29uc3QgeyBwYXJhbU5hbWVzLCBwYXJhbVZhbHVlcyB9ID0gbWF0Y2hQYXR0ZXJuKHBhdHRlcm4sIHBhdGhuYW1lKVxuXG4gIGlmIChwYXJhbVZhbHVlcyAhPSBudWxsKSB7XG4gICAgcmV0dXJuIHBhcmFtTmFtZXMucmVkdWNlKGZ1bmN0aW9uIChtZW1vLCBwYXJhbU5hbWUsIGluZGV4KSB7XG4gICAgICBtZW1vW3BhcmFtTmFtZV0gPSBwYXJhbVZhbHVlc1tpbmRleF1cbiAgICAgIHJldHVybiBtZW1vXG4gICAgfSwge30pXG4gIH1cblxuICByZXR1cm4gbnVsbFxufVxuXG4vKipcbiAqIFJldHVybnMgYSB2ZXJzaW9uIG9mIHRoZSBnaXZlbiBwYXR0ZXJuIHdpdGggcGFyYW1zIGludGVycG9sYXRlZC4gVGhyb3dzXG4gKiBpZiB0aGVyZSBpcyBhIGR5bmFtaWMgc2VnbWVudCBvZiB0aGUgcGF0dGVybiBmb3Igd2hpY2ggdGhlcmUgaXMgbm8gcGFyYW0uXG4gKi9cbmV4cG9ydCBmdW5jdGlvbiBmb3JtYXRQYXR0ZXJuKHBhdHRlcm4sIHBhcmFtcykge1xuICBwYXJhbXMgPSBwYXJhbXMgfHwge31cblxuICBjb25zdCB7IHRva2VucyB9ID0gY29tcGlsZVBhdHRlcm4ocGF0dGVybilcbiAgbGV0IHBhcmVuQ291bnQgPSAwLCBwYXRobmFtZSA9ICcnLCBzcGxhdEluZGV4ID0gMFxuXG4gIGxldCB0b2tlbiwgcGFyYW1OYW1lLCBwYXJhbVZhbHVlXG4gIGZvciAobGV0IGkgPSAwLCBsZW4gPSB0b2tlbnMubGVuZ3RoOyBpIDwgbGVuOyArK2kpIHtcbiAgICB0b2tlbiA9IHRva2Vuc1tpXVxuXG4gICAgaWYgKHRva2VuID09PSAnKicgfHwgdG9rZW4gPT09ICcqKicpIHtcbiAgICAgIHBhcmFtVmFsdWUgPSBBcnJheS5pc0FycmF5KHBhcmFtcy5zcGxhdCkgPyBwYXJhbXMuc3BsYXRbc3BsYXRJbmRleCsrXSA6IHBhcmFtcy5zcGxhdFxuXG4gICAgICBpbnZhcmlhbnQoXG4gICAgICAgIHBhcmFtVmFsdWUgIT0gbnVsbCB8fCBwYXJlbkNvdW50ID4gMCxcbiAgICAgICAgJ01pc3Npbmcgc3BsYXQgIyVzIGZvciBwYXRoIFwiJXNcIicsXG4gICAgICAgIHNwbGF0SW5kZXgsIHBhdHRlcm5cbiAgICAgIClcblxuICAgICAgaWYgKHBhcmFtVmFsdWUgIT0gbnVsbClcbiAgICAgICAgcGF0aG5hbWUgKz0gZW5jb2RlVVJJKHBhcmFtVmFsdWUpXG4gICAgfSBlbHNlIGlmICh0b2tlbiA9PT0gJygnKSB7XG4gICAgICBwYXJlbkNvdW50ICs9IDFcbiAgICB9IGVsc2UgaWYgKHRva2VuID09PSAnKScpIHtcbiAgICAgIHBhcmVuQ291bnQgLT0gMVxuICAgIH0gZWxzZSBpZiAodG9rZW4uY2hhckF0KDApID09PSAnOicpIHtcbiAgICAgIHBhcmFtTmFtZSA9IHRva2VuLnN1YnN0cmluZygxKVxuICAgICAgcGFyYW1WYWx1ZSA9IHBhcmFtc1twYXJhbU5hbWVdXG5cbiAgICAgIGludmFyaWFudChcbiAgICAgICAgcGFyYW1WYWx1ZSAhPSBudWxsIHx8IHBhcmVuQ291bnQgPiAwLFxuICAgICAgICAnTWlzc2luZyBcIiVzXCIgcGFyYW1ldGVyIGZvciBwYXRoIFwiJXNcIicsXG4gICAgICAgIHBhcmFtTmFtZSwgcGF0dGVyblxuICAgICAgKVxuXG4gICAgICBpZiAocGFyYW1WYWx1ZSAhPSBudWxsKVxuICAgICAgICBwYXRobmFtZSArPSBlbmNvZGVVUklDb21wb25lbnQocGFyYW1WYWx1ZSlcbiAgICB9IGVsc2Uge1xuICAgICAgcGF0aG5hbWUgKz0gdG9rZW5cbiAgICB9XG4gIH1cblxuICByZXR1cm4gcGF0aG5hbWUucmVwbGFjZSgvXFwvKy9nLCAnLycpXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tbW9uL3BhdHRlcm5VdGlscy5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciBhcGkgPSByZXF1aXJlKCdhcHAvc2VydmljZXMvYXBpJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xuXG52YXIgeyBUTFBUX1NFU1NJTlNfUkVDRUlWRSB9ICA9IHJlcXVpcmUoJy4vc2Vzc2lvbnMvYWN0aW9uVHlwZXMnKTtcbnZhciB7IFRMUFRfTk9ERVNfUkVDRUlWRSB9ICA9IHJlcXVpcmUoJy4vbm9kZXMvYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICBmZXRjaE5vZGVzQW5kU2Vzc2lvbnMoKXtcbiAgICBhcGkuZ2V0KGNmZy5hcGkubm9kZXNQYXRoKS5kb25lKGpzb249PntcbiAgICAgIHZhciBub2RlQXJyYXkgPSBbXTtcbiAgICAgIHZhciBzZXNzaW9ucyA9IHt9O1xuXG4gICAgICBqc29uLm5vZGVzLmZvckVhY2goaXRlbT0+IHtcbiAgICAgICAgbm9kZUFycmF5LnB1c2goaXRlbS5ub2RlKTtcbiAgICAgICAgaWYoaXRlbS5zZXNzaW9ucyl7XG4gICAgICAgICAgaXRlbS5zZXNzaW9ucy5mb3JFYWNoKGl0ZW0yPT57XG4gICAgICAgICAgICBzZXNzaW9uc1tpdGVtMi5pZF0gPSBpdGVtMjtcbiAgICAgICAgICB9KVxuICAgICAgICB9XG4gICAgICB9KTtcblxuICAgICAgcmVhY3Rvci5iYXRjaCgoKSA9PiB7XG4gICAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9OT0RFU19SRUNFSVZFLCBub2RlQXJyYXkpO1xuICAgICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfU0VTU0lOU19SRUNFSVZFLCBzZXNzaW9ucyk7XG4gICAgICB9KTtcblxuICAgIH0pO1xuICB9XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hY3Rpb25zLmpzXG4gKiovIiwiY29uc3QgYWN0aXZlU2Vzc2lvbiA9IFsgWyd0bHB0X2FjdGl2ZV90ZXJtaW5hbCddLCAoYWN0aXZlU2Vzc2lvbikgPT4ge1xuICAgIGlmKCFhY3RpdmVTZXNzaW9uKXtcbiAgICAgIHJldHVybiBudWxsO1xuICAgIH1cblxuICAgIHJldHVybiBhY3RpdmVTZXNzaW9uLnRvSlMoKTtcbiAgfVxuXTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICBhY3RpdmVTZXNzaW9uXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9nZXR0ZXJzLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMuZ2V0dGVycyA9IHJlcXVpcmUoJy4vZ2V0dGVycycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aW9ucyA9IHJlcXVpcmUoJy4vYWN0aW9ucycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aXZlVGVybVN0b3JlID0gcmVxdWlyZSgnLi9hY3RpdmVUZXJtU3RvcmUnKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2luZGV4LmpzXG4gKiovIiwidmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xucmVhY3Rvci5yZWdpc3RlclN0b3Jlcyh7XG4gICd0bHB0X2FjdGl2ZV90ZXJtaW5hbCc6IHJlcXVpcmUoJy4vYWN0aXZlVGVybWluYWwvYWN0aXZlVGVybVN0b3JlJyksXG4gICd0bHB0X3VzZXInOiByZXF1aXJlKCcuL3VzZXIvdXNlclN0b3JlJyksXG4gICd0bHB0X25vZGVzJzogcmVxdWlyZSgnLi9ub2Rlcy9ub2RlU3RvcmUnKSxcbiAgJ3RscHRfaW52aXRlJzogcmVxdWlyZSgnLi9pbnZpdGUvaW52aXRlU3RvcmUnKSxcbiAgJ3RscHRfcmVzdF9hcGknOiByZXF1aXJlKCcuL3Jlc3RBcGkvcmVzdEFwaVN0b3JlJyksXG4gICd0bHB0X3Nlc3Npb25zJzogcmVxdWlyZSgnLi9zZXNzaW9ucy9zZXNzaW9uU3RvcmUnKSAgXG59KTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2luZGV4LmpzXG4gKiovIiwidmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHsgVExQVF9SRUNFSVZFX1VTRVJfSU5WSVRFIH0gID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xudmFyIGFwaSA9IHJlcXVpcmUoJ2FwcC9zZXJ2aWNlcy9hcGknKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgZmV0Y2hJbnZpdGUoaW52aXRlVG9rZW4pe1xuICAgIHZhciBwYXRoID0gY2ZnLmFwaS5nZXRJbnZpdGVVcmwoaW52aXRlVG9rZW4pO1xuICAgIGFwaS5nZXQocGF0aCkuZG9uZShpbnZpdGU9PntcbiAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRUNFSVZFX1VTRVJfSU5WSVRFLCBpbnZpdGUpO1xuICAgIH0pO1xuICB9XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvYWN0aW9ucy5qc1xuICoqLyIsIi8qZXNsaW50IG5vLXVuZGVmOiAwLCAgbm8tdW51c2VkLXZhcnM6IDAsIG5vLWRlYnVnZ2VyOjAqL1xuXG52YXIge1RSWUlOR19UT19TSUdOX1VQfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3Jlc3RBcGkvY29uc3RhbnRzJyk7XG5cbmNvbnN0IGludml0ZSA9IFsgWyd0bHB0X2ludml0ZSddLCAoaW52aXRlKSA9PiB7XG4gIHJldHVybiBpbnZpdGU7XG4gfVxuXTtcblxuY29uc3QgYXR0ZW1wID0gWyBbJ3RscHRfcmVzdF9hcGknLCBUUllJTkdfVE9fU0lHTl9VUF0sIChhdHRlbXApID0+IHtcbiAgdmFyIGRlZmF1bHRPYmogPSB7XG4gICAgaXNQcm9jZXNzaW5nOiBmYWxzZSxcbiAgICBpc0Vycm9yOiBmYWxzZSxcbiAgICBpc1N1Y2Nlc3M6IGZhbHNlLFxuICAgIG1lc3NhZ2U6ICcnXG4gIH1cblxuICByZXR1cm4gYXR0ZW1wID8gYXR0ZW1wLnRvSlMoKSA6IGRlZmF1bHRPYmo7XG4gIFxuIH1cbl07XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgaW52aXRlLFxuICBhdHRlbXBcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9nZXR0ZXJzLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMuZ2V0dGVycyA9IHJlcXVpcmUoJy4vZ2V0dGVycycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aW9ucyA9IHJlcXVpcmUoJy4vYWN0aW9ucycpO1xubW9kdWxlLmV4cG9ydHMubm9kZVN0b3JlID0gcmVxdWlyZSgnLi9pbnZpdGVTdG9yZScpO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2luZGV4LmpzXG4gKiovIiwidmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHsgVExQVF9OT0RFU19SRUNFSVZFIH0gID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xudmFyIGFwaSA9IHJlcXVpcmUoJ2FwcC9zZXJ2aWNlcy9hcGknKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgZmV0Y2hOb2Rlcygpe1xuICAgIGFwaS5nZXQoY2ZnLmFwaS5ub2Rlc1BhdGgpLmRvbmUoZGF0YT0+e1xuICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX05PREVTX1JFQ0VJVkUsIGRhdGEubm9kZXMpO1xuICAgIH0pO1xuICB9XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9hY3Rpb25zLmpzXG4gKiovIiwidmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHtzZXNzaW9uc0J5U2VydmVyfSA9IHJlcXVpcmUoJy4vLi4vc2Vzc2lvbnMvZ2V0dGVycycpO1xuXG5jb25zdCBub2RlTGlzdFZpZXcgPSBbIFsndGxwdF9ub2RlcyddLCAobm9kZXMpID0+e1xuICAgIHJldHVybiBub2Rlcy5tYXAoKGl0ZW0pPT57XG4gICAgICB2YXIgYWRkciA9IGl0ZW0uZ2V0KCdhZGRyJyk7XG4gICAgICB2YXIgc2Vzc2lvbnMgPSByZWFjdG9yLmV2YWx1YXRlKHNlc3Npb25zQnlTZXJ2ZXIoYWRkcikpO1xuICAgICAgcmV0dXJuIHtcbiAgICAgICAgdGFnczogZ2V0VGFncyhpdGVtKSxcbiAgICAgICAgYWRkcjogYWRkcixcbiAgICAgICAgc2Vzc2lvbkNvdW50OiBzZXNzaW9ucy5zaXplXG4gICAgICB9XG4gICAgfSkudG9KUygpO1xuIH1cbl07XG5cbmZ1bmN0aW9uIGdldFRhZ3Mobm9kZSl7XG4gIHZhciBhbGxMYWJlbHMgPSBbXTtcbiAgdmFyIGxhYmVscyA9IG5vZGUuZ2V0KCdsYWJlbHMnKTtcblxuICBpZihsYWJlbHMpe1xuICAgIGxhYmVscy5lbnRyeVNlcSgpLnRvQXJyYXkoKS5mb3JFYWNoKGl0ZW09PntcbiAgICAgIGFsbExhYmVscy5wdXNoKHtcbiAgICAgICAgcm9sZTogaXRlbVswXSxcbiAgICAgICAgdmFsdWU6IGl0ZW1bMV1cbiAgICAgIH0pO1xuICAgIH0pO1xuICB9XG5cbiAgbGFiZWxzID0gbm9kZS5nZXQoJ2NtZF9sYWJlbHMnKTtcblxuICBpZihsYWJlbHMpe1xuICAgIGxhYmVscy5lbnRyeVNlcSgpLnRvQXJyYXkoKS5mb3JFYWNoKGl0ZW09PntcbiAgICAgIGFsbExhYmVscy5wdXNoKHtcbiAgICAgICAgcm9sZTogaXRlbVswXSxcbiAgICAgICAgdmFsdWU6IGl0ZW1bMV0uZ2V0KCdyZXN1bHQnKSxcbiAgICAgICAgdG9vbHRpcDogaXRlbVsxXS5nZXQoJ2NvbW1hbmQnKVxuICAgICAgfSk7XG4gICAgfSk7XG4gIH1cblxuICByZXR1cm4gYWxsTGFiZWxzO1xufVxuXG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgbm9kZUxpc3RWaWV3XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9nZXR0ZXJzLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMuZ2V0dGVycyA9IHJlcXVpcmUoJy4vZ2V0dGVycycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aW9ucyA9IHJlcXVpcmUoJy4vYWN0aW9ucycpO1xubW9kdWxlLmV4cG9ydHMubm9kZVN0b3JlID0gcmVxdWlyZSgnLi9ub2RlU3RvcmUnKTtcblxuLy8gbm9kZXM6IFt7XCJpZFwiOlwieDIyMFwiLFwiYWRkclwiOlwiMC4wLjAuMDozMDIyXCIsXCJob3N0bmFtZVwiOlwieDIyMFwiLFwibGFiZWxzXCI6bnVsbCxcImNtZF9sYWJlbHNcIjpudWxsfV1cblxuXG4vLyBzZXNzaW9uczogW3tcImlkXCI6XCIwNzYzMDYzNi1iYjNkLTQwZTEtYjA4Ni02MGIyY2FlMjFhYzRcIixcInBhcnRpZXNcIjpbe1wiaWRcIjpcIjg5Zjc2MmEzLTc0MjktNGM3YS1hOTEzLTc2NjQ5M2ZlN2M4YVwiLFwic2l0ZVwiOlwiMTI3LjAuMC4xOjM3NTE0XCIsXCJ1c2VyXCI6XCJha29udHNldm95XCIsXCJzZXJ2ZXJfYWRkclwiOlwiMC4wLjAuMDozMDIyXCIsXCJsYXN0X2FjdGl2ZVwiOlwiMjAxNi0wMi0yMlQxNDozOToyMC45MzEyMDUzNS0wNTowMFwifV19XVxuXG4vKlxubGV0IFRvZG9SZWNvcmQgPSBJbW11dGFibGUuUmVjb3JkKHtcbiAgICBpZDogMCxcbiAgICBkZXNjcmlwdGlvbjogXCJcIixcbiAgICBjb21wbGV0ZWQ6IGZhbHNlXG59KTtcbiovXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9pbmRleC5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcblxudmFyIHtcbiAgVExQVF9SRVNUX0FQSV9TVEFSVCxcbiAgVExQVF9SRVNUX0FQSV9TVUNDRVNTLFxuICBUTFBUX1JFU1RfQVBJX0ZBSUwgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuXG4gIHN0YXJ0KHJlcVR5cGUpe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRVNUX0FQSV9TVEFSVCwge3R5cGU6IHJlcVR5cGV9KTtcbiAgfSxcblxuICBmYWlsKHJlcVR5cGUsIG1lc3NhZ2Upe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRVNUX0FQSV9GQUlMLCAge3R5cGU6IHJlcVR5cGUsIG1lc3NhZ2V9KTtcbiAgfSxcblxuICBzdWNjZXNzKHJlcVR5cGUpe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRVNUX0FQSV9TVUNDRVNTLCB7dHlwZTogcmVxVHlwZX0pO1xuICB9XG5cbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvYWN0aW9ucy5qc1xuICoqLyIsInZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyIHtcbiAgVExQVF9SRVNUX0FQSV9TVEFSVCxcbiAgVExQVF9SRVNUX0FQSV9TVUNDRVNTLFxuICBUTFBUX1JFU1RfQVBJX0ZBSUwgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKHt9KTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9SRVNUX0FQSV9TVEFSVCwgc3RhcnQpO1xuICAgIHRoaXMub24oVExQVF9SRVNUX0FQSV9GQUlMLCBmYWlsKTtcbiAgICB0aGlzLm9uKFRMUFRfUkVTVF9BUElfU1VDQ0VTUywgc3VjY2Vzcyk7XG4gIH1cbn0pXG5cbmZ1bmN0aW9uIHN0YXJ0KHN0YXRlLCByZXF1ZXN0KXtcbiAgcmV0dXJuIHN0YXRlLnNldChyZXF1ZXN0LnR5cGUsIHRvSW1tdXRhYmxlKHtpc1Byb2Nlc3Npbmc6IHRydWV9KSk7XG59XG5cbmZ1bmN0aW9uIGZhaWwoc3RhdGUsIHJlcXVlc3Qpe1xuICByZXR1cm4gc3RhdGUuc2V0KHJlcXVlc3QudHlwZSwgdG9JbW11dGFibGUoe2lzRmFpbGVkOiB0cnVlLCBtZXNzYWdlOiByZXF1ZXN0Lm1lc3NhZ2V9KSk7XG59XG5cbmZ1bmN0aW9uIHN1Y2Nlc3Moc3RhdGUsIHJlcXVlc3Qpe1xuICByZXR1cm4gc3RhdGUuc2V0KHJlcXVlc3QudHlwZSwgdG9JbW11dGFibGUoe2lzU3VjY2VzczogdHJ1ZX0pKTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvcmVzdEFwaVN0b3JlLmpzXG4gKiovIiwidmFyIHsgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcblxuY29uc3Qgc2Vzc2lvbnNCeVNlcnZlciA9IChhZGRyKSA9PiBbWyd0bHB0X3Nlc3Npb25zJ10sIChzZXNzaW9ucykgPT57XG4gIHJldHVybiBzZXNzaW9ucy52YWx1ZVNlcSgpLmZpbHRlcihpdGVtPT57XG4gICAgdmFyIHBhcnRpZXMgPSBpdGVtLmdldCgncGFydGllcycpIHx8IHRvSW1tdXRhYmxlKFtdKTtcbiAgICB2YXIgaGFzU2VydmVyID0gcGFydGllcy5maW5kKGl0ZW0yPT4gaXRlbTIuZ2V0KCdzZXJ2ZXJfYWRkcicpID09PSBhZGRyKTtcbiAgICByZXR1cm4gaGFzU2VydmVyO1xuICB9KS50b0xpc3QoKTtcbn1dXG5cbmNvbnN0IHBhcnRpZXNCeVNlc3Npb25JZCA9IChzaWQpID0+XG4gW1sndGxwdF9zZXNzaW9ucycsIHNpZCwgJ3BhcnRpZXMnXSwgKHBhcnRpZXMpID0+e1xuXG4gIGlmKCFwYXJ0aWVzKXtcbiAgICByZXR1cm4gW107XG4gIH1cblxuICB2YXIgbGFzdEFjdGl2ZVVzck5hbWUgPSBnZXRMYXN0QWN0aXZlVXNlcihwYXJ0aWVzKTtcblxuICByZXR1cm4gcGFydGllcy5tYXAoaXRlbT0+e1xuICAgIHZhciB1c2VyID0gaXRlbS5nZXQoJ3VzZXInKTtcbiAgICByZXR1cm4ge1xuICAgICAgdXNlcjogaXRlbS5nZXQoJ3VzZXInKSxcbiAgICAgIGlzQWN0aXZlOiBsYXN0QWN0aXZlVXNyTmFtZSA9PT0gdXNlclxuICAgIH1cbiAgfSkudG9KUygpO1xufV07XG5cbmZ1bmN0aW9uIGdldExhc3RBY3RpdmVVc2VyKHBhcnRpZXMpe1xuICByZXR1cm4gcGFydGllcy5zb3J0QnkoaXRlbT0+IG5ldyBEYXRlKGl0ZW0uZ2V0KCdsYXN0QWN0aXZlJykpKS5maXJzdCgpLmdldCgndXNlcicpO1xufVxuXG5leHBvcnQgZGVmYXVsdCB7XG4gIHBhcnRpZXNCeVNlc3Npb25JZCxcbiAgc2Vzc2lvbnNCeVNlcnZlclxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvZ2V0dGVycy5qc1xuICoqLyIsInZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyIHsgVExQVF9TRVNTSU5TX1JFQ0VJVkUsIFRMUFRfU0VTU0lOU19VUERBVEUgfSAgPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IFN0b3JlKHtcbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB0b0ltbXV0YWJsZSh7fSk7XG4gIH0sXG5cbiAgaW5pdGlhbGl6ZSgpIHtcbiAgICB0aGlzLm9uKFRMUFRfU0VTU0lOU19SRUNFSVZFLCByZWNlaXZlU2Vzc2lvbnMpO1xuICAgIHRoaXMub24oVExQVF9TRVNTSU5TX1VQREFURSwgdXBkYXRlU2Vzc2lvbik7XG4gIH1cbn0pXG5cbmZ1bmN0aW9uIHVwZGF0ZVNlc3Npb24oc3RhdGUsIGpzb24pe1xuICByZXR1cm4gc3RhdGUuc2V0KGpzb24uaWQsIHRvSW1tdXRhYmxlKGpzb24pKTtcbn1cblxuZnVuY3Rpb24gcmVjZWl2ZVNlc3Npb25zKHN0YXRlLCBqc29uKXtcbiAgcmV0dXJuIHRvSW1tdXRhYmxlKGpzb24pO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvc2Vzc2lvblN0b3JlLmpzXG4gKiovIiwidmFyIHV0aWxzID0ge1xuXG4gIHV1aWQoKXtcbiAgICAvLyBuZXZlciB1c2UgaXQgaW4gcHJvZHVjdGlvblxuICAgIHJldHVybiAneHh4eHh4eHgteHh4eC00eHh4LXl4eHgteHh4eHh4eHh4eHh4Jy5yZXBsYWNlKC9beHldL2csIGZ1bmN0aW9uKGMpIHtcbiAgICAgIHZhciByID0gTWF0aC5yYW5kb20oKSoxNnwwLCB2ID0gYyA9PSAneCcgPyByIDogKHImMHgzfDB4OCk7XG4gICAgICByZXR1cm4gdi50b1N0cmluZygxNik7XG4gICAgfSk7XG4gIH0sXG5cbiAgZGlzcGxheURhdGUoZGF0ZSl7XG4gICAgdHJ5e1xuICAgICAgcmV0dXJuIGRhdGUudG9Mb2NhbGVEYXRlU3RyaW5nKCkgKyAnICcgKyBkYXRlLnRvTG9jYWxlVGltZVN0cmluZygpO1xuICAgIH1jYXRjaChlcnIpe1xuICAgICAgY29uc29sZS5lcnJvcihlcnIpO1xuICAgICAgcmV0dXJuICd1bmRlZmluZWQnO1xuICAgIH1cbiAgfSxcblxuICBmb3JtYXRTdHJpbmcoZm9ybWF0KSB7XG4gICAgdmFyIGFyZ3MgPSBBcnJheS5wcm90b3R5cGUuc2xpY2UuY2FsbChhcmd1bWVudHMsIDEpO1xuICAgIHJldHVybiBmb3JtYXQucmVwbGFjZShuZXcgUmVnRXhwKCdcXFxceyhcXFxcZCspXFxcXH0nLCAnZycpLFxuICAgICAgKG1hdGNoLCBudW1iZXIpID0+IHtcbiAgICAgICAgcmV0dXJuICEoYXJnc1tudW1iZXJdID09PSBudWxsIHx8IGFyZ3NbbnVtYmVyXSA9PT0gdW5kZWZpbmVkKSA/IGFyZ3NbbnVtYmVyXSA6ICcnO1xuICAgIH0pO1xuICB9XG59XG5cbm1vZHVsZS5leHBvcnRzID0gdXRpbHM7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvdXRpbHMuanNcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIE5hdkxlZnRCYXIgPSByZXF1aXJlKCcuL25hdkxlZnRCYXInKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG52YXIge1Rlcm1pbmFsSG9zdH0gPSByZXF1aXJlKCcuL3Rlcm1pbmFsSG9zdC5qc3gnKTtcbnZhciBhY3Rpb25zID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvYWN0aW9ucycpO1xuXG52YXIgQXBwID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIGNvbXBvbmVudERpZE1vdW50KCl7XG4gICAgYWN0aW9ucy5mZXRjaE5vZGVzQW5kU2Vzc2lvbnMoKTtcbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi10bHB0XCI+XG4gICAgICAgIDxUZXJtaW5hbEhvc3QvPlxuICAgICAgICA8TmF2TGVmdEJhci8+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwicm93XCI+XG4gICAgICAgICAgPG5hdiBjbGFzc05hbWU9XCJcIiByb2xlPVwibmF2aWdhdGlvblwiIHN0eWxlPXt7IG1hcmdpbkJvdHRvbTogMCB9fT5cbiAgICAgICAgICAgIDx1bCBjbGFzc05hbWU9XCJuYXYgbmF2YmFyLXRvcC1saW5rcyBuYXZiYXItcmlnaHRcIj5cbiAgICAgICAgICAgICAgPGxpPlxuICAgICAgICAgICAgICAgIDxhIGhyZWY9e2NmZy5yb3V0ZXMubG9nb3V0fT5cbiAgICAgICAgICAgICAgICAgIDxpIGNsYXNzTmFtZT1cImZhIGZhLXNpZ24tb3V0XCI+PC9pPlxuICAgICAgICAgICAgICAgICAgTG9nIG91dFxuICAgICAgICAgICAgICAgIDwvYT5cbiAgICAgICAgICAgICAgPC9saT5cbiAgICAgICAgICAgIDwvdWw+XG4gICAgICAgICAgPC9uYXY+XG4gICAgICAgIDwvZGl2PlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1wYWdlXCI+XG4gICAgICAgICAge3RoaXMucHJvcHMuY2hpbGRyZW59XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9kaXY+XG4gICAgKTtcbiAgfVxufSlcblxubW9kdWxlLmV4cG9ydHMgPSBBcHA7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9hcHAuanN4XG4gKiovIiwidmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcbnZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG5cbnZhciBFdmVudFN0cmVhbWVyID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICBjb21wb25lbnREaWRNb3VudCgpIHtcbiAgICB2YXIge3Rva2VuLCBzaWR9ID0gdGhpcy5wcm9wcztcbiAgICB2YXIgY29ublN0ciA9IGNmZy5hcGkuZ2V0RXZlbnRTdHJlYW1lckNvbm5TdHIodG9rZW4sIHNpZCk7XG5cbiAgICB0aGlzLnNvY2tldCA9IG5ldyBXZWJTb2NrZXQoY29ublN0ciwgXCJwcm90b1wiKTtcbiAgICB0aGlzLnNvY2tldC5vbm1lc3NhZ2UgPSAoKSA9PiB7fTtcbiAgICB0aGlzLnNvY2tldC5vbmNsb3NlID0gKCkgPT4ge307XG4gIH0sXG5cbiAgY29tcG9uZW50V2lsbFVubW91bnQoKSB7XG4gICAgdGhpcy5zb2NrZXQuY2xvc2UoKTtcbiAgfSxcblxuICBzaG91bGRDb21wb25lbnRVcGRhdGUoKXtcbiAgICByZXR1cm4gZmFsc2U7XG4gIH0sXG5cbiAgcmVuZGVyKCkge1xuICAgIHJldHVybiBudWxsO1xuICB9XG59KTtcblxuZXhwb3J0IGRlZmF1bHQgRXZlbnRTdHJlYW1lcjtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2V2ZW50U3RyZWFtZXIuanN4XG4gKiovIiwibW9kdWxlLmV4cG9ydHMuQXBwID0gcmVxdWlyZSgnLi9hcHAuanN4Jyk7XG5tb2R1bGUuZXhwb3J0cy5Mb2dpbiA9IHJlcXVpcmUoJy4vbG9naW4uanN4Jyk7XG5tb2R1bGUuZXhwb3J0cy5OZXdVc2VyID0gcmVxdWlyZSgnLi9uZXdVc2VyLmpzeCcpO1xubW9kdWxlLmV4cG9ydHMuTm9kZXMgPSByZXF1aXJlKCcuL25vZGVzL21haW4uanN4Jyk7XG5tb2R1bGUuZXhwb3J0cy5TZXNzaW9ucyA9IHJlcXVpcmUoJy4vc2Vzc2lvbnMvbWFpbi5qc3gnKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2luZGV4LmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgJCA9IHJlcXVpcmUoJ2pRdWVyeScpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIExpbmtlZFN0YXRlTWl4aW4gPSByZXF1aXJlKCdyZWFjdC1hZGRvbnMtbGlua2VkLXN0YXRlLW1peGluJyk7XG52YXIge2FjdGlvbnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvdXNlcicpO1xudmFyIEdvb2dsZUF1dGhJbmZvID0gcmVxdWlyZSgnLi9nb29nbGVBdXRoJyk7XG52YXIgTG9naW5JbnB1dEZvcm0gPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbTGlua2VkU3RhdGVNaXhpbl0sXG5cbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB7XG4gICAgICB1c2VyOiAnJyxcbiAgICAgIHBhc3N3b3JkOiAnJyxcbiAgICAgIHRva2VuOiAnJ1xuICAgIH1cbiAgfSxcblxuICBvbkNsaWNrOiBmdW5jdGlvbihlKSB7XG4gICAgZS5wcmV2ZW50RGVmYXVsdCgpO1xuICAgIGlmICh0aGlzLmlzVmFsaWQoKSkge1xuICAgICAgYWN0aW9ucy5sb2dpbih7IC4uLnRoaXMuc3RhdGV9LCAnL3dlYicpO1xuICAgIH1cbiAgfSxcblxuICBpc1ZhbGlkOiBmdW5jdGlvbigpIHtcbiAgICB2YXIgJGZvcm0gPSAkKHRoaXMucmVmcy5mb3JtKTtcbiAgICByZXR1cm4gJGZvcm0ubGVuZ3RoID09PSAwIHx8ICRmb3JtLnZhbGlkKCk7XG4gIH0sXG5cbiAgcmVuZGVyKCkge1xuICAgIHJldHVybiAoXG4gICAgICA8Zm9ybSByZWY9XCJmb3JtXCIgY2xhc3NOYW1lPVwiZ3J2LWxvZ2luLWlucHV0LWZvcm1cIj5cbiAgICAgICAgPGgzPiBXZWxjb21lIHRvIFRlbGVwb3J0IDwvaDM+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiXCI+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXQgdmFsdWVMaW5rPXt0aGlzLmxpbmtTdGF0ZSgndXNlcicpfSBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2wgcmVxdWlyZWRcIiBwbGFjZWhvbGRlcj1cIlVzZXIgbmFtZVwiIG5hbWU9XCJ1c2VyTmFtZVwiIC8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXQgdmFsdWVMaW5rPXt0aGlzLmxpbmtTdGF0ZSgncGFzc3dvcmQnKX0gdHlwZT1cInBhc3N3b3JkXCIgbmFtZT1cInBhc3N3b3JkXCIgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sIHJlcXVpcmVkXCIgcGxhY2Vob2xkZXI9XCJQYXNzd29yZFwiLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImZvcm0tZ3JvdXBcIj5cbiAgICAgICAgICAgIDxpbnB1dCB2YWx1ZUxpbms9e3RoaXMubGlua1N0YXRlKCd0b2tlbicpfSBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2wgcmVxdWlyZWRcIiBuYW1lPVwidG9rZW5cIiBwbGFjZWhvbGRlcj1cIlR3byBmYWN0b3IgdG9rZW4gKEdvb2dsZSBBdXRoZW50aWNhdG9yKVwiLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8YnV0dG9uIHR5cGU9XCJzdWJtaXRcIiBjbGFzc05hbWU9XCJidG4gYnRuLXByaW1hcnkgYmxvY2sgZnVsbC13aWR0aCBtLWJcIiBvbkNsaWNrPXt0aGlzLm9uQ2xpY2t9PkxvZ2luPC9idXR0b24+XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9mb3JtPlxuICAgICk7XG4gIH1cbn0pXG5cbnZhciBMb2dpbiA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBtaXhpbnM6IFtyZWFjdG9yLlJlYWN0TWl4aW5dLFxuXG4gIGdldERhdGFCaW5kaW5ncygpIHtcbiAgICByZXR1cm4ge1xuICAvLyAgICB1c2VyUmVxdWVzdDogZ2V0dGVycy51c2VyUmVxdWVzdFxuICAgIH1cbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIHZhciBpc1Byb2Nlc3NpbmcgPSBmYWxzZTsvL3RoaXMuc3RhdGUudXNlclJlcXVlc3QuZ2V0KCdpc0xvYWRpbmcnKTtcbiAgICB2YXIgaXNFcnJvciA9IGZhbHNlOy8vdGhpcy5zdGF0ZS51c2VyUmVxdWVzdC5nZXQoJ2lzRXJyb3InKTtcblxuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1sb2dpbiB0ZXh0LWNlbnRlclwiPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1sb2dvLXRwcnRcIj48L2Rpdj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtY29udGVudCBncnYtZmxleFwiPlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWZsZXgtY29sdW1uXCI+XG4gICAgICAgICAgICA8TG9naW5JbnB1dEZvcm0vPlxuICAgICAgICAgICAgPEdvb2dsZUF1dGhJbmZvLz5cbiAgICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWxvZ2luLWluZm9cIj5cbiAgICAgICAgICAgICAgPGkgY2xhc3NOYW1lPVwiZmEgZmEtcXVlc3Rpb25cIj48L2k+XG4gICAgICAgICAgICAgIDxzdHJvbmc+TmV3IEFjY291bnQgb3IgZm9yZ290IHBhc3N3b3JkPzwvc3Ryb25nPlxuICAgICAgICAgICAgICA8ZGl2PkFzayBmb3IgYXNzaXN0YW5jZSBmcm9tIHlvdXIgQ29tcGFueSBhZG1pbmlzdHJhdG9yPC9kaXY+XG4gICAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBMb2dpbjtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2xvZ2luLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgeyBSb3V0ZXIsIEluZGV4TGluaywgSGlzdG9yeSB9ID0gcmVxdWlyZSgncmVhY3Qtcm91dGVyJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xuXG52YXIgbWVudUl0ZW1zID0gW1xuICB7aWNvbjogJ2ZhIGZhLWNvZ3MnLCB0bzogY2ZnLnJvdXRlcy5ub2RlcywgdGl0bGU6ICdOb2Rlcyd9LFxuICB7aWNvbjogJ2ZhIGZhLXNpdGVtYXAnLCB0bzogY2ZnLnJvdXRlcy5zZXNzaW9ucywgdGl0bGU6ICdTZXNzaW9ucyd9LFxuICB7aWNvbjogJ2ZhIGZhLXF1ZXN0aW9uJywgdG86IGNmZy5yb3V0ZXMuc2Vzc2lvbnMsIHRpdGxlOiAnU2Vzc2lvbnMnfSxcbl07XG5cbnZhciBOYXZMZWZ0QmFyID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIHJlbmRlcjogZnVuY3Rpb24oKXtcbiAgICB2YXIgaXRlbXMgPSBtZW51SXRlbXMubWFwKChpLCBpbmRleCk9PntcbiAgICAgIHZhciBjbGFzc05hbWUgPSB0aGlzLmNvbnRleHQucm91dGVyLmlzQWN0aXZlKGkudG8pID8gJ2FjdGl2ZScgOiAnJztcbiAgICAgIHJldHVybiAoXG4gICAgICAgIDxsaSBrZXk9e2luZGV4fSBjbGFzc05hbWU9e2NsYXNzTmFtZX0+XG4gICAgICAgICAgPEluZGV4TGluayB0bz17aS50b30+XG4gICAgICAgICAgICA8aSBjbGFzc05hbWU9e2kuaWNvbn0gdGl0bGU9e2kudGl0bGV9Lz5cbiAgICAgICAgICA8L0luZGV4TGluaz5cbiAgICAgICAgPC9saT5cbiAgICAgICk7XG4gICAgfSk7XG5cbiAgICByZXR1cm4gKFxuICAgICAgPG5hdiBjbGFzc05hbWU9Jycgcm9sZT0nbmF2aWdhdGlvbicgc3R5bGU9e3t3aWR0aDogJzYwcHgnLCBmbG9hdDogJ2xlZnQnLCBwb3NpdGlvbjogJ2Fic29sdXRlJ319PlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT0nJz5cbiAgICAgICAgICA8dWwgY2xhc3NOYW1lPSduYXYgMW1ldGlzbWVudScgaWQ9J3NpZGUtbWVudSc+XG4gICAgICAgICAgICB7aXRlbXN9XG4gICAgICAgICAgPC91bD5cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L25hdj5cbiAgICApO1xuICB9XG59KTtcblxuTmF2TGVmdEJhci5jb250ZXh0VHlwZXMgPSB7XG4gIHJvdXRlcjogUmVhY3QuUHJvcFR5cGVzLm9iamVjdC5pc1JlcXVpcmVkXG59XG5cbm1vZHVsZS5leHBvcnRzID0gTmF2TGVmdEJhcjtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL25hdkxlZnRCYXIuanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciAkID0gcmVxdWlyZSgnalF1ZXJ5Jyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIge2FjdGlvbnMsIGdldHRlcnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvaW52aXRlJyk7XG52YXIgdXNlck1vZHVsZSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3VzZXInKTtcbnZhciBMaW5rZWRTdGF0ZU1peGluID0gcmVxdWlyZSgncmVhY3QtYWRkb25zLWxpbmtlZC1zdGF0ZS1taXhpbicpO1xudmFyIEdvb2dsZUF1dGhJbmZvID0gcmVxdWlyZSgnLi9nb29nbGVBdXRoJyk7XG5cbnZhciBJbnZpdGVJbnB1dEZvcm0gPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbTGlua2VkU3RhdGVNaXhpbl0sXG5cbiAgY29tcG9uZW50RGlkTW91bnQoKXtcbiAgICAkKHRoaXMucmVmcy5mb3JtKS52YWxpZGF0ZSh7XG4gICAgICBydWxlczp7XG4gICAgICAgIHBhc3N3b3JkOntcbiAgICAgICAgICBtaW5sZW5ndGg6IDUsXG4gICAgICAgICAgcmVxdWlyZWQ6IHRydWVcbiAgICAgICAgfSxcbiAgICAgICAgcGFzc3dvcmRDb25maXJtZWQ6e1xuICAgICAgICAgIHJlcXVpcmVkOiB0cnVlLFxuICAgICAgICAgIGVxdWFsVG86IHRoaXMucmVmcy5wYXNzd29yZFxuICAgICAgICB9XG4gICAgICB9LFxuXG4gICAgICBtZXNzYWdlczoge1xuICBcdFx0XHRwYXNzd29yZENvbmZpcm1lZDoge1xuICBcdFx0XHRcdG1pbmxlbmd0aDogJC52YWxpZGF0b3IuZm9ybWF0KCdFbnRlciBhdCBsZWFzdCB7MH0gY2hhcmFjdGVycycpLFxuICBcdFx0XHRcdGVxdWFsVG86ICdFbnRlciB0aGUgc2FtZSBwYXNzd29yZCBhcyBhYm92ZSdcbiAgXHRcdFx0fVxuICAgICAgfVxuICAgIH0pXG4gIH0sXG5cbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB7XG4gICAgICBuYW1lOiB0aGlzLnByb3BzLmludml0ZS51c2VyLFxuICAgICAgcHN3OiAnJyxcbiAgICAgIHBzd0NvbmZpcm1lZDogJycsXG4gICAgICB0b2tlbjogJydcbiAgICB9XG4gIH0sXG5cbiAgb25DbGljayhlKSB7XG4gICAgZS5wcmV2ZW50RGVmYXVsdCgpO1xuICAgIGlmICh0aGlzLmlzVmFsaWQoKSkge1xuICAgICAgdXNlck1vZHVsZS5hY3Rpb25zLnNpZ25VcCh7XG4gICAgICAgIG5hbWU6IHRoaXMuc3RhdGUubmFtZSxcbiAgICAgICAgcHN3OiB0aGlzLnN0YXRlLnBzdyxcbiAgICAgICAgdG9rZW46IHRoaXMuc3RhdGUudG9rZW4sXG4gICAgICAgIGludml0ZVRva2VuOiB0aGlzLnByb3BzLmludml0ZS5pbnZpdGVfdG9rZW59KTtcbiAgICB9XG4gIH0sXG5cbiAgaXNWYWxpZCgpIHtcbiAgICB2YXIgJGZvcm0gPSAkKHRoaXMucmVmcy5mb3JtKTtcbiAgICByZXR1cm4gJGZvcm0ubGVuZ3RoID09PSAwIHx8ICRmb3JtLnZhbGlkKCk7XG4gIH0sXG5cbiAgcmVuZGVyKCkge1xuICAgIHJldHVybiAoXG4gICAgICA8Zm9ybSByZWY9XCJmb3JtXCIgY2xhc3NOYW1lPVwiZ3J2LWludml0ZS1pbnB1dC1mb3JtXCI+XG4gICAgICAgIDxoMz4gR2V0IHN0YXJ0ZWQgd2l0aCBUZWxlcG9ydCA8L2gzPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0XG4gICAgICAgICAgICAgIHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ25hbWUnKX1cbiAgICAgICAgICAgICAgbmFtZT1cInVzZXJOYW1lXCJcbiAgICAgICAgICAgICAgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sIHJlcXVpcmVkXCJcbiAgICAgICAgICAgICAgcGxhY2Vob2xkZXI9XCJVc2VyIG5hbWVcIi8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXRcbiAgICAgICAgICAgICAgdmFsdWVMaW5rPXt0aGlzLmxpbmtTdGF0ZSgncHN3Jyl9XG4gICAgICAgICAgICAgIHJlZj1cInBhc3N3b3JkXCJcbiAgICAgICAgICAgICAgdHlwZT1cInBhc3N3b3JkXCJcbiAgICAgICAgICAgICAgbmFtZT1cInBhc3N3b3JkXCJcbiAgICAgICAgICAgICAgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sXCJcbiAgICAgICAgICAgICAgcGxhY2Vob2xkZXI9XCJQYXNzd29yZFwiIC8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwIGdydi1cIj5cbiAgICAgICAgICAgIDxpbnB1dFxuICAgICAgICAgICAgICB2YWx1ZUxpbms9e3RoaXMubGlua1N0YXRlKCdwc3dDb25maXJtZWQnKX1cbiAgICAgICAgICAgICAgdHlwZT1cInBhc3N3b3JkXCJcbiAgICAgICAgICAgICAgbmFtZT1cInBhc3N3b3JkQ29uZmlybWVkXCJcbiAgICAgICAgICAgICAgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sXCJcbiAgICAgICAgICAgICAgcGxhY2Vob2xkZXI9XCJQYXNzd29yZCBjb25maXJtXCIvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0XG4gICAgICAgICAgICAgIG5hbWU9XCJ0b2tlblwiXG4gICAgICAgICAgICAgIHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3Rva2VuJyl9XG4gICAgICAgICAgICAgIGNsYXNzTmFtZT1cImZvcm0tY29udHJvbCByZXF1aXJlZFwiXG4gICAgICAgICAgICAgIHBsYWNlaG9sZGVyPVwiVHdvIGZhY3RvciB0b2tlbiAoR29vZ2xlIEF1dGhlbnRpY2F0b3IpXCIgLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8YnV0dG9uIHR5cGU9XCJzdWJtaXRcIiBkaXNhYmxlZD17dGhpcy5wcm9wcy5hdHRlbXAuaXNQcm9jZXNzaW5nfSBjbGFzc05hbWU9XCJidG4gYnRuLXByaW1hcnkgYmxvY2sgZnVsbC13aWR0aCBtLWJcIiBvbkNsaWNrPXt0aGlzLm9uQ2xpY2t9ID5TaWduIHVwPC9idXR0b24+XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9mb3JtPlxuICAgICk7XG4gIH1cbn0pXG5cbnZhciBJbnZpdGUgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIGludml0ZTogZ2V0dGVycy5pbnZpdGUsXG4gICAgICBhdHRlbXA6IGdldHRlcnMuYXR0ZW1wXG4gICAgfVxuICB9LFxuXG4gIGNvbXBvbmVudERpZE1vdW50KCl7XG4gICAgYWN0aW9ucy5mZXRjaEludml0ZSh0aGlzLnByb3BzLnBhcmFtcy5pbnZpdGVUb2tlbik7XG4gIH0sXG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICBpZighdGhpcy5zdGF0ZS5pbnZpdGUpIHtcbiAgICAgIHJldHVybiBudWxsO1xuICAgIH1cblxuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1pbnZpdGUgdGV4dC1jZW50ZXJcIj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbG9nby10cHJ0XCI+PC9kaXY+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWNvbnRlbnQgZ3J2LWZsZXhcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LWNvbHVtblwiPlxuICAgICAgICAgICAgPEludml0ZUlucHV0Rm9ybSBhdHRlbXA9e3RoaXMuc3RhdGUuYXR0ZW1wfSBpbnZpdGU9e3RoaXMuc3RhdGUuaW52aXRlLnRvSlMoKX0vPlxuICAgICAgICAgICAgPEdvb2dsZUF1dGhJbmZvLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LWNvbHVtblwiPlxuICAgICAgICAgICAgPGg0PlNjYW4gYmFyIGNvZGUgZm9yIGF1dGggdG9rZW4gPGJyLz4gPHNtYWxsPlNjYW4gYmVsb3cgdG8gZ2VuZXJhdGUgeW91ciB0d28gZmFjdG9yIHRva2VuPC9zbWFsbD48L2g0PlxuICAgICAgICAgICAgPGltZyBjbGFzc05hbWU9XCJpbWctdGh1bWJuYWlsXCIgc3JjPXsgYGRhdGE6aW1hZ2UvcG5nO2Jhc2U2NCwke3RoaXMuc3RhdGUuaW52aXRlLmdldCgncXInKX1gIH0gLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBJbnZpdGU7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9uZXdVc2VyLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIge2dldHRlcnMsIGFjdGlvbnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvbm9kZXMnKTtcbnZhciB1c2VyR2V0dGVycyA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3VzZXIvZ2V0dGVycycpO1xudmFyIHtUYWJsZSwgQ29sdW1uLCBDZWxsfSA9IHJlcXVpcmUoJ2FwcC9jb21wb25lbnRzL3RhYmxlLmpzeCcpO1xudmFyIHtvcGVufSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGlvbnMnKTtcblxuY29uc3QgVGV4dENlbGwgPSAoe3Jvd0luZGV4LCBkYXRhLCBjb2x1bW5LZXksIC4uLnByb3BzfSkgPT4gKFxuICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgIHtkYXRhW3Jvd0luZGV4XVtjb2x1bW5LZXldfVxuICA8L0NlbGw+XG4pO1xuXG5jb25zdCBUYWdDZWxsID0gKHtyb3dJbmRleCwgZGF0YSwgY29sdW1uS2V5LCAuLi5wcm9wc30pID0+IChcbiAgPENlbGwgey4uLnByb3BzfT5cbiAgICB7IGRhdGFbcm93SW5kZXhdLnRhZ3MubWFwKChpdGVtLCBpbmRleCkgPT5cbiAgICAgICg8c3BhbiBrZXk9e2luZGV4fSBjbGFzc05hbWU9XCJsYWJlbCBsYWJlbC1kZWZhdWx0XCI+XG4gICAgICAgIHtpdGVtLnJvbGV9IDxsaSBjbGFzc05hbWU9XCJmYSBmYS1sb25nLWFycm93LXJpZ2h0XCI+PC9saT5cbiAgICAgICAge2l0ZW0udmFsdWV9XG4gICAgICA8L3NwYW4+KVxuICAgICkgfVxuICA8L0NlbGw+XG4pO1xuXG5jb25zdCBMb2dpbkNlbGwgPSAoe3VzZXIsIHJvd0luZGV4LCBkYXRhLCAuLi5wcm9wc30pID0+IHtcbiAgaWYoIXVzZXIgfHwgdXNlci5sb2dpbnMubGVuZ3RoID09PSAwKXtcbiAgICByZXR1cm4gPENlbGwgey4uLnByb3BzfSAvPjtcbiAgfVxuXG4gIHZhciAkbGlzID0gW107XG5cbiAgZm9yKHZhciBpID0gMDsgaSA8IHVzZXIubG9naW5zLmxlbmd0aDsgaSsrKXtcbiAgICAkbGlzLnB1c2goPGxpIGtleT17aX0+PGEgaHJlZj1cIiNcIiB0YXJnZXQ9XCJfYmxhbmtcIiBvbkNsaWNrPXtvcGVuLmJpbmQobnVsbCwgZGF0YVtyb3dJbmRleF0uYWRkciwgdXNlci5sb2dpbnNbaV0sIHVuZGVmaW5lZCl9Pnt1c2VyLmxvZ2luc1tpXX08L2E+PC9saT4pO1xuICB9XG5cbiAgcmV0dXJuIChcbiAgICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJidG4tZ3JvdXBcIj5cbiAgICAgICAgPGJ1dHRvbiB0eXBlPVwiYnV0dG9uXCIgb25DbGljaz17b3Blbi5iaW5kKG51bGwsIGRhdGFbcm93SW5kZXhdLmFkZHIsIHVzZXIubG9naW5zWzBdLCB1bmRlZmluZWQpfSBjbGFzc05hbWU9XCJidG4gYnRuLXNtIGJ0bi1wcmltYXJ5XCI+e3VzZXIubG9naW5zWzBdfTwvYnV0dG9uPlxuICAgICAgICB7XG4gICAgICAgICAgJGxpcy5sZW5ndGggPiAxID8gKFxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJidG4tZ3JvdXBcIj5cbiAgICAgICAgICAgICAgPGJ1dHRvbiBkYXRhLXRvZ2dsZT1cImRyb3Bkb3duXCIgY2xhc3NOYW1lPVwiYnRuIGJ0bi1kZWZhdWx0IGJ0bi1zbSBkcm9wZG93bi10b2dnbGVcIiBhcmlhLWV4cGFuZGVkPVwidHJ1ZVwiPlxuICAgICAgICAgICAgICAgIDxzcGFuIGNsYXNzTmFtZT1cImNhcmV0XCI+PC9zcGFuPlxuICAgICAgICAgICAgICA8L2J1dHRvbj5cbiAgICAgICAgICAgICAgPHVsIGNsYXNzTmFtZT1cImRyb3Bkb3duLW1lbnVcIj5cbiAgICAgICAgICAgICAgICA8bGk+PGEgaHJlZj1cIiNcIiB0YXJnZXQ9XCJfYmxhbmtcIj5Mb2dzPC9hPjwvbGk+XG4gICAgICAgICAgICAgICAgPGxpPjxhIGhyZWY9XCIjXCIgdGFyZ2V0PVwiX2JsYW5rXCI+TG9nczwvYT48L2xpPlxuICAgICAgICAgICAgICA8L3VsPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgKTogbnVsbFxuICAgICAgICB9XG4gICAgICA8L2Rpdj5cbiAgICA8L0NlbGw+XG4gIClcbn07XG5cbnZhciBOb2RlcyA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBtaXhpbnM6IFtyZWFjdG9yLlJlYWN0TWl4aW5dLFxuXG4gIGdldERhdGFCaW5kaW5ncygpIHtcbiAgICByZXR1cm4ge1xuICAgICAgbm9kZVJlY29yZHM6IGdldHRlcnMubm9kZUxpc3RWaWV3LFxuICAgICAgdXNlcjogdXNlckdldHRlcnMudXNlclxuICAgIH1cbiAgfSxcbiAgICBcbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICB2YXIgZGF0YSA9IHRoaXMuc3RhdGUubm9kZVJlY29yZHM7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LW5vZGVzXCI+XG4gICAgICAgIDxoMT4gTm9kZXMgPC9oMT5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICAgICAgPFRhYmxlIHJvd0NvdW50PXtkYXRhLmxlbmd0aH0gY2xhc3NOYW1lPVwidGFibGUtc3RyaXBwZWQgZ3J2LW5vZGVzLXRhYmxlXCI+XG4gICAgICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICAgICAgY29sdW1uS2V5PVwic2Vzc2lvbkNvdW50XCJcbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+IFNlc3Npb25zIDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17PFRleHRDZWxsIGRhdGE9e2RhdGF9Lz4gfVxuICAgICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICAgICAgY29sdW1uS2V5PVwiYWRkclwiXG4gICAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPiBOb2RlIDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17PFRleHRDZWxsIGRhdGE9e2RhdGF9Lz4gfVxuICAgICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICAgICAgY29sdW1uS2V5PVwidGFnc1wiXG4gICAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPjwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17PFRhZ0NlbGwgZGF0YT17ZGF0YX0vPiB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJyb2xlc1wiXG4gICAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPkxvZ2luIGFzPC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXs8TG9naW5DZWxsIGRhdGE9e2RhdGF9IHVzZXI9e3RoaXMuc3RhdGUudXNlcn0vPiB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgPC9UYWJsZT5cbiAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgIClcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gTm9kZXM7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9ub2Rlcy9tYWluLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgTm9kZXMgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXY+XG4gICAgICAgIDxoMT4gU2Vzc2lvbnMhPC9oMT5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICAgICAgPHRhYmxlIGNsYXNzTmFtZT1cInRhYmxlIHRhYmxlLXN0cmlwZWRcIj5cbiAgICAgICAgICAgICAgICA8dGhlYWQ+XG4gICAgICAgICAgICAgICAgICA8dHI+XG4gICAgICAgICAgICAgICAgICAgIDx0aD5Ob2RlPC90aD5cbiAgICAgICAgICAgICAgICAgICAgPHRoPlN0YXR1czwvdGg+XG4gICAgICAgICAgICAgICAgICAgIDx0aD5MYWJlbHM8L3RoPlxuICAgICAgICAgICAgICAgICAgICAgIDx0aD5DUFU8L3RoPlxuICAgICAgICAgICAgICAgICAgICAgIDx0aD5SQU08L3RoPlxuICAgICAgICAgICAgICAgICAgICAgIDx0aD5PUzwvdGg+XG4gICAgICAgICAgICAgICAgICAgICAgPHRoPiBMYXN0IEhlYXJ0YmVhdCA8L3RoPlxuICAgICAgICAgICAgICAgICAgICA8L3RyPlxuICAgICAgICAgICAgICAgICAgPC90aGVhZD5cbiAgICAgICAgICAgICAgICA8dGJvZHk+PC90Ym9keT5cbiAgICAgICAgICAgICAgPC90YWJsZT5cbiAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgIClcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gTm9kZXM7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9zZXNzaW9ucy9tYWluLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG5cbnZhciBHcnZUYWJsZUNlbGwgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIHJlbmRlcigpe1xuICAgIHZhciBwcm9wcyA9IHRoaXMucHJvcHM7XG4gICAgcmV0dXJuIHByb3BzLmlzSGVhZGVyID8gPHRoIGtleT17cHJvcHMua2V5fT57cHJvcHMuY2hpbGRyZW59PC90aD4gOiA8dGQga2V5PXtwcm9wcy5rZXl9Pntwcm9wcy5jaGlsZHJlbn08L3RkPjtcbiAgfVxufSk7XG5cbnZhciBHcnZUYWJsZSA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICByZW5kZXJIZWFkZXIoY2hpbGRyZW4pe1xuICAgIHZhciBjZWxscyA9IGNoaWxkcmVuLm1hcCgoaXRlbSwgaW5kZXgpPT57XG4gICAgICByZXR1cm4gdGhpcy5yZW5kZXJDZWxsKGl0ZW0ucHJvcHMuaGVhZGVyLCB7aW5kZXgsIGtleTogaW5kZXgsIGlzSGVhZGVyOiB0cnVlLCAuLi5pdGVtLnByb3BzfSk7XG4gICAgfSlcblxuICAgIHJldHVybiA8dGhlYWQ+PHRyPntjZWxsc308L3RyPjwvdGhlYWQ+XG4gIH0sXG5cbiAgcmVuZGVyQm9keShjaGlsZHJlbil7XG4gICAgdmFyIGNvdW50ID0gdGhpcy5wcm9wcy5yb3dDb3VudDtcbiAgICB2YXIgcm93cyA9IFtdO1xuICAgIGZvcih2YXIgaSA9IDA7IGkgPCBjb3VudDsgaSArKyl7XG4gICAgICB2YXIgY2VsbHMgPSBjaGlsZHJlbi5tYXAoKGl0ZW0sIGluZGV4KT0+e1xuICAgICAgICByZXR1cm4gdGhpcy5yZW5kZXJDZWxsKGl0ZW0ucHJvcHMuY2VsbCwge3Jvd0luZGV4OiBpLCBrZXk6IGluZGV4LCBpc0hlYWRlcjogZmFsc2UsIC4uLml0ZW0ucHJvcHN9KTtcbiAgICAgIH0pXG5cbiAgICAgIHJvd3MucHVzaCg8dHIga2V5PXtpfT57Y2VsbHN9PC90cj4pO1xuICAgIH1cblxuICAgIHJldHVybiA8dGJvZHk+e3Jvd3N9PC90Ym9keT47XG4gIH0sXG5cbiAgcmVuZGVyQ2VsbChjZWxsLCBjZWxsUHJvcHMpe1xuICAgIHZhciBjb250ZW50ID0gbnVsbDtcbiAgICBpZiAoUmVhY3QuaXNWYWxpZEVsZW1lbnQoY2VsbCkpIHtcbiAgICAgICBjb250ZW50ID0gUmVhY3QuY2xvbmVFbGVtZW50KGNlbGwsIGNlbGxQcm9wcyk7XG4gICAgIH0gZWxzZSBpZiAodHlwZW9mIHByb3BzLmNlbGwgPT09ICdmdW5jdGlvbicpIHtcbiAgICAgICBjb250ZW50ID0gY2VsbChjZWxsUHJvcHMpO1xuICAgICB9XG5cbiAgICAgcmV0dXJuIGNvbnRlbnQ7XG4gIH0sXG5cbiAgcmVuZGVyKCkge1xuICAgIHZhciBjaGlsZHJlbiA9IFtdO1xuICAgIFJlYWN0LkNoaWxkcmVuLmZvckVhY2godGhpcy5wcm9wcy5jaGlsZHJlbiwgKGNoaWxkLCBpbmRleCkgPT4ge1xuICAgICAgaWYgKGNoaWxkID09IG51bGwpIHtcbiAgICAgICAgcmV0dXJuO1xuICAgICAgfVxuXG4gICAgICBpZihjaGlsZC50eXBlLmRpc3BsYXlOYW1lICE9PSAnR3J2VGFibGVDb2x1bW4nKXtcbiAgICAgICAgdGhyb3cgJ1Nob3VsZCBiZSBHcnZUYWJsZUNvbHVtbic7XG4gICAgICB9XG5cbiAgICAgIGNoaWxkcmVuLnB1c2goY2hpbGQpO1xuICAgIH0pO1xuXG4gICAgdmFyIHRhYmxlQ2xhc3MgPSAndGFibGUgJyArIHRoaXMucHJvcHMuY2xhc3NOYW1lO1xuXG4gICAgcmV0dXJuIChcbiAgICAgIDx0YWJsZSBjbGFzc05hbWU9e3RhYmxlQ2xhc3N9PlxuICAgICAgICB7dGhpcy5yZW5kZXJIZWFkZXIoY2hpbGRyZW4pfVxuICAgICAgICB7dGhpcy5yZW5kZXJCb2R5KGNoaWxkcmVuKX1cbiAgICAgIDwvdGFibGU+XG4gICAgKTtcbiAgfVxufSlcblxudmFyIEdydlRhYmxlQ29sdW1uID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIHRocm93IG5ldyBFcnJvcignQ29tcG9uZW50IDxHcnZUYWJsZUNvbHVtbiAvPiBzaG91bGQgbmV2ZXIgcmVuZGVyJyk7XG4gIH1cbn0pXG5cbmV4cG9ydCBkZWZhdWx0IEdydlRhYmxlO1xuZXhwb3J0IHtHcnZUYWJsZUNvbHVtbiBhcyBDb2x1bW4sIEdydlRhYmxlIGFzIFRhYmxlLCBHcnZUYWJsZUNlbGwgYXMgQ2VsbH07XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy90YWJsZS5qc3hcbiAqKi8iLCJ2YXIgc2Vzc2lvbiA9IHJlcXVpcmUoJ2FwcC9zZXNzaW9uJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xudmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciB7Z2V0dGVycywgYWN0aW9uc30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC8nKTtcbnZhciBFdmVudFN0cmVhbWVyID0gcmVxdWlyZSgnLi9ldmVudFN0cmVhbWVyLmpzeCcpO1xuXG52YXIgVGVybWluYWxIb3N0ID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW3JlYWN0b3IuUmVhY3RNaXhpbl0sXG5cbiAgb25PcGVuKCl7XG4gICAgYWN0aW9ucy5jb25uZWN0ZWQoKTtcbiAgfSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIGFjdGl2ZVNlc3Npb246IGdldHRlcnMuYWN0aXZlU2Vzc2lvblxuICAgIH1cbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIGlmKCF0aGlzLnN0YXRlLmFjdGl2ZVNlc3Npb24pe1xuICAgICAgcmV0dXJuIG51bGw7XG4gICAgfVxuXG4gICAgdmFyIHtpc0Nvbm5lY3RlZCwgLi4uc2V0dGluZ3N9ID0gdGhpcy5zdGF0ZS5hY3RpdmVTZXNzaW9uO1xuICAgIHZhciB7dG9rZW59ID0gc2Vzc2lvbi5nZXRVc2VyRGF0YSgpO1xuXG4gICAgcmV0dXJuIChcbiAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtdGVybWluYWwtaG9zdFwiPlxuICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXRlcm1pbmFsLXBhcnRpY2lwYW5zXCI+XG4gICAgICAgICA8dWwgY2xhc3NOYW1lPVwibmF2XCI+XG4gICAgICAgICAgIDxsaT48YnV0dG9uIGNsYXNzTmFtZT1cImJ0biBidG4tcHJpbWFyeSBidG4tY2lyY2xlXCIgdHlwZT1cImJ1dHRvblwiPiA8c3Ryb25nPkE8L3N0cm9uZz48L2J1dHRvbj48L2xpPlxuICAgICAgICAgICA8bGk+PGJ1dHRvbiBjbGFzc05hbWU9XCJidG4gYnRuLXByaW1hcnkgYnRuLWNpcmNsZVwiIHR5cGU9XCJidXR0b25cIj4gQiA8L2J1dHRvbj48L2xpPlxuICAgICAgICAgICA8bGk+PGJ1dHRvbiBjbGFzc05hbWU9XCJidG4gYnRuLXByaW1hcnkgYnRuLWNpcmNsZVwiIHR5cGU9XCJidXR0b25cIj4gQyA8L2J1dHRvbj48L2xpPlxuICAgICAgICAgICA8bGk+XG4gICAgICAgICAgICAgPGJ1dHRvbiBvbkNsaWNrPXthY3Rpb25zLmNsb3NlfSBjbGFzc05hbWU9XCJidG4gYnRuLWRhbmdlciBidG4tY2lyY2xlXCIgdHlwZT1cImJ1dHRvblwiPlxuICAgICAgICAgICAgICAgPGkgY2xhc3NOYW1lPVwiZmEgZmEtdGltZXNcIj48L2k+XG4gICAgICAgICAgICAgPC9idXR0b24+XG4gICAgICAgICAgIDwvbGk+XG4gICAgICAgICA8L3VsPlxuICAgICAgIDwvZGl2PlxuICAgICAgIDxkaXY+XG4gICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImJ0bi1ncm91cFwiPlxuICAgICAgICAgICA8c3BhbiBjbGFzc05hbWU9XCJidG4gYnRuLXhzIGJ0bi1wcmltYXJ5XCI+MTI4LjAuMC4xOjg4ODg8L3NwYW4+XG4gICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiYnRuLWdyb3VwXCI+XG4gICAgICAgICAgICAgPGJ1dHRvbiBkYXRhLXRvZ2dsZT1cImRyb3Bkb3duXCIgY2xhc3NOYW1lPVwiYnRuIGJ0bi1kZWZhdWx0IGJ0bi14cyBkcm9wZG93bi10b2dnbGVcIiBhcmlhLWV4cGFuZGVkPVwidHJ1ZVwiPlxuICAgICAgICAgICAgICAgPHNwYW4gY2xhc3NOYW1lPVwiY2FyZXRcIj48L3NwYW4+XG4gICAgICAgICAgICAgPC9idXR0b24+XG4gICAgICAgICAgICAgPHVsIGNsYXNzTmFtZT1cImRyb3Bkb3duLW1lbnVcIj5cbiAgICAgICAgICAgICAgIDxsaT48YSBocmVmPVwiI1wiIHRhcmdldD1cIl9ibGFua1wiPkxvZ3M8L2E+PC9saT5cbiAgICAgICAgICAgICAgIDxsaT48YSBocmVmPVwiI1wiIHRhcmdldD1cIl9ibGFua1wiPkxvZ3M8L2E+PC9saT5cbiAgICAgICAgICAgICA8L3VsPlxuICAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgIDwvZGl2PlxuICAgICAgIDwvZGl2PlxuICAgICAgIHsgaXNDb25uZWN0ZWQgPyA8RXZlbnRTdHJlYW1lciB0b2tlbj17dG9rZW59IHNpZD17c2V0dGluZ3Muc2lkfS8+IDogbnVsbCB9XG4gICAgICAgPFRlcm1pbmFsQm94IHNldHRpbmdzPXtzZXR0aW5nc30gdG9rZW49e3Rva2VufSBvbk9wZW49e2FjdGlvbnMuY29ubmVjdGVkfS8+XG4gICAgIDwvZGl2PlxuICAgICApO1xuICB9XG59KTtcblxudmFyIFRlcm1pbmFsQm94ID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICByZW5kZXJUZXJtaW5hbDogZnVuY3Rpb24oKSB7XG4gICAgdmFyIGlzTmV3XG4gICAgdmFyIHBhcmVudCA9IGRvY3VtZW50LmdldEVsZW1lbnRCeUlkKFwidGVybWluYWwtYm94XCIpO1xuXG4gICAgdmFyIHtzZXR0aW5ncywgdG9rZW4sIHNpZCB9ID0gdGhpcy5wcm9wcztcblxuICAgIC8vc2V0dGluZ3Muc2lkID0gNTU1NTtcbiAgICBzZXR0aW5ncy50ZXJtID0ge1xuICAgICAgaDogMTIwLFxuICAgICAgdzogMTAwXG4gICAgfTtcblxuICAgIHZhciBjb25uZWN0aW9uU3RyID0gY2ZnLmFwaS5nZXRTZXNzaW9uQ29ublN0cih0b2tlbiwgc2V0dGluZ3MpO1xuXG4gICAgdGhpcy50ZXJtID0gbmV3IFRlcm1pbmFsKHtcbiAgICAgIGNvbHM6IDE4MCxcbiAgICAgIHJvd3M6IDUwLFxuICAgICAgdXNlU3R5bGU6IHRydWUsXG4gICAgICBzY3JlZW5LZXlzOiB0cnVlLFxuICAgICAgY3Vyc29yQmxpbms6IGZhbHNlXG4gICAgfSk7XG5cbiAgICB0aGlzLnRlcm0ub3BlbihwYXJlbnQpO1xuICAgIHRoaXMuc29ja2V0ID0gbmV3IFdlYlNvY2tldChjb25uZWN0aW9uU3RyLCBcInByb3RvXCIpO1xuICAgIHRoaXMudGVybS53cml0ZSgnXFx4MWJbOTRtY29ubmVjdGluZyB0byBcInBvZFwiXFx4MWJbbVxcclxcbicpO1xuXG4gICAgdGhpcy5zb2NrZXQub25vcGVuID0gKCkgPT4ge1xuICAgICAgdGhpcy5wcm9wcy5vbk9wZW4oKTtcbiAgICAgIHRoaXMudGVybS5vbignZGF0YScsIChkYXRhKSA9PiB7XG4gICAgICAgIHRoaXMuc29ja2V0LnNlbmQoZGF0YSk7XG4gICAgICB9KTtcblxuICAgICAgdGhpcy5zb2NrZXQub25tZXNzYWdlID0gKGUpID0+IHtcbiAgICAgICAgdGhpcy50ZXJtLndyaXRlKGUuZGF0YSk7XG4gICAgICB9XG5cbiAgICAgIHRoaXMuc29ja2V0Lm9uY2xvc2UgPSAoKSA9PiB7XG4gICAgICAgIHRoaXMudGVybS53cml0ZSgnXFx4MWJbMzFtZGlzY29ubmVjdGVkXFx4MWJbbVxcclxcbicpO1xuICAgICAgfVxuICAgIH1cbiAgfSxcblxuICBjb21wb25lbnREaWRNb3VudDogZnVuY3Rpb24oKSB7XG4gICAgdGhpcy5yZW5kZXJUZXJtaW5hbCgpO1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50OiBmdW5jdGlvbigpIHtcbiAgICB0aGlzLnNvY2tldC5jbG9zZSgpO1xuICAgIHRoaXMudGVybS5kZXN0cm95KCk7XG4gIH0sXG5cbiAgc2hvdWxkQ29tcG9uZW50VXBkYXRlOiBmdW5jdGlvbigpIHtcbiAgICByZXR1cm4gZmFsc2U7XG4gIH0sXG5cbiAgY29tcG9uZW50V2lsbFJlY2VpdmVQcm9wczogZnVuY3Rpb24ocHJvcHMpIHtcbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIHJldHVybiAoXG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXdpei10ZXJtaW5hbFwiIGlkPVwidGVybWluYWwtYm94XCI+XG4gICAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pO1xuXG5leHBvcnQgZGVmYXVsdCBUZXJtaW5hbEhvc3Q7XG5leHBvcnQge1Rlcm1pbmFsQm94LCBUZXJtaW5hbEhvc3R9O1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvdGVybWluYWxIb3N0LmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgcmVuZGVyID0gcmVxdWlyZSgncmVhY3QtZG9tJykucmVuZGVyO1xudmFyIHsgUm91dGVyLCBSb3V0ZSwgUmVkaXJlY3QsIEluZGV4Um91dGUsIGJyb3dzZXJIaXN0b3J5IH0gPSByZXF1aXJlKCdyZWFjdC1yb3V0ZXInKTtcbnZhciB7IEFwcCwgTG9naW4sIE5vZGVzLCBTZXNzaW9ucywgTmV3VXNlciB9ID0gcmVxdWlyZSgnLi9jb21wb25lbnRzJyk7XG52YXIge2Vuc3VyZVVzZXJ9ID0gcmVxdWlyZSgnLi9tb2R1bGVzL3VzZXIvYWN0aW9ucycpO1xudmFyIGF1dGggPSByZXF1aXJlKCcuL2F1dGgnKTtcbnZhciBzZXNzaW9uID0gcmVxdWlyZSgnLi9zZXNzaW9uJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnLi9jb25maWcnKTtcblxucmVxdWlyZSgnLi9tb2R1bGVzJyk7XG5cbi8vIGluaXQgc2Vzc2lvblxuc2Vzc2lvbi5pbml0KCk7XG5cbmZ1bmN0aW9uIGhhbmRsZUxvZ291dChuZXh0U3RhdGUsIHJlcGxhY2UsIGNiKXtcbiAgYXV0aC5sb2dvdXQoKTtcbiAgLy8gZ29pbmcgYmFjayB3aWxsIGhpdCByZXF1aXJlQXV0aCBoYW5kbGVyIHdoaWNoIHdpbGwgcmVkaXJlY3QgaXQgdG8gdGhlIGxvZ2luIHBhZ2VcbiAgc2Vzc2lvbi5nZXRIaXN0b3J5KCkuZ29CYWNrKCk7XG59XG5cbnJlbmRlcigoXG4gIDxSb3V0ZXIgaGlzdG9yeT17c2Vzc2lvbi5nZXRIaXN0b3J5KCl9PlxuICAgIDxSb3V0ZSBwYXRoPXtjZmcucm91dGVzLmxvZ2lufSBjb21wb25lbnQ9e0xvZ2lufS8+XG4gICAgPFJvdXRlIHBhdGg9e2NmZy5yb3V0ZXMubG9nb3V0fSBvbkVudGVyPXtoYW5kbGVMb2dvdXR9Lz5cbiAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5uZXdVc2VyfSBjb21wb25lbnQ9e05ld1VzZXJ9Lz5cbiAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5hcHB9IGNvbXBvbmVudD17QXBwfSBvbkVudGVyPXtlbnN1cmVVc2VyfSA+XG4gICAgICA8SW5kZXhSb3V0ZSBjb21wb25lbnQ9e05vZGVzfS8+XG4gICAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5ub2Rlc30gY29tcG9uZW50PXtOb2Rlc30vPlxuICAgICAgPFJvdXRlIHBhdGg9e2NmZy5yb3V0ZXMuc2Vzc2lvbnN9IGNvbXBvbmVudD17U2Vzc2lvbnN9Lz5cbiAgICA8L1JvdXRlPlxuICA8L1JvdXRlcj5cbiksIGRvY3VtZW50LmdldEVsZW1lbnRCeUlkKFwiYXBwXCIpKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9pbmRleC5qc3hcbiAqKi8iXSwic291cmNlUm9vdCI6IiJ9