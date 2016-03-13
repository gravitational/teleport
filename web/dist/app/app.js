webpackJsonp([1],{

/***/ 0:
/***/ function(module, exports, __webpack_require__) {

	module.exports = __webpack_require__(311);


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
	
	var _require = __webpack_require__(272);
	
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
	
	    getFetchSessionsUrl: function getFetchSessionsUrl(start, end) {
	      var params = {
	        start: start.toISOString(),
	        end: end.toISOString()
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
	
	var _require = __webpack_require__(41);
	
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
	
	var $ = __webpack_require__(39);
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

/***/ 39:
/***/ function(module, exports) {

	module.exports = jQuery;

/***/ },

/***/ 44:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	var session = __webpack_require__(26);
	
	var _require = __webpack_require__(285);
	
	var uuid = _require.uuid;
	
	var api = __webpack_require__(32);
	var cfg = __webpack_require__(11);
	var getters = __webpack_require__(60);
	var sessionModule = __webpack_require__(112);
	
	var _require2 = __webpack_require__(97);
	
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

/***/ 45:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(60);
	module.exports.actions = __webpack_require__(44);
	module.exports.activeTermStore = __webpack_require__(98);

/***/ },

/***/ 46:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(20);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TRYING_TO_SIGN_UP: null,
	  TRYING_TO_LOGIN: null,
	  FETCHING_INVITE: null
	});
	module.exports = exports['default'];

/***/ },

/***/ 47:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(46);
	
	var TRYING_TO_LOGIN = _require.TRYING_TO_LOGIN;
	
	var _require2 = __webpack_require__(110);
	
	var requestStatus = _require2.requestStatus;
	
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
	  user: user,
	  loginAttemp: requestStatus(TRYING_TO_LOGIN)
	};
	module.exports = exports['default'];

/***/ },

/***/ 59:
/***/ function(module, exports) {

	module.exports = _;

/***/ },

/***/ 60:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(64);
	
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

/***/ 61:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(101);
	
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

/***/ 62:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(64);
	
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

/***/ 63:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	var api = __webpack_require__(32);
	var cfg = __webpack_require__(11);
	
	var _require = __webpack_require__(111);
	
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
	
	  fetchSessions: function fetchSessions(startDate, endDate) {
	    return api.get(cfg.api.getFetchSessionsUrl(startDate, endDate)).done(function (json) {
	      reactor.dispatch(TLPT_SESSINS_RECEIVE, json.sessions);
	    });
	  },
	
	  updateSession: function updateSession(json) {
	    reactor.dispatch(TLPT_SESSINS_UPDATE, json);
	  }
	};
	module.exports = exports['default'];

/***/ },

/***/ 64:
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

/***/ 93:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var api = __webpack_require__(32);
	var session = __webpack_require__(26);
	var cfg = __webpack_require__(11);
	var $ = __webpack_require__(39);
	
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

/***/ 94:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var moment = __webpack_require__(1);
	
	module.exports.monthRange = function () {
	  var value = arguments.length <= 0 || arguments[0] === undefined ? new Date() : arguments[0];
	
	  var startDate = moment(value).startOf('month').toDate();
	  var endDate = moment(value).endOf('month').toDate();
	  return [startDate, endDate];
	};

/***/ },

/***/ 95:
/***/ function(module, exports) {

	'use strict';
	
	module.exports.isMatch = function (obj, searchValue, _ref) {
	  var searchableProps = _ref.searchableProps;
	  var cb = _ref.cb;
	
	  searchValue = searchValue.toLocaleUpperCase();
	  var propNames = searchableProps || Object.getOwnPropertyNames(obj);
	  for (var i = 0; i < propNames.length; i++) {
	    var targetValue = obj[propNames[i]];
	    if (targetValue) {
	      if (typeof cb === 'function') {
	        var result = cb(targetValue, searchValue, propNames[i]);
	        if (result !== undefined) {
	          return result;
	        }
	      }
	
	      if (targetValue.toString().toLocaleUpperCase().indexOf(searchValue) !== -1) {
	        return true;
	      }
	    }
	  }
	
	  return false;
	};

/***/ },

/***/ 96:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };
	
	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError('Cannot call a class as a function'); } }
	
	function _inherits(subClass, superClass) { if (typeof superClass !== 'function' && superClass !== null) { throw new TypeError('Super expression must either be null or a function, not ' + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }
	
	var EventEmitter = __webpack_require__(286).EventEmitter;
	var session = __webpack_require__(26);
	var cfg = __webpack_require__(11);
	
	var _require = __webpack_require__(45);
	
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

/***/ 97:
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

/***/ 98:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(97);
	
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

/***/ 99:
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

/***/ 100:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(99);
	
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

/***/ 101:
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

/***/ 102:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(101);
	
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

/***/ 103:
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

/***/ 104:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(103);
	
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

/***/ 105:
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

/***/ 106:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(105);
	
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

/***/ 107:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(105);
	
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

/***/ 108:
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

/***/ 109:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(108);
	
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

/***/ 110:
/***/ function(module, exports) {

	'use strict';
	
	exports.__esModule = true;
	var defaultObj = {
	  isProcessing: false,
	  isError: false,
	  isSuccess: false,
	  message: ''
	};
	
	var requestStatus = function requestStatus(reqType) {
	  return [['tlpt_rest_api', reqType], function (attemp) {
	    return attemp ? attemp.toJS() : defaultObj;
	  }];
	};
	
	exports['default'] = { requestStatus: requestStatus };
	module.exports = exports['default'];

/***/ },

/***/ 111:
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

/***/ 112:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(64);
	module.exports.actions = __webpack_require__(63);
	module.exports.activeTermStore = __webpack_require__(113);

/***/ },

/***/ 113:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(111);
	
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

/***/ 114:
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

/***/ 115:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(114);
	
	var TLPT_RECEIVE_USER = _require.TLPT_RECEIVE_USER;
	
	var _require2 = __webpack_require__(46);
	
	var TRYING_TO_SIGN_UP = _require2.TRYING_TO_SIGN_UP;
	var TRYING_TO_LOGIN = _require2.TRYING_TO_LOGIN;
	
	var restApiActions = __webpack_require__(109);
	var auth = __webpack_require__(93);
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
	    }).fail(function (err) {
	      restApiActions.fail(TRYING_TO_SIGN_UP, err.responseJSON.message || 'failed to sing up');
	    });
	  },
	
	  login: function login(_ref2, redirect) {
	    var user = _ref2.user;
	    var password = _ref2.password;
	    var token = _ref2.token;
	
	    restApiActions.start(TRYING_TO_LOGIN);
	    auth.login(user, password, token).done(function (sessionData) {
	      restApiActions.success(TRYING_TO_LOGIN);
	      reactor.dispatch(TLPT_RECEIVE_USER, sessionData.user);
	      session.getHistory().push({ pathname: redirect });
	    }).fail(function (err) {
	      return restApiActions.fail(TRYING_TO_LOGIN, err.responseJSON.message);
	    });
	  }
	};
	module.exports = exports['default'];

/***/ },

/***/ 116:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(47);
	module.exports.actions = __webpack_require__(115);
	module.exports.nodeStore = __webpack_require__(117);

/***/ },

/***/ 117:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(114);
	
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

/***/ 221:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	
	var _require = __webpack_require__(45);
	
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

/***/ 222:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	"use strict";
	
	exports.__esModule = true;
	var React = __webpack_require__(4);
	
	var NotFound = React.createClass({
	  displayName: "NotFound",
	
	  render: function render() {
	    return React.createElement(
	      "div",
	      { className: "grv-error-page" },
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
	
	var ExpiredInvite = React.createClass({
	  displayName: "ExpiredInvite",
	
	  render: function render() {
	    return React.createElement(
	      "div",
	      { className: "grv-error-page" },
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
	        "Invite code has expired"
	      ),
	      React.createElement(
	        "div",
	        null,
	        "Looks like your invite code isn't valid anymore"
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
	
	exports["default"] = NotFound;
	exports.NotFound = NotFound;
	exports.ExpiredInvite = ExpiredInvite;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "errorPage.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 223:
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

/***/ 224:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };
	
	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }
	
	var React = __webpack_require__(4);
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(283);
	
	var getters = _require.getters;
	var actions = _require.actions;
	
	var userGetters = __webpack_require__(47);
	
	var _require2 = __webpack_require__(226);
	
	var Table = _require2.Table;
	var Column = _require2.Column;
	var Cell = _require2.Cell;
	var SortHeaderCell = _require2.SortHeaderCell;
	var SortTypes = _require2.SortTypes;
	
	var _require3 = __webpack_require__(44);
	
	var createNewSession = _require3.createNewSession;
	
	var LinkedStateMixin = __webpack_require__(40);
	var _ = __webpack_require__(59);
	
	var _require4 = __webpack_require__(95);
	
	var isMatch = _require4.isMatch;
	
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
	
	  mixins: [LinkedStateMixin],
	
	  getInitialState: function getInitialState(props) {
	    this.searchableProps = ['sessionCount', 'addr'];
	    return { filter: '', colSortDirs: {} };
	  },
	
	  onSortChange: function onSortChange(columnKey, sortDir) {
	    var _colSortDirs;
	
	    this.setState(_extends({}, this.state, {
	      colSortDirs: (_colSortDirs = {}, _colSortDirs[columnKey] = sortDir, _colSortDirs)
	    }));
	  },
	
	  sortAndFilter: function sortAndFilter(data) {
	    var _this = this;
	
	    var filtered = data.filter(function (obj) {
	      return isMatch(obj, _this.state.filter, { searchableProps: _this.searchableProps });
	    });
	
	    var columnKey = Object.getOwnPropertyNames(this.state.colSortDirs)[0];
	    var sortDir = this.state.colSortDirs[columnKey];
	    var sorted = _.sortBy(filtered, columnKey);
	    if (sortDir === SortTypes.ASC) {
	      sorted = sorted.reverse();
	    }
	
	    return sorted;
	  },
	
	  render: function render() {
	    var data = this.sortAndFilter(this.props.nodeRecords);
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
	        { className: 'grv-search' },
	        React.createElement('input', { valueLink: this.linkState('filter'), placeholder: 'Search...', className: 'form-control input-sm' })
	      ),
	      React.createElement(
	        'div',
	        { className: '' },
	        React.createElement(
	          Table,
	          { rowCount: data.length, className: 'table-striped grv-nodes-table' },
	          React.createElement(Column, {
	            columnKey: 'sessionCount',
	            header: React.createElement(SortHeaderCell, {
	              sortDir: this.state.colSortDirs.sessionCount,
	              onSortChange: this.onSortChange,
	              title: 'Sessions'
	            }),
	            cell: React.createElement(TextCell, { data: data })
	          }),
	          React.createElement(Column, {
	            columnKey: 'addr',
	            header: React.createElement(SortHeaderCell, {
	              sortDir: this.state.colSortDirs.addr,
	              onSortChange: this.onSortChange,
	              title: 'Node'
	            }),
	
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
	    );
	  }
	});
	
	module.exports = NodeList;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "nodeList.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 225:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(278);
	
	var getters = _require.getters;
	
	var _require2 = __webpack_require__(61);
	
	var closeSelectNodeDialog = _require2.closeSelectNodeDialog;
	
	var _require3 = __webpack_require__(44);
	
	var changeServer = _require3.changeServer;
	
	var NodeList = __webpack_require__(224);
	var activeSessionGetters = __webpack_require__(60);
	var nodeGetters = __webpack_require__(62);
	
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

/***/ 226:
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
	
	/**
	* Sort indicator used by SortHeaderCell
	*/
	var SortTypes = {
	  ASC: 'ASC',
	  DESC: 'DESC'
	};
	
	var SortIndicator = function SortIndicator(_ref2) {
	  var sortDir = _ref2.sortDir;
	
	  var cls = 'grv-table-indicator-sort fa fa-sort';
	  if (sortDir === SortTypes.DESC) {
	    cls += '-desc';
	  }
	
	  if (sortDir === SortTypes.ASC) {
	    cls += '-asc';
	  }
	
	  return React.createElement('i', { className: cls });
	};
	
	/**
	* Sort Header Cell
	*/
	var SortHeaderCell = React.createClass({
	  displayName: 'SortHeaderCell',
	
	  render: function render() {
	    var _props2 = this.props;
	    var sortDir = _props2.sortDir;
	    var columnKey = _props2.columnKey;
	    var title = _props2.title;
	
	    var props = _objectWithoutProperties(_props2, ['sortDir', 'columnKey', 'title']);
	
	    return React.createElement(
	      GrvTableCell,
	      props,
	      React.createElement(
	        'a',
	        { onClick: this.onSortChange },
	        title
	      ),
	      React.createElement(SortIndicator, { sortDir: sortDir })
	    );
	  },
	
	  onSortChange: function onSortChange(e) {
	    e.preventDefault();
	    if (this.props.onSortChange) {
	      // default
	      var newDir = SortTypes.DESC;
	      if (this.props.sortDir) {
	        newDir = this.props.sortDir === SortTypes.DESC ? SortTypes.ASC : SortTypes.DESC;
	      }
	      this.props.onSortChange(this.props.columnKey, newDir);
	    }
	  }
	});
	
	/**
	* Default Cell
	*/
	var GrvTableCell = React.createClass({
	  displayName: 'GrvTableCell',
	
	  render: function render() {
	    var props = this.props;
	    return props.isHeader ? React.createElement(
	      'th',
	      { key: props.key, className: 'grv-table-cell' },
	      props.children
	    ) : React.createElement(
	      'td',
	      { key: props.key },
	      props.children
	    );
	  }
	});
	
	/**
	* Table
	*/
	var GrvTable = React.createClass({
	  displayName: 'GrvTable',
	
	  renderHeader: function renderHeader(children) {
	    var _this = this;
	
	    var cells = children.map(function (item, index) {
	      return _this.renderCell(item.props.header, _extends({ index: index, key: index, isHeader: true }, item.props));
	    });
	
	    return React.createElement(
	      'thead',
	      { className: 'grv-table-header' },
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
	exports.SortHeaderCell = SortHeaderCell;
	exports.SortIndicator = SortIndicator;
	exports.SortTypes = SortTypes;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "table.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 227:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var Term = __webpack_require__(411);
	var React = __webpack_require__(4);
	
	var _require = __webpack_require__(59);
	
	var debounce = _require.debounce;
	var isNumber = _require.isNumber;
	
	Term.colors[256] = '#252323';
	
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
	
	    var width = $container[0].clientWidth;
	    var height = $container[0].clientHeight;
	
	    var cols = Math.floor(width / fakeColWidth);
	    var rows = Math.floor(height / fakeColHeight);
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

/***/ 272:
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

/***/ 273:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError('Cannot call a class as a function'); } }
	
	function _inherits(subClass, superClass) { if (typeof superClass !== 'function' && superClass !== null) { throw new TypeError('Super expression must either be null or a function, not ' + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }
	
	var Tty = __webpack_require__(96);
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
	      this.emit('reset');
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

/***/ 274:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(63);
	
	var fetchSessions = _require.fetchSessions;
	
	var _require2 = __webpack_require__(106);
	
	var fetchNodes = _require2.fetchNodes;
	
	var _require3 = __webpack_require__(94);
	
	var monthRange = _require3.monthRange;
	
	var $ = __webpack_require__(39);
	
	var _require4 = __webpack_require__(99);
	
	var TLPT_APP_INIT = _require4.TLPT_APP_INIT;
	var TLPT_APP_FAILED = _require4.TLPT_APP_FAILED;
	var TLPT_APP_READY = _require4.TLPT_APP_READY;
	
	var actions = {
	
	  initApp: function initApp() {
	    reactor.dispatch(TLPT_APP_INIT);
	    actions.fetchNodesAndSessions().done(function () {
	      reactor.dispatch(TLPT_APP_READY);
	    }).fail(function () {
	      reactor.dispatch(TLPT_APP_FAILED);
	    });
	  },
	
	  fetchNodesAndSessions: function fetchNodesAndSessions() {
	    var _monthRange = monthRange();
	
	    var start = _monthRange[0];
	    var end = _monthRange[1];
	
	    return $.when(fetchNodes(), fetchSessions(start, end));
	  }
	};
	
	exports['default'] = actions;
	module.exports = exports['default'];

/***/ },

/***/ 275:
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

/***/ 276:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(275);
	module.exports.actions = __webpack_require__(274);
	module.exports.appStore = __webpack_require__(100);

/***/ },

/***/ 277:
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

/***/ 278:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(277);
	module.exports.actions = __webpack_require__(61);
	module.exports.dialogStore = __webpack_require__(102);

/***/ },

/***/ 279:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var reactor = __webpack_require__(7);
	reactor.registerStores({
	  'tlpt': __webpack_require__(100),
	  'tlpt_dialogs': __webpack_require__(102),
	  'tlpt_active_terminal': __webpack_require__(98),
	  'tlpt_user': __webpack_require__(117),
	  'tlpt_nodes': __webpack_require__(107),
	  'tlpt_invite': __webpack_require__(104),
	  'tlpt_rest_api': __webpack_require__(284),
	  'tlpt_sessions': __webpack_require__(113)
	});

/***/ },

/***/ 280:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(103);
	
	var TLPT_RECEIVE_USER_INVITE = _require.TLPT_RECEIVE_USER_INVITE;
	
	var _require2 = __webpack_require__(46);
	
	var FETCHING_INVITE = _require2.FETCHING_INVITE;
	
	var restApiActions = __webpack_require__(109);
	var api = __webpack_require__(32);
	var cfg = __webpack_require__(11);
	
	exports['default'] = {
	  fetchInvite: function fetchInvite(inviteToken) {
	    var path = cfg.api.getInviteUrl(inviteToken);
	    restApiActions.start(FETCHING_INVITE);
	    api.get(path).done(function (invite) {
	      restApiActions.success(FETCHING_INVITE);
	      reactor.dispatch(TLPT_RECEIVE_USER_INVITE, invite);
	    }).fail(function (err) {
	      restApiActions.fail(FETCHING_INVITE, err.responseJSON.message);
	    });
	  }
	};
	module.exports = exports['default'];

/***/ },

/***/ 281:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(46);
	
	var TRYING_TO_SIGN_UP = _require.TRYING_TO_SIGN_UP;
	var FETCHING_INVITE = _require.FETCHING_INVITE;
	
	var _require2 = __webpack_require__(110);
	
	var requestStatus = _require2.requestStatus;
	
	var invite = [['tlpt_invite'], function (invite) {
	  return invite;
	}];
	
	exports['default'] = {
	  invite: invite,
	  attemp: requestStatus(TRYING_TO_SIGN_UP),
	  fetchingInvite: requestStatus(FETCHING_INVITE)
	};
	module.exports = exports['default'];

/***/ },

/***/ 282:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(281);
	module.exports.actions = __webpack_require__(280);
	module.exports.nodeStore = __webpack_require__(104);

/***/ },

/***/ 283:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(62);
	module.exports.actions = __webpack_require__(106);
	module.exports.nodeStore = __webpack_require__(107);
	
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

/***/ 284:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(108);
	
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

/***/ 285:
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

/***/ 286:
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

/***/ 298:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	var NavLeftBar = __webpack_require__(306);
	var cfg = __webpack_require__(11);
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(276);
	
	var actions = _require.actions;
	var getters = _require.getters;
	
	var SelectNodeDialog = __webpack_require__(225);
	
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

/***/ 299:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };
	
	var React = __webpack_require__(4);
	
	var _require = __webpack_require__(45);
	
	var getters = _require.getters;
	var actions = _require.actions;
	
	var Tty = __webpack_require__(96);
	var TtyTerminal = __webpack_require__(227);
	var EventStreamer = __webpack_require__(300);
	var SessionLeftPanel = __webpack_require__(221);
	
	var _require2 = __webpack_require__(61);
	
	var showSelectNodeDialog = _require2.showSelectNodeDialog;
	var closeSelectNodeDialog = _require2.closeSelectNodeDialog;
	
	var SelectNodeDialog = __webpack_require__(225);
	
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

/***/ 300:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	exports.__esModule = true;
	var cfg = __webpack_require__(11);
	var React = __webpack_require__(4);
	var session = __webpack_require__(26);
	
	var _require = __webpack_require__(63);
	
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

/***/ 301:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	
	var _require = __webpack_require__(45);
	
	var getters = _require.getters;
	var actions = _require.actions;
	
	var SessionPlayer = __webpack_require__(302);
	var ActiveSession = __webpack_require__(299);
	
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

/***/ 302:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	exports.__esModule = true;
	var React = __webpack_require__(4);
	var ReactSlider = __webpack_require__(235);
	var TtyPlayer = __webpack_require__(273);
	var TtyTerminal = __webpack_require__(227);
	var SessionLeftPanel = __webpack_require__(221);
	
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

/***/ 303:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	exports.__esModule = true;
	var React = __webpack_require__(4);
	var $ = __webpack_require__(39);
	var moment = __webpack_require__(1);
	
	var _require = __webpack_require__(59);
	
	var debounce = _require.debounce;
	
	var DateRangePicker = React.createClass({
	  displayName: 'DateRangePicker',
	
	  getDates: function getDates() {
	    var startDate = $(this.refs.dpPicker1).datepicker('getDate');
	    var endDate = $(this.refs.dpPicker2).datepicker('getDate');
	    return [startDate, endDate];
	  },
	
	  setDates: function setDates(_ref) {
	    var startDate = _ref.startDate;
	    var endDate = _ref.endDate;
	
	    $(this.refs.dpPicker1).datepicker('setDate', startDate);
	    $(this.refs.dpPicker2).datepicker('setDate', endDate);
	  },
	
	  getDefaultProps: function getDefaultProps() {
	    return {
	      startDate: moment().startOf('month').toDate(),
	      endDate: moment().endOf('month').toDate(),
	      onChange: function onChange() {}
	    };
	  },
	
	  componentWillUnmount: function componentWillUnmount() {
	    $(this.refs.dp).datepicker('destroy');
	  },
	
	  componentWillReceiveProps: function componentWillReceiveProps(newProps) {
	    var _getDates = this.getDates();
	
	    var startDate = _getDates[0];
	    var endDate = _getDates[1];
	
	    if (!(isSame(startDate, newProps.startDate) && isSame(endDate, newProps.endDate))) {
	      this.setDates(newProps);
	    }
	  },
	
	  shouldComponentUpdate: function shouldComponentUpdate() {
	    return false;
	  },
	
	  componentDidMount: function componentDidMount() {
	    this.onChange = debounce(this.onChange, 1);
	    $(this.refs.rangePicker).datepicker({
	      todayBtn: 'linked',
	      keyboardNavigation: false,
	      forceParse: false,
	      calendarWeeks: true,
	      autoclose: true
	    }).on('changeDate', this.onChange);
	
	    this.setDates(this.props);
	  },
	
	  onChange: function onChange() {
	    var _getDates2 = this.getDates();
	
	    var startDate = _getDates2[0];
	    var endDate = _getDates2[1];
	
	    if (!(isSame(startDate, this.props.startDate) && isSame(endDate, this.props.endDate))) {
	      this.props.onChange({ startDate: startDate, endDate: endDate });
	    }
	  },
	
	  render: function render() {
	    return React.createElement(
	      'div',
	      { className: 'grv-datepicker input-group input-daterange', ref: 'rangePicker' },
	      React.createElement(
	        'span',
	        { className: 'input-group-addon' },
	        React.createElement('i', { className: 'fa fa-calendar' })
	      ),
	      React.createElement('input', { ref: 'dpPicker1', type: 'text', className: 'input-sm form-control', name: 'start' }),
	      React.createElement(
	        'span',
	        { className: 'input-group-addon' },
	        'to'
	      ),
	      React.createElement('input', { ref: 'dpPicker2', type: 'text', className: 'input-sm form-control', name: 'end' })
	    );
	  }
	});
	
	function isSame(date1, date2) {
	  return moment(date1).isSame(date2, 'day');
	}
	
	/**
	* Calendar Nav
	*/
	var CalendarNav = React.createClass({
	  displayName: 'CalendarNav',
	
	  render: function render() {
	    var value = this.props.value;
	
	    var displayValue = moment(value).format('MMMM, YYYY');
	
	    return React.createElement(
	      'div',
	      { className: "grv-calendar-nav " + this.props.className },
	      React.createElement(
	        'button',
	        { onClick: this.move.bind(this, -1), className: 'btn btn-outline btn-link' },
	        React.createElement('i', { className: 'fa fa-chevron-left' })
	      ),
	      React.createElement(
	        'span',
	        { className: 'text-muted' },
	        displayValue
	      ),
	      React.createElement(
	        'button',
	        { onClick: this.move.bind(this, 1), className: 'btn btn-outline btn-link' },
	        React.createElement('i', { className: 'fa fa-chevron-right' })
	      )
	    );
	  },
	
	  move: function move(at) {
	    var value = this.props.value;
	
	    var newValue = moment(value).add(at, 'month').toDate();
	    this.props.onValueChange(newValue);
	  }
	});
	
	CalendarNav.getMonthRange = function (value) {
	  var startDate = moment(value).startOf('month').toDate();
	  var endDate = moment(value).endOf('month').toDate();
	  return [startDate, endDate];
	};
	
	exports['default'] = DateRangePicker;
	exports.CalendarNav = CalendarNav;
	exports.DateRangePicker = DateRangePicker;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "datePicker.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 304:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	module.exports.App = __webpack_require__(298);
	module.exports.Login = __webpack_require__(305);
	module.exports.NewUser = __webpack_require__(307);
	module.exports.Nodes = __webpack_require__(308);
	module.exports.Sessions = __webpack_require__(309);
	module.exports.CurrentSessionHost = __webpack_require__(301);
	module.exports.NotFound = __webpack_require__(222).NotFound;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "index.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 305:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	var $ = __webpack_require__(39);
	var reactor = __webpack_require__(7);
	var LinkedStateMixin = __webpack_require__(40);
	
	var _require = __webpack_require__(116);
	
	var actions = _require.actions;
	var getters = _require.getters;
	
	var GoogleAuthInfo = __webpack_require__(223);
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
	    var _props$attemp = this.props.attemp;
	    var isProcessing = _props$attemp.isProcessing;
	    var isFailed = _props$attemp.isFailed;
	    var message = _props$attemp.message;
	
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
	          { onClick: this.onClick, disabled: isProcessing, type: 'submit', className: 'btn btn-primary block full-width m-b' },
	          'Login'
	        ),
	        isFailed ? React.createElement(
	          'label',
	          { className: 'error' },
	          message
	        ) : null
	      )
	    );
	  }
	});
	
	var Login = React.createClass({
	  displayName: 'Login',
	
	  mixins: [reactor.ReactMixin],
	
	  getDataBindings: function getDataBindings() {
	    return {
	      attemp: getters.loginAttemp
	    };
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
	          React.createElement(LoginInputForm, { attemp: this.state.attemp, onClick: this.onClick }),
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

/***/ 306:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	
	var _require = __webpack_require__(41);
	
	var Router = _require.Router;
	var IndexLink = _require.IndexLink;
	var History = _require.History;
	
	var getters = __webpack_require__(47);
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

/***/ 307:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	var $ = __webpack_require__(39);
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(282);
	
	var actions = _require.actions;
	var getters = _require.getters;
	
	var userModule = __webpack_require__(116);
	var LinkedStateMixin = __webpack_require__(40);
	var GoogleAuthInfo = __webpack_require__(223);
	
	var _require2 = __webpack_require__(222);
	
	var ExpiredInvite = _require2.ExpiredInvite;
	
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
	    var _props$attemp = this.props.attemp;
	    var isProcessing = _props$attemp.isProcessing;
	    var isFailed = _props$attemp.isFailed;
	    var message = _props$attemp.message;
	
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
	          { type: 'submit', disabled: isProcessing, className: 'btn btn-primary block full-width m-b', onClick: this.onClick },
	          'Sign up'
	        ),
	        isFailed ? React.createElement(
	          'label',
	          { className: 'error' },
	          message
	        ) : null
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
	      attemp: getters.attemp,
	      fetchingInvite: getters.fetchingInvite
	    };
	  },
	
	  componentDidMount: function componentDidMount() {
	    actions.fetchInvite(this.props.params.inviteToken);
	  },
	
	  render: function render() {
	    var _state = this.state;
	    var fetchingInvite = _state.fetchingInvite;
	    var invite = _state.invite;
	    var attemp = _state.attemp;
	
	    if (fetchingInvite.isFailed) {
	      return React.createElement(ExpiredInvite, null);
	    }
	
	    if (!invite) {
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
	          React.createElement(InviteInputForm, { attemp: attemp, invite: invite.toJS() }),
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
	          React.createElement('img', { className: 'img-thumbnail', src: 'data:image/png;base64,' + invite.get('qr') })
	        )
	      )
	    );
	  }
	});
	
	module.exports = Invite;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "newUser.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 308:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	var reactor = __webpack_require__(7);
	var userGetters = __webpack_require__(47);
	var nodeGetters = __webpack_require__(62);
	var NodeList = __webpack_require__(224);
	
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

/***/ 309:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(112);
	
	var getters = _require.getters;
	var actions = _require.actions;
	
	var SessionList = __webpack_require__(310);
	
	var _require2 = __webpack_require__(303);
	
	var DateRangePicker = _require2.DateRangePicker;
	var CalendarNav = _require2.CalendarNav;
	
	var moment = __webpack_require__(1);
	
	var _require3 = __webpack_require__(94);
	
	var monthRange = _require3.monthRange;
	
	var Sessions = React.createClass({
	  displayName: 'Sessions',
	
	  mixins: [reactor.ReactMixin],
	
	  getInitialState: function getInitialState() {
	    var _monthRange = monthRange(new Date());
	
	    var startDate = _monthRange[0];
	    var endDate = _monthRange[1];
	
	    return {
	      startDate: startDate,
	      endDate: endDate
	    };
	  },
	
	  getDataBindings: function getDataBindings() {
	    return {
	      sessionsView: getters.sessionsView
	    };
	  },
	
	  setNewState: function setNewState(startDate, endDate) {
	    actions.fetchSessions(startDate, endDate);
	    this.state.startDate = startDate;
	    this.state.endDate = endDate;
	    this.setState(this.state);
	  },
	
	  componentWillMount: function componentWillMount() {
	    actions.fetchSessions(this.state.startDate, this.state.endDate);
	  },
	
	  componentWillUnmount: function componentWillUnmount() {},
	
	  onRangePickerChange: function onRangePickerChange(_ref) {
	    var startDate = _ref.startDate;
	    var endDate = _ref.endDate;
	
	    this.setNewState(startDate, endDate);
	  },
	
	  onCalendarNavChange: function onCalendarNavChange(newValue) {
	    var _monthRange2 = monthRange(newValue);
	
	    var startDate = _monthRange2[0];
	    var endDate = _monthRange2[1];
	
	    this.setNewState(startDate, endDate);
	  },
	
	  render: function render() {
	    var _state = this.state;
	    var startDate = _state.startDate;
	    var endDate = _state.endDate;
	
	    var data = this.state.sessionsView.filter(function (item) {
	      return moment(item.created).isBetween(startDate, endDate);
	    });
	
	    return React.createElement(
	      'div',
	      { className: 'grv-sessions' },
	      React.createElement(
	        'div',
	        { className: 'grv-flex' },
	        React.createElement(
	          'div',
	          { className: 'grv-flex-column' },
	          React.createElement(
	            'h1',
	            null,
	            ' Sessions'
	          )
	        )
	      ),
	      React.createElement(
	        'div',
	        { className: 'grv-flex' },
	        React.createElement(
	          'div',
	          { className: 'grv-flex-column' },
	          React.createElement(DateRangePicker, { startDate: startDate, endDate: endDate, onChange: this.onRangePickerChange })
	        ),
	        React.createElement(
	          'div',
	          { className: 'grv-flex-column' },
	          React.createElement(CalendarNav, { value: startDate, onValueChange: this.onCalendarNavChange })
	        ),
	        React.createElement('div', { className: 'grv-flex-column' })
	      ),
	      React.createElement(SessionList, { sessionRecords: data })
	    );
	  }
	});
	
	module.exports = Sessions;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "main.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 310:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };
	
	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }
	
	var React = __webpack_require__(4);
	
	var _require = __webpack_require__(41);
	
	var Link = _require.Link;
	
	var LinkedStateMixin = __webpack_require__(40);
	
	var _require2 = __webpack_require__(226);
	
	var Table = _require2.Table;
	var Column = _require2.Column;
	var Cell = _require2.Cell;
	var TextCell = _require2.TextCell;
	var SortHeaderCell = _require2.SortHeaderCell;
	var SortTypes = _require2.SortTypes;
	
	var _require3 = __webpack_require__(44);
	
	var open = _require3.open;
	
	var moment = __webpack_require__(1);
	
	var _require4 = __webpack_require__(95);
	
	var isMatch = _require4.isMatch;
	
	var _ = __webpack_require__(59);
	
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
	
	  mixins: [LinkedStateMixin],
	
	  getInitialState: function getInitialState(props) {
	    this.searchableProps = ['serverIp', 'created', 'active'];
	    return { filter: '', colSortDirs: {} };
	  },
	
	  onSortChange: function onSortChange(columnKey, sortDir) {
	    var _colSortDirs;
	
	    this.setState(_extends({}, this.state, {
	      colSortDirs: (_colSortDirs = {}, _colSortDirs[columnKey] = sortDir, _colSortDirs)
	    }));
	  },
	
	  searchAndFilterCb: function searchAndFilterCb(targetValue, searchValue, propName) {
	    if (propName === 'created') {
	      var displayDate = moment(targetValue).fromNow().toLocaleUpperCase();
	      return displayDate.indexOf(searchValue) !== -1;
	    }
	  },
	
	  sortAndFilter: function sortAndFilter(data) {
	    var _this = this;
	
	    var filtered = data.filter(function (obj) {
	      return isMatch(obj, _this.state.filter, {
	        searchableProps: _this.searchableProps,
	        cb: _this.searchAndFilterCb
	      });
	    });
	
	    var columnKey = Object.getOwnPropertyNames(this.state.colSortDirs)[0];
	    var sortDir = this.state.colSortDirs[columnKey];
	    var sorted = _.sortBy(filtered, columnKey);
	    if (sortDir === SortTypes.ASC) {
	      sorted = sorted.reverse();
	    }
	
	    return sorted;
	  },
	
	  render: function render() {
	    var data = this.sortAndFilter(this.props.sessionRecords);
	    return React.createElement(
	      'div',
	      null,
	      React.createElement(
	        'div',
	        { className: 'grv-search' },
	        React.createElement('input', { valueLink: this.linkState('filter'), placeholder: 'Search...', className: 'form-control input-sm' })
	      ),
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
	            header: React.createElement(SortHeaderCell, {
	              sortDir: this.state.colSortDirs.serverIp,
	              onSortChange: this.onSortChange,
	              title: 'Node'
	            }),
	            cell: React.createElement(TextCell, { data: data })
	          }),
	          React.createElement(Column, {
	            columnKey: 'created',
	            header: React.createElement(SortHeaderCell, {
	              sortDir: this.state.colSortDirs.created,
	              onSortChange: this.onSortChange,
	              title: 'Created'
	            }),
	            cell: React.createElement(DateCreatedCell, { data: data })
	          }),
	          React.createElement(Column, {
	            columnKey: 'active',
	            header: React.createElement(SortHeaderCell, {
	              sortDir: this.state.colSortDirs.active,
	              onSortChange: this.onSortChange,
	              title: 'Active'
	            }),
	            cell: React.createElement(UsersCell, { data: data })
	          })
	        )
	      )
	    );
	  }
	});
	
	module.exports = SessionList;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "sessionList.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 311:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	var render = __webpack_require__(220).render;
	
	var _require = __webpack_require__(41);
	
	var Router = _require.Router;
	var Route = _require.Route;
	var Redirect = _require.Redirect;
	var IndexRoute = _require.IndexRoute;
	var browserHistory = _require.browserHistory;
	
	var _require2 = __webpack_require__(304);
	
	var App = _require2.App;
	var Login = _require2.Login;
	var Nodes = _require2.Nodes;
	var Sessions = _require2.Sessions;
	var NewUser = _require2.NewUser;
	var CurrentSessionHost = _require2.CurrentSessionHost;
	var NotFound = _require2.NotFound;
	
	var _require3 = __webpack_require__(115);
	
	var ensureUser = _require3.ensureUser;
	
	var auth = __webpack_require__(93);
	var session = __webpack_require__(26);
	var cfg = __webpack_require__(11);
	
	__webpack_require__(279);
	
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
	  React.createElement(Route, { path: '*', component: NotFound })
	), document.getElementById("app"));
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "index.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 411:
/***/ function(module, exports) {

	module.exports = Terminal;

/***/ }

});
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJzb3VyY2VzIjpbIndlYnBhY2s6Ly8vLi9zcmMvYXBwL3JlYWN0b3IuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb25maWcuanMiLCJ3ZWJwYWNrOi8vLy4vfi9rZXltaXJyb3IvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9zZXNzaW9uLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvc2VydmljZXMvYXBpLmpzIiwid2VicGFjazovLy9leHRlcm5hbCBcImpRdWVyeVwiIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9jb25zdGFudHMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vZXh0ZXJuYWwgXCJfXCIiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2F1dGguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21tb24vZGF0ZVV0aWxzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tbW9uL29iamVjdFV0aWxzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tbW9uL3R0eS5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvYWN0aW9uVHlwZXMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGl2ZVRlcm1TdG9yZS5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvYXBwL2FjdGlvblR5cGVzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hcHAvYXBwU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvYWN0aW9uVHlwZXMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvZGlhbG9nU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2ludml0ZVN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvbm9kZVN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2FjdGlvblR5cGVzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvYWN0aW9uVHlwZXMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9zZXNzaW9ucy9zZXNzaW9uU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvYWN0aW9uVHlwZXMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvdXNlci9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvdXNlci91c2VyU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2N1cnJlbnRTZXNzaW9uL3Nlc3Npb25MZWZ0UGFuZWwuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9lcnJvclBhZ2UuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9nb29nbGVBdXRoTG9nby5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL25vZGVzL25vZGVMaXN0LmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvc2VsZWN0Tm9kZURpYWxvZy5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL3RhYmxlLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvdGVybWluYWwuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tbW9uL3BhdHRlcm5VdGlscy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbW1vbi90dHlQbGF5ZXIuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FwcC9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hcHAvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvYXBwL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9kaWFsb2dzL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvcmVzdEFwaVN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvdXRpbHMuanMiLCJ3ZWJwYWNrOi8vLy4vfi9ldmVudHMvZXZlbnRzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9hcHAuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9jdXJyZW50U2Vzc2lvbi9hY3RpdmVTZXNzaW9uLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvY3VycmVudFNlc3Npb24vZXZlbnRTdHJlYW1lci5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2N1cnJlbnRTZXNzaW9uL21haW4uanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9jdXJyZW50U2Vzc2lvbi9zZXNzaW9uUGxheWVyLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvZGF0ZVBpY2tlci5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2luZGV4LmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvbG9naW4uanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9uYXZMZWZ0QmFyLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvbmV3VXNlci5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL25vZGVzL21haW4uanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9zZXNzaW9ucy9tYWluLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvc2Vzc2lvbnMvc2Vzc2lvbkxpc3QuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvaW5kZXguanN4Iiwid2VicGFjazovLy9leHRlcm5hbCBcIlRlcm1pbmFsXCIiXSwibmFtZXMiOltdLCJtYXBwaW5ncyI6Ijs7Ozs7Ozs7Ozs7Ozs7Ozs7c0NBQXdCLEVBQVk7O0FBRXBDLEtBQU0sT0FBTyxHQUFHLHVCQUFZO0FBQzFCLFFBQUssRUFBRSxJQUFJO0VBQ1osQ0FBQzs7QUFFRixPQUFNLENBQUMsT0FBTyxHQUFHLE9BQU8sQ0FBQzs7c0JBRVYsT0FBTzs7Ozs7Ozs7Ozs7O2dCQ1JBLG1CQUFPLENBQUMsR0FBeUIsQ0FBQzs7S0FBbkQsYUFBYSxZQUFiLGFBQWE7O0FBRWxCLEtBQUksR0FBRyxHQUFHOztBQUVSLFVBQU8sRUFBRSxNQUFNLENBQUMsUUFBUSxDQUFDLE1BQU07O0FBRS9CLFVBQU8sRUFBRSxpRUFBaUU7O0FBRTFFLE1BQUcsRUFBRTtBQUNILG1CQUFjLEVBQUMsMkJBQTJCO0FBQzFDLGNBQVMsRUFBRSxrQ0FBa0M7QUFDN0MsZ0JBQVcsRUFBRSxxQkFBcUI7QUFDbEMsb0JBQWUsRUFBRSwwQ0FBMEM7QUFDM0QsZUFBVSxFQUFFLHVDQUF1QztBQUNuRCxtQkFBYyxFQUFFLGtCQUFrQjtBQUNsQyxpQkFBWSxFQUFFLHVFQUF1RTtBQUNyRiwwQkFBcUIsRUFBRSxzREFBc0Q7O0FBRTdFLDRCQUF1QixFQUFFLGlDQUFDLElBQWlCLEVBQUc7V0FBbkIsR0FBRyxHQUFKLElBQWlCLENBQWhCLEdBQUc7V0FBRSxLQUFLLEdBQVgsSUFBaUIsQ0FBWCxLQUFLO1dBQUUsR0FBRyxHQUFoQixJQUFpQixDQUFKLEdBQUc7O0FBQ3hDLGNBQU8sYUFBYSxDQUFDLEdBQUcsQ0FBQyxHQUFHLENBQUMsWUFBWSxFQUFFLEVBQUMsR0FBRyxFQUFILEdB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QUFFLE9BQVEsR0FBRTtHQUN0RCxvQkFBQyxRQUFRLElBQUMsSUFBSSxFQUFFLEdBQUcsQ0FBQyxNQUFNLENBQUMsR0FBSSxFQUFDLEVBQUUsRUFBRSxHQUFHLENBQUMsTUFBTSxDQUFDLEtBQU0sR0FBRTtHQUN2RDtBQUFDLFVBQUs7T0FBQyxJQUFJLEVBQUUsR0FBRyxDQUFDLE1BQU0sQ0FBQyxHQUFJLEVBQUMsU0FBUyxFQUFFLEdBQUksRUFBQyxPQUFPLEVBQUUsVUFBVztLQUMvRCxvQkFBQyxLQUFLLElBQUMsSUFBSSxFQUFFLEdBQUcsQ0FBQyxNQUFNLENBQUMsS0FBTSxFQUFDLFNBQVMsRUFBRSxLQUFNLEdBQUU7S0FDbEQsb0JBQUMsS0FBSyxJQUFDLElBQUksRUFBRSxHQUFHLENBQUMsTUFBTSxDQUFDLGFBQWMsRUFBQyxVQUFVLEVBQUUsRUFBQyxrQkFBa0IsRUFBRSxrQkFBa0IsRUFBRSxHQUFFO0tBQzlGLG9CQUFDLEtBQUssSUFBQyxJQUFJLEVBQUUsR0FBRyxDQUFDLE1BQU0sQ0FBQyxRQUFTLEVBQUMsU0FBUyxFQUFFLFFBQVMsR0FBRTtJQUNsRDtHQUNSLG9CQUFDLEtBQUssSUFBQyxJQUFJLEVBQUMsR0FBRyxFQUFDLFNBQVMsRUFBRSxRQUFTLEdBQUc7RUFDaEMsRUFDUixRQUFRLENBQUMsY0FBYyxDQUFDLEtBQUssQ0FBQyxDQUFDLEM7Ozs7Ozs7OztBQy9CbEMsMkIiLCJmaWxlIjoiYXBwLmpzIiwic291cmNlc0NvbnRlbnQiOlsiaW1wb3J0IHsgUmVhY3RvciB9IGZyb20gJ251Y2xlYXItanMnXG5cbmNvbnN0IHJlYWN0b3IgPSBuZXcgUmVhY3Rvcih7XG4gIGRlYnVnOiB0cnVlXG59KVxuXG53aW5kb3cucmVhY3RvciA9IHJlYWN0b3I7XG5cbmV4cG9ydCBkZWZhdWx0IHJlYWN0b3JcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9yZWFjdG9yLmpzXG4gKiovIiwibGV0IHtmb3JtYXRQYXR0ZXJufSA9IHJlcXVpcmUoJ2FwcC9jb21tb24vcGF0dGVyblV0aWxzJyk7XG5cbmxldCBjZmcgPSB7XG5cbiAgYmFzZVVybDogd2luZG93LmxvY2F0aW9uLm9yaWdpbixcblxuICBoZWxwVXJsOiAnaHR0cHM6Ly9naXRodWIuY29tL2dyYXZpdGF0aW9uYWwvdGVsZXBvcnQvYmxvYi9tYXN0ZXIvUkVBRE1FLm1kJyxcblxuICBhcGk6IHtcbiAgICByZW5ld1Rva2VuUGF0aDonL3YxL3dlYmFwaS9zZXNzaW9ucy9yZW5ldycsXG4gICAgbm9kZXNQYXRoOiAnL3YxL3dlYmFwaS9zaXRlcy8tY3VycmVudC0vbm9kZXMnLFxuICAgIHNlc3Npb25QYXRoOiAnL3YxL3dlYmFwaS9zZXNzaW9ucycsXG4gICAgc2l0ZVNlc3Npb25QYXRoOiAnL3YxL3dlYmFwaS9zaXRlcy8tY3VycmVudC0vc2Vzc2lvbnMvOnNpZCcsXG4gICAgaW52aXRlUGF0aDogJy92MS93ZWJhcGkvdXNlcnMvaW52aXRlcy86aW52aXRlVG9rZW4nLFxuICAgIGNyZWF0ZVVzZXJQYXRoOiAnL3YxL3dlYmFwaS91c2VycycsXG4gICAgc2Vzc2lvbkNodW5rOiAnL3YxL3dlYmFwaS9zaXRlcy8tY3VycmVudC0vc2Vzc2lvbnMvOnNpZC9jaHVua3M/c3RhcnQ9OnN0YXJ0JmVuZD06ZW5kJyxcbiAgICBzZXNzaW9uQ2h1bmtDb3VudFBhdGg6ICcvdjEvd2ViYXBpL3NpdGVzLy1jdXJyZW50LS9zZXNzaW9ucy86c2lkL2NodW5rc2NvdW50JyxcblxuICAgIGdldEZldGNoU2Vzc2lvbkNodW5rVXJsOiAoe3NpZCwgc3RhcnQsIGVuZH0pPT57XG4gICAgICByZXR1cm4gZm9ybWF0UGF0dGVybihjZmcuYXBpLnNlc3Npb25DaHVuaywge3NpZCwgc3RhcnQsIGVuZH0pO1xuICAgIH0sXG5cbiAgICBnZXRGZXRjaFNlc3Npb25MZW5ndGhVcmw6IChzaWQpPT57XG4gICAgICByZXR1cm4gZm9ybWF0UGF0dGVybihjZmcuYXBpLnNlc3Npb25DaHVua0NvdW50UGF0aCwge3NpZH0pO1xuICAgIH0sXG5cbiAgICBnZXRGZXRjaFNlc3Npb25zVXJsOiAoc3RhcnQsIGVuZCk9PntcbiAgICAgIHZhciBwYXJhbXMgPSB7XG4gICAgICAgIHN0YXJ0OiBzdGFydC50b0lTT1N0cmluZygpLFxuICAgICAgICBlbmQ6IGVuZC50b0lTT1N0cmluZygpICAgICAgICBcbiAgICAgIH07XG5cbiAgICAgIHZhciBqc29uID0gSlNPTi5zdHJpbmdpZnkocGFyYW1zKTtcbiAgICAgIHZhciBqc29uRW5jb2RlZCA9IHdpbmRvdy5lbmNvZGVVUkkoanNvbik7XG5cbiAgICAgIHJldHVybiBgL3YxL3dlYmFwaS9zaXRlcy8tY3VycmVudC0vZXZlbnRzL3Nlc3Npb25zP2ZpbHRlcj0ke2pzb25FbmNvZGVkfWA7XG4gICAgfSxcblxuICAgIGdldEZldGNoU2Vzc2lvblVybDogKHNpZCk9PntcbiAgICAgIHJldHVybiBmb3JtYXRQYXR0ZXJuKGNmZy5hcGkuc2l0ZVNlc3Npb25QYXRoLCB7c2lkfSk7XG4gICAgfSxcblxuICAgIGdldFRlcm1pbmFsU2Vzc2lvblVybDogKHNpZCk9PiB7XG4gICAgICByZXR1cm4gZm9ybWF0UGF0dGVybihjZmcuYXBpLnNpdGVTZXNzaW9uUGF0aCwge3NpZH0pO1xuICAgIH0sXG5cbiAgICBnZXRJbnZpdGVVcmw6IChpbnZpdGVUb2tlbikgPT4ge1xuICAgICAgcmV0dXJuIGZvcm1hdFBhdHRlcm4oY2ZnLmFwaS5pbnZpdGVQYXRoLCB7aW52aXRlVG9rZW59KTtcbiAgICB9LFxuXG4gICAgZ2V0RXZlbnRTdHJlYW1Db25uU3RyOiAodG9rZW4sIHNpZCkgPT4ge1xuICAgICAgdmFyIGhvc3RuYW1lID0gZ2V0V3NIb3N0TmFtZSgpO1xuICAgICAgcmV0dXJuIGAke2hvc3RuYW1lfS92MS93ZWJhcGkvc2l0ZXMvLWN1cnJlbnQtL3Nlc3Npb25zLyR7c2lkfS9ldmVudHMvc3RyZWFtP2FjY2Vzc190b2tlbj0ke3Rva2VufWA7XG4gICAgfSxcblxuICAgIGdldFR0eUNvbm5TdHI6ICh7dG9rZW4sIHNlcnZlcklkLCBsb2dpbiwgc2lkLCByb3dzLCBjb2xzfSkgPT4ge1xuICAgICAgdmFyIHBhcmFtcyA9IHtcbiAgICAgICAgc2VydmVyX2lkOiBzZXJ2ZXJJZCxcbiAgICAgICAgbG9naW4sXG4gICAgICAgIHNpZCxcbiAgICAgICAgdGVybToge1xuICAgICAgICAgIGg6IHJvd3MsXG4gICAgICAgICAgdzogY29sc1xuICAgICAgICB9XG4gICAgICB9XG5cbiAgICAgIHZhciBqc29uID0gSlNPTi5zdHJpbmdpZnkocGFyYW1zKTtcbiAgICAgIHZhciBqc29uRW5jb2RlZCA9IHdpbmRvdy5lbmNvZGVVUkkoanNvbik7XG4gICAgICB2YXIgaG9zdG5hbWUgPSBnZXRXc0hvc3ROYW1lKCk7XG4gICAgICByZXR1cm4gYCR7aG9zdG5hbWV9L3YxL3dlYmFwaS9zaXRlcy8tY3VycmVudC0vY29ubmVjdD9hY2Nlc3NfdG9rZW49JHt0b2tlbn0mcGFyYW1zPSR7anNvbkVuY29kZWR9YDtcbiAgICB9XG4gIH0sXG5cbiAgcm91dGVzOiB7XG4gICAgYXBwOiAnL3dlYicsXG4gICAgbG9nb3V0OiAnL3dlYi9sb2dvdXQnLFxuICAgIGxvZ2luOiAnL3dlYi9sb2dpbicsXG4gICAgbm9kZXM6ICcvd2ViL25vZGVzJyxcbiAgICBhY3RpdmVTZXNzaW9uOiAnL3dlYi9zZXNzaW9ucy86c2lkJyxcbiAgICBuZXdVc2VyOiAnL3dlYi9uZXd1c2VyLzppbnZpdGVUb2tlbicsXG4gICAgc2Vzc2lvbnM6ICcvd2ViL3Nlc3Npb25zJyxcbiAgICBwYWdlTm90Rm91bmQ6ICcvd2ViL25vdGZvdW5kJ1xuICB9LFxuXG4gIGdldEFjdGl2ZVNlc3Npb25Sb3V0ZVVybChzaWQpe1xuICAgIHJldHVybiBmb3JtYXRQYXR0ZXJuKGNmZy5yb3V0ZXMuYWN0aXZlU2Vzc2lvbiwge3NpZH0pO1xuICB9XG59XG5cbmV4cG9ydCBkZWZhdWx0IGNmZztcblxuZnVuY3Rpb24gZ2V0V3NIb3N0TmFtZSgpe1xuICB2YXIgcHJlZml4ID0gbG9jYXRpb24ucHJvdG9jb2wgPT0gXCJodHRwczpcIj9cIndzczovL1wiOlwid3M6Ly9cIjtcbiAgdmFyIGhvc3Rwb3J0ID0gbG9jYXRpb24uaG9zdG5hbWUrKGxvY2F0aW9uLnBvcnQgPyAnOicrbG9jYXRpb24ucG9ydDogJycpO1xuICByZXR1cm4gYCR7cHJlZml4fSR7aG9zdHBvcnR9YDtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb25maWcuanNcbiAqKi8iLCIvKipcbiAqIENvcHlyaWdodCAyMDEzLTIwMTQgRmFjZWJvb2ssIEluYy5cbiAqXG4gKiBMaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xuICogeW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuICogWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG4gKlxuICogaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG4gKlxuICogVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuICogZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuICogV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG4gKiBTZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG4gKiBsaW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiAqXG4gKi9cblxuXCJ1c2Ugc3RyaWN0XCI7XG5cbi8qKlxuICogQ29uc3RydWN0cyBhbiBlbnVtZXJhdGlvbiB3aXRoIGtleXMgZXF1YWwgdG8gdGhlaXIgdmFsdWUuXG4gKlxuICogRm9yIGV4YW1wbGU6XG4gKlxuICogICB2YXIgQ09MT1JTID0ga2V5TWlycm9yKHtibHVlOiBudWxsLCByZWQ6IG51bGx9KTtcbiAqICAgdmFyIG15Q29sb3IgPSBDT0xPUlMuYmx1ZTtcbiAqICAgdmFyIGlzQ29sb3JWYWxpZCA9ICEhQ09MT1JTW215Q29sb3JdO1xuICpcbiAqIFRoZSBsYXN0IGxpbmUgY291bGQgbm90IGJlIHBlcmZvcm1lZCBpZiB0aGUgdmFsdWVzIG9mIHRoZSBnZW5lcmF0ZWQgZW51bSB3ZXJlXG4gKiBub3QgZXF1YWwgdG8gdGhlaXIga2V5cy5cbiAqXG4gKiAgIElucHV0OiAge2tleTE6IHZhbDEsIGtleTI6IHZhbDJ9XG4gKiAgIE91dHB1dDoge2tleTE6IGtleTEsIGtleTI6IGtleTJ9XG4gKlxuICogQHBhcmFtIHtvYmplY3R9IG9ialxuICogQHJldHVybiB7b2JqZWN0fVxuICovXG52YXIga2V5TWlycm9yID0gZnVuY3Rpb24ob2JqKSB7XG4gIHZhciByZXQgPSB7fTtcbiAgdmFyIGtleTtcbiAgaWYgKCEob2JqIGluc3RhbmNlb2YgT2JqZWN0ICYmICFBcnJheS5pc0FycmF5KG9iaikpKSB7XG4gICAgdGhyb3cgbmV3IEVycm9yKCdrZXlNaXJyb3IoLi4uKTogQXJndW1lbnQgbXVzdCBiZSBhbiBvYmplY3QuJyk7XG4gIH1cbiAgZm9yIChrZXkgaW4gb2JqKSB7XG4gICAgaWYgKCFvYmouaGFzT3duUHJvcGVydHkoa2V5KSkge1xuICAgICAgY29udGludWU7XG4gICAgfVxuICAgIHJldFtrZXldID0ga2V5O1xuICB9XG4gIHJldHVybiByZXQ7XG59O1xuXG5tb2R1bGUuZXhwb3J0cyA9IGtleU1pcnJvcjtcblxuXG5cbi8qKioqKioqKioqKioqKioqKlxuICoqIFdFQlBBQ0sgRk9PVEVSXG4gKiogLi9+L2tleW1pcnJvci9pbmRleC5qc1xuICoqIG1vZHVsZSBpZCA9IDIwXG4gKiogbW9kdWxlIGNodW5rcyA9IDFcbiAqKi8iLCJ2YXIgeyBicm93c2VySGlzdG9yeSwgY3JlYXRlTWVtb3J5SGlzdG9yeSB9ID0gcmVxdWlyZSgncmVhY3Qtcm91dGVyJyk7XG5cbmNvbnN0IEFVVEhfS0VZX0RBVEEgPSAnYXV0aERhdGEnO1xuXG52YXIgX2hpc3RvcnkgPSBjcmVhdGVNZW1vcnlIaXN0b3J5KCk7XG5cbnZhciBzZXNzaW9uID0ge1xuXG4gIGluaXQoaGlzdG9yeT1icm93c2VySGlzdG9yeSl7XG4gICAgX2hpc3RvcnkgPSBoaXN0b3J5O1xuICB9LFxuXG4gIGdldEhpc3RvcnkoKXtcbiAgICByZXR1cm4gX2hpc3Rvcnk7XG4gIH0sXG5cbiAgc2V0VXNlckRhdGEodXNlckRhdGEpe1xuICAgIGxvY2FsU3RvcmFnZS5zZXRJdGVtKEFVVEhfS0VZX0RBVEEsIEpTT04uc3RyaW5naWZ5KHVzZXJEYXRhKSk7XG4gIH0sXG5cbiAgZ2V0VXNlckRhdGEoKXtcbiAgICB2YXIgaXRlbSA9IGxvY2FsU3RvcmFnZS5nZXRJdGVtKEFVVEhfS0VZX0RBVEEpO1xuICAgIGlmKGl0ZW0pe1xuICAgICAgcmV0dXJuIEpTT04ucGFyc2UoaXRlbSk7XG4gICAgfVxuXG4gICAgcmV0dXJuIHt9O1xuICB9LFxuXG4gIGNsZWFyKCl7XG4gICAgbG9jYWxTdG9yYWdlLmNsZWFyKClcbiAgfVxuXG59XG5cbm1vZHVsZS5leHBvcnRzID0gc2Vzc2lvbjtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9zZXNzaW9uLmpzXG4gKiovIiwidmFyICQgPSByZXF1aXJlKFwialF1ZXJ5XCIpO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCdhcHAvc2Vzc2lvbicpO1xuXG5jb25zdCBhcGkgPSB7XG5cbiAgcHV0KHBhdGgsIGRhdGEsIHdpdGhUb2tlbil7XG4gICAgcmV0dXJuIGFwaS5hamF4KHt1cmw6IHBhdGgsIGRhdGE6IEpTT04uc3RyaW5naWZ5KGRhdGEpLCB0eXBlOiAnUFVUJ30sIHdpdGhUb2tlbik7XG4gIH0sXG5cbiAgcG9zdChwYXRoLCBkYXRhLCB3aXRoVG9rZW4pe1xuICAgIHJldHVybiBhcGkuYWpheCh7dXJsOiBwYXRoLCBkYXRhOiBKU09OLnN0cmluZ2lmeShkYXRhKSwgdHlwZTogJ1BPU1QnfSwgd2l0aFRva2VuKTtcbiAgfSxcblxuICBnZXQocGF0aCl7XG4gICAgcmV0dXJuIGFwaS5hamF4KHt1cmw6IHBhdGh9KTtcbiAgfSxcblxuICBhamF4KGNmZywgd2l0aFRva2VuID0gdHJ1ZSl7XG4gICAgdmFyIGRlZmF1bHRDZmcgPSB7XG4gICAgICB0eXBlOiBcIkdFVFwiLFxuICAgICAgZGF0YVR5cGU6IFwianNvblwiLFxuICAgICAgYmVmb3JlU2VuZDogZnVuY3Rpb24oeGhyKSB7XG4gICAgICAgIGlmKHdpdGhUb2tlbil7XG4gICAgICAgICAgdmFyIHsgdG9rZW4gfSA9IHNlc3Npb24uZ2V0VXNlckRhdGEoKTtcbiAgICAgICAgICB4aHIuc2V0UmVxdWVzdEhlYWRlcignQXV0aG9yaXphdGlvbicsJ0JlYXJlciAnICsgdG9rZW4pO1xuICAgICAgICB9XG4gICAgICAgfVxuICAgIH1cblxuICAgIHJldHVybiAkLmFqYXgoJC5leHRlbmQoe30sIGRlZmF1bHRDZmcsIGNmZykpO1xuICB9XG59XG5cbm1vZHVsZS5leHBvcnRzID0gYXBpO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL3NlcnZpY2VzL2FwaS5qc1xuICoqLyIsIm1vZHVsZS5leHBvcnRzID0galF1ZXJ5O1xuXG5cbi8qKioqKioqKioqKioqKioqKlxuICoqIFdFQlBBQ0sgRk9PVEVSXG4gKiogZXh0ZXJuYWwgXCJqUXVlcnlcIlxuICoqIG1vZHVsZSBpZCA9IDM5XG4gKiogbW9kdWxlIGNodW5rcyA9IDFcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgc2Vzc2lvbiA9IHJlcXVpcmUoJ2FwcC9zZXNzaW9uJyk7XG52YXIge3V1aWR9ID0gcmVxdWlyZSgnYXBwL3V0aWxzJyk7XG52YXIgYXBpID0gcmVxdWlyZSgnYXBwL3NlcnZpY2VzL2FwaScpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcbnZhciBnZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG52YXIgc2Vzc2lvbk1vZHVsZSA9IHJlcXVpcmUoJy4vLi4vc2Vzc2lvbnMnKTtcblxudmFyIHsgVExQVF9URVJNX09QRU4sIFRMUFRfVEVSTV9DTE9TRSwgVExQVF9URVJNX0NIQU5HRV9TRVJWRVIgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxudmFyIGFjdGlvbnMgPSB7XG5cbiAgY2hhbmdlU2VydmVyKHNlcnZlcklkLCBsb2dpbil7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1RFUk1fQ0hBTkdFX1NFUlZFUiwge1xuICAgICAgc2VydmVySWQsXG4gICAgICBsb2dpblxuICAgIH0pO1xuICB9LFxuXG4gIGNsb3NlKCl7XG4gICAgbGV0IHtpc05ld1Nlc3Npb259ID0gcmVhY3Rvci5ldmFsdWF0ZShnZXR0ZXJzLmFjdGl2ZVNlc3Npb24pO1xuXG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1RFUk1fQ0xPU0UpO1xuXG4gICAgaWYoaXNOZXdTZXNzaW9uKXtcbiAgICAgIHNlc3Npb24uZ2V0SGlzdG9yeSgpLnB1c2goY2ZnLnJvdXRlcy5ub2Rlcyk7XG4gICAgfWVsc2V7XG4gICAgICBzZXNzaW9uLmdldEhpc3RvcnkoKS5wdXNoKGNmZy5yb3V0ZXMuc2Vzc2lvbnMpO1xuICAgIH1cbiAgfSxcblxuICByZXNpemUodywgaCl7XG4gICAgLy8gc29tZSBtaW4gdmFsdWVzXG4gICAgdyA9IHcgPCA1ID8gNSA6IHc7XG4gICAgaCA9IGggPCA1ID8gNSA6IGg7XG5cbiAgICBsZXQgcmVxRGF0YSA9IHsgdGVybWluYWxfcGFyYW1zOiB7IHcsIGggfSB9O1xuICAgIGxldCB7c2lkfSA9IHJlYWN0b3IuZXZhbHVhdGUoZ2V0dGVycy5hY3RpdmVTZXNzaW9uKTtcblxuICAgIGFwaS5wdXQoY2ZnLmFwaS5nZXRUZXJtaW5hbFNlc3Npb25Vcmwoc2lkKSwgcmVxRGF0YSlcbiAgICAgIC5kb25lKCgpPT57XG4gICAgICAgIGNvbnNvbGUubG9nKGByZXNpemUgd2l0aCB3OiR7d30gYW5kIGg6JHtofSAtIE9LYCk7XG4gICAgICB9KVxuICAgICAgLmZhaWwoKCk9PntcbiAgICAgICAgY29uc29sZS5sb2coYGZhaWxlZCB0byByZXNpemUgd2l0aCB3OiR7d30gYW5kIGg6JHtofWApO1xuICAgIH0pXG4gIH0sXG5cbiAgb3BlblNlc3Npb24oc2lkKXtcbiAgICBzZXNzaW9uTW9kdWxlLmFjdGlvbnMuZmV0Y2hTZXNzaW9uKHNpZClcbiAgICAgIC5kb25lKCgpPT57XG4gICAgICAgIGxldCBzVmlldyA9IHJlYWN0b3IuZXZhbHVhdGUoc2Vzc2lvbk1vZHVsZS5nZXR0ZXJzLnNlc3Npb25WaWV3QnlJZChzaWQpKTtcbiAgICAgICAgbGV0IHsgc2VydmVySWQsIGxvZ2luIH0gPSBzVmlldztcbiAgICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1RFUk1fT1BFTiwge1xuICAgICAgICAgICAgc2VydmVySWQsXG4gICAgICAgICAgICBsb2dpbixcbiAgICAgICAgICAgIHNpZCxcbiAgICAgICAgICAgIGlzTmV3U2Vzc2lvbjogZmFsc2VcbiAgICAgICAgICB9KTtcbiAgICAgIH0pXG4gICAgICAuZmFpbCgoKT0+e1xuICAgICAgICBzZXNzaW9uLmdldEhpc3RvcnkoKS5wdXNoKGNmZy5yb3V0ZXMucGFnZU5vdEZvdW5kKTtcbiAgICAgIH0pXG4gIH0sXG5cbiAgY3JlYXRlTmV3U2Vzc2lvbihzZXJ2ZXJJZCwgbG9naW4pe1xuICAgIHZhciBzaWQgPSB1dWlkKCk7XG4gICAgdmFyIHJvdXRlVXJsID0gY2ZnLmdldEFjdGl2ZVNlc3Npb25Sb3V0ZVVybChzaWQpO1xuICAgIHZhciBoaXN0b3J5ID0gc2Vzc2lvbi5nZXRIaXN0b3J5KCk7XG5cbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfVEVSTV9PUEVOLCB7XG4gICAgICBzZXJ2ZXJJZCxcbiAgICAgIGxvZ2luLFxuICAgICAgc2lkLFxuICAgICAgaXNOZXdTZXNzaW9uOiB0cnVlXG4gICAgfSk7XG5cbiAgICBoaXN0b3J5LnB1c2gocm91dGVVcmwpO1xuICB9XG5cbn1cblxuZXhwb3J0IGRlZmF1bHQgYWN0aW9ucztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGlvbnMuanNcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cy5nZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3Rpb25zID0gcmVxdWlyZSgnLi9hY3Rpb25zJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3RpdmVUZXJtU3RvcmUgPSByZXF1aXJlKCcuL2FjdGl2ZVRlcm1TdG9yZScpO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvaW5kZXguanNcbiAqKi8iLCJpbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVFJZSU5HX1RPX1NJR05fVVA6IG51bGwsXG4gIFRSWUlOR19UT19MT0dJTjogbnVsbCxcbiAgRkVUQ0hJTkdfSU5WSVRFOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9jb25zdGFudHMuanNcbiAqKi8iLCJ2YXIge1RSWUlOR19UT19MT0dJTn0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9yZXN0QXBpL2NvbnN0YW50cycpO1xudmFyIHtyZXF1ZXN0U3RhdHVzfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3Jlc3RBcGkvZ2V0dGVycycpO1xuXG5jb25zdCB1c2VyID0gWyBbJ3RscHRfdXNlciddLCAoY3VycmVudFVzZXIpID0+IHtcbiAgICBpZighY3VycmVudFVzZXIpe1xuICAgICAgcmV0dXJuIG51bGw7XG4gICAgfVxuXG4gICAgdmFyIG5hbWUgPSBjdXJyZW50VXNlci5nZXQoJ25hbWUnKSB8fCAnJztcbiAgICB2YXIgc2hvcnREaXNwbGF5TmFtZSA9IG5hbWVbMF0gfHwgJyc7XG5cbiAgICByZXR1cm4ge1xuICAgICAgbmFtZSxcbiAgICAgIHNob3J0RGlzcGxheU5hbWUsXG4gICAgICBsb2dpbnM6IGN1cnJlbnRVc2VyLmdldCgnYWxsb3dlZF9sb2dpbnMnKS50b0pTKClcbiAgICB9XG4gIH1cbl07XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgdXNlcixcbiAgbG9naW5BdHRlbXA6IHJlcXVlc3RTdGF0dXMoVFJZSU5HX1RPX0xPR0lOKVxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvdXNlci9nZXR0ZXJzLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMgPSBfO1xuXG5cbi8qKioqKioqKioqKioqKioqKlxuICoqIFdFQlBBQ0sgRk9PVEVSXG4gKiogZXh0ZXJuYWwgXCJfXCJcbiAqKiBtb2R1bGUgaWQgPSA1OVxuICoqIG1vZHVsZSBjaHVua3MgPSAxXG4gKiovIiwidmFyIHtjcmVhdGVWaWV3fSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3Nlc3Npb25zL2dldHRlcnMnKTtcblxuY29uc3QgYWN0aXZlU2Vzc2lvbiA9IFtcblsndGxwdF9hY3RpdmVfdGVybWluYWwnXSwgWyd0bHB0X3Nlc3Npb25zJ10sXG4oYWN0aXZlVGVybSwgc2Vzc2lvbnMpID0+IHtcbiAgICBpZighYWN0aXZlVGVybSl7XG4gICAgICByZXR1cm4gbnVsbDtcbiAgICB9XG5cbiAgICAvKlxuICAgICogYWN0aXZlIHNlc3Npb24gbmVlZHMgdG8gaGF2ZSBpdHMgb3duIHZpZXcgYXMgYW4gYWN0dWFsIHNlc3Npb24gbWlnaHQgbm90XG4gICAgKiBleGlzdCBhdCB0aGlzIHBvaW50LiBGb3IgZXhhbXBsZSwgdXBvbiBjcmVhdGluZyBhIG5ldyBzZXNzaW9uIHdlIG5lZWQgdG8ga25vd1xuICAgICogbG9naW4gYW5kIHNlcnZlcklkLiBJdCB3aWxsIGJlIHNpbXBsaWZpZWQgb25jZSBzZXJ2ZXIgQVBJIGdldHMgZXh0ZW5kZWQuXG4gICAgKi9cbiAgICBsZXQgYXNWaWV3ID0ge1xuICAgICAgaXNOZXdTZXNzaW9uOiBhY3RpdmVUZXJtLmdldCgnaXNOZXdTZXNzaW9uJyksXG4gICAgICBub3RGb3VuZDogYWN0aXZlVGVybS5nZXQoJ25vdEZvdW5kJyksXG4gICAgICBhZGRyOiBhY3RpdmVUZXJtLmdldCgnYWRkcicpLFxuICAgICAgc2VydmVySWQ6IGFjdGl2ZVRlcm0uZ2V0KCdzZXJ2ZXJJZCcpLFxuICAgICAgc2VydmVySXA6IHVuZGVmaW5lZCxcbiAgICAgIGxvZ2luOiBhY3RpdmVUZXJtLmdldCgnbG9naW4nKSxcbiAgICAgIHNpZDogYWN0aXZlVGVybS5nZXQoJ3NpZCcpLFxuICAgICAgY29sczogdW5kZWZpbmVkLFxuICAgICAgcm93czogdW5kZWZpbmVkXG4gICAgfTtcblxuICAgIC8vIGluIGNhc2UgaWYgc2Vzc2lvbiBhbHJlYWR5IGV4aXN0cywgZ2V0IHRoZSBkYXRhIGZyb20gdGhlcmVcbiAgICAvLyAoZm9yIGV4YW1wbGUsIHdoZW4gam9pbmluZyBhbiBleGlzdGluZyBzZXNzaW9uKVxuICAgIGlmKHNlc3Npb25zLmhhcyhhc1ZpZXcuc2lkKSl7XG4gICAgICBsZXQgc1ZpZXcgPSBjcmVhdGVWaWV3KHNlc3Npb25zLmdldChhc1ZpZXcuc2lkKSk7XG5cbiAgICAgIGFzVmlldy5wYXJ0aWVzID0gc1ZpZXcucGFydGllcztcbiAgICAgIGFzVmlldy5zZXJ2ZXJJcCA9IHNWaWV3LnNlcnZlcklwO1xuICAgICAgYXNWaWV3LnNlcnZlcklkID0gc1ZpZXcuc2VydmVySWQ7XG4gICAgICBhc1ZpZXcuYWN0aXZlID0gc1ZpZXcuYWN0aXZlO1xuICAgICAgYXNWaWV3LmNvbHMgPSBzVmlldy5jb2xzO1xuICAgICAgYXNWaWV3LnJvd3MgPSBzVmlldy5yb3dzO1xuICAgIH1cblxuICAgIHJldHVybiBhc1ZpZXc7XG5cbiAgfVxuXTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICBhY3RpdmVTZXNzaW9uXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9nZXR0ZXJzLmpzXG4gKiovIiwidmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHsgVExQVF9ESUFMT0dfU0VMRUNUX05PREVfU0hPVywgVExQVF9ESUFMT0dfU0VMRUNUX05PREVfQ0xPU0UgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxudmFyIGFjdGlvbnMgPSB7XG4gIHNob3dTZWxlY3ROb2RlRGlhbG9nKCl7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX0RJQUxPR19TRUxFQ1RfTk9ERV9TSE9XKTtcbiAgfSxcblxuICBjbG9zZVNlbGVjdE5vZGVEaWFsb2coKXtcbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfRElBTE9HX1NFTEVDVF9OT0RFX0NMT1NFKTtcbiAgfVxufVxuXG5leHBvcnQgZGVmYXVsdCBhY3Rpb25zO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvZGlhbG9ncy9hY3Rpb25zLmpzXG4gKiovIiwidmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHtzZXNzaW9uc0J5U2VydmVyfSA9IHJlcXVpcmUoJy4vLi4vc2Vzc2lvbnMvZ2V0dGVycycpO1xuXG5jb25zdCBub2RlTGlzdFZpZXcgPSBbIFsndGxwdF9ub2RlcyddLCAobm9kZXMpID0+e1xuICAgIHJldHVybiBub2Rlcy5tYXAoKGl0ZW0pPT57XG4gICAgICB2YXIgc2VydmVySWQgPSBpdGVtLmdldCgnaWQnKTtcbiAgICAgIHZhciBzZXNzaW9ucyA9IHJlYWN0b3IuZXZhbHVhdGUoc2Vzc2lvbnNCeVNlcnZlcihzZXJ2ZXJJZCkpO1xuICAgICAgcmV0dXJuIHtcbiAgICAgICAgaWQ6IHNlcnZlcklkLFxuICAgICAgICBob3N0bmFtZTogaXRlbS5nZXQoJ2hvc3RuYW1lJyksXG4gICAgICAgIHRhZ3M6IGdldFRhZ3MoaXRlbSksXG4gICAgICAgIGFkZHI6IGl0ZW0uZ2V0KCdhZGRyJyksXG4gICAgICAgIHNlc3Npb25Db3VudDogc2Vzc2lvbnMuc2l6ZVxuICAgICAgfVxuICAgIH0pLnRvSlMoKTtcbiB9XG5dO1xuXG5mdW5jdGlvbiBnZXRUYWdzKG5vZGUpe1xuICB2YXIgYWxsTGFiZWxzID0gW107XG4gIHZhciBsYWJlbHMgPSBub2RlLmdldCgnbGFiZWxzJyk7XG5cbiAgaWYobGFiZWxzKXtcbiAgICBsYWJlbHMuZW50cnlTZXEoKS50b0FycmF5KCkuZm9yRWFjaChpdGVtPT57XG4gICAgICBhbGxMYWJlbHMucHVzaCh7XG4gICAgICAgIHJvbGU6IGl0ZW1bMF0sXG4gICAgICAgIHZhbHVlOiBpdGVtWzFdXG4gICAgICB9KTtcbiAgICB9KTtcbiAgfVxuXG4gIGxhYmVscyA9IG5vZGUuZ2V0KCdjbWRfbGFiZWxzJyk7XG5cbiAgaWYobGFiZWxzKXtcbiAgICBsYWJlbHMuZW50cnlTZXEoKS50b0FycmF5KCkuZm9yRWFjaChpdGVtPT57XG4gICAgICBhbGxMYWJlbHMucHVzaCh7XG4gICAgICAgIHJvbGU6IGl0ZW1bMF0sXG4gICAgICAgIHZhbHVlOiBpdGVtWzFdLmdldCgncmVzdWx0JyksXG4gICAgICAgIHRvb2x0aXA6IGl0ZW1bMV0uZ2V0KCdjb21tYW5kJylcbiAgICAgIH0pO1xuICAgIH0pO1xuICB9XG5cbiAgcmV0dXJuIGFsbExhYmVscztcbn1cblxuXG5leHBvcnQgZGVmYXVsdCB7XG4gIG5vZGVMaXN0Vmlld1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvZ2V0dGVycy5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciBhcGkgPSByZXF1aXJlKCdhcHAvc2VydmljZXMvYXBpJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xuXG52YXIgeyBUTFBUX1NFU1NJTlNfUkVDRUlWRSwgVExQVF9TRVNTSU5TX1VQREFURSB9ICA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuXG4gIGZldGNoU2Vzc2lvbihzaWQpe1xuICAgIHJldHVybiBhcGkuZ2V0KGNmZy5hcGkuZ2V0RmV0Y2hTZXNzaW9uVXJsKHNpZCkpLnRoZW4oanNvbj0+e1xuICAgICAgaWYoanNvbiAmJiBqc29uLnNlc3Npb24pe1xuICAgICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfU0VTU0lOU19VUERBVEUsIGpzb24uc2Vzc2lvbik7XG4gICAgICB9XG4gICAgfSk7XG4gIH0sXG5cbiAgZmV0Y2hTZXNzaW9ucyhzdGFydERhdGUsIGVuZERhdGUpe1xuICAgIHJldHVybiBhcGkuZ2V0KGNmZy5hcGkuZ2V0RmV0Y2hTZXNzaW9uc1VybChzdGFydERhdGUsIGVuZERhdGUpKS5kb25lKChqc29uKSA9PiB7XG4gICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfU0VTU0lOU19SRUNFSVZFLCBqc29uLnNlc3Npb25zKTtcbiAgICB9KTtcbiAgfSxcblxuICB1cGRhdGVTZXNzaW9uKGpzb24pe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9TRVNTSU5TX1VQREFURSwganNvbik7XG4gIH1cbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2FjdGlvbnMuanNcbiAqKi8iLCJ2YXIgeyB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxuY29uc3Qgc2Vzc2lvbnNCeVNlcnZlciA9IChzZXJ2ZXJJZCkgPT4gW1sndGxwdF9zZXNzaW9ucyddLCAoc2Vzc2lvbnMpID0+e1xuICByZXR1cm4gc2Vzc2lvbnMudmFsdWVTZXEoKS5maWx0ZXIoaXRlbT0+e1xuICAgIHZhciBwYXJ0aWVzID0gaXRlbS5nZXQoJ3BhcnRpZXMnKSB8fCB0b0ltbXV0YWJsZShbXSk7XG4gICAgdmFyIGhhc1NlcnZlciA9IHBhcnRpZXMuZmluZChpdGVtMj0+IGl0ZW0yLmdldCgnc2VydmVyX2lkJykgPT09IHNlcnZlcklkKTtcbiAgICByZXR1cm4gaGFzU2VydmVyO1xuICB9KS50b0xpc3QoKTtcbn1dXG5cbmNvbnN0IHNlc3Npb25zVmlldyA9IFtbJ3RscHRfc2Vzc2lvbnMnXSwgKHNlc3Npb25zKSA9PntcbiAgcmV0dXJuIHNlc3Npb25zLnZhbHVlU2VxKCkubWFwKGNyZWF0ZVZpZXcpLnRvSlMoKTtcbn1dO1xuXG5jb25zdCBzZXNzaW9uVmlld0J5SWQgPSAoc2lkKT0+IFtbJ3RscHRfc2Vzc2lvbnMnLCBzaWRdLCAoc2Vzc2lvbik9PntcbiAgaWYoIXNlc3Npb24pe1xuICAgIHJldHVybiBudWxsO1xuICB9XG5cbiAgcmV0dXJuIGNyZWF0ZVZpZXcoc2Vzc2lvbik7XG59XTtcblxuY29uc3QgcGFydGllc0J5U2Vzc2lvbklkID0gKHNpZCkgPT5cbiBbWyd0bHB0X3Nlc3Npb25zJywgc2lkLCAncGFydGllcyddLCAocGFydGllcykgPT57XG5cbiAgaWYoIXBhcnRpZXMpe1xuICAgIHJldHVybiBbXTtcbiAgfVxuXG4gIHZhciBsYXN0QWN0aXZlVXNyTmFtZSA9IGdldExhc3RBY3RpdmVVc2VyKHBhcnRpZXMpLmdldCgndXNlcicpO1xuXG4gIHJldHVybiBwYXJ0aWVzLm1hcChpdGVtPT57XG4gICAgdmFyIHVzZXIgPSBpdGVtLmdldCgndXNlcicpO1xuICAgIHJldHVybiB7XG4gICAgICB1c2VyOiBpdGVtLmdldCgndXNlcicpLFxuICAgICAgc2VydmVySXA6IGl0ZW0uZ2V0KCdyZW1vdGVfYWRkcicpLFxuICAgICAgc2VydmVySWQ6IGl0ZW0uZ2V0KCdzZXJ2ZXJfaWQnKSxcbiAgICAgIGlzQWN0aXZlOiBsYXN0QWN0aXZlVXNyTmFtZSA9PT0gdXNlclxuICAgIH1cbiAgfSkudG9KUygpO1xufV07XG5cbmZ1bmN0aW9uIGdldExhc3RBY3RpdmVVc2VyKHBhcnRpZXMpe1xuICByZXR1cm4gcGFydGllcy5zb3J0QnkoaXRlbT0+IG5ldyBEYXRlKGl0ZW0uZ2V0KCdsYXN0QWN0aXZlJykpKS5maXJzdCgpO1xufVxuXG5mdW5jdGlvbiBjcmVhdGVWaWV3KHNlc3Npb24pe1xuICB2YXIgc2lkID0gc2Vzc2lvbi5nZXQoJ2lkJyk7XG4gIHZhciBzZXJ2ZXJJcCwgc2VydmVySWQ7XG4gIHZhciBwYXJ0aWVzID0gcmVhY3Rvci5ldmFsdWF0ZShwYXJ0aWVzQnlTZXNzaW9uSWQoc2lkKSk7XG5cbiAgaWYocGFydGllcy5sZW5ndGggPiAwKXtcbiAgICBzZXJ2ZXJJcCA9IHBhcnRpZXNbMF0uc2VydmVySXA7XG4gICAgc2VydmVySWQgPSBwYXJ0aWVzWzBdLnNlcnZlcklkO1xuICB9XG5cbiAgcmV0dXJuIHtcbiAgICBzaWQ6IHNpZCxcbiAgICBzZXNzaW9uVXJsOiBjZmcuZ2V0QWN0aXZlU2Vzc2lvblJvdXRlVXJsKHNpZCksXG4gICAgc2VydmVySXAsXG4gICAgc2VydmVySWQsXG4gICAgYWN0aXZlOiBzZXNzaW9uLmdldCgnYWN0aXZlJyksXG4gICAgY3JlYXRlZDogbmV3IERhdGUoc2Vzc2lvbi5nZXQoJ2NyZWF0ZWQnKSksXG4gICAgbGFzdEFjdGl2ZTogbmV3IERhdGUoc2Vzc2lvbi5nZXQoJ2xhc3RfYWN0aXZlJykpLFxuICAgIGxvZ2luOiBzZXNzaW9uLmdldCgnbG9naW4nKSxcbiAgICBwYXJ0aWVzOiBwYXJ0aWVzLFxuICAgIGNvbHM6IHNlc3Npb24uZ2V0SW4oWyd0ZXJtaW5hbF9wYXJhbXMnLCAndyddKSxcbiAgICByb3dzOiBzZXNzaW9uLmdldEluKFsndGVybWluYWxfcGFyYW1zJywgJ2gnXSlcbiAgfVxufVxuXG5leHBvcnQgZGVmYXVsdCB7XG4gIHBhcnRpZXNCeVNlc3Npb25JZCxcbiAgc2Vzc2lvbnNCeVNlcnZlcixcbiAgc2Vzc2lvbnNWaWV3LFxuICBzZXNzaW9uVmlld0J5SWQsXG4gIGNyZWF0ZVZpZXdcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2dldHRlcnMuanNcbiAqKi8iLCJ2YXIgYXBpID0gcmVxdWlyZSgnLi9zZXJ2aWNlcy9hcGknKTtcbnZhciBzZXNzaW9uID0gcmVxdWlyZSgnLi9zZXNzaW9uJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xudmFyICQgPSByZXF1aXJlKCdqUXVlcnknKTtcblxuY29uc3QgcmVmcmVzaFJhdGUgPSA2MDAwMCAqIDU7IC8vIDEgbWluXG5cbnZhciByZWZyZXNoVG9rZW5UaW1lcklkID0gbnVsbDtcblxudmFyIGF1dGggPSB7XG5cbiAgc2lnblVwKG5hbWUsIHBhc3N3b3JkLCB0b2tlbiwgaW52aXRlVG9rZW4pe1xuICAgIHZhciBkYXRhID0ge3VzZXI6IG5hbWUsIHBhc3M6IHBhc3N3b3JkLCBzZWNvbmRfZmFjdG9yX3Rva2VuOiB0b2tlbiwgaW52aXRlX3Rva2VuOiBpbnZpdGVUb2tlbn07XG4gICAgcmV0dXJuIGFwaS5wb3N0KGNmZy5hcGkuY3JlYXRlVXNlclBhdGgsIGRhdGEpXG4gICAgICAudGhlbigodXNlcik9PntcbiAgICAgICAgc2Vzc2lvbi5zZXRVc2VyRGF0YSh1c2VyKTtcbiAgICAgICAgYXV0aC5fc3RhcnRUb2tlblJlZnJlc2hlcigpO1xuICAgICAgICByZXR1cm4gdXNlcjtcbiAgICAgIH0pO1xuICB9LFxuXG4gIGxvZ2luKG5hbWUsIHBhc3N3b3JkLCB0b2tlbil7XG4gICAgYXV0aC5fc3RvcFRva2VuUmVmcmVzaGVyKCk7XG4gICAgcmV0dXJuIGF1dGguX2xvZ2luKG5hbWUsIHBhc3N3b3JkLCB0b2tlbikuZG9uZShhdXRoLl9zdGFydFRva2VuUmVmcmVzaGVyKTtcbiAgfSxcblxuICBlbnN1cmVVc2VyKCl7XG4gICAgdmFyIHVzZXJEYXRhID0gc2Vzc2lvbi5nZXRVc2VyRGF0YSgpO1xuICAgIGlmKHVzZXJEYXRhLnRva2VuKXtcbiAgICAgIC8vIHJlZnJlc2ggdGltZXIgd2lsbCBub3QgYmUgc2V0IGluIGNhc2Ugb2YgYnJvd3NlciByZWZyZXNoIGV2ZW50XG4gICAgICBpZihhdXRoLl9nZXRSZWZyZXNoVG9rZW5UaW1lcklkKCkgPT09IG51bGwpe1xuICAgICAgICByZXR1cm4gYXV0aC5fcmVmcmVzaFRva2VuKCkuZG9uZShhdXRoLl9zdGFydFRva2VuUmVmcmVzaGVyKTtcbiAgICAgIH1cblxuICAgICAgcmV0dXJuICQuRGVmZXJyZWQoKS5yZXNvbHZlKHVzZXJEYXRhKTtcbiAgICB9XG5cbiAgICByZXR1cm4gJC5EZWZlcnJlZCgpLnJlamVjdCgpO1xuICB9LFxuXG4gIGxvZ291dCgpe1xuICAgIGF1dGguX3N0b3BUb2tlblJlZnJlc2hlcigpO1xuICAgIHNlc3Npb24uY2xlYXIoKTtcbiAgICBzZXNzaW9uLmdldEhpc3RvcnkoKS5yZXBsYWNlKHtwYXRobmFtZTogY2ZnLnJvdXRlcy5sb2dpbn0pOyAgICBcbiAgfSxcblxuICBfc3RhcnRUb2tlblJlZnJlc2hlcigpe1xuICAgIHJlZnJlc2hUb2tlblRpbWVySWQgPSBzZXRJbnRlcnZhbChhdXRoLl9yZWZyZXNoVG9rZW4sIHJlZnJlc2hSYXRlKTtcbiAgfSxcblxuICBfc3RvcFRva2VuUmVmcmVzaGVyKCl7XG4gICAgY2xlYXJJbnRlcnZhbChyZWZyZXNoVG9rZW5UaW1lcklkKTtcbiAgICByZWZyZXNoVG9rZW5UaW1lcklkID0gbnVsbDtcbiAgfSxcblxuICBfZ2V0UmVmcmVzaFRva2VuVGltZXJJZCgpe1xuICAgIHJldHVybiByZWZyZXNoVG9rZW5UaW1lcklkO1xuICB9LFxuXG4gIF9yZWZyZXNoVG9rZW4oKXtcbiAgICByZXR1cm4gYXBpLnBvc3QoY2ZnLmFwaS5yZW5ld1Rva2VuUGF0aCkudGhlbihkYXRhPT57XG4gICAgICBzZXNzaW9uLnNldFVzZXJEYXRhKGRhdGEpO1xuICAgICAgcmV0dXJuIGRhdGE7XG4gICAgfSkuZmFpbCgoKT0+e1xuICAgICAgYXV0aC5sb2dvdXQoKTtcbiAgICB9KTtcbiAgfSxcblxuICBfbG9naW4obmFtZSwgcGFzc3dvcmQsIHRva2VuKXtcbiAgICB2YXIgZGF0YSA9IHtcbiAgICAgIHVzZXI6IG5hbWUsXG4gICAgICBwYXNzOiBwYXNzd29yZCxcbiAgICAgIHNlY29uZF9mYWN0b3JfdG9rZW46IHRva2VuXG4gICAgfTtcblxuICAgIHJldHVybiBhcGkucG9zdChjZmcuYXBpLnNlc3Npb25QYXRoLCBkYXRhLCBmYWxzZSkudGhlbihkYXRhPT57XG4gICAgICBzZXNzaW9uLnNldFVzZXJEYXRhKGRhdGEpO1xuICAgICAgcmV0dXJuIGRhdGE7XG4gICAgfSk7XG4gIH1cbn1cblxubW9kdWxlLmV4cG9ydHMgPSBhdXRoO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2F1dGguanNcbiAqKi8iLCJ2YXIgbW9tZW50ID0gcmVxdWlyZSgnbW9tZW50Jyk7XG5cbm1vZHVsZS5leHBvcnRzLm1vbnRoUmFuZ2UgPSBmdW5jdGlvbih2YWx1ZSA9IG5ldyBEYXRlKCkpe1xuICBsZXQgc3RhcnREYXRlID0gbW9tZW50KHZhbHVlKS5zdGFydE9mKCdtb250aCcpLnRvRGF0ZSgpO1xuICBsZXQgZW5kRGF0ZSA9IG1vbWVudCh2YWx1ZSkuZW5kT2YoJ21vbnRoJykudG9EYXRlKCk7XG4gIHJldHVybiBbc3RhcnREYXRlLCBlbmREYXRlXTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21tb24vZGF0ZVV0aWxzLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMuaXNNYXRjaCA9IGZ1bmN0aW9uKG9iaiwgc2VhcmNoVmFsdWUsIHtzZWFyY2hhYmxlUHJvcHMsIGNifSkge1xuICBzZWFyY2hWYWx1ZSA9IHNlYXJjaFZhbHVlLnRvTG9jYWxlVXBwZXJDYXNlKCk7XG4gIGxldCBwcm9wTmFtZXMgPSBzZWFyY2hhYmxlUHJvcHMgfHwgT2JqZWN0LmdldE93blByb3BlcnR5TmFtZXMob2JqKTtcbiAgZm9yIChsZXQgaSA9IDA7IGkgPCBwcm9wTmFtZXMubGVuZ3RoOyBpKyspIHtcbiAgICBsZXQgdGFyZ2V0VmFsdWUgPSBvYmpbcHJvcE5hbWVzW2ldXTtcbiAgICBpZiAodGFyZ2V0VmFsdWUpIHtcbiAgICAgIGlmKHR5cGVvZiBjYiA9PT0gJ2Z1bmN0aW9uJyl7XG4gICAgICAgIGxldCByZXN1bHQgPSBjYih0YXJnZXRWYWx1ZSwgc2VhcmNoVmFsdWUsIHByb3BOYW1lc1tpXSk7XG4gICAgICAgIGlmKHJlc3VsdCAhPT0gdW5kZWZpbmVkKXtcbiAgICAgICAgICByZXR1cm4gcmVzdWx0O1xuICAgICAgICB9XG4gICAgICB9XG5cbiAgICAgIGlmICh0YXJnZXRWYWx1ZS50b1N0cmluZygpLnRvTG9jYWxlVXBwZXJDYXNlKCkuaW5kZXhPZihzZWFyY2hWYWx1ZSkgIT09IC0xKSB7XG4gICAgICAgIHJldHVybiB0cnVlO1xuICAgICAgfVxuICAgIH1cbiAgfVxuXG4gIHJldHVybiBmYWxzZTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21tb24vb2JqZWN0VXRpbHMuanNcbiAqKi8iLCJ2YXIgRXZlbnRFbWl0dGVyID0gcmVxdWlyZSgnZXZlbnRzJykuRXZlbnRFbWl0dGVyO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCdhcHAvc2Vzc2lvbicpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcbnZhciB7YWN0aW9uc30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC8nKTtcblxuY2xhc3MgVHR5IGV4dGVuZHMgRXZlbnRFbWl0dGVyIHtcblxuICBjb25zdHJ1Y3Rvcih7c2VydmVySWQsIGxvZ2luLCBzaWQsIHJvd3MsIGNvbHMgfSl7XG4gICAgc3VwZXIoKTtcbiAgICB0aGlzLm9wdGlvbnMgPSB7IHNlcnZlcklkLCBsb2dpbiwgc2lkLCByb3dzLCBjb2xzIH07XG4gICAgdGhpcy5zb2NrZXQgPSBudWxsO1xuICB9XG5cbiAgZGlzY29ubmVjdCgpe1xuICAgIHRoaXMuc29ja2V0LmNsb3NlKCk7XG4gIH1cblxuICByZWNvbm5lY3Qob3B0aW9ucyl7XG4gICAgdGhpcy5kaXNjb25uZWN0KCk7XG4gICAgdGhpcy5zb2NrZXQub25vcGVuID0gbnVsbDtcbiAgICB0aGlzLnNvY2tldC5vbm1lc3NhZ2UgPSBudWxsO1xuICAgIHRoaXMuc29ja2V0Lm9uY2xvc2UgPSBudWxsO1xuICAgIFxuICAgIHRoaXMuY29ubmVjdChvcHRpb25zKTtcbiAgfVxuXG4gIGNvbm5lY3Qob3B0aW9ucyl7XG4gICAgT2JqZWN0LmFzc2lnbih0aGlzLm9wdGlvbnMsIG9wdGlvbnMpO1xuXG4gICAgbGV0IHt0b2tlbn0gPSBzZXNzaW9uLmdldFVzZXJEYXRhKCk7XG4gICAgbGV0IGNvbm5TdHIgPSBjZmcuYXBpLmdldFR0eUNvbm5TdHIoe3Rva2VuLCAuLi50aGlzLm9wdGlvbnN9KTtcblxuICAgIHRoaXMuc29ja2V0ID0gbmV3IFdlYlNvY2tldChjb25uU3RyLCAncHJvdG8nKTtcblxuICAgIHRoaXMuc29ja2V0Lm9ub3BlbiA9ICgpID0+IHtcbiAgICAgIHRoaXMuZW1pdCgnb3BlbicpO1xuICAgIH1cblxuICAgIHRoaXMuc29ja2V0Lm9ubWVzc2FnZSA9IChlKT0+e1xuICAgICAgdGhpcy5lbWl0KCdkYXRhJywgZS5kYXRhKTtcbiAgICB9XG5cbiAgICB0aGlzLnNvY2tldC5vbmNsb3NlID0gKCk9PntcbiAgICAgIHRoaXMuZW1pdCgnY2xvc2UnKTtcbiAgICB9XG4gIH1cblxuICByZXNpemUoY29scywgcm93cyl7XG4gICAgYWN0aW9ucy5yZXNpemUoY29scywgcm93cyk7XG4gIH1cblxuICBzZW5kKGRhdGEpe1xuICAgIHRoaXMuc29ja2V0LnNlbmQoZGF0YSk7XG4gIH1cbn1cblxubW9kdWxlLmV4cG9ydHMgPSBUdHk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tbW9uL3R0eS5qc1xuICoqLyIsImltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX1RFUk1fT1BFTjogbnVsbCxcbiAgVExQVF9URVJNX0NMT1NFOiBudWxsLFxuICBUTFBUX1RFUk1fQ0hBTkdFX1NFUlZFUjogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGlvblR5cGVzLmpzXG4gKiovIiwidmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG52YXIgeyBUTFBUX1RFUk1fT1BFTiwgVExQVF9URVJNX0NMT1NFLCBUTFBUX1RFUk1fQ0hBTkdFX1NFUlZFUiB9ICA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKG51bGwpO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX1RFUk1fT1BFTiwgc2V0QWN0aXZlVGVybWluYWwpO1xuICAgIHRoaXMub24oVExQVF9URVJNX0NMT1NFLCBjbG9zZSk7XG4gICAgdGhpcy5vbihUTFBUX1RFUk1fQ0hBTkdFX1NFUlZFUiwgY2hhbmdlU2VydmVyKTtcbiAgfVxufSlcblxuZnVuY3Rpb24gY2hhbmdlU2VydmVyKHN0YXRlLCB7c2VydmVySWQsIGxvZ2lufSl7XG4gIHJldHVybiBzdGF0ZS5zZXQoJ3NlcnZlcklkJywgc2VydmVySWQpXG4gICAgICAgICAgICAgIC5zZXQoJ2xvZ2luJywgbG9naW4pO1xufVxuXG5mdW5jdGlvbiBjbG9zZSgpe1xuICByZXR1cm4gdG9JbW11dGFibGUobnVsbCk7XG59XG5cbmZ1bmN0aW9uIHNldEFjdGl2ZVRlcm1pbmFsKHN0YXRlLCB7c2VydmVySWQsIGxvZ2luLCBzaWQsIGlzTmV3U2Vzc2lvbn0gKXtcbiAgcmV0dXJuIHRvSW1tdXRhYmxlKHtcbiAgICBzZXJ2ZXJJZCxcbiAgICBsb2dpbixcbiAgICBzaWQsXG4gICAgaXNOZXdTZXNzaW9uXG4gIH0pO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvYWN0aXZlVGVybVN0b3JlLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfQVBQX0lOSVQ6IG51bGwsXG4gIFRMUFRfQVBQX0ZBSUxFRDogbnVsbCxcbiAgVExQVF9BUFBfUkVBRFk6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hcHAvYWN0aW9uVHlwZXMuanNcbiAqKi8iLCJ2YXIgeyBTdG9yZSwgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcblxudmFyIHsgVExQVF9BUFBfSU5JVCwgVExQVF9BUFBfRkFJTEVELCBUTFBUX0FQUF9SRUFEWSB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG52YXIgaW5pdFN0YXRlID0gdG9JbW11dGFibGUoe1xuICBpc1JlYWR5OiBmYWxzZSxcbiAgaXNJbml0aWFsaXppbmc6IGZhbHNlLFxuICBpc0ZhaWxlZDogZmFsc2Vcbn0pO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG5cbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiBpbml0U3RhdGUuc2V0KCdpc0luaXRpYWxpemluZycsIHRydWUpO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX0FQUF9JTklULCAoKT0+IGluaXRTdGF0ZS5zZXQoJ2lzSW5pdGlhbGl6aW5nJywgdHJ1ZSkpO1xuICAgIHRoaXMub24oVExQVF9BUFBfUkVBRFksKCk9PiBpbml0U3RhdGUuc2V0KCdpc1JlYWR5JywgdHJ1ZSkpO1xuICAgIHRoaXMub24oVExQVF9BUFBfRkFJTEVELCgpPT4gaW5pdFN0YXRlLnNldCgnaXNGYWlsZWQnLCB0cnVlKSk7XG4gIH1cbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hcHAvYXBwU3RvcmUuanNcbiAqKi8iLCJpbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVExQVF9ESUFMT0dfU0VMRUNUX05PREVfU0hPVzogbnVsbCxcbiAgVExQVF9ESUFMT0dfU0VMRUNUX05PREVfQ0xPU0U6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9kaWFsb2dzL2FjdGlvblR5cGVzLmpzXG4gKiovIiwidmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG5cbnZhciB7IFRMUFRfRElBTE9HX1NFTEVDVF9OT0RFX1NIT1csIFRMUFRfRElBTE9HX1NFTEVDVF9OT0RFX0NMT1NFIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IFN0b3JlKHtcblxuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKHtcbiAgICAgIGlzU2VsZWN0Tm9kZURpYWxvZ09wZW46IGZhbHNlXG4gICAgfSk7XG4gIH0sXG5cbiAgaW5pdGlhbGl6ZSgpIHtcbiAgICB0aGlzLm9uKFRMUFRfRElBTE9HX1NFTEVDVF9OT0RFX1NIT1csIHNob3dTZWxlY3ROb2RlRGlhbG9nKTtcbiAgICB0aGlzLm9uKFRMUFRfRElBTE9HX1NFTEVDVF9OT0RFX0NMT1NFLCBjbG9zZVNlbGVjdE5vZGVEaWFsb2cpO1xuICB9XG59KVxuXG5mdW5jdGlvbiBzaG93U2VsZWN0Tm9kZURpYWxvZyhzdGF0ZSl7XG4gIHJldHVybiBzdGF0ZS5zZXQoJ2lzU2VsZWN0Tm9kZURpYWxvZ09wZW4nLCB0cnVlKTtcbn1cblxuZnVuY3Rpb24gY2xvc2VTZWxlY3ROb2RlRGlhbG9nKHN0YXRlKXtcbiAgcmV0dXJuIHN0YXRlLnNldCgnaXNTZWxlY3ROb2RlRGlhbG9nT3BlbicsIGZhbHNlKTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvZGlhbG9nU3RvcmUuanNcbiAqKi8iLCJpbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVExQVF9SRUNFSVZFX1VTRVJfSU5WSVRFOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2FjdGlvblR5cGVzLmpzXG4gKiovIiwidmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG52YXIgIHsgVExQVF9SRUNFSVZFX1VTRVJfSU5WSVRFIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IFN0b3JlKHtcbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB0b0ltbXV0YWJsZShudWxsKTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9SRUNFSVZFX1VTRVJfSU5WSVRFLCByZWNlaXZlSW52aXRlKVxuICB9XG59KVxuXG5mdW5jdGlvbiByZWNlaXZlSW52aXRlKHN0YXRlLCBpbnZpdGUpe1xuICByZXR1cm4gdG9JbW11dGFibGUoaW52aXRlKTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9pbnZpdGVTdG9yZS5qc1xuICoqLyIsImltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX05PREVTX1JFQ0VJVkU6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9hY3Rpb25UeXBlcy5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7IFRMUFRfTk9ERVNfUkVDRUlWRSB9ICA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcbnZhciBhcGkgPSByZXF1aXJlKCdhcHAvc2VydmljZXMvYXBpJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xuXG5leHBvcnQgZGVmYXVsdCB7XG4gIGZldGNoTm9kZXMoKXtcbiAgICBhcGkuZ2V0KGNmZy5hcGkubm9kZXNQYXRoKS5kb25lKChkYXRhPVtdKT0+e1xuICAgICAgdmFyIG5vZGVBcnJheSA9IGRhdGEubm9kZXMubWFwKGl0ZW09Pml0ZW0ubm9kZSk7XG4gICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfTk9ERVNfUkVDRUlWRSwgbm9kZUFycmF5KTtcbiAgICB9KTtcbiAgfVxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvYWN0aW9ucy5qc1xuICoqLyIsInZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyICB7IFRMUFRfTk9ERVNfUkVDRUlWRSB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gdG9JbW11dGFibGUoW10pO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX05PREVTX1JFQ0VJVkUsIHJlY2VpdmVOb2RlcylcbiAgfVxufSlcblxuZnVuY3Rpb24gcmVjZWl2ZU5vZGVzKHN0YXRlLCBub2RlQXJyYXkpe1xuICByZXR1cm4gdG9JbW11dGFibGUobm9kZUFycmF5KTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL25vZGVTdG9yZS5qc1xuICoqLyIsImltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX1JFU1RfQVBJX1NUQVJUOiBudWxsLFxuICBUTFBUX1JFU1RfQVBJX1NVQ0NFU1M6IG51bGwsXG4gIFRMUFRfUkVTVF9BUElfRkFJTDogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvYWN0aW9uVHlwZXMuanNcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG5cbnZhciB7XG4gIFRMUFRfUkVTVF9BUElfU1RBUlQsXG4gIFRMUFRfUkVTVF9BUElfU1VDQ0VTUyxcbiAgVExQVF9SRVNUX0FQSV9GQUlMIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IHtcblxuICBzdGFydChyZXFUeXBlKXtcbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfUkVTVF9BUElfU1RBUlQsIHt0eXBlOiByZXFUeXBlfSk7XG4gIH0sXG5cbiAgZmFpbChyZXFUeXBlLCBtZXNzYWdlKXtcbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfUkVTVF9BUElfRkFJTCwgIHt0eXBlOiByZXFUeXBlLCBtZXNzYWdlfSk7XG4gIH0sXG5cbiAgc3VjY2VzcyhyZXFUeXBlKXtcbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfUkVTVF9BUElfU1VDQ0VTUywge3R5cGU6IHJlcVR5cGV9KTtcbiAgfVxuXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2FjdGlvbnMuanNcbiAqKi8iLCJ2YXIgZGVmYXVsdE9iaiA9IHtcbiAgaXNQcm9jZXNzaW5nOiBmYWxzZSxcbiAgaXNFcnJvcjogZmFsc2UsXG4gIGlzU3VjY2VzczogZmFsc2UsXG4gIG1lc3NhZ2U6ICcnXG59XG5cbmNvbnN0IHJlcXVlc3RTdGF0dXMgPSAocmVxVHlwZSkgPT4gIFsgWyd0bHB0X3Jlc3RfYXBpJywgcmVxVHlwZV0sIChhdHRlbXApID0+IHtcbiAgcmV0dXJuIGF0dGVtcCA/IGF0dGVtcC50b0pTKCkgOiBkZWZhdWx0T2JqO1xuIH1cbl07XG5cbmV4cG9ydCBkZWZhdWx0IHsgIHJlcXVlc3RTdGF0dXMgIH07XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2dldHRlcnMuanNcbiAqKi8iLCJpbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVExQVF9TRVNTSU5TX1JFQ0VJVkU6IG51bGwsXG4gIFRMUFRfU0VTU0lOU19VUERBVEU6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9zZXNzaW9ucy9hY3Rpb25UeXBlcy5qc1xuICoqLyIsIm1vZHVsZS5leHBvcnRzLmdldHRlcnMgPSByZXF1aXJlKCcuL2dldHRlcnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGlvbnMgPSByZXF1aXJlKCcuL2FjdGlvbnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGl2ZVRlcm1TdG9yZSA9IHJlcXVpcmUoJy4vc2Vzc2lvblN0b3JlJyk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9zZXNzaW9ucy9pbmRleC5qc1xuICoqLyIsInZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyIHsgVExQVF9TRVNTSU5TX1JFQ0VJVkUsIFRMUFRfU0VTU0lOU19VUERBVEUgfSAgPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IFN0b3JlKHtcbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB0b0ltbXV0YWJsZSh7fSk7XG4gIH0sXG5cbiAgaW5pdGlhbGl6ZSgpIHtcbiAgICB0aGlzLm9uKFRMUFRfU0VTU0lOU19SRUNFSVZFLCByZWNlaXZlU2Vzc2lvbnMpO1xuICAgIHRoaXMub24oVExQVF9TRVNTSU5TX1VQREFURSwgdXBkYXRlU2Vzc2lvbik7XG4gIH1cbn0pXG5cbmZ1bmN0aW9uIHVwZGF0ZVNlc3Npb24oc3RhdGUsIGpzb24pe1xuICByZXR1cm4gc3RhdGUuc2V0KGpzb24uaWQsIHRvSW1tdXRhYmxlKGpzb24pKTtcbn1cblxuZnVuY3Rpb24gcmVjZWl2ZVNlc3Npb25zKHN0YXRlLCBqc29uQXJyYXk9W10pe1xuICByZXR1cm4gc3RhdGUud2l0aE11dGF0aW9ucyhzdGF0ZSA9PiB7XG4gICAganNvbkFycmF5LmZvckVhY2goKGl0ZW0pID0+IHtcbiAgICAgIHN0YXRlLnNldChpdGVtLmlkLCB0b0ltbXV0YWJsZShpdGVtKSlcbiAgICB9KVxuICB9KTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL3Nlc3Npb25TdG9yZS5qc1xuICoqLyIsImltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX1JFQ0VJVkVfVVNFUjogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvYWN0aW9uVHlwZXMuanNcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgeyBUTFBUX1JFQ0VJVkVfVVNFUiB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xudmFyIHsgVFJZSU5HX1RPX1NJR05fVVAsIFRSWUlOR19UT19MT0dJTn0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9yZXN0QXBpL2NvbnN0YW50cycpO1xudmFyIHJlc3RBcGlBY3Rpb25zID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvcmVzdEFwaS9hY3Rpb25zJyk7XG52YXIgYXV0aCA9IHJlcXVpcmUoJ2FwcC9hdXRoJyk7XG52YXIgc2Vzc2lvbiA9IHJlcXVpcmUoJ2FwcC9zZXNzaW9uJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xuXG5leHBvcnQgZGVmYXVsdCB7XG5cbiAgZW5zdXJlVXNlcihuZXh0U3RhdGUsIHJlcGxhY2UsIGNiKXtcbiAgICBhdXRoLmVuc3VyZVVzZXIoKVxuICAgICAgLmRvbmUoKHVzZXJEYXRhKT0+IHtcbiAgICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFQ0VJVkVfVVNFUiwgdXNlckRhdGEudXNlciApO1xuICAgICAgICBjYigpO1xuICAgICAgfSlcbiAgICAgIC5mYWlsKCgpPT57XG4gICAgICAgIHJlcGxhY2Uoe3JlZGlyZWN0VG86IG5leHRTdGF0ZS5sb2NhdGlvbi5wYXRobmFtZSB9LCBjZmcucm91dGVzLmxvZ2luKTtcbiAgICAgICAgY2IoKTtcbiAgICAgIH0pO1xuICB9LFxuXG4gIHNpZ25VcCh7bmFtZSwgcHN3LCB0b2tlbiwgaW52aXRlVG9rZW59KXtcbiAgICByZXN0QXBpQWN0aW9ucy5zdGFydChUUllJTkdfVE9fU0lHTl9VUCk7XG4gICAgYXV0aC5zaWduVXAobmFtZSwgcHN3LCB0b2tlbiwgaW52aXRlVG9rZW4pXG4gICAgICAuZG9uZSgoc2Vzc2lvbkRhdGEpPT57XG4gICAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRUNFSVZFX1VTRVIsIHNlc3Npb25EYXRhLnVzZXIpO1xuICAgICAgICByZXN0QXBpQWN0aW9ucy5zdWNjZXNzKFRSWUlOR19UT19TSUdOX1VQKTtcbiAgICAgICAgc2Vzc2lvbi5nZXRIaXN0b3J5KCkucHVzaCh7cGF0aG5hbWU6IGNmZy5yb3V0ZXMuYXBwfSk7XG4gICAgICB9KVxuICAgICAgLmZhaWwoKGVycik9PntcbiAgICAgICAgcmVzdEFwaUFjdGlvbnMuZmFpbChUUllJTkdfVE9fU0lHTl9VUCwgZXJyLnJlc3BvbnNlSlNPTi5tZXNzYWdlIHx8ICdmYWlsZWQgdG8gc2luZyB1cCcpO1xuICAgICAgfSk7XG4gIH0sXG5cbiAgbG9naW4oe3VzZXIsIHBhc3N3b3JkLCB0b2tlbn0sIHJlZGlyZWN0KXtcbiAgICByZXN0QXBpQWN0aW9ucy5zdGFydChUUllJTkdfVE9fTE9HSU4pO1xuICAgIGF1dGgubG9naW4odXNlciwgcGFzc3dvcmQsIHRva2VuKVxuICAgICAgLmRvbmUoKHNlc3Npb25EYXRhKT0+e1xuICAgICAgICByZXN0QXBpQWN0aW9ucy5zdWNjZXNzKFRSWUlOR19UT19MT0dJTik7XG4gICAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRUNFSVZFX1VTRVIsIHNlc3Npb25EYXRhLnVzZXIpO1xuICAgICAgICBzZXNzaW9uLmdldEhpc3RvcnkoKS5wdXNoKHtwYXRobmFtZTogcmVkaXJlY3R9KTtcbiAgICAgIH0pXG4gICAgICAuZmFpbCgoZXJyKT0+IHJlc3RBcGlBY3Rpb25zLmZhaWwoVFJZSU5HX1RPX0xPR0lOLCBlcnIucmVzcG9uc2VKU09OLm1lc3NhZ2UpKVxuICAgIH1cbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvYWN0aW9ucy5qc1xuICoqLyIsIm1vZHVsZS5leHBvcnRzLmdldHRlcnMgPSByZXF1aXJlKCcuL2dldHRlcnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGlvbnMgPSByZXF1aXJlKCcuL2FjdGlvbnMnKTtcbm1vZHVsZS5leHBvcnRzLm5vZGVTdG9yZSA9IHJlcXVpcmUoJy4vdXNlclN0b3JlJyk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy91c2VyL2luZGV4LmpzXG4gKiovIiwidmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG52YXIgIHsgVExQVF9SRUNFSVZFX1VTRVIgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKG51bGwpO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX1JFQ0VJVkVfVVNFUiwgcmVjZWl2ZVVzZXIpXG4gIH1cblxufSlcblxuZnVuY3Rpb24gcmVjZWl2ZVVzZXIoc3RhdGUsIHVzZXIpe1xuICByZXR1cm4gdG9JbW11dGFibGUodXNlcik7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy91c2VyL3VzZXJTdG9yZS5qc1xuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIge2FjdGlvbnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvJyk7XG52YXIgY29sb3JzID0gWycjMWFiMzk0JywgJyMxYzg0YzYnLCAnIzIzYzZjOCcsICcjZjhhYzU5JywgJyNFRDU1NjUnLCAnI2MyYzJjMiddO1xuXG5jb25zdCBVc2VySWNvbiA9ICh7bmFtZSwgdGl0bGUsIGNvbG9ySW5kZXg9MH0pPT57XG4gIGxldCBjb2xvciA9IGNvbG9yc1tjb2xvckluZGV4ICUgY29sb3JzLmxlbmd0aF07XG4gIGxldCBzdHlsZSA9IHtcbiAgICAnYmFja2dyb3VuZENvbG9yJzogY29sb3IsXG4gICAgJ2JvcmRlckNvbG9yJzogY29sb3JcbiAgfTtcblxuICByZXR1cm4gKFxuICAgIDxsaT5cbiAgICAgIDxzcGFuIHN0eWxlPXtzdHlsZX0gY2xhc3NOYW1lPVwiYnRuIGJ0bi1wcmltYXJ5IGJ0bi1jaXJjbGUgdGV4dC11cHBlcmNhc2VcIj5cbiAgICAgICAgPHN0cm9uZz57bmFtZVswXX08L3N0cm9uZz5cbiAgICAgIDwvc3Bhbj5cbiAgICA8L2xpPlxuICApXG59O1xuXG5jb25zdCBTZXNzaW9uTGVmdFBhbmVsID0gKHtwYXJ0aWVzfSkgPT4ge1xuICBwYXJ0aWVzID0gcGFydGllcyB8fCBbXTtcbiAgbGV0IHVzZXJJY29ucyA9IHBhcnRpZXMubWFwKChpdGVtLCBpbmRleCk9PihcbiAgICA8VXNlckljb24ga2V5PXtpbmRleH0gY29sb3JJbmRleD17aW5kZXh9IG5hbWU9e2l0ZW0udXNlcn0vPlxuICApKTtcblxuICByZXR1cm4gKFxuICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXRlcm1pbmFsLXBhcnRpY2lwYW5zXCI+XG4gICAgICA8dWwgY2xhc3NOYW1lPVwibmF2XCI+XG4gICAgICAgIHt1c2VySWNvbnN9XG4gICAgICAgIDxsaT5cbiAgICAgICAgICA8YnV0dG9uIG9uQ2xpY2s9e2FjdGlvbnMuY2xvc2V9IGNsYXNzTmFtZT1cImJ0biBidG4tZGFuZ2VyIGJ0bi1jaXJjbGVcIiB0eXBlPVwiYnV0dG9uXCI+XG4gICAgICAgICAgICA8aSBjbGFzc05hbWU9XCJmYSBmYS10aW1lc1wiPjwvaT5cbiAgICAgICAgICA8L2J1dHRvbj5cbiAgICAgICAgPC9saT5cbiAgICAgIDwvdWw+XG4gICAgPC9kaXY+XG4gIClcbn07XG5cbm1vZHVsZS5leHBvcnRzID0gU2Vzc2lvbkxlZnRQYW5lbDtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2N1cnJlbnRTZXNzaW9uL3Nlc3Npb25MZWZ0UGFuZWwuanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcblxudmFyIE5vdEZvdW5kID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWVycm9yLXBhZ2VcIj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbG9nby10cHJ0XCI+VGVsZXBvcnQ8L2Rpdj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtd2FybmluZ1wiPjxpIGNsYXNzTmFtZT1cImZhIGZhLXdhcm5pbmdcIj48L2k+IDwvZGl2PlxuICAgICAgICA8aDE+V2hvb3BzLCB3ZSBjYW5ub3QgZmluZCB0aGF0PC9oMT5cbiAgICAgICAgPGRpdj5Mb29rcyBsaWtlIHRoZSBwYWdlIHlvdSBhcmUgbG9va2luZyBmb3IgaXNuJ3QgaGVyZSBhbnkgbG9uZ2VyPC9kaXY+XG4gICAgICAgIDxkaXY+SWYgeW91IGJlbGlldmUgdGhpcyBpcyBhbiBlcnJvciwgcGxlYXNlIGNvbnRhY3QgeW91ciBvcmdhbml6YXRpb24gYWRtaW5pc3RyYXRvci48L2Rpdj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJjb250YWN0LXNlY3Rpb25cIj5JZiB5b3UgYmVsaWV2ZSB0aGlzIGlzIGFuIGlzc3VlIHdpdGggVGVsZXBvcnQsIHBsZWFzZSA8YSBocmVmPVwiaHR0cHM6Ly9naXRodWIuY29tL2dyYXZpdGF0aW9uYWwvdGVsZXBvcnQvaXNzdWVzL25ld1wiPmNyZWF0ZSBhIEdpdEh1YiBpc3N1ZS48L2E+XG4gICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pXG5cbnZhciBFeHBpcmVkSW52aXRlID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWVycm9yLXBhZ2VcIj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbG9nby10cHJ0XCI+VGVsZXBvcnQ8L2Rpdj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtd2FybmluZ1wiPjxpIGNsYXNzTmFtZT1cImZhIGZhLXdhcm5pbmdcIj48L2k+IDwvZGl2PlxuICAgICAgICA8aDE+SW52aXRlIGNvZGUgaGFzIGV4cGlyZWQ8L2gxPlxuICAgICAgICA8ZGl2Pkxvb2tzIGxpa2UgeW91ciBpbnZpdGUgY29kZSBpc24ndCB2YWxpZCBhbnltb3JlPC9kaXY+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiY29udGFjdC1zZWN0aW9uXCI+SWYgeW91IGJlbGlldmUgdGhpcyBpcyBhbiBpc3N1ZSB3aXRoIFRlbGVwb3J0LCBwbGVhc2UgPGEgaHJlZj1cImh0dHBzOi8vZ2l0aHViLmNvbS9ncmF2aXRhdGlvbmFsL3RlbGVwb3J0L2lzc3Vlcy9uZXdcIj5jcmVhdGUgYSBHaXRIdWIgaXNzdWUuPC9hPlxuICAgICAgICAgPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KVxuXG5leHBvcnQgZGVmYXVsdCBOb3RGb3VuZDtcbmV4cG9ydCB7Tm90Rm91bmQsIEV4cGlyZWRJbnZpdGV9XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9lcnJvclBhZ2UuanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcblxudmFyIEdvb2dsZUF1dGhJbmZvID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWdvb2dsZS1hdXRoXCI+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWdvb2dsZS1hdXRoLWljb25cIj48L2Rpdj5cbiAgICAgICAgPHN0cm9uZz5Hb29nbGUgQXV0aGVudGljYXRvcjwvc3Ryb25nPlxuICAgICAgICA8ZGl2PkRvd25sb2FkIDxhIGhyZWY9XCJodHRwczovL3N1cHBvcnQuZ29vZ2xlLmNvbS9hY2NvdW50cy9hbnN3ZXIvMTA2NjQ0Nz9obD1lblwiPkdvb2dsZSBBdXRoZW50aWNhdG9yPC9hPiBvbiB5b3VyIHBob25lIHRvIGFjY2VzcyB5b3VyIHR3byBmYWN0b3J5IHRva2VuPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KVxuXG5tb2R1bGUuZXhwb3J0cyA9IEdvb2dsZUF1dGhJbmZvO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvZ29vZ2xlQXV0aExvZ28uanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7Z2V0dGVycywgYWN0aW9uc30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9ub2RlcycpO1xudmFyIHVzZXJHZXR0ZXJzID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvdXNlci9nZXR0ZXJzJyk7XG52YXIge1RhYmxlLCBDb2x1bW4sIENlbGwsIFNvcnRIZWFkZXJDZWxsLCBTb3J0VHlwZXN9ID0gcmVxdWlyZSgnYXBwL2NvbXBvbmVudHMvdGFibGUuanN4Jyk7XG52YXIge2NyZWF0ZU5ld1Nlc3Npb259ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvYWN0aW9ucycpO1xudmFyIExpbmtlZFN0YXRlTWl4aW4gPSByZXF1aXJlKCdyZWFjdC1hZGRvbnMtbGlua2VkLXN0YXRlLW1peGluJyk7XG52YXIgXyA9IHJlcXVpcmUoJ18nKTtcbnZhciB7aXNNYXRjaH0gPSByZXF1aXJlKCdhcHAvY29tbW9uL29iamVjdFV0aWxzJyk7XG5cbmNvbnN0IFRleHRDZWxsID0gKHtyb3dJbmRleCwgZGF0YSwgY29sdW1uS2V5LCAuLi5wcm9wc30pID0+IChcbiAgPENlbGwgey4uLnByb3BzfT5cbiAgICB7ZGF0YVtyb3dJbmRleF1bY29sdW1uS2V5XX1cbiAgPC9DZWxsPlxuKTtcblxuY29uc3QgVGFnQ2VsbCA9ICh7cm93SW5kZXgsIGRhdGEsIGNvbHVtbktleSwgLi4ucHJvcHN9KSA9PiAoXG4gIDxDZWxsIHsuLi5wcm9wc30+XG4gICAgeyBkYXRhW3Jvd0luZGV4XS50YWdzLm1hcCgoaXRlbSwgaW5kZXgpID0+XG4gICAgICAoPHNwYW4ga2V5PXtpbmRleH0gY2xhc3NOYW1lPVwibGFiZWwgbGFiZWwtZGVmYXVsdFwiPlxuICAgICAgICB7aXRlbS5yb2xlfSA8bGkgY2xhc3NOYW1lPVwiZmEgZmEtbG9uZy1hcnJvdy1yaWdodFwiPjwvbGk+XG4gICAgICAgIHtpdGVtLnZhbHVlfVxuICAgICAgPC9zcGFuPilcbiAgICApIH1cbiAgPC9DZWxsPlxuKTtcblxuY29uc3QgTG9naW5DZWxsID0gKHtsb2dpbnMsIG9uTG9naW5DbGljaywgcm93SW5kZXgsIGRhdGEsIC4uLnByb3BzfSkgPT4ge1xuICBpZighbG9naW5zIHx8bG9naW5zLmxlbmd0aCA9PT0gMCl7XG4gICAgcmV0dXJuIDxDZWxsIHsuLi5wcm9wc30gLz47XG4gIH1cblxuICB2YXIgc2VydmVySWQgPSBkYXRhW3Jvd0luZGV4XS5pZDtcbiAgdmFyICRsaXMgPSBbXTtcblxuICBmdW5jdGlvbiBvbkNsaWNrKGkpe1xuICAgIHZhciBsb2dpbiA9IGxvZ2luc1tpXTtcbiAgICBpZihvbkxvZ2luQ2xpY2spe1xuICAgICAgcmV0dXJuICgpPT4gb25Mb2dpbkNsaWNrKHNlcnZlcklkLCBsb2dpbik7XG4gICAgfWVsc2V7XG4gICAgICByZXR1cm4gKCkgPT4gY3JlYXRlTmV3U2Vzc2lvbihzZXJ2ZXJJZCwgbG9naW4pO1xuICAgIH1cbiAgfVxuXG4gIGZvcih2YXIgaSA9IDA7IGkgPCBsb2dpbnMubGVuZ3RoOyBpKyspe1xuICAgICRsaXMucHVzaCg8bGkga2V5PXtpfT48YSBvbkNsaWNrPXtvbkNsaWNrKGkpfT57bG9naW5zW2ldfTwvYT48L2xpPik7XG4gIH1cblxuICByZXR1cm4gKFxuICAgIDxDZWxsIHsuLi5wcm9wc30+XG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImJ0bi1ncm91cFwiPlxuICAgICAgICA8YnV0dG9uIHR5cGU9XCJidXR0b25cIiBvbkNsaWNrPXtvbkNsaWNrKDApfSBjbGFzc05hbWU9XCJidG4gYnRuLXhzIGJ0bi1wcmltYXJ5XCI+e2xvZ2luc1swXX08L2J1dHRvbj5cbiAgICAgICAge1xuICAgICAgICAgICRsaXMubGVuZ3RoID4gMSA/IChcbiAgICAgICAgICAgICAgW1xuICAgICAgICAgICAgICAgIDxidXR0b24ga2V5PXswfSBkYXRhLXRvZ2dsZT1cImRyb3Bkb3duXCIgY2xhc3NOYW1lPVwiYnRuIGJ0bi1kZWZhdWx0IGJ0bi14cyBkcm9wZG93bi10b2dnbGVcIiBhcmlhLWV4cGFuZGVkPVwidHJ1ZVwiPlxuICAgICAgICAgICAgICAgICAgPHNwYW4gY2xhc3NOYW1lPVwiY2FyZXRcIj48L3NwYW4+XG4gICAgICAgICAgICAgICAgPC9idXR0b24+LFxuICAgICAgICAgICAgICAgIDx1bCBrZXk9ezF9IGNsYXNzTmFtZT1cImRyb3Bkb3duLW1lbnVcIj5cbiAgICAgICAgICAgICAgICAgIHskbGlzfVxuICAgICAgICAgICAgICAgIDwvdWw+XG4gICAgICAgICAgICAgIF0gKVxuICAgICAgICAgICAgOiBudWxsXG4gICAgICAgIH1cbiAgICAgIDwvZGl2PlxuICAgIDwvQ2VsbD5cbiAgKVxufTtcblxudmFyIE5vZGVMaXN0ID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW0xpbmtlZFN0YXRlTWl4aW5dLFxuXG4gIGdldEluaXRpYWxTdGF0ZShwcm9wcyl7XG4gICAgdGhpcy5zZWFyY2hhYmxlUHJvcHMgPSBbJ3Nlc3Npb25Db3VudCcsICdhZGRyJ107XG4gICAgcmV0dXJuIHsgZmlsdGVyOiAnJywgY29sU29ydERpcnM6IHt9IH07XG4gIH0sXG5cbiAgb25Tb3J0Q2hhbmdlKGNvbHVtbktleSwgc29ydERpcikge1xuICAgIHRoaXMuc2V0U3RhdGUoe1xuICAgICAgLi4udGhpcy5zdGF0ZSxcbiAgICAgIGNvbFNvcnREaXJzOiB7XG4gICAgICAgIFtjb2x1bW5LZXldOiBzb3J0RGlyXG4gICAgICB9XG4gICAgfSk7XG4gIH0sXG5cbiAgc29ydEFuZEZpbHRlcihkYXRhKXtcbiAgICB2YXIgZmlsdGVyZWQgPSBkYXRhLmZpbHRlcihvYmo9PlxuICAgICAgaXNNYXRjaChvYmosIHRoaXMuc3RhdGUuZmlsdGVyLCB7IHNlYXJjaGFibGVQcm9wczogdGhpcy5zZWFyY2hhYmxlUHJvcHN9KSk7XG5cbiAgICB2YXIgY29sdW1uS2V5ID0gT2JqZWN0LmdldE93blByb3BlcnR5TmFtZXModGhpcy5zdGF0ZS5jb2xTb3J0RGlycylbMF07XG4gICAgdmFyIHNvcnREaXIgPSB0aGlzLnN0YXRlLmNvbFNvcnREaXJzW2NvbHVtbktleV07XG4gICAgdmFyIHNvcnRlZCA9IF8uc29ydEJ5KGZpbHRlcmVkLCBjb2x1bW5LZXkpO1xuICAgIGlmKHNvcnREaXIgPT09IFNvcnRUeXBlcy5BU0Mpe1xuICAgICAgc29ydGVkID0gc29ydGVkLnJldmVyc2UoKTtcbiAgICB9XG5cbiAgICByZXR1cm4gc29ydGVkO1xuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgdmFyIGRhdGEgPSB0aGlzLnNvcnRBbmRGaWx0ZXIodGhpcy5wcm9wcy5ub2RlUmVjb3Jkcyk7XG4gICAgdmFyIGxvZ2lucyA9IHRoaXMucHJvcHMubG9naW5zO1xuICAgIHZhciBvbkxvZ2luQ2xpY2sgPSB0aGlzLnByb3BzLm9uTG9naW5DbGljaztcblxuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1ub2Rlc1wiPlxuICAgICAgICA8aDE+IE5vZGVzIDwvaDE+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXNlYXJjaFwiPlxuICAgICAgICAgIDxpbnB1dCB2YWx1ZUxpbms9e3RoaXMubGlua1N0YXRlKCdmaWx0ZXInKX0gcGxhY2Vob2xkZXI9XCJTZWFyY2guLi5cIiBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2wgaW5wdXQtc21cIi8+XG4gICAgICAgIDwvZGl2PlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgIDxUYWJsZSByb3dDb3VudD17ZGF0YS5sZW5ndGh9IGNsYXNzTmFtZT1cInRhYmxlLXN0cmlwZWQgZ3J2LW5vZGVzLXRhYmxlXCI+XG4gICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgIGNvbHVtbktleT1cInNlc3Npb25Db3VudFwiXG4gICAgICAgICAgICAgIGhlYWRlcj17XG4gICAgICAgICAgICAgICAgPFNvcnRIZWFkZXJDZWxsXG4gICAgICAgICAgICAgICAgICBzb3J0RGlyPXt0aGlzLnN0YXRlLmNvbFNvcnREaXJzLnNlc3Npb25Db3VudH1cbiAgICAgICAgICAgICAgICAgIG9uU29ydENoYW5nZT17dGhpcy5vblNvcnRDaGFuZ2V9XG4gICAgICAgICAgICAgICAgICB0aXRsZT1cIlNlc3Npb25zXCJcbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICB9XG4gICAgICAgICAgICAgIGNlbGw9ezxUZXh0Q2VsbCBkYXRhPXtkYXRhfS8+IH1cbiAgICAgICAgICAgIC8+XG4gICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgIGNvbHVtbktleT1cImFkZHJcIlxuICAgICAgICAgICAgICBoZWFkZXI9e1xuICAgICAgICAgICAgICAgIDxTb3J0SGVhZGVyQ2VsbFxuICAgICAgICAgICAgICAgICAgc29ydERpcj17dGhpcy5zdGF0ZS5jb2xTb3J0RGlycy5hZGRyfVxuICAgICAgICAgICAgICAgICAgb25Tb3J0Q2hhbmdlPXt0aGlzLm9uU29ydENoYW5nZX1cbiAgICAgICAgICAgICAgICAgIHRpdGxlPVwiTm9kZVwiXG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgfVxuXG4gICAgICAgICAgICAgIGNlbGw9ezxUZXh0Q2VsbCBkYXRhPXtkYXRhfS8+IH1cbiAgICAgICAgICAgIC8+XG4gICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgIGNvbHVtbktleT1cInRhZ3NcIlxuICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPjwvQ2VsbD4gfVxuICAgICAgICAgICAgICBjZWxsPXs8VGFnQ2VsbCBkYXRhPXtkYXRhfS8+IH1cbiAgICAgICAgICAgIC8+XG4gICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgIGNvbHVtbktleT1cInJvbGVzXCJcbiAgICAgICAgICAgICAgb25Mb2dpbkNsaWNrPXtvbkxvZ2luQ2xpY2t9XG4gICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+TG9naW4gYXM8L0NlbGw+IH1cbiAgICAgICAgICAgICAgY2VsbD17PExvZ2luQ2VsbCBkYXRhPXtkYXRhfSBsb2dpbnM9e2xvZ2luc30vPiB9XG4gICAgICAgICAgICAvPlxuICAgICAgICAgIDwvVGFibGU+XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9kaXY+XG4gICAgKVxuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBOb2RlTGlzdDtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL25vZGVzL25vZGVMaXN0LmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIge2dldHRlcnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvZGlhbG9ncycpO1xudmFyIHtjbG9zZVNlbGVjdE5vZGVEaWFsb2d9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvZGlhbG9ncy9hY3Rpb25zJyk7XG52YXIge2NoYW5nZVNlcnZlcn0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3Rpb25zJyk7XG52YXIgTm9kZUxpc3QgPSByZXF1aXJlKCcuL25vZGVzL25vZGVMaXN0LmpzeCcpO1xudmFyIGFjdGl2ZVNlc3Npb25HZXR0ZXJzID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvZ2V0dGVycycpO1xudmFyIG5vZGVHZXR0ZXJzID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvbm9kZXMvZ2V0dGVycycpO1xuXG52YXIgU2VsZWN0Tm9kZURpYWxvZyA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBtaXhpbnM6IFtyZWFjdG9yLlJlYWN0TWl4aW5dLFxuXG4gIGdldERhdGFCaW5kaW5ncygpIHtcbiAgICByZXR1cm4ge1xuICAgICAgZGlhbG9nczogZ2V0dGVycy5kaWFsb2dzXG4gICAgfVxuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICByZXR1cm4gdGhpcy5zdGF0ZS5kaWFsb2dzLmlzU2VsZWN0Tm9kZURpYWxvZ09wZW4gPyA8RGlhbG9nLz4gOiBudWxsO1xuICB9XG59KTtcblxudmFyIERpYWxvZyA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBvbkxvZ2luQ2xpY2soc2VydmVySWQsIGxvZ2luKXtcbiAgICBpZihTZWxlY3ROb2RlRGlhbG9nLm9uU2VydmVyQ2hhbmdlQ2FsbEJhY2spe1xuICAgICAgU2VsZWN0Tm9kZURpYWxvZy5vblNlcnZlckNoYW5nZUNhbGxCYWNrKHtzZXJ2ZXJJZH0pO1xuICAgIH1cblxuICAgIGNsb3NlU2VsZWN0Tm9kZURpYWxvZygpO1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50KGNhbGxiYWNrKXtcbiAgICAkKCcubW9kYWwnKS5tb2RhbCgnaGlkZScpO1xuICB9LFxuXG4gIGNvbXBvbmVudERpZE1vdW50KCl7XG4gICAgJCgnLm1vZGFsJykubW9kYWwoJ3Nob3cnKTtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgdmFyIGFjdGl2ZVNlc3Npb24gPSByZWFjdG9yLmV2YWx1YXRlKGFjdGl2ZVNlc3Npb25HZXR0ZXJzLmFjdGl2ZVNlc3Npb24pIHx8IHt9O1xuICAgIHZhciBub2RlUmVjb3JkcyA9IHJlYWN0b3IuZXZhbHVhdGUobm9kZUdldHRlcnMubm9kZUxpc3RWaWV3KTtcbiAgICB2YXIgbG9naW5zID0gW2FjdGl2ZVNlc3Npb24ubG9naW5dO1xuXG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwibW9kYWwgZmFkZSBncnYtZGlhbG9nLXNlbGVjdC1ub2RlXCIgdGFiSW5kZXg9ey0xfSByb2xlPVwiZGlhbG9nXCI+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwibW9kYWwtZGlhbG9nXCI+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJtb2RhbC1jb250ZW50XCI+XG4gICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIm1vZGFsLWhlYWRlclwiPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIm1vZGFsLWJvZHlcIj5cbiAgICAgICAgICAgICAgPE5vZGVMaXN0IG5vZGVSZWNvcmRzPXtub2RlUmVjb3Jkc30gbG9naW5zPXtsb2dpbnN9IG9uTG9naW5DbGljaz17dGhpcy5vbkxvZ2luQ2xpY2t9Lz5cbiAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJtb2RhbC1mb290ZXJcIj5cbiAgICAgICAgICAgICAgPGJ1dHRvbiBvbkNsaWNrPXtjbG9zZVNlbGVjdE5vZGVEaWFsb2d9IHR5cGU9XCJidXR0b25cIiBjbGFzc05hbWU9XCJidG4gYnRuLXByaW1hcnlcIj5cbiAgICAgICAgICAgICAgICBDbG9zZVxuICAgICAgICAgICAgICA8L2J1dHRvbj5cbiAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pO1xuXG5TZWxlY3ROb2RlRGlhbG9nLm9uU2VydmVyQ2hhbmdlQ2FsbEJhY2sgPSAoKT0+e307XG5cbm1vZHVsZS5leHBvcnRzID0gU2VsZWN0Tm9kZURpYWxvZztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL3NlbGVjdE5vZGVEaWFsb2cuanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcblxuY29uc3QgR3J2VGFibGVUZXh0Q2VsbCA9ICh7cm93SW5kZXgsIGRhdGEsIGNvbHVtbktleSwgLi4ucHJvcHN9KSA9PiAoXG4gIDxHcnZUYWJsZUNlbGwgey4uLnByb3BzfT5cbiAgICB7ZGF0YVtyb3dJbmRleF1bY29sdW1uS2V5XX1cbiAgPC9HcnZUYWJsZUNlbGw+XG4pO1xuXG52YXIgR3J2U29ydEhlYWRlckNlbGwgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICB0aGlzLl9vblNvcnRDaGFuZ2UgPSB0aGlzLl9vblNvcnRDaGFuZ2UuYmluZCh0aGlzKTtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgdmFyIHtzb3J0RGlyLCBjaGlsZHJlbiwgLi4ucHJvcHN9ID0gdGhpcy5wcm9wcztcbiAgICByZXR1cm4gKFxuICAgICAgPENlbGwgey4uLnByb3BzfT5cbiAgICAgICAgPGEgb25DbGljaz17dGhpcy5fb25Tb3J0Q2hhbmdlfT5cbiAgICAgICAgICB7Y2hpbGRyZW59IHtzb3J0RGlyID8gKHNvcnREaXIgPT09IFNvcnRUeXBlcy5ERVNDID8gJ+KGkycgOiAn4oaRJykgOiAnJ31cbiAgICAgICAgPC9hPlxuICAgICAgPC9DZWxsPlxuICAgICk7XG4gIH0sXG5cbiAgX29uU29ydENoYW5nZShlKSB7XG4gICAgZS5wcmV2ZW50RGVmYXVsdCgpO1xuXG4gICAgaWYgKHRoaXMucHJvcHMub25Tb3J0Q2hhbmdlKSB7XG4gICAgICB0aGlzLnByb3BzLm9uU29ydENoYW5nZShcbiAgICAgICAgdGhpcy5wcm9wcy5jb2x1bW5LZXksXG4gICAgICAgIHRoaXMucHJvcHMuc29ydERpciA/XG4gICAgICAgICAgcmV2ZXJzZVNvcnREaXJlY3Rpb24odGhpcy5wcm9wcy5zb3J0RGlyKSA6XG4gICAgICAgICAgU29ydFR5cGVzLkRFU0NcbiAgICAgICk7XG4gICAgfVxuICB9XG59KTtcblxuLyoqXG4qIFNvcnQgaW5kaWNhdG9yIHVzZWQgYnkgU29ydEhlYWRlckNlbGxcbiovXG5jb25zdCBTb3J0VHlwZXMgPSB7XG4gIEFTQzogJ0FTQycsXG4gIERFU0M6ICdERVNDJ1xufTtcblxuY29uc3QgU29ydEluZGljYXRvciA9ICh7c29ydERpcn0pPT57XG4gIGxldCBjbHMgPSAnZ3J2LXRhYmxlLWluZGljYXRvci1zb3J0IGZhIGZhLXNvcnQnXG4gIGlmKHNvcnREaXIgPT09IFNvcnRUeXBlcy5ERVNDKXtcbiAgICBjbHMgKz0gJy1kZXNjJ1xuICB9XG5cbiAgaWYoIHNvcnREaXIgPT09IFNvcnRUeXBlcy5BU0Mpe1xuICAgIGNscyArPSAnLWFzYydcbiAgfVxuXG4gIHJldHVybiAoPGkgY2xhc3NOYW1lPXtjbHN9PjwvaT4pO1xufTtcblxuLyoqXG4qIFNvcnQgSGVhZGVyIENlbGxcbiovXG52YXIgU29ydEhlYWRlckNlbGwgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIHJlbmRlcigpIHtcbiAgICB2YXIge3NvcnREaXIsIGNvbHVtbktleSwgdGl0bGUsIC4uLnByb3BzfSA9IHRoaXMucHJvcHM7XG5cbiAgICByZXR1cm4gKFxuICAgICAgPEdydlRhYmxlQ2VsbCB7Li4ucHJvcHN9PlxuICAgICAgICA8YSBvbkNsaWNrPXt0aGlzLm9uU29ydENoYW5nZX0+XG4gICAgICAgICAge3RpdGxlfVxuICAgICAgICA8L2E+XG4gICAgICAgIDxTb3J0SW5kaWNhdG9yIHNvcnREaXI9e3NvcnREaXJ9Lz5cbiAgICAgIDwvR3J2VGFibGVDZWxsPlxuICAgICk7XG4gIH0sXG5cbiAgb25Tb3J0Q2hhbmdlKGUpIHtcbiAgICBlLnByZXZlbnREZWZhdWx0KCk7XG4gICAgaWYodGhpcy5wcm9wcy5vblNvcnRDaGFuZ2UpIHtcbiAgICAgIC8vIGRlZmF1bHRcbiAgICAgIGxldCBuZXdEaXIgPSBTb3J0VHlwZXMuREVTQztcbiAgICAgIGlmKHRoaXMucHJvcHMuc29ydERpcil7XG4gICAgICAgIG5ld0RpciA9IHRoaXMucHJvcHMuc29ydERpciA9PT0gU29ydFR5cGVzLkRFU0MgPyBTb3J0VHlwZXMuQVNDIDogU29ydFR5cGVzLkRFU0M7XG4gICAgICB9XG4gICAgICB0aGlzLnByb3BzLm9uU29ydENoYW5nZSh0aGlzLnByb3BzLmNvbHVtbktleSwgbmV3RGlyKTtcbiAgICB9XG4gIH1cbn0pO1xuXG4vKipcbiogRGVmYXVsdCBDZWxsXG4qL1xudmFyIEdydlRhYmxlQ2VsbCA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgcmVuZGVyKCl7XG4gICAgdmFyIHByb3BzID0gdGhpcy5wcm9wcztcbiAgICByZXR1cm4gcHJvcHMuaXNIZWFkZXIgPyA8dGgga2V5PXtwcm9wcy5rZXl9IGNsYXNzTmFtZT1cImdydi10YWJsZS1jZWxsXCI+e3Byb3BzLmNoaWxkcmVufTwvdGg+IDogPHRkIGtleT17cHJvcHMua2V5fT57cHJvcHMuY2hpbGRyZW59PC90ZD47XG4gIH1cbn0pO1xuXG4vKipcbiogVGFibGVcbiovXG52YXIgR3J2VGFibGUgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgcmVuZGVySGVhZGVyKGNoaWxkcmVuKXtcbiAgICB2YXIgY2VsbHMgPSBjaGlsZHJlbi5tYXAoKGl0ZW0sIGluZGV4KT0+e1xuICAgICAgcmV0dXJuIHRoaXMucmVuZGVyQ2VsbChpdGVtLnByb3BzLmhlYWRlciwge2luZGV4LCBrZXk6IGluZGV4LCBpc0hlYWRlcjogdHJ1ZSwgLi4uaXRlbS5wcm9wc30pO1xuICAgIH0pXG5cbiAgICByZXR1cm4gPHRoZWFkIGNsYXNzTmFtZT1cImdydi10YWJsZS1oZWFkZXJcIj48dHI+e2NlbGxzfTwvdHI+PC90aGVhZD5cbiAgfSxcblxuICByZW5kZXJCb2R5KGNoaWxkcmVuKXtcbiAgICB2YXIgY291bnQgPSB0aGlzLnByb3BzLnJvd0NvdW50O1xuICAgIHZhciByb3dzID0gW107XG4gICAgZm9yKHZhciBpID0gMDsgaSA8IGNvdW50OyBpICsrKXtcbiAgICAgIHZhciBjZWxscyA9IGNoaWxkcmVuLm1hcCgoaXRlbSwgaW5kZXgpPT57XG4gICAgICAgIHJldHVybiB0aGlzLnJlbmRlckNlbGwoaXRlbS5wcm9wcy5jZWxsLCB7cm93SW5kZXg6IGksIGtleTogaW5kZXgsIGlzSGVhZGVyOiBmYWxzZSwgLi4uaXRlbS5wcm9wc30pO1xuICAgICAgfSlcblxuICAgICAgcm93cy5wdXNoKDx0ciBrZXk9e2l9PntjZWxsc308L3RyPik7XG4gICAgfVxuXG4gICAgcmV0dXJuIDx0Ym9keT57cm93c308L3Rib2R5PjtcbiAgfSxcblxuICByZW5kZXJDZWxsKGNlbGwsIGNlbGxQcm9wcyl7XG4gICAgdmFyIGNvbnRlbnQgPSBudWxsO1xuICAgIGlmIChSZWFjdC5pc1ZhbGlkRWxlbWVudChjZWxsKSkge1xuICAgICAgIGNvbnRlbnQgPSBSZWFjdC5jbG9uZUVsZW1lbnQoY2VsbCwgY2VsbFByb3BzKTtcbiAgICAgfSBlbHNlIGlmICh0eXBlb2YgcHJvcHMuY2VsbCA9PT0gJ2Z1bmN0aW9uJykge1xuICAgICAgIGNvbnRlbnQgPSBjZWxsKGNlbGxQcm9wcyk7XG4gICAgIH1cblxuICAgICByZXR1cm4gY29udGVudDtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgdmFyIGNoaWxkcmVuID0gW107XG4gICAgUmVhY3QuQ2hpbGRyZW4uZm9yRWFjaCh0aGlzLnByb3BzLmNoaWxkcmVuLCAoY2hpbGQsIGluZGV4KSA9PiB7XG4gICAgICBpZiAoY2hpbGQgPT0gbnVsbCkge1xuICAgICAgICByZXR1cm47XG4gICAgICB9XG5cbiAgICAgIGlmKGNoaWxkLnR5cGUuZGlzcGxheU5hbWUgIT09ICdHcnZUYWJsZUNvbHVtbicpe1xuICAgICAgICB0aHJvdyAnU2hvdWxkIGJlIEdydlRhYmxlQ29sdW1uJztcbiAgICAgIH1cblxuICAgICAgY2hpbGRyZW4ucHVzaChjaGlsZCk7XG4gICAgfSk7XG5cbiAgICB2YXIgdGFibGVDbGFzcyA9ICd0YWJsZSAnICsgdGhpcy5wcm9wcy5jbGFzc05hbWU7XG5cbiAgICByZXR1cm4gKFxuICAgICAgPHRhYmxlIGNsYXNzTmFtZT17dGFibGVDbGFzc30+XG4gICAgICAgIHt0aGlzLnJlbmRlckhlYWRlcihjaGlsZHJlbil9XG4gICAgICAgIHt0aGlzLnJlbmRlckJvZHkoY2hpbGRyZW4pfVxuICAgICAgPC90YWJsZT5cbiAgICApO1xuICB9XG59KVxuXG52YXIgR3J2VGFibGVDb2x1bW4gPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgdGhyb3cgbmV3IEVycm9yKCdDb21wb25lbnQgPEdydlRhYmxlQ29sdW1uIC8+IHNob3VsZCBuZXZlciByZW5kZXInKTtcbiAgfVxufSlcblxuZXhwb3J0IGRlZmF1bHQgR3J2VGFibGU7XG5leHBvcnQge1xuICBHcnZUYWJsZUNvbHVtbiBhcyBDb2x1bW4sXG4gIEdydlRhYmxlIGFzIFRhYmxlLFxuICBHcnZUYWJsZUNlbGwgYXMgQ2VsbCxcbiAgR3J2VGFibGVUZXh0Q2VsbCBhcyBUZXh0Q2VsbCxcbiAgU29ydEhlYWRlckNlbGwsXG4gIFNvcnRJbmRpY2F0b3IsXG4gIFNvcnRUeXBlc307XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy90YWJsZS5qc3hcbiAqKi8iLCJ2YXIgVGVybSA9IHJlcXVpcmUoJ1Rlcm1pbmFsJyk7XG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHtkZWJvdW5jZSwgaXNOdW1iZXJ9ID0gcmVxdWlyZSgnXycpO1xuXG5UZXJtLmNvbG9yc1syNTZdID0gJyMyNTIzMjMnO1xuXG5jb25zdCBESVNDT05ORUNUX1RYVCA9ICdcXHgxYlszMW1kaXNjb25uZWN0ZWRcXHgxYlttXFxyXFxuJztcbmNvbnN0IENPTk5FQ1RFRF9UWFQgPSAnQ29ubmVjdGVkIVxcclxcbic7XG5cbnZhciBUdHlUZXJtaW5hbCA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBnZXRJbml0aWFsU3RhdGUoKXtcbiAgICB0aGlzLnJvd3MgPSB0aGlzLnByb3BzLnJvd3M7XG4gICAgdGhpcy5jb2xzID0gdGhpcy5wcm9wcy5jb2xzO1xuICAgIHRoaXMudHR5ID0gdGhpcy5wcm9wcy50dHk7XG5cbiAgICB0aGlzLmRlYm91bmNlZFJlc2l6ZSA9IGRlYm91bmNlKCgpPT57XG4gICAgICB0aGlzLnJlc2l6ZSgpO1xuICAgICAgdGhpcy50dHkucmVzaXplKHRoaXMuY29scywgdGhpcy5yb3dzKTtcbiAgICB9LCAyMDApO1xuXG4gICAgcmV0dXJuIHt9O1xuICB9LFxuXG4gIGNvbXBvbmVudERpZE1vdW50OiBmdW5jdGlvbigpIHtcbiAgICB0aGlzLnRlcm0gPSBuZXcgVGVybWluYWwoe1xuICAgICAgY29sczogNSxcbiAgICAgIHJvd3M6IDUsXG4gICAgICB1c2VTdHlsZTogdHJ1ZSxcbiAgICAgIHNjcmVlbktleXM6IHRydWUsXG4gICAgICBjdXJzb3JCbGluazogdHJ1ZVxuICAgIH0pO1xuXG4gICAgdGhpcy50ZXJtLm9wZW4odGhpcy5yZWZzLmNvbnRhaW5lcik7XG4gICAgdGhpcy50ZXJtLm9uKCdkYXRhJywgKGRhdGEpID0+IHRoaXMudHR5LnNlbmQoZGF0YSkpO1xuXG4gICAgdGhpcy5yZXNpemUodGhpcy5jb2xzLCB0aGlzLnJvd3MpO1xuXG4gICAgdGhpcy50dHkub24oJ29wZW4nLCAoKT0+IHRoaXMudGVybS53cml0ZShDT05ORUNURURfVFhUKSk7XG4gICAgdGhpcy50dHkub24oJ2RhdGEnLCAoZGF0YSkgPT4gdGhpcy50ZXJtLndyaXRlKGRhdGEpKTtcbiAgICB0aGlzLnR0eS5vbigncmVzZXQnLCAoKT0+IHRoaXMudGVybS5yZXNldCgpKTtcblxuICAgIHRoaXMudHR5LmNvbm5lY3Qoe2NvbHM6IHRoaXMuY29scywgcm93czogdGhpcy5yb3dzfSk7XG4gICAgd2luZG93LmFkZEV2ZW50TGlzdGVuZXIoJ3Jlc2l6ZScsIHRoaXMuZGVib3VuY2VkUmVzaXplKTtcbiAgfSxcblxuICBjb21wb25lbnRXaWxsVW5tb3VudDogZnVuY3Rpb24oKSB7XG4gICAgdGhpcy50ZXJtLmRlc3Ryb3koKTtcbiAgICB3aW5kb3cucmVtb3ZlRXZlbnRMaXN0ZW5lcigncmVzaXplJywgdGhpcy5kZWJvdW5jZWRSZXNpemUpO1xuICB9LFxuXG4gIHNob3VsZENvbXBvbmVudFVwZGF0ZTogZnVuY3Rpb24obmV3UHJvcHMpIHtcbiAgICB2YXIge3Jvd3MsIGNvbHN9ID0gbmV3UHJvcHM7XG5cbiAgICBpZiggIWlzTnVtYmVyKHJvd3MpIHx8ICFpc051bWJlcihjb2xzKSl7XG4gICAgICByZXR1cm4gZmFsc2U7XG4gICAgfVxuXG4gICAgaWYocm93cyAhPT0gdGhpcy5yb3dzIHx8IGNvbHMgIT09IHRoaXMuY29scyl7XG4gICAgICB0aGlzLnJlc2l6ZShjb2xzLCByb3dzKVxuICAgIH1cblxuICAgIHJldHVybiBmYWxzZTtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuICggPGRpdiBjbGFzc05hbWU9XCJncnYtdGVybWluYWxcIiBpZD1cInRlcm1pbmFsLWJveFwiIHJlZj1cImNvbnRhaW5lclwiPiAgPC9kaXY+ICk7XG4gIH0sXG5cbiAgcmVzaXplOiBmdW5jdGlvbihjb2xzLCByb3dzKSB7XG4gICAgLy8gaWYgbm90IGRlZmluZWQsIHVzZSB0aGUgc2l6ZSBvZiB0aGUgY29udGFpbmVyXG4gICAgaWYoIWlzTnVtYmVyKGNvbHMpIHx8ICFpc051bWJlcihyb3dzKSl7XG4gICAgICBsZXQgZGltID0gdGhpcy5fZ2V0RGltZW5zaW9ucygpO1xuICAgICAgY29scyA9IGRpbS5jb2xzO1xuICAgICAgcm93cyA9IGRpbS5yb3dzO1xuICAgIH1cblxuICAgIHRoaXMuY29scyA9IGNvbHM7XG4gICAgdGhpcy5yb3dzID0gcm93cztcblxuICAgIHRoaXMudGVybS5yZXNpemUodGhpcy5jb2xzLCB0aGlzLnJvd3MpO1xuICB9LFxuXG4gIF9nZXREaW1lbnNpb25zKCl7XG4gICAgbGV0ICRjb250YWluZXIgPSAkKHRoaXMucmVmcy5jb250YWluZXIpO1xuICAgIGxldCBmYWtlUm93ID0gJCgnPGRpdj48c3Bhbj4mbmJzcDs8L3NwYW4+PC9kaXY+Jyk7XG5cbiAgICAkY29udGFpbmVyLmZpbmQoJy50ZXJtaW5hbCcpLmFwcGVuZChmYWtlUm93KTtcbiAgICAvLyBnZXQgZGl2IGhlaWdodFxuICAgIGxldCBmYWtlQ29sSGVpZ2h0ID0gZmFrZVJvd1swXS5nZXRCb3VuZGluZ0NsaWVudFJlY3QoKS5oZWlnaHQ7XG4gICAgLy8gZ2V0IHNwYW4gd2lkdGhcbiAgICBsZXQgZmFrZUNvbFdpZHRoID0gZmFrZVJvdy5jaGlsZHJlbigpLmZpcnN0KClbMF0uZ2V0Qm91bmRpbmdDbGllbnRSZWN0KCkud2lkdGg7XG5cbiAgICBsZXQgd2lkdGggPSAkY29udGFpbmVyWzBdLmNsaWVudFdpZHRoO1xuICAgIGxldCBoZWlnaHQgPSAkY29udGFpbmVyWzBdLmNsaWVudEhlaWdodDtcblxuICAgIGxldCBjb2xzID0gTWF0aC5mbG9vcih3aWR0aCAvIChmYWtlQ29sV2lkdGgpKTtcbiAgICBsZXQgcm93cyA9IE1hdGguZmxvb3IoaGVpZ2h0IC8gKGZha2VDb2xIZWlnaHQpKTtcbiAgICBmYWtlUm93LnJlbW92ZSgpO1xuXG4gICAgcmV0dXJuIHtjb2xzLCByb3dzfTtcbiAgfVxuXG59KTtcblxuVHR5VGVybWluYWwucHJvcFR5cGVzID0ge1xuICB0dHk6IFJlYWN0LlByb3BUeXBlcy5vYmplY3QuaXNSZXF1aXJlZFxufVxuXG5tb2R1bGUuZXhwb3J0cyA9IFR0eVRlcm1pbmFsO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvdGVybWluYWwuanN4XG4gKiovIiwiLypcbiAqICBUaGUgTUlUIExpY2Vuc2UgKE1JVClcbiAqICBDb3B5cmlnaHQgKGMpIDIwMTUgUnlhbiBGbG9yZW5jZSwgTWljaGFlbCBKYWNrc29uXG4gKiAgUGVybWlzc2lvbiBpcyBoZXJlYnkgZ3JhbnRlZCwgZnJlZSBvZiBjaGFyZ2UsIHRvIGFueSBwZXJzb24gb2J0YWluaW5nIGEgY29weSBvZiB0aGlzIHNvZnR3YXJlIGFuZCBhc3NvY2lhdGVkIGRvY3VtZW50YXRpb24gZmlsZXMgKHRoZSBcIlNvZnR3YXJlXCIpLCB0byBkZWFsIGluIHRoZSBTb2Z0d2FyZSB3aXRob3V0IHJlc3RyaWN0aW9uLCBpbmNsdWRpbmcgd2l0aG91dCBsaW1pdGF0aW9uIHRoZSByaWdodHMgdG8gdXNlLCBjb3B5LCBtb2RpZnksIG1lcmdlLCBwdWJsaXNoLCBkaXN0cmlidXRlLCBzdWJsaWNlbnNlLCBhbmQvb3Igc2VsbCBjb3BpZXMgb2YgdGhlIFNvZnR3YXJlLCBhbmQgdG8gcGVybWl0IHBlcnNvbnMgdG8gd2hvbSB0aGUgU29mdHdhcmUgaXMgZnVybmlzaGVkIHRvIGRvIHNvLCBzdWJqZWN0IHRvIHRoZSBmb2xsb3dpbmcgY29uZGl0aW9uczpcbiAqICBUaGUgYWJvdmUgY29weXJpZ2h0IG5vdGljZSBhbmQgdGhpcyBwZXJtaXNzaW9uIG5vdGljZSBzaGFsbCBiZSBpbmNsdWRlZCBpbiBhbGwgY29waWVzIG9yIHN1YnN0YW50aWFsIHBvcnRpb25zIG9mIHRoZSBTb2Z0d2FyZS5cbiAqICBUSEUgU09GVFdBUkUgSVMgUFJPVklERUQgXCJBUyBJU1wiLCBXSVRIT1VUIFdBUlJBTlRZIE9GIEFOWSBLSU5ELCBFWFBSRVNTIE9SIElNUExJRUQsIElOQ0xVRElORyBCVVQgTk9UIExJTUlURUQgVE8gVEhFIFdBUlJBTlRJRVMgT0YgTUVSQ0hBTlRBQklMSVRZLCBGSVRORVNTIEZPUiBBIFBBUlRJQ1VMQVIgUFVSUE9TRSBBTkQgTk9OSU5GUklOR0VNRU5ULiBJTiBOTyBFVkVOVCBTSEFMTCBUSEUgQVVUSE9SUyBPUiBDT1BZUklHSFQgSE9MREVSUyBCRSBMSUFCTEUgRk9SIEFOWSBDTEFJTSwgREFNQUdFUyBPUiBPVEhFUiBMSUFCSUxJVFksIFdIRVRIRVIgSU4gQU4gQUNUSU9OIE9GIENPTlRSQUNULCBUT1JUIE9SIE9USEVSV0lTRSwgQVJJU0lORyBGUk9NLCBPVVQgT0YgT1IgSU4gQ09OTkVDVElPTiBXSVRIIFRIRSBTT0ZUV0FSRSBPUiBUSEUgVVNFIE9SIE9USEVSIERFQUxJTkdTIElOIFRIRSBTT0ZUV0FSRS5cbiovXG5cbmltcG9ydCBpbnZhcmlhbnQgZnJvbSAnaW52YXJpYW50J1xuXG5mdW5jdGlvbiBlc2NhcGVSZWdFeHAoc3RyaW5nKSB7XG4gIHJldHVybiBzdHJpbmcucmVwbGFjZSgvWy4qKz9eJHt9KCl8W1xcXVxcXFxdL2csICdcXFxcJCYnKVxufVxuXG5mdW5jdGlvbiBlc2NhcGVTb3VyY2Uoc3RyaW5nKSB7XG4gIHJldHVybiBlc2NhcGVSZWdFeHAoc3RyaW5nKS5yZXBsYWNlKC9cXC8rL2csICcvKycpXG59XG5cbmZ1bmN0aW9uIF9jb21waWxlUGF0dGVybihwYXR0ZXJuKSB7XG4gIGxldCByZWdleHBTb3VyY2UgPSAnJztcbiAgY29uc3QgcGFyYW1OYW1lcyA9IFtdO1xuICBjb25zdCB0b2tlbnMgPSBbXTtcblxuICBsZXQgbWF0Y2gsIGxhc3RJbmRleCA9IDAsIG1hdGNoZXIgPSAvOihbYS16QS1aXyRdW2EtekEtWjAtOV8kXSopfFxcKlxcKnxcXCp8XFwofFxcKS9nXG4gIC8qZXNsaW50IG5vLWNvbmQtYXNzaWduOiAwKi9cbiAgd2hpbGUgKChtYXRjaCA9IG1hdGNoZXIuZXhlYyhwYXR0ZXJuKSkpIHtcbiAgICBpZiAobWF0Y2guaW5kZXggIT09IGxhc3RJbmRleCkge1xuICAgICAgdG9rZW5zLnB1c2gocGF0dGVybi5zbGljZShsYXN0SW5kZXgsIG1hdGNoLmluZGV4KSlcbiAgICAgIHJlZ2V4cFNvdXJjZSArPSBlc2NhcGVTb3VyY2UocGF0dGVybi5zbGljZShsYXN0SW5kZXgsIG1hdGNoLmluZGV4KSlcbiAgICB9XG5cbiAgICBpZiAobWF0Y2hbMV0pIHtcbiAgICAgIHJlZ2V4cFNvdXJjZSArPSAnKFteLz8jXSspJztcbiAgICAgIHBhcmFtTmFtZXMucHVzaChtYXRjaFsxXSk7XG4gICAgfSBlbHNlIGlmIChtYXRjaFswXSA9PT0gJyoqJykge1xuICAgICAgcmVnZXhwU291cmNlICs9ICcoW1xcXFxzXFxcXFNdKiknXG4gICAgICBwYXJhbU5hbWVzLnB1c2goJ3NwbGF0Jyk7XG4gICAgfSBlbHNlIGlmIChtYXRjaFswXSA9PT0gJyonKSB7XG4gICAgICByZWdleHBTb3VyY2UgKz0gJyhbXFxcXHNcXFxcU10qPyknXG4gICAgICBwYXJhbU5hbWVzLnB1c2goJ3NwbGF0Jyk7XG4gICAgfSBlbHNlIGlmIChtYXRjaFswXSA9PT0gJygnKSB7XG4gICAgICByZWdleHBTb3VyY2UgKz0gJyg/Oic7XG4gICAgfSBlbHNlIGlmIChtYXRjaFswXSA9PT0gJyknKSB7XG4gICAgICByZWdleHBTb3VyY2UgKz0gJyk/JztcbiAgICB9XG5cbiAgICB0b2tlbnMucHVzaChtYXRjaFswXSk7XG5cbiAgICBsYXN0SW5kZXggPSBtYXRjaGVyLmxhc3RJbmRleDtcbiAgfVxuXG4gIGlmIChsYXN0SW5kZXggIT09IHBhdHRlcm4ubGVuZ3RoKSB7XG4gICAgdG9rZW5zLnB1c2gocGF0dGVybi5zbGljZShsYXN0SW5kZXgsIHBhdHRlcm4ubGVuZ3RoKSlcbiAgICByZWdleHBTb3VyY2UgKz0gZXNjYXBlU291cmNlKHBhdHRlcm4uc2xpY2UobGFzdEluZGV4LCBwYXR0ZXJuLmxlbmd0aCkpXG4gIH1cblxuICByZXR1cm4ge1xuICAgIHBhdHRlcm4sXG4gICAgcmVnZXhwU291cmNlLFxuICAgIHBhcmFtTmFtZXMsXG4gICAgdG9rZW5zXG4gIH1cbn1cblxuY29uc3QgQ29tcGlsZWRQYXR0ZXJuc0NhY2hlID0ge31cblxuZXhwb3J0IGZ1bmN0aW9uIGNvbXBpbGVQYXR0ZXJuKHBhdHRlcm4pIHtcbiAgaWYgKCEocGF0dGVybiBpbiBDb21waWxlZFBhdHRlcm5zQ2FjaGUpKVxuICAgIENvbXBpbGVkUGF0dGVybnNDYWNoZVtwYXR0ZXJuXSA9IF9jb21waWxlUGF0dGVybihwYXR0ZXJuKVxuXG4gIHJldHVybiBDb21waWxlZFBhdHRlcm5zQ2FjaGVbcGF0dGVybl1cbn1cblxuLyoqXG4gKiBBdHRlbXB0cyB0byBtYXRjaCBhIHBhdHRlcm4gb24gdGhlIGdpdmVuIHBhdGhuYW1lLiBQYXR0ZXJucyBtYXkgdXNlXG4gKiB0aGUgZm9sbG93aW5nIHNwZWNpYWwgY2hhcmFjdGVyczpcbiAqXG4gKiAtIDpwYXJhbU5hbWUgICAgIE1hdGNoZXMgYSBVUkwgc2VnbWVudCB1cCB0byB0aGUgbmV4dCAvLCA/LCBvciAjLiBUaGVcbiAqICAgICAgICAgICAgICAgICAgY2FwdHVyZWQgc3RyaW5nIGlzIGNvbnNpZGVyZWQgYSBcInBhcmFtXCJcbiAqIC0gKCkgICAgICAgICAgICAgV3JhcHMgYSBzZWdtZW50IG9mIHRoZSBVUkwgdGhhdCBpcyBvcHRpb25hbFxuICogLSAqICAgICAgICAgICAgICBDb25zdW1lcyAobm9uLWdyZWVkeSkgYWxsIGNoYXJhY3RlcnMgdXAgdG8gdGhlIG5leHRcbiAqICAgICAgICAgICAgICAgICAgY2hhcmFjdGVyIGluIHRoZSBwYXR0ZXJuLCBvciB0byB0aGUgZW5kIG9mIHRoZSBVUkwgaWZcbiAqICAgICAgICAgICAgICAgICAgdGhlcmUgaXMgbm9uZVxuICogLSAqKiAgICAgICAgICAgICBDb25zdW1lcyAoZ3JlZWR5KSBhbGwgY2hhcmFjdGVycyB1cCB0byB0aGUgbmV4dCBjaGFyYWN0ZXJcbiAqICAgICAgICAgICAgICAgICAgaW4gdGhlIHBhdHRlcm4sIG9yIHRvIHRoZSBlbmQgb2YgdGhlIFVSTCBpZiB0aGVyZSBpcyBub25lXG4gKlxuICogVGhlIHJldHVybiB2YWx1ZSBpcyBhbiBvYmplY3Qgd2l0aCB0aGUgZm9sbG93aW5nIHByb3BlcnRpZXM6XG4gKlxuICogLSByZW1haW5pbmdQYXRobmFtZVxuICogLSBwYXJhbU5hbWVzXG4gKiAtIHBhcmFtVmFsdWVzXG4gKi9cbmV4cG9ydCBmdW5jdGlvbiBtYXRjaFBhdHRlcm4ocGF0dGVybiwgcGF0aG5hbWUpIHtcbiAgLy8gTWFrZSBsZWFkaW5nIHNsYXNoZXMgY29uc2lzdGVudCBiZXR3ZWVuIHBhdHRlcm4gYW5kIHBhdGhuYW1lLlxuICBpZiAocGF0dGVybi5jaGFyQXQoMCkgIT09ICcvJykge1xuICAgIHBhdHRlcm4gPSBgLyR7cGF0dGVybn1gXG4gIH1cbiAgaWYgKHBhdGhuYW1lLmNoYXJBdCgwKSAhPT0gJy8nKSB7XG4gICAgcGF0aG5hbWUgPSBgLyR7cGF0aG5hbWV9YFxuICB9XG5cbiAgbGV0IHsgcmVnZXhwU291cmNlLCBwYXJhbU5hbWVzLCB0b2tlbnMgfSA9IGNvbXBpbGVQYXR0ZXJuKHBhdHRlcm4pXG5cbiAgcmVnZXhwU291cmNlICs9ICcvKicgLy8gQ2FwdHVyZSBwYXRoIHNlcGFyYXRvcnNcblxuICAvLyBTcGVjaWFsLWNhc2UgcGF0dGVybnMgbGlrZSAnKicgZm9yIGNhdGNoLWFsbCByb3V0ZXMuXG4gIGNvbnN0IGNhcHR1cmVSZW1haW5pbmcgPSB0b2tlbnNbdG9rZW5zLmxlbmd0aCAtIDFdICE9PSAnKidcblxuICBpZiAoY2FwdHVyZVJlbWFpbmluZykge1xuICAgIC8vIFRoaXMgd2lsbCBtYXRjaCBuZXdsaW5lcyBpbiB0aGUgcmVtYWluaW5nIHBhdGguXG4gICAgcmVnZXhwU291cmNlICs9ICcoW1xcXFxzXFxcXFNdKj8pJ1xuICB9XG5cbiAgY29uc3QgbWF0Y2ggPSBwYXRobmFtZS5tYXRjaChuZXcgUmVnRXhwKCdeJyArIHJlZ2V4cFNvdXJjZSArICckJywgJ2knKSlcblxuICBsZXQgcmVtYWluaW5nUGF0aG5hbWUsIHBhcmFtVmFsdWVzXG4gIGlmIChtYXRjaCAhPSBudWxsKSB7XG4gICAgaWYgKGNhcHR1cmVSZW1haW5pbmcpIHtcbiAgICAgIHJlbWFpbmluZ1BhdGhuYW1lID0gbWF0Y2gucG9wKClcbiAgICAgIGNvbnN0IG1hdGNoZWRQYXRoID1cbiAgICAgICAgbWF0Y2hbMF0uc3Vic3RyKDAsIG1hdGNoWzBdLmxlbmd0aCAtIHJlbWFpbmluZ1BhdGhuYW1lLmxlbmd0aClcblxuICAgICAgLy8gSWYgd2UgZGlkbid0IG1hdGNoIHRoZSBlbnRpcmUgcGF0aG5hbWUsIHRoZW4gbWFrZSBzdXJlIHRoYXQgdGhlIG1hdGNoXG4gICAgICAvLyB3ZSBkaWQgZ2V0IGVuZHMgYXQgYSBwYXRoIHNlcGFyYXRvciAocG90ZW50aWFsbHkgdGhlIG9uZSB3ZSBhZGRlZFxuICAgICAgLy8gYWJvdmUgYXQgdGhlIGJlZ2lubmluZyBvZiB0aGUgcGF0aCwgaWYgdGhlIGFjdHVhbCBtYXRjaCB3YXMgZW1wdHkpLlxuICAgICAgaWYgKFxuICAgICAgICByZW1haW5pbmdQYXRobmFtZSAmJlxuICAgICAgICBtYXRjaGVkUGF0aC5jaGFyQXQobWF0Y2hlZFBhdGgubGVuZ3RoIC0gMSkgIT09ICcvJ1xuICAgICAgKSB7XG4gICAgICAgIHJldHVybiB7XG4gICAgICAgICAgcmVtYWluaW5nUGF0aG5hbWU6IG51bGwsXG4gICAgICAgICAgcGFyYW1OYW1lcyxcbiAgICAgICAgICBwYXJhbVZhbHVlczogbnVsbFxuICAgICAgICB9XG4gICAgICB9XG4gICAgfSBlbHNlIHtcbiAgICAgIC8vIElmIHRoaXMgbWF0Y2hlZCBhdCBhbGwsIHRoZW4gdGhlIG1hdGNoIHdhcyB0aGUgZW50aXJlIHBhdGhuYW1lLlxuICAgICAgcmVtYWluaW5nUGF0aG5hbWUgPSAnJ1xuICAgIH1cblxuICAgIHBhcmFtVmFsdWVzID0gbWF0Y2guc2xpY2UoMSkubWFwKFxuICAgICAgdiA9PiB2ICE9IG51bGwgPyBkZWNvZGVVUklDb21wb25lbnQodikgOiB2XG4gICAgKVxuICB9IGVsc2Uge1xuICAgIHJlbWFpbmluZ1BhdGhuYW1lID0gcGFyYW1WYWx1ZXMgPSBudWxsXG4gIH1cblxuICByZXR1cm4ge1xuICAgIHJlbWFpbmluZ1BhdGhuYW1lLFxuICAgIHBhcmFtTmFtZXMsXG4gICAgcGFyYW1WYWx1ZXNcbiAgfVxufVxuXG5leHBvcnQgZnVuY3Rpb24gZ2V0UGFyYW1OYW1lcyhwYXR0ZXJuKSB7XG4gIHJldHVybiBjb21waWxlUGF0dGVybihwYXR0ZXJuKS5wYXJhbU5hbWVzXG59XG5cbmV4cG9ydCBmdW5jdGlvbiBnZXRQYXJhbXMocGF0dGVybiwgcGF0aG5hbWUpIHtcbiAgY29uc3QgeyBwYXJhbU5hbWVzLCBwYXJhbVZhbHVlcyB9ID0gbWF0Y2hQYXR0ZXJuKHBhdHRlcm4sIHBhdGhuYW1lKVxuXG4gIGlmIChwYXJhbVZhbHVlcyAhPSBudWxsKSB7XG4gICAgcmV0dXJuIHBhcmFtTmFtZXMucmVkdWNlKGZ1bmN0aW9uIChtZW1vLCBwYXJhbU5hbWUsIGluZGV4KSB7XG4gICAgICBtZW1vW3BhcmFtTmFtZV0gPSBwYXJhbVZhbHVlc1tpbmRleF1cbiAgICAgIHJldHVybiBtZW1vXG4gICAgfSwge30pXG4gIH1cblxuICByZXR1cm4gbnVsbFxufVxuXG4vKipcbiAqIFJldHVybnMgYSB2ZXJzaW9uIG9mIHRoZSBnaXZlbiBwYXR0ZXJuIHdpdGggcGFyYW1zIGludGVycG9sYXRlZC4gVGhyb3dzXG4gKiBpZiB0aGVyZSBpcyBhIGR5bmFtaWMgc2VnbWVudCBvZiB0aGUgcGF0dGVybiBmb3Igd2hpY2ggdGhlcmUgaXMgbm8gcGFyYW0uXG4gKi9cbmV4cG9ydCBmdW5jdGlvbiBmb3JtYXRQYXR0ZXJuKHBhdHRlcm4sIHBhcmFtcykge1xuICBwYXJhbXMgPSBwYXJhbXMgfHwge31cblxuICBjb25zdCB7IHRva2VucyB9ID0gY29tcGlsZVBhdHRlcm4ocGF0dGVybilcbiAgbGV0IHBhcmVuQ291bnQgPSAwLCBwYXRobmFtZSA9ICcnLCBzcGxhdEluZGV4ID0gMFxuXG4gIGxldCB0b2tlbiwgcGFyYW1OYW1lLCBwYXJhbVZhbHVlXG4gIGZvciAobGV0IGkgPSAwLCBsZW4gPSB0b2tlbnMubGVuZ3RoOyBpIDwgbGVuOyArK2kpIHtcbiAgICB0b2tlbiA9IHRva2Vuc1tpXVxuXG4gICAgaWYgKHRva2VuID09PSAnKicgfHwgdG9rZW4gPT09ICcqKicpIHtcbiAgICAgIHBhcmFtVmFsdWUgPSBBcnJheS5pc0FycmF5KHBhcmFtcy5zcGxhdCkgPyBwYXJhbXMuc3BsYXRbc3BsYXRJbmRleCsrXSA6IHBhcmFtcy5zcGxhdFxuXG4gICAgICBpbnZhcmlhbnQoXG4gICAgICAgIHBhcmFtVmFsdWUgIT0gbnVsbCB8fCBwYXJlbkNvdW50ID4gMCxcbiAgICAgICAgJ01pc3Npbmcgc3BsYXQgIyVzIGZvciBwYXRoIFwiJXNcIicsXG4gICAgICAgIHNwbGF0SW5kZXgsIHBhdHRlcm5cbiAgICAgIClcblxuICAgICAgaWYgKHBhcmFtVmFsdWUgIT0gbnVsbClcbiAgICAgICAgcGF0aG5hbWUgKz0gZW5jb2RlVVJJKHBhcmFtVmFsdWUpXG4gICAgfSBlbHNlIGlmICh0b2tlbiA9PT0gJygnKSB7XG4gICAgICBwYXJlbkNvdW50ICs9IDFcbiAgICB9IGVsc2UgaWYgKHRva2VuID09PSAnKScpIHtcbiAgICAgIHBhcmVuQ291bnQgLT0gMVxuICAgIH0gZWxzZSBpZiAodG9rZW4uY2hhckF0KDApID09PSAnOicpIHtcbiAgICAgIHBhcmFtTmFtZSA9IHRva2VuLnN1YnN0cmluZygxKVxuICAgICAgcGFyYW1WYWx1ZSA9IHBhcmFtc1twYXJhbU5hbWVdXG5cbiAgICAgIGludmFyaWFudChcbiAgICAgICAgcGFyYW1WYWx1ZSAhPSBudWxsIHx8IHBhcmVuQ291bnQgPiAwLFxuICAgICAgICAnTWlzc2luZyBcIiVzXCIgcGFyYW1ldGVyIGZvciBwYXRoIFwiJXNcIicsXG4gICAgICAgIHBhcmFtTmFtZSwgcGF0dGVyblxuICAgICAgKVxuXG4gICAgICBpZiAocGFyYW1WYWx1ZSAhPSBudWxsKVxuICAgICAgICBwYXRobmFtZSArPSBlbmNvZGVVUklDb21wb25lbnQocGFyYW1WYWx1ZSlcbiAgICB9IGVsc2Uge1xuICAgICAgcGF0aG5hbWUgKz0gdG9rZW5cbiAgICB9XG4gIH1cblxuICByZXR1cm4gcGF0aG5hbWUucmVwbGFjZSgvXFwvKy9nLCAnLycpXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tbW9uL3BhdHRlcm5VdGlscy5qc1xuICoqLyIsInZhciBUdHkgPSByZXF1aXJlKCdhcHAvY29tbW9uL3R0eScpO1xudmFyIGFwaSA9IHJlcXVpcmUoJ2FwcC9zZXJ2aWNlcy9hcGknKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG5cbmNsYXNzIFR0eVBsYXllciBleHRlbmRzIFR0eSB7XG4gIGNvbnN0cnVjdG9yKHtzaWR9KXtcbiAgICBzdXBlcih7fSk7XG4gICAgdGhpcy5zaWQgPSBzaWQ7XG4gICAgdGhpcy5jdXJyZW50ID0gMTtcbiAgICB0aGlzLmxlbmd0aCA9IC0xO1xuICAgIHRoaXMudHR5U3RlYW0gPSBuZXcgQXJyYXkoKTtcbiAgICB0aGlzLmlzTG9haW5kID0gZmFsc2U7XG4gICAgdGhpcy5pc1BsYXlpbmcgPSBmYWxzZTtcbiAgICB0aGlzLmlzRXJyb3IgPSBmYWxzZTtcbiAgICB0aGlzLmlzUmVhZHkgPSBmYWxzZTtcbiAgICB0aGlzLmlzTG9hZGluZyA9IHRydWU7XG4gIH1cblxuICBzZW5kKCl7XG4gIH1cblxuICByZXNpemUoKXtcbiAgfVxuXG4gIGNvbm5lY3QoKXtcbiAgICBhcGkuZ2V0KGNmZy5hcGkuZ2V0RmV0Y2hTZXNzaW9uTGVuZ3RoVXJsKHRoaXMuc2lkKSlcbiAgICAgIC5kb25lKChkYXRhKT0+e1xuICAgICAgICB0aGlzLmxlbmd0aCA9IGRhdGEuY291bnQ7XG4gICAgICAgIHRoaXMuaXNSZWFkeSA9IHRydWU7XG4gICAgICB9KVxuICAgICAgLmZhaWwoKCk9PntcbiAgICAgICAgdGhpcy5pc0Vycm9yID0gdHJ1ZTtcbiAgICAgIH0pXG4gICAgICAuYWx3YXlzKCgpPT57XG4gICAgICAgIHRoaXMuX2NoYW5nZSgpO1xuICAgICAgfSk7XG4gIH1cblxuICBtb3ZlKG5ld1Bvcyl7XG4gICAgaWYoIXRoaXMuaXNSZWFkeSl7XG4gICAgICByZXR1cm47XG4gICAgfVxuXG4gICAgaWYobmV3UG9zID09PSB1bmRlZmluZWQpe1xuICAgICAgbmV3UG9zID0gdGhpcy5jdXJyZW50ICsgMTtcbiAgICB9XG5cbiAgICBpZihuZXdQb3MgPiB0aGlzLmxlbmd0aCl7XG4gICAgICBuZXdQb3MgPSB0aGlzLmxlbmd0aDtcbiAgICAgIHRoaXMuc3RvcCgpO1xuICAgIH1cblxuICAgIGlmKG5ld1BvcyA9PT0gMCl7XG4gICAgICBuZXdQb3MgPSAxO1xuICAgIH1cblxuICAgIGlmKHRoaXMuaXNQbGF5aW5nKXtcbiAgICAgIGlmKHRoaXMuY3VycmVudCA8IG5ld1Bvcyl7XG4gICAgICAgIHRoaXMuX3Nob3dDaHVuayh0aGlzLmN1cnJlbnQsIG5ld1Bvcyk7XG4gICAgICB9ZWxzZXtcbiAgICAgICAgdGhpcy5lbWl0KCdyZXNldCcpO1xuICAgICAgICB0aGlzLl9zaG93Q2h1bmsodGhpcy5jdXJyZW50LCBuZXdQb3MpO1xuICAgICAgfVxuICAgIH1lbHNle1xuICAgICAgdGhpcy5jdXJyZW50ID0gbmV3UG9zO1xuICAgIH1cblxuICAgIHRoaXMuX2NoYW5nZSgpO1xuICB9XG5cbiAgc3RvcCgpe1xuICAgIHRoaXMuaXNQbGF5aW5nID0gZmFsc2U7XG4gICAgdGhpcy50aW1lciA9IGNsZWFySW50ZXJ2YWwodGhpcy50aW1lcik7XG4gICAgdGhpcy5fY2hhbmdlKCk7XG4gIH1cblxuICBwbGF5KCl7XG4gICAgaWYodGhpcy5pc1BsYXlpbmcpe1xuICAgICAgcmV0dXJuO1xuICAgIH1cblxuICAgIHRoaXMuaXNQbGF5aW5nID0gdHJ1ZTtcblxuICAgIC8vIHN0YXJ0IGZyb20gdGhlIGJlZ2lubmluZyBpZiBhdCB0aGUgZW5kXG4gICAgaWYodGhpcy5jdXJyZW50ID09PSB0aGlzLmxlbmd0aCl7XG4gICAgICB0aGlzLmN1cnJlbnQgPSAxO1xuICAgICAgdGhpcy5lbWl0KCdyZXNldCcpO1xuICAgIH1cblxuICAgIHRoaXMudGltZXIgPSBzZXRJbnRlcnZhbCh0aGlzLm1vdmUuYmluZCh0aGlzKSwgMTUwKTtcbiAgICB0aGlzLl9jaGFuZ2UoKTtcbiAgfVxuXG4gIF9zaG91bGRGZXRjaChzdGFydCwgZW5kKXtcbiAgICBmb3IodmFyIGkgPSBzdGFydDsgaSA8IGVuZDsgaSsrKXtcbiAgICAgIGlmKHRoaXMudHR5U3RlYW1baV0gPT09IHVuZGVmaW5lZCl7XG4gICAgICAgIHJldHVybiB0cnVlO1xuICAgICAgfVxuICAgIH1cblxuICAgIHJldHVybiBmYWxzZTtcbiAgfVxuXG4gIF9mZXRjaChzdGFydCwgZW5kKXtcbiAgICBlbmQgPSBlbmQgKyA1MDtcbiAgICBlbmQgPSBlbmQgPiB0aGlzLmxlbmd0aCA/IHRoaXMubGVuZ3RoIDogZW5kO1xuICAgIHJldHVybiBhcGkuZ2V0KGNmZy5hcGkuZ2V0RmV0Y2hTZXNzaW9uQ2h1bmtVcmwoe3NpZDogdGhpcy5zaWQsIHN0YXJ0LCBlbmR9KSkuXG4gICAgICBkb25lKChyZXNwb25zZSk9PntcbiAgICAgICAgZm9yKHZhciBpID0gMDsgaSA8IGVuZC1zdGFydDsgaSsrKXtcbiAgICAgICAgICB2YXIgZGF0YSA9IGF0b2IocmVzcG9uc2UuY2h1bmtzW2ldLmRhdGEpIHx8ICcnO1xuICAgICAgICAgIHZhciBkZWxheSA9IHJlc3BvbnNlLmNodW5rc1tpXS5kZWxheTtcbiAgICAgICAgICB0aGlzLnR0eVN0ZWFtW3N0YXJ0K2ldID0geyBkYXRhLCBkZWxheX07XG4gICAgICAgIH1cbiAgICAgIH0pO1xuICB9XG5cbiAgX3Nob3dDaHVuayhzdGFydCwgZW5kKXtcbiAgICB2YXIgZGlzcGxheSA9ICgpPT57XG4gICAgICBmb3IodmFyIGkgPSBzdGFydDsgaSA8IGVuZDsgaSsrKXtcbiAgICAgICAgdGhpcy5lbWl0KCdkYXRhJywgdGhpcy50dHlTdGVhbVtpXS5kYXRhKTtcbiAgICAgIH1cbiAgICAgIHRoaXMuY3VycmVudCA9IGVuZDtcbiAgICB9O1xuXG4gICAgaWYodGhpcy5fc2hvdWxkRmV0Y2goc3RhcnQsIGVuZCkpe1xuICAgICAgdGhpcy5fZmV0Y2goc3RhcnQsIGVuZCkudGhlbihkaXNwbGF5KTtcbiAgICB9ZWxzZXtcbiAgICAgIGRpc3BsYXkoKTtcbiAgICB9XG4gIH1cblxuICBfY2hhbmdlKCl7XG4gICAgdGhpcy5lbWl0KCdjaGFuZ2UnKTtcbiAgfVxufVxuXG5leHBvcnQgZGVmYXVsdCBUdHlQbGF5ZXI7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tbW9uL3R0eVBsYXllci5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7ZmV0Y2hTZXNzaW9uc30gPSByZXF1aXJlKCcuLy4uL3Nlc3Npb25zL2FjdGlvbnMnKTtcbnZhciB7ZmV0Y2hOb2Rlc30gPSByZXF1aXJlKCcuLy4uL25vZGVzL2FjdGlvbnMnKTtcbnZhciB7bW9udGhSYW5nZX0gPSByZXF1aXJlKCdhcHAvY29tbW9uL2RhdGVVdGlscycpO1xuXG52YXIgJCA9IHJlcXVpcmUoJ2pRdWVyeScpO1xuXG52YXIgeyBUTFBUX0FQUF9JTklULCBUTFBUX0FQUF9GQUlMRUQsIFRMUFRfQVBQX1JFQURZIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbnZhciBhY3Rpb25zID0ge1xuXG4gIGluaXRBcHAoKSB7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX0FQUF9JTklUKTtcbiAgICBhY3Rpb25zLmZldGNoTm9kZXNBbmRTZXNzaW9ucygpXG4gICAgICAuZG9uZSgoKT0+eyByZWFjdG9yLmRpc3BhdGNoKFRMUFRfQVBQX1JFQURZKTsgfSlcbiAgICAgIC5mYWlsKCgpPT57IHJlYWN0b3IuZGlzcGF0Y2goVExQVF9BUFBfRkFJTEVEKTsgfSk7XG4gIH0sXG5cbiAgZmV0Y2hOb2Rlc0FuZFNlc3Npb25zKCkge1xuICAgIHZhciBbc3RhcnQsIGVuZCBdID0gbW9udGhSYW5nZSgpO1xuICAgIHJldHVybiAkLndoZW4oZmV0Y2hOb2RlcygpLCBmZXRjaFNlc3Npb25zKHN0YXJ0LCBlbmQpKTtcbiAgfVxufVxuXG5leHBvcnQgZGVmYXVsdCBhY3Rpb25zO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvYXBwL2FjdGlvbnMuanNcbiAqKi8iLCJjb25zdCBhcHBTdGF0ZSA9IFtbJ3RscHQnXSwgYXBwPT4gYXBwLnRvSlMoKV07XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgYXBwU3RhdGVcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2FwcC9nZXR0ZXJzLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMuZ2V0dGVycyA9IHJlcXVpcmUoJy4vZ2V0dGVycycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aW9ucyA9IHJlcXVpcmUoJy4vYWN0aW9ucycpO1xubW9kdWxlLmV4cG9ydHMuYXBwU3RvcmUgPSByZXF1aXJlKCcuL2FwcFN0b3JlJyk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hcHAvaW5kZXguanNcbiAqKi8iLCJjb25zdCBkaWFsb2dzID0gW1sndGxwdF9kaWFsb2dzJ10sIHN0YXRlPT4gc3RhdGUudG9KUygpXTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICBkaWFsb2dzXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9kaWFsb2dzL2dldHRlcnMuanNcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cy5nZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3Rpb25zID0gcmVxdWlyZSgnLi9hY3Rpb25zJyk7XG5tb2R1bGUuZXhwb3J0cy5kaWFsb2dTdG9yZSA9IHJlcXVpcmUoJy4vZGlhbG9nU3RvcmUnKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvaW5kZXguanNcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG5yZWFjdG9yLnJlZ2lzdGVyU3RvcmVzKHtcbiAgJ3RscHQnOiByZXF1aXJlKCcuL2FwcC9hcHBTdG9yZScpLFxuICAndGxwdF9kaWFsb2dzJzogcmVxdWlyZSgnLi9kaWFsb2dzL2RpYWxvZ1N0b3JlJyksXG4gICd0bHB0X2FjdGl2ZV90ZXJtaW5hbCc6IHJlcXVpcmUoJy4vYWN0aXZlVGVybWluYWwvYWN0aXZlVGVybVN0b3JlJyksXG4gICd0bHB0X3VzZXInOiByZXF1aXJlKCcuL3VzZXIvdXNlclN0b3JlJyksXG4gICd0bHB0X25vZGVzJzogcmVxdWlyZSgnLi9ub2Rlcy9ub2RlU3RvcmUnKSxcbiAgJ3RscHRfaW52aXRlJzogcmVxdWlyZSgnLi9pbnZpdGUvaW52aXRlU3RvcmUnKSxcbiAgJ3RscHRfcmVzdF9hcGknOiByZXF1aXJlKCcuL3Jlc3RBcGkvcmVzdEFwaVN0b3JlJyksXG4gICd0bHB0X3Nlc3Npb25zJzogcmVxdWlyZSgnLi9zZXNzaW9ucy9zZXNzaW9uU3RvcmUnKVxufSk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9pbmRleC5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7IFRMUFRfUkVDRUlWRV9VU0VSX0lOVklURSB9ICA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcbnZhciB7IEZFVENISU5HX0lOVklURX0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9yZXN0QXBpL2NvbnN0YW50cycpO1xudmFyIHJlc3RBcGlBY3Rpb25zID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvcmVzdEFwaS9hY3Rpb25zJyk7XG52YXIgYXBpID0gcmVxdWlyZSgnYXBwL3NlcnZpY2VzL2FwaScpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICBmZXRjaEludml0ZShpbnZpdGVUb2tlbil7XG4gICAgdmFyIHBhdGggPSBjZmcuYXBpLmdldEludml0ZVVybChpbnZpdGVUb2tlbik7XG4gICAgcmVzdEFwaUFjdGlvbnMuc3RhcnQoRkVUQ0hJTkdfSU5WSVRFKTtcbiAgICBhcGkuZ2V0KHBhdGgpLmRvbmUoaW52aXRlPT57XG4gICAgICByZXN0QXBpQWN0aW9ucy5zdWNjZXNzKEZFVENISU5HX0lOVklURSk7XG4gICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfUkVDRUlWRV9VU0VSX0lOVklURSwgaW52aXRlKTtcbiAgICB9KS5cbiAgICBmYWlsKChlcnIpPT57XG4gICAgICByZXN0QXBpQWN0aW9ucy5mYWlsKEZFVENISU5HX0lOVklURSwgZXJyLnJlc3BvbnNlSlNPTi5tZXNzYWdlKTtcbiAgICB9KTtcbiAgfVxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2FjdGlvbnMuanNcbiAqKi8iLCJ2YXIge1RSWUlOR19UT19TSUdOX1VQLCBGRVRDSElOR19JTlZJVEV9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvcmVzdEFwaS9jb25zdGFudHMnKTtcbnZhciB7cmVxdWVzdFN0YXR1c30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9yZXN0QXBpL2dldHRlcnMnKTtcblxuY29uc3QgaW52aXRlID0gWyBbJ3RscHRfaW52aXRlJ10sIChpbnZpdGUpID0+IGludml0ZSBdO1xuXG5leHBvcnQgZGVmYXVsdCB7XG4gIGludml0ZSxcbiAgYXR0ZW1wOiByZXF1ZXN0U3RhdHVzKFRSWUlOR19UT19TSUdOX1VQKSxcbiAgZmV0Y2hpbmdJbnZpdGU6IHJlcXVlc3RTdGF0dXMoRkVUQ0hJTkdfSU5WSVRFKVxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2dldHRlcnMuanNcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cy5nZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3Rpb25zID0gcmVxdWlyZSgnLi9hY3Rpb25zJyk7XG5tb2R1bGUuZXhwb3J0cy5ub2RlU3RvcmUgPSByZXF1aXJlKCcuL2ludml0ZVN0b3JlJyk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvaW5kZXguanNcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cy5nZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3Rpb25zID0gcmVxdWlyZSgnLi9hY3Rpb25zJyk7XG5tb2R1bGUuZXhwb3J0cy5ub2RlU3RvcmUgPSByZXF1aXJlKCcuL25vZGVTdG9yZScpO1xuXG4vLyBub2RlczogW3tcImlkXCI6XCJ4MjIwXCIsXCJhZGRyXCI6XCIwLjAuMC4wOjMwMjJcIixcImhvc3RuYW1lXCI6XCJ4MjIwXCIsXCJsYWJlbHNcIjpudWxsLFwiY21kX2xhYmVsc1wiOm51bGx9XVxuXG5cbi8vIHNlc3Npb25zOiBbe1wiaWRcIjpcIjA3NjMwNjM2LWJiM2QtNDBlMS1iMDg2LTYwYjJjYWUyMWFjNFwiLFwicGFydGllc1wiOlt7XCJpZFwiOlwiODlmNzYyYTMtNzQyOS00YzdhLWE5MTMtNzY2NDkzZmU3YzhhXCIsXCJzaXRlXCI6XCIxMjcuMC4wLjE6Mzc1MTRcIixcInVzZXJcIjpcImFrb250c2V2b3lcIixcInNlcnZlcl9hZGRyXCI6XCIwLjAuMC4wOjMwMjJcIixcImxhc3RfYWN0aXZlXCI6XCIyMDE2LTAyLTIyVDE0OjM5OjIwLjkzMTIwNTM1LTA1OjAwXCJ9XX1dXG5cbi8qXG5sZXQgVG9kb1JlY29yZCA9IEltbXV0YWJsZS5SZWNvcmQoe1xuICAgIGlkOiAwLFxuICAgIGRlc2NyaXB0aW9uOiBcIlwiLFxuICAgIGNvbXBsZXRlZDogZmFsc2Vcbn0pO1xuKi9cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2luZGV4LmpzXG4gKiovIiwidmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG52YXIge1xuICBUTFBUX1JFU1RfQVBJX1NUQVJULFxuICBUTFBUX1JFU1RfQVBJX1NVQ0NFU1MsXG4gIFRMUFRfUkVTVF9BUElfRkFJTCB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gdG9JbW11dGFibGUoe30pO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX1JFU1RfQVBJX1NUQVJULCBzdGFydCk7XG4gICAgdGhpcy5vbihUTFBUX1JFU1RfQVBJX0ZBSUwsIGZhaWwpO1xuICAgIHRoaXMub24oVExQVF9SRVNUX0FQSV9TVUNDRVNTLCBzdWNjZXNzKTtcbiAgfVxufSlcblxuZnVuY3Rpb24gc3RhcnQoc3RhdGUsIHJlcXVlc3Qpe1xuICByZXR1cm4gc3RhdGUuc2V0KHJlcXVlc3QudHlwZSwgdG9JbW11dGFibGUoe2lzUHJvY2Vzc2luZzogdHJ1ZX0pKTtcbn1cblxuZnVuY3Rpb24gZmFpbChzdGF0ZSwgcmVxdWVzdCl7XG4gIHJldHVybiBzdGF0ZS5zZXQocmVxdWVzdC50eXBlLCB0b0ltbXV0YWJsZSh7aXNGYWlsZWQ6IHRydWUsIG1lc3NhZ2U6IHJlcXVlc3QubWVzc2FnZX0pKTtcbn1cblxuZnVuY3Rpb24gc3VjY2VzcyhzdGF0ZSwgcmVxdWVzdCl7XG4gIHJldHVybiBzdGF0ZS5zZXQocmVxdWVzdC50eXBlLCB0b0ltbXV0YWJsZSh7aXNTdWNjZXNzOiB0cnVlfSkpO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9yZXN0QXBpU3RvcmUuanNcbiAqKi8iLCJ2YXIgdXRpbHMgPSB7XG5cbiAgdXVpZCgpe1xuICAgIC8vIG5ldmVyIHVzZSBpdCBpbiBwcm9kdWN0aW9uXG4gICAgcmV0dXJuICd4eHh4eHh4eC14eHh4LTR4eHgteXh4eC14eHh4eHh4eHh4eHgnLnJlcGxhY2UoL1t4eV0vZywgZnVuY3Rpb24oYykge1xuICAgICAgdmFyIHIgPSBNYXRoLnJhbmRvbSgpKjE2fDAsIHYgPSBjID09ICd4JyA/IHIgOiAociYweDN8MHg4KTtcbiAgICAgIHJldHVybiB2LnRvU3RyaW5nKDE2KTtcbiAgICB9KTtcbiAgfSxcblxuICBkaXNwbGF5RGF0ZShkYXRlKXtcbiAgICB0cnl7XG4gICAgICByZXR1cm4gZGF0ZS50b0xvY2FsZURhdGVTdHJpbmcoKSArICcgJyArIGRhdGUudG9Mb2NhbGVUaW1lU3RyaW5nKCk7XG4gICAgfWNhdGNoKGVycil7XG4gICAgICBjb25zb2xlLmVycm9yKGVycik7XG4gICAgICByZXR1cm4gJ3VuZGVmaW5lZCc7XG4gICAgfVxuICB9LFxuXG4gIGZvcm1hdFN0cmluZyhmb3JtYXQpIHtcbiAgICB2YXIgYXJncyA9IEFycmF5LnByb3RvdHlwZS5zbGljZS5jYWxsKGFyZ3VtZW50cywgMSk7XG4gICAgcmV0dXJuIGZvcm1hdC5yZXBsYWNlKG5ldyBSZWdFeHAoJ1xcXFx7KFxcXFxkKylcXFxcfScsICdnJyksXG4gICAgICAobWF0Y2gsIG51bWJlcikgPT4ge1xuICAgICAgICByZXR1cm4gIShhcmdzW251bWJlcl0gPT09IG51bGwgfHwgYXJnc1tudW1iZXJdID09PSB1bmRlZmluZWQpID8gYXJnc1tudW1iZXJdIDogJyc7XG4gICAgfSk7XG4gIH1cbiAgICAgICAgICAgIFxufVxuXG5tb2R1bGUuZXhwb3J0cyA9IHV0aWxzO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL3V0aWxzLmpzXG4gKiovIiwiLy8gQ29weXJpZ2h0IEpveWVudCwgSW5jLiBhbmQgb3RoZXIgTm9kZSBjb250cmlidXRvcnMuXG4vL1xuLy8gUGVybWlzc2lvbiBpcyBoZXJlYnkgZ3JhbnRlZCwgZnJlZSBvZiBjaGFyZ2UsIHRvIGFueSBwZXJzb24gb2J0YWluaW5nIGFcbi8vIGNvcHkgb2YgdGhpcyBzb2Z0d2FyZSBhbmQgYXNzb2NpYXRlZCBkb2N1bWVudGF0aW9uIGZpbGVzICh0aGVcbi8vIFwiU29mdHdhcmVcIiksIHRvIGRlYWwgaW4gdGhlIFNvZnR3YXJlIHdpdGhvdXQgcmVzdHJpY3Rpb24sIGluY2x1ZGluZ1xuLy8gd2l0aG91dCBsaW1pdGF0aW9uIHRoZSByaWdodHMgdG8gdXNlLCBjb3B5LCBtb2RpZnksIG1lcmdlLCBwdWJsaXNoLFxuLy8gZGlzdHJpYnV0ZSwgc3VibGljZW5zZSwgYW5kL29yIHNlbGwgY29waWVzIG9mIHRoZSBTb2Z0d2FyZSwgYW5kIHRvIHBlcm1pdFxuLy8gcGVyc29ucyB0byB3aG9tIHRoZSBTb2Z0d2FyZSBpcyBmdXJuaXNoZWQgdG8gZG8gc28sIHN1YmplY3QgdG8gdGhlXG4vLyBmb2xsb3dpbmcgY29uZGl0aW9uczpcbi8vXG4vLyBUaGUgYWJvdmUgY29weXJpZ2h0IG5vdGljZSBhbmQgdGhpcyBwZXJtaXNzaW9uIG5vdGljZSBzaGFsbCBiZSBpbmNsdWRlZFxuLy8gaW4gYWxsIGNvcGllcyBvciBzdWJzdGFudGlhbCBwb3J0aW9ucyBvZiB0aGUgU29mdHdhcmUuXG4vL1xuLy8gVEhFIFNPRlRXQVJFIElTIFBST1ZJREVEIFwiQVMgSVNcIiwgV0lUSE9VVCBXQVJSQU5UWSBPRiBBTlkgS0lORCwgRVhQUkVTU1xuLy8gT1IgSU1QTElFRCwgSU5DTFVESU5HIEJVVCBOT1QgTElNSVRFRCBUTyBUSEUgV0FSUkFOVElFUyBPRlxuLy8gTUVSQ0hBTlRBQklMSVRZLCBGSVRORVNTIEZPUiBBIFBBUlRJQ1VMQVIgUFVSUE9TRSBBTkQgTk9OSU5GUklOR0VNRU5ULiBJTlxuLy8gTk8gRVZFTlQgU0hBTEwgVEhFIEFVVEhPUlMgT1IgQ09QWVJJR0hUIEhPTERFUlMgQkUgTElBQkxFIEZPUiBBTlkgQ0xBSU0sXG4vLyBEQU1BR0VTIE9SIE9USEVSIExJQUJJTElUWSwgV0hFVEhFUiBJTiBBTiBBQ1RJT04gT0YgQ09OVFJBQ1QsIFRPUlQgT1Jcbi8vIE9USEVSV0lTRSwgQVJJU0lORyBGUk9NLCBPVVQgT0YgT1IgSU4gQ09OTkVDVElPTiBXSVRIIFRIRSBTT0ZUV0FSRSBPUiBUSEVcbi8vIFVTRSBPUiBPVEhFUiBERUFMSU5HUyBJTiBUSEUgU09GVFdBUkUuXG5cbmZ1bmN0aW9uIEV2ZW50RW1pdHRlcigpIHtcbiAgdGhpcy5fZXZlbnRzID0gdGhpcy5fZXZlbnRzIHx8IHt9O1xuICB0aGlzLl9tYXhMaXN0ZW5lcnMgPSB0aGlzLl9tYXhMaXN0ZW5lcnMgfHwgdW5kZWZpbmVkO1xufVxubW9kdWxlLmV4cG9ydHMgPSBFdmVudEVtaXR0ZXI7XG5cbi8vIEJhY2t3YXJkcy1jb21wYXQgd2l0aCBub2RlIDAuMTAueFxuRXZlbnRFbWl0dGVyLkV2ZW50RW1pdHRlciA9IEV2ZW50RW1pdHRlcjtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5fZXZlbnRzID0gdW5kZWZpbmVkO1xuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5fbWF4TGlzdGVuZXJzID0gdW5kZWZpbmVkO1xuXG4vLyBCeSBkZWZhdWx0IEV2ZW50RW1pdHRlcnMgd2lsbCBwcmludCBhIHdhcm5pbmcgaWYgbW9yZSB0aGFuIDEwIGxpc3RlbmVycyBhcmVcbi8vIGFkZGVkIHRvIGl0LiBUaGlzIGlzIGEgdXNlZnVsIGRlZmF1bHQgd2hpY2ggaGVscHMgZmluZGluZyBtZW1vcnkgbGVha3MuXG5FdmVudEVtaXR0ZXIuZGVmYXVsdE1heExpc3RlbmVycyA9IDEwO1xuXG4vLyBPYnZpb3VzbHkgbm90IGFsbCBFbWl0dGVycyBzaG91bGQgYmUgbGltaXRlZCB0byAxMC4gVGhpcyBmdW5jdGlvbiBhbGxvd3Ncbi8vIHRoYXQgdG8gYmUgaW5jcmVhc2VkLiBTZXQgdG8gemVybyBmb3IgdW5saW1pdGVkLlxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5zZXRNYXhMaXN0ZW5lcnMgPSBmdW5jdGlvbihuKSB7XG4gIGlmICghaXNOdW1iZXIobikgfHwgbiA8IDAgfHwgaXNOYU4obikpXG4gICAgdGhyb3cgVHlwZUVycm9yKCduIG11c3QgYmUgYSBwb3NpdGl2ZSBudW1iZXInKTtcbiAgdGhpcy5fbWF4TGlzdGVuZXJzID0gbjtcbiAgcmV0dXJuIHRoaXM7XG59O1xuXG5FdmVudEVtaXR0ZXIucHJvdG90eXBlLmVtaXQgPSBmdW5jdGlvbih0eXBlKSB7XG4gIHZhciBlciwgaGFuZGxlciwgbGVuLCBhcmdzLCBpLCBsaXN0ZW5lcnM7XG5cbiAgaWYgKCF0aGlzLl9ldmVudHMpXG4gICAgdGhpcy5fZXZlbnRzID0ge307XG5cbiAgLy8gSWYgdGhlcmUgaXMgbm8gJ2Vycm9yJyBldmVudCBsaXN0ZW5lciB0aGVuIHRocm93LlxuICBpZiAodHlwZSA9PT0gJ2Vycm9yJykge1xuICAgIGlmICghdGhpcy5fZXZlbnRzLmVycm9yIHx8XG4gICAgICAgIChpc09iamVjdCh0aGlzLl9ldmVudHMuZXJyb3IpICYmICF0aGlzLl9ldmVudHMuZXJyb3IubGVuZ3RoKSkge1xuICAgICAgZXIgPSBhcmd1bWVudHNbMV07XG4gICAgICBpZiAoZXIgaW5zdGFuY2VvZiBFcnJvcikge1xuICAgICAgICB0aHJvdyBlcjsgLy8gVW5oYW5kbGVkICdlcnJvcicgZXZlbnRcbiAgICAgIH1cbiAgICAgIHRocm93IFR5cGVFcnJvcignVW5jYXVnaHQsIHVuc3BlY2lmaWVkIFwiZXJyb3JcIiBldmVudC4nKTtcbiAgICB9XG4gIH1cblxuICBoYW5kbGVyID0gdGhpcy5fZXZlbnRzW3R5cGVdO1xuXG4gIGlmIChpc1VuZGVmaW5lZChoYW5kbGVyKSlcbiAgICByZXR1cm4gZmFsc2U7XG5cbiAgaWYgKGlzRnVuY3Rpb24oaGFuZGxlcikpIHtcbiAgICBzd2l0Y2ggKGFyZ3VtZW50cy5sZW5ndGgpIHtcbiAgICAgIC8vIGZhc3QgY2FzZXNcbiAgICAgIGNhc2UgMTpcbiAgICAgICAgaGFuZGxlci5jYWxsKHRoaXMpO1xuICAgICAgICBicmVhaztcbiAgICAgIGNhc2UgMjpcbiAgICAgICAgaGFuZGxlci5jYWxsKHRoaXMsIGFyZ3VtZW50c1sxXSk7XG4gICAgICAgIGJyZWFrO1xuICAgICAgY2FzZSAzOlxuICAgICAgICBoYW5kbGVyLmNhbGwodGhpcywgYXJndW1lbnRzWzFdLCBhcmd1bWVudHNbMl0pO1xuICAgICAgICBicmVhaztcbiAgICAgIC8vIHNsb3dlclxuICAgICAgZGVmYXVsdDpcbiAgICAgICAgbGVuID0gYXJndW1lbnRzLmxlbmd0aDtcbiAgICAgICAgYXJncyA9IG5ldyBBcnJheShsZW4gLSAxKTtcbiAgICAgICAgZm9yIChpID0gMTsgaSA8IGxlbjsgaSsrKVxuICAgICAgICAgIGFyZ3NbaSAtIDFdID0gYXJndW1lbnRzW2ldO1xuICAgICAgICBoYW5kbGVyLmFwcGx5KHRoaXMsIGFyZ3MpO1xuICAgIH1cbiAgfSBlbHNlIGlmIChpc09iamVjdChoYW5kbGVyKSkge1xuICAgIGxlbiA9IGFyZ3VtZW50cy5sZW5ndGg7XG4gICAgYXJncyA9IG5ldyBBcnJheShsZW4gLSAxKTtcbiAgICBmb3IgKGkgPSAxOyBpIDwgbGVuOyBpKyspXG4gICAgICBhcmdzW2kgLSAxXSA9IGFyZ3VtZW50c1tpXTtcblxuICAgIGxpc3RlbmVycyA9IGhhbmRsZXIuc2xpY2UoKTtcbiAgICBsZW4gPSBsaXN0ZW5lcnMubGVuZ3RoO1xuICAgIGZvciAoaSA9IDA7IGkgPCBsZW47IGkrKylcbiAgICAgIGxpc3RlbmVyc1tpXS5hcHBseSh0aGlzLCBhcmdzKTtcbiAgfVxuXG4gIHJldHVybiB0cnVlO1xufTtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5hZGRMaXN0ZW5lciA9IGZ1bmN0aW9uKHR5cGUsIGxpc3RlbmVyKSB7XG4gIHZhciBtO1xuXG4gIGlmICghaXNGdW5jdGlvbihsaXN0ZW5lcikpXG4gICAgdGhyb3cgVHlwZUVycm9yKCdsaXN0ZW5lciBtdXN0IGJlIGEgZnVuY3Rpb24nKTtcblxuICBpZiAoIXRoaXMuX2V2ZW50cylcbiAgICB0aGlzLl9ldmVudHMgPSB7fTtcblxuICAvLyBUbyBhdm9pZCByZWN1cnNpb24gaW4gdGhlIGNhc2UgdGhhdCB0eXBlID09PSBcIm5ld0xpc3RlbmVyXCIhIEJlZm9yZVxuICAvLyBhZGRpbmcgaXQgdG8gdGhlIGxpc3RlbmVycywgZmlyc3QgZW1pdCBcIm5ld0xpc3RlbmVyXCIuXG4gIGlmICh0aGlzLl9ldmVudHMubmV3TGlzdGVuZXIpXG4gICAgdGhpcy5lbWl0KCduZXdMaXN0ZW5lcicsIHR5cGUsXG4gICAgICAgICAgICAgIGlzRnVuY3Rpb24obGlzdGVuZXIubGlzdGVuZXIpID9cbiAgICAgICAgICAgICAgbGlzdGVuZXIubGlzdGVuZXIgOiBsaXN0ZW5lcik7XG5cbiAgaWYgKCF0aGlzLl9ldmVudHNbdHlwZV0pXG4gICAgLy8gT3B0aW1pemUgdGhlIGNhc2Ugb2Ygb25lIGxpc3RlbmVyLiBEb24ndCBuZWVkIHRoZSBleHRyYSBhcnJheSBvYmplY3QuXG4gICAgdGhpcy5fZXZlbnRzW3R5cGVdID0gbGlzdGVuZXI7XG4gIGVsc2UgaWYgKGlzT2JqZWN0KHRoaXMuX2V2ZW50c1t0eXBlXSkpXG4gICAgLy8gSWYgd2UndmUgYWxyZWFkeSBnb3QgYW4gYXJyYXksIGp1c3QgYXBwZW5kLlxuICAgIHRoaXMuX2V2ZW50c1t0eXBlXS5wdXNoKGxpc3RlbmVyKTtcbiAgZWxzZVxuICAgIC8vIEFkZGluZyB0aGUgc2Vjb25kIGVsZW1lbnQsIG5lZWQgdG8gY2hhbmdlIHRvIGFycmF5LlxuICAgIHRoaXMuX2V2ZW50c1t0eXBlXSA9IFt0aGlzLl9ldmVudHNbdHlwZV0sIGxpc3RlbmVyXTtcblxuICAvLyBDaGVjayBmb3IgbGlzdGVuZXIgbGVha1xuICBpZiAoaXNPYmplY3QodGhpcy5fZXZlbnRzW3R5cGVdKSAmJiAhdGhpcy5fZXZlbnRzW3R5cGVdLndhcm5lZCkge1xuICAgIHZhciBtO1xuICAgIGlmICghaXNVbmRlZmluZWQodGhpcy5fbWF4TGlzdGVuZXJzKSkge1xuICAgICAgbSA9IHRoaXMuX21heExpc3RlbmVycztcbiAgICB9IGVsc2Uge1xuICAgICAgbSA9IEV2ZW50RW1pdHRlci5kZWZhdWx0TWF4TGlzdGVuZXJzO1xuICAgIH1cblxuICAgIGlmIChtICYmIG0gPiAwICYmIHRoaXMuX2V2ZW50c1t0eXBlXS5sZW5ndGggPiBtKSB7XG4gICAgICB0aGlzLl9ldmVudHNbdHlwZV0ud2FybmVkID0gdHJ1ZTtcbiAgICAgIGNvbnNvbGUuZXJyb3IoJyhub2RlKSB3YXJuaW5nOiBwb3NzaWJsZSBFdmVudEVtaXR0ZXIgbWVtb3J5ICcgK1xuICAgICAgICAgICAgICAgICAgICAnbGVhayBkZXRlY3RlZC4gJWQgbGlzdGVuZXJzIGFkZGVkLiAnICtcbiAgICAgICAgICAgICAgICAgICAgJ1VzZSBlbWl0dGVyLnNldE1heExpc3RlbmVycygpIHRvIGluY3JlYXNlIGxpbWl0LicsXG4gICAgICAgICAgICAgICAgICAgIHRoaXMuX2V2ZW50c1t0eXBlXS5sZW5ndGgpO1xuICAgICAgaWYgKHR5cGVvZiBjb25zb2xlLnRyYWNlID09PSAnZnVuY3Rpb24nKSB7XG4gICAgICAgIC8vIG5vdCBzdXBwb3J0ZWQgaW4gSUUgMTBcbiAgICAgICAgY29uc29sZS50cmFjZSgpO1xuICAgICAgfVxuICAgIH1cbiAgfVxuXG4gIHJldHVybiB0aGlzO1xufTtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5vbiA9IEV2ZW50RW1pdHRlci5wcm90b3R5cGUuYWRkTGlzdGVuZXI7XG5cbkV2ZW50RW1pdHRlci5wcm90b3R5cGUub25jZSA9IGZ1bmN0aW9uKHR5cGUsIGxpc3RlbmVyKSB7XG4gIGlmICghaXNGdW5jdGlvbihsaXN0ZW5lcikpXG4gICAgdGhyb3cgVHlwZUVycm9yKCdsaXN0ZW5lciBtdXN0IGJlIGEgZnVuY3Rpb24nKTtcblxuICB2YXIgZmlyZWQgPSBmYWxzZTtcblxuICBmdW5jdGlvbiBnKCkge1xuICAgIHRoaXMucmVtb3ZlTGlzdGVuZXIodHlwZSwgZyk7XG5cbiAgICBpZiAoIWZpcmVkKSB7XG4gICAgICBmaXJlZCA9IHRydWU7XG4gICAgICBsaXN0ZW5lci5hcHBseSh0aGlzLCBhcmd1bWVudHMpO1xuICAgIH1cbiAgfVxuXG4gIGcubGlzdGVuZXIgPSBsaXN0ZW5lcjtcbiAgdGhpcy5vbih0eXBlLCBnKTtcblxuICByZXR1cm4gdGhpcztcbn07XG5cbi8vIGVtaXRzIGEgJ3JlbW92ZUxpc3RlbmVyJyBldmVudCBpZmYgdGhlIGxpc3RlbmVyIHdhcyByZW1vdmVkXG5FdmVudEVtaXR0ZXIucHJvdG90eXBlLnJlbW92ZUxpc3RlbmVyID0gZnVuY3Rpb24odHlwZSwgbGlzdGVuZXIpIHtcbiAgdmFyIGxpc3QsIHBvc2l0aW9uLCBsZW5ndGgsIGk7XG5cbiAgaWYgKCFpc0Z1bmN0aW9uKGxpc3RlbmVyKSlcbiAgICB0aHJvdyBUeXBlRXJyb3IoJ2xpc3RlbmVyIG11c3QgYmUgYSBmdW5jdGlvbicpO1xuXG4gIGlmICghdGhpcy5fZXZlbnRzIHx8ICF0aGlzLl9ldmVudHNbdHlwZV0pXG4gICAgcmV0dXJuIHRoaXM7XG5cbiAgbGlzdCA9IHRoaXMuX2V2ZW50c1t0eXBlXTtcbiAgbGVuZ3RoID0gbGlzdC5sZW5ndGg7XG4gIHBvc2l0aW9uID0gLTE7XG5cbiAgaWYgKGxpc3QgPT09IGxpc3RlbmVyIHx8XG4gICAgICAoaXNGdW5jdGlvbihsaXN0Lmxpc3RlbmVyKSAmJiBsaXN0Lmxpc3RlbmVyID09PSBsaXN0ZW5lcikpIHtcbiAgICBkZWxldGUgdGhpcy5fZXZlbnRzW3R5cGVdO1xuICAgIGlmICh0aGlzLl9ldmVudHMucmVtb3ZlTGlzdGVuZXIpXG4gICAgICB0aGlzLmVtaXQoJ3JlbW92ZUxpc3RlbmVyJywgdHlwZSwgbGlzdGVuZXIpO1xuXG4gIH0gZWxzZSBpZiAoaXNPYmplY3QobGlzdCkpIHtcbiAgICBmb3IgKGkgPSBsZW5ndGg7IGktLSA+IDA7KSB7XG4gICAgICBpZiAobGlzdFtpXSA9PT0gbGlzdGVuZXIgfHxcbiAgICAgICAgICAobGlzdFtpXS5saXN0ZW5lciAmJiBsaXN0W2ldLmxpc3RlbmVyID09PSBsaXN0ZW5lcikpIHtcbiAgICAgICAgcG9zaXRpb24gPSBpO1xuICAgICAgICBicmVhaztcbiAgICAgIH1cbiAgICB9XG5cbiAgICBpZiAocG9zaXRpb24gPCAwKVxuICAgICAgcmV0dXJuIHRoaXM7XG5cbiAgICBpZiAobGlzdC5sZW5ndGggPT09IDEpIHtcbiAgICAgIGxpc3QubGVuZ3RoID0gMDtcbiAgICAgIGRlbGV0ZSB0aGlzLl9ldmVudHNbdHlwZV07XG4gICAgfSBlbHNlIHtcbiAgICAgIGxpc3Quc3BsaWNlKHBvc2l0aW9uLCAxKTtcbiAgICB9XG5cbiAgICBpZiAodGhpcy5fZXZlbnRzLnJlbW92ZUxpc3RlbmVyKVxuICAgICAgdGhpcy5lbWl0KCdyZW1vdmVMaXN0ZW5lcicsIHR5cGUsIGxpc3RlbmVyKTtcbiAgfVxuXG4gIHJldHVybiB0aGlzO1xufTtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5yZW1vdmVBbGxMaXN0ZW5lcnMgPSBmdW5jdGlvbih0eXBlKSB7XG4gIHZhciBrZXksIGxpc3RlbmVycztcblxuICBpZiAoIXRoaXMuX2V2ZW50cylcbiAgICByZXR1cm4gdGhpcztcblxuICAvLyBub3QgbGlzdGVuaW5nIGZvciByZW1vdmVMaXN0ZW5lciwgbm8gbmVlZCB0byBlbWl0XG4gIGlmICghdGhpcy5fZXZlbnRzLnJlbW92ZUxpc3RlbmVyKSB7XG4gICAgaWYgKGFyZ3VtZW50cy5sZW5ndGggPT09IDApXG4gICAgICB0aGlzLl9ldmVudHMgPSB7fTtcbiAgICBlbHNlIGlmICh0aGlzLl9ldmVudHNbdHlwZV0pXG4gICAgICBkZWxldGUgdGhpcy5fZXZlbnRzW3R5cGVdO1xuICAgIHJldHVybiB0aGlzO1xuICB9XG5cbiAgLy8gZW1pdCByZW1vdmVMaXN0ZW5lciBmb3IgYWxsIGxpc3RlbmVycyBvbiBhbGwgZXZlbnRzXG4gIGlmIChhcmd1bWVudHMubGVuZ3RoID09PSAwKSB7XG4gICAgZm9yIChrZXkgaW4gdGhpcy5fZXZlbnRzKSB7XG4gICAgICBpZiAoa2V5ID09PSAncmVtb3ZlTGlzdGVuZXInKSBjb250aW51ZTtcbiAgICAgIHRoaXMucmVtb3ZlQWxsTGlzdGVuZXJzKGtleSk7XG4gICAgfVxuICAgIHRoaXMucmVtb3ZlQWxsTGlzdGVuZXJzKCdyZW1vdmVMaXN0ZW5lcicpO1xuICAgIHRoaXMuX2V2ZW50cyA9IHt9O1xuICAgIHJldHVybiB0aGlzO1xuICB9XG5cbiAgbGlzdGVuZXJzID0gdGhpcy5fZXZlbnRzW3R5cGVdO1xuXG4gIGlmIChpc0Z1bmN0aW9uKGxpc3RlbmVycykpIHtcbiAgICB0aGlzLnJlbW92ZUxpc3RlbmVyKHR5cGUsIGxpc3RlbmVycyk7XG4gIH0gZWxzZSB7XG4gICAgLy8gTElGTyBvcmRlclxuICAgIHdoaWxlIChsaXN0ZW5lcnMubGVuZ3RoKVxuICAgICAgdGhpcy5yZW1vdmVMaXN0ZW5lcih0eXBlLCBsaXN0ZW5lcnNbbGlzdGVuZXJzLmxlbmd0aCAtIDFdKTtcbiAgfVxuICBkZWxldGUgdGhpcy5fZXZlbnRzW3R5cGVdO1xuXG4gIHJldHVybiB0aGlzO1xufTtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5saXN0ZW5lcnMgPSBmdW5jdGlvbih0eXBlKSB7XG4gIHZhciByZXQ7XG4gIGlmICghdGhpcy5fZXZlbnRzIHx8ICF0aGlzLl9ldmVudHNbdHlwZV0pXG4gICAgcmV0ID0gW107XG4gIGVsc2UgaWYgKGlzRnVuY3Rpb24odGhpcy5fZXZlbnRzW3R5cGVdKSlcbiAgICByZXQgPSBbdGhpcy5fZXZlbnRzW3R5cGVdXTtcbiAgZWxzZVxuICAgIHJldCA9IHRoaXMuX2V2ZW50c1t0eXBlXS5zbGljZSgpO1xuICByZXR1cm4gcmV0O1xufTtcblxuRXZlbnRFbWl0dGVyLmxpc3RlbmVyQ291bnQgPSBmdW5jdGlvbihlbWl0dGVyLCB0eXBlKSB7XG4gIHZhciByZXQ7XG4gIGlmICghZW1pdHRlci5fZXZlbnRzIHx8ICFlbWl0dGVyLl9ldmVudHNbdHlwZV0pXG4gICAgcmV0ID0gMDtcbiAgZWxzZSBpZiAoaXNGdW5jdGlvbihlbWl0dGVyLl9ldmVudHNbdHlwZV0pKVxuICAgIHJldCA9IDE7XG4gIGVsc2VcbiAgICByZXQgPSBlbWl0dGVyLl9ldmVudHNbdHlwZV0ubGVuZ3RoO1xuICByZXR1cm4gcmV0O1xufTtcblxuZnVuY3Rpb24gaXNGdW5jdGlvbihhcmcpIHtcbiAgcmV0dXJuIHR5cGVvZiBhcmcgPT09ICdmdW5jdGlvbic7XG59XG5cbmZ1bmN0aW9uIGlzTnVtYmVyKGFyZykge1xuICByZXR1cm4gdHlwZW9mIGFyZyA9PT0gJ251bWJlcic7XG59XG5cbmZ1bmN0aW9uIGlzT2JqZWN0KGFyZykge1xuICByZXR1cm4gdHlwZW9mIGFyZyA9PT0gJ29iamVjdCcgJiYgYXJnICE9PSBudWxsO1xufVxuXG5mdW5jdGlvbiBpc1VuZGVmaW5lZChhcmcpIHtcbiAgcmV0dXJuIGFyZyA9PT0gdm9pZCAwO1xufVxuXG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiAuL34vZXZlbnRzL2V2ZW50cy5qc1xuICoqIG1vZHVsZSBpZCA9IDI4NlxuICoqIG1vZHVsZSBjaHVua3MgPSAxXG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciBOYXZMZWZ0QmFyID0gcmVxdWlyZSgnLi9uYXZMZWZ0QmFyJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHthY3Rpb25zLCBnZXR0ZXJzfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2FwcCcpO1xudmFyIFNlbGVjdE5vZGVEaWFsb2cgPSByZXF1aXJlKCcuL3NlbGVjdE5vZGVEaWFsb2cuanN4Jyk7XG5cbnZhciBBcHAgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIGFwcDogZ2V0dGVycy5hcHBTdGF0ZVxuICAgIH1cbiAgfSxcblxuICBjb21wb25lbnRXaWxsTW91bnQoKXtcbiAgICBhY3Rpb25zLmluaXRBcHAoKTtcbiAgICB0aGlzLnJlZnJlc2hJbnRlcnZhbCA9IHNldEludGVydmFsKGFjdGlvbnMuZmV0Y2hOb2Rlc0FuZFNlc3Npb25zLCAzNTAwMCk7XG4gIH0sXG5cbiAgY29tcG9uZW50V2lsbFVubW91bnQ6IGZ1bmN0aW9uKCkge1xuICAgIGNsZWFySW50ZXJ2YWwodGhpcy5yZWZyZXNoSW50ZXJ2YWwpO1xuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgaWYodGhpcy5zdGF0ZS5hcHAuaXNJbml0aWFsaXppbmcpe1xuICAgICAgcmV0dXJuIG51bGw7XG4gICAgfVxuXG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXRscHRcIj5cbiAgICAgICAgPE5hdkxlZnRCYXIvPlxuICAgICAgICA8U2VsZWN0Tm9kZURpYWxvZy8+XG4gICAgICAgIHt0aGlzLnByb3BzLkN1cnJlbnRTZXNzaW9uSG9zdH1cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJyb3dcIj5cbiAgICAgICAgICA8bmF2IGNsYXNzTmFtZT1cIlwiIHJvbGU9XCJuYXZpZ2F0aW9uXCIgc3R5bGU9e3sgbWFyZ2luQm90dG9tOiAwLCBmbG9hdDogXCJyaWdodFwiIH19PlxuICAgICAgICAgICAgPHVsIGNsYXNzTmFtZT1cIm5hdiBuYXZiYXItdG9wLWxpbmtzIG5hdmJhci1yaWdodFwiPlxuICAgICAgICAgICAgICA8bGk+XG4gICAgICAgICAgICAgICAgPGEgaHJlZj17Y2ZnLnJvdXRlcy5sb2dvdXR9PlxuICAgICAgICAgICAgICAgICAgPGkgY2xhc3NOYW1lPVwiZmEgZmEtc2lnbi1vdXRcIj48L2k+XG4gICAgICAgICAgICAgICAgICBMb2cgb3V0XG4gICAgICAgICAgICAgICAgPC9hPlxuICAgICAgICAgICAgICA8L2xpPlxuICAgICAgICAgICAgPC91bD5cbiAgICAgICAgICA8L25hdj5cbiAgICAgICAgPC9kaXY+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXBhZ2VcIj5cbiAgICAgICAgICB7dGhpcy5wcm9wcy5jaGlsZHJlbn1cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KVxuXG5tb2R1bGUuZXhwb3J0cyA9IEFwcDtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2FwcC5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHtnZXR0ZXJzLCBhY3Rpb25zfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsLycpO1xudmFyIFR0eSA9IHJlcXVpcmUoJ2FwcC9jb21tb24vdHR5Jyk7XG52YXIgVHR5VGVybWluYWwgPSByZXF1aXJlKCcuLy4uL3Rlcm1pbmFsLmpzeCcpO1xudmFyIEV2ZW50U3RyZWFtZXIgPSByZXF1aXJlKCcuL2V2ZW50U3RyZWFtZXIuanN4Jyk7XG52YXIgU2Vzc2lvbkxlZnRQYW5lbCA9IHJlcXVpcmUoJy4vc2Vzc2lvbkxlZnRQYW5lbCcpO1xudmFyIHtzaG93U2VsZWN0Tm9kZURpYWxvZywgY2xvc2VTZWxlY3ROb2RlRGlhbG9nfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2RpYWxvZ3MvYWN0aW9ucycpO1xudmFyIFNlbGVjdE5vZGVEaWFsb2cgPSByZXF1aXJlKCcuLy4uL3NlbGVjdE5vZGVEaWFsb2cuanN4Jyk7XG5cbnZhciBBY3RpdmVTZXNzaW9uID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50KCl7XG4gICAgY2xvc2VTZWxlY3ROb2RlRGlhbG9nKCk7XG4gIH0sXG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICBsZXQge3NlcnZlcklwLCBsb2dpbiwgcGFydGllc30gPSB0aGlzLnByb3BzLmFjdGl2ZVNlc3Npb247XG4gICAgbGV0IHNlcnZlckxhYmVsVGV4dCA9IGAke2xvZ2lufUAke3NlcnZlcklwfWA7XG5cbiAgICBpZighc2VydmVySXApe1xuICAgICAgc2VydmVyTGFiZWxUZXh0ID0gJyc7XG4gICAgfVxuXG4gICAgcmV0dXJuIChcbiAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtY3VycmVudC1zZXNzaW9uXCI+XG4gICAgICAgPFNlc3Npb25MZWZ0UGFuZWwgcGFydGllcz17cGFydGllc30vPlxuICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWN1cnJlbnQtc2Vzc2lvbi1zZXJ2ZXItaW5mb1wiPlxuICAgICAgICAgPHNwYW4gY2xhc3NOYW1lPVwiYnRuIGJ0bi1wcmltYXJ5IGJ0bi1zbVwiIG9uQ2xpY2s9e3Nob3dTZWxlY3ROb2RlRGlhbG9nfT5cbiAgICAgICAgICAgQ2hhbmdlIG5vZGVcbiAgICAgICAgIDwvc3Bhbj5cbiAgICAgICAgIDxoMz57c2VydmVyTGFiZWxUZXh0fTwvaDM+XG4gICAgICAgPC9kaXY+XG4gICAgICAgPFR0eUNvbm5lY3Rpb24gey4uLnRoaXMucHJvcHMuYWN0aXZlU2Vzc2lvbn0gLz5cbiAgICAgPC9kaXY+XG4gICAgICk7XG4gIH1cbn0pO1xuXG52YXIgVHR5Q29ubmVjdGlvbiA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgdGhpcy50dHkgPSBuZXcgVHR5KHRoaXMucHJvcHMpXG4gICAgdGhpcy50dHkub24oJ29wZW4nLCAoKT0+IHRoaXMuc2V0U3RhdGUoeyAuLi50aGlzLnN0YXRlLCBpc0Nvbm5lY3RlZDogdHJ1ZSB9KSk7XG5cbiAgICB2YXIge3NlcnZlcklkLCBsb2dpbn0gPSB0aGlzLnByb3BzO1xuICAgIHJldHVybiB7c2VydmVySWQsIGxvZ2luLCBpc0Nvbm5lY3RlZDogZmFsc2V9O1xuICB9LFxuXG4gIGNvbXBvbmVudERpZE1vdW50KCl7XG4gICAgLy8gdGVtcG9yYXJ5IGhhY2tcbiAgICBTZWxlY3ROb2RlRGlhbG9nLm9uU2VydmVyQ2hhbmdlQ2FsbEJhY2sgPSB0aGlzLmNvbXBvbmVudFdpbGxSZWNlaXZlUHJvcHMuYmluZCh0aGlzKTtcbiAgfSxcblxuICBjb21wb25lbnRXaWxsVW5tb3VudCgpIHtcbiAgICBTZWxlY3ROb2RlRGlhbG9nLm9uU2VydmVyQ2hhbmdlQ2FsbEJhY2sgPSBudWxsO1xuICAgIHRoaXMudHR5LmRpc2Nvbm5lY3QoKTtcbiAgfSxcblxuICBjb21wb25lbnRXaWxsUmVjZWl2ZVByb3BzKG5leHRQcm9wcyl7XG4gICAgdmFyIHtzZXJ2ZXJJZH0gPSBuZXh0UHJvcHM7XG4gICAgaWYoc2VydmVySWQgJiYgc2VydmVySWQgIT09IHRoaXMuc3RhdGUuc2VydmVySWQpe1xuICAgICAgdGhpcy50dHkucmVjb25uZWN0KHtzZXJ2ZXJJZH0pO1xuICAgICAgdGhpcy5yZWZzLnR0eUNtbnRJbnN0YW5jZS50ZXJtLmZvY3VzKCk7XG4gICAgICB0aGlzLnNldFN0YXRlKHsuLi50aGlzLnN0YXRlLCBzZXJ2ZXJJZCB9KTtcbiAgICB9XG4gIH0sXG5cbiAgcmVuZGVyKCkge1xuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IHN0eWxlPXt7aGVpZ2h0OiAnMTAwJSd9fT5cbiAgICAgICAgPFR0eVRlcm1pbmFsIHJlZj1cInR0eUNtbnRJbnN0YW5jZVwiIHR0eT17dGhpcy50dHl9IGNvbHM9e3RoaXMucHJvcHMuY29sc30gcm93cz17dGhpcy5wcm9wcy5yb3dzfSAvPlxuICAgICAgICB7IHRoaXMuc3RhdGUuaXNDb25uZWN0ZWQgPyA8RXZlbnRTdHJlYW1lciBzaWQ9e3RoaXMucHJvcHMuc2lkfS8+IDogbnVsbCB9XG4gICAgICA8L2Rpdj5cbiAgICApXG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IEFjdGl2ZVNlc3Npb247XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9jdXJyZW50U2Vzc2lvbi9hY3RpdmVTZXNzaW9uLmpzeFxuICoqLyIsInZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCdhcHAvc2Vzc2lvbicpO1xudmFyIHt1cGRhdGVTZXNzaW9ufSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3Nlc3Npb25zL2FjdGlvbnMnKTtcblxudmFyIEV2ZW50U3RyZWFtZXIgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIGNvbXBvbmVudERpZE1vdW50KCkge1xuICAgIGxldCB7c2lkfSA9IHRoaXMucHJvcHM7XG4gICAgbGV0IHt0b2tlbn0gPSBzZXNzaW9uLmdldFVzZXJEYXRhKCk7XG4gICAgbGV0IGNvbm5TdHIgPSBjZmcuYXBpLmdldEV2ZW50U3RyZWFtQ29ublN0cih0b2tlbiwgc2lkKTtcblxuICAgIHRoaXMuc29ja2V0ID0gbmV3IFdlYlNvY2tldChjb25uU3RyLCAncHJvdG8nKTtcbiAgICB0aGlzLnNvY2tldC5vbm1lc3NhZ2UgPSAoZXZlbnQpID0+IHtcbiAgICAgIHRyeVxuICAgICAge1xuICAgICAgICBsZXQganNvbiA9IEpTT04ucGFyc2UoZXZlbnQuZGF0YSk7XG4gICAgICAgIHVwZGF0ZVNlc3Npb24oanNvbi5zZXNzaW9uKTtcbiAgICAgIH1cbiAgICAgIGNhdGNoKGVycil7XG4gICAgICAgIGNvbnNvbGUubG9nKCdmYWlsZWQgdG8gcGFyc2UgZXZlbnQgc3RyZWFtIGRhdGEnKTtcbiAgICAgIH1cblxuICAgIH07XG4gICAgdGhpcy5zb2NrZXQub25jbG9zZSA9ICgpID0+IHt9O1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50KCkge1xuICAgIHRoaXMuc29ja2V0LmNsb3NlKCk7XG4gIH0sXG5cbiAgc2hvdWxkQ29tcG9uZW50VXBkYXRlKCkge1xuICAgIHJldHVybiBmYWxzZTtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuIG51bGw7XG4gIH1cbn0pO1xuXG5leHBvcnQgZGVmYXVsdCBFdmVudFN0cmVhbWVyO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvY3VycmVudFNlc3Npb24vZXZlbnRTdHJlYW1lci5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHtnZXR0ZXJzLCBhY3Rpb25zfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsLycpO1xudmFyIFNlc3Npb25QbGF5ZXIgPSByZXF1aXJlKCcuL3Nlc3Npb25QbGF5ZXIuanN4Jyk7XG52YXIgQWN0aXZlU2Vzc2lvbiA9IHJlcXVpcmUoJy4vYWN0aXZlU2Vzc2lvbi5qc3gnKTtcblxudmFyIEN1cnJlbnRTZXNzaW9uSG9zdCA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBtaXhpbnM6IFtyZWFjdG9yLlJlYWN0TWl4aW5dLFxuXG4gIGdldERhdGFCaW5kaW5ncygpIHtcbiAgICByZXR1cm4ge1xuICAgICAgY3VycmVudFNlc3Npb246IGdldHRlcnMuYWN0aXZlU2Vzc2lvblxuICAgIH1cbiAgfSxcblxuICBjb21wb25lbnREaWRNb3VudCgpe1xuICAgIHZhciB7IHNpZCB9ID0gdGhpcy5wcm9wcy5wYXJhbXM7XG4gICAgaWYoIXRoaXMuc3RhdGUuY3VycmVudFNlc3Npb24pe1xuICAgICAgYWN0aW9ucy5vcGVuU2Vzc2lvbihzaWQpO1xuICAgIH1cbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIHZhciBjdXJyZW50U2Vzc2lvbiA9IHRoaXMuc3RhdGUuY3VycmVudFNlc3Npb247XG4gICAgaWYoIWN1cnJlbnRTZXNzaW9uKXtcbiAgICAgIHJldHVybiBudWxsO1xuICAgIH1cblxuICAgIGlmKGN1cnJlbnRTZXNzaW9uLmlzTmV3U2Vzc2lvbiB8fCBjdXJyZW50U2Vzc2lvbi5hY3RpdmUpe1xuICAgICAgcmV0dXJuIDxBY3RpdmVTZXNzaW9uIGFjdGl2ZVNlc3Npb249e2N1cnJlbnRTZXNzaW9ufS8+O1xuICAgIH1cblxuICAgIHJldHVybiA8U2Vzc2lvblBsYXllciBhY3RpdmVTZXNzaW9uPXtjdXJyZW50U2Vzc2lvbn0vPjtcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gQ3VycmVudFNlc3Npb25Ib3N0O1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvY3VycmVudFNlc3Npb24vbWFpbi5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIFJlYWN0U2xpZGVyID0gcmVxdWlyZSgncmVhY3Qtc2xpZGVyJyk7XG52YXIgVHR5UGxheWVyID0gcmVxdWlyZSgnYXBwL2NvbW1vbi90dHlQbGF5ZXInKVxudmFyIFR0eVRlcm1pbmFsID0gcmVxdWlyZSgnLi8uLi90ZXJtaW5hbC5qc3gnKTtcbnZhciBTZXNzaW9uTGVmdFBhbmVsID0gcmVxdWlyZSgnLi9zZXNzaW9uTGVmdFBhbmVsJyk7XG5cbnZhciBTZXNzaW9uUGxheWVyID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICBjYWxjdWxhdGVTdGF0ZSgpe1xuICAgIHJldHVybiB7XG4gICAgICBsZW5ndGg6IHRoaXMudHR5Lmxlbmd0aCxcbiAgICAgIG1pbjogMSxcbiAgICAgIGlzUGxheWluZzogdGhpcy50dHkuaXNQbGF5aW5nLFxuICAgICAgY3VycmVudDogdGhpcy50dHkuY3VycmVudCxcbiAgICAgIGNhblBsYXk6IHRoaXMudHR5Lmxlbmd0aCA+IDFcbiAgICB9O1xuICB9LFxuXG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICB2YXIgc2lkID0gdGhpcy5wcm9wcy5hY3RpdmVTZXNzaW9uLnNpZDtcbiAgICB0aGlzLnR0eSA9IG5ldyBUdHlQbGF5ZXIoe3NpZH0pO1xuICAgIHJldHVybiB0aGlzLmNhbGN1bGF0ZVN0YXRlKCk7XG4gIH0sXG5cbiAgY29tcG9uZW50V2lsbFVubW91bnQoKSB7XG4gICAgdGhpcy50dHkuc3RvcCgpO1xuICAgIHRoaXMudHR5LnJlbW92ZUFsbExpc3RlbmVycygpO1xuICB9LFxuXG4gIGNvbXBvbmVudERpZE1vdW50KCkge1xuICAgIHRoaXMudHR5Lm9uKCdjaGFuZ2UnLCAoKT0+e1xuICAgICAgdmFyIG5ld1N0YXRlID0gdGhpcy5jYWxjdWxhdGVTdGF0ZSgpO1xuICAgICAgdGhpcy5zZXRTdGF0ZShuZXdTdGF0ZSk7XG4gICAgfSk7XG4gIH0sXG5cbiAgdG9nZ2xlUGxheVN0b3AoKXtcbiAgICBpZih0aGlzLnN0YXRlLmlzUGxheWluZyl7XG4gICAgICB0aGlzLnR0eS5zdG9wKCk7XG4gICAgfWVsc2V7XG4gICAgICB0aGlzLnR0eS5wbGF5KCk7XG4gICAgfVxuICB9LFxuXG4gIG1vdmUodmFsdWUpe1xuICAgIHRoaXMudHR5Lm1vdmUodmFsdWUpO1xuICB9LFxuXG4gIG9uQmVmb3JlQ2hhbmdlKCl7XG4gICAgdGhpcy50dHkuc3RvcCgpO1xuICB9LFxuXG4gIG9uQWZ0ZXJDaGFuZ2UodmFsdWUpe1xuICAgIHRoaXMudHR5LnBsYXkoKTtcbiAgICB0aGlzLnR0eS5tb3ZlKHZhbHVlKTtcbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIHZhciB7aXNQbGF5aW5nfSA9IHRoaXMuc3RhdGU7XG5cbiAgICByZXR1cm4gKFxuICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1jdXJyZW50LXNlc3Npb24gZ3J2LXNlc3Npb24tcGxheWVyXCI+XG4gICAgICAgPFNlc3Npb25MZWZ0UGFuZWwvPlxuICAgICAgIDxUdHlUZXJtaW5hbCByZWY9XCJ0ZXJtXCIgdHR5PXt0aGlzLnR0eX0gY29scz1cIjVcIiByb3dzPVwiNVwiIC8+XG4gICAgICAgPFJlYWN0U2xpZGVyXG4gICAgICAgICAgbWluPXt0aGlzLnN0YXRlLm1pbn1cbiAgICAgICAgICBtYXg9e3RoaXMuc3RhdGUubGVuZ3RofVxuICAgICAgICAgIHZhbHVlPXt0aGlzLnN0YXRlLmN1cnJlbnR9ICAgIFxuICAgICAgICAgIG9uQWZ0ZXJDaGFuZ2U9e3RoaXMub25BZnRlckNoYW5nZX1cbiAgICAgICAgICBvbkJlZm9yZUNoYW5nZT17dGhpcy5vbkJlZm9yZUNoYW5nZX1cbiAgICAgICAgICBkZWZhdWx0VmFsdWU9ezF9XG4gICAgICAgICAgd2l0aEJhcnNcbiAgICAgICAgICBjbGFzc05hbWU9XCJncnYtc2xpZGVyXCI+XG4gICAgICAgPC9SZWFjdFNsaWRlcj5cbiAgICAgICA8YnV0dG9uIGNsYXNzTmFtZT1cImJ0blwiIG9uQ2xpY2s9e3RoaXMudG9nZ2xlUGxheVN0b3B9PlxuICAgICAgICAgeyBpc1BsYXlpbmcgPyA8aSBjbGFzc05hbWU9XCJmYSBmYS1zdG9wXCI+PC9pPiA6ICA8aSBjbGFzc05hbWU9XCJmYSBmYS1wbGF5XCI+PC9pPiB9XG4gICAgICAgPC9idXR0b24+XG4gICAgIDwvZGl2PlxuICAgICApO1xuICB9XG59KTtcblxuZXhwb3J0IGRlZmF1bHQgU2Vzc2lvblBsYXllcjtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2N1cnJlbnRTZXNzaW9uL3Nlc3Npb25QbGF5ZXIuanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciAkID0gcmVxdWlyZSgnalF1ZXJ5Jyk7XG52YXIgbW9tZW50ID0gcmVxdWlyZSgnbW9tZW50Jyk7XG52YXIge2RlYm91bmNlfSA9IHJlcXVpcmUoJ18nKTtcblxudmFyIERhdGVSYW5nZVBpY2tlciA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBnZXREYXRlcygpe1xuICAgIHZhciBzdGFydERhdGUgPSAkKHRoaXMucmVmcy5kcFBpY2tlcjEpLmRhdGVwaWNrZXIoJ2dldERhdGUnKTtcbiAgICB2YXIgZW5kRGF0ZSA9ICQodGhpcy5yZWZzLmRwUGlja2VyMikuZGF0ZXBpY2tlcignZ2V0RGF0ZScpO1xuICAgIHJldHVybiBbc3RhcnREYXRlLCBlbmREYXRlXTtcbiAgfSxcblxuICBzZXREYXRlcyh7c3RhcnREYXRlLCBlbmREYXRlfSl7XG4gICAgJCh0aGlzLnJlZnMuZHBQaWNrZXIxKS5kYXRlcGlja2VyKCdzZXREYXRlJywgc3RhcnREYXRlKTtcbiAgICAkKHRoaXMucmVmcy5kcFBpY2tlcjIpLmRhdGVwaWNrZXIoJ3NldERhdGUnLCBlbmREYXRlKTtcbiAgfSxcblxuICBnZXREZWZhdWx0UHJvcHMoKSB7XG4gICAgIHJldHVybiB7XG4gICAgICAgc3RhcnREYXRlOiBtb21lbnQoKS5zdGFydE9mKCdtb250aCcpLnRvRGF0ZSgpLFxuICAgICAgIGVuZERhdGU6IG1vbWVudCgpLmVuZE9mKCdtb250aCcpLnRvRGF0ZSgpLFxuICAgICAgIG9uQ2hhbmdlOiAoKT0+e31cbiAgICAgfTtcbiAgIH0sXG5cbiAgY29tcG9uZW50V2lsbFVubW91bnQoKXtcbiAgICAkKHRoaXMucmVmcy5kcCkuZGF0ZXBpY2tlcignZGVzdHJveScpO1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxSZWNlaXZlUHJvcHMobmV3UHJvcHMpe1xuICAgIHZhciBbc3RhcnREYXRlLCBlbmREYXRlXSA9IHRoaXMuZ2V0RGF0ZXMoKTtcbiAgICBpZighKGlzU2FtZShzdGFydERhdGUsIG5ld1Byb3BzLnN0YXJ0RGF0ZSkgJiZcbiAgICAgICAgICBpc1NhbWUoZW5kRGF0ZSwgbmV3UHJvcHMuZW5kRGF0ZSkpKXtcbiAgICAgICAgdGhpcy5zZXREYXRlcyhuZXdQcm9wcyk7XG4gICAgICB9XG4gIH0sXG5cbiAgc2hvdWxkQ29tcG9uZW50VXBkYXRlKCl7XG4gICAgcmV0dXJuIGZhbHNlO1xuICB9LFxuXG4gIGNvbXBvbmVudERpZE1vdW50KCl7XG4gICAgdGhpcy5vbkNoYW5nZSA9IGRlYm91bmNlKHRoaXMub25DaGFuZ2UsIDEpO1xuICAgICQodGhpcy5yZWZzLnJhbmdlUGlja2VyKS5kYXRlcGlja2VyKHtcbiAgICAgIHRvZGF5QnRuOiAnbGlua2VkJyxcbiAgICAgIGtleWJvYXJkTmF2aWdhdGlvbjogZmFsc2UsXG4gICAgICBmb3JjZVBhcnNlOiBmYWxzZSxcbiAgICAgIGNhbGVuZGFyV2Vla3M6IHRydWUsXG4gICAgICBhdXRvY2xvc2U6IHRydWVcbiAgICB9KS5vbignY2hhbmdlRGF0ZScsIHRoaXMub25DaGFuZ2UpO1xuXG4gICAgdGhpcy5zZXREYXRlcyh0aGlzLnByb3BzKTtcbiAgfSxcblxuICBvbkNoYW5nZSgpe1xuICAgIHZhciBbc3RhcnREYXRlLCBlbmREYXRlXSA9IHRoaXMuZ2V0RGF0ZXMoKVxuICAgIGlmKCEoaXNTYW1lKHN0YXJ0RGF0ZSwgdGhpcy5wcm9wcy5zdGFydERhdGUpICYmXG4gICAgICAgICAgaXNTYW1lKGVuZERhdGUsIHRoaXMucHJvcHMuZW5kRGF0ZSkpKXtcbiAgICAgICAgdGhpcy5wcm9wcy5vbkNoYW5nZSh7c3RhcnREYXRlLCBlbmREYXRlfSk7XG4gICAgfVxuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZGF0ZXBpY2tlciBpbnB1dC1ncm91cCBpbnB1dC1kYXRlcmFuZ2VcIiByZWY9XCJyYW5nZVBpY2tlclwiPlxuICAgICAgICA8c3BhbiBjbGFzc05hbWU9XCJpbnB1dC1ncm91cC1hZGRvblwiPlxuICAgICAgICAgIDxpIGNsYXNzTmFtZT1cImZhIGZhLWNhbGVuZGFyXCI+PC9pPlxuICAgICAgICA8L3NwYW4+XG4gICAgICAgIDxpbnB1dCByZWY9XCJkcFBpY2tlcjFcIiB0eXBlPVwidGV4dFwiIGNsYXNzTmFtZT1cImlucHV0LXNtIGZvcm0tY29udHJvbFwiIG5hbWU9XCJzdGFydFwiIC8+XG4gICAgICAgIDxzcGFuIGNsYXNzTmFtZT1cImlucHV0LWdyb3VwLWFkZG9uXCI+dG88L3NwYW4+XG4gICAgICAgIDxpbnB1dCByZWY9XCJkcFBpY2tlcjJcIiB0eXBlPVwidGV4dFwiIGNsYXNzTmFtZT1cImlucHV0LXNtIGZvcm0tY29udHJvbFwiIG5hbWU9XCJlbmRcIiAvPlxuICAgICAgPC9kaXY+XG4gICAgKTtcbiAgfVxufSk7XG5cbmZ1bmN0aW9uIGlzU2FtZShkYXRlMSwgZGF0ZTIpe1xuICByZXR1cm4gbW9tZW50KGRhdGUxKS5pc1NhbWUoZGF0ZTIsICdkYXknKTtcbn1cblxuLyoqXG4qIENhbGVuZGFyIE5hdlxuKi9cbnZhciBDYWxlbmRhck5hdiA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICByZW5kZXIoKSB7XG4gICAgbGV0IHt2YWx1ZX0gPSB0aGlzLnByb3BzO1xuICAgIGxldCBkaXNwbGF5VmFsdWUgPSBtb21lbnQodmFsdWUpLmZvcm1hdCgnTU1NTSwgWVlZWScpO1xuXG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPXtcImdydi1jYWxlbmRhci1uYXYgXCIgKyB0aGlzLnByb3BzLmNsYXNzTmFtZX0gPlxuICAgICAgICA8YnV0dG9uIG9uQ2xpY2s9e3RoaXMubW92ZS5iaW5kKHRoaXMsIC0xKX0gY2xhc3NOYW1lPVwiYnRuIGJ0bi1vdXRsaW5lIGJ0bi1saW5rXCI+PGkgY2xhc3NOYW1lPVwiZmEgZmEtY2hldnJvbi1sZWZ0XCI+PC9pPjwvYnV0dG9uPlxuICAgICAgICA8c3BhbiBjbGFzc05hbWU9XCJ0ZXh0LW11dGVkXCI+e2Rpc3BsYXlWYWx1ZX08L3NwYW4+XG4gICAgICAgIDxidXR0b24gb25DbGljaz17dGhpcy5tb3ZlLmJpbmQodGhpcywgMSl9IGNsYXNzTmFtZT1cImJ0biBidG4tb3V0bGluZSBidG4tbGlua1wiPjxpIGNsYXNzTmFtZT1cImZhIGZhLWNoZXZyb24tcmlnaHRcIj48L2k+PC9idXR0b24+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9LFxuXG4gIG1vdmUoYXQpe1xuICAgIGxldCB7dmFsdWV9ID0gdGhpcy5wcm9wcztcbiAgICBsZXQgbmV3VmFsdWUgPSBtb21lbnQodmFsdWUpLmFkZChhdCwgJ21vbnRoJykudG9EYXRlKCk7XG4gICAgdGhpcy5wcm9wcy5vblZhbHVlQ2hhbmdlKG5ld1ZhbHVlKTtcbiAgfVxufSk7XG5cbkNhbGVuZGFyTmF2LmdldE1vbnRoUmFuZ2UgPSBmdW5jdGlvbih2YWx1ZSl7XG4gIGxldCBzdGFydERhdGUgPSBtb21lbnQodmFsdWUpLnN0YXJ0T2YoJ21vbnRoJykudG9EYXRlKCk7XG4gIGxldCBlbmREYXRlID0gbW9tZW50KHZhbHVlKS5lbmRPZignbW9udGgnKS50b0RhdGUoKTtcbiAgcmV0dXJuIFtzdGFydERhdGUsIGVuZERhdGVdO1xufVxuXG5leHBvcnQgZGVmYXVsdCBEYXRlUmFuZ2VQaWNrZXI7XG5leHBvcnQge0NhbGVuZGFyTmF2LCBEYXRlUmFuZ2VQaWNrZXJ9O1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvZGF0ZVBpY2tlci5qc3hcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cy5BcHAgPSByZXF1aXJlKCcuL2FwcC5qc3gnKTtcbm1vZHVsZS5leHBvcnRzLkxvZ2luID0gcmVxdWlyZSgnLi9sb2dpbi5qc3gnKTtcbm1vZHVsZS5leHBvcnRzLk5ld1VzZXIgPSByZXF1aXJlKCcuL25ld1VzZXIuanN4Jyk7XG5tb2R1bGUuZXhwb3J0cy5Ob2RlcyA9IHJlcXVpcmUoJy4vbm9kZXMvbWFpbi5qc3gnKTtcbm1vZHVsZS5leHBvcnRzLlNlc3Npb25zID0gcmVxdWlyZSgnLi9zZXNzaW9ucy9tYWluLmpzeCcpO1xubW9kdWxlLmV4cG9ydHMuQ3VycmVudFNlc3Npb25Ib3N0ID0gcmVxdWlyZSgnLi9jdXJyZW50U2Vzc2lvbi9tYWluLmpzeCcpO1xubW9kdWxlLmV4cG9ydHMuTm90Rm91bmQgPSByZXF1aXJlKCcuL2Vycm9yUGFnZS5qc3gnKS5Ob3RGb3VuZDtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2luZGV4LmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgJCA9IHJlcXVpcmUoJ2pRdWVyeScpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIExpbmtlZFN0YXRlTWl4aW4gPSByZXF1aXJlKCdyZWFjdC1hZGRvbnMtbGlua2VkLXN0YXRlLW1peGluJyk7XG52YXIge2FjdGlvbnMsIGdldHRlcnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvdXNlcicpO1xudmFyIEdvb2dsZUF1dGhJbmZvID0gcmVxdWlyZSgnLi9nb29nbGVBdXRoTG9nbycpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxudmFyIExvZ2luSW5wdXRGb3JtID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW0xpbmtlZFN0YXRlTWl4aW5dLFxuXG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4ge1xuICAgICAgdXNlcjogJycsXG4gICAgICBwYXNzd29yZDogJycsXG4gICAgICB0b2tlbjogJydcbiAgICB9XG4gIH0sXG5cbiAgb25DbGljazogZnVuY3Rpb24oZSkge1xuICAgIGUucHJldmVudERlZmF1bHQoKTtcbiAgICBpZiAodGhpcy5pc1ZhbGlkKCkpIHtcbiAgICAgIHRoaXMucHJvcHMub25DbGljayh0aGlzLnN0YXRlKTtcbiAgICB9XG4gIH0sXG5cbiAgaXNWYWxpZDogZnVuY3Rpb24oKSB7XG4gICAgdmFyICRmb3JtID0gJCh0aGlzLnJlZnMuZm9ybSk7XG4gICAgcmV0dXJuICRmb3JtLmxlbmd0aCA9PT0gMCB8fCAkZm9ybS52YWxpZCgpO1xuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICBsZXQge2lzUHJvY2Vzc2luZywgaXNGYWlsZWQsIG1lc3NhZ2UgfSA9IHRoaXMucHJvcHMuYXR0ZW1wO1xuXG4gICAgcmV0dXJuIChcbiAgICAgIDxmb3JtIHJlZj1cImZvcm1cIiBjbGFzc05hbWU9XCJncnYtbG9naW4taW5wdXQtZm9ybVwiPlxuICAgICAgICA8aDM+IFdlbGNvbWUgdG8gVGVsZXBvcnQgPC9oMz5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImZvcm0tZ3JvdXBcIj5cbiAgICAgICAgICAgIDxpbnB1dCBhdXRvRm9jdXMgdmFsdWVMaW5rPXt0aGlzLmxpbmtTdGF0ZSgndXNlcicpfSBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2wgcmVxdWlyZWRcIiBwbGFjZWhvbGRlcj1cIlVzZXIgbmFtZVwiIG5hbWU9XCJ1c2VyTmFtZVwiIC8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXQgdmFsdWVMaW5rPXt0aGlzLmxpbmtTdGF0ZSgncGFzc3dvcmQnKX0gdHlwZT1cInBhc3N3b3JkXCIgbmFtZT1cInBhc3N3b3JkXCIgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sIHJlcXVpcmVkXCIgcGxhY2Vob2xkZXI9XCJQYXNzd29yZFwiLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImZvcm0tZ3JvdXBcIj5cbiAgICAgICAgICAgIDxpbnB1dCB2YWx1ZUxpbms9e3RoaXMubGlua1N0YXRlKCd0b2tlbicpfSBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2wgcmVxdWlyZWRcIiBuYW1lPVwidG9rZW5cIiBwbGFjZWhvbGRlcj1cIlR3byBmYWN0b3IgdG9rZW4gKEdvb2dsZSBBdXRoZW50aWNhdG9yKVwiLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8YnV0dG9uIG9uQ2xpY2s9e3RoaXMub25DbGlja30gZGlzYWJsZWQ9e2lzUHJvY2Vzc2luZ30gdHlwZT1cInN1Ym1pdFwiIGNsYXNzTmFtZT1cImJ0biBidG4tcHJpbWFyeSBibG9jayBmdWxsLXdpZHRoIG0tYlwiPkxvZ2luPC9idXR0b24+XG4gICAgICAgICAgeyBpc0ZhaWxlZCA/ICg8bGFiZWwgY2xhc3NOYW1lPVwiZXJyb3JcIj57bWVzc2FnZX08L2xhYmVsPikgOiBudWxsIH1cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Zvcm0+XG4gICAgKTtcbiAgfVxufSlcblxudmFyIExvZ2luID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW3JlYWN0b3IuUmVhY3RNaXhpbl0sXG5cbiAgZ2V0RGF0YUJpbmRpbmdzKCkge1xuICAgIHJldHVybiB7XG4gICAgICBhdHRlbXA6IGdldHRlcnMubG9naW5BdHRlbXBcbiAgICB9XG4gIH0sXG5cbiAgb25DbGljayhpbnB1dERhdGEpe1xuICAgIHZhciBsb2MgPSB0aGlzLnByb3BzLmxvY2F0aW9uO1xuICAgIHZhciByZWRpcmVjdCA9IGNmZy5yb3V0ZXMuYXBwO1xuXG4gICAgaWYobG9jLnN0YXRlICYmIGxvYy5zdGF0ZS5yZWRpcmVjdFRvKXtcbiAgICAgIHJlZGlyZWN0ID0gbG9jLnN0YXRlLnJlZGlyZWN0VG87XG4gICAgfVxuXG4gICAgYWN0aW9ucy5sb2dpbihpbnB1dERhdGEsIHJlZGlyZWN0KTtcbiAgfSxcblxuICByZW5kZXIoKSB7ICAgIFxuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1sb2dpbiB0ZXh0LWNlbnRlclwiPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1sb2dvLXRwcnRcIj48L2Rpdj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtY29udGVudCBncnYtZmxleFwiPlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWZsZXgtY29sdW1uXCI+XG4gICAgICAgICAgICA8TG9naW5JbnB1dEZvcm0gYXR0ZW1wPXt0aGlzLnN0YXRlLmF0dGVtcH0gb25DbGljaz17dGhpcy5vbkNsaWNrfS8+XG4gICAgICAgICAgICA8R29vZ2xlQXV0aEluZm8vPlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbG9naW4taW5mb1wiPlxuICAgICAgICAgICAgICA8aSBjbGFzc05hbWU9XCJmYSBmYS1xdWVzdGlvblwiPjwvaT5cbiAgICAgICAgICAgICAgPHN0cm9uZz5OZXcgQWNjb3VudCBvciBmb3Jnb3QgcGFzc3dvcmQ/PC9zdHJvbmc+XG4gICAgICAgICAgICAgIDxkaXY+QXNrIGZvciBhc3Npc3RhbmNlIGZyb20geW91ciBDb21wYW55IGFkbWluaXN0cmF0b3I8L2Rpdj5cbiAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IExvZ2luO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvbG9naW4uanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciB7IFJvdXRlciwgSW5kZXhMaW5rLCBIaXN0b3J5IH0gPSByZXF1aXJlKCdyZWFjdC1yb3V0ZXInKTtcbnZhciBnZXR0ZXJzID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvdXNlci9nZXR0ZXJzJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xuXG52YXIgbWVudUl0ZW1zID0gW1xuICB7aWNvbjogJ2ZhIGZhLWNvZ3MnLCB0bzogY2ZnLnJvdXRlcy5ub2RlcywgdGl0bGU6ICdOb2Rlcyd9LFxuICB7aWNvbjogJ2ZhIGZhLXNpdGVtYXAnLCB0bzogY2ZnLnJvdXRlcy5zZXNzaW9ucywgdGl0bGU6ICdTZXNzaW9ucyd9XG5dO1xuXG52YXIgTmF2TGVmdEJhciA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICByZW5kZXI6IGZ1bmN0aW9uKCl7XG4gICAgdmFyIGl0ZW1zID0gbWVudUl0ZW1zLm1hcCgoaSwgaW5kZXgpPT57XG4gICAgICB2YXIgY2xhc3NOYW1lID0gdGhpcy5jb250ZXh0LnJvdXRlci5pc0FjdGl2ZShpLnRvKSA/ICdhY3RpdmUnIDogJyc7XG4gICAgICByZXR1cm4gKFxuICAgICAgICA8bGkga2V5PXtpbmRleH0gY2xhc3NOYW1lPXtjbGFzc05hbWV9PlxuICAgICAgICAgIDxJbmRleExpbmsgdG89e2kudG99PlxuICAgICAgICAgICAgPGkgY2xhc3NOYW1lPXtpLmljb259IHRpdGxlPXtpLnRpdGxlfS8+XG4gICAgICAgICAgPC9JbmRleExpbms+XG4gICAgICAgIDwvbGk+XG4gICAgICApO1xuICAgIH0pO1xuXG4gICAgaXRlbXMucHVzaCgoXG4gICAgICA8bGkga2V5PXttZW51SXRlbXMubGVuZ3RofT5cbiAgICAgICAgPGEgaHJlZj17Y2ZnLmhlbHBVcmx9PlxuICAgICAgICAgIDxpIGNsYXNzTmFtZT1cImZhIGZhLXF1ZXN0aW9uXCIgdGl0bGU9XCJoZWxwXCIvPlxuICAgICAgICA8L2E+XG4gICAgICA8L2xpPikpO1xuXG4gICAgcmV0dXJuIChcbiAgICAgIDxuYXYgY2xhc3NOYW1lPSdncnYtbmF2IG5hdmJhci1kZWZhdWx0IG5hdmJhci1zdGF0aWMtc2lkZScgcm9sZT0nbmF2aWdhdGlvbic+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPScnPlxuICAgICAgICAgIDx1bCBjbGFzc05hbWU9J25hdicgaWQ9J3NpZGUtbWVudSc+XG4gICAgICAgICAgICA8bGk+PGRpdiBjbGFzc05hbWU9XCJncnYtY2lyY2xlIHRleHQtdXBwZXJjYXNlXCI+PHNwYW4+e2dldFVzZXJOYW1lTGV0dGVyKCl9PC9zcGFuPjwvZGl2PjwvbGk+XG4gICAgICAgICAgICB7aXRlbXN9XG4gICAgICAgICAgPC91bD5cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L25hdj5cbiAgICApO1xuICB9XG59KTtcblxuTmF2TGVmdEJhci5jb250ZXh0VHlwZXMgPSB7XG4gIHJvdXRlcjogUmVhY3QuUHJvcFR5cGVzLm9iamVjdC5pc1JlcXVpcmVkXG59XG5cbmZ1bmN0aW9uIGdldFVzZXJOYW1lTGV0dGVyKCl7XG4gIHZhciB7c2hvcnREaXNwbGF5TmFtZX0gPSByZWFjdG9yLmV2YWx1YXRlKGdldHRlcnMudXNlcik7XG4gIHJldHVybiBzaG9ydERpc3BsYXlOYW1lO1xufVxuXG5tb2R1bGUuZXhwb3J0cyA9IE5hdkxlZnRCYXI7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9uYXZMZWZ0QmFyLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgJCA9IHJlcXVpcmUoJ2pRdWVyeScpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHthY3Rpb25zLCBnZXR0ZXJzfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2ludml0ZScpO1xudmFyIHVzZXJNb2R1bGUgPSByZXF1aXJlKCdhcHAvbW9kdWxlcy91c2VyJyk7XG52YXIgTGlua2VkU3RhdGVNaXhpbiA9IHJlcXVpcmUoJ3JlYWN0LWFkZG9ucy1saW5rZWQtc3RhdGUtbWl4aW4nKTtcbnZhciBHb29nbGVBdXRoSW5mbyA9IHJlcXVpcmUoJy4vZ29vZ2xlQXV0aExvZ28nKTtcbnZhciB7RXhwaXJlZEludml0ZX0gPSByZXF1aXJlKCcuL2Vycm9yUGFnZScpO1xuXG52YXIgSW52aXRlSW5wdXRGb3JtID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW0xpbmtlZFN0YXRlTWl4aW5dLFxuXG4gIGNvbXBvbmVudERpZE1vdW50KCl7XG4gICAgJCh0aGlzLnJlZnMuZm9ybSkudmFsaWRhdGUoe1xuICAgICAgcnVsZXM6e1xuICAgICAgICBwYXNzd29yZDp7XG4gICAgICAgICAgbWlubGVuZ3RoOiA2LFxuICAgICAgICAgIHJlcXVpcmVkOiB0cnVlXG4gICAgICAgIH0sXG4gICAgICAgIHBhc3N3b3JkQ29uZmlybWVkOntcbiAgICAgICAgICByZXF1aXJlZDogdHJ1ZSxcbiAgICAgICAgICBlcXVhbFRvOiB0aGlzLnJlZnMucGFzc3dvcmRcbiAgICAgICAgfVxuICAgICAgfSxcblxuICAgICAgbWVzc2FnZXM6IHtcbiAgXHRcdFx0cGFzc3dvcmRDb25maXJtZWQ6IHtcbiAgXHRcdFx0XHRtaW5sZW5ndGg6ICQudmFsaWRhdG9yLmZvcm1hdCgnRW50ZXIgYXQgbGVhc3QgezB9IGNoYXJhY3RlcnMnKSxcbiAgXHRcdFx0XHRlcXVhbFRvOiAnRW50ZXIgdGhlIHNhbWUgcGFzc3dvcmQgYXMgYWJvdmUnXG4gIFx0XHRcdH1cbiAgICAgIH1cbiAgICB9KVxuICB9LFxuXG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4ge1xuICAgICAgbmFtZTogdGhpcy5wcm9wcy5pbnZpdGUudXNlcixcbiAgICAgIHBzdzogJycsXG4gICAgICBwc3dDb25maXJtZWQ6ICcnLFxuICAgICAgdG9rZW46ICcnXG4gICAgfVxuICB9LFxuXG4gIG9uQ2xpY2soZSkge1xuICAgIGUucHJldmVudERlZmF1bHQoKTtcbiAgICBpZiAodGhpcy5pc1ZhbGlkKCkpIHtcbiAgICAgIHVzZXJNb2R1bGUuYWN0aW9ucy5zaWduVXAoe1xuICAgICAgICBuYW1lOiB0aGlzLnN0YXRlLm5hbWUsXG4gICAgICAgIHBzdzogdGhpcy5zdGF0ZS5wc3csXG4gICAgICAgIHRva2VuOiB0aGlzLnN0YXRlLnRva2VuLFxuICAgICAgICBpbnZpdGVUb2tlbjogdGhpcy5wcm9wcy5pbnZpdGUuaW52aXRlX3Rva2VufSk7XG4gICAgfVxuICB9LFxuXG4gIGlzVmFsaWQoKSB7XG4gICAgdmFyICRmb3JtID0gJCh0aGlzLnJlZnMuZm9ybSk7XG4gICAgcmV0dXJuICRmb3JtLmxlbmd0aCA9PT0gMCB8fCAkZm9ybS52YWxpZCgpO1xuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICBsZXQge2lzUHJvY2Vzc2luZywgaXNGYWlsZWQsIG1lc3NhZ2UgfSA9IHRoaXMucHJvcHMuYXR0ZW1wO1xuICAgIHJldHVybiAoXG4gICAgICA8Zm9ybSByZWY9XCJmb3JtXCIgY2xhc3NOYW1lPVwiZ3J2LWludml0ZS1pbnB1dC1mb3JtXCI+XG4gICAgICAgIDxoMz4gR2V0IHN0YXJ0ZWQgd2l0aCBUZWxlcG9ydCA8L2gzPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0XG4gICAgICAgICAgICAgIHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ25hbWUnKX1cbiAgICAgICAgICAgICAgbmFtZT1cInVzZXJOYW1lXCJcbiAgICAgICAgICAgICAgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sIHJlcXVpcmVkXCJcbiAgICAgICAgICAgICAgcGxhY2Vob2xkZXI9XCJVc2VyIG5hbWVcIi8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXRcbiAgICAgICAgICAgICAgYXV0b0ZvY3VzXG4gICAgICAgICAgICAgIHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3BzdycpfVxuICAgICAgICAgICAgICByZWY9XCJwYXNzd29yZFwiXG4gICAgICAgICAgICAgIHR5cGU9XCJwYXNzd29yZFwiXG4gICAgICAgICAgICAgIG5hbWU9XCJwYXNzd29yZFwiXG4gICAgICAgICAgICAgIGNsYXNzTmFtZT1cImZvcm0tY29udHJvbFwiXG4gICAgICAgICAgICAgIHBsYWNlaG9sZGVyPVwiUGFzc3dvcmRcIiAvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0XG4gICAgICAgICAgICAgIHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3Bzd0NvbmZpcm1lZCcpfVxuICAgICAgICAgICAgICB0eXBlPVwicGFzc3dvcmRcIlxuICAgICAgICAgICAgICBuYW1lPVwicGFzc3dvcmRDb25maXJtZWRcIlxuICAgICAgICAgICAgICBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2xcIlxuICAgICAgICAgICAgICBwbGFjZWhvbGRlcj1cIlBhc3N3b3JkIGNvbmZpcm1cIi8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXRcbiAgICAgICAgICAgICAgbmFtZT1cInRva2VuXCJcbiAgICAgICAgICAgICAgdmFsdWVMaW5rPXt0aGlzLmxpbmtTdGF0ZSgndG9rZW4nKX1cbiAgICAgICAgICAgICAgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sIHJlcXVpcmVkXCJcbiAgICAgICAgICAgICAgcGxhY2Vob2xkZXI9XCJUd28gZmFjdG9yIHRva2VuIChHb29nbGUgQXV0aGVudGljYXRvcilcIiAvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxidXR0b24gdHlwZT1cInN1Ym1pdFwiIGRpc2FibGVkPXtpc1Byb2Nlc3Npbmd9IGNsYXNzTmFtZT1cImJ0biBidG4tcHJpbWFyeSBibG9jayBmdWxsLXdpZHRoIG0tYlwiIG9uQ2xpY2s9e3RoaXMub25DbGlja30gPlNpZ24gdXA8L2J1dHRvbj5cbiAgICAgICAgICB7IGlzRmFpbGVkID8gKDxsYWJlbCBjbGFzc05hbWU9XCJlcnJvclwiPnttZXNzYWdlfTwvbGFiZWw+KSA6IG51bGwgfVxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZm9ybT5cbiAgICApO1xuICB9XG59KVxuXG52YXIgSW52aXRlID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW3JlYWN0b3IuUmVhY3RNaXhpbl0sXG5cbiAgZ2V0RGF0YUJpbmRpbmdzKCkge1xuICAgIHJldHVybiB7XG4gICAgICBpbnZpdGU6IGdldHRlcnMuaW52aXRlLFxuICAgICAgYXR0ZW1wOiBnZXR0ZXJzLmF0dGVtcCxcbiAgICAgIGZldGNoaW5nSW52aXRlOiBnZXR0ZXJzLmZldGNoaW5nSW52aXRlXG4gICAgfVxuICB9LFxuXG4gIGNvbXBvbmVudERpZE1vdW50KCl7XG4gICAgYWN0aW9ucy5mZXRjaEludml0ZSh0aGlzLnByb3BzLnBhcmFtcy5pbnZpdGVUb2tlbik7XG4gIH0sXG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICBsZXQge2ZldGNoaW5nSW52aXRlLCBpbnZpdGUsIGF0dGVtcH0gPSB0aGlzLnN0YXRlO1xuXG4gICAgaWYoZmV0Y2hpbmdJbnZpdGUuaXNGYWlsZWQpe1xuICAgICAgcmV0dXJuIDxFeHBpcmVkSW52aXRlLz5cbiAgICB9XG5cbiAgICBpZighaW52aXRlKSB7XG4gICAgICByZXR1cm4gbnVsbDtcbiAgICB9XG5cbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtaW52aXRlIHRleHQtY2VudGVyXCI+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWxvZ28tdHBydFwiPjwvZGl2PlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1jb250ZW50IGdydi1mbGV4XCI+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZmxleC1jb2x1bW5cIj5cbiAgICAgICAgICAgIDxJbnZpdGVJbnB1dEZvcm0gYXR0ZW1wPXthdHRlbXB9IGludml0ZT17aW52aXRlLnRvSlMoKX0vPlxuICAgICAgICAgICAgPEdvb2dsZUF1dGhJbmZvLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LWNvbHVtbiBncnYtaW52aXRlLWJhcmNvZGVcIj5cbiAgICAgICAgICAgIDxoND5TY2FuIGJhciBjb2RlIGZvciBhdXRoIHRva2VuIDxici8+IDxzbWFsbD5TY2FuIGJlbG93IHRvIGdlbmVyYXRlIHlvdXIgdHdvIGZhY3RvciB0b2tlbjwvc21hbGw+PC9oND5cbiAgICAgICAgICAgIDxpbWcgY2xhc3NOYW1lPVwiaW1nLXRodW1ibmFpbFwiIHNyYz17IGBkYXRhOmltYWdlL3BuZztiYXNlNjQsJHtpbnZpdGUuZ2V0KCdxcicpfWAgfSAvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IEludml0ZTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL25ld1VzZXIuanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB1c2VyR2V0dGVycyA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3VzZXIvZ2V0dGVycycpO1xudmFyIG5vZGVHZXR0ZXJzID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvbm9kZXMvZ2V0dGVycycpO1xudmFyIE5vZGVMaXN0ID0gcmVxdWlyZSgnLi9ub2RlTGlzdC5qc3gnKTtcblxudmFyIE5vZGVzID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW3JlYWN0b3IuUmVhY3RNaXhpbl0sXG5cbiAgZ2V0RGF0YUJpbmRpbmdzKCkge1xuICAgIHJldHVybiB7XG4gICAgICBub2RlUmVjb3Jkczogbm9kZUdldHRlcnMubm9kZUxpc3RWaWV3LFxuICAgICAgdXNlcjogdXNlckdldHRlcnMudXNlclxuICAgIH1cbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIHZhciBub2RlUmVjb3JkcyA9IHRoaXMuc3RhdGUubm9kZVJlY29yZHM7XG4gICAgdmFyIGxvZ2lucyA9IHRoaXMuc3RhdGUudXNlci5sb2dpbnM7XG4gICAgcmV0dXJuICggPE5vZGVMaXN0IG5vZGVSZWNvcmRzPXtub2RlUmVjb3Jkc30gbG9naW5zPXtsb2dpbnN9Lz4gKTtcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gTm9kZXM7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9ub2Rlcy9tYWluLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIge2dldHRlcnMsIGFjdGlvbnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvc2Vzc2lvbnMnKTtcbnZhciBTZXNzaW9uTGlzdCA9IHJlcXVpcmUoJy4vc2Vzc2lvbkxpc3QuanN4Jyk7XG52YXIge0RhdGVSYW5nZVBpY2tlciwgQ2FsZW5kYXJOYXZ9ID0gcmVxdWlyZSgnLi8uLi9kYXRlUGlja2VyLmpzeCcpO1xudmFyIG1vbWVudCA9IHJlcXVpcmUoJ21vbWVudCcpO1xudmFyIHttb250aFJhbmdlfSA9IHJlcXVpcmUoJ2FwcC9jb21tb24vZGF0ZVV0aWxzJyk7XG5cbnZhciBTZXNzaW9ucyA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBtaXhpbnM6IFtyZWFjdG9yLlJlYWN0TWl4aW5dLFxuXG4gIGdldEluaXRpYWxTdGF0ZSgpe1xuICAgIGxldCBbc3RhcnREYXRlLCBlbmREYXRlXSA9IG1vbnRoUmFuZ2UobmV3IERhdGUoKSk7XG4gICAgcmV0dXJuIHtcbiAgICAgIHN0YXJ0RGF0ZSxcbiAgICAgIGVuZERhdGVcbiAgICB9XG4gIH0sXG5cbiAgZ2V0RGF0YUJpbmRpbmdzKCkge1xuICAgIHJldHVybiB7XG4gICAgICBzZXNzaW9uc1ZpZXc6IGdldHRlcnMuc2Vzc2lvbnNWaWV3XG4gICAgfVxuICB9LFxuXG4gIHNldE5ld1N0YXRlKHN0YXJ0RGF0ZSwgZW5kRGF0ZSl7XG4gICAgYWN0aW9ucy5mZXRjaFNlc3Npb25zKHN0YXJ0RGF0ZSwgZW5kRGF0ZSk7XG4gICAgdGhpcy5zdGF0ZS5zdGFydERhdGUgPSBzdGFydERhdGU7XG4gICAgdGhpcy5zdGF0ZS5lbmREYXRlID0gZW5kRGF0ZTtcbiAgICB0aGlzLnNldFN0YXRlKHRoaXMuc3RhdGUpO1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxNb3VudCgpe1xuICAgIGFjdGlvbnMuZmV0Y2hTZXNzaW9ucyh0aGlzLnN0YXRlLnN0YXJ0RGF0ZSwgdGhpcy5zdGF0ZS5lbmREYXRlKTtcbiAgfSxcblxuICBjb21wb25lbnRXaWxsVW5tb3VudDogZnVuY3Rpb24oKSB7XG4gIH0sXG5cbiAgb25SYW5nZVBpY2tlckNoYW5nZSh7c3RhcnREYXRlLCBlbmREYXRlfSl7XG4gICAgdGhpcy5zZXROZXdTdGF0ZShzdGFydERhdGUsIGVuZERhdGUpO1xuICB9LFxuXG4gIG9uQ2FsZW5kYXJOYXZDaGFuZ2UobmV3VmFsdWUpe1xuICAgIGxldCBbc3RhcnREYXRlLCBlbmREYXRlXSA9IG1vbnRoUmFuZ2UobmV3VmFsdWUpO1xuICAgIHRoaXMuc2V0TmV3U3RhdGUoc3RhcnREYXRlLCBlbmREYXRlKTtcbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIGxldCB7c3RhcnREYXRlLCBlbmREYXRlfSA9IHRoaXMuc3RhdGU7XG4gICAgbGV0IGRhdGEgPSB0aGlzLnN0YXRlLnNlc3Npb25zVmlldy5maWx0ZXIoXG4gICAgICBpdGVtID0+IG1vbWVudChpdGVtLmNyZWF0ZWQpLmlzQmV0d2VlbihzdGFydERhdGUsIGVuZERhdGUpKTtcblxuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1zZXNzaW9uc1wiPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4XCI+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZmxleC1jb2x1bW5cIj5cbiAgICAgICAgICAgIDxoMT4gU2Vzc2lvbnM8L2gxPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICA8L2Rpdj5cblxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4XCI+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZmxleC1jb2x1bW5cIj5cbiAgICAgICAgICAgIDxEYXRlUmFuZ2VQaWNrZXIgc3RhcnREYXRlPXtzdGFydERhdGV9IGVuZERhdGU9e2VuZERhdGV9IG9uQ2hhbmdlPXt0aGlzLm9uUmFuZ2VQaWNrZXJDaGFuZ2V9Lz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LWNvbHVtblwiPlxuICAgICAgICAgICAgPENhbGVuZGFyTmF2IHZhbHVlPXtzdGFydERhdGV9IG9uVmFsdWVDaGFuZ2U9e3RoaXMub25DYWxlbmRhck5hdkNoYW5nZX0vPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWZsZXgtY29sdW1uXCI+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgIDwvZGl2PlxuICAgICAgICA8U2Vzc2lvbkxpc3Qgc2Vzc2lvblJlY29yZHM9e2RhdGF9Lz5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pO1xuXG5cbm1vZHVsZS5leHBvcnRzID0gU2Vzc2lvbnM7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9zZXNzaW9ucy9tYWluLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgeyBMaW5rIH0gPSByZXF1aXJlKCdyZWFjdC1yb3V0ZXInKTtcbnZhciBMaW5rZWRTdGF0ZU1peGluID0gcmVxdWlyZSgncmVhY3QtYWRkb25zLWxpbmtlZC1zdGF0ZS1taXhpbicpO1xudmFyIHtUYWJsZSwgQ29sdW1uLCBDZWxsLCBUZXh0Q2VsbCwgU29ydEhlYWRlckNlbGwsIFNvcnRUeXBlc30gPSByZXF1aXJlKCdhcHAvY29tcG9uZW50cy90YWJsZS5qc3gnKTtcbnZhciB7b3Blbn0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3Rpb25zJyk7XG52YXIgbW9tZW50ID0gIHJlcXVpcmUoJ21vbWVudCcpO1xudmFyIHtpc01hdGNofSA9IHJlcXVpcmUoJ2FwcC9jb21tb24vb2JqZWN0VXRpbHMnKTtcbnZhciBfID0gcmVxdWlyZSgnXycpO1xuXG5jb25zdCBEYXRlQ3JlYXRlZENlbGwgPSAoeyByb3dJbmRleCwgZGF0YSwgLi4ucHJvcHMgfSkgPT4ge1xuICB2YXIgY3JlYXRlZCA9IGRhdGFbcm93SW5kZXhdLmNyZWF0ZWQ7XG4gIHZhciBkaXNwbGF5RGF0ZSA9IG1vbWVudChjcmVhdGVkKS5mcm9tTm93KCk7XG4gIHJldHVybiAoXG4gICAgPENlbGwgey4uLnByb3BzfT5cbiAgICAgIHsgZGlzcGxheURhdGUgfVxuICAgIDwvQ2VsbD5cbiAgKVxufTtcblxuY29uc3QgRHVyYXRpb25DZWxsID0gKHsgcm93SW5kZXgsIGRhdGEsIC4uLnByb3BzIH0pID0+IHtcbiAgdmFyIGNyZWF0ZWQgPSBkYXRhW3Jvd0luZGV4XS5jcmVhdGVkO1xuICB2YXIgbGFzdEFjdGl2ZSA9IGRhdGFbcm93SW5kZXhdLmxhc3RBY3RpdmU7XG5cbiAgdmFyIGVuZCA9IG1vbWVudChjcmVhdGVkKTtcbiAgdmFyIG5vdyA9IG1vbWVudChsYXN0QWN0aXZlKTtcbiAgdmFyIGR1cmF0aW9uID0gbW9tZW50LmR1cmF0aW9uKG5vdy5kaWZmKGVuZCkpO1xuICB2YXIgZGlzcGxheURhdGUgPSBkdXJhdGlvbi5odW1hbml6ZSgpO1xuXG4gIHJldHVybiAoXG4gICAgPENlbGwgey4uLnByb3BzfT5cbiAgICAgIHsgZGlzcGxheURhdGUgfVxuICAgIDwvQ2VsbD5cbiAgKVxufTtcblxuY29uc3QgVXNlcnNDZWxsID0gKHsgcm93SW5kZXgsIGRhdGEsIC4uLnByb3BzIH0pID0+IHtcbiAgdmFyICR1c2VycyA9IGRhdGFbcm93SW5kZXhdLnBhcnRpZXMubWFwKChpdGVtLCBpdGVtSW5kZXgpPT5cbiAgICAoPHNwYW4ga2V5PXtpdGVtSW5kZXh9IHN0eWxlPXt7YmFja2dyb3VuZENvbG9yOiAnIzFhYjM5NCd9fSBjbGFzc05hbWU9XCJ0ZXh0LXVwcGVyY2FzZSBncnYtcm91bmRlZCBsYWJlbCBsYWJlbC1wcmltYXJ5XCI+e2l0ZW0udXNlclswXX08L3NwYW4+KVxuICApXG5cbiAgcmV0dXJuIChcbiAgICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgICAgPGRpdj5cbiAgICAgICAgeyR1c2Vyc31cbiAgICAgIDwvZGl2PlxuICAgIDwvQ2VsbD5cbiAgKVxufTtcblxuY29uc3QgQnV0dG9uQ2VsbCA9ICh7IHJvd0luZGV4LCBkYXRhLCAuLi5wcm9wcyB9KSA9PiB7XG4gIHZhciB7IHNlc3Npb25VcmwsIGFjdGl2ZSB9ID0gZGF0YVtyb3dJbmRleF07XG4gIHZhciBbYWN0aW9uVGV4dCwgYWN0aW9uQ2xhc3NdID0gYWN0aXZlID8gWydqb2luJywgJ2J0bi13YXJuaW5nJ10gOiBbJ3BsYXknLCAnYnRuLXByaW1hcnknXTtcbiAgcmV0dXJuIChcbiAgICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgICAgPExpbmsgdG89e3Nlc3Npb25Vcmx9IGNsYXNzTmFtZT17XCJidG4gXCIgK2FjdGlvbkNsYXNzKyBcIiBidG4teHNcIn0gdHlwZT1cImJ1dHRvblwiPnthY3Rpb25UZXh0fTwvTGluaz5cbiAgICA8L0NlbGw+XG4gIClcbn1cblxudmFyIFNlc3Npb25MaXN0ID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW0xpbmtlZFN0YXRlTWl4aW5dLFxuXG4gIGdldEluaXRpYWxTdGF0ZShwcm9wcyl7XG4gICAgdGhpcy5zZWFyY2hhYmxlUHJvcHMgPSBbJ3NlcnZlcklwJywgJ2NyZWF0ZWQnLCAnYWN0aXZlJ107XG4gICAgcmV0dXJuIHsgZmlsdGVyOiAnJywgY29sU29ydERpcnM6IHt9IH07XG4gIH0sXG5cbiAgb25Tb3J0Q2hhbmdlKGNvbHVtbktleSwgc29ydERpcikge1xuICAgIHRoaXMuc2V0U3RhdGUoe1xuICAgICAgLi4udGhpcy5zdGF0ZSxcbiAgICAgIGNvbFNvcnREaXJzOiB7IFtjb2x1bW5LZXldOiBzb3J0RGlyIH1cbiAgICB9KTtcbiAgfSxcblxuICBzZWFyY2hBbmRGaWx0ZXJDYih0YXJnZXRWYWx1ZSwgc2VhcmNoVmFsdWUsIHByb3BOYW1lKXtcbiAgICBpZihwcm9wTmFtZSA9PT0gJ2NyZWF0ZWQnKXtcbiAgICAgIHZhciBkaXNwbGF5RGF0ZSA9IG1vbWVudCh0YXJnZXRWYWx1ZSkuZnJvbU5vdygpLnRvTG9jYWxlVXBwZXJDYXNlKCk7XG4gICAgICByZXR1cm4gZGlzcGxheURhdGUuaW5kZXhPZihzZWFyY2hWYWx1ZSkgIT09IC0xO1xuICAgIH1cbiAgfSxcblxuICBzb3J0QW5kRmlsdGVyKGRhdGEpe1xuICAgIHZhciBmaWx0ZXJlZCA9IGRhdGEuZmlsdGVyKG9iaj0+XG4gICAgICBpc01hdGNoKG9iaiwgdGhpcy5zdGF0ZS5maWx0ZXIsIHtcbiAgICAgICAgc2VhcmNoYWJsZVByb3BzOiB0aGlzLnNlYXJjaGFibGVQcm9wcyxcbiAgICAgICAgY2I6IHRoaXMuc2VhcmNoQW5kRmlsdGVyQ2JcbiAgICAgIH0pKTtcblxuICAgIHZhciBjb2x1bW5LZXkgPSBPYmplY3QuZ2V0T3duUHJvcGVydHlOYW1lcyh0aGlzLnN0YXRlLmNvbFNvcnREaXJzKVswXTtcbiAgICB2YXIgc29ydERpciA9IHRoaXMuc3RhdGUuY29sU29ydERpcnNbY29sdW1uS2V5XTtcbiAgICB2YXIgc29ydGVkID0gXy5zb3J0QnkoZmlsdGVyZWQsIGNvbHVtbktleSk7XG4gICAgaWYoc29ydERpciA9PT0gU29ydFR5cGVzLkFTQyl7XG4gICAgICBzb3J0ZWQgPSBzb3J0ZWQucmV2ZXJzZSgpO1xuICAgIH1cblxuICAgIHJldHVybiBzb3J0ZWQ7XG4gIH0sXG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICB2YXIgZGF0YSA9IHRoaXMuc29ydEFuZEZpbHRlcih0aGlzLnByb3BzLnNlc3Npb25SZWNvcmRzKTtcbiAgICByZXR1cm4gKFxuICAgICAgPGRpdj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtc2VhcmNoXCI+XG4gICAgICAgICAgPGlucHV0IHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ2ZpbHRlcicpfSBwbGFjZWhvbGRlcj1cIlNlYXJjaC4uLlwiIGNsYXNzTmFtZT1cImZvcm0tY29udHJvbCBpbnB1dC1zbVwiLz5cbiAgICAgICAgPC9kaXY+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiXCI+XG4gICAgICAgICAgPFRhYmxlIHJvd0NvdW50PXtkYXRhLmxlbmd0aH0gY2xhc3NOYW1lPVwidGFibGUtc3RyaXBlZFwiPlxuICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJzaWRcIlxuICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPiBTZXNzaW9uIElEIDwvQ2VsbD4gfVxuICAgICAgICAgICAgICBjZWxsPXs8VGV4dENlbGwgZGF0YT17ZGF0YX0vPiB9XG4gICAgICAgICAgICAvPlxuICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPiA8L0NlbGw+IH1cbiAgICAgICAgICAgICAgY2VsbD17XG4gICAgICAgICAgICAgICAgPEJ1dHRvbkNlbGwgZGF0YT17ZGF0YX0gLz5cbiAgICAgICAgICAgICAgfVxuICAgICAgICAgICAgLz5cbiAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgY29sdW1uS2V5PVwic2VydmVySXBcIlxuICAgICAgICAgICAgICBoZWFkZXI9e1xuICAgICAgICAgICAgICAgIDxTb3J0SGVhZGVyQ2VsbFxuICAgICAgICAgICAgICAgICAgc29ydERpcj17dGhpcy5zdGF0ZS5jb2xTb3J0RGlycy5zZXJ2ZXJJcH1cbiAgICAgICAgICAgICAgICAgIG9uU29ydENoYW5nZT17dGhpcy5vblNvcnRDaGFuZ2V9XG4gICAgICAgICAgICAgICAgICB0aXRsZT1cIk5vZGVcIlxuICAgICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgIH1cbiAgICAgICAgICAgICAgY2VsbD17PFRleHRDZWxsIGRhdGE9e2RhdGF9IC8+IH1cbiAgICAgICAgICAgIC8+XG4gICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgIGNvbHVtbktleT1cImNyZWF0ZWRcIlxuICAgICAgICAgICAgICBoZWFkZXI9e1xuICAgICAgICAgICAgICAgIDxTb3J0SGVhZGVyQ2VsbFxuICAgICAgICAgICAgICAgICAgc29ydERpcj17dGhpcy5zdGF0ZS5jb2xTb3J0RGlycy5jcmVhdGVkfVxuICAgICAgICAgICAgICAgICAgb25Tb3J0Q2hhbmdlPXt0aGlzLm9uU29ydENoYW5nZX1cbiAgICAgICAgICAgICAgICAgIHRpdGxlPVwiQ3JlYXRlZFwiXG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgfVxuICAgICAgICAgICAgICBjZWxsPXs8RGF0ZUNyZWF0ZWRDZWxsIGRhdGE9e2RhdGF9Lz4gfVxuICAgICAgICAgICAgLz5cbiAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgY29sdW1uS2V5PVwiYWN0aXZlXCJcbiAgICAgICAgICAgICAgaGVhZGVyPXtcbiAgICAgICAgICAgICAgICA8U29ydEhlYWRlckNlbGxcbiAgICAgICAgICAgICAgICAgIHNvcnREaXI9e3RoaXMuc3RhdGUuY29sU29ydERpcnMuYWN0aXZlfVxuICAgICAgICAgICAgICAgICAgb25Tb3J0Q2hhbmdlPXt0aGlzLm9uU29ydENoYW5nZX1cbiAgICAgICAgICAgICAgICAgIHRpdGxlPVwiQWN0aXZlXCJcbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICB9XG4gICAgICAgICAgICAgIGNlbGw9ezxVc2Vyc0NlbGwgZGF0YT17ZGF0YX0gLz4gfVxuICAgICAgICAgICAgLz5cbiAgICAgICAgICA8L1RhYmxlPlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgIClcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gU2Vzc2lvbkxpc3Q7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9zZXNzaW9ucy9zZXNzaW9uTGlzdC5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHJlbmRlciA9IHJlcXVpcmUoJ3JlYWN0LWRvbScpLnJlbmRlcjtcbnZhciB7IFJvdXRlciwgUm91dGUsIFJlZGlyZWN0LCBJbmRleFJvdXRlLCBicm93c2VySGlzdG9yeSB9ID0gcmVxdWlyZSgncmVhY3Qtcm91dGVyJyk7XG52YXIgeyBBcHAsIExvZ2luLCBOb2RlcywgU2Vzc2lvbnMsIE5ld1VzZXIsIEN1cnJlbnRTZXNzaW9uSG9zdCwgTm90Rm91bmQgfSA9IHJlcXVpcmUoJy4vY29tcG9uZW50cycpO1xudmFyIHtlbnN1cmVVc2VyfSA9IHJlcXVpcmUoJy4vbW9kdWxlcy91c2VyL2FjdGlvbnMnKTtcbnZhciBhdXRoID0gcmVxdWlyZSgnLi9hdXRoJyk7XG52YXIgc2Vzc2lvbiA9IHJlcXVpcmUoJy4vc2Vzc2lvbicpO1xudmFyIGNmZyA9IHJlcXVpcmUoJy4vY29uZmlnJyk7XG5cbnJlcXVpcmUoJy4vbW9kdWxlcycpO1xuXG4vLyBpbml0IHNlc3Npb25cbnNlc3Npb24uaW5pdCgpO1xuXG5mdW5jdGlvbiBoYW5kbGVMb2dvdXQobmV4dFN0YXRlLCByZXBsYWNlLCBjYil7XG4gIGF1dGgubG9nb3V0KCk7XG59XG5cbnJlbmRlcigoXG4gIDxSb3V0ZXIgaGlzdG9yeT17c2Vzc2lvbi5nZXRIaXN0b3J5KCl9PlxuICAgIDxSb3V0ZSBwYXRoPXtjZmcucm91dGVzLmxvZ2lufSBjb21wb25lbnQ9e0xvZ2lufS8+XG4gICAgPFJvdXRlIHBhdGg9e2NmZy5yb3V0ZXMubG9nb3V0fSBvbkVudGVyPXtoYW5kbGVMb2dvdXR9Lz5cbiAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5uZXdVc2VyfSBjb21wb25lbnQ9e05ld1VzZXJ9Lz5cbiAgICA8UmVkaXJlY3QgZnJvbT17Y2ZnLnJvdXRlcy5hcHB9IHRvPXtjZmcucm91dGVzLm5vZGVzfS8+XG4gICAgPFJvdXRlIHBhdGg9e2NmZy5yb3V0ZXMuYXBwfSBjb21wb25lbnQ9e0FwcH0gb25FbnRlcj17ZW5zdXJlVXNlcn0gPlxuICAgICAgPFJvdXRlIHBhdGg9e2NmZy5yb3V0ZXMubm9kZXN9IGNvbXBvbmVudD17Tm9kZXN9Lz5cbiAgICAgIDxSb3V0ZSBwYXRoPXtjZmcucm91dGVzLmFjdGl2ZVNlc3Npb259IGNvbXBvbmVudHM9e3tDdXJyZW50U2Vzc2lvbkhvc3Q6IEN1cnJlbnRTZXNzaW9uSG9zdH19Lz5cbiAgICAgIDxSb3V0ZSBwYXRoPXtjZmcucm91dGVzLnNlc3Npb25zfSBjb21wb25lbnQ9e1Nlc3Npb25zfS8+XG4gICAgPC9Sb3V0ZT4gICAgXG4gICAgPFJvdXRlIHBhdGg9XCIqXCIgY29tcG9uZW50PXtOb3RGb3VuZH0gLz5cbiAgPC9Sb3V0ZXI+XG4pLCBkb2N1bWVudC5nZXRFbGVtZW50QnlJZChcImFwcFwiKSk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvaW5kZXguanN4XG4gKiovIiwibW9kdWxlLmV4cG9ydHMgPSBUZXJtaW5hbDtcblxuXG4vKioqKioqKioqKioqKioqKipcbiAqKiBXRUJQQUNLIEZPT1RFUlxuICoqIGV4dGVybmFsIFwiVGVybWluYWxcIlxuICoqIG1vZHVsZSBpZCA9IDQxMVxuICoqIG1vZHVsZSBjaHVua3MgPSAxXG4gKiovIl0sInNvdXJjZVJvb3QiOiIifQ==