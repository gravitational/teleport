webpackJsonp([1],{

/***/ 0:
/***/ function(module, exports, __webpack_require__) {

	module.exports = __webpack_require__(304);


/***/ },

/***/ 7:
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

/***/ 11:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(265);
	
	var formatPattern = _require.formatPattern;
	
	var cfg = {
	
	  baseUrl: window.location.origin,
	
	  helpUrl: 'https://github.com/gravitational/teleport/blob/master/README.md',
	
	  api: {
	    renewTokenPath: '/v1/webapi/sessions/renew',
	    nodesPath: '/v1/webapi/sites/-current-/nodes',
	    sessionPath: '/v1/webapi/sessions',
	    siteSessionPath: '/v1/webapi/sites/-current-/sessions/:sid',
	    invitePath: '/v1/webapi/users/invites/:inviteToken',
	    createUserPath: '/v1/webapi/users',
	    sessionChunk: '/v1/webapi/sites/-current-/sessions/:sid/chunks?start=:start&end=:end',
	    sessionChunkCountPath: '/v1/webapi/sites/-current-/sessions/:sid/chunkscount',
	
	    getFetchSessionChunkUrl: function getFetchSessionChunkUrl(_ref) {
	      var sid = _ref.sid;
	      var start = _ref.start;
	      var end = _ref.end;
	
	      return formatPattern(cfg.api.sessionChunk, { sid: sid, start: start, end: end });
	    },
	
	    getFetchSessionLengthUrl: function getFetchSessionLengthUrl(sid) {
	      return formatPattern(cfg.api.sessionChunkCountPath, { sid: sid });
	    },
	
	    getFetchSessionsUrl: function getFetchSessionsUrl() /*start, end*/{
	      var params = {
	        start: new Date().toISOString(),
	        order: -1
	      };
	
	      var json = JSON.stringify(params);
	      var jsonEncoded = window.encodeURI(json);
	
	      return '/v1/webapi/sites/-current-/events/sessions?filter=' + jsonEncoded;
	    },
	
	    getFetchSessionUrl: function getFetchSessionUrl(sid) {
	      return formatPattern(cfg.api.siteSessionPath, { sid: sid });
	    },
	
	    getTerminalSessionUrl: function getTerminalSessionUrl(sid) {
	      return formatPattern(cfg.api.siteSessionPath, { sid: sid });
	    },
	
	    getInviteUrl: function getInviteUrl(inviteToken) {
	      return formatPattern(cfg.api.invitePath, { inviteToken: inviteToken });
	    },
	
	    getEventStreamConnStr: function getEventStreamConnStr(token, sid) {
	      var hostname = getWsHostName();
	      return hostname + '/v1/webapi/sites/-current-/sessions/' + sid + '/events/stream?access_token=' + token;
	    },
	
	    getTtyConnStr: function getTtyConnStr(_ref2) {
	      var token = _ref2.token;
	      var serverId = _ref2.serverId;
	      var login = _ref2.login;
	      var sid = _ref2.sid;
	      var rows = _ref2.rows;
	      var cols = _ref2.cols;
	
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

/***/ 20:
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

/***/ 26:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var _require = __webpack_require__(39);
	
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

/***/ 32:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var $ = __webpack_require__(42);
	var session = __webpack_require__(26);
	
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

/***/ 42:
/***/ function(module, exports) {

	module.exports = jQuery;

/***/ },

/***/ 43:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	var session = __webpack_require__(26);
	
	var _require = __webpack_require__(280);
	
	var uuid = _require.uuid;
	
	var api = __webpack_require__(32);
	var cfg = __webpack_require__(11);
	var getters = __webpack_require__(93);
	var sessionModule = __webpack_require__(106);
	
	var _require2 = __webpack_require__(91);
	
	var TLPT_TERM_OPEN = _require2.TLPT_TERM_OPEN;
	var TLPT_TERM_CLOSE = _require2.TLPT_TERM_CLOSE;
	var TLPT_TERM_CHANGE_SERVER = _require2.TLPT_TERM_CHANGE_SERVER;
	
	var actions = {
	
	  changeServer: function changeServer(serverId, login) {
	    reactor.dispatch(TLPT_TERM_CHANGE_SERVER, {
	      serverId: serverId,
	      login: login
	    });
	  },
	
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

/***/ 44:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(93);
	module.exports.actions = __webpack_require__(43);
	module.exports.activeTermStore = __webpack_require__(92);

/***/ },

/***/ 56:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(96);
	
	var TLPT_DIALOG_SHOW_SELECT_NODE = _require.TLPT_DIALOG_SHOW_SELECT_NODE;
	var TLPT_DIALOG_CLOSE_SELECT_NODE = _require.TLPT_DIALOG_CLOSE_SELECT_NODE;
	
	var actions = {
	  showSelectNodeDialog: function showSelectNodeDialog() {
	    reactor.dispatch(TLPT_DIALOG_SHOW_SELECT_NODE);
	  },
	
	  closeSelectNodeDialog: function closeSelectNodeDialog() {
	    reactor.dispatch(TLPT_DIALOG_CLOSE_SELECT_NODE);
	  }
	};
	
	exports['default'] = actions;
	module.exports = exports['default'];

/***/ },

/***/ 57:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	var api = __webpack_require__(32);
	var cfg = __webpack_require__(11);
	
	var _require = __webpack_require__(105);
	
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
	
	  fetchSessions: function fetchSessions() {
	    return api.get(cfg.api.getFetchSessionsUrl()).done(function (json) {
	      reactor.dispatch(TLPT_SESSINS_RECEIVE, json.sessions);
	    });
	  },
	
	  updateSession: function updateSession(json) {
	    reactor.dispatch(TLPT_SESSINS_UPDATE, json);
	  }
	};
	module.exports = exports['default'];

/***/ },

/***/ 58:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var toImmutable = _require.toImmutable;
	
	var reactor = __webpack_require__(7);
	var cfg = __webpack_require__(11);
	
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
	    active: session.get('active'),
	    created: new Date(session.get('created')),
	    lastActive: new Date(session.get('last_active')),
	    login: session.get('login'),
	    parties: parties,
	    cols: session.getIn(['terminal_params', 'w']),
	    rows: session.getIn(['terminal_params', 'h'])
	  };
	}
	
	exports['default'] = {
	  partiesBySessionId: partiesBySessionId,
	  sessionsByServer: sessionsByServer,
	  sessionsView: sessionsView,
	  sessionViewById: sessionViewById,
	  createView: createView
	};
	module.exports = exports['default'];

/***/ },

/***/ 59:
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

/***/ 89:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var api = __webpack_require__(32);
	var session = __webpack_require__(26);
	var cfg = __webpack_require__(11);
	var $ = __webpack_require__(42);
	
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

/***/ 90:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };
	
	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError('Cannot call a class as a function'); } }
	
	function _inherits(subClass, superClass) { if (typeof superClass !== 'function' && superClass !== null) { throw new TypeError('Super expression must either be null or a function, not ' + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }
	
	var EventEmitter = __webpack_require__(281).EventEmitter;
	var session = __webpack_require__(26);
	var cfg = __webpack_require__(11);
	
	var _require = __webpack_require__(44);
	
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
	
	  Tty.prototype.reconnect = function reconnect(options) {
	    this.socket.close();
	    this.connect(options);
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

/***/ 91:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(20);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_TERM_OPEN: null,
	  TLPT_TERM_CLOSE: null,
	  TLPT_TERM_CHANGE_SERVER: null
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
	
	var TLPT_TERM_OPEN = _require2.TLPT_TERM_OPEN;
	var TLPT_TERM_CLOSE = _require2.TLPT_TERM_CLOSE;
	var TLPT_TERM_CHANGE_SERVER = _require2.TLPT_TERM_CHANGE_SERVER;
	exports['default'] = Store({
	  getInitialState: function getInitialState() {
	    return toImmutable(null);
	  },
	
	  initialize: function initialize() {
	    this.on(TLPT_TERM_OPEN, setActiveTerminal);
	    this.on(TLPT_TERM_CLOSE, close);
	    this.on(TLPT_TERM_CHANGE_SERVER, changeServer);
	  }
	});
	
	function changeServer(state, _ref) {
	  var serverId = _ref.serverId;
	  var login = _ref.login;
	
	  return state.set('serverId', serverId).set('login', login);
	}
	
	function close() {
	  return toImmutable(null);
	}
	
	function setActiveTerminal(state, _ref2) {
	  var serverId = _ref2.serverId;
	  var login = _ref2.login;
	  var sid = _ref2.sid;
	  var isNewSession = _ref2.isNewSession;
	
	  return toImmutable({
	    serverId: serverId,
	    login: login,
	    sid: sid,
	    isNewSession: isNewSession
	  });
	}
	module.exports = exports['default'];

/***/ },

/***/ 93:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(58);
	
	var createView = _require.createView;
	
	var activeSession = [['tlpt_active_terminal'], ['tlpt_sessions'], function (activeTerm, sessions) {
	  if (!activeTerm) {
	    return null;
	  }
	
	  /*
	  * active session needs to have its own view as an actual session might not
	  * exist at this point. For example, upon creating a new session we need to know
	  * login and serverId. It will be simplified once server API gets extended.
	  */
	  var asView = {
	    isNewSession: activeTerm.get('isNewSession'),
	    notFound: activeTerm.get('notFound'),
	    addr: activeTerm.get('addr'),
	    serverId: activeTerm.get('serverId'),
	    serverIp: undefined,
	    login: activeTerm.get('login'),
	    sid: activeTerm.get('sid'),
	    cols: undefined,
	    rows: undefined
	  };
	
	  // in case if session already exists, get the data from there
	  // (for example, when joining an existing session)
	  if (sessions.has(asView.sid)) {
	    var sView = createView(sessions.get(asView.sid));
	
	    asView.parties = sView.parties;
	    asView.serverIp = sView.serverIp;
	    asView.serverId = sView.serverId;
	    asView.active = sView.active;
	    asView.cols = sView.cols;
	    asView.rows = sView.rows;
	  }
	
	  return asView;
	}];
	
	exports['default'] = {
	  activeSession: activeSession
	};
	module.exports = exports['default'];

/***/ },

/***/ 94:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(20);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_APP_INIT: null,
	  TLPT_APP_FAILED: null,
	  TLPT_APP_READY: null
	});
	module.exports = exports['default'];

/***/ },

/***/ 95:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(94);
	
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

/***/ 96:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(20);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_DIALOG_SHOW_SELECT_NODE: null,
	  TLPT_DIALOG_CLOSE_SELECT_NODE: null
	});
	module.exports = exports['default'];

/***/ },

/***/ 97:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(96);
	
	var TLPT_DIALOG_SHOW_SELECT_NODE = _require2.TLPT_DIALOG_SHOW_SELECT_NODE;
	var TLPT_DIALOG_CLOSE_SELECT_NODE = _require2.TLPT_DIALOG_CLOSE_SELECT_NODE;
	exports['default'] = Store({
	
	  getInitialState: function getInitialState() {
	    return toImmutable({
	      isSelectNodeDialogOpen: false
	    });
	  },
	
	  initialize: function initialize() {
	    this.on(TLPT_DIALOG_SHOW_SELECT_NODE, showSelectNodeDialog);
	    this.on(TLPT_DIALOG_CLOSE_SELECT_NODE, closeSelectNodeDialog);
	  }
	});
	
	function showSelectNodeDialog(state) {
	  return state.set('isSelectNodeDialogOpen', true);
	}
	
	function closeSelectNodeDialog(state) {
	  return state.set('isSelectNodeDialogOpen', false);
	}
	module.exports = exports['default'];

/***/ },

/***/ 98:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(20);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_RECEIVE_USER_INVITE: null
	});
	module.exports = exports['default'];

/***/ },

/***/ 99:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(98);
	
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

/***/ 100:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(20);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_NODES_RECEIVE: null
	});
	module.exports = exports['default'];

/***/ },

/***/ 101:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(100);
	
	var TLPT_NODES_RECEIVE = _require.TLPT_NODES_RECEIVE;
	
	var api = __webpack_require__(32);
	var cfg = __webpack_require__(11);
	
	exports['default'] = {
	  fetchNodes: function fetchNodes() {
	    api.get(cfg.api.nodesPath).done(function () {
	      var data = arguments.length <= 0 || arguments[0] === undefined ? [] : arguments[0];
	
	      var nodeArray = data.nodes.map(function (item) {
	        return item.node;
	      });
	      reactor.dispatch(TLPT_NODES_RECEIVE, nodeArray);
	    });
	  }
	};
	module.exports = exports['default'];

/***/ },

/***/ 102:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(100);
	
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

/***/ 103:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(20);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_REST_API_START: null,
	  TLPT_REST_API_SUCCESS: null,
	  TLPT_REST_API_FAIL: null
	});
	module.exports = exports['default'];

/***/ },

/***/ 104:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(20);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TRYING_TO_SIGN_UP: null
	});
	module.exports = exports['default'];

/***/ },

/***/ 105:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(20);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_SESSINS_RECEIVE: null,
	  TLPT_SESSINS_UPDATE: null
	});
	module.exports = exports['default'];

/***/ },

/***/ 106:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(58);
	module.exports.actions = __webpack_require__(57);
	module.exports.activeTermStore = __webpack_require__(107);

/***/ },

/***/ 107:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(105);
	
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
	
	function receiveSessions(state) {
	  var jsonArray = arguments.length <= 1 || arguments[1] === undefined ? [] : arguments[1];
	
	  return state.withMutations(function (state) {
	    jsonArray.forEach(function (item) {
	      state.set(item.id, toImmutable(item));
	    });
	  });
	}
	module.exports = exports['default'];

/***/ },

/***/ 108:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(20);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_RECEIVE_USER: null
	});
	module.exports = exports['default'];

/***/ },

/***/ 109:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(108);
	
	var TLPT_RECEIVE_USER = _require.TLPT_RECEIVE_USER;
	
	var _require2 = __webpack_require__(104);
	
	var TRYING_TO_SIGN_UP = _require2.TRYING_TO_SIGN_UP;
	
	var restApiActions = __webpack_require__(278);
	var auth = __webpack_require__(89);
	var session = __webpack_require__(26);
	var cfg = __webpack_require__(11);
	
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

/***/ 110:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(59);
	module.exports.actions = __webpack_require__(109);
	module.exports.nodeStore = __webpack_require__(111);

/***/ },

/***/ 111:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(108);
	
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

/***/ 215:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	
	var _require = __webpack_require__(44);
	
	var actions = _require.actions;
	
	var colors = ['#1ab394', '#1c84c6', '#23c6c8', '#f8ac59', '#ED5565', '#c2c2c2'];
	
	var UserIcon = function UserIcon(_ref) {
	  var name = _ref.name;
	  var title = _ref.title;
	  var _ref$colorIndex = _ref.colorIndex;
	  var colorIndex = _ref$colorIndex === undefined ? 0 : _ref$colorIndex;
	
	  var color = colors[colorIndex % colors.length];
	  var style = {
	    'backgroundColor': color,
	    'borderColor': color
	  };
	
	  return React.createElement(
	    'li',
	    null,
	    React.createElement(
	      'span',
	      { style: style, className: 'btn btn-primary btn-circle text-uppercase' },
	      React.createElement(
	        'strong',
	        null,
	        name[0]
	      )
	    )
	  );
	};
	
	var SessionLeftPanel = function SessionLeftPanel(_ref2) {
	  var parties = _ref2.parties;
	
	  parties = parties || [];
	  var userIcons = parties.map(function (item, index) {
	    return React.createElement(UserIcon, { key: index, colorIndex: index, name: item.user });
	  });
	
	  return React.createElement(
	    'div',
	    { className: 'grv-terminal-participans' },
	    React.createElement(
	      'ul',
	      { className: 'nav' },
	      userIcons,
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
	  );
	};
	
	module.exports = SessionLeftPanel;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "sessionLeftPanel.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 216:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	"use strict";
	
	var React = __webpack_require__(4);
	
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

/***/ 217:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }
	
	var React = __webpack_require__(4);
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(277);
	
	var getters = _require.getters;
	var actions = _require.actions;
	
	var userGetters = __webpack_require__(59);
	
	var _require2 = __webpack_require__(219);
	
	var Table = _require2.Table;
	var Column = _require2.Column;
	var Cell = _require2.Cell;
	
	var _require3 = __webpack_require__(43);
	
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
	  var onLoginClick = _ref3.onLoginClick;
	  var rowIndex = _ref3.rowIndex;
	  var data = _ref3.data;
	
	  var props = _objectWithoutProperties(_ref3, ['user', 'onLoginClick', 'rowIndex', 'data']);
	
	  if (!user || user.logins.length === 0) {
	    return React.createElement(Cell, props);
	  }
	
	  var serverId = data[rowIndex].id;
	  var $lis = [];
	
	  function onClick(i) {
	    var login = user.logins[i];
	    if (onLoginClick) {
	      return function () {
	        return onLoginClick(serverId, login);
	      };
	    } else {
	      return function () {
	        return createNewSession(serverId, login);
	      };
	    }
	  }
	
	  for (var i = 0; i < user.logins.length; i++) {
	    $lis.push(React.createElement(
	      'li',
	      { key: i },
	      React.createElement(
	        'a',
	        { onClick: onClick(i) },
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
	        { type: 'button', onClick: onClick(0), className: 'btn btn-xs btn-primary' },
	        user.logins[0]
	      ),
	      $lis.length > 1 ? [React.createElement(
	        'button',
	        { key: 0, 'data-toggle': 'dropdown', className: 'btn btn-default btn-xs dropdown-toggle', 'aria-expanded': 'true' },
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
	    var onLoginClick = this.props.onLoginClick;
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
	              { rowCount: data.length, className: 'table-striped grv-nodes-table' },
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
	                onLoginClick: onLoginClick,
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

/***/ 218:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	"use strict";
	
	var React = __webpack_require__(4);
	
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

/***/ 219:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	exports.__esModule = true;
	
	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };
	
	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }
	
	var React = __webpack_require__(4);
	
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

/***/ 220:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var Term = __webpack_require__(404);
	var React = __webpack_require__(4);
	
	var _require = __webpack_require__(405);
	
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
	    this.tty.on('reset', function () {
	      return _this2.term.reset();
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

/***/ 265:
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
	
	var _invariant = __webpack_require__(13);
	
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

/***/ 266:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError('Cannot call a class as a function'); } }
	
	function _inherits(subClass, superClass) { if (typeof superClass !== 'function' && superClass !== null) { throw new TypeError('Super expression must either be null or a function, not ' + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }
	
	var Tty = __webpack_require__(90);
	var api = __webpack_require__(32);
	var cfg = __webpack_require__(11);
	
	var TtyPlayer = (function (_Tty) {
	  _inherits(TtyPlayer, _Tty);
	
	  function TtyPlayer(_ref) {
	    var sid = _ref.sid;
	
	    _classCallCheck(this, TtyPlayer);
	
	    _Tty.call(this, {});
	    this.sid = sid;
	    this.current = 1;
	    this.length = -1;
	    this.ttySteam = new Array();
	    this.isLoaind = false;
	    this.isPlaying = false;
	    this.isError = false;
	    this.isReady = false;
	    this.isLoading = true;
	  }
	
	  TtyPlayer.prototype.send = function send() {};
	
	  TtyPlayer.prototype.resize = function resize() {};
	
	  TtyPlayer.prototype.connect = function connect() {
	    var _this = this;
	
	    api.get(cfg.api.getFetchSessionLengthUrl(this.sid)).done(function (data) {
	      _this.length = data.count;
	      _this.isReady = true;
	    }).fail(function () {
	      _this.isError = true;
	    }).always(function () {
	      _this._change();
	    });
	  };
	
	  TtyPlayer.prototype.move = function move(newPos) {
	    if (!this.isReady) {
	      return;
	    }
	
	    if (newPos === undefined) {
	      newPos = this.current + 1;
	    }
	
	    if (newPos > this.length) {
	      newPos = this.length;
	      this.stop();
	    }
	
	    if (newPos === 0) {
	      newPos = 1;
	    }
	
	    if (this.isPlaying) {
	      if (this.current < newPos) {
	        this._showChunk(this.current, newPos);
	      } else {
	        this.emit('reset');
	        this._showChunk(this.current, newPos);
	      }
	    } else {
	      this.current = newPos;
	    }
	
	    this._change();
	  };
	
	  TtyPlayer.prototype.stop = function stop() {
	    this.isPlaying = false;
	    this.timer = clearInterval(this.timer);
	    this._change();
	  };
	
	  TtyPlayer.prototype.play = function play() {
	    if (this.isPlaying) {
	      return;
	    }
	
	    this.isPlaying = true;
	    this.timer = setInterval(this.move.bind(this), 150);
	    this._change();
	  };
	
	  TtyPlayer.prototype._shouldFetch = function _shouldFetch(start, end) {
	    for (var i = start; i < end; i++) {
	      if (this.ttySteam[i] === undefined) {
	        return true;
	      }
	    }
	
	    return false;
	  };
	
	  TtyPlayer.prototype._fetch = function _fetch(start, end) {
	    var _this2 = this;
	
	    end = end + 50;
	    end = end > this.length ? this.length : end;
	    return api.get(cfg.api.getFetchSessionChunkUrl({ sid: this.sid, start: start, end: end })).done(function (response) {
	      for (var i = 0; i < end - start; i++) {
	        var data = atob(response.chunks[i].data) || '';
	        var delay = response.chunks[i].delay;
	        _this2.ttySteam[start + i] = { data: data, delay: delay };
	      }
	    });
	  };
	
	  TtyPlayer.prototype._showChunk = function _showChunk(start, end) {
	    var _this3 = this;
	
	    var display = function display() {
	      for (var i = start; i < end; i++) {
	        _this3.emit('data', _this3.ttySteam[i].data);
	      }
	      _this3.current = end;
	    };
	
	    if (this._shouldFetch(start, end)) {
	      this._fetch(start, end).then(display);
	    } else {
	      display();
	    }
	  };
	
	  TtyPlayer.prototype._change = function _change() {
	    this.emit('change');
	  };
	
	  return TtyPlayer;
	})(Tty);
	
	exports['default'] = TtyPlayer;
	module.exports = exports['default'];

/***/ },

/***/ 267:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(57);
	
	var fetchSessions = _require.fetchSessions;
	
	var _require2 = __webpack_require__(101);
	
	var fetchNodes = _require2.fetchNodes;
	
	var $ = __webpack_require__(42);
	
	var _require3 = __webpack_require__(94);
	
	var TLPT_APP_INIT = _require3.TLPT_APP_INIT;
	var TLPT_APP_FAILED = _require3.TLPT_APP_FAILED;
	var TLPT_APP_READY = _require3.TLPT_APP_READY;
	
	var actions = {
	
	  initApp: function initApp() {
	    reactor.dispatch(TLPT_APP_INIT);
	    actions.fetchNodesAndSessions().done(function () {
	      reactor.dispatch(TLPT_APP_READY);
	    }).fail(function () {
	      reactor.dispatch(TLPT_APP_FAILED);
	    });
	
	    //api.get(`/v1/webapi/sites/-current-/sessions/03d3e11d-45c1-4049-bceb-b233605666e4/chunks?start=0&end=100`).done(() => {
	    //});
	  },
	
	  fetchNodesAndSessions: function fetchNodesAndSessions() {
	    return $.when(fetchNodes(), fetchSessions());
	  }
	};
	
	exports['default'] = actions;
	module.exports = exports['default'];

/***/ },

/***/ 268:
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

/***/ 269:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(268);
	module.exports.actions = __webpack_require__(267);
	module.exports.appStore = __webpack_require__(95);

/***/ },

/***/ 270:
/***/ function(module, exports) {

	'use strict';
	
	exports.__esModule = true;
	var dialogs = [['tlpt_dialogs'], function (state) {
	  return state.toJS();
	}];
	
	exports['default'] = {
	  dialogs: dialogs
	};
	module.exports = exports['default'];

/***/ },

/***/ 271:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(270);
	module.exports.actions = __webpack_require__(56);
	module.exports.dialogStore = __webpack_require__(97);

/***/ },

/***/ 272:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var reactor = __webpack_require__(7);
	reactor.registerStores({
	  'tlpt': __webpack_require__(95),
	  'tlpt_dialogs': __webpack_require__(97),
	  'tlpt_active_terminal': __webpack_require__(92),
	  'tlpt_user': __webpack_require__(111),
	  'tlpt_nodes': __webpack_require__(102),
	  'tlpt_invite': __webpack_require__(99),
	  'tlpt_rest_api': __webpack_require__(279),
	  'tlpt_sessions': __webpack_require__(107)
	});

/***/ },

/***/ 273:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(98);
	
	var TLPT_RECEIVE_USER_INVITE = _require.TLPT_RECEIVE_USER_INVITE;
	
	var api = __webpack_require__(32);
	var cfg = __webpack_require__(11);
	
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

/***/ 274:
/***/ function(module, exports, __webpack_require__) {

	/*eslint no-undef: 0,  no-unused-vars: 0, no-debugger:0*/
	
	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(104);
	
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

/***/ 275:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(274);
	module.exports.actions = __webpack_require__(273);
	module.exports.nodeStore = __webpack_require__(99);

/***/ },

/***/ 276:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(58);
	
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

/***/ 277:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(276);
	module.exports.actions = __webpack_require__(101);
	module.exports.nodeStore = __webpack_require__(102);
	
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

/***/ 278:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(103);
	
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

/***/ 279:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(103);
	
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

/***/ 280:
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

/***/ 281:
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

/***/ 293:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	var NavLeftBar = __webpack_require__(300);
	var cfg = __webpack_require__(11);
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(269);
	
	var actions = _require.actions;
	var getters = _require.getters;
	
	var SelectNodeDialog = __webpack_require__(302);
	
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
	    this.refreshInterval = setInterval(actions.fetchNodesAndSessions, 35000);
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
	      React.createElement(SelectNodeDialog, null),
	      this.props.CurrentSessionHost,
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

/***/ 294:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	
	var _require = __webpack_require__(44);
	
	var getters = _require.getters;
	var actions = _require.actions;
	
	var Tty = __webpack_require__(90);
	var TtyTerminal = __webpack_require__(220);
	var EventStreamer = __webpack_require__(295);
	var SessionLeftPanel = __webpack_require__(215);
	
	var _require2 = __webpack_require__(56);
	
	var showSelectNodeDialog = _require2.showSelectNodeDialog;
	var closeSelectNodeDialog = _require2.closeSelectNodeDialog;
	
	var ActiveSession = React.createClass({
	  displayName: 'ActiveSession',
	
	  componentWillUnmount: function componentWillUnmount() {
	    closeSelectNodeDialog();
	  },
	
	  render: function render() {
	    var _props$activeSession = this.props.activeSession;
	    var serverIp = _props$activeSession.serverIp;
	    var login = _props$activeSession.login;
	    var parties = _props$activeSession.parties;
	
	    var serverLabelText = login + '@' + serverIp;
	
	    if (!serverIp) {
	      serverLabelText = '';
	    }
	
	    return React.createElement(
	      'div',
	      { className: 'grv-current-session' },
	      React.createElement(SessionLeftPanel, { parties: parties }),
	      React.createElement(
	        'div',
	        null,
	        React.createElement(
	          'div',
	          { className: 'grv-current-session-server-info' },
	          React.createElement(
	            'span',
	            { className: 'btn btn-primary btn-sm', onClick: showSelectNodeDialog },
	            'Change node'
	          ),
	          React.createElement(
	            'h3',
	            null,
	            serverLabelText
	          )
	        )
	      ),
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
	
	  componentWillReceiveProps: function componentWillReceiveProps(nextProps) {
	    if (nextProps.serverId !== this.props.serverId || nextProps.login !== this.props.login) {
	      this.tty.reconnect(nextProps);
	      this.refs.ttyCmntInstance.term.focus();
	    }
	  },
	
	  render: function render() {
	    return React.createElement(
	      'div',
	      { style: { height: '100%' } },
	      React.createElement(TtyTerminal, { ref: 'ttyCmntInstance', tty: this.tty, cols: this.props.cols, rows: this.props.rows }),
	      this.state.isConnected ? React.createElement(EventStreamer, { sid: this.props.sid }) : null
	    );
	  }
	});
	
	module.exports = ActiveSession;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "activeSession.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 295:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	exports.__esModule = true;
	var cfg = __webpack_require__(11);
	var React = __webpack_require__(4);
	var session = __webpack_require__(26);
	
	var _require = __webpack_require__(57);
	
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

/***/ 296:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	
	var _require = __webpack_require__(44);
	
	var getters = _require.getters;
	var actions = _require.actions;
	
	var NotFoundPage = __webpack_require__(218);
	var SessionPlayer = __webpack_require__(297);
	var ActiveSession = __webpack_require__(294);
	
	var CurrentSessionHost = React.createClass({
	  displayName: 'CurrentSessionHost',
	
	  mixins: [reactor.ReactMixin],
	
	  getDataBindings: function getDataBindings() {
	    return {
	      currentSession: getters.activeSession
	    };
	  },
	
	  componentDidMount: function componentDidMount() {
	    var sid = this.props.params.sid;
	
	    if (!this.state.currentSession) {
	      actions.openSession(sid);
	    }
	  },
	
	  render: function render() {
	    var currentSession = this.state.currentSession;
	    if (!currentSession) {
	      return null;
	    }
	
	    if (currentSession.isNewSession || currentSession.active) {
	      return React.createElement(ActiveSession, { activeSession: currentSession });
	    }
	
	    return React.createElement(SessionPlayer, { activeSession: currentSession });
	  }
	});
	
	module.exports = CurrentSessionHost;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "main.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 297:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	exports.__esModule = true;
	var React = __webpack_require__(4);
	var ReactSlider = __webpack_require__(228);
	var TtyPlayer = __webpack_require__(266);
	var TtyTerminal = __webpack_require__(220);
	var SessionLeftPanel = __webpack_require__(215);
	
	var SessionPlayer = React.createClass({
	  displayName: 'SessionPlayer',
	
	  calculateState: function calculateState() {
	    return {
	      length: this.tty.length,
	      min: 1,
	      isPlaying: this.tty.isPlaying,
	      current: this.tty.current,
	      canPlay: this.tty.length > 1
	    };
	  },
	
	  getInitialState: function getInitialState() {
	    var sid = this.props.activeSession.sid;
	    this.tty = new TtyPlayer({ sid: sid });
	    return this.calculateState();
	  },
	
	  componentWillUnmount: function componentWillUnmount() {
	    this.tty.stop();
	    this.tty.removeAllListeners();
	  },
	
	  componentDidMount: function componentDidMount() {
	    var _this = this;
	
	    this.tty.on('change', function () {
	      var newState = _this.calculateState();
	      _this.setState(newState);
	    });
	  },
	
	  togglePlayStop: function togglePlayStop() {
	    if (this.state.isPlaying) {
	      this.tty.stop();
	    } else {
	      this.tty.play();
	    }
	  },
	
	  move: function move(value) {
	    this.tty.move(value);
	  },
	
	  onBeforeChange: function onBeforeChange() {
	    this.tty.stop();
	  },
	
	  onAfterChange: function onAfterChange(value) {
	    this.tty.play();
	    this.tty.move(value);
	  },
	
	  render: function render() {
	    var isPlaying = this.state.isPlaying;
	
	    return React.createElement(
	      'div',
	      { className: 'grv-current-session grv-session-player' },
	      React.createElement(SessionLeftPanel, null),
	      React.createElement(TtyTerminal, { ref: 'term', tty: this.tty, cols: '5', rows: '5' }),
	      React.createElement(ReactSlider, {
	        min: this.state.min,
	        max: this.state.length,
	        value: this.state.current,
	        onAfterChange: this.onAfterChange,
	        onBeforeChange: this.onBeforeChange,
	        defaultValue: 1,
	        withBars: true,
	        className: 'grv-slider' }),
	      React.createElement(
	        'button',
	        { className: 'btn', onClick: this.togglePlayStop },
	        isPlaying ? React.createElement('i', { className: 'fa fa-stop' }) : React.createElement('i', { className: 'fa fa-play' })
	      )
	    );
	  }
	});
	
	exports['default'] = SessionPlayer;
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "sessionPlayer.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 298:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	module.exports.App = __webpack_require__(293);
	module.exports.Login = __webpack_require__(299);
	module.exports.NewUser = __webpack_require__(301);
	module.exports.Nodes = __webpack_require__(217);
	module.exports.Sessions = __webpack_require__(303);
	module.exports.CurrentSessionHost = __webpack_require__(296);
	module.exports.NotFoundPage = __webpack_require__(218);
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "index.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 299:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	var $ = __webpack_require__(42);
	var reactor = __webpack_require__(7);
	var LinkedStateMixin = __webpack_require__(63);
	
	var _require = __webpack_require__(110);
	
	var actions = _require.actions;
	
	var GoogleAuthInfo = __webpack_require__(216);
	var cfg = __webpack_require__(11);
	
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
	          React.createElement('input', { autoFocus: true, valueLink: this.linkState('user'), className: 'form-control required', placeholder: 'User name', name: 'userName' })
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

/***/ 300:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	
	var _require = __webpack_require__(39);
	
	var Router = _require.Router;
	var IndexLink = _require.IndexLink;
	var History = _require.History;
	
	var getters = __webpack_require__(59);
	var cfg = __webpack_require__(11);
	
	var menuItems = [{ icon: 'fa fa-cogs', to: cfg.routes.nodes, title: 'Nodes' }, { icon: 'fa fa-sitemap', to: cfg.routes.sessions, title: 'Sessions' }];
	
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
	
	    items.push(React.createElement(
	      'li',
	      { key: menuItems.length },
	      React.createElement(
	        'a',
	        { href: cfg.helpUrl },
	        React.createElement('i', { className: 'fa fa-question', title: 'help' })
	      )
	    ));
	
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

/***/ 301:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	var $ = __webpack_require__(42);
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(275);
	
	var actions = _require.actions;
	var getters = _require.getters;
	
	var userModule = __webpack_require__(110);
	var LinkedStateMixin = __webpack_require__(63);
	var GoogleAuthInfo = __webpack_require__(216);
	
	var InviteInputForm = React.createClass({
	  displayName: 'InviteInputForm',
	
	  mixins: [LinkedStateMixin],
	
	  componentDidMount: function componentDidMount() {
	    $(this.refs.form).validate({
	      rules: {
	        password: {
	          minlength: 6,
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
	            autoFocus: true,
	            valueLink: this.linkState('psw'),
	            ref: 'password',
	            type: 'password',
	            name: 'password',
	            className: 'form-control',
	            placeholder: 'Password' })
	        ),
	        React.createElement(
	          'div',
	          { className: 'form-group' },
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
	          { className: 'grv-flex-column grv-invite-barcode' },
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

/***/ 302:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(271);
	
	var getters = _require.getters;
	
	var _require2 = __webpack_require__(56);
	
	var closeSelectNodeDialog = _require2.closeSelectNodeDialog;
	
	var _require3 = __webpack_require__(43);
	
	var changeServer = _require3.changeServer;
	
	var NodeList = __webpack_require__(217);
	
	var SelectNodeDialog = React.createClass({
	  displayName: 'SelectNodeDialog',
	
	  mixins: [reactor.ReactMixin],
	
	  getDataBindings: function getDataBindings() {
	    return {
	      dialogs: getters.dialogs
	    };
	  },
	
	  render: function render() {
	    return this.state.dialogs.isSelectNodeDialogOpen ? React.createElement(Dialog, null) : null;
	  }
	});
	
	var Dialog = React.createClass({
	  displayName: 'Dialog',
	
	  onLoginClick: function onLoginClick(serverId, login) {
	    changeServer(serverId, login);
	    closeSelectNodeDialog();
	  },
	
	  componentWillUnmount: function componentWillUnmount(callback) {
	    $('.modal').modal('hide');
	  },
	
	  componentDidMount: function componentDidMount() {
	    $('.modal').modal('show');
	  },
	
	  render: function render() {
	    return React.createElement(
	      'div',
	      { className: 'modal fade grv-dialog-select-node', tabIndex: -1, role: 'dialog' },
	      React.createElement(
	        'div',
	        { className: 'modal-dialog' },
	        React.createElement(
	          'div',
	          { className: 'modal-content' },
	          React.createElement('div', { className: 'modal-header' }),
	          React.createElement(
	            'div',
	            { className: 'modal-body' },
	            React.createElement(NodeList, { onLoginClick: this.onLoginClick })
	          ),
	          React.createElement(
	            'div',
	            { className: 'modal-footer' },
	            React.createElement(
	              'button',
	              { onClick: closeSelectNodeDialog, type: 'button', className: 'btn btn-primary' },
	              'Close'
	            )
	          )
	        )
	      )
	    );
	  }
	});
	
	module.exports = SelectNodeDialog;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "selectNodeDialog.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 303:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }
	
	var React = __webpack_require__(4);
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(39);
	
	var Link = _require.Link;
	
	var _require2 = __webpack_require__(219);
	
	var Table = _require2.Table;
	var Column = _require2.Column;
	var Cell = _require2.Cell;
	var TextCell = _require2.TextCell;
	
	var _require3 = __webpack_require__(106);
	
	var getters = _require3.getters;
	
	var _require4 = __webpack_require__(43);
	
	var open = _require4.open;
	
	var moment = __webpack_require__(1);
	
	var DateCreatedCell = function DateCreatedCell(_ref) {
	  var rowIndex = _ref.rowIndex;
	  var data = _ref.data;
	
	  var props = _objectWithoutProperties(_ref, ['rowIndex', 'data']);
	
	  var created = data[rowIndex].created;
	  var displayDate = moment(created).fromNow();
	  return React.createElement(
	    Cell,
	    props,
	    displayDate
	  );
	};
	
	var DurationCell = function DurationCell(_ref2) {
	  var rowIndex = _ref2.rowIndex;
	  var data = _ref2.data;
	
	  var props = _objectWithoutProperties(_ref2, ['rowIndex', 'data']);
	
	  var created = data[rowIndex].created;
	  var lastActive = data[rowIndex].lastActive;
	
	  var end = moment(created);
	  var now = moment(lastActive);
	  var duration = moment.duration(now.diff(end));
	  var displayDate = duration.humanize();
	
	  return React.createElement(
	    Cell,
	    props,
	    displayDate
	  );
	};
	
	var UsersCell = function UsersCell(_ref3) {
	  var rowIndex = _ref3.rowIndex;
	  var data = _ref3.data;
	
	  var props = _objectWithoutProperties(_ref3, ['rowIndex', 'data']);
	
	  var $users = data[rowIndex].parties.map(function (item, itemIndex) {
	    return React.createElement(
	      'span',
	      { key: itemIndex, style: { backgroundColor: '#1ab394' }, className: 'text-uppercase grv-rounded label label-primary' },
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
	
	var ButtonCell = function ButtonCell(_ref4) {
	  var rowIndex = _ref4.rowIndex;
	  var data = _ref4.data;
	
	  var props = _objectWithoutProperties(_ref4, ['rowIndex', 'data']);
	
	  var _data$rowIndex = data[rowIndex];
	  var sessionUrl = _data$rowIndex.sessionUrl;
	  var active = _data$rowIndex.active;
	
	  var _ref5 = active ? ['join', 'btn-warning'] : ['play', 'btn-primary'];
	
	  var actionText = _ref5[0];
	  var actionClass = _ref5[1];
	
	  return React.createElement(
	    Cell,
	    props,
	    React.createElement(
	      Link,
	      { to: sessionUrl, className: "btn " + actionClass + " btn-xs", type: 'button' },
	      actionText
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
	              { rowCount: data.length, className: 'table-striped' },
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
	                columnKey: 'created',
	                header: React.createElement(
	                  Cell,
	                  null,
	                  ' Created '
	                ),
	                cell: React.createElement(DateCreatedCell, { data: data })
	              }),
	              React.createElement(Column, {
	                columnKey: 'serverId',
	                header: React.createElement(
	                  Cell,
	                  null,
	                  ' Active '
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

/***/ 304:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	var render = __webpack_require__(214).render;
	
	var _require = __webpack_require__(39);
	
	var Router = _require.Router;
	var Route = _require.Route;
	var Redirect = _require.Redirect;
	var IndexRoute = _require.IndexRoute;
	var browserHistory = _require.browserHistory;
	
	var _require2 = __webpack_require__(298);
	
	var App = _require2.App;
	var Login = _require2.Login;
	var Nodes = _require2.Nodes;
	var Sessions = _require2.Sessions;
	var NewUser = _require2.NewUser;
	var CurrentSessionHost = _require2.CurrentSessionHost;
	var NotFoundPage = _require2.NotFoundPage;
	
	var _require3 = __webpack_require__(109);
	
	var ensureUser = _require3.ensureUser;
	
	var auth = __webpack_require__(89);
	var session = __webpack_require__(26);
	var cfg = __webpack_require__(11);
	
	__webpack_require__(272);
	
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
	    React.createElement(Route, { path: cfg.routes.activeSession, components: { CurrentSessionHost: CurrentSessionHost } }),
	    React.createElement(Route, { path: cfg.routes.sessions, component: Sessions })
	  ),
	  React.createElement(Route, { path: '*', component: NotFoundPage })
	), document.getElementById("app"));
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "index.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 404:
/***/ function(module, exports) {

	module.exports = Terminal;

/***/ },

/***/ 405:
/***/ function(module, exports) {

	module.exports = _;

/***/ }

});
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJzb3VyY2VzIjpbIndlYnBhY2s6Ly8vLi9zcmMvYXBwL3JlYWN0b3IuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb25maWcuanMiLCJ3ZWJwYWNrOi8vLy4vfi9rZXltaXJyb3IvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9zZXNzaW9uLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvc2VydmljZXMvYXBpLmpzIiwid2VicGFjazovLy9leHRlcm5hbCBcImpRdWVyeVwiIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvZGlhbG9ncy9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9zZXNzaW9ucy9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9zZXNzaW9ucy9nZXR0ZXJzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy91c2VyL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9hdXRoLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tbW9uL3R0eS5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvYWN0aW9uVHlwZXMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGl2ZVRlcm1TdG9yZS5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvYXBwL2FjdGlvblR5cGVzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hcHAvYXBwU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvYWN0aW9uVHlwZXMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvZGlhbG9nU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2ludml0ZVN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvbm9kZVN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2FjdGlvblR5cGVzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2NvbnN0YW50cy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvYWN0aW9uVHlwZXMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9zZXNzaW9ucy9zZXNzaW9uU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvYWN0aW9uVHlwZXMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvdXNlci9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvdXNlci91c2VyU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2N1cnJlbnRTZXNzaW9uL3Nlc3Npb25MZWZ0UGFuZWwuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9nb29nbGVBdXRoTG9nby5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL25vZGVzL21haW4uanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9ub3RGb3VuZFBhZ2UuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy90YWJsZS5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL3Rlcm1pbmFsLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbW1vbi9wYXR0ZXJuVXRpbHMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21tb24vdHR5UGxheWVyLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hcHAvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvYXBwL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FwcC9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvZGlhbG9ncy9nZXR0ZXJzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9kaWFsb2dzL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9nZXR0ZXJzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvcmVzdEFwaVN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvdXRpbHMuanMiLCJ3ZWJwYWNrOi8vLy4vfi9ldmVudHMvZXZlbnRzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9hcHAuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9jdXJyZW50U2Vzc2lvbi9hY3RpdmVTZXNzaW9uLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvY3VycmVudFNlc3Npb24vZXZlbnRTdHJlYW1lci5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2N1cnJlbnRTZXNzaW9uL21haW4uanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9jdXJyZW50U2Vzc2lvbi9zZXNzaW9uUGxheWVyLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvaW5kZXguanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9sb2dpbi5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL25hdkxlZnRCYXIuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9uZXdVc2VyLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvc2VsZWN0Tm9kZURpYWxvZy5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL3Nlc3Npb25zL21haW4uanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvaW5kZXguanN4Iiwid2VicGFjazovLy9leHRlcm5hbCBcIlRlcm1pbmFsXCIiLCJ3ZWJwYWNrOi8vL2V4dGVybmFsIFwiX1wiIl0sIm5hbWVzIjpbXSwibWFwcGluZ3MiOiI7Ozs7Ozs7Ozs7Ozs7Ozs7O3NDQUF3QixFQUFZOztBQUVwQyxLQUFNLE9BQU8sR0FBRyx1QkFBWTtBQUMxQixRQUFLLEVBQUUsSUFBSTtFQUNaLENBQUM7O0FBRUYsT0FBTSxDQUFDLE9BQU8sR0FBRyxPQUFPLENBQUM7O3NCQUVWLE9BQU87Ozs7Ozs7Ozs7OztnQkNSQSxtQkFBTyxDQUFDLEdBQXlCLENBQUM7O0tBQW5ELGFBQWEsWUFBYixhQUFhOztBQUVsQixLQUFJLEdBQUcsR0FBRzs7QUFFUixVQUFPLEVBQUUsTUFBTSxDQUFDLFFBQVEsQ0FBQyxNQUFNOztBQUUvQixVQUFPLEVBQUUsaUVBQWlFOztBQUUxRSxNQUFHLEVBQUU7QUFDSCxtQkFBYyxFQUFDLDJCQUEyQjtBQUMxQyxjQUFTLEVBQUUsa0NBQWtDO0FBQzdDLGdCQUFXLEVBQUUscUJBQXFCO0FBQ2xDLG9CQUFlLEVBQUUsMENBQTBDO0FBQzNELGVBQVUsRUFBRSx1Q0FBdUM7QUFDbkQsbUJBQWMsRUFBRSxrQkFBa0I7QUFDbEMsaUJBQVksRUFBRSx1RUFBdUU7QUFDckYsMEJBQXFCLEVBQUUsc0RBQXNEOztBQUU3RSw0QkFBdUIsRUFBRSxpQ0FBQyxJQUFpQixFQUFHO1dBQW5CLEdBQUcsR0FBSixJQUFpQixDQUFoQixHQUFHO1dBQUUsS0FBSyxHQUFYLElBQWlCLENBQVgsS0FBSztXQUFFLEdBQUcsR0FBaEIsSUFBaUIsQ0FBSixHQUFHOztBQUN4QyxjQUFPLGFBQWEsQ0FBQyxHQUFHLENBQUMsR0FBRyxDQUFDLFlBQVksRUFBRSxFQUFDLEdBQUcsRUFBSCxHQUFHLEVBQUUsS0FBSyxFQUFMLEtBQUssRUFBRSxHQUFHLEVBQUgsR0FBRyxFQUFDLENBQUMsQ0FBQztNQUMvRDs7QUFFRCw2QkFBd0IsRUFBRSxrQ0FBQyxHQUFHLEVBQUc7QUFDL0IsY0FBTyxhQUFhLENBQUMsR0FBRyxDQUFDLEdBQUcsQ0FBQyxxQkFBcUIsRUFBRSxFQUFDLEdBQUcsRUFBSCxHQUFHLEVBQUMsQ0FBQyxDQUFDO01BQzVEOztBQUVELHdCQUFtQixFQUFFLDZDQUFrQjtBQUNyQyxXQUFJLE1BQU0sR0FBRztBQUNYLGNBQUssRUFBRSxJQUFJLElBQUksRUFBRSxDQUFDLFdBQVcsRUFBRTtBQUMvQixjQUFLLEVBQUUsQ0FBQyxDQUFDO1FBQ1YsQ0FBQzs7QUFFRixXQUFJLElBQUksR0FBRyxJQUFJLENBQUMsU0FBUyxDQUFDLE1BQU0sQ0FBQyxDQUFDO0FBQ2xDLFdBQUksV0FBVyxHQUFHLE1BQU0sQ0FBQyxTQUFTLENBQUMsSUFBSSxDQUFDLENBQUM7O0FBRXpDLHFFQUE0RCxXQUFXLENBQUc7TUFDM0U7O0FBRUQsdUJBQWtCLEVBQUUsNEJBQUMsR0FBRyxFQUFHO0FBQ3pCLGNBQU8sYUFBYSxDQUFDLEdBQUcsQ0FBQyxHQUFHLENBQUMsZUFBZSxFQUFFLEVBQUMsR0FBRyxFQUFILEdBQUcsRUFBQyxDQUFDLENBQUM7TUFDdEQ7O0FBRUQsMEJBQXFCLEVBQUUsK0JBQUMsR0FBRyxFQUFJO0FBQzdCLGNBQU8sYUFBYSxDQUFDLEdBQUcsQ0FBQyxHQUFHLENBQUMsZUFBZSxFQUFFLEVBQUMsR0FBRyxFQUFILEdBQUcsRUFBQyxDQUFDLENBQUM7TUFDdEQ7O0FBRUQsaUJBQVksRUFBRSxzQkFBQyxXQUFXLEVBQUs7QUFDN0IsY0FBTyxhQUFhLENBQUMsR0FBRyxDQUFDLEdBQUcsQ0FBQyxVQUFVLEVBQUUsRUFBQyxXQUFXLEVBQVgsV0FBVyxFQUFDLENBQUMsQ0FBQztNQUN6RDs7QUFFRCwwQkFBcUIsRUFBRSwrQkFBQyxLQUFLLEVBQUUsR0FBRyxFQUFLO0FBQ3JDLFdBQUksUUFBUSxHQUFHLGFBQWEsRUFBRSxDQUFDO0FBQy9CLGNBQVUsUUFBUSw0Q0FBdUMsR0FBRyxvQ0FBK0IsS0FBSyxDQUFHO01BQ3BHOztBQUVELGtCQUFhLEVBQUUsdUJBQUMsS0FBeUMsRUFBSztXQUE3QyxLQUFLLEdBQU4sS0FBeUMsQ0FBeEMsS0FBSztXQUFFLFFBQVEsR0FBaEIsS0FBeUMsQ0FBakMsUUFBUTtXQUFFLEtBQUssR0FBdkIsS0FBeUMsQ0FBdkIsS0FBSztXQUFFLEdBQUcsR0FBNUIsS0FBeUMsQ0FBaEIsR0FBRztXQUFFLElBQUksR0FBbEMsS0FBeUMsQ0FBWCxJQUFJO1dBQUUsSUFBSSxHQUF4QyxLQUF5QyxDQUFMLElBQUk7O0FBQ3RELFdBQUksTUFBTSxHQUFHO0FBQ1gsa0JBQVMsRUFBRSxRQUFRO0FBQ25CLGNBQUssRUFBTCxLQUFLO0FBQ0wsWUFBRyxFQUFILEdBQUc7QUFDSCxhQUFJLEVBQUU7QUFDSixZQUFDLEVBQUUsSUFBSTtBQUNQLFlBQUMsRUFBRSxJQUFJO1VBQ1I7UUFDRjs7QUFFRCxXQUFJLElBQUksR0FBRyxJQUFJLENBQUMsU0FBUyxDQUFDLE1BQU0sQ0FBQyxDQUFDO0FBQ2xDLFdBQUksV0FBVyxHQUFHLE1BQU0sQ0FBQyxTQUFTLENBQUMsSUFBSSxDQUFDLENBQUM7QUFDekMsV0FBSSxRQUFRLEdBQUcsYUFBYSxFQUFFLENBQUM7QUFDL0IsY0FBVSxRQUFRLHdEQUFtRCxLQUFLLGdCQUFXLFdBQVcsQ0FBRztNQUNwRztJQUNGOztBQUVELFNBQU0sRUFBRTtBQUNOLFFBQUcsRUFBRSxNQUFNO0FBQ1gsV0FBTSxFQUFFLGFBQWE7QUFDckIsVUFBSyxFQUFFLFlBQVk7QUFDbkIsVUFBSyxFQUFFLFlBQVk7QUFDbkIsa0JBQWEsRUFBRSxvQkFBb0I7QUFDbkMsWUFBTyxFQUFFLDJCQUEyQjtBQUNwQyxhQUFRLEVBQUUsZUFBZTtBQUN6QixpQkFBWSxFQUFFLGVBQWU7SUFDOUI7O0FBRUQsMkJBQXdCLG9DQUFDLEdBQUcsRUFBQztBQUMzQixZQUFPLGFBQWEsQ0FBQyxHQUFHLENBQUMsTUFBTSxDQUFDLGFBQWEsRUFBRSxFQUFDLEdBQUcsRUFBSCxHQUFHLEVBQUMsQ0FBQyxDQUFDO0lBQ3ZEO0VBQ0Y7O3NCQUVjLEdBQUc7O0FBRWxCLFVBQVMsYUFBYSxHQUFFO0FBQ3RCLE9BQUksTUFBTSxHQUFHLFFBQVEsQ0FBQyxRQUFRLElBQUksUUFBUSxHQUFDLFFBQVEsR0FBQyxPQUFPLENBQUM7QUFDNUQsT0FBSSxRQUFRLEdBQUcsUUFBUSxDQUFDLFFBQVEsSUFBRSxRQUFRLENBQUMsSUFBSSxHQUFHLEdBQUcsR0FBQyxRQUFRLENBQUMsSUFBSSxHQUFFLEVBQUUsQ0FBQyxDQUFDO0FBQ3pFLGVBQVUsTUFBTSxHQUFHLFFBQVEsQ0FBRztFQUMvQjs7Ozs7Ozs7QUMvRkQ7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBLDhCQUE2QixzQkFBc0I7QUFDbkQ7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0EsZUFBYztBQUNkLGVBQWM7QUFDZDtBQUNBLFlBQVcsT0FBTztBQUNsQixhQUFZO0FBQ1o7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBOztBQUVBOzs7Ozs7Ozs7O2dCQ3BEOEMsbUJBQU8sQ0FBQyxFQUFjLENBQUM7O0tBQS9ELGNBQWMsWUFBZCxjQUFjO0tBQUUsbUJBQW1CLFlBQW5CLG1CQUFtQjs7QUFFekMsS0FBTSxhQUFhLEdBQUcsVUFBVSxDQUFDOztBQUVqQyxLQUFJLFFBQVEsR0FBRyxtQkFBbUIsRUFBRSxDQUFDOztBQUVyQyxLQUFJLE9BQU8sR0FBRzs7QUFFWixPQUFJLGtCQUF3QjtTQUF2QixPQUFPLHlEQUFDLGNBQWM7O0FBQ3pCLGFBQVEsR0FBRyxPQUFPLENBQUM7SUFDcEI7O0FBRUQsYUFBVSx3QkFBRTtBQUNWLFlBQU8sUUFBUSxDQUFDO0lBQ2pCOztBQUVELGNBQVcsdUJBQUMsUUFBUSxFQUFDO0FBQ25CLGlCQUFZLENBQUMsT0FBTyxDQUFDLGFBQWEsRUFBRSxJQUFJLENBQUMsU0FBUyxDQUFDLFFBQVEsQ0FBQyxDQUFDLENBQUM7SUFDL0Q7O0FBRUQsY0FBVyx5QkFBRTtBQUNYLFNBQUksSUFBSSxHQUFHLFlBQVksQ0FBQyxPQUFPLENBQUMsYUFBYSxDQUFDLENBQUM7QUFDL0MsU0FBRyxJQUFJLEVBQUM7QUFDTixjQUFPLElBQUksQ0FBQyxLQUFLLENBQUMsSUFBSSxDQUFDLENBQUM7TUFDekI7O0FBRUQsWUFBTyxFQUFFLENBQUM7SUFDWDs7QUFFRCxRQUFLLG1CQUFFO0FBQ0wsaUJBQVksQ0FBQyxLQUFLLEVBQUU7SUFDckI7O0VBRUY7O0FBRUQsT0FBTSxDQUFDLE9BQU8sR0FBRyxPQUFPLEM7Ozs7Ozs7OztBQ25DeEIsS0FBSSxDQUFDLEdBQUcsbUJBQU8sQ0FBQyxFQUFRLENBQUMsQ0FBQztBQUMxQixLQUFJLE9BQU8sR0FBRyxtQkFBTyxDQUFDLEVBQWEsQ0FBQyxDQUFDOztBQUVyQyxLQUFNLEdBQUcsR0FBRzs7QUFFVixNQUFHLGVBQUMsSUFBSSxFQUFFLElBQUksRUFBRSxTQUFTLEVBQUM7QUFDeEIsWUFBTyxHQUFHLENBQUMsSUFBSSxDQUFDLEVBQUMsR0FBRyxFQUFFLElBQUksRUFBRSxJQUFJLEVBQUUsSUFBSSxDQUFDLFNBQVMsQ0FBQyxJQUFJLENBQUMsRUFBRSxJQUFJLEVBQUUsS0FBSyxFQUFDLEVBQUUsU0FBUyxDQUFDLENBQUM7SUFDbEY7O0FBRUQsT0FBSSxnQkFBQyxJQUFJLEVBQUUsSUFBSSxFQUFFLFNBQVMsRUFBQztBQUN6QixZQUFPLEdBQUcsQ0FBQyxJQUFJLENBQUMsRUFBQyxHQUFHLEVBQUUsSUFBSSxFQUFFLElBQUksRUFBRSxJQUFJLENBQUMsU0FBUyxDQUFDLElBQUksQ0FBQyxFQUFFLElBQUksRUFBRSxNQUFNLEVBQUMsRUFBRSxTQUFTLENBQUMsQ0FBQztJQUNuRjs7QUFFRCxNQUFHLGVBQUMsSUFBSSxFQUFDO0FBQ1AsWUFBTyxHQUFHLENBQUMsSUFBSSxDQUFDLEVBQUMsR0FBRyxFQUFFLElBQUksRUFBQyxDQUFDLENBQUM7SUFDOUI7O0FBRUQsT0FBSSxnQkFBQyxHQUFHLEVBQW1CO1NBQWpCLFNBQVMseURBQUcsSUFBSTs7QUFDeEIsU0FBSSxVQUFVLEdBQUc7QUFDZixXQUFJLEVBQUUsS0FBSztBQUNYLGVBQVEsRUFBRSxNQUFNO0FBQ2hCLGlCQUFVLEVBQUUsb0JBQVMsR0FBRyxFQUFFO0FBQ3hCLGFBQUcsU0FBUyxFQUFDO3NDQUNLLE9BQU8sQ0FBQyxXQUFXLEVBQUU7O2VBQS9CLEtBQUssd0JBQUwsS0FBSzs7QUFDWCxjQUFHLENBQUMsZ0JBQWdCLENBQUMsZUFBZSxFQUFDLFNBQVMsR0FBRyxLQUFLLENBQUMsQ0FBQztVQUN6RDtRQUNEO01BQ0g7O0FBRUQsWUFBTyxDQUFDLENBQUMsSUFBSSxDQUFDLENBQUMsQ0FBQyxNQUFNLENBQUMsRUFBRSxFQUFFLFVBQVUsRUFBRSxHQUFHLENBQUMsQ0FBQyxDQUFDO0lBQzlDO0VBQ0Y7O0FBRUQsT0FBTSxDQUFDLE9BQU8sR0FBRyxHQUFHLEM7Ozs7Ozs7QUNqQ3BCLHlCOzs7Ozs7Ozs7O0FDQUEsS0FBSSxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxDQUFhLENBQUMsQ0FBQztBQUNyQyxLQUFJLE9BQU8sR0FBRyxtQkFBTyxDQUFDLEVBQWEsQ0FBQyxDQUFDOztnQkFDeEIsbUJBQU8sQ0FBQyxHQUFXLENBQUM7O0tBQTVCLElBQUksWUFBSixJQUFJOztBQUNULEtBQUksR0FBRyxHQUFHLG1CQUFPLENBQUMsRUFBa0IsQ0FBQyxDQUFDO0FBQ3RDLEtBQUksR0FBRyxHQUFHLG1CQUFPLENBQUMsRUFBWSxDQUFDLENBQUM7QUFDaEMsS0FBSSxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxFQUFXLENBQUMsQ0FBQztBQUNuQyxLQUFJLGFBQWEsR0FBRyxtQkFBTyxDQUFDLEdBQWUsQ0FBQyxDQUFDOztpQkFFc0IsbUJBQU8sQ0FBQyxFQUFlLENBQUM7O0tBQXJGLGNBQWMsYUFBZCxjQUFjO0tBQUUsZUFBZSxhQUFmLGVBQWU7S0FBRSx1QkFBdUIsYUFBdkIsdUJBQXVCOztBQUU5RCxLQUFJLE9BQU8sR0FBRzs7QUFFWixlQUFZLHdCQUFDLFFBQVEsRUFBRSxLQUFLLEVBQUM7QUFDM0IsWUFBTyxDQUFDLFFBQVEsQ0FBQyx1QkFBdUIsRUFBRTtBQUN4QyxlQUFRLEVBQVIsUUFBUTtBQUNSLFlBQUssRUFBTCxLQUFLO01BQ04sQ0FBQyxDQUFDO0lBQ0o7O0FBRUQsUUFBSyxtQkFBRTs2QkFDZ0IsT0FBTyxDQUFDLFFBQVEsQ0FBQyxPQUFPLENBQUMsYUFBYSxDQUFDOztTQUF2RCxZQUFZLHFCQUFaLFlBQVk7O0FBRWpCLFlBQU8sQ0FBQyxRQUFRLENBQUMsZUFBZSxDQUFDLENBQUM7O0FBRWxDLFNBQUcsWUFBWSxFQUFDO0FBQ2QsY0FBTyxDQUFDLFVBQVUsRUFBRSxDQUFDLElBQUksQ0FBQyxHQUFHLENBQUMsTUFBTSxDQUFDLEtBQUssQ0FBQyxDQUFDO01BQzdDLE1BQUk7QUFDSCxjQUFPLENBQUMsVUFBVSxFQUFFLENBQUMsSUFBSSxDQUFDLEdBQUcsQ0FBQyxNQUFNLENBQUMsUUFBUSxDQUFDLENBQUM7TUFDaEQ7SUFDRjs7QUFFRCxTQUFNLGtCQUFDLENBQUMsRUFBRSxDQUFDLEVBQUM7O0FBRVYsTUFBQyxHQUFHLENBQUMsR0FBRyxDQUFDLEdBQUcsQ0FBQyxHQUFHLENBQUMsQ0FBQztBQUNsQixNQUFDLEdBQUcsQ0FBQyxHQUFHLENBQUMsR0FBRyxDQUFDLEdBQUcsQ0FBQyxDQUFDOztBQUVsQixTQUFJLE9BQU8sR0FBRyxFQUFFLGVBQWUsRUFBRSxFQUFFLENBQUMsRUFBRCxDQUFDLEVBQUUsQ0FBQyxFQUFELENBQUMsRUFBRSxFQUFFLENBQUM7OzhCQUNoQyxPQUFPLENBQUMsUUFBUSxDQUFDLE9BQU8sQ0FBQyxhQUFhLENBQUM7O1NBQTlDLEdBQUcsc0JBQUgsR0FBRzs7QUFFUixRQUFHLENBQUMsR0FBRyxDQUFDLEdBQUcsQ0FBQyxHQUFHLENBQUMscUJBQXFCLENBQUMsR0FBRyxDQUFDLEVBQUUsT0FBTyxDQUFDLENBQ2pELElBQUksQ0FBQyxZQUFJO0FBQ1IsY0FBTyxDQUFDLEdBQUcsb0JBQWtCLENBQUMsZUFBVSxDQUFDLFdBQVEsQ0FBQztNQUNuRCxDQUFDLENBQ0QsSUFBSSxDQUFDLFlBQUk7QUFDUixjQUFPLENBQUMsR0FBRyw4QkFBNEIsQ0FBQyxlQUFVLENBQUMsQ0FBRyxDQUFDO01BQzFELENBQUM7SUFDSDs7QUFFRCxjQUFXLHVCQUFDLEdBQUcsRUFBQztBQUNkLGtCQUFhLENBQUMsT0FBTyxDQUFDLFlBQVksQ0FBQyxHQUFHLENBQUMsQ0FDcEMsSUFBSSxDQUFDLFlBQUk7QUFDUixXQUFJLEtBQUssR0FBRyxPQUFPLENBQUMsUUFBUSxDQUFDLGFBQWEsQ0FBQyxPQUFPLENBQUMsZUFBZSxDQUFDLEdBQUcsQ0FBQyxDQUFDLENBQUM7V0FDbkUsUUFBUSxHQUFZLEtBQUssQ0FBekIsUUFBUTtXQUFFLEtBQUssR0FBSyxLQUFLLENBQWYsS0FBSzs7QUFDckIsY0FBTyxDQUFDLFFBQVEsQ0FBQyxjQUFjLEVBQUU7QUFDN0IsaUJBQVEsRUFBUixRQUFRO0FBQ1IsY0FBSyxFQUFMLEtBQUs7QUFDTCxZQUFHLEVBQUgsR0FBRztBQUNILHFCQUFZLEVBQUUsS0FBSztRQUNwQixDQUFDLENBQUM7TUFDTixDQUFDLENBQ0QsSUFBSSxDQUFDLFlBQUk7QUFDUixjQUFPLENBQUMsVUFBVSxFQUFFLENBQUMsSUFBSSxDQUFDLEdBQUcsQ0FBQyxNQUFNLENBQUMsWUFBWSxDQUFDLENBQUM7TUFDcEQsQ0FBQztJQUNMOztBQUVELG1CQUFnQiw0QkFBQyxRQUFRLEVBQUUsS0FBSyxFQUFDO0FBQy9CLFNBQUksR0FBRyxHQUFHLElBQUksRUFBRSxDQUFDO0FBQ2pCLFNBQUksUUFBUSxHQUFHLEdBQUcsQ0FBQyx3QkFBd0IsQ0FBQyxHQUFHLENBQUMsQ0FBQztBQUNqRCxTQUFJLE9BQU8sR0FBRyxPQUFPLENBQUMsVUFBVSxFQUFFLENBQUM7O0FBRW5DLFlBQU8sQ0FBQyxRQUFRLENBQUMsY0FBYyxFQUFFO0FBQy9CLGVBQVEsRUFBUixRQUFRO0FBQ1IsWUFBSyxFQUFMLEtBQUs7QUFDTCxVQUFHLEVBQUgsR0FBRztBQUNILG1CQUFZLEVBQUUsSUFBSTtNQUNuQixDQUFDLENBQUM7O0FBRUgsWUFBTyxDQUFDLElBQUksQ0FBQyxRQUFRLENBQUMsQ0FBQztJQUN4Qjs7RUFFRjs7c0JBRWMsT0FBTzs7Ozs7Ozs7OztBQ2xGdEIsT0FBTSxDQUFDLE9BQU8sQ0FBQyxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxFQUFXLENBQUMsQ0FBQztBQUM5QyxPQUFNLENBQUMsT0FBTyxDQUFDLE9BQU8sR0FBRyxtQkFBTyxDQUFDLEVBQVcsQ0FBQyxDQUFDO0FBQzlDLE9BQU0sQ0FBQyxPQUFPLENBQUMsZUFBZSxHQUFHLG1CQUFPLENBQUMsRUFBbUIsQ0FBQyxDOzs7Ozs7Ozs7O0FDRjdELEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsQ0FBYSxDQUFDLENBQUM7O2dCQUNpQyxtQkFBTyxDQUFDLEVBQWUsQ0FBQzs7S0FBeEYsNEJBQTRCLFlBQTVCLDRCQUE0QjtLQUFFLDZCQUE2QixZQUE3Qiw2QkFBNkI7O0FBRWpFLEtBQUksT0FBTyxHQUFHO0FBQ1osdUJBQW9CLGtDQUFFO0FBQ3BCLFlBQU8sQ0FBQyxRQUFRLENBQUMsNEJBQTRCLENBQUMsQ0FBQztJQUNoRDs7QUFFRCx3QkFBcUIsbUNBQUU7QUFDckIsWUFBTyxDQUFDLFFBQVEsQ0FBQyw2QkFBNkIsQ0FBQyxDQUFDO0lBQ2pEO0VBQ0Y7O3NCQUVjLE9BQU87Ozs7Ozs7Ozs7O0FDYnRCLEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsQ0FBYSxDQUFDLENBQUM7QUFDckMsS0FBSSxHQUFHLEdBQUcsbUJBQU8sQ0FBQyxFQUFrQixDQUFDLENBQUM7QUFDdEMsS0FBSSxHQUFHLEdBQUcsbUJBQU8sQ0FBQyxFQUFZLENBQUMsQ0FBQzs7Z0JBRXFCLG1CQUFPLENBQUMsR0FBZSxDQUFDOztLQUF2RSxvQkFBb0IsWUFBcEIsb0JBQW9CO0tBQUUsbUJBQW1CLFlBQW5CLG1CQUFtQjtzQkFFaEM7O0FBRWIsZUFBWSx3QkFBQyxHQUFHLEVBQUM7QUFDZixZQUFPLEdBQUcsQ0FBQyxHQUFHLENBQUMsR0FBRyxDQUFDLEdBQUcsQ0FBQyxrQkFBa0IsQ0FBQyxHQUFHLENBQUMsQ0FBQyxDQUFDLElBQUksQ0FBQyxjQUFJLEVBQUU7QUFDekQsV0FBRyxJQUFJLElBQUksSUFBSSxDQUFDLE9BQU8sRUFBQztBQUN0QixnQkFBTyxDQUFDLFFBQVEsQ0FBQyxtQkFBbUIsRUFBRSxJQUFJLENBQUMsT0FBTyxDQUFDLENBQUM7UUFDckQ7TUFDRixDQUFDLENBQUM7SUFDSjs7QUFFRCxnQkFBYSwyQkFBRTtBQUNiLFlBQU8sR0FBRyxDQUFDLEdBQUcsQ0FBQyxHQUFHLENBQUMsR0FBRyxDQUFDLG1CQUFtQixFQUFFLENBQUMsQ0FBQyxJQUFJLENBQUMsVUFBQyxJQUFJLEVBQUs7QUFDM0QsY0FBTyxDQUFDLFFBQVEsQ0FBQyxvQkFBb0IsRUFBRSxJQUFJLENBQUMsUUFBUSxDQUFDLENBQUM7TUFDdkQsQ0FBQyxDQUFDO0lBQ0o7O0FBRUQsZ0JBQWEseUJBQUMsSUFBSSxFQUFDO0FBQ2pCLFlBQU8sQ0FBQyxRQUFRLENBQUMsbUJBQW1CLEVBQUUsSUFBSSxDQUFDLENBQUM7SUFDN0M7RUFDRjs7Ozs7Ozs7Ozs7O2dCQ3pCcUIsbUJBQU8sQ0FBQyxFQUFZLENBQUM7O0tBQXJDLFdBQVcsWUFBWCxXQUFXOztBQUNqQixLQUFJLE9BQU8sR0FBRyxtQkFBTyxDQUFDLENBQWEsQ0FBQyxDQUFDO0FBQ3JDLEtBQUksR0FBRyxHQUFHLG1CQUFPLENBQUMsRUFBWSxDQUFDLENBQUM7O0FBRWhDLEtBQU0sZ0JBQWdCLEdBQUcsU0FBbkIsZ0JBQWdCLENBQUksUUFBUTtVQUFLLENBQUMsQ0FBQyxlQUFlLENBQUMsRUFBRSxVQUFDLFFBQVEsRUFBSTtBQUN0RSxZQUFPLFFBQVEsQ0FBQyxRQUFRLEVBQUUsQ0FBQyxNQUFNLENBQUMsY0FBSSxFQUFFO0FBQ3RDLFdBQUksT0FBTyxHQUFHLElBQUksQ0FBQyxHQUFHLENBQUMsU0FBUyxDQUFDLElBQUksV0FBVyxDQUFDLEVBQUUsQ0FBQyxDQUFDO0FBQ3JELFdBQUksU0FBUyxHQUFHLE9BQU8sQ0FBQyxJQUFJLENBQUMsZUFBSztnQkFBRyxLQUFLLENBQUMsR0FBRyxDQUFDLFdBQVcsQ0FBQyxLQUFLLFFBQVE7UUFBQSxDQUFDLENBQUM7QUFDMUUsY0FBTyxTQUFTLENBQUM7TUFDbEIsQ0FBQyxDQUFDLE1BQU0sRUFBRSxDQUFDO0lBQ2IsQ0FBQztFQUFBOztBQUVGLEtBQU0sWUFBWSxHQUFHLENBQUMsQ0FBQyxlQUFlLENBQUMsRUFBRSxVQUFDLFFBQVEsRUFBSTtBQUNwRCxVQUFPLFFBQVEsQ0FBQyxRQUFRLEVBQUUsQ0FBQyxHQUFHLENBQUMsVUFBVSxDQUFDLENBQUMsSUFBSSxFQUFFLENBQUM7RUFDbkQsQ0FBQyxDQUFDOztBQUVILEtBQU0sZUFBZSxHQUFHLFNBQWxCLGVBQWUsQ0FBSSxHQUFHO1VBQUksQ0FBQyxDQUFDLGVBQWUsRUFBRSxHQUFHLENBQUMsRUFBRSxVQUFDLE9BQU8sRUFBRztBQUNsRSxTQUFHLENBQUMsT0FBTyxFQUFDO0FBQ1YsY0FBTyxJQUFJLENBQUM7TUFDYjs7QUFFRCxZQUFPLFVBQVUsQ0FBQyxPQUFPLENBQUMsQ0FBQztJQUM1QixDQUFDO0VBQUEsQ0FBQzs7QUFFSCxLQUFNLGtCQUFrQixHQUFHLFNBQXJCLGtCQUFrQixDQUFJLEdBQUc7VUFDOUIsQ0FBQyxDQUFDLGVBQWUsRUFBRSxHQUFHLEVBQUUsU0FBUyxDQUFDLEVBQUUsVUFBQyxPQUFPLEVBQUk7O0FBRS9DLFNBQUcsQ0FBQyxPQUFPLEVBQUM7QUFDVixjQUFPLEVBQUUsQ0FBQztNQUNYOztBQUVELFNBQUksaUJBQWlCLEdBQUcsaUJBQWlCLENBQUMsT0FBTyxDQUFDLENBQUMsR0FBRyxDQUFDLE1BQU0sQ0FBQyxDQUFDOztBQUUvRCxZQUFPLE9BQU8sQ0FBQyxHQUFHLENBQUMsY0FBSSxFQUFFO0FBQ3ZCLFdBQUksSUFBSSxHQUFHLElBQUksQ0FBQyxHQUFHLENBQUMsTUFBTSxDQUFDLENBQUM7QUFDNUIsY0FBTztBQUNMLGFBQUksRUFBRSxJQUFJLENBQUMsR0FBRyxDQUFDLE1BQU0sQ0FBQztBQUN0QixpQkFBUSxFQUFFLElBQUksQ0FBQyxHQUFHLENBQUMsYUFBYSxDQUFDO0FBQ2pDLGlCQUFRLEVBQUUsSUFBSSxDQUFDLEdBQUcsQ0FBQyxXQUFXLENBQUM7QUFDL0IsaUJBQVEsRUFBRSxpQkFBaUIsS0FBSyxJQUFJO1FBQ3JDO01BQ0YsQ0FBQyxDQUFDLElBQUksRUFBRSxDQUFDO0lBQ1gsQ0FBQztFQUFBLENBQUM7O0FBRUgsVUFBUyxpQkFBaUIsQ0FBQyxPQUFPLEVBQUM7QUFDakMsVUFBTyxPQUFPLENBQUMsTUFBTSxDQUFDLGNBQUk7WUFBRyxJQUFJLElBQUksQ0FBQyxJQUFJLENBQUMsR0FBRyxDQUFDLFlBQVksQ0FBQyxDQUFDO0lBQUEsQ0FBQyxDQUFDLEtBQUssRUFBRSxDQUFDO0VBQ3hFOztBQUVELFVBQVMsVUFBVSxDQUFDLE9BQU8sRUFBQztBQUMxQixPQUFJLEdBQUcsR0FBRyxPQUFPLENBQUMsR0FBRyxDQUFDLElBQUksQ0FBQyxDQUFDO0FBQzVCLE9BQUksUUFBUSxFQUFFLFFBQVEsQ0FBQztBQUN2QixPQUFJLE9BQU8sR0FBRyxPQUFPLENBQUMsUUFBUSxDQUFDLGtCQUFrQixDQUFDLEdBQUcsQ0FBQyxDQUFDLENBQUM7O0FBRXhELE9BQUcsT0FBTyxDQUFDLE1BQU0sR0FBRyxDQUFDLEVBQUM7QUFDcEIsYUFBUSxHQUFHLE9BQU8sQ0FBQyxDQUFDLENBQUMsQ0FBQyxRQUFRLENBQUM7QUFDL0IsYUFBUSxHQUFHLE9BQU8sQ0FBQyxDQUFDLENBQUMsQ0FBQyxRQUFRLENBQUM7SUFDaEM7O0FBRUQsVUFBTztBQUNMLFFBQUcsRUFBRSxHQUFHO0FBQ1IsZUFBVSxFQUFFLEdBQUcsQ0FBQyx3QkFBd0IsQ0FBQyxHQUFHLENBQUM7QUFDN0MsYUFBUSxFQUFSLFFBQVE7QUFDUixhQUFRLEVBQVIsUUFBUTtBQUNSLFdBQU0sRUFBRSxPQUFPLENBQUMsR0FBRyxDQUFDLFFBQVEsQ0FBQztBQUM3QixZQUFPLEVBQUUsSUFBSSxJQUFJLENBQUMsT0FBTyxDQUFDLEdBQUcsQ0FBQyxTQUFTLENBQUMsQ0FBQztBQUN6QyxlQUFVLEVBQUUsSUFBSSxJQUFJLENBQUMsT0FBTyxDQUFDLEdBQUcsQ0FBQyxhQUFhLENBQUMsQ0FBQztBQUNoRCxVQUFLLEVBQUUsT0FBTyxDQUFDLEdBQUcsQ0FBQyxPQUFPLENBQUM7QUFDM0IsWUFBTyxFQUFFLE9BQU87QUFDaEIsU0FBSSxFQUFFLE9BQU8sQ0FBQyxLQUFLLENBQUMsQ0FBQyxpQkFBaUIsRUFBRSxHQUFHLENBQUMsQ0FBQztBQUM3QyxTQUFJLEVBQUUsT0FBTyxDQUFDLEtBQUssQ0FBQyxDQUFDLGlCQUFpQixFQUFFLEdBQUcsQ0FBQyxDQUFDO0lBQzlDO0VBQ0Y7O3NCQUVjO0FBQ2IscUJBQWtCLEVBQWxCLGtCQUFrQjtBQUNsQixtQkFBZ0IsRUFBaEIsZ0JBQWdCO0FBQ2hCLGVBQVksRUFBWixZQUFZO0FBQ1osa0JBQWUsRUFBZixlQUFlO0FBQ2YsYUFBVSxFQUFWLFVBQVU7RUFDWDs7Ozs7Ozs7Ozs7QUMvRUQsS0FBTSxJQUFJLEdBQUcsQ0FBRSxDQUFDLFdBQVcsQ0FBQyxFQUFFLFVBQUMsV0FBVyxFQUFLO0FBQzNDLE9BQUcsQ0FBQyxXQUFXLEVBQUM7QUFDZCxZQUFPLElBQUksQ0FBQztJQUNiOztBQUVELE9BQUksSUFBSSxHQUFHLFdBQVcsQ0FBQyxHQUFHLENBQUMsTUFBTSxDQUFDLElBQUksRUFBRSxDQUFDO0FBQ3pDLE9BQUksZ0JBQWdCLEdBQUcsSUFBSSxDQUFDLENBQUMsQ0FBQyxJQUFJLEVBQUUsQ0FBQzs7QUFFckMsVUFBTztBQUNMLFNBQUksRUFBSixJQUFJO0FBQ0oscUJBQWdCLEVBQWhCLGdCQUFnQjtBQUNoQixXQUFNLEVBQUUsV0FBVyxDQUFDLEdBQUcsQ0FBQyxnQkFBZ0IsQ0FBQyxDQUFDLElBQUksRUFBRTtJQUNqRDtFQUNGLENBQ0YsQ0FBQzs7c0JBRWE7QUFDYixPQUFJLEVBQUosSUFBSTtFQUNMOzs7Ozs7Ozs7O0FDbEJELEtBQUksR0FBRyxHQUFHLG1CQUFPLENBQUMsRUFBZ0IsQ0FBQyxDQUFDO0FBQ3BDLEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsRUFBVyxDQUFDLENBQUM7QUFDbkMsS0FBSSxHQUFHLEdBQUcsbUJBQU8sQ0FBQyxFQUFZLENBQUMsQ0FBQztBQUNoQyxLQUFJLENBQUMsR0FBRyxtQkFBTyxDQUFDLEVBQVEsQ0FBQyxDQUFDOztBQUUxQixLQUFNLFdBQVcsR0FBRyxLQUFLLEdBQUcsQ0FBQyxDQUFDOztBQUU5QixLQUFJLG1CQUFtQixHQUFHLElBQUksQ0FBQzs7QUFFL0IsS0FBSSxJQUFJLEdBQUc7O0FBRVQsU0FBTSxrQkFBQyxJQUFJLEVBQUUsUUFBUSxFQUFFLEtBQUssRUFBRSxXQUFXLEVBQUM7QUFDeEMsU0FBSSxJQUFJLEdBQUcsRUFBQyxJQUFJLEVBQUUsSUFBSSxFQUFFLElBQUksRUFBRSxRQUFRLEVBQUUsbUJBQW1CLEVBQUUsS0FBSyxFQUFFLFlBQVksRUFBRSxXQUFXLEVBQUMsQ0FBQztBQUMvRixZQUFPLEdBQUcsQ0FBQyxJQUFJLENBQUMsR0FBRyxDQUFDLEdBQUcsQ0FBQyxjQUFjLEVBQUUsSUFBSSxDQUFDLENBQzFDLElBQUksQ0FBQyxVQUFDLElBQUksRUFBRztBQUNaLGNBQU8sQ0FBQyxXQUFXLENBQUMsSUFBSSxDQUFDLENBQUM7QUFDMUIsV0FBSSxDQUFDLG9CQUFvQixFQUFFLENBQUM7QUFDNUIsY0FBTyxJQUFJLENBQUM7TUFDYixDQUFDLENBQUM7SUFDTjs7QUFFRCxRQUFLLGlCQUFDLElBQUksRUFBRSxRQUFRLEVBQUUsS0FBSyxFQUFDO0FBQzFCLFNBQUksQ0FBQyxtQkFBbUIsRUFBRSxDQUFDO0FBQzNCLFlBQU8sSUFBSSxDQUFDLE1BQU0sQ0FBQyxJQUFJLEVBQUUsUUFBUSxFQUFFLEtBQUssQ0FBQyxDQUFDLElBQUksQ0FBQyxJQUFJLENBQUMsb0JBQW9CLENBQUMsQ0FBQztJQUMzRTs7QUFFRCxhQUFVLHdCQUFFO0FBQ1YsU0FBSSxRQUFRLEdBQUcsT0FBTyxDQUFDLFdBQVcsRUFBRSxDQUFDO0FBQ3JDLFNBQUcsUUFBUSxDQUFDLEtBQUssRUFBQzs7QUFFaEIsV0FBRyxJQUFJLENBQUMsdUJBQXVCLEVBQUUsS0FBSyxJQUFJLEVBQUM7QUFDekMsZ0JBQU8sSUFBSSxDQUFDLGFBQWEsRUFBRSxDQUFDLElBQUksQ0FBQyxJQUFJLENBQUMsb0JBQW9CLENBQUMsQ0FBQztRQUM3RDs7QUFFRCxjQUFPLENBQUMsQ0FBQyxRQUFRLEVBQUUsQ0FBQyxPQUFPLENBQUMsUUFBUSxDQUFDLENBQUM7TUFDdkM7O0FBRUQsWUFBTyxDQUFDLENBQUMsUUFBUSxFQUFFLENBQUMsTUFBTSxFQUFFLENBQUM7SUFDOUI7O0FBRUQsU0FBTSxvQkFBRTtBQUNOLFNBQUksQ0FBQyxtQkFBbUIsRUFBRSxDQUFDO0FBQzNCLFlBQU8sQ0FBQyxLQUFLLEVBQUUsQ0FBQztBQUNoQixZQUFPLENBQUMsVUFBVSxFQUFFLENBQUMsT0FBTyxDQUFDLEVBQUMsUUFBUSxFQUFFLEdBQUcsQ0FBQyxNQUFNLENBQUMsS0FBSyxFQUFDLENBQUMsQ0FBQztJQUM1RDs7QUFFRCx1QkFBb0Isa0NBQUU7QUFDcEIsd0JBQW1CLEdBQUcsV0FBVyxDQUFDLElBQUksQ0FBQyxhQUFhLEVBQUUsV0FBVyxDQUFDLENBQUM7SUFDcEU7O0FBRUQsc0JBQW1CLGlDQUFFO0FBQ25CLGtCQUFhLENBQUMsbUJBQW1CLENBQUMsQ0FBQztBQUNuQyx3QkFBbUIsR0FBRyxJQUFJLENBQUM7SUFDNUI7O0FBRUQsMEJBQXVCLHFDQUFFO0FBQ3ZCLFlBQU8sbUJBQW1CLENBQUM7SUFDNUI7O0FBRUQsZ0JBQWEsMkJBQUU7QUFDYixZQUFPLEdBQUcsQ0FBQyxJQUFJLENBQUMsR0FBRyxDQUFDLEdBQUcsQ0FBQyxjQUFjLENBQUMsQ0FBQyxJQUFJLENBQUMsY0FBSSxFQUFFO0FBQ2pELGNBQU8sQ0FBQyxXQUFXLENBQUMsSUFBSSxDQUFDLENBQUM7QUFDMUIsY0FBTyxJQUFJLENBQUM7TUFDYixDQUFDLENBQUMsSUFBSSxDQUFDLFlBQUk7QUFDVixXQUFJLENBQUMsTUFBTSxFQUFFLENBQUM7TUFDZixDQUFDLENBQUM7SUFDSjs7QUFFRCxTQUFNLGtCQUFDLElBQUksRUFBRSxRQUFRLEVBQUUsS0FBSyxFQUFDO0FBQzNCLFNBQUksSUFBSSxHQUFHO0FBQ1QsV0FBSSxFQUFFLElBQUk7QUFDVixXQUFJLEVBQUUsUUFBUTtBQUNkLDBCQUFtQixFQUFFLEtBQUs7TUFDM0IsQ0FBQzs7QUFFRixZQUFPLEdBQUcsQ0FBQyxJQUFJLENBQUMsR0FBRyxDQUFDLEdBQUcsQ0FBQyxXQUFXLEVBQUUsSUFBSSxFQUFFLEtBQUssQ0FBQyxDQUFDLElBQUksQ0FBQyxjQUFJLEVBQUU7QUFDM0QsY0FBTyxDQUFDLFdBQVcsQ0FBQyxJQUFJLENBQUMsQ0FBQztBQUMxQixjQUFPLElBQUksQ0FBQztNQUNiLENBQUMsQ0FBQztJQUNKO0VBQ0Y7O0FBRUQsT0FBTSxDQUFDLE9BQU8sR0FBRyxJQUFJLEM7Ozs7Ozs7Ozs7Ozs7OztBQ2xGckIsS0FBSSxZQUFZLEdBQUcsbUJBQU8sQ0FBQyxHQUFRLENBQUMsQ0FBQyxZQUFZLENBQUM7QUFDbEQsS0FBSSxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxFQUFhLENBQUMsQ0FBQztBQUNyQyxLQUFJLEdBQUcsR0FBRyxtQkFBTyxDQUFDLEVBQVksQ0FBQyxDQUFDOztnQkFDaEIsbUJBQU8sQ0FBQyxFQUE2QixDQUFDOztLQUFqRCxPQUFPLFlBQVAsT0FBTzs7S0FFTixHQUFHO2FBQUgsR0FBRzs7QUFFSSxZQUZQLEdBQUcsQ0FFSyxJQUFtQyxFQUFDO1NBQW5DLFFBQVEsR0FBVCxJQUFtQyxDQUFsQyxRQUFRO1NBQUUsS0FBSyxHQUFoQixJQUFtQyxDQUF4QixLQUFLO1NBQUUsR0FBRyxHQUFyQixJQUFtQyxDQUFqQixHQUFHO1NBQUUsSUFBSSxHQUEzQixJQUFtQyxDQUFaLElBQUk7U0FBRSxJQUFJLEdBQWpDLElBQW1DLENBQU4sSUFBSTs7MkJBRnpDLEdBQUc7O0FBR0wsNkJBQU8sQ0FBQztBQUNSLFNBQUksQ0FBQyxPQUFPLEdBQUcsRUFBRSxRQUFRLEVBQVIsUUFBUSxFQUFFLEtBQUssRUFBTCxLQUFLLEVBQUUsR0FBRyxFQUFILEdBQUcsRUFBRSxJQUFJLEVBQUosSUFBSSxFQUFFLElBQUksRUFBSixJQUFJLEVBQUUsQ0FBQztBQUNwRCxTQUFJLENBQUMsTUFBTSxHQUFHLElBQUksQ0FBQztJQUNwQjs7QUFORyxNQUFHLFdBUVAsVUFBVSx5QkFBRTtBQUNWLFNBQUksQ0FBQyxNQUFNLENBQUMsS0FBSyxFQUFFLENBQUM7SUFDckI7O0FBVkcsTUFBRyxXQVlQLFNBQVMsc0JBQUMsT0FBTyxFQUFDO0FBQ2hCLFNBQUksQ0FBQyxNQUFNLENBQUMsS0FBSyxFQUFFLENBQUM7QUFDcEIsU0FBSSxDQUFDLE9BQU8sQ0FBQyxPQUFPLENBQUMsQ0FBQztJQUN2Qjs7QUFmRyxNQUFHLFdBaUJQLE9BQU8sb0JBQUMsT0FBTyxFQUFDOzs7QUFDZCxXQUFNLENBQUMsTUFBTSxDQUFDLElBQUksQ0FBQyxPQUFPLEVBQUUsT0FBTyxDQUFDLENBQUM7O2dDQUV2QixPQUFPLENBQUMsV0FBVyxFQUFFOztTQUE5QixLQUFLLHdCQUFMLEtBQUs7O0FBQ1YsU0FBSSxPQUFPLEdBQUcsR0FBRyxDQUFDLEdBQUcsQ0FBQyxhQUFhLFlBQUUsS0FBSyxFQUFMLEtBQUssSUFBSyxJQUFJLENBQUMsT0FBTyxFQUFFLENBQUM7O0FBRTlELFNBQUksQ0FBQyxNQUFNLEdBQUcsSUFBSSxTQUFTLENBQUMsT0FBTyxFQUFFLE9BQU8sQ0FBQyxDQUFDOztBQUU5QyxTQUFJLENBQUMsTUFBTSxDQUFDLE1BQU0sR0FBRyxZQUFNO0FBQ3pCLGFBQUssSUFBSSxDQUFDLE1BQU0sQ0FBQyxDQUFDO01BQ25COztBQUVELFNBQUksQ0FBQyxNQUFNLENBQUMsU0FBUyxHQUFHLFVBQUMsQ0FBQyxFQUFHO0FBQzNCLGFBQUssSUFBSSxDQUFDLE1BQU0sRUFBRSxDQUFDLENBQUMsSUFBSSxDQUFDLENBQUM7TUFDM0I7O0FBRUQsU0FBSSxDQUFDLE1BQU0sQ0FBQyxPQUFPLEdBQUcsWUFBSTtBQUN4QixhQUFLLElBQUksQ0FBQyxPQUFPLENBQUMsQ0FBQztNQUNwQjtJQUNGOztBQXBDRyxNQUFHLFdBc0NQLE1BQU0sbUJBQUMsSUFBSSxFQUFFLElBQUksRUFBQztBQUNoQixZQUFPLENBQUMsTUFBTSxDQUFDLElBQUksRUFBRSxJQUFJLENBQUMsQ0FBQztJQUM1Qjs7QUF4Q0csTUFBRyxXQTBDUCxJQUFJLGlCQUFDLElBQUksRUFBQztBQUNSLFNBQUksQ0FBQyxNQUFNLENBQUMsSUFBSSxDQUFDLElBQUksQ0FBQyxDQUFDO0lBQ3hCOztVQTVDRyxHQUFHO0lBQVMsWUFBWTs7QUErQzlCLE9BQU0sQ0FBQyxPQUFPLEdBQUcsR0FBRyxDOzs7Ozs7Ozs7Ozs7O3NDQ3BERSxFQUFXOzs7O3NCQUVsQix1QkFBVTtBQUN2QixpQkFBYyxFQUFFLElBQUk7QUFDcEIsa0JBQWUsRUFBRSxJQUFJO0FBQ3JCLDBCQUF1QixFQUFFLElBQUk7RUFDOUIsQ0FBQzs7Ozs7Ozs7Ozs7O2dCQ04yQixtQkFBTyxDQUFDLEVBQVksQ0FBQzs7S0FBNUMsS0FBSyxZQUFMLEtBQUs7S0FBRSxXQUFXLFlBQVgsV0FBVzs7aUJBQzRDLG1CQUFPLENBQUMsRUFBZSxDQUFDOztLQUF0RixjQUFjLGFBQWQsY0FBYztLQUFFLGVBQWUsYUFBZixlQUFlO0tBQUUsdUJBQXVCLGFBQXZCLHVCQUF1QjtzQkFFL0MsS0FBSyxDQUFDO0FBQ25CLGtCQUFlLDZCQUFHO0FBQ2hCLFlBQU8sV0FBVyxDQUFDLElBQUksQ0FBQyxDQUFDO0lBQzFCOztBQUVELGFBQVUsd0JBQUc7QUFDWCxTQUFJLENBQUMsRUFBRSxDQUFDLGNBQWMsRUFBRSxpQkFBaUIsQ0FBQyxDQUFDO0FBQzNDLFNBQUksQ0FBQyxFQUFFLENBQUMsZUFBZSxFQUFFLEtBQUssQ0FBQyxDQUFDO0FBQ2hDLFNBQUksQ0FBQyxFQUFFLENBQUMsdUJBQXVCLEVBQUUsWUFBWSxDQUFDLENBQUM7SUFDaEQ7RUFDRixDQUFDOztBQUVGLFVBQVMsWUFBWSxDQUFDLEtBQUssRUFBRSxJQUFpQixFQUFDO09BQWpCLFFBQVEsR0FBVCxJQUFpQixDQUFoQixRQUFRO09BQUUsS0FBSyxHQUFoQixJQUFpQixDQUFOLEtBQUs7O0FBQzNDLFVBQU8sS0FBSyxDQUFDLEdBQUcsQ0FBQyxVQUFVLEVBQUUsUUFBUSxDQUFDLENBQ3pCLEdBQUcsQ0FBQyxPQUFPLEVBQUUsS0FBSyxDQUFDLENBQUM7RUFDbEM7O0FBRUQsVUFBUyxLQUFLLEdBQUU7QUFDZCxVQUFPLFdBQVcsQ0FBQyxJQUFJLENBQUMsQ0FBQztFQUMxQjs7QUFFRCxVQUFTLGlCQUFpQixDQUFDLEtBQUssRUFBRSxLQUFvQyxFQUFFO09BQXJDLFFBQVEsR0FBVCxLQUFvQyxDQUFuQyxRQUFRO09BQUUsS0FBSyxHQUFoQixLQUFvQyxDQUF6QixLQUFLO09BQUUsR0FBRyxHQUFyQixLQUFvQyxDQUFsQixHQUFHO09BQUUsWUFBWSxHQUFuQyxLQUFvQyxDQUFiLFlBQVk7O0FBQ25FLFVBQU8sV0FBVyxDQUFDO0FBQ2pCLGFBQVEsRUFBUixRQUFRO0FBQ1IsVUFBSyxFQUFMLEtBQUs7QUFDTCxRQUFHLEVBQUgsR0FBRztBQUNILGlCQUFZLEVBQVosWUFBWTtJQUNiLENBQUMsQ0FBQztFQUNKOzs7Ozs7Ozs7Ozs7Z0JDL0JrQixtQkFBTyxDQUFDLEVBQThCLENBQUM7O0tBQXJELFVBQVUsWUFBVixVQUFVOztBQUVmLEtBQU0sYUFBYSxHQUFHLENBQ3RCLENBQUMsc0JBQXNCLENBQUMsRUFBRSxDQUFDLGVBQWUsQ0FBQyxFQUMzQyxVQUFDLFVBQVUsRUFBRSxRQUFRLEVBQUs7QUFDdEIsT0FBRyxDQUFDLFVBQVUsRUFBQztBQUNiLFlBQU8sSUFBSSxDQUFDO0lBQ2I7Ozs7Ozs7QUFPRCxPQUFJLE1BQU0sR0FBRztBQUNYLGlCQUFZLEVBQUUsVUFBVSxDQUFDLEdBQUcsQ0FBQyxjQUFjLENBQUM7QUFDNUMsYUFBUSxFQUFFLFVBQVUsQ0FBQyxHQUFHLENBQUMsVUFBVSxDQUFDO0FBQ3BDLFNBQUksRUFBRSxVQUFVLENBQUMsR0FBRyxDQUFDLE1BQU0sQ0FBQztBQUM1QixhQUFRLEVBQUUsVUFBVSxDQUFDLEdBQUcsQ0FBQyxVQUFVLENBQUM7QUFDcEMsYUFBUSxFQUFFLFNBQVM7QUFDbkIsVUFBSyxFQUFFLFVBQVUsQ0FBQyxHQUFHLENBQUMsT0FBTyxDQUFDO0FBQzlCLFFBQUcsRUFBRSxVQUFVLENBQUMsR0FBRyxDQUFDLEtBQUssQ0FBQztBQUMxQixTQUFJLEVBQUUsU0FBUztBQUNmLFNBQUksRUFBRSxTQUFTO0lBQ2hCLENBQUM7Ozs7QUFJRixPQUFHLFFBQVEsQ0FBQyxHQUFHLENBQUMsTUFBTSxDQUFDLEdBQUcsQ0FBQyxFQUFDO0FBQzFCLFNBQUksS0FBSyxHQUFHLFVBQVUsQ0FBQyxRQUFRLENBQUMsR0FBRyxDQUFDLE1BQU0sQ0FBQyxHQUFHLENBQUMsQ0FBQyxDQUFDOztBQUVqRCxXQUFNLENBQUMsT0FBTyxHQUFHLEtBQUssQ0FBQyxPQUFPLENBQUM7QUFDL0IsV0FBTSxDQUFDLFFBQVEsR0FBRyxLQUFLLENBQUMsUUFBUSxDQUFDO0FBQ2pDLFdBQU0sQ0FBQyxRQUFRLEdBQUcsS0FBSyxDQUFDLFFBQVEsQ0FBQztBQUNqQyxXQUFNLENBQUMsTUFBTSxHQUFHLEtBQUssQ0FBQyxNQUFNLENBQUM7QUFDN0IsV0FBTSxDQUFDLElBQUksR0FBRyxLQUFLLENBQUMsSUFBSSxDQUFDO0FBQ3pCLFdBQU0sQ0FBQyxJQUFJLEdBQUcsS0FBSyxDQUFDLElBQUksQ0FBQztJQUMxQjs7QUFFRCxVQUFPLE1BQU0sQ0FBQztFQUVmLENBQ0YsQ0FBQzs7c0JBRWE7QUFDYixnQkFBYSxFQUFiLGFBQWE7RUFDZDs7Ozs7Ozs7Ozs7Ozs7c0NDOUNxQixFQUFXOzs7O3NCQUVsQix1QkFBVTtBQUN2QixnQkFBYSxFQUFFLElBQUk7QUFDbkIsa0JBQWUsRUFBRSxJQUFJO0FBQ3JCLGlCQUFjLEVBQUUsSUFBSTtFQUNyQixDQUFDOzs7Ozs7Ozs7Ozs7Z0JDTjJCLG1CQUFPLENBQUMsRUFBWSxDQUFDOztLQUE1QyxLQUFLLFlBQUwsS0FBSztLQUFFLFdBQVcsWUFBWCxXQUFXOztpQkFFaUMsbUJBQU8sQ0FBQyxFQUFlLENBQUM7O0tBQTNFLGFBQWEsYUFBYixhQUFhO0tBQUUsZUFBZSxhQUFmLGVBQWU7S0FBRSxjQUFjLGFBQWQsY0FBYzs7QUFFcEQsS0FBSSxTQUFTLEdBQUcsV0FBVyxDQUFDO0FBQzFCLFVBQU8sRUFBRSxLQUFLO0FBQ2QsaUJBQWMsRUFBRSxLQUFLO0FBQ3JCLFdBQVEsRUFBRSxLQUFLO0VBQ2hCLENBQUMsQ0FBQzs7c0JBRVksS0FBSyxDQUFDOztBQUVuQixrQkFBZSw2QkFBRztBQUNoQixZQUFPLFNBQVMsQ0FBQyxHQUFHLENBQUMsZ0JBQWdCLEVBQUUsSUFBSSxDQUFDLENBQUM7SUFDOUM7O0FBRUQsYUFBVSx3QkFBRztBQUNYLFNBQUksQ0FBQyxFQUFFLENBQUMsYUFBYSxFQUFFO2NBQUssU0FBUyxDQUFDLEdBQUcsQ0FBQyxnQkFBZ0IsRUFBRSxJQUFJLENBQUM7TUFBQSxDQUFDLENBQUM7QUFDbkUsU0FBSSxDQUFDLEVBQUUsQ0FBQyxjQUFjLEVBQUM7Y0FBSyxTQUFTLENBQUMsR0FBRyxDQUFDLFNBQVMsRUFBRSxJQUFJLENBQUM7TUFBQSxDQUFDLENBQUM7QUFDNUQsU0FBSSxDQUFDLEVBQUUsQ0FBQyxlQUFlLEVBQUM7Y0FBSyxTQUFTLENBQUMsR0FBRyxDQUFDLFVBQVUsRUFBRSxJQUFJLENBQUM7TUFBQSxDQUFDLENBQUM7SUFDL0Q7RUFDRixDQUFDOzs7Ozs7Ozs7Ozs7OztzQ0NyQm9CLEVBQVc7Ozs7c0JBRWxCLHVCQUFVO0FBQ3ZCLCtCQUE0QixFQUFFLElBQUk7QUFDbEMsZ0NBQTZCLEVBQUUsSUFBSTtFQUNwQyxDQUFDOzs7Ozs7Ozs7Ozs7Z0JDTDJCLG1CQUFPLENBQUMsRUFBWSxDQUFDOztLQUE1QyxLQUFLLFlBQUwsS0FBSztLQUFFLFdBQVcsWUFBWCxXQUFXOztpQkFFOEMsbUJBQU8sQ0FBQyxFQUFlLENBQUM7O0tBQXhGLDRCQUE0QixhQUE1Qiw0QkFBNEI7S0FBRSw2QkFBNkIsYUFBN0IsNkJBQTZCO3NCQUVsRCxLQUFLLENBQUM7O0FBRW5CLGtCQUFlLDZCQUFHO0FBQ2hCLFlBQU8sV0FBVyxDQUFDO0FBQ2pCLDZCQUFzQixFQUFFLEtBQUs7TUFDOUIsQ0FBQyxDQUFDO0lBQ0o7O0FBRUQsYUFBVSx3QkFBRztBQUNYLFNBQUksQ0FBQyxFQUFFLENBQUMsNEJBQTRCLEVBQUUsb0JBQW9CLENBQUMsQ0FBQztBQUM1RCxTQUFJLENBQUMsRUFBRSxDQUFDLDZCQUE2QixFQUFFLHFCQUFxQixDQUFDLENBQUM7SUFDL0Q7RUFDRixDQUFDOztBQUVGLFVBQVMsb0JBQW9CLENBQUMsS0FBSyxFQUFDO0FBQ2xDLFVBQU8sS0FBSyxDQUFDLEdBQUcsQ0FBQyx3QkFBd0IsRUFBRSxJQUFJLENBQUMsQ0FBQztFQUNsRDs7QUFFRCxVQUFTLHFCQUFxQixDQUFDLEtBQUssRUFBQztBQUNuQyxVQUFPLEtBQUssQ0FBQyxHQUFHLENBQUMsd0JBQXdCLEVBQUUsS0FBSyxDQUFDLENBQUM7RUFDbkQ7Ozs7Ozs7Ozs7Ozs7O3NDQ3hCcUIsRUFBVzs7OztzQkFFbEIsdUJBQVU7QUFDdkIsMkJBQXdCLEVBQUUsSUFBSTtFQUMvQixDQUFDOzs7Ozs7Ozs7Ozs7Z0JDSjJCLG1CQUFPLENBQUMsRUFBWSxDQUFDOztLQUE1QyxLQUFLLFlBQUwsS0FBSztLQUFFLFdBQVcsWUFBWCxXQUFXOztpQkFDWSxtQkFBTyxDQUFDLEVBQWUsQ0FBQzs7S0FBckQsd0JBQXdCLGFBQXhCLHdCQUF3QjtzQkFFaEIsS0FBSyxDQUFDO0FBQ25CLGtCQUFlLDZCQUFHO0FBQ2hCLFlBQU8sV0FBVyxDQUFDLElBQUksQ0FBQyxDQUFDO0lBQzFCOztBQUVELGFBQVUsd0JBQUc7QUFDWCxTQUFJLENBQUMsRUFBRSxDQUFDLHdCQUF3QixFQUFFLGFBQWEsQ0FBQztJQUNqRDtFQUNGLENBQUM7O0FBRUYsVUFBUyxhQUFhLENBQUMsS0FBSyxFQUFFLE1BQU0sRUFBQztBQUNuQyxVQUFPLFdBQVcsQ0FBQyxNQUFNLENBQUMsQ0FBQztFQUM1Qjs7Ozs7Ozs7Ozs7Ozs7c0NDZnFCLEVBQVc7Ozs7c0JBRWxCLHVCQUFVO0FBQ3ZCLHFCQUFrQixFQUFFLElBQUk7RUFDekIsQ0FBQzs7Ozs7Ozs7Ozs7QUNKRixLQUFJLE9BQU8sR0FBRyxtQkFBTyxDQUFDLENBQWEsQ0FBQyxDQUFDOztnQkFDUCxtQkFBTyxDQUFDLEdBQWUsQ0FBQzs7S0FBaEQsa0JBQWtCLFlBQWxCLGtCQUFrQjs7QUFDeEIsS0FBSSxHQUFHLEdBQUcsbUJBQU8sQ0FBQyxFQUFrQixDQUFDLENBQUM7QUFDdEMsS0FBSSxHQUFHLEdBQUcsbUJBQU8sQ0FBQyxFQUFZLENBQUMsQ0FBQzs7c0JBRWpCO0FBQ2IsYUFBVSx3QkFBRTtBQUNWLFFBQUcsQ0FBQyxHQUFHLENBQUMsR0FBRyxDQUFDLEdBQUcsQ0FBQyxTQUFTLENBQUMsQ0FBQyxJQUFJLENBQUMsWUFBVztXQUFWLElBQUkseURBQUMsRUFBRTs7QUFDdEMsV0FBSSxTQUFTLEdBQUcsSUFBSSxDQUFDLEtBQUssQ0FBQyxHQUFHLENBQUMsY0FBSTtnQkFBRSxJQUFJLENBQUMsSUFBSTtRQUFBLENBQUMsQ0FBQztBQUNoRCxjQUFPLENBQUMsUUFBUSxDQUFDLGtCQUFrQixFQUFFLFNBQVMsQ0FBQyxDQUFDO01BQ2pELENBQUMsQ0FBQztJQUNKO0VBQ0Y7Ozs7Ozs7Ozs7OztnQkNaNEIsbUJBQU8sQ0FBQyxFQUFZLENBQUM7O0tBQTVDLEtBQUssWUFBTCxLQUFLO0tBQUUsV0FBVyxZQUFYLFdBQVc7O2lCQUNNLG1CQUFPLENBQUMsR0FBZSxDQUFDOztLQUEvQyxrQkFBa0IsYUFBbEIsa0JBQWtCO3NCQUVWLEtBQUssQ0FBQztBQUNuQixrQkFBZSw2QkFBRztBQUNoQixZQUFPLFdBQVcsQ0FBQyxFQUFFLENBQUMsQ0FBQztJQUN4Qjs7QUFFRCxhQUFVLHdCQUFHO0FBQ1gsU0FBSSxDQUFDLEVBQUUsQ0FBQyxrQkFBa0IsRUFBRSxZQUFZLENBQUM7SUFDMUM7RUFDRixDQUFDOztBQUVGLFVBQVMsWUFBWSxDQUFDLEtBQUssRUFBRSxTQUFTLEVBQUM7QUFDckMsVUFBTyxXQUFXLENBQUMsU0FBUyxDQUFDLENBQUM7RUFDL0I7Ozs7Ozs7Ozs7Ozs7O3NDQ2ZxQixFQUFXOzs7O3NCQUVsQix1QkFBVTtBQUN2QixzQkFBbUIsRUFBRSxJQUFJO0FBQ3pCLHdCQUFxQixFQUFFLElBQUk7QUFDM0IscUJBQWtCLEVBQUUsSUFBSTtFQUN6QixDQUFDOzs7Ozs7Ozs7Ozs7OztzQ0NOb0IsRUFBVzs7OztzQkFFbEIsdUJBQVU7QUFDdkIsb0JBQWlCLEVBQUUsSUFBSTtFQUN4QixDQUFDOzs7Ozs7Ozs7Ozs7OztzQ0NKb0IsRUFBVzs7OztzQkFFbEIsdUJBQVU7QUFDdkIsdUJBQW9CLEVBQUUsSUFBSTtBQUMxQixzQkFBbUIsRUFBRSxJQUFJO0VBQzFCLENBQUM7Ozs7Ozs7Ozs7QUNMRixPQUFNLENBQUMsT0FBTyxDQUFDLE9BQU8sR0FBRyxtQkFBTyxDQUFDLEVBQVcsQ0FBQyxDQUFDO0FBQzlDLE9BQU0sQ0FBQyxPQUFPLENBQUMsT0FBTyxHQUFHLG1CQUFPLENBQUMsRUFBVyxDQUFDLENBQUM7QUFDOUMsT0FBTSxDQUFDLE9BQU8sQ0FBQyxlQUFlLEdBQUcsbUJBQU8sQ0FBQyxHQUFnQixDQUFDLEM7Ozs7Ozs7Ozs7O2dCQ0Y3QixtQkFBTyxDQUFDLEVBQVksQ0FBQzs7S0FBNUMsS0FBSyxZQUFMLEtBQUs7S0FBRSxXQUFXLFlBQVgsV0FBVzs7aUJBQzZCLG1CQUFPLENBQUMsR0FBZSxDQUFDOztLQUF2RSxvQkFBb0IsYUFBcEIsb0JBQW9CO0tBQUUsbUJBQW1CLGFBQW5CLG1CQUFtQjtzQkFFaEMsS0FBSyxDQUFDO0FBQ25CLGtCQUFlLDZCQUFHO0FBQ2hCLFlBQU8sV0FBVyxDQUFDLEVBQUUsQ0FBQyxDQUFDO0lBQ3hCOztBQUVELGFBQVUsd0JBQUc7QUFDWCxTQUFJLENBQUMsRUFBRSxDQUFDLG9CQUFvQixFQUFFLGVBQWUsQ0FBQyxDQUFDO0FBQy9DLFNBQUksQ0FBQyxFQUFFLENBQUMsbUJBQW1CLEVBQUUsYUFBYSxDQUFDLENBQUM7SUFDN0M7RUFDRixDQUFDOztBQUVGLFVBQVMsYUFBYSxDQUFDLEtBQUssRUFBRSxJQUFJLEVBQUM7QUFDakMsVUFBTyxLQUFLLENBQUMsR0FBRyxDQUFDLElBQUksQ0FBQyxFQUFFLEVBQUUsV0FBVyxDQUFDLElBQUksQ0FBQyxDQUFDLENBQUM7RUFDOUM7O0FBRUQsVUFBUyxlQUFlLENBQUMsS0FBSyxFQUFlO09BQWIsU0FBUyx5REFBQyxFQUFFOztBQUMxQyxVQUFPLEtBQUssQ0FBQyxhQUFhLENBQUMsZUFBSyxFQUFJO0FBQ2xDLGNBQVMsQ0FBQyxPQUFPLENBQUMsVUFBQyxJQUFJLEVBQUs7QUFDMUIsWUFBSyxDQUFDLEdBQUcsQ0FBQyxJQUFJLENBQUMsRUFBRSxFQUFFLFdBQVcsQ0FBQyxJQUFJLENBQUMsQ0FBQztNQUN0QyxDQUFDO0lBQ0gsQ0FBQyxDQUFDO0VBQ0o7Ozs7Ozs7Ozs7Ozs7O3NDQ3hCcUIsRUFBVzs7OztzQkFFbEIsdUJBQVU7QUFDdkIsb0JBQWlCLEVBQUUsSUFBSTtFQUN4QixDQUFDOzs7Ozs7Ozs7OztBQ0pGLEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsQ0FBYSxDQUFDLENBQUM7O2dCQUNULG1CQUFPLENBQUMsR0FBZSxDQUFDOztLQUE5QyxpQkFBaUIsWUFBakIsaUJBQWlCOztpQkFDSSxtQkFBTyxDQUFDLEdBQStCLENBQUM7O0tBQTdELGlCQUFpQixhQUFqQixpQkFBaUI7O0FBQ3ZCLEtBQUksY0FBYyxHQUFHLG1CQUFPLENBQUMsR0FBNkIsQ0FBQyxDQUFDO0FBQzVELEtBQUksSUFBSSxHQUFHLG1CQUFPLENBQUMsRUFBVSxDQUFDLENBQUM7QUFDL0IsS0FBSSxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxFQUFhLENBQUMsQ0FBQztBQUNyQyxLQUFJLEdBQUcsR0FBRyxtQkFBTyxDQUFDLEVBQVksQ0FBQyxDQUFDOztzQkFFakI7O0FBRWIsYUFBVSxzQkFBQyxTQUFTLEVBQUUsT0FBTyxFQUFFLEVBQUUsRUFBQztBQUNoQyxTQUFJLENBQUMsVUFBVSxFQUFFLENBQ2QsSUFBSSxDQUFDLFVBQUMsUUFBUSxFQUFJO0FBQ2pCLGNBQU8sQ0FBQyxRQUFRLENBQUMsaUJBQWlCLEVBQUUsUUFBUSxDQUFDLElBQUksQ0FBRSxDQUFDO0FBQ3BELFNBQUUsRUFBRSxDQUFDO01BQ04sQ0FBQyxDQUNELElBQUksQ0FBQyxZQUFJO0FBQ1IsY0FBTyxDQUFDLEVBQUMsVUFBVSxFQUFFLFNBQVMsQ0FBQyxRQUFRLENBQUMsUUFBUSxFQUFFLEVBQUUsR0FBRyxDQUFDLE1BQU0sQ0FBQyxLQUFLLENBQUMsQ0FBQztBQUN0RSxTQUFFLEVBQUUsQ0FBQztNQUNOLENBQUMsQ0FBQztJQUNOOztBQUVELFNBQU0sa0JBQUMsSUFBK0IsRUFBQztTQUEvQixJQUFJLEdBQUwsSUFBK0IsQ0FBOUIsSUFBSTtTQUFFLEdBQUcsR0FBVixJQUErQixDQUF4QixHQUFHO1NBQUUsS0FBSyxHQUFqQixJQUErQixDQUFuQixLQUFLO1NBQUUsV0FBVyxHQUE5QixJQUErQixDQUFaLFdBQVc7O0FBQ25DLG1CQUFjLENBQUMsS0FBSyxDQUFDLGlCQUFpQixDQUFDLENBQUM7QUFDeEMsU0FBSSxDQUFDLE1BQU0sQ0FBQyxJQUFJLEVBQUUsR0FBRyxFQUFFLEtBQUssRUFBRSxXQUFXLENBQUMsQ0FDdkMsSUFBSSxDQUFDLFVBQUMsV0FBVyxFQUFHO0FBQ25CLGNBQU8sQ0FBQyxRQUFRLENBQUMsaUJBQWlCLEVBQUUsV0FBVyxDQUFDLElBQUksQ0FBQyxDQUFDO0FBQ3RELHFCQUFjLENBQUMsT0FBTyxDQUFDLGlCQUFpQixDQUFDLENBQUM7QUFDMUMsY0FBTyxDQUFDLFVBQVUsRUFBRSxDQUFDLElBQUksQ0FBQyxFQUFDLFFBQVEsRUFBRSxHQUFHLENBQUMsTUFBTSxDQUFDLEdBQUcsRUFBQyxDQUFDLENBQUM7TUFDdkQsQ0FBQyxDQUNELElBQUksQ0FBQyxZQUFJO0FBQ1IscUJBQWMsQ0FBQyxJQUFJLENBQUMsaUJBQWlCLEVBQUUsbUJBQW1CLENBQUMsQ0FBQztNQUM3RCxDQUFDLENBQUM7SUFDTjs7QUFFRCxRQUFLLGlCQUFDLEtBQXVCLEVBQUUsUUFBUSxFQUFDO1NBQWpDLElBQUksR0FBTCxLQUF1QixDQUF0QixJQUFJO1NBQUUsUUFBUSxHQUFmLEtBQXVCLENBQWhCLFFBQVE7U0FBRSxLQUFLLEdBQXRCLEtBQXVCLENBQU4sS0FBSzs7QUFDeEIsU0FBSSxDQUFDLEtBQUssQ0FBQyxJQUFJLEVBQUUsUUFBUSxFQUFFLEtBQUssQ0FBQyxDQUM5QixJQUFJLENBQUMsVUFBQyxXQUFXLEVBQUc7QUFDbkIsY0FBTyxDQUFDLFFBQVEsQ0FBQyxpQkFBaUIsRUFBRSxXQUFXLENBQUMsSUFBSSxDQUFDLENBQUM7QUFDdEQsY0FBTyxDQUFDLFVBQVUsRUFBRSxDQUFDLElBQUksQ0FBQyxFQUFDLFFBQVEsRUFBRSxRQUFRLEVBQUMsQ0FBQyxDQUFDO01BQ2pELENBQUMsQ0FDRCxJQUFJLENBQUMsWUFBSSxFQUNULENBQUM7SUFDTDtFQUNKOzs7Ozs7Ozs7O0FDNUNELE9BQU0sQ0FBQyxPQUFPLENBQUMsT0FBTyxHQUFHLG1CQUFPLENBQUMsRUFBVyxDQUFDLENBQUM7QUFDOUMsT0FBTSxDQUFDLE9BQU8sQ0FBQyxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxHQUFXLENBQUMsQ0FBQztBQUM5QyxPQUFNLENBQUMsT0FBTyxDQUFDLFNBQVMsR0FBRyxtQkFBTyxDQUFDLEdBQWEsQ0FBQyxDOzs7Ozs7Ozs7OztnQkNGcEIsbUJBQU8sQ0FBQyxFQUFZLENBQUM7O0tBQTVDLEtBQUssWUFBTCxLQUFLO0tBQUUsV0FBVyxZQUFYLFdBQVc7O2lCQUNLLG1CQUFPLENBQUMsR0FBZSxDQUFDOztLQUE5QyxpQkFBaUIsYUFBakIsaUJBQWlCO3NCQUVULEtBQUssQ0FBQztBQUNuQixrQkFBZSw2QkFBRztBQUNoQixZQUFPLFdBQVcsQ0FBQyxJQUFJLENBQUMsQ0FBQztJQUMxQjs7QUFFRCxhQUFVLHdCQUFHO0FBQ1gsU0FBSSxDQUFDLEVBQUUsQ0FBQyxpQkFBaUIsRUFBRSxXQUFXLENBQUM7SUFDeEM7O0VBRUYsQ0FBQzs7QUFFRixVQUFTLFdBQVcsQ0FBQyxLQUFLLEVBQUUsSUFBSSxFQUFDO0FBQy9CLFVBQU8sV0FBVyxDQUFDLElBQUksQ0FBQyxDQUFDO0VBQzFCOzs7Ozs7Ozs7Ozs7QUNoQkQsS0FBSSxLQUFLLEdBQUcsbUJBQU8sQ0FBQyxDQUFPLENBQUMsQ0FBQzs7Z0JBQ2IsbUJBQU8sQ0FBQyxFQUE2QixDQUFDOztLQUFqRCxPQUFPLFlBQVAsT0FBTzs7QUFDWixLQUFJLE1BQU0sR0FBRyxDQUFDLFNBQVMsRUFBRSxTQUFTLEVBQUUsU0FBUyxFQUFFLFNBQVMsRUFBRSxTQUFTLEVBQUUsU0FBUyxDQUFDLENBQUM7O0FBRWhGLEtBQU0sUUFBUSxHQUFHLFNBQVgsUUFBUSxDQUFJLElBQTJCLEVBQUc7T0FBN0IsSUFBSSxHQUFMLElBQTJCLENBQTFCLElBQUk7T0FBRSxLQUFLLEdBQVosSUFBMkIsQ0FBcEIsS0FBSzt5QkFBWixJQUEyQixDQUFiLFVBQVU7T0FBVixVQUFVLG1DQUFDLENBQUM7O0FBQzFDLE9BQUksS0FBSyxHQUFHLE1BQU0sQ0FBQyxVQUFVLEdBQUcsTUFBTSxDQUFDLE1BQU0sQ0FBQyxDQUFDO0FBQy9DLE9BQUksS0FBSyxHQUFHO0FBQ1Ysc0JBQWlCLEVBQUUsS0FBSztBQUN4QixrQkFBYSxFQUFFLEtBQUs7SUFDckIsQ0FBQzs7QUFFRixVQUNFOzs7S0FDRTs7U0FBTSxLQUFLLEVBQUUsS0FBTSxFQUFDLFNBQVMsRUFBQywyQ0FBMkM7T0FDdkU7OztTQUFTLElBQUksQ0FBQyxDQUFDLENBQUM7UUFBVTtNQUNyQjtJQUNKLENBQ047RUFDRixDQUFDOztBQUVGLEtBQU0sZ0JBQWdCLEdBQUcsU0FBbkIsZ0JBQWdCLENBQUksS0FBUyxFQUFLO09BQWIsT0FBTyxHQUFSLEtBQVMsQ0FBUixPQUFPOztBQUNoQyxVQUFPLEdBQUcsT0FBTyxJQUFJLEVBQUUsQ0FBQztBQUN4QixPQUFJLFNBQVMsR0FBRyxPQUFPLENBQUMsR0FBRyxDQUFDLFVBQUMsSUFBSSxFQUFFLEtBQUs7WUFDdEMsb0JBQUMsUUFBUSxJQUFDLEdBQUcsRUFBRSxLQUFNLEVBQUMsVUFBVSxFQUFFLEtBQU0sRUFBQyxJQUFJLEVBQUUsSUFBSSxDQUFDLElBQUssR0FBRTtJQUM1RCxDQUFDLENBQUM7O0FBRUgsVUFDRTs7T0FBSyxTQUFTLEVBQUMsMEJBQTBCO0tBQ3ZDOztTQUFJLFNBQVMsRUFBQyxLQUFLO09BQ2hCLFNBQVM7T0FDVjs7O1NBQ0U7O2FBQVEsT0FBTyxFQUFFLE9BQU8sQ0FBQyxLQUFNLEVBQUMsU0FBUyxFQUFDLDJCQUEyQixFQUFDLElBQUksRUFBQyxRQUFRO1dBQ2pGLDJCQUFHLFNBQVMsRUFBQyxhQUFhLEdBQUs7VUFDeEI7UUFDTjtNQUNGO0lBQ0QsQ0FDUDtFQUNGLENBQUM7O0FBRUYsT0FBTSxDQUFDLE9BQU8sR0FBRyxnQkFBZ0IsQzs7Ozs7Ozs7Ozs7OztBQ3hDakMsS0FBSSxLQUFLLEdBQUcsbUJBQU8sQ0FBQyxDQUFPLENBQUMsQ0FBQzs7QUFFN0IsS0FBSSxjQUFjLEdBQUcsS0FBSyxDQUFDLFdBQVcsQ0FBQzs7O0FBQ3JDLFNBQU0sb0JBQUc7QUFDUCxZQUNFOztTQUFLLFNBQVMsRUFBQyxpQkFBaUI7T0FDOUIsNkJBQUssU0FBUyxFQUFDLHNCQUFzQixHQUFPO09BQzVDOzs7O1FBQXFDO09BQ3JDOzs7O1NBQWM7O2FBQUcsSUFBSSxFQUFDLDBEQUEwRDs7VUFBeUI7O1FBQXFEO01BQzFKLENBQ047SUFDSDtFQUNGLENBQUM7O0FBRUYsT0FBTSxDQUFDLE9BQU8sR0FBRyxjQUFjLEM7Ozs7Ozs7Ozs7Ozs7OztBQ2QvQixLQUFJLEtBQUssR0FBRyxtQkFBTyxDQUFDLENBQU8sQ0FBQyxDQUFDO0FBQzdCLEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsQ0FBYSxDQUFDLENBQUM7O2dCQUNaLG1CQUFPLENBQUMsR0FBbUIsQ0FBQzs7S0FBaEQsT0FBTyxZQUFQLE9BQU87S0FBRSxPQUFPLFlBQVAsT0FBTzs7QUFDckIsS0FBSSxXQUFXLEdBQUcsbUJBQU8sQ0FBQyxFQUEwQixDQUFDLENBQUM7O2lCQUMxQixtQkFBTyxDQUFDLEdBQTBCLENBQUM7O0tBQTFELEtBQUssYUFBTCxLQUFLO0tBQUUsTUFBTSxhQUFOLE1BQU07S0FBRSxJQUFJLGFBQUosSUFBSTs7aUJBQ0MsbUJBQU8sQ0FBQyxFQUFvQyxDQUFDOztLQUFqRSxnQkFBZ0IsYUFBaEIsZ0JBQWdCOztBQUVyQixLQUFNLFFBQVEsR0FBRyxTQUFYLFFBQVEsQ0FBSSxJQUFxQztPQUFwQyxRQUFRLEdBQVQsSUFBcUMsQ0FBcEMsUUFBUTtPQUFFLElBQUksR0FBZixJQUFxQyxDQUExQixJQUFJO09BQUUsU0FBUyxHQUExQixJQUFxQyxDQUFwQixTQUFTOztPQUFLLEtBQUssNEJBQXBDLElBQXFDOztVQUNyRDtBQUFDLFNBQUk7S0FBSyxLQUFLO0tBQ1osSUFBSSxDQUFDLFFBQVEsQ0FBQyxDQUFDLFNBQVMsQ0FBQztJQUNyQjtFQUNSLENBQUM7O0FBRUYsS0FBTSxPQUFPLEdBQUcsU0FBVixPQUFPLENBQUksS0FBcUM7T0FBcEMsUUFBUSxHQUFULEtBQXFDLENBQXBDLFFBQVE7T0FBRSxJQUFJLEdBQWYsS0FBcUMsQ0FBMUIsSUFBSTtPQUFFLFNBQVMsR0FBMUIsS0FBcUMsQ0FBcEIsU0FBUzs7T0FBSyxLQUFLLDRCQUFwQyxLQUFxQzs7VUFDcEQ7QUFBQyxTQUFJO0tBQUssS0FBSztLQUNYLElBQUksQ0FBQyxRQUFRLENBQUMsQ0FBQyxJQUFJLENBQUMsR0FBRyxDQUFDLFVBQUMsSUFBSSxFQUFFLEtBQUs7Y0FDbkM7O1dBQU0sR0FBRyxFQUFFLEtBQU0sRUFBQyxTQUFTLEVBQUMscUJBQXFCO1NBQy9DLElBQUksQ0FBQyxJQUFJOztTQUFFLDRCQUFJLFNBQVMsRUFBQyx3QkFBd0IsR0FBTTtTQUN2RCxJQUFJLENBQUMsS0FBSztRQUNOO01BQUMsQ0FDVDtJQUNJO0VBQ1IsQ0FBQzs7QUFFRixLQUFNLFNBQVMsR0FBRyxTQUFaLFNBQVMsQ0FBSSxLQUE4QyxFQUFLO09BQWxELElBQUksR0FBTCxLQUE4QyxDQUE3QyxJQUFJO09BQUUsWUFBWSxHQUFuQixLQUE4QyxDQUF2QyxZQUFZO09BQUUsUUFBUSxHQUE3QixLQUE4QyxDQUF6QixRQUFRO09BQUUsSUFBSSxHQUFuQyxLQUE4QyxDQUFmLElBQUk7O09BQUssS0FBSyw0QkFBN0MsS0FBOEM7O0FBQy9ELE9BQUcsQ0FBQyxJQUFJLElBQUksSUFBSSxDQUFDLE1BQU0sQ0FBQyxNQUFNLEtBQUssQ0FBQyxFQUFDO0FBQ25DLFlBQU8sb0JBQUMsSUFBSSxFQUFLLEtBQUssQ0FBSSxDQUFDO0lBQzVCOztBQUVELE9BQUksUUFBUSxHQUFHLElBQUksQ0FBQyxRQUFRLENBQUMsQ0FBQyxFQUFFLENBQUM7QUFDakMsT0FBSSxJQUFJLEdBQUcsRUFBRSxDQUFDOztBQUVkLFlBQVMsT0FBTyxDQUFDLENBQUMsRUFBQztBQUNqQixTQUFJLEtBQUssR0FBRyxJQUFJLENBQUMsTUFBTSxDQUFDLENBQUMsQ0FBQyxDQUFDO0FBQzNCLFNBQUcsWUFBWSxFQUFDO0FBQ2QsY0FBTztnQkFBSyxZQUFZLENBQUMsUUFBUSxFQUFFLEtBQUssQ0FBQztRQUFBLENBQUM7TUFDM0MsTUFBSTtBQUNILGNBQU87Z0JBQU0sZ0JBQWdCLENBQUMsUUFBUSxFQUFFLEtBQUssQ0FBQztRQUFBLENBQUM7TUFDaEQ7SUFDRjs7QUFFRCxRQUFJLElBQUksQ0FBQyxHQUFHLENBQUMsRUFBRSxDQUFDLEdBQUcsSUFBSSxDQUFDLE1BQU0sQ0FBQyxNQUFNLEVBQUUsQ0FBQyxFQUFFLEVBQUM7QUFDekMsU0FBSSxDQUFDLElBQUksQ0FBQzs7U0FBSSxHQUFHLEVBQUUsQ0FBRTtPQUFDOztXQUFHLE9BQU8sRUFBRSxPQUFPLENBQUMsQ0FBQyxDQUFFO1NBQUUsSUFBSSxDQUFDLE1BQU0sQ0FBQyxDQUFDLENBQUM7UUFBSztNQUFLLENBQUMsQ0FBQztJQUMxRTs7QUFFRCxVQUNFO0FBQUMsU0FBSTtLQUFLLEtBQUs7S0FDYjs7U0FBSyxTQUFTLEVBQUMsV0FBVztPQUN4Qjs7V0FBUSxJQUFJLEVBQUMsUUFBUSxFQUFDLE9BQU8sRUFBRSxPQUFPLENBQUMsQ0FBQyxDQUFFLEVBQUMsU0FBUyxFQUFDLHdCQUF3QjtTQUFFLElBQUksQ0FBQyxNQUFNLENBQUMsQ0FBQyxDQUFDO1FBQVU7T0FFckcsSUFBSSxDQUFDLE1BQU0sR0FBRyxDQUFDLEdBQ1gsQ0FDRTs7V0FBUSxHQUFHLEVBQUUsQ0FBRSxFQUFDLGVBQVksVUFBVSxFQUFDLFNBQVMsRUFBQyx3Q0FBd0MsRUFBQyxpQkFBYyxNQUFNO1NBQzVHLDhCQUFNLFNBQVMsRUFBQyxPQUFPLEdBQVE7UUFDeEIsRUFDVDs7V0FBSSxHQUFHLEVBQUUsQ0FBRSxFQUFDLFNBQVMsRUFBQyxlQUFlO1NBQ2xDLElBQUk7UUFDRixDQUNOLEdBQ0QsSUFBSTtNQUVOO0lBQ0QsQ0FDUjtFQUNGLENBQUM7O0FBRUYsS0FBSSxLQUFLLEdBQUcsS0FBSyxDQUFDLFdBQVcsQ0FBQzs7O0FBRTVCLFNBQU0sRUFBRSxDQUFDLE9BQU8sQ0FBQyxVQUFVLENBQUM7O0FBRTVCLGtCQUFlLDZCQUFHO0FBQ2hCLFlBQU87QUFDTCxrQkFBVyxFQUFFLE9BQU8sQ0FBQyxZQUFZO0FBQ2pDLFdBQUksRUFBRSxXQUFXLENBQUMsSUFBSTtNQUN2QjtJQUNGOztBQUVELFNBQU0sRUFBRSxrQkFBVztBQUNqQixTQUFJLElBQUksR0FBRyxJQUFJLENBQUMsS0FBSyxDQUFDLFdBQVcsQ0FBQztBQUNsQyxTQUFJLFlBQVksR0FBRyxJQUFJLENBQUMsS0FBSyxDQUFDLFlBQVksQ0FBQztBQUMzQyxZQUNFOztTQUFLLFNBQVMsRUFBQyxXQUFXO09BQ3hCOzs7O1FBQWdCO09BQ2hCOztXQUFLLFNBQVMsRUFBQyxFQUFFO1NBQ2Y7O2FBQUssU0FBUyxFQUFDLEVBQUU7V0FDZjs7ZUFBSyxTQUFTLEVBQUMsRUFBRTthQUNmO0FBQUMsb0JBQUs7aUJBQUMsUUFBUSxFQUFFLElBQUksQ0FBQyxNQUFPLEVBQUMsU0FBUyxFQUFDLCtCQUErQjtlQUNyRSxvQkFBQyxNQUFNO0FBQ0wsMEJBQVMsRUFBQyxjQUFjO0FBQ3hCLHVCQUFNLEVBQUU7QUFBQyx1QkFBSTs7O2tCQUFvQjtBQUNqQyxxQkFBSSxFQUFFLG9CQUFDLFFBQVEsSUFBQyxJQUFJLEVBQUUsSUFBSyxHQUFJO2lCQUMvQjtlQUNGLG9CQUFDLE1BQU07QUFDTCwwQkFBUyxFQUFDLE1BQU07QUFDaEIsdUJBQU0sRUFBRTtBQUFDLHVCQUFJOzs7a0JBQWdCO0FBQzdCLHFCQUFJLEVBQUUsb0JBQUMsUUFBUSxJQUFDLElBQUksRUFBRSxJQUFLLEdBQUk7aUJBQy9CO2VBQ0Ysb0JBQUMsTUFBTTtBQUNMLDBCQUFTLEVBQUMsTUFBTTtBQUNoQix1QkFBTSxFQUFFLG9CQUFDLElBQUksT0FBVTtBQUN2QixxQkFBSSxFQUFFLG9CQUFDLE9BQU8sSUFBQyxJQUFJLEVBQUUsSUFBSyxHQUFJO2lCQUM5QjtlQUNGLG9CQUFDLE1BQU07QUFDTCwwQkFBUyxFQUFDLE9BQU87QUFDakIsNkJBQVksRUFBRSxZQUFhO0FBQzNCLHVCQUFNLEVBQUU7QUFBQyx1QkFBSTs7O2tCQUFrQjtBQUMvQixxQkFBSSxFQUFFLG9CQUFDLFNBQVMsSUFBQyxJQUFJLEVBQUUsSUFBSyxFQUFDLElBQUksRUFBRSxJQUFJLENBQUMsS0FBSyxDQUFDLElBQUssR0FBSTtpQkFDdkQ7Y0FDSTtZQUNKO1VBQ0Y7UUFDRjtNQUNGLENBQ1A7SUFDRjtFQUNGLENBQUMsQ0FBQzs7QUFFSCxPQUFNLENBQUMsT0FBTyxHQUFHLEtBQUssQzs7Ozs7Ozs7Ozs7OztBQ3JIdEIsS0FBSSxLQUFLLEdBQUcsbUJBQU8sQ0FBQyxDQUFPLENBQUMsQ0FBQzs7QUFFN0IsS0FBSSxZQUFZLEdBQUcsS0FBSyxDQUFDLFdBQVcsQ0FBQzs7O0FBQ25DLFNBQU0sb0JBQUc7QUFDUCxZQUNFOztTQUFLLFNBQVMsRUFBQyxtQkFBbUI7T0FDaEM7O1dBQUssU0FBUyxFQUFDLGVBQWU7O1FBQWU7T0FDN0M7O1dBQUssU0FBUyxFQUFDLGFBQWE7U0FBQywyQkFBRyxTQUFTLEVBQUMsZUFBZSxHQUFLOztRQUFPO09BQ3JFOzs7O1FBQW9DO09BQ3BDOzs7O1FBQXdFO09BQ3hFOzs7O1FBQTJGO09BQzNGOztXQUFLLFNBQVMsRUFBQyxpQkFBaUI7O1NBQXVEOzthQUFHLElBQUksRUFBQyxzREFBc0Q7O1VBQTJCO1FBQ3pLO01BQ0gsQ0FDTjtJQUNIO0VBQ0YsQ0FBQzs7QUFFRixPQUFNLENBQUMsT0FBTyxHQUFHLFlBQVksQzs7Ozs7Ozs7Ozs7Ozs7Ozs7OztBQ2xCN0IsS0FBSSxLQUFLLEdBQUcsbUJBQU8sQ0FBQyxDQUFPLENBQUMsQ0FBQzs7QUFFN0IsS0FBTSxnQkFBZ0IsR0FBRyxTQUFuQixnQkFBZ0IsQ0FBSSxJQUFxQztPQUFwQyxRQUFRLEdBQVQsSUFBcUMsQ0FBcEMsUUFBUTtPQUFFLElBQUksR0FBZixJQUFxQyxDQUExQixJQUFJO09BQUUsU0FBUyxHQUExQixJQUFxQyxDQUFwQixTQUFTOztPQUFLLEtBQUssNEJBQXBDLElBQXFDOztVQUM3RDtBQUFDLGlCQUFZO0tBQUssS0FBSztLQUNwQixJQUFJLENBQUMsUUFBUSxDQUFDLENBQUMsU0FBUyxDQUFDO0lBQ2I7RUFDaEIsQ0FBQzs7QUFFRixLQUFJLFlBQVksR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFDbkMsU0FBTSxvQkFBRTtBQUNOLFNBQUksS0FBSyxHQUFHLElBQUksQ0FBQyxLQUFLLENBQUM7QUFDdkIsWUFBTyxLQUFLLENBQUMsUUFBUSxHQUFHOztTQUFJLEdBQUcsRUFBRSxLQUFLLENBQUMsR0FBSTtPQUFFLEtBQUssQ0FBQyxRQUFRO01BQU0sR0FBRzs7U0FBSSxHQUFHLEVBQUUsS0FBSyxDQUFDLEdBQUk7T0FBRSxLQUFLLENBQUMsUUFBUTtNQUFNLENBQUM7SUFDL0c7RUFDRixDQUFDLENBQUM7O0FBRUgsS0FBSSxRQUFRLEdBQUcsS0FBSyxDQUFDLFdBQVcsQ0FBQzs7O0FBRS9CLGVBQVksd0JBQUMsUUFBUSxFQUFDOzs7QUFDcEIsU0FBSSxLQUFLLEdBQUcsUUFBUSxDQUFDLEdBQUcsQ0FBQyxVQUFDLElBQUksRUFBRSxLQUFLLEVBQUc7QUFDdEMsY0FBTyxNQUFLLFVBQVUsQ0FBQyxJQUFJLENBQUMsS0FBSyxDQUFDLE1BQU0sYUFBRyxLQUFLLEVBQUwsS0FBSyxFQUFFLEdBQUcsRUFBRSxLQUFLLEVBQUUsUUFBUSxFQUFFLElBQUksSUFBSyxJQUFJLENBQUMsS0FBSyxFQUFFLENBQUM7TUFDL0YsQ0FBQzs7QUFFRixZQUFPOzs7T0FBTzs7O1NBQUssS0FBSztRQUFNO01BQVE7SUFDdkM7O0FBRUQsYUFBVSxzQkFBQyxRQUFRLEVBQUM7OztBQUNsQixTQUFJLEtBQUssR0FBRyxJQUFJLENBQUMsS0FBSyxDQUFDLFFBQVEsQ0FBQztBQUNoQyxTQUFJLElBQUksR0FBRyxFQUFFLENBQUM7QUFDZCxVQUFJLElBQUksQ0FBQyxHQUFHLENBQUMsRUFBRSxDQUFDLEdBQUcsS0FBSyxFQUFFLENBQUMsRUFBRyxFQUFDO0FBQzdCLFdBQUksS0FBSyxHQUFHLFFBQVEsQ0FBQyxHQUFHLENBQUMsVUFBQyxJQUFJLEVBQUUsS0FBSyxFQUFHO0FBQ3RDLGdCQUFPLE9BQUssVUFBVSxDQUFDLElBQUksQ0FBQyxLQUFLLENBQUMsSUFBSSxhQUFHLFFBQVEsRUFBRSxDQUFDLEVBQUUsR0FBRyxFQUFFLEtBQUssRUFBRSxRQUFRLEVBQUUsS0FBSyxJQUFLLElBQUksQ0FBQyxLQUFLLEVBQUUsQ0FBQztRQUNwRyxDQUFDOztBQUVGLFdBQUksQ0FBQyxJQUFJLENBQUM7O1dBQUksR0FBRyxFQUFFLENBQUU7U0FBRSxLQUFLO1FBQU0sQ0FBQyxDQUFDO01BQ3JDOztBQUVELFlBQU87OztPQUFRLElBQUk7TUFBUyxDQUFDO0lBQzlCOztBQUVELGFBQVUsc0JBQUMsSUFBSSxFQUFFLFNBQVMsRUFBQztBQUN6QixTQUFJLE9BQU8sR0FBRyxJQUFJLENBQUM7QUFDbkIsU0FBSSxLQUFLLENBQUMsY0FBYyxDQUFDLElBQUksQ0FBQyxFQUFFO0FBQzdCLGNBQU8sR0FBRyxLQUFLLENBQUMsWUFBWSxDQUFDLElBQUksRUFBRSxTQUFTLENBQUMsQ0FBQztNQUMvQyxNQUFNLElBQUksT0FBTyxLQUFLLENBQUMsSUFBSSxLQUFLLFVBQVUsRUFBRTtBQUMzQyxjQUFPLEdBQUcsSUFBSSxDQUFDLFNBQVMsQ0FBQyxDQUFDO01BQzNCOztBQUVELFlBQU8sT0FBTyxDQUFDO0lBQ2pCOztBQUVELFNBQU0sb0JBQUc7QUFDUCxTQUFJLFFBQVEsR0FBRyxFQUFFLENBQUM7QUFDbEIsVUFBSyxDQUFDLFFBQVEsQ0FBQyxPQUFPLENBQUMsSUFBSSxDQUFDLEtBQUssQ0FBQyxRQUFRLEVBQUUsVUFBQyxLQUFLLEVBQUUsS0FBSyxFQUFLO0FBQzVELFdBQUksS0FBSyxJQUFJLElBQUksRUFBRTtBQUNqQixnQkFBTztRQUNSOztBQUVELFdBQUcsS0FBSyxDQUFDLElBQUksQ0FBQyxXQUFXLEtBQUssZ0JBQWdCLEVBQUM7QUFDN0MsZUFBTSwwQkFBMEIsQ0FBQztRQUNsQzs7QUFFRCxlQUFRLENBQUMsSUFBSSxDQUFDLEtBQUssQ0FBQyxDQUFDO01BQ3RCLENBQUMsQ0FBQzs7QUFFSCxTQUFJLFVBQVUsR0FBRyxRQUFRLEdBQUcsSUFBSSxDQUFDLEtBQUssQ0FBQyxTQUFTLENBQUM7O0FBRWpELFlBQ0U7O1NBQU8sU0FBUyxFQUFFLFVBQVc7T0FDMUIsSUFBSSxDQUFDLFlBQVksQ0FBQyxRQUFRLENBQUM7T0FDM0IsSUFBSSxDQUFDLFVBQVUsQ0FBQyxRQUFRLENBQUM7TUFDcEIsQ0FDUjtJQUNIO0VBQ0YsQ0FBQzs7QUFFRixLQUFJLGNBQWMsR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFDckMsU0FBTSxFQUFFLGtCQUFXO0FBQ2pCLFdBQU0sSUFBSSxLQUFLLENBQUMsa0RBQWtELENBQUMsQ0FBQztJQUNyRTtFQUNGLENBQUM7O3NCQUVhLFFBQVE7U0FDRyxNQUFNLEdBQXhCLGNBQWM7U0FBd0IsS0FBSyxHQUFqQixRQUFRO1NBQTJCLElBQUksR0FBcEIsWUFBWTtTQUE4QixRQUFRLEdBQTVCLGdCQUFnQixDOzs7Ozs7Ozs7Ozs7O0FDbEYzRixLQUFJLElBQUksR0FBRyxtQkFBTyxDQUFDLEdBQVUsQ0FBQyxDQUFDO0FBQy9CLEtBQUksS0FBSyxHQUFHLG1CQUFPLENBQUMsQ0FBTyxDQUFDLENBQUM7O2dCQUNGLG1CQUFPLENBQUMsR0FBRyxDQUFDOztLQUFsQyxRQUFRLFlBQVIsUUFBUTtLQUFFLFFBQVEsWUFBUixRQUFROztBQUV2QixLQUFJLENBQUMsTUFBTSxDQUFDLEdBQUcsQ0FBQyxHQUFHLFNBQVMsQ0FBQzs7QUFFN0IsS0FBTSxjQUFjLEdBQUcsZ0NBQWdDLENBQUM7QUFDeEQsS0FBTSxhQUFhLEdBQUcsZ0JBQWdCLENBQUM7O0FBRXZDLEtBQUksV0FBVyxHQUFHLEtBQUssQ0FBQyxXQUFXLENBQUM7OztBQUVsQyxrQkFBZSw2QkFBRTs7O0FBQ2YsU0FBSSxDQUFDLElBQUksR0FBRyxJQUFJLENBQUMsS0FBSyxDQUFDLElBQUksQ0FBQztBQUM1QixTQUFJLENBQUMsSUFBSSxHQUFHLElBQUksQ0FBQyxLQUFLLENBQUMsSUFBSSxDQUFDO0FBQzVCLFNBQUksQ0FBQyxHQUFHLEdBQUcsSUFBSSxDQUFDLEtBQUssQ0FBQyxHQUFHLENBQUM7O0FBRTFCLFNBQUksQ0FBQyxlQUFlLEdBQUcsUUFBUSxDQUFDLFlBQUk7QUFDbEMsYUFBSyxNQUFNLEVBQUUsQ0FBQztBQUNkLGFBQUssR0FBRyxDQUFDLE1BQU0sQ0FBQyxNQUFLLElBQUksRUFBRSxNQUFLLElBQUksQ0FBQyxDQUFDO01BQ3ZDLEVBQUUsR0FBRyxDQUFDLENBQUM7O0FBRVIsWUFBTyxFQUFFLENBQUM7SUFDWDs7QUFFRCxvQkFBaUIsRUFBRSw2QkFBVzs7O0FBQzVCLFNBQUksQ0FBQyxJQUFJLEdBQUcsSUFBSSxRQUFRLENBQUM7QUFDdkIsV0FBSSxFQUFFLENBQUM7QUFDUCxXQUFJLEVBQUUsQ0FBQztBQUNQLGVBQVEsRUFBRSxJQUFJO0FBQ2QsaUJBQVUsRUFBRSxJQUFJO0FBQ2hCLGtCQUFXLEVBQUUsSUFBSTtNQUNsQixDQUFDLENBQUM7O0FBRUgsU0FBSSxDQUFDLElBQUksQ0FBQyxJQUFJLENBQUMsSUFBSSxDQUFDLElBQUksQ0FBQyxTQUFTLENBQUMsQ0FBQztBQUNwQyxTQUFJLENBQUMsSUFBSSxDQUFDLEVBQUUsQ0FBQyxNQUFNLEVBQUUsVUFBQyxJQUFJO2NBQUssT0FBSyxHQUFHLENBQUMsSUFBSSxDQUFDLElBQUksQ0FBQztNQUFBLENBQUMsQ0FBQzs7QUFFcEQsU0FBSSxDQUFDLE1BQU0sQ0FBQyxJQUFJLENBQUMsSUFBSSxFQUFFLElBQUksQ0FBQyxJQUFJLENBQUMsQ0FBQzs7QUFFbEMsU0FBSSxDQUFDLEdBQUcsQ0FBQyxFQUFFLENBQUMsTUFBTSxFQUFFO2NBQUssT0FBSyxJQUFJLENBQUMsS0FBSyxDQUFDLGFBQWEsQ0FBQztNQUFBLENBQUMsQ0FBQztBQUN6RCxTQUFJLENBQUMsR0FBRyxDQUFDLEVBQUUsQ0FBQyxPQUFPLEVBQUU7Y0FBSyxPQUFLLElBQUksQ0FBQyxLQUFLLENBQUMsY0FBYyxDQUFDO01BQUEsQ0FBQyxDQUFDO0FBQzNELFNBQUksQ0FBQyxHQUFHLENBQUMsRUFBRSxDQUFDLE1BQU0sRUFBRSxVQUFDLElBQUk7Y0FBSyxPQUFLLElBQUksQ0FBQyxLQUFLLENBQUMsSUFBSSxDQUFDO01BQUEsQ0FBQyxDQUFDO0FBQ3JELFNBQUksQ0FBQyxHQUFHLENBQUMsRUFBRSxDQUFDLE9BQU8sRUFBRTtjQUFLLE9BQUssSUFBSSxDQUFDLEtBQUssRUFBRTtNQUFBLENBQUMsQ0FBQzs7QUFFN0MsU0FBSSxDQUFDLEdBQUcsQ0FBQyxPQUFPLENBQUMsRUFBQyxJQUFJLEVBQUUsSUFBSSxDQUFDLElBQUksRUFBRSxJQUFJLEVBQUUsSUFBSSxDQUFDLElBQUksRUFBQyxDQUFDLENBQUM7QUFDckQsV0FBTSxDQUFDLGdCQUFnQixDQUFDLFFBQVEsRUFBRSxJQUFJLENBQUMsZUFBZSxDQUFDLENBQUM7SUFDekQ7O0FBRUQsdUJBQW9CLEVBQUUsZ0NBQVc7QUFDL0IsU0FBSSxDQUFDLElBQUksQ0FBQyxPQUFPLEVBQUUsQ0FBQztBQUNwQixXQUFNLENBQUMsbUJBQW1CLENBQUMsUUFBUSxFQUFFLElBQUksQ0FBQyxlQUFlLENBQUMsQ0FBQztJQUM1RDs7QUFFRCx3QkFBcUIsRUFBRSwrQkFBUyxRQUFRLEVBQUU7U0FDbkMsSUFBSSxHQUFVLFFBQVEsQ0FBdEIsSUFBSTtTQUFFLElBQUksR0FBSSxRQUFRLENBQWhCLElBQUk7O0FBRWYsU0FBSSxDQUFDLFFBQVEsQ0FBQyxJQUFJLENBQUMsSUFBSSxDQUFDLFFBQVEsQ0FBQyxJQUFJLENBQUMsRUFBQztBQUNyQyxjQUFPLEtBQUssQ0FBQztNQUNkOztBQUVELFNBQUcsSUFBSSxLQUFLLElBQUksQ0FBQyxJQUFJLElBQUksSUFBSSxLQUFLLElBQUksQ0FBQyxJQUFJLEVBQUM7QUFDMUMsV0FBSSxDQUFDLE1BQU0sQ0FBQyxJQUFJLEVBQUUsSUFBSSxDQUFDO01BQ3hCOztBQUVELFlBQU8sS0FBSyxDQUFDO0lBQ2Q7O0FBRUQsU0FBTSxvQkFBRztBQUNQLFlBQVM7O1NBQUssU0FBUyxFQUFDLGNBQWMsRUFBQyxFQUFFLEVBQUMsY0FBYyxFQUFDLEdBQUcsRUFBQyxXQUFXOztNQUFTLENBQUc7SUFDckY7O0FBRUQsU0FBTSxFQUFFLGdCQUFTLElBQUksRUFBRSxJQUFJLEVBQUU7O0FBRTNCLFNBQUcsQ0FBQyxRQUFRLENBQUMsSUFBSSxDQUFDLElBQUksQ0FBQyxRQUFRLENBQUMsSUFBSSxDQUFDLEVBQUM7QUFDcEMsV0FBSSxHQUFHLEdBQUcsSUFBSSxDQUFDLGNBQWMsRUFBRSxDQUFDO0FBQ2hDLFdBQUksR0FBRyxHQUFHLENBQUMsSUFBSSxDQUFDO0FBQ2hCLFdBQUksR0FBRyxHQUFHLENBQUMsSUFBSSxDQUFDO01BQ2pCOztBQUVELFNBQUksQ0FBQyxJQUFJLEdBQUcsSUFBSSxDQUFDO0FBQ2pCLFNBQUksQ0FBQyxJQUFJLEdBQUcsSUFBSSxDQUFDOztBQUVqQixTQUFJLENBQUMsSUFBSSxDQUFDLE1BQU0sQ0FBQyxJQUFJLENBQUMsSUFBSSxFQUFFLElBQUksQ0FBQyxJQUFJLENBQUMsQ0FBQztJQUN4Qzs7QUFFRCxpQkFBYyw0QkFBRTtBQUNkLFNBQUksVUFBVSxHQUFHLENBQUMsQ0FBQyxJQUFJLENBQUMsSUFBSSxDQUFDLFNBQVMsQ0FBQyxDQUFDO0FBQ3hDLFNBQUksT0FBTyxHQUFHLENBQUMsQ0FBQyxnQ0FBZ0MsQ0FBQyxDQUFDOztBQUVsRCxlQUFVLENBQUMsSUFBSSxDQUFDLFdBQVcsQ0FBQyxDQUFDLE1BQU0sQ0FBQyxPQUFPLENBQUMsQ0FBQzs7QUFFN0MsU0FBSSxhQUFhLEdBQUcsT0FBTyxDQUFDLENBQUMsQ0FBQyxDQUFDLHFCQUFxQixFQUFFLENBQUMsTUFBTSxDQUFDOztBQUU5RCxTQUFJLFlBQVksR0FBRyxPQUFPLENBQUMsUUFBUSxFQUFFLENBQUMsS0FBSyxFQUFFLENBQUMsQ0FBQyxDQUFDLENBQUMscUJBQXFCLEVBQUUsQ0FBQyxLQUFLLENBQUM7QUFDL0UsU0FBSSxJQUFJLEdBQUcsSUFBSSxDQUFDLEtBQUssQ0FBQyxVQUFVLENBQUMsS0FBSyxFQUFFLEdBQUksWUFBYSxDQUFDLENBQUM7QUFDM0QsU0FBSSxJQUFJLEdBQUcsSUFBSSxDQUFDLEtBQUssQ0FBQyxVQUFVLENBQUMsTUFBTSxFQUFFLEdBQUksYUFBYyxDQUFDLENBQUM7QUFDN0QsWUFBTyxDQUFDLE1BQU0sRUFBRSxDQUFDOztBQUVqQixZQUFPLEVBQUMsSUFBSSxFQUFKLElBQUksRUFBRSxJQUFJLEVBQUosSUFBSSxFQUFDLENBQUM7SUFDckI7O0VBRUYsQ0FBQyxDQUFDOztBQUVILFlBQVcsQ0FBQyxTQUFTLEdBQUc7QUFDdEIsTUFBRyxFQUFFLEtBQUssQ0FBQyxTQUFTLENBQUMsTUFBTSxDQUFDLFVBQVU7RUFDdkM7O0FBRUQsT0FBTSxDQUFDLE9BQU8sR0FBRyxXQUFXLEM7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7c0NDbEdOLEVBQVc7Ozs7QUFFakMsVUFBUyxZQUFZLENBQUMsTUFBTSxFQUFFO0FBQzVCLFVBQU8sTUFBTSxDQUFDLE9BQU8sQ0FBQyxxQkFBcUIsRUFBRSxNQUFNLENBQUM7RUFDckQ7O0FBRUQsVUFBUyxZQUFZLENBQUMsTUFBTSxFQUFFO0FBQzVCLFVBQU8sWUFBWSxDQUFDLE1BQU0sQ0FBQyxDQUFDLE9BQU8sQ0FBQyxNQUFNLEVBQUUsSUFBSSxDQUFDO0VBQ2xEOztBQUVELFVBQVMsZUFBZSxDQUFDLE9BQU8sRUFBRTtBQUNoQyxPQUFJLFlBQVksR0FBRyxFQUFFLENBQUM7QUFDdEIsT0FBTSxVQUFVLEdBQUcsRUFBRSxDQUFDO0FBQ3RCLE9BQU0sTUFBTSxHQUFHLEVBQUUsQ0FBQzs7QUFFbEIsT0FBSSxLQUFLO09BQUUsU0FBUyxHQUFHLENBQUM7T0FBRSxPQUFPLEdBQUcsNENBQTRDOztBQUVoRixVQUFRLEtBQUssR0FBRyxPQUFPLENBQUMsSUFBSSxDQUFDLE9BQU8sQ0FBQyxFQUFHO0FBQ3RDLFNBQUksS0FBSyxDQUFDLEtBQUssS0FBSyxTQUFTLEVBQUU7QUFDN0IsYUFBTSxDQUFDLElBQUksQ0FBQyxPQUFPLENBQUMsS0FBSyxDQUFDLFNBQVMsRUFBRSxLQUFLLENBQUMsS0FBSyxDQUFDLENBQUM7QUFDbEQsbUJBQVksSUFBSSxZQUFZLENBQUMsT0FBTyxDQUFDLEtBQUssQ0FBQyxTQUFTLEVBQUUsS0FBSyxDQUFDLEtBQUssQ0FBQyxDQUFDO01BQ3BFOztBQUVELFNBQUksS0FBSyxDQUFDLENBQUMsQ0FBQyxFQUFFO0FBQ1osbUJBQVksSUFBSSxXQUFXLENBQUM7QUFDNUIsaUJBQVUsQ0FBQyxJQUFJLENBQUMsS0FBSyxDQUFDLENBQUMsQ0FBQyxDQUFDLENBQUM7TUFDM0IsTUFBTSxJQUFJLEtBQUssQ0FBQyxDQUFDLENBQUMsS0FBSyxJQUFJLEVBQUU7QUFDNUIsbUJBQVksSUFBSSxhQUFhO0FBQzdCLGlCQUFVLENBQUMsSUFBSSxDQUFDLE9BQU8sQ0FBQyxDQUFDO01BQzFCLE1BQU0sSUFBSSxLQUFLLENBQUMsQ0FBQyxDQUFDLEtBQUssR0FBRyxFQUFFO0FBQzNCLG1CQUFZLElBQUksY0FBYztBQUM5QixpQkFBVSxDQUFDLElBQUksQ0FBQyxPQUFPLENBQUMsQ0FBQztNQUMxQixNQUFNLElBQUksS0FBSyxDQUFDLENBQUMsQ0FBQyxLQUFLLEdBQUcsRUFBRTtBQUMzQixtQkFBWSxJQUFJLEtBQUssQ0FBQztNQUN2QixNQUFNLElBQUksS0FBSyxDQUFDLENBQUMsQ0FBQyxLQUFLLEdBQUcsRUFBRTtBQUMzQixtQkFBWSxJQUFJLElBQUksQ0FBQztNQUN0Qjs7QUFFRCxXQUFNLENBQUMsSUFBSSxDQUFDLEtBQUssQ0FBQyxDQUFDLENBQUMsQ0FBQyxDQUFDOztBQUV0QixjQUFTLEdBQUcsT0FBTyxDQUFDLFNBQVMsQ0FBQztJQUMvQjs7QUFFRCxPQUFJLFNBQVMsS0FBSyxPQUFPLENBQUMsTUFBTSxFQUFFO0FBQ2hDLFdBQU0sQ0FBQyxJQUFJLENBQUMsT0FBTyxDQUFDLEtBQUssQ0FBQyxTQUFTLEVBQUUsT0FBTyxDQUFDLE1BQU0sQ0FBQyxDQUFDO0FBQ3JELGlCQUFZLElBQUksWUFBWSxDQUFDLE9BQU8sQ0FBQyxLQUFLLENBQUMsU0FBUyxFQUFFLE9BQU8sQ0FBQyxNQUFNLENBQUMsQ0FBQztJQUN2RTs7QUFFRCxVQUFPO0FBQ0wsWUFBTyxFQUFQLE9BQU87QUFDUCxpQkFBWSxFQUFaLFlBQVk7QUFDWixlQUFVLEVBQVYsVUFBVTtBQUNWLFdBQU0sRUFBTixNQUFNO0lBQ1A7RUFDRjs7QUFFRCxLQUFNLHFCQUFxQixHQUFHLEVBQUU7O0FBRXpCLFVBQVMsY0FBYyxDQUFDLE9BQU8sRUFBRTtBQUN0QyxPQUFJLEVBQUUsT0FBTyxJQUFJLHFCQUFxQixDQUFDLEVBQ3JDLHFCQUFxQixDQUFDLE9BQU8sQ0FBQyxHQUFHLGVBQWUsQ0FBQyxPQUFPLENBQUM7O0FBRTNELFVBQU8scUJBQXFCLENBQUMsT0FBTyxDQUFDO0VBQ3RDOzs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7O0FBcUJNLFVBQVMsWUFBWSxDQUFDLE9BQU8sRUFBRSxRQUFRLEVBQUU7O0FBRTlDLE9BQUksT0FBTyxDQUFDLE1BQU0sQ0FBQyxDQUFDLENBQUMsS0FBSyxHQUFHLEVBQUU7QUFDN0IsWUFBTyxTQUFPLE9BQVM7SUFDeEI7QUFDRCxPQUFJLFFBQVEsQ0FBQyxNQUFNLENBQUMsQ0FBQyxDQUFDLEtBQUssR0FBRyxFQUFFO0FBQzlCLGFBQVEsU0FBTyxRQUFVO0lBQzFCOzswQkFFMEMsY0FBYyxDQUFDLE9BQU8sQ0FBQzs7T0FBNUQsWUFBWSxvQkFBWixZQUFZO09BQUUsVUFBVSxvQkFBVixVQUFVO09BQUUsTUFBTSxvQkFBTixNQUFNOztBQUV0QyxlQUFZLElBQUksSUFBSTs7O0FBR3BCLE9BQU0sZ0JBQWdCLEdBQUcsTUFBTSxDQUFDLE1BQU0sQ0FBQyxNQUFNLEdBQUcsQ0FBQyxDQUFDLEtBQUssR0FBRzs7QUFFMUQsT0FBSSxnQkFBZ0IsRUFBRTs7QUFFcEIsaUJBQVksSUFBSSxjQUFjO0lBQy9COztBQUVELE9BQU0sS0FBSyxHQUFHLFFBQVEsQ0FBQyxLQUFLLENBQUMsSUFBSSxNQUFNLENBQUMsR0FBRyxHQUFHLFlBQVksR0FBRyxHQUFHLEVBQUUsR0FBRyxDQUFDLENBQUM7O0FBRXZFLE9BQUksaUJBQWlCO09BQUUsV0FBVztBQUNsQyxPQUFJLEtBQUssSUFBSSxJQUFJLEVBQUU7QUFDakIsU0FBSSxnQkFBZ0IsRUFBRTtBQUNwQix3QkFBaUIsR0FBRyxLQUFLLENBQUMsR0FBRyxFQUFFO0FBQy9CLFdBQU0sV0FBVyxHQUNmLEtBQUssQ0FBQyxDQUFDLENBQUMsQ0FBQyxNQUFNLENBQUMsQ0FBQyxFQUFFLEtBQUssQ0FBQyxDQUFDLENBQUMsQ0FBQyxNQUFNLEdBQUcsaUJBQWlCLENBQUMsTUFBTSxDQUFDOzs7OztBQUtoRSxXQUNFLGlCQUFpQixJQUNqQixXQUFXLENBQUMsTUFBTSxDQUFDLFdBQVcsQ0FBQyxNQUFNLEdBQUcsQ0FBQyxDQUFDLEtBQUssR0FBRyxFQUNsRDtBQUNBLGdCQUFPO0FBQ0wsNEJBQWlCLEVBQUUsSUFBSTtBQUN2QixxQkFBVSxFQUFWLFVBQVU7QUFDVixzQkFBVyxFQUFFLElBQUk7VUFDbEI7UUFDRjtNQUNGLE1BQU07O0FBRUwsd0JBQWlCLEdBQUcsRUFBRTtNQUN2Qjs7QUFFRCxnQkFBVyxHQUFHLEtBQUssQ0FBQyxLQUFLLENBQUMsQ0FBQyxDQUFDLENBQUMsR0FBRyxDQUM5QixXQUFDO2NBQUksQ0FBQyxJQUFJLElBQUksR0FBRyxrQkFBa0IsQ0FBQyxDQUFDLENBQUMsR0FBRyxDQUFDO01BQUEsQ0FDM0M7SUFDRixNQUFNO0FBQ0wsc0JBQWlCLEdBQUcsV0FBVyxHQUFHLElBQUk7SUFDdkM7O0FBRUQsVUFBTztBQUNMLHNCQUFpQixFQUFqQixpQkFBaUI7QUFDakIsZUFBVSxFQUFWLFVBQVU7QUFDVixnQkFBVyxFQUFYLFdBQVc7SUFDWjtFQUNGOztBQUVNLFVBQVMsYUFBYSxDQUFDLE9BQU8sRUFBRTtBQUNyQyxVQUFPLGNBQWMsQ0FBQyxPQUFPLENBQUMsQ0FBQyxVQUFVO0VBQzFDOztBQUVNLFVBQVMsU0FBUyxDQUFDLE9BQU8sRUFBRSxRQUFRLEVBQUU7dUJBQ1AsWUFBWSxDQUFDLE9BQU8sRUFBRSxRQUFRLENBQUM7O09BQTNELFVBQVUsaUJBQVYsVUFBVTtPQUFFLFdBQVcsaUJBQVgsV0FBVzs7QUFFL0IsT0FBSSxXQUFXLElBQUksSUFBSSxFQUFFO0FBQ3ZCLFlBQU8sVUFBVSxDQUFDLE1BQU0sQ0FBQyxVQUFVLElBQUksRUFBRSxTQUFTLEVBQUUsS0FBSyxFQUFFO0FBQ3pELFdBQUksQ0FBQyxTQUFTLENBQUMsR0FBRyxXQUFXLENBQUMsS0FBSyxDQUFDO0FBQ3BDLGNBQU8sSUFBSTtNQUNaLEVBQUUsRUFBRSxDQUFDO0lBQ1A7O0FBRUQsVUFBTyxJQUFJO0VBQ1o7Ozs7Ozs7QUFNTSxVQUFTLGFBQWEsQ0FBQyxPQUFPLEVBQUUsTUFBTSxFQUFFO0FBQzdDLFNBQU0sR0FBRyxNQUFNLElBQUksRUFBRTs7MEJBRUYsY0FBYyxDQUFDLE9BQU8sQ0FBQzs7T0FBbEMsTUFBTSxvQkFBTixNQUFNOztBQUNkLE9BQUksVUFBVSxHQUFHLENBQUM7T0FBRSxRQUFRLEdBQUcsRUFBRTtPQUFFLFVBQVUsR0FBRyxDQUFDOztBQUVqRCxPQUFJLEtBQUs7T0FBRSxTQUFTO09BQUUsVUFBVTtBQUNoQyxRQUFLLElBQUksQ0FBQyxHQUFHLENBQUMsRUFBRSxHQUFHLEdBQUcsTUFBTSxDQUFDLE1BQU0sRUFBRSxDQUFDLEdBQUcsR0FBRyxFQUFFLEVBQUUsQ0FBQyxFQUFFO0FBQ2pELFVBQUssR0FBRyxNQUFNLENBQUMsQ0FBQyxDQUFDOztBQUVqQixTQUFJLEtBQUssS0FBSyxHQUFHLElBQUksS0FBSyxLQUFLLElBQUksRUFBRTtBQUNuQyxpQkFBVSxHQUFHLEtBQUssQ0FBQyxPQUFPLENBQUMsTUFBTSxDQUFDLEtBQUssQ0FBQyxHQUFHLE1BQU0sQ0FBQyxLQUFLLENBQUMsVUFBVSxFQUFFLENBQUMsR0FBRyxNQUFNLENBQUMsS0FBSzs7QUFFcEYsOEJBQ0UsVUFBVSxJQUFJLElBQUksSUFBSSxVQUFVLEdBQUcsQ0FBQyxFQUNwQyxpQ0FBaUMsRUFDakMsVUFBVSxFQUFFLE9BQU8sQ0FDcEI7O0FBRUQsV0FBSSxVQUFVLElBQUksSUFBSSxFQUNwQixRQUFRLElBQUksU0FBUyxDQUFDLFVBQVUsQ0FBQztNQUNwQyxNQUFNLElBQUksS0FBSyxLQUFLLEdBQUcsRUFBRTtBQUN4QixpQkFBVSxJQUFJLENBQUM7TUFDaEIsTUFBTSxJQUFJLEtBQUssS0FBSyxHQUFHLEVBQUU7QUFDeEIsaUJBQVUsSUFBSSxDQUFDO01BQ2hCLE1BQU0sSUFBSSxLQUFLLENBQUMsTUFBTSxDQUFDLENBQUMsQ0FBQyxLQUFLLEdBQUcsRUFBRTtBQUNsQyxnQkFBUyxHQUFHLEtBQUssQ0FBQyxTQUFTLENBQUMsQ0FBQyxDQUFDO0FBQzlCLGlCQUFVLEdBQUcsTUFBTSxDQUFDLFNBQVMsQ0FBQzs7QUFFOUIsOEJBQ0UsVUFBVSxJQUFJLElBQUksSUFBSSxVQUFVLEdBQUcsQ0FBQyxFQUNwQyxzQ0FBc0MsRUFDdEMsU0FBUyxFQUFFLE9BQU8sQ0FDbkI7O0FBRUQsV0FBSSxVQUFVLElBQUksSUFBSSxFQUNwQixRQUFRLElBQUksa0JBQWtCLENBQUMsVUFBVSxDQUFDO01BQzdDLE1BQU07QUFDTCxlQUFRLElBQUksS0FBSztNQUNsQjtJQUNGOztBQUVELFVBQU8sUUFBUSxDQUFDLE9BQU8sQ0FBQyxNQUFNLEVBQUUsR0FBRyxDQUFDOzs7Ozs7Ozs7Ozs7Ozs7O0FDek50QyxLQUFJLEdBQUcsR0FBRyxtQkFBTyxDQUFDLEVBQWdCLENBQUMsQ0FBQztBQUNwQyxLQUFJLEdBQUcsR0FBRyxtQkFBTyxDQUFDLEVBQWtCLENBQUMsQ0FBQztBQUN0QyxLQUFJLEdBQUcsR0FBRyxtQkFBTyxDQUFDLEVBQVksQ0FBQyxDQUFDOztLQUUxQixTQUFTO2FBQVQsU0FBUzs7QUFDRixZQURQLFNBQVMsQ0FDRCxJQUFLLEVBQUM7U0FBTCxHQUFHLEdBQUosSUFBSyxDQUFKLEdBQUc7OzJCQURaLFNBQVM7O0FBRVgscUJBQU0sRUFBRSxDQUFDLENBQUM7QUFDVixTQUFJLENBQUMsR0FBRyxHQUFHLEdBQUcsQ0FBQztBQUNmLFNBQUksQ0FBQyxPQUFPLEdBQUcsQ0FBQyxDQUFDO0FBQ2pCLFNBQUksQ0FBQyxNQUFNLEdBQUcsQ0FBQyxDQUFDLENBQUM7QUFDakIsU0FBSSxDQUFDLFFBQVEsR0FBRyxJQUFJLEtBQUssRUFBRSxDQUFDO0FBQzVCLFNBQUksQ0FBQyxRQUFRLEdBQUcsS0FBSyxDQUFDO0FBQ3RCLFNBQUksQ0FBQyxTQUFTLEdBQUcsS0FBSyxDQUFDO0FBQ3ZCLFNBQUksQ0FBQyxPQUFPLEdBQUcsS0FBSyxDQUFDO0FBQ3JCLFNBQUksQ0FBQyxPQUFPLEdBQUcsS0FBSyxDQUFDO0FBQ3JCLFNBQUksQ0FBQyxTQUFTLEdBQUcsSUFBSSxDQUFDO0lBQ3ZCOztBQVpHLFlBQVMsV0FjYixJQUFJLG1CQUFFLEVBQ0w7O0FBZkcsWUFBUyxXQWlCYixNQUFNLHFCQUFFLEVBQ1A7O0FBbEJHLFlBQVMsV0FvQmIsT0FBTyxzQkFBRTs7O0FBQ1AsUUFBRyxDQUFDLEdBQUcsQ0FBQyxHQUFHLENBQUMsR0FBRyxDQUFDLHdCQUF3QixDQUFDLElBQUksQ0FBQyxHQUFHLENBQUMsQ0FBQyxDQUNoRCxJQUFJLENBQUMsVUFBQyxJQUFJLEVBQUc7QUFDWixhQUFLLE1BQU0sR0FBRyxJQUFJLENBQUMsS0FBSyxDQUFDO0FBQ3pCLGFBQUssT0FBTyxHQUFHLElBQUksQ0FBQztNQUNyQixDQUFDLENBQ0QsSUFBSSxDQUFDLFlBQUk7QUFDUixhQUFLLE9BQU8sR0FBRyxJQUFJLENBQUM7TUFDckIsQ0FBQyxDQUNELE1BQU0sQ0FBQyxZQUFJO0FBQ1YsYUFBSyxPQUFPLEVBQUUsQ0FBQztNQUNoQixDQUFDLENBQUM7SUFDTjs7QUFoQ0csWUFBUyxXQWtDYixJQUFJLGlCQUFDLE1BQU0sRUFBQztBQUNWLFNBQUcsQ0FBQyxJQUFJLENBQUMsT0FBTyxFQUFDO0FBQ2YsY0FBTztNQUNSOztBQUVELFNBQUcsTUFBTSxLQUFLLFNBQVMsRUFBQztBQUN0QixhQUFNLEdBQUcsSUFBSSxDQUFDLE9BQU8sR0FBRyxDQUFDLENBQUM7TUFDM0I7O0FBRUQsU0FBRyxNQUFNLEdBQUcsSUFBSSxDQUFDLE1BQU0sRUFBQztBQUN0QixhQUFNLEdBQUcsSUFBSSxDQUFDLE1BQU0sQ0FBQztBQUNyQixXQUFJLENBQUMsSUFBSSxFQUFFLENBQUM7TUFDYjs7QUFFRCxTQUFHLE1BQU0sS0FBSyxDQUFDLEVBQUM7QUFDZCxhQUFNLEdBQUcsQ0FBQyxDQUFDO01BQ1o7O0FBRUQsU0FBRyxJQUFJLENBQUMsU0FBUyxFQUFDO0FBQ2hCLFdBQUcsSUFBSSxDQUFDLE9BQU8sR0FBRyxNQUFNLEVBQUM7QUFDdkIsYUFBSSxDQUFDLFVBQVUsQ0FBQyxJQUFJLENBQUMsT0FBTyxFQUFFLE1BQU0sQ0FBQyxDQUFDO1FBQ3ZDLE1BQUk7QUFDSCxhQUFJLENBQUMsSUFBSSxDQUFDLE9BQU8sQ0FBQyxDQUFDO0FBQ25CLGFBQUksQ0FBQyxVQUFVLENBQUMsSUFBSSxDQUFDLE9BQU8sRUFBRSxNQUFNLENBQUMsQ0FBQztRQUN2QztNQUNGLE1BQUk7QUFDSCxXQUFJLENBQUMsT0FBTyxHQUFHLE1BQU0sQ0FBQztNQUN2Qjs7QUFFRCxTQUFJLENBQUMsT0FBTyxFQUFFLENBQUM7SUFDaEI7O0FBaEVHLFlBQVMsV0FrRWIsSUFBSSxtQkFBRTtBQUNKLFNBQUksQ0FBQyxTQUFTLEdBQUcsS0FBSyxDQUFDO0FBQ3ZCLFNBQUksQ0FBQyxLQUFLLEdBQUcsYUFBYSxDQUFDLElBQUksQ0FBQyxLQUFLLENBQUMsQ0FBQztBQUN2QyxTQUFJLENBQUMsT0FBTyxFQUFFLENBQUM7SUFDaEI7O0FBdEVHLFlBQVMsV0F3RWIsSUFBSSxtQkFBRTtBQUNKLFNBQUcsSUFBSSxDQUFDLFNBQVMsRUFBQztBQUNoQixjQUFPO01BQ1I7O0FBRUQsU0FBSSxDQUFDLFNBQVMsR0FBRyxJQUFJLENBQUM7QUFDdEIsU0FBSSxDQUFDLEtBQUssR0FBRyxXQUFXLENBQUMsSUFBSSxDQUFDLElBQUksQ0FBQyxJQUFJLENBQUMsSUFBSSxDQUFDLEVBQUUsR0FBRyxDQUFDLENBQUM7QUFDcEQsU0FBSSxDQUFDLE9BQU8sRUFBRSxDQUFDO0lBQ2hCOztBQWhGRyxZQUFTLFdBa0ZiLFlBQVkseUJBQUMsS0FBSyxFQUFFLEdBQUcsRUFBQztBQUN0QixVQUFJLElBQUksQ0FBQyxHQUFHLEtBQUssRUFBRSxDQUFDLEdBQUcsR0FBRyxFQUFFLENBQUMsRUFBRSxFQUFDO0FBQzlCLFdBQUcsSUFBSSxDQUFDLFFBQVEsQ0FBQyxDQUFDLENBQUMsS0FBSyxTQUFTLEVBQUM7QUFDaEMsZ0JBQU8sSUFBSSxDQUFDO1FBQ2I7TUFDRjs7QUFFRCxZQUFPLEtBQUssQ0FBQztJQUNkOztBQTFGRyxZQUFTLFdBNEZiLE1BQU0sbUJBQUMsS0FBSyxFQUFFLEdBQUcsRUFBQzs7O0FBQ2hCLFFBQUcsR0FBRyxHQUFHLEdBQUcsRUFBRSxDQUFDO0FBQ2YsUUFBRyxHQUFHLEdBQUcsR0FBRyxJQUFJLENBQUMsTUFBTSxHQUFHLElBQUksQ0FBQyxNQUFNLEdBQUcsR0FBRyxDQUFDO0FBQzVDLFlBQU8sR0FBRyxDQUFDLEdBQUcsQ0FBQyxHQUFHLENBQUMsR0FBRyxDQUFDLHVCQUF1QixDQUFDLEVBQUMsR0FBRyxFQUFFLElBQUksQ0FBQyxHQUFHLEVBQUUsS0FBSyxFQUFMLEtBQUssRUFBRSxHQUFHLEVBQUgsR0FBRyxFQUFDLENBQUMsQ0FBQyxDQUMxRSxJQUFJLENBQUMsVUFBQyxRQUFRLEVBQUc7QUFDZixZQUFJLElBQUksQ0FBQyxHQUFHLENBQUMsRUFBRSxDQUFDLEdBQUcsR0FBRyxHQUFDLEtBQUssRUFBRSxDQUFDLEVBQUUsRUFBQztBQUNoQyxhQUFJLElBQUksR0FBRyxJQUFJLENBQUMsUUFBUSxDQUFDLE1BQU0sQ0FBQyxDQUFDLENBQUMsQ0FBQyxJQUFJLENBQUMsSUFBSSxFQUFFLENBQUM7QUFDL0MsYUFBSSxLQUFLLEdBQUcsUUFBUSxDQUFDLE1BQU0sQ0FBQyxDQUFDLENBQUMsQ0FBQyxLQUFLLENBQUM7QUFDckMsZ0JBQUssUUFBUSxDQUFDLEtBQUssR0FBQyxDQUFDLENBQUMsR0FBRyxFQUFFLElBQUksRUFBSixJQUFJLEVBQUUsS0FBSyxFQUFMLEtBQUssRUFBQyxDQUFDO1FBQ3pDO01BQ0YsQ0FBQyxDQUFDO0lBQ047O0FBdkdHLFlBQVMsV0F5R2IsVUFBVSx1QkFBQyxLQUFLLEVBQUUsR0FBRyxFQUFDOzs7QUFDcEIsU0FBSSxPQUFPLEdBQUcsU0FBVixPQUFPLEdBQU87QUFDaEIsWUFBSSxJQUFJLENBQUMsR0FBRyxLQUFLLEVBQUUsQ0FBQyxHQUFHLEdBQUcsRUFBRSxDQUFDLEVBQUUsRUFBQztBQUM5QixnQkFBSyxJQUFJLENBQUMsTUFBTSxFQUFFLE9BQUssUUFBUSxDQUFDLENBQUMsQ0FBQyxDQUFDLElBQUksQ0FBQyxDQUFDO1FBQzFDO0FBQ0QsY0FBSyxPQUFPLEdBQUcsR0FBRyxDQUFDO01BQ3BCLENBQUM7O0FBRUYsU0FBRyxJQUFJLENBQUMsWUFBWSxDQUFDLEtBQUssRUFBRSxHQUFHLENBQUMsRUFBQztBQUMvQixXQUFJLENBQUMsTUFBTSxDQUFDLEtBQUssRUFBRSxHQUFHLENBQUMsQ0FBQyxJQUFJLENBQUMsT0FBTyxDQUFDLENBQUM7TUFDdkMsTUFBSTtBQUNILGNBQU8sRUFBRSxDQUFDO01BQ1g7SUFDRjs7QUF0SEcsWUFBUyxXQXdIYixPQUFPLHNCQUFFO0FBQ1AsU0FBSSxDQUFDLElBQUksQ0FBQyxRQUFRLENBQUMsQ0FBQztJQUNyQjs7VUExSEcsU0FBUztJQUFTLEdBQUc7O3NCQTZIWixTQUFTOzs7Ozs7Ozs7OztBQ2pJeEIsS0FBSSxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxDQUFhLENBQUMsQ0FBQzs7Z0JBQ2YsbUJBQU8sQ0FBQyxFQUF1QixDQUFDOztLQUFqRCxhQUFhLFlBQWIsYUFBYTs7aUJBQ0UsbUJBQU8sQ0FBQyxHQUFvQixDQUFDOztLQUE1QyxVQUFVLGFBQVYsVUFBVTs7QUFDZixLQUFJLENBQUMsR0FBRyxtQkFBTyxDQUFDLEVBQVEsQ0FBQyxDQUFDOztpQkFFK0IsbUJBQU8sQ0FBQyxFQUFlLENBQUM7O0tBQTNFLGFBQWEsYUFBYixhQUFhO0tBQUUsZUFBZSxhQUFmLGVBQWU7S0FBRSxjQUFjLGFBQWQsY0FBYzs7QUFFcEQsS0FBSSxPQUFPLEdBQUc7O0FBRVosVUFBTyxxQkFBRztBQUNSLFlBQU8sQ0FBQyxRQUFRLENBQUMsYUFBYSxDQUFDLENBQUM7QUFDaEMsWUFBTyxDQUFDLHFCQUFxQixFQUFFLENBQzVCLElBQUksQ0FBQyxZQUFJO0FBQUUsY0FBTyxDQUFDLFFBQVEsQ0FBQyxjQUFjLENBQUMsQ0FBQztNQUFFLENBQUMsQ0FDL0MsSUFBSSxDQUFDLFlBQUk7QUFBRSxjQUFPLENBQUMsUUFBUSxDQUFDLGVBQWUsQ0FBQyxDQUFDO01BQUUsQ0FBQyxDQUFDOzs7O0lBSXJEOztBQUVELHdCQUFxQixtQ0FBRztBQUN0QixZQUFPLENBQUMsQ0FBQyxJQUFJLENBQUMsVUFBVSxFQUFFLEVBQUUsYUFBYSxFQUFFLENBQUMsQ0FBQztJQUM5QztFQUNGOztzQkFFYyxPQUFPOzs7Ozs7Ozs7OztBQ3hCdEIsS0FBTSxRQUFRLEdBQUcsQ0FBQyxDQUFDLE1BQU0sQ0FBQyxFQUFFLGFBQUc7VUFBRyxHQUFHLENBQUMsSUFBSSxFQUFFO0VBQUEsQ0FBQyxDQUFDOztzQkFFL0I7QUFDYixXQUFRLEVBQVIsUUFBUTtFQUNUOzs7Ozs7Ozs7O0FDSkQsT0FBTSxDQUFDLE9BQU8sQ0FBQyxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxHQUFXLENBQUMsQ0FBQztBQUM5QyxPQUFNLENBQUMsT0FBTyxDQUFDLE9BQU8sR0FBRyxtQkFBTyxDQUFDLEdBQVcsQ0FBQyxDQUFDO0FBQzlDLE9BQU0sQ0FBQyxPQUFPLENBQUMsUUFBUSxHQUFHLG1CQUFPLENBQUMsRUFBWSxDQUFDLEM7Ozs7Ozs7Ozs7QUNGL0MsS0FBTSxPQUFPLEdBQUcsQ0FBQyxDQUFDLGNBQWMsQ0FBQyxFQUFFLGVBQUs7VUFBRyxLQUFLLENBQUMsSUFBSSxFQUFFO0VBQUEsQ0FBQyxDQUFDOztzQkFFMUM7QUFDYixVQUFPLEVBQVAsT0FBTztFQUNSOzs7Ozs7Ozs7O0FDSkQsT0FBTSxDQUFDLE9BQU8sQ0FBQyxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxHQUFXLENBQUMsQ0FBQztBQUM5QyxPQUFNLENBQUMsT0FBTyxDQUFDLE9BQU8sR0FBRyxtQkFBTyxDQUFDLEVBQVcsQ0FBQyxDQUFDO0FBQzlDLE9BQU0sQ0FBQyxPQUFPLENBQUMsV0FBVyxHQUFHLG1CQUFPLENBQUMsRUFBZSxDQUFDLEM7Ozs7Ozs7OztBQ0ZyRCxLQUFJLE9BQU8sR0FBRyxtQkFBTyxDQUFDLENBQWEsQ0FBQyxDQUFDO0FBQ3JDLFFBQU8sQ0FBQyxjQUFjLENBQUM7QUFDckIsU0FBTSxFQUFFLG1CQUFPLENBQUMsRUFBZ0IsQ0FBQztBQUNqQyxpQkFBYyxFQUFFLG1CQUFPLENBQUMsRUFBdUIsQ0FBQztBQUNoRCx5QkFBc0IsRUFBRSxtQkFBTyxDQUFDLEVBQWtDLENBQUM7QUFDbkUsY0FBVyxFQUFFLG1CQUFPLENBQUMsR0FBa0IsQ0FBQztBQUN4QyxlQUFZLEVBQUUsbUJBQU8sQ0FBQyxHQUFtQixDQUFDO0FBQzFDLGdCQUFhLEVBQUUsbUJBQU8sQ0FBQyxFQUFzQixDQUFDO0FBQzlDLGtCQUFlLEVBQUUsbUJBQU8sQ0FBQyxHQUF3QixDQUFDO0FBQ2xELGtCQUFlLEVBQUUsbUJBQU8sQ0FBQyxHQUF5QixDQUFDO0VBQ3BELENBQUMsQzs7Ozs7Ozs7OztBQ1ZGLEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsQ0FBYSxDQUFDLENBQUM7O2dCQUNELG1CQUFPLENBQUMsRUFBZSxDQUFDOztLQUF0RCx3QkFBd0IsWUFBeEIsd0JBQXdCOztBQUM5QixLQUFJLEdBQUcsR0FBRyxtQkFBTyxDQUFDLEVBQWtCLENBQUMsQ0FBQztBQUN0QyxLQUFJLEdBQUcsR0FBRyxtQkFBTyxDQUFDLEVBQVksQ0FBQyxDQUFDOztzQkFFakI7QUFDYixjQUFXLHVCQUFDLFdBQVcsRUFBQztBQUN0QixTQUFJLElBQUksR0FBRyxHQUFHLENBQUMsR0FBRyxDQUFDLFlBQVksQ0FBQyxXQUFXLENBQUMsQ0FBQztBQUM3QyxRQUFHLENBQUMsR0FBRyxDQUFDLElBQUksQ0FBQyxDQUFDLElBQUksQ0FBQyxnQkFBTSxFQUFFO0FBQ3pCLGNBQU8sQ0FBQyxRQUFRLENBQUMsd0JBQXdCLEVBQUUsTUFBTSxDQUFDLENBQUM7TUFDcEQsQ0FBQyxDQUFDO0lBQ0o7RUFDRjs7Ozs7Ozs7Ozs7Ozs7Z0JDVnlCLG1CQUFPLENBQUMsR0FBK0IsQ0FBQzs7S0FBN0QsaUJBQWlCLFlBQWpCLGlCQUFpQjs7QUFFdEIsS0FBTSxNQUFNLEdBQUcsQ0FBRSxDQUFDLGFBQWEsQ0FBQyxFQUFFLFVBQUMsTUFBTSxFQUFLO0FBQzVDLFVBQU8sTUFBTSxDQUFDO0VBQ2QsQ0FDRCxDQUFDOztBQUVGLEtBQU0sTUFBTSxHQUFHLENBQUUsQ0FBQyxlQUFlLEVBQUUsaUJBQWlCLENBQUMsRUFBRSxVQUFDLE1BQU0sRUFBSztBQUNqRSxPQUFJLFVBQVUsR0FBRztBQUNmLGlCQUFZLEVBQUUsS0FBSztBQUNuQixZQUFPLEVBQUUsS0FBSztBQUNkLGNBQVMsRUFBRSxLQUFLO0FBQ2hCLFlBQU8sRUFBRSxFQUFFO0lBQ1o7O0FBRUQsVUFBTyxNQUFNLEdBQUcsTUFBTSxDQUFDLElBQUksRUFBRSxHQUFHLFVBQVUsQ0FBQztFQUUzQyxDQUNELENBQUM7O3NCQUVhO0FBQ2IsU0FBTSxFQUFOLE1BQU07QUFDTixTQUFNLEVBQU4sTUFBTTtFQUNQOzs7Ozs7Ozs7O0FDekJELE9BQU0sQ0FBQyxPQUFPLENBQUMsT0FBTyxHQUFHLG1CQUFPLENBQUMsR0FBVyxDQUFDLENBQUM7QUFDOUMsT0FBTSxDQUFDLE9BQU8sQ0FBQyxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxHQUFXLENBQUMsQ0FBQztBQUM5QyxPQUFNLENBQUMsT0FBTyxDQUFDLFNBQVMsR0FBRyxtQkFBTyxDQUFDLEVBQWUsQ0FBQyxDOzs7Ozs7Ozs7O0FDRm5ELEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsQ0FBYSxDQUFDLENBQUM7O2dCQUNaLG1CQUFPLENBQUMsRUFBdUIsQ0FBQzs7S0FBcEQsZ0JBQWdCLFlBQWhCLGdCQUFnQjs7QUFFckIsS0FBTSxZQUFZLEdBQUcsQ0FBRSxDQUFDLFlBQVksQ0FBQyxFQUFFLFVBQUMsS0FBSyxFQUFJO0FBQzdDLFVBQU8sS0FBSyxDQUFDLEdBQUcsQ0FBQyxVQUFDLElBQUksRUFBRztBQUN2QixTQUFJLFFBQVEsR0FBRyxJQUFJLENBQUMsR0FBRyxDQUFDLElBQUksQ0FBQyxDQUFDO0FBQzlCLFNBQUksUUFBUSxHQUFHLE9BQU8sQ0FBQyxRQUFRLENBQUMsZ0JBQWdCLENBQUMsUUFBUSxDQUFDLENBQUMsQ0FBQztBQUM1RCxZQUFPO0FBQ0wsU0FBRSxFQUFFLFFBQVE7QUFDWixlQUFRLEVBQUUsSUFBSSxDQUFDLEdBQUcsQ0FBQyxVQUFVLENBQUM7QUFDOUIsV0FBSSxFQUFFLE9BQU8sQ0FBQyxJQUFJLENBQUM7QUFDbkIsV0FBSSxFQUFFLElBQUksQ0FBQyxHQUFHLENBQUMsTUFBTSxDQUFDO0FBQ3RCLG1CQUFZLEVBQUUsUUFBUSxDQUFDLElBQUk7TUFDNUI7SUFDRixDQUFDLENBQUMsSUFBSSxFQUFFLENBQUM7RUFDWixDQUNELENBQUM7O0FBRUYsVUFBUyxPQUFPLENBQUMsSUFBSSxFQUFDO0FBQ3BCLE9BQUksU0FBUyxHQUFHLEVBQUUsQ0FBQztBQUNuQixPQUFJLE1BQU0sR0FBRyxJQUFJLENBQUMsR0FBRyxDQUFDLFFBQVEsQ0FBQyxDQUFDOztBQUVoQyxPQUFHLE1BQU0sRUFBQztBQUNSLFdBQU0sQ0FBQyxRQUFRLEVBQUUsQ0FBQyxPQUFPLEVBQUUsQ0FBQyxPQUFPLENBQUMsY0FBSSxFQUFFO0FBQ3hDLGdCQUFTLENBQUMsSUFBSSxDQUFDO0FBQ2IsYUFBSSxFQUFFLElBQUksQ0FBQyxDQUFDLENBQUM7QUFDYixjQUFLLEVBQUUsSUFBSSxDQUFDLENBQUMsQ0FBQztRQUNmLENBQUMsQ0FBQztNQUNKLENBQUMsQ0FBQztJQUNKOztBQUVELFNBQU0sR0FBRyxJQUFJLENBQUMsR0FBRyxDQUFDLFlBQVksQ0FBQyxDQUFDOztBQUVoQyxPQUFHLE1BQU0sRUFBQztBQUNSLFdBQU0sQ0FBQyxRQUFRLEVBQUUsQ0FBQyxPQUFPLEVBQUUsQ0FBQyxPQUFPLENBQUMsY0FBSSxFQUFFO0FBQ3hDLGdCQUFTLENBQUMsSUFBSSxDQUFDO0FBQ2IsYUFBSSxFQUFFLElBQUksQ0FBQyxDQUFDLENBQUM7QUFDYixjQUFLLEVBQUUsSUFBSSxDQUFDLENBQUMsQ0FBQyxDQUFDLEdBQUcsQ0FBQyxRQUFRLENBQUM7QUFDNUIsZ0JBQU8sRUFBRSxJQUFJLENBQUMsQ0FBQyxDQUFDLENBQUMsR0FBRyxDQUFDLFNBQVMsQ0FBQztRQUNoQyxDQUFDLENBQUM7TUFDSixDQUFDLENBQUM7SUFDSjs7QUFFRCxVQUFPLFNBQVMsQ0FBQztFQUNsQjs7c0JBR2M7QUFDYixlQUFZLEVBQVosWUFBWTtFQUNiOzs7Ozs7Ozs7O0FDakRELE9BQU0sQ0FBQyxPQUFPLENBQUMsT0FBTyxHQUFHLG1CQUFPLENBQUMsR0FBVyxDQUFDLENBQUM7QUFDOUMsT0FBTSxDQUFDLE9BQU8sQ0FBQyxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxHQUFXLENBQUMsQ0FBQztBQUM5QyxPQUFNLENBQUMsT0FBTyxDQUFDLFNBQVMsR0FBRyxtQkFBTyxDQUFDLEdBQWEsQ0FBQyxDQUFDOzs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7O0FDRmxELEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsQ0FBYSxDQUFDLENBQUM7O2dCQUtaLG1CQUFPLENBQUMsR0FBZSxDQUFDOztLQUYvQyxtQkFBbUIsWUFBbkIsbUJBQW1CO0tBQ25CLHFCQUFxQixZQUFyQixxQkFBcUI7S0FDckIsa0JBQWtCLFlBQWxCLGtCQUFrQjtzQkFFTDs7QUFFYixRQUFLLGlCQUFDLE9BQU8sRUFBQztBQUNaLFlBQU8sQ0FBQyxRQUFRLENBQUMsbUJBQW1CLEVBQUUsRUFBQyxJQUFJLEVBQUUsT0FBTyxFQUFDLENBQUMsQ0FBQztJQUN4RDs7QUFFRCxPQUFJLGdCQUFDLE9BQU8sRUFBRSxPQUFPLEVBQUM7QUFDcEIsWUFBTyxDQUFDLFFBQVEsQ0FBQyxrQkFBa0IsRUFBRyxFQUFDLElBQUksRUFBRSxPQUFPLEVBQUUsT0FBTyxFQUFQLE9BQU8sRUFBQyxDQUFDLENBQUM7SUFDakU7O0FBRUQsVUFBTyxtQkFBQyxPQUFPLEVBQUM7QUFDZCxZQUFPLENBQUMsUUFBUSxDQUFDLHFCQUFxQixFQUFFLEVBQUMsSUFBSSxFQUFFLE9BQU8sRUFBQyxDQUFDLENBQUM7SUFDMUQ7O0VBRUY7Ozs7Ozs7Ozs7OztnQkNyQjRCLG1CQUFPLENBQUMsRUFBWSxDQUFDOztLQUE1QyxLQUFLLFlBQUwsS0FBSztLQUFFLFdBQVcsWUFBWCxXQUFXOztpQkFJQyxtQkFBTyxDQUFDLEdBQWUsQ0FBQzs7S0FGL0MsbUJBQW1CLGFBQW5CLG1CQUFtQjtLQUNuQixxQkFBcUIsYUFBckIscUJBQXFCO0tBQ3JCLGtCQUFrQixhQUFsQixrQkFBa0I7c0JBRUwsS0FBSyxDQUFDO0FBQ25CLGtCQUFlLDZCQUFHO0FBQ2hCLFlBQU8sV0FBVyxDQUFDLEVBQUUsQ0FBQyxDQUFDO0lBQ3hCOztBQUVELGFBQVUsd0JBQUc7QUFDWCxTQUFJLENBQUMsRUFBRSxDQUFDLG1CQUFtQixFQUFFLEtBQUssQ0FBQyxDQUFDO0FBQ3BDLFNBQUksQ0FBQyxFQUFFLENBQUMsa0JBQWtCLEVBQUUsSUFBSSxDQUFDLENBQUM7QUFDbEMsU0FBSSxDQUFDLEVBQUUsQ0FBQyxxQkFBcUIsRUFBRSxPQUFPLENBQUMsQ0FBQztJQUN6QztFQUNGLENBQUM7O0FBRUYsVUFBUyxLQUFLLENBQUMsS0FBSyxFQUFFLE9BQU8sRUFBQztBQUM1QixVQUFPLEtBQUssQ0FBQyxHQUFHLENBQUMsT0FBTyxDQUFDLElBQUksRUFBRSxXQUFXLENBQUMsRUFBQyxZQUFZLEVBQUUsSUFBSSxFQUFDLENBQUMsQ0FBQyxDQUFDO0VBQ25FOztBQUVELFVBQVMsSUFBSSxDQUFDLEtBQUssRUFBRSxPQUFPLEVBQUM7QUFDM0IsVUFBTyxLQUFLLENBQUMsR0FBRyxDQUFDLE9BQU8sQ0FBQyxJQUFJLEVBQUUsV0FBVyxDQUFDLEVBQUMsUUFBUSxFQUFFLElBQUksRUFBRSxPQUFPLEVBQUUsT0FBTyxDQUFDLE9BQU8sRUFBQyxDQUFDLENBQUMsQ0FBQztFQUN6Rjs7QUFFRCxVQUFTLE9BQU8sQ0FBQyxLQUFLLEVBQUUsT0FBTyxFQUFDO0FBQzlCLFVBQU8sS0FBSyxDQUFDLEdBQUcsQ0FBQyxPQUFPLENBQUMsSUFBSSxFQUFFLFdBQVcsQ0FBQyxFQUFDLFNBQVMsRUFBRSxJQUFJLEVBQUMsQ0FBQyxDQUFDLENBQUM7RUFDaEU7Ozs7Ozs7Ozs7QUM1QkQsS0FBSSxLQUFLLEdBQUc7O0FBRVYsT0FBSSxrQkFBRTs7QUFFSixZQUFPLHNDQUFzQyxDQUFDLE9BQU8sQ0FBQyxPQUFPLEVBQUUsVUFBUyxDQUFDLEVBQUU7QUFDekUsV0FBSSxDQUFDLEdBQUcsSUFBSSxDQUFDLE1BQU0sRUFBRSxHQUFDLEVBQUUsR0FBQyxDQUFDO1dBQUUsQ0FBQyxHQUFHLENBQUMsSUFBSSxHQUFHLEdBQUcsQ0FBQyxHQUFJLENBQUMsR0FBQyxHQUFHLEdBQUMsR0FBSSxDQUFDO0FBQzNELGNBQU8sQ0FBQyxDQUFDLFFBQVEsQ0FBQyxFQUFFLENBQUMsQ0FBQztNQUN2QixDQUFDLENBQUM7SUFDSjs7QUFFRCxjQUFXLHVCQUFDLElBQUksRUFBQztBQUNmLFNBQUc7QUFDRCxjQUFPLElBQUksQ0FBQyxrQkFBa0IsRUFBRSxHQUFHLEdBQUcsR0FBRyxJQUFJLENBQUMsa0JBQWtCLEVBQUUsQ0FBQztNQUNwRSxRQUFNLEdBQUcsRUFBQztBQUNULGNBQU8sQ0FBQyxLQUFLLENBQUMsR0FBRyxDQUFDLENBQUM7QUFDbkIsY0FBTyxXQUFXLENBQUM7TUFDcEI7SUFDRjs7QUFFRCxlQUFZLHdCQUFDLE1BQU0sRUFBRTtBQUNuQixTQUFJLElBQUksR0FBRyxLQUFLLENBQUMsU0FBUyxDQUFDLEtBQUssQ0FBQyxJQUFJLENBQUMsU0FBUyxFQUFFLENBQUMsQ0FBQyxDQUFDO0FBQ3BELFlBQU8sTUFBTSxDQUFDLE9BQU8sQ0FBQyxJQUFJLE1BQU0sQ0FBQyxjQUFjLEVBQUUsR0FBRyxDQUFDLEVBQ25ELFVBQUMsS0FBSyxFQUFFLE1BQU0sRUFBSztBQUNqQixjQUFPLEVBQUUsSUFBSSxDQUFDLE1BQU0sQ0FBQyxLQUFLLElBQUksSUFBSSxJQUFJLENBQUMsTUFBTSxDQUFDLEtBQUssU0FBUyxDQUFDLEdBQUcsSUFBSSxDQUFDLE1BQU0sQ0FBQyxHQUFHLEVBQUUsQ0FBQztNQUNyRixDQUFDLENBQUM7SUFDSjs7RUFFRjs7QUFFRCxPQUFNLENBQUMsT0FBTyxHQUFHLEtBQUssQzs7Ozs7OztBQzdCdEI7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7QUFDQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0Esa0JBQWlCO0FBQ2pCO0FBQ0E7QUFDQTtBQUNBOztBQUVBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQSxvQkFBbUIsU0FBUztBQUM1QjtBQUNBO0FBQ0E7QUFDQSxJQUFHO0FBQ0g7QUFDQTtBQUNBLGdCQUFlLFNBQVM7QUFDeEI7O0FBRUE7QUFDQTtBQUNBLGdCQUFlLFNBQVM7QUFDeEI7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBLE1BQUs7QUFDTDtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBOztBQUVBOztBQUVBO0FBQ0E7QUFDQTs7QUFFQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUEsSUFBRztBQUNILHFCQUFvQixTQUFTO0FBQzdCO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBLE1BQUs7QUFDTDtBQUNBOztBQUVBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBOztBQUVBOztBQUVBO0FBQ0E7QUFDQSxJQUFHO0FBQ0g7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTs7Ozs7Ozs7Ozs7O0FDNVNBLEtBQUksS0FBSyxHQUFHLG1CQUFPLENBQUMsQ0FBTyxDQUFDLENBQUM7QUFDN0IsS0FBSSxVQUFVLEdBQUcsbUJBQU8sQ0FBQyxHQUFjLENBQUMsQ0FBQztBQUN6QyxLQUFJLEdBQUcsR0FBRyxtQkFBTyxDQUFDLEVBQVksQ0FBQyxDQUFDO0FBQ2hDLEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsQ0FBYSxDQUFDLENBQUM7O2dCQUNaLG1CQUFPLENBQUMsR0FBaUIsQ0FBQzs7S0FBOUMsT0FBTyxZQUFQLE9BQU87S0FBRSxPQUFPLFlBQVAsT0FBTzs7QUFDckIsS0FBSSxnQkFBZ0IsR0FBRyxtQkFBTyxDQUFDLEdBQXdCLENBQUMsQ0FBQzs7QUFFekQsS0FBSSxHQUFHLEdBQUcsS0FBSyxDQUFDLFdBQVcsQ0FBQzs7O0FBRTFCLFNBQU0sRUFBRSxDQUFDLE9BQU8sQ0FBQyxVQUFVLENBQUM7O0FBRTVCLGtCQUFlLDZCQUFHO0FBQ2hCLFlBQU87QUFDTCxVQUFHLEVBQUUsT0FBTyxDQUFDLFFBQVE7TUFDdEI7SUFDRjs7QUFFRCxxQkFBa0IsZ0NBQUU7QUFDbEIsWUFBTyxDQUFDLE9BQU8sRUFBRSxDQUFDO0FBQ2xCLFNBQUksQ0FBQyxlQUFlLEdBQUcsV0FBVyxDQUFDLE9BQU8sQ0FBQyxxQkFBcUIsRUFBRSxLQUFLLENBQUMsQ0FBQztJQUMxRTs7QUFFRCx1QkFBb0IsRUFBRSxnQ0FBVztBQUMvQixrQkFBYSxDQUFDLElBQUksQ0FBQyxlQUFlLENBQUMsQ0FBQztJQUNyQzs7QUFFRCxTQUFNLEVBQUUsa0JBQVc7QUFDakIsU0FBRyxJQUFJLENBQUMsS0FBSyxDQUFDLEdBQUcsQ0FBQyxjQUFjLEVBQUM7QUFDL0IsY0FBTyxJQUFJLENBQUM7TUFDYjs7QUFFRCxZQUNFOztTQUFLLFNBQVMsRUFBQyxVQUFVO09BQ3ZCLG9CQUFDLFVBQVUsT0FBRTtPQUNiLG9CQUFDLGdCQUFnQixPQUFFO09BQ2xCLElBQUksQ0FBQyxLQUFLLENBQUMsa0JBQWtCO09BQzlCOztXQUFLLFNBQVMsRUFBQyxLQUFLO1NBQ2xCOzthQUFLLFNBQVMsRUFBQyxFQUFFLEVBQUMsSUFBSSxFQUFDLFlBQVksRUFBQyxLQUFLLEVBQUUsRUFBRSxZQUFZLEVBQUUsQ0FBQyxFQUFFLEtBQUssRUFBRSxPQUFPLEVBQUc7V0FDN0U7O2VBQUksU0FBUyxFQUFDLG1DQUFtQzthQUMvQzs7O2VBQ0U7O21CQUFHLElBQUksRUFBRSxHQUFHLENBQUMsTUFBTSxDQUFDLE1BQU87aUJBQ3pCLDJCQUFHLFNBQVMsRUFBQyxnQkFBZ0IsR0FBSzs7Z0JBRWhDO2NBQ0Q7WUFDRjtVQUNEO1FBQ0Y7T0FDTjs7V0FBSyxTQUFTLEVBQUMsVUFBVTtTQUN0QixJQUFJLENBQUMsS0FBSyxDQUFDLFFBQVE7UUFDaEI7TUFDRixDQUNOO0lBQ0g7RUFDRixDQUFDOztBQUVGLE9BQU0sQ0FBQyxPQUFPLEdBQUcsR0FBRyxDOzs7Ozs7Ozs7Ozs7O0FDeERwQixLQUFJLEtBQUssR0FBRyxtQkFBTyxDQUFDLENBQU8sQ0FBQyxDQUFDOztnQkFDSixtQkFBTyxDQUFDLEVBQTZCLENBQUM7O0tBQTFELE9BQU8sWUFBUCxPQUFPO0tBQUUsT0FBTyxZQUFQLE9BQU87O0FBQ3JCLEtBQUksR0FBRyxHQUFHLG1CQUFPLENBQUMsRUFBZ0IsQ0FBQyxDQUFDO0FBQ3BDLEtBQUksV0FBVyxHQUFHLG1CQUFPLENBQUMsR0FBbUIsQ0FBQyxDQUFDO0FBQy9DLEtBQUksYUFBYSxHQUFHLG1CQUFPLENBQUMsR0FBcUIsQ0FBQyxDQUFDO0FBQ25ELEtBQUksZ0JBQWdCLEdBQUcsbUJBQU8sQ0FBQyxHQUFvQixDQUFDLENBQUM7O2lCQUNELG1CQUFPLENBQUMsRUFBNkIsQ0FBQzs7S0FBckYsb0JBQW9CLGFBQXBCLG9CQUFvQjtLQUFFLHFCQUFxQixhQUFyQixxQkFBcUI7O0FBRWhELEtBQUksYUFBYSxHQUFHLEtBQUssQ0FBQyxXQUFXLENBQUM7OztBQUVwQyx1QkFBb0Isa0NBQUU7QUFDcEIsMEJBQXFCLEVBQUUsQ0FBQztJQUN6Qjs7QUFFRCxTQUFNLEVBQUUsa0JBQVc7Z0NBQ2dCLElBQUksQ0FBQyxLQUFLLENBQUMsYUFBYTtTQUFwRCxRQUFRLHdCQUFSLFFBQVE7U0FBRSxLQUFLLHdCQUFMLEtBQUs7U0FBRSxPQUFPLHdCQUFQLE9BQU87O0FBQzdCLFNBQUksZUFBZSxHQUFNLEtBQUssU0FBSSxRQUFVLENBQUM7O0FBRTdDLFNBQUcsQ0FBQyxRQUFRLEVBQUM7QUFDWCxzQkFBZSxHQUFHLEVBQUUsQ0FBQztNQUN0Qjs7QUFFRCxZQUNDOztTQUFLLFNBQVMsRUFBQyxxQkFBcUI7T0FDbEMsb0JBQUMsZ0JBQWdCLElBQUMsT0FBTyxFQUFFLE9BQVEsR0FBRTtPQUNyQzs7O1NBQ0U7O2FBQUssU0FBUyxFQUFDLGlDQUFpQztXQUM5Qzs7ZUFBTSxTQUFTLEVBQUMsd0JBQXdCLEVBQUMsT0FBTyxFQUFFLG9CQUFxQjs7WUFFaEU7V0FDUDs7O2FBQUssZUFBZTtZQUFNO1VBQ3RCO1FBQ0Y7T0FDTixvQkFBQyxhQUFhLEVBQUssSUFBSSxDQUFDLEtBQUssQ0FBQyxhQUFhLENBQUk7TUFDM0MsQ0FDSjtJQUNKO0VBQ0YsQ0FBQyxDQUFDOztBQUVILEtBQUksYUFBYSxHQUFHLEtBQUssQ0FBQyxXQUFXLENBQUM7OztBQUVwQyxrQkFBZSw2QkFBRzs7O0FBQ2hCLFNBQUksQ0FBQyxHQUFHLEdBQUcsSUFBSSxHQUFHLENBQUMsSUFBSSxDQUFDLEtBQUssQ0FBQztBQUM5QixTQUFJLENBQUMsR0FBRyxDQUFDLEVBQUUsQ0FBQyxNQUFNLEVBQUU7Y0FBSyxNQUFLLFFBQVEsQ0FBQyxFQUFFLFdBQVcsRUFBRSxJQUFJLEVBQUUsQ0FBQztNQUFBLENBQUMsQ0FBQztBQUMvRCxZQUFPLEVBQUMsV0FBVyxFQUFFLEtBQUssRUFBQyxDQUFDO0lBQzdCOztBQUVELHVCQUFvQixrQ0FBRztBQUNyQixTQUFJLENBQUMsR0FBRyxDQUFDLFVBQVUsRUFBRSxDQUFDO0lBQ3ZCOztBQUVELDRCQUF5QixxQ0FBQyxTQUFTLEVBQUM7QUFDbEMsU0FBRyxTQUFTLENBQUMsUUFBUSxLQUFLLElBQUksQ0FBQyxLQUFLLENBQUMsUUFBUSxJQUMzQyxTQUFTLENBQUMsS0FBSyxLQUFLLElBQUksQ0FBQyxLQUFLLENBQUMsS0FBSyxFQUFDO0FBQ25DLFdBQUksQ0FBQyxHQUFHLENBQUMsU0FBUyxDQUFDLFNBQVMsQ0FBQyxDQUFDO0FBQzlCLFdBQUksQ0FBQyxJQUFJLENBQUMsZUFBZSxDQUFDLElBQUksQ0FBQyxLQUFLLEVBQUUsQ0FBQztNQUN4QztJQUNKOztBQUVELFNBQU0sb0JBQUc7QUFDUCxZQUNFOztTQUFLLEtBQUssRUFBRSxFQUFDLE1BQU0sRUFBRSxNQUFNLEVBQUU7T0FDM0Isb0JBQUMsV0FBVyxJQUFDLEdBQUcsRUFBQyxpQkFBaUIsRUFBQyxHQUFHLEVBQUUsSUFBSSxDQUFDLEdBQUksRUFBQyxJQUFJLEVBQUUsSUFBSSxDQUFDLEtBQUssQ0FBQyxJQUFLLEVBQUMsSUFBSSxFQUFFLElBQUksQ0FBQyxLQUFLLENBQUMsSUFBSyxHQUFHO09BQ2hHLElBQUksQ0FBQyxLQUFLLENBQUMsV0FBVyxHQUFHLG9CQUFDLGFBQWEsSUFBQyxHQUFHLEVBQUUsSUFBSSxDQUFDLEtBQUssQ0FBQyxHQUFJLEdBQUUsR0FBRyxJQUFJO01BQ25FLENBQ1A7SUFDRjtFQUNGLENBQUMsQ0FBQzs7QUFFSCxPQUFNLENBQUMsT0FBTyxHQUFHLGFBQWEsQzs7Ozs7Ozs7Ozs7Ozs7QUNyRTlCLEtBQUksR0FBRyxHQUFHLG1CQUFPLENBQUMsRUFBWSxDQUFDLENBQUM7QUFDaEMsS0FBSSxLQUFLLEdBQUcsbUJBQU8sQ0FBQyxDQUFPLENBQUMsQ0FBQztBQUM3QixLQUFJLE9BQU8sR0FBRyxtQkFBTyxDQUFDLEVBQWEsQ0FBQyxDQUFDOztnQkFDZixtQkFBTyxDQUFDLEVBQThCLENBQUM7O0tBQXhELGFBQWEsWUFBYixhQUFhOztBQUVsQixLQUFJLGFBQWEsR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFDcEMsb0JBQWlCLCtCQUFHO1NBQ2IsR0FBRyxHQUFJLElBQUksQ0FBQyxLQUFLLENBQWpCLEdBQUc7O2dDQUNNLE9BQU8sQ0FBQyxXQUFXLEVBQUU7O1NBQTlCLEtBQUssd0JBQUwsS0FBSzs7QUFDVixTQUFJLE9BQU8sR0FBRyxHQUFHLENBQUMsR0FBRyxDQUFDLHFCQUFxQixDQUFDLEtBQUssRUFBRSxHQUFHLENBQUMsQ0FBQzs7QUFFeEQsU0FBSSxDQUFDLE1BQU0sR0FBRyxJQUFJLFNBQVMsQ0FBQyxPQUFPLEVBQUUsT0FBTyxDQUFDLENBQUM7QUFDOUMsU0FBSSxDQUFDLE1BQU0sQ0FBQyxTQUFTLEdBQUcsVUFBQyxLQUFLLEVBQUs7QUFDakMsV0FDQTtBQUNFLGFBQUksSUFBSSxHQUFHLElBQUksQ0FBQyxLQUFLLENBQUMsS0FBSyxDQUFDLElBQUksQ0FBQyxDQUFDO0FBQ2xDLHNCQUFhLENBQUMsSUFBSSxDQUFDLE9BQU8sQ0FBQyxDQUFDO1FBQzdCLENBQ0QsT0FBTSxHQUFHLEVBQUM7QUFDUixnQkFBTyxDQUFDLEdBQUcsQ0FBQyxtQ0FBbUMsQ0FBQyxDQUFDO1FBQ2xEO01BRUYsQ0FBQztBQUNGLFNBQUksQ0FBQyxNQUFNLENBQUMsT0FBTyxHQUFHLFlBQU0sRUFBRSxDQUFDO0lBQ2hDOztBQUVELHVCQUFvQixrQ0FBRztBQUNyQixTQUFJLENBQUMsTUFBTSxDQUFDLEtBQUssRUFBRSxDQUFDO0lBQ3JCOztBQUVELHdCQUFxQixtQ0FBRztBQUN0QixZQUFPLEtBQUssQ0FBQztJQUNkOztBQUVELFNBQU0sb0JBQUc7QUFDUCxZQUFPLElBQUksQ0FBQztJQUNiO0VBQ0YsQ0FBQyxDQUFDOztzQkFFWSxhQUFhOzs7Ozs7Ozs7Ozs7OztBQ3ZDNUIsS0FBSSxLQUFLLEdBQUcsbUJBQU8sQ0FBQyxDQUFPLENBQUMsQ0FBQzs7Z0JBQ0osbUJBQU8sQ0FBQyxFQUE2QixDQUFDOztLQUExRCxPQUFPLFlBQVAsT0FBTztLQUFFLE9BQU8sWUFBUCxPQUFPOztBQUNyQixLQUFJLFlBQVksR0FBRyxtQkFBTyxDQUFDLEdBQWlDLENBQUMsQ0FBQztBQUM5RCxLQUFJLGFBQWEsR0FBRyxtQkFBTyxDQUFDLEdBQXFCLENBQUMsQ0FBQztBQUNuRCxLQUFJLGFBQWEsR0FBRyxtQkFBTyxDQUFDLEdBQXFCLENBQUMsQ0FBQzs7QUFFbkQsS0FBSSxrQkFBa0IsR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFFekMsU0FBTSxFQUFFLENBQUMsT0FBTyxDQUFDLFVBQVUsQ0FBQzs7QUFFNUIsa0JBQWUsNkJBQUc7QUFDaEIsWUFBTztBQUNMLHFCQUFjLEVBQUUsT0FBTyxDQUFDLGFBQWE7TUFDdEM7SUFDRjs7QUFFRCxvQkFBaUIsK0JBQUU7U0FDWCxHQUFHLEdBQUssSUFBSSxDQUFDLEtBQUssQ0FBQyxNQUFNLENBQXpCLEdBQUc7O0FBQ1QsU0FBRyxDQUFDLElBQUksQ0FBQyxLQUFLLENBQUMsY0FBYyxFQUFDO0FBQzVCLGNBQU8sQ0FBQyxXQUFXLENBQUMsR0FBRyxDQUFDLENBQUM7TUFDMUI7SUFDRjs7QUFFRCxTQUFNLEVBQUUsa0JBQVc7QUFDakIsU0FBSSxjQUFjLEdBQUcsSUFBSSxDQUFDLEtBQUssQ0FBQyxjQUFjLENBQUM7QUFDL0MsU0FBRyxDQUFDLGNBQWMsRUFBQztBQUNqQixjQUFPLElBQUksQ0FBQztNQUNiOztBQUVELFNBQUcsY0FBYyxDQUFDLFlBQVksSUFBSSxjQUFjLENBQUMsTUFBTSxFQUFDO0FBQ3RELGNBQU8sb0JBQUMsYUFBYSxJQUFDLGFBQWEsRUFBRSxjQUFlLEdBQUUsQ0FBQztNQUN4RDs7QUFFRCxZQUFPLG9CQUFDLGFBQWEsSUFBQyxhQUFhLEVBQUUsY0FBZSxHQUFFLENBQUM7SUFDeEQ7RUFDRixDQUFDLENBQUM7O0FBRUgsT0FBTSxDQUFDLE9BQU8sR0FBRyxrQkFBa0IsQzs7Ozs7Ozs7Ozs7Ozs7QUNyQ25DLEtBQUksS0FBSyxHQUFHLG1CQUFPLENBQUMsQ0FBTyxDQUFDLENBQUM7QUFDN0IsS0FBSSxXQUFXLEdBQUcsbUJBQU8sQ0FBQyxHQUFjLENBQUMsQ0FBQztBQUMxQyxLQUFJLFNBQVMsR0FBRyxtQkFBTyxDQUFDLEdBQXNCLENBQUM7QUFDL0MsS0FBSSxXQUFXLEdBQUcsbUJBQU8sQ0FBQyxHQUFtQixDQUFDLENBQUM7QUFDL0MsS0FBSSxnQkFBZ0IsR0FBRyxtQkFBTyxDQUFDLEdBQW9CLENBQUMsQ0FBQzs7QUFFckQsS0FBSSxhQUFhLEdBQUcsS0FBSyxDQUFDLFdBQVcsQ0FBQzs7O0FBQ3BDLGlCQUFjLDRCQUFFO0FBQ2QsWUFBTztBQUNMLGFBQU0sRUFBRSxJQUFJLENBQUMsR0FBRyxDQUFDLE1BQU07QUFDdkIsVUFBRyxFQUFFLENBQUM7QUFDTixnQkFBUyxFQUFFLElBQUksQ0FBQyxHQUFHLENBQUMsU0FBUztBQUM3QixjQUFPLEVBQUUsSUFBSSxDQUFDLEdBQUcsQ0FBQyxPQUFPO0FBQ3pCLGNBQU8sRUFBRSxJQUFJLENBQUMsR0FBRyxDQUFDLE1BQU0sR0FBRyxDQUFDO01BQzdCLENBQUM7SUFDSDs7QUFFRCxrQkFBZSw2QkFBRztBQUNoQixTQUFJLEdBQUcsR0FBRyxJQUFJLENBQUMsS0FBSyxDQUFDLGFBQWEsQ0FBQyxHQUFHLENBQUM7QUFDdkMsU0FBSSxDQUFDLEdBQUcsR0FBRyxJQUFJLFNBQVMsQ0FBQyxFQUFDLEdBQUcsRUFBSCxHQUFHLEVBQUMsQ0FBQyxDQUFDO0FBQ2hDLFlBQU8sSUFBSSxDQUFDLGNBQWMsRUFBRSxDQUFDO0lBQzlCOztBQUVELHVCQUFvQixrQ0FBRztBQUNyQixTQUFJLENBQUMsR0FBRyxDQUFDLElBQUksRUFBRSxDQUFDO0FBQ2hCLFNBQUksQ0FBQyxHQUFHLENBQUMsa0JBQWtCLEVBQUUsQ0FBQztJQUMvQjs7QUFFRCxvQkFBaUIsK0JBQUc7OztBQUNsQixTQUFJLENBQUMsR0FBRyxDQUFDLEVBQUUsQ0FBQyxRQUFRLEVBQUUsWUFBSTtBQUN4QixXQUFJLFFBQVEsR0FBRyxNQUFLLGNBQWMsRUFBRSxDQUFDO0FBQ3JDLGFBQUssUUFBUSxDQUFDLFFBQVEsQ0FBQyxDQUFDO01BQ3pCLENBQUMsQ0FBQztJQUNKOztBQUVELGlCQUFjLDRCQUFFO0FBQ2QsU0FBRyxJQUFJLENBQUMsS0FBSyxDQUFDLFNBQVMsRUFBQztBQUN0QixXQUFJLENBQUMsR0FBRyxDQUFDLElBQUksRUFBRSxDQUFDO01BQ2pCLE1BQUk7QUFDSCxXQUFJLENBQUMsR0FBRyxDQUFDLElBQUksRUFBRSxDQUFDO01BQ2pCO0lBQ0Y7O0FBRUQsT0FBSSxnQkFBQyxLQUFLLEVBQUM7QUFDVCxTQUFJLENBQUMsR0FBRyxDQUFDLElBQUksQ0FBQyxLQUFLLENBQUMsQ0FBQztJQUN0Qjs7QUFFRCxpQkFBYyw0QkFBRTtBQUNkLFNBQUksQ0FBQyxHQUFHLENBQUMsSUFBSSxFQUFFLENBQUM7SUFDakI7O0FBRUQsZ0JBQWEseUJBQUMsS0FBSyxFQUFDO0FBQ2xCLFNBQUksQ0FBQyxHQUFHLENBQUMsSUFBSSxFQUFFLENBQUM7QUFDaEIsU0FBSSxDQUFDLEdBQUcsQ0FBQyxJQUFJLENBQUMsS0FBSyxDQUFDLENBQUM7SUFDdEI7O0FBRUQsU0FBTSxFQUFFLGtCQUFXO1NBQ1osU0FBUyxHQUFJLElBQUksQ0FBQyxLQUFLLENBQXZCLFNBQVM7O0FBRWQsWUFDQzs7U0FBSyxTQUFTLEVBQUMsd0NBQXdDO09BQ3JELG9CQUFDLGdCQUFnQixPQUFFO09BQ25CLG9CQUFDLFdBQVcsSUFBQyxHQUFHLEVBQUMsTUFBTSxFQUFDLEdBQUcsRUFBRSxJQUFJLENBQUMsR0FBSSxFQUFDLElBQUksRUFBQyxHQUFHLEVBQUMsSUFBSSxFQUFDLEdBQUcsR0FBRztPQUMzRCxvQkFBQyxXQUFXO0FBQ1QsWUFBRyxFQUFFLElBQUksQ0FBQyxLQUFLLENBQUMsR0FBSTtBQUNwQixZQUFHLEVBQUUsSUFBSSxDQUFDLEtBQUssQ0FBQyxNQUFPO0FBQ3ZCLGNBQUssRUFBRSxJQUFJLENBQUMsS0FBSyxDQUFDLE9BQVE7QUFDMUIsc0JBQWEsRUFBRSxJQUFJLENBQUMsYUFBYztBQUNsQyx1QkFBYyxFQUFFLElBQUksQ0FBQyxjQUFlO0FBQ3BDLHFCQUFZLEVBQUUsQ0FBRTtBQUNoQixpQkFBUTtBQUNSLGtCQUFTLEVBQUMsWUFBWSxHQUNYO09BQ2Q7O1dBQVEsU0FBUyxFQUFDLEtBQUssRUFBQyxPQUFPLEVBQUUsSUFBSSxDQUFDLGNBQWU7U0FDakQsU0FBUyxHQUFHLDJCQUFHLFNBQVMsRUFBQyxZQUFZLEdBQUssR0FBSSwyQkFBRyxTQUFTLEVBQUMsWUFBWSxHQUFLO1FBQ3ZFO01BQ0wsQ0FDSjtJQUNKO0VBQ0YsQ0FBQyxDQUFDOztzQkFFWSxhQUFhOzs7Ozs7Ozs7Ozs7OztBQ2pGNUIsT0FBTSxDQUFDLE9BQU8sQ0FBQyxHQUFHLEdBQUcsbUJBQU8sQ0FBQyxHQUFXLENBQUMsQ0FBQztBQUMxQyxPQUFNLENBQUMsT0FBTyxDQUFDLEtBQUssR0FBRyxtQkFBTyxDQUFDLEdBQWEsQ0FBQyxDQUFDO0FBQzlDLE9BQU0sQ0FBQyxPQUFPLENBQUMsT0FBTyxHQUFHLG1CQUFPLENBQUMsR0FBZSxDQUFDLENBQUM7QUFDbEQsT0FBTSxDQUFDLE9BQU8sQ0FBQyxLQUFLLEdBQUcsbUJBQU8sQ0FBQyxHQUFrQixDQUFDLENBQUM7QUFDbkQsT0FBTSxDQUFDLE9BQU8sQ0FBQyxRQUFRLEdBQUcsbUJBQU8sQ0FBQyxHQUFxQixDQUFDLENBQUM7QUFDekQsT0FBTSxDQUFDLE9BQU8sQ0FBQyxrQkFBa0IsR0FBRyxtQkFBTyxDQUFDLEdBQTJCLENBQUMsQ0FBQztBQUN6RSxPQUFNLENBQUMsT0FBTyxDQUFDLFlBQVksR0FBRyxtQkFBTyxDQUFDLEdBQW9CLENBQUMsQzs7Ozs7Ozs7Ozs7OztBQ04zRCxLQUFJLEtBQUssR0FBRyxtQkFBTyxDQUFDLENBQU8sQ0FBQyxDQUFDO0FBQzdCLEtBQUksQ0FBQyxHQUFHLG1CQUFPLENBQUMsRUFBUSxDQUFDLENBQUM7QUFDMUIsS0FBSSxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxDQUFhLENBQUMsQ0FBQztBQUNyQyxLQUFJLGdCQUFnQixHQUFHLG1CQUFPLENBQUMsRUFBaUMsQ0FBQyxDQUFDOztnQkFDbEQsbUJBQU8sQ0FBQyxHQUFrQixDQUFDOztLQUF0QyxPQUFPLFlBQVAsT0FBTzs7QUFDWixLQUFJLGNBQWMsR0FBRyxtQkFBTyxDQUFDLEdBQWtCLENBQUMsQ0FBQztBQUNqRCxLQUFJLEdBQUcsR0FBRyxtQkFBTyxDQUFDLEVBQVksQ0FBQyxDQUFDOztBQUVoQyxLQUFJLGNBQWMsR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFFckMsU0FBTSxFQUFFLENBQUMsZ0JBQWdCLENBQUM7O0FBRTFCLGtCQUFlLDZCQUFHO0FBQ2hCLFlBQU87QUFDTCxXQUFJLEVBQUUsRUFBRTtBQUNSLGVBQVEsRUFBRSxFQUFFO0FBQ1osWUFBSyxFQUFFLEVBQUU7TUFDVjtJQUNGOztBQUVELFVBQU8sRUFBRSxpQkFBUyxDQUFDLEVBQUU7QUFDbkIsTUFBQyxDQUFDLGNBQWMsRUFBRSxDQUFDO0FBQ25CLFNBQUksSUFBSSxDQUFDLE9BQU8sRUFBRSxFQUFFO0FBQ2xCLFdBQUksQ0FBQyxLQUFLLENBQUMsT0FBTyxDQUFDLElBQUksQ0FBQyxLQUFLLENBQUMsQ0FBQztNQUNoQztJQUNGOztBQUVELFVBQU8sRUFBRSxtQkFBVztBQUNsQixTQUFJLEtBQUssR0FBRyxDQUFDLENBQUMsSUFBSSxDQUFDLElBQUksQ0FBQyxJQUFJLENBQUMsQ0FBQztBQUM5QixZQUFPLEtBQUssQ0FBQyxNQUFNLEtBQUssQ0FBQyxJQUFJLEtBQUssQ0FBQyxLQUFLLEVBQUUsQ0FBQztJQUM1Qzs7QUFFRCxTQUFNLG9CQUFHO0FBQ1AsWUFDRTs7U0FBTSxHQUFHLEVBQUMsTUFBTSxFQUFDLFNBQVMsRUFBQyxzQkFBc0I7T0FDL0M7Ozs7UUFBOEI7T0FDOUI7O1dBQUssU0FBUyxFQUFDLEVBQUU7U0FDZjs7YUFBSyxTQUFTLEVBQUMsWUFBWTtXQUN6QiwrQkFBTyxTQUFTLFFBQUMsU0FBUyxFQUFFLElBQUksQ0FBQyxTQUFTLENBQUMsTUFBTSxDQUFFLEVBQUMsU0FBUyxFQUFDLHVCQUF1QixFQUFDLFdBQVcsRUFBQyxXQUFXLEVBQUMsSUFBSSxFQUFDLFVBQVUsR0FBRztVQUM1SDtTQUNOOzthQUFLLFNBQVMsRUFBQyxZQUFZO1dBQ3pCLCtCQUFPLFNBQVMsRUFBRSxJQUFJLENBQUMsU0FBUyxDQUFDLFVBQVUsQ0FBRSxFQUFDLElBQUksRUFBQyxVQUFVLEVBQUMsSUFBSSxFQUFDLFVBQVUsRUFBQyxTQUFTLEVBQUMsdUJBQXVCLEVBQUMsV0FBVyxFQUFDLFVBQVUsR0FBRTtVQUNwSTtTQUNOOzthQUFLLFNBQVMsRUFBQyxZQUFZO1dBQ3pCLCtCQUFPLFNBQVMsRUFBRSxJQUFJLENBQUMsU0FBUyxDQUFDLE9BQU8sQ0FBRSxFQUFDLFNBQVMsRUFBQyx1QkFBdUIsRUFBQyxJQUFJLEVBQUMsT0FBTyxFQUFDLFdBQVcsRUFBQyx5Q0FBeUMsR0FBRTtVQUM3STtTQUNOOzthQUFRLElBQUksRUFBQyxRQUFRLEVBQUMsU0FBUyxFQUFDLHNDQUFzQyxFQUFDLE9BQU8sRUFBRSxJQUFJLENBQUMsT0FBUTs7VUFBZTtRQUN4RztNQUNELENBQ1A7SUFDSDtFQUNGLENBQUM7O0FBRUYsS0FBSSxLQUFLLEdBQUcsS0FBSyxDQUFDLFdBQVcsQ0FBQzs7O0FBRTVCLFNBQU0sRUFBRSxDQUFDLE9BQU8sQ0FBQyxVQUFVLENBQUM7O0FBRTVCLGtCQUFlLDZCQUFHO0FBQ2hCLFlBQU8sRUFDTjtJQUNGOztBQUVELFVBQU8sbUJBQUMsU0FBUyxFQUFDO0FBQ2hCLFNBQUksR0FBRyxHQUFHLElBQUksQ0FBQyxLQUFLLENBQUMsUUFBUSxDQUFDO0FBQzlCLFNBQUksUUFBUSxHQUFHLEdBQUcsQ0FBQyxNQUFNLENBQUMsR0FBRyxDQUFDOztBQUU5QixTQUFHLEdBQUcsQ0FBQyxLQUFLLElBQUksR0FBRyxDQUFDLEtBQUssQ0FBQyxVQUFVLEVBQUM7QUFDbkMsZUFBUSxHQUFHLEdBQUcsQ0FBQyxLQUFLLENBQUMsVUFBVSxDQUFDO01BQ2pDOztBQUVELFlBQU8sQ0FBQyxLQUFLLENBQUMsU0FBUyxFQUFFLFFBQVEsQ0FBQyxDQUFDO0lBQ3BDOztBQUVELFNBQU0sb0JBQUc7QUFDUCxTQUFJLFlBQVksR0FBRyxLQUFLLENBQUM7QUFDekIsU0FBSSxPQUFPLEdBQUcsS0FBSyxDQUFDO0FBQ3BCLFlBQ0U7O1NBQUssU0FBUyxFQUFDLHVCQUF1QjtPQUNwQyw2QkFBSyxTQUFTLEVBQUMsZUFBZSxHQUFPO09BQ3JDOztXQUFLLFNBQVMsRUFBQyxzQkFBc0I7U0FDbkM7O2FBQUssU0FBUyxFQUFDLGlCQUFpQjtXQUM5QixvQkFBQyxjQUFjLElBQUMsT0FBTyxFQUFFLElBQUksQ0FBQyxPQUFRLEdBQUU7V0FDeEMsb0JBQUMsY0FBYyxPQUFFO1dBQ2pCOztlQUFLLFNBQVMsRUFBQyxnQkFBZ0I7YUFDN0IsMkJBQUcsU0FBUyxFQUFDLGdCQUFnQixHQUFLO2FBQ2xDOzs7O2NBQWdEO2FBQ2hEOzs7O2NBQTZEO1lBQ3pEO1VBQ0Y7UUFDRjtNQUNGLENBQ047SUFDSDtFQUNGLENBQUMsQ0FBQzs7QUFFSCxPQUFNLENBQUMsT0FBTyxHQUFHLEtBQUssQzs7Ozs7Ozs7Ozs7OztBQy9GdEIsS0FBSSxLQUFLLEdBQUcsbUJBQU8sQ0FBQyxDQUFPLENBQUMsQ0FBQzs7Z0JBQ1EsbUJBQU8sQ0FBQyxFQUFjLENBQUM7O0tBQXRELE1BQU0sWUFBTixNQUFNO0tBQUUsU0FBUyxZQUFULFNBQVM7S0FBRSxPQUFPLFlBQVAsT0FBTzs7QUFDaEMsS0FBSSxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxFQUEwQixDQUFDLENBQUM7QUFDbEQsS0FBSSxHQUFHLEdBQUcsbUJBQU8sQ0FBQyxFQUFZLENBQUMsQ0FBQzs7QUFFaEMsS0FBSSxTQUFTLEdBQUcsQ0FDZCxFQUFDLElBQUksRUFBRSxZQUFZLEVBQUUsRUFBRSxFQUFFLEdBQUcsQ0FBQyxNQUFNLENBQUMsS0FBSyxFQUFFLEtBQUssRUFBRSxPQUFPLEVBQUMsRUFDMUQsRUFBQyxJQUFJLEVBQUUsZUFBZSxFQUFFLEVBQUUsRUFBRSxHQUFHLENBQUMsTUFBTSxDQUFDLFFBQVEsRUFBRSxLQUFLLEVBQUUsVUFBVSxFQUFDLENBQ3BFLENBQUM7O0FBRUYsS0FBSSxVQUFVLEdBQUcsS0FBSyxDQUFDLFdBQVcsQ0FBQzs7O0FBRWpDLFNBQU0sRUFBRSxrQkFBVTs7O0FBQ2hCLFNBQUksS0FBSyxHQUFHLFNBQVMsQ0FBQyxHQUFHLENBQUMsVUFBQyxDQUFDLEVBQUUsS0FBSyxFQUFHO0FBQ3BDLFdBQUksU0FBUyxHQUFHLE1BQUssT0FBTyxDQUFDLE1BQU0sQ0FBQyxRQUFRLENBQUMsQ0FBQyxDQUFDLEVBQUUsQ0FBQyxHQUFHLFFBQVEsR0FBRyxFQUFFLENBQUM7QUFDbkUsY0FDRTs7V0FBSSxHQUFHLEVBQUUsS0FBTSxFQUFDLFNBQVMsRUFBRSxTQUFVO1NBQ25DO0FBQUMsb0JBQVM7YUFBQyxFQUFFLEVBQUUsQ0FBQyxDQUFDLEVBQUc7V0FDbEIsMkJBQUcsU0FBUyxFQUFFLENBQUMsQ0FBQyxJQUFLLEVBQUMsS0FBSyxFQUFFLENBQUMsQ0FBQyxLQUFNLEdBQUU7VUFDN0I7UUFDVCxDQUNMO01BQ0gsQ0FBQyxDQUFDOztBQUVILFVBQUssQ0FBQyxJQUFJLENBQ1I7O1NBQUksR0FBRyxFQUFFLFNBQVMsQ0FBQyxNQUFPO09BQ3hCOztXQUFHLElBQUksRUFBRSxHQUFHLENBQUMsT0FBUTtTQUNuQiwyQkFBRyxTQUFTLEVBQUMsZ0JBQWdCLEVBQUMsS0FBSyxFQUFDLE1BQU0sR0FBRTtRQUMxQztNQUNELENBQUUsQ0FBQzs7QUFFVixZQUNFOztTQUFLLFNBQVMsRUFBQywyQ0FBMkMsRUFBQyxJQUFJLEVBQUMsWUFBWTtPQUMxRTs7V0FBSyxTQUFTLEVBQUMsRUFBRTtTQUNmOzthQUFJLFNBQVMsRUFBQyxLQUFLLEVBQUMsRUFBRSxFQUFDLFdBQVc7V0FDaEM7OzthQUFJOztpQkFBSyxTQUFTLEVBQUMsMkJBQTJCO2VBQUM7OztpQkFBTyxpQkFBaUIsRUFBRTtnQkFBUTtjQUFNO1lBQUs7V0FDM0YsS0FBSztVQUNIO1FBQ0Q7TUFDRixDQUNOO0lBQ0g7RUFDRixDQUFDLENBQUM7O0FBRUgsV0FBVSxDQUFDLFlBQVksR0FBRztBQUN4QixTQUFNLEVBQUUsS0FBSyxDQUFDLFNBQVMsQ0FBQyxNQUFNLENBQUMsVUFBVTtFQUMxQzs7QUFFRCxVQUFTLGlCQUFpQixHQUFFOzJCQUNELE9BQU8sQ0FBQyxRQUFRLENBQUMsT0FBTyxDQUFDLElBQUksQ0FBQzs7T0FBbEQsZ0JBQWdCLHFCQUFoQixnQkFBZ0I7O0FBQ3JCLFVBQU8sZ0JBQWdCLENBQUM7RUFDekI7O0FBRUQsT0FBTSxDQUFDLE9BQU8sR0FBRyxVQUFVLEM7Ozs7Ozs7Ozs7Ozs7QUNyRDNCLEtBQUksS0FBSyxHQUFHLG1CQUFPLENBQUMsQ0FBTyxDQUFDLENBQUM7QUFDN0IsS0FBSSxDQUFDLEdBQUcsbUJBQU8sQ0FBQyxFQUFRLENBQUMsQ0FBQztBQUMxQixLQUFJLE9BQU8sR0FBRyxtQkFBTyxDQUFDLENBQWEsQ0FBQyxDQUFDOztnQkFDWixtQkFBTyxDQUFDLEdBQW9CLENBQUM7O0tBQWpELE9BQU8sWUFBUCxPQUFPO0tBQUUsT0FBTyxZQUFQLE9BQU87O0FBQ3JCLEtBQUksVUFBVSxHQUFHLG1CQUFPLENBQUMsR0FBa0IsQ0FBQyxDQUFDO0FBQzdDLEtBQUksZ0JBQWdCLEdBQUcsbUJBQU8sQ0FBQyxFQUFpQyxDQUFDLENBQUM7QUFDbEUsS0FBSSxjQUFjLEdBQUcsbUJBQU8sQ0FBQyxHQUFrQixDQUFDLENBQUM7O0FBRWpELEtBQUksZUFBZSxHQUFHLEtBQUssQ0FBQyxXQUFXLENBQUM7OztBQUV0QyxTQUFNLEVBQUUsQ0FBQyxnQkFBZ0IsQ0FBQzs7QUFFMUIsb0JBQWlCLCtCQUFFO0FBQ2pCLE1BQUMsQ0FBQyxJQUFJLENBQUMsSUFBSSxDQUFDLElBQUksQ0FBQyxDQUFDLFFBQVEsQ0FBQztBQUN6QixZQUFLLEVBQUM7QUFDSixpQkFBUSxFQUFDO0FBQ1Asb0JBQVMsRUFBRSxDQUFDO0FBQ1osbUJBQVEsRUFBRSxJQUFJO1VBQ2Y7QUFDRCwwQkFBaUIsRUFBQztBQUNoQixtQkFBUSxFQUFFLElBQUk7QUFDZCxrQkFBTyxFQUFFLElBQUksQ0FBQyxJQUFJLENBQUMsUUFBUTtVQUM1QjtRQUNGOztBQUVELGVBQVEsRUFBRTtBQUNYLDBCQUFpQixFQUFFO0FBQ2xCLG9CQUFTLEVBQUUsQ0FBQyxDQUFDLFNBQVMsQ0FBQyxNQUFNLENBQUMsK0JBQStCLENBQUM7QUFDOUQsa0JBQU8sRUFBRSxrQ0FBa0M7VUFDM0M7UUFDQztNQUNGLENBQUM7SUFDSDs7QUFFRCxrQkFBZSw2QkFBRztBQUNoQixZQUFPO0FBQ0wsV0FBSSxFQUFFLElBQUksQ0FBQyxLQUFLLENBQUMsTUFBTSxDQUFDLElBQUk7QUFDNUIsVUFBRyxFQUFFLEVBQUU7QUFDUCxtQkFBWSxFQUFFLEVBQUU7QUFDaEIsWUFBSyxFQUFFLEVBQUU7TUFDVjtJQUNGOztBQUVELFVBQU8sbUJBQUMsQ0FBQyxFQUFFO0FBQ1QsTUFBQyxDQUFDLGNBQWMsRUFBRSxDQUFDO0FBQ25CLFNBQUksSUFBSSxDQUFDLE9BQU8sRUFBRSxFQUFFO0FBQ2xCLGlCQUFVLENBQUMsT0FBTyxDQUFDLE1BQU0sQ0FBQztBQUN4QixhQUFJLEVBQUUsSUFBSSxDQUFDLEtBQUssQ0FBQyxJQUFJO0FBQ3JCLFlBQUcsRUFBRSxJQUFJLENBQUMsS0FBSyxDQUFDLEdBQUc7QUFDbkIsY0FBSyxFQUFFLElBQUksQ0FBQyxLQUFLLENBQUMsS0FBSztBQUN2QixvQkFBVyxFQUFFLElBQUksQ0FBQyxLQUFLLENBQUMsTUFBTSxDQUFDLFlBQVksRUFBQyxDQUFDLENBQUM7TUFDakQ7SUFDRjs7QUFFRCxVQUFPLHFCQUFHO0FBQ1IsU0FBSSxLQUFLLEdBQUcsQ0FBQyxDQUFDLElBQUksQ0FBQyxJQUFJLENBQUMsSUFBSSxDQUFDLENBQUM7QUFDOUIsWUFBTyxLQUFLLENBQUMsTUFBTSxLQUFLLENBQUMsSUFBSSxLQUFLLENBQUMsS0FBSyxFQUFFLENBQUM7SUFDNUM7O0FBRUQsU0FBTSxvQkFBRztBQUNQLFlBQ0U7O1NBQU0sR0FBRyxFQUFDLE1BQU0sRUFBQyxTQUFTLEVBQUMsdUJBQXVCO09BQ2hEOzs7O1FBQW9DO09BQ3BDOztXQUFLLFNBQVMsRUFBQyxFQUFFO1NBQ2Y7O2FBQUssU0FBUyxFQUFDLFlBQVk7V0FDekI7QUFDRSxzQkFBUyxFQUFFLElBQUksQ0FBQyxTQUFTLENBQUMsTUFBTSxDQUFFO0FBQ2xDLGlCQUFJLEVBQUMsVUFBVTtBQUNmLHNCQUFTLEVBQUMsdUJBQXVCO0FBQ2pDLHdCQUFXLEVBQUMsV0FBVyxHQUFFO1VBQ3ZCO1NBQ047O2FBQUssU0FBUyxFQUFDLFlBQVk7V0FDekI7QUFDRSxzQkFBUztBQUNULHNCQUFTLEVBQUUsSUFBSSxDQUFDLFNBQVMsQ0FBQyxLQUFLLENBQUU7QUFDakMsZ0JBQUcsRUFBQyxVQUFVO0FBQ2QsaUJBQUksRUFBQyxVQUFVO0FBQ2YsaUJBQUksRUFBQyxVQUFVO0FBQ2Ysc0JBQVMsRUFBQyxjQUFjO0FBQ3hCLHdCQUFXLEVBQUMsVUFBVSxHQUFHO1VBQ3ZCO1NBQ047O2FBQUssU0FBUyxFQUFDLFlBQVk7V0FDekI7QUFDRSxzQkFBUyxFQUFFLElBQUksQ0FBQyxTQUFTLENBQUMsY0FBYyxDQUFFO0FBQzFDLGlCQUFJLEVBQUMsVUFBVTtBQUNmLGlCQUFJLEVBQUMsbUJBQW1CO0FBQ3hCLHNCQUFTLEVBQUMsY0FBYztBQUN4Qix3QkFBVyxFQUFDLGtCQUFrQixHQUFFO1VBQzlCO1NBQ047O2FBQUssU0FBUyxFQUFDLFlBQVk7V0FDekI7QUFDRSxpQkFBSSxFQUFDLE9BQU87QUFDWixzQkFBUyxFQUFFLElBQUksQ0FBQyxTQUFTLENBQUMsT0FBTyxDQUFFO0FBQ25DLHNCQUFTLEVBQUMsdUJBQXVCO0FBQ2pDLHdCQUFXLEVBQUMseUNBQXlDLEdBQUc7VUFDdEQ7U0FDTjs7YUFBUSxJQUFJLEVBQUMsUUFBUSxFQUFDLFFBQVEsRUFBRSxJQUFJLENBQUMsS0FBSyxDQUFDLE1BQU0sQ0FBQyxZQUFhLEVBQUMsU0FBUyxFQUFDLHNDQUFzQyxFQUFDLE9BQU8sRUFBRSxJQUFJLENBQUMsT0FBUTs7VUFBa0I7UUFDcko7TUFDRCxDQUNQO0lBQ0g7RUFDRixDQUFDOztBQUVGLEtBQUksTUFBTSxHQUFHLEtBQUssQ0FBQyxXQUFXLENBQUM7OztBQUU3QixTQUFNLEVBQUUsQ0FBQyxPQUFPLENBQUMsVUFBVSxDQUFDOztBQUU1QixrQkFBZSw2QkFBRztBQUNoQixZQUFPO0FBQ0wsYUFBTSxFQUFFLE9BQU8sQ0FBQyxNQUFNO0FBQ3RCLGFBQU0sRUFBRSxPQUFPLENBQUMsTUFBTTtNQUN2QjtJQUNGOztBQUVELG9CQUFpQiwrQkFBRTtBQUNqQixZQUFPLENBQUMsV0FBVyxDQUFDLElBQUksQ0FBQyxLQUFLLENBQUMsTUFBTSxDQUFDLFdBQVcsQ0FBQyxDQUFDO0lBQ3BEOztBQUVELFNBQU0sRUFBRSxrQkFBVztBQUNqQixTQUFHLENBQUMsSUFBSSxDQUFDLEtBQUssQ0FBQyxNQUFNLEVBQUU7QUFDckIsY0FBTyxJQUFJLENBQUM7TUFDYjs7QUFFRCxZQUNFOztTQUFLLFNBQVMsRUFBQyx3QkFBd0I7T0FDckMsNkJBQUssU0FBUyxFQUFDLGVBQWUsR0FBTztPQUNyQzs7V0FBSyxTQUFTLEVBQUMsc0JBQXNCO1NBQ25DOzthQUFLLFNBQVMsRUFBQyxpQkFBaUI7V0FDOUIsb0JBQUMsZUFBZSxJQUFDLE1BQU0sRUFBRSxJQUFJLENBQUMsS0FBSyxDQUFDLE1BQU8sRUFBQyxNQUFNLEVBQUUsSUFBSSxDQUFDLEtBQUssQ0FBQyxNQUFNLENBQUMsSUFBSSxFQUFHLEdBQUU7V0FDL0Usb0JBQUMsY0FBYyxPQUFFO1VBQ2I7U0FDTjs7YUFBSyxTQUFTLEVBQUMsb0NBQW9DO1dBQ2pEOzs7O2FBQWlDLCtCQUFLOzthQUFDOzs7O2NBQTJEO1lBQUs7V0FDdkcsNkJBQUssU0FBUyxFQUFDLGVBQWUsRUFBQyxHQUFHLDZCQUE0QixJQUFJLENBQUMsS0FBSyxDQUFDLE1BQU0sQ0FBQyxHQUFHLENBQUMsSUFBSSxDQUFLLEdBQUc7VUFDNUY7UUFDRjtNQUNGLENBQ047SUFDSDtFQUNGLENBQUMsQ0FBQzs7QUFFSCxPQUFNLENBQUMsT0FBTyxHQUFHLE1BQU0sQzs7Ozs7Ozs7Ozs7OztBQzdJdkIsS0FBSSxLQUFLLEdBQUcsbUJBQU8sQ0FBQyxDQUFPLENBQUMsQ0FBQztBQUM3QixLQUFJLE9BQU8sR0FBRyxtQkFBTyxDQUFDLENBQWEsQ0FBQyxDQUFDOztnQkFDckIsbUJBQU8sQ0FBQyxHQUFxQixDQUFDOztLQUF6QyxPQUFPLFlBQVAsT0FBTzs7aUJBQ2tCLG1CQUFPLENBQUMsRUFBNkIsQ0FBQzs7S0FBL0QscUJBQXFCLGFBQXJCLHFCQUFxQjs7aUJBQ0wsbUJBQU8sQ0FBQyxFQUFvQyxDQUFDOztLQUE3RCxZQUFZLGFBQVosWUFBWTs7QUFDakIsS0FBSSxRQUFRLEdBQUcsbUJBQU8sQ0FBQyxHQUFrQixDQUFDLENBQUM7O0FBRTNDLEtBQUksZ0JBQWdCLEdBQUcsS0FBSyxDQUFDLFdBQVcsQ0FBQzs7O0FBRXZDLFNBQU0sRUFBRSxDQUFDLE9BQU8sQ0FBQyxVQUFVLENBQUM7O0FBRTVCLGtCQUFlLDZCQUFHO0FBQ2hCLFlBQU87QUFDTCxjQUFPLEVBQUUsT0FBTyxDQUFDLE9BQU87TUFDekI7SUFDRjs7QUFFRCxTQUFNLG9CQUFHO0FBQ1AsWUFBTyxJQUFJLENBQUMsS0FBSyxDQUFDLE9BQU8sQ0FBQyxzQkFBc0IsR0FBRyxvQkFBQyxNQUFNLE9BQUUsR0FBRyxJQUFJLENBQUM7SUFDckU7RUFDRixDQUFDLENBQUM7O0FBRUgsS0FBSSxNQUFNLEdBQUcsS0FBSyxDQUFDLFdBQVcsQ0FBQzs7O0FBRTdCLGVBQVksd0JBQUMsUUFBUSxFQUFFLEtBQUssRUFBQztBQUMzQixpQkFBWSxDQUFDLFFBQVEsRUFBRSxLQUFLLENBQUMsQ0FBQztBQUM5QiwwQkFBcUIsRUFBRSxDQUFDO0lBQ3pCOztBQUVELHVCQUFvQixnQ0FBQyxRQUFRLEVBQUM7QUFDNUIsTUFBQyxDQUFDLFFBQVEsQ0FBQyxDQUFDLEtBQUssQ0FBQyxNQUFNLENBQUMsQ0FBQztJQUMzQjs7QUFFRCxvQkFBaUIsK0JBQUU7QUFDakIsTUFBQyxDQUFDLFFBQVEsQ0FBQyxDQUFDLEtBQUssQ0FBQyxNQUFNLENBQUMsQ0FBQztJQUMzQjs7QUFFRCxTQUFNLG9CQUFHO0FBQ1AsWUFDRTs7U0FBSyxTQUFTLEVBQUMsbUNBQW1DLEVBQUMsUUFBUSxFQUFFLENBQUMsQ0FBRSxFQUFDLElBQUksRUFBQyxRQUFRO09BQzVFOztXQUFLLFNBQVMsRUFBQyxjQUFjO1NBQzNCOzthQUFLLFNBQVMsRUFBQyxlQUFlO1dBQzVCLDZCQUFLLFNBQVMsRUFBQyxjQUFjLEdBQ3ZCO1dBQ047O2VBQUssU0FBUyxFQUFDLFlBQVk7YUFDekIsb0JBQUMsUUFBUSxJQUFDLFlBQVksRUFBRSxJQUFJLENBQUMsWUFBYSxHQUFFO1lBQ3hDO1dBQ047O2VBQUssU0FBUyxFQUFDLGNBQWM7YUFDM0I7O2lCQUFRLE9BQU8sRUFBRSxxQkFBc0IsRUFBQyxJQUFJLEVBQUMsUUFBUSxFQUFDLFNBQVMsRUFBQyxpQkFBaUI7O2NBRXhFO1lBQ0w7VUFDRjtRQUNGO01BQ0YsQ0FDTjtJQUNIO0VBQ0YsQ0FBQyxDQUFDOztBQUVILE9BQU0sQ0FBQyxPQUFPLEdBQUcsZ0JBQWdCLEM7Ozs7Ozs7Ozs7Ozs7OztBQzNEakMsS0FBSSxLQUFLLEdBQUcsbUJBQU8sQ0FBQyxDQUFPLENBQUMsQ0FBQztBQUM3QixLQUFJLE9BQU8sR0FBRyxtQkFBTyxDQUFDLENBQWEsQ0FBQyxDQUFDOztnQkFDdEIsbUJBQU8sQ0FBQyxFQUFjLENBQUM7O0tBQWhDLElBQUksWUFBSixJQUFJOztpQkFDNEIsbUJBQU8sQ0FBQyxHQUEwQixDQUFDOztLQUFwRSxLQUFLLGFBQUwsS0FBSztLQUFFLE1BQU0sYUFBTixNQUFNO0tBQUUsSUFBSSxhQUFKLElBQUk7S0FBRSxRQUFRLGFBQVIsUUFBUTs7aUJBQ2xCLG1CQUFPLENBQUMsR0FBc0IsQ0FBQzs7S0FBMUMsT0FBTyxhQUFQLE9BQU87O2lCQUNDLG1CQUFPLENBQUMsRUFBb0MsQ0FBQzs7S0FBckQsSUFBSSxhQUFKLElBQUk7O0FBQ1QsS0FBSSxNQUFNLEdBQUksbUJBQU8sQ0FBQyxDQUFRLENBQUMsQ0FBQzs7QUFFaEMsS0FBTSxlQUFlLEdBQUcsU0FBbEIsZUFBZSxDQUFJLElBQTRCLEVBQUs7T0FBL0IsUUFBUSxHQUFWLElBQTRCLENBQTFCLFFBQVE7T0FBRSxJQUFJLEdBQWhCLElBQTRCLENBQWhCLElBQUk7O09BQUssS0FBSyw0QkFBMUIsSUFBNEI7O0FBQ25ELE9BQUksT0FBTyxHQUFHLElBQUksQ0FBQyxRQUFRLENBQUMsQ0FBQyxPQUFPLENBQUM7QUFDckMsT0FBSSxXQUFXLEdBQUcsTUFBTSxDQUFDLE9BQU8sQ0FBQyxDQUFDLE9BQU8sRUFBRSxDQUFDO0FBQzVDLFVBQ0U7QUFBQyxTQUFJO0tBQUssS0FBSztLQUNYLFdBQVc7SUFDUixDQUNSO0VBQ0YsQ0FBQzs7QUFFRixLQUFNLFlBQVksR0FBRyxTQUFmLFlBQVksQ0FBSSxLQUE0QixFQUFLO09BQS9CLFFBQVEsR0FBVixLQUE0QixDQUExQixRQUFRO09BQUUsSUFBSSxHQUFoQixLQUE0QixDQUFoQixJQUFJOztPQUFLLEtBQUssNEJBQTFCLEtBQTRCOztBQUNoRCxPQUFJLE9BQU8sR0FBRyxJQUFJLENBQUMsUUFBUSxDQUFDLENBQUMsT0FBTyxDQUFDO0FBQ3JDLE9BQUksVUFBVSxHQUFHLElBQUksQ0FBQyxRQUFRLENBQUMsQ0FBQyxVQUFVLENBQUM7O0FBRTNDLE9BQUksR0FBRyxHQUFHLE1BQU0sQ0FBQyxPQUFPLENBQUMsQ0FBQztBQUMxQixPQUFJLEdBQUcsR0FBRyxNQUFNLENBQUMsVUFBVSxDQUFDLENBQUM7QUFDN0IsT0FBSSxRQUFRLEdBQUcsTUFBTSxDQUFDLFFBQVEsQ0FBQyxHQUFHLENBQUMsSUFBSSxDQUFDLEdBQUcsQ0FBQyxDQUFDLENBQUM7QUFDOUMsT0FBSSxXQUFXLEdBQUcsUUFBUSxDQUFDLFFBQVEsRUFBRSxDQUFDOztBQUV0QyxVQUNFO0FBQUMsU0FBSTtLQUFLLEtBQUs7S0FDWCxXQUFXO0lBQ1IsQ0FDUjtFQUNGLENBQUM7O0FBRUYsS0FBTSxTQUFTLEdBQUcsU0FBWixTQUFTLENBQUksS0FBNEIsRUFBSztPQUEvQixRQUFRLEdBQVYsS0FBNEIsQ0FBMUIsUUFBUTtPQUFFLElBQUksR0FBaEIsS0FBNEIsQ0FBaEIsSUFBSTs7T0FBSyxLQUFLLDRCQUExQixLQUE0Qjs7QUFDN0MsT0FBSSxNQUFNLEdBQUcsSUFBSSxDQUFDLFFBQVEsQ0FBQyxDQUFDLE9BQU8sQ0FBQyxHQUFHLENBQUMsVUFBQyxJQUFJLEVBQUUsU0FBUztZQUNyRDs7U0FBTSxHQUFHLEVBQUUsU0FBVSxFQUFDLEtBQUssRUFBRSxFQUFDLGVBQWUsRUFBRSxTQUFTLEVBQUUsRUFBQyxTQUFTLEVBQUMsZ0RBQWdEO09BQUUsSUFBSSxDQUFDLElBQUksQ0FBQyxDQUFDLENBQUM7TUFBUTtJQUFDLENBQzlJOztBQUVELFVBQ0U7QUFBQyxTQUFJO0tBQUssS0FBSztLQUNiOzs7T0FDRyxNQUFNO01BQ0g7SUFDRCxDQUNSO0VBQ0YsQ0FBQzs7QUFFRixLQUFNLFVBQVUsR0FBRyxTQUFiLFVBQVUsQ0FBSSxLQUE0QixFQUFLO09BQS9CLFFBQVEsR0FBVixLQUE0QixDQUExQixRQUFRO09BQUUsSUFBSSxHQUFoQixLQUE0QixDQUFoQixJQUFJOztPQUFLLEtBQUssNEJBQTFCLEtBQTRCOzt3QkFDakIsSUFBSSxDQUFDLFFBQVEsQ0FBQztPQUFyQyxVQUFVLGtCQUFWLFVBQVU7T0FBRSxNQUFNLGtCQUFOLE1BQU07O2VBQ1EsTUFBTSxHQUFHLENBQUMsTUFBTSxFQUFFLGFBQWEsQ0FBQyxHQUFHLENBQUMsTUFBTSxFQUFFLGFBQWEsQ0FBQzs7T0FBckYsVUFBVTtPQUFFLFdBQVc7O0FBQzVCLFVBQ0U7QUFBQyxTQUFJO0tBQUssS0FBSztLQUNiO0FBQUMsV0FBSTtTQUFDLEVBQUUsRUFBRSxVQUFXLEVBQUMsU0FBUyxFQUFFLE1BQU0sR0FBRSxXQUFXLEdBQUUsU0FBVSxFQUFDLElBQUksRUFBQyxRQUFRO09BQUUsVUFBVTtNQUFRO0lBQzdGLENBQ1I7RUFDRjs7QUFFRCxLQUFJLFdBQVcsR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFFbEMsU0FBTSxFQUFFLENBQUMsT0FBTyxDQUFDLFVBQVUsQ0FBQzs7QUFFNUIsa0JBQWUsNkJBQUc7QUFDaEIsWUFBTztBQUNMLG1CQUFZLEVBQUUsT0FBTyxDQUFDLFlBQVk7TUFDbkM7SUFDRjs7QUFFRCxTQUFNLEVBQUUsa0JBQVc7QUFDakIsU0FBSSxJQUFJLEdBQUcsSUFBSSxDQUFDLEtBQUssQ0FBQyxZQUFZLENBQUM7QUFDbkMsWUFDRTs7U0FBSyxTQUFTLEVBQUMsY0FBYztPQUMzQjs7OztRQUFrQjtPQUNsQjs7V0FBSyxTQUFTLEVBQUMsRUFBRTtTQUNmOzthQUFLLFNBQVMsRUFBQyxFQUFFO1dBQ2Y7O2VBQUssU0FBUyxFQUFDLEVBQUU7YUFDZjtBQUFDLG9CQUFLO2lCQUFDLFFBQVEsRUFBRSxJQUFJLENBQUMsTUFBTyxFQUFDLFNBQVMsRUFBQyxlQUFlO2VBQ3JELG9CQUFDLE1BQU07QUFDTCwwQkFBUyxFQUFDLEtBQUs7QUFDZix1QkFBTSxFQUFFO0FBQUMsdUJBQUk7OztrQkFBc0I7QUFDbkMscUJBQUksRUFBRSxvQkFBQyxRQUFRLElBQUMsSUFBSSxFQUFFLElBQUssR0FBSTtpQkFDL0I7ZUFDRixvQkFBQyxNQUFNO0FBQ0wsdUJBQU0sRUFBRTtBQUFDLHVCQUFJOzs7a0JBQVc7QUFDeEIscUJBQUksRUFDRixvQkFBQyxVQUFVLElBQUMsSUFBSSxFQUFFLElBQUssR0FDeEI7aUJBQ0Q7ZUFDRixvQkFBQyxNQUFNO0FBQ0wsMEJBQVMsRUFBQyxVQUFVO0FBQ3BCLHVCQUFNLEVBQUU7QUFBQyx1QkFBSTs7O2tCQUFnQjtBQUM3QixxQkFBSSxFQUFFLG9CQUFDLFFBQVEsSUFBQyxJQUFJLEVBQUUsSUFBSyxHQUFLO2lCQUNoQztlQUNGLG9CQUFDLE1BQU07QUFDTCwwQkFBUyxFQUFDLFNBQVM7QUFDbkIsdUJBQU0sRUFBRTtBQUFDLHVCQUFJOzs7a0JBQW1CO0FBQ2hDLHFCQUFJLEVBQUUsb0JBQUMsZUFBZSxJQUFDLElBQUksRUFBRSxJQUFLLEdBQUk7aUJBQ3RDO2VBQ0Ysb0JBQUMsTUFBTTtBQUNMLDBCQUFTLEVBQUMsVUFBVTtBQUNwQix1QkFBTSxFQUFFO0FBQUMsdUJBQUk7OztrQkFBa0I7QUFDL0IscUJBQUksRUFBRSxvQkFBQyxTQUFTLElBQUMsSUFBSSxFQUFFLElBQUssR0FBSztpQkFDakM7Y0FDSTtZQUNKO1VBQ0Y7UUFDRjtNQUNGLENBQ1A7SUFDRjtFQUNGLENBQUMsQ0FBQzs7QUFFSCxPQUFNLENBQUMsT0FBTyxHQUFHLFdBQVcsQzs7Ozs7Ozs7Ozs7OztBQ2hINUIsS0FBSSxLQUFLLEdBQUcsbUJBQU8sQ0FBQyxDQUFPLENBQUMsQ0FBQztBQUM3QixLQUFJLE1BQU0sR0FBRyxtQkFBTyxDQUFDLEdBQVcsQ0FBQyxDQUFDLE1BQU0sQ0FBQzs7Z0JBQ3FCLG1CQUFPLENBQUMsRUFBYyxDQUFDOztLQUEvRSxNQUFNLFlBQU4sTUFBTTtLQUFFLEtBQUssWUFBTCxLQUFLO0tBQUUsUUFBUSxZQUFSLFFBQVE7S0FBRSxVQUFVLFlBQVYsVUFBVTtLQUFFLGNBQWMsWUFBZCxjQUFjOztpQkFDd0IsbUJBQU8sQ0FBQyxHQUFjLENBQUM7O0tBQWxHLEdBQUcsYUFBSCxHQUFHO0tBQUUsS0FBSyxhQUFMLEtBQUs7S0FBRSxLQUFLLGFBQUwsS0FBSztLQUFFLFFBQVEsYUFBUixRQUFRO0tBQUUsT0FBTyxhQUFQLE9BQU87S0FBRSxrQkFBa0IsYUFBbEIsa0JBQWtCO0tBQUUsWUFBWSxhQUFaLFlBQVk7O2lCQUN6RCxtQkFBTyxDQUFDLEdBQXdCLENBQUM7O0tBQS9DLFVBQVUsYUFBVixVQUFVOztBQUNmLEtBQUksSUFBSSxHQUFHLG1CQUFPLENBQUMsRUFBUSxDQUFDLENBQUM7QUFDN0IsS0FBSSxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxFQUFXLENBQUMsQ0FBQztBQUNuQyxLQUFJLEdBQUcsR0FBRyxtQkFBTyxDQUFDLEVBQVUsQ0FBQyxDQUFDOztBQUU5QixvQkFBTyxDQUFDLEdBQVcsQ0FBQyxDQUFDOzs7QUFHckIsUUFBTyxDQUFDLElBQUksRUFBRSxDQUFDOztBQUVmLFVBQVMsWUFBWSxDQUFDLFNBQVMsRUFBRSxPQUFPLEVBQUUsRUFBRSxFQUFDO0FBQzNDLE9BQUksQ0FBQyxNQUFNLEVBQUUsQ0FBQztFQUNmOztBQUVELE9BQU0sQ0FDSjtBQUFDLFNBQU07S0FBQyxPQUFPLEVBQUUsT0FBTyxDQUFDLFVBQVUsRUFBRztHQUNwQyxvQkFBQyxLQUFLLElBQUMsSUFBSSxFQUFFLEdBQUcsQ0FBQyxNQUFNLENBQUMsS0FBTSxFQUFDLFNBQVMsRUFBRSxLQUFNLEdBQUU7R0FDbEQsb0JBQUMsS0FBSyxJQUFDLElBQUksRUFBRSxHQUFHLENBQUMsTUFBTSxDQUFDLE1BQU8sRUFBQyxPQUFPLEVBQUUsWUFBYSxHQUFFO0dBQ3hELG9CQUFDLEtBQUssSUFBQyxJQUFJLEVBQUUsR0FBRyxDQUFDLE1BQU0sQ0FBQyxPQUFRLEVBQUMsU0FBUyxFQUFFLE9BQVEsR0FBRTtHQUN0RCxvQkFBQyxRQUFRLElBQUMsSUFBSSxFQUFFLEdBQUcsQ0FBQyxNQUFNLENBQUMsR0FBSSxFQUFDLEVBQUUsRUFBRSxHQUFHLENBQUMsTUFBTSxDQUFDLEtBQU0sR0FBRTtHQUN2RDtBQUFDLFVBQUs7T0FBQyxJQUFJLEVBQUUsR0FBRyxDQUFDLE1BQU0sQ0FBQyxHQUFJLEVBQUMsU0FBUyxFQUFFLEdBQUksRUFBQyxPQUFPLEVBQUUsVUFBVztLQUMvRCxvQkFBQyxLQUFLLElBQUMsSUFBSSxFQUFFLEdBQUcsQ0FBQyxNQUFNLENBQUMsS0FBTSxFQUFDLFNBQVMsRUFBRSxLQUFNLEdBQUU7S0FDbEQsb0JBQUMsS0FBSyxJQUFDLElBQUksRUFBRSxHQUFHLENBQUMsTUFBTSxDQUFDLGFBQWMsRUFBQyxVQUFVLEVBQUUsRUFBQyxrQkFBa0IsRUFBRSxrQkFBa0IsRUFBRSxHQUFFO0tBQzlGLG9CQUFDLEtBQUssSUFBQyxJQUFJLEVBQUUsR0FBRyxDQUFDLE1BQU0sQ0FBQyxRQUFTLEVBQUMsU0FBUyxFQUFFLFFBQVMsR0FBRTtJQUNsRDtHQUNSLG9CQUFDLEtBQUssSUFBQyxJQUFJLEVBQUMsR0FBRyxFQUFDLFNBQVMsRUFBRSxZQUFhLEdBQUc7RUFDcEMsRUFDUixRQUFRLENBQUMsY0FBYyxDQUFDLEtBQUssQ0FBQyxDQUFDLEM7Ozs7Ozs7OztBQy9CbEMsMkI7Ozs7Ozs7QUNBQSxvQiIsImZpbGUiOiJhcHAuanMiLCJzb3VyY2VzQ29udGVudCI6WyJpbXBvcnQgeyBSZWFjdG9yIH0gZnJvbSAnbnVjbGVhci1qcydcblxuY29uc3QgcmVhY3RvciA9IG5ldyBSZWFjdG9yKHtcbiAgZGVidWc6IHRydWVcbn0pXG5cbndpbmRvdy5yZWFjdG9yID0gcmVhY3RvcjtcblxuZXhwb3J0IGRlZmF1bHQgcmVhY3RvclxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL3JlYWN0b3IuanNcbiAqKi8iLCJsZXQge2Zvcm1hdFBhdHRlcm59ID0gcmVxdWlyZSgnYXBwL2NvbW1vbi9wYXR0ZXJuVXRpbHMnKTtcblxubGV0IGNmZyA9IHtcblxuICBiYXNlVXJsOiB3aW5kb3cubG9jYXRpb24ub3JpZ2luLFxuXG4gIGhlbHBVcmw6ICdodHRwczovL2dpdGh1Yi5jb20vZ3Jhdml0YXRpb25hbC90ZWxlcG9ydC9ibG9iL21hc3Rlci9SRUFETUUubWQnLFxuXG4gIGFwaToge1xuICAgIHJlbmV3VG9rZW5QYXRoOicvdjEvd2ViYXBpL3Nlc3Npb25zL3JlbmV3JyxcbiAgICBub2Rlc1BhdGg6ICcvdjEvd2ViYXBpL3NpdGVzLy1jdXJyZW50LS9ub2RlcycsXG4gICAgc2Vzc2lvblBhdGg6ICcvdjEvd2ViYXBpL3Nlc3Npb25zJyxcbiAgICBzaXRlU2Vzc2lvblBhdGg6ICcvdjEvd2ViYXBpL3NpdGVzLy1jdXJyZW50LS9zZXNzaW9ucy86c2lkJyxcbiAgICBpbnZpdGVQYXRoOiAnL3YxL3dlYmFwaS91c2Vycy9pbnZpdGVzLzppbnZpdGVUb2tlbicsXG4gICAgY3JlYXRlVXNlclBhdGg6ICcvdjEvd2ViYXBpL3VzZXJzJyxcbiAgICBzZXNzaW9uQ2h1bms6ICcvdjEvd2ViYXBpL3NpdGVzLy1jdXJyZW50LS9zZXNzaW9ucy86c2lkL2NodW5rcz9zdGFydD06c3RhcnQmZW5kPTplbmQnLFxuICAgIHNlc3Npb25DaHVua0NvdW50UGF0aDogJy92MS93ZWJhcGkvc2l0ZXMvLWN1cnJlbnQtL3Nlc3Npb25zLzpzaWQvY2h1bmtzY291bnQnLFxuXG4gICAgZ2V0RmV0Y2hTZXNzaW9uQ2h1bmtVcmw6ICh7c2lkLCBzdGFydCwgZW5kfSk9PntcbiAgICAgIHJldHVybiBmb3JtYXRQYXR0ZXJuKGNmZy5hcGkuc2Vzc2lvbkNodW5rLCB7c2lkLCBzdGFydCwgZW5kfSk7XG4gICAgfSxcblxuICAgIGdldEZldGNoU2Vzc2lvbkxlbmd0aFVybDogKHNpZCk9PntcbiAgICAgIHJldHVybiBmb3JtYXRQYXR0ZXJuKGNmZy5hcGkuc2Vzc2lvbkNodW5rQ291bnRQYXRoLCB7c2lkfSk7XG4gICAgfSxcblxuICAgIGdldEZldGNoU2Vzc2lvbnNVcmw6ICgvKnN0YXJ0LCBlbmQqLyk9PntcbiAgICAgIHZhciBwYXJhbXMgPSB7XG4gICAgICAgIHN0YXJ0OiBuZXcgRGF0ZSgpLnRvSVNPU3RyaW5nKCksXG4gICAgICAgIG9yZGVyOiAtMVxuICAgICAgfTtcblxuICAgICAgdmFyIGpzb24gPSBKU09OLnN0cmluZ2lmeShwYXJhbXMpO1xuICAgICAgdmFyIGpzb25FbmNvZGVkID0gd2luZG93LmVuY29kZVVSSShqc29uKTtcblxuICAgICAgcmV0dXJuIGAvdjEvd2ViYXBpL3NpdGVzLy1jdXJyZW50LS9ldmVudHMvc2Vzc2lvbnM/ZmlsdGVyPSR7anNvbkVuY29kZWR9YDtcbiAgICB9LFxuXG4gICAgZ2V0RmV0Y2hTZXNzaW9uVXJsOiAoc2lkKT0+e1xuICAgICAgcmV0dXJuIGZvcm1hdFBhdHRlcm4oY2ZnLmFwaS5zaXRlU2Vzc2lvblBhdGgsIHtzaWR9KTtcbiAgICB9LFxuXG4gICAgZ2V0VGVybWluYWxTZXNzaW9uVXJsOiAoc2lkKT0+IHtcbiAgICAgIHJldHVybiBmb3JtYXRQYXR0ZXJuKGNmZy5hcGkuc2l0ZVNlc3Npb25QYXRoLCB7c2lkfSk7XG4gICAgfSxcblxuICAgIGdldEludml0ZVVybDogKGludml0ZVRva2VuKSA9PiB7XG4gICAgICByZXR1cm4gZm9ybWF0UGF0dGVybihjZmcuYXBpLmludml0ZVBhdGgsIHtpbnZpdGVUb2tlbn0pO1xuICAgIH0sXG5cbiAgICBnZXRFdmVudFN0cmVhbUNvbm5TdHI6ICh0b2tlbiwgc2lkKSA9PiB7XG4gICAgICB2YXIgaG9zdG5hbWUgPSBnZXRXc0hvc3ROYW1lKCk7XG4gICAgICByZXR1cm4gYCR7aG9zdG5hbWV9L3YxL3dlYmFwaS9zaXRlcy8tY3VycmVudC0vc2Vzc2lvbnMvJHtzaWR9L2V2ZW50cy9zdHJlYW0/YWNjZXNzX3Rva2VuPSR7dG9rZW59YDtcbiAgICB9LFxuXG4gICAgZ2V0VHR5Q29ublN0cjogKHt0b2tlbiwgc2VydmVySWQsIGxvZ2luLCBzaWQsIHJvd3MsIGNvbHN9KSA9PiB7XG4gICAgICB2YXIgcGFyYW1zID0ge1xuICAgICAgICBzZXJ2ZXJfaWQ6IHNlcnZlcklkLFxuICAgICAgICBsb2dpbixcbiAgICAgICAgc2lkLFxuICAgICAgICB0ZXJtOiB7XG4gICAgICAgICAgaDogcm93cyxcbiAgICAgICAgICB3OiBjb2xzXG4gICAgICAgIH1cbiAgICAgIH1cblxuICAgICAgdmFyIGpzb24gPSBKU09OLnN0cmluZ2lmeShwYXJhbXMpO1xuICAgICAgdmFyIGpzb25FbmNvZGVkID0gd2luZG93LmVuY29kZVVSSShqc29uKTtcbiAgICAgIHZhciBob3N0bmFtZSA9IGdldFdzSG9zdE5hbWUoKTtcbiAgICAgIHJldHVybiBgJHtob3N0bmFtZX0vdjEvd2ViYXBpL3NpdGVzLy1jdXJyZW50LS9jb25uZWN0P2FjY2Vzc190b2tlbj0ke3Rva2VufSZwYXJhbXM9JHtqc29uRW5jb2RlZH1gO1xuICAgIH1cbiAgfSxcblxuICByb3V0ZXM6IHtcbiAgICBhcHA6ICcvd2ViJyxcbiAgICBsb2dvdXQ6ICcvd2ViL2xvZ291dCcsXG4gICAgbG9naW46ICcvd2ViL2xvZ2luJyxcbiAgICBub2RlczogJy93ZWIvbm9kZXMnLFxuICAgIGFjdGl2ZVNlc3Npb246ICcvd2ViL3Nlc3Npb25zLzpzaWQnLFxuICAgIG5ld1VzZXI6ICcvd2ViL25ld3VzZXIvOmludml0ZVRva2VuJyxcbiAgICBzZXNzaW9uczogJy93ZWIvc2Vzc2lvbnMnLFxuICAgIHBhZ2VOb3RGb3VuZDogJy93ZWIvbm90Zm91bmQnXG4gIH0sXG5cbiAgZ2V0QWN0aXZlU2Vzc2lvblJvdXRlVXJsKHNpZCl7XG4gICAgcmV0dXJuIGZvcm1hdFBhdHRlcm4oY2ZnLnJvdXRlcy5hY3RpdmVTZXNzaW9uLCB7c2lkfSk7XG4gIH1cbn1cblxuZXhwb3J0IGRlZmF1bHQgY2ZnO1xuXG5mdW5jdGlvbiBnZXRXc0hvc3ROYW1lKCl7XG4gIHZhciBwcmVmaXggPSBsb2NhdGlvbi5wcm90b2NvbCA9PSBcImh0dHBzOlwiP1wid3NzOi8vXCI6XCJ3czovL1wiO1xuICB2YXIgaG9zdHBvcnQgPSBsb2NhdGlvbi5ob3N0bmFtZSsobG9jYXRpb24ucG9ydCA/ICc6Jytsb2NhdGlvbi5wb3J0OiAnJyk7XG4gIHJldHVybiBgJHtwcmVmaXh9JHtob3N0cG9ydH1gO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbmZpZy5qc1xuICoqLyIsIi8qKlxuICogQ29weXJpZ2h0IDIwMTMtMjAxNCBGYWNlYm9vaywgSW5jLlxuICpcbiAqIExpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG4gKiB5b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG4gKiBZb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcbiAqXG4gKiBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcbiAqXG4gKiBVbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG4gKiBkaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG4gKiBXSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cbiAqIFNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbiAqIGxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuICpcbiAqL1xuXG5cInVzZSBzdHJpY3RcIjtcblxuLyoqXG4gKiBDb25zdHJ1Y3RzIGFuIGVudW1lcmF0aW9uIHdpdGgga2V5cyBlcXVhbCB0byB0aGVpciB2YWx1ZS5cbiAqXG4gKiBGb3IgZXhhbXBsZTpcbiAqXG4gKiAgIHZhciBDT0xPUlMgPSBrZXlNaXJyb3Ioe2JsdWU6IG51bGwsIHJlZDogbnVsbH0pO1xuICogICB2YXIgbXlDb2xvciA9IENPTE9SUy5ibHVlO1xuICogICB2YXIgaXNDb2xvclZhbGlkID0gISFDT0xPUlNbbXlDb2xvcl07XG4gKlxuICogVGhlIGxhc3QgbGluZSBjb3VsZCBub3QgYmUgcGVyZm9ybWVkIGlmIHRoZSB2YWx1ZXMgb2YgdGhlIGdlbmVyYXRlZCBlbnVtIHdlcmVcbiAqIG5vdCBlcXVhbCB0byB0aGVpciBrZXlzLlxuICpcbiAqICAgSW5wdXQ6ICB7a2V5MTogdmFsMSwga2V5MjogdmFsMn1cbiAqICAgT3V0cHV0OiB7a2V5MToga2V5MSwga2V5Mjoga2V5Mn1cbiAqXG4gKiBAcGFyYW0ge29iamVjdH0gb2JqXG4gKiBAcmV0dXJuIHtvYmplY3R9XG4gKi9cbnZhciBrZXlNaXJyb3IgPSBmdW5jdGlvbihvYmopIHtcbiAgdmFyIHJldCA9IHt9O1xuICB2YXIga2V5O1xuICBpZiAoIShvYmogaW5zdGFuY2VvZiBPYmplY3QgJiYgIUFycmF5LmlzQXJyYXkob2JqKSkpIHtcbiAgICB0aHJvdyBuZXcgRXJyb3IoJ2tleU1pcnJvciguLi4pOiBBcmd1bWVudCBtdXN0IGJlIGFuIG9iamVjdC4nKTtcbiAgfVxuICBmb3IgKGtleSBpbiBvYmopIHtcbiAgICBpZiAoIW9iai5oYXNPd25Qcm9wZXJ0eShrZXkpKSB7XG4gICAgICBjb250aW51ZTtcbiAgICB9XG4gICAgcmV0W2tleV0gPSBrZXk7XG4gIH1cbiAgcmV0dXJuIHJldDtcbn07XG5cbm1vZHVsZS5leHBvcnRzID0ga2V5TWlycm9yO1xuXG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiAuL34va2V5bWlycm9yL2luZGV4LmpzXG4gKiogbW9kdWxlIGlkID0gMjBcbiAqKiBtb2R1bGUgY2h1bmtzID0gMVxuICoqLyIsInZhciB7IGJyb3dzZXJIaXN0b3J5LCBjcmVhdGVNZW1vcnlIaXN0b3J5IH0gPSByZXF1aXJlKCdyZWFjdC1yb3V0ZXInKTtcblxuY29uc3QgQVVUSF9LRVlfREFUQSA9ICdhdXRoRGF0YSc7XG5cbnZhciBfaGlzdG9yeSA9IGNyZWF0ZU1lbW9yeUhpc3RvcnkoKTtcblxudmFyIHNlc3Npb24gPSB7XG5cbiAgaW5pdChoaXN0b3J5PWJyb3dzZXJIaXN0b3J5KXtcbiAgICBfaGlzdG9yeSA9IGhpc3Rvcnk7XG4gIH0sXG5cbiAgZ2V0SGlzdG9yeSgpe1xuICAgIHJldHVybiBfaGlzdG9yeTtcbiAgfSxcblxuICBzZXRVc2VyRGF0YSh1c2VyRGF0YSl7XG4gICAgbG9jYWxTdG9yYWdlLnNldEl0ZW0oQVVUSF9LRVlfREFUQSwgSlNPTi5zdHJpbmdpZnkodXNlckRhdGEpKTtcbiAgfSxcblxuICBnZXRVc2VyRGF0YSgpe1xuICAgIHZhciBpdGVtID0gbG9jYWxTdG9yYWdlLmdldEl0ZW0oQVVUSF9LRVlfREFUQSk7XG4gICAgaWYoaXRlbSl7XG4gICAgICByZXR1cm4gSlNPTi5wYXJzZShpdGVtKTtcbiAgICB9XG5cbiAgICByZXR1cm4ge307XG4gIH0sXG5cbiAgY2xlYXIoKXtcbiAgICBsb2NhbFN0b3JhZ2UuY2xlYXIoKVxuICB9XG5cbn1cblxubW9kdWxlLmV4cG9ydHMgPSBzZXNzaW9uO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL3Nlc3Npb24uanNcbiAqKi8iLCJ2YXIgJCA9IHJlcXVpcmUoXCJqUXVlcnlcIik7XG52YXIgc2Vzc2lvbiA9IHJlcXVpcmUoJ2FwcC9zZXNzaW9uJyk7XG5cbmNvbnN0IGFwaSA9IHtcblxuICBwdXQocGF0aCwgZGF0YSwgd2l0aFRva2VuKXtcbiAgICByZXR1cm4gYXBpLmFqYXgoe3VybDogcGF0aCwgZGF0YTogSlNPTi5zdHJpbmdpZnkoZGF0YSksIHR5cGU6ICdQVVQnfSwgd2l0aFRva2VuKTtcbiAgfSxcblxuICBwb3N0KHBhdGgsIGRhdGEsIHdpdGhUb2tlbil7XG4gICAgcmV0dXJuIGFwaS5hamF4KHt1cmw6IHBhdGgsIGRhdGE6IEpTT04uc3RyaW5naWZ5KGRhdGEpLCB0eXBlOiAnUE9TVCd9LCB3aXRoVG9rZW4pO1xuICB9LFxuXG4gIGdldChwYXRoKXtcbiAgICByZXR1cm4gYXBpLmFqYXgoe3VybDogcGF0aH0pO1xuICB9LFxuXG4gIGFqYXgoY2ZnLCB3aXRoVG9rZW4gPSB0cnVlKXtcbiAgICB2YXIgZGVmYXVsdENmZyA9IHtcbiAgICAgIHR5cGU6IFwiR0VUXCIsXG4gICAgICBkYXRhVHlwZTogXCJqc29uXCIsXG4gICAgICBiZWZvcmVTZW5kOiBmdW5jdGlvbih4aHIpIHtcbiAgICAgICAgaWYod2l0aFRva2VuKXtcbiAgICAgICAgICB2YXIgeyB0b2tlbiB9ID0gc2Vzc2lvbi5nZXRVc2VyRGF0YSgpO1xuICAgICAgICAgIHhoci5zZXRSZXF1ZXN0SGVhZGVyKCdBdXRob3JpemF0aW9uJywnQmVhcmVyICcgKyB0b2tlbik7XG4gICAgICAgIH1cbiAgICAgICB9XG4gICAgfVxuXG4gICAgcmV0dXJuICQuYWpheCgkLmV4dGVuZCh7fSwgZGVmYXVsdENmZywgY2ZnKSk7XG4gIH1cbn1cblxubW9kdWxlLmV4cG9ydHMgPSBhcGk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvc2VydmljZXMvYXBpLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMgPSBqUXVlcnk7XG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiBleHRlcm5hbCBcImpRdWVyeVwiXG4gKiogbW9kdWxlIGlkID0gNDJcbiAqKiBtb2R1bGUgY2h1bmtzID0gMVxuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciBzZXNzaW9uID0gcmVxdWlyZSgnYXBwL3Nlc3Npb24nKTtcbnZhciB7dXVpZH0gPSByZXF1aXJlKCdhcHAvdXRpbHMnKTtcbnZhciBhcGkgPSByZXF1aXJlKCdhcHAvc2VydmljZXMvYXBpJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xudmFyIGdldHRlcnMgPSByZXF1aXJlKCcuL2dldHRlcnMnKTtcbnZhciBzZXNzaW9uTW9kdWxlID0gcmVxdWlyZSgnLi8uLi9zZXNzaW9ucycpO1xuXG52YXIgeyBUTFBUX1RFUk1fT1BFTiwgVExQVF9URVJNX0NMT1NFLCBUTFBUX1RFUk1fQ0hBTkdFX1NFUlZFUiB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG52YXIgYWN0aW9ucyA9IHtcblxuICBjaGFuZ2VTZXJ2ZXIoc2VydmVySWQsIGxvZ2luKXtcbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfVEVSTV9DSEFOR0VfU0VSVkVSLCB7XG4gICAgICBzZXJ2ZXJJZCxcbiAgICAgIGxvZ2luXG4gICAgfSk7XG4gIH0sXG5cbiAgY2xvc2UoKXtcbiAgICBsZXQge2lzTmV3U2Vzc2lvbn0gPSByZWFjdG9yLmV2YWx1YXRlKGdldHRlcnMuYWN0aXZlU2Vzc2lvbik7XG5cbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfVEVSTV9DTE9TRSk7XG5cbiAgICBpZihpc05ld1Nlc3Npb24pe1xuICAgICAgc2Vzc2lvbi5nZXRIaXN0b3J5KCkucHVzaChjZmcucm91dGVzLm5vZGVzKTtcbiAgICB9ZWxzZXtcbiAgICAgIHNlc3Npb24uZ2V0SGlzdG9yeSgpLnB1c2goY2ZnLnJvdXRlcy5zZXNzaW9ucyk7XG4gICAgfVxuICB9LFxuXG4gIHJlc2l6ZSh3LCBoKXtcbiAgICAvLyBzb21lIG1pbiB2YWx1ZXNcbiAgICB3ID0gdyA8IDUgPyA1IDogdztcbiAgICBoID0gaCA8IDUgPyA1IDogaDtcblxuICAgIGxldCByZXFEYXRhID0geyB0ZXJtaW5hbF9wYXJhbXM6IHsgdywgaCB9IH07XG4gICAgbGV0IHtzaWR9ID0gcmVhY3Rvci5ldmFsdWF0ZShnZXR0ZXJzLmFjdGl2ZVNlc3Npb24pO1xuXG4gICAgYXBpLnB1dChjZmcuYXBpLmdldFRlcm1pbmFsU2Vzc2lvblVybChzaWQpLCByZXFEYXRhKVxuICAgICAgLmRvbmUoKCk9PntcbiAgICAgICAgY29uc29sZS5sb2coYHJlc2l6ZSB3aXRoIHc6JHt3fSBhbmQgaDoke2h9IC0gT0tgKTtcbiAgICAgIH0pXG4gICAgICAuZmFpbCgoKT0+e1xuICAgICAgICBjb25zb2xlLmxvZyhgZmFpbGVkIHRvIHJlc2l6ZSB3aXRoIHc6JHt3fSBhbmQgaDoke2h9YCk7XG4gICAgfSlcbiAgfSxcblxuICBvcGVuU2Vzc2lvbihzaWQpe1xuICAgIHNlc3Npb25Nb2R1bGUuYWN0aW9ucy5mZXRjaFNlc3Npb24oc2lkKVxuICAgICAgLmRvbmUoKCk9PntcbiAgICAgICAgbGV0IHNWaWV3ID0gcmVhY3Rvci5ldmFsdWF0ZShzZXNzaW9uTW9kdWxlLmdldHRlcnMuc2Vzc2lvblZpZXdCeUlkKHNpZCkpO1xuICAgICAgICBsZXQgeyBzZXJ2ZXJJZCwgbG9naW4gfSA9IHNWaWV3O1xuICAgICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfVEVSTV9PUEVOLCB7XG4gICAgICAgICAgICBzZXJ2ZXJJZCxcbiAgICAgICAgICAgIGxvZ2luLFxuICAgICAgICAgICAgc2lkLFxuICAgICAgICAgICAgaXNOZXdTZXNzaW9uOiBmYWxzZVxuICAgICAgICAgIH0pO1xuICAgICAgfSlcbiAgICAgIC5mYWlsKCgpPT57XG4gICAgICAgIHNlc3Npb24uZ2V0SGlzdG9yeSgpLnB1c2goY2ZnLnJvdXRlcy5wYWdlTm90Rm91bmQpO1xuICAgICAgfSlcbiAgfSxcblxuICBjcmVhdGVOZXdTZXNzaW9uKHNlcnZlcklkLCBsb2dpbil7XG4gICAgdmFyIHNpZCA9IHV1aWQoKTtcbiAgICB2YXIgcm91dGVVcmwgPSBjZmcuZ2V0QWN0aXZlU2Vzc2lvblJvdXRlVXJsKHNpZCk7XG4gICAgdmFyIGhpc3RvcnkgPSBzZXNzaW9uLmdldEhpc3RvcnkoKTtcblxuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9URVJNX09QRU4sIHtcbiAgICAgIHNlcnZlcklkLFxuICAgICAgbG9naW4sXG4gICAgICBzaWQsXG4gICAgICBpc05ld1Nlc3Npb246IHRydWVcbiAgICB9KTtcblxuICAgIGhpc3RvcnkucHVzaChyb3V0ZVVybCk7XG4gIH1cblxufVxuXG5leHBvcnQgZGVmYXVsdCBhY3Rpb25zO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvYWN0aW9ucy5qc1xuICoqLyIsIm1vZHVsZS5leHBvcnRzLmdldHRlcnMgPSByZXF1aXJlKCcuL2dldHRlcnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGlvbnMgPSByZXF1aXJlKCcuL2FjdGlvbnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGl2ZVRlcm1TdG9yZSA9IHJlcXVpcmUoJy4vYWN0aXZlVGVybVN0b3JlJyk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9pbmRleC5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7IFRMUFRfRElBTE9HX1NIT1dfU0VMRUNUX05PREUsIFRMUFRfRElBTE9HX0NMT1NFX1NFTEVDVF9OT0RFIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbnZhciBhY3Rpb25zID0ge1xuICBzaG93U2VsZWN0Tm9kZURpYWxvZygpe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9ESUFMT0dfU0hPV19TRUxFQ1RfTk9ERSk7XG4gIH0sXG5cbiAgY2xvc2VTZWxlY3ROb2RlRGlhbG9nKCl7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX0RJQUxPR19DTE9TRV9TRUxFQ1RfTk9ERSk7XG4gIH1cbn1cblxuZXhwb3J0IGRlZmF1bHQgYWN0aW9ucztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvYWN0aW9ucy5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciBhcGkgPSByZXF1aXJlKCdhcHAvc2VydmljZXMvYXBpJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xuXG52YXIgeyBUTFBUX1NFU1NJTlNfUkVDRUlWRSwgVExQVF9TRVNTSU5TX1VQREFURSB9ICA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuXG4gIGZldGNoU2Vzc2lvbihzaWQpe1xuICAgIHJldHVybiBhcGkuZ2V0KGNmZy5hcGkuZ2V0RmV0Y2hTZXNzaW9uVXJsKHNpZCkpLnRoZW4oanNvbj0+e1xuICAgICAgaWYoanNvbiAmJiBqc29uLnNlc3Npb24pe1xuICAgICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfU0VTU0lOU19VUERBVEUsIGpzb24uc2Vzc2lvbik7XG4gICAgICB9XG4gICAgfSk7XG4gIH0sXG5cbiAgZmV0Y2hTZXNzaW9ucygpe1xuICAgIHJldHVybiBhcGkuZ2V0KGNmZy5hcGkuZ2V0RmV0Y2hTZXNzaW9uc1VybCgpKS5kb25lKChqc29uKSA9PiB7XG4gICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfU0VTU0lOU19SRUNFSVZFLCBqc29uLnNlc3Npb25zKTtcbiAgICB9KTtcbiAgfSxcblxuICB1cGRhdGVTZXNzaW9uKGpzb24pe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9TRVNTSU5TX1VQREFURSwganNvbik7XG4gIH0gIFxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvYWN0aW9ucy5qc1xuICoqLyIsInZhciB7IHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xuXG5jb25zdCBzZXNzaW9uc0J5U2VydmVyID0gKHNlcnZlcklkKSA9PiBbWyd0bHB0X3Nlc3Npb25zJ10sIChzZXNzaW9ucykgPT57XG4gIHJldHVybiBzZXNzaW9ucy52YWx1ZVNlcSgpLmZpbHRlcihpdGVtPT57XG4gICAgdmFyIHBhcnRpZXMgPSBpdGVtLmdldCgncGFydGllcycpIHx8IHRvSW1tdXRhYmxlKFtdKTtcbiAgICB2YXIgaGFzU2VydmVyID0gcGFydGllcy5maW5kKGl0ZW0yPT4gaXRlbTIuZ2V0KCdzZXJ2ZXJfaWQnKSA9PT0gc2VydmVySWQpO1xuICAgIHJldHVybiBoYXNTZXJ2ZXI7XG4gIH0pLnRvTGlzdCgpO1xufV1cblxuY29uc3Qgc2Vzc2lvbnNWaWV3ID0gW1sndGxwdF9zZXNzaW9ucyddLCAoc2Vzc2lvbnMpID0+e1xuICByZXR1cm4gc2Vzc2lvbnMudmFsdWVTZXEoKS5tYXAoY3JlYXRlVmlldykudG9KUygpO1xufV07XG5cbmNvbnN0IHNlc3Npb25WaWV3QnlJZCA9IChzaWQpPT4gW1sndGxwdF9zZXNzaW9ucycsIHNpZF0sIChzZXNzaW9uKT0+e1xuICBpZighc2Vzc2lvbil7XG4gICAgcmV0dXJuIG51bGw7XG4gIH1cblxuICByZXR1cm4gY3JlYXRlVmlldyhzZXNzaW9uKTtcbn1dO1xuXG5jb25zdCBwYXJ0aWVzQnlTZXNzaW9uSWQgPSAoc2lkKSA9PlxuIFtbJ3RscHRfc2Vzc2lvbnMnLCBzaWQsICdwYXJ0aWVzJ10sIChwYXJ0aWVzKSA9PntcblxuICBpZighcGFydGllcyl7XG4gICAgcmV0dXJuIFtdO1xuICB9XG5cbiAgdmFyIGxhc3RBY3RpdmVVc3JOYW1lID0gZ2V0TGFzdEFjdGl2ZVVzZXIocGFydGllcykuZ2V0KCd1c2VyJyk7XG5cbiAgcmV0dXJuIHBhcnRpZXMubWFwKGl0ZW09PntcbiAgICB2YXIgdXNlciA9IGl0ZW0uZ2V0KCd1c2VyJyk7XG4gICAgcmV0dXJuIHtcbiAgICAgIHVzZXI6IGl0ZW0uZ2V0KCd1c2VyJyksXG4gICAgICBzZXJ2ZXJJcDogaXRlbS5nZXQoJ3JlbW90ZV9hZGRyJyksXG4gICAgICBzZXJ2ZXJJZDogaXRlbS5nZXQoJ3NlcnZlcl9pZCcpLFxuICAgICAgaXNBY3RpdmU6IGxhc3RBY3RpdmVVc3JOYW1lID09PSB1c2VyXG4gICAgfVxuICB9KS50b0pTKCk7XG59XTtcblxuZnVuY3Rpb24gZ2V0TGFzdEFjdGl2ZVVzZXIocGFydGllcyl7XG4gIHJldHVybiBwYXJ0aWVzLnNvcnRCeShpdGVtPT4gbmV3IERhdGUoaXRlbS5nZXQoJ2xhc3RBY3RpdmUnKSkpLmZpcnN0KCk7XG59XG5cbmZ1bmN0aW9uIGNyZWF0ZVZpZXcoc2Vzc2lvbil7XG4gIHZhciBzaWQgPSBzZXNzaW9uLmdldCgnaWQnKTtcbiAgdmFyIHNlcnZlcklwLCBzZXJ2ZXJJZDtcbiAgdmFyIHBhcnRpZXMgPSByZWFjdG9yLmV2YWx1YXRlKHBhcnRpZXNCeVNlc3Npb25JZChzaWQpKTtcblxuICBpZihwYXJ0aWVzLmxlbmd0aCA+IDApe1xuICAgIHNlcnZlcklwID0gcGFydGllc1swXS5zZXJ2ZXJJcDtcbiAgICBzZXJ2ZXJJZCA9IHBhcnRpZXNbMF0uc2VydmVySWQ7XG4gIH1cblxuICByZXR1cm4ge1xuICAgIHNpZDogc2lkLFxuICAgIHNlc3Npb25Vcmw6IGNmZy5nZXRBY3RpdmVTZXNzaW9uUm91dGVVcmwoc2lkKSxcbiAgICBzZXJ2ZXJJcCxcbiAgICBzZXJ2ZXJJZCxcbiAgICBhY3RpdmU6IHNlc3Npb24uZ2V0KCdhY3RpdmUnKSxcbiAgICBjcmVhdGVkOiBuZXcgRGF0ZShzZXNzaW9uLmdldCgnY3JlYXRlZCcpKSxcbiAgICBsYXN0QWN0aXZlOiBuZXcgRGF0ZShzZXNzaW9uLmdldCgnbGFzdF9hY3RpdmUnKSksXG4gICAgbG9naW46IHNlc3Npb24uZ2V0KCdsb2dpbicpLFxuICAgIHBhcnRpZXM6IHBhcnRpZXMsXG4gICAgY29sczogc2Vzc2lvbi5nZXRJbihbJ3Rlcm1pbmFsX3BhcmFtcycsICd3J10pLFxuICAgIHJvd3M6IHNlc3Npb24uZ2V0SW4oWyd0ZXJtaW5hbF9wYXJhbXMnLCAnaCddKVxuICB9XG59XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgcGFydGllc0J5U2Vzc2lvbklkLFxuICBzZXNzaW9uc0J5U2VydmVyLFxuICBzZXNzaW9uc1ZpZXcsXG4gIHNlc3Npb25WaWV3QnlJZCxcbiAgY3JlYXRlVmlld1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvZ2V0dGVycy5qc1xuICoqLyIsImNvbnN0IHVzZXIgPSBbIFsndGxwdF91c2VyJ10sIChjdXJyZW50VXNlcikgPT4ge1xuICAgIGlmKCFjdXJyZW50VXNlcil7XG4gICAgICByZXR1cm4gbnVsbDtcbiAgICB9XG5cbiAgICB2YXIgbmFtZSA9IGN1cnJlbnRVc2VyLmdldCgnbmFtZScpIHx8ICcnO1xuICAgIHZhciBzaG9ydERpc3BsYXlOYW1lID0gbmFtZVswXSB8fCAnJztcblxuICAgIHJldHVybiB7XG4gICAgICBuYW1lLFxuICAgICAgc2hvcnREaXNwbGF5TmFtZSxcbiAgICAgIGxvZ2luczogY3VycmVudFVzZXIuZ2V0KCdhbGxvd2VkX2xvZ2lucycpLnRvSlMoKVxuICAgIH1cbiAgfVxuXTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICB1c2VyXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy91c2VyL2dldHRlcnMuanNcbiAqKi8iLCJ2YXIgYXBpID0gcmVxdWlyZSgnLi9zZXJ2aWNlcy9hcGknKTtcbnZhciBzZXNzaW9uID0gcmVxdWlyZSgnLi9zZXNzaW9uJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xudmFyICQgPSByZXF1aXJlKCdqUXVlcnknKTtcblxuY29uc3QgcmVmcmVzaFJhdGUgPSA2MDAwMCAqIDU7IC8vIDEgbWluXG5cbnZhciByZWZyZXNoVG9rZW5UaW1lcklkID0gbnVsbDtcblxudmFyIGF1dGggPSB7XG5cbiAgc2lnblVwKG5hbWUsIHBhc3N3b3JkLCB0b2tlbiwgaW52aXRlVG9rZW4pe1xuICAgIHZhciBkYXRhID0ge3VzZXI6IG5hbWUsIHBhc3M6IHBhc3N3b3JkLCBzZWNvbmRfZmFjdG9yX3Rva2VuOiB0b2tlbiwgaW52aXRlX3Rva2VuOiBpbnZpdGVUb2tlbn07XG4gICAgcmV0dXJuIGFwaS5wb3N0KGNmZy5hcGkuY3JlYXRlVXNlclBhdGgsIGRhdGEpXG4gICAgICAudGhlbigodXNlcik9PntcbiAgICAgICAgc2Vzc2lvbi5zZXRVc2VyRGF0YSh1c2VyKTtcbiAgICAgICAgYXV0aC5fc3RhcnRUb2tlblJlZnJlc2hlcigpO1xuICAgICAgICByZXR1cm4gdXNlcjtcbiAgICAgIH0pO1xuICB9LFxuXG4gIGxvZ2luKG5hbWUsIHBhc3N3b3JkLCB0b2tlbil7XG4gICAgYXV0aC5fc3RvcFRva2VuUmVmcmVzaGVyKCk7XG4gICAgcmV0dXJuIGF1dGguX2xvZ2luKG5hbWUsIHBhc3N3b3JkLCB0b2tlbikuZG9uZShhdXRoLl9zdGFydFRva2VuUmVmcmVzaGVyKTtcbiAgfSxcblxuICBlbnN1cmVVc2VyKCl7XG4gICAgdmFyIHVzZXJEYXRhID0gc2Vzc2lvbi5nZXRVc2VyRGF0YSgpO1xuICAgIGlmKHVzZXJEYXRhLnRva2VuKXtcbiAgICAgIC8vIHJlZnJlc2ggdGltZXIgd2lsbCBub3QgYmUgc2V0IGluIGNhc2Ugb2YgYnJvd3NlciByZWZyZXNoIGV2ZW50XG4gICAgICBpZihhdXRoLl9nZXRSZWZyZXNoVG9rZW5UaW1lcklkKCkgPT09IG51bGwpe1xuICAgICAgICByZXR1cm4gYXV0aC5fcmVmcmVzaFRva2VuKCkuZG9uZShhdXRoLl9zdGFydFRva2VuUmVmcmVzaGVyKTtcbiAgICAgIH1cblxuICAgICAgcmV0dXJuICQuRGVmZXJyZWQoKS5yZXNvbHZlKHVzZXJEYXRhKTtcbiAgICB9XG5cbiAgICByZXR1cm4gJC5EZWZlcnJlZCgpLnJlamVjdCgpO1xuICB9LFxuXG4gIGxvZ291dCgpe1xuICAgIGF1dGguX3N0b3BUb2tlblJlZnJlc2hlcigpO1xuICAgIHNlc3Npb24uY2xlYXIoKTtcbiAgICBzZXNzaW9uLmdldEhpc3RvcnkoKS5yZXBsYWNlKHtwYXRobmFtZTogY2ZnLnJvdXRlcy5sb2dpbn0pOyAgICBcbiAgfSxcblxuICBfc3RhcnRUb2tlblJlZnJlc2hlcigpe1xuICAgIHJlZnJlc2hUb2tlblRpbWVySWQgPSBzZXRJbnRlcnZhbChhdXRoLl9yZWZyZXNoVG9rZW4sIHJlZnJlc2hSYXRlKTtcbiAgfSxcblxuICBfc3RvcFRva2VuUmVmcmVzaGVyKCl7XG4gICAgY2xlYXJJbnRlcnZhbChyZWZyZXNoVG9rZW5UaW1lcklkKTtcbiAgICByZWZyZXNoVG9rZW5UaW1lcklkID0gbnVsbDtcbiAgfSxcblxuICBfZ2V0UmVmcmVzaFRva2VuVGltZXJJZCgpe1xuICAgIHJldHVybiByZWZyZXNoVG9rZW5UaW1lcklkO1xuICB9LFxuXG4gIF9yZWZyZXNoVG9rZW4oKXtcbiAgICByZXR1cm4gYXBpLnBvc3QoY2ZnLmFwaS5yZW5ld1Rva2VuUGF0aCkudGhlbihkYXRhPT57XG4gICAgICBzZXNzaW9uLnNldFVzZXJEYXRhKGRhdGEpO1xuICAgICAgcmV0dXJuIGRhdGE7XG4gICAgfSkuZmFpbCgoKT0+e1xuICAgICAgYXV0aC5sb2dvdXQoKTtcbiAgICB9KTtcbiAgfSxcblxuICBfbG9naW4obmFtZSwgcGFzc3dvcmQsIHRva2VuKXtcbiAgICB2YXIgZGF0YSA9IHtcbiAgICAgIHVzZXI6IG5hbWUsXG4gICAgICBwYXNzOiBwYXNzd29yZCxcbiAgICAgIHNlY29uZF9mYWN0b3JfdG9rZW46IHRva2VuXG4gICAgfTtcblxuICAgIHJldHVybiBhcGkucG9zdChjZmcuYXBpLnNlc3Npb25QYXRoLCBkYXRhLCBmYWxzZSkudGhlbihkYXRhPT57XG4gICAgICBzZXNzaW9uLnNldFVzZXJEYXRhKGRhdGEpO1xuICAgICAgcmV0dXJuIGRhdGE7XG4gICAgfSk7XG4gIH1cbn1cblxubW9kdWxlLmV4cG9ydHMgPSBhdXRoO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2F1dGguanNcbiAqKi8iLCJ2YXIgRXZlbnRFbWl0dGVyID0gcmVxdWlyZSgnZXZlbnRzJykuRXZlbnRFbWl0dGVyO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCdhcHAvc2Vzc2lvbicpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcbnZhciB7YWN0aW9uc30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC8nKTtcblxuY2xhc3MgVHR5IGV4dGVuZHMgRXZlbnRFbWl0dGVyIHtcblxuICBjb25zdHJ1Y3Rvcih7c2VydmVySWQsIGxvZ2luLCBzaWQsIHJvd3MsIGNvbHMgfSl7XG4gICAgc3VwZXIoKTtcbiAgICB0aGlzLm9wdGlvbnMgPSB7IHNlcnZlcklkLCBsb2dpbiwgc2lkLCByb3dzLCBjb2xzIH07XG4gICAgdGhpcy5zb2NrZXQgPSBudWxsO1xuICB9XG5cbiAgZGlzY29ubmVjdCgpe1xuICAgIHRoaXMuc29ja2V0LmNsb3NlKCk7XG4gIH1cblxuICByZWNvbm5lY3Qob3B0aW9ucyl7XG4gICAgdGhpcy5zb2NrZXQuY2xvc2UoKTtcbiAgICB0aGlzLmNvbm5lY3Qob3B0aW9ucyk7XG4gIH1cblxuICBjb25uZWN0KG9wdGlvbnMpe1xuICAgIE9iamVjdC5hc3NpZ24odGhpcy5vcHRpb25zLCBvcHRpb25zKTtcblxuICAgIGxldCB7dG9rZW59ID0gc2Vzc2lvbi5nZXRVc2VyRGF0YSgpO1xuICAgIGxldCBjb25uU3RyID0gY2ZnLmFwaS5nZXRUdHlDb25uU3RyKHt0b2tlbiwgLi4udGhpcy5vcHRpb25zfSk7XG5cbiAgICB0aGlzLnNvY2tldCA9IG5ldyBXZWJTb2NrZXQoY29ublN0ciwgJ3Byb3RvJyk7XG5cbiAgICB0aGlzLnNvY2tldC5vbm9wZW4gPSAoKSA9PiB7XG4gICAgICB0aGlzLmVtaXQoJ29wZW4nKTtcbiAgICB9XG5cbiAgICB0aGlzLnNvY2tldC5vbm1lc3NhZ2UgPSAoZSk9PntcbiAgICAgIHRoaXMuZW1pdCgnZGF0YScsIGUuZGF0YSk7XG4gICAgfVxuXG4gICAgdGhpcy5zb2NrZXQub25jbG9zZSA9ICgpPT57XG4gICAgICB0aGlzLmVtaXQoJ2Nsb3NlJyk7XG4gICAgfVxuICB9XG5cbiAgcmVzaXplKGNvbHMsIHJvd3Mpe1xuICAgIGFjdGlvbnMucmVzaXplKGNvbHMsIHJvd3MpO1xuICB9XG5cbiAgc2VuZChkYXRhKXtcbiAgICB0aGlzLnNvY2tldC5zZW5kKGRhdGEpO1xuICB9XG59XG5cbm1vZHVsZS5leHBvcnRzID0gVHR5O1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbW1vbi90dHkuanNcbiAqKi8iLCJpbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVExQVF9URVJNX09QRU46IG51bGwsXG4gIFRMUFRfVEVSTV9DTE9TRTogbnVsbCxcbiAgVExQVF9URVJNX0NIQU5HRV9TRVJWRVI6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3Rpb25UeXBlcy5qc1xuICoqLyIsInZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyIHsgVExQVF9URVJNX09QRU4sIFRMUFRfVEVSTV9DTE9TRSwgVExQVF9URVJNX0NIQU5HRV9TRVJWRVIgfSAgPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IFN0b3JlKHtcbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB0b0ltbXV0YWJsZShudWxsKTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9URVJNX09QRU4sIHNldEFjdGl2ZVRlcm1pbmFsKTtcbiAgICB0aGlzLm9uKFRMUFRfVEVSTV9DTE9TRSwgY2xvc2UpO1xuICAgIHRoaXMub24oVExQVF9URVJNX0NIQU5HRV9TRVJWRVIsIGNoYW5nZVNlcnZlcik7XG4gIH1cbn0pXG5cbmZ1bmN0aW9uIGNoYW5nZVNlcnZlcihzdGF0ZSwge3NlcnZlcklkLCBsb2dpbn0pe1xuICByZXR1cm4gc3RhdGUuc2V0KCdzZXJ2ZXJJZCcsIHNlcnZlcklkKVxuICAgICAgICAgICAgICAuc2V0KCdsb2dpbicsIGxvZ2luKTtcbn1cblxuZnVuY3Rpb24gY2xvc2UoKXtcbiAgcmV0dXJuIHRvSW1tdXRhYmxlKG51bGwpO1xufVxuXG5mdW5jdGlvbiBzZXRBY3RpdmVUZXJtaW5hbChzdGF0ZSwge3NlcnZlcklkLCBsb2dpbiwgc2lkLCBpc05ld1Nlc3Npb259ICl7XG4gIHJldHVybiB0b0ltbXV0YWJsZSh7XG4gICAgc2VydmVySWQsXG4gICAgbG9naW4sXG4gICAgc2lkLFxuICAgIGlzTmV3U2Vzc2lvblxuICB9KTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGl2ZVRlcm1TdG9yZS5qc1xuICoqLyIsInZhciB7Y3JlYXRlVmlld30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9zZXNzaW9ucy9nZXR0ZXJzJyk7XG5cbmNvbnN0IGFjdGl2ZVNlc3Npb24gPSBbXG5bJ3RscHRfYWN0aXZlX3Rlcm1pbmFsJ10sIFsndGxwdF9zZXNzaW9ucyddLFxuKGFjdGl2ZVRlcm0sIHNlc3Npb25zKSA9PiB7XG4gICAgaWYoIWFjdGl2ZVRlcm0pe1xuICAgICAgcmV0dXJuIG51bGw7XG4gICAgfVxuXG4gICAgLypcbiAgICAqIGFjdGl2ZSBzZXNzaW9uIG5lZWRzIHRvIGhhdmUgaXRzIG93biB2aWV3IGFzIGFuIGFjdHVhbCBzZXNzaW9uIG1pZ2h0IG5vdFxuICAgICogZXhpc3QgYXQgdGhpcyBwb2ludC4gRm9yIGV4YW1wbGUsIHVwb24gY3JlYXRpbmcgYSBuZXcgc2Vzc2lvbiB3ZSBuZWVkIHRvIGtub3dcbiAgICAqIGxvZ2luIGFuZCBzZXJ2ZXJJZC4gSXQgd2lsbCBiZSBzaW1wbGlmaWVkIG9uY2Ugc2VydmVyIEFQSSBnZXRzIGV4dGVuZGVkLlxuICAgICovXG4gICAgbGV0IGFzVmlldyA9IHtcbiAgICAgIGlzTmV3U2Vzc2lvbjogYWN0aXZlVGVybS5nZXQoJ2lzTmV3U2Vzc2lvbicpLFxuICAgICAgbm90Rm91bmQ6IGFjdGl2ZVRlcm0uZ2V0KCdub3RGb3VuZCcpLFxuICAgICAgYWRkcjogYWN0aXZlVGVybS5nZXQoJ2FkZHInKSxcbiAgICAgIHNlcnZlcklkOiBhY3RpdmVUZXJtLmdldCgnc2VydmVySWQnKSxcbiAgICAgIHNlcnZlcklwOiB1bmRlZmluZWQsXG4gICAgICBsb2dpbjogYWN0aXZlVGVybS5nZXQoJ2xvZ2luJyksXG4gICAgICBzaWQ6IGFjdGl2ZVRlcm0uZ2V0KCdzaWQnKSxcbiAgICAgIGNvbHM6IHVuZGVmaW5lZCxcbiAgICAgIHJvd3M6IHVuZGVmaW5lZFxuICAgIH07XG5cbiAgICAvLyBpbiBjYXNlIGlmIHNlc3Npb24gYWxyZWFkeSBleGlzdHMsIGdldCB0aGUgZGF0YSBmcm9tIHRoZXJlXG4gICAgLy8gKGZvciBleGFtcGxlLCB3aGVuIGpvaW5pbmcgYW4gZXhpc3Rpbmcgc2Vzc2lvbilcbiAgICBpZihzZXNzaW9ucy5oYXMoYXNWaWV3LnNpZCkpe1xuICAgICAgbGV0IHNWaWV3ID0gY3JlYXRlVmlldyhzZXNzaW9ucy5nZXQoYXNWaWV3LnNpZCkpO1xuXG4gICAgICBhc1ZpZXcucGFydGllcyA9IHNWaWV3LnBhcnRpZXM7XG4gICAgICBhc1ZpZXcuc2VydmVySXAgPSBzVmlldy5zZXJ2ZXJJcDtcbiAgICAgIGFzVmlldy5zZXJ2ZXJJZCA9IHNWaWV3LnNlcnZlcklkO1xuICAgICAgYXNWaWV3LmFjdGl2ZSA9IHNWaWV3LmFjdGl2ZTtcbiAgICAgIGFzVmlldy5jb2xzID0gc1ZpZXcuY29scztcbiAgICAgIGFzVmlldy5yb3dzID0gc1ZpZXcucm93cztcbiAgICB9XG5cbiAgICByZXR1cm4gYXNWaWV3O1xuXG4gIH1cbl07XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgYWN0aXZlU2Vzc2lvblxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvZ2V0dGVycy5qc1xuICoqLyIsImltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX0FQUF9JTklUOiBudWxsLFxuICBUTFBUX0FQUF9GQUlMRUQ6IG51bGwsXG4gIFRMUFRfQVBQX1JFQURZOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvYXBwL2FjdGlvblR5cGVzLmpzXG4gKiovIiwidmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG5cbnZhciB7IFRMUFRfQVBQX0lOSVQsIFRMUFRfQVBQX0ZBSUxFRCwgVExQVF9BUFBfUkVBRFkgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxudmFyIGluaXRTdGF0ZSA9IHRvSW1tdXRhYmxlKHtcbiAgaXNSZWFkeTogZmFsc2UsXG4gIGlzSW5pdGlhbGl6aW5nOiBmYWxzZSxcbiAgaXNGYWlsZWQ6IGZhbHNlXG59KTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuXG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gaW5pdFN0YXRlLnNldCgnaXNJbml0aWFsaXppbmcnLCB0cnVlKTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9BUFBfSU5JVCwgKCk9PiBpbml0U3RhdGUuc2V0KCdpc0luaXRpYWxpemluZycsIHRydWUpKTtcbiAgICB0aGlzLm9uKFRMUFRfQVBQX1JFQURZLCgpPT4gaW5pdFN0YXRlLnNldCgnaXNSZWFkeScsIHRydWUpKTtcbiAgICB0aGlzLm9uKFRMUFRfQVBQX0ZBSUxFRCwoKT0+IGluaXRTdGF0ZS5zZXQoJ2lzRmFpbGVkJywgdHJ1ZSkpO1xuICB9XG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvYXBwL2FwcFN0b3JlLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfRElBTE9HX1NIT1dfU0VMRUNUX05PREU6IG51bGwsXG4gIFRMUFRfRElBTE9HX0NMT1NFX1NFTEVDVF9OT0RFOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvZGlhbG9ncy9hY3Rpb25UeXBlcy5qc1xuICoqLyIsInZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xuXG52YXIgeyBUTFBUX0RJQUxPR19TSE9XX1NFTEVDVF9OT0RFLCBUTFBUX0RJQUxPR19DTE9TRV9TRUxFQ1RfTk9ERSB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG5cbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB0b0ltbXV0YWJsZSh7XG4gICAgICBpc1NlbGVjdE5vZGVEaWFsb2dPcGVuOiBmYWxzZVxuICAgIH0pO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX0RJQUxPR19TSE9XX1NFTEVDVF9OT0RFLCBzaG93U2VsZWN0Tm9kZURpYWxvZyk7XG4gICAgdGhpcy5vbihUTFBUX0RJQUxPR19DTE9TRV9TRUxFQ1RfTk9ERSwgY2xvc2VTZWxlY3ROb2RlRGlhbG9nKTtcbiAgfVxufSlcblxuZnVuY3Rpb24gc2hvd1NlbGVjdE5vZGVEaWFsb2coc3RhdGUpe1xuICByZXR1cm4gc3RhdGUuc2V0KCdpc1NlbGVjdE5vZGVEaWFsb2dPcGVuJywgdHJ1ZSk7XG59XG5cbmZ1bmN0aW9uIGNsb3NlU2VsZWN0Tm9kZURpYWxvZyhzdGF0ZSl7XG4gIHJldHVybiBzdGF0ZS5zZXQoJ2lzU2VsZWN0Tm9kZURpYWxvZ09wZW4nLCBmYWxzZSk7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9kaWFsb2dzL2RpYWxvZ1N0b3JlLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfUkVDRUlWRV9VU0VSX0lOVklURTogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9hY3Rpb25UeXBlcy5qc1xuICoqLyIsInZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyICB7IFRMUFRfUkVDRUlWRV9VU0VSX0lOVklURSB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gdG9JbW11dGFibGUobnVsbCk7XG4gIH0sXG5cbiAgaW5pdGlhbGl6ZSgpIHtcbiAgICB0aGlzLm9uKFRMUFRfUkVDRUlWRV9VU0VSX0lOVklURSwgcmVjZWl2ZUludml0ZSlcbiAgfVxufSlcblxuZnVuY3Rpb24gcmVjZWl2ZUludml0ZShzdGF0ZSwgaW52aXRlKXtcbiAgcmV0dXJuIHRvSW1tdXRhYmxlKGludml0ZSk7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvaW52aXRlU3RvcmUuanNcbiAqKi8iLCJpbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVExQVF9OT0RFU19SRUNFSVZFOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvYWN0aW9uVHlwZXMuanNcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgeyBUTFBUX05PREVTX1JFQ0VJVkUgfSAgPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG52YXIgYXBpID0gcmVxdWlyZSgnYXBwL3NlcnZpY2VzL2FwaScpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICBmZXRjaE5vZGVzKCl7XG4gICAgYXBpLmdldChjZmcuYXBpLm5vZGVzUGF0aCkuZG9uZSgoZGF0YT1bXSk9PntcbiAgICAgIHZhciBub2RlQXJyYXkgPSBkYXRhLm5vZGVzLm1hcChpdGVtPT5pdGVtLm5vZGUpO1xuICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX05PREVTX1JFQ0VJVkUsIG5vZGVBcnJheSk7XG4gICAgfSk7XG4gIH1cbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2FjdGlvbnMuanNcbiAqKi8iLCJ2YXIgeyBTdG9yZSwgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcbnZhciAgeyBUTFBUX05PREVTX1JFQ0VJVkUgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKFtdKTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9OT0RFU19SRUNFSVZFLCByZWNlaXZlTm9kZXMpXG4gIH1cbn0pXG5cbmZ1bmN0aW9uIHJlY2VpdmVOb2RlcyhzdGF0ZSwgbm9kZUFycmF5KXtcbiAgcmV0dXJuIHRvSW1tdXRhYmxlKG5vZGVBcnJheSk7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9ub2RlU3RvcmUuanNcbiAqKi8iLCJpbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVExQVF9SRVNUX0FQSV9TVEFSVDogbnVsbCxcbiAgVExQVF9SRVNUX0FQSV9TVUNDRVNTOiBudWxsLFxuICBUTFBUX1JFU1RfQVBJX0ZBSUw6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2FjdGlvblR5cGVzLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRSWUlOR19UT19TSUdOX1VQOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9jb25zdGFudHMuanNcbiAqKi8iLCJpbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVExQVF9TRVNTSU5TX1JFQ0VJVkU6IG51bGwsXG4gIFRMUFRfU0VTU0lOU19VUERBVEU6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9zZXNzaW9ucy9hY3Rpb25UeXBlcy5qc1xuICoqLyIsIm1vZHVsZS5leHBvcnRzLmdldHRlcnMgPSByZXF1aXJlKCcuL2dldHRlcnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGlvbnMgPSByZXF1aXJlKCcuL2FjdGlvbnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGl2ZVRlcm1TdG9yZSA9IHJlcXVpcmUoJy4vc2Vzc2lvblN0b3JlJyk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9zZXNzaW9ucy9pbmRleC5qc1xuICoqLyIsInZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyIHsgVExQVF9TRVNTSU5TX1JFQ0VJVkUsIFRMUFRfU0VTU0lOU19VUERBVEUgfSAgPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IFN0b3JlKHtcbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB0b0ltbXV0YWJsZSh7fSk7XG4gIH0sXG5cbiAgaW5pdGlhbGl6ZSgpIHtcbiAgICB0aGlzLm9uKFRMUFRfU0VTU0lOU19SRUNFSVZFLCByZWNlaXZlU2Vzc2lvbnMpO1xuICAgIHRoaXMub24oVExQVF9TRVNTSU5TX1VQREFURSwgdXBkYXRlU2Vzc2lvbik7XG4gIH1cbn0pXG5cbmZ1bmN0aW9uIHVwZGF0ZVNlc3Npb24oc3RhdGUsIGpzb24pe1xuICByZXR1cm4gc3RhdGUuc2V0KGpzb24uaWQsIHRvSW1tdXRhYmxlKGpzb24pKTtcbn1cblxuZnVuY3Rpb24gcmVjZWl2ZVNlc3Npb25zKHN0YXRlLCBqc29uQXJyYXk9W10pe1xuICByZXR1cm4gc3RhdGUud2l0aE11dGF0aW9ucyhzdGF0ZSA9PiB7XG4gICAganNvbkFycmF5LmZvckVhY2goKGl0ZW0pID0+IHtcbiAgICAgIHN0YXRlLnNldChpdGVtLmlkLCB0b0ltbXV0YWJsZShpdGVtKSlcbiAgICB9KVxuICB9KTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL3Nlc3Npb25TdG9yZS5qc1xuICoqLyIsImltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX1JFQ0VJVkVfVVNFUjogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvYWN0aW9uVHlwZXMuanNcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgeyBUTFBUX1JFQ0VJVkVfVVNFUiB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xudmFyIHsgVFJZSU5HX1RPX1NJR05fVVB9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvcmVzdEFwaS9jb25zdGFudHMnKTtcbnZhciByZXN0QXBpQWN0aW9ucyA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3Jlc3RBcGkvYWN0aW9ucycpO1xudmFyIGF1dGggPSByZXF1aXJlKCdhcHAvYXV0aCcpO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCdhcHAvc2Vzc2lvbicpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuXG4gIGVuc3VyZVVzZXIobmV4dFN0YXRlLCByZXBsYWNlLCBjYil7XG4gICAgYXV0aC5lbnN1cmVVc2VyKClcbiAgICAgIC5kb25lKCh1c2VyRGF0YSk9PiB7XG4gICAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRUNFSVZFX1VTRVIsIHVzZXJEYXRhLnVzZXIgKTtcbiAgICAgICAgY2IoKTtcbiAgICAgIH0pXG4gICAgICAuZmFpbCgoKT0+e1xuICAgICAgICByZXBsYWNlKHtyZWRpcmVjdFRvOiBuZXh0U3RhdGUubG9jYXRpb24ucGF0aG5hbWUgfSwgY2ZnLnJvdXRlcy5sb2dpbik7XG4gICAgICAgIGNiKCk7XG4gICAgICB9KTtcbiAgfSxcblxuICBzaWduVXAoe25hbWUsIHBzdywgdG9rZW4sIGludml0ZVRva2VufSl7XG4gICAgcmVzdEFwaUFjdGlvbnMuc3RhcnQoVFJZSU5HX1RPX1NJR05fVVApO1xuICAgIGF1dGguc2lnblVwKG5hbWUsIHBzdywgdG9rZW4sIGludml0ZVRva2VuKVxuICAgICAgLmRvbmUoKHNlc3Npb25EYXRhKT0+e1xuICAgICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfUkVDRUlWRV9VU0VSLCBzZXNzaW9uRGF0YS51c2VyKTtcbiAgICAgICAgcmVzdEFwaUFjdGlvbnMuc3VjY2VzcyhUUllJTkdfVE9fU0lHTl9VUCk7XG4gICAgICAgIHNlc3Npb24uZ2V0SGlzdG9yeSgpLnB1c2goe3BhdGhuYW1lOiBjZmcucm91dGVzLmFwcH0pO1xuICAgICAgfSlcbiAgICAgIC5mYWlsKCgpPT57XG4gICAgICAgIHJlc3RBcGlBY3Rpb25zLmZhaWwoVFJZSU5HX1RPX1NJR05fVVAsICdmYWlsZWQgdG8gc2luZyB1cCcpO1xuICAgICAgfSk7XG4gIH0sXG5cbiAgbG9naW4oe3VzZXIsIHBhc3N3b3JkLCB0b2tlbn0sIHJlZGlyZWN0KXtcbiAgICAgIGF1dGgubG9naW4odXNlciwgcGFzc3dvcmQsIHRva2VuKVxuICAgICAgICAuZG9uZSgoc2Vzc2lvbkRhdGEpPT57XG4gICAgICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFQ0VJVkVfVVNFUiwgc2Vzc2lvbkRhdGEudXNlcik7XG4gICAgICAgICAgc2Vzc2lvbi5nZXRIaXN0b3J5KCkucHVzaCh7cGF0aG5hbWU6IHJlZGlyZWN0fSk7XG4gICAgICAgIH0pXG4gICAgICAgIC5mYWlsKCgpPT57XG4gICAgICAgIH0pXG4gICAgfVxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvdXNlci9hY3Rpb25zLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMuZ2V0dGVycyA9IHJlcXVpcmUoJy4vZ2V0dGVycycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aW9ucyA9IHJlcXVpcmUoJy4vYWN0aW9ucycpO1xubW9kdWxlLmV4cG9ydHMubm9kZVN0b3JlID0gcmVxdWlyZSgnLi91c2VyU3RvcmUnKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvaW5kZXguanNcbiAqKi8iLCJ2YXIgeyBTdG9yZSwgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcbnZhciAgeyBUTFBUX1JFQ0VJVkVfVVNFUiB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gdG9JbW11dGFibGUobnVsbCk7XG4gIH0sXG5cbiAgaW5pdGlhbGl6ZSgpIHtcbiAgICB0aGlzLm9uKFRMUFRfUkVDRUlWRV9VU0VSLCByZWNlaXZlVXNlcilcbiAgfVxuXG59KVxuXG5mdW5jdGlvbiByZWNlaXZlVXNlcihzdGF0ZSwgdXNlcil7XG4gIHJldHVybiB0b0ltbXV0YWJsZSh1c2VyKTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvdXNlclN0b3JlLmpzXG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciB7YWN0aW9uc30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC8nKTtcbnZhciBjb2xvcnMgPSBbJyMxYWIzOTQnLCAnIzFjODRjNicsICcjMjNjNmM4JywgJyNmOGFjNTknLCAnI0VENTU2NScsICcjYzJjMmMyJ107XG5cbmNvbnN0IFVzZXJJY29uID0gKHtuYW1lLCB0aXRsZSwgY29sb3JJbmRleD0wfSk9PntcbiAgbGV0IGNvbG9yID0gY29sb3JzW2NvbG9ySW5kZXggJSBjb2xvcnMubGVuZ3RoXTtcbiAgbGV0IHN0eWxlID0ge1xuICAgICdiYWNrZ3JvdW5kQ29sb3InOiBjb2xvcixcbiAgICAnYm9yZGVyQ29sb3InOiBjb2xvclxuICB9O1xuXG4gIHJldHVybiAoXG4gICAgPGxpPlxuICAgICAgPHNwYW4gc3R5bGU9e3N0eWxlfSBjbGFzc05hbWU9XCJidG4gYnRuLXByaW1hcnkgYnRuLWNpcmNsZSB0ZXh0LXVwcGVyY2FzZVwiPlxuICAgICAgICA8c3Ryb25nPntuYW1lWzBdfTwvc3Ryb25nPlxuICAgICAgPC9zcGFuPlxuICAgIDwvbGk+XG4gIClcbn07XG5cbmNvbnN0IFNlc3Npb25MZWZ0UGFuZWwgPSAoe3BhcnRpZXN9KSA9PiB7XG4gIHBhcnRpZXMgPSBwYXJ0aWVzIHx8IFtdO1xuICBsZXQgdXNlckljb25zID0gcGFydGllcy5tYXAoKGl0ZW0sIGluZGV4KT0+KFxuICAgIDxVc2VySWNvbiBrZXk9e2luZGV4fSBjb2xvckluZGV4PXtpbmRleH0gbmFtZT17aXRlbS51c2VyfS8+XG4gICkpO1xuXG4gIHJldHVybiAoXG4gICAgPGRpdiBjbGFzc05hbWU9XCJncnYtdGVybWluYWwtcGFydGljaXBhbnNcIj5cbiAgICAgIDx1bCBjbGFzc05hbWU9XCJuYXZcIj5cbiAgICAgICAge3VzZXJJY29uc31cbiAgICAgICAgPGxpPlxuICAgICAgICAgIDxidXR0b24gb25DbGljaz17YWN0aW9ucy5jbG9zZX0gY2xhc3NOYW1lPVwiYnRuIGJ0bi1kYW5nZXIgYnRuLWNpcmNsZVwiIHR5cGU9XCJidXR0b25cIj5cbiAgICAgICAgICAgIDxpIGNsYXNzTmFtZT1cImZhIGZhLXRpbWVzXCI+PC9pPlxuICAgICAgICAgIDwvYnV0dG9uPlxuICAgICAgICA8L2xpPlxuICAgICAgPC91bD5cbiAgICA8L2Rpdj5cbiAgKVxufTtcblxubW9kdWxlLmV4cG9ydHMgPSBTZXNzaW9uTGVmdFBhbmVsO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvY3VycmVudFNlc3Npb24vc2Vzc2lvbkxlZnRQYW5lbC5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xuXG52YXIgR29vZ2xlQXV0aEluZm8gPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIHJlbmRlcigpIHtcbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZ29vZ2xlLWF1dGhcIj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZ29vZ2xlLWF1dGgtaWNvblwiPjwvZGl2PlxuICAgICAgICA8c3Ryb25nPkdvb2dsZSBBdXRoZW50aWNhdG9yPC9zdHJvbmc+XG4gICAgICAgIDxkaXY+RG93bmxvYWQgPGEgaHJlZj1cImh0dHBzOi8vc3VwcG9ydC5nb29nbGUuY29tL2FjY291bnRzL2Fuc3dlci8xMDY2NDQ3P2hsPWVuXCI+R29vZ2xlIEF1dGhlbnRpY2F0b3I8L2E+IG9uIHlvdXIgcGhvbmUgdG8gYWNjZXNzIHlvdXIgdHdvIGZhY3RvcnkgdG9rZW48L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pXG5cbm1vZHVsZS5leHBvcnRzID0gR29vZ2xlQXV0aEluZm87XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9nb29nbGVBdXRoTG9nby5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHtnZXR0ZXJzLCBhY3Rpb25zfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL25vZGVzJyk7XG52YXIgdXNlckdldHRlcnMgPSByZXF1aXJlKCdhcHAvbW9kdWxlcy91c2VyL2dldHRlcnMnKTtcbnZhciB7VGFibGUsIENvbHVtbiwgQ2VsbH0gPSByZXF1aXJlKCdhcHAvY29tcG9uZW50cy90YWJsZS5qc3gnKTtcbnZhciB7Y3JlYXRlTmV3U2Vzc2lvbn0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3Rpb25zJyk7XG5cbmNvbnN0IFRleHRDZWxsID0gKHtyb3dJbmRleCwgZGF0YSwgY29sdW1uS2V5LCAuLi5wcm9wc30pID0+IChcbiAgPENlbGwgey4uLnByb3BzfT5cbiAgICB7ZGF0YVtyb3dJbmRleF1bY29sdW1uS2V5XX1cbiAgPC9DZWxsPlxuKTtcblxuY29uc3QgVGFnQ2VsbCA9ICh7cm93SW5kZXgsIGRhdGEsIGNvbHVtbktleSwgLi4ucHJvcHN9KSA9PiAoXG4gIDxDZWxsIHsuLi5wcm9wc30+XG4gICAgeyBkYXRhW3Jvd0luZGV4XS50YWdzLm1hcCgoaXRlbSwgaW5kZXgpID0+XG4gICAgICAoPHNwYW4ga2V5PXtpbmRleH0gY2xhc3NOYW1lPVwibGFiZWwgbGFiZWwtZGVmYXVsdFwiPlxuICAgICAgICB7aXRlbS5yb2xlfSA8bGkgY2xhc3NOYW1lPVwiZmEgZmEtbG9uZy1hcnJvdy1yaWdodFwiPjwvbGk+XG4gICAgICAgIHtpdGVtLnZhbHVlfVxuICAgICAgPC9zcGFuPilcbiAgICApIH1cbiAgPC9DZWxsPlxuKTtcblxuY29uc3QgTG9naW5DZWxsID0gKHt1c2VyLCBvbkxvZ2luQ2xpY2ssIHJvd0luZGV4LCBkYXRhLCAuLi5wcm9wc30pID0+IHtcbiAgaWYoIXVzZXIgfHwgdXNlci5sb2dpbnMubGVuZ3RoID09PSAwKXtcbiAgICByZXR1cm4gPENlbGwgey4uLnByb3BzfSAvPjtcbiAgfVxuXG4gIHZhciBzZXJ2ZXJJZCA9IGRhdGFbcm93SW5kZXhdLmlkO1xuICB2YXIgJGxpcyA9IFtdO1xuXG4gIGZ1bmN0aW9uIG9uQ2xpY2soaSl7XG4gICAgdmFyIGxvZ2luID0gdXNlci5sb2dpbnNbaV07XG4gICAgaWYob25Mb2dpbkNsaWNrKXtcbiAgICAgIHJldHVybiAoKT0+IG9uTG9naW5DbGljayhzZXJ2ZXJJZCwgbG9naW4pO1xuICAgIH1lbHNle1xuICAgICAgcmV0dXJuICgpID0+IGNyZWF0ZU5ld1Nlc3Npb24oc2VydmVySWQsIGxvZ2luKTtcbiAgICB9XG4gIH1cblxuICBmb3IodmFyIGkgPSAwOyBpIDwgdXNlci5sb2dpbnMubGVuZ3RoOyBpKyspe1xuICAgICRsaXMucHVzaCg8bGkga2V5PXtpfT48YSBvbkNsaWNrPXtvbkNsaWNrKGkpfT57dXNlci5sb2dpbnNbaV19PC9hPjwvbGk+KTtcbiAgfVxuXG4gIHJldHVybiAoXG4gICAgPENlbGwgey4uLnByb3BzfT5cbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiYnRuLWdyb3VwXCI+XG4gICAgICAgIDxidXR0b24gdHlwZT1cImJ1dHRvblwiIG9uQ2xpY2s9e29uQ2xpY2soMCl9IGNsYXNzTmFtZT1cImJ0biBidG4teHMgYnRuLXByaW1hcnlcIj57dXNlci5sb2dpbnNbMF19PC9idXR0b24+XG4gICAgICAgIHtcbiAgICAgICAgICAkbGlzLmxlbmd0aCA+IDEgPyAoXG4gICAgICAgICAgICAgIFtcbiAgICAgICAgICAgICAgICA8YnV0dG9uIGtleT17MH0gZGF0YS10b2dnbGU9XCJkcm9wZG93blwiIGNsYXNzTmFtZT1cImJ0biBidG4tZGVmYXVsdCBidG4teHMgZHJvcGRvd24tdG9nZ2xlXCIgYXJpYS1leHBhbmRlZD1cInRydWVcIj5cbiAgICAgICAgICAgICAgICAgIDxzcGFuIGNsYXNzTmFtZT1cImNhcmV0XCI+PC9zcGFuPlxuICAgICAgICAgICAgICAgIDwvYnV0dG9uPixcbiAgICAgICAgICAgICAgICA8dWwga2V5PXsxfSBjbGFzc05hbWU9XCJkcm9wZG93bi1tZW51XCI+XG4gICAgICAgICAgICAgICAgICB7JGxpc31cbiAgICAgICAgICAgICAgICA8L3VsPlxuICAgICAgICAgICAgICBdIClcbiAgICAgICAgICAgIDogbnVsbFxuICAgICAgICB9XG4gICAgICA8L2Rpdj5cbiAgICA8L0NlbGw+XG4gIClcbn07XG5cbnZhciBOb2RlcyA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBtaXhpbnM6IFtyZWFjdG9yLlJlYWN0TWl4aW5dLFxuXG4gIGdldERhdGFCaW5kaW5ncygpIHtcbiAgICByZXR1cm4ge1xuICAgICAgbm9kZVJlY29yZHM6IGdldHRlcnMubm9kZUxpc3RWaWV3LFxuICAgICAgdXNlcjogdXNlckdldHRlcnMudXNlclxuICAgIH1cbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIHZhciBkYXRhID0gdGhpcy5zdGF0ZS5ub2RlUmVjb3JkcztcbiAgICB2YXIgb25Mb2dpbkNsaWNrID0gdGhpcy5wcm9wcy5vbkxvZ2luQ2xpY2s7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LW5vZGVzXCI+XG4gICAgICAgIDxoMT4gTm9kZXMgPC9oMT5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICAgICAgPFRhYmxlIHJvd0NvdW50PXtkYXRhLmxlbmd0aH0gY2xhc3NOYW1lPVwidGFibGUtc3RyaXBlZCBncnYtbm9kZXMtdGFibGVcIj5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJzZXNzaW9uQ291bnRcIlxuICAgICAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD4gU2Vzc2lvbnMgPC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXs8VGV4dENlbGwgZGF0YT17ZGF0YX0vPiB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJhZGRyXCJcbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+IE5vZGUgPC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXs8VGV4dENlbGwgZGF0YT17ZGF0YX0vPiB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJ0YWdzXCJcbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+PC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXs8VGFnQ2VsbCBkYXRhPXtkYXRhfS8+IH1cbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICAgIGNvbHVtbktleT1cInJvbGVzXCJcbiAgICAgICAgICAgICAgICAgIG9uTG9naW5DbGljaz17b25Mb2dpbkNsaWNrfVxuICAgICAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD5Mb2dpbiBhczwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17PExvZ2luQ2VsbCBkYXRhPXtkYXRhfSB1c2VyPXt0aGlzLnN0YXRlLnVzZXJ9Lz4gfVxuICAgICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgIDwvVGFibGU+XG4gICAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApXG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IE5vZGVzO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvbm9kZXMvbWFpbi5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xuXG52YXIgTm90Rm91bmRQYWdlID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXBhZ2Utbm90Zm91bmRcIj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbG9nby10cHJ0XCI+VGVsZXBvcnQ8L2Rpdj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtd2FybmluZ1wiPjxpIGNsYXNzTmFtZT1cImZhIGZhLXdhcm5pbmdcIj48L2k+IDwvZGl2PlxuICAgICAgICA8aDE+V2hvb3BzLCB3ZSBjYW5ub3QgZmluZCB0aGF0PC9oMT5cbiAgICAgICAgPGRpdj5Mb29rcyBsaWtlIHRoZSBwYWdlIHlvdSBhcmUgbG9va2luZyBmb3IgaXNuJ3QgaGVyZSBhbnkgbG9uZ2VyPC9kaXY+XG4gICAgICAgIDxkaXY+SWYgeW91IGJlbGlldmUgdGhpcyBpcyBhbiBlcnJvciwgcGxlYXNlIGNvbnRhY3QgeW91ciBvcmdhbml6YXRpb24gYWRtaW5pc3RyYXRvci48L2Rpdj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJjb250YWN0LXNlY3Rpb25cIj5JZiB5b3UgYmVsaWV2ZSB0aGlzIGlzIGFuIGlzc3VlIHdpdGggVGVsZXBvcnQsIHBsZWFzZSA8YSBocmVmPVwiaHR0cHM6Ly9naXRodWIuY29tL2dyYXZpdGF0aW9uYWwvdGVsZXBvcnQvaXNzdWVzL25ld1wiPmNyZWF0ZSBhIEdpdEh1YiBpc3N1ZS48L2E+XG4gICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pXG5cbm1vZHVsZS5leHBvcnRzID0gTm90Rm91bmRQYWdlO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvbm90Rm91bmRQYWdlLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG5cbmNvbnN0IEdydlRhYmxlVGV4dENlbGwgPSAoe3Jvd0luZGV4LCBkYXRhLCBjb2x1bW5LZXksIC4uLnByb3BzfSkgPT4gKFxuICA8R3J2VGFibGVDZWxsIHsuLi5wcm9wc30+XG4gICAge2RhdGFbcm93SW5kZXhdW2NvbHVtbktleV19XG4gIDwvR3J2VGFibGVDZWxsPlxuKTtcblxudmFyIEdydlRhYmxlQ2VsbCA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgcmVuZGVyKCl7XG4gICAgdmFyIHByb3BzID0gdGhpcy5wcm9wcztcbiAgICByZXR1cm4gcHJvcHMuaXNIZWFkZXIgPyA8dGgga2V5PXtwcm9wcy5rZXl9Pntwcm9wcy5jaGlsZHJlbn08L3RoPiA6IDx0ZCBrZXk9e3Byb3BzLmtleX0+e3Byb3BzLmNoaWxkcmVufTwvdGQ+O1xuICB9XG59KTtcblxudmFyIEdydlRhYmxlID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIHJlbmRlckhlYWRlcihjaGlsZHJlbil7XG4gICAgdmFyIGNlbGxzID0gY2hpbGRyZW4ubWFwKChpdGVtLCBpbmRleCk9PntcbiAgICAgIHJldHVybiB0aGlzLnJlbmRlckNlbGwoaXRlbS5wcm9wcy5oZWFkZXIsIHtpbmRleCwga2V5OiBpbmRleCwgaXNIZWFkZXI6IHRydWUsIC4uLml0ZW0ucHJvcHN9KTtcbiAgICB9KVxuXG4gICAgcmV0dXJuIDx0aGVhZD48dHI+e2NlbGxzfTwvdHI+PC90aGVhZD5cbiAgfSxcblxuICByZW5kZXJCb2R5KGNoaWxkcmVuKXtcbiAgICB2YXIgY291bnQgPSB0aGlzLnByb3BzLnJvd0NvdW50O1xuICAgIHZhciByb3dzID0gW107XG4gICAgZm9yKHZhciBpID0gMDsgaSA8IGNvdW50OyBpICsrKXtcbiAgICAgIHZhciBjZWxscyA9IGNoaWxkcmVuLm1hcCgoaXRlbSwgaW5kZXgpPT57XG4gICAgICAgIHJldHVybiB0aGlzLnJlbmRlckNlbGwoaXRlbS5wcm9wcy5jZWxsLCB7cm93SW5kZXg6IGksIGtleTogaW5kZXgsIGlzSGVhZGVyOiBmYWxzZSwgLi4uaXRlbS5wcm9wc30pO1xuICAgICAgfSlcblxuICAgICAgcm93cy5wdXNoKDx0ciBrZXk9e2l9PntjZWxsc308L3RyPik7XG4gICAgfVxuXG4gICAgcmV0dXJuIDx0Ym9keT57cm93c308L3Rib2R5PjtcbiAgfSxcblxuICByZW5kZXJDZWxsKGNlbGwsIGNlbGxQcm9wcyl7XG4gICAgdmFyIGNvbnRlbnQgPSBudWxsO1xuICAgIGlmIChSZWFjdC5pc1ZhbGlkRWxlbWVudChjZWxsKSkge1xuICAgICAgIGNvbnRlbnQgPSBSZWFjdC5jbG9uZUVsZW1lbnQoY2VsbCwgY2VsbFByb3BzKTtcbiAgICAgfSBlbHNlIGlmICh0eXBlb2YgcHJvcHMuY2VsbCA9PT0gJ2Z1bmN0aW9uJykge1xuICAgICAgIGNvbnRlbnQgPSBjZWxsKGNlbGxQcm9wcyk7XG4gICAgIH1cblxuICAgICByZXR1cm4gY29udGVudDtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgdmFyIGNoaWxkcmVuID0gW107XG4gICAgUmVhY3QuQ2hpbGRyZW4uZm9yRWFjaCh0aGlzLnByb3BzLmNoaWxkcmVuLCAoY2hpbGQsIGluZGV4KSA9PiB7XG4gICAgICBpZiAoY2hpbGQgPT0gbnVsbCkge1xuICAgICAgICByZXR1cm47XG4gICAgICB9XG5cbiAgICAgIGlmKGNoaWxkLnR5cGUuZGlzcGxheU5hbWUgIT09ICdHcnZUYWJsZUNvbHVtbicpe1xuICAgICAgICB0aHJvdyAnU2hvdWxkIGJlIEdydlRhYmxlQ29sdW1uJztcbiAgICAgIH1cblxuICAgICAgY2hpbGRyZW4ucHVzaChjaGlsZCk7XG4gICAgfSk7XG5cbiAgICB2YXIgdGFibGVDbGFzcyA9ICd0YWJsZSAnICsgdGhpcy5wcm9wcy5jbGFzc05hbWU7XG5cbiAgICByZXR1cm4gKFxuICAgICAgPHRhYmxlIGNsYXNzTmFtZT17dGFibGVDbGFzc30+XG4gICAgICAgIHt0aGlzLnJlbmRlckhlYWRlcihjaGlsZHJlbil9XG4gICAgICAgIHt0aGlzLnJlbmRlckJvZHkoY2hpbGRyZW4pfVxuICAgICAgPC90YWJsZT5cbiAgICApO1xuICB9XG59KVxuXG52YXIgR3J2VGFibGVDb2x1bW4gPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgdGhyb3cgbmV3IEVycm9yKCdDb21wb25lbnQgPEdydlRhYmxlQ29sdW1uIC8+IHNob3VsZCBuZXZlciByZW5kZXInKTtcbiAgfVxufSlcblxuZXhwb3J0IGRlZmF1bHQgR3J2VGFibGU7XG5leHBvcnQge0dydlRhYmxlQ29sdW1uIGFzIENvbHVtbiwgR3J2VGFibGUgYXMgVGFibGUsIEdydlRhYmxlQ2VsbCBhcyBDZWxsLCBHcnZUYWJsZVRleHRDZWxsIGFzIFRleHRDZWxsfTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL3RhYmxlLmpzeFxuICoqLyIsInZhciBUZXJtID0gcmVxdWlyZSgnVGVybWluYWwnKTtcbnZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIge2RlYm91bmNlLCBpc051bWJlcn0gPSByZXF1aXJlKCdfJyk7XG5cblRlcm0uY29sb3JzWzI1Nl0gPSAnaW5oZXJpdCc7XG5cbmNvbnN0IERJU0NPTk5FQ1RfVFhUID0gJ1xceDFiWzMxbWRpc2Nvbm5lY3RlZFxceDFiW21cXHJcXG4nO1xuY29uc3QgQ09OTkVDVEVEX1RYVCA9ICdDb25uZWN0ZWQhXFxyXFxuJztcblxudmFyIFR0eVRlcm1pbmFsID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIGdldEluaXRpYWxTdGF0ZSgpe1xuICAgIHRoaXMucm93cyA9IHRoaXMucHJvcHMucm93cztcbiAgICB0aGlzLmNvbHMgPSB0aGlzLnByb3BzLmNvbHM7XG4gICAgdGhpcy50dHkgPSB0aGlzLnByb3BzLnR0eTtcblxuICAgIHRoaXMuZGVib3VuY2VkUmVzaXplID0gZGVib3VuY2UoKCk9PntcbiAgICAgIHRoaXMucmVzaXplKCk7XG4gICAgICB0aGlzLnR0eS5yZXNpemUodGhpcy5jb2xzLCB0aGlzLnJvd3MpO1xuICAgIH0sIDIwMCk7XG5cbiAgICByZXR1cm4ge307XG4gIH0sXG5cbiAgY29tcG9uZW50RGlkTW91bnQ6IGZ1bmN0aW9uKCkge1xuICAgIHRoaXMudGVybSA9IG5ldyBUZXJtaW5hbCh7XG4gICAgICBjb2xzOiA1LFxuICAgICAgcm93czogNSxcbiAgICAgIHVzZVN0eWxlOiB0cnVlLFxuICAgICAgc2NyZWVuS2V5czogdHJ1ZSxcbiAgICAgIGN1cnNvckJsaW5rOiB0cnVlXG4gICAgfSk7XG5cbiAgICB0aGlzLnRlcm0ub3Blbih0aGlzLnJlZnMuY29udGFpbmVyKTtcbiAgICB0aGlzLnRlcm0ub24oJ2RhdGEnLCAoZGF0YSkgPT4gdGhpcy50dHkuc2VuZChkYXRhKSk7XG5cbiAgICB0aGlzLnJlc2l6ZSh0aGlzLmNvbHMsIHRoaXMucm93cyk7XG5cbiAgICB0aGlzLnR0eS5vbignb3BlbicsICgpPT4gdGhpcy50ZXJtLndyaXRlKENPTk5FQ1RFRF9UWFQpKTtcbiAgICB0aGlzLnR0eS5vbignY2xvc2UnLCAoKT0+IHRoaXMudGVybS53cml0ZShESVNDT05ORUNUX1RYVCkpO1xuICAgIHRoaXMudHR5Lm9uKCdkYXRhJywgKGRhdGEpID0+IHRoaXMudGVybS53cml0ZShkYXRhKSk7XG4gICAgdGhpcy50dHkub24oJ3Jlc2V0JywgKCk9PiB0aGlzLnRlcm0ucmVzZXQoKSk7XG5cbiAgICB0aGlzLnR0eS5jb25uZWN0KHtjb2xzOiB0aGlzLmNvbHMsIHJvd3M6IHRoaXMucm93c30pO1xuICAgIHdpbmRvdy5hZGRFdmVudExpc3RlbmVyKCdyZXNpemUnLCB0aGlzLmRlYm91bmNlZFJlc2l6ZSk7XG4gIH0sXG5cbiAgY29tcG9uZW50V2lsbFVubW91bnQ6IGZ1bmN0aW9uKCkge1xuICAgIHRoaXMudGVybS5kZXN0cm95KCk7XG4gICAgd2luZG93LnJlbW92ZUV2ZW50TGlzdGVuZXIoJ3Jlc2l6ZScsIHRoaXMuZGVib3VuY2VkUmVzaXplKTtcbiAgfSxcblxuICBzaG91bGRDb21wb25lbnRVcGRhdGU6IGZ1bmN0aW9uKG5ld1Byb3BzKSB7XG4gICAgdmFyIHtyb3dzLCBjb2xzfSA9IG5ld1Byb3BzO1xuXG4gICAgaWYoICFpc051bWJlcihyb3dzKSB8fCAhaXNOdW1iZXIoY29scykpe1xuICAgICAgcmV0dXJuIGZhbHNlO1xuICAgIH1cblxuICAgIGlmKHJvd3MgIT09IHRoaXMucm93cyB8fCBjb2xzICE9PSB0aGlzLmNvbHMpe1xuICAgICAgdGhpcy5yZXNpemUoY29scywgcm93cylcbiAgICB9XG5cbiAgICByZXR1cm4gZmFsc2U7XG4gIH0sXG5cbiAgcmVuZGVyKCkge1xuICAgIHJldHVybiAoIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXRlcm1pbmFsXCIgaWQ9XCJ0ZXJtaW5hbC1ib3hcIiByZWY9XCJjb250YWluZXJcIj4gIDwvZGl2PiApO1xuICB9LFxuXG4gIHJlc2l6ZTogZnVuY3Rpb24oY29scywgcm93cykge1xuICAgIC8vIGlmIG5vdCBkZWZpbmVkLCB1c2UgdGhlIHNpemUgb2YgdGhlIGNvbnRhaW5lclxuICAgIGlmKCFpc051bWJlcihjb2xzKSB8fCAhaXNOdW1iZXIocm93cykpe1xuICAgICAgbGV0IGRpbSA9IHRoaXMuX2dldERpbWVuc2lvbnMoKTtcbiAgICAgIGNvbHMgPSBkaW0uY29scztcbiAgICAgIHJvd3MgPSBkaW0ucm93cztcbiAgICB9XG5cbiAgICB0aGlzLmNvbHMgPSBjb2xzO1xuICAgIHRoaXMucm93cyA9IHJvd3M7XG5cbiAgICB0aGlzLnRlcm0ucmVzaXplKHRoaXMuY29scywgdGhpcy5yb3dzKTtcbiAgfSxcblxuICBfZ2V0RGltZW5zaW9ucygpe1xuICAgIGxldCAkY29udGFpbmVyID0gJCh0aGlzLnJlZnMuY29udGFpbmVyKTtcbiAgICBsZXQgZmFrZVJvdyA9ICQoJzxkaXY+PHNwYW4+Jm5ic3A7PC9zcGFuPjwvZGl2PicpO1xuXG4gICAgJGNvbnRhaW5lci5maW5kKCcudGVybWluYWwnKS5hcHBlbmQoZmFrZVJvdyk7XG4gICAgLy8gZ2V0IGRpdiBoZWlnaHRcbiAgICBsZXQgZmFrZUNvbEhlaWdodCA9IGZha2VSb3dbMF0uZ2V0Qm91bmRpbmdDbGllbnRSZWN0KCkuaGVpZ2h0O1xuICAgIC8vIGdldCBzcGFuIHdpZHRoXG4gICAgbGV0IGZha2VDb2xXaWR0aCA9IGZha2VSb3cuY2hpbGRyZW4oKS5maXJzdCgpWzBdLmdldEJvdW5kaW5nQ2xpZW50UmVjdCgpLndpZHRoO1xuICAgIGxldCBjb2xzID0gTWF0aC5mbG9vcigkY29udGFpbmVyLndpZHRoKCkgLyAoZmFrZUNvbFdpZHRoKSk7XG4gICAgbGV0IHJvd3MgPSBNYXRoLmZsb29yKCRjb250YWluZXIuaGVpZ2h0KCkgLyAoZmFrZUNvbEhlaWdodCkpO1xuICAgIGZha2VSb3cucmVtb3ZlKCk7XG5cbiAgICByZXR1cm4ge2NvbHMsIHJvd3N9O1xuICB9XG5cbn0pO1xuXG5UdHlUZXJtaW5hbC5wcm9wVHlwZXMgPSB7XG4gIHR0eTogUmVhY3QuUHJvcFR5cGVzLm9iamVjdC5pc1JlcXVpcmVkXG59XG5cbm1vZHVsZS5leHBvcnRzID0gVHR5VGVybWluYWw7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy90ZXJtaW5hbC5qc3hcbiAqKi8iLCIvKlxuICogIFRoZSBNSVQgTGljZW5zZSAoTUlUKVxuICogIENvcHlyaWdodCAoYykgMjAxNSBSeWFuIEZsb3JlbmNlLCBNaWNoYWVsIEphY2tzb25cbiAqICBQZXJtaXNzaW9uIGlzIGhlcmVieSBncmFudGVkLCBmcmVlIG9mIGNoYXJnZSwgdG8gYW55IHBlcnNvbiBvYnRhaW5pbmcgYSBjb3B5IG9mIHRoaXMgc29mdHdhcmUgYW5kIGFzc29jaWF0ZWQgZG9jdW1lbnRhdGlvbiBmaWxlcyAodGhlIFwiU29mdHdhcmVcIiksIHRvIGRlYWwgaW4gdGhlIFNvZnR3YXJlIHdpdGhvdXQgcmVzdHJpY3Rpb24sIGluY2x1ZGluZyB3aXRob3V0IGxpbWl0YXRpb24gdGhlIHJpZ2h0cyB0byB1c2UsIGNvcHksIG1vZGlmeSwgbWVyZ2UsIHB1Ymxpc2gsIGRpc3RyaWJ1dGUsIHN1YmxpY2Vuc2UsIGFuZC9vciBzZWxsIGNvcGllcyBvZiB0aGUgU29mdHdhcmUsIGFuZCB0byBwZXJtaXQgcGVyc29ucyB0byB3aG9tIHRoZSBTb2Z0d2FyZSBpcyBmdXJuaXNoZWQgdG8gZG8gc28sIHN1YmplY3QgdG8gdGhlIGZvbGxvd2luZyBjb25kaXRpb25zOlxuICogIFRoZSBhYm92ZSBjb3B5cmlnaHQgbm90aWNlIGFuZCB0aGlzIHBlcm1pc3Npb24gbm90aWNlIHNoYWxsIGJlIGluY2x1ZGVkIGluIGFsbCBjb3BpZXMgb3Igc3Vic3RhbnRpYWwgcG9ydGlvbnMgb2YgdGhlIFNvZnR3YXJlLlxuICogIFRIRSBTT0ZUV0FSRSBJUyBQUk9WSURFRCBcIkFTIElTXCIsIFdJVEhPVVQgV0FSUkFOVFkgT0YgQU5ZIEtJTkQsIEVYUFJFU1MgT1IgSU1QTElFRCwgSU5DTFVESU5HIEJVVCBOT1QgTElNSVRFRCBUTyBUSEUgV0FSUkFOVElFUyBPRiBNRVJDSEFOVEFCSUxJVFksIEZJVE5FU1MgRk9SIEEgUEFSVElDVUxBUiBQVVJQT1NFIEFORCBOT05JTkZSSU5HRU1FTlQuIElOIE5PIEVWRU5UIFNIQUxMIFRIRSBBVVRIT1JTIE9SIENPUFlSSUdIVCBIT0xERVJTIEJFIExJQUJMRSBGT1IgQU5ZIENMQUlNLCBEQU1BR0VTIE9SIE9USEVSIExJQUJJTElUWSwgV0hFVEhFUiBJTiBBTiBBQ1RJT04gT0YgQ09OVFJBQ1QsIFRPUlQgT1IgT1RIRVJXSVNFLCBBUklTSU5HIEZST00sIE9VVCBPRiBPUiBJTiBDT05ORUNUSU9OIFdJVEggVEhFIFNPRlRXQVJFIE9SIFRIRSBVU0UgT1IgT1RIRVIgREVBTElOR1MgSU4gVEhFIFNPRlRXQVJFLlxuKi9cblxuaW1wb3J0IGludmFyaWFudCBmcm9tICdpbnZhcmlhbnQnXG5cbmZ1bmN0aW9uIGVzY2FwZVJlZ0V4cChzdHJpbmcpIHtcbiAgcmV0dXJuIHN0cmluZy5yZXBsYWNlKC9bLiorP14ke30oKXxbXFxdXFxcXF0vZywgJ1xcXFwkJicpXG59XG5cbmZ1bmN0aW9uIGVzY2FwZVNvdXJjZShzdHJpbmcpIHtcbiAgcmV0dXJuIGVzY2FwZVJlZ0V4cChzdHJpbmcpLnJlcGxhY2UoL1xcLysvZywgJy8rJylcbn1cblxuZnVuY3Rpb24gX2NvbXBpbGVQYXR0ZXJuKHBhdHRlcm4pIHtcbiAgbGV0IHJlZ2V4cFNvdXJjZSA9ICcnO1xuICBjb25zdCBwYXJhbU5hbWVzID0gW107XG4gIGNvbnN0IHRva2VucyA9IFtdO1xuXG4gIGxldCBtYXRjaCwgbGFzdEluZGV4ID0gMCwgbWF0Y2hlciA9IC86KFthLXpBLVpfJF1bYS16QS1aMC05XyRdKil8XFwqXFwqfFxcKnxcXCh8XFwpL2dcbiAgLyplc2xpbnQgbm8tY29uZC1hc3NpZ246IDAqL1xuICB3aGlsZSAoKG1hdGNoID0gbWF0Y2hlci5leGVjKHBhdHRlcm4pKSkge1xuICAgIGlmIChtYXRjaC5pbmRleCAhPT0gbGFzdEluZGV4KSB7XG4gICAgICB0b2tlbnMucHVzaChwYXR0ZXJuLnNsaWNlKGxhc3RJbmRleCwgbWF0Y2guaW5kZXgpKVxuICAgICAgcmVnZXhwU291cmNlICs9IGVzY2FwZVNvdXJjZShwYXR0ZXJuLnNsaWNlKGxhc3RJbmRleCwgbWF0Y2guaW5kZXgpKVxuICAgIH1cblxuICAgIGlmIChtYXRjaFsxXSkge1xuICAgICAgcmVnZXhwU291cmNlICs9ICcoW14vPyNdKyknO1xuICAgICAgcGFyYW1OYW1lcy5wdXNoKG1hdGNoWzFdKTtcbiAgICB9IGVsc2UgaWYgKG1hdGNoWzBdID09PSAnKionKSB7XG4gICAgICByZWdleHBTb3VyY2UgKz0gJyhbXFxcXHNcXFxcU10qKSdcbiAgICAgIHBhcmFtTmFtZXMucHVzaCgnc3BsYXQnKTtcbiAgICB9IGVsc2UgaWYgKG1hdGNoWzBdID09PSAnKicpIHtcbiAgICAgIHJlZ2V4cFNvdXJjZSArPSAnKFtcXFxcc1xcXFxTXSo/KSdcbiAgICAgIHBhcmFtTmFtZXMucHVzaCgnc3BsYXQnKTtcbiAgICB9IGVsc2UgaWYgKG1hdGNoWzBdID09PSAnKCcpIHtcbiAgICAgIHJlZ2V4cFNvdXJjZSArPSAnKD86JztcbiAgICB9IGVsc2UgaWYgKG1hdGNoWzBdID09PSAnKScpIHtcbiAgICAgIHJlZ2V4cFNvdXJjZSArPSAnKT8nO1xuICAgIH1cblxuICAgIHRva2Vucy5wdXNoKG1hdGNoWzBdKTtcblxuICAgIGxhc3RJbmRleCA9IG1hdGNoZXIubGFzdEluZGV4O1xuICB9XG5cbiAgaWYgKGxhc3RJbmRleCAhPT0gcGF0dGVybi5sZW5ndGgpIHtcbiAgICB0b2tlbnMucHVzaChwYXR0ZXJuLnNsaWNlKGxhc3RJbmRleCwgcGF0dGVybi5sZW5ndGgpKVxuICAgIHJlZ2V4cFNvdXJjZSArPSBlc2NhcGVTb3VyY2UocGF0dGVybi5zbGljZShsYXN0SW5kZXgsIHBhdHRlcm4ubGVuZ3RoKSlcbiAgfVxuXG4gIHJldHVybiB7XG4gICAgcGF0dGVybixcbiAgICByZWdleHBTb3VyY2UsXG4gICAgcGFyYW1OYW1lcyxcbiAgICB0b2tlbnNcbiAgfVxufVxuXG5jb25zdCBDb21waWxlZFBhdHRlcm5zQ2FjaGUgPSB7fVxuXG5leHBvcnQgZnVuY3Rpb24gY29tcGlsZVBhdHRlcm4ocGF0dGVybikge1xuICBpZiAoIShwYXR0ZXJuIGluIENvbXBpbGVkUGF0dGVybnNDYWNoZSkpXG4gICAgQ29tcGlsZWRQYXR0ZXJuc0NhY2hlW3BhdHRlcm5dID0gX2NvbXBpbGVQYXR0ZXJuKHBhdHRlcm4pXG5cbiAgcmV0dXJuIENvbXBpbGVkUGF0dGVybnNDYWNoZVtwYXR0ZXJuXVxufVxuXG4vKipcbiAqIEF0dGVtcHRzIHRvIG1hdGNoIGEgcGF0dGVybiBvbiB0aGUgZ2l2ZW4gcGF0aG5hbWUuIFBhdHRlcm5zIG1heSB1c2VcbiAqIHRoZSBmb2xsb3dpbmcgc3BlY2lhbCBjaGFyYWN0ZXJzOlxuICpcbiAqIC0gOnBhcmFtTmFtZSAgICAgTWF0Y2hlcyBhIFVSTCBzZWdtZW50IHVwIHRvIHRoZSBuZXh0IC8sID8sIG9yICMuIFRoZVxuICogICAgICAgICAgICAgICAgICBjYXB0dXJlZCBzdHJpbmcgaXMgY29uc2lkZXJlZCBhIFwicGFyYW1cIlxuICogLSAoKSAgICAgICAgICAgICBXcmFwcyBhIHNlZ21lbnQgb2YgdGhlIFVSTCB0aGF0IGlzIG9wdGlvbmFsXG4gKiAtICogICAgICAgICAgICAgIENvbnN1bWVzIChub24tZ3JlZWR5KSBhbGwgY2hhcmFjdGVycyB1cCB0byB0aGUgbmV4dFxuICogICAgICAgICAgICAgICAgICBjaGFyYWN0ZXIgaW4gdGhlIHBhdHRlcm4sIG9yIHRvIHRoZSBlbmQgb2YgdGhlIFVSTCBpZlxuICogICAgICAgICAgICAgICAgICB0aGVyZSBpcyBub25lXG4gKiAtICoqICAgICAgICAgICAgIENvbnN1bWVzIChncmVlZHkpIGFsbCBjaGFyYWN0ZXJzIHVwIHRvIHRoZSBuZXh0IGNoYXJhY3RlclxuICogICAgICAgICAgICAgICAgICBpbiB0aGUgcGF0dGVybiwgb3IgdG8gdGhlIGVuZCBvZiB0aGUgVVJMIGlmIHRoZXJlIGlzIG5vbmVcbiAqXG4gKiBUaGUgcmV0dXJuIHZhbHVlIGlzIGFuIG9iamVjdCB3aXRoIHRoZSBmb2xsb3dpbmcgcHJvcGVydGllczpcbiAqXG4gKiAtIHJlbWFpbmluZ1BhdGhuYW1lXG4gKiAtIHBhcmFtTmFtZXNcbiAqIC0gcGFyYW1WYWx1ZXNcbiAqL1xuZXhwb3J0IGZ1bmN0aW9uIG1hdGNoUGF0dGVybihwYXR0ZXJuLCBwYXRobmFtZSkge1xuICAvLyBNYWtlIGxlYWRpbmcgc2xhc2hlcyBjb25zaXN0ZW50IGJldHdlZW4gcGF0dGVybiBhbmQgcGF0aG5hbWUuXG4gIGlmIChwYXR0ZXJuLmNoYXJBdCgwKSAhPT0gJy8nKSB7XG4gICAgcGF0dGVybiA9IGAvJHtwYXR0ZXJufWBcbiAgfVxuICBpZiAocGF0aG5hbWUuY2hhckF0KDApICE9PSAnLycpIHtcbiAgICBwYXRobmFtZSA9IGAvJHtwYXRobmFtZX1gXG4gIH1cblxuICBsZXQgeyByZWdleHBTb3VyY2UsIHBhcmFtTmFtZXMsIHRva2VucyB9ID0gY29tcGlsZVBhdHRlcm4ocGF0dGVybilcblxuICByZWdleHBTb3VyY2UgKz0gJy8qJyAvLyBDYXB0dXJlIHBhdGggc2VwYXJhdG9yc1xuXG4gIC8vIFNwZWNpYWwtY2FzZSBwYXR0ZXJucyBsaWtlICcqJyBmb3IgY2F0Y2gtYWxsIHJvdXRlcy5cbiAgY29uc3QgY2FwdHVyZVJlbWFpbmluZyA9IHRva2Vuc1t0b2tlbnMubGVuZ3RoIC0gMV0gIT09ICcqJ1xuXG4gIGlmIChjYXB0dXJlUmVtYWluaW5nKSB7XG4gICAgLy8gVGhpcyB3aWxsIG1hdGNoIG5ld2xpbmVzIGluIHRoZSByZW1haW5pbmcgcGF0aC5cbiAgICByZWdleHBTb3VyY2UgKz0gJyhbXFxcXHNcXFxcU10qPyknXG4gIH1cblxuICBjb25zdCBtYXRjaCA9IHBhdGhuYW1lLm1hdGNoKG5ldyBSZWdFeHAoJ14nICsgcmVnZXhwU291cmNlICsgJyQnLCAnaScpKVxuXG4gIGxldCByZW1haW5pbmdQYXRobmFtZSwgcGFyYW1WYWx1ZXNcbiAgaWYgKG1hdGNoICE9IG51bGwpIHtcbiAgICBpZiAoY2FwdHVyZVJlbWFpbmluZykge1xuICAgICAgcmVtYWluaW5nUGF0aG5hbWUgPSBtYXRjaC5wb3AoKVxuICAgICAgY29uc3QgbWF0Y2hlZFBhdGggPVxuICAgICAgICBtYXRjaFswXS5zdWJzdHIoMCwgbWF0Y2hbMF0ubGVuZ3RoIC0gcmVtYWluaW5nUGF0aG5hbWUubGVuZ3RoKVxuXG4gICAgICAvLyBJZiB3ZSBkaWRuJ3QgbWF0Y2ggdGhlIGVudGlyZSBwYXRobmFtZSwgdGhlbiBtYWtlIHN1cmUgdGhhdCB0aGUgbWF0Y2hcbiAgICAgIC8vIHdlIGRpZCBnZXQgZW5kcyBhdCBhIHBhdGggc2VwYXJhdG9yIChwb3RlbnRpYWxseSB0aGUgb25lIHdlIGFkZGVkXG4gICAgICAvLyBhYm92ZSBhdCB0aGUgYmVnaW5uaW5nIG9mIHRoZSBwYXRoLCBpZiB0aGUgYWN0dWFsIG1hdGNoIHdhcyBlbXB0eSkuXG4gICAgICBpZiAoXG4gICAgICAgIHJlbWFpbmluZ1BhdGhuYW1lICYmXG4gICAgICAgIG1hdGNoZWRQYXRoLmNoYXJBdChtYXRjaGVkUGF0aC5sZW5ndGggLSAxKSAhPT0gJy8nXG4gICAgICApIHtcbiAgICAgICAgcmV0dXJuIHtcbiAgICAgICAgICByZW1haW5pbmdQYXRobmFtZTogbnVsbCxcbiAgICAgICAgICBwYXJhbU5hbWVzLFxuICAgICAgICAgIHBhcmFtVmFsdWVzOiBudWxsXG4gICAgICAgIH1cbiAgICAgIH1cbiAgICB9IGVsc2Uge1xuICAgICAgLy8gSWYgdGhpcyBtYXRjaGVkIGF0IGFsbCwgdGhlbiB0aGUgbWF0Y2ggd2FzIHRoZSBlbnRpcmUgcGF0aG5hbWUuXG4gICAgICByZW1haW5pbmdQYXRobmFtZSA9ICcnXG4gICAgfVxuXG4gICAgcGFyYW1WYWx1ZXMgPSBtYXRjaC5zbGljZSgxKS5tYXAoXG4gICAgICB2ID0+IHYgIT0gbnVsbCA/IGRlY29kZVVSSUNvbXBvbmVudCh2KSA6IHZcbiAgICApXG4gIH0gZWxzZSB7XG4gICAgcmVtYWluaW5nUGF0aG5hbWUgPSBwYXJhbVZhbHVlcyA9IG51bGxcbiAgfVxuXG4gIHJldHVybiB7XG4gICAgcmVtYWluaW5nUGF0aG5hbWUsXG4gICAgcGFyYW1OYW1lcyxcbiAgICBwYXJhbVZhbHVlc1xuICB9XG59XG5cbmV4cG9ydCBmdW5jdGlvbiBnZXRQYXJhbU5hbWVzKHBhdHRlcm4pIHtcbiAgcmV0dXJuIGNvbXBpbGVQYXR0ZXJuKHBhdHRlcm4pLnBhcmFtTmFtZXNcbn1cblxuZXhwb3J0IGZ1bmN0aW9uIGdldFBhcmFtcyhwYXR0ZXJuLCBwYXRobmFtZSkge1xuICBjb25zdCB7IHBhcmFtTmFtZXMsIHBhcmFtVmFsdWVzIH0gPSBtYXRjaFBhdHRlcm4ocGF0dGVybiwgcGF0aG5hbWUpXG5cbiAgaWYgKHBhcmFtVmFsdWVzICE9IG51bGwpIHtcbiAgICByZXR1cm4gcGFyYW1OYW1lcy5yZWR1Y2UoZnVuY3Rpb24gKG1lbW8sIHBhcmFtTmFtZSwgaW5kZXgpIHtcbiAgICAgIG1lbW9bcGFyYW1OYW1lXSA9IHBhcmFtVmFsdWVzW2luZGV4XVxuICAgICAgcmV0dXJuIG1lbW9cbiAgICB9LCB7fSlcbiAgfVxuXG4gIHJldHVybiBudWxsXG59XG5cbi8qKlxuICogUmV0dXJucyBhIHZlcnNpb24gb2YgdGhlIGdpdmVuIHBhdHRlcm4gd2l0aCBwYXJhbXMgaW50ZXJwb2xhdGVkLiBUaHJvd3NcbiAqIGlmIHRoZXJlIGlzIGEgZHluYW1pYyBzZWdtZW50IG9mIHRoZSBwYXR0ZXJuIGZvciB3aGljaCB0aGVyZSBpcyBubyBwYXJhbS5cbiAqL1xuZXhwb3J0IGZ1bmN0aW9uIGZvcm1hdFBhdHRlcm4ocGF0dGVybiwgcGFyYW1zKSB7XG4gIHBhcmFtcyA9IHBhcmFtcyB8fCB7fVxuXG4gIGNvbnN0IHsgdG9rZW5zIH0gPSBjb21waWxlUGF0dGVybihwYXR0ZXJuKVxuICBsZXQgcGFyZW5Db3VudCA9IDAsIHBhdGhuYW1lID0gJycsIHNwbGF0SW5kZXggPSAwXG5cbiAgbGV0IHRva2VuLCBwYXJhbU5hbWUsIHBhcmFtVmFsdWVcbiAgZm9yIChsZXQgaSA9IDAsIGxlbiA9IHRva2Vucy5sZW5ndGg7IGkgPCBsZW47ICsraSkge1xuICAgIHRva2VuID0gdG9rZW5zW2ldXG5cbiAgICBpZiAodG9rZW4gPT09ICcqJyB8fCB0b2tlbiA9PT0gJyoqJykge1xuICAgICAgcGFyYW1WYWx1ZSA9IEFycmF5LmlzQXJyYXkocGFyYW1zLnNwbGF0KSA/IHBhcmFtcy5zcGxhdFtzcGxhdEluZGV4KytdIDogcGFyYW1zLnNwbGF0XG5cbiAgICAgIGludmFyaWFudChcbiAgICAgICAgcGFyYW1WYWx1ZSAhPSBudWxsIHx8IHBhcmVuQ291bnQgPiAwLFxuICAgICAgICAnTWlzc2luZyBzcGxhdCAjJXMgZm9yIHBhdGggXCIlc1wiJyxcbiAgICAgICAgc3BsYXRJbmRleCwgcGF0dGVyblxuICAgICAgKVxuXG4gICAgICBpZiAocGFyYW1WYWx1ZSAhPSBudWxsKVxuICAgICAgICBwYXRobmFtZSArPSBlbmNvZGVVUkkocGFyYW1WYWx1ZSlcbiAgICB9IGVsc2UgaWYgKHRva2VuID09PSAnKCcpIHtcbiAgICAgIHBhcmVuQ291bnQgKz0gMVxuICAgIH0gZWxzZSBpZiAodG9rZW4gPT09ICcpJykge1xuICAgICAgcGFyZW5Db3VudCAtPSAxXG4gICAgfSBlbHNlIGlmICh0b2tlbi5jaGFyQXQoMCkgPT09ICc6Jykge1xuICAgICAgcGFyYW1OYW1lID0gdG9rZW4uc3Vic3RyaW5nKDEpXG4gICAgICBwYXJhbVZhbHVlID0gcGFyYW1zW3BhcmFtTmFtZV1cblxuICAgICAgaW52YXJpYW50KFxuICAgICAgICBwYXJhbVZhbHVlICE9IG51bGwgfHwgcGFyZW5Db3VudCA+IDAsXG4gICAgICAgICdNaXNzaW5nIFwiJXNcIiBwYXJhbWV0ZXIgZm9yIHBhdGggXCIlc1wiJyxcbiAgICAgICAgcGFyYW1OYW1lLCBwYXR0ZXJuXG4gICAgICApXG5cbiAgICAgIGlmIChwYXJhbVZhbHVlICE9IG51bGwpXG4gICAgICAgIHBhdGhuYW1lICs9IGVuY29kZVVSSUNvbXBvbmVudChwYXJhbVZhbHVlKVxuICAgIH0gZWxzZSB7XG4gICAgICBwYXRobmFtZSArPSB0b2tlblxuICAgIH1cbiAgfVxuXG4gIHJldHVybiBwYXRobmFtZS5yZXBsYWNlKC9cXC8rL2csICcvJylcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21tb24vcGF0dGVyblV0aWxzLmpzXG4gKiovIiwidmFyIFR0eSA9IHJlcXVpcmUoJ2FwcC9jb21tb24vdHR5Jyk7XG52YXIgYXBpID0gcmVxdWlyZSgnYXBwL3NlcnZpY2VzL2FwaScpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxuY2xhc3MgVHR5UGxheWVyIGV4dGVuZHMgVHR5IHtcbiAgY29uc3RydWN0b3Ioe3NpZH0pe1xuICAgIHN1cGVyKHt9KTtcbiAgICB0aGlzLnNpZCA9IHNpZDtcbiAgICB0aGlzLmN1cnJlbnQgPSAxO1xuICAgIHRoaXMubGVuZ3RoID0gLTE7XG4gICAgdGhpcy50dHlTdGVhbSA9IG5ldyBBcnJheSgpO1xuICAgIHRoaXMuaXNMb2FpbmQgPSBmYWxzZTtcbiAgICB0aGlzLmlzUGxheWluZyA9IGZhbHNlO1xuICAgIHRoaXMuaXNFcnJvciA9IGZhbHNlO1xuICAgIHRoaXMuaXNSZWFkeSA9IGZhbHNlO1xuICAgIHRoaXMuaXNMb2FkaW5nID0gdHJ1ZTtcbiAgfVxuXG4gIHNlbmQoKXtcbiAgfVxuXG4gIHJlc2l6ZSgpe1xuICB9XG4gIFxuICBjb25uZWN0KCl7XG4gICAgYXBpLmdldChjZmcuYXBpLmdldEZldGNoU2Vzc2lvbkxlbmd0aFVybCh0aGlzLnNpZCkpXG4gICAgICAuZG9uZSgoZGF0YSk9PntcbiAgICAgICAgdGhpcy5sZW5ndGggPSBkYXRhLmNvdW50O1xuICAgICAgICB0aGlzLmlzUmVhZHkgPSB0cnVlO1xuICAgICAgfSlcbiAgICAgIC5mYWlsKCgpPT57XG4gICAgICAgIHRoaXMuaXNFcnJvciA9IHRydWU7XG4gICAgICB9KVxuICAgICAgLmFsd2F5cygoKT0+e1xuICAgICAgICB0aGlzLl9jaGFuZ2UoKTtcbiAgICAgIH0pO1xuICB9XG5cbiAgbW92ZShuZXdQb3Mpe1xuICAgIGlmKCF0aGlzLmlzUmVhZHkpe1xuICAgICAgcmV0dXJuO1xuICAgIH1cblxuICAgIGlmKG5ld1BvcyA9PT0gdW5kZWZpbmVkKXtcbiAgICAgIG5ld1BvcyA9IHRoaXMuY3VycmVudCArIDE7XG4gICAgfVxuXG4gICAgaWYobmV3UG9zID4gdGhpcy5sZW5ndGgpe1xuICAgICAgbmV3UG9zID0gdGhpcy5sZW5ndGg7XG4gICAgICB0aGlzLnN0b3AoKTtcbiAgICB9XG5cbiAgICBpZihuZXdQb3MgPT09IDApe1xuICAgICAgbmV3UG9zID0gMTtcbiAgICB9XG5cbiAgICBpZih0aGlzLmlzUGxheWluZyl7XG4gICAgICBpZih0aGlzLmN1cnJlbnQgPCBuZXdQb3Mpe1xuICAgICAgICB0aGlzLl9zaG93Q2h1bmsodGhpcy5jdXJyZW50LCBuZXdQb3MpO1xuICAgICAgfWVsc2V7XG4gICAgICAgIHRoaXMuZW1pdCgncmVzZXQnKTtcbiAgICAgICAgdGhpcy5fc2hvd0NodW5rKHRoaXMuY3VycmVudCwgbmV3UG9zKTtcbiAgICAgIH1cbiAgICB9ZWxzZXtcbiAgICAgIHRoaXMuY3VycmVudCA9IG5ld1BvcztcbiAgICB9XG5cbiAgICB0aGlzLl9jaGFuZ2UoKTtcbiAgfVxuXG4gIHN0b3AoKXtcbiAgICB0aGlzLmlzUGxheWluZyA9IGZhbHNlO1xuICAgIHRoaXMudGltZXIgPSBjbGVhckludGVydmFsKHRoaXMudGltZXIpO1xuICAgIHRoaXMuX2NoYW5nZSgpO1xuICB9XG5cbiAgcGxheSgpe1xuICAgIGlmKHRoaXMuaXNQbGF5aW5nKXtcbiAgICAgIHJldHVybjtcbiAgICB9XG5cbiAgICB0aGlzLmlzUGxheWluZyA9IHRydWU7XG4gICAgdGhpcy50aW1lciA9IHNldEludGVydmFsKHRoaXMubW92ZS5iaW5kKHRoaXMpLCAxNTApO1xuICAgIHRoaXMuX2NoYW5nZSgpO1xuICB9XG5cbiAgX3Nob3VsZEZldGNoKHN0YXJ0LCBlbmQpe1xuICAgIGZvcih2YXIgaSA9IHN0YXJ0OyBpIDwgZW5kOyBpKyspe1xuICAgICAgaWYodGhpcy50dHlTdGVhbVtpXSA9PT0gdW5kZWZpbmVkKXtcbiAgICAgICAgcmV0dXJuIHRydWU7XG4gICAgICB9XG4gICAgfVxuXG4gICAgcmV0dXJuIGZhbHNlO1xuICB9XG5cbiAgX2ZldGNoKHN0YXJ0LCBlbmQpe1xuICAgIGVuZCA9IGVuZCArIDUwO1xuICAgIGVuZCA9IGVuZCA+IHRoaXMubGVuZ3RoID8gdGhpcy5sZW5ndGggOiBlbmQ7XG4gICAgcmV0dXJuIGFwaS5nZXQoY2ZnLmFwaS5nZXRGZXRjaFNlc3Npb25DaHVua1VybCh7c2lkOiB0aGlzLnNpZCwgc3RhcnQsIGVuZH0pKS5cbiAgICAgIGRvbmUoKHJlc3BvbnNlKT0+e1xuICAgICAgICBmb3IodmFyIGkgPSAwOyBpIDwgZW5kLXN0YXJ0OyBpKyspe1xuICAgICAgICAgIHZhciBkYXRhID0gYXRvYihyZXNwb25zZS5jaHVua3NbaV0uZGF0YSkgfHwgJyc7XG4gICAgICAgICAgdmFyIGRlbGF5ID0gcmVzcG9uc2UuY2h1bmtzW2ldLmRlbGF5O1xuICAgICAgICAgIHRoaXMudHR5U3RlYW1bc3RhcnQraV0gPSB7IGRhdGEsIGRlbGF5fTtcbiAgICAgICAgfVxuICAgICAgfSk7XG4gIH1cblxuICBfc2hvd0NodW5rKHN0YXJ0LCBlbmQpe1xuICAgIHZhciBkaXNwbGF5ID0gKCk9PntcbiAgICAgIGZvcih2YXIgaSA9IHN0YXJ0OyBpIDwgZW5kOyBpKyspe1xuICAgICAgICB0aGlzLmVtaXQoJ2RhdGEnLCB0aGlzLnR0eVN0ZWFtW2ldLmRhdGEpO1xuICAgICAgfVxuICAgICAgdGhpcy5jdXJyZW50ID0gZW5kO1xuICAgIH07XG5cbiAgICBpZih0aGlzLl9zaG91bGRGZXRjaChzdGFydCwgZW5kKSl7XG4gICAgICB0aGlzLl9mZXRjaChzdGFydCwgZW5kKS50aGVuKGRpc3BsYXkpO1xuICAgIH1lbHNle1xuICAgICAgZGlzcGxheSgpO1xuICAgIH1cbiAgfVxuXG4gIF9jaGFuZ2UoKXtcbiAgICB0aGlzLmVtaXQoJ2NoYW5nZScpO1xuICB9XG59XG5cbmV4cG9ydCBkZWZhdWx0IFR0eVBsYXllcjtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21tb24vdHR5UGxheWVyLmpzXG4gKiovIiwidmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHtmZXRjaFNlc3Npb25zfSA9IHJlcXVpcmUoJy4vLi4vc2Vzc2lvbnMvYWN0aW9ucycpO1xudmFyIHtmZXRjaE5vZGVzIH0gPSByZXF1aXJlKCcuLy4uL25vZGVzL2FjdGlvbnMnKTtcbnZhciAkID0gcmVxdWlyZSgnalF1ZXJ5Jyk7XG5cbnZhciB7IFRMUFRfQVBQX0lOSVQsIFRMUFRfQVBQX0ZBSUxFRCwgVExQVF9BUFBfUkVBRFkgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxudmFyIGFjdGlvbnMgPSB7XG5cbiAgaW5pdEFwcCgpIHtcbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfQVBQX0lOSVQpO1xuICAgIGFjdGlvbnMuZmV0Y2hOb2Rlc0FuZFNlc3Npb25zKClcbiAgICAgIC5kb25lKCgpPT57IHJlYWN0b3IuZGlzcGF0Y2goVExQVF9BUFBfUkVBRFkpOyB9KVxuICAgICAgLmZhaWwoKCk9PnsgcmVhY3Rvci5kaXNwYXRjaChUTFBUX0FQUF9GQUlMRUQpOyB9KTtcblxuICAgIC8vYXBpLmdldChgL3YxL3dlYmFwaS9zaXRlcy8tY3VycmVudC0vc2Vzc2lvbnMvMDNkM2UxMWQtNDVjMS00MDQ5LWJjZWItYjIzMzYwNTY2NmU0L2NodW5rcz9zdGFydD0wJmVuZD0xMDBgKS5kb25lKCgpID0+IHtcbiAgICAvL30pO1xuICB9LFxuXG4gIGZldGNoTm9kZXNBbmRTZXNzaW9ucygpIHtcbiAgICByZXR1cm4gJC53aGVuKGZldGNoTm9kZXMoKSwgZmV0Y2hTZXNzaW9ucygpKTtcbiAgfVxufVxuXG5leHBvcnQgZGVmYXVsdCBhY3Rpb25zO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvYXBwL2FjdGlvbnMuanNcbiAqKi8iLCJjb25zdCBhcHBTdGF0ZSA9IFtbJ3RscHQnXSwgYXBwPT4gYXBwLnRvSlMoKV07XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgYXBwU3RhdGVcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2FwcC9nZXR0ZXJzLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMuZ2V0dGVycyA9IHJlcXVpcmUoJy4vZ2V0dGVycycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aW9ucyA9IHJlcXVpcmUoJy4vYWN0aW9ucycpO1xubW9kdWxlLmV4cG9ydHMuYXBwU3RvcmUgPSByZXF1aXJlKCcuL2FwcFN0b3JlJyk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hcHAvaW5kZXguanNcbiAqKi8iLCJjb25zdCBkaWFsb2dzID0gW1sndGxwdF9kaWFsb2dzJ10sIHN0YXRlPT4gc3RhdGUudG9KUygpXTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICBkaWFsb2dzXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9kaWFsb2dzL2dldHRlcnMuanNcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cy5nZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3Rpb25zID0gcmVxdWlyZSgnLi9hY3Rpb25zJyk7XG5tb2R1bGUuZXhwb3J0cy5kaWFsb2dTdG9yZSA9IHJlcXVpcmUoJy4vZGlhbG9nU3RvcmUnKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvaW5kZXguanNcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG5yZWFjdG9yLnJlZ2lzdGVyU3RvcmVzKHtcbiAgJ3RscHQnOiByZXF1aXJlKCcuL2FwcC9hcHBTdG9yZScpLFxuICAndGxwdF9kaWFsb2dzJzogcmVxdWlyZSgnLi9kaWFsb2dzL2RpYWxvZ1N0b3JlJyksXG4gICd0bHB0X2FjdGl2ZV90ZXJtaW5hbCc6IHJlcXVpcmUoJy4vYWN0aXZlVGVybWluYWwvYWN0aXZlVGVybVN0b3JlJyksXG4gICd0bHB0X3VzZXInOiByZXF1aXJlKCcuL3VzZXIvdXNlclN0b3JlJyksXG4gICd0bHB0X25vZGVzJzogcmVxdWlyZSgnLi9ub2Rlcy9ub2RlU3RvcmUnKSxcbiAgJ3RscHRfaW52aXRlJzogcmVxdWlyZSgnLi9pbnZpdGUvaW52aXRlU3RvcmUnKSxcbiAgJ3RscHRfcmVzdF9hcGknOiByZXF1aXJlKCcuL3Jlc3RBcGkvcmVzdEFwaVN0b3JlJyksXG4gICd0bHB0X3Nlc3Npb25zJzogcmVxdWlyZSgnLi9zZXNzaW9ucy9zZXNzaW9uU3RvcmUnKVxufSk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9pbmRleC5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7IFRMUFRfUkVDRUlWRV9VU0VSX0lOVklURSB9ICA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcbnZhciBhcGkgPSByZXF1aXJlKCdhcHAvc2VydmljZXMvYXBpJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xuXG5leHBvcnQgZGVmYXVsdCB7XG4gIGZldGNoSW52aXRlKGludml0ZVRva2VuKXtcbiAgICB2YXIgcGF0aCA9IGNmZy5hcGkuZ2V0SW52aXRlVXJsKGludml0ZVRva2VuKTtcbiAgICBhcGkuZ2V0KHBhdGgpLmRvbmUoaW52aXRlPT57XG4gICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfUkVDRUlWRV9VU0VSX0lOVklURSwgaW52aXRlKTtcbiAgICB9KTtcbiAgfVxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2FjdGlvbnMuanNcbiAqKi8iLCIvKmVzbGludCBuby11bmRlZjogMCwgIG5vLXVudXNlZC12YXJzOiAwLCBuby1kZWJ1Z2dlcjowKi9cblxudmFyIHtUUllJTkdfVE9fU0lHTl9VUH0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9yZXN0QXBpL2NvbnN0YW50cycpO1xuXG5jb25zdCBpbnZpdGUgPSBbIFsndGxwdF9pbnZpdGUnXSwgKGludml0ZSkgPT4ge1xuICByZXR1cm4gaW52aXRlO1xuIH1cbl07XG5cbmNvbnN0IGF0dGVtcCA9IFsgWyd0bHB0X3Jlc3RfYXBpJywgVFJZSU5HX1RPX1NJR05fVVBdLCAoYXR0ZW1wKSA9PiB7XG4gIHZhciBkZWZhdWx0T2JqID0ge1xuICAgIGlzUHJvY2Vzc2luZzogZmFsc2UsXG4gICAgaXNFcnJvcjogZmFsc2UsXG4gICAgaXNTdWNjZXNzOiBmYWxzZSxcbiAgICBtZXNzYWdlOiAnJ1xuICB9XG5cbiAgcmV0dXJuIGF0dGVtcCA/IGF0dGVtcC50b0pTKCkgOiBkZWZhdWx0T2JqO1xuICBcbiB9XG5dO1xuXG5leHBvcnQgZGVmYXVsdCB7XG4gIGludml0ZSxcbiAgYXR0ZW1wXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvZ2V0dGVycy5qc1xuICoqLyIsIm1vZHVsZS5leHBvcnRzLmdldHRlcnMgPSByZXF1aXJlKCcuL2dldHRlcnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGlvbnMgPSByZXF1aXJlKCcuL2FjdGlvbnMnKTtcbm1vZHVsZS5leHBvcnRzLm5vZGVTdG9yZSA9IHJlcXVpcmUoJy4vaW52aXRlU3RvcmUnKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9pbmRleC5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7c2Vzc2lvbnNCeVNlcnZlcn0gPSByZXF1aXJlKCcuLy4uL3Nlc3Npb25zL2dldHRlcnMnKTtcblxuY29uc3Qgbm9kZUxpc3RWaWV3ID0gWyBbJ3RscHRfbm9kZXMnXSwgKG5vZGVzKSA9PntcbiAgICByZXR1cm4gbm9kZXMubWFwKChpdGVtKT0+e1xuICAgICAgdmFyIHNlcnZlcklkID0gaXRlbS5nZXQoJ2lkJyk7XG4gICAgICB2YXIgc2Vzc2lvbnMgPSByZWFjdG9yLmV2YWx1YXRlKHNlc3Npb25zQnlTZXJ2ZXIoc2VydmVySWQpKTtcbiAgICAgIHJldHVybiB7XG4gICAgICAgIGlkOiBzZXJ2ZXJJZCxcbiAgICAgICAgaG9zdG5hbWU6IGl0ZW0uZ2V0KCdob3N0bmFtZScpLFxuICAgICAgICB0YWdzOiBnZXRUYWdzKGl0ZW0pLFxuICAgICAgICBhZGRyOiBpdGVtLmdldCgnYWRkcicpLFxuICAgICAgICBzZXNzaW9uQ291bnQ6IHNlc3Npb25zLnNpemVcbiAgICAgIH1cbiAgICB9KS50b0pTKCk7XG4gfVxuXTtcblxuZnVuY3Rpb24gZ2V0VGFncyhub2RlKXtcbiAgdmFyIGFsbExhYmVscyA9IFtdO1xuICB2YXIgbGFiZWxzID0gbm9kZS5nZXQoJ2xhYmVscycpO1xuXG4gIGlmKGxhYmVscyl7XG4gICAgbGFiZWxzLmVudHJ5U2VxKCkudG9BcnJheSgpLmZvckVhY2goaXRlbT0+e1xuICAgICAgYWxsTGFiZWxzLnB1c2goe1xuICAgICAgICByb2xlOiBpdGVtWzBdLFxuICAgICAgICB2YWx1ZTogaXRlbVsxXVxuICAgICAgfSk7XG4gICAgfSk7XG4gIH1cblxuICBsYWJlbHMgPSBub2RlLmdldCgnY21kX2xhYmVscycpO1xuXG4gIGlmKGxhYmVscyl7XG4gICAgbGFiZWxzLmVudHJ5U2VxKCkudG9BcnJheSgpLmZvckVhY2goaXRlbT0+e1xuICAgICAgYWxsTGFiZWxzLnB1c2goe1xuICAgICAgICByb2xlOiBpdGVtWzBdLFxuICAgICAgICB2YWx1ZTogaXRlbVsxXS5nZXQoJ3Jlc3VsdCcpLFxuICAgICAgICB0b29sdGlwOiBpdGVtWzFdLmdldCgnY29tbWFuZCcpXG4gICAgICB9KTtcbiAgICB9KTtcbiAgfVxuXG4gIHJldHVybiBhbGxMYWJlbHM7XG59XG5cblxuZXhwb3J0IGRlZmF1bHQge1xuICBub2RlTGlzdFZpZXdcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2dldHRlcnMuanNcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cy5nZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3Rpb25zID0gcmVxdWlyZSgnLi9hY3Rpb25zJyk7XG5tb2R1bGUuZXhwb3J0cy5ub2RlU3RvcmUgPSByZXF1aXJlKCcuL25vZGVTdG9yZScpO1xuXG4vLyBub2RlczogW3tcImlkXCI6XCJ4MjIwXCIsXCJhZGRyXCI6XCIwLjAuMC4wOjMwMjJcIixcImhvc3RuYW1lXCI6XCJ4MjIwXCIsXCJsYWJlbHNcIjpudWxsLFwiY21kX2xhYmVsc1wiOm51bGx9XVxuXG5cbi8vIHNlc3Npb25zOiBbe1wiaWRcIjpcIjA3NjMwNjM2LWJiM2QtNDBlMS1iMDg2LTYwYjJjYWUyMWFjNFwiLFwicGFydGllc1wiOlt7XCJpZFwiOlwiODlmNzYyYTMtNzQyOS00YzdhLWE5MTMtNzY2NDkzZmU3YzhhXCIsXCJzaXRlXCI6XCIxMjcuMC4wLjE6Mzc1MTRcIixcInVzZXJcIjpcImFrb250c2V2b3lcIixcInNlcnZlcl9hZGRyXCI6XCIwLjAuMC4wOjMwMjJcIixcImxhc3RfYWN0aXZlXCI6XCIyMDE2LTAyLTIyVDE0OjM5OjIwLjkzMTIwNTM1LTA1OjAwXCJ9XX1dXG5cbi8qXG5sZXQgVG9kb1JlY29yZCA9IEltbXV0YWJsZS5SZWNvcmQoe1xuICAgIGlkOiAwLFxuICAgIGRlc2NyaXB0aW9uOiBcIlwiLFxuICAgIGNvbXBsZXRlZDogZmFsc2Vcbn0pO1xuKi9cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2luZGV4LmpzXG4gKiovIiwidmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xuXG52YXIge1xuICBUTFBUX1JFU1RfQVBJX1NUQVJULFxuICBUTFBUX1JFU1RfQVBJX1NVQ0NFU1MsXG4gIFRMUFRfUkVTVF9BUElfRkFJTCB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCB7XG5cbiAgc3RhcnQocmVxVHlwZSl7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFU1RfQVBJX1NUQVJULCB7dHlwZTogcmVxVHlwZX0pO1xuICB9LFxuXG4gIGZhaWwocmVxVHlwZSwgbWVzc2FnZSl7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFU1RfQVBJX0ZBSUwsICB7dHlwZTogcmVxVHlwZSwgbWVzc2FnZX0pO1xuICB9LFxuXG4gIHN1Y2Nlc3MocmVxVHlwZSl7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFU1RfQVBJX1NVQ0NFU1MsIHt0eXBlOiByZXFUeXBlfSk7XG4gIH1cblxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9hY3Rpb25zLmpzXG4gKiovIiwidmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG52YXIge1xuICBUTFBUX1JFU1RfQVBJX1NUQVJULFxuICBUTFBUX1JFU1RfQVBJX1NVQ0NFU1MsXG4gIFRMUFRfUkVTVF9BUElfRkFJTCB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gdG9JbW11dGFibGUoe30pO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX1JFU1RfQVBJX1NUQVJULCBzdGFydCk7XG4gICAgdGhpcy5vbihUTFBUX1JFU1RfQVBJX0ZBSUwsIGZhaWwpO1xuICAgIHRoaXMub24oVExQVF9SRVNUX0FQSV9TVUNDRVNTLCBzdWNjZXNzKTtcbiAgfVxufSlcblxuZnVuY3Rpb24gc3RhcnQoc3RhdGUsIHJlcXVlc3Qpe1xuICByZXR1cm4gc3RhdGUuc2V0KHJlcXVlc3QudHlwZSwgdG9JbW11dGFibGUoe2lzUHJvY2Vzc2luZzogdHJ1ZX0pKTtcbn1cblxuZnVuY3Rpb24gZmFpbChzdGF0ZSwgcmVxdWVzdCl7XG4gIHJldHVybiBzdGF0ZS5zZXQocmVxdWVzdC50eXBlLCB0b0ltbXV0YWJsZSh7aXNGYWlsZWQ6IHRydWUsIG1lc3NhZ2U6IHJlcXVlc3QubWVzc2FnZX0pKTtcbn1cblxuZnVuY3Rpb24gc3VjY2VzcyhzdGF0ZSwgcmVxdWVzdCl7XG4gIHJldHVybiBzdGF0ZS5zZXQocmVxdWVzdC50eXBlLCB0b0ltbXV0YWJsZSh7aXNTdWNjZXNzOiB0cnVlfSkpO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9yZXN0QXBpU3RvcmUuanNcbiAqKi8iLCJ2YXIgdXRpbHMgPSB7XG5cbiAgdXVpZCgpe1xuICAgIC8vIG5ldmVyIHVzZSBpdCBpbiBwcm9kdWN0aW9uXG4gICAgcmV0dXJuICd4eHh4eHh4eC14eHh4LTR4eHgteXh4eC14eHh4eHh4eHh4eHgnLnJlcGxhY2UoL1t4eV0vZywgZnVuY3Rpb24oYykge1xuICAgICAgdmFyIHIgPSBNYXRoLnJhbmRvbSgpKjE2fDAsIHYgPSBjID09ICd4JyA/IHIgOiAociYweDN8MHg4KTtcbiAgICAgIHJldHVybiB2LnRvU3RyaW5nKDE2KTtcbiAgICB9KTtcbiAgfSxcblxuICBkaXNwbGF5RGF0ZShkYXRlKXtcbiAgICB0cnl7XG4gICAgICByZXR1cm4gZGF0ZS50b0xvY2FsZURhdGVTdHJpbmcoKSArICcgJyArIGRhdGUudG9Mb2NhbGVUaW1lU3RyaW5nKCk7XG4gICAgfWNhdGNoKGVycil7XG4gICAgICBjb25zb2xlLmVycm9yKGVycik7XG4gICAgICByZXR1cm4gJ3VuZGVmaW5lZCc7XG4gICAgfVxuICB9LFxuXG4gIGZvcm1hdFN0cmluZyhmb3JtYXQpIHtcbiAgICB2YXIgYXJncyA9IEFycmF5LnByb3RvdHlwZS5zbGljZS5jYWxsKGFyZ3VtZW50cywgMSk7XG4gICAgcmV0dXJuIGZvcm1hdC5yZXBsYWNlKG5ldyBSZWdFeHAoJ1xcXFx7KFxcXFxkKylcXFxcfScsICdnJyksXG4gICAgICAobWF0Y2gsIG51bWJlcikgPT4ge1xuICAgICAgICByZXR1cm4gIShhcmdzW251bWJlcl0gPT09IG51bGwgfHwgYXJnc1tudW1iZXJdID09PSB1bmRlZmluZWQpID8gYXJnc1tudW1iZXJdIDogJyc7XG4gICAgfSk7XG4gIH1cbiAgICAgICAgICAgIFxufVxuXG5tb2R1bGUuZXhwb3J0cyA9IHV0aWxzO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL3V0aWxzLmpzXG4gKiovIiwiLy8gQ29weXJpZ2h0IEpveWVudCwgSW5jLiBhbmQgb3RoZXIgTm9kZSBjb250cmlidXRvcnMuXG4vL1xuLy8gUGVybWlzc2lvbiBpcyBoZXJlYnkgZ3JhbnRlZCwgZnJlZSBvZiBjaGFyZ2UsIHRvIGFueSBwZXJzb24gb2J0YWluaW5nIGFcbi8vIGNvcHkgb2YgdGhpcyBzb2Z0d2FyZSBhbmQgYXNzb2NpYXRlZCBkb2N1bWVudGF0aW9uIGZpbGVzICh0aGVcbi8vIFwiU29mdHdhcmVcIiksIHRvIGRlYWwgaW4gdGhlIFNvZnR3YXJlIHdpdGhvdXQgcmVzdHJpY3Rpb24sIGluY2x1ZGluZ1xuLy8gd2l0aG91dCBsaW1pdGF0aW9uIHRoZSByaWdodHMgdG8gdXNlLCBjb3B5LCBtb2RpZnksIG1lcmdlLCBwdWJsaXNoLFxuLy8gZGlzdHJpYnV0ZSwgc3VibGljZW5zZSwgYW5kL29yIHNlbGwgY29waWVzIG9mIHRoZSBTb2Z0d2FyZSwgYW5kIHRvIHBlcm1pdFxuLy8gcGVyc29ucyB0byB3aG9tIHRoZSBTb2Z0d2FyZSBpcyBmdXJuaXNoZWQgdG8gZG8gc28sIHN1YmplY3QgdG8gdGhlXG4vLyBmb2xsb3dpbmcgY29uZGl0aW9uczpcbi8vXG4vLyBUaGUgYWJvdmUgY29weXJpZ2h0IG5vdGljZSBhbmQgdGhpcyBwZXJtaXNzaW9uIG5vdGljZSBzaGFsbCBiZSBpbmNsdWRlZFxuLy8gaW4gYWxsIGNvcGllcyBvciBzdWJzdGFudGlhbCBwb3J0aW9ucyBvZiB0aGUgU29mdHdhcmUuXG4vL1xuLy8gVEhFIFNPRlRXQVJFIElTIFBST1ZJREVEIFwiQVMgSVNcIiwgV0lUSE9VVCBXQVJSQU5UWSBPRiBBTlkgS0lORCwgRVhQUkVTU1xuLy8gT1IgSU1QTElFRCwgSU5DTFVESU5HIEJVVCBOT1QgTElNSVRFRCBUTyBUSEUgV0FSUkFOVElFUyBPRlxuLy8gTUVSQ0hBTlRBQklMSVRZLCBGSVRORVNTIEZPUiBBIFBBUlRJQ1VMQVIgUFVSUE9TRSBBTkQgTk9OSU5GUklOR0VNRU5ULiBJTlxuLy8gTk8gRVZFTlQgU0hBTEwgVEhFIEFVVEhPUlMgT1IgQ09QWVJJR0hUIEhPTERFUlMgQkUgTElBQkxFIEZPUiBBTlkgQ0xBSU0sXG4vLyBEQU1BR0VTIE9SIE9USEVSIExJQUJJTElUWSwgV0hFVEhFUiBJTiBBTiBBQ1RJT04gT0YgQ09OVFJBQ1QsIFRPUlQgT1Jcbi8vIE9USEVSV0lTRSwgQVJJU0lORyBGUk9NLCBPVVQgT0YgT1IgSU4gQ09OTkVDVElPTiBXSVRIIFRIRSBTT0ZUV0FSRSBPUiBUSEVcbi8vIFVTRSBPUiBPVEhFUiBERUFMSU5HUyBJTiBUSEUgU09GVFdBUkUuXG5cbmZ1bmN0aW9uIEV2ZW50RW1pdHRlcigpIHtcbiAgdGhpcy5fZXZlbnRzID0gdGhpcy5fZXZlbnRzIHx8IHt9O1xuICB0aGlzLl9tYXhMaXN0ZW5lcnMgPSB0aGlzLl9tYXhMaXN0ZW5lcnMgfHwgdW5kZWZpbmVkO1xufVxubW9kdWxlLmV4cG9ydHMgPSBFdmVudEVtaXR0ZXI7XG5cbi8vIEJhY2t3YXJkcy1jb21wYXQgd2l0aCBub2RlIDAuMTAueFxuRXZlbnRFbWl0dGVyLkV2ZW50RW1pdHRlciA9IEV2ZW50RW1pdHRlcjtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5fZXZlbnRzID0gdW5kZWZpbmVkO1xuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5fbWF4TGlzdGVuZXJzID0gdW5kZWZpbmVkO1xuXG4vLyBCeSBkZWZhdWx0IEV2ZW50RW1pdHRlcnMgd2lsbCBwcmludCBhIHdhcm5pbmcgaWYgbW9yZSB0aGFuIDEwIGxpc3RlbmVycyBhcmVcbi8vIGFkZGVkIHRvIGl0LiBUaGlzIGlzIGEgdXNlZnVsIGRlZmF1bHQgd2hpY2ggaGVscHMgZmluZGluZyBtZW1vcnkgbGVha3MuXG5FdmVudEVtaXR0ZXIuZGVmYXVsdE1heExpc3RlbmVycyA9IDEwO1xuXG4vLyBPYnZpb3VzbHkgbm90IGFsbCBFbWl0dGVycyBzaG91bGQgYmUgbGltaXRlZCB0byAxMC4gVGhpcyBmdW5jdGlvbiBhbGxvd3Ncbi8vIHRoYXQgdG8gYmUgaW5jcmVhc2VkLiBTZXQgdG8gemVybyBmb3IgdW5saW1pdGVkLlxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5zZXRNYXhMaXN0ZW5lcnMgPSBmdW5jdGlvbihuKSB7XG4gIGlmICghaXNOdW1iZXIobikgfHwgbiA8IDAgfHwgaXNOYU4obikpXG4gICAgdGhyb3cgVHlwZUVycm9yKCduIG11c3QgYmUgYSBwb3NpdGl2ZSBudW1iZXInKTtcbiAgdGhpcy5fbWF4TGlzdGVuZXJzID0gbjtcbiAgcmV0dXJuIHRoaXM7XG59O1xuXG5FdmVudEVtaXR0ZXIucHJvdG90eXBlLmVtaXQgPSBmdW5jdGlvbih0eXBlKSB7XG4gIHZhciBlciwgaGFuZGxlciwgbGVuLCBhcmdzLCBpLCBsaXN0ZW5lcnM7XG5cbiAgaWYgKCF0aGlzLl9ldmVudHMpXG4gICAgdGhpcy5fZXZlbnRzID0ge307XG5cbiAgLy8gSWYgdGhlcmUgaXMgbm8gJ2Vycm9yJyBldmVudCBsaXN0ZW5lciB0aGVuIHRocm93LlxuICBpZiAodHlwZSA9PT0gJ2Vycm9yJykge1xuICAgIGlmICghdGhpcy5fZXZlbnRzLmVycm9yIHx8XG4gICAgICAgIChpc09iamVjdCh0aGlzLl9ldmVudHMuZXJyb3IpICYmICF0aGlzLl9ldmVudHMuZXJyb3IubGVuZ3RoKSkge1xuICAgICAgZXIgPSBhcmd1bWVudHNbMV07XG4gICAgICBpZiAoZXIgaW5zdGFuY2VvZiBFcnJvcikge1xuICAgICAgICB0aHJvdyBlcjsgLy8gVW5oYW5kbGVkICdlcnJvcicgZXZlbnRcbiAgICAgIH1cbiAgICAgIHRocm93IFR5cGVFcnJvcignVW5jYXVnaHQsIHVuc3BlY2lmaWVkIFwiZXJyb3JcIiBldmVudC4nKTtcbiAgICB9XG4gIH1cblxuICBoYW5kbGVyID0gdGhpcy5fZXZlbnRzW3R5cGVdO1xuXG4gIGlmIChpc1VuZGVmaW5lZChoYW5kbGVyKSlcbiAgICByZXR1cm4gZmFsc2U7XG5cbiAgaWYgKGlzRnVuY3Rpb24oaGFuZGxlcikpIHtcbiAgICBzd2l0Y2ggKGFyZ3VtZW50cy5sZW5ndGgpIHtcbiAgICAgIC8vIGZhc3QgY2FzZXNcbiAgICAgIGNhc2UgMTpcbiAgICAgICAgaGFuZGxlci5jYWxsKHRoaXMpO1xuICAgICAgICBicmVhaztcbiAgICAgIGNhc2UgMjpcbiAgICAgICAgaGFuZGxlci5jYWxsKHRoaXMsIGFyZ3VtZW50c1sxXSk7XG4gICAgICAgIGJyZWFrO1xuICAgICAgY2FzZSAzOlxuICAgICAgICBoYW5kbGVyLmNhbGwodGhpcywgYXJndW1lbnRzWzFdLCBhcmd1bWVudHNbMl0pO1xuICAgICAgICBicmVhaztcbiAgICAgIC8vIHNsb3dlclxuICAgICAgZGVmYXVsdDpcbiAgICAgICAgbGVuID0gYXJndW1lbnRzLmxlbmd0aDtcbiAgICAgICAgYXJncyA9IG5ldyBBcnJheShsZW4gLSAxKTtcbiAgICAgICAgZm9yIChpID0gMTsgaSA8IGxlbjsgaSsrKVxuICAgICAgICAgIGFyZ3NbaSAtIDFdID0gYXJndW1lbnRzW2ldO1xuICAgICAgICBoYW5kbGVyLmFwcGx5KHRoaXMsIGFyZ3MpO1xuICAgIH1cbiAgfSBlbHNlIGlmIChpc09iamVjdChoYW5kbGVyKSkge1xuICAgIGxlbiA9IGFyZ3VtZW50cy5sZW5ndGg7XG4gICAgYXJncyA9IG5ldyBBcnJheShsZW4gLSAxKTtcbiAgICBmb3IgKGkgPSAxOyBpIDwgbGVuOyBpKyspXG4gICAgICBhcmdzW2kgLSAxXSA9IGFyZ3VtZW50c1tpXTtcblxuICAgIGxpc3RlbmVycyA9IGhhbmRsZXIuc2xpY2UoKTtcbiAgICBsZW4gPSBsaXN0ZW5lcnMubGVuZ3RoO1xuICAgIGZvciAoaSA9IDA7IGkgPCBsZW47IGkrKylcbiAgICAgIGxpc3RlbmVyc1tpXS5hcHBseSh0aGlzLCBhcmdzKTtcbiAgfVxuXG4gIHJldHVybiB0cnVlO1xufTtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5hZGRMaXN0ZW5lciA9IGZ1bmN0aW9uKHR5cGUsIGxpc3RlbmVyKSB7XG4gIHZhciBtO1xuXG4gIGlmICghaXNGdW5jdGlvbihsaXN0ZW5lcikpXG4gICAgdGhyb3cgVHlwZUVycm9yKCdsaXN0ZW5lciBtdXN0IGJlIGEgZnVuY3Rpb24nKTtcblxuICBpZiAoIXRoaXMuX2V2ZW50cylcbiAgICB0aGlzLl9ldmVudHMgPSB7fTtcblxuICAvLyBUbyBhdm9pZCByZWN1cnNpb24gaW4gdGhlIGNhc2UgdGhhdCB0eXBlID09PSBcIm5ld0xpc3RlbmVyXCIhIEJlZm9yZVxuICAvLyBhZGRpbmcgaXQgdG8gdGhlIGxpc3RlbmVycywgZmlyc3QgZW1pdCBcIm5ld0xpc3RlbmVyXCIuXG4gIGlmICh0aGlzLl9ldmVudHMubmV3TGlzdGVuZXIpXG4gICAgdGhpcy5lbWl0KCduZXdMaXN0ZW5lcicsIHR5cGUsXG4gICAgICAgICAgICAgIGlzRnVuY3Rpb24obGlzdGVuZXIubGlzdGVuZXIpID9cbiAgICAgICAgICAgICAgbGlzdGVuZXIubGlzdGVuZXIgOiBsaXN0ZW5lcik7XG5cbiAgaWYgKCF0aGlzLl9ldmVudHNbdHlwZV0pXG4gICAgLy8gT3B0aW1pemUgdGhlIGNhc2Ugb2Ygb25lIGxpc3RlbmVyLiBEb24ndCBuZWVkIHRoZSBleHRyYSBhcnJheSBvYmplY3QuXG4gICAgdGhpcy5fZXZlbnRzW3R5cGVdID0gbGlzdGVuZXI7XG4gIGVsc2UgaWYgKGlzT2JqZWN0KHRoaXMuX2V2ZW50c1t0eXBlXSkpXG4gICAgLy8gSWYgd2UndmUgYWxyZWFkeSBnb3QgYW4gYXJyYXksIGp1c3QgYXBwZW5kLlxuICAgIHRoaXMuX2V2ZW50c1t0eXBlXS5wdXNoKGxpc3RlbmVyKTtcbiAgZWxzZVxuICAgIC8vIEFkZGluZyB0aGUgc2Vjb25kIGVsZW1lbnQsIG5lZWQgdG8gY2hhbmdlIHRvIGFycmF5LlxuICAgIHRoaXMuX2V2ZW50c1t0eXBlXSA9IFt0aGlzLl9ldmVudHNbdHlwZV0sIGxpc3RlbmVyXTtcblxuICAvLyBDaGVjayBmb3IgbGlzdGVuZXIgbGVha1xuICBpZiAoaXNPYmplY3QodGhpcy5fZXZlbnRzW3R5cGVdKSAmJiAhdGhpcy5fZXZlbnRzW3R5cGVdLndhcm5lZCkge1xuICAgIHZhciBtO1xuICAgIGlmICghaXNVbmRlZmluZWQodGhpcy5fbWF4TGlzdGVuZXJzKSkge1xuICAgICAgbSA9IHRoaXMuX21heExpc3RlbmVycztcbiAgICB9IGVsc2Uge1xuICAgICAgbSA9IEV2ZW50RW1pdHRlci5kZWZhdWx0TWF4TGlzdGVuZXJzO1xuICAgIH1cblxuICAgIGlmIChtICYmIG0gPiAwICYmIHRoaXMuX2V2ZW50c1t0eXBlXS5sZW5ndGggPiBtKSB7XG4gICAgICB0aGlzLl9ldmVudHNbdHlwZV0ud2FybmVkID0gdHJ1ZTtcbiAgICAgIGNvbnNvbGUuZXJyb3IoJyhub2RlKSB3YXJuaW5nOiBwb3NzaWJsZSBFdmVudEVtaXR0ZXIgbWVtb3J5ICcgK1xuICAgICAgICAgICAgICAgICAgICAnbGVhayBkZXRlY3RlZC4gJWQgbGlzdGVuZXJzIGFkZGVkLiAnICtcbiAgICAgICAgICAgICAgICAgICAgJ1VzZSBlbWl0dGVyLnNldE1heExpc3RlbmVycygpIHRvIGluY3JlYXNlIGxpbWl0LicsXG4gICAgICAgICAgICAgICAgICAgIHRoaXMuX2V2ZW50c1t0eXBlXS5sZW5ndGgpO1xuICAgICAgaWYgKHR5cGVvZiBjb25zb2xlLnRyYWNlID09PSAnZnVuY3Rpb24nKSB7XG4gICAgICAgIC8vIG5vdCBzdXBwb3J0ZWQgaW4gSUUgMTBcbiAgICAgICAgY29uc29sZS50cmFjZSgpO1xuICAgICAgfVxuICAgIH1cbiAgfVxuXG4gIHJldHVybiB0aGlzO1xufTtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5vbiA9IEV2ZW50RW1pdHRlci5wcm90b3R5cGUuYWRkTGlzdGVuZXI7XG5cbkV2ZW50RW1pdHRlci5wcm90b3R5cGUub25jZSA9IGZ1bmN0aW9uKHR5cGUsIGxpc3RlbmVyKSB7XG4gIGlmICghaXNGdW5jdGlvbihsaXN0ZW5lcikpXG4gICAgdGhyb3cgVHlwZUVycm9yKCdsaXN0ZW5lciBtdXN0IGJlIGEgZnVuY3Rpb24nKTtcblxuICB2YXIgZmlyZWQgPSBmYWxzZTtcblxuICBmdW5jdGlvbiBnKCkge1xuICAgIHRoaXMucmVtb3ZlTGlzdGVuZXIodHlwZSwgZyk7XG5cbiAgICBpZiAoIWZpcmVkKSB7XG4gICAgICBmaXJlZCA9IHRydWU7XG4gICAgICBsaXN0ZW5lci5hcHBseSh0aGlzLCBhcmd1bWVudHMpO1xuICAgIH1cbiAgfVxuXG4gIGcubGlzdGVuZXIgPSBsaXN0ZW5lcjtcbiAgdGhpcy5vbih0eXBlLCBnKTtcblxuICByZXR1cm4gdGhpcztcbn07XG5cbi8vIGVtaXRzIGEgJ3JlbW92ZUxpc3RlbmVyJyBldmVudCBpZmYgdGhlIGxpc3RlbmVyIHdhcyByZW1vdmVkXG5FdmVudEVtaXR0ZXIucHJvdG90eXBlLnJlbW92ZUxpc3RlbmVyID0gZnVuY3Rpb24odHlwZSwgbGlzdGVuZXIpIHtcbiAgdmFyIGxpc3QsIHBvc2l0aW9uLCBsZW5ndGgsIGk7XG5cbiAgaWYgKCFpc0Z1bmN0aW9uKGxpc3RlbmVyKSlcbiAgICB0aHJvdyBUeXBlRXJyb3IoJ2xpc3RlbmVyIG11c3QgYmUgYSBmdW5jdGlvbicpO1xuXG4gIGlmICghdGhpcy5fZXZlbnRzIHx8ICF0aGlzLl9ldmVudHNbdHlwZV0pXG4gICAgcmV0dXJuIHRoaXM7XG5cbiAgbGlzdCA9IHRoaXMuX2V2ZW50c1t0eXBlXTtcbiAgbGVuZ3RoID0gbGlzdC5sZW5ndGg7XG4gIHBvc2l0aW9uID0gLTE7XG5cbiAgaWYgKGxpc3QgPT09IGxpc3RlbmVyIHx8XG4gICAgICAoaXNGdW5jdGlvbihsaXN0Lmxpc3RlbmVyKSAmJiBsaXN0Lmxpc3RlbmVyID09PSBsaXN0ZW5lcikpIHtcbiAgICBkZWxldGUgdGhpcy5fZXZlbnRzW3R5cGVdO1xuICAgIGlmICh0aGlzLl9ldmVudHMucmVtb3ZlTGlzdGVuZXIpXG4gICAgICB0aGlzLmVtaXQoJ3JlbW92ZUxpc3RlbmVyJywgdHlwZSwgbGlzdGVuZXIpO1xuXG4gIH0gZWxzZSBpZiAoaXNPYmplY3QobGlzdCkpIHtcbiAgICBmb3IgKGkgPSBsZW5ndGg7IGktLSA+IDA7KSB7XG4gICAgICBpZiAobGlzdFtpXSA9PT0gbGlzdGVuZXIgfHxcbiAgICAgICAgICAobGlzdFtpXS5saXN0ZW5lciAmJiBsaXN0W2ldLmxpc3RlbmVyID09PSBsaXN0ZW5lcikpIHtcbiAgICAgICAgcG9zaXRpb24gPSBpO1xuICAgICAgICBicmVhaztcbiAgICAgIH1cbiAgICB9XG5cbiAgICBpZiAocG9zaXRpb24gPCAwKVxuICAgICAgcmV0dXJuIHRoaXM7XG5cbiAgICBpZiAobGlzdC5sZW5ndGggPT09IDEpIHtcbiAgICAgIGxpc3QubGVuZ3RoID0gMDtcbiAgICAgIGRlbGV0ZSB0aGlzLl9ldmVudHNbdHlwZV07XG4gICAgfSBlbHNlIHtcbiAgICAgIGxpc3Quc3BsaWNlKHBvc2l0aW9uLCAxKTtcbiAgICB9XG5cbiAgICBpZiAodGhpcy5fZXZlbnRzLnJlbW92ZUxpc3RlbmVyKVxuICAgICAgdGhpcy5lbWl0KCdyZW1vdmVMaXN0ZW5lcicsIHR5cGUsIGxpc3RlbmVyKTtcbiAgfVxuXG4gIHJldHVybiB0aGlzO1xufTtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5yZW1vdmVBbGxMaXN0ZW5lcnMgPSBmdW5jdGlvbih0eXBlKSB7XG4gIHZhciBrZXksIGxpc3RlbmVycztcblxuICBpZiAoIXRoaXMuX2V2ZW50cylcbiAgICByZXR1cm4gdGhpcztcblxuICAvLyBub3QgbGlzdGVuaW5nIGZvciByZW1vdmVMaXN0ZW5lciwgbm8gbmVlZCB0byBlbWl0XG4gIGlmICghdGhpcy5fZXZlbnRzLnJlbW92ZUxpc3RlbmVyKSB7XG4gICAgaWYgKGFyZ3VtZW50cy5sZW5ndGggPT09IDApXG4gICAgICB0aGlzLl9ldmVudHMgPSB7fTtcbiAgICBlbHNlIGlmICh0aGlzLl9ldmVudHNbdHlwZV0pXG4gICAgICBkZWxldGUgdGhpcy5fZXZlbnRzW3R5cGVdO1xuICAgIHJldHVybiB0aGlzO1xuICB9XG5cbiAgLy8gZW1pdCByZW1vdmVMaXN0ZW5lciBmb3IgYWxsIGxpc3RlbmVycyBvbiBhbGwgZXZlbnRzXG4gIGlmIChhcmd1bWVudHMubGVuZ3RoID09PSAwKSB7XG4gICAgZm9yIChrZXkgaW4gdGhpcy5fZXZlbnRzKSB7XG4gICAgICBpZiAoa2V5ID09PSAncmVtb3ZlTGlzdGVuZXInKSBjb250aW51ZTtcbiAgICAgIHRoaXMucmVtb3ZlQWxsTGlzdGVuZXJzKGtleSk7XG4gICAgfVxuICAgIHRoaXMucmVtb3ZlQWxsTGlzdGVuZXJzKCdyZW1vdmVMaXN0ZW5lcicpO1xuICAgIHRoaXMuX2V2ZW50cyA9IHt9O1xuICAgIHJldHVybiB0aGlzO1xuICB9XG5cbiAgbGlzdGVuZXJzID0gdGhpcy5fZXZlbnRzW3R5cGVdO1xuXG4gIGlmIChpc0Z1bmN0aW9uKGxpc3RlbmVycykpIHtcbiAgICB0aGlzLnJlbW92ZUxpc3RlbmVyKHR5cGUsIGxpc3RlbmVycyk7XG4gIH0gZWxzZSB7XG4gICAgLy8gTElGTyBvcmRlclxuICAgIHdoaWxlIChsaXN0ZW5lcnMubGVuZ3RoKVxuICAgICAgdGhpcy5yZW1vdmVMaXN0ZW5lcih0eXBlLCBsaXN0ZW5lcnNbbGlzdGVuZXJzLmxlbmd0aCAtIDFdKTtcbiAgfVxuICBkZWxldGUgdGhpcy5fZXZlbnRzW3R5cGVdO1xuXG4gIHJldHVybiB0aGlzO1xufTtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5saXN0ZW5lcnMgPSBmdW5jdGlvbih0eXBlKSB7XG4gIHZhciByZXQ7XG4gIGlmICghdGhpcy5fZXZlbnRzIHx8ICF0aGlzLl9ldmVudHNbdHlwZV0pXG4gICAgcmV0ID0gW107XG4gIGVsc2UgaWYgKGlzRnVuY3Rpb24odGhpcy5fZXZlbnRzW3R5cGVdKSlcbiAgICByZXQgPSBbdGhpcy5fZXZlbnRzW3R5cGVdXTtcbiAgZWxzZVxuICAgIHJldCA9IHRoaXMuX2V2ZW50c1t0eXBlXS5zbGljZSgpO1xuICByZXR1cm4gcmV0O1xufTtcblxuRXZlbnRFbWl0dGVyLmxpc3RlbmVyQ291bnQgPSBmdW5jdGlvbihlbWl0dGVyLCB0eXBlKSB7XG4gIHZhciByZXQ7XG4gIGlmICghZW1pdHRlci5fZXZlbnRzIHx8ICFlbWl0dGVyLl9ldmVudHNbdHlwZV0pXG4gICAgcmV0ID0gMDtcbiAgZWxzZSBpZiAoaXNGdW5jdGlvbihlbWl0dGVyLl9ldmVudHNbdHlwZV0pKVxuICAgIHJldCA9IDE7XG4gIGVsc2VcbiAgICByZXQgPSBlbWl0dGVyLl9ldmVudHNbdHlwZV0ubGVuZ3RoO1xuICByZXR1cm4gcmV0O1xufTtcblxuZnVuY3Rpb24gaXNGdW5jdGlvbihhcmcpIHtcbiAgcmV0dXJuIHR5cGVvZiBhcmcgPT09ICdmdW5jdGlvbic7XG59XG5cbmZ1bmN0aW9uIGlzTnVtYmVyKGFyZykge1xuICByZXR1cm4gdHlwZW9mIGFyZyA9PT0gJ251bWJlcic7XG59XG5cbmZ1bmN0aW9uIGlzT2JqZWN0KGFyZykge1xuICByZXR1cm4gdHlwZW9mIGFyZyA9PT0gJ29iamVjdCcgJiYgYXJnICE9PSBudWxsO1xufVxuXG5mdW5jdGlvbiBpc1VuZGVmaW5lZChhcmcpIHtcbiAgcmV0dXJuIGFyZyA9PT0gdm9pZCAwO1xufVxuXG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiAuL34vZXZlbnRzL2V2ZW50cy5qc1xuICoqIG1vZHVsZSBpZCA9IDI4MVxuICoqIG1vZHVsZSBjaHVua3MgPSAxXG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciBOYXZMZWZ0QmFyID0gcmVxdWlyZSgnLi9uYXZMZWZ0QmFyJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHthY3Rpb25zLCBnZXR0ZXJzfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2FwcCcpO1xudmFyIFNlbGVjdE5vZGVEaWFsb2cgPSByZXF1aXJlKCcuL3NlbGVjdE5vZGVEaWFsb2cuanN4Jyk7XG5cbnZhciBBcHAgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIGFwcDogZ2V0dGVycy5hcHBTdGF0ZVxuICAgIH1cbiAgfSxcblxuICBjb21wb25lbnRXaWxsTW91bnQoKXtcbiAgICBhY3Rpb25zLmluaXRBcHAoKTtcbiAgICB0aGlzLnJlZnJlc2hJbnRlcnZhbCA9IHNldEludGVydmFsKGFjdGlvbnMuZmV0Y2hOb2Rlc0FuZFNlc3Npb25zLCAzNTAwMCk7XG4gIH0sXG5cbiAgY29tcG9uZW50V2lsbFVubW91bnQ6IGZ1bmN0aW9uKCkge1xuICAgIGNsZWFySW50ZXJ2YWwodGhpcy5yZWZyZXNoSW50ZXJ2YWwpO1xuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgaWYodGhpcy5zdGF0ZS5hcHAuaXNJbml0aWFsaXppbmcpe1xuICAgICAgcmV0dXJuIG51bGw7XG4gICAgfVxuXG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXRscHRcIj5cbiAgICAgICAgPE5hdkxlZnRCYXIvPlxuICAgICAgICA8U2VsZWN0Tm9kZURpYWxvZy8+XG4gICAgICAgIHt0aGlzLnByb3BzLkN1cnJlbnRTZXNzaW9uSG9zdH1cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJyb3dcIj5cbiAgICAgICAgICA8bmF2IGNsYXNzTmFtZT1cIlwiIHJvbGU9XCJuYXZpZ2F0aW9uXCIgc3R5bGU9e3sgbWFyZ2luQm90dG9tOiAwLCBmbG9hdDogXCJyaWdodFwiIH19PlxuICAgICAgICAgICAgPHVsIGNsYXNzTmFtZT1cIm5hdiBuYXZiYXItdG9wLWxpbmtzIG5hdmJhci1yaWdodFwiPlxuICAgICAgICAgICAgICA8bGk+XG4gICAgICAgICAgICAgICAgPGEgaHJlZj17Y2ZnLnJvdXRlcy5sb2dvdXR9PlxuICAgICAgICAgICAgICAgICAgPGkgY2xhc3NOYW1lPVwiZmEgZmEtc2lnbi1vdXRcIj48L2k+XG4gICAgICAgICAgICAgICAgICBMb2cgb3V0XG4gICAgICAgICAgICAgICAgPC9hPlxuICAgICAgICAgICAgICA8L2xpPlxuICAgICAgICAgICAgPC91bD5cbiAgICAgICAgICA8L25hdj5cbiAgICAgICAgPC9kaXY+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXBhZ2VcIj5cbiAgICAgICAgICB7dGhpcy5wcm9wcy5jaGlsZHJlbn1cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KVxuXG5tb2R1bGUuZXhwb3J0cyA9IEFwcDtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2FwcC5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHtnZXR0ZXJzLCBhY3Rpb25zfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsLycpO1xudmFyIFR0eSA9IHJlcXVpcmUoJ2FwcC9jb21tb24vdHR5Jyk7XG52YXIgVHR5VGVybWluYWwgPSByZXF1aXJlKCcuLy4uL3Rlcm1pbmFsLmpzeCcpO1xudmFyIEV2ZW50U3RyZWFtZXIgPSByZXF1aXJlKCcuL2V2ZW50U3RyZWFtZXIuanN4Jyk7XG52YXIgU2Vzc2lvbkxlZnRQYW5lbCA9IHJlcXVpcmUoJy4vc2Vzc2lvbkxlZnRQYW5lbCcpO1xudmFyIHtzaG93U2VsZWN0Tm9kZURpYWxvZywgY2xvc2VTZWxlY3ROb2RlRGlhbG9nfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2RpYWxvZ3MvYWN0aW9ucycpO1xuXG52YXIgQWN0aXZlU2Vzc2lvbiA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBjb21wb25lbnRXaWxsVW5tb3VudCgpe1xuICAgIGNsb3NlU2VsZWN0Tm9kZURpYWxvZygpO1xuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgbGV0IHtzZXJ2ZXJJcCwgbG9naW4sIHBhcnRpZXN9ID0gdGhpcy5wcm9wcy5hY3RpdmVTZXNzaW9uO1xuICAgIGxldCBzZXJ2ZXJMYWJlbFRleHQgPSBgJHtsb2dpbn1AJHtzZXJ2ZXJJcH1gO1xuXG4gICAgaWYoIXNlcnZlcklwKXtcbiAgICAgIHNlcnZlckxhYmVsVGV4dCA9ICcnO1xuICAgIH1cblxuICAgIHJldHVybiAoXG4gICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWN1cnJlbnQtc2Vzc2lvblwiPlxuICAgICAgIDxTZXNzaW9uTGVmdFBhbmVsIHBhcnRpZXM9e3BhcnRpZXN9Lz5cbiAgICAgICA8ZGl2PlxuICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtY3VycmVudC1zZXNzaW9uLXNlcnZlci1pbmZvXCI+XG4gICAgICAgICAgIDxzcGFuIGNsYXNzTmFtZT1cImJ0biBidG4tcHJpbWFyeSBidG4tc21cIiBvbkNsaWNrPXtzaG93U2VsZWN0Tm9kZURpYWxvZ30+XG4gICAgICAgICAgICAgQ2hhbmdlIG5vZGVcbiAgICAgICAgICAgPC9zcGFuPlxuICAgICAgICAgICA8aDM+e3NlcnZlckxhYmVsVGV4dH08L2gzPlxuICAgICAgICAgPC9kaXY+XG4gICAgICAgPC9kaXY+XG4gICAgICAgPFR0eUNvbm5lY3Rpb24gey4uLnRoaXMucHJvcHMuYWN0aXZlU2Vzc2lvbn0gLz5cbiAgICAgPC9kaXY+XG4gICAgICk7XG4gIH1cbn0pO1xuXG52YXIgVHR5Q29ubmVjdGlvbiA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgdGhpcy50dHkgPSBuZXcgVHR5KHRoaXMucHJvcHMpXG4gICAgdGhpcy50dHkub24oJ29wZW4nLCAoKT0+IHRoaXMuc2V0U3RhdGUoeyBpc0Nvbm5lY3RlZDogdHJ1ZSB9KSk7XG4gICAgcmV0dXJuIHtpc0Nvbm5lY3RlZDogZmFsc2V9O1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50KCkge1xuICAgIHRoaXMudHR5LmRpc2Nvbm5lY3QoKTtcbiAgfSxcblxuICBjb21wb25lbnRXaWxsUmVjZWl2ZVByb3BzKG5leHRQcm9wcyl7XG4gICAgaWYobmV4dFByb3BzLnNlcnZlcklkICE9PSB0aGlzLnByb3BzLnNlcnZlcklkIHx8XG4gICAgICBuZXh0UHJvcHMubG9naW4gIT09IHRoaXMucHJvcHMubG9naW4pe1xuICAgICAgICB0aGlzLnR0eS5yZWNvbm5lY3QobmV4dFByb3BzKTtcbiAgICAgICAgdGhpcy5yZWZzLnR0eUNtbnRJbnN0YW5jZS50ZXJtLmZvY3VzKCk7XG4gICAgICB9XG4gIH0sXG5cbiAgcmVuZGVyKCkge1xuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IHN0eWxlPXt7aGVpZ2h0OiAnMTAwJSd9fT5cbiAgICAgICAgPFR0eVRlcm1pbmFsIHJlZj1cInR0eUNtbnRJbnN0YW5jZVwiIHR0eT17dGhpcy50dHl9IGNvbHM9e3RoaXMucHJvcHMuY29sc30gcm93cz17dGhpcy5wcm9wcy5yb3dzfSAvPlxuICAgICAgICB7IHRoaXMuc3RhdGUuaXNDb25uZWN0ZWQgPyA8RXZlbnRTdHJlYW1lciBzaWQ9e3RoaXMucHJvcHMuc2lkfS8+IDogbnVsbCB9XG4gICAgICA8L2Rpdj5cbiAgICApXG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IEFjdGl2ZVNlc3Npb247XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9jdXJyZW50U2Vzc2lvbi9hY3RpdmVTZXNzaW9uLmpzeFxuICoqLyIsInZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCdhcHAvc2Vzc2lvbicpO1xudmFyIHt1cGRhdGVTZXNzaW9ufSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3Nlc3Npb25zL2FjdGlvbnMnKTtcblxudmFyIEV2ZW50U3RyZWFtZXIgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIGNvbXBvbmVudERpZE1vdW50KCkge1xuICAgIGxldCB7c2lkfSA9IHRoaXMucHJvcHM7XG4gICAgbGV0IHt0b2tlbn0gPSBzZXNzaW9uLmdldFVzZXJEYXRhKCk7XG4gICAgbGV0IGNvbm5TdHIgPSBjZmcuYXBpLmdldEV2ZW50U3RyZWFtQ29ublN0cih0b2tlbiwgc2lkKTtcblxuICAgIHRoaXMuc29ja2V0ID0gbmV3IFdlYlNvY2tldChjb25uU3RyLCAncHJvdG8nKTtcbiAgICB0aGlzLnNvY2tldC5vbm1lc3NhZ2UgPSAoZXZlbnQpID0+IHtcbiAgICAgIHRyeVxuICAgICAge1xuICAgICAgICBsZXQganNvbiA9IEpTT04ucGFyc2UoZXZlbnQuZGF0YSk7XG4gICAgICAgIHVwZGF0ZVNlc3Npb24oanNvbi5zZXNzaW9uKTtcbiAgICAgIH1cbiAgICAgIGNhdGNoKGVycil7XG4gICAgICAgIGNvbnNvbGUubG9nKCdmYWlsZWQgdG8gcGFyc2UgZXZlbnQgc3RyZWFtIGRhdGEnKTtcbiAgICAgIH1cblxuICAgIH07XG4gICAgdGhpcy5zb2NrZXQub25jbG9zZSA9ICgpID0+IHt9O1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50KCkge1xuICAgIHRoaXMuc29ja2V0LmNsb3NlKCk7XG4gIH0sXG5cbiAgc2hvdWxkQ29tcG9uZW50VXBkYXRlKCkge1xuICAgIHJldHVybiBmYWxzZTtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuIG51bGw7XG4gIH1cbn0pO1xuXG5leHBvcnQgZGVmYXVsdCBFdmVudFN0cmVhbWVyO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvY3VycmVudFNlc3Npb24vZXZlbnRTdHJlYW1lci5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHtnZXR0ZXJzLCBhY3Rpb25zfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsLycpO1xudmFyIE5vdEZvdW5kUGFnZSA9IHJlcXVpcmUoJ2FwcC9jb21wb25lbnRzL25vdEZvdW5kUGFnZS5qc3gnKTtcbnZhciBTZXNzaW9uUGxheWVyID0gcmVxdWlyZSgnLi9zZXNzaW9uUGxheWVyLmpzeCcpO1xudmFyIEFjdGl2ZVNlc3Npb24gPSByZXF1aXJlKCcuL2FjdGl2ZVNlc3Npb24uanN4Jyk7XG5cbnZhciBDdXJyZW50U2Vzc2lvbkhvc3QgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIGN1cnJlbnRTZXNzaW9uOiBnZXR0ZXJzLmFjdGl2ZVNlc3Npb25cbiAgICB9XG4gIH0sXG5cbiAgY29tcG9uZW50RGlkTW91bnQoKXtcbiAgICB2YXIgeyBzaWQgfSA9IHRoaXMucHJvcHMucGFyYW1zO1xuICAgIGlmKCF0aGlzLnN0YXRlLmN1cnJlbnRTZXNzaW9uKXtcbiAgICAgIGFjdGlvbnMub3BlblNlc3Npb24oc2lkKTtcbiAgICB9XG4gIH0sXG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICB2YXIgY3VycmVudFNlc3Npb24gPSB0aGlzLnN0YXRlLmN1cnJlbnRTZXNzaW9uO1xuICAgIGlmKCFjdXJyZW50U2Vzc2lvbil7XG4gICAgICByZXR1cm4gbnVsbDtcbiAgICB9XG5cbiAgICBpZihjdXJyZW50U2Vzc2lvbi5pc05ld1Nlc3Npb24gfHwgY3VycmVudFNlc3Npb24uYWN0aXZlKXtcbiAgICAgIHJldHVybiA8QWN0aXZlU2Vzc2lvbiBhY3RpdmVTZXNzaW9uPXtjdXJyZW50U2Vzc2lvbn0vPjtcbiAgICB9XG5cbiAgICByZXR1cm4gPFNlc3Npb25QbGF5ZXIgYWN0aXZlU2Vzc2lvbj17Y3VycmVudFNlc3Npb259Lz47XG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IEN1cnJlbnRTZXNzaW9uSG9zdDtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2N1cnJlbnRTZXNzaW9uL21haW4uanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciBSZWFjdFNsaWRlciA9IHJlcXVpcmUoJ3JlYWN0LXNsaWRlcicpO1xudmFyIFR0eVBsYXllciA9IHJlcXVpcmUoJ2FwcC9jb21tb24vdHR5UGxheWVyJylcbnZhciBUdHlUZXJtaW5hbCA9IHJlcXVpcmUoJy4vLi4vdGVybWluYWwuanN4Jyk7XG52YXIgU2Vzc2lvbkxlZnRQYW5lbCA9IHJlcXVpcmUoJy4vc2Vzc2lvbkxlZnRQYW5lbCcpO1xuXG52YXIgU2Vzc2lvblBsYXllciA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgY2FsY3VsYXRlU3RhdGUoKXtcbiAgICByZXR1cm4ge1xuICAgICAgbGVuZ3RoOiB0aGlzLnR0eS5sZW5ndGgsXG4gICAgICBtaW46IDEsXG4gICAgICBpc1BsYXlpbmc6IHRoaXMudHR5LmlzUGxheWluZyxcbiAgICAgIGN1cnJlbnQ6IHRoaXMudHR5LmN1cnJlbnQsXG4gICAgICBjYW5QbGF5OiB0aGlzLnR0eS5sZW5ndGggPiAxXG4gICAgfTtcbiAgfSxcblxuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgdmFyIHNpZCA9IHRoaXMucHJvcHMuYWN0aXZlU2Vzc2lvbi5zaWQ7XG4gICAgdGhpcy50dHkgPSBuZXcgVHR5UGxheWVyKHtzaWR9KTtcbiAgICByZXR1cm4gdGhpcy5jYWxjdWxhdGVTdGF0ZSgpO1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50KCkge1xuICAgIHRoaXMudHR5LnN0b3AoKTtcbiAgICB0aGlzLnR0eS5yZW1vdmVBbGxMaXN0ZW5lcnMoKTtcbiAgfSxcblxuICBjb21wb25lbnREaWRNb3VudCgpIHtcbiAgICB0aGlzLnR0eS5vbignY2hhbmdlJywgKCk9PntcbiAgICAgIHZhciBuZXdTdGF0ZSA9IHRoaXMuY2FsY3VsYXRlU3RhdGUoKTtcbiAgICAgIHRoaXMuc2V0U3RhdGUobmV3U3RhdGUpO1xuICAgIH0pO1xuICB9LFxuXG4gIHRvZ2dsZVBsYXlTdG9wKCl7XG4gICAgaWYodGhpcy5zdGF0ZS5pc1BsYXlpbmcpe1xuICAgICAgdGhpcy50dHkuc3RvcCgpO1xuICAgIH1lbHNle1xuICAgICAgdGhpcy50dHkucGxheSgpO1xuICAgIH1cbiAgfSxcblxuICBtb3ZlKHZhbHVlKXtcbiAgICB0aGlzLnR0eS5tb3ZlKHZhbHVlKTtcbiAgfSxcblxuICBvbkJlZm9yZUNoYW5nZSgpe1xuICAgIHRoaXMudHR5LnN0b3AoKTtcbiAgfSxcblxuICBvbkFmdGVyQ2hhbmdlKHZhbHVlKXtcbiAgICB0aGlzLnR0eS5wbGF5KCk7XG4gICAgdGhpcy50dHkubW92ZSh2YWx1ZSk7XG4gIH0sXG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICB2YXIge2lzUGxheWluZ30gPSB0aGlzLnN0YXRlO1xuXG4gICAgcmV0dXJuIChcbiAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtY3VycmVudC1zZXNzaW9uIGdydi1zZXNzaW9uLXBsYXllclwiPlxuICAgICAgIDxTZXNzaW9uTGVmdFBhbmVsLz5cbiAgICAgICA8VHR5VGVybWluYWwgcmVmPVwidGVybVwiIHR0eT17dGhpcy50dHl9IGNvbHM9XCI1XCIgcm93cz1cIjVcIiAvPlxuICAgICAgIDxSZWFjdFNsaWRlclxuICAgICAgICAgIG1pbj17dGhpcy5zdGF0ZS5taW59XG4gICAgICAgICAgbWF4PXt0aGlzLnN0YXRlLmxlbmd0aH1cbiAgICAgICAgICB2YWx1ZT17dGhpcy5zdGF0ZS5jdXJyZW50fSAgICBcbiAgICAgICAgICBvbkFmdGVyQ2hhbmdlPXt0aGlzLm9uQWZ0ZXJDaGFuZ2V9XG4gICAgICAgICAgb25CZWZvcmVDaGFuZ2U9e3RoaXMub25CZWZvcmVDaGFuZ2V9XG4gICAgICAgICAgZGVmYXVsdFZhbHVlPXsxfVxuICAgICAgICAgIHdpdGhCYXJzXG4gICAgICAgICAgY2xhc3NOYW1lPVwiZ3J2LXNsaWRlclwiPlxuICAgICAgIDwvUmVhY3RTbGlkZXI+XG4gICAgICAgPGJ1dHRvbiBjbGFzc05hbWU9XCJidG5cIiBvbkNsaWNrPXt0aGlzLnRvZ2dsZVBsYXlTdG9wfT5cbiAgICAgICAgIHsgaXNQbGF5aW5nID8gPGkgY2xhc3NOYW1lPVwiZmEgZmEtc3RvcFwiPjwvaT4gOiAgPGkgY2xhc3NOYW1lPVwiZmEgZmEtcGxheVwiPjwvaT4gfVxuICAgICAgIDwvYnV0dG9uPlxuICAgICA8L2Rpdj5cbiAgICAgKTtcbiAgfVxufSk7XG5cbmV4cG9ydCBkZWZhdWx0IFNlc3Npb25QbGF5ZXI7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9jdXJyZW50U2Vzc2lvbi9zZXNzaW9uUGxheWVyLmpzeFxuICoqLyIsIm1vZHVsZS5leHBvcnRzLkFwcCA9IHJlcXVpcmUoJy4vYXBwLmpzeCcpO1xubW9kdWxlLmV4cG9ydHMuTG9naW4gPSByZXF1aXJlKCcuL2xvZ2luLmpzeCcpO1xubW9kdWxlLmV4cG9ydHMuTmV3VXNlciA9IHJlcXVpcmUoJy4vbmV3VXNlci5qc3gnKTtcbm1vZHVsZS5leHBvcnRzLk5vZGVzID0gcmVxdWlyZSgnLi9ub2Rlcy9tYWluLmpzeCcpO1xubW9kdWxlLmV4cG9ydHMuU2Vzc2lvbnMgPSByZXF1aXJlKCcuL3Nlc3Npb25zL21haW4uanN4Jyk7XG5tb2R1bGUuZXhwb3J0cy5DdXJyZW50U2Vzc2lvbkhvc3QgPSByZXF1aXJlKCcuL2N1cnJlbnRTZXNzaW9uL21haW4uanN4Jyk7XG5tb2R1bGUuZXhwb3J0cy5Ob3RGb3VuZFBhZ2UgPSByZXF1aXJlKCcuL25vdEZvdW5kUGFnZS5qc3gnKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2luZGV4LmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgJCA9IHJlcXVpcmUoJ2pRdWVyeScpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIExpbmtlZFN0YXRlTWl4aW4gPSByZXF1aXJlKCdyZWFjdC1hZGRvbnMtbGlua2VkLXN0YXRlLW1peGluJyk7XG52YXIge2FjdGlvbnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvdXNlcicpO1xudmFyIEdvb2dsZUF1dGhJbmZvID0gcmVxdWlyZSgnLi9nb29nbGVBdXRoTG9nbycpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxudmFyIExvZ2luSW5wdXRGb3JtID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW0xpbmtlZFN0YXRlTWl4aW5dLFxuXG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4ge1xuICAgICAgdXNlcjogJycsXG4gICAgICBwYXNzd29yZDogJycsXG4gICAgICB0b2tlbjogJydcbiAgICB9XG4gIH0sXG5cbiAgb25DbGljazogZnVuY3Rpb24oZSkge1xuICAgIGUucHJldmVudERlZmF1bHQoKTtcbiAgICBpZiAodGhpcy5pc1ZhbGlkKCkpIHtcbiAgICAgIHRoaXMucHJvcHMub25DbGljayh0aGlzLnN0YXRlKTtcbiAgICB9XG4gIH0sXG5cbiAgaXNWYWxpZDogZnVuY3Rpb24oKSB7XG4gICAgdmFyICRmb3JtID0gJCh0aGlzLnJlZnMuZm9ybSk7XG4gICAgcmV0dXJuICRmb3JtLmxlbmd0aCA9PT0gMCB8fCAkZm9ybS52YWxpZCgpO1xuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICByZXR1cm4gKFxuICAgICAgPGZvcm0gcmVmPVwiZm9ybVwiIGNsYXNzTmFtZT1cImdydi1sb2dpbi1pbnB1dC1mb3JtXCI+XG4gICAgICAgIDxoMz4gV2VsY29tZSB0byBUZWxlcG9ydCA8L2gzPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0IGF1dG9Gb2N1cyB2YWx1ZUxpbms9e3RoaXMubGlua1N0YXRlKCd1c2VyJyl9IGNsYXNzTmFtZT1cImZvcm0tY29udHJvbCByZXF1aXJlZFwiIHBsYWNlaG9sZGVyPVwiVXNlciBuYW1lXCIgbmFtZT1cInVzZXJOYW1lXCIgLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImZvcm0tZ3JvdXBcIj5cbiAgICAgICAgICAgIDxpbnB1dCB2YWx1ZUxpbms9e3RoaXMubGlua1N0YXRlKCdwYXNzd29yZCcpfSB0eXBlPVwicGFzc3dvcmRcIiBuYW1lPVwicGFzc3dvcmRcIiBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2wgcmVxdWlyZWRcIiBwbGFjZWhvbGRlcj1cIlBhc3N3b3JkXCIvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0IHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3Rva2VuJyl9IGNsYXNzTmFtZT1cImZvcm0tY29udHJvbCByZXF1aXJlZFwiIG5hbWU9XCJ0b2tlblwiIHBsYWNlaG9sZGVyPVwiVHdvIGZhY3RvciB0b2tlbiAoR29vZ2xlIEF1dGhlbnRpY2F0b3IpXCIvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxidXR0b24gdHlwZT1cInN1Ym1pdFwiIGNsYXNzTmFtZT1cImJ0biBidG4tcHJpbWFyeSBibG9jayBmdWxsLXdpZHRoIG0tYlwiIG9uQ2xpY2s9e3RoaXMub25DbGlja30+TG9naW48L2J1dHRvbj5cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Zvcm0+XG4gICAgKTtcbiAgfVxufSlcblxudmFyIExvZ2luID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW3JlYWN0b3IuUmVhY3RNaXhpbl0sXG5cbiAgZ2V0RGF0YUJpbmRpbmdzKCkge1xuICAgIHJldHVybiB7XG4gICAgfVxuICB9LFxuXG4gIG9uQ2xpY2soaW5wdXREYXRhKXtcbiAgICB2YXIgbG9jID0gdGhpcy5wcm9wcy5sb2NhdGlvbjtcbiAgICB2YXIgcmVkaXJlY3QgPSBjZmcucm91dGVzLmFwcDtcblxuICAgIGlmKGxvYy5zdGF0ZSAmJiBsb2Muc3RhdGUucmVkaXJlY3RUbyl7XG4gICAgICByZWRpcmVjdCA9IGxvYy5zdGF0ZS5yZWRpcmVjdFRvO1xuICAgIH1cblxuICAgIGFjdGlvbnMubG9naW4oaW5wdXREYXRhLCByZWRpcmVjdCk7XG4gIH0sXG5cbiAgcmVuZGVyKCkge1xuICAgIHZhciBpc1Byb2Nlc3NpbmcgPSBmYWxzZTsvL3RoaXMuc3RhdGUudXNlclJlcXVlc3QuZ2V0KCdpc0xvYWRpbmcnKTtcbiAgICB2YXIgaXNFcnJvciA9IGZhbHNlOy8vdGhpcy5zdGF0ZS51c2VyUmVxdWVzdC5nZXQoJ2lzRXJyb3InKTtcbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbG9naW4gdGV4dC1jZW50ZXJcIj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbG9nby10cHJ0XCI+PC9kaXY+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWNvbnRlbnQgZ3J2LWZsZXhcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LWNvbHVtblwiPlxuICAgICAgICAgICAgPExvZ2luSW5wdXRGb3JtIG9uQ2xpY2s9e3RoaXMub25DbGlja30vPlxuICAgICAgICAgICAgPEdvb2dsZUF1dGhJbmZvLz5cbiAgICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWxvZ2luLWluZm9cIj5cbiAgICAgICAgICAgICAgPGkgY2xhc3NOYW1lPVwiZmEgZmEtcXVlc3Rpb25cIj48L2k+XG4gICAgICAgICAgICAgIDxzdHJvbmc+TmV3IEFjY291bnQgb3IgZm9yZ290IHBhc3N3b3JkPzwvc3Ryb25nPlxuICAgICAgICAgICAgICA8ZGl2PkFzayBmb3IgYXNzaXN0YW5jZSBmcm9tIHlvdXIgQ29tcGFueSBhZG1pbmlzdHJhdG9yPC9kaXY+XG4gICAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBMb2dpbjtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2xvZ2luLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgeyBSb3V0ZXIsIEluZGV4TGluaywgSGlzdG9yeSB9ID0gcmVxdWlyZSgncmVhY3Qtcm91dGVyJyk7XG52YXIgZ2V0dGVycyA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3VzZXIvZ2V0dGVycycpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxudmFyIG1lbnVJdGVtcyA9IFtcbiAge2ljb246ICdmYSBmYS1jb2dzJywgdG86IGNmZy5yb3V0ZXMubm9kZXMsIHRpdGxlOiAnTm9kZXMnfSxcbiAge2ljb246ICdmYSBmYS1zaXRlbWFwJywgdG86IGNmZy5yb3V0ZXMuc2Vzc2lvbnMsIHRpdGxlOiAnU2Vzc2lvbnMnfVxuXTtcblxudmFyIE5hdkxlZnRCYXIgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpe1xuICAgIHZhciBpdGVtcyA9IG1lbnVJdGVtcy5tYXAoKGksIGluZGV4KT0+e1xuICAgICAgdmFyIGNsYXNzTmFtZSA9IHRoaXMuY29udGV4dC5yb3V0ZXIuaXNBY3RpdmUoaS50bykgPyAnYWN0aXZlJyA6ICcnO1xuICAgICAgcmV0dXJuIChcbiAgICAgICAgPGxpIGtleT17aW5kZXh9IGNsYXNzTmFtZT17Y2xhc3NOYW1lfT5cbiAgICAgICAgICA8SW5kZXhMaW5rIHRvPXtpLnRvfT5cbiAgICAgICAgICAgIDxpIGNsYXNzTmFtZT17aS5pY29ufSB0aXRsZT17aS50aXRsZX0vPlxuICAgICAgICAgIDwvSW5kZXhMaW5rPlxuICAgICAgICA8L2xpPlxuICAgICAgKTtcbiAgICB9KTtcblxuICAgIGl0ZW1zLnB1c2goKFxuICAgICAgPGxpIGtleT17bWVudUl0ZW1zLmxlbmd0aH0+XG4gICAgICAgIDxhIGhyZWY9e2NmZy5oZWxwVXJsfT5cbiAgICAgICAgICA8aSBjbGFzc05hbWU9XCJmYSBmYS1xdWVzdGlvblwiIHRpdGxlPVwiaGVscFwiLz5cbiAgICAgICAgPC9hPlxuICAgICAgPC9saT4pKTtcblxuICAgIHJldHVybiAoXG4gICAgICA8bmF2IGNsYXNzTmFtZT0nZ3J2LW5hdiBuYXZiYXItZGVmYXVsdCBuYXZiYXItc3RhdGljLXNpZGUnIHJvbGU9J25hdmlnYXRpb24nPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT0nJz5cbiAgICAgICAgICA8dWwgY2xhc3NOYW1lPSduYXYnIGlkPSdzaWRlLW1lbnUnPlxuICAgICAgICAgICAgPGxpPjxkaXYgY2xhc3NOYW1lPVwiZ3J2LWNpcmNsZSB0ZXh0LXVwcGVyY2FzZVwiPjxzcGFuPntnZXRVc2VyTmFtZUxldHRlcigpfTwvc3Bhbj48L2Rpdj48L2xpPlxuICAgICAgICAgICAge2l0ZW1zfVxuICAgICAgICAgIDwvdWw+XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9uYXY+XG4gICAgKTtcbiAgfVxufSk7XG5cbk5hdkxlZnRCYXIuY29udGV4dFR5cGVzID0ge1xuICByb3V0ZXI6IFJlYWN0LlByb3BUeXBlcy5vYmplY3QuaXNSZXF1aXJlZFxufVxuXG5mdW5jdGlvbiBnZXRVc2VyTmFtZUxldHRlcigpe1xuICB2YXIge3Nob3J0RGlzcGxheU5hbWV9ID0gcmVhY3Rvci5ldmFsdWF0ZShnZXR0ZXJzLnVzZXIpO1xuICByZXR1cm4gc2hvcnREaXNwbGF5TmFtZTtcbn1cblxubW9kdWxlLmV4cG9ydHMgPSBOYXZMZWZ0QmFyO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvbmF2TGVmdEJhci5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyICQgPSByZXF1aXJlKCdqUXVlcnknKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7YWN0aW9ucywgZ2V0dGVyc30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9pbnZpdGUnKTtcbnZhciB1c2VyTW9kdWxlID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvdXNlcicpO1xudmFyIExpbmtlZFN0YXRlTWl4aW4gPSByZXF1aXJlKCdyZWFjdC1hZGRvbnMtbGlua2VkLXN0YXRlLW1peGluJyk7XG52YXIgR29vZ2xlQXV0aEluZm8gPSByZXF1aXJlKCcuL2dvb2dsZUF1dGhMb2dvJyk7XG5cbnZhciBJbnZpdGVJbnB1dEZvcm0gPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbTGlua2VkU3RhdGVNaXhpbl0sXG5cbiAgY29tcG9uZW50RGlkTW91bnQoKXtcbiAgICAkKHRoaXMucmVmcy5mb3JtKS52YWxpZGF0ZSh7XG4gICAgICBydWxlczp7XG4gICAgICAgIHBhc3N3b3JkOntcbiAgICAgICAgICBtaW5sZW5ndGg6IDYsXG4gICAgICAgICAgcmVxdWlyZWQ6IHRydWVcbiAgICAgICAgfSxcbiAgICAgICAgcGFzc3dvcmRDb25maXJtZWQ6e1xuICAgICAgICAgIHJlcXVpcmVkOiB0cnVlLFxuICAgICAgICAgIGVxdWFsVG86IHRoaXMucmVmcy5wYXNzd29yZFxuICAgICAgICB9XG4gICAgICB9LFxuXG4gICAgICBtZXNzYWdlczoge1xuICBcdFx0XHRwYXNzd29yZENvbmZpcm1lZDoge1xuICBcdFx0XHRcdG1pbmxlbmd0aDogJC52YWxpZGF0b3IuZm9ybWF0KCdFbnRlciBhdCBsZWFzdCB7MH0gY2hhcmFjdGVycycpLFxuICBcdFx0XHRcdGVxdWFsVG86ICdFbnRlciB0aGUgc2FtZSBwYXNzd29yZCBhcyBhYm92ZSdcbiAgXHRcdFx0fVxuICAgICAgfVxuICAgIH0pXG4gIH0sXG5cbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB7XG4gICAgICBuYW1lOiB0aGlzLnByb3BzLmludml0ZS51c2VyLFxuICAgICAgcHN3OiAnJyxcbiAgICAgIHBzd0NvbmZpcm1lZDogJycsXG4gICAgICB0b2tlbjogJydcbiAgICB9XG4gIH0sXG5cbiAgb25DbGljayhlKSB7XG4gICAgZS5wcmV2ZW50RGVmYXVsdCgpO1xuICAgIGlmICh0aGlzLmlzVmFsaWQoKSkge1xuICAgICAgdXNlck1vZHVsZS5hY3Rpb25zLnNpZ25VcCh7XG4gICAgICAgIG5hbWU6IHRoaXMuc3RhdGUubmFtZSxcbiAgICAgICAgcHN3OiB0aGlzLnN0YXRlLnBzdyxcbiAgICAgICAgdG9rZW46IHRoaXMuc3RhdGUudG9rZW4sXG4gICAgICAgIGludml0ZVRva2VuOiB0aGlzLnByb3BzLmludml0ZS5pbnZpdGVfdG9rZW59KTtcbiAgICB9XG4gIH0sXG5cbiAgaXNWYWxpZCgpIHtcbiAgICB2YXIgJGZvcm0gPSAkKHRoaXMucmVmcy5mb3JtKTtcbiAgICByZXR1cm4gJGZvcm0ubGVuZ3RoID09PSAwIHx8ICRmb3JtLnZhbGlkKCk7XG4gIH0sXG5cbiAgcmVuZGVyKCkge1xuICAgIHJldHVybiAoXG4gICAgICA8Zm9ybSByZWY9XCJmb3JtXCIgY2xhc3NOYW1lPVwiZ3J2LWludml0ZS1pbnB1dC1mb3JtXCI+XG4gICAgICAgIDxoMz4gR2V0IHN0YXJ0ZWQgd2l0aCBUZWxlcG9ydCA8L2gzPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0XG4gICAgICAgICAgICAgIHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ25hbWUnKX1cbiAgICAgICAgICAgICAgbmFtZT1cInVzZXJOYW1lXCJcbiAgICAgICAgICAgICAgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sIHJlcXVpcmVkXCJcbiAgICAgICAgICAgICAgcGxhY2Vob2xkZXI9XCJVc2VyIG5hbWVcIi8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXRcbiAgICAgICAgICAgICAgYXV0b0ZvY3VzXG4gICAgICAgICAgICAgIHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3BzdycpfVxuICAgICAgICAgICAgICByZWY9XCJwYXNzd29yZFwiXG4gICAgICAgICAgICAgIHR5cGU9XCJwYXNzd29yZFwiXG4gICAgICAgICAgICAgIG5hbWU9XCJwYXNzd29yZFwiXG4gICAgICAgICAgICAgIGNsYXNzTmFtZT1cImZvcm0tY29udHJvbFwiXG4gICAgICAgICAgICAgIHBsYWNlaG9sZGVyPVwiUGFzc3dvcmRcIiAvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0XG4gICAgICAgICAgICAgIHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3Bzd0NvbmZpcm1lZCcpfVxuICAgICAgICAgICAgICB0eXBlPVwicGFzc3dvcmRcIlxuICAgICAgICAgICAgICBuYW1lPVwicGFzc3dvcmRDb25maXJtZWRcIlxuICAgICAgICAgICAgICBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2xcIlxuICAgICAgICAgICAgICBwbGFjZWhvbGRlcj1cIlBhc3N3b3JkIGNvbmZpcm1cIi8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXRcbiAgICAgICAgICAgICAgbmFtZT1cInRva2VuXCIgICAgICAgICAgICAgIFxuICAgICAgICAgICAgICB2YWx1ZUxpbms9e3RoaXMubGlua1N0YXRlKCd0b2tlbicpfVxuICAgICAgICAgICAgICBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2wgcmVxdWlyZWRcIlxuICAgICAgICAgICAgICBwbGFjZWhvbGRlcj1cIlR3byBmYWN0b3IgdG9rZW4gKEdvb2dsZSBBdXRoZW50aWNhdG9yKVwiIC8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGJ1dHRvbiB0eXBlPVwic3VibWl0XCIgZGlzYWJsZWQ9e3RoaXMucHJvcHMuYXR0ZW1wLmlzUHJvY2Vzc2luZ30gY2xhc3NOYW1lPVwiYnRuIGJ0bi1wcmltYXJ5IGJsb2NrIGZ1bGwtd2lkdGggbS1iXCIgb25DbGljaz17dGhpcy5vbkNsaWNrfSA+U2lnbiB1cDwvYnV0dG9uPlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZm9ybT5cbiAgICApO1xuICB9XG59KVxuXG52YXIgSW52aXRlID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW3JlYWN0b3IuUmVhY3RNaXhpbl0sXG5cbiAgZ2V0RGF0YUJpbmRpbmdzKCkge1xuICAgIHJldHVybiB7XG4gICAgICBpbnZpdGU6IGdldHRlcnMuaW52aXRlLFxuICAgICAgYXR0ZW1wOiBnZXR0ZXJzLmF0dGVtcFxuICAgIH1cbiAgfSxcblxuICBjb21wb25lbnREaWRNb3VudCgpe1xuICAgIGFjdGlvbnMuZmV0Y2hJbnZpdGUodGhpcy5wcm9wcy5wYXJhbXMuaW52aXRlVG9rZW4pO1xuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgaWYoIXRoaXMuc3RhdGUuaW52aXRlKSB7XG4gICAgICByZXR1cm4gbnVsbDtcbiAgICB9XG5cbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtaW52aXRlIHRleHQtY2VudGVyXCI+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWxvZ28tdHBydFwiPjwvZGl2PlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1jb250ZW50IGdydi1mbGV4XCI+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZmxleC1jb2x1bW5cIj5cbiAgICAgICAgICAgIDxJbnZpdGVJbnB1dEZvcm0gYXR0ZW1wPXt0aGlzLnN0YXRlLmF0dGVtcH0gaW52aXRlPXt0aGlzLnN0YXRlLmludml0ZS50b0pTKCl9Lz5cbiAgICAgICAgICAgIDxHb29nbGVBdXRoSW5mby8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZmxleC1jb2x1bW4gZ3J2LWludml0ZS1iYXJjb2RlXCI+XG4gICAgICAgICAgICA8aDQ+U2NhbiBiYXIgY29kZSBmb3IgYXV0aCB0b2tlbiA8YnIvPiA8c21hbGw+U2NhbiBiZWxvdyB0byBnZW5lcmF0ZSB5b3VyIHR3byBmYWN0b3IgdG9rZW48L3NtYWxsPjwvaDQ+XG4gICAgICAgICAgICA8aW1nIGNsYXNzTmFtZT1cImltZy10aHVtYm5haWxcIiBzcmM9eyBgZGF0YTppbWFnZS9wbmc7YmFzZTY0LCR7dGhpcy5zdGF0ZS5pbnZpdGUuZ2V0KCdxcicpfWAgfSAvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IEludml0ZTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL25ld1VzZXIuanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7Z2V0dGVyc30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9kaWFsb2dzJyk7XG52YXIge2Nsb3NlU2VsZWN0Tm9kZURpYWxvZ30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9kaWFsb2dzL2FjdGlvbnMnKTtcbnZhciB7Y2hhbmdlU2VydmVyfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGlvbnMnKTtcbnZhciBOb2RlTGlzdCA9IHJlcXVpcmUoJy4vbm9kZXMvbWFpbi5qc3gnKTtcblxudmFyIFNlbGVjdE5vZGVEaWFsb2cgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIGRpYWxvZ3M6IGdldHRlcnMuZGlhbG9nc1xuICAgIH1cbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuIHRoaXMuc3RhdGUuZGlhbG9ncy5pc1NlbGVjdE5vZGVEaWFsb2dPcGVuID8gPERpYWxvZy8+IDogbnVsbDtcbiAgfVxufSk7XG5cbnZhciBEaWFsb2cgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgb25Mb2dpbkNsaWNrKHNlcnZlcklkLCBsb2dpbil7XG4gICAgY2hhbmdlU2VydmVyKHNlcnZlcklkLCBsb2dpbik7XG4gICAgY2xvc2VTZWxlY3ROb2RlRGlhbG9nKCk7XG4gIH0sXG5cbiAgY29tcG9uZW50V2lsbFVubW91bnQoY2FsbGJhY2spe1xuICAgICQoJy5tb2RhbCcpLm1vZGFsKCdoaWRlJyk7XG4gIH0sXG5cbiAgY29tcG9uZW50RGlkTW91bnQoKXtcbiAgICAkKCcubW9kYWwnKS5tb2RhbCgnc2hvdycpO1xuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJtb2RhbCBmYWRlIGdydi1kaWFsb2ctc2VsZWN0LW5vZGVcIiB0YWJJbmRleD17LTF9IHJvbGU9XCJkaWFsb2dcIj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJtb2RhbC1kaWFsb2dcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIm1vZGFsLWNvbnRlbnRcIj5cbiAgICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwibW9kYWwtaGVhZGVyXCI+XG4gICAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwibW9kYWwtYm9keVwiPlxuICAgICAgICAgICAgICA8Tm9kZUxpc3Qgb25Mb2dpbkNsaWNrPXt0aGlzLm9uTG9naW5DbGlja30vPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIm1vZGFsLWZvb3RlclwiPlxuICAgICAgICAgICAgICA8YnV0dG9uIG9uQ2xpY2s9e2Nsb3NlU2VsZWN0Tm9kZURpYWxvZ30gdHlwZT1cImJ1dHRvblwiIGNsYXNzTmFtZT1cImJ0biBidG4tcHJpbWFyeVwiPlxuICAgICAgICAgICAgICAgIENsb3NlXG4gICAgICAgICAgICAgIDwvYnV0dG9uPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9kaXY+XG4gICAgKTtcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gU2VsZWN0Tm9kZURpYWxvZztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL3NlbGVjdE5vZGVEaWFsb2cuanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7IExpbmsgfSA9IHJlcXVpcmUoJ3JlYWN0LXJvdXRlcicpO1xudmFyIHtUYWJsZSwgQ29sdW1uLCBDZWxsLCBUZXh0Q2VsbH0gPSByZXF1aXJlKCdhcHAvY29tcG9uZW50cy90YWJsZS5qc3gnKTtcbnZhciB7Z2V0dGVyc30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9zZXNzaW9ucycpO1xudmFyIHtvcGVufSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGlvbnMnKTtcbnZhciBtb21lbnQgPSAgcmVxdWlyZSgnbW9tZW50Jyk7XG5cbmNvbnN0IERhdGVDcmVhdGVkQ2VsbCA9ICh7IHJvd0luZGV4LCBkYXRhLCAuLi5wcm9wcyB9KSA9PiB7XG4gIHZhciBjcmVhdGVkID0gZGF0YVtyb3dJbmRleF0uY3JlYXRlZDtcbiAgdmFyIGRpc3BsYXlEYXRlID0gbW9tZW50KGNyZWF0ZWQpLmZyb21Ob3coKTtcbiAgcmV0dXJuIChcbiAgICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgICAgeyBkaXNwbGF5RGF0ZSB9XG4gICAgPC9DZWxsPlxuICApXG59O1xuXG5jb25zdCBEdXJhdGlvbkNlbGwgPSAoeyByb3dJbmRleCwgZGF0YSwgLi4ucHJvcHMgfSkgPT4ge1xuICB2YXIgY3JlYXRlZCA9IGRhdGFbcm93SW5kZXhdLmNyZWF0ZWQ7XG4gIHZhciBsYXN0QWN0aXZlID0gZGF0YVtyb3dJbmRleF0ubGFzdEFjdGl2ZTtcblxuICB2YXIgZW5kID0gbW9tZW50KGNyZWF0ZWQpO1xuICB2YXIgbm93ID0gbW9tZW50KGxhc3RBY3RpdmUpO1xuICB2YXIgZHVyYXRpb24gPSBtb21lbnQuZHVyYXRpb24obm93LmRpZmYoZW5kKSk7XG4gIHZhciBkaXNwbGF5RGF0ZSA9IGR1cmF0aW9uLmh1bWFuaXplKCk7XG5cbiAgcmV0dXJuIChcbiAgICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgICAgeyBkaXNwbGF5RGF0ZSB9XG4gICAgPC9DZWxsPlxuICApXG59O1xuXG5jb25zdCBVc2Vyc0NlbGwgPSAoeyByb3dJbmRleCwgZGF0YSwgLi4ucHJvcHMgfSkgPT4ge1xuICB2YXIgJHVzZXJzID0gZGF0YVtyb3dJbmRleF0ucGFydGllcy5tYXAoKGl0ZW0sIGl0ZW1JbmRleCk9PlxuICAgICg8c3BhbiBrZXk9e2l0ZW1JbmRleH0gc3R5bGU9e3tiYWNrZ3JvdW5kQ29sb3I6ICcjMWFiMzk0J319IGNsYXNzTmFtZT1cInRleHQtdXBwZXJjYXNlIGdydi1yb3VuZGVkIGxhYmVsIGxhYmVsLXByaW1hcnlcIj57aXRlbS51c2VyWzBdfTwvc3Bhbj4pXG4gIClcblxuICByZXR1cm4gKFxuICAgIDxDZWxsIHsuLi5wcm9wc30+XG4gICAgICA8ZGl2PlxuICAgICAgICB7JHVzZXJzfVxuICAgICAgPC9kaXY+XG4gICAgPC9DZWxsPlxuICApXG59O1xuXG5jb25zdCBCdXR0b25DZWxsID0gKHsgcm93SW5kZXgsIGRhdGEsIC4uLnByb3BzIH0pID0+IHtcbiAgdmFyIHsgc2Vzc2lvblVybCwgYWN0aXZlIH0gPSBkYXRhW3Jvd0luZGV4XTtcbiAgdmFyIFthY3Rpb25UZXh0LCBhY3Rpb25DbGFzc10gPSBhY3RpdmUgPyBbJ2pvaW4nLCAnYnRuLXdhcm5pbmcnXSA6IFsncGxheScsICdidG4tcHJpbWFyeSddO1xuICByZXR1cm4gKFxuICAgIDxDZWxsIHsuLi5wcm9wc30+XG4gICAgICA8TGluayB0bz17c2Vzc2lvblVybH0gY2xhc3NOYW1lPXtcImJ0biBcIiArYWN0aW9uQ2xhc3MrIFwiIGJ0bi14c1wifSB0eXBlPVwiYnV0dG9uXCI+e2FjdGlvblRleHR9PC9MaW5rPlxuICAgIDwvQ2VsbD5cbiAgKVxufVxuXG52YXIgU2Vzc2lvbkxpc3QgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIHNlc3Npb25zVmlldzogZ2V0dGVycy5zZXNzaW9uc1ZpZXdcbiAgICB9XG4gIH0sXG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICB2YXIgZGF0YSA9IHRoaXMuc3RhdGUuc2Vzc2lvbnNWaWV3O1xuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1zZXNzaW9uc1wiPlxuICAgICAgICA8aDE+IFNlc3Npb25zPC9oMT5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICAgICAgPFRhYmxlIHJvd0NvdW50PXtkYXRhLmxlbmd0aH0gY2xhc3NOYW1lPVwidGFibGUtc3RyaXBlZFwiPlxuICAgICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICAgIGNvbHVtbktleT1cInNpZFwiXG4gICAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPiBTZXNzaW9uIElEIDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17PFRleHRDZWxsIGRhdGE9e2RhdGF9Lz4gfVxuICAgICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD4gPC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXtcbiAgICAgICAgICAgICAgICAgICAgPEJ1dHRvbkNlbGwgZGF0YT17ZGF0YX0gLz5cbiAgICAgICAgICAgICAgICAgIH1cbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICAgIGNvbHVtbktleT1cInNlcnZlcklwXCJcbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+IE5vZGUgPC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXs8VGV4dENlbGwgZGF0YT17ZGF0YX0gLz4gfVxuICAgICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICAgICAgY29sdW1uS2V5PVwiY3JlYXRlZFwiXG4gICAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPiBDcmVhdGVkIDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17PERhdGVDcmVhdGVkQ2VsbCBkYXRhPXtkYXRhfS8+IH1cbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICAgIGNvbHVtbktleT1cInNlcnZlcklkXCJcbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+IEFjdGl2ZSA8L0NlbGw+IH1cbiAgICAgICAgICAgICAgICAgIGNlbGw9ezxVc2Vyc0NlbGwgZGF0YT17ZGF0YX0gLz4gfVxuICAgICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgIDwvVGFibGU+XG4gICAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApXG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IFNlc3Npb25MaXN0O1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvc2Vzc2lvbnMvbWFpbi5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHJlbmRlciA9IHJlcXVpcmUoJ3JlYWN0LWRvbScpLnJlbmRlcjtcbnZhciB7IFJvdXRlciwgUm91dGUsIFJlZGlyZWN0LCBJbmRleFJvdXRlLCBicm93c2VySGlzdG9yeSB9ID0gcmVxdWlyZSgncmVhY3Qtcm91dGVyJyk7XG52YXIgeyBBcHAsIExvZ2luLCBOb2RlcywgU2Vzc2lvbnMsIE5ld1VzZXIsIEN1cnJlbnRTZXNzaW9uSG9zdCwgTm90Rm91bmRQYWdlIH0gPSByZXF1aXJlKCcuL2NvbXBvbmVudHMnKTtcbnZhciB7ZW5zdXJlVXNlcn0gPSByZXF1aXJlKCcuL21vZHVsZXMvdXNlci9hY3Rpb25zJyk7XG52YXIgYXV0aCA9IHJlcXVpcmUoJy4vYXV0aCcpO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCcuL3Nlc3Npb24nKTtcbnZhciBjZmcgPSByZXF1aXJlKCcuL2NvbmZpZycpO1xuXG5yZXF1aXJlKCcuL21vZHVsZXMnKTtcblxuLy8gaW5pdCBzZXNzaW9uXG5zZXNzaW9uLmluaXQoKTtcblxuZnVuY3Rpb24gaGFuZGxlTG9nb3V0KG5leHRTdGF0ZSwgcmVwbGFjZSwgY2Ipe1xuICBhdXRoLmxvZ291dCgpO1xufVxuXG5yZW5kZXIoKFxuICA8Um91dGVyIGhpc3Rvcnk9e3Nlc3Npb24uZ2V0SGlzdG9yeSgpfT5cbiAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5sb2dpbn0gY29tcG9uZW50PXtMb2dpbn0vPlxuICAgIDxSb3V0ZSBwYXRoPXtjZmcucm91dGVzLmxvZ291dH0gb25FbnRlcj17aGFuZGxlTG9nb3V0fS8+XG4gICAgPFJvdXRlIHBhdGg9e2NmZy5yb3V0ZXMubmV3VXNlcn0gY29tcG9uZW50PXtOZXdVc2VyfS8+XG4gICAgPFJlZGlyZWN0IGZyb209e2NmZy5yb3V0ZXMuYXBwfSB0bz17Y2ZnLnJvdXRlcy5ub2Rlc30vPlxuICAgIDxSb3V0ZSBwYXRoPXtjZmcucm91dGVzLmFwcH0gY29tcG9uZW50PXtBcHB9IG9uRW50ZXI9e2Vuc3VyZVVzZXJ9ID5cbiAgICAgIDxSb3V0ZSBwYXRoPXtjZmcucm91dGVzLm5vZGVzfSBjb21wb25lbnQ9e05vZGVzfS8+XG4gICAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5hY3RpdmVTZXNzaW9ufSBjb21wb25lbnRzPXt7Q3VycmVudFNlc3Npb25Ib3N0OiBDdXJyZW50U2Vzc2lvbkhvc3R9fS8+XG4gICAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5zZXNzaW9uc30gY29tcG9uZW50PXtTZXNzaW9uc30vPlxuICAgIDwvUm91dGU+XG4gICAgPFJvdXRlIHBhdGg9XCIqXCIgY29tcG9uZW50PXtOb3RGb3VuZFBhZ2V9IC8+XG4gIDwvUm91dGVyPlxuKSwgZG9jdW1lbnQuZ2V0RWxlbWVudEJ5SWQoXCJhcHBcIikpO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2luZGV4LmpzeFxuICoqLyIsIm1vZHVsZS5leHBvcnRzID0gVGVybWluYWw7XG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiBleHRlcm5hbCBcIlRlcm1pbmFsXCJcbiAqKiBtb2R1bGUgaWQgPSA0MDRcbiAqKiBtb2R1bGUgY2h1bmtzID0gMVxuICoqLyIsIm1vZHVsZS5leHBvcnRzID0gXztcblxuXG4vKioqKioqKioqKioqKioqKipcbiAqKiBXRUJQQUNLIEZPT1RFUlxuICoqIGV4dGVybmFsIFwiX1wiXG4gKiogbW9kdWxlIGlkID0gNDA1XG4gKiogbW9kdWxlIGNodW5rcyA9IDFcbiAqKi8iXSwic291cmNlUm9vdCI6IiJ9