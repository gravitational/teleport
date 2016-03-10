webpackJsonp([1],{

/***/ 0:
/***/ function(module, exports, __webpack_require__) {

	module.exports = __webpack_require__(305);


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
	
	var _require = __webpack_require__(268);
	
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
	
	var _require = __webpack_require__(282);
	
	var uuid = _require.uuid;
	
	var api = __webpack_require__(32);
	var cfg = __webpack_require__(11);
	var getters = __webpack_require__(57);
	var sessionModule = __webpack_require__(107);
	
	var _require2 = __webpack_require__(93);
	
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
	
	module.exports.getters = __webpack_require__(57);
	module.exports.actions = __webpack_require__(43);
	module.exports.activeTermStore = __webpack_require__(94);

/***/ },

/***/ 45:
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

/***/ 57:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(61);
	
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

/***/ 58:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(97);
	
	var TLPT_DIALOG_SELECT_NODE_SHOW = _require.TLPT_DIALOG_SELECT_NODE_SHOW;
	var TLPT_DIALOG_SELECT_NODE_CLOSE = _require.TLPT_DIALOG_SELECT_NODE_CLOSE;
	
	var actions = {
	  showSelectNodeDialog: function showSelectNodeDialog() {
	    reactor.dispatch(TLPT_DIALOG_SELECT_NODE_SHOW);
	  },
	
	  closeSelectNodeDialog: function closeSelectNodeDialog() {
	    reactor.dispatch(TLPT_DIALOG_SELECT_NODE_CLOSE);
	  }
	};
	
	exports['default'] = actions;
	module.exports = exports['default'];

/***/ },

/***/ 59:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(61);
	
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

/***/ 60:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	var api = __webpack_require__(32);
	var cfg = __webpack_require__(11);
	
	var _require = __webpack_require__(106);
	
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

/***/ 61:
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

/***/ 91:
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

/***/ 92:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };
	
	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError('Cannot call a class as a function'); } }
	
	function _inherits(subClass, superClass) { if (typeof superClass !== 'function' && superClass !== null) { throw new TypeError('Super expression must either be null or a function, not ' + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }
	
	var EventEmitter = __webpack_require__(283).EventEmitter;
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
	    this.disconnect();
	    this.socket.onopen = null;
	    this.socket.onmessage = null;
	    this.socket.onclose = null;
	
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

/***/ 93:
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

/***/ 94:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(93);
	
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

/***/ 95:
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

/***/ 96:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(95);
	
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

/***/ 97:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(20);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_DIALOG_SELECT_NODE_SHOW: null,
	  TLPT_DIALOG_SELECT_NODE_CLOSE: null
	});
	module.exports = exports['default'];

/***/ },

/***/ 98:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(97);
	
	var TLPT_DIALOG_SELECT_NODE_SHOW = _require2.TLPT_DIALOG_SELECT_NODE_SHOW;
	var TLPT_DIALOG_SELECT_NODE_CLOSE = _require2.TLPT_DIALOG_SELECT_NODE_CLOSE;
	exports['default'] = Store({
	
	  getInitialState: function getInitialState() {
	    return toImmutable({
	      isSelectNodeDialogOpen: false
	    });
	  },
	
	  initialize: function initialize() {
	    this.on(TLPT_DIALOG_SELECT_NODE_SHOW, showSelectNodeDialog);
	    this.on(TLPT_DIALOG_SELECT_NODE_CLOSE, closeSelectNodeDialog);
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

/***/ 99:
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

/***/ 100:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(99);
	
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

/***/ 101:
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

/***/ 102:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(101);
	
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

/***/ 103:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(101);
	
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

/***/ 104:
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

/***/ 105:
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

/***/ 106:
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

/***/ 107:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(61);
	module.exports.actions = __webpack_require__(60);
	module.exports.activeTermStore = __webpack_require__(108);

/***/ },

/***/ 108:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(106);
	
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

/***/ 109:
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

/***/ 110:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(109);
	
	var TLPT_RECEIVE_USER = _require.TLPT_RECEIVE_USER;
	
	var _require2 = __webpack_require__(105);
	
	var TRYING_TO_SIGN_UP = _require2.TRYING_TO_SIGN_UP;
	
	var restApiActions = __webpack_require__(280);
	var auth = __webpack_require__(91);
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

/***/ 111:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(45);
	module.exports.actions = __webpack_require__(110);
	module.exports.nodeStore = __webpack_require__(112);

/***/ },

/***/ 112:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(109);
	
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

/***/ 217:
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

/***/ 218:
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

/***/ 219:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }
	
	var React = __webpack_require__(4);
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(279);
	
	var getters = _require.getters;
	var actions = _require.actions;
	
	var userGetters = __webpack_require__(45);
	
	var _require2 = __webpack_require__(222);
	
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
	  var logins = _ref3.logins;
	  var onLoginClick = _ref3.onLoginClick;
	  var rowIndex = _ref3.rowIndex;
	  var data = _ref3.data;
	
	  var props = _objectWithoutProperties(_ref3, ['logins', 'onLoginClick', 'rowIndex', 'data']);
	
	  if (!logins || logins.length === 0) {
	    return React.createElement(Cell, props);
	  }
	
	  var serverId = data[rowIndex].id;
	  var $lis = [];
	
	  function onClick(i) {
	    var login = logins[i];
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
	
	  for (var i = 0; i < logins.length; i++) {
	    $lis.push(React.createElement(
	      'li',
	      { key: i },
	      React.createElement(
	        'a',
	        { onClick: onClick(i) },
	        logins[i]
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
	        logins[0]
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
	
	var NodeList = React.createClass({
	  displayName: 'NodeList',
	
	  render: function render() {
	    var data = this.props.nodeRecords;
	    var logins = this.props.logins;
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
	                cell: React.createElement(LoginCell, { data: data, logins: logins })
	              })
	            )
	          )
	        )
	      )
	    );
	  }
	});
	
	module.exports = NodeList;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "nodeList.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 220:
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

/***/ 221:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(274);
	
	var getters = _require.getters;
	
	var _require2 = __webpack_require__(58);
	
	var closeSelectNodeDialog = _require2.closeSelectNodeDialog;
	
	var _require3 = __webpack_require__(43);
	
	var changeServer = _require3.changeServer;
	
	var NodeList = __webpack_require__(219);
	var activeSessionGetters = __webpack_require__(57);
	var nodeGetters = __webpack_require__(59);
	
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
	    if (SelectNodeDialog.onServerChangeCallBack) {
	      SelectNodeDialog.onServerChangeCallBack({ serverId: serverId });
	    }
	
	    closeSelectNodeDialog();
	  },
	
	  componentWillUnmount: function componentWillUnmount(callback) {
	    $('.modal').modal('hide');
	  },
	
	  componentDidMount: function componentDidMount() {
	    $('.modal').modal('show');
	  },
	
	  render: function render() {
	    var activeSession = reactor.evaluate(activeSessionGetters.activeSession) || {};
	    var nodeRecords = reactor.evaluate(nodeGetters.nodeListView);
	    var logins = [activeSession.login];
	
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
	            React.createElement(NodeList, { nodeRecords: nodeRecords, logins: logins, onLoginClick: this.onLoginClick })
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
	
	SelectNodeDialog.onServerChangeCallBack = function () {};
	
	module.exports = SelectNodeDialog;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "selectNodeDialog.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 222:
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
	
	var GrvSortHeaderCell = React.createClass({
	  displayName: 'GrvSortHeaderCell',
	
	  getInitialState: function getInitialState() {
	    this._onSortChange = this._onSortChange.bind(this);
	  },
	
	  render: function render() {
	    var _props = this.props;
	    var sortDir = _props.sortDir;
	    var children = _props.children;
	
	    var props = _objectWithoutProperties(_props, ['sortDir', 'children']);
	
	    return React.createElement(
	      Cell,
	      props,
	      React.createElement(
	        'a',
	        { onClick: this._onSortChange },
	        children,
	        ' ',
	        sortDir ? sortDir === SortTypes.DESC ? '↓' : '↑' : ''
	      )
	    );
	  },
	
	  _onSortChange: function _onSortChange(e) {
	    e.preventDefault();
	
	    if (this.props.onSortChange) {
	      this.props.onSortChange(this.props.columnKey, this.props.sortDir ? reverseSortDirection(this.props.sortDir) : SortTypes.DESC);
	    }
	  }
	});
	
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
	exports.SortHeaderCell = GrvSortHeaderCell;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "table.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 223:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var Term = __webpack_require__(405);
	var React = __webpack_require__(4);
	
	var _require = __webpack_require__(406);
	
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

/***/ 268:
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

/***/ 269:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError('Cannot call a class as a function'); } }
	
	function _inherits(subClass, superClass) { if (typeof superClass !== 'function' && superClass !== null) { throw new TypeError('Super expression must either be null or a function, not ' + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }
	
	var Tty = __webpack_require__(92);
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
	
	    // start from the beginning if at the end
	    if (this.current === this.length) {
	      this.current = 1;
	    }
	
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

/***/ 270:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(60);
	
	var fetchSessions = _require.fetchSessions;
	
	var _require2 = __webpack_require__(102);
	
	var fetchNodes = _require2.fetchNodes;
	
	var $ = __webpack_require__(42);
	
	var _require3 = __webpack_require__(95);
	
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

/***/ 271:
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

/***/ 272:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(271);
	module.exports.actions = __webpack_require__(270);
	module.exports.appStore = __webpack_require__(96);

/***/ },

/***/ 273:
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

/***/ 274:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(273);
	module.exports.actions = __webpack_require__(58);
	module.exports.dialogStore = __webpack_require__(98);

/***/ },

/***/ 275:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var reactor = __webpack_require__(7);
	reactor.registerStores({
	  'tlpt': __webpack_require__(96),
	  'tlpt_dialogs': __webpack_require__(98),
	  'tlpt_active_terminal': __webpack_require__(94),
	  'tlpt_user': __webpack_require__(112),
	  'tlpt_nodes': __webpack_require__(103),
	  'tlpt_invite': __webpack_require__(100),
	  'tlpt_rest_api': __webpack_require__(281),
	  'tlpt_sessions': __webpack_require__(108)
	});

/***/ },

/***/ 276:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(99);
	
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

/***/ 277:
/***/ function(module, exports, __webpack_require__) {

	/*eslint no-undef: 0,  no-unused-vars: 0, no-debugger:0*/
	
	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(105);
	
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

/***/ 278:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(277);
	module.exports.actions = __webpack_require__(276);
	module.exports.nodeStore = __webpack_require__(100);

/***/ },

/***/ 279:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(59);
	module.exports.actions = __webpack_require__(102);
	module.exports.nodeStore = __webpack_require__(103);
	
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

/***/ 280:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(104);
	
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

/***/ 281:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(104);
	
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

/***/ 282:
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

/***/ 283:
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

/***/ 294:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	var NavLeftBar = __webpack_require__(301);
	var cfg = __webpack_require__(11);
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(272);
	
	var actions = _require.actions;
	var getters = _require.getters;
	
	var SelectNodeDialog = __webpack_require__(221);
	
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

/***/ 295:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };
	
	var React = __webpack_require__(4);
	
	var _require = __webpack_require__(44);
	
	var getters = _require.getters;
	var actions = _require.actions;
	
	var Tty = __webpack_require__(92);
	var TtyTerminal = __webpack_require__(223);
	var EventStreamer = __webpack_require__(296);
	var SessionLeftPanel = __webpack_require__(217);
	
	var _require2 = __webpack_require__(58);
	
	var showSelectNodeDialog = _require2.showSelectNodeDialog;
	var closeSelectNodeDialog = _require2.closeSelectNodeDialog;
	
	var SelectNodeDialog = __webpack_require__(221);
	
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
	      return _this.setState(_extends({}, _this.state, { isConnected: true }));
	    });
	
	    var _props = this.props;
	    var serverId = _props.serverId;
	    var login = _props.login;
	
	    return { serverId: serverId, login: login, isConnected: false };
	  },
	
	  componentDidMount: function componentDidMount() {
	    // temporary hack
	    SelectNodeDialog.onServerChangeCallBack = this.componentWillReceiveProps.bind(this);
	  },
	
	  componentWillUnmount: function componentWillUnmount() {
	    SelectNodeDialog.onServerChangeCallBack = null;
	    this.tty.disconnect();
	  },
	
	  componentWillReceiveProps: function componentWillReceiveProps(nextProps) {
	    var serverId = nextProps.serverId;
	
	    if (serverId && serverId !== this.state.serverId) {
	      this.tty.reconnect({ serverId: serverId });
	      this.refs.ttyCmntInstance.term.focus();
	      this.setState(_extends({}, this.state, { serverId: serverId }));
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

/***/ 296:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	exports.__esModule = true;
	var cfg = __webpack_require__(11);
	var React = __webpack_require__(4);
	var session = __webpack_require__(26);
	
	var _require = __webpack_require__(60);
	
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

/***/ 297:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	
	var _require = __webpack_require__(44);
	
	var getters = _require.getters;
	var actions = _require.actions;
	
	var NotFoundPage = __webpack_require__(220);
	var SessionPlayer = __webpack_require__(298);
	var ActiveSession = __webpack_require__(295);
	
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

/***/ 298:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	exports.__esModule = true;
	var React = __webpack_require__(4);
	var ReactSlider = __webpack_require__(231);
	var TtyPlayer = __webpack_require__(269);
	var TtyTerminal = __webpack_require__(223);
	var SessionLeftPanel = __webpack_require__(217);
	
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

/***/ 299:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	module.exports.App = __webpack_require__(294);
	module.exports.Login = __webpack_require__(300);
	module.exports.NewUser = __webpack_require__(302);
	module.exports.Nodes = __webpack_require__(303);
	module.exports.Sessions = __webpack_require__(304);
	module.exports.CurrentSessionHost = __webpack_require__(297);
	module.exports.NotFoundPage = __webpack_require__(220);
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "index.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 300:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	var $ = __webpack_require__(42);
	var reactor = __webpack_require__(7);
	var LinkedStateMixin = __webpack_require__(65);
	
	var _require = __webpack_require__(111);
	
	var actions = _require.actions;
	
	var GoogleAuthInfo = __webpack_require__(218);
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

/***/ 301:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	
	var _require = __webpack_require__(39);
	
	var Router = _require.Router;
	var IndexLink = _require.IndexLink;
	var History = _require.History;
	
	var getters = __webpack_require__(45);
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

/***/ 302:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	var $ = __webpack_require__(42);
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(278);
	
	var actions = _require.actions;
	var getters = _require.getters;
	
	var userModule = __webpack_require__(111);
	var LinkedStateMixin = __webpack_require__(65);
	var GoogleAuthInfo = __webpack_require__(218);
	
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

/***/ 303:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	var reactor = __webpack_require__(7);
	var userGetters = __webpack_require__(45);
	var nodeGetters = __webpack_require__(59);
	var NodeList = __webpack_require__(219);
	
	var Nodes = React.createClass({
	  displayName: 'Nodes',
	
	  mixins: [reactor.ReactMixin],
	
	  getDataBindings: function getDataBindings() {
	    return {
	      nodeRecords: nodeGetters.nodeListView,
	      user: userGetters.user
	    };
	  },
	
	  render: function render() {
	    var nodeRecords = this.state.nodeRecords;
	    var logins = this.state.user.logins;
	    return React.createElement(NodeList, { nodeRecords: nodeRecords, logins: logins });
	  }
	});
	
	module.exports = Nodes;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "main.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 304:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }
	
	var React = __webpack_require__(4);
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(39);
	
	var Link = _require.Link;
	
	var _require2 = __webpack_require__(222);
	
	var Table = _require2.Table;
	var Column = _require2.Column;
	var Cell = _require2.Cell;
	var TextCell = _require2.TextCell;
	
	var _require3 = __webpack_require__(107);
	
	var getters = _require3.getters;
	
	var _require4 = __webpack_require__(43);
	
	var open = _require4.open;
	
	var moment = __webpack_require__(1);
	var PureRenderMixin = __webpack_require__(215);
	
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
	
	  mixins: [reactor.ReactMixin, PureRenderMixin],
	
	  componentWillReceiveProps: function componentWillReceiveProps(nextProps) {
	    //this.setState({ });
	  },
	
	  getInitialState: function getInitialState() {
	    debugger;
	    /*  this._dataList = new FakeObjectDataListStore(2000);
	      this._defaultSortIndexes = [];
	       var size = this._dataList.getSize();
	      for (var index = 0; index < size; index++) {
	        this._defaultSortIndexes.push(index);
	      }
	       this.state = {
	        sortedDataList: this._dataList,
	        colSortDirs: {}
	      };
	       this._onSortChange = this._onSortChange.bind(this);
	    */
	  },
	
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

/***/ 305:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	var render = __webpack_require__(216).render;
	
	var _require = __webpack_require__(39);
	
	var Router = _require.Router;
	var Route = _require.Route;
	var Redirect = _require.Redirect;
	var IndexRoute = _require.IndexRoute;
	var browserHistory = _require.browserHistory;
	
	var _require2 = __webpack_require__(299);
	
	var App = _require2.App;
	var Login = _require2.Login;
	var Nodes = _require2.Nodes;
	var Sessions = _require2.Sessions;
	var NewUser = _require2.NewUser;
	var CurrentSessionHost = _require2.CurrentSessionHost;
	var NotFoundPage = _require2.NotFoundPage;
	
	var _require3 = __webpack_require__(110);
	
	var ensureUser = _require3.ensureUser;
	
	var auth = __webpack_require__(91);
	var session = __webpack_require__(26);
	var cfg = __webpack_require__(11);
	
	__webpack_require__(275);
	
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

/***/ 405:
/***/ function(module, exports) {

	module.exports = Terminal;

/***/ },

/***/ 406:
/***/ function(module, exports) {

	module.exports = _;

/***/ }

});
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJzb3VyY2VzIjpbIndlYnBhY2s6Ly8vLi9zcmMvYXBwL3JlYWN0b3IuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb25maWcuanMiLCJ3ZWJwYWNrOi8vLy4vfi9rZXltaXJyb3IvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9zZXNzaW9uLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvc2VydmljZXMvYXBpLmpzIiwid2VicGFjazovLy9leHRlcm5hbCBcImpRdWVyeVwiIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvdXNlci9nZXR0ZXJzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9nZXR0ZXJzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9kaWFsb2dzL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9hdXRoLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tbW9uL3R0eS5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvYWN0aW9uVHlwZXMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGl2ZVRlcm1TdG9yZS5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvYXBwL2FjdGlvblR5cGVzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hcHAvYXBwU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvYWN0aW9uVHlwZXMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvZGlhbG9nU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2ludml0ZVN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvbm9kZVN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2FjdGlvblR5cGVzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2NvbnN0YW50cy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvYWN0aW9uVHlwZXMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9zZXNzaW9ucy9zZXNzaW9uU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvYWN0aW9uVHlwZXMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvdXNlci9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvdXNlci91c2VyU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2N1cnJlbnRTZXNzaW9uL3Nlc3Npb25MZWZ0UGFuZWwuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9nb29nbGVBdXRoTG9nby5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL25vZGVzL25vZGVMaXN0LmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvbm90Rm91bmRQYWdlLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvc2VsZWN0Tm9kZURpYWxvZy5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL3RhYmxlLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvdGVybWluYWwuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tbW9uL3BhdHRlcm5VdGlscy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbW1vbi90dHlQbGF5ZXIuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FwcC9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hcHAvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvYXBwL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9kaWFsb2dzL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9yZXN0QXBpU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC91dGlscy5qcyIsIndlYnBhY2s6Ly8vLi9+L2V2ZW50cy9ldmVudHMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2FwcC5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2N1cnJlbnRTZXNzaW9uL2FjdGl2ZVNlc3Npb24uanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9jdXJyZW50U2Vzc2lvbi9ldmVudFN0cmVhbWVyLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvY3VycmVudFNlc3Npb24vbWFpbi5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2N1cnJlbnRTZXNzaW9uL3Nlc3Npb25QbGF5ZXIuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9pbmRleC5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2xvZ2luLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvbmF2TGVmdEJhci5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL25ld1VzZXIuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9ub2Rlcy9tYWluLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvc2Vzc2lvbnMvbWFpbi5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9pbmRleC5qc3giLCJ3ZWJwYWNrOi8vL2V4dGVybmFsIFwiVGVybWluYWxcIiIsIndlYnBhY2s6Ly8vZXh0ZXJuYWwgXCJfXCIiXSwibmFtZXMiOltdLCJtYXBwaW5ncyI6Ijs7Ozs7Ozs7Ozs7Ozs7Ozs7c0NBQXdCLEVBQVk7O0FBRXBD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LEVBQUMsU0FBUyxFQUFFLEtBQU0sR0FBRTtLQUNsRCxvQkFBQyxLQUFLLElBQUMsSUFBSSxFQUFFLEdBQUcsQ0FBQyxNQUFNLENBQUMsYUFBYyxFQUFDLFVBQVUsRUFBRSxFQUFDLGtCQUFrQixFQUFFLGtCQUFrQixFQUFFLEdBQUU7S0FDOUYsb0JBQUMsS0FBSyxJQUFDLElBQUksRUFBRSxHQUFHLENBQUMsTUFBTSxDQUFDLFFBQVMsRUFBQyxTQUFTLEVBQUUsUUFBUyxHQUFFO0lBQ2xEO0dBQ1Isb0JBQUMsS0FBSyxJQUFDLElBQUksRUFBQyxHQUFHLEVBQUMsU0FBUyxFQUFFLFlBQWEsR0FBRztFQUNwQyxFQUNSLFFBQVEsQ0FBQyxjQUFjLENBQUMsS0FBSyxDQUFDLENBQUMsQzs7Ozs7Ozs7O0FDL0JsQywyQjs7Ozs7OztBQ0FBLG9CIiwiZmlsZSI6ImFwcC5qcyIsInNvdXJjZXNDb250ZW50IjpbImltcG9ydCB7IFJlYWN0b3IgfSBmcm9tICdudWNsZWFyLWpzJ1xuXG5jb25zdCByZWFjdG9yID0gbmV3IFJlYWN0b3Ioe1xuICBkZWJ1ZzogdHJ1ZVxufSlcblxud2luZG93LnJlYWN0b3IgPSByZWFjdG9yO1xuXG5leHBvcnQgZGVmYXVsdCByZWFjdG9yXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvcmVhY3Rvci5qc1xuICoqLyIsImxldCB7Zm9ybWF0UGF0dGVybn0gPSByZXF1aXJlKCdhcHAvY29tbW9uL3BhdHRlcm5VdGlscycpO1xuXG5sZXQgY2ZnID0ge1xuXG4gIGJhc2VVcmw6IHdpbmRvdy5sb2NhdGlvbi5vcmlnaW4sXG5cbiAgaGVscFVybDogJ2h0dHBzOi8vZ2l0aHViLmNvbS9ncmF2aXRhdGlvbmFsL3RlbGVwb3J0L2Jsb2IvbWFzdGVyL1JFQURNRS5tZCcsXG5cbiAgYXBpOiB7XG4gICAgcmVuZXdUb2tlblBhdGg6Jy92MS93ZWJhcGkvc2Vzc2lvbnMvcmVuZXcnLFxuICAgIG5vZGVzUGF0aDogJy92MS93ZWJhcGkvc2l0ZXMvLWN1cnJlbnQtL25vZGVzJyxcbiAgICBzZXNzaW9uUGF0aDogJy92MS93ZWJhcGkvc2Vzc2lvbnMnLFxuICAgIHNpdGVTZXNzaW9uUGF0aDogJy92MS93ZWJhcGkvc2l0ZXMvLWN1cnJlbnQtL3Nlc3Npb25zLzpzaWQnLFxuICAgIGludml0ZVBhdGg6ICcvdjEvd2ViYXBpL3VzZXJzL2ludml0ZXMvOmludml0ZVRva2VuJyxcbiAgICBjcmVhdGVVc2VyUGF0aDogJy92MS93ZWJhcGkvdXNlcnMnLFxuICAgIHNlc3Npb25DaHVuazogJy92MS93ZWJhcGkvc2l0ZXMvLWN1cnJlbnQtL3Nlc3Npb25zLzpzaWQvY2h1bmtzP3N0YXJ0PTpzdGFydCZlbmQ9OmVuZCcsXG4gICAgc2Vzc2lvbkNodW5rQ291bnRQYXRoOiAnL3YxL3dlYmFwaS9zaXRlcy8tY3VycmVudC0vc2Vzc2lvbnMvOnNpZC9jaHVua3Njb3VudCcsXG5cbiAgICBnZXRGZXRjaFNlc3Npb25DaHVua1VybDogKHtzaWQsIHN0YXJ0LCBlbmR9KT0+e1xuICAgICAgcmV0dXJuIGZvcm1hdFBhdHRlcm4oY2ZnLmFwaS5zZXNzaW9uQ2h1bmssIHtzaWQsIHN0YXJ0LCBlbmR9KTtcbiAgICB9LFxuXG4gICAgZ2V0RmV0Y2hTZXNzaW9uTGVuZ3RoVXJsOiAoc2lkKT0+e1xuICAgICAgcmV0dXJuIGZvcm1hdFBhdHRlcm4oY2ZnLmFwaS5zZXNzaW9uQ2h1bmtDb3VudFBhdGgsIHtzaWR9KTtcbiAgICB9LFxuXG4gICAgZ2V0RmV0Y2hTZXNzaW9uc1VybDogKC8qc3RhcnQsIGVuZCovKT0+e1xuICAgICAgdmFyIHBhcmFtcyA9IHtcbiAgICAgICAgc3RhcnQ6IG5ldyBEYXRlKCkudG9JU09TdHJpbmcoKSxcbiAgICAgICAgb3JkZXI6IC0xXG4gICAgICB9O1xuXG4gICAgICB2YXIganNvbiA9IEpTT04uc3RyaW5naWZ5KHBhcmFtcyk7XG4gICAgICB2YXIganNvbkVuY29kZWQgPSB3aW5kb3cuZW5jb2RlVVJJKGpzb24pO1xuXG4gICAgICByZXR1cm4gYC92MS93ZWJhcGkvc2l0ZXMvLWN1cnJlbnQtL2V2ZW50cy9zZXNzaW9ucz9maWx0ZXI9JHtqc29uRW5jb2RlZH1gO1xuICAgIH0sXG5cbiAgICBnZXRGZXRjaFNlc3Npb25Vcmw6IChzaWQpPT57XG4gICAgICByZXR1cm4gZm9ybWF0UGF0dGVybihjZmcuYXBpLnNpdGVTZXNzaW9uUGF0aCwge3NpZH0pO1xuICAgIH0sXG5cbiAgICBnZXRUZXJtaW5hbFNlc3Npb25Vcmw6IChzaWQpPT4ge1xuICAgICAgcmV0dXJuIGZvcm1hdFBhdHRlcm4oY2ZnLmFwaS5zaXRlU2Vzc2lvblBhdGgsIHtzaWR9KTtcbiAgICB9LFxuXG4gICAgZ2V0SW52aXRlVXJsOiAoaW52aXRlVG9rZW4pID0+IHtcbiAgICAgIHJldHVybiBmb3JtYXRQYXR0ZXJuKGNmZy5hcGkuaW52aXRlUGF0aCwge2ludml0ZVRva2VufSk7XG4gICAgfSxcblxuICAgIGdldEV2ZW50U3RyZWFtQ29ublN0cjogKHRva2VuLCBzaWQpID0+IHtcbiAgICAgIHZhciBob3N0bmFtZSA9IGdldFdzSG9zdE5hbWUoKTtcbiAgICAgIHJldHVybiBgJHtob3N0bmFtZX0vdjEvd2ViYXBpL3NpdGVzLy1jdXJyZW50LS9zZXNzaW9ucy8ke3NpZH0vZXZlbnRzL3N0cmVhbT9hY2Nlc3NfdG9rZW49JHt0b2tlbn1gO1xuICAgIH0sXG5cbiAgICBnZXRUdHlDb25uU3RyOiAoe3Rva2VuLCBzZXJ2ZXJJZCwgbG9naW4sIHNpZCwgcm93cywgY29sc30pID0+IHtcbiAgICAgIHZhciBwYXJhbXMgPSB7XG4gICAgICAgIHNlcnZlcl9pZDogc2VydmVySWQsXG4gICAgICAgIGxvZ2luLFxuICAgICAgICBzaWQsXG4gICAgICAgIHRlcm06IHtcbiAgICAgICAgICBoOiByb3dzLFxuICAgICAgICAgIHc6IGNvbHNcbiAgICAgICAgfVxuICAgICAgfVxuXG4gICAgICB2YXIganNvbiA9IEpTT04uc3RyaW5naWZ5KHBhcmFtcyk7XG4gICAgICB2YXIganNvbkVuY29kZWQgPSB3aW5kb3cuZW5jb2RlVVJJKGpzb24pO1xuICAgICAgdmFyIGhvc3RuYW1lID0gZ2V0V3NIb3N0TmFtZSgpO1xuICAgICAgcmV0dXJuIGAke2hvc3RuYW1lfS92MS93ZWJhcGkvc2l0ZXMvLWN1cnJlbnQtL2Nvbm5lY3Q/YWNjZXNzX3Rva2VuPSR7dG9rZW59JnBhcmFtcz0ke2pzb25FbmNvZGVkfWA7XG4gICAgfVxuICB9LFxuXG4gIHJvdXRlczoge1xuICAgIGFwcDogJy93ZWInLFxuICAgIGxvZ291dDogJy93ZWIvbG9nb3V0JyxcbiAgICBsb2dpbjogJy93ZWIvbG9naW4nLFxuICAgIG5vZGVzOiAnL3dlYi9ub2RlcycsXG4gICAgYWN0aXZlU2Vzc2lvbjogJy93ZWIvc2Vzc2lvbnMvOnNpZCcsXG4gICAgbmV3VXNlcjogJy93ZWIvbmV3dXNlci86aW52aXRlVG9rZW4nLFxuICAgIHNlc3Npb25zOiAnL3dlYi9zZXNzaW9ucycsXG4gICAgcGFnZU5vdEZvdW5kOiAnL3dlYi9ub3Rmb3VuZCdcbiAgfSxcblxuICBnZXRBY3RpdmVTZXNzaW9uUm91dGVVcmwoc2lkKXtcbiAgICByZXR1cm4gZm9ybWF0UGF0dGVybihjZmcucm91dGVzLmFjdGl2ZVNlc3Npb24sIHtzaWR9KTtcbiAgfVxufVxuXG5leHBvcnQgZGVmYXVsdCBjZmc7XG5cbmZ1bmN0aW9uIGdldFdzSG9zdE5hbWUoKXtcbiAgdmFyIHByZWZpeCA9IGxvY2F0aW9uLnByb3RvY29sID09IFwiaHR0cHM6XCI/XCJ3c3M6Ly9cIjpcIndzOi8vXCI7XG4gIHZhciBob3N0cG9ydCA9IGxvY2F0aW9uLmhvc3RuYW1lKyhsb2NhdGlvbi5wb3J0ID8gJzonK2xvY2F0aW9uLnBvcnQ6ICcnKTtcbiAgcmV0dXJuIGAke3ByZWZpeH0ke2hvc3Rwb3J0fWA7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29uZmlnLmpzXG4gKiovIiwiLyoqXG4gKiBDb3B5cmlnaHQgMjAxMy0yMDE0IEZhY2Vib29rLCBJbmMuXG4gKlxuICogTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbiAqIHlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbiAqIFlvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuICpcbiAqIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuICpcbiAqIFVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbiAqIGRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbiAqIFdJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuICogU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxuICogbGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4gKlxuICovXG5cblwidXNlIHN0cmljdFwiO1xuXG4vKipcbiAqIENvbnN0cnVjdHMgYW4gZW51bWVyYXRpb24gd2l0aCBrZXlzIGVxdWFsIHRvIHRoZWlyIHZhbHVlLlxuICpcbiAqIEZvciBleGFtcGxlOlxuICpcbiAqICAgdmFyIENPTE9SUyA9IGtleU1pcnJvcih7Ymx1ZTogbnVsbCwgcmVkOiBudWxsfSk7XG4gKiAgIHZhciBteUNvbG9yID0gQ09MT1JTLmJsdWU7XG4gKiAgIHZhciBpc0NvbG9yVmFsaWQgPSAhIUNPTE9SU1tteUNvbG9yXTtcbiAqXG4gKiBUaGUgbGFzdCBsaW5lIGNvdWxkIG5vdCBiZSBwZXJmb3JtZWQgaWYgdGhlIHZhbHVlcyBvZiB0aGUgZ2VuZXJhdGVkIGVudW0gd2VyZVxuICogbm90IGVxdWFsIHRvIHRoZWlyIGtleXMuXG4gKlxuICogICBJbnB1dDogIHtrZXkxOiB2YWwxLCBrZXkyOiB2YWwyfVxuICogICBPdXRwdXQ6IHtrZXkxOiBrZXkxLCBrZXkyOiBrZXkyfVxuICpcbiAqIEBwYXJhbSB7b2JqZWN0fSBvYmpcbiAqIEByZXR1cm4ge29iamVjdH1cbiAqL1xudmFyIGtleU1pcnJvciA9IGZ1bmN0aW9uKG9iaikge1xuICB2YXIgcmV0ID0ge307XG4gIHZhciBrZXk7XG4gIGlmICghKG9iaiBpbnN0YW5jZW9mIE9iamVjdCAmJiAhQXJyYXkuaXNBcnJheShvYmopKSkge1xuICAgIHRocm93IG5ldyBFcnJvcigna2V5TWlycm9yKC4uLik6IEFyZ3VtZW50IG11c3QgYmUgYW4gb2JqZWN0LicpO1xuICB9XG4gIGZvciAoa2V5IGluIG9iaikge1xuICAgIGlmICghb2JqLmhhc093blByb3BlcnR5KGtleSkpIHtcbiAgICAgIGNvbnRpbnVlO1xuICAgIH1cbiAgICByZXRba2V5XSA9IGtleTtcbiAgfVxuICByZXR1cm4gcmV0O1xufTtcblxubW9kdWxlLmV4cG9ydHMgPSBrZXlNaXJyb3I7XG5cblxuXG4vKioqKioqKioqKioqKioqKipcbiAqKiBXRUJQQUNLIEZPT1RFUlxuICoqIC4vfi9rZXltaXJyb3IvaW5kZXguanNcbiAqKiBtb2R1bGUgaWQgPSAyMFxuICoqIG1vZHVsZSBjaHVua3MgPSAxXG4gKiovIiwidmFyIHsgYnJvd3Nlckhpc3RvcnksIGNyZWF0ZU1lbW9yeUhpc3RvcnkgfSA9IHJlcXVpcmUoJ3JlYWN0LXJvdXRlcicpO1xuXG5jb25zdCBBVVRIX0tFWV9EQVRBID0gJ2F1dGhEYXRhJztcblxudmFyIF9oaXN0b3J5ID0gY3JlYXRlTWVtb3J5SGlzdG9yeSgpO1xuXG52YXIgc2Vzc2lvbiA9IHtcblxuICBpbml0KGhpc3Rvcnk9YnJvd3Nlckhpc3Rvcnkpe1xuICAgIF9oaXN0b3J5ID0gaGlzdG9yeTtcbiAgfSxcblxuICBnZXRIaXN0b3J5KCl7XG4gICAgcmV0dXJuIF9oaXN0b3J5O1xuICB9LFxuXG4gIHNldFVzZXJEYXRhKHVzZXJEYXRhKXtcbiAgICBsb2NhbFN0b3JhZ2Uuc2V0SXRlbShBVVRIX0tFWV9EQVRBLCBKU09OLnN0cmluZ2lmeSh1c2VyRGF0YSkpO1xuICB9LFxuXG4gIGdldFVzZXJEYXRhKCl7XG4gICAgdmFyIGl0ZW0gPSBsb2NhbFN0b3JhZ2UuZ2V0SXRlbShBVVRIX0tFWV9EQVRBKTtcbiAgICBpZihpdGVtKXtcbiAgICAgIHJldHVybiBKU09OLnBhcnNlKGl0ZW0pO1xuICAgIH1cblxuICAgIHJldHVybiB7fTtcbiAgfSxcblxuICBjbGVhcigpe1xuICAgIGxvY2FsU3RvcmFnZS5jbGVhcigpXG4gIH1cblxufVxuXG5tb2R1bGUuZXhwb3J0cyA9IHNlc3Npb247XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvc2Vzc2lvbi5qc1xuICoqLyIsInZhciAkID0gcmVxdWlyZShcImpRdWVyeVwiKTtcbnZhciBzZXNzaW9uID0gcmVxdWlyZSgnYXBwL3Nlc3Npb24nKTtcblxuY29uc3QgYXBpID0ge1xuXG4gIHB1dChwYXRoLCBkYXRhLCB3aXRoVG9rZW4pe1xuICAgIHJldHVybiBhcGkuYWpheCh7dXJsOiBwYXRoLCBkYXRhOiBKU09OLnN0cmluZ2lmeShkYXRhKSwgdHlwZTogJ1BVVCd9LCB3aXRoVG9rZW4pO1xuICB9LFxuXG4gIHBvc3QocGF0aCwgZGF0YSwgd2l0aFRva2VuKXtcbiAgICByZXR1cm4gYXBpLmFqYXgoe3VybDogcGF0aCwgZGF0YTogSlNPTi5zdHJpbmdpZnkoZGF0YSksIHR5cGU6ICdQT1NUJ30sIHdpdGhUb2tlbik7XG4gIH0sXG5cbiAgZ2V0KHBhdGgpe1xuICAgIHJldHVybiBhcGkuYWpheCh7dXJsOiBwYXRofSk7XG4gIH0sXG5cbiAgYWpheChjZmcsIHdpdGhUb2tlbiA9IHRydWUpe1xuICAgIHZhciBkZWZhdWx0Q2ZnID0ge1xuICAgICAgdHlwZTogXCJHRVRcIixcbiAgICAgIGRhdGFUeXBlOiBcImpzb25cIixcbiAgICAgIGJlZm9yZVNlbmQ6IGZ1bmN0aW9uKHhocikge1xuICAgICAgICBpZih3aXRoVG9rZW4pe1xuICAgICAgICAgIHZhciB7IHRva2VuIH0gPSBzZXNzaW9uLmdldFVzZXJEYXRhKCk7XG4gICAgICAgICAgeGhyLnNldFJlcXVlc3RIZWFkZXIoJ0F1dGhvcml6YXRpb24nLCdCZWFyZXIgJyArIHRva2VuKTtcbiAgICAgICAgfVxuICAgICAgIH1cbiAgICB9XG5cbiAgICByZXR1cm4gJC5hamF4KCQuZXh0ZW5kKHt9LCBkZWZhdWx0Q2ZnLCBjZmcpKTtcbiAgfVxufVxuXG5tb2R1bGUuZXhwb3J0cyA9IGFwaTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9zZXJ2aWNlcy9hcGkuanNcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cyA9IGpRdWVyeTtcblxuXG4vKioqKioqKioqKioqKioqKipcbiAqKiBXRUJQQUNLIEZPT1RFUlxuICoqIGV4dGVybmFsIFwialF1ZXJ5XCJcbiAqKiBtb2R1bGUgaWQgPSA0MlxuICoqIG1vZHVsZSBjaHVua3MgPSAxXG4gKiovIiwidmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCdhcHAvc2Vzc2lvbicpO1xudmFyIHt1dWlkfSA9IHJlcXVpcmUoJ2FwcC91dGlscycpO1xudmFyIGFwaSA9IHJlcXVpcmUoJ2FwcC9zZXJ2aWNlcy9hcGknKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG52YXIgZ2V0dGVycyA9IHJlcXVpcmUoJy4vZ2V0dGVycycpO1xudmFyIHNlc3Npb25Nb2R1bGUgPSByZXF1aXJlKCcuLy4uL3Nlc3Npb25zJyk7XG5cbnZhciB7IFRMUFRfVEVSTV9PUEVOLCBUTFBUX1RFUk1fQ0xPU0UsIFRMUFRfVEVSTV9DSEFOR0VfU0VSVkVSIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbnZhciBhY3Rpb25zID0ge1xuXG4gIGNoYW5nZVNlcnZlcihzZXJ2ZXJJZCwgbG9naW4pe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9URVJNX0NIQU5HRV9TRVJWRVIsIHtcbiAgICAgIHNlcnZlcklkLFxuICAgICAgbG9naW5cbiAgICB9KTtcbiAgfSxcblxuICBjbG9zZSgpe1xuICAgIGxldCB7aXNOZXdTZXNzaW9ufSA9IHJlYWN0b3IuZXZhbHVhdGUoZ2V0dGVycy5hY3RpdmVTZXNzaW9uKTtcblxuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9URVJNX0NMT1NFKTtcblxuICAgIGlmKGlzTmV3U2Vzc2lvbil7XG4gICAgICBzZXNzaW9uLmdldEhpc3RvcnkoKS5wdXNoKGNmZy5yb3V0ZXMubm9kZXMpO1xuICAgIH1lbHNle1xuICAgICAgc2Vzc2lvbi5nZXRIaXN0b3J5KCkucHVzaChjZmcucm91dGVzLnNlc3Npb25zKTtcbiAgICB9XG4gIH0sXG5cbiAgcmVzaXplKHcsIGgpe1xuICAgIC8vIHNvbWUgbWluIHZhbHVlc1xuICAgIHcgPSB3IDwgNSA/IDUgOiB3O1xuICAgIGggPSBoIDwgNSA/IDUgOiBoO1xuXG4gICAgbGV0IHJlcURhdGEgPSB7IHRlcm1pbmFsX3BhcmFtczogeyB3LCBoIH0gfTtcbiAgICBsZXQge3NpZH0gPSByZWFjdG9yLmV2YWx1YXRlKGdldHRlcnMuYWN0aXZlU2Vzc2lvbik7XG5cbiAgICBhcGkucHV0KGNmZy5hcGkuZ2V0VGVybWluYWxTZXNzaW9uVXJsKHNpZCksIHJlcURhdGEpXG4gICAgICAuZG9uZSgoKT0+e1xuICAgICAgICBjb25zb2xlLmxvZyhgcmVzaXplIHdpdGggdzoke3d9IGFuZCBoOiR7aH0gLSBPS2ApO1xuICAgICAgfSlcbiAgICAgIC5mYWlsKCgpPT57XG4gICAgICAgIGNvbnNvbGUubG9nKGBmYWlsZWQgdG8gcmVzaXplIHdpdGggdzoke3d9IGFuZCBoOiR7aH1gKTtcbiAgICB9KVxuICB9LFxuXG4gIG9wZW5TZXNzaW9uKHNpZCl7XG4gICAgc2Vzc2lvbk1vZHVsZS5hY3Rpb25zLmZldGNoU2Vzc2lvbihzaWQpXG4gICAgICAuZG9uZSgoKT0+e1xuICAgICAgICBsZXQgc1ZpZXcgPSByZWFjdG9yLmV2YWx1YXRlKHNlc3Npb25Nb2R1bGUuZ2V0dGVycy5zZXNzaW9uVmlld0J5SWQoc2lkKSk7XG4gICAgICAgIGxldCB7IHNlcnZlcklkLCBsb2dpbiB9ID0gc1ZpZXc7XG4gICAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9URVJNX09QRU4sIHtcbiAgICAgICAgICAgIHNlcnZlcklkLFxuICAgICAgICAgICAgbG9naW4sXG4gICAgICAgICAgICBzaWQsXG4gICAgICAgICAgICBpc05ld1Nlc3Npb246IGZhbHNlXG4gICAgICAgICAgfSk7XG4gICAgICB9KVxuICAgICAgLmZhaWwoKCk9PntcbiAgICAgICAgc2Vzc2lvbi5nZXRIaXN0b3J5KCkucHVzaChjZmcucm91dGVzLnBhZ2VOb3RGb3VuZCk7XG4gICAgICB9KVxuICB9LFxuXG4gIGNyZWF0ZU5ld1Nlc3Npb24oc2VydmVySWQsIGxvZ2luKXtcbiAgICB2YXIgc2lkID0gdXVpZCgpO1xuICAgIHZhciByb3V0ZVVybCA9IGNmZy5nZXRBY3RpdmVTZXNzaW9uUm91dGVVcmwoc2lkKTtcbiAgICB2YXIgaGlzdG9yeSA9IHNlc3Npb24uZ2V0SGlzdG9yeSgpO1xuXG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1RFUk1fT1BFTiwge1xuICAgICAgc2VydmVySWQsXG4gICAgICBsb2dpbixcbiAgICAgIHNpZCxcbiAgICAgIGlzTmV3U2Vzc2lvbjogdHJ1ZVxuICAgIH0pO1xuXG4gICAgaGlzdG9yeS5wdXNoKHJvdXRlVXJsKTtcbiAgfVxuXG59XG5cbmV4cG9ydCBkZWZhdWx0IGFjdGlvbnM7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3Rpb25zLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMuZ2V0dGVycyA9IHJlcXVpcmUoJy4vZ2V0dGVycycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aW9ucyA9IHJlcXVpcmUoJy4vYWN0aW9ucycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aXZlVGVybVN0b3JlID0gcmVxdWlyZSgnLi9hY3RpdmVUZXJtU3RvcmUnKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2luZGV4LmpzXG4gKiovIiwiY29uc3QgdXNlciA9IFsgWyd0bHB0X3VzZXInXSwgKGN1cnJlbnRVc2VyKSA9PiB7XG4gICAgaWYoIWN1cnJlbnRVc2VyKXtcbiAgICAgIHJldHVybiBudWxsO1xuICAgIH1cblxuICAgIHZhciBuYW1lID0gY3VycmVudFVzZXIuZ2V0KCduYW1lJykgfHwgJyc7XG4gICAgdmFyIHNob3J0RGlzcGxheU5hbWUgPSBuYW1lWzBdIHx8ICcnO1xuXG4gICAgcmV0dXJuIHtcbiAgICAgIG5hbWUsXG4gICAgICBzaG9ydERpc3BsYXlOYW1lLFxuICAgICAgbG9naW5zOiBjdXJyZW50VXNlci5nZXQoJ2FsbG93ZWRfbG9naW5zJykudG9KUygpXG4gICAgfVxuICB9XG5dO1xuXG5leHBvcnQgZGVmYXVsdCB7XG4gIHVzZXJcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvZ2V0dGVycy5qc1xuICoqLyIsInZhciB7Y3JlYXRlVmlld30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9zZXNzaW9ucy9nZXR0ZXJzJyk7XG5cbmNvbnN0IGFjdGl2ZVNlc3Npb24gPSBbXG5bJ3RscHRfYWN0aXZlX3Rlcm1pbmFsJ10sIFsndGxwdF9zZXNzaW9ucyddLFxuKGFjdGl2ZVRlcm0sIHNlc3Npb25zKSA9PiB7XG4gICAgaWYoIWFjdGl2ZVRlcm0pe1xuICAgICAgcmV0dXJuIG51bGw7XG4gICAgfVxuXG4gICAgLypcbiAgICAqIGFjdGl2ZSBzZXNzaW9uIG5lZWRzIHRvIGhhdmUgaXRzIG93biB2aWV3IGFzIGFuIGFjdHVhbCBzZXNzaW9uIG1pZ2h0IG5vdFxuICAgICogZXhpc3QgYXQgdGhpcyBwb2ludC4gRm9yIGV4YW1wbGUsIHVwb24gY3JlYXRpbmcgYSBuZXcgc2Vzc2lvbiB3ZSBuZWVkIHRvIGtub3dcbiAgICAqIGxvZ2luIGFuZCBzZXJ2ZXJJZC4gSXQgd2lsbCBiZSBzaW1wbGlmaWVkIG9uY2Ugc2VydmVyIEFQSSBnZXRzIGV4dGVuZGVkLlxuICAgICovXG4gICAgbGV0IGFzVmlldyA9IHtcbiAgICAgIGlzTmV3U2Vzc2lvbjogYWN0aXZlVGVybS5nZXQoJ2lzTmV3U2Vzc2lvbicpLFxuICAgICAgbm90Rm91bmQ6IGFjdGl2ZVRlcm0uZ2V0KCdub3RGb3VuZCcpLFxuICAgICAgYWRkcjogYWN0aXZlVGVybS5nZXQoJ2FkZHInKSxcbiAgICAgIHNlcnZlcklkOiBhY3RpdmVUZXJtLmdldCgnc2VydmVySWQnKSxcbiAgICAgIHNlcnZlcklwOiB1bmRlZmluZWQsXG4gICAgICBsb2dpbjogYWN0aXZlVGVybS5nZXQoJ2xvZ2luJyksXG4gICAgICBzaWQ6IGFjdGl2ZVRlcm0uZ2V0KCdzaWQnKSxcbiAgICAgIGNvbHM6IHVuZGVmaW5lZCxcbiAgICAgIHJvd3M6IHVuZGVmaW5lZFxuICAgIH07XG5cbiAgICAvLyBpbiBjYXNlIGlmIHNlc3Npb24gYWxyZWFkeSBleGlzdHMsIGdldCB0aGUgZGF0YSBmcm9tIHRoZXJlXG4gICAgLy8gKGZvciBleGFtcGxlLCB3aGVuIGpvaW5pbmcgYW4gZXhpc3Rpbmcgc2Vzc2lvbilcbiAgICBpZihzZXNzaW9ucy5oYXMoYXNWaWV3LnNpZCkpe1xuICAgICAgbGV0IHNWaWV3ID0gY3JlYXRlVmlldyhzZXNzaW9ucy5nZXQoYXNWaWV3LnNpZCkpO1xuXG4gICAgICBhc1ZpZXcucGFydGllcyA9IHNWaWV3LnBhcnRpZXM7XG4gICAgICBhc1ZpZXcuc2VydmVySXAgPSBzVmlldy5zZXJ2ZXJJcDtcbiAgICAgIGFzVmlldy5zZXJ2ZXJJZCA9IHNWaWV3LnNlcnZlcklkO1xuICAgICAgYXNWaWV3LmFjdGl2ZSA9IHNWaWV3LmFjdGl2ZTtcbiAgICAgIGFzVmlldy5jb2xzID0gc1ZpZXcuY29scztcbiAgICAgIGFzVmlldy5yb3dzID0gc1ZpZXcucm93cztcbiAgICB9XG5cbiAgICByZXR1cm4gYXNWaWV3O1xuXG4gIH1cbl07XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgYWN0aXZlU2Vzc2lvblxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvZ2V0dGVycy5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7IFRMUFRfRElBTE9HX1NFTEVDVF9OT0RFX1NIT1csIFRMUFRfRElBTE9HX1NFTEVDVF9OT0RFX0NMT1NFIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbnZhciBhY3Rpb25zID0ge1xuICBzaG93U2VsZWN0Tm9kZURpYWxvZygpe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9ESUFMT0dfU0VMRUNUX05PREVfU0hPVyk7XG4gIH0sXG5cbiAgY2xvc2VTZWxlY3ROb2RlRGlhbG9nKCl7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX0RJQUxPR19TRUxFQ1RfTk9ERV9DTE9TRSk7XG4gIH1cbn1cblxuZXhwb3J0IGRlZmF1bHQgYWN0aW9ucztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvYWN0aW9ucy5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7c2Vzc2lvbnNCeVNlcnZlcn0gPSByZXF1aXJlKCcuLy4uL3Nlc3Npb25zL2dldHRlcnMnKTtcblxuY29uc3Qgbm9kZUxpc3RWaWV3ID0gWyBbJ3RscHRfbm9kZXMnXSwgKG5vZGVzKSA9PntcbiAgICByZXR1cm4gbm9kZXMubWFwKChpdGVtKT0+e1xuICAgICAgdmFyIHNlcnZlcklkID0gaXRlbS5nZXQoJ2lkJyk7XG4gICAgICB2YXIgc2Vzc2lvbnMgPSByZWFjdG9yLmV2YWx1YXRlKHNlc3Npb25zQnlTZXJ2ZXIoc2VydmVySWQpKTtcbiAgICAgIHJldHVybiB7XG4gICAgICAgIGlkOiBzZXJ2ZXJJZCxcbiAgICAgICAgaG9zdG5hbWU6IGl0ZW0uZ2V0KCdob3N0bmFtZScpLFxuICAgICAgICB0YWdzOiBnZXRUYWdzKGl0ZW0pLFxuICAgICAgICBhZGRyOiBpdGVtLmdldCgnYWRkcicpLFxuICAgICAgICBzZXNzaW9uQ291bnQ6IHNlc3Npb25zLnNpemVcbiAgICAgIH1cbiAgICB9KS50b0pTKCk7XG4gfVxuXTtcblxuZnVuY3Rpb24gZ2V0VGFncyhub2RlKXtcbiAgdmFyIGFsbExhYmVscyA9IFtdO1xuICB2YXIgbGFiZWxzID0gbm9kZS5nZXQoJ2xhYmVscycpO1xuXG4gIGlmKGxhYmVscyl7XG4gICAgbGFiZWxzLmVudHJ5U2VxKCkudG9BcnJheSgpLmZvckVhY2goaXRlbT0+e1xuICAgICAgYWxsTGFiZWxzLnB1c2goe1xuICAgICAgICByb2xlOiBpdGVtWzBdLFxuICAgICAgICB2YWx1ZTogaXRlbVsxXVxuICAgICAgfSk7XG4gICAgfSk7XG4gIH1cblxuICBsYWJlbHMgPSBub2RlLmdldCgnY21kX2xhYmVscycpO1xuXG4gIGlmKGxhYmVscyl7XG4gICAgbGFiZWxzLmVudHJ5U2VxKCkudG9BcnJheSgpLmZvckVhY2goaXRlbT0+e1xuICAgICAgYWxsTGFiZWxzLnB1c2goe1xuICAgICAgICByb2xlOiBpdGVtWzBdLFxuICAgICAgICB2YWx1ZTogaXRlbVsxXS5nZXQoJ3Jlc3VsdCcpLFxuICAgICAgICB0b29sdGlwOiBpdGVtWzFdLmdldCgnY29tbWFuZCcpXG4gICAgICB9KTtcbiAgICB9KTtcbiAgfVxuXG4gIHJldHVybiBhbGxMYWJlbHM7XG59XG5cblxuZXhwb3J0IGRlZmF1bHQge1xuICBub2RlTGlzdFZpZXdcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2dldHRlcnMuanNcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgYXBpID0gcmVxdWlyZSgnYXBwL3NlcnZpY2VzL2FwaScpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxudmFyIHsgVExQVF9TRVNTSU5TX1JFQ0VJVkUsIFRMUFRfU0VTU0lOU19VUERBVEUgfSAgPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IHtcblxuICBmZXRjaFNlc3Npb24oc2lkKXtcbiAgICByZXR1cm4gYXBpLmdldChjZmcuYXBpLmdldEZldGNoU2Vzc2lvblVybChzaWQpKS50aGVuKGpzb249PntcbiAgICAgIGlmKGpzb24gJiYganNvbi5zZXNzaW9uKXtcbiAgICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1NFU1NJTlNfVVBEQVRFLCBqc29uLnNlc3Npb24pO1xuICAgICAgfVxuICAgIH0pO1xuICB9LFxuXG4gIGZldGNoU2Vzc2lvbnMoKXtcbiAgICByZXR1cm4gYXBpLmdldChjZmcuYXBpLmdldEZldGNoU2Vzc2lvbnNVcmwoKSkuZG9uZSgoanNvbikgPT4ge1xuICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1NFU1NJTlNfUkVDRUlWRSwganNvbi5zZXNzaW9ucyk7XG4gICAgfSk7XG4gIH0sXG5cbiAgdXBkYXRlU2Vzc2lvbihqc29uKXtcbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfU0VTU0lOU19VUERBVEUsIGpzb24pO1xuICB9ICBcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2FjdGlvbnMuanNcbiAqKi8iLCJ2YXIgeyB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxuY29uc3Qgc2Vzc2lvbnNCeVNlcnZlciA9IChzZXJ2ZXJJZCkgPT4gW1sndGxwdF9zZXNzaW9ucyddLCAoc2Vzc2lvbnMpID0+e1xuICByZXR1cm4gc2Vzc2lvbnMudmFsdWVTZXEoKS5maWx0ZXIoaXRlbT0+e1xuICAgIHZhciBwYXJ0aWVzID0gaXRlbS5nZXQoJ3BhcnRpZXMnKSB8fCB0b0ltbXV0YWJsZShbXSk7XG4gICAgdmFyIGhhc1NlcnZlciA9IHBhcnRpZXMuZmluZChpdGVtMj0+IGl0ZW0yLmdldCgnc2VydmVyX2lkJykgPT09IHNlcnZlcklkKTtcbiAgICByZXR1cm4gaGFzU2VydmVyO1xuICB9KS50b0xpc3QoKTtcbn1dXG5cbmNvbnN0IHNlc3Npb25zVmlldyA9IFtbJ3RscHRfc2Vzc2lvbnMnXSwgKHNlc3Npb25zKSA9PntcbiAgcmV0dXJuIHNlc3Npb25zLnZhbHVlU2VxKCkubWFwKGNyZWF0ZVZpZXcpLnRvSlMoKTtcbn1dO1xuXG5jb25zdCBzZXNzaW9uVmlld0J5SWQgPSAoc2lkKT0+IFtbJ3RscHRfc2Vzc2lvbnMnLCBzaWRdLCAoc2Vzc2lvbik9PntcbiAgaWYoIXNlc3Npb24pe1xuICAgIHJldHVybiBudWxsO1xuICB9XG5cbiAgcmV0dXJuIGNyZWF0ZVZpZXcoc2Vzc2lvbik7XG59XTtcblxuY29uc3QgcGFydGllc0J5U2Vzc2lvbklkID0gKHNpZCkgPT5cbiBbWyd0bHB0X3Nlc3Npb25zJywgc2lkLCAncGFydGllcyddLCAocGFydGllcykgPT57XG5cbiAgaWYoIXBhcnRpZXMpe1xuICAgIHJldHVybiBbXTtcbiAgfVxuXG4gIHZhciBsYXN0QWN0aXZlVXNyTmFtZSA9IGdldExhc3RBY3RpdmVVc2VyKHBhcnRpZXMpLmdldCgndXNlcicpO1xuXG4gIHJldHVybiBwYXJ0aWVzLm1hcChpdGVtPT57XG4gICAgdmFyIHVzZXIgPSBpdGVtLmdldCgndXNlcicpO1xuICAgIHJldHVybiB7XG4gICAgICB1c2VyOiBpdGVtLmdldCgndXNlcicpLFxuICAgICAgc2VydmVySXA6IGl0ZW0uZ2V0KCdyZW1vdGVfYWRkcicpLFxuICAgICAgc2VydmVySWQ6IGl0ZW0uZ2V0KCdzZXJ2ZXJfaWQnKSxcbiAgICAgIGlzQWN0aXZlOiBsYXN0QWN0aXZlVXNyTmFtZSA9PT0gdXNlclxuICAgIH1cbiAgfSkudG9KUygpO1xufV07XG5cbmZ1bmN0aW9uIGdldExhc3RBY3RpdmVVc2VyKHBhcnRpZXMpe1xuICByZXR1cm4gcGFydGllcy5zb3J0QnkoaXRlbT0+IG5ldyBEYXRlKGl0ZW0uZ2V0KCdsYXN0QWN0aXZlJykpKS5maXJzdCgpO1xufVxuXG5mdW5jdGlvbiBjcmVhdGVWaWV3KHNlc3Npb24pe1xuICB2YXIgc2lkID0gc2Vzc2lvbi5nZXQoJ2lkJyk7XG4gIHZhciBzZXJ2ZXJJcCwgc2VydmVySWQ7XG4gIHZhciBwYXJ0aWVzID0gcmVhY3Rvci5ldmFsdWF0ZShwYXJ0aWVzQnlTZXNzaW9uSWQoc2lkKSk7XG5cbiAgaWYocGFydGllcy5sZW5ndGggPiAwKXtcbiAgICBzZXJ2ZXJJcCA9IHBhcnRpZXNbMF0uc2VydmVySXA7XG4gICAgc2VydmVySWQgPSBwYXJ0aWVzWzBdLnNlcnZlcklkO1xuICB9XG5cbiAgcmV0dXJuIHtcbiAgICBzaWQ6IHNpZCxcbiAgICBzZXNzaW9uVXJsOiBjZmcuZ2V0QWN0aXZlU2Vzc2lvblJvdXRlVXJsKHNpZCksXG4gICAgc2VydmVySXAsXG4gICAgc2VydmVySWQsXG4gICAgYWN0aXZlOiBzZXNzaW9uLmdldCgnYWN0aXZlJyksXG4gICAgY3JlYXRlZDogbmV3IERhdGUoc2Vzc2lvbi5nZXQoJ2NyZWF0ZWQnKSksXG4gICAgbGFzdEFjdGl2ZTogbmV3IERhdGUoc2Vzc2lvbi5nZXQoJ2xhc3RfYWN0aXZlJykpLFxuICAgIGxvZ2luOiBzZXNzaW9uLmdldCgnbG9naW4nKSxcbiAgICBwYXJ0aWVzOiBwYXJ0aWVzLFxuICAgIGNvbHM6IHNlc3Npb24uZ2V0SW4oWyd0ZXJtaW5hbF9wYXJhbXMnLCAndyddKSxcbiAgICByb3dzOiBzZXNzaW9uLmdldEluKFsndGVybWluYWxfcGFyYW1zJywgJ2gnXSlcbiAgfVxufVxuXG5leHBvcnQgZGVmYXVsdCB7XG4gIHBhcnRpZXNCeVNlc3Npb25JZCxcbiAgc2Vzc2lvbnNCeVNlcnZlcixcbiAgc2Vzc2lvbnNWaWV3LFxuICBzZXNzaW9uVmlld0J5SWQsXG4gIGNyZWF0ZVZpZXdcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2dldHRlcnMuanNcbiAqKi8iLCJ2YXIgYXBpID0gcmVxdWlyZSgnLi9zZXJ2aWNlcy9hcGknKTtcbnZhciBzZXNzaW9uID0gcmVxdWlyZSgnLi9zZXNzaW9uJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xudmFyICQgPSByZXF1aXJlKCdqUXVlcnknKTtcblxuY29uc3QgcmVmcmVzaFJhdGUgPSA2MDAwMCAqIDU7IC8vIDEgbWluXG5cbnZhciByZWZyZXNoVG9rZW5UaW1lcklkID0gbnVsbDtcblxudmFyIGF1dGggPSB7XG5cbiAgc2lnblVwKG5hbWUsIHBhc3N3b3JkLCB0b2tlbiwgaW52aXRlVG9rZW4pe1xuICAgIHZhciBkYXRhID0ge3VzZXI6IG5hbWUsIHBhc3M6IHBhc3N3b3JkLCBzZWNvbmRfZmFjdG9yX3Rva2VuOiB0b2tlbiwgaW52aXRlX3Rva2VuOiBpbnZpdGVUb2tlbn07XG4gICAgcmV0dXJuIGFwaS5wb3N0KGNmZy5hcGkuY3JlYXRlVXNlclBhdGgsIGRhdGEpXG4gICAgICAudGhlbigodXNlcik9PntcbiAgICAgICAgc2Vzc2lvbi5zZXRVc2VyRGF0YSh1c2VyKTtcbiAgICAgICAgYXV0aC5fc3RhcnRUb2tlblJlZnJlc2hlcigpO1xuICAgICAgICByZXR1cm4gdXNlcjtcbiAgICAgIH0pO1xuICB9LFxuXG4gIGxvZ2luKG5hbWUsIHBhc3N3b3JkLCB0b2tlbil7XG4gICAgYXV0aC5fc3RvcFRva2VuUmVmcmVzaGVyKCk7XG4gICAgcmV0dXJuIGF1dGguX2xvZ2luKG5hbWUsIHBhc3N3b3JkLCB0b2tlbikuZG9uZShhdXRoLl9zdGFydFRva2VuUmVmcmVzaGVyKTtcbiAgfSxcblxuICBlbnN1cmVVc2VyKCl7XG4gICAgdmFyIHVzZXJEYXRhID0gc2Vzc2lvbi5nZXRVc2VyRGF0YSgpO1xuICAgIGlmKHVzZXJEYXRhLnRva2VuKXtcbiAgICAgIC8vIHJlZnJlc2ggdGltZXIgd2lsbCBub3QgYmUgc2V0IGluIGNhc2Ugb2YgYnJvd3NlciByZWZyZXNoIGV2ZW50XG4gICAgICBpZihhdXRoLl9nZXRSZWZyZXNoVG9rZW5UaW1lcklkKCkgPT09IG51bGwpe1xuICAgICAgICByZXR1cm4gYXV0aC5fcmVmcmVzaFRva2VuKCkuZG9uZShhdXRoLl9zdGFydFRva2VuUmVmcmVzaGVyKTtcbiAgICAgIH1cblxuICAgICAgcmV0dXJuICQuRGVmZXJyZWQoKS5yZXNvbHZlKHVzZXJEYXRhKTtcbiAgICB9XG5cbiAgICByZXR1cm4gJC5EZWZlcnJlZCgpLnJlamVjdCgpO1xuICB9LFxuXG4gIGxvZ291dCgpe1xuICAgIGF1dGguX3N0b3BUb2tlblJlZnJlc2hlcigpO1xuICAgIHNlc3Npb24uY2xlYXIoKTtcbiAgICBzZXNzaW9uLmdldEhpc3RvcnkoKS5yZXBsYWNlKHtwYXRobmFtZTogY2ZnLnJvdXRlcy5sb2dpbn0pOyAgICBcbiAgfSxcblxuICBfc3RhcnRUb2tlblJlZnJlc2hlcigpe1xuICAgIHJlZnJlc2hUb2tlblRpbWVySWQgPSBzZXRJbnRlcnZhbChhdXRoLl9yZWZyZXNoVG9rZW4sIHJlZnJlc2hSYXRlKTtcbiAgfSxcblxuICBfc3RvcFRva2VuUmVmcmVzaGVyKCl7XG4gICAgY2xlYXJJbnRlcnZhbChyZWZyZXNoVG9rZW5UaW1lcklkKTtcbiAgICByZWZyZXNoVG9rZW5UaW1lcklkID0gbnVsbDtcbiAgfSxcblxuICBfZ2V0UmVmcmVzaFRva2VuVGltZXJJZCgpe1xuICAgIHJldHVybiByZWZyZXNoVG9rZW5UaW1lcklkO1xuICB9LFxuXG4gIF9yZWZyZXNoVG9rZW4oKXtcbiAgICByZXR1cm4gYXBpLnBvc3QoY2ZnLmFwaS5yZW5ld1Rva2VuUGF0aCkudGhlbihkYXRhPT57XG4gICAgICBzZXNzaW9uLnNldFVzZXJEYXRhKGRhdGEpO1xuICAgICAgcmV0dXJuIGRhdGE7XG4gICAgfSkuZmFpbCgoKT0+e1xuICAgICAgYXV0aC5sb2dvdXQoKTtcbiAgICB9KTtcbiAgfSxcblxuICBfbG9naW4obmFtZSwgcGFzc3dvcmQsIHRva2VuKXtcbiAgICB2YXIgZGF0YSA9IHtcbiAgICAgIHVzZXI6IG5hbWUsXG4gICAgICBwYXNzOiBwYXNzd29yZCxcbiAgICAgIHNlY29uZF9mYWN0b3JfdG9rZW46IHRva2VuXG4gICAgfTtcblxuICAgIHJldHVybiBhcGkucG9zdChjZmcuYXBpLnNlc3Npb25QYXRoLCBkYXRhLCBmYWxzZSkudGhlbihkYXRhPT57XG4gICAgICBzZXNzaW9uLnNldFVzZXJEYXRhKGRhdGEpO1xuICAgICAgcmV0dXJuIGRhdGE7XG4gICAgfSk7XG4gIH1cbn1cblxubW9kdWxlLmV4cG9ydHMgPSBhdXRoO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2F1dGguanNcbiAqKi8iLCJ2YXIgRXZlbnRFbWl0dGVyID0gcmVxdWlyZSgnZXZlbnRzJykuRXZlbnRFbWl0dGVyO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCdhcHAvc2Vzc2lvbicpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcbnZhciB7YWN0aW9uc30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC8nKTtcblxuY2xhc3MgVHR5IGV4dGVuZHMgRXZlbnRFbWl0dGVyIHtcblxuICBjb25zdHJ1Y3Rvcih7c2VydmVySWQsIGxvZ2luLCBzaWQsIHJvd3MsIGNvbHMgfSl7XG4gICAgc3VwZXIoKTtcbiAgICB0aGlzLm9wdGlvbnMgPSB7IHNlcnZlcklkLCBsb2dpbiwgc2lkLCByb3dzLCBjb2xzIH07XG4gICAgdGhpcy5zb2NrZXQgPSBudWxsO1xuICB9XG5cbiAgZGlzY29ubmVjdCgpe1xuICAgIHRoaXMuc29ja2V0LmNsb3NlKCk7XG4gIH1cblxuICByZWNvbm5lY3Qob3B0aW9ucyl7XG4gICAgdGhpcy5kaXNjb25uZWN0KCk7XG4gICAgdGhpcy5zb2NrZXQub25vcGVuID0gbnVsbDtcbiAgICB0aGlzLnNvY2tldC5vbm1lc3NhZ2UgPSBudWxsO1xuICAgIHRoaXMuc29ja2V0Lm9uY2xvc2UgPSBudWxsO1xuICAgIFxuICAgIHRoaXMuY29ubmVjdChvcHRpb25zKTtcbiAgfVxuXG4gIGNvbm5lY3Qob3B0aW9ucyl7XG4gICAgT2JqZWN0LmFzc2lnbih0aGlzLm9wdGlvbnMsIG9wdGlvbnMpO1xuXG4gICAgbGV0IHt0b2tlbn0gPSBzZXNzaW9uLmdldFVzZXJEYXRhKCk7XG4gICAgbGV0IGNvbm5TdHIgPSBjZmcuYXBpLmdldFR0eUNvbm5TdHIoe3Rva2VuLCAuLi50aGlzLm9wdGlvbnN9KTtcblxuICAgIHRoaXMuc29ja2V0ID0gbmV3IFdlYlNvY2tldChjb25uU3RyLCAncHJvdG8nKTtcblxuICAgIHRoaXMuc29ja2V0Lm9ub3BlbiA9ICgpID0+IHtcbiAgICAgIHRoaXMuZW1pdCgnb3BlbicpO1xuICAgIH1cblxuICAgIHRoaXMuc29ja2V0Lm9ubWVzc2FnZSA9IChlKT0+e1xuICAgICAgdGhpcy5lbWl0KCdkYXRhJywgZS5kYXRhKTtcbiAgICB9XG5cbiAgICB0aGlzLnNvY2tldC5vbmNsb3NlID0gKCk9PntcbiAgICAgIHRoaXMuZW1pdCgnY2xvc2UnKTtcbiAgICB9XG4gIH1cblxuICByZXNpemUoY29scywgcm93cyl7XG4gICAgYWN0aW9ucy5yZXNpemUoY29scywgcm93cyk7XG4gIH1cblxuICBzZW5kKGRhdGEpe1xuICAgIHRoaXMuc29ja2V0LnNlbmQoZGF0YSk7XG4gIH1cbn1cblxubW9kdWxlLmV4cG9ydHMgPSBUdHk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tbW9uL3R0eS5qc1xuICoqLyIsImltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX1RFUk1fT1BFTjogbnVsbCxcbiAgVExQVF9URVJNX0NMT1NFOiBudWxsLFxuICBUTFBUX1RFUk1fQ0hBTkdFX1NFUlZFUjogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGlvblR5cGVzLmpzXG4gKiovIiwidmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG52YXIgeyBUTFBUX1RFUk1fT1BFTiwgVExQVF9URVJNX0NMT1NFLCBUTFBUX1RFUk1fQ0hBTkdFX1NFUlZFUiB9ICA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKG51bGwpO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX1RFUk1fT1BFTiwgc2V0QWN0aXZlVGVybWluYWwpO1xuICAgIHRoaXMub24oVExQVF9URVJNX0NMT1NFLCBjbG9zZSk7XG4gICAgdGhpcy5vbihUTFBUX1RFUk1fQ0hBTkdFX1NFUlZFUiwgY2hhbmdlU2VydmVyKTtcbiAgfVxufSlcblxuZnVuY3Rpb24gY2hhbmdlU2VydmVyKHN0YXRlLCB7c2VydmVySWQsIGxvZ2lufSl7XG4gIHJldHVybiBzdGF0ZS5zZXQoJ3NlcnZlcklkJywgc2VydmVySWQpXG4gICAgICAgICAgICAgIC5zZXQoJ2xvZ2luJywgbG9naW4pO1xufVxuXG5mdW5jdGlvbiBjbG9zZSgpe1xuICByZXR1cm4gdG9JbW11dGFibGUobnVsbCk7XG59XG5cbmZ1bmN0aW9uIHNldEFjdGl2ZVRlcm1pbmFsKHN0YXRlLCB7c2VydmVySWQsIGxvZ2luLCBzaWQsIGlzTmV3U2Vzc2lvbn0gKXtcbiAgcmV0dXJuIHRvSW1tdXRhYmxlKHtcbiAgICBzZXJ2ZXJJZCxcbiAgICBsb2dpbixcbiAgICBzaWQsXG4gICAgaXNOZXdTZXNzaW9uXG4gIH0pO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvYWN0aXZlVGVybVN0b3JlLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfQVBQX0lOSVQ6IG51bGwsXG4gIFRMUFRfQVBQX0ZBSUxFRDogbnVsbCxcbiAgVExQVF9BUFBfUkVBRFk6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hcHAvYWN0aW9uVHlwZXMuanNcbiAqKi8iLCJ2YXIgeyBTdG9yZSwgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcblxudmFyIHsgVExQVF9BUFBfSU5JVCwgVExQVF9BUFBfRkFJTEVELCBUTFBUX0FQUF9SRUFEWSB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG52YXIgaW5pdFN0YXRlID0gdG9JbW11dGFibGUoe1xuICBpc1JlYWR5OiBmYWxzZSxcbiAgaXNJbml0aWFsaXppbmc6IGZhbHNlLFxuICBpc0ZhaWxlZDogZmFsc2Vcbn0pO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG5cbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiBpbml0U3RhdGUuc2V0KCdpc0luaXRpYWxpemluZycsIHRydWUpO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX0FQUF9JTklULCAoKT0+IGluaXRTdGF0ZS5zZXQoJ2lzSW5pdGlhbGl6aW5nJywgdHJ1ZSkpO1xuICAgIHRoaXMub24oVExQVF9BUFBfUkVBRFksKCk9PiBpbml0U3RhdGUuc2V0KCdpc1JlYWR5JywgdHJ1ZSkpO1xuICAgIHRoaXMub24oVExQVF9BUFBfRkFJTEVELCgpPT4gaW5pdFN0YXRlLnNldCgnaXNGYWlsZWQnLCB0cnVlKSk7XG4gIH1cbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hcHAvYXBwU3RvcmUuanNcbiAqKi8iLCJpbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVExQVF9ESUFMT0dfU0VMRUNUX05PREVfU0hPVzogbnVsbCxcbiAgVExQVF9ESUFMT0dfU0VMRUNUX05PREVfQ0xPU0U6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9kaWFsb2dzL2FjdGlvblR5cGVzLmpzXG4gKiovIiwidmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG5cbnZhciB7IFRMUFRfRElBTE9HX1NFTEVDVF9OT0RFX1NIT1csIFRMUFRfRElBTE9HX1NFTEVDVF9OT0RFX0NMT1NFIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IFN0b3JlKHtcblxuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKHtcbiAgICAgIGlzU2VsZWN0Tm9kZURpYWxvZ09wZW46IGZhbHNlXG4gICAgfSk7XG4gIH0sXG5cbiAgaW5pdGlhbGl6ZSgpIHtcbiAgICB0aGlzLm9uKFRMUFRfRElBTE9HX1NFTEVDVF9OT0RFX1NIT1csIHNob3dTZWxlY3ROb2RlRGlhbG9nKTtcbiAgICB0aGlzLm9uKFRMUFRfRElBTE9HX1NFTEVDVF9OT0RFX0NMT1NFLCBjbG9zZVNlbGVjdE5vZGVEaWFsb2cpO1xuICB9XG59KVxuXG5mdW5jdGlvbiBzaG93U2VsZWN0Tm9kZURpYWxvZyhzdGF0ZSl7XG4gIHJldHVybiBzdGF0ZS5zZXQoJ2lzU2VsZWN0Tm9kZURpYWxvZ09wZW4nLCB0cnVlKTtcbn1cblxuZnVuY3Rpb24gY2xvc2VTZWxlY3ROb2RlRGlhbG9nKHN0YXRlKXtcbiAgcmV0dXJuIHN0YXRlLnNldCgnaXNTZWxlY3ROb2RlRGlhbG9nT3BlbicsIGZhbHNlKTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvZGlhbG9nU3RvcmUuanNcbiAqKi8iLCJpbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVExQVF9SRUNFSVZFX1VTRVJfSU5WSVRFOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2FjdGlvblR5cGVzLmpzXG4gKiovIiwidmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG52YXIgIHsgVExQVF9SRUNFSVZFX1VTRVJfSU5WSVRFIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IFN0b3JlKHtcbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB0b0ltbXV0YWJsZShudWxsKTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9SRUNFSVZFX1VTRVJfSU5WSVRFLCByZWNlaXZlSW52aXRlKVxuICB9XG59KVxuXG5mdW5jdGlvbiByZWNlaXZlSW52aXRlKHN0YXRlLCBpbnZpdGUpe1xuICByZXR1cm4gdG9JbW11dGFibGUoaW52aXRlKTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9pbnZpdGVTdG9yZS5qc1xuICoqLyIsImltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX05PREVTX1JFQ0VJVkU6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9hY3Rpb25UeXBlcy5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7IFRMUFRfTk9ERVNfUkVDRUlWRSB9ICA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcbnZhciBhcGkgPSByZXF1aXJlKCdhcHAvc2VydmljZXMvYXBpJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xuXG5leHBvcnQgZGVmYXVsdCB7XG4gIGZldGNoTm9kZXMoKXtcbiAgICBhcGkuZ2V0KGNmZy5hcGkubm9kZXNQYXRoKS5kb25lKChkYXRhPVtdKT0+e1xuICAgICAgdmFyIG5vZGVBcnJheSA9IGRhdGEubm9kZXMubWFwKGl0ZW09Pml0ZW0ubm9kZSk7XG4gICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfTk9ERVNfUkVDRUlWRSwgbm9kZUFycmF5KTtcbiAgICB9KTtcbiAgfVxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvYWN0aW9ucy5qc1xuICoqLyIsInZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyICB7IFRMUFRfTk9ERVNfUkVDRUlWRSB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gdG9JbW11dGFibGUoW10pO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX05PREVTX1JFQ0VJVkUsIHJlY2VpdmVOb2RlcylcbiAgfVxufSlcblxuZnVuY3Rpb24gcmVjZWl2ZU5vZGVzKHN0YXRlLCBub2RlQXJyYXkpe1xuICByZXR1cm4gdG9JbW11dGFibGUobm9kZUFycmF5KTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL25vZGVTdG9yZS5qc1xuICoqLyIsImltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX1JFU1RfQVBJX1NUQVJUOiBudWxsLFxuICBUTFBUX1JFU1RfQVBJX1NVQ0NFU1M6IG51bGwsXG4gIFRMUFRfUkVTVF9BUElfRkFJTDogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvYWN0aW9uVHlwZXMuanNcbiAqKi8iLCJpbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVFJZSU5HX1RPX1NJR05fVVA6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2NvbnN0YW50cy5qc1xuICoqLyIsImltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX1NFU1NJTlNfUkVDRUlWRTogbnVsbCxcbiAgVExQVF9TRVNTSU5TX1VQREFURTogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2FjdGlvblR5cGVzLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMuZ2V0dGVycyA9IHJlcXVpcmUoJy4vZ2V0dGVycycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aW9ucyA9IHJlcXVpcmUoJy4vYWN0aW9ucycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aXZlVGVybVN0b3JlID0gcmVxdWlyZSgnLi9zZXNzaW9uU3RvcmUnKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2luZGV4LmpzXG4gKiovIiwidmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG52YXIgeyBUTFBUX1NFU1NJTlNfUkVDRUlWRSwgVExQVF9TRVNTSU5TX1VQREFURSB9ICA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKHt9KTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9TRVNTSU5TX1JFQ0VJVkUsIHJlY2VpdmVTZXNzaW9ucyk7XG4gICAgdGhpcy5vbihUTFBUX1NFU1NJTlNfVVBEQVRFLCB1cGRhdGVTZXNzaW9uKTtcbiAgfVxufSlcblxuZnVuY3Rpb24gdXBkYXRlU2Vzc2lvbihzdGF0ZSwganNvbil7XG4gIHJldHVybiBzdGF0ZS5zZXQoanNvbi5pZCwgdG9JbW11dGFibGUoanNvbikpO1xufVxuXG5mdW5jdGlvbiByZWNlaXZlU2Vzc2lvbnMoc3RhdGUsIGpzb25BcnJheT1bXSl7XG4gIHJldHVybiBzdGF0ZS53aXRoTXV0YXRpb25zKHN0YXRlID0+IHtcbiAgICBqc29uQXJyYXkuZm9yRWFjaCgoaXRlbSkgPT4ge1xuICAgICAgc3RhdGUuc2V0KGl0ZW0uaWQsIHRvSW1tdXRhYmxlKGl0ZW0pKVxuICAgIH0pXG4gIH0pO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvc2Vzc2lvblN0b3JlLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfUkVDRUlWRV9VU0VSOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvdXNlci9hY3Rpb25UeXBlcy5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7IFRMUFRfUkVDRUlWRV9VU0VSIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG52YXIgeyBUUllJTkdfVE9fU0lHTl9VUH0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9yZXN0QXBpL2NvbnN0YW50cycpO1xudmFyIHJlc3RBcGlBY3Rpb25zID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvcmVzdEFwaS9hY3Rpb25zJyk7XG52YXIgYXV0aCA9IHJlcXVpcmUoJ2FwcC9hdXRoJyk7XG52YXIgc2Vzc2lvbiA9IHJlcXVpcmUoJ2FwcC9zZXNzaW9uJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xuXG5leHBvcnQgZGVmYXVsdCB7XG5cbiAgZW5zdXJlVXNlcihuZXh0U3RhdGUsIHJlcGxhY2UsIGNiKXtcbiAgICBhdXRoLmVuc3VyZVVzZXIoKVxuICAgICAgLmRvbmUoKHVzZXJEYXRhKT0+IHtcbiAgICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFQ0VJVkVfVVNFUiwgdXNlckRhdGEudXNlciApO1xuICAgICAgICBjYigpO1xuICAgICAgfSlcbiAgICAgIC5mYWlsKCgpPT57XG4gICAgICAgIHJlcGxhY2Uoe3JlZGlyZWN0VG86IG5leHRTdGF0ZS5sb2NhdGlvbi5wYXRobmFtZSB9LCBjZmcucm91dGVzLmxvZ2luKTtcbiAgICAgICAgY2IoKTtcbiAgICAgIH0pO1xuICB9LFxuXG4gIHNpZ25VcCh7bmFtZSwgcHN3LCB0b2tlbiwgaW52aXRlVG9rZW59KXtcbiAgICByZXN0QXBpQWN0aW9ucy5zdGFydChUUllJTkdfVE9fU0lHTl9VUCk7XG4gICAgYXV0aC5zaWduVXAobmFtZSwgcHN3LCB0b2tlbiwgaW52aXRlVG9rZW4pXG4gICAgICAuZG9uZSgoc2Vzc2lvbkRhdGEpPT57XG4gICAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRUNFSVZFX1VTRVIsIHNlc3Npb25EYXRhLnVzZXIpO1xuICAgICAgICByZXN0QXBpQWN0aW9ucy5zdWNjZXNzKFRSWUlOR19UT19TSUdOX1VQKTtcbiAgICAgICAgc2Vzc2lvbi5nZXRIaXN0b3J5KCkucHVzaCh7cGF0aG5hbWU6IGNmZy5yb3V0ZXMuYXBwfSk7XG4gICAgICB9KVxuICAgICAgLmZhaWwoKCk9PntcbiAgICAgICAgcmVzdEFwaUFjdGlvbnMuZmFpbChUUllJTkdfVE9fU0lHTl9VUCwgJ2ZhaWxlZCB0byBzaW5nIHVwJyk7XG4gICAgICB9KTtcbiAgfSxcblxuICBsb2dpbih7dXNlciwgcGFzc3dvcmQsIHRva2VufSwgcmVkaXJlY3Qpe1xuICAgICAgYXV0aC5sb2dpbih1c2VyLCBwYXNzd29yZCwgdG9rZW4pXG4gICAgICAgIC5kb25lKChzZXNzaW9uRGF0YSk9PntcbiAgICAgICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfUkVDRUlWRV9VU0VSLCBzZXNzaW9uRGF0YS51c2VyKTtcbiAgICAgICAgICBzZXNzaW9uLmdldEhpc3RvcnkoKS5wdXNoKHtwYXRobmFtZTogcmVkaXJlY3R9KTtcbiAgICAgICAgfSlcbiAgICAgICAgLmZhaWwoKCk9PntcbiAgICAgICAgfSlcbiAgICB9XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy91c2VyL2FjdGlvbnMuanNcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cy5nZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3Rpb25zID0gcmVxdWlyZSgnLi9hY3Rpb25zJyk7XG5tb2R1bGUuZXhwb3J0cy5ub2RlU3RvcmUgPSByZXF1aXJlKCcuL3VzZXJTdG9yZScpO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvdXNlci9pbmRleC5qc1xuICoqLyIsInZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyICB7IFRMUFRfUkVDRUlWRV9VU0VSIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IFN0b3JlKHtcbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB0b0ltbXV0YWJsZShudWxsKTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9SRUNFSVZFX1VTRVIsIHJlY2VpdmVVc2VyKVxuICB9XG5cbn0pXG5cbmZ1bmN0aW9uIHJlY2VpdmVVc2VyKHN0YXRlLCB1c2VyKXtcbiAgcmV0dXJuIHRvSW1tdXRhYmxlKHVzZXIpO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvdXNlci91c2VyU3RvcmUuanNcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHthY3Rpb25zfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsLycpO1xudmFyIGNvbG9ycyA9IFsnIzFhYjM5NCcsICcjMWM4NGM2JywgJyMyM2M2YzgnLCAnI2Y4YWM1OScsICcjRUQ1NTY1JywgJyNjMmMyYzInXTtcblxuY29uc3QgVXNlckljb24gPSAoe25hbWUsIHRpdGxlLCBjb2xvckluZGV4PTB9KT0+e1xuICBsZXQgY29sb3IgPSBjb2xvcnNbY29sb3JJbmRleCAlIGNvbG9ycy5sZW5ndGhdO1xuICBsZXQgc3R5bGUgPSB7XG4gICAgJ2JhY2tncm91bmRDb2xvcic6IGNvbG9yLFxuICAgICdib3JkZXJDb2xvcic6IGNvbG9yXG4gIH07XG5cbiAgcmV0dXJuIChcbiAgICA8bGk+XG4gICAgICA8c3BhbiBzdHlsZT17c3R5bGV9IGNsYXNzTmFtZT1cImJ0biBidG4tcHJpbWFyeSBidG4tY2lyY2xlIHRleHQtdXBwZXJjYXNlXCI+XG4gICAgICAgIDxzdHJvbmc+e25hbWVbMF19PC9zdHJvbmc+XG4gICAgICA8L3NwYW4+XG4gICAgPC9saT5cbiAgKVxufTtcblxuY29uc3QgU2Vzc2lvbkxlZnRQYW5lbCA9ICh7cGFydGllc30pID0+IHtcbiAgcGFydGllcyA9IHBhcnRpZXMgfHwgW107XG4gIGxldCB1c2VySWNvbnMgPSBwYXJ0aWVzLm1hcCgoaXRlbSwgaW5kZXgpPT4oXG4gICAgPFVzZXJJY29uIGtleT17aW5kZXh9IGNvbG9ySW5kZXg9e2luZGV4fSBuYW1lPXtpdGVtLnVzZXJ9Lz5cbiAgKSk7XG5cbiAgcmV0dXJuIChcbiAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi10ZXJtaW5hbC1wYXJ0aWNpcGFuc1wiPlxuICAgICAgPHVsIGNsYXNzTmFtZT1cIm5hdlwiPlxuICAgICAgICB7dXNlckljb25zfVxuICAgICAgICA8bGk+XG4gICAgICAgICAgPGJ1dHRvbiBvbkNsaWNrPXthY3Rpb25zLmNsb3NlfSBjbGFzc05hbWU9XCJidG4gYnRuLWRhbmdlciBidG4tY2lyY2xlXCIgdHlwZT1cImJ1dHRvblwiPlxuICAgICAgICAgICAgPGkgY2xhc3NOYW1lPVwiZmEgZmEtdGltZXNcIj48L2k+XG4gICAgICAgICAgPC9idXR0b24+XG4gICAgICAgIDwvbGk+XG4gICAgICA8L3VsPlxuICAgIDwvZGl2PlxuICApXG59O1xuXG5tb2R1bGUuZXhwb3J0cyA9IFNlc3Npb25MZWZ0UGFuZWw7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9jdXJyZW50U2Vzc2lvbi9zZXNzaW9uTGVmdFBhbmVsLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG5cbnZhciBHb29nbGVBdXRoSW5mbyA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgcmVuZGVyKCkge1xuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1nb29nbGUtYXV0aFwiPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1nb29nbGUtYXV0aC1pY29uXCI+PC9kaXY+XG4gICAgICAgIDxzdHJvbmc+R29vZ2xlIEF1dGhlbnRpY2F0b3I8L3N0cm9uZz5cbiAgICAgICAgPGRpdj5Eb3dubG9hZCA8YSBocmVmPVwiaHR0cHM6Ly9zdXBwb3J0Lmdvb2dsZS5jb20vYWNjb3VudHMvYW5zd2VyLzEwNjY0NDc/aGw9ZW5cIj5Hb29nbGUgQXV0aGVudGljYXRvcjwvYT4gb24geW91ciBwaG9uZSB0byBhY2Nlc3MgeW91ciB0d28gZmFjdG9yeSB0b2tlbjwvZGl2PlxuICAgICAgPC9kaXY+XG4gICAgKTtcbiAgfVxufSlcblxubW9kdWxlLmV4cG9ydHMgPSBHb29nbGVBdXRoSW5mbztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2dvb2dsZUF1dGhMb2dvLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIge2dldHRlcnMsIGFjdGlvbnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvbm9kZXMnKTtcbnZhciB1c2VyR2V0dGVycyA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3VzZXIvZ2V0dGVycycpO1xudmFyIHtUYWJsZSwgQ29sdW1uLCBDZWxsfSA9IHJlcXVpcmUoJ2FwcC9jb21wb25lbnRzL3RhYmxlLmpzeCcpO1xudmFyIHtjcmVhdGVOZXdTZXNzaW9ufSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGlvbnMnKTtcblxuY29uc3QgVGV4dENlbGwgPSAoe3Jvd0luZGV4LCBkYXRhLCBjb2x1bW5LZXksIC4uLnByb3BzfSkgPT4gKFxuICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgIHtkYXRhW3Jvd0luZGV4XVtjb2x1bW5LZXldfVxuICA8L0NlbGw+XG4pO1xuXG5jb25zdCBUYWdDZWxsID0gKHtyb3dJbmRleCwgZGF0YSwgY29sdW1uS2V5LCAuLi5wcm9wc30pID0+IChcbiAgPENlbGwgey4uLnByb3BzfT5cbiAgICB7IGRhdGFbcm93SW5kZXhdLnRhZ3MubWFwKChpdGVtLCBpbmRleCkgPT5cbiAgICAgICg8c3BhbiBrZXk9e2luZGV4fSBjbGFzc05hbWU9XCJsYWJlbCBsYWJlbC1kZWZhdWx0XCI+XG4gICAgICAgIHtpdGVtLnJvbGV9IDxsaSBjbGFzc05hbWU9XCJmYSBmYS1sb25nLWFycm93LXJpZ2h0XCI+PC9saT5cbiAgICAgICAge2l0ZW0udmFsdWV9XG4gICAgICA8L3NwYW4+KVxuICAgICkgfVxuICA8L0NlbGw+XG4pO1xuXG5jb25zdCBMb2dpbkNlbGwgPSAoe2xvZ2lucywgb25Mb2dpbkNsaWNrLCByb3dJbmRleCwgZGF0YSwgLi4ucHJvcHN9KSA9PiB7XG4gIGlmKCFsb2dpbnMgfHxsb2dpbnMubGVuZ3RoID09PSAwKXtcbiAgICByZXR1cm4gPENlbGwgey4uLnByb3BzfSAvPjtcbiAgfVxuXG4gIHZhciBzZXJ2ZXJJZCA9IGRhdGFbcm93SW5kZXhdLmlkO1xuICB2YXIgJGxpcyA9IFtdO1xuXG4gIGZ1bmN0aW9uIG9uQ2xpY2soaSl7XG4gICAgdmFyIGxvZ2luID0gbG9naW5zW2ldO1xuICAgIGlmKG9uTG9naW5DbGljayl7XG4gICAgICByZXR1cm4gKCk9PiBvbkxvZ2luQ2xpY2soc2VydmVySWQsIGxvZ2luKTtcbiAgICB9ZWxzZXtcbiAgICAgIHJldHVybiAoKSA9PiBjcmVhdGVOZXdTZXNzaW9uKHNlcnZlcklkLCBsb2dpbik7XG4gICAgfVxuICB9XG5cbiAgZm9yKHZhciBpID0gMDsgaSA8IGxvZ2lucy5sZW5ndGg7IGkrKyl7XG4gICAgJGxpcy5wdXNoKDxsaSBrZXk9e2l9PjxhIG9uQ2xpY2s9e29uQ2xpY2soaSl9Pntsb2dpbnNbaV19PC9hPjwvbGk+KTtcbiAgfVxuXG4gIHJldHVybiAoXG4gICAgPENlbGwgey4uLnByb3BzfT5cbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiYnRuLWdyb3VwXCI+XG4gICAgICAgIDxidXR0b24gdHlwZT1cImJ1dHRvblwiIG9uQ2xpY2s9e29uQ2xpY2soMCl9IGNsYXNzTmFtZT1cImJ0biBidG4teHMgYnRuLXByaW1hcnlcIj57bG9naW5zWzBdfTwvYnV0dG9uPlxuICAgICAgICB7XG4gICAgICAgICAgJGxpcy5sZW5ndGggPiAxID8gKFxuICAgICAgICAgICAgICBbXG4gICAgICAgICAgICAgICAgPGJ1dHRvbiBrZXk9ezB9IGRhdGEtdG9nZ2xlPVwiZHJvcGRvd25cIiBjbGFzc05hbWU9XCJidG4gYnRuLWRlZmF1bHQgYnRuLXhzIGRyb3Bkb3duLXRvZ2dsZVwiIGFyaWEtZXhwYW5kZWQ9XCJ0cnVlXCI+XG4gICAgICAgICAgICAgICAgICA8c3BhbiBjbGFzc05hbWU9XCJjYXJldFwiPjwvc3Bhbj5cbiAgICAgICAgICAgICAgICA8L2J1dHRvbj4sXG4gICAgICAgICAgICAgICAgPHVsIGtleT17MX0gY2xhc3NOYW1lPVwiZHJvcGRvd24tbWVudVwiPlxuICAgICAgICAgICAgICAgICAgeyRsaXN9XG4gICAgICAgICAgICAgICAgPC91bD5cbiAgICAgICAgICAgICAgXSApXG4gICAgICAgICAgICA6IG51bGxcbiAgICAgICAgfVxuICAgICAgPC9kaXY+XG4gICAgPC9DZWxsPlxuICApXG59O1xuXG52YXIgTm9kZUxpc3QgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgdmFyIGRhdGEgPSB0aGlzLnByb3BzLm5vZGVSZWNvcmRzO1xuICAgIHZhciBsb2dpbnMgPSB0aGlzLnByb3BzLmxvZ2lucztcbiAgICB2YXIgb25Mb2dpbkNsaWNrID0gdGhpcy5wcm9wcy5vbkxvZ2luQ2xpY2s7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LW5vZGVzXCI+XG4gICAgICAgIDxoMT4gTm9kZXMgPC9oMT5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICAgICAgPFRhYmxlIHJvd0NvdW50PXtkYXRhLmxlbmd0aH0gY2xhc3NOYW1lPVwidGFibGUtc3RyaXBlZCBncnYtbm9kZXMtdGFibGVcIj5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJzZXNzaW9uQ291bnRcIlxuICAgICAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD4gU2Vzc2lvbnMgPC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXs8VGV4dENlbGwgZGF0YT17ZGF0YX0vPiB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJhZGRyXCJcbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+IE5vZGUgPC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXs8VGV4dENlbGwgZGF0YT17ZGF0YX0vPiB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJ0YWdzXCJcbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+PC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXs8VGFnQ2VsbCBkYXRhPXtkYXRhfS8+IH1cbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICAgIGNvbHVtbktleT1cInJvbGVzXCJcbiAgICAgICAgICAgICAgICAgIG9uTG9naW5DbGljaz17b25Mb2dpbkNsaWNrfVxuICAgICAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD5Mb2dpbiBhczwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17PExvZ2luQ2VsbCBkYXRhPXtkYXRhfSBsb2dpbnM9e2xvZ2luc30vPiB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgPC9UYWJsZT5cbiAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgIClcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gTm9kZUxpc3Q7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9ub2Rlcy9ub2RlTGlzdC5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xuXG52YXIgTm90Rm91bmRQYWdlID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXBhZ2Utbm90Zm91bmRcIj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbG9nby10cHJ0XCI+VGVsZXBvcnQ8L2Rpdj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtd2FybmluZ1wiPjxpIGNsYXNzTmFtZT1cImZhIGZhLXdhcm5pbmdcIj48L2k+IDwvZGl2PlxuICAgICAgICA8aDE+V2hvb3BzLCB3ZSBjYW5ub3QgZmluZCB0aGF0PC9oMT5cbiAgICAgICAgPGRpdj5Mb29rcyBsaWtlIHRoZSBwYWdlIHlvdSBhcmUgbG9va2luZyBmb3IgaXNuJ3QgaGVyZSBhbnkgbG9uZ2VyPC9kaXY+XG4gICAgICAgIDxkaXY+SWYgeW91IGJlbGlldmUgdGhpcyBpcyBhbiBlcnJvciwgcGxlYXNlIGNvbnRhY3QgeW91ciBvcmdhbml6YXRpb24gYWRtaW5pc3RyYXRvci48L2Rpdj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJjb250YWN0LXNlY3Rpb25cIj5JZiB5b3UgYmVsaWV2ZSB0aGlzIGlzIGFuIGlzc3VlIHdpdGggVGVsZXBvcnQsIHBsZWFzZSA8YSBocmVmPVwiaHR0cHM6Ly9naXRodWIuY29tL2dyYXZpdGF0aW9uYWwvdGVsZXBvcnQvaXNzdWVzL25ld1wiPmNyZWF0ZSBhIEdpdEh1YiBpc3N1ZS48L2E+XG4gICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pXG5cbm1vZHVsZS5leHBvcnRzID0gTm90Rm91bmRQYWdlO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvbm90Rm91bmRQYWdlLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIge2dldHRlcnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvZGlhbG9ncycpO1xudmFyIHtjbG9zZVNlbGVjdE5vZGVEaWFsb2d9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvZGlhbG9ncy9hY3Rpb25zJyk7XG52YXIge2NoYW5nZVNlcnZlcn0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3Rpb25zJyk7XG52YXIgTm9kZUxpc3QgPSByZXF1aXJlKCcuL25vZGVzL25vZGVMaXN0LmpzeCcpO1xudmFyIGFjdGl2ZVNlc3Npb25HZXR0ZXJzID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvZ2V0dGVycycpO1xudmFyIG5vZGVHZXR0ZXJzID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvbm9kZXMvZ2V0dGVycycpO1xuXG52YXIgU2VsZWN0Tm9kZURpYWxvZyA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBtaXhpbnM6IFtyZWFjdG9yLlJlYWN0TWl4aW5dLFxuXG4gIGdldERhdGFCaW5kaW5ncygpIHtcbiAgICByZXR1cm4ge1xuICAgICAgZGlhbG9nczogZ2V0dGVycy5kaWFsb2dzXG4gICAgfVxuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICByZXR1cm4gdGhpcy5zdGF0ZS5kaWFsb2dzLmlzU2VsZWN0Tm9kZURpYWxvZ09wZW4gPyA8RGlhbG9nLz4gOiBudWxsO1xuICB9XG59KTtcblxudmFyIERpYWxvZyA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBvbkxvZ2luQ2xpY2soc2VydmVySWQsIGxvZ2luKXtcbiAgICBpZihTZWxlY3ROb2RlRGlhbG9nLm9uU2VydmVyQ2hhbmdlQ2FsbEJhY2spe1xuICAgICAgU2VsZWN0Tm9kZURpYWxvZy5vblNlcnZlckNoYW5nZUNhbGxCYWNrKHtzZXJ2ZXJJZH0pO1xuICAgIH1cblxuICAgIGNsb3NlU2VsZWN0Tm9kZURpYWxvZygpO1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50KGNhbGxiYWNrKXtcbiAgICAkKCcubW9kYWwnKS5tb2RhbCgnaGlkZScpO1xuICB9LFxuXG4gIGNvbXBvbmVudERpZE1vdW50KCl7XG4gICAgJCgnLm1vZGFsJykubW9kYWwoJ3Nob3cnKTtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgdmFyIGFjdGl2ZVNlc3Npb24gPSByZWFjdG9yLmV2YWx1YXRlKGFjdGl2ZVNlc3Npb25HZXR0ZXJzLmFjdGl2ZVNlc3Npb24pIHx8IHt9O1xuICAgIHZhciBub2RlUmVjb3JkcyA9IHJlYWN0b3IuZXZhbHVhdGUobm9kZUdldHRlcnMubm9kZUxpc3RWaWV3KTtcbiAgICB2YXIgbG9naW5zID0gW2FjdGl2ZVNlc3Npb24ubG9naW5dO1xuXG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwibW9kYWwgZmFkZSBncnYtZGlhbG9nLXNlbGVjdC1ub2RlXCIgdGFiSW5kZXg9ey0xfSByb2xlPVwiZGlhbG9nXCI+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwibW9kYWwtZGlhbG9nXCI+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJtb2RhbC1jb250ZW50XCI+XG4gICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIm1vZGFsLWhlYWRlclwiPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIm1vZGFsLWJvZHlcIj5cbiAgICAgICAgICAgICAgPE5vZGVMaXN0IG5vZGVSZWNvcmRzPXtub2RlUmVjb3Jkc30gbG9naW5zPXtsb2dpbnN9IG9uTG9naW5DbGljaz17dGhpcy5vbkxvZ2luQ2xpY2t9Lz5cbiAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJtb2RhbC1mb290ZXJcIj5cbiAgICAgICAgICAgICAgPGJ1dHRvbiBvbkNsaWNrPXtjbG9zZVNlbGVjdE5vZGVEaWFsb2d9IHR5cGU9XCJidXR0b25cIiBjbGFzc05hbWU9XCJidG4gYnRuLXByaW1hcnlcIj5cbiAgICAgICAgICAgICAgICBDbG9zZVxuICAgICAgICAgICAgICA8L2J1dHRvbj5cbiAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pO1xuXG5TZWxlY3ROb2RlRGlhbG9nLm9uU2VydmVyQ2hhbmdlQ2FsbEJhY2sgPSAoKT0+e307XG5cbm1vZHVsZS5leHBvcnRzID0gU2VsZWN0Tm9kZURpYWxvZztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL3NlbGVjdE5vZGVEaWFsb2cuanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcblxuY29uc3QgR3J2VGFibGVUZXh0Q2VsbCA9ICh7cm93SW5kZXgsIGRhdGEsIGNvbHVtbktleSwgLi4ucHJvcHN9KSA9PiAoXG4gIDxHcnZUYWJsZUNlbGwgey4uLnByb3BzfT5cbiAgICB7ZGF0YVtyb3dJbmRleF1bY29sdW1uS2V5XX1cbiAgPC9HcnZUYWJsZUNlbGw+XG4pO1xuXG52YXIgR3J2U29ydEhlYWRlckNlbGwgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICB0aGlzLl9vblNvcnRDaGFuZ2UgPSB0aGlzLl9vblNvcnRDaGFuZ2UuYmluZCh0aGlzKTtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgdmFyIHtzb3J0RGlyLCBjaGlsZHJlbiwgLi4ucHJvcHN9ID0gdGhpcy5wcm9wcztcbiAgICByZXR1cm4gKFxuICAgICAgPENlbGwgey4uLnByb3BzfT5cbiAgICAgICAgPGEgb25DbGljaz17dGhpcy5fb25Tb3J0Q2hhbmdlfT5cbiAgICAgICAgICB7Y2hpbGRyZW59IHtzb3J0RGlyID8gKHNvcnREaXIgPT09IFNvcnRUeXBlcy5ERVNDID8gJ+KGkycgOiAn4oaRJykgOiAnJ31cbiAgICAgICAgPC9hPlxuICAgICAgPC9DZWxsPlxuICAgICk7XG4gIH0sXG5cbiAgX29uU29ydENoYW5nZShlKSB7XG4gICAgZS5wcmV2ZW50RGVmYXVsdCgpO1xuXG4gICAgaWYgKHRoaXMucHJvcHMub25Tb3J0Q2hhbmdlKSB7XG4gICAgICB0aGlzLnByb3BzLm9uU29ydENoYW5nZShcbiAgICAgICAgdGhpcy5wcm9wcy5jb2x1bW5LZXksXG4gICAgICAgIHRoaXMucHJvcHMuc29ydERpciA/XG4gICAgICAgICAgcmV2ZXJzZVNvcnREaXJlY3Rpb24odGhpcy5wcm9wcy5zb3J0RGlyKSA6XG4gICAgICAgICAgU29ydFR5cGVzLkRFU0NcbiAgICAgICk7XG4gICAgfVxuICB9XG59KTtcblxudmFyIEdydlRhYmxlQ2VsbCA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgcmVuZGVyKCl7XG4gICAgdmFyIHByb3BzID0gdGhpcy5wcm9wcztcbiAgICByZXR1cm4gcHJvcHMuaXNIZWFkZXIgPyA8dGgga2V5PXtwcm9wcy5rZXl9Pntwcm9wcy5jaGlsZHJlbn08L3RoPiA6IDx0ZCBrZXk9e3Byb3BzLmtleX0+e3Byb3BzLmNoaWxkcmVufTwvdGQ+O1xuICB9XG59KTtcblxudmFyIEdydlRhYmxlID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIHJlbmRlckhlYWRlcihjaGlsZHJlbil7XG4gICAgdmFyIGNlbGxzID0gY2hpbGRyZW4ubWFwKChpdGVtLCBpbmRleCk9PntcbiAgICAgIHJldHVybiB0aGlzLnJlbmRlckNlbGwoaXRlbS5wcm9wcy5oZWFkZXIsIHtpbmRleCwga2V5OiBpbmRleCwgaXNIZWFkZXI6IHRydWUsIC4uLml0ZW0ucHJvcHN9KTtcbiAgICB9KVxuXG4gICAgcmV0dXJuIDx0aGVhZD48dHI+e2NlbGxzfTwvdHI+PC90aGVhZD5cbiAgfSxcblxuICByZW5kZXJCb2R5KGNoaWxkcmVuKXtcbiAgICB2YXIgY291bnQgPSB0aGlzLnByb3BzLnJvd0NvdW50O1xuICAgIHZhciByb3dzID0gW107XG4gICAgZm9yKHZhciBpID0gMDsgaSA8IGNvdW50OyBpICsrKXtcbiAgICAgIHZhciBjZWxscyA9IGNoaWxkcmVuLm1hcCgoaXRlbSwgaW5kZXgpPT57XG4gICAgICAgIHJldHVybiB0aGlzLnJlbmRlckNlbGwoaXRlbS5wcm9wcy5jZWxsLCB7cm93SW5kZXg6IGksIGtleTogaW5kZXgsIGlzSGVhZGVyOiBmYWxzZSwgLi4uaXRlbS5wcm9wc30pO1xuICAgICAgfSlcblxuICAgICAgcm93cy5wdXNoKDx0ciBrZXk9e2l9PntjZWxsc308L3RyPik7XG4gICAgfVxuXG4gICAgcmV0dXJuIDx0Ym9keT57cm93c308L3Rib2R5PjtcbiAgfSxcblxuICByZW5kZXJDZWxsKGNlbGwsIGNlbGxQcm9wcyl7XG4gICAgdmFyIGNvbnRlbnQgPSBudWxsO1xuICAgIGlmIChSZWFjdC5pc1ZhbGlkRWxlbWVudChjZWxsKSkge1xuICAgICAgIGNvbnRlbnQgPSBSZWFjdC5jbG9uZUVsZW1lbnQoY2VsbCwgY2VsbFByb3BzKTtcbiAgICAgfSBlbHNlIGlmICh0eXBlb2YgcHJvcHMuY2VsbCA9PT0gJ2Z1bmN0aW9uJykge1xuICAgICAgIGNvbnRlbnQgPSBjZWxsKGNlbGxQcm9wcyk7XG4gICAgIH1cblxuICAgICByZXR1cm4gY29udGVudDtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgdmFyIGNoaWxkcmVuID0gW107XG4gICAgUmVhY3QuQ2hpbGRyZW4uZm9yRWFjaCh0aGlzLnByb3BzLmNoaWxkcmVuLCAoY2hpbGQsIGluZGV4KSA9PiB7XG4gICAgICBpZiAoY2hpbGQgPT0gbnVsbCkge1xuICAgICAgICByZXR1cm47XG4gICAgICB9XG5cbiAgICAgIGlmKGNoaWxkLnR5cGUuZGlzcGxheU5hbWUgIT09ICdHcnZUYWJsZUNvbHVtbicpe1xuICAgICAgICB0aHJvdyAnU2hvdWxkIGJlIEdydlRhYmxlQ29sdW1uJztcbiAgICAgIH1cblxuICAgICAgY2hpbGRyZW4ucHVzaChjaGlsZCk7XG4gICAgfSk7XG5cbiAgICB2YXIgdGFibGVDbGFzcyA9ICd0YWJsZSAnICsgdGhpcy5wcm9wcy5jbGFzc05hbWU7XG5cbiAgICByZXR1cm4gKFxuICAgICAgPHRhYmxlIGNsYXNzTmFtZT17dGFibGVDbGFzc30+XG4gICAgICAgIHt0aGlzLnJlbmRlckhlYWRlcihjaGlsZHJlbil9XG4gICAgICAgIHt0aGlzLnJlbmRlckJvZHkoY2hpbGRyZW4pfVxuICAgICAgPC90YWJsZT5cbiAgICApO1xuICB9XG59KVxuXG52YXIgR3J2VGFibGVDb2x1bW4gPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgdGhyb3cgbmV3IEVycm9yKCdDb21wb25lbnQgPEdydlRhYmxlQ29sdW1uIC8+IHNob3VsZCBuZXZlciByZW5kZXInKTtcbiAgfVxufSlcblxuZXhwb3J0IGRlZmF1bHQgR3J2VGFibGU7XG5leHBvcnQge0dydlRhYmxlQ29sdW1uIGFzIENvbHVtbiwgR3J2VGFibGUgYXMgVGFibGUsIEdydlRhYmxlQ2VsbCBhcyBDZWxsLCBHcnZUYWJsZVRleHRDZWxsIGFzIFRleHRDZWxsLCBHcnZTb3J0SGVhZGVyQ2VsbCBhcyBTb3J0SGVhZGVyQ2VsbH07XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy90YWJsZS5qc3hcbiAqKi8iLCJ2YXIgVGVybSA9IHJlcXVpcmUoJ1Rlcm1pbmFsJyk7XG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHtkZWJvdW5jZSwgaXNOdW1iZXJ9ID0gcmVxdWlyZSgnXycpO1xuXG5UZXJtLmNvbG9yc1syNTZdID0gJ2luaGVyaXQnO1xuXG5jb25zdCBESVNDT05ORUNUX1RYVCA9ICdcXHgxYlszMW1kaXNjb25uZWN0ZWRcXHgxYlttXFxyXFxuJztcbmNvbnN0IENPTk5FQ1RFRF9UWFQgPSAnQ29ubmVjdGVkIVxcclxcbic7XG5cbnZhciBUdHlUZXJtaW5hbCA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBnZXRJbml0aWFsU3RhdGUoKXtcbiAgICB0aGlzLnJvd3MgPSB0aGlzLnByb3BzLnJvd3M7XG4gICAgdGhpcy5jb2xzID0gdGhpcy5wcm9wcy5jb2xzO1xuICAgIHRoaXMudHR5ID0gdGhpcy5wcm9wcy50dHk7XG5cbiAgICB0aGlzLmRlYm91bmNlZFJlc2l6ZSA9IGRlYm91bmNlKCgpPT57XG4gICAgICB0aGlzLnJlc2l6ZSgpO1xuICAgICAgdGhpcy50dHkucmVzaXplKHRoaXMuY29scywgdGhpcy5yb3dzKTtcbiAgICB9LCAyMDApO1xuXG4gICAgcmV0dXJuIHt9O1xuICB9LFxuXG4gIGNvbXBvbmVudERpZE1vdW50OiBmdW5jdGlvbigpIHtcbiAgICB0aGlzLnRlcm0gPSBuZXcgVGVybWluYWwoe1xuICAgICAgY29sczogNSxcbiAgICAgIHJvd3M6IDUsXG4gICAgICB1c2VTdHlsZTogdHJ1ZSxcbiAgICAgIHNjcmVlbktleXM6IHRydWUsXG4gICAgICBjdXJzb3JCbGluazogdHJ1ZVxuICAgIH0pO1xuXG4gICAgdGhpcy50ZXJtLm9wZW4odGhpcy5yZWZzLmNvbnRhaW5lcik7XG4gICAgdGhpcy50ZXJtLm9uKCdkYXRhJywgKGRhdGEpID0+IHRoaXMudHR5LnNlbmQoZGF0YSkpO1xuXG4gICAgdGhpcy5yZXNpemUodGhpcy5jb2xzLCB0aGlzLnJvd3MpO1xuXG4gICAgdGhpcy50dHkub24oJ29wZW4nLCAoKT0+IHRoaXMudGVybS53cml0ZShDT05ORUNURURfVFhUKSk7ICBcbiAgICB0aGlzLnR0eS5vbignZGF0YScsIChkYXRhKSA9PiB0aGlzLnRlcm0ud3JpdGUoZGF0YSkpO1xuICAgIHRoaXMudHR5Lm9uKCdyZXNldCcsICgpPT4gdGhpcy50ZXJtLnJlc2V0KCkpO1xuXG4gICAgdGhpcy50dHkuY29ubmVjdCh7Y29sczogdGhpcy5jb2xzLCByb3dzOiB0aGlzLnJvd3N9KTtcbiAgICB3aW5kb3cuYWRkRXZlbnRMaXN0ZW5lcigncmVzaXplJywgdGhpcy5kZWJvdW5jZWRSZXNpemUpO1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50OiBmdW5jdGlvbigpIHtcbiAgICB0aGlzLnRlcm0uZGVzdHJveSgpO1xuICAgIHdpbmRvdy5yZW1vdmVFdmVudExpc3RlbmVyKCdyZXNpemUnLCB0aGlzLmRlYm91bmNlZFJlc2l6ZSk7XG4gIH0sXG5cbiAgc2hvdWxkQ29tcG9uZW50VXBkYXRlOiBmdW5jdGlvbihuZXdQcm9wcykge1xuICAgIHZhciB7cm93cywgY29sc30gPSBuZXdQcm9wcztcblxuICAgIGlmKCAhaXNOdW1iZXIocm93cykgfHwgIWlzTnVtYmVyKGNvbHMpKXtcbiAgICAgIHJldHVybiBmYWxzZTtcbiAgICB9XG5cbiAgICBpZihyb3dzICE9PSB0aGlzLnJvd3MgfHwgY29scyAhPT0gdGhpcy5jb2xzKXtcbiAgICAgIHRoaXMucmVzaXplKGNvbHMsIHJvd3MpXG4gICAgfVxuXG4gICAgcmV0dXJuIGZhbHNlO1xuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICByZXR1cm4gKCA8ZGl2IGNsYXNzTmFtZT1cImdydi10ZXJtaW5hbFwiIGlkPVwidGVybWluYWwtYm94XCIgcmVmPVwiY29udGFpbmVyXCI+ICA8L2Rpdj4gKTtcbiAgfSxcblxuICByZXNpemU6IGZ1bmN0aW9uKGNvbHMsIHJvd3MpIHtcbiAgICAvLyBpZiBub3QgZGVmaW5lZCwgdXNlIHRoZSBzaXplIG9mIHRoZSBjb250YWluZXJcbiAgICBpZighaXNOdW1iZXIoY29scykgfHwgIWlzTnVtYmVyKHJvd3MpKXtcbiAgICAgIGxldCBkaW0gPSB0aGlzLl9nZXREaW1lbnNpb25zKCk7XG4gICAgICBjb2xzID0gZGltLmNvbHM7XG4gICAgICByb3dzID0gZGltLnJvd3M7XG4gICAgfVxuXG4gICAgdGhpcy5jb2xzID0gY29scztcbiAgICB0aGlzLnJvd3MgPSByb3dzO1xuXG4gICAgdGhpcy50ZXJtLnJlc2l6ZSh0aGlzLmNvbHMsIHRoaXMucm93cyk7XG4gIH0sXG5cbiAgX2dldERpbWVuc2lvbnMoKXtcbiAgICBsZXQgJGNvbnRhaW5lciA9ICQodGhpcy5yZWZzLmNvbnRhaW5lcik7XG4gICAgbGV0IGZha2VSb3cgPSAkKCc8ZGl2PjxzcGFuPiZuYnNwOzwvc3Bhbj48L2Rpdj4nKTtcblxuICAgICRjb250YWluZXIuZmluZCgnLnRlcm1pbmFsJykuYXBwZW5kKGZha2VSb3cpO1xuICAgIC8vIGdldCBkaXYgaGVpZ2h0XG4gICAgbGV0IGZha2VDb2xIZWlnaHQgPSBmYWtlUm93WzBdLmdldEJvdW5kaW5nQ2xpZW50UmVjdCgpLmhlaWdodDtcbiAgICAvLyBnZXQgc3BhbiB3aWR0aFxuICAgIGxldCBmYWtlQ29sV2lkdGggPSBmYWtlUm93LmNoaWxkcmVuKCkuZmlyc3QoKVswXS5nZXRCb3VuZGluZ0NsaWVudFJlY3QoKS53aWR0aDtcbiAgICBsZXQgY29scyA9IE1hdGguZmxvb3IoJGNvbnRhaW5lci53aWR0aCgpIC8gKGZha2VDb2xXaWR0aCkpO1xuICAgIGxldCByb3dzID0gTWF0aC5mbG9vcigkY29udGFpbmVyLmhlaWdodCgpIC8gKGZha2VDb2xIZWlnaHQpKTtcbiAgICBmYWtlUm93LnJlbW92ZSgpO1xuXG4gICAgcmV0dXJuIHtjb2xzLCByb3dzfTtcbiAgfVxuXG59KTtcblxuVHR5VGVybWluYWwucHJvcFR5cGVzID0ge1xuICB0dHk6IFJlYWN0LlByb3BUeXBlcy5vYmplY3QuaXNSZXF1aXJlZFxufVxuXG5tb2R1bGUuZXhwb3J0cyA9IFR0eVRlcm1pbmFsO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvdGVybWluYWwuanN4XG4gKiovIiwiLypcbiAqICBUaGUgTUlUIExpY2Vuc2UgKE1JVClcbiAqICBDb3B5cmlnaHQgKGMpIDIwMTUgUnlhbiBGbG9yZW5jZSwgTWljaGFlbCBKYWNrc29uXG4gKiAgUGVybWlzc2lvbiBpcyBoZXJlYnkgZ3JhbnRlZCwgZnJlZSBvZiBjaGFyZ2UsIHRvIGFueSBwZXJzb24gb2J0YWluaW5nIGEgY29weSBvZiB0aGlzIHNvZnR3YXJlIGFuZCBhc3NvY2lhdGVkIGRvY3VtZW50YXRpb24gZmlsZXMgKHRoZSBcIlNvZnR3YXJlXCIpLCB0byBkZWFsIGluIHRoZSBTb2Z0d2FyZSB3aXRob3V0IHJlc3RyaWN0aW9uLCBpbmNsdWRpbmcgd2l0aG91dCBsaW1pdGF0aW9uIHRoZSByaWdodHMgdG8gdXNlLCBjb3B5LCBtb2RpZnksIG1lcmdlLCBwdWJsaXNoLCBkaXN0cmlidXRlLCBzdWJsaWNlbnNlLCBhbmQvb3Igc2VsbCBjb3BpZXMgb2YgdGhlIFNvZnR3YXJlLCBhbmQgdG8gcGVybWl0IHBlcnNvbnMgdG8gd2hvbSB0aGUgU29mdHdhcmUgaXMgZnVybmlzaGVkIHRvIGRvIHNvLCBzdWJqZWN0IHRvIHRoZSBmb2xsb3dpbmcgY29uZGl0aW9uczpcbiAqICBUaGUgYWJvdmUgY29weXJpZ2h0IG5vdGljZSBhbmQgdGhpcyBwZXJtaXNzaW9uIG5vdGljZSBzaGFsbCBiZSBpbmNsdWRlZCBpbiBhbGwgY29waWVzIG9yIHN1YnN0YW50aWFsIHBvcnRpb25zIG9mIHRoZSBTb2Z0d2FyZS5cbiAqICBUSEUgU09GVFdBUkUgSVMgUFJPVklERUQgXCJBUyBJU1wiLCBXSVRIT1VUIFdBUlJBTlRZIE9GIEFOWSBLSU5ELCBFWFBSRVNTIE9SIElNUExJRUQsIElOQ0xVRElORyBCVVQgTk9UIExJTUlURUQgVE8gVEhFIFdBUlJBTlRJRVMgT0YgTUVSQ0hBTlRBQklMSVRZLCBGSVRORVNTIEZPUiBBIFBBUlRJQ1VMQVIgUFVSUE9TRSBBTkQgTk9OSU5GUklOR0VNRU5ULiBJTiBOTyBFVkVOVCBTSEFMTCBUSEUgQVVUSE9SUyBPUiBDT1BZUklHSFQgSE9MREVSUyBCRSBMSUFCTEUgRk9SIEFOWSBDTEFJTSwgREFNQUdFUyBPUiBPVEhFUiBMSUFCSUxJVFksIFdIRVRIRVIgSU4gQU4gQUNUSU9OIE9GIENPTlRSQUNULCBUT1JUIE9SIE9USEVSV0lTRSwgQVJJU0lORyBGUk9NLCBPVVQgT0YgT1IgSU4gQ09OTkVDVElPTiBXSVRIIFRIRSBTT0ZUV0FSRSBPUiBUSEUgVVNFIE9SIE9USEVSIERFQUxJTkdTIElOIFRIRSBTT0ZUV0FSRS5cbiovXG5cbmltcG9ydCBpbnZhcmlhbnQgZnJvbSAnaW52YXJpYW50J1xuXG5mdW5jdGlvbiBlc2NhcGVSZWdFeHAoc3RyaW5nKSB7XG4gIHJldHVybiBzdHJpbmcucmVwbGFjZSgvWy4qKz9eJHt9KCl8W1xcXVxcXFxdL2csICdcXFxcJCYnKVxufVxuXG5mdW5jdGlvbiBlc2NhcGVTb3VyY2Uoc3RyaW5nKSB7XG4gIHJldHVybiBlc2NhcGVSZWdFeHAoc3RyaW5nKS5yZXBsYWNlKC9cXC8rL2csICcvKycpXG59XG5cbmZ1bmN0aW9uIF9jb21waWxlUGF0dGVybihwYXR0ZXJuKSB7XG4gIGxldCByZWdleHBTb3VyY2UgPSAnJztcbiAgY29uc3QgcGFyYW1OYW1lcyA9IFtdO1xuICBjb25zdCB0b2tlbnMgPSBbXTtcblxuICBsZXQgbWF0Y2gsIGxhc3RJbmRleCA9IDAsIG1hdGNoZXIgPSAvOihbYS16QS1aXyRdW2EtekEtWjAtOV8kXSopfFxcKlxcKnxcXCp8XFwofFxcKS9nXG4gIC8qZXNsaW50IG5vLWNvbmQtYXNzaWduOiAwKi9cbiAgd2hpbGUgKChtYXRjaCA9IG1hdGNoZXIuZXhlYyhwYXR0ZXJuKSkpIHtcbiAgICBpZiAobWF0Y2guaW5kZXggIT09IGxhc3RJbmRleCkge1xuICAgICAgdG9rZW5zLnB1c2gocGF0dGVybi5zbGljZShsYXN0SW5kZXgsIG1hdGNoLmluZGV4KSlcbiAgICAgIHJlZ2V4cFNvdXJjZSArPSBlc2NhcGVTb3VyY2UocGF0dGVybi5zbGljZShsYXN0SW5kZXgsIG1hdGNoLmluZGV4KSlcbiAgICB9XG5cbiAgICBpZiAobWF0Y2hbMV0pIHtcbiAgICAgIHJlZ2V4cFNvdXJjZSArPSAnKFteLz8jXSspJztcbiAgICAgIHBhcmFtTmFtZXMucHVzaChtYXRjaFsxXSk7XG4gICAgfSBlbHNlIGlmIChtYXRjaFswXSA9PT0gJyoqJykge1xuICAgICAgcmVnZXhwU291cmNlICs9ICcoW1xcXFxzXFxcXFNdKiknXG4gICAgICBwYXJhbU5hbWVzLnB1c2goJ3NwbGF0Jyk7XG4gICAgfSBlbHNlIGlmIChtYXRjaFswXSA9PT0gJyonKSB7XG4gICAgICByZWdleHBTb3VyY2UgKz0gJyhbXFxcXHNcXFxcU10qPyknXG4gICAgICBwYXJhbU5hbWVzLnB1c2goJ3NwbGF0Jyk7XG4gICAgfSBlbHNlIGlmIChtYXRjaFswXSA9PT0gJygnKSB7XG4gICAgICByZWdleHBTb3VyY2UgKz0gJyg/Oic7XG4gICAgfSBlbHNlIGlmIChtYXRjaFswXSA9PT0gJyknKSB7XG4gICAgICByZWdleHBTb3VyY2UgKz0gJyk/JztcbiAgICB9XG5cbiAgICB0b2tlbnMucHVzaChtYXRjaFswXSk7XG5cbiAgICBsYXN0SW5kZXggPSBtYXRjaGVyLmxhc3RJbmRleDtcbiAgfVxuXG4gIGlmIChsYXN0SW5kZXggIT09IHBhdHRlcm4ubGVuZ3RoKSB7XG4gICAgdG9rZW5zLnB1c2gocGF0dGVybi5zbGljZShsYXN0SW5kZXgsIHBhdHRlcm4ubGVuZ3RoKSlcbiAgICByZWdleHBTb3VyY2UgKz0gZXNjYXBlU291cmNlKHBhdHRlcm4uc2xpY2UobGFzdEluZGV4LCBwYXR0ZXJuLmxlbmd0aCkpXG4gIH1cblxuICByZXR1cm4ge1xuICAgIHBhdHRlcm4sXG4gICAgcmVnZXhwU291cmNlLFxuICAgIHBhcmFtTmFtZXMsXG4gICAgdG9rZW5zXG4gIH1cbn1cblxuY29uc3QgQ29tcGlsZWRQYXR0ZXJuc0NhY2hlID0ge31cblxuZXhwb3J0IGZ1bmN0aW9uIGNvbXBpbGVQYXR0ZXJuKHBhdHRlcm4pIHtcbiAgaWYgKCEocGF0dGVybiBpbiBDb21waWxlZFBhdHRlcm5zQ2FjaGUpKVxuICAgIENvbXBpbGVkUGF0dGVybnNDYWNoZVtwYXR0ZXJuXSA9IF9jb21waWxlUGF0dGVybihwYXR0ZXJuKVxuXG4gIHJldHVybiBDb21waWxlZFBhdHRlcm5zQ2FjaGVbcGF0dGVybl1cbn1cblxuLyoqXG4gKiBBdHRlbXB0cyB0byBtYXRjaCBhIHBhdHRlcm4gb24gdGhlIGdpdmVuIHBhdGhuYW1lLiBQYXR0ZXJucyBtYXkgdXNlXG4gKiB0aGUgZm9sbG93aW5nIHNwZWNpYWwgY2hhcmFjdGVyczpcbiAqXG4gKiAtIDpwYXJhbU5hbWUgICAgIE1hdGNoZXMgYSBVUkwgc2VnbWVudCB1cCB0byB0aGUgbmV4dCAvLCA/LCBvciAjLiBUaGVcbiAqICAgICAgICAgICAgICAgICAgY2FwdHVyZWQgc3RyaW5nIGlzIGNvbnNpZGVyZWQgYSBcInBhcmFtXCJcbiAqIC0gKCkgICAgICAgICAgICAgV3JhcHMgYSBzZWdtZW50IG9mIHRoZSBVUkwgdGhhdCBpcyBvcHRpb25hbFxuICogLSAqICAgICAgICAgICAgICBDb25zdW1lcyAobm9uLWdyZWVkeSkgYWxsIGNoYXJhY3RlcnMgdXAgdG8gdGhlIG5leHRcbiAqICAgICAgICAgICAgICAgICAgY2hhcmFjdGVyIGluIHRoZSBwYXR0ZXJuLCBvciB0byB0aGUgZW5kIG9mIHRoZSBVUkwgaWZcbiAqICAgICAgICAgICAgICAgICAgdGhlcmUgaXMgbm9uZVxuICogLSAqKiAgICAgICAgICAgICBDb25zdW1lcyAoZ3JlZWR5KSBhbGwgY2hhcmFjdGVycyB1cCB0byB0aGUgbmV4dCBjaGFyYWN0ZXJcbiAqICAgICAgICAgICAgICAgICAgaW4gdGhlIHBhdHRlcm4sIG9yIHRvIHRoZSBlbmQgb2YgdGhlIFVSTCBpZiB0aGVyZSBpcyBub25lXG4gKlxuICogVGhlIHJldHVybiB2YWx1ZSBpcyBhbiBvYmplY3Qgd2l0aCB0aGUgZm9sbG93aW5nIHByb3BlcnRpZXM6XG4gKlxuICogLSByZW1haW5pbmdQYXRobmFtZVxuICogLSBwYXJhbU5hbWVzXG4gKiAtIHBhcmFtVmFsdWVzXG4gKi9cbmV4cG9ydCBmdW5jdGlvbiBtYXRjaFBhdHRlcm4ocGF0dGVybiwgcGF0aG5hbWUpIHtcbiAgLy8gTWFrZSBsZWFkaW5nIHNsYXNoZXMgY29uc2lzdGVudCBiZXR3ZWVuIHBhdHRlcm4gYW5kIHBhdGhuYW1lLlxuICBpZiAocGF0dGVybi5jaGFyQXQoMCkgIT09ICcvJykge1xuICAgIHBhdHRlcm4gPSBgLyR7cGF0dGVybn1gXG4gIH1cbiAgaWYgKHBhdGhuYW1lLmNoYXJBdCgwKSAhPT0gJy8nKSB7XG4gICAgcGF0aG5hbWUgPSBgLyR7cGF0aG5hbWV9YFxuICB9XG5cbiAgbGV0IHsgcmVnZXhwU291cmNlLCBwYXJhbU5hbWVzLCB0b2tlbnMgfSA9IGNvbXBpbGVQYXR0ZXJuKHBhdHRlcm4pXG5cbiAgcmVnZXhwU291cmNlICs9ICcvKicgLy8gQ2FwdHVyZSBwYXRoIHNlcGFyYXRvcnNcblxuICAvLyBTcGVjaWFsLWNhc2UgcGF0dGVybnMgbGlrZSAnKicgZm9yIGNhdGNoLWFsbCByb3V0ZXMuXG4gIGNvbnN0IGNhcHR1cmVSZW1haW5pbmcgPSB0b2tlbnNbdG9rZW5zLmxlbmd0aCAtIDFdICE9PSAnKidcblxuICBpZiAoY2FwdHVyZVJlbWFpbmluZykge1xuICAgIC8vIFRoaXMgd2lsbCBtYXRjaCBuZXdsaW5lcyBpbiB0aGUgcmVtYWluaW5nIHBhdGguXG4gICAgcmVnZXhwU291cmNlICs9ICcoW1xcXFxzXFxcXFNdKj8pJ1xuICB9XG5cbiAgY29uc3QgbWF0Y2ggPSBwYXRobmFtZS5tYXRjaChuZXcgUmVnRXhwKCdeJyArIHJlZ2V4cFNvdXJjZSArICckJywgJ2knKSlcblxuICBsZXQgcmVtYWluaW5nUGF0aG5hbWUsIHBhcmFtVmFsdWVzXG4gIGlmIChtYXRjaCAhPSBudWxsKSB7XG4gICAgaWYgKGNhcHR1cmVSZW1haW5pbmcpIHtcbiAgICAgIHJlbWFpbmluZ1BhdGhuYW1lID0gbWF0Y2gucG9wKClcbiAgICAgIGNvbnN0IG1hdGNoZWRQYXRoID1cbiAgICAgICAgbWF0Y2hbMF0uc3Vic3RyKDAsIG1hdGNoWzBdLmxlbmd0aCAtIHJlbWFpbmluZ1BhdGhuYW1lLmxlbmd0aClcblxuICAgICAgLy8gSWYgd2UgZGlkbid0IG1hdGNoIHRoZSBlbnRpcmUgcGF0aG5hbWUsIHRoZW4gbWFrZSBzdXJlIHRoYXQgdGhlIG1hdGNoXG4gICAgICAvLyB3ZSBkaWQgZ2V0IGVuZHMgYXQgYSBwYXRoIHNlcGFyYXRvciAocG90ZW50aWFsbHkgdGhlIG9uZSB3ZSBhZGRlZFxuICAgICAgLy8gYWJvdmUgYXQgdGhlIGJlZ2lubmluZyBvZiB0aGUgcGF0aCwgaWYgdGhlIGFjdHVhbCBtYXRjaCB3YXMgZW1wdHkpLlxuICAgICAgaWYgKFxuICAgICAgICByZW1haW5pbmdQYXRobmFtZSAmJlxuICAgICAgICBtYXRjaGVkUGF0aC5jaGFyQXQobWF0Y2hlZFBhdGgubGVuZ3RoIC0gMSkgIT09ICcvJ1xuICAgICAgKSB7XG4gICAgICAgIHJldHVybiB7XG4gICAgICAgICAgcmVtYWluaW5nUGF0aG5hbWU6IG51bGwsXG4gICAgICAgICAgcGFyYW1OYW1lcyxcbiAgICAgICAgICBwYXJhbVZhbHVlczogbnVsbFxuICAgICAgICB9XG4gICAgICB9XG4gICAgfSBlbHNlIHtcbiAgICAgIC8vIElmIHRoaXMgbWF0Y2hlZCBhdCBhbGwsIHRoZW4gdGhlIG1hdGNoIHdhcyB0aGUgZW50aXJlIHBhdGhuYW1lLlxuICAgICAgcmVtYWluaW5nUGF0aG5hbWUgPSAnJ1xuICAgIH1cblxuICAgIHBhcmFtVmFsdWVzID0gbWF0Y2guc2xpY2UoMSkubWFwKFxuICAgICAgdiA9PiB2ICE9IG51bGwgPyBkZWNvZGVVUklDb21wb25lbnQodikgOiB2XG4gICAgKVxuICB9IGVsc2Uge1xuICAgIHJlbWFpbmluZ1BhdGhuYW1lID0gcGFyYW1WYWx1ZXMgPSBudWxsXG4gIH1cblxuICByZXR1cm4ge1xuICAgIHJlbWFpbmluZ1BhdGhuYW1lLFxuICAgIHBhcmFtTmFtZXMsXG4gICAgcGFyYW1WYWx1ZXNcbiAgfVxufVxuXG5leHBvcnQgZnVuY3Rpb24gZ2V0UGFyYW1OYW1lcyhwYXR0ZXJuKSB7XG4gIHJldHVybiBjb21waWxlUGF0dGVybihwYXR0ZXJuKS5wYXJhbU5hbWVzXG59XG5cbmV4cG9ydCBmdW5jdGlvbiBnZXRQYXJhbXMocGF0dGVybiwgcGF0aG5hbWUpIHtcbiAgY29uc3QgeyBwYXJhbU5hbWVzLCBwYXJhbVZhbHVlcyB9ID0gbWF0Y2hQYXR0ZXJuKHBhdHRlcm4sIHBhdGhuYW1lKVxuXG4gIGlmIChwYXJhbVZhbHVlcyAhPSBudWxsKSB7XG4gICAgcmV0dXJuIHBhcmFtTmFtZXMucmVkdWNlKGZ1bmN0aW9uIChtZW1vLCBwYXJhbU5hbWUsIGluZGV4KSB7XG4gICAgICBtZW1vW3BhcmFtTmFtZV0gPSBwYXJhbVZhbHVlc1tpbmRleF1cbiAgICAgIHJldHVybiBtZW1vXG4gICAgfSwge30pXG4gIH1cblxuICByZXR1cm4gbnVsbFxufVxuXG4vKipcbiAqIFJldHVybnMgYSB2ZXJzaW9uIG9mIHRoZSBnaXZlbiBwYXR0ZXJuIHdpdGggcGFyYW1zIGludGVycG9sYXRlZC4gVGhyb3dzXG4gKiBpZiB0aGVyZSBpcyBhIGR5bmFtaWMgc2VnbWVudCBvZiB0aGUgcGF0dGVybiBmb3Igd2hpY2ggdGhlcmUgaXMgbm8gcGFyYW0uXG4gKi9cbmV4cG9ydCBmdW5jdGlvbiBmb3JtYXRQYXR0ZXJuKHBhdHRlcm4sIHBhcmFtcykge1xuICBwYXJhbXMgPSBwYXJhbXMgfHwge31cblxuICBjb25zdCB7IHRva2VucyB9ID0gY29tcGlsZVBhdHRlcm4ocGF0dGVybilcbiAgbGV0IHBhcmVuQ291bnQgPSAwLCBwYXRobmFtZSA9ICcnLCBzcGxhdEluZGV4ID0gMFxuXG4gIGxldCB0b2tlbiwgcGFyYW1OYW1lLCBwYXJhbVZhbHVlXG4gIGZvciAobGV0IGkgPSAwLCBsZW4gPSB0b2tlbnMubGVuZ3RoOyBpIDwgbGVuOyArK2kpIHtcbiAgICB0b2tlbiA9IHRva2Vuc1tpXVxuXG4gICAgaWYgKHRva2VuID09PSAnKicgfHwgdG9rZW4gPT09ICcqKicpIHtcbiAgICAgIHBhcmFtVmFsdWUgPSBBcnJheS5pc0FycmF5KHBhcmFtcy5zcGxhdCkgPyBwYXJhbXMuc3BsYXRbc3BsYXRJbmRleCsrXSA6IHBhcmFtcy5zcGxhdFxuXG4gICAgICBpbnZhcmlhbnQoXG4gICAgICAgIHBhcmFtVmFsdWUgIT0gbnVsbCB8fCBwYXJlbkNvdW50ID4gMCxcbiAgICAgICAgJ01pc3Npbmcgc3BsYXQgIyVzIGZvciBwYXRoIFwiJXNcIicsXG4gICAgICAgIHNwbGF0SW5kZXgsIHBhdHRlcm5cbiAgICAgIClcblxuICAgICAgaWYgKHBhcmFtVmFsdWUgIT0gbnVsbClcbiAgICAgICAgcGF0aG5hbWUgKz0gZW5jb2RlVVJJKHBhcmFtVmFsdWUpXG4gICAgfSBlbHNlIGlmICh0b2tlbiA9PT0gJygnKSB7XG4gICAgICBwYXJlbkNvdW50ICs9IDFcbiAgICB9IGVsc2UgaWYgKHRva2VuID09PSAnKScpIHtcbiAgICAgIHBhcmVuQ291bnQgLT0gMVxuICAgIH0gZWxzZSBpZiAodG9rZW4uY2hhckF0KDApID09PSAnOicpIHtcbiAgICAgIHBhcmFtTmFtZSA9IHRva2VuLnN1YnN0cmluZygxKVxuICAgICAgcGFyYW1WYWx1ZSA9IHBhcmFtc1twYXJhbU5hbWVdXG5cbiAgICAgIGludmFyaWFudChcbiAgICAgICAgcGFyYW1WYWx1ZSAhPSBudWxsIHx8IHBhcmVuQ291bnQgPiAwLFxuICAgICAgICAnTWlzc2luZyBcIiVzXCIgcGFyYW1ldGVyIGZvciBwYXRoIFwiJXNcIicsXG4gICAgICAgIHBhcmFtTmFtZSwgcGF0dGVyblxuICAgICAgKVxuXG4gICAgICBpZiAocGFyYW1WYWx1ZSAhPSBudWxsKVxuICAgICAgICBwYXRobmFtZSArPSBlbmNvZGVVUklDb21wb25lbnQocGFyYW1WYWx1ZSlcbiAgICB9IGVsc2Uge1xuICAgICAgcGF0aG5hbWUgKz0gdG9rZW5cbiAgICB9XG4gIH1cblxuICByZXR1cm4gcGF0aG5hbWUucmVwbGFjZSgvXFwvKy9nLCAnLycpXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tbW9uL3BhdHRlcm5VdGlscy5qc1xuICoqLyIsInZhciBUdHkgPSByZXF1aXJlKCdhcHAvY29tbW9uL3R0eScpO1xudmFyIGFwaSA9IHJlcXVpcmUoJ2FwcC9zZXJ2aWNlcy9hcGknKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG5cbmNsYXNzIFR0eVBsYXllciBleHRlbmRzIFR0eSB7XG4gIGNvbnN0cnVjdG9yKHtzaWR9KXtcbiAgICBzdXBlcih7fSk7XG4gICAgdGhpcy5zaWQgPSBzaWQ7XG4gICAgdGhpcy5jdXJyZW50ID0gMTtcbiAgICB0aGlzLmxlbmd0aCA9IC0xO1xuICAgIHRoaXMudHR5U3RlYW0gPSBuZXcgQXJyYXkoKTtcbiAgICB0aGlzLmlzTG9haW5kID0gZmFsc2U7XG4gICAgdGhpcy5pc1BsYXlpbmcgPSBmYWxzZTtcbiAgICB0aGlzLmlzRXJyb3IgPSBmYWxzZTtcbiAgICB0aGlzLmlzUmVhZHkgPSBmYWxzZTtcbiAgICB0aGlzLmlzTG9hZGluZyA9IHRydWU7XG4gIH1cblxuICBzZW5kKCl7XG4gIH1cblxuICByZXNpemUoKXtcbiAgfVxuXG4gIGNvbm5lY3QoKXtcbiAgICBhcGkuZ2V0KGNmZy5hcGkuZ2V0RmV0Y2hTZXNzaW9uTGVuZ3RoVXJsKHRoaXMuc2lkKSlcbiAgICAgIC5kb25lKChkYXRhKT0+e1xuICAgICAgICB0aGlzLmxlbmd0aCA9IGRhdGEuY291bnQ7XG4gICAgICAgIHRoaXMuaXNSZWFkeSA9IHRydWU7XG4gICAgICB9KVxuICAgICAgLmZhaWwoKCk9PntcbiAgICAgICAgdGhpcy5pc0Vycm9yID0gdHJ1ZTtcbiAgICAgIH0pXG4gICAgICAuYWx3YXlzKCgpPT57XG4gICAgICAgIHRoaXMuX2NoYW5nZSgpO1xuICAgICAgfSk7XG4gIH1cblxuICBtb3ZlKG5ld1Bvcyl7XG4gICAgaWYoIXRoaXMuaXNSZWFkeSl7XG4gICAgICByZXR1cm47XG4gICAgfVxuXG4gICAgaWYobmV3UG9zID09PSB1bmRlZmluZWQpe1xuICAgICAgbmV3UG9zID0gdGhpcy5jdXJyZW50ICsgMTtcbiAgICB9XG5cbiAgICBpZihuZXdQb3MgPiB0aGlzLmxlbmd0aCl7XG4gICAgICBuZXdQb3MgPSB0aGlzLmxlbmd0aDtcbiAgICAgIHRoaXMuc3RvcCgpO1xuICAgIH1cblxuICAgIGlmKG5ld1BvcyA9PT0gMCl7XG4gICAgICBuZXdQb3MgPSAxO1xuICAgIH1cblxuICAgIGlmKHRoaXMuaXNQbGF5aW5nKXtcbiAgICAgIGlmKHRoaXMuY3VycmVudCA8IG5ld1Bvcyl7XG4gICAgICAgIHRoaXMuX3Nob3dDaHVuayh0aGlzLmN1cnJlbnQsIG5ld1Bvcyk7XG4gICAgICB9ZWxzZXtcbiAgICAgICAgdGhpcy5lbWl0KCdyZXNldCcpO1xuICAgICAgICB0aGlzLl9zaG93Q2h1bmsodGhpcy5jdXJyZW50LCBuZXdQb3MpO1xuICAgICAgfVxuICAgIH1lbHNle1xuICAgICAgdGhpcy5jdXJyZW50ID0gbmV3UG9zO1xuICAgIH1cblxuICAgIHRoaXMuX2NoYW5nZSgpO1xuICB9XG5cbiAgc3RvcCgpe1xuICAgIHRoaXMuaXNQbGF5aW5nID0gZmFsc2U7XG4gICAgdGhpcy50aW1lciA9IGNsZWFySW50ZXJ2YWwodGhpcy50aW1lcik7XG4gICAgdGhpcy5fY2hhbmdlKCk7XG4gIH1cblxuICBwbGF5KCl7XG4gICAgaWYodGhpcy5pc1BsYXlpbmcpe1xuICAgICAgcmV0dXJuO1xuICAgIH1cblxuICAgIHRoaXMuaXNQbGF5aW5nID0gdHJ1ZTtcblxuICAgIC8vIHN0YXJ0IGZyb20gdGhlIGJlZ2lubmluZyBpZiBhdCB0aGUgZW5kXG4gICAgaWYodGhpcy5jdXJyZW50ID09PSB0aGlzLmxlbmd0aCl7XG4gICAgICB0aGlzLmN1cnJlbnQgPSAxO1xuICAgIH1cblxuICAgIHRoaXMudGltZXIgPSBzZXRJbnRlcnZhbCh0aGlzLm1vdmUuYmluZCh0aGlzKSwgMTUwKTtcbiAgICB0aGlzLl9jaGFuZ2UoKTtcbiAgfVxuXG4gIF9zaG91bGRGZXRjaChzdGFydCwgZW5kKXtcbiAgICBmb3IodmFyIGkgPSBzdGFydDsgaSA8IGVuZDsgaSsrKXtcbiAgICAgIGlmKHRoaXMudHR5U3RlYW1baV0gPT09IHVuZGVmaW5lZCl7XG4gICAgICAgIHJldHVybiB0cnVlO1xuICAgICAgfVxuICAgIH1cblxuICAgIHJldHVybiBmYWxzZTtcbiAgfVxuXG4gIF9mZXRjaChzdGFydCwgZW5kKXtcbiAgICBlbmQgPSBlbmQgKyA1MDtcbiAgICBlbmQgPSBlbmQgPiB0aGlzLmxlbmd0aCA/IHRoaXMubGVuZ3RoIDogZW5kO1xuICAgIHJldHVybiBhcGkuZ2V0KGNmZy5hcGkuZ2V0RmV0Y2hTZXNzaW9uQ2h1bmtVcmwoe3NpZDogdGhpcy5zaWQsIHN0YXJ0LCBlbmR9KSkuXG4gICAgICBkb25lKChyZXNwb25zZSk9PntcbiAgICAgICAgZm9yKHZhciBpID0gMDsgaSA8IGVuZC1zdGFydDsgaSsrKXtcbiAgICAgICAgICB2YXIgZGF0YSA9IGF0b2IocmVzcG9uc2UuY2h1bmtzW2ldLmRhdGEpIHx8ICcnO1xuICAgICAgICAgIHZhciBkZWxheSA9IHJlc3BvbnNlLmNodW5rc1tpXS5kZWxheTtcbiAgICAgICAgICB0aGlzLnR0eVN0ZWFtW3N0YXJ0K2ldID0geyBkYXRhLCBkZWxheX07XG4gICAgICAgIH1cbiAgICAgIH0pO1xuICB9XG5cbiAgX3Nob3dDaHVuayhzdGFydCwgZW5kKXtcbiAgICB2YXIgZGlzcGxheSA9ICgpPT57XG4gICAgICBmb3IodmFyIGkgPSBzdGFydDsgaSA8IGVuZDsgaSsrKXtcbiAgICAgICAgdGhpcy5lbWl0KCdkYXRhJywgdGhpcy50dHlTdGVhbVtpXS5kYXRhKTtcbiAgICAgIH1cbiAgICAgIHRoaXMuY3VycmVudCA9IGVuZDtcbiAgICB9O1xuXG4gICAgaWYodGhpcy5fc2hvdWxkRmV0Y2goc3RhcnQsIGVuZCkpe1xuICAgICAgdGhpcy5fZmV0Y2goc3RhcnQsIGVuZCkudGhlbihkaXNwbGF5KTtcbiAgICB9ZWxzZXtcbiAgICAgIGRpc3BsYXkoKTtcbiAgICB9XG4gIH1cblxuICBfY2hhbmdlKCl7XG4gICAgdGhpcy5lbWl0KCdjaGFuZ2UnKTtcbiAgfVxufVxuXG5leHBvcnQgZGVmYXVsdCBUdHlQbGF5ZXI7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tbW9uL3R0eVBsYXllci5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7ZmV0Y2hTZXNzaW9uc30gPSByZXF1aXJlKCcuLy4uL3Nlc3Npb25zL2FjdGlvbnMnKTtcbnZhciB7ZmV0Y2hOb2RlcyB9ID0gcmVxdWlyZSgnLi8uLi9ub2Rlcy9hY3Rpb25zJyk7XG52YXIgJCA9IHJlcXVpcmUoJ2pRdWVyeScpO1xuXG52YXIgeyBUTFBUX0FQUF9JTklULCBUTFBUX0FQUF9GQUlMRUQsIFRMUFRfQVBQX1JFQURZIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbnZhciBhY3Rpb25zID0ge1xuXG4gIGluaXRBcHAoKSB7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX0FQUF9JTklUKTtcbiAgICBhY3Rpb25zLmZldGNoTm9kZXNBbmRTZXNzaW9ucygpXG4gICAgICAuZG9uZSgoKT0+eyByZWFjdG9yLmRpc3BhdGNoKFRMUFRfQVBQX1JFQURZKTsgfSlcbiAgICAgIC5mYWlsKCgpPT57IHJlYWN0b3IuZGlzcGF0Y2goVExQVF9BUFBfRkFJTEVEKTsgfSk7XG5cbiAgICAvL2FwaS5nZXQoYC92MS93ZWJhcGkvc2l0ZXMvLWN1cnJlbnQtL3Nlc3Npb25zLzAzZDNlMTFkLTQ1YzEtNDA0OS1iY2ViLWIyMzM2MDU2NjZlNC9jaHVua3M/c3RhcnQ9MCZlbmQ9MTAwYCkuZG9uZSgoKSA9PiB7XG4gICAgLy99KTtcbiAgfSxcblxuICBmZXRjaE5vZGVzQW5kU2Vzc2lvbnMoKSB7XG4gICAgcmV0dXJuICQud2hlbihmZXRjaE5vZGVzKCksIGZldGNoU2Vzc2lvbnMoKSk7XG4gIH1cbn1cblxuZXhwb3J0IGRlZmF1bHQgYWN0aW9ucztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2FwcC9hY3Rpb25zLmpzXG4gKiovIiwiY29uc3QgYXBwU3RhdGUgPSBbWyd0bHB0J10sIGFwcD0+IGFwcC50b0pTKCldO1xuXG5leHBvcnQgZGVmYXVsdCB7XG4gIGFwcFN0YXRlXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hcHAvZ2V0dGVycy5qc1xuICoqLyIsIm1vZHVsZS5leHBvcnRzLmdldHRlcnMgPSByZXF1aXJlKCcuL2dldHRlcnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGlvbnMgPSByZXF1aXJlKCcuL2FjdGlvbnMnKTtcbm1vZHVsZS5leHBvcnRzLmFwcFN0b3JlID0gcmVxdWlyZSgnLi9hcHBTdG9yZScpO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvYXBwL2luZGV4LmpzXG4gKiovIiwiY29uc3QgZGlhbG9ncyA9IFtbJ3RscHRfZGlhbG9ncyddLCBzdGF0ZT0+IHN0YXRlLnRvSlMoKV07XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgZGlhbG9nc1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvZGlhbG9ncy9nZXR0ZXJzLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMuZ2V0dGVycyA9IHJlcXVpcmUoJy4vZ2V0dGVycycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aW9ucyA9IHJlcXVpcmUoJy4vYWN0aW9ucycpO1xubW9kdWxlLmV4cG9ydHMuZGlhbG9nU3RvcmUgPSByZXF1aXJlKCcuL2RpYWxvZ1N0b3JlJyk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9kaWFsb2dzL2luZGV4LmpzXG4gKiovIiwidmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xucmVhY3Rvci5yZWdpc3RlclN0b3Jlcyh7XG4gICd0bHB0JzogcmVxdWlyZSgnLi9hcHAvYXBwU3RvcmUnKSxcbiAgJ3RscHRfZGlhbG9ncyc6IHJlcXVpcmUoJy4vZGlhbG9ncy9kaWFsb2dTdG9yZScpLFxuICAndGxwdF9hY3RpdmVfdGVybWluYWwnOiByZXF1aXJlKCcuL2FjdGl2ZVRlcm1pbmFsL2FjdGl2ZVRlcm1TdG9yZScpLFxuICAndGxwdF91c2VyJzogcmVxdWlyZSgnLi91c2VyL3VzZXJTdG9yZScpLFxuICAndGxwdF9ub2Rlcyc6IHJlcXVpcmUoJy4vbm9kZXMvbm9kZVN0b3JlJyksXG4gICd0bHB0X2ludml0ZSc6IHJlcXVpcmUoJy4vaW52aXRlL2ludml0ZVN0b3JlJyksXG4gICd0bHB0X3Jlc3RfYXBpJzogcmVxdWlyZSgnLi9yZXN0QXBpL3Jlc3RBcGlTdG9yZScpLFxuICAndGxwdF9zZXNzaW9ucyc6IHJlcXVpcmUoJy4vc2Vzc2lvbnMvc2Vzc2lvblN0b3JlJylcbn0pO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvaW5kZXguanNcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgeyBUTFBUX1JFQ0VJVkVfVVNFUl9JTlZJVEUgfSAgPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG52YXIgYXBpID0gcmVxdWlyZSgnYXBwL3NlcnZpY2VzL2FwaScpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICBmZXRjaEludml0ZShpbnZpdGVUb2tlbil7XG4gICAgdmFyIHBhdGggPSBjZmcuYXBpLmdldEludml0ZVVybChpbnZpdGVUb2tlbik7XG4gICAgYXBpLmdldChwYXRoKS5kb25lKGludml0ZT0+e1xuICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFQ0VJVkVfVVNFUl9JTlZJVEUsIGludml0ZSk7XG4gICAgfSk7XG4gIH1cbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9hY3Rpb25zLmpzXG4gKiovIiwiLyplc2xpbnQgbm8tdW5kZWY6IDAsICBuby11bnVzZWQtdmFyczogMCwgbm8tZGVidWdnZXI6MCovXG5cbnZhciB7VFJZSU5HX1RPX1NJR05fVVB9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvcmVzdEFwaS9jb25zdGFudHMnKTtcblxuY29uc3QgaW52aXRlID0gWyBbJ3RscHRfaW52aXRlJ10sIChpbnZpdGUpID0+IHtcbiAgcmV0dXJuIGludml0ZTtcbiB9XG5dO1xuXG5jb25zdCBhdHRlbXAgPSBbIFsndGxwdF9yZXN0X2FwaScsIFRSWUlOR19UT19TSUdOX1VQXSwgKGF0dGVtcCkgPT4ge1xuICB2YXIgZGVmYXVsdE9iaiA9IHtcbiAgICBpc1Byb2Nlc3Npbmc6IGZhbHNlLFxuICAgIGlzRXJyb3I6IGZhbHNlLFxuICAgIGlzU3VjY2VzczogZmFsc2UsXG4gICAgbWVzc2FnZTogJydcbiAgfVxuXG4gIHJldHVybiBhdHRlbXAgPyBhdHRlbXAudG9KUygpIDogZGVmYXVsdE9iajtcbiAgXG4gfVxuXTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICBpbnZpdGUsXG4gIGF0dGVtcFxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2dldHRlcnMuanNcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cy5nZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3Rpb25zID0gcmVxdWlyZSgnLi9hY3Rpb25zJyk7XG5tb2R1bGUuZXhwb3J0cy5ub2RlU3RvcmUgPSByZXF1aXJlKCcuL2ludml0ZVN0b3JlJyk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvaW5kZXguanNcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cy5nZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3Rpb25zID0gcmVxdWlyZSgnLi9hY3Rpb25zJyk7XG5tb2R1bGUuZXhwb3J0cy5ub2RlU3RvcmUgPSByZXF1aXJlKCcuL25vZGVTdG9yZScpO1xuXG4vLyBub2RlczogW3tcImlkXCI6XCJ4MjIwXCIsXCJhZGRyXCI6XCIwLjAuMC4wOjMwMjJcIixcImhvc3RuYW1lXCI6XCJ4MjIwXCIsXCJsYWJlbHNcIjpudWxsLFwiY21kX2xhYmVsc1wiOm51bGx9XVxuXG5cbi8vIHNlc3Npb25zOiBbe1wiaWRcIjpcIjA3NjMwNjM2LWJiM2QtNDBlMS1iMDg2LTYwYjJjYWUyMWFjNFwiLFwicGFydGllc1wiOlt7XCJpZFwiOlwiODlmNzYyYTMtNzQyOS00YzdhLWE5MTMtNzY2NDkzZmU3YzhhXCIsXCJzaXRlXCI6XCIxMjcuMC4wLjE6Mzc1MTRcIixcInVzZXJcIjpcImFrb250c2V2b3lcIixcInNlcnZlcl9hZGRyXCI6XCIwLjAuMC4wOjMwMjJcIixcImxhc3RfYWN0aXZlXCI6XCIyMDE2LTAyLTIyVDE0OjM5OjIwLjkzMTIwNTM1LTA1OjAwXCJ9XX1dXG5cbi8qXG5sZXQgVG9kb1JlY29yZCA9IEltbXV0YWJsZS5SZWNvcmQoe1xuICAgIGlkOiAwLFxuICAgIGRlc2NyaXB0aW9uOiBcIlwiLFxuICAgIGNvbXBsZXRlZDogZmFsc2Vcbn0pO1xuKi9cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2luZGV4LmpzXG4gKiovIiwidmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xuXG52YXIge1xuICBUTFBUX1JFU1RfQVBJX1NUQVJULFxuICBUTFBUX1JFU1RfQVBJX1NVQ0NFU1MsXG4gIFRMUFRfUkVTVF9BUElfRkFJTCB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCB7XG5cbiAgc3RhcnQocmVxVHlwZSl7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFU1RfQVBJX1NUQVJULCB7dHlwZTogcmVxVHlwZX0pO1xuICB9LFxuXG4gIGZhaWwocmVxVHlwZSwgbWVzc2FnZSl7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFU1RfQVBJX0ZBSUwsICB7dHlwZTogcmVxVHlwZSwgbWVzc2FnZX0pO1xuICB9LFxuXG4gIHN1Y2Nlc3MocmVxVHlwZSl7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFU1RfQVBJX1NVQ0NFU1MsIHt0eXBlOiByZXFUeXBlfSk7XG4gIH1cblxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9hY3Rpb25zLmpzXG4gKiovIiwidmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG52YXIge1xuICBUTFBUX1JFU1RfQVBJX1NUQVJULFxuICBUTFBUX1JFU1RfQVBJX1NVQ0NFU1MsXG4gIFRMUFRfUkVTVF9BUElfRkFJTCB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gdG9JbW11dGFibGUoe30pO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX1JFU1RfQVBJX1NUQVJULCBzdGFydCk7XG4gICAgdGhpcy5vbihUTFBUX1JFU1RfQVBJX0ZBSUwsIGZhaWwpO1xuICAgIHRoaXMub24oVExQVF9SRVNUX0FQSV9TVUNDRVNTLCBzdWNjZXNzKTtcbiAgfVxufSlcblxuZnVuY3Rpb24gc3RhcnQoc3RhdGUsIHJlcXVlc3Qpe1xuICByZXR1cm4gc3RhdGUuc2V0KHJlcXVlc3QudHlwZSwgdG9JbW11dGFibGUoe2lzUHJvY2Vzc2luZzogdHJ1ZX0pKTtcbn1cblxuZnVuY3Rpb24gZmFpbChzdGF0ZSwgcmVxdWVzdCl7XG4gIHJldHVybiBzdGF0ZS5zZXQocmVxdWVzdC50eXBlLCB0b0ltbXV0YWJsZSh7aXNGYWlsZWQ6IHRydWUsIG1lc3NhZ2U6IHJlcXVlc3QubWVzc2FnZX0pKTtcbn1cblxuZnVuY3Rpb24gc3VjY2VzcyhzdGF0ZSwgcmVxdWVzdCl7XG4gIHJldHVybiBzdGF0ZS5zZXQocmVxdWVzdC50eXBlLCB0b0ltbXV0YWJsZSh7aXNTdWNjZXNzOiB0cnVlfSkpO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9yZXN0QXBpU3RvcmUuanNcbiAqKi8iLCJ2YXIgdXRpbHMgPSB7XG5cbiAgdXVpZCgpe1xuICAgIC8vIG5ldmVyIHVzZSBpdCBpbiBwcm9kdWN0aW9uXG4gICAgcmV0dXJuICd4eHh4eHh4eC14eHh4LTR4eHgteXh4eC14eHh4eHh4eHh4eHgnLnJlcGxhY2UoL1t4eV0vZywgZnVuY3Rpb24oYykge1xuICAgICAgdmFyIHIgPSBNYXRoLnJhbmRvbSgpKjE2fDAsIHYgPSBjID09ICd4JyA/IHIgOiAociYweDN8MHg4KTtcbiAgICAgIHJldHVybiB2LnRvU3RyaW5nKDE2KTtcbiAgICB9KTtcbiAgfSxcblxuICBkaXNwbGF5RGF0ZShkYXRlKXtcbiAgICB0cnl7XG4gICAgICByZXR1cm4gZGF0ZS50b0xvY2FsZURhdGVTdHJpbmcoKSArICcgJyArIGRhdGUudG9Mb2NhbGVUaW1lU3RyaW5nKCk7XG4gICAgfWNhdGNoKGVycil7XG4gICAgICBjb25zb2xlLmVycm9yKGVycik7XG4gICAgICByZXR1cm4gJ3VuZGVmaW5lZCc7XG4gICAgfVxuICB9LFxuXG4gIGZvcm1hdFN0cmluZyhmb3JtYXQpIHtcbiAgICB2YXIgYXJncyA9IEFycmF5LnByb3RvdHlwZS5zbGljZS5jYWxsKGFyZ3VtZW50cywgMSk7XG4gICAgcmV0dXJuIGZvcm1hdC5yZXBsYWNlKG5ldyBSZWdFeHAoJ1xcXFx7KFxcXFxkKylcXFxcfScsICdnJyksXG4gICAgICAobWF0Y2gsIG51bWJlcikgPT4ge1xuICAgICAgICByZXR1cm4gIShhcmdzW251bWJlcl0gPT09IG51bGwgfHwgYXJnc1tudW1iZXJdID09PSB1bmRlZmluZWQpID8gYXJnc1tudW1iZXJdIDogJyc7XG4gICAgfSk7XG4gIH1cbiAgICAgICAgICAgIFxufVxuXG5tb2R1bGUuZXhwb3J0cyA9IHV0aWxzO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL3V0aWxzLmpzXG4gKiovIiwiLy8gQ29weXJpZ2h0IEpveWVudCwgSW5jLiBhbmQgb3RoZXIgTm9kZSBjb250cmlidXRvcnMuXG4vL1xuLy8gUGVybWlzc2lvbiBpcyBoZXJlYnkgZ3JhbnRlZCwgZnJlZSBvZiBjaGFyZ2UsIHRvIGFueSBwZXJzb24gb2J0YWluaW5nIGFcbi8vIGNvcHkgb2YgdGhpcyBzb2Z0d2FyZSBhbmQgYXNzb2NpYXRlZCBkb2N1bWVudGF0aW9uIGZpbGVzICh0aGVcbi8vIFwiU29mdHdhcmVcIiksIHRvIGRlYWwgaW4gdGhlIFNvZnR3YXJlIHdpdGhvdXQgcmVzdHJpY3Rpb24sIGluY2x1ZGluZ1xuLy8gd2l0aG91dCBsaW1pdGF0aW9uIHRoZSByaWdodHMgdG8gdXNlLCBjb3B5LCBtb2RpZnksIG1lcmdlLCBwdWJsaXNoLFxuLy8gZGlzdHJpYnV0ZSwgc3VibGljZW5zZSwgYW5kL29yIHNlbGwgY29waWVzIG9mIHRoZSBTb2Z0d2FyZSwgYW5kIHRvIHBlcm1pdFxuLy8gcGVyc29ucyB0byB3aG9tIHRoZSBTb2Z0d2FyZSBpcyBmdXJuaXNoZWQgdG8gZG8gc28sIHN1YmplY3QgdG8gdGhlXG4vLyBmb2xsb3dpbmcgY29uZGl0aW9uczpcbi8vXG4vLyBUaGUgYWJvdmUgY29weXJpZ2h0IG5vdGljZSBhbmQgdGhpcyBwZXJtaXNzaW9uIG5vdGljZSBzaGFsbCBiZSBpbmNsdWRlZFxuLy8gaW4gYWxsIGNvcGllcyBvciBzdWJzdGFudGlhbCBwb3J0aW9ucyBvZiB0aGUgU29mdHdhcmUuXG4vL1xuLy8gVEhFIFNPRlRXQVJFIElTIFBST1ZJREVEIFwiQVMgSVNcIiwgV0lUSE9VVCBXQVJSQU5UWSBPRiBBTlkgS0lORCwgRVhQUkVTU1xuLy8gT1IgSU1QTElFRCwgSU5DTFVESU5HIEJVVCBOT1QgTElNSVRFRCBUTyBUSEUgV0FSUkFOVElFUyBPRlxuLy8gTUVSQ0hBTlRBQklMSVRZLCBGSVRORVNTIEZPUiBBIFBBUlRJQ1VMQVIgUFVSUE9TRSBBTkQgTk9OSU5GUklOR0VNRU5ULiBJTlxuLy8gTk8gRVZFTlQgU0hBTEwgVEhFIEFVVEhPUlMgT1IgQ09QWVJJR0hUIEhPTERFUlMgQkUgTElBQkxFIEZPUiBBTlkgQ0xBSU0sXG4vLyBEQU1BR0VTIE9SIE9USEVSIExJQUJJTElUWSwgV0hFVEhFUiBJTiBBTiBBQ1RJT04gT0YgQ09OVFJBQ1QsIFRPUlQgT1Jcbi8vIE9USEVSV0lTRSwgQVJJU0lORyBGUk9NLCBPVVQgT0YgT1IgSU4gQ09OTkVDVElPTiBXSVRIIFRIRSBTT0ZUV0FSRSBPUiBUSEVcbi8vIFVTRSBPUiBPVEhFUiBERUFMSU5HUyBJTiBUSEUgU09GVFdBUkUuXG5cbmZ1bmN0aW9uIEV2ZW50RW1pdHRlcigpIHtcbiAgdGhpcy5fZXZlbnRzID0gdGhpcy5fZXZlbnRzIHx8IHt9O1xuICB0aGlzLl9tYXhMaXN0ZW5lcnMgPSB0aGlzLl9tYXhMaXN0ZW5lcnMgfHwgdW5kZWZpbmVkO1xufVxubW9kdWxlLmV4cG9ydHMgPSBFdmVudEVtaXR0ZXI7XG5cbi8vIEJhY2t3YXJkcy1jb21wYXQgd2l0aCBub2RlIDAuMTAueFxuRXZlbnRFbWl0dGVyLkV2ZW50RW1pdHRlciA9IEV2ZW50RW1pdHRlcjtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5fZXZlbnRzID0gdW5kZWZpbmVkO1xuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5fbWF4TGlzdGVuZXJzID0gdW5kZWZpbmVkO1xuXG4vLyBCeSBkZWZhdWx0IEV2ZW50RW1pdHRlcnMgd2lsbCBwcmludCBhIHdhcm5pbmcgaWYgbW9yZSB0aGFuIDEwIGxpc3RlbmVycyBhcmVcbi8vIGFkZGVkIHRvIGl0LiBUaGlzIGlzIGEgdXNlZnVsIGRlZmF1bHQgd2hpY2ggaGVscHMgZmluZGluZyBtZW1vcnkgbGVha3MuXG5FdmVudEVtaXR0ZXIuZGVmYXVsdE1heExpc3RlbmVycyA9IDEwO1xuXG4vLyBPYnZpb3VzbHkgbm90IGFsbCBFbWl0dGVycyBzaG91bGQgYmUgbGltaXRlZCB0byAxMC4gVGhpcyBmdW5jdGlvbiBhbGxvd3Ncbi8vIHRoYXQgdG8gYmUgaW5jcmVhc2VkLiBTZXQgdG8gemVybyBmb3IgdW5saW1pdGVkLlxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5zZXRNYXhMaXN0ZW5lcnMgPSBmdW5jdGlvbihuKSB7XG4gIGlmICghaXNOdW1iZXIobikgfHwgbiA8IDAgfHwgaXNOYU4obikpXG4gICAgdGhyb3cgVHlwZUVycm9yKCduIG11c3QgYmUgYSBwb3NpdGl2ZSBudW1iZXInKTtcbiAgdGhpcy5fbWF4TGlzdGVuZXJzID0gbjtcbiAgcmV0dXJuIHRoaXM7XG59O1xuXG5FdmVudEVtaXR0ZXIucHJvdG90eXBlLmVtaXQgPSBmdW5jdGlvbih0eXBlKSB7XG4gIHZhciBlciwgaGFuZGxlciwgbGVuLCBhcmdzLCBpLCBsaXN0ZW5lcnM7XG5cbiAgaWYgKCF0aGlzLl9ldmVudHMpXG4gICAgdGhpcy5fZXZlbnRzID0ge307XG5cbiAgLy8gSWYgdGhlcmUgaXMgbm8gJ2Vycm9yJyBldmVudCBsaXN0ZW5lciB0aGVuIHRocm93LlxuICBpZiAodHlwZSA9PT0gJ2Vycm9yJykge1xuICAgIGlmICghdGhpcy5fZXZlbnRzLmVycm9yIHx8XG4gICAgICAgIChpc09iamVjdCh0aGlzLl9ldmVudHMuZXJyb3IpICYmICF0aGlzLl9ldmVudHMuZXJyb3IubGVuZ3RoKSkge1xuICAgICAgZXIgPSBhcmd1bWVudHNbMV07XG4gICAgICBpZiAoZXIgaW5zdGFuY2VvZiBFcnJvcikge1xuICAgICAgICB0aHJvdyBlcjsgLy8gVW5oYW5kbGVkICdlcnJvcicgZXZlbnRcbiAgICAgIH1cbiAgICAgIHRocm93IFR5cGVFcnJvcignVW5jYXVnaHQsIHVuc3BlY2lmaWVkIFwiZXJyb3JcIiBldmVudC4nKTtcbiAgICB9XG4gIH1cblxuICBoYW5kbGVyID0gdGhpcy5fZXZlbnRzW3R5cGVdO1xuXG4gIGlmIChpc1VuZGVmaW5lZChoYW5kbGVyKSlcbiAgICByZXR1cm4gZmFsc2U7XG5cbiAgaWYgKGlzRnVuY3Rpb24oaGFuZGxlcikpIHtcbiAgICBzd2l0Y2ggKGFyZ3VtZW50cy5sZW5ndGgpIHtcbiAgICAgIC8vIGZhc3QgY2FzZXNcbiAgICAgIGNhc2UgMTpcbiAgICAgICAgaGFuZGxlci5jYWxsKHRoaXMpO1xuICAgICAgICBicmVhaztcbiAgICAgIGNhc2UgMjpcbiAgICAgICAgaGFuZGxlci5jYWxsKHRoaXMsIGFyZ3VtZW50c1sxXSk7XG4gICAgICAgIGJyZWFrO1xuICAgICAgY2FzZSAzOlxuICAgICAgICBoYW5kbGVyLmNhbGwodGhpcywgYXJndW1lbnRzWzFdLCBhcmd1bWVudHNbMl0pO1xuICAgICAgICBicmVhaztcbiAgICAgIC8vIHNsb3dlclxuICAgICAgZGVmYXVsdDpcbiAgICAgICAgbGVuID0gYXJndW1lbnRzLmxlbmd0aDtcbiAgICAgICAgYXJncyA9IG5ldyBBcnJheShsZW4gLSAxKTtcbiAgICAgICAgZm9yIChpID0gMTsgaSA8IGxlbjsgaSsrKVxuICAgICAgICAgIGFyZ3NbaSAtIDFdID0gYXJndW1lbnRzW2ldO1xuICAgICAgICBoYW5kbGVyLmFwcGx5KHRoaXMsIGFyZ3MpO1xuICAgIH1cbiAgfSBlbHNlIGlmIChpc09iamVjdChoYW5kbGVyKSkge1xuICAgIGxlbiA9IGFyZ3VtZW50cy5sZW5ndGg7XG4gICAgYXJncyA9IG5ldyBBcnJheShsZW4gLSAxKTtcbiAgICBmb3IgKGkgPSAxOyBpIDwgbGVuOyBpKyspXG4gICAgICBhcmdzW2kgLSAxXSA9IGFyZ3VtZW50c1tpXTtcblxuICAgIGxpc3RlbmVycyA9IGhhbmRsZXIuc2xpY2UoKTtcbiAgICBsZW4gPSBsaXN0ZW5lcnMubGVuZ3RoO1xuICAgIGZvciAoaSA9IDA7IGkgPCBsZW47IGkrKylcbiAgICAgIGxpc3RlbmVyc1tpXS5hcHBseSh0aGlzLCBhcmdzKTtcbiAgfVxuXG4gIHJldHVybiB0cnVlO1xufTtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5hZGRMaXN0ZW5lciA9IGZ1bmN0aW9uKHR5cGUsIGxpc3RlbmVyKSB7XG4gIHZhciBtO1xuXG4gIGlmICghaXNGdW5jdGlvbihsaXN0ZW5lcikpXG4gICAgdGhyb3cgVHlwZUVycm9yKCdsaXN0ZW5lciBtdXN0IGJlIGEgZnVuY3Rpb24nKTtcblxuICBpZiAoIXRoaXMuX2V2ZW50cylcbiAgICB0aGlzLl9ldmVudHMgPSB7fTtcblxuICAvLyBUbyBhdm9pZCByZWN1cnNpb24gaW4gdGhlIGNhc2UgdGhhdCB0eXBlID09PSBcIm5ld0xpc3RlbmVyXCIhIEJlZm9yZVxuICAvLyBhZGRpbmcgaXQgdG8gdGhlIGxpc3RlbmVycywgZmlyc3QgZW1pdCBcIm5ld0xpc3RlbmVyXCIuXG4gIGlmICh0aGlzLl9ldmVudHMubmV3TGlzdGVuZXIpXG4gICAgdGhpcy5lbWl0KCduZXdMaXN0ZW5lcicsIHR5cGUsXG4gICAgICAgICAgICAgIGlzRnVuY3Rpb24obGlzdGVuZXIubGlzdGVuZXIpID9cbiAgICAgICAgICAgICAgbGlzdGVuZXIubGlzdGVuZXIgOiBsaXN0ZW5lcik7XG5cbiAgaWYgKCF0aGlzLl9ldmVudHNbdHlwZV0pXG4gICAgLy8gT3B0aW1pemUgdGhlIGNhc2Ugb2Ygb25lIGxpc3RlbmVyLiBEb24ndCBuZWVkIHRoZSBleHRyYSBhcnJheSBvYmplY3QuXG4gICAgdGhpcy5fZXZlbnRzW3R5cGVdID0gbGlzdGVuZXI7XG4gIGVsc2UgaWYgKGlzT2JqZWN0KHRoaXMuX2V2ZW50c1t0eXBlXSkpXG4gICAgLy8gSWYgd2UndmUgYWxyZWFkeSBnb3QgYW4gYXJyYXksIGp1c3QgYXBwZW5kLlxuICAgIHRoaXMuX2V2ZW50c1t0eXBlXS5wdXNoKGxpc3RlbmVyKTtcbiAgZWxzZVxuICAgIC8vIEFkZGluZyB0aGUgc2Vjb25kIGVsZW1lbnQsIG5lZWQgdG8gY2hhbmdlIHRvIGFycmF5LlxuICAgIHRoaXMuX2V2ZW50c1t0eXBlXSA9IFt0aGlzLl9ldmVudHNbdHlwZV0sIGxpc3RlbmVyXTtcblxuICAvLyBDaGVjayBmb3IgbGlzdGVuZXIgbGVha1xuICBpZiAoaXNPYmplY3QodGhpcy5fZXZlbnRzW3R5cGVdKSAmJiAhdGhpcy5fZXZlbnRzW3R5cGVdLndhcm5lZCkge1xuICAgIHZhciBtO1xuICAgIGlmICghaXNVbmRlZmluZWQodGhpcy5fbWF4TGlzdGVuZXJzKSkge1xuICAgICAgbSA9IHRoaXMuX21heExpc3RlbmVycztcbiAgICB9IGVsc2Uge1xuICAgICAgbSA9IEV2ZW50RW1pdHRlci5kZWZhdWx0TWF4TGlzdGVuZXJzO1xuICAgIH1cblxuICAgIGlmIChtICYmIG0gPiAwICYmIHRoaXMuX2V2ZW50c1t0eXBlXS5sZW5ndGggPiBtKSB7XG4gICAgICB0aGlzLl9ldmVudHNbdHlwZV0ud2FybmVkID0gdHJ1ZTtcbiAgICAgIGNvbnNvbGUuZXJyb3IoJyhub2RlKSB3YXJuaW5nOiBwb3NzaWJsZSBFdmVudEVtaXR0ZXIgbWVtb3J5ICcgK1xuICAgICAgICAgICAgICAgICAgICAnbGVhayBkZXRlY3RlZC4gJWQgbGlzdGVuZXJzIGFkZGVkLiAnICtcbiAgICAgICAgICAgICAgICAgICAgJ1VzZSBlbWl0dGVyLnNldE1heExpc3RlbmVycygpIHRvIGluY3JlYXNlIGxpbWl0LicsXG4gICAgICAgICAgICAgICAgICAgIHRoaXMuX2V2ZW50c1t0eXBlXS5sZW5ndGgpO1xuICAgICAgaWYgKHR5cGVvZiBjb25zb2xlLnRyYWNlID09PSAnZnVuY3Rpb24nKSB7XG4gICAgICAgIC8vIG5vdCBzdXBwb3J0ZWQgaW4gSUUgMTBcbiAgICAgICAgY29uc29sZS50cmFjZSgpO1xuICAgICAgfVxuICAgIH1cbiAgfVxuXG4gIHJldHVybiB0aGlzO1xufTtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5vbiA9IEV2ZW50RW1pdHRlci5wcm90b3R5cGUuYWRkTGlzdGVuZXI7XG5cbkV2ZW50RW1pdHRlci5wcm90b3R5cGUub25jZSA9IGZ1bmN0aW9uKHR5cGUsIGxpc3RlbmVyKSB7XG4gIGlmICghaXNGdW5jdGlvbihsaXN0ZW5lcikpXG4gICAgdGhyb3cgVHlwZUVycm9yKCdsaXN0ZW5lciBtdXN0IGJlIGEgZnVuY3Rpb24nKTtcblxuICB2YXIgZmlyZWQgPSBmYWxzZTtcblxuICBmdW5jdGlvbiBnKCkge1xuICAgIHRoaXMucmVtb3ZlTGlzdGVuZXIodHlwZSwgZyk7XG5cbiAgICBpZiAoIWZpcmVkKSB7XG4gICAgICBmaXJlZCA9IHRydWU7XG4gICAgICBsaXN0ZW5lci5hcHBseSh0aGlzLCBhcmd1bWVudHMpO1xuICAgIH1cbiAgfVxuXG4gIGcubGlzdGVuZXIgPSBsaXN0ZW5lcjtcbiAgdGhpcy5vbih0eXBlLCBnKTtcblxuICByZXR1cm4gdGhpcztcbn07XG5cbi8vIGVtaXRzIGEgJ3JlbW92ZUxpc3RlbmVyJyBldmVudCBpZmYgdGhlIGxpc3RlbmVyIHdhcyByZW1vdmVkXG5FdmVudEVtaXR0ZXIucHJvdG90eXBlLnJlbW92ZUxpc3RlbmVyID0gZnVuY3Rpb24odHlwZSwgbGlzdGVuZXIpIHtcbiAgdmFyIGxpc3QsIHBvc2l0aW9uLCBsZW5ndGgsIGk7XG5cbiAgaWYgKCFpc0Z1bmN0aW9uKGxpc3RlbmVyKSlcbiAgICB0aHJvdyBUeXBlRXJyb3IoJ2xpc3RlbmVyIG11c3QgYmUgYSBmdW5jdGlvbicpO1xuXG4gIGlmICghdGhpcy5fZXZlbnRzIHx8ICF0aGlzLl9ldmVudHNbdHlwZV0pXG4gICAgcmV0dXJuIHRoaXM7XG5cbiAgbGlzdCA9IHRoaXMuX2V2ZW50c1t0eXBlXTtcbiAgbGVuZ3RoID0gbGlzdC5sZW5ndGg7XG4gIHBvc2l0aW9uID0gLTE7XG5cbiAgaWYgKGxpc3QgPT09IGxpc3RlbmVyIHx8XG4gICAgICAoaXNGdW5jdGlvbihsaXN0Lmxpc3RlbmVyKSAmJiBsaXN0Lmxpc3RlbmVyID09PSBsaXN0ZW5lcikpIHtcbiAgICBkZWxldGUgdGhpcy5fZXZlbnRzW3R5cGVdO1xuICAgIGlmICh0aGlzLl9ldmVudHMucmVtb3ZlTGlzdGVuZXIpXG4gICAgICB0aGlzLmVtaXQoJ3JlbW92ZUxpc3RlbmVyJywgdHlwZSwgbGlzdGVuZXIpO1xuXG4gIH0gZWxzZSBpZiAoaXNPYmplY3QobGlzdCkpIHtcbiAgICBmb3IgKGkgPSBsZW5ndGg7IGktLSA+IDA7KSB7XG4gICAgICBpZiAobGlzdFtpXSA9PT0gbGlzdGVuZXIgfHxcbiAgICAgICAgICAobGlzdFtpXS5saXN0ZW5lciAmJiBsaXN0W2ldLmxpc3RlbmVyID09PSBsaXN0ZW5lcikpIHtcbiAgICAgICAgcG9zaXRpb24gPSBpO1xuICAgICAgICBicmVhaztcbiAgICAgIH1cbiAgICB9XG5cbiAgICBpZiAocG9zaXRpb24gPCAwKVxuICAgICAgcmV0dXJuIHRoaXM7XG5cbiAgICBpZiAobGlzdC5sZW5ndGggPT09IDEpIHtcbiAgICAgIGxpc3QubGVuZ3RoID0gMDtcbiAgICAgIGRlbGV0ZSB0aGlzLl9ldmVudHNbdHlwZV07XG4gICAgfSBlbHNlIHtcbiAgICAgIGxpc3Quc3BsaWNlKHBvc2l0aW9uLCAxKTtcbiAgICB9XG5cbiAgICBpZiAodGhpcy5fZXZlbnRzLnJlbW92ZUxpc3RlbmVyKVxuICAgICAgdGhpcy5lbWl0KCdyZW1vdmVMaXN0ZW5lcicsIHR5cGUsIGxpc3RlbmVyKTtcbiAgfVxuXG4gIHJldHVybiB0aGlzO1xufTtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5yZW1vdmVBbGxMaXN0ZW5lcnMgPSBmdW5jdGlvbih0eXBlKSB7XG4gIHZhciBrZXksIGxpc3RlbmVycztcblxuICBpZiAoIXRoaXMuX2V2ZW50cylcbiAgICByZXR1cm4gdGhpcztcblxuICAvLyBub3QgbGlzdGVuaW5nIGZvciByZW1vdmVMaXN0ZW5lciwgbm8gbmVlZCB0byBlbWl0XG4gIGlmICghdGhpcy5fZXZlbnRzLnJlbW92ZUxpc3RlbmVyKSB7XG4gICAgaWYgKGFyZ3VtZW50cy5sZW5ndGggPT09IDApXG4gICAgICB0aGlzLl9ldmVudHMgPSB7fTtcbiAgICBlbHNlIGlmICh0aGlzLl9ldmVudHNbdHlwZV0pXG4gICAgICBkZWxldGUgdGhpcy5fZXZlbnRzW3R5cGVdO1xuICAgIHJldHVybiB0aGlzO1xuICB9XG5cbiAgLy8gZW1pdCByZW1vdmVMaXN0ZW5lciBmb3IgYWxsIGxpc3RlbmVycyBvbiBhbGwgZXZlbnRzXG4gIGlmIChhcmd1bWVudHMubGVuZ3RoID09PSAwKSB7XG4gICAgZm9yIChrZXkgaW4gdGhpcy5fZXZlbnRzKSB7XG4gICAgICBpZiAoa2V5ID09PSAncmVtb3ZlTGlzdGVuZXInKSBjb250aW51ZTtcbiAgICAgIHRoaXMucmVtb3ZlQWxsTGlzdGVuZXJzKGtleSk7XG4gICAgfVxuICAgIHRoaXMucmVtb3ZlQWxsTGlzdGVuZXJzKCdyZW1vdmVMaXN0ZW5lcicpO1xuICAgIHRoaXMuX2V2ZW50cyA9IHt9O1xuICAgIHJldHVybiB0aGlzO1xuICB9XG5cbiAgbGlzdGVuZXJzID0gdGhpcy5fZXZlbnRzW3R5cGVdO1xuXG4gIGlmIChpc0Z1bmN0aW9uKGxpc3RlbmVycykpIHtcbiAgICB0aGlzLnJlbW92ZUxpc3RlbmVyKHR5cGUsIGxpc3RlbmVycyk7XG4gIH0gZWxzZSB7XG4gICAgLy8gTElGTyBvcmRlclxuICAgIHdoaWxlIChsaXN0ZW5lcnMubGVuZ3RoKVxuICAgICAgdGhpcy5yZW1vdmVMaXN0ZW5lcih0eXBlLCBsaXN0ZW5lcnNbbGlzdGVuZXJzLmxlbmd0aCAtIDFdKTtcbiAgfVxuICBkZWxldGUgdGhpcy5fZXZlbnRzW3R5cGVdO1xuXG4gIHJldHVybiB0aGlzO1xufTtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5saXN0ZW5lcnMgPSBmdW5jdGlvbih0eXBlKSB7XG4gIHZhciByZXQ7XG4gIGlmICghdGhpcy5fZXZlbnRzIHx8ICF0aGlzLl9ldmVudHNbdHlwZV0pXG4gICAgcmV0ID0gW107XG4gIGVsc2UgaWYgKGlzRnVuY3Rpb24odGhpcy5fZXZlbnRzW3R5cGVdKSlcbiAgICByZXQgPSBbdGhpcy5fZXZlbnRzW3R5cGVdXTtcbiAgZWxzZVxuICAgIHJldCA9IHRoaXMuX2V2ZW50c1t0eXBlXS5zbGljZSgpO1xuICByZXR1cm4gcmV0O1xufTtcblxuRXZlbnRFbWl0dGVyLmxpc3RlbmVyQ291bnQgPSBmdW5jdGlvbihlbWl0dGVyLCB0eXBlKSB7XG4gIHZhciByZXQ7XG4gIGlmICghZW1pdHRlci5fZXZlbnRzIHx8ICFlbWl0dGVyLl9ldmVudHNbdHlwZV0pXG4gICAgcmV0ID0gMDtcbiAgZWxzZSBpZiAoaXNGdW5jdGlvbihlbWl0dGVyLl9ldmVudHNbdHlwZV0pKVxuICAgIHJldCA9IDE7XG4gIGVsc2VcbiAgICByZXQgPSBlbWl0dGVyLl9ldmVudHNbdHlwZV0ubGVuZ3RoO1xuICByZXR1cm4gcmV0O1xufTtcblxuZnVuY3Rpb24gaXNGdW5jdGlvbihhcmcpIHtcbiAgcmV0dXJuIHR5cGVvZiBhcmcgPT09ICdmdW5jdGlvbic7XG59XG5cbmZ1bmN0aW9uIGlzTnVtYmVyKGFyZykge1xuICByZXR1cm4gdHlwZW9mIGFyZyA9PT0gJ251bWJlcic7XG59XG5cbmZ1bmN0aW9uIGlzT2JqZWN0KGFyZykge1xuICByZXR1cm4gdHlwZW9mIGFyZyA9PT0gJ29iamVjdCcgJiYgYXJnICE9PSBudWxsO1xufVxuXG5mdW5jdGlvbiBpc1VuZGVmaW5lZChhcmcpIHtcbiAgcmV0dXJuIGFyZyA9PT0gdm9pZCAwO1xufVxuXG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiAuL34vZXZlbnRzL2V2ZW50cy5qc1xuICoqIG1vZHVsZSBpZCA9IDI4M1xuICoqIG1vZHVsZSBjaHVua3MgPSAxXG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciBOYXZMZWZ0QmFyID0gcmVxdWlyZSgnLi9uYXZMZWZ0QmFyJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHthY3Rpb25zLCBnZXR0ZXJzfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2FwcCcpO1xudmFyIFNlbGVjdE5vZGVEaWFsb2cgPSByZXF1aXJlKCcuL3NlbGVjdE5vZGVEaWFsb2cuanN4Jyk7XG5cbnZhciBBcHAgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIGFwcDogZ2V0dGVycy5hcHBTdGF0ZVxuICAgIH1cbiAgfSxcblxuICBjb21wb25lbnRXaWxsTW91bnQoKXtcbiAgICBhY3Rpb25zLmluaXRBcHAoKTtcbiAgICB0aGlzLnJlZnJlc2hJbnRlcnZhbCA9IHNldEludGVydmFsKGFjdGlvbnMuZmV0Y2hOb2Rlc0FuZFNlc3Npb25zLCAzNTAwMCk7XG4gIH0sXG5cbiAgY29tcG9uZW50V2lsbFVubW91bnQ6IGZ1bmN0aW9uKCkge1xuICAgIGNsZWFySW50ZXJ2YWwodGhpcy5yZWZyZXNoSW50ZXJ2YWwpO1xuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgaWYodGhpcy5zdGF0ZS5hcHAuaXNJbml0aWFsaXppbmcpe1xuICAgICAgcmV0dXJuIG51bGw7XG4gICAgfVxuXG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXRscHRcIj5cbiAgICAgICAgPE5hdkxlZnRCYXIvPlxuICAgICAgICA8U2VsZWN0Tm9kZURpYWxvZy8+XG4gICAgICAgIHt0aGlzLnByb3BzLkN1cnJlbnRTZXNzaW9uSG9zdH1cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJyb3dcIj5cbiAgICAgICAgICA8bmF2IGNsYXNzTmFtZT1cIlwiIHJvbGU9XCJuYXZpZ2F0aW9uXCIgc3R5bGU9e3sgbWFyZ2luQm90dG9tOiAwLCBmbG9hdDogXCJyaWdodFwiIH19PlxuICAgICAgICAgICAgPHVsIGNsYXNzTmFtZT1cIm5hdiBuYXZiYXItdG9wLWxpbmtzIG5hdmJhci1yaWdodFwiPlxuICAgICAgICAgICAgICA8bGk+XG4gICAgICAgICAgICAgICAgPGEgaHJlZj17Y2ZnLnJvdXRlcy5sb2dvdXR9PlxuICAgICAgICAgICAgICAgICAgPGkgY2xhc3NOYW1lPVwiZmEgZmEtc2lnbi1vdXRcIj48L2k+XG4gICAgICAgICAgICAgICAgICBMb2cgb3V0XG4gICAgICAgICAgICAgICAgPC9hPlxuICAgICAgICAgICAgICA8L2xpPlxuICAgICAgICAgICAgPC91bD5cbiAgICAgICAgICA8L25hdj5cbiAgICAgICAgPC9kaXY+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXBhZ2VcIj5cbiAgICAgICAgICB7dGhpcy5wcm9wcy5jaGlsZHJlbn1cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KVxuXG5tb2R1bGUuZXhwb3J0cyA9IEFwcDtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2FwcC5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHtnZXR0ZXJzLCBhY3Rpb25zfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsLycpO1xudmFyIFR0eSA9IHJlcXVpcmUoJ2FwcC9jb21tb24vdHR5Jyk7XG52YXIgVHR5VGVybWluYWwgPSByZXF1aXJlKCcuLy4uL3Rlcm1pbmFsLmpzeCcpO1xudmFyIEV2ZW50U3RyZWFtZXIgPSByZXF1aXJlKCcuL2V2ZW50U3RyZWFtZXIuanN4Jyk7XG52YXIgU2Vzc2lvbkxlZnRQYW5lbCA9IHJlcXVpcmUoJy4vc2Vzc2lvbkxlZnRQYW5lbCcpO1xudmFyIHtzaG93U2VsZWN0Tm9kZURpYWxvZywgY2xvc2VTZWxlY3ROb2RlRGlhbG9nfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2RpYWxvZ3MvYWN0aW9ucycpO1xudmFyIFNlbGVjdE5vZGVEaWFsb2cgPSByZXF1aXJlKCcuLy4uL3NlbGVjdE5vZGVEaWFsb2cuanN4Jyk7XG5cbnZhciBBY3RpdmVTZXNzaW9uID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50KCl7XG4gICAgY2xvc2VTZWxlY3ROb2RlRGlhbG9nKCk7XG4gIH0sXG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICBsZXQge3NlcnZlcklwLCBsb2dpbiwgcGFydGllc30gPSB0aGlzLnByb3BzLmFjdGl2ZVNlc3Npb247XG4gICAgbGV0IHNlcnZlckxhYmVsVGV4dCA9IGAke2xvZ2lufUAke3NlcnZlcklwfWA7XG5cbiAgICBpZighc2VydmVySXApe1xuICAgICAgc2VydmVyTGFiZWxUZXh0ID0gJyc7XG4gICAgfVxuXG4gICAgcmV0dXJuIChcbiAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtY3VycmVudC1zZXNzaW9uXCI+XG4gICAgICAgPFNlc3Npb25MZWZ0UGFuZWwgcGFydGllcz17cGFydGllc30vPlxuICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWN1cnJlbnQtc2Vzc2lvbi1zZXJ2ZXItaW5mb1wiPlxuICAgICAgICAgPHNwYW4gY2xhc3NOYW1lPVwiYnRuIGJ0bi1wcmltYXJ5IGJ0bi1zbVwiIG9uQ2xpY2s9e3Nob3dTZWxlY3ROb2RlRGlhbG9nfT5cbiAgICAgICAgICAgQ2hhbmdlIG5vZGVcbiAgICAgICAgIDwvc3Bhbj5cbiAgICAgICAgIDxoMz57c2VydmVyTGFiZWxUZXh0fTwvaDM+XG4gICAgICAgPC9kaXY+XG4gICAgICAgPFR0eUNvbm5lY3Rpb24gey4uLnRoaXMucHJvcHMuYWN0aXZlU2Vzc2lvbn0gLz5cbiAgICAgPC9kaXY+XG4gICAgICk7XG4gIH1cbn0pO1xuXG52YXIgVHR5Q29ubmVjdGlvbiA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgdGhpcy50dHkgPSBuZXcgVHR5KHRoaXMucHJvcHMpXG4gICAgdGhpcy50dHkub24oJ29wZW4nLCAoKT0+IHRoaXMuc2V0U3RhdGUoeyAuLi50aGlzLnN0YXRlLCBpc0Nvbm5lY3RlZDogdHJ1ZSB9KSk7XG5cbiAgICB2YXIge3NlcnZlcklkLCBsb2dpbn0gPSB0aGlzLnByb3BzO1xuICAgIHJldHVybiB7c2VydmVySWQsIGxvZ2luLCBpc0Nvbm5lY3RlZDogZmFsc2V9O1xuICB9LFxuXG4gIGNvbXBvbmVudERpZE1vdW50KCl7XG4gICAgLy8gdGVtcG9yYXJ5IGhhY2tcbiAgICBTZWxlY3ROb2RlRGlhbG9nLm9uU2VydmVyQ2hhbmdlQ2FsbEJhY2sgPSB0aGlzLmNvbXBvbmVudFdpbGxSZWNlaXZlUHJvcHMuYmluZCh0aGlzKTtcbiAgfSxcblxuICBjb21wb25lbnRXaWxsVW5tb3VudCgpIHtcbiAgICBTZWxlY3ROb2RlRGlhbG9nLm9uU2VydmVyQ2hhbmdlQ2FsbEJhY2sgPSBudWxsO1xuICAgIHRoaXMudHR5LmRpc2Nvbm5lY3QoKTtcbiAgfSxcblxuICBjb21wb25lbnRXaWxsUmVjZWl2ZVByb3BzKG5leHRQcm9wcyl7XG4gICAgdmFyIHtzZXJ2ZXJJZH0gPSBuZXh0UHJvcHM7XG4gICAgaWYoc2VydmVySWQgJiYgc2VydmVySWQgIT09IHRoaXMuc3RhdGUuc2VydmVySWQpe1xuICAgICAgdGhpcy50dHkucmVjb25uZWN0KHtzZXJ2ZXJJZH0pO1xuICAgICAgdGhpcy5yZWZzLnR0eUNtbnRJbnN0YW5jZS50ZXJtLmZvY3VzKCk7XG4gICAgICB0aGlzLnNldFN0YXRlKHsuLi50aGlzLnN0YXRlLCBzZXJ2ZXJJZCB9KTtcbiAgICB9XG4gIH0sXG5cbiAgcmVuZGVyKCkge1xuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IHN0eWxlPXt7aGVpZ2h0OiAnMTAwJSd9fT5cbiAgICAgICAgPFR0eVRlcm1pbmFsIHJlZj1cInR0eUNtbnRJbnN0YW5jZVwiIHR0eT17dGhpcy50dHl9IGNvbHM9e3RoaXMucHJvcHMuY29sc30gcm93cz17dGhpcy5wcm9wcy5yb3dzfSAvPlxuICAgICAgICB7IHRoaXMuc3RhdGUuaXNDb25uZWN0ZWQgPyA8RXZlbnRTdHJlYW1lciBzaWQ9e3RoaXMucHJvcHMuc2lkfS8+IDogbnVsbCB9XG4gICAgICA8L2Rpdj5cbiAgICApXG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IEFjdGl2ZVNlc3Npb247XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9jdXJyZW50U2Vzc2lvbi9hY3RpdmVTZXNzaW9uLmpzeFxuICoqLyIsInZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCdhcHAvc2Vzc2lvbicpO1xudmFyIHt1cGRhdGVTZXNzaW9ufSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3Nlc3Npb25zL2FjdGlvbnMnKTtcblxudmFyIEV2ZW50U3RyZWFtZXIgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIGNvbXBvbmVudERpZE1vdW50KCkge1xuICAgIGxldCB7c2lkfSA9IHRoaXMucHJvcHM7XG4gICAgbGV0IHt0b2tlbn0gPSBzZXNzaW9uLmdldFVzZXJEYXRhKCk7XG4gICAgbGV0IGNvbm5TdHIgPSBjZmcuYXBpLmdldEV2ZW50U3RyZWFtQ29ublN0cih0b2tlbiwgc2lkKTtcblxuICAgIHRoaXMuc29ja2V0ID0gbmV3IFdlYlNvY2tldChjb25uU3RyLCAncHJvdG8nKTtcbiAgICB0aGlzLnNvY2tldC5vbm1lc3NhZ2UgPSAoZXZlbnQpID0+IHtcbiAgICAgIHRyeVxuICAgICAge1xuICAgICAgICBsZXQganNvbiA9IEpTT04ucGFyc2UoZXZlbnQuZGF0YSk7XG4gICAgICAgIHVwZGF0ZVNlc3Npb24oanNvbi5zZXNzaW9uKTtcbiAgICAgIH1cbiAgICAgIGNhdGNoKGVycil7XG4gICAgICAgIGNvbnNvbGUubG9nKCdmYWlsZWQgdG8gcGFyc2UgZXZlbnQgc3RyZWFtIGRhdGEnKTtcbiAgICAgIH1cblxuICAgIH07XG4gICAgdGhpcy5zb2NrZXQub25jbG9zZSA9ICgpID0+IHt9O1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50KCkge1xuICAgIHRoaXMuc29ja2V0LmNsb3NlKCk7XG4gIH0sXG5cbiAgc2hvdWxkQ29tcG9uZW50VXBkYXRlKCkge1xuICAgIHJldHVybiBmYWxzZTtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuIG51bGw7XG4gIH1cbn0pO1xuXG5leHBvcnQgZGVmYXVsdCBFdmVudFN0cmVhbWVyO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvY3VycmVudFNlc3Npb24vZXZlbnRTdHJlYW1lci5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHtnZXR0ZXJzLCBhY3Rpb25zfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsLycpO1xudmFyIE5vdEZvdW5kUGFnZSA9IHJlcXVpcmUoJ2FwcC9jb21wb25lbnRzL25vdEZvdW5kUGFnZS5qc3gnKTtcbnZhciBTZXNzaW9uUGxheWVyID0gcmVxdWlyZSgnLi9zZXNzaW9uUGxheWVyLmpzeCcpO1xudmFyIEFjdGl2ZVNlc3Npb24gPSByZXF1aXJlKCcuL2FjdGl2ZVNlc3Npb24uanN4Jyk7XG5cbnZhciBDdXJyZW50U2Vzc2lvbkhvc3QgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIGN1cnJlbnRTZXNzaW9uOiBnZXR0ZXJzLmFjdGl2ZVNlc3Npb25cbiAgICB9XG4gIH0sXG5cbiAgY29tcG9uZW50RGlkTW91bnQoKXtcbiAgICB2YXIgeyBzaWQgfSA9IHRoaXMucHJvcHMucGFyYW1zO1xuICAgIGlmKCF0aGlzLnN0YXRlLmN1cnJlbnRTZXNzaW9uKXtcbiAgICAgIGFjdGlvbnMub3BlblNlc3Npb24oc2lkKTtcbiAgICB9XG4gIH0sXG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICB2YXIgY3VycmVudFNlc3Npb24gPSB0aGlzLnN0YXRlLmN1cnJlbnRTZXNzaW9uO1xuICAgIGlmKCFjdXJyZW50U2Vzc2lvbil7XG4gICAgICByZXR1cm4gbnVsbDtcbiAgICB9XG5cbiAgICBpZihjdXJyZW50U2Vzc2lvbi5pc05ld1Nlc3Npb24gfHwgY3VycmVudFNlc3Npb24uYWN0aXZlKXtcbiAgICAgIHJldHVybiA8QWN0aXZlU2Vzc2lvbiBhY3RpdmVTZXNzaW9uPXtjdXJyZW50U2Vzc2lvbn0vPjtcbiAgICB9XG5cbiAgICByZXR1cm4gPFNlc3Npb25QbGF5ZXIgYWN0aXZlU2Vzc2lvbj17Y3VycmVudFNlc3Npb259Lz47XG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IEN1cnJlbnRTZXNzaW9uSG9zdDtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2N1cnJlbnRTZXNzaW9uL21haW4uanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciBSZWFjdFNsaWRlciA9IHJlcXVpcmUoJ3JlYWN0LXNsaWRlcicpO1xudmFyIFR0eVBsYXllciA9IHJlcXVpcmUoJ2FwcC9jb21tb24vdHR5UGxheWVyJylcbnZhciBUdHlUZXJtaW5hbCA9IHJlcXVpcmUoJy4vLi4vdGVybWluYWwuanN4Jyk7XG52YXIgU2Vzc2lvbkxlZnRQYW5lbCA9IHJlcXVpcmUoJy4vc2Vzc2lvbkxlZnRQYW5lbCcpO1xuXG52YXIgU2Vzc2lvblBsYXllciA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgY2FsY3VsYXRlU3RhdGUoKXtcbiAgICByZXR1cm4ge1xuICAgICAgbGVuZ3RoOiB0aGlzLnR0eS5sZW5ndGgsXG4gICAgICBtaW46IDEsXG4gICAgICBpc1BsYXlpbmc6IHRoaXMudHR5LmlzUGxheWluZyxcbiAgICAgIGN1cnJlbnQ6IHRoaXMudHR5LmN1cnJlbnQsXG4gICAgICBjYW5QbGF5OiB0aGlzLnR0eS5sZW5ndGggPiAxXG4gICAgfTtcbiAgfSxcblxuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgdmFyIHNpZCA9IHRoaXMucHJvcHMuYWN0aXZlU2Vzc2lvbi5zaWQ7XG4gICAgdGhpcy50dHkgPSBuZXcgVHR5UGxheWVyKHtzaWR9KTtcbiAgICByZXR1cm4gdGhpcy5jYWxjdWxhdGVTdGF0ZSgpO1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50KCkge1xuICAgIHRoaXMudHR5LnN0b3AoKTtcbiAgICB0aGlzLnR0eS5yZW1vdmVBbGxMaXN0ZW5lcnMoKTtcbiAgfSxcblxuICBjb21wb25lbnREaWRNb3VudCgpIHtcbiAgICB0aGlzLnR0eS5vbignY2hhbmdlJywgKCk9PntcbiAgICAgIHZhciBuZXdTdGF0ZSA9IHRoaXMuY2FsY3VsYXRlU3RhdGUoKTtcbiAgICAgIHRoaXMuc2V0U3RhdGUobmV3U3RhdGUpO1xuICAgIH0pO1xuICB9LFxuXG4gIHRvZ2dsZVBsYXlTdG9wKCl7XG4gICAgaWYodGhpcy5zdGF0ZS5pc1BsYXlpbmcpe1xuICAgICAgdGhpcy50dHkuc3RvcCgpO1xuICAgIH1lbHNle1xuICAgICAgdGhpcy50dHkucGxheSgpO1xuICAgIH1cbiAgfSxcblxuICBtb3ZlKHZhbHVlKXtcbiAgICB0aGlzLnR0eS5tb3ZlKHZhbHVlKTtcbiAgfSxcblxuICBvbkJlZm9yZUNoYW5nZSgpe1xuICAgIHRoaXMudHR5LnN0b3AoKTtcbiAgfSxcblxuICBvbkFmdGVyQ2hhbmdlKHZhbHVlKXtcbiAgICB0aGlzLnR0eS5wbGF5KCk7XG4gICAgdGhpcy50dHkubW92ZSh2YWx1ZSk7XG4gIH0sXG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICB2YXIge2lzUGxheWluZ30gPSB0aGlzLnN0YXRlO1xuXG4gICAgcmV0dXJuIChcbiAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtY3VycmVudC1zZXNzaW9uIGdydi1zZXNzaW9uLXBsYXllclwiPlxuICAgICAgIDxTZXNzaW9uTGVmdFBhbmVsLz5cbiAgICAgICA8VHR5VGVybWluYWwgcmVmPVwidGVybVwiIHR0eT17dGhpcy50dHl9IGNvbHM9XCI1XCIgcm93cz1cIjVcIiAvPlxuICAgICAgIDxSZWFjdFNsaWRlclxuICAgICAgICAgIG1pbj17dGhpcy5zdGF0ZS5taW59XG4gICAgICAgICAgbWF4PXt0aGlzLnN0YXRlLmxlbmd0aH1cbiAgICAgICAgICB2YWx1ZT17dGhpcy5zdGF0ZS5jdXJyZW50fSAgICBcbiAgICAgICAgICBvbkFmdGVyQ2hhbmdlPXt0aGlzLm9uQWZ0ZXJDaGFuZ2V9XG4gICAgICAgICAgb25CZWZvcmVDaGFuZ2U9e3RoaXMub25CZWZvcmVDaGFuZ2V9XG4gICAgICAgICAgZGVmYXVsdFZhbHVlPXsxfVxuICAgICAgICAgIHdpdGhCYXJzXG4gICAgICAgICAgY2xhc3NOYW1lPVwiZ3J2LXNsaWRlclwiPlxuICAgICAgIDwvUmVhY3RTbGlkZXI+XG4gICAgICAgPGJ1dHRvbiBjbGFzc05hbWU9XCJidG5cIiBvbkNsaWNrPXt0aGlzLnRvZ2dsZVBsYXlTdG9wfT5cbiAgICAgICAgIHsgaXNQbGF5aW5nID8gPGkgY2xhc3NOYW1lPVwiZmEgZmEtc3RvcFwiPjwvaT4gOiAgPGkgY2xhc3NOYW1lPVwiZmEgZmEtcGxheVwiPjwvaT4gfVxuICAgICAgIDwvYnV0dG9uPlxuICAgICA8L2Rpdj5cbiAgICAgKTtcbiAgfVxufSk7XG5cbmV4cG9ydCBkZWZhdWx0IFNlc3Npb25QbGF5ZXI7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9jdXJyZW50U2Vzc2lvbi9zZXNzaW9uUGxheWVyLmpzeFxuICoqLyIsIm1vZHVsZS5leHBvcnRzLkFwcCA9IHJlcXVpcmUoJy4vYXBwLmpzeCcpO1xubW9kdWxlLmV4cG9ydHMuTG9naW4gPSByZXF1aXJlKCcuL2xvZ2luLmpzeCcpO1xubW9kdWxlLmV4cG9ydHMuTmV3VXNlciA9IHJlcXVpcmUoJy4vbmV3VXNlci5qc3gnKTtcbm1vZHVsZS5leHBvcnRzLk5vZGVzID0gcmVxdWlyZSgnLi9ub2Rlcy9tYWluLmpzeCcpO1xubW9kdWxlLmV4cG9ydHMuU2Vzc2lvbnMgPSByZXF1aXJlKCcuL3Nlc3Npb25zL21haW4uanN4Jyk7XG5tb2R1bGUuZXhwb3J0cy5DdXJyZW50U2Vzc2lvbkhvc3QgPSByZXF1aXJlKCcuL2N1cnJlbnRTZXNzaW9uL21haW4uanN4Jyk7XG5tb2R1bGUuZXhwb3J0cy5Ob3RGb3VuZFBhZ2UgPSByZXF1aXJlKCcuL25vdEZvdW5kUGFnZS5qc3gnKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2luZGV4LmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgJCA9IHJlcXVpcmUoJ2pRdWVyeScpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIExpbmtlZFN0YXRlTWl4aW4gPSByZXF1aXJlKCdyZWFjdC1hZGRvbnMtbGlua2VkLXN0YXRlLW1peGluJyk7XG52YXIge2FjdGlvbnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvdXNlcicpO1xudmFyIEdvb2dsZUF1dGhJbmZvID0gcmVxdWlyZSgnLi9nb29nbGVBdXRoTG9nbycpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxudmFyIExvZ2luSW5wdXRGb3JtID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW0xpbmtlZFN0YXRlTWl4aW5dLFxuXG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4ge1xuICAgICAgdXNlcjogJycsXG4gICAgICBwYXNzd29yZDogJycsXG4gICAgICB0b2tlbjogJydcbiAgICB9XG4gIH0sXG5cbiAgb25DbGljazogZnVuY3Rpb24oZSkge1xuICAgIGUucHJldmVudERlZmF1bHQoKTtcbiAgICBpZiAodGhpcy5pc1ZhbGlkKCkpIHtcbiAgICAgIHRoaXMucHJvcHMub25DbGljayh0aGlzLnN0YXRlKTtcbiAgICB9XG4gIH0sXG5cbiAgaXNWYWxpZDogZnVuY3Rpb24oKSB7XG4gICAgdmFyICRmb3JtID0gJCh0aGlzLnJlZnMuZm9ybSk7XG4gICAgcmV0dXJuICRmb3JtLmxlbmd0aCA9PT0gMCB8fCAkZm9ybS52YWxpZCgpO1xuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICByZXR1cm4gKFxuICAgICAgPGZvcm0gcmVmPVwiZm9ybVwiIGNsYXNzTmFtZT1cImdydi1sb2dpbi1pbnB1dC1mb3JtXCI+XG4gICAgICAgIDxoMz4gV2VsY29tZSB0byBUZWxlcG9ydCA8L2gzPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0IGF1dG9Gb2N1cyB2YWx1ZUxpbms9e3RoaXMubGlua1N0YXRlKCd1c2VyJyl9IGNsYXNzTmFtZT1cImZvcm0tY29udHJvbCByZXF1aXJlZFwiIHBsYWNlaG9sZGVyPVwiVXNlciBuYW1lXCIgbmFtZT1cInVzZXJOYW1lXCIgLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImZvcm0tZ3JvdXBcIj5cbiAgICAgICAgICAgIDxpbnB1dCB2YWx1ZUxpbms9e3RoaXMubGlua1N0YXRlKCdwYXNzd29yZCcpfSB0eXBlPVwicGFzc3dvcmRcIiBuYW1lPVwicGFzc3dvcmRcIiBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2wgcmVxdWlyZWRcIiBwbGFjZWhvbGRlcj1cIlBhc3N3b3JkXCIvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0IHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3Rva2VuJyl9IGNsYXNzTmFtZT1cImZvcm0tY29udHJvbCByZXF1aXJlZFwiIG5hbWU9XCJ0b2tlblwiIHBsYWNlaG9sZGVyPVwiVHdvIGZhY3RvciB0b2tlbiAoR29vZ2xlIEF1dGhlbnRpY2F0b3IpXCIvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxidXR0b24gdHlwZT1cInN1Ym1pdFwiIGNsYXNzTmFtZT1cImJ0biBidG4tcHJpbWFyeSBibG9jayBmdWxsLXdpZHRoIG0tYlwiIG9uQ2xpY2s9e3RoaXMub25DbGlja30+TG9naW48L2J1dHRvbj5cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Zvcm0+XG4gICAgKTtcbiAgfVxufSlcblxudmFyIExvZ2luID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW3JlYWN0b3IuUmVhY3RNaXhpbl0sXG5cbiAgZ2V0RGF0YUJpbmRpbmdzKCkge1xuICAgIHJldHVybiB7XG4gICAgfVxuICB9LFxuXG4gIG9uQ2xpY2soaW5wdXREYXRhKXtcbiAgICB2YXIgbG9jID0gdGhpcy5wcm9wcy5sb2NhdGlvbjtcbiAgICB2YXIgcmVkaXJlY3QgPSBjZmcucm91dGVzLmFwcDtcblxuICAgIGlmKGxvYy5zdGF0ZSAmJiBsb2Muc3RhdGUucmVkaXJlY3RUbyl7XG4gICAgICByZWRpcmVjdCA9IGxvYy5zdGF0ZS5yZWRpcmVjdFRvO1xuICAgIH1cblxuICAgIGFjdGlvbnMubG9naW4oaW5wdXREYXRhLCByZWRpcmVjdCk7XG4gIH0sXG5cbiAgcmVuZGVyKCkge1xuICAgIHZhciBpc1Byb2Nlc3NpbmcgPSBmYWxzZTsvL3RoaXMuc3RhdGUudXNlclJlcXVlc3QuZ2V0KCdpc0xvYWRpbmcnKTtcbiAgICB2YXIgaXNFcnJvciA9IGZhbHNlOy8vdGhpcy5zdGF0ZS51c2VyUmVxdWVzdC5nZXQoJ2lzRXJyb3InKTtcbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbG9naW4gdGV4dC1jZW50ZXJcIj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbG9nby10cHJ0XCI+PC9kaXY+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWNvbnRlbnQgZ3J2LWZsZXhcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LWNvbHVtblwiPlxuICAgICAgICAgICAgPExvZ2luSW5wdXRGb3JtIG9uQ2xpY2s9e3RoaXMub25DbGlja30vPlxuICAgICAgICAgICAgPEdvb2dsZUF1dGhJbmZvLz5cbiAgICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWxvZ2luLWluZm9cIj5cbiAgICAgICAgICAgICAgPGkgY2xhc3NOYW1lPVwiZmEgZmEtcXVlc3Rpb25cIj48L2k+XG4gICAgICAgICAgICAgIDxzdHJvbmc+TmV3IEFjY291bnQgb3IgZm9yZ290IHBhc3N3b3JkPzwvc3Ryb25nPlxuICAgICAgICAgICAgICA8ZGl2PkFzayBmb3IgYXNzaXN0YW5jZSBmcm9tIHlvdXIgQ29tcGFueSBhZG1pbmlzdHJhdG9yPC9kaXY+XG4gICAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBMb2dpbjtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2xvZ2luLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgeyBSb3V0ZXIsIEluZGV4TGluaywgSGlzdG9yeSB9ID0gcmVxdWlyZSgncmVhY3Qtcm91dGVyJyk7XG52YXIgZ2V0dGVycyA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3VzZXIvZ2V0dGVycycpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxudmFyIG1lbnVJdGVtcyA9IFtcbiAge2ljb246ICdmYSBmYS1jb2dzJywgdG86IGNmZy5yb3V0ZXMubm9kZXMsIHRpdGxlOiAnTm9kZXMnfSxcbiAge2ljb246ICdmYSBmYS1zaXRlbWFwJywgdG86IGNmZy5yb3V0ZXMuc2Vzc2lvbnMsIHRpdGxlOiAnU2Vzc2lvbnMnfVxuXTtcblxudmFyIE5hdkxlZnRCYXIgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpe1xuICAgIHZhciBpdGVtcyA9IG1lbnVJdGVtcy5tYXAoKGksIGluZGV4KT0+e1xuICAgICAgdmFyIGNsYXNzTmFtZSA9IHRoaXMuY29udGV4dC5yb3V0ZXIuaXNBY3RpdmUoaS50bykgPyAnYWN0aXZlJyA6ICcnO1xuICAgICAgcmV0dXJuIChcbiAgICAgICAgPGxpIGtleT17aW5kZXh9IGNsYXNzTmFtZT17Y2xhc3NOYW1lfT5cbiAgICAgICAgICA8SW5kZXhMaW5rIHRvPXtpLnRvfT5cbiAgICAgICAgICAgIDxpIGNsYXNzTmFtZT17aS5pY29ufSB0aXRsZT17aS50aXRsZX0vPlxuICAgICAgICAgIDwvSW5kZXhMaW5rPlxuICAgICAgICA8L2xpPlxuICAgICAgKTtcbiAgICB9KTtcblxuICAgIGl0ZW1zLnB1c2goKFxuICAgICAgPGxpIGtleT17bWVudUl0ZW1zLmxlbmd0aH0+XG4gICAgICAgIDxhIGhyZWY9e2NmZy5oZWxwVXJsfT5cbiAgICAgICAgICA8aSBjbGFzc05hbWU9XCJmYSBmYS1xdWVzdGlvblwiIHRpdGxlPVwiaGVscFwiLz5cbiAgICAgICAgPC9hPlxuICAgICAgPC9saT4pKTtcblxuICAgIHJldHVybiAoXG4gICAgICA8bmF2IGNsYXNzTmFtZT0nZ3J2LW5hdiBuYXZiYXItZGVmYXVsdCBuYXZiYXItc3RhdGljLXNpZGUnIHJvbGU9J25hdmlnYXRpb24nPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT0nJz5cbiAgICAgICAgICA8dWwgY2xhc3NOYW1lPSduYXYnIGlkPSdzaWRlLW1lbnUnPlxuICAgICAgICAgICAgPGxpPjxkaXYgY2xhc3NOYW1lPVwiZ3J2LWNpcmNsZSB0ZXh0LXVwcGVyY2FzZVwiPjxzcGFuPntnZXRVc2VyTmFtZUxldHRlcigpfTwvc3Bhbj48L2Rpdj48L2xpPlxuICAgICAgICAgICAge2l0ZW1zfVxuICAgICAgICAgIDwvdWw+XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9uYXY+XG4gICAgKTtcbiAgfVxufSk7XG5cbk5hdkxlZnRCYXIuY29udGV4dFR5cGVzID0ge1xuICByb3V0ZXI6IFJlYWN0LlByb3BUeXBlcy5vYmplY3QuaXNSZXF1aXJlZFxufVxuXG5mdW5jdGlvbiBnZXRVc2VyTmFtZUxldHRlcigpe1xuICB2YXIge3Nob3J0RGlzcGxheU5hbWV9ID0gcmVhY3Rvci5ldmFsdWF0ZShnZXR0ZXJzLnVzZXIpO1xuICByZXR1cm4gc2hvcnREaXNwbGF5TmFtZTtcbn1cblxubW9kdWxlLmV4cG9ydHMgPSBOYXZMZWZ0QmFyO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvbmF2TGVmdEJhci5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyICQgPSByZXF1aXJlKCdqUXVlcnknKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7YWN0aW9ucywgZ2V0dGVyc30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9pbnZpdGUnKTtcbnZhciB1c2VyTW9kdWxlID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvdXNlcicpO1xudmFyIExpbmtlZFN0YXRlTWl4aW4gPSByZXF1aXJlKCdyZWFjdC1hZGRvbnMtbGlua2VkLXN0YXRlLW1peGluJyk7XG52YXIgR29vZ2xlQXV0aEluZm8gPSByZXF1aXJlKCcuL2dvb2dsZUF1dGhMb2dvJyk7XG5cbnZhciBJbnZpdGVJbnB1dEZvcm0gPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbTGlua2VkU3RhdGVNaXhpbl0sXG5cbiAgY29tcG9uZW50RGlkTW91bnQoKXtcbiAgICAkKHRoaXMucmVmcy5mb3JtKS52YWxpZGF0ZSh7XG4gICAgICBydWxlczp7XG4gICAgICAgIHBhc3N3b3JkOntcbiAgICAgICAgICBtaW5sZW5ndGg6IDYsXG4gICAgICAgICAgcmVxdWlyZWQ6IHRydWVcbiAgICAgICAgfSxcbiAgICAgICAgcGFzc3dvcmRDb25maXJtZWQ6e1xuICAgICAgICAgIHJlcXVpcmVkOiB0cnVlLFxuICAgICAgICAgIGVxdWFsVG86IHRoaXMucmVmcy5wYXNzd29yZFxuICAgICAgICB9XG4gICAgICB9LFxuXG4gICAgICBtZXNzYWdlczoge1xuICBcdFx0XHRwYXNzd29yZENvbmZpcm1lZDoge1xuICBcdFx0XHRcdG1pbmxlbmd0aDogJC52YWxpZGF0b3IuZm9ybWF0KCdFbnRlciBhdCBsZWFzdCB7MH0gY2hhcmFjdGVycycpLFxuICBcdFx0XHRcdGVxdWFsVG86ICdFbnRlciB0aGUgc2FtZSBwYXNzd29yZCBhcyBhYm92ZSdcbiAgXHRcdFx0fVxuICAgICAgfVxuICAgIH0pXG4gIH0sXG5cbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB7XG4gICAgICBuYW1lOiB0aGlzLnByb3BzLmludml0ZS51c2VyLFxuICAgICAgcHN3OiAnJyxcbiAgICAgIHBzd0NvbmZpcm1lZDogJycsXG4gICAgICB0b2tlbjogJydcbiAgICB9XG4gIH0sXG5cbiAgb25DbGljayhlKSB7XG4gICAgZS5wcmV2ZW50RGVmYXVsdCgpO1xuICAgIGlmICh0aGlzLmlzVmFsaWQoKSkge1xuICAgICAgdXNlck1vZHVsZS5hY3Rpb25zLnNpZ25VcCh7XG4gICAgICAgIG5hbWU6IHRoaXMuc3RhdGUubmFtZSxcbiAgICAgICAgcHN3OiB0aGlzLnN0YXRlLnBzdyxcbiAgICAgICAgdG9rZW46IHRoaXMuc3RhdGUudG9rZW4sXG4gICAgICAgIGludml0ZVRva2VuOiB0aGlzLnByb3BzLmludml0ZS5pbnZpdGVfdG9rZW59KTtcbiAgICB9XG4gIH0sXG5cbiAgaXNWYWxpZCgpIHtcbiAgICB2YXIgJGZvcm0gPSAkKHRoaXMucmVmcy5mb3JtKTtcbiAgICByZXR1cm4gJGZvcm0ubGVuZ3RoID09PSAwIHx8ICRmb3JtLnZhbGlkKCk7XG4gIH0sXG5cbiAgcmVuZGVyKCkge1xuICAgIHJldHVybiAoXG4gICAgICA8Zm9ybSByZWY9XCJmb3JtXCIgY2xhc3NOYW1lPVwiZ3J2LWludml0ZS1pbnB1dC1mb3JtXCI+XG4gICAgICAgIDxoMz4gR2V0IHN0YXJ0ZWQgd2l0aCBUZWxlcG9ydCA8L2gzPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0XG4gICAgICAgICAgICAgIHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ25hbWUnKX1cbiAgICAgICAgICAgICAgbmFtZT1cInVzZXJOYW1lXCJcbiAgICAgICAgICAgICAgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sIHJlcXVpcmVkXCJcbiAgICAgICAgICAgICAgcGxhY2Vob2xkZXI9XCJVc2VyIG5hbWVcIi8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXRcbiAgICAgICAgICAgICAgYXV0b0ZvY3VzXG4gICAgICAgICAgICAgIHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3BzdycpfVxuICAgICAgICAgICAgICByZWY9XCJwYXNzd29yZFwiXG4gICAgICAgICAgICAgIHR5cGU9XCJwYXNzd29yZFwiXG4gICAgICAgICAgICAgIG5hbWU9XCJwYXNzd29yZFwiXG4gICAgICAgICAgICAgIGNsYXNzTmFtZT1cImZvcm0tY29udHJvbFwiXG4gICAgICAgICAgICAgIHBsYWNlaG9sZGVyPVwiUGFzc3dvcmRcIiAvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0XG4gICAgICAgICAgICAgIHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3Bzd0NvbmZpcm1lZCcpfVxuICAgICAgICAgICAgICB0eXBlPVwicGFzc3dvcmRcIlxuICAgICAgICAgICAgICBuYW1lPVwicGFzc3dvcmRDb25maXJtZWRcIlxuICAgICAgICAgICAgICBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2xcIlxuICAgICAgICAgICAgICBwbGFjZWhvbGRlcj1cIlBhc3N3b3JkIGNvbmZpcm1cIi8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXRcbiAgICAgICAgICAgICAgbmFtZT1cInRva2VuXCIgICAgICAgICAgICAgIFxuICAgICAgICAgICAgICB2YWx1ZUxpbms9e3RoaXMubGlua1N0YXRlKCd0b2tlbicpfVxuICAgICAgICAgICAgICBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2wgcmVxdWlyZWRcIlxuICAgICAgICAgICAgICBwbGFjZWhvbGRlcj1cIlR3byBmYWN0b3IgdG9rZW4gKEdvb2dsZSBBdXRoZW50aWNhdG9yKVwiIC8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGJ1dHRvbiB0eXBlPVwic3VibWl0XCIgZGlzYWJsZWQ9e3RoaXMucHJvcHMuYXR0ZW1wLmlzUHJvY2Vzc2luZ30gY2xhc3NOYW1lPVwiYnRuIGJ0bi1wcmltYXJ5IGJsb2NrIGZ1bGwtd2lkdGggbS1iXCIgb25DbGljaz17dGhpcy5vbkNsaWNrfSA+U2lnbiB1cDwvYnV0dG9uPlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZm9ybT5cbiAgICApO1xuICB9XG59KVxuXG52YXIgSW52aXRlID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW3JlYWN0b3IuUmVhY3RNaXhpbl0sXG5cbiAgZ2V0RGF0YUJpbmRpbmdzKCkge1xuICAgIHJldHVybiB7XG4gICAgICBpbnZpdGU6IGdldHRlcnMuaW52aXRlLFxuICAgICAgYXR0ZW1wOiBnZXR0ZXJzLmF0dGVtcFxuICAgIH1cbiAgfSxcblxuICBjb21wb25lbnREaWRNb3VudCgpe1xuICAgIGFjdGlvbnMuZmV0Y2hJbnZpdGUodGhpcy5wcm9wcy5wYXJhbXMuaW52aXRlVG9rZW4pO1xuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgaWYoIXRoaXMuc3RhdGUuaW52aXRlKSB7XG4gICAgICByZXR1cm4gbnVsbDtcbiAgICB9XG5cbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtaW52aXRlIHRleHQtY2VudGVyXCI+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWxvZ28tdHBydFwiPjwvZGl2PlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1jb250ZW50IGdydi1mbGV4XCI+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZmxleC1jb2x1bW5cIj5cbiAgICAgICAgICAgIDxJbnZpdGVJbnB1dEZvcm0gYXR0ZW1wPXt0aGlzLnN0YXRlLmF0dGVtcH0gaW52aXRlPXt0aGlzLnN0YXRlLmludml0ZS50b0pTKCl9Lz5cbiAgICAgICAgICAgIDxHb29nbGVBdXRoSW5mby8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZmxleC1jb2x1bW4gZ3J2LWludml0ZS1iYXJjb2RlXCI+XG4gICAgICAgICAgICA8aDQ+U2NhbiBiYXIgY29kZSBmb3IgYXV0aCB0b2tlbiA8YnIvPiA8c21hbGw+U2NhbiBiZWxvdyB0byBnZW5lcmF0ZSB5b3VyIHR3byBmYWN0b3IgdG9rZW48L3NtYWxsPjwvaDQ+XG4gICAgICAgICAgICA8aW1nIGNsYXNzTmFtZT1cImltZy10aHVtYm5haWxcIiBzcmM9eyBgZGF0YTppbWFnZS9wbmc7YmFzZTY0LCR7dGhpcy5zdGF0ZS5pbnZpdGUuZ2V0KCdxcicpfWAgfSAvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IEludml0ZTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL25ld1VzZXIuanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB1c2VyR2V0dGVycyA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3VzZXIvZ2V0dGVycycpO1xudmFyIG5vZGVHZXR0ZXJzID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvbm9kZXMvZ2V0dGVycycpO1xudmFyIE5vZGVMaXN0ID0gcmVxdWlyZSgnLi9ub2RlTGlzdC5qc3gnKTtcblxudmFyIE5vZGVzID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW3JlYWN0b3IuUmVhY3RNaXhpbl0sXG5cbiAgZ2V0RGF0YUJpbmRpbmdzKCkge1xuICAgIHJldHVybiB7XG4gICAgICBub2RlUmVjb3Jkczogbm9kZUdldHRlcnMubm9kZUxpc3RWaWV3LFxuICAgICAgdXNlcjogdXNlckdldHRlcnMudXNlclxuICAgIH1cbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIHZhciBub2RlUmVjb3JkcyA9IHRoaXMuc3RhdGUubm9kZVJlY29yZHM7XG4gICAgdmFyIGxvZ2lucyA9IHRoaXMuc3RhdGUudXNlci5sb2dpbnM7XG4gICAgcmV0dXJuICggPE5vZGVMaXN0IG5vZGVSZWNvcmRzPXtub2RlUmVjb3Jkc30gbG9naW5zPXtsb2dpbnN9Lz4gKTtcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gTm9kZXM7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9ub2Rlcy9tYWluLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgeyBMaW5rIH0gPSByZXF1aXJlKCdyZWFjdC1yb3V0ZXInKTtcbnZhciB7VGFibGUsIENvbHVtbiwgQ2VsbCwgVGV4dENlbGx9ID0gcmVxdWlyZSgnYXBwL2NvbXBvbmVudHMvdGFibGUuanN4Jyk7XG52YXIge2dldHRlcnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvc2Vzc2lvbnMnKTtcbnZhciB7b3Blbn0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3Rpb25zJyk7XG52YXIgbW9tZW50ID0gIHJlcXVpcmUoJ21vbWVudCcpO1xudmFyIFB1cmVSZW5kZXJNaXhpbiA9IHJlcXVpcmUoJ3JlYWN0LWFkZG9ucy1wdXJlLXJlbmRlci1taXhpbicpO1xuXG5jb25zdCBEYXRlQ3JlYXRlZENlbGwgPSAoeyByb3dJbmRleCwgZGF0YSwgLi4ucHJvcHMgfSkgPT4ge1xuICB2YXIgY3JlYXRlZCA9IGRhdGFbcm93SW5kZXhdLmNyZWF0ZWQ7XG4gIHZhciBkaXNwbGF5RGF0ZSA9IG1vbWVudChjcmVhdGVkKS5mcm9tTm93KCk7XG4gIHJldHVybiAoXG4gICAgPENlbGwgey4uLnByb3BzfT5cbiAgICAgIHsgZGlzcGxheURhdGUgfVxuICAgIDwvQ2VsbD5cbiAgKVxufTtcblxuY29uc3QgRHVyYXRpb25DZWxsID0gKHsgcm93SW5kZXgsIGRhdGEsIC4uLnByb3BzIH0pID0+IHtcbiAgdmFyIGNyZWF0ZWQgPSBkYXRhW3Jvd0luZGV4XS5jcmVhdGVkO1xuICB2YXIgbGFzdEFjdGl2ZSA9IGRhdGFbcm93SW5kZXhdLmxhc3RBY3RpdmU7XG5cbiAgdmFyIGVuZCA9IG1vbWVudChjcmVhdGVkKTtcbiAgdmFyIG5vdyA9IG1vbWVudChsYXN0QWN0aXZlKTtcbiAgdmFyIGR1cmF0aW9uID0gbW9tZW50LmR1cmF0aW9uKG5vdy5kaWZmKGVuZCkpO1xuICB2YXIgZGlzcGxheURhdGUgPSBkdXJhdGlvbi5odW1hbml6ZSgpO1xuXG4gIHJldHVybiAoXG4gICAgPENlbGwgey4uLnByb3BzfT5cbiAgICAgIHsgZGlzcGxheURhdGUgfVxuICAgIDwvQ2VsbD5cbiAgKVxufTtcblxuY29uc3QgVXNlcnNDZWxsID0gKHsgcm93SW5kZXgsIGRhdGEsIC4uLnByb3BzIH0pID0+IHtcbiAgdmFyICR1c2VycyA9IGRhdGFbcm93SW5kZXhdLnBhcnRpZXMubWFwKChpdGVtLCBpdGVtSW5kZXgpPT5cbiAgICAoPHNwYW4ga2V5PXtpdGVtSW5kZXh9IHN0eWxlPXt7YmFja2dyb3VuZENvbG9yOiAnIzFhYjM5NCd9fSBjbGFzc05hbWU9XCJ0ZXh0LXVwcGVyY2FzZSBncnYtcm91bmRlZCBsYWJlbCBsYWJlbC1wcmltYXJ5XCI+e2l0ZW0udXNlclswXX08L3NwYW4+KVxuICApXG5cbiAgcmV0dXJuIChcbiAgICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgICAgPGRpdj5cbiAgICAgICAgeyR1c2Vyc31cbiAgICAgIDwvZGl2PlxuICAgIDwvQ2VsbD5cbiAgKVxufTtcblxuY29uc3QgQnV0dG9uQ2VsbCA9ICh7IHJvd0luZGV4LCBkYXRhLCAuLi5wcm9wcyB9KSA9PiB7XG4gIHZhciB7IHNlc3Npb25VcmwsIGFjdGl2ZSB9ID0gZGF0YVtyb3dJbmRleF07XG4gIHZhciBbYWN0aW9uVGV4dCwgYWN0aW9uQ2xhc3NdID0gYWN0aXZlID8gWydqb2luJywgJ2J0bi13YXJuaW5nJ10gOiBbJ3BsYXknLCAnYnRuLXByaW1hcnknXTtcbiAgcmV0dXJuIChcbiAgICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgICAgPExpbmsgdG89e3Nlc3Npb25Vcmx9IGNsYXNzTmFtZT17XCJidG4gXCIgK2FjdGlvbkNsYXNzKyBcIiBidG4teHNcIn0gdHlwZT1cImJ1dHRvblwiPnthY3Rpb25UZXh0fTwvTGluaz5cbiAgICA8L0NlbGw+XG4gIClcbn1cblxudmFyIFNlc3Npb25MaXN0ID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW3JlYWN0b3IuUmVhY3RNaXhpbiwgUHVyZVJlbmRlck1peGluXSxcblxuICBjb21wb25lbnRXaWxsUmVjZWl2ZVByb3BzKG5leHRQcm9wcykge1xuICAgIC8vdGhpcy5zZXRTdGF0ZSh7IH0pO1xuICB9LFxuXG4gIGdldEluaXRpYWxTdGF0ZSgpe1xuICAgIGRlYnVnZ2VyO1xuICAvKiAgdGhpcy5fZGF0YUxpc3QgPSBuZXcgRmFrZU9iamVjdERhdGFMaXN0U3RvcmUoMjAwMCk7XG4gICAgdGhpcy5fZGVmYXVsdFNvcnRJbmRleGVzID0gW107XG5cbiAgICB2YXIgc2l6ZSA9IHRoaXMuX2RhdGFMaXN0LmdldFNpemUoKTtcbiAgICBmb3IgKHZhciBpbmRleCA9IDA7IGluZGV4IDwgc2l6ZTsgaW5kZXgrKykge1xuICAgICAgdGhpcy5fZGVmYXVsdFNvcnRJbmRleGVzLnB1c2goaW5kZXgpO1xuICAgIH1cblxuICAgIHRoaXMuc3RhdGUgPSB7XG4gICAgICBzb3J0ZWREYXRhTGlzdDogdGhpcy5fZGF0YUxpc3QsXG4gICAgICBjb2xTb3J0RGlyczoge31cbiAgICB9O1xuXG4gICAgdGhpcy5fb25Tb3J0Q2hhbmdlID0gdGhpcy5fb25Tb3J0Q2hhbmdlLmJpbmQodGhpcyk7XG4qL1xuXG4gIH0sXG5cbiAgZ2V0RGF0YUJpbmRpbmdzKCkge1xuICAgIHJldHVybiB7XG4gICAgICBzZXNzaW9uc1ZpZXc6IGdldHRlcnMuc2Vzc2lvbnNWaWV3XG4gICAgfVxuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgdmFyIGRhdGEgPSB0aGlzLnN0YXRlLnNlc3Npb25zVmlldztcbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtc2Vzc2lvbnNcIj5cbiAgICAgICAgPGgxPiBTZXNzaW9uczwvaDE+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiXCI+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiXCI+XG4gICAgICAgICAgICAgIDxUYWJsZSByb3dDb3VudD17ZGF0YS5sZW5ndGh9IGNsYXNzTmFtZT1cInRhYmxlLXN0cmlwZWRcIj5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJzaWRcIlxuICAgICAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD4gU2Vzc2lvbiBJRCA8L0NlbGw+IH1cbiAgICAgICAgICAgICAgICAgIGNlbGw9ezxUZXh0Q2VsbCBkYXRhPXtkYXRhfS8+IH1cbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+IDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17XG4gICAgICAgICAgICAgICAgICAgIDxCdXR0b25DZWxsIGRhdGE9e2RhdGF9IC8+XG4gICAgICAgICAgICAgICAgICB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJzZXJ2ZXJJcFwiXG4gICAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPiBOb2RlIDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17PFRleHRDZWxsIGRhdGE9e2RhdGF9IC8+IH1cbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICAgIGNvbHVtbktleT1cImNyZWF0ZWRcIlxuICAgICAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD4gQ3JlYXRlZCA8L0NlbGw+IH1cbiAgICAgICAgICAgICAgICAgIGNlbGw9ezxEYXRlQ3JlYXRlZENlbGwgZGF0YT17ZGF0YX0vPiB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJzZXJ2ZXJJZFwiXG4gICAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPiBBY3RpdmUgPC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXs8VXNlcnNDZWxsIGRhdGE9e2RhdGF9IC8+IH1cbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICA8L1RhYmxlPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9kaXY+XG4gICAgKVxuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBTZXNzaW9uTGlzdDtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL3Nlc3Npb25zL21haW4uanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciByZW5kZXIgPSByZXF1aXJlKCdyZWFjdC1kb20nKS5yZW5kZXI7XG52YXIgeyBSb3V0ZXIsIFJvdXRlLCBSZWRpcmVjdCwgSW5kZXhSb3V0ZSwgYnJvd3Nlckhpc3RvcnkgfSA9IHJlcXVpcmUoJ3JlYWN0LXJvdXRlcicpO1xudmFyIHsgQXBwLCBMb2dpbiwgTm9kZXMsIFNlc3Npb25zLCBOZXdVc2VyLCBDdXJyZW50U2Vzc2lvbkhvc3QsIE5vdEZvdW5kUGFnZSB9ID0gcmVxdWlyZSgnLi9jb21wb25lbnRzJyk7XG52YXIge2Vuc3VyZVVzZXJ9ID0gcmVxdWlyZSgnLi9tb2R1bGVzL3VzZXIvYWN0aW9ucycpO1xudmFyIGF1dGggPSByZXF1aXJlKCcuL2F1dGgnKTtcbnZhciBzZXNzaW9uID0gcmVxdWlyZSgnLi9zZXNzaW9uJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnLi9jb25maWcnKTtcblxucmVxdWlyZSgnLi9tb2R1bGVzJyk7XG5cbi8vIGluaXQgc2Vzc2lvblxuc2Vzc2lvbi5pbml0KCk7XG5cbmZ1bmN0aW9uIGhhbmRsZUxvZ291dChuZXh0U3RhdGUsIHJlcGxhY2UsIGNiKXtcbiAgYXV0aC5sb2dvdXQoKTtcbn1cblxucmVuZGVyKChcbiAgPFJvdXRlciBoaXN0b3J5PXtzZXNzaW9uLmdldEhpc3RvcnkoKX0+XG4gICAgPFJvdXRlIHBhdGg9e2NmZy5yb3V0ZXMubG9naW59IGNvbXBvbmVudD17TG9naW59Lz5cbiAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5sb2dvdXR9IG9uRW50ZXI9e2hhbmRsZUxvZ291dH0vPlxuICAgIDxSb3V0ZSBwYXRoPXtjZmcucm91dGVzLm5ld1VzZXJ9IGNvbXBvbmVudD17TmV3VXNlcn0vPlxuICAgIDxSZWRpcmVjdCBmcm9tPXtjZmcucm91dGVzLmFwcH0gdG89e2NmZy5yb3V0ZXMubm9kZXN9Lz5cbiAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5hcHB9IGNvbXBvbmVudD17QXBwfSBvbkVudGVyPXtlbnN1cmVVc2VyfSA+XG4gICAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5ub2Rlc30gY29tcG9uZW50PXtOb2Rlc30vPlxuICAgICAgPFJvdXRlIHBhdGg9e2NmZy5yb3V0ZXMuYWN0aXZlU2Vzc2lvbn0gY29tcG9uZW50cz17e0N1cnJlbnRTZXNzaW9uSG9zdDogQ3VycmVudFNlc3Npb25Ib3N0fX0vPlxuICAgICAgPFJvdXRlIHBhdGg9e2NmZy5yb3V0ZXMuc2Vzc2lvbnN9IGNvbXBvbmVudD17U2Vzc2lvbnN9Lz5cbiAgICA8L1JvdXRlPlxuICAgIDxSb3V0ZSBwYXRoPVwiKlwiIGNvbXBvbmVudD17Tm90Rm91bmRQYWdlfSAvPlxuICA8L1JvdXRlcj5cbiksIGRvY3VtZW50LmdldEVsZW1lbnRCeUlkKFwiYXBwXCIpKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9pbmRleC5qc3hcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cyA9IFRlcm1pbmFsO1xuXG5cbi8qKioqKioqKioqKioqKioqKlxuICoqIFdFQlBBQ0sgRk9PVEVSXG4gKiogZXh0ZXJuYWwgXCJUZXJtaW5hbFwiXG4gKiogbW9kdWxlIGlkID0gNDA1XG4gKiogbW9kdWxlIGNodW5rcyA9IDFcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cyA9IF87XG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiBleHRlcm5hbCBcIl9cIlxuICoqIG1vZHVsZSBpZCA9IDQwNlxuICoqIG1vZHVsZSBjaHVua3MgPSAxXG4gKiovIl0sInNvdXJjZVJvb3QiOiIifQ==