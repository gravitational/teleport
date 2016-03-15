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
	
	module.exports.getters = __webpack_require__(63);
	module.exports.actions = __webpack_require__(62);
	module.exports.activeTermStore = __webpack_require__(99);

/***/ },

/***/ 45:
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
	
	var LinkedStateMixin = __webpack_require__(40);
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
	var nodeGetters = __webpack_require__(45);
	
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
	
	var _require = __webpack_require__(41);
	
	var Link = _require.Link;
	
	var _require2 = __webpack_require__(47);
	
	var actions = _require2.actions;
	
	var _require3 = __webpack_require__(45);
	
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
	
	module.exports.getters = __webpack_require__(45);
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
	
	var _require = __webpack_require__(44);
	
	var getters = _require.getters;
	var actions = _require.actions;
	
	var Tty = __webpack_require__(97);
	var TtyTerminal = __webpack_require__(229);
	var EventStreamer = __webpack_require__(302);
	var SessionLeftPanel = __webpack_require__(222);
	
	var _require2 = __webpack_require__(64);
	
	var showSelectNodeDialog = _require2.showSelectNodeDialog;
	var closeSelectNodeDialog = _require2.closeSelectNodeDialog;
	
	var SelectNodeDialog = __webpack_require__(227);
	
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
	
	var _require = __webpack_require__(44);
	
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
	var LinkedStateMixin = __webpack_require__(40);
	
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
	
	var _require = __webpack_require__(41);
	
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
	var LinkedStateMixin = __webpack_require__(40);
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
	var nodeGetters = __webpack_require__(45);
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
	
	var LinkedStateMixin = __webpack_require__(40);
	
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
	
	var _require = __webpack_require__(41);
	
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
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJzb3VyY2VzIjpbIndlYnBhY2s6Ly8vLi9zcmMvYXBwL3JlYWN0b3IuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb25maWcuanMiLCJ3ZWJwYWNrOi8vLy4vfi9rZXltaXJyb3IvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9zZXNzaW9uLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvc2VydmljZXMvYXBpLmpzIiwid2VicGFjazovLy9leHRlcm5hbCBcImpRdWVyeVwiIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9jb25zdGFudHMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy91c2VyL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL3RhYmxlLmpzeCIsIndlYnBhY2s6Ly8vZXh0ZXJuYWwgXCJfXCIiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2F1dGguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21tb24vZGF0ZVV0aWxzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tbW9uL29iamVjdFV0aWxzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tbW9uL3R0eS5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvYWN0aW9uVHlwZXMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGl2ZVRlcm1TdG9yZS5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvYXBwL2FjdGlvblR5cGVzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hcHAvYXBwU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvYWN0aW9uVHlwZXMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvZGlhbG9nU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2ludml0ZVN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvbm9kZVN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2FjdGlvblR5cGVzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvYWN0aW9uVHlwZXMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL3Nlc3Npb25TdG9yZS5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvdXNlci9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvdXNlci9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy91c2VyL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy91c2VyL3VzZXJTdG9yZS5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvY3VycmVudFNlc3Npb24vc2Vzc2lvbkxlZnRQYW5lbC5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2RhdGVQaWNrZXIuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9lcnJvclBhZ2UuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9nb29nbGVBdXRoTG9nby5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL25vZGVzL25vZGVMaXN0LmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvc2VsZWN0Tm9kZURpYWxvZy5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL3Nlc3Npb25zL2xpc3RJdGVtcy5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL3Rlcm1pbmFsLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbW1vbi9wYXR0ZXJuVXRpbHMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21tb24vdHR5UGxheWVyLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hcHAvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvYXBwL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FwcC9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvZGlhbG9ncy9nZXR0ZXJzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9kaWFsb2dzL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9nZXR0ZXJzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL3Jlc3RBcGlTdG9yZS5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL3V0aWxzLmpzIiwid2VicGFjazovLy8uL34vZXZlbnRzL2V2ZW50cy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvYXBwLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvY3VycmVudFNlc3Npb24vYWN0aXZlU2Vzc2lvbi5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2N1cnJlbnRTZXNzaW9uL2V2ZW50U3RyZWFtZXIuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9jdXJyZW50U2Vzc2lvbi9tYWluLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvY3VycmVudFNlc3Npb24vc2Vzc2lvblBsYXllci5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2luZGV4LmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvbG9naW4uanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9uYXZMZWZ0QmFyLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvbmV3VXNlci5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL25vZGVzL21haW4uanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9zZXNzaW9ucy9hY3RpdmVTZXNzaW9uTGlzdC5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL3Nlc3Npb25zL21haW4uanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9zZXNzaW9ucy9zdG9yZWRTZXNzaW9uTGlzdC5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9pbmRleC5qc3giLCJ3ZWJwYWNrOi8vL2V4dGVybmFsIFwiVGVybWluYWxcIiJdLCJuYW1lcyI6W10sIm1hcHBpbmdzIjoiOzs7Ozs7Ozs7Ozs7Ozs7OztzQ0FBd0IsRUFBWTs7QUFFcEMsS0FBTSxPQUFPLEdBQUcsdUJBQVk7QUFDMUIsUUFBSyxFQUFFLElBQUk7RUFDWixDQUFDOztBQUVGLE9BQU0sQ0FBQyxPQUFPLEdBQUcsT0FBTyxDQUFDOztzQkFFVixPQUFPOzs7Ozs7Ozs7Ozs7Z0JDUkEsbUJBQU8sQ0FBQyxHQUF5QixDQUFDOztLQUFuRCxhQUFhLFlBQWIsYUFBYTs7QUFFbEIsS0FBSSxHQUFHLEdBQUc7O0FBRVIsVUFBTyxFQUFFLE1BQU0sQ0FBQyxRQUFRLENBQUMsTUFBTTs7QUFFL0IsVUFBTyxFQUFFLGlFQUFpRTs7QUFFMUUsTUFBRyxFQUFFO0FBQ0gsbUJBQWMsRUFBQywyQkFBMkI7QUFDMUMsY0FBUyxFQUFFLGtDQUFrQztBQUM3QyxnQkFBVyxFQUFFLHFCQUFxQjtBQUNsQyxvQkFBZSxFQUFFLDBDQUEwQztBQUMzRCxlQUFVLEVBQUUsdUNBQXVDO0FBQ25ELG1CQUFjLEVBQUUsa0JBQWtCO0FBQ2xDLGlCQUFZLEVBQUUsdUVBQXVFO0FBQ3JGLDBCQUFxQixFQUFFLHNEQUFzRDs7QUFFN0UsNEJBQXVCLEVBQUUsaUNBQUMsSUFBaUIsRUFBRztXQUFuQixHQUFHLEdBQUosSUFBaUIsQ0FBaEIsR0FBRztXQUFFLEtBQUssR0FBWCxJQUFpQixDQUFYLEtBQUs7V0FBRSxHQUFHLEdBQWhCLElBQWlCLENBQUosR0FBRzs7QUFDeEMsY0FBTyxhQUFhLENBQUMsR0FBRyxDQUFDLEdBQUcsQ0FBQyxZQUFZLEVBQUUsRUFBQyxHQUFHLEVBQUgsR0FBRyxFQUFFLEtBQUssRUFBTCxLQUFLLEVBQUUsR0FBRyxFQUFILEdBQUcsRUFBQyxDQUFDLENBQUM7TUFDL0Q7O0FBRUQsNkJBQXdCLEVBQUUsa0NBQUMsR0FBRyxFQUFHO0FBQy9CLGNBQU8sYUFBYSxDQUFDLEdBQUcsQ0FBQyxHQUFHLENBQUMscUJBQXFCLEVBQUUsRUFBQyxHQUFHLEVBQUgsR0FBRyxFQUFDLENBQUMsQ0FBQztNQUM1RDs7QUFFRCx3QkFBbUIsRUFBRSw2QkFBQyxLQUFLLEVBQUUsR0FBRyxFQUFHO0FBQ2pDLFdBQUksTUFBTSxHQUFHO0FBQ1gsY0FBSyxFQUFFLEtBQUssQ0FBQyxXQUFXLEVBQUU7QUFDMUIsWUFBRyxFQUFFLEdBQUcsQ0FBQyxXQUFXLEVBQUU7UUFDdkIsQ0FBQzs7QUFFRixXQUFJLElBQUksR0FBRyxJQUFJLENBQUMsU0FBUyxDQUFDLE1BQU0sQ0FBQyxDQUFDO0FBQ2xDLFdBQUksV0FBVyxHQUFHLE1BQU0sQ0FBQyxTQUFTLENBQUMsSUFBSSxDQUFDLENBQUM7O0FBRXpDLHFFQUE0RCxXQUFXLENBQUc7TUFDM0U7O0FBRUQsdUJBQWtCLEVBQUUsNEJBQUMsR0FBRyxFQUFHO0FBQ3pCLGNBQU8sYUFBYSxDQUFDLEdBQUcsQ0FBQyxHQUFHLENBQUMsZUFBZSxFQUFFLEVBQUMsR0FBRyxFQUFILEdBQUcsRUFBQyxDQUFDLENBQUM7TUFDdEQ7O0FBRUQsMEJBQXFCLEVBQUUsK0JBQUMsR0FBRyxFQUFJO0FBQzdCLGNBQU8sYUFBYSxDQUFDLEdBQUcsQ0FBQyxHQUFHLENBQUMsZUFBZSxFQUFFLEVBQUMsR0FBRyxFQUFILEdBQUcsRUFBQyxDQUFDLENBQUM7TUFDdEQ7O0FBRUQsaUJBQVksRUFBRSxzQkFBQyxXQUFXLEVBQUs7QUFDN0IsY0FBTyxhQUFhLENBQUMsR0FBRyxDQUFDLEdBQUcsQ0FBQyxVQUFVLEVBQUUsRUFBQyxXQUFXLEVBQVgsV0FBVyxFQUFDLENBQUMsQ0FBQztNQUN6RDs7QUFFRCwwQkFBcUIsRUFBRSwrQkFBQyxLQUFLLEVBQUUsR0FBRyxFQUFLO0FBQ3JDLFdBQUksUUFBUSxHQUFHLGFBQWEsRUFBRSxDQUFDO0FBQy9CLGNBQVUsUUFBUSw0Q0FBdUMsR0FBRyxvQ0FBK0IsS0FBSyxDQUFHO01BQ3BHOztBQUVELGtCQUFhLEVBQUUsdUJBQUMsS0FBeUMsRUFBSztXQUE3QyxLQUFLLEdBQU4sS0FBeUMsQ0FBeEMsS0FBSztXQUFFLFFBQVEsR0FBaEIsS0FBeUMsQ0FBakMsUUFBUTtXQUFFLEtBQUssR0FBdkIsS0FBeUMsQ0FBdkIsS0FBSztXQUFFLEdBQUcsR0FBNUIsS0FBeUMsQ0FBaEIsR0FBRztXQUFFLElBQUksR0FBbEMsS0FBeUMsQ0FBWCxJQUFJO1dBQUUsSUFBSSxHQUF4QyxLQUF5QyxDQUFMLElBQUk7O0FBQ3RELFdBQUksTUFBTSxHQUFHO0FBQ1gsa0JBQVMsRUFBRSxRQUFRO0FBQ25CLGNBQUssRUFBTCxLQUFLO0FBQ0wsWUFBRyxFQUFILEdBQUc7QUFDSCxhQUFJLEVBQUU7QUFDSixZQUFDLEVBQUUsSUFBSTtBQUNQLFlBQUMsRUFBRSxJQUFJO1VBQ1I7UUFDRjs7QUFFRCxXQUFJLElBQUksR0FBRyxJQUFJLENBQUMsU0FBUyxDQUFDLE1BQU0sQ0FBQyxDQUFDO0FBQ2xDLFdBQUksV0FBVyxHQUFHLE1BQU0sQ0FBQyxTQUFTLENBQUMsSUFBSSxDQUFDLENBQUM7QUFDekMsV0FBSSxRQUFRLEdBQUcsYUFBYSxFQUFFLENBQUM7QUFDL0IsY0FBVSxRQUFRLHdEQUFtRCxLQUFLLGdCQUFXLFdBQVcsQ0FBRztNQUNwRztJQUNGOztBQUVELFNBQU0sRUFBRTtBQUNOLFFBQUcsRUFBRSxNQUFNO0FBQ1gsV0FBTSxFQUFFLGFBQWE7QUFDckIsVUFBSyxFQUFFLFlBQVk7QUFDbkIsVUFBSyxFQUFFLFlBQVk7QUFDbkIsa0JBQWEsRUFBRSxvQkFBb0I7QUFDbkMsWUFBTyxFQUFFLDJCQUEyQjtBQUNwQyxhQUFRLEVBQUUsZUFBZTtBQUN6QixpQkFBWSxFQUFFLGVBQWU7SUFDOUI7O0FBRUQsMkJBQXdCLG9DQUFDLEdBQUcsRUFBQztBQUMzQixZQUFPLGFBQWEsQ0FBQyxHQUFHLENBQUMsTUFBTSxDQUFDLGFBQWEsRUFBRSxFQUFDLEdBQUcsRUFBSCxHQUFHLEVBQUMsQ0FBQyxDQUFDO0lBQ3ZEO0VBQ0Y7O3NCQUVjLEdBQUc7O0FBRWxCLFVBQVMsYUFBYSxHQUFFO0FBQ3RCLE9BQUksTUFBTSxHQUFHLFFBQVEsQ0FBQyxRQUFRLElBQUksUUFBUSxHQUFDLFFBQVEsR0FBQyxPQUFPLENBQUM7QUFDNUQsT0FBSSxRQUFRLEdBQUcsUUFBUSxDQUFDLFFBQVEsSUFBRSxRQUFRLENBQUMsSUFBSSxHQUFHLEdBQUcsR0FBQyxRQUFRLENBQUMsSUFBSSxHQUFFLEVBQUUsQ0FBQyxDQUFDO0FBQ3pFLGVBQVUsTUFBTSxHQUFHLFFBQVEsQ0FBRztFQUMvQjs7Ozs7Ozs7QUMvRkQ7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBLDhCQUE2QixzQkFBc0I7QUFDbkQ7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0EsZUFBYztBQUNkLGVBQWM7QUFDZDtBQUNBLFlBQVcsT0FBTztBQUNsQixhQUFZO0FBQ1o7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBOztBQUVBOzs7Ozs7Ozs7O2dCQ3BEOEMsbUJBQU8sQ0FBQyxFQUFjLENBQUM7O0tBQS9ELGNBQWMsWUFBZCxjQUFjO0tBQUUsbUJBQW1CLFlBQW5CLG1CQUFtQjs7QUFFekMsS0FBTSxhQUFhLEdBQUcsVUFBVSxDQUFDOztBQUVqQyxLQUFJLFFBQVEsR0FBRyxtQkFBbUIsRUFBRSxDQUFDOztBQUVyQyxLQUFJLE9BQU8sR0FBRzs7QUFFWixPQUFJLGtCQUF3QjtTQUF2QixPQUFPLHlEQUFDLGNBQWM7O0FBQ3pCLGFBQVEsR0FBRyxPQUFPLENBQUM7SUFDcEI7O0FBRUQsYUFBVSx3QkFBRTtBQUNWLFlBQU8sUUFBUSxDQUFDO0lBQ2pCOztBQUVELGNBQVcsdUJBQUMsUUFBUSxFQUFDO0FBQ25CLGlCQUFZLENBQUMsT0FBTyxDQUFDLGFBQWEsRUFBRSxJQUFJLENBQUMsU0FBUyxDQUFDLFFBQVEsQ0FBQyxDQUFDLENBQUM7SUFDL0Q7O0FBRUQsY0FBVyx5QkFBRTtBQUNYLFNBQUksSUFBSSxHQUFHLFlBQVksQ0FBQyxPQUFPLENBQUMsYUFBYSxDQUFDLENBQUM7QUFDL0MsU0FBRyxJQUFJLEVBQUM7QUFDTixjQUFPLElBQUksQ0FBQyxLQUFLLENBQUMsSUFBSSxDQUFDLENBQUM7TUFDekI7O0FBRUQsWUFBTyxFQUFFLENBQUM7SUFDWDs7QUFFRCxRQUFLLG1CQUFFO0FBQ0wsaUJBQVksQ0FBQyxLQUFLLEVBQUU7SUFDckI7O0VBRUY7O0FBRUQsT0FBTSxDQUFDLE9BQU8sR0FBRyxPQUFPLEM7Ozs7Ozs7OztBQ25DeEIsS0FBSSxDQUFDLEdBQUcsbUJBQU8sQ0FBQyxFQUFRLENBQUMsQ0FBQztBQUMxQixLQUFJLE9BQU8sR0FBRyxtQkFBTyxDQUFDLEVBQWEsQ0FBQyxDQUFDOztBQUVyQyxLQUFNLEdBQUcsR0FBRzs7QUFFVixNQUFHLGVBQUMsSUFBSSxFQUFFLElBQUksRUFBRSxTQUFTLEVBQUM7QUFDeEIsWUFBTyxHQUFHLENBQUMsSUFBSSxDQUFDLEVBQUMsR0FBRyxFQUFFLElBQUksRUFBRSxJQUFJLEVBQUUsSUFBSSxDQUFDLFNBQVMsQ0FBQyxJQUFJLENBQUMsRUFBRSxJQUFJLEVBQUUsS0FBSyxFQUFDLEVBQUUsU0FBUyxDQUFDLENBQUM7SUFDbEY7O0FBRUQsT0FBSSxnQkFBQyxJQUFJLEVBQUUsSUFBSSxFQUFFLFNBQVMsRUFBQztBQUN6QixZQUFPLEdBQUcsQ0FBQyxJQUFJLENBQUMsRUFBQyxHQUFHLEVBQUUsSUFBSSxFQUFFLElBQUksRUFBRSxJQUFJLENBQUMsU0FBUyxDQUFDLElBQUksQ0FBQyxFQUFFLElBQUksRUFBRSxNQUFNLEVBQUMsRUFBRSxTQUFTLENBQUMsQ0FBQztJQUNuRjs7QUFFRCxNQUFHLGVBQUMsSUFBSSxFQUFDO0FBQ1AsWUFBTyxHQUFHLENBQUMsSUFBSSxDQUFDLEVBQUMsR0FBRyxFQUFFLElBQUksRUFBQyxDQUFDLENBQUM7SUFDOUI7O0FBRUQsT0FBSSxnQkFBQyxHQUFHLEVBQW1CO1NBQWpCLFNBQVMseURBQUcsSUFBSTs7QUFDeEIsU0FBSSxVQUFVLEdBQUc7QUFDZixXQUFJLEVBQUUsS0FBSztBQUNYLGVBQVEsRUFBRSxNQUFNO0FBQ2hCLGlCQUFVLEVBQUUsb0JBQVMsR0FBRyxFQUFFO0FBQ3hCLGFBQUcsU0FBUyxFQUFDO3NDQUNLLE9BQU8sQ0FBQyxXQUFXLEVBQUU7O2VBQS9CLEtBQUssd0JBQUwsS0FBSzs7QUFDWCxjQUFHLENBQUMsZ0JBQWdCLENBQUMsZUFBZSxFQUFDLFNBQVMsR0FBRyxLQUFLLENBQUMsQ0FBQztVQUN6RDtRQUNEO01BQ0g7O0FBRUQsWUFBTyxDQUFDLENBQUMsSUFBSSxDQUFDLENBQUMsQ0FBQyxNQUFNLENBQUMsRUFBRSxFQUFFLFVBQVUsRUFBRSxHQUFHLENBQUMsQ0FBQyxDQUFDO0lBQzlDO0VBQ0Y7O0FBRUQsT0FBTSxDQUFDLE9BQU8sR0FBRyxHQUFHLEM7Ozs7Ozs7QUNqQ3BCLHlCOzs7Ozs7Ozs7QUNBQSxPQUFNLENBQUMsT0FBTyxDQUFDLE9BQU8sR0FBRyxtQkFBTyxDQUFDLEVBQVcsQ0FBQyxDQUFDO0FBQzlDLE9BQU0sQ0FBQyxPQUFPLENBQUMsT0FBTyxHQUFHLG1CQUFPLENBQUMsRUFBVyxDQUFDLENBQUM7QUFDOUMsT0FBTSxDQUFDLE9BQU8sQ0FBQyxlQUFlLEdBQUcsbUJBQU8sQ0FBQyxFQUFtQixDQUFDLEM7Ozs7Ozs7Ozs7QUNGN0QsS0FBTSxzQkFBc0IsR0FBRyxTQUF6QixzQkFBc0IsQ0FBSSxRQUFRO1VBQUssQ0FBRSxDQUFDLFlBQVksQ0FBQyxFQUFFLFVBQUMsS0FBSyxFQUFJO0FBQ3ZFLFNBQUksTUFBTSxHQUFHLEtBQUssQ0FBQyxJQUFJLENBQUMsY0FBSTtjQUFHLElBQUksQ0FBQyxHQUFHLENBQUMsSUFBSSxDQUFDLEtBQUssUUFBUTtNQUFBLENBQUMsQ0FBQztBQUM1RCxZQUFPLENBQUMsTUFBTSxHQUFHLEVBQUUsR0FBRyxNQUFNLENBQUMsR0FBRyxDQUFDLFVBQVUsQ0FBQyxDQUFDO0lBQzlDLENBQUM7RUFBQSxDQUFDOztBQUVILEtBQU0sWUFBWSxHQUFHLENBQUUsQ0FBQyxZQUFZLENBQUMsRUFBRSxVQUFDLEtBQUssRUFBSTtBQUM3QyxVQUFPLEtBQUssQ0FBQyxHQUFHLENBQUMsVUFBQyxJQUFJLEVBQUc7QUFDdkIsU0FBSSxRQUFRLEdBQUcsSUFBSSxDQUFDLEdBQUcsQ0FBQyxJQUFJLENBQUMsQ0FBQztBQUM5QixZQUFPO0FBQ0wsU0FBRSxFQUFFLFFBQVE7QUFDWixlQUFRLEVBQUUsSUFBSSxDQUFDLEdBQUcsQ0FBQyxVQUFVLENBQUM7QUFDOUIsV0FBSSxFQUFFLE9BQU8sQ0FBQyxJQUFJLENBQUM7QUFDbkIsV0FBSSxFQUFFLElBQUksQ0FBQyxHQUFHLENBQUMsTUFBTSxDQUFDO01BQ3ZCO0lBQ0YsQ0FBQyxDQUFDLElBQUksRUFBRSxDQUFDO0VBQ1osQ0FDRCxDQUFDOztBQUVGLFVBQVMsT0FBTyxDQUFDLElBQUksRUFBQztBQUNwQixPQUFJLFNBQVMsR0FBRyxFQUFFLENBQUM7QUFDbkIsT0FBSSxNQUFNLEdBQUcsSUFBSSxDQUFDLEdBQUcsQ0FBQyxRQUFRLENBQUMsQ0FBQzs7QUFFaEMsT0FBRyxNQUFNLEVBQUM7QUFDUixXQUFNLENBQUMsUUFBUSxFQUFFLENBQUMsT0FBTyxFQUFFLENBQUMsT0FBTyxDQUFDLGNBQUksRUFBRTtBQUN4QyxnQkFBUyxDQUFDLElBQUksQ0FBQztBQUNiLGFBQUksRUFBRSxJQUFJLENBQUMsQ0FBQyxDQUFDO0FBQ2IsY0FBSyxFQUFFLElBQUksQ0FBQyxDQUFDLENBQUM7UUFDZixDQUFDLENBQUM7TUFDSixDQUFDLENBQUM7SUFDSjs7QUFFRCxTQUFNLEdBQUcsSUFBSSxDQUFDLEdBQUcsQ0FBQyxZQUFZLENBQUMsQ0FBQzs7QUFFaEMsT0FBRyxNQUFNLEVBQUM7QUFDUixXQUFNLENBQUMsUUFBUSxFQUFFLENBQUMsT0FBTyxFQUFFLENBQUMsT0FBTyxDQUFDLGNBQUksRUFBRTtBQUN4QyxnQkFBUyxDQUFDLElBQUksQ0FBQztBQUNiLGFBQUksRUFBRSxJQUFJLENBQUMsQ0FBQyxDQUFDO0FBQ2IsY0FBSyxFQUFFLElBQUksQ0FBQyxDQUFDLENBQUMsQ0FBQyxHQUFHLENBQUMsUUFBUSxDQUFDO0FBQzVCLGdCQUFPLEVBQUUsSUFBSSxDQUFDLENBQUMsQ0FBQyxDQUFDLEdBQUcsQ0FBQyxTQUFTLENBQUM7UUFDaEMsQ0FBQyxDQUFDO01BQ0osQ0FBQyxDQUFDO0lBQ0o7O0FBRUQsVUFBTyxTQUFTLENBQUM7RUFDbEI7O3NCQUdjO0FBQ2IsZUFBWSxFQUFaLFlBQVk7QUFDWix5QkFBc0IsRUFBdEIsc0JBQXNCO0VBQ3ZCOzs7Ozs7Ozs7Ozs7OztzQ0NsRHFCLEVBQVc7Ozs7c0JBRWxCLHVCQUFVO0FBQ3ZCLG9CQUFpQixFQUFFLElBQUk7QUFDdkIsa0JBQWUsRUFBRSxJQUFJO0FBQ3JCLGtCQUFlLEVBQUUsSUFBSTtFQUN0QixDQUFDOzs7Ozs7Ozs7O0FDTkYsT0FBTSxDQUFDLE9BQU8sQ0FBQyxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxHQUFXLENBQUMsQ0FBQztBQUM5QyxPQUFNLENBQUMsT0FBTyxDQUFDLE9BQU8sR0FBRyxtQkFBTyxDQUFDLEVBQVcsQ0FBQyxDQUFDO0FBQzlDLE9BQU0sQ0FBQyxPQUFPLENBQUMsZUFBZSxHQUFHLG1CQUFPLENBQUMsR0FBZ0IsQ0FBQyxDOzs7Ozs7Ozs7OztnQkNGbEMsbUJBQU8sQ0FBQyxFQUErQixDQUFDOztLQUEzRCxlQUFlLFlBQWYsZUFBZTs7aUJBQ0UsbUJBQU8sQ0FBQyxHQUE2QixDQUFDOztLQUF2RCxhQUFhLGFBQWIsYUFBYTs7QUFFbEIsS0FBTSxJQUFJLEdBQUcsQ0FBRSxDQUFDLFdBQVcsQ0FBQyxFQUFFLFVBQUMsV0FBVyxFQUFLO0FBQzNDLE9BQUcsQ0FBQyxXQUFXLEVBQUM7QUFDZCxZQUFPLElBQUksQ0FBQztJQUNiOztBQUVELE9BQUksSUFBSSxHQUFHLFdBQVcsQ0FBQyxHQUFHLENBQUMsTUFBTSxDQUFDLElBQUksRUFBRSxDQUFDO0FBQ3pDLE9BQUksZ0JBQWdCLEdBQUcsSUFBSSxDQUFDLENBQUMsQ0FBQyxJQUFJLEVBQUUsQ0FBQzs7QUFFckMsVUFBTztBQUNMLFNBQUksRUFBSixJQUFJO0FBQ0oscUJBQWdCLEVBQWhCLGdCQUFnQjtBQUNoQixXQUFNLEVBQUUsV0FBVyxDQUFDLEdBQUcsQ0FBQyxnQkFBZ0IsQ0FBQyxDQUFDLElBQUksRUFBRTtJQUNqRDtFQUNGLENBQ0YsQ0FBQzs7c0JBRWE7QUFDYixPQUFJLEVBQUosSUFBSTtBQUNKLGNBQVcsRUFBRSxhQUFhLENBQUMsZUFBZSxDQUFDO0VBQzVDOzs7Ozs7Ozs7Ozs7Ozs7Ozs7QUN0QkQsS0FBSSxLQUFLLEdBQUcsbUJBQU8sQ0FBQyxDQUFPLENBQUMsQ0FBQzs7QUFFN0IsS0FBTSxnQkFBZ0IsR0FBRyxTQUFuQixnQkFBZ0IsQ0FBSSxJQUFxQztPQUFwQyxRQUFRLEdBQVQsSUFBcUMsQ0FBcEMsUUFBUTtPQUFFLElBQUksR0FBZixJQUFxQyxDQUExQixJQUFJO09BQUUsU0FBUyxHQUExQixJQUFxQyxDQUFwQixTQUFTOztPQUFLLEtBQUssNEJBQXBDLElBQXFDOztVQUM3RDtBQUFDLGlCQUFZO0tBQUssS0FBSztLQUNwQixJQUFJLENBQUMsUUFBUSxDQUFDLENBQUMsU0FBUyxDQUFDO0lBQ2I7RUFDaEIsQ0FBQzs7QUFFRixLQUFJLGlCQUFpQixHQUFHLEtBQUssQ0FBQyxXQUFXLENBQUM7OztBQUN4QyxrQkFBZSw2QkFBRztBQUNoQixTQUFJLENBQUMsYUFBYSxHQUFHLElBQUksQ0FBQyxhQUFhLENBQUMsSUFBSSxDQUFDLElBQUksQ0FBQyxDQUFDO0lBQ3BEOztBQUVELFNBQU0sb0JBQUc7a0JBQzZCLElBQUksQ0FBQyxLQUFLO1NBQXpDLE9BQU8sVUFBUCxPQUFPO1NBQUUsUUFBUSxVQUFSLFFBQVE7O1NBQUssS0FBSzs7QUFDaEMsWUFDRTtBQUFDLFdBQUk7T0FBSyxLQUFLO09BQ2I7O1dBQUcsT0FBTyxFQUFFLElBQUksQ0FBQyxhQUFjO1NBQzVCLFFBQVE7O1NBQUcsT0FBTyxHQUFJLE9BQU8sS0FBSyxTQUFTLENBQUMsSUFBSSxHQUFHLEdBQUcsR0FBRyxHQUFHLEdBQUksRUFBRTtRQUNqRTtNQUNDLENBQ1A7SUFDSDs7QUFFRCxnQkFBYSx5QkFBQyxDQUFDLEVBQUU7QUFDZixNQUFDLENBQUMsY0FBYyxFQUFFLENBQUM7O0FBRW5CLFNBQUksSUFBSSxDQUFDLEtBQUssQ0FBQyxZQUFZLEVBQUU7QUFDM0IsV0FBSSxDQUFDLEtBQUssQ0FBQyxZQUFZLENBQ3JCLElBQUksQ0FBQyxLQUFLLENBQUMsU0FBUyxFQUNwQixJQUFJLENBQUMsS0FBSyxDQUFDLE9BQU8sR0FDaEIsb0JBQW9CLENBQUMsSUFBSSxDQUFDLEtBQUssQ0FBQyxPQUFPLENBQUMsR0FDeEMsU0FBUyxDQUFDLElBQUksQ0FDakIsQ0FBQztNQUNIO0lBQ0Y7RUFDRixDQUFDLENBQUM7Ozs7O0FBS0gsS0FBTSxTQUFTLEdBQUc7QUFDaEIsTUFBRyxFQUFFLEtBQUs7QUFDVixPQUFJLEVBQUUsTUFBTTtFQUNiLENBQUM7O0FBRUYsS0FBTSxhQUFhLEdBQUcsU0FBaEIsYUFBYSxDQUFJLEtBQVMsRUFBRztPQUFYLE9BQU8sR0FBUixLQUFTLENBQVIsT0FBTzs7QUFDN0IsT0FBSSxHQUFHLEdBQUcscUNBQXFDO0FBQy9DLE9BQUcsT0FBTyxLQUFLLFNBQVMsQ0FBQyxJQUFJLEVBQUM7QUFDNUIsUUFBRyxJQUFJLE9BQU87SUFDZjs7QUFFRCxPQUFJLE9BQU8sS0FBSyxTQUFTLENBQUMsR0FBRyxFQUFDO0FBQzVCLFFBQUcsSUFBSSxNQUFNO0lBQ2Q7O0FBRUQsVUFBUSwyQkFBRyxTQUFTLEVBQUUsR0FBSSxHQUFLLENBQUU7RUFDbEMsQ0FBQzs7Ozs7QUFLRixLQUFJLGNBQWMsR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFDckMsU0FBTSxvQkFBRzttQkFDcUMsSUFBSSxDQUFDLEtBQUs7U0FBakQsT0FBTyxXQUFQLE9BQU87U0FBRSxTQUFTLFdBQVQsU0FBUztTQUFFLEtBQUssV0FBTCxLQUFLOztTQUFLLEtBQUs7O0FBRXhDLFlBQ0U7QUFBQyxtQkFBWTtPQUFLLEtBQUs7T0FDckI7O1dBQUcsT0FBTyxFQUFFLElBQUksQ0FBQyxZQUFhO1NBQzNCLEtBQUs7UUFDSjtPQUNKLG9CQUFDLGFBQWEsSUFBQyxPQUFPLEVBQUUsT0FBUSxHQUFFO01BQ3JCLENBQ2Y7SUFDSDs7QUFFRCxlQUFZLHdCQUFDLENBQUMsRUFBRTtBQUNkLE1BQUMsQ0FBQyxjQUFjLEVBQUUsQ0FBQztBQUNuQixTQUFHLElBQUksQ0FBQyxLQUFLLENBQUMsWUFBWSxFQUFFOztBQUUxQixXQUFJLE1BQU0sR0FBRyxTQUFTLENBQUMsSUFBSSxDQUFDO0FBQzVCLFdBQUcsSUFBSSxDQUFDLEtBQUssQ0FBQyxPQUFPLEVBQUM7QUFDcEIsZUFBTSxHQUFHLElBQUksQ0FBQyxLQUFLLENBQUMsT0FBTyxLQUFLLFNBQVMsQ0FBQyxJQUFJLEdBQUcsU0FBUyxDQUFDLEdBQUcsR0FBRyxTQUFTLENBQUMsSUFBSSxDQUFDO1FBQ2pGO0FBQ0QsV0FBSSxDQUFDLEtBQUssQ0FBQyxZQUFZLENBQUMsSUFBSSxDQUFDLEtBQUssQ0FBQyxTQUFTLEVBQUUsTUFBTSxDQUFDLENBQUM7TUFDdkQ7SUFDRjtFQUNGLENBQUMsQ0FBQzs7Ozs7QUFLSCxLQUFJLFlBQVksR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFDbkMsU0FBTSxvQkFBRTtBQUNOLFNBQUksS0FBSyxHQUFHLElBQUksQ0FBQyxLQUFLLENBQUM7QUFDdkIsWUFBTyxLQUFLLENBQUMsUUFBUSxHQUFHOztTQUFJLEdBQUcsRUFBRSxLQUFLLENBQUMsR0FBSSxFQUFDLFNBQVMsRUFBQyxnQkFBZ0I7T0FBRSxLQUFLLENBQUMsUUFBUTtNQUFNLEdBQUc7O1NBQUksR0FBRyxFQUFFLEtBQUssQ0FBQyxHQUFJO09BQUUsS0FBSyxDQUFDLFFBQVE7TUFBTSxDQUFDO0lBQzFJO0VBQ0YsQ0FBQyxDQUFDOzs7OztBQUtILEtBQUksUUFBUSxHQUFHLEtBQUssQ0FBQyxXQUFXLENBQUM7OztBQUUvQixlQUFZLHdCQUFDLFFBQVEsRUFBQzs7O0FBQ3BCLFNBQUksS0FBSyxHQUFHLFFBQVEsQ0FBQyxHQUFHLENBQUMsVUFBQyxJQUFJLEVBQUUsS0FBSyxFQUFHO0FBQ3RDLGNBQU8sTUFBSyxVQUFVLENBQUMsSUFBSSxDQUFDLEtBQUssQ0FBQyxNQUFNLGFBQUcsS0FBSyxFQUFMLEtBQUssRUFBRSxHQUFHLEVBQUUsS0FBSyxFQUFFLFFBQVEsRUFBRSxJQUFJLElBQUssSUFBSSxDQUFDLEtBQUssRUFBRSxDQUFDO01BQy9GLENBQUM7O0FBRUYsWUFBTzs7U0FBTyxTQUFTLEVBQUMsa0JBQWtCO09BQUM7OztTQUFLLEtBQUs7UUFBTTtNQUFRO0lBQ3BFOztBQUVELGFBQVUsc0JBQUMsUUFBUSxFQUFDOzs7QUFDbEIsU0FBSSxLQUFLLEdBQUcsSUFBSSxDQUFDLEtBQUssQ0FBQyxRQUFRLENBQUM7QUFDaEMsU0FBSSxJQUFJLEdBQUcsRUFBRSxDQUFDO0FBQ2QsVUFBSSxJQUFJLENBQUMsR0FBRyxDQUFDLEVBQUUsQ0FBQyxHQUFHLEtBQUssRUFBRSxDQUFDLEVBQUcsRUFBQztBQUM3QixXQUFJLEtBQUssR0FBRyxRQUFRLENBQUMsR0FBRyxDQUFDLFVBQUMsSUFBSSxFQUFFLEtBQUssRUFBRztBQUN0QyxnQkFBTyxPQUFLLFVBQVUsQ0FBQyxJQUFJLENBQUMsS0FBSyxDQUFDLElBQUksYUFBRyxRQUFRLEVBQUUsQ0FBQyxFQUFFLEdBQUcsRUFBRSxLQUFLLEVBQUUsUUFBUSxFQUFFLEtBQUssSUFBSyxJQUFJLENBQUMsS0FBSyxFQUFFLENBQUM7UUFDcEcsQ0FBQzs7QUFFRixXQUFJLENBQUMsSUFBSSxDQUFDOztXQUFJLEdBQUcsRUFBRSxDQUFFO1NBQUUsS0FBSztRQUFNLENBQUMsQ0FBQztNQUNyQzs7QUFFRCxZQUFPOzs7T0FBUSxJQUFJO01BQVMsQ0FBQztJQUM5Qjs7QUFFRCxhQUFVLHNCQUFDLElBQUksRUFBRSxTQUFTLEVBQUM7QUFDekIsU0FBSSxPQUFPLEdBQUcsSUFBSSxDQUFDO0FBQ25CLFNBQUksS0FBSyxDQUFDLGNBQWMsQ0FBQyxJQUFJLENBQUMsRUFBRTtBQUM3QixjQUFPLEdBQUcsS0FBSyxDQUFDLFlBQVksQ0FBQyxJQUFJLEVBQUUsU0FBUyxDQUFDLENBQUM7TUFDL0MsTUFBTSxJQUFJLE9BQU8sS0FBSyxDQUFDLElBQUksS0FBSyxVQUFVLEVBQUU7QUFDM0MsY0FBTyxHQUFHLElBQUksQ0FBQyxTQUFTLENBQUMsQ0FBQztNQUMzQjs7QUFFRCxZQUFPLE9BQU8sQ0FBQztJQUNqQjs7QUFFRCxTQUFNLG9CQUFHO0FBQ1AsU0FBSSxRQUFRLEdBQUcsRUFBRSxDQUFDO0FBQ2xCLFVBQUssQ0FBQyxRQUFRLENBQUMsT0FBTyxDQUFDLElBQUksQ0FBQyxLQUFLLENBQUMsUUFBUSxFQUFFLFVBQUMsS0FBSyxFQUFFLEtBQUssRUFBSztBQUM1RCxXQUFJLEtBQUssSUFBSSxJQUFJLEVBQUU7QUFDakIsZ0JBQU87UUFDUjs7QUFFRCxXQUFHLEtBQUssQ0FBQyxJQUFJLENBQUMsV0FBVyxLQUFLLGdCQUFnQixFQUFDO0FBQzdDLGVBQU0sMEJBQTBCLENBQUM7UUFDbEM7O0FBRUQsZUFBUSxDQUFDLElBQUksQ0FBQyxLQUFLLENBQUMsQ0FBQztNQUN0QixDQUFDLENBQUM7O0FBRUgsU0FBSSxVQUFVLEdBQUcsUUFBUSxHQUFHLElBQUksQ0FBQyxLQUFLLENBQUMsU0FBUyxDQUFDOztBQUVqRCxZQUNFOztTQUFPLFNBQVMsRUFBRSxVQUFXO09BQzFCLElBQUksQ0FBQyxZQUFZLENBQUMsUUFBUSxDQUFDO09BQzNCLElBQUksQ0FBQyxVQUFVLENBQUMsUUFBUSxDQUFDO01BQ3BCLENBQ1I7SUFDSDtFQUNGLENBQUM7O0FBRUYsS0FBSSxjQUFjLEdBQUcsS0FBSyxDQUFDLFdBQVcsQ0FBQzs7O0FBQ3JDLFNBQU0sRUFBRSxrQkFBVztBQUNqQixXQUFNLElBQUksS0FBSyxDQUFDLGtEQUFrRCxDQUFDLENBQUM7SUFDckU7RUFDRixDQUFDOztzQkFFYSxRQUFRO1NBRUgsTUFBTSxHQUF4QixjQUFjO1NBQ0YsS0FBSyxHQUFqQixRQUFRO1NBQ1EsSUFBSSxHQUFwQixZQUFZO1NBQ1EsUUFBUSxHQUE1QixnQkFBZ0I7U0FDaEIsY0FBYyxHQUFkLGNBQWM7U0FDZCxhQUFhLEdBQWIsYUFBYTtTQUNiLFNBQVMsR0FBVCxTQUFTLEM7Ozs7Ozs7OztBQ2hMWCxvQjs7Ozs7Ozs7OztBQ0FBLEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsQ0FBYSxDQUFDLENBQUM7QUFDckMsS0FBSSxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxFQUFhLENBQUMsQ0FBQzs7Z0JBQ3hCLG1CQUFPLENBQUMsR0FBVyxDQUFDOztLQUE1QixJQUFJLFlBQUosSUFBSTs7QUFDVCxLQUFJLEdBQUcsR0FBRyxtQkFBTyxDQUFDLEVBQWtCLENBQUMsQ0FBQztBQUN0QyxLQUFJLEdBQUcsR0FBRyxtQkFBTyxDQUFDLEVBQVksQ0FBQyxDQUFDO0FBQ2hDLEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsRUFBVyxDQUFDLENBQUM7QUFDbkMsS0FBSSxhQUFhLEdBQUcsbUJBQU8sQ0FBQyxFQUFlLENBQUMsQ0FBQzs7aUJBRXNCLG1CQUFPLENBQUMsRUFBZSxDQUFDOztLQUFyRixjQUFjLGFBQWQsY0FBYztLQUFFLGVBQWUsYUFBZixlQUFlO0tBQUUsdUJBQXVCLGFBQXZCLHVCQUF1Qjs7QUFFOUQsS0FBSSxPQUFPLEdBQUc7O0FBRVosZUFBWSx3QkFBQyxRQUFRLEVBQUUsS0FBSyxFQUFDO0FBQzNCLFlBQU8sQ0FBQyxRQUFRLENBQUMsdUJBQXVCLEVBQUU7QUFDeEMsZUFBUSxFQUFSLFFBQVE7QUFDUixZQUFLLEVBQUwsS0FBSztNQUNOLENBQUMsQ0FBQztJQUNKOztBQUVELFFBQUssbUJBQUU7NkJBQ2dCLE9BQU8sQ0FBQyxRQUFRLENBQUMsT0FBTyxDQUFDLGFBQWEsQ0FBQzs7U0FBdkQsWUFBWSxxQkFBWixZQUFZOztBQUVqQixZQUFPLENBQUMsUUFBUSxDQUFDLGVBQWUsQ0FBQyxDQUFDOztBQUVsQyxTQUFHLFlBQVksRUFBQztBQUNkLGNBQU8sQ0FBQyxVQUFVLEVBQUUsQ0FBQyxJQUFJLENBQUMsR0FBRyxDQUFDLE1BQU0sQ0FBQyxLQUFLLENBQUMsQ0FBQztNQUM3QyxNQUFJO0FBQ0gsY0FBTyxDQUFDLFVBQVUsRUFBRSxDQUFDLElBQUksQ0FBQyxHQUFHLENBQUMsTUFBTSxDQUFDLFFBQVEsQ0FBQyxDQUFDO01BQ2hEO0lBQ0Y7O0FBRUQsU0FBTSxrQkFBQyxDQUFDLEVBQUUsQ0FBQyxFQUFDOztBQUVWLE1BQUMsR0FBRyxDQUFDLEdBQUcsQ0FBQyxHQUFHLENBQUMsR0FBRyxDQUFDLENBQUM7QUFDbEIsTUFBQyxHQUFHLENBQUMsR0FBRyxDQUFDLEdBQUcsQ0FBQyxHQUFHLENBQUMsQ0FBQzs7QUFFbEIsU0FBSSxPQUFPLEdBQUcsRUFBRSxlQUFlLEVBQUUsRUFBRSxDQUFDLEVBQUQsQ0FBQyxFQUFFLENBQUMsRUFBRCxDQUFDLEVBQUUsRUFBRSxDQUFDOzs4QkFDaEMsT0FBTyxDQUFDLFFBQVEsQ0FBQyxPQUFPLENBQUMsYUFBYSxDQUFDOztTQUE5QyxHQUFHLHNCQUFILEdBQUc7O0FBRVIsUUFBRyxDQUFDLEdBQUcsQ0FBQyxHQUFHLENBQUMsR0FBRyxDQUFDLHFCQUFxQixDQUFDLEdBQUcsQ0FBQyxFQUFFLE9BQU8sQ0FBQyxDQUNqRCxJQUFJLENBQUMsWUFBSTtBQUNSLGNBQU8sQ0FBQyxHQUFHLG9CQUFrQixDQUFDLGVBQVUsQ0FBQyxXQUFRLENBQUM7TUFDbkQsQ0FBQyxDQUNELElBQUksQ0FBQyxZQUFJO0FBQ1IsY0FBTyxDQUFDLEdBQUcsOEJBQTRCLENBQUMsZUFBVSxDQUFDLENBQUcsQ0FBQztNQUMxRCxDQUFDO0lBQ0g7O0FBRUQsY0FBVyx1QkFBQyxHQUFHLEVBQUM7QUFDZCxrQkFBYSxDQUFDLE9BQU8sQ0FBQyxZQUFZLENBQUMsR0FBRyxDQUFDLENBQ3BDLElBQUksQ0FBQyxZQUFJO0FBQ1IsV0FBSSxLQUFLLEdBQUcsT0FBTyxDQUFDLFFBQVEsQ0FBQyxhQUFhLENBQUMsT0FBTyxDQUFDLGVBQWUsQ0FBQyxHQUFHLENBQUMsQ0FBQyxDQUFDO1dBQ25FLFFBQVEsR0FBWSxLQUFLLENBQXpCLFFBQVE7V0FBRSxLQUFLLEdBQUssS0FBSyxDQUFmLEtBQUs7O0FBQ3JCLGNBQU8sQ0FBQyxRQUFRLENBQUMsY0FBYyxFQUFFO0FBQzdCLGlCQUFRLEVBQVIsUUFBUTtBQUNSLGNBQUssRUFBTCxLQUFLO0FBQ0wsWUFBRyxFQUFILEdBQUc7QUFDSCxxQkFBWSxFQUFFLEtBQUs7UUFDcEIsQ0FBQyxDQUFDO01BQ04sQ0FBQyxDQUNELElBQUksQ0FBQyxZQUFJO0FBQ1IsY0FBTyxDQUFDLFVBQVUsRUFBRSxDQUFDLElBQUksQ0FBQyxHQUFHLENBQUMsTUFBTSxDQUFDLFlBQVksQ0FBQyxDQUFDO01BQ3BELENBQUM7SUFDTDs7QUFFRCxtQkFBZ0IsNEJBQUMsUUFBUSxFQUFFLEtBQUssRUFBQztBQUMvQixTQUFJLEdBQUcsR0FBRyxJQUFJLEVBQUUsQ0FBQztBQUNqQixTQUFJLFFBQVEsR0FBRyxHQUFHLENBQUMsd0JBQXdCLENBQUMsR0FBRyxDQUFDLENBQUM7QUFDakQsU0FBSSxPQUFPLEdBQUcsT0FBTyxDQUFDLFVBQVUsRUFBRSxDQUFDOztBQUVuQyxZQUFPLENBQUMsUUFBUSxDQUFDLGNBQWMsRUFBRTtBQUMvQixlQUFRLEVBQVIsUUFBUTtBQUNSLFlBQUssRUFBTCxLQUFLO0FBQ0wsVUFBRyxFQUFILEdBQUc7QUFDSCxtQkFBWSxFQUFFLElBQUk7TUFDbkIsQ0FBQyxDQUFDOztBQUVILFlBQU8sQ0FBQyxJQUFJLENBQUMsUUFBUSxDQUFDLENBQUM7SUFDeEI7O0VBRUY7O3NCQUVjLE9BQU87Ozs7Ozs7Ozs7OztnQkNsRkgsbUJBQU8sQ0FBQyxHQUE4QixDQUFDOztLQUFyRCxVQUFVLFlBQVYsVUFBVTs7QUFFZixLQUFNLGFBQWEsR0FBRyxDQUN0QixDQUFDLHNCQUFzQixDQUFDLEVBQUUsQ0FBQyxlQUFlLENBQUMsRUFDM0MsVUFBQyxVQUFVLEVBQUUsUUFBUSxFQUFLO0FBQ3RCLE9BQUcsQ0FBQyxVQUFVLEVBQUM7QUFDYixZQUFPLElBQUksQ0FBQztJQUNiOzs7Ozs7O0FBT0QsT0FBSSxNQUFNLEdBQUc7QUFDWCxpQkFBWSxFQUFFLFVBQVUsQ0FBQyxHQUFHLENBQUMsY0FBYyxDQUFDO0FBQzVDLGFBQVEsRUFBRSxVQUFVLENBQUMsR0FBRyxDQUFDLFVBQVUsQ0FBQztBQUNwQyxTQUFJLEVBQUUsVUFBVSxDQUFDLEdBQUcsQ0FBQyxNQUFNLENBQUM7QUFDNUIsYUFBUSxFQUFFLFVBQVUsQ0FBQyxHQUFHLENBQUMsVUFBVSxDQUFDO0FBQ3BDLGFBQVEsRUFBRSxTQUFTO0FBQ25CLFVBQUssRUFBRSxVQUFVLENBQUMsR0FBRyxDQUFDLE9BQU8sQ0FBQztBQUM5QixRQUFHLEVBQUUsVUFBVSxDQUFDLEdBQUcsQ0FBQyxLQUFLLENBQUM7QUFDMUIsU0FBSSxFQUFFLFNBQVM7QUFDZixTQUFJLEVBQUUsU0FBUztJQUNoQixDQUFDOzs7O0FBSUYsT0FBRyxRQUFRLENBQUMsR0FBRyxDQUFDLE1BQU0sQ0FBQyxHQUFHLENBQUMsRUFBQztBQUMxQixTQUFJLEtBQUssR0FBRyxVQUFVLENBQUMsUUFBUSxDQUFDLEdBQUcsQ0FBQyxNQUFNLENBQUMsR0FBRyxDQUFDLENBQUMsQ0FBQzs7QUFFakQsV0FBTSxDQUFDLE9BQU8sR0FBRyxLQUFLLENBQUMsT0FBTyxDQUFDO0FBQy9CLFdBQU0sQ0FBQyxRQUFRLEdBQUcsS0FBSyxDQUFDLFFBQVEsQ0FBQztBQUNqQyxXQUFNLENBQUMsUUFBUSxHQUFHLEtBQUssQ0FBQyxRQUFRLENBQUM7QUFDakMsV0FBTSxDQUFDLE1BQU0sR0FBRyxLQUFLLENBQUMsTUFBTSxDQUFDO0FBQzdCLFdBQU0sQ0FBQyxJQUFJLEdBQUcsS0FBSyxDQUFDLElBQUksQ0FBQztBQUN6QixXQUFNLENBQUMsSUFBSSxHQUFHLEtBQUssQ0FBQyxJQUFJLENBQUM7SUFDMUI7O0FBRUQsVUFBTyxNQUFNLENBQUM7RUFFZixDQUNGLENBQUM7O3NCQUVhO0FBQ2IsZ0JBQWEsRUFBYixhQUFhO0VBQ2Q7Ozs7Ozs7Ozs7O0FDOUNELEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsQ0FBYSxDQUFDLENBQUM7O2dCQUNpQyxtQkFBTyxDQUFDLEdBQWUsQ0FBQzs7S0FBeEYsNEJBQTRCLFlBQTVCLDRCQUE0QjtLQUFFLDZCQUE2QixZQUE3Qiw2QkFBNkI7O0FBRWpFLEtBQUksT0FBTyxHQUFHO0FBQ1osdUJBQW9CLGtDQUFFO0FBQ3BCLFlBQU8sQ0FBQyxRQUFRLENBQUMsNEJBQTRCLENBQUMsQ0FBQztJQUNoRDs7QUFFRCx3QkFBcUIsbUNBQUU7QUFDckIsWUFBTyxDQUFDLFFBQVEsQ0FBQyw2QkFBNkIsQ0FBQyxDQUFDO0lBQ2pEO0VBQ0Y7O3NCQUVjLE9BQU87Ozs7Ozs7Ozs7O0FDYnRCLEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsQ0FBYSxDQUFDLENBQUM7QUFDckMsS0FBSSxHQUFHLEdBQUcsbUJBQU8sQ0FBQyxFQUFrQixDQUFDLENBQUM7QUFDdEMsS0FBSSxHQUFHLEdBQUcsbUJBQU8sQ0FBQyxFQUFZLENBQUMsQ0FBQzs7Z0JBRXFCLG1CQUFPLENBQUMsR0FBZSxDQUFDOztLQUF2RSxvQkFBb0IsWUFBcEIsb0JBQW9CO0tBQUUsbUJBQW1CLFlBQW5CLG1CQUFtQjtzQkFFaEM7O0FBRWIsZUFBWSx3QkFBQyxHQUFHLEVBQUM7QUFDZixZQUFPLEdBQUcsQ0FBQyxHQUFHLENBQUMsR0FBRyxDQUFDLEdBQUcsQ0FBQyxrQkFBa0IsQ0FBQyxHQUFHLENBQUMsQ0FBQyxDQUFDLElBQUksQ0FBQyxjQUFJLEVBQUU7QUFDekQsV0FBRyxJQUFJLElBQUksSUFBSSxDQUFDLE9BQU8sRUFBQztBQUN0QixnQkFBTyxDQUFDLFFBQVEsQ0FBQyxtQkFBbUIsRUFBRSxJQUFJLENBQUMsT0FBTyxDQUFDLENBQUM7UUFDckQ7TUFDRixDQUFDLENBQUM7SUFDSjs7QUFFRCxnQkFBYSx5QkFBQyxTQUFTLEVBQUUsT0FBTyxFQUFDO0FBQy9CLFlBQU8sR0FBRyxDQUFDLEdBQUcsQ0FBQyxHQUFHLENBQUMsR0FBRyxDQUFDLG1CQUFtQixDQUFDLFNBQVMsRUFBRSxPQUFPLENBQUMsQ0FBQyxDQUFDLElBQUksQ0FBQyxVQUFDLElBQUksRUFBSztBQUM3RSxjQUFPLENBQUMsUUFBUSxDQUFDLG9CQUFvQixFQUFFLElBQUksQ0FBQyxRQUFRLENBQUMsQ0FBQztNQUN2RCxDQUFDLENBQUM7SUFDSjs7QUFFRCxnQkFBYSx5QkFBQyxJQUFJLEVBQUM7QUFDakIsWUFBTyxDQUFDLFFBQVEsQ0FBQyxtQkFBbUIsRUFBRSxJQUFJLENBQUMsQ0FBQztJQUM3QztFQUNGOzs7Ozs7Ozs7O0FDekJELEtBQUksR0FBRyxHQUFHLG1CQUFPLENBQUMsRUFBZ0IsQ0FBQyxDQUFDO0FBQ3BDLEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsRUFBVyxDQUFDLENBQUM7QUFDbkMsS0FBSSxHQUFHLEdBQUcsbUJBQU8sQ0FBQyxFQUFZLENBQUMsQ0FBQztBQUNoQyxLQUFJLENBQUMsR0FBRyxtQkFBTyxDQUFDLEVBQVEsQ0FBQyxDQUFDOztBQUUxQixLQUFNLFdBQVcsR0FBRyxLQUFLLEdBQUcsQ0FBQyxDQUFDOztBQUU5QixLQUFJLG1CQUFtQixHQUFHLElBQUksQ0FBQzs7QUFFL0IsS0FBSSxJQUFJLEdBQUc7O0FBRVQsU0FBTSxrQkFBQyxJQUFJLEVBQUUsUUFBUSxFQUFFLEtBQUssRUFBRSxXQUFXLEVBQUM7QUFDeEMsU0FBSSxJQUFJLEdBQUcsRUFBQyxJQUFJLEVBQUUsSUFBSSxFQUFFLElBQUksRUFBRSxRQUFRLEVBQUUsbUJBQW1CLEVBQUUsS0FBSyxFQUFFLFlBQVksRUFBRSxXQUFXLEVBQUMsQ0FBQztBQUMvRixZQUFPLEdBQUcsQ0FBQyxJQUFJLENBQUMsR0FBRyxDQUFDLEdBQUcsQ0FBQyxjQUFjLEVBQUUsSUFBSSxDQUFDLENBQzFDLElBQUksQ0FBQyxVQUFDLElBQUksRUFBRztBQUNaLGNBQU8sQ0FBQyxXQUFXLENBQUMsSUFBSSxDQUFDLENBQUM7QUFDMUIsV0FBSSxDQUFDLG9CQUFvQixFQUFFLENBQUM7QUFDNUIsY0FBTyxJQUFJLENBQUM7TUFDYixDQUFDLENBQUM7SUFDTjs7QUFFRCxRQUFLLGlCQUFDLElBQUksRUFBRSxRQUFRLEVBQUUsS0FBSyxFQUFDO0FBQzFCLFNBQUksQ0FBQyxtQkFBbUIsRUFBRSxDQUFDO0FBQzNCLFlBQU8sSUFBSSxDQUFDLE1BQU0sQ0FBQyxJQUFJLEVBQUUsUUFBUSxFQUFFLEtBQUssQ0FBQyxDQUFDLElBQUksQ0FBQyxJQUFJLENBQUMsb0JBQW9CLENBQUMsQ0FBQztJQUMzRTs7QUFFRCxhQUFVLHdCQUFFO0FBQ1YsU0FBSSxRQUFRLEdBQUcsT0FBTyxDQUFDLFdBQVcsRUFBRSxDQUFDO0FBQ3JDLFNBQUcsUUFBUSxDQUFDLEtBQUssRUFBQzs7QUFFaEIsV0FBRyxJQUFJLENBQUMsdUJBQXVCLEVBQUUsS0FBSyxJQUFJLEVBQUM7QUFDekMsZ0JBQU8sSUFBSSxDQUFDLGFBQWEsRUFBRSxDQUFDLElBQUksQ0FBQyxJQUFJLENBQUMsb0JBQW9CLENBQUMsQ0FBQztRQUM3RDs7QUFFRCxjQUFPLENBQUMsQ0FBQyxRQUFRLEVBQUUsQ0FBQyxPQUFPLENBQUMsUUFBUSxDQUFDLENBQUM7TUFDdkM7O0FBRUQsWUFBTyxDQUFDLENBQUMsUUFBUSxFQUFFLENBQUMsTUFBTSxFQUFFLENBQUM7SUFDOUI7O0FBRUQsU0FBTSxvQkFBRTtBQUNOLFNBQUksQ0FBQyxtQkFBbUIsRUFBRSxDQUFDO0FBQzNCLFlBQU8sQ0FBQyxLQUFLLEVBQUUsQ0FBQztBQUNoQixZQUFPLENBQUMsVUFBVSxFQUFFLENBQUMsT0FBTyxDQUFDLEVBQUMsUUFBUSxFQUFFLEdBQUcsQ0FBQyxNQUFNLENBQUMsS0FBSyxFQUFDLENBQUMsQ0FBQztJQUM1RDs7QUFFRCx1QkFBb0Isa0NBQUU7QUFDcEIsd0JBQW1CLEdBQUcsV0FBVyxDQUFDLElBQUksQ0FBQyxhQUFhLEVBQUUsV0FBVyxDQUFDLENBQUM7SUFDcEU7O0FBRUQsc0JBQW1CLGlDQUFFO0FBQ25CLGtCQUFhLENBQUMsbUJBQW1CLENBQUMsQ0FBQztBQUNuQyx3QkFBbUIsR0FBRyxJQUFJLENBQUM7SUFDNUI7O0FBRUQsMEJBQXVCLHFDQUFFO0FBQ3ZCLFlBQU8sbUJBQW1CLENBQUM7SUFDNUI7O0FBRUQsZ0JBQWEsMkJBQUU7QUFDYixZQUFPLEdBQUcsQ0FBQyxJQUFJLENBQUMsR0FBRyxDQUFDLEdBQUcsQ0FBQyxjQUFjLENBQUMsQ0FBQyxJQUFJLENBQUMsY0FBSSxFQUFFO0FBQ2pELGNBQU8sQ0FBQyxXQUFXLENBQUMsSUFBSSxDQUFDLENBQUM7QUFDMUIsY0FBTyxJQUFJLENBQUM7TUFDYixDQUFDLENBQUMsSUFBSSxDQUFDLFlBQUk7QUFDVixXQUFJLENBQUMsTUFBTSxFQUFFLENBQUM7TUFDZixDQUFDLENBQUM7SUFDSjs7QUFFRCxTQUFNLGtCQUFDLElBQUksRUFBRSxRQUFRLEVBQUUsS0FBSyxFQUFDO0FBQzNCLFNBQUksSUFBSSxHQUFHO0FBQ1QsV0FBSSxFQUFFLElBQUk7QUFDVixXQUFJLEVBQUUsUUFBUTtBQUNkLDBCQUFtQixFQUFFLEtBQUs7TUFDM0IsQ0FBQzs7QUFFRixZQUFPLEdBQUcsQ0FBQyxJQUFJLENBQUMsR0FBRyxDQUFDLEdBQUcsQ0FBQyxXQUFXLEVBQUUsSUFBSSxFQUFFLEtBQUssQ0FBQyxDQUFDLElBQUksQ0FBQyxjQUFJLEVBQUU7QUFDM0QsY0FBTyxDQUFDLFdBQVcsQ0FBQyxJQUFJLENBQUMsQ0FBQztBQUMxQixjQUFPLElBQUksQ0FBQztNQUNiLENBQUMsQ0FBQztJQUNKO0VBQ0Y7O0FBRUQsT0FBTSxDQUFDLE9BQU8sR0FBRyxJQUFJLEM7Ozs7Ozs7OztBQ2xGckIsS0FBSSxNQUFNLEdBQUcsbUJBQU8sQ0FBQyxDQUFRLENBQUMsQ0FBQzs7QUFFL0IsT0FBTSxDQUFDLE9BQU8sQ0FBQyxVQUFVLEdBQUcsWUFBNEI7T0FBbkIsS0FBSyx5REFBRyxJQUFJLElBQUksRUFBRTs7QUFDckQsT0FBSSxTQUFTLEdBQUcsTUFBTSxDQUFDLEtBQUssQ0FBQyxDQUFDLE9BQU8sQ0FBQyxPQUFPLENBQUMsQ0FBQyxNQUFNLEVBQUUsQ0FBQztBQUN4RCxPQUFJLE9BQU8sR0FBRyxNQUFNLENBQUMsS0FBSyxDQUFDLENBQUMsS0FBSyxDQUFDLE9BQU8sQ0FBQyxDQUFDLE1BQU0sRUFBRSxDQUFDO0FBQ3BELFVBQU8sQ0FBQyxTQUFTLEVBQUUsT0FBTyxDQUFDLENBQUM7RUFDN0IsQzs7Ozs7Ozs7O0FDTkQsT0FBTSxDQUFDLE9BQU8sQ0FBQyxPQUFPLEdBQUcsVUFBUyxHQUFHLEVBQUUsV0FBVyxFQUFFLElBQXFCLEVBQUU7T0FBdEIsZUFBZSxHQUFoQixJQUFxQixDQUFwQixlQUFlO09BQUUsRUFBRSxHQUFwQixJQUFxQixDQUFILEVBQUU7O0FBQ3RFLGNBQVcsR0FBRyxXQUFXLENBQUMsaUJBQWlCLEVBQUUsQ0FBQztBQUM5QyxPQUFJLFNBQVMsR0FBRyxlQUFlLElBQUksTUFBTSxDQUFDLG1CQUFtQixDQUFDLEdBQUcsQ0FBQyxDQUFDO0FBQ25FLFFBQUssSUFBSSxDQUFDLEdBQUcsQ0FBQyxFQUFFLENBQUMsR0FBRyxTQUFTLENBQUMsTUFBTSxFQUFFLENBQUMsRUFBRSxFQUFFO0FBQ3pDLFNBQUksV0FBVyxHQUFHLEdBQUcsQ0FBQyxTQUFTLENBQUMsQ0FBQyxDQUFDLENBQUMsQ0FBQztBQUNwQyxTQUFJLFdBQVcsRUFBRTtBQUNmLFdBQUcsT0FBTyxFQUFFLEtBQUssVUFBVSxFQUFDO0FBQzFCLGFBQUksTUFBTSxHQUFHLEVBQUUsQ0FBQyxXQUFXLEVBQUUsV0FBVyxFQUFFLFNBQVMsQ0FBQyxDQUFDLENBQUMsQ0FBQyxDQUFDO0FBQ3hELGFBQUcsTUFBTSxLQUFLLElBQUksRUFBQztBQUNqQixrQkFBTyxNQUFNLENBQUM7VUFDZjtRQUNGOztBQUVELFdBQUksV0FBVyxDQUFDLFFBQVEsRUFBRSxDQUFDLGlCQUFpQixFQUFFLENBQUMsT0FBTyxDQUFDLFdBQVcsQ0FBQyxLQUFLLENBQUMsQ0FBQyxFQUFFO0FBQzFFLGdCQUFPLElBQUksQ0FBQztRQUNiO01BQ0Y7SUFDRjs7QUFFRCxVQUFPLEtBQUssQ0FBQztFQUNkLEM7Ozs7Ozs7Ozs7Ozs7OztBQ3BCRCxLQUFJLFlBQVksR0FBRyxtQkFBTyxDQUFDLEdBQVEsQ0FBQyxDQUFDLFlBQVksQ0FBQztBQUNsRCxLQUFJLE9BQU8sR0FBRyxtQkFBTyxDQUFDLEVBQWEsQ0FBQyxDQUFDO0FBQ3JDLEtBQUksR0FBRyxHQUFHLG1CQUFPLENBQUMsRUFBWSxDQUFDLENBQUM7O2dCQUNoQixtQkFBTyxDQUFDLEVBQTZCLENBQUM7O0tBQWpELE9BQU8sWUFBUCxPQUFPOztLQUVOLEdBQUc7YUFBSCxHQUFHOztBQUVJLFlBRlAsR0FBRyxDQUVLLElBQW1DLEVBQUM7U0FBbkMsUUFBUSxHQUFULElBQW1DLENBQWxDLFFBQVE7U0FBRSxLQUFLLEdBQWhCLElBQW1DLENBQXhCLEtBQUs7U0FBRSxHQUFHLEdBQXJCLElBQW1DLENBQWpCLEdBQUc7U0FBRSxJQUFJLEdBQTNCLElBQW1DLENBQVosSUFBSTtTQUFFLElBQUksR0FBakMsSUFBbUMsQ0FBTixJQUFJOzsyQkFGekMsR0FBRzs7QUFHTCw2QkFBTyxDQUFDO0FBQ1IsU0FBSSxDQUFDLE9BQU8sR0FBRyxFQUFFLFFBQVEsRUFBUixRQUFRLEVBQUUsS0FBSyxFQUFMLEtBQUssRUFBRSxHQUFHLEVBQUgsR0FBRyxFQUFFLElBQUksRUFBSixJQUFJLEVBQUUsSUFBSSxFQUFKLElBQUksRUFBRSxDQUFDO0FBQ3BELFNBQUksQ0FBQyxNQUFNLEdBQUcsSUFBSSxDQUFDO0lBQ3BCOztBQU5HLE1BQUcsV0FRUCxVQUFVLHlCQUFFO0FBQ1YsU0FBSSxDQUFDLE1BQU0sQ0FBQyxLQUFLLEVBQUUsQ0FBQztJQUNyQjs7QUFWRyxNQUFHLFdBWVAsU0FBUyxzQkFBQyxPQUFPLEVBQUM7QUFDaEIsU0FBSSxDQUFDLFVBQVUsRUFBRSxDQUFDO0FBQ2xCLFNBQUksQ0FBQyxNQUFNLENBQUMsTUFBTSxHQUFHLElBQUksQ0FBQztBQUMxQixTQUFJLENBQUMsTUFBTSxDQUFDLFNBQVMsR0FBRyxJQUFJLENBQUM7QUFDN0IsU0FBSSxDQUFDLE1BQU0sQ0FBQyxPQUFPLEdBQUcsSUFBSSxDQUFDOztBQUUzQixTQUFJLENBQUMsT0FBTyxDQUFDLE9BQU8sQ0FBQyxDQUFDO0lBQ3ZCOztBQW5CRyxNQUFHLFdBcUJQLE9BQU8sb0JBQUMsT0FBTyxFQUFDOzs7QUFDZCxXQUFNLENBQUMsTUFBTSxDQUFDLElBQUksQ0FBQyxPQUFPLEVBQUUsT0FBTyxDQUFDLENBQUM7O2dDQUV2QixPQUFPLENBQUMsV0FBVyxFQUFFOztTQUE5QixLQUFLLHdCQUFMLEtBQUs7O0FBQ1YsU0FBSSxPQUFPLEdBQUcsR0FBRyxDQUFDLEdBQUcsQ0FBQyxhQUFhLFlBQUUsS0FBSyxFQUFMLEtBQUssSUFBSyxJQUFJLENBQUMsT0FBTyxFQUFFLENBQUM7O0FBRTlELFNBQUksQ0FBQyxNQUFNLEdBQUcsSUFBSSxTQUFTLENBQUMsT0FBTyxFQUFFLE9BQU8sQ0FBQyxDQUFDOztBQUU5QyxTQUFJLENBQUMsTUFBTSxDQUFDLE1BQU0sR0FBRyxZQUFNO0FBQ3pCLGFBQUssSUFBSSxDQUFDLE1BQU0sQ0FBQyxDQUFDO01BQ25COztBQUVELFNBQUksQ0FBQyxNQUFNLENBQUMsU0FBUyxHQUFHLFVBQUMsQ0FBQyxFQUFHO0FBQzNCLGFBQUssSUFBSSxDQUFDLE1BQU0sRUFBRSxDQUFDLENBQUMsSUFBSSxDQUFDLENBQUM7TUFDM0I7O0FBRUQsU0FBSSxDQUFDLE1BQU0sQ0FBQyxPQUFPLEdBQUcsWUFBSTtBQUN4QixhQUFLLElBQUksQ0FBQyxPQUFPLENBQUMsQ0FBQztNQUNwQjtJQUNGOztBQXhDRyxNQUFHLFdBMENQLE1BQU0sbUJBQUMsSUFBSSxFQUFFLElBQUksRUFBQztBQUNoQixZQUFPLENBQUMsTUFBTSxDQUFDLElBQUksRUFBRSxJQUFJLENBQUMsQ0FBQztJQUM1Qjs7QUE1Q0csTUFBRyxXQThDUCxJQUFJLGlCQUFDLElBQUksRUFBQztBQUNSLFNBQUksQ0FBQyxNQUFNLENBQUMsSUFBSSxDQUFDLElBQUksQ0FBQyxDQUFDO0lBQ3hCOztVQWhERyxHQUFHO0lBQVMsWUFBWTs7QUFtRDlCLE9BQU0sQ0FBQyxPQUFPLEdBQUcsR0FBRyxDOzs7Ozs7Ozs7Ozs7O3NDQ3hERSxFQUFXOzs7O3NCQUVsQix1QkFBVTtBQUN2QixpQkFBYyxFQUFFLElBQUk7QUFDcEIsa0JBQWUsRUFBRSxJQUFJO0FBQ3JCLDBCQUF1QixFQUFFLElBQUk7RUFDOUIsQ0FBQzs7Ozs7Ozs7Ozs7O2dCQ04yQixtQkFBTyxDQUFDLEVBQVksQ0FBQzs7S0FBNUMsS0FBSyxZQUFMLEtBQUs7S0FBRSxXQUFXLFlBQVgsV0FBVzs7aUJBQzRDLG1CQUFPLENBQUMsRUFBZSxDQUFDOztLQUF0RixjQUFjLGFBQWQsY0FBYztLQUFFLGVBQWUsYUFBZixlQUFlO0tBQUUsdUJBQXVCLGFBQXZCLHVCQUF1QjtzQkFFL0MsS0FBSyxDQUFDO0FBQ25CLGtCQUFlLDZCQUFHO0FBQ2hCLFlBQU8sV0FBVyxDQUFDLElBQUksQ0FBQyxDQUFDO0lBQzFCOztBQUVELGFBQVUsd0JBQUc7QUFDWCxTQUFJLENBQUMsRUFBRSxDQUFDLGNBQWMsRUFBRSxpQkFBaUIsQ0FBQyxDQUFDO0FBQzNDLFNBQUksQ0FBQyxFQUFFLENBQUMsZUFBZSxFQUFFLEtBQUssQ0FBQyxDQUFDO0FBQ2hDLFNBQUksQ0FBQyxFQUFFLENBQUMsdUJBQXVCLEVBQUUsWUFBWSxDQUFDLENBQUM7SUFDaEQ7RUFDRixDQUFDOztBQUVGLFVBQVMsWUFBWSxDQUFDLEtBQUssRUFBRSxJQUFpQixFQUFDO09BQWpCLFFBQVEsR0FBVCxJQUFpQixDQUFoQixRQUFRO09BQUUsS0FBSyxHQUFoQixJQUFpQixDQUFOLEtBQUs7O0FBQzNDLFVBQU8sS0FBSyxDQUFDLEdBQUcsQ0FBQyxVQUFVLEVBQUUsUUFBUSxDQUFDLENBQ3pCLEdBQUcsQ0FBQyxPQUFPLEVBQUUsS0FBSyxDQUFDLENBQUM7RUFDbEM7O0FBRUQsVUFBUyxLQUFLLEdBQUU7QUFDZCxVQUFPLFdBQVcsQ0FBQyxJQUFJLENBQUMsQ0FBQztFQUMxQjs7QUFFRCxVQUFTLGlCQUFpQixDQUFDLEtBQUssRUFBRSxLQUFvQyxFQUFFO09BQXJDLFFBQVEsR0FBVCxLQUFvQyxDQUFuQyxRQUFRO09BQUUsS0FBSyxHQUFoQixLQUFvQyxDQUF6QixLQUFLO09BQUUsR0FBRyxHQUFyQixLQUFvQyxDQUFsQixHQUFHO09BQUUsWUFBWSxHQUFuQyxLQUFvQyxDQUFiLFlBQVk7O0FBQ25FLFVBQU8sV0FBVyxDQUFDO0FBQ2pCLGFBQVEsRUFBUixRQUFRO0FBQ1IsVUFBSyxFQUFMLEtBQUs7QUFDTCxRQUFHLEVBQUgsR0FBRztBQUNILGlCQUFZLEVBQVosWUFBWTtJQUNiLENBQUMsQ0FBQztFQUNKOzs7Ozs7Ozs7Ozs7OztzQ0MvQnFCLEVBQVc7Ozs7c0JBRWxCLHVCQUFVO0FBQ3ZCLGdCQUFhLEVBQUUsSUFBSTtBQUNuQixrQkFBZSxFQUFFLElBQUk7QUFDckIsaUJBQWMsRUFBRSxJQUFJO0VBQ3JCLENBQUM7Ozs7Ozs7Ozs7OztnQkNOMkIsbUJBQU8sQ0FBQyxFQUFZLENBQUM7O0tBQTVDLEtBQUssWUFBTCxLQUFLO0tBQUUsV0FBVyxZQUFYLFdBQVc7O2lCQUVpQyxtQkFBTyxDQUFDLEdBQWUsQ0FBQzs7S0FBM0UsYUFBYSxhQUFiLGFBQWE7S0FBRSxlQUFlLGFBQWYsZUFBZTtLQUFFLGNBQWMsYUFBZCxjQUFjOztBQUVwRCxLQUFJLFNBQVMsR0FBRyxXQUFXLENBQUM7QUFDMUIsVUFBTyxFQUFFLEtBQUs7QUFDZCxpQkFBYyxFQUFFLEtBQUs7QUFDckIsV0FBUSxFQUFFLEtBQUs7RUFDaEIsQ0FBQyxDQUFDOztzQkFFWSxLQUFLLENBQUM7O0FBRW5CLGtCQUFlLDZCQUFHO0FBQ2hCLFlBQU8sU0FBUyxDQUFDLEdBQUcsQ0FBQyxnQkFBZ0IsRUFBRSxJQUFJLENBQUMsQ0FBQztJQUM5Qzs7QUFFRCxhQUFVLHdCQUFHO0FBQ1gsU0FBSSxDQUFDLEVBQUUsQ0FBQyxhQUFhLEVBQUU7Y0FBSyxTQUFTLENBQUMsR0FBRyxDQUFDLGdCQUFnQixFQUFFLElBQUksQ0FBQztNQUFBLENBQUMsQ0FBQztBQUNuRSxTQUFJLENBQUMsRUFBRSxDQUFDLGNBQWMsRUFBQztjQUFLLFNBQVMsQ0FBQyxHQUFHLENBQUMsU0FBUyxFQUFFLElBQUksQ0FBQztNQUFBLENBQUMsQ0FBQztBQUM1RCxTQUFJLENBQUMsRUFBRSxDQUFDLGVBQWUsRUFBQztjQUFLLFNBQVMsQ0FBQyxHQUFHLENBQUMsVUFBVSxFQUFFLElBQUksQ0FBQztNQUFBLENBQUMsQ0FBQztJQUMvRDtFQUNGLENBQUM7Ozs7Ozs7Ozs7Ozs7O3NDQ3JCb0IsRUFBVzs7OztzQkFFbEIsdUJBQVU7QUFDdkIsK0JBQTRCLEVBQUUsSUFBSTtBQUNsQyxnQ0FBNkIsRUFBRSxJQUFJO0VBQ3BDLENBQUM7Ozs7Ozs7Ozs7OztnQkNMMkIsbUJBQU8sQ0FBQyxFQUFZLENBQUM7O0tBQTVDLEtBQUssWUFBTCxLQUFLO0tBQUUsV0FBVyxZQUFYLFdBQVc7O2lCQUU4QyxtQkFBTyxDQUFDLEdBQWUsQ0FBQzs7S0FBeEYsNEJBQTRCLGFBQTVCLDRCQUE0QjtLQUFFLDZCQUE2QixhQUE3Qiw2QkFBNkI7c0JBRWxELEtBQUssQ0FBQzs7QUFFbkIsa0JBQWUsNkJBQUc7QUFDaEIsWUFBTyxXQUFXLENBQUM7QUFDakIsNkJBQXNCLEVBQUUsS0FBSztNQUM5QixDQUFDLENBQUM7SUFDSjs7QUFFRCxhQUFVLHdCQUFHO0FBQ1gsU0FBSSxDQUFDLEVBQUUsQ0FBQyw0QkFBNEIsRUFBRSxvQkFBb0IsQ0FBQyxDQUFDO0FBQzVELFNBQUksQ0FBQyxFQUFFLENBQUMsNkJBQTZCLEVBQUUscUJBQXFCLENBQUMsQ0FBQztJQUMvRDtFQUNGLENBQUM7O0FBRUYsVUFBUyxvQkFBb0IsQ0FBQyxLQUFLLEVBQUM7QUFDbEMsVUFBTyxLQUFLLENBQUMsR0FBRyxDQUFDLHdCQUF3QixFQUFFLElBQUksQ0FBQyxDQUFDO0VBQ2xEOztBQUVELFVBQVMscUJBQXFCLENBQUMsS0FBSyxFQUFDO0FBQ25DLFVBQU8sS0FBSyxDQUFDLEdBQUcsQ0FBQyx3QkFBd0IsRUFBRSxLQUFLLENBQUMsQ0FBQztFQUNuRDs7Ozs7Ozs7Ozs7Ozs7c0NDeEJxQixFQUFXOzs7O3NCQUVsQix1QkFBVTtBQUN2QiwyQkFBd0IsRUFBRSxJQUFJO0VBQy9CLENBQUM7Ozs7Ozs7Ozs7OztnQkNKMkIsbUJBQU8sQ0FBQyxFQUFZLENBQUM7O0tBQTVDLEtBQUssWUFBTCxLQUFLO0tBQUUsV0FBVyxZQUFYLFdBQVc7O2lCQUNZLG1CQUFPLENBQUMsR0FBZSxDQUFDOztLQUFyRCx3QkFBd0IsYUFBeEIsd0JBQXdCO3NCQUVoQixLQUFLLENBQUM7QUFDbkIsa0JBQWUsNkJBQUc7QUFDaEIsWUFBTyxXQUFXLENBQUMsSUFBSSxDQUFDLENBQUM7SUFDMUI7O0FBRUQsYUFBVSx3QkFBRztBQUNYLFNBQUksQ0FBQyxFQUFFLENBQUMsd0JBQXdCLEVBQUUsYUFBYSxDQUFDO0lBQ2pEO0VBQ0YsQ0FBQzs7QUFFRixVQUFTLGFBQWEsQ0FBQyxLQUFLLEVBQUUsTUFBTSxFQUFDO0FBQ25DLFVBQU8sV0FBVyxDQUFDLE1BQU0sQ0FBQyxDQUFDO0VBQzVCOzs7Ozs7Ozs7Ozs7OztzQ0NmcUIsRUFBVzs7OztzQkFFbEIsdUJBQVU7QUFDdkIscUJBQWtCLEVBQUUsSUFBSTtFQUN6QixDQUFDOzs7Ozs7Ozs7OztBQ0pGLEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsQ0FBYSxDQUFDLENBQUM7O2dCQUNQLG1CQUFPLENBQUMsR0FBZSxDQUFDOztLQUFoRCxrQkFBa0IsWUFBbEIsa0JBQWtCOztBQUN4QixLQUFJLEdBQUcsR0FBRyxtQkFBTyxDQUFDLEVBQWtCLENBQUMsQ0FBQztBQUN0QyxLQUFJLEdBQUcsR0FBRyxtQkFBTyxDQUFDLEVBQVksQ0FBQyxDQUFDOztzQkFFakI7QUFDYixhQUFVLHdCQUFFO0FBQ1YsUUFBRyxDQUFDLEdBQUcsQ0FBQyxHQUFHLENBQUMsR0FBRyxDQUFDLFNBQVMsQ0FBQyxDQUFDLElBQUksQ0FBQyxZQUFXO1dBQVYsSUFBSSx5REFBQyxFQUFFOztBQUN0QyxXQUFJLFNBQVMsR0FBRyxJQUFJLENBQUMsS0FBSyxDQUFDLEdBQUcsQ0FBQyxjQUFJO2dCQUFFLElBQUksQ0FBQyxJQUFJO1FBQUEsQ0FBQyxDQUFDO0FBQ2hELGNBQU8sQ0FBQyxRQUFRLENBQUMsa0JBQWtCLEVBQUUsU0FBUyxDQUFDLENBQUM7TUFDakQsQ0FBQyxDQUFDO0lBQ0o7RUFDRjs7Ozs7Ozs7Ozs7O2dCQ1o0QixtQkFBTyxDQUFDLEVBQVksQ0FBQzs7S0FBNUMsS0FBSyxZQUFMLEtBQUs7S0FBRSxXQUFXLFlBQVgsV0FBVzs7aUJBQ00sbUJBQU8sQ0FBQyxHQUFlLENBQUM7O0tBQS9DLGtCQUFrQixhQUFsQixrQkFBa0I7c0JBRVYsS0FBSyxDQUFDO0FBQ25CLGtCQUFlLDZCQUFHO0FBQ2hCLFlBQU8sV0FBVyxDQUFDLEVBQUUsQ0FBQyxDQUFDO0lBQ3hCOztBQUVELGFBQVUsd0JBQUc7QUFDWCxTQUFJLENBQUMsRUFBRSxDQUFDLGtCQUFrQixFQUFFLFlBQVksQ0FBQztJQUMxQztFQUNGLENBQUM7O0FBRUYsVUFBUyxZQUFZLENBQUMsS0FBSyxFQUFFLFNBQVMsRUFBQztBQUNyQyxVQUFPLFdBQVcsQ0FBQyxTQUFTLENBQUMsQ0FBQztFQUMvQjs7Ozs7Ozs7Ozs7Ozs7c0NDZnFCLEVBQVc7Ozs7c0JBRWxCLHVCQUFVO0FBQ3ZCLHNCQUFtQixFQUFFLElBQUk7QUFDekIsd0JBQXFCLEVBQUUsSUFBSTtBQUMzQixxQkFBa0IsRUFBRSxJQUFJO0VBQ3pCLENBQUM7Ozs7Ozs7Ozs7O0FDTkYsS0FBSSxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxDQUFhLENBQUMsQ0FBQzs7Z0JBS1osbUJBQU8sQ0FBQyxHQUFlLENBQUM7O0tBRi9DLG1CQUFtQixZQUFuQixtQkFBbUI7S0FDbkIscUJBQXFCLFlBQXJCLHFCQUFxQjtLQUNyQixrQkFBa0IsWUFBbEIsa0JBQWtCO3NCQUVMOztBQUViLFFBQUssaUJBQUMsT0FBTyxFQUFDO0FBQ1osWUFBTyxDQUFDLFFBQVEsQ0FBQyxtQkFBbUIsRUFBRSxFQUFDLElBQUksRUFBRSxPQUFPLEVBQUMsQ0FBQyxDQUFDO0lBQ3hEOztBQUVELE9BQUksZ0JBQUMsT0FBTyxFQUFFLE9BQU8sRUFBQztBQUNwQixZQUFPLENBQUMsUUFBUSxDQUFDLGtCQUFrQixFQUFHLEVBQUMsSUFBSSxFQUFFLE9BQU8sRUFBRSxPQUFPLEVBQVAsT0FBTyxFQUFDLENBQUMsQ0FBQztJQUNqRTs7QUFFRCxVQUFPLG1CQUFDLE9BQU8sRUFBQztBQUNkLFlBQU8sQ0FBQyxRQUFRLENBQUMscUJBQXFCLEVBQUUsRUFBQyxJQUFJLEVBQUUsT0FBTyxFQUFDLENBQUMsQ0FBQztJQUMxRDs7RUFFRjs7Ozs7Ozs7Ozs7QUNyQkQsS0FBSSxVQUFVLEdBQUc7QUFDZixlQUFZLEVBQUUsS0FBSztBQUNuQixVQUFPLEVBQUUsS0FBSztBQUNkLFlBQVMsRUFBRSxLQUFLO0FBQ2hCLFVBQU8sRUFBRSxFQUFFO0VBQ1o7O0FBRUQsS0FBTSxhQUFhLEdBQUcsU0FBaEIsYUFBYSxDQUFJLE9BQU87VUFBTSxDQUFFLENBQUMsZUFBZSxFQUFFLE9BQU8sQ0FBQyxFQUFFLFVBQUMsTUFBTSxFQUFLO0FBQzVFLFlBQU8sTUFBTSxHQUFHLE1BQU0sQ0FBQyxJQUFJLEVBQUUsR0FBRyxVQUFVLENBQUM7SUFDM0MsQ0FDRDtFQUFBLENBQUM7O3NCQUVhLEVBQUcsYUFBYSxFQUFiLGFBQWEsRUFBRzs7Ozs7Ozs7Ozs7Ozs7c0NDWlosRUFBVzs7OztzQkFFbEIsdUJBQVU7QUFDdkIsdUJBQW9CLEVBQUUsSUFBSTtBQUMxQixzQkFBbUIsRUFBRSxJQUFJO0VBQzFCLENBQUM7Ozs7Ozs7Ozs7OztnQkNMb0IsbUJBQU8sQ0FBQyxFQUFZLENBQUM7O0tBQXJDLFdBQVcsWUFBWCxXQUFXOztBQUNqQixLQUFJLE9BQU8sR0FBRyxtQkFBTyxDQUFDLENBQWEsQ0FBQyxDQUFDO0FBQ3JDLEtBQUksR0FBRyxHQUFHLG1CQUFPLENBQUMsRUFBWSxDQUFDLENBQUM7O0FBRWhDLEtBQU0sZ0JBQWdCLEdBQUcsU0FBbkIsZ0JBQWdCLENBQUksUUFBUTtVQUFLLENBQUMsQ0FBQyxlQUFlLENBQUMsRUFBRSxVQUFDLFFBQVEsRUFBSTtBQUN0RSxZQUFPLFFBQVEsQ0FBQyxRQUFRLEVBQUUsQ0FBQyxNQUFNLENBQUMsY0FBSSxFQUFFO0FBQ3RDLFdBQUksT0FBTyxHQUFHLElBQUksQ0FBQyxHQUFHLENBQUMsU0FBUyxDQUFDLElBQUksV0FBVyxDQUFDLEVBQUUsQ0FBQyxDQUFDO0FBQ3JELFdBQUksU0FBUyxHQUFHLE9BQU8sQ0FBQyxJQUFJLENBQUMsZUFBSztnQkFBRyxLQUFLLENBQUMsR0FBRyxDQUFDLFdBQVcsQ0FBQyxLQUFLLFFBQVE7UUFBQSxDQUFDLENBQUM7QUFDMUUsY0FBTyxTQUFTLENBQUM7TUFDbEIsQ0FBQyxDQUFDLE1BQU0sRUFBRSxDQUFDO0lBQ2IsQ0FBQztFQUFBOztBQUVGLEtBQU0sWUFBWSxHQUFHLENBQUMsQ0FBQyxlQUFlLENBQUMsRUFBRSxVQUFDLFFBQVEsRUFBSTtBQUNwRCxVQUFPLFFBQVEsQ0FBQyxRQUFRLEVBQUUsQ0FBQyxHQUFHLENBQUMsVUFBVSxDQUFDLENBQUMsSUFBSSxFQUFFLENBQUM7RUFDbkQsQ0FBQyxDQUFDOztBQUVILEtBQU0sZUFBZSxHQUFHLFNBQWxCLGVBQWUsQ0FBSSxHQUFHO1VBQUksQ0FBQyxDQUFDLGVBQWUsRUFBRSxHQUFHLENBQUMsRUFBRSxVQUFDLE9BQU8sRUFBRztBQUNsRSxTQUFHLENBQUMsT0FBTyxFQUFDO0FBQ1YsY0FBTyxJQUFJLENBQUM7TUFDYjs7QUFFRCxZQUFPLFVBQVUsQ0FBQyxPQUFPLENBQUMsQ0FBQztJQUM1QixDQUFDO0VBQUEsQ0FBQzs7QUFFSCxLQUFNLGtCQUFrQixHQUFHLFNBQXJCLGtCQUFrQixDQUFJLEdBQUc7VUFDOUIsQ0FBQyxDQUFDLGVBQWUsRUFBRSxHQUFHLEVBQUUsU0FBUyxDQUFDLEVBQUUsVUFBQyxPQUFPLEVBQUk7O0FBRS9DLFNBQUcsQ0FBQyxPQUFPLEVBQUM7QUFDVixjQUFPLEVBQUUsQ0FBQztNQUNYOztBQUVELFNBQUksaUJBQWlCLEdBQUcsaUJBQWlCLENBQUMsT0FBTyxDQUFDLENBQUMsR0FBRyxDQUFDLE1BQU0sQ0FBQyxDQUFDOztBQUUvRCxZQUFPLE9BQU8sQ0FBQyxHQUFHLENBQUMsY0FBSSxFQUFFO0FBQ3ZCLFdBQUksSUFBSSxHQUFHLElBQUksQ0FBQyxHQUFHLENBQUMsTUFBTSxDQUFDLENBQUM7QUFDNUIsY0FBTztBQUNMLGFBQUksRUFBRSxJQUFJLENBQUMsR0FBRyxDQUFDLE1BQU0sQ0FBQztBQUN0QixpQkFBUSxFQUFFLElBQUksQ0FBQyxHQUFHLENBQUMsYUFBYSxDQUFDO0FBQ2pDLGlCQUFRLEVBQUUsSUFBSSxDQUFDLEdBQUcsQ0FBQyxXQUFXLENBQUM7QUFDL0IsaUJBQVEsRUFBRSxpQkFBaUIsS0FBSyxJQUFJO1FBQ3JDO01BQ0YsQ0FBQyxDQUFDLElBQUksRUFBRSxDQUFDO0lBQ1gsQ0FBQztFQUFBLENBQUM7O0FBRUgsVUFBUyxpQkFBaUIsQ0FBQyxPQUFPLEVBQUM7QUFDakMsVUFBTyxPQUFPLENBQUMsTUFBTSxDQUFDLGNBQUk7WUFBRyxJQUFJLElBQUksQ0FBQyxJQUFJLENBQUMsR0FBRyxDQUFDLFlBQVksQ0FBQyxDQUFDO0lBQUEsQ0FBQyxDQUFDLEtBQUssRUFBRSxDQUFDO0VBQ3hFOztBQUVELFVBQVMsVUFBVSxDQUFDLE9BQU8sRUFBQztBQUMxQixPQUFJLEdBQUcsR0FBRyxPQUFPLENBQUMsR0FBRyxDQUFDLElBQUksQ0FBQyxDQUFDO0FBQzVCLE9BQUksUUFBUSxFQUFFLFFBQVEsQ0FBQztBQUN2QixPQUFJLE9BQU8sR0FBRyxPQUFPLENBQUMsUUFBUSxDQUFDLGtCQUFrQixDQUFDLEdBQUcsQ0FBQyxDQUFDLENBQUM7O0FBRXhELE9BQUcsT0FBTyxDQUFDLE1BQU0sR0FBRyxDQUFDLEVBQUM7QUFDcEIsYUFBUSxHQUFHLE9BQU8sQ0FBQyxDQUFDLENBQUMsQ0FBQyxRQUFRLENBQUM7QUFDL0IsYUFBUSxHQUFHLE9BQU8sQ0FBQyxDQUFDLENBQUMsQ0FBQyxRQUFRLENBQUM7SUFDaEM7O0FBRUQsVUFBTztBQUNMLFFBQUcsRUFBRSxHQUFHO0FBQ1IsZUFBVSxFQUFFLEdBQUcsQ0FBQyx3QkFBd0IsQ0FBQyxHQUFHLENBQUM7QUFDN0MsYUFBUSxFQUFSLFFBQVE7QUFDUixhQUFRLEVBQVIsUUFBUTtBQUNSLFdBQU0sRUFBRSxPQUFPLENBQUMsR0FBRyxDQUFDLFFBQVEsQ0FBQztBQUM3QixZQUFPLEVBQUUsSUFBSSxJQUFJLENBQUMsT0FBTyxDQUFDLEdBQUcsQ0FBQyxTQUFTLENBQUMsQ0FBQztBQUN6QyxlQUFVLEVBQUUsSUFBSSxJQUFJLENBQUMsT0FBTyxDQUFDLEdBQUcsQ0FBQyxhQUFhLENBQUMsQ0FBQztBQUNoRCxVQUFLLEVBQUUsT0FBTyxDQUFDLEdBQUcsQ0FBQyxPQUFPLENBQUM7QUFDM0IsWUFBTyxFQUFFLE9BQU87QUFDaEIsU0FBSSxFQUFFLE9BQU8sQ0FBQyxLQUFLLENBQUMsQ0FBQyxpQkFBaUIsRUFBRSxHQUFHLENBQUMsQ0FBQztBQUM3QyxTQUFJLEVBQUUsT0FBTyxDQUFDLEtBQUssQ0FBQyxDQUFDLGlCQUFpQixFQUFFLEdBQUcsQ0FBQyxDQUFDO0lBQzlDO0VBQ0Y7O3NCQUVjO0FBQ2IscUJBQWtCLEVBQWxCLGtCQUFrQjtBQUNsQixtQkFBZ0IsRUFBaEIsZ0JBQWdCO0FBQ2hCLGVBQVksRUFBWixZQUFZO0FBQ1osa0JBQWUsRUFBZixlQUFlO0FBQ2YsYUFBVSxFQUFWLFVBQVU7RUFDWDs7Ozs7Ozs7Ozs7O2dCQy9FNEIsbUJBQU8sQ0FBQyxFQUFZLENBQUM7O0tBQTVDLEtBQUssWUFBTCxLQUFLO0tBQUUsV0FBVyxZQUFYLFdBQVc7O2lCQUM2QixtQkFBTyxDQUFDLEdBQWUsQ0FBQzs7S0FBdkUsb0JBQW9CLGFBQXBCLG9CQUFvQjtLQUFFLG1CQUFtQixhQUFuQixtQkFBbUI7c0JBRWhDLEtBQUssQ0FBQztBQUNuQixrQkFBZSw2QkFBRztBQUNoQixZQUFPLFdBQVcsQ0FBQyxFQUFFLENBQUMsQ0FBQztJQUN4Qjs7QUFFRCxhQUFVLHdCQUFHO0FBQ1gsU0FBSSxDQUFDLEVBQUUsQ0FBQyxvQkFBb0IsRUFBRSxlQUFlLENBQUMsQ0FBQztBQUMvQyxTQUFJLENBQUMsRUFBRSxDQUFDLG1CQUFtQixFQUFFLGFBQWEsQ0FBQyxDQUFDO0lBQzdDO0VBQ0YsQ0FBQzs7QUFFRixVQUFTLGFBQWEsQ0FBQyxLQUFLLEVBQUUsSUFBSSxFQUFDO0FBQ2pDLFVBQU8sS0FBSyxDQUFDLEdBQUcsQ0FBQyxJQUFJLENBQUMsRUFBRSxFQUFFLFdBQVcsQ0FBQyxJQUFJLENBQUMsQ0FBQyxDQUFDO0VBQzlDOztBQUVELFVBQVMsZUFBZSxDQUFDLEtBQUssRUFBZTtPQUFiLFNBQVMseURBQUMsRUFBRTs7QUFDMUMsVUFBTyxLQUFLLENBQUMsYUFBYSxDQUFDLGVBQUssRUFBSTtBQUNsQyxjQUFTLENBQUMsT0FBTyxDQUFDLFVBQUMsSUFBSSxFQUFLO0FBQzFCLFlBQUssQ0FBQyxHQUFHLENBQUMsSUFBSSxDQUFDLEVBQUUsRUFBRSxXQUFXLENBQUMsSUFBSSxDQUFDLENBQUM7TUFDdEMsQ0FBQztJQUNILENBQUMsQ0FBQztFQUNKOzs7Ozs7Ozs7Ozs7OztzQ0N4QnFCLEVBQVc7Ozs7c0JBRWxCLHVCQUFVO0FBQ3ZCLG9CQUFpQixFQUFFLElBQUk7RUFDeEIsQ0FBQzs7Ozs7Ozs7Ozs7QUNKRixLQUFJLE9BQU8sR0FBRyxtQkFBTyxDQUFDLENBQWEsQ0FBQyxDQUFDOztnQkFDVCxtQkFBTyxDQUFDLEdBQWUsQ0FBQzs7S0FBOUMsaUJBQWlCLFlBQWpCLGlCQUFpQjs7aUJBQ3FCLG1CQUFPLENBQUMsRUFBK0IsQ0FBQzs7S0FBOUUsaUJBQWlCLGFBQWpCLGlCQUFpQjtLQUFFLGVBQWUsYUFBZixlQUFlOztBQUN4QyxLQUFJLGNBQWMsR0FBRyxtQkFBTyxDQUFDLEdBQTZCLENBQUMsQ0FBQztBQUM1RCxLQUFJLElBQUksR0FBRyxtQkFBTyxDQUFDLEVBQVUsQ0FBQyxDQUFDO0FBQy9CLEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsRUFBYSxDQUFDLENBQUM7QUFDckMsS0FBSSxHQUFHLEdBQUcsbUJBQU8sQ0FBQyxFQUFZLENBQUMsQ0FBQzs7c0JBRWpCOztBQUViLGFBQVUsc0JBQUMsU0FBUyxFQUFFLE9BQU8sRUFBRSxFQUFFLEVBQUM7QUFDaEMsU0FBSSxDQUFDLFVBQVUsRUFBRSxDQUNkLElBQUksQ0FBQyxVQUFDLFFBQVEsRUFBSTtBQUNqQixjQUFPLENBQUMsUUFBUSxDQUFDLGlCQUFpQixFQUFFLFFBQVEsQ0FBQyxJQUFJLENBQUUsQ0FBQztBQUNwRCxTQUFFLEVBQUUsQ0FBQztNQUNOLENBQUMsQ0FDRCxJQUFJLENBQUMsWUFBSTtBQUNSLGNBQU8sQ0FBQyxFQUFDLFVBQVUsRUFBRSxTQUFTLENBQUMsUUFBUSxDQUFDLFFBQVEsRUFBRSxFQUFFLEdBQUcsQ0FBQyxNQUFNLENBQUMsS0FBSyxDQUFDLENBQUM7QUFDdEUsU0FBRSxFQUFFLENBQUM7TUFDTixDQUFDLENBQUM7SUFDTjs7QUFFRCxTQUFNLGtCQUFDLElBQStCLEVBQUM7U0FBL0IsSUFBSSxHQUFMLElBQStCLENBQTlCLElBQUk7U0FBRSxHQUFHLEdBQVYsSUFBK0IsQ0FBeEIsR0FBRztTQUFFLEtBQUssR0FBakIsSUFBK0IsQ0FBbkIsS0FBSztTQUFFLFdBQVcsR0FBOUIsSUFBK0IsQ0FBWixXQUFXOztBQUNuQyxtQkFBYyxDQUFDLEtBQUssQ0FBQyxpQkFBaUIsQ0FBQyxDQUFDO0FBQ3hDLFNBQUksQ0FBQyxNQUFNLENBQUMsSUFBSSxFQUFFLEdBQUcsRUFBRSxLQUFLLEVBQUUsV0FBVyxDQUFDLENBQ3ZDLElBQUksQ0FBQyxVQUFDLFdBQVcsRUFBRztBQUNuQixjQUFPLENBQUMsUUFBUSxDQUFDLGlCQUFpQixFQUFFLFdBQVcsQ0FBQyxJQUFJLENBQUMsQ0FBQztBQUN0RCxxQkFBYyxDQUFDLE9BQU8sQ0FBQyxpQkFBaUIsQ0FBQyxDQUFDO0FBQzFDLGNBQU8sQ0FBQyxVQUFVLEVBQUUsQ0FBQyxJQUFJLENBQUMsRUFBQyxRQUFRLEVBQUUsR0FBRyxDQUFDLE1BQU0sQ0FBQyxHQUFHLEVBQUMsQ0FBQyxDQUFDO01BQ3ZELENBQUMsQ0FDRCxJQUFJLENBQUMsVUFBQyxHQUFHLEVBQUc7QUFDWCxxQkFBYyxDQUFDLElBQUksQ0FBQyxpQkFBaUIsRUFBRSxHQUFHLENBQUMsWUFBWSxDQUFDLE9BQU8sSUFBSSxtQkFBbUIsQ0FBQyxDQUFDO01BQ3pGLENBQUMsQ0FBQztJQUNOOztBQUVELFFBQUssaUJBQUMsS0FBdUIsRUFBRSxRQUFRLEVBQUM7U0FBakMsSUFBSSxHQUFMLEtBQXVCLENBQXRCLElBQUk7U0FBRSxRQUFRLEdBQWYsS0FBdUIsQ0FBaEIsUUFBUTtTQUFFLEtBQUssR0FBdEIsS0FBdUIsQ0FBTixLQUFLOztBQUMxQixtQkFBYyxDQUFDLEtBQUssQ0FBQyxlQUFlLENBQUMsQ0FBQztBQUN0QyxTQUFJLENBQUMsS0FBSyxDQUFDLElBQUksRUFBRSxRQUFRLEVBQUUsS0FBSyxDQUFDLENBQzlCLElBQUksQ0FBQyxVQUFDLFdBQVcsRUFBRztBQUNuQixxQkFBYyxDQUFDLE9BQU8sQ0FBQyxlQUFlLENBQUMsQ0FBQztBQUN4QyxjQUFPLENBQUMsUUFBUSxDQUFDLGlCQUFpQixFQUFFLFdBQVcsQ0FBQyxJQUFJLENBQUMsQ0FBQztBQUN0RCxjQUFPLENBQUMsVUFBVSxFQUFFLENBQUMsSUFBSSxDQUFDLEVBQUMsUUFBUSxFQUFFLFFBQVEsRUFBQyxDQUFDLENBQUM7TUFDakQsQ0FBQyxDQUNELElBQUksQ0FBQyxVQUFDLEdBQUc7Y0FBSSxjQUFjLENBQUMsSUFBSSxDQUFDLGVBQWUsRUFBRSxHQUFHLENBQUMsWUFBWSxDQUFDLE9BQU8sQ0FBQztNQUFBLENBQUM7SUFDOUU7RUFDSjs7Ozs7Ozs7OztBQzdDRCxPQUFNLENBQUMsT0FBTyxDQUFDLE9BQU8sR0FBRyxtQkFBTyxDQUFDLEVBQVcsQ0FBQyxDQUFDO0FBQzlDLE9BQU0sQ0FBQyxPQUFPLENBQUMsT0FBTyxHQUFHLG1CQUFPLENBQUMsR0FBVyxDQUFDLENBQUM7QUFDOUMsT0FBTSxDQUFDLE9BQU8sQ0FBQyxTQUFTLEdBQUcsbUJBQU8sQ0FBQyxHQUFhLENBQUMsQzs7Ozs7Ozs7Ozs7Z0JDRnBCLG1CQUFPLENBQUMsRUFBWSxDQUFDOztLQUE1QyxLQUFLLFlBQUwsS0FBSztLQUFFLFdBQVcsWUFBWCxXQUFXOztpQkFDSyxtQkFBTyxDQUFDLEdBQWUsQ0FBQzs7S0FBOUMsaUJBQWlCLGFBQWpCLGlCQUFpQjtzQkFFVCxLQUFLLENBQUM7QUFDbkIsa0JBQWUsNkJBQUc7QUFDaEIsWUFBTyxXQUFXLENBQUMsSUFBSSxDQUFDLENBQUM7SUFDMUI7O0FBRUQsYUFBVSx3QkFBRztBQUNYLFNBQUksQ0FBQyxFQUFFLENBQUMsaUJBQWlCLEVBQUUsV0FBVyxDQUFDO0lBQ3hDOztFQUVGLENBQUM7O0FBRUYsVUFBUyxXQUFXLENBQUMsS0FBSyxFQUFFLElBQUksRUFBQztBQUMvQixVQUFPLFdBQVcsQ0FBQyxJQUFJLENBQUMsQ0FBQztFQUMxQjs7Ozs7Ozs7Ozs7O0FDaEJELEtBQUksS0FBSyxHQUFHLG1CQUFPLENBQUMsQ0FBTyxDQUFDLENBQUM7O2dCQUNiLG1CQUFPLENBQUMsRUFBNkIsQ0FBQzs7S0FBakQsT0FBTyxZQUFQLE9BQU87O0FBQ1osS0FBSSxNQUFNLEdBQUcsQ0FBQyxTQUFTLEVBQUUsU0FBUyxFQUFFLFNBQVMsRUFBRSxTQUFTLEVBQUUsU0FBUyxFQUFFLFNBQVMsQ0FBQyxDQUFDOztBQUVoRixLQUFNLFFBQVEsR0FBRyxTQUFYLFFBQVEsQ0FBSSxJQUEyQixFQUFHO09BQTdCLElBQUksR0FBTCxJQUEyQixDQUExQixJQUFJO09BQUUsS0FBSyxHQUFaLElBQTJCLENBQXBCLEtBQUs7eUJBQVosSUFBMkIsQ0FBYixVQUFVO09BQVYsVUFBVSxtQ0FBQyxDQUFDOztBQUMxQyxPQUFJLEtBQUssR0FBRyxNQUFNLENBQUMsVUFBVSxHQUFHLE1BQU0sQ0FBQyxNQUFNLENBQUMsQ0FBQztBQUMvQyxPQUFJLEtBQUssR0FBRztBQUNWLHNCQUFpQixFQUFFLEtBQUs7QUFDeEIsa0JBQWEsRUFBRSxLQUFLO0lBQ3JCLENBQUM7O0FBRUYsVUFDRTs7O0tBQ0U7O1NBQU0sS0FBSyxFQUFFLEtBQU0sRUFBQyxTQUFTLEVBQUMsMkNBQTJDO09BQ3ZFOzs7U0FBUyxJQUFJLENBQUMsQ0FBQyxDQUFDO1FBQVU7TUFDckI7SUFDSixDQUNOO0VBQ0YsQ0FBQzs7QUFFRixLQUFNLGdCQUFnQixHQUFHLFNBQW5CLGdCQUFnQixDQUFJLEtBQVMsRUFBSztPQUFiLE9BQU8sR0FBUixLQUFTLENBQVIsT0FBTzs7QUFDaEMsVUFBTyxHQUFHLE9BQU8sSUFBSSxFQUFFLENBQUM7QUFDeEIsT0FBSSxTQUFTLEdBQUcsT0FBTyxDQUFDLEdBQUcsQ0FBQyxVQUFDLElBQUksRUFBRSxLQUFLO1lBQ3RDLG9CQUFDLFFBQVEsSUFBQyxHQUFHLEVBQUUsS0FBTSxFQUFDLFVBQVUsRUFBRSxLQUFNLEVBQUMsSUFBSSxFQUFFLElBQUksQ0FBQyxJQUFLLEdBQUU7SUFDNUQsQ0FBQyxDQUFDOztBQUVILFVBQ0U7O09BQUssU0FBUyxFQUFDLDBCQUEwQjtLQUN2Qzs7U0FBSSxTQUFTLEVBQUMsS0FBSztPQUNoQixTQUFTO09BQ1Y7OztTQUNFOzthQUFRLE9BQU8sRUFBRSxPQUFPLENBQUMsS0FBTSxFQUFDLFNBQVMsRUFBQywyQkFBMkIsRUFBQyxJQUFJLEVBQUMsUUFBUTtXQUNqRiwyQkFBRyxTQUFTLEVBQUMsYUFBYSxHQUFLO1VBQ3hCO1FBQ047TUFDRjtJQUNELENBQ1A7RUFDRixDQUFDOztBQUVGLE9BQU0sQ0FBQyxPQUFPLEdBQUcsZ0JBQWdCLEM7Ozs7Ozs7Ozs7Ozs7O0FDeENqQyxLQUFJLEtBQUssR0FBRyxtQkFBTyxDQUFDLENBQU8sQ0FBQyxDQUFDO0FBQzdCLEtBQUksQ0FBQyxHQUFHLG1CQUFPLENBQUMsRUFBUSxDQUFDLENBQUM7QUFDMUIsS0FBSSxNQUFNLEdBQUcsbUJBQU8sQ0FBQyxDQUFRLENBQUMsQ0FBQzs7Z0JBQ2QsbUJBQU8sQ0FBQyxFQUFHLENBQUM7O0tBQXhCLFFBQVEsWUFBUixRQUFROztBQUViLEtBQUksZUFBZSxHQUFHLEtBQUssQ0FBQyxXQUFXLENBQUM7OztBQUV0QyxXQUFRLHNCQUFFO0FBQ1IsU0FBSSxTQUFTLEdBQUcsQ0FBQyxDQUFDLElBQUksQ0FBQyxJQUFJLENBQUMsU0FBUyxDQUFDLENBQUMsVUFBVSxDQUFDLFNBQVMsQ0FBQyxDQUFDO0FBQzdELFNBQUksT0FBTyxHQUFHLENBQUMsQ0FBQyxJQUFJLENBQUMsSUFBSSxDQUFDLFNBQVMsQ0FBQyxDQUFDLFVBQVUsQ0FBQyxTQUFTLENBQUMsQ0FBQztBQUMzRCxZQUFPLENBQUMsU0FBUyxFQUFFLE9BQU8sQ0FBQyxDQUFDO0lBQzdCOztBQUVELFdBQVEsb0JBQUMsSUFBb0IsRUFBQztTQUFwQixTQUFTLEdBQVYsSUFBb0IsQ0FBbkIsU0FBUztTQUFFLE9BQU8sR0FBbkIsSUFBb0IsQ0FBUixPQUFPOztBQUMxQixNQUFDLENBQUMsSUFBSSxDQUFDLElBQUksQ0FBQyxTQUFTLENBQUMsQ0FBQyxVQUFVLENBQUMsU0FBUyxFQUFFLFNBQVMsQ0FBQyxDQUFDO0FBQ3hELE1BQUMsQ0FBQyxJQUFJLENBQUMsSUFBSSxDQUFDLFNBQVMsQ0FBQyxDQUFDLFVBQVUsQ0FBQyxTQUFTLEVBQUUsT0FBTyxDQUFDLENBQUM7SUFDdkQ7O0FBRUQsa0JBQWUsNkJBQUc7QUFDZixZQUFPO0FBQ0wsZ0JBQVMsRUFBRSxNQUFNLEVBQUUsQ0FBQyxPQUFPLENBQUMsT0FBTyxDQUFDLENBQUMsTUFBTSxFQUFFO0FBQzdDLGNBQU8sRUFBRSxNQUFNLEVBQUUsQ0FBQyxLQUFLLENBQUMsT0FBTyxDQUFDLENBQUMsTUFBTSxFQUFFO0FBQ3pDLGVBQVEsRUFBRSxvQkFBSSxFQUFFO01BQ2pCLENBQUM7SUFDSDs7QUFFRix1QkFBb0Isa0NBQUU7QUFDcEIsTUFBQyxDQUFDLElBQUksQ0FBQyxJQUFJLENBQUMsRUFBRSxDQUFDLENBQUMsVUFBVSxDQUFDLFNBQVMsQ0FBQyxDQUFDO0lBQ3ZDOztBQUVELDRCQUF5QixxQ0FBQyxRQUFRLEVBQUM7cUJBQ04sSUFBSSxDQUFDLFFBQVEsRUFBRTs7U0FBckMsU0FBUztTQUFFLE9BQU87O0FBQ3ZCLFNBQUcsRUFBRSxNQUFNLENBQUMsU0FBUyxFQUFFLFFBQVEsQ0FBQyxTQUFTLENBQUMsSUFDcEMsTUFBTSxDQUFDLE9BQU8sRUFBRSxRQUFRLENBQUMsT0FBTyxDQUFDLENBQUMsRUFBQztBQUNyQyxXQUFJLENBQUMsUUFBUSxDQUFDLFFBQVEsQ0FBQyxDQUFDO01BQ3pCO0lBQ0o7O0FBRUQsd0JBQXFCLG1DQUFFO0FBQ3JCLFlBQU8sS0FBSyxDQUFDO0lBQ2Q7O0FBRUQsb0JBQWlCLCtCQUFFO0FBQ2pCLFNBQUksQ0FBQyxRQUFRLEdBQUcsUUFBUSxDQUFDLElBQUksQ0FBQyxRQUFRLEVBQUUsQ0FBQyxDQUFDLENBQUM7QUFDM0MsTUFBQyxDQUFDLElBQUksQ0FBQyxJQUFJLENBQUMsV0FBVyxDQUFDLENBQUMsVUFBVSxDQUFDO0FBQ2xDLGVBQVEsRUFBRSxRQUFRO0FBQ2xCLHlCQUFrQixFQUFFLEtBQUs7QUFDekIsaUJBQVUsRUFBRSxLQUFLO0FBQ2pCLG9CQUFhLEVBQUUsSUFBSTtBQUNuQixnQkFBUyxFQUFFLElBQUk7TUFDaEIsQ0FBQyxDQUFDLEVBQUUsQ0FBQyxZQUFZLEVBQUUsSUFBSSxDQUFDLFFBQVEsQ0FBQyxDQUFDOztBQUVuQyxTQUFJLENBQUMsUUFBUSxDQUFDLElBQUksQ0FBQyxLQUFLLENBQUMsQ0FBQztJQUMzQjs7QUFFRCxXQUFRLHNCQUFFO3NCQUNtQixJQUFJLENBQUMsUUFBUSxFQUFFOztTQUFyQyxTQUFTO1NBQUUsT0FBTzs7QUFDdkIsU0FBRyxFQUFFLE1BQU0sQ0FBQyxTQUFTLEVBQUUsSUFBSSxDQUFDLEtBQUssQ0FBQyxTQUFTLENBQUMsSUFDdEMsTUFBTSxDQUFDLE9BQU8sRUFBRSxJQUFJLENBQUMsS0FBSyxDQUFDLE9BQU8sQ0FBQyxDQUFDLEVBQUM7QUFDdkMsV0FBSSxDQUFDLEtBQUssQ0FBQyxRQUFRLENBQUMsRUFBQyxTQUFTLEVBQVQsU0FBUyxFQUFFLE9BQU8sRUFBUCxPQUFPLEVBQUMsQ0FBQyxDQUFDO01BQzdDO0lBQ0Y7O0FBRUQsU0FBTSxvQkFBRztBQUNQLFlBQ0U7O1NBQUssU0FBUyxFQUFDLDRDQUE0QyxFQUFDLEdBQUcsRUFBQyxhQUFhO09BQzNFLCtCQUFPLEdBQUcsRUFBQyxXQUFXLEVBQUMsSUFBSSxFQUFDLE1BQU0sRUFBQyxTQUFTLEVBQUMsdUJBQXVCLEVBQUMsSUFBSSxFQUFDLE9BQU8sR0FBRztPQUNwRjs7V0FBTSxTQUFTLEVBQUMsbUJBQW1COztRQUFVO09BQzdDLCtCQUFPLEdBQUcsRUFBQyxXQUFXLEVBQUMsSUFBSSxFQUFDLE1BQU0sRUFBQyxTQUFTLEVBQUMsdUJBQXVCLEVBQUMsSUFBSSxFQUFDLEtBQUssR0FBRztNQUM5RSxDQUNOO0lBQ0g7RUFDRixDQUFDLENBQUM7O0FBRUgsVUFBUyxNQUFNLENBQUMsS0FBSyxFQUFFLEtBQUssRUFBQztBQUMzQixVQUFPLE1BQU0sQ0FBQyxLQUFLLENBQUMsQ0FBQyxNQUFNLENBQUMsS0FBSyxFQUFFLEtBQUssQ0FBQyxDQUFDO0VBQzNDOzs7OztBQUtELEtBQUksV0FBVyxHQUFHLEtBQUssQ0FBQyxXQUFXLENBQUM7OztBQUVsQyxTQUFNLG9CQUFHO1NBQ0YsS0FBSyxHQUFJLElBQUksQ0FBQyxLQUFLLENBQW5CLEtBQUs7O0FBQ1YsU0FBSSxZQUFZLEdBQUcsTUFBTSxDQUFDLEtBQUssQ0FBQyxDQUFDLE1BQU0sQ0FBQyxZQUFZLENBQUMsQ0FBQzs7QUFFdEQsWUFDRTs7U0FBSyxTQUFTLEVBQUUsbUJBQW1CLEdBQUcsSUFBSSxDQUFDLEtBQUssQ0FBQyxTQUFVO09BQ3pEOztXQUFRLE9BQU8sRUFBRSxJQUFJLENBQUMsSUFBSSxDQUFDLElBQUksQ0FBQyxJQUFJLEVBQUUsQ0FBQyxDQUFDLENBQUUsRUFBQyxTQUFTLEVBQUMsMEJBQTBCO1NBQUMsMkJBQUcsU0FBUyxFQUFDLG9CQUFvQixHQUFLO1FBQVM7T0FDL0g7O1dBQU0sU0FBUyxFQUFDLFlBQVk7U0FBRSxZQUFZO1FBQVE7T0FDbEQ7O1dBQVEsT0FBTyxFQUFFLElBQUksQ0FBQyxJQUFJLENBQUMsSUFBSSxDQUFDLElBQUksRUFBRSxDQUFDLENBQUUsRUFBQyxTQUFTLEVBQUMsMEJBQTBCO1NBQUMsMkJBQUcsU0FBUyxFQUFDLHFCQUFxQixHQUFLO1FBQVM7TUFDM0gsQ0FDTjtJQUNIOztBQUVELE9BQUksZ0JBQUMsRUFBRSxFQUFDO1NBQ0QsS0FBSyxHQUFJLElBQUksQ0FBQyxLQUFLLENBQW5CLEtBQUs7O0FBQ1YsU0FBSSxRQUFRLEdBQUcsTUFBTSxDQUFDLEtBQUssQ0FBQyxDQUFDLEdBQUcsQ0FBQyxFQUFFLEVBQUUsT0FBTyxDQUFDLENBQUMsTUFBTSxFQUFFLENBQUM7QUFDdkQsU0FBSSxDQUFDLEtBQUssQ0FBQyxhQUFhLENBQUMsUUFBUSxDQUFDLENBQUM7SUFDcEM7RUFDRixDQUFDLENBQUM7O0FBRUgsWUFBVyxDQUFDLGFBQWEsR0FBRyxVQUFTLEtBQUssRUFBQztBQUN6QyxPQUFJLFNBQVMsR0FBRyxNQUFNLENBQUMsS0FBSyxDQUFDLENBQUMsT0FBTyxDQUFDLE9BQU8sQ0FBQyxDQUFDLE1BQU0sRUFBRSxDQUFDO0FBQ3hELE9BQUksT0FBTyxHQUFHLE1BQU0sQ0FBQyxLQUFLLENBQUMsQ0FBQyxLQUFLLENBQUMsT0FBTyxDQUFDLENBQUMsTUFBTSxFQUFFLENBQUM7QUFDcEQsVUFBTyxDQUFDLFNBQVMsRUFBRSxPQUFPLENBQUMsQ0FBQztFQUM3Qjs7c0JBRWMsZUFBZTtTQUN0QixXQUFXLEdBQVgsV0FBVztTQUFFLGVBQWUsR0FBZixlQUFlLEM7Ozs7Ozs7Ozs7Ozs7O0FDOUdwQyxLQUFJLEtBQUssR0FBRyxtQkFBTyxDQUFDLENBQU8sQ0FBQyxDQUFDOztBQUU3QixLQUFJLFFBQVEsR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFDL0IsU0FBTSxvQkFBRztBQUNQLFlBQ0U7O1NBQUssU0FBUyxFQUFDLGdCQUFnQjtPQUM3Qjs7V0FBSyxTQUFTLEVBQUMsZUFBZTs7UUFBZTtPQUM3Qzs7V0FBSyxTQUFTLEVBQUMsYUFBYTtTQUFDLDJCQUFHLFNBQVMsRUFBQyxlQUFlLEdBQUs7O1FBQU87T0FDckU7Ozs7UUFBb0M7T0FDcEM7Ozs7UUFBd0U7T0FDeEU7Ozs7UUFBMkY7T0FDM0Y7O1dBQUssU0FBUyxFQUFDLGlCQUFpQjs7U0FBdUQ7O2FBQUcsSUFBSSxFQUFDLHNEQUFzRDs7VUFBMkI7UUFDeks7TUFDSCxDQUNOO0lBQ0g7RUFDRixDQUFDOztBQUVGLEtBQUksYUFBYSxHQUFHLEtBQUssQ0FBQyxXQUFXLENBQUM7OztBQUNwQyxTQUFNLG9CQUFHO0FBQ1AsWUFDRTs7U0FBSyxTQUFTLEVBQUMsZ0JBQWdCO09BQzdCOztXQUFLLFNBQVMsRUFBQyxlQUFlOztRQUFlO09BQzdDOztXQUFLLFNBQVMsRUFBQyxhQUFhO1NBQUMsMkJBQUcsU0FBUyxFQUFDLGVBQWUsR0FBSzs7UUFBTztPQUNyRTs7OztRQUFnQztPQUNoQzs7OztRQUEwRDtPQUMxRDs7V0FBSyxTQUFTLEVBQUMsaUJBQWlCOztTQUF1RDs7YUFBRyxJQUFJLEVBQUMsc0RBQXNEOztVQUEyQjtRQUN6SztNQUNILENBQ047SUFDSDtFQUNGLENBQUM7O3NCQUVhLFFBQVE7U0FDZixRQUFRLEdBQVIsUUFBUTtTQUFFLGFBQWEsR0FBYixhQUFhLEM7Ozs7Ozs7Ozs7Ozs7QUNsQy9CLEtBQUksS0FBSyxHQUFHLG1CQUFPLENBQUMsQ0FBTyxDQUFDLENBQUM7O0FBRTdCLEtBQUksY0FBYyxHQUFHLEtBQUssQ0FBQyxXQUFXLENBQUM7OztBQUNyQyxTQUFNLG9CQUFHO0FBQ1AsWUFDRTs7U0FBSyxTQUFTLEVBQUMsaUJBQWlCO09BQzlCLDZCQUFLLFNBQVMsRUFBQyxzQkFBc0IsR0FBTztPQUM1Qzs7OztRQUFxQztPQUNyQzs7OztTQUFjOzthQUFHLElBQUksRUFBQywwREFBMEQ7O1VBQXlCOztRQUFxRDtNQUMxSixDQUNOO0lBQ0g7RUFDRixDQUFDOztBQUVGLE9BQU0sQ0FBQyxPQUFPLEdBQUcsY0FBYyxDOzs7Ozs7Ozs7Ozs7Ozs7OztBQ2QvQixLQUFJLEtBQUssR0FBRyxtQkFBTyxDQUFDLENBQU8sQ0FBQyxDQUFDO0FBQzdCLEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsQ0FBYSxDQUFDLENBQUM7O2dCQUNaLG1CQUFPLENBQUMsR0FBbUIsQ0FBQzs7S0FBaEQsT0FBTyxZQUFQLE9BQU87S0FBRSxPQUFPLFlBQVAsT0FBTzs7QUFDckIsS0FBSSxXQUFXLEdBQUcsbUJBQU8sQ0FBQyxFQUEwQixDQUFDLENBQUM7O2lCQUNDLG1CQUFPLENBQUMsRUFBMEIsQ0FBQzs7S0FBckYsS0FBSyxhQUFMLEtBQUs7S0FBRSxNQUFNLGFBQU4sTUFBTTtLQUFFLElBQUksYUFBSixJQUFJO0tBQUUsY0FBYyxhQUFkLGNBQWM7S0FBRSxTQUFTLGFBQVQsU0FBUzs7aUJBQzFCLG1CQUFPLENBQUMsRUFBb0MsQ0FBQzs7S0FBakUsZ0JBQWdCLGFBQWhCLGdCQUFnQjs7QUFDckIsS0FBSSxnQkFBZ0IsR0FBRyxtQkFBTyxDQUFDLEVBQWlDLENBQUMsQ0FBQztBQUNsRSxLQUFJLENBQUMsR0FBRyxtQkFBTyxDQUFDLEVBQUcsQ0FBQyxDQUFDOztpQkFDTCxtQkFBTyxDQUFDLEVBQXdCLENBQUM7O0tBQTVDLE9BQU8sYUFBUCxPQUFPOztBQUVaLEtBQU0sUUFBUSxHQUFHLFNBQVgsUUFBUSxDQUFJLElBQXFDO09BQXBDLFFBQVEsR0FBVCxJQUFxQyxDQUFwQyxRQUFRO09BQUUsSUFBSSxHQUFmLElBQXFDLENBQTFCLElBQUk7T0FBRSxTQUFTLEdBQTFCLElBQXFDLENBQXBCLFNBQVM7O09BQUssS0FBSyw0QkFBcEMsSUFBcUM7O1VBQ3JEO0FBQUMsU0FBSTtLQUFLLEtBQUs7S0FDWixJQUFJLENBQUMsUUFBUSxDQUFDLENBQUMsU0FBUyxDQUFDO0lBQ3JCO0VBQ1IsQ0FBQzs7QUFFRixLQUFNLE9BQU8sR0FBRyxTQUFWLE9BQU8sQ0FBSSxLQUFxQztPQUFwQyxRQUFRLEdBQVQsS0FBcUMsQ0FBcEMsUUFBUTtPQUFFLElBQUksR0FBZixLQUFxQyxDQUExQixJQUFJO09BQUUsU0FBUyxHQUExQixLQUFxQyxDQUFwQixTQUFTOztPQUFLLEtBQUssNEJBQXBDLEtBQXFDOztVQUNwRDtBQUFDLFNBQUk7S0FBSyxLQUFLO0tBQ1gsSUFBSSxDQUFDLFFBQVEsQ0FBQyxDQUFDLElBQUksQ0FBQyxHQUFHLENBQUMsVUFBQyxJQUFJLEVBQUUsS0FBSztjQUNuQzs7V0FBTSxHQUFHLEVBQUUsS0FBTSxFQUFDLFNBQVMsRUFBQyxxQkFBcUI7U0FDL0MsSUFBSSxDQUFDLElBQUk7O1NBQUUsNEJBQUksU0FBUyxFQUFDLHdCQUF3QixHQUFNO1NBQ3ZELElBQUksQ0FBQyxLQUFLO1FBQ047TUFBQyxDQUNUO0lBQ0k7RUFDUixDQUFDOztBQUVGLEtBQU0sU0FBUyxHQUFHLFNBQVosU0FBUyxDQUFJLEtBQWdELEVBQUs7T0FBcEQsTUFBTSxHQUFQLEtBQWdELENBQS9DLE1BQU07T0FBRSxZQUFZLEdBQXJCLEtBQWdELENBQXZDLFlBQVk7T0FBRSxRQUFRLEdBQS9CLEtBQWdELENBQXpCLFFBQVE7T0FBRSxJQUFJLEdBQXJDLEtBQWdELENBQWYsSUFBSTs7T0FBSyxLQUFLLDRCQUEvQyxLQUFnRDs7QUFDakUsT0FBRyxDQUFDLE1BQU0sSUFBRyxNQUFNLENBQUMsTUFBTSxLQUFLLENBQUMsRUFBQztBQUMvQixZQUFPLG9CQUFDLElBQUksRUFBSyxLQUFLLENBQUksQ0FBQztJQUM1Qjs7QUFFRCxPQUFJLFFBQVEsR0FBRyxJQUFJLENBQUMsUUFBUSxDQUFDLENBQUMsRUFBRSxDQUFDO0FBQ2pDLE9BQUksSUFBSSxHQUFHLEVBQUUsQ0FBQzs7QUFFZCxZQUFTLE9BQU8sQ0FBQyxDQUFDLEVBQUM7QUFDakIsU0FBSSxLQUFLLEdBQUcsTUFBTSxDQUFDLENBQUMsQ0FBQyxDQUFDO0FBQ3RCLFNBQUcsWUFBWSxFQUFDO0FBQ2QsY0FBTztnQkFBSyxZQUFZLENBQUMsUUFBUSxFQUFFLEtBQUssQ0FBQztRQUFBLENBQUM7TUFDM0MsTUFBSTtBQUNILGNBQU87Z0JBQU0sZ0JBQWdCLENBQUMsUUFBUSxFQUFFLEtBQUssQ0FBQztRQUFBLENBQUM7TUFDaEQ7SUFDRjs7QUFFRCxRQUFJLElBQUksQ0FBQyxHQUFHLENBQUMsRUFBRSxDQUFDLEdBQUcsTUFBTSxDQUFDLE1BQU0sRUFBRSxDQUFDLEVBQUUsRUFBQztBQUNwQyxTQUFJLENBQUMsSUFBSSxDQUFDOztTQUFJLEdBQUcsRUFBRSxDQUFFO09BQUM7O1dBQUcsT0FBTyxFQUFFLE9BQU8sQ0FBQyxDQUFDLENBQUU7U0FBRSxNQUFNLENBQUMsQ0FBQyxDQUFDO1FBQUs7TUFBSyxDQUFDLENBQUM7SUFDckU7O0FBRUQsVUFDRTtBQUFDLFNBQUk7S0FBSyxLQUFLO0tBQ2I7O1NBQUssU0FBUyxFQUFDLFdBQVc7T0FDeEI7O1dBQVEsSUFBSSxFQUFDLFFBQVEsRUFBQyxPQUFPLEVBQUUsT0FBTyxDQUFDLENBQUMsQ0FBRSxFQUFDLFNBQVMsRUFBQyx3QkFBd0I7U0FBRSxNQUFNLENBQUMsQ0FBQyxDQUFDO1FBQVU7T0FFaEcsSUFBSSxDQUFDLE1BQU0sR0FBRyxDQUFDLEdBQ1gsQ0FDRTs7V0FBUSxHQUFHLEVBQUUsQ0FBRSxFQUFDLGVBQVksVUFBVSxFQUFDLFNBQVMsRUFBQyx3Q0FBd0MsRUFBQyxpQkFBYyxNQUFNO1NBQzVHLDhCQUFNLFNBQVMsRUFBQyxPQUFPLEdBQVE7UUFDeEIsRUFDVDs7V0FBSSxHQUFHLEVBQUUsQ0FBRSxFQUFDLFNBQVMsRUFBQyxlQUFlO1NBQ2xDLElBQUk7UUFDRixDQUNOLEdBQ0QsSUFBSTtNQUVOO0lBQ0QsQ0FDUjtFQUNGLENBQUM7O0FBRUYsS0FBSSxRQUFRLEdBQUcsS0FBSyxDQUFDLFdBQVcsQ0FBQzs7O0FBRS9CLFNBQU0sRUFBRSxDQUFDLGdCQUFnQixDQUFDOztBQUUxQixrQkFBZSwyQkFBQyxLQUFLLEVBQUM7QUFDcEIsU0FBSSxDQUFDLGVBQWUsR0FBRyxDQUFDLE1BQU0sRUFBRSxVQUFVLENBQUMsQ0FBQztBQUM1QyxZQUFPLEVBQUUsTUFBTSxFQUFFLEVBQUUsRUFBRSxXQUFXLEVBQUUsRUFBQyxRQUFRLEVBQUUsTUFBTSxFQUFDLEVBQUUsQ0FBQztJQUN4RDs7QUFFRCxlQUFZLHdCQUFDLFNBQVMsRUFBRSxPQUFPLEVBQUU7OztBQUMvQixTQUFJLENBQUMsUUFBUSxjQUNSLElBQUksQ0FBQyxLQUFLO0FBQ2Isa0JBQVcsbUNBQ1IsU0FBUyxJQUFHLE9BQU8sZUFDckI7UUFDRCxDQUFDO0lBQ0o7O0FBRUQsZ0JBQWEseUJBQUMsSUFBSSxFQUFDOzs7QUFDakIsU0FBSSxRQUFRLEdBQUcsSUFBSSxDQUFDLE1BQU0sQ0FBQyxhQUFHO2NBQzVCLE9BQU8sQ0FBQyxHQUFHLEVBQUUsTUFBSyxLQUFLLENBQUMsTUFBTSxFQUFFLEVBQUUsZUFBZSxFQUFFLE1BQUssZUFBZSxFQUFDLENBQUM7TUFBQSxDQUFDLENBQUM7O0FBRTdFLFNBQUksU0FBUyxHQUFHLE1BQU0sQ0FBQyxtQkFBbUIsQ0FBQyxJQUFJLENBQUMsS0FBSyxDQUFDLFdBQVcsQ0FBQyxDQUFDLENBQUMsQ0FBQyxDQUFDO0FBQ3RFLFNBQUksT0FBTyxHQUFHLElBQUksQ0FBQyxLQUFLLENBQUMsV0FBVyxDQUFDLFNBQVMsQ0FBQyxDQUFDO0FBQ2hELFNBQUksTUFBTSxHQUFHLENBQUMsQ0FBQyxNQUFNLENBQUMsUUFBUSxFQUFFLFNBQVMsQ0FBQyxDQUFDO0FBQzNDLFNBQUcsT0FBTyxLQUFLLFNBQVMsQ0FBQyxHQUFHLEVBQUM7QUFDM0IsYUFBTSxHQUFHLE1BQU0sQ0FBQyxPQUFPLEVBQUUsQ0FBQztNQUMzQjs7QUFFRCxZQUFPLE1BQU0sQ0FBQztJQUNmOztBQUVELFNBQU0sRUFBRSxrQkFBVztBQUNqQixTQUFJLElBQUksR0FBRyxJQUFJLENBQUMsYUFBYSxDQUFDLElBQUksQ0FBQyxLQUFLLENBQUMsV0FBVyxDQUFDLENBQUM7QUFDdEQsU0FBSSxNQUFNLEdBQUcsSUFBSSxDQUFDLEtBQUssQ0FBQyxNQUFNLENBQUM7QUFDL0IsU0FBSSxZQUFZLEdBQUcsSUFBSSxDQUFDLEtBQUssQ0FBQyxZQUFZLENBQUM7O0FBRTNDLFlBQ0U7O1NBQUssU0FBUyxFQUFDLG9CQUFvQjtPQUNqQzs7V0FBSyxTQUFTLEVBQUMscUJBQXFCO1NBQ2xDLDZCQUFLLFNBQVMsRUFBQyxpQkFBaUIsR0FBTztTQUN2Qzs7YUFBSyxTQUFTLEVBQUMsaUJBQWlCO1dBQzlCOzs7O1lBQWdCO1VBQ1o7U0FDTjs7YUFBSyxTQUFTLEVBQUMsaUJBQWlCO1dBQzlCOztlQUFLLFNBQVMsRUFBQyxZQUFZO2FBQ3pCLCtCQUFPLFNBQVMsRUFBRSxJQUFJLENBQUMsU0FBUyxDQUFDLFFBQVEsQ0FBRSxFQUFDLFdBQVcsRUFBQyxXQUFXLEVBQUMsU0FBUyxFQUFDLHVCQUF1QixHQUFFO1lBQ25HO1VBQ0Y7UUFDRjtPQUNOOztXQUFLLFNBQVMsRUFBQyxFQUFFO1NBQ2Y7QUFBQyxnQkFBSzthQUFDLFFBQVEsRUFBRSxJQUFJLENBQUMsTUFBTyxFQUFDLFNBQVMsRUFBQywrQkFBK0I7V0FDckUsb0JBQUMsTUFBTTtBQUNMLHNCQUFTLEVBQUMsVUFBVTtBQUNwQixtQkFBTSxFQUNKLG9CQUFDLGNBQWM7QUFDYixzQkFBTyxFQUFFLElBQUksQ0FBQyxLQUFLLENBQUMsV0FBVyxDQUFDLFFBQVM7QUFDekMsMkJBQVksRUFBRSxJQUFJLENBQUMsWUFBYTtBQUNoQyxvQkFBSyxFQUFDLE1BQU07ZUFFZjtBQUNELGlCQUFJLEVBQUUsb0JBQUMsUUFBUSxJQUFDLElBQUksRUFBRSxJQUFLLEdBQUk7YUFDL0I7V0FDRixvQkFBQyxNQUFNO0FBQ0wsc0JBQVMsRUFBQyxNQUFNO0FBQ2hCLG1CQUFNLEVBQ0osb0JBQUMsY0FBYztBQUNiLHNCQUFPLEVBQUUsSUFBSSxDQUFDLEtBQUssQ0FBQyxXQUFXLENBQUMsSUFBSztBQUNyQywyQkFBWSxFQUFFLElBQUksQ0FBQyxZQUFhO0FBQ2hDLG9CQUFLLEVBQUMsSUFBSTtlQUViOztBQUVELGlCQUFJLEVBQUUsb0JBQUMsUUFBUSxJQUFDLElBQUksRUFBRSxJQUFLLEdBQUk7YUFDL0I7V0FDRixvQkFBQyxNQUFNO0FBQ0wsc0JBQVMsRUFBQyxNQUFNO0FBQ2hCLG1CQUFNLEVBQUUsb0JBQUMsSUFBSSxPQUFVO0FBQ3ZCLGlCQUFJLEVBQUUsb0JBQUMsT0FBTyxJQUFDLElBQUksRUFBRSxJQUFLLEdBQUk7YUFDOUI7V0FDRixvQkFBQyxNQUFNO0FBQ0wsc0JBQVMsRUFBQyxPQUFPO0FBQ2pCLHlCQUFZLEVBQUUsWUFBYTtBQUMzQixtQkFBTSxFQUFFO0FBQUMsbUJBQUk7OztjQUFrQjtBQUMvQixpQkFBSSxFQUFFLG9CQUFDLFNBQVMsSUFBQyxJQUFJLEVBQUUsSUFBSyxFQUFDLE1BQU0sRUFBRSxNQUFPLEdBQUk7YUFDaEQ7VUFDSTtRQUNKO01BQ0YsQ0FDUDtJQUNGO0VBQ0YsQ0FBQyxDQUFDOztBQUVILE9BQU0sQ0FBQyxPQUFPLEdBQUcsUUFBUSxDOzs7Ozs7Ozs7Ozs7O0FDbEt6QixLQUFJLEtBQUssR0FBRyxtQkFBTyxDQUFDLENBQU8sQ0FBQyxDQUFDO0FBQzdCLEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsQ0FBYSxDQUFDLENBQUM7O2dCQUNyQixtQkFBTyxDQUFDLEdBQXFCLENBQUM7O0tBQXpDLE9BQU8sWUFBUCxPQUFPOztpQkFDa0IsbUJBQU8sQ0FBQyxFQUE2QixDQUFDOztLQUEvRCxxQkFBcUIsYUFBckIscUJBQXFCOztpQkFDTCxtQkFBTyxDQUFDLEVBQW9DLENBQUM7O0tBQTdELFlBQVksYUFBWixZQUFZOztBQUNqQixLQUFJLFFBQVEsR0FBRyxtQkFBTyxDQUFDLEdBQXNCLENBQUMsQ0FBQztBQUMvQyxLQUFJLG9CQUFvQixHQUFHLG1CQUFPLENBQUMsRUFBb0MsQ0FBQyxDQUFDO0FBQ3pFLEtBQUksV0FBVyxHQUFHLG1CQUFPLENBQUMsRUFBMkIsQ0FBQyxDQUFDOztBQUV2RCxLQUFJLGdCQUFnQixHQUFHLEtBQUssQ0FBQyxXQUFXLENBQUM7OztBQUV2QyxTQUFNLEVBQUUsQ0FBQyxPQUFPLENBQUMsVUFBVSxDQUFDOztBQUU1QixrQkFBZSw2QkFBRztBQUNoQixZQUFPO0FBQ0wsY0FBTyxFQUFFLE9BQU8sQ0FBQyxPQUFPO01BQ3pCO0lBQ0Y7O0FBRUQsU0FBTSxvQkFBRztBQUNQLFlBQU8sSUFBSSxDQUFDLEtBQUssQ0FBQyxPQUFPLENBQUMsc0JBQXNCLEdBQUcsb0JBQUMsTUFBTSxPQUFFLEdBQUcsSUFBSSxDQUFDO0lBQ3JFO0VBQ0YsQ0FBQyxDQUFDOztBQUVILEtBQUksTUFBTSxHQUFHLEtBQUssQ0FBQyxXQUFXLENBQUM7OztBQUU3QixlQUFZLHdCQUFDLFFBQVEsRUFBRSxLQUFLLEVBQUM7QUFDM0IsU0FBRyxnQkFBZ0IsQ0FBQyxzQkFBc0IsRUFBQztBQUN6Qyx1QkFBZ0IsQ0FBQyxzQkFBc0IsQ0FBQyxFQUFDLFFBQVEsRUFBUixRQUFRLEVBQUMsQ0FBQyxDQUFDO01BQ3JEOztBQUVELDBCQUFxQixFQUFFLENBQUM7SUFDekI7O0FBRUQsdUJBQW9CLGdDQUFDLFFBQVEsRUFBQztBQUM1QixNQUFDLENBQUMsUUFBUSxDQUFDLENBQUMsS0FBSyxDQUFDLE1BQU0sQ0FBQyxDQUFDO0lBQzNCOztBQUVELG9CQUFpQiwrQkFBRTtBQUNqQixNQUFDLENBQUMsUUFBUSxDQUFDLENBQUMsS0FBSyxDQUFDLE1BQU0sQ0FBQyxDQUFDO0lBQzNCOztBQUVELFNBQU0sb0JBQUc7QUFDUCxTQUFJLGFBQWEsR0FBRyxPQUFPLENBQUMsUUFBUSxDQUFDLG9CQUFvQixDQUFDLGFBQWEsQ0FBQyxJQUFJLEVBQUUsQ0FBQztBQUMvRSxTQUFJLFdBQVcsR0FBRyxPQUFPLENBQUMsUUFBUSxDQUFDLFdBQVcsQ0FBQyxZQUFZLENBQUMsQ0FBQztBQUM3RCxTQUFJLE1BQU0sR0FBRyxDQUFDLGFBQWEsQ0FBQyxLQUFLLENBQUMsQ0FBQzs7QUFFbkMsWUFDRTs7U0FBSyxTQUFTLEVBQUMsbUNBQW1DLEVBQUMsUUFBUSxFQUFFLENBQUMsQ0FBRSxFQUFDLElBQUksRUFBQyxRQUFRO09BQzVFOztXQUFLLFNBQVMsRUFBQyxjQUFjO1NBQzNCOzthQUFLLFNBQVMsRUFBQyxlQUFlO1dBQzVCLDZCQUFLLFNBQVMsRUFBQyxjQUFjLEdBQ3ZCO1dBQ047O2VBQUssU0FBUyxFQUFDLFlBQVk7YUFDekIsb0JBQUMsUUFBUSxJQUFDLFdBQVcsRUFBRSxXQUFZLEVBQUMsTUFBTSxFQUFFLE1BQU8sRUFBQyxZQUFZLEVBQUUsSUFBSSxDQUFDLFlBQWEsR0FBRTtZQUNsRjtXQUNOOztlQUFLLFNBQVMsRUFBQyxjQUFjO2FBQzNCOztpQkFBUSxPQUFPLEVBQUUscUJBQXNCLEVBQUMsSUFBSSxFQUFDLFFBQVEsRUFBQyxTQUFTLEVBQUMsaUJBQWlCOztjQUV4RTtZQUNMO1VBQ0Y7UUFDRjtNQUNGLENBQ047SUFDSDtFQUNGLENBQUMsQ0FBQzs7QUFFSCxpQkFBZ0IsQ0FBQyxzQkFBc0IsR0FBRyxZQUFJLEVBQUUsQ0FBQzs7QUFFakQsT0FBTSxDQUFDLE9BQU8sR0FBRyxnQkFBZ0IsQzs7Ozs7Ozs7Ozs7Ozs7Ozs7QUN0RWpDLEtBQUksS0FBSyxHQUFHLG1CQUFPLENBQUMsQ0FBTyxDQUFDLENBQUM7O2dCQUNkLG1CQUFPLENBQUMsRUFBYyxDQUFDOztLQUFoQyxJQUFJLFlBQUosSUFBSTs7aUJBQ00sbUJBQU8sQ0FBQyxFQUFzQixDQUFDOztLQUExQyxPQUFPLGFBQVAsT0FBTzs7aUJBQ21CLG1CQUFPLENBQUMsRUFBMkIsQ0FBQzs7S0FBOUQsc0JBQXNCLGFBQXRCLHNCQUFzQjs7aUJBQ0osbUJBQU8sQ0FBQyxFQUEwQixDQUFDOztLQUFyRCxJQUFJLGFBQUosSUFBSTtLQUFFLFFBQVEsYUFBUixRQUFROztBQUNuQixLQUFJLE1BQU0sR0FBSSxtQkFBTyxDQUFDLENBQVEsQ0FBQyxDQUFDOztBQUVoQyxLQUFNLGVBQWUsR0FBRyxTQUFsQixlQUFlLENBQUksSUFBNEIsRUFBSztPQUEvQixRQUFRLEdBQVYsSUFBNEIsQ0FBMUIsUUFBUTtPQUFFLElBQUksR0FBaEIsSUFBNEIsQ0FBaEIsSUFBSTs7T0FBSyxLQUFLLDRCQUExQixJQUE0Qjs7QUFDbkQsT0FBSSxPQUFPLEdBQUcsSUFBSSxDQUFDLFFBQVEsQ0FBQyxDQUFDLE9BQU8sQ0FBQztBQUNyQyxPQUFJLFdBQVcsR0FBRyxNQUFNLENBQUMsT0FBTyxDQUFDLENBQUMsTUFBTSxDQUFDLE9BQU8sQ0FBQyxDQUFDO0FBQ2xELFVBQ0U7QUFBQyxTQUFJO0tBQUssS0FBSztLQUNYLFdBQVc7SUFDUixDQUNSO0VBQ0YsQ0FBQzs7QUFFRixLQUFNLFlBQVksR0FBRyxTQUFmLFlBQVksQ0FBSSxLQUE0QixFQUFLO09BQS9CLFFBQVEsR0FBVixLQUE0QixDQUExQixRQUFRO09BQUUsSUFBSSxHQUFoQixLQUE0QixDQUFoQixJQUFJOztPQUFLLEtBQUssNEJBQTFCLEtBQTRCOztBQUNoRCxPQUFJLE9BQU8sR0FBRyxJQUFJLENBQUMsUUFBUSxDQUFDLENBQUMsT0FBTyxDQUFDO0FBQ3JDLE9BQUksVUFBVSxHQUFHLElBQUksQ0FBQyxRQUFRLENBQUMsQ0FBQyxVQUFVLENBQUM7O0FBRTNDLE9BQUksR0FBRyxHQUFHLE1BQU0sQ0FBQyxPQUFPLENBQUMsQ0FBQztBQUMxQixPQUFJLEdBQUcsR0FBRyxNQUFNLENBQUMsVUFBVSxDQUFDLENBQUM7QUFDN0IsT0FBSSxRQUFRLEdBQUcsTUFBTSxDQUFDLFFBQVEsQ0FBQyxHQUFHLENBQUMsSUFBSSxDQUFDLEdBQUcsQ0FBQyxDQUFDLENBQUM7QUFDOUMsT0FBSSxXQUFXLEdBQUcsUUFBUSxDQUFDLFFBQVEsRUFBRSxDQUFDOztBQUV0QyxVQUNFO0FBQUMsU0FBSTtLQUFLLEtBQUs7S0FDWCxXQUFXO0lBQ1IsQ0FDUjtFQUNGLENBQUM7O0FBRUYsS0FBTSxjQUFjLEdBQUcsU0FBakIsY0FBYyxDQUFJLEtBQTRCLEVBQUs7T0FBL0IsUUFBUSxHQUFWLEtBQTRCLENBQTFCLFFBQVE7T0FBRSxJQUFJLEdBQWhCLEtBQTRCLENBQWhCLElBQUk7O09BQUssS0FBSyw0QkFBMUIsS0FBNEI7O0FBQ2xELFVBQ0U7QUFBQyxTQUFJO0tBQUssS0FBSztLQUNiOztTQUFNLFNBQVMsRUFBQyx1Q0FBdUM7T0FBRSxJQUFJLENBQUMsUUFBUSxDQUFDLENBQUMsS0FBSztNQUFRO0lBQ2hGLENBQ1I7RUFDRixDQUFDOztBQUVGLEtBQU0sU0FBUyxHQUFHLFNBQVosU0FBUyxDQUFJLEtBQTRCLEVBQUs7T0FBL0IsUUFBUSxHQUFWLEtBQTRCLENBQTFCLFFBQVE7T0FBRSxJQUFJLEdBQWhCLEtBQTRCLENBQWhCLElBQUk7O09BQUssS0FBSyw0QkFBMUIsS0FBNEI7O0FBQzdDLE9BQUksTUFBTSxHQUFHLElBQUksQ0FBQyxRQUFRLENBQUMsQ0FBQyxPQUFPLENBQUMsR0FBRyxDQUFDLFVBQUMsSUFBSSxFQUFFLFNBQVM7WUFDckQ7O1NBQU0sR0FBRyxFQUFFLFNBQVUsRUFBQyxTQUFTLEVBQUMsdUNBQXVDO09BQUUsSUFBSSxDQUFDLElBQUk7TUFBUTtJQUFDLENBQzdGOztBQUVELFVBQ0U7QUFBQyxTQUFJO0tBQUssS0FBSztLQUNiOzs7T0FDRyxNQUFNO01BQ0g7SUFDRCxDQUNSO0VBQ0YsQ0FBQzs7QUFFRixLQUFNLFVBQVUsR0FBRyxTQUFiLFVBQVUsQ0FBSSxLQUE0QixFQUFLO09BQS9CLFFBQVEsR0FBVixLQUE0QixDQUExQixRQUFRO09BQUUsSUFBSSxHQUFoQixLQUE0QixDQUFoQixJQUFJOztPQUFLLEtBQUssNEJBQTFCLEtBQTRCOzt3QkFDakIsSUFBSSxDQUFDLFFBQVEsQ0FBQztPQUFyQyxVQUFVLGtCQUFWLFVBQVU7T0FBRSxNQUFNLGtCQUFOLE1BQU07O2VBQ1EsTUFBTSxHQUFHLENBQUMsTUFBTSxFQUFFLGFBQWEsQ0FBQyxHQUFHLENBQUMsTUFBTSxFQUFFLGFBQWEsQ0FBQzs7T0FBckYsVUFBVTtPQUFFLFdBQVc7O0FBQzVCLFVBQ0U7QUFBQyxTQUFJO0tBQUssS0FBSztLQUNiO0FBQUMsV0FBSTtTQUFDLEVBQUUsRUFBRSxVQUFXLEVBQUMsU0FBUyxFQUFFLE1BQU0sR0FBRSxXQUFXLEdBQUUsU0FBVSxFQUFDLElBQUksRUFBQyxRQUFRO09BQUUsVUFBVTtNQUFRO0lBQzdGLENBQ1I7RUFDRjs7QUFFRCxLQUFNLFNBQVMsR0FBRyxTQUFaLFNBQVMsQ0FBSSxLQUFNO09BQUwsSUFBSSxHQUFMLEtBQU0sQ0FBTCxJQUFJO1VBQ3RCOztPQUFLLFNBQVMsRUFBQywyQ0FBMkM7S0FBQzs7O09BQU8sSUFBSTtNQUFRO0lBQU07RUFDckY7O0FBRUQsS0FBTSxRQUFRLEdBQUcsU0FBWCxRQUFRLENBQUksS0FBNEIsRUFBSztPQUEvQixRQUFRLEdBQVYsS0FBNEIsQ0FBMUIsUUFBUTtPQUFFLElBQUksR0FBaEIsS0FBNEIsQ0FBaEIsSUFBSTs7T0FBSyxLQUFLLDRCQUExQixLQUE0Qjs7T0FDdkMsUUFBUSxHQUFJLElBQUksQ0FBQyxRQUFRLENBQUMsQ0FBMUIsUUFBUTs7QUFDYixPQUFJLFFBQVEsR0FBRyxPQUFPLENBQUMsUUFBUSxDQUFDLHNCQUFzQixDQUFDLFFBQVEsQ0FBQyxDQUFDLElBQUksU0FBUyxDQUFDOztBQUUvRSxVQUNFO0FBQUMsU0FBSTtLQUFLLEtBQUs7S0FDWixRQUFRO0lBQ0osQ0FDUjtFQUNGOztzQkFFYyxVQUFVO1NBR3ZCLFVBQVUsR0FBVixVQUFVO1NBQ1YsU0FBUyxHQUFULFNBQVM7U0FDVCxZQUFZLEdBQVosWUFBWTtTQUNaLGVBQWUsR0FBZixlQUFlO1NBQ2YsU0FBUyxHQUFULFNBQVM7U0FDVCxjQUFjLEdBQWQsY0FBYztTQUNkLFFBQVEsR0FBUixRQUFRLEM7Ozs7Ozs7Ozs7Ozs7QUN6RlYsS0FBSSxJQUFJLEdBQUcsbUJBQU8sQ0FBQyxHQUFVLENBQUMsQ0FBQztBQUMvQixLQUFJLEtBQUssR0FBRyxtQkFBTyxDQUFDLENBQU8sQ0FBQyxDQUFDOztnQkFDRixtQkFBTyxDQUFDLEVBQUcsQ0FBQzs7S0FBbEMsUUFBUSxZQUFSLFFBQVE7S0FBRSxRQUFRLFlBQVIsUUFBUTs7QUFFdkIsS0FBSSxDQUFDLE1BQU0sQ0FBQyxHQUFHLENBQUMsR0FBRyxTQUFTLENBQUM7O0FBRTdCLEtBQU0sY0FBYyxHQUFHLGdDQUFnQyxDQUFDO0FBQ3hELEtBQU0sYUFBYSxHQUFHLGdCQUFnQixDQUFDOztBQUV2QyxLQUFJLFdBQVcsR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFFbEMsa0JBQWUsNkJBQUU7OztBQUNmLFNBQUksQ0FBQyxJQUFJLEdBQUcsSUFBSSxDQUFDLEtBQUssQ0FBQyxJQUFJLENBQUM7QUFDNUIsU0FBSSxDQUFDLElBQUksR0FBRyxJQUFJLENBQUMsS0FBSyxDQUFDLElBQUksQ0FBQztBQUM1QixTQUFJLENBQUMsR0FBRyxHQUFHLElBQUksQ0FBQyxLQUFLLENBQUMsR0FBRyxDQUFDOztBQUUxQixTQUFJLENBQUMsZUFBZSxHQUFHLFFBQVEsQ0FBQyxZQUFJO0FBQ2xDLGFBQUssTUFBTSxFQUFFLENBQUM7QUFDZCxhQUFLLEdBQUcsQ0FBQyxNQUFNLENBQUMsTUFBSyxJQUFJLEVBQUUsTUFBSyxJQUFJLENBQUMsQ0FBQztNQUN2QyxFQUFFLEdBQUcsQ0FBQyxDQUFDOztBQUVSLFlBQU8sRUFBRSxDQUFDO0lBQ1g7O0FBRUQsb0JBQWlCLEVBQUUsNkJBQVc7OztBQUM1QixTQUFJLENBQUMsSUFBSSxHQUFHLElBQUksUUFBUSxDQUFDO0FBQ3ZCLFdBQUksRUFBRSxDQUFDO0FBQ1AsV0FBSSxFQUFFLENBQUM7QUFDUCxlQUFRLEVBQUUsSUFBSTtBQUNkLGlCQUFVLEVBQUUsSUFBSTtBQUNoQixrQkFBVyxFQUFFLElBQUk7TUFDbEIsQ0FBQyxDQUFDOztBQUVILFNBQUksQ0FBQyxJQUFJLENBQUMsSUFBSSxDQUFDLElBQUksQ0FBQyxJQUFJLENBQUMsU0FBUyxDQUFDLENBQUM7QUFDcEMsU0FBSSxDQUFDLElBQUksQ0FBQyxFQUFFLENBQUMsTUFBTSxFQUFFLFVBQUMsSUFBSTtjQUFLLE9BQUssR0FBRyxDQUFDLElBQUksQ0FBQyxJQUFJLENBQUM7TUFBQSxDQUFDLENBQUM7O0FBRXBELFNBQUksQ0FBQyxNQUFNLENBQUMsSUFBSSxDQUFDLElBQUksRUFBRSxJQUFJLENBQUMsSUFBSSxDQUFDLENBQUM7O0FBRWxDLFNBQUksQ0FBQyxHQUFHLENBQUMsRUFBRSxDQUFDLE1BQU0sRUFBRTtjQUFLLE9BQUssSUFBSSxDQUFDLEtBQUssQ0FBQyxhQUFhLENBQUM7TUFBQSxDQUFDLENBQUM7QUFDekQsU0FBSSxDQUFDLEdBQUcsQ0FBQyxFQUFFLENBQUMsTUFBTSxFQUFFLFVBQUMsSUFBSTtjQUFLLE9BQUssSUFBSSxDQUFDLEtBQUssQ0FBQyxJQUFJLENBQUM7TUFBQSxDQUFDLENBQUM7QUFDckQsU0FBSSxDQUFDLEdBQUcsQ0FBQyxFQUFFLENBQUMsT0FBTyxFQUFFO2NBQUssT0FBSyxJQUFJLENBQUMsS0FBSyxFQUFFO01BQUEsQ0FBQyxDQUFDOztBQUU3QyxTQUFJLENBQUMsR0FBRyxDQUFDLE9BQU8sQ0FBQyxFQUFDLElBQUksRUFBRSxJQUFJLENBQUMsSUFBSSxFQUFFLElBQUksRUFBRSxJQUFJLENBQUMsSUFBSSxFQUFDLENBQUMsQ0FBQztBQUNyRCxXQUFNLENBQUMsZ0JBQWdCLENBQUMsUUFBUSxFQUFFLElBQUksQ0FBQyxlQUFlLENBQUMsQ0FBQztJQUN6RDs7QUFFRCx1QkFBb0IsRUFBRSxnQ0FBVztBQUMvQixTQUFJLENBQUMsSUFBSSxDQUFDLE9BQU8sRUFBRSxDQUFDO0FBQ3BCLFdBQU0sQ0FBQyxtQkFBbUIsQ0FBQyxRQUFRLEVBQUUsSUFBSSxDQUFDLGVBQWUsQ0FBQyxDQUFDO0lBQzVEOztBQUVELHdCQUFxQixFQUFFLCtCQUFTLFFBQVEsRUFBRTtTQUNuQyxJQUFJLEdBQVUsUUFBUSxDQUF0QixJQUFJO1NBQUUsSUFBSSxHQUFJLFFBQVEsQ0FBaEIsSUFBSTs7QUFFZixTQUFJLENBQUMsUUFBUSxDQUFDLElBQUksQ0FBQyxJQUFJLENBQUMsUUFBUSxDQUFDLElBQUksQ0FBQyxFQUFDO0FBQ3JDLGNBQU8sS0FBSyxDQUFDO01BQ2Q7O0FBRUQsU0FBRyxJQUFJLEtBQUssSUFBSSxDQUFDLElBQUksSUFBSSxJQUFJLEtBQUssSUFBSSxDQUFDLElBQUksRUFBQztBQUMxQyxXQUFJLENBQUMsTUFBTSxDQUFDLElBQUksRUFBRSxJQUFJLENBQUM7TUFDeEI7O0FBRUQsWUFBTyxLQUFLLENBQUM7SUFDZDs7QUFFRCxTQUFNLG9CQUFHO0FBQ1AsWUFBUzs7U0FBSyxTQUFTLEVBQUMsY0FBYyxFQUFDLEVBQUUsRUFBQyxjQUFjLEVBQUMsR0FBRyxFQUFDLFdBQVc7O01BQVMsQ0FBRztJQUNyRjs7QUFFRCxTQUFNLEVBQUUsZ0JBQVMsSUFBSSxFQUFFLElBQUksRUFBRTs7QUFFM0IsU0FBRyxDQUFDLFFBQVEsQ0FBQyxJQUFJLENBQUMsSUFBSSxDQUFDLFFBQVEsQ0FBQyxJQUFJLENBQUMsRUFBQztBQUNwQyxXQUFJLEdBQUcsR0FBRyxJQUFJLENBQUMsY0FBYyxFQUFFLENBQUM7QUFDaEMsV0FBSSxHQUFHLEdBQUcsQ0FBQyxJQUFJLENBQUM7QUFDaEIsV0FBSSxHQUFHLEdBQUcsQ0FBQyxJQUFJLENBQUM7TUFDakI7O0FBRUQsU0FBSSxDQUFDLElBQUksR0FBRyxJQUFJLENBQUM7QUFDakIsU0FBSSxDQUFDLElBQUksR0FBRyxJQUFJLENBQUM7O0FBRWpCLFNBQUksQ0FBQyxJQUFJLENBQUMsTUFBTSxDQUFDLElBQUksQ0FBQyxJQUFJLEVBQUUsSUFBSSxDQUFDLElBQUksQ0FBQyxDQUFDO0lBQ3hDOztBQUVELGlCQUFjLDRCQUFFO0FBQ2QsU0FBSSxVQUFVLEdBQUcsQ0FBQyxDQUFDLElBQUksQ0FBQyxJQUFJLENBQUMsU0FBUyxDQUFDLENBQUM7QUFDeEMsU0FBSSxPQUFPLEdBQUcsQ0FBQyxDQUFDLGdDQUFnQyxDQUFDLENBQUM7O0FBRWxELGVBQVUsQ0FBQyxJQUFJLENBQUMsV0FBVyxDQUFDLENBQUMsTUFBTSxDQUFDLE9BQU8sQ0FBQyxDQUFDOztBQUU3QyxTQUFJLGFBQWEsR0FBRyxPQUFPLENBQUMsQ0FBQyxDQUFDLENBQUMscUJBQXFCLEVBQUUsQ0FBQyxNQUFNLENBQUM7O0FBRTlELFNBQUksWUFBWSxHQUFHLE9BQU8sQ0FBQyxRQUFRLEVBQUUsQ0FBQyxLQUFLLEVBQUUsQ0FBQyxDQUFDLENBQUMsQ0FBQyxxQkFBcUIsRUFBRSxDQUFDLEtBQUssQ0FBQzs7QUFFL0UsU0FBSSxLQUFLLEdBQUcsVUFBVSxDQUFDLENBQUMsQ0FBQyxDQUFDLFdBQVcsQ0FBQztBQUN0QyxTQUFJLE1BQU0sR0FBRyxVQUFVLENBQUMsQ0FBQyxDQUFDLENBQUMsWUFBWSxDQUFDOztBQUV4QyxTQUFJLElBQUksR0FBRyxJQUFJLENBQUMsS0FBSyxDQUFDLEtBQUssR0FBSSxZQUFhLENBQUMsQ0FBQztBQUM5QyxTQUFJLElBQUksR0FBRyxJQUFJLENBQUMsS0FBSyxDQUFDLE1BQU0sR0FBSSxhQUFjLENBQUMsQ0FBQztBQUNoRCxZQUFPLENBQUMsTUFBTSxFQUFFLENBQUM7O0FBRWpCLFlBQU8sRUFBQyxJQUFJLEVBQUosSUFBSSxFQUFFLElBQUksRUFBSixJQUFJLEVBQUMsQ0FBQztJQUNyQjs7RUFFRixDQUFDLENBQUM7O0FBRUgsWUFBVyxDQUFDLFNBQVMsR0FBRztBQUN0QixNQUFHLEVBQUUsS0FBSyxDQUFDLFNBQVMsQ0FBQyxNQUFNLENBQUMsVUFBVTtFQUN2Qzs7QUFFRCxPQUFNLENBQUMsT0FBTyxHQUFHLFdBQVcsQzs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7OztzQ0NyR04sRUFBVzs7OztBQUVqQyxVQUFTLFlBQVksQ0FBQyxNQUFNLEVBQUU7QUFDNUIsVUFBTyxNQUFNLENBQUMsT0FBTyxDQUFDLHFCQUFxQixFQUFFLE1BQU0sQ0FBQztFQUNyRDs7QUFFRCxVQUFTLFlBQVksQ0FBQyxNQUFNLEVBQUU7QUFDNUIsVUFBTyxZQUFZLENBQUMsTUFBTSxDQUFDLENBQUMsT0FBTyxDQUFDLE1BQU0sRUFBRSxJQUFJLENBQUM7RUFDbEQ7O0FBRUQsVUFBUyxlQUFlLENBQUMsT0FBTyxFQUFFO0FBQ2hDLE9BQUksWUFBWSxHQUFHLEVBQUUsQ0FBQztBQUN0QixPQUFNLFVBQVUsR0FBRyxFQUFFLENBQUM7QUFDdEIsT0FBTSxNQUFNLEdBQUcsRUFBRSxDQUFDOztBQUVsQixPQUFJLEtBQUs7T0FBRSxTQUFTLEdBQUcsQ0FBQztPQUFFLE9BQU8sR0FBRyw0Q0FBNEM7O0FBRWhGLFVBQVEsS0FBSyxHQUFHLE9BQU8sQ0FBQyxJQUFJLENBQUMsT0FBTyxDQUFDLEVBQUc7QUFDdEMsU0FBSSxLQUFLLENBQUMsS0FBSyxLQUFLLFNBQVMsRUFBRTtBQUM3QixhQUFNLENBQUMsSUFBSSxDQUFDLE9BQU8sQ0FBQyxLQUFLLENBQUMsU0FBUyxFQUFFLEtBQUssQ0FBQyxLQUFLLENBQUMsQ0FBQztBQUNsRCxtQkFBWSxJQUFJLFlBQVksQ0FBQyxPQUFPLENBQUMsS0FBSyxDQUFDLFNBQVMsRUFBRSxLQUFLLENBQUMsS0FBSyxDQUFDLENBQUM7TUFDcEU7O0FBRUQsU0FBSSxLQUFLLENBQUMsQ0FBQyxDQUFDLEVBQUU7QUFDWixtQkFBWSxJQUFJLFdBQVcsQ0FBQztBQUM1QixpQkFBVSxDQUFDLElBQUksQ0FBQyxLQUFLLENBQUMsQ0FBQyxDQUFDLENBQUMsQ0FBQztNQUMzQixNQUFNLElBQUksS0FBSyxDQUFDLENBQUMsQ0FBQyxLQUFLLElBQUksRUFBRTtBQUM1QixtQkFBWSxJQUFJLGFBQWE7QUFDN0IsaUJBQVUsQ0FBQyxJQUFJLENBQUMsT0FBTyxDQUFDLENBQUM7TUFDMUIsTUFBTSxJQUFJLEtBQUssQ0FBQyxDQUFDLENBQUMsS0FBSyxHQUFHLEVBQUU7QUFDM0IsbUJBQVksSUFBSSxjQUFjO0FBQzlCLGlCQUFVLENBQUMsSUFBSSxDQUFDLE9BQU8sQ0FBQyxDQUFDO01BQzFCLE1BQU0sSUFBSSxLQUFLLENBQUMsQ0FBQyxDQUFDLEtBQUssR0FBRyxFQUFFO0FBQzNCLG1CQUFZLElBQUksS0FBSyxDQUFDO01BQ3ZCLE1BQU0sSUFBSSxLQUFLLENBQUMsQ0FBQyxDQUFDLEtBQUssR0FBRyxFQUFFO0FBQzNCLG1CQUFZLElBQUksSUFBSSxDQUFDO01BQ3RCOztBQUVELFdBQU0sQ0FBQyxJQUFJLENBQUMsS0FBSyxDQUFDLENBQUMsQ0FBQyxDQUFDLENBQUM7O0FBRXRCLGNBQVMsR0FBRyxPQUFPLENBQUMsU0FBUyxDQUFDO0lBQy9COztBQUVELE9BQUksU0FBUyxLQUFLLE9BQU8sQ0FBQyxNQUFNLEVBQUU7QUFDaEMsV0FBTSxDQUFDLElBQUksQ0FBQyxPQUFPLENBQUMsS0FBSyxDQUFDLFNBQVMsRUFBRSxPQUFPLENBQUMsTUFBTSxDQUFDLENBQUM7QUFDckQsaUJBQVksSUFBSSxZQUFZLENBQUMsT0FBTyxDQUFDLEtBQUssQ0FBQyxTQUFTLEVBQUUsT0FBTyxDQUFDLE1BQU0sQ0FBQyxDQUFDO0lBQ3ZFOztBQUVELFVBQU87QUFDTCxZQUFPLEVBQVAsT0FBTztBQUNQLGlCQUFZLEVBQVosWUFBWTtBQUNaLGVBQVUsRUFBVixVQUFVO0FBQ1YsV0FBTSxFQUFOLE1BQU07SUFDUDtFQUNGOztBQUVELEtBQU0scUJBQXFCLEdBQUcsRUFBRTs7QUFFekIsVUFBUyxjQUFjLENBQUMsT0FBTyxFQUFFO0FBQ3RDLE9BQUksRUFBRSxPQUFPLElBQUkscUJBQXFCLENBQUMsRUFDckMscUJBQXFCLENBQUMsT0FBTyxDQUFDLEdBQUcsZUFBZSxDQUFDLE9BQU8sQ0FBQzs7QUFFM0QsVUFBTyxxQkFBcUIsQ0FBQyxPQUFPLENBQUM7RUFDdEM7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7QUFxQk0sVUFBUyxZQUFZLENBQUMsT0FBTyxFQUFFLFFBQVEsRUFBRTs7QUFFOUMsT0FBSSxPQUFPLENBQUMsTUFBTSxDQUFDLENBQUMsQ0FBQyxLQUFLLEdBQUcsRUFBRTtBQUM3QixZQUFPLFNBQU8sT0FBUztJQUN4QjtBQUNELE9BQUksUUFBUSxDQUFDLE1BQU0sQ0FBQyxDQUFDLENBQUMsS0FBSyxHQUFHLEVBQUU7QUFDOUIsYUFBUSxTQUFPLFFBQVU7SUFDMUI7OzBCQUUwQyxjQUFjLENBQUMsT0FBTyxDQUFDOztPQUE1RCxZQUFZLG9CQUFaLFlBQVk7T0FBRSxVQUFVLG9CQUFWLFVBQVU7T0FBRSxNQUFNLG9CQUFOLE1BQU07O0FBRXRDLGVBQVksSUFBSSxJQUFJOzs7QUFHcEIsT0FBTSxnQkFBZ0IsR0FBRyxNQUFNLENBQUMsTUFBTSxDQUFDLE1BQU0sR0FBRyxDQUFDLENBQUMsS0FBSyxHQUFHOztBQUUxRCxPQUFJLGdCQUFnQixFQUFFOztBQUVwQixpQkFBWSxJQUFJLGNBQWM7SUFDL0I7O0FBRUQsT0FBTSxLQUFLLEdBQUcsUUFBUSxDQUFDLEtBQUssQ0FBQyxJQUFJLE1BQU0sQ0FBQyxHQUFHLEdBQUcsWUFBWSxHQUFHLEdBQUcsRUFBRSxHQUFHLENBQUMsQ0FBQzs7QUFFdkUsT0FBSSxpQkFBaUI7T0FBRSxXQUFXO0FBQ2xDLE9BQUksS0FBSyxJQUFJLElBQUksRUFBRTtBQUNqQixTQUFJLGdCQUFnQixFQUFFO0FBQ3BCLHdCQUFpQixHQUFHLEtBQUssQ0FBQyxHQUFHLEVBQUU7QUFDL0IsV0FBTSxXQUFXLEdBQ2YsS0FBSyxDQUFDLENBQUMsQ0FBQyxDQUFDLE1BQU0sQ0FBQyxDQUFDLEVBQUUsS0FBSyxDQUFDLENBQUMsQ0FBQyxDQUFDLE1BQU0sR0FBRyxpQkFBaUIsQ0FBQyxNQUFNLENBQUM7Ozs7O0FBS2hFLFdBQ0UsaUJBQWlCLElBQ2pCLFdBQVcsQ0FBQyxNQUFNLENBQUMsV0FBVyxDQUFDLE1BQU0sR0FBRyxDQUFDLENBQUMsS0FBSyxHQUFHLEVBQ2xEO0FBQ0EsZ0JBQU87QUFDTCw0QkFBaUIsRUFBRSxJQUFJO0FBQ3ZCLHFCQUFVLEVBQVYsVUFBVTtBQUNWLHNCQUFXLEVBQUUsSUFBSTtVQUNsQjtRQUNGO01BQ0YsTUFBTTs7QUFFTCx3QkFBaUIsR0FBRyxFQUFFO01BQ3ZCOztBQUVELGdCQUFXLEdBQUcsS0FBSyxDQUFDLEtBQUssQ0FBQyxDQUFDLENBQUMsQ0FBQyxHQUFHLENBQzlCLFdBQUM7Y0FBSSxDQUFDLElBQUksSUFBSSxHQUFHLGtCQUFrQixDQUFDLENBQUMsQ0FBQyxHQUFHLENBQUM7TUFBQSxDQUMzQztJQUNGLE1BQU07QUFDTCxzQkFBaUIsR0FBRyxXQUFXLEdBQUcsSUFBSTtJQUN2Qzs7QUFFRCxVQUFPO0FBQ0wsc0JBQWlCLEVBQWpCLGlCQUFpQjtBQUNqQixlQUFVLEVBQVYsVUFBVTtBQUNWLGdCQUFXLEVBQVgsV0FBVztJQUNaO0VBQ0Y7O0FBRU0sVUFBUyxhQUFhLENBQUMsT0FBTyxFQUFFO0FBQ3JDLFVBQU8sY0FBYyxDQUFDLE9BQU8sQ0FBQyxDQUFDLFVBQVU7RUFDMUM7O0FBRU0sVUFBUyxTQUFTLENBQUMsT0FBTyxFQUFFLFFBQVEsRUFBRTt1QkFDUCxZQUFZLENBQUMsT0FBTyxFQUFFLFFBQVEsQ0FBQzs7T0FBM0QsVUFBVSxpQkFBVixVQUFVO09BQUUsV0FBVyxpQkFBWCxXQUFXOztBQUUvQixPQUFJLFdBQVcsSUFBSSxJQUFJLEVBQUU7QUFDdkIsWUFBTyxVQUFVLENBQUMsTUFBTSxDQUFDLFVBQVUsSUFBSSxFQUFFLFNBQVMsRUFBRSxLQUFLLEVBQUU7QUFDekQsV0FBSSxDQUFDLFNBQVMsQ0FBQyxHQUFHLFdBQVcsQ0FBQyxLQUFLLENBQUM7QUFDcEMsY0FBTyxJQUFJO01BQ1osRUFBRSxFQUFFLENBQUM7SUFDUDs7QUFFRCxVQUFPLElBQUk7RUFDWjs7Ozs7OztBQU1NLFVBQVMsYUFBYSxDQUFDLE9BQU8sRUFBRSxNQUFNLEVBQUU7QUFDN0MsU0FBTSxHQUFHLE1BQU0sSUFBSSxFQUFFOzswQkFFRixjQUFjLENBQUMsT0FBTyxDQUFDOztPQUFsQyxNQUFNLG9CQUFOLE1BQU07O0FBQ2QsT0FBSSxVQUFVLEdBQUcsQ0FBQztPQUFFLFFBQVEsR0FBRyxFQUFFO09BQUUsVUFBVSxHQUFHLENBQUM7O0FBRWpELE9BQUksS0FBSztPQUFFLFNBQVM7T0FBRSxVQUFVO0FBQ2hDLFFBQUssSUFBSSxDQUFDLEdBQUcsQ0FBQyxFQUFFLEdBQUcsR0FBRyxNQUFNLENBQUMsTUFBTSxFQUFFLENBQUMsR0FBRyxHQUFHLEVBQUUsRUFBRSxDQUFDLEVBQUU7QUFDakQsVUFBSyxHQUFHLE1BQU0sQ0FBQyxDQUFDLENBQUM7O0FBRWpCLFNBQUksS0FBSyxLQUFLLEdBQUcsSUFBSSxLQUFLLEtBQUssSUFBSSxFQUFFO0FBQ25DLGlCQUFVLEdBQUcsS0FBSyxDQUFDLE9BQU8sQ0FBQyxNQUFNLENBQUMsS0FBSyxDQUFDLEdBQUcsTUFBTSxDQUFDLEtBQUssQ0FBQyxVQUFVLEVBQUUsQ0FBQyxHQUFHLE1BQU0sQ0FBQyxLQUFLOztBQUVwRiw4QkFDRSxVQUFVLElBQUksSUFBSSxJQUFJLFVBQVUsR0FBRyxDQUFDLEVBQ3BDLGlDQUFpQyxFQUNqQyxVQUFVLEVBQUUsT0FBTyxDQUNwQjs7QUFFRCxXQUFJLFVBQVUsSUFBSSxJQUFJLEVBQ3BCLFFBQVEsSUFBSSxTQUFTLENBQUMsVUFBVSxDQUFDO01BQ3BDLE1BQU0sSUFBSSxLQUFLLEtBQUssR0FBRyxFQUFFO0FBQ3hCLGlCQUFVLElBQUksQ0FBQztNQUNoQixNQUFNLElBQUksS0FBSyxLQUFLLEdBQUcsRUFBRTtBQUN4QixpQkFBVSxJQUFJLENBQUM7TUFDaEIsTUFBTSxJQUFJLEtBQUssQ0FBQyxNQUFNLENBQUMsQ0FBQyxDQUFDLEtBQUssR0FBRyxFQUFFO0FBQ2xDLGdCQUFTLEdBQUcsS0FBSyxDQUFDLFNBQVMsQ0FBQyxDQUFDLENBQUM7QUFDOUIsaUJBQVUsR0FBRyxNQUFNLENBQUMsU0FBUyxDQUFDOztBQUU5Qiw4QkFDRSxVQUFVLElBQUksSUFBSSxJQUFJLFVBQVUsR0FBRyxDQUFDLEVBQ3BDLHNDQUFzQyxFQUN0QyxTQUFTLEVBQUUsT0FBTyxDQUNuQjs7QUFFRCxXQUFJLFVBQVUsSUFBSSxJQUFJLEVBQ3BCLFFBQVEsSUFBSSxrQkFBa0IsQ0FBQyxVQUFVLENBQUM7TUFDN0MsTUFBTTtBQUNMLGVBQVEsSUFBSSxLQUFLO01BQ2xCO0lBQ0Y7O0FBRUQsVUFBTyxRQUFRLENBQUMsT0FBTyxDQUFDLE1BQU0sRUFBRSxHQUFHLENBQUM7Ozs7Ozs7Ozs7Ozs7Ozs7QUN6TnRDLEtBQUksR0FBRyxHQUFHLG1CQUFPLENBQUMsRUFBZ0IsQ0FBQyxDQUFDO0FBQ3BDLEtBQUksR0FBRyxHQUFHLG1CQUFPLENBQUMsRUFBa0IsQ0FBQyxDQUFDO0FBQ3RDLEtBQUksR0FBRyxHQUFHLG1CQUFPLENBQUMsRUFBWSxDQUFDLENBQUM7O0tBRTFCLFNBQVM7YUFBVCxTQUFTOztBQUNGLFlBRFAsU0FBUyxDQUNELElBQUssRUFBQztTQUFMLEdBQUcsR0FBSixJQUFLLENBQUosR0FBRzs7MkJBRFosU0FBUzs7QUFFWCxxQkFBTSxFQUFFLENBQUMsQ0FBQztBQUNWLFNBQUksQ0FBQyxHQUFHLEdBQUcsR0FBRyxDQUFDO0FBQ2YsU0FBSSxDQUFDLE9BQU8sR0FBRyxDQUFDLENBQUM7QUFDakIsU0FBSSxDQUFDLE1BQU0sR0FBRyxDQUFDLENBQUMsQ0FBQztBQUNqQixTQUFJLENBQUMsUUFBUSxHQUFHLElBQUksS0FBSyxFQUFFLENBQUM7QUFDNUIsU0FBSSxDQUFDLFFBQVEsR0FBRyxLQUFLLENBQUM7QUFDdEIsU0FBSSxDQUFDLFNBQVMsR0FBRyxLQUFLLENBQUM7QUFDdkIsU0FBSSxDQUFDLE9BQU8sR0FBRyxLQUFLLENBQUM7QUFDckIsU0FBSSxDQUFDLE9BQU8sR0FBRyxLQUFLLENBQUM7QUFDckIsU0FBSSxDQUFDLFNBQVMsR0FBRyxJQUFJLENBQUM7SUFDdkI7O0FBWkcsWUFBUyxXQWNiLElBQUksbUJBQUUsRUFDTDs7QUFmRyxZQUFTLFdBaUJiLE1BQU0scUJBQUUsRUFDUDs7QUFsQkcsWUFBUyxXQW9CYixPQUFPLHNCQUFFOzs7QUFDUCxRQUFHLENBQUMsR0FBRyxDQUFDLEdBQUcsQ0FBQyxHQUFHLENBQUMsd0JBQXdCLENBQUMsSUFBSSxDQUFDLEdBQUcsQ0FBQyxDQUFDLENBQ2hELElBQUksQ0FBQyxVQUFDLElBQUksRUFBRztBQUNaLGFBQUssTUFBTSxHQUFHLElBQUksQ0FBQyxLQUFLLENBQUM7QUFDekIsYUFBSyxPQUFPLEdBQUcsSUFBSSxDQUFDO01BQ3JCLENBQUMsQ0FDRCxJQUFJLENBQUMsWUFBSTtBQUNSLGFBQUssT0FBTyxHQUFHLElBQUksQ0FBQztNQUNyQixDQUFDLENBQ0QsTUFBTSxDQUFDLFlBQUk7QUFDVixhQUFLLE9BQU8sRUFBRSxDQUFDO01BQ2hCLENBQUMsQ0FBQztJQUNOOztBQWhDRyxZQUFTLFdBa0NiLElBQUksaUJBQUMsTUFBTSxFQUFDO0FBQ1YsU0FBRyxDQUFDLElBQUksQ0FBQyxPQUFPLEVBQUM7QUFDZixjQUFPO01BQ1I7O0FBRUQsU0FBRyxNQUFNLEtBQUssU0FBUyxFQUFDO0FBQ3RCLGFBQU0sR0FBRyxJQUFJLENBQUMsT0FBTyxHQUFHLENBQUMsQ0FBQztNQUMzQjs7QUFFRCxTQUFHLE1BQU0sR0FBRyxJQUFJLENBQUMsTUFBTSxFQUFDO0FBQ3RCLGFBQU0sR0FBRyxJQUFJLENBQUMsTUFBTSxDQUFDO0FBQ3JCLFdBQUksQ0FBQyxJQUFJLEVBQUUsQ0FBQztNQUNiOztBQUVELFNBQUcsTUFBTSxLQUFLLENBQUMsRUFBQztBQUNkLGFBQU0sR0FBRyxDQUFDLENBQUM7TUFDWjs7QUFFRCxTQUFHLElBQUksQ0FBQyxTQUFTLEVBQUM7QUFDaEIsV0FBRyxJQUFJLENBQUMsT0FBTyxHQUFHLE1BQU0sRUFBQztBQUN2QixhQUFJLENBQUMsVUFBVSxDQUFDLElBQUksQ0FBQyxPQUFPLEVBQUUsTUFBTSxDQUFDLENBQUM7UUFDdkMsTUFBSTtBQUNILGFBQUksQ0FBQyxJQUFJLENBQUMsT0FBTyxDQUFDLENBQUM7QUFDbkIsYUFBSSxDQUFDLFVBQVUsQ0FBQyxJQUFJLENBQUMsT0FBTyxFQUFFLE1BQU0sQ0FBQyxDQUFDO1FBQ3ZDO01BQ0YsTUFBSTtBQUNILFdBQUksQ0FBQyxPQUFPLEdBQUcsTUFBTSxDQUFDO01BQ3ZCOztBQUVELFNBQUksQ0FBQyxPQUFPLEVBQUUsQ0FBQztJQUNoQjs7QUFoRUcsWUFBUyxXQWtFYixJQUFJLG1CQUFFO0FBQ0osU0FBSSxDQUFDLFNBQVMsR0FBRyxLQUFLLENBQUM7QUFDdkIsU0FBSSxDQUFDLEtBQUssR0FBRyxhQUFhLENBQUMsSUFBSSxDQUFDLEtBQUssQ0FBQyxDQUFDO0FBQ3ZDLFNBQUksQ0FBQyxPQUFPLEVBQUUsQ0FBQztJQUNoQjs7QUF0RUcsWUFBUyxXQXdFYixJQUFJLG1CQUFFO0FBQ0osU0FBRyxJQUFJLENBQUMsU0FBUyxFQUFDO0FBQ2hCLGNBQU87TUFDUjs7QUFFRCxTQUFJLENBQUMsU0FBUyxHQUFHLElBQUksQ0FBQzs7O0FBR3RCLFNBQUcsSUFBSSxDQUFDLE9BQU8sS0FBSyxJQUFJLENBQUMsTUFBTSxFQUFDO0FBQzlCLFdBQUksQ0FBQyxPQUFPLEdBQUcsQ0FBQyxDQUFDO0FBQ2pCLFdBQUksQ0FBQyxJQUFJLENBQUMsT0FBTyxDQUFDLENBQUM7TUFDcEI7O0FBRUQsU0FBSSxDQUFDLEtBQUssR0FBRyxXQUFXLENBQUMsSUFBSSxDQUFDLElBQUksQ0FBQyxJQUFJLENBQUMsSUFBSSxDQUFDLEVBQUUsR0FBRyxDQUFDLENBQUM7QUFDcEQsU0FBSSxDQUFDLE9BQU8sRUFBRSxDQUFDO0lBQ2hCOztBQXZGRyxZQUFTLFdBeUZiLFlBQVkseUJBQUMsS0FBSyxFQUFFLEdBQUcsRUFBQztBQUN0QixVQUFJLElBQUksQ0FBQyxHQUFHLEtBQUssRUFBRSxDQUFDLEdBQUcsR0FBRyxFQUFFLENBQUMsRUFBRSxFQUFDO0FBQzlCLFdBQUcsSUFBSSxDQUFDLFFBQVEsQ0FBQyxDQUFDLENBQUMsS0FBSyxTQUFTLEVBQUM7QUFDaEMsZ0JBQU8sSUFBSSxDQUFDO1FBQ2I7TUFDRjs7QUFFRCxZQUFPLEtBQUssQ0FBQztJQUNkOztBQWpHRyxZQUFTLFdBbUdiLE1BQU0sbUJBQUMsS0FBSyxFQUFFLEdBQUcsRUFBQzs7O0FBQ2hCLFFBQUcsR0FBRyxHQUFHLEdBQUcsRUFBRSxDQUFDO0FBQ2YsUUFBRyxHQUFHLEdBQUcsR0FBRyxJQUFJLENBQUMsTUFBTSxHQUFHLElBQUksQ0FBQyxNQUFNLEdBQUcsR0FBRyxDQUFDO0FBQzVDLFlBQU8sR0FBRyxDQUFDLEdBQUcsQ0FBQyxHQUFHLENBQUMsR0FBRyxDQUFDLHVCQUF1QixDQUFDLEVBQUMsR0FBRyxFQUFFLElBQUksQ0FBQyxHQUFHLEVBQUUsS0FBSyxFQUFMLEtBQUssRUFBRSxHQUFHLEVBQUgsR0FBRyxFQUFDLENBQUMsQ0FBQyxDQUMxRSxJQUFJLENBQUMsVUFBQyxRQUFRLEVBQUc7QUFDZixZQUFJLElBQUksQ0FBQyxHQUFHLENBQUMsRUFBRSxDQUFDLEdBQUcsR0FBRyxHQUFDLEtBQUssRUFBRSxDQUFDLEVBQUUsRUFBQztBQUNoQyxhQUFJLElBQUksR0FBRyxJQUFJLENBQUMsUUFBUSxDQUFDLE1BQU0sQ0FBQyxDQUFDLENBQUMsQ0FBQyxJQUFJLENBQUMsSUFBSSxFQUFFLENBQUM7QUFDL0MsYUFBSSxLQUFLLEdBQUcsUUFBUSxDQUFDLE1BQU0sQ0FBQyxDQUFDLENBQUMsQ0FBQyxLQUFLLENBQUM7QUFDckMsZ0JBQUssUUFBUSxDQUFDLEtBQUssR0FBQyxDQUFDLENBQUMsR0FBRyxFQUFFLElBQUksRUFBSixJQUFJLEVBQUUsS0FBSyxFQUFMLEtBQUssRUFBQyxDQUFDO1FBQ3pDO01BQ0YsQ0FBQyxDQUFDO0lBQ047O0FBOUdHLFlBQVMsV0FnSGIsVUFBVSx1QkFBQyxLQUFLLEVBQUUsR0FBRyxFQUFDOzs7QUFDcEIsU0FBSSxPQUFPLEdBQUcsU0FBVixPQUFPLEdBQU87QUFDaEIsWUFBSSxJQUFJLENBQUMsR0FBRyxLQUFLLEVBQUUsQ0FBQyxHQUFHLEdBQUcsRUFBRSxDQUFDLEVBQUUsRUFBQztBQUM5QixnQkFBSyxJQUFJLENBQUMsTUFBTSxFQUFFLE9BQUssUUFBUSxDQUFDLENBQUMsQ0FBQyxDQUFDLElBQUksQ0FBQyxDQUFDO1FBQzFDO0FBQ0QsY0FBSyxPQUFPLEdBQUcsR0FBRyxDQUFDO01BQ3BCLENBQUM7O0FBRUYsU0FBRyxJQUFJLENBQUMsWUFBWSxDQUFDLEtBQUssRUFBRSxHQUFHLENBQUMsRUFBQztBQUMvQixXQUFJLENBQUMsTUFBTSxDQUFDLEtBQUssRUFBRSxHQUFHLENBQUMsQ0FBQyxJQUFJLENBQUMsT0FBTyxDQUFDLENBQUM7TUFDdkMsTUFBSTtBQUNILGNBQU8sRUFBRSxDQUFDO01BQ1g7SUFDRjs7QUE3SEcsWUFBUyxXQStIYixPQUFPLHNCQUFFO0FBQ1AsU0FBSSxDQUFDLElBQUksQ0FBQyxRQUFRLENBQUMsQ0FBQztJQUNyQjs7VUFqSUcsU0FBUztJQUFTLEdBQUc7O3NCQW9JWixTQUFTOzs7Ozs7Ozs7OztBQ3hJeEIsS0FBSSxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxDQUFhLENBQUMsQ0FBQzs7Z0JBQ2YsbUJBQU8sQ0FBQyxFQUF1QixDQUFDOztLQUFqRCxhQUFhLFlBQWIsYUFBYTs7aUJBQ0MsbUJBQU8sQ0FBQyxHQUFvQixDQUFDOztLQUEzQyxVQUFVLGFBQVYsVUFBVTs7aUJBQ0ksbUJBQU8sQ0FBQyxFQUFzQixDQUFDOztLQUE3QyxVQUFVLGFBQVYsVUFBVTs7QUFFZixLQUFJLENBQUMsR0FBRyxtQkFBTyxDQUFDLEVBQVEsQ0FBQyxDQUFDOztpQkFFK0IsbUJBQU8sQ0FBQyxHQUFlLENBQUM7O0tBQTNFLGFBQWEsYUFBYixhQUFhO0tBQUUsZUFBZSxhQUFmLGVBQWU7S0FBRSxjQUFjLGFBQWQsY0FBYzs7QUFFcEQsS0FBSSxPQUFPLEdBQUc7O0FBRVosVUFBTyxxQkFBRztBQUNSLFlBQU8sQ0FBQyxRQUFRLENBQUMsYUFBYSxDQUFDLENBQUM7QUFDaEMsWUFBTyxDQUFDLHFCQUFxQixFQUFFLENBQzVCLElBQUksQ0FBQyxZQUFJO0FBQUUsY0FBTyxDQUFDLFFBQVEsQ0FBQyxjQUFjLENBQUMsQ0FBQztNQUFFLENBQUMsQ0FDL0MsSUFBSSxDQUFDLFlBQUk7QUFBRSxjQUFPLENBQUMsUUFBUSxDQUFDLGVBQWUsQ0FBQyxDQUFDO01BQUUsQ0FBQyxDQUFDO0lBQ3JEOztBQUVELHdCQUFxQixtQ0FBRzt1QkFDRixVQUFVLEVBQUU7O1NBQTNCLEtBQUs7U0FBRSxHQUFHOztBQUNmLFlBQU8sQ0FBQyxDQUFDLElBQUksQ0FBQyxVQUFVLEVBQUUsRUFBRSxhQUFhLENBQUMsS0FBSyxFQUFFLEdBQUcsQ0FBQyxDQUFDLENBQUM7SUFDeEQ7RUFDRjs7c0JBRWMsT0FBTzs7Ozs7Ozs7Ozs7QUN4QnRCLEtBQU0sUUFBUSxHQUFHLENBQUMsQ0FBQyxNQUFNLENBQUMsRUFBRSxhQUFHO1VBQUcsR0FBRyxDQUFDLElBQUksRUFBRTtFQUFBLENBQUMsQ0FBQzs7c0JBRS9CO0FBQ2IsV0FBUSxFQUFSLFFBQVE7RUFDVDs7Ozs7Ozs7OztBQ0pELE9BQU0sQ0FBQyxPQUFPLENBQUMsT0FBTyxHQUFHLG1CQUFPLENBQUMsR0FBVyxDQUFDLENBQUM7QUFDOUMsT0FBTSxDQUFDLE9BQU8sQ0FBQyxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxHQUFXLENBQUMsQ0FBQztBQUM5QyxPQUFNLENBQUMsT0FBTyxDQUFDLFFBQVEsR0FBRyxtQkFBTyxDQUFDLEdBQVksQ0FBQyxDOzs7Ozs7Ozs7O0FDRi9DLEtBQU0sT0FBTyxHQUFHLENBQUMsQ0FBQyxjQUFjLENBQUMsRUFBRSxlQUFLO1VBQUcsS0FBSyxDQUFDLElBQUksRUFBRTtFQUFBLENBQUMsQ0FBQzs7c0JBRTFDO0FBQ2IsVUFBTyxFQUFQLE9BQU87RUFDUjs7Ozs7Ozs7OztBQ0pELE9BQU0sQ0FBQyxPQUFPLENBQUMsT0FBTyxHQUFHLG1CQUFPLENBQUMsR0FBVyxDQUFDLENBQUM7QUFDOUMsT0FBTSxDQUFDLE9BQU8sQ0FBQyxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxFQUFXLENBQUMsQ0FBQztBQUM5QyxPQUFNLENBQUMsT0FBTyxDQUFDLFdBQVcsR0FBRyxtQkFBTyxDQUFDLEdBQWUsQ0FBQyxDOzs7Ozs7Ozs7QUNGckQsS0FBSSxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxDQUFhLENBQUMsQ0FBQztBQUNyQyxRQUFPLENBQUMsY0FBYyxDQUFDO0FBQ3JCLFNBQU0sRUFBRSxtQkFBTyxDQUFDLEdBQWdCLENBQUM7QUFDakMsaUJBQWMsRUFBRSxtQkFBTyxDQUFDLEdBQXVCLENBQUM7QUFDaEQseUJBQXNCLEVBQUUsbUJBQU8sQ0FBQyxFQUFrQyxDQUFDO0FBQ25FLGNBQVcsRUFBRSxtQkFBTyxDQUFDLEdBQWtCLENBQUM7QUFDeEMsZUFBWSxFQUFFLG1CQUFPLENBQUMsR0FBbUIsQ0FBQztBQUMxQyxnQkFBYSxFQUFFLG1CQUFPLENBQUMsR0FBc0IsQ0FBQztBQUM5QyxrQkFBZSxFQUFFLG1CQUFPLENBQUMsR0FBd0IsQ0FBQztBQUNsRCxrQkFBZSxFQUFFLG1CQUFPLENBQUMsR0FBeUIsQ0FBQztFQUNwRCxDQUFDLEM7Ozs7Ozs7Ozs7QUNWRixLQUFJLE9BQU8sR0FBRyxtQkFBTyxDQUFDLENBQWEsQ0FBQyxDQUFDOztnQkFDRCxtQkFBTyxDQUFDLEdBQWUsQ0FBQzs7S0FBdEQsd0JBQXdCLFlBQXhCLHdCQUF3Qjs7aUJBQ0wsbUJBQU8sQ0FBQyxFQUErQixDQUFDOztLQUEzRCxlQUFlLGFBQWYsZUFBZTs7QUFDckIsS0FBSSxjQUFjLEdBQUcsbUJBQU8sQ0FBQyxHQUE2QixDQUFDLENBQUM7QUFDNUQsS0FBSSxHQUFHLEdBQUcsbUJBQU8sQ0FBQyxFQUFrQixDQUFDLENBQUM7QUFDdEMsS0FBSSxHQUFHLEdBQUcsbUJBQU8sQ0FBQyxFQUFZLENBQUMsQ0FBQzs7c0JBRWpCO0FBQ2IsY0FBVyx1QkFBQyxXQUFXLEVBQUM7QUFDdEIsU0FBSSxJQUFJLEdBQUcsR0FBRyxDQUFDLEdBQUcsQ0FBQyxZQUFZLENBQUMsV0FBVyxDQUFDLENBQUM7QUFDN0MsbUJBQWMsQ0FBQyxLQUFLLENBQUMsZUFBZSxDQUFDLENBQUM7QUFDdEMsUUFBRyxDQUFDLEdBQUcsQ0FBQyxJQUFJLENBQUMsQ0FBQyxJQUFJLENBQUMsZ0JBQU0sRUFBRTtBQUN6QixxQkFBYyxDQUFDLE9BQU8sQ0FBQyxlQUFlLENBQUMsQ0FBQztBQUN4QyxjQUFPLENBQUMsUUFBUSxDQUFDLHdCQUF3QixFQUFFLE1BQU0sQ0FBQyxDQUFDO01BQ3BELENBQUMsQ0FDRixJQUFJLENBQUMsVUFBQyxHQUFHLEVBQUc7QUFDVixxQkFBYyxDQUFDLElBQUksQ0FBQyxlQUFlLEVBQUUsR0FBRyxDQUFDLFlBQVksQ0FBQyxPQUFPLENBQUMsQ0FBQztNQUNoRSxDQUFDLENBQUM7SUFDSjtFQUNGOzs7Ozs7Ozs7Ozs7Z0JDbkIwQyxtQkFBTyxDQUFDLEVBQStCLENBQUM7O0tBQTlFLGlCQUFpQixZQUFqQixpQkFBaUI7S0FBRSxlQUFlLFlBQWYsZUFBZTs7aUJBQ2pCLG1CQUFPLENBQUMsR0FBNkIsQ0FBQzs7S0FBdkQsYUFBYSxhQUFiLGFBQWE7O0FBRWxCLEtBQU0sTUFBTSxHQUFHLENBQUUsQ0FBQyxhQUFhLENBQUMsRUFBRSxVQUFDLE1BQU07VUFBSyxNQUFNO0VBQUEsQ0FBRSxDQUFDOztzQkFFeEM7QUFDYixTQUFNLEVBQU4sTUFBTTtBQUNOLFNBQU0sRUFBRSxhQUFhLENBQUMsaUJBQWlCLENBQUM7QUFDeEMsaUJBQWMsRUFBRSxhQUFhLENBQUMsZUFBZSxDQUFDO0VBQy9DOzs7Ozs7Ozs7O0FDVEQsT0FBTSxDQUFDLE9BQU8sQ0FBQyxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxHQUFXLENBQUMsQ0FBQztBQUM5QyxPQUFNLENBQUMsT0FBTyxDQUFDLE9BQU8sR0FBRyxtQkFBTyxDQUFDLEdBQVcsQ0FBQyxDQUFDO0FBQzlDLE9BQU0sQ0FBQyxPQUFPLENBQUMsU0FBUyxHQUFHLG1CQUFPLENBQUMsR0FBZSxDQUFDLEM7Ozs7Ozs7OztBQ0ZuRCxPQUFNLENBQUMsT0FBTyxDQUFDLE9BQU8sR0FBRyxtQkFBTyxDQUFDLEVBQVcsQ0FBQyxDQUFDO0FBQzlDLE9BQU0sQ0FBQyxPQUFPLENBQUMsT0FBTyxHQUFHLG1CQUFPLENBQUMsR0FBVyxDQUFDLENBQUM7QUFDOUMsT0FBTSxDQUFDLE9BQU8sQ0FBQyxTQUFTLEdBQUcsbUJBQU8sQ0FBQyxHQUFhLENBQUMsQ0FBQzs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Z0JDRnJCLG1CQUFPLENBQUMsRUFBWSxDQUFDOztLQUE1QyxLQUFLLFlBQUwsS0FBSztLQUFFLFdBQVcsWUFBWCxXQUFXOztpQkFJQyxtQkFBTyxDQUFDLEdBQWUsQ0FBQzs7S0FGL0MsbUJBQW1CLGFBQW5CLG1CQUFtQjtLQUNuQixxQkFBcUIsYUFBckIscUJBQXFCO0tBQ3JCLGtCQUFrQixhQUFsQixrQkFBa0I7c0JBRUwsS0FBSyxDQUFDO0FBQ25CLGtCQUFlLDZCQUFHO0FBQ2hCLFlBQU8sV0FBVyxDQUFDLEVBQUUsQ0FBQyxDQUFDO0lBQ3hCOztBQUVELGFBQVUsd0JBQUc7QUFDWCxTQUFJLENBQUMsRUFBRSxDQUFDLG1CQUFtQixFQUFFLEtBQUssQ0FBQyxDQUFDO0FBQ3BDLFNBQUksQ0FBQyxFQUFFLENBQUMsa0JBQWtCLEVBQUUsSUFBSSxDQUFDLENBQUM7QUFDbEMsU0FBSSxDQUFDLEVBQUUsQ0FBQyxxQkFBcUIsRUFBRSxPQUFPLENBQUMsQ0FBQztJQUN6QztFQUNGLENBQUM7O0FBRUYsVUFBUyxLQUFLLENBQUMsS0FBSyxFQUFFLE9BQU8sRUFBQztBQUM1QixVQUFPLEtBQUssQ0FBQyxHQUFHLENBQUMsT0FBTyxDQUFDLElBQUksRUFBRSxXQUFXLENBQUMsRUFBQyxZQUFZLEVBQUUsSUFBSSxFQUFDLENBQUMsQ0FBQyxDQUFDO0VBQ25FOztBQUVELFVBQVMsSUFBSSxDQUFDLEtBQUssRUFBRSxPQUFPLEVBQUM7QUFDM0IsVUFBTyxLQUFLLENBQUMsR0FBRyxDQUFDLE9BQU8sQ0FBQyxJQUFJLEVBQUUsV0FBVyxDQUFDLEVBQUMsUUFBUSxFQUFFLElBQUksRUFBRSxPQUFPLEVBQUUsT0FBTyxDQUFDLE9BQU8sRUFBQyxDQUFDLENBQUMsQ0FBQztFQUN6Rjs7QUFFRCxVQUFTLE9BQU8sQ0FBQyxLQUFLLEVBQUUsT0FBTyxFQUFDO0FBQzlCLFVBQU8sS0FBSyxDQUFDLEdBQUcsQ0FBQyxPQUFPLENBQUMsSUFBSSxFQUFFLFdBQVcsQ0FBQyxFQUFDLFNBQVMsRUFBRSxJQUFJLEVBQUMsQ0FBQyxDQUFDLENBQUM7RUFDaEU7Ozs7Ozs7Ozs7QUM1QkQsS0FBSSxLQUFLLEdBQUc7O0FBRVYsT0FBSSxrQkFBRTs7QUFFSixZQUFPLHNDQUFzQyxDQUFDLE9BQU8sQ0FBQyxPQUFPLEVBQUUsVUFBUyxDQUFDLEVBQUU7QUFDekUsV0FBSSxDQUFDLEdBQUcsSUFBSSxDQUFDLE1BQU0sRUFBRSxHQUFDLEVBQUUsR0FBQyxDQUFDO1dBQUUsQ0FBQyxHQUFHLENBQUMsSUFBSSxHQUFHLEdBQUcsQ0FBQyxHQUFJLENBQUMsR0FBQyxHQUFHLEdBQUMsR0FBSSxDQUFDO0FBQzNELGNBQU8sQ0FBQyxDQUFDLFFBQVEsQ0FBQyxFQUFFLENBQUMsQ0FBQztNQUN2QixDQUFDLENBQUM7SUFDSjs7QUFFRCxjQUFXLHVCQUFDLElBQUksRUFBQztBQUNmLFNBQUc7QUFDRCxjQUFPLElBQUksQ0FBQyxrQkFBa0IsRUFBRSxHQUFHLEdBQUcsR0FBRyxJQUFJLENBQUMsa0JBQWtCLEVBQUUsQ0FBQztNQUNwRSxRQUFNLEdBQUcsRUFBQztBQUNULGNBQU8sQ0FBQyxLQUFLLENBQUMsR0FBRyxDQUFDLENBQUM7QUFDbkIsY0FBTyxXQUFXLENBQUM7TUFDcEI7SUFDRjs7QUFFRCxlQUFZLHdCQUFDLE1BQU0sRUFBRTtBQUNuQixTQUFJLElBQUksR0FBRyxLQUFLLENBQUMsU0FBUyxDQUFDLEtBQUssQ0FBQyxJQUFJLENBQUMsU0FBUyxFQUFFLENBQUMsQ0FBQyxDQUFDO0FBQ3BELFlBQU8sTUFBTSxDQUFDLE9BQU8sQ0FBQyxJQUFJLE1BQU0sQ0FBQyxjQUFjLEVBQUUsR0FBRyxDQUFDLEVBQ25ELFVBQUMsS0FBSyxFQUFFLE1BQU0sRUFBSztBQUNqQixjQUFPLEVBQUUsSUFBSSxDQUFDLE1BQU0sQ0FBQyxLQUFLLElBQUksSUFBSSxJQUFJLENBQUMsTUFBTSxDQUFDLEtBQUssU0FBUyxDQUFDLEdBQUcsSUFBSSxDQUFDLE1BQU0sQ0FBQyxHQUFHLEVBQUUsQ0FBQztNQUNyRixDQUFDLENBQUM7SUFDSjs7RUFFRjs7QUFFRCxPQUFNLENBQUMsT0FBTyxHQUFHLEtBQUssQzs7Ozs7OztBQzdCdEI7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7QUFDQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0Esa0JBQWlCO0FBQ2pCO0FBQ0E7QUFDQTtBQUNBOztBQUVBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQSxvQkFBbUIsU0FBUztBQUM1QjtBQUNBO0FBQ0E7QUFDQSxJQUFHO0FBQ0g7QUFDQTtBQUNBLGdCQUFlLFNBQVM7QUFDeEI7O0FBRUE7QUFDQTtBQUNBLGdCQUFlLFNBQVM7QUFDeEI7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBLE1BQUs7QUFDTDtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBOztBQUVBOztBQUVBO0FBQ0E7QUFDQTs7QUFFQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUEsSUFBRztBQUNILHFCQUFvQixTQUFTO0FBQzdCO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBLE1BQUs7QUFDTDtBQUNBOztBQUVBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBOztBQUVBOztBQUVBO0FBQ0E7QUFDQSxJQUFHO0FBQ0g7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTs7Ozs7Ozs7Ozs7O0FDNVNBLEtBQUksS0FBSyxHQUFHLG1CQUFPLENBQUMsQ0FBTyxDQUFDLENBQUM7QUFDN0IsS0FBSSxVQUFVLEdBQUcsbUJBQU8sQ0FBQyxHQUFjLENBQUMsQ0FBQztBQUN6QyxLQUFJLEdBQUcsR0FBRyxtQkFBTyxDQUFDLEVBQVksQ0FBQyxDQUFDO0FBQ2hDLEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsQ0FBYSxDQUFDLENBQUM7O2dCQUNaLG1CQUFPLENBQUMsR0FBaUIsQ0FBQzs7S0FBOUMsT0FBTyxZQUFQLE9BQU87S0FBRSxPQUFPLFlBQVAsT0FBTzs7QUFDckIsS0FBSSxnQkFBZ0IsR0FBRyxtQkFBTyxDQUFDLEdBQXdCLENBQUMsQ0FBQzs7QUFFekQsS0FBSSxHQUFHLEdBQUcsS0FBSyxDQUFDLFdBQVcsQ0FBQzs7O0FBRTFCLFNBQU0sRUFBRSxDQUFDLE9BQU8sQ0FBQyxVQUFVLENBQUM7O0FBRTVCLGtCQUFlLDZCQUFHO0FBQ2hCLFlBQU87QUFDTCxVQUFHLEVBQUUsT0FBTyxDQUFDLFFBQVE7TUFDdEI7SUFDRjs7QUFFRCxxQkFBa0IsZ0NBQUU7QUFDbEIsWUFBTyxDQUFDLE9BQU8sRUFBRSxDQUFDO0FBQ2xCLFNBQUksQ0FBQyxlQUFlLEdBQUcsV0FBVyxDQUFDLE9BQU8sQ0FBQyxxQkFBcUIsRUFBRSxLQUFLLENBQUMsQ0FBQztJQUMxRTs7QUFFRCx1QkFBb0IsRUFBRSxnQ0FBVztBQUMvQixrQkFBYSxDQUFDLElBQUksQ0FBQyxlQUFlLENBQUMsQ0FBQztJQUNyQzs7QUFFRCxTQUFNLEVBQUUsa0JBQVc7QUFDakIsU0FBRyxJQUFJLENBQUMsS0FBSyxDQUFDLEdBQUcsQ0FBQyxjQUFjLEVBQUM7QUFDL0IsY0FBTyxJQUFJLENBQUM7TUFDYjs7QUFFRCxZQUNFOztTQUFLLFNBQVMsRUFBQyxnQ0FBZ0M7T0FDN0Msb0JBQUMsZ0JBQWdCLE9BQUU7T0FDbEIsSUFBSSxDQUFDLEtBQUssQ0FBQyxrQkFBa0I7T0FDOUIsb0JBQUMsVUFBVSxPQUFFO09BQ1osSUFBSSxDQUFDLEtBQUssQ0FBQyxRQUFRO01BQ2hCLENBQ047SUFDSDtFQUNGLENBQUM7O0FBRUYsT0FBTSxDQUFDLE9BQU8sR0FBRyxHQUFHLEM7Ozs7Ozs7Ozs7Ozs7OztBQzFDcEIsS0FBSSxLQUFLLEdBQUcsbUJBQU8sQ0FBQyxDQUFPLENBQUMsQ0FBQzs7Z0JBQ0osbUJBQU8sQ0FBQyxFQUE2QixDQUFDOztLQUExRCxPQUFPLFlBQVAsT0FBTztLQUFFLE9BQU8sWUFBUCxPQUFPOztBQUNyQixLQUFJLEdBQUcsR0FBRyxtQkFBTyxDQUFDLEVBQWdCLENBQUMsQ0FBQztBQUNwQyxLQUFJLFdBQVcsR0FBRyxtQkFBTyxDQUFDLEdBQW1CLENBQUMsQ0FBQztBQUMvQyxLQUFJLGFBQWEsR0FBRyxtQkFBTyxDQUFDLEdBQXFCLENBQUMsQ0FBQztBQUNuRCxLQUFJLGdCQUFnQixHQUFHLG1CQUFPLENBQUMsR0FBb0IsQ0FBQyxDQUFDOztpQkFDRCxtQkFBTyxDQUFDLEVBQTZCLENBQUM7O0tBQXJGLG9CQUFvQixhQUFwQixvQkFBb0I7S0FBRSxxQkFBcUIsYUFBckIscUJBQXFCOztBQUNoRCxLQUFJLGdCQUFnQixHQUFHLG1CQUFPLENBQUMsR0FBMkIsQ0FBQyxDQUFDOztBQUU1RCxLQUFJLGFBQWEsR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFFcEMsdUJBQW9CLGtDQUFFO0FBQ3BCLDBCQUFxQixFQUFFLENBQUM7SUFDekI7O0FBRUQsU0FBTSxFQUFFLGtCQUFXO2dDQUNnQixJQUFJLENBQUMsS0FBSyxDQUFDLGFBQWE7U0FBcEQsUUFBUSx3QkFBUixRQUFRO1NBQUUsS0FBSyx3QkFBTCxLQUFLO1NBQUUsT0FBTyx3QkFBUCxPQUFPOztBQUM3QixTQUFJLGVBQWUsR0FBTSxLQUFLLFNBQUksUUFBVSxDQUFDOztBQUU3QyxTQUFHLENBQUMsUUFBUSxFQUFDO0FBQ1gsc0JBQWUsR0FBRyxFQUFFLENBQUM7TUFDdEI7O0FBRUQsWUFDQzs7U0FBSyxTQUFTLEVBQUMscUJBQXFCO09BQ2xDLG9CQUFDLGdCQUFnQixJQUFDLE9BQU8sRUFBRSxPQUFRLEdBQUU7T0FDckM7O1dBQUssU0FBUyxFQUFDLGlDQUFpQztTQUM5Qzs7O1dBQUssZUFBZTtVQUFNO1FBQ3RCO09BQ04sb0JBQUMsYUFBYSxFQUFLLElBQUksQ0FBQyxLQUFLLENBQUMsYUFBYSxDQUFJO01BQzNDLENBQ0o7SUFDSjtFQUNGLENBQUMsQ0FBQzs7QUFFSCxLQUFJLGFBQWEsR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFFcEMsa0JBQWUsNkJBQUc7OztBQUNoQixTQUFJLENBQUMsR0FBRyxHQUFHLElBQUksR0FBRyxDQUFDLElBQUksQ0FBQyxLQUFLLENBQUM7QUFDOUIsU0FBSSxDQUFDLEdBQUcsQ0FBQyxFQUFFLENBQUMsTUFBTSxFQUFFO2NBQUssTUFBSyxRQUFRLGNBQU0sTUFBSyxLQUFLLElBQUUsV0FBVyxFQUFFLElBQUksSUFBRztNQUFBLENBQUMsQ0FBQzs7a0JBRXRELElBQUksQ0FBQyxLQUFLO1NBQTdCLFFBQVEsVUFBUixRQUFRO1NBQUUsS0FBSyxVQUFMLEtBQUs7O0FBQ3BCLFlBQU8sRUFBQyxRQUFRLEVBQVIsUUFBUSxFQUFFLEtBQUssRUFBTCxLQUFLLEVBQUUsV0FBVyxFQUFFLEtBQUssRUFBQyxDQUFDO0lBQzlDOztBQUVELG9CQUFpQiwrQkFBRTs7QUFFakIscUJBQWdCLENBQUMsc0JBQXNCLEdBQUcsSUFBSSxDQUFDLHlCQUF5QixDQUFDLElBQUksQ0FBQyxJQUFJLENBQUMsQ0FBQztJQUNyRjs7QUFFRCx1QkFBb0Isa0NBQUc7QUFDckIscUJBQWdCLENBQUMsc0JBQXNCLEdBQUcsSUFBSSxDQUFDO0FBQy9DLFNBQUksQ0FBQyxHQUFHLENBQUMsVUFBVSxFQUFFLENBQUM7SUFDdkI7O0FBRUQsNEJBQXlCLHFDQUFDLFNBQVMsRUFBQztTQUM3QixRQUFRLEdBQUksU0FBUyxDQUFyQixRQUFROztBQUNiLFNBQUcsUUFBUSxJQUFJLFFBQVEsS0FBSyxJQUFJLENBQUMsS0FBSyxDQUFDLFFBQVEsRUFBQztBQUM5QyxXQUFJLENBQUMsR0FBRyxDQUFDLFNBQVMsQ0FBQyxFQUFDLFFBQVEsRUFBUixRQUFRLEVBQUMsQ0FBQyxDQUFDO0FBQy9CLFdBQUksQ0FBQyxJQUFJLENBQUMsZUFBZSxDQUFDLElBQUksQ0FBQyxLQUFLLEVBQUUsQ0FBQztBQUN2QyxXQUFJLENBQUMsUUFBUSxjQUFLLElBQUksQ0FBQyxLQUFLLElBQUUsUUFBUSxFQUFSLFFBQVEsSUFBRyxDQUFDO01BQzNDO0lBQ0Y7O0FBRUQsU0FBTSxvQkFBRztBQUNQLFlBQ0U7O1NBQUssS0FBSyxFQUFFLEVBQUMsTUFBTSxFQUFFLE1BQU0sRUFBRTtPQUMzQixvQkFBQyxXQUFXLElBQUMsR0FBRyxFQUFDLGlCQUFpQixFQUFDLEdBQUcsRUFBRSxJQUFJLENBQUMsR0FBSSxFQUFDLElBQUksRUFBRSxJQUFJLENBQUMsS0FBSyxDQUFDLElBQUssRUFBQyxJQUFJLEVBQUUsSUFBSSxDQUFDLEtBQUssQ0FBQyxJQUFLLEdBQUc7T0FDaEcsSUFBSSxDQUFDLEtBQUssQ0FBQyxXQUFXLEdBQUcsb0JBQUMsYUFBYSxJQUFDLEdBQUcsRUFBRSxJQUFJLENBQUMsS0FBSyxDQUFDLEdBQUksR0FBRSxHQUFHLElBQUk7TUFDbkUsQ0FDUDtJQUNGO0VBQ0YsQ0FBQyxDQUFDOztBQUVILE9BQU0sQ0FBQyxPQUFPLEdBQUcsYUFBYSxDOzs7Ozs7Ozs7Ozs7OztBQzFFOUIsS0FBSSxHQUFHLEdBQUcsbUJBQU8sQ0FBQyxFQUFZLENBQUMsQ0FBQztBQUNoQyxLQUFJLEtBQUssR0FBRyxtQkFBTyxDQUFDLENBQU8sQ0FBQyxDQUFDO0FBQzdCLEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsRUFBYSxDQUFDLENBQUM7O2dCQUNmLG1CQUFPLENBQUMsRUFBOEIsQ0FBQzs7S0FBeEQsYUFBYSxZQUFiLGFBQWE7O0FBRWxCLEtBQUksYUFBYSxHQUFHLEtBQUssQ0FBQyxXQUFXLENBQUM7OztBQUNwQyxvQkFBaUIsK0JBQUc7U0FDYixHQUFHLEdBQUksSUFBSSxDQUFDLEtBQUssQ0FBakIsR0FBRzs7Z0NBQ00sT0FBTyxDQUFDLFdBQVcsRUFBRTs7U0FBOUIsS0FBSyx3QkFBTCxLQUFLOztBQUNWLFNBQUksT0FBTyxHQUFHLEdBQUcsQ0FBQyxHQUFHLENBQUMscUJBQXFCLENBQUMsS0FBSyxFQUFFLEdBQUcsQ0FBQyxDQUFDOztBQUV4RCxTQUFJLENBQUMsTUFBTSxHQUFHLElBQUksU0FBUyxDQUFDLE9BQU8sRUFBRSxPQUFPLENBQUMsQ0FBQztBQUM5QyxTQUFJLENBQUMsTUFBTSxDQUFDLFNBQVMsR0FBRyxVQUFDLEtBQUssRUFBSztBQUNqQyxXQUNBO0FBQ0UsYUFBSSxJQUFJLEdBQUcsSUFBSSxDQUFDLEtBQUssQ0FBQyxLQUFLLENBQUMsSUFBSSxDQUFDLENBQUM7QUFDbEMsc0JBQWEsQ0FBQyxJQUFJLENBQUMsT0FBTyxDQUFDLENBQUM7UUFDN0IsQ0FDRCxPQUFNLEdBQUcsRUFBQztBQUNSLGdCQUFPLENBQUMsR0FBRyxDQUFDLG1DQUFtQyxDQUFDLENBQUM7UUFDbEQ7TUFFRixDQUFDO0FBQ0YsU0FBSSxDQUFDLE1BQU0sQ0FBQyxPQUFPLEdBQUcsWUFBTSxFQUFFLENBQUM7SUFDaEM7O0FBRUQsdUJBQW9CLGtDQUFHO0FBQ3JCLFNBQUksQ0FBQyxNQUFNLENBQUMsS0FBSyxFQUFFLENBQUM7SUFDckI7O0FBRUQsd0JBQXFCLG1DQUFHO0FBQ3RCLFlBQU8sS0FBSyxDQUFDO0lBQ2Q7O0FBRUQsU0FBTSxvQkFBRztBQUNQLFlBQU8sSUFBSSxDQUFDO0lBQ2I7RUFDRixDQUFDLENBQUM7O3NCQUVZLGFBQWE7Ozs7Ozs7Ozs7Ozs7O0FDdkM1QixLQUFJLEtBQUssR0FBRyxtQkFBTyxDQUFDLENBQU8sQ0FBQyxDQUFDOztnQkFDSixtQkFBTyxDQUFDLEVBQTZCLENBQUM7O0tBQTFELE9BQU8sWUFBUCxPQUFPO0tBQUUsT0FBTyxZQUFQLE9BQU87O0FBQ3JCLEtBQUksYUFBYSxHQUFHLG1CQUFPLENBQUMsR0FBcUIsQ0FBQyxDQUFDO0FBQ25ELEtBQUksYUFBYSxHQUFHLG1CQUFPLENBQUMsR0FBcUIsQ0FBQyxDQUFDOztBQUVuRCxLQUFJLGtCQUFrQixHQUFHLEtBQUssQ0FBQyxXQUFXLENBQUM7OztBQUV6QyxTQUFNLEVBQUUsQ0FBQyxPQUFPLENBQUMsVUFBVSxDQUFDOztBQUU1QixrQkFBZSw2QkFBRztBQUNoQixZQUFPO0FBQ0wscUJBQWMsRUFBRSxPQUFPLENBQUMsYUFBYTtNQUN0QztJQUNGOztBQUVELG9CQUFpQiwrQkFBRTtTQUNYLEdBQUcsR0FBSyxJQUFJLENBQUMsS0FBSyxDQUFDLE1BQU0sQ0FBekIsR0FBRzs7QUFDVCxTQUFHLENBQUMsSUFBSSxDQUFDLEtBQUssQ0FBQyxjQUFjLEVBQUM7QUFDNUIsY0FBTyxDQUFDLFdBQVcsQ0FBQyxHQUFHLENBQUMsQ0FBQztNQUMxQjtJQUNGOztBQUVELFNBQU0sRUFBRSxrQkFBVztBQUNqQixTQUFJLGNBQWMsR0FBRyxJQUFJLENBQUMsS0FBSyxDQUFDLGNBQWMsQ0FBQztBQUMvQyxTQUFHLENBQUMsY0FBYyxFQUFDO0FBQ2pCLGNBQU8sSUFBSSxDQUFDO01BQ2I7O0FBRUQsU0FBRyxjQUFjLENBQUMsWUFBWSxJQUFJLGNBQWMsQ0FBQyxNQUFNLEVBQUM7QUFDdEQsY0FBTyxvQkFBQyxhQUFhLElBQUMsYUFBYSxFQUFFLGNBQWUsR0FBRSxDQUFDO01BQ3hEOztBQUVELFlBQU8sb0JBQUMsYUFBYSxJQUFDLGFBQWEsRUFBRSxjQUFlLEdBQUUsQ0FBQztJQUN4RDtFQUNGLENBQUMsQ0FBQzs7QUFFSCxPQUFNLENBQUMsT0FBTyxHQUFHLGtCQUFrQixDOzs7Ozs7Ozs7Ozs7OztBQ3BDbkMsS0FBSSxLQUFLLEdBQUcsbUJBQU8sQ0FBQyxDQUFPLENBQUMsQ0FBQztBQUM3QixLQUFJLFdBQVcsR0FBRyxtQkFBTyxDQUFDLEdBQWMsQ0FBQyxDQUFDO0FBQzFDLEtBQUksU0FBUyxHQUFHLG1CQUFPLENBQUMsR0FBc0IsQ0FBQztBQUMvQyxLQUFJLFdBQVcsR0FBRyxtQkFBTyxDQUFDLEdBQW1CLENBQUMsQ0FBQztBQUMvQyxLQUFJLGdCQUFnQixHQUFHLG1CQUFPLENBQUMsR0FBb0IsQ0FBQyxDQUFDOztBQUVyRCxLQUFJLGFBQWEsR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFDcEMsaUJBQWMsNEJBQUU7QUFDZCxZQUFPO0FBQ0wsYUFBTSxFQUFFLElBQUksQ0FBQyxHQUFHLENBQUMsTUFBTTtBQUN2QixVQUFHLEVBQUUsQ0FBQztBQUNOLGdCQUFTLEVBQUUsSUFBSSxDQUFDLEdBQUcsQ0FBQyxTQUFTO0FBQzdCLGNBQU8sRUFBRSxJQUFJLENBQUMsR0FBRyxDQUFDLE9BQU87QUFDekIsY0FBTyxFQUFFLElBQUksQ0FBQyxHQUFHLENBQUMsTUFBTSxHQUFHLENBQUM7TUFDN0IsQ0FBQztJQUNIOztBQUVELGtCQUFlLDZCQUFHO0FBQ2hCLFNBQUksR0FBRyxHQUFHLElBQUksQ0FBQyxLQUFLLENBQUMsYUFBYSxDQUFDLEdBQUcsQ0FBQztBQUN2QyxTQUFJLENBQUMsR0FBRyxHQUFHLElBQUksU0FBUyxDQUFDLEVBQUMsR0FBRyxFQUFILEdBQUcsRUFBQyxDQUFDLENBQUM7QUFDaEMsWUFBTyxJQUFJLENBQUMsY0FBYyxFQUFFLENBQUM7SUFDOUI7O0FBRUQsdUJBQW9CLGtDQUFHO0FBQ3JCLFNBQUksQ0FBQyxHQUFHLENBQUMsSUFBSSxFQUFFLENBQUM7QUFDaEIsU0FBSSxDQUFDLEdBQUcsQ0FBQyxrQkFBa0IsRUFBRSxDQUFDO0lBQy9COztBQUVELG9CQUFpQiwrQkFBRzs7O0FBQ2xCLFNBQUksQ0FBQyxHQUFHLENBQUMsRUFBRSxDQUFDLFFBQVEsRUFBRSxZQUFJO0FBQ3hCLFdBQUksUUFBUSxHQUFHLE1BQUssY0FBYyxFQUFFLENBQUM7QUFDckMsYUFBSyxRQUFRLENBQUMsUUFBUSxDQUFDLENBQUM7TUFDekIsQ0FBQyxDQUFDO0lBQ0o7O0FBRUQsaUJBQWMsNEJBQUU7QUFDZCxTQUFHLElBQUksQ0FBQyxLQUFLLENBQUMsU0FBUyxFQUFDO0FBQ3RCLFdBQUksQ0FBQyxHQUFHLENBQUMsSUFBSSxFQUFFLENBQUM7TUFDakIsTUFBSTtBQUNILFdBQUksQ0FBQyxHQUFHLENBQUMsSUFBSSxFQUFFLENBQUM7TUFDakI7SUFDRjs7QUFFRCxPQUFJLGdCQUFDLEtBQUssRUFBQztBQUNULFNBQUksQ0FBQyxHQUFHLENBQUMsSUFBSSxDQUFDLEtBQUssQ0FBQyxDQUFDO0lBQ3RCOztBQUVELGlCQUFjLDRCQUFFO0FBQ2QsU0FBSSxDQUFDLEdBQUcsQ0FBQyxJQUFJLEVBQUUsQ0FBQztJQUNqQjs7QUFFRCxnQkFBYSx5QkFBQyxLQUFLLEVBQUM7QUFDbEIsU0FBSSxDQUFDLEdBQUcsQ0FBQyxJQUFJLEVBQUUsQ0FBQztBQUNoQixTQUFJLENBQUMsR0FBRyxDQUFDLElBQUksQ0FBQyxLQUFLLENBQUMsQ0FBQztJQUN0Qjs7QUFFRCxTQUFNLEVBQUUsa0JBQVc7U0FDWixTQUFTLEdBQUksSUFBSSxDQUFDLEtBQUssQ0FBdkIsU0FBUzs7QUFFZCxZQUNDOztTQUFLLFNBQVMsRUFBQyx3Q0FBd0M7T0FDckQsb0JBQUMsZ0JBQWdCLE9BQUU7T0FDbkIsb0JBQUMsV0FBVyxJQUFDLEdBQUcsRUFBQyxNQUFNLEVBQUMsR0FBRyxFQUFFLElBQUksQ0FBQyxHQUFJLEVBQUMsSUFBSSxFQUFDLEdBQUcsRUFBQyxJQUFJLEVBQUMsR0FBRyxHQUFHO09BQzNELG9CQUFDLFdBQVc7QUFDVCxZQUFHLEVBQUUsSUFBSSxDQUFDLEtBQUssQ0FBQyxHQUFJO0FBQ3BCLFlBQUcsRUFBRSxJQUFJLENBQUMsS0FBSyxDQUFDLE1BQU87QUFDdkIsY0FBSyxFQUFFLElBQUksQ0FBQyxLQUFLLENBQUMsT0FBUTtBQUMxQixzQkFBYSxFQUFFLElBQUksQ0FBQyxhQUFjO0FBQ2xDLHVCQUFjLEVBQUUsSUFBSSxDQUFDLGNBQWU7QUFDcEMscUJBQVksRUFBRSxDQUFFO0FBQ2hCLGlCQUFRO0FBQ1Isa0JBQVMsRUFBQyxZQUFZLEdBQ1g7T0FDZDs7V0FBUSxTQUFTLEVBQUMsS0FBSyxFQUFDLE9BQU8sRUFBRSxJQUFJLENBQUMsY0FBZTtTQUNqRCxTQUFTLEdBQUcsMkJBQUcsU0FBUyxFQUFDLFlBQVksR0FBSyxHQUFJLDJCQUFHLFNBQVMsRUFBQyxZQUFZLEdBQUs7UUFDdkU7TUFDTCxDQUNKO0lBQ0o7RUFDRixDQUFDLENBQUM7O3NCQUVZLGFBQWE7Ozs7Ozs7Ozs7Ozs7O0FDakY1QixPQUFNLENBQUMsT0FBTyxDQUFDLEdBQUcsR0FBRyxtQkFBTyxDQUFDLEdBQVcsQ0FBQyxDQUFDO0FBQzFDLE9BQU0sQ0FBQyxPQUFPLENBQUMsS0FBSyxHQUFHLG1CQUFPLENBQUMsR0FBYSxDQUFDLENBQUM7QUFDOUMsT0FBTSxDQUFDLE9BQU8sQ0FBQyxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxHQUFlLENBQUMsQ0FBQztBQUNsRCxPQUFNLENBQUMsT0FBTyxDQUFDLEtBQUssR0FBRyxtQkFBTyxDQUFDLEdBQWtCLENBQUMsQ0FBQztBQUNuRCxPQUFNLENBQUMsT0FBTyxDQUFDLFFBQVEsR0FBRyxtQkFBTyxDQUFDLEdBQXFCLENBQUMsQ0FBQztBQUN6RCxPQUFNLENBQUMsT0FBTyxDQUFDLGtCQUFrQixHQUFHLG1CQUFPLENBQUMsR0FBMkIsQ0FBQyxDQUFDO0FBQ3pFLE9BQU0sQ0FBQyxPQUFPLENBQUMsUUFBUSxHQUFHLG1CQUFPLENBQUMsR0FBaUIsQ0FBQyxDQUFDLFFBQVEsQzs7Ozs7Ozs7Ozs7OztBQ043RCxLQUFJLEtBQUssR0FBRyxtQkFBTyxDQUFDLENBQU8sQ0FBQyxDQUFDO0FBQzdCLEtBQUksQ0FBQyxHQUFHLG1CQUFPLENBQUMsRUFBUSxDQUFDLENBQUM7QUFDMUIsS0FBSSxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxDQUFhLENBQUMsQ0FBQztBQUNyQyxLQUFJLGdCQUFnQixHQUFHLG1CQUFPLENBQUMsRUFBaUMsQ0FBQyxDQUFDOztnQkFDekMsbUJBQU8sQ0FBQyxHQUFrQixDQUFDOztLQUEvQyxPQUFPLFlBQVAsT0FBTztLQUFFLE9BQU8sWUFBUCxPQUFPOztBQUNyQixLQUFJLGNBQWMsR0FBRyxtQkFBTyxDQUFDLEdBQWtCLENBQUMsQ0FBQztBQUNqRCxLQUFJLEdBQUcsR0FBRyxtQkFBTyxDQUFDLEVBQVksQ0FBQyxDQUFDOztBQUVoQyxLQUFJLGNBQWMsR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFFckMsU0FBTSxFQUFFLENBQUMsZ0JBQWdCLENBQUM7O0FBRTFCLGtCQUFlLDZCQUFHO0FBQ2hCLFlBQU87QUFDTCxXQUFJLEVBQUUsRUFBRTtBQUNSLGVBQVEsRUFBRSxFQUFFO0FBQ1osWUFBSyxFQUFFLEVBQUU7TUFDVjtJQUNGOztBQUVELFVBQU8sRUFBRSxpQkFBUyxDQUFDLEVBQUU7QUFDbkIsTUFBQyxDQUFDLGNBQWMsRUFBRSxDQUFDO0FBQ25CLFNBQUksSUFBSSxDQUFDLE9BQU8sRUFBRSxFQUFFO0FBQ2xCLFdBQUksQ0FBQyxLQUFLLENBQUMsT0FBTyxDQUFDLElBQUksQ0FBQyxLQUFLLENBQUMsQ0FBQztNQUNoQztJQUNGOztBQUVELFVBQU8sRUFBRSxtQkFBVztBQUNsQixTQUFJLEtBQUssR0FBRyxDQUFDLENBQUMsSUFBSSxDQUFDLElBQUksQ0FBQyxJQUFJLENBQUMsQ0FBQztBQUM5QixZQUFPLEtBQUssQ0FBQyxNQUFNLEtBQUssQ0FBQyxJQUFJLEtBQUssQ0FBQyxLQUFLLEVBQUUsQ0FBQztJQUM1Qzs7QUFFRCxTQUFNLG9CQUFHO3lCQUNrQyxJQUFJLENBQUMsS0FBSyxDQUFDLE1BQU07U0FBckQsWUFBWSxpQkFBWixZQUFZO1NBQUUsUUFBUSxpQkFBUixRQUFRO1NBQUUsT0FBTyxpQkFBUCxPQUFPOztBQUVwQyxZQUNFOztTQUFNLEdBQUcsRUFBQyxNQUFNLEVBQUMsU0FBUyxFQUFDLHNCQUFzQjtPQUMvQzs7OztRQUE4QjtPQUM5Qjs7V0FBSyxTQUFTLEVBQUMsRUFBRTtTQUNmOzthQUFLLFNBQVMsRUFBQyxZQUFZO1dBQ3pCLCtCQUFPLFNBQVMsUUFBQyxTQUFTLEVBQUUsSUFBSSxDQUFDLFNBQVMsQ0FBQyxNQUFNLENBQUUsRUFBQyxTQUFTLEVBQUMsdUJBQXVCLEVBQUMsV0FBVyxFQUFDLFdBQVcsRUFBQyxJQUFJLEVBQUMsVUFBVSxHQUFHO1VBQzVIO1NBQ047O2FBQUssU0FBUyxFQUFDLFlBQVk7V0FDekIsK0JBQU8sU0FBUyxFQUFFLElBQUksQ0FBQyxTQUFTLENBQUMsVUFBVSxDQUFFLEVBQUMsSUFBSSxFQUFDLFVBQVUsRUFBQyxJQUFJLEVBQUMsVUFBVSxFQUFDLFNBQVMsRUFBQyx1QkFBdUIsRUFBQyxXQUFXLEVBQUMsVUFBVSxHQUFFO1VBQ3BJO1NBQ047O2FBQUssU0FBUyxFQUFDLFlBQVk7V0FDekIsK0JBQU8sU0FBUyxFQUFFLElBQUksQ0FBQyxTQUFTLENBQUMsT0FBTyxDQUFFLEVBQUMsU0FBUyxFQUFDLHVCQUF1QixFQUFDLElBQUksRUFBQyxPQUFPLEVBQUMsV0FBVyxFQUFDLHlDQUF5QyxHQUFFO1VBQzdJO1NBQ047O2FBQVEsT0FBTyxFQUFFLElBQUksQ0FBQyxPQUFRLEVBQUMsUUFBUSxFQUFFLFlBQWEsRUFBQyxJQUFJLEVBQUMsUUFBUSxFQUFDLFNBQVMsRUFBQyxzQ0FBc0M7O1VBQWU7U0FDbEksUUFBUSxHQUFJOzthQUFPLFNBQVMsRUFBQyxPQUFPO1dBQUUsT0FBTztVQUFTLEdBQUksSUFBSTtRQUM1RDtNQUNELENBQ1A7SUFDSDtFQUNGLENBQUM7O0FBRUYsS0FBSSxLQUFLLEdBQUcsS0FBSyxDQUFDLFdBQVcsQ0FBQzs7O0FBRTVCLFNBQU0sRUFBRSxDQUFDLE9BQU8sQ0FBQyxVQUFVLENBQUM7O0FBRTVCLGtCQUFlLDZCQUFHO0FBQ2hCLFlBQU87QUFDTCxhQUFNLEVBQUUsT0FBTyxDQUFDLFdBQVc7TUFDNUI7SUFDRjs7QUFFRCxVQUFPLG1CQUFDLFNBQVMsRUFBQztBQUNoQixTQUFJLEdBQUcsR0FBRyxJQUFJLENBQUMsS0FBSyxDQUFDLFFBQVEsQ0FBQztBQUM5QixTQUFJLFFBQVEsR0FBRyxHQUFHLENBQUMsTUFBTSxDQUFDLEdBQUcsQ0FBQzs7QUFFOUIsU0FBRyxHQUFHLENBQUMsS0FBSyxJQUFJLEdBQUcsQ0FBQyxLQUFLLENBQUMsVUFBVSxFQUFDO0FBQ25DLGVBQVEsR0FBRyxHQUFHLENBQUMsS0FBSyxDQUFDLFVBQVUsQ0FBQztNQUNqQzs7QUFFRCxZQUFPLENBQUMsS0FBSyxDQUFDLFNBQVMsRUFBRSxRQUFRLENBQUMsQ0FBQztJQUNwQzs7QUFFRCxTQUFNLG9CQUFHO0FBQ1AsWUFDRTs7U0FBSyxTQUFTLEVBQUMsdUJBQXVCO09BQ3BDLDZCQUFLLFNBQVMsRUFBQyxlQUFlLEdBQU87T0FDckM7O1dBQUssU0FBUyxFQUFDLHNCQUFzQjtTQUNuQzs7YUFBSyxTQUFTLEVBQUMsaUJBQWlCO1dBQzlCLG9CQUFDLGNBQWMsSUFBQyxNQUFNLEVBQUUsSUFBSSxDQUFDLEtBQUssQ0FBQyxNQUFPLEVBQUMsT0FBTyxFQUFFLElBQUksQ0FBQyxPQUFRLEdBQUU7V0FDbkUsb0JBQUMsY0FBYyxPQUFFO1dBQ2pCOztlQUFLLFNBQVMsRUFBQyxnQkFBZ0I7YUFDN0IsMkJBQUcsU0FBUyxFQUFDLGdCQUFnQixHQUFLO2FBQ2xDOzs7O2NBQWdEO2FBQ2hEOzs7O2NBQTZEO1lBQ3pEO1VBQ0Y7UUFDRjtNQUNGLENBQ047SUFDSDtFQUNGLENBQUMsQ0FBQzs7QUFFSCxPQUFNLENBQUMsT0FBTyxHQUFHLEtBQUssQzs7Ozs7Ozs7Ozs7OztBQ2pHdEIsS0FBSSxLQUFLLEdBQUcsbUJBQU8sQ0FBQyxDQUFPLENBQUMsQ0FBQzs7Z0JBQ1EsbUJBQU8sQ0FBQyxFQUFjLENBQUM7O0tBQXRELE1BQU0sWUFBTixNQUFNO0tBQUUsU0FBUyxZQUFULFNBQVM7S0FBRSxPQUFPLFlBQVAsT0FBTzs7QUFDaEMsS0FBSSxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxFQUEwQixDQUFDLENBQUM7QUFDbEQsS0FBSSxHQUFHLEdBQUcsbUJBQU8sQ0FBQyxFQUFZLENBQUMsQ0FBQzs7QUFFaEMsS0FBSSxTQUFTLEdBQUcsQ0FDZCxFQUFDLElBQUksRUFBRSxZQUFZLEVBQUUsRUFBRSxFQUFFLEdBQUcsQ0FBQyxNQUFNLENBQUMsS0FBSyxFQUFFLEtBQUssRUFBRSxPQUFPLEVBQUMsRUFDMUQsRUFBQyxJQUFJLEVBQUUsZUFBZSxFQUFFLEVBQUUsRUFBRSxHQUFHLENBQUMsTUFBTSxDQUFDLFFBQVEsRUFBRSxLQUFLLEVBQUUsVUFBVSxFQUFDLENBQ3BFLENBQUM7O0FBRUYsS0FBSSxVQUFVLEdBQUcsS0FBSyxDQUFDLFdBQVcsQ0FBQzs7O0FBRWpDLFNBQU0sRUFBRSxrQkFBVTs7O0FBQ2hCLFNBQUksS0FBSyxHQUFHLFNBQVMsQ0FBQyxHQUFHLENBQUMsVUFBQyxDQUFDLEVBQUUsS0FBSyxFQUFHO0FBQ3BDLFdBQUksU0FBUyxHQUFHLE1BQUssT0FBTyxDQUFDLE1BQU0sQ0FBQyxRQUFRLENBQUMsQ0FBQyxDQUFDLEVBQUUsQ0FBQyxHQUFHLFFBQVEsR0FBRyxFQUFFLENBQUM7QUFDbkUsY0FDRTs7V0FBSSxHQUFHLEVBQUUsS0FBTSxFQUFDLFNBQVMsRUFBRSxTQUFVLEVBQUMsS0FBSyxFQUFFLENBQUMsQ0FBQyxLQUFNO1NBQ25EO0FBQUMsb0JBQVM7YUFBQyxFQUFFLEVBQUUsQ0FBQyxDQUFDLEVBQUc7V0FDbEIsMkJBQUcsU0FBUyxFQUFFLENBQUMsQ0FBQyxJQUFLLEdBQUc7VUFDZDtRQUNULENBQ0w7TUFDSCxDQUFDLENBQUM7O0FBRUgsVUFBSyxDQUFDLElBQUksQ0FDUjs7U0FBSSxHQUFHLEVBQUUsS0FBSyxDQUFDLE1BQU8sRUFBQyxLQUFLLEVBQUMsTUFBTTtPQUNqQzs7V0FBRyxJQUFJLEVBQUUsR0FBRyxDQUFDLE9BQVEsRUFBQyxNQUFNLEVBQUMsUUFBUTtTQUNuQywyQkFBRyxTQUFTLEVBQUMsZ0JBQWdCLEdBQUc7UUFDOUI7TUFDRCxDQUFFLENBQUM7O0FBRVYsVUFBSyxDQUFDLElBQUksQ0FDUjs7U0FBSSxHQUFHLEVBQUUsS0FBSyxDQUFDLE1BQU8sRUFBQyxLQUFLLEVBQUMsUUFBUTtPQUNuQzs7V0FBRyxJQUFJLEVBQUUsR0FBRyxDQUFDLE1BQU0sQ0FBQyxNQUFPO1NBQ3pCLDJCQUFHLFNBQVMsRUFBQyxnQkFBZ0IsR0FBSztRQUNoQztNQUNELENBQ0wsQ0FBQzs7QUFFSCxZQUNFOztTQUFLLFNBQVMsRUFBQyx3QkFBd0IsRUFBQyxJQUFJLEVBQUMsWUFBWTtPQUN2RDs7V0FBSSxTQUFTLEVBQUMsaUJBQWlCLEVBQUMsRUFBRSxFQUFDLFdBQVc7U0FDNUM7O2FBQUksS0FBSyxFQUFDLGNBQWM7V0FBQzs7ZUFBSyxTQUFTLEVBQUMsMkJBQTJCO2FBQUM7OztlQUFPLGlCQUFpQixFQUFFO2NBQVE7WUFBTTtVQUFLO1NBQ2hILEtBQUs7UUFDSDtNQUNELENBQ047SUFDSDtFQUNGLENBQUMsQ0FBQzs7QUFFSCxXQUFVLENBQUMsWUFBWSxHQUFHO0FBQ3hCLFNBQU0sRUFBRSxLQUFLLENBQUMsU0FBUyxDQUFDLE1BQU0sQ0FBQyxVQUFVO0VBQzFDOztBQUVELFVBQVMsaUJBQWlCLEdBQUU7MkJBQ0QsT0FBTyxDQUFDLFFBQVEsQ0FBQyxPQUFPLENBQUMsSUFBSSxDQUFDOztPQUFsRCxnQkFBZ0IscUJBQWhCLGdCQUFnQjs7QUFDckIsVUFBTyxnQkFBZ0IsQ0FBQztFQUN6Qjs7QUFFRCxPQUFNLENBQUMsT0FBTyxHQUFHLFVBQVUsQzs7Ozs7Ozs7Ozs7OztBQzNEM0IsS0FBSSxLQUFLLEdBQUcsbUJBQU8sQ0FBQyxDQUFPLENBQUMsQ0FBQztBQUM3QixLQUFJLENBQUMsR0FBRyxtQkFBTyxDQUFDLEVBQVEsQ0FBQyxDQUFDO0FBQzFCLEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsQ0FBYSxDQUFDLENBQUM7O2dCQUNaLG1CQUFPLENBQUMsR0FBb0IsQ0FBQzs7S0FBakQsT0FBTyxZQUFQLE9BQU87S0FBRSxPQUFPLFlBQVAsT0FBTzs7QUFDckIsS0FBSSxVQUFVLEdBQUcsbUJBQU8sQ0FBQyxHQUFrQixDQUFDLENBQUM7QUFDN0MsS0FBSSxnQkFBZ0IsR0FBRyxtQkFBTyxDQUFDLEVBQWlDLENBQUMsQ0FBQztBQUNsRSxLQUFJLGNBQWMsR0FBRyxtQkFBTyxDQUFDLEdBQWtCLENBQUMsQ0FBQzs7aUJBQzNCLG1CQUFPLENBQUMsR0FBYSxDQUFDOztLQUF2QyxhQUFhLGFBQWIsYUFBYTs7QUFFbEIsS0FBSSxlQUFlLEdBQUcsS0FBSyxDQUFDLFdBQVcsQ0FBQzs7O0FBRXRDLFNBQU0sRUFBRSxDQUFDLGdCQUFnQixDQUFDOztBQUUxQixvQkFBaUIsK0JBQUU7QUFDakIsTUFBQyxDQUFDLElBQUksQ0FBQyxJQUFJLENBQUMsSUFBSSxDQUFDLENBQUMsUUFBUSxDQUFDO0FBQ3pCLFlBQUssRUFBQztBQUNKLGlCQUFRLEVBQUM7QUFDUCxvQkFBUyxFQUFFLENBQUM7QUFDWixtQkFBUSxFQUFFLElBQUk7VUFDZjtBQUNELDBCQUFpQixFQUFDO0FBQ2hCLG1CQUFRLEVBQUUsSUFBSTtBQUNkLGtCQUFPLEVBQUUsSUFBSSxDQUFDLElBQUksQ0FBQyxRQUFRO1VBQzVCO1FBQ0Y7O0FBRUQsZUFBUSxFQUFFO0FBQ1gsMEJBQWlCLEVBQUU7QUFDbEIsb0JBQVMsRUFBRSxDQUFDLENBQUMsU0FBUyxDQUFDLE1BQU0sQ0FBQywrQkFBK0IsQ0FBQztBQUM5RCxrQkFBTyxFQUFFLGtDQUFrQztVQUMzQztRQUNDO01BQ0YsQ0FBQztJQUNIOztBQUVELGtCQUFlLDZCQUFHO0FBQ2hCLFlBQU87QUFDTCxXQUFJLEVBQUUsSUFBSSxDQUFDLEtBQUssQ0FBQyxNQUFNLENBQUMsSUFBSTtBQUM1QixVQUFHLEVBQUUsRUFBRTtBQUNQLG1CQUFZLEVBQUUsRUFBRTtBQUNoQixZQUFLLEVBQUUsRUFBRTtNQUNWO0lBQ0Y7O0FBRUQsVUFBTyxtQkFBQyxDQUFDLEVBQUU7QUFDVCxNQUFDLENBQUMsY0FBYyxFQUFFLENBQUM7QUFDbkIsU0FBSSxJQUFJLENBQUMsT0FBTyxFQUFFLEVBQUU7QUFDbEIsaUJBQVUsQ0FBQyxPQUFPLENBQUMsTUFBTSxDQUFDO0FBQ3hCLGFBQUksRUFBRSxJQUFJLENBQUMsS0FBSyxDQUFDLElBQUk7QUFDckIsWUFBRyxFQUFFLElBQUksQ0FBQyxLQUFLLENBQUMsR0FBRztBQUNuQixjQUFLLEVBQUUsSUFBSSxDQUFDLEtBQUssQ0FBQyxLQUFLO0FBQ3ZCLG9CQUFXLEVBQUUsSUFBSSxDQUFDLEtBQUssQ0FBQyxNQUFNLENBQUMsWUFBWSxFQUFDLENBQUMsQ0FBQztNQUNqRDtJQUNGOztBQUVELFVBQU8scUJBQUc7QUFDUixTQUFJLEtBQUssR0FBRyxDQUFDLENBQUMsSUFBSSxDQUFDLElBQUksQ0FBQyxJQUFJLENBQUMsQ0FBQztBQUM5QixZQUFPLEtBQUssQ0FBQyxNQUFNLEtBQUssQ0FBQyxJQUFJLEtBQUssQ0FBQyxLQUFLLEVBQUUsQ0FBQztJQUM1Qzs7QUFFRCxTQUFNLG9CQUFHO3lCQUNrQyxJQUFJLENBQUMsS0FBSyxDQUFDLE1BQU07U0FBckQsWUFBWSxpQkFBWixZQUFZO1NBQUUsUUFBUSxpQkFBUixRQUFRO1NBQUUsT0FBTyxpQkFBUCxPQUFPOztBQUNwQyxZQUNFOztTQUFNLEdBQUcsRUFBQyxNQUFNLEVBQUMsU0FBUyxFQUFDLHVCQUF1QjtPQUNoRDs7OztRQUFvQztPQUNwQzs7V0FBSyxTQUFTLEVBQUMsRUFBRTtTQUNmOzthQUFLLFNBQVMsRUFBQyxZQUFZO1dBQ3pCO0FBQ0Usc0JBQVMsRUFBRSxJQUFJLENBQUMsU0FBUyxDQUFDLE1BQU0sQ0FBRTtBQUNsQyxpQkFBSSxFQUFDLFVBQVU7QUFDZixzQkFBUyxFQUFDLHVCQUF1QjtBQUNqQyx3QkFBVyxFQUFDLFdBQVcsR0FBRTtVQUN2QjtTQUNOOzthQUFLLFNBQVMsRUFBQyxZQUFZO1dBQ3pCO0FBQ0Usc0JBQVM7QUFDVCxzQkFBUyxFQUFFLElBQUksQ0FBQyxTQUFTLENBQUMsS0FBSyxDQUFFO0FBQ2pDLGdCQUFHLEVBQUMsVUFBVTtBQUNkLGlCQUFJLEVBQUMsVUFBVTtBQUNmLGlCQUFJLEVBQUMsVUFBVTtBQUNmLHNCQUFTLEVBQUMsY0FBYztBQUN4Qix3QkFBVyxFQUFDLFVBQVUsR0FBRztVQUN2QjtTQUNOOzthQUFLLFNBQVMsRUFBQyxZQUFZO1dBQ3pCO0FBQ0Usc0JBQVMsRUFBRSxJQUFJLENBQUMsU0FBUyxDQUFDLGNBQWMsQ0FBRTtBQUMxQyxpQkFBSSxFQUFDLFVBQVU7QUFDZixpQkFBSSxFQUFDLG1CQUFtQjtBQUN4QixzQkFBUyxFQUFDLGNBQWM7QUFDeEIsd0JBQVcsRUFBQyxrQkFBa0IsR0FBRTtVQUM5QjtTQUNOOzthQUFLLFNBQVMsRUFBQyxZQUFZO1dBQ3pCO0FBQ0UsaUJBQUksRUFBQyxPQUFPO0FBQ1osc0JBQVMsRUFBRSxJQUFJLENBQUMsU0FBUyxDQUFDLE9BQU8sQ0FBRTtBQUNuQyxzQkFBUyxFQUFDLHVCQUF1QjtBQUNqQyx3QkFBVyxFQUFDLHlDQUF5QyxHQUFHO1VBQ3REO1NBQ047O2FBQVEsSUFBSSxFQUFDLFFBQVEsRUFBQyxRQUFRLEVBQUUsWUFBYSxFQUFDLFNBQVMsRUFBQyxzQ0FBc0MsRUFBQyxPQUFPLEVBQUUsSUFBSSxDQUFDLE9BQVE7O1VBQWtCO1NBQ3JJLFFBQVEsR0FBSTs7YUFBTyxTQUFTLEVBQUMsT0FBTztXQUFFLE9BQU87VUFBUyxHQUFJLElBQUk7UUFDNUQ7TUFDRCxDQUNQO0lBQ0g7RUFDRixDQUFDOztBQUVGLEtBQUksTUFBTSxHQUFHLEtBQUssQ0FBQyxXQUFXLENBQUM7OztBQUU3QixTQUFNLEVBQUUsQ0FBQyxPQUFPLENBQUMsVUFBVSxDQUFDOztBQUU1QixrQkFBZSw2QkFBRztBQUNoQixZQUFPO0FBQ0wsYUFBTSxFQUFFLE9BQU8sQ0FBQyxNQUFNO0FBQ3RCLGFBQU0sRUFBRSxPQUFPLENBQUMsTUFBTTtBQUN0QixxQkFBYyxFQUFFLE9BQU8sQ0FBQyxjQUFjO01BQ3ZDO0lBQ0Y7O0FBRUQsb0JBQWlCLCtCQUFFO0FBQ2pCLFlBQU8sQ0FBQyxXQUFXLENBQUMsSUFBSSxDQUFDLEtBQUssQ0FBQyxNQUFNLENBQUMsV0FBVyxDQUFDLENBQUM7SUFDcEQ7O0FBRUQsU0FBTSxFQUFFLGtCQUFXO2tCQUNzQixJQUFJLENBQUMsS0FBSztTQUE1QyxjQUFjLFVBQWQsY0FBYztTQUFFLE1BQU0sVUFBTixNQUFNO1NBQUUsTUFBTSxVQUFOLE1BQU07O0FBRW5DLFNBQUcsY0FBYyxDQUFDLFFBQVEsRUFBQztBQUN6QixjQUFPLG9CQUFDLGFBQWEsT0FBRTtNQUN4Qjs7QUFFRCxTQUFHLENBQUMsTUFBTSxFQUFFO0FBQ1YsY0FBTyxJQUFJLENBQUM7TUFDYjs7QUFFRCxZQUNFOztTQUFLLFNBQVMsRUFBQyx3QkFBd0I7T0FDckMsNkJBQUssU0FBUyxFQUFDLGVBQWUsR0FBTztPQUNyQzs7V0FBSyxTQUFTLEVBQUMsc0JBQXNCO1NBQ25DOzthQUFLLFNBQVMsRUFBQyxpQkFBaUI7V0FDOUIsb0JBQUMsZUFBZSxJQUFDLE1BQU0sRUFBRSxNQUFPLEVBQUMsTUFBTSxFQUFFLE1BQU0sQ0FBQyxJQUFJLEVBQUcsR0FBRTtXQUN6RCxvQkFBQyxjQUFjLE9BQUU7VUFDYjtTQUNOOzthQUFLLFNBQVMsRUFBQyxvQ0FBb0M7V0FDakQ7Ozs7YUFBaUMsK0JBQUs7O2FBQUM7Ozs7Y0FBMkQ7WUFBSztXQUN2Ryw2QkFBSyxTQUFTLEVBQUMsZUFBZSxFQUFDLEdBQUcsNkJBQTRCLE1BQU0sQ0FBQyxHQUFHLENBQUMsSUFBSSxDQUFLLEdBQUc7VUFDakY7UUFDRjtNQUNGLENBQ047SUFDSDtFQUNGLENBQUMsQ0FBQzs7QUFFSCxPQUFNLENBQUMsT0FBTyxHQUFHLE1BQU0sQzs7Ozs7Ozs7Ozs7OztBQ3ZKdkIsS0FBSSxLQUFLLEdBQUcsbUJBQU8sQ0FBQyxDQUFPLENBQUMsQ0FBQztBQUM3QixLQUFJLE9BQU8sR0FBRyxtQkFBTyxDQUFDLENBQWEsQ0FBQyxDQUFDO0FBQ3JDLEtBQUksV0FBVyxHQUFHLG1CQUFPLENBQUMsRUFBMEIsQ0FBQyxDQUFDO0FBQ3RELEtBQUksV0FBVyxHQUFHLG1CQUFPLENBQUMsRUFBMkIsQ0FBQyxDQUFDO0FBQ3ZELEtBQUksUUFBUSxHQUFHLG1CQUFPLENBQUMsR0FBZ0IsQ0FBQyxDQUFDOztBQUV6QyxLQUFJLEtBQUssR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFFNUIsU0FBTSxFQUFFLENBQUMsT0FBTyxDQUFDLFVBQVUsQ0FBQzs7QUFFNUIsa0JBQWUsNkJBQUc7QUFDaEIsWUFBTztBQUNMLGtCQUFXLEVBQUUsV0FBVyxDQUFDLFlBQVk7QUFDckMsV0FBSSxFQUFFLFdBQVcsQ0FBQyxJQUFJO01BQ3ZCO0lBQ0Y7O0FBRUQsU0FBTSxFQUFFLGtCQUFXO0FBQ2pCLFNBQUksV0FBVyxHQUFHLElBQUksQ0FBQyxLQUFLLENBQUMsV0FBVyxDQUFDO0FBQ3pDLFNBQUksTUFBTSxHQUFHLElBQUksQ0FBQyxLQUFLLENBQUMsSUFBSSxDQUFDLE1BQU0sQ0FBQztBQUNwQyxZQUFTLG9CQUFDLFFBQVEsSUFBQyxXQUFXLEVBQUUsV0FBWSxFQUFDLE1BQU0sRUFBRSxNQUFPLEdBQUUsQ0FBRztJQUNsRTtFQUNGLENBQUMsQ0FBQzs7QUFFSCxPQUFNLENBQUMsT0FBTyxHQUFHLEtBQUssQzs7Ozs7Ozs7Ozs7OztBQ3hCdEIsS0FBSSxLQUFLLEdBQUcsbUJBQU8sQ0FBQyxDQUFPLENBQUMsQ0FBQztBQUM3QixLQUFJLE9BQU8sR0FBRyxtQkFBTyxDQUFDLENBQWEsQ0FBQyxDQUFDOztnQkFDQSxtQkFBTyxDQUFDLEdBQXFCLENBQUM7O0tBQTlELGVBQWUsWUFBZixlQUFlO0tBQUUsV0FBVyxZQUFYLFdBQVc7O2lCQUNnQyxtQkFBTyxDQUFDLEVBQTBCLENBQUM7O0tBQS9GLEtBQUssYUFBTCxLQUFLO0tBQUUsTUFBTSxhQUFOLE1BQU07S0FBRSxJQUFJLGFBQUosSUFBSTtLQUFFLFFBQVEsYUFBUixRQUFRO0tBQUUsY0FBYyxhQUFkLGNBQWM7S0FBRSxTQUFTLGFBQVQsU0FBUzs7aUJBQ3FCLG1CQUFPLENBQUMsR0FBYSxDQUFDOztLQUFuRyxVQUFVLGFBQVYsVUFBVTtLQUFFLFNBQVMsYUFBVCxTQUFTO0tBQUUsU0FBUyxhQUFULFNBQVM7S0FBRSxRQUFRLGFBQVIsUUFBUTtLQUFFLFlBQVksYUFBWixZQUFZO0tBQUUsZUFBZSxhQUFmLGVBQWU7O0FBRTlFLEtBQUksaUJBQWlCLEdBQUcsS0FBSyxDQUFDLFdBQVcsQ0FBQzs7O0FBQ3hDLFNBQU0sRUFBRSxrQkFBVztBQUNqQixTQUFJLElBQUksR0FBRyxJQUFJLENBQUMsS0FBSyxDQUFDLElBQUksQ0FBQyxNQUFNLENBQUMsY0FBSTtjQUFJLElBQUksQ0FBQyxNQUFNO01BQUEsQ0FBQyxDQUFDO0FBQ3ZELFlBQ0U7O1NBQUssU0FBUyxFQUFDLHFCQUFxQjtPQUNsQzs7V0FBSyxTQUFTLEVBQUMsWUFBWTtTQUN6Qjs7OztVQUEwQjtRQUN0QjtPQUNOOztXQUFLLFNBQVMsRUFBQyxhQUFhO1NBQ3pCLElBQUksQ0FBQyxNQUFNLEtBQUssQ0FBQyxHQUFHLG9CQUFDLFNBQVMsSUFBQyxJQUFJLEVBQUMsOEJBQThCLEdBQUUsR0FDbkU7O2FBQUssU0FBUyxFQUFDLEVBQUU7V0FDZjtBQUFDLGtCQUFLO2VBQUMsUUFBUSxFQUFFLElBQUksQ0FBQyxNQUFPLEVBQUMsU0FBUyxFQUFDLGVBQWU7YUFDckQsb0JBQUMsTUFBTTtBQUNMLHdCQUFTLEVBQUMsS0FBSztBQUNmLHFCQUFNLEVBQUU7QUFBQyxxQkFBSTs7O2dCQUFzQjtBQUNuQyxtQkFBSSxFQUFFLG9CQUFDLFFBQVEsSUFBQyxJQUFJLEVBQUUsSUFBSyxHQUFJO2VBQy9CO2FBQ0Ysb0JBQUMsTUFBTTtBQUNMLHFCQUFNLEVBQUU7QUFBQyxxQkFBSTs7O2dCQUFXO0FBQ3hCLG1CQUFJLEVBQ0Ysb0JBQUMsVUFBVSxJQUFDLElBQUksRUFBRSxJQUFLLEdBQ3hCO2VBQ0Q7YUFDRixvQkFBQyxNQUFNO0FBQ0wscUJBQU0sRUFBRTtBQUFDLHFCQUFJOzs7Z0JBQWdCO0FBQzdCLG1CQUFJLEVBQUUsb0JBQUMsUUFBUSxJQUFDLElBQUksRUFBRSxJQUFLLEdBQUs7ZUFDaEM7YUFDRixvQkFBQyxNQUFNO0FBQ0wsd0JBQVMsRUFBQyxTQUFTO0FBQ25CLHFCQUFNLEVBQUU7QUFBQyxxQkFBSTs7O2dCQUFtQjtBQUNoQyxtQkFBSSxFQUFFLG9CQUFDLGVBQWUsSUFBQyxJQUFJLEVBQUUsSUFBSyxHQUFJO2VBQ3RDO2FBQ0Ysb0JBQUMsTUFBTTtBQUNMLHFCQUFNLEVBQUU7QUFBQyxxQkFBSTs7O2dCQUFpQjtBQUM5QixtQkFBSSxFQUFFLG9CQUFDLFNBQVMsSUFBQyxJQUFJLEVBQUUsSUFBSyxHQUFLO2VBQ2pDO1lBQ0k7VUFDSjtRQUVKO01BQ0YsQ0FDUDtJQUNGO0VBQ0YsQ0FBQyxDQUFDOztBQUVILE9BQU0sQ0FBQyxPQUFPLEdBQUcsaUJBQWlCLEM7Ozs7Ozs7Ozs7Ozs7QUNuRGxDLEtBQUksS0FBSyxHQUFHLG1CQUFPLENBQUMsQ0FBTyxDQUFDLENBQUM7QUFDN0IsS0FBSSxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxDQUFhLENBQUMsQ0FBQzs7Z0JBQ1osbUJBQU8sQ0FBQyxFQUFzQixDQUFDOztLQUFuRCxPQUFPLFlBQVAsT0FBTztLQUFFLE9BQU8sWUFBUCxPQUFPOztBQUNyQixLQUFJLGlCQUFpQixHQUFHLG1CQUFPLENBQUMsR0FBeUIsQ0FBQyxDQUFDO0FBQzNELEtBQUksaUJBQWlCLEdBQUcsbUJBQU8sQ0FBQyxHQUF5QixDQUFDLENBQUM7O0FBRTNELEtBQUksUUFBUSxHQUFHLEtBQUssQ0FBQyxXQUFXLENBQUM7OztBQUMvQixTQUFNLEVBQUUsQ0FBQyxPQUFPLENBQUMsVUFBVSxDQUFDOztBQUU1QixrQkFBZSw2QkFBRztBQUNoQixZQUFPLEVBQUMsSUFBSSxFQUFFLE9BQU8sQ0FBQyxZQUFZLEVBQUM7SUFDcEM7O0FBRUQsU0FBTSxFQUFFLGtCQUFXO1NBQ1osSUFBSSxHQUFJLElBQUksQ0FBQyxLQUFLLENBQWxCLElBQUk7O0FBQ1QsWUFDRTs7U0FBSyxTQUFTLEVBQUMsdUJBQXVCO09BQ3BDLG9CQUFDLGlCQUFpQixJQUFDLElBQUksRUFBRSxJQUFLLEdBQUU7T0FDaEMsNEJBQUksU0FBUyxFQUFDLGFBQWEsR0FBRTtPQUM3QixvQkFBQyxpQkFBaUIsSUFBQyxJQUFJLEVBQUUsSUFBSyxHQUFFO01BQzVCLENBQ047SUFDSDtFQUNGLENBQUMsQ0FBQzs7QUFFSCxPQUFNLENBQUMsT0FBTyxHQUFHLFFBQVEsQzs7Ozs7Ozs7Ozs7Ozs7O0FDekJ6QixLQUFJLEtBQUssR0FBRyxtQkFBTyxDQUFDLENBQU8sQ0FBQyxDQUFDO0FBQzdCLEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsQ0FBYSxDQUFDLENBQUM7O2dCQUNwQixtQkFBTyxDQUFDLEVBQXNCLENBQUM7O0tBQTFDLE9BQU8sWUFBUCxPQUFPOztBQUNiLEtBQUksZ0JBQWdCLEdBQUcsbUJBQU8sQ0FBQyxFQUFpQyxDQUFDLENBQUM7O2lCQUNELG1CQUFPLENBQUMsRUFBMEIsQ0FBQzs7S0FBL0YsS0FBSyxhQUFMLEtBQUs7S0FBRSxNQUFNLGFBQU4sTUFBTTtLQUFFLElBQUksYUFBSixJQUFJO0tBQUUsUUFBUSxhQUFSLFFBQVE7S0FBRSxjQUFjLGFBQWQsY0FBYztLQUFFLFNBQVMsYUFBVCxTQUFTOztpQkFDMkIsbUJBQU8sQ0FBQyxHQUFhLENBQUM7O0tBQXpHLFVBQVUsYUFBVixVQUFVO0tBQUUsY0FBYyxhQUFkLGNBQWM7S0FBRSxTQUFTLGFBQVQsU0FBUztLQUFFLFNBQVMsYUFBVCxTQUFTO0tBQUUsWUFBWSxhQUFaLFlBQVk7S0FBRSxlQUFlLGFBQWYsZUFBZTs7aUJBQy9DLG1CQUFPLENBQUMsR0FBcUIsQ0FBQzs7S0FBOUQsZUFBZSxhQUFmLGVBQWU7S0FBRSxXQUFXLGFBQVgsV0FBVzs7QUFDakMsS0FBSSxNQUFNLEdBQUksbUJBQU8sQ0FBQyxDQUFRLENBQUMsQ0FBQzs7aUJBQ2IsbUJBQU8sQ0FBQyxFQUFzQixDQUFDOztLQUE3QyxVQUFVLGFBQVYsVUFBVTs7aUJBQ0MsbUJBQU8sQ0FBQyxFQUF3QixDQUFDOztLQUE1QyxPQUFPLGFBQVAsT0FBTzs7QUFDWixLQUFJLENBQUMsR0FBRyxtQkFBTyxDQUFDLEVBQUcsQ0FBQyxDQUFDOztBQUVyQixLQUFJLGdCQUFnQixHQUFHLEtBQUssQ0FBQyxXQUFXLENBQUM7OztBQUV2QyxTQUFNLEVBQUUsQ0FBQyxnQkFBZ0IsQ0FBQzs7QUFFMUIsa0JBQWUsMkJBQUMsS0FBSyxFQUFDO3VCQUNPLFVBQVUsQ0FBQyxJQUFJLElBQUksRUFBRSxDQUFDOztTQUE1QyxTQUFTO1NBQUUsT0FBTzs7QUFDdkIsU0FBSSxDQUFDLGVBQWUsR0FBRyxDQUFDLFVBQVUsRUFBRSxTQUFTLEVBQUUsS0FBSyxFQUFFLE9BQU8sQ0FBQyxDQUFDO0FBQy9ELFlBQU8sRUFBRSxNQUFNLEVBQUUsRUFBRSxFQUFFLFdBQVcsRUFBRSxFQUFDLE9BQU8sRUFBRSxLQUFLLEVBQUMsRUFBRSxTQUFTLEVBQVQsU0FBUyxFQUFFLE9BQU8sRUFBUCxPQUFPLEVBQUUsQ0FBQztJQUMxRTs7QUFFRCxxQkFBa0IsZ0NBQUU7QUFDbEIsWUFBTyxDQUFDLGFBQWEsQ0FBQyxJQUFJLENBQUMsS0FBSyxDQUFDLFNBQVMsRUFBRSxJQUFJLENBQUMsS0FBSyxDQUFDLE9BQU8sQ0FBQyxDQUFDO0lBQ2pFOztBQUVELHFCQUFrQiw4QkFBQyxTQUFTLEVBQUUsT0FBTyxFQUFDO0FBQ3BDLFlBQU8sQ0FBQyxhQUFhLENBQUMsU0FBUyxFQUFFLE9BQU8sQ0FBQyxDQUFDO0FBQzFDLFNBQUksQ0FBQyxLQUFLLENBQUMsU0FBUyxHQUFHLFNBQVMsQ0FBQztBQUNqQyxTQUFJLENBQUMsS0FBSyxDQUFDLE9BQU8sR0FBRyxPQUFPLENBQUM7QUFDN0IsU0FBSSxDQUFDLFFBQVEsQ0FBQyxJQUFJLENBQUMsS0FBSyxDQUFDLENBQUM7SUFDM0I7O0FBRUQsZUFBWSx3QkFBQyxTQUFTLEVBQUUsT0FBTyxFQUFFOzs7QUFDL0IsU0FBSSxDQUFDLFFBQVEsY0FDUixJQUFJLENBQUMsS0FBSztBQUNiLGtCQUFXLG1DQUFLLFNBQVMsSUFBRyxPQUFPLGVBQUU7UUFDckMsQ0FBQztJQUNKOztBQUVELHNCQUFtQiwrQkFBQyxJQUFvQixFQUFDO1NBQXBCLFNBQVMsR0FBVixJQUFvQixDQUFuQixTQUFTO1NBQUUsT0FBTyxHQUFuQixJQUFvQixDQUFSLE9BQU87O0FBQ3JDLFNBQUksQ0FBQyxrQkFBa0IsQ0FBQyxTQUFTLEVBQUUsT0FBTyxDQUFDLENBQUM7SUFDN0M7O0FBRUQsc0JBQW1CLCtCQUFDLFFBQVEsRUFBQzt3QkFDQSxVQUFVLENBQUMsUUFBUSxDQUFDOztTQUExQyxTQUFTO1NBQUUsT0FBTzs7QUFDdkIsU0FBSSxDQUFDLGtCQUFrQixDQUFDLFNBQVMsRUFBRSxPQUFPLENBQUMsQ0FBQztJQUM3Qzs7QUFFRCxvQkFBaUIsNkJBQUMsV0FBVyxFQUFFLFdBQVcsRUFBRSxRQUFRLEVBQUM7QUFDbkQsU0FBRyxRQUFRLEtBQUssU0FBUyxFQUFDO0FBQ3hCLFdBQUksV0FBVyxHQUFHLE1BQU0sQ0FBQyxXQUFXLENBQUMsQ0FBQyxNQUFNLENBQUMsT0FBTyxDQUFDLENBQUMsaUJBQWlCLEVBQUUsQ0FBQztBQUMxRSxjQUFPLFdBQVcsQ0FBQyxPQUFPLENBQUMsV0FBVyxDQUFDLEtBQUssQ0FBQyxDQUFDLENBQUM7TUFDaEQ7SUFDRjs7QUFFRCxnQkFBYSx5QkFBQyxJQUFJLEVBQUM7OztBQUNqQixTQUFJLFFBQVEsR0FBRyxJQUFJLENBQUMsTUFBTSxDQUFDLGFBQUc7Y0FDNUIsT0FBTyxDQUFDLEdBQUcsRUFBRSxNQUFLLEtBQUssQ0FBQyxNQUFNLEVBQUU7QUFDOUIsd0JBQWUsRUFBRSxNQUFLLGVBQWU7QUFDckMsV0FBRSxFQUFFLE1BQUssaUJBQWlCO1FBQzNCLENBQUM7TUFBQSxDQUFDLENBQUM7O0FBRU4sU0FBSSxTQUFTLEdBQUcsTUFBTSxDQUFDLG1CQUFtQixDQUFDLElBQUksQ0FBQyxLQUFLLENBQUMsV0FBVyxDQUFDLENBQUMsQ0FBQyxDQUFDLENBQUM7QUFDdEUsU0FBSSxPQUFPLEdBQUcsSUFBSSxDQUFDLEtBQUssQ0FBQyxXQUFXLENBQUMsU0FBUyxDQUFDLENBQUM7QUFDaEQsU0FBSSxNQUFNLEdBQUcsQ0FBQyxDQUFDLE1BQU0sQ0FBQyxRQUFRLEVBQUUsU0FBUyxDQUFDLENBQUM7QUFDM0MsU0FBRyxPQUFPLEtBQUssU0FBUyxDQUFDLEdBQUcsRUFBQztBQUMzQixhQUFNLEdBQUcsTUFBTSxDQUFDLE9BQU8sRUFBRSxDQUFDO01BQzNCOztBQUVELFlBQU8sTUFBTSxDQUFDO0lBQ2Y7O0FBRUQsU0FBTSxFQUFFLGtCQUFXO2tCQUNVLElBQUksQ0FBQyxLQUFLO1NBQWhDLFNBQVMsVUFBVCxTQUFTO1NBQUUsT0FBTyxVQUFQLE9BQU87O0FBQ3ZCLFNBQUksSUFBSSxHQUFHLElBQUksQ0FBQyxLQUFLLENBQUMsSUFBSSxDQUFDLE1BQU0sQ0FBQyxjQUFJO2NBQUksQ0FBQyxJQUFJLENBQUMsTUFBTSxJQUFJLE1BQU0sQ0FBQyxJQUFJLENBQUMsT0FBTyxDQUFDLENBQUMsU0FBUyxDQUFDLFNBQVMsRUFBRSxPQUFPLENBQUM7TUFBQSxDQUFDLENBQUM7QUFDOUcsU0FBSSxHQUFHLElBQUksQ0FBQyxhQUFhLENBQUMsSUFBSSxDQUFDLENBQUM7O0FBRWhDLFlBQ0U7O1NBQUssU0FBUyxFQUFDLHFCQUFxQjtPQUNsQzs7V0FBSyxTQUFTLEVBQUMsWUFBWTtTQUN6Qjs7OztVQUE0QjtTQUM1Qjs7YUFBSyxTQUFTLEVBQUMsVUFBVTtXQUN2Qjs7ZUFBSyxTQUFTLEVBQUMsY0FBYzthQUMzQixvQkFBQyxlQUFlLElBQUMsU0FBUyxFQUFFLFNBQVUsRUFBQyxPQUFPLEVBQUUsT0FBUSxFQUFDLFFBQVEsRUFBRSxJQUFJLENBQUMsbUJBQW9CLEdBQUU7WUFDMUY7V0FDTjs7ZUFBSyxTQUFTLEVBQUMsY0FBYzthQUMzQixvQkFBQyxXQUFXLElBQUMsS0FBSyxFQUFFLFNBQVUsRUFBQyxhQUFhLEVBQUUsSUFBSSxDQUFDLG1CQUFvQixHQUFFO1lBQ3JFO1dBQ047O2VBQUssU0FBUyxFQUFDLGNBQWM7YUFDM0I7O2lCQUFLLFNBQVMsRUFBQyxZQUFZO2VBQ3pCLCtCQUFPLFNBQVMsRUFBRSxJQUFJLENBQUMsU0FBUyxDQUFDLFFBQVEsQ0FBRSxFQUFDLFdBQVcsRUFBQyxXQUFXLEVBQUMsU0FBUyxFQUFDLHVCQUF1QixHQUFFO2NBQ25HO1lBQ0Y7VUFDRjtRQUNGO09BQ047O1dBQUssU0FBUyxFQUFDLGFBQWE7U0FDMUI7O2FBQUssU0FBUyxFQUFDLEVBQUU7V0FDZjtBQUFDLGtCQUFLO2VBQUMsUUFBUSxFQUFFLElBQUksQ0FBQyxNQUFPLEVBQUMsU0FBUyxFQUFDLGVBQWU7YUFDckQsb0JBQUMsTUFBTTtBQUNMLHdCQUFTLEVBQUMsS0FBSztBQUNmLHFCQUFNLEVBQUU7QUFBQyxxQkFBSTs7O2dCQUFzQjtBQUNuQyxtQkFBSSxFQUFFLG9CQUFDLFFBQVEsSUFBQyxJQUFJLEVBQUUsSUFBSyxHQUFJO2VBQy9CO2FBQ0Ysb0JBQUMsTUFBTTtBQUNMLHFCQUFNLEVBQUU7QUFBQyxxQkFBSTs7O2dCQUFXO0FBQ3hCLG1CQUFJLEVBQ0Ysb0JBQUMsVUFBVSxJQUFDLElBQUksRUFBRSxJQUFLLEdBQ3hCO2VBQ0Q7YUFDRixvQkFBQyxNQUFNO0FBQ0wsd0JBQVMsRUFBQyxTQUFTO0FBQ25CLHFCQUFNLEVBQ0osb0JBQUMsY0FBYztBQUNiLHdCQUFPLEVBQUUsSUFBSSxDQUFDLEtBQUssQ0FBQyxXQUFXLENBQUMsT0FBUTtBQUN4Qyw2QkFBWSxFQUFFLElBQUksQ0FBQyxZQUFhO0FBQ2hDLHNCQUFLLEVBQUMsU0FBUztpQkFFbEI7QUFDRCxtQkFBSSxFQUFFLG9CQUFDLGVBQWUsSUFBQyxJQUFJLEVBQUUsSUFBSyxHQUFJO2VBQ3RDO2FBQ0Ysb0JBQUMsTUFBTTtBQUNMLHFCQUFNLEVBQUU7QUFBQyxxQkFBSTs7O2dCQUFnQjtBQUM3QixtQkFBSSxFQUFFLG9CQUFDLGNBQWMsSUFBQyxJQUFJLEVBQUUsSUFBSyxHQUFJO2VBQ3JDO1lBQ0k7VUFDSjtRQUNGO01BQ0YsQ0FDUDtJQUNGO0VBQ0YsQ0FBQyxDQUFDOztBQUVILE9BQU0sQ0FBQyxPQUFPLEdBQUcsZ0JBQWdCLEM7Ozs7Ozs7Ozs7Ozs7QUNySWpDLEtBQUksS0FBSyxHQUFHLG1CQUFPLENBQUMsQ0FBTyxDQUFDLENBQUM7QUFDN0IsS0FBSSxNQUFNLEdBQUcsbUJBQU8sQ0FBQyxHQUFXLENBQUMsQ0FBQyxNQUFNLENBQUM7O2dCQUNxQixtQkFBTyxDQUFDLEVBQWMsQ0FBQzs7S0FBL0UsTUFBTSxZQUFOLE1BQU07S0FBRSxLQUFLLFlBQUwsS0FBSztLQUFFLFFBQVEsWUFBUixRQUFRO0tBQUUsVUFBVSxZQUFWLFVBQVU7S0FBRSxjQUFjLFlBQWQsY0FBYzs7aUJBQ29CLG1CQUFPLENBQUMsR0FBYyxDQUFDOztLQUE5RixHQUFHLGFBQUgsR0FBRztLQUFFLEtBQUssYUFBTCxLQUFLO0tBQUUsS0FBSyxhQUFMLEtBQUs7S0FBRSxRQUFRLGFBQVIsUUFBUTtLQUFFLE9BQU8sYUFBUCxPQUFPO0tBQUUsa0JBQWtCLGFBQWxCLGtCQUFrQjtLQUFFLFFBQVEsYUFBUixRQUFROztpQkFDckQsbUJBQU8sQ0FBQyxHQUF3QixDQUFDOztLQUEvQyxVQUFVLGFBQVYsVUFBVTs7QUFDZixLQUFJLElBQUksR0FBRyxtQkFBTyxDQUFDLEVBQVEsQ0FBQyxDQUFDO0FBQzdCLEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsRUFBVyxDQUFDLENBQUM7QUFDbkMsS0FBSSxHQUFHLEdBQUcsbUJBQU8sQ0FBQyxFQUFVLENBQUMsQ0FBQzs7QUFFOUIsb0JBQU8sQ0FBQyxHQUFXLENBQUMsQ0FBQzs7O0FBR3JCLFFBQU8sQ0FBQyxJQUFJLEVBQUUsQ0FBQzs7QUFFZixVQUFTLFlBQVksQ0FBQyxTQUFTLEVBQUUsT0FBTyxFQUFFLEVBQUUsRUFBQztBQUMzQyxPQUFJLENBQUMsTUFBTSxFQUFFLENBQUM7RUFDZjs7QUFFRCxPQUFNLENBQ0o7QUFBQyxTQUFNO0tBQUMsT0FBTyxFQUFFLE9BQU8sQ0FBQyxVQUFVLEVBQUc7R0FDcEMsb0JBQUMsS0FBSyxJQUFDLElBQUksRUFBRSxHQUFHLENBQUMsTUFBTSxDQUFDLEtBQU0sRUFBQyxTQUFTLEVBQUUsS0FBTSxHQUFFO0dBQ2xELG9CQUFDLEtBQUssSUFBQyxJQUFJLEVBQUUsR0FBRyxDQUFDLE1BQU0sQ0FBQyxNQUFPLEVBQUMsT0FBTyxFQUFFLFlBQWEsR0FBRTtHQUN4RCxvQkFBQyxLQUFLLElBQUMsSUFBSSxFQUFFLEdBQUcsQ0FBQyxNQUFNLENBQUMsT0FBUSxFQUFDLFNBQVMsRUFBRSxPQUFRLEdBQUU7R0FDdEQsb0JBQUMsUUFBUSxJQUFDLElBQUksRUFBRSxHQUFHLENBQUMsTUFBTSxDQUFDLEdBQUksRUFBQyxFQUFFLEVBQUUsR0FBRyxDQUFDLE1BQU0sQ0FBQyxLQUFNLEdBQUU7R0FDdkQ7QUFBQyxVQUFLO09BQUMsSUFBSSxFQUFFLEdBQUcsQ0FBQyxNQUFNLENBQUMsR0FBSSxFQUFDLFNBQVMsRUFBRSxHQUFJLEVBQUMsT0FBTyxFQUFFLFVBQVc7S0FDL0Qsb0JBQUMsS0FBSyxJQUFDLElBQUksRUFBRSxHQUFHLENBQUMsTUFBTSxDQUFDLEtBQU0sRUFBQyxTQUFTLEVBQUUsS0FBTSxHQUFFO0tBQ2xELG9CQUFDLEtBQUssSUFBQyxJQUFJLEVBQUUsR0FBRyxDQUFDLE1BQU0sQ0FBQyxhQUFjLEVBQUMsVUFBVSxFQUFFLEVBQUMsa0JBQWtCLEVBQUUsa0JBQWtCLEVBQUUsR0FBRTtLQUM5RixvQkFBQyxLQUFLLElBQUMsSUFBSSxFQUFFLEdBQUcsQ0FBQyxNQUFNLENBQUMsUUFBUyxFQUFDLFNBQVMsRUFBRSxRQUFTLEdBQUU7SUFDbEQ7R0FDUixvQkFBQyxLQUFLLElBQUMsSUFBSSxFQUFDLEdBQUcsRUFBQyxTQUFTLEVBQUUsUUFBUyxHQUFHO0VBQ2hDLEVBQ1IsUUFBUSxDQUFDLGNBQWMsQ0FBQyxLQUFLLENBQUMsQ0FBQyxDOzs7Ozs7Ozs7QUMvQmxDLDJCIiwiZmlsZSI6ImFwcC5qcyIsInNvdXJjZXNDb250ZW50IjpbImltcG9ydCB7IFJlYWN0b3IgfSBmcm9tICdudWNsZWFyLWpzJ1xuXG5jb25zdCByZWFjdG9yID0gbmV3IFJlYWN0b3Ioe1xuICBkZWJ1ZzogdHJ1ZVxufSlcblxud2luZG93LnJlYWN0b3IgPSByZWFjdG9yO1xuXG5leHBvcnQgZGVmYXVsdCByZWFjdG9yXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvcmVhY3Rvci5qc1xuICoqLyIsImxldCB7Zm9ybWF0UGF0dGVybn0gPSByZXF1aXJlKCdhcHAvY29tbW9uL3BhdHRlcm5VdGlscycpO1xuXG5sZXQgY2ZnID0ge1xuXG4gIGJhc2VVcmw6IHdpbmRvdy5sb2NhdGlvbi5vcmlnaW4sXG5cbiAgaGVscFVybDogJ2h0dHBzOi8vZ2l0aHViLmNvbS9ncmF2aXRhdGlvbmFsL3RlbGVwb3J0L2Jsb2IvbWFzdGVyL1JFQURNRS5tZCcsXG5cbiAgYXBpOiB7XG4gICAgcmVuZXdUb2tlblBhdGg6Jy92MS93ZWJhcGkvc2Vzc2lvbnMvcmVuZXcnLFxuICAgIG5vZGVzUGF0aDogJy92MS93ZWJhcGkvc2l0ZXMvLWN1cnJlbnQtL25vZGVzJyxcbiAgICBzZXNzaW9uUGF0aDogJy92MS93ZWJhcGkvc2Vzc2lvbnMnLFxuICAgIHNpdGVTZXNzaW9uUGF0aDogJy92MS93ZWJhcGkvc2l0ZXMvLWN1cnJlbnQtL3Nlc3Npb25zLzpzaWQnLFxuICAgIGludml0ZVBhdGg6ICcvdjEvd2ViYXBpL3VzZXJzL2ludml0ZXMvOmludml0ZVRva2VuJyxcbiAgICBjcmVhdGVVc2VyUGF0aDogJy92MS93ZWJhcGkvdXNlcnMnLFxuICAgIHNlc3Npb25DaHVuazogJy92MS93ZWJhcGkvc2l0ZXMvLWN1cnJlbnQtL3Nlc3Npb25zLzpzaWQvY2h1bmtzP3N0YXJ0PTpzdGFydCZlbmQ9OmVuZCcsXG4gICAgc2Vzc2lvbkNodW5rQ291bnRQYXRoOiAnL3YxL3dlYmFwaS9zaXRlcy8tY3VycmVudC0vc2Vzc2lvbnMvOnNpZC9jaHVua3Njb3VudCcsXG5cbiAgICBnZXRGZXRjaFNlc3Npb25DaHVua1VybDogKHtzaWQsIHN0YXJ0LCBlbmR9KT0+e1xuICAgICAgcmV0dXJuIGZvcm1hdFBhdHRlcm4oY2ZnLmFwaS5zZXNzaW9uQ2h1bmssIHtzaWQsIHN0YXJ0LCBlbmR9KTtcbiAgICB9LFxuXG4gICAgZ2V0RmV0Y2hTZXNzaW9uTGVuZ3RoVXJsOiAoc2lkKT0+e1xuICAgICAgcmV0dXJuIGZvcm1hdFBhdHRlcm4oY2ZnLmFwaS5zZXNzaW9uQ2h1bmtDb3VudFBhdGgsIHtzaWR9KTtcbiAgICB9LFxuXG4gICAgZ2V0RmV0Y2hTZXNzaW9uc1VybDogKHN0YXJ0LCBlbmQpPT57XG4gICAgICB2YXIgcGFyYW1zID0ge1xuICAgICAgICBzdGFydDogc3RhcnQudG9JU09TdHJpbmcoKSxcbiAgICAgICAgZW5kOiBlbmQudG9JU09TdHJpbmcoKSAgICAgICAgXG4gICAgICB9O1xuXG4gICAgICB2YXIganNvbiA9IEpTT04uc3RyaW5naWZ5KHBhcmFtcyk7XG4gICAgICB2YXIganNvbkVuY29kZWQgPSB3aW5kb3cuZW5jb2RlVVJJKGpzb24pO1xuXG4gICAgICByZXR1cm4gYC92MS93ZWJhcGkvc2l0ZXMvLWN1cnJlbnQtL2V2ZW50cy9zZXNzaW9ucz9maWx0ZXI9JHtqc29uRW5jb2RlZH1gO1xuICAgIH0sXG5cbiAgICBnZXRGZXRjaFNlc3Npb25Vcmw6IChzaWQpPT57XG4gICAgICByZXR1cm4gZm9ybWF0UGF0dGVybihjZmcuYXBpLnNpdGVTZXNzaW9uUGF0aCwge3NpZH0pO1xuICAgIH0sXG5cbiAgICBnZXRUZXJtaW5hbFNlc3Npb25Vcmw6IChzaWQpPT4ge1xuICAgICAgcmV0dXJuIGZvcm1hdFBhdHRlcm4oY2ZnLmFwaS5zaXRlU2Vzc2lvblBhdGgsIHtzaWR9KTtcbiAgICB9LFxuXG4gICAgZ2V0SW52aXRlVXJsOiAoaW52aXRlVG9rZW4pID0+IHtcbiAgICAgIHJldHVybiBmb3JtYXRQYXR0ZXJuKGNmZy5hcGkuaW52aXRlUGF0aCwge2ludml0ZVRva2VufSk7XG4gICAgfSxcblxuICAgIGdldEV2ZW50U3RyZWFtQ29ublN0cjogKHRva2VuLCBzaWQpID0+IHtcbiAgICAgIHZhciBob3N0bmFtZSA9IGdldFdzSG9zdE5hbWUoKTtcbiAgICAgIHJldHVybiBgJHtob3N0bmFtZX0vdjEvd2ViYXBpL3NpdGVzLy1jdXJyZW50LS9zZXNzaW9ucy8ke3NpZH0vZXZlbnRzL3N0cmVhbT9hY2Nlc3NfdG9rZW49JHt0b2tlbn1gO1xuICAgIH0sXG5cbiAgICBnZXRUdHlDb25uU3RyOiAoe3Rva2VuLCBzZXJ2ZXJJZCwgbG9naW4sIHNpZCwgcm93cywgY29sc30pID0+IHtcbiAgICAgIHZhciBwYXJhbXMgPSB7XG4gICAgICAgIHNlcnZlcl9pZDogc2VydmVySWQsXG4gICAgICAgIGxvZ2luLFxuICAgICAgICBzaWQsXG4gICAgICAgIHRlcm06IHtcbiAgICAgICAgICBoOiByb3dzLFxuICAgICAgICAgIHc6IGNvbHNcbiAgICAgICAgfVxuICAgICAgfVxuXG4gICAgICB2YXIganNvbiA9IEpTT04uc3RyaW5naWZ5KHBhcmFtcyk7XG4gICAgICB2YXIganNvbkVuY29kZWQgPSB3aW5kb3cuZW5jb2RlVVJJKGpzb24pO1xuICAgICAgdmFyIGhvc3RuYW1lID0gZ2V0V3NIb3N0TmFtZSgpO1xuICAgICAgcmV0dXJuIGAke2hvc3RuYW1lfS92MS93ZWJhcGkvc2l0ZXMvLWN1cnJlbnQtL2Nvbm5lY3Q/YWNjZXNzX3Rva2VuPSR7dG9rZW59JnBhcmFtcz0ke2pzb25FbmNvZGVkfWA7XG4gICAgfVxuICB9LFxuXG4gIHJvdXRlczoge1xuICAgIGFwcDogJy93ZWInLFxuICAgIGxvZ291dDogJy93ZWIvbG9nb3V0JyxcbiAgICBsb2dpbjogJy93ZWIvbG9naW4nLFxuICAgIG5vZGVzOiAnL3dlYi9ub2RlcycsXG4gICAgYWN0aXZlU2Vzc2lvbjogJy93ZWIvc2Vzc2lvbnMvOnNpZCcsXG4gICAgbmV3VXNlcjogJy93ZWIvbmV3dXNlci86aW52aXRlVG9rZW4nLFxuICAgIHNlc3Npb25zOiAnL3dlYi9zZXNzaW9ucycsXG4gICAgcGFnZU5vdEZvdW5kOiAnL3dlYi9ub3Rmb3VuZCdcbiAgfSxcblxuICBnZXRBY3RpdmVTZXNzaW9uUm91dGVVcmwoc2lkKXtcbiAgICByZXR1cm4gZm9ybWF0UGF0dGVybihjZmcucm91dGVzLmFjdGl2ZVNlc3Npb24sIHtzaWR9KTtcbiAgfVxufVxuXG5leHBvcnQgZGVmYXVsdCBjZmc7XG5cbmZ1bmN0aW9uIGdldFdzSG9zdE5hbWUoKXtcbiAgdmFyIHByZWZpeCA9IGxvY2F0aW9uLnByb3RvY29sID09IFwiaHR0cHM6XCI/XCJ3c3M6Ly9cIjpcIndzOi8vXCI7XG4gIHZhciBob3N0cG9ydCA9IGxvY2F0aW9uLmhvc3RuYW1lKyhsb2NhdGlvbi5wb3J0ID8gJzonK2xvY2F0aW9uLnBvcnQ6ICcnKTtcbiAgcmV0dXJuIGAke3ByZWZpeH0ke2hvc3Rwb3J0fWA7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29uZmlnLmpzXG4gKiovIiwiLyoqXG4gKiBDb3B5cmlnaHQgMjAxMy0yMDE0IEZhY2Vib29rLCBJbmMuXG4gKlxuICogTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbiAqIHlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbiAqIFlvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuICpcbiAqIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuICpcbiAqIFVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbiAqIGRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbiAqIFdJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuICogU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxuICogbGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4gKlxuICovXG5cblwidXNlIHN0cmljdFwiO1xuXG4vKipcbiAqIENvbnN0cnVjdHMgYW4gZW51bWVyYXRpb24gd2l0aCBrZXlzIGVxdWFsIHRvIHRoZWlyIHZhbHVlLlxuICpcbiAqIEZvciBleGFtcGxlOlxuICpcbiAqICAgdmFyIENPTE9SUyA9IGtleU1pcnJvcih7Ymx1ZTogbnVsbCwgcmVkOiBudWxsfSk7XG4gKiAgIHZhciBteUNvbG9yID0gQ09MT1JTLmJsdWU7XG4gKiAgIHZhciBpc0NvbG9yVmFsaWQgPSAhIUNPTE9SU1tteUNvbG9yXTtcbiAqXG4gKiBUaGUgbGFzdCBsaW5lIGNvdWxkIG5vdCBiZSBwZXJmb3JtZWQgaWYgdGhlIHZhbHVlcyBvZiB0aGUgZ2VuZXJhdGVkIGVudW0gd2VyZVxuICogbm90IGVxdWFsIHRvIHRoZWlyIGtleXMuXG4gKlxuICogICBJbnB1dDogIHtrZXkxOiB2YWwxLCBrZXkyOiB2YWwyfVxuICogICBPdXRwdXQ6IHtrZXkxOiBrZXkxLCBrZXkyOiBrZXkyfVxuICpcbiAqIEBwYXJhbSB7b2JqZWN0fSBvYmpcbiAqIEByZXR1cm4ge29iamVjdH1cbiAqL1xudmFyIGtleU1pcnJvciA9IGZ1bmN0aW9uKG9iaikge1xuICB2YXIgcmV0ID0ge307XG4gIHZhciBrZXk7XG4gIGlmICghKG9iaiBpbnN0YW5jZW9mIE9iamVjdCAmJiAhQXJyYXkuaXNBcnJheShvYmopKSkge1xuICAgIHRocm93IG5ldyBFcnJvcigna2V5TWlycm9yKC4uLik6IEFyZ3VtZW50IG11c3QgYmUgYW4gb2JqZWN0LicpO1xuICB9XG4gIGZvciAoa2V5IGluIG9iaikge1xuICAgIGlmICghb2JqLmhhc093blByb3BlcnR5KGtleSkpIHtcbiAgICAgIGNvbnRpbnVlO1xuICAgIH1cbiAgICByZXRba2V5XSA9IGtleTtcbiAgfVxuICByZXR1cm4gcmV0O1xufTtcblxubW9kdWxlLmV4cG9ydHMgPSBrZXlNaXJyb3I7XG5cblxuXG4vKioqKioqKioqKioqKioqKipcbiAqKiBXRUJQQUNLIEZPT1RFUlxuICoqIC4vfi9rZXltaXJyb3IvaW5kZXguanNcbiAqKiBtb2R1bGUgaWQgPSAyMFxuICoqIG1vZHVsZSBjaHVua3MgPSAxXG4gKiovIiwidmFyIHsgYnJvd3Nlckhpc3RvcnksIGNyZWF0ZU1lbW9yeUhpc3RvcnkgfSA9IHJlcXVpcmUoJ3JlYWN0LXJvdXRlcicpO1xuXG5jb25zdCBBVVRIX0tFWV9EQVRBID0gJ2F1dGhEYXRhJztcblxudmFyIF9oaXN0b3J5ID0gY3JlYXRlTWVtb3J5SGlzdG9yeSgpO1xuXG52YXIgc2Vzc2lvbiA9IHtcblxuICBpbml0KGhpc3Rvcnk9YnJvd3Nlckhpc3Rvcnkpe1xuICAgIF9oaXN0b3J5ID0gaGlzdG9yeTtcbiAgfSxcblxuICBnZXRIaXN0b3J5KCl7XG4gICAgcmV0dXJuIF9oaXN0b3J5O1xuICB9LFxuXG4gIHNldFVzZXJEYXRhKHVzZXJEYXRhKXtcbiAgICBsb2NhbFN0b3JhZ2Uuc2V0SXRlbShBVVRIX0tFWV9EQVRBLCBKU09OLnN0cmluZ2lmeSh1c2VyRGF0YSkpO1xuICB9LFxuXG4gIGdldFVzZXJEYXRhKCl7XG4gICAgdmFyIGl0ZW0gPSBsb2NhbFN0b3JhZ2UuZ2V0SXRlbShBVVRIX0tFWV9EQVRBKTtcbiAgICBpZihpdGVtKXtcbiAgICAgIHJldHVybiBKU09OLnBhcnNlKGl0ZW0pO1xuICAgIH1cblxuICAgIHJldHVybiB7fTtcbiAgfSxcblxuICBjbGVhcigpe1xuICAgIGxvY2FsU3RvcmFnZS5jbGVhcigpXG4gIH1cblxufVxuXG5tb2R1bGUuZXhwb3J0cyA9IHNlc3Npb247XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvc2Vzc2lvbi5qc1xuICoqLyIsInZhciAkID0gcmVxdWlyZShcImpRdWVyeVwiKTtcbnZhciBzZXNzaW9uID0gcmVxdWlyZSgnYXBwL3Nlc3Npb24nKTtcblxuY29uc3QgYXBpID0ge1xuXG4gIHB1dChwYXRoLCBkYXRhLCB3aXRoVG9rZW4pe1xuICAgIHJldHVybiBhcGkuYWpheCh7dXJsOiBwYXRoLCBkYXRhOiBKU09OLnN0cmluZ2lmeShkYXRhKSwgdHlwZTogJ1BVVCd9LCB3aXRoVG9rZW4pO1xuICB9LFxuXG4gIHBvc3QocGF0aCwgZGF0YSwgd2l0aFRva2VuKXtcbiAgICByZXR1cm4gYXBpLmFqYXgoe3VybDogcGF0aCwgZGF0YTogSlNPTi5zdHJpbmdpZnkoZGF0YSksIHR5cGU6ICdQT1NUJ30sIHdpdGhUb2tlbik7XG4gIH0sXG5cbiAgZ2V0KHBhdGgpe1xuICAgIHJldHVybiBhcGkuYWpheCh7dXJsOiBwYXRofSk7XG4gIH0sXG5cbiAgYWpheChjZmcsIHdpdGhUb2tlbiA9IHRydWUpe1xuICAgIHZhciBkZWZhdWx0Q2ZnID0ge1xuICAgICAgdHlwZTogXCJHRVRcIixcbiAgICAgIGRhdGFUeXBlOiBcImpzb25cIixcbiAgICAgIGJlZm9yZVNlbmQ6IGZ1bmN0aW9uKHhocikge1xuICAgICAgICBpZih3aXRoVG9rZW4pe1xuICAgICAgICAgIHZhciB7IHRva2VuIH0gPSBzZXNzaW9uLmdldFVzZXJEYXRhKCk7XG4gICAgICAgICAgeGhyLnNldFJlcXVlc3RIZWFkZXIoJ0F1dGhvcml6YXRpb24nLCdCZWFyZXIgJyArIHRva2VuKTtcbiAgICAgICAgfVxuICAgICAgIH1cbiAgICB9XG5cbiAgICByZXR1cm4gJC5hamF4KCQuZXh0ZW5kKHt9LCBkZWZhdWx0Q2ZnLCBjZmcpKTtcbiAgfVxufVxuXG5tb2R1bGUuZXhwb3J0cyA9IGFwaTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9zZXJ2aWNlcy9hcGkuanNcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cyA9IGpRdWVyeTtcblxuXG4vKioqKioqKioqKioqKioqKipcbiAqKiBXRUJQQUNLIEZPT1RFUlxuICoqIGV4dGVybmFsIFwialF1ZXJ5XCJcbiAqKiBtb2R1bGUgaWQgPSAzOVxuICoqIG1vZHVsZSBjaHVua3MgPSAxXG4gKiovIiwibW9kdWxlLmV4cG9ydHMuZ2V0dGVycyA9IHJlcXVpcmUoJy4vZ2V0dGVycycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aW9ucyA9IHJlcXVpcmUoJy4vYWN0aW9ucycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aXZlVGVybVN0b3JlID0gcmVxdWlyZSgnLi9hY3RpdmVUZXJtU3RvcmUnKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2luZGV4LmpzXG4gKiovIiwiY29uc3Qgbm9kZUhvc3ROYW1lQnlTZXJ2ZXJJZCA9IChzZXJ2ZXJJZCkgPT4gWyBbJ3RscHRfbm9kZXMnXSwgKG5vZGVzKSA9PntcbiAgbGV0IHNlcnZlciA9IG5vZGVzLmZpbmQoaXRlbT0+IGl0ZW0uZ2V0KCdpZCcpID09PSBzZXJ2ZXJJZCk7ICBcbiAgcmV0dXJuICFzZXJ2ZXIgPyAnJyA6IHNlcnZlci5nZXQoJ2hvc3RuYW1lJyk7XG59XTtcblxuY29uc3Qgbm9kZUxpc3RWaWV3ID0gWyBbJ3RscHRfbm9kZXMnXSwgKG5vZGVzKSA9PntcbiAgICByZXR1cm4gbm9kZXMubWFwKChpdGVtKT0+e1xuICAgICAgdmFyIHNlcnZlcklkID0gaXRlbS5nZXQoJ2lkJyk7XG4gICAgICByZXR1cm4ge1xuICAgICAgICBpZDogc2VydmVySWQsXG4gICAgICAgIGhvc3RuYW1lOiBpdGVtLmdldCgnaG9zdG5hbWUnKSxcbiAgICAgICAgdGFnczogZ2V0VGFncyhpdGVtKSxcbiAgICAgICAgYWRkcjogaXRlbS5nZXQoJ2FkZHInKVxuICAgICAgfVxuICAgIH0pLnRvSlMoKTtcbiB9XG5dO1xuXG5mdW5jdGlvbiBnZXRUYWdzKG5vZGUpe1xuICB2YXIgYWxsTGFiZWxzID0gW107XG4gIHZhciBsYWJlbHMgPSBub2RlLmdldCgnbGFiZWxzJyk7XG5cbiAgaWYobGFiZWxzKXtcbiAgICBsYWJlbHMuZW50cnlTZXEoKS50b0FycmF5KCkuZm9yRWFjaChpdGVtPT57XG4gICAgICBhbGxMYWJlbHMucHVzaCh7XG4gICAgICAgIHJvbGU6IGl0ZW1bMF0sXG4gICAgICAgIHZhbHVlOiBpdGVtWzFdXG4gICAgICB9KTtcbiAgICB9KTtcbiAgfVxuXG4gIGxhYmVscyA9IG5vZGUuZ2V0KCdjbWRfbGFiZWxzJyk7XG5cbiAgaWYobGFiZWxzKXtcbiAgICBsYWJlbHMuZW50cnlTZXEoKS50b0FycmF5KCkuZm9yRWFjaChpdGVtPT57XG4gICAgICBhbGxMYWJlbHMucHVzaCh7XG4gICAgICAgIHJvbGU6IGl0ZW1bMF0sXG4gICAgICAgIHZhbHVlOiBpdGVtWzFdLmdldCgncmVzdWx0JyksXG4gICAgICAgIHRvb2x0aXA6IGl0ZW1bMV0uZ2V0KCdjb21tYW5kJylcbiAgICAgIH0pO1xuICAgIH0pO1xuICB9XG5cbiAgcmV0dXJuIGFsbExhYmVscztcbn1cblxuXG5leHBvcnQgZGVmYXVsdCB7XG4gIG5vZGVMaXN0VmlldyxcbiAgbm9kZUhvc3ROYW1lQnlTZXJ2ZXJJZFxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvZ2V0dGVycy5qc1xuICoqLyIsImltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUUllJTkdfVE9fU0lHTl9VUDogbnVsbCxcbiAgVFJZSU5HX1RPX0xPR0lOOiBudWxsLFxuICBGRVRDSElOR19JTlZJVEU6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2NvbnN0YW50cy5qc1xuICoqLyIsIm1vZHVsZS5leHBvcnRzLmdldHRlcnMgPSByZXF1aXJlKCcuL2dldHRlcnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGlvbnMgPSByZXF1aXJlKCcuL2FjdGlvbnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGl2ZVRlcm1TdG9yZSA9IHJlcXVpcmUoJy4vc2Vzc2lvblN0b3JlJyk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9zZXNzaW9ucy9pbmRleC5qc1xuICoqLyIsInZhciB7VFJZSU5HX1RPX0xPR0lOfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3Jlc3RBcGkvY29uc3RhbnRzJyk7XG52YXIge3JlcXVlc3RTdGF0dXN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvcmVzdEFwaS9nZXR0ZXJzJyk7XG5cbmNvbnN0IHVzZXIgPSBbIFsndGxwdF91c2VyJ10sIChjdXJyZW50VXNlcikgPT4ge1xuICAgIGlmKCFjdXJyZW50VXNlcil7XG4gICAgICByZXR1cm4gbnVsbDtcbiAgICB9XG5cbiAgICB2YXIgbmFtZSA9IGN1cnJlbnRVc2VyLmdldCgnbmFtZScpIHx8ICcnO1xuICAgIHZhciBzaG9ydERpc3BsYXlOYW1lID0gbmFtZVswXSB8fCAnJztcblxuICAgIHJldHVybiB7XG4gICAgICBuYW1lLFxuICAgICAgc2hvcnREaXNwbGF5TmFtZSxcbiAgICAgIGxvZ2luczogY3VycmVudFVzZXIuZ2V0KCdhbGxvd2VkX2xvZ2lucycpLnRvSlMoKVxuICAgIH1cbiAgfVxuXTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICB1c2VyLFxuICBsb2dpbkF0dGVtcDogcmVxdWVzdFN0YXR1cyhUUllJTkdfVE9fTE9HSU4pXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy91c2VyL2dldHRlcnMuanNcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xuXG5jb25zdCBHcnZUYWJsZVRleHRDZWxsID0gKHtyb3dJbmRleCwgZGF0YSwgY29sdW1uS2V5LCAuLi5wcm9wc30pID0+IChcbiAgPEdydlRhYmxlQ2VsbCB7Li4ucHJvcHN9PlxuICAgIHtkYXRhW3Jvd0luZGV4XVtjb2x1bW5LZXldfVxuICA8L0dydlRhYmxlQ2VsbD5cbik7XG5cbnZhciBHcnZTb3J0SGVhZGVyQ2VsbCA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHRoaXMuX29uU29ydENoYW5nZSA9IHRoaXMuX29uU29ydENoYW5nZS5iaW5kKHRoaXMpO1xuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICB2YXIge3NvcnREaXIsIGNoaWxkcmVuLCAuLi5wcm9wc30gPSB0aGlzLnByb3BzO1xuICAgIHJldHVybiAoXG4gICAgICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgICAgICA8YSBvbkNsaWNrPXt0aGlzLl9vblNvcnRDaGFuZ2V9PlxuICAgICAgICAgIHtjaGlsZHJlbn0ge3NvcnREaXIgPyAoc29ydERpciA9PT0gU29ydFR5cGVzLkRFU0MgPyAn4oaTJyA6ICfihpEnKSA6ICcnfVxuICAgICAgICA8L2E+XG4gICAgICA8L0NlbGw+XG4gICAgKTtcbiAgfSxcblxuICBfb25Tb3J0Q2hhbmdlKGUpIHtcbiAgICBlLnByZXZlbnREZWZhdWx0KCk7XG5cbiAgICBpZiAodGhpcy5wcm9wcy5vblNvcnRDaGFuZ2UpIHtcbiAgICAgIHRoaXMucHJvcHMub25Tb3J0Q2hhbmdlKFxuICAgICAgICB0aGlzLnByb3BzLmNvbHVtbktleSxcbiAgICAgICAgdGhpcy5wcm9wcy5zb3J0RGlyID9cbiAgICAgICAgICByZXZlcnNlU29ydERpcmVjdGlvbih0aGlzLnByb3BzLnNvcnREaXIpIDpcbiAgICAgICAgICBTb3J0VHlwZXMuREVTQ1xuICAgICAgKTtcbiAgICB9XG4gIH1cbn0pO1xuXG4vKipcbiogU29ydCBpbmRpY2F0b3IgdXNlZCBieSBTb3J0SGVhZGVyQ2VsbFxuKi9cbmNvbnN0IFNvcnRUeXBlcyA9IHtcbiAgQVNDOiAnQVNDJyxcbiAgREVTQzogJ0RFU0MnXG59O1xuXG5jb25zdCBTb3J0SW5kaWNhdG9yID0gKHtzb3J0RGlyfSk9PntcbiAgbGV0IGNscyA9ICdncnYtdGFibGUtaW5kaWNhdG9yLXNvcnQgZmEgZmEtc29ydCdcbiAgaWYoc29ydERpciA9PT0gU29ydFR5cGVzLkRFU0Mpe1xuICAgIGNscyArPSAnLWRlc2MnXG4gIH1cblxuICBpZiggc29ydERpciA9PT0gU29ydFR5cGVzLkFTQyl7XG4gICAgY2xzICs9ICctYXNjJ1xuICB9XG5cbiAgcmV0dXJuICg8aSBjbGFzc05hbWU9e2Nsc30+PC9pPik7XG59O1xuXG4vKipcbiogU29ydCBIZWFkZXIgQ2VsbFxuKi9cbnZhciBTb3J0SGVhZGVyQ2VsbCA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgcmVuZGVyKCkge1xuICAgIHZhciB7c29ydERpciwgY29sdW1uS2V5LCB0aXRsZSwgLi4ucHJvcHN9ID0gdGhpcy5wcm9wcztcblxuICAgIHJldHVybiAoXG4gICAgICA8R3J2VGFibGVDZWxsIHsuLi5wcm9wc30+XG4gICAgICAgIDxhIG9uQ2xpY2s9e3RoaXMub25Tb3J0Q2hhbmdlfT5cbiAgICAgICAgICB7dGl0bGV9XG4gICAgICAgIDwvYT5cbiAgICAgICAgPFNvcnRJbmRpY2F0b3Igc29ydERpcj17c29ydERpcn0vPlxuICAgICAgPC9HcnZUYWJsZUNlbGw+XG4gICAgKTtcbiAgfSxcblxuICBvblNvcnRDaGFuZ2UoZSkge1xuICAgIGUucHJldmVudERlZmF1bHQoKTtcbiAgICBpZih0aGlzLnByb3BzLm9uU29ydENoYW5nZSkge1xuICAgICAgLy8gZGVmYXVsdFxuICAgICAgbGV0IG5ld0RpciA9IFNvcnRUeXBlcy5ERVNDO1xuICAgICAgaWYodGhpcy5wcm9wcy5zb3J0RGlyKXtcbiAgICAgICAgbmV3RGlyID0gdGhpcy5wcm9wcy5zb3J0RGlyID09PSBTb3J0VHlwZXMuREVTQyA/IFNvcnRUeXBlcy5BU0MgOiBTb3J0VHlwZXMuREVTQztcbiAgICAgIH1cbiAgICAgIHRoaXMucHJvcHMub25Tb3J0Q2hhbmdlKHRoaXMucHJvcHMuY29sdW1uS2V5LCBuZXdEaXIpO1xuICAgIH1cbiAgfVxufSk7XG5cbi8qKlxuKiBEZWZhdWx0IENlbGxcbiovXG52YXIgR3J2VGFibGVDZWxsID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICByZW5kZXIoKXtcbiAgICB2YXIgcHJvcHMgPSB0aGlzLnByb3BzO1xuICAgIHJldHVybiBwcm9wcy5pc0hlYWRlciA/IDx0aCBrZXk9e3Byb3BzLmtleX0gY2xhc3NOYW1lPVwiZ3J2LXRhYmxlLWNlbGxcIj57cHJvcHMuY2hpbGRyZW59PC90aD4gOiA8dGQga2V5PXtwcm9wcy5rZXl9Pntwcm9wcy5jaGlsZHJlbn08L3RkPjtcbiAgfVxufSk7XG5cbi8qKlxuKiBUYWJsZVxuKi9cbnZhciBHcnZUYWJsZSA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICByZW5kZXJIZWFkZXIoY2hpbGRyZW4pe1xuICAgIHZhciBjZWxscyA9IGNoaWxkcmVuLm1hcCgoaXRlbSwgaW5kZXgpPT57XG4gICAgICByZXR1cm4gdGhpcy5yZW5kZXJDZWxsKGl0ZW0ucHJvcHMuaGVhZGVyLCB7aW5kZXgsIGtleTogaW5kZXgsIGlzSGVhZGVyOiB0cnVlLCAuLi5pdGVtLnByb3BzfSk7XG4gICAgfSlcblxuICAgIHJldHVybiA8dGhlYWQgY2xhc3NOYW1lPVwiZ3J2LXRhYmxlLWhlYWRlclwiPjx0cj57Y2VsbHN9PC90cj48L3RoZWFkPlxuICB9LFxuXG4gIHJlbmRlckJvZHkoY2hpbGRyZW4pe1xuICAgIHZhciBjb3VudCA9IHRoaXMucHJvcHMucm93Q291bnQ7XG4gICAgdmFyIHJvd3MgPSBbXTtcbiAgICBmb3IodmFyIGkgPSAwOyBpIDwgY291bnQ7IGkgKyspe1xuICAgICAgdmFyIGNlbGxzID0gY2hpbGRyZW4ubWFwKChpdGVtLCBpbmRleCk9PntcbiAgICAgICAgcmV0dXJuIHRoaXMucmVuZGVyQ2VsbChpdGVtLnByb3BzLmNlbGwsIHtyb3dJbmRleDogaSwga2V5OiBpbmRleCwgaXNIZWFkZXI6IGZhbHNlLCAuLi5pdGVtLnByb3BzfSk7XG4gICAgICB9KVxuXG4gICAgICByb3dzLnB1c2goPHRyIGtleT17aX0+e2NlbGxzfTwvdHI+KTtcbiAgICB9XG5cbiAgICByZXR1cm4gPHRib2R5Pntyb3dzfTwvdGJvZHk+O1xuICB9LFxuXG4gIHJlbmRlckNlbGwoY2VsbCwgY2VsbFByb3BzKXtcbiAgICB2YXIgY29udGVudCA9IG51bGw7XG4gICAgaWYgKFJlYWN0LmlzVmFsaWRFbGVtZW50KGNlbGwpKSB7XG4gICAgICAgY29udGVudCA9IFJlYWN0LmNsb25lRWxlbWVudChjZWxsLCBjZWxsUHJvcHMpO1xuICAgICB9IGVsc2UgaWYgKHR5cGVvZiBwcm9wcy5jZWxsID09PSAnZnVuY3Rpb24nKSB7XG4gICAgICAgY29udGVudCA9IGNlbGwoY2VsbFByb3BzKTtcbiAgICAgfVxuXG4gICAgIHJldHVybiBjb250ZW50O1xuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICB2YXIgY2hpbGRyZW4gPSBbXTtcbiAgICBSZWFjdC5DaGlsZHJlbi5mb3JFYWNoKHRoaXMucHJvcHMuY2hpbGRyZW4sIChjaGlsZCwgaW5kZXgpID0+IHtcbiAgICAgIGlmIChjaGlsZCA9PSBudWxsKSB7XG4gICAgICAgIHJldHVybjtcbiAgICAgIH1cblxuICAgICAgaWYoY2hpbGQudHlwZS5kaXNwbGF5TmFtZSAhPT0gJ0dydlRhYmxlQ29sdW1uJyl7XG4gICAgICAgIHRocm93ICdTaG91bGQgYmUgR3J2VGFibGVDb2x1bW4nO1xuICAgICAgfVxuXG4gICAgICBjaGlsZHJlbi5wdXNoKGNoaWxkKTtcbiAgICB9KTtcblxuICAgIHZhciB0YWJsZUNsYXNzID0gJ3RhYmxlICcgKyB0aGlzLnByb3BzLmNsYXNzTmFtZTtcblxuICAgIHJldHVybiAoXG4gICAgICA8dGFibGUgY2xhc3NOYW1lPXt0YWJsZUNsYXNzfT5cbiAgICAgICAge3RoaXMucmVuZGVySGVhZGVyKGNoaWxkcmVuKX1cbiAgICAgICAge3RoaXMucmVuZGVyQm9keShjaGlsZHJlbil9XG4gICAgICA8L3RhYmxlPlxuICAgICk7XG4gIH1cbn0pXG5cbnZhciBHcnZUYWJsZUNvbHVtbiA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICB0aHJvdyBuZXcgRXJyb3IoJ0NvbXBvbmVudCA8R3J2VGFibGVDb2x1bW4gLz4gc2hvdWxkIG5ldmVyIHJlbmRlcicpO1xuICB9XG59KVxuXG5leHBvcnQgZGVmYXVsdCBHcnZUYWJsZTtcbmV4cG9ydCB7XG4gIEdydlRhYmxlQ29sdW1uIGFzIENvbHVtbixcbiAgR3J2VGFibGUgYXMgVGFibGUsXG4gIEdydlRhYmxlQ2VsbCBhcyBDZWxsLFxuICBHcnZUYWJsZVRleHRDZWxsIGFzIFRleHRDZWxsLFxuICBTb3J0SGVhZGVyQ2VsbCxcbiAgU29ydEluZGljYXRvcixcbiAgU29ydFR5cGVzfTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL3RhYmxlLmpzeFxuICoqLyIsIm1vZHVsZS5leHBvcnRzID0gXztcblxuXG4vKioqKioqKioqKioqKioqKipcbiAqKiBXRUJQQUNLIEZPT1RFUlxuICoqIGV4dGVybmFsIFwiX1wiXG4gKiogbW9kdWxlIGlkID0gNjFcbiAqKiBtb2R1bGUgY2h1bmtzID0gMVxuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciBzZXNzaW9uID0gcmVxdWlyZSgnYXBwL3Nlc3Npb24nKTtcbnZhciB7dXVpZH0gPSByZXF1aXJlKCdhcHAvdXRpbHMnKTtcbnZhciBhcGkgPSByZXF1aXJlKCdhcHAvc2VydmljZXMvYXBpJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xudmFyIGdldHRlcnMgPSByZXF1aXJlKCcuL2dldHRlcnMnKTtcbnZhciBzZXNzaW9uTW9kdWxlID0gcmVxdWlyZSgnLi8uLi9zZXNzaW9ucycpO1xuXG52YXIgeyBUTFBUX1RFUk1fT1BFTiwgVExQVF9URVJNX0NMT1NFLCBUTFBUX1RFUk1fQ0hBTkdFX1NFUlZFUiB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG52YXIgYWN0aW9ucyA9IHtcblxuICBjaGFuZ2VTZXJ2ZXIoc2VydmVySWQsIGxvZ2luKXtcbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfVEVSTV9DSEFOR0VfU0VSVkVSLCB7XG4gICAgICBzZXJ2ZXJJZCxcbiAgICAgIGxvZ2luXG4gICAgfSk7XG4gIH0sXG5cbiAgY2xvc2UoKXtcbiAgICBsZXQge2lzTmV3U2Vzc2lvbn0gPSByZWFjdG9yLmV2YWx1YXRlKGdldHRlcnMuYWN0aXZlU2Vzc2lvbik7XG5cbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfVEVSTV9DTE9TRSk7XG5cbiAgICBpZihpc05ld1Nlc3Npb24pe1xuICAgICAgc2Vzc2lvbi5nZXRIaXN0b3J5KCkucHVzaChjZmcucm91dGVzLm5vZGVzKTtcbiAgICB9ZWxzZXtcbiAgICAgIHNlc3Npb24uZ2V0SGlzdG9yeSgpLnB1c2goY2ZnLnJvdXRlcy5zZXNzaW9ucyk7XG4gICAgfVxuICB9LFxuXG4gIHJlc2l6ZSh3LCBoKXtcbiAgICAvLyBzb21lIG1pbiB2YWx1ZXNcbiAgICB3ID0gdyA8IDUgPyA1IDogdztcbiAgICBoID0gaCA8IDUgPyA1IDogaDtcblxuICAgIGxldCByZXFEYXRhID0geyB0ZXJtaW5hbF9wYXJhbXM6IHsgdywgaCB9IH07XG4gICAgbGV0IHtzaWR9ID0gcmVhY3Rvci5ldmFsdWF0ZShnZXR0ZXJzLmFjdGl2ZVNlc3Npb24pO1xuXG4gICAgYXBpLnB1dChjZmcuYXBpLmdldFRlcm1pbmFsU2Vzc2lvblVybChzaWQpLCByZXFEYXRhKVxuICAgICAgLmRvbmUoKCk9PntcbiAgICAgICAgY29uc29sZS5sb2coYHJlc2l6ZSB3aXRoIHc6JHt3fSBhbmQgaDoke2h9IC0gT0tgKTtcbiAgICAgIH0pXG4gICAgICAuZmFpbCgoKT0+e1xuICAgICAgICBjb25zb2xlLmxvZyhgZmFpbGVkIHRvIHJlc2l6ZSB3aXRoIHc6JHt3fSBhbmQgaDoke2h9YCk7XG4gICAgfSlcbiAgfSxcblxuICBvcGVuU2Vzc2lvbihzaWQpe1xuICAgIHNlc3Npb25Nb2R1bGUuYWN0aW9ucy5mZXRjaFNlc3Npb24oc2lkKVxuICAgICAgLmRvbmUoKCk9PntcbiAgICAgICAgbGV0IHNWaWV3ID0gcmVhY3Rvci5ldmFsdWF0ZShzZXNzaW9uTW9kdWxlLmdldHRlcnMuc2Vzc2lvblZpZXdCeUlkKHNpZCkpO1xuICAgICAgICBsZXQgeyBzZXJ2ZXJJZCwgbG9naW4gfSA9IHNWaWV3O1xuICAgICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfVEVSTV9PUEVOLCB7XG4gICAgICAgICAgICBzZXJ2ZXJJZCxcbiAgICAgICAgICAgIGxvZ2luLFxuICAgICAgICAgICAgc2lkLFxuICAgICAgICAgICAgaXNOZXdTZXNzaW9uOiBmYWxzZVxuICAgICAgICAgIH0pO1xuICAgICAgfSlcbiAgICAgIC5mYWlsKCgpPT57XG4gICAgICAgIHNlc3Npb24uZ2V0SGlzdG9yeSgpLnB1c2goY2ZnLnJvdXRlcy5wYWdlTm90Rm91bmQpO1xuICAgICAgfSlcbiAgfSxcblxuICBjcmVhdGVOZXdTZXNzaW9uKHNlcnZlcklkLCBsb2dpbil7XG4gICAgdmFyIHNpZCA9IHV1aWQoKTtcbiAgICB2YXIgcm91dGVVcmwgPSBjZmcuZ2V0QWN0aXZlU2Vzc2lvblJvdXRlVXJsKHNpZCk7XG4gICAgdmFyIGhpc3RvcnkgPSBzZXNzaW9uLmdldEhpc3RvcnkoKTtcblxuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9URVJNX09QRU4sIHtcbiAgICAgIHNlcnZlcklkLFxuICAgICAgbG9naW4sXG4gICAgICBzaWQsXG4gICAgICBpc05ld1Nlc3Npb246IHRydWVcbiAgICB9KTtcblxuICAgIGhpc3RvcnkucHVzaChyb3V0ZVVybCk7XG4gIH1cblxufVxuXG5leHBvcnQgZGVmYXVsdCBhY3Rpb25zO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvYWN0aW9ucy5qc1xuICoqLyIsInZhciB7Y3JlYXRlVmlld30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9zZXNzaW9ucy9nZXR0ZXJzJyk7XG5cbmNvbnN0IGFjdGl2ZVNlc3Npb24gPSBbXG5bJ3RscHRfYWN0aXZlX3Rlcm1pbmFsJ10sIFsndGxwdF9zZXNzaW9ucyddLFxuKGFjdGl2ZVRlcm0sIHNlc3Npb25zKSA9PiB7XG4gICAgaWYoIWFjdGl2ZVRlcm0pe1xuICAgICAgcmV0dXJuIG51bGw7XG4gICAgfVxuXG4gICAgLypcbiAgICAqIGFjdGl2ZSBzZXNzaW9uIG5lZWRzIHRvIGhhdmUgaXRzIG93biB2aWV3IGFzIGFuIGFjdHVhbCBzZXNzaW9uIG1pZ2h0IG5vdFxuICAgICogZXhpc3QgYXQgdGhpcyBwb2ludC4gRm9yIGV4YW1wbGUsIHVwb24gY3JlYXRpbmcgYSBuZXcgc2Vzc2lvbiB3ZSBuZWVkIHRvIGtub3dcbiAgICAqIGxvZ2luIGFuZCBzZXJ2ZXJJZC4gSXQgd2lsbCBiZSBzaW1wbGlmaWVkIG9uY2Ugc2VydmVyIEFQSSBnZXRzIGV4dGVuZGVkLlxuICAgICovXG4gICAgbGV0IGFzVmlldyA9IHtcbiAgICAgIGlzTmV3U2Vzc2lvbjogYWN0aXZlVGVybS5nZXQoJ2lzTmV3U2Vzc2lvbicpLFxuICAgICAgbm90Rm91bmQ6IGFjdGl2ZVRlcm0uZ2V0KCdub3RGb3VuZCcpLFxuICAgICAgYWRkcjogYWN0aXZlVGVybS5nZXQoJ2FkZHInKSxcbiAgICAgIHNlcnZlcklkOiBhY3RpdmVUZXJtLmdldCgnc2VydmVySWQnKSxcbiAgICAgIHNlcnZlcklwOiB1bmRlZmluZWQsXG4gICAgICBsb2dpbjogYWN0aXZlVGVybS5nZXQoJ2xvZ2luJyksXG4gICAgICBzaWQ6IGFjdGl2ZVRlcm0uZ2V0KCdzaWQnKSxcbiAgICAgIGNvbHM6IHVuZGVmaW5lZCxcbiAgICAgIHJvd3M6IHVuZGVmaW5lZFxuICAgIH07XG5cbiAgICAvLyBpbiBjYXNlIGlmIHNlc3Npb24gYWxyZWFkeSBleGlzdHMsIGdldCB0aGUgZGF0YSBmcm9tIHRoZXJlXG4gICAgLy8gKGZvciBleGFtcGxlLCB3aGVuIGpvaW5pbmcgYW4gZXhpc3Rpbmcgc2Vzc2lvbilcbiAgICBpZihzZXNzaW9ucy5oYXMoYXNWaWV3LnNpZCkpe1xuICAgICAgbGV0IHNWaWV3ID0gY3JlYXRlVmlldyhzZXNzaW9ucy5nZXQoYXNWaWV3LnNpZCkpO1xuXG4gICAgICBhc1ZpZXcucGFydGllcyA9IHNWaWV3LnBhcnRpZXM7XG4gICAgICBhc1ZpZXcuc2VydmVySXAgPSBzVmlldy5zZXJ2ZXJJcDtcbiAgICAgIGFzVmlldy5zZXJ2ZXJJZCA9IHNWaWV3LnNlcnZlcklkO1xuICAgICAgYXNWaWV3LmFjdGl2ZSA9IHNWaWV3LmFjdGl2ZTtcbiAgICAgIGFzVmlldy5jb2xzID0gc1ZpZXcuY29scztcbiAgICAgIGFzVmlldy5yb3dzID0gc1ZpZXcucm93cztcbiAgICB9XG5cbiAgICByZXR1cm4gYXNWaWV3O1xuXG4gIH1cbl07XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgYWN0aXZlU2Vzc2lvblxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvZ2V0dGVycy5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7IFRMUFRfRElBTE9HX1NFTEVDVF9OT0RFX1NIT1csIFRMUFRfRElBTE9HX1NFTEVDVF9OT0RFX0NMT1NFIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbnZhciBhY3Rpb25zID0ge1xuICBzaG93U2VsZWN0Tm9kZURpYWxvZygpe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9ESUFMT0dfU0VMRUNUX05PREVfU0hPVyk7XG4gIH0sXG5cbiAgY2xvc2VTZWxlY3ROb2RlRGlhbG9nKCl7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX0RJQUxPR19TRUxFQ1RfTk9ERV9DTE9TRSk7XG4gIH1cbn1cblxuZXhwb3J0IGRlZmF1bHQgYWN0aW9ucztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvYWN0aW9ucy5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciBhcGkgPSByZXF1aXJlKCdhcHAvc2VydmljZXMvYXBpJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xuXG52YXIgeyBUTFBUX1NFU1NJTlNfUkVDRUlWRSwgVExQVF9TRVNTSU5TX1VQREFURSB9ICA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuXG4gIGZldGNoU2Vzc2lvbihzaWQpe1xuICAgIHJldHVybiBhcGkuZ2V0KGNmZy5hcGkuZ2V0RmV0Y2hTZXNzaW9uVXJsKHNpZCkpLnRoZW4oanNvbj0+e1xuICAgICAgaWYoanNvbiAmJiBqc29uLnNlc3Npb24pe1xuICAgICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfU0VTU0lOU19VUERBVEUsIGpzb24uc2Vzc2lvbik7XG4gICAgICB9XG4gICAgfSk7XG4gIH0sXG5cbiAgZmV0Y2hTZXNzaW9ucyhzdGFydERhdGUsIGVuZERhdGUpe1xuICAgIHJldHVybiBhcGkuZ2V0KGNmZy5hcGkuZ2V0RmV0Y2hTZXNzaW9uc1VybChzdGFydERhdGUsIGVuZERhdGUpKS5kb25lKChqc29uKSA9PiB7XG4gICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfU0VTU0lOU19SRUNFSVZFLCBqc29uLnNlc3Npb25zKTtcbiAgICB9KTtcbiAgfSxcblxuICB1cGRhdGVTZXNzaW9uKGpzb24pe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9TRVNTSU5TX1VQREFURSwganNvbik7XG4gIH1cbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2FjdGlvbnMuanNcbiAqKi8iLCJ2YXIgYXBpID0gcmVxdWlyZSgnLi9zZXJ2aWNlcy9hcGknKTtcbnZhciBzZXNzaW9uID0gcmVxdWlyZSgnLi9zZXNzaW9uJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xudmFyICQgPSByZXF1aXJlKCdqUXVlcnknKTtcblxuY29uc3QgcmVmcmVzaFJhdGUgPSA2MDAwMCAqIDU7IC8vIDEgbWluXG5cbnZhciByZWZyZXNoVG9rZW5UaW1lcklkID0gbnVsbDtcblxudmFyIGF1dGggPSB7XG5cbiAgc2lnblVwKG5hbWUsIHBhc3N3b3JkLCB0b2tlbiwgaW52aXRlVG9rZW4pe1xuICAgIHZhciBkYXRhID0ge3VzZXI6IG5hbWUsIHBhc3M6IHBhc3N3b3JkLCBzZWNvbmRfZmFjdG9yX3Rva2VuOiB0b2tlbiwgaW52aXRlX3Rva2VuOiBpbnZpdGVUb2tlbn07XG4gICAgcmV0dXJuIGFwaS5wb3N0KGNmZy5hcGkuY3JlYXRlVXNlclBhdGgsIGRhdGEpXG4gICAgICAudGhlbigodXNlcik9PntcbiAgICAgICAgc2Vzc2lvbi5zZXRVc2VyRGF0YSh1c2VyKTtcbiAgICAgICAgYXV0aC5fc3RhcnRUb2tlblJlZnJlc2hlcigpO1xuICAgICAgICByZXR1cm4gdXNlcjtcbiAgICAgIH0pO1xuICB9LFxuXG4gIGxvZ2luKG5hbWUsIHBhc3N3b3JkLCB0b2tlbil7XG4gICAgYXV0aC5fc3RvcFRva2VuUmVmcmVzaGVyKCk7XG4gICAgcmV0dXJuIGF1dGguX2xvZ2luKG5hbWUsIHBhc3N3b3JkLCB0b2tlbikuZG9uZShhdXRoLl9zdGFydFRva2VuUmVmcmVzaGVyKTtcbiAgfSxcblxuICBlbnN1cmVVc2VyKCl7XG4gICAgdmFyIHVzZXJEYXRhID0gc2Vzc2lvbi5nZXRVc2VyRGF0YSgpO1xuICAgIGlmKHVzZXJEYXRhLnRva2VuKXtcbiAgICAgIC8vIHJlZnJlc2ggdGltZXIgd2lsbCBub3QgYmUgc2V0IGluIGNhc2Ugb2YgYnJvd3NlciByZWZyZXNoIGV2ZW50XG4gICAgICBpZihhdXRoLl9nZXRSZWZyZXNoVG9rZW5UaW1lcklkKCkgPT09IG51bGwpe1xuICAgICAgICByZXR1cm4gYXV0aC5fcmVmcmVzaFRva2VuKCkuZG9uZShhdXRoLl9zdGFydFRva2VuUmVmcmVzaGVyKTtcbiAgICAgIH1cblxuICAgICAgcmV0dXJuICQuRGVmZXJyZWQoKS5yZXNvbHZlKHVzZXJEYXRhKTtcbiAgICB9XG5cbiAgICByZXR1cm4gJC5EZWZlcnJlZCgpLnJlamVjdCgpO1xuICB9LFxuXG4gIGxvZ291dCgpe1xuICAgIGF1dGguX3N0b3BUb2tlblJlZnJlc2hlcigpO1xuICAgIHNlc3Npb24uY2xlYXIoKTtcbiAgICBzZXNzaW9uLmdldEhpc3RvcnkoKS5yZXBsYWNlKHtwYXRobmFtZTogY2ZnLnJvdXRlcy5sb2dpbn0pOyAgICBcbiAgfSxcblxuICBfc3RhcnRUb2tlblJlZnJlc2hlcigpe1xuICAgIHJlZnJlc2hUb2tlblRpbWVySWQgPSBzZXRJbnRlcnZhbChhdXRoLl9yZWZyZXNoVG9rZW4sIHJlZnJlc2hSYXRlKTtcbiAgfSxcblxuICBfc3RvcFRva2VuUmVmcmVzaGVyKCl7XG4gICAgY2xlYXJJbnRlcnZhbChyZWZyZXNoVG9rZW5UaW1lcklkKTtcbiAgICByZWZyZXNoVG9rZW5UaW1lcklkID0gbnVsbDtcbiAgfSxcblxuICBfZ2V0UmVmcmVzaFRva2VuVGltZXJJZCgpe1xuICAgIHJldHVybiByZWZyZXNoVG9rZW5UaW1lcklkO1xuICB9LFxuXG4gIF9yZWZyZXNoVG9rZW4oKXtcbiAgICByZXR1cm4gYXBpLnBvc3QoY2ZnLmFwaS5yZW5ld1Rva2VuUGF0aCkudGhlbihkYXRhPT57XG4gICAgICBzZXNzaW9uLnNldFVzZXJEYXRhKGRhdGEpO1xuICAgICAgcmV0dXJuIGRhdGE7XG4gICAgfSkuZmFpbCgoKT0+e1xuICAgICAgYXV0aC5sb2dvdXQoKTtcbiAgICB9KTtcbiAgfSxcblxuICBfbG9naW4obmFtZSwgcGFzc3dvcmQsIHRva2VuKXtcbiAgICB2YXIgZGF0YSA9IHtcbiAgICAgIHVzZXI6IG5hbWUsXG4gICAgICBwYXNzOiBwYXNzd29yZCxcbiAgICAgIHNlY29uZF9mYWN0b3JfdG9rZW46IHRva2VuXG4gICAgfTtcblxuICAgIHJldHVybiBhcGkucG9zdChjZmcuYXBpLnNlc3Npb25QYXRoLCBkYXRhLCBmYWxzZSkudGhlbihkYXRhPT57XG4gICAgICBzZXNzaW9uLnNldFVzZXJEYXRhKGRhdGEpO1xuICAgICAgcmV0dXJuIGRhdGE7XG4gICAgfSk7XG4gIH1cbn1cblxubW9kdWxlLmV4cG9ydHMgPSBhdXRoO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2F1dGguanNcbiAqKi8iLCJ2YXIgbW9tZW50ID0gcmVxdWlyZSgnbW9tZW50Jyk7XG5cbm1vZHVsZS5leHBvcnRzLm1vbnRoUmFuZ2UgPSBmdW5jdGlvbih2YWx1ZSA9IG5ldyBEYXRlKCkpe1xuICBsZXQgc3RhcnREYXRlID0gbW9tZW50KHZhbHVlKS5zdGFydE9mKCdtb250aCcpLnRvRGF0ZSgpO1xuICBsZXQgZW5kRGF0ZSA9IG1vbWVudCh2YWx1ZSkuZW5kT2YoJ21vbnRoJykudG9EYXRlKCk7XG4gIHJldHVybiBbc3RhcnREYXRlLCBlbmREYXRlXTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21tb24vZGF0ZVV0aWxzLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMuaXNNYXRjaCA9IGZ1bmN0aW9uKG9iaiwgc2VhcmNoVmFsdWUsIHtzZWFyY2hhYmxlUHJvcHMsIGNifSkge1xuICBzZWFyY2hWYWx1ZSA9IHNlYXJjaFZhbHVlLnRvTG9jYWxlVXBwZXJDYXNlKCk7XG4gIGxldCBwcm9wTmFtZXMgPSBzZWFyY2hhYmxlUHJvcHMgfHwgT2JqZWN0LmdldE93blByb3BlcnR5TmFtZXMob2JqKTtcbiAgZm9yIChsZXQgaSA9IDA7IGkgPCBwcm9wTmFtZXMubGVuZ3RoOyBpKyspIHtcbiAgICBsZXQgdGFyZ2V0VmFsdWUgPSBvYmpbcHJvcE5hbWVzW2ldXTtcbiAgICBpZiAodGFyZ2V0VmFsdWUpIHtcbiAgICAgIGlmKHR5cGVvZiBjYiA9PT0gJ2Z1bmN0aW9uJyl7XG4gICAgICAgIGxldCByZXN1bHQgPSBjYih0YXJnZXRWYWx1ZSwgc2VhcmNoVmFsdWUsIHByb3BOYW1lc1tpXSk7XG4gICAgICAgIGlmKHJlc3VsdCA9PT0gdHJ1ZSl7XG4gICAgICAgICAgcmV0dXJuIHJlc3VsdDtcbiAgICAgICAgfVxuICAgICAgfVxuXG4gICAgICBpZiAodGFyZ2V0VmFsdWUudG9TdHJpbmcoKS50b0xvY2FsZVVwcGVyQ2FzZSgpLmluZGV4T2Yoc2VhcmNoVmFsdWUpICE9PSAtMSkge1xuICAgICAgICByZXR1cm4gdHJ1ZTtcbiAgICAgIH1cbiAgICB9XG4gIH1cblxuICByZXR1cm4gZmFsc2U7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tbW9uL29iamVjdFV0aWxzLmpzXG4gKiovIiwidmFyIEV2ZW50RW1pdHRlciA9IHJlcXVpcmUoJ2V2ZW50cycpLkV2ZW50RW1pdHRlcjtcbnZhciBzZXNzaW9uID0gcmVxdWlyZSgnYXBwL3Nlc3Npb24nKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG52YXIge2FjdGlvbnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvJyk7XG5cbmNsYXNzIFR0eSBleHRlbmRzIEV2ZW50RW1pdHRlciB7XG5cbiAgY29uc3RydWN0b3Ioe3NlcnZlcklkLCBsb2dpbiwgc2lkLCByb3dzLCBjb2xzIH0pe1xuICAgIHN1cGVyKCk7XG4gICAgdGhpcy5vcHRpb25zID0geyBzZXJ2ZXJJZCwgbG9naW4sIHNpZCwgcm93cywgY29scyB9O1xuICAgIHRoaXMuc29ja2V0ID0gbnVsbDtcbiAgfVxuXG4gIGRpc2Nvbm5lY3QoKXtcbiAgICB0aGlzLnNvY2tldC5jbG9zZSgpO1xuICB9XG5cbiAgcmVjb25uZWN0KG9wdGlvbnMpe1xuICAgIHRoaXMuZGlzY29ubmVjdCgpO1xuICAgIHRoaXMuc29ja2V0Lm9ub3BlbiA9IG51bGw7XG4gICAgdGhpcy5zb2NrZXQub25tZXNzYWdlID0gbnVsbDtcbiAgICB0aGlzLnNvY2tldC5vbmNsb3NlID0gbnVsbDtcbiAgICBcbiAgICB0aGlzLmNvbm5lY3Qob3B0aW9ucyk7XG4gIH1cblxuICBjb25uZWN0KG9wdGlvbnMpe1xuICAgIE9iamVjdC5hc3NpZ24odGhpcy5vcHRpb25zLCBvcHRpb25zKTtcblxuICAgIGxldCB7dG9rZW59ID0gc2Vzc2lvbi5nZXRVc2VyRGF0YSgpO1xuICAgIGxldCBjb25uU3RyID0gY2ZnLmFwaS5nZXRUdHlDb25uU3RyKHt0b2tlbiwgLi4udGhpcy5vcHRpb25zfSk7XG5cbiAgICB0aGlzLnNvY2tldCA9IG5ldyBXZWJTb2NrZXQoY29ublN0ciwgJ3Byb3RvJyk7XG5cbiAgICB0aGlzLnNvY2tldC5vbm9wZW4gPSAoKSA9PiB7XG4gICAgICB0aGlzLmVtaXQoJ29wZW4nKTtcbiAgICB9XG5cbiAgICB0aGlzLnNvY2tldC5vbm1lc3NhZ2UgPSAoZSk9PntcbiAgICAgIHRoaXMuZW1pdCgnZGF0YScsIGUuZGF0YSk7XG4gICAgfVxuXG4gICAgdGhpcy5zb2NrZXQub25jbG9zZSA9ICgpPT57XG4gICAgICB0aGlzLmVtaXQoJ2Nsb3NlJyk7XG4gICAgfVxuICB9XG5cbiAgcmVzaXplKGNvbHMsIHJvd3Mpe1xuICAgIGFjdGlvbnMucmVzaXplKGNvbHMsIHJvd3MpO1xuICB9XG5cbiAgc2VuZChkYXRhKXtcbiAgICB0aGlzLnNvY2tldC5zZW5kKGRhdGEpO1xuICB9XG59XG5cbm1vZHVsZS5leHBvcnRzID0gVHR5O1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbW1vbi90dHkuanNcbiAqKi8iLCJpbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVExQVF9URVJNX09QRU46IG51bGwsXG4gIFRMUFRfVEVSTV9DTE9TRTogbnVsbCxcbiAgVExQVF9URVJNX0NIQU5HRV9TRVJWRVI6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hY3RpdmVUZXJtaW5hbC9hY3Rpb25UeXBlcy5qc1xuICoqLyIsInZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyIHsgVExQVF9URVJNX09QRU4sIFRMUFRfVEVSTV9DTE9TRSwgVExQVF9URVJNX0NIQU5HRV9TRVJWRVIgfSAgPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IFN0b3JlKHtcbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB0b0ltbXV0YWJsZShudWxsKTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9URVJNX09QRU4sIHNldEFjdGl2ZVRlcm1pbmFsKTtcbiAgICB0aGlzLm9uKFRMUFRfVEVSTV9DTE9TRSwgY2xvc2UpO1xuICAgIHRoaXMub24oVExQVF9URVJNX0NIQU5HRV9TRVJWRVIsIGNoYW5nZVNlcnZlcik7XG4gIH1cbn0pXG5cbmZ1bmN0aW9uIGNoYW5nZVNlcnZlcihzdGF0ZSwge3NlcnZlcklkLCBsb2dpbn0pe1xuICByZXR1cm4gc3RhdGUuc2V0KCdzZXJ2ZXJJZCcsIHNlcnZlcklkKVxuICAgICAgICAgICAgICAuc2V0KCdsb2dpbicsIGxvZ2luKTtcbn1cblxuZnVuY3Rpb24gY2xvc2UoKXtcbiAgcmV0dXJuIHRvSW1tdXRhYmxlKG51bGwpO1xufVxuXG5mdW5jdGlvbiBzZXRBY3RpdmVUZXJtaW5hbChzdGF0ZSwge3NlcnZlcklkLCBsb2dpbiwgc2lkLCBpc05ld1Nlc3Npb259ICl7XG4gIHJldHVybiB0b0ltbXV0YWJsZSh7XG4gICAgc2VydmVySWQsXG4gICAgbG9naW4sXG4gICAgc2lkLFxuICAgIGlzTmV3U2Vzc2lvblxuICB9KTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2FjdGl2ZVRlcm1TdG9yZS5qc1xuICoqLyIsImltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX0FQUF9JTklUOiBudWxsLFxuICBUTFBUX0FQUF9GQUlMRUQ6IG51bGwsXG4gIFRMUFRfQVBQX1JFQURZOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvYXBwL2FjdGlvblR5cGVzLmpzXG4gKiovIiwidmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG5cbnZhciB7IFRMUFRfQVBQX0lOSVQsIFRMUFRfQVBQX0ZBSUxFRCwgVExQVF9BUFBfUkVBRFkgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxudmFyIGluaXRTdGF0ZSA9IHRvSW1tdXRhYmxlKHtcbiAgaXNSZWFkeTogZmFsc2UsXG4gIGlzSW5pdGlhbGl6aW5nOiBmYWxzZSxcbiAgaXNGYWlsZWQ6IGZhbHNlXG59KTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuXG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gaW5pdFN0YXRlLnNldCgnaXNJbml0aWFsaXppbmcnLCB0cnVlKTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9BUFBfSU5JVCwgKCk9PiBpbml0U3RhdGUuc2V0KCdpc0luaXRpYWxpemluZycsIHRydWUpKTtcbiAgICB0aGlzLm9uKFRMUFRfQVBQX1JFQURZLCgpPT4gaW5pdFN0YXRlLnNldCgnaXNSZWFkeScsIHRydWUpKTtcbiAgICB0aGlzLm9uKFRMUFRfQVBQX0ZBSUxFRCwoKT0+IGluaXRTdGF0ZS5zZXQoJ2lzRmFpbGVkJywgdHJ1ZSkpO1xuICB9XG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvYXBwL2FwcFN0b3JlLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfRElBTE9HX1NFTEVDVF9OT0RFX1NIT1c6IG51bGwsXG4gIFRMUFRfRElBTE9HX1NFTEVDVF9OT0RFX0NMT1NFOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvZGlhbG9ncy9hY3Rpb25UeXBlcy5qc1xuICoqLyIsInZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xuXG52YXIgeyBUTFBUX0RJQUxPR19TRUxFQ1RfTk9ERV9TSE9XLCBUTFBUX0RJQUxPR19TRUxFQ1RfTk9ERV9DTE9TRSB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG5cbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB0b0ltbXV0YWJsZSh7XG4gICAgICBpc1NlbGVjdE5vZGVEaWFsb2dPcGVuOiBmYWxzZVxuICAgIH0pO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX0RJQUxPR19TRUxFQ1RfTk9ERV9TSE9XLCBzaG93U2VsZWN0Tm9kZURpYWxvZyk7XG4gICAgdGhpcy5vbihUTFBUX0RJQUxPR19TRUxFQ1RfTk9ERV9DTE9TRSwgY2xvc2VTZWxlY3ROb2RlRGlhbG9nKTtcbiAgfVxufSlcblxuZnVuY3Rpb24gc2hvd1NlbGVjdE5vZGVEaWFsb2coc3RhdGUpe1xuICByZXR1cm4gc3RhdGUuc2V0KCdpc1NlbGVjdE5vZGVEaWFsb2dPcGVuJywgdHJ1ZSk7XG59XG5cbmZ1bmN0aW9uIGNsb3NlU2VsZWN0Tm9kZURpYWxvZyhzdGF0ZSl7XG4gIHJldHVybiBzdGF0ZS5zZXQoJ2lzU2VsZWN0Tm9kZURpYWxvZ09wZW4nLCBmYWxzZSk7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9kaWFsb2dzL2RpYWxvZ1N0b3JlLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfUkVDRUlWRV9VU0VSX0lOVklURTogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9hY3Rpb25UeXBlcy5qc1xuICoqLyIsInZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyICB7IFRMUFRfUkVDRUlWRV9VU0VSX0lOVklURSB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gdG9JbW11dGFibGUobnVsbCk7XG4gIH0sXG5cbiAgaW5pdGlhbGl6ZSgpIHtcbiAgICB0aGlzLm9uKFRMUFRfUkVDRUlWRV9VU0VSX0lOVklURSwgcmVjZWl2ZUludml0ZSlcbiAgfVxufSlcblxuZnVuY3Rpb24gcmVjZWl2ZUludml0ZShzdGF0ZSwgaW52aXRlKXtcbiAgcmV0dXJuIHRvSW1tdXRhYmxlKGludml0ZSk7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvaW52aXRlU3RvcmUuanNcbiAqKi8iLCJpbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVExQVF9OT0RFU19SRUNFSVZFOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvYWN0aW9uVHlwZXMuanNcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgeyBUTFBUX05PREVTX1JFQ0VJVkUgfSAgPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG52YXIgYXBpID0gcmVxdWlyZSgnYXBwL3NlcnZpY2VzL2FwaScpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICBmZXRjaE5vZGVzKCl7XG4gICAgYXBpLmdldChjZmcuYXBpLm5vZGVzUGF0aCkuZG9uZSgoZGF0YT1bXSk9PntcbiAgICAgIHZhciBub2RlQXJyYXkgPSBkYXRhLm5vZGVzLm1hcChpdGVtPT5pdGVtLm5vZGUpO1xuICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX05PREVTX1JFQ0VJVkUsIG5vZGVBcnJheSk7XG4gICAgfSk7XG4gIH1cbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2FjdGlvbnMuanNcbiAqKi8iLCJ2YXIgeyBTdG9yZSwgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcbnZhciAgeyBUTFBUX05PREVTX1JFQ0VJVkUgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKFtdKTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9OT0RFU19SRUNFSVZFLCByZWNlaXZlTm9kZXMpXG4gIH1cbn0pXG5cbmZ1bmN0aW9uIHJlY2VpdmVOb2RlcyhzdGF0ZSwgbm9kZUFycmF5KXtcbiAgcmV0dXJuIHRvSW1tdXRhYmxlKG5vZGVBcnJheSk7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9ub2RlU3RvcmUuanNcbiAqKi8iLCJpbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVExQVF9SRVNUX0FQSV9TVEFSVDogbnVsbCxcbiAgVExQVF9SRVNUX0FQSV9TVUNDRVNTOiBudWxsLFxuICBUTFBUX1JFU1RfQVBJX0ZBSUw6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2FjdGlvblR5cGVzLmpzXG4gKiovIiwidmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xuXG52YXIge1xuICBUTFBUX1JFU1RfQVBJX1NUQVJULFxuICBUTFBUX1JFU1RfQVBJX1NVQ0NFU1MsXG4gIFRMUFRfUkVTVF9BUElfRkFJTCB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCB7XG5cbiAgc3RhcnQocmVxVHlwZSl7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFU1RfQVBJX1NUQVJULCB7dHlwZTogcmVxVHlwZX0pO1xuICB9LFxuXG4gIGZhaWwocmVxVHlwZSwgbWVzc2FnZSl7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFU1RfQVBJX0ZBSUwsICB7dHlwZTogcmVxVHlwZSwgbWVzc2FnZX0pO1xuICB9LFxuXG4gIHN1Y2Nlc3MocmVxVHlwZSl7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFU1RfQVBJX1NVQ0NFU1MsIHt0eXBlOiByZXFUeXBlfSk7XG4gIH1cblxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9hY3Rpb25zLmpzXG4gKiovIiwidmFyIGRlZmF1bHRPYmogPSB7XG4gIGlzUHJvY2Vzc2luZzogZmFsc2UsXG4gIGlzRXJyb3I6IGZhbHNlLFxuICBpc1N1Y2Nlc3M6IGZhbHNlLFxuICBtZXNzYWdlOiAnJ1xufVxuXG5jb25zdCByZXF1ZXN0U3RhdHVzID0gKHJlcVR5cGUpID0+ICBbIFsndGxwdF9yZXN0X2FwaScsIHJlcVR5cGVdLCAoYXR0ZW1wKSA9PiB7XG4gIHJldHVybiBhdHRlbXAgPyBhdHRlbXAudG9KUygpIDogZGVmYXVsdE9iajtcbiB9XG5dO1xuXG5leHBvcnQgZGVmYXVsdCB7ICByZXF1ZXN0U3RhdHVzICB9O1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9nZXR0ZXJzLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfU0VTU0lOU19SRUNFSVZFOiBudWxsLFxuICBUTFBUX1NFU1NJTlNfVVBEQVRFOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvYWN0aW9uVHlwZXMuanNcbiAqKi8iLCJ2YXIgeyB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxuY29uc3Qgc2Vzc2lvbnNCeVNlcnZlciA9IChzZXJ2ZXJJZCkgPT4gW1sndGxwdF9zZXNzaW9ucyddLCAoc2Vzc2lvbnMpID0+e1xuICByZXR1cm4gc2Vzc2lvbnMudmFsdWVTZXEoKS5maWx0ZXIoaXRlbT0+e1xuICAgIHZhciBwYXJ0aWVzID0gaXRlbS5nZXQoJ3BhcnRpZXMnKSB8fCB0b0ltbXV0YWJsZShbXSk7XG4gICAgdmFyIGhhc1NlcnZlciA9IHBhcnRpZXMuZmluZChpdGVtMj0+IGl0ZW0yLmdldCgnc2VydmVyX2lkJykgPT09IHNlcnZlcklkKTtcbiAgICByZXR1cm4gaGFzU2VydmVyO1xuICB9KS50b0xpc3QoKTtcbn1dXG5cbmNvbnN0IHNlc3Npb25zVmlldyA9IFtbJ3RscHRfc2Vzc2lvbnMnXSwgKHNlc3Npb25zKSA9PntcbiAgcmV0dXJuIHNlc3Npb25zLnZhbHVlU2VxKCkubWFwKGNyZWF0ZVZpZXcpLnRvSlMoKTtcbn1dO1xuXG5jb25zdCBzZXNzaW9uVmlld0J5SWQgPSAoc2lkKT0+IFtbJ3RscHRfc2Vzc2lvbnMnLCBzaWRdLCAoc2Vzc2lvbik9PntcbiAgaWYoIXNlc3Npb24pe1xuICAgIHJldHVybiBudWxsO1xuICB9XG5cbiAgcmV0dXJuIGNyZWF0ZVZpZXcoc2Vzc2lvbik7XG59XTtcblxuY29uc3QgcGFydGllc0J5U2Vzc2lvbklkID0gKHNpZCkgPT5cbiBbWyd0bHB0X3Nlc3Npb25zJywgc2lkLCAncGFydGllcyddLCAocGFydGllcykgPT57XG5cbiAgaWYoIXBhcnRpZXMpe1xuICAgIHJldHVybiBbXTtcbiAgfVxuXG4gIHZhciBsYXN0QWN0aXZlVXNyTmFtZSA9IGdldExhc3RBY3RpdmVVc2VyKHBhcnRpZXMpLmdldCgndXNlcicpO1xuXG4gIHJldHVybiBwYXJ0aWVzLm1hcChpdGVtPT57XG4gICAgdmFyIHVzZXIgPSBpdGVtLmdldCgndXNlcicpO1xuICAgIHJldHVybiB7XG4gICAgICB1c2VyOiBpdGVtLmdldCgndXNlcicpLFxuICAgICAgc2VydmVySXA6IGl0ZW0uZ2V0KCdyZW1vdGVfYWRkcicpLFxuICAgICAgc2VydmVySWQ6IGl0ZW0uZ2V0KCdzZXJ2ZXJfaWQnKSxcbiAgICAgIGlzQWN0aXZlOiBsYXN0QWN0aXZlVXNyTmFtZSA9PT0gdXNlclxuICAgIH1cbiAgfSkudG9KUygpO1xufV07XG5cbmZ1bmN0aW9uIGdldExhc3RBY3RpdmVVc2VyKHBhcnRpZXMpe1xuICByZXR1cm4gcGFydGllcy5zb3J0QnkoaXRlbT0+IG5ldyBEYXRlKGl0ZW0uZ2V0KCdsYXN0QWN0aXZlJykpKS5maXJzdCgpO1xufVxuXG5mdW5jdGlvbiBjcmVhdGVWaWV3KHNlc3Npb24pe1xuICB2YXIgc2lkID0gc2Vzc2lvbi5nZXQoJ2lkJyk7XG4gIHZhciBzZXJ2ZXJJcCwgc2VydmVySWQ7XG4gIHZhciBwYXJ0aWVzID0gcmVhY3Rvci5ldmFsdWF0ZShwYXJ0aWVzQnlTZXNzaW9uSWQoc2lkKSk7XG5cbiAgaWYocGFydGllcy5sZW5ndGggPiAwKXtcbiAgICBzZXJ2ZXJJcCA9IHBhcnRpZXNbMF0uc2VydmVySXA7XG4gICAgc2VydmVySWQgPSBwYXJ0aWVzWzBdLnNlcnZlcklkO1xuICB9XG5cbiAgcmV0dXJuIHtcbiAgICBzaWQ6IHNpZCxcbiAgICBzZXNzaW9uVXJsOiBjZmcuZ2V0QWN0aXZlU2Vzc2lvblJvdXRlVXJsKHNpZCksXG4gICAgc2VydmVySXAsXG4gICAgc2VydmVySWQsXG4gICAgYWN0aXZlOiBzZXNzaW9uLmdldCgnYWN0aXZlJyksXG4gICAgY3JlYXRlZDogbmV3IERhdGUoc2Vzc2lvbi5nZXQoJ2NyZWF0ZWQnKSksXG4gICAgbGFzdEFjdGl2ZTogbmV3IERhdGUoc2Vzc2lvbi5nZXQoJ2xhc3RfYWN0aXZlJykpLFxuICAgIGxvZ2luOiBzZXNzaW9uLmdldCgnbG9naW4nKSxcbiAgICBwYXJ0aWVzOiBwYXJ0aWVzLFxuICAgIGNvbHM6IHNlc3Npb24uZ2V0SW4oWyd0ZXJtaW5hbF9wYXJhbXMnLCAndyddKSxcbiAgICByb3dzOiBzZXNzaW9uLmdldEluKFsndGVybWluYWxfcGFyYW1zJywgJ2gnXSlcbiAgfVxufVxuXG5leHBvcnQgZGVmYXVsdCB7XG4gIHBhcnRpZXNCeVNlc3Npb25JZCxcbiAgc2Vzc2lvbnNCeVNlcnZlcixcbiAgc2Vzc2lvbnNWaWV3LFxuICBzZXNzaW9uVmlld0J5SWQsXG4gIGNyZWF0ZVZpZXdcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2dldHRlcnMuanNcbiAqKi8iLCJ2YXIgeyBTdG9yZSwgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcbnZhciB7IFRMUFRfU0VTU0lOU19SRUNFSVZFLCBUTFBUX1NFU1NJTlNfVVBEQVRFIH0gID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gdG9JbW11dGFibGUoe30pO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX1NFU1NJTlNfUkVDRUlWRSwgcmVjZWl2ZVNlc3Npb25zKTtcbiAgICB0aGlzLm9uKFRMUFRfU0VTU0lOU19VUERBVEUsIHVwZGF0ZVNlc3Npb24pO1xuICB9XG59KVxuXG5mdW5jdGlvbiB1cGRhdGVTZXNzaW9uKHN0YXRlLCBqc29uKXtcbiAgcmV0dXJuIHN0YXRlLnNldChqc29uLmlkLCB0b0ltbXV0YWJsZShqc29uKSk7XG59XG5cbmZ1bmN0aW9uIHJlY2VpdmVTZXNzaW9ucyhzdGF0ZSwganNvbkFycmF5PVtdKXtcbiAgcmV0dXJuIHN0YXRlLndpdGhNdXRhdGlvbnMoc3RhdGUgPT4ge1xuICAgIGpzb25BcnJheS5mb3JFYWNoKChpdGVtKSA9PiB7XG4gICAgICBzdGF0ZS5zZXQoaXRlbS5pZCwgdG9JbW11dGFibGUoaXRlbSkpXG4gICAgfSlcbiAgfSk7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9zZXNzaW9ucy9zZXNzaW9uU3RvcmUuanNcbiAqKi8iLCJpbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVExQVF9SRUNFSVZFX1VTRVI6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy91c2VyL2FjdGlvblR5cGVzLmpzXG4gKiovIiwidmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHsgVExQVF9SRUNFSVZFX1VTRVIgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcbnZhciB7IFRSWUlOR19UT19TSUdOX1VQLCBUUllJTkdfVE9fTE9HSU59ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvcmVzdEFwaS9jb25zdGFudHMnKTtcbnZhciByZXN0QXBpQWN0aW9ucyA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3Jlc3RBcGkvYWN0aW9ucycpO1xudmFyIGF1dGggPSByZXF1aXJlKCdhcHAvYXV0aCcpO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCdhcHAvc2Vzc2lvbicpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuXG4gIGVuc3VyZVVzZXIobmV4dFN0YXRlLCByZXBsYWNlLCBjYil7XG4gICAgYXV0aC5lbnN1cmVVc2VyKClcbiAgICAgIC5kb25lKCh1c2VyRGF0YSk9PiB7XG4gICAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRUNFSVZFX1VTRVIsIHVzZXJEYXRhLnVzZXIgKTtcbiAgICAgICAgY2IoKTtcbiAgICAgIH0pXG4gICAgICAuZmFpbCgoKT0+e1xuICAgICAgICByZXBsYWNlKHtyZWRpcmVjdFRvOiBuZXh0U3RhdGUubG9jYXRpb24ucGF0aG5hbWUgfSwgY2ZnLnJvdXRlcy5sb2dpbik7XG4gICAgICAgIGNiKCk7XG4gICAgICB9KTtcbiAgfSxcblxuICBzaWduVXAoe25hbWUsIHBzdywgdG9rZW4sIGludml0ZVRva2VufSl7XG4gICAgcmVzdEFwaUFjdGlvbnMuc3RhcnQoVFJZSU5HX1RPX1NJR05fVVApO1xuICAgIGF1dGguc2lnblVwKG5hbWUsIHBzdywgdG9rZW4sIGludml0ZVRva2VuKVxuICAgICAgLmRvbmUoKHNlc3Npb25EYXRhKT0+e1xuICAgICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfUkVDRUlWRV9VU0VSLCBzZXNzaW9uRGF0YS51c2VyKTtcbiAgICAgICAgcmVzdEFwaUFjdGlvbnMuc3VjY2VzcyhUUllJTkdfVE9fU0lHTl9VUCk7XG4gICAgICAgIHNlc3Npb24uZ2V0SGlzdG9yeSgpLnB1c2goe3BhdGhuYW1lOiBjZmcucm91dGVzLmFwcH0pO1xuICAgICAgfSlcbiAgICAgIC5mYWlsKChlcnIpPT57XG4gICAgICAgIHJlc3RBcGlBY3Rpb25zLmZhaWwoVFJZSU5HX1RPX1NJR05fVVAsIGVyci5yZXNwb25zZUpTT04ubWVzc2FnZSB8fCAnZmFpbGVkIHRvIHNpbmcgdXAnKTtcbiAgICAgIH0pO1xuICB9LFxuXG4gIGxvZ2luKHt1c2VyLCBwYXNzd29yZCwgdG9rZW59LCByZWRpcmVjdCl7XG4gICAgcmVzdEFwaUFjdGlvbnMuc3RhcnQoVFJZSU5HX1RPX0xPR0lOKTtcbiAgICBhdXRoLmxvZ2luKHVzZXIsIHBhc3N3b3JkLCB0b2tlbilcbiAgICAgIC5kb25lKChzZXNzaW9uRGF0YSk9PntcbiAgICAgICAgcmVzdEFwaUFjdGlvbnMuc3VjY2VzcyhUUllJTkdfVE9fTE9HSU4pO1xuICAgICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfUkVDRUlWRV9VU0VSLCBzZXNzaW9uRGF0YS51c2VyKTtcbiAgICAgICAgc2Vzc2lvbi5nZXRIaXN0b3J5KCkucHVzaCh7cGF0aG5hbWU6IHJlZGlyZWN0fSk7XG4gICAgICB9KVxuICAgICAgLmZhaWwoKGVycik9PiByZXN0QXBpQWN0aW9ucy5mYWlsKFRSWUlOR19UT19MT0dJTiwgZXJyLnJlc3BvbnNlSlNPTi5tZXNzYWdlKSlcbiAgICB9XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy91c2VyL2FjdGlvbnMuanNcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cy5nZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3Rpb25zID0gcmVxdWlyZSgnLi9hY3Rpb25zJyk7XG5tb2R1bGUuZXhwb3J0cy5ub2RlU3RvcmUgPSByZXF1aXJlKCcuL3VzZXJTdG9yZScpO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvdXNlci9pbmRleC5qc1xuICoqLyIsInZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyICB7IFRMUFRfUkVDRUlWRV9VU0VSIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IFN0b3JlKHtcbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB0b0ltbXV0YWJsZShudWxsKTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9SRUNFSVZFX1VTRVIsIHJlY2VpdmVVc2VyKVxuICB9XG5cbn0pXG5cbmZ1bmN0aW9uIHJlY2VpdmVVc2VyKHN0YXRlLCB1c2VyKXtcbiAgcmV0dXJuIHRvSW1tdXRhYmxlKHVzZXIpO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvdXNlci91c2VyU3RvcmUuanNcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHthY3Rpb25zfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsLycpO1xudmFyIGNvbG9ycyA9IFsnIzFhYjM5NCcsICcjMWM4NGM2JywgJyMyM2M2YzgnLCAnI2Y4YWM1OScsICcjRUQ1NTY1JywgJyNjMmMyYzInXTtcblxuY29uc3QgVXNlckljb24gPSAoe25hbWUsIHRpdGxlLCBjb2xvckluZGV4PTB9KT0+e1xuICBsZXQgY29sb3IgPSBjb2xvcnNbY29sb3JJbmRleCAlIGNvbG9ycy5sZW5ndGhdO1xuICBsZXQgc3R5bGUgPSB7XG4gICAgJ2JhY2tncm91bmRDb2xvcic6IGNvbG9yLFxuICAgICdib3JkZXJDb2xvcic6IGNvbG9yXG4gIH07XG5cbiAgcmV0dXJuIChcbiAgICA8bGk+XG4gICAgICA8c3BhbiBzdHlsZT17c3R5bGV9IGNsYXNzTmFtZT1cImJ0biBidG4tcHJpbWFyeSBidG4tY2lyY2xlIHRleHQtdXBwZXJjYXNlXCI+XG4gICAgICAgIDxzdHJvbmc+e25hbWVbMF19PC9zdHJvbmc+XG4gICAgICA8L3NwYW4+XG4gICAgPC9saT5cbiAgKVxufTtcblxuY29uc3QgU2Vzc2lvbkxlZnRQYW5lbCA9ICh7cGFydGllc30pID0+IHtcbiAgcGFydGllcyA9IHBhcnRpZXMgfHwgW107XG4gIGxldCB1c2VySWNvbnMgPSBwYXJ0aWVzLm1hcCgoaXRlbSwgaW5kZXgpPT4oXG4gICAgPFVzZXJJY29uIGtleT17aW5kZXh9IGNvbG9ySW5kZXg9e2luZGV4fSBuYW1lPXtpdGVtLnVzZXJ9Lz5cbiAgKSk7XG5cbiAgcmV0dXJuIChcbiAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi10ZXJtaW5hbC1wYXJ0aWNpcGFuc1wiPlxuICAgICAgPHVsIGNsYXNzTmFtZT1cIm5hdlwiPlxuICAgICAgICB7dXNlckljb25zfVxuICAgICAgICA8bGk+XG4gICAgICAgICAgPGJ1dHRvbiBvbkNsaWNrPXthY3Rpb25zLmNsb3NlfSBjbGFzc05hbWU9XCJidG4gYnRuLWRhbmdlciBidG4tY2lyY2xlXCIgdHlwZT1cImJ1dHRvblwiPlxuICAgICAgICAgICAgPGkgY2xhc3NOYW1lPVwiZmEgZmEtdGltZXNcIj48L2k+XG4gICAgICAgICAgPC9idXR0b24+XG4gICAgICAgIDwvbGk+XG4gICAgICA8L3VsPlxuICAgIDwvZGl2PlxuICApXG59O1xuXG5tb2R1bGUuZXhwb3J0cyA9IFNlc3Npb25MZWZ0UGFuZWw7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9jdXJyZW50U2Vzc2lvbi9zZXNzaW9uTGVmdFBhbmVsLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgJCA9IHJlcXVpcmUoJ2pRdWVyeScpO1xudmFyIG1vbWVudCA9IHJlcXVpcmUoJ21vbWVudCcpO1xudmFyIHtkZWJvdW5jZX0gPSByZXF1aXJlKCdfJyk7XG5cbnZhciBEYXRlUmFuZ2VQaWNrZXIgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgZ2V0RGF0ZXMoKXtcbiAgICB2YXIgc3RhcnREYXRlID0gJCh0aGlzLnJlZnMuZHBQaWNrZXIxKS5kYXRlcGlja2VyKCdnZXREYXRlJyk7XG4gICAgdmFyIGVuZERhdGUgPSAkKHRoaXMucmVmcy5kcFBpY2tlcjIpLmRhdGVwaWNrZXIoJ2dldERhdGUnKTtcbiAgICByZXR1cm4gW3N0YXJ0RGF0ZSwgZW5kRGF0ZV07XG4gIH0sXG5cbiAgc2V0RGF0ZXMoe3N0YXJ0RGF0ZSwgZW5kRGF0ZX0pe1xuICAgICQodGhpcy5yZWZzLmRwUGlja2VyMSkuZGF0ZXBpY2tlcignc2V0RGF0ZScsIHN0YXJ0RGF0ZSk7XG4gICAgJCh0aGlzLnJlZnMuZHBQaWNrZXIyKS5kYXRlcGlja2VyKCdzZXREYXRlJywgZW5kRGF0ZSk7XG4gIH0sXG5cbiAgZ2V0RGVmYXVsdFByb3BzKCkge1xuICAgICByZXR1cm4ge1xuICAgICAgIHN0YXJ0RGF0ZTogbW9tZW50KCkuc3RhcnRPZignbW9udGgnKS50b0RhdGUoKSxcbiAgICAgICBlbmREYXRlOiBtb21lbnQoKS5lbmRPZignbW9udGgnKS50b0RhdGUoKSxcbiAgICAgICBvbkNoYW5nZTogKCk9Pnt9XG4gICAgIH07XG4gICB9LFxuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50KCl7XG4gICAgJCh0aGlzLnJlZnMuZHApLmRhdGVwaWNrZXIoJ2Rlc3Ryb3knKTtcbiAgfSxcblxuICBjb21wb25lbnRXaWxsUmVjZWl2ZVByb3BzKG5ld1Byb3BzKXtcbiAgICB2YXIgW3N0YXJ0RGF0ZSwgZW5kRGF0ZV0gPSB0aGlzLmdldERhdGVzKCk7XG4gICAgaWYoIShpc1NhbWUoc3RhcnREYXRlLCBuZXdQcm9wcy5zdGFydERhdGUpICYmXG4gICAgICAgICAgaXNTYW1lKGVuZERhdGUsIG5ld1Byb3BzLmVuZERhdGUpKSl7XG4gICAgICAgIHRoaXMuc2V0RGF0ZXMobmV3UHJvcHMpO1xuICAgICAgfVxuICB9LFxuXG4gIHNob3VsZENvbXBvbmVudFVwZGF0ZSgpe1xuICAgIHJldHVybiBmYWxzZTtcbiAgfSxcblxuICBjb21wb25lbnREaWRNb3VudCgpe1xuICAgIHRoaXMub25DaGFuZ2UgPSBkZWJvdW5jZSh0aGlzLm9uQ2hhbmdlLCAxKTtcbiAgICAkKHRoaXMucmVmcy5yYW5nZVBpY2tlcikuZGF0ZXBpY2tlcih7XG4gICAgICB0b2RheUJ0bjogJ2xpbmtlZCcsXG4gICAgICBrZXlib2FyZE5hdmlnYXRpb246IGZhbHNlLFxuICAgICAgZm9yY2VQYXJzZTogZmFsc2UsXG4gICAgICBjYWxlbmRhcldlZWtzOiB0cnVlLFxuICAgICAgYXV0b2Nsb3NlOiB0cnVlXG4gICAgfSkub24oJ2NoYW5nZURhdGUnLCB0aGlzLm9uQ2hhbmdlKTtcblxuICAgIHRoaXMuc2V0RGF0ZXModGhpcy5wcm9wcyk7XG4gIH0sXG5cbiAgb25DaGFuZ2UoKXtcbiAgICB2YXIgW3N0YXJ0RGF0ZSwgZW5kRGF0ZV0gPSB0aGlzLmdldERhdGVzKClcbiAgICBpZighKGlzU2FtZShzdGFydERhdGUsIHRoaXMucHJvcHMuc3RhcnREYXRlKSAmJlxuICAgICAgICAgIGlzU2FtZShlbmREYXRlLCB0aGlzLnByb3BzLmVuZERhdGUpKSl7XG4gICAgICAgIHRoaXMucHJvcHMub25DaGFuZ2Uoe3N0YXJ0RGF0ZSwgZW5kRGF0ZX0pO1xuICAgIH1cbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWRhdGVwaWNrZXIgaW5wdXQtZ3JvdXAgaW5wdXQtZGF0ZXJhbmdlXCIgcmVmPVwicmFuZ2VQaWNrZXJcIj4gICAgICAgIFxuICAgICAgICA8aW5wdXQgcmVmPVwiZHBQaWNrZXIxXCIgdHlwZT1cInRleHRcIiBjbGFzc05hbWU9XCJpbnB1dC1zbSBmb3JtLWNvbnRyb2xcIiBuYW1lPVwic3RhcnRcIiAvPlxuICAgICAgICA8c3BhbiBjbGFzc05hbWU9XCJpbnB1dC1ncm91cC1hZGRvblwiPnRvPC9zcGFuPlxuICAgICAgICA8aW5wdXQgcmVmPVwiZHBQaWNrZXIyXCIgdHlwZT1cInRleHRcIiBjbGFzc05hbWU9XCJpbnB1dC1zbSBmb3JtLWNvbnRyb2xcIiBuYW1lPVwiZW5kXCIgLz5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pO1xuXG5mdW5jdGlvbiBpc1NhbWUoZGF0ZTEsIGRhdGUyKXtcbiAgcmV0dXJuIG1vbWVudChkYXRlMSkuaXNTYW1lKGRhdGUyLCAnZGF5Jyk7XG59XG5cbi8qKlxuKiBDYWxlbmRhciBOYXZcbiovXG52YXIgQ2FsZW5kYXJOYXYgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgcmVuZGVyKCkge1xuICAgIGxldCB7dmFsdWV9ID0gdGhpcy5wcm9wcztcbiAgICBsZXQgZGlzcGxheVZhbHVlID0gbW9tZW50KHZhbHVlKS5mb3JtYXQoJ01NTU0sIFlZWVknKTtcblxuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT17XCJncnYtY2FsZW5kYXItbmF2IFwiICsgdGhpcy5wcm9wcy5jbGFzc05hbWV9ID5cbiAgICAgICAgPGJ1dHRvbiBvbkNsaWNrPXt0aGlzLm1vdmUuYmluZCh0aGlzLCAtMSl9IGNsYXNzTmFtZT1cImJ0biBidG4tb3V0bGluZSBidG4tbGlua1wiPjxpIGNsYXNzTmFtZT1cImZhIGZhLWNoZXZyb24tbGVmdFwiPjwvaT48L2J1dHRvbj5cbiAgICAgICAgPHNwYW4gY2xhc3NOYW1lPVwidGV4dC1tdXRlZFwiPntkaXNwbGF5VmFsdWV9PC9zcGFuPlxuICAgICAgICA8YnV0dG9uIG9uQ2xpY2s9e3RoaXMubW92ZS5iaW5kKHRoaXMsIDEpfSBjbGFzc05hbWU9XCJidG4gYnRuLW91dGxpbmUgYnRuLWxpbmtcIj48aSBjbGFzc05hbWU9XCJmYSBmYS1jaGV2cm9uLXJpZ2h0XCI+PC9pPjwvYnV0dG9uPlxuICAgICAgPC9kaXY+XG4gICAgKTtcbiAgfSxcblxuICBtb3ZlKGF0KXtcbiAgICBsZXQge3ZhbHVlfSA9IHRoaXMucHJvcHM7XG4gICAgbGV0IG5ld1ZhbHVlID0gbW9tZW50KHZhbHVlKS5hZGQoYXQsICdtb250aCcpLnRvRGF0ZSgpO1xuICAgIHRoaXMucHJvcHMub25WYWx1ZUNoYW5nZShuZXdWYWx1ZSk7XG4gIH1cbn0pO1xuXG5DYWxlbmRhck5hdi5nZXRNb250aFJhbmdlID0gZnVuY3Rpb24odmFsdWUpe1xuICBsZXQgc3RhcnREYXRlID0gbW9tZW50KHZhbHVlKS5zdGFydE9mKCdtb250aCcpLnRvRGF0ZSgpO1xuICBsZXQgZW5kRGF0ZSA9IG1vbWVudCh2YWx1ZSkuZW5kT2YoJ21vbnRoJykudG9EYXRlKCk7XG4gIHJldHVybiBbc3RhcnREYXRlLCBlbmREYXRlXTtcbn1cblxuZXhwb3J0IGRlZmF1bHQgRGF0ZVJhbmdlUGlja2VyO1xuZXhwb3J0IHtDYWxlbmRhck5hdiwgRGF0ZVJhbmdlUGlja2VyfTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2RhdGVQaWNrZXIuanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcblxudmFyIE5vdEZvdW5kID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWVycm9yLXBhZ2VcIj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbG9nby10cHJ0XCI+VGVsZXBvcnQ8L2Rpdj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtd2FybmluZ1wiPjxpIGNsYXNzTmFtZT1cImZhIGZhLXdhcm5pbmdcIj48L2k+IDwvZGl2PlxuICAgICAgICA8aDE+V2hvb3BzLCB3ZSBjYW5ub3QgZmluZCB0aGF0PC9oMT5cbiAgICAgICAgPGRpdj5Mb29rcyBsaWtlIHRoZSBwYWdlIHlvdSBhcmUgbG9va2luZyBmb3IgaXNuJ3QgaGVyZSBhbnkgbG9uZ2VyPC9kaXY+XG4gICAgICAgIDxkaXY+SWYgeW91IGJlbGlldmUgdGhpcyBpcyBhbiBlcnJvciwgcGxlYXNlIGNvbnRhY3QgeW91ciBvcmdhbml6YXRpb24gYWRtaW5pc3RyYXRvci48L2Rpdj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJjb250YWN0LXNlY3Rpb25cIj5JZiB5b3UgYmVsaWV2ZSB0aGlzIGlzIGFuIGlzc3VlIHdpdGggVGVsZXBvcnQsIHBsZWFzZSA8YSBocmVmPVwiaHR0cHM6Ly9naXRodWIuY29tL2dyYXZpdGF0aW9uYWwvdGVsZXBvcnQvaXNzdWVzL25ld1wiPmNyZWF0ZSBhIEdpdEh1YiBpc3N1ZS48L2E+XG4gICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pXG5cbnZhciBFeHBpcmVkSW52aXRlID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWVycm9yLXBhZ2VcIj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbG9nby10cHJ0XCI+VGVsZXBvcnQ8L2Rpdj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtd2FybmluZ1wiPjxpIGNsYXNzTmFtZT1cImZhIGZhLXdhcm5pbmdcIj48L2k+IDwvZGl2PlxuICAgICAgICA8aDE+SW52aXRlIGNvZGUgaGFzIGV4cGlyZWQ8L2gxPlxuICAgICAgICA8ZGl2Pkxvb2tzIGxpa2UgeW91ciBpbnZpdGUgY29kZSBpc24ndCB2YWxpZCBhbnltb3JlPC9kaXY+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiY29udGFjdC1zZWN0aW9uXCI+SWYgeW91IGJlbGlldmUgdGhpcyBpcyBhbiBpc3N1ZSB3aXRoIFRlbGVwb3J0LCBwbGVhc2UgPGEgaHJlZj1cImh0dHBzOi8vZ2l0aHViLmNvbS9ncmF2aXRhdGlvbmFsL3RlbGVwb3J0L2lzc3Vlcy9uZXdcIj5jcmVhdGUgYSBHaXRIdWIgaXNzdWUuPC9hPlxuICAgICAgICAgPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KVxuXG5leHBvcnQgZGVmYXVsdCBOb3RGb3VuZDtcbmV4cG9ydCB7Tm90Rm91bmQsIEV4cGlyZWRJbnZpdGV9XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9lcnJvclBhZ2UuanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcblxudmFyIEdvb2dsZUF1dGhJbmZvID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWdvb2dsZS1hdXRoXCI+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWdvb2dsZS1hdXRoLWljb25cIj48L2Rpdj5cbiAgICAgICAgPHN0cm9uZz5Hb29nbGUgQXV0aGVudGljYXRvcjwvc3Ryb25nPlxuICAgICAgICA8ZGl2PkRvd25sb2FkIDxhIGhyZWY9XCJodHRwczovL3N1cHBvcnQuZ29vZ2xlLmNvbS9hY2NvdW50cy9hbnN3ZXIvMTA2NjQ0Nz9obD1lblwiPkdvb2dsZSBBdXRoZW50aWNhdG9yPC9hPiBvbiB5b3VyIHBob25lIHRvIGFjY2VzcyB5b3VyIHR3byBmYWN0b3J5IHRva2VuPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KVxuXG5tb2R1bGUuZXhwb3J0cyA9IEdvb2dsZUF1dGhJbmZvO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvZ29vZ2xlQXV0aExvZ28uanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7Z2V0dGVycywgYWN0aW9uc30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9ub2RlcycpO1xudmFyIHVzZXJHZXR0ZXJzID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvdXNlci9nZXR0ZXJzJyk7XG52YXIge1RhYmxlLCBDb2x1bW4sIENlbGwsIFNvcnRIZWFkZXJDZWxsLCBTb3J0VHlwZXN9ID0gcmVxdWlyZSgnYXBwL2NvbXBvbmVudHMvdGFibGUuanN4Jyk7XG52YXIge2NyZWF0ZU5ld1Nlc3Npb259ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvYWN0aW9ucycpO1xudmFyIExpbmtlZFN0YXRlTWl4aW4gPSByZXF1aXJlKCdyZWFjdC1hZGRvbnMtbGlua2VkLXN0YXRlLW1peGluJyk7XG52YXIgXyA9IHJlcXVpcmUoJ18nKTtcbnZhciB7aXNNYXRjaH0gPSByZXF1aXJlKCdhcHAvY29tbW9uL29iamVjdFV0aWxzJyk7XG5cbmNvbnN0IFRleHRDZWxsID0gKHtyb3dJbmRleCwgZGF0YSwgY29sdW1uS2V5LCAuLi5wcm9wc30pID0+IChcbiAgPENlbGwgey4uLnByb3BzfT5cbiAgICB7ZGF0YVtyb3dJbmRleF1bY29sdW1uS2V5XX1cbiAgPC9DZWxsPlxuKTtcblxuY29uc3QgVGFnQ2VsbCA9ICh7cm93SW5kZXgsIGRhdGEsIGNvbHVtbktleSwgLi4ucHJvcHN9KSA9PiAoXG4gIDxDZWxsIHsuLi5wcm9wc30+XG4gICAgeyBkYXRhW3Jvd0luZGV4XS50YWdzLm1hcCgoaXRlbSwgaW5kZXgpID0+XG4gICAgICAoPHNwYW4ga2V5PXtpbmRleH0gY2xhc3NOYW1lPVwibGFiZWwgbGFiZWwtZGVmYXVsdFwiPlxuICAgICAgICB7aXRlbS5yb2xlfSA8bGkgY2xhc3NOYW1lPVwiZmEgZmEtbG9uZy1hcnJvdy1yaWdodFwiPjwvbGk+XG4gICAgICAgIHtpdGVtLnZhbHVlfVxuICAgICAgPC9zcGFuPilcbiAgICApIH1cbiAgPC9DZWxsPlxuKTtcblxuY29uc3QgTG9naW5DZWxsID0gKHtsb2dpbnMsIG9uTG9naW5DbGljaywgcm93SW5kZXgsIGRhdGEsIC4uLnByb3BzfSkgPT4ge1xuICBpZighbG9naW5zIHx8bG9naW5zLmxlbmd0aCA9PT0gMCl7XG4gICAgcmV0dXJuIDxDZWxsIHsuLi5wcm9wc30gLz47XG4gIH1cblxuICB2YXIgc2VydmVySWQgPSBkYXRhW3Jvd0luZGV4XS5pZDtcbiAgdmFyICRsaXMgPSBbXTtcblxuICBmdW5jdGlvbiBvbkNsaWNrKGkpe1xuICAgIHZhciBsb2dpbiA9IGxvZ2luc1tpXTtcbiAgICBpZihvbkxvZ2luQ2xpY2spe1xuICAgICAgcmV0dXJuICgpPT4gb25Mb2dpbkNsaWNrKHNlcnZlcklkLCBsb2dpbik7XG4gICAgfWVsc2V7XG4gICAgICByZXR1cm4gKCkgPT4gY3JlYXRlTmV3U2Vzc2lvbihzZXJ2ZXJJZCwgbG9naW4pO1xuICAgIH1cbiAgfVxuXG4gIGZvcih2YXIgaSA9IDA7IGkgPCBsb2dpbnMubGVuZ3RoOyBpKyspe1xuICAgICRsaXMucHVzaCg8bGkga2V5PXtpfT48YSBvbkNsaWNrPXtvbkNsaWNrKGkpfT57bG9naW5zW2ldfTwvYT48L2xpPik7XG4gIH1cblxuICByZXR1cm4gKFxuICAgIDxDZWxsIHsuLi5wcm9wc30+XG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImJ0bi1ncm91cFwiPlxuICAgICAgICA8YnV0dG9uIHR5cGU9XCJidXR0b25cIiBvbkNsaWNrPXtvbkNsaWNrKDApfSBjbGFzc05hbWU9XCJidG4gYnRuLXhzIGJ0bi1wcmltYXJ5XCI+e2xvZ2luc1swXX08L2J1dHRvbj5cbiAgICAgICAge1xuICAgICAgICAgICRsaXMubGVuZ3RoID4gMSA/IChcbiAgICAgICAgICAgICAgW1xuICAgICAgICAgICAgICAgIDxidXR0b24ga2V5PXswfSBkYXRhLXRvZ2dsZT1cImRyb3Bkb3duXCIgY2xhc3NOYW1lPVwiYnRuIGJ0bi1kZWZhdWx0IGJ0bi14cyBkcm9wZG93bi10b2dnbGVcIiBhcmlhLWV4cGFuZGVkPVwidHJ1ZVwiPlxuICAgICAgICAgICAgICAgICAgPHNwYW4gY2xhc3NOYW1lPVwiY2FyZXRcIj48L3NwYW4+XG4gICAgICAgICAgICAgICAgPC9idXR0b24+LFxuICAgICAgICAgICAgICAgIDx1bCBrZXk9ezF9IGNsYXNzTmFtZT1cImRyb3Bkb3duLW1lbnVcIj5cbiAgICAgICAgICAgICAgICAgIHskbGlzfVxuICAgICAgICAgICAgICAgIDwvdWw+XG4gICAgICAgICAgICAgIF0gKVxuICAgICAgICAgICAgOiBudWxsXG4gICAgICAgIH1cbiAgICAgIDwvZGl2PlxuICAgIDwvQ2VsbD5cbiAgKVxufTtcblxudmFyIE5vZGVMaXN0ID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW0xpbmtlZFN0YXRlTWl4aW5dLFxuXG4gIGdldEluaXRpYWxTdGF0ZShwcm9wcyl7XG4gICAgdGhpcy5zZWFyY2hhYmxlUHJvcHMgPSBbJ2FkZHInLCAnaG9zdG5hbWUnXTtcbiAgICByZXR1cm4geyBmaWx0ZXI6ICcnLCBjb2xTb3J0RGlyczoge2hvc3RuYW1lOiAnREVTQyd9IH07XG4gIH0sXG5cbiAgb25Tb3J0Q2hhbmdlKGNvbHVtbktleSwgc29ydERpcikge1xuICAgIHRoaXMuc2V0U3RhdGUoe1xuICAgICAgLi4udGhpcy5zdGF0ZSxcbiAgICAgIGNvbFNvcnREaXJzOiB7XG4gICAgICAgIFtjb2x1bW5LZXldOiBzb3J0RGlyXG4gICAgICB9XG4gICAgfSk7XG4gIH0sXG5cbiAgc29ydEFuZEZpbHRlcihkYXRhKXtcbiAgICB2YXIgZmlsdGVyZWQgPSBkYXRhLmZpbHRlcihvYmo9PlxuICAgICAgaXNNYXRjaChvYmosIHRoaXMuc3RhdGUuZmlsdGVyLCB7IHNlYXJjaGFibGVQcm9wczogdGhpcy5zZWFyY2hhYmxlUHJvcHN9KSk7XG5cbiAgICB2YXIgY29sdW1uS2V5ID0gT2JqZWN0LmdldE93blByb3BlcnR5TmFtZXModGhpcy5zdGF0ZS5jb2xTb3J0RGlycylbMF07XG4gICAgdmFyIHNvcnREaXIgPSB0aGlzLnN0YXRlLmNvbFNvcnREaXJzW2NvbHVtbktleV07XG4gICAgdmFyIHNvcnRlZCA9IF8uc29ydEJ5KGZpbHRlcmVkLCBjb2x1bW5LZXkpO1xuICAgIGlmKHNvcnREaXIgPT09IFNvcnRUeXBlcy5BU0Mpe1xuICAgICAgc29ydGVkID0gc29ydGVkLnJldmVyc2UoKTtcbiAgICB9XG5cbiAgICByZXR1cm4gc29ydGVkO1xuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgdmFyIGRhdGEgPSB0aGlzLnNvcnRBbmRGaWx0ZXIodGhpcy5wcm9wcy5ub2RlUmVjb3Jkcyk7XG4gICAgdmFyIGxvZ2lucyA9IHRoaXMucHJvcHMubG9naW5zO1xuICAgIHZhciBvbkxvZ2luQ2xpY2sgPSB0aGlzLnByb3BzLm9uTG9naW5DbGljaztcblxuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1ub2RlcyBncnYtcGFnZVwiPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4IGdydi1oZWFkZXJcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LWNvbHVtblwiPjwvZGl2PlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWZsZXgtY29sdW1uXCI+XG4gICAgICAgICAgICA8aDE+IE5vZGVzIDwvaDE+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZmxleC1jb2x1bW5cIj5cbiAgICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXNlYXJjaFwiPlxuICAgICAgICAgICAgICA8aW5wdXQgdmFsdWVMaW5rPXt0aGlzLmxpbmtTdGF0ZSgnZmlsdGVyJyl9IHBsYWNlaG9sZGVyPVwiU2VhcmNoLi4uXCIgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sIGlucHV0LXNtXCIvPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgIDwvZGl2PlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgIDxUYWJsZSByb3dDb3VudD17ZGF0YS5sZW5ndGh9IGNsYXNzTmFtZT1cInRhYmxlLXN0cmlwZWQgZ3J2LW5vZGVzLXRhYmxlXCI+XG4gICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgIGNvbHVtbktleT1cImhvc3RuYW1lXCJcbiAgICAgICAgICAgICAgaGVhZGVyPXtcbiAgICAgICAgICAgICAgICA8U29ydEhlYWRlckNlbGxcbiAgICAgICAgICAgICAgICAgIHNvcnREaXI9e3RoaXMuc3RhdGUuY29sU29ydERpcnMuaG9zdG5hbWV9XG4gICAgICAgICAgICAgICAgICBvblNvcnRDaGFuZ2U9e3RoaXMub25Tb3J0Q2hhbmdlfVxuICAgICAgICAgICAgICAgICAgdGl0bGU9XCJOb2RlXCJcbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICB9XG4gICAgICAgICAgICAgIGNlbGw9ezxUZXh0Q2VsbCBkYXRhPXtkYXRhfS8+IH1cbiAgICAgICAgICAgIC8+XG4gICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgIGNvbHVtbktleT1cImFkZHJcIlxuICAgICAgICAgICAgICBoZWFkZXI9e1xuICAgICAgICAgICAgICAgIDxTb3J0SGVhZGVyQ2VsbFxuICAgICAgICAgICAgICAgICAgc29ydERpcj17dGhpcy5zdGF0ZS5jb2xTb3J0RGlycy5hZGRyfVxuICAgICAgICAgICAgICAgICAgb25Tb3J0Q2hhbmdlPXt0aGlzLm9uU29ydENoYW5nZX1cbiAgICAgICAgICAgICAgICAgIHRpdGxlPVwiSVBcIlxuICAgICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgIH1cblxuICAgICAgICAgICAgICBjZWxsPXs8VGV4dENlbGwgZGF0YT17ZGF0YX0vPiB9XG4gICAgICAgICAgICAvPlxuICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJ0YWdzXCJcbiAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD48L0NlbGw+IH1cbiAgICAgICAgICAgICAgY2VsbD17PFRhZ0NlbGwgZGF0YT17ZGF0YX0vPiB9XG4gICAgICAgICAgICAvPlxuICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJyb2xlc1wiXG4gICAgICAgICAgICAgIG9uTG9naW5DbGljaz17b25Mb2dpbkNsaWNrfVxuICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPkxvZ2luIGFzPC9DZWxsPiB9XG4gICAgICAgICAgICAgIGNlbGw9ezxMb2dpbkNlbGwgZGF0YT17ZGF0YX0gbG9naW5zPXtsb2dpbnN9Lz4gfVxuICAgICAgICAgICAgLz5cbiAgICAgICAgICA8L1RhYmxlPlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgIClcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gTm9kZUxpc3Q7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9ub2Rlcy9ub2RlTGlzdC5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHtnZXR0ZXJzfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2RpYWxvZ3MnKTtcbnZhciB7Y2xvc2VTZWxlY3ROb2RlRGlhbG9nfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2RpYWxvZ3MvYWN0aW9ucycpO1xudmFyIHtjaGFuZ2VTZXJ2ZXJ9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvYWN0aW9ucycpO1xudmFyIE5vZGVMaXN0ID0gcmVxdWlyZSgnLi9ub2Rlcy9ub2RlTGlzdC5qc3gnKTtcbnZhciBhY3RpdmVTZXNzaW9uR2V0dGVycyA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsL2dldHRlcnMnKTtcbnZhciBub2RlR2V0dGVycyA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL25vZGVzL2dldHRlcnMnKTtcblxudmFyIFNlbGVjdE5vZGVEaWFsb2cgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIGRpYWxvZ3M6IGdldHRlcnMuZGlhbG9nc1xuICAgIH1cbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuIHRoaXMuc3RhdGUuZGlhbG9ncy5pc1NlbGVjdE5vZGVEaWFsb2dPcGVuID8gPERpYWxvZy8+IDogbnVsbDtcbiAgfVxufSk7XG5cbnZhciBEaWFsb2cgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgb25Mb2dpbkNsaWNrKHNlcnZlcklkLCBsb2dpbil7XG4gICAgaWYoU2VsZWN0Tm9kZURpYWxvZy5vblNlcnZlckNoYW5nZUNhbGxCYWNrKXtcbiAgICAgIFNlbGVjdE5vZGVEaWFsb2cub25TZXJ2ZXJDaGFuZ2VDYWxsQmFjayh7c2VydmVySWR9KTtcbiAgICB9XG5cbiAgICBjbG9zZVNlbGVjdE5vZGVEaWFsb2coKTtcbiAgfSxcblxuICBjb21wb25lbnRXaWxsVW5tb3VudChjYWxsYmFjayl7XG4gICAgJCgnLm1vZGFsJykubW9kYWwoJ2hpZGUnKTtcbiAgfSxcblxuICBjb21wb25lbnREaWRNb3VudCgpe1xuICAgICQoJy5tb2RhbCcpLm1vZGFsKCdzaG93Jyk7XG4gIH0sXG5cbiAgcmVuZGVyKCkge1xuICAgIHZhciBhY3RpdmVTZXNzaW9uID0gcmVhY3Rvci5ldmFsdWF0ZShhY3RpdmVTZXNzaW9uR2V0dGVycy5hY3RpdmVTZXNzaW9uKSB8fCB7fTtcbiAgICB2YXIgbm9kZVJlY29yZHMgPSByZWFjdG9yLmV2YWx1YXRlKG5vZGVHZXR0ZXJzLm5vZGVMaXN0Vmlldyk7XG4gICAgdmFyIGxvZ2lucyA9IFthY3RpdmVTZXNzaW9uLmxvZ2luXTtcblxuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cIm1vZGFsIGZhZGUgZ3J2LWRpYWxvZy1zZWxlY3Qtbm9kZVwiIHRhYkluZGV4PXstMX0gcm9sZT1cImRpYWxvZ1wiPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIm1vZGFsLWRpYWxvZ1wiPlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwibW9kYWwtY29udGVudFwiPlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJtb2RhbC1oZWFkZXJcIj5cbiAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJtb2RhbC1ib2R5XCI+XG4gICAgICAgICAgICAgIDxOb2RlTGlzdCBub2RlUmVjb3Jkcz17bm9kZVJlY29yZHN9IGxvZ2lucz17bG9naW5zfSBvbkxvZ2luQ2xpY2s9e3RoaXMub25Mb2dpbkNsaWNrfS8+XG4gICAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwibW9kYWwtZm9vdGVyXCI+XG4gICAgICAgICAgICAgIDxidXR0b24gb25DbGljaz17Y2xvc2VTZWxlY3ROb2RlRGlhbG9nfSB0eXBlPVwiYnV0dG9uXCIgY2xhc3NOYW1lPVwiYnRuIGJ0bi1wcmltYXJ5XCI+XG4gICAgICAgICAgICAgICAgQ2xvc2VcbiAgICAgICAgICAgICAgPC9idXR0b24+XG4gICAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KTtcblxuU2VsZWN0Tm9kZURpYWxvZy5vblNlcnZlckNoYW5nZUNhbGxCYWNrID0gKCk9Pnt9O1xuXG5tb2R1bGUuZXhwb3J0cyA9IFNlbGVjdE5vZGVEaWFsb2c7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9zZWxlY3ROb2RlRGlhbG9nLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgeyBMaW5rIH0gPSByZXF1aXJlKCdyZWFjdC1yb3V0ZXInKTtcbnZhciB7YWN0aW9uc30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9zZXNzaW9ucycpO1xudmFyIHtub2RlSG9zdE5hbWVCeVNlcnZlcklkfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL25vZGVzL2dldHRlcnMnKTtcbnZhciB7Q2VsbCwgVGV4dENlbGx9ID0gcmVxdWlyZSgnYXBwL2NvbXBvbmVudHMvdGFibGUuanN4Jyk7XG52YXIgbW9tZW50ID0gIHJlcXVpcmUoJ21vbWVudCcpO1xuXG5jb25zdCBEYXRlQ3JlYXRlZENlbGwgPSAoeyByb3dJbmRleCwgZGF0YSwgLi4ucHJvcHMgfSkgPT4ge1xuICBsZXQgY3JlYXRlZCA9IGRhdGFbcm93SW5kZXhdLmNyZWF0ZWQ7XG4gIGxldCBkaXNwbGF5RGF0ZSA9IG1vbWVudChjcmVhdGVkKS5mb3JtYXQoJ2wgTFRTJyk7XG4gIHJldHVybiAoXG4gICAgPENlbGwgey4uLnByb3BzfT5cbiAgICAgIHsgZGlzcGxheURhdGUgfVxuICAgIDwvQ2VsbD5cbiAgKVxufTtcblxuY29uc3QgRHVyYXRpb25DZWxsID0gKHsgcm93SW5kZXgsIGRhdGEsIC4uLnByb3BzIH0pID0+IHtcbiAgbGV0IGNyZWF0ZWQgPSBkYXRhW3Jvd0luZGV4XS5jcmVhdGVkO1xuICBsZXQgbGFzdEFjdGl2ZSA9IGRhdGFbcm93SW5kZXhdLmxhc3RBY3RpdmU7XG5cbiAgbGV0IGVuZCA9IG1vbWVudChjcmVhdGVkKTtcbiAgbGV0IG5vdyA9IG1vbWVudChsYXN0QWN0aXZlKTtcbiAgbGV0IGR1cmF0aW9uID0gbW9tZW50LmR1cmF0aW9uKG5vdy5kaWZmKGVuZCkpO1xuICBsZXQgZGlzcGxheURhdGUgPSBkdXJhdGlvbi5odW1hbml6ZSgpO1xuXG4gIHJldHVybiAoXG4gICAgPENlbGwgey4uLnByb3BzfT5cbiAgICAgIHsgZGlzcGxheURhdGUgfVxuICAgIDwvQ2VsbD5cbiAgKVxufTtcblxuY29uc3QgU2luZ2xlVXNlckNlbGwgPSAoeyByb3dJbmRleCwgZGF0YSwgLi4ucHJvcHMgfSkgPT4ge1xuICByZXR1cm4gKFxuICAgIDxDZWxsIHsuLi5wcm9wc30+XG4gICAgICA8c3BhbiBjbGFzc05hbWU9XCJncnYtc2Vzc2lvbnMtdXNlciBsYWJlbCBsYWJlbC1kZWZhdWx0XCI+e2RhdGFbcm93SW5kZXhdLmxvZ2lufTwvc3Bhbj5cbiAgICA8L0NlbGw+XG4gIClcbn07XG5cbmNvbnN0IFVzZXJzQ2VsbCA9ICh7IHJvd0luZGV4LCBkYXRhLCAuLi5wcm9wcyB9KSA9PiB7XG4gIGxldCAkdXNlcnMgPSBkYXRhW3Jvd0luZGV4XS5wYXJ0aWVzLm1hcCgoaXRlbSwgaXRlbUluZGV4KT0+XG4gICAgKDxzcGFuIGtleT17aXRlbUluZGV4fSBjbGFzc05hbWU9XCJncnYtc2Vzc2lvbnMtdXNlciBsYWJlbCBsYWJlbC1kZWZhdWx0XCI+e2l0ZW0udXNlcn08L3NwYW4+KVxuICApXG5cbiAgcmV0dXJuIChcbiAgICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgICAgPGRpdj5cbiAgICAgICAgeyR1c2Vyc31cbiAgICAgIDwvZGl2PlxuICAgIDwvQ2VsbD5cbiAgKVxufTtcblxuY29uc3QgQnV0dG9uQ2VsbCA9ICh7IHJvd0luZGV4LCBkYXRhLCAuLi5wcm9wcyB9KSA9PiB7XG4gIGxldCB7IHNlc3Npb25VcmwsIGFjdGl2ZSB9ID0gZGF0YVtyb3dJbmRleF07XG4gIGxldCBbYWN0aW9uVGV4dCwgYWN0aW9uQ2xhc3NdID0gYWN0aXZlID8gWydqb2luJywgJ2J0bi13YXJuaW5nJ10gOiBbJ3BsYXknLCAnYnRuLXByaW1hcnknXTtcbiAgcmV0dXJuIChcbiAgICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgICAgPExpbmsgdG89e3Nlc3Npb25Vcmx9IGNsYXNzTmFtZT17XCJidG4gXCIgK2FjdGlvbkNsYXNzKyBcIiBidG4teHNcIn0gdHlwZT1cImJ1dHRvblwiPnthY3Rpb25UZXh0fTwvTGluaz5cbiAgICA8L0NlbGw+XG4gIClcbn1cblxuY29uc3QgRW1wdHlMaXN0ID0gKHt0ZXh0fSkgPT4gKFxuICA8ZGl2IGNsYXNzTmFtZT1cImdydi1zZXNzaW9ucy1lbXB0eSB0ZXh0LWNlbnRlciB0ZXh0LW11dGVkXCI+PHNwYW4+e3RleHR9PC9zcGFuPjwvZGl2PlxuKVxuXG5jb25zdCBOb2RlQ2VsbCA9ICh7IHJvd0luZGV4LCBkYXRhLCAuLi5wcm9wcyB9KSA9PiB7XG4gIGxldCB7c2VydmVySWR9ID0gZGF0YVtyb3dJbmRleF07XG4gIGxldCBob3N0bmFtZSA9IHJlYWN0b3IuZXZhbHVhdGUobm9kZUhvc3ROYW1lQnlTZXJ2ZXJJZChzZXJ2ZXJJZCkpIHx8ICd1bmtub3duJztcblxuICByZXR1cm4gKFxuICAgIDxDZWxsIHsuLi5wcm9wc30+XG4gICAgICB7aG9zdG5hbWV9XG4gICAgPC9DZWxsPlxuICApXG59XG5cbmV4cG9ydCBkZWZhdWx0IEJ1dHRvbkNlbGw7XG5cbmV4cG9ydCB7XG4gIEJ1dHRvbkNlbGwsXG4gIFVzZXJzQ2VsbCxcbiAgRHVyYXRpb25DZWxsLFxuICBEYXRlQ3JlYXRlZENlbGwsXG4gIEVtcHR5TGlzdCxcbiAgU2luZ2xlVXNlckNlbGwsXG4gIE5vZGVDZWxsXG59O1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvc2Vzc2lvbnMvbGlzdEl0ZW1zLmpzeFxuICoqLyIsInZhciBUZXJtID0gcmVxdWlyZSgnVGVybWluYWwnKTtcbnZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIge2RlYm91bmNlLCBpc051bWJlcn0gPSByZXF1aXJlKCdfJyk7XG5cblRlcm0uY29sb3JzWzI1Nl0gPSAnIzI1MjMyMyc7XG5cbmNvbnN0IERJU0NPTk5FQ1RfVFhUID0gJ1xceDFiWzMxbWRpc2Nvbm5lY3RlZFxceDFiW21cXHJcXG4nO1xuY29uc3QgQ09OTkVDVEVEX1RYVCA9ICdDb25uZWN0ZWQhXFxyXFxuJztcblxudmFyIFR0eVRlcm1pbmFsID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIGdldEluaXRpYWxTdGF0ZSgpe1xuICAgIHRoaXMucm93cyA9IHRoaXMucHJvcHMucm93cztcbiAgICB0aGlzLmNvbHMgPSB0aGlzLnByb3BzLmNvbHM7XG4gICAgdGhpcy50dHkgPSB0aGlzLnByb3BzLnR0eTtcblxuICAgIHRoaXMuZGVib3VuY2VkUmVzaXplID0gZGVib3VuY2UoKCk9PntcbiAgICAgIHRoaXMucmVzaXplKCk7XG4gICAgICB0aGlzLnR0eS5yZXNpemUodGhpcy5jb2xzLCB0aGlzLnJvd3MpO1xuICAgIH0sIDIwMCk7XG5cbiAgICByZXR1cm4ge307XG4gIH0sXG5cbiAgY29tcG9uZW50RGlkTW91bnQ6IGZ1bmN0aW9uKCkge1xuICAgIHRoaXMudGVybSA9IG5ldyBUZXJtaW5hbCh7XG4gICAgICBjb2xzOiA1LFxuICAgICAgcm93czogNSxcbiAgICAgIHVzZVN0eWxlOiB0cnVlLFxuICAgICAgc2NyZWVuS2V5czogdHJ1ZSxcbiAgICAgIGN1cnNvckJsaW5rOiB0cnVlXG4gICAgfSk7XG5cbiAgICB0aGlzLnRlcm0ub3Blbih0aGlzLnJlZnMuY29udGFpbmVyKTtcbiAgICB0aGlzLnRlcm0ub24oJ2RhdGEnLCAoZGF0YSkgPT4gdGhpcy50dHkuc2VuZChkYXRhKSk7XG5cbiAgICB0aGlzLnJlc2l6ZSh0aGlzLmNvbHMsIHRoaXMucm93cyk7XG5cbiAgICB0aGlzLnR0eS5vbignb3BlbicsICgpPT4gdGhpcy50ZXJtLndyaXRlKENPTk5FQ1RFRF9UWFQpKTtcbiAgICB0aGlzLnR0eS5vbignZGF0YScsIChkYXRhKSA9PiB0aGlzLnRlcm0ud3JpdGUoZGF0YSkpO1xuICAgIHRoaXMudHR5Lm9uKCdyZXNldCcsICgpPT4gdGhpcy50ZXJtLnJlc2V0KCkpO1xuXG4gICAgdGhpcy50dHkuY29ubmVjdCh7Y29sczogdGhpcy5jb2xzLCByb3dzOiB0aGlzLnJvd3N9KTtcbiAgICB3aW5kb3cuYWRkRXZlbnRMaXN0ZW5lcigncmVzaXplJywgdGhpcy5kZWJvdW5jZWRSZXNpemUpO1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50OiBmdW5jdGlvbigpIHtcbiAgICB0aGlzLnRlcm0uZGVzdHJveSgpO1xuICAgIHdpbmRvdy5yZW1vdmVFdmVudExpc3RlbmVyKCdyZXNpemUnLCB0aGlzLmRlYm91bmNlZFJlc2l6ZSk7XG4gIH0sXG5cbiAgc2hvdWxkQ29tcG9uZW50VXBkYXRlOiBmdW5jdGlvbihuZXdQcm9wcykge1xuICAgIHZhciB7cm93cywgY29sc30gPSBuZXdQcm9wcztcblxuICAgIGlmKCAhaXNOdW1iZXIocm93cykgfHwgIWlzTnVtYmVyKGNvbHMpKXtcbiAgICAgIHJldHVybiBmYWxzZTtcbiAgICB9XG5cbiAgICBpZihyb3dzICE9PSB0aGlzLnJvd3MgfHwgY29scyAhPT0gdGhpcy5jb2xzKXtcbiAgICAgIHRoaXMucmVzaXplKGNvbHMsIHJvd3MpXG4gICAgfVxuXG4gICAgcmV0dXJuIGZhbHNlO1xuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICByZXR1cm4gKCA8ZGl2IGNsYXNzTmFtZT1cImdydi10ZXJtaW5hbFwiIGlkPVwidGVybWluYWwtYm94XCIgcmVmPVwiY29udGFpbmVyXCI+ICA8L2Rpdj4gKTtcbiAgfSxcblxuICByZXNpemU6IGZ1bmN0aW9uKGNvbHMsIHJvd3MpIHtcbiAgICAvLyBpZiBub3QgZGVmaW5lZCwgdXNlIHRoZSBzaXplIG9mIHRoZSBjb250YWluZXJcbiAgICBpZighaXNOdW1iZXIoY29scykgfHwgIWlzTnVtYmVyKHJvd3MpKXtcbiAgICAgIGxldCBkaW0gPSB0aGlzLl9nZXREaW1lbnNpb25zKCk7XG4gICAgICBjb2xzID0gZGltLmNvbHM7XG4gICAgICByb3dzID0gZGltLnJvd3M7XG4gICAgfVxuXG4gICAgdGhpcy5jb2xzID0gY29scztcbiAgICB0aGlzLnJvd3MgPSByb3dzO1xuXG4gICAgdGhpcy50ZXJtLnJlc2l6ZSh0aGlzLmNvbHMsIHRoaXMucm93cyk7XG4gIH0sXG5cbiAgX2dldERpbWVuc2lvbnMoKXtcbiAgICBsZXQgJGNvbnRhaW5lciA9ICQodGhpcy5yZWZzLmNvbnRhaW5lcik7XG4gICAgbGV0IGZha2VSb3cgPSAkKCc8ZGl2PjxzcGFuPiZuYnNwOzwvc3Bhbj48L2Rpdj4nKTtcblxuICAgICRjb250YWluZXIuZmluZCgnLnRlcm1pbmFsJykuYXBwZW5kKGZha2VSb3cpO1xuICAgIC8vIGdldCBkaXYgaGVpZ2h0XG4gICAgbGV0IGZha2VDb2xIZWlnaHQgPSBmYWtlUm93WzBdLmdldEJvdW5kaW5nQ2xpZW50UmVjdCgpLmhlaWdodDtcbiAgICAvLyBnZXQgc3BhbiB3aWR0aFxuICAgIGxldCBmYWtlQ29sV2lkdGggPSBmYWtlUm93LmNoaWxkcmVuKCkuZmlyc3QoKVswXS5nZXRCb3VuZGluZ0NsaWVudFJlY3QoKS53aWR0aDtcblxuICAgIGxldCB3aWR0aCA9ICRjb250YWluZXJbMF0uY2xpZW50V2lkdGg7XG4gICAgbGV0IGhlaWdodCA9ICRjb250YWluZXJbMF0uY2xpZW50SGVpZ2h0O1xuXG4gICAgbGV0IGNvbHMgPSBNYXRoLmZsb29yKHdpZHRoIC8gKGZha2VDb2xXaWR0aCkpO1xuICAgIGxldCByb3dzID0gTWF0aC5mbG9vcihoZWlnaHQgLyAoZmFrZUNvbEhlaWdodCkpO1xuICAgIGZha2VSb3cucmVtb3ZlKCk7XG5cbiAgICByZXR1cm4ge2NvbHMsIHJvd3N9O1xuICB9XG5cbn0pO1xuXG5UdHlUZXJtaW5hbC5wcm9wVHlwZXMgPSB7XG4gIHR0eTogUmVhY3QuUHJvcFR5cGVzLm9iamVjdC5pc1JlcXVpcmVkXG59XG5cbm1vZHVsZS5leHBvcnRzID0gVHR5VGVybWluYWw7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy90ZXJtaW5hbC5qc3hcbiAqKi8iLCIvKlxuICogIFRoZSBNSVQgTGljZW5zZSAoTUlUKVxuICogIENvcHlyaWdodCAoYykgMjAxNSBSeWFuIEZsb3JlbmNlLCBNaWNoYWVsIEphY2tzb25cbiAqICBQZXJtaXNzaW9uIGlzIGhlcmVieSBncmFudGVkLCBmcmVlIG9mIGNoYXJnZSwgdG8gYW55IHBlcnNvbiBvYnRhaW5pbmcgYSBjb3B5IG9mIHRoaXMgc29mdHdhcmUgYW5kIGFzc29jaWF0ZWQgZG9jdW1lbnRhdGlvbiBmaWxlcyAodGhlIFwiU29mdHdhcmVcIiksIHRvIGRlYWwgaW4gdGhlIFNvZnR3YXJlIHdpdGhvdXQgcmVzdHJpY3Rpb24sIGluY2x1ZGluZyB3aXRob3V0IGxpbWl0YXRpb24gdGhlIHJpZ2h0cyB0byB1c2UsIGNvcHksIG1vZGlmeSwgbWVyZ2UsIHB1Ymxpc2gsIGRpc3RyaWJ1dGUsIHN1YmxpY2Vuc2UsIGFuZC9vciBzZWxsIGNvcGllcyBvZiB0aGUgU29mdHdhcmUsIGFuZCB0byBwZXJtaXQgcGVyc29ucyB0byB3aG9tIHRoZSBTb2Z0d2FyZSBpcyBmdXJuaXNoZWQgdG8gZG8gc28sIHN1YmplY3QgdG8gdGhlIGZvbGxvd2luZyBjb25kaXRpb25zOlxuICogIFRoZSBhYm92ZSBjb3B5cmlnaHQgbm90aWNlIGFuZCB0aGlzIHBlcm1pc3Npb24gbm90aWNlIHNoYWxsIGJlIGluY2x1ZGVkIGluIGFsbCBjb3BpZXMgb3Igc3Vic3RhbnRpYWwgcG9ydGlvbnMgb2YgdGhlIFNvZnR3YXJlLlxuICogIFRIRSBTT0ZUV0FSRSBJUyBQUk9WSURFRCBcIkFTIElTXCIsIFdJVEhPVVQgV0FSUkFOVFkgT0YgQU5ZIEtJTkQsIEVYUFJFU1MgT1IgSU1QTElFRCwgSU5DTFVESU5HIEJVVCBOT1QgTElNSVRFRCBUTyBUSEUgV0FSUkFOVElFUyBPRiBNRVJDSEFOVEFCSUxJVFksIEZJVE5FU1MgRk9SIEEgUEFSVElDVUxBUiBQVVJQT1NFIEFORCBOT05JTkZSSU5HRU1FTlQuIElOIE5PIEVWRU5UIFNIQUxMIFRIRSBBVVRIT1JTIE9SIENPUFlSSUdIVCBIT0xERVJTIEJFIExJQUJMRSBGT1IgQU5ZIENMQUlNLCBEQU1BR0VTIE9SIE9USEVSIExJQUJJTElUWSwgV0hFVEhFUiBJTiBBTiBBQ1RJT04gT0YgQ09OVFJBQ1QsIFRPUlQgT1IgT1RIRVJXSVNFLCBBUklTSU5HIEZST00sIE9VVCBPRiBPUiBJTiBDT05ORUNUSU9OIFdJVEggVEhFIFNPRlRXQVJFIE9SIFRIRSBVU0UgT1IgT1RIRVIgREVBTElOR1MgSU4gVEhFIFNPRlRXQVJFLlxuKi9cblxuaW1wb3J0IGludmFyaWFudCBmcm9tICdpbnZhcmlhbnQnXG5cbmZ1bmN0aW9uIGVzY2FwZVJlZ0V4cChzdHJpbmcpIHtcbiAgcmV0dXJuIHN0cmluZy5yZXBsYWNlKC9bLiorP14ke30oKXxbXFxdXFxcXF0vZywgJ1xcXFwkJicpXG59XG5cbmZ1bmN0aW9uIGVzY2FwZVNvdXJjZShzdHJpbmcpIHtcbiAgcmV0dXJuIGVzY2FwZVJlZ0V4cChzdHJpbmcpLnJlcGxhY2UoL1xcLysvZywgJy8rJylcbn1cblxuZnVuY3Rpb24gX2NvbXBpbGVQYXR0ZXJuKHBhdHRlcm4pIHtcbiAgbGV0IHJlZ2V4cFNvdXJjZSA9ICcnO1xuICBjb25zdCBwYXJhbU5hbWVzID0gW107XG4gIGNvbnN0IHRva2VucyA9IFtdO1xuXG4gIGxldCBtYXRjaCwgbGFzdEluZGV4ID0gMCwgbWF0Y2hlciA9IC86KFthLXpBLVpfJF1bYS16QS1aMC05XyRdKil8XFwqXFwqfFxcKnxcXCh8XFwpL2dcbiAgLyplc2xpbnQgbm8tY29uZC1hc3NpZ246IDAqL1xuICB3aGlsZSAoKG1hdGNoID0gbWF0Y2hlci5leGVjKHBhdHRlcm4pKSkge1xuICAgIGlmIChtYXRjaC5pbmRleCAhPT0gbGFzdEluZGV4KSB7XG4gICAgICB0b2tlbnMucHVzaChwYXR0ZXJuLnNsaWNlKGxhc3RJbmRleCwgbWF0Y2guaW5kZXgpKVxuICAgICAgcmVnZXhwU291cmNlICs9IGVzY2FwZVNvdXJjZShwYXR0ZXJuLnNsaWNlKGxhc3RJbmRleCwgbWF0Y2guaW5kZXgpKVxuICAgIH1cblxuICAgIGlmIChtYXRjaFsxXSkge1xuICAgICAgcmVnZXhwU291cmNlICs9ICcoW14vPyNdKyknO1xuICAgICAgcGFyYW1OYW1lcy5wdXNoKG1hdGNoWzFdKTtcbiAgICB9IGVsc2UgaWYgKG1hdGNoWzBdID09PSAnKionKSB7XG4gICAgICByZWdleHBTb3VyY2UgKz0gJyhbXFxcXHNcXFxcU10qKSdcbiAgICAgIHBhcmFtTmFtZXMucHVzaCgnc3BsYXQnKTtcbiAgICB9IGVsc2UgaWYgKG1hdGNoWzBdID09PSAnKicpIHtcbiAgICAgIHJlZ2V4cFNvdXJjZSArPSAnKFtcXFxcc1xcXFxTXSo/KSdcbiAgICAgIHBhcmFtTmFtZXMucHVzaCgnc3BsYXQnKTtcbiAgICB9IGVsc2UgaWYgKG1hdGNoWzBdID09PSAnKCcpIHtcbiAgICAgIHJlZ2V4cFNvdXJjZSArPSAnKD86JztcbiAgICB9IGVsc2UgaWYgKG1hdGNoWzBdID09PSAnKScpIHtcbiAgICAgIHJlZ2V4cFNvdXJjZSArPSAnKT8nO1xuICAgIH1cblxuICAgIHRva2Vucy5wdXNoKG1hdGNoWzBdKTtcblxuICAgIGxhc3RJbmRleCA9IG1hdGNoZXIubGFzdEluZGV4O1xuICB9XG5cbiAgaWYgKGxhc3RJbmRleCAhPT0gcGF0dGVybi5sZW5ndGgpIHtcbiAgICB0b2tlbnMucHVzaChwYXR0ZXJuLnNsaWNlKGxhc3RJbmRleCwgcGF0dGVybi5sZW5ndGgpKVxuICAgIHJlZ2V4cFNvdXJjZSArPSBlc2NhcGVTb3VyY2UocGF0dGVybi5zbGljZShsYXN0SW5kZXgsIHBhdHRlcm4ubGVuZ3RoKSlcbiAgfVxuXG4gIHJldHVybiB7XG4gICAgcGF0dGVybixcbiAgICByZWdleHBTb3VyY2UsXG4gICAgcGFyYW1OYW1lcyxcbiAgICB0b2tlbnNcbiAgfVxufVxuXG5jb25zdCBDb21waWxlZFBhdHRlcm5zQ2FjaGUgPSB7fVxuXG5leHBvcnQgZnVuY3Rpb24gY29tcGlsZVBhdHRlcm4ocGF0dGVybikge1xuICBpZiAoIShwYXR0ZXJuIGluIENvbXBpbGVkUGF0dGVybnNDYWNoZSkpXG4gICAgQ29tcGlsZWRQYXR0ZXJuc0NhY2hlW3BhdHRlcm5dID0gX2NvbXBpbGVQYXR0ZXJuKHBhdHRlcm4pXG5cbiAgcmV0dXJuIENvbXBpbGVkUGF0dGVybnNDYWNoZVtwYXR0ZXJuXVxufVxuXG4vKipcbiAqIEF0dGVtcHRzIHRvIG1hdGNoIGEgcGF0dGVybiBvbiB0aGUgZ2l2ZW4gcGF0aG5hbWUuIFBhdHRlcm5zIG1heSB1c2VcbiAqIHRoZSBmb2xsb3dpbmcgc3BlY2lhbCBjaGFyYWN0ZXJzOlxuICpcbiAqIC0gOnBhcmFtTmFtZSAgICAgTWF0Y2hlcyBhIFVSTCBzZWdtZW50IHVwIHRvIHRoZSBuZXh0IC8sID8sIG9yICMuIFRoZVxuICogICAgICAgICAgICAgICAgICBjYXB0dXJlZCBzdHJpbmcgaXMgY29uc2lkZXJlZCBhIFwicGFyYW1cIlxuICogLSAoKSAgICAgICAgICAgICBXcmFwcyBhIHNlZ21lbnQgb2YgdGhlIFVSTCB0aGF0IGlzIG9wdGlvbmFsXG4gKiAtICogICAgICAgICAgICAgIENvbnN1bWVzIChub24tZ3JlZWR5KSBhbGwgY2hhcmFjdGVycyB1cCB0byB0aGUgbmV4dFxuICogICAgICAgICAgICAgICAgICBjaGFyYWN0ZXIgaW4gdGhlIHBhdHRlcm4sIG9yIHRvIHRoZSBlbmQgb2YgdGhlIFVSTCBpZlxuICogICAgICAgICAgICAgICAgICB0aGVyZSBpcyBub25lXG4gKiAtICoqICAgICAgICAgICAgIENvbnN1bWVzIChncmVlZHkpIGFsbCBjaGFyYWN0ZXJzIHVwIHRvIHRoZSBuZXh0IGNoYXJhY3RlclxuICogICAgICAgICAgICAgICAgICBpbiB0aGUgcGF0dGVybiwgb3IgdG8gdGhlIGVuZCBvZiB0aGUgVVJMIGlmIHRoZXJlIGlzIG5vbmVcbiAqXG4gKiBUaGUgcmV0dXJuIHZhbHVlIGlzIGFuIG9iamVjdCB3aXRoIHRoZSBmb2xsb3dpbmcgcHJvcGVydGllczpcbiAqXG4gKiAtIHJlbWFpbmluZ1BhdGhuYW1lXG4gKiAtIHBhcmFtTmFtZXNcbiAqIC0gcGFyYW1WYWx1ZXNcbiAqL1xuZXhwb3J0IGZ1bmN0aW9uIG1hdGNoUGF0dGVybihwYXR0ZXJuLCBwYXRobmFtZSkge1xuICAvLyBNYWtlIGxlYWRpbmcgc2xhc2hlcyBjb25zaXN0ZW50IGJldHdlZW4gcGF0dGVybiBhbmQgcGF0aG5hbWUuXG4gIGlmIChwYXR0ZXJuLmNoYXJBdCgwKSAhPT0gJy8nKSB7XG4gICAgcGF0dGVybiA9IGAvJHtwYXR0ZXJufWBcbiAgfVxuICBpZiAocGF0aG5hbWUuY2hhckF0KDApICE9PSAnLycpIHtcbiAgICBwYXRobmFtZSA9IGAvJHtwYXRobmFtZX1gXG4gIH1cblxuICBsZXQgeyByZWdleHBTb3VyY2UsIHBhcmFtTmFtZXMsIHRva2VucyB9ID0gY29tcGlsZVBhdHRlcm4ocGF0dGVybilcblxuICByZWdleHBTb3VyY2UgKz0gJy8qJyAvLyBDYXB0dXJlIHBhdGggc2VwYXJhdG9yc1xuXG4gIC8vIFNwZWNpYWwtY2FzZSBwYXR0ZXJucyBsaWtlICcqJyBmb3IgY2F0Y2gtYWxsIHJvdXRlcy5cbiAgY29uc3QgY2FwdHVyZVJlbWFpbmluZyA9IHRva2Vuc1t0b2tlbnMubGVuZ3RoIC0gMV0gIT09ICcqJ1xuXG4gIGlmIChjYXB0dXJlUmVtYWluaW5nKSB7XG4gICAgLy8gVGhpcyB3aWxsIG1hdGNoIG5ld2xpbmVzIGluIHRoZSByZW1haW5pbmcgcGF0aC5cbiAgICByZWdleHBTb3VyY2UgKz0gJyhbXFxcXHNcXFxcU10qPyknXG4gIH1cblxuICBjb25zdCBtYXRjaCA9IHBhdGhuYW1lLm1hdGNoKG5ldyBSZWdFeHAoJ14nICsgcmVnZXhwU291cmNlICsgJyQnLCAnaScpKVxuXG4gIGxldCByZW1haW5pbmdQYXRobmFtZSwgcGFyYW1WYWx1ZXNcbiAgaWYgKG1hdGNoICE9IG51bGwpIHtcbiAgICBpZiAoY2FwdHVyZVJlbWFpbmluZykge1xuICAgICAgcmVtYWluaW5nUGF0aG5hbWUgPSBtYXRjaC5wb3AoKVxuICAgICAgY29uc3QgbWF0Y2hlZFBhdGggPVxuICAgICAgICBtYXRjaFswXS5zdWJzdHIoMCwgbWF0Y2hbMF0ubGVuZ3RoIC0gcmVtYWluaW5nUGF0aG5hbWUubGVuZ3RoKVxuXG4gICAgICAvLyBJZiB3ZSBkaWRuJ3QgbWF0Y2ggdGhlIGVudGlyZSBwYXRobmFtZSwgdGhlbiBtYWtlIHN1cmUgdGhhdCB0aGUgbWF0Y2hcbiAgICAgIC8vIHdlIGRpZCBnZXQgZW5kcyBhdCBhIHBhdGggc2VwYXJhdG9yIChwb3RlbnRpYWxseSB0aGUgb25lIHdlIGFkZGVkXG4gICAgICAvLyBhYm92ZSBhdCB0aGUgYmVnaW5uaW5nIG9mIHRoZSBwYXRoLCBpZiB0aGUgYWN0dWFsIG1hdGNoIHdhcyBlbXB0eSkuXG4gICAgICBpZiAoXG4gICAgICAgIHJlbWFpbmluZ1BhdGhuYW1lICYmXG4gICAgICAgIG1hdGNoZWRQYXRoLmNoYXJBdChtYXRjaGVkUGF0aC5sZW5ndGggLSAxKSAhPT0gJy8nXG4gICAgICApIHtcbiAgICAgICAgcmV0dXJuIHtcbiAgICAgICAgICByZW1haW5pbmdQYXRobmFtZTogbnVsbCxcbiAgICAgICAgICBwYXJhbU5hbWVzLFxuICAgICAgICAgIHBhcmFtVmFsdWVzOiBudWxsXG4gICAgICAgIH1cbiAgICAgIH1cbiAgICB9IGVsc2Uge1xuICAgICAgLy8gSWYgdGhpcyBtYXRjaGVkIGF0IGFsbCwgdGhlbiB0aGUgbWF0Y2ggd2FzIHRoZSBlbnRpcmUgcGF0aG5hbWUuXG4gICAgICByZW1haW5pbmdQYXRobmFtZSA9ICcnXG4gICAgfVxuXG4gICAgcGFyYW1WYWx1ZXMgPSBtYXRjaC5zbGljZSgxKS5tYXAoXG4gICAgICB2ID0+IHYgIT0gbnVsbCA/IGRlY29kZVVSSUNvbXBvbmVudCh2KSA6IHZcbiAgICApXG4gIH0gZWxzZSB7XG4gICAgcmVtYWluaW5nUGF0aG5hbWUgPSBwYXJhbVZhbHVlcyA9IG51bGxcbiAgfVxuXG4gIHJldHVybiB7XG4gICAgcmVtYWluaW5nUGF0aG5hbWUsXG4gICAgcGFyYW1OYW1lcyxcbiAgICBwYXJhbVZhbHVlc1xuICB9XG59XG5cbmV4cG9ydCBmdW5jdGlvbiBnZXRQYXJhbU5hbWVzKHBhdHRlcm4pIHtcbiAgcmV0dXJuIGNvbXBpbGVQYXR0ZXJuKHBhdHRlcm4pLnBhcmFtTmFtZXNcbn1cblxuZXhwb3J0IGZ1bmN0aW9uIGdldFBhcmFtcyhwYXR0ZXJuLCBwYXRobmFtZSkge1xuICBjb25zdCB7IHBhcmFtTmFtZXMsIHBhcmFtVmFsdWVzIH0gPSBtYXRjaFBhdHRlcm4ocGF0dGVybiwgcGF0aG5hbWUpXG5cbiAgaWYgKHBhcmFtVmFsdWVzICE9IG51bGwpIHtcbiAgICByZXR1cm4gcGFyYW1OYW1lcy5yZWR1Y2UoZnVuY3Rpb24gKG1lbW8sIHBhcmFtTmFtZSwgaW5kZXgpIHtcbiAgICAgIG1lbW9bcGFyYW1OYW1lXSA9IHBhcmFtVmFsdWVzW2luZGV4XVxuICAgICAgcmV0dXJuIG1lbW9cbiAgICB9LCB7fSlcbiAgfVxuXG4gIHJldHVybiBudWxsXG59XG5cbi8qKlxuICogUmV0dXJucyBhIHZlcnNpb24gb2YgdGhlIGdpdmVuIHBhdHRlcm4gd2l0aCBwYXJhbXMgaW50ZXJwb2xhdGVkLiBUaHJvd3NcbiAqIGlmIHRoZXJlIGlzIGEgZHluYW1pYyBzZWdtZW50IG9mIHRoZSBwYXR0ZXJuIGZvciB3aGljaCB0aGVyZSBpcyBubyBwYXJhbS5cbiAqL1xuZXhwb3J0IGZ1bmN0aW9uIGZvcm1hdFBhdHRlcm4ocGF0dGVybiwgcGFyYW1zKSB7XG4gIHBhcmFtcyA9IHBhcmFtcyB8fCB7fVxuXG4gIGNvbnN0IHsgdG9rZW5zIH0gPSBjb21waWxlUGF0dGVybihwYXR0ZXJuKVxuICBsZXQgcGFyZW5Db3VudCA9IDAsIHBhdGhuYW1lID0gJycsIHNwbGF0SW5kZXggPSAwXG5cbiAgbGV0IHRva2VuLCBwYXJhbU5hbWUsIHBhcmFtVmFsdWVcbiAgZm9yIChsZXQgaSA9IDAsIGxlbiA9IHRva2Vucy5sZW5ndGg7IGkgPCBsZW47ICsraSkge1xuICAgIHRva2VuID0gdG9rZW5zW2ldXG5cbiAgICBpZiAodG9rZW4gPT09ICcqJyB8fCB0b2tlbiA9PT0gJyoqJykge1xuICAgICAgcGFyYW1WYWx1ZSA9IEFycmF5LmlzQXJyYXkocGFyYW1zLnNwbGF0KSA/IHBhcmFtcy5zcGxhdFtzcGxhdEluZGV4KytdIDogcGFyYW1zLnNwbGF0XG5cbiAgICAgIGludmFyaWFudChcbiAgICAgICAgcGFyYW1WYWx1ZSAhPSBudWxsIHx8IHBhcmVuQ291bnQgPiAwLFxuICAgICAgICAnTWlzc2luZyBzcGxhdCAjJXMgZm9yIHBhdGggXCIlc1wiJyxcbiAgICAgICAgc3BsYXRJbmRleCwgcGF0dGVyblxuICAgICAgKVxuXG4gICAgICBpZiAocGFyYW1WYWx1ZSAhPSBudWxsKVxuICAgICAgICBwYXRobmFtZSArPSBlbmNvZGVVUkkocGFyYW1WYWx1ZSlcbiAgICB9IGVsc2UgaWYgKHRva2VuID09PSAnKCcpIHtcbiAgICAgIHBhcmVuQ291bnQgKz0gMVxuICAgIH0gZWxzZSBpZiAodG9rZW4gPT09ICcpJykge1xuICAgICAgcGFyZW5Db3VudCAtPSAxXG4gICAgfSBlbHNlIGlmICh0b2tlbi5jaGFyQXQoMCkgPT09ICc6Jykge1xuICAgICAgcGFyYW1OYW1lID0gdG9rZW4uc3Vic3RyaW5nKDEpXG4gICAgICBwYXJhbVZhbHVlID0gcGFyYW1zW3BhcmFtTmFtZV1cblxuICAgICAgaW52YXJpYW50KFxuICAgICAgICBwYXJhbVZhbHVlICE9IG51bGwgfHwgcGFyZW5Db3VudCA+IDAsXG4gICAgICAgICdNaXNzaW5nIFwiJXNcIiBwYXJhbWV0ZXIgZm9yIHBhdGggXCIlc1wiJyxcbiAgICAgICAgcGFyYW1OYW1lLCBwYXR0ZXJuXG4gICAgICApXG5cbiAgICAgIGlmIChwYXJhbVZhbHVlICE9IG51bGwpXG4gICAgICAgIHBhdGhuYW1lICs9IGVuY29kZVVSSUNvbXBvbmVudChwYXJhbVZhbHVlKVxuICAgIH0gZWxzZSB7XG4gICAgICBwYXRobmFtZSArPSB0b2tlblxuICAgIH1cbiAgfVxuXG4gIHJldHVybiBwYXRobmFtZS5yZXBsYWNlKC9cXC8rL2csICcvJylcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21tb24vcGF0dGVyblV0aWxzLmpzXG4gKiovIiwidmFyIFR0eSA9IHJlcXVpcmUoJ2FwcC9jb21tb24vdHR5Jyk7XG52YXIgYXBpID0gcmVxdWlyZSgnYXBwL3NlcnZpY2VzL2FwaScpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxuY2xhc3MgVHR5UGxheWVyIGV4dGVuZHMgVHR5IHtcbiAgY29uc3RydWN0b3Ioe3NpZH0pe1xuICAgIHN1cGVyKHt9KTtcbiAgICB0aGlzLnNpZCA9IHNpZDtcbiAgICB0aGlzLmN1cnJlbnQgPSAxO1xuICAgIHRoaXMubGVuZ3RoID0gLTE7XG4gICAgdGhpcy50dHlTdGVhbSA9IG5ldyBBcnJheSgpO1xuICAgIHRoaXMuaXNMb2FpbmQgPSBmYWxzZTtcbiAgICB0aGlzLmlzUGxheWluZyA9IGZhbHNlO1xuICAgIHRoaXMuaXNFcnJvciA9IGZhbHNlO1xuICAgIHRoaXMuaXNSZWFkeSA9IGZhbHNlO1xuICAgIHRoaXMuaXNMb2FkaW5nID0gdHJ1ZTtcbiAgfVxuXG4gIHNlbmQoKXtcbiAgfVxuXG4gIHJlc2l6ZSgpe1xuICB9XG5cbiAgY29ubmVjdCgpe1xuICAgIGFwaS5nZXQoY2ZnLmFwaS5nZXRGZXRjaFNlc3Npb25MZW5ndGhVcmwodGhpcy5zaWQpKVxuICAgICAgLmRvbmUoKGRhdGEpPT57XG4gICAgICAgIHRoaXMubGVuZ3RoID0gZGF0YS5jb3VudDtcbiAgICAgICAgdGhpcy5pc1JlYWR5ID0gdHJ1ZTtcbiAgICAgIH0pXG4gICAgICAuZmFpbCgoKT0+e1xuICAgICAgICB0aGlzLmlzRXJyb3IgPSB0cnVlO1xuICAgICAgfSlcbiAgICAgIC5hbHdheXMoKCk9PntcbiAgICAgICAgdGhpcy5fY2hhbmdlKCk7XG4gICAgICB9KTtcbiAgfVxuXG4gIG1vdmUobmV3UG9zKXtcbiAgICBpZighdGhpcy5pc1JlYWR5KXtcbiAgICAgIHJldHVybjtcbiAgICB9XG5cbiAgICBpZihuZXdQb3MgPT09IHVuZGVmaW5lZCl7XG4gICAgICBuZXdQb3MgPSB0aGlzLmN1cnJlbnQgKyAxO1xuICAgIH1cblxuICAgIGlmKG5ld1BvcyA+IHRoaXMubGVuZ3RoKXtcbiAgICAgIG5ld1BvcyA9IHRoaXMubGVuZ3RoO1xuICAgICAgdGhpcy5zdG9wKCk7XG4gICAgfVxuXG4gICAgaWYobmV3UG9zID09PSAwKXtcbiAgICAgIG5ld1BvcyA9IDE7XG4gICAgfVxuXG4gICAgaWYodGhpcy5pc1BsYXlpbmcpe1xuICAgICAgaWYodGhpcy5jdXJyZW50IDwgbmV3UG9zKXtcbiAgICAgICAgdGhpcy5fc2hvd0NodW5rKHRoaXMuY3VycmVudCwgbmV3UG9zKTtcbiAgICAgIH1lbHNle1xuICAgICAgICB0aGlzLmVtaXQoJ3Jlc2V0Jyk7XG4gICAgICAgIHRoaXMuX3Nob3dDaHVuayh0aGlzLmN1cnJlbnQsIG5ld1Bvcyk7XG4gICAgICB9XG4gICAgfWVsc2V7XG4gICAgICB0aGlzLmN1cnJlbnQgPSBuZXdQb3M7XG4gICAgfVxuXG4gICAgdGhpcy5fY2hhbmdlKCk7XG4gIH1cblxuICBzdG9wKCl7XG4gICAgdGhpcy5pc1BsYXlpbmcgPSBmYWxzZTtcbiAgICB0aGlzLnRpbWVyID0gY2xlYXJJbnRlcnZhbCh0aGlzLnRpbWVyKTtcbiAgICB0aGlzLl9jaGFuZ2UoKTtcbiAgfVxuXG4gIHBsYXkoKXtcbiAgICBpZih0aGlzLmlzUGxheWluZyl7XG4gICAgICByZXR1cm47XG4gICAgfVxuXG4gICAgdGhpcy5pc1BsYXlpbmcgPSB0cnVlO1xuXG4gICAgLy8gc3RhcnQgZnJvbSB0aGUgYmVnaW5uaW5nIGlmIGF0IHRoZSBlbmRcbiAgICBpZih0aGlzLmN1cnJlbnQgPT09IHRoaXMubGVuZ3RoKXtcbiAgICAgIHRoaXMuY3VycmVudCA9IDE7XG4gICAgICB0aGlzLmVtaXQoJ3Jlc2V0Jyk7XG4gICAgfVxuXG4gICAgdGhpcy50aW1lciA9IHNldEludGVydmFsKHRoaXMubW92ZS5iaW5kKHRoaXMpLCAxNTApO1xuICAgIHRoaXMuX2NoYW5nZSgpO1xuICB9XG5cbiAgX3Nob3VsZEZldGNoKHN0YXJ0LCBlbmQpe1xuICAgIGZvcih2YXIgaSA9IHN0YXJ0OyBpIDwgZW5kOyBpKyspe1xuICAgICAgaWYodGhpcy50dHlTdGVhbVtpXSA9PT0gdW5kZWZpbmVkKXtcbiAgICAgICAgcmV0dXJuIHRydWU7XG4gICAgICB9XG4gICAgfVxuXG4gICAgcmV0dXJuIGZhbHNlO1xuICB9XG5cbiAgX2ZldGNoKHN0YXJ0LCBlbmQpe1xuICAgIGVuZCA9IGVuZCArIDUwO1xuICAgIGVuZCA9IGVuZCA+IHRoaXMubGVuZ3RoID8gdGhpcy5sZW5ndGggOiBlbmQ7XG4gICAgcmV0dXJuIGFwaS5nZXQoY2ZnLmFwaS5nZXRGZXRjaFNlc3Npb25DaHVua1VybCh7c2lkOiB0aGlzLnNpZCwgc3RhcnQsIGVuZH0pKS5cbiAgICAgIGRvbmUoKHJlc3BvbnNlKT0+e1xuICAgICAgICBmb3IodmFyIGkgPSAwOyBpIDwgZW5kLXN0YXJ0OyBpKyspe1xuICAgICAgICAgIHZhciBkYXRhID0gYXRvYihyZXNwb25zZS5jaHVua3NbaV0uZGF0YSkgfHwgJyc7XG4gICAgICAgICAgdmFyIGRlbGF5ID0gcmVzcG9uc2UuY2h1bmtzW2ldLmRlbGF5O1xuICAgICAgICAgIHRoaXMudHR5U3RlYW1bc3RhcnQraV0gPSB7IGRhdGEsIGRlbGF5fTtcbiAgICAgICAgfVxuICAgICAgfSk7XG4gIH1cblxuICBfc2hvd0NodW5rKHN0YXJ0LCBlbmQpe1xuICAgIHZhciBkaXNwbGF5ID0gKCk9PntcbiAgICAgIGZvcih2YXIgaSA9IHN0YXJ0OyBpIDwgZW5kOyBpKyspe1xuICAgICAgICB0aGlzLmVtaXQoJ2RhdGEnLCB0aGlzLnR0eVN0ZWFtW2ldLmRhdGEpO1xuICAgICAgfVxuICAgICAgdGhpcy5jdXJyZW50ID0gZW5kO1xuICAgIH07XG5cbiAgICBpZih0aGlzLl9zaG91bGRGZXRjaChzdGFydCwgZW5kKSl7XG4gICAgICB0aGlzLl9mZXRjaChzdGFydCwgZW5kKS50aGVuKGRpc3BsYXkpO1xuICAgIH1lbHNle1xuICAgICAgZGlzcGxheSgpO1xuICAgIH1cbiAgfVxuXG4gIF9jaGFuZ2UoKXtcbiAgICB0aGlzLmVtaXQoJ2NoYW5nZScpO1xuICB9XG59XG5cbmV4cG9ydCBkZWZhdWx0IFR0eVBsYXllcjtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21tb24vdHR5UGxheWVyLmpzXG4gKiovIiwidmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHtmZXRjaFNlc3Npb25zfSA9IHJlcXVpcmUoJy4vLi4vc2Vzc2lvbnMvYWN0aW9ucycpO1xudmFyIHtmZXRjaE5vZGVzfSA9IHJlcXVpcmUoJy4vLi4vbm9kZXMvYWN0aW9ucycpO1xudmFyIHttb250aFJhbmdlfSA9IHJlcXVpcmUoJ2FwcC9jb21tb24vZGF0ZVV0aWxzJyk7XG5cbnZhciAkID0gcmVxdWlyZSgnalF1ZXJ5Jyk7XG5cbnZhciB7IFRMUFRfQVBQX0lOSVQsIFRMUFRfQVBQX0ZBSUxFRCwgVExQVF9BUFBfUkVBRFkgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxudmFyIGFjdGlvbnMgPSB7XG5cbiAgaW5pdEFwcCgpIHtcbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfQVBQX0lOSVQpO1xuICAgIGFjdGlvbnMuZmV0Y2hOb2Rlc0FuZFNlc3Npb25zKClcbiAgICAgIC5kb25lKCgpPT57IHJlYWN0b3IuZGlzcGF0Y2goVExQVF9BUFBfUkVBRFkpOyB9KVxuICAgICAgLmZhaWwoKCk9PnsgcmVhY3Rvci5kaXNwYXRjaChUTFBUX0FQUF9GQUlMRUQpOyB9KTtcbiAgfSxcblxuICBmZXRjaE5vZGVzQW5kU2Vzc2lvbnMoKSB7XG4gICAgdmFyIFtzdGFydCwgZW5kIF0gPSBtb250aFJhbmdlKCk7XG4gICAgcmV0dXJuICQud2hlbihmZXRjaE5vZGVzKCksIGZldGNoU2Vzc2lvbnMoc3RhcnQsIGVuZCkpO1xuICB9XG59XG5cbmV4cG9ydCBkZWZhdWx0IGFjdGlvbnM7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hcHAvYWN0aW9ucy5qc1xuICoqLyIsImNvbnN0IGFwcFN0YXRlID0gW1sndGxwdCddLCBhcHA9PiBhcHAudG9KUygpXTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICBhcHBTdGF0ZVxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvYXBwL2dldHRlcnMuanNcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cy5nZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3Rpb25zID0gcmVxdWlyZSgnLi9hY3Rpb25zJyk7XG5tb2R1bGUuZXhwb3J0cy5hcHBTdG9yZSA9IHJlcXVpcmUoJy4vYXBwU3RvcmUnKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2FwcC9pbmRleC5qc1xuICoqLyIsImNvbnN0IGRpYWxvZ3MgPSBbWyd0bHB0X2RpYWxvZ3MnXSwgc3RhdGU9PiBzdGF0ZS50b0pTKCldO1xuXG5leHBvcnQgZGVmYXVsdCB7XG4gIGRpYWxvZ3Ncbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvZ2V0dGVycy5qc1xuICoqLyIsIm1vZHVsZS5leHBvcnRzLmdldHRlcnMgPSByZXF1aXJlKCcuL2dldHRlcnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGlvbnMgPSByZXF1aXJlKCcuL2FjdGlvbnMnKTtcbm1vZHVsZS5leHBvcnRzLmRpYWxvZ1N0b3JlID0gcmVxdWlyZSgnLi9kaWFsb2dTdG9yZScpO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvZGlhbG9ncy9pbmRleC5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnJlYWN0b3IucmVnaXN0ZXJTdG9yZXMoe1xuICAndGxwdCc6IHJlcXVpcmUoJy4vYXBwL2FwcFN0b3JlJyksXG4gICd0bHB0X2RpYWxvZ3MnOiByZXF1aXJlKCcuL2RpYWxvZ3MvZGlhbG9nU3RvcmUnKSxcbiAgJ3RscHRfYWN0aXZlX3Rlcm1pbmFsJzogcmVxdWlyZSgnLi9hY3RpdmVUZXJtaW5hbC9hY3RpdmVUZXJtU3RvcmUnKSxcbiAgJ3RscHRfdXNlcic6IHJlcXVpcmUoJy4vdXNlci91c2VyU3RvcmUnKSxcbiAgJ3RscHRfbm9kZXMnOiByZXF1aXJlKCcuL25vZGVzL25vZGVTdG9yZScpLFxuICAndGxwdF9pbnZpdGUnOiByZXF1aXJlKCcuL2ludml0ZS9pbnZpdGVTdG9yZScpLFxuICAndGxwdF9yZXN0X2FwaSc6IHJlcXVpcmUoJy4vcmVzdEFwaS9yZXN0QXBpU3RvcmUnKSxcbiAgJ3RscHRfc2Vzc2lvbnMnOiByZXF1aXJlKCcuL3Nlc3Npb25zL3Nlc3Npb25TdG9yZScpXG59KTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2luZGV4LmpzXG4gKiovIiwidmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHsgVExQVF9SRUNFSVZFX1VTRVJfSU5WSVRFIH0gID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xudmFyIHsgRkVUQ0hJTkdfSU5WSVRFfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3Jlc3RBcGkvY29uc3RhbnRzJyk7XG52YXIgcmVzdEFwaUFjdGlvbnMgPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9yZXN0QXBpL2FjdGlvbnMnKTtcbnZhciBhcGkgPSByZXF1aXJlKCdhcHAvc2VydmljZXMvYXBpJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xuXG5leHBvcnQgZGVmYXVsdCB7XG4gIGZldGNoSW52aXRlKGludml0ZVRva2VuKXtcbiAgICB2YXIgcGF0aCA9IGNmZy5hcGkuZ2V0SW52aXRlVXJsKGludml0ZVRva2VuKTtcbiAgICByZXN0QXBpQWN0aW9ucy5zdGFydChGRVRDSElOR19JTlZJVEUpO1xuICAgIGFwaS5nZXQocGF0aCkuZG9uZShpbnZpdGU9PntcbiAgICAgIHJlc3RBcGlBY3Rpb25zLnN1Y2Nlc3MoRkVUQ0hJTkdfSU5WSVRFKTtcbiAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRUNFSVZFX1VTRVJfSU5WSVRFLCBpbnZpdGUpO1xuICAgIH0pLlxuICAgIGZhaWwoKGVycik9PntcbiAgICAgIHJlc3RBcGlBY3Rpb25zLmZhaWwoRkVUQ0hJTkdfSU5WSVRFLCBlcnIucmVzcG9uc2VKU09OLm1lc3NhZ2UpO1xuICAgIH0pO1xuICB9XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvYWN0aW9ucy5qc1xuICoqLyIsInZhciB7VFJZSU5HX1RPX1NJR05fVVAsIEZFVENISU5HX0lOVklURX0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9yZXN0QXBpL2NvbnN0YW50cycpO1xudmFyIHtyZXF1ZXN0U3RhdHVzfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3Jlc3RBcGkvZ2V0dGVycycpO1xuXG5jb25zdCBpbnZpdGUgPSBbIFsndGxwdF9pbnZpdGUnXSwgKGludml0ZSkgPT4gaW52aXRlIF07XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgaW52aXRlLFxuICBhdHRlbXA6IHJlcXVlc3RTdGF0dXMoVFJZSU5HX1RPX1NJR05fVVApLFxuICBmZXRjaGluZ0ludml0ZTogcmVxdWVzdFN0YXR1cyhGRVRDSElOR19JTlZJVEUpXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvZ2V0dGVycy5qc1xuICoqLyIsIm1vZHVsZS5leHBvcnRzLmdldHRlcnMgPSByZXF1aXJlKCcuL2dldHRlcnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGlvbnMgPSByZXF1aXJlKCcuL2FjdGlvbnMnKTtcbm1vZHVsZS5leHBvcnRzLm5vZGVTdG9yZSA9IHJlcXVpcmUoJy4vaW52aXRlU3RvcmUnKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9pbmRleC5qc1xuICoqLyIsIm1vZHVsZS5leHBvcnRzLmdldHRlcnMgPSByZXF1aXJlKCcuL2dldHRlcnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGlvbnMgPSByZXF1aXJlKCcuL2FjdGlvbnMnKTtcbm1vZHVsZS5leHBvcnRzLm5vZGVTdG9yZSA9IHJlcXVpcmUoJy4vbm9kZVN0b3JlJyk7XG5cbi8vIG5vZGVzOiBbe1wiaWRcIjpcIngyMjBcIixcImFkZHJcIjpcIjAuMC4wLjA6MzAyMlwiLFwiaG9zdG5hbWVcIjpcIngyMjBcIixcImxhYmVsc1wiOm51bGwsXCJjbWRfbGFiZWxzXCI6bnVsbH1dXG5cblxuLy8gc2Vzc2lvbnM6IFt7XCJpZFwiOlwiMDc2MzA2MzYtYmIzZC00MGUxLWIwODYtNjBiMmNhZTIxYWM0XCIsXCJwYXJ0aWVzXCI6W3tcImlkXCI6XCI4OWY3NjJhMy03NDI5LTRjN2EtYTkxMy03NjY0OTNmZTdjOGFcIixcInNpdGVcIjpcIjEyNy4wLjAuMTozNzUxNFwiLFwidXNlclwiOlwiYWtvbnRzZXZveVwiLFwic2VydmVyX2FkZHJcIjpcIjAuMC4wLjA6MzAyMlwiLFwibGFzdF9hY3RpdmVcIjpcIjIwMTYtMDItMjJUMTQ6Mzk6MjAuOTMxMjA1MzUtMDU6MDBcIn1dfV1cblxuLypcbmxldCBUb2RvUmVjb3JkID0gSW1tdXRhYmxlLlJlY29yZCh7XG4gICAgaWQ6IDAsXG4gICAgZGVzY3JpcHRpb246IFwiXCIsXG4gICAgY29tcGxldGVkOiBmYWxzZVxufSk7XG4qL1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvaW5kZXguanNcbiAqKi8iLCJ2YXIgeyBTdG9yZSwgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcbnZhciB7XG4gIFRMUFRfUkVTVF9BUElfU1RBUlQsXG4gIFRMUFRfUkVTVF9BUElfU1VDQ0VTUyxcbiAgVExQVF9SRVNUX0FQSV9GQUlMIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IFN0b3JlKHtcbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB0b0ltbXV0YWJsZSh7fSk7XG4gIH0sXG5cbiAgaW5pdGlhbGl6ZSgpIHtcbiAgICB0aGlzLm9uKFRMUFRfUkVTVF9BUElfU1RBUlQsIHN0YXJ0KTtcbiAgICB0aGlzLm9uKFRMUFRfUkVTVF9BUElfRkFJTCwgZmFpbCk7XG4gICAgdGhpcy5vbihUTFBUX1JFU1RfQVBJX1NVQ0NFU1MsIHN1Y2Nlc3MpO1xuICB9XG59KVxuXG5mdW5jdGlvbiBzdGFydChzdGF0ZSwgcmVxdWVzdCl7XG4gIHJldHVybiBzdGF0ZS5zZXQocmVxdWVzdC50eXBlLCB0b0ltbXV0YWJsZSh7aXNQcm9jZXNzaW5nOiB0cnVlfSkpO1xufVxuXG5mdW5jdGlvbiBmYWlsKHN0YXRlLCByZXF1ZXN0KXtcbiAgcmV0dXJuIHN0YXRlLnNldChyZXF1ZXN0LnR5cGUsIHRvSW1tdXRhYmxlKHtpc0ZhaWxlZDogdHJ1ZSwgbWVzc2FnZTogcmVxdWVzdC5tZXNzYWdlfSkpO1xufVxuXG5mdW5jdGlvbiBzdWNjZXNzKHN0YXRlLCByZXF1ZXN0KXtcbiAgcmV0dXJuIHN0YXRlLnNldChyZXF1ZXN0LnR5cGUsIHRvSW1tdXRhYmxlKHtpc1N1Y2Nlc3M6IHRydWV9KSk7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL3Jlc3RBcGlTdG9yZS5qc1xuICoqLyIsInZhciB1dGlscyA9IHtcblxuICB1dWlkKCl7XG4gICAgLy8gbmV2ZXIgdXNlIGl0IGluIHByb2R1Y3Rpb25cbiAgICByZXR1cm4gJ3h4eHh4eHh4LXh4eHgtNHh4eC15eHh4LXh4eHh4eHh4eHh4eCcucmVwbGFjZSgvW3h5XS9nLCBmdW5jdGlvbihjKSB7XG4gICAgICB2YXIgciA9IE1hdGgucmFuZG9tKCkqMTZ8MCwgdiA9IGMgPT0gJ3gnID8gciA6IChyJjB4M3wweDgpO1xuICAgICAgcmV0dXJuIHYudG9TdHJpbmcoMTYpO1xuICAgIH0pO1xuICB9LFxuXG4gIGRpc3BsYXlEYXRlKGRhdGUpe1xuICAgIHRyeXtcbiAgICAgIHJldHVybiBkYXRlLnRvTG9jYWxlRGF0ZVN0cmluZygpICsgJyAnICsgZGF0ZS50b0xvY2FsZVRpbWVTdHJpbmcoKTtcbiAgICB9Y2F0Y2goZXJyKXtcbiAgICAgIGNvbnNvbGUuZXJyb3IoZXJyKTtcbiAgICAgIHJldHVybiAndW5kZWZpbmVkJztcbiAgICB9XG4gIH0sXG5cbiAgZm9ybWF0U3RyaW5nKGZvcm1hdCkge1xuICAgIHZhciBhcmdzID0gQXJyYXkucHJvdG90eXBlLnNsaWNlLmNhbGwoYXJndW1lbnRzLCAxKTtcbiAgICByZXR1cm4gZm9ybWF0LnJlcGxhY2UobmV3IFJlZ0V4cCgnXFxcXHsoXFxcXGQrKVxcXFx9JywgJ2cnKSxcbiAgICAgIChtYXRjaCwgbnVtYmVyKSA9PiB7XG4gICAgICAgIHJldHVybiAhKGFyZ3NbbnVtYmVyXSA9PT0gbnVsbCB8fCBhcmdzW251bWJlcl0gPT09IHVuZGVmaW5lZCkgPyBhcmdzW251bWJlcl0gOiAnJztcbiAgICB9KTtcbiAgfVxuICAgICAgICAgICAgXG59XG5cbm1vZHVsZS5leHBvcnRzID0gdXRpbHM7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvdXRpbHMuanNcbiAqKi8iLCIvLyBDb3B5cmlnaHQgSm95ZW50LCBJbmMuIGFuZCBvdGhlciBOb2RlIGNvbnRyaWJ1dG9ycy5cbi8vXG4vLyBQZXJtaXNzaW9uIGlzIGhlcmVieSBncmFudGVkLCBmcmVlIG9mIGNoYXJnZSwgdG8gYW55IHBlcnNvbiBvYnRhaW5pbmcgYVxuLy8gY29weSBvZiB0aGlzIHNvZnR3YXJlIGFuZCBhc3NvY2lhdGVkIGRvY3VtZW50YXRpb24gZmlsZXMgKHRoZVxuLy8gXCJTb2Z0d2FyZVwiKSwgdG8gZGVhbCBpbiB0aGUgU29mdHdhcmUgd2l0aG91dCByZXN0cmljdGlvbiwgaW5jbHVkaW5nXG4vLyB3aXRob3V0IGxpbWl0YXRpb24gdGhlIHJpZ2h0cyB0byB1c2UsIGNvcHksIG1vZGlmeSwgbWVyZ2UsIHB1Ymxpc2gsXG4vLyBkaXN0cmlidXRlLCBzdWJsaWNlbnNlLCBhbmQvb3Igc2VsbCBjb3BpZXMgb2YgdGhlIFNvZnR3YXJlLCBhbmQgdG8gcGVybWl0XG4vLyBwZXJzb25zIHRvIHdob20gdGhlIFNvZnR3YXJlIGlzIGZ1cm5pc2hlZCB0byBkbyBzbywgc3ViamVjdCB0byB0aGVcbi8vIGZvbGxvd2luZyBjb25kaXRpb25zOlxuLy9cbi8vIFRoZSBhYm92ZSBjb3B5cmlnaHQgbm90aWNlIGFuZCB0aGlzIHBlcm1pc3Npb24gbm90aWNlIHNoYWxsIGJlIGluY2x1ZGVkXG4vLyBpbiBhbGwgY29waWVzIG9yIHN1YnN0YW50aWFsIHBvcnRpb25zIG9mIHRoZSBTb2Z0d2FyZS5cbi8vXG4vLyBUSEUgU09GVFdBUkUgSVMgUFJPVklERUQgXCJBUyBJU1wiLCBXSVRIT1VUIFdBUlJBTlRZIE9GIEFOWSBLSU5ELCBFWFBSRVNTXG4vLyBPUiBJTVBMSUVELCBJTkNMVURJTkcgQlVUIE5PVCBMSU1JVEVEIFRPIFRIRSBXQVJSQU5USUVTIE9GXG4vLyBNRVJDSEFOVEFCSUxJVFksIEZJVE5FU1MgRk9SIEEgUEFSVElDVUxBUiBQVVJQT1NFIEFORCBOT05JTkZSSU5HRU1FTlQuIElOXG4vLyBOTyBFVkVOVCBTSEFMTCBUSEUgQVVUSE9SUyBPUiBDT1BZUklHSFQgSE9MREVSUyBCRSBMSUFCTEUgRk9SIEFOWSBDTEFJTSxcbi8vIERBTUFHRVMgT1IgT1RIRVIgTElBQklMSVRZLCBXSEVUSEVSIElOIEFOIEFDVElPTiBPRiBDT05UUkFDVCwgVE9SVCBPUlxuLy8gT1RIRVJXSVNFLCBBUklTSU5HIEZST00sIE9VVCBPRiBPUiBJTiBDT05ORUNUSU9OIFdJVEggVEhFIFNPRlRXQVJFIE9SIFRIRVxuLy8gVVNFIE9SIE9USEVSIERFQUxJTkdTIElOIFRIRSBTT0ZUV0FSRS5cblxuZnVuY3Rpb24gRXZlbnRFbWl0dGVyKCkge1xuICB0aGlzLl9ldmVudHMgPSB0aGlzLl9ldmVudHMgfHwge307XG4gIHRoaXMuX21heExpc3RlbmVycyA9IHRoaXMuX21heExpc3RlbmVycyB8fCB1bmRlZmluZWQ7XG59XG5tb2R1bGUuZXhwb3J0cyA9IEV2ZW50RW1pdHRlcjtcblxuLy8gQmFja3dhcmRzLWNvbXBhdCB3aXRoIG5vZGUgMC4xMC54XG5FdmVudEVtaXR0ZXIuRXZlbnRFbWl0dGVyID0gRXZlbnRFbWl0dGVyO1xuXG5FdmVudEVtaXR0ZXIucHJvdG90eXBlLl9ldmVudHMgPSB1bmRlZmluZWQ7XG5FdmVudEVtaXR0ZXIucHJvdG90eXBlLl9tYXhMaXN0ZW5lcnMgPSB1bmRlZmluZWQ7XG5cbi8vIEJ5IGRlZmF1bHQgRXZlbnRFbWl0dGVycyB3aWxsIHByaW50IGEgd2FybmluZyBpZiBtb3JlIHRoYW4gMTAgbGlzdGVuZXJzIGFyZVxuLy8gYWRkZWQgdG8gaXQuIFRoaXMgaXMgYSB1c2VmdWwgZGVmYXVsdCB3aGljaCBoZWxwcyBmaW5kaW5nIG1lbW9yeSBsZWFrcy5cbkV2ZW50RW1pdHRlci5kZWZhdWx0TWF4TGlzdGVuZXJzID0gMTA7XG5cbi8vIE9idmlvdXNseSBub3QgYWxsIEVtaXR0ZXJzIHNob3VsZCBiZSBsaW1pdGVkIHRvIDEwLiBUaGlzIGZ1bmN0aW9uIGFsbG93c1xuLy8gdGhhdCB0byBiZSBpbmNyZWFzZWQuIFNldCB0byB6ZXJvIGZvciB1bmxpbWl0ZWQuXG5FdmVudEVtaXR0ZXIucHJvdG90eXBlLnNldE1heExpc3RlbmVycyA9IGZ1bmN0aW9uKG4pIHtcbiAgaWYgKCFpc051bWJlcihuKSB8fCBuIDwgMCB8fCBpc05hTihuKSlcbiAgICB0aHJvdyBUeXBlRXJyb3IoJ24gbXVzdCBiZSBhIHBvc2l0aXZlIG51bWJlcicpO1xuICB0aGlzLl9tYXhMaXN0ZW5lcnMgPSBuO1xuICByZXR1cm4gdGhpcztcbn07XG5cbkV2ZW50RW1pdHRlci5wcm90b3R5cGUuZW1pdCA9IGZ1bmN0aW9uKHR5cGUpIHtcbiAgdmFyIGVyLCBoYW5kbGVyLCBsZW4sIGFyZ3MsIGksIGxpc3RlbmVycztcblxuICBpZiAoIXRoaXMuX2V2ZW50cylcbiAgICB0aGlzLl9ldmVudHMgPSB7fTtcblxuICAvLyBJZiB0aGVyZSBpcyBubyAnZXJyb3InIGV2ZW50IGxpc3RlbmVyIHRoZW4gdGhyb3cuXG4gIGlmICh0eXBlID09PSAnZXJyb3InKSB7XG4gICAgaWYgKCF0aGlzLl9ldmVudHMuZXJyb3IgfHxcbiAgICAgICAgKGlzT2JqZWN0KHRoaXMuX2V2ZW50cy5lcnJvcikgJiYgIXRoaXMuX2V2ZW50cy5lcnJvci5sZW5ndGgpKSB7XG4gICAgICBlciA9IGFyZ3VtZW50c1sxXTtcbiAgICAgIGlmIChlciBpbnN0YW5jZW9mIEVycm9yKSB7XG4gICAgICAgIHRocm93IGVyOyAvLyBVbmhhbmRsZWQgJ2Vycm9yJyBldmVudFxuICAgICAgfVxuICAgICAgdGhyb3cgVHlwZUVycm9yKCdVbmNhdWdodCwgdW5zcGVjaWZpZWQgXCJlcnJvclwiIGV2ZW50LicpO1xuICAgIH1cbiAgfVxuXG4gIGhhbmRsZXIgPSB0aGlzLl9ldmVudHNbdHlwZV07XG5cbiAgaWYgKGlzVW5kZWZpbmVkKGhhbmRsZXIpKVxuICAgIHJldHVybiBmYWxzZTtcblxuICBpZiAoaXNGdW5jdGlvbihoYW5kbGVyKSkge1xuICAgIHN3aXRjaCAoYXJndW1lbnRzLmxlbmd0aCkge1xuICAgICAgLy8gZmFzdCBjYXNlc1xuICAgICAgY2FzZSAxOlxuICAgICAgICBoYW5kbGVyLmNhbGwodGhpcyk7XG4gICAgICAgIGJyZWFrO1xuICAgICAgY2FzZSAyOlxuICAgICAgICBoYW5kbGVyLmNhbGwodGhpcywgYXJndW1lbnRzWzFdKTtcbiAgICAgICAgYnJlYWs7XG4gICAgICBjYXNlIDM6XG4gICAgICAgIGhhbmRsZXIuY2FsbCh0aGlzLCBhcmd1bWVudHNbMV0sIGFyZ3VtZW50c1syXSk7XG4gICAgICAgIGJyZWFrO1xuICAgICAgLy8gc2xvd2VyXG4gICAgICBkZWZhdWx0OlxuICAgICAgICBsZW4gPSBhcmd1bWVudHMubGVuZ3RoO1xuICAgICAgICBhcmdzID0gbmV3IEFycmF5KGxlbiAtIDEpO1xuICAgICAgICBmb3IgKGkgPSAxOyBpIDwgbGVuOyBpKyspXG4gICAgICAgICAgYXJnc1tpIC0gMV0gPSBhcmd1bWVudHNbaV07XG4gICAgICAgIGhhbmRsZXIuYXBwbHkodGhpcywgYXJncyk7XG4gICAgfVxuICB9IGVsc2UgaWYgKGlzT2JqZWN0KGhhbmRsZXIpKSB7XG4gICAgbGVuID0gYXJndW1lbnRzLmxlbmd0aDtcbiAgICBhcmdzID0gbmV3IEFycmF5KGxlbiAtIDEpO1xuICAgIGZvciAoaSA9IDE7IGkgPCBsZW47IGkrKylcbiAgICAgIGFyZ3NbaSAtIDFdID0gYXJndW1lbnRzW2ldO1xuXG4gICAgbGlzdGVuZXJzID0gaGFuZGxlci5zbGljZSgpO1xuICAgIGxlbiA9IGxpc3RlbmVycy5sZW5ndGg7XG4gICAgZm9yIChpID0gMDsgaSA8IGxlbjsgaSsrKVxuICAgICAgbGlzdGVuZXJzW2ldLmFwcGx5KHRoaXMsIGFyZ3MpO1xuICB9XG5cbiAgcmV0dXJuIHRydWU7XG59O1xuXG5FdmVudEVtaXR0ZXIucHJvdG90eXBlLmFkZExpc3RlbmVyID0gZnVuY3Rpb24odHlwZSwgbGlzdGVuZXIpIHtcbiAgdmFyIG07XG5cbiAgaWYgKCFpc0Z1bmN0aW9uKGxpc3RlbmVyKSlcbiAgICB0aHJvdyBUeXBlRXJyb3IoJ2xpc3RlbmVyIG11c3QgYmUgYSBmdW5jdGlvbicpO1xuXG4gIGlmICghdGhpcy5fZXZlbnRzKVxuICAgIHRoaXMuX2V2ZW50cyA9IHt9O1xuXG4gIC8vIFRvIGF2b2lkIHJlY3Vyc2lvbiBpbiB0aGUgY2FzZSB0aGF0IHR5cGUgPT09IFwibmV3TGlzdGVuZXJcIiEgQmVmb3JlXG4gIC8vIGFkZGluZyBpdCB0byB0aGUgbGlzdGVuZXJzLCBmaXJzdCBlbWl0IFwibmV3TGlzdGVuZXJcIi5cbiAgaWYgKHRoaXMuX2V2ZW50cy5uZXdMaXN0ZW5lcilcbiAgICB0aGlzLmVtaXQoJ25ld0xpc3RlbmVyJywgdHlwZSxcbiAgICAgICAgICAgICAgaXNGdW5jdGlvbihsaXN0ZW5lci5saXN0ZW5lcikgP1xuICAgICAgICAgICAgICBsaXN0ZW5lci5saXN0ZW5lciA6IGxpc3RlbmVyKTtcblxuICBpZiAoIXRoaXMuX2V2ZW50c1t0eXBlXSlcbiAgICAvLyBPcHRpbWl6ZSB0aGUgY2FzZSBvZiBvbmUgbGlzdGVuZXIuIERvbid0IG5lZWQgdGhlIGV4dHJhIGFycmF5IG9iamVjdC5cbiAgICB0aGlzLl9ldmVudHNbdHlwZV0gPSBsaXN0ZW5lcjtcbiAgZWxzZSBpZiAoaXNPYmplY3QodGhpcy5fZXZlbnRzW3R5cGVdKSlcbiAgICAvLyBJZiB3ZSd2ZSBhbHJlYWR5IGdvdCBhbiBhcnJheSwganVzdCBhcHBlbmQuXG4gICAgdGhpcy5fZXZlbnRzW3R5cGVdLnB1c2gobGlzdGVuZXIpO1xuICBlbHNlXG4gICAgLy8gQWRkaW5nIHRoZSBzZWNvbmQgZWxlbWVudCwgbmVlZCB0byBjaGFuZ2UgdG8gYXJyYXkuXG4gICAgdGhpcy5fZXZlbnRzW3R5cGVdID0gW3RoaXMuX2V2ZW50c1t0eXBlXSwgbGlzdGVuZXJdO1xuXG4gIC8vIENoZWNrIGZvciBsaXN0ZW5lciBsZWFrXG4gIGlmIChpc09iamVjdCh0aGlzLl9ldmVudHNbdHlwZV0pICYmICF0aGlzLl9ldmVudHNbdHlwZV0ud2FybmVkKSB7XG4gICAgdmFyIG07XG4gICAgaWYgKCFpc1VuZGVmaW5lZCh0aGlzLl9tYXhMaXN0ZW5lcnMpKSB7XG4gICAgICBtID0gdGhpcy5fbWF4TGlzdGVuZXJzO1xuICAgIH0gZWxzZSB7XG4gICAgICBtID0gRXZlbnRFbWl0dGVyLmRlZmF1bHRNYXhMaXN0ZW5lcnM7XG4gICAgfVxuXG4gICAgaWYgKG0gJiYgbSA+IDAgJiYgdGhpcy5fZXZlbnRzW3R5cGVdLmxlbmd0aCA+IG0pIHtcbiAgICAgIHRoaXMuX2V2ZW50c1t0eXBlXS53YXJuZWQgPSB0cnVlO1xuICAgICAgY29uc29sZS5lcnJvcignKG5vZGUpIHdhcm5pbmc6IHBvc3NpYmxlIEV2ZW50RW1pdHRlciBtZW1vcnkgJyArXG4gICAgICAgICAgICAgICAgICAgICdsZWFrIGRldGVjdGVkLiAlZCBsaXN0ZW5lcnMgYWRkZWQuICcgK1xuICAgICAgICAgICAgICAgICAgICAnVXNlIGVtaXR0ZXIuc2V0TWF4TGlzdGVuZXJzKCkgdG8gaW5jcmVhc2UgbGltaXQuJyxcbiAgICAgICAgICAgICAgICAgICAgdGhpcy5fZXZlbnRzW3R5cGVdLmxlbmd0aCk7XG4gICAgICBpZiAodHlwZW9mIGNvbnNvbGUudHJhY2UgPT09ICdmdW5jdGlvbicpIHtcbiAgICAgICAgLy8gbm90IHN1cHBvcnRlZCBpbiBJRSAxMFxuICAgICAgICBjb25zb2xlLnRyYWNlKCk7XG4gICAgICB9XG4gICAgfVxuICB9XG5cbiAgcmV0dXJuIHRoaXM7XG59O1xuXG5FdmVudEVtaXR0ZXIucHJvdG90eXBlLm9uID0gRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5hZGRMaXN0ZW5lcjtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5vbmNlID0gZnVuY3Rpb24odHlwZSwgbGlzdGVuZXIpIHtcbiAgaWYgKCFpc0Z1bmN0aW9uKGxpc3RlbmVyKSlcbiAgICB0aHJvdyBUeXBlRXJyb3IoJ2xpc3RlbmVyIG11c3QgYmUgYSBmdW5jdGlvbicpO1xuXG4gIHZhciBmaXJlZCA9IGZhbHNlO1xuXG4gIGZ1bmN0aW9uIGcoKSB7XG4gICAgdGhpcy5yZW1vdmVMaXN0ZW5lcih0eXBlLCBnKTtcblxuICAgIGlmICghZmlyZWQpIHtcbiAgICAgIGZpcmVkID0gdHJ1ZTtcbiAgICAgIGxpc3RlbmVyLmFwcGx5KHRoaXMsIGFyZ3VtZW50cyk7XG4gICAgfVxuICB9XG5cbiAgZy5saXN0ZW5lciA9IGxpc3RlbmVyO1xuICB0aGlzLm9uKHR5cGUsIGcpO1xuXG4gIHJldHVybiB0aGlzO1xufTtcblxuLy8gZW1pdHMgYSAncmVtb3ZlTGlzdGVuZXInIGV2ZW50IGlmZiB0aGUgbGlzdGVuZXIgd2FzIHJlbW92ZWRcbkV2ZW50RW1pdHRlci5wcm90b3R5cGUucmVtb3ZlTGlzdGVuZXIgPSBmdW5jdGlvbih0eXBlLCBsaXN0ZW5lcikge1xuICB2YXIgbGlzdCwgcG9zaXRpb24sIGxlbmd0aCwgaTtcblxuICBpZiAoIWlzRnVuY3Rpb24obGlzdGVuZXIpKVxuICAgIHRocm93IFR5cGVFcnJvcignbGlzdGVuZXIgbXVzdCBiZSBhIGZ1bmN0aW9uJyk7XG5cbiAgaWYgKCF0aGlzLl9ldmVudHMgfHwgIXRoaXMuX2V2ZW50c1t0eXBlXSlcbiAgICByZXR1cm4gdGhpcztcblxuICBsaXN0ID0gdGhpcy5fZXZlbnRzW3R5cGVdO1xuICBsZW5ndGggPSBsaXN0Lmxlbmd0aDtcbiAgcG9zaXRpb24gPSAtMTtcblxuICBpZiAobGlzdCA9PT0gbGlzdGVuZXIgfHxcbiAgICAgIChpc0Z1bmN0aW9uKGxpc3QubGlzdGVuZXIpICYmIGxpc3QubGlzdGVuZXIgPT09IGxpc3RlbmVyKSkge1xuICAgIGRlbGV0ZSB0aGlzLl9ldmVudHNbdHlwZV07XG4gICAgaWYgKHRoaXMuX2V2ZW50cy5yZW1vdmVMaXN0ZW5lcilcbiAgICAgIHRoaXMuZW1pdCgncmVtb3ZlTGlzdGVuZXInLCB0eXBlLCBsaXN0ZW5lcik7XG5cbiAgfSBlbHNlIGlmIChpc09iamVjdChsaXN0KSkge1xuICAgIGZvciAoaSA9IGxlbmd0aDsgaS0tID4gMDspIHtcbiAgICAgIGlmIChsaXN0W2ldID09PSBsaXN0ZW5lciB8fFxuICAgICAgICAgIChsaXN0W2ldLmxpc3RlbmVyICYmIGxpc3RbaV0ubGlzdGVuZXIgPT09IGxpc3RlbmVyKSkge1xuICAgICAgICBwb3NpdGlvbiA9IGk7XG4gICAgICAgIGJyZWFrO1xuICAgICAgfVxuICAgIH1cblxuICAgIGlmIChwb3NpdGlvbiA8IDApXG4gICAgICByZXR1cm4gdGhpcztcblxuICAgIGlmIChsaXN0Lmxlbmd0aCA9PT0gMSkge1xuICAgICAgbGlzdC5sZW5ndGggPSAwO1xuICAgICAgZGVsZXRlIHRoaXMuX2V2ZW50c1t0eXBlXTtcbiAgICB9IGVsc2Uge1xuICAgICAgbGlzdC5zcGxpY2UocG9zaXRpb24sIDEpO1xuICAgIH1cblxuICAgIGlmICh0aGlzLl9ldmVudHMucmVtb3ZlTGlzdGVuZXIpXG4gICAgICB0aGlzLmVtaXQoJ3JlbW92ZUxpc3RlbmVyJywgdHlwZSwgbGlzdGVuZXIpO1xuICB9XG5cbiAgcmV0dXJuIHRoaXM7XG59O1xuXG5FdmVudEVtaXR0ZXIucHJvdG90eXBlLnJlbW92ZUFsbExpc3RlbmVycyA9IGZ1bmN0aW9uKHR5cGUpIHtcbiAgdmFyIGtleSwgbGlzdGVuZXJzO1xuXG4gIGlmICghdGhpcy5fZXZlbnRzKVxuICAgIHJldHVybiB0aGlzO1xuXG4gIC8vIG5vdCBsaXN0ZW5pbmcgZm9yIHJlbW92ZUxpc3RlbmVyLCBubyBuZWVkIHRvIGVtaXRcbiAgaWYgKCF0aGlzLl9ldmVudHMucmVtb3ZlTGlzdGVuZXIpIHtcbiAgICBpZiAoYXJndW1lbnRzLmxlbmd0aCA9PT0gMClcbiAgICAgIHRoaXMuX2V2ZW50cyA9IHt9O1xuICAgIGVsc2UgaWYgKHRoaXMuX2V2ZW50c1t0eXBlXSlcbiAgICAgIGRlbGV0ZSB0aGlzLl9ldmVudHNbdHlwZV07XG4gICAgcmV0dXJuIHRoaXM7XG4gIH1cblxuICAvLyBlbWl0IHJlbW92ZUxpc3RlbmVyIGZvciBhbGwgbGlzdGVuZXJzIG9uIGFsbCBldmVudHNcbiAgaWYgKGFyZ3VtZW50cy5sZW5ndGggPT09IDApIHtcbiAgICBmb3IgKGtleSBpbiB0aGlzLl9ldmVudHMpIHtcbiAgICAgIGlmIChrZXkgPT09ICdyZW1vdmVMaXN0ZW5lcicpIGNvbnRpbnVlO1xuICAgICAgdGhpcy5yZW1vdmVBbGxMaXN0ZW5lcnMoa2V5KTtcbiAgICB9XG4gICAgdGhpcy5yZW1vdmVBbGxMaXN0ZW5lcnMoJ3JlbW92ZUxpc3RlbmVyJyk7XG4gICAgdGhpcy5fZXZlbnRzID0ge307XG4gICAgcmV0dXJuIHRoaXM7XG4gIH1cblxuICBsaXN0ZW5lcnMgPSB0aGlzLl9ldmVudHNbdHlwZV07XG5cbiAgaWYgKGlzRnVuY3Rpb24obGlzdGVuZXJzKSkge1xuICAgIHRoaXMucmVtb3ZlTGlzdGVuZXIodHlwZSwgbGlzdGVuZXJzKTtcbiAgfSBlbHNlIHtcbiAgICAvLyBMSUZPIG9yZGVyXG4gICAgd2hpbGUgKGxpc3RlbmVycy5sZW5ndGgpXG4gICAgICB0aGlzLnJlbW92ZUxpc3RlbmVyKHR5cGUsIGxpc3RlbmVyc1tsaXN0ZW5lcnMubGVuZ3RoIC0gMV0pO1xuICB9XG4gIGRlbGV0ZSB0aGlzLl9ldmVudHNbdHlwZV07XG5cbiAgcmV0dXJuIHRoaXM7XG59O1xuXG5FdmVudEVtaXR0ZXIucHJvdG90eXBlLmxpc3RlbmVycyA9IGZ1bmN0aW9uKHR5cGUpIHtcbiAgdmFyIHJldDtcbiAgaWYgKCF0aGlzLl9ldmVudHMgfHwgIXRoaXMuX2V2ZW50c1t0eXBlXSlcbiAgICByZXQgPSBbXTtcbiAgZWxzZSBpZiAoaXNGdW5jdGlvbih0aGlzLl9ldmVudHNbdHlwZV0pKVxuICAgIHJldCA9IFt0aGlzLl9ldmVudHNbdHlwZV1dO1xuICBlbHNlXG4gICAgcmV0ID0gdGhpcy5fZXZlbnRzW3R5cGVdLnNsaWNlKCk7XG4gIHJldHVybiByZXQ7XG59O1xuXG5FdmVudEVtaXR0ZXIubGlzdGVuZXJDb3VudCA9IGZ1bmN0aW9uKGVtaXR0ZXIsIHR5cGUpIHtcbiAgdmFyIHJldDtcbiAgaWYgKCFlbWl0dGVyLl9ldmVudHMgfHwgIWVtaXR0ZXIuX2V2ZW50c1t0eXBlXSlcbiAgICByZXQgPSAwO1xuICBlbHNlIGlmIChpc0Z1bmN0aW9uKGVtaXR0ZXIuX2V2ZW50c1t0eXBlXSkpXG4gICAgcmV0ID0gMTtcbiAgZWxzZVxuICAgIHJldCA9IGVtaXR0ZXIuX2V2ZW50c1t0eXBlXS5sZW5ndGg7XG4gIHJldHVybiByZXQ7XG59O1xuXG5mdW5jdGlvbiBpc0Z1bmN0aW9uKGFyZykge1xuICByZXR1cm4gdHlwZW9mIGFyZyA9PT0gJ2Z1bmN0aW9uJztcbn1cblxuZnVuY3Rpb24gaXNOdW1iZXIoYXJnKSB7XG4gIHJldHVybiB0eXBlb2YgYXJnID09PSAnbnVtYmVyJztcbn1cblxuZnVuY3Rpb24gaXNPYmplY3QoYXJnKSB7XG4gIHJldHVybiB0eXBlb2YgYXJnID09PSAnb2JqZWN0JyAmJiBhcmcgIT09IG51bGw7XG59XG5cbmZ1bmN0aW9uIGlzVW5kZWZpbmVkKGFyZykge1xuICByZXR1cm4gYXJnID09PSB2b2lkIDA7XG59XG5cblxuXG4vKioqKioqKioqKioqKioqKipcbiAqKiBXRUJQQUNLIEZPT1RFUlxuICoqIC4vfi9ldmVudHMvZXZlbnRzLmpzXG4gKiogbW9kdWxlIGlkID0gMjg4XG4gKiogbW9kdWxlIGNodW5rcyA9IDFcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIE5hdkxlZnRCYXIgPSByZXF1aXJlKCcuL25hdkxlZnRCYXInKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIge2FjdGlvbnMsIGdldHRlcnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvYXBwJyk7XG52YXIgU2VsZWN0Tm9kZURpYWxvZyA9IHJlcXVpcmUoJy4vc2VsZWN0Tm9kZURpYWxvZy5qc3gnKTtcblxudmFyIEFwcCA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBtaXhpbnM6IFtyZWFjdG9yLlJlYWN0TWl4aW5dLFxuXG4gIGdldERhdGFCaW5kaW5ncygpIHtcbiAgICByZXR1cm4ge1xuICAgICAgYXBwOiBnZXR0ZXJzLmFwcFN0YXRlXG4gICAgfVxuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxNb3VudCgpe1xuICAgIGFjdGlvbnMuaW5pdEFwcCgpO1xuICAgIHRoaXMucmVmcmVzaEludGVydmFsID0gc2V0SW50ZXJ2YWwoYWN0aW9ucy5mZXRjaE5vZGVzQW5kU2Vzc2lvbnMsIDM1MDAwKTtcbiAgfSxcblxuICBjb21wb25lbnRXaWxsVW5tb3VudDogZnVuY3Rpb24oKSB7XG4gICAgY2xlYXJJbnRlcnZhbCh0aGlzLnJlZnJlc2hJbnRlcnZhbCk7XG4gIH0sXG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICBpZih0aGlzLnN0YXRlLmFwcC5pc0luaXRpYWxpemluZyl7XG4gICAgICByZXR1cm4gbnVsbDtcbiAgICB9XG5cbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtdGxwdCBncnYtZmxleCBncnYtZmxleC1yb3dcIj5cbiAgICAgICAgPFNlbGVjdE5vZGVEaWFsb2cvPlxuICAgICAgICB7dGhpcy5wcm9wcy5DdXJyZW50U2Vzc2lvbkhvc3R9XG4gICAgICAgIDxOYXZMZWZ0QmFyLz5cbiAgICAgICAge3RoaXMucHJvcHMuY2hpbGRyZW59XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KVxuXG5tb2R1bGUuZXhwb3J0cyA9IEFwcDtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2FwcC5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHtnZXR0ZXJzLCBhY3Rpb25zfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2FjdGl2ZVRlcm1pbmFsLycpO1xudmFyIFR0eSA9IHJlcXVpcmUoJ2FwcC9jb21tb24vdHR5Jyk7XG52YXIgVHR5VGVybWluYWwgPSByZXF1aXJlKCcuLy4uL3Rlcm1pbmFsLmpzeCcpO1xudmFyIEV2ZW50U3RyZWFtZXIgPSByZXF1aXJlKCcuL2V2ZW50U3RyZWFtZXIuanN4Jyk7XG52YXIgU2Vzc2lvbkxlZnRQYW5lbCA9IHJlcXVpcmUoJy4vc2Vzc2lvbkxlZnRQYW5lbCcpO1xudmFyIHtzaG93U2VsZWN0Tm9kZURpYWxvZywgY2xvc2VTZWxlY3ROb2RlRGlhbG9nfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2RpYWxvZ3MvYWN0aW9ucycpO1xudmFyIFNlbGVjdE5vZGVEaWFsb2cgPSByZXF1aXJlKCcuLy4uL3NlbGVjdE5vZGVEaWFsb2cuanN4Jyk7XG5cbnZhciBBY3RpdmVTZXNzaW9uID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50KCl7XG4gICAgY2xvc2VTZWxlY3ROb2RlRGlhbG9nKCk7XG4gIH0sXG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICBsZXQge3NlcnZlcklwLCBsb2dpbiwgcGFydGllc30gPSB0aGlzLnByb3BzLmFjdGl2ZVNlc3Npb247XG4gICAgbGV0IHNlcnZlckxhYmVsVGV4dCA9IGAke2xvZ2lufUAke3NlcnZlcklwfWA7XG5cbiAgICBpZighc2VydmVySXApe1xuICAgICAgc2VydmVyTGFiZWxUZXh0ID0gJyc7XG4gICAgfVxuXG4gICAgcmV0dXJuIChcbiAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtY3VycmVudC1zZXNzaW9uXCI+XG4gICAgICAgPFNlc3Npb25MZWZ0UGFuZWwgcGFydGllcz17cGFydGllc30vPlxuICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWN1cnJlbnQtc2Vzc2lvbi1zZXJ2ZXItaW5mb1wiPiAgICAgIFxuICAgICAgICAgPGgzPntzZXJ2ZXJMYWJlbFRleHR9PC9oMz5cbiAgICAgICA8L2Rpdj5cbiAgICAgICA8VHR5Q29ubmVjdGlvbiB7Li4udGhpcy5wcm9wcy5hY3RpdmVTZXNzaW9ufSAvPlxuICAgICA8L2Rpdj5cbiAgICAgKTtcbiAgfVxufSk7XG5cbnZhciBUdHlDb25uZWN0aW9uID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICB0aGlzLnR0eSA9IG5ldyBUdHkodGhpcy5wcm9wcylcbiAgICB0aGlzLnR0eS5vbignb3BlbicsICgpPT4gdGhpcy5zZXRTdGF0ZSh7IC4uLnRoaXMuc3RhdGUsIGlzQ29ubmVjdGVkOiB0cnVlIH0pKTtcblxuICAgIHZhciB7c2VydmVySWQsIGxvZ2lufSA9IHRoaXMucHJvcHM7XG4gICAgcmV0dXJuIHtzZXJ2ZXJJZCwgbG9naW4sIGlzQ29ubmVjdGVkOiBmYWxzZX07XG4gIH0sXG5cbiAgY29tcG9uZW50RGlkTW91bnQoKXtcbiAgICAvLyB0ZW1wb3JhcnkgaGFja1xuICAgIFNlbGVjdE5vZGVEaWFsb2cub25TZXJ2ZXJDaGFuZ2VDYWxsQmFjayA9IHRoaXMuY29tcG9uZW50V2lsbFJlY2VpdmVQcm9wcy5iaW5kKHRoaXMpO1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50KCkge1xuICAgIFNlbGVjdE5vZGVEaWFsb2cub25TZXJ2ZXJDaGFuZ2VDYWxsQmFjayA9IG51bGw7XG4gICAgdGhpcy50dHkuZGlzY29ubmVjdCgpO1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxSZWNlaXZlUHJvcHMobmV4dFByb3BzKXtcbiAgICB2YXIge3NlcnZlcklkfSA9IG5leHRQcm9wcztcbiAgICBpZihzZXJ2ZXJJZCAmJiBzZXJ2ZXJJZCAhPT0gdGhpcy5zdGF0ZS5zZXJ2ZXJJZCl7XG4gICAgICB0aGlzLnR0eS5yZWNvbm5lY3Qoe3NlcnZlcklkfSk7XG4gICAgICB0aGlzLnJlZnMudHR5Q21udEluc3RhbmNlLnRlcm0uZm9jdXMoKTtcbiAgICAgIHRoaXMuc2V0U3RhdGUoey4uLnRoaXMuc3RhdGUsIHNlcnZlcklkIH0pO1xuICAgIH1cbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgc3R5bGU9e3toZWlnaHQ6ICcxMDAlJ319PlxuICAgICAgICA8VHR5VGVybWluYWwgcmVmPVwidHR5Q21udEluc3RhbmNlXCIgdHR5PXt0aGlzLnR0eX0gY29scz17dGhpcy5wcm9wcy5jb2xzfSByb3dzPXt0aGlzLnByb3BzLnJvd3N9IC8+XG4gICAgICAgIHsgdGhpcy5zdGF0ZS5pc0Nvbm5lY3RlZCA/IDxFdmVudFN0cmVhbWVyIHNpZD17dGhpcy5wcm9wcy5zaWR9Lz4gOiBudWxsIH1cbiAgICAgIDwvZGl2PlxuICAgIClcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gQWN0aXZlU2Vzc2lvbjtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2N1cnJlbnRTZXNzaW9uL2FjdGl2ZVNlc3Npb24uanN4XG4gKiovIiwidmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcbnZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgc2Vzc2lvbiA9IHJlcXVpcmUoJ2FwcC9zZXNzaW9uJyk7XG52YXIge3VwZGF0ZVNlc3Npb259ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvc2Vzc2lvbnMvYWN0aW9ucycpO1xuXG52YXIgRXZlbnRTdHJlYW1lciA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgY29tcG9uZW50RGlkTW91bnQoKSB7XG4gICAgbGV0IHtzaWR9ID0gdGhpcy5wcm9wcztcbiAgICBsZXQge3Rva2VufSA9IHNlc3Npb24uZ2V0VXNlckRhdGEoKTtcbiAgICBsZXQgY29ublN0ciA9IGNmZy5hcGkuZ2V0RXZlbnRTdHJlYW1Db25uU3RyKHRva2VuLCBzaWQpO1xuXG4gICAgdGhpcy5zb2NrZXQgPSBuZXcgV2ViU29ja2V0KGNvbm5TdHIsICdwcm90bycpO1xuICAgIHRoaXMuc29ja2V0Lm9ubWVzc2FnZSA9IChldmVudCkgPT4ge1xuICAgICAgdHJ5XG4gICAgICB7XG4gICAgICAgIGxldCBqc29uID0gSlNPTi5wYXJzZShldmVudC5kYXRhKTtcbiAgICAgICAgdXBkYXRlU2Vzc2lvbihqc29uLnNlc3Npb24pO1xuICAgICAgfVxuICAgICAgY2F0Y2goZXJyKXtcbiAgICAgICAgY29uc29sZS5sb2coJ2ZhaWxlZCB0byBwYXJzZSBldmVudCBzdHJlYW0gZGF0YScpO1xuICAgICAgfVxuXG4gICAgfTtcbiAgICB0aGlzLnNvY2tldC5vbmNsb3NlID0gKCkgPT4ge307XG4gIH0sXG5cbiAgY29tcG9uZW50V2lsbFVubW91bnQoKSB7XG4gICAgdGhpcy5zb2NrZXQuY2xvc2UoKTtcbiAgfSxcblxuICBzaG91bGRDb21wb25lbnRVcGRhdGUoKSB7XG4gICAgcmV0dXJuIGZhbHNlO1xuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICByZXR1cm4gbnVsbDtcbiAgfVxufSk7XG5cbmV4cG9ydCBkZWZhdWx0IEV2ZW50U3RyZWFtZXI7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9jdXJyZW50U2Vzc2lvbi9ldmVudFN0cmVhbWVyLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIge2dldHRlcnMsIGFjdGlvbnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvYWN0aXZlVGVybWluYWwvJyk7XG52YXIgU2Vzc2lvblBsYXllciA9IHJlcXVpcmUoJy4vc2Vzc2lvblBsYXllci5qc3gnKTtcbnZhciBBY3RpdmVTZXNzaW9uID0gcmVxdWlyZSgnLi9hY3RpdmVTZXNzaW9uLmpzeCcpO1xuXG52YXIgQ3VycmVudFNlc3Npb25Ib3N0ID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW3JlYWN0b3IuUmVhY3RNaXhpbl0sXG5cbiAgZ2V0RGF0YUJpbmRpbmdzKCkge1xuICAgIHJldHVybiB7XG4gICAgICBjdXJyZW50U2Vzc2lvbjogZ2V0dGVycy5hY3RpdmVTZXNzaW9uXG4gICAgfVxuICB9LFxuXG4gIGNvbXBvbmVudERpZE1vdW50KCl7XG4gICAgdmFyIHsgc2lkIH0gPSB0aGlzLnByb3BzLnBhcmFtcztcbiAgICBpZighdGhpcy5zdGF0ZS5jdXJyZW50U2Vzc2lvbil7XG4gICAgICBhY3Rpb25zLm9wZW5TZXNzaW9uKHNpZCk7XG4gICAgfVxuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgdmFyIGN1cnJlbnRTZXNzaW9uID0gdGhpcy5zdGF0ZS5jdXJyZW50U2Vzc2lvbjtcbiAgICBpZighY3VycmVudFNlc3Npb24pe1xuICAgICAgcmV0dXJuIG51bGw7XG4gICAgfVxuXG4gICAgaWYoY3VycmVudFNlc3Npb24uaXNOZXdTZXNzaW9uIHx8IGN1cnJlbnRTZXNzaW9uLmFjdGl2ZSl7XG4gICAgICByZXR1cm4gPEFjdGl2ZVNlc3Npb24gYWN0aXZlU2Vzc2lvbj17Y3VycmVudFNlc3Npb259Lz47XG4gICAgfVxuXG4gICAgcmV0dXJuIDxTZXNzaW9uUGxheWVyIGFjdGl2ZVNlc3Npb249e2N1cnJlbnRTZXNzaW9ufS8+O1xuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBDdXJyZW50U2Vzc2lvbkhvc3Q7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9jdXJyZW50U2Vzc2lvbi9tYWluLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgUmVhY3RTbGlkZXIgPSByZXF1aXJlKCdyZWFjdC1zbGlkZXInKTtcbnZhciBUdHlQbGF5ZXIgPSByZXF1aXJlKCdhcHAvY29tbW9uL3R0eVBsYXllcicpXG52YXIgVHR5VGVybWluYWwgPSByZXF1aXJlKCcuLy4uL3Rlcm1pbmFsLmpzeCcpO1xudmFyIFNlc3Npb25MZWZ0UGFuZWwgPSByZXF1aXJlKCcuL3Nlc3Npb25MZWZ0UGFuZWwnKTtcblxudmFyIFNlc3Npb25QbGF5ZXIgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIGNhbGN1bGF0ZVN0YXRlKCl7XG4gICAgcmV0dXJuIHtcbiAgICAgIGxlbmd0aDogdGhpcy50dHkubGVuZ3RoLFxuICAgICAgbWluOiAxLFxuICAgICAgaXNQbGF5aW5nOiB0aGlzLnR0eS5pc1BsYXlpbmcsXG4gICAgICBjdXJyZW50OiB0aGlzLnR0eS5jdXJyZW50LFxuICAgICAgY2FuUGxheTogdGhpcy50dHkubGVuZ3RoID4gMVxuICAgIH07XG4gIH0sXG5cbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHZhciBzaWQgPSB0aGlzLnByb3BzLmFjdGl2ZVNlc3Npb24uc2lkO1xuICAgIHRoaXMudHR5ID0gbmV3IFR0eVBsYXllcih7c2lkfSk7XG4gICAgcmV0dXJuIHRoaXMuY2FsY3VsYXRlU3RhdGUoKTtcbiAgfSxcblxuICBjb21wb25lbnRXaWxsVW5tb3VudCgpIHtcbiAgICB0aGlzLnR0eS5zdG9wKCk7XG4gICAgdGhpcy50dHkucmVtb3ZlQWxsTGlzdGVuZXJzKCk7XG4gIH0sXG5cbiAgY29tcG9uZW50RGlkTW91bnQoKSB7XG4gICAgdGhpcy50dHkub24oJ2NoYW5nZScsICgpPT57XG4gICAgICB2YXIgbmV3U3RhdGUgPSB0aGlzLmNhbGN1bGF0ZVN0YXRlKCk7XG4gICAgICB0aGlzLnNldFN0YXRlKG5ld1N0YXRlKTtcbiAgICB9KTtcbiAgfSxcblxuICB0b2dnbGVQbGF5U3RvcCgpe1xuICAgIGlmKHRoaXMuc3RhdGUuaXNQbGF5aW5nKXtcbiAgICAgIHRoaXMudHR5LnN0b3AoKTtcbiAgICB9ZWxzZXtcbiAgICAgIHRoaXMudHR5LnBsYXkoKTtcbiAgICB9XG4gIH0sXG5cbiAgbW92ZSh2YWx1ZSl7XG4gICAgdGhpcy50dHkubW92ZSh2YWx1ZSk7XG4gIH0sXG5cbiAgb25CZWZvcmVDaGFuZ2UoKXtcbiAgICB0aGlzLnR0eS5zdG9wKCk7XG4gIH0sXG5cbiAgb25BZnRlckNoYW5nZSh2YWx1ZSl7XG4gICAgdGhpcy50dHkucGxheSgpO1xuICAgIHRoaXMudHR5Lm1vdmUodmFsdWUpO1xuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgdmFyIHtpc1BsYXlpbmd9ID0gdGhpcy5zdGF0ZTtcblxuICAgIHJldHVybiAoXG4gICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWN1cnJlbnQtc2Vzc2lvbiBncnYtc2Vzc2lvbi1wbGF5ZXJcIj5cbiAgICAgICA8U2Vzc2lvbkxlZnRQYW5lbC8+XG4gICAgICAgPFR0eVRlcm1pbmFsIHJlZj1cInRlcm1cIiB0dHk9e3RoaXMudHR5fSBjb2xzPVwiNVwiIHJvd3M9XCI1XCIgLz5cbiAgICAgICA8UmVhY3RTbGlkZXJcbiAgICAgICAgICBtaW49e3RoaXMuc3RhdGUubWlufVxuICAgICAgICAgIG1heD17dGhpcy5zdGF0ZS5sZW5ndGh9XG4gICAgICAgICAgdmFsdWU9e3RoaXMuc3RhdGUuY3VycmVudH0gICAgXG4gICAgICAgICAgb25BZnRlckNoYW5nZT17dGhpcy5vbkFmdGVyQ2hhbmdlfVxuICAgICAgICAgIG9uQmVmb3JlQ2hhbmdlPXt0aGlzLm9uQmVmb3JlQ2hhbmdlfVxuICAgICAgICAgIGRlZmF1bHRWYWx1ZT17MX1cbiAgICAgICAgICB3aXRoQmFyc1xuICAgICAgICAgIGNsYXNzTmFtZT1cImdydi1zbGlkZXJcIj5cbiAgICAgICA8L1JlYWN0U2xpZGVyPlxuICAgICAgIDxidXR0b24gY2xhc3NOYW1lPVwiYnRuXCIgb25DbGljaz17dGhpcy50b2dnbGVQbGF5U3RvcH0+XG4gICAgICAgICB7IGlzUGxheWluZyA/IDxpIGNsYXNzTmFtZT1cImZhIGZhLXN0b3BcIj48L2k+IDogIDxpIGNsYXNzTmFtZT1cImZhIGZhLXBsYXlcIj48L2k+IH1cbiAgICAgICA8L2J1dHRvbj5cbiAgICAgPC9kaXY+XG4gICAgICk7XG4gIH1cbn0pO1xuXG5leHBvcnQgZGVmYXVsdCBTZXNzaW9uUGxheWVyO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvY3VycmVudFNlc3Npb24vc2Vzc2lvblBsYXllci5qc3hcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cy5BcHAgPSByZXF1aXJlKCcuL2FwcC5qc3gnKTtcbm1vZHVsZS5leHBvcnRzLkxvZ2luID0gcmVxdWlyZSgnLi9sb2dpbi5qc3gnKTtcbm1vZHVsZS5leHBvcnRzLk5ld1VzZXIgPSByZXF1aXJlKCcuL25ld1VzZXIuanN4Jyk7XG5tb2R1bGUuZXhwb3J0cy5Ob2RlcyA9IHJlcXVpcmUoJy4vbm9kZXMvbWFpbi5qc3gnKTtcbm1vZHVsZS5leHBvcnRzLlNlc3Npb25zID0gcmVxdWlyZSgnLi9zZXNzaW9ucy9tYWluLmpzeCcpO1xubW9kdWxlLmV4cG9ydHMuQ3VycmVudFNlc3Npb25Ib3N0ID0gcmVxdWlyZSgnLi9jdXJyZW50U2Vzc2lvbi9tYWluLmpzeCcpO1xubW9kdWxlLmV4cG9ydHMuTm90Rm91bmQgPSByZXF1aXJlKCcuL2Vycm9yUGFnZS5qc3gnKS5Ob3RGb3VuZDtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2luZGV4LmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgJCA9IHJlcXVpcmUoJ2pRdWVyeScpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIExpbmtlZFN0YXRlTWl4aW4gPSByZXF1aXJlKCdyZWFjdC1hZGRvbnMtbGlua2VkLXN0YXRlLW1peGluJyk7XG52YXIge2FjdGlvbnMsIGdldHRlcnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvdXNlcicpO1xudmFyIEdvb2dsZUF1dGhJbmZvID0gcmVxdWlyZSgnLi9nb29nbGVBdXRoTG9nbycpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxudmFyIExvZ2luSW5wdXRGb3JtID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW0xpbmtlZFN0YXRlTWl4aW5dLFxuXG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4ge1xuICAgICAgdXNlcjogJycsXG4gICAgICBwYXNzd29yZDogJycsXG4gICAgICB0b2tlbjogJydcbiAgICB9XG4gIH0sXG5cbiAgb25DbGljazogZnVuY3Rpb24oZSkge1xuICAgIGUucHJldmVudERlZmF1bHQoKTtcbiAgICBpZiAodGhpcy5pc1ZhbGlkKCkpIHtcbiAgICAgIHRoaXMucHJvcHMub25DbGljayh0aGlzLnN0YXRlKTtcbiAgICB9XG4gIH0sXG5cbiAgaXNWYWxpZDogZnVuY3Rpb24oKSB7XG4gICAgdmFyICRmb3JtID0gJCh0aGlzLnJlZnMuZm9ybSk7XG4gICAgcmV0dXJuICRmb3JtLmxlbmd0aCA9PT0gMCB8fCAkZm9ybS52YWxpZCgpO1xuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICBsZXQge2lzUHJvY2Vzc2luZywgaXNGYWlsZWQsIG1lc3NhZ2UgfSA9IHRoaXMucHJvcHMuYXR0ZW1wO1xuXG4gICAgcmV0dXJuIChcbiAgICAgIDxmb3JtIHJlZj1cImZvcm1cIiBjbGFzc05hbWU9XCJncnYtbG9naW4taW5wdXQtZm9ybVwiPlxuICAgICAgICA8aDM+IFdlbGNvbWUgdG8gVGVsZXBvcnQgPC9oMz5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImZvcm0tZ3JvdXBcIj5cbiAgICAgICAgICAgIDxpbnB1dCBhdXRvRm9jdXMgdmFsdWVMaW5rPXt0aGlzLmxpbmtTdGF0ZSgndXNlcicpfSBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2wgcmVxdWlyZWRcIiBwbGFjZWhvbGRlcj1cIlVzZXIgbmFtZVwiIG5hbWU9XCJ1c2VyTmFtZVwiIC8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXQgdmFsdWVMaW5rPXt0aGlzLmxpbmtTdGF0ZSgncGFzc3dvcmQnKX0gdHlwZT1cInBhc3N3b3JkXCIgbmFtZT1cInBhc3N3b3JkXCIgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sIHJlcXVpcmVkXCIgcGxhY2Vob2xkZXI9XCJQYXNzd29yZFwiLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImZvcm0tZ3JvdXBcIj5cbiAgICAgICAgICAgIDxpbnB1dCB2YWx1ZUxpbms9e3RoaXMubGlua1N0YXRlKCd0b2tlbicpfSBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2wgcmVxdWlyZWRcIiBuYW1lPVwidG9rZW5cIiBwbGFjZWhvbGRlcj1cIlR3byBmYWN0b3IgdG9rZW4gKEdvb2dsZSBBdXRoZW50aWNhdG9yKVwiLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8YnV0dG9uIG9uQ2xpY2s9e3RoaXMub25DbGlja30gZGlzYWJsZWQ9e2lzUHJvY2Vzc2luZ30gdHlwZT1cInN1Ym1pdFwiIGNsYXNzTmFtZT1cImJ0biBidG4tcHJpbWFyeSBibG9jayBmdWxsLXdpZHRoIG0tYlwiPkxvZ2luPC9idXR0b24+XG4gICAgICAgICAgeyBpc0ZhaWxlZCA/ICg8bGFiZWwgY2xhc3NOYW1lPVwiZXJyb3JcIj57bWVzc2FnZX08L2xhYmVsPikgOiBudWxsIH1cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Zvcm0+XG4gICAgKTtcbiAgfVxufSlcblxudmFyIExvZ2luID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW3JlYWN0b3IuUmVhY3RNaXhpbl0sXG5cbiAgZ2V0RGF0YUJpbmRpbmdzKCkge1xuICAgIHJldHVybiB7XG4gICAgICBhdHRlbXA6IGdldHRlcnMubG9naW5BdHRlbXBcbiAgICB9XG4gIH0sXG5cbiAgb25DbGljayhpbnB1dERhdGEpe1xuICAgIHZhciBsb2MgPSB0aGlzLnByb3BzLmxvY2F0aW9uO1xuICAgIHZhciByZWRpcmVjdCA9IGNmZy5yb3V0ZXMuYXBwO1xuXG4gICAgaWYobG9jLnN0YXRlICYmIGxvYy5zdGF0ZS5yZWRpcmVjdFRvKXtcbiAgICAgIHJlZGlyZWN0ID0gbG9jLnN0YXRlLnJlZGlyZWN0VG87XG4gICAgfVxuXG4gICAgYWN0aW9ucy5sb2dpbihpbnB1dERhdGEsIHJlZGlyZWN0KTtcbiAgfSxcblxuICByZW5kZXIoKSB7ICAgIFxuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1sb2dpbiB0ZXh0LWNlbnRlclwiPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1sb2dvLXRwcnRcIj48L2Rpdj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtY29udGVudCBncnYtZmxleFwiPlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWZsZXgtY29sdW1uXCI+XG4gICAgICAgICAgICA8TG9naW5JbnB1dEZvcm0gYXR0ZW1wPXt0aGlzLnN0YXRlLmF0dGVtcH0gb25DbGljaz17dGhpcy5vbkNsaWNrfS8+XG4gICAgICAgICAgICA8R29vZ2xlQXV0aEluZm8vPlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbG9naW4taW5mb1wiPlxuICAgICAgICAgICAgICA8aSBjbGFzc05hbWU9XCJmYSBmYS1xdWVzdGlvblwiPjwvaT5cbiAgICAgICAgICAgICAgPHN0cm9uZz5OZXcgQWNjb3VudCBvciBmb3Jnb3QgcGFzc3dvcmQ/PC9zdHJvbmc+XG4gICAgICAgICAgICAgIDxkaXY+QXNrIGZvciBhc3Npc3RhbmNlIGZyb20geW91ciBDb21wYW55IGFkbWluaXN0cmF0b3I8L2Rpdj5cbiAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IExvZ2luO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvbG9naW4uanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciB7IFJvdXRlciwgSW5kZXhMaW5rLCBIaXN0b3J5IH0gPSByZXF1aXJlKCdyZWFjdC1yb3V0ZXInKTtcbnZhciBnZXR0ZXJzID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvdXNlci9nZXR0ZXJzJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xuXG52YXIgbWVudUl0ZW1zID0gW1xuICB7aWNvbjogJ2ZhIGZhLWNvZ3MnLCB0bzogY2ZnLnJvdXRlcy5ub2RlcywgdGl0bGU6ICdOb2Rlcyd9LFxuICB7aWNvbjogJ2ZhIGZhLXNpdGVtYXAnLCB0bzogY2ZnLnJvdXRlcy5zZXNzaW9ucywgdGl0bGU6ICdTZXNzaW9ucyd9XG5dO1xuXG52YXIgTmF2TGVmdEJhciA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICByZW5kZXI6IGZ1bmN0aW9uKCl7XG4gICAgdmFyIGl0ZW1zID0gbWVudUl0ZW1zLm1hcCgoaSwgaW5kZXgpPT57XG4gICAgICB2YXIgY2xhc3NOYW1lID0gdGhpcy5jb250ZXh0LnJvdXRlci5pc0FjdGl2ZShpLnRvKSA/ICdhY3RpdmUnIDogJyc7XG4gICAgICByZXR1cm4gKFxuICAgICAgICA8bGkga2V5PXtpbmRleH0gY2xhc3NOYW1lPXtjbGFzc05hbWV9IHRpdGxlPXtpLnRpdGxlfT5cbiAgICAgICAgICA8SW5kZXhMaW5rIHRvPXtpLnRvfT5cbiAgICAgICAgICAgIDxpIGNsYXNzTmFtZT17aS5pY29ufSAvPlxuICAgICAgICAgIDwvSW5kZXhMaW5rPlxuICAgICAgICA8L2xpPlxuICAgICAgKTtcbiAgICB9KTtcblxuICAgIGl0ZW1zLnB1c2goKFxuICAgICAgPGxpIGtleT17aXRlbXMubGVuZ3RofSB0aXRsZT1cImhlbHBcIj5cbiAgICAgICAgPGEgaHJlZj17Y2ZnLmhlbHBVcmx9IHRhcmdldD1cIl9ibGFua1wiPlxuICAgICAgICAgIDxpIGNsYXNzTmFtZT1cImZhIGZhLXF1ZXN0aW9uXCIgLz5cbiAgICAgICAgPC9hPlxuICAgICAgPC9saT4pKTtcblxuICAgIGl0ZW1zLnB1c2goKFxuICAgICAgPGxpIGtleT17aXRlbXMubGVuZ3RofSB0aXRsZT1cImxvZ291dFwiPlxuICAgICAgICA8YSBocmVmPXtjZmcucm91dGVzLmxvZ291dH0+XG4gICAgICAgICAgPGkgY2xhc3NOYW1lPVwiZmEgZmEtc2lnbi1vdXRcIj48L2k+XG4gICAgICAgIDwvYT5cbiAgICAgIDwvbGk+XG4gICAgKSk7XG5cbiAgICByZXR1cm4gKFxuICAgICAgPG5hdiBjbGFzc05hbWU9J2dydi1uYXYgbmF2YmFyLWRlZmF1bHQnIHJvbGU9J25hdmlnYXRpb24nPlxuICAgICAgICA8dWwgY2xhc3NOYW1lPSduYXYgdGV4dC1jZW50ZXInIGlkPSdzaWRlLW1lbnUnPlxuICAgICAgICAgIDxsaSB0aXRsZT1cImN1cnJlbnQgdXNlclwiPjxkaXYgY2xhc3NOYW1lPVwiZ3J2LWNpcmNsZSB0ZXh0LXVwcGVyY2FzZVwiPjxzcGFuPntnZXRVc2VyTmFtZUxldHRlcigpfTwvc3Bhbj48L2Rpdj48L2xpPlxuICAgICAgICAgIHtpdGVtc31cbiAgICAgICAgPC91bD5cbiAgICAgIDwvbmF2PlxuICAgICk7XG4gIH1cbn0pO1xuXG5OYXZMZWZ0QmFyLmNvbnRleHRUeXBlcyA9IHtcbiAgcm91dGVyOiBSZWFjdC5Qcm9wVHlwZXMub2JqZWN0LmlzUmVxdWlyZWRcbn1cblxuZnVuY3Rpb24gZ2V0VXNlck5hbWVMZXR0ZXIoKXtcbiAgdmFyIHtzaG9ydERpc3BsYXlOYW1lfSA9IHJlYWN0b3IuZXZhbHVhdGUoZ2V0dGVycy51c2VyKTtcbiAgcmV0dXJuIHNob3J0RGlzcGxheU5hbWU7XG59XG5cbm1vZHVsZS5leHBvcnRzID0gTmF2TGVmdEJhcjtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL25hdkxlZnRCYXIuanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciAkID0gcmVxdWlyZSgnalF1ZXJ5Jyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIge2FjdGlvbnMsIGdldHRlcnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvaW52aXRlJyk7XG52YXIgdXNlck1vZHVsZSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3VzZXInKTtcbnZhciBMaW5rZWRTdGF0ZU1peGluID0gcmVxdWlyZSgncmVhY3QtYWRkb25zLWxpbmtlZC1zdGF0ZS1taXhpbicpO1xudmFyIEdvb2dsZUF1dGhJbmZvID0gcmVxdWlyZSgnLi9nb29nbGVBdXRoTG9nbycpO1xudmFyIHtFeHBpcmVkSW52aXRlfSA9IHJlcXVpcmUoJy4vZXJyb3JQYWdlJyk7XG5cbnZhciBJbnZpdGVJbnB1dEZvcm0gPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbTGlua2VkU3RhdGVNaXhpbl0sXG5cbiAgY29tcG9uZW50RGlkTW91bnQoKXtcbiAgICAkKHRoaXMucmVmcy5mb3JtKS52YWxpZGF0ZSh7XG4gICAgICBydWxlczp7XG4gICAgICAgIHBhc3N3b3JkOntcbiAgICAgICAgICBtaW5sZW5ndGg6IDYsXG4gICAgICAgICAgcmVxdWlyZWQ6IHRydWVcbiAgICAgICAgfSxcbiAgICAgICAgcGFzc3dvcmRDb25maXJtZWQ6e1xuICAgICAgICAgIHJlcXVpcmVkOiB0cnVlLFxuICAgICAgICAgIGVxdWFsVG86IHRoaXMucmVmcy5wYXNzd29yZFxuICAgICAgICB9XG4gICAgICB9LFxuXG4gICAgICBtZXNzYWdlczoge1xuICBcdFx0XHRwYXNzd29yZENvbmZpcm1lZDoge1xuICBcdFx0XHRcdG1pbmxlbmd0aDogJC52YWxpZGF0b3IuZm9ybWF0KCdFbnRlciBhdCBsZWFzdCB7MH0gY2hhcmFjdGVycycpLFxuICBcdFx0XHRcdGVxdWFsVG86ICdFbnRlciB0aGUgc2FtZSBwYXNzd29yZCBhcyBhYm92ZSdcbiAgXHRcdFx0fVxuICAgICAgfVxuICAgIH0pXG4gIH0sXG5cbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB7XG4gICAgICBuYW1lOiB0aGlzLnByb3BzLmludml0ZS51c2VyLFxuICAgICAgcHN3OiAnJyxcbiAgICAgIHBzd0NvbmZpcm1lZDogJycsXG4gICAgICB0b2tlbjogJydcbiAgICB9XG4gIH0sXG5cbiAgb25DbGljayhlKSB7XG4gICAgZS5wcmV2ZW50RGVmYXVsdCgpO1xuICAgIGlmICh0aGlzLmlzVmFsaWQoKSkge1xuICAgICAgdXNlck1vZHVsZS5hY3Rpb25zLnNpZ25VcCh7XG4gICAgICAgIG5hbWU6IHRoaXMuc3RhdGUubmFtZSxcbiAgICAgICAgcHN3OiB0aGlzLnN0YXRlLnBzdyxcbiAgICAgICAgdG9rZW46IHRoaXMuc3RhdGUudG9rZW4sXG4gICAgICAgIGludml0ZVRva2VuOiB0aGlzLnByb3BzLmludml0ZS5pbnZpdGVfdG9rZW59KTtcbiAgICB9XG4gIH0sXG5cbiAgaXNWYWxpZCgpIHtcbiAgICB2YXIgJGZvcm0gPSAkKHRoaXMucmVmcy5mb3JtKTtcbiAgICByZXR1cm4gJGZvcm0ubGVuZ3RoID09PSAwIHx8ICRmb3JtLnZhbGlkKCk7XG4gIH0sXG5cbiAgcmVuZGVyKCkge1xuICAgIGxldCB7aXNQcm9jZXNzaW5nLCBpc0ZhaWxlZCwgbWVzc2FnZSB9ID0gdGhpcy5wcm9wcy5hdHRlbXA7XG4gICAgcmV0dXJuIChcbiAgICAgIDxmb3JtIHJlZj1cImZvcm1cIiBjbGFzc05hbWU9XCJncnYtaW52aXRlLWlucHV0LWZvcm1cIj5cbiAgICAgICAgPGgzPiBHZXQgc3RhcnRlZCB3aXRoIFRlbGVwb3J0IDwvaDM+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiXCI+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXRcbiAgICAgICAgICAgICAgdmFsdWVMaW5rPXt0aGlzLmxpbmtTdGF0ZSgnbmFtZScpfVxuICAgICAgICAgICAgICBuYW1lPVwidXNlck5hbWVcIlxuICAgICAgICAgICAgICBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2wgcmVxdWlyZWRcIlxuICAgICAgICAgICAgICBwbGFjZWhvbGRlcj1cIlVzZXIgbmFtZVwiLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImZvcm0tZ3JvdXBcIj5cbiAgICAgICAgICAgIDxpbnB1dFxuICAgICAgICAgICAgICBhdXRvRm9jdXNcbiAgICAgICAgICAgICAgdmFsdWVMaW5rPXt0aGlzLmxpbmtTdGF0ZSgncHN3Jyl9XG4gICAgICAgICAgICAgIHJlZj1cInBhc3N3b3JkXCJcbiAgICAgICAgICAgICAgdHlwZT1cInBhc3N3b3JkXCJcbiAgICAgICAgICAgICAgbmFtZT1cInBhc3N3b3JkXCJcbiAgICAgICAgICAgICAgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sXCJcbiAgICAgICAgICAgICAgcGxhY2Vob2xkZXI9XCJQYXNzd29yZFwiIC8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXRcbiAgICAgICAgICAgICAgdmFsdWVMaW5rPXt0aGlzLmxpbmtTdGF0ZSgncHN3Q29uZmlybWVkJyl9XG4gICAgICAgICAgICAgIHR5cGU9XCJwYXNzd29yZFwiXG4gICAgICAgICAgICAgIG5hbWU9XCJwYXNzd29yZENvbmZpcm1lZFwiXG4gICAgICAgICAgICAgIGNsYXNzTmFtZT1cImZvcm0tY29udHJvbFwiXG4gICAgICAgICAgICAgIHBsYWNlaG9sZGVyPVwiUGFzc3dvcmQgY29uZmlybVwiLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImZvcm0tZ3JvdXBcIj5cbiAgICAgICAgICAgIDxpbnB1dFxuICAgICAgICAgICAgICBuYW1lPVwidG9rZW5cIlxuICAgICAgICAgICAgICB2YWx1ZUxpbms9e3RoaXMubGlua1N0YXRlKCd0b2tlbicpfVxuICAgICAgICAgICAgICBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2wgcmVxdWlyZWRcIlxuICAgICAgICAgICAgICBwbGFjZWhvbGRlcj1cIlR3byBmYWN0b3IgdG9rZW4gKEdvb2dsZSBBdXRoZW50aWNhdG9yKVwiIC8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGJ1dHRvbiB0eXBlPVwic3VibWl0XCIgZGlzYWJsZWQ9e2lzUHJvY2Vzc2luZ30gY2xhc3NOYW1lPVwiYnRuIGJ0bi1wcmltYXJ5IGJsb2NrIGZ1bGwtd2lkdGggbS1iXCIgb25DbGljaz17dGhpcy5vbkNsaWNrfSA+U2lnbiB1cDwvYnV0dG9uPlxuICAgICAgICAgIHsgaXNGYWlsZWQgPyAoPGxhYmVsIGNsYXNzTmFtZT1cImVycm9yXCI+e21lc3NhZ2V9PC9sYWJlbD4pIDogbnVsbCB9XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9mb3JtPlxuICAgICk7XG4gIH1cbn0pXG5cbnZhciBJbnZpdGUgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIGludml0ZTogZ2V0dGVycy5pbnZpdGUsXG4gICAgICBhdHRlbXA6IGdldHRlcnMuYXR0ZW1wLFxuICAgICAgZmV0Y2hpbmdJbnZpdGU6IGdldHRlcnMuZmV0Y2hpbmdJbnZpdGVcbiAgICB9XG4gIH0sXG5cbiAgY29tcG9uZW50RGlkTW91bnQoKXtcbiAgICBhY3Rpb25zLmZldGNoSW52aXRlKHRoaXMucHJvcHMucGFyYW1zLmludml0ZVRva2VuKTtcbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIGxldCB7ZmV0Y2hpbmdJbnZpdGUsIGludml0ZSwgYXR0ZW1wfSA9IHRoaXMuc3RhdGU7XG5cbiAgICBpZihmZXRjaGluZ0ludml0ZS5pc0ZhaWxlZCl7XG4gICAgICByZXR1cm4gPEV4cGlyZWRJbnZpdGUvPlxuICAgIH1cblxuICAgIGlmKCFpbnZpdGUpIHtcbiAgICAgIHJldHVybiBudWxsO1xuICAgIH1cblxuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1pbnZpdGUgdGV4dC1jZW50ZXJcIj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbG9nby10cHJ0XCI+PC9kaXY+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWNvbnRlbnQgZ3J2LWZsZXhcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LWNvbHVtblwiPlxuICAgICAgICAgICAgPEludml0ZUlucHV0Rm9ybSBhdHRlbXA9e2F0dGVtcH0gaW52aXRlPXtpbnZpdGUudG9KUygpfS8+XG4gICAgICAgICAgICA8R29vZ2xlQXV0aEluZm8vPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWZsZXgtY29sdW1uIGdydi1pbnZpdGUtYmFyY29kZVwiPlxuICAgICAgICAgICAgPGg0PlNjYW4gYmFyIGNvZGUgZm9yIGF1dGggdG9rZW4gPGJyLz4gPHNtYWxsPlNjYW4gYmVsb3cgdG8gZ2VuZXJhdGUgeW91ciB0d28gZmFjdG9yIHRva2VuPC9zbWFsbD48L2g0PlxuICAgICAgICAgICAgPGltZyBjbGFzc05hbWU9XCJpbWctdGh1bWJuYWlsXCIgc3JjPXsgYGRhdGE6aW1hZ2UvcG5nO2Jhc2U2NCwke2ludml0ZS5nZXQoJ3FyJyl9YCB9IC8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9kaXY+XG4gICAgKTtcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gSW52aXRlO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvbmV3VXNlci5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHVzZXJHZXR0ZXJzID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvdXNlci9nZXR0ZXJzJyk7XG52YXIgbm9kZUdldHRlcnMgPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9ub2Rlcy9nZXR0ZXJzJyk7XG52YXIgTm9kZUxpc3QgPSByZXF1aXJlKCcuL25vZGVMaXN0LmpzeCcpO1xuXG52YXIgTm9kZXMgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIG5vZGVSZWNvcmRzOiBub2RlR2V0dGVycy5ub2RlTGlzdFZpZXcsXG4gICAgICB1c2VyOiB1c2VyR2V0dGVycy51c2VyXG4gICAgfVxuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgdmFyIG5vZGVSZWNvcmRzID0gdGhpcy5zdGF0ZS5ub2RlUmVjb3JkcztcbiAgICB2YXIgbG9naW5zID0gdGhpcy5zdGF0ZS51c2VyLmxvZ2lucztcbiAgICByZXR1cm4gKCA8Tm9kZUxpc3Qgbm9kZVJlY29yZHM9e25vZGVSZWNvcmRzfSBsb2dpbnM9e2xvZ2luc30vPiApO1xuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBOb2RlcztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL25vZGVzL21haW4uanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7RGF0ZVJhbmdlUGlja2VyLCBDYWxlbmRhck5hdn0gPSByZXF1aXJlKCcuLy4uL2RhdGVQaWNrZXIuanN4Jyk7XG52YXIge1RhYmxlLCBDb2x1bW4sIENlbGwsIFRleHRDZWxsLCBTb3J0SGVhZGVyQ2VsbCwgU29ydFR5cGVzfSA9IHJlcXVpcmUoJ2FwcC9jb21wb25lbnRzL3RhYmxlLmpzeCcpO1xudmFyIHtCdXR0b25DZWxsLCBVc2Vyc0NlbGwsIEVtcHR5TGlzdCwgTm9kZUNlbGwsIER1cmF0aW9uQ2VsbCwgRGF0ZUNyZWF0ZWRDZWxsfSA9IHJlcXVpcmUoJy4vbGlzdEl0ZW1zJyk7XG5cbnZhciBBY3RpdmVTZXNzaW9uTGlzdCA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICBsZXQgZGF0YSA9IHRoaXMucHJvcHMuZGF0YS5maWx0ZXIoaXRlbSA9PiBpdGVtLmFjdGl2ZSk7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXNlc3Npb25zLWFjdGl2ZVwiPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1oZWFkZXJcIj5cbiAgICAgICAgICA8aDE+IEFjdGl2ZSBTZXNzaW9ucyA8L2gxPlxuICAgICAgICA8L2Rpdj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtY29udGVudFwiPlxuICAgICAgICAgIHtkYXRhLmxlbmd0aCA9PT0gMCA/IDxFbXB0eUxpc3QgdGV4dD1cIllvdSBoYXZlIG5vIGFjdGl2ZSBzZXNzaW9ucy5cIi8+IDpcbiAgICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiXCI+XG4gICAgICAgICAgICAgIDxUYWJsZSByb3dDb3VudD17ZGF0YS5sZW5ndGh9IGNsYXNzTmFtZT1cInRhYmxlLXN0cmlwZWRcIj5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJzaWRcIlxuICAgICAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD4gU2Vzc2lvbiBJRCA8L0NlbGw+IH1cbiAgICAgICAgICAgICAgICAgIGNlbGw9ezxUZXh0Q2VsbCBkYXRhPXtkYXRhfS8+IH1cbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+IDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17XG4gICAgICAgICAgICAgICAgICAgIDxCdXR0b25DZWxsIGRhdGE9e2RhdGF9IC8+XG4gICAgICAgICAgICAgICAgICB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPiBOb2RlIDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17PE5vZGVDZWxsIGRhdGE9e2RhdGF9IC8+IH1cbiAgICAgICAgICAgICAgICAvPiAgICAgICAgICAgICAgICBcbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJjcmVhdGVkXCJcbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+IENyZWF0ZWQgPC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXs8RGF0ZUNyZWF0ZWRDZWxsIGRhdGE9e2RhdGF9Lz4gfVxuICAgICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD4gVXNlcnMgPC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXs8VXNlcnNDZWxsIGRhdGE9e2RhdGF9IC8+IH1cbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICA8L1RhYmxlPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgfVxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgIClcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gQWN0aXZlU2Vzc2lvbkxpc3Q7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9zZXNzaW9ucy9hY3RpdmVTZXNzaW9uTGlzdC5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHtnZXR0ZXJzLCBhY3Rpb25zfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3Nlc3Npb25zJyk7XG52YXIgU3RvcmVkU2Vzc2lvbkxpc3QgPSByZXF1aXJlKCcuL3N0b3JlZFNlc3Npb25MaXN0LmpzeCcpO1xudmFyIEFjdGl2ZVNlc3Npb25MaXN0ID0gcmVxdWlyZSgnLi9hY3RpdmVTZXNzaW9uTGlzdC5qc3gnKTtcblxudmFyIFNlc3Npb25zID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICBtaXhpbnM6IFtyZWFjdG9yLlJlYWN0TWl4aW5dLFxuXG4gIGdldERhdGFCaW5kaW5ncygpIHtcbiAgICByZXR1cm4ge2RhdGE6IGdldHRlcnMuc2Vzc2lvbnNWaWV3fVxuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgbGV0IHtkYXRhfSA9IHRoaXMuc3RhdGU7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXNlc3Npb25zIGdydi1wYWdlXCI+XG4gICAgICAgIDxBY3RpdmVTZXNzaW9uTGlzdCBkYXRhPXtkYXRhfS8+XG4gICAgICAgIDxociBjbGFzc05hbWU9XCJncnYtZGl2aWRlclwiLz5cbiAgICAgICAgPFN0b3JlZFNlc3Npb25MaXN0IGRhdGE9e2RhdGF9Lz5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IFNlc3Npb25zO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvc2Vzc2lvbnMvbWFpbi5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHsgYWN0aW9uc30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9zZXNzaW9ucycpO1xudmFyIExpbmtlZFN0YXRlTWl4aW4gPSByZXF1aXJlKCdyZWFjdC1hZGRvbnMtbGlua2VkLXN0YXRlLW1peGluJyk7XG52YXIge1RhYmxlLCBDb2x1bW4sIENlbGwsIFRleHRDZWxsLCBTb3J0SGVhZGVyQ2VsbCwgU29ydFR5cGVzfSA9IHJlcXVpcmUoJ2FwcC9jb21wb25lbnRzL3RhYmxlLmpzeCcpO1xudmFyIHtCdXR0b25DZWxsLCBTaW5nbGVVc2VyQ2VsbCwgVXNlcnNDZWxsLCBFbXB0eUxpc3QsIER1cmF0aW9uQ2VsbCwgRGF0ZUNyZWF0ZWRDZWxsfSA9IHJlcXVpcmUoJy4vbGlzdEl0ZW1zJyk7XG52YXIge0RhdGVSYW5nZVBpY2tlciwgQ2FsZW5kYXJOYXZ9ID0gcmVxdWlyZSgnLi8uLi9kYXRlUGlja2VyLmpzeCcpO1xudmFyIG1vbWVudCA9ICByZXF1aXJlKCdtb21lbnQnKTtcbnZhciB7bW9udGhSYW5nZX0gPSByZXF1aXJlKCdhcHAvY29tbW9uL2RhdGVVdGlscycpO1xudmFyIHtpc01hdGNofSA9IHJlcXVpcmUoJ2FwcC9jb21tb24vb2JqZWN0VXRpbHMnKTtcbnZhciBfID0gcmVxdWlyZSgnXycpO1xuXG52YXIgQXJjaGl2ZWRTZXNzaW9ucyA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBtaXhpbnM6IFtMaW5rZWRTdGF0ZU1peGluXSxcblxuICBnZXRJbml0aWFsU3RhdGUocHJvcHMpe1xuICAgIGxldCBbc3RhcnREYXRlLCBlbmREYXRlXSA9IG1vbnRoUmFuZ2UobmV3IERhdGUoKSk7XG4gICAgdGhpcy5zZWFyY2hhYmxlUHJvcHMgPSBbJ3NlcnZlcklwJywgJ2NyZWF0ZWQnLCAnc2lkJywgJ2xvZ2luJ107XG4gICAgcmV0dXJuIHsgZmlsdGVyOiAnJywgY29sU29ydERpcnM6IHtjcmVhdGVkOiAnQVNDJ30sIHN0YXJ0RGF0ZSwgZW5kRGF0ZSB9O1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxNb3VudCgpe1xuICAgIGFjdGlvbnMuZmV0Y2hTZXNzaW9ucyh0aGlzLnN0YXRlLnN0YXJ0RGF0ZSwgdGhpcy5zdGF0ZS5lbmREYXRlKTtcbiAgfSxcblxuICBzZXREYXRlc0FuZFJlZmV0Y2goc3RhcnREYXRlLCBlbmREYXRlKXtcbiAgICBhY3Rpb25zLmZldGNoU2Vzc2lvbnMoc3RhcnREYXRlLCBlbmREYXRlKTtcbiAgICB0aGlzLnN0YXRlLnN0YXJ0RGF0ZSA9IHN0YXJ0RGF0ZTtcbiAgICB0aGlzLnN0YXRlLmVuZERhdGUgPSBlbmREYXRlO1xuICAgIHRoaXMuc2V0U3RhdGUodGhpcy5zdGF0ZSk7XG4gIH0sXG5cbiAgb25Tb3J0Q2hhbmdlKGNvbHVtbktleSwgc29ydERpcikge1xuICAgIHRoaXMuc2V0U3RhdGUoe1xuICAgICAgLi4udGhpcy5zdGF0ZSxcbiAgICAgIGNvbFNvcnREaXJzOiB7IFtjb2x1bW5LZXldOiBzb3J0RGlyIH1cbiAgICB9KTtcbiAgfSxcblxuICBvblJhbmdlUGlja2VyQ2hhbmdlKHtzdGFydERhdGUsIGVuZERhdGV9KXtcbiAgICB0aGlzLnNldERhdGVzQW5kUmVmZXRjaChzdGFydERhdGUsIGVuZERhdGUpO1xuICB9LFxuXG4gIG9uQ2FsZW5kYXJOYXZDaGFuZ2UobmV3VmFsdWUpe1xuICAgIGxldCBbc3RhcnREYXRlLCBlbmREYXRlXSA9IG1vbnRoUmFuZ2UobmV3VmFsdWUpO1xuICAgIHRoaXMuc2V0RGF0ZXNBbmRSZWZldGNoKHN0YXJ0RGF0ZSwgZW5kRGF0ZSk7XG4gIH0sXG5cbiAgc2VhcmNoQW5kRmlsdGVyQ2IodGFyZ2V0VmFsdWUsIHNlYXJjaFZhbHVlLCBwcm9wTmFtZSl7XG4gICAgaWYocHJvcE5hbWUgPT09ICdjcmVhdGVkJyl7XG4gICAgICB2YXIgZGlzcGxheURhdGUgPSBtb21lbnQodGFyZ2V0VmFsdWUpLmZvcm1hdCgnbCBMVFMnKS50b0xvY2FsZVVwcGVyQ2FzZSgpO1xuICAgICAgcmV0dXJuIGRpc3BsYXlEYXRlLmluZGV4T2Yoc2VhcmNoVmFsdWUpICE9PSAtMTtcbiAgICB9XG4gIH0sXG5cbiAgc29ydEFuZEZpbHRlcihkYXRhKXtcbiAgICB2YXIgZmlsdGVyZWQgPSBkYXRhLmZpbHRlcihvYmo9PlxuICAgICAgaXNNYXRjaChvYmosIHRoaXMuc3RhdGUuZmlsdGVyLCB7XG4gICAgICAgIHNlYXJjaGFibGVQcm9wczogdGhpcy5zZWFyY2hhYmxlUHJvcHMsXG4gICAgICAgIGNiOiB0aGlzLnNlYXJjaEFuZEZpbHRlckNiXG4gICAgICB9KSk7XG5cbiAgICB2YXIgY29sdW1uS2V5ID0gT2JqZWN0LmdldE93blByb3BlcnR5TmFtZXModGhpcy5zdGF0ZS5jb2xTb3J0RGlycylbMF07XG4gICAgdmFyIHNvcnREaXIgPSB0aGlzLnN0YXRlLmNvbFNvcnREaXJzW2NvbHVtbktleV07XG4gICAgdmFyIHNvcnRlZCA9IF8uc29ydEJ5KGZpbHRlcmVkLCBjb2x1bW5LZXkpO1xuICAgIGlmKHNvcnREaXIgPT09IFNvcnRUeXBlcy5BU0Mpe1xuICAgICAgc29ydGVkID0gc29ydGVkLnJldmVyc2UoKTtcbiAgICB9XG5cbiAgICByZXR1cm4gc29ydGVkO1xuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgbGV0IHtzdGFydERhdGUsIGVuZERhdGV9ID0gdGhpcy5zdGF0ZTtcbiAgICBsZXQgZGF0YSA9IHRoaXMucHJvcHMuZGF0YS5maWx0ZXIoaXRlbSA9PiAhaXRlbS5hY3RpdmUgJiYgbW9tZW50KGl0ZW0uY3JlYXRlZCkuaXNCZXR3ZWVuKHN0YXJ0RGF0ZSwgZW5kRGF0ZSkpO1xuICAgIGRhdGEgPSB0aGlzLnNvcnRBbmRGaWx0ZXIoZGF0YSk7XG5cbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtc2Vzc2lvbnMtc3RvcmVkXCI+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWhlYWRlclwiPlxuICAgICAgICAgIDxoMT4gQXJjaGl2ZWQgU2Vzc2lvbnMgPC9oMT5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4XCI+XG4gICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LXJvd1wiPlxuICAgICAgICAgICAgICA8RGF0ZVJhbmdlUGlja2VyIHN0YXJ0RGF0ZT17c3RhcnREYXRlfSBlbmREYXRlPXtlbmREYXRlfSBvbkNoYW5nZT17dGhpcy5vblJhbmdlUGlja2VyQ2hhbmdlfS8+XG4gICAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWZsZXgtcm93XCI+XG4gICAgICAgICAgICAgIDxDYWxlbmRhck5hdiB2YWx1ZT17c3RhcnREYXRlfSBvblZhbHVlQ2hhbmdlPXt0aGlzLm9uQ2FsZW5kYXJOYXZDaGFuZ2V9Lz5cbiAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZmxleC1yb3dcIj5cbiAgICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtc2VhcmNoXCI+XG4gICAgICAgICAgICAgICAgPGlucHV0IHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ2ZpbHRlcicpfSBwbGFjZWhvbGRlcj1cIlNlYXJjaC4uLlwiIGNsYXNzTmFtZT1cImZvcm0tY29udHJvbCBpbnB1dC1zbVwiLz5cbiAgICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgPC9kaXY+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWNvbnRlbnRcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgICAgPFRhYmxlIHJvd0NvdW50PXtkYXRhLmxlbmd0aH0gY2xhc3NOYW1lPVwidGFibGUtc3RyaXBlZFwiPlxuICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgY29sdW1uS2V5PVwic2lkXCJcbiAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPiBTZXNzaW9uIElEIDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgIGNlbGw9ezxUZXh0Q2VsbCBkYXRhPXtkYXRhfS8+IH1cbiAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+IDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgIGNlbGw9e1xuICAgICAgICAgICAgICAgICAgPEJ1dHRvbkNlbGwgZGF0YT17ZGF0YX0gLz5cbiAgICAgICAgICAgICAgICB9XG4gICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJjcmVhdGVkXCJcbiAgICAgICAgICAgICAgICBoZWFkZXI9e1xuICAgICAgICAgICAgICAgICAgPFNvcnRIZWFkZXJDZWxsXG4gICAgICAgICAgICAgICAgICAgIHNvcnREaXI9e3RoaXMuc3RhdGUuY29sU29ydERpcnMuY3JlYXRlZH1cbiAgICAgICAgICAgICAgICAgICAgb25Tb3J0Q2hhbmdlPXt0aGlzLm9uU29ydENoYW5nZX1cbiAgICAgICAgICAgICAgICAgICAgdGl0bGU9XCJDcmVhdGVkXCJcbiAgICAgICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgICAgfVxuICAgICAgICAgICAgICAgIGNlbGw9ezxEYXRlQ3JlYXRlZENlbGwgZGF0YT17ZGF0YX0vPiB9XG4gICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPiBVc2VyIDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgIGNlbGw9ezxTaW5nbGVVc2VyQ2VsbCBkYXRhPXtkYXRhfS8+IH1cbiAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgIDwvVGFibGU+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9kaXY+XG4gICAgKVxuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBBcmNoaXZlZFNlc3Npb25zO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvc2Vzc2lvbnMvc3RvcmVkU2Vzc2lvbkxpc3QuanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciByZW5kZXIgPSByZXF1aXJlKCdyZWFjdC1kb20nKS5yZW5kZXI7XG52YXIgeyBSb3V0ZXIsIFJvdXRlLCBSZWRpcmVjdCwgSW5kZXhSb3V0ZSwgYnJvd3Nlckhpc3RvcnkgfSA9IHJlcXVpcmUoJ3JlYWN0LXJvdXRlcicpO1xudmFyIHsgQXBwLCBMb2dpbiwgTm9kZXMsIFNlc3Npb25zLCBOZXdVc2VyLCBDdXJyZW50U2Vzc2lvbkhvc3QsIE5vdEZvdW5kIH0gPSByZXF1aXJlKCcuL2NvbXBvbmVudHMnKTtcbnZhciB7ZW5zdXJlVXNlcn0gPSByZXF1aXJlKCcuL21vZHVsZXMvdXNlci9hY3Rpb25zJyk7XG52YXIgYXV0aCA9IHJlcXVpcmUoJy4vYXV0aCcpO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCcuL3Nlc3Npb24nKTtcbnZhciBjZmcgPSByZXF1aXJlKCcuL2NvbmZpZycpO1xuXG5yZXF1aXJlKCcuL21vZHVsZXMnKTtcblxuLy8gaW5pdCBzZXNzaW9uXG5zZXNzaW9uLmluaXQoKTtcblxuZnVuY3Rpb24gaGFuZGxlTG9nb3V0KG5leHRTdGF0ZSwgcmVwbGFjZSwgY2Ipe1xuICBhdXRoLmxvZ291dCgpO1xufVxuXG5yZW5kZXIoKFxuICA8Um91dGVyIGhpc3Rvcnk9e3Nlc3Npb24uZ2V0SGlzdG9yeSgpfT5cbiAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5sb2dpbn0gY29tcG9uZW50PXtMb2dpbn0vPlxuICAgIDxSb3V0ZSBwYXRoPXtjZmcucm91dGVzLmxvZ291dH0gb25FbnRlcj17aGFuZGxlTG9nb3V0fS8+XG4gICAgPFJvdXRlIHBhdGg9e2NmZy5yb3V0ZXMubmV3VXNlcn0gY29tcG9uZW50PXtOZXdVc2VyfS8+XG4gICAgPFJlZGlyZWN0IGZyb209e2NmZy5yb3V0ZXMuYXBwfSB0bz17Y2ZnLnJvdXRlcy5ub2Rlc30vPlxuICAgIDxSb3V0ZSBwYXRoPXtjZmcucm91dGVzLmFwcH0gY29tcG9uZW50PXtBcHB9IG9uRW50ZXI9e2Vuc3VyZVVzZXJ9ID5cbiAgICAgIDxSb3V0ZSBwYXRoPXtjZmcucm91dGVzLm5vZGVzfSBjb21wb25lbnQ9e05vZGVzfS8+XG4gICAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5hY3RpdmVTZXNzaW9ufSBjb21wb25lbnRzPXt7Q3VycmVudFNlc3Npb25Ib3N0OiBDdXJyZW50U2Vzc2lvbkhvc3R9fS8+XG4gICAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5zZXNzaW9uc30gY29tcG9uZW50PXtTZXNzaW9uc30vPlxuICAgIDwvUm91dGU+XG4gICAgPFJvdXRlIHBhdGg9XCIqXCIgY29tcG9uZW50PXtOb3RGb3VuZH0gLz5cbiAgPC9Sb3V0ZXI+XG4pLCBkb2N1bWVudC5nZXRFbGVtZW50QnlJZChcImFwcFwiKSk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvaW5kZXguanN4XG4gKiovIiwibW9kdWxlLmV4cG9ydHMgPSBUZXJtaW5hbDtcblxuXG4vKioqKioqKioqKioqKioqKipcbiAqKiBXRUJQQUNLIEZPT1RFUlxuICoqIGV4dGVybmFsIFwiVGVybWluYWxcIlxuICoqIG1vZHVsZSBpZCA9IDQxM1xuICoqIG1vZHVsZSBjaHVua3MgPSAxXG4gKiovIl0sInNvdXJjZVJvb3QiOiIifQ==