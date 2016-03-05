webpackJsonp([1],{

/***/ 0:
/***/ function(module, exports, __webpack_require__) {

	module.exports = __webpack_require__(208);


/***/ },

/***/ 8:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _nuclearJs = __webpack_require__(16);
	
	var reactor = new _nuclearJs.Reactor({
	  debug: true
	});
	
	window.reactor = reactor;
	
	exports['default'] = reactor;
	module.exports = exports['default'];

/***/ },

/***/ 10:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(158);
	
	var formatPattern = _require.formatPattern;
	
	var cfg = {
	
	  baseUrl: window.location.origin,
	
	  api: {
	    renewTokenPath: '/v1/webapi/sessions/renew',
	    nodesPath: '/v1/webapi/sites/-current-/nodes',
	    sessionPath: '/v1/webapi/sessions',
	    fetchSessionPath: '/v1/webapi/sites/-current-/sessions/:sid',
	    terminalSessionPath: '/v1/webapi/sites/-current-/sessions/:sid',
	    invitePath: '/v1/webapi/users/invites/:inviteToken',
	    createUserPath: '/v1/webapi/users',
	
	    getFetchSessionUrl: function getFetchSessionUrl(sid) {
	      return formatPattern(cfg.api.fetchSessionPath, { sid: sid });
	    },
	
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
	    activeSession: '/web/sessions/:sid',
	    newUser: '/web/newuser/:inviteToken',
	    sessions: '/web/sessions',
	    pageNotFound: '/web/notfound'
	  },
	
	  getActiveSessionRouteUrl: function getActiveSessionRouteUrl(sid) {
	    return formatPattern(cfg.routes.activeSession, { sid: sid });
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

/***/ 21:
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

/***/ 25:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var _require = __webpack_require__(38);
	
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

/***/ 31:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var $ = __webpack_require__(52);
	var session = __webpack_require__(25);
	
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
	var reactor = __webpack_require__(8);
	var session = __webpack_require__(25);
	
	var _require = __webpack_require__(172);
	
	var uuid = _require.uuid;
	
	var api = __webpack_require__(31);
	var cfg = __webpack_require__(10);
	var getters = __webpack_require__(89);
	var sessionModule = __webpack_require__(100);
	
	var _require2 = __webpack_require__(87);
	
	var TLPT_TERM_OPEN = _require2.TLPT_TERM_OPEN;
	var TLPT_TERM_CLOSE = _require2.TLPT_TERM_CLOSE;
	
	var actions = {
	
	  close: function close() {
	    var _reactor$evaluate = reactor.evaluate(getters.activeSession);
	
	    var isNewSession = _reactor$evaluate.isNewSession;
	
	    reactor.dispatch(TLPT_TERM_CLOSE);
	
	    if (isNewSession) {
	      session.getHistory().push(cfg.routes.nodes);
	    } else {
	      session.getHistory().push(cfg.routes.sessions);
	    }
	  },
	
	  resize: function resize(w, h) {
	    // some min values
	    w = w < 5 ? 5 : w;
	    h = h < 5 ? 5 : h;
	
	    var reqData = { terminal_params: { w: w, h: h } };
	
	    var _reactor$evaluate2 = reactor.evaluate(getters.activeSession);
	
	    var sid = _reactor$evaluate2.sid;
	
	    api.put(cfg.api.getTerminalSessionUrl(sid), reqData).done(function () {
	      console.log('resize with w:' + w + ' and h:' + h + ' - OK');
	    }).fail(function () {
	      console.log('failed to resize with w:' + w + ' and h:' + h);
	    });
	  },
	
	  openSession: function openSession(sid) {
	    sessionModule.actions.fetchSession(sid).done(function () {
	      var sView = reactor.evaluate(sessionModule.getters.sessionViewById(sid));
	      var serverId = sView.serverId;
	      var login = sView.login;
	
	      reactor.dispatch(TLPT_TERM_OPEN, {
	        serverId: serverId,
	        login: login,
	        sid: sid,
	        isNewSession: false
	      });
	    }).fail(function () {
	      session.getHistory().push(cfg.routes.pageNotFound);
	    });
	  },
	
	  createNewSession: function createNewSession(serverId, login) {
	    var sid = uuid();
	    var routeUrl = cfg.getActiveSessionRouteUrl(sid);
	    var history = session.getHistory();
	
	    reactor.dispatch(TLPT_TERM_OPEN, {
	      serverId: serverId,
	      login: login,
	      sid: sid,
	      isNewSession: true
	    });
	
	    history.push(routeUrl);
	  }
	
	};
	
	exports['default'] = actions;
	module.exports = exports['default'];

/***/ },

/***/ 54:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(21);
	
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
	
	var _keymirror = __webpack_require__(21);
	
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
	
	var api = __webpack_require__(31);
	var session = __webpack_require__(25);
	var cfg = __webpack_require__(10);
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
	
	var _keymirror = __webpack_require__(21);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_TERM_OPEN: null,
	  TLPT_TERM_CLOSE: null
	});
	module.exports = exports['default'];

/***/ },

/***/ 88:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(87);
	
	var TLPT_TERM_OPEN = _require2.TLPT_TERM_OPEN;
	var TLPT_TERM_CLOSE = _require2.TLPT_TERM_CLOSE;
	exports['default'] = Store({
	  getInitialState: function getInitialState() {
	    return toImmutable(null);
	  },
	
	  initialize: function initialize() {
	    this.on(TLPT_TERM_OPEN, setActiveTerminal);
	    this.on(TLPT_TERM_CLOSE, close);
	  }
	});
	
	function close() {
	  return toImmutable(null);
	}
	
	function setActiveTerminal(state, _ref) {
	  var serverId = _ref.serverId;
	  var login = _ref.login;
	  var sid = _ref.sid;
	  var isNewSession = _ref.isNewSession;
	
	  return toImmutable({
	    serverId: serverId,
	    login: login,
	    sid: sid,
	    isNewSession: isNewSession
	  });
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
	    isNewSession: activeTerm.get('isNewSession'),
	    notFound: activeTerm.get('notFound'),
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
	
	var _keymirror = __webpack_require__(21);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_APP_INIT: null,
	  TLPT_APP_FAILED: null,
	  TLPT_APP_READY: null
	});
	module.exports = exports['default'];

/***/ },

/***/ 92:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(91);
	
	var TLPT_APP_INIT = _require2.TLPT_APP_INIT;
	var TLPT_APP_FAILED = _require2.TLPT_APP_FAILED;
	var TLPT_APP_READY = _require2.TLPT_APP_READY;
	
	var initState = toImmutable({
	  isReady: false,
	  isInitializing: false,
	  isFailed: false
	});
	
	exports['default'] = Store({
	
	  getInitialState: function getInitialState() {
	    return initState.set('isInitializing', true);
	  },
	
	  initialize: function initialize() {
	    this.on(TLPT_APP_INIT, function () {
	      return initState.set('isInitializing', true);
	    });
	    this.on(TLPT_APP_READY, function () {
	      return initState.set('isReady', true);
	    });
	    this.on(TLPT_APP_FAILED, function () {
	      return initState.set('isFailed', true);
	    });
	  }
	});
	module.exports = exports['default'];

/***/ },

/***/ 93:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(21);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_RECEIVE_USER_INVITE: null
	});
	module.exports = exports['default'];

/***/ },

/***/ 94:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(93);
	
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

/***/ 95:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
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

/***/ 96:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(21);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_REST_API_START: null,
	  TLPT_REST_API_SUCCESS: null,
	  TLPT_REST_API_FAIL: null
	});
	module.exports = exports['default'];

/***/ },

/***/ 97:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(21);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TRYING_TO_SIGN_UP: null
	});
	module.exports = exports['default'];

/***/ },

/***/ 98:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(8);
	var api = __webpack_require__(31);
	var cfg = __webpack_require__(10);
	
	var _require = __webpack_require__(55);
	
	var TLPT_SESSINS_RECEIVE = _require.TLPT_SESSINS_RECEIVE;
	var TLPT_SESSINS_UPDATE = _require.TLPT_SESSINS_UPDATE;
	exports['default'] = {
	
	  fetchSession: function fetchSession(sid) {
	    return api.get(cfg.api.getFetchSessionUrl(sid)).then(function (json) {
	      if (json && json.session) {
	        reactor.dispatch(TLPT_SESSINS_UPDATE, json.session);
	      }
	    });
	  },
	
	  updateSession: function updateSession(json) {
	    reactor.dispatch(TLPT_SESSINS_UPDATE, json);
	  },
	
	  receive: function receive(json) {
	    reactor.dispatch(TLPT_SESSINS_RECEIVE, json);
	  }
	};
	module.exports = exports['default'];

/***/ },

/***/ 99:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var toImmutable = _require.toImmutable;
	
	var reactor = __webpack_require__(8);
	var cfg = __webpack_require__(10);
	
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
	  return sessions.valueSeq().map(createView).toJS();
	}];
	
	var sessionViewById = function sessionViewById(sid) {
	  return [['tlpt_sessions', sid], function (session) {
	    if (!session) {
	      return null;
	    }
	
	    return createView(session);
	  }];
	};
	
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
	
	function createView(session) {
	  var sid = session.get('id');
	  var serverIp, serverId;
	  var parties = reactor.evaluate(partiesBySessionId(sid));
	
	  if (parties.length > 0) {
	    serverIp = parties[0].serverIp;
	    serverId = parties[0].serverId;
	  }
	
	  return {
	    sid: sid,
	    sessionUrl: cfg.getActiveSessionRouteUrl(sid),
	    serverIp: serverIp,
	    serverId: serverId,
	    login: session.get('login'),
	    parties: parties
	  };
	}
	
	exports['default'] = {
	  partiesBySessionId: partiesBySessionId,
	  sessionsByServer: sessionsByServer,
	  sessionsView: sessionsView,
	  sessionViewById: sessionViewById
	};
	module.exports = exports['default'];

/***/ },

/***/ 100:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(99);
	module.exports.actions = __webpack_require__(98);
	module.exports.activeTermStore = __webpack_require__(101);

/***/ },

/***/ 101:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
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

/***/ 102:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(21);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_RECEIVE_USER: null
	});
	module.exports = exports['default'];

/***/ },

/***/ 103:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(8);
	
	var _require = __webpack_require__(102);
	
	var TLPT_RECEIVE_USER = _require.TLPT_RECEIVE_USER;
	
	var _require2 = __webpack_require__(97);
	
	var TRYING_TO_SIGN_UP = _require2.TRYING_TO_SIGN_UP;
	
	var restApiActions = __webpack_require__(170);
	var auth = __webpack_require__(86);
	var session = __webpack_require__(25);
	var cfg = __webpack_require__(10);
	
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

/***/ 104:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(56);
	module.exports.actions = __webpack_require__(103);
	module.exports.nodeStore = __webpack_require__(105);

/***/ },

/***/ 105:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(102);
	
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

/***/ 117:
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
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "googleAuthLogo.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 118:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	"use strict";
	
	var React = __webpack_require__(5);
	
	var NotFoundPage = React.createClass({
	  displayName: "NotFoundPage",
	
	  render: function render() {
	    return React.createElement(
	      "div",
	      { className: "grv-page-notfound" },
	      React.createElement(
	        "div",
	        { className: "grv-logo-tprt" },
	        "Teleport"
	      ),
	      React.createElement(
	        "div",
	        { className: "grv-warning" },
	        React.createElement("i", { className: "fa fa-warning" }),
	        " "
	      ),
	      React.createElement(
	        "h1",
	        null,
	        "Whoops, we cannot find that"
	      ),
	      React.createElement(
	        "div",
	        null,
	        "Looks like the page you are looking for isn't here any longer"
	      ),
	      React.createElement(
	        "div",
	        null,
	        "If you believe this is an error, please contact your organization administrator."
	      ),
	      React.createElement(
	        "div",
	        { className: "contact-section" },
	        "If you believe this is an issue with Teleport, please ",
	        React.createElement(
	          "a",
	          { href: "https://github.com/gravitational/teleport/issues/new" },
	          "create a GitHub issue."
	        )
	      )
	    );
	  }
	});
	
	module.exports = NotFoundPage;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "notFoundPage.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 119:
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

/***/ 158:
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
	
	var _invariant = __webpack_require__(12);
	
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

/***/ 159:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };
	
	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError('Cannot call a class as a function'); } }
	
	function _inherits(subClass, superClass) { if (typeof superClass !== 'function' && superClass !== null) { throw new TypeError('Super expression must either be null or a function, not ' + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }
	
	var EventEmitter = __webpack_require__(173).EventEmitter;
	var session = __webpack_require__(25);
	var cfg = __webpack_require__(10);
	
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

/***/ 160:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(8);
	var api = __webpack_require__(31);
	var cfg = __webpack_require__(10);
	
	var _require = __webpack_require__(55);
	
	var TLPT_SESSINS_RECEIVE = _require.TLPT_SESSINS_RECEIVE;
	
	var _require2 = __webpack_require__(54);
	
	var TLPT_NODES_RECEIVE = _require2.TLPT_NODES_RECEIVE;
	
	var _require3 = __webpack_require__(91);
	
	var TLPT_APP_INIT = _require3.TLPT_APP_INIT;
	var TLPT_APP_FAILED = _require3.TLPT_APP_FAILED;
	var TLPT_APP_READY = _require3.TLPT_APP_READY;
	exports['default'] = {
	
	  initApp: function initApp() {
	    reactor.dispatch(TLPT_APP_INIT);
	    module.exports.fetchNodesAndSessions().done(function () {
	      reactor.dispatch(TLPT_APP_READY);
	    }).fail(function () {
	      reactor.dispatch(TLPT_APP_FAILED);
	    });
	  },
	
	  fetchNodesAndSessions: function fetchNodesAndSessions() {
	    return api.get(cfg.api.nodesPath).done(function (json) {
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

/***/ 161:
/***/ function(module, exports) {

	'use strict';
	
	exports.__esModule = true;
	var appState = [['tlpt'], function (app) {
	  return app.toJS();
	}];
	
	exports['default'] = {
	  appState: appState
	};
	module.exports = exports['default'];

/***/ },

/***/ 162:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(161);
	module.exports.actions = __webpack_require__(160);
	module.exports.appStore = __webpack_require__(92);

/***/ },

/***/ 163:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var reactor = __webpack_require__(8);
	reactor.registerStores({
	  'tlpt': __webpack_require__(92),
	  'tlpt_active_terminal': __webpack_require__(88),
	  'tlpt_user': __webpack_require__(105),
	  'tlpt_nodes': __webpack_require__(95),
	  'tlpt_invite': __webpack_require__(94),
	  'tlpt_rest_api': __webpack_require__(171),
	  'tlpt_sessions': __webpack_require__(101)
	});

/***/ },

/***/ 164:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(8);
	
	var _require = __webpack_require__(93);
	
	var TLPT_RECEIVE_USER_INVITE = _require.TLPT_RECEIVE_USER_INVITE;
	
	var api = __webpack_require__(31);
	var cfg = __webpack_require__(10);
	
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

/***/ 165:
/***/ function(module, exports, __webpack_require__) {

	/*eslint no-undef: 0,  no-unused-vars: 0, no-debugger:0*/
	
	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(97);
	
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

/***/ 166:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(165);
	module.exports.actions = __webpack_require__(164);
	module.exports.nodeStore = __webpack_require__(94);

/***/ },

/***/ 167:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(8);
	
	var _require = __webpack_require__(54);
	
	var TLPT_NODES_RECEIVE = _require.TLPT_NODES_RECEIVE;
	
	var api = __webpack_require__(31);
	var cfg = __webpack_require__(10);
	
	exports['default'] = {
	  fetchNodes: function fetchNodes() {
	    api.get(cfg.api.nodesPath).done(function (data) {
	      reactor.dispatch(TLPT_NODES_RECEIVE, data.nodes);
	    });
	  }
	};
	module.exports = exports['default'];

/***/ },

/***/ 168:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(8);
	
	var _require = __webpack_require__(99);
	
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

/***/ 169:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(168);
	module.exports.actions = __webpack_require__(167);
	module.exports.nodeStore = __webpack_require__(95);
	
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

/***/ 170:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(8);
	
	var _require = __webpack_require__(96);
	
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

/***/ 171:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(96);
	
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

/***/ 172:
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

/***/ 173:
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

/***/ 198:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	exports.__esModule = true;
	var cfg = __webpack_require__(10);
	var React = __webpack_require__(5);
	var session = __webpack_require__(25);
	
	var _require = __webpack_require__(98);
	
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

/***/ 199:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	
	var _require = __webpack_require__(90);
	
	var getters = _require.getters;
	var actions = _require.actions;
	
	var EventStreamer = __webpack_require__(198);
	var Tty = __webpack_require__(159);
	var TtyTerminal = __webpack_require__(207);
	var NotFoundPage = __webpack_require__(118);
	
	var ActiveSessionHost = React.createClass({
	  displayName: 'ActiveSessionHost',
	
	  mixins: [reactor.ReactMixin],
	
	  getDataBindings: function getDataBindings() {
	    return {
	      activeSession: getters.activeSession
	    };
	  },
	
	  componentDidMount: function componentDidMount() {
	    var sid = this.props.params.sid;
	
	    if (!this.state.activeSession) {
	      actions.openSession(sid);
	    }
	  },
	
	  render: function render() {
	    if (!this.state.activeSession) {
	      return null;
	    }
	
	    return React.createElement(ActiveSession, { activeSession: this.state.activeSession });
	  }
	});
	
	var ActiveSession = React.createClass({
	  displayName: 'ActiveSession',
	
	  render: function render() {
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
	              { onClick: actions.close, className: 'btn btn-danger btn-circle', type: 'button' },
	              React.createElement('i', { className: 'fa fa-times' })
	            )
	          )
	        )
	      ),
	      React.createElement('div', null),
	      React.createElement(TtyConnection, this.props.activeSession)
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
	
	    return React.createElement(
	      'div',
	      { style: { height: '100%' } },
	      React.createElement(TtyTerminal, { tty: this.tty, cols: this.props.cols, rows: this.props.rows }),
	      this.state.isConnected ? React.createElement(EventStreamer, { sid: this.props.sid }) : null
	    );
	  }
	});
	
	module.exports = { ActiveSession: ActiveSession, ActiveSessionHost: ActiveSessionHost };
	/*
	<li><button className="btn btn-primary btn-circle" type="button"> <strong>A</strong></button></li>
	<li><button className="btn btn-primary btn-circle" type="button"> B </button></li>
	<li><button className="btn btn-primary btn-circle" type="button"> C </button></li>
	*/ /*<div className="btn-group">
	    <span className="btn btn-xs btn-primary">128.0.0.1:8888</span>
	    <div className="btn-group">
	      <button data-toggle="dropdown" className="btn btn-default btn-xs dropdown-toggle" aria-expanded="true">
	        <span className="caret"></span>
	      </button>
	      <ul className="dropdown-menu">
	        <li><a href="#" target="_blank">Logs</a></li>
	        <li><a href="#" target="_blank">Logs</a></li>
	      </ul>
	    </div>
	   </div>*/

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "main.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 200:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	var NavLeftBar = __webpack_require__(203);
	var cfg = __webpack_require__(10);
	var reactor = __webpack_require__(8);
	
	var _require = __webpack_require__(162);
	
	var actions = _require.actions;
	var getters = _require.getters;
	
	var App = React.createClass({
	  displayName: 'App',
	
	  mixins: [reactor.ReactMixin],
	
	  getDataBindings: function getDataBindings() {
	    return {
	      app: getters.appState
	    };
	  },
	
	  componentWillMount: function componentWillMount() {
	    actions.initApp();
	    this.refreshInterval = setInterval(actions.fetchNodesAndSessions, 3000);
	  },
	
	  componentWillUnmount: function componentWillUnmount() {
	    clearInterval(this.refreshInterval);
	  },
	
	  render: function render() {
	    if (this.state.app.isInitializing) {
	      return null;
	    }
	
	    return React.createElement(
	      'div',
	      { className: 'grv-tlpt' },
	      React.createElement(NavLeftBar, null),
	      this.props.activeSessionHost,
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

/***/ 201:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	module.exports.App = __webpack_require__(200);
	module.exports.Login = __webpack_require__(202);
	module.exports.NewUser = __webpack_require__(204);
	module.exports.Nodes = __webpack_require__(205);
	module.exports.Sessions = __webpack_require__(206);
	module.exports.ActiveSessionHost = __webpack_require__(199).ActiveSessionHost;
	module.exports.NotFoundPage = __webpack_require__(118);
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "index.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 202:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	var $ = __webpack_require__(52);
	var reactor = __webpack_require__(8);
	var LinkedStateMixin = __webpack_require__(61);
	
	var _require = __webpack_require__(104);
	
	var actions = _require.actions;
	
	var GoogleAuthInfo = __webpack_require__(117);
	var cfg = __webpack_require__(10);
	
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
	      this.props.onClick(this.state);
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
	    return {};
	  },
	
	  onClick: function onClick(inputData) {
	    var loc = this.props.location;
	    var redirect = cfg.routes.app;
	
	    if (loc.state && loc.state.redirectTo) {
	      redirect = loc.state.redirectTo;
	    }
	
	    actions.login(inputData, redirect);
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
	          React.createElement(LoginInputForm, { onClick: this.onClick }),
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

/***/ 203:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	
	var _require = __webpack_require__(38);
	
	var Router = _require.Router;
	var IndexLink = _require.IndexLink;
	var History = _require.History;
	
	var getters = __webpack_require__(56);
	var cfg = __webpack_require__(10);
	
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

/***/ 204:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	var $ = __webpack_require__(52);
	var reactor = __webpack_require__(8);
	
	var _require = __webpack_require__(166);
	
	var actions = _require.actions;
	var getters = _require.getters;
	
	var userModule = __webpack_require__(104);
	var LinkedStateMixin = __webpack_require__(61);
	var GoogleAuthInfo = __webpack_require__(117);
	
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

/***/ 205:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }
	
	var React = __webpack_require__(5);
	var reactor = __webpack_require__(8);
	
	var _require = __webpack_require__(169);
	
	var getters = _require.getters;
	var actions = _require.actions;
	
	var userGetters = __webpack_require__(56);
	
	var _require2 = __webpack_require__(119);
	
	var Table = _require2.Table;
	var Column = _require2.Column;
	var Cell = _require2.Cell;
	
	var _require3 = __webpack_require__(53);
	
	var createNewSession = _require3.createNewSession;
	
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
	
	  var serverId = data[rowIndex].id;
	  var $lis = [];
	
	  function onNewSessionClick(i) {
	    var login = user.logins[i];
	    return function () {
	      return createNewSession(serverId, login);
	    };
	  }
	
	  for (var i = 0; i < user.logins.length; i++) {
	    $lis.push(React.createElement(
	      'li',
	      { key: i },
	      React.createElement(
	        'a',
	        { onClick: onNewSessionClick(i) },
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
	        { type: 'button', onClick: onNewSessionClick(0), className: 'btn btn-sm btn-primary' },
	        user.logins[0]
	      ),
	      $lis.length > 1 ? [React.createElement(
	        'button',
	        { key: 0, 'data-toggle': 'dropdown', className: 'btn btn-default btn-sm dropdown-toggle', 'aria-expanded': 'true' },
	        React.createElement('span', { className: 'caret' })
	      ), React.createElement(
	        'ul',
	        { key: 1, className: 'dropdown-menu' },
	        $lis
	      )] : null
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

/***/ 206:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }
	
	var React = __webpack_require__(5);
	var reactor = __webpack_require__(8);
	
	var _require = __webpack_require__(38);
	
	var Link = _require.Link;
	
	var _require2 = __webpack_require__(119);
	
	var Table = _require2.Table;
	var Column = _require2.Column;
	var Cell = _require2.Cell;
	var TextCell = _require2.TextCell;
	
	var _require3 = __webpack_require__(100);
	
	var getters = _require3.getters;
	
	var _require4 = __webpack_require__(53);
	
	var open = _require4.open;
	
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
	
	  var sessionUrl = data[rowIndex].sessionUrl;
	  return React.createElement(
	    Cell,
	    props,
	    React.createElement(
	      Link,
	      { to: sessionUrl, className: 'btn btn-info btn-circle', type: 'button' },
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

/***/ 207:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var Term = __webpack_require__(293);
	var React = __webpack_require__(5);
	
	var _require = __webpack_require__(294);
	
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

/***/ 208:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	var render = __webpack_require__(116).render;
	
	var _require = __webpack_require__(38);
	
	var Router = _require.Router;
	var Route = _require.Route;
	var Redirect = _require.Redirect;
	var IndexRoute = _require.IndexRoute;
	var browserHistory = _require.browserHistory;
	
	var _require2 = __webpack_require__(201);
	
	var App = _require2.App;
	var Login = _require2.Login;
	var Nodes = _require2.Nodes;
	var Sessions = _require2.Sessions;
	var NewUser = _require2.NewUser;
	var ActiveSessionHost = _require2.ActiveSessionHost;
	var NotFoundPage = _require2.NotFoundPage;
	
	var _require3 = __webpack_require__(103);
	
	var ensureUser = _require3.ensureUser;
	
	var auth = __webpack_require__(86);
	var session = __webpack_require__(25);
	var cfg = __webpack_require__(10);
	
	__webpack_require__(163);
	
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
	  React.createElement(Redirect, { from: cfg.routes.app, to: cfg.routes.nodes }),
	  React.createElement(
	    Route,
	    { path: cfg.routes.app, component: App, onEnter: ensureUser },
	    React.createElement(Route, { path: cfg.routes.nodes, component: Nodes }),
	    React.createElement(Route, { path: cfg.routes.activeSession, components: { activeSessionHost: ActiveSessionHost } }),
	    React.createElement(Route, { path: cfg.routes.sessions, component: Sessions })
	  ),
	  React.createElement(Route, { path: '*', component: NotFoundPage })
	), document.getElementById("app"));
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "index.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 293:
/***/ function(module, exports) {

	module.exports = Terminal;

/***/ },

/***/ 294:
/***/ function(module, exports) {

	module.exports = _;

/***/ }

});
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJzb3VyY2VzIjpbIndlYnBhY2s6Ly8vLi9zcmMvYXBwL3JlYWN0b3IuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb25maWcuanMiLCJ3ZWJwYWNrOi8vLy4vfi9rZXltaXJyb3IvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9zZXNzaW9uLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvc2VydmljZXMvYXBpLmpzIiwid2VicGFjazovLy9leHRlcm5hbCBcImpRdWVyeVwiIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvYWN0aW9uVHlwZXMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2F1dGguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGlvblR5cGVzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3RpdmVUZXJtU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hcHAvYWN0aW9uVHlwZXMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FwcC9hcHBTdG9yZS5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2FjdGlvblR5cGVzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvaW52aXRlU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL25vZGVTdG9yZS5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9jb25zdGFudHMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9zZXNzaW9ucy9zZXNzaW9uU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvYWN0aW9uVHlwZXMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvdXNlci9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvdXNlci91c2VyU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2dvb2dsZUF1dGhMb2dvLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvbm90Rm91bmRQYWdlLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvdGFibGUuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tbW9uL3BhdHRlcm5VdGlscy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbW1vbi90dHkuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FwcC9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hcHAvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvYXBwL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9nZXR0ZXJzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvcmVzdEFwaVN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvdXRpbHMuanMiLCJ3ZWJwYWNrOi8vLy4vfi9ldmVudHMvZXZlbnRzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9hY3RpdmVTZXNzaW9uL2V2ZW50U3RyZWFtZXIuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9hY3RpdmVTZXNzaW9uL21haW4uanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9hcHAuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9pbmRleC5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2xvZ2luLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvbmF2TGVmdEJhci5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL25ld1VzZXIuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9ub2Rlcy9tYWluLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvc2Vzc2lvbnMvbWFpbi5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL3Rlcm1pbmFsLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2luZGV4LmpzeCIsIndlYnBhY2s6Ly8vZXh0ZXJuYWwgXCJUZXJtaW5hbFwiIiwid2VicGFjazovLy9leHRlcm5hbCBcIl9cIiJdLCJuYW1lcyI6W10sIm1hcHBpbmdzIjoiOzs7Ozs7Ozs7Ozs7Ozs7OztzQ0FBd0IsRUFBWTs7QUFFcEMsS0FBTSxPQUFPLEdBQUcsdUJBQVk7QUFDMUIsUUFBSyxFQUFFLElBQUk7RUFDWixDQUFDOztBQUVGLE9BQU0sQ0FBQyxPQUFPLEdBQUcsT0FBTyxDQUFDOztzQkFFVixPQUFPOzs7Ozs7Ozs7Ozs7Z0JDUkEsbUJBQU8sQ0FBQyxHQUF5QixDQUFDOztLQUFuRCxhQUFhLFlBQWIsYUFBYTs7QUFFbEIsS0FBSSxHQUFHLEdBQUc7O0FBRVIsVUFBTyxFQUFFLE1BQU0sQ0FBQyxRQUFRLENBQUMsTUFBTTs7QUFFL0IsTUFBRyxFQUFFO0FBQ0gsbUJBQWMsRUFBQywyQkFBMkI7QUFDMUMsY0FBUyxFQUFFLGtDQUFrQztBQUM3QyxnQkFBVyxFQUFFLHFCQUFxQjtBQUNsQyxxQkFBZ0IsRUFBRSwwQ0FBMEM7QUFDNUQsd0JBQW1CLEVBQUUsMENBQTBDO0FBQy9ELGVBQVUsRUFBRSx1Q0FBdUM7QUFDbkQsbUJBQWMsRUFBRSxrQkFBa0I7O0FBRWxDLHVCQUFrQixFQUFFLDRCQUFDLEdBQUcsRUFBRztBQUN6QixjQUFPLGFBQWEsQ0FBQyxHQUFHLENBQUMsR0FBRyxDQUFDLGdCQUFnQixFQUFFLEVBQUMsR0FBRyxFQUFILEdBQUcsRUFBQyxDQUFDLENBQUM7TUFDdkQ7O0FBRUQsMEJBQXFCLEVBQUUsK0JBQUMsR0FBRyxFQUFJO0FBQzdCLGNBQU8sYUFBYSxDQUFDLEdBQUcsQ0FBQyxHQUFHLENBQUMs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QUksRUFBRSxHQUFHLENBQUMsTUFBTSxDQUFDLE9BQVEsRUFBQyxTQUFTLEVBQUUsT0FBUSxHQUFFO0dBQ3RELG9CQUFDLFFBQVEsSUFBQyxJQUFJLEVBQUUsR0FBRyxDQUFDLE1BQU0sQ0FBQyxHQUFJLEVBQUMsRUFBRSxFQUFFLEdBQUcsQ0FBQyxNQUFNLENBQUMsS0FBTSxHQUFFO0dBQ3ZEO0FBQUMsVUFBSztPQUFDLElBQUksRUFBRSxHQUFHLENBQUMsTUFBTSxDQUFDLEdBQUksRUFBQyxTQUFTLEVBQUUsR0FBSSxFQUFDLE9BQU8sRUFBRSxVQUFXO0tBQy9ELG9CQUFDLEtBQUssSUFBQyxJQUFJLEVBQUUsR0FBRyxDQUFDLE1BQU0sQ0FBQyxLQUFNLEVBQUMsU0FBUyxFQUFFLEtBQU0sR0FBRTtLQUNsRCxvQkFBQyxLQUFLLElBQUMsSUFBSSxFQUFFLEdBQUcsQ0FBQyxNQUFNLENBQUMsYUFBYyxFQUFDLFVBQVUsRUFBRSxFQUFDLGlCQUFpQixFQUFFLGlCQUFpQixFQUFFLEdBQUU7S0FDNUYsb0JBQUMsS0FBSyxJQUFDLElBQUksRUFBRSxHQUFHLENBQUMsTUFBTSxDQUFDLFFBQVMsRUFBQyxTQUFTLEVBQUUsUUFBUyxHQUFFO0lBQ2xEO0dBQ1Isb0JBQUMsS0FBSyxJQUFDLElBQUksRUFBQyxHQUFHLEVBQUMsU0FBUyxFQUFFLFlBQWEsR0FBRztFQUNwQyxFQUNSLFFBQVEsQ0FBQyxjQUFjLENBQUMsS0FBSyxDQUFDLENBQUMsQzs7Ozs7Ozs7O0FDL0JsQywyQjs7Ozs7OztBQ0FBLG9CIiwiZmlsZSI6ImFwcC5qcyIsInNvdXJjZXNDb250ZW50IjpbImltcG9ydCB7IFJlYWN0b3IgfSBmcm9tICdudWNsZWFyLWpzJ1xuXG5jb25zdCByZWFjdG9yID0gbmV3IFJlYWN0b3Ioe1xuICBkZWJ1ZzogdHJ1ZVxufSlcblxud2luZG93LnJlYWN0b3IgPSByZWFjdG9yO1xuXG5leHBvcnQgZGVmYXVsdCByZWFjdG9yXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvcmVhY3Rvci5qc1xuICoqLyIsImxldCB7Zm9ybWF0UGF0dGVybn0gPSByZXF1aXJlKCdhcHAvY29tbW9uL3BhdHRlcm5VdGlscycpO1xuXG5sZXQgY2ZnID0ge1xuXG4gIGJhc2VVcmw6IHdpbmRvdy5sb2NhdGlvbi5vcmlnaW4sXG5cbiAgYXBpOiB7XG4gICAgcmVuZXdUb2tlblBhdGg6Jy92MS93ZWJhcGkvc2Vzc2lvbnMvcmVuZXcnLFxuICAgIG5vZGVzUGF0aDogJy92MS93ZWJhcGkvc2l0ZXMvLWN1cnJlbnQtL25vZGVzJyxcbiAgICBzZXNzaW9uUGF0aDogJy92MS93ZWJhcGkvc2Vzc2lvbnMnLFxuICAgIGZldGNoU2Vzc2lvblBhdGg6ICcvdjEvd2ViYXBpL3NpdGVzLy1jdXJyZW50LS9zZXNzaW9ucy86c2lkJyxcbiAgICB0ZXJtaW5hbFNlc3Npb25QYXRoOiAnL3YxL3dlYmFwaS9zaXRlcy8tY3VycmVudC0vc2Vzc2lvbnMvOnNpZCcsXG4gICAgaW52aXRlUGF0aDogJy92MS93ZWJhcGkvdXNlcnMvaW52aXRlcy86aW52aXRlVG9rZW4nLFxuICAgIGNyZWF0ZVVzZXJQYXRoOiAnL3YxL3dlYmFwaS91c2VycycsXG5cbiAgICBnZXRGZXRjaFNlc3Npb25Vcmw6IChzaWQpPT57XG4gICAgICByZXR1cm4gZm9ybWF0UGF0dGVybihjZmcuYXBpLmZldGNoU2Vzc2lvblBhdGgsIHtzaWR9KTtcbiAgICB9LFxuXG4gICAgZ2V0VGVybWluYWxTZXNzaW9uVXJsOiAoc2lkKT0+IHtcbiAgICAgIHJldHVybiBmb3JtYXRQYXR0ZXJuKGNmZy5hcGkudGVybWluYWxTZXNzaW9uUGF0aCwge3NpZH0pO1xuICAgIH0sXG5cbiAgICBnZXRJbnZpdGVVcmw6IChpbnZpdGVUb2tlbikgPT4ge1xuICAgICAgcmV0dXJuIGZvcm1hdFBhdHRlcm4oY2ZnLmFwaS5pbnZpdGVQYXRoLCB7aW52aXRlVG9rZW59KTtcbiAgICB9LFxuXG4gICAgZ2V0RXZlbnRTdHJlYW1Db25uU3RyOiAodG9rZW4sIHNpZCkgPT4ge1xuICAgICAgdmFyIGhvc3RuYW1lID0gZ2V0V3NIb3N0TmFtZSgpO1xuICAgICAgcmV0dXJuIGAke2hvc3RuYW1lfS92MS93ZWJhcGkvc2l0ZXMvLWN1cnJlbnQtL3Nlc3Npb25zLyR7c2lkfS9ldmVudHMvc3RyZWFtP2FjY2Vzc190b2tlbj0ke3Rva2VufWA7XG4gICAgfSxcblxuICAgIGdldFR0eUNvbm5TdHI6ICh7dG9rZW4sIHNlcnZlcklkLCBsb2dpbiwgc2lkLCByb3dzLCBjb2xzfSkgPT4ge1xuICAgICAgdmFyIHBhcmFtcyA9IHtcbiAgICAgICAgc2VydmVyX2lkOiBzZXJ2ZXJJZCxcbiAgICAgICAgbG9naW4sXG4gICAgICAgIHNpZCxcbiAgICAgICAgdGVybToge1xuICAgICAgICAgIGg6IHJvd3MsXG4gICAgICAgICAgdzogY29sc1xuICAgICAgICB9XG4gICAgICB9XG5cbiAgICAgIHZhciBqc29uID0gSlNPTi5zdHJpbmdpZnkocGFyYW1zKTtcbiAgICAgIHZhciBqc29uRW5jb2RlZCA9IHdpbmRvdy5lbmNvZGVVUkkoanNvbik7XG4gICAgICB2YXIgaG9zdG5hbWUgPSBnZXRXc0hvc3ROYW1lKCk7XG4gICAgICByZXR1cm4gYCR7aG9zdG5hbWV9L3YxL3dlYmFwaS9zaXRlcy8tY3VycmVudC0vY29ubmVjdD9hY2Nlc3NfdG9rZW49JHt0b2tlbn0mcGFyYW1zPSR7anNvbkVuY29kZWR9YDtcbiAgICB9XG4gIH0sXG5cbiAgcm91dGVzOiB7XG4gICAgYXBwOiAnL3dlYicsXG4gICAgbG9nb3V0OiAnL3dlYi9sb2dvdXQnLFxuICAgIGxvZ2luOiAnL3dlYi9sb2dpbicsXG4gICAgbm9kZXM6ICcvd2ViL25vZGVzJyxcbiAgICBhY3RpdmVTZXNzaW9uOiAnL3dlYi9zZXNzaW9ucy86c2lkJyxcbiAgICBuZXdVc2VyOiAnL3dlYi9uZXd1c2VyLzppbnZpdGVUb2tlbicsXG4gICAgc2Vzc2lvbnM6ICcvd2ViL3Nlc3Npb25zJyxcbiAgICBwYWdlTm90Rm91bmQ6ICcvd2ViL25vdGZvdW5kJ1xuICB9LFxuXG4gIGdldEFjdGl2ZVNlc3Npb25Sb3V0ZVVybChzaWQpe1xuICAgIHJldHVybiBmb3JtYXRQYXR0ZXJuKGNmZy5yb3V0ZXMuYWN0aXZlU2Vzc2lvbiwge3NpZH0pO1xuICB9XG59XG5cbmV4cG9ydCBkZWZhdWx0IGNmZztcblxuZnVuY3Rpb24gZ2V0V3NIb3N0TmFtZSgpe1xuICB2YXIgcHJlZml4ID0gbG9jYXRpb24ucHJvdG9jb2wgPT0gXCJodHRwczpcIj9cIndzczovL1wiOlwid3M6Ly9cIjtcbiAgdmFyIGhvc3Rwb3J0ID0gbG9jYXRpb24uaG9zdG5hbWUrKGxvY2F0aW9uLnBvcnQgPyAnOicrbG9jYXRpb24ucG9ydDogJycpO1xuICByZXR1cm4gYCR7cHJlZml4fSR7aG9zdHBvcnR9YDtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb25maWcuanNcbiAqKi8iLCIvKipcbiAqIENvcHlyaWdodCAyMDEzLTIwMTQgRmFjZWJvb2ssIEluYy5cbiAqXG4gKiBMaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xuICogeW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuICogWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG4gKlxuICogaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG4gKlxuICogVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuICogZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuICogV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG4gKiBTZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG4gKiBsaW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiAqXG4gKi9cblxuXCJ1c2Ugc3RyaWN0XCI7XG5cbi8qKlxuICogQ29uc3RydWN0cyBhbiBlbnVtZXJhdGlvbiB3aXRoIGtleXMgZXF1YWwgdG8gdGhlaXIgdmFsdWUuXG4gKlxuICogRm9yIGV4YW1wbGU6XG4gKlxuICogICB2YXIgQ09MT1JTID0ga2V5TWlycm9yKHtibHVlOiBudWxsLCByZWQ6IG51bGx9KTtcbiAqICAgdmFyIG15Q29sb3IgPSBDT0xPUlMuYmx1ZTtcbiAqICAgdmFyIGlzQ29sb3JWYWxpZCA9ICEhQ09MT1JTW215Q29sb3JdO1xuICpcbiAqIFRoZSBsYXN0IGxpbmUgY291bGQgbm90IGJlIHBlcmZvcm1lZCBpZiB0aGUgdmFsdWVzIG9mIHRoZSBnZW5lcmF0ZWQgZW51bSB3ZXJlXG4gKiBub3QgZXF1YWwgdG8gdGhlaXIga2V5cy5cbiAqXG4gKiAgIElucHV0OiAge2tleTE6IHZhbDEsIGtleTI6IHZhbDJ9XG4gKiAgIE91dHB1dDoge2tleTE6IGtleTEsIGtleTI6IGtleTJ9XG4gKlxuICogQHBhcmFtIHtvYmplY3R9IG9ialxuICogQHJldHVybiB7b2JqZWN0fVxuICovXG52YXIga2V5TWlycm9yID0gZnVuY3Rpb24ob2JqKSB7XG4gIHZhciByZXQgPSB7fTtcbiAgdmFyIGtleTtcbiAgaWYgKCEob2JqIGluc3RhbmNlb2YgT2JqZWN0ICYmICFBcnJheS5pc0FycmF5KG9iaikpKSB7XG4gICAgdGhyb3cgbmV3IEVycm9yKCdrZXlNaXJyb3IoLi4uKTogQXJndW1lbnQgbXVzdCBiZSBhbiBvYmplY3QuJyk7XG4gIH1cbiAgZm9yIChrZXkgaW4gb2JqKSB7XG4gICAgaWYgKCFvYmouaGFzT3duUHJvcGVydHkoa2V5KSkge1xuICAgICAgY29udGludWU7XG4gICAgfVxuICAgIHJldFtrZXldID0ga2V5O1xuICB9XG4gIHJldHVybiByZXQ7XG59O1xuXG5tb2R1bGUuZXhwb3J0cyA9IGtleU1pcnJvcjtcblxuXG5cbi8qKioqKioqKioqKioqKioqKlxuICoqIFdFQlBBQ0sgRk9PVEVSXG4gKiogLi9+L2tleW1pcnJvci9pbmRleC5qc1xuICoqIG1vZHVsZSBpZCA9IDIxXG4gKiogbW9kdWxlIGNodW5rcyA9IDFcbiAqKi8iLCJ2YXIgeyBicm93c2VySGlzdG9yeSwgY3JlYXRlTWVtb3J5SGlzdG9yeSB9ID0gcmVxdWlyZSgncmVhY3Qtcm91dGVyJyk7XG5cbmNvbnN0IEFVVEhfS0VZX0RBVEEgPSAnYXV0aERhdGEnO1xuXG52YXIgX2hpc3RvcnkgPSBjcmVhdGVNZW1vcnlIaXN0b3J5KCk7XG5cbnZhciBzZXNzaW9uID0ge1xuXG4gIGluaXQoaGlzdG9yeT1icm93c2VySGlzdG9yeSl7XG4gICAgX2hpc3RvcnkgPSBoaXN0b3J5O1xuICB9LFxuXG4gIGdldEhpc3RvcnkoKXtcbiAgICByZXR1cm4gX2hpc3Rvcnk7XG4gIH0sXG5cbiAgc2V0VXNlckRhdGEodXNlckRhdGEpe1xuICAgIGxvY2FsU3RvcmFnZS5zZXRJdGVtKEFVVEhfS0VZX0RBVEEsIEpTT04uc3RyaW5naWZ5KHVzZXJEYXRhKSk7XG4gIH0sXG5cbiAgZ2V0VXNlckRhdGEoKXtcbiAgICB2YXIgaXRlbSA9IGxvY2FsU3RvcmFnZS5nZXRJdGVtKEFVVEhfS0VZX0RBVEEpO1xuICAgIGlmKGl0ZW0pe1xuICAgICAgcmV0dXJuIEpTT04ucGFyc2UoaXRlbSk7XG4gICAgfVxuXG4gICAgcmV0dXJuIHt9O1xuICB9LFxuXG4gIGNsZWFyKCl7XG4gICAgbG9jYWxTdG9yYWdlLmNsZWFyKClcbiAgfVxuXG59XG5cbm1vZHVsZS5leHBvcnRzID0gc2Vzc2lvbjtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9zZXNzaW9uLmpzXG4gKiovIiwidmFyICQgPSByZXF1aXJlKFwialF1ZXJ5XCIpO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCdhcHAvc2Vzc2lvbicpO1xuXG5jb25zdCBhcGkgPSB7XG5cbiAgcHV0KHBhdGgsIGRhdGEsIHdpdGhUb2tlbil7XG4gICAgcmV0dXJuIGFwaS5hamF4KHt1cmw6IHBhdGgsIGRhdGE6IEpTT04uc3RyaW5naWZ5KGRhdGEpLCB0eXBlOiAnUFVUJ30sIHdpdGhUb2tlbik7XG4gIH0sXG5cbiAgcG9zdChwYXRoLCBkYXRhLCB3aXRoVG9rZW4pe1xuICAgIHJldHVybiBhcGkuYWpheCh7dXJsOiBwYXRoLCBkYXRhOiBKU09OLnN0cmluZ2lmeShkYXRhKSwgdHlwZTogJ1BPU1QnfSwgd2l0aFRva2VuKTtcbiAgfSxcblxuICBnZXQocGF0aCl7XG4gICAgcmV0dXJuIGFwaS5hamF4KHt1cmw6IHBhdGh9KTtcbiAgfSxcblxuICBhamF4KGNmZywgd2l0aFRva2VuID0gdHJ1ZSl7XG4gICAgdmFyIGRlZmF1bHRDZmcgPSB7XG4gICAgICB0eXBlOiBcIkdFVFwiLFxuICAgICAgZGF0YVR5cGU6IFwianNvblwiLFxuICAgICAgYmVmb3JlU2VuZDogZnVuY3Rpb24oeGhyKSB7XG4gICAgICAgIGlmKHdpdGhUb2tlbil7XG4gICAgICAgICAgdmFyIHsgdG9rZW4gfSA9IHNlc3Npb24uZ2V0VXNlckRhdGEoKTtcbiAgICAgICAgICB4aHIuc2V0UmVxdWVzdEhlYWRlcignQXV0aG9yaXphdGlvbicsJ0JlYXJlciAnICsgdG9rZW4pO1xuICAgICAgICB9XG4gICAgICAgfVxuICAgIH1cblxuICAgIHJldHVybiAkLmFqYXgoJC5leHRlbmQoe30sIGRlZmF1bHRDZmcsIGNmZykpO1xuICB9XG59XG5cbm1vZHVsZS5leHBvcnRzID0gYXBpO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL3NlcnZpY2VzL2FwaS5qc1xuICoqLyIsIm1vZHVsZS5leHBvcnRzID0galF1ZXJ5O1xuXG5cbi8qKioqKioqKioqKioqKioqKlxuICoqIFdFQlBBQ0sgRk9PVEVSXG4gKiogZXh0ZXJuYWwgXCJqUXVlcnlcIlxuICoqIG1vZHVsZSBpZCA9IDUyXG4gKiogbW9kdWxlIGNodW5rcyA9IDFcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgc2Vzc2lvbiA9IHJlcXVpcmUoJ2FwcC9zZXNzaW9uJyk7XG52YXIge3V1aWR9ID0gcmVxdWlyZSgnYXBwL3V0aWxzJyk7XG52YXIgYXBpID0gcmVxdWlyZSgnYXBwL3NlcnZpY2VzL2FwaScpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcbnZhciBnZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG52YXIgc2Vzc2lvbk1vZHVsZSA9IHJlcXVpcmUoJy4vLi4vc2Vzc2lvbnMnKTtcblxudmFyIHsgVExQVF9URVJNX09QRU4sIFRMUFRfVEVSTV9DTE9TRSB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG52YXIgYWN0aW9ucyA9IHtcblxuICBjbG9zZSgpe1xuICAgIGxldCB7aXNOZXdTZXNzaW9ufSA9IHJlYWN0b3IuZXZhbHVhdGUoZ2V0dGVycy5hY3RpdmVTZXNzaW9uKTtcblxuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9URVJNX0NMT1NFKTtcblxuICAgIGlmKGlzTmV3U2Vzc2lvbil7XG4gICAgICBzZXNzaW9uLmdldEhpc3RvcnkoKS5wdXNoKGNmZy5yb3V0ZXMubm9kZXMpO1xuICAgIH1lbHNle1xuICAgICAgc2Vzc2lvbi5nZXRIaXN0b3J5KCkucHVzaChjZmcucm91dGVzLnNlc3Npb25zKTtcbiAgICB9XG4gIH0sXG5cbiAgcmVzaXplKHcsIGgpe1xuICAgIC8vIHNvbWUgbWluIHZhbHVlc1xuICAgIHcgPSB3IDwgNSA/IDUgOiB3O1xuICAgIGggPSBoIDwgNSA/IDUgOiBoO1xuXG4gICAgbGV0IHJlcURhdGEgPSB7IHRlcm1pbmFsX3BhcmFtczogeyB3LCBoIH0gfTtcbiAgICBsZXQge3NpZH0gPSByZWFjdG9yLmV2YWx1YXRlKGdldHRlcnMuYWN0aXZlU2Vzc2lvbik7XG5cbiAgICBhcGkucHV0KGNmZy5hcGkuZ2V0VGVybWluYWxTZXNzaW9uVXJsKHNpZCksIHJlcURhdGEpXG4gICAgICAuZG9uZSgoKT0+e1xuICAgICAgICBjb25zb2xlLmxvZyhgcmVzaXplIHdpdGggdzoke3d9IGFuZCBoOiR7aH0gLSBPS2ApO1xuICAgICAgfSlcbiAgICAgIC5mYWlsKCgpPT57XG4gICAgICAgIGNvbnNvbGUubG9nKGBmYWlsZWQgdG8gcmVzaXplIHdpdGggdzoke3d9IGFuZCBoOiR7aH1gKTtcbiAgICB9KVxuICB9LFxuXG4gIG9wZW5TZXNzaW9uKHNpZCl7XG4gICAgc2Vzc2lvbk1vZHVsZS5hY3Rpb25zLmZldGNoU2Vzc2lvbihzaWQpXG4gICAgICAuZG9uZSgoKT0+e1xuICAgICAgICBsZXQgc1ZpZXcgPSByZWFjdG9yLmV2YWx1YXRlKHNlc3Npb25Nb2R1bGUuZ2V0dGVycy5zZXNzaW9uVmlld0J5SWQoc2lkKSk7XG4gICAgICAgIGxldCB7IHNlcnZlcklkLCBsb2dpbiB9ID0gc1ZpZXc7XG4gICAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9URVJNX09QRU4sIHtcbiAgICAgICAgICAgIHNlcnZlcklkLFxuICAgICAgICAgICAgbG9naW4sXG4gICAgICAgICAgICBzaWQsXG4gICAgICAgICAgICBpc05ld1Nlc3Npb246IGZhbHNlXG4gICAgICAgICAgfSk7XG4gICAgICB9KVxuICAgICAgLmZhaWwoKCk9PntcbiAgICAgICAgc2Vzc2lvbi5nZXRIaXN0b3J5KCkucHVzaChjZmcucm91dGVzLnBhZ2VOb3RGb3VuZCk7XG4gICAgICB9KVxuICB9LFxuXG4gIGNyZWF0ZU5ld1Nlc3Npb24oc2VydmVySWQsIGxvZ2luKXtcbiAgICB2YXIgc2lkID0gdXVpZCgpO1xuICAgIHZhciByb3V0ZVVybCA9IGNmZy5nZXRBY3RpdmVTZXNzaW9uUm91dGVVcmwoc2lkKTtcbiAgICB2YXIgaGlzdG9yeSA9IHNlc3Npb24uZ2V0SGlzdG9yeSgpO1xuXG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1RFUk1fT1BFTiwge1xuICAgICAgc2VydmVySWQsXG4gICAgICBsb2dpbixcbiAgICAgIHNpZCxcbiAgICAgIGlzTmV3U2Vzc2lvbjogdHJ1ZVxuICAgIH0pO1xuXG4gICAgaGlzdG9yeS5wdXNoKHJvdXRlVXJsKTtcbiAgfVxuXG59XG5cbmV4cG9ydCBkZWZhdWx0IGFjdGlvbnM7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3Rpb25zLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfTk9ERVNfUkVDRUlWRTogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2FjdGlvblR5cGVzLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfU0VTU0lOU19SRUNFSVZFOiBudWxsLFxuICBUTFBUX1NFU1NJTlNfVVBEQVRFOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvYWN0aW9uVHlwZXMuanNcbiAqKi8iLCJjb25zdCB1c2VyID0gWyBbJ3RscHRfdXNlciddLCAoY3VycmVudFVzZXIpID0+IHtcbiAgICBpZighY3VycmVudFVzZXIpe1xuICAgICAgcmV0dXJuIG51bGw7XG4gICAgfVxuXG4gICAgdmFyIG5hbWUgPSBjdXJyZW50VXNlci5nZXQoJ25hbWUnKSB8fCAnJztcbiAgICB2YXIgc2hvcnREaXNwbGF5TmFtZSA9IG5hbWVbMF0gfHwgJyc7XG5cbiAgICByZXR1cm4ge1xuICAgICAgbmFtZSxcbiAgICAgIHNob3J0RGlzcGxheU5hbWUsXG4gICAgICBsb2dpbnM6IGN1cnJlbnRVc2VyLmdldCgnYWxsb3dlZF9sb2dpbnMnKS50b0pTKClcbiAgICB9XG4gIH1cbl07XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgdXNlclxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvdXNlci9nZXR0ZXJzLmpzXG4gKiovIiwidmFyIGFwaSA9IHJlcXVpcmUoJy4vc2VydmljZXMvYXBpJyk7XG52YXIgc2Vzc2lvbiA9IHJlcXVpcmUoJy4vc2Vzc2lvbicpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcbnZhciAkID0gcmVxdWlyZSgnalF1ZXJ5Jyk7XG5cbmNvbnN0IHJlZnJlc2hSYXRlID0gNjAwMDAgKiA1OyAvLyAxIG1pblxuXG52YXIgcmVmcmVzaFRva2VuVGltZXJJZCA9IG51bGw7XG5cbnZhciBhdXRoID0ge1xuXG4gIHNpZ25VcChuYW1lLCBwYXNzd29yZCwgdG9rZW4sIGludml0ZVRva2VuKXtcbiAgICB2YXIgZGF0YSA9IHt1c2VyOiBuYW1lLCBwYXNzOiBwYXNzd29yZCwgc2Vjb25kX2ZhY3Rvcl90b2tlbjogdG9rZW4sIGludml0ZV90b2tlbjogaW52aXRlVG9rZW59O1xuICAgIHJldHVybiBhcGkucG9zdChjZmcuYXBpLmNyZWF0ZVVzZXJQYXRoLCBkYXRhKVxuICAgICAgLnRoZW4oKHVzZXIpPT57XG4gICAgICAgIHNlc3Npb24uc2V0VXNlckRhdGEodXNlcik7XG4gICAgICAgIGF1dGguX3N0YXJ0VG9rZW5SZWZyZXNoZXIoKTtcbiAgICAgICAgcmV0dXJuIHVzZXI7XG4gICAgICB9KTtcbiAgfSxcblxuICBsb2dpbihuYW1lLCBwYXNzd29yZCwgdG9rZW4pe1xuICAgIGF1dGguX3N0b3BUb2tlblJlZnJlc2hlcigpO1xuICAgIHJldHVybiBhdXRoLl9sb2dpbihuYW1lLCBwYXNzd29yZCwgdG9rZW4pLmRvbmUoYXV0aC5fc3RhcnRUb2tlblJlZnJlc2hlcik7XG4gIH0sXG5cbiAgZW5zdXJlVXNlcigpe1xuICAgIHZhciB1c2VyRGF0YSA9IHNlc3Npb24uZ2V0VXNlckRhdGEoKTtcbiAgICBpZih1c2VyRGF0YS50b2tlbil7XG4gICAgICAvLyByZWZyZXNoIHRpbWVyIHdpbGwgbm90IGJlIHNldCBpbiBjYXNlIG9mIGJyb3dzZXIgcmVmcmVzaCBldmVudFxuICAgICAgaWYoYXV0aC5fZ2V0UmVmcmVzaFRva2VuVGltZXJJZCgpID09PSBudWxsKXtcbiAgICAgICAgcmV0dXJuIGF1dGguX3JlZnJlc2hUb2tlbigpLmRvbmUoYXV0aC5fc3RhcnRUb2tlblJlZnJlc2hlcik7XG4gICAgICB9XG5cbiAgICAgIHJldHVybiAkLkRlZmVycmVkKCkucmVzb2x2ZSh1c2VyRGF0YSk7XG4gICAgfVxuXG4gICAgcmV0dXJuICQuRGVmZXJyZWQoKS5yZWplY3QoKTtcbiAgfSxcblxuICBsb2dvdXQoKXtcbiAgICBhdXRoLl9zdG9wVG9rZW5SZWZyZXNoZXIoKTtcbiAgICBzZXNzaW9uLmNsZWFyKCk7XG4gICAgc2Vzc2lvbi5nZXRIaXN0b3J5KCkucmVwbGFjZSh7cGF0aG5hbWU6IGNmZy5yb3V0ZXMubG9naW59KTsgICAgXG4gIH0sXG5cbiAgX3N0YXJ0VG9rZW5SZWZyZXNoZXIoKXtcbiAgICByZWZyZXNoVG9rZW5UaW1lcklkID0gc2V0SW50ZXJ2YWwoYXV0aC5fcmVmcmVzaFRva2VuLCByZWZyZXNoUmF0ZSk7XG4gIH0sXG5cbiAgX3N0b3BUb2tlblJlZnJlc2hlcigpe1xuICAgIGNsZWFySW50ZXJ2YWwocmVmcmVzaFRva2VuVGltZXJJZCk7XG4gICAgcmVmcmVzaFRva2VuVGltZXJJZCA9IG51bGw7XG4gIH0sXG5cbiAgX2dldFJlZnJlc2hUb2tlblRpbWVySWQoKXtcbiAgICByZXR1cm4gcmVmcmVzaFRva2VuVGltZXJJZDtcbiAgfSxcblxuICBfcmVmcmVzaFRva2VuKCl7XG4gICAgcmV0dXJuIGFwaS5wb3N0KGNmZy5hcGkucmVuZXdUb2tlblBhdGgpLnRoZW4oZGF0YT0+e1xuICAgICAgc2Vzc2lvbi5zZXRVc2VyRGF0YShkYXRhKTtcbiAgICAgIHJldHVybiBkYXRhO1xuICAgIH0pLmZhaWwoKCk9PntcbiAgICAgIGF1dGgubG9nb3V0KCk7XG4gICAgfSk7XG4gIH0sXG5cbiAgX2xvZ2luKG5hbWUsIHBhc3N3b3JkLCB0b2tlbil7XG4gICAgdmFyIGRhdGEgPSB7XG4gICAgICB1c2VyOiBuYW1lLFxuICAgICAgcGFzczogcGFzc3dvcmQsXG4gICAgICBzZWNvbmRfZmFjdG9yX3Rva2VuOiB0b2tlblxuICAgIH07XG5cbiAgICByZXR1cm4gYXBpLnBvc3QoY2ZnLmFwaS5zZXNzaW9uUGF0aCwgZGF0YSwgZmFsc2UpLnRoZW4oZGF0YT0+e1xuICAgICAgc2Vzc2lvbi5zZXRVc2VyRGF0YShkYXRhKTtcbiAgICAgIHJldHVybiBkYXRhO1xuICAgIH0pO1xuICB9XG59XG5cbm1vZHVsZS5leHBvcnRzID0gYXV0aDtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9hdXRoLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfVEVSTV9PUEVOOiBudWxsLFxuICBUTFBUX1RFUk1fQ0xPU0U6IG51bGwgIFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGlvblR5cGVzLmpzXG4gKiovIiwidmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG52YXIgeyBUTFBUX1RFUk1fT1BFTiwgVExQVF9URVJNX0NMT1NFIH0gID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gdG9JbW11dGFibGUobnVsbCk7XG4gIH0sXG5cbiAgaW5pdGlhbGl6ZSgpIHtcbiAgICB0aGlzLm9uKFRMUFRfVEVSTV9PUEVOLCBzZXRBY3RpdmVUZXJtaW5hbCk7XG4gICAgdGhpcy5vbihUTFBUX1RFUk1fQ0xPU0UsIGNsb3NlKTtcbiAgfVxufSlcblxuZnVuY3Rpb24gY2xvc2UoKXtcbiAgcmV0dXJuIHRvSW1tdXRhYmxlKG51bGwpO1xufVxuXG5mdW5jdGlvbiBzZXRBY3RpdmVUZXJtaW5hbChzdGF0ZSwge3NlcnZlcklkLCBsb2dpbiwgc2lkLCBpc05ld1Nlc3Npb259ICl7XG4gIHJldHVybiB0b0ltbXV0YWJsZSh7XG4gICAgc2VydmVySWQsXG4gICAgbG9naW4sXG4gICAgc2lkLFxuICAgIGlzTmV3U2Vzc2lvblxuICB9KTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGl2ZVRlcm1TdG9yZS5qc1xuICoqLyIsImNvbnN0IGFjdGl2ZVNlc3Npb24gPSBbXG5bJ3RscHRfYWN0aXZlX3Rlcm1pbmFsJ10sIFsndGxwdF9zZXNzaW9ucyddLFxuKGFjdGl2ZVRlcm0sIHNlc3Npb25zKSA9PiB7XG4gICAgaWYoIWFjdGl2ZVRlcm0pe1xuICAgICAgcmV0dXJuIG51bGw7XG4gICAgfVxuXG4gICAgbGV0IHZpZXcgPSB7XG4gICAgICBpc05ld1Nlc3Npb246IGFjdGl2ZVRlcm0uZ2V0KCdpc05ld1Nlc3Npb24nKSxcbiAgICAgIG5vdEZvdW5kOiBhY3RpdmVUZXJtLmdldCgnbm90Rm91bmQnKSxcbiAgICAgIGFkZHI6IGFjdGl2ZVRlcm0uZ2V0KCdhZGRyJyksXG4gICAgICBzZXJ2ZXJJZDogYWN0aXZlVGVybS5nZXQoJ3NlcnZlcklkJyksXG4gICAgICBsb2dpbjogYWN0aXZlVGVybS5nZXQoJ2xvZ2luJyksXG4gICAgICBzaWQ6IGFjdGl2ZVRlcm0uZ2V0KCdzaWQnKSxcbiAgICAgIGNvbHM6IHVuZGVmaW5lZCxcbiAgICAgIHJvd3M6IHVuZGVmaW5lZFxuICAgIH07XG5cbiAgICBpZihzZXNzaW9ucy5oYXModmlldy5zaWQpKXtcbiAgICAgIHZpZXcuY29scyA9IHNlc3Npb25zLmdldEluKFt2aWV3LnNpZCwgJ3Rlcm1pbmFsX3BhcmFtcycsICd3J10pO1xuICAgICAgdmlldy5yb3dzID0gc2Vzc2lvbnMuZ2V0SW4oW3ZpZXcuc2lkLCAndGVybWluYWxfcGFyYW1zJywgJ2gnXSk7XG4gICAgfVxuXG4gICAgcmV0dXJuIHZpZXc7XG5cbiAgfVxuXTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICBhY3RpdmVTZXNzaW9uXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9nZXR0ZXJzLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMuZ2V0dGVycyA9IHJlcXVpcmUoJy4vZ2V0dGVycycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aW9ucyA9IHJlcXVpcmUoJy4vYWN0aW9ucycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aXZlVGVybVN0b3JlID0gcmVxdWlyZSgnLi9hY3RpdmVUZXJtU3RvcmUnKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2luZGV4LmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfQVBQX0lOSVQ6IG51bGwsXG4gIFRMUFRfQVBQX0ZBSUxFRDogbnVsbCxcbiAgVExQVF9BUFBfUkVBRFk6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hcHAvYWN0aW9uVHlwZXMuanNcbiAqKi8iLCJ2YXIgeyBTdG9yZSwgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcblxudmFyIHsgVExQVF9BUFBfSU5JVCwgVExQVF9BUFBfRkFJTEVELCBUTFBUX0FQUF9SRUFEWSB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG52YXIgaW5pdFN0YXRlID0gdG9JbW11dGFibGUoe1xuICBpc1JlYWR5OiBmYWxzZSxcbiAgaXNJbml0aWFsaXppbmc6IGZhbHNlLFxuICBpc0ZhaWxlZDogZmFsc2Vcbn0pO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG5cbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiBpbml0U3RhdGUuc2V0KCdpc0luaXRpYWxpemluZycsIHRydWUpO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX0FQUF9JTklULCAoKT0+IGluaXRTdGF0ZS5zZXQoJ2lzSW5pdGlhbGl6aW5nJywgdHJ1ZSkpO1xuICAgIHRoaXMub24oVExQVF9BUFBfUkVBRFksKCk9PiBpbml0U3RhdGUuc2V0KCdpc1JlYWR5JywgdHJ1ZSkpO1xuICAgIHRoaXMub24oVExQVF9BUFBfRkFJTEVELCgpPT4gaW5pdFN0YXRlLnNldCgnaXNGYWlsZWQnLCB0cnVlKSk7XG4gIH1cbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hcHAvYXBwU3RvcmUuanNcbiAqKi8iLCJpbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVExQVF9SRUNFSVZFX1VTRVJfSU5WSVRFOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2FjdGlvblR5cGVzLmpzXG4gKiovIiwidmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG52YXIgIHsgVExQVF9SRUNFSVZFX1VTRVJfSU5WSVRFIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IFN0b3JlKHtcbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB0b0ltbXV0YWJsZShudWxsKTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9SRUNFSVZFX1VTRVJfSU5WSVRFLCByZWNlaXZlSW52aXRlKVxuICB9XG59KVxuXG5mdW5jdGlvbiByZWNlaXZlSW52aXRlKHN0YXRlLCBpbnZpdGUpe1xuICByZXR1cm4gdG9JbW11dGFibGUoaW52aXRlKTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9pbnZpdGVTdG9yZS5qc1xuICoqLyIsInZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyICB7IFRMUFRfTk9ERVNfUkVDRUlWRSB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gdG9JbW11dGFibGUoW10pO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX05PREVTX1JFQ0VJVkUsIHJlY2VpdmVOb2RlcylcbiAgfVxufSlcblxuZnVuY3Rpb24gcmVjZWl2ZU5vZGVzKHN0YXRlLCBub2RlQXJyYXkpe1xuICByZXR1cm4gdG9JbW11dGFibGUobm9kZUFycmF5KTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL25vZGVTdG9yZS5qc1xuICoqLyIsImltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX1JFU1RfQVBJX1NUQVJUOiBudWxsLFxuICBUTFBUX1JFU1RfQVBJX1NVQ0NFU1M6IG51bGwsXG4gIFRMUFRfUkVTVF9BUElfRkFJTDogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvYWN0aW9uVHlwZXMuanNcbiAqKi8iLCJpbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVFJZSU5HX1RPX1NJR05fVVA6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2NvbnN0YW50cy5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciBhcGkgPSByZXF1aXJlKCdhcHAvc2VydmljZXMvYXBpJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xuXG52YXIgeyBUTFBUX1NFU1NJTlNfUkVDRUlWRSwgVExQVF9TRVNTSU5TX1VQREFURSB9ICA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuXG4gIGZldGNoU2Vzc2lvbihzaWQpe1xuICAgIHJldHVybiBhcGkuZ2V0KGNmZy5hcGkuZ2V0RmV0Y2hTZXNzaW9uVXJsKHNpZCkpLnRoZW4oanNvbj0+e1xuICAgICAgaWYoanNvbiAmJiBqc29uLnNlc3Npb24pe1xuICAgICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfU0VTU0lOU19VUERBVEUsIGpzb24uc2Vzc2lvbik7XG4gICAgICB9XG4gICAgfSk7XG4gIH0sXG5cbiAgdXBkYXRlU2Vzc2lvbihqc29uKXtcbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfU0VTU0lOU19VUERBVEUsIGpzb24pO1xuICB9LFxuXG4gIHJlY2VpdmUoanNvbil7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1NFU1NJTlNfUkVDRUlWRSwganNvbik7XG4gIH1cbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2FjdGlvbnMuanNcbiAqKi8iLCJ2YXIgeyB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxuY29uc3Qgc2Vzc2lvbnNCeVNlcnZlciA9IChzZXJ2ZXJJZCkgPT4gW1sndGxwdF9zZXNzaW9ucyddLCAoc2Vzc2lvbnMpID0+e1xuICByZXR1cm4gc2Vzc2lvbnMudmFsdWVTZXEoKS5maWx0ZXIoaXRlbT0+e1xuICAgIHZhciBwYXJ0aWVzID0gaXRlbS5nZXQoJ3BhcnRpZXMnKSB8fCB0b0ltbXV0YWJsZShbXSk7XG4gICAgdmFyIGhhc1NlcnZlciA9IHBhcnRpZXMuZmluZChpdGVtMj0+IGl0ZW0yLmdldCgnc2VydmVyX2lkJykgPT09IHNlcnZlcklkKTtcbiAgICByZXR1cm4gaGFzU2VydmVyO1xuICB9KS50b0xpc3QoKTtcbn1dXG5cbmNvbnN0IHNlc3Npb25zVmlldyA9IFtbJ3RscHRfc2Vzc2lvbnMnXSwgKHNlc3Npb25zKSA9PntcbiAgcmV0dXJuIHNlc3Npb25zLnZhbHVlU2VxKCkubWFwKGNyZWF0ZVZpZXcpLnRvSlMoKTtcbn1dO1xuXG5jb25zdCBzZXNzaW9uVmlld0J5SWQgPSAoc2lkKT0+IFtbJ3RscHRfc2Vzc2lvbnMnLCBzaWRdLCAoc2Vzc2lvbik9PntcbiAgaWYoIXNlc3Npb24pe1xuICAgIHJldHVybiBudWxsO1xuICB9XG5cbiAgcmV0dXJuIGNyZWF0ZVZpZXcoc2Vzc2lvbik7XG59XTtcblxuY29uc3QgcGFydGllc0J5U2Vzc2lvbklkID0gKHNpZCkgPT5cbiBbWyd0bHB0X3Nlc3Npb25zJywgc2lkLCAncGFydGllcyddLCAocGFydGllcykgPT57XG5cbiAgaWYoIXBhcnRpZXMpe1xuICAgIHJldHVybiBbXTtcbiAgfVxuXG4gIHZhciBsYXN0QWN0aXZlVXNyTmFtZSA9IGdldExhc3RBY3RpdmVVc2VyKHBhcnRpZXMpLmdldCgndXNlcicpO1xuXG4gIHJldHVybiBwYXJ0aWVzLm1hcChpdGVtPT57XG4gICAgdmFyIHVzZXIgPSBpdGVtLmdldCgndXNlcicpO1xuICAgIHJldHVybiB7XG4gICAgICB1c2VyOiBpdGVtLmdldCgndXNlcicpLFxuICAgICAgc2VydmVySXA6IGl0ZW0uZ2V0KCdyZW1vdGVfYWRkcicpLFxuICAgICAgc2VydmVySWQ6IGl0ZW0uZ2V0KCdzZXJ2ZXJfaWQnKSxcbiAgICAgIGlzQWN0aXZlOiBsYXN0QWN0aXZlVXNyTmFtZSA9PT0gdXNlclxuICAgIH1cbiAgfSkudG9KUygpO1xufV07XG5cbmZ1bmN0aW9uIGdldExhc3RBY3RpdmVVc2VyKHBhcnRpZXMpe1xuICByZXR1cm4gcGFydGllcy5zb3J0QnkoaXRlbT0+IG5ldyBEYXRlKGl0ZW0uZ2V0KCdsYXN0QWN0aXZlJykpKS5maXJzdCgpO1xufVxuXG5mdW5jdGlvbiBjcmVhdGVWaWV3KHNlc3Npb24pe1xuICB2YXIgc2lkID0gc2Vzc2lvbi5nZXQoJ2lkJyk7XG4gIHZhciBzZXJ2ZXJJcCwgc2VydmVySWQ7XG4gIHZhciBwYXJ0aWVzID0gcmVhY3Rvci5ldmFsdWF0ZShwYXJ0aWVzQnlTZXNzaW9uSWQoc2lkKSk7XG5cbiAgaWYocGFydGllcy5sZW5ndGggPiAwKXtcbiAgICBzZXJ2ZXJJcCA9IHBhcnRpZXNbMF0uc2VydmVySXA7XG4gICAgc2VydmVySWQgPSBwYXJ0aWVzWzBdLnNlcnZlcklkO1xuICB9XG5cbiAgcmV0dXJuIHtcbiAgICBzaWQ6IHNpZCxcbiAgICBzZXNzaW9uVXJsOiBjZmcuZ2V0QWN0aXZlU2Vzc2lvblJvdXRlVXJsKHNpZCksXG4gICAgc2VydmVySXAsXG4gICAgc2VydmVySWQsXG4gICAgbG9naW46IHNlc3Npb24uZ2V0KCdsb2dpbicpLFxuICAgIHBhcnRpZXM6IHBhcnRpZXNcbiAgfVxufVxuXG5leHBvcnQgZGVmYXVsdCB7XG4gIHBhcnRpZXNCeVNlc3Npb25JZCxcbiAgc2Vzc2lvbnNCeVNlcnZlcixcbiAgc2Vzc2lvbnNWaWV3LFxuICBzZXNzaW9uVmlld0J5SWRcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2dldHRlcnMuanNcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cy5nZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3Rpb25zID0gcmVxdWlyZSgnLi9hY3Rpb25zJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3RpdmVUZXJtU3RvcmUgPSByZXF1aXJlKCcuL3Nlc3Npb25TdG9yZScpO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvaW5kZXguanNcbiAqKi8iLCJ2YXIgeyBTdG9yZSwgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcbnZhciB7IFRMUFRfU0VTU0lOU19SRUNFSVZFLCBUTFBUX1NFU1NJTlNfVVBEQVRFIH0gID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gdG9JbW11dGFibGUoe30pO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX1NFU1NJTlNfUkVDRUlWRSwgcmVjZWl2ZVNlc3Npb25zKTtcbiAgICB0aGlzLm9uKFRMUFRfU0VTU0lOU19VUERBVEUsIHVwZGF0ZVNlc3Npb24pO1xuICB9XG59KVxuXG5mdW5jdGlvbiB1cGRhdGVTZXNzaW9uKHN0YXRlLCBqc29uKXtcbiAgcmV0dXJuIHN0YXRlLnNldChqc29uLmlkLCB0b0ltbXV0YWJsZShqc29uKSk7XG59XG5cbmZ1bmN0aW9uIHJlY2VpdmVTZXNzaW9ucyhzdGF0ZSwganNvbil7XG4gIHJldHVybiB0b0ltbXV0YWJsZShqc29uKTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL3Nlc3Npb25TdG9yZS5qc1xuICoqLyIsImltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX1JFQ0VJVkVfVVNFUjogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvYWN0aW9uVHlwZXMuanNcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgeyBUTFBUX1JFQ0VJVkVfVVNFUiB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xudmFyIHsgVFJZSU5HX1RPX1NJR05fVVB9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvcmVzdEFwaS9jb25zdGFudHMnKTtcbnZhciByZXN0QXBpQWN0aW9ucyA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3Jlc3RBcGkvYWN0aW9ucycpO1xudmFyIGF1dGggPSByZXF1aXJlKCdhcHAvYXV0aCcpO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCdhcHAvc2Vzc2lvbicpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuXG4gIGVuc3VyZVVzZXIobmV4dFN0YXRlLCByZXBsYWNlLCBjYil7XG4gICAgYXV0aC5lbnN1cmVVc2VyKClcbiAgICAgIC5kb25lKCh1c2VyRGF0YSk9PiB7XG4gICAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRUNFSVZFX1VTRVIsIHVzZXJEYXRhLnVzZXIgKTtcbiAgICAgICAgY2IoKTtcbiAgICAgIH0pXG4gICAgICAuZmFpbCgoKT0+e1xuICAgICAgICByZXBsYWNlKHtyZWRpcmVjdFRvOiBuZXh0U3RhdGUubG9jYXRpb24ucGF0aG5hbWUgfSwgY2ZnLnJvdXRlcy5sb2dpbik7XG4gICAgICAgIGNiKCk7XG4gICAgICB9KTtcbiAgfSxcblxuICBzaWduVXAoe25hbWUsIHBzdywgdG9rZW4sIGludml0ZVRva2VufSl7XG4gICAgcmVzdEFwaUFjdGlvbnMuc3RhcnQoVFJZSU5HX1RPX1NJR05fVVApO1xuICAgIGF1dGguc2lnblVwKG5hbWUsIHBzdywgdG9rZW4sIGludml0ZVRva2VuKVxuICAgICAgLmRvbmUoKHNlc3Npb25EYXRhKT0+e1xuICAgICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfUkVDRUlWRV9VU0VSLCBzZXNzaW9uRGF0YS51c2VyKTtcbiAgICAgICAgcmVzdEFwaUFjdGlvbnMuc3VjY2VzcyhUUllJTkdfVE9fU0lHTl9VUCk7XG4gICAgICAgIHNlc3Npb24uZ2V0SGlzdG9yeSgpLnB1c2goe3BhdGhuYW1lOiBjZmcucm91dGVzLmFwcH0pO1xuICAgICAgfSlcbiAgICAgIC5mYWlsKCgpPT57XG4gICAgICAgIHJlc3RBcGlBY3Rpb25zLmZhaWwoVFJZSU5HX1RPX1NJR05fVVAsICdmYWlsZWQgdG8gc2luZyB1cCcpO1xuICAgICAgfSk7XG4gIH0sXG5cbiAgbG9naW4oe3VzZXIsIHBhc3N3b3JkLCB0b2tlbn0sIHJlZGlyZWN0KXtcbiAgICAgIGF1dGgubG9naW4odXNlciwgcGFzc3dvcmQsIHRva2VuKVxuICAgICAgICAuZG9uZSgoc2Vzc2lvbkRhdGEpPT57XG4gICAgICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFQ0VJVkVfVVNFUiwgc2Vzc2lvbkRhdGEudXNlcik7XG4gICAgICAgICAgc2Vzc2lvbi5nZXRIaXN0b3J5KCkucHVzaCh7cGF0aG5hbWU6IHJlZGlyZWN0fSk7XG4gICAgICAgIH0pXG4gICAgICAgIC5mYWlsKCgpPT57XG4gICAgICAgIH0pXG4gICAgfVxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvdXNlci9hY3Rpb25zLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMuZ2V0dGVycyA9IHJlcXVpcmUoJy4vZ2V0dGVycycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aW9ucyA9IHJlcXVpcmUoJy4vYWN0aW9ucycpO1xubW9kdWxlLmV4cG9ydHMubm9kZVN0b3JlID0gcmVxdWlyZSgnLi91c2VyU3RvcmUnKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvaW5kZXguanNcbiAqKi8iLCJ2YXIgeyBTdG9yZSwgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcbnZhciAgeyBUTFBUX1JFQ0VJVkVfVVNFUiB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gdG9JbW11dGFibGUobnVsbCk7XG4gIH0sXG5cbiAgaW5pdGlhbGl6ZSgpIHtcbiAgICB0aGlzLm9uKFRMUFRfUkVDRUlWRV9VU0VSLCByZWNlaXZlVXNlcilcbiAgfVxuXG59KVxuXG5mdW5jdGlvbiByZWNlaXZlVXNlcihzdGF0ZSwgdXNlcil7XG4gIHJldHVybiB0b0ltbXV0YWJsZSh1c2VyKTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvdXNlclN0b3JlLmpzXG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcblxudmFyIEdvb2dsZUF1dGhJbmZvID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWdvb2dsZS1hdXRoXCI+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWdvb2dsZS1hdXRoLWljb25cIj48L2Rpdj5cbiAgICAgICAgPHN0cm9uZz5Hb29nbGUgQXV0aGVudGljYXRvcjwvc3Ryb25nPlxuICAgICAgICA8ZGl2PkRvd25sb2FkIDxhIGhyZWY9XCJodHRwczovL3N1cHBvcnQuZ29vZ2xlLmNvbS9hY2NvdW50cy9hbnN3ZXIvMTA2NjQ0Nz9obD1lblwiPkdvb2dsZSBBdXRoZW50aWNhdG9yPC9hPiBvbiB5b3VyIHBob25lIHRvIGFjY2VzcyB5b3VyIHR3byBmYWN0b3J5IHRva2VuPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KVxuXG5tb2R1bGUuZXhwb3J0cyA9IEdvb2dsZUF1dGhJbmZvO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvZ29vZ2xlQXV0aExvZ28uanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcblxudmFyIE5vdEZvdW5kUGFnZSA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgcmVuZGVyKCkge1xuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1wYWdlLW5vdGZvdW5kXCI+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWxvZ28tdHBydFwiPlRlbGVwb3J0PC9kaXY+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXdhcm5pbmdcIj48aSBjbGFzc05hbWU9XCJmYSBmYS13YXJuaW5nXCI+PC9pPiA8L2Rpdj5cbiAgICAgICAgPGgxPldob29wcywgd2UgY2Fubm90IGZpbmQgdGhhdDwvaDE+XG4gICAgICAgIDxkaXY+TG9va3MgbGlrZSB0aGUgcGFnZSB5b3UgYXJlIGxvb2tpbmcgZm9yIGlzbid0IGhlcmUgYW55IGxvbmdlcjwvZGl2PlxuICAgICAgICA8ZGl2PklmIHlvdSBiZWxpZXZlIHRoaXMgaXMgYW4gZXJyb3IsIHBsZWFzZSBjb250YWN0IHlvdXIgb3JnYW5pemF0aW9uIGFkbWluaXN0cmF0b3IuPC9kaXY+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiY29udGFjdC1zZWN0aW9uXCI+SWYgeW91IGJlbGlldmUgdGhpcyBpcyBhbiBpc3N1ZSB3aXRoIFRlbGVwb3J0LCBwbGVhc2UgPGEgaHJlZj1cImh0dHBzOi8vZ2l0aHViLmNvbS9ncmF2aXRhdGlvbmFsL3RlbGVwb3J0L2lzc3Vlcy9uZXdcIj5jcmVhdGUgYSBHaXRIdWIgaXNzdWUuPC9hPlxuICAgICAgICAgPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KVxuXG5tb2R1bGUuZXhwb3J0cyA9IE5vdEZvdW5kUGFnZTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL25vdEZvdW5kUGFnZS5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xuXG5jb25zdCBHcnZUYWJsZVRleHRDZWxsID0gKHtyb3dJbmRleCwgZGF0YSwgY29sdW1uS2V5LCAuLi5wcm9wc30pID0+IChcbiAgPEdydlRhYmxlQ2VsbCB7Li4ucHJvcHN9PlxuICAgIHtkYXRhW3Jvd0luZGV4XVtjb2x1bW5LZXldfVxuICA8L0dydlRhYmxlQ2VsbD5cbik7XG5cbnZhciBHcnZUYWJsZUNlbGwgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIHJlbmRlcigpe1xuICAgIHZhciBwcm9wcyA9IHRoaXMucHJvcHM7XG4gICAgcmV0dXJuIHByb3BzLmlzSGVhZGVyID8gPHRoIGtleT17cHJvcHMua2V5fT57cHJvcHMuY2hpbGRyZW59PC90aD4gOiA8dGQga2V5PXtwcm9wcy5rZXl9Pntwcm9wcy5jaGlsZHJlbn08L3RkPjtcbiAgfVxufSk7XG5cbnZhciBHcnZUYWJsZSA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICByZW5kZXJIZWFkZXIoY2hpbGRyZW4pe1xuICAgIHZhciBjZWxscyA9IGNoaWxkcmVuLm1hcCgoaXRlbSwgaW5kZXgpPT57XG4gICAgICByZXR1cm4gdGhpcy5yZW5kZXJDZWxsKGl0ZW0ucHJvcHMuaGVhZGVyLCB7aW5kZXgsIGtleTogaW5kZXgsIGlzSGVhZGVyOiB0cnVlLCAuLi5pdGVtLnByb3BzfSk7XG4gICAgfSlcblxuICAgIHJldHVybiA8dGhlYWQ+PHRyPntjZWxsc308L3RyPjwvdGhlYWQ+XG4gIH0sXG5cbiAgcmVuZGVyQm9keShjaGlsZHJlbil7XG4gICAgdmFyIGNvdW50ID0gdGhpcy5wcm9wcy5yb3dDb3VudDtcbiAgICB2YXIgcm93cyA9IFtdO1xuICAgIGZvcih2YXIgaSA9IDA7IGkgPCBjb3VudDsgaSArKyl7XG4gICAgICB2YXIgY2VsbHMgPSBjaGlsZHJlbi5tYXAoKGl0ZW0sIGluZGV4KT0+e1xuICAgICAgICByZXR1cm4gdGhpcy5yZW5kZXJDZWxsKGl0ZW0ucHJvcHMuY2VsbCwge3Jvd0luZGV4OiBpLCBrZXk6IGluZGV4LCBpc0hlYWRlcjogZmFsc2UsIC4uLml0ZW0ucHJvcHN9KTtcbiAgICAgIH0pXG5cbiAgICAgIHJvd3MucHVzaCg8dHIga2V5PXtpfT57Y2VsbHN9PC90cj4pO1xuICAgIH1cblxuICAgIHJldHVybiA8dGJvZHk+e3Jvd3N9PC90Ym9keT47XG4gIH0sXG5cbiAgcmVuZGVyQ2VsbChjZWxsLCBjZWxsUHJvcHMpe1xuICAgIHZhciBjb250ZW50ID0gbnVsbDtcbiAgICBpZiAoUmVhY3QuaXNWYWxpZEVsZW1lbnQoY2VsbCkpIHtcbiAgICAgICBjb250ZW50ID0gUmVhY3QuY2xvbmVFbGVtZW50KGNlbGwsIGNlbGxQcm9wcyk7XG4gICAgIH0gZWxzZSBpZiAodHlwZW9mIHByb3BzLmNlbGwgPT09ICdmdW5jdGlvbicpIHtcbiAgICAgICBjb250ZW50ID0gY2VsbChjZWxsUHJvcHMpO1xuICAgICB9XG5cbiAgICAgcmV0dXJuIGNvbnRlbnQ7XG4gIH0sXG5cbiAgcmVuZGVyKCkge1xuICAgIHZhciBjaGlsZHJlbiA9IFtdO1xuICAgIFJlYWN0LkNoaWxkcmVuLmZvckVhY2godGhpcy5wcm9wcy5jaGlsZHJlbiwgKGNoaWxkLCBpbmRleCkgPT4ge1xuICAgICAgaWYgKGNoaWxkID09IG51bGwpIHtcbiAgICAgICAgcmV0dXJuO1xuICAgICAgfVxuXG4gICAgICBpZihjaGlsZC50eXBlLmRpc3BsYXlOYW1lICE9PSAnR3J2VGFibGVDb2x1bW4nKXtcbiAgICAgICAgdGhyb3cgJ1Nob3VsZCBiZSBHcnZUYWJsZUNvbHVtbic7XG4gICAgICB9XG5cbiAgICAgIGNoaWxkcmVuLnB1c2goY2hpbGQpO1xuICAgIH0pO1xuXG4gICAgdmFyIHRhYmxlQ2xhc3MgPSAndGFibGUgJyArIHRoaXMucHJvcHMuY2xhc3NOYW1lO1xuXG4gICAgcmV0dXJuIChcbiAgICAgIDx0YWJsZSBjbGFzc05hbWU9e3RhYmxlQ2xhc3N9PlxuICAgICAgICB7dGhpcy5yZW5kZXJIZWFkZXIoY2hpbGRyZW4pfVxuICAgICAgICB7dGhpcy5yZW5kZXJCb2R5KGNoaWxkcmVuKX1cbiAgICAgIDwvdGFibGU+XG4gICAgKTtcbiAgfVxufSlcblxudmFyIEdydlRhYmxlQ29sdW1uID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIHRocm93IG5ldyBFcnJvcignQ29tcG9uZW50IDxHcnZUYWJsZUNvbHVtbiAvPiBzaG91bGQgbmV2ZXIgcmVuZGVyJyk7XG4gIH1cbn0pXG5cbmV4cG9ydCBkZWZhdWx0IEdydlRhYmxlO1xuZXhwb3J0IHtHcnZUYWJsZUNvbHVtbiBhcyBDb2x1bW4sIEdydlRhYmxlIGFzIFRhYmxlLCBHcnZUYWJsZUNlbGwgYXMgQ2VsbCwgR3J2VGFibGVUZXh0Q2VsbCBhcyBUZXh0Q2VsbH07XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy90YWJsZS5qc3hcbiAqKi8iLCIvKlxuICogIFRoZSBNSVQgTGljZW5zZSAoTUlUKVxuICogIENvcHlyaWdodCAoYykgMjAxNSBSeWFuIEZsb3JlbmNlLCBNaWNoYWVsIEphY2tzb25cbiAqICBQZXJtaXNzaW9uIGlzIGhlcmVieSBncmFudGVkLCBmcmVlIG9mIGNoYXJnZSwgdG8gYW55IHBlcnNvbiBvYnRhaW5pbmcgYSBjb3B5IG9mIHRoaXMgc29mdHdhcmUgYW5kIGFzc29jaWF0ZWQgZG9jdW1lbnRhdGlvbiBmaWxlcyAodGhlIFwiU29mdHdhcmVcIiksIHRvIGRlYWwgaW4gdGhlIFNvZnR3YXJlIHdpdGhvdXQgcmVzdHJpY3Rpb24sIGluY2x1ZGluZyB3aXRob3V0IGxpbWl0YXRpb24gdGhlIHJpZ2h0cyB0byB1c2UsIGNvcHksIG1vZGlmeSwgbWVyZ2UsIHB1Ymxpc2gsIGRpc3RyaWJ1dGUsIHN1YmxpY2Vuc2UsIGFuZC9vciBzZWxsIGNvcGllcyBvZiB0aGUgU29mdHdhcmUsIGFuZCB0byBwZXJtaXQgcGVyc29ucyB0byB3aG9tIHRoZSBTb2Z0d2FyZSBpcyBmdXJuaXNoZWQgdG8gZG8gc28sIHN1YmplY3QgdG8gdGhlIGZvbGxvd2luZyBjb25kaXRpb25zOlxuICogIFRoZSBhYm92ZSBjb3B5cmlnaHQgbm90aWNlIGFuZCB0aGlzIHBlcm1pc3Npb24gbm90aWNlIHNoYWxsIGJlIGluY2x1ZGVkIGluIGFsbCBjb3BpZXMgb3Igc3Vic3RhbnRpYWwgcG9ydGlvbnMgb2YgdGhlIFNvZnR3YXJlLlxuICogIFRIRSBTT0ZUV0FSRSBJUyBQUk9WSURFRCBcIkFTIElTXCIsIFdJVEhPVVQgV0FSUkFOVFkgT0YgQU5ZIEtJTkQsIEVYUFJFU1MgT1IgSU1QTElFRCwgSU5DTFVESU5HIEJVVCBOT1QgTElNSVRFRCBUTyBUSEUgV0FSUkFOVElFUyBPRiBNRVJDSEFOVEFCSUxJVFksIEZJVE5FU1MgRk9SIEEgUEFSVElDVUxBUiBQVVJQT1NFIEFORCBOT05JTkZSSU5HRU1FTlQuIElOIE5PIEVWRU5UIFNIQUxMIFRIRSBBVVRIT1JTIE9SIENPUFlSSUdIVCBIT0xERVJTIEJFIExJQUJMRSBGT1IgQU5ZIENMQUlNLCBEQU1BR0VTIE9SIE9USEVSIExJQUJJTElUWSwgV0hFVEhFUiBJTiBBTiBBQ1RJT04gT0YgQ09OVFJBQ1QsIFRPUlQgT1IgT1RIRVJXSVNFLCBBUklTSU5HIEZST00sIE9VVCBPRiBPUiBJTiBDT05ORUNUSU9OIFdJVEggVEhFIFNPRlRXQVJFIE9SIFRIRSBVU0UgT1IgT1RIRVIgREVBTElOR1MgSU4gVEhFIFNPRlRXQVJFLlxuKi9cblxuaW1wb3J0IGludmFyaWFudCBmcm9tICdpbnZhcmlhbnQnXG5cbmZ1bmN0aW9uIGVzY2FwZVJlZ0V4cChzdHJpbmcpIHtcbiAgcmV0dXJuIHN0cmluZy5yZXBsYWNlKC9bLiorP14ke30oKXxbXFxdXFxcXF0vZywgJ1xcXFwkJicpXG59XG5cbmZ1bmN0aW9uIGVzY2FwZVNvdXJjZShzdHJpbmcpIHtcbiAgcmV0dXJuIGVzY2FwZVJlZ0V4cChzdHJpbmcpLnJlcGxhY2UoL1xcLysvZywgJy8rJylcbn1cblxuZnVuY3Rpb24gX2NvbXBpbGVQYXR0ZXJuKHBhdHRlcm4pIHtcbiAgbGV0IHJlZ2V4cFNvdXJjZSA9ICcnO1xuICBjb25zdCBwYXJhbU5hbWVzID0gW107XG4gIGNvbnN0IHRva2VucyA9IFtdO1xuXG4gIGxldCBtYXRjaCwgbGFzdEluZGV4ID0gMCwgbWF0Y2hlciA9IC86KFthLXpBLVpfJF1bYS16QS1aMC05XyRdKil8XFwqXFwqfFxcKnxcXCh8XFwpL2dcbiAgLyplc2xpbnQgbm8tY29uZC1hc3NpZ246IDAqL1xuICB3aGlsZSAoKG1hdGNoID0gbWF0Y2hlci5leGVjKHBhdHRlcm4pKSkge1xuICAgIGlmIChtYXRjaC5pbmRleCAhPT0gbGFzdEluZGV4KSB7XG4gICAgICB0b2tlbnMucHVzaChwYXR0ZXJuLnNsaWNlKGxhc3RJbmRleCwgbWF0Y2guaW5kZXgpKVxuICAgICAgcmVnZXhwU291cmNlICs9IGVzY2FwZVNvdXJjZShwYXR0ZXJuLnNsaWNlKGxhc3RJbmRleCwgbWF0Y2guaW5kZXgpKVxuICAgIH1cblxuICAgIGlmIChtYXRjaFsxXSkge1xuICAgICAgcmVnZXhwU291cmNlICs9ICcoW14vPyNdKyknO1xuICAgICAgcGFyYW1OYW1lcy5wdXNoKG1hdGNoWzFdKTtcbiAgICB9IGVsc2UgaWYgKG1hdGNoWzBdID09PSAnKionKSB7XG4gICAgICByZWdleHBTb3VyY2UgKz0gJyhbXFxcXHNcXFxcU10qKSdcbiAgICAgIHBhcmFtTmFtZXMucHVzaCgnc3BsYXQnKTtcbiAgICB9IGVsc2UgaWYgKG1hdGNoWzBdID09PSAnKicpIHtcbiAgICAgIHJlZ2V4cFNvdXJjZSArPSAnKFtcXFxcc1xcXFxTXSo/KSdcbiAgICAgIHBhcmFtTmFtZXMucHVzaCgnc3BsYXQnKTtcbiAgICB9IGVsc2UgaWYgKG1hdGNoWzBdID09PSAnKCcpIHtcbiAgICAgIHJlZ2V4cFNvdXJjZSArPSAnKD86JztcbiAgICB9IGVsc2UgaWYgKG1hdGNoWzBdID09PSAnKScpIHtcbiAgICAgIHJlZ2V4cFNvdXJjZSArPSAnKT8nO1xuICAgIH1cblxuICAgIHRva2Vucy5wdXNoKG1hdGNoWzBdKTtcblxuICAgIGxhc3RJbmRleCA9IG1hdGNoZXIubGFzdEluZGV4O1xuICB9XG5cbiAgaWYgKGxhc3RJbmRleCAhPT0gcGF0dGVybi5sZW5ndGgpIHtcbiAgICB0b2tlbnMucHVzaChwYXR0ZXJuLnNsaWNlKGxhc3RJbmRleCwgcGF0dGVybi5sZW5ndGgpKVxuICAgIHJlZ2V4cFNvdXJjZSArPSBlc2NhcGVTb3VyY2UocGF0dGVybi5zbGljZShsYXN0SW5kZXgsIHBhdHRlcm4ubGVuZ3RoKSlcbiAgfVxuXG4gIHJldHVybiB7XG4gICAgcGF0dGVybixcbiAgICByZWdleHBTb3VyY2UsXG4gICAgcGFyYW1OYW1lcyxcbiAgICB0b2tlbnNcbiAgfVxufVxuXG5jb25zdCBDb21waWxlZFBhdHRlcm5zQ2FjaGUgPSB7fVxuXG5leHBvcnQgZnVuY3Rpb24gY29tcGlsZVBhdHRlcm4ocGF0dGVybikge1xuICBpZiAoIShwYXR0ZXJuIGluIENvbXBpbGVkUGF0dGVybnNDYWNoZSkpXG4gICAgQ29tcGlsZWRQYXR0ZXJuc0NhY2hlW3BhdHRlcm5dID0gX2NvbXBpbGVQYXR0ZXJuKHBhdHRlcm4pXG5cbiAgcmV0dXJuIENvbXBpbGVkUGF0dGVybnNDYWNoZVtwYXR0ZXJuXVxufVxuXG4vKipcbiAqIEF0dGVtcHRzIHRvIG1hdGNoIGEgcGF0dGVybiBvbiB0aGUgZ2l2ZW4gcGF0aG5hbWUuIFBhdHRlcm5zIG1heSB1c2VcbiAqIHRoZSBmb2xsb3dpbmcgc3BlY2lhbCBjaGFyYWN0ZXJzOlxuICpcbiAqIC0gOnBhcmFtTmFtZSAgICAgTWF0Y2hlcyBhIFVSTCBzZWdtZW50IHVwIHRvIHRoZSBuZXh0IC8sID8sIG9yICMuIFRoZVxuICogICAgICAgICAgICAgICAgICBjYXB0dXJlZCBzdHJpbmcgaXMgY29uc2lkZXJlZCBhIFwicGFyYW1cIlxuICogLSAoKSAgICAgICAgICAgICBXcmFwcyBhIHNlZ21lbnQgb2YgdGhlIFVSTCB0aGF0IGlzIG9wdGlvbmFsXG4gKiAtICogICAgICAgICAgICAgIENvbnN1bWVzIChub24tZ3JlZWR5KSBhbGwgY2hhcmFjdGVycyB1cCB0byB0aGUgbmV4dFxuICogICAgICAgICAgICAgICAgICBjaGFyYWN0ZXIgaW4gdGhlIHBhdHRlcm4sIG9yIHRvIHRoZSBlbmQgb2YgdGhlIFVSTCBpZlxuICogICAgICAgICAgICAgICAgICB0aGVyZSBpcyBub25lXG4gKiAtICoqICAgICAgICAgICAgIENvbnN1bWVzIChncmVlZHkpIGFsbCBjaGFyYWN0ZXJzIHVwIHRvIHRoZSBuZXh0IGNoYXJhY3RlclxuICogICAgICAgICAgICAgICAgICBpbiB0aGUgcGF0dGVybiwgb3IgdG8gdGhlIGVuZCBvZiB0aGUgVVJMIGlmIHRoZXJlIGlzIG5vbmVcbiAqXG4gKiBUaGUgcmV0dXJuIHZhbHVlIGlzIGFuIG9iamVjdCB3aXRoIHRoZSBmb2xsb3dpbmcgcHJvcGVydGllczpcbiAqXG4gKiAtIHJlbWFpbmluZ1BhdGhuYW1lXG4gKiAtIHBhcmFtTmFtZXNcbiAqIC0gcGFyYW1WYWx1ZXNcbiAqL1xuZXhwb3J0IGZ1bmN0aW9uIG1hdGNoUGF0dGVybihwYXR0ZXJuLCBwYXRobmFtZSkge1xuICAvLyBNYWtlIGxlYWRpbmcgc2xhc2hlcyBjb25zaXN0ZW50IGJldHdlZW4gcGF0dGVybiBhbmQgcGF0aG5hbWUuXG4gIGlmIChwYXR0ZXJuLmNoYXJBdCgwKSAhPT0gJy8nKSB7XG4gICAgcGF0dGVybiA9IGAvJHtwYXR0ZXJufWBcbiAgfVxuICBpZiAocGF0aG5hbWUuY2hhckF0KDApICE9PSAnLycpIHtcbiAgICBwYXRobmFtZSA9IGAvJHtwYXRobmFtZX1gXG4gIH1cblxuICBsZXQgeyByZWdleHBTb3VyY2UsIHBhcmFtTmFtZXMsIHRva2VucyB9ID0gY29tcGlsZVBhdHRlcm4ocGF0dGVybilcblxuICByZWdleHBTb3VyY2UgKz0gJy8qJyAvLyBDYXB0dXJlIHBhdGggc2VwYXJhdG9yc1xuXG4gIC8vIFNwZWNpYWwtY2FzZSBwYXR0ZXJucyBsaWtlICcqJyBmb3IgY2F0Y2gtYWxsIHJvdXRlcy5cbiAgY29uc3QgY2FwdHVyZVJlbWFpbmluZyA9IHRva2Vuc1t0b2tlbnMubGVuZ3RoIC0gMV0gIT09ICcqJ1xuXG4gIGlmIChjYXB0dXJlUmVtYWluaW5nKSB7XG4gICAgLy8gVGhpcyB3aWxsIG1hdGNoIG5ld2xpbmVzIGluIHRoZSByZW1haW5pbmcgcGF0aC5cbiAgICByZWdleHBTb3VyY2UgKz0gJyhbXFxcXHNcXFxcU10qPyknXG4gIH1cblxuICBjb25zdCBtYXRjaCA9IHBhdGhuYW1lLm1hdGNoKG5ldyBSZWdFeHAoJ14nICsgcmVnZXhwU291cmNlICsgJyQnLCAnaScpKVxuXG4gIGxldCByZW1haW5pbmdQYXRobmFtZSwgcGFyYW1WYWx1ZXNcbiAgaWYgKG1hdGNoICE9IG51bGwpIHtcbiAgICBpZiAoY2FwdHVyZVJlbWFpbmluZykge1xuICAgICAgcmVtYWluaW5nUGF0aG5hbWUgPSBtYXRjaC5wb3AoKVxuICAgICAgY29uc3QgbWF0Y2hlZFBhdGggPVxuICAgICAgICBtYXRjaFswXS5zdWJzdHIoMCwgbWF0Y2hbMF0ubGVuZ3RoIC0gcmVtYWluaW5nUGF0aG5hbWUubGVuZ3RoKVxuXG4gICAgICAvLyBJZiB3ZSBkaWRuJ3QgbWF0Y2ggdGhlIGVudGlyZSBwYXRobmFtZSwgdGhlbiBtYWtlIHN1cmUgdGhhdCB0aGUgbWF0Y2hcbiAgICAgIC8vIHdlIGRpZCBnZXQgZW5kcyBhdCBhIHBhdGggc2VwYXJhdG9yIChwb3RlbnRpYWxseSB0aGUgb25lIHdlIGFkZGVkXG4gICAgICAvLyBhYm92ZSBhdCB0aGUgYmVnaW5uaW5nIG9mIHRoZSBwYXRoLCBpZiB0aGUgYWN0dWFsIG1hdGNoIHdhcyBlbXB0eSkuXG4gICAgICBpZiAoXG4gICAgICAgIHJlbWFpbmluZ1BhdGhuYW1lICYmXG4gICAgICAgIG1hdGNoZWRQYXRoLmNoYXJBdChtYXRjaGVkUGF0aC5sZW5ndGggLSAxKSAhPT0gJy8nXG4gICAgICApIHtcbiAgICAgICAgcmV0dXJuIHtcbiAgICAgICAgICByZW1haW5pbmdQYXRobmFtZTogbnVsbCxcbiAgICAgICAgICBwYXJhbU5hbWVzLFxuICAgICAgICAgIHBhcmFtVmFsdWVzOiBudWxsXG4gICAgICAgIH1cbiAgICAgIH1cbiAgICB9IGVsc2Uge1xuICAgICAgLy8gSWYgdGhpcyBtYXRjaGVkIGF0IGFsbCwgdGhlbiB0aGUgbWF0Y2ggd2FzIHRoZSBlbnRpcmUgcGF0aG5hbWUuXG4gICAgICByZW1haW5pbmdQYXRobmFtZSA9ICcnXG4gICAgfVxuXG4gICAgcGFyYW1WYWx1ZXMgPSBtYXRjaC5zbGljZSgxKS5tYXAoXG4gICAgICB2ID0+IHYgIT0gbnVsbCA/IGRlY29kZVVSSUNvbXBvbmVudCh2KSA6IHZcbiAgICApXG4gIH0gZWxzZSB7XG4gICAgcmVtYWluaW5nUGF0aG5hbWUgPSBwYXJhbVZhbHVlcyA9IG51bGxcbiAgfVxuXG4gIHJldHVybiB7XG4gICAgcmVtYWluaW5nUGF0aG5hbWUsXG4gICAgcGFyYW1OYW1lcyxcbiAgICBwYXJhbVZhbHVlc1xuICB9XG59XG5cbmV4cG9ydCBmdW5jdGlvbiBnZXRQYXJhbU5hbWVzKHBhdHRlcm4pIHtcbiAgcmV0dXJuIGNvbXBpbGVQYXR0ZXJuKHBhdHRlcm4pLnBhcmFtTmFtZXNcbn1cblxuZXhwb3J0IGZ1bmN0aW9uIGdldFBhcmFtcyhwYXR0ZXJuLCBwYXRobmFtZSkge1xuICBjb25zdCB7IHBhcmFtTmFtZXMsIHBhcmFtVmFsdWVzIH0gPSBtYXRjaFBhdHRlcm4ocGF0dGVybiwgcGF0aG5hbWUpXG5cbiAgaWYgKHBhcmFtVmFsdWVzICE9IG51bGwpIHtcbiAgICByZXR1cm4gcGFyYW1OYW1lcy5yZWR1Y2UoZnVuY3Rpb24gKG1lbW8sIHBhcmFtTmFtZSwgaW5kZXgpIHtcbiAgICAgIG1lbW9bcGFyYW1OYW1lXSA9IHBhcmFtVmFsdWVzW2luZGV4XVxuICAgICAgcmV0dXJuIG1lbW9cbiAgICB9LCB7fSlcbiAgfVxuXG4gIHJldHVybiBudWxsXG59XG5cbi8qKlxuICogUmV0dXJucyBhIHZlcnNpb24gb2YgdGhlIGdpdmVuIHBhdHRlcm4gd2l0aCBwYXJhbXMgaW50ZXJwb2xhdGVkLiBUaHJvd3NcbiAqIGlmIHRoZXJlIGlzIGEgZHluYW1pYyBzZWdtZW50IG9mIHRoZSBwYXR0ZXJuIGZvciB3aGljaCB0aGVyZSBpcyBubyBwYXJhbS5cbiAqL1xuZXhwb3J0IGZ1bmN0aW9uIGZvcm1hdFBhdHRlcm4ocGF0dGVybiwgcGFyYW1zKSB7XG4gIHBhcmFtcyA9IHBhcmFtcyB8fCB7fVxuXG4gIGNvbnN0IHsgdG9rZW5zIH0gPSBjb21waWxlUGF0dGVybihwYXR0ZXJuKVxuICBsZXQgcGFyZW5Db3VudCA9IDAsIHBhdGhuYW1lID0gJycsIHNwbGF0SW5kZXggPSAwXG5cbiAgbGV0IHRva2VuLCBwYXJhbU5hbWUsIHBhcmFtVmFsdWVcbiAgZm9yIChsZXQgaSA9IDAsIGxlbiA9IHRva2Vucy5sZW5ndGg7IGkgPCBsZW47ICsraSkge1xuICAgIHRva2VuID0gdG9rZW5zW2ldXG5cbiAgICBpZiAodG9rZW4gPT09ICcqJyB8fCB0b2tlbiA9PT0gJyoqJykge1xuICAgICAgcGFyYW1WYWx1ZSA9IEFycmF5LmlzQXJyYXkocGFyYW1zLnNwbGF0KSA/IHBhcmFtcy5zcGxhdFtzcGxhdEluZGV4KytdIDogcGFyYW1zLnNwbGF0XG5cbiAgICAgIGludmFyaWFudChcbiAgICAgICAgcGFyYW1WYWx1ZSAhPSBudWxsIHx8IHBhcmVuQ291bnQgPiAwLFxuICAgICAgICAnTWlzc2luZyBzcGxhdCAjJXMgZm9yIHBhdGggXCIlc1wiJyxcbiAgICAgICAgc3BsYXRJbmRleCwgcGF0dGVyblxuICAgICAgKVxuXG4gICAgICBpZiAocGFyYW1WYWx1ZSAhPSBudWxsKVxuICAgICAgICBwYXRobmFtZSArPSBlbmNvZGVVUkkocGFyYW1WYWx1ZSlcbiAgICB9IGVsc2UgaWYgKHRva2VuID09PSAnKCcpIHtcbiAgICAgIHBhcmVuQ291bnQgKz0gMVxuICAgIH0gZWxzZSBpZiAodG9rZW4gPT09ICcpJykge1xuICAgICAgcGFyZW5Db3VudCAtPSAxXG4gICAgfSBlbHNlIGlmICh0b2tlbi5jaGFyQXQoMCkgPT09ICc6Jykge1xuICAgICAgcGFyYW1OYW1lID0gdG9rZW4uc3Vic3RyaW5nKDEpXG4gICAgICBwYXJhbVZhbHVlID0gcGFyYW1zW3BhcmFtTmFtZV1cblxuICAgICAgaW52YXJpYW50KFxuICAgICAgICBwYXJhbVZhbHVlICE9IG51bGwgfHwgcGFyZW5Db3VudCA+IDAsXG4gICAgICAgICdNaXNzaW5nIFwiJXNcIiBwYXJhbWV0ZXIgZm9yIHBhdGggXCIlc1wiJyxcbiAgICAgICAgcGFyYW1OYW1lLCBwYXR0ZXJuXG4gICAgICApXG5cbiAgICAgIGlmIChwYXJhbVZhbHVlICE9IG51bGwpXG4gICAgICAgIHBhdGhuYW1lICs9IGVuY29kZVVSSUNvbXBvbmVudChwYXJhbVZhbHVlKVxuICAgIH0gZWxzZSB7XG4gICAgICBwYXRobmFtZSArPSB0b2tlblxuICAgIH1cbiAgfVxuXG4gIHJldHVybiBwYXRobmFtZS5yZXBsYWNlKC9cXC8rL2csICcvJylcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21tb24vcGF0dGVyblV0aWxzLmpzXG4gKiovIiwidmFyIEV2ZW50RW1pdHRlciA9IHJlcXVpcmUoJ2V2ZW50cycpLkV2ZW50RW1pdHRlcjtcbnZhciBzZXNzaW9uID0gcmVxdWlyZSgnYXBwL3Nlc3Npb24nKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG52YXIge2FjdGlvbnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvJyk7XG5cbmNsYXNzIFR0eSBleHRlbmRzIEV2ZW50RW1pdHRlciB7XG5cbiAgY29uc3RydWN0b3Ioe3NlcnZlcklkLCBsb2dpbiwgc2lkLCByb3dzLCBjb2xzIH0pe1xuICAgIHN1cGVyKCk7XG4gICAgdGhpcy5vcHRpb25zID0geyBzZXJ2ZXJJZCwgbG9naW4sIHNpZCwgcm93cywgY29scyB9O1xuICAgIHRoaXMuc29ja2V0ID0gbnVsbDtcbiAgfVxuXG4gIGRpc2Nvbm5lY3QoKXtcbiAgICB0aGlzLnNvY2tldC5jbG9zZSgpO1xuICB9XG5cbiAgY29ubmVjdChvcHRpb25zKXtcbiAgICBPYmplY3QuYXNzaWduKHRoaXMub3B0aW9ucywgb3B0aW9ucyk7XG5cbiAgICBsZXQge3Rva2VufSA9IHNlc3Npb24uZ2V0VXNlckRhdGEoKTtcbiAgICBsZXQgY29ublN0ciA9IGNmZy5hcGkuZ2V0VHR5Q29ublN0cih7dG9rZW4sIC4uLnRoaXMub3B0aW9uc30pO1xuXG4gICAgdGhpcy5zb2NrZXQgPSBuZXcgV2ViU29ja2V0KGNvbm5TdHIsICdwcm90bycpO1xuXG4gICAgdGhpcy5zb2NrZXQub25vcGVuID0gKCkgPT4ge1xuICAgICAgdGhpcy5lbWl0KCdvcGVuJyk7XG4gICAgfVxuXG4gICAgdGhpcy5zb2NrZXQub25tZXNzYWdlID0gKGUpPT57XG4gICAgICB0aGlzLmVtaXQoJ2RhdGEnLCBlLmRhdGEpO1xuICAgIH1cblxuICAgIHRoaXMuc29ja2V0Lm9uY2xvc2UgPSAoKT0+e1xuICAgICAgdGhpcy5lbWl0KCdjbG9zZScpO1xuICAgIH1cbiAgfVxuXG4gIHJlc2l6ZShjb2xzLCByb3dzKXtcbiAgICBhY3Rpb25zLnJlc2l6ZShjb2xzLCByb3dzKTtcbiAgfVxuXG4gIHNlbmQoZGF0YSl7XG4gICAgdGhpcy5zb2NrZXQuc2VuZChkYXRhKTtcbiAgfVxufVxuXG5tb2R1bGUuZXhwb3J0cyA9IFR0eTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21tb24vdHR5LmpzXG4gKiovIiwidmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIGFwaSA9IHJlcXVpcmUoJ2FwcC9zZXJ2aWNlcy9hcGknKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG5cbnZhciB7IFRMUFRfU0VTU0lOU19SRUNFSVZFIH0gPSByZXF1aXJlKCcuLy4uL3Nlc3Npb25zL2FjdGlvblR5cGVzJyk7XG52YXIgeyBUTFBUX05PREVTX1JFQ0VJVkUgfSA9IHJlcXVpcmUoJy4vLi4vbm9kZXMvYWN0aW9uVHlwZXMnKTtcbnZhciB7IFRMUFRfQVBQX0lOSVQsIFRMUFRfQVBQX0ZBSUxFRCwgVExQVF9BUFBfUkVBRFkgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuXG4gIGluaXRBcHAoKSB7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX0FQUF9JTklUKTtcbiAgICBtb2R1bGUuZXhwb3J0cy5mZXRjaE5vZGVzQW5kU2Vzc2lvbnMoKVxuICAgICAgLmRvbmUoKCk9PntcbiAgICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX0FQUF9SRUFEWSk7XG4gICAgICB9KVxuICAgICAgLmZhaWwoKCk9PntcbiAgICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX0FQUF9GQUlMRUQpO1xuICAgICAgfSk7XG4gIH0sXG5cbiAgZmV0Y2hOb2Rlc0FuZFNlc3Npb25zKCkge1xuICAgIHJldHVybiBhcGkuZ2V0KGNmZy5hcGkubm9kZXNQYXRoKS5kb25lKGpzb24gPT4ge1xuICAgICAgdmFyIG5vZGVBcnJheSA9IFtdO1xuICAgICAgdmFyIHNlc3Npb25zID0ge307XG5cbiAgICAgIGpzb24ubm9kZXMuZm9yRWFjaChpdGVtID0+IHtcbiAgICAgICAgbm9kZUFycmF5LnB1c2goaXRlbS5ub2RlKTtcbiAgICAgICAgaWYgKGl0ZW0uc2Vzc2lvbnMpIHtcbiAgICAgICAgICBpdGVtLnNlc3Npb25zLmZvckVhY2goaXRlbTIgPT4ge1xuICAgICAgICAgICAgc2Vzc2lvbnNbaXRlbTIuaWRdID0gaXRlbTI7XG4gICAgICAgICAgfSlcbiAgICAgICAgfVxuICAgICAgfSk7XG5cbiAgICAgIHJlYWN0b3IuYmF0Y2goKCkgPT4ge1xuICAgICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfTk9ERVNfUkVDRUlWRSwgbm9kZUFycmF5KTtcbiAgICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1NFU1NJTlNfUkVDRUlWRSwgc2Vzc2lvbnMpO1xuICAgICAgfSk7XG5cbiAgICB9KTtcbiAgfVxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvYXBwL2FjdGlvbnMuanNcbiAqKi8iLCJjb25zdCBhcHBTdGF0ZSA9IFtbJ3RscHQnXSwgYXBwPT4gYXBwLnRvSlMoKV07XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgYXBwU3RhdGVcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2FwcC9nZXR0ZXJzLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMuZ2V0dGVycyA9IHJlcXVpcmUoJy4vZ2V0dGVycycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aW9ucyA9IHJlcXVpcmUoJy4vYWN0aW9ucycpO1xubW9kdWxlLmV4cG9ydHMuYXBwU3RvcmUgPSByZXF1aXJlKCcuL2FwcFN0b3JlJyk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hcHAvaW5kZXguanNcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG5yZWFjdG9yLnJlZ2lzdGVyU3RvcmVzKHtcbiAgJ3RscHQnOiByZXF1aXJlKCcuL2FwcC9hcHBTdG9yZScpLFxuICAndGxwdF9hY3RpdmVfdGVybWluYWwnOiByZXF1aXJlKCcuL2FjdGl2ZVRlcm1pbmFsL2FjdGl2ZVRlcm1TdG9yZScpLFxuICAndGxwdF91c2VyJzogcmVxdWlyZSgnLi91c2VyL3VzZXJTdG9yZScpLFxuICAndGxwdF9ub2Rlcyc6IHJlcXVpcmUoJy4vbm9kZXMvbm9kZVN0b3JlJyksXG4gICd0bHB0X2ludml0ZSc6IHJlcXVpcmUoJy4vaW52aXRlL2ludml0ZVN0b3JlJyksXG4gICd0bHB0X3Jlc3RfYXBpJzogcmVxdWlyZSgnLi9yZXN0QXBpL3Jlc3RBcGlTdG9yZScpLFxuICAndGxwdF9zZXNzaW9ucyc6IHJlcXVpcmUoJy4vc2Vzc2lvbnMvc2Vzc2lvblN0b3JlJylcbn0pO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvaW5kZXguanNcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgeyBUTFBUX1JFQ0VJVkVfVVNFUl9JTlZJVEUgfSAgPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG52YXIgYXBpID0gcmVxdWlyZSgnYXBwL3NlcnZpY2VzL2FwaScpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICBmZXRjaEludml0ZShpbnZpdGVUb2tlbil7XG4gICAgdmFyIHBhdGggPSBjZmcuYXBpLmdldEludml0ZVVybChpbnZpdGVUb2tlbik7XG4gICAgYXBpLmdldChwYXRoKS5kb25lKGludml0ZT0+e1xuICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFQ0VJVkVfVVNFUl9JTlZJVEUsIGludml0ZSk7XG4gICAgfSk7XG4gIH1cbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9hY3Rpb25zLmpzXG4gKiovIiwiLyplc2xpbnQgbm8tdW5kZWY6IDAsICBuby11bnVzZWQtdmFyczogMCwgbm8tZGVidWdnZXI6MCovXG5cbnZhciB7VFJZSU5HX1RPX1NJR05fVVB9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvcmVzdEFwaS9jb25zdGFudHMnKTtcblxuY29uc3QgaW52aXRlID0gWyBbJ3RscHRfaW52aXRlJ10sIChpbnZpdGUpID0+IHtcbiAgcmV0dXJuIGludml0ZTtcbiB9XG5dO1xuXG5jb25zdCBhdHRlbXAgPSBbIFsndGxwdF9yZXN0X2FwaScsIFRSWUlOR19UT19TSUdOX1VQXSwgKGF0dGVtcCkgPT4ge1xuICB2YXIgZGVmYXVsdE9iaiA9IHtcbiAgICBpc1Byb2Nlc3Npbmc6IGZhbHNlLFxuICAgIGlzRXJyb3I6IGZhbHNlLFxuICAgIGlzU3VjY2VzczogZmFsc2UsXG4gICAgbWVzc2FnZTogJydcbiAgfVxuXG4gIHJldHVybiBhdHRlbXAgPyBhdHRlbXAudG9KUygpIDogZGVmYXVsdE9iajtcbiAgXG4gfVxuXTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICBpbnZpdGUsXG4gIGF0dGVtcFxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2dldHRlcnMuanNcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cy5nZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3Rpb25zID0gcmVxdWlyZSgnLi9hY3Rpb25zJyk7XG5tb2R1bGUuZXhwb3J0cy5ub2RlU3RvcmUgPSByZXF1aXJlKCcuL2ludml0ZVN0b3JlJyk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvaW5kZXguanNcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgeyBUTFBUX05PREVTX1JFQ0VJVkUgfSAgPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG52YXIgYXBpID0gcmVxdWlyZSgnYXBwL3NlcnZpY2VzL2FwaScpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICBmZXRjaE5vZGVzKCl7XG4gICAgYXBpLmdldChjZmcuYXBpLm5vZGVzUGF0aCkuZG9uZShkYXRhPT57XG4gICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfTk9ERVNfUkVDRUlWRSwgZGF0YS5ub2Rlcyk7XG4gICAgfSk7XG4gIH1cbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2FjdGlvbnMuanNcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIge3Nlc3Npb25zQnlTZXJ2ZXJ9ID0gcmVxdWlyZSgnLi8uLi9zZXNzaW9ucy9nZXR0ZXJzJyk7XG5cbmNvbnN0IG5vZGVMaXN0VmlldyA9IFsgWyd0bHB0X25vZGVzJ10sIChub2RlcykgPT57XG4gICAgcmV0dXJuIG5vZGVzLm1hcCgoaXRlbSk9PntcbiAgICAgIHZhciBzZXJ2ZXJJZCA9IGl0ZW0uZ2V0KCdpZCcpO1xuICAgICAgdmFyIHNlc3Npb25zID0gcmVhY3Rvci5ldmFsdWF0ZShzZXNzaW9uc0J5U2VydmVyKHNlcnZlcklkKSk7XG4gICAgICByZXR1cm4ge1xuICAgICAgICBpZDogc2VydmVySWQsXG4gICAgICAgIGhvc3RuYW1lOiBpdGVtLmdldCgnaG9zdG5hbWUnKSxcbiAgICAgICAgdGFnczogZ2V0VGFncyhpdGVtKSxcbiAgICAgICAgYWRkcjogaXRlbS5nZXQoJ2FkZHInKSxcbiAgICAgICAgc2Vzc2lvbkNvdW50OiBzZXNzaW9ucy5zaXplXG4gICAgICB9XG4gICAgfSkudG9KUygpO1xuIH1cbl07XG5cbmZ1bmN0aW9uIGdldFRhZ3Mobm9kZSl7XG4gIHZhciBhbGxMYWJlbHMgPSBbXTtcbiAgdmFyIGxhYmVscyA9IG5vZGUuZ2V0KCdsYWJlbHMnKTtcblxuICBpZihsYWJlbHMpe1xuICAgIGxhYmVscy5lbnRyeVNlcSgpLnRvQXJyYXkoKS5mb3JFYWNoKGl0ZW09PntcbiAgICAgIGFsbExhYmVscy5wdXNoKHtcbiAgICAgICAgcm9sZTogaXRlbVswXSxcbiAgICAgICAgdmFsdWU6IGl0ZW1bMV1cbiAgICAgIH0pO1xuICAgIH0pO1xuICB9XG5cbiAgbGFiZWxzID0gbm9kZS5nZXQoJ2NtZF9sYWJlbHMnKTtcblxuICBpZihsYWJlbHMpe1xuICAgIGxhYmVscy5lbnRyeVNlcSgpLnRvQXJyYXkoKS5mb3JFYWNoKGl0ZW09PntcbiAgICAgIGFsbExhYmVscy5wdXNoKHtcbiAgICAgICAgcm9sZTogaXRlbVswXSxcbiAgICAgICAgdmFsdWU6IGl0ZW1bMV0uZ2V0KCdyZXN1bHQnKSxcbiAgICAgICAgdG9vbHRpcDogaXRlbVsxXS5nZXQoJ2NvbW1hbmQnKVxuICAgICAgfSk7XG4gICAgfSk7XG4gIH1cblxuICByZXR1cm4gYWxsTGFiZWxzO1xufVxuXG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgbm9kZUxpc3RWaWV3XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9nZXR0ZXJzLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMuZ2V0dGVycyA9IHJlcXVpcmUoJy4vZ2V0dGVycycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aW9ucyA9IHJlcXVpcmUoJy4vYWN0aW9ucycpO1xubW9kdWxlLmV4cG9ydHMubm9kZVN0b3JlID0gcmVxdWlyZSgnLi9ub2RlU3RvcmUnKTtcblxuLy8gbm9kZXM6IFt7XCJpZFwiOlwieDIyMFwiLFwiYWRkclwiOlwiMC4wLjAuMDozMDIyXCIsXCJob3N0bmFtZVwiOlwieDIyMFwiLFwibGFiZWxzXCI6bnVsbCxcImNtZF9sYWJlbHNcIjpudWxsfV1cblxuXG4vLyBzZXNzaW9uczogW3tcImlkXCI6XCIwNzYzMDYzNi1iYjNkLTQwZTEtYjA4Ni02MGIyY2FlMjFhYzRcIixcInBhcnRpZXNcIjpbe1wiaWRcIjpcIjg5Zjc2MmEzLTc0MjktNGM3YS1hOTEzLTc2NjQ5M2ZlN2M4YVwiLFwic2l0ZVwiOlwiMTI3LjAuMC4xOjM3NTE0XCIsXCJ1c2VyXCI6XCJha29udHNldm95XCIsXCJzZXJ2ZXJfYWRkclwiOlwiMC4wLjAuMDozMDIyXCIsXCJsYXN0X2FjdGl2ZVwiOlwiMjAxNi0wMi0yMlQxNDozOToyMC45MzEyMDUzNS0wNTowMFwifV19XVxuXG4vKlxubGV0IFRvZG9SZWNvcmQgPSBJbW11dGFibGUuUmVjb3JkKHtcbiAgICBpZDogMCxcbiAgICBkZXNjcmlwdGlvbjogXCJcIixcbiAgICBjb21wbGV0ZWQ6IGZhbHNlXG59KTtcbiovXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9pbmRleC5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcblxudmFyIHtcbiAgVExQVF9SRVNUX0FQSV9TVEFSVCxcbiAgVExQVF9SRVNUX0FQSV9TVUNDRVNTLFxuICBUTFBUX1JFU1RfQVBJX0ZBSUwgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuXG4gIHN0YXJ0KHJlcVR5cGUpe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRVNUX0FQSV9TVEFSVCwge3R5cGU6IHJlcVR5cGV9KTtcbiAgfSxcblxuICBmYWlsKHJlcVR5cGUsIG1lc3NhZ2Upe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRVNUX0FQSV9GQUlMLCAge3R5cGU6IHJlcVR5cGUsIG1lc3NhZ2V9KTtcbiAgfSxcblxuICBzdWNjZXNzKHJlcVR5cGUpe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRVNUX0FQSV9TVUNDRVNTLCB7dHlwZTogcmVxVHlwZX0pO1xuICB9XG5cbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvYWN0aW9ucy5qc1xuICoqLyIsInZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyIHtcbiAgVExQVF9SRVNUX0FQSV9TVEFSVCxcbiAgVExQVF9SRVNUX0FQSV9TVUNDRVNTLFxuICBUTFBUX1JFU1RfQVBJX0ZBSUwgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKHt9KTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9SRVNUX0FQSV9TVEFSVCwgc3RhcnQpO1xuICAgIHRoaXMub24oVExQVF9SRVNUX0FQSV9GQUlMLCBmYWlsKTtcbiAgICB0aGlzLm9uKFRMUFRfUkVTVF9BUElfU1VDQ0VTUywgc3VjY2Vzcyk7XG4gIH1cbn0pXG5cbmZ1bmN0aW9uIHN0YXJ0KHN0YXRlLCByZXF1ZXN0KXtcbiAgcmV0dXJuIHN0YXRlLnNldChyZXF1ZXN0LnR5cGUsIHRvSW1tdXRhYmxlKHtpc1Byb2Nlc3Npbmc6IHRydWV9KSk7XG59XG5cbmZ1bmN0aW9uIGZhaWwoc3RhdGUsIHJlcXVlc3Qpe1xuICByZXR1cm4gc3RhdGUuc2V0KHJlcXVlc3QudHlwZSwgdG9JbW11dGFibGUoe2lzRmFpbGVkOiB0cnVlLCBtZXNzYWdlOiByZXF1ZXN0Lm1lc3NhZ2V9KSk7XG59XG5cbmZ1bmN0aW9uIHN1Y2Nlc3Moc3RhdGUsIHJlcXVlc3Qpe1xuICByZXR1cm4gc3RhdGUuc2V0KHJlcXVlc3QudHlwZSwgdG9JbW11dGFibGUoe2lzU3VjY2VzczogdHJ1ZX0pKTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvcmVzdEFwaVN0b3JlLmpzXG4gKiovIiwidmFyIHV0aWxzID0ge1xuXG4gIHV1aWQoKXtcbiAgICAvLyBuZXZlciB1c2UgaXQgaW4gcHJvZHVjdGlvblxuICAgIHJldHVybiAneHh4eHh4eHgteHh4eC00eHh4LXl4eHgteHh4eHh4eHh4eHh4Jy5yZXBsYWNlKC9beHldL2csIGZ1bmN0aW9uKGMpIHtcbiAgICAgIHZhciByID0gTWF0aC5yYW5kb20oKSoxNnwwLCB2ID0gYyA9PSAneCcgPyByIDogKHImMHgzfDB4OCk7XG4gICAgICByZXR1cm4gdi50b1N0cmluZygxNik7XG4gICAgfSk7XG4gIH0sXG5cbiAgZGlzcGxheURhdGUoZGF0ZSl7XG4gICAgdHJ5e1xuICAgICAgcmV0dXJuIGRhdGUudG9Mb2NhbGVEYXRlU3RyaW5nKCkgKyAnICcgKyBkYXRlLnRvTG9jYWxlVGltZVN0cmluZygpO1xuICAgIH1jYXRjaChlcnIpe1xuICAgICAgY29uc29sZS5lcnJvcihlcnIpO1xuICAgICAgcmV0dXJuICd1bmRlZmluZWQnO1xuICAgIH1cbiAgfSxcblxuICBmb3JtYXRTdHJpbmcoZm9ybWF0KSB7XG4gICAgdmFyIGFyZ3MgPSBBcnJheS5wcm90b3R5cGUuc2xpY2UuY2FsbChhcmd1bWVudHMsIDEpO1xuICAgIHJldHVybiBmb3JtYXQucmVwbGFjZShuZXcgUmVnRXhwKCdcXFxceyhcXFxcZCspXFxcXH0nLCAnZycpLFxuICAgICAgKG1hdGNoLCBudW1iZXIpID0+IHtcbiAgICAgICAgcmV0dXJuICEoYXJnc1tudW1iZXJdID09PSBudWxsIHx8IGFyZ3NbbnVtYmVyXSA9PT0gdW5kZWZpbmVkKSA/IGFyZ3NbbnVtYmVyXSA6ICcnO1xuICAgIH0pO1xuICB9XG4gICAgICAgICAgICBcbn1cblxubW9kdWxlLmV4cG9ydHMgPSB1dGlscztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC91dGlscy5qc1xuICoqLyIsIi8vIENvcHlyaWdodCBKb3llbnQsIEluYy4gYW5kIG90aGVyIE5vZGUgY29udHJpYnV0b3JzLlxuLy9cbi8vIFBlcm1pc3Npb24gaXMgaGVyZWJ5IGdyYW50ZWQsIGZyZWUgb2YgY2hhcmdlLCB0byBhbnkgcGVyc29uIG9idGFpbmluZyBhXG4vLyBjb3B5IG9mIHRoaXMgc29mdHdhcmUgYW5kIGFzc29jaWF0ZWQgZG9jdW1lbnRhdGlvbiBmaWxlcyAodGhlXG4vLyBcIlNvZnR3YXJlXCIpLCB0byBkZWFsIGluIHRoZSBTb2Z0d2FyZSB3aXRob3V0IHJlc3RyaWN0aW9uLCBpbmNsdWRpbmdcbi8vIHdpdGhvdXQgbGltaXRhdGlvbiB0aGUgcmlnaHRzIHRvIHVzZSwgY29weSwgbW9kaWZ5LCBtZXJnZSwgcHVibGlzaCxcbi8vIGRpc3RyaWJ1dGUsIHN1YmxpY2Vuc2UsIGFuZC9vciBzZWxsIGNvcGllcyBvZiB0aGUgU29mdHdhcmUsIGFuZCB0byBwZXJtaXRcbi8vIHBlcnNvbnMgdG8gd2hvbSB0aGUgU29mdHdhcmUgaXMgZnVybmlzaGVkIHRvIGRvIHNvLCBzdWJqZWN0IHRvIHRoZVxuLy8gZm9sbG93aW5nIGNvbmRpdGlvbnM6XG4vL1xuLy8gVGhlIGFib3ZlIGNvcHlyaWdodCBub3RpY2UgYW5kIHRoaXMgcGVybWlzc2lvbiBub3RpY2Ugc2hhbGwgYmUgaW5jbHVkZWRcbi8vIGluIGFsbCBjb3BpZXMgb3Igc3Vic3RhbnRpYWwgcG9ydGlvbnMgb2YgdGhlIFNvZnR3YXJlLlxuLy9cbi8vIFRIRSBTT0ZUV0FSRSBJUyBQUk9WSURFRCBcIkFTIElTXCIsIFdJVEhPVVQgV0FSUkFOVFkgT0YgQU5ZIEtJTkQsIEVYUFJFU1Ncbi8vIE9SIElNUExJRUQsIElOQ0xVRElORyBCVVQgTk9UIExJTUlURUQgVE8gVEhFIFdBUlJBTlRJRVMgT0Zcbi8vIE1FUkNIQU5UQUJJTElUWSwgRklUTkVTUyBGT1IgQSBQQVJUSUNVTEFSIFBVUlBPU0UgQU5EIE5PTklORlJJTkdFTUVOVC4gSU5cbi8vIE5PIEVWRU5UIFNIQUxMIFRIRSBBVVRIT1JTIE9SIENPUFlSSUdIVCBIT0xERVJTIEJFIExJQUJMRSBGT1IgQU5ZIENMQUlNLFxuLy8gREFNQUdFUyBPUiBPVEhFUiBMSUFCSUxJVFksIFdIRVRIRVIgSU4gQU4gQUNUSU9OIE9GIENPTlRSQUNULCBUT1JUIE9SXG4vLyBPVEhFUldJU0UsIEFSSVNJTkcgRlJPTSwgT1VUIE9GIE9SIElOIENPTk5FQ1RJT04gV0lUSCBUSEUgU09GVFdBUkUgT1IgVEhFXG4vLyBVU0UgT1IgT1RIRVIgREVBTElOR1MgSU4gVEhFIFNPRlRXQVJFLlxuXG5mdW5jdGlvbiBFdmVudEVtaXR0ZXIoKSB7XG4gIHRoaXMuX2V2ZW50cyA9IHRoaXMuX2V2ZW50cyB8fCB7fTtcbiAgdGhpcy5fbWF4TGlzdGVuZXJzID0gdGhpcy5fbWF4TGlzdGVuZXJzIHx8IHVuZGVmaW5lZDtcbn1cbm1vZHVsZS5leHBvcnRzID0gRXZlbnRFbWl0dGVyO1xuXG4vLyBCYWNrd2FyZHMtY29tcGF0IHdpdGggbm9kZSAwLjEwLnhcbkV2ZW50RW1pdHRlci5FdmVudEVtaXR0ZXIgPSBFdmVudEVtaXR0ZXI7XG5cbkV2ZW50RW1pdHRlci5wcm90b3R5cGUuX2V2ZW50cyA9IHVuZGVmaW5lZDtcbkV2ZW50RW1pdHRlci5wcm90b3R5cGUuX21heExpc3RlbmVycyA9IHVuZGVmaW5lZDtcblxuLy8gQnkgZGVmYXVsdCBFdmVudEVtaXR0ZXJzIHdpbGwgcHJpbnQgYSB3YXJuaW5nIGlmIG1vcmUgdGhhbiAxMCBsaXN0ZW5lcnMgYXJlXG4vLyBhZGRlZCB0byBpdC4gVGhpcyBpcyBhIHVzZWZ1bCBkZWZhdWx0IHdoaWNoIGhlbHBzIGZpbmRpbmcgbWVtb3J5IGxlYWtzLlxuRXZlbnRFbWl0dGVyLmRlZmF1bHRNYXhMaXN0ZW5lcnMgPSAxMDtcblxuLy8gT2J2aW91c2x5IG5vdCBhbGwgRW1pdHRlcnMgc2hvdWxkIGJlIGxpbWl0ZWQgdG8gMTAuIFRoaXMgZnVuY3Rpb24gYWxsb3dzXG4vLyB0aGF0IHRvIGJlIGluY3JlYXNlZC4gU2V0IHRvIHplcm8gZm9yIHVubGltaXRlZC5cbkV2ZW50RW1pdHRlci5wcm90b3R5cGUuc2V0TWF4TGlzdGVuZXJzID0gZnVuY3Rpb24obikge1xuICBpZiAoIWlzTnVtYmVyKG4pIHx8IG4gPCAwIHx8IGlzTmFOKG4pKVxuICAgIHRocm93IFR5cGVFcnJvcignbiBtdXN0IGJlIGEgcG9zaXRpdmUgbnVtYmVyJyk7XG4gIHRoaXMuX21heExpc3RlbmVycyA9IG47XG4gIHJldHVybiB0aGlzO1xufTtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5lbWl0ID0gZnVuY3Rpb24odHlwZSkge1xuICB2YXIgZXIsIGhhbmRsZXIsIGxlbiwgYXJncywgaSwgbGlzdGVuZXJzO1xuXG4gIGlmICghdGhpcy5fZXZlbnRzKVxuICAgIHRoaXMuX2V2ZW50cyA9IHt9O1xuXG4gIC8vIElmIHRoZXJlIGlzIG5vICdlcnJvcicgZXZlbnQgbGlzdGVuZXIgdGhlbiB0aHJvdy5cbiAgaWYgKHR5cGUgPT09ICdlcnJvcicpIHtcbiAgICBpZiAoIXRoaXMuX2V2ZW50cy5lcnJvciB8fFxuICAgICAgICAoaXNPYmplY3QodGhpcy5fZXZlbnRzLmVycm9yKSAmJiAhdGhpcy5fZXZlbnRzLmVycm9yLmxlbmd0aCkpIHtcbiAgICAgIGVyID0gYXJndW1lbnRzWzFdO1xuICAgICAgaWYgKGVyIGluc3RhbmNlb2YgRXJyb3IpIHtcbiAgICAgICAgdGhyb3cgZXI7IC8vIFVuaGFuZGxlZCAnZXJyb3InIGV2ZW50XG4gICAgICB9XG4gICAgICB0aHJvdyBUeXBlRXJyb3IoJ1VuY2F1Z2h0LCB1bnNwZWNpZmllZCBcImVycm9yXCIgZXZlbnQuJyk7XG4gICAgfVxuICB9XG5cbiAgaGFuZGxlciA9IHRoaXMuX2V2ZW50c1t0eXBlXTtcblxuICBpZiAoaXNVbmRlZmluZWQoaGFuZGxlcikpXG4gICAgcmV0dXJuIGZhbHNlO1xuXG4gIGlmIChpc0Z1bmN0aW9uKGhhbmRsZXIpKSB7XG4gICAgc3dpdGNoIChhcmd1bWVudHMubGVuZ3RoKSB7XG4gICAgICAvLyBmYXN0IGNhc2VzXG4gICAgICBjYXNlIDE6XG4gICAgICAgIGhhbmRsZXIuY2FsbCh0aGlzKTtcbiAgICAgICAgYnJlYWs7XG4gICAgICBjYXNlIDI6XG4gICAgICAgIGhhbmRsZXIuY2FsbCh0aGlzLCBhcmd1bWVudHNbMV0pO1xuICAgICAgICBicmVhaztcbiAgICAgIGNhc2UgMzpcbiAgICAgICAgaGFuZGxlci5jYWxsKHRoaXMsIGFyZ3VtZW50c1sxXSwgYXJndW1lbnRzWzJdKTtcbiAgICAgICAgYnJlYWs7XG4gICAgICAvLyBzbG93ZXJcbiAgICAgIGRlZmF1bHQ6XG4gICAgICAgIGxlbiA9IGFyZ3VtZW50cy5sZW5ndGg7XG4gICAgICAgIGFyZ3MgPSBuZXcgQXJyYXkobGVuIC0gMSk7XG4gICAgICAgIGZvciAoaSA9IDE7IGkgPCBsZW47IGkrKylcbiAgICAgICAgICBhcmdzW2kgLSAxXSA9IGFyZ3VtZW50c1tpXTtcbiAgICAgICAgaGFuZGxlci5hcHBseSh0aGlzLCBhcmdzKTtcbiAgICB9XG4gIH0gZWxzZSBpZiAoaXNPYmplY3QoaGFuZGxlcikpIHtcbiAgICBsZW4gPSBhcmd1bWVudHMubGVuZ3RoO1xuICAgIGFyZ3MgPSBuZXcgQXJyYXkobGVuIC0gMSk7XG4gICAgZm9yIChpID0gMTsgaSA8IGxlbjsgaSsrKVxuICAgICAgYXJnc1tpIC0gMV0gPSBhcmd1bWVudHNbaV07XG5cbiAgICBsaXN0ZW5lcnMgPSBoYW5kbGVyLnNsaWNlKCk7XG4gICAgbGVuID0gbGlzdGVuZXJzLmxlbmd0aDtcbiAgICBmb3IgKGkgPSAwOyBpIDwgbGVuOyBpKyspXG4gICAgICBsaXN0ZW5lcnNbaV0uYXBwbHkodGhpcywgYXJncyk7XG4gIH1cblxuICByZXR1cm4gdHJ1ZTtcbn07XG5cbkV2ZW50RW1pdHRlci5wcm90b3R5cGUuYWRkTGlzdGVuZXIgPSBmdW5jdGlvbih0eXBlLCBsaXN0ZW5lcikge1xuICB2YXIgbTtcblxuICBpZiAoIWlzRnVuY3Rpb24obGlzdGVuZXIpKVxuICAgIHRocm93IFR5cGVFcnJvcignbGlzdGVuZXIgbXVzdCBiZSBhIGZ1bmN0aW9uJyk7XG5cbiAgaWYgKCF0aGlzLl9ldmVudHMpXG4gICAgdGhpcy5fZXZlbnRzID0ge307XG5cbiAgLy8gVG8gYXZvaWQgcmVjdXJzaW9uIGluIHRoZSBjYXNlIHRoYXQgdHlwZSA9PT0gXCJuZXdMaXN0ZW5lclwiISBCZWZvcmVcbiAgLy8gYWRkaW5nIGl0IHRvIHRoZSBsaXN0ZW5lcnMsIGZpcnN0IGVtaXQgXCJuZXdMaXN0ZW5lclwiLlxuICBpZiAodGhpcy5fZXZlbnRzLm5ld0xpc3RlbmVyKVxuICAgIHRoaXMuZW1pdCgnbmV3TGlzdGVuZXInLCB0eXBlLFxuICAgICAgICAgICAgICBpc0Z1bmN0aW9uKGxpc3RlbmVyLmxpc3RlbmVyKSA/XG4gICAgICAgICAgICAgIGxpc3RlbmVyLmxpc3RlbmVyIDogbGlzdGVuZXIpO1xuXG4gIGlmICghdGhpcy5fZXZlbnRzW3R5cGVdKVxuICAgIC8vIE9wdGltaXplIHRoZSBjYXNlIG9mIG9uZSBsaXN0ZW5lci4gRG9uJ3QgbmVlZCB0aGUgZXh0cmEgYXJyYXkgb2JqZWN0LlxuICAgIHRoaXMuX2V2ZW50c1t0eXBlXSA9IGxpc3RlbmVyO1xuICBlbHNlIGlmIChpc09iamVjdCh0aGlzLl9ldmVudHNbdHlwZV0pKVxuICAgIC8vIElmIHdlJ3ZlIGFscmVhZHkgZ290IGFuIGFycmF5LCBqdXN0IGFwcGVuZC5cbiAgICB0aGlzLl9ldmVudHNbdHlwZV0ucHVzaChsaXN0ZW5lcik7XG4gIGVsc2VcbiAgICAvLyBBZGRpbmcgdGhlIHNlY29uZCBlbGVtZW50LCBuZWVkIHRvIGNoYW5nZSB0byBhcnJheS5cbiAgICB0aGlzLl9ldmVudHNbdHlwZV0gPSBbdGhpcy5fZXZlbnRzW3R5cGVdLCBsaXN0ZW5lcl07XG5cbiAgLy8gQ2hlY2sgZm9yIGxpc3RlbmVyIGxlYWtcbiAgaWYgKGlzT2JqZWN0KHRoaXMuX2V2ZW50c1t0eXBlXSkgJiYgIXRoaXMuX2V2ZW50c1t0eXBlXS53YXJuZWQpIHtcbiAgICB2YXIgbTtcbiAgICBpZiAoIWlzVW5kZWZpbmVkKHRoaXMuX21heExpc3RlbmVycykpIHtcbiAgICAgIG0gPSB0aGlzLl9tYXhMaXN0ZW5lcnM7XG4gICAgfSBlbHNlIHtcbiAgICAgIG0gPSBFdmVudEVtaXR0ZXIuZGVmYXVsdE1heExpc3RlbmVycztcbiAgICB9XG5cbiAgICBpZiAobSAmJiBtID4gMCAmJiB0aGlzLl9ldmVudHNbdHlwZV0ubGVuZ3RoID4gbSkge1xuICAgICAgdGhpcy5fZXZlbnRzW3R5cGVdLndhcm5lZCA9IHRydWU7XG4gICAgICBjb25zb2xlLmVycm9yKCcobm9kZSkgd2FybmluZzogcG9zc2libGUgRXZlbnRFbWl0dGVyIG1lbW9yeSAnICtcbiAgICAgICAgICAgICAgICAgICAgJ2xlYWsgZGV0ZWN0ZWQuICVkIGxpc3RlbmVycyBhZGRlZC4gJyArXG4gICAgICAgICAgICAgICAgICAgICdVc2UgZW1pdHRlci5zZXRNYXhMaXN0ZW5lcnMoKSB0byBpbmNyZWFzZSBsaW1pdC4nLFxuICAgICAgICAgICAgICAgICAgICB0aGlzLl9ldmVudHNbdHlwZV0ubGVuZ3RoKTtcbiAgICAgIGlmICh0eXBlb2YgY29uc29sZS50cmFjZSA9PT0gJ2Z1bmN0aW9uJykge1xuICAgICAgICAvLyBub3Qgc3VwcG9ydGVkIGluIElFIDEwXG4gICAgICAgIGNvbnNvbGUudHJhY2UoKTtcbiAgICAgIH1cbiAgICB9XG4gIH1cblxuICByZXR1cm4gdGhpcztcbn07XG5cbkV2ZW50RW1pdHRlci5wcm90b3R5cGUub24gPSBFdmVudEVtaXR0ZXIucHJvdG90eXBlLmFkZExpc3RlbmVyO1xuXG5FdmVudEVtaXR0ZXIucHJvdG90eXBlLm9uY2UgPSBmdW5jdGlvbih0eXBlLCBsaXN0ZW5lcikge1xuICBpZiAoIWlzRnVuY3Rpb24obGlzdGVuZXIpKVxuICAgIHRocm93IFR5cGVFcnJvcignbGlzdGVuZXIgbXVzdCBiZSBhIGZ1bmN0aW9uJyk7XG5cbiAgdmFyIGZpcmVkID0gZmFsc2U7XG5cbiAgZnVuY3Rpb24gZygpIHtcbiAgICB0aGlzLnJlbW92ZUxpc3RlbmVyKHR5cGUsIGcpO1xuXG4gICAgaWYgKCFmaXJlZCkge1xuICAgICAgZmlyZWQgPSB0cnVlO1xuICAgICAgbGlzdGVuZXIuYXBwbHkodGhpcywgYXJndW1lbnRzKTtcbiAgICB9XG4gIH1cblxuICBnLmxpc3RlbmVyID0gbGlzdGVuZXI7XG4gIHRoaXMub24odHlwZSwgZyk7XG5cbiAgcmV0dXJuIHRoaXM7XG59O1xuXG4vLyBlbWl0cyBhICdyZW1vdmVMaXN0ZW5lcicgZXZlbnQgaWZmIHRoZSBsaXN0ZW5lciB3YXMgcmVtb3ZlZFxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5yZW1vdmVMaXN0ZW5lciA9IGZ1bmN0aW9uKHR5cGUsIGxpc3RlbmVyKSB7XG4gIHZhciBsaXN0LCBwb3NpdGlvbiwgbGVuZ3RoLCBpO1xuXG4gIGlmICghaXNGdW5jdGlvbihsaXN0ZW5lcikpXG4gICAgdGhyb3cgVHlwZUVycm9yKCdsaXN0ZW5lciBtdXN0IGJlIGEgZnVuY3Rpb24nKTtcblxuICBpZiAoIXRoaXMuX2V2ZW50cyB8fCAhdGhpcy5fZXZlbnRzW3R5cGVdKVxuICAgIHJldHVybiB0aGlzO1xuXG4gIGxpc3QgPSB0aGlzLl9ldmVudHNbdHlwZV07XG4gIGxlbmd0aCA9IGxpc3QubGVuZ3RoO1xuICBwb3NpdGlvbiA9IC0xO1xuXG4gIGlmIChsaXN0ID09PSBsaXN0ZW5lciB8fFxuICAgICAgKGlzRnVuY3Rpb24obGlzdC5saXN0ZW5lcikgJiYgbGlzdC5saXN0ZW5lciA9PT0gbGlzdGVuZXIpKSB7XG4gICAgZGVsZXRlIHRoaXMuX2V2ZW50c1t0eXBlXTtcbiAgICBpZiAodGhpcy5fZXZlbnRzLnJlbW92ZUxpc3RlbmVyKVxuICAgICAgdGhpcy5lbWl0KCdyZW1vdmVMaXN0ZW5lcicsIHR5cGUsIGxpc3RlbmVyKTtcblxuICB9IGVsc2UgaWYgKGlzT2JqZWN0KGxpc3QpKSB7XG4gICAgZm9yIChpID0gbGVuZ3RoOyBpLS0gPiAwOykge1xuICAgICAgaWYgKGxpc3RbaV0gPT09IGxpc3RlbmVyIHx8XG4gICAgICAgICAgKGxpc3RbaV0ubGlzdGVuZXIgJiYgbGlzdFtpXS5saXN0ZW5lciA9PT0gbGlzdGVuZXIpKSB7XG4gICAgICAgIHBvc2l0aW9uID0gaTtcbiAgICAgICAgYnJlYWs7XG4gICAgICB9XG4gICAgfVxuXG4gICAgaWYgKHBvc2l0aW9uIDwgMClcbiAgICAgIHJldHVybiB0aGlzO1xuXG4gICAgaWYgKGxpc3QubGVuZ3RoID09PSAxKSB7XG4gICAgICBsaXN0Lmxlbmd0aCA9IDA7XG4gICAgICBkZWxldGUgdGhpcy5fZXZlbnRzW3R5cGVdO1xuICAgIH0gZWxzZSB7XG4gICAgICBsaXN0LnNwbGljZShwb3NpdGlvbiwgMSk7XG4gICAgfVxuXG4gICAgaWYgKHRoaXMuX2V2ZW50cy5yZW1vdmVMaXN0ZW5lcilcbiAgICAgIHRoaXMuZW1pdCgncmVtb3ZlTGlzdGVuZXInLCB0eXBlLCBsaXN0ZW5lcik7XG4gIH1cblxuICByZXR1cm4gdGhpcztcbn07XG5cbkV2ZW50RW1pdHRlci5wcm90b3R5cGUucmVtb3ZlQWxsTGlzdGVuZXJzID0gZnVuY3Rpb24odHlwZSkge1xuICB2YXIga2V5LCBsaXN0ZW5lcnM7XG5cbiAgaWYgKCF0aGlzLl9ldmVudHMpXG4gICAgcmV0dXJuIHRoaXM7XG5cbiAgLy8gbm90IGxpc3RlbmluZyBmb3IgcmVtb3ZlTGlzdGVuZXIsIG5vIG5lZWQgdG8gZW1pdFxuICBpZiAoIXRoaXMuX2V2ZW50cy5yZW1vdmVMaXN0ZW5lcikge1xuICAgIGlmIChhcmd1bWVudHMubGVuZ3RoID09PSAwKVxuICAgICAgdGhpcy5fZXZlbnRzID0ge307XG4gICAgZWxzZSBpZiAodGhpcy5fZXZlbnRzW3R5cGVdKVxuICAgICAgZGVsZXRlIHRoaXMuX2V2ZW50c1t0eXBlXTtcbiAgICByZXR1cm4gdGhpcztcbiAgfVxuXG4gIC8vIGVtaXQgcmVtb3ZlTGlzdGVuZXIgZm9yIGFsbCBsaXN0ZW5lcnMgb24gYWxsIGV2ZW50c1xuICBpZiAoYXJndW1lbnRzLmxlbmd0aCA9PT0gMCkge1xuICAgIGZvciAoa2V5IGluIHRoaXMuX2V2ZW50cykge1xuICAgICAgaWYgKGtleSA9PT0gJ3JlbW92ZUxpc3RlbmVyJykgY29udGludWU7XG4gICAgICB0aGlzLnJlbW92ZUFsbExpc3RlbmVycyhrZXkpO1xuICAgIH1cbiAgICB0aGlzLnJlbW92ZUFsbExpc3RlbmVycygncmVtb3ZlTGlzdGVuZXInKTtcbiAgICB0aGlzLl9ldmVudHMgPSB7fTtcbiAgICByZXR1cm4gdGhpcztcbiAgfVxuXG4gIGxpc3RlbmVycyA9IHRoaXMuX2V2ZW50c1t0eXBlXTtcblxuICBpZiAoaXNGdW5jdGlvbihsaXN0ZW5lcnMpKSB7XG4gICAgdGhpcy5yZW1vdmVMaXN0ZW5lcih0eXBlLCBsaXN0ZW5lcnMpO1xuICB9IGVsc2Uge1xuICAgIC8vIExJRk8gb3JkZXJcbiAgICB3aGlsZSAobGlzdGVuZXJzLmxlbmd0aClcbiAgICAgIHRoaXMucmVtb3ZlTGlzdGVuZXIodHlwZSwgbGlzdGVuZXJzW2xpc3RlbmVycy5sZW5ndGggLSAxXSk7XG4gIH1cbiAgZGVsZXRlIHRoaXMuX2V2ZW50c1t0eXBlXTtcblxuICByZXR1cm4gdGhpcztcbn07XG5cbkV2ZW50RW1pdHRlci5wcm90b3R5cGUubGlzdGVuZXJzID0gZnVuY3Rpb24odHlwZSkge1xuICB2YXIgcmV0O1xuICBpZiAoIXRoaXMuX2V2ZW50cyB8fCAhdGhpcy5fZXZlbnRzW3R5cGVdKVxuICAgIHJldCA9IFtdO1xuICBlbHNlIGlmIChpc0Z1bmN0aW9uKHRoaXMuX2V2ZW50c1t0eXBlXSkpXG4gICAgcmV0ID0gW3RoaXMuX2V2ZW50c1t0eXBlXV07XG4gIGVsc2VcbiAgICByZXQgPSB0aGlzLl9ldmVudHNbdHlwZV0uc2xpY2UoKTtcbiAgcmV0dXJuIHJldDtcbn07XG5cbkV2ZW50RW1pdHRlci5saXN0ZW5lckNvdW50ID0gZnVuY3Rpb24oZW1pdHRlciwgdHlwZSkge1xuICB2YXIgcmV0O1xuICBpZiAoIWVtaXR0ZXIuX2V2ZW50cyB8fCAhZW1pdHRlci5fZXZlbnRzW3R5cGVdKVxuICAgIHJldCA9IDA7XG4gIGVsc2UgaWYgKGlzRnVuY3Rpb24oZW1pdHRlci5fZXZlbnRzW3R5cGVdKSlcbiAgICByZXQgPSAxO1xuICBlbHNlXG4gICAgcmV0ID0gZW1pdHRlci5fZXZlbnRzW3R5cGVdLmxlbmd0aDtcbiAgcmV0dXJuIHJldDtcbn07XG5cbmZ1bmN0aW9uIGlzRnVuY3Rpb24oYXJnKSB7XG4gIHJldHVybiB0eXBlb2YgYXJnID09PSAnZnVuY3Rpb24nO1xufVxuXG5mdW5jdGlvbiBpc051bWJlcihhcmcpIHtcbiAgcmV0dXJuIHR5cGVvZiBhcmcgPT09ICdudW1iZXInO1xufVxuXG5mdW5jdGlvbiBpc09iamVjdChhcmcpIHtcbiAgcmV0dXJuIHR5cGVvZiBhcmcgPT09ICdvYmplY3QnICYmIGFyZyAhPT0gbnVsbDtcbn1cblxuZnVuY3Rpb24gaXNVbmRlZmluZWQoYXJnKSB7XG4gIHJldHVybiBhcmcgPT09IHZvaWQgMDtcbn1cblxuXG5cbi8qKioqKioqKioqKioqKioqKlxuICoqIFdFQlBBQ0sgRk9PVEVSXG4gKiogLi9+L2V2ZW50cy9ldmVudHMuanNcbiAqKiBtb2R1bGUgaWQgPSAxNzNcbiAqKiBtb2R1bGUgY2h1bmtzID0gMVxuICoqLyIsInZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCdhcHAvc2Vzc2lvbicpO1xudmFyIHt1cGRhdGVTZXNzaW9ufSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3Nlc3Npb25zL2FjdGlvbnMnKTtcblxudmFyIEV2ZW50U3RyZWFtZXIgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIGNvbXBvbmVudERpZE1vdW50KCkge1xuICAgIGxldCB7c2lkfSA9IHRoaXMucHJvcHM7XG4gICAgbGV0IHt0b2tlbn0gPSBzZXNzaW9uLmdldFVzZXJEYXRhKCk7XG4gICAgbGV0IGNvbm5TdHIgPSBjZmcuYXBpLmdldEV2ZW50U3RyZWFtQ29ublN0cih0b2tlbiwgc2lkKTtcblxuICAgIHRoaXMuc29ja2V0ID0gbmV3IFdlYlNvY2tldChjb25uU3RyLCAncHJvdG8nKTtcbiAgICB0aGlzLnNvY2tldC5vbm1lc3NhZ2UgPSAoZXZlbnQpID0+IHtcbiAgICAgIHRyeVxuICAgICAge1xuICAgICAgICBsZXQganNvbiA9IEpTT04ucGFyc2UoZXZlbnQuZGF0YSk7XG4gICAgICAgIHVwZGF0ZVNlc3Npb24oanNvbi5zZXNzaW9uKTtcbiAgICAgIH1cbiAgICAgIGNhdGNoKGVycil7XG4gICAgICAgIGNvbnNvbGUubG9nKCdmYWlsZWQgdG8gcGFyc2UgZXZlbnQgc3RyZWFtIGRhdGEnKTtcbiAgICAgIH1cblxuICAgIH07XG4gICAgdGhpcy5zb2NrZXQub25jbG9zZSA9ICgpID0+IHt9O1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50KCkge1xuICAgIHRoaXMuc29ja2V0LmNsb3NlKCk7XG4gIH0sXG5cbiAgc2hvdWxkQ29tcG9uZW50VXBkYXRlKCkge1xuICAgIHJldHVybiBmYWxzZTtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuIG51bGw7XG4gIH1cbn0pO1xuXG5leHBvcnQgZGVmYXVsdCBFdmVudFN0cmVhbWVyO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvYWN0aXZlU2Vzc2lvbi9ldmVudFN0cmVhbWVyLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIge2dldHRlcnMsIGFjdGlvbnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvJyk7XG52YXIgRXZlbnRTdHJlYW1lciA9IHJlcXVpcmUoJy4vZXZlbnRTdHJlYW1lci5qc3gnKTtcbnZhciBUdHkgPSByZXF1aXJlKCdhcHAvY29tbW9uL3R0eScpO1xudmFyIFR0eVRlcm1pbmFsID0gcmVxdWlyZSgnLi8uLi90ZXJtaW5hbC5qc3gnKTtcbnZhciBOb3RGb3VuZFBhZ2UgPSByZXF1aXJlKCdhcHAvY29tcG9uZW50cy9ub3RGb3VuZFBhZ2UuanN4Jyk7XG5cblxudmFyIEFjdGl2ZVNlc3Npb25Ib3N0ID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW3JlYWN0b3IuUmVhY3RNaXhpbl0sXG5cbiAgZ2V0RGF0YUJpbmRpbmdzKCkge1xuICAgIHJldHVybiB7XG4gICAgICBhY3RpdmVTZXNzaW9uOiBnZXR0ZXJzLmFjdGl2ZVNlc3Npb25cbiAgICB9XG4gIH0sXG5cbiAgY29tcG9uZW50RGlkTW91bnQoKXtcbiAgICB2YXIgeyBzaWQgfSA9IHRoaXMucHJvcHMucGFyYW1zO1xuICAgIGlmKCF0aGlzLnN0YXRlLmFjdGl2ZVNlc3Npb24pe1xuICAgICAgYWN0aW9ucy5vcGVuU2Vzc2lvbihzaWQpO1xuICAgIH1cbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIGlmKCF0aGlzLnN0YXRlLmFjdGl2ZVNlc3Npb24pe1xuICAgICAgcmV0dXJuIG51bGw7XG4gICAgfVxuXG4gICAgcmV0dXJuIDxBY3RpdmVTZXNzaW9uIGFjdGl2ZVNlc3Npb249e3RoaXMuc3RhdGUuYWN0aXZlU2Vzc2lvbn0vPjtcbiAgfVxufSk7XG5cblxudmFyIEFjdGl2ZVNlc3Npb24gPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgcmV0dXJuIChcbiAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtdGVybWluYWwtaG9zdFwiPlxuICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXRlcm1pbmFsLXBhcnRpY2lwYW5zXCI+XG4gICAgICAgICA8dWwgY2xhc3NOYW1lPVwibmF2XCI+XG4gICAgICAgICAgIHsvKlxuICAgICAgICAgICA8bGk+PGJ1dHRvbiBjbGFzc05hbWU9XCJidG4gYnRuLXByaW1hcnkgYnRuLWNpcmNsZVwiIHR5cGU9XCJidXR0b25cIj4gPHN0cm9uZz5BPC9zdHJvbmc+PC9idXR0b24+PC9saT5cbiAgICAgICAgICAgPGxpPjxidXR0b24gY2xhc3NOYW1lPVwiYnRuIGJ0bi1wcmltYXJ5IGJ0bi1jaXJjbGVcIiB0eXBlPVwiYnV0dG9uXCI+IEIgPC9idXR0b24+PC9saT5cbiAgICAgICAgICAgPGxpPjxidXR0b24gY2xhc3NOYW1lPVwiYnRuIGJ0bi1wcmltYXJ5IGJ0bi1jaXJjbGVcIiB0eXBlPVwiYnV0dG9uXCI+IEMgPC9idXR0b24+PC9saT5cbiAgICAgICAgICAgKi99XG4gICAgICAgICAgIDxsaT5cbiAgICAgICAgICAgICA8YnV0dG9uIG9uQ2xpY2s9e2FjdGlvbnMuY2xvc2V9IGNsYXNzTmFtZT1cImJ0biBidG4tZGFuZ2VyIGJ0bi1jaXJjbGVcIiB0eXBlPVwiYnV0dG9uXCI+XG4gICAgICAgICAgICAgICA8aSBjbGFzc05hbWU9XCJmYSBmYS10aW1lc1wiPjwvaT5cbiAgICAgICAgICAgICA8L2J1dHRvbj5cbiAgICAgICAgICAgPC9saT5cbiAgICAgICAgIDwvdWw+XG4gICAgICAgPC9kaXY+XG4gICAgICAgPGRpdj5cbiAgICAgICAgIHsvKjxkaXYgY2xhc3NOYW1lPVwiYnRuLWdyb3VwXCI+XG4gICAgICAgICAgIDxzcGFuIGNsYXNzTmFtZT1cImJ0biBidG4teHMgYnRuLXByaW1hcnlcIj4xMjguMC4wLjE6ODg4ODwvc3Bhbj5cbiAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJidG4tZ3JvdXBcIj5cbiAgICAgICAgICAgICA8YnV0dG9uIGRhdGEtdG9nZ2xlPVwiZHJvcGRvd25cIiBjbGFzc05hbWU9XCJidG4gYnRuLWRlZmF1bHQgYnRuLXhzIGRyb3Bkb3duLXRvZ2dsZVwiIGFyaWEtZXhwYW5kZWQ9XCJ0cnVlXCI+XG4gICAgICAgICAgICAgICA8c3BhbiBjbGFzc05hbWU9XCJjYXJldFwiPjwvc3Bhbj5cbiAgICAgICAgICAgICA8L2J1dHRvbj5cbiAgICAgICAgICAgICA8dWwgY2xhc3NOYW1lPVwiZHJvcGRvd24tbWVudVwiPlxuICAgICAgICAgICAgICAgPGxpPjxhIGhyZWY9XCIjXCIgdGFyZ2V0PVwiX2JsYW5rXCI+TG9nczwvYT48L2xpPlxuICAgICAgICAgICAgICAgPGxpPjxhIGhyZWY9XCIjXCIgdGFyZ2V0PVwiX2JsYW5rXCI+TG9nczwvYT48L2xpPlxuICAgICAgICAgICAgIDwvdWw+XG4gICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgPC9kaXY+Ki99XG4gICAgICAgPC9kaXY+XG4gICAgICAgPFR0eUNvbm5lY3Rpb24gey4uLnRoaXMucHJvcHMuYWN0aXZlU2Vzc2lvbn0gLz5cbiAgICAgPC9kaXY+XG4gICAgICk7XG4gIH1cbn0pO1xuXG52YXIgVHR5Q29ubmVjdGlvbiA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgdGhpcy50dHkgPSBuZXcgVHR5KHRoaXMucHJvcHMpXG4gICAgdGhpcy50dHkub24oJ29wZW4nLCAoKT0+IHRoaXMuc2V0U3RhdGUoeyBpc0Nvbm5lY3RlZDogdHJ1ZSB9KSk7XG4gICAgcmV0dXJuIHtpc0Nvbm5lY3RlZDogZmFsc2V9O1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50KCkge1xuICAgIHRoaXMudHR5LmRpc2Nvbm5lY3QoKTtcbiAgfSxcblxuICByZW5kZXIoKSB7XG5cbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBzdHlsZT17e2hlaWdodDogJzEwMCUnfX0+XG4gICAgICAgIDxUdHlUZXJtaW5hbCB0dHk9e3RoaXMudHR5fSBjb2xzPXt0aGlzLnByb3BzLmNvbHN9IHJvd3M9e3RoaXMucHJvcHMucm93c30gLz5cbiAgICAgICAgeyB0aGlzLnN0YXRlLmlzQ29ubmVjdGVkID8gPEV2ZW50U3RyZWFtZXIgc2lkPXt0aGlzLnByb3BzLnNpZH0vPiA6IG51bGwgfVxuICAgICAgPC9kaXY+XG4gICAgKVxuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSB7QWN0aXZlU2Vzc2lvbiwgQWN0aXZlU2Vzc2lvbkhvc3R9O1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvYWN0aXZlU2Vzc2lvbi9tYWluLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgTmF2TGVmdEJhciA9IHJlcXVpcmUoJy4vbmF2TGVmdEJhcicpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7YWN0aW9ucywgZ2V0dGVyc30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9hcHAnKTtcblxudmFyIEFwcCA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBtaXhpbnM6IFtyZWFjdG9yLlJlYWN0TWl4aW5dLFxuXG4gIGdldERhdGFCaW5kaW5ncygpIHtcbiAgICByZXR1cm4ge1xuICAgICAgYXBwOiBnZXR0ZXJzLmFwcFN0YXRlXG4gICAgfVxuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxNb3VudCgpe1xuICAgIGFjdGlvbnMuaW5pdEFwcCgpO1xuICAgIHRoaXMucmVmcmVzaEludGVydmFsID0gc2V0SW50ZXJ2YWwoYWN0aW9ucy5mZXRjaE5vZGVzQW5kU2Vzc2lvbnMsIDMwMDApO1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50OiBmdW5jdGlvbigpIHtcbiAgICBjbGVhckludGVydmFsKHRoaXMucmVmcmVzaEludGVydmFsKTtcbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIGlmKHRoaXMuc3RhdGUuYXBwLmlzSW5pdGlhbGl6aW5nKXtcbiAgICAgIHJldHVybiBudWxsO1xuICAgIH1cblxuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi10bHB0XCI+XG4gICAgICAgIDxOYXZMZWZ0QmFyLz5cbiAgICAgICAge3RoaXMucHJvcHMuYWN0aXZlU2Vzc2lvbkhvc3R9XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwicm93XCI+XG4gICAgICAgICAgPG5hdiBjbGFzc05hbWU9XCJcIiByb2xlPVwibmF2aWdhdGlvblwiIHN0eWxlPXt7IG1hcmdpbkJvdHRvbTogMCwgZmxvYXQ6IFwicmlnaHRcIiB9fT5cbiAgICAgICAgICAgIDx1bCBjbGFzc05hbWU9XCJuYXYgbmF2YmFyLXRvcC1saW5rcyBuYXZiYXItcmlnaHRcIj5cbiAgICAgICAgICAgICAgPGxpPlxuICAgICAgICAgICAgICAgIDxhIGhyZWY9e2NmZy5yb3V0ZXMubG9nb3V0fT5cbiAgICAgICAgICAgICAgICAgIDxpIGNsYXNzTmFtZT1cImZhIGZhLXNpZ24tb3V0XCI+PC9pPlxuICAgICAgICAgICAgICAgICAgTG9nIG91dFxuICAgICAgICAgICAgICAgIDwvYT5cbiAgICAgICAgICAgICAgPC9saT5cbiAgICAgICAgICAgIDwvdWw+XG4gICAgICAgICAgPC9uYXY+XG4gICAgICAgIDwvZGl2PlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1wYWdlXCI+XG4gICAgICAgICAge3RoaXMucHJvcHMuY2hpbGRyZW59XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9kaXY+XG4gICAgKTtcbiAgfVxufSlcblxubW9kdWxlLmV4cG9ydHMgPSBBcHA7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9hcHAuanN4XG4gKiovIiwibW9kdWxlLmV4cG9ydHMuQXBwID0gcmVxdWlyZSgnLi9hcHAuanN4Jyk7XG5tb2R1bGUuZXhwb3J0cy5Mb2dpbiA9IHJlcXVpcmUoJy4vbG9naW4uanN4Jyk7XG5tb2R1bGUuZXhwb3J0cy5OZXdVc2VyID0gcmVxdWlyZSgnLi9uZXdVc2VyLmpzeCcpO1xubW9kdWxlLmV4cG9ydHMuTm9kZXMgPSByZXF1aXJlKCcuL25vZGVzL21haW4uanN4Jyk7XG5tb2R1bGUuZXhwb3J0cy5TZXNzaW9ucyA9IHJlcXVpcmUoJy4vc2Vzc2lvbnMvbWFpbi5qc3gnKTtcbm1vZHVsZS5leHBvcnRzLkFjdGl2ZVNlc3Npb25Ib3N0ID0gcmVxdWlyZSgnLi9hY3RpdmVTZXNzaW9uL21haW4uanN4JykuQWN0aXZlU2Vzc2lvbkhvc3Q7XG5tb2R1bGUuZXhwb3J0cy5Ob3RGb3VuZFBhZ2UgPSByZXF1aXJlKCcuL25vdEZvdW5kUGFnZS5qc3gnKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2luZGV4LmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgJCA9IHJlcXVpcmUoJ2pRdWVyeScpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIExpbmtlZFN0YXRlTWl4aW4gPSByZXF1aXJlKCdyZWFjdC1hZGRvbnMtbGlua2VkLXN0YXRlLW1peGluJyk7XG52YXIge2FjdGlvbnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvdXNlcicpO1xudmFyIEdvb2dsZUF1dGhJbmZvID0gcmVxdWlyZSgnLi9nb29nbGVBdXRoTG9nbycpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxudmFyIExvZ2luSW5wdXRGb3JtID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW0xpbmtlZFN0YXRlTWl4aW5dLFxuXG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4ge1xuICAgICAgdXNlcjogJycsXG4gICAgICBwYXNzd29yZDogJycsXG4gICAgICB0b2tlbjogJydcbiAgICB9XG4gIH0sXG5cbiAgb25DbGljazogZnVuY3Rpb24oZSkge1xuICAgIGUucHJldmVudERlZmF1bHQoKTtcbiAgICBpZiAodGhpcy5pc1ZhbGlkKCkpIHtcbiAgICAgIHRoaXMucHJvcHMub25DbGljayh0aGlzLnN0YXRlKTtcbiAgICB9XG4gIH0sXG5cbiAgaXNWYWxpZDogZnVuY3Rpb24oKSB7XG4gICAgdmFyICRmb3JtID0gJCh0aGlzLnJlZnMuZm9ybSk7XG4gICAgcmV0dXJuICRmb3JtLmxlbmd0aCA9PT0gMCB8fCAkZm9ybS52YWxpZCgpO1xuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICByZXR1cm4gKFxuICAgICAgPGZvcm0gcmVmPVwiZm9ybVwiIGNsYXNzTmFtZT1cImdydi1sb2dpbi1pbnB1dC1mb3JtXCI+XG4gICAgICAgIDxoMz4gV2VsY29tZSB0byBUZWxlcG9ydCA8L2gzPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0IHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3VzZXInKX0gY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sIHJlcXVpcmVkXCIgcGxhY2Vob2xkZXI9XCJVc2VyIG5hbWVcIiBuYW1lPVwidXNlck5hbWVcIiAvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0IHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3Bhc3N3b3JkJyl9IHR5cGU9XCJwYXNzd29yZFwiIG5hbWU9XCJwYXNzd29yZFwiIGNsYXNzTmFtZT1cImZvcm0tY29udHJvbCByZXF1aXJlZFwiIHBsYWNlaG9sZGVyPVwiUGFzc3dvcmRcIi8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXQgdmFsdWVMaW5rPXt0aGlzLmxpbmtTdGF0ZSgndG9rZW4nKX0gY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sIHJlcXVpcmVkXCIgbmFtZT1cInRva2VuXCIgcGxhY2Vob2xkZXI9XCJUd28gZmFjdG9yIHRva2VuIChHb29nbGUgQXV0aGVudGljYXRvcilcIi8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGJ1dHRvbiB0eXBlPVwic3VibWl0XCIgY2xhc3NOYW1lPVwiYnRuIGJ0bi1wcmltYXJ5IGJsb2NrIGZ1bGwtd2lkdGggbS1iXCIgb25DbGljaz17dGhpcy5vbkNsaWNrfT5Mb2dpbjwvYnV0dG9uPlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZm9ybT5cbiAgICApO1xuICB9XG59KVxuXG52YXIgTG9naW4gPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgICB9XG4gIH0sXG5cbiAgb25DbGljayhpbnB1dERhdGEpe1xuICAgIHZhciBsb2MgPSB0aGlzLnByb3BzLmxvY2F0aW9uO1xuICAgIHZhciByZWRpcmVjdCA9IGNmZy5yb3V0ZXMuYXBwO1xuXG4gICAgaWYobG9jLnN0YXRlICYmIGxvYy5zdGF0ZS5yZWRpcmVjdFRvKXtcbiAgICAgIHJlZGlyZWN0ID0gbG9jLnN0YXRlLnJlZGlyZWN0VG87XG4gICAgfVxuXG4gICAgYWN0aW9ucy5sb2dpbihpbnB1dERhdGEsIHJlZGlyZWN0KTtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgdmFyIGlzUHJvY2Vzc2luZyA9IGZhbHNlOy8vdGhpcy5zdGF0ZS51c2VyUmVxdWVzdC5nZXQoJ2lzTG9hZGluZycpO1xuICAgIHZhciBpc0Vycm9yID0gZmFsc2U7Ly90aGlzLnN0YXRlLnVzZXJSZXF1ZXN0LmdldCgnaXNFcnJvcicpO1xuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1sb2dpbiB0ZXh0LWNlbnRlclwiPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1sb2dvLXRwcnRcIj48L2Rpdj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtY29udGVudCBncnYtZmxleFwiPlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWZsZXgtY29sdW1uXCI+XG4gICAgICAgICAgICA8TG9naW5JbnB1dEZvcm0gb25DbGljaz17dGhpcy5vbkNsaWNrfS8+XG4gICAgICAgICAgICA8R29vZ2xlQXV0aEluZm8vPlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbG9naW4taW5mb1wiPlxuICAgICAgICAgICAgICA8aSBjbGFzc05hbWU9XCJmYSBmYS1xdWVzdGlvblwiPjwvaT5cbiAgICAgICAgICAgICAgPHN0cm9uZz5OZXcgQWNjb3VudCBvciBmb3Jnb3QgcGFzc3dvcmQ/PC9zdHJvbmc+XG4gICAgICAgICAgICAgIDxkaXY+QXNrIGZvciBhc3Npc3RhbmNlIGZyb20geW91ciBDb21wYW55IGFkbWluaXN0cmF0b3I8L2Rpdj5cbiAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IExvZ2luO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvbG9naW4uanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciB7IFJvdXRlciwgSW5kZXhMaW5rLCBIaXN0b3J5IH0gPSByZXF1aXJlKCdyZWFjdC1yb3V0ZXInKTtcbnZhciBnZXR0ZXJzID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvdXNlci9nZXR0ZXJzJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xuXG52YXIgbWVudUl0ZW1zID0gW1xuICB7aWNvbjogJ2ZhIGZhLWNvZ3MnLCB0bzogY2ZnLnJvdXRlcy5ub2RlcywgdGl0bGU6ICdOb2Rlcyd9LFxuICB7aWNvbjogJ2ZhIGZhLXNpdGVtYXAnLCB0bzogY2ZnLnJvdXRlcy5zZXNzaW9ucywgdGl0bGU6ICdTZXNzaW9ucyd9LFxuICB7aWNvbjogJ2ZhIGZhLXF1ZXN0aW9uJywgdG86ICcjJywgdGl0bGU6ICdTZXNzaW9ucyd9LFxuXTtcblxudmFyIE5hdkxlZnRCYXIgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpe1xuICAgIHZhciBpdGVtcyA9IG1lbnVJdGVtcy5tYXAoKGksIGluZGV4KT0+e1xuICAgICAgdmFyIGNsYXNzTmFtZSA9IHRoaXMuY29udGV4dC5yb3V0ZXIuaXNBY3RpdmUoaS50bykgPyAnYWN0aXZlJyA6ICcnO1xuICAgICAgcmV0dXJuIChcbiAgICAgICAgPGxpIGtleT17aW5kZXh9IGNsYXNzTmFtZT17Y2xhc3NOYW1lfT5cbiAgICAgICAgICA8SW5kZXhMaW5rIHRvPXtpLnRvfT5cbiAgICAgICAgICAgIDxpIGNsYXNzTmFtZT17aS5pY29ufSB0aXRsZT17aS50aXRsZX0vPlxuICAgICAgICAgIDwvSW5kZXhMaW5rPlxuICAgICAgICA8L2xpPlxuICAgICAgKTtcbiAgICB9KTtcblxuICAgIHJldHVybiAoXG4gICAgICA8bmF2IGNsYXNzTmFtZT0nZ3J2LW5hdiBuYXZiYXItZGVmYXVsdCBuYXZiYXItc3RhdGljLXNpZGUnIHJvbGU9J25hdmlnYXRpb24nPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT0nJz5cbiAgICAgICAgICA8dWwgY2xhc3NOYW1lPSduYXYnIGlkPSdzaWRlLW1lbnUnPlxuICAgICAgICAgICAgPGxpPjxkaXYgY2xhc3NOYW1lPVwiZ3J2LWNpcmNsZSB0ZXh0LXVwcGVyY2FzZVwiPjxzcGFuPntnZXRVc2VyTmFtZUxldHRlcigpfTwvc3Bhbj48L2Rpdj48L2xpPlxuICAgICAgICAgICAge2l0ZW1zfVxuICAgICAgICAgIDwvdWw+XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9uYXY+XG4gICAgKTtcbiAgfVxufSk7XG5cbk5hdkxlZnRCYXIuY29udGV4dFR5cGVzID0ge1xuICByb3V0ZXI6IFJlYWN0LlByb3BUeXBlcy5vYmplY3QuaXNSZXF1aXJlZFxufVxuXG5mdW5jdGlvbiBnZXRVc2VyTmFtZUxldHRlcigpe1xuICB2YXIge3Nob3J0RGlzcGxheU5hbWV9ID0gcmVhY3Rvci5ldmFsdWF0ZShnZXR0ZXJzLnVzZXIpO1xuICByZXR1cm4gc2hvcnREaXNwbGF5TmFtZTtcbn1cblxubW9kdWxlLmV4cG9ydHMgPSBOYXZMZWZ0QmFyO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvbmF2TGVmdEJhci5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyICQgPSByZXF1aXJlKCdqUXVlcnknKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7YWN0aW9ucywgZ2V0dGVyc30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9pbnZpdGUnKTtcbnZhciB1c2VyTW9kdWxlID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvdXNlcicpO1xudmFyIExpbmtlZFN0YXRlTWl4aW4gPSByZXF1aXJlKCdyZWFjdC1hZGRvbnMtbGlua2VkLXN0YXRlLW1peGluJyk7XG52YXIgR29vZ2xlQXV0aEluZm8gPSByZXF1aXJlKCcuL2dvb2dsZUF1dGhMb2dvJyk7XG5cbnZhciBJbnZpdGVJbnB1dEZvcm0gPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbTGlua2VkU3RhdGVNaXhpbl0sXG5cbiAgY29tcG9uZW50RGlkTW91bnQoKXtcbiAgICAkKHRoaXMucmVmcy5mb3JtKS52YWxpZGF0ZSh7XG4gICAgICBydWxlczp7XG4gICAgICAgIHBhc3N3b3JkOntcbiAgICAgICAgICBtaW5sZW5ndGg6IDUsXG4gICAgICAgICAgcmVxdWlyZWQ6IHRydWVcbiAgICAgICAgfSxcbiAgICAgICAgcGFzc3dvcmRDb25maXJtZWQ6e1xuICAgICAgICAgIHJlcXVpcmVkOiB0cnVlLFxuICAgICAgICAgIGVxdWFsVG86IHRoaXMucmVmcy5wYXNzd29yZFxuICAgICAgICB9XG4gICAgICB9LFxuXG4gICAgICBtZXNzYWdlczoge1xuICBcdFx0XHRwYXNzd29yZENvbmZpcm1lZDoge1xuICBcdFx0XHRcdG1pbmxlbmd0aDogJC52YWxpZGF0b3IuZm9ybWF0KCdFbnRlciBhdCBsZWFzdCB7MH0gY2hhcmFjdGVycycpLFxuICBcdFx0XHRcdGVxdWFsVG86ICdFbnRlciB0aGUgc2FtZSBwYXNzd29yZCBhcyBhYm92ZSdcbiAgXHRcdFx0fVxuICAgICAgfVxuICAgIH0pXG4gIH0sXG5cbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB7XG4gICAgICBuYW1lOiB0aGlzLnByb3BzLmludml0ZS51c2VyLFxuICAgICAgcHN3OiAnJyxcbiAgICAgIHBzd0NvbmZpcm1lZDogJycsXG4gICAgICB0b2tlbjogJydcbiAgICB9XG4gIH0sXG5cbiAgb25DbGljayhlKSB7XG4gICAgZS5wcmV2ZW50RGVmYXVsdCgpO1xuICAgIGlmICh0aGlzLmlzVmFsaWQoKSkge1xuICAgICAgdXNlck1vZHVsZS5hY3Rpb25zLnNpZ25VcCh7XG4gICAgICAgIG5hbWU6IHRoaXMuc3RhdGUubmFtZSxcbiAgICAgICAgcHN3OiB0aGlzLnN0YXRlLnBzdyxcbiAgICAgICAgdG9rZW46IHRoaXMuc3RhdGUudG9rZW4sXG4gICAgICAgIGludml0ZVRva2VuOiB0aGlzLnByb3BzLmludml0ZS5pbnZpdGVfdG9rZW59KTtcbiAgICB9XG4gIH0sXG5cbiAgaXNWYWxpZCgpIHtcbiAgICB2YXIgJGZvcm0gPSAkKHRoaXMucmVmcy5mb3JtKTtcbiAgICByZXR1cm4gJGZvcm0ubGVuZ3RoID09PSAwIHx8ICRmb3JtLnZhbGlkKCk7XG4gIH0sXG5cbiAgcmVuZGVyKCkge1xuICAgIHJldHVybiAoXG4gICAgICA8Zm9ybSByZWY9XCJmb3JtXCIgY2xhc3NOYW1lPVwiZ3J2LWludml0ZS1pbnB1dC1mb3JtXCI+XG4gICAgICAgIDxoMz4gR2V0IHN0YXJ0ZWQgd2l0aCBUZWxlcG9ydCA8L2gzPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0XG4gICAgICAgICAgICAgIHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ25hbWUnKX1cbiAgICAgICAgICAgICAgbmFtZT1cInVzZXJOYW1lXCJcbiAgICAgICAgICAgICAgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sIHJlcXVpcmVkXCJcbiAgICAgICAgICAgICAgcGxhY2Vob2xkZXI9XCJVc2VyIG5hbWVcIi8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXRcbiAgICAgICAgICAgICAgdmFsdWVMaW5rPXt0aGlzLmxpbmtTdGF0ZSgncHN3Jyl9XG4gICAgICAgICAgICAgIHJlZj1cInBhc3N3b3JkXCJcbiAgICAgICAgICAgICAgdHlwZT1cInBhc3N3b3JkXCJcbiAgICAgICAgICAgICAgbmFtZT1cInBhc3N3b3JkXCJcbiAgICAgICAgICAgICAgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sXCJcbiAgICAgICAgICAgICAgcGxhY2Vob2xkZXI9XCJQYXNzd29yZFwiIC8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwIGdydi1cIj5cbiAgICAgICAgICAgIDxpbnB1dFxuICAgICAgICAgICAgICB2YWx1ZUxpbms9e3RoaXMubGlua1N0YXRlKCdwc3dDb25maXJtZWQnKX1cbiAgICAgICAgICAgICAgdHlwZT1cInBhc3N3b3JkXCJcbiAgICAgICAgICAgICAgbmFtZT1cInBhc3N3b3JkQ29uZmlybWVkXCJcbiAgICAgICAgICAgICAgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sXCJcbiAgICAgICAgICAgICAgcGxhY2Vob2xkZXI9XCJQYXNzd29yZCBjb25maXJtXCIvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0XG4gICAgICAgICAgICAgIG5hbWU9XCJ0b2tlblwiXG4gICAgICAgICAgICAgIHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3Rva2VuJyl9XG4gICAgICAgICAgICAgIGNsYXNzTmFtZT1cImZvcm0tY29udHJvbCByZXF1aXJlZFwiXG4gICAgICAgICAgICAgIHBsYWNlaG9sZGVyPVwiVHdvIGZhY3RvciB0b2tlbiAoR29vZ2xlIEF1dGhlbnRpY2F0b3IpXCIgLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8YnV0dG9uIHR5cGU9XCJzdWJtaXRcIiBkaXNhYmxlZD17dGhpcy5wcm9wcy5hdHRlbXAuaXNQcm9jZXNzaW5nfSBjbGFzc05hbWU9XCJidG4gYnRuLXByaW1hcnkgYmxvY2sgZnVsbC13aWR0aCBtLWJcIiBvbkNsaWNrPXt0aGlzLm9uQ2xpY2t9ID5TaWduIHVwPC9idXR0b24+XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9mb3JtPlxuICAgICk7XG4gIH1cbn0pXG5cbnZhciBJbnZpdGUgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIGludml0ZTogZ2V0dGVycy5pbnZpdGUsXG4gICAgICBhdHRlbXA6IGdldHRlcnMuYXR0ZW1wXG4gICAgfVxuICB9LFxuXG4gIGNvbXBvbmVudERpZE1vdW50KCl7XG4gICAgYWN0aW9ucy5mZXRjaEludml0ZSh0aGlzLnByb3BzLnBhcmFtcy5pbnZpdGVUb2tlbik7XG4gIH0sXG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICBpZighdGhpcy5zdGF0ZS5pbnZpdGUpIHtcbiAgICAgIHJldHVybiBudWxsO1xuICAgIH1cblxuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1pbnZpdGUgdGV4dC1jZW50ZXJcIj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbG9nby10cHJ0XCI+PC9kaXY+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWNvbnRlbnQgZ3J2LWZsZXhcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LWNvbHVtblwiPlxuICAgICAgICAgICAgPEludml0ZUlucHV0Rm9ybSBhdHRlbXA9e3RoaXMuc3RhdGUuYXR0ZW1wfSBpbnZpdGU9e3RoaXMuc3RhdGUuaW52aXRlLnRvSlMoKX0vPlxuICAgICAgICAgICAgPEdvb2dsZUF1dGhJbmZvLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LWNvbHVtblwiPlxuICAgICAgICAgICAgPGg0PlNjYW4gYmFyIGNvZGUgZm9yIGF1dGggdG9rZW4gPGJyLz4gPHNtYWxsPlNjYW4gYmVsb3cgdG8gZ2VuZXJhdGUgeW91ciB0d28gZmFjdG9yIHRva2VuPC9zbWFsbD48L2g0PlxuICAgICAgICAgICAgPGltZyBjbGFzc05hbWU9XCJpbWctdGh1bWJuYWlsXCIgc3JjPXsgYGRhdGE6aW1hZ2UvcG5nO2Jhc2U2NCwke3RoaXMuc3RhdGUuaW52aXRlLmdldCgncXInKX1gIH0gLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBJbnZpdGU7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9uZXdVc2VyLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIge2dldHRlcnMsIGFjdGlvbnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvbm9kZXMnKTtcbnZhciB1c2VyR2V0dGVycyA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3VzZXIvZ2V0dGVycycpO1xudmFyIHtUYWJsZSwgQ29sdW1uLCBDZWxsfSA9IHJlcXVpcmUoJ2FwcC9jb21wb25lbnRzL3RhYmxlLmpzeCcpO1xudmFyIHtjcmVhdGVOZXdTZXNzaW9ufSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGlvbnMnKTtcblxuY29uc3QgVGV4dENlbGwgPSAoe3Jvd0luZGV4LCBkYXRhLCBjb2x1bW5LZXksIC4uLnByb3BzfSkgPT4gKFxuICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgIHtkYXRhW3Jvd0luZGV4XVtjb2x1bW5LZXldfVxuICA8L0NlbGw+XG4pO1xuXG5jb25zdCBUYWdDZWxsID0gKHtyb3dJbmRleCwgZGF0YSwgY29sdW1uS2V5LCAuLi5wcm9wc30pID0+IChcbiAgPENlbGwgey4uLnByb3BzfT5cbiAgICB7IGRhdGFbcm93SW5kZXhdLnRhZ3MubWFwKChpdGVtLCBpbmRleCkgPT5cbiAgICAgICg8c3BhbiBrZXk9e2luZGV4fSBjbGFzc05hbWU9XCJsYWJlbCBsYWJlbC1kZWZhdWx0XCI+XG4gICAgICAgIHtpdGVtLnJvbGV9IDxsaSBjbGFzc05hbWU9XCJmYSBmYS1sb25nLWFycm93LXJpZ2h0XCI+PC9saT5cbiAgICAgICAge2l0ZW0udmFsdWV9XG4gICAgICA8L3NwYW4+KVxuICAgICkgfVxuICA8L0NlbGw+XG4pO1xuXG5jb25zdCBMb2dpbkNlbGwgPSAoe3VzZXIsIHJvd0luZGV4LCBkYXRhLCAuLi5wcm9wc30pID0+IHtcbiAgaWYoIXVzZXIgfHwgdXNlci5sb2dpbnMubGVuZ3RoID09PSAwKXtcbiAgICByZXR1cm4gPENlbGwgey4uLnByb3BzfSAvPjtcbiAgfVxuXG4gIHZhciBzZXJ2ZXJJZCA9IGRhdGFbcm93SW5kZXhdLmlkO1xuICB2YXIgJGxpcyA9IFtdO1xuXG4gIGZ1bmN0aW9uIG9uTmV3U2Vzc2lvbkNsaWNrKGkpe1xuICAgIHZhciBsb2dpbiA9IHVzZXIubG9naW5zW2ldO1xuICAgIHJldHVybiAoKSA9PiBjcmVhdGVOZXdTZXNzaW9uKHNlcnZlcklkLCBsb2dpbik7XG4gIH1cblxuICBmb3IodmFyIGkgPSAwOyBpIDwgdXNlci5sb2dpbnMubGVuZ3RoOyBpKyspe1xuICAgICRsaXMucHVzaCg8bGkga2V5PXtpfT48YSBvbkNsaWNrPXtvbk5ld1Nlc3Npb25DbGljayhpKX0+e3VzZXIubG9naW5zW2ldfTwvYT48L2xpPik7XG4gIH1cblxuICByZXR1cm4gKFxuICAgIDxDZWxsIHsuLi5wcm9wc30+XG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImJ0bi1ncm91cFwiPlxuICAgICAgICA8YnV0dG9uIHR5cGU9XCJidXR0b25cIiBvbkNsaWNrPXtvbk5ld1Nlc3Npb25DbGljaygwKX0gY2xhc3NOYW1lPVwiYnRuIGJ0bi1zbSBidG4tcHJpbWFyeVwiPnt1c2VyLmxvZ2luc1swXX08L2J1dHRvbj5cbiAgICAgICAge1xuICAgICAgICAgICRsaXMubGVuZ3RoID4gMSA/IChcbiAgICAgICAgICAgICAgW1xuICAgICAgICAgICAgICAgIDxidXR0b24ga2V5PXswfSBkYXRhLXRvZ2dsZT1cImRyb3Bkb3duXCIgY2xhc3NOYW1lPVwiYnRuIGJ0bi1kZWZhdWx0IGJ0bi1zbSBkcm9wZG93bi10b2dnbGVcIiBhcmlhLWV4cGFuZGVkPVwidHJ1ZVwiPlxuICAgICAgICAgICAgICAgICAgPHNwYW4gY2xhc3NOYW1lPVwiY2FyZXRcIj48L3NwYW4+XG4gICAgICAgICAgICAgICAgPC9idXR0b24+LFxuICAgICAgICAgICAgICAgIDx1bCBrZXk9ezF9IGNsYXNzTmFtZT1cImRyb3Bkb3duLW1lbnVcIj5cbiAgICAgICAgICAgICAgICAgIHskbGlzfVxuICAgICAgICAgICAgICAgIDwvdWw+XG4gICAgICAgICAgICAgIF0gKVxuICAgICAgICAgICAgOiBudWxsXG4gICAgICAgIH1cbiAgICAgIDwvZGl2PlxuICAgIDwvQ2VsbD5cbiAgKVxufTtcblxudmFyIE5vZGVzID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW3JlYWN0b3IuUmVhY3RNaXhpbl0sXG5cbiAgZ2V0RGF0YUJpbmRpbmdzKCkge1xuICAgIHJldHVybiB7XG4gICAgICBub2RlUmVjb3JkczogZ2V0dGVycy5ub2RlTGlzdFZpZXcsXG4gICAgICB1c2VyOiB1c2VyR2V0dGVycy51c2VyXG4gICAgfVxuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgdmFyIGRhdGEgPSB0aGlzLnN0YXRlLm5vZGVSZWNvcmRzO1xuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1ub2Rlc1wiPlxuICAgICAgICA8aDE+IE5vZGVzIDwvaDE+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiXCI+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiXCI+XG4gICAgICAgICAgICAgIDxUYWJsZSByb3dDb3VudD17ZGF0YS5sZW5ndGh9IGNsYXNzTmFtZT1cInRhYmxlLXN0cmlwcGVkIGdydi1ub2Rlcy10YWJsZVwiPlxuICAgICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICAgIGNvbHVtbktleT1cInNlc3Npb25Db3VudFwiXG4gICAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPiBTZXNzaW9ucyA8L0NlbGw+IH1cbiAgICAgICAgICAgICAgICAgIGNlbGw9ezxUZXh0Q2VsbCBkYXRhPXtkYXRhfS8+IH1cbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICAgIGNvbHVtbktleT1cImFkZHJcIlxuICAgICAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD4gTm9kZSA8L0NlbGw+IH1cbiAgICAgICAgICAgICAgICAgIGNlbGw9ezxUZXh0Q2VsbCBkYXRhPXtkYXRhfS8+IH1cbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICAgIGNvbHVtbktleT1cInRhZ3NcIlxuICAgICAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD48L0NlbGw+IH1cbiAgICAgICAgICAgICAgICAgIGNlbGw9ezxUYWdDZWxsIGRhdGE9e2RhdGF9Lz4gfVxuICAgICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICAgICAgY29sdW1uS2V5PVwicm9sZXNcIlxuICAgICAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD5Mb2dpbiBhczwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17PExvZ2luQ2VsbCBkYXRhPXtkYXRhfSB1c2VyPXt0aGlzLnN0YXRlLnVzZXJ9Lz4gfVxuICAgICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgIDwvVGFibGU+XG4gICAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApXG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IE5vZGVzO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvbm9kZXMvbWFpbi5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHsgTGluayB9ID0gcmVxdWlyZSgncmVhY3Qtcm91dGVyJyk7XG52YXIge1RhYmxlLCBDb2x1bW4sIENlbGwsIFRleHRDZWxsfSA9IHJlcXVpcmUoJ2FwcC9jb21wb25lbnRzL3RhYmxlLmpzeCcpO1xudmFyIHtnZXR0ZXJzfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3Nlc3Npb25zJyk7XG52YXIge29wZW59ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvYWN0aW9ucycpO1xuXG5jb25zdCBVc2Vyc0NlbGwgPSAoeyByb3dJbmRleCwgZGF0YSwgLi4ucHJvcHMgfSkgPT4ge1xuICB2YXIgJHVzZXJzID0gZGF0YVtyb3dJbmRleF0ucGFydGllcy5tYXAoKGl0ZW0sIGl0ZW1JbmRleCk9PlxuICAgICg8c3BhbiBrZXk9e2l0ZW1JbmRleH0gY2xhc3NOYW1lPVwidGV4dC11cHBlcmNhc2UgbGFiZWwgbGFiZWwtcHJpbWFyeVwiPntpdGVtLnVzZXJbMF19PC9zcGFuPilcbiAgKVxuXG4gIHJldHVybiAoXG4gICAgPENlbGwgey4uLnByb3BzfT5cbiAgICAgIDxkaXY+XG4gICAgICAgIHskdXNlcnN9XG4gICAgICA8L2Rpdj5cbiAgICA8L0NlbGw+XG4gIClcbn07XG5cbmNvbnN0IEJ1dHRvbkNlbGwgPSAoeyByb3dJbmRleCwgZGF0YSwgLi4ucHJvcHMgfSkgPT4ge1xuICB2YXIgc2Vzc2lvblVybCA9IGRhdGFbcm93SW5kZXhdLnNlc3Npb25Vcmw7XG4gIHJldHVybiAoXG4gICAgPENlbGwgey4uLnByb3BzfT5cbiAgICAgIDxMaW5rIHRvPXtzZXNzaW9uVXJsfSBjbGFzc05hbWU9XCJidG4gYnRuLWluZm8gYnRuLWNpcmNsZVwiIHR5cGU9XCJidXR0b25cIj5cbiAgICAgICAgPGkgY2xhc3NOYW1lPVwiZmEgZmEtdGVybWluYWxcIj48L2k+XG4gICAgICA8L0xpbms+XG4gICAgICA8YnV0dG9uIGNsYXNzTmFtZT1cImJ0biBidG4taW5mbyBidG4tY2lyY2xlXCIgdHlwZT1cImJ1dHRvblwiPlxuICAgICAgICA8aSBjbGFzc05hbWU9XCJmYSBmYS1wbGF5LWNpcmNsZVwiPjwvaT5cbiAgICAgIDwvYnV0dG9uPlxuICAgIDwvQ2VsbD5cbiAgKVxufVxuXG52YXIgU2Vzc2lvbkxpc3QgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIHNlc3Npb25zVmlldzogZ2V0dGVycy5zZXNzaW9uc1ZpZXdcbiAgICB9XG4gIH0sXG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICB2YXIgZGF0YSA9IHRoaXMuc3RhdGUuc2Vzc2lvbnNWaWV3O1xuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1zZXNzaW9uc1wiPlxuICAgICAgICA8aDE+IFNlc3Npb25zPC9oMT5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICAgICAgPFRhYmxlIHJvd0NvdW50PXtkYXRhLmxlbmd0aH0gY2xhc3NOYW1lPVwidGFibGUtc3RyaXBwZWRcIj5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJzaWRcIlxuICAgICAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD4gU2Vzc2lvbiBJRCA8L0NlbGw+IH1cbiAgICAgICAgICAgICAgICAgIGNlbGw9ezxUZXh0Q2VsbCBkYXRhPXtkYXRhfS8+IH1cbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+IDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17XG4gICAgICAgICAgICAgICAgICAgIDxCdXR0b25DZWxsIGRhdGE9e2RhdGF9IC8+XG4gICAgICAgICAgICAgICAgICB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJzZXJ2ZXJJcFwiXG4gICAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPiBOb2RlIDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17PFRleHRDZWxsIGRhdGE9e2RhdGF9IC8+IH1cbiAgICAgICAgICAgICAgICAvPlxuXG4gICAgICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICAgICAgY29sdW1uS2V5PVwic2VydmVySWRcIlxuICAgICAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD4gVXNlcnMgPC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXs8VXNlcnNDZWxsIGRhdGE9e2RhdGF9IC8+IH1cbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICA8L1RhYmxlPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9kaXY+XG4gICAgKVxuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBTZXNzaW9uTGlzdDtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL3Nlc3Npb25zL21haW4uanN4XG4gKiovIiwidmFyIFRlcm0gPSByZXF1aXJlKCdUZXJtaW5hbCcpO1xudmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciB7ZGVib3VuY2UsIGlzTnVtYmVyfSA9IHJlcXVpcmUoJ18nKTtcblxuVGVybS5jb2xvcnNbMjU2XSA9ICdpbmhlcml0JztcblxuY29uc3QgRElTQ09OTkVDVF9UWFQgPSAnXFx4MWJbMzFtZGlzY29ubmVjdGVkXFx4MWJbbVxcclxcbic7XG5jb25zdCBDT05ORUNURURfVFhUID0gJ0Nvbm5lY3RlZCFcXHJcXG4nO1xuXG52YXIgVHR5VGVybWluYWwgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgZ2V0SW5pdGlhbFN0YXRlKCl7XG4gICAgdGhpcy5yb3dzID0gdGhpcy5wcm9wcy5yb3dzO1xuICAgIHRoaXMuY29scyA9IHRoaXMucHJvcHMuY29scztcbiAgICB0aGlzLnR0eSA9IHRoaXMucHJvcHMudHR5O1xuXG4gICAgdGhpcy5kZWJvdW5jZWRSZXNpemUgPSBkZWJvdW5jZSgoKT0+e1xuICAgICAgdGhpcy5yZXNpemUoKTtcbiAgICAgIHRoaXMudHR5LnJlc2l6ZSh0aGlzLmNvbHMsIHRoaXMucm93cyk7XG4gICAgfSwgMjAwKTtcblxuICAgIHJldHVybiB7fTtcbiAgfSxcblxuICBjb21wb25lbnREaWRNb3VudDogZnVuY3Rpb24oKSB7XG4gICAgdGhpcy50ZXJtID0gbmV3IFRlcm1pbmFsKHtcbiAgICAgIGNvbHM6IDUsXG4gICAgICByb3dzOiA1LFxuICAgICAgdXNlU3R5bGU6IHRydWUsXG4gICAgICBzY3JlZW5LZXlzOiB0cnVlLFxuICAgICAgY3Vyc29yQmxpbms6IHRydWVcbiAgICB9KTtcblxuICAgIHRoaXMudGVybS5vcGVuKHRoaXMucmVmcy5jb250YWluZXIpO1xuICAgIHRoaXMudGVybS5vbignZGF0YScsIChkYXRhKSA9PiB0aGlzLnR0eS5zZW5kKGRhdGEpKTtcblxuICAgIHRoaXMucmVzaXplKHRoaXMuY29scywgdGhpcy5yb3dzKTtcblxuICAgIHRoaXMudHR5Lm9uKCdvcGVuJywgKCk9PiB0aGlzLnRlcm0ud3JpdGUoQ09OTkVDVEVEX1RYVCkpO1xuICAgIHRoaXMudHR5Lm9uKCdjbG9zZScsICgpPT4gdGhpcy50ZXJtLndyaXRlKERJU0NPTk5FQ1RfVFhUKSk7XG4gICAgdGhpcy50dHkub24oJ2RhdGEnLCAoZGF0YSkgPT4gdGhpcy50ZXJtLndyaXRlKGRhdGEpKTtcblxuICAgIHRoaXMudHR5LmNvbm5lY3Qoe2NvbHM6IHRoaXMuY29scywgcm93czogdGhpcy5yb3dzfSk7XG4gICAgd2luZG93LmFkZEV2ZW50TGlzdGVuZXIoJ3Jlc2l6ZScsIHRoaXMuZGVib3VuY2VkUmVzaXplKTtcbiAgfSxcblxuICBjb21wb25lbnRXaWxsVW5tb3VudDogZnVuY3Rpb24oKSB7XG4gICAgdGhpcy50ZXJtLmRlc3Ryb3koKTtcbiAgICB3aW5kb3cucmVtb3ZlRXZlbnRMaXN0ZW5lcigncmVzaXplJywgdGhpcy5kZWJvdW5jZWRSZXNpemUpO1xuICB9LFxuXG4gIHNob3VsZENvbXBvbmVudFVwZGF0ZTogZnVuY3Rpb24obmV3UHJvcHMpIHtcbiAgICB2YXIge3Jvd3MsIGNvbHN9ID0gbmV3UHJvcHM7XG5cbiAgICBpZiggIWlzTnVtYmVyKHJvd3MpIHx8ICFpc051bWJlcihjb2xzKSl7XG4gICAgICByZXR1cm4gZmFsc2U7XG4gICAgfVxuXG4gICAgaWYocm93cyAhPT0gdGhpcy5yb3dzIHx8IGNvbHMgIT09IHRoaXMuY29scyl7XG4gICAgICB0aGlzLnJlc2l6ZShjb2xzLCByb3dzKVxuICAgIH1cblxuICAgIHJldHVybiBmYWxzZTtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuICggPGRpdiBjbGFzc05hbWU9XCJncnYtdGVybWluYWxcIiBpZD1cInRlcm1pbmFsLWJveFwiIHJlZj1cImNvbnRhaW5lclwiPiAgPC9kaXY+ICk7XG4gIH0sXG5cbiAgcmVzaXplOiBmdW5jdGlvbihjb2xzLCByb3dzKSB7XG4gICAgLy8gaWYgbm90IGRlZmluZWQsIHVzZSB0aGUgc2l6ZSBvZiB0aGUgY29udGFpbmVyXG4gICAgaWYoIWlzTnVtYmVyKGNvbHMpIHx8ICFpc051bWJlcihyb3dzKSl7XG4gICAgICBsZXQgZGltID0gdGhpcy5fZ2V0RGltZW5zaW9ucygpO1xuICAgICAgY29scyA9IGRpbS5jb2xzO1xuICAgICAgcm93cyA9IGRpbS5yb3dzO1xuICAgIH1cblxuICAgIHRoaXMuY29scyA9IGNvbHM7XG4gICAgdGhpcy5yb3dzID0gcm93cztcblxuICAgIHRoaXMudGVybS5yZXNpemUodGhpcy5jb2xzLCB0aGlzLnJvd3MpO1xuICB9LFxuXG4gIF9nZXREaW1lbnNpb25zKCl7XG4gICAgbGV0ICRjb250YWluZXIgPSAkKHRoaXMucmVmcy5jb250YWluZXIpO1xuICAgIGxldCBmYWtlUm93ID0gJCgnPGRpdj48c3Bhbj4mbmJzcDs8L3NwYW4+PC9kaXY+Jyk7XG5cbiAgICAkY29udGFpbmVyLmZpbmQoJy50ZXJtaW5hbCcpLmFwcGVuZChmYWtlUm93KTtcbiAgICAvLyBnZXQgZGl2IGhlaWdodFxuICAgIGxldCBmYWtlQ29sSGVpZ2h0ID0gZmFrZVJvd1swXS5nZXRCb3VuZGluZ0NsaWVudFJlY3QoKS5oZWlnaHQ7XG4gICAgLy8gZ2V0IHNwYW4gd2lkdGhcbiAgICBsZXQgZmFrZUNvbFdpZHRoID0gZmFrZVJvdy5jaGlsZHJlbigpLmZpcnN0KClbMF0uZ2V0Qm91bmRpbmdDbGllbnRSZWN0KCkud2lkdGg7XG4gICAgbGV0IGNvbHMgPSBNYXRoLmZsb29yKCRjb250YWluZXIud2lkdGgoKSAvIChmYWtlQ29sV2lkdGgpKTtcbiAgICBsZXQgcm93cyA9IE1hdGguZmxvb3IoJGNvbnRhaW5lci5oZWlnaHQoKSAvIChmYWtlQ29sSGVpZ2h0KSk7XG4gICAgZmFrZVJvdy5yZW1vdmUoKTtcblxuICAgIHJldHVybiB7Y29scywgcm93c307XG4gIH1cblxufSk7XG5cblR0eVRlcm1pbmFsLnByb3BUeXBlcyA9IHtcbiAgdHR5OiBSZWFjdC5Qcm9wVHlwZXMub2JqZWN0LmlzUmVxdWlyZWRcbn1cblxubW9kdWxlLmV4cG9ydHMgPSBUdHlUZXJtaW5hbDtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL3Rlcm1pbmFsLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgcmVuZGVyID0gcmVxdWlyZSgncmVhY3QtZG9tJykucmVuZGVyO1xudmFyIHsgUm91dGVyLCBSb3V0ZSwgUmVkaXJlY3QsIEluZGV4Um91dGUsIGJyb3dzZXJIaXN0b3J5IH0gPSByZXF1aXJlKCdyZWFjdC1yb3V0ZXInKTtcbnZhciB7IEFwcCwgTG9naW4sIE5vZGVzLCBTZXNzaW9ucywgTmV3VXNlciwgQWN0aXZlU2Vzc2lvbkhvc3QsIE5vdEZvdW5kUGFnZSB9ID0gcmVxdWlyZSgnLi9jb21wb25lbnRzJyk7XG52YXIge2Vuc3VyZVVzZXJ9ID0gcmVxdWlyZSgnLi9tb2R1bGVzL3VzZXIvYWN0aW9ucycpO1xudmFyIGF1dGggPSByZXF1aXJlKCcuL2F1dGgnKTtcbnZhciBzZXNzaW9uID0gcmVxdWlyZSgnLi9zZXNzaW9uJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnLi9jb25maWcnKTtcblxucmVxdWlyZSgnLi9tb2R1bGVzJyk7XG5cbi8vIGluaXQgc2Vzc2lvblxuc2Vzc2lvbi5pbml0KCk7XG5cbmZ1bmN0aW9uIGhhbmRsZUxvZ291dChuZXh0U3RhdGUsIHJlcGxhY2UsIGNiKXtcbiAgYXV0aC5sb2dvdXQoKTtcbn1cblxucmVuZGVyKChcbiAgPFJvdXRlciBoaXN0b3J5PXtzZXNzaW9uLmdldEhpc3RvcnkoKX0+XG4gICAgPFJvdXRlIHBhdGg9e2NmZy5yb3V0ZXMubG9naW59IGNvbXBvbmVudD17TG9naW59Lz5cbiAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5sb2dvdXR9IG9uRW50ZXI9e2hhbmRsZUxvZ291dH0vPlxuICAgIDxSb3V0ZSBwYXRoPXtjZmcucm91dGVzLm5ld1VzZXJ9IGNvbXBvbmVudD17TmV3VXNlcn0vPlxuICAgIDxSZWRpcmVjdCBmcm9tPXtjZmcucm91dGVzLmFwcH0gdG89e2NmZy5yb3V0ZXMubm9kZXN9Lz5cbiAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5hcHB9IGNvbXBvbmVudD17QXBwfSBvbkVudGVyPXtlbnN1cmVVc2VyfSA+XG4gICAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5ub2Rlc30gY29tcG9uZW50PXtOb2Rlc30vPlxuICAgICAgPFJvdXRlIHBhdGg9e2NmZy5yb3V0ZXMuYWN0aXZlU2Vzc2lvbn0gY29tcG9uZW50cz17e2FjdGl2ZVNlc3Npb25Ib3N0OiBBY3RpdmVTZXNzaW9uSG9zdH19Lz5cbiAgICAgIDxSb3V0ZSBwYXRoPXtjZmcucm91dGVzLnNlc3Npb25zfSBjb21wb25lbnQ9e1Nlc3Npb25zfS8+XG4gICAgPC9Sb3V0ZT5cbiAgICA8Um91dGUgcGF0aD1cIipcIiBjb21wb25lbnQ9e05vdEZvdW5kUGFnZX0gLz5cbiAgPC9Sb3V0ZXI+XG4pLCBkb2N1bWVudC5nZXRFbGVtZW50QnlJZChcImFwcFwiKSk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvaW5kZXguanN4XG4gKiovIiwibW9kdWxlLmV4cG9ydHMgPSBUZXJtaW5hbDtcblxuXG4vKioqKioqKioqKioqKioqKipcbiAqKiBXRUJQQUNLIEZPT1RFUlxuICoqIGV4dGVybmFsIFwiVGVybWluYWxcIlxuICoqIG1vZHVsZSBpZCA9IDI5M1xuICoqIG1vZHVsZSBjaHVua3MgPSAxXG4gKiovIiwibW9kdWxlLmV4cG9ydHMgPSBfO1xuXG5cbi8qKioqKioqKioqKioqKioqKlxuICoqIFdFQlBBQ0sgRk9PVEVSXG4gKiogZXh0ZXJuYWwgXCJfXCJcbiAqKiBtb2R1bGUgaWQgPSAyOTRcbiAqKiBtb2R1bGUgY2h1bmtzID0gMVxuICoqLyJdLCJzb3VyY2VSb290IjoiIn0=