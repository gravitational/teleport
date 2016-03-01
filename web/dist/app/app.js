webpackJsonp([1],{

/***/ 0:
/***/ function(module, exports, __webpack_require__) {

	module.exports = __webpack_require__(200);


/***/ },

/***/ 9:
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

/***/ 14:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(152);
	
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
	    activeSession: '/web/active-session/:sid',
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

/***/ 25:
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

/***/ 30:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var _require = __webpack_require__(43);
	
	var browserHistory = _require.browserHistory;
	var createMemoryHistory = _require.createMemoryHistory;
	
	var AUTH_KEY_DATA = 'authData';
	
	var _history = createMemoryHistory();
	
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

/***/ 39:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var $ = __webpack_require__(52);
	var session = __webpack_require__(30);
	
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

/***/ 52:
/***/ function(module, exports) {

	module.exports = jQuery;

/***/ },

/***/ 53:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(9);
	
	var _require = __webpack_require__(167);
	
	var uuid = _require.uuid;
	
	var _require2 = __webpack_require__(86);
	
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

/***/ 54:
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

/***/ 55:
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

/***/ 85:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var api = __webpack_require__(39);
	var session = __webpack_require__(30);
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
	    session.clear();
	    session.getHistory().replace({ pathname: cfg.routes.login });
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

/***/ 86:
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

/***/ 87:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };
	
	var _require = __webpack_require__(18);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(86);
	
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

/***/ 88:
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

/***/ 89:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(18);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(88);
	
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

/***/ 90:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(18);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(54);
	
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

/***/ 91:
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

/***/ 92:
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

/***/ 93:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(18);
	
	var toImmutable = _require.toImmutable;
	
	var reactor = __webpack_require__(9);
	
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
	
	var sessionsView = [['tlpt_sessions'], function (sessions) {
	  return sessions.valueSeq().map(function (item) {
	    var sid = item.get('id');
	    var parties = reactor.evaluate(partiesBySessionId(sid));
	    return {
	      sid: sid,
	      addr: parties[0].addr,
	      parties: parties
	    };
	  }).toJS();
	}];
	
	var partiesBySessionId = function partiesBySessionId(sid) {
	  return [['tlpt_sessions', sid, 'parties'], function (parties) {
	
	    if (!parties) {
	      return [];
	    }
	
	    var lastActiveUsrName = getLastActiveUser(parties).get('user');
	
	    return parties.map(function (item) {
	      var user = item.get('user');
	      return {
	        user: item.get('user'),
	        addr: item.get('server_addr'),
	        isActive: lastActiveUsrName === user
	      };
	    }).toJS();
	  }];
	};
	
	function getLastActiveUser(parties) {
	  return parties.sortBy(function (item) {
	    return new Date(item.get('lastActive'));
	  }).first();
	}
	
	exports['default'] = {
	  partiesBySessionId: partiesBySessionId,
	  sessionsByServer: sessionsByServer,
	  sessionsView: sessionsView
	};
	module.exports = exports['default'];

/***/ },

/***/ 94:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(18);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(55);
	
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

/***/ 95:
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

/***/ 96:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(9);
	
	var _require = __webpack_require__(95);
	
	var TLPT_RECEIVE_USER = _require.TLPT_RECEIVE_USER;
	
	var _require2 = __webpack_require__(92);
	
	var TRYING_TO_SIGN_UP = _require2.TRYING_TO_SIGN_UP;
	
	var restApiActions = __webpack_require__(163);
	var auth = __webpack_require__(85);
	var session = __webpack_require__(30);
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

/***/ 97:
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

/***/ 98:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(97);
	module.exports.actions = __webpack_require__(96);
	module.exports.nodeStore = __webpack_require__(99);

/***/ },

/***/ 99:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(18);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(95);
	
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

/***/ 111:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	exports.__esModule = true;
	
	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }
	
	var session = __webpack_require__(30);
	var cfg = __webpack_require__(14);
	var React = __webpack_require__(5);
	
	var _require = __webpack_require__(155);
	
	var getters = _require.getters;
	var actions = _require.actions;
	
	var EventStreamer = __webpack_require__(192);
	
	var _require2 = __webpack_require__(285);
	
	var debounce = _require2.debounce;
	
	var ActiveSession = React.createClass({
	  displayName: 'ActiveSession',
	
	  mixins: [reactor.ReactMixin],
	
	  componentDidMount: function componentDidMount() {
	    //actions.open
	    //actions.open(data[rowIndex].addr, user.logins[i], undefined)}>{user.logins[i]}</a></li>);
	  },
	
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
	      isConnected ? React.createElement(EventStreamer, { sid: settings.sid }) : null,
	      React.createElement(TerminalBox, { settings: settings, onOpen: actions.connected })
	    );
	  }
	});
	
	Terminal.colors[256] = 'inherit';
	
	var TerminalBox = React.createClass({
	  displayName: 'TerminalBox',
	
	  resize: function resize(e) {
	    this.destroy();
	    this.renderTerminal();
	  },
	
	  connect: function connect(cols, rows) {
	    var _this = this;
	
	    var _session$getUserData = session.getUserData();
	
	    var token = _session$getUserData.token;
	    var _props = this.props;
	    var settings = _props.settings;
	    var sid = _props.sid;
	
	    settings.term = {
	      h: rows,
	      w: cols
	    };
	
	    var connectionStr = cfg.api.getSessionConnStr(token, settings);
	
	    this.socket = new WebSocket(connectionStr, 'proto');
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
	
	  getDimensions: function getDimensions() {
	    var $container = $(this.refs.container);
	    var $term = $container.find('.terminal');
	    var cols, rows;
	
	    if ($term.length === 1) {
	      var fakeRow = $('<div><span>&nbsp;</span></div>');
	      $term.append(fakeRow);
	      var fakeCol = fakeRow.children().first()[0].getBoundingClientRect();
	      cols = Math.floor($term.width() / (fakeCol.width || 9));
	      rows = Math.floor($term.height() / (fakeCol.height || 20));
	      fakeRow.remove();
	    } else {
	      var $container = $(this.refs.container);
	      cols = Math.floor($container.width() / 9);
	      rows = Math.floor($container.height() / 20);
	    }
	
	    return { cols: cols, rows: rows };
	  },
	
	  destroy: function destroy() {
	    if (this.socket) {
	      this.socket.close();
	    }
	
	    this.term.destroy();
	  },
	
	  renderTerminal: function renderTerminal() {
	    var _getDimensions = this.getDimensions();
	
	    var cols = _getDimensions.cols;
	    var rows = _getDimensions.rows;
	    var _props2 = this.props;
	    var settings = _props2.settings;
	    var token = _props2.token;
	    var sid = _props2.sid;
	
	    this.term = new Terminal({
	      cols: cols,
	      rows: rows,
	      useStyle: true,
	      screenKeys: true,
	      cursorBlink: true
	    });
	
	    this.term.open(this.refs.container);
	    this.term.write('\x1b[94mconnecting...\x1b[m\r\n');
	    this.connect(cols, rows);
	  },
	
	  componentDidMount: function componentDidMount() {
	    this.renderTerminal();
	    this.resize = debounce(this.resize, 100);
	    window.addEventListener('resize', this.resize);
	  },
	
	  componentWillUnmount: function componentWillUnmount() {
	    this.destroy();
	    window.removeEventListener('resize', this.resize);
	  },
	
	  shouldComponentUpdate: function shouldComponentUpdate() {
	    return false;
	  },
	
	  render: function render() {
	    return React.createElement('div', { className: 'grv-terminal', id: 'terminal-box', ref: 'container' });
	  }
	});
	
	exports['default'] = ActiveSession;
	exports.TerminalBox = TerminalBox;
	exports.ActiveSession = ActiveSession;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "main.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 112:
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

/***/ 113:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	exports.__esModule = true;
	
	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };
	
	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }
	
	var React = __webpack_require__(5);
	
	var GrvTableTextCell = function GrvTableTextCell(_ref) {
	  var rowIndex = _ref.rowIndex;
	  var data = _ref.data;
	  var columnKey = _ref.columnKey;
	
	  var props = _objectWithoutProperties(_ref, ['rowIndex', 'data', 'columnKey']);
	
	  return React.createElement(
	    GrvTableCell,
	    props,
	    data[rowIndex][columnKey]
	  );
	};
	
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
	exports.TextCell = GrvTableTextCell;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "table.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 152:
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
	
	var _invariant = __webpack_require__(11);
	
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

/***/ 153:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(9);
	var api = __webpack_require__(39);
	var cfg = __webpack_require__(14);
	
	var _require = __webpack_require__(55);
	
	var TLPT_SESSINS_RECEIVE = _require.TLPT_SESSINS_RECEIVE;
	
	var _require2 = __webpack_require__(54);
	
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

/***/ 154:
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

/***/ 155:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(154);
	module.exports.actions = __webpack_require__(53);
	module.exports.activeTermStore = __webpack_require__(87);

/***/ },

/***/ 156:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var reactor = __webpack_require__(9);
	reactor.registerStores({
	  'tlpt_active_terminal': __webpack_require__(87),
	  'tlpt_user': __webpack_require__(99),
	  'tlpt_nodes': __webpack_require__(90),
	  'tlpt_invite': __webpack_require__(89),
	  'tlpt_rest_api': __webpack_require__(164),
	  'tlpt_sessions': __webpack_require__(94)
	});

/***/ },

/***/ 157:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(9);
	
	var _require = __webpack_require__(88);
	
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

/***/ 158:
/***/ function(module, exports, __webpack_require__) {

	/*eslint no-undef: 0,  no-unused-vars: 0, no-debugger:0*/
	
	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(92);
	
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

/***/ 159:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(158);
	module.exports.actions = __webpack_require__(157);
	module.exports.nodeStore = __webpack_require__(89);

/***/ },

/***/ 160:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(9);
	
	var _require = __webpack_require__(54);
	
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

/***/ 161:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(9);
	
	var _require = __webpack_require__(93);
	
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

/***/ 162:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(161);
	module.exports.actions = __webpack_require__(160);
	module.exports.nodeStore = __webpack_require__(90);
	
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

/***/ 163:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(9);
	
	var _require = __webpack_require__(91);
	
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

/***/ 164:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(18);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(91);
	
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

/***/ 165:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(9);
	
	var _require = __webpack_require__(55);
	
	var TLPT_SESSINS_RECEIVE = _require.TLPT_SESSINS_RECEIVE;
	var TLPT_SESSINS_UPDATE = _require.TLPT_SESSINS_UPDATE;
	exports['default'] = {
	  update: function update(json) {
	    reactor.dispatch(TLPT_SESSINS_UPDATE, json);
	  },
	
	  receive: function receive(json) {
	    reactor.dispatch(TLPT_SESSINS_RECEIVE, json);
	  }
	};
	module.exports = exports['default'];

/***/ },

/***/ 166:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(93);
	module.exports.actions = __webpack_require__(165);
	module.exports.activeTermStore = __webpack_require__(94);

/***/ },

/***/ 167:
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

/***/ 192:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	exports.__esModule = true;
	var cfg = __webpack_require__(14);
	var React = __webpack_require__(5);
	var session = __webpack_require__(30);
	
	var EventStreamer = React.createClass({
	  displayName: 'EventStreamer',
	
	  componentDidMount: function componentDidMount() {
	    var sid = this.props.sid;
	
	    var _session$getUserData = session.getUserData();
	
	    var token = _session$getUserData.token;
	
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

/***/ 193:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	var NavLeftBar = __webpack_require__(196);
	var cfg = __webpack_require__(14);
	var actions = __webpack_require__(153);
	
	var _require = __webpack_require__(111);
	
	var ActiveSession = _require.ActiveSession;
	
	var App = React.createClass({
	  displayName: 'App',
	
	  componentDidMount: function componentDidMount() {
	    actions.fetchNodesAndSessions();
	  },
	
	  render: function render() {
	    return React.createElement(
	      'div',
	      { className: 'grv-tlpt' },
	      React.createElement(NavLeftBar, null),
	      React.createElement(ActiveSession, null),
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

/***/ 194:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	module.exports.App = __webpack_require__(193);
	module.exports.Login = __webpack_require__(195);
	module.exports.NewUser = __webpack_require__(197);
	module.exports.Nodes = __webpack_require__(198);
	module.exports.Sessions = __webpack_require__(199);
	module.exports.ActiveSession = __webpack_require__(111);
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "index.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 195:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };
	
	var React = __webpack_require__(5);
	var $ = __webpack_require__(52);
	var reactor = __webpack_require__(9);
	var LinkedStateMixin = __webpack_require__(60);
	
	var _require = __webpack_require__(98);
	
	var actions = _require.actions;
	
	var GoogleAuthInfo = __webpack_require__(112);
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

/***/ 196:
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

/***/ 197:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	var $ = __webpack_require__(52);
	var reactor = __webpack_require__(9);
	
	var _require = __webpack_require__(159);
	
	var actions = _require.actions;
	var getters = _require.getters;
	
	var userModule = __webpack_require__(98);
	var LinkedStateMixin = __webpack_require__(60);
	var GoogleAuthInfo = __webpack_require__(112);
	
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

/***/ 198:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }
	
	var React = __webpack_require__(5);
	var reactor = __webpack_require__(9);
	
	var _require = __webpack_require__(162);
	
	var getters = _require.getters;
	var actions = _require.actions;
	
	var userGetters = __webpack_require__(97);
	
	var _require2 = __webpack_require__(113);
	
	var Table = _require2.Table;
	var Column = _require2.Column;
	var Cell = _require2.Cell;
	
	var _require3 = __webpack_require__(53);
	
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

/***/ 199:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }
	
	var React = __webpack_require__(5);
	var reactor = __webpack_require__(9);
	
	var _require = __webpack_require__(113);
	
	var Table = _require.Table;
	var Column = _require.Column;
	var Cell = _require.Cell;
	var TextCell = _require.TextCell;
	
	var _require2 = __webpack_require__(166);
	
	var getters = _require2.getters;
	
	var _require3 = __webpack_require__(53);
	
	var open = _require3.open;
	
	var UsersCell = function UsersCell(_ref) {
	  var rowIndex = _ref.rowIndex;
	  var data = _ref.data;
	
	  var props = _objectWithoutProperties(_ref, ['rowIndex', 'data']);
	
	  var $users = data[rowIndex].parties.map(function (item, itemIndex) {
	    return React.createElement(
	      'span',
	      { key: itemIndex, className: 'text-uppercase label label-primary' },
	      item.user[0]
	    );
	  });
	
	  return React.createElement(
	    Cell,
	    props,
	    React.createElement(
	      'div',
	      null,
	      $users
	    )
	  );
	};
	
	var ButtonCell = function ButtonCell(_ref2) {
	  var rowIndex = _ref2.rowIndex;
	  var data = _ref2.data;
	
	  var props = _objectWithoutProperties(_ref2, ['rowIndex', 'data']);
	
	  var onClick = function onClick() {
	    var rowData = data[rowIndex];
	    var sid = rowData.sid;
	    var addr = rowData.addr;
	
	    var user = rowData.parties[0].user;
	    open(addr, user, sid);
	  };
	
	  return React.createElement(
	    Cell,
	    props,
	    React.createElement(
	      'button',
	      { onClick: onClick, className: 'btn btn-info btn-circle', type: 'button' },
	      React.createElement('i', { className: 'fa fa-terminal' })
	    )
	  );
	};
	
	var SessionList = React.createClass({
	  displayName: 'SessionList',
	
	  mixins: [reactor.ReactMixin],
	
	  getDataBindings: function getDataBindings() {
	    return {
	      sessionsView: getters.sessionsView
	    };
	  },
	
	  render: function render() {
	    var data = this.state.sessionsView;
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
	              Table,
	              { rowCount: data.length, className: 'table-stripped' },
	              React.createElement(Column, {
	                columnKey: 'sid',
	                header: React.createElement(
	                  Cell,
	                  null,
	                  ' Session ID '
	                ),
	                cell: React.createElement(TextCell, { data: data })
	              }),
	              React.createElement(Column, {
	                header: React.createElement(
	                  Cell,
	                  null,
	                  ' '
	                ),
	                cell: React.createElement(ButtonCell, { data: data })
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
	                columnKey: 'addr',
	                header: React.createElement(
	                  Cell,
	                  null,
	                  ' Users '
	                ),
	                cell: React.createElement(UsersCell, { data: data })
	              })
	            )
	          )
	        )
	      )
	    );
	  }
	});
	
	module.exports = SessionList;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "main.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 200:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	var render = __webpack_require__(110).render;
	
	var _require = __webpack_require__(43);
	
	var Router = _require.Router;
	var Route = _require.Route;
	var Redirect = _require.Redirect;
	var IndexRoute = _require.IndexRoute;
	var browserHistory = _require.browserHistory;
	
	var _require2 = __webpack_require__(194);
	
	var App = _require2.App;
	var Login = _require2.Login;
	var Nodes = _require2.Nodes;
	var Sessions = _require2.Sessions;
	var NewUser = _require2.NewUser;
	var ActiveSession = _require2.ActiveSession;
	
	var _require3 = __webpack_require__(96);
	
	var ensureUser = _require3.ensureUser;
	
	var auth = __webpack_require__(85);
	var session = __webpack_require__(30);
	var cfg = __webpack_require__(14);
	
	__webpack_require__(156);
	
	// init session
	session.init();
	
	function handleLogout(nextState, replace, cb) {
	  auth.logout();
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

/***/ },

/***/ 285:
/***/ function(module, exports) {

	module.exports = _;

/***/ }

});
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJzb3VyY2VzIjpbIndlYnBhY2s6Ly8vLi9zcmMvYXBwL3JlYWN0b3IuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb25maWcuanMiLCJ3ZWJwYWNrOi8vLy4vfi9rZXltaXJyb3IvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9zZXNzaW9uLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvc2VydmljZXMvYXBpLmpzIiwid2VicGFjazovLy9leHRlcm5hbCBcImpRdWVyeVwiIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvYWN0aW9uVHlwZXMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9hdXRoLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvYWN0aXZlVGVybVN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvYWN0aW9uVHlwZXMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9pbnZpdGVTdG9yZS5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvbm9kZVN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2FjdGlvblR5cGVzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2NvbnN0YW50cy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvc2Vzc2lvblN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy91c2VyL2FjdGlvblR5cGVzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy91c2VyL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvdXNlci9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvdXNlci91c2VyU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2FjdGl2ZVNlc3Npb24vbWFpbi5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2dvb2dsZUF1dGguanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy90YWJsZS5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21tb24vcGF0dGVyblV0aWxzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9nZXR0ZXJzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9nZXR0ZXJzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL3Jlc3RBcGlTdG9yZS5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC91dGlscy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvYWN0aXZlU2Vzc2lvbi9ldmVudFN0cmVhbWVyLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvYXBwLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvaW5kZXguanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9sb2dpbi5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL25hdkxlZnRCYXIuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9uZXdVc2VyLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvbm9kZXMvbWFpbi5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL3Nlc3Npb25zL21haW4uanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvaW5kZXguanN4Iiwid2VicGFjazovLy9leHRlcm5hbCBcIl9cIiJdLCJuYW1lcyI6W10sIm1hcHBpbmdzIjoiOzs7Ozs7Ozs7Ozs7Ozs7OztzQ0FBd0IsRUFBWTs7QUFFcEMsS0FBTSxPQUFPLEdBQUcsdUJBQVk7QUFDMUIsUUFBSyxFQUFFLElBQUk7RUFDWixDQUFDOztBQUVGLE9BQU0sQ0FBQyxPQUFPLEdBQUcsT0FBTyxDQUFDOztzQkFFVixPQUFPOzs7Ozs7Ozs7Ozs7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QUksRUFBQyxPQUFPLEVBQUUsVUFBVztLQUMvRCxvQkFBQyxVQUFVLElBQUMsU0FBUyxFQUFFLEtBQU0sR0FBRTtLQUMvQixvQkFBQyxLQUFLLElBQUMsSUFBSSxFQUFFLEdBQUcsQ0FBQyxNQUFNLENBQUMsS0FBTSxFQUFDLFNBQVMsRUFBRSxLQUFNLEdBQUU7S0FDbEQsb0JBQUMsS0FBSyxJQUFDLElBQUksRUFBRSxHQUFHLENBQUMsTUFBTSxDQUFDLFFBQVMsRUFBQyxTQUFTLEVBQUUsUUFBUyxHQUFFO0lBQ2xEO0VBQ0QsRUFDUixRQUFRLENBQUMsY0FBYyxDQUFDLEtBQUssQ0FBQyxDQUFDLEM7Ozs7Ozs7OztBQzdCbEMsb0IiLCJmaWxlIjoiYXBwLmpzIiwic291cmNlc0NvbnRlbnQiOlsiaW1wb3J0IHsgUmVhY3RvciB9IGZyb20gJ251Y2xlYXItanMnXG5cbmNvbnN0IHJlYWN0b3IgPSBuZXcgUmVhY3Rvcih7XG4gIGRlYnVnOiB0cnVlXG59KVxuXG53aW5kb3cucmVhY3RvciA9IHJlYWN0b3I7XG5cbmV4cG9ydCBkZWZhdWx0IHJlYWN0b3JcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9yZWFjdG9yLmpzXG4gKiovIiwibGV0IHtmb3JtYXRQYXR0ZXJufSA9IHJlcXVpcmUoJ2FwcC9jb21tb24vcGF0dGVyblV0aWxzJyk7XG5cbmxldCBjZmcgPSB7XG5cbiAgYmFzZVVybDogd2luZG93LmxvY2F0aW9uLm9yaWdpbixcblxuICBhcGk6IHtcbiAgICByZW5ld1Rva2VuUGF0aDonL3YxL3dlYmFwaS9zZXNzaW9ucy9yZW5ldycsXG4gICAgbm9kZXNQYXRoOiAnL3YxL3dlYmFwaS9zaXRlcy8tY3VycmVudC0vbm9kZXMnLFxuICAgIHNlc3Npb25QYXRoOiAnL3YxL3dlYmFwaS9zZXNzaW9ucycsXG4gICAgaW52aXRlUGF0aDogJy92MS93ZWJhcGkvdXNlcnMvaW52aXRlcy86aW52aXRlVG9rZW4nLFxuICAgIGNyZWF0ZVVzZXJQYXRoOiAnL3YxL3dlYmFwaS91c2VycycsXG4gICAgZ2V0SW52aXRlVXJsOiAoaW52aXRlVG9rZW4pID0+IHtcbiAgICAgIHJldHVybiBmb3JtYXRQYXR0ZXJuKGNmZy5hcGkuaW52aXRlUGF0aCwge2ludml0ZVRva2VufSk7XG4gICAgfSxcblxuICAgIGdldEV2ZW50U3RyZWFtZXJDb25uU3RyOiAodG9rZW4sIHNpZCkgPT4ge1xuICAgICAgdmFyIGhvc3RuYW1lID0gZ2V0V3NIb3N0TmFtZSgpO1xuICAgICAgcmV0dXJuIGAke2hvc3RuYW1lfS92MS93ZWJhcGkvc2l0ZXMvLWN1cnJlbnQtL3Nlc3Npb25zLyR7c2lkfS9ldmVudHMvc3RyZWFtP2FjY2Vzc190b2tlbj0ke3Rva2VufWA7XG4gICAgfSxcblxuICAgIGdldFNlc3Npb25Db25uU3RyOiAodG9rZW4sIHBhcmFtcykgPT4ge1xuICAgICAgdmFyIGpzb24gPSBKU09OLnN0cmluZ2lmeShwYXJhbXMpO1xuICAgICAgdmFyIGpzb25FbmNvZGVkID0gd2luZG93LmVuY29kZVVSSShqc29uKTtcbiAgICAgIHZhciBob3N0bmFtZSA9IGdldFdzSG9zdE5hbWUoKTtcbiAgICAgIHJldHVybiBgJHtob3N0bmFtZX0vdjEvd2ViYXBpL3NpdGVzLy1jdXJyZW50LS9jb25uZWN0P2FjY2Vzc190b2tlbj0ke3Rva2VufSZwYXJhbXM9JHtqc29uRW5jb2RlZH1gO1xuICAgIH1cbiAgfSxcblxuICByb3V0ZXM6IHtcbiAgICBhcHA6ICcvd2ViJyxcbiAgICBsb2dvdXQ6ICcvd2ViL2xvZ291dCcsXG4gICAgbG9naW46ICcvd2ViL2xvZ2luJyxcbiAgICBub2RlczogJy93ZWIvbm9kZXMnLFxuICAgIGFjdGl2ZVNlc3Npb246ICcvd2ViL2FjdGl2ZS1zZXNzaW9uLzpzaWQnLFxuICAgIG5ld1VzZXI6ICcvd2ViL25ld3VzZXIvOmludml0ZVRva2VuJyxcbiAgICBzZXNzaW9uczogJy93ZWIvc2Vzc2lvbnMnXG4gIH1cblxufVxuXG5leHBvcnQgZGVmYXVsdCBjZmc7XG5cbmZ1bmN0aW9uIGdldFdzSG9zdE5hbWUoKXtcbiAgdmFyIHByZWZpeCA9IGxvY2F0aW9uLnByb3RvY29sID09IFwiaHR0cHM6XCI/XCJ3c3M6Ly9cIjpcIndzOi8vXCI7XG4gIHZhciBob3N0cG9ydCA9IGxvY2F0aW9uLmhvc3RuYW1lKyhsb2NhdGlvbi5wb3J0ID8gJzonK2xvY2F0aW9uLnBvcnQ6ICcnKTtcbiAgcmV0dXJuIGAke3ByZWZpeH0ke2hvc3Rwb3J0fWA7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29uZmlnLmpzXG4gKiovIiwiLyoqXG4gKiBDb3B5cmlnaHQgMjAxMy0yMDE0IEZhY2Vib29rLCBJbmMuXG4gKlxuICogTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbiAqIHlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbiAqIFlvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuICpcbiAqIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuICpcbiAqIFVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbiAqIGRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbiAqIFdJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuICogU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxuICogbGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4gKlxuICovXG5cblwidXNlIHN0cmljdFwiO1xuXG4vKipcbiAqIENvbnN0cnVjdHMgYW4gZW51bWVyYXRpb24gd2l0aCBrZXlzIGVxdWFsIHRvIHRoZWlyIHZhbHVlLlxuICpcbiAqIEZvciBleGFtcGxlOlxuICpcbiAqICAgdmFyIENPTE9SUyA9IGtleU1pcnJvcih7Ymx1ZTogbnVsbCwgcmVkOiBudWxsfSk7XG4gKiAgIHZhciBteUNvbG9yID0gQ09MT1JTLmJsdWU7XG4gKiAgIHZhciBpc0NvbG9yVmFsaWQgPSAhIUNPTE9SU1tteUNvbG9yXTtcbiAqXG4gKiBUaGUgbGFzdCBsaW5lIGNvdWxkIG5vdCBiZSBwZXJmb3JtZWQgaWYgdGhlIHZhbHVlcyBvZiB0aGUgZ2VuZXJhdGVkIGVudW0gd2VyZVxuICogbm90IGVxdWFsIHRvIHRoZWlyIGtleXMuXG4gKlxuICogICBJbnB1dDogIHtrZXkxOiB2YWwxLCBrZXkyOiB2YWwyfVxuICogICBPdXRwdXQ6IHtrZXkxOiBrZXkxLCBrZXkyOiBrZXkyfVxuICpcbiAqIEBwYXJhbSB7b2JqZWN0fSBvYmpcbiAqIEByZXR1cm4ge29iamVjdH1cbiAqL1xudmFyIGtleU1pcnJvciA9IGZ1bmN0aW9uKG9iaikge1xuICB2YXIgcmV0ID0ge307XG4gIHZhciBrZXk7XG4gIGlmICghKG9iaiBpbnN0YW5jZW9mIE9iamVjdCAmJiAhQXJyYXkuaXNBcnJheShvYmopKSkge1xuICAgIHRocm93IG5ldyBFcnJvcigna2V5TWlycm9yKC4uLik6IEFyZ3VtZW50IG11c3QgYmUgYW4gb2JqZWN0LicpO1xuICB9XG4gIGZvciAoa2V5IGluIG9iaikge1xuICAgIGlmICghb2JqLmhhc093blByb3BlcnR5KGtleSkpIHtcbiAgICAgIGNvbnRpbnVlO1xuICAgIH1cbiAgICByZXRba2V5XSA9IGtleTtcbiAgfVxuICByZXR1cm4gcmV0O1xufTtcblxubW9kdWxlLmV4cG9ydHMgPSBrZXlNaXJyb3I7XG5cblxuXG4vKioqKioqKioqKioqKioqKipcbiAqKiBXRUJQQUNLIEZPT1RFUlxuICoqIC4vfi9rZXltaXJyb3IvaW5kZXguanNcbiAqKiBtb2R1bGUgaWQgPSAyNVxuICoqIG1vZHVsZSBjaHVua3MgPSAxXG4gKiovIiwidmFyIHsgYnJvd3Nlckhpc3RvcnksIGNyZWF0ZU1lbW9yeUhpc3RvcnkgfSA9IHJlcXVpcmUoJ3JlYWN0LXJvdXRlcicpO1xuXG5jb25zdCBBVVRIX0tFWV9EQVRBID0gJ2F1dGhEYXRhJztcblxudmFyIF9oaXN0b3J5ID0gY3JlYXRlTWVtb3J5SGlzdG9yeSgpO1xuXG52YXIgc2Vzc2lvbiA9IHtcblxuICBpbml0KGhpc3Rvcnk9YnJvd3Nlckhpc3Rvcnkpe1xuICAgIF9oaXN0b3J5ID0gaGlzdG9yeTtcbiAgfSxcblxuICBnZXRIaXN0b3J5KCl7XG4gICAgcmV0dXJuIF9oaXN0b3J5O1xuICB9LFxuXG4gIHNldFVzZXJEYXRhKHVzZXJEYXRhKXtcbiAgICBsb2NhbFN0b3JhZ2Uuc2V0SXRlbShBVVRIX0tFWV9EQVRBLCBKU09OLnN0cmluZ2lmeSh1c2VyRGF0YSkpO1xuICB9LFxuXG4gIGdldFVzZXJEYXRhKCl7XG4gICAgdmFyIGl0ZW0gPSBsb2NhbFN0b3JhZ2UuZ2V0SXRlbShBVVRIX0tFWV9EQVRBKTtcbiAgICBpZihpdGVtKXtcbiAgICAgIHJldHVybiBKU09OLnBhcnNlKGl0ZW0pO1xuICAgIH1cblxuICAgIHJldHVybiB7fTtcbiAgfSxcblxuICBjbGVhcigpe1xuICAgIGxvY2FsU3RvcmFnZS5jbGVhcigpXG4gIH1cblxufVxuXG5tb2R1bGUuZXhwb3J0cyA9IHNlc3Npb247XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvc2Vzc2lvbi5qc1xuICoqLyIsInZhciAkID0gcmVxdWlyZShcImpRdWVyeVwiKTtcbnZhciBzZXNzaW9uID0gcmVxdWlyZSgnYXBwL3Nlc3Npb24nKTtcblxuY29uc3QgYXBpID0ge1xuXG4gIHBvc3QocGF0aCwgZGF0YSwgd2l0aFRva2VuKXtcbiAgICByZXR1cm4gYXBpLmFqYXgoe3VybDogcGF0aCwgZGF0YTogSlNPTi5zdHJpbmdpZnkoZGF0YSksIHR5cGU6ICdQT1NUJ30sIHdpdGhUb2tlbik7XG4gIH0sXG5cbiAgZ2V0KHBhdGgpe1xuICAgIHJldHVybiBhcGkuYWpheCh7dXJsOiBwYXRofSk7XG4gIH0sXG5cbiAgYWpheChjZmcsIHdpdGhUb2tlbiA9IHRydWUpe1xuICAgIHZhciBkZWZhdWx0Q2ZnID0ge1xuICAgICAgdHlwZTogXCJHRVRcIixcbiAgICAgIGRhdGFUeXBlOiBcImpzb25cIixcbiAgICAgIGJlZm9yZVNlbmQ6IGZ1bmN0aW9uKHhocikge1xuICAgICAgICBpZih3aXRoVG9rZW4pe1xuICAgICAgICAgIHZhciB7IHRva2VuIH0gPSBzZXNzaW9uLmdldFVzZXJEYXRhKCk7XG4gICAgICAgICAgeGhyLnNldFJlcXVlc3RIZWFkZXIoJ0F1dGhvcml6YXRpb24nLCdCZWFyZXIgJyArIHRva2VuKTtcbiAgICAgICAgfVxuICAgICAgIH1cbiAgICB9XG5cbiAgICByZXR1cm4gJC5hamF4KCQuZXh0ZW5kKHt9LCBkZWZhdWx0Q2ZnLCBjZmcpKTtcbiAgfVxufVxuXG5tb2R1bGUuZXhwb3J0cyA9IGFwaTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9zZXJ2aWNlcy9hcGkuanNcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cyA9IGpRdWVyeTtcblxuXG4vKioqKioqKioqKioqKioqKipcbiAqKiBXRUJQQUNLIEZPT1RFUlxuICoqIGV4dGVybmFsIFwialF1ZXJ5XCJcbiAqKiBtb2R1bGUgaWQgPSA1MlxuICoqIG1vZHVsZSBjaHVua3MgPSAxXG4gKiovIiwidmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHt1dWlkfSA9IHJlcXVpcmUoJ2FwcC91dGlscycpO1xudmFyIHsgVExQVF9URVJNX09QRU4sIFRMUFRfVEVSTV9DTE9TRSwgVExQVF9URVJNX0NPTk5FQ1RFRCwgVExQVF9URVJNX1JFQ0VJVkVfUEFSVElFUyB9ICA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuXG4gIGNsb3NlKCl7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1RFUk1fQ0xPU0UpO1xuICB9LFxuXG4gIGNvbm5lY3RlZCgpe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9URVJNX0NPTk5FQ1RFRCk7XG4gIH0sXG5cbiAgcmVjZWl2ZVBhcnRpZXMoanNvbil7XG4gICAgdmFyIHBhcnRpZXMgPSBqc29uLm1hcChpdGVtPT57XG4gICAgICByZXR1cm4ge1xuICAgICAgICB1c2VyOiBpdGVtLnVzZXIsXG4gICAgICAgIGxhc3RBY3RpdmU6IG5ldyBEYXRlKGl0ZW0ubGFzdF9hY3RpdmUpXG4gICAgICB9XG4gICAgfSlcblxuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9URVJNX1JFQ0VJVkVfUEFSVElFUywgcGFydGllcyk7XG4gIH0sXG5cbiAgb3BlbihhZGRyLCBsb2dpbiwgc2lkPXV1aWQoKSl7XG4gICAgLypcbiAgICAqICAge1xuICAgICogICBcImFkZHJcIjogXCIxMjcuMC4wLjE6NTAwMFwiLFxuICAgICogICBcImxvZ2luXCI6IFwiYWRtaW5cIixcbiAgICAqICAgXCJ0ZXJtXCI6IHtcImhcIjogMTIwLCBcIndcIjogMTAwfSxcbiAgICAqICAgXCJzaWRcIjogXCIxMjNcIlxuICAgICogIH1cbiAgICAqL1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9URVJNX09QRU4sIHthZGRyLCBsb2dpbiwgc2lkfSApO1xuICB9XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3Rpb25zLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfTk9ERVNfUkVDRUlWRTogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2FjdGlvblR5cGVzLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfU0VTU0lOU19SRUNFSVZFOiBudWxsLFxuICBUTFBUX1NFU1NJTlNfVVBEQVRFOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvYWN0aW9uVHlwZXMuanNcbiAqKi8iLCJ2YXIgYXBpID0gcmVxdWlyZSgnLi9zZXJ2aWNlcy9hcGknKTtcbnZhciBzZXNzaW9uID0gcmVxdWlyZSgnLi9zZXNzaW9uJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xudmFyICQgPSByZXF1aXJlKCdqUXVlcnknKTtcblxuY29uc3QgcmVmcmVzaFJhdGUgPSA2MDAwMCAqIDU7IC8vIDEgbWluXG5cbnZhciByZWZyZXNoVG9rZW5UaW1lcklkID0gbnVsbDtcblxudmFyIGF1dGggPSB7XG5cbiAgc2lnblVwKG5hbWUsIHBhc3N3b3JkLCB0b2tlbiwgaW52aXRlVG9rZW4pe1xuICAgIHZhciBkYXRhID0ge3VzZXI6IG5hbWUsIHBhc3M6IHBhc3N3b3JkLCBzZWNvbmRfZmFjdG9yX3Rva2VuOiB0b2tlbiwgaW52aXRlX3Rva2VuOiBpbnZpdGVUb2tlbn07XG4gICAgcmV0dXJuIGFwaS5wb3N0KGNmZy5hcGkuY3JlYXRlVXNlclBhdGgsIGRhdGEpXG4gICAgICAudGhlbigodXNlcik9PntcbiAgICAgICAgc2Vzc2lvbi5zZXRVc2VyRGF0YSh1c2VyKTtcbiAgICAgICAgYXV0aC5fc3RhcnRUb2tlblJlZnJlc2hlcigpO1xuICAgICAgICByZXR1cm4gdXNlcjtcbiAgICAgIH0pO1xuICB9LFxuXG4gIGxvZ2luKG5hbWUsIHBhc3N3b3JkLCB0b2tlbil7XG4gICAgYXV0aC5fc3RvcFRva2VuUmVmcmVzaGVyKCk7XG4gICAgcmV0dXJuIGF1dGguX2xvZ2luKG5hbWUsIHBhc3N3b3JkLCB0b2tlbikuZG9uZShhdXRoLl9zdGFydFRva2VuUmVmcmVzaGVyKTtcbiAgfSxcblxuICBlbnN1cmVVc2VyKCl7XG4gICAgdmFyIHVzZXJEYXRhID0gc2Vzc2lvbi5nZXRVc2VyRGF0YSgpO1xuICAgIGlmKHVzZXJEYXRhLnRva2VuKXtcbiAgICAgIC8vIHJlZnJlc2ggdGltZXIgd2lsbCBub3QgYmUgc2V0IGluIGNhc2Ugb2YgYnJvd3NlciByZWZyZXNoIGV2ZW50XG4gICAgICBpZihhdXRoLl9nZXRSZWZyZXNoVG9rZW5UaW1lcklkKCkgPT09IG51bGwpe1xuICAgICAgICByZXR1cm4gYXV0aC5fcmVmcmVzaFRva2VuKCkuZG9uZShhdXRoLl9zdGFydFRva2VuUmVmcmVzaGVyKTtcbiAgICAgIH1cblxuICAgICAgcmV0dXJuICQuRGVmZXJyZWQoKS5yZXNvbHZlKHVzZXJEYXRhKTtcbiAgICB9XG5cbiAgICByZXR1cm4gJC5EZWZlcnJlZCgpLnJlamVjdCgpO1xuICB9LFxuXG4gIGxvZ291dCgpe1xuICAgIGF1dGguX3N0b3BUb2tlblJlZnJlc2hlcigpO1xuICAgIHNlc3Npb24uY2xlYXIoKTtcbiAgICBzZXNzaW9uLmdldEhpc3RvcnkoKS5yZXBsYWNlKHtwYXRobmFtZTogY2ZnLnJvdXRlcy5sb2dpbn0pOyAgICBcbiAgfSxcblxuICBfc3RhcnRUb2tlblJlZnJlc2hlcigpe1xuICAgIHJlZnJlc2hUb2tlblRpbWVySWQgPSBzZXRJbnRlcnZhbChhdXRoLl9yZWZyZXNoVG9rZW4sIHJlZnJlc2hSYXRlKTtcbiAgfSxcblxuICBfc3RvcFRva2VuUmVmcmVzaGVyKCl7XG4gICAgY2xlYXJJbnRlcnZhbChyZWZyZXNoVG9rZW5UaW1lcklkKTtcbiAgICByZWZyZXNoVG9rZW5UaW1lcklkID0gbnVsbDtcbiAgfSxcblxuICBfZ2V0UmVmcmVzaFRva2VuVGltZXJJZCgpe1xuICAgIHJldHVybiByZWZyZXNoVG9rZW5UaW1lcklkO1xuICB9LFxuXG4gIF9yZWZyZXNoVG9rZW4oKXtcbiAgICByZXR1cm4gYXBpLnBvc3QoY2ZnLmFwaS5yZW5ld1Rva2VuUGF0aCkudGhlbihkYXRhPT57XG4gICAgICBzZXNzaW9uLnNldFVzZXJEYXRhKGRhdGEpO1xuICAgICAgcmV0dXJuIGRhdGE7XG4gICAgfSkuZmFpbCgoKT0+e1xuICAgICAgYXV0aC5sb2dvdXQoKTtcbiAgICB9KTtcbiAgfSxcblxuICBfbG9naW4obmFtZSwgcGFzc3dvcmQsIHRva2VuKXtcbiAgICB2YXIgZGF0YSA9IHtcbiAgICAgIHVzZXI6IG5hbWUsXG4gICAgICBwYXNzOiBwYXNzd29yZCxcbiAgICAgIHNlY29uZF9mYWN0b3JfdG9rZW46IHRva2VuXG4gICAgfTtcblxuICAgIHJldHVybiBhcGkucG9zdChjZmcuYXBpLnNlc3Npb25QYXRoLCBkYXRhLCBmYWxzZSkudGhlbihkYXRhPT57XG4gICAgICBzZXNzaW9uLnNldFVzZXJEYXRhKGRhdGEpO1xuICAgICAgcmV0dXJuIGRhdGE7XG4gICAgfSk7XG4gIH1cbn1cblxubW9kdWxlLmV4cG9ydHMgPSBhdXRoO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2F1dGguanNcbiAqKi8iLCJpbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVExQVF9URVJNX09QRU46IG51bGwsXG4gIFRMUFRfVEVSTV9DTE9TRTogbnVsbCxcbiAgVExQVF9URVJNX0NPTk5FQ1RFRDogbnVsbCxcbiAgVExQVF9URVJNX1JFQ0VJVkVfUEFSVElFUzogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGlvblR5cGVzLmpzXG4gKiovIiwidmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG52YXIgeyBUTFBUX1RFUk1fT1BFTiwgVExQVF9URVJNX0NMT1NFLCBUTFBUX1RFUk1fQ09OTkVDVEVELCBUTFBUX1RFUk1fUkVDRUlWRV9QQVJUSUVTIH0gID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gdG9JbW11dGFibGUobnVsbCk7XG4gIH0sXG5cbiAgaW5pdGlhbGl6ZSgpIHtcbiAgICB0aGlzLm9uKFRMUFRfVEVSTV9DT05ORUNURUQsIGNvbm5lY3RlZCk7XG4gICAgdGhpcy5vbihUTFBUX1RFUk1fT1BFTiwgc2V0QWN0aXZlVGVybWluYWwpO1xuICAgIHRoaXMub24oVExQVF9URVJNX0NMT1NFLCBjbG9zZSk7XG4gICAgdGhpcy5vbihUTFBUX1RFUk1fUkVDRUlWRV9QQVJUSUVTLCByZWNlaXZlUGFydGllcyk7XG4gIH1cblxufSlcblxuZnVuY3Rpb24gY2xvc2UoKXtcbiAgcmV0dXJuIHRvSW1tdXRhYmxlKG51bGwpO1xufVxuXG5mdW5jdGlvbiByZWNlaXZlUGFydGllcyhzdGF0ZSwgcGFydGllcyl7XG4gIHJldHVybiBzdGF0ZS5zZXQoJ3BhcnRpZXMnLCB0b0ltbXV0YWJsZShwYXJ0aWVzKSk7XG59XG5cbmZ1bmN0aW9uIHNldEFjdGl2ZVRlcm1pbmFsKHN0YXRlLCBzZXR0aW5ncyl7XG4gIHJldHVybiB0b0ltbXV0YWJsZSh7XG4gICAgICBwYXJ0aWVzOiBbXSxcbiAgICAgIGlzQ29ubmVjdGluZzogdHJ1ZSxcbiAgICAgIC4uLnNldHRpbmdzXG4gIH0pO1xufVxuXG5mdW5jdGlvbiBjb25uZWN0ZWQoc3RhdGUpe1xuICByZXR1cm4gc3RhdGUuc2V0KCdpc0Nvbm5lY3RlZCcsIHRydWUpXG4gICAgICAgICAgICAgIC5zZXQoJ2lzQ29ubmVjdGluZycsIGZhbHNlKTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGl2ZVRlcm1TdG9yZS5qc1xuICoqLyIsImltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX1JFQ0VJVkVfVVNFUl9JTlZJVEU6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvYWN0aW9uVHlwZXMuanNcbiAqKi8iLCJ2YXIgeyBTdG9yZSwgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcbnZhciAgeyBUTFBUX1JFQ0VJVkVfVVNFUl9JTlZJVEUgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKG51bGwpO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX1JFQ0VJVkVfVVNFUl9JTlZJVEUsIHJlY2VpdmVJbnZpdGUpXG4gIH1cbn0pXG5cbmZ1bmN0aW9uIHJlY2VpdmVJbnZpdGUoc3RhdGUsIGludml0ZSl7XG4gIHJldHVybiB0b0ltbXV0YWJsZShpbnZpdGUpO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2ludml0ZVN0b3JlLmpzXG4gKiovIiwidmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG52YXIgIHsgVExQVF9OT0RFU19SRUNFSVZFIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IFN0b3JlKHtcbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB0b0ltbXV0YWJsZShbXSk7XG4gIH0sXG5cbiAgaW5pdGlhbGl6ZSgpIHtcbiAgICB0aGlzLm9uKFRMUFRfTk9ERVNfUkVDRUlWRSwgcmVjZWl2ZU5vZGVzKVxuICB9XG59KVxuXG5mdW5jdGlvbiByZWNlaXZlTm9kZXMoc3RhdGUsIG5vZGVBcnJheSl7XG4gIHJldHVybiB0b0ltbXV0YWJsZShub2RlQXJyYXkpO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvbm9kZVN0b3JlLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfUkVTVF9BUElfU1RBUlQ6IG51bGwsXG4gIFRMUFRfUkVTVF9BUElfU1VDQ0VTUzogbnVsbCxcbiAgVExQVF9SRVNUX0FQSV9GQUlMOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9hY3Rpb25UeXBlcy5qc1xuICoqLyIsImltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUUllJTkdfVE9fU0lHTl9VUDogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvY29uc3RhbnRzLmpzXG4gKiovIiwidmFyIHsgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcblxuY29uc3Qgc2Vzc2lvbnNCeVNlcnZlciA9IChhZGRyKSA9PiBbWyd0bHB0X3Nlc3Npb25zJ10sIChzZXNzaW9ucykgPT57XG4gIHJldHVybiBzZXNzaW9ucy52YWx1ZVNlcSgpLmZpbHRlcihpdGVtPT57XG4gICAgdmFyIHBhcnRpZXMgPSBpdGVtLmdldCgncGFydGllcycpIHx8IHRvSW1tdXRhYmxlKFtdKTtcbiAgICB2YXIgaGFzU2VydmVyID0gcGFydGllcy5maW5kKGl0ZW0yPT4gaXRlbTIuZ2V0KCdzZXJ2ZXJfYWRkcicpID09PSBhZGRyKTtcbiAgICByZXR1cm4gaGFzU2VydmVyO1xuICB9KS50b0xpc3QoKTtcbn1dXG5cbmNvbnN0IHNlc3Npb25zVmlldyA9IFtbJ3RscHRfc2Vzc2lvbnMnXSwgKHNlc3Npb25zKSA9PntcbiAgcmV0dXJuIHNlc3Npb25zLnZhbHVlU2VxKCkubWFwKGl0ZW09PntcbiAgICB2YXIgc2lkID0gaXRlbS5nZXQoJ2lkJyk7XG4gICAgdmFyIHBhcnRpZXMgPSByZWFjdG9yLmV2YWx1YXRlKHBhcnRpZXNCeVNlc3Npb25JZChzaWQpKTtcbiAgICByZXR1cm4ge1xuICAgICAgc2lkOiBzaWQsXG4gICAgICBhZGRyOiBwYXJ0aWVzWzBdLmFkZHIsXG4gICAgICBwYXJ0aWVzOiBwYXJ0aWVzXG4gICAgfVxuICB9KS50b0pTKCk7XG59XTtcblxuY29uc3QgcGFydGllc0J5U2Vzc2lvbklkID0gKHNpZCkgPT5cbiBbWyd0bHB0X3Nlc3Npb25zJywgc2lkLCAncGFydGllcyddLCAocGFydGllcykgPT57XG5cbiAgaWYoIXBhcnRpZXMpe1xuICAgIHJldHVybiBbXTtcbiAgfVxuXG4gIHZhciBsYXN0QWN0aXZlVXNyTmFtZSA9IGdldExhc3RBY3RpdmVVc2VyKHBhcnRpZXMpLmdldCgndXNlcicpO1xuXG4gIHJldHVybiBwYXJ0aWVzLm1hcChpdGVtPT57XG4gICAgdmFyIHVzZXIgPSBpdGVtLmdldCgndXNlcicpO1xuICAgIHJldHVybiB7XG4gICAgICB1c2VyOiBpdGVtLmdldCgndXNlcicpLFxuICAgICAgYWRkcjogaXRlbS5nZXQoJ3NlcnZlcl9hZGRyJyksXG4gICAgICBpc0FjdGl2ZTogbGFzdEFjdGl2ZVVzck5hbWUgPT09IHVzZXJcbiAgICB9XG4gIH0pLnRvSlMoKTtcbn1dO1xuXG5mdW5jdGlvbiBnZXRMYXN0QWN0aXZlVXNlcihwYXJ0aWVzKXtcbiAgcmV0dXJuIHBhcnRpZXMuc29ydEJ5KGl0ZW09PiBuZXcgRGF0ZShpdGVtLmdldCgnbGFzdEFjdGl2ZScpKSkuZmlyc3QoKTtcbn1cblxuZXhwb3J0IGRlZmF1bHQge1xuICBwYXJ0aWVzQnlTZXNzaW9uSWQsXG4gIHNlc3Npb25zQnlTZXJ2ZXIsXG4gIHNlc3Npb25zVmlld1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvZ2V0dGVycy5qc1xuICoqLyIsInZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyIHsgVExQVF9TRVNTSU5TX1JFQ0VJVkUsIFRMUFRfU0VTU0lOU19VUERBVEUgfSAgPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IFN0b3JlKHtcbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB0b0ltbXV0YWJsZSh7fSk7XG4gIH0sXG5cbiAgaW5pdGlhbGl6ZSgpIHtcbiAgICB0aGlzLm9uKFRMUFRfU0VTU0lOU19SRUNFSVZFLCByZWNlaXZlU2Vzc2lvbnMpO1xuICAgIHRoaXMub24oVExQVF9TRVNTSU5TX1VQREFURSwgdXBkYXRlU2Vzc2lvbik7XG4gIH1cbn0pXG5cbmZ1bmN0aW9uIHVwZGF0ZVNlc3Npb24oc3RhdGUsIGpzb24pe1xuICByZXR1cm4gc3RhdGUuc2V0KGpzb24uaWQsIHRvSW1tdXRhYmxlKGpzb24pKTtcbn1cblxuZnVuY3Rpb24gcmVjZWl2ZVNlc3Npb25zKHN0YXRlLCBqc29uKXtcbiAgcmV0dXJuIHRvSW1tdXRhYmxlKGpzb24pO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvc2Vzc2lvblN0b3JlLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfUkVDRUlWRV9VU0VSOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvdXNlci9hY3Rpb25UeXBlcy5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7IFRMUFRfUkVDRUlWRV9VU0VSIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG52YXIgeyBUUllJTkdfVE9fU0lHTl9VUH0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9yZXN0QXBpL2NvbnN0YW50cycpO1xudmFyIHJlc3RBcGlBY3Rpb25zID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvcmVzdEFwaS9hY3Rpb25zJyk7XG52YXIgYXV0aCA9IHJlcXVpcmUoJ2FwcC9hdXRoJyk7XG52YXIgc2Vzc2lvbiA9IHJlcXVpcmUoJ2FwcC9zZXNzaW9uJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xuXG5leHBvcnQgZGVmYXVsdCB7XG5cbiAgZW5zdXJlVXNlcihuZXh0U3RhdGUsIHJlcGxhY2UsIGNiKXtcbiAgICBhdXRoLmVuc3VyZVVzZXIoKVxuICAgICAgLmRvbmUoKHVzZXJEYXRhKT0+IHsgICAgICAgIFxuICAgICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfUkVDRUlWRV9VU0VSLCB1c2VyRGF0YS51c2VyICk7XG4gICAgICAgIGNiKCk7XG4gICAgICB9KVxuICAgICAgLmZhaWwoKCk9PntcbiAgICAgICAgcmVwbGFjZSh7cmVkaXJlY3RUbzogbmV4dFN0YXRlLmxvY2F0aW9uLnBhdGhuYW1lIH0sIGNmZy5yb3V0ZXMubG9naW4pO1xuICAgICAgICBjYigpO1xuICAgICAgfSk7XG4gIH0sXG5cbiAgc2lnblVwKHtuYW1lLCBwc3csIHRva2VuLCBpbnZpdGVUb2tlbn0pe1xuICAgIHJlc3RBcGlBY3Rpb25zLnN0YXJ0KFRSWUlOR19UT19TSUdOX1VQKTtcbiAgICBhdXRoLnNpZ25VcChuYW1lLCBwc3csIHRva2VuLCBpbnZpdGVUb2tlbilcbiAgICAgIC5kb25lKChzZXNzaW9uRGF0YSk9PntcbiAgICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFQ0VJVkVfVVNFUiwgc2Vzc2lvbkRhdGEudXNlcik7XG4gICAgICAgIHJlc3RBcGlBY3Rpb25zLnN1Y2Nlc3MoVFJZSU5HX1RPX1NJR05fVVApO1xuICAgICAgICBzZXNzaW9uLmdldEhpc3RvcnkoKS5wdXNoKHtwYXRobmFtZTogY2ZnLnJvdXRlcy5hcHB9KTtcbiAgICAgIH0pXG4gICAgICAuZmFpbCgoKT0+e1xuICAgICAgICByZXN0QXBpQWN0aW9ucy5mYWlsKFRSWUlOR19UT19TSUdOX1VQLCAnZmFpbGVkIHRvIHNpbmcgdXAnKTtcbiAgICAgIH0pO1xuICB9LFxuXG4gIGxvZ2luKHt1c2VyLCBwYXNzd29yZCwgdG9rZW59LCByZWRpcmVjdCl7XG4gICAgICBhdXRoLmxvZ2luKHVzZXIsIHBhc3N3b3JkLCB0b2tlbilcbiAgICAgICAgLmRvbmUoKHNlc3Npb25EYXRhKT0+e1xuICAgICAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRUNFSVZFX1VTRVIsIHNlc3Npb25EYXRhLnVzZXIpO1xuICAgICAgICAgIHNlc3Npb24uZ2V0SGlzdG9yeSgpLnB1c2goe3BhdGhuYW1lOiByZWRpcmVjdH0pO1xuICAgICAgICB9KVxuICAgICAgICAuZmFpbCgoKT0+e1xuICAgICAgICB9KVxuICAgIH1cbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvYWN0aW9ucy5qc1xuICoqLyIsImNvbnN0IHVzZXIgPSBbIFsndGxwdF91c2VyJ10sIChjdXJyZW50VXNlcikgPT4ge1xuICAgIGlmKCFjdXJyZW50VXNlcil7XG4gICAgICByZXR1cm4gbnVsbDtcbiAgICB9XG4gICAgXG4gICAgcmV0dXJuIHtcbiAgICAgIG5hbWU6IGN1cnJlbnRVc2VyLmdldCgnbmFtZScpLFxuICAgICAgbG9naW5zOiBjdXJyZW50VXNlci5nZXQoJ2FsbG93ZWRfbG9naW5zJykudG9KUygpXG4gICAgfVxuICB9XG5dO1xuXG5leHBvcnQgZGVmYXVsdCB7XG4gIHVzZXJcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvZ2V0dGVycy5qc1xuICoqLyIsIm1vZHVsZS5leHBvcnRzLmdldHRlcnMgPSByZXF1aXJlKCcuL2dldHRlcnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGlvbnMgPSByZXF1aXJlKCcuL2FjdGlvbnMnKTtcbm1vZHVsZS5leHBvcnRzLm5vZGVTdG9yZSA9IHJlcXVpcmUoJy4vdXNlclN0b3JlJyk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy91c2VyL2luZGV4LmpzXG4gKiovIiwidmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG52YXIgIHsgVExQVF9SRUNFSVZFX1VTRVIgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKG51bGwpO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX1JFQ0VJVkVfVVNFUiwgcmVjZWl2ZVVzZXIpXG4gIH1cblxufSlcblxuZnVuY3Rpb24gcmVjZWl2ZVVzZXIoc3RhdGUsIHVzZXIpe1xuICByZXR1cm4gdG9JbW11dGFibGUodXNlcik7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy91c2VyL3VzZXJTdG9yZS5qc1xuICoqLyIsInZhciBzZXNzaW9uID0gcmVxdWlyZSgnYXBwL3Nlc3Npb24nKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHtnZXR0ZXJzLCBhY3Rpb25zfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsLycpO1xudmFyIEV2ZW50U3RyZWFtZXIgPSByZXF1aXJlKCcuL2V2ZW50U3RyZWFtZXIuanN4Jyk7XG52YXIge2RlYm91bmNlfSA9IHJlcXVpcmUoJ18nKTtcblxudmFyIEFjdGl2ZVNlc3Npb24gPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBjb21wb25lbnREaWRNb3VudDogZnVuY3Rpb24oKSB7XG4gICAgLy9hY3Rpb25zLm9wZW5cbiAgICAvL2FjdGlvbnMub3BlbihkYXRhW3Jvd0luZGV4XS5hZGRyLCB1c2VyLmxvZ2luc1tpXSwgdW5kZWZpbmVkKX0+e3VzZXIubG9naW5zW2ldfTwvYT48L2xpPik7XG4gIH0sXG5cbiAgb25PcGVuKCl7XG4gICAgYWN0aW9ucy5jb25uZWN0ZWQoKTtcbiAgfSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIGFjdGl2ZVNlc3Npb246IGdldHRlcnMuYWN0aXZlU2Vzc2lvblxuICAgIH1cbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIGlmKCF0aGlzLnN0YXRlLmFjdGl2ZVNlc3Npb24pe1xuICAgICAgcmV0dXJuIG51bGw7XG4gICAgfVxuXG4gICAgdmFyIHtpc0Nvbm5lY3RlZCwgLi4uc2V0dGluZ3N9ID0gdGhpcy5zdGF0ZS5hY3RpdmVTZXNzaW9uO1xuXG4gICAgcmV0dXJuIChcbiAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtdGVybWluYWwtaG9zdFwiPlxuICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXRlcm1pbmFsLXBhcnRpY2lwYW5zXCI+XG4gICAgICAgICA8dWwgY2xhc3NOYW1lPVwibmF2XCI+XG4gICAgICAgICAgIDxsaT48YnV0dG9uIGNsYXNzTmFtZT1cImJ0biBidG4tcHJpbWFyeSBidG4tY2lyY2xlXCIgdHlwZT1cImJ1dHRvblwiPiA8c3Ryb25nPkE8L3N0cm9uZz48L2J1dHRvbj48L2xpPlxuICAgICAgICAgICA8bGk+PGJ1dHRvbiBjbGFzc05hbWU9XCJidG4gYnRuLXByaW1hcnkgYnRuLWNpcmNsZVwiIHR5cGU9XCJidXR0b25cIj4gQiA8L2J1dHRvbj48L2xpPlxuICAgICAgICAgICA8bGk+PGJ1dHRvbiBjbGFzc05hbWU9XCJidG4gYnRuLXByaW1hcnkgYnRuLWNpcmNsZVwiIHR5cGU9XCJidXR0b25cIj4gQyA8L2J1dHRvbj48L2xpPlxuICAgICAgICAgICA8bGk+XG4gICAgICAgICAgICAgPGJ1dHRvbiBvbkNsaWNrPXthY3Rpb25zLmNsb3NlfSBjbGFzc05hbWU9XCJidG4gYnRuLWRhbmdlciBidG4tY2lyY2xlXCIgdHlwZT1cImJ1dHRvblwiPlxuICAgICAgICAgICAgICAgPGkgY2xhc3NOYW1lPVwiZmEgZmEtdGltZXNcIj48L2k+XG4gICAgICAgICAgICAgPC9idXR0b24+XG4gICAgICAgICAgIDwvbGk+XG4gICAgICAgICA8L3VsPlxuICAgICAgIDwvZGl2PlxuICAgICAgIDxkaXY+XG4gICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImJ0bi1ncm91cFwiPlxuICAgICAgICAgICA8c3BhbiBjbGFzc05hbWU9XCJidG4gYnRuLXhzIGJ0bi1wcmltYXJ5XCI+MTI4LjAuMC4xOjg4ODg8L3NwYW4+XG4gICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiYnRuLWdyb3VwXCI+XG4gICAgICAgICAgICAgPGJ1dHRvbiBkYXRhLXRvZ2dsZT1cImRyb3Bkb3duXCIgY2xhc3NOYW1lPVwiYnRuIGJ0bi1kZWZhdWx0IGJ0bi14cyBkcm9wZG93bi10b2dnbGVcIiBhcmlhLWV4cGFuZGVkPVwidHJ1ZVwiPlxuICAgICAgICAgICAgICAgPHNwYW4gY2xhc3NOYW1lPVwiY2FyZXRcIj48L3NwYW4+XG4gICAgICAgICAgICAgPC9idXR0b24+XG4gICAgICAgICAgICAgPHVsIGNsYXNzTmFtZT1cImRyb3Bkb3duLW1lbnVcIj5cbiAgICAgICAgICAgICAgIDxsaT48YSBocmVmPVwiI1wiIHRhcmdldD1cIl9ibGFua1wiPkxvZ3M8L2E+PC9saT5cbiAgICAgICAgICAgICAgIDxsaT48YSBocmVmPVwiI1wiIHRhcmdldD1cIl9ibGFua1wiPkxvZ3M8L2E+PC9saT5cbiAgICAgICAgICAgICA8L3VsPlxuICAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgIDwvZGl2PlxuICAgICAgIDwvZGl2PlxuICAgICAgIHsgaXNDb25uZWN0ZWQgPyA8RXZlbnRTdHJlYW1lciBzaWQ9e3NldHRpbmdzLnNpZH0vPiA6IG51bGwgfVxuICAgICAgIDxUZXJtaW5hbEJveCBzZXR0aW5ncz17c2V0dGluZ3N9IG9uT3Blbj17YWN0aW9ucy5jb25uZWN0ZWR9Lz5cbiAgICAgPC9kaXY+XG4gICAgICk7XG4gIH1cbn0pO1xuXG5UZXJtaW5hbC5jb2xvcnNbMjU2XSA9ICdpbmhlcml0JztcblxudmFyIFRlcm1pbmFsQm94ID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICByZXNpemU6IGZ1bmN0aW9uKGUpIHtcbiAgICB0aGlzLmRlc3Ryb3koKTtcbiAgICB0aGlzLnJlbmRlclRlcm1pbmFsKCk7XG4gIH0sXG5cbiAgY29ubmVjdChjb2xzLCByb3dzKXtcbiAgICBsZXQge3Rva2VufSA9IHNlc3Npb24uZ2V0VXNlckRhdGEoKTtcbiAgICBsZXQge3NldHRpbmdzLCBzaWQgfSA9IHRoaXMucHJvcHM7XG5cbiAgICBzZXR0aW5ncy50ZXJtID0ge1xuICAgICAgaDogcm93cyxcbiAgICAgIHc6IGNvbHNcbiAgICB9O1xuXG4gICAgbGV0IGNvbm5lY3Rpb25TdHIgPSBjZmcuYXBpLmdldFNlc3Npb25Db25uU3RyKHRva2VuLCBzZXR0aW5ncyk7XG5cbiAgICB0aGlzLnNvY2tldCA9IG5ldyBXZWJTb2NrZXQoY29ubmVjdGlvblN0ciwgJ3Byb3RvJyk7XG4gICAgdGhpcy5zb2NrZXQub25vcGVuID0gKCkgPT4ge1xuICAgICAgdGhpcy5wcm9wcy5vbk9wZW4oKTtcbiAgICAgIHRoaXMudGVybS5vbignZGF0YScsIChkYXRhKSA9PiB7XG4gICAgICAgIHRoaXMuc29ja2V0LnNlbmQoZGF0YSk7XG4gICAgICB9KTtcblxuICAgICAgdGhpcy5zb2NrZXQub25tZXNzYWdlID0gKGUpID0+IHtcbiAgICAgICAgdGhpcy50ZXJtLndyaXRlKGUuZGF0YSk7XG4gICAgICB9XG5cbiAgICAgIHRoaXMuc29ja2V0Lm9uY2xvc2UgPSAoKSA9PiB7XG4gICAgICAgIHRoaXMudGVybS53cml0ZSgnXFx4MWJbMzFtZGlzY29ubmVjdGVkXFx4MWJbbVxcclxcbicpO1xuICAgICAgfVxuICAgIH1cbiAgfSxcblxuICBnZXREaW1lbnNpb25zKCl7XG4gICAgdmFyICRjb250YWluZXIgPSAkKHRoaXMucmVmcy5jb250YWluZXIpO1xuICAgIHZhciAkdGVybSA9ICRjb250YWluZXIuZmluZCgnLnRlcm1pbmFsJyk7XG4gICAgdmFyIGNvbHMsIHJvd3M7XG5cbiAgICBpZigkdGVybS5sZW5ndGggPT09IDEpe1xuICAgICAgdmFyIGZha2VSb3cgPSAkKCc8ZGl2PjxzcGFuPiZuYnNwOzwvc3Bhbj48L2Rpdj4nKTtcbiAgICAgICR0ZXJtLmFwcGVuZChmYWtlUm93KTtcbiAgICAgIHZhciBmYWtlQ29sID0gZmFrZVJvdy5jaGlsZHJlbigpLmZpcnN0KClbMF0uZ2V0Qm91bmRpbmdDbGllbnRSZWN0KCk7XG4gICAgICBjb2xzID0gTWF0aC5mbG9vcigkdGVybS53aWR0aCgpIC8gKGZha2VDb2wud2lkdGggfHwgOSkpO1xuICAgICAgcm93cyA9IE1hdGguZmxvb3IoJHRlcm0uaGVpZ2h0KCkgLyAoZmFrZUNvbC5oZWlnaHQgfHwgMjApKTtcbiAgICAgIGZha2VSb3cucmVtb3ZlKCk7XG4gICAgfWVsc2V7XG4gICAgICB2YXIgJGNvbnRhaW5lciA9ICQodGhpcy5yZWZzLmNvbnRhaW5lcik7XG4gICAgICBjb2xzID0gTWF0aC5mbG9vcigkY29udGFpbmVyLndpZHRoKCkgLyA5KTtcbiAgICAgIHJvd3MgPSBNYXRoLmZsb29yKCRjb250YWluZXIuaGVpZ2h0KCkgLyAyMCk7XG4gICAgfVxuXG4gICAgcmV0dXJuIHtjb2xzLCByb3dzfTtcblxuICB9LFxuXG4gIGRlc3Ryb3koKXtcbiAgICBpZih0aGlzLnNvY2tldCl7XG4gICAgICB0aGlzLnNvY2tldC5jbG9zZSgpO1xuICAgIH1cblxuICAgIHRoaXMudGVybS5kZXN0cm95KCk7XG4gIH0sXG5cbiAgcmVuZGVyVGVybWluYWwoKSB7XG4gICAgdmFyIHtjb2xzLCByb3dzfSA9IHRoaXMuZ2V0RGltZW5zaW9ucygpO1xuICAgIHZhciB7c2V0dGluZ3MsIHRva2VuLCBzaWQgfSA9IHRoaXMucHJvcHM7XG5cbiAgICB0aGlzLnRlcm0gPSBuZXcgVGVybWluYWwoe1xuICAgICAgY29scyxcbiAgICAgIHJvd3MsXG4gICAgICB1c2VTdHlsZTogdHJ1ZSxcbiAgICAgIHNjcmVlbktleXM6IHRydWUsXG4gICAgICBjdXJzb3JCbGluazogdHJ1ZVxuICAgIH0pO1xuXG4gICAgdGhpcy50ZXJtLm9wZW4odGhpcy5yZWZzLmNvbnRhaW5lcik7XG4gICAgdGhpcy50ZXJtLndyaXRlKCdcXHgxYls5NG1jb25uZWN0aW5nLi4uXFx4MWJbbVxcclxcbicpO1xuICAgIHRoaXMuY29ubmVjdChjb2xzLCByb3dzKTtcbiAgfSxcblxuICBjb21wb25lbnREaWRNb3VudDogZnVuY3Rpb24oKSB7XG4gICAgdGhpcy5yZW5kZXJUZXJtaW5hbCgpO1xuICAgIHRoaXMucmVzaXplID0gZGVib3VuY2UodGhpcy5yZXNpemUsIDEwMCk7XG4gICAgd2luZG93LmFkZEV2ZW50TGlzdGVuZXIoJ3Jlc2l6ZScsIHRoaXMucmVzaXplKTtcbiAgfSxcblxuICBjb21wb25lbnRXaWxsVW5tb3VudDogZnVuY3Rpb24oKSB7XG4gICAgdGhpcy5kZXN0cm95KCk7XG4gICAgd2luZG93LnJlbW92ZUV2ZW50TGlzdGVuZXIoJ3Jlc2l6ZScsIHRoaXMucmVzaXplKTtcbiAgfSxcblxuICBzaG91bGRDb21wb25lbnRVcGRhdGU6IGZ1bmN0aW9uKCkge1xuICAgIHJldHVybiBmYWxzZTtcbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIHJldHVybiAoXG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXRlcm1pbmFsXCIgaWQ9XCJ0ZXJtaW5hbC1ib3hcIiByZWY9XCJjb250YWluZXJcIj5cbiAgICAgICAgPC9kaXY+XG4gICAgKTtcbiAgfVxufSk7XG5cbmV4cG9ydCBkZWZhdWx0IEFjdGl2ZVNlc3Npb247XG5leHBvcnQge1Rlcm1pbmFsQm94LCBBY3RpdmVTZXNzaW9ufTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2FjdGl2ZVNlc3Npb24vbWFpbi5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xuXG52YXIgR29vZ2xlQXV0aEluZm8gPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIHJlbmRlcigpIHtcbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZ29vZ2xlLWF1dGhcIj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZ29vZ2xlLWF1dGgtaWNvblwiPjwvZGl2PlxuICAgICAgICA8c3Ryb25nPkdvb2dsZSBBdXRoZW50aWNhdG9yPC9zdHJvbmc+XG4gICAgICAgIDxkaXY+RG93bmxvYWQgPGEgaHJlZj1cImh0dHBzOi8vc3VwcG9ydC5nb29nbGUuY29tL2FjY291bnRzL2Fuc3dlci8xMDY2NDQ3P2hsPWVuXCI+R29vZ2xlIEF1dGhlbnRpY2F0b3I8L2E+IG9uIHlvdXIgcGhvbmUgdG8gYWNjZXNzIHlvdXIgdHdvIGZhY3RvcnkgdG9rZW48L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pXG5cbm1vZHVsZS5leHBvcnRzID0gR29vZ2xlQXV0aEluZm87XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9nb29nbGVBdXRoLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG5cbmNvbnN0IEdydlRhYmxlVGV4dENlbGwgPSAoe3Jvd0luZGV4LCBkYXRhLCBjb2x1bW5LZXksIC4uLnByb3BzfSkgPT4gKFxuICA8R3J2VGFibGVDZWxsIHsuLi5wcm9wc30+XG4gICAge2RhdGFbcm93SW5kZXhdW2NvbHVtbktleV19XG4gIDwvR3J2VGFibGVDZWxsPlxuKTtcblxudmFyIEdydlRhYmxlQ2VsbCA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgcmVuZGVyKCl7XG4gICAgdmFyIHByb3BzID0gdGhpcy5wcm9wcztcbiAgICByZXR1cm4gcHJvcHMuaXNIZWFkZXIgPyA8dGgga2V5PXtwcm9wcy5rZXl9Pntwcm9wcy5jaGlsZHJlbn08L3RoPiA6IDx0ZCBrZXk9e3Byb3BzLmtleX0+e3Byb3BzLmNoaWxkcmVufTwvdGQ+O1xuICB9XG59KTtcblxudmFyIEdydlRhYmxlID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIHJlbmRlckhlYWRlcihjaGlsZHJlbil7XG4gICAgdmFyIGNlbGxzID0gY2hpbGRyZW4ubWFwKChpdGVtLCBpbmRleCk9PntcbiAgICAgIHJldHVybiB0aGlzLnJlbmRlckNlbGwoaXRlbS5wcm9wcy5oZWFkZXIsIHtpbmRleCwga2V5OiBpbmRleCwgaXNIZWFkZXI6IHRydWUsIC4uLml0ZW0ucHJvcHN9KTtcbiAgICB9KVxuXG4gICAgcmV0dXJuIDx0aGVhZD48dHI+e2NlbGxzfTwvdHI+PC90aGVhZD5cbiAgfSxcblxuICByZW5kZXJCb2R5KGNoaWxkcmVuKXtcbiAgICB2YXIgY291bnQgPSB0aGlzLnByb3BzLnJvd0NvdW50O1xuICAgIHZhciByb3dzID0gW107XG4gICAgZm9yKHZhciBpID0gMDsgaSA8IGNvdW50OyBpICsrKXtcbiAgICAgIHZhciBjZWxscyA9IGNoaWxkcmVuLm1hcCgoaXRlbSwgaW5kZXgpPT57XG4gICAgICAgIHJldHVybiB0aGlzLnJlbmRlckNlbGwoaXRlbS5wcm9wcy5jZWxsLCB7cm93SW5kZXg6IGksIGtleTogaW5kZXgsIGlzSGVhZGVyOiBmYWxzZSwgLi4uaXRlbS5wcm9wc30pO1xuICAgICAgfSlcblxuICAgICAgcm93cy5wdXNoKDx0ciBrZXk9e2l9PntjZWxsc308L3RyPik7XG4gICAgfVxuXG4gICAgcmV0dXJuIDx0Ym9keT57cm93c308L3Rib2R5PjtcbiAgfSxcblxuICByZW5kZXJDZWxsKGNlbGwsIGNlbGxQcm9wcyl7XG4gICAgdmFyIGNvbnRlbnQgPSBudWxsO1xuICAgIGlmIChSZWFjdC5pc1ZhbGlkRWxlbWVudChjZWxsKSkge1xuICAgICAgIGNvbnRlbnQgPSBSZWFjdC5jbG9uZUVsZW1lbnQoY2VsbCwgY2VsbFByb3BzKTtcbiAgICAgfSBlbHNlIGlmICh0eXBlb2YgcHJvcHMuY2VsbCA9PT0gJ2Z1bmN0aW9uJykge1xuICAgICAgIGNvbnRlbnQgPSBjZWxsKGNlbGxQcm9wcyk7XG4gICAgIH1cblxuICAgICByZXR1cm4gY29udGVudDtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgdmFyIGNoaWxkcmVuID0gW107XG4gICAgUmVhY3QuQ2hpbGRyZW4uZm9yRWFjaCh0aGlzLnByb3BzLmNoaWxkcmVuLCAoY2hpbGQsIGluZGV4KSA9PiB7XG4gICAgICBpZiAoY2hpbGQgPT0gbnVsbCkge1xuICAgICAgICByZXR1cm47XG4gICAgICB9XG5cbiAgICAgIGlmKGNoaWxkLnR5cGUuZGlzcGxheU5hbWUgIT09ICdHcnZUYWJsZUNvbHVtbicpe1xuICAgICAgICB0aHJvdyAnU2hvdWxkIGJlIEdydlRhYmxlQ29sdW1uJztcbiAgICAgIH1cblxuICAgICAgY2hpbGRyZW4ucHVzaChjaGlsZCk7XG4gICAgfSk7XG5cbiAgICB2YXIgdGFibGVDbGFzcyA9ICd0YWJsZSAnICsgdGhpcy5wcm9wcy5jbGFzc05hbWU7XG5cbiAgICByZXR1cm4gKFxuICAgICAgPHRhYmxlIGNsYXNzTmFtZT17dGFibGVDbGFzc30+XG4gICAgICAgIHt0aGlzLnJlbmRlckhlYWRlcihjaGlsZHJlbil9XG4gICAgICAgIHt0aGlzLnJlbmRlckJvZHkoY2hpbGRyZW4pfVxuICAgICAgPC90YWJsZT5cbiAgICApO1xuICB9XG59KVxuXG52YXIgR3J2VGFibGVDb2x1bW4gPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgdGhyb3cgbmV3IEVycm9yKCdDb21wb25lbnQgPEdydlRhYmxlQ29sdW1uIC8+IHNob3VsZCBuZXZlciByZW5kZXInKTtcbiAgfVxufSlcblxuZXhwb3J0IGRlZmF1bHQgR3J2VGFibGU7XG5leHBvcnQge0dydlRhYmxlQ29sdW1uIGFzIENvbHVtbiwgR3J2VGFibGUgYXMgVGFibGUsIEdydlRhYmxlQ2VsbCBhcyBDZWxsLCBHcnZUYWJsZVRleHRDZWxsIGFzIFRleHRDZWxsfTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL3RhYmxlLmpzeFxuICoqLyIsIi8qXG4gKiAgVGhlIE1JVCBMaWNlbnNlIChNSVQpXG4gKiAgQ29weXJpZ2h0IChjKSAyMDE1IFJ5YW4gRmxvcmVuY2UsIE1pY2hhZWwgSmFja3NvblxuICogIFBlcm1pc3Npb24gaXMgaGVyZWJ5IGdyYW50ZWQsIGZyZWUgb2YgY2hhcmdlLCB0byBhbnkgcGVyc29uIG9idGFpbmluZyBhIGNvcHkgb2YgdGhpcyBzb2Z0d2FyZSBhbmQgYXNzb2NpYXRlZCBkb2N1bWVudGF0aW9uIGZpbGVzICh0aGUgXCJTb2Z0d2FyZVwiKSwgdG8gZGVhbCBpbiB0aGUgU29mdHdhcmUgd2l0aG91dCByZXN0cmljdGlvbiwgaW5jbHVkaW5nIHdpdGhvdXQgbGltaXRhdGlvbiB0aGUgcmlnaHRzIHRvIHVzZSwgY29weSwgbW9kaWZ5LCBtZXJnZSwgcHVibGlzaCwgZGlzdHJpYnV0ZSwgc3VibGljZW5zZSwgYW5kL29yIHNlbGwgY29waWVzIG9mIHRoZSBTb2Z0d2FyZSwgYW5kIHRvIHBlcm1pdCBwZXJzb25zIHRvIHdob20gdGhlIFNvZnR3YXJlIGlzIGZ1cm5pc2hlZCB0byBkbyBzbywgc3ViamVjdCB0byB0aGUgZm9sbG93aW5nIGNvbmRpdGlvbnM6XG4gKiAgVGhlIGFib3ZlIGNvcHlyaWdodCBub3RpY2UgYW5kIHRoaXMgcGVybWlzc2lvbiBub3RpY2Ugc2hhbGwgYmUgaW5jbHVkZWQgaW4gYWxsIGNvcGllcyBvciBzdWJzdGFudGlhbCBwb3J0aW9ucyBvZiB0aGUgU29mdHdhcmUuXG4gKiAgVEhFIFNPRlRXQVJFIElTIFBST1ZJREVEIFwiQVMgSVNcIiwgV0lUSE9VVCBXQVJSQU5UWSBPRiBBTlkgS0lORCwgRVhQUkVTUyBPUiBJTVBMSUVELCBJTkNMVURJTkcgQlVUIE5PVCBMSU1JVEVEIFRPIFRIRSBXQVJSQU5USUVTIE9GIE1FUkNIQU5UQUJJTElUWSwgRklUTkVTUyBGT1IgQSBQQVJUSUNVTEFSIFBVUlBPU0UgQU5EIE5PTklORlJJTkdFTUVOVC4gSU4gTk8gRVZFTlQgU0hBTEwgVEhFIEFVVEhPUlMgT1IgQ09QWVJJR0hUIEhPTERFUlMgQkUgTElBQkxFIEZPUiBBTlkgQ0xBSU0sIERBTUFHRVMgT1IgT1RIRVIgTElBQklMSVRZLCBXSEVUSEVSIElOIEFOIEFDVElPTiBPRiBDT05UUkFDVCwgVE9SVCBPUiBPVEhFUldJU0UsIEFSSVNJTkcgRlJPTSwgT1VUIE9GIE9SIElOIENPTk5FQ1RJT04gV0lUSCBUSEUgU09GVFdBUkUgT1IgVEhFIFVTRSBPUiBPVEhFUiBERUFMSU5HUyBJTiBUSEUgU09GVFdBUkUuXG4qL1xuXG5pbXBvcnQgaW52YXJpYW50IGZyb20gJ2ludmFyaWFudCdcblxuZnVuY3Rpb24gZXNjYXBlUmVnRXhwKHN0cmluZykge1xuICByZXR1cm4gc3RyaW5nLnJlcGxhY2UoL1suKis/XiR7fSgpfFtcXF1cXFxcXS9nLCAnXFxcXCQmJylcbn1cblxuZnVuY3Rpb24gZXNjYXBlU291cmNlKHN0cmluZykge1xuICByZXR1cm4gZXNjYXBlUmVnRXhwKHN0cmluZykucmVwbGFjZSgvXFwvKy9nLCAnLysnKVxufVxuXG5mdW5jdGlvbiBfY29tcGlsZVBhdHRlcm4ocGF0dGVybikge1xuICBsZXQgcmVnZXhwU291cmNlID0gJyc7XG4gIGNvbnN0IHBhcmFtTmFtZXMgPSBbXTtcbiAgY29uc3QgdG9rZW5zID0gW107XG5cbiAgbGV0IG1hdGNoLCBsYXN0SW5kZXggPSAwLCBtYXRjaGVyID0gLzooW2EtekEtWl8kXVthLXpBLVowLTlfJF0qKXxcXCpcXCp8XFwqfFxcKHxcXCkvZ1xuICAvKmVzbGludCBuby1jb25kLWFzc2lnbjogMCovXG4gIHdoaWxlICgobWF0Y2ggPSBtYXRjaGVyLmV4ZWMocGF0dGVybikpKSB7XG4gICAgaWYgKG1hdGNoLmluZGV4ICE9PSBsYXN0SW5kZXgpIHtcbiAgICAgIHRva2Vucy5wdXNoKHBhdHRlcm4uc2xpY2UobGFzdEluZGV4LCBtYXRjaC5pbmRleCkpXG4gICAgICByZWdleHBTb3VyY2UgKz0gZXNjYXBlU291cmNlKHBhdHRlcm4uc2xpY2UobGFzdEluZGV4LCBtYXRjaC5pbmRleCkpXG4gICAgfVxuXG4gICAgaWYgKG1hdGNoWzFdKSB7XG4gICAgICByZWdleHBTb3VyY2UgKz0gJyhbXi8/I10rKSc7XG4gICAgICBwYXJhbU5hbWVzLnB1c2gobWF0Y2hbMV0pO1xuICAgIH0gZWxzZSBpZiAobWF0Y2hbMF0gPT09ICcqKicpIHtcbiAgICAgIHJlZ2V4cFNvdXJjZSArPSAnKFtcXFxcc1xcXFxTXSopJ1xuICAgICAgcGFyYW1OYW1lcy5wdXNoKCdzcGxhdCcpO1xuICAgIH0gZWxzZSBpZiAobWF0Y2hbMF0gPT09ICcqJykge1xuICAgICAgcmVnZXhwU291cmNlICs9ICcoW1xcXFxzXFxcXFNdKj8pJ1xuICAgICAgcGFyYW1OYW1lcy5wdXNoKCdzcGxhdCcpO1xuICAgIH0gZWxzZSBpZiAobWF0Y2hbMF0gPT09ICcoJykge1xuICAgICAgcmVnZXhwU291cmNlICs9ICcoPzonO1xuICAgIH0gZWxzZSBpZiAobWF0Y2hbMF0gPT09ICcpJykge1xuICAgICAgcmVnZXhwU291cmNlICs9ICcpPyc7XG4gICAgfVxuXG4gICAgdG9rZW5zLnB1c2gobWF0Y2hbMF0pO1xuXG4gICAgbGFzdEluZGV4ID0gbWF0Y2hlci5sYXN0SW5kZXg7XG4gIH1cblxuICBpZiAobGFzdEluZGV4ICE9PSBwYXR0ZXJuLmxlbmd0aCkge1xuICAgIHRva2Vucy5wdXNoKHBhdHRlcm4uc2xpY2UobGFzdEluZGV4LCBwYXR0ZXJuLmxlbmd0aCkpXG4gICAgcmVnZXhwU291cmNlICs9IGVzY2FwZVNvdXJjZShwYXR0ZXJuLnNsaWNlKGxhc3RJbmRleCwgcGF0dGVybi5sZW5ndGgpKVxuICB9XG5cbiAgcmV0dXJuIHtcbiAgICBwYXR0ZXJuLFxuICAgIHJlZ2V4cFNvdXJjZSxcbiAgICBwYXJhbU5hbWVzLFxuICAgIHRva2Vuc1xuICB9XG59XG5cbmNvbnN0IENvbXBpbGVkUGF0dGVybnNDYWNoZSA9IHt9XG5cbmV4cG9ydCBmdW5jdGlvbiBjb21waWxlUGF0dGVybihwYXR0ZXJuKSB7XG4gIGlmICghKHBhdHRlcm4gaW4gQ29tcGlsZWRQYXR0ZXJuc0NhY2hlKSlcbiAgICBDb21waWxlZFBhdHRlcm5zQ2FjaGVbcGF0dGVybl0gPSBfY29tcGlsZVBhdHRlcm4ocGF0dGVybilcblxuICByZXR1cm4gQ29tcGlsZWRQYXR0ZXJuc0NhY2hlW3BhdHRlcm5dXG59XG5cbi8qKlxuICogQXR0ZW1wdHMgdG8gbWF0Y2ggYSBwYXR0ZXJuIG9uIHRoZSBnaXZlbiBwYXRobmFtZS4gUGF0dGVybnMgbWF5IHVzZVxuICogdGhlIGZvbGxvd2luZyBzcGVjaWFsIGNoYXJhY3RlcnM6XG4gKlxuICogLSA6cGFyYW1OYW1lICAgICBNYXRjaGVzIGEgVVJMIHNlZ21lbnQgdXAgdG8gdGhlIG5leHQgLywgPywgb3IgIy4gVGhlXG4gKiAgICAgICAgICAgICAgICAgIGNhcHR1cmVkIHN0cmluZyBpcyBjb25zaWRlcmVkIGEgXCJwYXJhbVwiXG4gKiAtICgpICAgICAgICAgICAgIFdyYXBzIGEgc2VnbWVudCBvZiB0aGUgVVJMIHRoYXQgaXMgb3B0aW9uYWxcbiAqIC0gKiAgICAgICAgICAgICAgQ29uc3VtZXMgKG5vbi1ncmVlZHkpIGFsbCBjaGFyYWN0ZXJzIHVwIHRvIHRoZSBuZXh0XG4gKiAgICAgICAgICAgICAgICAgIGNoYXJhY3RlciBpbiB0aGUgcGF0dGVybiwgb3IgdG8gdGhlIGVuZCBvZiB0aGUgVVJMIGlmXG4gKiAgICAgICAgICAgICAgICAgIHRoZXJlIGlzIG5vbmVcbiAqIC0gKiogICAgICAgICAgICAgQ29uc3VtZXMgKGdyZWVkeSkgYWxsIGNoYXJhY3RlcnMgdXAgdG8gdGhlIG5leHQgY2hhcmFjdGVyXG4gKiAgICAgICAgICAgICAgICAgIGluIHRoZSBwYXR0ZXJuLCBvciB0byB0aGUgZW5kIG9mIHRoZSBVUkwgaWYgdGhlcmUgaXMgbm9uZVxuICpcbiAqIFRoZSByZXR1cm4gdmFsdWUgaXMgYW4gb2JqZWN0IHdpdGggdGhlIGZvbGxvd2luZyBwcm9wZXJ0aWVzOlxuICpcbiAqIC0gcmVtYWluaW5nUGF0aG5hbWVcbiAqIC0gcGFyYW1OYW1lc1xuICogLSBwYXJhbVZhbHVlc1xuICovXG5leHBvcnQgZnVuY3Rpb24gbWF0Y2hQYXR0ZXJuKHBhdHRlcm4sIHBhdGhuYW1lKSB7XG4gIC8vIE1ha2UgbGVhZGluZyBzbGFzaGVzIGNvbnNpc3RlbnQgYmV0d2VlbiBwYXR0ZXJuIGFuZCBwYXRobmFtZS5cbiAgaWYgKHBhdHRlcm4uY2hhckF0KDApICE9PSAnLycpIHtcbiAgICBwYXR0ZXJuID0gYC8ke3BhdHRlcm59YFxuICB9XG4gIGlmIChwYXRobmFtZS5jaGFyQXQoMCkgIT09ICcvJykge1xuICAgIHBhdGhuYW1lID0gYC8ke3BhdGhuYW1lfWBcbiAgfVxuXG4gIGxldCB7IHJlZ2V4cFNvdXJjZSwgcGFyYW1OYW1lcywgdG9rZW5zIH0gPSBjb21waWxlUGF0dGVybihwYXR0ZXJuKVxuXG4gIHJlZ2V4cFNvdXJjZSArPSAnLyonIC8vIENhcHR1cmUgcGF0aCBzZXBhcmF0b3JzXG5cbiAgLy8gU3BlY2lhbC1jYXNlIHBhdHRlcm5zIGxpa2UgJyonIGZvciBjYXRjaC1hbGwgcm91dGVzLlxuICBjb25zdCBjYXB0dXJlUmVtYWluaW5nID0gdG9rZW5zW3Rva2Vucy5sZW5ndGggLSAxXSAhPT0gJyonXG5cbiAgaWYgKGNhcHR1cmVSZW1haW5pbmcpIHtcbiAgICAvLyBUaGlzIHdpbGwgbWF0Y2ggbmV3bGluZXMgaW4gdGhlIHJlbWFpbmluZyBwYXRoLlxuICAgIHJlZ2V4cFNvdXJjZSArPSAnKFtcXFxcc1xcXFxTXSo/KSdcbiAgfVxuXG4gIGNvbnN0IG1hdGNoID0gcGF0aG5hbWUubWF0Y2gobmV3IFJlZ0V4cCgnXicgKyByZWdleHBTb3VyY2UgKyAnJCcsICdpJykpXG5cbiAgbGV0IHJlbWFpbmluZ1BhdGhuYW1lLCBwYXJhbVZhbHVlc1xuICBpZiAobWF0Y2ggIT0gbnVsbCkge1xuICAgIGlmIChjYXB0dXJlUmVtYWluaW5nKSB7XG4gICAgICByZW1haW5pbmdQYXRobmFtZSA9IG1hdGNoLnBvcCgpXG4gICAgICBjb25zdCBtYXRjaGVkUGF0aCA9XG4gICAgICAgIG1hdGNoWzBdLnN1YnN0cigwLCBtYXRjaFswXS5sZW5ndGggLSByZW1haW5pbmdQYXRobmFtZS5sZW5ndGgpXG5cbiAgICAgIC8vIElmIHdlIGRpZG4ndCBtYXRjaCB0aGUgZW50aXJlIHBhdGhuYW1lLCB0aGVuIG1ha2Ugc3VyZSB0aGF0IHRoZSBtYXRjaFxuICAgICAgLy8gd2UgZGlkIGdldCBlbmRzIGF0IGEgcGF0aCBzZXBhcmF0b3IgKHBvdGVudGlhbGx5IHRoZSBvbmUgd2UgYWRkZWRcbiAgICAgIC8vIGFib3ZlIGF0IHRoZSBiZWdpbm5pbmcgb2YgdGhlIHBhdGgsIGlmIHRoZSBhY3R1YWwgbWF0Y2ggd2FzIGVtcHR5KS5cbiAgICAgIGlmIChcbiAgICAgICAgcmVtYWluaW5nUGF0aG5hbWUgJiZcbiAgICAgICAgbWF0Y2hlZFBhdGguY2hhckF0KG1hdGNoZWRQYXRoLmxlbmd0aCAtIDEpICE9PSAnLydcbiAgICAgICkge1xuICAgICAgICByZXR1cm4ge1xuICAgICAgICAgIHJlbWFpbmluZ1BhdGhuYW1lOiBudWxsLFxuICAgICAgICAgIHBhcmFtTmFtZXMsXG4gICAgICAgICAgcGFyYW1WYWx1ZXM6IG51bGxcbiAgICAgICAgfVxuICAgICAgfVxuICAgIH0gZWxzZSB7XG4gICAgICAvLyBJZiB0aGlzIG1hdGNoZWQgYXQgYWxsLCB0aGVuIHRoZSBtYXRjaCB3YXMgdGhlIGVudGlyZSBwYXRobmFtZS5cbiAgICAgIHJlbWFpbmluZ1BhdGhuYW1lID0gJydcbiAgICB9XG5cbiAgICBwYXJhbVZhbHVlcyA9IG1hdGNoLnNsaWNlKDEpLm1hcChcbiAgICAgIHYgPT4gdiAhPSBudWxsID8gZGVjb2RlVVJJQ29tcG9uZW50KHYpIDogdlxuICAgIClcbiAgfSBlbHNlIHtcbiAgICByZW1haW5pbmdQYXRobmFtZSA9IHBhcmFtVmFsdWVzID0gbnVsbFxuICB9XG5cbiAgcmV0dXJuIHtcbiAgICByZW1haW5pbmdQYXRobmFtZSxcbiAgICBwYXJhbU5hbWVzLFxuICAgIHBhcmFtVmFsdWVzXG4gIH1cbn1cblxuZXhwb3J0IGZ1bmN0aW9uIGdldFBhcmFtTmFtZXMocGF0dGVybikge1xuICByZXR1cm4gY29tcGlsZVBhdHRlcm4ocGF0dGVybikucGFyYW1OYW1lc1xufVxuXG5leHBvcnQgZnVuY3Rpb24gZ2V0UGFyYW1zKHBhdHRlcm4sIHBhdGhuYW1lKSB7XG4gIGNvbnN0IHsgcGFyYW1OYW1lcywgcGFyYW1WYWx1ZXMgfSA9IG1hdGNoUGF0dGVybihwYXR0ZXJuLCBwYXRobmFtZSlcblxuICBpZiAocGFyYW1WYWx1ZXMgIT0gbnVsbCkge1xuICAgIHJldHVybiBwYXJhbU5hbWVzLnJlZHVjZShmdW5jdGlvbiAobWVtbywgcGFyYW1OYW1lLCBpbmRleCkge1xuICAgICAgbWVtb1twYXJhbU5hbWVdID0gcGFyYW1WYWx1ZXNbaW5kZXhdXG4gICAgICByZXR1cm4gbWVtb1xuICAgIH0sIHt9KVxuICB9XG5cbiAgcmV0dXJuIG51bGxcbn1cblxuLyoqXG4gKiBSZXR1cm5zIGEgdmVyc2lvbiBvZiB0aGUgZ2l2ZW4gcGF0dGVybiB3aXRoIHBhcmFtcyBpbnRlcnBvbGF0ZWQuIFRocm93c1xuICogaWYgdGhlcmUgaXMgYSBkeW5hbWljIHNlZ21lbnQgb2YgdGhlIHBhdHRlcm4gZm9yIHdoaWNoIHRoZXJlIGlzIG5vIHBhcmFtLlxuICovXG5leHBvcnQgZnVuY3Rpb24gZm9ybWF0UGF0dGVybihwYXR0ZXJuLCBwYXJhbXMpIHtcbiAgcGFyYW1zID0gcGFyYW1zIHx8IHt9XG5cbiAgY29uc3QgeyB0b2tlbnMgfSA9IGNvbXBpbGVQYXR0ZXJuKHBhdHRlcm4pXG4gIGxldCBwYXJlbkNvdW50ID0gMCwgcGF0aG5hbWUgPSAnJywgc3BsYXRJbmRleCA9IDBcblxuICBsZXQgdG9rZW4sIHBhcmFtTmFtZSwgcGFyYW1WYWx1ZVxuICBmb3IgKGxldCBpID0gMCwgbGVuID0gdG9rZW5zLmxlbmd0aDsgaSA8IGxlbjsgKytpKSB7XG4gICAgdG9rZW4gPSB0b2tlbnNbaV1cblxuICAgIGlmICh0b2tlbiA9PT0gJyonIHx8IHRva2VuID09PSAnKionKSB7XG4gICAgICBwYXJhbVZhbHVlID0gQXJyYXkuaXNBcnJheShwYXJhbXMuc3BsYXQpID8gcGFyYW1zLnNwbGF0W3NwbGF0SW5kZXgrK10gOiBwYXJhbXMuc3BsYXRcblxuICAgICAgaW52YXJpYW50KFxuICAgICAgICBwYXJhbVZhbHVlICE9IG51bGwgfHwgcGFyZW5Db3VudCA+IDAsXG4gICAgICAgICdNaXNzaW5nIHNwbGF0ICMlcyBmb3IgcGF0aCBcIiVzXCInLFxuICAgICAgICBzcGxhdEluZGV4LCBwYXR0ZXJuXG4gICAgICApXG5cbiAgICAgIGlmIChwYXJhbVZhbHVlICE9IG51bGwpXG4gICAgICAgIHBhdGhuYW1lICs9IGVuY29kZVVSSShwYXJhbVZhbHVlKVxuICAgIH0gZWxzZSBpZiAodG9rZW4gPT09ICcoJykge1xuICAgICAgcGFyZW5Db3VudCArPSAxXG4gICAgfSBlbHNlIGlmICh0b2tlbiA9PT0gJyknKSB7XG4gICAgICBwYXJlbkNvdW50IC09IDFcbiAgICB9IGVsc2UgaWYgKHRva2VuLmNoYXJBdCgwKSA9PT0gJzonKSB7XG4gICAgICBwYXJhbU5hbWUgPSB0b2tlbi5zdWJzdHJpbmcoMSlcbiAgICAgIHBhcmFtVmFsdWUgPSBwYXJhbXNbcGFyYW1OYW1lXVxuXG4gICAgICBpbnZhcmlhbnQoXG4gICAgICAgIHBhcmFtVmFsdWUgIT0gbnVsbCB8fCBwYXJlbkNvdW50ID4gMCxcbiAgICAgICAgJ01pc3NpbmcgXCIlc1wiIHBhcmFtZXRlciBmb3IgcGF0aCBcIiVzXCInLFxuICAgICAgICBwYXJhbU5hbWUsIHBhdHRlcm5cbiAgICAgIClcblxuICAgICAgaWYgKHBhcmFtVmFsdWUgIT0gbnVsbClcbiAgICAgICAgcGF0aG5hbWUgKz0gZW5jb2RlVVJJQ29tcG9uZW50KHBhcmFtVmFsdWUpXG4gICAgfSBlbHNlIHtcbiAgICAgIHBhdGhuYW1lICs9IHRva2VuXG4gICAgfVxuICB9XG5cbiAgcmV0dXJuIHBhdGhuYW1lLnJlcGxhY2UoL1xcLysvZywgJy8nKVxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbW1vbi9wYXR0ZXJuVXRpbHMuanNcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgYXBpID0gcmVxdWlyZSgnYXBwL3NlcnZpY2VzL2FwaScpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxudmFyIHsgVExQVF9TRVNTSU5TX1JFQ0VJVkUgfSAgPSByZXF1aXJlKCcuL3Nlc3Npb25zL2FjdGlvblR5cGVzJyk7XG52YXIgeyBUTFBUX05PREVTX1JFQ0VJVkUgfSAgPSByZXF1aXJlKCcuL25vZGVzL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgZmV0Y2hOb2Rlc0FuZFNlc3Npb25zKCl7XG4gICAgYXBpLmdldChjZmcuYXBpLm5vZGVzUGF0aCkuZG9uZShqc29uPT57XG4gICAgICB2YXIgbm9kZUFycmF5ID0gW107XG4gICAgICB2YXIgc2Vzc2lvbnMgPSB7fTtcblxuICAgICAganNvbi5ub2Rlcy5mb3JFYWNoKGl0ZW09PiB7XG4gICAgICAgIG5vZGVBcnJheS5wdXNoKGl0ZW0ubm9kZSk7XG4gICAgICAgIGlmKGl0ZW0uc2Vzc2lvbnMpe1xuICAgICAgICAgIGl0ZW0uc2Vzc2lvbnMuZm9yRWFjaChpdGVtMj0+e1xuICAgICAgICAgICAgc2Vzc2lvbnNbaXRlbTIuaWRdID0gaXRlbTI7XG4gICAgICAgICAgfSlcbiAgICAgICAgfVxuICAgICAgfSk7XG5cbiAgICAgIHJlYWN0b3IuYmF0Y2goKCkgPT4ge1xuICAgICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfTk9ERVNfUkVDRUlWRSwgbm9kZUFycmF5KTtcbiAgICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1NFU1NJTlNfUkVDRUlWRSwgc2Vzc2lvbnMpO1xuICAgICAgfSk7XG5cbiAgICB9KTtcbiAgfVxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvYWN0aW9ucy5qc1xuICoqLyIsImNvbnN0IGFjdGl2ZVNlc3Npb24gPSBbIFsndGxwdF9hY3RpdmVfdGVybWluYWwnXSwgKGFjdGl2ZVNlc3Npb24pID0+IHtcbiAgICBpZighYWN0aXZlU2Vzc2lvbil7XG4gICAgICByZXR1cm4gbnVsbDtcbiAgICB9XG5cbiAgICByZXR1cm4gYWN0aXZlU2Vzc2lvbi50b0pTKCk7XG4gIH1cbl07XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgYWN0aXZlU2Vzc2lvblxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvZ2V0dGVycy5qc1xuICoqLyIsIm1vZHVsZS5leHBvcnRzLmdldHRlcnMgPSByZXF1aXJlKCcuL2dldHRlcnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGlvbnMgPSByZXF1aXJlKCcuL2FjdGlvbnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGl2ZVRlcm1TdG9yZSA9IHJlcXVpcmUoJy4vYWN0aXZlVGVybVN0b3JlJyk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9pbmRleC5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnJlYWN0b3IucmVnaXN0ZXJTdG9yZXMoe1xuICAndGxwdF9hY3RpdmVfdGVybWluYWwnOiByZXF1aXJlKCcuL2FjdGl2ZVRlcm1pbmFsL2FjdGl2ZVRlcm1TdG9yZScpLFxuICAndGxwdF91c2VyJzogcmVxdWlyZSgnLi91c2VyL3VzZXJTdG9yZScpLFxuICAndGxwdF9ub2Rlcyc6IHJlcXVpcmUoJy4vbm9kZXMvbm9kZVN0b3JlJyksXG4gICd0bHB0X2ludml0ZSc6IHJlcXVpcmUoJy4vaW52aXRlL2ludml0ZVN0b3JlJyksXG4gICd0bHB0X3Jlc3RfYXBpJzogcmVxdWlyZSgnLi9yZXN0QXBpL3Jlc3RBcGlTdG9yZScpLFxuICAndGxwdF9zZXNzaW9ucyc6IHJlcXVpcmUoJy4vc2Vzc2lvbnMvc2Vzc2lvblN0b3JlJykgIFxufSk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9pbmRleC5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7IFRMUFRfUkVDRUlWRV9VU0VSX0lOVklURSB9ICA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcbnZhciBhcGkgPSByZXF1aXJlKCdhcHAvc2VydmljZXMvYXBpJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xuXG5leHBvcnQgZGVmYXVsdCB7XG4gIGZldGNoSW52aXRlKGludml0ZVRva2VuKXtcbiAgICB2YXIgcGF0aCA9IGNmZy5hcGkuZ2V0SW52aXRlVXJsKGludml0ZVRva2VuKTtcbiAgICBhcGkuZ2V0KHBhdGgpLmRvbmUoaW52aXRlPT57XG4gICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfUkVDRUlWRV9VU0VSX0lOVklURSwgaW52aXRlKTtcbiAgICB9KTtcbiAgfVxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2FjdGlvbnMuanNcbiAqKi8iLCIvKmVzbGludCBuby11bmRlZjogMCwgIG5vLXVudXNlZC12YXJzOiAwLCBuby1kZWJ1Z2dlcjowKi9cblxudmFyIHtUUllJTkdfVE9fU0lHTl9VUH0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9yZXN0QXBpL2NvbnN0YW50cycpO1xuXG5jb25zdCBpbnZpdGUgPSBbIFsndGxwdF9pbnZpdGUnXSwgKGludml0ZSkgPT4ge1xuICByZXR1cm4gaW52aXRlO1xuIH1cbl07XG5cbmNvbnN0IGF0dGVtcCA9IFsgWyd0bHB0X3Jlc3RfYXBpJywgVFJZSU5HX1RPX1NJR05fVVBdLCAoYXR0ZW1wKSA9PiB7XG4gIHZhciBkZWZhdWx0T2JqID0ge1xuICAgIGlzUHJvY2Vzc2luZzogZmFsc2UsXG4gICAgaXNFcnJvcjogZmFsc2UsXG4gICAgaXNTdWNjZXNzOiBmYWxzZSxcbiAgICBtZXNzYWdlOiAnJ1xuICB9XG5cbiAgcmV0dXJuIGF0dGVtcCA/IGF0dGVtcC50b0pTKCkgOiBkZWZhdWx0T2JqO1xuICBcbiB9XG5dO1xuXG5leHBvcnQgZGVmYXVsdCB7XG4gIGludml0ZSxcbiAgYXR0ZW1wXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvZ2V0dGVycy5qc1xuICoqLyIsIm1vZHVsZS5leHBvcnRzLmdldHRlcnMgPSByZXF1aXJlKCcuL2dldHRlcnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGlvbnMgPSByZXF1aXJlKCcuL2FjdGlvbnMnKTtcbm1vZHVsZS5leHBvcnRzLm5vZGVTdG9yZSA9IHJlcXVpcmUoJy4vaW52aXRlU3RvcmUnKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9pbmRleC5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7IFRMUFRfTk9ERVNfUkVDRUlWRSB9ICA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcbnZhciBhcGkgPSByZXF1aXJlKCdhcHAvc2VydmljZXMvYXBpJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xuXG5leHBvcnQgZGVmYXVsdCB7XG4gIGZldGNoTm9kZXMoKXtcbiAgICBhcGkuZ2V0KGNmZy5hcGkubm9kZXNQYXRoKS5kb25lKGRhdGE9PntcbiAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9OT0RFU19SRUNFSVZFLCBkYXRhLm5vZGVzKTtcbiAgICB9KTtcbiAgfVxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvYWN0aW9ucy5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7c2Vzc2lvbnNCeVNlcnZlcn0gPSByZXF1aXJlKCcuLy4uL3Nlc3Npb25zL2dldHRlcnMnKTtcblxuY29uc3Qgbm9kZUxpc3RWaWV3ID0gWyBbJ3RscHRfbm9kZXMnXSwgKG5vZGVzKSA9PntcbiAgICByZXR1cm4gbm9kZXMubWFwKChpdGVtKT0+e1xuICAgICAgdmFyIGFkZHIgPSBpdGVtLmdldCgnYWRkcicpO1xuICAgICAgdmFyIHNlc3Npb25zID0gcmVhY3Rvci5ldmFsdWF0ZShzZXNzaW9uc0J5U2VydmVyKGFkZHIpKTtcbiAgICAgIHJldHVybiB7XG4gICAgICAgIHRhZ3M6IGdldFRhZ3MoaXRlbSksXG4gICAgICAgIGFkZHI6IGFkZHIsXG4gICAgICAgIHNlc3Npb25Db3VudDogc2Vzc2lvbnMuc2l6ZVxuICAgICAgfVxuICAgIH0pLnRvSlMoKTtcbiB9XG5dO1xuXG5mdW5jdGlvbiBnZXRUYWdzKG5vZGUpe1xuICB2YXIgYWxsTGFiZWxzID0gW107XG4gIHZhciBsYWJlbHMgPSBub2RlLmdldCgnbGFiZWxzJyk7XG5cbiAgaWYobGFiZWxzKXtcbiAgICBsYWJlbHMuZW50cnlTZXEoKS50b0FycmF5KCkuZm9yRWFjaChpdGVtPT57XG4gICAgICBhbGxMYWJlbHMucHVzaCh7XG4gICAgICAgIHJvbGU6IGl0ZW1bMF0sXG4gICAgICAgIHZhbHVlOiBpdGVtWzFdXG4gICAgICB9KTtcbiAgICB9KTtcbiAgfVxuXG4gIGxhYmVscyA9IG5vZGUuZ2V0KCdjbWRfbGFiZWxzJyk7XG5cbiAgaWYobGFiZWxzKXtcbiAgICBsYWJlbHMuZW50cnlTZXEoKS50b0FycmF5KCkuZm9yRWFjaChpdGVtPT57XG4gICAgICBhbGxMYWJlbHMucHVzaCh7XG4gICAgICAgIHJvbGU6IGl0ZW1bMF0sXG4gICAgICAgIHZhbHVlOiBpdGVtWzFdLmdldCgncmVzdWx0JyksXG4gICAgICAgIHRvb2x0aXA6IGl0ZW1bMV0uZ2V0KCdjb21tYW5kJylcbiAgICAgIH0pO1xuICAgIH0pO1xuICB9XG5cbiAgcmV0dXJuIGFsbExhYmVscztcbn1cblxuXG5leHBvcnQgZGVmYXVsdCB7XG4gIG5vZGVMaXN0VmlldyAgXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9nZXR0ZXJzLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMuZ2V0dGVycyA9IHJlcXVpcmUoJy4vZ2V0dGVycycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aW9ucyA9IHJlcXVpcmUoJy4vYWN0aW9ucycpO1xubW9kdWxlLmV4cG9ydHMubm9kZVN0b3JlID0gcmVxdWlyZSgnLi9ub2RlU3RvcmUnKTtcblxuLy8gbm9kZXM6IFt7XCJpZFwiOlwieDIyMFwiLFwiYWRkclwiOlwiMC4wLjAuMDozMDIyXCIsXCJob3N0bmFtZVwiOlwieDIyMFwiLFwibGFiZWxzXCI6bnVsbCxcImNtZF9sYWJlbHNcIjpudWxsfV1cblxuXG4vLyBzZXNzaW9uczogW3tcImlkXCI6XCIwNzYzMDYzNi1iYjNkLTQwZTEtYjA4Ni02MGIyY2FlMjFhYzRcIixcInBhcnRpZXNcIjpbe1wiaWRcIjpcIjg5Zjc2MmEzLTc0MjktNGM3YS1hOTEzLTc2NjQ5M2ZlN2M4YVwiLFwic2l0ZVwiOlwiMTI3LjAuMC4xOjM3NTE0XCIsXCJ1c2VyXCI6XCJha29udHNldm95XCIsXCJzZXJ2ZXJfYWRkclwiOlwiMC4wLjAuMDozMDIyXCIsXCJsYXN0X2FjdGl2ZVwiOlwiMjAxNi0wMi0yMlQxNDozOToyMC45MzEyMDUzNS0wNTowMFwifV19XVxuXG4vKlxubGV0IFRvZG9SZWNvcmQgPSBJbW11dGFibGUuUmVjb3JkKHtcbiAgICBpZDogMCxcbiAgICBkZXNjcmlwdGlvbjogXCJcIixcbiAgICBjb21wbGV0ZWQ6IGZhbHNlXG59KTtcbiovXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9pbmRleC5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcblxudmFyIHtcbiAgVExQVF9SRVNUX0FQSV9TVEFSVCxcbiAgVExQVF9SRVNUX0FQSV9TVUNDRVNTLFxuICBUTFBUX1JFU1RfQVBJX0ZBSUwgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuXG4gIHN0YXJ0KHJlcVR5cGUpe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRVNUX0FQSV9TVEFSVCwge3R5cGU6IHJlcVR5cGV9KTtcbiAgfSxcblxuICBmYWlsKHJlcVR5cGUsIG1lc3NhZ2Upe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRVNUX0FQSV9GQUlMLCAge3R5cGU6IHJlcVR5cGUsIG1lc3NhZ2V9KTtcbiAgfSxcblxuICBzdWNjZXNzKHJlcVR5cGUpe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRVNUX0FQSV9TVUNDRVNTLCB7dHlwZTogcmVxVHlwZX0pO1xuICB9XG5cbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvYWN0aW9ucy5qc1xuICoqLyIsInZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyIHtcbiAgVExQVF9SRVNUX0FQSV9TVEFSVCxcbiAgVExQVF9SRVNUX0FQSV9TVUNDRVNTLFxuICBUTFBUX1JFU1RfQVBJX0ZBSUwgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKHt9KTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9SRVNUX0FQSV9TVEFSVCwgc3RhcnQpO1xuICAgIHRoaXMub24oVExQVF9SRVNUX0FQSV9GQUlMLCBmYWlsKTtcbiAgICB0aGlzLm9uKFRMUFRfUkVTVF9BUElfU1VDQ0VTUywgc3VjY2Vzcyk7XG4gIH1cbn0pXG5cbmZ1bmN0aW9uIHN0YXJ0KHN0YXRlLCByZXF1ZXN0KXtcbiAgcmV0dXJuIHN0YXRlLnNldChyZXF1ZXN0LnR5cGUsIHRvSW1tdXRhYmxlKHtpc1Byb2Nlc3Npbmc6IHRydWV9KSk7XG59XG5cbmZ1bmN0aW9uIGZhaWwoc3RhdGUsIHJlcXVlc3Qpe1xuICByZXR1cm4gc3RhdGUuc2V0KHJlcXVlc3QudHlwZSwgdG9JbW11dGFibGUoe2lzRmFpbGVkOiB0cnVlLCBtZXNzYWdlOiByZXF1ZXN0Lm1lc3NhZ2V9KSk7XG59XG5cbmZ1bmN0aW9uIHN1Y2Nlc3Moc3RhdGUsIHJlcXVlc3Qpe1xuICByZXR1cm4gc3RhdGUuc2V0KHJlcXVlc3QudHlwZSwgdG9JbW11dGFibGUoe2lzU3VjY2VzczogdHJ1ZX0pKTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvcmVzdEFwaVN0b3JlLmpzXG4gKiovIiwidmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHsgVExQVF9TRVNTSU5TX1JFQ0VJVkUsIFRMUFRfU0VTU0lOU19VUERBVEUgfSAgPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgdXBkYXRlKGpzb24pe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9TRVNTSU5TX1VQREFURSwganNvbik7XG4gIH0sXG5cbiAgcmVjZWl2ZShqc29uKXtcbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfU0VTU0lOU19SRUNFSVZFLCBqc29uKTtcbiAgfVxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvYWN0aW9ucy5qc1xuICoqLyIsIm1vZHVsZS5leHBvcnRzLmdldHRlcnMgPSByZXF1aXJlKCcuL2dldHRlcnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGlvbnMgPSByZXF1aXJlKCcuL2FjdGlvbnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGl2ZVRlcm1TdG9yZSA9IHJlcXVpcmUoJy4vc2Vzc2lvblN0b3JlJyk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9zZXNzaW9ucy9pbmRleC5qc1xuICoqLyIsInZhciB1dGlscyA9IHtcblxuICB1dWlkKCl7XG4gICAgLy8gbmV2ZXIgdXNlIGl0IGluIHByb2R1Y3Rpb25cbiAgICByZXR1cm4gJ3h4eHh4eHh4LXh4eHgtNHh4eC15eHh4LXh4eHh4eHh4eHh4eCcucmVwbGFjZSgvW3h5XS9nLCBmdW5jdGlvbihjKSB7XG4gICAgICB2YXIgciA9IE1hdGgucmFuZG9tKCkqMTZ8MCwgdiA9IGMgPT0gJ3gnID8gciA6IChyJjB4M3wweDgpO1xuICAgICAgcmV0dXJuIHYudG9TdHJpbmcoMTYpO1xuICAgIH0pO1xuICB9LFxuXG4gIGRpc3BsYXlEYXRlKGRhdGUpe1xuICAgIHRyeXtcbiAgICAgIHJldHVybiBkYXRlLnRvTG9jYWxlRGF0ZVN0cmluZygpICsgJyAnICsgZGF0ZS50b0xvY2FsZVRpbWVTdHJpbmcoKTtcbiAgICB9Y2F0Y2goZXJyKXtcbiAgICAgIGNvbnNvbGUuZXJyb3IoZXJyKTtcbiAgICAgIHJldHVybiAndW5kZWZpbmVkJztcbiAgICB9XG4gIH0sXG5cbiAgZm9ybWF0U3RyaW5nKGZvcm1hdCkge1xuICAgIHZhciBhcmdzID0gQXJyYXkucHJvdG90eXBlLnNsaWNlLmNhbGwoYXJndW1lbnRzLCAxKTtcbiAgICByZXR1cm4gZm9ybWF0LnJlcGxhY2UobmV3IFJlZ0V4cCgnXFxcXHsoXFxcXGQrKVxcXFx9JywgJ2cnKSxcbiAgICAgIChtYXRjaCwgbnVtYmVyKSA9PiB7XG4gICAgICAgIHJldHVybiAhKGFyZ3NbbnVtYmVyXSA9PT0gbnVsbCB8fCBhcmdzW251bWJlcl0gPT09IHVuZGVmaW5lZCkgPyBhcmdzW251bWJlcl0gOiAnJztcbiAgICB9KTtcbiAgfVxuICAgICAgICAgICAgXG59XG5cbm1vZHVsZS5leHBvcnRzID0gdXRpbHM7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvdXRpbHMuanNcbiAqKi8iLCJ2YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xudmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciBzZXNzaW9uID0gcmVxdWlyZSgnYXBwL3Nlc3Npb24nKTtcblxudmFyIEV2ZW50U3RyZWFtZXIgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIGNvbXBvbmVudERpZE1vdW50KCkge1xuICAgIGxldCB7c2lkfSA9IHRoaXMucHJvcHM7XG4gICAgbGV0IHt0b2tlbn0gPSBzZXNzaW9uLmdldFVzZXJEYXRhKCk7XG4gICAgbGV0IGNvbm5TdHIgPSBjZmcuYXBpLmdldEV2ZW50U3RyZWFtZXJDb25uU3RyKHRva2VuLCBzaWQpO1xuXG4gICAgdGhpcy5zb2NrZXQgPSBuZXcgV2ViU29ja2V0KGNvbm5TdHIsIFwicHJvdG9cIik7XG4gICAgdGhpcy5zb2NrZXQub25tZXNzYWdlID0gKCkgPT4ge307XG4gICAgdGhpcy5zb2NrZXQub25jbG9zZSA9ICgpID0+IHt9O1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50KCkge1xuICAgIHRoaXMuc29ja2V0LmNsb3NlKCk7XG4gIH0sXG5cbiAgc2hvdWxkQ29tcG9uZW50VXBkYXRlKCl7XG4gICAgcmV0dXJuIGZhbHNlO1xuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICByZXR1cm4gbnVsbDtcbiAgfVxufSk7XG5cbmV4cG9ydCBkZWZhdWx0IEV2ZW50U3RyZWFtZXI7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9hY3RpdmVTZXNzaW9uL2V2ZW50U3RyZWFtZXIuanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciBOYXZMZWZ0QmFyID0gcmVxdWlyZSgnLi9uYXZMZWZ0QmFyJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xudmFyIGFjdGlvbnMgPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9hY3Rpb25zJyk7XG52YXIge0FjdGl2ZVNlc3Npb259ID0gcmVxdWlyZSgnLi9hY3RpdmVTZXNzaW9uL21haW4uanN4Jyk7XG5cbnZhciBBcHAgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgY29tcG9uZW50RGlkTW91bnQoKXtcbiAgICBhY3Rpb25zLmZldGNoTm9kZXNBbmRTZXNzaW9ucygpO1xuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXRscHRcIj5cbiAgICAgICAgPE5hdkxlZnRCYXIvPlxuICAgICAgICA8QWN0aXZlU2Vzc2lvbi8+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwicm93XCI+XG4gICAgICAgICAgPG5hdiBjbGFzc05hbWU9XCJcIiByb2xlPVwibmF2aWdhdGlvblwiIHN0eWxlPXt7IG1hcmdpbkJvdHRvbTogMCB9fT5cbiAgICAgICAgICAgIDx1bCBjbGFzc05hbWU9XCJuYXYgbmF2YmFyLXRvcC1saW5rcyBuYXZiYXItcmlnaHRcIj5cbiAgICAgICAgICAgICAgPGxpPlxuICAgICAgICAgICAgICAgIDxhIGhyZWY9e2NmZy5yb3V0ZXMubG9nb3V0fT5cbiAgICAgICAgICAgICAgICAgIDxpIGNsYXNzTmFtZT1cImZhIGZhLXNpZ24tb3V0XCI+PC9pPlxuICAgICAgICAgICAgICAgICAgTG9nIG91dFxuICAgICAgICAgICAgICAgIDwvYT5cbiAgICAgICAgICAgICAgPC9saT5cbiAgICAgICAgICAgIDwvdWw+XG4gICAgICAgICAgPC9uYXY+XG4gICAgICAgIDwvZGl2PlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1wYWdlXCI+XG4gICAgICAgICAge3RoaXMucHJvcHMuY2hpbGRyZW59XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9kaXY+XG4gICAgKTtcbiAgfVxufSlcblxubW9kdWxlLmV4cG9ydHMgPSBBcHA7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9hcHAuanN4XG4gKiovIiwibW9kdWxlLmV4cG9ydHMuQXBwID0gcmVxdWlyZSgnLi9hcHAuanN4Jyk7XG5tb2R1bGUuZXhwb3J0cy5Mb2dpbiA9IHJlcXVpcmUoJy4vbG9naW4uanN4Jyk7XG5tb2R1bGUuZXhwb3J0cy5OZXdVc2VyID0gcmVxdWlyZSgnLi9uZXdVc2VyLmpzeCcpO1xubW9kdWxlLmV4cG9ydHMuTm9kZXMgPSByZXF1aXJlKCcuL25vZGVzL21haW4uanN4Jyk7XG5tb2R1bGUuZXhwb3J0cy5TZXNzaW9ucyA9IHJlcXVpcmUoJy4vc2Vzc2lvbnMvbWFpbi5qc3gnKTtcbm1vZHVsZS5leHBvcnRzLkFjdGl2ZVNlc3Npb24gPSByZXF1aXJlKCcuL2FjdGl2ZVNlc3Npb24vbWFpbi5qc3gnKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2luZGV4LmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgJCA9IHJlcXVpcmUoJ2pRdWVyeScpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIExpbmtlZFN0YXRlTWl4aW4gPSByZXF1aXJlKCdyZWFjdC1hZGRvbnMtbGlua2VkLXN0YXRlLW1peGluJyk7XG52YXIge2FjdGlvbnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvdXNlcicpO1xudmFyIEdvb2dsZUF1dGhJbmZvID0gcmVxdWlyZSgnLi9nb29nbGVBdXRoJyk7XG52YXIgTG9naW5JbnB1dEZvcm0gPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbTGlua2VkU3RhdGVNaXhpbl0sXG5cbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB7XG4gICAgICB1c2VyOiAnJyxcbiAgICAgIHBhc3N3b3JkOiAnJyxcbiAgICAgIHRva2VuOiAnJ1xuICAgIH1cbiAgfSxcblxuICBvbkNsaWNrOiBmdW5jdGlvbihlKSB7XG4gICAgZS5wcmV2ZW50RGVmYXVsdCgpO1xuICAgIGlmICh0aGlzLmlzVmFsaWQoKSkge1xuICAgICAgYWN0aW9ucy5sb2dpbih7IC4uLnRoaXMuc3RhdGV9LCAnL3dlYicpO1xuICAgIH1cbiAgfSxcblxuICBpc1ZhbGlkOiBmdW5jdGlvbigpIHtcbiAgICB2YXIgJGZvcm0gPSAkKHRoaXMucmVmcy5mb3JtKTtcbiAgICByZXR1cm4gJGZvcm0ubGVuZ3RoID09PSAwIHx8ICRmb3JtLnZhbGlkKCk7XG4gIH0sXG5cbiAgcmVuZGVyKCkge1xuICAgIHJldHVybiAoXG4gICAgICA8Zm9ybSByZWY9XCJmb3JtXCIgY2xhc3NOYW1lPVwiZ3J2LWxvZ2luLWlucHV0LWZvcm1cIj5cbiAgICAgICAgPGgzPiBXZWxjb21lIHRvIFRlbGVwb3J0IDwvaDM+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiXCI+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXQgdmFsdWVMaW5rPXt0aGlzLmxpbmtTdGF0ZSgndXNlcicpfSBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2wgcmVxdWlyZWRcIiBwbGFjZWhvbGRlcj1cIlVzZXIgbmFtZVwiIG5hbWU9XCJ1c2VyTmFtZVwiIC8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXQgdmFsdWVMaW5rPXt0aGlzLmxpbmtTdGF0ZSgncGFzc3dvcmQnKX0gdHlwZT1cInBhc3N3b3JkXCIgbmFtZT1cInBhc3N3b3JkXCIgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sIHJlcXVpcmVkXCIgcGxhY2Vob2xkZXI9XCJQYXNzd29yZFwiLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImZvcm0tZ3JvdXBcIj5cbiAgICAgICAgICAgIDxpbnB1dCB2YWx1ZUxpbms9e3RoaXMubGlua1N0YXRlKCd0b2tlbicpfSBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2wgcmVxdWlyZWRcIiBuYW1lPVwidG9rZW5cIiBwbGFjZWhvbGRlcj1cIlR3byBmYWN0b3IgdG9rZW4gKEdvb2dsZSBBdXRoZW50aWNhdG9yKVwiLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8YnV0dG9uIHR5cGU9XCJzdWJtaXRcIiBjbGFzc05hbWU9XCJidG4gYnRuLXByaW1hcnkgYmxvY2sgZnVsbC13aWR0aCBtLWJcIiBvbkNsaWNrPXt0aGlzLm9uQ2xpY2t9PkxvZ2luPC9idXR0b24+XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9mb3JtPlxuICAgICk7XG4gIH1cbn0pXG5cbnZhciBMb2dpbiA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBtaXhpbnM6IFtyZWFjdG9yLlJlYWN0TWl4aW5dLFxuXG4gIGdldERhdGFCaW5kaW5ncygpIHtcbiAgICByZXR1cm4ge1xuICAvLyAgICB1c2VyUmVxdWVzdDogZ2V0dGVycy51c2VyUmVxdWVzdFxuICAgIH1cbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIHZhciBpc1Byb2Nlc3NpbmcgPSBmYWxzZTsvL3RoaXMuc3RhdGUudXNlclJlcXVlc3QuZ2V0KCdpc0xvYWRpbmcnKTtcbiAgICB2YXIgaXNFcnJvciA9IGZhbHNlOy8vdGhpcy5zdGF0ZS51c2VyUmVxdWVzdC5nZXQoJ2lzRXJyb3InKTtcblxuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1sb2dpbiB0ZXh0LWNlbnRlclwiPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1sb2dvLXRwcnRcIj48L2Rpdj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtY29udGVudCBncnYtZmxleFwiPlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWZsZXgtY29sdW1uXCI+XG4gICAgICAgICAgICA8TG9naW5JbnB1dEZvcm0vPlxuICAgICAgICAgICAgPEdvb2dsZUF1dGhJbmZvLz5cbiAgICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWxvZ2luLWluZm9cIj5cbiAgICAgICAgICAgICAgPGkgY2xhc3NOYW1lPVwiZmEgZmEtcXVlc3Rpb25cIj48L2k+XG4gICAgICAgICAgICAgIDxzdHJvbmc+TmV3IEFjY291bnQgb3IgZm9yZ290IHBhc3N3b3JkPzwvc3Ryb25nPlxuICAgICAgICAgICAgICA8ZGl2PkFzayBmb3IgYXNzaXN0YW5jZSBmcm9tIHlvdXIgQ29tcGFueSBhZG1pbmlzdHJhdG9yPC9kaXY+XG4gICAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBMb2dpbjtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2xvZ2luLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgeyBSb3V0ZXIsIEluZGV4TGluaywgSGlzdG9yeSB9ID0gcmVxdWlyZSgncmVhY3Qtcm91dGVyJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xuXG52YXIgbWVudUl0ZW1zID0gW1xuICB7aWNvbjogJ2ZhIGZhLWNvZ3MnLCB0bzogY2ZnLnJvdXRlcy5ub2RlcywgdGl0bGU6ICdOb2Rlcyd9LFxuICB7aWNvbjogJ2ZhIGZhLXNpdGVtYXAnLCB0bzogY2ZnLnJvdXRlcy5zZXNzaW9ucywgdGl0bGU6ICdTZXNzaW9ucyd9LFxuICB7aWNvbjogJ2ZhIGZhLXF1ZXN0aW9uJywgdG86IGNmZy5yb3V0ZXMuc2Vzc2lvbnMsIHRpdGxlOiAnU2Vzc2lvbnMnfSxcbl07XG5cbnZhciBOYXZMZWZ0QmFyID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIHJlbmRlcjogZnVuY3Rpb24oKXtcbiAgICB2YXIgaXRlbXMgPSBtZW51SXRlbXMubWFwKChpLCBpbmRleCk9PntcbiAgICAgIHZhciBjbGFzc05hbWUgPSB0aGlzLmNvbnRleHQucm91dGVyLmlzQWN0aXZlKGkudG8pID8gJ2FjdGl2ZScgOiAnJztcbiAgICAgIHJldHVybiAoXG4gICAgICAgIDxsaSBrZXk9e2luZGV4fSBjbGFzc05hbWU9e2NsYXNzTmFtZX0+XG4gICAgICAgICAgPEluZGV4TGluayB0bz17aS50b30+XG4gICAgICAgICAgICA8aSBjbGFzc05hbWU9e2kuaWNvbn0gdGl0bGU9e2kudGl0bGV9Lz5cbiAgICAgICAgICA8L0luZGV4TGluaz5cbiAgICAgICAgPC9saT5cbiAgICAgICk7XG4gICAgfSk7XG5cbiAgICByZXR1cm4gKFxuICAgICAgPG5hdiBjbGFzc05hbWU9Jycgcm9sZT0nbmF2aWdhdGlvbicgc3R5bGU9e3t3aWR0aDogJzYwcHgnLCBmbG9hdDogJ2xlZnQnLCBwb3NpdGlvbjogJ2Fic29sdXRlJ319PlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT0nJz5cbiAgICAgICAgICA8dWwgY2xhc3NOYW1lPSduYXYgMW1ldGlzbWVudScgaWQ9J3NpZGUtbWVudSc+XG4gICAgICAgICAgICB7aXRlbXN9XG4gICAgICAgICAgPC91bD5cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L25hdj5cbiAgICApO1xuICB9XG59KTtcblxuTmF2TGVmdEJhci5jb250ZXh0VHlwZXMgPSB7XG4gIHJvdXRlcjogUmVhY3QuUHJvcFR5cGVzLm9iamVjdC5pc1JlcXVpcmVkXG59XG5cbm1vZHVsZS5leHBvcnRzID0gTmF2TGVmdEJhcjtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL25hdkxlZnRCYXIuanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciAkID0gcmVxdWlyZSgnalF1ZXJ5Jyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIge2FjdGlvbnMsIGdldHRlcnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvaW52aXRlJyk7XG52YXIgdXNlck1vZHVsZSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3VzZXInKTtcbnZhciBMaW5rZWRTdGF0ZU1peGluID0gcmVxdWlyZSgncmVhY3QtYWRkb25zLWxpbmtlZC1zdGF0ZS1taXhpbicpO1xudmFyIEdvb2dsZUF1dGhJbmZvID0gcmVxdWlyZSgnLi9nb29nbGVBdXRoJyk7XG5cbnZhciBJbnZpdGVJbnB1dEZvcm0gPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbTGlua2VkU3RhdGVNaXhpbl0sXG5cbiAgY29tcG9uZW50RGlkTW91bnQoKXtcbiAgICAkKHRoaXMucmVmcy5mb3JtKS52YWxpZGF0ZSh7XG4gICAgICBydWxlczp7XG4gICAgICAgIHBhc3N3b3JkOntcbiAgICAgICAgICBtaW5sZW5ndGg6IDUsXG4gICAgICAgICAgcmVxdWlyZWQ6IHRydWVcbiAgICAgICAgfSxcbiAgICAgICAgcGFzc3dvcmRDb25maXJtZWQ6e1xuICAgICAgICAgIHJlcXVpcmVkOiB0cnVlLFxuICAgICAgICAgIGVxdWFsVG86IHRoaXMucmVmcy5wYXNzd29yZFxuICAgICAgICB9XG4gICAgICB9LFxuXG4gICAgICBtZXNzYWdlczoge1xuICBcdFx0XHRwYXNzd29yZENvbmZpcm1lZDoge1xuICBcdFx0XHRcdG1pbmxlbmd0aDogJC52YWxpZGF0b3IuZm9ybWF0KCdFbnRlciBhdCBsZWFzdCB7MH0gY2hhcmFjdGVycycpLFxuICBcdFx0XHRcdGVxdWFsVG86ICdFbnRlciB0aGUgc2FtZSBwYXNzd29yZCBhcyBhYm92ZSdcbiAgXHRcdFx0fVxuICAgICAgfVxuICAgIH0pXG4gIH0sXG5cbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB7XG4gICAgICBuYW1lOiB0aGlzLnByb3BzLmludml0ZS51c2VyLFxuICAgICAgcHN3OiAnJyxcbiAgICAgIHBzd0NvbmZpcm1lZDogJycsXG4gICAgICB0b2tlbjogJydcbiAgICB9XG4gIH0sXG5cbiAgb25DbGljayhlKSB7XG4gICAgZS5wcmV2ZW50RGVmYXVsdCgpO1xuICAgIGlmICh0aGlzLmlzVmFsaWQoKSkge1xuICAgICAgdXNlck1vZHVsZS5hY3Rpb25zLnNpZ25VcCh7XG4gICAgICAgIG5hbWU6IHRoaXMuc3RhdGUubmFtZSxcbiAgICAgICAgcHN3OiB0aGlzLnN0YXRlLnBzdyxcbiAgICAgICAgdG9rZW46IHRoaXMuc3RhdGUudG9rZW4sXG4gICAgICAgIGludml0ZVRva2VuOiB0aGlzLnByb3BzLmludml0ZS5pbnZpdGVfdG9rZW59KTtcbiAgICB9XG4gIH0sXG5cbiAgaXNWYWxpZCgpIHtcbiAgICB2YXIgJGZvcm0gPSAkKHRoaXMucmVmcy5mb3JtKTtcbiAgICByZXR1cm4gJGZvcm0ubGVuZ3RoID09PSAwIHx8ICRmb3JtLnZhbGlkKCk7XG4gIH0sXG5cbiAgcmVuZGVyKCkge1xuICAgIHJldHVybiAoXG4gICAgICA8Zm9ybSByZWY9XCJmb3JtXCIgY2xhc3NOYW1lPVwiZ3J2LWludml0ZS1pbnB1dC1mb3JtXCI+XG4gICAgICAgIDxoMz4gR2V0IHN0YXJ0ZWQgd2l0aCBUZWxlcG9ydCA8L2gzPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0XG4gICAgICAgICAgICAgIHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ25hbWUnKX1cbiAgICAgICAgICAgICAgbmFtZT1cInVzZXJOYW1lXCJcbiAgICAgICAgICAgICAgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sIHJlcXVpcmVkXCJcbiAgICAgICAgICAgICAgcGxhY2Vob2xkZXI9XCJVc2VyIG5hbWVcIi8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXRcbiAgICAgICAgICAgICAgdmFsdWVMaW5rPXt0aGlzLmxpbmtTdGF0ZSgncHN3Jyl9XG4gICAgICAgICAgICAgIHJlZj1cInBhc3N3b3JkXCJcbiAgICAgICAgICAgICAgdHlwZT1cInBhc3N3b3JkXCJcbiAgICAgICAgICAgICAgbmFtZT1cInBhc3N3b3JkXCJcbiAgICAgICAgICAgICAgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sXCJcbiAgICAgICAgICAgICAgcGxhY2Vob2xkZXI9XCJQYXNzd29yZFwiIC8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwIGdydi1cIj5cbiAgICAgICAgICAgIDxpbnB1dFxuICAgICAgICAgICAgICB2YWx1ZUxpbms9e3RoaXMubGlua1N0YXRlKCdwc3dDb25maXJtZWQnKX1cbiAgICAgICAgICAgICAgdHlwZT1cInBhc3N3b3JkXCJcbiAgICAgICAgICAgICAgbmFtZT1cInBhc3N3b3JkQ29uZmlybWVkXCJcbiAgICAgICAgICAgICAgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sXCJcbiAgICAgICAgICAgICAgcGxhY2Vob2xkZXI9XCJQYXNzd29yZCBjb25maXJtXCIvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0XG4gICAgICAgICAgICAgIG5hbWU9XCJ0b2tlblwiXG4gICAgICAgICAgICAgIHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3Rva2VuJyl9XG4gICAgICAgICAgICAgIGNsYXNzTmFtZT1cImZvcm0tY29udHJvbCByZXF1aXJlZFwiXG4gICAgICAgICAgICAgIHBsYWNlaG9sZGVyPVwiVHdvIGZhY3RvciB0b2tlbiAoR29vZ2xlIEF1dGhlbnRpY2F0b3IpXCIgLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8YnV0dG9uIHR5cGU9XCJzdWJtaXRcIiBkaXNhYmxlZD17dGhpcy5wcm9wcy5hdHRlbXAuaXNQcm9jZXNzaW5nfSBjbGFzc05hbWU9XCJidG4gYnRuLXByaW1hcnkgYmxvY2sgZnVsbC13aWR0aCBtLWJcIiBvbkNsaWNrPXt0aGlzLm9uQ2xpY2t9ID5TaWduIHVwPC9idXR0b24+XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9mb3JtPlxuICAgICk7XG4gIH1cbn0pXG5cbnZhciBJbnZpdGUgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIGludml0ZTogZ2V0dGVycy5pbnZpdGUsXG4gICAgICBhdHRlbXA6IGdldHRlcnMuYXR0ZW1wXG4gICAgfVxuICB9LFxuXG4gIGNvbXBvbmVudERpZE1vdW50KCl7XG4gICAgYWN0aW9ucy5mZXRjaEludml0ZSh0aGlzLnByb3BzLnBhcmFtcy5pbnZpdGVUb2tlbik7XG4gIH0sXG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICBpZighdGhpcy5zdGF0ZS5pbnZpdGUpIHtcbiAgICAgIHJldHVybiBudWxsO1xuICAgIH1cblxuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1pbnZpdGUgdGV4dC1jZW50ZXJcIj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbG9nby10cHJ0XCI+PC9kaXY+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWNvbnRlbnQgZ3J2LWZsZXhcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LWNvbHVtblwiPlxuICAgICAgICAgICAgPEludml0ZUlucHV0Rm9ybSBhdHRlbXA9e3RoaXMuc3RhdGUuYXR0ZW1wfSBpbnZpdGU9e3RoaXMuc3RhdGUuaW52aXRlLnRvSlMoKX0vPlxuICAgICAgICAgICAgPEdvb2dsZUF1dGhJbmZvLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LWNvbHVtblwiPlxuICAgICAgICAgICAgPGg0PlNjYW4gYmFyIGNvZGUgZm9yIGF1dGggdG9rZW4gPGJyLz4gPHNtYWxsPlNjYW4gYmVsb3cgdG8gZ2VuZXJhdGUgeW91ciB0d28gZmFjdG9yIHRva2VuPC9zbWFsbD48L2g0PlxuICAgICAgICAgICAgPGltZyBjbGFzc05hbWU9XCJpbWctdGh1bWJuYWlsXCIgc3JjPXsgYGRhdGE6aW1hZ2UvcG5nO2Jhc2U2NCwke3RoaXMuc3RhdGUuaW52aXRlLmdldCgncXInKX1gIH0gLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBJbnZpdGU7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9uZXdVc2VyLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIge2dldHRlcnMsIGFjdGlvbnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvbm9kZXMnKTtcbnZhciB1c2VyR2V0dGVycyA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3VzZXIvZ2V0dGVycycpO1xudmFyIHtUYWJsZSwgQ29sdW1uLCBDZWxsfSA9IHJlcXVpcmUoJ2FwcC9jb21wb25lbnRzL3RhYmxlLmpzeCcpO1xudmFyIHtvcGVufSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGlvbnMnKTtcblxuY29uc3QgVGV4dENlbGwgPSAoe3Jvd0luZGV4LCBkYXRhLCBjb2x1bW5LZXksIC4uLnByb3BzfSkgPT4gKFxuICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgIHtkYXRhW3Jvd0luZGV4XVtjb2x1bW5LZXldfVxuICA8L0NlbGw+XG4pO1xuXG5jb25zdCBUYWdDZWxsID0gKHtyb3dJbmRleCwgZGF0YSwgY29sdW1uS2V5LCAuLi5wcm9wc30pID0+IChcbiAgPENlbGwgey4uLnByb3BzfT5cbiAgICB7IGRhdGFbcm93SW5kZXhdLnRhZ3MubWFwKChpdGVtLCBpbmRleCkgPT5cbiAgICAgICg8c3BhbiBrZXk9e2luZGV4fSBjbGFzc05hbWU9XCJsYWJlbCBsYWJlbC1kZWZhdWx0XCI+XG4gICAgICAgIHtpdGVtLnJvbGV9IDxsaSBjbGFzc05hbWU9XCJmYSBmYS1sb25nLWFycm93LXJpZ2h0XCI+PC9saT5cbiAgICAgICAge2l0ZW0udmFsdWV9XG4gICAgICA8L3NwYW4+KVxuICAgICkgfVxuICA8L0NlbGw+XG4pO1xuXG5jb25zdCBMb2dpbkNlbGwgPSAoe3VzZXIsIHJvd0luZGV4LCBkYXRhLCAuLi5wcm9wc30pID0+IHtcbiAgaWYoIXVzZXIgfHwgdXNlci5sb2dpbnMubGVuZ3RoID09PSAwKXtcbiAgICByZXR1cm4gPENlbGwgey4uLnByb3BzfSAvPjtcbiAgfVxuXG4gIHZhciAkbGlzID0gW107XG5cbiAgZm9yKHZhciBpID0gMDsgaSA8IHVzZXIubG9naW5zLmxlbmd0aDsgaSsrKXtcbiAgICAkbGlzLnB1c2goPGxpIGtleT17aX0+PGEgaHJlZj1cIiNcIiB0YXJnZXQ9XCJfYmxhbmtcIiBvbkNsaWNrPXtvcGVuLmJpbmQobnVsbCwgZGF0YVtyb3dJbmRleF0uYWRkciwgdXNlci5sb2dpbnNbaV0sIHVuZGVmaW5lZCl9Pnt1c2VyLmxvZ2luc1tpXX08L2E+PC9saT4pO1xuICB9XG5cbiAgcmV0dXJuIChcbiAgICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJidG4tZ3JvdXBcIj5cbiAgICAgICAgPGJ1dHRvbiB0eXBlPVwiYnV0dG9uXCIgb25DbGljaz17b3Blbi5iaW5kKG51bGwsIGRhdGFbcm93SW5kZXhdLmFkZHIsIHVzZXIubG9naW5zWzBdLCB1bmRlZmluZWQpfSBjbGFzc05hbWU9XCJidG4gYnRuLXNtIGJ0bi1wcmltYXJ5XCI+e3VzZXIubG9naW5zWzBdfTwvYnV0dG9uPlxuICAgICAgICB7XG4gICAgICAgICAgJGxpcy5sZW5ndGggPiAxID8gKFxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJidG4tZ3JvdXBcIj5cbiAgICAgICAgICAgICAgPGJ1dHRvbiBkYXRhLXRvZ2dsZT1cImRyb3Bkb3duXCIgY2xhc3NOYW1lPVwiYnRuIGJ0bi1kZWZhdWx0IGJ0bi1zbSBkcm9wZG93bi10b2dnbGVcIiBhcmlhLWV4cGFuZGVkPVwidHJ1ZVwiPlxuICAgICAgICAgICAgICAgIDxzcGFuIGNsYXNzTmFtZT1cImNhcmV0XCI+PC9zcGFuPlxuICAgICAgICAgICAgICA8L2J1dHRvbj5cbiAgICAgICAgICAgICAgPHVsIGNsYXNzTmFtZT1cImRyb3Bkb3duLW1lbnVcIj5cbiAgICAgICAgICAgICAgICA8bGk+PGEgaHJlZj1cIiNcIiB0YXJnZXQ9XCJfYmxhbmtcIj5Mb2dzPC9hPjwvbGk+XG4gICAgICAgICAgICAgICAgPGxpPjxhIGhyZWY9XCIjXCIgdGFyZ2V0PVwiX2JsYW5rXCI+TG9nczwvYT48L2xpPlxuICAgICAgICAgICAgICA8L3VsPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgKTogbnVsbFxuICAgICAgICB9XG4gICAgICA8L2Rpdj5cbiAgICA8L0NlbGw+XG4gIClcbn07XG5cbnZhciBOb2RlcyA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBtaXhpbnM6IFtyZWFjdG9yLlJlYWN0TWl4aW5dLFxuXG4gIGdldERhdGFCaW5kaW5ncygpIHtcbiAgICByZXR1cm4ge1xuICAgICAgbm9kZVJlY29yZHM6IGdldHRlcnMubm9kZUxpc3RWaWV3LFxuICAgICAgdXNlcjogdXNlckdldHRlcnMudXNlclxuICAgIH1cbiAgfSxcbiAgICBcbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICB2YXIgZGF0YSA9IHRoaXMuc3RhdGUubm9kZVJlY29yZHM7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LW5vZGVzXCI+XG4gICAgICAgIDxoMT4gTm9kZXMgPC9oMT5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICAgICAgPFRhYmxlIHJvd0NvdW50PXtkYXRhLmxlbmd0aH0gY2xhc3NOYW1lPVwidGFibGUtc3RyaXBwZWQgZ3J2LW5vZGVzLXRhYmxlXCI+XG4gICAgICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICAgICAgY29sdW1uS2V5PVwic2Vzc2lvbkNvdW50XCJcbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+IFNlc3Npb25zIDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17PFRleHRDZWxsIGRhdGE9e2RhdGF9Lz4gfVxuICAgICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICAgICAgY29sdW1uS2V5PVwiYWRkclwiXG4gICAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPiBOb2RlIDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17PFRleHRDZWxsIGRhdGE9e2RhdGF9Lz4gfVxuICAgICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICAgICAgY29sdW1uS2V5PVwidGFnc1wiXG4gICAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPjwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17PFRhZ0NlbGwgZGF0YT17ZGF0YX0vPiB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJyb2xlc1wiXG4gICAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPkxvZ2luIGFzPC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXs8TG9naW5DZWxsIGRhdGE9e2RhdGF9IHVzZXI9e3RoaXMuc3RhdGUudXNlcn0vPiB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgPC9UYWJsZT5cbiAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgIClcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gTm9kZXM7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9ub2Rlcy9tYWluLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIge1RhYmxlLCBDb2x1bW4sIENlbGwsIFRleHRDZWxsfSA9IHJlcXVpcmUoJ2FwcC9jb21wb25lbnRzL3RhYmxlLmpzeCcpO1xudmFyIHtnZXR0ZXJzfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3Nlc3Npb25zJyk7XG52YXIge29wZW59ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvYWN0aW9ucycpO1xuXG5jb25zdCBVc2Vyc0NlbGwgPSAoeyByb3dJbmRleCwgZGF0YSwgLi4ucHJvcHMgfSkgPT4ge1xuICB2YXIgJHVzZXJzID0gZGF0YVtyb3dJbmRleF0ucGFydGllcy5tYXAoKGl0ZW0sIGl0ZW1JbmRleCk9PlxuICAgICg8c3BhbiBrZXk9e2l0ZW1JbmRleH0gY2xhc3NOYW1lPVwidGV4dC11cHBlcmNhc2UgbGFiZWwgbGFiZWwtcHJpbWFyeVwiPntpdGVtLnVzZXJbMF19PC9zcGFuPilcbiAgKVxuXG4gIHJldHVybiAoXG4gICAgPENlbGwgey4uLnByb3BzfT5cbiAgICAgIDxkaXY+XG4gICAgICAgIHskdXNlcnN9XG4gICAgICA8L2Rpdj5cbiAgICA8L0NlbGw+XG4gIClcbn07XG5cbmNvbnN0IEJ1dHRvbkNlbGwgPSAoeyByb3dJbmRleCwgZGF0YSwgLi4ucHJvcHMgfSkgPT4ge1xuICBsZXQgb25DbGljayA9ICgpID0+IHtcbiAgICB2YXIgcm93RGF0YSA9IGRhdGFbcm93SW5kZXhdO1xuICAgIHZhciB7c2lkLCBhZGRyfSA9IHJvd0RhdGFcbiAgICB2YXIgdXNlciA9IHJvd0RhdGEucGFydGllc1swXS51c2VyO1xuICAgIG9wZW4oYWRkciwgdXNlciwgc2lkKTtcbiAgfVxuXG4gIHJldHVybiAoXG4gICAgPENlbGwgey4uLnByb3BzfT5cbiAgICAgIDxidXR0b24gb25DbGljaz17b25DbGlja30gY2xhc3NOYW1lPVwiYnRuIGJ0bi1pbmZvIGJ0bi1jaXJjbGVcIiB0eXBlPVwiYnV0dG9uXCI+XG4gICAgICAgIDxpIGNsYXNzTmFtZT1cImZhIGZhLXRlcm1pbmFsXCI+PC9pPlxuICAgICAgPC9idXR0b24+XG4gICAgPC9DZWxsPlxuICApXG59XG5cbnZhciBTZXNzaW9uTGlzdCA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBtaXhpbnM6IFtyZWFjdG9yLlJlYWN0TWl4aW5dLFxuXG4gIGdldERhdGFCaW5kaW5ncygpIHtcbiAgICByZXR1cm4ge1xuICAgICAgc2Vzc2lvbnNWaWV3OiBnZXR0ZXJzLnNlc3Npb25zVmlld1xuICAgIH1cbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIHZhciBkYXRhID0gdGhpcy5zdGF0ZS5zZXNzaW9uc1ZpZXc7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXY+XG4gICAgICAgIDxoMT4gU2Vzc2lvbnMhPC9oMT5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICAgICAgPFRhYmxlIHJvd0NvdW50PXtkYXRhLmxlbmd0aH0gY2xhc3NOYW1lPVwidGFibGUtc3RyaXBwZWRcIj5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJzaWRcIlxuICAgICAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD4gU2Vzc2lvbiBJRCA8L0NlbGw+IH1cbiAgICAgICAgICAgICAgICAgIGNlbGw9ezxUZXh0Q2VsbCBkYXRhPXtkYXRhfS8+IH1cbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+IDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17XG4gICAgICAgICAgICAgICAgICAgIDxCdXR0b25DZWxsIGRhdGE9e2RhdGF9IC8+XG4gICAgICAgICAgICAgICAgICB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJhZGRyXCJcbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+IE5vZGUgPC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXs8VGV4dENlbGwgZGF0YT17ZGF0YX0gLz4gfVxuICAgICAgICAgICAgICAgIC8+XG5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJhZGRyXCJcbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+IFVzZXJzIDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17PFVzZXJzQ2VsbCBkYXRhPXtkYXRhfSAvPiB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgPC9UYWJsZT5cbiAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgIClcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gU2Vzc2lvbkxpc3Q7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9zZXNzaW9ucy9tYWluLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgcmVuZGVyID0gcmVxdWlyZSgncmVhY3QtZG9tJykucmVuZGVyO1xudmFyIHsgUm91dGVyLCBSb3V0ZSwgUmVkaXJlY3QsIEluZGV4Um91dGUsIGJyb3dzZXJIaXN0b3J5IH0gPSByZXF1aXJlKCdyZWFjdC1yb3V0ZXInKTtcbnZhciB7IEFwcCwgTG9naW4sIE5vZGVzLCBTZXNzaW9ucywgTmV3VXNlciwgQWN0aXZlU2Vzc2lvbiB9ID0gcmVxdWlyZSgnLi9jb21wb25lbnRzJyk7XG52YXIge2Vuc3VyZVVzZXJ9ID0gcmVxdWlyZSgnLi9tb2R1bGVzL3VzZXIvYWN0aW9ucycpO1xudmFyIGF1dGggPSByZXF1aXJlKCcuL2F1dGgnKTtcbnZhciBzZXNzaW9uID0gcmVxdWlyZSgnLi9zZXNzaW9uJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnLi9jb25maWcnKTtcblxucmVxdWlyZSgnLi9tb2R1bGVzJyk7XG5cbi8vIGluaXQgc2Vzc2lvblxuc2Vzc2lvbi5pbml0KCk7XG5cbmZ1bmN0aW9uIGhhbmRsZUxvZ291dChuZXh0U3RhdGUsIHJlcGxhY2UsIGNiKXtcbiAgYXV0aC5sb2dvdXQoKTtcbn1cblxucmVuZGVyKChcbiAgPFJvdXRlciBoaXN0b3J5PXtzZXNzaW9uLmdldEhpc3RvcnkoKX0+XG4gICAgPFJvdXRlIHBhdGg9e2NmZy5yb3V0ZXMubG9naW59IGNvbXBvbmVudD17TG9naW59Lz5cbiAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5sb2dvdXR9IG9uRW50ZXI9e2hhbmRsZUxvZ291dH0vPlxuICAgIDxSb3V0ZSBwYXRoPXtjZmcucm91dGVzLm5ld1VzZXJ9IGNvbXBvbmVudD17TmV3VXNlcn0vPlxuICAgIDxSb3V0ZSBwYXRoPXtjZmcucm91dGVzLmFwcH0gY29tcG9uZW50PXtBcHB9IG9uRW50ZXI9e2Vuc3VyZVVzZXJ9ID5cbiAgICAgIDxJbmRleFJvdXRlIGNvbXBvbmVudD17Tm9kZXN9Lz5cbiAgICAgIDxSb3V0ZSBwYXRoPXtjZmcucm91dGVzLm5vZGVzfSBjb21wb25lbnQ9e05vZGVzfS8+XG4gICAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5zZXNzaW9uc30gY29tcG9uZW50PXtTZXNzaW9uc30vPlxuICAgIDwvUm91dGU+XG4gIDwvUm91dGVyPlxuKSwgZG9jdW1lbnQuZ2V0RWxlbWVudEJ5SWQoXCJhcHBcIikpO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2luZGV4LmpzeFxuICoqLyIsIm1vZHVsZS5leHBvcnRzID0gXztcblxuXG4vKioqKioqKioqKioqKioqKipcbiAqKiBXRUJQQUNLIEZPT1RFUlxuICoqIGV4dGVybmFsIFwiX1wiXG4gKiogbW9kdWxlIGlkID0gMjg1XG4gKiogbW9kdWxlIGNodW5rcyA9IDFcbiAqKi8iXSwic291cmNlUm9vdCI6IiJ9