webpackJsonp([1],{

/***/ 0:
/***/ function(module, exports, __webpack_require__) {

	module.exports = __webpack_require__(203);


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
	
	var _require = __webpack_require__(155);
	
	var formatPattern = _require.formatPattern;
	
	var cfg = {
	
	  baseUrl: window.location.origin,
	
	  api: {
	    renewTokenPath: '/v1/webapi/sessions/renew',
	    nodesPath: '/v1/webapi/sites/-current-/nodes',
	    sessionPath: '/v1/webapi/sessions',
	    terminalSessionPath: '/v1/webapi/sites/-current-/sessions/:sid',
	    invitePath: '/v1/webapi/users/invites/:inviteToken',
	    createUserPath: '/v1/webapi/users',
	
	    getTerminalSessionUrl: function getTerminalSessionUrl(sid) {
	      return formatPattern(cfg.api.terminalSessionPath, { sid: sid });
	    },
	
	    getInviteUrl: function getInviteUrl(inviteToken) {
	      return formatPattern(cfg.api.invitePath, { inviteToken: inviteToken });
	    },
	
	    getEventStreamConnStr: function getEventStreamConnStr(token, sid) {
	      var hostname = getWsHostName();
	      return hostname + '/v1/webapi/sites/-current-/sessions/' + sid + '/events/stream?access_token=' + token;
	    },
	
	    getTtyConnStr: function getTtyConnStr(_ref) {
	      var token = _ref.token;
	      var serverId = _ref.serverId;
	      var login = _ref.login;
	      var sid = _ref.sid;
	      var rows = _ref.rows;
	      var cols = _ref.cols;
	
	      var params = {
	        server_id: serverId,
	        login: login,
	        sid: sid,
	        term: {
	          h: rows,
	          w: cols
	        }
	      };
	
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

/***/ 37:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var $ = __webpack_require__(52);
	var session = __webpack_require__(30);
	
	var api = {
	
	  put: function put(path, data, withToken) {
	    return api.ajax({ url: path, data: JSON.stringify(data), type: 'PUT' }, withToken);
	  },
	
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
	
	var _require = __webpack_require__(168);
	
	var uuid = _require.uuid;
	
	var api = __webpack_require__(37);
	var cfg = __webpack_require__(14);
	var invariant = __webpack_require__(10);
	var getters = __webpack_require__(89);
	
	var _require2 = __webpack_require__(87);
	
	var TLPT_TERM_OPEN = _require2.TLPT_TERM_OPEN;
	var TLPT_TERM_CLOSE = _require2.TLPT_TERM_CLOSE;
	var TLPT_TERM_CONNECTED = _require2.TLPT_TERM_CONNECTED;
	var TLPT_TERM_RECEIVE_PARTIES = _require2.TLPT_TERM_RECEIVE_PARTIES;
	exports['default'] = {
	
	  close: function close() {
	    reactor.dispatch(TLPT_TERM_CLOSE);
	  },
	
	  resize: function resize(w, h) {
	    invariant(w > 5 || h > 5, 'invalid resize parameters');
	    var reqData = { terminal_params: { w: w, h: h } };
	
	    var _reactor$evaluate = reactor.evaluate(getters.activeSession);
	
	    var sid = _reactor$evaluate.sid;
	
	    api.put(cfg.api.getTerminalSessionUrl(sid), reqData).done(function () {
	      console.log('resize with w:' + w + ' and h:' + h + ' - OK');
	    }).fail(function () {
	      console.log('failed to resize with w:' + w + ' and h:' + h);
	    });
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
	
	  open: function open(serverId, login, sid) {
	    var isNew = !sid;
	    if (isNew) {
	      sid = uuid();
	    }
	
	    reactor.dispatch(TLPT_TERM_OPEN, { serverId: serverId, login: login, sid: sid, isNew: isNew });
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

/***/ 56:
/***/ function(module, exports) {

	'use strict';
	
	exports.__esModule = true;
	var user = [['tlpt_user'], function (currentUser) {
	  if (!currentUser) {
	    return null;
	  }
	
	  var name = currentUser.get('name') || '';
	  var shortDisplayName = name[0] || '';
	
	  return {
	    name: name,
	    shortDisplayName: shortDisplayName,
	    logins: currentUser.get('allowed_logins').toJS()
	  };
	}];
	
	exports['default'] = {
	  user: user
	};
	module.exports = exports['default'];

/***/ },

/***/ 86:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var api = __webpack_require__(37);
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

/***/ 87:
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

/***/ 88:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };
	
	var _require = __webpack_require__(18);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(87);
	
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
	    isConnecting: true
	  }, settings));
	}
	
	function connected(state) {
	  return state.set('isConnected', true).set('isConnecting', false);
	}
	module.exports = exports['default'];

/***/ },

/***/ 89:
/***/ function(module, exports) {

	'use strict';
	
	exports.__esModule = true;
	var activeSession = [['tlpt_active_terminal'], ['tlpt_sessions'], function (activeTerm, sessions) {
	  if (!activeTerm) {
	    return null;
	  }
	
	  var view = {
	    isNew: activeTerm.get('isNew'),
	    addr: activeTerm.get('addr'),
	    serverId: activeTerm.get('serverId'),
	    login: activeTerm.get('login'),
	    sid: activeTerm.get('sid'),
	    cols: undefined,
	    rows: undefined
	  };
	
	  if (sessions.has(view.sid)) {
	    view.cols = sessions.getIn([view.sid, 'terminal_params', 'w']);
	    view.rows = sessions.getIn([view.sid, 'terminal_params', 'h']);
	  }
	
	  return view;
	}];
	
	exports['default'] = {
	  activeSession: activeSession
	};
	module.exports = exports['default'];

/***/ },

/***/ 90:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(89);
	module.exports.actions = __webpack_require__(53);
	module.exports.activeTermStore = __webpack_require__(88);

/***/ },

/***/ 91:
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

/***/ 92:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(18);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(91);
	
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

/***/ 93:
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

/***/ 94:
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

/***/ 95:
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

/***/ 96:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(9);
	
	var _require = __webpack_require__(55);
	
	var TLPT_SESSINS_RECEIVE = _require.TLPT_SESSINS_RECEIVE;
	var TLPT_SESSINS_UPDATE = _require.TLPT_SESSINS_UPDATE;
	exports['default'] = {
	  updateSession: function updateSession(json) {
	    reactor.dispatch(TLPT_SESSINS_UPDATE, json);
	  },
	
	  receive: function receive(json) {
	    reactor.dispatch(TLPT_SESSINS_RECEIVE, json);
	  }
	};
	module.exports = exports['default'];

/***/ },

/***/ 97:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(18);
	
	var toImmutable = _require.toImmutable;
	
	var reactor = __webpack_require__(9);
	
	var sessionsByServer = function sessionsByServer(serverId) {
	  return [['tlpt_sessions'], function (sessions) {
	    return sessions.valueSeq().filter(function (item) {
	      var parties = item.get('parties') || toImmutable([]);
	      var hasServer = parties.find(function (item2) {
	        return item2.get('server_id') === serverId;
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
	      serverIp: parties[0].serverIp,
	      serverId: parties[0].serverId,
	      login: item.get('login'),
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
	        serverIp: item.get('remote_addr'),
	        serverId: item.get('server_id'),
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

/***/ 98:
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
	  return state.merge(json);
	}
	module.exports = exports['default'];

/***/ },

/***/ 99:
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

/***/ 100:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(9);
	
	var _require = __webpack_require__(99);
	
	var TLPT_RECEIVE_USER = _require.TLPT_RECEIVE_USER;
	
	var _require2 = __webpack_require__(95);
	
	var TRYING_TO_SIGN_UP = _require2.TRYING_TO_SIGN_UP;
	
	var restApiActions = __webpack_require__(165);
	var auth = __webpack_require__(86);
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

/***/ 101:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(56);
	module.exports.actions = __webpack_require__(100);
	module.exports.nodeStore = __webpack_require__(102);

/***/ },

/***/ 102:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(18);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(99);
	
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

/***/ 114:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	exports.__esModule = true;
	var React = __webpack_require__(5);
	
	var _require = __webpack_require__(90);
	
	var getters = _require.getters;
	var actions = _require.actions;
	
	var EventStreamer = __webpack_require__(194);
	var Tty = __webpack_require__(156);
	var TtyTerminal = __webpack_require__(202);
	
	var ActiveSession = React.createClass({
	  displayName: 'ActiveSession',
	
	  mixins: [reactor.ReactMixin],
	
	  getDataBindings: function getDataBindings() {
	    return {
	      activeSession: getters.activeSession
	    };
	  },
	
	  render: function render() {
	    if (!this.state.activeSession) {
	      return null;
	    }
	
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
	      React.createElement(TtyConnection, this.state.activeSession)
	    );
	  }
	});
	
	var TtyConnection = React.createClass({
	  displayName: 'TtyConnection',
	
	  getInitialState: function getInitialState() {
	    var _this = this;
	
	    this.tty = new Tty(this.props);
	    this.tty.on('open', function () {
	      return _this.setState({ isConnected: true });
	    });
	    return { isConnected: false };
	  },
	
	  componentWillUnmount: function componentWillUnmount() {
	    this.tty.disconnect();
	  },
	
	  render: function render() {
	    var component = new React.Component();
	    return React.createElement(
	      'component',
	      null,
	      React.createElement(TtyTerminal, { tty: this.tty, cols: this.props.cols, rows: this.props.rows }),
	      this.state.isConnected ? React.createElement(EventStreamer, { sid: this.props.sid }) : null
	    );
	  }
	});
	
	exports.ActiveSession = ActiveSession;
	exports.TtyConnection = TtyConnection;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "main.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 115:
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

/***/ 116:
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

/***/ 155:
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

/***/ 156:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };
	
	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError('Cannot call a class as a function'); } }
	
	function _inherits(subClass, superClass) { if (typeof superClass !== 'function' && superClass !== null) { throw new TypeError('Super expression must either be null or a function, not ' + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }
	
	var EventEmitter = __webpack_require__(169).EventEmitter;
	var session = __webpack_require__(30);
	var cfg = __webpack_require__(14);
	
	var _require = __webpack_require__(90);
	
	var actions = _require.actions;
	
	var Tty = (function (_EventEmitter) {
	  _inherits(Tty, _EventEmitter);
	
	  function Tty(_ref) {
	    var serverId = _ref.serverId;
	    var login = _ref.login;
	    var sid = _ref.sid;
	    var rows = _ref.rows;
	    var cols = _ref.cols;
	
	    _classCallCheck(this, Tty);
	
	    _EventEmitter.call(this);
	    this.options = { serverId: serverId, login: login, sid: sid, rows: rows, cols: cols };
	    this.socket = null;
	  }
	
	  Tty.prototype.disconnect = function disconnect() {
	    this.socket.close();
	  };
	
	  Tty.prototype.connect = function connect(options) {
	    var _this = this;
	
	    Object.assign(this.options, options);
	
	    var _session$getUserData = session.getUserData();
	
	    var token = _session$getUserData.token;
	
	    var connStr = cfg.api.getTtyConnStr(_extends({ token: token }, this.options));
	
	    this.socket = new WebSocket(connStr, 'proto');
	
	    this.socket.onopen = function () {
	      _this.emit('open');
	    };
	
	    this.socket.onmessage = function (e) {
	      _this.emit('data', e.data);
	    };
	
	    this.socket.onclose = function () {
	      _this.emit('close');
	    };
	  };
	
	  Tty.prototype.resize = function resize(cols, rows) {
	    actions.resize(cols, rows);
	  };
	
	  Tty.prototype.send = function send(data) {
	    this.socket.send(data);
	  };
	
	  return Tty;
	})(EventEmitter);
	
	module.exports = Tty;

/***/ },

/***/ 157:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(9);
	var api = __webpack_require__(37);
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

/***/ 158:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var reactor = __webpack_require__(9);
	reactor.registerStores({
	  'tlpt_active_terminal': __webpack_require__(88),
	  'tlpt_user': __webpack_require__(102),
	  'tlpt_nodes': __webpack_require__(93),
	  'tlpt_invite': __webpack_require__(92),
	  'tlpt_rest_api': __webpack_require__(166),
	  'tlpt_sessions': __webpack_require__(98)
	});

/***/ },

/***/ 159:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(9);
	
	var _require = __webpack_require__(91);
	
	var TLPT_RECEIVE_USER_INVITE = _require.TLPT_RECEIVE_USER_INVITE;
	
	var api = __webpack_require__(37);
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

/***/ 160:
/***/ function(module, exports, __webpack_require__) {

	/*eslint no-undef: 0,  no-unused-vars: 0, no-debugger:0*/
	
	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(95);
	
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

/***/ 161:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(160);
	module.exports.actions = __webpack_require__(159);
	module.exports.nodeStore = __webpack_require__(92);

/***/ },

/***/ 162:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(9);
	
	var _require = __webpack_require__(54);
	
	var TLPT_NODES_RECEIVE = _require.TLPT_NODES_RECEIVE;
	
	var api = __webpack_require__(37);
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

/***/ 163:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(9);
	
	var _require = __webpack_require__(97);
	
	var sessionsByServer = _require.sessionsByServer;
	
	var nodeListView = [['tlpt_nodes'], function (nodes) {
	  return nodes.map(function (item) {
	    var serverId = item.get('id');
	    var sessions = reactor.evaluate(sessionsByServer(serverId));
	    return {
	      id: serverId,
	      hostname: item.get('hostname'),
	      tags: getTags(item),
	      addr: item.get('addr'),
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

/***/ 164:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(163);
	module.exports.actions = __webpack_require__(162);
	module.exports.nodeStore = __webpack_require__(93);
	
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

/***/ 165:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(9);
	
	var _require = __webpack_require__(94);
	
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

/***/ 166:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(18);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(94);
	
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

/***/ 167:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(97);
	module.exports.actions = __webpack_require__(96);
	module.exports.activeTermStore = __webpack_require__(98);

/***/ },

/***/ 168:
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

/***/ 169:
/***/ function(module, exports) {

	// Copyright Joyent, Inc. and other Node contributors.
	//
	// Permission is hereby granted, free of charge, to any person obtaining a
	// copy of this software and associated documentation files (the
	// "Software"), to deal in the Software without restriction, including
	// without limitation the rights to use, copy, modify, merge, publish,
	// distribute, sublicense, and/or sell copies of the Software, and to permit
	// persons to whom the Software is furnished to do so, subject to the
	// following conditions:
	//
	// The above copyright notice and this permission notice shall be included
	// in all copies or substantial portions of the Software.
	//
	// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS
	// OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
	// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN
	// NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM,
	// DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR
	// OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE
	// USE OR OTHER DEALINGS IN THE SOFTWARE.
	
	function EventEmitter() {
	  this._events = this._events || {};
	  this._maxListeners = this._maxListeners || undefined;
	}
	module.exports = EventEmitter;
	
	// Backwards-compat with node 0.10.x
	EventEmitter.EventEmitter = EventEmitter;
	
	EventEmitter.prototype._events = undefined;
	EventEmitter.prototype._maxListeners = undefined;
	
	// By default EventEmitters will print a warning if more than 10 listeners are
	// added to it. This is a useful default which helps finding memory leaks.
	EventEmitter.defaultMaxListeners = 10;
	
	// Obviously not all Emitters should be limited to 10. This function allows
	// that to be increased. Set to zero for unlimited.
	EventEmitter.prototype.setMaxListeners = function(n) {
	  if (!isNumber(n) || n < 0 || isNaN(n))
	    throw TypeError('n must be a positive number');
	  this._maxListeners = n;
	  return this;
	};
	
	EventEmitter.prototype.emit = function(type) {
	  var er, handler, len, args, i, listeners;
	
	  if (!this._events)
	    this._events = {};
	
	  // If there is no 'error' event listener then throw.
	  if (type === 'error') {
	    if (!this._events.error ||
	        (isObject(this._events.error) && !this._events.error.length)) {
	      er = arguments[1];
	      if (er instanceof Error) {
	        throw er; // Unhandled 'error' event
	      }
	      throw TypeError('Uncaught, unspecified "error" event.');
	    }
	  }
	
	  handler = this._events[type];
	
	  if (isUndefined(handler))
	    return false;
	
	  if (isFunction(handler)) {
	    switch (arguments.length) {
	      // fast cases
	      case 1:
	        handler.call(this);
	        break;
	      case 2:
	        handler.call(this, arguments[1]);
	        break;
	      case 3:
	        handler.call(this, arguments[1], arguments[2]);
	        break;
	      // slower
	      default:
	        len = arguments.length;
	        args = new Array(len - 1);
	        for (i = 1; i < len; i++)
	          args[i - 1] = arguments[i];
	        handler.apply(this, args);
	    }
	  } else if (isObject(handler)) {
	    len = arguments.length;
	    args = new Array(len - 1);
	    for (i = 1; i < len; i++)
	      args[i - 1] = arguments[i];
	
	    listeners = handler.slice();
	    len = listeners.length;
	    for (i = 0; i < len; i++)
	      listeners[i].apply(this, args);
	  }
	
	  return true;
	};
	
	EventEmitter.prototype.addListener = function(type, listener) {
	  var m;
	
	  if (!isFunction(listener))
	    throw TypeError('listener must be a function');
	
	  if (!this._events)
	    this._events = {};
	
	  // To avoid recursion in the case that type === "newListener"! Before
	  // adding it to the listeners, first emit "newListener".
	  if (this._events.newListener)
	    this.emit('newListener', type,
	              isFunction(listener.listener) ?
	              listener.listener : listener);
	
	  if (!this._events[type])
	    // Optimize the case of one listener. Don't need the extra array object.
	    this._events[type] = listener;
	  else if (isObject(this._events[type]))
	    // If we've already got an array, just append.
	    this._events[type].push(listener);
	  else
	    // Adding the second element, need to change to array.
	    this._events[type] = [this._events[type], listener];
	
	  // Check for listener leak
	  if (isObject(this._events[type]) && !this._events[type].warned) {
	    var m;
	    if (!isUndefined(this._maxListeners)) {
	      m = this._maxListeners;
	    } else {
	      m = EventEmitter.defaultMaxListeners;
	    }
	
	    if (m && m > 0 && this._events[type].length > m) {
	      this._events[type].warned = true;
	      console.error('(node) warning: possible EventEmitter memory ' +
	                    'leak detected. %d listeners added. ' +
	                    'Use emitter.setMaxListeners() to increase limit.',
	                    this._events[type].length);
	      if (typeof console.trace === 'function') {
	        // not supported in IE 10
	        console.trace();
	      }
	    }
	  }
	
	  return this;
	};
	
	EventEmitter.prototype.on = EventEmitter.prototype.addListener;
	
	EventEmitter.prototype.once = function(type, listener) {
	  if (!isFunction(listener))
	    throw TypeError('listener must be a function');
	
	  var fired = false;
	
	  function g() {
	    this.removeListener(type, g);
	
	    if (!fired) {
	      fired = true;
	      listener.apply(this, arguments);
	    }
	  }
	
	  g.listener = listener;
	  this.on(type, g);
	
	  return this;
	};
	
	// emits a 'removeListener' event iff the listener was removed
	EventEmitter.prototype.removeListener = function(type, listener) {
	  var list, position, length, i;
	
	  if (!isFunction(listener))
	    throw TypeError('listener must be a function');
	
	  if (!this._events || !this._events[type])
	    return this;
	
	  list = this._events[type];
	  length = list.length;
	  position = -1;
	
	  if (list === listener ||
	      (isFunction(list.listener) && list.listener === listener)) {
	    delete this._events[type];
	    if (this._events.removeListener)
	      this.emit('removeListener', type, listener);
	
	  } else if (isObject(list)) {
	    for (i = length; i-- > 0;) {
	      if (list[i] === listener ||
	          (list[i].listener && list[i].listener === listener)) {
	        position = i;
	        break;
	      }
	    }
	
	    if (position < 0)
	      return this;
	
	    if (list.length === 1) {
	      list.length = 0;
	      delete this._events[type];
	    } else {
	      list.splice(position, 1);
	    }
	
	    if (this._events.removeListener)
	      this.emit('removeListener', type, listener);
	  }
	
	  return this;
	};
	
	EventEmitter.prototype.removeAllListeners = function(type) {
	  var key, listeners;
	
	  if (!this._events)
	    return this;
	
	  // not listening for removeListener, no need to emit
	  if (!this._events.removeListener) {
	    if (arguments.length === 0)
	      this._events = {};
	    else if (this._events[type])
	      delete this._events[type];
	    return this;
	  }
	
	  // emit removeListener for all listeners on all events
	  if (arguments.length === 0) {
	    for (key in this._events) {
	      if (key === 'removeListener') continue;
	      this.removeAllListeners(key);
	    }
	    this.removeAllListeners('removeListener');
	    this._events = {};
	    return this;
	  }
	
	  listeners = this._events[type];
	
	  if (isFunction(listeners)) {
	    this.removeListener(type, listeners);
	  } else {
	    // LIFO order
	    while (listeners.length)
	      this.removeListener(type, listeners[listeners.length - 1]);
	  }
	  delete this._events[type];
	
	  return this;
	};
	
	EventEmitter.prototype.listeners = function(type) {
	  var ret;
	  if (!this._events || !this._events[type])
	    ret = [];
	  else if (isFunction(this._events[type]))
	    ret = [this._events[type]];
	  else
	    ret = this._events[type].slice();
	  return ret;
	};
	
	EventEmitter.listenerCount = function(emitter, type) {
	  var ret;
	  if (!emitter._events || !emitter._events[type])
	    ret = 0;
	  else if (isFunction(emitter._events[type]))
	    ret = 1;
	  else
	    ret = emitter._events[type].length;
	  return ret;
	};
	
	function isFunction(arg) {
	  return typeof arg === 'function';
	}
	
	function isNumber(arg) {
	  return typeof arg === 'number';
	}
	
	function isObject(arg) {
	  return typeof arg === 'object' && arg !== null;
	}
	
	function isUndefined(arg) {
	  return arg === void 0;
	}


/***/ },

/***/ 194:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	exports.__esModule = true;
	var cfg = __webpack_require__(14);
	var React = __webpack_require__(5);
	var session = __webpack_require__(30);
	
	var _require = __webpack_require__(96);
	
	var updateSession = _require.updateSession;
	
	var EventStreamer = React.createClass({
	  displayName: 'EventStreamer',
	
	  componentDidMount: function componentDidMount() {
	    var sid = this.props.sid;
	
	    var _session$getUserData = session.getUserData();
	
	    var token = _session$getUserData.token;
	
	    var connStr = cfg.api.getEventStreamConnStr(token, sid);
	
	    this.socket = new WebSocket(connStr, 'proto');
	    this.socket.onmessage = function (event) {
	      try {
	        var json = JSON.parse(event.data);
	        updateSession(json.session);
	      } catch (err) {
	        console.log('failed to parse event stream data');
	      }
	    };
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

/***/ 195:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	var NavLeftBar = __webpack_require__(198);
	var cfg = __webpack_require__(14);
	var actions = __webpack_require__(157);
	
	var _require = __webpack_require__(114);
	
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
	          { className: '', role: 'navigation', style: { marginBottom: 0, float: "right" } },
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

/***/ 196:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	module.exports.App = __webpack_require__(195);
	module.exports.Login = __webpack_require__(197);
	module.exports.NewUser = __webpack_require__(199);
	module.exports.Nodes = __webpack_require__(200);
	module.exports.Sessions = __webpack_require__(201);
	module.exports.ActiveSession = __webpack_require__(114);
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "index.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 197:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };
	
	var React = __webpack_require__(5);
	var $ = __webpack_require__(52);
	var reactor = __webpack_require__(9);
	var LinkedStateMixin = __webpack_require__(61);
	
	var _require = __webpack_require__(101);
	
	var actions = _require.actions;
	
	var GoogleAuthInfo = __webpack_require__(115);
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

/***/ 198:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	
	var _require = __webpack_require__(43);
	
	var Router = _require.Router;
	var IndexLink = _require.IndexLink;
	var History = _require.History;
	
	var getters = __webpack_require__(56);
	var cfg = __webpack_require__(14);
	
	var menuItems = [{ icon: 'fa fa-cogs', to: cfg.routes.nodes, title: 'Nodes' }, { icon: 'fa fa-sitemap', to: cfg.routes.sessions, title: 'Sessions' }, { icon: 'fa fa-question', to: '#', title: 'Sessions' }];
	
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
	      { className: 'grv-nav navbar-default navbar-static-side', role: 'navigation' },
	      React.createElement(
	        'div',
	        { className: '' },
	        React.createElement(
	          'ul',
	          { className: 'nav', id: 'side-menu' },
	          React.createElement(
	            'li',
	            null,
	            React.createElement(
	              'div',
	              { className: 'grv-circle text-uppercase' },
	              React.createElement(
	                'span',
	                null,
	                getUserNameLetter()
	              )
	            )
	          ),
	          items
	        )
	      )
	    );
	  }
	});
	
	NavLeftBar.contextTypes = {
	  router: React.PropTypes.object.isRequired
	};
	
	function getUserNameLetter() {
	  var _reactor$evaluate = reactor.evaluate(getters.user);
	
	  var shortDisplayName = _reactor$evaluate.shortDisplayName;
	
	  return shortDisplayName;
	}
	
	module.exports = NavLeftBar;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "navLeftBar.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 199:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	var $ = __webpack_require__(52);
	var reactor = __webpack_require__(9);
	
	var _require = __webpack_require__(161);
	
	var actions = _require.actions;
	var getters = _require.getters;
	
	var userModule = __webpack_require__(101);
	var LinkedStateMixin = __webpack_require__(61);
	var GoogleAuthInfo = __webpack_require__(115);
	
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

/***/ 200:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }
	
	var React = __webpack_require__(5);
	var reactor = __webpack_require__(9);
	
	var _require = __webpack_require__(164);
	
	var getters = _require.getters;
	var actions = _require.actions;
	
	var userGetters = __webpack_require__(56);
	
	var _require2 = __webpack_require__(116);
	
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
	        { href: '#', target: '_blank', onClick: open.bind(null, data[rowIndex].id, user.logins[i], undefined) },
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
	        { type: 'button', onClick: open.bind(null, data[rowIndex].id, user.logins[0], undefined), className: 'btn btn-sm btn-primary' },
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

/***/ 201:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }
	
	var React = __webpack_require__(5);
	var reactor = __webpack_require__(9);
	
	var _require = __webpack_require__(116);
	
	var Table = _require.Table;
	var Column = _require.Column;
	var Cell = _require.Cell;
	var TextCell = _require.TextCell;
	
	var _require2 = __webpack_require__(167);
	
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
	    var serverId = rowData.serverId;
	
	    var login = rowData.login;
	    open(serverId, login, sid);
	  };
	
	  return React.createElement(
	    Cell,
	    props,
	    React.createElement(
	      'button',
	      { onClick: onClick, className: 'btn btn-info btn-circle', type: 'button' },
	      React.createElement('i', { className: 'fa fa-terminal' })
	    ),
	    React.createElement(
	      'button',
	      { className: 'btn btn-info btn-circle', type: 'button' },
	      React.createElement('i', { className: 'fa fa-play-circle' })
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
	      { className: 'grv-sessions' },
	      React.createElement(
	        'h1',
	        null,
	        ' Sessions'
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
	                columnKey: 'serverIp',
	                header: React.createElement(
	                  Cell,
	                  null,
	                  ' Node '
	                ),
	                cell: React.createElement(TextCell, { data: data })
	              }),
	              React.createElement(Column, {
	                columnKey: 'serverId',
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

/***/ 202:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var Term = __webpack_require__(288);
	var React = __webpack_require__(5);
	
	var _require = __webpack_require__(289);
	
	var debounce = _require.debounce;
	var isNumber = _require.isNumber;
	
	Term.colors[256] = 'inherit';
	
	var DISCONNECT_TXT = '\x1b[31mdisconnected\x1b[m\r\n';
	var CONNECTED_TXT = 'Connected!\r\n';
	
	var TtyTerminal = React.createClass({
	  displayName: 'TtyTerminal',
	
	  getInitialState: function getInitialState() {
	    var _this = this;
	
	    this.rows = this.props.rows;
	    this.cols = this.props.cols;
	    this.tty = this.props.tty;
	
	    this.debouncedResize = debounce(function () {
	      _this.resize();
	      _this.tty.resize(_this.cols, _this.rows);
	    }, 200);
	
	    return {};
	  },
	
	  componentDidMount: function componentDidMount() {
	    var _this2 = this;
	
	    this.term = new Terminal({
	      cols: 5,
	      rows: 5,
	      useStyle: true,
	      screenKeys: true,
	      cursorBlink: true
	    });
	
	    this.term.open(this.refs.container);
	    this.term.on('data', function (data) {
	      return _this2.tty.send(data);
	    });
	
	    this.resize(this.cols, this.rows);
	
	    this.tty.on('open', function () {
	      return _this2.term.write(CONNECTED_TXT);
	    });
	    this.tty.on('close', function () {
	      return _this2.term.write(DISCONNECT_TXT);
	    });
	    this.tty.on('data', function (data) {
	      return _this2.term.write(data);
	    });
	
	    this.tty.connect({ cols: this.cols, rows: this.rows });
	    window.addEventListener('resize', this.debouncedResize);
	  },
	
	  componentWillUnmount: function componentWillUnmount() {
	    this.term.destroy();
	    window.removeEventListener('resize', this.debouncedResize);
	  },
	
	  shouldComponentUpdate: function shouldComponentUpdate(newProps) {
	    var rows = newProps.rows;
	    var cols = newProps.cols;
	
	    if (!isNumber(rows) || !isNumber(cols)) {
	      return false;
	    }
	
	    if (rows !== this.rows || cols !== this.cols) {
	      this.resize(cols, rows);
	    }
	
	    return false;
	  },
	
	  render: function render() {
	    return React.createElement(
	      'div',
	      { className: 'grv-terminal', id: 'terminal-box', ref: 'container' },
	      '  '
	    );
	  },
	
	  resize: function resize(cols, rows) {
	    // if not defined, use the size of the container
	    if (!isNumber(cols) || !isNumber(rows)) {
	      var dim = this._getDimensions();
	      cols = dim.cols;
	      rows = dim.rows;
	    }
	
	    this.cols = cols;
	    this.rows = rows;
	
	    this.term.resize(this.cols, this.rows);
	  },
	
	  _getDimensions: function _getDimensions() {
	    var $container = $(this.refs.container);
	    var fakeRow = $('<div><span>&nbsp;</span></div>');
	
	    $container.find('.terminal').append(fakeRow);
	    // get div height
	    var fakeColHeight = fakeRow[0].getBoundingClientRect().height;
	    // get span width
	    var fakeColWidth = fakeRow.children().first()[0].getBoundingClientRect().width;
	    var cols = Math.floor($container.width() / fakeColWidth);
	    var rows = Math.floor($container.height() / fakeColHeight);
	    fakeRow.remove();
	
	    return { cols: cols, rows: rows };
	  }
	
	});
	
	TtyTerminal.propTypes = {
	  tty: React.PropTypes.object.isRequired
	};
	
	module.exports = TtyTerminal;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "terminal.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 203:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	var render = __webpack_require__(113).render;
	
	var _require = __webpack_require__(43);
	
	var Router = _require.Router;
	var Route = _require.Route;
	var Redirect = _require.Redirect;
	var IndexRoute = _require.IndexRoute;
	var browserHistory = _require.browserHistory;
	
	var _require2 = __webpack_require__(196);
	
	var App = _require2.App;
	var Login = _require2.Login;
	var Nodes = _require2.Nodes;
	var Sessions = _require2.Sessions;
	var NewUser = _require2.NewUser;
	var ActiveSession = _require2.ActiveSession;
	
	var _require3 = __webpack_require__(100);
	
	var ensureUser = _require3.ensureUser;
	
	var auth = __webpack_require__(86);
	var session = __webpack_require__(30);
	var cfg = __webpack_require__(14);
	
	__webpack_require__(158);
	
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

/***/ 288:
/***/ function(module, exports) {

	module.exports = Terminal;

/***/ },

/***/ 289:
/***/ function(module, exports) {

	module.exports = _;

/***/ }

});
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJzb3VyY2VzIjpbIndlYnBhY2s6Ly8vLi9zcmMvYXBwL3JlYWN0b3IuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb25maWcuanMiLCJ3ZWJwYWNrOi8vLy4vfi9rZXltaXJyb3IvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9zZXNzaW9uLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvc2VydmljZXMvYXBpLmpzIiwid2VicGFjazovLy9leHRlcm5hbCBcImpRdWVyeVwiIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvYWN0aW9uVHlwZXMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2F1dGguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGlvblR5cGVzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3RpdmVUZXJtU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvYWN0aW9uVHlwZXMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9pbnZpdGVTdG9yZS5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvbm9kZVN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2FjdGlvblR5cGVzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2NvbnN0YW50cy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvc2Vzc2lvblN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy91c2VyL2FjdGlvblR5cGVzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy91c2VyL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvdXNlclN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9hY3RpdmVTZXNzaW9uL21haW4uanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9nb29nbGVBdXRoLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvdGFibGUuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tbW9uL3BhdHRlcm5VdGlscy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbW1vbi90dHkuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9yZXN0QXBpU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvdXRpbHMuanMiLCJ3ZWJwYWNrOi8vLy4vfi9ldmVudHMvZXZlbnRzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9hY3RpdmVTZXNzaW9uL2V2ZW50U3RyZWFtZXIuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9hcHAuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9pbmRleC5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2xvZ2luLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvbmF2TGVmdEJhci5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL25ld1VzZXIuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9ub2Rlcy9tYWluLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvc2Vzc2lvbnMvbWFpbi5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL3Rlcm1pbmFsLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2luZGV4LmpzeCIsIndlYnBhY2s6Ly8vZXh0ZXJuYWwgXCJUZXJtaW5hbFwiIiwid2VicGFjazovLy9leHRlcm5hbCBcIl9cIiJdLCJuYW1lcyI6W10sIm1hcHBp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QTs7QUFFQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQSxNQUFLO0FBQ0w7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTs7QUFFQTs7QUFFQTtBQUNBO0FBQ0E7O0FBRUE7O0FBRUE7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7QUFDQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7QUFDQTtBQUNBOztBQUVBLElBQUc7QUFDSCxxQkFBb0IsU0FBUztBQUM3QjtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7QUFDQSxNQUFLO0FBQ0w7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTs7QUFFQTtBQUNBO0FBQ0EsSUFBRztBQUNIO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7Ozs7Ozs7Ozs7Ozs7QUM1U0EsS0FBSSxHQUFHLEdBQUcsbUJBQU8sQ0FBQyxFQUFZLENBQUMsQ0FBQztB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QyxJQUFJLEVBQUUsQ0FBQzs7QUFFZixVQUFTLFlBQVksQ0FBQyxTQUFTLEVBQUUsT0FBTyxFQUFFLEVBQUUsRUFBQztBQUMzQyxPQUFJLENBQUMsTUFBTSxFQUFFLENBQUM7RUFDZjs7QUFFRCxPQUFNLENBQ0o7QUFBQyxTQUFNO0tBQUMsT0FBTyxFQUFFLE9BQU8sQ0FBQyxVQUFVLEVBQUc7R0FDcEMsb0JBQUMsS0FBSyxJQUFDLElBQUksRUFBRSxHQUFHLENBQUMsTUFBTSxDQUFDLEtBQU0sRUFBQyxTQUFTLEVBQUUsS0FBTSxHQUFFO0dBQ2xELG9CQUFDLEtBQUssSUFBQyxJQUFJLEVBQUUsR0FBRyxDQUFDLE1BQU0sQ0FBQyxNQUFPLEVBQUMsT0FBTyxFQUFFLFlBQWEsR0FBRTtHQUN4RCxvQkFBQyxLQUFLLElBQUMsSUFBSSxFQUFFLEdBQUcsQ0FBQyxNQUFNLENBQUMsT0FBUSxFQUFDLFNBQVMsRUFBRSxPQUFRLEdBQUU7R0FDdEQ7QUFBQyxVQUFLO09BQUMsSUFBSSxFQUFFLEdBQUcsQ0FBQyxNQUFNLENBQUMsR0FBSSxFQUFDLFNBQVMsRUFBRSxHQUFJLEVBQUMsT0FBTyxFQUFFLFVBQVc7S0FDL0Qsb0JBQUMsVUFBVSxJQUFDLFNBQVMsRUFBRSxLQUFNLEdBQUU7S0FDL0Isb0JBQUMsS0FBSyxJQUFDLElBQUksRUFBRSxHQUFHLENBQUMsTUFBTSxDQUFDLEtBQU0sRUFBQyxTQUFTLEVBQUUsS0FBTSxHQUFFO0tBQ2xELG9CQUFDLEtBQUssSUFBQyxJQUFJLEVBQUUsR0FBRyxDQUFDLE1BQU0sQ0FBQyxRQUFTLEVBQUMsU0FBUyxFQUFFLFFBQVMsR0FBRTtJQUNsRDtFQUNELEVBQ1IsUUFBUSxDQUFDLGNBQWMsQ0FBQyxLQUFLLENBQUMsQ0FBQyxDOzs7Ozs7Ozs7QUM3QmxDLDJCOzs7Ozs7O0FDQUEsb0IiLCJmaWxlIjoiYXBwLmpzIiwic291cmNlc0NvbnRlbnQiOlsiaW1wb3J0IHsgUmVhY3RvciB9IGZyb20gJ251Y2xlYXItanMnXG5cbmNvbnN0IHJlYWN0b3IgPSBuZXcgUmVhY3Rvcih7XG4gIGRlYnVnOiB0cnVlXG59KVxuXG53aW5kb3cucmVhY3RvciA9IHJlYWN0b3I7XG5cbmV4cG9ydCBkZWZhdWx0IHJlYWN0b3JcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9yZWFjdG9yLmpzXG4gKiovIiwibGV0IHtmb3JtYXRQYXR0ZXJufSA9IHJlcXVpcmUoJ2FwcC9jb21tb24vcGF0dGVyblV0aWxzJyk7XG5cbmxldCBjZmcgPSB7XG5cbiAgYmFzZVVybDogd2luZG93LmxvY2F0aW9uLm9yaWdpbixcblxuICBhcGk6IHtcbiAgICByZW5ld1Rva2VuUGF0aDonL3YxL3dlYmFwaS9zZXNzaW9ucy9yZW5ldycsXG4gICAgbm9kZXNQYXRoOiAnL3YxL3dlYmFwaS9zaXRlcy8tY3VycmVudC0vbm9kZXMnLFxuICAgIHNlc3Npb25QYXRoOiAnL3YxL3dlYmFwaS9zZXNzaW9ucycsXG4gICAgdGVybWluYWxTZXNzaW9uUGF0aDogJy92MS93ZWJhcGkvc2l0ZXMvLWN1cnJlbnQtL3Nlc3Npb25zLzpzaWQnLFxuICAgIGludml0ZVBhdGg6ICcvdjEvd2ViYXBpL3VzZXJzL2ludml0ZXMvOmludml0ZVRva2VuJyxcbiAgICBjcmVhdGVVc2VyUGF0aDogJy92MS93ZWJhcGkvdXNlcnMnLFxuXG4gICAgZ2V0VGVybWluYWxTZXNzaW9uVXJsOiAoc2lkKT0+IHtcbiAgICAgIHJldHVybiBmb3JtYXRQYXR0ZXJuKGNmZy5hcGkudGVybWluYWxTZXNzaW9uUGF0aCwge3NpZH0pO1xuICAgIH0sXG5cbiAgICBnZXRJbnZpdGVVcmw6IChpbnZpdGVUb2tlbikgPT4ge1xuICAgICAgcmV0dXJuIGZvcm1hdFBhdHRlcm4oY2ZnLmFwaS5pbnZpdGVQYXRoLCB7aW52aXRlVG9rZW59KTtcbiAgICB9LFxuXG4gICAgZ2V0RXZlbnRTdHJlYW1Db25uU3RyOiAodG9rZW4sIHNpZCkgPT4ge1xuICAgICAgdmFyIGhvc3RuYW1lID0gZ2V0V3NIb3N0TmFtZSgpO1xuICAgICAgcmV0dXJuIGAke2hvc3RuYW1lfS92MS93ZWJhcGkvc2l0ZXMvLWN1cnJlbnQtL3Nlc3Npb25zLyR7c2lkfS9ldmVudHMvc3RyZWFtP2FjY2Vzc190b2tlbj0ke3Rva2VufWA7XG4gICAgfSxcblxuICAgIGdldFR0eUNvbm5TdHI6ICh7dG9rZW4sIHNlcnZlcklkLCBsb2dpbiwgc2lkLCByb3dzLCBjb2xzfSkgPT4ge1xuICAgICAgdmFyIHBhcmFtcyA9IHtcbiAgICAgICAgc2VydmVyX2lkOiBzZXJ2ZXJJZCxcbiAgICAgICAgbG9naW4sXG4gICAgICAgIHNpZCxcbiAgICAgICAgdGVybToge1xuICAgICAgICAgIGg6IHJvd3MsXG4gICAgICAgICAgdzogY29sc1xuICAgICAgICB9XG4gICAgICB9XG5cbiAgICAgIHZhciBqc29uID0gSlNPTi5zdHJpbmdpZnkocGFyYW1zKTtcbiAgICAgIHZhciBqc29uRW5jb2RlZCA9IHdpbmRvdy5lbmNvZGVVUkkoanNvbik7XG4gICAgICB2YXIgaG9zdG5hbWUgPSBnZXRXc0hvc3ROYW1lKCk7XG4gICAgICByZXR1cm4gYCR7aG9zdG5hbWV9L3YxL3dlYmFwaS9zaXRlcy8tY3VycmVudC0vY29ubmVjdD9hY2Nlc3NfdG9rZW49JHt0b2tlbn0mcGFyYW1zPSR7anNvbkVuY29kZWR9YDtcbiAgICB9XG4gIH0sXG5cbiAgcm91dGVzOiB7XG4gICAgYXBwOiAnL3dlYicsXG4gICAgbG9nb3V0OiAnL3dlYi9sb2dvdXQnLFxuICAgIGxvZ2luOiAnL3dlYi9sb2dpbicsXG4gICAgbm9kZXM6ICcvd2ViL25vZGVzJyxcbiAgICBhY3RpdmVTZXNzaW9uOiAnL3dlYi9hY3RpdmUtc2Vzc2lvbi86c2lkJyxcbiAgICBuZXdVc2VyOiAnL3dlYi9uZXd1c2VyLzppbnZpdGVUb2tlbicsXG4gICAgc2Vzc2lvbnM6ICcvd2ViL3Nlc3Npb25zJ1xuICB9XG5cbn1cblxuZXhwb3J0IGRlZmF1bHQgY2ZnO1xuXG5mdW5jdGlvbiBnZXRXc0hvc3ROYW1lKCl7XG4gIHZhciBwcmVmaXggPSBsb2NhdGlvbi5wcm90b2NvbCA9PSBcImh0dHBzOlwiP1wid3NzOi8vXCI6XCJ3czovL1wiO1xuICB2YXIgaG9zdHBvcnQgPSBsb2NhdGlvbi5ob3N0bmFtZSsobG9jYXRpb24ucG9ydCA/ICc6Jytsb2NhdGlvbi5wb3J0OiAnJyk7XG4gIHJldHVybiBgJHtwcmVmaXh9JHtob3N0cG9ydH1gO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbmZpZy5qc1xuICoqLyIsIi8qKlxuICogQ29weXJpZ2h0IDIwMTMtMjAxNCBGYWNlYm9vaywgSW5jLlxuICpcbiAqIExpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG4gKiB5b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG4gKiBZb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcbiAqXG4gKiBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcbiAqXG4gKiBVbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG4gKiBkaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG4gKiBXSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cbiAqIFNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbiAqIGxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuICpcbiAqL1xuXG5cInVzZSBzdHJpY3RcIjtcblxuLyoqXG4gKiBDb25zdHJ1Y3RzIGFuIGVudW1lcmF0aW9uIHdpdGgga2V5cyBlcXVhbCB0byB0aGVpciB2YWx1ZS5cbiAqXG4gKiBGb3IgZXhhbXBsZTpcbiAqXG4gKiAgIHZhciBDT0xPUlMgPSBrZXlNaXJyb3Ioe2JsdWU6IG51bGwsIHJlZDogbnVsbH0pO1xuICogICB2YXIgbXlDb2xvciA9IENPTE9SUy5ibHVlO1xuICogICB2YXIgaXNDb2xvclZhbGlkID0gISFDT0xPUlNbbXlDb2xvcl07XG4gKlxuICogVGhlIGxhc3QgbGluZSBjb3VsZCBub3QgYmUgcGVyZm9ybWVkIGlmIHRoZSB2YWx1ZXMgb2YgdGhlIGdlbmVyYXRlZCBlbnVtIHdlcmVcbiAqIG5vdCBlcXVhbCB0byB0aGVpciBrZXlzLlxuICpcbiAqICAgSW5wdXQ6ICB7a2V5MTogdmFsMSwga2V5MjogdmFsMn1cbiAqICAgT3V0cHV0OiB7a2V5MToga2V5MSwga2V5Mjoga2V5Mn1cbiAqXG4gKiBAcGFyYW0ge29iamVjdH0gb2JqXG4gKiBAcmV0dXJuIHtvYmplY3R9XG4gKi9cbnZhciBrZXlNaXJyb3IgPSBmdW5jdGlvbihvYmopIHtcbiAgdmFyIHJldCA9IHt9O1xuICB2YXIga2V5O1xuICBpZiAoIShvYmogaW5zdGFuY2VvZiBPYmplY3QgJiYgIUFycmF5LmlzQXJyYXkob2JqKSkpIHtcbiAgICB0aHJvdyBuZXcgRXJyb3IoJ2tleU1pcnJvciguLi4pOiBBcmd1bWVudCBtdXN0IGJlIGFuIG9iamVjdC4nKTtcbiAgfVxuICBmb3IgKGtleSBpbiBvYmopIHtcbiAgICBpZiAoIW9iai5oYXNPd25Qcm9wZXJ0eShrZXkpKSB7XG4gICAgICBjb250aW51ZTtcbiAgICB9XG4gICAgcmV0W2tleV0gPSBrZXk7XG4gIH1cbiAgcmV0dXJuIHJldDtcbn07XG5cbm1vZHVsZS5leHBvcnRzID0ga2V5TWlycm9yO1xuXG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiAuL34va2V5bWlycm9yL2luZGV4LmpzXG4gKiogbW9kdWxlIGlkID0gMjVcbiAqKiBtb2R1bGUgY2h1bmtzID0gMVxuICoqLyIsInZhciB7IGJyb3dzZXJIaXN0b3J5LCBjcmVhdGVNZW1vcnlIaXN0b3J5IH0gPSByZXF1aXJlKCdyZWFjdC1yb3V0ZXInKTtcblxuY29uc3QgQVVUSF9LRVlfREFUQSA9ICdhdXRoRGF0YSc7XG5cbnZhciBfaGlzdG9yeSA9IGNyZWF0ZU1lbW9yeUhpc3RvcnkoKTtcblxudmFyIHNlc3Npb24gPSB7XG5cbiAgaW5pdChoaXN0b3J5PWJyb3dzZXJIaXN0b3J5KXtcbiAgICBfaGlzdG9yeSA9IGhpc3Rvcnk7XG4gIH0sXG5cbiAgZ2V0SGlzdG9yeSgpe1xuICAgIHJldHVybiBfaGlzdG9yeTtcbiAgfSxcblxuICBzZXRVc2VyRGF0YSh1c2VyRGF0YSl7XG4gICAgbG9jYWxTdG9yYWdlLnNldEl0ZW0oQVVUSF9LRVlfREFUQSwgSlNPTi5zdHJpbmdpZnkodXNlckRhdGEpKTtcbiAgfSxcblxuICBnZXRVc2VyRGF0YSgpe1xuICAgIHZhciBpdGVtID0gbG9jYWxTdG9yYWdlLmdldEl0ZW0oQVVUSF9LRVlfREFUQSk7XG4gICAgaWYoaXRlbSl7XG4gICAgICByZXR1cm4gSlNPTi5wYXJzZShpdGVtKTtcbiAgICB9XG5cbiAgICByZXR1cm4ge307XG4gIH0sXG5cbiAgY2xlYXIoKXtcbiAgICBsb2NhbFN0b3JhZ2UuY2xlYXIoKVxuICB9XG5cbn1cblxubW9kdWxlLmV4cG9ydHMgPSBzZXNzaW9uO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL3Nlc3Npb24uanNcbiAqKi8iLCJ2YXIgJCA9IHJlcXVpcmUoXCJqUXVlcnlcIik7XG52YXIgc2Vzc2lvbiA9IHJlcXVpcmUoJ2FwcC9zZXNzaW9uJyk7XG5cbmNvbnN0IGFwaSA9IHtcblxuICBwdXQocGF0aCwgZGF0YSwgd2l0aFRva2VuKXtcbiAgICByZXR1cm4gYXBpLmFqYXgoe3VybDogcGF0aCwgZGF0YTogSlNPTi5zdHJpbmdpZnkoZGF0YSksIHR5cGU6ICdQVVQnfSwgd2l0aFRva2VuKTtcbiAgfSxcblxuICBwb3N0KHBhdGgsIGRhdGEsIHdpdGhUb2tlbil7XG4gICAgcmV0dXJuIGFwaS5hamF4KHt1cmw6IHBhdGgsIGRhdGE6IEpTT04uc3RyaW5naWZ5KGRhdGEpLCB0eXBlOiAnUE9TVCd9LCB3aXRoVG9rZW4pO1xuICB9LFxuXG4gIGdldChwYXRoKXtcbiAgICByZXR1cm4gYXBpLmFqYXgoe3VybDogcGF0aH0pO1xuICB9LFxuXG4gIGFqYXgoY2ZnLCB3aXRoVG9rZW4gPSB0cnVlKXtcbiAgICB2YXIgZGVmYXVsdENmZyA9IHtcbiAgICAgIHR5cGU6IFwiR0VUXCIsXG4gICAgICBkYXRhVHlwZTogXCJqc29uXCIsXG4gICAgICBiZWZvcmVTZW5kOiBmdW5jdGlvbih4aHIpIHtcbiAgICAgICAgaWYod2l0aFRva2VuKXtcbiAgICAgICAgICB2YXIgeyB0b2tlbiB9ID0gc2Vzc2lvbi5nZXRVc2VyRGF0YSgpO1xuICAgICAgICAgIHhoci5zZXRSZXF1ZXN0SGVhZGVyKCdBdXRob3JpemF0aW9uJywnQmVhcmVyICcgKyB0b2tlbik7XG4gICAgICAgIH1cbiAgICAgICB9XG4gICAgfVxuXG4gICAgcmV0dXJuICQuYWpheCgkLmV4dGVuZCh7fSwgZGVmYXVsdENmZywgY2ZnKSk7XG4gIH1cbn1cblxubW9kdWxlLmV4cG9ydHMgPSBhcGk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvc2VydmljZXMvYXBpLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMgPSBqUXVlcnk7XG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiBleHRlcm5hbCBcImpRdWVyeVwiXG4gKiogbW9kdWxlIGlkID0gNTJcbiAqKiBtb2R1bGUgY2h1bmtzID0gMVxuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7dXVpZH0gPSByZXF1aXJlKCdhcHAvdXRpbHMnKTtcbnZhciBhcGkgPSByZXF1aXJlKCdhcHAvc2VydmljZXMvYXBpJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xudmFyIGludmFyaWFudCA9IHJlcXVpcmUoJ2ludmFyaWFudCcpO1xudmFyIGdldHRlcnMgPSByZXF1aXJlKCcuL2dldHRlcnMnKTtcblxudmFyIHtcbiAgVExQVF9URVJNX09QRU4sXG4gIFRMUFRfVEVSTV9DTE9TRSxcbiAgVExQVF9URVJNX0NPTk5FQ1RFRCxcbiAgVExQVF9URVJNX1JFQ0VJVkVfUEFSVElFUyB9ICA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuXG4gIGNsb3NlKCl7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1RFUk1fQ0xPU0UpO1xuICB9LFxuXG4gIHJlc2l6ZSh3LCBoKXtcbiAgICBpbnZhcmlhbnQodyA+IDUgfHwgaCA+IDUsICdpbnZhbGlkIHJlc2l6ZSBwYXJhbWV0ZXJzJyk7XG4gICAgbGV0IHJlcURhdGEgPSB7IHRlcm1pbmFsX3BhcmFtczogeyB3LCBoIH0gfTtcbiAgICBsZXQge3NpZH0gPSByZWFjdG9yLmV2YWx1YXRlKGdldHRlcnMuYWN0aXZlU2Vzc2lvbik7XG5cbiAgICBhcGkucHV0KGNmZy5hcGkuZ2V0VGVybWluYWxTZXNzaW9uVXJsKHNpZCksIHJlcURhdGEpLmRvbmUoKCk9PntcbiAgICAgIGNvbnNvbGUubG9nKGByZXNpemUgd2l0aCB3OiR7d30gYW5kIGg6JHtofSAtIE9LYCk7XG4gICAgfSkuZmFpbCgoKT0+e1xuICAgICAgY29uc29sZS5sb2coYGZhaWxlZCB0byByZXNpemUgd2l0aCB3OiR7d30gYW5kIGg6JHtofWApO1xuICAgIH0pXG4gIH0sXG5cbiAgY29ubmVjdGVkKCl7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1RFUk1fQ09OTkVDVEVEKTtcbiAgfSxcblxuICByZWNlaXZlUGFydGllcyhqc29uKXtcbiAgICB2YXIgcGFydGllcyA9IGpzb24ubWFwKGl0ZW09PntcbiAgICAgIHJldHVybiB7XG4gICAgICAgIHVzZXI6IGl0ZW0udXNlcixcbiAgICAgICAgbGFzdEFjdGl2ZTogbmV3IERhdGUoaXRlbS5sYXN0X2FjdGl2ZSlcbiAgICAgIH1cbiAgICB9KVxuXG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1RFUk1fUkVDRUlWRV9QQVJUSUVTLCBwYXJ0aWVzKTtcbiAgfSxcblxuICBvcGVuKHNlcnZlcklkLCBsb2dpbiwgc2lkKXtcbiAgICBsZXQgaXNOZXcgPSAhc2lkO1xuICAgIGlmKGlzTmV3KXtcbiAgICAgIHNpZCA9IHV1aWQoKTtcbiAgICB9XG5cbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfVEVSTV9PUEVOLCB7c2VydmVySWQsIGxvZ2luLCBzaWQsIGlzTmV3fSApO1xuICB9XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3Rpb25zLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfTk9ERVNfUkVDRUlWRTogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2FjdGlvblR5cGVzLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfU0VTU0lOU19SRUNFSVZFOiBudWxsLFxuICBUTFBUX1NFU1NJTlNfVVBEQVRFOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvYWN0aW9uVHlwZXMuanNcbiAqKi8iLCJjb25zdCB1c2VyID0gWyBbJ3RscHRfdXNlciddLCAoY3VycmVudFVzZXIpID0+IHtcbiAgICBpZighY3VycmVudFVzZXIpe1xuICAgICAgcmV0dXJuIG51bGw7XG4gICAgfVxuXG4gICAgdmFyIG5hbWUgPSBjdXJyZW50VXNlci5nZXQoJ25hbWUnKSB8fCAnJztcbiAgICB2YXIgc2hvcnREaXNwbGF5TmFtZSA9IG5hbWVbMF0gfHwgJyc7XG5cbiAgICByZXR1cm4ge1xuICAgICAgbmFtZSxcbiAgICAgIHNob3J0RGlzcGxheU5hbWUsXG4gICAgICBsb2dpbnM6IGN1cnJlbnRVc2VyLmdldCgnYWxsb3dlZF9sb2dpbnMnKS50b0pTKClcbiAgICB9XG4gIH1cbl07XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgdXNlclxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvdXNlci9nZXR0ZXJzLmpzXG4gKiovIiwidmFyIGFwaSA9IHJlcXVpcmUoJy4vc2VydmljZXMvYXBpJyk7XG52YXIgc2Vzc2lvbiA9IHJlcXVpcmUoJy4vc2Vzc2lvbicpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcbnZhciAkID0gcmVxdWlyZSgnalF1ZXJ5Jyk7XG5cbmNvbnN0IHJlZnJlc2hSYXRlID0gNjAwMDAgKiA1OyAvLyAxIG1pblxuXG52YXIgcmVmcmVzaFRva2VuVGltZXJJZCA9IG51bGw7XG5cbnZhciBhdXRoID0ge1xuXG4gIHNpZ25VcChuYW1lLCBwYXNzd29yZCwgdG9rZW4sIGludml0ZVRva2VuKXtcbiAgICB2YXIgZGF0YSA9IHt1c2VyOiBuYW1lLCBwYXNzOiBwYXNzd29yZCwgc2Vjb25kX2ZhY3Rvcl90b2tlbjogdG9rZW4sIGludml0ZV90b2tlbjogaW52aXRlVG9rZW59O1xuICAgIHJldHVybiBhcGkucG9zdChjZmcuYXBpLmNyZWF0ZVVzZXJQYXRoLCBkYXRhKVxuICAgICAgLnRoZW4oKHVzZXIpPT57XG4gICAgICAgIHNlc3Npb24uc2V0VXNlckRhdGEodXNlcik7XG4gICAgICAgIGF1dGguX3N0YXJ0VG9rZW5SZWZyZXNoZXIoKTtcbiAgICAgICAgcmV0dXJuIHVzZXI7XG4gICAgICB9KTtcbiAgfSxcblxuICBsb2dpbihuYW1lLCBwYXNzd29yZCwgdG9rZW4pe1xuICAgIGF1dGguX3N0b3BUb2tlblJlZnJlc2hlcigpO1xuICAgIHJldHVybiBhdXRoLl9sb2dpbihuYW1lLCBwYXNzd29yZCwgdG9rZW4pLmRvbmUoYXV0aC5fc3RhcnRUb2tlblJlZnJlc2hlcik7XG4gIH0sXG5cbiAgZW5zdXJlVXNlcigpe1xuICAgIHZhciB1c2VyRGF0YSA9IHNlc3Npb24uZ2V0VXNlckRhdGEoKTtcbiAgICBpZih1c2VyRGF0YS50b2tlbil7XG4gICAgICAvLyByZWZyZXNoIHRpbWVyIHdpbGwgbm90IGJlIHNldCBpbiBjYXNlIG9mIGJyb3dzZXIgcmVmcmVzaCBldmVudFxuICAgICAgaWYoYXV0aC5fZ2V0UmVmcmVzaFRva2VuVGltZXJJZCgpID09PSBudWxsKXtcbiAgICAgICAgcmV0dXJuIGF1dGguX3JlZnJlc2hUb2tlbigpLmRvbmUoYXV0aC5fc3RhcnRUb2tlblJlZnJlc2hlcik7XG4gICAgICB9XG5cbiAgICAgIHJldHVybiAkLkRlZmVycmVkKCkucmVzb2x2ZSh1c2VyRGF0YSk7XG4gICAgfVxuXG4gICAgcmV0dXJuICQuRGVmZXJyZWQoKS5yZWplY3QoKTtcbiAgfSxcblxuICBsb2dvdXQoKXtcbiAgICBhdXRoLl9zdG9wVG9rZW5SZWZyZXNoZXIoKTtcbiAgICBzZXNzaW9uLmNsZWFyKCk7XG4gICAgc2Vzc2lvbi5nZXRIaXN0b3J5KCkucmVwbGFjZSh7cGF0aG5hbWU6IGNmZy5yb3V0ZXMubG9naW59KTsgICAgXG4gIH0sXG5cbiAgX3N0YXJ0VG9rZW5SZWZyZXNoZXIoKXtcbiAgICByZWZyZXNoVG9rZW5UaW1lcklkID0gc2V0SW50ZXJ2YWwoYXV0aC5fcmVmcmVzaFRva2VuLCByZWZyZXNoUmF0ZSk7XG4gIH0sXG5cbiAgX3N0b3BUb2tlblJlZnJlc2hlcigpe1xuICAgIGNsZWFySW50ZXJ2YWwocmVmcmVzaFRva2VuVGltZXJJZCk7XG4gICAgcmVmcmVzaFRva2VuVGltZXJJZCA9IG51bGw7XG4gIH0sXG5cbiAgX2dldFJlZnJlc2hUb2tlblRpbWVySWQoKXtcbiAgICByZXR1cm4gcmVmcmVzaFRva2VuVGltZXJJZDtcbiAgfSxcblxuICBfcmVmcmVzaFRva2VuKCl7XG4gICAgcmV0dXJuIGFwaS5wb3N0KGNmZy5hcGkucmVuZXdUb2tlblBhdGgpLnRoZW4oZGF0YT0+e1xuICAgICAgc2Vzc2lvbi5zZXRVc2VyRGF0YShkYXRhKTtcbiAgICAgIHJldHVybiBkYXRhO1xuICAgIH0pLmZhaWwoKCk9PntcbiAgICAgIGF1dGgubG9nb3V0KCk7XG4gICAgfSk7XG4gIH0sXG5cbiAgX2xvZ2luKG5hbWUsIHBhc3N3b3JkLCB0b2tlbil7XG4gICAgdmFyIGRhdGEgPSB7XG4gICAgICB1c2VyOiBuYW1lLFxuICAgICAgcGFzczogcGFzc3dvcmQsXG4gICAgICBzZWNvbmRfZmFjdG9yX3Rva2VuOiB0b2tlblxuICAgIH07XG5cbiAgICByZXR1cm4gYXBpLnBvc3QoY2ZnLmFwaS5zZXNzaW9uUGF0aCwgZGF0YSwgZmFsc2UpLnRoZW4oZGF0YT0+e1xuICAgICAgc2Vzc2lvbi5zZXRVc2VyRGF0YShkYXRhKTtcbiAgICAgIHJldHVybiBkYXRhO1xuICAgIH0pO1xuICB9XG59XG5cbm1vZHVsZS5leHBvcnRzID0gYXV0aDtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9hdXRoLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfVEVSTV9PUEVOOiBudWxsLFxuICBUTFBUX1RFUk1fQ0xPU0U6IG51bGwsXG4gIFRMUFRfVEVSTV9DT05ORUNURUQ6IG51bGwsXG4gIFRMUFRfVEVSTV9SRUNFSVZFX1BBUlRJRVM6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3Rpb25UeXBlcy5qc1xuICoqLyIsInZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyIHsgVExQVF9URVJNX09QRU4sIFRMUFRfVEVSTV9DTE9TRSwgVExQVF9URVJNX0NPTk5FQ1RFRCwgVExQVF9URVJNX1JFQ0VJVkVfUEFSVElFUyB9ICA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKG51bGwpO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX1RFUk1fQ09OTkVDVEVELCBjb25uZWN0ZWQpO1xuICAgIHRoaXMub24oVExQVF9URVJNX09QRU4sIHNldEFjdGl2ZVRlcm1pbmFsKTtcbiAgICB0aGlzLm9uKFRMUFRfVEVSTV9DTE9TRSwgY2xvc2UpO1xuICAgIHRoaXMub24oVExQVF9URVJNX1JFQ0VJVkVfUEFSVElFUywgcmVjZWl2ZVBhcnRpZXMpO1xuICB9XG5cbn0pXG5cbmZ1bmN0aW9uIGNsb3NlKCl7XG4gIHJldHVybiB0b0ltbXV0YWJsZShudWxsKTtcbn1cblxuZnVuY3Rpb24gcmVjZWl2ZVBhcnRpZXMoc3RhdGUsIHBhcnRpZXMpe1xuICByZXR1cm4gc3RhdGUuc2V0KCdwYXJ0aWVzJywgdG9JbW11dGFibGUocGFydGllcykpO1xufVxuXG5mdW5jdGlvbiBzZXRBY3RpdmVUZXJtaW5hbChzdGF0ZSwgc2V0dGluZ3Mpe1xuICByZXR1cm4gdG9JbW11dGFibGUoeyAgICAgIFxuICAgICAgaXNDb25uZWN0aW5nOiB0cnVlLFxuICAgICAgLi4uc2V0dGluZ3NcbiAgfSk7XG59XG5cbmZ1bmN0aW9uIGNvbm5lY3RlZChzdGF0ZSl7XG4gIHJldHVybiBzdGF0ZS5zZXQoJ2lzQ29ubmVjdGVkJywgdHJ1ZSlcbiAgICAgICAgICAgICAgLnNldCgnaXNDb25uZWN0aW5nJywgZmFsc2UpO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvYWN0aXZlVGVybVN0b3JlLmpzXG4gKiovIiwiY29uc3QgYWN0aXZlU2Vzc2lvbiA9IFtcblsndGxwdF9hY3RpdmVfdGVybWluYWwnXSwgWyd0bHB0X3Nlc3Npb25zJ10sXG4oYWN0aXZlVGVybSwgc2Vzc2lvbnMpID0+IHtcbiAgICBpZighYWN0aXZlVGVybSl7XG4gICAgICByZXR1cm4gbnVsbDtcbiAgICB9XG5cbiAgICBsZXQgdmlldyA9IHtcbiAgICAgIGlzTmV3OiBhY3RpdmVUZXJtLmdldCgnaXNOZXcnKSxcbiAgICAgIGFkZHI6IGFjdGl2ZVRlcm0uZ2V0KCdhZGRyJyksXG4gICAgICBzZXJ2ZXJJZDogYWN0aXZlVGVybS5nZXQoJ3NlcnZlcklkJyksXG4gICAgICBsb2dpbjogYWN0aXZlVGVybS5nZXQoJ2xvZ2luJyksXG4gICAgICBzaWQ6IGFjdGl2ZVRlcm0uZ2V0KCdzaWQnKSxcbiAgICAgIGNvbHM6IHVuZGVmaW5lZCxcbiAgICAgIHJvd3M6IHVuZGVmaW5lZFxuICAgIH07XG5cbiAgICBpZihzZXNzaW9ucy5oYXModmlldy5zaWQpKXtcbiAgICAgIHZpZXcuY29scyA9IHNlc3Npb25zLmdldEluKFt2aWV3LnNpZCwgJ3Rlcm1pbmFsX3BhcmFtcycsICd3J10pO1xuICAgICAgdmlldy5yb3dzID0gc2Vzc2lvbnMuZ2V0SW4oW3ZpZXcuc2lkLCAndGVybWluYWxfcGFyYW1zJywgJ2gnXSk7XG4gICAgfVxuXG4gICAgcmV0dXJuIHZpZXc7XG5cbiAgfVxuXTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICBhY3RpdmVTZXNzaW9uXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9nZXR0ZXJzLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMuZ2V0dGVycyA9IHJlcXVpcmUoJy4vZ2V0dGVycycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aW9ucyA9IHJlcXVpcmUoJy4vYWN0aW9ucycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aXZlVGVybVN0b3JlID0gcmVxdWlyZSgnLi9hY3RpdmVUZXJtU3RvcmUnKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2luZGV4LmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfUkVDRUlWRV9VU0VSX0lOVklURTogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9hY3Rpb25UeXBlcy5qc1xuICoqLyIsInZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyICB7IFRMUFRfUkVDRUlWRV9VU0VSX0lOVklURSB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gdG9JbW11dGFibGUobnVsbCk7XG4gIH0sXG5cbiAgaW5pdGlhbGl6ZSgpIHtcbiAgICB0aGlzLm9uKFRMUFRfUkVDRUlWRV9VU0VSX0lOVklURSwgcmVjZWl2ZUludml0ZSlcbiAgfVxufSlcblxuZnVuY3Rpb24gcmVjZWl2ZUludml0ZShzdGF0ZSwgaW52aXRlKXtcbiAgcmV0dXJuIHRvSW1tdXRhYmxlKGludml0ZSk7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvaW52aXRlU3RvcmUuanNcbiAqKi8iLCJ2YXIgeyBTdG9yZSwgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcbnZhciAgeyBUTFBUX05PREVTX1JFQ0VJVkUgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKFtdKTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9OT0RFU19SRUNFSVZFLCByZWNlaXZlTm9kZXMpXG4gIH1cbn0pXG5cbmZ1bmN0aW9uIHJlY2VpdmVOb2RlcyhzdGF0ZSwgbm9kZUFycmF5KXtcbiAgcmV0dXJuIHRvSW1tdXRhYmxlKG5vZGVBcnJheSk7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9ub2RlU3RvcmUuanNcbiAqKi8iLCJpbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVExQVF9SRVNUX0FQSV9TVEFSVDogbnVsbCxcbiAgVExQVF9SRVNUX0FQSV9TVUNDRVNTOiBudWxsLFxuICBUTFBUX1JFU1RfQVBJX0ZBSUw6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2FjdGlvblR5cGVzLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRSWUlOR19UT19TSUdOX1VQOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9jb25zdGFudHMuanNcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgeyBUTFBUX1NFU1NJTlNfUkVDRUlWRSwgVExQVF9TRVNTSU5TX1VQREFURSB9ICA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICB1cGRhdGVTZXNzaW9uKGpzb24pe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9TRVNTSU5TX1VQREFURSwganNvbik7XG4gIH0sXG5cbiAgcmVjZWl2ZShqc29uKXtcbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfU0VTU0lOU19SRUNFSVZFLCBqc29uKTtcbiAgfVxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvYWN0aW9ucy5qc1xuICoqLyIsInZhciB7IHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG5cbmNvbnN0IHNlc3Npb25zQnlTZXJ2ZXIgPSAoc2VydmVySWQpID0+IFtbJ3RscHRfc2Vzc2lvbnMnXSwgKHNlc3Npb25zKSA9PntcbiAgcmV0dXJuIHNlc3Npb25zLnZhbHVlU2VxKCkuZmlsdGVyKGl0ZW09PntcbiAgICB2YXIgcGFydGllcyA9IGl0ZW0uZ2V0KCdwYXJ0aWVzJykgfHwgdG9JbW11dGFibGUoW10pO1xuICAgIHZhciBoYXNTZXJ2ZXIgPSBwYXJ0aWVzLmZpbmQoaXRlbTI9PiBpdGVtMi5nZXQoJ3NlcnZlcl9pZCcpID09PSBzZXJ2ZXJJZCk7XG4gICAgcmV0dXJuIGhhc1NlcnZlcjtcbiAgfSkudG9MaXN0KCk7XG59XVxuXG5jb25zdCBzZXNzaW9uc1ZpZXcgPSBbWyd0bHB0X3Nlc3Npb25zJ10sIChzZXNzaW9ucykgPT57XG4gIHJldHVybiBzZXNzaW9ucy52YWx1ZVNlcSgpLm1hcChpdGVtPT57XG4gICAgdmFyIHNpZCA9IGl0ZW0uZ2V0KCdpZCcpO1xuICAgIHZhciBwYXJ0aWVzID0gcmVhY3Rvci5ldmFsdWF0ZShwYXJ0aWVzQnlTZXNzaW9uSWQoc2lkKSk7XG4gICAgcmV0dXJuIHtcbiAgICAgIHNpZDogc2lkLFxuICAgICAgc2VydmVySXA6IHBhcnRpZXNbMF0uc2VydmVySXAsXG4gICAgICBzZXJ2ZXJJZDogcGFydGllc1swXS5zZXJ2ZXJJZCxcbiAgICAgIGxvZ2luOiBpdGVtLmdldCgnbG9naW4nKSxcbiAgICAgIHBhcnRpZXM6IHBhcnRpZXNcbiAgICB9XG4gIH0pLnRvSlMoKTtcbn1dO1xuXG5jb25zdCBwYXJ0aWVzQnlTZXNzaW9uSWQgPSAoc2lkKSA9PlxuIFtbJ3RscHRfc2Vzc2lvbnMnLCBzaWQsICdwYXJ0aWVzJ10sIChwYXJ0aWVzKSA9PntcblxuICBpZighcGFydGllcyl7XG4gICAgcmV0dXJuIFtdO1xuICB9XG5cbiAgdmFyIGxhc3RBY3RpdmVVc3JOYW1lID0gZ2V0TGFzdEFjdGl2ZVVzZXIocGFydGllcykuZ2V0KCd1c2VyJyk7XG5cbiAgcmV0dXJuIHBhcnRpZXMubWFwKGl0ZW09PntcbiAgICB2YXIgdXNlciA9IGl0ZW0uZ2V0KCd1c2VyJyk7XG4gICAgcmV0dXJuIHtcbiAgICAgIHVzZXI6IGl0ZW0uZ2V0KCd1c2VyJyksXG4gICAgICBzZXJ2ZXJJcDogaXRlbS5nZXQoJ3JlbW90ZV9hZGRyJyksXG4gICAgICBzZXJ2ZXJJZDogaXRlbS5nZXQoJ3NlcnZlcl9pZCcpLFxuICAgICAgaXNBY3RpdmU6IGxhc3RBY3RpdmVVc3JOYW1lID09PSB1c2VyXG4gICAgfVxuICB9KS50b0pTKCk7XG59XTtcblxuZnVuY3Rpb24gZ2V0TGFzdEFjdGl2ZVVzZXIocGFydGllcyl7XG4gIHJldHVybiBwYXJ0aWVzLnNvcnRCeShpdGVtPT4gbmV3IERhdGUoaXRlbS5nZXQoJ2xhc3RBY3RpdmUnKSkpLmZpcnN0KCk7XG59XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgcGFydGllc0J5U2Vzc2lvbklkLFxuICBzZXNzaW9uc0J5U2VydmVyLFxuICBzZXNzaW9uc1ZpZXdcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2dldHRlcnMuanNcbiAqKi8iLCJ2YXIgeyBTdG9yZSwgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcbnZhciB7IFRMUFRfU0VTU0lOU19SRUNFSVZFLCBUTFBUX1NFU1NJTlNfVVBEQVRFIH0gID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gdG9JbW11dGFibGUoe30pO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX1NFU1NJTlNfUkVDRUlWRSwgcmVjZWl2ZVNlc3Npb25zKTtcbiAgICB0aGlzLm9uKFRMUFRfU0VTU0lOU19VUERBVEUsIHVwZGF0ZVNlc3Npb24pO1xuICB9XG59KVxuXG5mdW5jdGlvbiB1cGRhdGVTZXNzaW9uKHN0YXRlLCBqc29uKXtcbiAgcmV0dXJuIHN0YXRlLnNldChqc29uLmlkLCB0b0ltbXV0YWJsZShqc29uKSk7XG59XG5cbmZ1bmN0aW9uIHJlY2VpdmVTZXNzaW9ucyhzdGF0ZSwganNvbil7XG4gIHJldHVybiBzdGF0ZS5tZXJnZShqc29uKTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL3Nlc3Npb25TdG9yZS5qc1xuICoqLyIsImltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX1JFQ0VJVkVfVVNFUjogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvYWN0aW9uVHlwZXMuanNcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgeyBUTFBUX1JFQ0VJVkVfVVNFUiB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xudmFyIHsgVFJZSU5HX1RPX1NJR05fVVB9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvcmVzdEFwaS9jb25zdGFudHMnKTtcbnZhciByZXN0QXBpQWN0aW9ucyA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3Jlc3RBcGkvYWN0aW9ucycpO1xudmFyIGF1dGggPSByZXF1aXJlKCdhcHAvYXV0aCcpO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCdhcHAvc2Vzc2lvbicpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuXG4gIGVuc3VyZVVzZXIobmV4dFN0YXRlLCByZXBsYWNlLCBjYil7XG4gICAgYXV0aC5lbnN1cmVVc2VyKClcbiAgICAgIC5kb25lKCh1c2VyRGF0YSk9PiB7ICAgICAgICBcbiAgICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFQ0VJVkVfVVNFUiwgdXNlckRhdGEudXNlciApO1xuICAgICAgICBjYigpO1xuICAgICAgfSlcbiAgICAgIC5mYWlsKCgpPT57XG4gICAgICAgIHJlcGxhY2Uoe3JlZGlyZWN0VG86IG5leHRTdGF0ZS5sb2NhdGlvbi5wYXRobmFtZSB9LCBjZmcucm91dGVzLmxvZ2luKTtcbiAgICAgICAgY2IoKTtcbiAgICAgIH0pO1xuICB9LFxuXG4gIHNpZ25VcCh7bmFtZSwgcHN3LCB0b2tlbiwgaW52aXRlVG9rZW59KXtcbiAgICByZXN0QXBpQWN0aW9ucy5zdGFydChUUllJTkdfVE9fU0lHTl9VUCk7XG4gICAgYXV0aC5zaWduVXAobmFtZSwgcHN3LCB0b2tlbiwgaW52aXRlVG9rZW4pXG4gICAgICAuZG9uZSgoc2Vzc2lvbkRhdGEpPT57XG4gICAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRUNFSVZFX1VTRVIsIHNlc3Npb25EYXRhLnVzZXIpO1xuICAgICAgICByZXN0QXBpQWN0aW9ucy5zdWNjZXNzKFRSWUlOR19UT19TSUdOX1VQKTtcbiAgICAgICAgc2Vzc2lvbi5nZXRIaXN0b3J5KCkucHVzaCh7cGF0aG5hbWU6IGNmZy5yb3V0ZXMuYXBwfSk7XG4gICAgICB9KVxuICAgICAgLmZhaWwoKCk9PntcbiAgICAgICAgcmVzdEFwaUFjdGlvbnMuZmFpbChUUllJTkdfVE9fU0lHTl9VUCwgJ2ZhaWxlZCB0byBzaW5nIHVwJyk7XG4gICAgICB9KTtcbiAgfSxcblxuICBsb2dpbih7dXNlciwgcGFzc3dvcmQsIHRva2VufSwgcmVkaXJlY3Qpe1xuICAgICAgYXV0aC5sb2dpbih1c2VyLCBwYXNzd29yZCwgdG9rZW4pXG4gICAgICAgIC5kb25lKChzZXNzaW9uRGF0YSk9PntcbiAgICAgICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfUkVDRUlWRV9VU0VSLCBzZXNzaW9uRGF0YS51c2VyKTtcbiAgICAgICAgICBzZXNzaW9uLmdldEhpc3RvcnkoKS5wdXNoKHtwYXRobmFtZTogcmVkaXJlY3R9KTtcbiAgICAgICAgfSlcbiAgICAgICAgLmZhaWwoKCk9PntcbiAgICAgICAgfSlcbiAgICB9XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy91c2VyL2FjdGlvbnMuanNcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cy5nZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3Rpb25zID0gcmVxdWlyZSgnLi9hY3Rpb25zJyk7XG5tb2R1bGUuZXhwb3J0cy5ub2RlU3RvcmUgPSByZXF1aXJlKCcuL3VzZXJTdG9yZScpO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvdXNlci9pbmRleC5qc1xuICoqLyIsInZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyICB7IFRMUFRfUkVDRUlWRV9VU0VSIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IFN0b3JlKHtcbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB0b0ltbXV0YWJsZShudWxsKTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9SRUNFSVZFX1VTRVIsIHJlY2VpdmVVc2VyKVxuICB9XG5cbn0pXG5cbmZ1bmN0aW9uIHJlY2VpdmVVc2VyKHN0YXRlLCB1c2VyKXtcbiAgcmV0dXJuIHRvSW1tdXRhYmxlKHVzZXIpO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvdXNlci91c2VyU3RvcmUuanNcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHtnZXR0ZXJzLCBhY3Rpb25zfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsLycpO1xudmFyIEV2ZW50U3RyZWFtZXIgPSByZXF1aXJlKCcuL2V2ZW50U3RyZWFtZXIuanN4Jyk7XG52YXIgVHR5ID0gcmVxdWlyZSgnYXBwL2NvbW1vbi90dHknKTtcbnZhciBUdHlUZXJtaW5hbCA9IHJlcXVpcmUoJy4vLi4vdGVybWluYWwuanN4Jyk7XG5cbnZhciBBY3RpdmVTZXNzaW9uID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW3JlYWN0b3IuUmVhY3RNaXhpbl0sXG5cbiAgZ2V0RGF0YUJpbmRpbmdzKCkge1xuICAgIHJldHVybiB7XG4gICAgICBhY3RpdmVTZXNzaW9uOiBnZXR0ZXJzLmFjdGl2ZVNlc3Npb25cbiAgICB9XG4gIH0sXG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICBpZighdGhpcy5zdGF0ZS5hY3RpdmVTZXNzaW9uKXtcbiAgICAgIHJldHVybiBudWxsO1xuICAgIH1cblxuICAgIHJldHVybiAoXG4gICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXRlcm1pbmFsLWhvc3RcIj5cbiAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi10ZXJtaW5hbC1wYXJ0aWNpcGFuc1wiPlxuICAgICAgICAgPHVsIGNsYXNzTmFtZT1cIm5hdlwiPlxuICAgICAgICAgICA8bGk+PGJ1dHRvbiBjbGFzc05hbWU9XCJidG4gYnRuLXByaW1hcnkgYnRuLWNpcmNsZVwiIHR5cGU9XCJidXR0b25cIj4gPHN0cm9uZz5BPC9zdHJvbmc+PC9idXR0b24+PC9saT5cbiAgICAgICAgICAgPGxpPjxidXR0b24gY2xhc3NOYW1lPVwiYnRuIGJ0bi1wcmltYXJ5IGJ0bi1jaXJjbGVcIiB0eXBlPVwiYnV0dG9uXCI+IEIgPC9idXR0b24+PC9saT5cbiAgICAgICAgICAgPGxpPjxidXR0b24gY2xhc3NOYW1lPVwiYnRuIGJ0bi1wcmltYXJ5IGJ0bi1jaXJjbGVcIiB0eXBlPVwiYnV0dG9uXCI+IEMgPC9idXR0b24+PC9saT5cbiAgICAgICAgICAgPGxpPlxuICAgICAgICAgICAgIDxidXR0b24gb25DbGljaz17YWN0aW9ucy5jbG9zZX0gY2xhc3NOYW1lPVwiYnRuIGJ0bi1kYW5nZXIgYnRuLWNpcmNsZVwiIHR5cGU9XCJidXR0b25cIj5cbiAgICAgICAgICAgICAgIDxpIGNsYXNzTmFtZT1cImZhIGZhLXRpbWVzXCI+PC9pPlxuICAgICAgICAgICAgIDwvYnV0dG9uPlxuICAgICAgICAgICA8L2xpPlxuICAgICAgICAgPC91bD5cbiAgICAgICA8L2Rpdj5cbiAgICAgICA8ZGl2PlxuICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJidG4tZ3JvdXBcIj5cbiAgICAgICAgICAgPHNwYW4gY2xhc3NOYW1lPVwiYnRuIGJ0bi14cyBidG4tcHJpbWFyeVwiPjEyOC4wLjAuMTo4ODg4PC9zcGFuPlxuICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImJ0bi1ncm91cFwiPlxuICAgICAgICAgICAgIDxidXR0b24gZGF0YS10b2dnbGU9XCJkcm9wZG93blwiIGNsYXNzTmFtZT1cImJ0biBidG4tZGVmYXVsdCBidG4teHMgZHJvcGRvd24tdG9nZ2xlXCIgYXJpYS1leHBhbmRlZD1cInRydWVcIj5cbiAgICAgICAgICAgICAgIDxzcGFuIGNsYXNzTmFtZT1cImNhcmV0XCI+PC9zcGFuPlxuICAgICAgICAgICAgIDwvYnV0dG9uPlxuICAgICAgICAgICAgIDx1bCBjbGFzc05hbWU9XCJkcm9wZG93bi1tZW51XCI+XG4gICAgICAgICAgICAgICA8bGk+PGEgaHJlZj1cIiNcIiB0YXJnZXQ9XCJfYmxhbmtcIj5Mb2dzPC9hPjwvbGk+XG4gICAgICAgICAgICAgICA8bGk+PGEgaHJlZj1cIiNcIiB0YXJnZXQ9XCJfYmxhbmtcIj5Mb2dzPC9hPjwvbGk+XG4gICAgICAgICAgICAgPC91bD5cbiAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICA8L2Rpdj5cbiAgICAgICA8L2Rpdj5cbiAgICAgICA8VHR5Q29ubmVjdGlvbiB7Li4udGhpcy5zdGF0ZS5hY3RpdmVTZXNzaW9ufSAvPlxuICAgICA8L2Rpdj5cbiAgICAgKTtcbiAgfVxufSk7XG5cbnZhciBUdHlDb25uZWN0aW9uID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICB0aGlzLnR0eSA9IG5ldyBUdHkodGhpcy5wcm9wcylcbiAgICB0aGlzLnR0eS5vbignb3BlbicsICgpPT4gdGhpcy5zZXRTdGF0ZSh7IGlzQ29ubmVjdGVkOiB0cnVlIH0pKTtcbiAgICByZXR1cm4ge2lzQ29ubmVjdGVkOiBmYWxzZX07XG4gIH0sXG5cbiAgY29tcG9uZW50V2lsbFVubW91bnQoKSB7XG4gICAgdGhpcy50dHkuZGlzY29ubmVjdCgpO1xuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICBsZXQgY29tcG9uZW50ID0gbmV3IFJlYWN0LkNvbXBvbmVudCgpO1xuICAgIHJldHVybiAoXG4gICAgICA8Y29tcG9uZW50PlxuICAgICAgICA8VHR5VGVybWluYWwgdHR5PXt0aGlzLnR0eX0gY29scz17dGhpcy5wcm9wcy5jb2xzfSByb3dzPXt0aGlzLnByb3BzLnJvd3N9IC8+XG4gICAgICAgIHsgdGhpcy5zdGF0ZS5pc0Nvbm5lY3RlZCA/IDxFdmVudFN0cmVhbWVyIHNpZD17dGhpcy5wcm9wcy5zaWR9Lz4gOiBudWxsIH1cbiAgICAgIDwvY29tcG9uZW50PlxuICAgIClcbiAgfVxufSk7XG5cbmV4cG9ydCB7QWN0aXZlU2Vzc2lvbiwgVHR5Q29ubmVjdGlvbn07XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9hY3RpdmVTZXNzaW9uL21haW4uanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcblxudmFyIEdvb2dsZUF1dGhJbmZvID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWdvb2dsZS1hdXRoXCI+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWdvb2dsZS1hdXRoLWljb25cIj48L2Rpdj5cbiAgICAgICAgPHN0cm9uZz5Hb29nbGUgQXV0aGVudGljYXRvcjwvc3Ryb25nPlxuICAgICAgICA8ZGl2PkRvd25sb2FkIDxhIGhyZWY9XCJodHRwczovL3N1cHBvcnQuZ29vZ2xlLmNvbS9hY2NvdW50cy9hbnN3ZXIvMTA2NjQ0Nz9obD1lblwiPkdvb2dsZSBBdXRoZW50aWNhdG9yPC9hPiBvbiB5b3VyIHBob25lIHRvIGFjY2VzcyB5b3VyIHR3byBmYWN0b3J5IHRva2VuPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KVxuXG5tb2R1bGUuZXhwb3J0cyA9IEdvb2dsZUF1dGhJbmZvO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvZ29vZ2xlQXV0aC5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xuXG5jb25zdCBHcnZUYWJsZVRleHRDZWxsID0gKHtyb3dJbmRleCwgZGF0YSwgY29sdW1uS2V5LCAuLi5wcm9wc30pID0+IChcbiAgPEdydlRhYmxlQ2VsbCB7Li4ucHJvcHN9PlxuICAgIHtkYXRhW3Jvd0luZGV4XVtjb2x1bW5LZXldfVxuICA8L0dydlRhYmxlQ2VsbD5cbik7XG5cbnZhciBHcnZUYWJsZUNlbGwgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIHJlbmRlcigpe1xuICAgIHZhciBwcm9wcyA9IHRoaXMucHJvcHM7XG4gICAgcmV0dXJuIHByb3BzLmlzSGVhZGVyID8gPHRoIGtleT17cHJvcHMua2V5fT57cHJvcHMuY2hpbGRyZW59PC90aD4gOiA8dGQga2V5PXtwcm9wcy5rZXl9Pntwcm9wcy5jaGlsZHJlbn08L3RkPjtcbiAgfVxufSk7XG5cbnZhciBHcnZUYWJsZSA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICByZW5kZXJIZWFkZXIoY2hpbGRyZW4pe1xuICAgIHZhciBjZWxscyA9IGNoaWxkcmVuLm1hcCgoaXRlbSwgaW5kZXgpPT57XG4gICAgICByZXR1cm4gdGhpcy5yZW5kZXJDZWxsKGl0ZW0ucHJvcHMuaGVhZGVyLCB7aW5kZXgsIGtleTogaW5kZXgsIGlzSGVhZGVyOiB0cnVlLCAuLi5pdGVtLnByb3BzfSk7XG4gICAgfSlcblxuICAgIHJldHVybiA8dGhlYWQ+PHRyPntjZWxsc308L3RyPjwvdGhlYWQ+XG4gIH0sXG5cbiAgcmVuZGVyQm9keShjaGlsZHJlbil7XG4gICAgdmFyIGNvdW50ID0gdGhpcy5wcm9wcy5yb3dDb3VudDtcbiAgICB2YXIgcm93cyA9IFtdO1xuICAgIGZvcih2YXIgaSA9IDA7IGkgPCBjb3VudDsgaSArKyl7XG4gICAgICB2YXIgY2VsbHMgPSBjaGlsZHJlbi5tYXAoKGl0ZW0sIGluZGV4KT0+e1xuICAgICAgICByZXR1cm4gdGhpcy5yZW5kZXJDZWxsKGl0ZW0ucHJvcHMuY2VsbCwge3Jvd0luZGV4OiBpLCBrZXk6IGluZGV4LCBpc0hlYWRlcjogZmFsc2UsIC4uLml0ZW0ucHJvcHN9KTtcbiAgICAgIH0pXG5cbiAgICAgIHJvd3MucHVzaCg8dHIga2V5PXtpfT57Y2VsbHN9PC90cj4pO1xuICAgIH1cblxuICAgIHJldHVybiA8dGJvZHk+e3Jvd3N9PC90Ym9keT47XG4gIH0sXG5cbiAgcmVuZGVyQ2VsbChjZWxsLCBjZWxsUHJvcHMpe1xuICAgIHZhciBjb250ZW50ID0gbnVsbDtcbiAgICBpZiAoUmVhY3QuaXNWYWxpZEVsZW1lbnQoY2VsbCkpIHtcbiAgICAgICBjb250ZW50ID0gUmVhY3QuY2xvbmVFbGVtZW50KGNlbGwsIGNlbGxQcm9wcyk7XG4gICAgIH0gZWxzZSBpZiAodHlwZW9mIHByb3BzLmNlbGwgPT09ICdmdW5jdGlvbicpIHtcbiAgICAgICBjb250ZW50ID0gY2VsbChjZWxsUHJvcHMpO1xuICAgICB9XG5cbiAgICAgcmV0dXJuIGNvbnRlbnQ7XG4gIH0sXG5cbiAgcmVuZGVyKCkge1xuICAgIHZhciBjaGlsZHJlbiA9IFtdO1xuICAgIFJlYWN0LkNoaWxkcmVuLmZvckVhY2godGhpcy5wcm9wcy5jaGlsZHJlbiwgKGNoaWxkLCBpbmRleCkgPT4ge1xuICAgICAgaWYgKGNoaWxkID09IG51bGwpIHtcbiAgICAgICAgcmV0dXJuO1xuICAgICAgfVxuXG4gICAgICBpZihjaGlsZC50eXBlLmRpc3BsYXlOYW1lICE9PSAnR3J2VGFibGVDb2x1bW4nKXtcbiAgICAgICAgdGhyb3cgJ1Nob3VsZCBiZSBHcnZUYWJsZUNvbHVtbic7XG4gICAgICB9XG5cbiAgICAgIGNoaWxkcmVuLnB1c2goY2hpbGQpO1xuICAgIH0pO1xuXG4gICAgdmFyIHRhYmxlQ2xhc3MgPSAndGFibGUgJyArIHRoaXMucHJvcHMuY2xhc3NOYW1lO1xuXG4gICAgcmV0dXJuIChcbiAgICAgIDx0YWJsZSBjbGFzc05hbWU9e3RhYmxlQ2xhc3N9PlxuICAgICAgICB7dGhpcy5yZW5kZXJIZWFkZXIoY2hpbGRyZW4pfVxuICAgICAgICB7dGhpcy5yZW5kZXJCb2R5KGNoaWxkcmVuKX1cbiAgICAgIDwvdGFibGU+XG4gICAgKTtcbiAgfVxufSlcblxudmFyIEdydlRhYmxlQ29sdW1uID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIHRocm93IG5ldyBFcnJvcignQ29tcG9uZW50IDxHcnZUYWJsZUNvbHVtbiAvPiBzaG91bGQgbmV2ZXIgcmVuZGVyJyk7XG4gIH1cbn0pXG5cbmV4cG9ydCBkZWZhdWx0IEdydlRhYmxlO1xuZXhwb3J0IHtHcnZUYWJsZUNvbHVtbiBhcyBDb2x1bW4sIEdydlRhYmxlIGFzIFRhYmxlLCBHcnZUYWJsZUNlbGwgYXMgQ2VsbCwgR3J2VGFibGVUZXh0Q2VsbCBhcyBUZXh0Q2VsbH07XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy90YWJsZS5qc3hcbiAqKi8iLCIvKlxuICogIFRoZSBNSVQgTGljZW5zZSAoTUlUKVxuICogIENvcHlyaWdodCAoYykgMjAxNSBSeWFuIEZsb3JlbmNlLCBNaWNoYWVsIEphY2tzb25cbiAqICBQZXJtaXNzaW9uIGlzIGhlcmVieSBncmFudGVkLCBmcmVlIG9mIGNoYXJnZSwgdG8gYW55IHBlcnNvbiBvYnRhaW5pbmcgYSBjb3B5IG9mIHRoaXMgc29mdHdhcmUgYW5kIGFzc29jaWF0ZWQgZG9jdW1lbnRhdGlvbiBmaWxlcyAodGhlIFwiU29mdHdhcmVcIiksIHRvIGRlYWwgaW4gdGhlIFNvZnR3YXJlIHdpdGhvdXQgcmVzdHJpY3Rpb24sIGluY2x1ZGluZyB3aXRob3V0IGxpbWl0YXRpb24gdGhlIHJpZ2h0cyB0byB1c2UsIGNvcHksIG1vZGlmeSwgbWVyZ2UsIHB1Ymxpc2gsIGRpc3RyaWJ1dGUsIHN1YmxpY2Vuc2UsIGFuZC9vciBzZWxsIGNvcGllcyBvZiB0aGUgU29mdHdhcmUsIGFuZCB0byBwZXJtaXQgcGVyc29ucyB0byB3aG9tIHRoZSBTb2Z0d2FyZSBpcyBmdXJuaXNoZWQgdG8gZG8gc28sIHN1YmplY3QgdG8gdGhlIGZvbGxvd2luZyBjb25kaXRpb25zOlxuICogIFRoZSBhYm92ZSBjb3B5cmlnaHQgbm90aWNlIGFuZCB0aGlzIHBlcm1pc3Npb24gbm90aWNlIHNoYWxsIGJlIGluY2x1ZGVkIGluIGFsbCBjb3BpZXMgb3Igc3Vic3RhbnRpYWwgcG9ydGlvbnMgb2YgdGhlIFNvZnR3YXJlLlxuICogIFRIRSBTT0ZUV0FSRSBJUyBQUk9WSURFRCBcIkFTIElTXCIsIFdJVEhPVVQgV0FSUkFOVFkgT0YgQU5ZIEtJTkQsIEVYUFJFU1MgT1IgSU1QTElFRCwgSU5DTFVESU5HIEJVVCBOT1QgTElNSVRFRCBUTyBUSEUgV0FSUkFOVElFUyBPRiBNRVJDSEFOVEFCSUxJVFksIEZJVE5FU1MgRk9SIEEgUEFSVElDVUxBUiBQVVJQT1NFIEFORCBOT05JTkZSSU5HRU1FTlQuIElOIE5PIEVWRU5UIFNIQUxMIFRIRSBBVVRIT1JTIE9SIENPUFlSSUdIVCBIT0xERVJTIEJFIExJQUJMRSBGT1IgQU5ZIENMQUlNLCBEQU1BR0VTIE9SIE9USEVSIExJQUJJTElUWSwgV0hFVEhFUiBJTiBBTiBBQ1RJT04gT0YgQ09OVFJBQ1QsIFRPUlQgT1IgT1RIRVJXSVNFLCBBUklTSU5HIEZST00sIE9VVCBPRiBPUiBJTiBDT05ORUNUSU9OIFdJVEggVEhFIFNPRlRXQVJFIE9SIFRIRSBVU0UgT1IgT1RIRVIgREVBTElOR1MgSU4gVEhFIFNPRlRXQVJFLlxuKi9cblxuaW1wb3J0IGludmFyaWFudCBmcm9tICdpbnZhcmlhbnQnXG5cbmZ1bmN0aW9uIGVzY2FwZVJlZ0V4cChzdHJpbmcpIHtcbiAgcmV0dXJuIHN0cmluZy5yZXBsYWNlKC9bLiorP14ke30oKXxbXFxdXFxcXF0vZywgJ1xcXFwkJicpXG59XG5cbmZ1bmN0aW9uIGVzY2FwZVNvdXJjZShzdHJpbmcpIHtcbiAgcmV0dXJuIGVzY2FwZVJlZ0V4cChzdHJpbmcpLnJlcGxhY2UoL1xcLysvZywgJy8rJylcbn1cblxuZnVuY3Rpb24gX2NvbXBpbGVQYXR0ZXJuKHBhdHRlcm4pIHtcbiAgbGV0IHJlZ2V4cFNvdXJjZSA9ICcnO1xuICBjb25zdCBwYXJhbU5hbWVzID0gW107XG4gIGNvbnN0IHRva2VucyA9IFtdO1xuXG4gIGxldCBtYXRjaCwgbGFzdEluZGV4ID0gMCwgbWF0Y2hlciA9IC86KFthLXpBLVpfJF1bYS16QS1aMC05XyRdKil8XFwqXFwqfFxcKnxcXCh8XFwpL2dcbiAgLyplc2xpbnQgbm8tY29uZC1hc3NpZ246IDAqL1xuICB3aGlsZSAoKG1hdGNoID0gbWF0Y2hlci5leGVjKHBhdHRlcm4pKSkge1xuICAgIGlmIChtYXRjaC5pbmRleCAhPT0gbGFzdEluZGV4KSB7XG4gICAgICB0b2tlbnMucHVzaChwYXR0ZXJuLnNsaWNlKGxhc3RJbmRleCwgbWF0Y2guaW5kZXgpKVxuICAgICAgcmVnZXhwU291cmNlICs9IGVzY2FwZVNvdXJjZShwYXR0ZXJuLnNsaWNlKGxhc3RJbmRleCwgbWF0Y2guaW5kZXgpKVxuICAgIH1cblxuICAgIGlmIChtYXRjaFsxXSkge1xuICAgICAgcmVnZXhwU291cmNlICs9ICcoW14vPyNdKyknO1xuICAgICAgcGFyYW1OYW1lcy5wdXNoKG1hdGNoWzFdKTtcbiAgICB9IGVsc2UgaWYgKG1hdGNoWzBdID09PSAnKionKSB7XG4gICAgICByZWdleHBTb3VyY2UgKz0gJyhbXFxcXHNcXFxcU10qKSdcbiAgICAgIHBhcmFtTmFtZXMucHVzaCgnc3BsYXQnKTtcbiAgICB9IGVsc2UgaWYgKG1hdGNoWzBdID09PSAnKicpIHtcbiAgICAgIHJlZ2V4cFNvdXJjZSArPSAnKFtcXFxcc1xcXFxTXSo/KSdcbiAgICAgIHBhcmFtTmFtZXMucHVzaCgnc3BsYXQnKTtcbiAgICB9IGVsc2UgaWYgKG1hdGNoWzBdID09PSAnKCcpIHtcbiAgICAgIHJlZ2V4cFNvdXJjZSArPSAnKD86JztcbiAgICB9IGVsc2UgaWYgKG1hdGNoWzBdID09PSAnKScpIHtcbiAgICAgIHJlZ2V4cFNvdXJjZSArPSAnKT8nO1xuICAgIH1cblxuICAgIHRva2Vucy5wdXNoKG1hdGNoWzBdKTtcblxuICAgIGxhc3RJbmRleCA9IG1hdGNoZXIubGFzdEluZGV4O1xuICB9XG5cbiAgaWYgKGxhc3RJbmRleCAhPT0gcGF0dGVybi5sZW5ndGgpIHtcbiAgICB0b2tlbnMucHVzaChwYXR0ZXJuLnNsaWNlKGxhc3RJbmRleCwgcGF0dGVybi5sZW5ndGgpKVxuICAgIHJlZ2V4cFNvdXJjZSArPSBlc2NhcGVTb3VyY2UocGF0dGVybi5zbGljZShsYXN0SW5kZXgsIHBhdHRlcm4ubGVuZ3RoKSlcbiAgfVxuXG4gIHJldHVybiB7XG4gICAgcGF0dGVybixcbiAgICByZWdleHBTb3VyY2UsXG4gICAgcGFyYW1OYW1lcyxcbiAgICB0b2tlbnNcbiAgfVxufVxuXG5jb25zdCBDb21waWxlZFBhdHRlcm5zQ2FjaGUgPSB7fVxuXG5leHBvcnQgZnVuY3Rpb24gY29tcGlsZVBhdHRlcm4ocGF0dGVybikge1xuICBpZiAoIShwYXR0ZXJuIGluIENvbXBpbGVkUGF0dGVybnNDYWNoZSkpXG4gICAgQ29tcGlsZWRQYXR0ZXJuc0NhY2hlW3BhdHRlcm5dID0gX2NvbXBpbGVQYXR0ZXJuKHBhdHRlcm4pXG5cbiAgcmV0dXJuIENvbXBpbGVkUGF0dGVybnNDYWNoZVtwYXR0ZXJuXVxufVxuXG4vKipcbiAqIEF0dGVtcHRzIHRvIG1hdGNoIGEgcGF0dGVybiBvbiB0aGUgZ2l2ZW4gcGF0aG5hbWUuIFBhdHRlcm5zIG1heSB1c2VcbiAqIHRoZSBmb2xsb3dpbmcgc3BlY2lhbCBjaGFyYWN0ZXJzOlxuICpcbiAqIC0gOnBhcmFtTmFtZSAgICAgTWF0Y2hlcyBhIFVSTCBzZWdtZW50IHVwIHRvIHRoZSBuZXh0IC8sID8sIG9yICMuIFRoZVxuICogICAgICAgICAgICAgICAgICBjYXB0dXJlZCBzdHJpbmcgaXMgY29uc2lkZXJlZCBhIFwicGFyYW1cIlxuICogLSAoKSAgICAgICAgICAgICBXcmFwcyBhIHNlZ21lbnQgb2YgdGhlIFVSTCB0aGF0IGlzIG9wdGlvbmFsXG4gKiAtICogICAgICAgICAgICAgIENvbnN1bWVzIChub24tZ3JlZWR5KSBhbGwgY2hhcmFjdGVycyB1cCB0byB0aGUgbmV4dFxuICogICAgICAgICAgICAgICAgICBjaGFyYWN0ZXIgaW4gdGhlIHBhdHRlcm4sIG9yIHRvIHRoZSBlbmQgb2YgdGhlIFVSTCBpZlxuICogICAgICAgICAgICAgICAgICB0aGVyZSBpcyBub25lXG4gKiAtICoqICAgICAgICAgICAgIENvbnN1bWVzIChncmVlZHkpIGFsbCBjaGFyYWN0ZXJzIHVwIHRvIHRoZSBuZXh0IGNoYXJhY3RlclxuICogICAgICAgICAgICAgICAgICBpbiB0aGUgcGF0dGVybiwgb3IgdG8gdGhlIGVuZCBvZiB0aGUgVVJMIGlmIHRoZXJlIGlzIG5vbmVcbiAqXG4gKiBUaGUgcmV0dXJuIHZhbHVlIGlzIGFuIG9iamVjdCB3aXRoIHRoZSBmb2xsb3dpbmcgcHJvcGVydGllczpcbiAqXG4gKiAtIHJlbWFpbmluZ1BhdGhuYW1lXG4gKiAtIHBhcmFtTmFtZXNcbiAqIC0gcGFyYW1WYWx1ZXNcbiAqL1xuZXhwb3J0IGZ1bmN0aW9uIG1hdGNoUGF0dGVybihwYXR0ZXJuLCBwYXRobmFtZSkge1xuICAvLyBNYWtlIGxlYWRpbmcgc2xhc2hlcyBjb25zaXN0ZW50IGJldHdlZW4gcGF0dGVybiBhbmQgcGF0aG5hbWUuXG4gIGlmIChwYXR0ZXJuLmNoYXJBdCgwKSAhPT0gJy8nKSB7XG4gICAgcGF0dGVybiA9IGAvJHtwYXR0ZXJufWBcbiAgfVxuICBpZiAocGF0aG5hbWUuY2hhckF0KDApICE9PSAnLycpIHtcbiAgICBwYXRobmFtZSA9IGAvJHtwYXRobmFtZX1gXG4gIH1cblxuICBsZXQgeyByZWdleHBTb3VyY2UsIHBhcmFtTmFtZXMsIHRva2VucyB9ID0gY29tcGlsZVBhdHRlcm4ocGF0dGVybilcblxuICByZWdleHBTb3VyY2UgKz0gJy8qJyAvLyBDYXB0dXJlIHBhdGggc2VwYXJhdG9yc1xuXG4gIC8vIFNwZWNpYWwtY2FzZSBwYXR0ZXJucyBsaWtlICcqJyBmb3IgY2F0Y2gtYWxsIHJvdXRlcy5cbiAgY29uc3QgY2FwdHVyZVJlbWFpbmluZyA9IHRva2Vuc1t0b2tlbnMubGVuZ3RoIC0gMV0gIT09ICcqJ1xuXG4gIGlmIChjYXB0dXJlUmVtYWluaW5nKSB7XG4gICAgLy8gVGhpcyB3aWxsIG1hdGNoIG5ld2xpbmVzIGluIHRoZSByZW1haW5pbmcgcGF0aC5cbiAgICByZWdleHBTb3VyY2UgKz0gJyhbXFxcXHNcXFxcU10qPyknXG4gIH1cblxuICBjb25zdCBtYXRjaCA9IHBhdGhuYW1lLm1hdGNoKG5ldyBSZWdFeHAoJ14nICsgcmVnZXhwU291cmNlICsgJyQnLCAnaScpKVxuXG4gIGxldCByZW1haW5pbmdQYXRobmFtZSwgcGFyYW1WYWx1ZXNcbiAgaWYgKG1hdGNoICE9IG51bGwpIHtcbiAgICBpZiAoY2FwdHVyZVJlbWFpbmluZykge1xuICAgICAgcmVtYWluaW5nUGF0aG5hbWUgPSBtYXRjaC5wb3AoKVxuICAgICAgY29uc3QgbWF0Y2hlZFBhdGggPVxuICAgICAgICBtYXRjaFswXS5zdWJzdHIoMCwgbWF0Y2hbMF0ubGVuZ3RoIC0gcmVtYWluaW5nUGF0aG5hbWUubGVuZ3RoKVxuXG4gICAgICAvLyBJZiB3ZSBkaWRuJ3QgbWF0Y2ggdGhlIGVudGlyZSBwYXRobmFtZSwgdGhlbiBtYWtlIHN1cmUgdGhhdCB0aGUgbWF0Y2hcbiAgICAgIC8vIHdlIGRpZCBnZXQgZW5kcyBhdCBhIHBhdGggc2VwYXJhdG9yIChwb3RlbnRpYWxseSB0aGUgb25lIHdlIGFkZGVkXG4gICAgICAvLyBhYm92ZSBhdCB0aGUgYmVnaW5uaW5nIG9mIHRoZSBwYXRoLCBpZiB0aGUgYWN0dWFsIG1hdGNoIHdhcyBlbXB0eSkuXG4gICAgICBpZiAoXG4gICAgICAgIHJlbWFpbmluZ1BhdGhuYW1lICYmXG4gICAgICAgIG1hdGNoZWRQYXRoLmNoYXJBdChtYXRjaGVkUGF0aC5sZW5ndGggLSAxKSAhPT0gJy8nXG4gICAgICApIHtcbiAgICAgICAgcmV0dXJuIHtcbiAgICAgICAgICByZW1haW5pbmdQYXRobmFtZTogbnVsbCxcbiAgICAgICAgICBwYXJhbU5hbWVzLFxuICAgICAgICAgIHBhcmFtVmFsdWVzOiBudWxsXG4gICAgICAgIH1cbiAgICAgIH1cbiAgICB9IGVsc2Uge1xuICAgICAgLy8gSWYgdGhpcyBtYXRjaGVkIGF0IGFsbCwgdGhlbiB0aGUgbWF0Y2ggd2FzIHRoZSBlbnRpcmUgcGF0aG5hbWUuXG4gICAgICByZW1haW5pbmdQYXRobmFtZSA9ICcnXG4gICAgfVxuXG4gICAgcGFyYW1WYWx1ZXMgPSBtYXRjaC5zbGljZSgxKS5tYXAoXG4gICAgICB2ID0+IHYgIT0gbnVsbCA/IGRlY29kZVVSSUNvbXBvbmVudCh2KSA6IHZcbiAgICApXG4gIH0gZWxzZSB7XG4gICAgcmVtYWluaW5nUGF0aG5hbWUgPSBwYXJhbVZhbHVlcyA9IG51bGxcbiAgfVxuXG4gIHJldHVybiB7XG4gICAgcmVtYWluaW5nUGF0aG5hbWUsXG4gICAgcGFyYW1OYW1lcyxcbiAgICBwYXJhbVZhbHVlc1xuICB9XG59XG5cbmV4cG9ydCBmdW5jdGlvbiBnZXRQYXJhbU5hbWVzKHBhdHRlcm4pIHtcbiAgcmV0dXJuIGNvbXBpbGVQYXR0ZXJuKHBhdHRlcm4pLnBhcmFtTmFtZXNcbn1cblxuZXhwb3J0IGZ1bmN0aW9uIGdldFBhcmFtcyhwYXR0ZXJuLCBwYXRobmFtZSkge1xuICBjb25zdCB7IHBhcmFtTmFtZXMsIHBhcmFtVmFsdWVzIH0gPSBtYXRjaFBhdHRlcm4ocGF0dGVybiwgcGF0aG5hbWUpXG5cbiAgaWYgKHBhcmFtVmFsdWVzICE9IG51bGwpIHtcbiAgICByZXR1cm4gcGFyYW1OYW1lcy5yZWR1Y2UoZnVuY3Rpb24gKG1lbW8sIHBhcmFtTmFtZSwgaW5kZXgpIHtcbiAgICAgIG1lbW9bcGFyYW1OYW1lXSA9IHBhcmFtVmFsdWVzW2luZGV4XVxuICAgICAgcmV0dXJuIG1lbW9cbiAgICB9LCB7fSlcbiAgfVxuXG4gIHJldHVybiBudWxsXG59XG5cbi8qKlxuICogUmV0dXJucyBhIHZlcnNpb24gb2YgdGhlIGdpdmVuIHBhdHRlcm4gd2l0aCBwYXJhbXMgaW50ZXJwb2xhdGVkLiBUaHJvd3NcbiAqIGlmIHRoZXJlIGlzIGEgZHluYW1pYyBzZWdtZW50IG9mIHRoZSBwYXR0ZXJuIGZvciB3aGljaCB0aGVyZSBpcyBubyBwYXJhbS5cbiAqL1xuZXhwb3J0IGZ1bmN0aW9uIGZvcm1hdFBhdHRlcm4ocGF0dGVybiwgcGFyYW1zKSB7XG4gIHBhcmFtcyA9IHBhcmFtcyB8fCB7fVxuXG4gIGNvbnN0IHsgdG9rZW5zIH0gPSBjb21waWxlUGF0dGVybihwYXR0ZXJuKVxuICBsZXQgcGFyZW5Db3VudCA9IDAsIHBhdGhuYW1lID0gJycsIHNwbGF0SW5kZXggPSAwXG5cbiAgbGV0IHRva2VuLCBwYXJhbU5hbWUsIHBhcmFtVmFsdWVcbiAgZm9yIChsZXQgaSA9IDAsIGxlbiA9IHRva2Vucy5sZW5ndGg7IGkgPCBsZW47ICsraSkge1xuICAgIHRva2VuID0gdG9rZW5zW2ldXG5cbiAgICBpZiAodG9rZW4gPT09ICcqJyB8fCB0b2tlbiA9PT0gJyoqJykge1xuICAgICAgcGFyYW1WYWx1ZSA9IEFycmF5LmlzQXJyYXkocGFyYW1zLnNwbGF0KSA/IHBhcmFtcy5zcGxhdFtzcGxhdEluZGV4KytdIDogcGFyYW1zLnNwbGF0XG5cbiAgICAgIGludmFyaWFudChcbiAgICAgICAgcGFyYW1WYWx1ZSAhPSBudWxsIHx8IHBhcmVuQ291bnQgPiAwLFxuICAgICAgICAnTWlzc2luZyBzcGxhdCAjJXMgZm9yIHBhdGggXCIlc1wiJyxcbiAgICAgICAgc3BsYXRJbmRleCwgcGF0dGVyblxuICAgICAgKVxuXG4gICAgICBpZiAocGFyYW1WYWx1ZSAhPSBudWxsKVxuICAgICAgICBwYXRobmFtZSArPSBlbmNvZGVVUkkocGFyYW1WYWx1ZSlcbiAgICB9IGVsc2UgaWYgKHRva2VuID09PSAnKCcpIHtcbiAgICAgIHBhcmVuQ291bnQgKz0gMVxuICAgIH0gZWxzZSBpZiAodG9rZW4gPT09ICcpJykge1xuICAgICAgcGFyZW5Db3VudCAtPSAxXG4gICAgfSBlbHNlIGlmICh0b2tlbi5jaGFyQXQoMCkgPT09ICc6Jykge1xuICAgICAgcGFyYW1OYW1lID0gdG9rZW4uc3Vic3RyaW5nKDEpXG4gICAgICBwYXJhbVZhbHVlID0gcGFyYW1zW3BhcmFtTmFtZV1cblxuICAgICAgaW52YXJpYW50KFxuICAgICAgICBwYXJhbVZhbHVlICE9IG51bGwgfHwgcGFyZW5Db3VudCA+IDAsXG4gICAgICAgICdNaXNzaW5nIFwiJXNcIiBwYXJhbWV0ZXIgZm9yIHBhdGggXCIlc1wiJyxcbiAgICAgICAgcGFyYW1OYW1lLCBwYXR0ZXJuXG4gICAgICApXG5cbiAgICAgIGlmIChwYXJhbVZhbHVlICE9IG51bGwpXG4gICAgICAgIHBhdGhuYW1lICs9IGVuY29kZVVSSUNvbXBvbmVudChwYXJhbVZhbHVlKVxuICAgIH0gZWxzZSB7XG4gICAgICBwYXRobmFtZSArPSB0b2tlblxuICAgIH1cbiAgfVxuXG4gIHJldHVybiBwYXRobmFtZS5yZXBsYWNlKC9cXC8rL2csICcvJylcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21tb24vcGF0dGVyblV0aWxzLmpzXG4gKiovIiwidmFyIEV2ZW50RW1pdHRlciA9IHJlcXVpcmUoJ2V2ZW50cycpLkV2ZW50RW1pdHRlcjtcbnZhciBzZXNzaW9uID0gcmVxdWlyZSgnYXBwL3Nlc3Npb24nKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG52YXIge2FjdGlvbnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvJyk7XG5cbmNsYXNzIFR0eSBleHRlbmRzIEV2ZW50RW1pdHRlciB7XG5cbiAgY29uc3RydWN0b3Ioe3NlcnZlcklkLCBsb2dpbiwgc2lkLCByb3dzLCBjb2xzIH0pe1xuICAgIHN1cGVyKCk7XG4gICAgdGhpcy5vcHRpb25zID0geyBzZXJ2ZXJJZCwgbG9naW4sIHNpZCwgcm93cywgY29scyB9O1xuICAgIHRoaXMuc29ja2V0ID0gbnVsbDtcbiAgfVxuXG4gIGRpc2Nvbm5lY3QoKXtcbiAgICB0aGlzLnNvY2tldC5jbG9zZSgpO1xuICB9XG5cbiAgY29ubmVjdChvcHRpb25zKXtcbiAgICBPYmplY3QuYXNzaWduKHRoaXMub3B0aW9ucywgb3B0aW9ucyk7XG5cbiAgICBsZXQge3Rva2VufSA9IHNlc3Npb24uZ2V0VXNlckRhdGEoKTtcbiAgICBsZXQgY29ublN0ciA9IGNmZy5hcGkuZ2V0VHR5Q29ublN0cih7dG9rZW4sIC4uLnRoaXMub3B0aW9uc30pO1xuXG4gICAgdGhpcy5zb2NrZXQgPSBuZXcgV2ViU29ja2V0KGNvbm5TdHIsICdwcm90bycpO1xuXG4gICAgdGhpcy5zb2NrZXQub25vcGVuID0gKCkgPT4ge1xuICAgICAgdGhpcy5lbWl0KCdvcGVuJyk7XG4gICAgfVxuXG4gICAgdGhpcy5zb2NrZXQub25tZXNzYWdlID0gKGUpPT57XG4gICAgICB0aGlzLmVtaXQoJ2RhdGEnLCBlLmRhdGEpO1xuICAgIH1cblxuICAgIHRoaXMuc29ja2V0Lm9uY2xvc2UgPSAoKT0+e1xuICAgICAgdGhpcy5lbWl0KCdjbG9zZScpO1xuICAgIH1cbiAgfVxuXG4gIHJlc2l6ZShjb2xzLCByb3dzKXtcbiAgICBhY3Rpb25zLnJlc2l6ZShjb2xzLCByb3dzKTtcbiAgfVxuXG4gIHNlbmQoZGF0YSl7XG4gICAgdGhpcy5zb2NrZXQuc2VuZChkYXRhKTtcbiAgfVxufVxuXG5tb2R1bGUuZXhwb3J0cyA9IFR0eTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21tb24vdHR5LmpzXG4gKiovIiwidmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIGFwaSA9IHJlcXVpcmUoJ2FwcC9zZXJ2aWNlcy9hcGknKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG5cbnZhciB7IFRMUFRfU0VTU0lOU19SRUNFSVZFIH0gID0gcmVxdWlyZSgnLi9zZXNzaW9ucy9hY3Rpb25UeXBlcycpO1xudmFyIHsgVExQVF9OT0RFU19SRUNFSVZFIH0gID0gcmVxdWlyZSgnLi9ub2Rlcy9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCB7XG4gIGZldGNoTm9kZXNBbmRTZXNzaW9ucygpe1xuICAgIGFwaS5nZXQoY2ZnLmFwaS5ub2Rlc1BhdGgpLmRvbmUoanNvbj0+e1xuICAgICAgdmFyIG5vZGVBcnJheSA9IFtdO1xuICAgICAgdmFyIHNlc3Npb25zID0ge307XG5cbiAgICAgIGpzb24ubm9kZXMuZm9yRWFjaChpdGVtPT4ge1xuICAgICAgICBub2RlQXJyYXkucHVzaChpdGVtLm5vZGUpO1xuICAgICAgICBpZihpdGVtLnNlc3Npb25zKXtcbiAgICAgICAgICBpdGVtLnNlc3Npb25zLmZvckVhY2goaXRlbTI9PntcbiAgICAgICAgICAgIHNlc3Npb25zW2l0ZW0yLmlkXSA9IGl0ZW0yO1xuICAgICAgICAgIH0pXG4gICAgICAgIH1cbiAgICAgIH0pO1xuXG4gICAgICByZWFjdG9yLmJhdGNoKCgpID0+IHtcbiAgICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX05PREVTX1JFQ0VJVkUsIG5vZGVBcnJheSk7XG4gICAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9TRVNTSU5TX1JFQ0VJVkUsIHNlc3Npb25zKTtcbiAgICAgIH0pO1xuXG4gICAgfSk7XG4gIH1cbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2FjdGlvbnMuanNcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG5yZWFjdG9yLnJlZ2lzdGVyU3RvcmVzKHtcbiAgJ3RscHRfYWN0aXZlX3Rlcm1pbmFsJzogcmVxdWlyZSgnLi9hY3RpdmVUZXJtaW5hbC9hY3RpdmVUZXJtU3RvcmUnKSxcbiAgJ3RscHRfdXNlcic6IHJlcXVpcmUoJy4vdXNlci91c2VyU3RvcmUnKSxcbiAgJ3RscHRfbm9kZXMnOiByZXF1aXJlKCcuL25vZGVzL25vZGVTdG9yZScpLFxuICAndGxwdF9pbnZpdGUnOiByZXF1aXJlKCcuL2ludml0ZS9pbnZpdGVTdG9yZScpLFxuICAndGxwdF9yZXN0X2FwaSc6IHJlcXVpcmUoJy4vcmVzdEFwaS9yZXN0QXBpU3RvcmUnKSxcbiAgJ3RscHRfc2Vzc2lvbnMnOiByZXF1aXJlKCcuL3Nlc3Npb25zL3Nlc3Npb25TdG9yZScpICBcbn0pO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvaW5kZXguanNcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgeyBUTFBUX1JFQ0VJVkVfVVNFUl9JTlZJVEUgfSAgPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG52YXIgYXBpID0gcmVxdWlyZSgnYXBwL3NlcnZpY2VzL2FwaScpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICBmZXRjaEludml0ZShpbnZpdGVUb2tlbil7XG4gICAgdmFyIHBhdGggPSBjZmcuYXBpLmdldEludml0ZVVybChpbnZpdGVUb2tlbik7XG4gICAgYXBpLmdldChwYXRoKS5kb25lKGludml0ZT0+e1xuICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFQ0VJVkVfVVNFUl9JTlZJVEUsIGludml0ZSk7XG4gICAgfSk7XG4gIH1cbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9hY3Rpb25zLmpzXG4gKiovIiwiLyplc2xpbnQgbm8tdW5kZWY6IDAsICBuby11bnVzZWQtdmFyczogMCwgbm8tZGVidWdnZXI6MCovXG5cbnZhciB7VFJZSU5HX1RPX1NJR05fVVB9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvcmVzdEFwaS9jb25zdGFudHMnKTtcblxuY29uc3QgaW52aXRlID0gWyBbJ3RscHRfaW52aXRlJ10sIChpbnZpdGUpID0+IHtcbiAgcmV0dXJuIGludml0ZTtcbiB9XG5dO1xuXG5jb25zdCBhdHRlbXAgPSBbIFsndGxwdF9yZXN0X2FwaScsIFRSWUlOR19UT19TSUdOX1VQXSwgKGF0dGVtcCkgPT4ge1xuICB2YXIgZGVmYXVsdE9iaiA9IHtcbiAgICBpc1Byb2Nlc3Npbmc6IGZhbHNlLFxuICAgIGlzRXJyb3I6IGZhbHNlLFxuICAgIGlzU3VjY2VzczogZmFsc2UsXG4gICAgbWVzc2FnZTogJydcbiAgfVxuXG4gIHJldHVybiBhdHRlbXAgPyBhdHRlbXAudG9KUygpIDogZGVmYXVsdE9iajtcbiAgXG4gfVxuXTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICBpbnZpdGUsXG4gIGF0dGVtcFxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2dldHRlcnMuanNcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cy5nZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3Rpb25zID0gcmVxdWlyZSgnLi9hY3Rpb25zJyk7XG5tb2R1bGUuZXhwb3J0cy5ub2RlU3RvcmUgPSByZXF1aXJlKCcuL2ludml0ZVN0b3JlJyk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvaW5kZXguanNcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgeyBUTFBUX05PREVTX1JFQ0VJVkUgfSAgPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG52YXIgYXBpID0gcmVxdWlyZSgnYXBwL3NlcnZpY2VzL2FwaScpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICBmZXRjaE5vZGVzKCl7XG4gICAgYXBpLmdldChjZmcuYXBpLm5vZGVzUGF0aCkuZG9uZShkYXRhPT57XG4gICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfTk9ERVNfUkVDRUlWRSwgZGF0YS5ub2Rlcyk7XG4gICAgfSk7XG4gIH1cbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2FjdGlvbnMuanNcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIge3Nlc3Npb25zQnlTZXJ2ZXJ9ID0gcmVxdWlyZSgnLi8uLi9zZXNzaW9ucy9nZXR0ZXJzJyk7XG5cbmNvbnN0IG5vZGVMaXN0VmlldyA9IFsgWyd0bHB0X25vZGVzJ10sIChub2RlcykgPT57XG4gICAgcmV0dXJuIG5vZGVzLm1hcCgoaXRlbSk9PntcbiAgICAgIHZhciBzZXJ2ZXJJZCA9IGl0ZW0uZ2V0KCdpZCcpO1xuICAgICAgdmFyIHNlc3Npb25zID0gcmVhY3Rvci5ldmFsdWF0ZShzZXNzaW9uc0J5U2VydmVyKHNlcnZlcklkKSk7XG4gICAgICByZXR1cm4ge1xuICAgICAgICBpZDogc2VydmVySWQsXG4gICAgICAgIGhvc3RuYW1lOiBpdGVtLmdldCgnaG9zdG5hbWUnKSxcbiAgICAgICAgdGFnczogZ2V0VGFncyhpdGVtKSxcbiAgICAgICAgYWRkcjogaXRlbS5nZXQoJ2FkZHInKSxcbiAgICAgICAgc2Vzc2lvbkNvdW50OiBzZXNzaW9ucy5zaXplXG4gICAgICB9XG4gICAgfSkudG9KUygpO1xuIH1cbl07XG5cbmZ1bmN0aW9uIGdldFRhZ3Mobm9kZSl7XG4gIHZhciBhbGxMYWJlbHMgPSBbXTtcbiAgdmFyIGxhYmVscyA9IG5vZGUuZ2V0KCdsYWJlbHMnKTtcblxuICBpZihsYWJlbHMpe1xuICAgIGxhYmVscy5lbnRyeVNlcSgpLnRvQXJyYXkoKS5mb3JFYWNoKGl0ZW09PntcbiAgICAgIGFsbExhYmVscy5wdXNoKHtcbiAgICAgICAgcm9sZTogaXRlbVswXSxcbiAgICAgICAgdmFsdWU6IGl0ZW1bMV1cbiAgICAgIH0pO1xuICAgIH0pO1xuICB9XG5cbiAgbGFiZWxzID0gbm9kZS5nZXQoJ2NtZF9sYWJlbHMnKTtcblxuICBpZihsYWJlbHMpe1xuICAgIGxhYmVscy5lbnRyeVNlcSgpLnRvQXJyYXkoKS5mb3JFYWNoKGl0ZW09PntcbiAgICAgIGFsbExhYmVscy5wdXNoKHtcbiAgICAgICAgcm9sZTogaXRlbVswXSxcbiAgICAgICAgdmFsdWU6IGl0ZW1bMV0uZ2V0KCdyZXN1bHQnKSxcbiAgICAgICAgdG9vbHRpcDogaXRlbVsxXS5nZXQoJ2NvbW1hbmQnKVxuICAgICAgfSk7XG4gICAgfSk7XG4gIH1cblxuICByZXR1cm4gYWxsTGFiZWxzO1xufVxuXG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgbm9kZUxpc3RWaWV3XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9nZXR0ZXJzLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMuZ2V0dGVycyA9IHJlcXVpcmUoJy4vZ2V0dGVycycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aW9ucyA9IHJlcXVpcmUoJy4vYWN0aW9ucycpO1xubW9kdWxlLmV4cG9ydHMubm9kZVN0b3JlID0gcmVxdWlyZSgnLi9ub2RlU3RvcmUnKTtcblxuLy8gbm9kZXM6IFt7XCJpZFwiOlwieDIyMFwiLFwiYWRkclwiOlwiMC4wLjAuMDozMDIyXCIsXCJob3N0bmFtZVwiOlwieDIyMFwiLFwibGFiZWxzXCI6bnVsbCxcImNtZF9sYWJlbHNcIjpudWxsfV1cblxuXG4vLyBzZXNzaW9uczogW3tcImlkXCI6XCIwNzYzMDYzNi1iYjNkLTQwZTEtYjA4Ni02MGIyY2FlMjFhYzRcIixcInBhcnRpZXNcIjpbe1wiaWRcIjpcIjg5Zjc2MmEzLTc0MjktNGM3YS1hOTEzLTc2NjQ5M2ZlN2M4YVwiLFwic2l0ZVwiOlwiMTI3LjAuMC4xOjM3NTE0XCIsXCJ1c2VyXCI6XCJha29udHNldm95XCIsXCJzZXJ2ZXJfYWRkclwiOlwiMC4wLjAuMDozMDIyXCIsXCJsYXN0X2FjdGl2ZVwiOlwiMjAxNi0wMi0yMlQxNDozOToyMC45MzEyMDUzNS0wNTowMFwifV19XVxuXG4vKlxubGV0IFRvZG9SZWNvcmQgPSBJbW11dGFibGUuUmVjb3JkKHtcbiAgICBpZDogMCxcbiAgICBkZXNjcmlwdGlvbjogXCJcIixcbiAgICBjb21wbGV0ZWQ6IGZhbHNlXG59KTtcbiovXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9pbmRleC5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcblxudmFyIHtcbiAgVExQVF9SRVNUX0FQSV9TVEFSVCxcbiAgVExQVF9SRVNUX0FQSV9TVUNDRVNTLFxuICBUTFBUX1JFU1RfQVBJX0ZBSUwgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuXG4gIHN0YXJ0KHJlcVR5cGUpe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRVNUX0FQSV9TVEFSVCwge3R5cGU6IHJlcVR5cGV9KTtcbiAgfSxcblxuICBmYWlsKHJlcVR5cGUsIG1lc3NhZ2Upe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRVNUX0FQSV9GQUlMLCAge3R5cGU6IHJlcVR5cGUsIG1lc3NhZ2V9KTtcbiAgfSxcblxuICBzdWNjZXNzKHJlcVR5cGUpe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRVNUX0FQSV9TVUNDRVNTLCB7dHlwZTogcmVxVHlwZX0pO1xuICB9XG5cbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvYWN0aW9ucy5qc1xuICoqLyIsInZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyIHtcbiAgVExQVF9SRVNUX0FQSV9TVEFSVCxcbiAgVExQVF9SRVNUX0FQSV9TVUNDRVNTLFxuICBUTFBUX1JFU1RfQVBJX0ZBSUwgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKHt9KTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9SRVNUX0FQSV9TVEFSVCwgc3RhcnQpO1xuICAgIHRoaXMub24oVExQVF9SRVNUX0FQSV9GQUlMLCBmYWlsKTtcbiAgICB0aGlzLm9uKFRMUFRfUkVTVF9BUElfU1VDQ0VTUywgc3VjY2Vzcyk7XG4gIH1cbn0pXG5cbmZ1bmN0aW9uIHN0YXJ0KHN0YXRlLCByZXF1ZXN0KXtcbiAgcmV0dXJuIHN0YXRlLnNldChyZXF1ZXN0LnR5cGUsIHRvSW1tdXRhYmxlKHtpc1Byb2Nlc3Npbmc6IHRydWV9KSk7XG59XG5cbmZ1bmN0aW9uIGZhaWwoc3RhdGUsIHJlcXVlc3Qpe1xuICByZXR1cm4gc3RhdGUuc2V0KHJlcXVlc3QudHlwZSwgdG9JbW11dGFibGUoe2lzRmFpbGVkOiB0cnVlLCBtZXNzYWdlOiByZXF1ZXN0Lm1lc3NhZ2V9KSk7XG59XG5cbmZ1bmN0aW9uIHN1Y2Nlc3Moc3RhdGUsIHJlcXVlc3Qpe1xuICByZXR1cm4gc3RhdGUuc2V0KHJlcXVlc3QudHlwZSwgdG9JbW11dGFibGUoe2lzU3VjY2VzczogdHJ1ZX0pKTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvcmVzdEFwaVN0b3JlLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMuZ2V0dGVycyA9IHJlcXVpcmUoJy4vZ2V0dGVycycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aW9ucyA9IHJlcXVpcmUoJy4vYWN0aW9ucycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aXZlVGVybVN0b3JlID0gcmVxdWlyZSgnLi9zZXNzaW9uU3RvcmUnKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2luZGV4LmpzXG4gKiovIiwidmFyIHV0aWxzID0ge1xuXG4gIHV1aWQoKXtcbiAgICAvLyBuZXZlciB1c2UgaXQgaW4gcHJvZHVjdGlvblxuICAgIHJldHVybiAneHh4eHh4eHgteHh4eC00eHh4LXl4eHgteHh4eHh4eHh4eHh4Jy5yZXBsYWNlKC9beHldL2csIGZ1bmN0aW9uKGMpIHtcbiAgICAgIHZhciByID0gTWF0aC5yYW5kb20oKSoxNnwwLCB2ID0gYyA9PSAneCcgPyByIDogKHImMHgzfDB4OCk7XG4gICAgICByZXR1cm4gdi50b1N0cmluZygxNik7XG4gICAgfSk7XG4gIH0sXG5cbiAgZGlzcGxheURhdGUoZGF0ZSl7XG4gICAgdHJ5e1xuICAgICAgcmV0dXJuIGRhdGUudG9Mb2NhbGVEYXRlU3RyaW5nKCkgKyAnICcgKyBkYXRlLnRvTG9jYWxlVGltZVN0cmluZygpO1xuICAgIH1jYXRjaChlcnIpe1xuICAgICAgY29uc29sZS5lcnJvcihlcnIpO1xuICAgICAgcmV0dXJuICd1bmRlZmluZWQnO1xuICAgIH1cbiAgfSxcblxuICBmb3JtYXRTdHJpbmcoZm9ybWF0KSB7XG4gICAgdmFyIGFyZ3MgPSBBcnJheS5wcm90b3R5cGUuc2xpY2UuY2FsbChhcmd1bWVudHMsIDEpO1xuICAgIHJldHVybiBmb3JtYXQucmVwbGFjZShuZXcgUmVnRXhwKCdcXFxceyhcXFxcZCspXFxcXH0nLCAnZycpLFxuICAgICAgKG1hdGNoLCBudW1iZXIpID0+IHtcbiAgICAgICAgcmV0dXJuICEoYXJnc1tudW1iZXJdID09PSBudWxsIHx8IGFyZ3NbbnVtYmVyXSA9PT0gdW5kZWZpbmVkKSA/IGFyZ3NbbnVtYmVyXSA6ICcnO1xuICAgIH0pO1xuICB9XG4gICAgICAgICAgICBcbn1cblxubW9kdWxlLmV4cG9ydHMgPSB1dGlscztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC91dGlscy5qc1xuICoqLyIsIi8vIENvcHlyaWdodCBKb3llbnQsIEluYy4gYW5kIG90aGVyIE5vZGUgY29udHJpYnV0b3JzLlxuLy9cbi8vIFBlcm1pc3Npb24gaXMgaGVyZWJ5IGdyYW50ZWQsIGZyZWUgb2YgY2hhcmdlLCB0byBhbnkgcGVyc29uIG9idGFpbmluZyBhXG4vLyBjb3B5IG9mIHRoaXMgc29mdHdhcmUgYW5kIGFzc29jaWF0ZWQgZG9jdW1lbnRhdGlvbiBmaWxlcyAodGhlXG4vLyBcIlNvZnR3YXJlXCIpLCB0byBkZWFsIGluIHRoZSBTb2Z0d2FyZSB3aXRob3V0IHJlc3RyaWN0aW9uLCBpbmNsdWRpbmdcbi8vIHdpdGhvdXQgbGltaXRhdGlvbiB0aGUgcmlnaHRzIHRvIHVzZSwgY29weSwgbW9kaWZ5LCBtZXJnZSwgcHVibGlzaCxcbi8vIGRpc3RyaWJ1dGUsIHN1YmxpY2Vuc2UsIGFuZC9vciBzZWxsIGNvcGllcyBvZiB0aGUgU29mdHdhcmUsIGFuZCB0byBwZXJtaXRcbi8vIHBlcnNvbnMgdG8gd2hvbSB0aGUgU29mdHdhcmUgaXMgZnVybmlzaGVkIHRvIGRvIHNvLCBzdWJqZWN0IHRvIHRoZVxuLy8gZm9sbG93aW5nIGNvbmRpdGlvbnM6XG4vL1xuLy8gVGhlIGFib3ZlIGNvcHlyaWdodCBub3RpY2UgYW5kIHRoaXMgcGVybWlzc2lvbiBub3RpY2Ugc2hhbGwgYmUgaW5jbHVkZWRcbi8vIGluIGFsbCBjb3BpZXMgb3Igc3Vic3RhbnRpYWwgcG9ydGlvbnMgb2YgdGhlIFNvZnR3YXJlLlxuLy9cbi8vIFRIRSBTT0ZUV0FSRSBJUyBQUk9WSURFRCBcIkFTIElTXCIsIFdJVEhPVVQgV0FSUkFOVFkgT0YgQU5ZIEtJTkQsIEVYUFJFU1Ncbi8vIE9SIElNUExJRUQsIElOQ0xVRElORyBCVVQgTk9UIExJTUlURUQgVE8gVEhFIFdBUlJBTlRJRVMgT0Zcbi8vIE1FUkNIQU5UQUJJTElUWSwgRklUTkVTUyBGT1IgQSBQQVJUSUNVTEFSIFBVUlBPU0UgQU5EIE5PTklORlJJTkdFTUVOVC4gSU5cbi8vIE5PIEVWRU5UIFNIQUxMIFRIRSBBVVRIT1JTIE9SIENPUFlSSUdIVCBIT0xERVJTIEJFIExJQUJMRSBGT1IgQU5ZIENMQUlNLFxuLy8gREFNQUdFUyBPUiBPVEhFUiBMSUFCSUxJVFksIFdIRVRIRVIgSU4gQU4gQUNUSU9OIE9GIENPTlRSQUNULCBUT1JUIE9SXG4vLyBPVEhFUldJU0UsIEFSSVNJTkcgRlJPTSwgT1VUIE9GIE9SIElOIENPTk5FQ1RJT04gV0lUSCBUSEUgU09GVFdBUkUgT1IgVEhFXG4vLyBVU0UgT1IgT1RIRVIgREVBTElOR1MgSU4gVEhFIFNPRlRXQVJFLlxuXG5mdW5jdGlvbiBFdmVudEVtaXR0ZXIoKSB7XG4gIHRoaXMuX2V2ZW50cyA9IHRoaXMuX2V2ZW50cyB8fCB7fTtcbiAgdGhpcy5fbWF4TGlzdGVuZXJzID0gdGhpcy5fbWF4TGlzdGVuZXJzIHx8IHVuZGVmaW5lZDtcbn1cbm1vZHVsZS5leHBvcnRzID0gRXZlbnRFbWl0dGVyO1xuXG4vLyBCYWNrd2FyZHMtY29tcGF0IHdpdGggbm9kZSAwLjEwLnhcbkV2ZW50RW1pdHRlci5FdmVudEVtaXR0ZXIgPSBFdmVudEVtaXR0ZXI7XG5cbkV2ZW50RW1pdHRlci5wcm90b3R5cGUuX2V2ZW50cyA9IHVuZGVmaW5lZDtcbkV2ZW50RW1pdHRlci5wcm90b3R5cGUuX21heExpc3RlbmVycyA9IHVuZGVmaW5lZDtcblxuLy8gQnkgZGVmYXVsdCBFdmVudEVtaXR0ZXJzIHdpbGwgcHJpbnQgYSB3YXJuaW5nIGlmIG1vcmUgdGhhbiAxMCBsaXN0ZW5lcnMgYXJlXG4vLyBhZGRlZCB0byBpdC4gVGhpcyBpcyBhIHVzZWZ1bCBkZWZhdWx0IHdoaWNoIGhlbHBzIGZpbmRpbmcgbWVtb3J5IGxlYWtzLlxuRXZlbnRFbWl0dGVyLmRlZmF1bHRNYXhMaXN0ZW5lcnMgPSAxMDtcblxuLy8gT2J2aW91c2x5IG5vdCBhbGwgRW1pdHRlcnMgc2hvdWxkIGJlIGxpbWl0ZWQgdG8gMTAuIFRoaXMgZnVuY3Rpb24gYWxsb3dzXG4vLyB0aGF0IHRvIGJlIGluY3JlYXNlZC4gU2V0IHRvIHplcm8gZm9yIHVubGltaXRlZC5cbkV2ZW50RW1pdHRlci5wcm90b3R5cGUuc2V0TWF4TGlzdGVuZXJzID0gZnVuY3Rpb24obikge1xuICBpZiAoIWlzTnVtYmVyKG4pIHx8IG4gPCAwIHx8IGlzTmFOKG4pKVxuICAgIHRocm93IFR5cGVFcnJvcignbiBtdXN0IGJlIGEgcG9zaXRpdmUgbnVtYmVyJyk7XG4gIHRoaXMuX21heExpc3RlbmVycyA9IG47XG4gIHJldHVybiB0aGlzO1xufTtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5lbWl0ID0gZnVuY3Rpb24odHlwZSkge1xuICB2YXIgZXIsIGhhbmRsZXIsIGxlbiwgYXJncywgaSwgbGlzdGVuZXJzO1xuXG4gIGlmICghdGhpcy5fZXZlbnRzKVxuICAgIHRoaXMuX2V2ZW50cyA9IHt9O1xuXG4gIC8vIElmIHRoZXJlIGlzIG5vICdlcnJvcicgZXZlbnQgbGlzdGVuZXIgdGhlbiB0aHJvdy5cbiAgaWYgKHR5cGUgPT09ICdlcnJvcicpIHtcbiAgICBpZiAoIXRoaXMuX2V2ZW50cy5lcnJvciB8fFxuICAgICAgICAoaXNPYmplY3QodGhpcy5fZXZlbnRzLmVycm9yKSAmJiAhdGhpcy5fZXZlbnRzLmVycm9yLmxlbmd0aCkpIHtcbiAgICAgIGVyID0gYXJndW1lbnRzWzFdO1xuICAgICAgaWYgKGVyIGluc3RhbmNlb2YgRXJyb3IpIHtcbiAgICAgICAgdGhyb3cgZXI7IC8vIFVuaGFuZGxlZCAnZXJyb3InIGV2ZW50XG4gICAgICB9XG4gICAgICB0aHJvdyBUeXBlRXJyb3IoJ1VuY2F1Z2h0LCB1bnNwZWNpZmllZCBcImVycm9yXCIgZXZlbnQuJyk7XG4gICAgfVxuICB9XG5cbiAgaGFuZGxlciA9IHRoaXMuX2V2ZW50c1t0eXBlXTtcblxuICBpZiAoaXNVbmRlZmluZWQoaGFuZGxlcikpXG4gICAgcmV0dXJuIGZhbHNlO1xuXG4gIGlmIChpc0Z1bmN0aW9uKGhhbmRsZXIpKSB7XG4gICAgc3dpdGNoIChhcmd1bWVudHMubGVuZ3RoKSB7XG4gICAgICAvLyBmYXN0IGNhc2VzXG4gICAgICBjYXNlIDE6XG4gICAgICAgIGhhbmRsZXIuY2FsbCh0aGlzKTtcbiAgICAgICAgYnJlYWs7XG4gICAgICBjYXNlIDI6XG4gICAgICAgIGhhbmRsZXIuY2FsbCh0aGlzLCBhcmd1bWVudHNbMV0pO1xuICAgICAgICBicmVhaztcbiAgICAgIGNhc2UgMzpcbiAgICAgICAgaGFuZGxlci5jYWxsKHRoaXMsIGFyZ3VtZW50c1sxXSwgYXJndW1lbnRzWzJdKTtcbiAgICAgICAgYnJlYWs7XG4gICAgICAvLyBzbG93ZXJcbiAgICAgIGRlZmF1bHQ6XG4gICAgICAgIGxlbiA9IGFyZ3VtZW50cy5sZW5ndGg7XG4gICAgICAgIGFyZ3MgPSBuZXcgQXJyYXkobGVuIC0gMSk7XG4gICAgICAgIGZvciAoaSA9IDE7IGkgPCBsZW47IGkrKylcbiAgICAgICAgICBhcmdzW2kgLSAxXSA9IGFyZ3VtZW50c1tpXTtcbiAgICAgICAgaGFuZGxlci5hcHBseSh0aGlzLCBhcmdzKTtcbiAgICB9XG4gIH0gZWxzZSBpZiAoaXNPYmplY3QoaGFuZGxlcikpIHtcbiAgICBsZW4gPSBhcmd1bWVudHMubGVuZ3RoO1xuICAgIGFyZ3MgPSBuZXcgQXJyYXkobGVuIC0gMSk7XG4gICAgZm9yIChpID0gMTsgaSA8IGxlbjsgaSsrKVxuICAgICAgYXJnc1tpIC0gMV0gPSBhcmd1bWVudHNbaV07XG5cbiAgICBsaXN0ZW5lcnMgPSBoYW5kbGVyLnNsaWNlKCk7XG4gICAgbGVuID0gbGlzdGVuZXJzLmxlbmd0aDtcbiAgICBmb3IgKGkgPSAwOyBpIDwgbGVuOyBpKyspXG4gICAgICBsaXN0ZW5lcnNbaV0uYXBwbHkodGhpcywgYXJncyk7XG4gIH1cblxuICByZXR1cm4gdHJ1ZTtcbn07XG5cbkV2ZW50RW1pdHRlci5wcm90b3R5cGUuYWRkTGlzdGVuZXIgPSBmdW5jdGlvbih0eXBlLCBsaXN0ZW5lcikge1xuICB2YXIgbTtcblxuICBpZiAoIWlzRnVuY3Rpb24obGlzdGVuZXIpKVxuICAgIHRocm93IFR5cGVFcnJvcignbGlzdGVuZXIgbXVzdCBiZSBhIGZ1bmN0aW9uJyk7XG5cbiAgaWYgKCF0aGlzLl9ldmVudHMpXG4gICAgdGhpcy5fZXZlbnRzID0ge307XG5cbiAgLy8gVG8gYXZvaWQgcmVjdXJzaW9uIGluIHRoZSBjYXNlIHRoYXQgdHlwZSA9PT0gXCJuZXdMaXN0ZW5lclwiISBCZWZvcmVcbiAgLy8gYWRkaW5nIGl0IHRvIHRoZSBsaXN0ZW5lcnMsIGZpcnN0IGVtaXQgXCJuZXdMaXN0ZW5lclwiLlxuICBpZiAodGhpcy5fZXZlbnRzLm5ld0xpc3RlbmVyKVxuICAgIHRoaXMuZW1pdCgnbmV3TGlzdGVuZXInLCB0eXBlLFxuICAgICAgICAgICAgICBpc0Z1bmN0aW9uKGxpc3RlbmVyLmxpc3RlbmVyKSA/XG4gICAgICAgICAgICAgIGxpc3RlbmVyLmxpc3RlbmVyIDogbGlzdGVuZXIpO1xuXG4gIGlmICghdGhpcy5fZXZlbnRzW3R5cGVdKVxuICAgIC8vIE9wdGltaXplIHRoZSBjYXNlIG9mIG9uZSBsaXN0ZW5lci4gRG9uJ3QgbmVlZCB0aGUgZXh0cmEgYXJyYXkgb2JqZWN0LlxuICAgIHRoaXMuX2V2ZW50c1t0eXBlXSA9IGxpc3RlbmVyO1xuICBlbHNlIGlmIChpc09iamVjdCh0aGlzLl9ldmVudHNbdHlwZV0pKVxuICAgIC8vIElmIHdlJ3ZlIGFscmVhZHkgZ290IGFuIGFycmF5LCBqdXN0IGFwcGVuZC5cbiAgICB0aGlzLl9ldmVudHNbdHlwZV0ucHVzaChsaXN0ZW5lcik7XG4gIGVsc2VcbiAgICAvLyBBZGRpbmcgdGhlIHNlY29uZCBlbGVtZW50LCBuZWVkIHRvIGNoYW5nZSB0byBhcnJheS5cbiAgICB0aGlzLl9ldmVudHNbdHlwZV0gPSBbdGhpcy5fZXZlbnRzW3R5cGVdLCBsaXN0ZW5lcl07XG5cbiAgLy8gQ2hlY2sgZm9yIGxpc3RlbmVyIGxlYWtcbiAgaWYgKGlzT2JqZWN0KHRoaXMuX2V2ZW50c1t0eXBlXSkgJiYgIXRoaXMuX2V2ZW50c1t0eXBlXS53YXJuZWQpIHtcbiAgICB2YXIgbTtcbiAgICBpZiAoIWlzVW5kZWZpbmVkKHRoaXMuX21heExpc3RlbmVycykpIHtcbiAgICAgIG0gPSB0aGlzLl9tYXhMaXN0ZW5lcnM7XG4gICAgfSBlbHNlIHtcbiAgICAgIG0gPSBFdmVudEVtaXR0ZXIuZGVmYXVsdE1heExpc3RlbmVycztcbiAgICB9XG5cbiAgICBpZiAobSAmJiBtID4gMCAmJiB0aGlzLl9ldmVudHNbdHlwZV0ubGVuZ3RoID4gbSkge1xuICAgICAgdGhpcy5fZXZlbnRzW3R5cGVdLndhcm5lZCA9IHRydWU7XG4gICAgICBjb25zb2xlLmVycm9yKCcobm9kZSkgd2FybmluZzogcG9zc2libGUgRXZlbnRFbWl0dGVyIG1lbW9yeSAnICtcbiAgICAgICAgICAgICAgICAgICAgJ2xlYWsgZGV0ZWN0ZWQuICVkIGxpc3RlbmVycyBhZGRlZC4gJyArXG4gICAgICAgICAgICAgICAgICAgICdVc2UgZW1pdHRlci5zZXRNYXhMaXN0ZW5lcnMoKSB0byBpbmNyZWFzZSBsaW1pdC4nLFxuICAgICAgICAgICAgICAgICAgICB0aGlzLl9ldmVudHNbdHlwZV0ubGVuZ3RoKTtcbiAgICAgIGlmICh0eXBlb2YgY29uc29sZS50cmFjZSA9PT0gJ2Z1bmN0aW9uJykge1xuICAgICAgICAvLyBub3Qgc3VwcG9ydGVkIGluIElFIDEwXG4gICAgICAgIGNvbnNvbGUudHJhY2UoKTtcbiAgICAgIH1cbiAgICB9XG4gIH1cblxuICByZXR1cm4gdGhpcztcbn07XG5cbkV2ZW50RW1pdHRlci5wcm90b3R5cGUub24gPSBFdmVudEVtaXR0ZXIucHJvdG90eXBlLmFkZExpc3RlbmVyO1xuXG5FdmVudEVtaXR0ZXIucHJvdG90eXBlLm9uY2UgPSBmdW5jdGlvbih0eXBlLCBsaXN0ZW5lcikge1xuICBpZiAoIWlzRnVuY3Rpb24obGlzdGVuZXIpKVxuICAgIHRocm93IFR5cGVFcnJvcignbGlzdGVuZXIgbXVzdCBiZSBhIGZ1bmN0aW9uJyk7XG5cbiAgdmFyIGZpcmVkID0gZmFsc2U7XG5cbiAgZnVuY3Rpb24gZygpIHtcbiAgICB0aGlzLnJlbW92ZUxpc3RlbmVyKHR5cGUsIGcpO1xuXG4gICAgaWYgKCFmaXJlZCkge1xuICAgICAgZmlyZWQgPSB0cnVlO1xuICAgICAgbGlzdGVuZXIuYXBwbHkodGhpcywgYXJndW1lbnRzKTtcbiAgICB9XG4gIH1cblxuICBnLmxpc3RlbmVyID0gbGlzdGVuZXI7XG4gIHRoaXMub24odHlwZSwgZyk7XG5cbiAgcmV0dXJuIHRoaXM7XG59O1xuXG4vLyBlbWl0cyBhICdyZW1vdmVMaXN0ZW5lcicgZXZlbnQgaWZmIHRoZSBsaXN0ZW5lciB3YXMgcmVtb3ZlZFxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5yZW1vdmVMaXN0ZW5lciA9IGZ1bmN0aW9uKHR5cGUsIGxpc3RlbmVyKSB7XG4gIHZhciBsaXN0LCBwb3NpdGlvbiwgbGVuZ3RoLCBpO1xuXG4gIGlmICghaXNGdW5jdGlvbihsaXN0ZW5lcikpXG4gICAgdGhyb3cgVHlwZUVycm9yKCdsaXN0ZW5lciBtdXN0IGJlIGEgZnVuY3Rpb24nKTtcblxuICBpZiAoIXRoaXMuX2V2ZW50cyB8fCAhdGhpcy5fZXZlbnRzW3R5cGVdKVxuICAgIHJldHVybiB0aGlzO1xuXG4gIGxpc3QgPSB0aGlzLl9ldmVudHNbdHlwZV07XG4gIGxlbmd0aCA9IGxpc3QubGVuZ3RoO1xuICBwb3NpdGlvbiA9IC0xO1xuXG4gIGlmIChsaXN0ID09PSBsaXN0ZW5lciB8fFxuICAgICAgKGlzRnVuY3Rpb24obGlzdC5saXN0ZW5lcikgJiYgbGlzdC5saXN0ZW5lciA9PT0gbGlzdGVuZXIpKSB7XG4gICAgZGVsZXRlIHRoaXMuX2V2ZW50c1t0eXBlXTtcbiAgICBpZiAodGhpcy5fZXZlbnRzLnJlbW92ZUxpc3RlbmVyKVxuICAgICAgdGhpcy5lbWl0KCdyZW1vdmVMaXN0ZW5lcicsIHR5cGUsIGxpc3RlbmVyKTtcblxuICB9IGVsc2UgaWYgKGlzT2JqZWN0KGxpc3QpKSB7XG4gICAgZm9yIChpID0gbGVuZ3RoOyBpLS0gPiAwOykge1xuICAgICAgaWYgKGxpc3RbaV0gPT09IGxpc3RlbmVyIHx8XG4gICAgICAgICAgKGxpc3RbaV0ubGlzdGVuZXIgJiYgbGlzdFtpXS5saXN0ZW5lciA9PT0gbGlzdGVuZXIpKSB7XG4gICAgICAgIHBvc2l0aW9uID0gaTtcbiAgICAgICAgYnJlYWs7XG4gICAgICB9XG4gICAgfVxuXG4gICAgaWYgKHBvc2l0aW9uIDwgMClcbiAgICAgIHJldHVybiB0aGlzO1xuXG4gICAgaWYgKGxpc3QubGVuZ3RoID09PSAxKSB7XG4gICAgICBsaXN0Lmxlbmd0aCA9IDA7XG4gICAgICBkZWxldGUgdGhpcy5fZXZlbnRzW3R5cGVdO1xuICAgIH0gZWxzZSB7XG4gICAgICBsaXN0LnNwbGljZShwb3NpdGlvbiwgMSk7XG4gICAgfVxuXG4gICAgaWYgKHRoaXMuX2V2ZW50cy5yZW1vdmVMaXN0ZW5lcilcbiAgICAgIHRoaXMuZW1pdCgncmVtb3ZlTGlzdGVuZXInLCB0eXBlLCBsaXN0ZW5lcik7XG4gIH1cblxuICByZXR1cm4gdGhpcztcbn07XG5cbkV2ZW50RW1pdHRlci5wcm90b3R5cGUucmVtb3ZlQWxsTGlzdGVuZXJzID0gZnVuY3Rpb24odHlwZSkge1xuICB2YXIga2V5LCBsaXN0ZW5lcnM7XG5cbiAgaWYgKCF0aGlzLl9ldmVudHMpXG4gICAgcmV0dXJuIHRoaXM7XG5cbiAgLy8gbm90IGxpc3RlbmluZyBmb3IgcmVtb3ZlTGlzdGVuZXIsIG5vIG5lZWQgdG8gZW1pdFxuICBpZiAoIXRoaXMuX2V2ZW50cy5yZW1vdmVMaXN0ZW5lcikge1xuICAgIGlmIChhcmd1bWVudHMubGVuZ3RoID09PSAwKVxuICAgICAgdGhpcy5fZXZlbnRzID0ge307XG4gICAgZWxzZSBpZiAodGhpcy5fZXZlbnRzW3R5cGVdKVxuICAgICAgZGVsZXRlIHRoaXMuX2V2ZW50c1t0eXBlXTtcbiAgICByZXR1cm4gdGhpcztcbiAgfVxuXG4gIC8vIGVtaXQgcmVtb3ZlTGlzdGVuZXIgZm9yIGFsbCBsaXN0ZW5lcnMgb24gYWxsIGV2ZW50c1xuICBpZiAoYXJndW1lbnRzLmxlbmd0aCA9PT0gMCkge1xuICAgIGZvciAoa2V5IGluIHRoaXMuX2V2ZW50cykge1xuICAgICAgaWYgKGtleSA9PT0gJ3JlbW92ZUxpc3RlbmVyJykgY29udGludWU7XG4gICAgICB0aGlzLnJlbW92ZUFsbExpc3RlbmVycyhrZXkpO1xuICAgIH1cbiAgICB0aGlzLnJlbW92ZUFsbExpc3RlbmVycygncmVtb3ZlTGlzdGVuZXInKTtcbiAgICB0aGlzLl9ldmVudHMgPSB7fTtcbiAgICByZXR1cm4gdGhpcztcbiAgfVxuXG4gIGxpc3RlbmVycyA9IHRoaXMuX2V2ZW50c1t0eXBlXTtcblxuICBpZiAoaXNGdW5jdGlvbihsaXN0ZW5lcnMpKSB7XG4gICAgdGhpcy5yZW1vdmVMaXN0ZW5lcih0eXBlLCBsaXN0ZW5lcnMpO1xuICB9IGVsc2Uge1xuICAgIC8vIExJRk8gb3JkZXJcbiAgICB3aGlsZSAobGlzdGVuZXJzLmxlbmd0aClcbiAgICAgIHRoaXMucmVtb3ZlTGlzdGVuZXIodHlwZSwgbGlzdGVuZXJzW2xpc3RlbmVycy5sZW5ndGggLSAxXSk7XG4gIH1cbiAgZGVsZXRlIHRoaXMuX2V2ZW50c1t0eXBlXTtcblxuICByZXR1cm4gdGhpcztcbn07XG5cbkV2ZW50RW1pdHRlci5wcm90b3R5cGUubGlzdGVuZXJzID0gZnVuY3Rpb24odHlwZSkge1xuICB2YXIgcmV0O1xuICBpZiAoIXRoaXMuX2V2ZW50cyB8fCAhdGhpcy5fZXZlbnRzW3R5cGVdKVxuICAgIHJldCA9IFtdO1xuICBlbHNlIGlmIChpc0Z1bmN0aW9uKHRoaXMuX2V2ZW50c1t0eXBlXSkpXG4gICAgcmV0ID0gW3RoaXMuX2V2ZW50c1t0eXBlXV07XG4gIGVsc2VcbiAgICByZXQgPSB0aGlzLl9ldmVudHNbdHlwZV0uc2xpY2UoKTtcbiAgcmV0dXJuIHJldDtcbn07XG5cbkV2ZW50RW1pdHRlci5saXN0ZW5lckNvdW50ID0gZnVuY3Rpb24oZW1pdHRlciwgdHlwZSkge1xuICB2YXIgcmV0O1xuICBpZiAoIWVtaXR0ZXIuX2V2ZW50cyB8fCAhZW1pdHRlci5fZXZlbnRzW3R5cGVdKVxuICAgIHJldCA9IDA7XG4gIGVsc2UgaWYgKGlzRnVuY3Rpb24oZW1pdHRlci5fZXZlbnRzW3R5cGVdKSlcbiAgICByZXQgPSAxO1xuICBlbHNlXG4gICAgcmV0ID0gZW1pdHRlci5fZXZlbnRzW3R5cGVdLmxlbmd0aDtcbiAgcmV0dXJuIHJldDtcbn07XG5cbmZ1bmN0aW9uIGlzRnVuY3Rpb24oYXJnKSB7XG4gIHJldHVybiB0eXBlb2YgYXJnID09PSAnZnVuY3Rpb24nO1xufVxuXG5mdW5jdGlvbiBpc051bWJlcihhcmcpIHtcbiAgcmV0dXJuIHR5cGVvZiBhcmcgPT09ICdudW1iZXInO1xufVxuXG5mdW5jdGlvbiBpc09iamVjdChhcmcpIHtcbiAgcmV0dXJuIHR5cGVvZiBhcmcgPT09ICdvYmplY3QnICYmIGFyZyAhPT0gbnVsbDtcbn1cblxuZnVuY3Rpb24gaXNVbmRlZmluZWQoYXJnKSB7XG4gIHJldHVybiBhcmcgPT09IHZvaWQgMDtcbn1cblxuXG5cbi8qKioqKioqKioqKioqKioqKlxuICoqIFdFQlBBQ0sgRk9PVEVSXG4gKiogLi9+L2V2ZW50cy9ldmVudHMuanNcbiAqKiBtb2R1bGUgaWQgPSAxNjlcbiAqKiBtb2R1bGUgY2h1bmtzID0gMVxuICoqLyIsInZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCdhcHAvc2Vzc2lvbicpO1xudmFyIHt1cGRhdGVTZXNzaW9ufSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3Nlc3Npb25zL2FjdGlvbnMnKTtcblxudmFyIEV2ZW50U3RyZWFtZXIgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIGNvbXBvbmVudERpZE1vdW50KCkge1xuICAgIGxldCB7c2lkfSA9IHRoaXMucHJvcHM7XG4gICAgbGV0IHt0b2tlbn0gPSBzZXNzaW9uLmdldFVzZXJEYXRhKCk7XG4gICAgbGV0IGNvbm5TdHIgPSBjZmcuYXBpLmdldEV2ZW50U3RyZWFtQ29ublN0cih0b2tlbiwgc2lkKTtcblxuICAgIHRoaXMuc29ja2V0ID0gbmV3IFdlYlNvY2tldChjb25uU3RyLCAncHJvdG8nKTtcbiAgICB0aGlzLnNvY2tldC5vbm1lc3NhZ2UgPSAoZXZlbnQpID0+IHtcbiAgICAgIHRyeVxuICAgICAge1xuICAgICAgICBsZXQganNvbiA9IEpTT04ucGFyc2UoZXZlbnQuZGF0YSk7XG4gICAgICAgIHVwZGF0ZVNlc3Npb24oanNvbi5zZXNzaW9uKTtcbiAgICAgIH1cbiAgICAgIGNhdGNoKGVycil7XG4gICAgICAgIGNvbnNvbGUubG9nKCdmYWlsZWQgdG8gcGFyc2UgZXZlbnQgc3RyZWFtIGRhdGEnKTtcbiAgICAgIH1cblxuICAgIH07XG4gICAgdGhpcy5zb2NrZXQub25jbG9zZSA9ICgpID0+IHt9O1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50KCkge1xuICAgIHRoaXMuc29ja2V0LmNsb3NlKCk7XG4gIH0sXG5cbiAgc2hvdWxkQ29tcG9uZW50VXBkYXRlKCkge1xuICAgIHJldHVybiBmYWxzZTtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuIG51bGw7XG4gIH1cbn0pO1xuXG5leHBvcnQgZGVmYXVsdCBFdmVudFN0cmVhbWVyO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvYWN0aXZlU2Vzc2lvbi9ldmVudFN0cmVhbWVyLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgTmF2TGVmdEJhciA9IHJlcXVpcmUoJy4vbmF2TGVmdEJhcicpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcbnZhciBhY3Rpb25zID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvYWN0aW9ucycpO1xudmFyIHtBY3RpdmVTZXNzaW9ufSA9IHJlcXVpcmUoJy4vYWN0aXZlU2Vzc2lvbi9tYWluLmpzeCcpO1xuXG52YXIgQXBwID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIGNvbXBvbmVudERpZE1vdW50KCl7XG4gICAgYWN0aW9ucy5mZXRjaE5vZGVzQW5kU2Vzc2lvbnMoKTtcbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi10bHB0XCI+XG4gICAgICAgIDxOYXZMZWZ0QmFyLz5cbiAgICAgICAgPEFjdGl2ZVNlc3Npb24vPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cInJvd1wiPlxuICAgICAgICAgIDxuYXYgY2xhc3NOYW1lPVwiXCIgcm9sZT1cIm5hdmlnYXRpb25cIiBzdHlsZT17eyBtYXJnaW5Cb3R0b206IDAsIGZsb2F0OiBcInJpZ2h0XCIgfX0+XG4gICAgICAgICAgICA8dWwgY2xhc3NOYW1lPVwibmF2IG5hdmJhci10b3AtbGlua3MgbmF2YmFyLXJpZ2h0XCI+XG4gICAgICAgICAgICAgIDxsaT5cbiAgICAgICAgICAgICAgICA8YSBocmVmPXtjZmcucm91dGVzLmxvZ291dH0+XG4gICAgICAgICAgICAgICAgICA8aSBjbGFzc05hbWU9XCJmYSBmYS1zaWduLW91dFwiPjwvaT5cbiAgICAgICAgICAgICAgICAgIExvZyBvdXRcbiAgICAgICAgICAgICAgICA8L2E+XG4gICAgICAgICAgICAgIDwvbGk+XG4gICAgICAgICAgICA8L3VsPlxuICAgICAgICAgIDwvbmF2PlxuICAgICAgICA8L2Rpdj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtcGFnZVwiPlxuICAgICAgICAgIHt0aGlzLnByb3BzLmNoaWxkcmVufVxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pXG5cbm1vZHVsZS5leHBvcnRzID0gQXBwO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvYXBwLmpzeFxuICoqLyIsIm1vZHVsZS5leHBvcnRzLkFwcCA9IHJlcXVpcmUoJy4vYXBwLmpzeCcpO1xubW9kdWxlLmV4cG9ydHMuTG9naW4gPSByZXF1aXJlKCcuL2xvZ2luLmpzeCcpO1xubW9kdWxlLmV4cG9ydHMuTmV3VXNlciA9IHJlcXVpcmUoJy4vbmV3VXNlci5qc3gnKTtcbm1vZHVsZS5leHBvcnRzLk5vZGVzID0gcmVxdWlyZSgnLi9ub2Rlcy9tYWluLmpzeCcpO1xubW9kdWxlLmV4cG9ydHMuU2Vzc2lvbnMgPSByZXF1aXJlKCcuL3Nlc3Npb25zL21haW4uanN4Jyk7XG5tb2R1bGUuZXhwb3J0cy5BY3RpdmVTZXNzaW9uID0gcmVxdWlyZSgnLi9hY3RpdmVTZXNzaW9uL21haW4uanN4Jyk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9pbmRleC5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyICQgPSByZXF1aXJlKCdqUXVlcnknKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciBMaW5rZWRTdGF0ZU1peGluID0gcmVxdWlyZSgncmVhY3QtYWRkb25zLWxpbmtlZC1zdGF0ZS1taXhpbicpO1xudmFyIHthY3Rpb25zfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3VzZXInKTtcbnZhciBHb29nbGVBdXRoSW5mbyA9IHJlcXVpcmUoJy4vZ29vZ2xlQXV0aCcpO1xudmFyIExvZ2luSW5wdXRGb3JtID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW0xpbmtlZFN0YXRlTWl4aW5dLFxuXG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4ge1xuICAgICAgdXNlcjogJycsXG4gICAgICBwYXNzd29yZDogJycsXG4gICAgICB0b2tlbjogJydcbiAgICB9XG4gIH0sXG5cbiAgb25DbGljazogZnVuY3Rpb24oZSkge1xuICAgIGUucHJldmVudERlZmF1bHQoKTtcbiAgICBpZiAodGhpcy5pc1ZhbGlkKCkpIHtcbiAgICAgIGFjdGlvbnMubG9naW4oeyAuLi50aGlzLnN0YXRlfSwgJy93ZWInKTtcbiAgICB9XG4gIH0sXG5cbiAgaXNWYWxpZDogZnVuY3Rpb24oKSB7XG4gICAgdmFyICRmb3JtID0gJCh0aGlzLnJlZnMuZm9ybSk7XG4gICAgcmV0dXJuICRmb3JtLmxlbmd0aCA9PT0gMCB8fCAkZm9ybS52YWxpZCgpO1xuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICByZXR1cm4gKFxuICAgICAgPGZvcm0gcmVmPVwiZm9ybVwiIGNsYXNzTmFtZT1cImdydi1sb2dpbi1pbnB1dC1mb3JtXCI+XG4gICAgICAgIDxoMz4gV2VsY29tZSB0byBUZWxlcG9ydCA8L2gzPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0IHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3VzZXInKX0gY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sIHJlcXVpcmVkXCIgcGxhY2Vob2xkZXI9XCJVc2VyIG5hbWVcIiBuYW1lPVwidXNlck5hbWVcIiAvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0IHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3Bhc3N3b3JkJyl9IHR5cGU9XCJwYXNzd29yZFwiIG5hbWU9XCJwYXNzd29yZFwiIGNsYXNzTmFtZT1cImZvcm0tY29udHJvbCByZXF1aXJlZFwiIHBsYWNlaG9sZGVyPVwiUGFzc3dvcmRcIi8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXQgdmFsdWVMaW5rPXt0aGlzLmxpbmtTdGF0ZSgndG9rZW4nKX0gY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sIHJlcXVpcmVkXCIgbmFtZT1cInRva2VuXCIgcGxhY2Vob2xkZXI9XCJUd28gZmFjdG9yIHRva2VuIChHb29nbGUgQXV0aGVudGljYXRvcilcIi8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGJ1dHRvbiB0eXBlPVwic3VibWl0XCIgY2xhc3NOYW1lPVwiYnRuIGJ0bi1wcmltYXJ5IGJsb2NrIGZ1bGwtd2lkdGggbS1iXCIgb25DbGljaz17dGhpcy5vbkNsaWNrfT5Mb2dpbjwvYnV0dG9uPlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZm9ybT5cbiAgICApO1xuICB9XG59KVxuXG52YXIgTG9naW4gPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgLy8gICAgdXNlclJlcXVlc3Q6IGdldHRlcnMudXNlclJlcXVlc3RcbiAgICB9XG4gIH0sXG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICB2YXIgaXNQcm9jZXNzaW5nID0gZmFsc2U7Ly90aGlzLnN0YXRlLnVzZXJSZXF1ZXN0LmdldCgnaXNMb2FkaW5nJyk7XG4gICAgdmFyIGlzRXJyb3IgPSBmYWxzZTsvL3RoaXMuc3RhdGUudXNlclJlcXVlc3QuZ2V0KCdpc0Vycm9yJyk7XG5cbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbG9naW4gdGV4dC1jZW50ZXJcIj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbG9nby10cHJ0XCI+PC9kaXY+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWNvbnRlbnQgZ3J2LWZsZXhcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LWNvbHVtblwiPlxuICAgICAgICAgICAgPExvZ2luSW5wdXRGb3JtLz5cbiAgICAgICAgICAgIDxHb29nbGVBdXRoSW5mby8+XG4gICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1sb2dpbi1pbmZvXCI+XG4gICAgICAgICAgICAgIDxpIGNsYXNzTmFtZT1cImZhIGZhLXF1ZXN0aW9uXCI+PC9pPlxuICAgICAgICAgICAgICA8c3Ryb25nPk5ldyBBY2NvdW50IG9yIGZvcmdvdCBwYXNzd29yZD88L3N0cm9uZz5cbiAgICAgICAgICAgICAgPGRpdj5Bc2sgZm9yIGFzc2lzdGFuY2UgZnJvbSB5b3VyIENvbXBhbnkgYWRtaW5pc3RyYXRvcjwvZGl2PlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9kaXY+XG4gICAgKTtcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gTG9naW47XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9sb2dpbi5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHsgUm91dGVyLCBJbmRleExpbmssIEhpc3RvcnkgfSA9IHJlcXVpcmUoJ3JlYWN0LXJvdXRlcicpO1xudmFyIGdldHRlcnMgPSByZXF1aXJlKCdhcHAvbW9kdWxlcy91c2VyL2dldHRlcnMnKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG5cbnZhciBtZW51SXRlbXMgPSBbXG4gIHtpY29uOiAnZmEgZmEtY29ncycsIHRvOiBjZmcucm91dGVzLm5vZGVzLCB0aXRsZTogJ05vZGVzJ30sXG4gIHtpY29uOiAnZmEgZmEtc2l0ZW1hcCcsIHRvOiBjZmcucm91dGVzLnNlc3Npb25zLCB0aXRsZTogJ1Nlc3Npb25zJ30sXG4gIHtpY29uOiAnZmEgZmEtcXVlc3Rpb24nLCB0bzogJyMnLCB0aXRsZTogJ1Nlc3Npb25zJ30sXG5dO1xuXG52YXIgTmF2TGVmdEJhciA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICByZW5kZXI6IGZ1bmN0aW9uKCl7XG4gICAgdmFyIGl0ZW1zID0gbWVudUl0ZW1zLm1hcCgoaSwgaW5kZXgpPT57XG4gICAgICB2YXIgY2xhc3NOYW1lID0gdGhpcy5jb250ZXh0LnJvdXRlci5pc0FjdGl2ZShpLnRvKSA/ICdhY3RpdmUnIDogJyc7XG4gICAgICByZXR1cm4gKFxuICAgICAgICA8bGkga2V5PXtpbmRleH0gY2xhc3NOYW1lPXtjbGFzc05hbWV9PlxuICAgICAgICAgIDxJbmRleExpbmsgdG89e2kudG99PlxuICAgICAgICAgICAgPGkgY2xhc3NOYW1lPXtpLmljb259IHRpdGxlPXtpLnRpdGxlfS8+XG4gICAgICAgICAgPC9JbmRleExpbms+XG4gICAgICAgIDwvbGk+XG4gICAgICApO1xuICAgIH0pO1xuXG4gICAgcmV0dXJuIChcbiAgICAgIDxuYXYgY2xhc3NOYW1lPSdncnYtbmF2IG5hdmJhci1kZWZhdWx0IG5hdmJhci1zdGF0aWMtc2lkZScgcm9sZT0nbmF2aWdhdGlvbic+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPScnPlxuICAgICAgICAgIDx1bCBjbGFzc05hbWU9J25hdicgaWQ9J3NpZGUtbWVudSc+XG4gICAgICAgICAgICA8bGk+PGRpdiBjbGFzc05hbWU9XCJncnYtY2lyY2xlIHRleHQtdXBwZXJjYXNlXCI+PHNwYW4+e2dldFVzZXJOYW1lTGV0dGVyKCl9PC9zcGFuPjwvZGl2PjwvbGk+XG4gICAgICAgICAgICB7aXRlbXN9XG4gICAgICAgICAgPC91bD5cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L25hdj5cbiAgICApO1xuICB9XG59KTtcblxuTmF2TGVmdEJhci5jb250ZXh0VHlwZXMgPSB7XG4gIHJvdXRlcjogUmVhY3QuUHJvcFR5cGVzLm9iamVjdC5pc1JlcXVpcmVkXG59XG5cbmZ1bmN0aW9uIGdldFVzZXJOYW1lTGV0dGVyKCl7XG4gIHZhciB7c2hvcnREaXNwbGF5TmFtZX0gPSByZWFjdG9yLmV2YWx1YXRlKGdldHRlcnMudXNlcik7XG4gIHJldHVybiBzaG9ydERpc3BsYXlOYW1lO1xufVxuXG5tb2R1bGUuZXhwb3J0cyA9IE5hdkxlZnRCYXI7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9uYXZMZWZ0QmFyLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgJCA9IHJlcXVpcmUoJ2pRdWVyeScpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHthY3Rpb25zLCBnZXR0ZXJzfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2ludml0ZScpO1xudmFyIHVzZXJNb2R1bGUgPSByZXF1aXJlKCdhcHAvbW9kdWxlcy91c2VyJyk7XG52YXIgTGlua2VkU3RhdGVNaXhpbiA9IHJlcXVpcmUoJ3JlYWN0LWFkZG9ucy1saW5rZWQtc3RhdGUtbWl4aW4nKTtcbnZhciBHb29nbGVBdXRoSW5mbyA9IHJlcXVpcmUoJy4vZ29vZ2xlQXV0aCcpO1xuXG52YXIgSW52aXRlSW5wdXRGb3JtID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW0xpbmtlZFN0YXRlTWl4aW5dLFxuXG4gIGNvbXBvbmVudERpZE1vdW50KCl7XG4gICAgJCh0aGlzLnJlZnMuZm9ybSkudmFsaWRhdGUoe1xuICAgICAgcnVsZXM6e1xuICAgICAgICBwYXNzd29yZDp7XG4gICAgICAgICAgbWlubGVuZ3RoOiA1LFxuICAgICAgICAgIHJlcXVpcmVkOiB0cnVlXG4gICAgICAgIH0sXG4gICAgICAgIHBhc3N3b3JkQ29uZmlybWVkOntcbiAgICAgICAgICByZXF1aXJlZDogdHJ1ZSxcbiAgICAgICAgICBlcXVhbFRvOiB0aGlzLnJlZnMucGFzc3dvcmRcbiAgICAgICAgfVxuICAgICAgfSxcblxuICAgICAgbWVzc2FnZXM6IHtcbiAgXHRcdFx0cGFzc3dvcmRDb25maXJtZWQ6IHtcbiAgXHRcdFx0XHRtaW5sZW5ndGg6ICQudmFsaWRhdG9yLmZvcm1hdCgnRW50ZXIgYXQgbGVhc3QgezB9IGNoYXJhY3RlcnMnKSxcbiAgXHRcdFx0XHRlcXVhbFRvOiAnRW50ZXIgdGhlIHNhbWUgcGFzc3dvcmQgYXMgYWJvdmUnXG4gIFx0XHRcdH1cbiAgICAgIH1cbiAgICB9KVxuICB9LFxuXG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4ge1xuICAgICAgbmFtZTogdGhpcy5wcm9wcy5pbnZpdGUudXNlcixcbiAgICAgIHBzdzogJycsXG4gICAgICBwc3dDb25maXJtZWQ6ICcnLFxuICAgICAgdG9rZW46ICcnXG4gICAgfVxuICB9LFxuXG4gIG9uQ2xpY2soZSkge1xuICAgIGUucHJldmVudERlZmF1bHQoKTtcbiAgICBpZiAodGhpcy5pc1ZhbGlkKCkpIHtcbiAgICAgIHVzZXJNb2R1bGUuYWN0aW9ucy5zaWduVXAoe1xuICAgICAgICBuYW1lOiB0aGlzLnN0YXRlLm5hbWUsXG4gICAgICAgIHBzdzogdGhpcy5zdGF0ZS5wc3csXG4gICAgICAgIHRva2VuOiB0aGlzLnN0YXRlLnRva2VuLFxuICAgICAgICBpbnZpdGVUb2tlbjogdGhpcy5wcm9wcy5pbnZpdGUuaW52aXRlX3Rva2VufSk7XG4gICAgfVxuICB9LFxuXG4gIGlzVmFsaWQoKSB7XG4gICAgdmFyICRmb3JtID0gJCh0aGlzLnJlZnMuZm9ybSk7XG4gICAgcmV0dXJuICRmb3JtLmxlbmd0aCA9PT0gMCB8fCAkZm9ybS52YWxpZCgpO1xuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICByZXR1cm4gKFxuICAgICAgPGZvcm0gcmVmPVwiZm9ybVwiIGNsYXNzTmFtZT1cImdydi1pbnZpdGUtaW5wdXQtZm9ybVwiPlxuICAgICAgICA8aDM+IEdldCBzdGFydGVkIHdpdGggVGVsZXBvcnQgPC9oMz5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImZvcm0tZ3JvdXBcIj5cbiAgICAgICAgICAgIDxpbnB1dFxuICAgICAgICAgICAgICB2YWx1ZUxpbms9e3RoaXMubGlua1N0YXRlKCduYW1lJyl9XG4gICAgICAgICAgICAgIG5hbWU9XCJ1c2VyTmFtZVwiXG4gICAgICAgICAgICAgIGNsYXNzTmFtZT1cImZvcm0tY29udHJvbCByZXF1aXJlZFwiXG4gICAgICAgICAgICAgIHBsYWNlaG9sZGVyPVwiVXNlciBuYW1lXCIvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0XG4gICAgICAgICAgICAgIHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3BzdycpfVxuICAgICAgICAgICAgICByZWY9XCJwYXNzd29yZFwiXG4gICAgICAgICAgICAgIHR5cGU9XCJwYXNzd29yZFwiXG4gICAgICAgICAgICAgIG5hbWU9XCJwYXNzd29yZFwiXG4gICAgICAgICAgICAgIGNsYXNzTmFtZT1cImZvcm0tY29udHJvbFwiXG4gICAgICAgICAgICAgIHBsYWNlaG9sZGVyPVwiUGFzc3dvcmRcIiAvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cCBncnYtXCI+XG4gICAgICAgICAgICA8aW5wdXRcbiAgICAgICAgICAgICAgdmFsdWVMaW5rPXt0aGlzLmxpbmtTdGF0ZSgncHN3Q29uZmlybWVkJyl9XG4gICAgICAgICAgICAgIHR5cGU9XCJwYXNzd29yZFwiXG4gICAgICAgICAgICAgIG5hbWU9XCJwYXNzd29yZENvbmZpcm1lZFwiXG4gICAgICAgICAgICAgIGNsYXNzTmFtZT1cImZvcm0tY29udHJvbFwiXG4gICAgICAgICAgICAgIHBsYWNlaG9sZGVyPVwiUGFzc3dvcmQgY29uZmlybVwiLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImZvcm0tZ3JvdXBcIj5cbiAgICAgICAgICAgIDxpbnB1dFxuICAgICAgICAgICAgICBuYW1lPVwidG9rZW5cIlxuICAgICAgICAgICAgICB2YWx1ZUxpbms9e3RoaXMubGlua1N0YXRlKCd0b2tlbicpfVxuICAgICAgICAgICAgICBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2wgcmVxdWlyZWRcIlxuICAgICAgICAgICAgICBwbGFjZWhvbGRlcj1cIlR3byBmYWN0b3IgdG9rZW4gKEdvb2dsZSBBdXRoZW50aWNhdG9yKVwiIC8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGJ1dHRvbiB0eXBlPVwic3VibWl0XCIgZGlzYWJsZWQ9e3RoaXMucHJvcHMuYXR0ZW1wLmlzUHJvY2Vzc2luZ30gY2xhc3NOYW1lPVwiYnRuIGJ0bi1wcmltYXJ5IGJsb2NrIGZ1bGwtd2lkdGggbS1iXCIgb25DbGljaz17dGhpcy5vbkNsaWNrfSA+U2lnbiB1cDwvYnV0dG9uPlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZm9ybT5cbiAgICApO1xuICB9XG59KVxuXG52YXIgSW52aXRlID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW3JlYWN0b3IuUmVhY3RNaXhpbl0sXG5cbiAgZ2V0RGF0YUJpbmRpbmdzKCkge1xuICAgIHJldHVybiB7XG4gICAgICBpbnZpdGU6IGdldHRlcnMuaW52aXRlLFxuICAgICAgYXR0ZW1wOiBnZXR0ZXJzLmF0dGVtcFxuICAgIH1cbiAgfSxcblxuICBjb21wb25lbnREaWRNb3VudCgpe1xuICAgIGFjdGlvbnMuZmV0Y2hJbnZpdGUodGhpcy5wcm9wcy5wYXJhbXMuaW52aXRlVG9rZW4pO1xuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgaWYoIXRoaXMuc3RhdGUuaW52aXRlKSB7XG4gICAgICByZXR1cm4gbnVsbDtcbiAgICB9XG5cbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtaW52aXRlIHRleHQtY2VudGVyXCI+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWxvZ28tdHBydFwiPjwvZGl2PlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1jb250ZW50IGdydi1mbGV4XCI+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZmxleC1jb2x1bW5cIj5cbiAgICAgICAgICAgIDxJbnZpdGVJbnB1dEZvcm0gYXR0ZW1wPXt0aGlzLnN0YXRlLmF0dGVtcH0gaW52aXRlPXt0aGlzLnN0YXRlLmludml0ZS50b0pTKCl9Lz5cbiAgICAgICAgICAgIDxHb29nbGVBdXRoSW5mby8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZmxleC1jb2x1bW5cIj5cbiAgICAgICAgICAgIDxoND5TY2FuIGJhciBjb2RlIGZvciBhdXRoIHRva2VuIDxici8+IDxzbWFsbD5TY2FuIGJlbG93IHRvIGdlbmVyYXRlIHlvdXIgdHdvIGZhY3RvciB0b2tlbjwvc21hbGw+PC9oND5cbiAgICAgICAgICAgIDxpbWcgY2xhc3NOYW1lPVwiaW1nLXRodW1ibmFpbFwiIHNyYz17IGBkYXRhOmltYWdlL3BuZztiYXNlNjQsJHt0aGlzLnN0YXRlLmludml0ZS5nZXQoJ3FyJyl9YCB9IC8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9kaXY+XG4gICAgKTtcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gSW52aXRlO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvbmV3VXNlci5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHtnZXR0ZXJzLCBhY3Rpb25zfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL25vZGVzJyk7XG52YXIgdXNlckdldHRlcnMgPSByZXF1aXJlKCdhcHAvbW9kdWxlcy91c2VyL2dldHRlcnMnKTtcbnZhciB7VGFibGUsIENvbHVtbiwgQ2VsbH0gPSByZXF1aXJlKCdhcHAvY29tcG9uZW50cy90YWJsZS5qc3gnKTtcbnZhciB7b3Blbn0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3Rpb25zJyk7XG5cbmNvbnN0IFRleHRDZWxsID0gKHtyb3dJbmRleCwgZGF0YSwgY29sdW1uS2V5LCAuLi5wcm9wc30pID0+IChcbiAgPENlbGwgey4uLnByb3BzfT5cbiAgICB7ZGF0YVtyb3dJbmRleF1bY29sdW1uS2V5XX1cbiAgPC9DZWxsPlxuKTtcblxuY29uc3QgVGFnQ2VsbCA9ICh7cm93SW5kZXgsIGRhdGEsIGNvbHVtbktleSwgLi4ucHJvcHN9KSA9PiAoXG4gIDxDZWxsIHsuLi5wcm9wc30+XG4gICAgeyBkYXRhW3Jvd0luZGV4XS50YWdzLm1hcCgoaXRlbSwgaW5kZXgpID0+XG4gICAgICAoPHNwYW4ga2V5PXtpbmRleH0gY2xhc3NOYW1lPVwibGFiZWwgbGFiZWwtZGVmYXVsdFwiPlxuICAgICAgICB7aXRlbS5yb2xlfSA8bGkgY2xhc3NOYW1lPVwiZmEgZmEtbG9uZy1hcnJvdy1yaWdodFwiPjwvbGk+XG4gICAgICAgIHtpdGVtLnZhbHVlfVxuICAgICAgPC9zcGFuPilcbiAgICApIH1cbiAgPC9DZWxsPlxuKTtcblxuY29uc3QgTG9naW5DZWxsID0gKHt1c2VyLCByb3dJbmRleCwgZGF0YSwgLi4ucHJvcHN9KSA9PiB7XG4gIGlmKCF1c2VyIHx8IHVzZXIubG9naW5zLmxlbmd0aCA9PT0gMCl7XG4gICAgcmV0dXJuIDxDZWxsIHsuLi5wcm9wc30gLz47XG4gIH1cblxuICB2YXIgJGxpcyA9IFtdO1xuXG4gIGZvcih2YXIgaSA9IDA7IGkgPCB1c2VyLmxvZ2lucy5sZW5ndGg7IGkrKyl7XG4gICAgJGxpcy5wdXNoKDxsaSBrZXk9e2l9PjxhIGhyZWY9XCIjXCIgdGFyZ2V0PVwiX2JsYW5rXCIgb25DbGljaz17b3Blbi5iaW5kKG51bGwsIGRhdGFbcm93SW5kZXhdLmlkLCB1c2VyLmxvZ2luc1tpXSwgdW5kZWZpbmVkKX0+e3VzZXIubG9naW5zW2ldfTwvYT48L2xpPik7XG4gIH1cblxuICByZXR1cm4gKFxuICAgIDxDZWxsIHsuLi5wcm9wc30+XG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImJ0bi1ncm91cFwiPlxuICAgICAgICA8YnV0dG9uIHR5cGU9XCJidXR0b25cIiBvbkNsaWNrPXtvcGVuLmJpbmQobnVsbCwgZGF0YVtyb3dJbmRleF0uaWQsIHVzZXIubG9naW5zWzBdLCB1bmRlZmluZWQpfSBjbGFzc05hbWU9XCJidG4gYnRuLXNtIGJ0bi1wcmltYXJ5XCI+e3VzZXIubG9naW5zWzBdfTwvYnV0dG9uPlxuICAgICAgICB7XG4gICAgICAgICAgJGxpcy5sZW5ndGggPiAxID8gKFxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJidG4tZ3JvdXBcIj5cbiAgICAgICAgICAgICAgPGJ1dHRvbiBkYXRhLXRvZ2dsZT1cImRyb3Bkb3duXCIgY2xhc3NOYW1lPVwiYnRuIGJ0bi1kZWZhdWx0IGJ0bi1zbSBkcm9wZG93bi10b2dnbGVcIiBhcmlhLWV4cGFuZGVkPVwidHJ1ZVwiPlxuICAgICAgICAgICAgICAgIDxzcGFuIGNsYXNzTmFtZT1cImNhcmV0XCI+PC9zcGFuPlxuICAgICAgICAgICAgICA8L2J1dHRvbj5cbiAgICAgICAgICAgICAgPHVsIGNsYXNzTmFtZT1cImRyb3Bkb3duLW1lbnVcIj5cbiAgICAgICAgICAgICAgICA8bGk+PGEgaHJlZj1cIiNcIiB0YXJnZXQ9XCJfYmxhbmtcIj5Mb2dzPC9hPjwvbGk+XG4gICAgICAgICAgICAgICAgPGxpPjxhIGhyZWY9XCIjXCIgdGFyZ2V0PVwiX2JsYW5rXCI+TG9nczwvYT48L2xpPlxuICAgICAgICAgICAgICA8L3VsPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgKTogbnVsbFxuICAgICAgICB9XG4gICAgICA8L2Rpdj5cbiAgICA8L0NlbGw+XG4gIClcbn07XG5cbnZhciBOb2RlcyA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBtaXhpbnM6IFtyZWFjdG9yLlJlYWN0TWl4aW5dLFxuXG4gIGdldERhdGFCaW5kaW5ncygpIHtcbiAgICByZXR1cm4ge1xuICAgICAgbm9kZVJlY29yZHM6IGdldHRlcnMubm9kZUxpc3RWaWV3LFxuICAgICAgdXNlcjogdXNlckdldHRlcnMudXNlclxuICAgIH1cbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIHZhciBkYXRhID0gdGhpcy5zdGF0ZS5ub2RlUmVjb3JkcztcbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbm9kZXNcIj5cbiAgICAgICAgPGgxPiBOb2RlcyA8L2gxPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiXCI+XG4gICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgICAgICA8VGFibGUgcm93Q291bnQ9e2RhdGEubGVuZ3RofSBjbGFzc05hbWU9XCJ0YWJsZS1zdHJpcHBlZCBncnYtbm9kZXMtdGFibGVcIj5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJzZXNzaW9uQ291bnRcIlxuICAgICAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD4gU2Vzc2lvbnMgPC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXs8VGV4dENlbGwgZGF0YT17ZGF0YX0vPiB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJhZGRyXCJcbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+IE5vZGUgPC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXs8VGV4dENlbGwgZGF0YT17ZGF0YX0vPiB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJ0YWdzXCJcbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+PC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXs8VGFnQ2VsbCBkYXRhPXtkYXRhfS8+IH1cbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICAgIGNvbHVtbktleT1cInJvbGVzXCJcbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+TG9naW4gYXM8L0NlbGw+IH1cbiAgICAgICAgICAgICAgICAgIGNlbGw9ezxMb2dpbkNlbGwgZGF0YT17ZGF0YX0gdXNlcj17dGhpcy5zdGF0ZS51c2VyfS8+IH1cbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICA8L1RhYmxlPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9kaXY+XG4gICAgKVxuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBOb2RlcztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL25vZGVzL21haW4uanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7VGFibGUsIENvbHVtbiwgQ2VsbCwgVGV4dENlbGx9ID0gcmVxdWlyZSgnYXBwL2NvbXBvbmVudHMvdGFibGUuanN4Jyk7XG52YXIge2dldHRlcnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvc2Vzc2lvbnMnKTtcbnZhciB7b3Blbn0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3Rpb25zJyk7XG5cbmNvbnN0IFVzZXJzQ2VsbCA9ICh7IHJvd0luZGV4LCBkYXRhLCAuLi5wcm9wcyB9KSA9PiB7XG4gIHZhciAkdXNlcnMgPSBkYXRhW3Jvd0luZGV4XS5wYXJ0aWVzLm1hcCgoaXRlbSwgaXRlbUluZGV4KT0+XG4gICAgKDxzcGFuIGtleT17aXRlbUluZGV4fSBjbGFzc05hbWU9XCJ0ZXh0LXVwcGVyY2FzZSBsYWJlbCBsYWJlbC1wcmltYXJ5XCI+e2l0ZW0udXNlclswXX08L3NwYW4+KVxuICApXG5cbiAgcmV0dXJuIChcbiAgICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgICAgPGRpdj5cbiAgICAgICAgeyR1c2Vyc31cbiAgICAgIDwvZGl2PlxuICAgIDwvQ2VsbD5cbiAgKVxufTtcblxuY29uc3QgQnV0dG9uQ2VsbCA9ICh7IHJvd0luZGV4LCBkYXRhLCAuLi5wcm9wcyB9KSA9PiB7XG4gIGxldCBvbkNsaWNrID0gKCkgPT4ge1xuICAgIHZhciByb3dEYXRhID0gZGF0YVtyb3dJbmRleF07XG4gICAgdmFyIHtzaWQsIHNlcnZlcklkfSA9IHJvd0RhdGFcbiAgICB2YXIgbG9naW4gPSByb3dEYXRhLmxvZ2luO1xuICAgIG9wZW4oc2VydmVySWQsIGxvZ2luLCBzaWQpO1xuICB9XG5cbiAgcmV0dXJuIChcbiAgICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgICAgPGJ1dHRvbiBvbkNsaWNrPXtvbkNsaWNrfSBjbGFzc05hbWU9XCJidG4gYnRuLWluZm8gYnRuLWNpcmNsZVwiIHR5cGU9XCJidXR0b25cIj5cbiAgICAgICAgPGkgY2xhc3NOYW1lPVwiZmEgZmEtdGVybWluYWxcIj48L2k+XG4gICAgICA8L2J1dHRvbj5cblxuICAgICAgPGJ1dHRvbiBjbGFzc05hbWU9XCJidG4gYnRuLWluZm8gYnRuLWNpcmNsZVwiIHR5cGU9XCJidXR0b25cIj5cbiAgICAgICAgPGkgY2xhc3NOYW1lPVwiZmEgZmEtcGxheS1jaXJjbGVcIj48L2k+XG4gICAgICA8L2J1dHRvbj5cblxuICAgIDwvQ2VsbD5cbiAgKVxufVxuXG52YXIgU2Vzc2lvbkxpc3QgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIHNlc3Npb25zVmlldzogZ2V0dGVycy5zZXNzaW9uc1ZpZXdcbiAgICB9XG4gIH0sXG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICB2YXIgZGF0YSA9IHRoaXMuc3RhdGUuc2Vzc2lvbnNWaWV3O1xuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1zZXNzaW9uc1wiPlxuICAgICAgICA8aDE+IFNlc3Npb25zPC9oMT5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICAgICAgPFRhYmxlIHJvd0NvdW50PXtkYXRhLmxlbmd0aH0gY2xhc3NOYW1lPVwidGFibGUtc3RyaXBwZWRcIj5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJzaWRcIlxuICAgICAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD4gU2Vzc2lvbiBJRCA8L0NlbGw+IH1cbiAgICAgICAgICAgICAgICAgIGNlbGw9ezxUZXh0Q2VsbCBkYXRhPXtkYXRhfS8+IH1cbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+IDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17XG4gICAgICAgICAgICAgICAgICAgIDxCdXR0b25DZWxsIGRhdGE9e2RhdGF9IC8+XG4gICAgICAgICAgICAgICAgICB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJzZXJ2ZXJJcFwiXG4gICAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPiBOb2RlIDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17PFRleHRDZWxsIGRhdGE9e2RhdGF9IC8+IH1cbiAgICAgICAgICAgICAgICAvPlxuXG4gICAgICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICAgICAgY29sdW1uS2V5PVwic2VydmVySWRcIlxuICAgICAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD4gVXNlcnMgPC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXs8VXNlcnNDZWxsIGRhdGE9e2RhdGF9IC8+IH1cbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICA8L1RhYmxlPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9kaXY+XG4gICAgKVxuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBTZXNzaW9uTGlzdDtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL3Nlc3Npb25zL21haW4uanN4XG4gKiovIiwidmFyIFRlcm0gPSByZXF1aXJlKCdUZXJtaW5hbCcpO1xudmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciB7ZGVib3VuY2UsIGlzTnVtYmVyfSA9IHJlcXVpcmUoJ18nKTtcblxuVGVybS5jb2xvcnNbMjU2XSA9ICdpbmhlcml0JztcblxuY29uc3QgRElTQ09OTkVDVF9UWFQgPSAnXFx4MWJbMzFtZGlzY29ubmVjdGVkXFx4MWJbbVxcclxcbic7XG5jb25zdCBDT05ORUNURURfVFhUID0gJ0Nvbm5lY3RlZCFcXHJcXG4nO1xuXG52YXIgVHR5VGVybWluYWwgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgZ2V0SW5pdGlhbFN0YXRlKCl7XG4gICAgdGhpcy5yb3dzID0gdGhpcy5wcm9wcy5yb3dzO1xuICAgIHRoaXMuY29scyA9IHRoaXMucHJvcHMuY29scztcbiAgICB0aGlzLnR0eSA9IHRoaXMucHJvcHMudHR5O1xuXG4gICAgdGhpcy5kZWJvdW5jZWRSZXNpemUgPSBkZWJvdW5jZSgoKT0+e1xuICAgICAgdGhpcy5yZXNpemUoKTtcbiAgICAgIHRoaXMudHR5LnJlc2l6ZSh0aGlzLmNvbHMsIHRoaXMucm93cyk7XG4gICAgfSwgMjAwKTtcblxuICAgIHJldHVybiB7fTtcbiAgfSxcblxuICBjb21wb25lbnREaWRNb3VudDogZnVuY3Rpb24oKSB7XG4gICAgdGhpcy50ZXJtID0gbmV3IFRlcm1pbmFsKHtcbiAgICAgIGNvbHM6IDUsXG4gICAgICByb3dzOiA1LFxuICAgICAgdXNlU3R5bGU6IHRydWUsXG4gICAgICBzY3JlZW5LZXlzOiB0cnVlLFxuICAgICAgY3Vyc29yQmxpbms6IHRydWVcbiAgICB9KTtcblxuICAgIHRoaXMudGVybS5vcGVuKHRoaXMucmVmcy5jb250YWluZXIpO1xuICAgIHRoaXMudGVybS5vbignZGF0YScsIChkYXRhKSA9PiB0aGlzLnR0eS5zZW5kKGRhdGEpKTtcblxuICAgIHRoaXMucmVzaXplKHRoaXMuY29scywgdGhpcy5yb3dzKTtcblxuICAgIHRoaXMudHR5Lm9uKCdvcGVuJywgKCk9PiB0aGlzLnRlcm0ud3JpdGUoQ09OTkVDVEVEX1RYVCkpO1xuICAgIHRoaXMudHR5Lm9uKCdjbG9zZScsICgpPT4gdGhpcy50ZXJtLndyaXRlKERJU0NPTk5FQ1RfVFhUKSk7XG4gICAgdGhpcy50dHkub24oJ2RhdGEnLCAoZGF0YSkgPT4gdGhpcy50ZXJtLndyaXRlKGRhdGEpKTtcblxuICAgIHRoaXMudHR5LmNvbm5lY3Qoe2NvbHM6IHRoaXMuY29scywgcm93czogdGhpcy5yb3dzfSk7XG4gICAgd2luZG93LmFkZEV2ZW50TGlzdGVuZXIoJ3Jlc2l6ZScsIHRoaXMuZGVib3VuY2VkUmVzaXplKTtcbiAgfSxcblxuICBjb21wb25lbnRXaWxsVW5tb3VudDogZnVuY3Rpb24oKSB7XG4gICAgdGhpcy50ZXJtLmRlc3Ryb3koKTtcbiAgICB3aW5kb3cucmVtb3ZlRXZlbnRMaXN0ZW5lcigncmVzaXplJywgdGhpcy5kZWJvdW5jZWRSZXNpemUpO1xuICB9LFxuXG4gIHNob3VsZENvbXBvbmVudFVwZGF0ZTogZnVuY3Rpb24obmV3UHJvcHMpIHtcbiAgICB2YXIge3Jvd3MsIGNvbHN9ID0gbmV3UHJvcHM7XG5cbiAgICBpZiggIWlzTnVtYmVyKHJvd3MpIHx8ICFpc051bWJlcihjb2xzKSl7XG4gICAgICByZXR1cm4gZmFsc2U7XG4gICAgfVxuXG4gICAgaWYocm93cyAhPT0gdGhpcy5yb3dzIHx8IGNvbHMgIT09IHRoaXMuY29scyl7XG4gICAgICB0aGlzLnJlc2l6ZShjb2xzLCByb3dzKVxuICAgIH1cblxuICAgIHJldHVybiBmYWxzZTtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuICggPGRpdiBjbGFzc05hbWU9XCJncnYtdGVybWluYWxcIiBpZD1cInRlcm1pbmFsLWJveFwiIHJlZj1cImNvbnRhaW5lclwiPiAgPC9kaXY+ICk7XG4gIH0sXG5cbiAgcmVzaXplOiBmdW5jdGlvbihjb2xzLCByb3dzKSB7XG4gICAgLy8gaWYgbm90IGRlZmluZWQsIHVzZSB0aGUgc2l6ZSBvZiB0aGUgY29udGFpbmVyXG4gICAgaWYoIWlzTnVtYmVyKGNvbHMpIHx8ICFpc051bWJlcihyb3dzKSl7XG4gICAgICBsZXQgZGltID0gdGhpcy5fZ2V0RGltZW5zaW9ucygpO1xuICAgICAgY29scyA9IGRpbS5jb2xzO1xuICAgICAgcm93cyA9IGRpbS5yb3dzO1xuICAgIH1cblxuICAgIHRoaXMuY29scyA9IGNvbHM7XG4gICAgdGhpcy5yb3dzID0gcm93cztcblxuICAgIHRoaXMudGVybS5yZXNpemUodGhpcy5jb2xzLCB0aGlzLnJvd3MpO1xuICB9LFxuXG4gIF9nZXREaW1lbnNpb25zKCl7XG4gICAgbGV0ICRjb250YWluZXIgPSAkKHRoaXMucmVmcy5jb250YWluZXIpO1xuICAgIGxldCBmYWtlUm93ID0gJCgnPGRpdj48c3Bhbj4mbmJzcDs8L3NwYW4+PC9kaXY+Jyk7XG5cbiAgICAkY29udGFpbmVyLmZpbmQoJy50ZXJtaW5hbCcpLmFwcGVuZChmYWtlUm93KTtcbiAgICAvLyBnZXQgZGl2IGhlaWdodFxuICAgIGxldCBmYWtlQ29sSGVpZ2h0ID0gZmFrZVJvd1swXS5nZXRCb3VuZGluZ0NsaWVudFJlY3QoKS5oZWlnaHQ7XG4gICAgLy8gZ2V0IHNwYW4gd2lkdGhcbiAgICBsZXQgZmFrZUNvbFdpZHRoID0gZmFrZVJvdy5jaGlsZHJlbigpLmZpcnN0KClbMF0uZ2V0Qm91bmRpbmdDbGllbnRSZWN0KCkud2lkdGg7XG4gICAgbGV0IGNvbHMgPSBNYXRoLmZsb29yKCRjb250YWluZXIud2lkdGgoKSAvIChmYWtlQ29sV2lkdGgpKTtcbiAgICBsZXQgcm93cyA9IE1hdGguZmxvb3IoJGNvbnRhaW5lci5oZWlnaHQoKSAvIChmYWtlQ29sSGVpZ2h0KSk7XG4gICAgZmFrZVJvdy5yZW1vdmUoKTtcblxuICAgIHJldHVybiB7Y29scywgcm93c307XG4gIH1cblxufSk7XG5cblR0eVRlcm1pbmFsLnByb3BUeXBlcyA9IHtcbiAgdHR5OiBSZWFjdC5Qcm9wVHlwZXMub2JqZWN0LmlzUmVxdWlyZWRcbn1cblxubW9kdWxlLmV4cG9ydHMgPSBUdHlUZXJtaW5hbDtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL3Rlcm1pbmFsLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgcmVuZGVyID0gcmVxdWlyZSgncmVhY3QtZG9tJykucmVuZGVyO1xudmFyIHsgUm91dGVyLCBSb3V0ZSwgUmVkaXJlY3QsIEluZGV4Um91dGUsIGJyb3dzZXJIaXN0b3J5IH0gPSByZXF1aXJlKCdyZWFjdC1yb3V0ZXInKTtcbnZhciB7IEFwcCwgTG9naW4sIE5vZGVzLCBTZXNzaW9ucywgTmV3VXNlciwgQWN0aXZlU2Vzc2lvbiB9ID0gcmVxdWlyZSgnLi9jb21wb25lbnRzJyk7XG52YXIge2Vuc3VyZVVzZXJ9ID0gcmVxdWlyZSgnLi9tb2R1bGVzL3VzZXIvYWN0aW9ucycpO1xudmFyIGF1dGggPSByZXF1aXJlKCcuL2F1dGgnKTtcbnZhciBzZXNzaW9uID0gcmVxdWlyZSgnLi9zZXNzaW9uJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnLi9jb25maWcnKTtcblxucmVxdWlyZSgnLi9tb2R1bGVzJyk7XG5cbi8vIGluaXQgc2Vzc2lvblxuc2Vzc2lvbi5pbml0KCk7XG5cbmZ1bmN0aW9uIGhhbmRsZUxvZ291dChuZXh0U3RhdGUsIHJlcGxhY2UsIGNiKXtcbiAgYXV0aC5sb2dvdXQoKTtcbn1cblxucmVuZGVyKChcbiAgPFJvdXRlciBoaXN0b3J5PXtzZXNzaW9uLmdldEhpc3RvcnkoKX0+XG4gICAgPFJvdXRlIHBhdGg9e2NmZy5yb3V0ZXMubG9naW59IGNvbXBvbmVudD17TG9naW59Lz5cbiAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5sb2dvdXR9IG9uRW50ZXI9e2hhbmRsZUxvZ291dH0vPlxuICAgIDxSb3V0ZSBwYXRoPXtjZmcucm91dGVzLm5ld1VzZXJ9IGNvbXBvbmVudD17TmV3VXNlcn0vPlxuICAgIDxSb3V0ZSBwYXRoPXtjZmcucm91dGVzLmFwcH0gY29tcG9uZW50PXtBcHB9IG9uRW50ZXI9e2Vuc3VyZVVzZXJ9ID5cbiAgICAgIDxJbmRleFJvdXRlIGNvbXBvbmVudD17Tm9kZXN9Lz5cbiAgICAgIDxSb3V0ZSBwYXRoPXtjZmcucm91dGVzLm5vZGVzfSBjb21wb25lbnQ9e05vZGVzfS8+XG4gICAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5zZXNzaW9uc30gY29tcG9uZW50PXtTZXNzaW9uc30vPlxuICAgIDwvUm91dGU+XG4gIDwvUm91dGVyPlxuKSwgZG9jdW1lbnQuZ2V0RWxlbWVudEJ5SWQoXCJhcHBcIikpO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2luZGV4LmpzeFxuICoqLyIsIm1vZHVsZS5leHBvcnRzID0gVGVybWluYWw7XG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiBleHRlcm5hbCBcIlRlcm1pbmFsXCJcbiAqKiBtb2R1bGUgaWQgPSAyODhcbiAqKiBtb2R1bGUgY2h1bmtzID0gMVxuICoqLyIsIm1vZHVsZS5leHBvcnRzID0gXztcblxuXG4vKioqKioqKioqKioqKioqKipcbiAqKiBXRUJQQUNLIEZPT1RFUlxuICoqIGV4dGVybmFsIFwiX1wiXG4gKiogbW9kdWxlIGlkID0gMjg5XG4gKiogbW9kdWxlIGNodW5rcyA9IDFcbiAqKi8iXSwic291cmNlUm9vdCI6IiJ9