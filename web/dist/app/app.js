webpackJsonp([1],{

/***/ 0:
/***/ function(module, exports, __webpack_require__) {

	module.exports = __webpack_require__(313);


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
	
	var _require = __webpack_require__(274);
	
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
	
	var _require = __webpack_require__(42);
	
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

/***/ 40:
/***/ function(module, exports) {

	'use strict';
	
	exports.__esModule = true;
	var nodeHostNameByServerId = function nodeHostNameByServerId(serverId) {
	  return [['tlpt_nodes'], function (nodes) {
	    var server = nodes.find(function (item) {
	      return item.get('id') === serverId;
	    });
	    return !server ? '' : server.get('hostname');
	  }];
	};
	
	var nodeListView = [['tlpt_nodes'], function (nodes) {
	  return nodes.map(function (item) {
	    var serverId = item.get('id');
	    return {
	      id: serverId,
	      hostname: item.get('hostname'),
	      tags: getTags(item),
	      addr: item.get('addr')
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
	  nodeListView: nodeListView,
	  nodeHostNameByServerId: nodeHostNameByServerId
	};
	module.exports = exports['default'];

/***/ },

/***/ 45:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(63);
	module.exports.actions = __webpack_require__(62);
	module.exports.activeTermStore = __webpack_require__(99);

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
	
	module.exports.getters = __webpack_require__(113);
	module.exports.actions = __webpack_require__(65);
	module.exports.activeTermStore = __webpack_require__(114);

/***/ },

/***/ 48:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(46);
	
	var TRYING_TO_LOGIN = _require.TRYING_TO_LOGIN;
	
	var _require2 = __webpack_require__(111);
	
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

/***/ 51:
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

/***/ 61:
/***/ function(module, exports) {

	module.exports = _;

/***/ },

/***/ 62:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	var session = __webpack_require__(26);
	
	var _require = __webpack_require__(287);
	
	var uuid = _require.uuid;
	
	var api = __webpack_require__(32);
	var cfg = __webpack_require__(11);
	var getters = __webpack_require__(63);
	var sessionModule = __webpack_require__(47);
	
	var _require2 = __webpack_require__(98);
	
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

/***/ 63:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(113);
	
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

/***/ 64:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(102);
	
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

/***/ 65:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	var api = __webpack_require__(32);
	var cfg = __webpack_require__(11);
	
	var _require = __webpack_require__(112);
	
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

/***/ 94:
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

/***/ 95:
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

/***/ 96:
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
	        if (result === true) {
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

/***/ 97:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };
	
	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError('Cannot call a class as a function'); } }
	
	function _inherits(subClass, superClass) { if (typeof superClass !== 'function' && superClass !== null) { throw new TypeError('Super expression must either be null or a function, not ' + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }
	
	var EventEmitter = __webpack_require__(288).EventEmitter;
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

/***/ 98:
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

/***/ 99:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(98);
	
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

/***/ 100:
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

/***/ 101:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(100);
	
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

/***/ 102:
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

/***/ 103:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(102);
	
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

/***/ 104:
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

/***/ 105:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(104);
	
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

/***/ 106:
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

/***/ 107:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(106);
	
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

/***/ 108:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(106);
	
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

/***/ 109:
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

/***/ 110:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(109);
	
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

/***/ 111:
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

/***/ 112:
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

/***/ 113:
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

/***/ 114:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(112);
	
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

/***/ 115:
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

/***/ 116:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(115);
	
	var TLPT_RECEIVE_USER = _require.TLPT_RECEIVE_USER;
	
	var _require2 = __webpack_require__(46);
	
	var TRYING_TO_SIGN_UP = _require2.TRYING_TO_SIGN_UP;
	var TRYING_TO_LOGIN = _require2.TRYING_TO_LOGIN;
	
	var restApiActions = __webpack_require__(110);
	var auth = __webpack_require__(94);
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

/***/ 117:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(48);
	module.exports.actions = __webpack_require__(116);
	module.exports.nodeStore = __webpack_require__(118);

/***/ },

/***/ 118:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(115);
	
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

/***/ 222:
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

/***/ 223:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	exports.__esModule = true;
	var React = __webpack_require__(4);
	var $ = __webpack_require__(39);
	var moment = __webpack_require__(1);
	
	var _require = __webpack_require__(61);
	
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

/***/ 224:
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

/***/ 225:
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

/***/ 226:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };
	
	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }
	
	var React = __webpack_require__(4);
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(285);
	
	var getters = _require.getters;
	var actions = _require.actions;
	
	var userGetters = __webpack_require__(48);
	
	var _require2 = __webpack_require__(51);
	
	var Table = _require2.Table;
	var Column = _require2.Column;
	var Cell = _require2.Cell;
	var SortHeaderCell = _require2.SortHeaderCell;
	var SortTypes = _require2.SortTypes;
	
	var _require3 = __webpack_require__(62);
	
	var createNewSession = _require3.createNewSession;
	
	var LinkedStateMixin = __webpack_require__(41);
	var _ = __webpack_require__(61);
	
	var _require4 = __webpack_require__(96);
	
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
	    this.searchableProps = ['addr', 'hostname'];
	    return { filter: '', colSortDirs: { hostname: 'DESC' } };
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
	      { className: 'grv-nodes grv-page' },
	      React.createElement(
	        'div',
	        { className: 'grv-flex grv-header' },
	        React.createElement('div', { className: 'grv-flex-column' }),
	        React.createElement(
	          'div',
	          { className: 'grv-flex-column' },
	          React.createElement(
	            'h1',
	            null,
	            ' Nodes '
	          )
	        ),
	        React.createElement(
	          'div',
	          { className: 'grv-flex-column' },
	          React.createElement(
	            'div',
	            { className: 'grv-search' },
	            React.createElement('input', { valueLink: this.linkState('filter'), placeholder: 'Search...', className: 'form-control input-sm' })
	          )
	        )
	      ),
	      React.createElement(
	        'div',
	        { className: '' },
	        React.createElement(
	          Table,
	          { rowCount: data.length, className: 'table-striped grv-nodes-table' },
	          React.createElement(Column, {
	            columnKey: 'hostname',
	            header: React.createElement(SortHeaderCell, {
	              sortDir: this.state.colSortDirs.hostname,
	              onSortChange: this.onSortChange,
	              title: 'Node'
	            }),
	            cell: React.createElement(TextCell, { data: data })
	          }),
	          React.createElement(Column, {
	            columnKey: 'addr',
	            header: React.createElement(SortHeaderCell, {
	              sortDir: this.state.colSortDirs.addr,
	              onSortChange: this.onSortChange,
	              title: 'IP'
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

/***/ 227:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(280);
	
	var getters = _require.getters;
	
	var _require2 = __webpack_require__(64);
	
	var closeSelectNodeDialog = _require2.closeSelectNodeDialog;
	
	var _require3 = __webpack_require__(62);
	
	var changeServer = _require3.changeServer;
	
	var NodeList = __webpack_require__(226);
	var activeSessionGetters = __webpack_require__(63);
	var nodeGetters = __webpack_require__(40);
	
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

/***/ 228:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	exports.__esModule = true;
	
	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }
	
	var React = __webpack_require__(4);
	
	var _require = __webpack_require__(42);
	
	var Link = _require.Link;
	
	var _require2 = __webpack_require__(47);
	
	var actions = _require2.actions;
	
	var _require3 = __webpack_require__(40);
	
	var nodeHostNameByServerId = _require3.nodeHostNameByServerId;
	
	var _require4 = __webpack_require__(51);
	
	var Cell = _require4.Cell;
	var TextCell = _require4.TextCell;
	
	var moment = __webpack_require__(1);
	
	var DateCreatedCell = function DateCreatedCell(_ref) {
	  var rowIndex = _ref.rowIndex;
	  var data = _ref.data;
	
	  var props = _objectWithoutProperties(_ref, ['rowIndex', 'data']);
	
	  var created = data[rowIndex].created;
	  var displayDate = moment(created).format('l LTS');
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
	
	var SingleUserCell = function SingleUserCell(_ref3) {
	  var rowIndex = _ref3.rowIndex;
	  var data = _ref3.data;
	
	  var props = _objectWithoutProperties(_ref3, ['rowIndex', 'data']);
	
	  return React.createElement(
	    Cell,
	    props,
	    React.createElement(
	      'span',
	      { className: 'grv-sessions-user label label-default' },
	      data[rowIndex].login
	    )
	  );
	};
	
	var UsersCell = function UsersCell(_ref4) {
	  var rowIndex = _ref4.rowIndex;
	  var data = _ref4.data;
	
	  var props = _objectWithoutProperties(_ref4, ['rowIndex', 'data']);
	
	  var $users = data[rowIndex].parties.map(function (item, itemIndex) {
	    return React.createElement(
	      'span',
	      { key: itemIndex, className: 'grv-sessions-user label label-default' },
	      item.user
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
	
	var ButtonCell = function ButtonCell(_ref5) {
	  var rowIndex = _ref5.rowIndex;
	  var data = _ref5.data;
	
	  var props = _objectWithoutProperties(_ref5, ['rowIndex', 'data']);
	
	  var _data$rowIndex = data[rowIndex];
	  var sessionUrl = _data$rowIndex.sessionUrl;
	  var active = _data$rowIndex.active;
	
	  var _ref6 = active ? ['join', 'btn-warning'] : ['play', 'btn-primary'];
	
	  var actionText = _ref6[0];
	  var actionClass = _ref6[1];
	
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
	
	var EmptyList = function EmptyList(_ref7) {
	  var text = _ref7.text;
	  return React.createElement(
	    'div',
	    { className: 'grv-sessions-empty text-center text-muted' },
	    React.createElement(
	      'span',
	      null,
	      text
	    )
	  );
	};
	
	var NodeCell = function NodeCell(_ref8) {
	  var rowIndex = _ref8.rowIndex;
	  var data = _ref8.data;
	
	  var props = _objectWithoutProperties(_ref8, ['rowIndex', 'data']);
	
	  var serverId = data[rowIndex].serverId;
	
	  var hostname = reactor.evaluate(nodeHostNameByServerId(serverId)) || 'unknown';
	
	  return React.createElement(
	    Cell,
	    props,
	    hostname
	  );
	};
	
	exports['default'] = ButtonCell;
	exports.ButtonCell = ButtonCell;
	exports.UsersCell = UsersCell;
	exports.DurationCell = DurationCell;
	exports.DateCreatedCell = DateCreatedCell;
	exports.EmptyList = EmptyList;
	exports.SingleUserCell = SingleUserCell;
	exports.NodeCell = NodeCell;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "listItems.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 229:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var Term = __webpack_require__(413);
	var React = __webpack_require__(4);
	
	var _require = __webpack_require__(61);
	
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

/***/ 274:
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

/***/ 275:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError('Cannot call a class as a function'); } }
	
	function _inherits(subClass, superClass) { if (typeof superClass !== 'function' && superClass !== null) { throw new TypeError('Super expression must either be null or a function, not ' + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }
	
	var Tty = __webpack_require__(97);
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

/***/ 276:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(65);
	
	var fetchSessions = _require.fetchSessions;
	
	var _require2 = __webpack_require__(107);
	
	var fetchNodes = _require2.fetchNodes;
	
	var _require3 = __webpack_require__(95);
	
	var monthRange = _require3.monthRange;
	
	var $ = __webpack_require__(39);
	
	var _require4 = __webpack_require__(100);
	
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

/***/ 277:
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

/***/ 278:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(277);
	module.exports.actions = __webpack_require__(276);
	module.exports.appStore = __webpack_require__(101);

/***/ },

/***/ 279:
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

/***/ 280:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(279);
	module.exports.actions = __webpack_require__(64);
	module.exports.dialogStore = __webpack_require__(103);

/***/ },

/***/ 281:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var reactor = __webpack_require__(7);
	reactor.registerStores({
	  'tlpt': __webpack_require__(101),
	  'tlpt_dialogs': __webpack_require__(103),
	  'tlpt_active_terminal': __webpack_require__(99),
	  'tlpt_user': __webpack_require__(118),
	  'tlpt_nodes': __webpack_require__(108),
	  'tlpt_invite': __webpack_require__(105),
	  'tlpt_rest_api': __webpack_require__(286),
	  'tlpt_sessions': __webpack_require__(114)
	});

/***/ },

/***/ 282:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(104);
	
	var TLPT_RECEIVE_USER_INVITE = _require.TLPT_RECEIVE_USER_INVITE;
	
	var _require2 = __webpack_require__(46);
	
	var FETCHING_INVITE = _require2.FETCHING_INVITE;
	
	var restApiActions = __webpack_require__(110);
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

/***/ 283:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(46);
	
	var TRYING_TO_SIGN_UP = _require.TRYING_TO_SIGN_UP;
	var FETCHING_INVITE = _require.FETCHING_INVITE;
	
	var _require2 = __webpack_require__(111);
	
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

/***/ 284:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(283);
	module.exports.actions = __webpack_require__(282);
	module.exports.nodeStore = __webpack_require__(105);

/***/ },

/***/ 285:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(40);
	module.exports.actions = __webpack_require__(107);
	module.exports.nodeStore = __webpack_require__(108);
	
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

/***/ 286:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(16);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(109);
	
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

/***/ 287:
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

/***/ 288:
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

/***/ 300:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	var NavLeftBar = __webpack_require__(307);
	var cfg = __webpack_require__(11);
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(278);
	
	var actions = _require.actions;
	var getters = _require.getters;
	
	var SelectNodeDialog = __webpack_require__(227);
	
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
	      { className: 'grv-tlpt grv-flex grv-flex-row' },
	      React.createElement(SelectNodeDialog, null),
	      this.props.CurrentSessionHost,
	      React.createElement(NavLeftBar, null),
	      this.props.children
	    );
	  }
	});
	
	module.exports = App;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "app.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 301:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };
	
	var React = __webpack_require__(4);
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(45);
	
	var getters = _require.getters;
	var actions = _require.actions;
	
	var _require2 = __webpack_require__(40);
	
	var nodeHostNameByServerId = _require2.nodeHostNameByServerId;
	
	var Tty = __webpack_require__(97);
	var TtyTerminal = __webpack_require__(229);
	var EventStreamer = __webpack_require__(302);
	var SessionLeftPanel = __webpack_require__(222);
	
	var _require3 = __webpack_require__(64);
	
	var showSelectNodeDialog = _require3.showSelectNodeDialog;
	var closeSelectNodeDialog = _require3.closeSelectNodeDialog;
	
	var SelectNodeDialog = __webpack_require__(227);
	
	var ActiveSession = React.createClass({
	  displayName: 'ActiveSession',
	
	  componentWillUnmount: function componentWillUnmount() {
	    closeSelectNodeDialog();
	  },
	
	  render: function render() {
	    var _props$activeSession = this.props.activeSession;
	    var login = _props$activeSession.login;
	    var parties = _props$activeSession.parties;
	    var serverId = _props$activeSession.serverId;
	
	    var serverLabelText = '';
	    if (serverId) {
	      var hostname = reactor.evaluate(nodeHostNameByServerId(serverId));
	      serverLabelText = login + '@' + hostname;
	    }
	
	    return React.createElement(
	      'div',
	      { className: 'grv-current-session' },
	      React.createElement(SessionLeftPanel, { parties: parties }),
	      React.createElement(
	        'div',
	        { className: 'grv-current-session-server-info' },
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

/***/ 302:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	exports.__esModule = true;
	var cfg = __webpack_require__(11);
	var React = __webpack_require__(4);
	var session = __webpack_require__(26);
	
	var _require = __webpack_require__(65);
	
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

/***/ 303:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	
	var _require = __webpack_require__(45);
	
	var getters = _require.getters;
	var actions = _require.actions;
	
	var SessionPlayer = __webpack_require__(304);
	var ActiveSession = __webpack_require__(301);
	
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

/***/ 304:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	exports.__esModule = true;
	var React = __webpack_require__(4);
	var ReactSlider = __webpack_require__(237);
	var TtyPlayer = __webpack_require__(275);
	var TtyTerminal = __webpack_require__(229);
	var SessionLeftPanel = __webpack_require__(222);
	
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

/***/ 305:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	module.exports.App = __webpack_require__(300);
	module.exports.Login = __webpack_require__(306);
	module.exports.NewUser = __webpack_require__(308);
	module.exports.Nodes = __webpack_require__(309);
	module.exports.Sessions = __webpack_require__(311);
	module.exports.CurrentSessionHost = __webpack_require__(303);
	module.exports.NotFound = __webpack_require__(224).NotFound;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "index.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 306:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	var $ = __webpack_require__(39);
	var reactor = __webpack_require__(7);
	var LinkedStateMixin = __webpack_require__(41);
	
	var _require = __webpack_require__(117);
	
	var actions = _require.actions;
	var getters = _require.getters;
	
	var GoogleAuthInfo = __webpack_require__(225);
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

/***/ 307:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	
	var _require = __webpack_require__(42);
	
	var Router = _require.Router;
	var IndexLink = _require.IndexLink;
	var History = _require.History;
	
	var getters = __webpack_require__(48);
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
	        { key: index, className: className, title: i.title },
	        React.createElement(
	          IndexLink,
	          { to: i.to },
	          React.createElement('i', { className: i.icon })
	        )
	      );
	    });
	
	    items.push(React.createElement(
	      'li',
	      { key: items.length, title: 'help' },
	      React.createElement(
	        'a',
	        { href: cfg.helpUrl, target: '_blank' },
	        React.createElement('i', { className: 'fa fa-question' })
	      )
	    ));
	
	    items.push(React.createElement(
	      'li',
	      { key: items.length, title: 'logout' },
	      React.createElement(
	        'a',
	        { href: cfg.routes.logout },
	        React.createElement('i', { className: 'fa fa-sign-out' })
	      )
	    ));
	
	    return React.createElement(
	      'nav',
	      { className: 'grv-nav navbar-default', role: 'navigation' },
	      React.createElement(
	        'ul',
	        { className: 'nav text-center', id: 'side-menu' },
	        React.createElement(
	          'li',
	          { title: 'current user' },
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

/***/ 308:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	var $ = __webpack_require__(39);
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(284);
	
	var actions = _require.actions;
	var getters = _require.getters;
	
	var userModule = __webpack_require__(117);
	var LinkedStateMixin = __webpack_require__(41);
	var GoogleAuthInfo = __webpack_require__(225);
	
	var _require2 = __webpack_require__(224);
	
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

/***/ 309:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	var reactor = __webpack_require__(7);
	var userGetters = __webpack_require__(48);
	var nodeGetters = __webpack_require__(40);
	var NodeList = __webpack_require__(226);
	
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

/***/ 310:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(223);
	
	var DateRangePicker = _require.DateRangePicker;
	var CalendarNav = _require.CalendarNav;
	
	var _require2 = __webpack_require__(51);
	
	var Table = _require2.Table;
	var Column = _require2.Column;
	var Cell = _require2.Cell;
	var TextCell = _require2.TextCell;
	var SortHeaderCell = _require2.SortHeaderCell;
	var SortTypes = _require2.SortTypes;
	
	var _require3 = __webpack_require__(228);
	
	var ButtonCell = _require3.ButtonCell;
	var UsersCell = _require3.UsersCell;
	var EmptyList = _require3.EmptyList;
	var NodeCell = _require3.NodeCell;
	var DurationCell = _require3.DurationCell;
	var DateCreatedCell = _require3.DateCreatedCell;
	
	var ActiveSessionList = React.createClass({
	  displayName: 'ActiveSessionList',
	
	  render: function render() {
	    var data = this.props.data.filter(function (item) {
	      return item.active;
	    });
	    return React.createElement(
	      'div',
	      { className: 'grv-sessions-active' },
	      React.createElement(
	        'div',
	        { className: 'grv-header' },
	        React.createElement(
	          'h1',
	          null,
	          ' Active Sessions '
	        )
	      ),
	      React.createElement(
	        'div',
	        { className: 'grv-content' },
	        data.length === 0 ? React.createElement(EmptyList, { text: 'You have no active sessions.' }) : React.createElement(
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
	              header: React.createElement(
	                Cell,
	                null,
	                ' Node '
	              ),
	              cell: React.createElement(NodeCell, { data: data })
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
	    );
	  }
	});
	
	module.exports = ActiveSessionList;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "activeSessionList.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 311:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(47);
	
	var getters = _require.getters;
	var actions = _require.actions;
	
	var StoredSessionList = __webpack_require__(312);
	var ActiveSessionList = __webpack_require__(310);
	
	var Sessions = React.createClass({
	  displayName: 'Sessions',
	
	  mixins: [reactor.ReactMixin],
	
	  getDataBindings: function getDataBindings() {
	    return { data: getters.sessionsView };
	  },
	
	  render: function render() {
	    var data = this.state.data;
	
	    return React.createElement(
	      'div',
	      { className: 'grv-sessions grv-page' },
	      React.createElement(ActiveSessionList, { data: data }),
	      React.createElement('hr', { className: 'grv-divider' }),
	      React.createElement(StoredSessionList, { data: data })
	    );
	  }
	});
	
	module.exports = Sessions;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "main.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 312:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };
	
	var React = __webpack_require__(4);
	var reactor = __webpack_require__(7);
	
	var _require = __webpack_require__(47);
	
	var actions = _require.actions;
	
	var LinkedStateMixin = __webpack_require__(41);
	
	var _require2 = __webpack_require__(51);
	
	var Table = _require2.Table;
	var Column = _require2.Column;
	var Cell = _require2.Cell;
	var TextCell = _require2.TextCell;
	var SortHeaderCell = _require2.SortHeaderCell;
	var SortTypes = _require2.SortTypes;
	
	var _require3 = __webpack_require__(228);
	
	var ButtonCell = _require3.ButtonCell;
	var SingleUserCell = _require3.SingleUserCell;
	var UsersCell = _require3.UsersCell;
	var EmptyList = _require3.EmptyList;
	var DurationCell = _require3.DurationCell;
	var DateCreatedCell = _require3.DateCreatedCell;
	
	var _require4 = __webpack_require__(223);
	
	var DateRangePicker = _require4.DateRangePicker;
	var CalendarNav = _require4.CalendarNav;
	
	var moment = __webpack_require__(1);
	
	var _require5 = __webpack_require__(95);
	
	var monthRange = _require5.monthRange;
	
	var _require6 = __webpack_require__(96);
	
	var isMatch = _require6.isMatch;
	
	var _ = __webpack_require__(61);
	
	var ArchivedSessions = React.createClass({
	  displayName: 'ArchivedSessions',
	
	  mixins: [LinkedStateMixin],
	
	  getInitialState: function getInitialState(props) {
	    var _monthRange = monthRange(new Date());
	
	    var startDate = _monthRange[0];
	    var endDate = _monthRange[1];
	
	    this.searchableProps = ['serverIp', 'created', 'sid', 'login'];
	    return { filter: '', colSortDirs: { created: 'ASC' }, startDate: startDate, endDate: endDate };
	  },
	
	  componentWillMount: function componentWillMount() {
	    actions.fetchSessions(this.state.startDate, this.state.endDate);
	  },
	
	  setDatesAndRefetch: function setDatesAndRefetch(startDate, endDate) {
	    actions.fetchSessions(startDate, endDate);
	    this.state.startDate = startDate;
	    this.state.endDate = endDate;
	    this.setState(this.state);
	  },
	
	  onSortChange: function onSortChange(columnKey, sortDir) {
	    var _colSortDirs;
	
	    this.setState(_extends({}, this.state, {
	      colSortDirs: (_colSortDirs = {}, _colSortDirs[columnKey] = sortDir, _colSortDirs)
	    }));
	  },
	
	  onRangePickerChange: function onRangePickerChange(_ref) {
	    var startDate = _ref.startDate;
	    var endDate = _ref.endDate;
	
	    this.setDatesAndRefetch(startDate, endDate);
	  },
	
	  onCalendarNavChange: function onCalendarNavChange(newValue) {
	    var _monthRange2 = monthRange(newValue);
	
	    var startDate = _monthRange2[0];
	    var endDate = _monthRange2[1];
	
	    this.setDatesAndRefetch(startDate, endDate);
	  },
	
	  searchAndFilterCb: function searchAndFilterCb(targetValue, searchValue, propName) {
	    if (propName === 'created') {
	      var displayDate = moment(targetValue).format('l LTS').toLocaleUpperCase();
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
	    var _state = this.state;
	    var startDate = _state.startDate;
	    var endDate = _state.endDate;
	
	    var data = this.props.data.filter(function (item) {
	      return !item.active && moment(item.created).isBetween(startDate, endDate);
	    });
	    data = this.sortAndFilter(data);
	
	    return React.createElement(
	      'div',
	      { className: 'grv-sessions-stored' },
	      React.createElement(
	        'div',
	        { className: 'grv-header' },
	        React.createElement(
	          'h1',
	          null,
	          ' Archived Sessions '
	        ),
	        React.createElement(
	          'div',
	          { className: 'grv-flex' },
	          React.createElement(
	            'div',
	            { className: 'grv-flex-row' },
	            React.createElement(DateRangePicker, { startDate: startDate, endDate: endDate, onChange: this.onRangePickerChange })
	          ),
	          React.createElement(
	            'div',
	            { className: 'grv-flex-row' },
	            React.createElement(CalendarNav, { value: startDate, onValueChange: this.onCalendarNavChange })
	          ),
	          React.createElement(
	            'div',
	            { className: 'grv-flex-row' },
	            React.createElement(
	              'div',
	              { className: 'grv-search' },
	              React.createElement('input', { valueLink: this.linkState('filter'), placeholder: 'Search...', className: 'form-control input-sm' })
	            )
	          )
	        )
	      ),
	      React.createElement(
	        'div',
	        { className: 'grv-content' },
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
	              columnKey: 'created',
	              header: React.createElement(SortHeaderCell, {
	                sortDir: this.state.colSortDirs.created,
	                onSortChange: this.onSortChange,
	                title: 'Created'
	              }),
	              cell: React.createElement(DateCreatedCell, { data: data })
	            }),
	            React.createElement(Column, {
	              header: React.createElement(
	                Cell,
	                null,
	                ' User '
	              ),
	              cell: React.createElement(SingleUserCell, { data: data })
	            })
	          )
	        )
	      )
	    );
	  }
	});
	
	module.exports = ArchivedSessions;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "storedSessionList.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 313:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(4);
	var render = __webpack_require__(221).render;
	
	var _require = __webpack_require__(42);
	
	var Router = _require.Router;
	var Route = _require.Route;
	var Redirect = _require.Redirect;
	var IndexRoute = _require.IndexRoute;
	var browserHistory = _require.browserHistory;
	
	var _require2 = __webpack_require__(305);
	
	var App = _require2.App;
	var Login = _require2.Login;
	var Nodes = _require2.Nodes;
	var Sessions = _require2.Sessions;
	var NewUser = _require2.NewUser;
	var CurrentSessionHost = _require2.CurrentSessionHost;
	var NotFound = _require2.NotFound;
	
	var _require3 = __webpack_require__(116);
	
	var ensureUser = _require3.ensureUser;
	
	var auth = __webpack_require__(94);
	var session = __webpack_require__(26);
	var cfg = __webpack_require__(11);
	
	__webpack_require__(281);
	
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

/***/ 413:
/***/ function(module, exports) {

	module.exports = Terminal;

/***/ }

});
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJzb3VyY2VzIjpbIndlYnBhY2s6Ly8vLi9zcmMvYXBwL3JlYWN0b3IuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb25maWcuanMiLCJ3ZWJwYWNrOi8vLy4vfi9rZXltaXJyb3IvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9zZXNzaW9uLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvc2VydmljZXMvYXBpLmpzIiwid2VicGFjazovLy9leHRlcm5hbCBcImpRdWVyeVwiIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9nZXR0ZXJzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9jb25zdGFudHMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy91c2VyL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL3RhYmxlLmpzeCIsIndlYnBhY2s6Ly8vZXh0ZXJuYWwgXCJfXCIiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2F1dGguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21tb24vZGF0ZVV0aWxzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tbW9uL29iamVjdFV0aWxzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tbW9uL3R0eS5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvYWN0aW9uVHlwZXMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGl2ZVRlcm1TdG9yZS5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvYXBwL2FjdGlvblR5cGVzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hcHAvYXBwU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvYWN0aW9uVHlwZXMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvZGlhbG9nU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2ludml0ZVN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvbm9kZVN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2FjdGlvblR5cGVzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvYWN0aW9uVHlwZXMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL3Nlc3Npb25TdG9yZS5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvdXNlci9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvdXNlci9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy91c2VyL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy91c2VyL3VzZXJTdG9yZS5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvY3VycmVudFNlc3Npb24vc2Vzc2lvbkxlZnRQYW5lbC5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2RhdGVQaWNrZXIuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9lcnJvclBhZ2UuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9nb29nbGVBdXRoTG9nby5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL25vZGVzL25vZGVMaXN0LmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvc2VsZWN0Tm9kZURpYWxvZy5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL3Nlc3Npb25zL2xpc3RJdGVtcy5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL3Rlcm1pbmFsLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbW1vbi9wYXR0ZXJuVXRpbHMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21tb24vdHR5UGxheWVyLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hcHAvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvYXBwL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FwcC9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvZGlhbG9ncy9nZXR0ZXJzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9kaWFsb2dzL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9nZXR0ZXJzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL3Jlc3RBcGlTdG9yZS5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL3V0aWxzLmpzIiwid2VicGFjazovLy8uL34vZXZlbnRzL2V2ZW50cy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvYXBwLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvY3VycmVudFNlc3Npb24vYWN0aXZlU2Vzc2lvbi5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2N1cnJlbnRTZXNzaW9uL2V2ZW50U3RyZWFtZXIuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9jdXJyZW50U2Vzc2lvbi9tYWluLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvY3VycmVudFNlc3Npb24vc2Vzc2lvblBsYXllci5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2luZGV4LmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvbG9naW4uanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9uYXZMZWZ0QmFyLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvbmV3VXNlci5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL25vZGVzL21haW4uanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9zZXNzaW9ucy9hY3RpdmVTZXNzaW9uTGlzdC5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL3Nlc3Npb25zL21haW4uanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9zZXNzaW9ucy9zdG9yZWRTZXNzaW9uTGlzdC5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9pbmRleC5qc3giLCJ3ZWJwYWNrOi8vL2V4dGVybmFsIFwiVGVybWluYWxcIiJdLCJuYW1lcyI6W10sIm1hcHBpbmdzIjoiOzs7Ozs7Ozs7Ozs7Ozs7OztzQ0FBd0IsRUFBWTs7QUFFcEMsS0FBTSxPQUFPLEdBQUcsdUJBQVk7QUFDMUIsUUFBSyxFQUFFLElBQUk7RUFDWixDQUFDOztBQUVGLE9BQU0sQ0FBQyxPQUFPLEdBQUcsT0FBTyxDQUFDOztzQkFFVixPQUFPOzs7Ozs7Ozs7Ozs7Z0JDUkEsbUJBQU8sQ0FBQyxHQUF5QixDQUFDOztLQUFuRCxhQUFhLFlBQWIsYUFBYTs7QUFFbEIsS0FBSSxHQUFHLEdBQUc7O0FBRVIsVUFBTyxFQUFFLE1BQU0sQ0FBQyxRQUFRLENBQUMsTUFBTTs7QUFFL0IsVUFBTyxFQUFFLGlFQUFpRTs7QUFFMUUsTUFBRyxFQUFFO0FBQ0gsbUJBQWMsRUFBQywyQkFBMkI7QUFDMUMsY0FBUyxFQUFFLGtDQUFrQztBQUM3QyxnQkFBVyxFQUFFLHFCQUFxQjtBQUNsQyxvQkFBZSxFQUFFLDBDQUEwQztBQUMzRCxlQUFVLEVBQUUsdUNBQXVDO0FBQ25ELG1CQUFjLEVBQUUsa0JBQWtCO0FBQ2xDLGlCQUFZLEVBQUUsdUVBQXVFO0FBQ3JGLDBCQUFxQixFQUFFLHNEQUFzRDs7QUFFN0UsNEJBQXVCLEVBQUUsaUNBQUMsSUFBaUIsRUFBRztXQUFuQixHQUFHLEdBQUosSUFBaUIsQ0FBaEIsR0FBRztXQUFF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QUFDQSxrQkFBaUI7QUFDakI7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7O0FBRUE7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBLG9CQUFtQixTQUFTO0FBQzVCO0FBQ0E7QUFDQTtBQUNBLElBQUc7QUFDSDtBQUNBO0FBQ0EsZ0JBQWUsU0FBUztBQUN4Qjs7QUFFQTtBQUNBO0FBQ0EsZ0JBQWUsU0FBUztBQUN4QjtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0EsTUFBSztBQUNMO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7O0FBRUE7QUFDQTtBQUNBOztBQUVBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQSxJQUFHO0FBQ0gscUJBQW9CLFNBQVM7QUFDN0I7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0EsTUFBSztBQUNMO0FBQ0E7O0FBRUE7QUFDQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7O0FBRUE7QUFDQTtBQUNBLElBQUc7QUFDSDtBQUNBO0FBQ0E7QUFDQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBOzs7Ozs7Ozs7Ozs7QUM1U0EsS0FBSSxLQUFLLEdBQUcsbUJBQU8sQ0FBQyxDQUFPLENBQUMsQ0FBQztBQUM3QixLQUFJLFVBQVUsR0FBRyxtQkFBTyxDQUFDLEdBQWMsQ0FBQyxDQUFDO0FBQ3pDLEtBQUksR0FBRyxHQUFHLG1CQUFPLENBQUMsRUFBWSxDQUFDLENBQUM7QUFDaEMsS0FBSSxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxDQUFhLENBQUMsQ0FBQzs7Z0JBQ1osbUJBQU8sQ0FBQyxHQUFpQixDQUFDOztLQUE5QyxPQUFPLFlBQVAsT0FBTztLQUFFLE9BQU8sWUFBUCxPQUFPOztBQUNyQixLQUFJLGdCQUFnQixHQUFHLG1CQUFPLENBQUMsR0FBd0IsQ0FBQyxDQUFDOztBQUV6RCxLQUFJLEdBQUcsR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFFMUIsU0FBTSxFQUFFLENBQUMsT0FBTyxDQUFDLFVBQVUsQ0FBQzs7QUFFNUIsa0JBQWUsNkJBQUc7QUFDaEIsWUFBTztBQUNMLFVBQUcsRUFBRSxPQUFPLENBQUMsUUFBUTtNQUN0QjtJQUNGOztBQUVELHFCQUFrQixnQ0FBRTtBQUNsQixZQUFPLENBQUMsT0FBTyxFQUFFLENBQUM7QUFDbEIsU0FBSSxDQUFDLGVBQWUsR0FBRyxXQUFXLENBQUMsT0FBTyxDQUFDLHFCQUFxQixFQUFFLEtBQUssQ0FBQyxDQUFDO0lBQzFFOztBQUVELHVCQUFvQixFQUFFLGdDQUFXO0FBQy9CLGtCQUFhLENBQUMsSUFBSSxDQUFDLGVBQWUsQ0FBQyxDQUFDO0lBQ3JDOztBQUVELFNBQU0sRUFBRSxrQkFBVztBQUNqQixTQUFHLElBQUksQ0FBQyxLQUFLLENBQUMsR0FBRyxDQUFDLGNBQWMsRUFBQztBQUMvQixjQUFPLElBQUksQ0FBQztNQUNiOztBQUVELFlBQ0U7O1NBQUssU0FBUyxFQUFDLGdDQUFnQztPQUM3QyxvQkFBQyxnQkFBZ0IsT0FBRTtPQUNsQixJQUFJLENBQUMsS0FBSyxDQUFDLGtCQUFrQjtPQUM5QixvQkFBQyxVQUFVLE9BQUU7T0FDWixJQUFJLENBQUMsS0FBSyxDQUFDLFFBQVE7TUFDaEIsQ0FDTjtJQUNIO0VBQ0YsQ0FBQzs7QUFFRixPQUFNLENBQUMsT0FBTyxHQUFHLEdBQUcsQzs7Ozs7Ozs7Ozs7Ozs7O0FDMUNwQixLQUFJLEtBQUssR0FBRyxtQkFBTyxDQUFDLENBQU8sQ0FBQyxDQUFDO0FBQzdCLEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsQ0FBYSxDQUFDLENBQUM7O2dCQUNaLG1CQUFPLENBQUMsRUFBNkIsQ0FBQzs7S0FBMUQsT0FBTyxZQUFQLE9BQU87S0FBRSxPQUFPLFlBQVAsT0FBTzs7aUJBQ1UsbUJBQU8sQ0FBQyxFQUEyQixDQUFDOztLQUE5RCxzQkFBc0IsYUFBdEIsc0JBQXNCOztBQUMzQixLQUFJLEdBQUcsR0FBRyxtQkFBTyxDQUFDLEVBQWdCLENBQUMsQ0FBQztBQUNwQyxLQUFJLFdBQVcsR0FBRyxtQkFBTyxDQUFDLEdBQW1CLENBQUMsQ0FBQztBQUMvQyxLQUFJLGFBQWEsR0FBRyxtQkFBTyxDQUFDLEdBQXFCLENBQUMsQ0FBQztBQUNuRCxLQUFJLGdCQUFnQixHQUFHLG1CQUFPLENBQUMsR0FBb0IsQ0FBQyxDQUFDOztpQkFDRCxtQkFBTyxDQUFDLEVBQTZCLENBQUM7O0tBQXJGLG9CQUFvQixhQUFwQixvQkFBb0I7S0FBRSxxQkFBcUIsYUFBckIscUJBQXFCOztBQUNoRCxLQUFJLGdCQUFnQixHQUFHLG1CQUFPLENBQUMsR0FBMkIsQ0FBQyxDQUFDOztBQUU1RCxLQUFJLGFBQWEsR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFFcEMsdUJBQW9CLGtDQUFFO0FBQ3BCLDBCQUFxQixFQUFFLENBQUM7SUFDekI7O0FBRUQsU0FBTSxFQUFFLGtCQUFXO2dDQUNnQixJQUFJLENBQUMsS0FBSyxDQUFDLGFBQWE7U0FBcEQsS0FBSyx3QkFBTCxLQUFLO1NBQUUsT0FBTyx3QkFBUCxPQUFPO1NBQUUsUUFBUSx3QkFBUixRQUFROztBQUM3QixTQUFJLGVBQWUsR0FBRyxFQUFFLENBQUM7QUFDekIsU0FBRyxRQUFRLEVBQUM7QUFDVixXQUFJLFFBQVEsR0FBRyxPQUFPLENBQUMsUUFBUSxDQUFDLHNCQUFzQixDQUFDLFFBQVEsQ0FBQyxDQUFDLENBQUM7QUFDbEUsc0JBQWUsR0FBTSxLQUFLLFNBQUksUUFBVSxDQUFDO01BQzFDOztBQUVELFlBQ0M7O1NBQUssU0FBUyxFQUFDLHFCQUFxQjtPQUNsQyxvQkFBQyxnQkFBZ0IsSUFBQyxPQUFPLEVBQUUsT0FBUSxHQUFFO09BQ3JDOztXQUFLLFNBQVMsRUFBQyxpQ0FBaUM7U0FDOUM7OztXQUFLLGVBQWU7VUFBTTtRQUN0QjtPQUNOLG9CQUFDLGFBQWEsRUFBSyxJQUFJLENBQUMsS0FBSyxDQUFDLGFBQWEsQ0FBSTtNQUMzQyxDQUNKO0lBQ0o7RUFDRixDQUFDLENBQUM7O0FBRUgsS0FBSSxhQUFhLEdBQUcsS0FBSyxDQUFDLFdBQVcsQ0FBQzs7O0FBRXBDLGtCQUFlLDZCQUFHOzs7QUFDaEIsU0FBSSxDQUFDLEdBQUcsR0FBRyxJQUFJLEdBQUcsQ0FBQyxJQUFJLENBQUMsS0FBSyxDQUFDO0FBQzlCLFNBQUksQ0FBQyxHQUFHLENBQUMsRUFBRSxDQUFDLE1BQU0sRUFBRTtjQUFLLE1BQUssUUFBUSxjQUFNLE1BQUssS0FBSyxJQUFFLFdBQVcsRUFBRSxJQUFJLElBQUc7TUFBQSxDQUFDLENBQUM7O2tCQUV0RCxJQUFJLENBQUMsS0FBSztTQUE3QixRQUFRLFVBQVIsUUFBUTtTQUFFLEtBQUssVUFBTCxLQUFLOztBQUNwQixZQUFPLEVBQUMsUUFBUSxFQUFSLFFBQVEsRUFBRSxLQUFLLEVBQUwsS0FBSyxFQUFFLFdBQVcsRUFBRSxLQUFLLEVBQUMsQ0FBQztJQUM5Qzs7QUFFRCxvQkFBaUIsK0JBQUU7O0FBRWpCLHFCQUFnQixDQUFDLHNCQUFzQixHQUFHLElBQUksQ0FBQyx5QkFBeUIsQ0FBQyxJQUFJLENBQUMsSUFBSSxDQUFDLENBQUM7SUFDckY7O0FBRUQsdUJBQW9CLGtDQUFHO0FBQ3JCLHFCQUFnQixDQUFDLHNCQUFzQixHQUFHLElBQUksQ0FBQztBQUMvQyxTQUFJLENBQUMsR0FBRyxDQUFDLFVBQVUsRUFBRSxDQUFDO0lBQ3ZCOztBQUVELDRCQUF5QixxQ0FBQyxTQUFTLEVBQUM7U0FDN0IsUUFBUSxHQUFJLFNBQVMsQ0FBckIsUUFBUTs7QUFDYixTQUFHLFFBQVEsSUFBSSxRQUFRLEtBQUssSUFBSSxDQUFDLEtBQUssQ0FBQyxRQUFRLEVBQUM7QUFDOUMsV0FBSSxDQUFDLEdBQUcsQ0FBQyxTQUFTLENBQUMsRUFBQyxRQUFRLEVBQVIsUUFBUSxFQUFDLENBQUMsQ0FBQztBQUMvQixXQUFJLENBQUMsSUFBSSxDQUFDLGVBQWUsQ0FBQyxJQUFJLENBQUMsS0FBSyxFQUFFLENBQUM7QUFDdkMsV0FBSSxDQUFDLFFBQVEsY0FBSyxJQUFJLENBQUMsS0FBSyxJQUFFLFFBQVEsRUFBUixRQUFRLElBQUcsQ0FBQztNQUMzQztJQUNGOztBQUVELFNBQU0sb0JBQUc7QUFDUCxZQUNFOztTQUFLLEtBQUssRUFBRSxFQUFDLE1BQU0sRUFBRSxNQUFNLEVBQUU7T0FDM0Isb0JBQUMsV0FBVyxJQUFDLEdBQUcsRUFBQyxpQkFBaUIsRUFBQyxHQUFHLEVBQUUsSUFBSSxDQUFDLEdBQUksRUFBQyxJQUFJLEVBQUUsSUFBSSxDQUFDLEtBQUssQ0FBQyxJQUFLLEVBQUMsSUFBSSxFQUFFLElBQUksQ0FBQyxLQUFLLENBQUMsSUFBSyxHQUFHO09BQ2hHLElBQUksQ0FBQyxLQUFLLENBQUMsV0FBVyxHQUFHLG9CQUFDLGFBQWEsSUFBQyxHQUFHLEVBQUUsSUFBSSxDQUFDLEtBQUssQ0FBQyxHQUFJLEdBQUUsR0FBRyxJQUFJO01BQ25FLENBQ1A7SUFDRjtFQUNGLENBQUMsQ0FBQzs7QUFFSCxPQUFNLENBQUMsT0FBTyxHQUFHLGFBQWEsQzs7Ozs7Ozs7Ozs7Ozs7QUM1RTlCLEtBQUksR0FBRyxHQUFHLG1CQUFPLENBQUMsRUFBWSxDQUFDLENBQUM7QUFDaEMsS0FBSSxLQUFLLEdBQUcsbUJBQU8sQ0FBQyxDQUFPLENBQUMsQ0FBQztBQUM3QixLQUFJLE9BQU8sR0FBRyxtQkFBTyxDQUFDLEVBQWEsQ0FBQyxDQUFDOztnQkFDZixtQkFBTyxDQUFDLEVBQThCLENBQUM7O0tBQXhELGFBQWEsWUFBYixhQUFhOztBQUVsQixLQUFJLGFBQWEsR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFDcEMsb0JBQWlCLCtCQUFHO1NBQ2IsR0FBRyxHQUFJLElBQUksQ0FBQyxLQUFLLENBQWpCLEdBQUc7O2dDQUNNLE9BQU8sQ0FBQyxXQUFXLEVBQUU7O1NBQTlCLEtBQUssd0JBQUwsS0FBSzs7QUFDVixTQUFJLE9BQU8sR0FBRyxHQUFHLENBQUMsR0FBRyxDQUFDLHFCQUFxQixDQUFDLEtBQUssRUFBRSxHQUFHLENBQUMsQ0FBQzs7QUFFeEQsU0FBSSxDQUFDLE1BQU0sR0FBRyxJQUFJLFNBQVMsQ0FBQyxPQUFPLEVBQUUsT0FBTyxDQUFDLENBQUM7QUFDOUMsU0FBSSxDQUFDLE1BQU0sQ0FBQyxTQUFTLEdBQUcsVUFBQyxLQUFLLEVBQUs7QUFDakMsV0FDQTtBQUNFLGFBQUksSUFBSSxHQUFHLElBQUksQ0FBQyxLQUFLLENBQUMsS0FBSyxDQUFDLElBQUksQ0FBQyxDQUFDO0FBQ2xDLHNCQUFhLENBQUMsSUFBSSxDQUFDLE9BQU8sQ0FBQyxDQUFDO1FBQzdCLENBQ0QsT0FBTSxHQUFHLEVBQUM7QUFDUixnQkFBTyxDQUFDLEdBQUcsQ0FBQyxtQ0FBbUMsQ0FBQyxDQUFDO1FBQ2xEO01BRUYsQ0FBQztBQUNGLFNBQUksQ0FBQyxNQUFNLENBQUMsT0FBTyxHQUFHLFlBQU0sRUFBRSxDQUFDO0lBQ2hDOztBQUVELHVCQUFvQixrQ0FBRztBQUNyQixTQUFJLENBQUMsTUFBTSxDQUFDLEtBQUssRUFBRSxDQUFDO0lBQ3JCOztBQUVELHdCQUFxQixtQ0FBRztBQUN0QixZQUFPLEtBQUssQ0FBQztJQUNkOztBQUVELFNBQU0sb0JBQUc7QUFDUCxZQUFPLElBQUksQ0FBQztJQUNiO0VBQ0YsQ0FBQyxDQUFDOztzQkFFWSxhQUFhOzs7Ozs7Ozs7Ozs7OztBQ3ZDNUIsS0FBSSxLQUFLLEdBQUcsbUJBQU8sQ0FBQyxDQUFPLENBQUMsQ0FBQzs7Z0JBQ0osbUJBQU8sQ0FBQyxFQUE2QixDQUFDOztLQUExRCxPQUFPLFlBQVAsT0FBTztLQUFFLE9BQU8sWUFBUCxPQUFPOztBQUNyQixLQUFJLGFBQWEsR0FBRyxtQkFBTyxDQUFDLEdBQXFCLENBQUMsQ0FBQztBQUNuRCxLQUFJLGFBQWEsR0FBRyxtQkFBTyxDQUFDLEdBQXFCLENBQUMsQ0FBQzs7QUFFbkQsS0FBSSxrQkFBa0IsR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFFekMsU0FBTSxFQUFFLENBQUMsT0FBTyxDQUFDLFVBQVUsQ0FBQzs7QUFFNUIsa0JBQWUsNkJBQUc7QUFDaEIsWUFBTztBQUNMLHFCQUFjLEVBQUUsT0FBTyxDQUFDLGFBQWE7TUFDdEM7SUFDRjs7QUFFRCxvQkFBaUIsK0JBQUU7U0FDWCxHQUFHLEdBQUssSUFBSSxDQUFDLEtBQUssQ0FBQyxNQUFNLENBQXpCLEdBQUc7O0FBQ1QsU0FBRyxDQUFDLElBQUksQ0FBQyxLQUFLLENBQUMsY0FBYyxFQUFDO0FBQzVCLGNBQU8sQ0FBQyxXQUFXLENBQUMsR0FBRyxDQUFDLENBQUM7TUFDMUI7SUFDRjs7QUFFRCxTQUFNLEVBQUUsa0JBQVc7QUFDakIsU0FBSSxjQUFjLEdBQUcsSUFBSSxDQUFDLEtBQUssQ0FBQyxjQUFjLENBQUM7QUFDL0MsU0FBRyxDQUFDLGNBQWMsRUFBQztBQUNqQixjQUFPLElBQUksQ0FBQztNQUNiOztBQUVELFNBQUcsY0FBYyxDQUFDLFlBQVksSUFBSSxjQUFjLENBQUMsTUFBTSxFQUFDO0FBQ3RELGNBQU8sb0JBQUMsYUFBYSxJQUFDLGFBQWEsRUFBRSxjQUFlLEdBQUUsQ0FBQztNQUN4RDs7QUFFRCxZQUFPLG9CQUFDLGFBQWEsSUFBQyxhQUFhLEVBQUUsY0FBZSxHQUFFLENBQUM7SUFDeEQ7RUFDRixDQUFDLENBQUM7O0FBRUgsT0FBTSxDQUFDLE9BQU8sR0FBRyxrQkFBa0IsQzs7Ozs7Ozs7Ozs7Ozs7QUNwQ25DLEtBQUksS0FBSyxHQUFHLG1CQUFPLENBQUMsQ0FBTyxDQUFDLENBQUM7QUFDN0IsS0FBSSxXQUFXLEdBQUcsbUJBQU8sQ0FBQyxHQUFjLENBQUMsQ0FBQztBQUMxQyxLQUFJLFNBQVMsR0FBRyxtQkFBTyxDQUFDLEdBQXNCLENBQUM7QUFDL0MsS0FBSSxXQUFXLEdBQUcsbUJBQU8sQ0FBQyxHQUFtQixDQUFDLENBQUM7QUFDL0MsS0FBSSxnQkFBZ0IsR0FBRyxtQkFBTyxDQUFDLEdBQW9CLENBQUMsQ0FBQzs7QUFFckQsS0FBSSxhQUFhLEdBQUcsS0FBSyxDQUFDLFdBQVcsQ0FBQzs7O0FBQ3BDLGlCQUFjLDRCQUFFO0FBQ2QsWUFBTztBQUNMLGFBQU0sRUFBRSxJQUFJLENBQUMsR0FBRyxDQUFDLE1BQU07QUFDdkIsVUFBRyxFQUFFLENBQUM7QUFDTixnQkFBUyxFQUFFLElBQUksQ0FBQyxHQUFHLENBQUMsU0FBUztBQUM3QixjQUFPLEVBQUUsSUFBSSxDQUFDLEdBQUcsQ0FBQyxPQUFPO0FBQ3pCLGNBQU8sRUFBRSxJQUFJLENBQUMsR0FBRyxDQUFDLE1BQU0sR0FBRyxDQUFDO01BQzdCLENBQUM7SUFDSDs7QUFFRCxrQkFBZSw2QkFBRztBQUNoQixTQUFJLEdBQUcsR0FBRyxJQUFJLENBQUMsS0FBSyxDQUFDLGFBQWEsQ0FBQyxHQUFHLENBQUM7QUFDdkMsU0FBSSxDQUFDLEdBQUcsR0FBRyxJQUFJLFNBQVMsQ0FBQyxFQUFDLEdBQUcsRUFBSCxHQUFHLEVBQUMsQ0FBQyxDQUFDO0FBQ2hDLFlBQU8sSUFBSSxDQUFDLGNBQWMsRUFBRSxDQUFDO0lBQzlCOztBQUVELHVCQUFvQixrQ0FBRztBQUNyQixTQUFJLENBQUMsR0FBRyxDQUFDLElBQUksRUFBRSxDQUFDO0FBQ2hCLFNBQUksQ0FBQyxHQUFHLENBQUMsa0JBQWtCLEVBQUUsQ0FBQztJQUMvQjs7QUFFRCxvQkFBaUIsK0JBQUc7OztBQUNsQixTQUFJLENBQUMsR0FBRyxDQUFDLEVBQUUsQ0FBQyxRQUFRLEVBQUUsWUFBSTtBQUN4QixXQUFJLFFBQVEsR0FBRyxNQUFLLGNBQWMsRUFBRSxDQUFDO0FBQ3JDLGFBQUssUUFBUSxDQUFDLFFBQVEsQ0FBQyxDQUFDO01BQ3pCLENBQUMsQ0FBQztJQUNKOztBQUVELGlCQUFjLDRCQUFFO0FBQ2QsU0FBRyxJQUFJLENBQUMsS0FBSyxDQUFDLFNBQVMsRUFBQztBQUN0QixXQUFJLENBQUMsR0FBRyxDQUFDLElBQUksRUFBRSxDQUFDO01BQ2pCLE1BQUk7QUFDSCxXQUFJLENBQUMsR0FBRyxDQUFDLElBQUksRUFBRSxDQUFDO01BQ2pCO0lBQ0Y7O0FBRUQsT0FBSSxnQkFBQyxLQUFLLEVBQUM7QUFDVCxTQUFJLENBQUMsR0FBRyxDQUFDLElBQUksQ0FBQyxLQUFLLENBQUMsQ0FBQztJQUN0Qjs7QUFFRCxpQkFBYyw0QkFBRTtBQUNkLFNBQUksQ0FBQyxHQUFHLENBQUMsSUFBSSxFQUFFLENBQUM7SUFDakI7O0FBRUQsZ0JBQWEseUJBQUMsS0FBSyxFQUFDO0FBQ2xCLFNBQUksQ0FBQyxHQUFHLENBQUMsSUFBSSxFQUFFLENBQUM7QUFDaEIsU0FBSSxDQUFDLEdBQUcsQ0FBQyxJQUFJLENBQUMsS0FBSyxDQUFDLENBQUM7SUFDdEI7O0FBRUQsU0FBTSxFQUFFLGtCQUFXO1NBQ1osU0FBUyxHQUFJLElBQUksQ0FBQyxLQUFLLENBQXZCLFNBQVM7O0FBRWQsWUFDQzs7U0FBSyxTQUFTLEVBQUMsd0NBQXdDO09BQ3JELG9CQUFDLGdCQUFnQixPQUFFO09BQ25CLG9CQUFDLFdBQVcsSUFBQyxHQUFHLEVBQUMsTUFBTSxFQUFDLEdBQUcsRUFBRSxJQUFJLENBQUMsR0FBSSxFQUFDLElBQUksRUFBQyxHQUFHLEVBQUMsSUFBSSxFQUFDLEdBQUcsR0FBRztPQUMzRCxvQkFBQyxXQUFXO0FBQ1QsWUFBRyxFQUFFLElBQUksQ0FBQyxLQUFLLENBQUMsR0FBSTtBQUNwQixZQUFHLEVBQUUsSUFBSSxDQUFDLEtBQUssQ0FBQyxNQUFPO0FBQ3ZCLGNBQUssRUFBRSxJQUFJLENBQUMsS0FBSyxDQUFDLE9BQVE7QUFDMUIsc0JBQWEsRUFBRSxJQUFJLENBQUMsYUFBYztBQUNsQyx1QkFBYyxFQUFFLElBQUksQ0FBQyxjQUFlO0FBQ3BDLHFCQUFZLEVBQUUsQ0FBRTtBQUNoQixpQkFBUTtBQUNSLGtCQUFTLEVBQUMsWUFBWSxHQUNYO09BQ2Q7O1dBQVEsU0FBUyxFQUFDLEtBQUssRUFBQyxPQUFPLEVBQUUsSUFBSSxDQUFDLGNBQWU7U0FDakQsU0FBUyxHQUFHLDJCQUFHLFNBQVMsRUFBQyxZQUFZLEdBQUssR0FBSSwyQkFBRyxTQUFTLEVBQUMsWUFBWSxHQUFLO1FBQ3ZFO01BQ0wsQ0FDSjtJQUNKO0VBQ0YsQ0FBQyxDQUFDOztzQkFFWSxhQUFhOzs7Ozs7Ozs7Ozs7OztBQ2pGNUIsT0FBTSxDQUFDLE9BQU8sQ0FBQyxHQUFHLEdBQUcsbUJBQU8sQ0FBQyxHQUFXLENBQUMsQ0FBQztBQUMxQyxPQUFNLENBQUMsT0FBTyxDQUFDLEtBQUssR0FBRyxtQkFBTyxDQUFDLEdBQWEsQ0FBQyxDQUFDO0FBQzlDLE9BQU0sQ0FBQyxPQUFPLENBQUMsT0FBTyxHQUFHLG1CQUFPLENBQUMsR0FBZSxDQUFDLENBQUM7QUFDbEQsT0FBTSxDQUFDLE9BQU8sQ0FBQyxLQUFLLEdBQUcsbUJBQU8sQ0FBQyxHQUFrQixDQUFDLENBQUM7QUFDbkQsT0FBTSxDQUFDLE9BQU8sQ0FBQyxRQUFRLEdBQUcsbUJBQU8sQ0FBQyxHQUFxQixDQUFDLENBQUM7QUFDekQsT0FBTSxDQUFDLE9BQU8sQ0FBQyxrQkFBa0IsR0FBRyxtQkFBTyxDQUFDLEdBQTJCLENBQUMsQ0FBQztBQUN6RSxPQUFNLENBQUMsT0FBTyxDQUFDLFFBQVEsR0FBRyxtQkFBTyxDQUFDLEdBQWlCLENBQUMsQ0FBQyxRQUFRLEM7Ozs7Ozs7Ozs7Ozs7QUNON0QsS0FBSSxLQUFLLEdBQUcsbUJBQU8sQ0FBQyxDQUFPLENBQUMsQ0FBQztBQUM3QixLQUFJLENBQUMsR0FBRyxtQkFBTyxDQUFDLEVBQVEsQ0FBQyxDQUFDO0FBQzFCLEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsQ0FBYSxDQUFDLENBQUM7QUFDckMsS0FBSSxnQkFBZ0IsR0FBRyxtQkFBTyxDQUFDLEVBQWlDLENBQUMsQ0FBQzs7Z0JBQ3pDLG1CQUFPLENBQUMsR0FBa0IsQ0FBQzs7S0FBL0MsT0FBTyxZQUFQLE9BQU87S0FBRSxPQUFPLFlBQVAsT0FBTzs7QUFDckIsS0FBSSxjQUFjLEdBQUcsbUJBQU8sQ0FBQyxHQUFrQixDQUFDLENBQUM7QUFDakQsS0FBSSxHQUFHLEdBQUcsbUJBQU8sQ0FBQyxFQUFZLENBQUMsQ0FBQzs7QUFFaEMsS0FBSSxjQUFjLEdBQUcsS0FBSyxDQUFDLFdBQVcsQ0FBQzs7O0FBRXJDLFNBQU0sRUFBRSxDQUFDLGdCQUFnQixDQUFDOztBQUUxQixrQkFBZSw2QkFBRztBQUNoQixZQUFPO0FBQ0wsV0FBSSxFQUFFLEVBQUU7QUFDUixlQUFRLEVBQUUsRUFBRTtBQUNaLFlBQUssRUFBRSxFQUFFO01BQ1Y7SUFDRjs7QUFFRCxVQUFPLEVBQUUsaUJBQVMsQ0FBQyxFQUFFO0FBQ25CLE1BQUMsQ0FBQyxjQUFjLEVBQUUsQ0FBQztBQUNuQixTQUFJLElBQUksQ0FBQyxPQUFPLEVBQUUsRUFBRTtBQUNsQixXQUFJLENBQUMsS0FBSyxDQUFDLE9BQU8sQ0FBQyxJQUFJLENBQUMsS0FBSyxDQUFDLENBQUM7TUFDaEM7SUFDRjs7QUFFRCxVQUFPLEVBQUUsbUJBQVc7QUFDbEIsU0FBSSxLQUFLLEdBQUcsQ0FBQyxDQUFDLElBQUksQ0FBQyxJQUFJLENBQUMsSUFBSSxDQUFDLENBQUM7QUFDOUIsWUFBTyxLQUFLLENBQUMsTUFBTSxLQUFLLENBQUMsSUFBSSxLQUFLLENBQUMsS0FBSyxFQUFFLENBQUM7SUFDNUM7O0FBRUQsU0FBTSxvQkFBRzt5QkFDa0MsSUFBSSxDQUFDLEtBQUssQ0FBQyxNQUFNO1NBQXJELFlBQVksaUJBQVosWUFBWTtTQUFFLFFBQVEsaUJBQVIsUUFBUTtTQUFFLE9BQU8saUJBQVAsT0FBTzs7QUFFcEMsWUFDRTs7U0FBTSxHQUFHLEVBQUMsTUFBTSxFQUFDLFNBQVMsRUFBQyxzQkFBc0I7T0FDL0M7Ozs7UUFBOEI7T0FDOUI7O1dBQUssU0FBUyxFQUFDLEVBQUU7U0FDZjs7YUFBSyxTQUFTLEVBQUMsWUFBWTtXQUN6QiwrQkFBTyxTQUFTLFFBQUMsU0FBUyxFQUFFLElBQUksQ0FBQyxTQUFTLENBQUMsTUFBTSxDQUFFLEVBQUMsU0FBUyxFQUFDLHVCQUF1QixFQUFDLFdBQVcsRUFBQyxXQUFXLEVBQUMsSUFBSSxFQUFDLFVBQVUsR0FBRztVQUM1SDtTQUNOOzthQUFLLFNBQVMsRUFBQyxZQUFZO1dBQ3pCLCtCQUFPLFNBQVMsRUFBRSxJQUFJLENBQUMsU0FBUyxDQUFDLFVBQVUsQ0FBRSxFQUFDLElBQUksRUFBQyxVQUFVLEVBQUMsSUFBSSxFQUFDLFVBQVUsRUFBQyxTQUFTLEVBQUMsdUJBQXVCLEVBQUMsV0FBVyxFQUFDLFVBQVUsR0FBRTtVQUNwSTtTQUNOOzthQUFLLFNBQVMsRUFBQyxZQUFZO1dBQ3pCLCtCQUFPLFNBQVMsRUFBRSxJQUFJLENBQUMsU0FBUyxDQUFDLE9BQU8sQ0FBRSxFQUFDLFNBQVMsRUFBQyx1QkFBdUIsRUFBQyxJQUFJLEVBQUMsT0FBTyxFQUFDLFdBQVcsRUFBQyx5Q0FBeUMsR0FBRTtVQUM3STtTQUNOOzthQUFRLE9BQU8sRUFBRSxJQUFJLENBQUMsT0FBUSxFQUFDLFFBQVEsRUFBRSxZQUFhLEVBQUMsSUFBSSxFQUFDLFFBQVEsRUFBQyxTQUFTLEVBQUMsc0NBQXNDOztVQUFlO1NBQ2xJLFFBQVEsR0FBSTs7YUFBTyxTQUFTLEVBQUMsT0FBTztXQUFFLE9BQU87VUFBUyxHQUFJLElBQUk7UUFDNUQ7TUFDRCxDQUNQO0lBQ0g7RUFDRixDQUFDOztBQUVGLEtBQUksS0FBSyxHQUFHLEtBQUssQ0FBQyxXQUFXLENBQUM7OztBQUU1QixTQUFNLEVBQUUsQ0FBQyxPQUFPLENBQUMsVUFBVSxDQUFDOztBQUU1QixrQkFBZSw2QkFBRztBQUNoQixZQUFPO0FBQ0wsYUFBTSxFQUFFLE9BQU8sQ0FBQyxXQUFXO01BQzVCO0lBQ0Y7O0FBRUQsVUFBTyxtQkFBQyxTQUFTLEVBQUM7QUFDaEIsU0FBSSxHQUFHLEdBQUcsSUFBSSxDQUFDLEtBQUssQ0FBQyxRQUFRLENBQUM7QUFDOUIsU0FBSSxRQUFRLEdBQUcsR0FBRyxDQUFDLE1BQU0sQ0FBQyxHQUFHLENBQUM7O0FBRTlCLFNBQUcsR0FBRyxDQUFDLEtBQUssSUFBSSxHQUFHLENBQUMsS0FBSyxDQUFDLFVBQVUsRUFBQztBQUNuQyxlQUFRLEdBQUcsR0FBRyxDQUFDLEtBQUssQ0FBQyxVQUFVLENBQUM7TUFDakM7O0FBRUQsWUFBTyxDQUFDLEtBQUssQ0FBQyxTQUFTLEVBQUUsUUFBUSxDQUFDLENBQUM7SUFDcEM7O0FBRUQsU0FBTSxvQkFBRztBQUNQLFlBQ0U7O1NBQUssU0FBUyxFQUFDLHVCQUF1QjtPQUNwQyw2QkFBSyxTQUFTLEVBQUMsZUFBZSxHQUFPO09BQ3JDOztXQUFLLFNBQVMsRUFBQyxzQkFBc0I7U0FDbkM7O2FBQUssU0FBUyxFQUFDLGlCQUFpQjtXQUM5QixvQkFBQyxjQUFjLElBQUMsTUFBTSxFQUFFLElBQUksQ0FBQyxLQUFLLENBQUMsTUFBTyxFQUFDLE9BQU8sRUFBRSxJQUFJLENBQUMsT0FBUSxHQUFFO1dBQ25FLG9CQUFDLGNBQWMsT0FBRTtXQUNqQjs7ZUFBSyxTQUFTLEVBQUMsZ0JBQWdCO2FBQzdCLDJCQUFHLFNBQVMsRUFBQyxnQkFBZ0IsR0FBSzthQUNsQzs7OztjQUFnRDthQUNoRDs7OztjQUE2RDtZQUN6RDtVQUNGO1FBQ0Y7TUFDRixDQUNOO0lBQ0g7RUFDRixDQUFDLENBQUM7O0FBRUgsT0FBTSxDQUFDLE9BQU8sR0FBRyxLQUFLLEM7Ozs7Ozs7Ozs7Ozs7QUNqR3RCLEtBQUksS0FBSyxHQUFHLG1CQUFPLENBQUMsQ0FBTyxDQUFDLENBQUM7O2dCQUNRLG1CQUFPLENBQUMsRUFBYyxDQUFDOztLQUF0RCxNQUFNLFlBQU4sTUFBTTtLQUFFLFNBQVMsWUFBVCxTQUFTO0tBQUUsT0FBTyxZQUFQLE9BQU87O0FBQ2hDLEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsRUFBMEIsQ0FBQyxDQUFDO0FBQ2xELEtBQUksR0FBRyxHQUFHLG1CQUFPLENBQUMsRUFBWSxDQUFDLENBQUM7O0FBRWhDLEtBQUksU0FBUyxHQUFHLENBQ2QsRUFBQyxJQUFJLEVBQUUsWUFBWSxFQUFFLEVBQUUsRUFBRSxHQUFHLENBQUMsTUFBTSxDQUFDLEtBQUssRUFBRSxLQUFLLEVBQUUsT0FBTyxFQUFDLEVBQzFELEVBQUMsSUFBSSxFQUFFLGVBQWUsRUFBRSxFQUFFLEVBQUUsR0FBRyxDQUFDLE1BQU0sQ0FBQyxRQUFRLEVBQUUsS0FBSyxFQUFFLFVBQVUsRUFBQyxDQUNwRSxDQUFDOztBQUVGLEtBQUksVUFBVSxHQUFHLEtBQUssQ0FBQyxXQUFXLENBQUM7OztBQUVqQyxTQUFNLEVBQUUsa0JBQVU7OztBQUNoQixTQUFJLEtBQUssR0FBRyxTQUFTLENBQUMsR0FBRyxDQUFDLFVBQUMsQ0FBQyxFQUFFLEtBQUssRUFBRztBQUNwQyxXQUFJLFNBQVMsR0FBRyxNQUFLLE9BQU8sQ0FBQyxNQUFNLENBQUMsUUFBUSxDQUFDLENBQUMsQ0FBQyxFQUFFLENBQUMsR0FBRyxRQUFRLEdBQUcsRUFBRSxDQUFDO0FBQ25FLGNBQ0U7O1dBQUksR0FBRyxFQUFFLEtBQU0sRUFBQyxTQUFTLEVBQUUsU0FBVSxFQUFDLEtBQUssRUFBRSxDQUFDLENBQUMsS0FBTTtTQUNuRDtBQUFDLG9CQUFTO2FBQUMsRUFBRSxFQUFFLENBQUMsQ0FBQyxFQUFHO1dBQ2xCLDJCQUFHLFNBQVMsRUFBRSxDQUFDLENBQUMsSUFBSyxHQUFHO1VBQ2Q7UUFDVCxDQUNMO01BQ0gsQ0FBQyxDQUFDOztBQUVILFVBQUssQ0FBQyxJQUFJLENBQ1I7O1NBQUksR0FBRyxFQUFFLEtBQUssQ0FBQyxNQUFPLEVBQUMsS0FBSyxFQUFDLE1BQU07T0FDakM7O1dBQUcsSUFBSSxFQUFFLEdBQUcsQ0FBQyxPQUFRLEVBQUMsTUFBTSxFQUFDLFFBQVE7U0FDbkMsMkJBQUcsU0FBUyxFQUFDLGdCQUFnQixHQUFHO1FBQzlCO01BQ0QsQ0FBRSxDQUFDOztBQUVWLFVBQUssQ0FBQyxJQUFJLENBQ1I7O1NBQUksR0FBRyxFQUFFLEtBQUssQ0FBQyxNQUFPLEVBQUMsS0FBSyxFQUFDLFFBQVE7T0FDbkM7O1dBQUcsSUFBSSxFQUFFLEdBQUcsQ0FBQyxNQUFNLENBQUMsTUFBTztTQUN6QiwyQkFBRyxTQUFTLEVBQUMsZ0JBQWdCLEdBQUs7UUFDaEM7TUFDRCxDQUNMLENBQUM7O0FBRUgsWUFDRTs7U0FBSyxTQUFTLEVBQUMsd0JBQXdCLEVBQUMsSUFBSSxFQUFDLFlBQVk7T0FDdkQ7O1dBQUksU0FBUyxFQUFDLGlCQUFpQixFQUFDLEVBQUUsRUFBQyxXQUFXO1NBQzVDOzthQUFJLEtBQUssRUFBQyxjQUFjO1dBQUM7O2VBQUssU0FBUyxFQUFDLDJCQUEyQjthQUFDOzs7ZUFBTyxpQkFBaUIsRUFBRTtjQUFRO1lBQU07VUFBSztTQUNoSCxLQUFLO1FBQ0g7TUFDRCxDQUNOO0lBQ0g7RUFDRixDQUFDLENBQUM7O0FBRUgsV0FBVSxDQUFDLFlBQVksR0FBRztBQUN4QixTQUFNLEVBQUUsS0FBSyxDQUFDLFNBQVMsQ0FBQyxNQUFNLENBQUMsVUFBVTtFQUMxQzs7QUFFRCxVQUFTLGlCQUFpQixHQUFFOzJCQUNELE9BQU8sQ0FBQyxRQUFRLENBQUMsT0FBTyxDQUFDLElBQUksQ0FBQzs7T0FBbEQsZ0JBQWdCLHFCQUFoQixnQkFBZ0I7O0FBQ3JCLFVBQU8sZ0JBQWdCLENBQUM7RUFDekI7O0FBRUQsT0FBTSxDQUFDLE9BQU8sR0FBRyxVQUFVLEM7Ozs7Ozs7Ozs7Ozs7QUMzRDNCLEtBQUksS0FBSyxHQUFHLG1CQUFPLENBQUMsQ0FBTyxDQUFDLENBQUM7QUFDN0IsS0FBSSxDQUFDLEdBQUcsbUJBQU8sQ0FBQyxFQUFRLENBQUMsQ0FBQztBQUMxQixLQUFJLE9BQU8sR0FBRyxtQkFBTyxDQUFDLENBQWEsQ0FBQyxDQUFDOztnQkFDWixtQkFBTyxDQUFDLEdBQW9CLENBQUM7O0tBQWpELE9BQU8sWUFBUCxPQUFPO0tBQUUsT0FBTyxZQUFQLE9BQU87O0FBQ3JCLEtBQUksVUFBVSxHQUFHLG1CQUFPLENBQUMsR0FBa0IsQ0FBQyxDQUFDO0FBQzdDLEtBQUksZ0JBQWdCLEdBQUcsbUJBQU8sQ0FBQyxFQUFpQyxDQUFDLENBQUM7QUFDbEUsS0FBSSxjQUFjLEdBQUcsbUJBQU8sQ0FBQyxHQUFrQixDQUFDLENBQUM7O2lCQUMzQixtQkFBTyxDQUFDLEdBQWEsQ0FBQzs7S0FBdkMsYUFBYSxhQUFiLGFBQWE7O0FBRWxCLEtBQUksZUFBZSxHQUFHLEtBQUssQ0FBQyxXQUFXLENBQUM7OztBQUV0QyxTQUFNLEVBQUUsQ0FBQyxnQkFBZ0IsQ0FBQzs7QUFFMUIsb0JBQWlCLCtCQUFFO0FBQ2pCLE1BQUMsQ0FBQyxJQUFJLENBQUMsSUFBSSxDQUFDLElBQUksQ0FBQyxDQUFDLFFBQVEsQ0FBQztBQUN6QixZQUFLLEVBQUM7QUFDSixpQkFBUSxFQUFDO0FBQ1Asb0JBQVMsRUFBRSxDQUFDO0FBQ1osbUJBQVEsRUFBRSxJQUFJO1VBQ2Y7QUFDRCwwQkFBaUIsRUFBQztBQUNoQixtQkFBUSxFQUFFLElBQUk7QUFDZCxrQkFBTyxFQUFFLElBQUksQ0FBQyxJQUFJLENBQUMsUUFBUTtVQUM1QjtRQUNGOztBQUVELGVBQVEsRUFBRTtBQUNYLDBCQUFpQixFQUFFO0FBQ2xCLG9CQUFTLEVBQUUsQ0FBQyxDQUFDLFNBQVMsQ0FBQyxNQUFNLENBQUMsK0JBQStCLENBQUM7QUFDOUQsa0JBQU8sRUFBRSxrQ0FBa0M7VUFDM0M7UUFDQztNQUNGLENBQUM7SUFDSDs7QUFFRCxrQkFBZSw2QkFBRztBQUNoQixZQUFPO0FBQ0wsV0FBSSxFQUFFLElBQUksQ0FBQyxLQUFLLENBQUMsTUFBTSxDQUFDLElBQUk7QUFDNUIsVUFBRyxFQUFFLEVBQUU7QUFDUCxtQkFBWSxFQUFFLEVBQUU7QUFDaEIsWUFBSyxFQUFFLEVBQUU7TUFDVjtJQUNGOztBQUVELFVBQU8sbUJBQUMsQ0FBQyxFQUFFO0FBQ1QsTUFBQyxDQUFDLGNBQWMsRUFBRSxDQUFDO0FBQ25CLFNBQUksSUFBSSxDQUFDLE9BQU8sRUFBRSxFQUFFO0FBQ2xCLGlCQUFVLENBQUMsT0FBTyxDQUFDLE1BQU0sQ0FBQztBQUN4QixhQUFJLEVBQUUsSUFBSSxDQUFDLEtBQUssQ0FBQyxJQUFJO0FBQ3JCLFlBQUcsRUFBRSxJQUFJLENBQUMsS0FBSyxDQUFDLEdBQUc7QUFDbkIsY0FBSyxFQUFFLElBQUksQ0FBQyxLQUFLLENBQUMsS0FBSztBQUN2QixvQkFBVyxFQUFFLElBQUksQ0FBQyxLQUFLLENBQUMsTUFBTSxDQUFDLFlBQVksRUFBQyxDQUFDLENBQUM7TUFDakQ7SUFDRjs7QUFFRCxVQUFPLHFCQUFHO0FBQ1IsU0FBSSxLQUFLLEdBQUcsQ0FBQyxDQUFDLElBQUksQ0FBQyxJQUFJLENBQUMsSUFBSSxDQUFDLENBQUM7QUFDOUIsWUFBTyxLQUFLLENBQUMsTUFBTSxLQUFLLENBQUMsSUFBSSxLQUFLLENBQUMsS0FBSyxFQUFFLENBQUM7SUFDNUM7O0FBRUQsU0FBTSxvQkFBRzt5QkFDa0MsSUFBSSxDQUFDLEtBQUssQ0FBQyxNQUFNO1NBQXJELFlBQVksaUJBQVosWUFBWTtTQUFFLFFBQVEsaUJBQVIsUUFBUTtTQUFFLE9BQU8saUJBQVAsT0FBTzs7QUFDcEMsWUFDRTs7U0FBTSxHQUFHLEVBQUMsTUFBTSxFQUFDLFNBQVMsRUFBQyx1QkFBdUI7T0FDaEQ7Ozs7UUFBb0M7T0FDcEM7O1dBQUssU0FBUyxFQUFDLEVBQUU7U0FDZjs7YUFBSyxTQUFTLEVBQUMsWUFBWTtXQUN6QjtBQUNFLHNCQUFTLEVBQUUsSUFBSSxDQUFDLFNBQVMsQ0FBQyxNQUFNLENBQUU7QUFDbEMsaUJBQUksRUFBQyxVQUFVO0FBQ2Ysc0JBQVMsRUFBQyx1QkFBdUI7QUFDakMsd0JBQVcsRUFBQyxXQUFXLEdBQUU7VUFDdkI7U0FDTjs7YUFBSyxTQUFTLEVBQUMsWUFBWTtXQUN6QjtBQUNFLHNCQUFTO0FBQ1Qsc0JBQVMsRUFBRSxJQUFJLENBQUMsU0FBUyxDQUFDLEtBQUssQ0FBRTtBQUNqQyxnQkFBRyxFQUFDLFVBQVU7QUFDZCxpQkFBSSxFQUFDLFVBQVU7QUFDZixpQkFBSSxFQUFDLFVBQVU7QUFDZixzQkFBUyxFQUFDLGNBQWM7QUFDeEIsd0JBQVcsRUFBQyxVQUFVLEdBQUc7VUFDdkI7U0FDTjs7YUFBSyxTQUFTLEVBQUMsWUFBWTtXQUN6QjtBQUNFLHNCQUFTLEVBQUUsSUFBSSxDQUFDLFNBQVMsQ0FBQyxjQUFjLENBQUU7QUFDMUMsaUJBQUksRUFBQyxVQUFVO0FBQ2YsaUJBQUksRUFBQyxtQkFBbUI7QUFDeEIsc0JBQVMsRUFBQyxjQUFjO0FBQ3hCLHdCQUFXLEVBQUMsa0JBQWtCLEdBQUU7VUFDOUI7U0FDTjs7YUFBSyxTQUFTLEVBQUMsWUFBWTtXQUN6QjtBQUNFLGlCQUFJLEVBQUMsT0FBTztBQUNaLHNCQUFTLEVBQUUsSUFBSSxDQUFDLFNBQVMsQ0FBQyxPQUFPLENBQUU7QUFDbkMsc0JBQVMsRUFBQyx1QkFBdUI7QUFDakMsd0JBQVcsRUFBQyx5Q0FBeUMsR0FBRztVQUN0RDtTQUNOOzthQUFRLElBQUksRUFBQyxRQUFRLEVBQUMsUUFBUSxFQUFFLFlBQWEsRUFBQyxTQUFTLEVBQUMsc0NBQXNDLEVBQUMsT0FBTyxFQUFFLElBQUksQ0FBQyxPQUFROztVQUFrQjtTQUNySSxRQUFRLEdBQUk7O2FBQU8sU0FBUyxFQUFDLE9BQU87V0FBRSxPQUFPO1VBQVMsR0FBSSxJQUFJO1FBQzVEO01BQ0QsQ0FDUDtJQUNIO0VBQ0YsQ0FBQzs7QUFFRixLQUFJLE1BQU0sR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFFN0IsU0FBTSxFQUFFLENBQUMsT0FBTyxDQUFDLFVBQVUsQ0FBQzs7QUFFNUIsa0JBQWUsNkJBQUc7QUFDaEIsWUFBTztBQUNMLGFBQU0sRUFBRSxPQUFPLENBQUMsTUFBTTtBQUN0QixhQUFNLEVBQUUsT0FBTyxDQUFDLE1BQU07QUFDdEIscUJBQWMsRUFBRSxPQUFPLENBQUMsY0FBYztNQUN2QztJQUNGOztBQUVELG9CQUFpQiwrQkFBRTtBQUNqQixZQUFPLENBQUMsV0FBVyxDQUFDLElBQUksQ0FBQyxLQUFLLENBQUMsTUFBTSxDQUFDLFdBQVcsQ0FBQyxDQUFDO0lBQ3BEOztBQUVELFNBQU0sRUFBRSxrQkFBVztrQkFDc0IsSUFBSSxDQUFDLEtBQUs7U0FBNUMsY0FBYyxVQUFkLGNBQWM7U0FBRSxNQUFNLFVBQU4sTUFBTTtTQUFFLE1BQU0sVUFBTixNQUFNOztBQUVuQyxTQUFHLGNBQWMsQ0FBQyxRQUFRLEVBQUM7QUFDekIsY0FBTyxvQkFBQyxhQUFhLE9BQUU7TUFDeEI7O0FBRUQsU0FBRyxDQUFDLE1BQU0sRUFBRTtBQUNWLGNBQU8sSUFBSSxDQUFDO01BQ2I7O0FBRUQsWUFDRTs7U0FBSyxTQUFTLEVBQUMsd0JBQXdCO09BQ3JDLDZCQUFLLFNBQVMsRUFBQyxlQUFlLEdBQU87T0FDckM7O1dBQUssU0FBUyxFQUFDLHNCQUFzQjtTQUNuQzs7YUFBSyxTQUFTLEVBQUMsaUJBQWlCO1dBQzlCLG9CQUFDLGVBQWUsSUFBQyxNQUFNLEVBQUUsTUFBTyxFQUFDLE1BQU0sRUFBRSxNQUFNLENBQUMsSUFBSSxFQUFHLEdBQUU7V0FDekQsb0JBQUMsY0FBYyxPQUFFO1VBQ2I7U0FDTjs7YUFBSyxTQUFTLEVBQUMsb0NBQW9DO1dBQ2pEOzs7O2FBQWlDLCtCQUFLOzthQUFDOzs7O2NBQTJEO1lBQUs7V0FDdkcsNkJBQUssU0FBUyxFQUFDLGVBQWUsRUFBQyxHQUFHLDZCQUE0QixNQUFNLENBQUMsR0FBRyxDQUFDLElBQUksQ0FBSyxHQUFHO1VBQ2pGO1FBQ0Y7TUFDRixDQUNOO0lBQ0g7RUFDRixDQUFDLENBQUM7O0FBRUgsT0FBTSxDQUFDLE9BQU8sR0FBRyxNQUFNLEM7Ozs7Ozs7Ozs7Ozs7QUN2SnZCLEtBQUksS0FBSyxHQUFHLG1CQUFPLENBQUMsQ0FBTyxDQUFDLENBQUM7QUFDN0IsS0FBSSxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxDQUFhLENBQUMsQ0FBQztBQUNyQyxLQUFJLFdBQVcsR0FBRyxtQkFBTyxDQUFDLEVBQTBCLENBQUMsQ0FBQztBQUN0RCxLQUFJLFdBQVcsR0FBRyxtQkFBTyxDQUFDLEVBQTJCLENBQUMsQ0FBQztBQUN2RCxLQUFJLFFBQVEsR0FBRyxtQkFBTyxDQUFDLEdBQWdCLENBQUMsQ0FBQzs7QUFFekMsS0FBSSxLQUFLLEdBQUcsS0FBSyxDQUFDLFdBQVcsQ0FBQzs7O0FBRTVCLFNBQU0sRUFBRSxDQUFDLE9BQU8sQ0FBQyxVQUFVLENBQUM7O0FBRTVCLGtCQUFlLDZCQUFHO0FBQ2hCLFlBQU87QUFDTCxrQkFBVyxFQUFFLFdBQVcsQ0FBQyxZQUFZO0FBQ3JDLFdBQUksRUFBRSxXQUFXLENBQUMsSUFBSTtNQUN2QjtJQUNGOztBQUVELFNBQU0sRUFBRSxrQkFBVztBQUNqQixTQUFJLFdBQVcsR0FBRyxJQUFJLENBQUMsS0FBSyxDQUFDLFdBQVcsQ0FBQztBQUN6QyxTQUFJLE1BQU0sR0FBRyxJQUFJLENBQUMsS0FBSyxDQUFDLElBQUksQ0FBQyxNQUFNLENBQUM7QUFDcEMsWUFBUyxvQkFBQyxRQUFRLElBQUMsV0FBVyxFQUFFLFdBQVksRUFBQyxNQUFNLEVBQUUsTUFBTyxHQUFFLENBQUc7SUFDbEU7RUFDRixDQUFDLENBQUM7O0FBRUgsT0FBTSxDQUFDLE9BQU8sR0FBRyxLQUFLLEM7Ozs7Ozs7Ozs7Ozs7QUN4QnRCLEtBQUksS0FBSyxHQUFHLG1CQUFPLENBQUMsQ0FBTyxDQUFDLENBQUM7QUFDN0IsS0FBSSxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxDQUFhLENBQUMsQ0FBQzs7Z0JBQ0EsbUJBQU8sQ0FBQyxHQUFxQixDQUFDOztLQUE5RCxlQUFlLFlBQWYsZUFBZTtLQUFFLFdBQVcsWUFBWCxXQUFXOztpQkFDZ0MsbUJBQU8sQ0FBQyxFQUEwQixDQUFDOztLQUEvRixLQUFLLGFBQUwsS0FBSztLQUFFLE1BQU0sYUFBTixNQUFNO0tBQUUsSUFBSSxhQUFKLElBQUk7S0FBRSxRQUFRLGFBQVIsUUFBUTtLQUFFLGNBQWMsYUFBZCxjQUFjO0tBQUUsU0FBUyxhQUFULFNBQVM7O2lCQUNxQixtQkFBTyxDQUFDLEdBQWEsQ0FBQzs7S0FBbkcsVUFBVSxhQUFWLFVBQVU7S0FBRSxTQUFTLGFBQVQsU0FBUztLQUFFLFNBQVMsYUFBVCxTQUFTO0tBQUUsUUFBUSxhQUFSLFFBQVE7S0FBRSxZQUFZLGFBQVosWUFBWTtLQUFFLGVBQWUsYUFBZixlQUFlOztBQUU5RSxLQUFJLGlCQUFpQixHQUFHLEtBQUssQ0FBQyxXQUFXLENBQUM7OztBQUN4QyxTQUFNLEVBQUUsa0JBQVc7QUFDakIsU0FBSSxJQUFJLEdBQUcsSUFBSSxDQUFDLEtBQUssQ0FBQyxJQUFJLENBQUMsTUFBTSxDQUFDLGNBQUk7Y0FBSSxJQUFJLENBQUMsTUFBTTtNQUFBLENBQUMsQ0FBQztBQUN2RCxZQUNFOztTQUFLLFNBQVMsRUFBQyxxQkFBcUI7T0FDbEM7O1dBQUssU0FBUyxFQUFDLFlBQVk7U0FDekI7Ozs7VUFBMEI7UUFDdEI7T0FDTjs7V0FBSyxTQUFTLEVBQUMsYUFBYTtTQUN6QixJQUFJLENBQUMsTUFBTSxLQUFLLENBQUMsR0FBRyxvQkFBQyxTQUFTLElBQUMsSUFBSSxFQUFDLDhCQUE4QixHQUFFLEdBQ25FOzthQUFLLFNBQVMsRUFBQyxFQUFFO1dBQ2Y7QUFBQyxrQkFBSztlQUFDLFFBQVEsRUFBRSxJQUFJLENBQUMsTUFBTyxFQUFDLFNBQVMsRUFBQyxlQUFlO2FBQ3JELG9CQUFDLE1BQU07QUFDTCx3QkFBUyxFQUFDLEtBQUs7QUFDZixxQkFBTSxFQUFFO0FBQUMscUJBQUk7OztnQkFBc0I7QUFDbkMsbUJBQUksRUFBRSxvQkFBQyxRQUFRLElBQUMsSUFBSSxFQUFFLElBQUssR0FBSTtlQUMvQjthQUNGLG9CQUFDLE1BQU07QUFDTCxxQkFBTSxFQUFFO0FBQUMscUJBQUk7OztnQkFBVztBQUN4QixtQkFBSSxFQUNGLG9CQUFDLFVBQVUsSUFBQyxJQUFJLEVBQUUsSUFBSyxHQUN4QjtlQUNEO2FBQ0Ysb0JBQUMsTUFBTTtBQUNMLHFCQUFNLEVBQUU7QUFBQyxxQkFBSTs7O2dCQUFnQjtBQUM3QixtQkFBSSxFQUFFLG9CQUFDLFFBQVEsSUFBQyxJQUFJLEVBQUUsSUFBSyxHQUFLO2VBQ2hDO2FBQ0Ysb0JBQUMsTUFBTTtBQUNMLHdCQUFTLEVBQUMsU0FBUztBQUNuQixxQkFBTSxFQUFFO0FBQUMscUJBQUk7OztnQkFBbUI7QUFDaEMsbUJBQUksRUFBRSxvQkFBQyxlQUFlLElBQUMsSUFBSSxFQUFFLElBQUssR0FBSTtlQUN0QzthQUNGLG9CQUFDLE1BQU07QUFDTCxxQkFBTSxFQUFFO0FBQUMscUJBQUk7OztnQkFBaUI7QUFDOUIsbUJBQUksRUFBRSxvQkFBQyxTQUFTLElBQUMsSUFBSSxFQUFFLElBQUssR0FBSztlQUNqQztZQUNJO1VBQ0o7UUFFSjtNQUNGLENBQ1A7SUFDRjtFQUNGLENBQUMsQ0FBQzs7QUFFSCxPQUFNLENBQUMsT0FBTyxHQUFHLGlCQUFpQixDOzs7Ozs7Ozs7Ozs7O0FDbkRsQyxLQUFJLEtBQUssR0FBRyxtQkFBTyxDQUFDLENBQU8sQ0FBQyxDQUFDO0FBQzdCLEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsQ0FBYSxDQUFDLENBQUM7O2dCQUNaLG1CQUFPLENBQUMsRUFBc0IsQ0FBQzs7S0FBbkQsT0FBTyxZQUFQLE9BQU87S0FBRSxPQUFPLFlBQVAsT0FBTzs7QUFDckIsS0FBSSxpQkFBaUIsR0FBRyxtQkFBTyxDQUFDLEdBQXlCLENBQUMsQ0FBQztBQUMzRCxLQUFJLGlCQUFpQixHQUFHLG1CQUFPLENBQUMsR0FBeUIsQ0FBQyxDQUFDOztBQUUzRCxLQUFJLFFBQVEsR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFDL0IsU0FBTSxFQUFFLENBQUMsT0FBTyxDQUFDLFVBQVUsQ0FBQzs7QUFFNUIsa0JBQWUsNkJBQUc7QUFDaEIsWUFBTyxFQUFDLElBQUksRUFBRSxPQUFPLENBQUMsWUFBWSxFQUFDO0lBQ3BDOztBQUVELFNBQU0sRUFBRSxrQkFBVztTQUNaLElBQUksR0FBSSxJQUFJLENBQUMsS0FBSyxDQUFsQixJQUFJOztBQUNULFlBQ0U7O1NBQUssU0FBUyxFQUFDLHVCQUF1QjtPQUNwQyxvQkFBQyxpQkFBaUIsSUFBQyxJQUFJLEVBQUUsSUFBSyxHQUFFO09BQ2hDLDRCQUFJLFNBQVMsRUFBQyxhQUFhLEdBQUU7T0FDN0Isb0JBQUMsaUJBQWlCLElBQUMsSUFBSSxFQUFFLElBQUssR0FBRTtNQUM1QixDQUNOO0lBQ0g7RUFDRixDQUFDLENBQUM7O0FBRUgsT0FBTSxDQUFDLE9BQU8sR0FBRyxRQUFRLEM7Ozs7Ozs7Ozs7Ozs7OztBQ3pCekIsS0FBSSxLQUFLLEdBQUcsbUJBQU8sQ0FBQyxDQUFPLENBQUMsQ0FBQztBQUM3QixLQUFJLE9BQU8sR0FBRyxtQkFBTyxDQUFDLENBQWEsQ0FBQyxDQUFDOztnQkFDcEIsbUJBQU8sQ0FBQyxFQUFzQixDQUFDOztLQUExQyxPQUFPLFlBQVAsT0FBTzs7QUFDYixLQUFJLGdCQUFnQixHQUFHLG1CQUFPLENBQUMsRUFBaUMsQ0FBQyxDQUFDOztpQkFDRCxtQkFBTyxDQUFDLEVBQTBCLENBQUM7O0tBQS9GLEtBQUssYUFBTCxLQUFLO0tBQUUsTUFBTSxhQUFOLE1BQU07S0FBRSxJQUFJLGFBQUosSUFBSTtLQUFFLFFBQVEsYUFBUixRQUFRO0tBQUUsY0FBYyxhQUFkLGNBQWM7S0FBRSxTQUFTLGFBQVQsU0FBUzs7aUJBQzJCLG1CQUFPLENBQUMsR0FBYSxDQUFDOztLQUF6RyxVQUFVLGFBQVYsVUFBVTtLQUFFLGNBQWMsYUFBZCxjQUFjO0tBQUUsU0FBUyxhQUFULFNBQVM7S0FBRSxTQUFTLGFBQVQsU0FBUztLQUFFLFlBQVksYUFBWixZQUFZO0tBQUUsZUFBZSxhQUFmLGVBQWU7O2lCQUMvQyxtQkFBTyxDQUFDLEdBQXFCLENBQUM7O0tBQTlELGVBQWUsYUFBZixlQUFlO0tBQUUsV0FBVyxhQUFYLFdBQVc7O0FBQ2pDLEtBQUksTUFBTSxHQUFJLG1CQUFPLENBQUMsQ0FBUSxDQUFDLENBQUM7O2lCQUNiLG1CQUFPLENBQUMsRUFBc0IsQ0FBQzs7S0FBN0MsVUFBVSxhQUFWLFVBQVU7O2lCQUNDLG1CQUFPLENBQUMsRUFBd0IsQ0FBQzs7S0FBNUMsT0FBTyxhQUFQLE9BQU87O0FBQ1osS0FBSSxDQUFDLEdBQUcsbUJBQU8sQ0FBQyxFQUFHLENBQUMsQ0FBQzs7QUFFckIsS0FBSSxnQkFBZ0IsR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFFdkMsU0FBTSxFQUFFLENBQUMsZ0JBQWdCLENBQUM7O0FBRTFCLGtCQUFlLDJCQUFDLEtBQUssRUFBQzt1QkFDTyxVQUFVLENBQUMsSUFBSSxJQUFJLEVBQUUsQ0FBQzs7U0FBNUMsU0FBUztTQUFFLE9BQU87O0FBQ3ZCLFNBQUksQ0FBQyxlQUFlLEdBQUcsQ0FBQyxVQUFVLEVBQUUsU0FBUyxFQUFFLEtBQUssRUFBRSxPQUFPLENBQUMsQ0FBQztBQUMvRCxZQUFPLEVBQUUsTUFBTSxFQUFFLEVBQUUsRUFBRSxXQUFXLEVBQUUsRUFBQyxPQUFPLEVBQUUsS0FBSyxFQUFDLEVBQUUsU0FBUyxFQUFULFNBQVMsRUFBRSxPQUFPLEVBQVAsT0FBTyxFQUFFLENBQUM7SUFDMUU7O0FBRUQscUJBQWtCLGdDQUFFO0FBQ2xCLFlBQU8sQ0FBQyxhQUFhLENBQUMsSUFBSSxDQUFDLEtBQUssQ0FBQyxTQUFTLEVBQUUsSUFBSSxDQUFDLEtBQUssQ0FBQyxPQUFPLENBQUMsQ0FBQztJQUNqRTs7QUFFRCxxQkFBa0IsOEJBQUMsU0FBUyxFQUFFLE9BQU8sRUFBQztBQUNwQyxZQUFPLENBQUMsYUFBYSxDQUFDLFNBQVMsRUFBRSxPQUFPLENBQUMsQ0FBQztBQUMxQyxTQUFJLENBQUMsS0FBSyxDQUFDLFNBQVMsR0FBRyxTQUFTLENBQUM7QUFDakMsU0FBSSxDQUFDLEtBQUssQ0FBQyxPQUFPLEdBQUcsT0FBTyxDQUFDO0FBQzdCLFNBQUksQ0FBQyxRQUFRLENBQUMsSUFBSSxDQUFDLEtBQUssQ0FBQyxDQUFDO0lBQzNCOztBQUVELGVBQVksd0JBQUMsU0FBUyxFQUFFLE9BQU8sRUFBRTs7O0FBQy9CLFNBQUksQ0FBQyxRQUFRLGNBQ1IsSUFBSSxDQUFDLEtBQUs7QUFDYixrQkFBVyxtQ0FBSyxTQUFTLElBQUcsT0FBTyxlQUFFO1FBQ3JDLENBQUM7SUFDSjs7QUFFRCxzQkFBbUIsK0JBQUMsSUFBb0IsRUFBQztTQUFwQixTQUFTLEdBQVYsSUFBb0IsQ0FBbkIsU0FBUztTQUFFLE9BQU8sR0FBbkIsSUFBb0IsQ0FBUixPQUFPOztBQUNyQyxTQUFJLENBQUMsa0JBQWtCLENBQUMsU0FBUyxFQUFFLE9BQU8sQ0FBQyxDQUFDO0lBQzdDOztBQUVELHNCQUFtQiwrQkFBQyxRQUFRLEVBQUM7d0JBQ0EsVUFBVSxDQUFDLFFBQVEsQ0FBQzs7U0FBMUMsU0FBUztTQUFFLE9BQU87O0FBQ3ZCLFNBQUksQ0FBQyxrQkFBa0IsQ0FBQyxTQUFTLEVBQUUsT0FBTyxDQUFDLENBQUM7SUFDN0M7O0FBRUQsb0JBQWlCLDZCQUFDLFdBQVcsRUFBRSxXQUFXLEVBQUUsUUFBUSxFQUFDO0FBQ25ELFNBQUcsUUFBUSxLQUFLLFNBQVMsRUFBQztBQUN4QixXQUFJLFdBQVcsR0FBRyxNQUFNLENBQUMsV0FBVyxDQUFDLENBQUMsTUFBTSxDQUFDLE9BQU8sQ0FBQyxDQUFDLGlCQUFpQixFQUFFLENBQUM7QUFDMUUsY0FBTyxXQUFXLENBQUMsT0FBTyxDQUFDLFdBQVcsQ0FBQyxLQUFLLENBQUMsQ0FBQyxDQUFDO01BQ2hEO0lBQ0Y7O0FBRUQsZ0JBQWEseUJBQUMsSUFBSSxFQUFDOzs7QUFDakIsU0FBSSxRQUFRLEdBQUcsSUFBSSxDQUFDLE1BQU0sQ0FBQyxhQUFHO2NBQzVCLE9BQU8sQ0FBQyxHQUFHLEVBQUUsTUFBSyxLQUFLLENBQUMsTUFBTSxFQUFFO0FBQzlCLHdCQUFlLEVBQUUsTUFBSyxlQUFlO0FBQ3JDLFdBQUUsRUFBRSxNQUFLLGlCQUFpQjtRQUMzQixDQUFDO01BQUEsQ0FBQyxDQUFDOztBQUVOLFNBQUksU0FBUyxHQUFHLE1BQU0sQ0FBQyxtQkFBbUIsQ0FBQyxJQUFJLENBQUMsS0FBSyxDQUFDLFdBQVcsQ0FBQyxDQUFDLENBQUMsQ0FBQyxDQUFDO0FBQ3RFLFNBQUksT0FBTyxHQUFHLElBQUksQ0FBQyxLQUFLLENBQUMsV0FBVyxDQUFDLFNBQVMsQ0FBQyxDQUFDO0FBQ2hELFNBQUksTUFBTSxHQUFHLENBQUMsQ0FBQyxNQUFNLENBQUMsUUFBUSxFQUFFLFNBQVMsQ0FBQyxDQUFDO0FBQzNDLFNBQUcsT0FBTyxLQUFLLFNBQVMsQ0FBQyxHQUFHLEVBQUM7QUFDM0IsYUFBTSxHQUFHLE1BQU0sQ0FBQyxPQUFPLEVBQUUsQ0FBQztNQUMzQjs7QUFFRCxZQUFPLE1BQU0sQ0FBQztJQUNmOztBQUVELFNBQU0sRUFBRSxrQkFBVztrQkFDVSxJQUFJLENBQUMsS0FBSztTQUFoQyxTQUFTLFVBQVQsU0FBUztTQUFFLE9BQU8sVUFBUCxPQUFPOztBQUN2QixTQUFJLElBQUksR0FBRyxJQUFJLENBQUMsS0FBSyxDQUFDLElBQUksQ0FBQyxNQUFNLENBQUMsY0FBSTtjQUFJLENBQUMsSUFBSSxDQUFDLE1BQU0sSUFBSSxNQUFNLENBQUMsSUFBSSxDQUFDLE9BQU8sQ0FBQyxDQUFDLFNBQVMsQ0FBQyxTQUFTLEVBQUUsT0FBTyxDQUFDO01BQUEsQ0FBQyxDQUFDO0FBQzlHLFNBQUksR0FBRyxJQUFJLENBQUMsYUFBYSxDQUFDLElBQUksQ0FBQyxDQUFDOztBQUVoQyxZQUNFOztTQUFLLFNBQVMsRUFBQyxxQkFBcUI7T0FDbEM7O1dBQUssU0FBUyxFQUFDLFlBQVk7U0FDekI7Ozs7VUFBNEI7U0FDNUI7O2FBQUssU0FBUyxFQUFDLFVBQVU7V0FDdkI7O2VBQUssU0FBUyxFQUFDLGNBQWM7YUFDM0Isb0JBQUMsZUFBZSxJQUFDLFNBQVMsRUFBRSxTQUFVLEVBQUMsT0FBTyxFQUFFLE9BQVEsRUFBQyxRQUFRLEVBQUUsSUFBSSxDQUFDLG1CQUFvQixHQUFFO1lBQzFGO1dBQ047O2VBQUssU0FBUyxFQUFDLGNBQWM7YUFDM0Isb0JBQUMsV0FBVyxJQUFDLEtBQUssRUFBRSxTQUFVLEVBQUMsYUFBYSxFQUFFLElBQUksQ0FBQyxtQkFBb0IsR0FBRTtZQUNyRTtXQUNOOztlQUFLLFNBQVMsRUFBQyxjQUFjO2FBQzNCOztpQkFBSyxTQUFTLEVBQUMsWUFBWTtlQUN6QiwrQkFBTyxTQUFTLEVBQUUsSUFBSSxDQUFDLFNBQVMsQ0FBQyxRQUFRLENBQUUsRUFBQyxXQUFXLEVBQUMsV0FBVyxFQUFDLFNBQVMsRUFBQyx1QkFBdUIsR0FBRTtjQUNuRztZQUNGO1VBQ0Y7UUFDRjtPQUNOOztXQUFLLFNBQVMsRUFBQyxhQUFhO1NBQzFCOzthQUFLLFNBQVMsRUFBQyxFQUFFO1dBQ2Y7QUFBQyxrQkFBSztlQUFDLFFBQVEsRUFBRSxJQUFJLENBQUMsTUFBTyxFQUFDLFNBQVMsRUFBQyxlQUFlO2FBQ3JELG9CQUFDLE1BQU07QUFDTCx3QkFBUyxFQUFDLEtBQUs7QUFDZixxQkFBTSxFQUFFO0FBQUMscUJBQUk7OztnQkFBc0I7QUFDbkMsbUJBQUksRUFBRSxvQkFBQyxRQUFRLElBQUMsSUFBSSxFQUFFLElBQUssR0FBSTtlQUMvQjthQUNGLG9CQUFDLE1BQU07QUFDTCxxQkFBTSxFQUFFO0FBQUMscUJBQUk7OztnQkFBVztBQUN4QixtQkFBSSxFQUNGLG9CQUFDLFVBQVUsSUFBQyxJQUFJLEVBQUUsSUFBSyxHQUN4QjtlQUNEO2FBQ0Ysb0JBQUMsTUFBTTtBQUNMLHdCQUFTLEVBQUMsU0FBUztBQUNuQixxQkFBTSxFQUNKLG9CQUFDLGNBQWM7QUFDYix3QkFBTyxFQUFFLElBQUksQ0FBQyxLQUFLLENBQUMsV0FBVyxDQUFDLE9BQVE7QUFDeEMsNkJBQVksRUFBRSxJQUFJLENBQUMsWUFBYTtBQUNoQyxzQkFBSyxFQUFDLFNBQVM7aUJBRWxCO0FBQ0QsbUJBQUksRUFBRSxvQkFBQyxlQUFlLElBQUMsSUFBSSxFQUFFLElBQUssR0FBSTtlQUN0QzthQUNGLG9CQUFDLE1BQU07QUFDTCxxQkFBTSxFQUFFO0FBQUMscUJBQUk7OztnQkFBZ0I7QUFDN0IsbUJBQUksRUFBRSxvQkFBQyxjQUFjLElBQUMsSUFBSSxFQUFFLElBQUssR0FBSTtlQUNyQztZQUNJO1VBQ0o7UUFDRjtNQUNGLENBQ1A7SUFDRjtFQUNGLENBQUMsQ0FBQzs7QUFFSCxPQUFNLENBQUMsT0FBTyxHQUFHLGdCQUFnQixDOzs7Ozs7Ozs7Ozs7O0FDcklqQyxLQUFJLEtBQUssR0FBRyxtQkFBTyxDQUFDLENBQU8sQ0FBQyxDQUFDO0FBQzdCLEtBQUksTUFBTSxHQUFHLG1CQUFPLENBQUMsR0FBVyxDQUFDLENBQUMsTUFBTSxDQUFDOztnQkFDcUIsbUJBQU8sQ0FBQyxFQUFjLENBQUM7O0tBQS9FLE1BQU0sWUFBTixNQUFNO0tBQUUsS0FBSyxZQUFMLEtBQUs7S0FBRSxRQUFRLFlBQVIsUUFBUTtLQUFFLFVBQVUsWUFBVixVQUFVO0tBQUUsY0FBYyxZQUFkLGNBQWM7O2lCQUNvQixtQkFBTyxDQUFDLEdBQWMsQ0FBQzs7S0FBOUYsR0FBRyxhQUFILEdBQUc7S0FBRSxLQUFLLGFBQUwsS0FBSztLQUFFLEtBQUssYUFBTCxLQUFLO0tBQUUsUUFBUSxhQUFSLFFBQVE7S0FBRSxPQUFPLGFBQVAsT0FBTztLQUFFLGtCQUFrQixhQUFsQixrQkFBa0I7S0FBRSxRQUFRLGFBQVIsUUFBUTs7aUJBQ3JELG1CQUFPLENBQUMsR0FBd0IsQ0FBQzs7S0FBL0MsVUFBVSxhQUFWLFVBQVU7O0FBQ2YsS0FBSSxJQUFJLEdBQUcsbUJBQU8sQ0FBQyxFQUFRLENBQUMsQ0FBQztBQUM3QixLQUFJLE9BQU8sR0FBRyxtQkFBTyxDQUFDLEVBQVcsQ0FBQyxDQUFDO0FBQ25DLEtBQUksR0FBRyxHQUFHLG1CQUFPLENBQUMsRUFBVSxDQUFDLENBQUM7O0FBRTlCLG9CQUFPLENBQUMsR0FBVyxDQUFDLENBQUM7OztBQUdyQixRQUFPLENBQUMsSUFBSSxFQUFFLENBQUM7O0FBRWYsVUFBUyxZQUFZLENBQUMsU0FBUyxFQUFFLE9BQU8sRUFBRSxFQUFFLEVBQUM7QUFDM0MsT0FBSSxDQUFDLE1BQU0sRUFBRSxDQUFDO0VBQ2Y7O0FBRUQsT0FBTSxDQUNKO0FBQUMsU0FBTTtLQUFDLE9BQU8sRUFBRSxPQUFPLENBQUMsVUFBVSxFQUFHO0dBQ3BDLG9CQUFDLEtBQUssSUFBQyxJQUFJLEVBQUUsR0FBRyxDQUFDLE1BQU0sQ0FBQyxLQUFNLEVBQUMsU0FBUyxFQUFFLEtBQU0sR0FBRTtHQUNsRCxvQkFBQyxLQUFLLElBQUMsSUFBSSxFQUFFLEdBQUcsQ0FBQyxNQUFNLENBQUMsTUFBTyxFQUFDLE9BQU8sRUFBRSxZQUFhLEdBQUU7R0FDeEQsb0JBQUMsS0FBSyxJQUFDLElBQUksRUFBRSxHQUFHLENBQUMsTUFBTSxDQUFDLE9BQVEsRUFBQyxTQUFTLEVBQUUsT0FBUSxHQUFFO0dBQ3RELG9CQUFDLFFBQVEsSUFBQyxJQUFJLEVBQUUsR0FBRyxDQUFDLE1BQU0sQ0FBQyxHQUFJLEVBQUMsRUFBRSxFQUFFLEdBQUcsQ0FBQyxNQUFNLENBQUMsS0FBTSxHQUFFO0dBQ3ZEO0FBQUMsVUFBSztPQUFDLElBQUksRUFBRSxHQUFHLENBQUMsTUFBTSxDQUFDLEdBQUksRUFBQyxTQUFTLEVBQUUsR0FBSSxFQUFDLE9BQU8sRUFBRSxVQUFXO0tBQy9ELG9CQUFDLEtBQUssSUFBQyxJQUFJLEVBQUUsR0FBRyxDQUFDLE1BQU0sQ0FBQyxLQUFNLEVBQUMsU0FBUyxFQUFFLEtBQU0sR0FBRTtLQUNsRCxvQkFBQyxLQUFLLElBQUMsSUFBSSxFQUFFLEdBQUcsQ0FBQyxNQUFNLENBQUMsYUFBYyxFQUFDLFVBQVUsRUFBRSxFQUFDLGtCQUFrQixFQUFFLGtCQUFrQixFQUFFLEdBQUU7S0FDOUYsb0JBQUMsS0FBSyxJQUFDLElBQUksRUFBRSxHQUFHLENBQUMsTUFBTSxDQUFDLFFBQVMsRUFBQyxTQUFTLEVBQUUsUUFBUyxHQUFFO0lBQ2xEO0dBQ1Isb0JBQUMsS0FBSyxJQUFDLElBQUksRUFBQyxHQUFHLEVBQUMsU0FBUyxFQUFFLFFBQVMsR0FBRztFQUNoQyxFQUNSLFFBQVEsQ0FBQyxjQUFjLENBQUMsS0FBSyxDQUFDLENBQUMsQzs7Ozs7Ozs7O0FDL0JsQywyQiIsImZpbGUiOiJhcHAuanMiLCJzb3VyY2VzQ29udGVudCI6WyJpbXBvcnQgeyBSZWFjdG9yIH0gZnJvbSAnbnVjbGVhci1qcydcblxuY29uc3QgcmVhY3RvciA9IG5ldyBSZWFjdG9yKHtcbiAgZGVidWc6IHRydWVcbn0pXG5cbndpbmRvdy5yZWFjdG9yID0gcmVhY3RvcjtcblxuZXhwb3J0IGRlZmF1bHQgcmVhY3RvclxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL3JlYWN0b3IuanNcbiAqKi8iLCJsZXQge2Zvcm1hdFBhdHRlcm59ID0gcmVxdWlyZSgnYXBwL2NvbW1vbi9wYXR0ZXJuVXRpbHMnKTtcblxubGV0IGNmZyA9IHtcblxuICBiYXNlVXJsOiB3aW5kb3cubG9jYXRpb24ub3JpZ2luLFxuXG4gIGhlbHBVcmw6ICdodHRwczovL2dpdGh1Yi5jb20vZ3Jhdml0YXRpb25hbC90ZWxlcG9ydC9ibG9iL21hc3Rlci9SRUFETUUubWQnLFxuXG4gIGFwaToge1xuICAgIHJlbmV3VG9rZW5QYXRoOicvdjEvd2ViYXBpL3Nlc3Npb25zL3JlbmV3JyxcbiAgICBub2Rlc1BhdGg6ICcvdjEvd2ViYXBpL3NpdGVzLy1jdXJyZW50LS9ub2RlcycsXG4gICAgc2Vzc2lvblBhdGg6ICcvdjEvd2ViYXBpL3Nlc3Npb25zJyxcbiAgICBzaXRlU2Vzc2lvblBhdGg6ICcvdjEvd2ViYXBpL3NpdGVzLy1jdXJyZW50LS9zZXNzaW9ucy86c2lkJyxcbiAgICBpbnZpdGVQYXRoOiAnL3YxL3dlYmFwaS91c2Vycy9pbnZpdGVzLzppbnZpdGVUb2tlbicsXG4gICAgY3JlYXRlVXNlclBhdGg6ICcvdjEvd2ViYXBpL3VzZXJzJyxcbiAgICBzZXNzaW9uQ2h1bms6ICcvdjEvd2ViYXBpL3NpdGVzLy1jdXJyZW50LS9zZXNzaW9ucy86c2lkL2NodW5rcz9zdGFydD06c3RhcnQmZW5kPTplbmQnLFxuICAgIHNlc3Npb25DaHVua0NvdW50UGF0aDogJy92MS93ZWJhcGkvc2l0ZXMvLWN1cnJlbnQtL3Nlc3Npb25zLzpzaWQvY2h1bmtzY291bnQnLFxuXG4gICAgZ2V0RmV0Y2hTZXNzaW9uQ2h1bmtVcmw6ICh7c2lkLCBzdGFydCwgZW5kfSk9PntcbiAgICAgIHJldHVybiBmb3JtYXRQYXR0ZXJuKGNmZy5hcGkuc2Vzc2lvbkNodW5rLCB7c2lkLCBzdGFydCwgZW5kfSk7XG4gICAgfSxcblxuICAgIGdldEZldGNoU2Vzc2lvbkxlbmd0aFVybDogKHNpZCk9PntcbiAgICAgIHJldHVybiBmb3JtYXRQYXR0ZXJuKGNmZy5hcGkuc2Vzc2lvbkNodW5rQ291bnRQYXRoLCB7c2lkfSk7XG4gICAgfSxcblxuICAgIGdldEZldGNoU2Vzc2lvbnNVcmw6IChzdGFydCwgZW5kKT0+e1xuICAgICAgdmFyIHBhcmFtcyA9IHtcbiAgICAgICAgc3RhcnQ6IHN0YXJ0LnRvSVNPU3RyaW5nKCksXG4gICAgICAgIGVuZDogZW5kLnRvSVNPU3RyaW5nKCkgICAgICAgIFxuICAgICAgfTtcblxuICAgICAgdmFyIGpzb24gPSBKU09OLnN0cmluZ2lmeShwYXJhbXMpO1xuICAgICAgdmFyIGpzb25FbmNvZGVkID0gd2luZG93LmVuY29kZVVSSShqc29uKTtcblxuICAgICAgcmV0dXJuIGAvdjEvd2ViYXBpL3NpdGVzLy1jdXJyZW50LS9ldmVudHMvc2Vzc2lvbnM/ZmlsdGVyPSR7anNvbkVuY29kZWR9YDtcbiAgICB9LFxuXG4gICAgZ2V0RmV0Y2hTZXNzaW9uVXJsOiAoc2lkKT0+e1xuICAgICAgcmV0dXJuIGZvcm1hdFBhdHRlcm4oY2ZnLmFwaS5zaXRlU2Vzc2lvblBhdGgsIHtzaWR9KTtcbiAgICB9LFxuXG4gICAgZ2V0VGVybWluYWxTZXNzaW9uVXJsOiAoc2lkKT0+IHtcbiAgICAgIHJldHVybiBmb3JtYXRQYXR0ZXJuKGNmZy5hcGkuc2l0ZVNlc3Npb25QYXRoLCB7c2lkfSk7XG4gICAgfSxcblxuICAgIGdldEludml0ZVVybDogKGludml0ZVRva2VuKSA9PiB7XG4gICAgICByZXR1cm4gZm9ybWF0UGF0dGVybihjZmcuYXBpLmludml0ZVBhdGgsIHtpbnZpdGVUb2tlbn0pO1xuICAgIH0sXG5cbiAgICBnZXRFdmVudFN0cmVhbUNvbm5TdHI6ICh0b2tlbiwgc2lkKSA9PiB7XG4gICAgICB2YXIgaG9zdG5hbWUgPSBnZXRXc0hvc3ROYW1lKCk7XG4gICAgICByZXR1cm4gYCR7aG9zdG5hbWV9L3YxL3dlYmFwaS9zaXRlcy8tY3VycmVudC0vc2Vzc2lvbnMvJHtzaWR9L2V2ZW50cy9zdHJlYW0/YWNjZXNzX3Rva2VuPSR7dG9rZW59YDtcbiAgICB9LFxuXG4gICAgZ2V0VHR5Q29ublN0cjogKHt0b2tlbiwgc2VydmVySWQsIGxvZ2luLCBzaWQsIHJvd3MsIGNvbHN9KSA9PiB7XG4gICAgICB2YXIgcGFyYW1zID0ge1xuICAgICAgICBzZXJ2ZXJfaWQ6IHNlcnZlcklkLFxuICAgICAgICBsb2dpbixcbiAgICAgICAgc2lkLFxuICAgICAgICB0ZXJtOiB7XG4gICAgICAgICAgaDogcm93cyxcbiAgICAgICAgICB3OiBjb2xzXG4gICAgICAgIH1cbiAgICAgIH1cblxuICAgICAgdmFyIGpzb24gPSBKU09OLnN0cmluZ2lmeShwYXJhbXMpO1xuICAgICAgdmFyIGpzb25FbmNvZGVkID0gd2luZG93LmVuY29kZVVSSShqc29uKTtcbiAgICAgIHZhciBob3N0bmFtZSA9IGdldFdzSG9zdE5hbWUoKTtcbiAgICAgIHJldHVybiBgJHtob3N0bmFtZX0vdjEvd2ViYXBpL3NpdGVzLy1jdXJyZW50LS9jb25uZWN0P2FjY2Vzc190b2tlbj0ke3Rva2VufSZwYXJhbXM9JHtqc29uRW5jb2RlZH1gO1xuICAgIH1cbiAgfSxcblxuICByb3V0ZXM6IHtcbiAgICBhcHA6ICcvd2ViJyxcbiAgICBsb2dvdXQ6ICcvd2ViL2xvZ291dCcsXG4gICAgbG9naW46ICcvd2ViL2xvZ2luJyxcbiAgICBub2RlczogJy93ZWIvbm9kZXMnLFxuICAgIGFjdGl2ZVNlc3Npb246ICcvd2ViL3Nlc3Npb25zLzpzaWQnLFxuICAgIG5ld1VzZXI6ICcvd2ViL25ld3VzZXIvOmludml0ZVRva2VuJyxcbiAgICBzZXNzaW9uczogJy93ZWIvc2Vzc2lvbnMnLFxuICAgIHBhZ2VOb3RGb3VuZDogJy93ZWIvbm90Zm91bmQnXG4gIH0sXG5cbiAgZ2V0QWN0aXZlU2Vzc2lvblJvdXRlVXJsKHNpZCl7XG4gICAgcmV0dXJuIGZvcm1hdFBhdHRlcm4oY2ZnLnJvdXRlcy5hY3RpdmVTZXNzaW9uLCB7c2lkfSk7XG4gIH1cbn1cblxuZXhwb3J0IGRlZmF1bHQgY2ZnO1xuXG5mdW5jdGlvbiBnZXRXc0hvc3ROYW1lKCl7XG4gIHZhciBwcmVmaXggPSBsb2NhdGlvbi5wcm90b2NvbCA9PSBcImh0dHBzOlwiP1wid3NzOi8vXCI6XCJ3czovL1wiO1xuICB2YXIgaG9zdHBvcnQgPSBsb2NhdGlvbi5ob3N0bmFtZSsobG9jYXRpb24ucG9ydCA/ICc6Jytsb2NhdGlvbi5wb3J0OiAnJyk7XG4gIHJldHVybiBgJHtwcmVmaXh9JHtob3N0cG9ydH1gO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbmZpZy5qc1xuICoqLyIsIi8qKlxuICogQ29weXJpZ2h0IDIwMTMtMjAxNCBGYWNlYm9vaywgSW5jLlxuICpcbiAqIExpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG4gKiB5b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG4gKiBZb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcbiAqXG4gKiBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcbiAqXG4gKiBVbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG4gKiBkaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG4gKiBXSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cbiAqIFNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbiAqIGxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuICpcbiAqL1xuXG5cInVzZSBzdHJpY3RcIjtcblxuLyoqXG4gKiBDb25zdHJ1Y3RzIGFuIGVudW1lcmF0aW9uIHdpdGgga2V5cyBlcXVhbCB0byB0aGVpciB2YWx1ZS5cbiAqXG4gKiBGb3IgZXhhbXBsZTpcbiAqXG4gKiAgIHZhciBDT0xPUlMgPSBrZXlNaXJyb3Ioe2JsdWU6IG51bGwsIHJlZDogbnVsbH0pO1xuICogICB2YXIgbXlDb2xvciA9IENPTE9SUy5ibHVlO1xuICogICB2YXIgaXNDb2xvclZhbGlkID0gISFDT0xPUlNbbXlDb2xvcl07XG4gKlxuICogVGhlIGxhc3QgbGluZSBjb3VsZCBub3QgYmUgcGVyZm9ybWVkIGlmIHRoZSB2YWx1ZXMgb2YgdGhlIGdlbmVyYXRlZCBlbnVtIHdlcmVcbiAqIG5vdCBlcXVhbCB0byB0aGVpciBrZXlzLlxuICpcbiAqICAgSW5wdXQ6ICB7a2V5MTogdmFsMSwga2V5MjogdmFsMn1cbiAqICAgT3V0cHV0OiB7a2V5MToga2V5MSwga2V5Mjoga2V5Mn1cbiAqXG4gKiBAcGFyYW0ge29iamVjdH0gb2JqXG4gKiBAcmV0dXJuIHtvYmplY3R9XG4gKi9cbnZhciBrZXlNaXJyb3IgPSBmdW5jdGlvbihvYmopIHtcbiAgdmFyIHJldCA9IHt9O1xuICB2YXIga2V5O1xuICBpZiAoIShvYmogaW5zdGFuY2VvZiBPYmplY3QgJiYgIUFycmF5LmlzQXJyYXkob2JqKSkpIHtcbiAgICB0aHJvdyBuZXcgRXJyb3IoJ2tleU1pcnJvciguLi4pOiBBcmd1bWVudCBtdXN0IGJlIGFuIG9iamVjdC4nKTtcbiAgfVxuICBmb3IgKGtleSBpbiBvYmopIHtcbiAgICBpZiAoIW9iai5oYXNPd25Qcm9wZXJ0eShrZXkpKSB7XG4gICAgICBjb250aW51ZTtcbiAgICB9XG4gICAgcmV0W2tleV0gPSBrZXk7XG4gIH1cbiAgcmV0dXJuIHJldDtcbn07XG5cbm1vZHVsZS5leHBvcnRzID0ga2V5TWlycm9yO1xuXG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiAuL34va2V5bWlycm9yL2luZGV4LmpzXG4gKiogbW9kdWxlIGlkID0gMjBcbiAqKiBtb2R1bGUgY2h1bmtzID0gMVxuICoqLyIsInZhciB7IGJyb3dzZXJIaXN0b3J5LCBjcmVhdGVNZW1vcnlIaXN0b3J5IH0gPSByZXF1aXJlKCdyZWFjdC1yb3V0ZXInKTtcblxuY29uc3QgQVVUSF9LRVlfREFUQSA9ICdhdXRoRGF0YSc7XG5cbnZhciBfaGlzdG9yeSA9IGNyZWF0ZU1lbW9yeUhpc3RvcnkoKTtcblxudmFyIHNlc3Npb24gPSB7XG5cbiAgaW5pdChoaXN0b3J5PWJyb3dzZXJIaXN0b3J5KXtcbiAgICBfaGlzdG9yeSA9IGhpc3Rvcnk7XG4gIH0sXG5cbiAgZ2V0SGlzdG9yeSgpe1xuICAgIHJldHVybiBfaGlzdG9yeTtcbiAgfSxcblxuICBzZXRVc2VyRGF0YSh1c2VyRGF0YSl7XG4gICAgbG9jYWxTdG9yYWdlLnNldEl0ZW0oQVVUSF9LRVlfREFUQSwgSlNPTi5zdHJpbmdpZnkodXNlckRhdGEpKTtcbiAgfSxcblxuICBnZXRVc2VyRGF0YSgpe1xuICAgIHZhciBpdGVtID0gbG9jYWxTdG9yYWdlLmdldEl0ZW0oQVVUSF9LRVlfREFUQSk7XG4gICAgaWYoaXRlbSl7XG4gICAgICByZXR1cm4gSlNPTi5wYXJzZShpdGVtKTtcbiAgICB9XG5cbiAgICByZXR1cm4ge307XG4gIH0sXG5cbiAgY2xlYXIoKXtcbiAgICBsb2NhbFN0b3JhZ2UuY2xlYXIoKVxuICB9XG5cbn1cblxubW9kdWxlLmV4cG9ydHMgPSBzZXNzaW9uO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL3Nlc3Npb24uanNcbiAqKi8iLCJ2YXIgJCA9IHJlcXVpcmUoXCJqUXVlcnlcIik7XG52YXIgc2Vzc2lvbiA9IHJlcXVpcmUoJ2FwcC9zZXNzaW9uJyk7XG5cbmNvbnN0IGFwaSA9IHtcblxuICBwdXQocGF0aCwgZGF0YSwgd2l0aFRva2VuKXtcbiAgICByZXR1cm4gYXBpLmFqYXgoe3VybDogcGF0aCwgZGF0YTogSlNPTi5zdHJpbmdpZnkoZGF0YSksIHR5cGU6ICdQVVQnfSwgd2l0aFRva2VuKTtcbiAgfSxcblxuICBwb3N0KHBhdGgsIGRhdGEsIHdpdGhUb2tlbil7XG4gICAgcmV0dXJuIGFwaS5hamF4KHt1cmw6IHBhdGgsIGRhdGE6IEpTT04uc3RyaW5naWZ5KGRhdGEpLCB0eXBlOiAnUE9TVCd9LCB3aXRoVG9rZW4pO1xuICB9LFxuXG4gIGdldChwYXRoKXtcbiAgICByZXR1cm4gYXBpLmFqYXgoe3VybDogcGF0aH0pO1xuICB9LFxuXG4gIGFqYXgoY2ZnLCB3aXRoVG9rZW4gPSB0cnVlKXtcbiAgICB2YXIgZGVmYXVsdENmZyA9IHtcbiAgICAgIHR5cGU6IFwiR0VUXCIsXG4gICAgICBkYXRhVHlwZTogXCJqc29uXCIsXG4gICAgICBiZWZvcmVTZW5kOiBmdW5jdGlvbih4aHIpIHtcbiAgICAgICAgaWYod2l0aFRva2VuKXtcbiAgICAgICAgICB2YXIgeyB0b2tlbiB9ID0gc2Vzc2lvbi5nZXRVc2VyRGF0YSgpO1xuICAgICAgICAgIHhoci5zZXRSZXF1ZXN0SGVhZGVyKCdBdXRob3JpemF0aW9uJywnQmVhcmVyICcgKyB0b2tlbik7XG4gICAgICAgIH1cbiAgICAgICB9XG4gICAgfVxuXG4gICAgcmV0dXJuICQuYWpheCgkLmV4dGVuZCh7fSwgZGVmYXVsdENmZywgY2ZnKSk7XG4gIH1cbn1cblxubW9kdWxlLmV4cG9ydHMgPSBhcGk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvc2VydmljZXMvYXBpLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMgPSBqUXVlcnk7XG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiBleHRlcm5hbCBcImpRdWVyeVwiXG4gKiogbW9kdWxlIGlkID0gMzlcbiAqKiBtb2R1bGUgY2h1bmtzID0gMVxuICoqLyIsImNvbnN0IG5vZGVIb3N0TmFtZUJ5U2VydmVySWQgPSAoc2VydmVySWQpID0+IFsgWyd0bHB0X25vZGVzJ10sIChub2RlcykgPT57XG4gIGxldCBzZXJ2ZXIgPSBub2Rlcy5maW5kKGl0ZW09PiBpdGVtLmdldCgnaWQnKSA9PT0gc2VydmVySWQpOyAgXG4gIHJldHVybiAhc2VydmVyID8gJycgOiBzZXJ2ZXIuZ2V0KCdob3N0bmFtZScpO1xufV07XG5cbmNvbnN0IG5vZGVMaXN0VmlldyA9IFsgWyd0bHB0X25vZGVzJ10sIChub2RlcykgPT57XG4gICAgcmV0dXJuIG5vZGVzLm1hcCgoaXRlbSk9PntcbiAgICAgIHZhciBzZXJ2ZXJJZCA9IGl0ZW0uZ2V0KCdpZCcpO1xuICAgICAgcmV0dXJuIHtcbiAgICAgICAgaWQ6IHNlcnZlcklkLFxuICAgICAgICBob3N0bmFtZTogaXRlbS5nZXQoJ2hvc3RuYW1lJyksXG4gICAgICAgIHRhZ3M6IGdldFRhZ3MoaXRlbSksXG4gICAgICAgIGFkZHI6IGl0ZW0uZ2V0KCdhZGRyJylcbiAgICAgIH1cbiAgICB9KS50b0pTKCk7XG4gfVxuXTtcblxuZnVuY3Rpb24gZ2V0VGFncyhub2RlKXtcbiAgdmFyIGFsbExhYmVscyA9IFtdO1xuICB2YXIgbGFiZWxzID0gbm9kZS5nZXQoJ2xhYmVscycpO1xuXG4gIGlmKGxhYmVscyl7XG4gICAgbGFiZWxzLmVudHJ5U2VxKCkudG9BcnJheSgpLmZvckVhY2goaXRlbT0+e1xuICAgICAgYWxsTGFiZWxzLnB1c2goe1xuICAgICAgICByb2xlOiBpdGVtWzBdLFxuICAgICAgICB2YWx1ZTogaXRlbVsxXVxuICAgICAgfSk7XG4gICAgfSk7XG4gIH1cblxuICBsYWJlbHMgPSBub2RlLmdldCgnY21kX2xhYmVscycpO1xuXG4gIGlmKGxhYmVscyl7XG4gICAgbGFiZWxzLmVudHJ5U2VxKCkudG9BcnJheSgpLmZvckVhY2goaXRlbT0+e1xuICAgICAgYWxsTGFiZWxzLnB1c2goe1xuICAgICAgICByb2xlOiBpdGVtWzBdLFxuICAgICAgICB2YWx1ZTogaXRlbVsxXS5nZXQoJ3Jlc3VsdCcpLFxuICAgICAgICB0b29sdGlwOiBpdGVtWzFdLmdldCgnY29tbWFuZCcpXG4gICAgICB9KTtcbiAgICB9KTtcbiAgfVxuXG4gIHJldHVybiBhbGxMYWJlbHM7XG59XG5cblxuZXhwb3J0IGRlZmF1bHQge1xuICBub2RlTGlzdFZpZXcsXG4gIG5vZGVIb3N0TmFtZUJ5U2VydmVySWRcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2dldHRlcnMuanNcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cy5nZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3Rpb25zID0gcmVxdWlyZSgnLi9hY3Rpb25zJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3RpdmVUZXJtU3RvcmUgPSByZXF1aXJlKCcuL2FjdGl2ZVRlcm1TdG9yZScpO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvaW5kZXguanNcbiAqKi8iLCJpbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVFJZSU5HX1RPX1NJR05fVVA6IG51bGwsXG4gIFRSWUlOR19UT19MT0dJTjogbnVsbCxcbiAgRkVUQ0hJTkdfSU5WSVRFOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9jb25zdGFudHMuanNcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cy5nZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3Rpb25zID0gcmVxdWlyZSgnLi9hY3Rpb25zJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3RpdmVUZXJtU3RvcmUgPSByZXF1aXJlKCcuL3Nlc3Npb25TdG9yZScpO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvaW5kZXguanNcbiAqKi8iLCJ2YXIge1RSWUlOR19UT19MT0dJTn0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9yZXN0QXBpL2NvbnN0YW50cycpO1xudmFyIHtyZXF1ZXN0U3RhdHVzfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3Jlc3RBcGkvZ2V0dGVycycpO1xuXG5jb25zdCB1c2VyID0gWyBbJ3RscHRfdXNlciddLCAoY3VycmVudFVzZXIpID0+IHtcbiAgICBpZighY3VycmVudFVzZXIpe1xuICAgICAgcmV0dXJuIG51bGw7XG4gICAgfVxuXG4gICAgdmFyIG5hbWUgPSBjdXJyZW50VXNlci5nZXQoJ25hbWUnKSB8fCAnJztcbiAgICB2YXIgc2hvcnREaXNwbGF5TmFtZSA9IG5hbWVbMF0gfHwgJyc7XG5cbiAgICByZXR1cm4ge1xuICAgICAgbmFtZSxcbiAgICAgIHNob3J0RGlzcGxheU5hbWUsXG4gICAgICBsb2dpbnM6IGN1cnJlbnRVc2VyLmdldCgnYWxsb3dlZF9sb2dpbnMnKS50b0pTKClcbiAgICB9XG4gIH1cbl07XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgdXNlcixcbiAgbG9naW5BdHRlbXA6IHJlcXVlc3RTdGF0dXMoVFJZSU5HX1RPX0xPR0lOKVxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvdXNlci9nZXR0ZXJzLmpzXG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcblxuY29uc3QgR3J2VGFibGVUZXh0Q2VsbCA9ICh7cm93SW5kZXgsIGRhdGEsIGNvbHVtbktleSwgLi4ucHJvcHN9KSA9PiAoXG4gIDxHcnZUYWJsZUNlbGwgey4uLnByb3BzfT5cbiAgICB7ZGF0YVtyb3dJbmRleF1bY29sdW1uS2V5XX1cbiAgPC9HcnZUYWJsZUNlbGw+XG4pO1xuXG52YXIgR3J2U29ydEhlYWRlckNlbGwgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICB0aGlzLl9vblNvcnRDaGFuZ2UgPSB0aGlzLl9vblNvcnRDaGFuZ2UuYmluZCh0aGlzKTtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgdmFyIHtzb3J0RGlyLCBjaGlsZHJlbiwgLi4ucHJvcHN9ID0gdGhpcy5wcm9wcztcbiAgICByZXR1cm4gKFxuICAgICAgPENlbGwgey4uLnByb3BzfT5cbiAgICAgICAgPGEgb25DbGljaz17dGhpcy5fb25Tb3J0Q2hhbmdlfT5cbiAgICAgICAgICB7Y2hpbGRyZW59IHtzb3J0RGlyID8gKHNvcnREaXIgPT09IFNvcnRUeXBlcy5ERVNDID8gJ+KGkycgOiAn4oaRJykgOiAnJ31cbiAgICAgICAgPC9hPlxuICAgICAgPC9DZWxsPlxuICAgICk7XG4gIH0sXG5cbiAgX29uU29ydENoYW5nZShlKSB7XG4gICAgZS5wcmV2ZW50RGVmYXVsdCgpO1xuXG4gICAgaWYgKHRoaXMucHJvcHMub25Tb3J0Q2hhbmdlKSB7XG4gICAgICB0aGlzLnByb3BzLm9uU29ydENoYW5nZShcbiAgICAgICAgdGhpcy5wcm9wcy5jb2x1bW5LZXksXG4gICAgICAgIHRoaXMucHJvcHMuc29ydERpciA/XG4gICAgICAgICAgcmV2ZXJzZVNvcnREaXJlY3Rpb24odGhpcy5wcm9wcy5zb3J0RGlyKSA6XG4gICAgICAgICAgU29ydFR5cGVzLkRFU0NcbiAgICAgICk7XG4gICAgfVxuICB9XG59KTtcblxuLyoqXG4qIFNvcnQgaW5kaWNhdG9yIHVzZWQgYnkgU29ydEhlYWRlckNlbGxcbiovXG5jb25zdCBTb3J0VHlwZXMgPSB7XG4gIEFTQzogJ0FTQycsXG4gIERFU0M6ICdERVNDJ1xufTtcblxuY29uc3QgU29ydEluZGljYXRvciA9ICh7c29ydERpcn0pPT57XG4gIGxldCBjbHMgPSAnZ3J2LXRhYmxlLWluZGljYXRvci1zb3J0IGZhIGZhLXNvcnQnXG4gIGlmKHNvcnREaXIgPT09IFNvcnRUeXBlcy5ERVNDKXtcbiAgICBjbHMgKz0gJy1kZXNjJ1xuICB9XG5cbiAgaWYoIHNvcnREaXIgPT09IFNvcnRUeXBlcy5BU0Mpe1xuICAgIGNscyArPSAnLWFzYydcbiAgfVxuXG4gIHJldHVybiAoPGkgY2xhc3NOYW1lPXtjbHN9PjwvaT4pO1xufTtcblxuLyoqXG4qIFNvcnQgSGVhZGVyIENlbGxcbiovXG52YXIgU29ydEhlYWRlckNlbGwgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIHJlbmRlcigpIHtcbiAgICB2YXIge3NvcnREaXIsIGNvbHVtbktleSwgdGl0bGUsIC4uLnByb3BzfSA9IHRoaXMucHJvcHM7XG5cbiAgICByZXR1cm4gKFxuICAgICAgPEdydlRhYmxlQ2VsbCB7Li4ucHJvcHN9PlxuICAgICAgICA8YSBvbkNsaWNrPXt0aGlzLm9uU29ydENoYW5nZX0+XG4gICAgICAgICAge3RpdGxlfVxuICAgICAgICA8L2E+XG4gICAgICAgIDxTb3J0SW5kaWNhdG9yIHNvcnREaXI9e3NvcnREaXJ9Lz5cbiAgICAgIDwvR3J2VGFibGVDZWxsPlxuICAgICk7XG4gIH0sXG5cbiAgb25Tb3J0Q2hhbmdlKGUpIHtcbiAgICBlLnByZXZlbnREZWZhdWx0KCk7XG4gICAgaWYodGhpcy5wcm9wcy5vblNvcnRDaGFuZ2UpIHtcbiAgICAgIC8vIGRlZmF1bHRcbiAgICAgIGxldCBuZXdEaXIgPSBTb3J0VHlwZXMuREVTQztcbiAgICAgIGlmKHRoaXMucHJvcHMuc29ydERpcil7XG4gICAgICAgIG5ld0RpciA9IHRoaXMucHJvcHMuc29ydERpciA9PT0gU29ydFR5cGVzLkRFU0MgPyBTb3J0VHlwZXMuQVNDIDogU29ydFR5cGVzLkRFU0M7XG4gICAgICB9XG4gICAgICB0aGlzLnByb3BzLm9uU29ydENoYW5nZSh0aGlzLnByb3BzLmNvbHVtbktleSwgbmV3RGlyKTtcbiAgICB9XG4gIH1cbn0pO1xuXG4vKipcbiogRGVmYXVsdCBDZWxsXG4qL1xudmFyIEdydlRhYmxlQ2VsbCA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgcmVuZGVyKCl7XG4gICAgdmFyIHByb3BzID0gdGhpcy5wcm9wcztcbiAgICByZXR1cm4gcHJvcHMuaXNIZWFkZXIgPyA8dGgga2V5PXtwcm9wcy5rZXl9IGNsYXNzTmFtZT1cImdydi10YWJsZS1jZWxsXCI+e3Byb3BzLmNoaWxkcmVufTwvdGg+IDogPHRkIGtleT17cHJvcHMua2V5fT57cHJvcHMuY2hpbGRyZW59PC90ZD47XG4gIH1cbn0pO1xuXG4vKipcbiogVGFibGVcbiovXG52YXIgR3J2VGFibGUgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgcmVuZGVySGVhZGVyKGNoaWxkcmVuKXtcbiAgICB2YXIgY2VsbHMgPSBjaGlsZHJlbi5tYXAoKGl0ZW0sIGluZGV4KT0+e1xuICAgICAgcmV0dXJuIHRoaXMucmVuZGVyQ2VsbChpdGVtLnByb3BzLmhlYWRlciwge2luZGV4LCBrZXk6IGluZGV4LCBpc0hlYWRlcjogdHJ1ZSwgLi4uaXRlbS5wcm9wc30pO1xuICAgIH0pXG5cbiAgICByZXR1cm4gPHRoZWFkIGNsYXNzTmFtZT1cImdydi10YWJsZS1oZWFkZXJcIj48dHI+e2NlbGxzfTwvdHI+PC90aGVhZD5cbiAgfSxcblxuICByZW5kZXJCb2R5KGNoaWxkcmVuKXtcbiAgICB2YXIgY291bnQgPSB0aGlzLnByb3BzLnJvd0NvdW50O1xuICAgIHZhciByb3dzID0gW107XG4gICAgZm9yKHZhciBpID0gMDsgaSA8IGNvdW50OyBpICsrKXtcbiAgICAgIHZhciBjZWxscyA9IGNoaWxkcmVuLm1hcCgoaXRlbSwgaW5kZXgpPT57XG4gICAgICAgIHJldHVybiB0aGlzLnJlbmRlckNlbGwoaXRlbS5wcm9wcy5jZWxsLCB7cm93SW5kZXg6IGksIGtleTogaW5kZXgsIGlzSGVhZGVyOiBmYWxzZSwgLi4uaXRlbS5wcm9wc30pO1xuICAgICAgfSlcblxuICAgICAgcm93cy5wdXNoKDx0ciBrZXk9e2l9PntjZWxsc308L3RyPik7XG4gICAgfVxuXG4gICAgcmV0dXJuIDx0Ym9keT57cm93c308L3Rib2R5PjtcbiAgfSxcblxuICByZW5kZXJDZWxsKGNlbGwsIGNlbGxQcm9wcyl7XG4gICAgdmFyIGNvbnRlbnQgPSBudWxsO1xuICAgIGlmIChSZWFjdC5pc1ZhbGlkRWxlbWVudChjZWxsKSkge1xuICAgICAgIGNvbnRlbnQgPSBSZWFjdC5jbG9uZUVsZW1lbnQoY2VsbCwgY2VsbFByb3BzKTtcbiAgICAgfSBlbHNlIGlmICh0eXBlb2YgcHJvcHMuY2VsbCA9PT0gJ2Z1bmN0aW9uJykge1xuICAgICAgIGNvbnRlbnQgPSBjZWxsKGNlbGxQcm9wcyk7XG4gICAgIH1cblxuICAgICByZXR1cm4gY29udGVudDtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgdmFyIGNoaWxkcmVuID0gW107XG4gICAgUmVhY3QuQ2hpbGRyZW4uZm9yRWFjaCh0aGlzLnByb3BzLmNoaWxkcmVuLCAoY2hpbGQsIGluZGV4KSA9PiB7XG4gICAgICBpZiAoY2hpbGQgPT0gbnVsbCkge1xuICAgICAgICByZXR1cm47XG4gICAgICB9XG5cbiAgICAgIGlmKGNoaWxkLnR5cGUuZGlzcGxheU5hbWUgIT09ICdHcnZUYWJsZUNvbHVtbicpe1xuICAgICAgICB0aHJvdyAnU2hvdWxkIGJlIEdydlRhYmxlQ29sdW1uJztcbiAgICAgIH1cblxuICAgICAgY2hpbGRyZW4ucHVzaChjaGlsZCk7XG4gICAgfSk7XG5cbiAgICB2YXIgdGFibGVDbGFzcyA9ICd0YWJsZSAnICsgdGhpcy5wcm9wcy5jbGFzc05hbWU7XG5cbiAgICByZXR1cm4gKFxuICAgICAgPHRhYmxlIGNsYXNzTmFtZT17dGFibGVDbGFzc30+XG4gICAgICAgIHt0aGlzLnJlbmRlckhlYWRlcihjaGlsZHJlbil9XG4gICAgICAgIHt0aGlzLnJlbmRlckJvZHkoY2hpbGRyZW4pfVxuICAgICAgPC90YWJsZT5cbiAgICApO1xuICB9XG59KVxuXG52YXIgR3J2VGFibGVDb2x1bW4gPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgdGhyb3cgbmV3IEVycm9yKCdDb21wb25lbnQgPEdydlRhYmxlQ29sdW1uIC8+IHNob3VsZCBuZXZlciByZW5kZXInKTtcbiAgfVxufSlcblxuZXhwb3J0IGRlZmF1bHQgR3J2VGFibGU7XG5leHBvcnQge1xuICBHcnZUYWJsZUNvbHVtbiBhcyBDb2x1bW4sXG4gIEdydlRhYmxlIGFzIFRhYmxlLFxuICBHcnZUYWJsZUNlbGwgYXMgQ2VsbCxcbiAgR3J2VGFibGVUZXh0Q2VsbCBhcyBUZXh0Q2VsbCxcbiAgU29ydEhlYWRlckNlbGwsXG4gIFNvcnRJbmRpY2F0b3IsXG4gIFNvcnRUeXBlc307XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy90YWJsZS5qc3hcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cyA9IF87XG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiBleHRlcm5hbCBcIl9cIlxuICoqIG1vZHVsZSBpZCA9IDYxXG4gKiogbW9kdWxlIGNodW5rcyA9IDFcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgc2Vzc2lvbiA9IHJlcXVpcmUoJ2FwcC9zZXNzaW9uJyk7XG52YXIge3V1aWR9ID0gcmVxdWlyZSgnYXBwL3V0aWxzJyk7XG52YXIgYXBpID0gcmVxdWlyZSgnYXBwL3NlcnZpY2VzL2FwaScpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcbnZhciBnZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG52YXIgc2Vzc2lvbk1vZHVsZSA9IHJlcXVpcmUoJy4vLi4vc2Vzc2lvbnMnKTtcblxudmFyIHsgVExQVF9URVJNX09QRU4sIFRMUFRfVEVSTV9DTE9TRSwgVExQVF9URVJNX0NIQU5HRV9TRVJWRVIgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxudmFyIGFjdGlvbnMgPSB7XG5cbiAgY2hhbmdlU2VydmVyKHNlcnZlcklkLCBsb2dpbil7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1RFUk1fQ0hBTkdFX1NFUlZFUiwge1xuICAgICAgc2VydmVySWQsXG4gICAgICBsb2dpblxuICAgIH0pO1xuICB9LFxuXG4gIGNsb3NlKCl7XG4gICAgbGV0IHtpc05ld1Nlc3Npb259ID0gcmVhY3Rvci5ldmFsdWF0ZShnZXR0ZXJzLmFjdGl2ZVNlc3Npb24pO1xuXG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1RFUk1fQ0xPU0UpO1xuXG4gICAgaWYoaXNOZXdTZXNzaW9uKXtcbiAgICAgIHNlc3Npb24uZ2V0SGlzdG9yeSgpLnB1c2goY2ZnLnJvdXRlcy5ub2Rlcyk7XG4gICAgfWVsc2V7XG4gICAgICBzZXNzaW9uLmdldEhpc3RvcnkoKS5wdXNoKGNmZy5yb3V0ZXMuc2Vzc2lvbnMpO1xuICAgIH1cbiAgfSxcblxuICByZXNpemUodywgaCl7XG4gICAgLy8gc29tZSBtaW4gdmFsdWVzXG4gICAgdyA9IHcgPCA1ID8gNSA6IHc7XG4gICAgaCA9IGggPCA1ID8gNSA6IGg7XG5cbiAgICBsZXQgcmVxRGF0YSA9IHsgdGVybWluYWxfcGFyYW1zOiB7IHcsIGggfSB9O1xuICAgIGxldCB7c2lkfSA9IHJlYWN0b3IuZXZhbHVhdGUoZ2V0dGVycy5hY3RpdmVTZXNzaW9uKTtcblxuICAgIGFwaS5wdXQoY2ZnLmFwaS5nZXRUZXJtaW5hbFNlc3Npb25Vcmwoc2lkKSwgcmVxRGF0YSlcbiAgICAgIC5kb25lKCgpPT57XG4gICAgICAgIGNvbnNvbGUubG9nKGByZXNpemUgd2l0aCB3OiR7d30gYW5kIGg6JHtofSAtIE9LYCk7XG4gICAgICB9KVxuICAgICAgLmZhaWwoKCk9PntcbiAgICAgICAgY29uc29sZS5sb2coYGZhaWxlZCB0byByZXNpemUgd2l0aCB3OiR7d30gYW5kIGg6JHtofWApO1xuICAgIH0pXG4gIH0sXG5cbiAgb3BlblNlc3Npb24oc2lkKXtcbiAgICBzZXNzaW9uTW9kdWxlLmFjdGlvbnMuZmV0Y2hTZXNzaW9uKHNpZClcbiAgICAgIC5kb25lKCgpPT57XG4gICAgICAgIGxldCBzVmlldyA9IHJlYWN0b3IuZXZhbHVhdGUoc2Vzc2lvbk1vZHVsZS5nZXR0ZXJzLnNlc3Npb25WaWV3QnlJZChzaWQpKTtcbiAgICAgICAgbGV0IHsgc2VydmVySWQsIGxvZ2luIH0gPSBzVmlldztcbiAgICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1RFUk1fT1BFTiwge1xuICAgICAgICAgICAgc2VydmVySWQsXG4gICAgICAgICAgICBsb2dpbixcbiAgICAgICAgICAgIHNpZCxcbiAgICAgICAgICAgIGlzTmV3U2Vzc2lvbjogZmFsc2VcbiAgICAgICAgICB9KTtcbiAgICAgIH0pXG4gICAgICAuZmFpbCgoKT0+e1xuICAgICAgICBzZXNzaW9uLmdldEhpc3RvcnkoKS5wdXNoKGNmZy5yb3V0ZXMucGFnZU5vdEZvdW5kKTtcbiAgICAgIH0pXG4gIH0sXG5cbiAgY3JlYXRlTmV3U2Vzc2lvbihzZXJ2ZXJJZCwgbG9naW4pe1xuICAgIHZhciBzaWQgPSB1dWlkKCk7XG4gICAgdmFyIHJvdXRlVXJsID0gY2ZnLmdldEFjdGl2ZVNlc3Npb25Sb3V0ZVVybChzaWQpO1xuICAgIHZhciBoaXN0b3J5ID0gc2Vzc2lvbi5nZXRIaXN0b3J5KCk7XG5cbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfVEVSTV9PUEVOLCB7XG4gICAgICBzZXJ2ZXJJZCxcbiAgICAgIGxvZ2luLFxuICAgICAgc2lkLFxuICAgICAgaXNOZXdTZXNzaW9uOiB0cnVlXG4gICAgfSk7XG5cbiAgICBoaXN0b3J5LnB1c2gocm91dGVVcmwpO1xuICB9XG5cbn1cblxuZXhwb3J0IGRlZmF1bHQgYWN0aW9ucztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGlvbnMuanNcbiAqKi8iLCJ2YXIge2NyZWF0ZVZpZXd9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvc2Vzc2lvbnMvZ2V0dGVycycpO1xuXG5jb25zdCBhY3RpdmVTZXNzaW9uID0gW1xuWyd0bHB0X2FjdGl2ZV90ZXJtaW5hbCddLCBbJ3RscHRfc2Vzc2lvbnMnXSxcbihhY3RpdmVUZXJtLCBzZXNzaW9ucykgPT4ge1xuICAgIGlmKCFhY3RpdmVUZXJtKXtcbiAgICAgIHJldHVybiBudWxsO1xuICAgIH1cblxuICAgIC8qXG4gICAgKiBhY3RpdmUgc2Vzc2lvbiBuZWVkcyB0byBoYXZlIGl0cyBvd24gdmlldyBhcyBhbiBhY3R1YWwgc2Vzc2lvbiBtaWdodCBub3RcbiAgICAqIGV4aXN0IGF0IHRoaXMgcG9pbnQuIEZvciBleGFtcGxlLCB1cG9uIGNyZWF0aW5nIGEgbmV3IHNlc3Npb24gd2UgbmVlZCB0byBrbm93XG4gICAgKiBsb2dpbiBhbmQgc2VydmVySWQuIEl0IHdpbGwgYmUgc2ltcGxpZmllZCBvbmNlIHNlcnZlciBBUEkgZ2V0cyBleHRlbmRlZC5cbiAgICAqL1xuICAgIGxldCBhc1ZpZXcgPSB7XG4gICAgICBpc05ld1Nlc3Npb246IGFjdGl2ZVRlcm0uZ2V0KCdpc05ld1Nlc3Npb24nKSxcbiAgICAgIG5vdEZvdW5kOiBhY3RpdmVUZXJtLmdldCgnbm90Rm91bmQnKSxcbiAgICAgIGFkZHI6IGFjdGl2ZVRlcm0uZ2V0KCdhZGRyJyksXG4gICAgICBzZXJ2ZXJJZDogYWN0aXZlVGVybS5nZXQoJ3NlcnZlcklkJyksXG4gICAgICBzZXJ2ZXJJcDogdW5kZWZpbmVkLFxuICAgICAgbG9naW46IGFjdGl2ZVRlcm0uZ2V0KCdsb2dpbicpLFxuICAgICAgc2lkOiBhY3RpdmVUZXJtLmdldCgnc2lkJyksXG4gICAgICBjb2xzOiB1bmRlZmluZWQsXG4gICAgICByb3dzOiB1bmRlZmluZWRcbiAgICB9O1xuXG4gICAgLy8gaW4gY2FzZSBpZiBzZXNzaW9uIGFscmVhZHkgZXhpc3RzLCBnZXQgdGhlIGRhdGEgZnJvbSB0aGVyZVxuICAgIC8vIChmb3IgZXhhbXBsZSwgd2hlbiBqb2luaW5nIGFuIGV4aXN0aW5nIHNlc3Npb24pXG4gICAgaWYoc2Vzc2lvbnMuaGFzKGFzVmlldy5zaWQpKXtcbiAgICAgIGxldCBzVmlldyA9IGNyZWF0ZVZpZXcoc2Vzc2lvbnMuZ2V0KGFzVmlldy5zaWQpKTtcblxuICAgICAgYXNWaWV3LnBhcnRpZXMgPSBzVmlldy5wYXJ0aWVzO1xuICAgICAgYXNWaWV3LnNlcnZlcklwID0gc1ZpZXcuc2VydmVySXA7XG4gICAgICBhc1ZpZXcuc2VydmVySWQgPSBzVmlldy5zZXJ2ZXJJZDtcbiAgICAgIGFzVmlldy5hY3RpdmUgPSBzVmlldy5hY3RpdmU7XG4gICAgICBhc1ZpZXcuY29scyA9IHNWaWV3LmNvbHM7XG4gICAgICBhc1ZpZXcucm93cyA9IHNWaWV3LnJvd3M7XG4gICAgfVxuXG4gICAgcmV0dXJuIGFzVmlldztcblxuICB9XG5dO1xuXG5leHBvcnQgZGVmYXVsdCB7XG4gIGFjdGl2ZVNlc3Npb25cbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2dldHRlcnMuanNcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgeyBUTFBUX0RJQUxPR19TRUxFQ1RfTk9ERV9TSE9XLCBUTFBUX0RJQUxPR19TRUxFQ1RfTk9ERV9DTE9TRSB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG52YXIgYWN0aW9ucyA9IHtcbiAgc2hvd1NlbGVjdE5vZGVEaWFsb2coKXtcbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfRElBTE9HX1NFTEVDVF9OT0RFX1NIT1cpO1xuICB9LFxuXG4gIGNsb3NlU2VsZWN0Tm9kZURpYWxvZygpe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9ESUFMT0dfU0VMRUNUX05PREVfQ0xPU0UpO1xuICB9XG59XG5cbmV4cG9ydCBkZWZhdWx0IGFjdGlvbnM7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9kaWFsb2dzL2FjdGlvbnMuanNcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgYXBpID0gcmVxdWlyZSgnYXBwL3NlcnZpY2VzL2FwaScpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxudmFyIHsgVExQVF9TRVNTSU5TX1JFQ0VJVkUsIFRMUFRfU0VTU0lOU19VUERBVEUgfSAgPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IHtcblxuICBmZXRjaFNlc3Npb24oc2lkKXtcbiAgICByZXR1cm4gYXBpLmdldChjZmcuYXBpLmdldEZldGNoU2Vzc2lvblVybChzaWQpKS50aGVuKGpzb249PntcbiAgICAgIGlmKGpzb24gJiYganNvbi5zZXNzaW9uKXtcbiAgICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1NFU1NJTlNfVVBEQVRFLCBqc29uLnNlc3Npb24pO1xuICAgICAgfVxuICAgIH0pO1xuICB9LFxuXG4gIGZldGNoU2Vzc2lvbnMoc3RhcnREYXRlLCBlbmREYXRlKXtcbiAgICByZXR1cm4gYXBpLmdldChjZmcuYXBpLmdldEZldGNoU2Vzc2lvbnNVcmwoc3RhcnREYXRlLCBlbmREYXRlKSkuZG9uZSgoanNvbikgPT4ge1xuICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1NFU1NJTlNfUkVDRUlWRSwganNvbi5zZXNzaW9ucyk7XG4gICAgfSk7XG4gIH0sXG5cbiAgdXBkYXRlU2Vzc2lvbihqc29uKXtcbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfU0VTU0lOU19VUERBVEUsIGpzb24pO1xuICB9XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9zZXNzaW9ucy9hY3Rpb25zLmpzXG4gKiovIiwidmFyIGFwaSA9IHJlcXVpcmUoJy4vc2VydmljZXMvYXBpJyk7XG52YXIgc2Vzc2lvbiA9IHJlcXVpcmUoJy4vc2Vzc2lvbicpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcbnZhciAkID0gcmVxdWlyZSgnalF1ZXJ5Jyk7XG5cbmNvbnN0IHJlZnJlc2hSYXRlID0gNjAwMDAgKiA1OyAvLyAxIG1pblxuXG52YXIgcmVmcmVzaFRva2VuVGltZXJJZCA9IG51bGw7XG5cbnZhciBhdXRoID0ge1xuXG4gIHNpZ25VcChuYW1lLCBwYXNzd29yZCwgdG9rZW4sIGludml0ZVRva2VuKXtcbiAgICB2YXIgZGF0YSA9IHt1c2VyOiBuYW1lLCBwYXNzOiBwYXNzd29yZCwgc2Vjb25kX2ZhY3Rvcl90b2tlbjogdG9rZW4sIGludml0ZV90b2tlbjogaW52aXRlVG9rZW59O1xuICAgIHJldHVybiBhcGkucG9zdChjZmcuYXBpLmNyZWF0ZVVzZXJQYXRoLCBkYXRhKVxuICAgICAgLnRoZW4oKHVzZXIpPT57XG4gICAgICAgIHNlc3Npb24uc2V0VXNlckRhdGEodXNlcik7XG4gICAgICAgIGF1dGguX3N0YXJ0VG9rZW5SZWZyZXNoZXIoKTtcbiAgICAgICAgcmV0dXJuIHVzZXI7XG4gICAgICB9KTtcbiAgfSxcblxuICBsb2dpbihuYW1lLCBwYXNzd29yZCwgdG9rZW4pe1xuICAgIGF1dGguX3N0b3BUb2tlblJlZnJlc2hlcigpO1xuICAgIHJldHVybiBhdXRoLl9sb2dpbihuYW1lLCBwYXNzd29yZCwgdG9rZW4pLmRvbmUoYXV0aC5fc3RhcnRUb2tlblJlZnJlc2hlcik7XG4gIH0sXG5cbiAgZW5zdXJlVXNlcigpe1xuICAgIHZhciB1c2VyRGF0YSA9IHNlc3Npb24uZ2V0VXNlckRhdGEoKTtcbiAgICBpZih1c2VyRGF0YS50b2tlbil7XG4gICAgICAvLyByZWZyZXNoIHRpbWVyIHdpbGwgbm90IGJlIHNldCBpbiBjYXNlIG9mIGJyb3dzZXIgcmVmcmVzaCBldmVudFxuICAgICAgaWYoYXV0aC5fZ2V0UmVmcmVzaFRva2VuVGltZXJJZCgpID09PSBudWxsKXtcbiAgICAgICAgcmV0dXJuIGF1dGguX3JlZnJlc2hUb2tlbigpLmRvbmUoYXV0aC5fc3RhcnRUb2tlblJlZnJlc2hlcik7XG4gICAgICB9XG5cbiAgICAgIHJldHVybiAkLkRlZmVycmVkKCkucmVzb2x2ZSh1c2VyRGF0YSk7XG4gICAgfVxuXG4gICAgcmV0dXJuICQuRGVmZXJyZWQoKS5yZWplY3QoKTtcbiAgfSxcblxuICBsb2dvdXQoKXtcbiAgICBhdXRoLl9zdG9wVG9rZW5SZWZyZXNoZXIoKTtcbiAgICBzZXNzaW9uLmNsZWFyKCk7XG4gICAgc2Vzc2lvbi5nZXRIaXN0b3J5KCkucmVwbGFjZSh7cGF0aG5hbWU6IGNmZy5yb3V0ZXMubG9naW59KTsgICAgXG4gIH0sXG5cbiAgX3N0YXJ0VG9rZW5SZWZyZXNoZXIoKXtcbiAgICByZWZyZXNoVG9rZW5UaW1lcklkID0gc2V0SW50ZXJ2YWwoYXV0aC5fcmVmcmVzaFRva2VuLCByZWZyZXNoUmF0ZSk7XG4gIH0sXG5cbiAgX3N0b3BUb2tlblJlZnJlc2hlcigpe1xuICAgIGNsZWFySW50ZXJ2YWwocmVmcmVzaFRva2VuVGltZXJJZCk7XG4gICAgcmVmcmVzaFRva2VuVGltZXJJZCA9IG51bGw7XG4gIH0sXG5cbiAgX2dldFJlZnJlc2hUb2tlblRpbWVySWQoKXtcbiAgICByZXR1cm4gcmVmcmVzaFRva2VuVGltZXJJZDtcbiAgfSxcblxuICBfcmVmcmVzaFRva2VuKCl7XG4gICAgcmV0dXJuIGFwaS5wb3N0KGNmZy5hcGkucmVuZXdUb2tlblBhdGgpLnRoZW4oZGF0YT0+e1xuICAgICAgc2Vzc2lvbi5zZXRVc2VyRGF0YShkYXRhKTtcbiAgICAgIHJldHVybiBkYXRhO1xuICAgIH0pLmZhaWwoKCk9PntcbiAgICAgIGF1dGgubG9nb3V0KCk7XG4gICAgfSk7XG4gIH0sXG5cbiAgX2xvZ2luKG5hbWUsIHBhc3N3b3JkLCB0b2tlbil7XG4gICAgdmFyIGRhdGEgPSB7XG4gICAgICB1c2VyOiBuYW1lLFxuICAgICAgcGFzczogcGFzc3dvcmQsXG4gICAgICBzZWNvbmRfZmFjdG9yX3Rva2VuOiB0b2tlblxuICAgIH07XG5cbiAgICByZXR1cm4gYXBpLnBvc3QoY2ZnLmFwaS5zZXNzaW9uUGF0aCwgZGF0YSwgZmFsc2UpLnRoZW4oZGF0YT0+e1xuICAgICAgc2Vzc2lvbi5zZXRVc2VyRGF0YShkYXRhKTtcbiAgICAgIHJldHVybiBkYXRhO1xuICAgIH0pO1xuICB9XG59XG5cbm1vZHVsZS5leHBvcnRzID0gYXV0aDtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9hdXRoLmpzXG4gKiovIiwidmFyIG1vbWVudCA9IHJlcXVpcmUoJ21vbWVudCcpO1xuXG5tb2R1bGUuZXhwb3J0cy5tb250aFJhbmdlID0gZnVuY3Rpb24odmFsdWUgPSBuZXcgRGF0ZSgpKXtcbiAgbGV0IHN0YXJ0RGF0ZSA9IG1vbWVudCh2YWx1ZSkuc3RhcnRPZignbW9udGgnKS50b0RhdGUoKTtcbiAgbGV0IGVuZERhdGUgPSBtb21lbnQodmFsdWUpLmVuZE9mKCdtb250aCcpLnRvRGF0ZSgpO1xuICByZXR1cm4gW3N0YXJ0RGF0ZSwgZW5kRGF0ZV07XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tbW9uL2RhdGVVdGlscy5qc1xuICoqLyIsIm1vZHVsZS5leHBvcnRzLmlzTWF0Y2ggPSBmdW5jdGlvbihvYmosIHNlYXJjaFZhbHVlLCB7c2VhcmNoYWJsZVByb3BzLCBjYn0pIHtcbiAgc2VhcmNoVmFsdWUgPSBzZWFyY2hWYWx1ZS50b0xvY2FsZVVwcGVyQ2FzZSgpO1xuICBsZXQgcHJvcE5hbWVzID0gc2VhcmNoYWJsZVByb3BzIHx8IE9iamVjdC5nZXRPd25Qcm9wZXJ0eU5hbWVzKG9iaik7XG4gIGZvciAobGV0IGkgPSAwOyBpIDwgcHJvcE5hbWVzLmxlbmd0aDsgaSsrKSB7XG4gICAgbGV0IHRhcmdldFZhbHVlID0gb2JqW3Byb3BOYW1lc1tpXV07XG4gICAgaWYgKHRhcmdldFZhbHVlKSB7XG4gICAgICBpZih0eXBlb2YgY2IgPT09ICdmdW5jdGlvbicpe1xuICAgICAgICBsZXQgcmVzdWx0ID0gY2IodGFyZ2V0VmFsdWUsIHNlYXJjaFZhbHVlLCBwcm9wTmFtZXNbaV0pO1xuICAgICAgICBpZihyZXN1bHQgPT09IHRydWUpe1xuICAgICAgICAgIHJldHVybiByZXN1bHQ7XG4gICAgICAgIH1cbiAgICAgIH1cblxuICAgICAgaWYgKHRhcmdldFZhbHVlLnRvU3RyaW5nKCkudG9Mb2NhbGVVcHBlckNhc2UoKS5pbmRleE9mKHNlYXJjaFZhbHVlKSAhPT0gLTEpIHtcbiAgICAgICAgcmV0dXJuIHRydWU7XG4gICAgICB9XG4gICAgfVxuICB9XG5cbiAgcmV0dXJuIGZhbHNlO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbW1vbi9vYmplY3RVdGlscy5qc1xuICoqLyIsInZhciBFdmVudEVtaXR0ZXIgPSByZXF1aXJlKCdldmVudHMnKS5FdmVudEVtaXR0ZXI7XG52YXIgc2Vzc2lvbiA9IHJlcXVpcmUoJ2FwcC9zZXNzaW9uJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xudmFyIHthY3Rpb25zfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsLycpO1xuXG5jbGFzcyBUdHkgZXh0ZW5kcyBFdmVudEVtaXR0ZXIge1xuXG4gIGNvbnN0cnVjdG9yKHtzZXJ2ZXJJZCwgbG9naW4sIHNpZCwgcm93cywgY29scyB9KXtcbiAgICBzdXBlcigpO1xuICAgIHRoaXMub3B0aW9ucyA9IHsgc2VydmVySWQsIGxvZ2luLCBzaWQsIHJvd3MsIGNvbHMgfTtcbiAgICB0aGlzLnNvY2tldCA9IG51bGw7XG4gIH1cblxuICBkaXNjb25uZWN0KCl7XG4gICAgdGhpcy5zb2NrZXQuY2xvc2UoKTtcbiAgfVxuXG4gIHJlY29ubmVjdChvcHRpb25zKXtcbiAgICB0aGlzLmRpc2Nvbm5lY3QoKTtcbiAgICB0aGlzLnNvY2tldC5vbm9wZW4gPSBudWxsO1xuICAgIHRoaXMuc29ja2V0Lm9ubWVzc2FnZSA9IG51bGw7XG4gICAgdGhpcy5zb2NrZXQub25jbG9zZSA9IG51bGw7XG4gICAgXG4gICAgdGhpcy5jb25uZWN0KG9wdGlvbnMpO1xuICB9XG5cbiAgY29ubmVjdChvcHRpb25zKXtcbiAgICBPYmplY3QuYXNzaWduKHRoaXMub3B0aW9ucywgb3B0aW9ucyk7XG5cbiAgICBsZXQge3Rva2VufSA9IHNlc3Npb24uZ2V0VXNlckRhdGEoKTtcbiAgICBsZXQgY29ublN0ciA9IGNmZy5hcGkuZ2V0VHR5Q29ublN0cih7dG9rZW4sIC4uLnRoaXMub3B0aW9uc30pO1xuXG4gICAgdGhpcy5zb2NrZXQgPSBuZXcgV2ViU29ja2V0KGNvbm5TdHIsICdwcm90bycpO1xuXG4gICAgdGhpcy5zb2NrZXQub25vcGVuID0gKCkgPT4ge1xuICAgICAgdGhpcy5lbWl0KCdvcGVuJyk7XG4gICAgfVxuXG4gICAgdGhpcy5zb2NrZXQub25tZXNzYWdlID0gKGUpPT57XG4gICAgICB0aGlzLmVtaXQoJ2RhdGEnLCBlLmRhdGEpO1xuICAgIH1cblxuICAgIHRoaXMuc29ja2V0Lm9uY2xvc2UgPSAoKT0+e1xuICAgICAgdGhpcy5lbWl0KCdjbG9zZScpO1xuICAgIH1cbiAgfVxuXG4gIHJlc2l6ZShjb2xzLCByb3dzKXtcbiAgICBhY3Rpb25zLnJlc2l6ZShjb2xzLCByb3dzKTtcbiAgfVxuXG4gIHNlbmQoZGF0YSl7XG4gICAgdGhpcy5zb2NrZXQuc2VuZChkYXRhKTtcbiAgfVxufVxuXG5tb2R1bGUuZXhwb3J0cyA9IFR0eTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21tb24vdHR5LmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfVEVSTV9PUEVOOiBudWxsLFxuICBUTFBUX1RFUk1fQ0xPU0U6IG51bGwsXG4gIFRMUFRfVEVSTV9DSEFOR0VfU0VSVkVSOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvYWN0aW9uVHlwZXMuanNcbiAqKi8iLCJ2YXIgeyBTdG9yZSwgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcbnZhciB7IFRMUFRfVEVSTV9PUEVOLCBUTFBUX1RFUk1fQ0xPU0UsIFRMUFRfVEVSTV9DSEFOR0VfU0VSVkVSIH0gID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gdG9JbW11dGFibGUobnVsbCk7XG4gIH0sXG5cbiAgaW5pdGlhbGl6ZSgpIHtcbiAgICB0aGlzLm9uKFRMUFRfVEVSTV9PUEVOLCBzZXRBY3RpdmVUZXJtaW5hbCk7XG4gICAgdGhpcy5vbihUTFBUX1RFUk1fQ0xPU0UsIGNsb3NlKTtcbiAgICB0aGlzLm9uKFRMUFRfVEVSTV9DSEFOR0VfU0VSVkVSLCBjaGFuZ2VTZXJ2ZXIpO1xuICB9XG59KVxuXG5mdW5jdGlvbiBjaGFuZ2VTZXJ2ZXIoc3RhdGUsIHtzZXJ2ZXJJZCwgbG9naW59KXtcbiAgcmV0dXJuIHN0YXRlLnNldCgnc2VydmVySWQnLCBzZXJ2ZXJJZClcbiAgICAgICAgICAgICAgLnNldCgnbG9naW4nLCBsb2dpbik7XG59XG5cbmZ1bmN0aW9uIGNsb3NlKCl7XG4gIHJldHVybiB0b0ltbXV0YWJsZShudWxsKTtcbn1cblxuZnVuY3Rpb24gc2V0QWN0aXZlVGVybWluYWwoc3RhdGUsIHtzZXJ2ZXJJZCwgbG9naW4sIHNpZCwgaXNOZXdTZXNzaW9ufSApe1xuICByZXR1cm4gdG9JbW11dGFibGUoe1xuICAgIHNlcnZlcklkLFxuICAgIGxvZ2luLFxuICAgIHNpZCxcbiAgICBpc05ld1Nlc3Npb25cbiAgfSk7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3RpdmVUZXJtU3RvcmUuanNcbiAqKi8iLCJpbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVExQVF9BUFBfSU5JVDogbnVsbCxcbiAgVExQVF9BUFBfRkFJTEVEOiBudWxsLFxuICBUTFBUX0FQUF9SRUFEWTogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2FwcC9hY3Rpb25UeXBlcy5qc1xuICoqLyIsInZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xuXG52YXIgeyBUTFBUX0FQUF9JTklULCBUTFBUX0FQUF9GQUlMRUQsIFRMUFRfQVBQX1JFQURZIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbnZhciBpbml0U3RhdGUgPSB0b0ltbXV0YWJsZSh7XG4gIGlzUmVhZHk6IGZhbHNlLFxuICBpc0luaXRpYWxpemluZzogZmFsc2UsXG4gIGlzRmFpbGVkOiBmYWxzZVxufSk7XG5cbmV4cG9ydCBkZWZhdWx0IFN0b3JlKHtcblxuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIGluaXRTdGF0ZS5zZXQoJ2lzSW5pdGlhbGl6aW5nJywgdHJ1ZSk7XG4gIH0sXG5cbiAgaW5pdGlhbGl6ZSgpIHtcbiAgICB0aGlzLm9uKFRMUFRfQVBQX0lOSVQsICgpPT4gaW5pdFN0YXRlLnNldCgnaXNJbml0aWFsaXppbmcnLCB0cnVlKSk7XG4gICAgdGhpcy5vbihUTFBUX0FQUF9SRUFEWSwoKT0+IGluaXRTdGF0ZS5zZXQoJ2lzUmVhZHknLCB0cnVlKSk7XG4gICAgdGhpcy5vbihUTFBUX0FQUF9GQUlMRUQsKCk9PiBpbml0U3RhdGUuc2V0KCdpc0ZhaWxlZCcsIHRydWUpKTtcbiAgfVxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2FwcC9hcHBTdG9yZS5qc1xuICoqLyIsImltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX0RJQUxPR19TRUxFQ1RfTk9ERV9TSE9XOiBudWxsLFxuICBUTFBUX0RJQUxPR19TRUxFQ1RfTk9ERV9DTE9TRTogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvYWN0aW9uVHlwZXMuanNcbiAqKi8iLCJ2YXIgeyBTdG9yZSwgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcblxudmFyIHsgVExQVF9ESUFMT0dfU0VMRUNUX05PREVfU0hPVywgVExQVF9ESUFMT0dfU0VMRUNUX05PREVfQ0xPU0UgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuXG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gdG9JbW11dGFibGUoe1xuICAgICAgaXNTZWxlY3ROb2RlRGlhbG9nT3BlbjogZmFsc2VcbiAgICB9KTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9ESUFMT0dfU0VMRUNUX05PREVfU0hPVywgc2hvd1NlbGVjdE5vZGVEaWFsb2cpO1xuICAgIHRoaXMub24oVExQVF9ESUFMT0dfU0VMRUNUX05PREVfQ0xPU0UsIGNsb3NlU2VsZWN0Tm9kZURpYWxvZyk7XG4gIH1cbn0pXG5cbmZ1bmN0aW9uIHNob3dTZWxlY3ROb2RlRGlhbG9nKHN0YXRlKXtcbiAgcmV0dXJuIHN0YXRlLnNldCgnaXNTZWxlY3ROb2RlRGlhbG9nT3BlbicsIHRydWUpO1xufVxuXG5mdW5jdGlvbiBjbG9zZVNlbGVjdE5vZGVEaWFsb2coc3RhdGUpe1xuICByZXR1cm4gc3RhdGUuc2V0KCdpc1NlbGVjdE5vZGVEaWFsb2dPcGVuJywgZmFsc2UpO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvZGlhbG9ncy9kaWFsb2dTdG9yZS5qc1xuICoqLyIsImltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX1JFQ0VJVkVfVVNFUl9JTlZJVEU6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvYWN0aW9uVHlwZXMuanNcbiAqKi8iLCJ2YXIgeyBTdG9yZSwgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcbnZhciAgeyBUTFBUX1JFQ0VJVkVfVVNFUl9JTlZJVEUgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKG51bGwpO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX1JFQ0VJVkVfVVNFUl9JTlZJVEUsIHJlY2VpdmVJbnZpdGUpXG4gIH1cbn0pXG5cbmZ1bmN0aW9uIHJlY2VpdmVJbnZpdGUoc3RhdGUsIGludml0ZSl7XG4gIHJldHVybiB0b0ltbXV0YWJsZShpbnZpdGUpO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2ludml0ZVN0b3JlLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfTk9ERVNfUkVDRUlWRTogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2FjdGlvblR5cGVzLmpzXG4gKiovIiwidmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHsgVExQVF9OT0RFU19SRUNFSVZFIH0gID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xudmFyIGFwaSA9IHJlcXVpcmUoJ2FwcC9zZXJ2aWNlcy9hcGknKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgZmV0Y2hOb2Rlcygpe1xuICAgIGFwaS5nZXQoY2ZnLmFwaS5ub2Rlc1BhdGgpLmRvbmUoKGRhdGE9W10pPT57XG4gICAgICB2YXIgbm9kZUFycmF5ID0gZGF0YS5ub2Rlcy5tYXAoaXRlbT0+aXRlbS5ub2RlKTtcbiAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9OT0RFU19SRUNFSVZFLCBub2RlQXJyYXkpO1xuICAgIH0pO1xuICB9XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9hY3Rpb25zLmpzXG4gKiovIiwidmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG52YXIgIHsgVExQVF9OT0RFU19SRUNFSVZFIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IFN0b3JlKHtcbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB0b0ltbXV0YWJsZShbXSk7XG4gIH0sXG5cbiAgaW5pdGlhbGl6ZSgpIHtcbiAgICB0aGlzLm9uKFRMUFRfTk9ERVNfUkVDRUlWRSwgcmVjZWl2ZU5vZGVzKVxuICB9XG59KVxuXG5mdW5jdGlvbiByZWNlaXZlTm9kZXMoc3RhdGUsIG5vZGVBcnJheSl7XG4gIHJldHVybiB0b0ltbXV0YWJsZShub2RlQXJyYXkpO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvbm9kZVN0b3JlLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfUkVTVF9BUElfU1RBUlQ6IG51bGwsXG4gIFRMUFRfUkVTVF9BUElfU1VDQ0VTUzogbnVsbCxcbiAgVExQVF9SRVNUX0FQSV9GQUlMOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9hY3Rpb25UeXBlcy5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcblxudmFyIHtcbiAgVExQVF9SRVNUX0FQSV9TVEFSVCxcbiAgVExQVF9SRVNUX0FQSV9TVUNDRVNTLFxuICBUTFBUX1JFU1RfQVBJX0ZBSUwgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuXG4gIHN0YXJ0KHJlcVR5cGUpe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRVNUX0FQSV9TVEFSVCwge3R5cGU6IHJlcVR5cGV9KTtcbiAgfSxcblxuICBmYWlsKHJlcVR5cGUsIG1lc3NhZ2Upe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRVNUX0FQSV9GQUlMLCAge3R5cGU6IHJlcVR5cGUsIG1lc3NhZ2V9KTtcbiAgfSxcblxuICBzdWNjZXNzKHJlcVR5cGUpe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRVNUX0FQSV9TVUNDRVNTLCB7dHlwZTogcmVxVHlwZX0pO1xuICB9XG5cbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvYWN0aW9ucy5qc1xuICoqLyIsInZhciBkZWZhdWx0T2JqID0ge1xuICBpc1Byb2Nlc3Npbmc6IGZhbHNlLFxuICBpc0Vycm9yOiBmYWxzZSxcbiAgaXNTdWNjZXNzOiBmYWxzZSxcbiAgbWVzc2FnZTogJydcbn1cblxuY29uc3QgcmVxdWVzdFN0YXR1cyA9IChyZXFUeXBlKSA9PiAgWyBbJ3RscHRfcmVzdF9hcGknLCByZXFUeXBlXSwgKGF0dGVtcCkgPT4ge1xuICByZXR1cm4gYXR0ZW1wID8gYXR0ZW1wLnRvSlMoKSA6IGRlZmF1bHRPYmo7XG4gfVxuXTtcblxuZXhwb3J0IGRlZmF1bHQgeyAgcmVxdWVzdFN0YXR1cyAgfTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvZ2V0dGVycy5qc1xuICoqLyIsImltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX1NFU1NJTlNfUkVDRUlWRTogbnVsbCxcbiAgVExQVF9TRVNTSU5TX1VQREFURTogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2FjdGlvblR5cGVzLmpzXG4gKiovIiwidmFyIHsgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG5cbmNvbnN0IHNlc3Npb25zQnlTZXJ2ZXIgPSAoc2VydmVySWQpID0+IFtbJ3RscHRfc2Vzc2lvbnMnXSwgKHNlc3Npb25zKSA9PntcbiAgcmV0dXJuIHNlc3Npb25zLnZhbHVlU2VxKCkuZmlsdGVyKGl0ZW09PntcbiAgICB2YXIgcGFydGllcyA9IGl0ZW0uZ2V0KCdwYXJ0aWVzJykgfHwgdG9JbW11dGFibGUoW10pO1xuICAgIHZhciBoYXNTZXJ2ZXIgPSBwYXJ0aWVzLmZpbmQoaXRlbTI9PiBpdGVtMi5nZXQoJ3NlcnZlcl9pZCcpID09PSBzZXJ2ZXJJZCk7XG4gICAgcmV0dXJuIGhhc1NlcnZlcjtcbiAgfSkudG9MaXN0KCk7XG59XVxuXG5jb25zdCBzZXNzaW9uc1ZpZXcgPSBbWyd0bHB0X3Nlc3Npb25zJ10sIChzZXNzaW9ucykgPT57XG4gIHJldHVybiBzZXNzaW9ucy52YWx1ZVNlcSgpLm1hcChjcmVhdGVWaWV3KS50b0pTKCk7XG59XTtcblxuY29uc3Qgc2Vzc2lvblZpZXdCeUlkID0gKHNpZCk9PiBbWyd0bHB0X3Nlc3Npb25zJywgc2lkXSwgKHNlc3Npb24pPT57XG4gIGlmKCFzZXNzaW9uKXtcbiAgICByZXR1cm4gbnVsbDtcbiAgfVxuXG4gIHJldHVybiBjcmVhdGVWaWV3KHNlc3Npb24pO1xufV07XG5cbmNvbnN0IHBhcnRpZXNCeVNlc3Npb25JZCA9IChzaWQpID0+XG4gW1sndGxwdF9zZXNzaW9ucycsIHNpZCwgJ3BhcnRpZXMnXSwgKHBhcnRpZXMpID0+e1xuXG4gIGlmKCFwYXJ0aWVzKXtcbiAgICByZXR1cm4gW107XG4gIH1cblxuICB2YXIgbGFzdEFjdGl2ZVVzck5hbWUgPSBnZXRMYXN0QWN0aXZlVXNlcihwYXJ0aWVzKS5nZXQoJ3VzZXInKTtcblxuICByZXR1cm4gcGFydGllcy5tYXAoaXRlbT0+e1xuICAgIHZhciB1c2VyID0gaXRlbS5nZXQoJ3VzZXInKTtcbiAgICByZXR1cm4ge1xuICAgICAgdXNlcjogaXRlbS5nZXQoJ3VzZXInKSxcbiAgICAgIHNlcnZlcklwOiBpdGVtLmdldCgncmVtb3RlX2FkZHInKSxcbiAgICAgIHNlcnZlcklkOiBpdGVtLmdldCgnc2VydmVyX2lkJyksXG4gICAgICBpc0FjdGl2ZTogbGFzdEFjdGl2ZVVzck5hbWUgPT09IHVzZXJcbiAgICB9XG4gIH0pLnRvSlMoKTtcbn1dO1xuXG5mdW5jdGlvbiBnZXRMYXN0QWN0aXZlVXNlcihwYXJ0aWVzKXtcbiAgcmV0dXJuIHBhcnRpZXMuc29ydEJ5KGl0ZW09PiBuZXcgRGF0ZShpdGVtLmdldCgnbGFzdEFjdGl2ZScpKSkuZmlyc3QoKTtcbn1cblxuZnVuY3Rpb24gY3JlYXRlVmlldyhzZXNzaW9uKXtcbiAgdmFyIHNpZCA9IHNlc3Npb24uZ2V0KCdpZCcpO1xuICB2YXIgc2VydmVySXAsIHNlcnZlcklkO1xuICB2YXIgcGFydGllcyA9IHJlYWN0b3IuZXZhbHVhdGUocGFydGllc0J5U2Vzc2lvbklkKHNpZCkpO1xuXG4gIGlmKHBhcnRpZXMubGVuZ3RoID4gMCl7XG4gICAgc2VydmVySXAgPSBwYXJ0aWVzWzBdLnNlcnZlcklwO1xuICAgIHNlcnZlcklkID0gcGFydGllc1swXS5zZXJ2ZXJJZDtcbiAgfVxuXG4gIHJldHVybiB7XG4gICAgc2lkOiBzaWQsXG4gICAgc2Vzc2lvblVybDogY2ZnLmdldEFjdGl2ZVNlc3Npb25Sb3V0ZVVybChzaWQpLFxuICAgIHNlcnZlcklwLFxuICAgIHNlcnZlcklkLFxuICAgIGFjdGl2ZTogc2Vzc2lvbi5nZXQoJ2FjdGl2ZScpLFxuICAgIGNyZWF0ZWQ6IG5ldyBEYXRlKHNlc3Npb24uZ2V0KCdjcmVhdGVkJykpLFxuICAgIGxhc3RBY3RpdmU6IG5ldyBEYXRlKHNlc3Npb24uZ2V0KCdsYXN0X2FjdGl2ZScpKSxcbiAgICBsb2dpbjogc2Vzc2lvbi5nZXQoJ2xvZ2luJyksXG4gICAgcGFydGllczogcGFydGllcyxcbiAgICBjb2xzOiBzZXNzaW9uLmdldEluKFsndGVybWluYWxfcGFyYW1zJywgJ3cnXSksXG4gICAgcm93czogc2Vzc2lvbi5nZXRJbihbJ3Rlcm1pbmFsX3BhcmFtcycsICdoJ10pXG4gIH1cbn1cblxuZXhwb3J0IGRlZmF1bHQge1xuICBwYXJ0aWVzQnlTZXNzaW9uSWQsXG4gIHNlc3Npb25zQnlTZXJ2ZXIsXG4gIHNlc3Npb25zVmlldyxcbiAgc2Vzc2lvblZpZXdCeUlkLFxuICBjcmVhdGVWaWV3XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9zZXNzaW9ucy9nZXR0ZXJzLmpzXG4gKiovIiwidmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG52YXIgeyBUTFBUX1NFU1NJTlNfUkVDRUlWRSwgVExQVF9TRVNTSU5TX1VQREFURSB9ICA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKHt9KTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9TRVNTSU5TX1JFQ0VJVkUsIHJlY2VpdmVTZXNzaW9ucyk7XG4gICAgdGhpcy5vbihUTFBUX1NFU1NJTlNfVVBEQVRFLCB1cGRhdGVTZXNzaW9uKTtcbiAgfVxufSlcblxuZnVuY3Rpb24gdXBkYXRlU2Vzc2lvbihzdGF0ZSwganNvbil7XG4gIHJldHVybiBzdGF0ZS5zZXQoanNvbi5pZCwgdG9JbW11dGFibGUoanNvbikpO1xufVxuXG5mdW5jdGlvbiByZWNlaXZlU2Vzc2lvbnMoc3RhdGUsIGpzb25BcnJheT1bXSl7XG4gIHJldHVybiBzdGF0ZS53aXRoTXV0YXRpb25zKHN0YXRlID0+IHtcbiAgICBqc29uQXJyYXkuZm9yRWFjaCgoaXRlbSkgPT4ge1xuICAgICAgc3RhdGUuc2V0KGl0ZW0uaWQsIHRvSW1tdXRhYmxlKGl0ZW0pKVxuICAgIH0pXG4gIH0pO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvc2Vzc2lvblN0b3JlLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfUkVDRUlWRV9VU0VSOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvdXNlci9hY3Rpb25UeXBlcy5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7IFRMUFRfUkVDRUlWRV9VU0VSIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG52YXIgeyBUUllJTkdfVE9fU0lHTl9VUCwgVFJZSU5HX1RPX0xPR0lOfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3Jlc3RBcGkvY29uc3RhbnRzJyk7XG52YXIgcmVzdEFwaUFjdGlvbnMgPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9yZXN0QXBpL2FjdGlvbnMnKTtcbnZhciBhdXRoID0gcmVxdWlyZSgnYXBwL2F1dGgnKTtcbnZhciBzZXNzaW9uID0gcmVxdWlyZSgnYXBwL3Nlc3Npb24nKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG5cbmV4cG9ydCBkZWZhdWx0IHtcblxuICBlbnN1cmVVc2VyKG5leHRTdGF0ZSwgcmVwbGFjZSwgY2Ipe1xuICAgIGF1dGguZW5zdXJlVXNlcigpXG4gICAgICAuZG9uZSgodXNlckRhdGEpPT4ge1xuICAgICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfUkVDRUlWRV9VU0VSLCB1c2VyRGF0YS51c2VyICk7XG4gICAgICAgIGNiKCk7XG4gICAgICB9KVxuICAgICAgLmZhaWwoKCk9PntcbiAgICAgICAgcmVwbGFjZSh7cmVkaXJlY3RUbzogbmV4dFN0YXRlLmxvY2F0aW9uLnBhdGhuYW1lIH0sIGNmZy5yb3V0ZXMubG9naW4pO1xuICAgICAgICBjYigpO1xuICAgICAgfSk7XG4gIH0sXG5cbiAgc2lnblVwKHtuYW1lLCBwc3csIHRva2VuLCBpbnZpdGVUb2tlbn0pe1xuICAgIHJlc3RBcGlBY3Rpb25zLnN0YXJ0KFRSWUlOR19UT19TSUdOX1VQKTtcbiAgICBhdXRoLnNpZ25VcChuYW1lLCBwc3csIHRva2VuLCBpbnZpdGVUb2tlbilcbiAgICAgIC5kb25lKChzZXNzaW9uRGF0YSk9PntcbiAgICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFQ0VJVkVfVVNFUiwgc2Vzc2lvbkRhdGEudXNlcik7XG4gICAgICAgIHJlc3RBcGlBY3Rpb25zLnN1Y2Nlc3MoVFJZSU5HX1RPX1NJR05fVVApO1xuICAgICAgICBzZXNzaW9uLmdldEhpc3RvcnkoKS5wdXNoKHtwYXRobmFtZTogY2ZnLnJvdXRlcy5hcHB9KTtcbiAgICAgIH0pXG4gICAgICAuZmFpbCgoZXJyKT0+e1xuICAgICAgICByZXN0QXBpQWN0aW9ucy5mYWlsKFRSWUlOR19UT19TSUdOX1VQLCBlcnIucmVzcG9uc2VKU09OLm1lc3NhZ2UgfHwgJ2ZhaWxlZCB0byBzaW5nIHVwJyk7XG4gICAgICB9KTtcbiAgfSxcblxuICBsb2dpbih7dXNlciwgcGFzc3dvcmQsIHRva2VufSwgcmVkaXJlY3Qpe1xuICAgIHJlc3RBcGlBY3Rpb25zLnN0YXJ0KFRSWUlOR19UT19MT0dJTik7XG4gICAgYXV0aC5sb2dpbih1c2VyLCBwYXNzd29yZCwgdG9rZW4pXG4gICAgICAuZG9uZSgoc2Vzc2lvbkRhdGEpPT57XG4gICAgICAgIHJlc3RBcGlBY3Rpb25zLnN1Y2Nlc3MoVFJZSU5HX1RPX0xPR0lOKTtcbiAgICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFQ0VJVkVfVVNFUiwgc2Vzc2lvbkRhdGEudXNlcik7XG4gICAgICAgIHNlc3Npb24uZ2V0SGlzdG9yeSgpLnB1c2goe3BhdGhuYW1lOiByZWRpcmVjdH0pO1xuICAgICAgfSlcbiAgICAgIC5mYWlsKChlcnIpPT4gcmVzdEFwaUFjdGlvbnMuZmFpbChUUllJTkdfVE9fTE9HSU4sIGVyci5yZXNwb25zZUpTT04ubWVzc2FnZSkpXG4gICAgfVxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvdXNlci9hY3Rpb25zLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMuZ2V0dGVycyA9IHJlcXVpcmUoJy4vZ2V0dGVycycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aW9ucyA9IHJlcXVpcmUoJy4vYWN0aW9ucycpO1xubW9kdWxlLmV4cG9ydHMubm9kZVN0b3JlID0gcmVxdWlyZSgnLi91c2VyU3RvcmUnKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvaW5kZXguanNcbiAqKi8iLCJ2YXIgeyBTdG9yZSwgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcbnZhciAgeyBUTFBUX1JFQ0VJVkVfVVNFUiB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gdG9JbW11dGFibGUobnVsbCk7XG4gIH0sXG5cbiAgaW5pdGlhbGl6ZSgpIHtcbiAgICB0aGlzLm9uKFRMUFRfUkVDRUlWRV9VU0VSLCByZWNlaXZlVXNlcilcbiAgfVxuXG59KVxuXG5mdW5jdGlvbiByZWNlaXZlVXNlcihzdGF0ZSwgdXNlcil7XG4gIHJldHVybiB0b0ltbXV0YWJsZSh1c2VyKTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvdXNlclN0b3JlLmpzXG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciB7YWN0aW9uc30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC8nKTtcbnZhciBjb2xvcnMgPSBbJyMxYWIzOTQnLCAnIzFjODRjNicsICcjMjNjNmM4JywgJyNmOGFjNTknLCAnI0VENTU2NScsICcjYzJjMmMyJ107XG5cbmNvbnN0IFVzZXJJY29uID0gKHtuYW1lLCB0aXRsZSwgY29sb3JJbmRleD0wfSk9PntcbiAgbGV0IGNvbG9yID0gY29sb3JzW2NvbG9ySW5kZXggJSBjb2xvcnMubGVuZ3RoXTtcbiAgbGV0IHN0eWxlID0ge1xuICAgICdiYWNrZ3JvdW5kQ29sb3InOiBjb2xvcixcbiAgICAnYm9yZGVyQ29sb3InOiBjb2xvclxuICB9O1xuXG4gIHJldHVybiAoXG4gICAgPGxpPlxuICAgICAgPHNwYW4gc3R5bGU9e3N0eWxlfSBjbGFzc05hbWU9XCJidG4gYnRuLXByaW1hcnkgYnRuLWNpcmNsZSB0ZXh0LXVwcGVyY2FzZVwiPlxuICAgICAgICA8c3Ryb25nPntuYW1lWzBdfTwvc3Ryb25nPlxuICAgICAgPC9zcGFuPlxuICAgIDwvbGk+XG4gIClcbn07XG5cbmNvbnN0IFNlc3Npb25MZWZ0UGFuZWwgPSAoe3BhcnRpZXN9KSA9PiB7XG4gIHBhcnRpZXMgPSBwYXJ0aWVzIHx8IFtdO1xuICBsZXQgdXNlckljb25zID0gcGFydGllcy5tYXAoKGl0ZW0sIGluZGV4KT0+KFxuICAgIDxVc2VySWNvbiBrZXk9e2luZGV4fSBjb2xvckluZGV4PXtpbmRleH0gbmFtZT17aXRlbS51c2VyfS8+XG4gICkpO1xuXG4gIHJldHVybiAoXG4gICAgPGRpdiBjbGFzc05hbWU9XCJncnYtdGVybWluYWwtcGFydGljaXBhbnNcIj5cbiAgICAgIDx1bCBjbGFzc05hbWU9XCJuYXZcIj5cbiAgICAgICAge3VzZXJJY29uc31cbiAgICAgICAgPGxpPlxuICAgICAgICAgIDxidXR0b24gb25DbGljaz17YWN0aW9ucy5jbG9zZX0gY2xhc3NOYW1lPVwiYnRuIGJ0bi1kYW5nZXIgYnRuLWNpcmNsZVwiIHR5cGU9XCJidXR0b25cIj5cbiAgICAgICAgICAgIDxpIGNsYXNzTmFtZT1cImZhIGZhLXRpbWVzXCI+PC9pPlxuICAgICAgICAgIDwvYnV0dG9uPlxuICAgICAgICA8L2xpPlxuICAgICAgPC91bD5cbiAgICA8L2Rpdj5cbiAgKVxufTtcblxubW9kdWxlLmV4cG9ydHMgPSBTZXNzaW9uTGVmdFBhbmVsO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvY3VycmVudFNlc3Npb24vc2Vzc2lvbkxlZnRQYW5lbC5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyICQgPSByZXF1aXJlKCdqUXVlcnknKTtcbnZhciBtb21lbnQgPSByZXF1aXJlKCdtb21lbnQnKTtcbnZhciB7ZGVib3VuY2V9ID0gcmVxdWlyZSgnXycpO1xuXG52YXIgRGF0ZVJhbmdlUGlja2VyID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIGdldERhdGVzKCl7XG4gICAgdmFyIHN0YXJ0RGF0ZSA9ICQodGhpcy5yZWZzLmRwUGlja2VyMSkuZGF0ZXBpY2tlcignZ2V0RGF0ZScpO1xuICAgIHZhciBlbmREYXRlID0gJCh0aGlzLnJlZnMuZHBQaWNrZXIyKS5kYXRlcGlja2VyKCdnZXREYXRlJyk7XG4gICAgcmV0dXJuIFtzdGFydERhdGUsIGVuZERhdGVdO1xuICB9LFxuXG4gIHNldERhdGVzKHtzdGFydERhdGUsIGVuZERhdGV9KXtcbiAgICAkKHRoaXMucmVmcy5kcFBpY2tlcjEpLmRhdGVwaWNrZXIoJ3NldERhdGUnLCBzdGFydERhdGUpO1xuICAgICQodGhpcy5yZWZzLmRwUGlja2VyMikuZGF0ZXBpY2tlcignc2V0RGF0ZScsIGVuZERhdGUpO1xuICB9LFxuXG4gIGdldERlZmF1bHRQcm9wcygpIHtcbiAgICAgcmV0dXJuIHtcbiAgICAgICBzdGFydERhdGU6IG1vbWVudCgpLnN0YXJ0T2YoJ21vbnRoJykudG9EYXRlKCksXG4gICAgICAgZW5kRGF0ZTogbW9tZW50KCkuZW5kT2YoJ21vbnRoJykudG9EYXRlKCksXG4gICAgICAgb25DaGFuZ2U6ICgpPT57fVxuICAgICB9O1xuICAgfSxcblxuICBjb21wb25lbnRXaWxsVW5tb3VudCgpe1xuICAgICQodGhpcy5yZWZzLmRwKS5kYXRlcGlja2VyKCdkZXN0cm95Jyk7XG4gIH0sXG5cbiAgY29tcG9uZW50V2lsbFJlY2VpdmVQcm9wcyhuZXdQcm9wcyl7XG4gICAgdmFyIFtzdGFydERhdGUsIGVuZERhdGVdID0gdGhpcy5nZXREYXRlcygpO1xuICAgIGlmKCEoaXNTYW1lKHN0YXJ0RGF0ZSwgbmV3UHJvcHMuc3RhcnREYXRlKSAmJlxuICAgICAgICAgIGlzU2FtZShlbmREYXRlLCBuZXdQcm9wcy5lbmREYXRlKSkpe1xuICAgICAgICB0aGlzLnNldERhdGVzKG5ld1Byb3BzKTtcbiAgICAgIH1cbiAgfSxcblxuICBzaG91bGRDb21wb25lbnRVcGRhdGUoKXtcbiAgICByZXR1cm4gZmFsc2U7XG4gIH0sXG5cbiAgY29tcG9uZW50RGlkTW91bnQoKXtcbiAgICB0aGlzLm9uQ2hhbmdlID0gZGVib3VuY2UodGhpcy5vbkNoYW5nZSwgMSk7XG4gICAgJCh0aGlzLnJlZnMucmFuZ2VQaWNrZXIpLmRhdGVwaWNrZXIoe1xuICAgICAgdG9kYXlCdG46ICdsaW5rZWQnLFxuICAgICAga2V5Ym9hcmROYXZpZ2F0aW9uOiBmYWxzZSxcbiAgICAgIGZvcmNlUGFyc2U6IGZhbHNlLFxuICAgICAgY2FsZW5kYXJXZWVrczogdHJ1ZSxcbiAgICAgIGF1dG9jbG9zZTogdHJ1ZVxuICAgIH0pLm9uKCdjaGFuZ2VEYXRlJywgdGhpcy5vbkNoYW5nZSk7XG5cbiAgICB0aGlzLnNldERhdGVzKHRoaXMucHJvcHMpO1xuICB9LFxuXG4gIG9uQ2hhbmdlKCl7XG4gICAgdmFyIFtzdGFydERhdGUsIGVuZERhdGVdID0gdGhpcy5nZXREYXRlcygpXG4gICAgaWYoIShpc1NhbWUoc3RhcnREYXRlLCB0aGlzLnByb3BzLnN0YXJ0RGF0ZSkgJiZcbiAgICAgICAgICBpc1NhbWUoZW5kRGF0ZSwgdGhpcy5wcm9wcy5lbmREYXRlKSkpe1xuICAgICAgICB0aGlzLnByb3BzLm9uQ2hhbmdlKHtzdGFydERhdGUsIGVuZERhdGV9KTtcbiAgICB9XG4gIH0sXG5cbiAgcmVuZGVyKCkge1xuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1kYXRlcGlja2VyIGlucHV0LWdyb3VwIGlucHV0LWRhdGVyYW5nZVwiIHJlZj1cInJhbmdlUGlja2VyXCI+ICAgICAgICBcbiAgICAgICAgPGlucHV0IHJlZj1cImRwUGlja2VyMVwiIHR5cGU9XCJ0ZXh0XCIgY2xhc3NOYW1lPVwiaW5wdXQtc20gZm9ybS1jb250cm9sXCIgbmFtZT1cInN0YXJ0XCIgLz5cbiAgICAgICAgPHNwYW4gY2xhc3NOYW1lPVwiaW5wdXQtZ3JvdXAtYWRkb25cIj50bzwvc3Bhbj5cbiAgICAgICAgPGlucHV0IHJlZj1cImRwUGlja2VyMlwiIHR5cGU9XCJ0ZXh0XCIgY2xhc3NOYW1lPVwiaW5wdXQtc20gZm9ybS1jb250cm9sXCIgbmFtZT1cImVuZFwiIC8+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KTtcblxuZnVuY3Rpb24gaXNTYW1lKGRhdGUxLCBkYXRlMil7XG4gIHJldHVybiBtb21lbnQoZGF0ZTEpLmlzU2FtZShkYXRlMiwgJ2RheScpO1xufVxuXG4vKipcbiogQ2FsZW5kYXIgTmF2XG4qL1xudmFyIENhbGVuZGFyTmF2ID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIHJlbmRlcigpIHtcbiAgICBsZXQge3ZhbHVlfSA9IHRoaXMucHJvcHM7XG4gICAgbGV0IGRpc3BsYXlWYWx1ZSA9IG1vbWVudCh2YWx1ZSkuZm9ybWF0KCdNTU1NLCBZWVlZJyk7XG5cbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9e1wiZ3J2LWNhbGVuZGFyLW5hdiBcIiArIHRoaXMucHJvcHMuY2xhc3NOYW1lfSA+XG4gICAgICAgIDxidXR0b24gb25DbGljaz17dGhpcy5tb3ZlLmJpbmQodGhpcywgLTEpfSBjbGFzc05hbWU9XCJidG4gYnRuLW91dGxpbmUgYnRuLWxpbmtcIj48aSBjbGFzc05hbWU9XCJmYSBmYS1jaGV2cm9uLWxlZnRcIj48L2k+PC9idXR0b24+XG4gICAgICAgIDxzcGFuIGNsYXNzTmFtZT1cInRleHQtbXV0ZWRcIj57ZGlzcGxheVZhbHVlfTwvc3Bhbj5cbiAgICAgICAgPGJ1dHRvbiBvbkNsaWNrPXt0aGlzLm1vdmUuYmluZCh0aGlzLCAxKX0gY2xhc3NOYW1lPVwiYnRuIGJ0bi1vdXRsaW5lIGJ0bi1saW5rXCI+PGkgY2xhc3NOYW1lPVwiZmEgZmEtY2hldnJvbi1yaWdodFwiPjwvaT48L2J1dHRvbj5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH0sXG5cbiAgbW92ZShhdCl7XG4gICAgbGV0IHt2YWx1ZX0gPSB0aGlzLnByb3BzO1xuICAgIGxldCBuZXdWYWx1ZSA9IG1vbWVudCh2YWx1ZSkuYWRkKGF0LCAnbW9udGgnKS50b0RhdGUoKTtcbiAgICB0aGlzLnByb3BzLm9uVmFsdWVDaGFuZ2UobmV3VmFsdWUpO1xuICB9XG59KTtcblxuQ2FsZW5kYXJOYXYuZ2V0TW9udGhSYW5nZSA9IGZ1bmN0aW9uKHZhbHVlKXtcbiAgbGV0IHN0YXJ0RGF0ZSA9IG1vbWVudCh2YWx1ZSkuc3RhcnRPZignbW9udGgnKS50b0RhdGUoKTtcbiAgbGV0IGVuZERhdGUgPSBtb21lbnQodmFsdWUpLmVuZE9mKCdtb250aCcpLnRvRGF0ZSgpO1xuICByZXR1cm4gW3N0YXJ0RGF0ZSwgZW5kRGF0ZV07XG59XG5cbmV4cG9ydCBkZWZhdWx0IERhdGVSYW5nZVBpY2tlcjtcbmV4cG9ydCB7Q2FsZW5kYXJOYXYsIERhdGVSYW5nZVBpY2tlcn07XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9kYXRlUGlja2VyLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG5cbnZhciBOb3RGb3VuZCA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgcmVuZGVyKCkge1xuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1lcnJvci1wYWdlXCI+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWxvZ28tdHBydFwiPlRlbGVwb3J0PC9kaXY+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXdhcm5pbmdcIj48aSBjbGFzc05hbWU9XCJmYSBmYS13YXJuaW5nXCI+PC9pPiA8L2Rpdj5cbiAgICAgICAgPGgxPldob29wcywgd2UgY2Fubm90IGZpbmQgdGhhdDwvaDE+XG4gICAgICAgIDxkaXY+TG9va3MgbGlrZSB0aGUgcGFnZSB5b3UgYXJlIGxvb2tpbmcgZm9yIGlzbid0IGhlcmUgYW55IGxvbmdlcjwvZGl2PlxuICAgICAgICA8ZGl2PklmIHlvdSBiZWxpZXZlIHRoaXMgaXMgYW4gZXJyb3IsIHBsZWFzZSBjb250YWN0IHlvdXIgb3JnYW5pemF0aW9uIGFkbWluaXN0cmF0b3IuPC9kaXY+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiY29udGFjdC1zZWN0aW9uXCI+SWYgeW91IGJlbGlldmUgdGhpcyBpcyBhbiBpc3N1ZSB3aXRoIFRlbGVwb3J0LCBwbGVhc2UgPGEgaHJlZj1cImh0dHBzOi8vZ2l0aHViLmNvbS9ncmF2aXRhdGlvbmFsL3RlbGVwb3J0L2lzc3Vlcy9uZXdcIj5jcmVhdGUgYSBHaXRIdWIgaXNzdWUuPC9hPlxuICAgICAgICAgPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KVxuXG52YXIgRXhwaXJlZEludml0ZSA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgcmVuZGVyKCkge1xuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1lcnJvci1wYWdlXCI+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWxvZ28tdHBydFwiPlRlbGVwb3J0PC9kaXY+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXdhcm5pbmdcIj48aSBjbGFzc05hbWU9XCJmYSBmYS13YXJuaW5nXCI+PC9pPiA8L2Rpdj5cbiAgICAgICAgPGgxPkludml0ZSBjb2RlIGhhcyBleHBpcmVkPC9oMT5cbiAgICAgICAgPGRpdj5Mb29rcyBsaWtlIHlvdXIgaW52aXRlIGNvZGUgaXNuJ3QgdmFsaWQgYW55bW9yZTwvZGl2PlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImNvbnRhY3Qtc2VjdGlvblwiPklmIHlvdSBiZWxpZXZlIHRoaXMgaXMgYW4gaXNzdWUgd2l0aCBUZWxlcG9ydCwgcGxlYXNlIDxhIGhyZWY9XCJodHRwczovL2dpdGh1Yi5jb20vZ3Jhdml0YXRpb25hbC90ZWxlcG9ydC9pc3N1ZXMvbmV3XCI+Y3JlYXRlIGEgR2l0SHViIGlzc3VlLjwvYT5cbiAgICAgICAgIDwvZGl2PlxuICAgICAgPC9kaXY+XG4gICAgKTtcbiAgfVxufSlcblxuZXhwb3J0IGRlZmF1bHQgTm90Rm91bmQ7XG5leHBvcnQge05vdEZvdW5kLCBFeHBpcmVkSW52aXRlfVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvZXJyb3JQYWdlLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG5cbnZhciBHb29nbGVBdXRoSW5mbyA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgcmVuZGVyKCkge1xuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1nb29nbGUtYXV0aFwiPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1nb29nbGUtYXV0aC1pY29uXCI+PC9kaXY+XG4gICAgICAgIDxzdHJvbmc+R29vZ2xlIEF1dGhlbnRpY2F0b3I8L3N0cm9uZz5cbiAgICAgICAgPGRpdj5Eb3dubG9hZCA8YSBocmVmPVwiaHR0cHM6Ly9zdXBwb3J0Lmdvb2dsZS5jb20vYWNjb3VudHMvYW5zd2VyLzEwNjY0NDc/aGw9ZW5cIj5Hb29nbGUgQXV0aGVudGljYXRvcjwvYT4gb24geW91ciBwaG9uZSB0byBhY2Nlc3MgeW91ciB0d28gZmFjdG9yeSB0b2tlbjwvZGl2PlxuICAgICAgPC9kaXY+XG4gICAgKTtcbiAgfVxufSlcblxubW9kdWxlLmV4cG9ydHMgPSBHb29nbGVBdXRoSW5mbztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2dvb2dsZUF1dGhMb2dvLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIge2dldHRlcnMsIGFjdGlvbnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvbm9kZXMnKTtcbnZhciB1c2VyR2V0dGVycyA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3VzZXIvZ2V0dGVycycpO1xudmFyIHtUYWJsZSwgQ29sdW1uLCBDZWxsLCBTb3J0SGVhZGVyQ2VsbCwgU29ydFR5cGVzfSA9IHJlcXVpcmUoJ2FwcC9jb21wb25lbnRzL3RhYmxlLmpzeCcpO1xudmFyIHtjcmVhdGVOZXdTZXNzaW9ufSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGlvbnMnKTtcbnZhciBMaW5rZWRTdGF0ZU1peGluID0gcmVxdWlyZSgncmVhY3QtYWRkb25zLWxpbmtlZC1zdGF0ZS1taXhpbicpO1xudmFyIF8gPSByZXF1aXJlKCdfJyk7XG52YXIge2lzTWF0Y2h9ID0gcmVxdWlyZSgnYXBwL2NvbW1vbi9vYmplY3RVdGlscycpO1xuXG5jb25zdCBUZXh0Q2VsbCA9ICh7cm93SW5kZXgsIGRhdGEsIGNvbHVtbktleSwgLi4ucHJvcHN9KSA9PiAoXG4gIDxDZWxsIHsuLi5wcm9wc30+XG4gICAge2RhdGFbcm93SW5kZXhdW2NvbHVtbktleV19XG4gIDwvQ2VsbD5cbik7XG5cbmNvbnN0IFRhZ0NlbGwgPSAoe3Jvd0luZGV4LCBkYXRhLCBjb2x1bW5LZXksIC4uLnByb3BzfSkgPT4gKFxuICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgIHsgZGF0YVtyb3dJbmRleF0udGFncy5tYXAoKGl0ZW0sIGluZGV4KSA9PlxuICAgICAgKDxzcGFuIGtleT17aW5kZXh9IGNsYXNzTmFtZT1cImxhYmVsIGxhYmVsLWRlZmF1bHRcIj5cbiAgICAgICAge2l0ZW0ucm9sZX0gPGxpIGNsYXNzTmFtZT1cImZhIGZhLWxvbmctYXJyb3ctcmlnaHRcIj48L2xpPlxuICAgICAgICB7aXRlbS52YWx1ZX1cbiAgICAgIDwvc3Bhbj4pXG4gICAgKSB9XG4gIDwvQ2VsbD5cbik7XG5cbmNvbnN0IExvZ2luQ2VsbCA9ICh7bG9naW5zLCBvbkxvZ2luQ2xpY2ssIHJvd0luZGV4LCBkYXRhLCAuLi5wcm9wc30pID0+IHtcbiAgaWYoIWxvZ2lucyB8fGxvZ2lucy5sZW5ndGggPT09IDApe1xuICAgIHJldHVybiA8Q2VsbCB7Li4ucHJvcHN9IC8+O1xuICB9XG5cbiAgdmFyIHNlcnZlcklkID0gZGF0YVtyb3dJbmRleF0uaWQ7XG4gIHZhciAkbGlzID0gW107XG5cbiAgZnVuY3Rpb24gb25DbGljayhpKXtcbiAgICB2YXIgbG9naW4gPSBsb2dpbnNbaV07XG4gICAgaWYob25Mb2dpbkNsaWNrKXtcbiAgICAgIHJldHVybiAoKT0+IG9uTG9naW5DbGljayhzZXJ2ZXJJZCwgbG9naW4pO1xuICAgIH1lbHNle1xuICAgICAgcmV0dXJuICgpID0+IGNyZWF0ZU5ld1Nlc3Npb24oc2VydmVySWQsIGxvZ2luKTtcbiAgICB9XG4gIH1cblxuICBmb3IodmFyIGkgPSAwOyBpIDwgbG9naW5zLmxlbmd0aDsgaSsrKXtcbiAgICAkbGlzLnB1c2goPGxpIGtleT17aX0+PGEgb25DbGljaz17b25DbGljayhpKX0+e2xvZ2luc1tpXX08L2E+PC9saT4pO1xuICB9XG5cbiAgcmV0dXJuIChcbiAgICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJidG4tZ3JvdXBcIj5cbiAgICAgICAgPGJ1dHRvbiB0eXBlPVwiYnV0dG9uXCIgb25DbGljaz17b25DbGljaygwKX0gY2xhc3NOYW1lPVwiYnRuIGJ0bi14cyBidG4tcHJpbWFyeVwiPntsb2dpbnNbMF19PC9idXR0b24+XG4gICAgICAgIHtcbiAgICAgICAgICAkbGlzLmxlbmd0aCA+IDEgPyAoXG4gICAgICAgICAgICAgIFtcbiAgICAgICAgICAgICAgICA8YnV0dG9uIGtleT17MH0gZGF0YS10b2dnbGU9XCJkcm9wZG93blwiIGNsYXNzTmFtZT1cImJ0biBidG4tZGVmYXVsdCBidG4teHMgZHJvcGRvd24tdG9nZ2xlXCIgYXJpYS1leHBhbmRlZD1cInRydWVcIj5cbiAgICAgICAgICAgICAgICAgIDxzcGFuIGNsYXNzTmFtZT1cImNhcmV0XCI+PC9zcGFuPlxuICAgICAgICAgICAgICAgIDwvYnV0dG9uPixcbiAgICAgICAgICAgICAgICA8dWwga2V5PXsxfSBjbGFzc05hbWU9XCJkcm9wZG93bi1tZW51XCI+XG4gICAgICAgICAgICAgICAgICB7JGxpc31cbiAgICAgICAgICAgICAgICA8L3VsPlxuICAgICAgICAgICAgICBdIClcbiAgICAgICAgICAgIDogbnVsbFxuICAgICAgICB9XG4gICAgICA8L2Rpdj5cbiAgICA8L0NlbGw+XG4gIClcbn07XG5cbnZhciBOb2RlTGlzdCA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBtaXhpbnM6IFtMaW5rZWRTdGF0ZU1peGluXSxcblxuICBnZXRJbml0aWFsU3RhdGUocHJvcHMpe1xuICAgIHRoaXMuc2VhcmNoYWJsZVByb3BzID0gWydhZGRyJywgJ2hvc3RuYW1lJ107XG4gICAgcmV0dXJuIHsgZmlsdGVyOiAnJywgY29sU29ydERpcnM6IHtob3N0bmFtZTogJ0RFU0MnfSB9O1xuICB9LFxuXG4gIG9uU29ydENoYW5nZShjb2x1bW5LZXksIHNvcnREaXIpIHtcbiAgICB0aGlzLnNldFN0YXRlKHtcbiAgICAgIC4uLnRoaXMuc3RhdGUsXG4gICAgICBjb2xTb3J0RGlyczoge1xuICAgICAgICBbY29sdW1uS2V5XTogc29ydERpclxuICAgICAgfVxuICAgIH0pO1xuICB9LFxuXG4gIHNvcnRBbmRGaWx0ZXIoZGF0YSl7XG4gICAgdmFyIGZpbHRlcmVkID0gZGF0YS5maWx0ZXIob2JqPT5cbiAgICAgIGlzTWF0Y2gob2JqLCB0aGlzLnN0YXRlLmZpbHRlciwgeyBzZWFyY2hhYmxlUHJvcHM6IHRoaXMuc2VhcmNoYWJsZVByb3BzfSkpO1xuXG4gICAgdmFyIGNvbHVtbktleSA9IE9iamVjdC5nZXRPd25Qcm9wZXJ0eU5hbWVzKHRoaXMuc3RhdGUuY29sU29ydERpcnMpWzBdO1xuICAgIHZhciBzb3J0RGlyID0gdGhpcy5zdGF0ZS5jb2xTb3J0RGlyc1tjb2x1bW5LZXldO1xuICAgIHZhciBzb3J0ZWQgPSBfLnNvcnRCeShmaWx0ZXJlZCwgY29sdW1uS2V5KTtcbiAgICBpZihzb3J0RGlyID09PSBTb3J0VHlwZXMuQVNDKXtcbiAgICAgIHNvcnRlZCA9IHNvcnRlZC5yZXZlcnNlKCk7XG4gICAgfVxuXG4gICAgcmV0dXJuIHNvcnRlZDtcbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIHZhciBkYXRhID0gdGhpcy5zb3J0QW5kRmlsdGVyKHRoaXMucHJvcHMubm9kZVJlY29yZHMpO1xuICAgIHZhciBsb2dpbnMgPSB0aGlzLnByb3BzLmxvZ2lucztcbiAgICB2YXIgb25Mb2dpbkNsaWNrID0gdGhpcy5wcm9wcy5vbkxvZ2luQ2xpY2s7XG5cbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbm9kZXMgZ3J2LXBhZ2VcIj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZmxleCBncnYtaGVhZGVyXCI+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZmxleC1jb2x1bW5cIj48L2Rpdj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LWNvbHVtblwiPlxuICAgICAgICAgICAgPGgxPiBOb2RlcyA8L2gxPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWZsZXgtY29sdW1uXCI+XG4gICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1zZWFyY2hcIj5cbiAgICAgICAgICAgICAgPGlucHV0IHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ2ZpbHRlcicpfSBwbGFjZWhvbGRlcj1cIlNlYXJjaC4uLlwiIGNsYXNzTmFtZT1cImZvcm0tY29udHJvbCBpbnB1dC1zbVwiLz5cbiAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICA8L2Rpdj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICA8VGFibGUgcm93Q291bnQ9e2RhdGEubGVuZ3RofSBjbGFzc05hbWU9XCJ0YWJsZS1zdHJpcGVkIGdydi1ub2Rlcy10YWJsZVwiPlxuICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJob3N0bmFtZVwiXG4gICAgICAgICAgICAgIGhlYWRlcj17XG4gICAgICAgICAgICAgICAgPFNvcnRIZWFkZXJDZWxsXG4gICAgICAgICAgICAgICAgICBzb3J0RGlyPXt0aGlzLnN0YXRlLmNvbFNvcnREaXJzLmhvc3RuYW1lfVxuICAgICAgICAgICAgICAgICAgb25Tb3J0Q2hhbmdlPXt0aGlzLm9uU29ydENoYW5nZX1cbiAgICAgICAgICAgICAgICAgIHRpdGxlPVwiTm9kZVwiXG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgfVxuICAgICAgICAgICAgICBjZWxsPXs8VGV4dENlbGwgZGF0YT17ZGF0YX0vPiB9XG4gICAgICAgICAgICAvPlxuICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJhZGRyXCJcbiAgICAgICAgICAgICAgaGVhZGVyPXtcbiAgICAgICAgICAgICAgICA8U29ydEhlYWRlckNlbGxcbiAgICAgICAgICAgICAgICAgIHNvcnREaXI9e3RoaXMuc3RhdGUuY29sU29ydERpcnMuYWRkcn1cbiAgICAgICAgICAgICAgICAgIG9uU29ydENoYW5nZT17dGhpcy5vblNvcnRDaGFuZ2V9XG4gICAgICAgICAgICAgICAgICB0aXRsZT1cIklQXCJcbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICB9XG5cbiAgICAgICAgICAgICAgY2VsbD17PFRleHRDZWxsIGRhdGE9e2RhdGF9Lz4gfVxuICAgICAgICAgICAgLz5cbiAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgY29sdW1uS2V5PVwidGFnc1wiXG4gICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+PC9DZWxsPiB9XG4gICAgICAgICAgICAgIGNlbGw9ezxUYWdDZWxsIGRhdGE9e2RhdGF9Lz4gfVxuICAgICAgICAgICAgLz5cbiAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgY29sdW1uS2V5PVwicm9sZXNcIlxuICAgICAgICAgICAgICBvbkxvZ2luQ2xpY2s9e29uTG9naW5DbGlja31cbiAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD5Mb2dpbiBhczwvQ2VsbD4gfVxuICAgICAgICAgICAgICBjZWxsPXs8TG9naW5DZWxsIGRhdGE9e2RhdGF9IGxvZ2lucz17bG9naW5zfS8+IH1cbiAgICAgICAgICAgIC8+XG4gICAgICAgICAgPC9UYWJsZT5cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApXG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IE5vZGVMaXN0O1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvbm9kZXMvbm9kZUxpc3QuanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7Z2V0dGVyc30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9kaWFsb2dzJyk7XG52YXIge2Nsb3NlU2VsZWN0Tm9kZURpYWxvZ30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9kaWFsb2dzL2FjdGlvbnMnKTtcbnZhciB7Y2hhbmdlU2VydmVyfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGlvbnMnKTtcbnZhciBOb2RlTGlzdCA9IHJlcXVpcmUoJy4vbm9kZXMvbm9kZUxpc3QuanN4Jyk7XG52YXIgYWN0aXZlU2Vzc2lvbkdldHRlcnMgPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9nZXR0ZXJzJyk7XG52YXIgbm9kZUdldHRlcnMgPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9ub2Rlcy9nZXR0ZXJzJyk7XG5cbnZhciBTZWxlY3ROb2RlRGlhbG9nID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW3JlYWN0b3IuUmVhY3RNaXhpbl0sXG5cbiAgZ2V0RGF0YUJpbmRpbmdzKCkge1xuICAgIHJldHVybiB7XG4gICAgICBkaWFsb2dzOiBnZXR0ZXJzLmRpYWxvZ3NcbiAgICB9XG4gIH0sXG5cbiAgcmVuZGVyKCkge1xuICAgIHJldHVybiB0aGlzLnN0YXRlLmRpYWxvZ3MuaXNTZWxlY3ROb2RlRGlhbG9nT3BlbiA/IDxEaWFsb2cvPiA6IG51bGw7XG4gIH1cbn0pO1xuXG52YXIgRGlhbG9nID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG9uTG9naW5DbGljayhzZXJ2ZXJJZCwgbG9naW4pe1xuICAgIGlmKFNlbGVjdE5vZGVEaWFsb2cub25TZXJ2ZXJDaGFuZ2VDYWxsQmFjayl7XG4gICAgICBTZWxlY3ROb2RlRGlhbG9nLm9uU2VydmVyQ2hhbmdlQ2FsbEJhY2soe3NlcnZlcklkfSk7XG4gICAgfVxuXG4gICAgY2xvc2VTZWxlY3ROb2RlRGlhbG9nKCk7XG4gIH0sXG5cbiAgY29tcG9uZW50V2lsbFVubW91bnQoY2FsbGJhY2spe1xuICAgICQoJy5tb2RhbCcpLm1vZGFsKCdoaWRlJyk7XG4gIH0sXG5cbiAgY29tcG9uZW50RGlkTW91bnQoKXtcbiAgICAkKCcubW9kYWwnKS5tb2RhbCgnc2hvdycpO1xuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICB2YXIgYWN0aXZlU2Vzc2lvbiA9IHJlYWN0b3IuZXZhbHVhdGUoYWN0aXZlU2Vzc2lvbkdldHRlcnMuYWN0aXZlU2Vzc2lvbikgfHwge307XG4gICAgdmFyIG5vZGVSZWNvcmRzID0gcmVhY3Rvci5ldmFsdWF0ZShub2RlR2V0dGVycy5ub2RlTGlzdFZpZXcpO1xuICAgIHZhciBsb2dpbnMgPSBbYWN0aXZlU2Vzc2lvbi5sb2dpbl07XG5cbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJtb2RhbCBmYWRlIGdydi1kaWFsb2ctc2VsZWN0LW5vZGVcIiB0YWJJbmRleD17LTF9IHJvbGU9XCJkaWFsb2dcIj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJtb2RhbC1kaWFsb2dcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIm1vZGFsLWNvbnRlbnRcIj5cbiAgICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwibW9kYWwtaGVhZGVyXCI+XG4gICAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwibW9kYWwtYm9keVwiPlxuICAgICAgICAgICAgICA8Tm9kZUxpc3Qgbm9kZVJlY29yZHM9e25vZGVSZWNvcmRzfSBsb2dpbnM9e2xvZ2luc30gb25Mb2dpbkNsaWNrPXt0aGlzLm9uTG9naW5DbGlja30vPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIm1vZGFsLWZvb3RlclwiPlxuICAgICAgICAgICAgICA8YnV0dG9uIG9uQ2xpY2s9e2Nsb3NlU2VsZWN0Tm9kZURpYWxvZ30gdHlwZT1cImJ1dHRvblwiIGNsYXNzTmFtZT1cImJ0biBidG4tcHJpbWFyeVwiPlxuICAgICAgICAgICAgICAgIENsb3NlXG4gICAgICAgICAgICAgIDwvYnV0dG9uPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9kaXY+XG4gICAgKTtcbiAgfVxufSk7XG5cblNlbGVjdE5vZGVEaWFsb2cub25TZXJ2ZXJDaGFuZ2VDYWxsQmFjayA9ICgpPT57fTtcblxubW9kdWxlLmV4cG9ydHMgPSBTZWxlY3ROb2RlRGlhbG9nO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvc2VsZWN0Tm9kZURpYWxvZy5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHsgTGluayB9ID0gcmVxdWlyZSgncmVhY3Qtcm91dGVyJyk7XG52YXIge2FjdGlvbnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvc2Vzc2lvbnMnKTtcbnZhciB7bm9kZUhvc3ROYW1lQnlTZXJ2ZXJJZH0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9ub2Rlcy9nZXR0ZXJzJyk7XG52YXIge0NlbGwsIFRleHRDZWxsfSA9IHJlcXVpcmUoJ2FwcC9jb21wb25lbnRzL3RhYmxlLmpzeCcpO1xudmFyIG1vbWVudCA9ICByZXF1aXJlKCdtb21lbnQnKTtcblxuY29uc3QgRGF0ZUNyZWF0ZWRDZWxsID0gKHsgcm93SW5kZXgsIGRhdGEsIC4uLnByb3BzIH0pID0+IHtcbiAgbGV0IGNyZWF0ZWQgPSBkYXRhW3Jvd0luZGV4XS5jcmVhdGVkO1xuICBsZXQgZGlzcGxheURhdGUgPSBtb21lbnQoY3JlYXRlZCkuZm9ybWF0KCdsIExUUycpO1xuICByZXR1cm4gKFxuICAgIDxDZWxsIHsuLi5wcm9wc30+XG4gICAgICB7IGRpc3BsYXlEYXRlIH1cbiAgICA8L0NlbGw+XG4gIClcbn07XG5cbmNvbnN0IER1cmF0aW9uQ2VsbCA9ICh7IHJvd0luZGV4LCBkYXRhLCAuLi5wcm9wcyB9KSA9PiB7XG4gIGxldCBjcmVhdGVkID0gZGF0YVtyb3dJbmRleF0uY3JlYXRlZDtcbiAgbGV0IGxhc3RBY3RpdmUgPSBkYXRhW3Jvd0luZGV4XS5sYXN0QWN0aXZlO1xuXG4gIGxldCBlbmQgPSBtb21lbnQoY3JlYXRlZCk7XG4gIGxldCBub3cgPSBtb21lbnQobGFzdEFjdGl2ZSk7XG4gIGxldCBkdXJhdGlvbiA9IG1vbWVudC5kdXJhdGlvbihub3cuZGlmZihlbmQpKTtcbiAgbGV0IGRpc3BsYXlEYXRlID0gZHVyYXRpb24uaHVtYW5pemUoKTtcblxuICByZXR1cm4gKFxuICAgIDxDZWxsIHsuLi5wcm9wc30+XG4gICAgICB7IGRpc3BsYXlEYXRlIH1cbiAgICA8L0NlbGw+XG4gIClcbn07XG5cbmNvbnN0IFNpbmdsZVVzZXJDZWxsID0gKHsgcm93SW5kZXgsIGRhdGEsIC4uLnByb3BzIH0pID0+IHtcbiAgcmV0dXJuIChcbiAgICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgICAgPHNwYW4gY2xhc3NOYW1lPVwiZ3J2LXNlc3Npb25zLXVzZXIgbGFiZWwgbGFiZWwtZGVmYXVsdFwiPntkYXRhW3Jvd0luZGV4XS5sb2dpbn08L3NwYW4+XG4gICAgPC9DZWxsPlxuICApXG59O1xuXG5jb25zdCBVc2Vyc0NlbGwgPSAoeyByb3dJbmRleCwgZGF0YSwgLi4ucHJvcHMgfSkgPT4ge1xuICBsZXQgJHVzZXJzID0gZGF0YVtyb3dJbmRleF0ucGFydGllcy5tYXAoKGl0ZW0sIGl0ZW1JbmRleCk9PlxuICAgICg8c3BhbiBrZXk9e2l0ZW1JbmRleH0gY2xhc3NOYW1lPVwiZ3J2LXNlc3Npb25zLXVzZXIgbGFiZWwgbGFiZWwtZGVmYXVsdFwiPntpdGVtLnVzZXJ9PC9zcGFuPilcbiAgKVxuXG4gIHJldHVybiAoXG4gICAgPENlbGwgey4uLnByb3BzfT5cbiAgICAgIDxkaXY+XG4gICAgICAgIHskdXNlcnN9XG4gICAgICA8L2Rpdj5cbiAgICA8L0NlbGw+XG4gIClcbn07XG5cbmNvbnN0IEJ1dHRvbkNlbGwgPSAoeyByb3dJbmRleCwgZGF0YSwgLi4ucHJvcHMgfSkgPT4ge1xuICBsZXQgeyBzZXNzaW9uVXJsLCBhY3RpdmUgfSA9IGRhdGFbcm93SW5kZXhdO1xuICBsZXQgW2FjdGlvblRleHQsIGFjdGlvbkNsYXNzXSA9IGFjdGl2ZSA/IFsnam9pbicsICdidG4td2FybmluZyddIDogWydwbGF5JywgJ2J0bi1wcmltYXJ5J107XG4gIHJldHVybiAoXG4gICAgPENlbGwgey4uLnByb3BzfT5cbiAgICAgIDxMaW5rIHRvPXtzZXNzaW9uVXJsfSBjbGFzc05hbWU9e1wiYnRuIFwiICthY3Rpb25DbGFzcysgXCIgYnRuLXhzXCJ9IHR5cGU9XCJidXR0b25cIj57YWN0aW9uVGV4dH08L0xpbms+XG4gICAgPC9DZWxsPlxuICApXG59XG5cbmNvbnN0IEVtcHR5TGlzdCA9ICh7dGV4dH0pID0+IChcbiAgPGRpdiBjbGFzc05hbWU9XCJncnYtc2Vzc2lvbnMtZW1wdHkgdGV4dC1jZW50ZXIgdGV4dC1tdXRlZFwiPjxzcGFuPnt0ZXh0fTwvc3Bhbj48L2Rpdj5cbilcblxuY29uc3QgTm9kZUNlbGwgPSAoeyByb3dJbmRleCwgZGF0YSwgLi4ucHJvcHMgfSkgPT4ge1xuICBsZXQge3NlcnZlcklkfSA9IGRhdGFbcm93SW5kZXhdO1xuICBsZXQgaG9zdG5hbWUgPSByZWFjdG9yLmV2YWx1YXRlKG5vZGVIb3N0TmFtZUJ5U2VydmVySWQoc2VydmVySWQpKSB8fCAndW5rbm93bic7XG5cbiAgcmV0dXJuIChcbiAgICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgICAge2hvc3RuYW1lfVxuICAgIDwvQ2VsbD5cbiAgKVxufVxuXG5leHBvcnQgZGVmYXVsdCBCdXR0b25DZWxsO1xuXG5leHBvcnQge1xuICBCdXR0b25DZWxsLFxuICBVc2Vyc0NlbGwsXG4gIER1cmF0aW9uQ2VsbCxcbiAgRGF0ZUNyZWF0ZWRDZWxsLFxuICBFbXB0eUxpc3QsXG4gIFNpbmdsZVVzZXJDZWxsLFxuICBOb2RlQ2VsbFxufTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL3Nlc3Npb25zL2xpc3RJdGVtcy5qc3hcbiAqKi8iLCJ2YXIgVGVybSA9IHJlcXVpcmUoJ1Rlcm1pbmFsJyk7XG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHtkZWJvdW5jZSwgaXNOdW1iZXJ9ID0gcmVxdWlyZSgnXycpO1xuXG5UZXJtLmNvbG9yc1syNTZdID0gJyMyNTIzMjMnO1xuXG5jb25zdCBESVNDT05ORUNUX1RYVCA9ICdcXHgxYlszMW1kaXNjb25uZWN0ZWRcXHgxYlttXFxyXFxuJztcbmNvbnN0IENPTk5FQ1RFRF9UWFQgPSAnQ29ubmVjdGVkIVxcclxcbic7XG5cbnZhciBUdHlUZXJtaW5hbCA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBnZXRJbml0aWFsU3RhdGUoKXtcbiAgICB0aGlzLnJvd3MgPSB0aGlzLnByb3BzLnJvd3M7XG4gICAgdGhpcy5jb2xzID0gdGhpcy5wcm9wcy5jb2xzO1xuICAgIHRoaXMudHR5ID0gdGhpcy5wcm9wcy50dHk7XG5cbiAgICB0aGlzLmRlYm91bmNlZFJlc2l6ZSA9IGRlYm91bmNlKCgpPT57XG4gICAgICB0aGlzLnJlc2l6ZSgpO1xuICAgICAgdGhpcy50dHkucmVzaXplKHRoaXMuY29scywgdGhpcy5yb3dzKTtcbiAgICB9LCAyMDApO1xuXG4gICAgcmV0dXJuIHt9O1xuICB9LFxuXG4gIGNvbXBvbmVudERpZE1vdW50OiBmdW5jdGlvbigpIHtcbiAgICB0aGlzLnRlcm0gPSBuZXcgVGVybWluYWwoe1xuICAgICAgY29sczogNSxcbiAgICAgIHJvd3M6IDUsXG4gICAgICB1c2VTdHlsZTogdHJ1ZSxcbiAgICAgIHNjcmVlbktleXM6IHRydWUsXG4gICAgICBjdXJzb3JCbGluazogdHJ1ZVxuICAgIH0pO1xuXG4gICAgdGhpcy50ZXJtLm9wZW4odGhpcy5yZWZzLmNvbnRhaW5lcik7XG4gICAgdGhpcy50ZXJtLm9uKCdkYXRhJywgKGRhdGEpID0+IHRoaXMudHR5LnNlbmQoZGF0YSkpO1xuXG4gICAgdGhpcy5yZXNpemUodGhpcy5jb2xzLCB0aGlzLnJvd3MpO1xuXG4gICAgdGhpcy50dHkub24oJ29wZW4nLCAoKT0+IHRoaXMudGVybS53cml0ZShDT05ORUNURURfVFhUKSk7XG4gICAgdGhpcy50dHkub24oJ2RhdGEnLCAoZGF0YSkgPT4gdGhpcy50ZXJtLndyaXRlKGRhdGEpKTtcbiAgICB0aGlzLnR0eS5vbigncmVzZXQnLCAoKT0+IHRoaXMudGVybS5yZXNldCgpKTtcblxuICAgIHRoaXMudHR5LmNvbm5lY3Qoe2NvbHM6IHRoaXMuY29scywgcm93czogdGhpcy5yb3dzfSk7XG4gICAgd2luZG93LmFkZEV2ZW50TGlzdGVuZXIoJ3Jlc2l6ZScsIHRoaXMuZGVib3VuY2VkUmVzaXplKTtcbiAgfSxcblxuICBjb21wb25lbnRXaWxsVW5tb3VudDogZnVuY3Rpb24oKSB7XG4gICAgdGhpcy50ZXJtLmRlc3Ryb3koKTtcbiAgICB3aW5kb3cucmVtb3ZlRXZlbnRMaXN0ZW5lcigncmVzaXplJywgdGhpcy5kZWJvdW5jZWRSZXNpemUpO1xuICB9LFxuXG4gIHNob3VsZENvbXBvbmVudFVwZGF0ZTogZnVuY3Rpb24obmV3UHJvcHMpIHtcbiAgICB2YXIge3Jvd3MsIGNvbHN9ID0gbmV3UHJvcHM7XG5cbiAgICBpZiggIWlzTnVtYmVyKHJvd3MpIHx8ICFpc051bWJlcihjb2xzKSl7XG4gICAgICByZXR1cm4gZmFsc2U7XG4gICAgfVxuXG4gICAgaWYocm93cyAhPT0gdGhpcy5yb3dzIHx8IGNvbHMgIT09IHRoaXMuY29scyl7XG4gICAgICB0aGlzLnJlc2l6ZShjb2xzLCByb3dzKVxuICAgIH1cblxuICAgIHJldHVybiBmYWxzZTtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuICggPGRpdiBjbGFzc05hbWU9XCJncnYtdGVybWluYWxcIiBpZD1cInRlcm1pbmFsLWJveFwiIHJlZj1cImNvbnRhaW5lclwiPiAgPC9kaXY+ICk7XG4gIH0sXG5cbiAgcmVzaXplOiBmdW5jdGlvbihjb2xzLCByb3dzKSB7XG4gICAgLy8gaWYgbm90IGRlZmluZWQsIHVzZSB0aGUgc2l6ZSBvZiB0aGUgY29udGFpbmVyXG4gICAgaWYoIWlzTnVtYmVyKGNvbHMpIHx8ICFpc051bWJlcihyb3dzKSl7XG4gICAgICBsZXQgZGltID0gdGhpcy5fZ2V0RGltZW5zaW9ucygpO1xuICAgICAgY29scyA9IGRpbS5jb2xzO1xuICAgICAgcm93cyA9IGRpbS5yb3dzO1xuICAgIH1cblxuICAgIHRoaXMuY29scyA9IGNvbHM7XG4gICAgdGhpcy5yb3dzID0gcm93cztcblxuICAgIHRoaXMudGVybS5yZXNpemUodGhpcy5jb2xzLCB0aGlzLnJvd3MpO1xuICB9LFxuXG4gIF9nZXREaW1lbnNpb25zKCl7XG4gICAgbGV0ICRjb250YWluZXIgPSAkKHRoaXMucmVmcy5jb250YWluZXIpO1xuICAgIGxldCBmYWtlUm93ID0gJCgnPGRpdj48c3Bhbj4mbmJzcDs8L3NwYW4+PC9kaXY+Jyk7XG5cbiAgICAkY29udGFpbmVyLmZpbmQoJy50ZXJtaW5hbCcpLmFwcGVuZChmYWtlUm93KTtcbiAgICAvLyBnZXQgZGl2IGhlaWdodFxuICAgIGxldCBmYWtlQ29sSGVpZ2h0ID0gZmFrZVJvd1swXS5nZXRCb3VuZGluZ0NsaWVudFJlY3QoKS5oZWlnaHQ7XG4gICAgLy8gZ2V0IHNwYW4gd2lkdGhcbiAgICBsZXQgZmFrZUNvbFdpZHRoID0gZmFrZVJvdy5jaGlsZHJlbigpLmZpcnN0KClbMF0uZ2V0Qm91bmRpbmdDbGllbnRSZWN0KCkud2lkdGg7XG5cbiAgICBsZXQgd2lkdGggPSAkY29udGFpbmVyWzBdLmNsaWVudFdpZHRoO1xuICAgIGxldCBoZWlnaHQgPSAkY29udGFpbmVyWzBdLmNsaWVudEhlaWdodDtcblxuICAgIGxldCBjb2xzID0gTWF0aC5mbG9vcih3aWR0aCAvIChmYWtlQ29sV2lkdGgpKTtcbiAgICBsZXQgcm93cyA9IE1hdGguZmxvb3IoaGVpZ2h0IC8gKGZha2VDb2xIZWlnaHQpKTtcbiAgICBmYWtlUm93LnJlbW92ZSgpO1xuXG4gICAgcmV0dXJuIHtjb2xzLCByb3dzfTtcbiAgfVxuXG59KTtcblxuVHR5VGVybWluYWwucHJvcFR5cGVzID0ge1xuICB0dHk6IFJlYWN0LlByb3BUeXBlcy5vYmplY3QuaXNSZXF1aXJlZFxufVxuXG5tb2R1bGUuZXhwb3J0cyA9IFR0eVRlcm1pbmFsO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvdGVybWluYWwuanN4XG4gKiovIiwiLypcbiAqICBUaGUgTUlUIExpY2Vuc2UgKE1JVClcbiAqICBDb3B5cmlnaHQgKGMpIDIwMTUgUnlhbiBGbG9yZW5jZSwgTWljaGFlbCBKYWNrc29uXG4gKiAgUGVybWlzc2lvbiBpcyBoZXJlYnkgZ3JhbnRlZCwgZnJlZSBvZiBjaGFyZ2UsIHRvIGFueSBwZXJzb24gb2J0YWluaW5nIGEgY29weSBvZiB0aGlzIHNvZnR3YXJlIGFuZCBhc3NvY2lhdGVkIGRvY3VtZW50YXRpb24gZmlsZXMgKHRoZSBcIlNvZnR3YXJlXCIpLCB0byBkZWFsIGluIHRoZSBTb2Z0d2FyZSB3aXRob3V0IHJlc3RyaWN0aW9uLCBpbmNsdWRpbmcgd2l0aG91dCBsaW1pdGF0aW9uIHRoZSByaWdodHMgdG8gdXNlLCBjb3B5LCBtb2RpZnksIG1lcmdlLCBwdWJsaXNoLCBkaXN0cmlidXRlLCBzdWJsaWNlbnNlLCBhbmQvb3Igc2VsbCBjb3BpZXMgb2YgdGhlIFNvZnR3YXJlLCBhbmQgdG8gcGVybWl0IHBlcnNvbnMgdG8gd2hvbSB0aGUgU29mdHdhcmUgaXMgZnVybmlzaGVkIHRvIGRvIHNvLCBzdWJqZWN0IHRvIHRoZSBmb2xsb3dpbmcgY29uZGl0aW9uczpcbiAqICBUaGUgYWJvdmUgY29weXJpZ2h0IG5vdGljZSBhbmQgdGhpcyBwZXJtaXNzaW9uIG5vdGljZSBzaGFsbCBiZSBpbmNsdWRlZCBpbiBhbGwgY29waWVzIG9yIHN1YnN0YW50aWFsIHBvcnRpb25zIG9mIHRoZSBTb2Z0d2FyZS5cbiAqICBUSEUgU09GVFdBUkUgSVMgUFJPVklERUQgXCJBUyBJU1wiLCBXSVRIT1VUIFdBUlJBTlRZIE9GIEFOWSBLSU5ELCBFWFBSRVNTIE9SIElNUExJRUQsIElOQ0xVRElORyBCVVQgTk9UIExJTUlURUQgVE8gVEhFIFdBUlJBTlRJRVMgT0YgTUVSQ0hBTlRBQklMSVRZLCBGSVRORVNTIEZPUiBBIFBBUlRJQ1VMQVIgUFVSUE9TRSBBTkQgTk9OSU5GUklOR0VNRU5ULiBJTiBOTyBFVkVOVCBTSEFMTCBUSEUgQVVUSE9SUyBPUiBDT1BZUklHSFQgSE9MREVSUyBCRSBMSUFCTEUgRk9SIEFOWSBDTEFJTSwgREFNQUdFUyBPUiBPVEhFUiBMSUFCSUxJVFksIFdIRVRIRVIgSU4gQU4gQUNUSU9OIE9GIENPTlRSQUNULCBUT1JUIE9SIE9USEVSV0lTRSwgQVJJU0lORyBGUk9NLCBPVVQgT0YgT1IgSU4gQ09OTkVDVElPTiBXSVRIIFRIRSBTT0ZUV0FSRSBPUiBUSEUgVVNFIE9SIE9USEVSIERFQUxJTkdTIElOIFRIRSBTT0ZUV0FSRS5cbiovXG5cbmltcG9ydCBpbnZhcmlhbnQgZnJvbSAnaW52YXJpYW50J1xuXG5mdW5jdGlvbiBlc2NhcGVSZWdFeHAoc3RyaW5nKSB7XG4gIHJldHVybiBzdHJpbmcucmVwbGFjZSgvWy4qKz9eJHt9KCl8W1xcXVxcXFxdL2csICdcXFxcJCYnKVxufVxuXG5mdW5jdGlvbiBlc2NhcGVTb3VyY2Uoc3RyaW5nKSB7XG4gIHJldHVybiBlc2NhcGVSZWdFeHAoc3RyaW5nKS5yZXBsYWNlKC9cXC8rL2csICcvKycpXG59XG5cbmZ1bmN0aW9uIF9jb21waWxlUGF0dGVybihwYXR0ZXJuKSB7XG4gIGxldCByZWdleHBTb3VyY2UgPSAnJztcbiAgY29uc3QgcGFyYW1OYW1lcyA9IFtdO1xuICBjb25zdCB0b2tlbnMgPSBbXTtcblxuICBsZXQgbWF0Y2gsIGxhc3RJbmRleCA9IDAsIG1hdGNoZXIgPSAvOihbYS16QS1aXyRdW2EtekEtWjAtOV8kXSopfFxcKlxcKnxcXCp8XFwofFxcKS9nXG4gIC8qZXNsaW50IG5vLWNvbmQtYXNzaWduOiAwKi9cbiAgd2hpbGUgKChtYXRjaCA9IG1hdGNoZXIuZXhlYyhwYXR0ZXJuKSkpIHtcbiAgICBpZiAobWF0Y2guaW5kZXggIT09IGxhc3RJbmRleCkge1xuICAgICAgdG9rZW5zLnB1c2gocGF0dGVybi5zbGljZShsYXN0SW5kZXgsIG1hdGNoLmluZGV4KSlcbiAgICAgIHJlZ2V4cFNvdXJjZSArPSBlc2NhcGVTb3VyY2UocGF0dGVybi5zbGljZShsYXN0SW5kZXgsIG1hdGNoLmluZGV4KSlcbiAgICB9XG5cbiAgICBpZiAobWF0Y2hbMV0pIHtcbiAgICAgIHJlZ2V4cFNvdXJjZSArPSAnKFteLz8jXSspJztcbiAgICAgIHBhcmFtTmFtZXMucHVzaChtYXRjaFsxXSk7XG4gICAgfSBlbHNlIGlmIChtYXRjaFswXSA9PT0gJyoqJykge1xuICAgICAgcmVnZXhwU291cmNlICs9ICcoW1xcXFxzXFxcXFNdKiknXG4gICAgICBwYXJhbU5hbWVzLnB1c2goJ3NwbGF0Jyk7XG4gICAgfSBlbHNlIGlmIChtYXRjaFswXSA9PT0gJyonKSB7XG4gICAgICByZWdleHBTb3VyY2UgKz0gJyhbXFxcXHNcXFxcU10qPyknXG4gICAgICBwYXJhbU5hbWVzLnB1c2goJ3NwbGF0Jyk7XG4gICAgfSBlbHNlIGlmIChtYXRjaFswXSA9PT0gJygnKSB7XG4gICAgICByZWdleHBTb3VyY2UgKz0gJyg/Oic7XG4gICAgfSBlbHNlIGlmIChtYXRjaFswXSA9PT0gJyknKSB7XG4gICAgICByZWdleHBTb3VyY2UgKz0gJyk/JztcbiAgICB9XG5cbiAgICB0b2tlbnMucHVzaChtYXRjaFswXSk7XG5cbiAgICBsYXN0SW5kZXggPSBtYXRjaGVyLmxhc3RJbmRleDtcbiAgfVxuXG4gIGlmIChsYXN0SW5kZXggIT09IHBhdHRlcm4ubGVuZ3RoKSB7XG4gICAgdG9rZW5zLnB1c2gocGF0dGVybi5zbGljZShsYXN0SW5kZXgsIHBhdHRlcm4ubGVuZ3RoKSlcbiAgICByZWdleHBTb3VyY2UgKz0gZXNjYXBlU291cmNlKHBhdHRlcm4uc2xpY2UobGFzdEluZGV4LCBwYXR0ZXJuLmxlbmd0aCkpXG4gIH1cblxuICByZXR1cm4ge1xuICAgIHBhdHRlcm4sXG4gICAgcmVnZXhwU291cmNlLFxuICAgIHBhcmFtTmFtZXMsXG4gICAgdG9rZW5zXG4gIH1cbn1cblxuY29uc3QgQ29tcGlsZWRQYXR0ZXJuc0NhY2hlID0ge31cblxuZXhwb3J0IGZ1bmN0aW9uIGNvbXBpbGVQYXR0ZXJuKHBhdHRlcm4pIHtcbiAgaWYgKCEocGF0dGVybiBpbiBDb21waWxlZFBhdHRlcm5zQ2FjaGUpKVxuICAgIENvbXBpbGVkUGF0dGVybnNDYWNoZVtwYXR0ZXJuXSA9IF9jb21waWxlUGF0dGVybihwYXR0ZXJuKVxuXG4gIHJldHVybiBDb21waWxlZFBhdHRlcm5zQ2FjaGVbcGF0dGVybl1cbn1cblxuLyoqXG4gKiBBdHRlbXB0cyB0byBtYXRjaCBhIHBhdHRlcm4gb24gdGhlIGdpdmVuIHBhdGhuYW1lLiBQYXR0ZXJucyBtYXkgdXNlXG4gKiB0aGUgZm9sbG93aW5nIHNwZWNpYWwgY2hhcmFjdGVyczpcbiAqXG4gKiAtIDpwYXJhbU5hbWUgICAgIE1hdGNoZXMgYSBVUkwgc2VnbWVudCB1cCB0byB0aGUgbmV4dCAvLCA/LCBvciAjLiBUaGVcbiAqICAgICAgICAgICAgICAgICAgY2FwdHVyZWQgc3RyaW5nIGlzIGNvbnNpZGVyZWQgYSBcInBhcmFtXCJcbiAqIC0gKCkgICAgICAgICAgICAgV3JhcHMgYSBzZWdtZW50IG9mIHRoZSBVUkwgdGhhdCBpcyBvcHRpb25hbFxuICogLSAqICAgICAgICAgICAgICBDb25zdW1lcyAobm9uLWdyZWVkeSkgYWxsIGNoYXJhY3RlcnMgdXAgdG8gdGhlIG5leHRcbiAqICAgICAgICAgICAgICAgICAgY2hhcmFjdGVyIGluIHRoZSBwYXR0ZXJuLCBvciB0byB0aGUgZW5kIG9mIHRoZSBVUkwgaWZcbiAqICAgICAgICAgICAgICAgICAgdGhlcmUgaXMgbm9uZVxuICogLSAqKiAgICAgICAgICAgICBDb25zdW1lcyAoZ3JlZWR5KSBhbGwgY2hhcmFjdGVycyB1cCB0byB0aGUgbmV4dCBjaGFyYWN0ZXJcbiAqICAgICAgICAgICAgICAgICAgaW4gdGhlIHBhdHRlcm4sIG9yIHRvIHRoZSBlbmQgb2YgdGhlIFVSTCBpZiB0aGVyZSBpcyBub25lXG4gKlxuICogVGhlIHJldHVybiB2YWx1ZSBpcyBhbiBvYmplY3Qgd2l0aCB0aGUgZm9sbG93aW5nIHByb3BlcnRpZXM6XG4gKlxuICogLSByZW1haW5pbmdQYXRobmFtZVxuICogLSBwYXJhbU5hbWVzXG4gKiAtIHBhcmFtVmFsdWVzXG4gKi9cbmV4cG9ydCBmdW5jdGlvbiBtYXRjaFBhdHRlcm4ocGF0dGVybiwgcGF0aG5hbWUpIHtcbiAgLy8gTWFrZSBsZWFkaW5nIHNsYXNoZXMgY29uc2lzdGVudCBiZXR3ZWVuIHBhdHRlcm4gYW5kIHBhdGhuYW1lLlxuICBpZiAocGF0dGVybi5jaGFyQXQoMCkgIT09ICcvJykge1xuICAgIHBhdHRlcm4gPSBgLyR7cGF0dGVybn1gXG4gIH1cbiAgaWYgKHBhdGhuYW1lLmNoYXJBdCgwKSAhPT0gJy8nKSB7XG4gICAgcGF0aG5hbWUgPSBgLyR7cGF0aG5hbWV9YFxuICB9XG5cbiAgbGV0IHsgcmVnZXhwU291cmNlLCBwYXJhbU5hbWVzLCB0b2tlbnMgfSA9IGNvbXBpbGVQYXR0ZXJuKHBhdHRlcm4pXG5cbiAgcmVnZXhwU291cmNlICs9ICcvKicgLy8gQ2FwdHVyZSBwYXRoIHNlcGFyYXRvcnNcblxuICAvLyBTcGVjaWFsLWNhc2UgcGF0dGVybnMgbGlrZSAnKicgZm9yIGNhdGNoLWFsbCByb3V0ZXMuXG4gIGNvbnN0IGNhcHR1cmVSZW1haW5pbmcgPSB0b2tlbnNbdG9rZW5zLmxlbmd0aCAtIDFdICE9PSAnKidcblxuICBpZiAoY2FwdHVyZVJlbWFpbmluZykge1xuICAgIC8vIFRoaXMgd2lsbCBtYXRjaCBuZXdsaW5lcyBpbiB0aGUgcmVtYWluaW5nIHBhdGguXG4gICAgcmVnZXhwU291cmNlICs9ICcoW1xcXFxzXFxcXFNdKj8pJ1xuICB9XG5cbiAgY29uc3QgbWF0Y2ggPSBwYXRobmFtZS5tYXRjaChuZXcgUmVnRXhwKCdeJyArIHJlZ2V4cFNvdXJjZSArICckJywgJ2knKSlcblxuICBsZXQgcmVtYWluaW5nUGF0aG5hbWUsIHBhcmFtVmFsdWVzXG4gIGlmIChtYXRjaCAhPSBudWxsKSB7XG4gICAgaWYgKGNhcHR1cmVSZW1haW5pbmcpIHtcbiAgICAgIHJlbWFpbmluZ1BhdGhuYW1lID0gbWF0Y2gucG9wKClcbiAgICAgIGNvbnN0IG1hdGNoZWRQYXRoID1cbiAgICAgICAgbWF0Y2hbMF0uc3Vic3RyKDAsIG1hdGNoWzBdLmxlbmd0aCAtIHJlbWFpbmluZ1BhdGhuYW1lLmxlbmd0aClcblxuICAgICAgLy8gSWYgd2UgZGlkbid0IG1hdGNoIHRoZSBlbnRpcmUgcGF0aG5hbWUsIHRoZW4gbWFrZSBzdXJlIHRoYXQgdGhlIG1hdGNoXG4gICAgICAvLyB3ZSBkaWQgZ2V0IGVuZHMgYXQgYSBwYXRoIHNlcGFyYXRvciAocG90ZW50aWFsbHkgdGhlIG9uZSB3ZSBhZGRlZFxuICAgICAgLy8gYWJvdmUgYXQgdGhlIGJlZ2lubmluZyBvZiB0aGUgcGF0aCwgaWYgdGhlIGFjdHVhbCBtYXRjaCB3YXMgZW1wdHkpLlxuICAgICAgaWYgKFxuICAgICAgICByZW1haW5pbmdQYXRobmFtZSAmJlxuICAgICAgICBtYXRjaGVkUGF0aC5jaGFyQXQobWF0Y2hlZFBhdGgubGVuZ3RoIC0gMSkgIT09ICcvJ1xuICAgICAgKSB7XG4gICAgICAgIHJldHVybiB7XG4gICAgICAgICAgcmVtYWluaW5nUGF0aG5hbWU6IG51bGwsXG4gICAgICAgICAgcGFyYW1OYW1lcyxcbiAgICAgICAgICBwYXJhbVZhbHVlczogbnVsbFxuICAgICAgICB9XG4gICAgICB9XG4gICAgfSBlbHNlIHtcbiAgICAgIC8vIElmIHRoaXMgbWF0Y2hlZCBhdCBhbGwsIHRoZW4gdGhlIG1hdGNoIHdhcyB0aGUgZW50aXJlIHBhdGhuYW1lLlxuICAgICAgcmVtYWluaW5nUGF0aG5hbWUgPSAnJ1xuICAgIH1cblxuICAgIHBhcmFtVmFsdWVzID0gbWF0Y2guc2xpY2UoMSkubWFwKFxuICAgICAgdiA9PiB2ICE9IG51bGwgPyBkZWNvZGVVUklDb21wb25lbnQodikgOiB2XG4gICAgKVxuICB9IGVsc2Uge1xuICAgIHJlbWFpbmluZ1BhdGhuYW1lID0gcGFyYW1WYWx1ZXMgPSBudWxsXG4gIH1cblxuICByZXR1cm4ge1xuICAgIHJlbWFpbmluZ1BhdGhuYW1lLFxuICAgIHBhcmFtTmFtZXMsXG4gICAgcGFyYW1WYWx1ZXNcbiAgfVxufVxuXG5leHBvcnQgZnVuY3Rpb24gZ2V0UGFyYW1OYW1lcyhwYXR0ZXJuKSB7XG4gIHJldHVybiBjb21waWxlUGF0dGVybihwYXR0ZXJuKS5wYXJhbU5hbWVzXG59XG5cbmV4cG9ydCBmdW5jdGlvbiBnZXRQYXJhbXMocGF0dGVybiwgcGF0aG5hbWUpIHtcbiAgY29uc3QgeyBwYXJhbU5hbWVzLCBwYXJhbVZhbHVlcyB9ID0gbWF0Y2hQYXR0ZXJuKHBhdHRlcm4sIHBhdGhuYW1lKVxuXG4gIGlmIChwYXJhbVZhbHVlcyAhPSBudWxsKSB7XG4gICAgcmV0dXJuIHBhcmFtTmFtZXMucmVkdWNlKGZ1bmN0aW9uIChtZW1vLCBwYXJhbU5hbWUsIGluZGV4KSB7XG4gICAgICBtZW1vW3BhcmFtTmFtZV0gPSBwYXJhbVZhbHVlc1tpbmRleF1cbiAgICAgIHJldHVybiBtZW1vXG4gICAgfSwge30pXG4gIH1cblxuICByZXR1cm4gbnVsbFxufVxuXG4vKipcbiAqIFJldHVybnMgYSB2ZXJzaW9uIG9mIHRoZSBnaXZlbiBwYXR0ZXJuIHdpdGggcGFyYW1zIGludGVycG9sYXRlZC4gVGhyb3dzXG4gKiBpZiB0aGVyZSBpcyBhIGR5bmFtaWMgc2VnbWVudCBvZiB0aGUgcGF0dGVybiBmb3Igd2hpY2ggdGhlcmUgaXMgbm8gcGFyYW0uXG4gKi9cbmV4cG9ydCBmdW5jdGlvbiBmb3JtYXRQYXR0ZXJuKHBhdHRlcm4sIHBhcmFtcykge1xuICBwYXJhbXMgPSBwYXJhbXMgfHwge31cblxuICBjb25zdCB7IHRva2VucyB9ID0gY29tcGlsZVBhdHRlcm4ocGF0dGVybilcbiAgbGV0IHBhcmVuQ291bnQgPSAwLCBwYXRobmFtZSA9ICcnLCBzcGxhdEluZGV4ID0gMFxuXG4gIGxldCB0b2tlbiwgcGFyYW1OYW1lLCBwYXJhbVZhbHVlXG4gIGZvciAobGV0IGkgPSAwLCBsZW4gPSB0b2tlbnMubGVuZ3RoOyBpIDwgbGVuOyArK2kpIHtcbiAgICB0b2tlbiA9IHRva2Vuc1tpXVxuXG4gICAgaWYgKHRva2VuID09PSAnKicgfHwgdG9rZW4gPT09ICcqKicpIHtcbiAgICAgIHBhcmFtVmFsdWUgPSBBcnJheS5pc0FycmF5KHBhcmFtcy5zcGxhdCkgPyBwYXJhbXMuc3BsYXRbc3BsYXRJbmRleCsrXSA6IHBhcmFtcy5zcGxhdFxuXG4gICAgICBpbnZhcmlhbnQoXG4gICAgICAgIHBhcmFtVmFsdWUgIT0gbnVsbCB8fCBwYXJlbkNvdW50ID4gMCxcbiAgICAgICAgJ01pc3Npbmcgc3BsYXQgIyVzIGZvciBwYXRoIFwiJXNcIicsXG4gICAgICAgIHNwbGF0SW5kZXgsIHBhdHRlcm5cbiAgICAgIClcblxuICAgICAgaWYgKHBhcmFtVmFsdWUgIT0gbnVsbClcbiAgICAgICAgcGF0aG5hbWUgKz0gZW5jb2RlVVJJKHBhcmFtVmFsdWUpXG4gICAgfSBlbHNlIGlmICh0b2tlbiA9PT0gJygnKSB7XG4gICAgICBwYXJlbkNvdW50ICs9IDFcbiAgICB9IGVsc2UgaWYgKHRva2VuID09PSAnKScpIHtcbiAgICAgIHBhcmVuQ291bnQgLT0gMVxuICAgIH0gZWxzZSBpZiAodG9rZW4uY2hhckF0KDApID09PSAnOicpIHtcbiAgICAgIHBhcmFtTmFtZSA9IHRva2VuLnN1YnN0cmluZygxKVxuICAgICAgcGFyYW1WYWx1ZSA9IHBhcmFtc1twYXJhbU5hbWVdXG5cbiAgICAgIGludmFyaWFudChcbiAgICAgICAgcGFyYW1WYWx1ZSAhPSBudWxsIHx8IHBhcmVuQ291bnQgPiAwLFxuICAgICAgICAnTWlzc2luZyBcIiVzXCIgcGFyYW1ldGVyIGZvciBwYXRoIFwiJXNcIicsXG4gICAgICAgIHBhcmFtTmFtZSwgcGF0dGVyblxuICAgICAgKVxuXG4gICAgICBpZiAocGFyYW1WYWx1ZSAhPSBudWxsKVxuICAgICAgICBwYXRobmFtZSArPSBlbmNvZGVVUklDb21wb25lbnQocGFyYW1WYWx1ZSlcbiAgICB9IGVsc2Uge1xuICAgICAgcGF0aG5hbWUgKz0gdG9rZW5cbiAgICB9XG4gIH1cblxuICByZXR1cm4gcGF0aG5hbWUucmVwbGFjZSgvXFwvKy9nLCAnLycpXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tbW9uL3BhdHRlcm5VdGlscy5qc1xuICoqLyIsInZhciBUdHkgPSByZXF1aXJlKCdhcHAvY29tbW9uL3R0eScpO1xudmFyIGFwaSA9IHJlcXVpcmUoJ2FwcC9zZXJ2aWNlcy9hcGknKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG5cbmNsYXNzIFR0eVBsYXllciBleHRlbmRzIFR0eSB7XG4gIGNvbnN0cnVjdG9yKHtzaWR9KXtcbiAgICBzdXBlcih7fSk7XG4gICAgdGhpcy5zaWQgPSBzaWQ7XG4gICAgdGhpcy5jdXJyZW50ID0gMTtcbiAgICB0aGlzLmxlbmd0aCA9IC0xO1xuICAgIHRoaXMudHR5U3RlYW0gPSBuZXcgQXJyYXkoKTtcbiAgICB0aGlzLmlzTG9haW5kID0gZmFsc2U7XG4gICAgdGhpcy5pc1BsYXlpbmcgPSBmYWxzZTtcbiAgICB0aGlzLmlzRXJyb3IgPSBmYWxzZTtcbiAgICB0aGlzLmlzUmVhZHkgPSBmYWxzZTtcbiAgICB0aGlzLmlzTG9hZGluZyA9IHRydWU7XG4gIH1cblxuICBzZW5kKCl7XG4gIH1cblxuICByZXNpemUoKXtcbiAgfVxuXG4gIGNvbm5lY3QoKXtcbiAgICBhcGkuZ2V0KGNmZy5hcGkuZ2V0RmV0Y2hTZXNzaW9uTGVuZ3RoVXJsKHRoaXMuc2lkKSlcbiAgICAgIC5kb25lKChkYXRhKT0+e1xuICAgICAgICB0aGlzLmxlbmd0aCA9IGRhdGEuY291bnQ7XG4gICAgICAgIHRoaXMuaXNSZWFkeSA9IHRydWU7XG4gICAgICB9KVxuICAgICAgLmZhaWwoKCk9PntcbiAgICAgICAgdGhpcy5pc0Vycm9yID0gdHJ1ZTtcbiAgICAgIH0pXG4gICAgICAuYWx3YXlzKCgpPT57XG4gICAgICAgIHRoaXMuX2NoYW5nZSgpO1xuICAgICAgfSk7XG4gIH1cblxuICBtb3ZlKG5ld1Bvcyl7XG4gICAgaWYoIXRoaXMuaXNSZWFkeSl7XG4gICAgICByZXR1cm47XG4gICAgfVxuXG4gICAgaWYobmV3UG9zID09PSB1bmRlZmluZWQpe1xuICAgICAgbmV3UG9zID0gdGhpcy5jdXJyZW50ICsgMTtcbiAgICB9XG5cbiAgICBpZihuZXdQb3MgPiB0aGlzLmxlbmd0aCl7XG4gICAgICBuZXdQb3MgPSB0aGlzLmxlbmd0aDtcbiAgICAgIHRoaXMuc3RvcCgpO1xuICAgIH1cblxuICAgIGlmKG5ld1BvcyA9PT0gMCl7XG4gICAgICBuZXdQb3MgPSAxO1xuICAgIH1cblxuICAgIGlmKHRoaXMuaXNQbGF5aW5nKXtcbiAgICAgIGlmKHRoaXMuY3VycmVudCA8IG5ld1Bvcyl7XG4gICAgICAgIHRoaXMuX3Nob3dDaHVuayh0aGlzLmN1cnJlbnQsIG5ld1Bvcyk7XG4gICAgICB9ZWxzZXtcbiAgICAgICAgdGhpcy5lbWl0KCdyZXNldCcpO1xuICAgICAgICB0aGlzLl9zaG93Q2h1bmsodGhpcy5jdXJyZW50LCBuZXdQb3MpO1xuICAgICAgfVxuICAgIH1lbHNle1xuICAgICAgdGhpcy5jdXJyZW50ID0gbmV3UG9zO1xuICAgIH1cblxuICAgIHRoaXMuX2NoYW5nZSgpO1xuICB9XG5cbiAgc3RvcCgpe1xuICAgIHRoaXMuaXNQbGF5aW5nID0gZmFsc2U7XG4gICAgdGhpcy50aW1lciA9IGNsZWFySW50ZXJ2YWwodGhpcy50aW1lcik7XG4gICAgdGhpcy5fY2hhbmdlKCk7XG4gIH1cblxuICBwbGF5KCl7XG4gICAgaWYodGhpcy5pc1BsYXlpbmcpe1xuICAgICAgcmV0dXJuO1xuICAgIH1cblxuICAgIHRoaXMuaXNQbGF5aW5nID0gdHJ1ZTtcblxuICAgIC8vIHN0YXJ0IGZyb20gdGhlIGJlZ2lubmluZyBpZiBhdCB0aGUgZW5kXG4gICAgaWYodGhpcy5jdXJyZW50ID09PSB0aGlzLmxlbmd0aCl7XG4gICAgICB0aGlzLmN1cnJlbnQgPSAxO1xuICAgICAgdGhpcy5lbWl0KCdyZXNldCcpO1xuICAgIH1cblxuICAgIHRoaXMudGltZXIgPSBzZXRJbnRlcnZhbCh0aGlzLm1vdmUuYmluZCh0aGlzKSwgMTUwKTtcbiAgICB0aGlzLl9jaGFuZ2UoKTtcbiAgfVxuXG4gIF9zaG91bGRGZXRjaChzdGFydCwgZW5kKXtcbiAgICBmb3IodmFyIGkgPSBzdGFydDsgaSA8IGVuZDsgaSsrKXtcbiAgICAgIGlmKHRoaXMudHR5U3RlYW1baV0gPT09IHVuZGVmaW5lZCl7XG4gICAgICAgIHJldHVybiB0cnVlO1xuICAgICAgfVxuICAgIH1cblxuICAgIHJldHVybiBmYWxzZTtcbiAgfVxuXG4gIF9mZXRjaChzdGFydCwgZW5kKXtcbiAgICBlbmQgPSBlbmQgKyA1MDtcbiAgICBlbmQgPSBlbmQgPiB0aGlzLmxlbmd0aCA/IHRoaXMubGVuZ3RoIDogZW5kO1xuICAgIHJldHVybiBhcGkuZ2V0KGNmZy5hcGkuZ2V0RmV0Y2hTZXNzaW9uQ2h1bmtVcmwoe3NpZDogdGhpcy5zaWQsIHN0YXJ0LCBlbmR9KSkuXG4gICAgICBkb25lKChyZXNwb25zZSk9PntcbiAgICAgICAgZm9yKHZhciBpID0gMDsgaSA8IGVuZC1zdGFydDsgaSsrKXtcbiAgICAgICAgICB2YXIgZGF0YSA9IGF0b2IocmVzcG9uc2UuY2h1bmtzW2ldLmRhdGEpIHx8ICcnO1xuICAgICAgICAgIHZhciBkZWxheSA9IHJlc3BvbnNlLmNodW5rc1tpXS5kZWxheTtcbiAgICAgICAgICB0aGlzLnR0eVN0ZWFtW3N0YXJ0K2ldID0geyBkYXRhLCBkZWxheX07XG4gICAgICAgIH1cbiAgICAgIH0pO1xuICB9XG5cbiAgX3Nob3dDaHVuayhzdGFydCwgZW5kKXtcbiAgICB2YXIgZGlzcGxheSA9ICgpPT57XG4gICAgICBmb3IodmFyIGkgPSBzdGFydDsgaSA8IGVuZDsgaSsrKXtcbiAgICAgICAgdGhpcy5lbWl0KCdkYXRhJywgdGhpcy50dHlTdGVhbVtpXS5kYXRhKTtcbiAgICAgIH1cbiAgICAgIHRoaXMuY3VycmVudCA9IGVuZDtcbiAgICB9O1xuXG4gICAgaWYodGhpcy5fc2hvdWxkRmV0Y2goc3RhcnQsIGVuZCkpe1xuICAgICAgdGhpcy5fZmV0Y2goc3RhcnQsIGVuZCkudGhlbihkaXNwbGF5KTtcbiAgICB9ZWxzZXtcbiAgICAgIGRpc3BsYXkoKTtcbiAgICB9XG4gIH1cblxuICBfY2hhbmdlKCl7XG4gICAgdGhpcy5lbWl0KCdjaGFuZ2UnKTtcbiAgfVxufVxuXG5leHBvcnQgZGVmYXVsdCBUdHlQbGF5ZXI7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tbW9uL3R0eVBsYXllci5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7ZmV0Y2hTZXNzaW9uc30gPSByZXF1aXJlKCcuLy4uL3Nlc3Npb25zL2FjdGlvbnMnKTtcbnZhciB7ZmV0Y2hOb2Rlc30gPSByZXF1aXJlKCcuLy4uL25vZGVzL2FjdGlvbnMnKTtcbnZhciB7bW9udGhSYW5nZX0gPSByZXF1aXJlKCdhcHAvY29tbW9uL2RhdGVVdGlscycpO1xuXG52YXIgJCA9IHJlcXVpcmUoJ2pRdWVyeScpO1xuXG52YXIgeyBUTFBUX0FQUF9JTklULCBUTFBUX0FQUF9GQUlMRUQsIFRMUFRfQVBQX1JFQURZIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbnZhciBhY3Rpb25zID0ge1xuXG4gIGluaXRBcHAoKSB7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX0FQUF9JTklUKTtcbiAgICBhY3Rpb25zLmZldGNoTm9kZXNBbmRTZXNzaW9ucygpXG4gICAgICAuZG9uZSgoKT0+eyByZWFjdG9yLmRpc3BhdGNoKFRMUFRfQVBQX1JFQURZKTsgfSlcbiAgICAgIC5mYWlsKCgpPT57IHJlYWN0b3IuZGlzcGF0Y2goVExQVF9BUFBfRkFJTEVEKTsgfSk7XG4gIH0sXG5cbiAgZmV0Y2hOb2Rlc0FuZFNlc3Npb25zKCkge1xuICAgIHZhciBbc3RhcnQsIGVuZCBdID0gbW9udGhSYW5nZSgpO1xuICAgIHJldHVybiAkLndoZW4oZmV0Y2hOb2RlcygpLCBmZXRjaFNlc3Npb25zKHN0YXJ0LCBlbmQpKTtcbiAgfVxufVxuXG5leHBvcnQgZGVmYXVsdCBhY3Rpb25zO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvYXBwL2FjdGlvbnMuanNcbiAqKi8iLCJjb25zdCBhcHBTdGF0ZSA9IFtbJ3RscHQnXSwgYXBwPT4gYXBwLnRvSlMoKV07XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgYXBwU3RhdGVcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2FwcC9nZXR0ZXJzLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMuZ2V0dGVycyA9IHJlcXVpcmUoJy4vZ2V0dGVycycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aW9ucyA9IHJlcXVpcmUoJy4vYWN0aW9ucycpO1xubW9kdWxlLmV4cG9ydHMuYXBwU3RvcmUgPSByZXF1aXJlKCcuL2FwcFN0b3JlJyk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hcHAvaW5kZXguanNcbiAqKi8iLCJjb25zdCBkaWFsb2dzID0gW1sndGxwdF9kaWFsb2dzJ10sIHN0YXRlPT4gc3RhdGUudG9KUygpXTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICBkaWFsb2dzXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9kaWFsb2dzL2dldHRlcnMuanNcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cy5nZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3Rpb25zID0gcmVxdWlyZSgnLi9hY3Rpb25zJyk7XG5tb2R1bGUuZXhwb3J0cy5kaWFsb2dTdG9yZSA9IHJlcXVpcmUoJy4vZGlhbG9nU3RvcmUnKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvaW5kZXguanNcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG5yZWFjdG9yLnJlZ2lzdGVyU3RvcmVzKHtcbiAgJ3RscHQnOiByZXF1aXJlKCcuL2FwcC9hcHBTdG9yZScpLFxuICAndGxwdF9kaWFsb2dzJzogcmVxdWlyZSgnLi9kaWFsb2dzL2RpYWxvZ1N0b3JlJyksXG4gICd0bHB0X2FjdGl2ZV90ZXJtaW5hbCc6IHJlcXVpcmUoJy4vYWN0aXZlVGVybWluYWwvYWN0aXZlVGVybVN0b3JlJyksXG4gICd0bHB0X3VzZXInOiByZXF1aXJlKCcuL3VzZXIvdXNlclN0b3JlJyksXG4gICd0bHB0X25vZGVzJzogcmVxdWlyZSgnLi9ub2Rlcy9ub2RlU3RvcmUnKSxcbiAgJ3RscHRfaW52aXRlJzogcmVxdWlyZSgnLi9pbnZpdGUvaW52aXRlU3RvcmUnKSxcbiAgJ3RscHRfcmVzdF9hcGknOiByZXF1aXJlKCcuL3Jlc3RBcGkvcmVzdEFwaVN0b3JlJyksXG4gICd0bHB0X3Nlc3Npb25zJzogcmVxdWlyZSgnLi9zZXNzaW9ucy9zZXNzaW9uU3RvcmUnKVxufSk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9pbmRleC5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7IFRMUFRfUkVDRUlWRV9VU0VSX0lOVklURSB9ICA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcbnZhciB7IEZFVENISU5HX0lOVklURX0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9yZXN0QXBpL2NvbnN0YW50cycpO1xudmFyIHJlc3RBcGlBY3Rpb25zID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvcmVzdEFwaS9hY3Rpb25zJyk7XG52YXIgYXBpID0gcmVxdWlyZSgnYXBwL3NlcnZpY2VzL2FwaScpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICBmZXRjaEludml0ZShpbnZpdGVUb2tlbil7XG4gICAgdmFyIHBhdGggPSBjZmcuYXBpLmdldEludml0ZVVybChpbnZpdGVUb2tlbik7XG4gICAgcmVzdEFwaUFjdGlvbnMuc3RhcnQoRkVUQ0hJTkdfSU5WSVRFKTtcbiAgICBhcGkuZ2V0KHBhdGgpLmRvbmUoaW52aXRlPT57XG4gICAgICByZXN0QXBpQWN0aW9ucy5zdWNjZXNzKEZFVENISU5HX0lOVklURSk7XG4gICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfUkVDRUlWRV9VU0VSX0lOVklURSwgaW52aXRlKTtcbiAgICB9KS5cbiAgICBmYWlsKChlcnIpPT57XG4gICAgICByZXN0QXBpQWN0aW9ucy5mYWlsKEZFVENISU5HX0lOVklURSwgZXJyLnJlc3BvbnNlSlNPTi5tZXNzYWdlKTtcbiAgICB9KTtcbiAgfVxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2FjdGlvbnMuanNcbiAqKi8iLCJ2YXIge1RSWUlOR19UT19TSUdOX1VQLCBGRVRDSElOR19JTlZJVEV9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvcmVzdEFwaS9jb25zdGFudHMnKTtcbnZhciB7cmVxdWVzdFN0YXR1c30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9yZXN0QXBpL2dldHRlcnMnKTtcblxuY29uc3QgaW52aXRlID0gWyBbJ3RscHRfaW52aXRlJ10sIChpbnZpdGUpID0+IGludml0ZSBdO1xuXG5leHBvcnQgZGVmYXVsdCB7XG4gIGludml0ZSxcbiAgYXR0ZW1wOiByZXF1ZXN0U3RhdHVzKFRSWUlOR19UT19TSUdOX1VQKSxcbiAgZmV0Y2hpbmdJbnZpdGU6IHJlcXVlc3RTdGF0dXMoRkVUQ0hJTkdfSU5WSVRFKVxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2dldHRlcnMuanNcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cy5nZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3Rpb25zID0gcmVxdWlyZSgnLi9hY3Rpb25zJyk7XG5tb2R1bGUuZXhwb3J0cy5ub2RlU3RvcmUgPSByZXF1aXJlKCcuL2ludml0ZVN0b3JlJyk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvaW5kZXguanNcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cy5nZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3Rpb25zID0gcmVxdWlyZSgnLi9hY3Rpb25zJyk7XG5tb2R1bGUuZXhwb3J0cy5ub2RlU3RvcmUgPSByZXF1aXJlKCcuL25vZGVTdG9yZScpO1xuXG4vLyBub2RlczogW3tcImlkXCI6XCJ4MjIwXCIsXCJhZGRyXCI6XCIwLjAuMC4wOjMwMjJcIixcImhvc3RuYW1lXCI6XCJ4MjIwXCIsXCJsYWJlbHNcIjpudWxsLFwiY21kX2xhYmVsc1wiOm51bGx9XVxuXG5cbi8vIHNlc3Npb25zOiBbe1wiaWRcIjpcIjA3NjMwNjM2LWJiM2QtNDBlMS1iMDg2LTYwYjJjYWUyMWFjNFwiLFwicGFydGllc1wiOlt7XCJpZFwiOlwiODlmNzYyYTMtNzQyOS00YzdhLWE5MTMtNzY2NDkzZmU3YzhhXCIsXCJzaXRlXCI6XCIxMjcuMC4wLjE6Mzc1MTRcIixcInVzZXJcIjpcImFrb250c2V2b3lcIixcInNlcnZlcl9hZGRyXCI6XCIwLjAuMC4wOjMwMjJcIixcImxhc3RfYWN0aXZlXCI6XCIyMDE2LTAyLTIyVDE0OjM5OjIwLjkzMTIwNTM1LTA1OjAwXCJ9XX1dXG5cbi8qXG5sZXQgVG9kb1JlY29yZCA9IEltbXV0YWJsZS5SZWNvcmQoe1xuICAgIGlkOiAwLFxuICAgIGRlc2NyaXB0aW9uOiBcIlwiLFxuICAgIGNvbXBsZXRlZDogZmFsc2Vcbn0pO1xuKi9cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2luZGV4LmpzXG4gKiovIiwidmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG52YXIge1xuICBUTFBUX1JFU1RfQVBJX1NUQVJULFxuICBUTFBUX1JFU1RfQVBJX1NVQ0NFU1MsXG4gIFRMUFRfUkVTVF9BUElfRkFJTCB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gdG9JbW11dGFibGUoe30pO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX1JFU1RfQVBJX1NUQVJULCBzdGFydCk7XG4gICAgdGhpcy5vbihUTFBUX1JFU1RfQVBJX0ZBSUwsIGZhaWwpO1xuICAgIHRoaXMub24oVExQVF9SRVNUX0FQSV9TVUNDRVNTLCBzdWNjZXNzKTtcbiAgfVxufSlcblxuZnVuY3Rpb24gc3RhcnQoc3RhdGUsIHJlcXVlc3Qpe1xuICByZXR1cm4gc3RhdGUuc2V0KHJlcXVlc3QudHlwZSwgdG9JbW11dGFibGUoe2lzUHJvY2Vzc2luZzogdHJ1ZX0pKTtcbn1cblxuZnVuY3Rpb24gZmFpbChzdGF0ZSwgcmVxdWVzdCl7XG4gIHJldHVybiBzdGF0ZS5zZXQocmVxdWVzdC50eXBlLCB0b0ltbXV0YWJsZSh7aXNGYWlsZWQ6IHRydWUsIG1lc3NhZ2U6IHJlcXVlc3QubWVzc2FnZX0pKTtcbn1cblxuZnVuY3Rpb24gc3VjY2VzcyhzdGF0ZSwgcmVxdWVzdCl7XG4gIHJldHVybiBzdGF0ZS5zZXQocmVxdWVzdC50eXBlLCB0b0ltbXV0YWJsZSh7aXNTdWNjZXNzOiB0cnVlfSkpO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9yZXN0QXBpU3RvcmUuanNcbiAqKi8iLCJ2YXIgdXRpbHMgPSB7XG5cbiAgdXVpZCgpe1xuICAgIC8vIG5ldmVyIHVzZSBpdCBpbiBwcm9kdWN0aW9uXG4gICAgcmV0dXJuICd4eHh4eHh4eC14eHh4LTR4eHgteXh4eC14eHh4eHh4eHh4eHgnLnJlcGxhY2UoL1t4eV0vZywgZnVuY3Rpb24oYykge1xuICAgICAgdmFyIHIgPSBNYXRoLnJhbmRvbSgpKjE2fDAsIHYgPSBjID09ICd4JyA/IHIgOiAociYweDN8MHg4KTtcbiAgICAgIHJldHVybiB2LnRvU3RyaW5nKDE2KTtcbiAgICB9KTtcbiAgfSxcblxuICBkaXNwbGF5RGF0ZShkYXRlKXtcbiAgICB0cnl7XG4gICAgICByZXR1cm4gZGF0ZS50b0xvY2FsZURhdGVTdHJpbmcoKSArICcgJyArIGRhdGUudG9Mb2NhbGVUaW1lU3RyaW5nKCk7XG4gICAgfWNhdGNoKGVycil7XG4gICAgICBjb25zb2xlLmVycm9yKGVycik7XG4gICAgICByZXR1cm4gJ3VuZGVmaW5lZCc7XG4gICAgfVxuICB9LFxuXG4gIGZvcm1hdFN0cmluZyhmb3JtYXQpIHtcbiAgICB2YXIgYXJncyA9IEFycmF5LnByb3RvdHlwZS5zbGljZS5jYWxsKGFyZ3VtZW50cywgMSk7XG4gICAgcmV0dXJuIGZvcm1hdC5yZXBsYWNlKG5ldyBSZWdFeHAoJ1xcXFx7KFxcXFxkKylcXFxcfScsICdnJyksXG4gICAgICAobWF0Y2gsIG51bWJlcikgPT4ge1xuICAgICAgICByZXR1cm4gIShhcmdzW251bWJlcl0gPT09IG51bGwgfHwgYXJnc1tudW1iZXJdID09PSB1bmRlZmluZWQpID8gYXJnc1tudW1iZXJdIDogJyc7XG4gICAgfSk7XG4gIH1cbiAgICAgICAgICAgIFxufVxuXG5tb2R1bGUuZXhwb3J0cyA9IHV0aWxzO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL3V0aWxzLmpzXG4gKiovIiwiLy8gQ29weXJpZ2h0IEpveWVudCwgSW5jLiBhbmQgb3RoZXIgTm9kZSBjb250cmlidXRvcnMuXG4vL1xuLy8gUGVybWlzc2lvbiBpcyBoZXJlYnkgZ3JhbnRlZCwgZnJlZSBvZiBjaGFyZ2UsIHRvIGFueSBwZXJzb24gb2J0YWluaW5nIGFcbi8vIGNvcHkgb2YgdGhpcyBzb2Z0d2FyZSBhbmQgYXNzb2NpYXRlZCBkb2N1bWVudGF0aW9uIGZpbGVzICh0aGVcbi8vIFwiU29mdHdhcmVcIiksIHRvIGRlYWwgaW4gdGhlIFNvZnR3YXJlIHdpdGhvdXQgcmVzdHJpY3Rpb24sIGluY2x1ZGluZ1xuLy8gd2l0aG91dCBsaW1pdGF0aW9uIHRoZSByaWdodHMgdG8gdXNlLCBjb3B5LCBtb2RpZnksIG1lcmdlLCBwdWJsaXNoLFxuLy8gZGlzdHJpYnV0ZSwgc3VibGljZW5zZSwgYW5kL29yIHNlbGwgY29waWVzIG9mIHRoZSBTb2Z0d2FyZSwgYW5kIHRvIHBlcm1pdFxuLy8gcGVyc29ucyB0byB3aG9tIHRoZSBTb2Z0d2FyZSBpcyBmdXJuaXNoZWQgdG8gZG8gc28sIHN1YmplY3QgdG8gdGhlXG4vLyBmb2xsb3dpbmcgY29uZGl0aW9uczpcbi8vXG4vLyBUaGUgYWJvdmUgY29weXJpZ2h0IG5vdGljZSBhbmQgdGhpcyBwZXJtaXNzaW9uIG5vdGljZSBzaGFsbCBiZSBpbmNsdWRlZFxuLy8gaW4gYWxsIGNvcGllcyBvciBzdWJzdGFudGlhbCBwb3J0aW9ucyBvZiB0aGUgU29mdHdhcmUuXG4vL1xuLy8gVEhFIFNPRlRXQVJFIElTIFBST1ZJREVEIFwiQVMgSVNcIiwgV0lUSE9VVCBXQVJSQU5UWSBPRiBBTlkgS0lORCwgRVhQUkVTU1xuLy8gT1IgSU1QTElFRCwgSU5DTFVESU5HIEJVVCBOT1QgTElNSVRFRCBUTyBUSEUgV0FSUkFOVElFUyBPRlxuLy8gTUVSQ0hBTlRBQklMSVRZLCBGSVRORVNTIEZPUiBBIFBBUlRJQ1VMQVIgUFVSUE9TRSBBTkQgTk9OSU5GUklOR0VNRU5ULiBJTlxuLy8gTk8gRVZFTlQgU0hBTEwgVEhFIEFVVEhPUlMgT1IgQ09QWVJJR0hUIEhPTERFUlMgQkUgTElBQkxFIEZPUiBBTlkgQ0xBSU0sXG4vLyBEQU1BR0VTIE9SIE9USEVSIExJQUJJTElUWSwgV0hFVEhFUiBJTiBBTiBBQ1RJT04gT0YgQ09OVFJBQ1QsIFRPUlQgT1Jcbi8vIE9USEVSV0lTRSwgQVJJU0lORyBGUk9NLCBPVVQgT0YgT1IgSU4gQ09OTkVDVElPTiBXSVRIIFRIRSBTT0ZUV0FSRSBPUiBUSEVcbi8vIFVTRSBPUiBPVEhFUiBERUFMSU5HUyBJTiBUSEUgU09GVFdBUkUuXG5cbmZ1bmN0aW9uIEV2ZW50RW1pdHRlcigpIHtcbiAgdGhpcy5fZXZlbnRzID0gdGhpcy5fZXZlbnRzIHx8IHt9O1xuICB0aGlzLl9tYXhMaXN0ZW5lcnMgPSB0aGlzLl9tYXhMaXN0ZW5lcnMgfHwgdW5kZWZpbmVkO1xufVxubW9kdWxlLmV4cG9ydHMgPSBFdmVudEVtaXR0ZXI7XG5cbi8vIEJhY2t3YXJkcy1jb21wYXQgd2l0aCBub2RlIDAuMTAueFxuRXZlbnRFbWl0dGVyLkV2ZW50RW1pdHRlciA9IEV2ZW50RW1pdHRlcjtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5fZXZlbnRzID0gdW5kZWZpbmVkO1xuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5fbWF4TGlzdGVuZXJzID0gdW5kZWZpbmVkO1xuXG4vLyBCeSBkZWZhdWx0IEV2ZW50RW1pdHRlcnMgd2lsbCBwcmludCBhIHdhcm5pbmcgaWYgbW9yZSB0aGFuIDEwIGxpc3RlbmVycyBhcmVcbi8vIGFkZGVkIHRvIGl0LiBUaGlzIGlzIGEgdXNlZnVsIGRlZmF1bHQgd2hpY2ggaGVscHMgZmluZGluZyBtZW1vcnkgbGVha3MuXG5FdmVudEVtaXR0ZXIuZGVmYXVsdE1heExpc3RlbmVycyA9IDEwO1xuXG4vLyBPYnZpb3VzbHkgbm90IGFsbCBFbWl0dGVycyBzaG91bGQgYmUgbGltaXRlZCB0byAxMC4gVGhpcyBmdW5jdGlvbiBhbGxvd3Ncbi8vIHRoYXQgdG8gYmUgaW5jcmVhc2VkLiBTZXQgdG8gemVybyBmb3IgdW5saW1pdGVkLlxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5zZXRNYXhMaXN0ZW5lcnMgPSBmdW5jdGlvbihuKSB7XG4gIGlmICghaXNOdW1iZXIobikgfHwgbiA8IDAgfHwgaXNOYU4obikpXG4gICAgdGhyb3cgVHlwZUVycm9yKCduIG11c3QgYmUgYSBwb3NpdGl2ZSBudW1iZXInKTtcbiAgdGhpcy5fbWF4TGlzdGVuZXJzID0gbjtcbiAgcmV0dXJuIHRoaXM7XG59O1xuXG5FdmVudEVtaXR0ZXIucHJvdG90eXBlLmVtaXQgPSBmdW5jdGlvbih0eXBlKSB7XG4gIHZhciBlciwgaGFuZGxlciwgbGVuLCBhcmdzLCBpLCBsaXN0ZW5lcnM7XG5cbiAgaWYgKCF0aGlzLl9ldmVudHMpXG4gICAgdGhpcy5fZXZlbnRzID0ge307XG5cbiAgLy8gSWYgdGhlcmUgaXMgbm8gJ2Vycm9yJyBldmVudCBsaXN0ZW5lciB0aGVuIHRocm93LlxuICBpZiAodHlwZSA9PT0gJ2Vycm9yJykge1xuICAgIGlmICghdGhpcy5fZXZlbnRzLmVycm9yIHx8XG4gICAgICAgIChpc09iamVjdCh0aGlzLl9ldmVudHMuZXJyb3IpICYmICF0aGlzLl9ldmVudHMuZXJyb3IubGVuZ3RoKSkge1xuICAgICAgZXIgPSBhcmd1bWVudHNbMV07XG4gICAgICBpZiAoZXIgaW5zdGFuY2VvZiBFcnJvcikge1xuICAgICAgICB0aHJvdyBlcjsgLy8gVW5oYW5kbGVkICdlcnJvcicgZXZlbnRcbiAgICAgIH1cbiAgICAgIHRocm93IFR5cGVFcnJvcignVW5jYXVnaHQsIHVuc3BlY2lmaWVkIFwiZXJyb3JcIiBldmVudC4nKTtcbiAgICB9XG4gIH1cblxuICBoYW5kbGVyID0gdGhpcy5fZXZlbnRzW3R5cGVdO1xuXG4gIGlmIChpc1VuZGVmaW5lZChoYW5kbGVyKSlcbiAgICByZXR1cm4gZmFsc2U7XG5cbiAgaWYgKGlzRnVuY3Rpb24oaGFuZGxlcikpIHtcbiAgICBzd2l0Y2ggKGFyZ3VtZW50cy5sZW5ndGgpIHtcbiAgICAgIC8vIGZhc3QgY2FzZXNcbiAgICAgIGNhc2UgMTpcbiAgICAgICAgaGFuZGxlci5jYWxsKHRoaXMpO1xuICAgICAgICBicmVhaztcbiAgICAgIGNhc2UgMjpcbiAgICAgICAgaGFuZGxlci5jYWxsKHRoaXMsIGFyZ3VtZW50c1sxXSk7XG4gICAgICAgIGJyZWFrO1xuICAgICAgY2FzZSAzOlxuICAgICAgICBoYW5kbGVyLmNhbGwodGhpcywgYXJndW1lbnRzWzFdLCBhcmd1bWVudHNbMl0pO1xuICAgICAgICBicmVhaztcbiAgICAgIC8vIHNsb3dlclxuICAgICAgZGVmYXVsdDpcbiAgICAgICAgbGVuID0gYXJndW1lbnRzLmxlbmd0aDtcbiAgICAgICAgYXJncyA9IG5ldyBBcnJheShsZW4gLSAxKTtcbiAgICAgICAgZm9yIChpID0gMTsgaSA8IGxlbjsgaSsrKVxuICAgICAgICAgIGFyZ3NbaSAtIDFdID0gYXJndW1lbnRzW2ldO1xuICAgICAgICBoYW5kbGVyLmFwcGx5KHRoaXMsIGFyZ3MpO1xuICAgIH1cbiAgfSBlbHNlIGlmIChpc09iamVjdChoYW5kbGVyKSkge1xuICAgIGxlbiA9IGFyZ3VtZW50cy5sZW5ndGg7XG4gICAgYXJncyA9IG5ldyBBcnJheShsZW4gLSAxKTtcbiAgICBmb3IgKGkgPSAxOyBpIDwgbGVuOyBpKyspXG4gICAgICBhcmdzW2kgLSAxXSA9IGFyZ3VtZW50c1tpXTtcblxuICAgIGxpc3RlbmVycyA9IGhhbmRsZXIuc2xpY2UoKTtcbiAgICBsZW4gPSBsaXN0ZW5lcnMubGVuZ3RoO1xuICAgIGZvciAoaSA9IDA7IGkgPCBsZW47IGkrKylcbiAgICAgIGxpc3RlbmVyc1tpXS5hcHBseSh0aGlzLCBhcmdzKTtcbiAgfVxuXG4gIHJldHVybiB0cnVlO1xufTtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5hZGRMaXN0ZW5lciA9IGZ1bmN0aW9uKHR5cGUsIGxpc3RlbmVyKSB7XG4gIHZhciBtO1xuXG4gIGlmICghaXNGdW5jdGlvbihsaXN0ZW5lcikpXG4gICAgdGhyb3cgVHlwZUVycm9yKCdsaXN0ZW5lciBtdXN0IGJlIGEgZnVuY3Rpb24nKTtcblxuICBpZiAoIXRoaXMuX2V2ZW50cylcbiAgICB0aGlzLl9ldmVudHMgPSB7fTtcblxuICAvLyBUbyBhdm9pZCByZWN1cnNpb24gaW4gdGhlIGNhc2UgdGhhdCB0eXBlID09PSBcIm5ld0xpc3RlbmVyXCIhIEJlZm9yZVxuICAvLyBhZGRpbmcgaXQgdG8gdGhlIGxpc3RlbmVycywgZmlyc3QgZW1pdCBcIm5ld0xpc3RlbmVyXCIuXG4gIGlmICh0aGlzLl9ldmVudHMubmV3TGlzdGVuZXIpXG4gICAgdGhpcy5lbWl0KCduZXdMaXN0ZW5lcicsIHR5cGUsXG4gICAgICAgICAgICAgIGlzRnVuY3Rpb24obGlzdGVuZXIubGlzdGVuZXIpID9cbiAgICAgICAgICAgICAgbGlzdGVuZXIubGlzdGVuZXIgOiBsaXN0ZW5lcik7XG5cbiAgaWYgKCF0aGlzLl9ldmVudHNbdHlwZV0pXG4gICAgLy8gT3B0aW1pemUgdGhlIGNhc2Ugb2Ygb25lIGxpc3RlbmVyLiBEb24ndCBuZWVkIHRoZSBleHRyYSBhcnJheSBvYmplY3QuXG4gICAgdGhpcy5fZXZlbnRzW3R5cGVdID0gbGlzdGVuZXI7XG4gIGVsc2UgaWYgKGlzT2JqZWN0KHRoaXMuX2V2ZW50c1t0eXBlXSkpXG4gICAgLy8gSWYgd2UndmUgYWxyZWFkeSBnb3QgYW4gYXJyYXksIGp1c3QgYXBwZW5kLlxuICAgIHRoaXMuX2V2ZW50c1t0eXBlXS5wdXNoKGxpc3RlbmVyKTtcbiAgZWxzZVxuICAgIC8vIEFkZGluZyB0aGUgc2Vjb25kIGVsZW1lbnQsIG5lZWQgdG8gY2hhbmdlIHRvIGFycmF5LlxuICAgIHRoaXMuX2V2ZW50c1t0eXBlXSA9IFt0aGlzLl9ldmVudHNbdHlwZV0sIGxpc3RlbmVyXTtcblxuICAvLyBDaGVjayBmb3IgbGlzdGVuZXIgbGVha1xuICBpZiAoaXNPYmplY3QodGhpcy5fZXZlbnRzW3R5cGVdKSAmJiAhdGhpcy5fZXZlbnRzW3R5cGVdLndhcm5lZCkge1xuICAgIHZhciBtO1xuICAgIGlmICghaXNVbmRlZmluZWQodGhpcy5fbWF4TGlzdGVuZXJzKSkge1xuICAgICAgbSA9IHRoaXMuX21heExpc3RlbmVycztcbiAgICB9IGVsc2Uge1xuICAgICAgbSA9IEV2ZW50RW1pdHRlci5kZWZhdWx0TWF4TGlzdGVuZXJzO1xuICAgIH1cblxuICAgIGlmIChtICYmIG0gPiAwICYmIHRoaXMuX2V2ZW50c1t0eXBlXS5sZW5ndGggPiBtKSB7XG4gICAgICB0aGlzLl9ldmVudHNbdHlwZV0ud2FybmVkID0gdHJ1ZTtcbiAgICAgIGNvbnNvbGUuZXJyb3IoJyhub2RlKSB3YXJuaW5nOiBwb3NzaWJsZSBFdmVudEVtaXR0ZXIgbWVtb3J5ICcgK1xuICAgICAgICAgICAgICAgICAgICAnbGVhayBkZXRlY3RlZC4gJWQgbGlzdGVuZXJzIGFkZGVkLiAnICtcbiAgICAgICAgICAgICAgICAgICAgJ1VzZSBlbWl0dGVyLnNldE1heExpc3RlbmVycygpIHRvIGluY3JlYXNlIGxpbWl0LicsXG4gICAgICAgICAgICAgICAgICAgIHRoaXMuX2V2ZW50c1t0eXBlXS5sZW5ndGgpO1xuICAgICAgaWYgKHR5cGVvZiBjb25zb2xlLnRyYWNlID09PSAnZnVuY3Rpb24nKSB7XG4gICAgICAgIC8vIG5vdCBzdXBwb3J0ZWQgaW4gSUUgMTBcbiAgICAgICAgY29uc29sZS50cmFjZSgpO1xuICAgICAgfVxuICAgIH1cbiAgfVxuXG4gIHJldHVybiB0aGlzO1xufTtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5vbiA9IEV2ZW50RW1pdHRlci5wcm90b3R5cGUuYWRkTGlzdGVuZXI7XG5cbkV2ZW50RW1pdHRlci5wcm90b3R5cGUub25jZSA9IGZ1bmN0aW9uKHR5cGUsIGxpc3RlbmVyKSB7XG4gIGlmICghaXNGdW5jdGlvbihsaXN0ZW5lcikpXG4gICAgdGhyb3cgVHlwZUVycm9yKCdsaXN0ZW5lciBtdXN0IGJlIGEgZnVuY3Rpb24nKTtcblxuICB2YXIgZmlyZWQgPSBmYWxzZTtcblxuICBmdW5jdGlvbiBnKCkge1xuICAgIHRoaXMucmVtb3ZlTGlzdGVuZXIodHlwZSwgZyk7XG5cbiAgICBpZiAoIWZpcmVkKSB7XG4gICAgICBmaXJlZCA9IHRydWU7XG4gICAgICBsaXN0ZW5lci5hcHBseSh0aGlzLCBhcmd1bWVudHMpO1xuICAgIH1cbiAgfVxuXG4gIGcubGlzdGVuZXIgPSBsaXN0ZW5lcjtcbiAgdGhpcy5vbih0eXBlLCBnKTtcblxuICByZXR1cm4gdGhpcztcbn07XG5cbi8vIGVtaXRzIGEgJ3JlbW92ZUxpc3RlbmVyJyBldmVudCBpZmYgdGhlIGxpc3RlbmVyIHdhcyByZW1vdmVkXG5FdmVudEVtaXR0ZXIucHJvdG90eXBlLnJlbW92ZUxpc3RlbmVyID0gZnVuY3Rpb24odHlwZSwgbGlzdGVuZXIpIHtcbiAgdmFyIGxpc3QsIHBvc2l0aW9uLCBsZW5ndGgsIGk7XG5cbiAgaWYgKCFpc0Z1bmN0aW9uKGxpc3RlbmVyKSlcbiAgICB0aHJvdyBUeXBlRXJyb3IoJ2xpc3RlbmVyIG11c3QgYmUgYSBmdW5jdGlvbicpO1xuXG4gIGlmICghdGhpcy5fZXZlbnRzIHx8ICF0aGlzLl9ldmVudHNbdHlwZV0pXG4gICAgcmV0dXJuIHRoaXM7XG5cbiAgbGlzdCA9IHRoaXMuX2V2ZW50c1t0eXBlXTtcbiAgbGVuZ3RoID0gbGlzdC5sZW5ndGg7XG4gIHBvc2l0aW9uID0gLTE7XG5cbiAgaWYgKGxpc3QgPT09IGxpc3RlbmVyIHx8XG4gICAgICAoaXNGdW5jdGlvbihsaXN0Lmxpc3RlbmVyKSAmJiBsaXN0Lmxpc3RlbmVyID09PSBsaXN0ZW5lcikpIHtcbiAgICBkZWxldGUgdGhpcy5fZXZlbnRzW3R5cGVdO1xuICAgIGlmICh0aGlzLl9ldmVudHMucmVtb3ZlTGlzdGVuZXIpXG4gICAgICB0aGlzLmVtaXQoJ3JlbW92ZUxpc3RlbmVyJywgdHlwZSwgbGlzdGVuZXIpO1xuXG4gIH0gZWxzZSBpZiAoaXNPYmplY3QobGlzdCkpIHtcbiAgICBmb3IgKGkgPSBsZW5ndGg7IGktLSA+IDA7KSB7XG4gICAgICBpZiAobGlzdFtpXSA9PT0gbGlzdGVuZXIgfHxcbiAgICAgICAgICAobGlzdFtpXS5saXN0ZW5lciAmJiBsaXN0W2ldLmxpc3RlbmVyID09PSBsaXN0ZW5lcikpIHtcbiAgICAgICAgcG9zaXRpb24gPSBpO1xuICAgICAgICBicmVhaztcbiAgICAgIH1cbiAgICB9XG5cbiAgICBpZiAocG9zaXRpb24gPCAwKVxuICAgICAgcmV0dXJuIHRoaXM7XG5cbiAgICBpZiAobGlzdC5sZW5ndGggPT09IDEpIHtcbiAgICAgIGxpc3QubGVuZ3RoID0gMDtcbiAgICAgIGRlbGV0ZSB0aGlzLl9ldmVudHNbdHlwZV07XG4gICAgfSBlbHNlIHtcbiAgICAgIGxpc3Quc3BsaWNlKHBvc2l0aW9uLCAxKTtcbiAgICB9XG5cbiAgICBpZiAodGhpcy5fZXZlbnRzLnJlbW92ZUxpc3RlbmVyKVxuICAgICAgdGhpcy5lbWl0KCdyZW1vdmVMaXN0ZW5lcicsIHR5cGUsIGxpc3RlbmVyKTtcbiAgfVxuXG4gIHJldHVybiB0aGlzO1xufTtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5yZW1vdmVBbGxMaXN0ZW5lcnMgPSBmdW5jdGlvbih0eXBlKSB7XG4gIHZhciBrZXksIGxpc3RlbmVycztcblxuICBpZiAoIXRoaXMuX2V2ZW50cylcbiAgICByZXR1cm4gdGhpcztcblxuICAvLyBub3QgbGlzdGVuaW5nIGZvciByZW1vdmVMaXN0ZW5lciwgbm8gbmVlZCB0byBlbWl0XG4gIGlmICghdGhpcy5fZXZlbnRzLnJlbW92ZUxpc3RlbmVyKSB7XG4gICAgaWYgKGFyZ3VtZW50cy5sZW5ndGggPT09IDApXG4gICAgICB0aGlzLl9ldmVudHMgPSB7fTtcbiAgICBlbHNlIGlmICh0aGlzLl9ldmVudHNbdHlwZV0pXG4gICAgICBkZWxldGUgdGhpcy5fZXZlbnRzW3R5cGVdO1xuICAgIHJldHVybiB0aGlzO1xuICB9XG5cbiAgLy8gZW1pdCByZW1vdmVMaXN0ZW5lciBmb3IgYWxsIGxpc3RlbmVycyBvbiBhbGwgZXZlbnRzXG4gIGlmIChhcmd1bWVudHMubGVuZ3RoID09PSAwKSB7XG4gICAgZm9yIChrZXkgaW4gdGhpcy5fZXZlbnRzKSB7XG4gICAgICBpZiAoa2V5ID09PSAncmVtb3ZlTGlzdGVuZXInKSBjb250aW51ZTtcbiAgICAgIHRoaXMucmVtb3ZlQWxsTGlzdGVuZXJzKGtleSk7XG4gICAgfVxuICAgIHRoaXMucmVtb3ZlQWxsTGlzdGVuZXJzKCdyZW1vdmVMaXN0ZW5lcicpO1xuICAgIHRoaXMuX2V2ZW50cyA9IHt9O1xuICAgIHJldHVybiB0aGlzO1xuICB9XG5cbiAgbGlzdGVuZXJzID0gdGhpcy5fZXZlbnRzW3R5cGVdO1xuXG4gIGlmIChpc0Z1bmN0aW9uKGxpc3RlbmVycykpIHtcbiAgICB0aGlzLnJlbW92ZUxpc3RlbmVyKHR5cGUsIGxpc3RlbmVycyk7XG4gIH0gZWxzZSB7XG4gICAgLy8gTElGTyBvcmRlclxuICAgIHdoaWxlIChsaXN0ZW5lcnMubGVuZ3RoKVxuICAgICAgdGhpcy5yZW1vdmVMaXN0ZW5lcih0eXBlLCBsaXN0ZW5lcnNbbGlzdGVuZXJzLmxlbmd0aCAtIDFdKTtcbiAgfVxuICBkZWxldGUgdGhpcy5fZXZlbnRzW3R5cGVdO1xuXG4gIHJldHVybiB0aGlzO1xufTtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5saXN0ZW5lcnMgPSBmdW5jdGlvbih0eXBlKSB7XG4gIHZhciByZXQ7XG4gIGlmICghdGhpcy5fZXZlbnRzIHx8ICF0aGlzLl9ldmVudHNbdHlwZV0pXG4gICAgcmV0ID0gW107XG4gIGVsc2UgaWYgKGlzRnVuY3Rpb24odGhpcy5fZXZlbnRzW3R5cGVdKSlcbiAgICByZXQgPSBbdGhpcy5fZXZlbnRzW3R5cGVdXTtcbiAgZWxzZVxuICAgIHJldCA9IHRoaXMuX2V2ZW50c1t0eXBlXS5zbGljZSgpO1xuICByZXR1cm4gcmV0O1xufTtcblxuRXZlbnRFbWl0dGVyLmxpc3RlbmVyQ291bnQgPSBmdW5jdGlvbihlbWl0dGVyLCB0eXBlKSB7XG4gIHZhciByZXQ7XG4gIGlmICghZW1pdHRlci5fZXZlbnRzIHx8ICFlbWl0dGVyLl9ldmVudHNbdHlwZV0pXG4gICAgcmV0ID0gMDtcbiAgZWxzZSBpZiAoaXNGdW5jdGlvbihlbWl0dGVyLl9ldmVudHNbdHlwZV0pKVxuICAgIHJldCA9IDE7XG4gIGVsc2VcbiAgICByZXQgPSBlbWl0dGVyLl9ldmVudHNbdHlwZV0ubGVuZ3RoO1xuICByZXR1cm4gcmV0O1xufTtcblxuZnVuY3Rpb24gaXNGdW5jdGlvbihhcmcpIHtcbiAgcmV0dXJuIHR5cGVvZiBhcmcgPT09ICdmdW5jdGlvbic7XG59XG5cbmZ1bmN0aW9uIGlzTnVtYmVyKGFyZykge1xuICByZXR1cm4gdHlwZW9mIGFyZyA9PT0gJ251bWJlcic7XG59XG5cbmZ1bmN0aW9uIGlzT2JqZWN0KGFyZykge1xuICByZXR1cm4gdHlwZW9mIGFyZyA9PT0gJ29iamVjdCcgJiYgYXJnICE9PSBudWxsO1xufVxuXG5mdW5jdGlvbiBpc1VuZGVmaW5lZChhcmcpIHtcbiAgcmV0dXJuIGFyZyA9PT0gdm9pZCAwO1xufVxuXG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiAuL34vZXZlbnRzL2V2ZW50cy5qc1xuICoqIG1vZHVsZSBpZCA9IDI4OFxuICoqIG1vZHVsZSBjaHVua3MgPSAxXG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciBOYXZMZWZ0QmFyID0gcmVxdWlyZSgnLi9uYXZMZWZ0QmFyJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHthY3Rpb25zLCBnZXR0ZXJzfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2FwcCcpO1xudmFyIFNlbGVjdE5vZGVEaWFsb2cgPSByZXF1aXJlKCcuL3NlbGVjdE5vZGVEaWFsb2cuanN4Jyk7XG5cbnZhciBBcHAgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIGFwcDogZ2V0dGVycy5hcHBTdGF0ZVxuICAgIH1cbiAgfSxcblxuICBjb21wb25lbnRXaWxsTW91bnQoKXtcbiAgICBhY3Rpb25zLmluaXRBcHAoKTtcbiAgICB0aGlzLnJlZnJlc2hJbnRlcnZhbCA9IHNldEludGVydmFsKGFjdGlvbnMuZmV0Y2hOb2Rlc0FuZFNlc3Npb25zLCAzNTAwMCk7XG4gIH0sXG5cbiAgY29tcG9uZW50V2lsbFVubW91bnQ6IGZ1bmN0aW9uKCkge1xuICAgIGNsZWFySW50ZXJ2YWwodGhpcy5yZWZyZXNoSW50ZXJ2YWwpO1xuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgaWYodGhpcy5zdGF0ZS5hcHAuaXNJbml0aWFsaXppbmcpe1xuICAgICAgcmV0dXJuIG51bGw7XG4gICAgfVxuXG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXRscHQgZ3J2LWZsZXggZ3J2LWZsZXgtcm93XCI+XG4gICAgICAgIDxTZWxlY3ROb2RlRGlhbG9nLz5cbiAgICAgICAge3RoaXMucHJvcHMuQ3VycmVudFNlc3Npb25Ib3N0fVxuICAgICAgICA8TmF2TGVmdEJhci8+XG4gICAgICAgIHt0aGlzLnByb3BzLmNoaWxkcmVufVxuICAgICAgPC9kaXY+XG4gICAgKTtcbiAgfVxufSlcblxubW9kdWxlLmV4cG9ydHMgPSBBcHA7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9hcHAuanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7Z2V0dGVycywgYWN0aW9uc30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC8nKTtcbnZhciB7bm9kZUhvc3ROYW1lQnlTZXJ2ZXJJZH0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9ub2Rlcy9nZXR0ZXJzJyk7XG52YXIgVHR5ID0gcmVxdWlyZSgnYXBwL2NvbW1vbi90dHknKTtcbnZhciBUdHlUZXJtaW5hbCA9IHJlcXVpcmUoJy4vLi4vdGVybWluYWwuanN4Jyk7XG52YXIgRXZlbnRTdHJlYW1lciA9IHJlcXVpcmUoJy4vZXZlbnRTdHJlYW1lci5qc3gnKTtcbnZhciBTZXNzaW9uTGVmdFBhbmVsID0gcmVxdWlyZSgnLi9zZXNzaW9uTGVmdFBhbmVsJyk7XG52YXIge3Nob3dTZWxlY3ROb2RlRGlhbG9nLCBjbG9zZVNlbGVjdE5vZGVEaWFsb2d9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvZGlhbG9ncy9hY3Rpb25zJyk7XG52YXIgU2VsZWN0Tm9kZURpYWxvZyA9IHJlcXVpcmUoJy4vLi4vc2VsZWN0Tm9kZURpYWxvZy5qc3gnKTtcblxudmFyIEFjdGl2ZVNlc3Npb24gPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgY29tcG9uZW50V2lsbFVubW91bnQoKXtcbiAgICBjbG9zZVNlbGVjdE5vZGVEaWFsb2coKTtcbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIGxldCB7bG9naW4sIHBhcnRpZXMsIHNlcnZlcklkfSA9IHRoaXMucHJvcHMuYWN0aXZlU2Vzc2lvbjtcbiAgICBsZXQgc2VydmVyTGFiZWxUZXh0ID0gJyc7XG4gICAgaWYoc2VydmVySWQpe1xuICAgICAgbGV0IGhvc3RuYW1lID0gcmVhY3Rvci5ldmFsdWF0ZShub2RlSG9zdE5hbWVCeVNlcnZlcklkKHNlcnZlcklkKSk7XG4gICAgICBzZXJ2ZXJMYWJlbFRleHQgPSBgJHtsb2dpbn1AJHtob3N0bmFtZX1gO1xuICAgIH1cblxuICAgIHJldHVybiAoXG4gICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWN1cnJlbnQtc2Vzc2lvblwiPlxuICAgICAgIDxTZXNzaW9uTGVmdFBhbmVsIHBhcnRpZXM9e3BhcnRpZXN9Lz5cbiAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1jdXJyZW50LXNlc3Npb24tc2VydmVyLWluZm9cIj5cbiAgICAgICAgIDxoMz57c2VydmVyTGFiZWxUZXh0fTwvaDM+XG4gICAgICAgPC9kaXY+XG4gICAgICAgPFR0eUNvbm5lY3Rpb24gey4uLnRoaXMucHJvcHMuYWN0aXZlU2Vzc2lvbn0gLz5cbiAgICAgPC9kaXY+XG4gICAgICk7XG4gIH1cbn0pO1xuXG52YXIgVHR5Q29ubmVjdGlvbiA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgdGhpcy50dHkgPSBuZXcgVHR5KHRoaXMucHJvcHMpXG4gICAgdGhpcy50dHkub24oJ29wZW4nLCAoKT0+IHRoaXMuc2V0U3RhdGUoeyAuLi50aGlzLnN0YXRlLCBpc0Nvbm5lY3RlZDogdHJ1ZSB9KSk7XG5cbiAgICB2YXIge3NlcnZlcklkLCBsb2dpbn0gPSB0aGlzLnByb3BzO1xuICAgIHJldHVybiB7c2VydmVySWQsIGxvZ2luLCBpc0Nvbm5lY3RlZDogZmFsc2V9O1xuICB9LFxuXG4gIGNvbXBvbmVudERpZE1vdW50KCl7XG4gICAgLy8gdGVtcG9yYXJ5IGhhY2tcbiAgICBTZWxlY3ROb2RlRGlhbG9nLm9uU2VydmVyQ2hhbmdlQ2FsbEJhY2sgPSB0aGlzLmNvbXBvbmVudFdpbGxSZWNlaXZlUHJvcHMuYmluZCh0aGlzKTtcbiAgfSxcblxuICBjb21wb25lbnRXaWxsVW5tb3VudCgpIHtcbiAgICBTZWxlY3ROb2RlRGlhbG9nLm9uU2VydmVyQ2hhbmdlQ2FsbEJhY2sgPSBudWxsO1xuICAgIHRoaXMudHR5LmRpc2Nvbm5lY3QoKTtcbiAgfSxcblxuICBjb21wb25lbnRXaWxsUmVjZWl2ZVByb3BzKG5leHRQcm9wcyl7XG4gICAgdmFyIHtzZXJ2ZXJJZH0gPSBuZXh0UHJvcHM7XG4gICAgaWYoc2VydmVySWQgJiYgc2VydmVySWQgIT09IHRoaXMuc3RhdGUuc2VydmVySWQpe1xuICAgICAgdGhpcy50dHkucmVjb25uZWN0KHtzZXJ2ZXJJZH0pO1xuICAgICAgdGhpcy5yZWZzLnR0eUNtbnRJbnN0YW5jZS50ZXJtLmZvY3VzKCk7XG4gICAgICB0aGlzLnNldFN0YXRlKHsuLi50aGlzLnN0YXRlLCBzZXJ2ZXJJZCB9KTtcbiAgICB9XG4gIH0sXG5cbiAgcmVuZGVyKCkge1xuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IHN0eWxlPXt7aGVpZ2h0OiAnMTAwJSd9fT5cbiAgICAgICAgPFR0eVRlcm1pbmFsIHJlZj1cInR0eUNtbnRJbnN0YW5jZVwiIHR0eT17dGhpcy50dHl9IGNvbHM9e3RoaXMucHJvcHMuY29sc30gcm93cz17dGhpcy5wcm9wcy5yb3dzfSAvPlxuICAgICAgICB7IHRoaXMuc3RhdGUuaXNDb25uZWN0ZWQgPyA8RXZlbnRTdHJlYW1lciBzaWQ9e3RoaXMucHJvcHMuc2lkfS8+IDogbnVsbCB9XG4gICAgICA8L2Rpdj5cbiAgICApXG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IEFjdGl2ZVNlc3Npb247XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9jdXJyZW50U2Vzc2lvbi9hY3RpdmVTZXNzaW9uLmpzeFxuICoqLyIsInZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCdhcHAvc2Vzc2lvbicpO1xudmFyIHt1cGRhdGVTZXNzaW9ufSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3Nlc3Npb25zL2FjdGlvbnMnKTtcblxudmFyIEV2ZW50U3RyZWFtZXIgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIGNvbXBvbmVudERpZE1vdW50KCkge1xuICAgIGxldCB7c2lkfSA9IHRoaXMucHJvcHM7XG4gICAgbGV0IHt0b2tlbn0gPSBzZXNzaW9uLmdldFVzZXJEYXRhKCk7XG4gICAgbGV0IGNvbm5TdHIgPSBjZmcuYXBpLmdldEV2ZW50U3RyZWFtQ29ublN0cih0b2tlbiwgc2lkKTtcblxuICAgIHRoaXMuc29ja2V0ID0gbmV3IFdlYlNvY2tldChjb25uU3RyLCAncHJvdG8nKTtcbiAgICB0aGlzLnNvY2tldC5vbm1lc3NhZ2UgPSAoZXZlbnQpID0+IHtcbiAgICAgIHRyeVxuICAgICAge1xuICAgICAgICBsZXQganNvbiA9IEpTT04ucGFyc2UoZXZlbnQuZGF0YSk7XG4gICAgICAgIHVwZGF0ZVNlc3Npb24oanNvbi5zZXNzaW9uKTtcbiAgICAgIH1cbiAgICAgIGNhdGNoKGVycil7XG4gICAgICAgIGNvbnNvbGUubG9nKCdmYWlsZWQgdG8gcGFyc2UgZXZlbnQgc3RyZWFtIGRhdGEnKTtcbiAgICAgIH1cblxuICAgIH07XG4gICAgdGhpcy5zb2NrZXQub25jbG9zZSA9ICgpID0+IHt9O1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50KCkge1xuICAgIHRoaXMuc29ja2V0LmNsb3NlKCk7XG4gIH0sXG5cbiAgc2hvdWxkQ29tcG9uZW50VXBkYXRlKCkge1xuICAgIHJldHVybiBmYWxzZTtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuIG51bGw7XG4gIH1cbn0pO1xuXG5leHBvcnQgZGVmYXVsdCBFdmVudFN0cmVhbWVyO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvY3VycmVudFNlc3Npb24vZXZlbnRTdHJlYW1lci5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHtnZXR0ZXJzLCBhY3Rpb25zfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsLycpO1xudmFyIFNlc3Npb25QbGF5ZXIgPSByZXF1aXJlKCcuL3Nlc3Npb25QbGF5ZXIuanN4Jyk7XG52YXIgQWN0aXZlU2Vzc2lvbiA9IHJlcXVpcmUoJy4vYWN0aXZlU2Vzc2lvbi5qc3gnKTtcblxudmFyIEN1cnJlbnRTZXNzaW9uSG9zdCA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBtaXhpbnM6IFtyZWFjdG9yLlJlYWN0TWl4aW5dLFxuXG4gIGdldERhdGFCaW5kaW5ncygpIHtcbiAgICByZXR1cm4ge1xuICAgICAgY3VycmVudFNlc3Npb246IGdldHRlcnMuYWN0aXZlU2Vzc2lvblxuICAgIH1cbiAgfSxcblxuICBjb21wb25lbnREaWRNb3VudCgpe1xuICAgIHZhciB7IHNpZCB9ID0gdGhpcy5wcm9wcy5wYXJhbXM7XG4gICAgaWYoIXRoaXMuc3RhdGUuY3VycmVudFNlc3Npb24pe1xuICAgICAgYWN0aW9ucy5vcGVuU2Vzc2lvbihzaWQpO1xuICAgIH1cbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIHZhciBjdXJyZW50U2Vzc2lvbiA9IHRoaXMuc3RhdGUuY3VycmVudFNlc3Npb247XG4gICAgaWYoIWN1cnJlbnRTZXNzaW9uKXtcbiAgICAgIHJldHVybiBudWxsO1xuICAgIH1cblxuICAgIGlmKGN1cnJlbnRTZXNzaW9uLmlzTmV3U2Vzc2lvbiB8fCBjdXJyZW50U2Vzc2lvbi5hY3RpdmUpe1xuICAgICAgcmV0dXJuIDxBY3RpdmVTZXNzaW9uIGFjdGl2ZVNlc3Npb249e2N1cnJlbnRTZXNzaW9ufS8+O1xuICAgIH1cblxuICAgIHJldHVybiA8U2Vzc2lvblBsYXllciBhY3RpdmVTZXNzaW9uPXtjdXJyZW50U2Vzc2lvbn0vPjtcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gQ3VycmVudFNlc3Npb25Ib3N0O1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvY3VycmVudFNlc3Npb24vbWFpbi5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIFJlYWN0U2xpZGVyID0gcmVxdWlyZSgncmVhY3Qtc2xpZGVyJyk7XG52YXIgVHR5UGxheWVyID0gcmVxdWlyZSgnYXBwL2NvbW1vbi90dHlQbGF5ZXInKVxudmFyIFR0eVRlcm1pbmFsID0gcmVxdWlyZSgnLi8uLi90ZXJtaW5hbC5qc3gnKTtcbnZhciBTZXNzaW9uTGVmdFBhbmVsID0gcmVxdWlyZSgnLi9zZXNzaW9uTGVmdFBhbmVsJyk7XG5cbnZhciBTZXNzaW9uUGxheWVyID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICBjYWxjdWxhdGVTdGF0ZSgpe1xuICAgIHJldHVybiB7XG4gICAgICBsZW5ndGg6IHRoaXMudHR5Lmxlbmd0aCxcbiAgICAgIG1pbjogMSxcbiAgICAgIGlzUGxheWluZzogdGhpcy50dHkuaXNQbGF5aW5nLFxuICAgICAgY3VycmVudDogdGhpcy50dHkuY3VycmVudCxcbiAgICAgIGNhblBsYXk6IHRoaXMudHR5Lmxlbmd0aCA+IDFcbiAgICB9O1xuICB9LFxuXG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICB2YXIgc2lkID0gdGhpcy5wcm9wcy5hY3RpdmVTZXNzaW9uLnNpZDtcbiAgICB0aGlzLnR0eSA9IG5ldyBUdHlQbGF5ZXIoe3NpZH0pO1xuICAgIHJldHVybiB0aGlzLmNhbGN1bGF0ZVN0YXRlKCk7XG4gIH0sXG5cbiAgY29tcG9uZW50V2lsbFVubW91bnQoKSB7XG4gICAgdGhpcy50dHkuc3RvcCgpO1xuICAgIHRoaXMudHR5LnJlbW92ZUFsbExpc3RlbmVycygpO1xuICB9LFxuXG4gIGNvbXBvbmVudERpZE1vdW50KCkge1xuICAgIHRoaXMudHR5Lm9uKCdjaGFuZ2UnLCAoKT0+e1xuICAgICAgdmFyIG5ld1N0YXRlID0gdGhpcy5jYWxjdWxhdGVTdGF0ZSgpO1xuICAgICAgdGhpcy5zZXRTdGF0ZShuZXdTdGF0ZSk7XG4gICAgfSk7XG4gIH0sXG5cbiAgdG9nZ2xlUGxheVN0b3AoKXtcbiAgICBpZih0aGlzLnN0YXRlLmlzUGxheWluZyl7XG4gICAgICB0aGlzLnR0eS5zdG9wKCk7XG4gICAgfWVsc2V7XG4gICAgICB0aGlzLnR0eS5wbGF5KCk7XG4gICAgfVxuICB9LFxuXG4gIG1vdmUodmFsdWUpe1xuICAgIHRoaXMudHR5Lm1vdmUodmFsdWUpO1xuICB9LFxuXG4gIG9uQmVmb3JlQ2hhbmdlKCl7XG4gICAgdGhpcy50dHkuc3RvcCgpO1xuICB9LFxuXG4gIG9uQWZ0ZXJDaGFuZ2UodmFsdWUpe1xuICAgIHRoaXMudHR5LnBsYXkoKTtcbiAgICB0aGlzLnR0eS5tb3ZlKHZhbHVlKTtcbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIHZhciB7aXNQbGF5aW5nfSA9IHRoaXMuc3RhdGU7XG5cbiAgICByZXR1cm4gKFxuICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1jdXJyZW50LXNlc3Npb24gZ3J2LXNlc3Npb24tcGxheWVyXCI+XG4gICAgICAgPFNlc3Npb25MZWZ0UGFuZWwvPlxuICAgICAgIDxUdHlUZXJtaW5hbCByZWY9XCJ0ZXJtXCIgdHR5PXt0aGlzLnR0eX0gY29scz1cIjVcIiByb3dzPVwiNVwiIC8+XG4gICAgICAgPFJlYWN0U2xpZGVyXG4gICAgICAgICAgbWluPXt0aGlzLnN0YXRlLm1pbn1cbiAgICAgICAgICBtYXg9e3RoaXMuc3RhdGUubGVuZ3RofVxuICAgICAgICAgIHZhbHVlPXt0aGlzLnN0YXRlLmN1cnJlbnR9ICAgIFxuICAgICAgICAgIG9uQWZ0ZXJDaGFuZ2U9e3RoaXMub25BZnRlckNoYW5nZX1cbiAgICAgICAgICBvbkJlZm9yZUNoYW5nZT17dGhpcy5vbkJlZm9yZUNoYW5nZX1cbiAgICAgICAgICBkZWZhdWx0VmFsdWU9ezF9XG4gICAgICAgICAgd2l0aEJhcnNcbiAgICAgICAgICBjbGFzc05hbWU9XCJncnYtc2xpZGVyXCI+XG4gICAgICAgPC9SZWFjdFNsaWRlcj5cbiAgICAgICA8YnV0dG9uIGNsYXNzTmFtZT1cImJ0blwiIG9uQ2xpY2s9e3RoaXMudG9nZ2xlUGxheVN0b3B9PlxuICAgICAgICAgeyBpc1BsYXlpbmcgPyA8aSBjbGFzc05hbWU9XCJmYSBmYS1zdG9wXCI+PC9pPiA6ICA8aSBjbGFzc05hbWU9XCJmYSBmYS1wbGF5XCI+PC9pPiB9XG4gICAgICAgPC9idXR0b24+XG4gICAgIDwvZGl2PlxuICAgICApO1xuICB9XG59KTtcblxuZXhwb3J0IGRlZmF1bHQgU2Vzc2lvblBsYXllcjtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2N1cnJlbnRTZXNzaW9uL3Nlc3Npb25QbGF5ZXIuanN4XG4gKiovIiwibW9kdWxlLmV4cG9ydHMuQXBwID0gcmVxdWlyZSgnLi9hcHAuanN4Jyk7XG5tb2R1bGUuZXhwb3J0cy5Mb2dpbiA9IHJlcXVpcmUoJy4vbG9naW4uanN4Jyk7XG5tb2R1bGUuZXhwb3J0cy5OZXdVc2VyID0gcmVxdWlyZSgnLi9uZXdVc2VyLmpzeCcpO1xubW9kdWxlLmV4cG9ydHMuTm9kZXMgPSByZXF1aXJlKCcuL25vZGVzL21haW4uanN4Jyk7XG5tb2R1bGUuZXhwb3J0cy5TZXNzaW9ucyA9IHJlcXVpcmUoJy4vc2Vzc2lvbnMvbWFpbi5qc3gnKTtcbm1vZHVsZS5leHBvcnRzLkN1cnJlbnRTZXNzaW9uSG9zdCA9IHJlcXVpcmUoJy4vY3VycmVudFNlc3Npb24vbWFpbi5qc3gnKTtcbm1vZHVsZS5leHBvcnRzLk5vdEZvdW5kID0gcmVxdWlyZSgnLi9lcnJvclBhZ2UuanN4JykuTm90Rm91bmQ7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9pbmRleC5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyICQgPSByZXF1aXJlKCdqUXVlcnknKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciBMaW5rZWRTdGF0ZU1peGluID0gcmVxdWlyZSgncmVhY3QtYWRkb25zLWxpbmtlZC1zdGF0ZS1taXhpbicpO1xudmFyIHthY3Rpb25zLCBnZXR0ZXJzfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3VzZXInKTtcbnZhciBHb29nbGVBdXRoSW5mbyA9IHJlcXVpcmUoJy4vZ29vZ2xlQXV0aExvZ28nKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG5cbnZhciBMb2dpbklucHV0Rm9ybSA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBtaXhpbnM6IFtMaW5rZWRTdGF0ZU1peGluXSxcblxuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIHVzZXI6ICcnLFxuICAgICAgcGFzc3dvcmQ6ICcnLFxuICAgICAgdG9rZW46ICcnXG4gICAgfVxuICB9LFxuXG4gIG9uQ2xpY2s6IGZ1bmN0aW9uKGUpIHtcbiAgICBlLnByZXZlbnREZWZhdWx0KCk7XG4gICAgaWYgKHRoaXMuaXNWYWxpZCgpKSB7XG4gICAgICB0aGlzLnByb3BzLm9uQ2xpY2sodGhpcy5zdGF0ZSk7XG4gICAgfVxuICB9LFxuXG4gIGlzVmFsaWQ6IGZ1bmN0aW9uKCkge1xuICAgIHZhciAkZm9ybSA9ICQodGhpcy5yZWZzLmZvcm0pO1xuICAgIHJldHVybiAkZm9ybS5sZW5ndGggPT09IDAgfHwgJGZvcm0udmFsaWQoKTtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgbGV0IHtpc1Byb2Nlc3NpbmcsIGlzRmFpbGVkLCBtZXNzYWdlIH0gPSB0aGlzLnByb3BzLmF0dGVtcDtcblxuICAgIHJldHVybiAoXG4gICAgICA8Zm9ybSByZWY9XCJmb3JtXCIgY2xhc3NOYW1lPVwiZ3J2LWxvZ2luLWlucHV0LWZvcm1cIj5cbiAgICAgICAgPGgzPiBXZWxjb21lIHRvIFRlbGVwb3J0IDwvaDM+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiXCI+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXQgYXV0b0ZvY3VzIHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3VzZXInKX0gY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sIHJlcXVpcmVkXCIgcGxhY2Vob2xkZXI9XCJVc2VyIG5hbWVcIiBuYW1lPVwidXNlck5hbWVcIiAvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0IHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3Bhc3N3b3JkJyl9IHR5cGU9XCJwYXNzd29yZFwiIG5hbWU9XCJwYXNzd29yZFwiIGNsYXNzTmFtZT1cImZvcm0tY29udHJvbCByZXF1aXJlZFwiIHBsYWNlaG9sZGVyPVwiUGFzc3dvcmRcIi8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXQgdmFsdWVMaW5rPXt0aGlzLmxpbmtTdGF0ZSgndG9rZW4nKX0gY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sIHJlcXVpcmVkXCIgbmFtZT1cInRva2VuXCIgcGxhY2Vob2xkZXI9XCJUd28gZmFjdG9yIHRva2VuIChHb29nbGUgQXV0aGVudGljYXRvcilcIi8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGJ1dHRvbiBvbkNsaWNrPXt0aGlzLm9uQ2xpY2t9IGRpc2FibGVkPXtpc1Byb2Nlc3Npbmd9IHR5cGU9XCJzdWJtaXRcIiBjbGFzc05hbWU9XCJidG4gYnRuLXByaW1hcnkgYmxvY2sgZnVsbC13aWR0aCBtLWJcIj5Mb2dpbjwvYnV0dG9uPlxuICAgICAgICAgIHsgaXNGYWlsZWQgPyAoPGxhYmVsIGNsYXNzTmFtZT1cImVycm9yXCI+e21lc3NhZ2V9PC9sYWJlbD4pIDogbnVsbCB9XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9mb3JtPlxuICAgICk7XG4gIH1cbn0pXG5cbnZhciBMb2dpbiA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBtaXhpbnM6IFtyZWFjdG9yLlJlYWN0TWl4aW5dLFxuXG4gIGdldERhdGFCaW5kaW5ncygpIHtcbiAgICByZXR1cm4ge1xuICAgICAgYXR0ZW1wOiBnZXR0ZXJzLmxvZ2luQXR0ZW1wXG4gICAgfVxuICB9LFxuXG4gIG9uQ2xpY2soaW5wdXREYXRhKXtcbiAgICB2YXIgbG9jID0gdGhpcy5wcm9wcy5sb2NhdGlvbjtcbiAgICB2YXIgcmVkaXJlY3QgPSBjZmcucm91dGVzLmFwcDtcblxuICAgIGlmKGxvYy5zdGF0ZSAmJiBsb2Muc3RhdGUucmVkaXJlY3RUbyl7XG4gICAgICByZWRpcmVjdCA9IGxvYy5zdGF0ZS5yZWRpcmVjdFRvO1xuICAgIH1cblxuICAgIGFjdGlvbnMubG9naW4oaW5wdXREYXRhLCByZWRpcmVjdCk7XG4gIH0sXG5cbiAgcmVuZGVyKCkgeyAgICBcbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbG9naW4gdGV4dC1jZW50ZXJcIj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbG9nby10cHJ0XCI+PC9kaXY+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWNvbnRlbnQgZ3J2LWZsZXhcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LWNvbHVtblwiPlxuICAgICAgICAgICAgPExvZ2luSW5wdXRGb3JtIGF0dGVtcD17dGhpcy5zdGF0ZS5hdHRlbXB9IG9uQ2xpY2s9e3RoaXMub25DbGlja30vPlxuICAgICAgICAgICAgPEdvb2dsZUF1dGhJbmZvLz5cbiAgICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWxvZ2luLWluZm9cIj5cbiAgICAgICAgICAgICAgPGkgY2xhc3NOYW1lPVwiZmEgZmEtcXVlc3Rpb25cIj48L2k+XG4gICAgICAgICAgICAgIDxzdHJvbmc+TmV3IEFjY291bnQgb3IgZm9yZ290IHBhc3N3b3JkPzwvc3Ryb25nPlxuICAgICAgICAgICAgICA8ZGl2PkFzayBmb3IgYXNzaXN0YW5jZSBmcm9tIHlvdXIgQ29tcGFueSBhZG1pbmlzdHJhdG9yPC9kaXY+XG4gICAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBMb2dpbjtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2xvZ2luLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgeyBSb3V0ZXIsIEluZGV4TGluaywgSGlzdG9yeSB9ID0gcmVxdWlyZSgncmVhY3Qtcm91dGVyJyk7XG52YXIgZ2V0dGVycyA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3VzZXIvZ2V0dGVycycpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxudmFyIG1lbnVJdGVtcyA9IFtcbiAge2ljb246ICdmYSBmYS1jb2dzJywgdG86IGNmZy5yb3V0ZXMubm9kZXMsIHRpdGxlOiAnTm9kZXMnfSxcbiAge2ljb246ICdmYSBmYS1zaXRlbWFwJywgdG86IGNmZy5yb3V0ZXMuc2Vzc2lvbnMsIHRpdGxlOiAnU2Vzc2lvbnMnfVxuXTtcblxudmFyIE5hdkxlZnRCYXIgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpe1xuICAgIHZhciBpdGVtcyA9IG1lbnVJdGVtcy5tYXAoKGksIGluZGV4KT0+e1xuICAgICAgdmFyIGNsYXNzTmFtZSA9IHRoaXMuY29udGV4dC5yb3V0ZXIuaXNBY3RpdmUoaS50bykgPyAnYWN0aXZlJyA6ICcnO1xuICAgICAgcmV0dXJuIChcbiAgICAgICAgPGxpIGtleT17aW5kZXh9IGNsYXNzTmFtZT17Y2xhc3NOYW1lfSB0aXRsZT17aS50aXRsZX0+XG4gICAgICAgICAgPEluZGV4TGluayB0bz17aS50b30+XG4gICAgICAgICAgICA8aSBjbGFzc05hbWU9e2kuaWNvbn0gLz5cbiAgICAgICAgICA8L0luZGV4TGluaz5cbiAgICAgICAgPC9saT5cbiAgICAgICk7XG4gICAgfSk7XG5cbiAgICBpdGVtcy5wdXNoKChcbiAgICAgIDxsaSBrZXk9e2l0ZW1zLmxlbmd0aH0gdGl0bGU9XCJoZWxwXCI+XG4gICAgICAgIDxhIGhyZWY9e2NmZy5oZWxwVXJsfSB0YXJnZXQ9XCJfYmxhbmtcIj5cbiAgICAgICAgICA8aSBjbGFzc05hbWU9XCJmYSBmYS1xdWVzdGlvblwiIC8+XG4gICAgICAgIDwvYT5cbiAgICAgIDwvbGk+KSk7XG5cbiAgICBpdGVtcy5wdXNoKChcbiAgICAgIDxsaSBrZXk9e2l0ZW1zLmxlbmd0aH0gdGl0bGU9XCJsb2dvdXRcIj5cbiAgICAgICAgPGEgaHJlZj17Y2ZnLnJvdXRlcy5sb2dvdXR9PlxuICAgICAgICAgIDxpIGNsYXNzTmFtZT1cImZhIGZhLXNpZ24tb3V0XCI+PC9pPlxuICAgICAgICA8L2E+XG4gICAgICA8L2xpPlxuICAgICkpO1xuXG4gICAgcmV0dXJuIChcbiAgICAgIDxuYXYgY2xhc3NOYW1lPSdncnYtbmF2IG5hdmJhci1kZWZhdWx0JyByb2xlPSduYXZpZ2F0aW9uJz5cbiAgICAgICAgPHVsIGNsYXNzTmFtZT0nbmF2IHRleHQtY2VudGVyJyBpZD0nc2lkZS1tZW51Jz5cbiAgICAgICAgICA8bGkgdGl0bGU9XCJjdXJyZW50IHVzZXJcIj48ZGl2IGNsYXNzTmFtZT1cImdydi1jaXJjbGUgdGV4dC11cHBlcmNhc2VcIj48c3Bhbj57Z2V0VXNlck5hbWVMZXR0ZXIoKX08L3NwYW4+PC9kaXY+PC9saT5cbiAgICAgICAgICB7aXRlbXN9XG4gICAgICAgIDwvdWw+XG4gICAgICA8L25hdj5cbiAgICApO1xuICB9XG59KTtcblxuTmF2TGVmdEJhci5jb250ZXh0VHlwZXMgPSB7XG4gIHJvdXRlcjogUmVhY3QuUHJvcFR5cGVzLm9iamVjdC5pc1JlcXVpcmVkXG59XG5cbmZ1bmN0aW9uIGdldFVzZXJOYW1lTGV0dGVyKCl7XG4gIHZhciB7c2hvcnREaXNwbGF5TmFtZX0gPSByZWFjdG9yLmV2YWx1YXRlKGdldHRlcnMudXNlcik7XG4gIHJldHVybiBzaG9ydERpc3BsYXlOYW1lO1xufVxuXG5tb2R1bGUuZXhwb3J0cyA9IE5hdkxlZnRCYXI7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9uYXZMZWZ0QmFyLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgJCA9IHJlcXVpcmUoJ2pRdWVyeScpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHthY3Rpb25zLCBnZXR0ZXJzfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2ludml0ZScpO1xudmFyIHVzZXJNb2R1bGUgPSByZXF1aXJlKCdhcHAvbW9kdWxlcy91c2VyJyk7XG52YXIgTGlua2VkU3RhdGVNaXhpbiA9IHJlcXVpcmUoJ3JlYWN0LWFkZG9ucy1saW5rZWQtc3RhdGUtbWl4aW4nKTtcbnZhciBHb29nbGVBdXRoSW5mbyA9IHJlcXVpcmUoJy4vZ29vZ2xlQXV0aExvZ28nKTtcbnZhciB7RXhwaXJlZEludml0ZX0gPSByZXF1aXJlKCcuL2Vycm9yUGFnZScpO1xuXG52YXIgSW52aXRlSW5wdXRGb3JtID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW0xpbmtlZFN0YXRlTWl4aW5dLFxuXG4gIGNvbXBvbmVudERpZE1vdW50KCl7XG4gICAgJCh0aGlzLnJlZnMuZm9ybSkudmFsaWRhdGUoe1xuICAgICAgcnVsZXM6e1xuICAgICAgICBwYXNzd29yZDp7XG4gICAgICAgICAgbWlubGVuZ3RoOiA2LFxuICAgICAgICAgIHJlcXVpcmVkOiB0cnVlXG4gICAgICAgIH0sXG4gICAgICAgIHBhc3N3b3JkQ29uZmlybWVkOntcbiAgICAgICAgICByZXF1aXJlZDogdHJ1ZSxcbiAgICAgICAgICBlcXVhbFRvOiB0aGlzLnJlZnMucGFzc3dvcmRcbiAgICAgICAgfVxuICAgICAgfSxcblxuICAgICAgbWVzc2FnZXM6IHtcbiAgXHRcdFx0cGFzc3dvcmRDb25maXJtZWQ6IHtcbiAgXHRcdFx0XHRtaW5sZW5ndGg6ICQudmFsaWRhdG9yLmZvcm1hdCgnRW50ZXIgYXQgbGVhc3QgezB9IGNoYXJhY3RlcnMnKSxcbiAgXHRcdFx0XHRlcXVhbFRvOiAnRW50ZXIgdGhlIHNhbWUgcGFzc3dvcmQgYXMgYWJvdmUnXG4gIFx0XHRcdH1cbiAgICAgIH1cbiAgICB9KVxuICB9LFxuXG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4ge1xuICAgICAgbmFtZTogdGhpcy5wcm9wcy5pbnZpdGUudXNlcixcbiAgICAgIHBzdzogJycsXG4gICAgICBwc3dDb25maXJtZWQ6ICcnLFxuICAgICAgdG9rZW46ICcnXG4gICAgfVxuICB9LFxuXG4gIG9uQ2xpY2soZSkge1xuICAgIGUucHJldmVudERlZmF1bHQoKTtcbiAgICBpZiAodGhpcy5pc1ZhbGlkKCkpIHtcbiAgICAgIHVzZXJNb2R1bGUuYWN0aW9ucy5zaWduVXAoe1xuICAgICAgICBuYW1lOiB0aGlzLnN0YXRlLm5hbWUsXG4gICAgICAgIHBzdzogdGhpcy5zdGF0ZS5wc3csXG4gICAgICAgIHRva2VuOiB0aGlzLnN0YXRlLnRva2VuLFxuICAgICAgICBpbnZpdGVUb2tlbjogdGhpcy5wcm9wcy5pbnZpdGUuaW52aXRlX3Rva2VufSk7XG4gICAgfVxuICB9LFxuXG4gIGlzVmFsaWQoKSB7XG4gICAgdmFyICRmb3JtID0gJCh0aGlzLnJlZnMuZm9ybSk7XG4gICAgcmV0dXJuICRmb3JtLmxlbmd0aCA9PT0gMCB8fCAkZm9ybS52YWxpZCgpO1xuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICBsZXQge2lzUHJvY2Vzc2luZywgaXNGYWlsZWQsIG1lc3NhZ2UgfSA9IHRoaXMucHJvcHMuYXR0ZW1wO1xuICAgIHJldHVybiAoXG4gICAgICA8Zm9ybSByZWY9XCJmb3JtXCIgY2xhc3NOYW1lPVwiZ3J2LWludml0ZS1pbnB1dC1mb3JtXCI+XG4gICAgICAgIDxoMz4gR2V0IHN0YXJ0ZWQgd2l0aCBUZWxlcG9ydCA8L2gzPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0XG4gICAgICAgICAgICAgIHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ25hbWUnKX1cbiAgICAgICAgICAgICAgbmFtZT1cInVzZXJOYW1lXCJcbiAgICAgICAgICAgICAgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sIHJlcXVpcmVkXCJcbiAgICAgICAgICAgICAgcGxhY2Vob2xkZXI9XCJVc2VyIG5hbWVcIi8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXRcbiAgICAgICAgICAgICAgYXV0b0ZvY3VzXG4gICAgICAgICAgICAgIHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3BzdycpfVxuICAgICAgICAgICAgICByZWY9XCJwYXNzd29yZFwiXG4gICAgICAgICAgICAgIHR5cGU9XCJwYXNzd29yZFwiXG4gICAgICAgICAgICAgIG5hbWU9XCJwYXNzd29yZFwiXG4gICAgICAgICAgICAgIGNsYXNzTmFtZT1cImZvcm0tY29udHJvbFwiXG4gICAgICAgICAgICAgIHBsYWNlaG9sZGVyPVwiUGFzc3dvcmRcIiAvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0XG4gICAgICAgICAgICAgIHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3Bzd0NvbmZpcm1lZCcpfVxuICAgICAgICAgICAgICB0eXBlPVwicGFzc3dvcmRcIlxuICAgICAgICAgICAgICBuYW1lPVwicGFzc3dvcmRDb25maXJtZWRcIlxuICAgICAgICAgICAgICBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2xcIlxuICAgICAgICAgICAgICBwbGFjZWhvbGRlcj1cIlBhc3N3b3JkIGNvbmZpcm1cIi8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXRcbiAgICAgICAgICAgICAgbmFtZT1cInRva2VuXCJcbiAgICAgICAgICAgICAgdmFsdWVMaW5rPXt0aGlzLmxpbmtTdGF0ZSgndG9rZW4nKX1cbiAgICAgICAgICAgICAgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sIHJlcXVpcmVkXCJcbiAgICAgICAgICAgICAgcGxhY2Vob2xkZXI9XCJUd28gZmFjdG9yIHRva2VuIChHb29nbGUgQXV0aGVudGljYXRvcilcIiAvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxidXR0b24gdHlwZT1cInN1Ym1pdFwiIGRpc2FibGVkPXtpc1Byb2Nlc3Npbmd9IGNsYXNzTmFtZT1cImJ0biBidG4tcHJpbWFyeSBibG9jayBmdWxsLXdpZHRoIG0tYlwiIG9uQ2xpY2s9e3RoaXMub25DbGlja30gPlNpZ24gdXA8L2J1dHRvbj5cbiAgICAgICAgICB7IGlzRmFpbGVkID8gKDxsYWJlbCBjbGFzc05hbWU9XCJlcnJvclwiPnttZXNzYWdlfTwvbGFiZWw+KSA6IG51bGwgfVxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZm9ybT5cbiAgICApO1xuICB9XG59KVxuXG52YXIgSW52aXRlID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW3JlYWN0b3IuUmVhY3RNaXhpbl0sXG5cbiAgZ2V0RGF0YUJpbmRpbmdzKCkge1xuICAgIHJldHVybiB7XG4gICAgICBpbnZpdGU6IGdldHRlcnMuaW52aXRlLFxuICAgICAgYXR0ZW1wOiBnZXR0ZXJzLmF0dGVtcCxcbiAgICAgIGZldGNoaW5nSW52aXRlOiBnZXR0ZXJzLmZldGNoaW5nSW52aXRlXG4gICAgfVxuICB9LFxuXG4gIGNvbXBvbmVudERpZE1vdW50KCl7XG4gICAgYWN0aW9ucy5mZXRjaEludml0ZSh0aGlzLnByb3BzLnBhcmFtcy5pbnZpdGVUb2tlbik7XG4gIH0sXG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICBsZXQge2ZldGNoaW5nSW52aXRlLCBpbnZpdGUsIGF0dGVtcH0gPSB0aGlzLnN0YXRlO1xuXG4gICAgaWYoZmV0Y2hpbmdJbnZpdGUuaXNGYWlsZWQpe1xuICAgICAgcmV0dXJuIDxFeHBpcmVkSW52aXRlLz5cbiAgICB9XG5cbiAgICBpZighaW52aXRlKSB7XG4gICAgICByZXR1cm4gbnVsbDtcbiAgICB9XG5cbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtaW52aXRlIHRleHQtY2VudGVyXCI+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWxvZ28tdHBydFwiPjwvZGl2PlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1jb250ZW50IGdydi1mbGV4XCI+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZmxleC1jb2x1bW5cIj5cbiAgICAgICAgICAgIDxJbnZpdGVJbnB1dEZvcm0gYXR0ZW1wPXthdHRlbXB9IGludml0ZT17aW52aXRlLnRvSlMoKX0vPlxuICAgICAgICAgICAgPEdvb2dsZUF1dGhJbmZvLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LWNvbHVtbiBncnYtaW52aXRlLWJhcmNvZGVcIj5cbiAgICAgICAgICAgIDxoND5TY2FuIGJhciBjb2RlIGZvciBhdXRoIHRva2VuIDxici8+IDxzbWFsbD5TY2FuIGJlbG93IHRvIGdlbmVyYXRlIHlvdXIgdHdvIGZhY3RvciB0b2tlbjwvc21hbGw+PC9oND5cbiAgICAgICAgICAgIDxpbWcgY2xhc3NOYW1lPVwiaW1nLXRodW1ibmFpbFwiIHNyYz17IGBkYXRhOmltYWdlL3BuZztiYXNlNjQsJHtpbnZpdGUuZ2V0KCdxcicpfWAgfSAvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IEludml0ZTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL25ld1VzZXIuanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB1c2VyR2V0dGVycyA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3VzZXIvZ2V0dGVycycpO1xudmFyIG5vZGVHZXR0ZXJzID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvbm9kZXMvZ2V0dGVycycpO1xudmFyIE5vZGVMaXN0ID0gcmVxdWlyZSgnLi9ub2RlTGlzdC5qc3gnKTtcblxudmFyIE5vZGVzID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW3JlYWN0b3IuUmVhY3RNaXhpbl0sXG5cbiAgZ2V0RGF0YUJpbmRpbmdzKCkge1xuICAgIHJldHVybiB7XG4gICAgICBub2RlUmVjb3Jkczogbm9kZUdldHRlcnMubm9kZUxpc3RWaWV3LFxuICAgICAgdXNlcjogdXNlckdldHRlcnMudXNlclxuICAgIH1cbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIHZhciBub2RlUmVjb3JkcyA9IHRoaXMuc3RhdGUubm9kZVJlY29yZHM7XG4gICAgdmFyIGxvZ2lucyA9IHRoaXMuc3RhdGUudXNlci5sb2dpbnM7XG4gICAgcmV0dXJuICggPE5vZGVMaXN0IG5vZGVSZWNvcmRzPXtub2RlUmVjb3Jkc30gbG9naW5zPXtsb2dpbnN9Lz4gKTtcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gTm9kZXM7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9ub2Rlcy9tYWluLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIge0RhdGVSYW5nZVBpY2tlciwgQ2FsZW5kYXJOYXZ9ID0gcmVxdWlyZSgnLi8uLi9kYXRlUGlja2VyLmpzeCcpO1xudmFyIHtUYWJsZSwgQ29sdW1uLCBDZWxsLCBUZXh0Q2VsbCwgU29ydEhlYWRlckNlbGwsIFNvcnRUeXBlc30gPSByZXF1aXJlKCdhcHAvY29tcG9uZW50cy90YWJsZS5qc3gnKTtcbnZhciB7QnV0dG9uQ2VsbCwgVXNlcnNDZWxsLCBFbXB0eUxpc3QsIE5vZGVDZWxsLCBEdXJhdGlvbkNlbGwsIERhdGVDcmVhdGVkQ2VsbH0gPSByZXF1aXJlKCcuL2xpc3RJdGVtcycpO1xuXG52YXIgQWN0aXZlU2Vzc2lvbkxpc3QgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgbGV0IGRhdGEgPSB0aGlzLnByb3BzLmRhdGEuZmlsdGVyKGl0ZW0gPT4gaXRlbS5hY3RpdmUpO1xuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1zZXNzaW9ucy1hY3RpdmVcIj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtaGVhZGVyXCI+XG4gICAgICAgICAgPGgxPiBBY3RpdmUgU2Vzc2lvbnMgPC9oMT5cbiAgICAgICAgPC9kaXY+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWNvbnRlbnRcIj5cbiAgICAgICAgICB7ZGF0YS5sZW5ndGggPT09IDAgPyA8RW1wdHlMaXN0IHRleHQ9XCJZb3UgaGF2ZSBubyBhY3RpdmUgc2Vzc2lvbnMuXCIvPiA6XG4gICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgICAgICA8VGFibGUgcm93Q291bnQ9e2RhdGEubGVuZ3RofSBjbGFzc05hbWU9XCJ0YWJsZS1zdHJpcGVkXCI+XG4gICAgICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICAgICAgY29sdW1uS2V5PVwic2lkXCJcbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+IFNlc3Npb24gSUQgPC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXs8VGV4dENlbGwgZGF0YT17ZGF0YX0vPiB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPiA8L0NlbGw+IH1cbiAgICAgICAgICAgICAgICAgIGNlbGw9e1xuICAgICAgICAgICAgICAgICAgICA8QnV0dG9uQ2VsbCBkYXRhPXtkYXRhfSAvPlxuICAgICAgICAgICAgICAgICAgfVxuICAgICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD4gTm9kZSA8L0NlbGw+IH1cbiAgICAgICAgICAgICAgICAgIGNlbGw9ezxOb2RlQ2VsbCBkYXRhPXtkYXRhfSAvPiB9XG4gICAgICAgICAgICAgICAgLz4gICAgICAgICAgICAgICAgXG4gICAgICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICAgICAgY29sdW1uS2V5PVwiY3JlYXRlZFwiXG4gICAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPiBDcmVhdGVkIDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17PERhdGVDcmVhdGVkQ2VsbCBkYXRhPXtkYXRhfS8+IH1cbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+IFVzZXJzIDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17PFVzZXJzQ2VsbCBkYXRhPXtkYXRhfSAvPiB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgPC9UYWJsZT5cbiAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIH1cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApXG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IEFjdGl2ZVNlc3Npb25MaXN0O1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvc2Vzc2lvbnMvYWN0aXZlU2Vzc2lvbkxpc3QuanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7Z2V0dGVycywgYWN0aW9uc30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9zZXNzaW9ucycpO1xudmFyIFN0b3JlZFNlc3Npb25MaXN0ID0gcmVxdWlyZSgnLi9zdG9yZWRTZXNzaW9uTGlzdC5qc3gnKTtcbnZhciBBY3RpdmVTZXNzaW9uTGlzdCA9IHJlcXVpcmUoJy4vYWN0aXZlU2Vzc2lvbkxpc3QuanN4Jyk7XG5cbnZhciBTZXNzaW9ucyA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtkYXRhOiBnZXR0ZXJzLnNlc3Npb25zVmlld31cbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIGxldCB7ZGF0YX0gPSB0aGlzLnN0YXRlO1xuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1zZXNzaW9ucyBncnYtcGFnZVwiPlxuICAgICAgICA8QWN0aXZlU2Vzc2lvbkxpc3QgZGF0YT17ZGF0YX0vPlxuICAgICAgICA8aHIgY2xhc3NOYW1lPVwiZ3J2LWRpdmlkZXJcIi8+XG4gICAgICAgIDxTdG9yZWRTZXNzaW9uTGlzdCBkYXRhPXtkYXRhfS8+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBTZXNzaW9ucztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL3Nlc3Npb25zL21haW4uanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7IGFjdGlvbnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvc2Vzc2lvbnMnKTtcbnZhciBMaW5rZWRTdGF0ZU1peGluID0gcmVxdWlyZSgncmVhY3QtYWRkb25zLWxpbmtlZC1zdGF0ZS1taXhpbicpO1xudmFyIHtUYWJsZSwgQ29sdW1uLCBDZWxsLCBUZXh0Q2VsbCwgU29ydEhlYWRlckNlbGwsIFNvcnRUeXBlc30gPSByZXF1aXJlKCdhcHAvY29tcG9uZW50cy90YWJsZS5qc3gnKTtcbnZhciB7QnV0dG9uQ2VsbCwgU2luZ2xlVXNlckNlbGwsIFVzZXJzQ2VsbCwgRW1wdHlMaXN0LCBEdXJhdGlvbkNlbGwsIERhdGVDcmVhdGVkQ2VsbH0gPSByZXF1aXJlKCcuL2xpc3RJdGVtcycpO1xudmFyIHtEYXRlUmFuZ2VQaWNrZXIsIENhbGVuZGFyTmF2fSA9IHJlcXVpcmUoJy4vLi4vZGF0ZVBpY2tlci5qc3gnKTtcbnZhciBtb21lbnQgPSAgcmVxdWlyZSgnbW9tZW50Jyk7XG52YXIge21vbnRoUmFuZ2V9ID0gcmVxdWlyZSgnYXBwL2NvbW1vbi9kYXRlVXRpbHMnKTtcbnZhciB7aXNNYXRjaH0gPSByZXF1aXJlKCdhcHAvY29tbW9uL29iamVjdFV0aWxzJyk7XG52YXIgXyA9IHJlcXVpcmUoJ18nKTtcblxudmFyIEFyY2hpdmVkU2Vzc2lvbnMgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbTGlua2VkU3RhdGVNaXhpbl0sXG5cbiAgZ2V0SW5pdGlhbFN0YXRlKHByb3BzKXtcbiAgICBsZXQgW3N0YXJ0RGF0ZSwgZW5kRGF0ZV0gPSBtb250aFJhbmdlKG5ldyBEYXRlKCkpO1xuICAgIHRoaXMuc2VhcmNoYWJsZVByb3BzID0gWydzZXJ2ZXJJcCcsICdjcmVhdGVkJywgJ3NpZCcsICdsb2dpbiddO1xuICAgIHJldHVybiB7IGZpbHRlcjogJycsIGNvbFNvcnREaXJzOiB7Y3JlYXRlZDogJ0FTQyd9LCBzdGFydERhdGUsIGVuZERhdGUgfTtcbiAgfSxcblxuICBjb21wb25lbnRXaWxsTW91bnQoKXtcbiAgICBhY3Rpb25zLmZldGNoU2Vzc2lvbnModGhpcy5zdGF0ZS5zdGFydERhdGUsIHRoaXMuc3RhdGUuZW5kRGF0ZSk7XG4gIH0sXG5cbiAgc2V0RGF0ZXNBbmRSZWZldGNoKHN0YXJ0RGF0ZSwgZW5kRGF0ZSl7XG4gICAgYWN0aW9ucy5mZXRjaFNlc3Npb25zKHN0YXJ0RGF0ZSwgZW5kRGF0ZSk7XG4gICAgdGhpcy5zdGF0ZS5zdGFydERhdGUgPSBzdGFydERhdGU7XG4gICAgdGhpcy5zdGF0ZS5lbmREYXRlID0gZW5kRGF0ZTtcbiAgICB0aGlzLnNldFN0YXRlKHRoaXMuc3RhdGUpO1xuICB9LFxuXG4gIG9uU29ydENoYW5nZShjb2x1bW5LZXksIHNvcnREaXIpIHtcbiAgICB0aGlzLnNldFN0YXRlKHtcbiAgICAgIC4uLnRoaXMuc3RhdGUsXG4gICAgICBjb2xTb3J0RGlyczogeyBbY29sdW1uS2V5XTogc29ydERpciB9XG4gICAgfSk7XG4gIH0sXG5cbiAgb25SYW5nZVBpY2tlckNoYW5nZSh7c3RhcnREYXRlLCBlbmREYXRlfSl7XG4gICAgdGhpcy5zZXREYXRlc0FuZFJlZmV0Y2goc3RhcnREYXRlLCBlbmREYXRlKTtcbiAgfSxcblxuICBvbkNhbGVuZGFyTmF2Q2hhbmdlKG5ld1ZhbHVlKXtcbiAgICBsZXQgW3N0YXJ0RGF0ZSwgZW5kRGF0ZV0gPSBtb250aFJhbmdlKG5ld1ZhbHVlKTtcbiAgICB0aGlzLnNldERhdGVzQW5kUmVmZXRjaChzdGFydERhdGUsIGVuZERhdGUpO1xuICB9LFxuXG4gIHNlYXJjaEFuZEZpbHRlckNiKHRhcmdldFZhbHVlLCBzZWFyY2hWYWx1ZSwgcHJvcE5hbWUpe1xuICAgIGlmKHByb3BOYW1lID09PSAnY3JlYXRlZCcpe1xuICAgICAgdmFyIGRpc3BsYXlEYXRlID0gbW9tZW50KHRhcmdldFZhbHVlKS5mb3JtYXQoJ2wgTFRTJykudG9Mb2NhbGVVcHBlckNhc2UoKTtcbiAgICAgIHJldHVybiBkaXNwbGF5RGF0ZS5pbmRleE9mKHNlYXJjaFZhbHVlKSAhPT0gLTE7XG4gICAgfVxuICB9LFxuXG4gIHNvcnRBbmRGaWx0ZXIoZGF0YSl7XG4gICAgdmFyIGZpbHRlcmVkID0gZGF0YS5maWx0ZXIob2JqPT5cbiAgICAgIGlzTWF0Y2gob2JqLCB0aGlzLnN0YXRlLmZpbHRlciwge1xuICAgICAgICBzZWFyY2hhYmxlUHJvcHM6IHRoaXMuc2VhcmNoYWJsZVByb3BzLFxuICAgICAgICBjYjogdGhpcy5zZWFyY2hBbmRGaWx0ZXJDYlxuICAgICAgfSkpO1xuXG4gICAgdmFyIGNvbHVtbktleSA9IE9iamVjdC5nZXRPd25Qcm9wZXJ0eU5hbWVzKHRoaXMuc3RhdGUuY29sU29ydERpcnMpWzBdO1xuICAgIHZhciBzb3J0RGlyID0gdGhpcy5zdGF0ZS5jb2xTb3J0RGlyc1tjb2x1bW5LZXldO1xuICAgIHZhciBzb3J0ZWQgPSBfLnNvcnRCeShmaWx0ZXJlZCwgY29sdW1uS2V5KTtcbiAgICBpZihzb3J0RGlyID09PSBTb3J0VHlwZXMuQVNDKXtcbiAgICAgIHNvcnRlZCA9IHNvcnRlZC5yZXZlcnNlKCk7XG4gICAgfVxuXG4gICAgcmV0dXJuIHNvcnRlZDtcbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIGxldCB7c3RhcnREYXRlLCBlbmREYXRlfSA9IHRoaXMuc3RhdGU7XG4gICAgbGV0IGRhdGEgPSB0aGlzLnByb3BzLmRhdGEuZmlsdGVyKGl0ZW0gPT4gIWl0ZW0uYWN0aXZlICYmIG1vbWVudChpdGVtLmNyZWF0ZWQpLmlzQmV0d2VlbihzdGFydERhdGUsIGVuZERhdGUpKTtcbiAgICBkYXRhID0gdGhpcy5zb3J0QW5kRmlsdGVyKGRhdGEpO1xuXG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXNlc3Npb25zLXN0b3JlZFwiPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1oZWFkZXJcIj5cbiAgICAgICAgICA8aDE+IEFyY2hpdmVkIFNlc3Npb25zIDwvaDE+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZmxleFwiPlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZmxleC1yb3dcIj5cbiAgICAgICAgICAgICAgPERhdGVSYW5nZVBpY2tlciBzdGFydERhdGU9e3N0YXJ0RGF0ZX0gZW5kRGF0ZT17ZW5kRGF0ZX0gb25DaGFuZ2U9e3RoaXMub25SYW5nZVBpY2tlckNoYW5nZX0vPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LXJvd1wiPlxuICAgICAgICAgICAgICA8Q2FsZW5kYXJOYXYgdmFsdWU9e3N0YXJ0RGF0ZX0gb25WYWx1ZUNoYW5nZT17dGhpcy5vbkNhbGVuZGFyTmF2Q2hhbmdlfS8+XG4gICAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWZsZXgtcm93XCI+XG4gICAgICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXNlYXJjaFwiPlxuICAgICAgICAgICAgICAgIDxpbnB1dCB2YWx1ZUxpbms9e3RoaXMubGlua1N0YXRlKCdmaWx0ZXInKX0gcGxhY2Vob2xkZXI9XCJTZWFyY2guLi5cIiBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2wgaW5wdXQtc21cIi8+XG4gICAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgIDwvZGl2PlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1jb250ZW50XCI+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICAgIDxUYWJsZSByb3dDb3VudD17ZGF0YS5sZW5ndGh9IGNsYXNzTmFtZT1cInRhYmxlLXN0cmlwZWRcIj5cbiAgICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICAgIGNvbHVtbktleT1cInNpZFwiXG4gICAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD4gU2Vzc2lvbiBJRCA8L0NlbGw+IH1cbiAgICAgICAgICAgICAgICBjZWxsPXs8VGV4dENlbGwgZGF0YT17ZGF0YX0vPiB9XG4gICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPiA8L0NlbGw+IH1cbiAgICAgICAgICAgICAgICBjZWxsPXtcbiAgICAgICAgICAgICAgICAgIDxCdXR0b25DZWxsIGRhdGE9e2RhdGF9IC8+XG4gICAgICAgICAgICAgICAgfVxuICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgY29sdW1uS2V5PVwiY3JlYXRlZFwiXG4gICAgICAgICAgICAgICAgaGVhZGVyPXtcbiAgICAgICAgICAgICAgICAgIDxTb3J0SGVhZGVyQ2VsbFxuICAgICAgICAgICAgICAgICAgICBzb3J0RGlyPXt0aGlzLnN0YXRlLmNvbFNvcnREaXJzLmNyZWF0ZWR9XG4gICAgICAgICAgICAgICAgICAgIG9uU29ydENoYW5nZT17dGhpcy5vblNvcnRDaGFuZ2V9XG4gICAgICAgICAgICAgICAgICAgIHRpdGxlPVwiQ3JlYXRlZFwiXG4gICAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICAgIH1cbiAgICAgICAgICAgICAgICBjZWxsPXs8RGF0ZUNyZWF0ZWRDZWxsIGRhdGE9e2RhdGF9Lz4gfVxuICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD4gVXNlciA8L0NlbGw+IH1cbiAgICAgICAgICAgICAgICBjZWxsPXs8U2luZ2xlVXNlckNlbGwgZGF0YT17ZGF0YX0vPiB9XG4gICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICA8L1RhYmxlPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgIClcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gQXJjaGl2ZWRTZXNzaW9ucztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL3Nlc3Npb25zL3N0b3JlZFNlc3Npb25MaXN0LmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgcmVuZGVyID0gcmVxdWlyZSgncmVhY3QtZG9tJykucmVuZGVyO1xudmFyIHsgUm91dGVyLCBSb3V0ZSwgUmVkaXJlY3QsIEluZGV4Um91dGUsIGJyb3dzZXJIaXN0b3J5IH0gPSByZXF1aXJlKCdyZWFjdC1yb3V0ZXInKTtcbnZhciB7IEFwcCwgTG9naW4sIE5vZGVzLCBTZXNzaW9ucywgTmV3VXNlciwgQ3VycmVudFNlc3Npb25Ib3N0LCBOb3RGb3VuZCB9ID0gcmVxdWlyZSgnLi9jb21wb25lbnRzJyk7XG52YXIge2Vuc3VyZVVzZXJ9ID0gcmVxdWlyZSgnLi9tb2R1bGVzL3VzZXIvYWN0aW9ucycpO1xudmFyIGF1dGggPSByZXF1aXJlKCcuL2F1dGgnKTtcbnZhciBzZXNzaW9uID0gcmVxdWlyZSgnLi9zZXNzaW9uJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnLi9jb25maWcnKTtcblxucmVxdWlyZSgnLi9tb2R1bGVzJyk7XG5cbi8vIGluaXQgc2Vzc2lvblxuc2Vzc2lvbi5pbml0KCk7XG5cbmZ1bmN0aW9uIGhhbmRsZUxvZ291dChuZXh0U3RhdGUsIHJlcGxhY2UsIGNiKXtcbiAgYXV0aC5sb2dvdXQoKTtcbn1cblxucmVuZGVyKChcbiAgPFJvdXRlciBoaXN0b3J5PXtzZXNzaW9uLmdldEhpc3RvcnkoKX0+XG4gICAgPFJvdXRlIHBhdGg9e2NmZy5yb3V0ZXMubG9naW59IGNvbXBvbmVudD17TG9naW59Lz5cbiAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5sb2dvdXR9IG9uRW50ZXI9e2hhbmRsZUxvZ291dH0vPlxuICAgIDxSb3V0ZSBwYXRoPXtjZmcucm91dGVzLm5ld1VzZXJ9IGNvbXBvbmVudD17TmV3VXNlcn0vPlxuICAgIDxSZWRpcmVjdCBmcm9tPXtjZmcucm91dGVzLmFwcH0gdG89e2NmZy5yb3V0ZXMubm9kZXN9Lz5cbiAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5hcHB9IGNvbXBvbmVudD17QXBwfSBvbkVudGVyPXtlbnN1cmVVc2VyfSA+XG4gICAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5ub2Rlc30gY29tcG9uZW50PXtOb2Rlc30vPlxuICAgICAgPFJvdXRlIHBhdGg9e2NmZy5yb3V0ZXMuYWN0aXZlU2Vzc2lvbn0gY29tcG9uZW50cz17e0N1cnJlbnRTZXNzaW9uSG9zdDogQ3VycmVudFNlc3Npb25Ib3N0fX0vPlxuICAgICAgPFJvdXRlIHBhdGg9e2NmZy5yb3V0ZXMuc2Vzc2lvbnN9IGNvbXBvbmVudD17U2Vzc2lvbnN9Lz5cbiAgICA8L1JvdXRlPlxuICAgIDxSb3V0ZSBwYXRoPVwiKlwiIGNvbXBvbmVudD17Tm90Rm91bmR9IC8+XG4gIDwvUm91dGVyPlxuKSwgZG9jdW1lbnQuZ2V0RWxlbWVudEJ5SWQoXCJhcHBcIikpO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2luZGV4LmpzeFxuICoqLyIsIm1vZHVsZS5leHBvcnRzID0gVGVybWluYWw7XG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiBleHRlcm5hbCBcIlRlcm1pbmFsXCJcbiAqKiBtb2R1bGUgaWQgPSA0MTNcbiAqKiBtb2R1bGUgY2h1bmtzID0gMVxuICoqLyJdLCJzb3VyY2VSb290IjoiIn0=