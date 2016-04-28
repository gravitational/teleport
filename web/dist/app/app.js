webpackJsonp([1],[
/* 0 */
/***/ function(module, exports, __webpack_require__) {

	module.exports = __webpack_require__(331);


/***/ },
/* 1 */,
/* 2 */,
/* 3 */,
/* 4 */,
/* 5 */,
/* 6 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	
	var _nuclearJs = __webpack_require__(12);
	
	var enabled = true;
	
	// temporary workaround to disable debug info during unit-tests
	var karma = window.__karma__;
	if (karma && karma.config.args.length === 1) {
	  enabled = false;
	}
	
	var reactor = new _nuclearJs.Reactor({
	  debug: enabled
	});
	
	window.reactor = reactor;
	
	exports['default'] = reactor;
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "reactor.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 7 */,
/* 8 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(312);
	
	var formatPattern = _require.formatPattern;
	
	var $ = __webpack_require__(22);
	
	var cfg = {
	
	  baseUrl: window.location.origin,
	
	  helpUrl: 'http://gravitational.com/teleport/docs/quickstart/',
	
	  maxSessionLoadSize: 50,
	
	  displayDateFormat: 'l LTS Z',
	
	  auth: {
	    oidc_connectors: []
	  },
	
	  routes: {
	    app: '/web',
	    logout: '/web/logout',
	    login: '/web/login',
	    nodes: '/web/nodes',
	    activeSession: '/web/sessions/:sid',
	    newUser: '/web/newuser/:inviteToken',
	    sessions: '/web/sessions',
	    msgs: '/web/msg/:type(/:subType)',
	    pageNotFound: '/web/notfound'
	  },
	
	  api: {
	    sso: '/v1/webapi/oidc/login/web?redirect_url=:redirect&connector_id=:provider',
	    renewTokenPath: '/v1/webapi/sessions/renew',
	    nodesPath: '/v1/webapi/sites/-current-/nodes',
	    sessionPath: '/v1/webapi/sessions',
	    siteSessionPath: '/v1/webapi/sites/-current-/sessions',
	    invitePath: '/v1/webapi/users/invites/:inviteToken',
	    createUserPath: '/v1/webapi/users',
	    sessionChunk: '/v1/webapi/sites/-current-/sessions/:sid/chunks?start=:start&end=:end',
	    sessionChunkCountPath: '/v1/webapi/sites/-current-/sessions/:sid/chunkscount',
	    siteEventSessionFilterPath: '/v1/webapi/sites/-current-/sessions?filter=:filter',
	
	    getSsoUrl: function getSsoUrl(redirect, provider) {
	      return cfg.baseUrl + formatPattern(cfg.api.sso, { redirect: redirect, provider: provider });
	    },
	
	    getFetchSessionChunkUrl: function getFetchSessionChunkUrl(_ref) {
	      var sid = _ref.sid;
	      var start = _ref.start;
	      var end = _ref.end;
	
	      return formatPattern(cfg.api.sessionChunk, { sid: sid, start: start, end: end });
	    },
	
	    getFetchSessionLengthUrl: function getFetchSessionLengthUrl(sid) {
	      return formatPattern(cfg.api.sessionChunkCountPath, { sid: sid });
	    },
	
	    getFetchSessionsUrl: function getFetchSessionsUrl(args) {
	      var filter = JSON.stringify(args);
	      return formatPattern(cfg.api.siteEventSessionFilterPath, { filter: filter });
	    },
	
	    getFetchSessionUrl: function getFetchSessionUrl(sid) {
	      return formatPattern(cfg.api.siteSessionPath + '/:sid', { sid: sid });
	    },
	
	    getTerminalSessionUrl: function getTerminalSessionUrl(sid) {
	      return formatPattern(cfg.api.siteSessionPath + '/:sid', { sid: sid });
	    },
	
	    getInviteUrl: function getInviteUrl(inviteToken) {
	      return formatPattern(cfg.api.invitePath, { inviteToken: inviteToken });
	    },
	
	    getEventStreamConnStr: function getEventStreamConnStr() {
	      var hostname = getWsHostName();
	      return hostname + '/v1/webapi/sites/-current-';
	    },
	
	    getTtyUrl: function getTtyUrl() {
	      var hostname = getWsHostName();
	      return hostname + '/v1/webapi/sites/-current-';
	    }
	
	  },
	
	  getFullUrl: function getFullUrl(url) {
	    return cfg.baseUrl + url;
	  },
	
	  getActiveSessionRouteUrl: function getActiveSessionRouteUrl(sid) {
	    return formatPattern(cfg.routes.activeSession, { sid: sid });
	  },
	
	  getAuthProviders: function getAuthProviders() {
	    return cfg.auth.oidc_connectors;
	  },
	
	  init: function init() {
	    var config = arguments.length <= 0 || arguments[0] === undefined ? {} : arguments[0];
	
	    $.extend(true, this, config);
	  }
	};
	
	exports['default'] = cfg;
	
	function getWsHostName() {
	  var prefix = location.protocol == "https:" ? "wss://" : "ws://";
	  var hostport = location.hostname + (location.port ? ':' + location.port : '');
	  return '' + prefix + hostport;
	}
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "config.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 9 */,
/* 10 */,
/* 11 */,
/* 12 */,
/* 13 */,
/* 14 */,
/* 15 */,
/* 16 */,
/* 17 */,
/* 18 */,
/* 19 */,
/* 20 */,
/* 21 */,
/* 22 */
/***/ function(module, exports) {

	module.exports = jQuery;

/***/ },
/* 23 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	var _bind = Function.prototype.bind;
	
	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError('Cannot call a class as a function'); } }
	
	var Logger = (function () {
	  function Logger() {
	    var name = arguments.length <= 0 || arguments[0] === undefined ? 'default' : arguments[0];
	
	    _classCallCheck(this, Logger);
	
	    this.name = name;
	  }
	
	  Logger.prototype.log = function log() {
	    var level = arguments.length <= 0 || arguments[0] === undefined ? 'log' : arguments[0];
	
	    for (var _len = arguments.length, args = Array(_len > 1 ? _len - 1 : 0), _key = 1; _key < _len; _key++) {
	      args[_key - 1] = arguments[_key];
	    }
	
	    console[level].apply(console, ['%c[' + this.name + ']', 'color: blue;'].concat(args));
	  };
	
	  Logger.prototype.trace = function trace() {
	    for (var _len2 = arguments.length, args = Array(_len2), _key2 = 0; _key2 < _len2; _key2++) {
	      args[_key2] = arguments[_key2];
	    }
	
	    this.log.apply(this, ['trace'].concat(args));
	  };
	
	  Logger.prototype.warn = function warn() {
	    for (var _len3 = arguments.length, args = Array(_len3), _key3 = 0; _key3 < _len3; _key3++) {
	      args[_key3] = arguments[_key3];
	    }
	
	    this.log.apply(this, ['warn'].concat(args));
	  };
	
	  Logger.prototype.info = function info() {
	    for (var _len4 = arguments.length, args = Array(_len4), _key4 = 0; _key4 < _len4; _key4++) {
	      args[_key4] = arguments[_key4];
	    }
	
	    this.log.apply(this, ['info'].concat(args));
	  };
	
	  Logger.prototype.error = function error() {
	    for (var _len5 = arguments.length, args = Array(_len5), _key5 = 0; _key5 < _len5; _key5++) {
	      args[_key5] = arguments[_key5];
	    }
	
	    this.log.apply(this, ['error'].concat(args));
	  };
	
	  return Logger;
	})();
	
	exports['default'] = {
	  create: function create() {
	    for (var _len6 = arguments.length, args = Array(_len6), _key6 = 0; _key6 < _len6; _key6++) {
	      args[_key6] = arguments[_key6];
	    }
	
	    return new (_bind.apply(Logger, [null].concat(args)))();
	  }
	};
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "logger.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 24 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	var $ = __webpack_require__(22);
	var session = __webpack_require__(32);
	
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
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "api.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 25 */,
/* 26 */,
/* 27 */,
/* 28 */,
/* 29 */,
/* 30 */,
/* 31 */,
/* 32 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	var _require = __webpack_require__(37);
	
	var browserHistory = _require.browserHistory;
	var createMemoryHistory = _require.createMemoryHistory;
	
	var logger = __webpack_require__(23).create('services/sessions');
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
	
	    // for sso use-cases, try to grab the token from HTML
	    var hiddenDiv = document.getElementById("bearer_token");
	    if (hiddenDiv !== null) {
	      try {
	        var json = window.atob(hiddenDiv.textContent);
	        var userData = JSON.parse(json);
	        if (userData.token) {
	          // put it into the session
	          this.setUserData(userData);
	          // remove the element
	          hiddenDiv.remove();
	          return userData;
	        }
	      } catch (err) {
	        logger.error('error parsing SSO token:', err);
	      }
	    }
	
	    return {};
	  },
	
	  clear: function clear() {
	    localStorage.clear();
	  }
	
	};
	
	module.exports = session;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "session.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 33 */,
/* 34 */,
/* 35 */,
/* 36 */,
/* 37 */,
/* 38 */,
/* 39 */,
/* 40 */,
/* 41 */,
/* 42 */,
/* 43 */
/***/ function(module, exports) {

	module.exports = _;

/***/ },
/* 44 */,
/* 45 */,
/* 46 */,
/* 47 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	var React = __webpack_require__(2);
	var logoSvg = __webpack_require__(439);
	var classnames = __webpack_require__(62);
	
	var TeleportLogo = function TeleportLogo() {
	  return React.createElement(
	    'svg',
	    { className: 'grv-icon-logo-tlpt' },
	    React.createElement('use', { xlinkHref: logoSvg })
	  );
	};
	
	var UserIcon = function UserIcon(_ref) {
	  var _ref$name = _ref.name;
	  var name = _ref$name === undefined ? '' : _ref$name;
	  var isDark = _ref.isDark;
	
	  var iconClass = classnames('grv-icon-user', {
	    '--dark': isDark
	  });
	
	  return React.createElement(
	    'div',
	    { title: name, className: iconClass },
	    React.createElement(
	      'span',
	      null,
	      React.createElement(
	        'strong',
	        null,
	        name[0]
	      )
	    )
	  );
	};
	
	exports.TeleportLogo = TeleportLogo;
	exports.UserIcon = UserIcon;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "icons.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 48 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	var React = __webpack_require__(2);
	
	var MSG_INFO_LOGIN_SUCCESS = 'Login was successful, you can close this window and continue using tsh.';
	var MSG_ERROR_LOGIN_FAILED = 'Login unsuccessful. Please try again, if the problem persists, contact your system administator.';
	var MSG_ERROR_DEFAULT = 'Whoops, something went wrong.';
	
	var MSG_ERROR_NOT_FOUND = 'Whoops, we cannot find that.';
	var MSG_ERROR_NOT_FOUND_DETAILS = 'Looks like the page you are looking for isn\'t here any longer.';
	
	var MSG_ERROR_EXPIRED_INVITE = 'Invite code has expired.';
	var MSG_ERROR_EXPIRED_INVITE_DETAILS = 'Looks like your invite code isn\'t valid anymore.';
	
	var MsgType = {
	  INFO: 'info',
	  ERROR: 'error'
	};
	
	var ErrorTypes = {
	  FAILED_TO_LOGIN: 'login_failed',
	  EXPIRED_INVITE: 'expired_invite',
	  NOT_FOUND: 'not_found'
	};
	
	var InfoTypes = {
	  LOGIN_SUCCESS: 'login_success'
	};
	
	var MessagePage = React.createClass({
	  displayName: 'MessagePage',
	
	  render: function render() {
	    var _props$params = this.props.params;
	    var type = _props$params.type;
	    var subType = _props$params.subType;
	
	    if (type === MsgType.ERROR) {
	      return React.createElement(ErrorPage, { type: subType });
	    }
	
	    if (type === MsgType.INFO) {
	      return React.createElement(InfoPage, { type: subType });
	    }
	
	    return null;
	  }
	});
	
	var ErrorPage = React.createClass({
	  displayName: 'ErrorPage',
	
	  render: function render() {
	    var type = this.props.type;
	
	    var msgBody = React.createElement(
	      'div',
	      null,
	      React.createElement(
	        'h1',
	        null,
	        MSG_ERROR_DEFAULT
	      )
	    );
	
	    if (type === ErrorTypes.FAILED_TO_LOGIN) {
	      msgBody = React.createElement(
	        'div',
	        null,
	        React.createElement(
	          'h1',
	          null,
	          MSG_ERROR_LOGIN_FAILED
	        )
	      );
	    }
	
	    if (type === ErrorTypes.EXPIRED_INVITE) {
	      msgBody = React.createElement(
	        'div',
	        null,
	        React.createElement(
	          'h1',
	          null,
	          MSG_ERROR_EXPIRED_INVITE
	        ),
	        React.createElement(
	          'div',
	          null,
	          MSG_ERROR_EXPIRED_INVITE_DETAILS
	        )
	      );
	    }
	
	    if (type === ErrorTypes.NOT_FOUND) {
	      msgBody = React.createElement(
	        'div',
	        null,
	        React.createElement(
	          'h1',
	          null,
	          MSG_ERROR_NOT_FOUND
	        ),
	        React.createElement(
	          'div',
	          null,
	          MSG_ERROR_NOT_FOUND_DETAILS
	        )
	      );
	    }
	
	    return React.createElement(
	      'div',
	      { className: 'grv-msg-page' },
	      React.createElement(
	        'div',
	        { className: 'grv-header' },
	        React.createElement('i', { className: 'fa fa-frown-o' }),
	        ' '
	      ),
	      msgBody,
	      React.createElement(
	        'div',
	        { className: 'contact-section' },
	        'If you believe this is an issue with Teleport, please ',
	        React.createElement(
	          'a',
	          { href: 'https://github.com/gravitational/teleport/issues/new' },
	          'create a GitHub issue.'
	        )
	      )
	    );
	  }
	});
	
	var InfoPage = React.createClass({
	  displayName: 'InfoPage',
	
	  render: function render() {
	    var type = this.props.type;
	
	    var msgBody = null;
	
	    if (type === InfoTypes.LOGIN_SUCCESS) {
	      msgBody = React.createElement(
	        'div',
	        null,
	        React.createElement(
	          'h1',
	          null,
	          MSG_INFO_LOGIN_SUCCESS
	        )
	      );
	    }
	
	    return React.createElement(
	      'div',
	      { className: 'grv-msg-page' },
	      React.createElement(
	        'div',
	        { className: 'grv-header' },
	        React.createElement('i', { className: 'fa fa-smile-o' }),
	        ' '
	      ),
	      msgBody
	    );
	  }
	});
	
	var NotFound = function NotFound() {
	  return React.createElement(ErrorPage, { type: ErrorTypes.NOT_FOUND });
	};
	
	exports.ErrorPage = ErrorPage;
	exports.InfoPage = InfoPage;
	exports.NotFound = NotFound;
	exports.ErrorTypes = ErrorTypes;
	exports.MessagePage = MessagePage;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "msgPage.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 49 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	
	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };
	
	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }
	
	var React = __webpack_require__(2);
	
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
	    var _props = this.props;
	    var sortDir = _props.sortDir;
	    var title = _props.title;
	
	    var props = _objectWithoutProperties(_props, ['sortDir', 'title']);
	
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
	    } else if (typeof cell === 'function') {
	      content = cell(cellProps);
	    }
	
	    return content;
	  },
	
	  render: function render() {
	    var children = [];
	    React.Children.forEach(this.props.children, function (child) {
	      if (child == null) {
	        return;
	      }
	
	      if (child.type.displayName !== 'GrvTableColumn') {
	        throw 'Should be GrvTableColumn';
	      }
	
	      children.push(child);
	    });
	
	    var tableClass = 'table grv-table ' + this.props.className;
	
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
	
	var EmptyIndicator = function EmptyIndicator(_ref3) {
	  var text = _ref3.text;
	  return React.createElement(
	    'div',
	    { className: 'grv-table-indicator-empty text-center text-muted' },
	    React.createElement(
	      'span',
	      null,
	      text
	    )
	  );
	};
	
	exports['default'] = GrvTable;
	exports.Column = GrvTableColumn;
	exports.Table = GrvTable;
	exports.Cell = GrvTableCell;
	exports.TextCell = GrvTableTextCell;
	exports.SortHeaderCell = SortHeaderCell;
	exports.SortIndicator = SortIndicator;
	exports.SortTypes = SortTypes;
	exports.EmptyIndicator = EmptyIndicator;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "table.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 50 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
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

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "getters.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 51 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(6);
	
	var _require = __webpack_require__(230);
	
	var TLPT_NOTIFICATIONS_ADD = _require.TLPT_NOTIFICATIONS_ADD;
	exports['default'] = {
	
	  showError: function showError(text) {
	    var title = arguments.length <= 1 || arguments[1] === undefined ? 'ERROR' : arguments[1];
	
	    dispatch({ isError: true, text: text, title: title });
	  },
	
	  showSuccess: function showSuccess(text) {
	    var title = arguments.length <= 1 || arguments[1] === undefined ? 'SUCCESS' : arguments[1];
	
	    dispatch({ isSuccess: true, text: text, title: title });
	  },
	
	  showInfo: function showInfo(text) {
	    var title = arguments.length <= 1 || arguments[1] === undefined ? 'INFO' : arguments[1];
	
	    dispatch({ isInfo: true, text: text, title: title });
	  },
	
	  showWarning: function showWarning(text) {
	    var title = arguments.length <= 1 || arguments[1] === undefined ? 'WARNING' : arguments[1];
	
	    dispatch({ isWarning: true, text: text, title: title });
	  }
	
	};
	
	function dispatch(msg) {
	  reactor.dispatch(TLPT_NOTIFICATIONS_ADD, msg);
	}
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "actions.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 52 */,
/* 53 */,
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
/* 68 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(71);
	
	var createView = _require.createView;
	
	var currentSession = [['tlpt_current_session'], ['tlpt_sessions'], function (current, sessions) {
	  if (!current) {
	    return null;
	  }
	
	  /*
	  * active session needs to have its own view as an actual session might not
	  * exist at this point. For example, upon creating a new session we need to know
	  * login and serverId. It will be simplified once server API gets extended.
	  */
	  var curSessionView = {
	    isNewSession: current.get('isNewSession'),
	    notFound: current.get('notFound'),
	    addr: current.get('addr'),
	    serverId: current.get('serverId'),
	    serverIp: undefined,
	    login: current.get('login'),
	    sid: current.get('sid'),
	    cols: undefined,
	    rows: undefined
	  };
	
	  /*
	  * in case if session already exists, get its view data (for example, when joining an existing session)
	  */
	  if (sessions.has(curSessionView.sid)) {
	    var existing = createView(sessions.get(curSessionView.sid));
	
	    curSessionView.parties = existing.parties;
	    curSessionView.serverIp = existing.serverIp;
	    curSessionView.serverId = existing.serverId;
	    curSessionView.active = existing.active;
	    curSessionView.cols = existing.cols;
	    curSessionView.rows = existing.rows;
	  }
	
	  return curSessionView;
	}];
	
	exports['default'] = {
	  currentSession: currentSession
	};
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "getters.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 69 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(15);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_SESSINS_RECEIVE: null,
	  TLPT_SESSINS_UPDATE: null,
	  TLPT_SESSINS_REMOVE_STORED: null
	});
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "actionTypes.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 70 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(6);
	var api = __webpack_require__(24);
	var apiUtils = __webpack_require__(237);
	var cfg = __webpack_require__(8);
	
	var _require = __webpack_require__(51);
	
	var showError = _require.showError;
	
	var logger = __webpack_require__(23).create('Modules/Sessions');
	
	var _require2 = __webpack_require__(69);
	
	var TLPT_SESSINS_RECEIVE = _require2.TLPT_SESSINS_RECEIVE;
	var TLPT_SESSINS_UPDATE = _require2.TLPT_SESSINS_UPDATE;
	
	var actions = {
	
	  fetchSession: function fetchSession(sid) {
	    return api.get(cfg.api.getFetchSessionUrl(sid)).then(function (json) {
	      if (json && json.session) {
	        reactor.dispatch(TLPT_SESSINS_UPDATE, json.session);
	      }
	    });
	  },
	
	  fetchSessions: function fetchSessions() {
	    var _ref = arguments.length <= 0 || arguments[0] === undefined ? {} : arguments[0];
	
	    var end = _ref.end;
	    var sid = _ref.sid;
	    var _ref$limit = _ref.limit;
	    var limit = _ref$limit === undefined ? cfg.maxSessionLoadSize : _ref$limit;
	
	    var start = end || new Date();
	    var params = {
	      order: -1,
	      limit: limit,
	      start: start,
	      sid: sid
	    };
	
	    return apiUtils.filterSessions(params).done(function (json) {
	      reactor.dispatch(TLPT_SESSINS_RECEIVE, json.sessions);
	    }).fail(function (err) {
	      showError('Unable to retrieve list of sessions');
	      logger.error('fetchSessions', err);
	    });
	  },
	
	  updateSession: function updateSession(json) {
	    reactor.dispatch(TLPT_SESSINS_UPDATE, json);
	  }
	};
	
	exports['default'] = actions;
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "actions.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 71 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(12);
	
	var toImmutable = _require.toImmutable;
	
	var reactor = __webpack_require__(6);
	var cfg = __webpack_require__(8);
	
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
	  }).last();
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
	    created: session.get('created'),
	    lastActive: session.get('last_active'),
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
	  createView: createView,
	  count: [['tlpt_sessions'], function (sessions) {
	    return sessions.size;
	  }]
	};
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "getters.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 72 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	var filter = [['tlpt_stored_sessions_filter'], function (filter) {
	  return filter.toJS();
	}];
	
	exports['default'] = {
	  filter: filter
	};
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "getters.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 73 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(15);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_RECEIVE_USER: null,
	  TLPT_RECEIVE_USER_INVITE: null
	});
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "actionTypes.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 74 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(232);
	
	var TRYING_TO_LOGIN = _require.TRYING_TO_LOGIN;
	var TRYING_TO_SIGN_UP = _require.TRYING_TO_SIGN_UP;
	var FETCHING_INVITE = _require.FETCHING_INVITE;
	
	var _require2 = __webpack_require__(343);
	
	var requestStatus = _require2.requestStatus;
	
	var invite = [['tlpt_user_invite'], function (invite) {
	  return invite;
	}];
	
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
	  invite: invite,
	  loginAttemp: requestStatus(TRYING_TO_LOGIN),
	  attemp: requestStatus(TRYING_TO_SIGN_UP),
	  fetchingInvite: requestStatus(FETCHING_INVITE)
	};
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "getters.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 75 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	var api = __webpack_require__(24);
	var session = __webpack_require__(32);
	var cfg = __webpack_require__(8);
	var $ = __webpack_require__(22);
	
	var PROVIDER_GOOGLE = 'google';
	
	var refreshRate = 60000 * 5; // 5 min
	
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
	    session.clear();
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
	    auth._redirect();
	  },
	
	  _redirect: function _redirect() {
	    window.location = cfg.routes.login;
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
	module.exports.PROVIDER_GOOGLE = PROVIDER_GOOGLE;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "auth.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 76 */,
/* 77 */,
/* 78 */,
/* 79 */,
/* 80 */,
/* 81 */,
/* 82 */,
/* 83 */,
/* 84 */,
/* 85 */,
/* 86 */,
/* 87 */,
/* 88 */,
/* 89 */,
/* 90 */,
/* 91 */,
/* 92 */,
/* 93 */,
/* 94 */,
/* 95 */,
/* 96 */,
/* 97 */,
/* 98 */,
/* 99 */
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
/* 100 */,
/* 101 */,
/* 102 */,
/* 103 */,
/* 104 */,
/* 105 */,
/* 106 */,
/* 107 */,
/* 108 */,
/* 109 */,
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
/* 148 */,
/* 149 */,
/* 150 */,
/* 151 */,
/* 152 */,
/* 153 */,
/* 154 */,
/* 155 */,
/* 156 */,
/* 157 */,
/* 158 */,
/* 159 */,
/* 160 */,
/* 161 */,
/* 162 */,
/* 163 */,
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
/* 188 */,
/* 189 */,
/* 190 */,
/* 191 */,
/* 192 */,
/* 193 */,
/* 194 */,
/* 195 */,
/* 196 */,
/* 197 */,
/* 198 */,
/* 199 */,
/* 200 */,
/* 201 */,
/* 202 */,
/* 203 */,
/* 204 */,
/* 205 */,
/* 206 */,
/* 207 */,
/* 208 */,
/* 209 */,
/* 210 */,
/* 211 */,
/* 212 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
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
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "objectUtils.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 213 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError('Cannot call a class as a function'); } }
	
	var Term = __webpack_require__(443);
	var Tty = __webpack_require__(214);
	var TtyEvents = __webpack_require__(313);
	
	var _require = __webpack_require__(43);
	
	var debounce = _require.debounce;
	var isNumber = _require.isNumber;
	
	var cfg = __webpack_require__(8);
	var api = __webpack_require__(24);
	var logger = __webpack_require__(23).create('terminal');
	var $ = __webpack_require__(22);
	
	Term.colors[256] = '#252323';
	
	var DISCONNECT_TXT = '\x1b[31mdisconnected\x1b[m\r\n';
	var CONNECTED_TXT = 'Connected!\r\n';
	var GRV_CLASS = 'grv-terminal';
	
	var TtyTerminal = (function () {
	  function TtyTerminal(options) {
	    _classCallCheck(this, TtyTerminal);
	
	    var tty = options.tty;
	    var cols = options.cols;
	    var rows = options.rows;
	    var _options$scrollBack = options.scrollBack;
	    var scrollBack = _options$scrollBack === undefined ? 1000 : _options$scrollBack;
	
	    this.ttyParams = tty;
	    this.tty = new Tty();
	    this.ttyEvents = new TtyEvents();
	
	    this.scrollBack = scrollBack;
	    this.rows = rows;
	    this.cols = cols;
	    this.term = null;
	    this._el = options.el;
	
	    this.debouncedResize = debounce(this._requestResize.bind(this), 200);
	  }
	
	  TtyTerminal.prototype.open = function open() {
	    var _this = this;
	
	    $(this._el).addClass(GRV_CLASS);
	
	    this.term = new Term({
	      cols: 15,
	      rows: 5,
	      scrollback: this.scrollback,
	      useStyle: true,
	      screenKeys: true,
	      cursorBlink: true
	    });
	
	    this.term.open(this._el);
	
	    this.resize(this.cols, this.rows);
	
	    // term events
	    this.term.on('data', function (data) {
	      return _this.tty.send(data);
	    });
	
	    // tty
	    this.tty.on('resize', function (_ref) {
	      var h = _ref.h;
	      var w = _ref.w;
	      return _this.resize(w, h);
	    });
	    this.tty.on('reset', function () {
	      return _this.term.reset();
	    });
	    this.tty.on('open', function () {
	      return _this.term.write(CONNECTED_TXT);
	    });
	    this.tty.on('close', function () {
	      return _this.term.write(DISCONNECT_TXT);
	    });
	    this.tty.on('data', function (data) {
	      try {
	        _this.term.write(data);
	      } catch (err) {
	        console.error(err);
	      }
	    });
	
	    // ttyEvents
	    this.ttyEvents.on('data', this._handleTtyEventsData.bind(this));
	    this.connect();
	    window.addEventListener('resize', this.debouncedResize);
	  };
	
	  TtyTerminal.prototype.connect = function connect() {
	    this.tty.connect(this._getTtyConnStr());
	    this.ttyEvents.connect(this._getTtyEventsConnStr());
	  };
	
	  TtyTerminal.prototype.destroy = function destroy() {
	    if (this.tty !== null) {
	      this.tty.disconnect();
	    }
	
	    if (this.ttyEvents !== null) {
	      this.ttyEvents.disconnect();
	      this.ttyEvents.removeAllListeners();
	    }
	
	    if (this.term !== null) {
	      this.term.destroy();
	      this.term.removeAllListeners();
	    }
	
	    $(this._el).empty().removeClass(GRV_CLASS);
	
	    window.removeEventListener('resize', this.debouncedResize);
	  };
	
	  TtyTerminal.prototype.resize = function resize(cols, rows) {
	    // if not defined, use the size of the container
	    if (!isNumber(cols) || !isNumber(rows)) {
	      var dim = this._getDimensions();
	      cols = dim.cols;
	      rows = dim.rows;
	    }
	
	    this.cols = cols;
	    this.rows = rows;
	    this.term.resize(this.cols, this.rows);
	  };
	
	  TtyTerminal.prototype._requestResize = function _requestResize() {
	    var _getDimensions2 = this._getDimensions();
	
	    var cols = _getDimensions2.cols;
	    var rows = _getDimensions2.rows;
	
	    var w = cols;
	    var h = rows;
	
	    // some min values
	    w = w < 5 ? 5 : w;
	    h = h < 5 ? 5 : h;
	
	    var sid = this.ttyParams.sid;
	
	    var reqData = { terminal_params: { w: w, h: h } };
	
	    logger.info('resize', 'w:' + w + ' and h:' + h);
	    api.put(cfg.api.getTerminalSessionUrl(sid), reqData).done(function () {
	      return logger.info('resized');
	    }).fail(function (err) {
	      return logger.error('failed to resize', err);
	    });
	  };
	
	  TtyTerminal.prototype._handleTtyEventsData = function _handleTtyEventsData(data) {
	    if (data && data.terminal_params) {
	      var _data$terminal_params = data.terminal_params;
	      var w = _data$terminal_params.w;
	      var h = _data$terminal_params.h;
	
	      if (h !== this.rows || w !== this.cols) {
	        this.resize(w, h);
	      }
	    }
	  };
	
	  TtyTerminal.prototype._getDimensions = function _getDimensions() {
	    var $container = $(this._el);
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
	  };
	
	  TtyTerminal.prototype._getTtyEventsConnStr = function _getTtyEventsConnStr() {
	    var _ttyParams = this.ttyParams;
	    var sid = _ttyParams.sid;
	    var url = _ttyParams.url;
	    var token = _ttyParams.token;
	
	    return url + '/sessions/' + sid + '/events/stream?access_token=' + token;
	  };
	
	  TtyTerminal.prototype._getTtyConnStr = function _getTtyConnStr() {
	    var _ttyParams2 = this.ttyParams;
	    var serverId = _ttyParams2.serverId;
	    var login = _ttyParams2.login;
	    var sid = _ttyParams2.sid;
	    var url = _ttyParams2.url;
	    var token = _ttyParams2.token;
	
	    var params = {
	      server_id: serverId,
	      login: login,
	      sid: sid,
	      term: {
	        h: this.rows,
	        w: this.cols
	      }
	    };
	
	    var json = JSON.stringify(params);
	    var jsonEncoded = window.encodeURI(json);
	
	    return url + '/connect?access_token=' + token + '&params=' + jsonEncoded;
	  };
	
	  return TtyTerminal;
	})();
	
	module.exports = TtyTerminal;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "terminal.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 214 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError('Cannot call a class as a function'); } }
	
	function _inherits(subClass, superClass) { if (typeof superClass !== 'function' && superClass !== null) { throw new TypeError('Super expression must either be null or a function, not ' + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }
	
	var EventEmitter = __webpack_require__(99).EventEmitter;
	var Buffer = __webpack_require__(61).Buffer;
	
	var Tty = (function (_EventEmitter) {
	  _inherits(Tty, _EventEmitter);
	
	  function Tty() {
	    _classCallCheck(this, Tty);
	
	    _EventEmitter.call(this);
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
	
	  Tty.prototype.connect = function connect(connStr) {
	    var _this = this;
	
	    this.socket = new WebSocket(connStr, 'proto');
	
	    this.socket.onopen = function () {
	      _this.emit('open');
	    };
	
	    this.socket.onmessage = function (e) {
	      var data = new Buffer(e.data, 'base64').toString('utf8');
	      _this.emit('data', data);
	    };
	
	    this.socket.onclose = function () {
	      _this.emit('close');
	    };
	  };
	
	  Tty.prototype.send = function send(data) {
	    this.socket.send(data);
	  };
	
	  return Tty;
	})(EventEmitter);
	
	module.exports = Tty;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "tty.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 215 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	var React = __webpack_require__(2);
	
	var _require = __webpack_require__(225);
	
	var actions = _require.actions;
	
	var _require2 = __webpack_require__(47);
	
	var UserIcon = _require2.UserIcon;
	
	var ReactCSSTransitionGroup = __webpack_require__(310);
	
	var SessionLeftPanel = function SessionLeftPanel(_ref) {
	  var parties = _ref.parties;
	
	  parties = parties || [];
	  var userIcons = parties.map(function (item, index) {
	    return React.createElement(
	      'li',
	      { key: index, className: 'animated' },
	      React.createElement(UserIcon, { colorIndex: index, isDark: true, name: item.user })
	    );
	  });
	
	  return React.createElement(
	    'div',
	    { className: 'grv-terminal-participans' },
	    React.createElement(
	      'ul',
	      { className: 'nav' },
	      React.createElement(
	        'li',
	        { title: 'Close' },
	        React.createElement(
	          'button',
	          { onClick: actions.close, className: 'btn btn-danger btn-circle', type: 'button' },
	          React.createElement('i', { className: 'fa fa-times' })
	        )
	      )
	    ),
	    userIcons.length > 0 ? React.createElement('hr', { className: 'grv-divider' }) : null,
	    React.createElement(
	      ReactCSSTransitionGroup,
	      { className: 'nav', component: 'ul',
	        transitionEnterTimeout: 500,
	        transitionLeaveTimeout: 500,
	        transitionName: {
	          enter: "fadeIn",
	          leave: "fadeOut"
	        } },
	      userIcons
	    )
	  );
	};
	
	module.exports = SessionLeftPanel;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "sessionLeftPanel.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 216 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	"use strict";
	
	var React = __webpack_require__(2);
	
	var GoogleAuthInfo = React.createClass({
	  displayName: "GoogleAuthInfo",
	
	  render: function render() {
	    return React.createElement(
	      "div",
	      { className: "grv-google-auth" },
	      React.createElement("div", { className: "grv-icon-google-auth" }),
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
	        " on your phone to access your two factor token"
	      )
	    );
	  }
	});
	
	module.exports = GoogleAuthInfo;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "googleAuthLogo.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 217 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	var React = __webpack_require__(2);
	
	var _require = __webpack_require__(43);
	
	var debounce = _require.debounce;
	
	var InputSearch = React.createClass({
	  displayName: 'InputSearch',
	
	  getInitialState: function getInitialState() {
	    var _this = this;
	
	    this.debouncedNotify = debounce(function () {
	      _this.props.onChange(_this.state.value);
	    }, 200);
	
	    return { value: this.props.value };
	  },
	
	  onChange: function onChange(e) {
	    this.setState({ value: e.target.value });
	    this.debouncedNotify();
	  },
	
	  componentDidMount: function componentDidMount() {},
	
	  componentWillUnmount: function componentWillUnmount() {},
	
	  render: function render() {
	    return React.createElement(
	      'div',
	      { className: 'grv-search' },
	      React.createElement('input', { placeholder: 'Search...', className: 'form-control input-sm',
	        value: this.state.value,
	        onChange: this.onChange })
	    );
	  }
	});
	
	module.exports = InputSearch;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "inputSearch.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 218 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }
	
	var React = __webpack_require__(2);
	var InputSearch = __webpack_require__(217);
	
	var _require = __webpack_require__(49);
	
	var Table = _require.Table;
	var Column = _require.Column;
	var Cell = _require.Cell;
	var SortHeaderCell = _require.SortHeaderCell;
	var SortTypes = _require.SortTypes;
	var EmptyIndicator = _require.EmptyIndicator;
	
	var _require2 = __webpack_require__(223);
	
	var createNewSession = _require2.createNewSession;
	
	var _ = __webpack_require__(43);
	
	var _require3 = __webpack_require__(212);
	
	var isMatch = _require3.isMatch;
	
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
	
	  var props = _objectWithoutProperties(_ref2, ['rowIndex', 'data']);
	
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
	
	  getInitialState: function getInitialState() /*props*/{
	    this.searchableProps = ['addr', 'hostname', 'tags'];
	    return { filter: '', colSortDirs: { hostname: SortTypes.DESC } };
	  },
	
	  onSortChange: function onSortChange(columnKey, sortDir) {
	    var _state$colSortDirs;
	
	    this.state.colSortDirs = (_state$colSortDirs = {}, _state$colSortDirs[columnKey] = sortDir, _state$colSortDirs);
	    this.setState(this.state);
	  },
	
	  onFilterChange: function onFilterChange(value) {
	    this.state.filter = value;
	    this.setState(this.state);
	  },
	
	  searchAndFilterCb: function searchAndFilterCb(targetValue, searchValue, propName) {
	    if (propName === 'tags') {
	      return targetValue.some(function (item) {
	        var role = item.role;
	        var value = item.value;
	
	        return role.toLocaleUpperCase().indexOf(searchValue) !== -1 || value.toLocaleUpperCase().indexOf(searchValue) !== -1;
	      });
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
	          React.createElement(InputSearch, { value: this.filter, onChange: this.onFilterChange })
	        )
	      ),
	      React.createElement(
	        'div',
	        { className: '' },
	        data.length === 0 && this.state.filter.length > 0 ? React.createElement(EmptyIndicator, { text: 'No matching nodes found.' }) : React.createElement(
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
/* 219 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	
	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }
	
	var React = __webpack_require__(2);
	var reactor = __webpack_require__(6);
	
	var _require = __webpack_require__(37);
	
	var Link = _require.Link;
	
	var _require2 = __webpack_require__(50);
	
	var nodeHostNameByServerId = _require2.nodeHostNameByServerId;
	
	var _require3 = __webpack_require__(8);
	
	var displayDateFormat = _require3.displayDateFormat;
	
	var _require4 = __webpack_require__(49);
	
	var Cell = _require4.Cell;
	
	var moment = __webpack_require__(1);
	
	var DateCreatedCell = function DateCreatedCell(_ref) {
	  var rowIndex = _ref.rowIndex;
	  var data = _ref.data;
	
	  var props = _objectWithoutProperties(_ref, ['rowIndex', 'data']);
	
	  var created = data[rowIndex].created;
	  var displayDate = moment(created).format(displayDateFormat);
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
	
	var NodeCell = function NodeCell(_ref7) {
	  var rowIndex = _ref7.rowIndex;
	  var data = _ref7.data;
	
	  var props = _objectWithoutProperties(_ref7, ['rowIndex', 'data']);
	
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
	exports.SingleUserCell = SingleUserCell;
	exports.NodeCell = NodeCell;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "listItems.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 220 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(15);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_APP_INIT: null,
	  TLPT_APP_FAILED: null,
	  TLPT_APP_READY: null
	});
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "actionTypes.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 221 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(12);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(220);
	
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

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "appStore.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 222 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(15);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_CURRENT_SESSION_OPEN: null,
	  TLPT_CURRENT_SESSION_CLOSE: null
	});
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "actionTypes.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 223 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(6);
	var session = __webpack_require__(32);
	var api = __webpack_require__(24);
	var cfg = __webpack_require__(8);
	var getters = __webpack_require__(68);
	var sessionModule = __webpack_require__(345);
	
	var logger = __webpack_require__(23).create('Current Session');
	
	var _require = __webpack_require__(222);
	
	var TLPT_CURRENT_SESSION_OPEN = _require.TLPT_CURRENT_SESSION_OPEN;
	var TLPT_CURRENT_SESSION_CLOSE = _require.TLPT_CURRENT_SESSION_CLOSE;
	
	var actions = {
	
	  close: function close() {
	    var _reactor$evaluate = reactor.evaluate(getters.currentSession);
	
	    var isNewSession = _reactor$evaluate.isNewSession;
	
	    reactor.dispatch(TLPT_CURRENT_SESSION_CLOSE);
	
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
	
	    var _reactor$evaluate2 = reactor.evaluate(getters.currentSession);
	
	    var sid = _reactor$evaluate2.sid;
	
	    logger.info('resize', 'w:' + w + ' and h:' + h);
	    api.put(cfg.api.getTerminalSessionUrl(sid), reqData).done(function () {
	      return logger.info('resized');
	    }).fail(function (err) {
	      return logger.error('failed to resize', err);
	    });
	  },
	
	  openSession: function openSession(sid) {
	    logger.info('attempt to open session', { sid: sid });
	    sessionModule.actions.fetchSession(sid).done(function () {
	      var sView = reactor.evaluate(sessionModule.getters.sessionViewById(sid));
	      var serverId = sView.serverId;
	      var login = sView.login;
	
	      logger.info('open session', 'OK');
	      reactor.dispatch(TLPT_CURRENT_SESSION_OPEN, {
	        serverId: serverId,
	        login: login,
	        sid: sid,
	        isNewSession: false
	      });
	    }).fail(function (err) {
	      logger.error('open session', err);
	      session.getHistory().push(cfg.routes.pageNotFound);
	    });
	  },
	
	  createNewSession: function createNewSession(serverId, login) {
	    var data = { 'session': { 'terminal_params': { 'w': 45, 'h': 5 }, login: login } };
	    api.post(cfg.api.siteSessionPath, data).then(function (json) {
	      var sid = json.session.id;
	      var routeUrl = cfg.getActiveSessionRouteUrl(sid);
	      var history = session.getHistory();
	
	      reactor.dispatch(TLPT_CURRENT_SESSION_OPEN, {
	        serverId: serverId,
	        login: login,
	        sid: sid,
	        isNewSession: true
	      });
	
	      history.push(routeUrl);
	    });
	  }
	};
	
	exports['default'] = actions;
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "actions.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 224 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(12);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(222);
	
	var TLPT_CURRENT_SESSION_OPEN = _require2.TLPT_CURRENT_SESSION_OPEN;
	var TLPT_CURRENT_SESSION_CLOSE = _require2.TLPT_CURRENT_SESSION_CLOSE;
	exports['default'] = Store({
	  getInitialState: function getInitialState() {
	    return toImmutable(null);
	  },
	
	  initialize: function initialize() {
	    this.on(TLPT_CURRENT_SESSION_OPEN, setCurrentSession);
	    this.on(TLPT_CURRENT_SESSION_CLOSE, close);
	  }
	});
	
	function close() {
	  return toImmutable(null);
	}
	
	function setCurrentSession(state, _ref) {
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

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "currentSessionStore.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 225 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	'use strict';
	
	module.exports.getters = __webpack_require__(68);
	module.exports.actions = __webpack_require__(223);
	module.exports.activeTermStore = __webpack_require__(224);
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "index.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 226 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(15);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_DIALOG_SELECT_NODE_SHOW: null,
	  TLPT_DIALOG_SELECT_NODE_CLOSE: null
	});
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "actionTypes.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 227 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(6);
	
	var _require = __webpack_require__(226);
	
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

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "actions.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 228 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(12);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(226);
	
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

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "dialogStore.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 229 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(15);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_NODES_RECEIVE: null
	});
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "actionTypes.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 230 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(15);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_NOTIFICATIONS_ADD: null
	});
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "actionTypes.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 231 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(15);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_REST_API_START: null,
	  TLPT_REST_API_SUCCESS: null,
	  TLPT_REST_API_FAIL: null
	});
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "actionTypes.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 232 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(15);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TRYING_TO_SIGN_UP: null,
	  TRYING_TO_LOGIN: null,
	  FETCHING_INVITE: null
	});
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "constants.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 233 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(15);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_STORED_SESSINS_FILTER_SET_RANGE: null,
	  TLPT_STORED_SESSINS_FILTER_SET_STATUS: null,
	  TLPT_STORED_SESSINS_FILTER_RECEIVE_MORE: null
	});
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "actionTypes.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 234 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(6);
	
	var _require = __webpack_require__(73);
	
	var TLPT_RECEIVE_USER = _require.TLPT_RECEIVE_USER;
	var TLPT_RECEIVE_USER_INVITE = _require.TLPT_RECEIVE_USER_INVITE;
	
	var _require2 = __webpack_require__(232);
	
	var TRYING_TO_SIGN_UP = _require2.TRYING_TO_SIGN_UP;
	var TRYING_TO_LOGIN = _require2.TRYING_TO_LOGIN;
	var FETCHING_INVITE = _require2.FETCHING_INVITE;
	
	var restApiActions = __webpack_require__(342);
	var auth = __webpack_require__(75);
	var session = __webpack_require__(32);
	var cfg = __webpack_require__(8);
	var api = __webpack_require__(24);
	
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
	  },
	
	  ensureUser: function ensureUser(nextState, replace, cb) {
	    auth.ensureUser().done(function (userData) {
	      reactor.dispatch(TLPT_RECEIVE_USER, userData.user);
	      cb();
	    }).fail(function () {
	      var newLocation = {
	        pathname: cfg.routes.login,
	        state: {
	          redirectTo: nextState.location.pathname
	        }
	      };
	
	      replace(newLocation);
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
	    var provider = _ref2.provider;
	
	    if (provider) {
	      var fullPath = cfg.getFullUrl(redirect);
	      window.location = cfg.api.getSsoUrl(fullPath, provider);
	      return;
	    }
	
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

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "actions.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 235 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	module.exports.getters = __webpack_require__(74);
	module.exports.actions = __webpack_require__(234);
	module.exports.nodeStore = __webpack_require__(236);
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "index.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 236 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(12);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(73);
	
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

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "userStore.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 237 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	var api = __webpack_require__(24);
	var cfg = __webpack_require__(8);
	
	var apiUtils = {
	  filterSessions: function filterSessions(_ref) {
	    var start = _ref.start;
	    var end = _ref.end;
	    var sid = _ref.sid;
	    var limit = _ref.limit;
	    var _ref$order = _ref.order;
	    var order = _ref$order === undefined ? -1 : _ref$order;
	
	    var params = {
	      start: start.toISOString(),
	      end: end,
	      order: order,
	      limit: limit
	    };
	
	    if (sid) {
	      params.session_id = sid;
	    }
	
	    return api.get(cfg.api.getFetchSessionsUrl(params));
	  }
	};
	
	module.exports = apiUtils;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "apiUtils.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 238 */,
/* 239 */,
/* 240 */,
/* 241 */,
/* 242 */,
/* 243 */,
/* 244 */,
/* 245 */,
/* 246 */,
/* 247 */,
/* 248 */,
/* 249 */,
/* 250 */,
/* 251 */,
/* 252 */,
/* 253 */,
/* 254 */,
/* 255 */,
/* 256 */,
/* 257 */,
/* 258 */,
/* 259 */,
/* 260 */,
/* 261 */,
/* 262 */,
/* 263 */,
/* 264 */,
/* 265 */,
/* 266 */,
/* 267 */,
/* 268 */,
/* 269 */,
/* 270 */,
/* 271 */,
/* 272 */,
/* 273 */,
/* 274 */,
/* 275 */,
/* 276 */,
/* 277 */,
/* 278 */,
/* 279 */,
/* 280 */,
/* 281 */,
/* 282 */,
/* 283 */,
/* 284 */,
/* 285 */,
/* 286 */
/***/ function(module, exports, __webpack_require__) {

	/**
	 * Copyright 2013-2015, Facebook, Inc.
	 * All rights reserved.
	 *
	 * This source code is licensed under the BSD-style license found in the
	 * LICENSE file in the root directory of this source tree. An additional grant
	 * of patent rights can be found in the PATENTS file in the same directory.
	 *
	 * @providesModule CSSCore
	 * @typechecks
	 */
	
	'use strict';
	
	var invariant = __webpack_require__(3);
	
	/**
	 * The CSSCore module specifies the API (and implements most of the methods)
	 * that should be used when dealing with the display of elements (via their
	 * CSS classes and visibility on screen. It is an API focused on mutating the
	 * display and not reading it as no logical state should be encoded in the
	 * display of elements.
	 */
	
	var CSSCore = {
	
	  /**
	   * Adds the class passed in to the element if it doesn't already have it.
	   *
	   * @param {DOMElement} element the element to set the class on
	   * @param {string} className the CSS className
	   * @return {DOMElement} the element passed in
	   */
	  addClass: function (element, className) {
	    !!/\s/.test(className) ?  false ? invariant(false, 'CSSCore.addClass takes only a single class name. "%s" contains ' + 'multiple classes.', className) : invariant(false) : undefined;
	
	    if (className) {
	      if (element.classList) {
	        element.classList.add(className);
	      } else if (!CSSCore.hasClass(element, className)) {
	        element.className = element.className + ' ' + className;
	      }
	    }
	    return element;
	  },
	
	  /**
	   * Removes the class passed in from the element
	   *
	   * @param {DOMElement} element the element to set the class on
	   * @param {string} className the CSS className
	   * @return {DOMElement} the element passed in
	   */
	  removeClass: function (element, className) {
	    !!/\s/.test(className) ?  false ? invariant(false, 'CSSCore.removeClass takes only a single class name. "%s" contains ' + 'multiple classes.', className) : invariant(false) : undefined;
	
	    if (className) {
	      if (element.classList) {
	        element.classList.remove(className);
	      } else if (CSSCore.hasClass(element, className)) {
	        element.className = element.className.replace(new RegExp('(^|\\s)' + className + '(?:\\s|$)', 'g'), '$1').replace(/\s+/g, ' ') // multiple spaces to one
	        .replace(/^\s*|\s*$/g, ''); // trim the ends
	      }
	    }
	    return element;
	  },
	
	  /**
	   * Helper to add or remove a class from an element based on a condition.
	   *
	   * @param {DOMElement} element the element to set the class on
	   * @param {string} className the CSS className
	   * @param {*} bool condition to whether to add or remove the class
	   * @return {DOMElement} the element passed in
	   */
	  conditionClass: function (element, className, bool) {
	    return (bool ? CSSCore.addClass : CSSCore.removeClass)(element, className);
	  },
	
	  /**
	   * Tests whether the element has the class specified.
	   *
	   * @param {DOMNode|DOMWindow} element the element to set the class on
	   * @param {string} className the CSS className
	   * @return {boolean} true if the element has the class, false if not
	   */
	  hasClass: function (element, className) {
	    !!/\s/.test(className) ?  false ? invariant(false, 'CSS.hasClass takes only a single class name.') : invariant(false) : undefined;
	    if (element.classList) {
	      return !!className && element.classList.contains(className);
	    }
	    return (' ' + element.className + ' ').indexOf(' ' + className + ' ') > -1;
	  }
	
	};
	
	module.exports = CSSCore;

/***/ },
/* 287 */,
/* 288 */,
/* 289 */,
/* 290 */,
/* 291 */,
/* 292 */,
/* 293 */,
/* 294 */,
/* 295 */,
/* 296 */,
/* 297 */,
/* 298 */,
/* 299 */,
/* 300 */,
/* 301 */,
/* 302 */,
/* 303 */,
/* 304 */,
/* 305 */,
/* 306 */,
/* 307 */,
/* 308 */,
/* 309 */,
/* 310 */
/***/ function(module, exports, __webpack_require__) {

	module.exports = __webpack_require__(386);

/***/ },
/* 311 */,
/* 312 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
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

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "patternUtils.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 313 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError('Cannot call a class as a function'); } }
	
	function _inherits(subClass, superClass) { if (typeof superClass !== 'function' && superClass !== null) { throw new TypeError('Super expression must either be null or a function, not ' + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }
	
	var EventEmitter = __webpack_require__(99).EventEmitter;
	
	var logger = __webpack_require__(23).create('TtyEvents');
	
	var TtyEvents = (function (_EventEmitter) {
	  _inherits(TtyEvents, _EventEmitter);
	
	  function TtyEvents() {
	    _classCallCheck(this, TtyEvents);
	
	    _EventEmitter.call(this);
	    this.socket = null;
	  }
	
	  TtyEvents.prototype.connect = function connect(connStr) {
	    var _this = this;
	
	    this.socket = new WebSocket(connStr, 'proto');
	
	    this.socket.onopen = function () {
	      logger.info('Tty event stream is open');
	    };
	
	    this.socket.onmessage = function (event) {
	      try {
	        var json = JSON.parse(event.data);
	        _this.emit('data', json.session);
	      } catch (err) {
	        logger.error('failed to parse event stream data', err);
	      }
	    };
	
	    this.socket.onclose = function () {
	      logger.info('Tty event stream is closed');
	    };
	  };
	
	  TtyEvents.prototype.disconnect = function disconnect() {
	    this.socket.close();
	  };
	
	  return TtyEvents;
	})(EventEmitter);
	
	module.exports = TtyEvents;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "ttyEvents.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 314 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	
	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError('Cannot call a class as a function'); } }
	
	function _inherits(subClass, superClass) { if (typeof superClass !== 'function' && superClass !== null) { throw new TypeError('Super expression must either be null or a function, not ' + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }
	
	var Tty = __webpack_require__(214);
	var api = __webpack_require__(24);
	var cfg = __webpack_require__(8);
	
	var _require = __webpack_require__(51);
	
	var showError = _require.showError;
	
	var Buffer = __webpack_require__(61).Buffer;
	
	var logger = __webpack_require__(23).create('TtyPlayer');
	var STREAM_START_INDEX = 1;
	var PRE_FETCH_BUF_SIZE = 5000;
	
	function handleAjaxError(err) {
	  showError('Unable to retrieve session info');
	  logger.error('fetching session length', err);
	}
	
	var TtyPlayer = (function (_Tty) {
	  _inherits(TtyPlayer, _Tty);
	
	  function TtyPlayer(_ref) {
	    var sid = _ref.sid;
	
	    _classCallCheck(this, TtyPlayer);
	
	    _Tty.call(this, {});
	    this.sid = sid;
	    this.current = STREAM_START_INDEX;
	    this.length = -1;
	    this.ttyStream = new Array();
	    this.isPlaying = false;
	    this.isError = false;
	    this.isReady = false;
	    this.isLoading = true;
	  }
	
	  TtyPlayer.prototype.send = function send() {};
	
	  TtyPlayer.prototype.resize = function resize() {};
	
	  TtyPlayer.prototype.getDimensions = function getDimensions() {
	    var chunkInfo = this.ttyStream[this.current - 1];
	    if (chunkInfo) {
	      return {
	        w: chunkInfo.w,
	        h: chunkInfo.h
	      };
	    } else {
	      return { w: undefined, h: undefined };
	    }
	  };
	
	  TtyPlayer.prototype._handleError = function _handleError() {};
	
	  TtyPlayer.prototype._fetchEvents = function _fetchEvents() {
	
	    api.get('/v1/webapi/sites/-current-/sessions/' + this.sid + '/events');
	    api.get('/v1/webapi/sites/-current-/sessions/' + this.sid + '/stream');
	    //.then(this._initEvents.bind(this))
	    //.fail(handleAjaxError)
	  };
	
	  TtyPlayer.prototype.connect = function connect() {
	    var _this = this;
	
	    this._setStatusFlag({ isLoading: true });
	
	    this._fetchEvents();
	    api.get(cfg.api.getFetchSessionLengthUrl(this.sid)).done(function (data) {
	      /*
	      * temporary hotfix to back-end issue related to session chunks starting at
	      * index=1 and ending at index=length+1
	      **/
	      _this.length = data.count + 1;
	      _this._setStatusFlag({ isReady: true });
	    }).fail(function (err) {
	      handleAjaxError(err);
	    }).always(function () {
	      _this._change();
	    });
	
	    this._change();
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
	      newPos = STREAM_START_INDEX;
	    }
	
	    if (this.current < newPos) {
	      this._showChunk(this.current, newPos);
	    } else {
	      this.emit('reset');
	      this._showChunk(STREAM_START_INDEX, newPos);
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
	      this.current = STREAM_START_INDEX;
	      this.emit('reset');
	    }
	
	    this.timer = setInterval(this.move.bind(this), 150);
	    this._change();
	  };
	
	  TtyPlayer.prototype._shouldFetch = function _shouldFetch(start, end) {
	    for (var i = start; i < end; i++) {
	      if (this.ttyStream[i] === undefined) {
	        return true;
	      }
	    }
	
	    return false;
	  };
	
	  TtyPlayer.prototype._fetch = function _fetch(start, end) {
	    var _this2 = this;
	
	    end = end + PRE_FETCH_BUF_SIZE;
	    end = end > this.length ? this.length : end;
	
	    this._setStatusFlag({ isLoading: true });
	
	    return api.get(cfg.api.getFetchSessionChunkUrl({ sid: this.sid, start: start, end: end })).done(function (response) {
	      for (var i = 0; i < end - start; i++) {
	        var _response$chunks$i = response.chunks[i];
	        var data = _response$chunks$i.data;
	        var delay = _response$chunks$i.delay;
	        var _response$chunks$i$term = _response$chunks$i.term;
	        var h = _response$chunks$i$term.h;
	        var w = _response$chunks$i$term.w;
	
	        data = new Buffer(data, 'base64').toString('utf8');
	        _this2.ttyStream[start + i] = { data: data, delay: delay, w: w, h: h };
	      }
	
	      _this2._setStatusFlag({ isReady: true });
	    }).fail(function (err) {
	      handleAjaxError(err);
	      _this2._setStatusFlag({ isError: true });
	    });
	  };
	
	  TtyPlayer.prototype._display = function _display(start, end) {
	    var stream = this.ttyStream;
	    var i = undefined;
	    var tmp = [{
	      data: [stream[start].data],
	      w: stream[start].w,
	      h: stream[start].h
	    }];
	
	    var cur = tmp[0];
	
	    for (i = start + 1; i < end; i++) {
	      if (cur.w === stream[i].w && cur.h === stream[i].h) {
	        cur.data.push(stream[i].data);
	      } else {
	        cur = {
	          data: [stream[i].data],
	          w: stream[i].w,
	          h: stream[i].h
	        };
	
	        tmp.push(cur);
	      }
	    }
	
	    for (i = 0; i < tmp.length; i++) {
	      var str = tmp[i].data.join('');
	      var _tmp$i = tmp[i];
	      var h = _tmp$i.h;
	      var w = _tmp$i.w;
	
	      this.emit('resize', { h: h, w: w });
	      this.emit('data', str);
	    }
	
	    this.current = end;
	  };
	
	  TtyPlayer.prototype._showChunk = function _showChunk(start, end) {
	    var _this3 = this;
	
	    if (this._shouldFetch(start, end)) {
	      this._fetch(start, end).then(function () {
	        return _this3._display(start, end);
	      });
	    } else {
	      this._display(start, end);
	    }
	  };
	
	  TtyPlayer.prototype._setStatusFlag = function _setStatusFlag(newStatus) {
	    var _newStatus$isReady = newStatus.isReady;
	    var isReady = _newStatus$isReady === undefined ? false : _newStatus$isReady;
	    var _newStatus$isError = newStatus.isError;
	    var isError = _newStatus$isError === undefined ? false : _newStatus$isError;
	    var _newStatus$isLoading = newStatus.isLoading;
	    var isLoading = _newStatus$isLoading === undefined ? false : _newStatus$isLoading;
	
	    this.isReady = isReady;
	    this.isError = isError;
	    this.isLoading = isLoading;
	  };
	
	  TtyPlayer.prototype._change = function _change() {
	    this.emit('change');
	  };
	
	  return TtyPlayer;
	})(Tty);
	
	exports['default'] = TtyPlayer;
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "ttyPlayer.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 315 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	var React = __webpack_require__(2);
	var NavLeftBar = __webpack_require__(322);
	var reactor = __webpack_require__(6);
	
	var _require = __webpack_require__(334);
	
	var actions = _require.actions;
	var getters = _require.getters;
	
	var SelectNodeDialog = __webpack_require__(326);
	var NotificationHost = __webpack_require__(325);
	
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
	      React.createElement(NotificationHost, null),
	      this.props.CurrentSessionHost,
	      React.createElement(NavLeftBar, null),
	      this.props.children
	    );
	  }
	});
	
	module.exports = App;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "app.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 316 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };
	
	var React = __webpack_require__(2);
	var reactor = __webpack_require__(6);
	
	var _require = __webpack_require__(50);
	
	var nodeHostNameByServerId = _require.nodeHostNameByServerId;
	
	var TtyTerminal = __webpack_require__(330);
	var SessionLeftPanel = __webpack_require__(215);
	
	var ActiveSession = React.createClass({
	  displayName: 'ActiveSession',
	
	  render: function render() {
	    var _props = this.props;
	    var login = _props.login;
	    var parties = _props.parties;
	    var serverId = _props.serverId;
	
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
	      React.createElement(TtyTerminal, _extends({ ref: 'ttyCmntInstance' }, this.props))
	    );
	  }
	});
	
	module.exports = ActiveSession;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "activeSession.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 317 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	var React = __webpack_require__(2);
	var reactor = __webpack_require__(6);
	
	var _require = __webpack_require__(225);
	
	var getters = _require.getters;
	var actions = _require.actions;
	
	var SessionPlayer = __webpack_require__(318);
	var ActiveSession = __webpack_require__(316);
	
	var CurrentSessionHost = React.createClass({
	  displayName: 'CurrentSessionHost',
	
	  mixins: [reactor.ReactMixin],
	
	  getDataBindings: function getDataBindings() {
	    return {
	      currentSession: getters.currentSession
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
	      return React.createElement(ActiveSession, currentSession);
	    }
	
	    return React.createElement(SessionPlayer, currentSession);
	  }
	});
	
	module.exports = CurrentSessionHost;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "main.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 318 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	
	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError('Cannot call a class as a function'); } }
	
	function _inherits(subClass, superClass) { if (typeof superClass !== 'function' && superClass !== null) { throw new TypeError('Super expression must either be null or a function, not ' + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }
	
	var React = __webpack_require__(2);
	var ReactSlider = __webpack_require__(245);
	var TtyPlayer = __webpack_require__(314);
	var Terminal = __webpack_require__(213);
	var SessionLeftPanel = __webpack_require__(215);
	
	var MyTerminal = (function (_Terminal) {
	  _inherits(MyTerminal, _Terminal);
	
	  function MyTerminal(tty, el) {
	    _classCallCheck(this, MyTerminal);
	
	    _Terminal.call(this, { el: el });
	    this.tty = tty;
	  }
	
	  MyTerminal.prototype.connect = function connect() {
	    this.tty.connect();
	  };
	
	  return MyTerminal;
	})(Terminal);
	
	var TerminalPlayer = React.createClass({
	  displayName: 'TerminalPlayer',
	
	  componentDidMount: function componentDidMount() {
	    this.terminal = new MyTerminal(this.props.tty, this.refs.container);
	    this.terminal.open();
	  },
	
	  componentWillUnmount: function componentWillUnmount() {
	    this.terminal.destroy();
	  },
	
	  shouldComponentUpdate: function shouldComponentUpdate() {
	    return false;
	  },
	
	  render: function render() {
	    return React.createElement(
	      'div',
	      { ref: 'container' },
	      '  '
	    );
	  }
	});
	
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
	    var sid = this.props.sid;
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
	
	    this.tty.play();
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
	      React.createElement(TerminalPlayer, { ref: 'term', tty: this.tty, scrollback: 0 }),
	      React.createElement(
	        'div',
	        { className: 'grv-session-player-controls' },
	        React.createElement(
	          'button',
	          { className: 'btn', onClick: this.togglePlayStop },
	          isPlaying ? React.createElement('i', { className: 'fa fa-stop' }) : React.createElement('i', { className: 'fa fa-play' })
	        ),
	        React.createElement(
	          'div',
	          { className: 'grv-flex-column' },
	          React.createElement(ReactSlider, {
	            min: this.state.min,
	            max: this.state.length,
	            value: this.state.current,
	            onChange: this.move,
	            defaultValue: 1,
	            withBars: true,
	            className: 'grv-slider' })
	        )
	      )
	    );
	  }
	});
	
	exports['default'] = SessionPlayer;
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "sessionPlayer.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 319 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	var React = __webpack_require__(2);
	var $ = __webpack_require__(22);
	var moment = __webpack_require__(1);
	
	var _require = __webpack_require__(43);
	
	var debounce = _require.debounce;
	
	var DateRangePicker = React.createClass({
	  displayName: 'DateRangePicker',
	
	  getDates: function getDates() {
	    var startDate = $(this.refs.dpPicker1).datepicker('getDate');
	    var endDate = $(this.refs.dpPicker2).datepicker('getDate');
	    return [startDate, moment(endDate).endOf('day').toDate()];
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
	
	    var displayValue = moment(value).format('MMM Do, YYYY');
	
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
	
	    var newValue = moment(value).add(at, 'week').toDate();
	    this.props.onValueChange(newValue);
	  }
	});
	
	CalendarNav.getweekRange = function (value) {
	  var startDate = moment(value).startOf('month').toDate();
	  var endDate = moment(value).endOf('month').toDate();
	  return [startDate, endDate];
	};
	
	exports['default'] = DateRangePicker;
	exports.CalendarNav = CalendarNav;
	exports.DateRangePicker = DateRangePicker;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "datePicker.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 320 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	module.exports.App = __webpack_require__(315);
	module.exports.Login = __webpack_require__(321);
	module.exports.NewUser = __webpack_require__(323);
	module.exports.Nodes = __webpack_require__(324);
	module.exports.Sessions = __webpack_require__(328);
	module.exports.CurrentSessionHost = __webpack_require__(317);
	module.exports.ErrorPage = __webpack_require__(48).ErrorPage;
	module.exports.NotFound = __webpack_require__(48).NotFound;
	module.exports.MessagePage = __webpack_require__(48).MessagePage;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "index.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 321 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	var React = __webpack_require__(2);
	var $ = __webpack_require__(22);
	var reactor = __webpack_require__(6);
	var LinkedStateMixin = __webpack_require__(67);
	
	var _require = __webpack_require__(235);
	
	var actions = _require.actions;
	var getters = _require.getters;
	
	var GoogleAuthInfo = __webpack_require__(216);
	var cfg = __webpack_require__(8);
	
	var _require2 = __webpack_require__(47);
	
	var TeleportLogo = _require2.TeleportLogo;
	
	var _require3 = __webpack_require__(75);
	
	var PROVIDER_GOOGLE = _require3.PROVIDER_GOOGLE;
	
	var LoginInputForm = React.createClass({
	  displayName: 'LoginInputForm',
	
	  mixins: [LinkedStateMixin],
	
	  getInitialState: function getInitialState() {
	    return {
	      user: '',
	      password: '',
	      token: '',
	      provider: null
	    };
	  },
	
	  onLogin: function onLogin(e) {
	    e.preventDefault();
	    if (this.isValid()) {
	      this.props.onClick(this.state);
	    }
	  },
	
	  onLoginWithGoogle: function onLoginWithGoogle(e) {
	    e.preventDefault();
	    this.state.provider = PROVIDER_GOOGLE;
	    this.props.onClick(this.state);
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
	
	    var providers = cfg.getAuthProviders();
	    var useGoogle = providers.indexOf(PROVIDER_GOOGLE) !== -1;
	
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
	          React.createElement('input', { autoComplete: 'off', valueLink: this.linkState('token'), className: 'form-control required', name: 'token', placeholder: 'Two factor token (Google Authenticator)' })
	        ),
	        React.createElement(
	          'button',
	          { onClick: this.onLogin, disabled: isProcessing, type: 'submit', className: 'btn btn-primary block full-width m-b' },
	          'Login'
	        ),
	        useGoogle ? React.createElement(
	          'button',
	          { onClick: this.onLoginWithGoogle, type: 'submit', className: 'btn btn-danger block full-width m-b' },
	          'With Google'
	        ) : null,
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
	      React.createElement(TeleportLogo, null),
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
/* 322 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	'use strict';
	
	var React = __webpack_require__(2);
	var reactor = __webpack_require__(6);
	
	var _require = __webpack_require__(37);
	
	var IndexLink = _require.IndexLink;
	
	var getters = __webpack_require__(74);
	var cfg = __webpack_require__(8);
	
	var _require2 = __webpack_require__(47);
	
	var UserIcon = _require2.UserIcon;
	
	var menuItems = [{ icon: 'fa fa-share-alt', to: cfg.routes.nodes, title: 'Nodes' }, { icon: 'fa  fa-group', to: cfg.routes.sessions, title: 'Sessions' }];
	
	var NavLeftBar = React.createClass({
	  displayName: 'NavLeftBar',
	
	  render: function render() {
	    var _this = this;
	
	    var _reactor$evaluate = reactor.evaluate(getters.user);
	
	    var name = _reactor$evaluate.name;
	
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
	        React.createElement('i', { className: 'fa fa-sign-out', style: { marginRight: 0 } })
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
	          null,
	          React.createElement(UserIcon, { name: name })
	        ),
	        items
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
/* 323 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	var React = __webpack_require__(2);
	var $ = __webpack_require__(22);
	var reactor = __webpack_require__(6);
	
	var _require = __webpack_require__(235);
	
	var actions = _require.actions;
	var getters = _require.getters;
	
	var LinkedStateMixin = __webpack_require__(67);
	var GoogleAuthInfo = __webpack_require__(216);
	
	var _require2 = __webpack_require__(48);
	
	var ErrorPage = _require2.ErrorPage;
	var ErrorTypes = _require2.ErrorTypes;
	
	var _require3 = __webpack_require__(47);
	
	var TeleportLogo = _require3.TeleportLogo;
	
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
	      actions.signUp({
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
	            disabled: true,
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
	            autoComplete: 'off',
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
	      return React.createElement(ErrorPage, { type: ErrorTypes.EXPIRED_INVITE });
	    }
	
	    if (!invite) {
	      return null;
	    }
	
	    return React.createElement(
	      'div',
	      { className: 'grv-invite text-center' },
	      React.createElement(TeleportLogo, null),
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
/* 324 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	var React = __webpack_require__(2);
	var reactor = __webpack_require__(6);
	var userGetters = __webpack_require__(74);
	var nodeGetters = __webpack_require__(50);
	var NodeList = __webpack_require__(218);
	
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
/* 325 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	var React = __webpack_require__(2);
	var reactor = __webpack_require__(6);
	var PureRenderMixin = __webpack_require__(209);
	
	var _require = __webpack_require__(340);
	
	var lastMessage = _require.lastMessage;
	
	var _require2 = __webpack_require__(247);
	
	var ToastContainer = _require2.ToastContainer;
	var ToastMessage = _require2.ToastMessage;
	
	var ToastMessageFactory = React.createFactory(ToastMessage.animation);
	
	var animationOptions = {
	  showAnimation: 'animated fadeIn',
	  hideAnimation: 'animated fadeOut'
	};
	
	var NotificationHost = React.createClass({
	  displayName: 'NotificationHost',
	
	  mixins: [reactor.ReactMixin, PureRenderMixin],
	
	  getDataBindings: function getDataBindings() {
	    return { msg: lastMessage };
	  },
	
	  update: function update(msg) {
	    if (msg) {
	      if (msg.isError) {
	        this.refs.container.error(msg.text, msg.title, animationOptions);
	      } else if (msg.isWarning) {
	        this.refs.container.warning(msg.text, msg.title, animationOptions);
	      } else if (msg.isSuccess) {
	        this.refs.container.success(msg.text, msg.title, animationOptions);
	      } else {
	        this.refs.container.info(msg.text, msg.title, animationOptions);
	      }
	    }
	  },
	
	  componentDidMount: function componentDidMount() {
	    reactor.observe(lastMessage, this.update);
	  },
	
	  componentWillUnmount: function componentWillUnmount() {
	    reactor.unobserve(lastMessage, this.update);
	  },
	
	  render: function render() {
	    return React.createElement(ToastContainer, {
	      ref: 'container', toastMessageFactory: ToastMessageFactory, className: 'toast-top-right' });
	  }
	});
	
	module.exports = NotificationHost;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "notificationHost.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 326 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	var React = __webpack_require__(2);
	var reactor = __webpack_require__(6);
	
	var _require = __webpack_require__(336);
	
	var getters = _require.getters;
	
	var _require2 = __webpack_require__(227);
	
	var closeSelectNodeDialog = _require2.closeSelectNodeDialog;
	
	var NodeList = __webpack_require__(218);
	var currentSessionGetters = __webpack_require__(68);
	var nodeGetters = __webpack_require__(50);
	var $ = __webpack_require__(22);
	
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
	
	  onLoginClick: function onLoginClick(serverId) {
	    if (SelectNodeDialog.onServerChangeCallBack) {
	      SelectNodeDialog.onServerChangeCallBack({ serverId: serverId });
	    }
	
	    closeSelectNodeDialog();
	  },
	
	  componentWillUnmount: function componentWillUnmount() {
	    $('.modal').modal('hide');
	  },
	
	  componentDidMount: function componentDidMount() {
	    $('.modal').modal('show');
	  },
	
	  render: function render() {
	    var activeSession = reactor.evaluate(currentSessionGetters.currentSession) || {};
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
/* 327 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	var React = __webpack_require__(2);
	
	var _require = __webpack_require__(49);
	
	var Table = _require.Table;
	var Column = _require.Column;
	var Cell = _require.Cell;
	var TextCell = _require.TextCell;
	var EmptyIndicator = _require.EmptyIndicator;
	
	var _require2 = __webpack_require__(219);
	
	var ButtonCell = _require2.ButtonCell;
	var UsersCell = _require2.UsersCell;
	var NodeCell = _require2.NodeCell;
	var DateCreatedCell = _require2.DateCreatedCell;
	
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
	        data.length === 0 ? React.createElement(EmptyIndicator, { text: 'You have no active sessions.' }) : React.createElement(
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
/* 328 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	var React = __webpack_require__(2);
	var reactor = __webpack_require__(6);
	
	var _require = __webpack_require__(71);
	
	var sessionsView = _require.sessionsView;
	
	var _require2 = __webpack_require__(72);
	
	var filter = _require2.filter;
	
	var StoredSessionList = __webpack_require__(329);
	var ActiveSessionList = __webpack_require__(327);
	
	var Sessions = React.createClass({
	  displayName: 'Sessions',
	
	  mixins: [reactor.ReactMixin],
	
	  getDataBindings: function getDataBindings() {
	    return {
	      data: sessionsView,
	      storedSessionsFilter: filter
	    };
	  },
	
	  render: function render() {
	    var _state = this.state;
	    var data = _state.data;
	    var storedSessionsFilter = _state.storedSessionsFilter;
	
	    return React.createElement(
	      'div',
	      { className: 'grv-sessions grv-page' },
	      React.createElement(ActiveSessionList, { data: data }),
	      React.createElement('hr', { className: 'grv-divider' }),
	      React.createElement(StoredSessionList, { data: data, filter: storedSessionsFilter })
	    );
	  }
	});
	
	module.exports = Sessions;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "main.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 329 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	var React = __webpack_require__(2);
	
	var _require = __webpack_require__(348);
	
	var actions = _require.actions;
	
	var InputSearch = __webpack_require__(217);
	
	var _require2 = __webpack_require__(49);
	
	var Table = _require2.Table;
	var Column = _require2.Column;
	var Cell = _require2.Cell;
	var TextCell = _require2.TextCell;
	var SortHeaderCell = _require2.SortHeaderCell;
	var SortTypes = _require2.SortTypes;
	var EmptyIndicator = _require2.EmptyIndicator;
	
	var _require3 = __webpack_require__(219);
	
	var ButtonCell = _require3.ButtonCell;
	var SingleUserCell = _require3.SingleUserCell;
	var DateCreatedCell = _require3.DateCreatedCell;
	
	var _require4 = __webpack_require__(319);
	
	var DateRangePicker = _require4.DateRangePicker;
	
	var moment = __webpack_require__(1);
	
	var _require5 = __webpack_require__(212);
	
	var isMatch = _require5.isMatch;
	
	var _ = __webpack_require__(43);
	
	var _require6 = __webpack_require__(8);
	
	var displayDateFormat = _require6.displayDateFormat;
	
	var ArchivedSessions = React.createClass({
	  displayName: 'ArchivedSessions',
	
	  getInitialState: function getInitialState() {
	    this.searchableProps = ['serverIp', 'created', 'sid', 'login'];
	    return { filter: '', colSortDirs: { created: 'ASC' } };
	  },
	
	  componentWillMount: function componentWillMount() {
	    setTimeout(function () {
	      return actions.fetch();
	    }, 0);
	  },
	
	  componentWillUnmount: function componentWillUnmount() {
	    actions.removeStoredSessions();
	  },
	
	  onFilterChange: function onFilterChange(value) {
	    this.state.filter = value;
	    this.setState(this.state);
	  },
	
	  onSortChange: function onSortChange(columnKey, sortDir) {
	    var _state$colSortDirs;
	
	    this.state.colSortDirs = (_state$colSortDirs = {}, _state$colSortDirs[columnKey] = sortDir, _state$colSortDirs);
	    this.setState(this.state);
	  },
	
	  onRangePickerChange: function onRangePickerChange(_ref) {
	    var startDate = _ref.startDate;
	    var endDate = _ref.endDate;
	
	    actions.setTimeRange(startDate, endDate);
	  },
	
	  searchAndFilterCb: function searchAndFilterCb(targetValue, searchValue, propName) {
	    if (propName === 'created') {
	      var displayDate = moment(targetValue).format(displayDateFormat).toLocaleUpperCase();
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
	    var _props$filter = this.props.filter;
	    var start = _props$filter.start;
	    var end = _props$filter.end;
	    var status = _props$filter.status;
	
	    var data = this.props.data.filter(function (item) {
	      return !item.active && moment(item.created).isBetween(start, end);
	    });
	
	    data = this.sortAndFilter(data);
	
	    return React.createElement(
	      'div',
	      { className: 'grv-sessions-stored' },
	      React.createElement(
	        'div',
	        { className: 'grv-header' },
	        React.createElement(
	          'div',
	          { className: 'grv-flex' },
	          React.createElement('div', { className: 'grv-flex-column' }),
	          React.createElement(
	            'div',
	            { className: 'grv-flex-column' },
	            React.createElement(
	              'h1',
	              null,
	              ' Archived Sessions '
	            )
	          ),
	          React.createElement(
	            'div',
	            { className: 'grv-flex-column' },
	            React.createElement(InputSearch, { value: this.filter, onChange: this.onFilterChange })
	          )
	        ),
	        React.createElement(
	          'div',
	          { className: 'grv-flex' },
	          React.createElement('div', { className: 'grv-flex-row' }),
	          React.createElement(
	            'div',
	            { className: 'grv-flex-row' },
	            React.createElement(DateRangePicker, { startDate: start, endDate: end, onChange: this.onRangePickerChange })
	          ),
	          React.createElement('div', { className: 'grv-flex-row' })
	        )
	      ),
	      React.createElement(
	        'div',
	        { className: 'grv-content' },
	        data.length === 0 && !status.isLoading ? React.createElement(EmptyIndicator, { text: 'No matching archived sessions found.' }) : React.createElement(
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
	      ),
	      status.hasMore ? React.createElement(
	        'div',
	        { className: 'grv-footer' },
	        React.createElement(
	          'button',
	          { disabled: status.isLoading, className: 'btn btn-primary btn-outline', onClick: actions.fetchMore },
	          React.createElement(
	            'span',
	            null,
	            'Load more...'
	          )
	        )
	      ) : null
	    );
	  }
	});
	
	module.exports = ArchivedSessions;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "storedSessionList.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 330 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	var React = __webpack_require__(2);
	var cfg = __webpack_require__(8);
	var session = __webpack_require__(32);
	var Terminal = __webpack_require__(213);
	
	var _require = __webpack_require__(70);
	
	var updateSession = _require.updateSession;
	
	var TtyTerminal = React.createClass({
	  displayName: 'TtyTerminal',
	
	  componentDidMount: function componentDidMount() {
	    var _props = this.props;
	    var serverId = _props.serverId;
	    var login = _props.login;
	    var sid = _props.sid;
	    var rows = _props.rows;
	    var cols = _props.cols;
	
	    var _session$getUserData = session.getUserData();
	
	    var token = _session$getUserData.token;
	
	    var url = cfg.api.getTtyUrl();
	
	    var options = {
	      tty: {
	        serverId: serverId, login: login, sid: sid, token: token, url: url
	      },
	      rows: rows,
	      cols: cols,
	      el: this.refs.container
	    };
	
	    this.terminal = new Terminal(options);
	    this.terminal.ttyEvents.on('data', updateSession);
	    this.terminal.open();
	  },
	
	  componentWillUnmount: function componentWillUnmount() {
	    this.terminal.destroy();
	  },
	
	  shouldComponentUpdate: function shouldComponentUpdate() {
	    return false;
	  },
	
	  render: function render() {
	    return React.createElement(
	      'div',
	      { ref: 'container' },
	      '  '
	    );
	  }
	});
	
	module.exports = TtyTerminal;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "terminal.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 331 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	var React = __webpack_require__(2);
	var render = __webpack_require__(211).render;
	
	var _require = __webpack_require__(37);
	
	var Router = _require.Router;
	var Route = _require.Route;
	var Redirect = _require.Redirect;
	
	var _require2 = __webpack_require__(320);
	
	var App = _require2.App;
	var Login = _require2.Login;
	var Nodes = _require2.Nodes;
	var Sessions = _require2.Sessions;
	var NewUser = _require2.NewUser;
	var CurrentSessionHost = _require2.CurrentSessionHost;
	var MessagePage = _require2.MessagePage;
	var NotFound = _require2.NotFound;
	
	var _require3 = __webpack_require__(234);
	
	var ensureUser = _require3.ensureUser;
	
	var auth = __webpack_require__(75);
	var session = __webpack_require__(32);
	var cfg = __webpack_require__(8);
	
	__webpack_require__(337);
	
	// init session
	session.init();
	
	cfg.init(window.GRV_CONFIG);
	
	render(React.createElement(
	  Router,
	  { history: session.getHistory() },
	  React.createElement(Route, { path: cfg.routes.msgs, component: MessagePage }),
	  React.createElement(Route, { path: cfg.routes.login, component: Login }),
	  React.createElement(Route, { path: cfg.routes.logout, onEnter: auth.logout }),
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
/* 332 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(6);
	
	var _require = __webpack_require__(70);
	
	var fetchSessions = _require.fetchSessions;
	
	var _require2 = __webpack_require__(338);
	
	var fetchNodes = _require2.fetchNodes;
	
	var $ = __webpack_require__(22);
	
	var _require3 = __webpack_require__(220);
	
	var TLPT_APP_INIT = _require3.TLPT_APP_INIT;
	var TLPT_APP_FAILED = _require3.TLPT_APP_FAILED;
	var TLPT_APP_READY = _require3.TLPT_APP_READY;
	
	var actions = {
	
	  initApp: function initApp() {
	    reactor.dispatch(TLPT_APP_INIT);
	    actions.fetchNodesAndSessions().done(function () {
	      return reactor.dispatch(TLPT_APP_READY);
	    }).fail(function () {
	      return reactor.dispatch(TLPT_APP_FAILED);
	    });
	  },
	
	  fetchNodesAndSessions: function fetchNodesAndSessions() {
	    return $.when(fetchNodes(), fetchSessions());
	  }
	};
	
	exports['default'] = actions;
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "actions.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 333 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	var appState = [['tlpt'], function (app) {
	  return app.toJS();
	}];
	
	exports['default'] = {
	  appState: appState
	};
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "getters.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 334 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	'use strict';
	
	module.exports.getters = __webpack_require__(333);
	module.exports.actions = __webpack_require__(332);
	module.exports.appStore = __webpack_require__(221);
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "index.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 335 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	var dialogs = [['tlpt_dialogs'], function (state) {
	  return state.toJS();
	}];
	
	exports['default'] = {
	  dialogs: dialogs
	};
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "getters.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 336 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	module.exports.getters = __webpack_require__(335);
	module.exports.actions = __webpack_require__(227);
	module.exports.dialogStore = __webpack_require__(228);
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "index.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 337 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	var reactor = __webpack_require__(6);
	reactor.registerStores({
	  'tlpt': __webpack_require__(221),
	  'tlpt_dialogs': __webpack_require__(228),
	  'tlpt_current_session': __webpack_require__(224),
	  'tlpt_user': __webpack_require__(236),
	  'tlpt_user_invite': __webpack_require__(350),
	  'tlpt_nodes': __webpack_require__(339),
	  'tlpt_rest_api': __webpack_require__(344),
	  'tlpt_sessions': __webpack_require__(346),
	  'tlpt_stored_sessions_filter': __webpack_require__(349),
	  'tlpt_notifications': __webpack_require__(341)
	});
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "index.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 338 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(6);
	
	var _require = __webpack_require__(229);
	
	var TLPT_NODES_RECEIVE = _require.TLPT_NODES_RECEIVE;
	
	var api = __webpack_require__(24);
	var cfg = __webpack_require__(8);
	
	var _require2 = __webpack_require__(51);
	
	var showError = _require2.showError;
	
	var logger = __webpack_require__(23).create('Modules/Nodes');
	
	exports['default'] = {
	  fetchNodes: function fetchNodes() {
	    api.get(cfg.api.nodesPath).done(function () {
	      var data = arguments.length <= 0 || arguments[0] === undefined ? [] : arguments[0];
	
	      var nodeArray = data.nodes.map(function (item) {
	        return item.node;
	      });
	      reactor.dispatch(TLPT_NODES_RECEIVE, nodeArray);
	    }).fail(function (err) {
	      showError('Unable to retrieve list of nodes');
	      logger.error('fetchNodes', err);
	    });
	  }
	};
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "actions.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 339 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(12);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(229);
	
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

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "nodeStore.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 340 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	var lastMessage = [['tlpt_notifications'], function (notifications) {
	    return notifications.last();
	}];
	exports.lastMessage = lastMessage;

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "getters.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 341 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	
	var _nuclearJs = __webpack_require__(12);
	
	var _actionTypes = __webpack_require__(230);
	
	exports['default'] = _nuclearJs.Store({
	  getInitialState: function getInitialState() {
	    return new _nuclearJs.Immutable.OrderedMap();
	  },
	
	  initialize: function initialize() {
	    this.on(_actionTypes.TLPT_NOTIFICATIONS_ADD, addNotification);
	  }
	});
	
	function addNotification(state, message) {
	  return state.set(state.size, message);
	}
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "notificationStore.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 342 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(6);
	
	var _require = __webpack_require__(231);
	
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

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "actions.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 343 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
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

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "getters.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 344 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(12);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(231);
	
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

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "restApiStore.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 345 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	module.exports.getters = __webpack_require__(71);
	module.exports.actions = __webpack_require__(70);
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "index.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 346 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(12);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(69);
	
	var TLPT_SESSINS_RECEIVE = _require2.TLPT_SESSINS_RECEIVE;
	var TLPT_SESSINS_UPDATE = _require2.TLPT_SESSINS_UPDATE;
	var TLPT_SESSINS_REMOVE_STORED = _require2.TLPT_SESSINS_REMOVE_STORED;
	exports['default'] = Store({
	  getInitialState: function getInitialState() {
	    return toImmutable({});
	  },
	
	  initialize: function initialize() {
	    this.on(TLPT_SESSINS_RECEIVE, receiveSessions);
	    this.on(TLPT_SESSINS_UPDATE, updateSession);
	    this.on(TLPT_SESSINS_REMOVE_STORED, removeStoredSessions);
	  }
	});
	
	function removeStoredSessions(state) {
	  return state.withMutations(function (state) {
	    state.valueSeq().forEach(function (item) {
	      if (item.get('active') !== true) {
	        state.remove(item.get('id'));
	      }
	    });
	  });
	}
	
	function updateSession(state, json) {
	  return state.set(json.id, toImmutable(json));
	}
	
	function receiveSessions(state, jsonArray) {
	  jsonArray = jsonArray || [];
	
	  return state.withMutations(function (state) {
	    jsonArray.forEach(function (item) {
	      item.created = new Date(item.created);
	      item.last_active = new Date(item.last_active);
	      state.set(item.id, toImmutable(item));
	    });
	  });
	}
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "sessionStore.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 347 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(6);
	
	var _require = __webpack_require__(72);
	
	var filter = _require.filter;
	
	var _require2 = __webpack_require__(8);
	
	var maxSessionLoadSize = _require2.maxSessionLoadSize;
	
	var moment = __webpack_require__(1);
	var apiUtils = __webpack_require__(237);
	
	var _require3 = __webpack_require__(51);
	
	var showError = _require3.showError;
	
	var logger = __webpack_require__(23).create('Modules/Sessions');
	
	var _require4 = __webpack_require__(233);
	
	var TLPT_STORED_SESSINS_FILTER_SET_RANGE = _require4.TLPT_STORED_SESSINS_FILTER_SET_RANGE;
	var TLPT_STORED_SESSINS_FILTER_SET_STATUS = _require4.TLPT_STORED_SESSINS_FILTER_SET_STATUS;
	
	var _require5 = __webpack_require__(69);
	
	var TLPT_SESSINS_RECEIVE = _require5.TLPT_SESSINS_RECEIVE;
	var TLPT_SESSINS_REMOVE_STORED = _require5.TLPT_SESSINS_REMOVE_STORED;
	
	/**
	* Due to current limitations of the backend API, the filtering logic for the Archived list of Session
	* works as follows:
	*  1) each time a new date range is set, all previously retrieved inactive sessions get deleted.
	*  2) hasMore flag will be determine after a consequent fetch request with new date range values.
	*/
	
	var actions = {
	
	  fetch: function fetch() {
	    var _reactor$evaluate = reactor.evaluate(filter);
	
	    var end = _reactor$evaluate.end;
	
	    _fetch(end);
	  },
	
	  fetchMore: function fetchMore() {
	    var _reactor$evaluate2 = reactor.evaluate(filter);
	
	    var status = _reactor$evaluate2.status;
	    var end = _reactor$evaluate2.end;
	
	    if (status.hasMore === true && !status.isLoading) {
	      _fetch(end, status.sid);
	    }
	  },
	
	  removeStoredSessions: function removeStoredSessions() {
	    reactor.dispatch(TLPT_SESSINS_REMOVE_STORED);
	  },
	
	  setTimeRange: function setTimeRange(start, end) {
	    reactor.batch(function () {
	      reactor.dispatch(TLPT_STORED_SESSINS_FILTER_SET_RANGE, { start: start, end: end, hasMore: false });
	      reactor.dispatch(TLPT_SESSINS_REMOVE_STORED);
	      _fetch(end);
	    });
	  }
	};
	
	function _fetch(end, sid) {
	  var status = {
	    hasMore: false,
	    isLoading: true
	  };
	
	  reactor.dispatch(TLPT_STORED_SESSINS_FILTER_SET_STATUS, status);
	
	  var start = end || new Date();
	  var params = {
	    order: -1,
	    limit: maxSessionLoadSize,
	    start: start,
	    sid: sid
	  };
	
	  return apiUtils.filterSessions(params).done(function (json) {
	    var sessions = json.sessions;
	
	    var _reactor$evaluate3 = reactor.evaluate(filter);
	
	    var start = _reactor$evaluate3.start;
	
	    status.hasMore = false;
	    status.isLoading = false;
	
	    if (sessions.length === maxSessionLoadSize) {
	      var _sessions = sessions[sessions.length - 1];
	      var id = _sessions.id;
	      var created = _sessions.created;
	
	      status.sid = id;
	      status.hasMore = moment(start).isBefore(created);
	
	      /**
	      * remove at least 1 item before storing the sessions, this way we ensure that
	      * there will be always at least one item on the next 'fetchMore' request.
	      */
	      sessions = sessions.slice(0, maxSessionLoadSize - 1);
	    }
	
	    reactor.batch(function () {
	      reactor.dispatch(TLPT_SESSINS_RECEIVE, sessions);
	      reactor.dispatch(TLPT_STORED_SESSINS_FILTER_SET_STATUS, status);
	    });
	  }).fail(function (err) {
	    showError('Unable to retrieve list of sessions');
	    logger.error('fetching filtered set of sessions', err);
	  });
	}
	
	exports['default'] = actions;
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "actions.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 348 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	'use strict';
	
	module.exports.getters = __webpack_require__(72);
	module.exports.actions = __webpack_require__(347);
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "index.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 349 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(12);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var moment = __webpack_require__(1);
	
	var _require2 = __webpack_require__(233);
	
	var TLPT_STORED_SESSINS_FILTER_SET_RANGE = _require2.TLPT_STORED_SESSINS_FILTER_SET_RANGE;
	var TLPT_STORED_SESSINS_FILTER_SET_STATUS = _require2.TLPT_STORED_SESSINS_FILTER_SET_STATUS;
	exports['default'] = Store({
	  getInitialState: function getInitialState() {
	
	    var end = moment(new Date()).endOf('day').toDate();
	    var start = moment(end).subtract(3, 'day').startOf('day').toDate();
	    var state = {
	      start: start,
	      end: end,
	      status: {
	        isLoading: false,
	        hasMore: false
	      }
	    };
	
	    return toImmutable(state);
	  },
	
	  initialize: function initialize() {
	    this.on(TLPT_STORED_SESSINS_FILTER_SET_RANGE, setRange);
	    this.on(TLPT_STORED_SESSINS_FILTER_SET_STATUS, setStatus);
	  }
	});
	
	function setStatus(state, status) {
	  return state.mergeIn(['status'], status);
	}
	
	function setRange(state, newState) {
	  return state.merge(newState);
	}
	module.exports = exports['default'];

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "storedSessionFilterStore.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 350 */
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	/*
	Copyright 2015 Gravitational, Inc.
	
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
	
	    http://www.apache.org/licenses/LICENSE-2.0
	
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/
	
	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(12);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(73);
	
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

	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "userInviteStore.js" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },
/* 351 */,
/* 352 */,
/* 353 */,
/* 354 */,
/* 355 */,
/* 356 */,
/* 357 */,
/* 358 */,
/* 359 */,
/* 360 */,
/* 361 */,
/* 362 */,
/* 363 */,
/* 364 */,
/* 365 */,
/* 366 */,
/* 367 */,
/* 368 */,
/* 369 */,
/* 370 */,
/* 371 */,
/* 372 */,
/* 373 */,
/* 374 */,
/* 375 */,
/* 376 */,
/* 377 */,
/* 378 */,
/* 379 */,
/* 380 */,
/* 381 */,
/* 382 */,
/* 383 */,
/* 384 */,
/* 385 */,
/* 386 */
/***/ function(module, exports, __webpack_require__) {

	/**
	 * Copyright 2013-2015, Facebook, Inc.
	 * All rights reserved.
	 *
	 * This source code is licensed under the BSD-style license found in the
	 * LICENSE file in the root directory of this source tree. An additional grant
	 * of patent rights can be found in the PATENTS file in the same directory.
	 *
	 * @typechecks
	 * @providesModule ReactCSSTransitionGroup
	 */
	
	'use strict';
	
	var React = __webpack_require__(40);
	
	var assign = __webpack_require__(4);
	
	var ReactTransitionGroup = __webpack_require__(272);
	var ReactCSSTransitionGroupChild = __webpack_require__(387);
	
	function createTransitionTimeoutPropValidator(transitionType) {
	  var timeoutPropName = 'transition' + transitionType + 'Timeout';
	  var enabledPropName = 'transition' + transitionType;
	
	  return function (props) {
	    // If the transition is enabled
	    if (props[enabledPropName]) {
	      // If no timeout duration is provided
	      if (props[timeoutPropName] == null) {
	        return new Error(timeoutPropName + ' wasn\'t supplied to ReactCSSTransitionGroup: ' + 'this can cause unreliable animations and won\'t be supported in ' + 'a future version of React. See ' + 'https://fb.me/react-animation-transition-group-timeout for more ' + 'information.');
	
	        // If the duration isn't a number
	      } else if (typeof props[timeoutPropName] !== 'number') {
	          return new Error(timeoutPropName + ' must be a number (in milliseconds)');
	        }
	    }
	  };
	}
	
	var ReactCSSTransitionGroup = React.createClass({
	  displayName: 'ReactCSSTransitionGroup',
	
	  propTypes: {
	    transitionName: ReactCSSTransitionGroupChild.propTypes.name,
	
	    transitionAppear: React.PropTypes.bool,
	    transitionEnter: React.PropTypes.bool,
	    transitionLeave: React.PropTypes.bool,
	    transitionAppearTimeout: createTransitionTimeoutPropValidator('Appear'),
	    transitionEnterTimeout: createTransitionTimeoutPropValidator('Enter'),
	    transitionLeaveTimeout: createTransitionTimeoutPropValidator('Leave')
	  },
	
	  getDefaultProps: function () {
	    return {
	      transitionAppear: false,
	      transitionEnter: true,
	      transitionLeave: true
	    };
	  },
	
	  _wrapChild: function (child) {
	    // We need to provide this childFactory so that
	    // ReactCSSTransitionGroupChild can receive updates to name, enter, and
	    // leave while it is leaving.
	    return React.createElement(ReactCSSTransitionGroupChild, {
	      name: this.props.transitionName,
	      appear: this.props.transitionAppear,
	      enter: this.props.transitionEnter,
	      leave: this.props.transitionLeave,
	      appearTimeout: this.props.transitionAppearTimeout,
	      enterTimeout: this.props.transitionEnterTimeout,
	      leaveTimeout: this.props.transitionLeaveTimeout
	    }, child);
	  },
	
	  render: function () {
	    return React.createElement(ReactTransitionGroup, assign({}, this.props, { childFactory: this._wrapChild }));
	  }
	});
	
	module.exports = ReactCSSTransitionGroup;

/***/ },
/* 387 */
/***/ function(module, exports, __webpack_require__) {

	/**
	 * Copyright 2013-2015, Facebook, Inc.
	 * All rights reserved.
	 *
	 * This source code is licensed under the BSD-style license found in the
	 * LICENSE file in the root directory of this source tree. An additional grant
	 * of patent rights can be found in the PATENTS file in the same directory.
	 *
	 * @typechecks
	 * @providesModule ReactCSSTransitionGroupChild
	 */
	
	'use strict';
	
	var React = __webpack_require__(40);
	var ReactDOM = __webpack_require__(53);
	
	var CSSCore = __webpack_require__(286);
	var ReactTransitionEvents = __webpack_require__(271);
	
	var onlyChild = __webpack_require__(279);
	
	// We don't remove the element from the DOM until we receive an animationend or
	// transitionend event. If the user screws up and forgets to add an animation
	// their node will be stuck in the DOM forever, so we detect if an animation
	// does not start and if it doesn't, we just call the end listener immediately.
	var TICK = 17;
	
	var ReactCSSTransitionGroupChild = React.createClass({
	  displayName: 'ReactCSSTransitionGroupChild',
	
	  propTypes: {
	    name: React.PropTypes.oneOfType([React.PropTypes.string, React.PropTypes.shape({
	      enter: React.PropTypes.string,
	      leave: React.PropTypes.string,
	      active: React.PropTypes.string
	    }), React.PropTypes.shape({
	      enter: React.PropTypes.string,
	      enterActive: React.PropTypes.string,
	      leave: React.PropTypes.string,
	      leaveActive: React.PropTypes.string,
	      appear: React.PropTypes.string,
	      appearActive: React.PropTypes.string
	    })]).isRequired,
	
	    // Once we require timeouts to be specified, we can remove the
	    // boolean flags (appear etc.) and just accept a number
	    // or a bool for the timeout flags (appearTimeout etc.)
	    appear: React.PropTypes.bool,
	    enter: React.PropTypes.bool,
	    leave: React.PropTypes.bool,
	    appearTimeout: React.PropTypes.number,
	    enterTimeout: React.PropTypes.number,
	    leaveTimeout: React.PropTypes.number
	  },
	
	  transition: function (animationType, finishCallback, userSpecifiedDelay) {
	    var node = ReactDOM.findDOMNode(this);
	
	    if (!node) {
	      if (finishCallback) {
	        finishCallback();
	      }
	      return;
	    }
	
	    var className = this.props.name[animationType] || this.props.name + '-' + animationType;
	    var activeClassName = this.props.name[animationType + 'Active'] || className + '-active';
	    var timeout = null;
	
	    var endListener = function (e) {
	      if (e && e.target !== node) {
	        return;
	      }
	
	      clearTimeout(timeout);
	
	      CSSCore.removeClass(node, className);
	      CSSCore.removeClass(node, activeClassName);
	
	      ReactTransitionEvents.removeEndEventListener(node, endListener);
	
	      // Usually this optional callback is used for informing an owner of
	      // a leave animation and telling it to remove the child.
	      if (finishCallback) {
	        finishCallback();
	      }
	    };
	
	    CSSCore.addClass(node, className);
	
	    // Need to do this to actually trigger a transition.
	    this.queueClass(activeClassName);
	
	    // If the user specified a timeout delay.
	    if (userSpecifiedDelay) {
	      // Clean-up the animation after the specified delay
	      timeout = setTimeout(endListener, userSpecifiedDelay);
	      this.transitionTimeouts.push(timeout);
	    } else {
	      // DEPRECATED: this listener will be removed in a future version of react
	      ReactTransitionEvents.addEndEventListener(node, endListener);
	    }
	  },
	
	  queueClass: function (className) {
	    this.classNameQueue.push(className);
	
	    if (!this.timeout) {
	      this.timeout = setTimeout(this.flushClassNameQueue, TICK);
	    }
	  },
	
	  flushClassNameQueue: function () {
	    if (this.isMounted()) {
	      this.classNameQueue.forEach(CSSCore.addClass.bind(CSSCore, ReactDOM.findDOMNode(this)));
	    }
	    this.classNameQueue.length = 0;
	    this.timeout = null;
	  },
	
	  componentWillMount: function () {
	    this.classNameQueue = [];
	    this.transitionTimeouts = [];
	  },
	
	  componentWillUnmount: function () {
	    if (this.timeout) {
	      clearTimeout(this.timeout);
	    }
	    this.transitionTimeouts.forEach(function (timeout) {
	      clearTimeout(timeout);
	    });
	  },
	
	  componentWillAppear: function (done) {
	    if (this.props.appear) {
	      this.transition('appear', done, this.props.appearTimeout);
	    } else {
	      done();
	    }
	  },
	
	  componentWillEnter: function (done) {
	    if (this.props.enter) {
	      this.transition('enter', done, this.props.enterTimeout);
	    } else {
	      done();
	    }
	  },
	
	  componentWillLeave: function (done) {
	    if (this.props.leave) {
	      this.transition('leave', done, this.props.leaveTimeout);
	    } else {
	      done();
	    }
	  },
	
	  render: function () {
	    return onlyChild(this.props.children);
	  }
	});
	
	module.exports = ReactCSSTransitionGroupChild;

/***/ },
/* 388 */,
/* 389 */,
/* 390 */,
/* 391 */,
/* 392 */,
/* 393 */,
/* 394 */,
/* 395 */,
/* 396 */,
/* 397 */,
/* 398 */,
/* 399 */,
/* 400 */,
/* 401 */,
/* 402 */,
/* 403 */,
/* 404 */,
/* 405 */,
/* 406 */,
/* 407 */,
/* 408 */,
/* 409 */,
/* 410 */,
/* 411 */,
/* 412 */,
/* 413 */,
/* 414 */,
/* 415 */,
/* 416 */,
/* 417 */,
/* 418 */,
/* 419 */,
/* 420 */,
/* 421 */,
/* 422 */,
/* 423 */,
/* 424 */,
/* 425 */,
/* 426 */,
/* 427 */,
/* 428 */,
/* 429 */,
/* 430 */,
/* 431 */,
/* 432 */,
/* 433 */,
/* 434 */
/***/ function(module, exports) {

	(function(host) {
	
	  var properties = {
	    browser: [
	      [/msie ([\.\_\d]+)/, "ie"],
	      [/trident\/.*?rv:([\.\_\d]+)/, "ie"],
	      [/firefox\/([\.\_\d]+)/, "firefox"],
	      [/chrome\/([\.\_\d]+)/, "chrome"],
	      [/version\/([\.\_\d]+).*?safari/, "safari"],
	      [/mobile safari ([\.\_\d]+)/, "safari"],
	      [/android.*?version\/([\.\_\d]+).*?safari/, "com.android.browser"],
	      [/crios\/([\.\_\d]+).*?safari/, "chrome"],
	      [/opera/, "opera"],
	      [/opera\/([\.\_\d]+)/, "opera"],
	      [/opera ([\.\_\d]+)/, "opera"],
	      [/opera mini.*?version\/([\.\_\d]+)/, "opera.mini"],
	      [/opios\/([a-z\.\_\d]+)/, "opera"],
	      [/blackberry/, "blackberry"],
	      [/blackberry.*?version\/([\.\_\d]+)/, "blackberry"],
	      [/bb\d+.*?version\/([\.\_\d]+)/, "blackberry"],
	      [/rim.*?version\/([\.\_\d]+)/, "blackberry"],
	      [/iceweasel\/([\.\_\d]+)/, "iceweasel"],
	      [/edge\/([\.\d]+)/, "edge"]
	    ],
	    os: [
	      [/linux ()([a-z\.\_\d]+)/, "linux"],
	      [/mac os x/, "macos"],
	      [/mac os x.*?([\.\_\d]+)/, "macos"],
	      [/os ([\.\_\d]+) like mac os/, "ios"],
	      [/openbsd ()([a-z\.\_\d]+)/, "openbsd"],
	      [/android/, "android"],
	      [/android ([a-z\.\_\d]+);/, "android"],
	      [/mozilla\/[a-z\.\_\d]+ \((?:mobile)|(?:tablet)/, "firefoxos"],
	      [/windows\s*(?:nt)?\s*([\.\_\d]+)/, "windows"],
	      [/windows phone.*?([\.\_\d]+)/, "windows.phone"],
	      [/windows mobile/, "windows.mobile"],
	      [/blackberry/, "blackberryos"],
	      [/bb\d+/, "blackberryos"],
	      [/rim.*?os\s*([\.\_\d]+)/, "blackberryos"]
	    ],
	    device: [
	      [/ipad/, "ipad"],
	      [/iphone/, "iphone"],
	      [/lumia/, "lumia"],
	      [/htc/, "htc"],
	      [/nexus/, "nexus"],
	      [/galaxy nexus/, "galaxy.nexus"],
	      [/nokia/, "nokia"],
	      [/ gt\-/, "galaxy"],
	      [/ sm\-/, "galaxy"],
	      [/xbox/, "xbox"],
	      [/(?:bb\d+)|(?:blackberry)|(?: rim )/, "blackberry"]
	    ]
	  };
	
	  var UNKNOWN = "Unknown";
	
	  var propertyNames = Object.keys(properties);
	
	  function Sniffr() {
	    var self = this;
	
	    propertyNames.forEach(function(propertyName) {
	      self[propertyName] = {
	        name: UNKNOWN,
	        version: [],
	        versionString: UNKNOWN
	      };
	    });
	  }
	
	  function determineProperty(self, propertyName, userAgent) {
	    properties[propertyName].forEach(function(propertyMatcher) {
	      var propertyRegex = propertyMatcher[0];
	      var propertyValue = propertyMatcher[1];
	
	      var match = userAgent.match(propertyRegex);
	
	      if (match) {
	        self[propertyName].name = propertyValue;
	
	        if (match[2]) {
	          self[propertyName].versionString = match[2];
	          self[propertyName].version = [];
	        } else if (match[1]) {
	          self[propertyName].versionString = match[1].replace(/_/g, ".");
	          self[propertyName].version = parseVersion(match[1]);
	        } else {
	          self[propertyName].versionString = UNKNOWN;
	          self[propertyName].version = [];
	        }
	      }
	    });
	  }
	
	  function parseVersion(versionString) {
	    return versionString.split(/[\._]/).map(function(versionPart) {
	      return parseInt(versionPart);
	    });
	  }
	
	  Sniffr.prototype.sniff = function(userAgentString) {
	    var self = this;
	    var userAgent = (userAgentString || navigator.userAgent || "").toLowerCase();
	
	    propertyNames.forEach(function(propertyName) {
	      determineProperty(self, propertyName, userAgent);
	    });
	  };
	
	
	  if (typeof module !== 'undefined' && module.exports) {
	    module.exports = Sniffr;
	  } else {
	    host.Sniffr = new Sniffr();
	    host.Sniffr.sniff(navigator.userAgent);
	  }
	})(this);


/***/ },
/* 435 */,
/* 436 */,
/* 437 */,
/* 438 */,
/* 439 */
/***/ function(module, exports, __webpack_require__) {

	;
	var sprite = __webpack_require__(440);;
	var image = "<symbol viewBox=\"0 0 340 100\" id=\"grv-tlpt-logo-full\" xmlns:xlink=\"http://www.w3.org/1999/xlink\"> <g> <g id=\"grv-tlpt-logo-full_Layer_2\"> <g> <g> <path d=\"m47.671001,21.444c-7.396,0 -14.102001,3.007999 -18.960003,7.866001c-4.856998,4.856998 -7.865999,11.563 -7.865999,18.959999c0,7.396 3.008001,14.101002 7.865999,18.957996s11.564003,7.865005 18.960003,7.865005s14.102001,-3.008003 18.958996,-7.865005s7.865005,-11.561996 7.865005,-18.957996s-3.008003,-14.104 -7.865005,-18.959999c-4.857994,-4.858002 -11.562996,-7.866001 -18.958996,-7.866001zm11.386997,19.509998h-8.213997v23.180004h-6.344002v-23.180004h-8.215v-5.612h22.772999v5.612l0,0z\"/> </g> <g> <path d=\"m92.782997,63.357002c-0.098999,-0.371002 -0.320999,-0.709 -0.646996,-0.942001l-4.562004,-3.958l-4.561996,-3.957001c0.163002,-0.887001 0.267998,-1.805 0.331001,-2.736c0.063995,-0.931 0.086998,-1.874001 0.086998,-2.805c0,-0.932999 -0.022003,-1.875 -0.086998,-2.806999c-0.063004,-0.931999 -0.167999,-1.851002 -0.331001,-2.736l4.561996,-3.957001l4.562004,-3.958c0.325996,-0.232998 0.548996,-0.57 0.646996,-0.942001c0.099007,-0.372997 0.075005,-0.778999 -0.087997,-1.153c-0.931999,-2.862 -2.199997,-5.655998 -3.731003,-8.299c-1.530998,-2.641998 -3.321999,-5.132998 -5.301994,-7.390999c-0.278999,-0.326 -0.617004,-0.548 -0.978004,-0.646c-0.360001,-0.098999 -0.744995,-0.074999 -1.116997,0.087l-5.750999,2.002001l-5.749001,2.000999c-1.419998,-1.164 -2.933998,-2.211 -4.522003,-3.136999c-1.589996,-0.925001 -3.253998,-1.728001 -4.977997,-2.404001l-1.139999,-5.959l-1.140999,-5.959c-0.069,-0.373 -0.268005,-0.733 -0.547005,-1.013c-0.278999,-0.28 -0.640999,-0.478 -1.036995,-0.524c-2.980003,-0.605 -6.007004,-0.908 -9.033005,-0.908s-6.052998,0.302 -9.032997,0.908c-0.396,0.046 -0.756001,0.245001 -1.036003,0.524c-0.278999,0.279 -0.477997,0.64 -0.546997,1.013l-1.141003,5.959l-1.140999,5.960001c-1.723,0.675999 -3.410999,1.479 -5.012001,2.403999c-1.599998,0.924999 -3.112999,1.973 -4.487,3.136999l-5.75,-2.000999l-5.75,-2.001999c-0.372,-0.164001 -0.755999,-0.187 -1.116999,-0.088001c-0.361,0.1 -0.699001,0.32 -0.978001,0.646c-1.979,2.259001 -3.771,4.75 -5.302,7.392002c-1.53,2.641998 -2.799,5.436996 -3.73,8.299c-0.163,0.372997 -0.187,0.780998 -0.087001,1.151997c0.099,0.372002 0.320001,0.710003 0.646001,0.943001l4.563,3.957001l4.562,3.958c-0.163,0.884998 -0.268,1.804001 -0.331001,2.735001c-0.063999,0.931999 -0.087999,1.875 -0.087999,2.806s0.023001,1.875 0.087,2.806c0.064001,0.931999 0.168001,1.851002 0.332001,2.735001l-4.562,3.957001l-4.562,3.959c-0.325,0.231003 -0.547,0.569 -0.646,0.942001c-0.099,0.370995 -0.076,0.778999 0.087,1.150002c0.931,2.864998 2.2,5.657997 3.73,8.300995c1.531,2.642998 3.323,5.133003 5.302,7.391998c0.280001,0.325005 0.618,0.548004 0.978001,0.646004c0.361,0.099998 0.744999,0.074997 1.118,-0.087997l5.75,-2.003006l5.749998,-2.000999c1.373001,1.164001 2.886002,2.213005 4.487003,3.139c1.600998,0.924004 3.288998,1.728004 5.010998,2.401001l1.140999,5.961998l1.141003,5.959c0.07,0.372002 0.267998,0.733002 0.547001,1.014c0.278999,0.279007 0.640999,0.479004 1.035999,0.522003c1.489998,0.278 2.979,0.500999 4.480999,0.651001c1.500999,0.152 3.014999,0.232002 4.551998,0.232002s3.049004,-0.080002 4.551003,-0.232002c1.501999,-0.150002 2.990997,-0.373001 4.479996,-0.651001c0.396004,-0.044998 0.757004,-0.243996 1.037003,-0.522003c0.279999,-0.278999 0.476997,-0.641998 0.547005,-1.014l1.140999,-5.959l1.140999,-5.961998c1.723,-0.674995 3.387001,-1.477997 4.976997,-2.401001c1.588005,-0.925995 3.103004,-1.974998 4.522003,-3.139l5.75,2.000999l5.75,2.003006c0.373001,0.162994 0.756996,0.185997 1.117996,0.087997c0.360001,-0.098999 0.698006,-0.32 0.978004,-0.646004c1.978996,-2.258995 3.770996,-4.749001 5.301994,-7.391998c1.531006,-2.642998 2.800003,-5.436996 3.731003,-8.300995c0.164001,-0.368004 0.188004,-0.778008 0.087997,-1.150002zm-24.237999,5.787994c-5.348,5.349007 -12.731995,8.660004 -20.875,8.660004c-8.143997,0 -15.526997,-3.312004 -20.875,-8.660004s-8.659998,-12.730995 -8.659998,-20.874996c0,-8.144001 3.312,-15.527 8.661001,-20.875999c5.348,-5.348001 12.731998,-8.661001 20.875999,-8.661001c8.143002,0 15.525997,3.312 20.874996,8.661001c5.348,5.348999 8.661003,12.731998 8.661003,20.875999c-0.000999,8.141998 -3.314003,15.525997 -8.663002,20.874996z\"/> </g> </g> </g> <g> <path d=\"m119.773003,30.861h-13.020004v-6.841h33.599998v6.841h-13.020004v35.639999h-7.55999v-35.639999l0,0z\"/> <path d=\"m143.953003,54.620998c0.23999,2.16 1.080002,3.84 2.520004,5.039997s3.179993,1.800003 5.219986,1.800003c1.800003,0 3.309006,-0.368996 4.530014,-1.110001c1.219986,-0.738998 2.289993,-1.668999 3.209991,-2.790001l5.160004,3.900002c-1.680008,2.080002 -3.561005,3.561005 -5.639999,4.440002c-2.080002,0.878998 -4.26001,1.319 -6.540009,1.319c-2.159988,0 -4.199997,-0.359001 -6.119995,-1.080002c-1.919998,-0.720001 -3.580994,-1.738998 -4.979996,-3.059998c-1.401001,-1.320007 -2.511002,-2.910004 -3.330002,-4.771004c-0.820007,-1.858997 -1.229996,-3.929996 -1.229996,-6.209999c0,-2.278999 0.409988,-4.349998 1.229996,-6.209999c0.819,-1.859001 1.929001,-3.449001 3.330002,-4.77c1.399002,-1.32 3.059998,-2.34 4.979996,-3.061001c1.919998,-0.719997 3.960007,-1.078999 6.119995,-1.078999c2,0 3.830002,0.351002 5.490005,1.049999c1.658997,0.700001 3.080002,1.709999 4.259995,3.028999c1.180008,1.32 2.100006,2.951 2.76001,4.891003c0.659988,1.939999 0.98999,4.169998 0.98999,6.688999v1.98h-21.959991l0,0.002998zm14.759995,-5.399998c-0.041,-2.118999 -0.699997,-3.789001 -1.979996,-5.010002c-1.281006,-1.219997 -3.059998,-1.829998 -5.339996,-1.829998c-2.160004,0 -3.87001,0.620998 -5.130005,1.860001c-1.259995,1.239998 -2.031006,2.899998 -2.309998,4.979h14.759995l0,0.000999z\"/> <path d=\"m172.753006,21.141001h7.199997v45.359999h-7.199997v-45.359999l0,0z\"/> <path d=\"m193.992004,54.620998c0.23999,2.16 1.080002,3.84 2.519989,5.039997c1.440002,1.200005 3.181,1.800003 5.221008,1.800003c1.800003,0 3.309006,-0.368996 4.528992,-1.110001c1.221008,-0.738998 2.290009,-1.668999 3.211014,-2.790001l5.159988,3.900002c-1.681,2.080002 -3.560989,3.561005 -5.640991,4.440002c-2.080002,0.878998 -4.26001,1.319 -6.540009,1.319c-2.158997,0 -4.199997,-0.359001 -6.119995,-1.080002c-1.919998,-0.720001 -3.580002,-1.738998 -4.979004,-3.059998c-1.401001,-1.320007 -2.511002,-2.910004 -3.330002,-4.771004c-0.819992,-1.858997 -1.228989,-3.929996 -1.228989,-6.209999c0,-2.278999 0.408997,-4.349998 1.228989,-6.209999c0.819,-1.859001 1.929001,-3.449001 3.330002,-4.77c1.399002,-1.32 3.059998,-2.34 4.979004,-3.061001c1.919998,-0.719997 3.960999,-1.078999 6.119995,-1.078999c2,0 3.830002,0.351002 5.490005,1.049999c1.658997,0.700001 3.078995,1.709999 4.259995,3.028999c1.180008,1.32 2.100998,2.951 2.761002,4.891003c0.660004,1.939999 0.988998,4.169998 0.988998,6.688999v1.98h-21.959991l0,0.002998zm14.759995,-5.399998c-0.039993,-2.118999 -0.699005,-3.789001 -1.979004,-5.010002c-1.279999,-1.219997 -3.059998,-1.829998 -5.340988,-1.829998c-2.159012,0 -3.869003,0.620998 -5.129013,1.860001c-1.259995,1.239998 -2.030991,2.899998 -2.310989,4.979h14.759995l0,0.000999z\"/> <path d=\"m222.671997,37.701h6.839996v4.319h0.12001c1.039993,-1.758999 2.438995,-3.039001 4.199997,-3.84c1.759995,-0.799999 3.660004,-1.199001 5.699005,-1.199001c2.19899,0 4.179993,0.389999 5.939987,1.170002c1.76001,0.778999 3.260025,1.850998 4.500015,3.209999c1.239014,1.360001 2.179993,2.959999 2.820007,4.799999c0.639984,1.84 0.959991,3.82 0.959991,5.938999c0,2.121002 -0.339996,4.101002 -1.019989,5.940002c-0.682007,1.840004 -1.631012,3.440002 -2.851013,4.800003c-1.221008,1.359993 -2.690002,2.43 -4.410004,3.209999s-3.600998,1.169998 -5.639999,1.169998c-1.360001,0 -2.561005,-0.140999 -3.600006,-0.420006c-1.041,-0.279991 -1.960999,-0.639992 -2.761002,-1.079994c-0.799988,-0.439003 -1.478989,-0.909004 -2.039993,-1.410004c-0.561005,-0.499001 -1.020004,-0.988998 -1.380005,-1.469994h-0.181v17.339996h-7.19899v-42.479l0.002991,0zm23.880005,14.400002c0,-1.119003 -0.190002,-2.199001 -0.569,-3.239002c-0.380997,-1.040001 -0.940994,-1.959999 -1.681,-2.760998c-0.740997,-0.799004 -1.630005,-1.439003 -2.669998,-1.920002c-1.040009,-0.479 -2.220001,-0.720001 -3.540009,-0.720001s-2.5,0.240002 -3.539993,0.720001c-1.040009,0.48 -1.931,1.120998 -2.669998,1.920002c-0.740997,0.800999 -1.300003,1.720997 -1.681,2.760998c-0.380005,1.040001 -0.569,2.119999 -0.569,3.239002c0,1.120998 0.188995,2.200996 0.569,3.239998c0.380997,1.041 0.938995,1.960995 1.681,2.759998c0.738998,0.801003 1.62999,1.440002 2.669998,1.919998c1.039993,0.480003 2.220001,0.721001 3.539993,0.721001s2.5,-0.239998 3.540009,-0.721001c1.039993,-0.478996 1.929001,-1.118996 2.669998,-1.919998c0.738998,-0.799004 1.300003,-1.718998 1.681,-2.759998c0.377991,-1.039001 0.569,-2.118999 0.569,-3.239998z\"/> <path d=\"m259.031006,52.101002c0,-2.279003 0.410004,-4.350002 1.230011,-6.210003c0.817993,-1.858997 1.928986,-3.448997 3.329987,-4.77c1.39801,-1.32 3.059021,-2.34 4.979004,-3.060997c1.920013,-0.720001 3.959991,-1.079002 6.119995,-1.079002s4.199005,0.359001 6.119019,1.079002c1.919983,0.720997 3.579987,1.739998 4.97998,3.060997s2.51001,2.91 3.330017,4.77c0.819977,1.860001 1.22998,3.931 1.22998,6.210003c0,2.279999 -0.410004,4.350998 -1.22998,6.210003c-0.820007,1.860001 -1.930023,3.449997 -3.330017,4.770996s-3.061005,2.340004 -4.97998,3.059998c-1.920013,0.721001 -3.959015,1.080002 -6.119019,1.080002s-4.199982,-0.359001 -6.119995,-1.080002c-1.92099,-0.719994 -3.580994,-1.738998 -4.979004,-3.059998c-1.401001,-1.32 -2.511993,-2.909996 -3.329987,-4.770996c-0.820007,-1.860004 -1.230011,-3.930004 -1.230011,-6.210003zm7.199005,0c0,1.120998 0.188995,2.200996 0.570007,3.239998c0.380005,1.041 0.938995,1.960995 1.679993,2.759998c0.73999,0.801003 1.630005,1.440002 2.670013,1.919998c1.040985,0.480003 2.220978,0.721001 3.540985,0.721001s2.498993,-0.239998 3.539001,-0.721001c1.040985,-0.478996 1.929993,-1.118996 2.670013,-1.919998c0.73999,-0.799004 1.300995,-1.718998 1.681976,-2.759998c0.378998,-1.039001 0.568024,-2.118999 0.568024,-3.239998c0,-1.119003 -0.189026,-2.199001 -0.568024,-3.239002c-0.380981,-1.040001 -0.940979,-1.959999 -1.681976,-2.760998c-0.740021,-0.799004 -1.629028,-1.439003 -2.670013,-1.920002c-1.040009,-0.479 -2.218994,-0.720001 -3.539001,-0.720001s-2.5,0.240002 -3.540985,0.720001c-1.040009,0.48 -1.930023,1.120998 -2.670013,1.920002c-0.73999,0.800999 -1.299988,1.720997 -1.679993,2.760998c-0.380005,1.039001 -0.570007,2.118999 -0.570007,3.239002z\"/> <path d=\"m297.070007,37.701h7.200989v4.560001h0.119019c0.798981,-1.68 1.938995,-2.979 3.419983,-3.899002s3.179993,-1.380001 5.100006,-1.380001c0.438995,0 0.871002,0.040001 1.290985,0.119003c0.420013,0.080997 0.850006,0.181 1.289001,0.300999v6.959999c-0.599976,-0.16 -1.188995,-0.290001 -1.769989,-0.390999c-0.579987,-0.098999 -1.149994,-0.149002 -1.710999,-0.149002c-1.679993,0 -3.028992,0.310001 -4.049011,0.93c-1.019989,0.621002 -1.800995,1.330002 -2.339996,2.130001c-0.540985,0.800999 -0.899994,1.601002 -1.079987,2.400002c-0.180023,0.800999 -0.27002,1.399998 -0.27002,1.799999v15.419998h-7.200989v-28.800999l0.001007,0z\"/> <path d=\"m317.049011,43.820999v-6.119999h5.940979v-8.34h7.199005v8.34h7.920013v6.119999h-7.920013v12.600002c0,1.439999 0.27002,2.579998 0.811005,3.420002c0.539001,0.839996 1.609009,1.259995 3.209015,1.259995c0.640991,0 1.339996,-0.069 2.10199,-0.209999c0.757996,-0.139999 1.359009,-0.369003 1.798981,-0.689003v6.060005c-0.759979,0.360001 -1.688995,0.608994 -2.788971,0.75c-1.10202,0.139999 -2.070007,0.209999 -2.910004,0.209999c-1.920013,0 -3.490021,-0.209999 -4.710999,-0.630005s-2.180023,-1.059998 -2.878998,-1.919998c-0.701019,-0.859001 -1.182007,-1.93 -1.44101,-3.209991c-0.26001,-1.279007 -0.389008,-2.76001 -0.389008,-4.440002v-13.201004h-5.941986l0,0z\"/> </g> <g> <path d=\"m119.194,86.295998h3.587997c0.346001,0 0.689003,0.041 1.027,0.124001c0.338005,0.082001 0.639,0.217003 0.903,0.402c0.264,0.187004 0.479004,0.427002 0.644005,0.722s0.246994,0.650002 0.246994,1.066002c0,0.519997 -0.146996,0.947998 -0.441994,1.287003c-0.295006,0.337997 -0.681,0.579994 -1.157005,0.727997v0.026001c0.286003,0.033997 0.553001,0.113998 0.800003,0.239998c0.247002,0.125999 0.457001,0.286003 0.629997,0.480003c0.173004,0.195 0.310005,0.420998 0.409004,0.676994s0.149994,0.530006 0.149994,0.825005c0,0.502998 -0.099998,0.920998 -0.298996,1.254997c-0.198997,0.333 -0.460999,0.603004 -0.786003,0.806c-0.324997,0.204002 -0.697998,0.348999 -1.117996,0.436005s-0.848,0.129997 -1.280998,0.129997h-3.315002v-9.204002l0,0zm1.638,3.744003h1.495003c0.545998,0 0.955994,-0.106003 1.228996,-0.318001c0.273003,-0.212997 0.408997,-0.491997 0.408997,-0.838997c0,-0.398003 -0.140999,-0.695 -0.421997,-0.891006c-0.281998,-0.194 -0.734001,-0.292 -1.358002,-0.292h-1.351997v2.340004l-0.000999,0zm0,4.056h1.507996c0.208,0 0.431007,-0.013 0.669006,-0.039001c0.237999,-0.025002 0.457001,-0.085999 0.656998,-0.181999c0.198997,-0.096001 0.363998,-0.231003 0.494003,-0.408997c0.129997,-0.178001 0.195,-0.418007 0.195,-0.722c0,-0.485001 -0.158005,-0.823006 -0.475006,-1.014c-0.315994,-0.191002 -0.807999,-0.286003 -1.475998,-0.286003h-1.572998v2.652l0.000999,0z\"/> <path d=\"m130.854996,91.560997l-3.457993,-5.264999h2.054001l2.261993,3.666l2.28801,-3.666h1.949997l-3.458008,5.264999v3.939003h-1.638v-3.939003l0,0z\"/> <path d=\"m150.796997,94.823997c-1.136002,0.606003 -2.404999,0.910004 -3.80899,0.910004c-0.711014,0 -1.363007,-0.114998 -1.957001,-0.345001s-1.105011,-0.555 -1.534012,-0.975998c-0.429001,-0.420006 -0.764999,-0.925003 -1.006989,-1.514c-0.243011,-0.590004 -0.363998,-1.244003 -0.363998,-1.964005c0,-0.736 0.120987,-1.404999 0.363998,-2.007996s0.578995,-1.116005 1.006989,-1.541c0.429001,-0.424004 0.940002,-0.750999 1.534012,-0.981003c0.593994,-0.228996 1.245987,-0.345001 1.957001,-0.345001c0.701996,0 1.360001,0.084999 1.975998,0.254005c0.61499,0.168999 1.166,0.471001 1.651001,0.903l-1.209,1.223c-0.295013,-0.286003 -0.652008,-0.508003 -1.072006,-0.663002c-0.421005,-0.155998 -0.865005,-0.234001 -1.332993,-0.234001c-0.477005,0 -0.908005,0.084999 -1.294006,0.253998c-0.384995,0.169006 -0.716995,0.402 -0.994003,0.701004c-0.276993,0.299995 -0.492004,0.648003 -0.643997,1.046997c-0.151993,0.398003 -0.227997,0.828003 -0.227997,1.287003c0,0.493996 0.076004,0.948997 0.227997,1.364998c0.151001,0.416 0.365997,0.775002 0.643997,1.079002c0.277008,0.303001 0.609009,0.541 0.994003,0.714996c0.386002,0.173004 0.817001,0.260002 1.294006,0.260002c0.416,0 0.807999,-0.039001 1.175995,-0.116997c0.367996,-0.078003 0.694992,-0.199005 0.981003,-0.362999v-2.171005h-1.88501v-1.480995h3.52301v4.704994l0.000992,0z\"/> <path d=\"m153.722,86.295998h3.197998c0.442001,0 0.869003,0.041 1.279999,0.124001c0.412003,0.082001 0.778,0.223 1.098999,0.422005c0.320007,0.198997 0.576004,0.467995 0.766998,0.806999c0.190002,0.337997 0.286011,0.766998 0.286011,1.285995c0,0.667999 -0.184998,1.227005 -0.553009,1.678001c-0.369003,0.450005 -0.894989,0.723999 -1.580002,0.818001l2.445007,4.069h-1.975998l-2.132004,-3.900002h-1.195999v3.900002h-1.638v-9.204002l0,0zm2.912003,3.900002c0.233994,0 0.468002,-0.011002 0.701996,-0.032997c0.234009,-0.021004 0.447998,-0.073006 0.643997,-0.154999c0.195007,-0.083 0.352997,-0.208 0.473999,-0.377007c0.122009,-0.168999 0.182007,-0.404999 0.182007,-0.709c0,-0.268997 -0.056,-0.485001 -0.169006,-0.648994c-0.112991,-0.165001 -0.259995,-0.288002 -0.442001,-0.371002c-0.181992,-0.082001 -0.383987,-0.137001 -0.603989,-0.162003c-0.221008,-0.026001 -0.436005,-0.039001 -0.644012,-0.039001h-1.416992v2.496002h1.274002l0,-0.000999z\"/> <path d=\"m165.876007,86.295998h1.416992l3.966003,9.204002h-1.872009l-0.857986,-2.106003h-3.991013l-0.832001,2.106003h-1.832993l4.003006,-9.204002zm2.080994,5.694l-1.417007,-3.743996l-1.442993,3.743996h2.860001l0,0z\"/> <path d=\"m171.401001,86.295998h1.884995l2.509003,6.955002l2.587006,-6.955002h1.76799l-3.716995,9.204002h-1.416992l-3.615005,-9.204002z\"/> <path d=\"m182.087006,86.295998h1.638v9.204002h-1.638v-9.204002l0,0z\"/> <path d=\"m188.613007,87.778h-2.820999v-1.482002h7.279999v1.482002h-2.820999v7.722h-1.638v-7.722l0,0z\"/> <path d=\"m196.959,86.295998h1.417007l3.965988,9.204002h-1.873001l-0.856995,-2.106003h-3.990997l-0.833008,2.106003h-1.832993l4.003998,-9.204002zm2.080002,5.694l-1.417007,-3.743996l-1.442001,3.743996h2.859009l0,0z\"/> <path d=\"m205.044998,87.778h-2.819992v-1.482002h7.278992v1.482002h-2.819992v7.722h-1.639008v-7.722l0,0z\"/> <path d=\"m211.570007,86.295998h1.638992v9.204002h-1.638992v-9.204002l0,0z\"/> <path d=\"m215.718994,90.936996c0,-0.736 0.121002,-1.404999 0.362991,-2.007996s0.578003,-1.115997 1.008011,-1.541c0.429001,-0.424004 0.938995,-0.750999 1.53299,-0.981003c0.594009,-0.228996 1.246002,-0.345001 1.957001,-0.345001c0.719009,-0.007996 1.378006,0.098007 1.977005,0.319c0.597992,0.221001 1.112991,0.544006 1.546997,0.968002c0.432999,0.425003 0.770996,0.937004 1.014008,1.534004c0.241989,0.598999 0.362991,1.265999 0.362991,2.001999c0,0.720001 -0.121002,1.374001 -0.362991,1.962997c-0.242004,0.590004 -0.581009,1.097 -1.014008,1.521004c-0.434006,0.424995 -0.949005,0.755997 -1.546997,0.993996c-0.598999,0.237999 -1.257996,0.362 -1.977005,0.371002c-0.710999,0 -1.362991,-0.114998 -1.957001,-0.345001s-1.103989,-0.555 -1.53299,-0.975998c-0.430008,-0.420006 -0.766006,-0.925003 -1.008011,-1.514c-0.241989,-0.588005 -0.362991,-1.243004 -0.362991,-1.962006zm1.715012,-0.103996c0,0.494003 0.076004,0.948997 0.229004,1.364998c0.149994,0.416 0.365005,0.775002 0.643005,1.079002c0.276993,0.303001 0.608994,0.541 0.993988,0.714996c0.387009,0.173004 0.817001,0.260002 1.295013,0.260002c0.47699,0 0.908997,-0.086998 1.298996,-0.260002c0.390991,-0.173996 0.724991,-0.411995 1.001999,-0.714996c0.276993,-0.304001 0.490997,-0.663002 0.643005,-1.079002c0.151993,-0.416 0.228989,-0.870995 0.228989,-1.364998c0,-0.459 -0.075989,-0.889 -0.228989,-1.287003c-0.151001,-0.397995 -0.365005,-0.746994 -0.643005,-1.046997c-0.277008,-0.299004 -0.611008,-0.531998 -1.001999,-0.701004c-0.389999,-0.168999 -0.822006,-0.253998 -1.298996,-0.253998c-0.478012,0 -0.908005,0.084999 -1.295013,0.253998c-0.384995,0.169006 -0.716995,0.402 -0.993988,0.701004c-0.277008,0.300003 -0.492004,0.648003 -0.643005,1.046997c-0.153015,0.398003 -0.229004,0.828003 -0.229004,1.287003z\"/> <path d=\"m228.029007,86.295998h2.17099l4.459,6.838005h0.026001v-6.838005h1.637009v9.204002h-2.07901l-4.550003,-7.058998h-0.025986v7.058998h-1.638v-9.204002l0,0z\"/> <path d=\"m242.341995,86.295998h1.417007l3.966003,9.204002h-1.873001l-0.85701,-2.106003h-3.990997l-0.832993,2.106003h-1.833008l4.003998,-9.204002zm2.080002,5.694l-1.416992,-3.743996l-1.442001,3.743996h2.858994l0,0z\"/> <path d=\"m249.738007,86.295998h1.638992v7.722h3.912003v1.482002h-5.550995v-9.204002l0,0z\"/> </g> </g> </symbol>";
	module.exports = sprite.add(image, "grv-tlpt-logo-full");

/***/ },
/* 440 */
/***/ function(module, exports, __webpack_require__) {

	var Sprite = __webpack_require__(441);
	var globalSprite = new Sprite();
	
	if (document.body) {
	  globalSprite.elem = globalSprite.render(document.body);
	} else {
	  document.addEventListener('DOMContentLoaded', function () {
	    globalSprite.elem = globalSprite.render(document.body);
	  }, false);
	}
	
	module.exports = globalSprite;


/***/ },
/* 441 */
/***/ function(module, exports, __webpack_require__) {

	var Sniffr = __webpack_require__(434);
	
	/**
	 * List of SVG attributes to fix url target in them
	 * @type {string[]}
	 */
	var fixAttributes = [
	  'clipPath',
	  'colorProfile',
	  'src',
	  'cursor',
	  'fill',
	  'filter',
	  'marker',
	  'markerStart',
	  'markerMid',
	  'markerEnd',
	  'mask',
	  'stroke'
	];
	
	/**
	 * Query to find'em
	 * @type {string}
	 */
	var fixAttributesQuery = '[' + fixAttributes.join('],[') + ']';
	/**
	 * @type {RegExp}
	 */
	var URI_FUNC_REGEX = /^url\((.*)\)$/;
	
	/**
	 * Convert array-like to array
	 * @param {Object} arrayLike
	 * @returns {Array.<*>}
	 */
	function arrayFrom(arrayLike) {
	  return Array.prototype.slice.call(arrayLike, 0);
	}
	
	/**
	 * Handles forbidden symbols which cannot be directly used inside attributes with url(...) content.
	 * Adds leading slash for the brackets
	 * @param {string} url
	 * @return {string} encoded url
	 */
	function encodeUrlForEmbedding(url) {
	  return url.replace(/\(|\)/g, "\\$&");
	}
	
	/**
	 * Replaces prefix in `url()` functions
	 * @param {Element} svg
	 * @param {string} currentUrlPrefix
	 * @param {string} newUrlPrefix
	 */
	function baseUrlWorkAround(svg, currentUrlPrefix, newUrlPrefix) {
	  var nodes = svg.querySelectorAll(fixAttributesQuery);
	
	  if (!nodes) {
	    return;
	  }
	
	  arrayFrom(nodes).forEach(function (node) {
	    if (!node.attributes) {
	      return;
	    }
	
	    arrayFrom(node.attributes).forEach(function (attribute) {
	      var attributeName = attribute.localName.toLowerCase();
	
	      if (fixAttributes.indexOf(attributeName) !== -1) {
	        var match = URI_FUNC_REGEX.exec(node.getAttribute(attributeName));
	
	        // Do not touch urls with unexpected prefix
	        if (match && match[1].indexOf(currentUrlPrefix) === 0) {
	          var referenceUrl = encodeUrlForEmbedding(newUrlPrefix + match[1].split(currentUrlPrefix)[1]);
	          node.setAttribute(attributeName, 'url(' + referenceUrl + ')');
	        }
	      }
	    });
	  });
	}
	
	/**
	 * Because of Firefox bug #353575 gradients and patterns don't work if they are within a symbol.
	 * To workaround this we move the gradient definition outside the symbol element
	 * @see https://bugzilla.mozilla.org/show_bug.cgi?id=353575
	 * @param {Element} svg
	 */
	var FirefoxSymbolBugWorkaround = function (svg) {
	  var defs = svg.querySelector('defs');
	
	  var moveToDefsElems = svg.querySelectorAll('symbol linearGradient, symbol radialGradient, symbol pattern');
	  for (var i = 0, len = moveToDefsElems.length; i < len; i++) {
	    defs.appendChild(moveToDefsElems[i]);
	  }
	};
	
	/**
	 * @type {string}
	 */
	var DEFAULT_URI_PREFIX = '#';
	
	/**
	 * @type {string}
	 */
	var xLinkHref = 'xlink:href';
	/**
	 * @type {string}
	 */
	var xLinkNS = 'http://www.w3.org/1999/xlink';
	/**
	 * @type {string}
	 */
	var svgOpening = '<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="' + xLinkNS + '"';
	/**
	 * @type {string}
	 */
	var svgClosing = '</svg>';
	/**
	 * @type {string}
	 */
	var contentPlaceHolder = '{content}';
	
	/**
	 * Representation of SVG sprite
	 * @constructor
	 */
	function Sprite() {
	  var baseElement = document.getElementsByTagName('base')[0];
	  var currentUrl = window.location.href.split('#')[0];
	  var baseUrl = baseElement && baseElement.href;
	  this.urlPrefix = baseUrl && baseUrl !== currentUrl ? currentUrl + DEFAULT_URI_PREFIX : DEFAULT_URI_PREFIX;
	
	  var sniffr = new Sniffr();
	  sniffr.sniff();
	  this.browser = sniffr.browser;
	  this.content = [];
	
	  if (this.browser.name !== 'ie' && baseUrl) {
	    window.addEventListener('spriteLoaderLocationUpdated', function (e) {
	      var currentPrefix = this.urlPrefix;
	      var newUrlPrefix = e.detail.newUrl.split(DEFAULT_URI_PREFIX)[0] + DEFAULT_URI_PREFIX;
	      baseUrlWorkAround(this.svg, currentPrefix, newUrlPrefix);
	      this.urlPrefix = newUrlPrefix;
	
	      if (this.browser.name === 'firefox' || this.browser.name === 'edge' || this.browser.name === 'chrome' && this.browser.version[0] >= 49) {
	        var nodes = arrayFrom(document.querySelectorAll('use[*|href]'));
	        nodes.forEach(function (node) {
	          var href = node.getAttribute(xLinkHref);
	          if (href && href.indexOf(currentPrefix) === 0) {
	            node.setAttributeNS(xLinkNS, xLinkHref, newUrlPrefix + href.split(DEFAULT_URI_PREFIX)[1]);
	          }
	        });
	      }
	    }.bind(this));
	  }
	}
	
	Sprite.styles = ['position:absolute', 'width:0', 'height:0', 'visibility:hidden'];
	
	Sprite.spriteTemplate = svgOpening + ' style="'+ Sprite.styles.join(';') +'"><defs>' + contentPlaceHolder + '</defs>' + svgClosing;
	Sprite.symbolTemplate = svgOpening + '>' + contentPlaceHolder + svgClosing;
	
	/**
	 * @type {Array<String>}
	 */
	Sprite.prototype.content = null;
	
	/**
	 * @param {String} content
	 * @param {String} id
	 */
	Sprite.prototype.add = function (content, id) {
	  if (this.svg) {
	    this.appendSymbol(content);
	  }
	
	  this.content.push(content);
	
	  return DEFAULT_URI_PREFIX + id;
	};
	
	/**
	 *
	 * @param content
	 * @param template
	 * @returns {Element}
	 */
	Sprite.prototype.wrapSVG = function (content, template) {
	  var svgString = template.replace(contentPlaceHolder, content);
	
	  var svg = new DOMParser().parseFromString(svgString, 'image/svg+xml').documentElement;
	
	  if (this.browser.name !== 'ie' && this.urlPrefix) {
	    baseUrlWorkAround(svg, DEFAULT_URI_PREFIX, this.urlPrefix);
	  }
	
	  return svg;
	};
	
	Sprite.prototype.appendSymbol = function (content) {
	  var symbol = this.wrapSVG(content, Sprite.symbolTemplate).childNodes[0];
	
	  this.svg.querySelector('defs').appendChild(symbol);
	  if (this.browser.name === 'firefox') {
	    FirefoxSymbolBugWorkaround(this.svg);
	  }
	};
	
	/**
	 * @returns {String}
	 */
	Sprite.prototype.toString = function () {
	  var wrapper = document.createElement('div');
	  wrapper.appendChild(this.render());
	  return wrapper.innerHTML;
	};
	
	/**
	 * @param {HTMLElement} [target]
	 * @param {Boolean} [prepend=true]
	 * @returns {HTMLElement} Rendered sprite node
	 */
	Sprite.prototype.render = function (target, prepend) {
	  target = target || null;
	  prepend = typeof prepend === 'boolean' ? prepend : true;
	
	  var svg = this.wrapSVG(this.content.join(''), Sprite.spriteTemplate);
	
	  if (this.browser.name === 'firefox') {
	    FirefoxSymbolBugWorkaround(svg);
	  }
	
	  if (target) {
	    if (prepend && target.childNodes[0]) {
	      target.insertBefore(svg, target.childNodes[0]);
	    } else {
	      target.appendChild(svg);
	    }
	  }
	
	  this.svg = svg;
	
	  return svg;
	};
	
	module.exports = Sprite;


/***/ },
/* 442 */,
/* 443 */
/***/ function(module, exports) {

	module.exports = Terminal;

/***/ }
]);
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJzb3VyY2VzIjpbIndlYnBhY2s6Ly8vLi9zcmMvYXBwL3JlYWN0b3IuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb25maWcuanMiLCJ3ZWJwYWNrOi8vL2V4dGVybmFsIFwialF1ZXJ5XCIiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21tb24vbG9nZ2VyLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvc2VydmljZXMvYXBpLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvc2VydmljZXMvc2Vzc2lvbi5qcyIsIndlYnBhY2s6Ly8vZXh0ZXJuYWwgXCJfXCIiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2ljb25zLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvbXNnUGFnZS5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL3RhYmxlLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvbm90aWZpY2F0aW9ucy9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9jdXJyZW50U2Vzc2lvbi9nZXR0ZXJzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9zZXNzaW9ucy9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc3RvcmVkU2Vzc2lvbnNGaWx0ZXIvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvdXNlci9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvdXNlci9nZXR0ZXJzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvc2VydmljZXMvYXV0aC5qcyIsIndlYnBhY2s6Ly8vLi9+L2V2ZW50cy9ldmVudHMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21tb24vb2JqZWN0VXRpbHMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21tb24vdGVybWluYWwuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21tb24vdHR5LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9jdXJyZW50U2Vzc2lvbi9zZXNzaW9uTGVmdFBhbmVsLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvZ29vZ2xlQXV0aExvZ28uanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9pbnB1dFNlYXJjaC5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL25vZGVzL25vZGVMaXN0LmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvc2Vzc2lvbnMvbGlzdEl0ZW1zLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvYXBwL2FjdGlvblR5cGVzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hcHAvYXBwU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2N1cnJlbnRTZXNzaW9uL2FjdGlvblR5cGVzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9jdXJyZW50U2Vzc2lvbi9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9jdXJyZW50U2Vzc2lvbi9jdXJyZW50U2Vzc2lvblN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9jdXJyZW50U2Vzc2lvbi9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvZGlhbG9ncy9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvZGlhbG9ncy9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9kaWFsb2dzL2RpYWxvZ1N0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvbm90aWZpY2F0aW9ucy9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9jb25zdGFudHMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3N0b3JlZFNlc3Npb25zRmlsdGVyL2FjdGlvblR5cGVzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy91c2VyL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvdXNlclN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvc2VydmljZXMvYXBpVXRpbHMuanMiLCJ3ZWJwYWNrOi8vLy4vfi9mYmpzL2xpYi9DU1NDb3JlLmpzIiwid2VicGFjazovLy8uL34vcmVhY3QtYWRkb25zLWNzcy10cmFuc2l0aW9uLWdyb3VwL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tbW9uL3BhdHRlcm5VdGlscy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbW1vbi90dHlFdmVudHMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21tb24vdHR5UGxheWVyLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9hcHAuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9jdXJyZW50U2Vzc2lvbi9hY3RpdmVTZXNzaW9uLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvY3VycmVudFNlc3Npb24vbWFpbi5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2N1cnJlbnRTZXNzaW9uL3Nlc3Npb25QbGF5ZXIuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9kYXRlUGlja2VyLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvaW5kZXguanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9sb2dpbi5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL25hdkxlZnRCYXIuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9uZXdVc2VyLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvbm9kZXMvbWFpbi5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL25vdGlmaWNhdGlvbkhvc3QuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9zZWxlY3ROb2RlRGlhbG9nLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvc2Vzc2lvbnMvYWN0aXZlU2Vzc2lvbkxpc3QuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9zZXNzaW9ucy9tYWluLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvc2Vzc2lvbnMvc3RvcmVkU2Vzc2lvbkxpc3QuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy90ZXJtaW5hbC5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9pbmRleC5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FwcC9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hcHAvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvYXBwL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9kaWFsb2dzL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9ub2RlU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL25vdGlmaWNhdGlvbnMvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvbm90aWZpY2F0aW9ucy9ub3RpZmljYXRpb25TdG9yZS5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvcmVzdEFwaVN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9zZXNzaW9ucy9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvc2Vzc2lvblN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9zdG9yZWRTZXNzaW9uc0ZpbHRlci9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9zdG9yZWRTZXNzaW9uc0ZpbHRlci9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc3RvcmVkU2Vzc2lvbnNGaWx0ZXIvc3RvcmVkU2Vzc2lvbkZpbHRlclN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy91c2VyL3VzZXJJbnZpdGVTdG9yZS5qcyIsIndlYnBhY2s6Ly8vLi9+L3JlYWN0L2xpYi9SZWFjdENTU1RyYW5zaXRpb25Hcm91cC5qcyIsIndlYnBhY2s6Ly8vLi9+L3JlYWN0L2xpYi9SZWFjdENTU1RyYW5zaXRpb25Hcm91cENoaWxkLmpzIiwid2VicGFjazovLy8uL34vc25pZmZyL3NyYy9zbmlmZnIuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2Fzc2V0cy9pbWcvc3ZnL2dydi10bHB0LWxvZ28tZnVsbC5zdmciLCJ3ZWJwYWNrOi8vLy4vfi9zdmctc3ByaXRlLWxvYWRlci9saWIvd2ViL2dsb2JhbC1zcHJpdGUuanMiLCJ3ZWJwYWNrOi8vLy4vfi9zdmctc3ByaXRlLWxvYWRlci9saWIvd2ViL3Nwcml0ZS5qcyIsIndlYnBhY2s6Ly8vZXh0ZXJuYWwgXCJUZXJtaW5hbFwiIl0sIm5hbWVzIjpbXSwibWFwcGluZ3MiOiI7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7O3NDQWdCd0IsRUFBWTs7QUFFcEMsS0FBSSxPQUFPLEdBQUcsSUFBSSxDQUFDOzs7QUFHbkIsS0FBSSxLQUFLLEdBQUcsTUFBTSxDQUFDLFNBQVMsQ0FBQztBQUM3QixLQUFHLEtBQUssSUFBSSxLQUFLLENBQUMsTUFBTSxDQUFDLElBQUksQ0FBQyxNQUFNLEtBQUssQ0FBQyxFQUFDO0FBQ3pDLFVBQU8sR0FBRyxLQUFLLENBQUM7RUFDakI7O0FBRUQsS0FBTSxPQUFPLEdBQUcsdUJBQVk7QUFDMUIsUUFBSyxFQUFFLE9BQU87RUFDZixDQUFDOztBQUVGLE9BQU0sQ0FBQyxPQUFPLEdBQUcsT0FBTyxDQUFDOztzQkFFVixPQUFPOzs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7OztnQkNoQkEsbUJBQU8sQ0FBQyxHQUF5QixDQUFDOztLQUFuRCxhQUFhLFlBQWIsYUFBYTs7QUFDbEIsS0FBSSxDQUFDLEdBQUcsbUJBQU8sQ0FBQyxFQUFRLENBQUMsQ0FBQzs7QUFFMUIsS0FBSSxHQUFHLEdBQUc7O0FBRVIsVUFBTyxFQUFFLE1BQU0sQ0FBQyxRQUFRLENBQUMsTUFBTTs7QUFFL0IsVUFBTyxFQUFFLG9EQUFvRDs7QUFFN0QscUJBQWtCLEVBQUUsRUFBRTs7QUFFdEIsb0JBQWlCLEVBQUUsU0FBUzs7QUFFNUIsT0FBSSxFQUFFO0FBQ0osb0JBQWUsRUFBRSxFQUFFO0lBQ3BCOztBQUVELFNBQU0sRUFBRTtBQUNOLFFBQUcsRUFBRSxNQUFNO0FBQ1gsV0FBTSxFQUFFLGFBQWE7QUFDckIsVUFBSyxFQUFFLFlBQVk7QUFDbkIsVUFBSyxFQUFFLFlBQVk7QUFDbkIsa0JBQWEsRUFBRSxvQkFBb0I7QUFDbkMsWUFBTyxFQUFFLDJCQUEyQjtBQUNwQyxhQUFRLEVBQUUsZUFBZTtBQUN6QixTQUFJLEVBQUUsMkJBQTJCO0FBQ2pDLGlCQUFZLEVBQUUsZUFBZTtJQUM5Qjs7QUFFRCxNQUFHLEVBQUU7QUFDSCxRQUFHLEVBQUUseUVBQXlFO0FBQzlFLG1CQUFjLEVBQUMsMkJBQTJCO0FBQzFDLGNBQVMsRUFBRSxrQ0FBa0M7QUFDN0MsZ0JBQVcsRUFBRSxxQkFBcUI7QUFDbEMsb0JBQWUsRUFBRSxxQ0FBcUM7QUFDdEQsZUFBVSxFQUFFLHVDQUF1QztBQUNuRCxtQkFBYyxFQUFFLGtCQUFrQjtBQUNsQyxpQkFBWSxFQUFFLHVFQUF1RTtBQUNyRiwwQkFBcUIsRUFBRSxzREFBc0Q7QUFDN0UsK0JBQTBCLHNEQUFzRDs7QUFFaEYsY0FBUyxxQkFBQyxRQUFRLEVBQUUsUUFBUSxFQUFDO0FBQzNCLGNBQU8sR0FBRyxDQUFDLE9BQU8sR0FBRyxhQUFhLENBQUMsR0FBRyxDQUFDLEdBQUcsQ0FBQyxHQUFHLEVBQUUsRUFBQyxRQUFRLEVBQVIsUUFBUSxFQUFFLFFBQVEsRUFBUixRQUFRLEVBQUMsQ0FBQyxDQUFDO01BQ3ZFOztBQUVELDRCQUF1QixtQ0FBQyxJQUFpQixFQUFDO1dBQWpCLEdBQUcsR0FBSixJQUFpQixDQUFoQixHQUFHO1dBQUUsS0FBSyxHQUFYLElBQWlCLENBQVgsS0FBSztXQUFFLEdBQUcsR0FBaEIsSUFBaUIsQ0FBSixHQUFHOztBQUN0QyxjQUFPLGFBQWEsQ0FBQyxHQUFHLENBQUMsR0FBRyxDQUFDLFlBQVksRUFBRSxFQUFDLEdBQUcsRUFBSCxHQUFHLEVBQUUsS0FBSyxFQUFMLEtBQUssRUFBRSxHQUFHLEVBQUgsR0FBRyxFQUFDLENBQUMsQ0FBQztNQUMvRDs7QUFFRCw2QkFBd0Isb0NBQUMsR0FBRyxFQUFDO0FBQzNCLGNBQU8sYUFBYSxDQUFDLEdBQUcsQ0FBQyxHQUFHLENBQUMscUJBQXFCLEVBQUUsRUFBQyxHQUFHLEVBQUgsR0FBRyxFQUFDLENBQUMsQ0FBQztNQUM1RDs7QUFFRCx3QkFBbUIsK0JBQUMsSUFBSSxFQUFDO0FBQ3ZCLFdBQUksTUFBTSxHQUFHLElBQUksQ0FBQyxTQUFTLENBQUMsSUFBSSxDQUFDLENBQUM7QUFDbEMsY0FBTyxhQUFhLENBQUMsR0FBRyxDQUFDLEdBQUcsQ0FBQywwQkFBMEIsRUFBRSxFQUFDLE1BQU0sRUFBTixNQUFNLEVBQUMsQ0FBQyxDQUFDO01BQ3BFOztBQUVELHVCQUFrQiw4QkFBQyxHQUFHLEVBQUM7QUFDckIsY0FBTyxhQUFhLENBQUMsR0FBRyxDQUFDLEdBQUcsQ0FBQyxlQUFlLEdBQUMsT0FBTyxFQUFFLEVBQUMsR0FBRyxFQUFILEdBQUcsRUFBQyxDQUFDLENBQUM7TUFDOUQ7O0FBRUQsMEJBQXFCLGlDQUFDLEdBQUcsRUFBQztBQUN4QixjQUFPLGFBQWEsQ0FBQyxHQUFHLENBQUMsR0FBRyxDQUFDLGVBQWUsR0FBQyxPQUFPLEVBQUUsRUFBQyxHQUFHLEVBQUgsR0FBRyxFQUFDLENBQUMsQ0FBQztNQUM5RDs7QUFFRCxpQkFBWSx3QkFBQyxXQUFXLEVBQUM7QUFDdkIsY0FBTyxhQUFhLENBQUMsR0FBRyxDQUFDLEdBQUcsQ0FBQyxVQUFVLEVBQUUsRUFBQyxXQUFXLEVBQVgsV0FBVyxFQUFDLENBQUMsQ0FBQztNQUN6RDs7QUFFRCwwQkFBcUIsbUNBQUU7QUFDckIsV0FBSSxRQUFRLEdBQUcsYUFBYSxFQUFFLENBQUM7QUFDL0IsY0FBVSxRQUFRLGdDQUE2QjtNQUNoRDs7QUFFRCxjQUFTLHVCQUFFO0FBQ1QsV0FBSSxRQUFRLEdBQUcsYUFBYSxFQUFFLENBQUM7QUFDL0IsY0FBVSxRQUFRLGdDQUE2QjtNQUNoRDs7SUFHRjs7QUFFRCxhQUFVLHNCQUFDLEdBQUcsRUFBQztBQUNiLFlBQU8sR0FBRyxDQUFDLE9BQU8sR0FBRyxHQUFHLENBQUM7SUFDMUI7O0FBRUQsMkJBQXdCLG9DQUFDLEdBQUcsRUFBQztBQUMzQixZQUFPLGFBQWEsQ0FBQyxHQUFHLENBQUMsTUFBTSxDQUFDLGFBQWEsRUFBRSxFQUFDLEdBQUcsRUFBSCxHQUFHLEVBQUMsQ0FBQyxDQUFDO0lBQ3ZEOztBQUVELG1CQUFnQiw4QkFBRTtBQUNoQixZQUFPLEdBQUcsQ0FBQyxJQUFJLENBQUMsZUFBZSxDQUFDO0lBQ2pDOztBQUVELE9BQUksa0JBQVc7U0FBVixNQUFNLHlEQUFDLEVBQUU7O0FBQ1osTUFBQyxDQUFDLE1BQU0sQ0FBQyxJQUFJLEVBQUUsSUFBSSxFQUFFLE1BQU0sQ0FBQyxDQUFDO0lBQzlCO0VBQ0Y7O3NCQUVjLEdBQUc7O0FBRWxCLFVBQVMsYUFBYSxHQUFFO0FBQ3RCLE9BQUksTUFBTSxHQUFHLFFBQVEsQ0FBQyxRQUFRLElBQUksUUFBUSxHQUFDLFFBQVEsR0FBQyxPQUFPLENBQUM7QUFDNUQsT0FBSSxRQUFRLEdBQUcsUUFBUSxDQUFDLFFBQVEsSUFBRSxRQUFRLENBQUMsSUFBSSxHQUFHLEdBQUcsR0FBQyxRQUFRLENBQUMsSUFBSSxHQUFFLEVBQUUsQ0FBQyxDQUFDO0FBQ3pFLGVBQVUsTUFBTSxHQUFHLFFBQVEsQ0FBRztFQUMvQjs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7OztBQzFIRCx5Qjs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7OztLQ2dCTSxNQUFNO0FBQ0MsWUFEUCxNQUFNLEdBQ2tCO1NBQWhCLElBQUkseURBQUMsU0FBUzs7MkJBRHRCLE1BQU07O0FBRVIsU0FBSSxDQUFDLElBQUksR0FBRyxJQUFJLENBQUM7SUFDbEI7O0FBSEcsU0FBTSxXQUtWLEdBQUcsa0JBQXVCO1NBQXRCLEtBQUsseURBQUMsS0FBSzs7dUNBQUssSUFBSTtBQUFKLFdBQUk7OztBQUN0QixZQUFPLENBQUMsS0FBSyxPQUFDLENBQWQsT0FBTyxXQUFjLElBQUksQ0FBQyxJQUFJLCtCQUF3QixJQUFJLEVBQUMsQ0FBQztJQUM3RDs7QUFQRyxTQUFNLFdBU1YsS0FBSyxvQkFBVTt3Q0FBTixJQUFJO0FBQUosV0FBSTs7O0FBQ1gsU0FBSSxDQUFDLEdBQUcsT0FBUixJQUFJLEdBQUssT0FBTyxTQUFLLElBQUksRUFBQyxDQUFDO0lBQzVCOztBQVhHLFNBQU0sV0FhVixJQUFJLG1CQUFVO3dDQUFOLElBQUk7QUFBSixXQUFJOzs7QUFDVixTQUFJLENBQUMsR0FBRyxPQUFSLElBQUksR0FBSyxNQUFNLFNBQUssSUFBSSxFQUFDLENBQUM7SUFDM0I7O0FBZkcsU0FBTSxXQWlCVixJQUFJLG1CQUFVO3dDQUFOLElBQUk7QUFBSixXQUFJOzs7QUFDVixTQUFJLENBQUMsR0FBRyxPQUFSLElBQUksR0FBSyxNQUFNLFNBQUssSUFBSSxFQUFDLENBQUM7SUFDM0I7O0FBbkJHLFNBQU0sV0FxQlYsS0FBSyxvQkFBVTt3Q0FBTixJQUFJO0FBQUosV0FBSTs7O0FBQ1gsU0FBSSxDQUFDLEdBQUcsT0FBUixJQUFJLEdBQUssT0FBTyxTQUFLLElBQUksRUFBQyxDQUFDO0lBQzVCOztVQXZCRyxNQUFNOzs7c0JBMEJHO0FBQ2IsU0FBTSxFQUFFO3dDQUFJLElBQUk7QUFBSixXQUFJOzs7NkJBQVMsTUFBTSxnQkFBSSxJQUFJO0lBQUM7RUFDekM7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7O0FDNUJELEtBQUksQ0FBQyxHQUFHLG1CQUFPLENBQUMsRUFBUSxDQUFDLENBQUM7QUFDMUIsS0FBSSxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxFQUFXLENBQUMsQ0FBQzs7QUFFbkMsS0FBTSxHQUFHLEdBQUc7O0FBRVYsTUFBRyxlQUFDLElBQUksRUFBRSxJQUFJLEVBQUUsU0FBUyxFQUFDO0FBQ3hCLFlBQU8sR0FBRyxDQUFDLElBQUksQ0FBQyxFQUFDLEdBQUcsRUFBRSxJQUFJLEVBQUUsSUFBSSxFQUFFLElBQUksQ0FBQyxTQUFTLENBQUMsSUFBSSxDQUFDLEVBQUUsSUFBSSxFQUFFLEtBQUssRUFBQyxFQUFFLFNBQVMsQ0FBQyxDQUFDO0lBQ2xGOztBQUVELE9BQUksZ0JBQUMsSUFBSSxFQUFFLElBQUksRUFBRSxTQUFTLEVBQUM7QUFDekIsWUFBTyxHQUFHLENBQUMsSUFBSSxDQUFDLEVBQUMsR0FBRyxFQUFFLElBQUksRUFBRSxJQUFJLEVBQUUsSUFBSSxDQUFDLFNBQVMsQ0FBQyxJQUFJLENBQUMsRUFBRSxJQUFJLEVBQUUsTUFBTSxFQUFDLEVBQUUsU0FBUyxDQUFDLENBQUM7SUFDbkY7O0FBRUQsTUFBRyxlQUFDLElBQUksRUFBQztBQUNQLFlBQU8sR0FBRyxDQUFDLElBQUksQ0FBQyxFQUFDLEdBQUcsRUFBRSxJQUFJLEVBQUMsQ0FBQyxDQUFDO0lBQzlCOztBQUVELE9BQUksZ0JBQUMsR0FBRyxFQUFtQjtTQUFqQixTQUFTLHlEQUFHLElBQUk7O0FBQ3hCLFNBQUksVUFBVSxHQUFHO0FBQ2YsV0FBSSxFQUFFLEtBQUs7QUFDWCxlQUFRLEVBQUUsTUFBTTtBQUNoQixpQkFBVSxFQUFFLG9CQUFTLEdBQUcsRUFBRTtBQUN4QixhQUFHLFNBQVMsRUFBQztzQ0FDSyxPQUFPLENBQUMsV0FBVyxFQUFFOztlQUEvQixLQUFLLHdCQUFMLEtBQUs7O0FBQ1gsY0FBRyxDQUFDLGdCQUFnQixDQUFDLGVBQWUsRUFBQyxTQUFTLEdBQUcsS0FBSyxDQUFDLENBQUM7VUFDekQ7UUFDRDtNQUNIOztBQUVELFlBQU8sQ0FBQyxDQUFDLElBQUksQ0FBQyxDQUFDLENBQUMsTUFBTSxDQUFDLEVBQUUsRUFBRSxVQUFVLEVBQUUsR0FBRyxDQUFDLENBQUMsQ0FBQztJQUM5QztFQUNGOztBQUVELE9BQU0sQ0FBQyxPQUFPLEdBQUcsR0FBRyxDOzs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7OztnQkNqQzBCLG1CQUFPLENBQUMsRUFBYyxDQUFDOztLQUEvRCxjQUFjLFlBQWQsY0FBYztLQUFFLG1CQUFtQixZQUFuQixtQkFBbUI7O0FBRXpDLEtBQU0sTUFBTSxHQUFHLG1CQUFPLENBQUMsRUFBbUIsQ0FBQyxDQUFDLE1BQU0sQ0FBQyxtQkFBbUIsQ0FBQyxDQUFDO0FBQ3hFLEtBQU0sYUFBYSxHQUFHLFVBQVUsQ0FBQzs7QUFFakMsS0FBSSxRQUFRLEdBQUcsbUJBQW1CLEVBQUUsQ0FBQzs7QUFFckMsS0FBSSxPQUFPLEdBQUc7O0FBRVosT0FBSSxrQkFBd0I7U0FBdkIsT0FBTyx5REFBQyxjQUFjOztBQUN6QixhQUFRLEdBQUcsT0FBTyxDQUFDO0lBQ3BCOztBQUVELGFBQVUsd0JBQUU7QUFDVixZQUFPLFFBQVEsQ0FBQztJQUNqQjs7QUFFRCxjQUFXLHVCQUFDLFFBQVEsRUFBQztBQUNuQixpQkFBWSxDQUFDLE9BQU8sQ0FBQyxhQUFhLEVBQUUsSUFBSSxDQUFDLFNBQVMsQ0FBQyxRQUFRLENBQUMsQ0FBQyxDQUFDO0lBQy9EOztBQUVELGNBQVcseUJBQUU7QUFDWCxTQUFJLElBQUksR0FBRyxZQUFZLENBQUMsT0FBTyxDQUFDLGFBQWEsQ0FBQyxDQUFDO0FBQy9DLFNBQUcsSUFBSSxFQUFDO0FBQ04sY0FBTyxJQUFJLENBQUMsS0FBSyxDQUFDLElBQUksQ0FBQyxDQUFDO01BQ3pCOzs7QUFHRCxTQUFJLFNBQVMsR0FBRyxRQUFRLENBQUMsY0FBYyxDQUFDLGNBQWMsQ0FBQyxDQUFDO0FBQ3hELFNBQUcsU0FBUyxLQUFLLElBQUksRUFBRTtBQUNyQixXQUFHO0FBQ0QsYUFBSSxJQUFJLEdBQUcsTUFBTSxDQUFDLElBQUksQ0FBQyxTQUFTLENBQUMsV0FBVyxDQUFDLENBQUM7QUFDOUMsYUFBSSxRQUFRLEdBQUcsSUFBSSxDQUFDLEtBQUssQ0FBQyxJQUFJLENBQUMsQ0FBQztBQUNoQyxhQUFHLFFBQVEsQ0FBQyxLQUFLLEVBQUM7O0FBRWhCLGVBQUksQ0FBQyxXQUFXLENBQUMsUUFBUSxDQUFDLENBQUM7O0FBRTNCLG9CQUFTLENBQUMsTUFBTSxFQUFFLENBQUM7QUFDbkIsa0JBQU8sUUFBUSxDQUFDO1VBQ2pCO1FBQ0YsUUFBTSxHQUFHLEVBQUM7QUFDVCxlQUFNLENBQUMsS0FBSyxDQUFDLDBCQUEwQixFQUFFLEdBQUcsQ0FBQyxDQUFDO1FBQy9DO01BQ0Y7O0FBRUQsWUFBTyxFQUFFLENBQUM7SUFDWDs7QUFFRCxRQUFLLG1CQUFFO0FBQ0wsaUJBQVksQ0FBQyxLQUFLLEVBQUU7SUFDckI7O0VBRUY7O0FBRUQsT0FBTSxDQUFDLE9BQU8sR0FBRyxPQUFPLEM7Ozs7Ozs7Ozs7Ozs7Ozs7OztBQ3RFeEIsb0I7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7OztBQ2dCQSxLQUFJLEtBQUssR0FBRyxtQkFBTyxDQUFDLENBQU8sQ0FBQyxDQUFDO0FBQzdCLEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsR0FBdUMsQ0FBQyxDQUFDO0FBQy9ELEtBQUksVUFBVSxHQUFHLG1CQUFPLENBQUMsRUFBWSxDQUFDLENBQUM7O0FBRXZDLEtBQU0sWUFBWSxHQUFHLFNBQWYsWUFBWTtVQUNoQjs7T0FBSyxTQUFTLEVBQUMsb0JBQW9CO0tBQUMsNkJBQUssU0FBUyxFQUFFLE9BQVEsR0FBRTtJQUFNO0VBQ3JFOztBQUVELEtBQU0sUUFBUSxHQUFHLFNBQVgsUUFBUSxDQUFJLElBQWlCLEVBQUc7bUJBQXBCLElBQWlCLENBQWhCLElBQUk7T0FBSixJQUFJLDZCQUFDLEVBQUU7T0FBRSxNQUFNLEdBQWhCLElBQWlCLENBQVAsTUFBTTs7QUFDaEMsT0FBSSxTQUFTLEdBQUcsVUFBVSxDQUFDLGVBQWUsRUFBRTtBQUMxQyxhQUFRLEVBQUcsTUFBTTtJQUNsQixDQUFDLENBQUM7O0FBRUgsVUFDRTs7T0FBSyxLQUFLLEVBQUUsSUFBSyxFQUFDLFNBQVMsRUFBRSxTQUFVO0tBQ3JDOzs7T0FDRTs7O1NBQVMsSUFBSSxDQUFDLENBQUMsQ0FBQztRQUFVO01BQ3JCO0lBQ0gsQ0FDUDtFQUNGLENBQUM7O1NBRU0sWUFBWSxHQUFaLFlBQVk7U0FBRSxRQUFRLEdBQVIsUUFBUSxDOzs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7OztBQ3RCOUIsS0FBSSxLQUFLLEdBQUcsbUJBQU8sQ0FBQyxDQUFPLENBQUMsQ0FBQzs7QUFFN0IsS0FBTSxzQkFBc0IsR0FBRyx5RUFBeUUsQ0FBQztBQUN6RyxLQUFNLHNCQUFzQixHQUFHLGtHQUFrRyxDQUFDO0FBQ2xJLEtBQU0saUJBQWlCLEdBQUcsK0JBQStCLENBQUM7O0FBRTFELEtBQU0sbUJBQW1CLEdBQUcsOEJBQThCLENBQUM7QUFDM0QsS0FBTSwyQkFBMkIsb0VBQW1FLENBQUM7O0FBRXJHLEtBQU0sd0JBQXdCLEdBQUcsMEJBQTBCLENBQUM7QUFDNUQsS0FBTSxnQ0FBZ0Msc0RBQXFELENBQUM7O0FBRTVGLEtBQU0sT0FBTyxHQUFHO0FBQ2QsT0FBSSxFQUFFLE1BQU07QUFDWixRQUFLLEVBQUUsT0FBTztFQUNmOztBQUVELEtBQU0sVUFBVSxHQUFHO0FBQ2pCLGtCQUFlLEVBQUUsY0FBYztBQUMvQixpQkFBYyxFQUFFLGdCQUFnQjtBQUNoQyxZQUFTLEVBQUUsV0FBVztFQUN2QixDQUFDOztBQUVGLEtBQU0sU0FBUyxHQUFHO0FBQ2hCLGdCQUFhLEVBQUUsZUFBZTtFQUMvQixDQUFDOztBQUVGLEtBQUksV0FBVyxHQUFHLEtBQUssQ0FBQyxXQUFXLENBQUM7OztBQUNsQyxTQUFNLG9CQUFFO3lCQUNnQixJQUFJLENBQUMsS0FBSyxDQUFDLE1BQU07U0FBbEMsSUFBSSxpQkFBSixJQUFJO1NBQUUsT0FBTyxpQkFBUCxPQUFPOztBQUNsQixTQUFHLElBQUksS0FBSyxPQUFPLENBQUMsS0FBSyxFQUFDO0FBQ3hCLGNBQU8sb0JBQUMsU0FBUyxJQUFDLElBQUksRUFBRSxPQUFRLEdBQUU7TUFDbkM7O0FBRUQsU0FBRyxJQUFJLEtBQUssT0FBTyxDQUFDLElBQUksRUFBQztBQUN2QixjQUFPLG9CQUFDLFFBQVEsSUFBQyxJQUFJLEVBQUUsT0FBUSxHQUFFO01BQ2xDOztBQUVELFlBQU8sSUFBSSxDQUFDO0lBQ2I7RUFDRixDQUFDLENBQUM7O0FBRUgsS0FBSSxTQUFTLEdBQUcsS0FBSyxDQUFDLFdBQVcsQ0FBQzs7O0FBQ2hDLFNBQU0sb0JBQUc7U0FDRixJQUFJLEdBQUksSUFBSSxDQUFDLEtBQUssQ0FBbEIsSUFBSTs7QUFDVCxTQUFJLE9BQU8sR0FDVDs7O09BQ0U7OztTQUFLLGlCQUFpQjtRQUFNO01BRS9CLENBQUM7O0FBRUYsU0FBRyxJQUFJLEtBQUssVUFBVSxDQUFDLGVBQWUsRUFBQztBQUNyQyxjQUFPLEdBQ0w7OztTQUNFOzs7V0FBSyxzQkFBc0I7VUFBTTtRQUVwQztNQUNGOztBQUVELFNBQUcsSUFBSSxLQUFLLFVBQVUsQ0FBQyxjQUFjLEVBQUM7QUFDcEMsY0FBTyxHQUNMOzs7U0FDRTs7O1dBQUssd0JBQXdCO1VBQU07U0FDbkM7OztXQUFNLGdDQUFnQztVQUFPO1FBRWhEO01BQ0Y7O0FBRUQsU0FBSSxJQUFJLEtBQUssVUFBVSxDQUFDLFNBQVMsRUFBQztBQUNoQyxjQUFPLEdBQ0w7OztTQUNFOzs7V0FBSyxtQkFBbUI7VUFBTTtTQUM5Qjs7O1dBQU0sMkJBQTJCO1VBQU87UUFFM0MsQ0FBQztNQUNIOztBQUVELFlBQ0U7O1NBQUssU0FBUyxFQUFDLGNBQWM7T0FDM0I7O1dBQUssU0FBUyxFQUFDLFlBQVk7U0FBQywyQkFBRyxTQUFTLEVBQUMsZUFBZSxHQUFLOztRQUFPO09BQ25FLE9BQU87T0FDUjs7V0FBSyxTQUFTLEVBQUMsaUJBQWlCOztTQUF1RDs7YUFBRyxJQUFJLEVBQUMsc0RBQXNEOztVQUEyQjtRQUFNO01BQ2xMLENBQ047SUFDSDtFQUNGLENBQUM7O0FBRUYsS0FBSSxRQUFRLEdBQUcsS0FBSyxDQUFDLFdBQVcsQ0FBQzs7O0FBQy9CLFNBQU0sb0JBQUc7U0FDRixJQUFJLEdBQUksSUFBSSxDQUFDLEtBQUssQ0FBbEIsSUFBSTs7QUFDVCxTQUFJLE9BQU8sR0FBRyxJQUFJLENBQUM7O0FBRW5CLFNBQUcsSUFBSSxLQUFLLFNBQVMsQ0FBQyxhQUFhLEVBQUM7QUFDbEMsY0FBTyxHQUNMOzs7U0FDRTs7O1dBQUssc0JBQXNCO1VBQU07UUFFcEMsQ0FBQztNQUNIOztBQUVELFlBQ0U7O1NBQUssU0FBUyxFQUFDLGNBQWM7T0FDM0I7O1dBQUssU0FBUyxFQUFDLFlBQVk7U0FBQywyQkFBRyxTQUFTLEVBQUMsZUFBZSxHQUFLOztRQUFPO09BQ25FLE9BQU87TUFDSixDQUNOO0lBQ0g7RUFDRixDQUFDOztBQUVGLEtBQUksUUFBUSxHQUFHLFNBQVgsUUFBUTtVQUNWLG9CQUFDLFNBQVMsSUFBQyxJQUFJLEVBQUUsVUFBVSxDQUFDLFNBQVUsR0FBRTtFQUN6Qzs7U0FFTyxTQUFTLEdBQVQsU0FBUztTQUFFLFFBQVEsR0FBUixRQUFRO1NBQUUsUUFBUSxHQUFSLFFBQVE7U0FBRSxVQUFVLEdBQVYsVUFBVTtTQUFFLFdBQVcsR0FBWCxXQUFXLEM7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7QUNqSDlELEtBQUksS0FBSyxHQUFHLG1CQUFPLENBQUMsQ0FBTyxDQUFDLENBQUM7O0FBRTdCLEtBQU0sZ0JBQWdCLEdBQUcsU0FBbkIsZ0JBQWdCLENBQUksSUFBcUM7T0FBcEMsUUFBUSxHQUFULElBQXFDLENBQXBDLFFBQVE7T0FBRSxJQUFJLEdBQWYsSUFBcUMsQ0FBMUIsSUFBSTtPQUFFLFNBQVMsR0FBMUIsSUFBcUMsQ0FBcEIsU0FBUzs7T0FBSyxLQUFLLDRCQUFwQyxJQUFxQzs7VUFDN0Q7QUFBQyxpQkFBWTtLQUFLLEtBQUs7S0FDcEIsSUFBSSxDQUFDLFFBQVEsQ0FBQyxDQUFDLFNBQVMsQ0FBQztJQUNiO0VBQ2hCLENBQUM7Ozs7O0FBS0YsS0FBTSxTQUFTLEdBQUc7QUFDaEIsTUFBRyxFQUFFLEtBQUs7QUFDVixPQUFJLEVBQUUsTUFBTTtFQUNiLENBQUM7O0FBRUYsS0FBTSxhQUFhLEdBQUcsU0FBaEIsYUFBYSxDQUFJLEtBQVMsRUFBRztPQUFYLE9BQU8sR0FBUixLQUFTLENBQVIsT0FBTzs7QUFDN0IsT0FBSSxHQUFHLEdBQUcscUNBQXFDO0FBQy9DLE9BQUcsT0FBTyxLQUFLLFNBQVMsQ0FBQyxJQUFJLEVBQUM7QUFDNUIsUUFBRyxJQUFJLE9BQU87SUFDZjs7QUFFRCxPQUFJLE9BQU8sS0FBSyxTQUFTLENBQUMsR0FBRyxFQUFDO0FBQzVCLFFBQUcsSUFBSSxNQUFNO0lBQ2Q7O0FBRUQsVUFBUSwyQkFBRyxTQUFTLEVBQUUsR0FBSSxHQUFLLENBQUU7RUFDbEMsQ0FBQzs7Ozs7QUFLRixLQUFJLGNBQWMsR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFDckMsU0FBTSxvQkFBRztrQkFDMEIsSUFBSSxDQUFDLEtBQUs7U0FBdEMsT0FBTyxVQUFQLE9BQU87U0FBRSxLQUFLLFVBQUwsS0FBSzs7U0FBSyxLQUFLOztBQUU3QixZQUNFO0FBQUMsbUJBQVk7T0FBSyxLQUFLO09BQ3JCOztXQUFHLE9BQU8sRUFBRSxJQUFJLENBQUMsWUFBYTtTQUMzQixLQUFLO1FBQ0o7T0FDSixvQkFBQyxhQUFhLElBQUMsT0FBTyxFQUFFLE9BQVEsR0FBRTtNQUNyQixDQUNmO0lBQ0g7O0FBRUQsZUFBWSx3QkFBQyxDQUFDLEVBQUU7QUFDZCxNQUFDLENBQUMsY0FBYyxFQUFFLENBQUM7QUFDbkIsU0FBRyxJQUFJLENBQUMsS0FBSyxDQUFDLFlBQVksRUFBRTs7QUFFMUIsV0FBSSxNQUFNLEdBQUcsU0FBUyxDQUFDLElBQUksQ0FBQztBQUM1QixXQUFHLElBQUksQ0FBQyxLQUFLLENBQUMsT0FBTyxFQUFDO0FBQ3BCLGVBQU0sR0FBRyxJQUFJLENBQUMsS0FBSyxDQUFDLE9BQU8sS0FBSyxTQUFTLENBQUMsSUFBSSxHQUFHLFNBQVMsQ0FBQyxHQUFHLEdBQUcsU0FBUyxDQUFDLElBQUksQ0FBQztRQUNqRjtBQUNELFdBQUksQ0FBQyxLQUFLLENBQUMsWUFBWSxDQUFDLElBQUksQ0FBQyxLQUFLLENBQUMsU0FBUyxFQUFFLE1BQU0sQ0FBQyxDQUFDO01BQ3ZEO0lBQ0Y7RUFDRixDQUFDLENBQUM7Ozs7O0FBS0gsS0FBSSxZQUFZLEdBQUcsS0FBSyxDQUFDLFdBQVcsQ0FBQzs7O0FBQ25DLFNBQU0sb0JBQUU7QUFDTixTQUFJLEtBQUssR0FBRyxJQUFJLENBQUMsS0FBSyxDQUFDO0FBQ3ZCLFlBQU8sS0FBSyxDQUFDLFFBQVEsR0FBRzs7U0FBSSxHQUFHLEVBQUUsS0FBSyxDQUFDLEdBQUksRUFBQyxTQUFTLEVBQUMsZ0JBQWdCO09BQUUsS0FBSyxDQUFDLFFBQVE7TUFBTSxHQUFHOztTQUFJLEdBQUcsRUFBRSxLQUFLLENBQUMsR0FBSTtPQUFFLEtBQUssQ0FBQyxRQUFRO01BQU0sQ0FBQztJQUMxSTtFQUNGLENBQUMsQ0FBQzs7Ozs7QUFLSCxLQUFJLFFBQVEsR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFFL0IsZUFBWSx3QkFBQyxRQUFRLEVBQUM7OztBQUNwQixTQUFJLEtBQUssR0FBRyxRQUFRLENBQUMsR0FBRyxDQUFDLFVBQUMsSUFBSSxFQUFFLEtBQUssRUFBRztBQUN0QyxjQUFPLE1BQUssVUFBVSxDQUFDLElBQUksQ0FBQyxLQUFLLENBQUMsTUFBTSxhQUFHLEtBQUssRUFBTCxLQUFLLEVBQUUsR0FBRyxFQUFFLEtBQUssRUFBRSxRQUFRLEVBQUUsSUFBSSxJQUFLLElBQUksQ0FBQyxLQUFLLEVBQUUsQ0FBQztNQUMvRixDQUFDOztBQUVGLFlBQU87O1NBQU8sU0FBUyxFQUFDLGtCQUFrQjtPQUFDOzs7U0FBSyxLQUFLO1FBQU07TUFBUTtJQUNwRTs7QUFFRCxhQUFVLHNCQUFDLFFBQVEsRUFBQzs7O0FBQ2xCLFNBQUksS0FBSyxHQUFHLElBQUksQ0FBQyxLQUFLLENBQUMsUUFBUSxDQUFDO0FBQ2hDLFNBQUksSUFBSSxHQUFHLEVBQUUsQ0FBQztBQUNkLFVBQUksSUFBSSxDQUFDLEdBQUcsQ0FBQyxFQUFFLENBQUMsR0FBRyxLQUFLLEVBQUUsQ0FBQyxFQUFHLEVBQUM7QUFDN0IsV0FBSSxLQUFLLEdBQUcsUUFBUSxDQUFDLEdBQUcsQ0FBQyxVQUFDLElBQUksRUFBRSxLQUFLLEVBQUc7QUFDdEMsZ0JBQU8sT0FBSyxVQUFVLENBQUMsSUFBSSxDQUFDLEtBQUssQ0FBQyxJQUFJLGFBQUcsUUFBUSxFQUFFLENBQUMsRUFBRSxHQUFHLEVBQUUsS0FBSyxFQUFFLFFBQVEsRUFBRSxLQUFLLElBQUssSUFBSSxDQUFDLEtBQUssRUFBRSxDQUFDO1FBQ3BHLENBQUM7O0FBRUYsV0FBSSxDQUFDLElBQUksQ0FBQzs7V0FBSSxHQUFHLEVBQUUsQ0FBRTtTQUFFLEtBQUs7UUFBTSxDQUFDLENBQUM7TUFDckM7O0FBRUQsWUFBTzs7O09BQVEsSUFBSTtNQUFTLENBQUM7SUFDOUI7O0FBRUQsYUFBVSxzQkFBQyxJQUFJLEVBQUUsU0FBUyxFQUFDO0FBQ3pCLFNBQUksT0FBTyxHQUFHLElBQUksQ0FBQztBQUNuQixTQUFJLEtBQUssQ0FBQyxjQUFjLENBQUMsSUFBSSxDQUFDLEVBQUU7QUFDN0IsY0FBTyxHQUFHLEtBQUssQ0FBQyxZQUFZLENBQUMsSUFBSSxFQUFFLFNBQVMsQ0FBQyxDQUFDO01BQy9DLE1BQU0sSUFBSSxPQUFPLElBQUksS0FBSyxVQUFVLEVBQUU7QUFDckMsY0FBTyxHQUFHLElBQUksQ0FBQyxTQUFTLENBQUMsQ0FBQztNQUMzQjs7QUFFRCxZQUFPLE9BQU8sQ0FBQztJQUNqQjs7QUFFRCxTQUFNLG9CQUFHO0FBQ1AsU0FBSSxRQUFRLEdBQUcsRUFBRSxDQUFDO0FBQ2xCLFVBQUssQ0FBQyxRQUFRLENBQUMsT0FBTyxDQUFDLElBQUksQ0FBQyxLQUFLLENBQUMsUUFBUSxFQUFFLFVBQUMsS0FBSyxFQUFLO0FBQ3JELFdBQUksS0FBSyxJQUFJLElBQUksRUFBRTtBQUNqQixnQkFBTztRQUNSOztBQUVELFdBQUcsS0FBSyxDQUFDLElBQUksQ0FBQyxXQUFXLEtBQUssZ0JBQWdCLEVBQUM7QUFDN0MsZUFBTSwwQkFBMEIsQ0FBQztRQUNsQzs7QUFFRCxlQUFRLENBQUMsSUFBSSxDQUFDLEtBQUssQ0FBQyxDQUFDO01BQ3RCLENBQUMsQ0FBQzs7QUFFSCxTQUFJLFVBQVUsR0FBRyxrQkFBa0IsR0FBRyxJQUFJLENBQUMsS0FBSyxDQUFDLFNBQVMsQ0FBQzs7QUFFM0QsWUFDRTs7U0FBTyxTQUFTLEVBQUUsVUFBVztPQUMxQixJQUFJLENBQUMsWUFBWSxDQUFDLFFBQVEsQ0FBQztPQUMzQixJQUFJLENBQUMsVUFBVSxDQUFDLFFBQVEsQ0FBQztNQUNwQixDQUNSO0lBQ0g7RUFDRixDQUFDOztBQUVGLEtBQUksY0FBYyxHQUFHLEtBQUssQ0FBQyxXQUFXLENBQUM7OztBQUNyQyxTQUFNLEVBQUUsa0JBQVc7QUFDakIsV0FBTSxJQUFJLEtBQUssQ0FBQyxrREFBa0QsQ0FBQyxDQUFDO0lBQ3JFO0VBQ0YsQ0FBQzs7QUFFRixLQUFNLGNBQWMsR0FBRyxTQUFqQixjQUFjLENBQUksS0FBTTtPQUFMLElBQUksR0FBTCxLQUFNLENBQUwsSUFBSTtVQUMzQjs7T0FBSyxTQUFTLEVBQUMsa0RBQWtEO0tBQUM7OztPQUFPLElBQUk7TUFBUTtJQUFNO0VBQzVGOztzQkFFYyxRQUFRO1NBRUgsTUFBTSxHQUF4QixjQUFjO1NBQ0YsS0FBSyxHQUFqQixRQUFRO1NBQ1EsSUFBSSxHQUFwQixZQUFZO1NBQ1EsUUFBUSxHQUE1QixnQkFBZ0I7U0FDaEIsY0FBYyxHQUFkLGNBQWM7U0FDZCxhQUFhLEdBQWIsYUFBYTtTQUNiLFNBQVMsR0FBVCxTQUFTO1NBQ1QsY0FBYyxHQUFkLGNBQWMsQzs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7QUN2SmhCLEtBQU0sc0JBQXNCLEdBQUcsU0FBekIsc0JBQXNCLENBQUksUUFBUTtVQUFLLENBQUUsQ0FBQyxZQUFZLENBQUMsRUFBRSxVQUFDLEtBQUssRUFBSTtBQUN2RSxTQUFJLE1BQU0sR0FBRyxLQUFLLENBQUMsSUFBSSxDQUFDLGNBQUk7Y0FBRyxJQUFJLENBQUMsR0FBRyxDQUFDLElBQUksQ0FBQyxLQUFLLFFBQVE7TUFBQSxDQUFDLENBQUM7QUFDNUQsWUFBTyxDQUFDLE1BQU0sR0FBRyxFQUFFLEdBQUcsTUFBTSxDQUFDLEdBQUcsQ0FBQyxVQUFVLENBQUMsQ0FBQztJQUM5QyxDQUFDO0VBQUEsQ0FBQzs7QUFFSCxLQUFNLFlBQVksR0FBRyxDQUFFLENBQUMsWUFBWSxDQUFDLEVBQUUsVUFBQyxLQUFLLEVBQUk7QUFDN0MsVUFBTyxLQUFLLENBQUMsR0FBRyxDQUFDLFVBQUMsSUFBSSxFQUFHO0FBQ3ZCLFNBQUksUUFBUSxHQUFHLElBQUksQ0FBQyxHQUFHLENBQUMsSUFBSSxDQUFDLENBQUM7QUFDOUIsWUFBTztBQUNMLFNBQUUsRUFBRSxRQUFRO0FBQ1osZUFBUSxFQUFFLElBQUksQ0FBQyxHQUFHLENBQUMsVUFBVSxDQUFDO0FBQzlCLFdBQUksRUFBRSxPQUFPLENBQUMsSUFBSSxDQUFDO0FBQ25CLFdBQUksRUFBRSxJQUFJLENBQUMsR0FBRyxDQUFDLE1BQU0sQ0FBQztNQUN2QjtJQUNGLENBQUMsQ0FBQyxJQUFJLEVBQUUsQ0FBQztFQUNaLENBQ0QsQ0FBQzs7QUFFRixVQUFTLE9BQU8sQ0FBQyxJQUFJLEVBQUM7QUFDcEIsT0FBSSxTQUFTLEdBQUcsRUFBRSxDQUFDO0FBQ25CLE9BQUksTUFBTSxHQUFHLElBQUksQ0FBQyxHQUFHLENBQUMsUUFBUSxDQUFDLENBQUM7O0FBRWhDLE9BQUcsTUFBTSxFQUFDO0FBQ1IsV0FBTSxDQUFDLFFBQVEsRUFBRSxDQUFDLE9BQU8sRUFBRSxDQUFDLE9BQU8sQ0FBQyxjQUFJLEVBQUU7QUFDeEMsZ0JBQVMsQ0FBQyxJQUFJLENBQUM7QUFDYixhQUFJLEVBQUUsSUFBSSxDQUFDLENBQUMsQ0FBQztBQUNiLGNBQUssRUFBRSxJQUFJLENBQUMsQ0FBQyxDQUFDO1FBQ2YsQ0FBQyxDQUFDO01BQ0osQ0FBQyxDQUFDO0lBQ0o7O0FBRUQsU0FBTSxHQUFHLElBQUksQ0FBQyxHQUFHLENBQUMsWUFBWSxDQUFDLENBQUM7O0FBRWhDLE9BQUcsTUFBTSxFQUFDO0FBQ1IsV0FBTSxDQUFDLFFBQVEsRUFBRSxDQUFDLE9BQU8sRUFBRSxDQUFDLE9BQU8sQ0FBQyxjQUFJLEVBQUU7QUFDeEMsZ0JBQVMsQ0FBQyxJQUFJLENBQUM7QUFDYixhQUFJLEVBQUUsSUFBSSxDQUFDLENBQUMsQ0FBQztBQUNiLGNBQUssRUFBRSxJQUFJLENBQUMsQ0FBQyxDQUFDLENBQUMsR0FBRyxDQUFDLFFBQVEsQ0FBQztBQUM1QixnQkFBTyxFQUFFLElBQUksQ0FBQyxDQUFDLENBQUMsQ0FBQyxHQUFHLENBQUMsU0FBUyxDQUFDO1FBQ2hDLENBQUMsQ0FBQztNQUNKLENBQUMsQ0FBQztJQUNKOztBQUVELFVBQU8sU0FBUyxDQUFDO0VBQ2xCOztzQkFFYztBQUNiLGVBQVksRUFBWixZQUFZO0FBQ1oseUJBQXNCLEVBQXRCLHNCQUFzQjtFQUN2Qjs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7O0FDakRELEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsQ0FBYSxDQUFDLENBQUM7O2dCQUNILG1CQUFPLENBQUMsR0FBZSxDQUFDOztLQUFwRCxzQkFBc0IsWUFBdEIsc0JBQXNCO3NCQUViOztBQUViLFlBQVMscUJBQUMsSUFBSSxFQUFnQjtTQUFkLEtBQUsseURBQUMsT0FBTzs7QUFDM0IsYUFBUSxDQUFDLEVBQUMsT0FBTyxFQUFFLElBQUksRUFBRSxJQUFJLEVBQUUsSUFBSSxFQUFFLEtBQUssRUFBTCxLQUFLLEVBQUMsQ0FBQyxDQUFDO0lBQzlDOztBQUVELGNBQVcsdUJBQUMsSUFBSSxFQUFrQjtTQUFoQixLQUFLLHlEQUFDLFNBQVM7O0FBQy9CLGFBQVEsQ0FBQyxFQUFDLFNBQVMsRUFBQyxJQUFJLEVBQUUsSUFBSSxFQUFFLElBQUksRUFBRSxLQUFLLEVBQUwsS0FBSyxFQUFDLENBQUMsQ0FBQztJQUMvQzs7QUFFRCxXQUFRLG9CQUFDLElBQUksRUFBZTtTQUFiLEtBQUsseURBQUMsTUFBTTs7QUFDekIsYUFBUSxDQUFDLEVBQUMsTUFBTSxFQUFDLElBQUksRUFBRSxJQUFJLEVBQUUsSUFBSSxFQUFFLEtBQUssRUFBTCxLQUFLLEVBQUMsQ0FBQyxDQUFDO0lBQzVDOztBQUVELGNBQVcsdUJBQUMsSUFBSSxFQUFrQjtTQUFoQixLQUFLLHlEQUFDLFNBQVM7O0FBQy9CLGFBQVEsQ0FBQyxFQUFDLFNBQVMsRUFBRSxJQUFJLEVBQUUsSUFBSSxFQUFFLElBQUksRUFBRSxLQUFLLEVBQUwsS0FBSyxFQUFDLENBQUMsQ0FBQztJQUNoRDs7RUFFRjs7QUFFRCxVQUFTLFFBQVEsQ0FBQyxHQUFHLEVBQUM7QUFDcEIsVUFBTyxDQUFDLFFBQVEsQ0FBQyxzQkFBc0IsRUFBRSxHQUFHLENBQUMsQ0FBQztFQUMvQzs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Z0JDekJrQixtQkFBTyxDQUFDLEVBQThCLENBQUM7O0tBQXJELFVBQVUsWUFBVixVQUFVOztBQUVmLEtBQU0sY0FBYyxHQUFHLENBQUUsQ0FBQyxzQkFBc0IsQ0FBQyxFQUFFLENBQUMsZUFBZSxDQUFDLEVBQ3BFLFVBQUMsT0FBTyxFQUFFLFFBQVEsRUFBSztBQUNuQixPQUFHLENBQUMsT0FBTyxFQUFDO0FBQ1YsWUFBTyxJQUFJLENBQUM7SUFDYjs7Ozs7OztBQU9ELE9BQUksY0FBYyxHQUFHO0FBQ25CLGlCQUFZLEVBQUUsT0FBTyxDQUFDLEdBQUcsQ0FBQyxjQUFjLENBQUM7QUFDekMsYUFBUSxFQUFFLE9BQU8sQ0FBQyxHQUFHLENBQUMsVUFBVSxDQUFDO0FBQ2pDLFNBQUksRUFBRSxPQUFPLENBQUMsR0FBRyxDQUFDLE1BQU0sQ0FBQztBQUN6QixhQUFRLEVBQUUsT0FBTyxDQUFDLEdBQUcsQ0FBQyxVQUFVLENBQUM7QUFDakMsYUFBUSxFQUFFLFNBQVM7QUFDbkIsVUFBSyxFQUFFLE9BQU8sQ0FBQyxHQUFHLENBQUMsT0FBTyxDQUFDO0FBQzNCLFFBQUcsRUFBRSxPQUFPLENBQUMsR0FBRyxDQUFDLEtBQUssQ0FBQztBQUN2QixTQUFJLEVBQUUsU0FBUztBQUNmLFNBQUksRUFBRSxTQUFTO0lBQ2hCLENBQUM7Ozs7O0FBS0YsT0FBRyxRQUFRLENBQUMsR0FBRyxDQUFDLGNBQWMsQ0FBQyxHQUFHLENBQUMsRUFBQztBQUNsQyxTQUFJLFFBQVEsR0FBRyxVQUFVLENBQUMsUUFBUSxDQUFDLEdBQUcsQ0FBQyxjQUFjLENBQUMsR0FBRyxDQUFDLENBQUMsQ0FBQzs7QUFFNUQsbUJBQWMsQ0FBQyxPQUFPLEdBQUcsUUFBUSxDQUFDLE9BQU8sQ0FBQztBQUMxQyxtQkFBYyxDQUFDLFFBQVEsR0FBRyxRQUFRLENBQUMsUUFBUSxDQUFDO0FBQzVDLG1CQUFjLENBQUMsUUFBUSxHQUFHLFFBQVEsQ0FBQyxRQUFRLENBQUM7QUFDNUMsbUJBQWMsQ0FBQyxNQUFNLEdBQUcsUUFBUSxDQUFDLE1BQU0sQ0FBQztBQUN4QyxtQkFBYyxDQUFDLElBQUksR0FBRyxRQUFRLENBQUMsSUFBSSxDQUFDO0FBQ3BDLG1CQUFjLENBQUMsSUFBSSxHQUFHLFFBQVEsQ0FBQyxJQUFJLENBQUM7SUFDckM7O0FBRUQsVUFBTyxjQUFjLENBQUM7RUFDdkIsQ0FDRixDQUFDOztzQkFFYTtBQUNiLGlCQUFjLEVBQWQsY0FBYztFQUNmOzs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7c0NDN0NxQixFQUFXOzs7O3NCQUVsQix1QkFBVTtBQUN2Qix1QkFBb0IsRUFBRSxJQUFJO0FBQzFCLHNCQUFtQixFQUFFLElBQUk7QUFDekIsNkJBQTBCLEVBQUUsSUFBSTtFQUNqQyxDQUFDOzs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7QUNORixLQUFJLE9BQU8sR0FBRyxtQkFBTyxDQUFDLENBQWEsQ0FBQyxDQUFDO0FBQ3JDLEtBQUksR0FBRyxHQUFHLG1CQUFPLENBQUMsRUFBa0IsQ0FBQyxDQUFDO0FBQ3RDLEtBQUksUUFBUSxHQUFHLG1CQUFPLENBQUMsR0FBdUIsQ0FBQyxDQUFDO0FBQ2hELEtBQUksR0FBRyxHQUFHLG1CQUFPLENBQUMsQ0FBWSxDQUFDLENBQUM7O2dCQUNkLG1CQUFPLENBQUMsRUFBbUMsQ0FBQzs7S0FBekQsU0FBUyxZQUFULFNBQVM7O0FBRWQsS0FBTSxNQUFNLEdBQUcsbUJBQU8sQ0FBQyxFQUFtQixDQUFDLENBQUMsTUFBTSxDQUFDLGtCQUFrQixDQUFDLENBQUM7O2lCQUNoQixtQkFBTyxDQUFDLEVBQWUsQ0FBQzs7S0FBdkUsb0JBQW9CLGFBQXBCLG9CQUFvQjtLQUFFLG1CQUFtQixhQUFuQixtQkFBbUI7O0FBRWpELEtBQU0sT0FBTyxHQUFHOztBQUVkLGVBQVksd0JBQUMsR0FBRyxFQUFDO0FBQ2YsWUFBTyxHQUFHLENBQUMsR0FBRyxDQUFDLEdBQUcsQ0FBQyxHQUFHLENBQUMsa0JBQWtCLENBQUMsR0FBRyxDQUFDLENBQUMsQ0FBQyxJQUFJLENBQUMsY0FBSSxFQUFFO0FBQ3pELFdBQUcsSUFBSSxJQUFJLElBQUksQ0FBQyxPQUFPLEVBQUM7QUFDdEIsZ0JBQU8sQ0FBQyxRQUFRLENBQUMsbUJBQW1CLEVBQUUsSUFBSSxDQUFDLE9BQU8sQ0FBQyxDQUFDO1FBQ3JEO01BQ0YsQ0FBQyxDQUFDO0lBQ0o7O0FBRUQsZ0JBQWEsMkJBQTZDO3NFQUFILEVBQUU7O1NBQTFDLEdBQUcsUUFBSCxHQUFHO1NBQUUsR0FBRyxRQUFILEdBQUc7MkJBQUUsS0FBSztTQUFMLEtBQUssOEJBQUMsR0FBRyxDQUFDLGtCQUFrQjs7QUFDbkQsU0FBSSxLQUFLLEdBQUcsR0FBRyxJQUFJLElBQUksSUFBSSxFQUFFLENBQUM7QUFDOUIsU0FBSSxNQUFNLEdBQUc7QUFDWCxZQUFLLEVBQUUsQ0FBQyxDQUFDO0FBQ1QsWUFBSyxFQUFMLEtBQUs7QUFDTCxZQUFLLEVBQUwsS0FBSztBQUNMLFVBQUcsRUFBSCxHQUFHO01BQ0osQ0FBQzs7QUFFRixZQUFPLFFBQVEsQ0FBQyxjQUFjLENBQUMsTUFBTSxDQUFDLENBQ25DLElBQUksQ0FBQyxVQUFDLElBQUksRUFBSztBQUNkLGNBQU8sQ0FBQyxRQUFRLENBQUMsb0JBQW9CLEVBQUUsSUFBSSxDQUFDLFFBQVEsQ0FBQyxDQUFDO01BQ3ZELENBQUMsQ0FDRCxJQUFJLENBQUMsVUFBQyxHQUFHLEVBQUc7QUFDWCxnQkFBUyxDQUFDLHFDQUFxQyxDQUFDLENBQUM7QUFDakQsYUFBTSxDQUFDLEtBQUssQ0FBQyxlQUFlLEVBQUUsR0FBRyxDQUFDLENBQUM7TUFDcEMsQ0FBQyxDQUFDO0lBQ047O0FBRUQsZ0JBQWEseUJBQUMsSUFBSSxFQUFDO0FBQ2pCLFlBQU8sQ0FBQyxRQUFRLENBQUMsbUJBQW1CLEVBQUUsSUFBSSxDQUFDLENBQUM7SUFDN0M7RUFDRjs7c0JBRWMsT0FBTzs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7OztnQkMzQ0EsbUJBQU8sQ0FBQyxFQUFZLENBQUM7O0tBQXJDLFdBQVcsWUFBWCxXQUFXOztBQUNqQixLQUFJLE9BQU8sR0FBRyxtQkFBTyxDQUFDLENBQWEsQ0FBQyxDQUFDO0FBQ3JDLEtBQUksR0FBRyxHQUFHLG1CQUFPLENBQUMsQ0FBWSxDQUFDLENBQUM7O0FBRWhDLEtBQU0sZ0JBQWdCLEdBQUcsU0FBbkIsZ0JBQWdCLENBQUksUUFBUTtVQUFLLENBQUMsQ0FBQyxlQUFlLENBQUMsRUFBRSxVQUFDLFFBQVEsRUFBSTtBQUN0RSxZQUFPLFFBQVEsQ0FBQyxRQUFRLEVBQUUsQ0FBQyxNQUFNLENBQUMsY0FBSSxFQUFFO0FBQ3RDLFdBQUksT0FBTyxHQUFHLElBQUksQ0FBQyxHQUFHLENBQUMsU0FBUyxDQUFDLElBQUksV0FBVyxDQUFDLEVBQUUsQ0FBQyxDQUFDO0FBQ3JELFdBQUksU0FBUyxHQUFHLE9BQU8sQ0FBQyxJQUFJLENBQUMsZUFBSztnQkFBRyxLQUFLLENBQUMsR0FBRyxDQUFDLFdBQVcsQ0FBQyxLQUFLLFFBQVE7UUFBQSxDQUFDLENBQUM7QUFDMUUsY0FBTyxTQUFTLENBQUM7TUFDbEIsQ0FBQyxDQUFDLE1BQU0sRUFBRSxDQUFDO0lBQ2IsQ0FBQztFQUFBOztBQUVGLEtBQU0sWUFBWSxHQUFHLENBQUMsQ0FBQyxlQUFlLENBQUMsRUFBRSxVQUFDLFFBQVEsRUFBSTtBQUNwRCxVQUFPLFFBQVEsQ0FBQyxRQUFRLEVBQUUsQ0FBQyxHQUFHLENBQUMsVUFBVSxDQUFDLENBQUMsSUFBSSxFQUFFLENBQUM7RUFDbkQsQ0FBQyxDQUFDOztBQUVILEtBQU0sZUFBZSxHQUFHLFNBQWxCLGVBQWUsQ0FBSSxHQUFHO1VBQUksQ0FBQyxDQUFDLGVBQWUsRUFBRSxHQUFHLENBQUMsRUFBRSxVQUFDLE9BQU8sRUFBRztBQUNsRSxTQUFHLENBQUMsT0FBTyxFQUFDO0FBQ1YsY0FBTyxJQUFJLENBQUM7TUFDYjs7QUFFRCxZQUFPLFVBQVUsQ0FBQyxPQUFPLENBQUMsQ0FBQztJQUM1QixDQUFDO0VBQUEsQ0FBQzs7QUFFSCxLQUFNLGtCQUFrQixHQUFHLFNBQXJCLGtCQUFrQixDQUFJLEdBQUc7VUFDOUIsQ0FBQyxDQUFDLGVBQWUsRUFBRSxHQUFHLEVBQUUsU0FBUyxDQUFDLEVBQUUsVUFBQyxPQUFPLEVBQUk7O0FBRS9DLFNBQUcsQ0FBQyxPQUFPLEVBQUM7QUFDVixjQUFPLEVBQUUsQ0FBQztNQUNYOztBQUVELFNBQUksaUJBQWlCLEdBQUcsaUJBQWlCLENBQUMsT0FBTyxDQUFDLENBQUMsR0FBRyxDQUFDLE1BQU0sQ0FBQyxDQUFDOztBQUUvRCxZQUFPLE9BQU8sQ0FBQyxHQUFHLENBQUMsY0FBSSxFQUFFO0FBQ3ZCLFdBQUksSUFBSSxHQUFHLElBQUksQ0FBQyxHQUFHLENBQUMsTUFBTSxDQUFDLENBQUM7QUFDNUIsY0FBTztBQUNMLGFBQUksRUFBRSxJQUFJLENBQUMsR0FBRyxDQUFDLE1BQU0sQ0FBQztBQUN0QixpQkFBUSxFQUFFLElBQUksQ0FBQyxHQUFHLENBQUMsYUFBYSxDQUFDO0FBQ2pDLGlCQUFRLEVBQUUsSUFBSSxDQUFDLEdBQUcsQ0FBQyxXQUFXLENBQUM7QUFDL0IsaUJBQVEsRUFBRSxpQkFBaUIsS0FBSyxJQUFJO1FBQ3JDO01BQ0YsQ0FBQyxDQUFDLElBQUksRUFBRSxDQUFDO0lBQ1gsQ0FBQztFQUFBLENBQUM7O0FBRUgsVUFBUyxpQkFBaUIsQ0FBQyxPQUFPLEVBQUM7QUFDakMsVUFBTyxPQUFPLENBQUMsTUFBTSxDQUFDLGNBQUk7WUFBRyxJQUFJLElBQUksQ0FBQyxJQUFJLENBQUMsR0FBRyxDQUFDLFlBQVksQ0FBQyxDQUFDO0lBQUEsQ0FBQyxDQUFDLElBQUksRUFBRSxDQUFDO0VBQ3ZFOztBQUVELFVBQVMsVUFBVSxDQUFDLE9BQU8sRUFBQztBQUMxQixPQUFJLEdBQUcsR0FBRyxPQUFPLENBQUMsR0FBRyxDQUFDLElBQUksQ0FBQyxDQUFDO0FBQzVCLE9BQUksUUFBUSxFQUFFLFFBQVEsQ0FBQztBQUN2QixPQUFJLE9BQU8sR0FBRyxPQUFPLENBQUMsUUFBUSxDQUFDLGtCQUFrQixDQUFDLEdBQUcsQ0FBQyxDQUFDLENBQUM7O0FBRXhELE9BQUcsT0FBTyxDQUFDLE1BQU0sR0FBRyxDQUFDLEVBQUM7QUFDcEIsYUFBUSxHQUFHLE9BQU8sQ0FBQyxDQUFDLENBQUMsQ0FBQyxRQUFRLENBQUM7QUFDL0IsYUFBUSxHQUFHLE9BQU8sQ0FBQyxDQUFDLENBQUMsQ0FBQyxRQUFRLENBQUM7SUFDaEM7O0FBRUQsVUFBTztBQUNMLFFBQUcsRUFBRSxHQUFHO0FBQ1IsZUFBVSxFQUFFLEdBQUcsQ0FBQyx3QkFBd0IsQ0FBQyxHQUFHLENBQUM7QUFDN0MsYUFBUSxFQUFSLFFBQVE7QUFDUixhQUFRLEVBQVIsUUFBUTtBQUNSLFdBQU0sRUFBRSxPQUFPLENBQUMsR0FBRyxDQUFDLFFBQVEsQ0FBQztBQUM3QixZQUFPLEVBQUUsT0FBTyxDQUFDLEdBQUcsQ0FBQyxTQUFTLENBQUM7QUFDL0IsZUFBVSxFQUFFLE9BQU8sQ0FBQyxHQUFHLENBQUMsYUFBYSxDQUFDO0FBQ3RDLFVBQUssRUFBRSxPQUFPLENBQUMsR0FBRyxDQUFDLE9BQU8sQ0FBQztBQUMzQixZQUFPLEVBQUUsT0FBTztBQUNoQixTQUFJLEVBQUUsT0FBTyxDQUFDLEtBQUssQ0FBQyxDQUFDLGlCQUFpQixFQUFFLEdBQUcsQ0FBQyxDQUFDO0FBQzdDLFNBQUksRUFBRSxPQUFPLENBQUMsS0FBSyxDQUFDLENBQUMsaUJBQWlCLEVBQUUsR0FBRyxDQUFDLENBQUM7SUFDOUM7RUFDRjs7c0JBRWM7QUFDYixxQkFBa0IsRUFBbEIsa0JBQWtCO0FBQ2xCLG1CQUFnQixFQUFoQixnQkFBZ0I7QUFDaEIsZUFBWSxFQUFaLFlBQVk7QUFDWixrQkFBZSxFQUFmLGVBQWU7QUFDZixhQUFVLEVBQVYsVUFBVTtBQUNWLFFBQUssRUFBRSxDQUFDLENBQUMsZUFBZSxDQUFDLEVBQUUsa0JBQVE7WUFBSSxRQUFRLENBQUMsSUFBSTtJQUFBLENBQUU7RUFDdkQ7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7OztBQ2hGRCxLQUFNLE1BQU0sR0FBRyxDQUFDLENBQUMsNkJBQTZCLENBQUMsRUFBRSxVQUFDLE1BQU0sRUFBSTtBQUMxRCxVQUFPLE1BQU0sQ0FBQyxJQUFJLEVBQUUsQ0FBQztFQUN0QixDQUFDLENBQUM7O3NCQUVZO0FBQ2IsU0FBTSxFQUFOLE1BQU07RUFDUDs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7O3NDQ05xQixFQUFXOzs7O3NCQUVsQix1QkFBVTtBQUN2QixvQkFBaUIsRUFBRSxJQUFJO0FBQ3ZCLDJCQUF3QixFQUFFLElBQUk7RUFDL0IsQ0FBQzs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7OztnQkNMMEQsbUJBQU8sQ0FBQyxHQUErQixDQUFDOztLQUEvRixlQUFlLFlBQWYsZUFBZTtLQUFFLGlCQUFpQixZQUFqQixpQkFBaUI7S0FBRSxlQUFlLFlBQWYsZUFBZTs7aUJBQ2xDLG1CQUFPLENBQUMsR0FBNkIsQ0FBQzs7S0FBdkQsYUFBYSxhQUFiLGFBQWE7O0FBRWxCLEtBQU0sTUFBTSxHQUFHLENBQUUsQ0FBQyxrQkFBa0IsQ0FBQyxFQUFFLFVBQUMsTUFBTTtVQUFLLE1BQU07RUFBQSxDQUFFLENBQUM7O0FBRTVELEtBQU0sSUFBSSxHQUFHLENBQUUsQ0FBQyxXQUFXLENBQUMsRUFBRSxVQUFDLFdBQVcsRUFBSztBQUMzQyxPQUFHLENBQUMsV0FBVyxFQUFDO0FBQ2QsWUFBTyxJQUFJLENBQUM7SUFDYjs7QUFFRCxPQUFJLElBQUksR0FBRyxXQUFXLENBQUMsR0FBRyxDQUFDLE1BQU0sQ0FBQyxJQUFJLEVBQUUsQ0FBQztBQUN6QyxPQUFJLGdCQUFnQixHQUFHLElBQUksQ0FBQyxDQUFDLENBQUMsSUFBSSxFQUFFLENBQUM7O0FBRXJDLFVBQU87QUFDTCxTQUFJLEVBQUosSUFBSTtBQUNKLHFCQUFnQixFQUFoQixnQkFBZ0I7QUFDaEIsV0FBTSxFQUFFLFdBQVcsQ0FBQyxHQUFHLENBQUMsZ0JBQWdCLENBQUMsQ0FBQyxJQUFJLEVBQUU7SUFDakQ7RUFDRixDQUNGLENBQUM7O3NCQUVhO0FBQ2IsT0FBSSxFQUFKLElBQUk7QUFDSixTQUFNLEVBQU4sTUFBTTtBQUNOLGNBQVcsRUFBRSxhQUFhLENBQUMsZUFBZSxDQUFDO0FBQzNDLFNBQU0sRUFBRSxhQUFhLENBQUMsaUJBQWlCLENBQUM7QUFDeEMsaUJBQWMsRUFBRSxhQUFhLENBQUMsZUFBZSxDQUFDO0VBQy9DOzs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7OztBQzNCRCxLQUFJLEdBQUcsR0FBRyxtQkFBTyxDQUFDLEVBQU8sQ0FBQyxDQUFDO0FBQzNCLEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsRUFBVyxDQUFDLENBQUM7QUFDbkMsS0FBSSxHQUFHLEdBQUcsbUJBQU8sQ0FBQyxDQUFZLENBQUMsQ0FBQztBQUNoQyxLQUFJLENBQUMsR0FBRyxtQkFBTyxDQUFDLEVBQVEsQ0FBQyxDQUFDOztBQUUxQixLQUFNLGVBQWUsR0FBRyxRQUFRLENBQUM7O0FBRWpDLEtBQU0sV0FBVyxHQUFHLEtBQUssR0FBRyxDQUFDLENBQUM7O0FBRTlCLEtBQUksbUJBQW1CLEdBQUcsSUFBSSxDQUFDOztBQUUvQixLQUFJLElBQUksR0FBRzs7QUFFVCxTQUFNLGtCQUFDLElBQUksRUFBRSxRQUFRLEVBQUUsS0FBSyxFQUFFLFdBQVcsRUFBQztBQUN4QyxTQUFJLElBQUksR0FBRyxFQUFDLElBQUksRUFBRSxJQUFJLEVBQUUsSUFBSSxFQUFFLFFBQVEsRUFBRSxtQkFBbUIsRUFBRSxLQUFLLEVBQUUsWUFBWSxFQUFFLFdBQVcsRUFBQyxDQUFDO0FBQy9GLFlBQU8sR0FBRyxDQUFDLElBQUksQ0FBQyxHQUFHLENBQUMsR0FBRyxDQUFDLGNBQWMsRUFBRSxJQUFJLENBQUMsQ0FDMUMsSUFBSSxDQUFDLFVBQUMsSUFBSSxFQUFHO0FBQ1osY0FBTyxDQUFDLFdBQVcsQ0FBQyxJQUFJLENBQUMsQ0FBQztBQUMxQixXQUFJLENBQUMsb0JBQW9CLEVBQUUsQ0FBQztBQUM1QixjQUFPLElBQUksQ0FBQztNQUNiLENBQUMsQ0FBQztJQUNOOztBQUVELFFBQUssaUJBQUMsSUFBSSxFQUFFLFFBQVEsRUFBRSxLQUFLLEVBQUM7QUFDMUIsU0FBSSxDQUFDLG1CQUFtQixFQUFFLENBQUM7QUFDM0IsWUFBTyxDQUFDLEtBQUssRUFBRSxDQUFDO0FBQ2hCLFlBQU8sSUFBSSxDQUFDLE1BQU0sQ0FBQyxJQUFJLEVBQUUsUUFBUSxFQUFFLEtBQUssQ0FBQyxDQUFDLElBQUksQ0FBQyxJQUFJLENBQUMsb0JBQW9CLENBQUMsQ0FBQztJQUMzRTs7QUFFRCxhQUFVLHdCQUFFO0FBQ1YsU0FBSSxRQUFRLEdBQUcsT0FBTyxDQUFDLFdBQVcsRUFBRSxDQUFDO0FBQ3JDLFNBQUcsUUFBUSxDQUFDLEtBQUssRUFBQzs7QUFFaEIsV0FBRyxJQUFJLENBQUMsdUJBQXVCLEVBQUUsS0FBSyxJQUFJLEVBQUM7QUFDekMsZ0JBQU8sSUFBSSxDQUFDLGFBQWEsRUFBRSxDQUFDLElBQUksQ0FBQyxJQUFJLENBQUMsb0JBQW9CLENBQUMsQ0FBQztRQUM3RDs7QUFFRCxjQUFPLENBQUMsQ0FBQyxRQUFRLEVBQUUsQ0FBQyxPQUFPLENBQUMsUUFBUSxDQUFDLENBQUM7TUFDdkM7O0FBRUQsWUFBTyxDQUFDLENBQUMsUUFBUSxFQUFFLENBQUMsTUFBTSxFQUFFLENBQUM7SUFDOUI7O0FBRUQsU0FBTSxvQkFBRTtBQUNOLFNBQUksQ0FBQyxtQkFBbUIsRUFBRSxDQUFDO0FBQzNCLFlBQU8sQ0FBQyxLQUFLLEVBQUUsQ0FBQztBQUNoQixTQUFJLENBQUMsU0FBUyxFQUFFLENBQUM7SUFDbEI7O0FBRUQsWUFBUyx1QkFBRTtBQUNULFdBQU0sQ0FBQyxRQUFRLEdBQUcsR0FBRyxDQUFDLE1BQU0sQ0FBQyxLQUFLLENBQUM7SUFDcEM7O0FBRUQsdUJBQW9CLGtDQUFFO0FBQ3BCLHdCQUFtQixHQUFHLFdBQVcsQ0FBQyxJQUFJLENBQUMsYUFBYSxFQUFFLFdBQVcsQ0FBQyxDQUFDO0lBQ3BFOztBQUVELHNCQUFtQixpQ0FBRTtBQUNuQixrQkFBYSxDQUFDLG1CQUFtQixDQUFDLENBQUM7QUFDbkMsd0JBQW1CLEdBQUcsSUFBSSxDQUFDO0lBQzVCOztBQUVELDBCQUF1QixxQ0FBRTtBQUN2QixZQUFPLG1CQUFtQixDQUFDO0lBQzVCOztBQUVELGdCQUFhLDJCQUFFO0FBQ2IsWUFBTyxHQUFHLENBQUMsSUFBSSxDQUFDLEdBQUcsQ0FBQyxHQUFHLENBQUMsY0FBYyxDQUFDLENBQUMsSUFBSSxDQUFDLGNBQUksRUFBRTtBQUNqRCxjQUFPLENBQUMsV0FBVyxDQUFDLElBQUksQ0FBQyxDQUFDO0FBQzFCLGNBQU8sSUFBSSxDQUFDO01BQ2IsQ0FBQyxDQUFDLElBQUksQ0FBQyxZQUFJO0FBQ1YsV0FBSSxDQUFDLE1BQU0sRUFBRSxDQUFDO01BQ2YsQ0FBQyxDQUFDO0lBQ0o7O0FBRUQsU0FBTSxrQkFBQyxJQUFJLEVBQUUsUUFBUSxFQUFFLEtBQUssRUFBQztBQUMzQixTQUFJLElBQUksR0FBRztBQUNULFdBQUksRUFBRSxJQUFJO0FBQ1YsV0FBSSxFQUFFLFFBQVE7QUFDZCwwQkFBbUIsRUFBRSxLQUFLO01BQzNCLENBQUM7O0FBRUYsWUFBTyxHQUFHLENBQUMsSUFBSSxDQUFDLEdBQUcsQ0FBQyxHQUFHLENBQUMsV0FBVyxFQUFFLElBQUksRUFBRSxLQUFLLENBQUMsQ0FBQyxJQUFJLENBQUMsY0FBSSxFQUFFO0FBQzNELGNBQU8sQ0FBQyxXQUFXLENBQUMsSUFBSSxDQUFDLENBQUM7QUFDMUIsY0FBTyxJQUFJLENBQUM7TUFDYixDQUFDLENBQUM7SUFDSjtFQUNGOztBQUVELE9BQU0sQ0FBQyxPQUFPLEdBQUcsSUFBSSxDQUFDO0FBQ3RCLE9BQU0sQ0FBQyxPQUFPLENBQUMsZUFBZSxHQUFHLGVBQWUsQzs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7OztBQzFHaEQ7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7QUFDQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0Esa0JBQWlCO0FBQ2pCO0FBQ0E7QUFDQTtBQUNBOztBQUVBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQSxvQkFBbUIsU0FBUztBQUM1QjtBQUNBO0FBQ0E7QUFDQSxJQUFHO0FBQ0g7QUFDQTtBQUNBLGdCQUFlLFNBQVM7QUFDeEI7O0FBRUE7QUFDQTtBQUNBLGdCQUFlLFNBQVM7QUFDeEI7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBLE1BQUs7QUFDTDtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBOztBQUVBOztBQUVBO0FBQ0E7QUFDQTs7QUFFQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUEsSUFBRztBQUNILHFCQUFvQixTQUFTO0FBQzdCO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7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QUVELDBDOzs7Ozs7QUNsRkE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQSxNQUFLO0FBQ0w7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0EsTUFBSzs7QUFFTDtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQSxJQUFHOztBQUVIO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTs7QUFFQTtBQUNBOztBQUVBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQSxNQUFLO0FBQ0w7QUFDQTtBQUNBO0FBQ0EsSUFBRzs7QUFFSDtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBLElBQUc7O0FBRUg7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0EsSUFBRzs7QUFFSDtBQUNBO0FBQ0E7QUFDQSxJQUFHOztBQUVIO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBLE1BQUs7QUFDTCxJQUFHOztBQUVIO0FBQ0E7QUFDQTtBQUNBLE1BQUs7QUFDTDtBQUNBO0FBQ0EsSUFBRzs7QUFFSDtBQUNBO0FBQ0E7QUFDQSxNQUFLO0FBQ0w7QUFDQTtBQUNBLElBQUc7O0FBRUg7QUFDQTtBQUNBO0FBQ0EsTUFBSztBQUNMO0FBQ0E7QUFDQSxJQUFHOztBQUVIO0FBQ0E7QUFDQTtBQUNBLEVBQUM7O0FBRUQsK0M7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7QUNwS0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FB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QUFDQSxjQUFhO0FBQ2I7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBOztBQUVBO0FBQ0EsWUFBVyxZQUFZO0FBQ3ZCLFlBQVcsUUFBUTtBQUNuQixjQUFhLFlBQVk7QUFDekI7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7O0FBRUE7QUFDQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBLE1BQUs7QUFDTDtBQUNBO0FBQ0E7O0FBRUE7O0FBRUE7QUFDQTs7QUFFQTs7Ozs7Ozs7QUN4UEEsMkIiLCJmaWxlIjoiYXBwLmpzIiwic291cmNlc0NvbnRlbnQiOlsiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG5pbXBvcnQgeyBSZWFjdG9yIH0gZnJvbSAnbnVjbGVhci1qcydcblxubGV0IGVuYWJsZWQgPSB0cnVlO1xuXG4vLyB0ZW1wb3Jhcnkgd29ya2Fyb3VuZCB0byBkaXNhYmxlIGRlYnVnIGluZm8gZHVyaW5nIHVuaXQtdGVzdHNcbmxldCBrYXJtYSA9IHdpbmRvdy5fX2thcm1hX187XG5pZihrYXJtYSAmJiBrYXJtYS5jb25maWcuYXJncy5sZW5ndGggPT09IDEpe1xuICBlbmFibGVkID0gZmFsc2U7XG59XG5cbmNvbnN0IHJlYWN0b3IgPSBuZXcgUmVhY3Rvcih7XG4gIGRlYnVnOiBlbmFibGVkXG59KVxuXG53aW5kb3cucmVhY3RvciA9IHJlYWN0b3I7XG5cbmV4cG9ydCBkZWZhdWx0IHJlYWN0b3JcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9yZWFjdG9yLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG5sZXQge2Zvcm1hdFBhdHRlcm59ID0gcmVxdWlyZSgnYXBwL2NvbW1vbi9wYXR0ZXJuVXRpbHMnKTtcbmxldCAkID0gcmVxdWlyZSgnalF1ZXJ5Jyk7XG5cbmxldCBjZmcgPSB7XG5cbiAgYmFzZVVybDogd2luZG93LmxvY2F0aW9uLm9yaWdpbixcblxuICBoZWxwVXJsOiAnaHR0cDovL2dyYXZpdGF0aW9uYWwuY29tL3RlbGVwb3J0L2RvY3MvcXVpY2tzdGFydC8nLFxuXG4gIG1heFNlc3Npb25Mb2FkU2l6ZTogNTAsXG5cbiAgZGlzcGxheURhdGVGb3JtYXQ6ICdsIExUUyBaJyxcblxuICBhdXRoOiB7XG4gICAgb2lkY19jb25uZWN0b3JzOiBbXVxuICB9LFxuXG4gIHJvdXRlczoge1xuICAgIGFwcDogJy93ZWInLFxuICAgIGxvZ291dDogJy93ZWIvbG9nb3V0JyxcbiAgICBsb2dpbjogJy93ZWIvbG9naW4nLFxuICAgIG5vZGVzOiAnL3dlYi9ub2RlcycsXG4gICAgYWN0aXZlU2Vzc2lvbjogJy93ZWIvc2Vzc2lvbnMvOnNpZCcsXG4gICAgbmV3VXNlcjogJy93ZWIvbmV3dXNlci86aW52aXRlVG9rZW4nLFxuICAgIHNlc3Npb25zOiAnL3dlYi9zZXNzaW9ucycsXG4gICAgbXNnczogJy93ZWIvbXNnLzp0eXBlKC86c3ViVHlwZSknLFxuICAgIHBhZ2VOb3RGb3VuZDogJy93ZWIvbm90Zm91bmQnXG4gIH0sXG5cbiAgYXBpOiB7XG4gICAgc3NvOiAnL3YxL3dlYmFwaS9vaWRjL2xvZ2luL3dlYj9yZWRpcmVjdF91cmw9OnJlZGlyZWN0JmNvbm5lY3Rvcl9pZD06cHJvdmlkZXInLFxuICAgIHJlbmV3VG9rZW5QYXRoOicvdjEvd2ViYXBpL3Nlc3Npb25zL3JlbmV3JyxcbiAgICBub2Rlc1BhdGg6ICcvdjEvd2ViYXBpL3NpdGVzLy1jdXJyZW50LS9ub2RlcycsXG4gICAgc2Vzc2lvblBhdGg6ICcvdjEvd2ViYXBpL3Nlc3Npb25zJyxcbiAgICBzaXRlU2Vzc2lvblBhdGg6ICcvdjEvd2ViYXBpL3NpdGVzLy1jdXJyZW50LS9zZXNzaW9ucycsXG4gICAgaW52aXRlUGF0aDogJy92MS93ZWJhcGkvdXNlcnMvaW52aXRlcy86aW52aXRlVG9rZW4nLFxuICAgIGNyZWF0ZVVzZXJQYXRoOiAnL3YxL3dlYmFwaS91c2VycycsXG4gICAgc2Vzc2lvbkNodW5rOiAnL3YxL3dlYmFwaS9zaXRlcy8tY3VycmVudC0vc2Vzc2lvbnMvOnNpZC9jaHVua3M/c3RhcnQ9OnN0YXJ0JmVuZD06ZW5kJyxcbiAgICBzZXNzaW9uQ2h1bmtDb3VudFBhdGg6ICcvdjEvd2ViYXBpL3NpdGVzLy1jdXJyZW50LS9zZXNzaW9ucy86c2lkL2NodW5rc2NvdW50JyxcbiAgICBzaXRlRXZlbnRTZXNzaW9uRmlsdGVyUGF0aDogYC92MS93ZWJhcGkvc2l0ZXMvLWN1cnJlbnQtL3Nlc3Npb25zP2ZpbHRlcj06ZmlsdGVyYCxcblxuICAgIGdldFNzb1VybChyZWRpcmVjdCwgcHJvdmlkZXIpe1xuICAgICAgcmV0dXJuIGNmZy5iYXNlVXJsICsgZm9ybWF0UGF0dGVybihjZmcuYXBpLnNzbywge3JlZGlyZWN0LCBwcm92aWRlcn0pO1xuICAgIH0sXG5cbiAgICBnZXRGZXRjaFNlc3Npb25DaHVua1VybCh7c2lkLCBzdGFydCwgZW5kfSl7XG4gICAgICByZXR1cm4gZm9ybWF0UGF0dGVybihjZmcuYXBpLnNlc3Npb25DaHVuaywge3NpZCwgc3RhcnQsIGVuZH0pO1xuICAgIH0sXG5cbiAgICBnZXRGZXRjaFNlc3Npb25MZW5ndGhVcmwoc2lkKXtcbiAgICAgIHJldHVybiBmb3JtYXRQYXR0ZXJuKGNmZy5hcGkuc2Vzc2lvbkNodW5rQ291bnRQYXRoLCB7c2lkfSk7XG4gICAgfSxcblxuICAgIGdldEZldGNoU2Vzc2lvbnNVcmwoYXJncyl7XG4gICAgICB2YXIgZmlsdGVyID0gSlNPTi5zdHJpbmdpZnkoYXJncyk7XG4gICAgICByZXR1cm4gZm9ybWF0UGF0dGVybihjZmcuYXBpLnNpdGVFdmVudFNlc3Npb25GaWx0ZXJQYXRoLCB7ZmlsdGVyfSk7XG4gICAgfSxcblxuICAgIGdldEZldGNoU2Vzc2lvblVybChzaWQpe1xuICAgICAgcmV0dXJuIGZvcm1hdFBhdHRlcm4oY2ZnLmFwaS5zaXRlU2Vzc2lvblBhdGgrJy86c2lkJywge3NpZH0pO1xuICAgIH0sXG5cbiAgICBnZXRUZXJtaW5hbFNlc3Npb25Vcmwoc2lkKXtcbiAgICAgIHJldHVybiBmb3JtYXRQYXR0ZXJuKGNmZy5hcGkuc2l0ZVNlc3Npb25QYXRoKycvOnNpZCcsIHtzaWR9KTtcbiAgICB9LFxuXG4gICAgZ2V0SW52aXRlVXJsKGludml0ZVRva2VuKXtcbiAgICAgIHJldHVybiBmb3JtYXRQYXR0ZXJuKGNmZy5hcGkuaW52aXRlUGF0aCwge2ludml0ZVRva2VufSk7XG4gICAgfSxcblxuICAgIGdldEV2ZW50U3RyZWFtQ29ublN0cigpe1xuICAgICAgdmFyIGhvc3RuYW1lID0gZ2V0V3NIb3N0TmFtZSgpO1xuICAgICAgcmV0dXJuIGAke2hvc3RuYW1lfS92MS93ZWJhcGkvc2l0ZXMvLWN1cnJlbnQtYDtcbiAgICB9LFxuXG4gICAgZ2V0VHR5VXJsKCl7XG4gICAgICB2YXIgaG9zdG5hbWUgPSBnZXRXc0hvc3ROYW1lKCk7XG4gICAgICByZXR1cm4gYCR7aG9zdG5hbWV9L3YxL3dlYmFwaS9zaXRlcy8tY3VycmVudC1gO1xuICAgIH1cblxuXG4gIH0sXG5cbiAgZ2V0RnVsbFVybCh1cmwpe1xuICAgIHJldHVybiBjZmcuYmFzZVVybCArIHVybDtcbiAgfSxcblxuICBnZXRBY3RpdmVTZXNzaW9uUm91dGVVcmwoc2lkKXtcbiAgICByZXR1cm4gZm9ybWF0UGF0dGVybihjZmcucm91dGVzLmFjdGl2ZVNlc3Npb24sIHtzaWR9KTtcbiAgfSxcblxuICBnZXRBdXRoUHJvdmlkZXJzKCl7XG4gICAgcmV0dXJuIGNmZy5hdXRoLm9pZGNfY29ubmVjdG9ycztcbiAgfSxcblxuICBpbml0KGNvbmZpZz17fSl7XG4gICAgJC5leHRlbmQodHJ1ZSwgdGhpcywgY29uZmlnKTtcbiAgfVxufVxuXG5leHBvcnQgZGVmYXVsdCBjZmc7XG5cbmZ1bmN0aW9uIGdldFdzSG9zdE5hbWUoKXtcbiAgdmFyIHByZWZpeCA9IGxvY2F0aW9uLnByb3RvY29sID09IFwiaHR0cHM6XCI/XCJ3c3M6Ly9cIjpcIndzOi8vXCI7XG4gIHZhciBob3N0cG9ydCA9IGxvY2F0aW9uLmhvc3RuYW1lKyhsb2NhdGlvbi5wb3J0ID8gJzonK2xvY2F0aW9uLnBvcnQ6ICcnKTtcbiAgcmV0dXJuIGAke3ByZWZpeH0ke2hvc3Rwb3J0fWA7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29uZmlnLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMgPSBqUXVlcnk7XG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiBleHRlcm5hbCBcImpRdWVyeVwiXG4gKiogbW9kdWxlIGlkID0gMjJcbiAqKiBtb2R1bGUgY2h1bmtzID0gMVxuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxuY2xhc3MgTG9nZ2VyIHtcbiAgY29uc3RydWN0b3IobmFtZT0nZGVmYXVsdCcpIHtcbiAgICB0aGlzLm5hbWUgPSBuYW1lO1xuICB9XG5cbiAgbG9nKGxldmVsPSdsb2cnLCAuLi5hcmdzKSB7XG4gICAgY29uc29sZVtsZXZlbF0oYCVjWyR7dGhpcy5uYW1lfV1gLCBgY29sb3I6IGJsdWU7YCwgLi4uYXJncyk7XG4gIH1cblxuICB0cmFjZSguLi5hcmdzKSB7XG4gICAgdGhpcy5sb2coJ3RyYWNlJywgLi4uYXJncyk7XG4gIH1cblxuICB3YXJuKC4uLmFyZ3MpIHtcbiAgICB0aGlzLmxvZygnd2FybicsIC4uLmFyZ3MpO1xuICB9XG5cbiAgaW5mbyguLi5hcmdzKSB7XG4gICAgdGhpcy5sb2coJ2luZm8nLCAuLi5hcmdzKTtcbiAgfVxuXG4gIGVycm9yKC4uLmFyZ3MpIHtcbiAgICB0aGlzLmxvZygnZXJyb3InLCAuLi5hcmdzKTtcbiAgfVxufVxuXG5leHBvcnQgZGVmYXVsdCB7XG4gIGNyZWF0ZTogKC4uLmFyZ3MpID0+IG5ldyBMb2dnZXIoLi4uYXJncylcbn07XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tbW9uL2xvZ2dlci5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyICQgPSByZXF1aXJlKFwialF1ZXJ5XCIpO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCcuL3Nlc3Npb24nKTtcblxuY29uc3QgYXBpID0ge1xuXG4gIHB1dChwYXRoLCBkYXRhLCB3aXRoVG9rZW4pe1xuICAgIHJldHVybiBhcGkuYWpheCh7dXJsOiBwYXRoLCBkYXRhOiBKU09OLnN0cmluZ2lmeShkYXRhKSwgdHlwZTogJ1BVVCd9LCB3aXRoVG9rZW4pO1xuICB9LFxuXG4gIHBvc3QocGF0aCwgZGF0YSwgd2l0aFRva2VuKXtcbiAgICByZXR1cm4gYXBpLmFqYXgoe3VybDogcGF0aCwgZGF0YTogSlNPTi5zdHJpbmdpZnkoZGF0YSksIHR5cGU6ICdQT1NUJ30sIHdpdGhUb2tlbik7XG4gIH0sXG5cbiAgZ2V0KHBhdGgpe1xuICAgIHJldHVybiBhcGkuYWpheCh7dXJsOiBwYXRofSk7XG4gIH0sXG5cbiAgYWpheChjZmcsIHdpdGhUb2tlbiA9IHRydWUpe1xuICAgIHZhciBkZWZhdWx0Q2ZnID0ge1xuICAgICAgdHlwZTogXCJHRVRcIixcbiAgICAgIGRhdGFUeXBlOiBcImpzb25cIixcbiAgICAgIGJlZm9yZVNlbmQ6IGZ1bmN0aW9uKHhocikge1xuICAgICAgICBpZih3aXRoVG9rZW4pe1xuICAgICAgICAgIHZhciB7IHRva2VuIH0gPSBzZXNzaW9uLmdldFVzZXJEYXRhKCk7XG4gICAgICAgICAgeGhyLnNldFJlcXVlc3RIZWFkZXIoJ0F1dGhvcml6YXRpb24nLCdCZWFyZXIgJyArIHRva2VuKTtcbiAgICAgICAgfVxuICAgICAgIH1cbiAgICB9XG5cbiAgICByZXR1cm4gJC5hamF4KCQuZXh0ZW5kKHt9LCBkZWZhdWx0Q2ZnLCBjZmcpKTtcbiAgfVxufVxuXG5tb2R1bGUuZXhwb3J0cyA9IGFwaTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9zZXJ2aWNlcy9hcGkuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciB7IGJyb3dzZXJIaXN0b3J5LCBjcmVhdGVNZW1vcnlIaXN0b3J5IH0gPSByZXF1aXJlKCdyZWFjdC1yb3V0ZXInKTtcblxuY29uc3QgbG9nZ2VyID0gcmVxdWlyZSgnYXBwL2NvbW1vbi9sb2dnZXInKS5jcmVhdGUoJ3NlcnZpY2VzL3Nlc3Npb25zJyk7XG5jb25zdCBBVVRIX0tFWV9EQVRBID0gJ2F1dGhEYXRhJztcblxudmFyIF9oaXN0b3J5ID0gY3JlYXRlTWVtb3J5SGlzdG9yeSgpO1xuXG52YXIgc2Vzc2lvbiA9IHtcblxuICBpbml0KGhpc3Rvcnk9YnJvd3Nlckhpc3Rvcnkpe1xuICAgIF9oaXN0b3J5ID0gaGlzdG9yeTtcbiAgfSxcblxuICBnZXRIaXN0b3J5KCl7XG4gICAgcmV0dXJuIF9oaXN0b3J5O1xuICB9LFxuXG4gIHNldFVzZXJEYXRhKHVzZXJEYXRhKXtcbiAgICBsb2NhbFN0b3JhZ2Uuc2V0SXRlbShBVVRIX0tFWV9EQVRBLCBKU09OLnN0cmluZ2lmeSh1c2VyRGF0YSkpO1xuICB9LFxuXG4gIGdldFVzZXJEYXRhKCl7XG4gICAgdmFyIGl0ZW0gPSBsb2NhbFN0b3JhZ2UuZ2V0SXRlbShBVVRIX0tFWV9EQVRBKTtcbiAgICBpZihpdGVtKXtcbiAgICAgIHJldHVybiBKU09OLnBhcnNlKGl0ZW0pO1xuICAgIH1cblxuICAgIC8vIGZvciBzc28gdXNlLWNhc2VzLCB0cnkgdG8gZ3JhYiB0aGUgdG9rZW4gZnJvbSBIVE1MXG4gICAgdmFyIGhpZGRlbkRpdiA9IGRvY3VtZW50LmdldEVsZW1lbnRCeUlkKFwiYmVhcmVyX3Rva2VuXCIpO1xuICAgIGlmKGhpZGRlbkRpdiAhPT0gbnVsbCApe1xuICAgICAgdHJ5e1xuICAgICAgICBsZXQganNvbiA9IHdpbmRvdy5hdG9iKGhpZGRlbkRpdi50ZXh0Q29udGVudCk7XG4gICAgICAgIGxldCB1c2VyRGF0YSA9IEpTT04ucGFyc2UoanNvbik7XG4gICAgICAgIGlmKHVzZXJEYXRhLnRva2VuKXtcbiAgICAgICAgICAvLyBwdXQgaXQgaW50byB0aGUgc2Vzc2lvblxuICAgICAgICAgIHRoaXMuc2V0VXNlckRhdGEodXNlckRhdGEpO1xuICAgICAgICAgIC8vIHJlbW92ZSB0aGUgZWxlbWVudFxuICAgICAgICAgIGhpZGRlbkRpdi5yZW1vdmUoKTtcbiAgICAgICAgICByZXR1cm4gdXNlckRhdGE7XG4gICAgICAgIH1cbiAgICAgIH1jYXRjaChlcnIpe1xuICAgICAgICBsb2dnZXIuZXJyb3IoJ2Vycm9yIHBhcnNpbmcgU1NPIHRva2VuOicsIGVycik7XG4gICAgICB9XG4gICAgfVxuXG4gICAgcmV0dXJuIHt9O1xuICB9LFxuXG4gIGNsZWFyKCl7XG4gICAgbG9jYWxTdG9yYWdlLmNsZWFyKClcbiAgfVxuXG59XG5cbm1vZHVsZS5leHBvcnRzID0gc2Vzc2lvbjtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9zZXJ2aWNlcy9zZXNzaW9uLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMgPSBfO1xuXG5cbi8qKioqKioqKioqKioqKioqKlxuICoqIFdFQlBBQ0sgRk9PVEVSXG4gKiogZXh0ZXJuYWwgXCJfXCJcbiAqKiBtb2R1bGUgaWQgPSA0M1xuICoqIG1vZHVsZSBjaHVua3MgPSAxXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIGxvZ29TdmcgPSByZXF1aXJlKCdhc3NldHMvaW1nL3N2Zy9ncnYtdGxwdC1sb2dvLWZ1bGwuc3ZnJyk7XG52YXIgY2xhc3NuYW1lcyA9IHJlcXVpcmUoJ2NsYXNzbmFtZXMnKTtcblxuY29uc3QgVGVsZXBvcnRMb2dvID0gKCkgPT4gKFxuICA8c3ZnIGNsYXNzTmFtZT1cImdydi1pY29uLWxvZ28tdGxwdFwiPjx1c2UgeGxpbmtIcmVmPXtsb2dvU3ZnfS8+PC9zdmc+XG4pXG5cbmNvbnN0IFVzZXJJY29uID0gKHtuYW1lPScnLCBpc0Rhcmt9KT0+e1xuICBsZXQgaWNvbkNsYXNzID0gY2xhc3NuYW1lcygnZ3J2LWljb24tdXNlcicsIHtcbiAgICAnLS1kYXJrJyA6IGlzRGFya1xuICB9KTtcblxuICByZXR1cm4gKFxuICAgIDxkaXYgdGl0bGU9e25hbWV9IGNsYXNzTmFtZT17aWNvbkNsYXNzfT5cbiAgICAgIDxzcGFuPlxuICAgICAgICA8c3Ryb25nPntuYW1lWzBdfTwvc3Ryb25nPlxuICAgICAgPC9zcGFuPlxuICAgIDwvZGl2PlxuICApXG59O1xuXG5leHBvcnQge1RlbGVwb3J0TG9nbywgVXNlckljb259XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9pY29ucy5qc3hcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG5cbmNvbnN0IE1TR19JTkZPX0xPR0lOX1NVQ0NFU1MgPSAnTG9naW4gd2FzIHN1Y2Nlc3NmdWwsIHlvdSBjYW4gY2xvc2UgdGhpcyB3aW5kb3cgYW5kIGNvbnRpbnVlIHVzaW5nIHRzaC4nO1xuY29uc3QgTVNHX0VSUk9SX0xPR0lOX0ZBSUxFRCA9ICdMb2dpbiB1bnN1Y2Nlc3NmdWwuIFBsZWFzZSB0cnkgYWdhaW4sIGlmIHRoZSBwcm9ibGVtIHBlcnNpc3RzLCBjb250YWN0IHlvdXIgc3lzdGVtIGFkbWluaXN0YXRvci4nO1xuY29uc3QgTVNHX0VSUk9SX0RFRkFVTFQgPSAnV2hvb3BzLCBzb21ldGhpbmcgd2VudCB3cm9uZy4nO1xuXG5jb25zdCBNU0dfRVJST1JfTk9UX0ZPVU5EID0gJ1dob29wcywgd2UgY2Fubm90IGZpbmQgdGhhdC4nO1xuY29uc3QgTVNHX0VSUk9SX05PVF9GT1VORF9ERVRBSUxTID0gYExvb2tzIGxpa2UgdGhlIHBhZ2UgeW91IGFyZSBsb29raW5nIGZvciBpc24ndCBoZXJlIGFueSBsb25nZXIuYDtcblxuY29uc3QgTVNHX0VSUk9SX0VYUElSRURfSU5WSVRFID0gJ0ludml0ZSBjb2RlIGhhcyBleHBpcmVkLic7XG5jb25zdCBNU0dfRVJST1JfRVhQSVJFRF9JTlZJVEVfREVUQUlMUyA9IGBMb29rcyBsaWtlIHlvdXIgaW52aXRlIGNvZGUgaXNuJ3QgdmFsaWQgYW55bW9yZS5gO1xuXG5jb25zdCBNc2dUeXBlID0ge1xuICBJTkZPOiAnaW5mbycsXG4gIEVSUk9SOiAnZXJyb3InXG59XG5cbmNvbnN0IEVycm9yVHlwZXMgPSB7XG4gIEZBSUxFRF9UT19MT0dJTjogJ2xvZ2luX2ZhaWxlZCcsXG4gIEVYUElSRURfSU5WSVRFOiAnZXhwaXJlZF9pbnZpdGUnLFxuICBOT1RfRk9VTkQ6ICdub3RfZm91bmQnXG59O1xuXG5jb25zdCBJbmZvVHlwZXMgPSB7XG4gIExPR0lOX1NVQ0NFU1M6ICdsb2dpbl9zdWNjZXNzJ1xufTtcblxudmFyIE1lc3NhZ2VQYWdlID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICByZW5kZXIoKXtcbiAgICBsZXQge3R5cGUsIHN1YlR5cGV9ID0gdGhpcy5wcm9wcy5wYXJhbXM7XG4gICAgaWYodHlwZSA9PT0gTXNnVHlwZS5FUlJPUil7XG4gICAgICByZXR1cm4gPEVycm9yUGFnZSB0eXBlPXtzdWJUeXBlfS8+XG4gICAgfVxuXG4gICAgaWYodHlwZSA9PT0gTXNnVHlwZS5JTkZPKXtcbiAgICAgIHJldHVybiA8SW5mb1BhZ2UgdHlwZT17c3ViVHlwZX0vPlxuICAgIH1cblxuICAgIHJldHVybiBudWxsO1xuICB9XG59KTtcblxudmFyIEVycm9yUGFnZSA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgcmVuZGVyKCkge1xuICAgIGxldCB7dHlwZX0gPSB0aGlzLnByb3BzO1xuICAgIGxldCBtc2dCb2R5ID0gKFxuICAgICAgPGRpdj5cbiAgICAgICAgPGgxPntNU0dfRVJST1JfREVGQVVMVH08L2gxPlxuICAgICAgPC9kaXY+XG4gICAgKTtcblxuICAgIGlmKHR5cGUgPT09IEVycm9yVHlwZXMuRkFJTEVEX1RPX0xPR0lOKXtcbiAgICAgIG1zZ0JvZHkgPSAoXG4gICAgICAgIDxkaXY+XG4gICAgICAgICAgPGgxPntNU0dfRVJST1JfTE9HSU5fRkFJTEVEfTwvaDE+XG4gICAgICAgIDwvZGl2PlxuICAgICAgKVxuICAgIH1cblxuICAgIGlmKHR5cGUgPT09IEVycm9yVHlwZXMuRVhQSVJFRF9JTlZJVEUpe1xuICAgICAgbXNnQm9keSA9IChcbiAgICAgICAgPGRpdj5cbiAgICAgICAgICA8aDE+e01TR19FUlJPUl9FWFBJUkVEX0lOVklURX08L2gxPlxuICAgICAgICAgIDxkaXY+e01TR19FUlJPUl9FWFBJUkVEX0lOVklURV9ERVRBSUxTfTwvZGl2PlxuICAgICAgICA8L2Rpdj5cbiAgICAgIClcbiAgICB9XG5cbiAgICBpZiggdHlwZSA9PT0gRXJyb3JUeXBlcy5OT1RfRk9VTkQpe1xuICAgICAgbXNnQm9keSA9IChcbiAgICAgICAgPGRpdj5cbiAgICAgICAgICA8aDE+e01TR19FUlJPUl9OT1RfRk9VTkR9PC9oMT5cbiAgICAgICAgICA8ZGl2PntNU0dfRVJST1JfTk9UX0ZPVU5EX0RFVEFJTFN9PC9kaXY+XG4gICAgICAgIDwvZGl2PlxuICAgICAgKTtcbiAgICB9XG5cbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbXNnLXBhZ2VcIj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtaGVhZGVyXCI+PGkgY2xhc3NOYW1lPVwiZmEgZmEtZnJvd24tb1wiPjwvaT4gPC9kaXY+XG4gICAgICAgIHttc2dCb2R5fVxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImNvbnRhY3Qtc2VjdGlvblwiPklmIHlvdSBiZWxpZXZlIHRoaXMgaXMgYW4gaXNzdWUgd2l0aCBUZWxlcG9ydCwgcGxlYXNlIDxhIGhyZWY9XCJodHRwczovL2dpdGh1Yi5jb20vZ3Jhdml0YXRpb25hbC90ZWxlcG9ydC9pc3N1ZXMvbmV3XCI+Y3JlYXRlIGEgR2l0SHViIGlzc3VlLjwvYT48L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pXG5cbnZhciBJbmZvUGFnZSA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgcmVuZGVyKCkge1xuICAgIGxldCB7dHlwZX0gPSB0aGlzLnByb3BzO1xuICAgIGxldCBtc2dCb2R5ID0gbnVsbDtcblxuICAgIGlmKHR5cGUgPT09IEluZm9UeXBlcy5MT0dJTl9TVUNDRVNTKXtcbiAgICAgIG1zZ0JvZHkgPSAoXG4gICAgICAgIDxkaXY+XG4gICAgICAgICAgPGgxPntNU0dfSU5GT19MT0dJTl9TVUNDRVNTfTwvaDE+XG4gICAgICAgIDwvZGl2PlxuICAgICAgKTtcbiAgICB9XG5cbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbXNnLXBhZ2VcIj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtaGVhZGVyXCI+PGkgY2xhc3NOYW1lPVwiZmEgZmEtc21pbGUtb1wiPjwvaT4gPC9kaXY+XG4gICAgICAgIHttc2dCb2R5fVxuICAgICAgPC9kaXY+XG4gICAgKTtcbiAgfVxufSlcblxudmFyIE5vdEZvdW5kID0gKCkgPT4gKFxuICA8RXJyb3JQYWdlIHR5cGU9e0Vycm9yVHlwZXMuTk9UX0ZPVU5EfS8+XG4pXG5cbmV4cG9ydCB7RXJyb3JQYWdlLCBJbmZvUGFnZSwgTm90Rm91bmQsIEVycm9yVHlwZXMsIE1lc3NhZ2VQYWdlfTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL21zZ1BhZ2UuanN4XG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xuXG5jb25zdCBHcnZUYWJsZVRleHRDZWxsID0gKHtyb3dJbmRleCwgZGF0YSwgY29sdW1uS2V5LCAuLi5wcm9wc30pID0+IChcbiAgPEdydlRhYmxlQ2VsbCB7Li4ucHJvcHN9PlxuICAgIHtkYXRhW3Jvd0luZGV4XVtjb2x1bW5LZXldfVxuICA8L0dydlRhYmxlQ2VsbD5cbik7XG5cbi8qKlxuKiBTb3J0IGluZGljYXRvciB1c2VkIGJ5IFNvcnRIZWFkZXJDZWxsXG4qL1xuY29uc3QgU29ydFR5cGVzID0ge1xuICBBU0M6ICdBU0MnLFxuICBERVNDOiAnREVTQydcbn07XG5cbmNvbnN0IFNvcnRJbmRpY2F0b3IgPSAoe3NvcnREaXJ9KT0+e1xuICBsZXQgY2xzID0gJ2dydi10YWJsZS1pbmRpY2F0b3Itc29ydCBmYSBmYS1zb3J0J1xuICBpZihzb3J0RGlyID09PSBTb3J0VHlwZXMuREVTQyl7XG4gICAgY2xzICs9ICctZGVzYydcbiAgfVxuXG4gIGlmKCBzb3J0RGlyID09PSBTb3J0VHlwZXMuQVNDKXtcbiAgICBjbHMgKz0gJy1hc2MnXG4gIH1cblxuICByZXR1cm4gKDxpIGNsYXNzTmFtZT17Y2xzfT48L2k+KTtcbn07XG5cbi8qKlxuKiBTb3J0IEhlYWRlciBDZWxsXG4qL1xudmFyIFNvcnRIZWFkZXJDZWxsID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICByZW5kZXIoKSB7XG4gICAgdmFyIHtzb3J0RGlyLCB0aXRsZSwgLi4ucHJvcHN9ID0gdGhpcy5wcm9wcztcblxuICAgIHJldHVybiAoXG4gICAgICA8R3J2VGFibGVDZWxsIHsuLi5wcm9wc30+XG4gICAgICAgIDxhIG9uQ2xpY2s9e3RoaXMub25Tb3J0Q2hhbmdlfT5cbiAgICAgICAgICB7dGl0bGV9XG4gICAgICAgIDwvYT5cbiAgICAgICAgPFNvcnRJbmRpY2F0b3Igc29ydERpcj17c29ydERpcn0vPlxuICAgICAgPC9HcnZUYWJsZUNlbGw+XG4gICAgKTtcbiAgfSxcblxuICBvblNvcnRDaGFuZ2UoZSkge1xuICAgIGUucHJldmVudERlZmF1bHQoKTtcbiAgICBpZih0aGlzLnByb3BzLm9uU29ydENoYW5nZSkge1xuICAgICAgLy8gZGVmYXVsdFxuICAgICAgbGV0IG5ld0RpciA9IFNvcnRUeXBlcy5ERVNDO1xuICAgICAgaWYodGhpcy5wcm9wcy5zb3J0RGlyKXtcbiAgICAgICAgbmV3RGlyID0gdGhpcy5wcm9wcy5zb3J0RGlyID09PSBTb3J0VHlwZXMuREVTQyA/IFNvcnRUeXBlcy5BU0MgOiBTb3J0VHlwZXMuREVTQztcbiAgICAgIH1cbiAgICAgIHRoaXMucHJvcHMub25Tb3J0Q2hhbmdlKHRoaXMucHJvcHMuY29sdW1uS2V5LCBuZXdEaXIpO1xuICAgIH1cbiAgfVxufSk7XG5cbi8qKlxuKiBEZWZhdWx0IENlbGxcbiovXG52YXIgR3J2VGFibGVDZWxsID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICByZW5kZXIoKXtcbiAgICB2YXIgcHJvcHMgPSB0aGlzLnByb3BzO1xuICAgIHJldHVybiBwcm9wcy5pc0hlYWRlciA/IDx0aCBrZXk9e3Byb3BzLmtleX0gY2xhc3NOYW1lPVwiZ3J2LXRhYmxlLWNlbGxcIj57cHJvcHMuY2hpbGRyZW59PC90aD4gOiA8dGQga2V5PXtwcm9wcy5rZXl9Pntwcm9wcy5jaGlsZHJlbn08L3RkPjtcbiAgfVxufSk7XG5cbi8qKlxuKiBUYWJsZVxuKi9cbnZhciBHcnZUYWJsZSA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICByZW5kZXJIZWFkZXIoY2hpbGRyZW4pe1xuICAgIHZhciBjZWxscyA9IGNoaWxkcmVuLm1hcCgoaXRlbSwgaW5kZXgpPT57XG4gICAgICByZXR1cm4gdGhpcy5yZW5kZXJDZWxsKGl0ZW0ucHJvcHMuaGVhZGVyLCB7aW5kZXgsIGtleTogaW5kZXgsIGlzSGVhZGVyOiB0cnVlLCAuLi5pdGVtLnByb3BzfSk7XG4gICAgfSlcblxuICAgIHJldHVybiA8dGhlYWQgY2xhc3NOYW1lPVwiZ3J2LXRhYmxlLWhlYWRlclwiPjx0cj57Y2VsbHN9PC90cj48L3RoZWFkPlxuICB9LFxuXG4gIHJlbmRlckJvZHkoY2hpbGRyZW4pe1xuICAgIHZhciBjb3VudCA9IHRoaXMucHJvcHMucm93Q291bnQ7XG4gICAgdmFyIHJvd3MgPSBbXTtcbiAgICBmb3IodmFyIGkgPSAwOyBpIDwgY291bnQ7IGkgKyspe1xuICAgICAgdmFyIGNlbGxzID0gY2hpbGRyZW4ubWFwKChpdGVtLCBpbmRleCk9PntcbiAgICAgICAgcmV0dXJuIHRoaXMucmVuZGVyQ2VsbChpdGVtLnByb3BzLmNlbGwsIHtyb3dJbmRleDogaSwga2V5OiBpbmRleCwgaXNIZWFkZXI6IGZhbHNlLCAuLi5pdGVtLnByb3BzfSk7XG4gICAgICB9KVxuXG4gICAgICByb3dzLnB1c2goPHRyIGtleT17aX0+e2NlbGxzfTwvdHI+KTtcbiAgICB9XG5cbiAgICByZXR1cm4gPHRib2R5Pntyb3dzfTwvdGJvZHk+O1xuICB9LFxuXG4gIHJlbmRlckNlbGwoY2VsbCwgY2VsbFByb3BzKXtcbiAgICB2YXIgY29udGVudCA9IG51bGw7XG4gICAgaWYgKFJlYWN0LmlzVmFsaWRFbGVtZW50KGNlbGwpKSB7XG4gICAgICAgY29udGVudCA9IFJlYWN0LmNsb25lRWxlbWVudChjZWxsLCBjZWxsUHJvcHMpO1xuICAgICB9IGVsc2UgaWYgKHR5cGVvZiBjZWxsID09PSAnZnVuY3Rpb24nKSB7XG4gICAgICAgY29udGVudCA9IGNlbGwoY2VsbFByb3BzKTtcbiAgICAgfVxuXG4gICAgIHJldHVybiBjb250ZW50O1xuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICB2YXIgY2hpbGRyZW4gPSBbXTtcbiAgICBSZWFjdC5DaGlsZHJlbi5mb3JFYWNoKHRoaXMucHJvcHMuY2hpbGRyZW4sIChjaGlsZCkgPT4ge1xuICAgICAgaWYgKGNoaWxkID09IG51bGwpIHtcbiAgICAgICAgcmV0dXJuO1xuICAgICAgfVxuXG4gICAgICBpZihjaGlsZC50eXBlLmRpc3BsYXlOYW1lICE9PSAnR3J2VGFibGVDb2x1bW4nKXtcbiAgICAgICAgdGhyb3cgJ1Nob3VsZCBiZSBHcnZUYWJsZUNvbHVtbic7XG4gICAgICB9XG5cbiAgICAgIGNoaWxkcmVuLnB1c2goY2hpbGQpO1xuICAgIH0pO1xuXG4gICAgdmFyIHRhYmxlQ2xhc3MgPSAndGFibGUgZ3J2LXRhYmxlICcgKyB0aGlzLnByb3BzLmNsYXNzTmFtZTtcblxuICAgIHJldHVybiAoXG4gICAgICA8dGFibGUgY2xhc3NOYW1lPXt0YWJsZUNsYXNzfT5cbiAgICAgICAge3RoaXMucmVuZGVySGVhZGVyKGNoaWxkcmVuKX1cbiAgICAgICAge3RoaXMucmVuZGVyQm9keShjaGlsZHJlbil9XG4gICAgICA8L3RhYmxlPlxuICAgICk7XG4gIH1cbn0pXG5cbnZhciBHcnZUYWJsZUNvbHVtbiA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICB0aHJvdyBuZXcgRXJyb3IoJ0NvbXBvbmVudCA8R3J2VGFibGVDb2x1bW4gLz4gc2hvdWxkIG5ldmVyIHJlbmRlcicpO1xuICB9XG59KVxuXG5jb25zdCBFbXB0eUluZGljYXRvciA9ICh7dGV4dH0pID0+IChcbiAgPGRpdiBjbGFzc05hbWU9XCJncnYtdGFibGUtaW5kaWNhdG9yLWVtcHR5IHRleHQtY2VudGVyIHRleHQtbXV0ZWRcIj48c3Bhbj57dGV4dH08L3NwYW4+PC9kaXY+XG4pXG5cbmV4cG9ydCBkZWZhdWx0IEdydlRhYmxlO1xuZXhwb3J0IHtcbiAgR3J2VGFibGVDb2x1bW4gYXMgQ29sdW1uLFxuICBHcnZUYWJsZSBhcyBUYWJsZSxcbiAgR3J2VGFibGVDZWxsIGFzIENlbGwsXG4gIEdydlRhYmxlVGV4dENlbGwgYXMgVGV4dENlbGwsXG4gIFNvcnRIZWFkZXJDZWxsLFxuICBTb3J0SW5kaWNhdG9yLFxuICBTb3J0VHlwZXMsXG4gIEVtcHR5SW5kaWNhdG9yfTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL3RhYmxlLmpzeFxuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxuY29uc3Qgbm9kZUhvc3ROYW1lQnlTZXJ2ZXJJZCA9IChzZXJ2ZXJJZCkgPT4gWyBbJ3RscHRfbm9kZXMnXSwgKG5vZGVzKSA9PntcbiAgbGV0IHNlcnZlciA9IG5vZGVzLmZpbmQoaXRlbT0+IGl0ZW0uZ2V0KCdpZCcpID09PSBzZXJ2ZXJJZCk7XG4gIHJldHVybiAhc2VydmVyID8gJycgOiBzZXJ2ZXIuZ2V0KCdob3N0bmFtZScpO1xufV07XG5cbmNvbnN0IG5vZGVMaXN0VmlldyA9IFsgWyd0bHB0X25vZGVzJ10sIChub2RlcykgPT57XG4gICAgcmV0dXJuIG5vZGVzLm1hcCgoaXRlbSk9PntcbiAgICAgIHZhciBzZXJ2ZXJJZCA9IGl0ZW0uZ2V0KCdpZCcpO1xuICAgICAgcmV0dXJuIHtcbiAgICAgICAgaWQ6IHNlcnZlcklkLFxuICAgICAgICBob3N0bmFtZTogaXRlbS5nZXQoJ2hvc3RuYW1lJyksXG4gICAgICAgIHRhZ3M6IGdldFRhZ3MoaXRlbSksXG4gICAgICAgIGFkZHI6IGl0ZW0uZ2V0KCdhZGRyJylcbiAgICAgIH1cbiAgICB9KS50b0pTKCk7XG4gfVxuXTtcblxuZnVuY3Rpb24gZ2V0VGFncyhub2RlKXtcbiAgdmFyIGFsbExhYmVscyA9IFtdO1xuICB2YXIgbGFiZWxzID0gbm9kZS5nZXQoJ2xhYmVscycpO1xuXG4gIGlmKGxhYmVscyl7XG4gICAgbGFiZWxzLmVudHJ5U2VxKCkudG9BcnJheSgpLmZvckVhY2goaXRlbT0+e1xuICAgICAgYWxsTGFiZWxzLnB1c2goe1xuICAgICAgICByb2xlOiBpdGVtWzBdLFxuICAgICAgICB2YWx1ZTogaXRlbVsxXVxuICAgICAgfSk7XG4gICAgfSk7XG4gIH1cblxuICBsYWJlbHMgPSBub2RlLmdldCgnY21kX2xhYmVscycpO1xuXG4gIGlmKGxhYmVscyl7XG4gICAgbGFiZWxzLmVudHJ5U2VxKCkudG9BcnJheSgpLmZvckVhY2goaXRlbT0+e1xuICAgICAgYWxsTGFiZWxzLnB1c2goe1xuICAgICAgICByb2xlOiBpdGVtWzBdLFxuICAgICAgICB2YWx1ZTogaXRlbVsxXS5nZXQoJ3Jlc3VsdCcpLFxuICAgICAgICB0b29sdGlwOiBpdGVtWzFdLmdldCgnY29tbWFuZCcpXG4gICAgICB9KTtcbiAgICB9KTtcbiAgfVxuXG4gIHJldHVybiBhbGxMYWJlbHM7XG59XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgbm9kZUxpc3RWaWV3LFxuICBub2RlSG9zdE5hbWVCeVNlcnZlcklkXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9nZXR0ZXJzLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgeyBUTFBUX05PVElGSUNBVElPTlNfQUREIH0gID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCB7XG5cbiAgc2hvd0Vycm9yKHRleHQsIHRpdGxlPSdFUlJPUicpe1xuICAgIGRpc3BhdGNoKHtpc0Vycm9yOiB0cnVlLCB0ZXh0OiB0ZXh0LCB0aXRsZX0pO1xuICB9LFxuXG4gIHNob3dTdWNjZXNzKHRleHQsIHRpdGxlPSdTVUNDRVNTJyl7XG4gICAgZGlzcGF0Y2goe2lzU3VjY2Vzczp0cnVlLCB0ZXh0OiB0ZXh0LCB0aXRsZX0pO1xuICB9LFxuXG4gIHNob3dJbmZvKHRleHQsIHRpdGxlPSdJTkZPJyl7XG4gICAgZGlzcGF0Y2goe2lzSW5mbzp0cnVlLCB0ZXh0OiB0ZXh0LCB0aXRsZX0pO1xuICB9LFxuXG4gIHNob3dXYXJuaW5nKHRleHQsIHRpdGxlPSdXQVJOSU5HJyl7XG4gICAgZGlzcGF0Y2goe2lzV2FybmluZzogdHJ1ZSwgdGV4dDogdGV4dCwgdGl0bGV9KTtcbiAgfVxuXG59XG5cbmZ1bmN0aW9uIGRpc3BhdGNoKG1zZyl7XG4gIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9OT1RJRklDQVRJT05TX0FERCwgbXNnKTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL25vdGlmaWNhdGlvbnMvYWN0aW9ucy5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIHtjcmVhdGVWaWV3fSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3Nlc3Npb25zL2dldHRlcnMnKTtcblxuY29uc3QgY3VycmVudFNlc3Npb24gPSBbIFsndGxwdF9jdXJyZW50X3Nlc3Npb24nXSwgWyd0bHB0X3Nlc3Npb25zJ10sXG4oY3VycmVudCwgc2Vzc2lvbnMpID0+IHtcbiAgICBpZighY3VycmVudCl7XG4gICAgICByZXR1cm4gbnVsbDtcbiAgICB9XG5cbiAgICAvKlxuICAgICogYWN0aXZlIHNlc3Npb24gbmVlZHMgdG8gaGF2ZSBpdHMgb3duIHZpZXcgYXMgYW4gYWN0dWFsIHNlc3Npb24gbWlnaHQgbm90XG4gICAgKiBleGlzdCBhdCB0aGlzIHBvaW50LiBGb3IgZXhhbXBsZSwgdXBvbiBjcmVhdGluZyBhIG5ldyBzZXNzaW9uIHdlIG5lZWQgdG8ga25vd1xuICAgICogbG9naW4gYW5kIHNlcnZlcklkLiBJdCB3aWxsIGJlIHNpbXBsaWZpZWQgb25jZSBzZXJ2ZXIgQVBJIGdldHMgZXh0ZW5kZWQuXG4gICAgKi9cbiAgICBsZXQgY3VyU2Vzc2lvblZpZXcgPSB7XG4gICAgICBpc05ld1Nlc3Npb246IGN1cnJlbnQuZ2V0KCdpc05ld1Nlc3Npb24nKSxcbiAgICAgIG5vdEZvdW5kOiBjdXJyZW50LmdldCgnbm90Rm91bmQnKSxcbiAgICAgIGFkZHI6IGN1cnJlbnQuZ2V0KCdhZGRyJyksXG4gICAgICBzZXJ2ZXJJZDogY3VycmVudC5nZXQoJ3NlcnZlcklkJyksXG4gICAgICBzZXJ2ZXJJcDogdW5kZWZpbmVkLFxuICAgICAgbG9naW46IGN1cnJlbnQuZ2V0KCdsb2dpbicpLFxuICAgICAgc2lkOiBjdXJyZW50LmdldCgnc2lkJyksXG4gICAgICBjb2xzOiB1bmRlZmluZWQsXG4gICAgICByb3dzOiB1bmRlZmluZWRcbiAgICB9O1xuXG4gICAgLypcbiAgICAqIGluIGNhc2UgaWYgc2Vzc2lvbiBhbHJlYWR5IGV4aXN0cywgZ2V0IGl0cyB2aWV3IGRhdGEgKGZvciBleGFtcGxlLCB3aGVuIGpvaW5pbmcgYW4gZXhpc3Rpbmcgc2Vzc2lvbilcbiAgICAqL1xuICAgIGlmKHNlc3Npb25zLmhhcyhjdXJTZXNzaW9uVmlldy5zaWQpKXtcbiAgICAgIGxldCBleGlzdGluZyA9IGNyZWF0ZVZpZXcoc2Vzc2lvbnMuZ2V0KGN1clNlc3Npb25WaWV3LnNpZCkpO1xuXG4gICAgICBjdXJTZXNzaW9uVmlldy5wYXJ0aWVzID0gZXhpc3RpbmcucGFydGllcztcbiAgICAgIGN1clNlc3Npb25WaWV3LnNlcnZlcklwID0gZXhpc3Rpbmcuc2VydmVySXA7XG4gICAgICBjdXJTZXNzaW9uVmlldy5zZXJ2ZXJJZCA9IGV4aXN0aW5nLnNlcnZlcklkO1xuICAgICAgY3VyU2Vzc2lvblZpZXcuYWN0aXZlID0gZXhpc3RpbmcuYWN0aXZlO1xuICAgICAgY3VyU2Vzc2lvblZpZXcuY29scyA9IGV4aXN0aW5nLmNvbHM7XG4gICAgICBjdXJTZXNzaW9uVmlldy5yb3dzID0gZXhpc3Rpbmcucm93cztcbiAgICB9XG5cbiAgICByZXR1cm4gY3VyU2Vzc2lvblZpZXc7XG4gIH1cbl07XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgY3VycmVudFNlc3Npb25cbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2N1cnJlbnRTZXNzaW9uL2dldHRlcnMuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbmltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX1NFU1NJTlNfUkVDRUlWRTogbnVsbCxcbiAgVExQVF9TRVNTSU5TX1VQREFURTogbnVsbCxcbiAgVExQVF9TRVNTSU5TX1JFTU9WRV9TVE9SRUQ6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9zZXNzaW9ucy9hY3Rpb25UeXBlcy5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIGFwaSA9IHJlcXVpcmUoJ2FwcC9zZXJ2aWNlcy9hcGknKTtcbnZhciBhcGlVdGlscyA9IHJlcXVpcmUoJ2FwcC9zZXJ2aWNlcy9hcGlVdGlscycpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcbnZhciB7c2hvd0Vycm9yfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL25vdGlmaWNhdGlvbnMvYWN0aW9ucycpO1xuXG5jb25zdCBsb2dnZXIgPSByZXF1aXJlKCdhcHAvY29tbW9uL2xvZ2dlcicpLmNyZWF0ZSgnTW9kdWxlcy9TZXNzaW9ucycpO1xuY29uc3QgeyBUTFBUX1NFU1NJTlNfUkVDRUlWRSwgVExQVF9TRVNTSU5TX1VQREFURSB9ICA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuY29uc3QgYWN0aW9ucyA9IHtcblxuICBmZXRjaFNlc3Npb24oc2lkKXtcbiAgICByZXR1cm4gYXBpLmdldChjZmcuYXBpLmdldEZldGNoU2Vzc2lvblVybChzaWQpKS50aGVuKGpzb249PntcbiAgICAgIGlmKGpzb24gJiYganNvbi5zZXNzaW9uKXtcbiAgICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1NFU1NJTlNfVVBEQVRFLCBqc29uLnNlc3Npb24pO1xuICAgICAgfVxuICAgIH0pO1xuICB9LFxuXG4gIGZldGNoU2Vzc2lvbnMoe2VuZCwgc2lkLCBsaW1pdD1jZmcubWF4U2Vzc2lvbkxvYWRTaXplfT17fSl7XG4gICAgbGV0IHN0YXJ0ID0gZW5kIHx8IG5ldyBEYXRlKCk7XG4gICAgbGV0IHBhcmFtcyA9IHtcbiAgICAgIG9yZGVyOiAtMSxcbiAgICAgIGxpbWl0LFxuICAgICAgc3RhcnQsXG4gICAgICBzaWRcbiAgICB9O1xuXG4gICAgcmV0dXJuIGFwaVV0aWxzLmZpbHRlclNlc3Npb25zKHBhcmFtcylcbiAgICAgIC5kb25lKChqc29uKSA9PiB7XG4gICAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9TRVNTSU5TX1JFQ0VJVkUsIGpzb24uc2Vzc2lvbnMpO1xuICAgICAgfSlcbiAgICAgIC5mYWlsKChlcnIpPT57XG4gICAgICAgIHNob3dFcnJvcignVW5hYmxlIHRvIHJldHJpZXZlIGxpc3Qgb2Ygc2Vzc2lvbnMnKTtcbiAgICAgICAgbG9nZ2VyLmVycm9yKCdmZXRjaFNlc3Npb25zJywgZXJyKTtcbiAgICAgIH0pO1xuICB9LFxuXG4gIHVwZGF0ZVNlc3Npb24oanNvbil7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1NFU1NJTlNfVVBEQVRFLCBqc29uKTtcbiAgfVxufVxuXG5leHBvcnQgZGVmYXVsdCBhY3Rpb25zO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvYWN0aW9ucy5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIHsgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG5cbmNvbnN0IHNlc3Npb25zQnlTZXJ2ZXIgPSAoc2VydmVySWQpID0+IFtbJ3RscHRfc2Vzc2lvbnMnXSwgKHNlc3Npb25zKSA9PntcbiAgcmV0dXJuIHNlc3Npb25zLnZhbHVlU2VxKCkuZmlsdGVyKGl0ZW09PntcbiAgICB2YXIgcGFydGllcyA9IGl0ZW0uZ2V0KCdwYXJ0aWVzJykgfHwgdG9JbW11dGFibGUoW10pO1xuICAgIHZhciBoYXNTZXJ2ZXIgPSBwYXJ0aWVzLmZpbmQoaXRlbTI9PiBpdGVtMi5nZXQoJ3NlcnZlcl9pZCcpID09PSBzZXJ2ZXJJZCk7XG4gICAgcmV0dXJuIGhhc1NlcnZlcjtcbiAgfSkudG9MaXN0KCk7XG59XVxuXG5jb25zdCBzZXNzaW9uc1ZpZXcgPSBbWyd0bHB0X3Nlc3Npb25zJ10sIChzZXNzaW9ucykgPT57XG4gIHJldHVybiBzZXNzaW9ucy52YWx1ZVNlcSgpLm1hcChjcmVhdGVWaWV3KS50b0pTKCk7XG59XTtcblxuY29uc3Qgc2Vzc2lvblZpZXdCeUlkID0gKHNpZCk9PiBbWyd0bHB0X3Nlc3Npb25zJywgc2lkXSwgKHNlc3Npb24pPT57XG4gIGlmKCFzZXNzaW9uKXtcbiAgICByZXR1cm4gbnVsbDtcbiAgfVxuXG4gIHJldHVybiBjcmVhdGVWaWV3KHNlc3Npb24pO1xufV07XG5cbmNvbnN0IHBhcnRpZXNCeVNlc3Npb25JZCA9IChzaWQpID0+XG4gW1sndGxwdF9zZXNzaW9ucycsIHNpZCwgJ3BhcnRpZXMnXSwgKHBhcnRpZXMpID0+e1xuXG4gIGlmKCFwYXJ0aWVzKXtcbiAgICByZXR1cm4gW107XG4gIH1cblxuICB2YXIgbGFzdEFjdGl2ZVVzck5hbWUgPSBnZXRMYXN0QWN0aXZlVXNlcihwYXJ0aWVzKS5nZXQoJ3VzZXInKTtcblxuICByZXR1cm4gcGFydGllcy5tYXAoaXRlbT0+e1xuICAgIHZhciB1c2VyID0gaXRlbS5nZXQoJ3VzZXInKTtcbiAgICByZXR1cm4ge1xuICAgICAgdXNlcjogaXRlbS5nZXQoJ3VzZXInKSxcbiAgICAgIHNlcnZlcklwOiBpdGVtLmdldCgncmVtb3RlX2FkZHInKSxcbiAgICAgIHNlcnZlcklkOiBpdGVtLmdldCgnc2VydmVyX2lkJyksXG4gICAgICBpc0FjdGl2ZTogbGFzdEFjdGl2ZVVzck5hbWUgPT09IHVzZXJcbiAgICB9XG4gIH0pLnRvSlMoKTtcbn1dO1xuXG5mdW5jdGlvbiBnZXRMYXN0QWN0aXZlVXNlcihwYXJ0aWVzKXtcbiAgcmV0dXJuIHBhcnRpZXMuc29ydEJ5KGl0ZW09PiBuZXcgRGF0ZShpdGVtLmdldCgnbGFzdEFjdGl2ZScpKSkubGFzdCgpO1xufVxuXG5mdW5jdGlvbiBjcmVhdGVWaWV3KHNlc3Npb24pe1xuICB2YXIgc2lkID0gc2Vzc2lvbi5nZXQoJ2lkJyk7XG4gIHZhciBzZXJ2ZXJJcCwgc2VydmVySWQ7XG4gIHZhciBwYXJ0aWVzID0gcmVhY3Rvci5ldmFsdWF0ZShwYXJ0aWVzQnlTZXNzaW9uSWQoc2lkKSk7XG5cbiAgaWYocGFydGllcy5sZW5ndGggPiAwKXtcbiAgICBzZXJ2ZXJJcCA9IHBhcnRpZXNbMF0uc2VydmVySXA7XG4gICAgc2VydmVySWQgPSBwYXJ0aWVzWzBdLnNlcnZlcklkO1xuICB9XG5cbiAgcmV0dXJuIHtcbiAgICBzaWQ6IHNpZCxcbiAgICBzZXNzaW9uVXJsOiBjZmcuZ2V0QWN0aXZlU2Vzc2lvblJvdXRlVXJsKHNpZCksXG4gICAgc2VydmVySXAsXG4gICAgc2VydmVySWQsXG4gICAgYWN0aXZlOiBzZXNzaW9uLmdldCgnYWN0aXZlJyksXG4gICAgY3JlYXRlZDogc2Vzc2lvbi5nZXQoJ2NyZWF0ZWQnKSxcbiAgICBsYXN0QWN0aXZlOiBzZXNzaW9uLmdldCgnbGFzdF9hY3RpdmUnKSxcbiAgICBsb2dpbjogc2Vzc2lvbi5nZXQoJ2xvZ2luJyksXG4gICAgcGFydGllczogcGFydGllcyxcbiAgICBjb2xzOiBzZXNzaW9uLmdldEluKFsndGVybWluYWxfcGFyYW1zJywgJ3cnXSksXG4gICAgcm93czogc2Vzc2lvbi5nZXRJbihbJ3Rlcm1pbmFsX3BhcmFtcycsICdoJ10pXG4gIH1cbn1cblxuZXhwb3J0IGRlZmF1bHQge1xuICBwYXJ0aWVzQnlTZXNzaW9uSWQsXG4gIHNlc3Npb25zQnlTZXJ2ZXIsXG4gIHNlc3Npb25zVmlldyxcbiAgc2Vzc2lvblZpZXdCeUlkLFxuICBjcmVhdGVWaWV3LFxuICBjb3VudDogW1sndGxwdF9zZXNzaW9ucyddLCBzZXNzaW9ucyA9PiBzZXNzaW9ucy5zaXplIF1cbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2dldHRlcnMuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbmNvbnN0IGZpbHRlciA9IFtbJ3RscHRfc3RvcmVkX3Nlc3Npb25zX2ZpbHRlciddLCAoZmlsdGVyKSA9PntcbiAgcmV0dXJuIGZpbHRlci50b0pTKCk7XG59XTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICBmaWx0ZXJcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3N0b3JlZFNlc3Npb25zRmlsdGVyL2dldHRlcnMuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbmltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX1JFQ0VJVkVfVVNFUjogbnVsbCxcbiAgVExQVF9SRUNFSVZFX1VTRVJfSU5WSVRFOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvdXNlci9hY3Rpb25UeXBlcy5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIHtUUllJTkdfVE9fTE9HSU4sIFRSWUlOR19UT19TSUdOX1VQLCBGRVRDSElOR19JTlZJVEV9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvcmVzdEFwaS9jb25zdGFudHMnKTtcbnZhciB7cmVxdWVzdFN0YXR1c30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9yZXN0QXBpL2dldHRlcnMnKTtcblxuY29uc3QgaW52aXRlID0gWyBbJ3RscHRfdXNlcl9pbnZpdGUnXSwgKGludml0ZSkgPT4gaW52aXRlIF07XG5cbmNvbnN0IHVzZXIgPSBbIFsndGxwdF91c2VyJ10sIChjdXJyZW50VXNlcikgPT4ge1xuICAgIGlmKCFjdXJyZW50VXNlcil7XG4gICAgICByZXR1cm4gbnVsbDtcbiAgICB9XG5cbiAgICB2YXIgbmFtZSA9IGN1cnJlbnRVc2VyLmdldCgnbmFtZScpIHx8ICcnO1xuICAgIHZhciBzaG9ydERpc3BsYXlOYW1lID0gbmFtZVswXSB8fCAnJztcblxuICAgIHJldHVybiB7XG4gICAgICBuYW1lLFxuICAgICAgc2hvcnREaXNwbGF5TmFtZSxcbiAgICAgIGxvZ2luczogY3VycmVudFVzZXIuZ2V0KCdhbGxvd2VkX2xvZ2lucycpLnRvSlMoKVxuICAgIH1cbiAgfVxuXTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICB1c2VyLFxuICBpbnZpdGUsXG4gIGxvZ2luQXR0ZW1wOiByZXF1ZXN0U3RhdHVzKFRSWUlOR19UT19MT0dJTiksXG4gIGF0dGVtcDogcmVxdWVzdFN0YXR1cyhUUllJTkdfVE9fU0lHTl9VUCksXG4gIGZldGNoaW5nSW52aXRlOiByZXF1ZXN0U3RhdHVzKEZFVENISU5HX0lOVklURSlcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvZ2V0dGVycy5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIGFwaSA9IHJlcXVpcmUoJy4vYXBpJyk7XG52YXIgc2Vzc2lvbiA9IHJlcXVpcmUoJy4vc2Vzc2lvbicpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcbnZhciAkID0gcmVxdWlyZSgnalF1ZXJ5Jyk7XG5cbmNvbnN0IFBST1ZJREVSX0dPT0dMRSA9ICdnb29nbGUnO1xuXG5jb25zdCByZWZyZXNoUmF0ZSA9IDYwMDAwICogNTsgLy8gNSBtaW5cblxudmFyIHJlZnJlc2hUb2tlblRpbWVySWQgPSBudWxsO1xuXG52YXIgYXV0aCA9IHtcblxuICBzaWduVXAobmFtZSwgcGFzc3dvcmQsIHRva2VuLCBpbnZpdGVUb2tlbil7XG4gICAgdmFyIGRhdGEgPSB7dXNlcjogbmFtZSwgcGFzczogcGFzc3dvcmQsIHNlY29uZF9mYWN0b3JfdG9rZW46IHRva2VuLCBpbnZpdGVfdG9rZW46IGludml0ZVRva2VufTtcbiAgICByZXR1cm4gYXBpLnBvc3QoY2ZnLmFwaS5jcmVhdGVVc2VyUGF0aCwgZGF0YSlcbiAgICAgIC50aGVuKCh1c2VyKT0+e1xuICAgICAgICBzZXNzaW9uLnNldFVzZXJEYXRhKHVzZXIpO1xuICAgICAgICBhdXRoLl9zdGFydFRva2VuUmVmcmVzaGVyKCk7XG4gICAgICAgIHJldHVybiB1c2VyO1xuICAgICAgfSk7XG4gIH0sXG5cbiAgbG9naW4obmFtZSwgcGFzc3dvcmQsIHRva2VuKXtcbiAgICBhdXRoLl9zdG9wVG9rZW5SZWZyZXNoZXIoKTtcbiAgICBzZXNzaW9uLmNsZWFyKCk7XG4gICAgcmV0dXJuIGF1dGguX2xvZ2luKG5hbWUsIHBhc3N3b3JkLCB0b2tlbikuZG9uZShhdXRoLl9zdGFydFRva2VuUmVmcmVzaGVyKTtcbiAgfSxcblxuICBlbnN1cmVVc2VyKCl7XG4gICAgdmFyIHVzZXJEYXRhID0gc2Vzc2lvbi5nZXRVc2VyRGF0YSgpO1xuICAgIGlmKHVzZXJEYXRhLnRva2VuKXtcbiAgICAgIC8vIHJlZnJlc2ggdGltZXIgd2lsbCBub3QgYmUgc2V0IGluIGNhc2Ugb2YgYnJvd3NlciByZWZyZXNoIGV2ZW50XG4gICAgICBpZihhdXRoLl9nZXRSZWZyZXNoVG9rZW5UaW1lcklkKCkgPT09IG51bGwpe1xuICAgICAgICByZXR1cm4gYXV0aC5fcmVmcmVzaFRva2VuKCkuZG9uZShhdXRoLl9zdGFydFRva2VuUmVmcmVzaGVyKTtcbiAgICAgIH1cblxuICAgICAgcmV0dXJuICQuRGVmZXJyZWQoKS5yZXNvbHZlKHVzZXJEYXRhKTtcbiAgICB9XG5cbiAgICByZXR1cm4gJC5EZWZlcnJlZCgpLnJlamVjdCgpO1xuICB9LFxuXG4gIGxvZ291dCgpe1xuICAgIGF1dGguX3N0b3BUb2tlblJlZnJlc2hlcigpO1xuICAgIHNlc3Npb24uY2xlYXIoKTtcbiAgICBhdXRoLl9yZWRpcmVjdCgpO1xuICB9LFxuXG4gIF9yZWRpcmVjdCgpe1xuICAgIHdpbmRvdy5sb2NhdGlvbiA9IGNmZy5yb3V0ZXMubG9naW47XG4gIH0sXG5cbiAgX3N0YXJ0VG9rZW5SZWZyZXNoZXIoKXtcbiAgICByZWZyZXNoVG9rZW5UaW1lcklkID0gc2V0SW50ZXJ2YWwoYXV0aC5fcmVmcmVzaFRva2VuLCByZWZyZXNoUmF0ZSk7XG4gIH0sXG5cbiAgX3N0b3BUb2tlblJlZnJlc2hlcigpe1xuICAgIGNsZWFySW50ZXJ2YWwocmVmcmVzaFRva2VuVGltZXJJZCk7XG4gICAgcmVmcmVzaFRva2VuVGltZXJJZCA9IG51bGw7XG4gIH0sXG5cbiAgX2dldFJlZnJlc2hUb2tlblRpbWVySWQoKXtcbiAgICByZXR1cm4gcmVmcmVzaFRva2VuVGltZXJJZDtcbiAgfSxcblxuICBfcmVmcmVzaFRva2VuKCl7XG4gICAgcmV0dXJuIGFwaS5wb3N0KGNmZy5hcGkucmVuZXdUb2tlblBhdGgpLnRoZW4oZGF0YT0+e1xuICAgICAgc2Vzc2lvbi5zZXRVc2VyRGF0YShkYXRhKTtcbiAgICAgIHJldHVybiBkYXRhO1xuICAgIH0pLmZhaWwoKCk9PntcbiAgICAgIGF1dGgubG9nb3V0KCk7XG4gICAgfSk7XG4gIH0sXG5cbiAgX2xvZ2luKG5hbWUsIHBhc3N3b3JkLCB0b2tlbil7XG4gICAgdmFyIGRhdGEgPSB7XG4gICAgICB1c2VyOiBuYW1lLFxuICAgICAgcGFzczogcGFzc3dvcmQsXG4gICAgICBzZWNvbmRfZmFjdG9yX3Rva2VuOiB0b2tlblxuICAgIH07XG5cbiAgICByZXR1cm4gYXBpLnBvc3QoY2ZnLmFwaS5zZXNzaW9uUGF0aCwgZGF0YSwgZmFsc2UpLnRoZW4oZGF0YT0+e1xuICAgICAgc2Vzc2lvbi5zZXRVc2VyRGF0YShkYXRhKTtcbiAgICAgIHJldHVybiBkYXRhO1xuICAgIH0pO1xuICB9XG59XG5cbm1vZHVsZS5leHBvcnRzID0gYXV0aDtcbm1vZHVsZS5leHBvcnRzLlBST1ZJREVSX0dPT0dMRSA9IFBST1ZJREVSX0dPT0dMRTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9zZXJ2aWNlcy9hdXRoLmpzXG4gKiovIiwiLy8gQ29weXJpZ2h0IEpveWVudCwgSW5jLiBhbmQgb3RoZXIgTm9kZSBjb250cmlidXRvcnMuXG4vL1xuLy8gUGVybWlzc2lvbiBpcyBoZXJlYnkgZ3JhbnRlZCwgZnJlZSBvZiBjaGFyZ2UsIHRvIGFueSBwZXJzb24gb2J0YWluaW5nIGFcbi8vIGNvcHkgb2YgdGhpcyBzb2Z0d2FyZSBhbmQgYXNzb2NpYXRlZCBkb2N1bWVudGF0aW9uIGZpbGVzICh0aGVcbi8vIFwiU29mdHdhcmVcIiksIHRvIGRlYWwgaW4gdGhlIFNvZnR3YXJlIHdpdGhvdXQgcmVzdHJpY3Rpb24sIGluY2x1ZGluZ1xuLy8gd2l0aG91dCBsaW1pdGF0aW9uIHRoZSByaWdodHMgdG8gdXNlLCBjb3B5LCBtb2RpZnksIG1lcmdlLCBwdWJsaXNoLFxuLy8gZGlzdHJpYnV0ZSwgc3VibGljZW5zZSwgYW5kL29yIHNlbGwgY29waWVzIG9mIHRoZSBTb2Z0d2FyZSwgYW5kIHRvIHBlcm1pdFxuLy8gcGVyc29ucyB0byB3aG9tIHRoZSBTb2Z0d2FyZSBpcyBmdXJuaXNoZWQgdG8gZG8gc28sIHN1YmplY3QgdG8gdGhlXG4vLyBmb2xsb3dpbmcgY29uZGl0aW9uczpcbi8vXG4vLyBUaGUgYWJvdmUgY29weXJpZ2h0IG5vdGljZSBhbmQgdGhpcyBwZXJtaXNzaW9uIG5vdGljZSBzaGFsbCBiZSBpbmNsdWRlZFxuLy8gaW4gYWxsIGNvcGllcyBvciBzdWJzdGFudGlhbCBwb3J0aW9ucyBvZiB0aGUgU29mdHdhcmUuXG4vL1xuLy8gVEhFIFNPRlRXQVJFIElTIFBST1ZJREVEIFwiQVMgSVNcIiwgV0lUSE9VVCBXQVJSQU5UWSBPRiBBTlkgS0lORCwgRVhQUkVTU1xuLy8gT1IgSU1QTElFRCwgSU5DTFVESU5HIEJVVCBOT1QgTElNSVRFRCBUTyBUSEUgV0FSUkFOVElFUyBPRlxuLy8gTUVSQ0hBTlRBQklMSVRZLCBGSVRORVNTIEZPUiBBIFBBUlRJQ1VMQVIgUFVSUE9TRSBBTkQgTk9OSU5GUklOR0VNRU5ULiBJTlxuLy8gTk8gRVZFTlQgU0hBTEwgVEhFIEFVVEhPUlMgT1IgQ09QWVJJR0hUIEhPTERFUlMgQkUgTElBQkxFIEZPUiBBTlkgQ0xBSU0sXG4vLyBEQU1BR0VTIE9SIE9USEVSIExJQUJJTElUWSwgV0hFVEhFUiBJTiBBTiBBQ1RJT04gT0YgQ09OVFJBQ1QsIFRPUlQgT1Jcbi8vIE9USEVSV0lTRSwgQVJJU0lORyBGUk9NLCBPVVQgT0YgT1IgSU4gQ09OTkVDVElPTiBXSVRIIFRIRSBTT0ZUV0FSRSBPUiBUSEVcbi8vIFVTRSBPUiBPVEhFUiBERUFMSU5HUyBJTiBUSEUgU09GVFdBUkUuXG5cbmZ1bmN0aW9uIEV2ZW50RW1pdHRlcigpIHtcbiAgdGhpcy5fZXZlbnRzID0gdGhpcy5fZXZlbnRzIHx8IHt9O1xuICB0aGlzLl9tYXhMaXN0ZW5lcnMgPSB0aGlzLl9tYXhMaXN0ZW5lcnMgfHwgdW5kZWZpbmVkO1xufVxubW9kdWxlLmV4cG9ydHMgPSBFdmVudEVtaXR0ZXI7XG5cbi8vIEJhY2t3YXJkcy1jb21wYXQgd2l0aCBub2RlIDAuMTAueFxuRXZlbnRFbWl0dGVyLkV2ZW50RW1pdHRlciA9IEV2ZW50RW1pdHRlcjtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5fZXZlbnRzID0gdW5kZWZpbmVkO1xuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5fbWF4TGlzdGVuZXJzID0gdW5kZWZpbmVkO1xuXG4vLyBCeSBkZWZhdWx0IEV2ZW50RW1pdHRlcnMgd2lsbCBwcmludCBhIHdhcm5pbmcgaWYgbW9yZSB0aGFuIDEwIGxpc3RlbmVycyBhcmVcbi8vIGFkZGVkIHRvIGl0LiBUaGlzIGlzIGEgdXNlZnVsIGRlZmF1bHQgd2hpY2ggaGVscHMgZmluZGluZyBtZW1vcnkgbGVha3MuXG5FdmVudEVtaXR0ZXIuZGVmYXVsdE1heExpc3RlbmVycyA9IDEwO1xuXG4vLyBPYnZpb3VzbHkgbm90IGFsbCBFbWl0dGVycyBzaG91bGQgYmUgbGltaXRlZCB0byAxMC4gVGhpcyBmdW5jdGlvbiBhbGxvd3Ncbi8vIHRoYXQgdG8gYmUgaW5jcmVhc2VkLiBTZXQgdG8gemVybyBmb3IgdW5saW1pdGVkLlxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5zZXRNYXhMaXN0ZW5lcnMgPSBmdW5jdGlvbihuKSB7XG4gIGlmICghaXNOdW1iZXIobikgfHwgbiA8IDAgfHwgaXNOYU4obikpXG4gICAgdGhyb3cgVHlwZUVycm9yKCduIG11c3QgYmUgYSBwb3NpdGl2ZSBudW1iZXInKTtcbiAgdGhpcy5fbWF4TGlzdGVuZXJzID0gbjtcbiAgcmV0dXJuIHRoaXM7XG59O1xuXG5FdmVudEVtaXR0ZXIucHJvdG90eXBlLmVtaXQgPSBmdW5jdGlvbih0eXBlKSB7XG4gIHZhciBlciwgaGFuZGxlciwgbGVuLCBhcmdzLCBpLCBsaXN0ZW5lcnM7XG5cbiAgaWYgKCF0aGlzLl9ldmVudHMpXG4gICAgdGhpcy5fZXZlbnRzID0ge307XG5cbiAgLy8gSWYgdGhlcmUgaXMgbm8gJ2Vycm9yJyBldmVudCBsaXN0ZW5lciB0aGVuIHRocm93LlxuICBpZiAodHlwZSA9PT0gJ2Vycm9yJykge1xuICAgIGlmICghdGhpcy5fZXZlbnRzLmVycm9yIHx8XG4gICAgICAgIChpc09iamVjdCh0aGlzLl9ldmVudHMuZXJyb3IpICYmICF0aGlzLl9ldmVudHMuZXJyb3IubGVuZ3RoKSkge1xuICAgICAgZXIgPSBhcmd1bWVudHNbMV07XG4gICAgICBpZiAoZXIgaW5zdGFuY2VvZiBFcnJvcikge1xuICAgICAgICB0aHJvdyBlcjsgLy8gVW5oYW5kbGVkICdlcnJvcicgZXZlbnRcbiAgICAgIH1cbiAgICAgIHRocm93IFR5cGVFcnJvcignVW5jYXVnaHQsIHVuc3BlY2lmaWVkIFwiZXJyb3JcIiBldmVudC4nKTtcbiAgICB9XG4gIH1cblxuICBoYW5kbGVyID0gdGhpcy5fZXZlbnRzW3R5cGVdO1xuXG4gIGlmIChpc1VuZGVmaW5lZChoYW5kbGVyKSlcbiAgICByZXR1cm4gZmFsc2U7XG5cbiAgaWYgKGlzRnVuY3Rpb24oaGFuZGxlcikpIHtcbiAgICBzd2l0Y2ggKGFyZ3VtZW50cy5sZW5ndGgpIHtcbiAgICAgIC8vIGZhc3QgY2FzZXNcbiAgICAgIGNhc2UgMTpcbiAgICAgICAgaGFuZGxlci5jYWxsKHRoaXMpO1xuICAgICAgICBicmVhaztcbiAgICAgIGNhc2UgMjpcbiAgICAgICAgaGFuZGxlci5jYWxsKHRoaXMsIGFyZ3VtZW50c1sxXSk7XG4gICAgICAgIGJyZWFrO1xuICAgICAgY2FzZSAzOlxuICAgICAgICBoYW5kbGVyLmNhbGwodGhpcywgYXJndW1lbnRzWzFdLCBhcmd1bWVudHNbMl0pO1xuICAgICAgICBicmVhaztcbiAgICAgIC8vIHNsb3dlclxuICAgICAgZGVmYXVsdDpcbiAgICAgICAgbGVuID0gYXJndW1lbnRzLmxlbmd0aDtcbiAgICAgICAgYXJncyA9IG5ldyBBcnJheShsZW4gLSAxKTtcbiAgICAgICAgZm9yIChpID0gMTsgaSA8IGxlbjsgaSsrKVxuICAgICAgICAgIGFyZ3NbaSAtIDFdID0gYXJndW1lbnRzW2ldO1xuICAgICAgICBoYW5kbGVyLmFwcGx5KHRoaXMsIGFyZ3MpO1xuICAgIH1cbiAgfSBlbHNlIGlmIChpc09iamVjdChoYW5kbGVyKSkge1xuICAgIGxlbiA9IGFyZ3VtZW50cy5sZW5ndGg7XG4gICAgYXJncyA9IG5ldyBBcnJheShsZW4gLSAxKTtcbiAgICBmb3IgKGkgPSAxOyBpIDwgbGVuOyBpKyspXG4gICAgICBhcmdzW2kgLSAxXSA9IGFyZ3VtZW50c1tpXTtcblxuICAgIGxpc3RlbmVycyA9IGhhbmRsZXIuc2xpY2UoKTtcbiAgICBsZW4gPSBsaXN0ZW5lcnMubGVuZ3RoO1xuICAgIGZvciAoaSA9IDA7IGkgPCBsZW47IGkrKylcbiAgICAgIGxpc3RlbmVyc1tpXS5hcHBseSh0aGlzLCBhcmdzKTtcbiAgfVxuXG4gIHJldHVybiB0cnVlO1xufTtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5hZGRMaXN0ZW5lciA9IGZ1bmN0aW9uKHR5cGUsIGxpc3RlbmVyKSB7XG4gIHZhciBtO1xuXG4gIGlmICghaXNGdW5jdGlvbihsaXN0ZW5lcikpXG4gICAgdGhyb3cgVHlwZUVycm9yKCdsaXN0ZW5lciBtdXN0IGJlIGEgZnVuY3Rpb24nKTtcblxuICBpZiAoIXRoaXMuX2V2ZW50cylcbiAgICB0aGlzLl9ldmVudHMgPSB7fTtcblxuICAvLyBUbyBhdm9pZCByZWN1cnNpb24gaW4gdGhlIGNhc2UgdGhhdCB0eXBlID09PSBcIm5ld0xpc3RlbmVyXCIhIEJlZm9yZVxuICAvLyBhZGRpbmcgaXQgdG8gdGhlIGxpc3RlbmVycywgZmlyc3QgZW1pdCBcIm5ld0xpc3RlbmVyXCIuXG4gIGlmICh0aGlzLl9ldmVudHMubmV3TGlzdGVuZXIpXG4gICAgdGhpcy5lbWl0KCduZXdMaXN0ZW5lcicsIHR5cGUsXG4gICAgICAgICAgICAgIGlzRnVuY3Rpb24obGlzdGVuZXIubGlzdGVuZXIpID9cbiAgICAgICAgICAgICAgbGlzdGVuZXIubGlzdGVuZXIgOiBsaXN0ZW5lcik7XG5cbiAgaWYgKCF0aGlzLl9ldmVudHNbdHlwZV0pXG4gICAgLy8gT3B0aW1pemUgdGhlIGNhc2Ugb2Ygb25lIGxpc3RlbmVyLiBEb24ndCBuZWVkIHRoZSBleHRyYSBhcnJheSBvYmplY3QuXG4gICAgdGhpcy5fZXZlbnRzW3R5cGVdID0gbGlzdGVuZXI7XG4gIGVsc2UgaWYgKGlzT2JqZWN0KHRoaXMuX2V2ZW50c1t0eXBlXSkpXG4gICAgLy8gSWYgd2UndmUgYWxyZWFkeSBnb3QgYW4gYXJyYXksIGp1c3QgYXBwZW5kLlxuICAgIHRoaXMuX2V2ZW50c1t0eXBlXS5wdXNoKGxpc3RlbmVyKTtcbiAgZWxzZVxuICAgIC8vIEFkZGluZyB0aGUgc2Vjb25kIGVsZW1lbnQsIG5lZWQgdG8gY2hhbmdlIHRvIGFycmF5LlxuICAgIHRoaXMuX2V2ZW50c1t0eXBlXSA9IFt0aGlzLl9ldmVudHNbdHlwZV0sIGxpc3RlbmVyXTtcblxuICAvLyBDaGVjayBmb3IgbGlzdGVuZXIgbGVha1xuICBpZiAoaXNPYmplY3QodGhpcy5fZXZlbnRzW3R5cGVdKSAmJiAhdGhpcy5fZXZlbnRzW3R5cGVdLndhcm5lZCkge1xuICAgIHZhciBtO1xuICAgIGlmICghaXNVbmRlZmluZWQodGhpcy5fbWF4TGlzdGVuZXJzKSkge1xuICAgICAgbSA9IHRoaXMuX21heExpc3RlbmVycztcbiAgICB9IGVsc2Uge1xuICAgICAgbSA9IEV2ZW50RW1pdHRlci5kZWZhdWx0TWF4TGlzdGVuZXJzO1xuICAgIH1cblxuICAgIGlmIChtICYmIG0gPiAwICYmIHRoaXMuX2V2ZW50c1t0eXBlXS5sZW5ndGggPiBtKSB7XG4gICAgICB0aGlzLl9ldmVudHNbdHlwZV0ud2FybmVkID0gdHJ1ZTtcbiAgICAgIGNvbnNvbGUuZXJyb3IoJyhub2RlKSB3YXJuaW5nOiBwb3NzaWJsZSBFdmVudEVtaXR0ZXIgbWVtb3J5ICcgK1xuICAgICAgICAgICAgICAgICAgICAnbGVhayBkZXRlY3RlZC4gJWQgbGlzdGVuZXJzIGFkZGVkLiAnICtcbiAgICAgICAgICAgICAgICAgICAgJ1VzZSBlbWl0dGVyLnNldE1heExpc3RlbmVycygpIHRvIGluY3JlYXNlIGxpbWl0LicsXG4gICAgICAgICAgICAgICAgICAgIHRoaXMuX2V2ZW50c1t0eXBlXS5sZW5ndGgpO1xuICAgICAgaWYgKHR5cGVvZiBjb25zb2xlLnRyYWNlID09PSAnZnVuY3Rpb24nKSB7XG4gICAgICAgIC8vIG5vdCBzdXBwb3J0ZWQgaW4gSUUgMTBcbiAgICAgICAgY29uc29sZS50cmFjZSgpO1xuICAgICAgfVxuICAgIH1cbiAgfVxuXG4gIHJldHVybiB0aGlzO1xufTtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5vbiA9IEV2ZW50RW1pdHRlci5wcm90b3R5cGUuYWRkTGlzdGVuZXI7XG5cbkV2ZW50RW1pdHRlci5wcm90b3R5cGUub25jZSA9IGZ1bmN0aW9uKHR5cGUsIGxpc3RlbmVyKSB7XG4gIGlmICghaXNGdW5jdGlvbihsaXN0ZW5lcikpXG4gICAgdGhyb3cgVHlwZUVycm9yKCdsaXN0ZW5lciBtdXN0IGJlIGEgZnVuY3Rpb24nKTtcblxuICB2YXIgZmlyZWQgPSBmYWxzZTtcblxuICBmdW5jdGlvbiBnKCkge1xuICAgIHRoaXMucmVtb3ZlTGlzdGVuZXIodHlwZSwgZyk7XG5cbiAgICBpZiAoIWZpcmVkKSB7XG4gICAgICBmaXJlZCA9IHRydWU7XG4gICAgICBsaXN0ZW5lci5hcHBseSh0aGlzLCBhcmd1bWVudHMpO1xuICAgIH1cbiAgfVxuXG4gIGcubGlzdGVuZXIgPSBsaXN0ZW5lcjtcbiAgdGhpcy5vbih0eXBlLCBnKTtcblxuICByZXR1cm4gdGhpcztcbn07XG5cbi8vIGVtaXRzIGEgJ3JlbW92ZUxpc3RlbmVyJyBldmVudCBpZmYgdGhlIGxpc3RlbmVyIHdhcyByZW1vdmVkXG5FdmVudEVtaXR0ZXIucHJvdG90eXBlLnJlbW92ZUxpc3RlbmVyID0gZnVuY3Rpb24odHlwZSwgbGlzdGVuZXIpIHtcbiAgdmFyIGxpc3QsIHBvc2l0aW9uLCBsZW5ndGgsIGk7XG5cbiAgaWYgKCFpc0Z1bmN0aW9uKGxpc3RlbmVyKSlcbiAgICB0aHJvdyBUeXBlRXJyb3IoJ2xpc3RlbmVyIG11c3QgYmUgYSBmdW5jdGlvbicpO1xuXG4gIGlmICghdGhpcy5fZXZlbnRzIHx8ICF0aGlzLl9ldmVudHNbdHlwZV0pXG4gICAgcmV0dXJuIHRoaXM7XG5cbiAgbGlzdCA9IHRoaXMuX2V2ZW50c1t0eXBlXTtcbiAgbGVuZ3RoID0gbGlzdC5sZW5ndGg7XG4gIHBvc2l0aW9uID0gLTE7XG5cbiAgaWYgKGxpc3QgPT09IGxpc3RlbmVyIHx8XG4gICAgICAoaXNGdW5jdGlvbihsaXN0Lmxpc3RlbmVyKSAmJiBsaXN0Lmxpc3RlbmVyID09PSBsaXN0ZW5lcikpIHtcbiAgICBkZWxldGUgdGhpcy5fZXZlbnRzW3R5cGVdO1xuICAgIGlmICh0aGlzLl9ldmVudHMucmVtb3ZlTGlzdGVuZXIpXG4gICAgICB0aGlzLmVtaXQoJ3JlbW92ZUxpc3RlbmVyJywgdHlwZSwgbGlzdGVuZXIpO1xuXG4gIH0gZWxzZSBpZiAoaXNPYmplY3QobGlzdCkpIHtcbiAgICBmb3IgKGkgPSBsZW5ndGg7IGktLSA+IDA7KSB7XG4gICAgICBpZiAobGlzdFtpXSA9PT0gbGlzdGVuZXIgfHxcbiAgICAgICAgICAobGlzdFtpXS5saXN0ZW5lciAmJiBsaXN0W2ldLmxpc3RlbmVyID09PSBsaXN0ZW5lcikpIHtcbiAgICAgICAgcG9zaXRpb24gPSBpO1xuICAgICAgICBicmVhaztcbiAgICAgIH1cbiAgICB9XG5cbiAgICBpZiAocG9zaXRpb24gPCAwKVxuICAgICAgcmV0dXJuIHRoaXM7XG5cbiAgICBpZiAobGlzdC5sZW5ndGggPT09IDEpIHtcbiAgICAgIGxpc3QubGVuZ3RoID0gMDtcbiAgICAgIGRlbGV0ZSB0aGlzLl9ldmVudHNbdHlwZV07XG4gICAgfSBlbHNlIHtcbiAgICAgIGxpc3Quc3BsaWNlKHBvc2l0aW9uLCAxKTtcbiAgICB9XG5cbiAgICBpZiAodGhpcy5fZXZlbnRzLnJlbW92ZUxpc3RlbmVyKVxuICAgICAgdGhpcy5lbWl0KCdyZW1vdmVMaXN0ZW5lcicsIHR5cGUsIGxpc3RlbmVyKTtcbiAgfVxuXG4gIHJldHVybiB0aGlzO1xufTtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5yZW1vdmVBbGxMaXN0ZW5lcnMgPSBmdW5jdGlvbih0eXBlKSB7XG4gIHZhciBrZXksIGxpc3RlbmVycztcblxuICBpZiAoIXRoaXMuX2V2ZW50cylcbiAgICByZXR1cm4gdGhpcztcblxuICAvLyBub3QgbGlzdGVuaW5nIGZvciByZW1vdmVMaXN0ZW5lciwgbm8gbmVlZCB0byBlbWl0XG4gIGlmICghdGhpcy5fZXZlbnRzLnJlbW92ZUxpc3RlbmVyKSB7XG4gICAgaWYgKGFyZ3VtZW50cy5sZW5ndGggPT09IDApXG4gICAgICB0aGlzLl9ldmVudHMgPSB7fTtcbiAgICBlbHNlIGlmICh0aGlzLl9ldmVudHNbdHlwZV0pXG4gICAgICBkZWxldGUgdGhpcy5fZXZlbnRzW3R5cGVdO1xuICAgIHJldHVybiB0aGlzO1xuICB9XG5cbiAgLy8gZW1pdCByZW1vdmVMaXN0ZW5lciBmb3IgYWxsIGxpc3RlbmVycyBvbiBhbGwgZXZlbnRzXG4gIGlmIChhcmd1bWVudHMubGVuZ3RoID09PSAwKSB7XG4gICAgZm9yIChrZXkgaW4gdGhpcy5fZXZlbnRzKSB7XG4gICAgICBpZiAoa2V5ID09PSAncmVtb3ZlTGlzdGVuZXInKSBjb250aW51ZTtcbiAgICAgIHRoaXMucmVtb3ZlQWxsTGlzdGVuZXJzKGtleSk7XG4gICAgfVxuICAgIHRoaXMucmVtb3ZlQWxsTGlzdGVuZXJzKCdyZW1vdmVMaXN0ZW5lcicpO1xuICAgIHRoaXMuX2V2ZW50cyA9IHt9O1xuICAgIHJldHVybiB0aGlzO1xuICB9XG5cbiAgbGlzdGVuZXJzID0gdGhpcy5fZXZlbnRzW3R5cGVdO1xuXG4gIGlmIChpc0Z1bmN0aW9uKGxpc3RlbmVycykpIHtcbiAgICB0aGlzLnJlbW92ZUxpc3RlbmVyKHR5cGUsIGxpc3RlbmVycyk7XG4gIH0gZWxzZSB7XG4gICAgLy8gTElGTyBvcmRlclxuICAgIHdoaWxlIChsaXN0ZW5lcnMubGVuZ3RoKVxuICAgICAgdGhpcy5yZW1vdmVMaXN0ZW5lcih0eXBlLCBsaXN0ZW5lcnNbbGlzdGVuZXJzLmxlbmd0aCAtIDFdKTtcbiAgfVxuICBkZWxldGUgdGhpcy5fZXZlbnRzW3R5cGVdO1xuXG4gIHJldHVybiB0aGlzO1xufTtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5saXN0ZW5lcnMgPSBmdW5jdGlvbih0eXBlKSB7XG4gIHZhciByZXQ7XG4gIGlmICghdGhpcy5fZXZlbnRzIHx8ICF0aGlzLl9ldmVudHNbdHlwZV0pXG4gICAgcmV0ID0gW107XG4gIGVsc2UgaWYgKGlzRnVuY3Rpb24odGhpcy5fZXZlbnRzW3R5cGVdKSlcbiAgICByZXQgPSBbdGhpcy5fZXZlbnRzW3R5cGVdXTtcbiAgZWxzZVxuICAgIHJldCA9IHRoaXMuX2V2ZW50c1t0eXBlXS5zbGljZSgpO1xuICByZXR1cm4gcmV0O1xufTtcblxuRXZlbnRFbWl0dGVyLmxpc3RlbmVyQ291bnQgPSBmdW5jdGlvbihlbWl0dGVyLCB0eXBlKSB7XG4gIHZhciByZXQ7XG4gIGlmICghZW1pdHRlci5fZXZlbnRzIHx8ICFlbWl0dGVyLl9ldmVudHNbdHlwZV0pXG4gICAgcmV0ID0gMDtcbiAgZWxzZSBpZiAoaXNGdW5jdGlvbihlbWl0dGVyLl9ldmVudHNbdHlwZV0pKVxuICAgIHJldCA9IDE7XG4gIGVsc2VcbiAgICByZXQgPSBlbWl0dGVyLl9ldmVudHNbdHlwZV0ubGVuZ3RoO1xuICByZXR1cm4gcmV0O1xufTtcblxuZnVuY3Rpb24gaXNGdW5jdGlvbihhcmcpIHtcbiAgcmV0dXJuIHR5cGVvZiBhcmcgPT09ICdmdW5jdGlvbic7XG59XG5cbmZ1bmN0aW9uIGlzTnVtYmVyKGFyZykge1xuICByZXR1cm4gdHlwZW9mIGFyZyA9PT0gJ251bWJlcic7XG59XG5cbmZ1bmN0aW9uIGlzT2JqZWN0KGFyZykge1xuICByZXR1cm4gdHlwZW9mIGFyZyA9PT0gJ29iamVjdCcgJiYgYXJnICE9PSBudWxsO1xufVxuXG5mdW5jdGlvbiBpc1VuZGVmaW5lZChhcmcpIHtcbiAgcmV0dXJuIGFyZyA9PT0gdm9pZCAwO1xufVxuXG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiAuL34vZXZlbnRzL2V2ZW50cy5qc1xuICoqIG1vZHVsZSBpZCA9IDk5XG4gKiogbW9kdWxlIGNodW5rcyA9IDFcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbm1vZHVsZS5leHBvcnRzLmlzTWF0Y2ggPSBmdW5jdGlvbihvYmosIHNlYXJjaFZhbHVlLCB7c2VhcmNoYWJsZVByb3BzLCBjYn0pIHtcbiAgc2VhcmNoVmFsdWUgPSBzZWFyY2hWYWx1ZS50b0xvY2FsZVVwcGVyQ2FzZSgpO1xuICBsZXQgcHJvcE5hbWVzID0gc2VhcmNoYWJsZVByb3BzIHx8IE9iamVjdC5nZXRPd25Qcm9wZXJ0eU5hbWVzKG9iaik7XG4gIGZvciAobGV0IGkgPSAwOyBpIDwgcHJvcE5hbWVzLmxlbmd0aDsgaSsrKSB7XG4gICAgbGV0IHRhcmdldFZhbHVlID0gb2JqW3Byb3BOYW1lc1tpXV07XG4gICAgaWYgKHRhcmdldFZhbHVlKSB7XG4gICAgICBpZih0eXBlb2YgY2IgPT09ICdmdW5jdGlvbicpe1xuICAgICAgICBsZXQgcmVzdWx0ID0gY2IodGFyZ2V0VmFsdWUsIHNlYXJjaFZhbHVlLCBwcm9wTmFtZXNbaV0pO1xuICAgICAgICBpZihyZXN1bHQgPT09IHRydWUpe1xuICAgICAgICAgIHJldHVybiByZXN1bHQ7XG4gICAgICAgIH1cbiAgICAgIH1cblxuICAgICAgaWYgKHRhcmdldFZhbHVlLnRvU3RyaW5nKCkudG9Mb2NhbGVVcHBlckNhc2UoKS5pbmRleE9mKHNlYXJjaFZhbHVlKSAhPT0gLTEpIHtcbiAgICAgICAgcmV0dXJuIHRydWU7XG4gICAgICB9XG4gICAgfVxuICB9XG5cbiAgcmV0dXJuIGZhbHNlO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbW1vbi9vYmplY3RVdGlscy5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIFRlcm0gPSByZXF1aXJlKCdUZXJtaW5hbCcpO1xudmFyIFR0eSA9IHJlcXVpcmUoJy4vdHR5Jyk7XG52YXIgVHR5RXZlbnRzID0gcmVxdWlyZSgnLi90dHlFdmVudHMnKTtcbnZhciB7ZGVib3VuY2UsIGlzTnVtYmVyfSA9IHJlcXVpcmUoJ18nKTtcblxudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcbnZhciBhcGkgPSByZXF1aXJlKCdhcHAvc2VydmljZXMvYXBpJyk7XG52YXIgbG9nZ2VyID0gcmVxdWlyZSgnYXBwL2NvbW1vbi9sb2dnZXInKS5jcmVhdGUoJ3Rlcm1pbmFsJyk7XG52YXIgJCA9IHJlcXVpcmUoJ2pRdWVyeScpO1xuXG5UZXJtLmNvbG9yc1syNTZdID0gJyMyNTIzMjMnO1xuXG5jb25zdCBESVNDT05ORUNUX1RYVCA9ICdcXHgxYlszMW1kaXNjb25uZWN0ZWRcXHgxYlttXFxyXFxuJztcbmNvbnN0IENPTk5FQ1RFRF9UWFQgPSAnQ29ubmVjdGVkIVxcclxcbic7XG5jb25zdCBHUlZfQ0xBU1MgPSAnZ3J2LXRlcm1pbmFsJztcblxuY2xhc3MgVHR5VGVybWluYWwge1xuICBjb25zdHJ1Y3RvcihvcHRpb25zKXtcbiAgICBsZXQge1xuICAgICAgdHR5LFxuICAgICAgY29scyxcbiAgICAgIHJvd3MsXG4gICAgICBzY3JvbGxCYWNrID0gMTAwMCB9ID0gb3B0aW9ucztcblxuICAgIHRoaXMudHR5UGFyYW1zID0gdHR5O1xuICAgIHRoaXMudHR5ID0gbmV3IFR0eSgpO1xuICAgIHRoaXMudHR5RXZlbnRzID0gbmV3IFR0eUV2ZW50cygpO1xuXG4gICAgdGhpcy5zY3JvbGxCYWNrID0gc2Nyb2xsQmFja1xuICAgIHRoaXMucm93cyA9IHJvd3M7XG4gICAgdGhpcy5jb2xzID0gY29scztcbiAgICB0aGlzLnRlcm0gPSBudWxsO1xuICAgIHRoaXMuX2VsID0gb3B0aW9ucy5lbDtcblxuICAgIHRoaXMuZGVib3VuY2VkUmVzaXplID0gZGVib3VuY2UodGhpcy5fcmVxdWVzdFJlc2l6ZS5iaW5kKHRoaXMpLCAyMDApO1xuICB9XG5cbiAgb3BlbigpIHtcbiAgICAkKHRoaXMuX2VsKS5hZGRDbGFzcyhHUlZfQ0xBU1MpO1xuXG4gICAgdGhpcy50ZXJtID0gbmV3IFRlcm0oe1xuICAgICAgY29sczogMTUsXG4gICAgICByb3dzOiA1LFxuICAgICAgc2Nyb2xsYmFjazogdGhpcy5zY3JvbGxiYWNrLFxuICAgICAgdXNlU3R5bGU6IHRydWUsXG4gICAgICBzY3JlZW5LZXlzOiB0cnVlLFxuICAgICAgY3Vyc29yQmxpbms6IHRydWVcbiAgICB9KTtcblxuICAgIHRoaXMudGVybS5vcGVuKHRoaXMuX2VsKTtcblxuICAgIHRoaXMucmVzaXplKHRoaXMuY29scywgdGhpcy5yb3dzKTtcblxuICAgIC8vIHRlcm0gZXZlbnRzXG4gICAgdGhpcy50ZXJtLm9uKCdkYXRhJywgKGRhdGEpID0+IHRoaXMudHR5LnNlbmQoZGF0YSkpO1xuXG4gICAgLy8gdHR5XG4gICAgdGhpcy50dHkub24oJ3Jlc2l6ZScsICh7aCwgd30pPT4gdGhpcy5yZXNpemUodywgaCkpO1xuICAgIHRoaXMudHR5Lm9uKCdyZXNldCcsICgpPT4gdGhpcy50ZXJtLnJlc2V0KCkpO1xuICAgIHRoaXMudHR5Lm9uKCdvcGVuJywgKCk9PiB0aGlzLnRlcm0ud3JpdGUoQ09OTkVDVEVEX1RYVCkpO1xuICAgIHRoaXMudHR5Lm9uKCdjbG9zZScsICgpPT4gdGhpcy50ZXJtLndyaXRlKERJU0NPTk5FQ1RfVFhUKSk7XG4gICAgdGhpcy50dHkub24oJ2RhdGEnLCAoZGF0YSkgPT4ge1xuICAgICAgdHJ5e1xuICAgICAgICB0aGlzLnRlcm0ud3JpdGUoZGF0YSk7XG4gICAgICB9Y2F0Y2goZXJyKXtcbiAgICAgICAgY29uc29sZS5lcnJvcihlcnIpO1xuICAgICAgfVxuICAgIH0pO1xuXG4gICAgLy8gdHR5RXZlbnRzXG4gICAgdGhpcy50dHlFdmVudHMub24oJ2RhdGEnLCB0aGlzLl9oYW5kbGVUdHlFdmVudHNEYXRhLmJpbmQodGhpcykpO1xuICAgIHRoaXMuY29ubmVjdCgpO1xuICAgIHdpbmRvdy5hZGRFdmVudExpc3RlbmVyKCdyZXNpemUnLCB0aGlzLmRlYm91bmNlZFJlc2l6ZSk7XG4gIH1cblxuICBjb25uZWN0KCl7XG4gICAgdGhpcy50dHkuY29ubmVjdCh0aGlzLl9nZXRUdHlDb25uU3RyKCkpO1xuICAgIHRoaXMudHR5RXZlbnRzLmNvbm5lY3QodGhpcy5fZ2V0VHR5RXZlbnRzQ29ublN0cigpKTtcbiAgfVxuXG4gIGRlc3Ryb3koKSB7XG4gICAgaWYodGhpcy50dHkgIT09IG51bGwpe1xuICAgICAgdGhpcy50dHkuZGlzY29ubmVjdCgpO1xuICAgIH1cblxuICAgIGlmKHRoaXMudHR5RXZlbnRzICE9PSBudWxsKXtcbiAgICAgIHRoaXMudHR5RXZlbnRzLmRpc2Nvbm5lY3QoKTtcbiAgICAgIHRoaXMudHR5RXZlbnRzLnJlbW92ZUFsbExpc3RlbmVycygpO1xuICAgIH1cblxuICAgIGlmKHRoaXMudGVybSAhPT0gbnVsbCl7XG4gICAgICB0aGlzLnRlcm0uZGVzdHJveSgpO1xuICAgICAgdGhpcy50ZXJtLnJlbW92ZUFsbExpc3RlbmVycygpO1xuICAgIH1cblxuICAgICQodGhpcy5fZWwpLmVtcHR5KCkucmVtb3ZlQ2xhc3MoR1JWX0NMQVNTKTtcblxuICAgIHdpbmRvdy5yZW1vdmVFdmVudExpc3RlbmVyKCdyZXNpemUnLCB0aGlzLmRlYm91bmNlZFJlc2l6ZSk7XG4gIH1cblxuICByZXNpemUoY29scywgcm93cykge1xuICAgIC8vIGlmIG5vdCBkZWZpbmVkLCB1c2UgdGhlIHNpemUgb2YgdGhlIGNvbnRhaW5lclxuICAgIGlmKCFpc051bWJlcihjb2xzKSB8fCAhaXNOdW1iZXIocm93cykpe1xuICAgICAgbGV0IGRpbSA9IHRoaXMuX2dldERpbWVuc2lvbnMoKTtcbiAgICAgIGNvbHMgPSBkaW0uY29scztcbiAgICAgIHJvd3MgPSBkaW0ucm93cztcbiAgICB9XG5cbiAgICB0aGlzLmNvbHMgPSBjb2xzO1xuICAgIHRoaXMucm93cyA9IHJvd3M7XG4gICAgdGhpcy50ZXJtLnJlc2l6ZSh0aGlzLmNvbHMsIHRoaXMucm93cyk7XG4gIH1cblxuICBfcmVxdWVzdFJlc2l6ZSgpe1xuICAgIGxldCB7Y29scywgcm93c30gPSB0aGlzLl9nZXREaW1lbnNpb25zKCk7XG4gICAgbGV0IHcgPSBjb2xzO1xuICAgIGxldCBoID0gcm93cztcblxuICAgIC8vIHNvbWUgbWluIHZhbHVlc1xuICAgIHcgPSB3IDwgNSA/IDUgOiB3O1xuICAgIGggPSBoIDwgNSA/IDUgOiBoO1xuXG4gICAgbGV0IHtzaWQgfSA9IHRoaXMudHR5UGFyYW1zO1xuICAgIGxldCByZXFEYXRhID0geyB0ZXJtaW5hbF9wYXJhbXM6IHsgdywgaCB9IH07XG5cbiAgICBsb2dnZXIuaW5mbygncmVzaXplJywgYHc6JHt3fSBhbmQgaDoke2h9YCk7XG4gICAgYXBpLnB1dChjZmcuYXBpLmdldFRlcm1pbmFsU2Vzc2lvblVybChzaWQpLCByZXFEYXRhKVxuICAgICAgLmRvbmUoKCk9PiBsb2dnZXIuaW5mbygncmVzaXplZCcpKVxuICAgICAgLmZhaWwoKGVycik9PiBsb2dnZXIuZXJyb3IoJ2ZhaWxlZCB0byByZXNpemUnLCBlcnIpKTtcbiAgfVxuXG4gIF9oYW5kbGVUdHlFdmVudHNEYXRhKGRhdGEpe1xuICAgIGlmKGRhdGEgJiYgZGF0YS50ZXJtaW5hbF9wYXJhbXMpe1xuICAgICAgbGV0IHt3LCBofSA9IGRhdGEudGVybWluYWxfcGFyYW1zO1xuICAgICAgaWYoaCAhPT0gdGhpcy5yb3dzIHx8IHcgIT09IHRoaXMuY29scyl7XG4gICAgICAgIHRoaXMucmVzaXplKHcsIGgpO1xuICAgICAgfVxuICAgIH1cbiAgfVxuXG4gIF9nZXREaW1lbnNpb25zKCl7XG4gICAgbGV0ICRjb250YWluZXIgPSAkKHRoaXMuX2VsKTtcbiAgICBsZXQgZmFrZVJvdyA9ICQoJzxkaXY+PHNwYW4+Jm5ic3A7PC9zcGFuPjwvZGl2PicpO1xuXG4gICAgJGNvbnRhaW5lci5maW5kKCcudGVybWluYWwnKS5hcHBlbmQoZmFrZVJvdyk7XG4gICAgLy8gZ2V0IGRpdiBoZWlnaHRcbiAgICBsZXQgZmFrZUNvbEhlaWdodCA9IGZha2VSb3dbMF0uZ2V0Qm91bmRpbmdDbGllbnRSZWN0KCkuaGVpZ2h0O1xuICAgIC8vIGdldCBzcGFuIHdpZHRoXG4gICAgbGV0IGZha2VDb2xXaWR0aCA9IGZha2VSb3cuY2hpbGRyZW4oKS5maXJzdCgpWzBdLmdldEJvdW5kaW5nQ2xpZW50UmVjdCgpLndpZHRoO1xuXG4gICAgbGV0IHdpZHRoID0gJGNvbnRhaW5lclswXS5jbGllbnRXaWR0aDtcbiAgICBsZXQgaGVpZ2h0ID0gJGNvbnRhaW5lclswXS5jbGllbnRIZWlnaHQ7XG5cbiAgICBsZXQgY29scyA9IE1hdGguZmxvb3Iod2lkdGggLyAoZmFrZUNvbFdpZHRoKSk7XG4gICAgbGV0IHJvd3MgPSBNYXRoLmZsb29yKGhlaWdodCAvIChmYWtlQ29sSGVpZ2h0KSk7XG4gICAgZmFrZVJvdy5yZW1vdmUoKTtcblxuICAgIHJldHVybiB7Y29scywgcm93c307XG4gIH1cblxuICBfZ2V0VHR5RXZlbnRzQ29ublN0cigpe1xuICAgIGxldCB7c2lkLCB1cmwsIHRva2VuIH0gPSB0aGlzLnR0eVBhcmFtcztcbiAgICByZXR1cm4gYCR7dXJsfS9zZXNzaW9ucy8ke3NpZH0vZXZlbnRzL3N0cmVhbT9hY2Nlc3NfdG9rZW49JHt0b2tlbn1gO1xuICB9XG5cbiAgX2dldFR0eUNvbm5TdHIoKXtcbiAgICBsZXQge3NlcnZlcklkLCBsb2dpbiwgc2lkLCB1cmwsIHRva2VuIH0gPSB0aGlzLnR0eVBhcmFtcztcbiAgICB2YXIgcGFyYW1zID0ge1xuICAgICAgc2VydmVyX2lkOiBzZXJ2ZXJJZCxcbiAgICAgIGxvZ2luLFxuICAgICAgc2lkLFxuICAgICAgdGVybToge1xuICAgICAgICBoOiB0aGlzLnJvd3MsXG4gICAgICAgIHc6IHRoaXMuY29sc1xuICAgICAgfVxuICAgIH1cblxuICAgIHZhciBqc29uID0gSlNPTi5zdHJpbmdpZnkocGFyYW1zKTtcbiAgICB2YXIganNvbkVuY29kZWQgPSB3aW5kb3cuZW5jb2RlVVJJKGpzb24pO1xuXG4gICAgcmV0dXJuIGAke3VybH0vY29ubmVjdD9hY2Nlc3NfdG9rZW49JHt0b2tlbn0mcGFyYW1zPSR7anNvbkVuY29kZWR9YDtcbiAgfVxuXG59XG5cbm1vZHVsZS5leHBvcnRzID0gVHR5VGVybWluYWw7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tbW9uL3Rlcm1pbmFsLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgRXZlbnRFbWl0dGVyID0gcmVxdWlyZSgnZXZlbnRzJykuRXZlbnRFbWl0dGVyO1xudmFyIEJ1ZmZlciA9IHJlcXVpcmUoJ2J1ZmZlci8nKS5CdWZmZXI7XG5cbmNsYXNzIFR0eSBleHRlbmRzIEV2ZW50RW1pdHRlciB7XG5cbiAgY29uc3RydWN0b3IoKXtcbiAgICBzdXBlcigpO1xuICAgIHRoaXMuc29ja2V0ID0gbnVsbDtcbiAgfVxuXG4gIGRpc2Nvbm5lY3QoKXtcbiAgICB0aGlzLnNvY2tldC5jbG9zZSgpO1xuICB9XG5cbiAgcmVjb25uZWN0KG9wdGlvbnMpe1xuICAgIHRoaXMuZGlzY29ubmVjdCgpO1xuICAgIHRoaXMuc29ja2V0Lm9ub3BlbiA9IG51bGw7XG4gICAgdGhpcy5zb2NrZXQub25tZXNzYWdlID0gbnVsbDtcbiAgICB0aGlzLnNvY2tldC5vbmNsb3NlID0gbnVsbDtcblxuICAgIHRoaXMuY29ubmVjdChvcHRpb25zKTtcbiAgfVxuXG4gIGNvbm5lY3QoY29ublN0cil7XG4gICAgdGhpcy5zb2NrZXQgPSBuZXcgV2ViU29ja2V0KGNvbm5TdHIsICdwcm90bycpO1xuXG4gICAgdGhpcy5zb2NrZXQub25vcGVuID0gKCkgPT4ge1xuICAgICAgdGhpcy5lbWl0KCdvcGVuJyk7XG4gICAgfVxuXG4gICAgdGhpcy5zb2NrZXQub25tZXNzYWdlID0gKGUpPT57XG4gICAgICBsZXQgZGF0YSA9IG5ldyBCdWZmZXIoZS5kYXRhLCAnYmFzZTY0JykudG9TdHJpbmcoJ3V0ZjgnKTtcbiAgICAgIHRoaXMuZW1pdCgnZGF0YScsIGRhdGEpO1xuICAgIH1cblxuICAgIHRoaXMuc29ja2V0Lm9uY2xvc2UgPSAoKT0+e1xuICAgICAgdGhpcy5lbWl0KCdjbG9zZScpO1xuICAgIH1cbiAgfVxuXG4gIHNlbmQoZGF0YSl7XG4gICAgdGhpcy5zb2NrZXQuc2VuZChkYXRhKTtcbiAgfVxufVxuXG5tb2R1bGUuZXhwb3J0cyA9IFR0eTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21tb24vdHR5LmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHthY3Rpb25zfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2N1cnJlbnRTZXNzaW9uLycpO1xudmFyIHtVc2VySWNvbn0gPSByZXF1aXJlKCcuLy4uL2ljb25zLmpzeCcpO1xudmFyIFJlYWN0Q1NTVHJhbnNpdGlvbkdyb3VwID0gcmVxdWlyZSgncmVhY3QtYWRkb25zLWNzcy10cmFuc2l0aW9uLWdyb3VwJyk7XG5cbmNvbnN0IFNlc3Npb25MZWZ0UGFuZWwgPSAoe3BhcnRpZXN9KSA9PiB7XG4gIHBhcnRpZXMgPSBwYXJ0aWVzIHx8IFtdO1xuICBsZXQgdXNlckljb25zID0gcGFydGllcy5tYXAoKGl0ZW0sIGluZGV4KT0+KFxuICAgIDxsaSBrZXk9e2luZGV4fSBjbGFzc05hbWU9XCJhbmltYXRlZFwiPjxVc2VySWNvbiBjb2xvckluZGV4PXtpbmRleH0gaXNEYXJrPXt0cnVlfSBuYW1lPXtpdGVtLnVzZXJ9Lz48L2xpPlxuICApKTtcblxuICByZXR1cm4gKFxuICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXRlcm1pbmFsLXBhcnRpY2lwYW5zXCI+XG4gICAgICA8dWwgY2xhc3NOYW1lPVwibmF2XCI+XG4gICAgICAgIDxsaSB0aXRsZT1cIkNsb3NlXCI+XG4gICAgICAgICAgPGJ1dHRvbiBvbkNsaWNrPXthY3Rpb25zLmNsb3NlfSBjbGFzc05hbWU9XCJidG4gYnRuLWRhbmdlciBidG4tY2lyY2xlXCIgdHlwZT1cImJ1dHRvblwiPlxuICAgICAgICAgICAgPGkgY2xhc3NOYW1lPVwiZmEgZmEtdGltZXNcIj48L2k+XG4gICAgICAgICAgPC9idXR0b24+XG4gICAgICAgIDwvbGk+XG4gICAgICA8L3VsPlxuICAgICAgeyB1c2VySWNvbnMubGVuZ3RoID4gMCA/IDxociBjbGFzc05hbWU9XCJncnYtZGl2aWRlclwiLz4gOiBudWxsIH1cbiAgICAgIDxSZWFjdENTU1RyYW5zaXRpb25Hcm91cCBjbGFzc05hbWU9XCJuYXZcIiBjb21wb25lbnQ9J3VsJ1xuICAgICAgICB0cmFuc2l0aW9uRW50ZXJUaW1lb3V0PXs1MDB9XG4gICAgICAgIHRyYW5zaXRpb25MZWF2ZVRpbWVvdXQ9ezUwMH1cbiAgICAgICAgdHJhbnNpdGlvbk5hbWU9e3tcbiAgICAgICAgICBlbnRlcjogXCJmYWRlSW5cIixcbiAgICAgICAgICBsZWF2ZTogXCJmYWRlT3V0XCJcbiAgICAgICAgfX0+XG4gICAgICAgIHt1c2VySWNvbnN9XG4gICAgICA8L1JlYWN0Q1NTVHJhbnNpdGlvbkdyb3VwPlxuICAgIDwvZGl2PlxuICApXG59O1xuXG5tb2R1bGUuZXhwb3J0cyA9IFNlc3Npb25MZWZ0UGFuZWw7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9jdXJyZW50U2Vzc2lvbi9zZXNzaW9uTGVmdFBhbmVsLmpzeFxuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcblxudmFyIEdvb2dsZUF1dGhJbmZvID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWdvb2dsZS1hdXRoXCI+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWljb24tZ29vZ2xlLWF1dGhcIj48L2Rpdj5cbiAgICAgICAgPHN0cm9uZz5Hb29nbGUgQXV0aGVudGljYXRvcjwvc3Ryb25nPlxuICAgICAgICA8ZGl2PkRvd25sb2FkIDxhIGhyZWY9XCJodHRwczovL3N1cHBvcnQuZ29vZ2xlLmNvbS9hY2NvdW50cy9hbnN3ZXIvMTA2NjQ0Nz9obD1lblwiPkdvb2dsZSBBdXRoZW50aWNhdG9yPC9hPiBvbiB5b3VyIHBob25lIHRvIGFjY2VzcyB5b3VyIHR3byBmYWN0b3IgdG9rZW48L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pXG5cbm1vZHVsZS5leHBvcnRzID0gR29vZ2xlQXV0aEluZm87XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9nb29nbGVBdXRoTG9nby5qc3hcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIge2RlYm91bmNlfSA9IHJlcXVpcmUoJ18nKTtcblxudmFyIElucHV0U2VhcmNoID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIGdldEluaXRpYWxTdGF0ZSgpe1xuICAgIHRoaXMuZGVib3VuY2VkTm90aWZ5ID0gZGVib3VuY2UoKCk9PnsgICAgICAgIFxuICAgICAgICB0aGlzLnByb3BzLm9uQ2hhbmdlKHRoaXMuc3RhdGUudmFsdWUpO1xuICAgIH0sIDIwMCk7XG5cbiAgICByZXR1cm4ge3ZhbHVlOiB0aGlzLnByb3BzLnZhbHVlfTtcbiAgfSxcblxuICBvbkNoYW5nZShlKXtcbiAgICB0aGlzLnNldFN0YXRlKHt2YWx1ZTogZS50YXJnZXQudmFsdWV9KTtcbiAgICB0aGlzLmRlYm91bmNlZE5vdGlmeSgpO1xuICB9LFxuXG4gIGNvbXBvbmVudERpZE1vdW50KCkge1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50KCkge1xuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXNlYXJjaFwiPlxuICAgICAgICA8aW5wdXQgcGxhY2Vob2xkZXI9XCJTZWFyY2guLi5cIiBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2wgaW5wdXQtc21cIlxuICAgICAgICAgIHZhbHVlPXt0aGlzLnN0YXRlLnZhbHVlfVxuICAgICAgICAgIG9uQ2hhbmdlPXt0aGlzLm9uQ2hhbmdlfSAvPlxuICAgICAgPC9kaXY+XG4gICAgKTtcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gSW5wdXRTZWFyY2g7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9pbnB1dFNlYXJjaC5qc3hcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgSW5wdXRTZWFyY2ggPSByZXF1aXJlKCcuLy4uL2lucHV0U2VhcmNoLmpzeCcpO1xudmFyIHtUYWJsZSwgQ29sdW1uLCBDZWxsLCBTb3J0SGVhZGVyQ2VsbCwgU29ydFR5cGVzLCBFbXB0eUluZGljYXRvcn0gPSByZXF1aXJlKCdhcHAvY29tcG9uZW50cy90YWJsZS5qc3gnKTtcbnZhciB7Y3JlYXRlTmV3U2Vzc2lvbn0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9jdXJyZW50U2Vzc2lvbi9hY3Rpb25zJyk7XG5cbnZhciBfID0gcmVxdWlyZSgnXycpO1xudmFyIHtpc01hdGNofSA9IHJlcXVpcmUoJ2FwcC9jb21tb24vb2JqZWN0VXRpbHMnKTtcblxuY29uc3QgVGV4dENlbGwgPSAoe3Jvd0luZGV4LCBkYXRhLCBjb2x1bW5LZXksIC4uLnByb3BzfSkgPT4gKFxuICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgIHtkYXRhW3Jvd0luZGV4XVtjb2x1bW5LZXldfVxuICA8L0NlbGw+XG4pO1xuXG5jb25zdCBUYWdDZWxsID0gKHtyb3dJbmRleCwgZGF0YSwgLi4ucHJvcHN9KSA9PiAoXG4gIDxDZWxsIHsuLi5wcm9wc30+XG4gICAgeyBkYXRhW3Jvd0luZGV4XS50YWdzLm1hcCgoaXRlbSwgaW5kZXgpID0+XG4gICAgICAoPHNwYW4ga2V5PXtpbmRleH0gY2xhc3NOYW1lPVwibGFiZWwgbGFiZWwtZGVmYXVsdFwiPlxuICAgICAgICB7aXRlbS5yb2xlfSA8bGkgY2xhc3NOYW1lPVwiZmEgZmEtbG9uZy1hcnJvdy1yaWdodFwiPjwvbGk+XG4gICAgICAgIHtpdGVtLnZhbHVlfVxuICAgICAgPC9zcGFuPilcbiAgICApIH1cbiAgPC9DZWxsPlxuKTtcblxuY29uc3QgTG9naW5DZWxsID0gKHtsb2dpbnMsIG9uTG9naW5DbGljaywgcm93SW5kZXgsIGRhdGEsIC4uLnByb3BzfSkgPT4ge1xuICBpZighbG9naW5zIHx8bG9naW5zLmxlbmd0aCA9PT0gMCl7XG4gICAgcmV0dXJuIDxDZWxsIHsuLi5wcm9wc30gLz47XG4gIH1cblxuICB2YXIgc2VydmVySWQgPSBkYXRhW3Jvd0luZGV4XS5pZDtcbiAgdmFyICRsaXMgPSBbXTtcblxuICBmdW5jdGlvbiBvbkNsaWNrKGkpe1xuICAgIHZhciBsb2dpbiA9IGxvZ2luc1tpXTtcbiAgICBpZihvbkxvZ2luQ2xpY2spe1xuICAgICAgcmV0dXJuICgpPT4gb25Mb2dpbkNsaWNrKHNlcnZlcklkLCBsb2dpbik7XG4gICAgfWVsc2V7XG4gICAgICByZXR1cm4gKCkgPT4gY3JlYXRlTmV3U2Vzc2lvbihzZXJ2ZXJJZCwgbG9naW4pO1xuICAgIH1cbiAgfVxuXG4gIGZvcih2YXIgaSA9IDA7IGkgPCBsb2dpbnMubGVuZ3RoOyBpKyspe1xuICAgICRsaXMucHVzaCg8bGkga2V5PXtpfT48YSBvbkNsaWNrPXtvbkNsaWNrKGkpfT57bG9naW5zW2ldfTwvYT48L2xpPik7XG4gIH1cblxuICByZXR1cm4gKFxuICAgIDxDZWxsIHsuLi5wcm9wc30+XG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImJ0bi1ncm91cFwiPlxuICAgICAgICA8YnV0dG9uIHR5cGU9XCJidXR0b25cIiBvbkNsaWNrPXtvbkNsaWNrKDApfSBjbGFzc05hbWU9XCJidG4gYnRuLXhzIGJ0bi1wcmltYXJ5XCI+e2xvZ2luc1swXX08L2J1dHRvbj5cbiAgICAgICAge1xuICAgICAgICAgICRsaXMubGVuZ3RoID4gMSA/IChcbiAgICAgICAgICAgICAgW1xuICAgICAgICAgICAgICAgIDxidXR0b24ga2V5PXswfSBkYXRhLXRvZ2dsZT1cImRyb3Bkb3duXCIgY2xhc3NOYW1lPVwiYnRuIGJ0bi1kZWZhdWx0IGJ0bi14cyBkcm9wZG93bi10b2dnbGVcIiBhcmlhLWV4cGFuZGVkPVwidHJ1ZVwiPlxuICAgICAgICAgICAgICAgICAgPHNwYW4gY2xhc3NOYW1lPVwiY2FyZXRcIj48L3NwYW4+XG4gICAgICAgICAgICAgICAgPC9idXR0b24+LFxuICAgICAgICAgICAgICAgIDx1bCBrZXk9ezF9IGNsYXNzTmFtZT1cImRyb3Bkb3duLW1lbnVcIj5cbiAgICAgICAgICAgICAgICAgIHskbGlzfVxuICAgICAgICAgICAgICAgIDwvdWw+XG4gICAgICAgICAgICAgIF0gKVxuICAgICAgICAgICAgOiBudWxsXG4gICAgICAgIH1cbiAgICAgIDwvZGl2PlxuICAgIDwvQ2VsbD5cbiAgKVxufTtcblxudmFyIE5vZGVMaXN0ID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIGdldEluaXRpYWxTdGF0ZSgvKnByb3BzKi8pe1xuICAgIHRoaXMuc2VhcmNoYWJsZVByb3BzID0gWydhZGRyJywgJ2hvc3RuYW1lJywgJ3RhZ3MnXTtcbiAgICByZXR1cm4geyBmaWx0ZXI6ICcnLCBjb2xTb3J0RGlyczoge2hvc3RuYW1lOiBTb3J0VHlwZXMuREVTQ30gfTtcbiAgfSxcblxuICBvblNvcnRDaGFuZ2UoY29sdW1uS2V5LCBzb3J0RGlyKSB7XG4gICAgdGhpcy5zdGF0ZS5jb2xTb3J0RGlycyA9IHsgW2NvbHVtbktleV06IHNvcnREaXIgfTtcbiAgICB0aGlzLnNldFN0YXRlKHRoaXMuc3RhdGUpO1xuICB9LFxuXG4gIG9uRmlsdGVyQ2hhbmdlKHZhbHVlKXtcbiAgICB0aGlzLnN0YXRlLmZpbHRlciA9IHZhbHVlO1xuICAgIHRoaXMuc2V0U3RhdGUodGhpcy5zdGF0ZSk7XG4gIH0sXG5cbiAgc2VhcmNoQW5kRmlsdGVyQ2IodGFyZ2V0VmFsdWUsIHNlYXJjaFZhbHVlLCBwcm9wTmFtZSl7XG4gICAgaWYocHJvcE5hbWUgPT09ICd0YWdzJyl7XG4gICAgICByZXR1cm4gdGFyZ2V0VmFsdWUuc29tZSgoaXRlbSkgPT4ge1xuICAgICAgICBsZXQge3JvbGUsIHZhbHVlfSA9IGl0ZW07XG4gICAgICAgIHJldHVybiByb2xlLnRvTG9jYWxlVXBwZXJDYXNlKCkuaW5kZXhPZihzZWFyY2hWYWx1ZSkgIT09LTEgfHxcbiAgICAgICAgICB2YWx1ZS50b0xvY2FsZVVwcGVyQ2FzZSgpLmluZGV4T2Yoc2VhcmNoVmFsdWUpICE9PS0xO1xuICAgICAgfSk7XG4gICAgfVxuICB9LFxuXG4gIHNvcnRBbmRGaWx0ZXIoZGF0YSl7XG4gICAgdmFyIGZpbHRlcmVkID0gZGF0YS5maWx0ZXIob2JqPT4gaXNNYXRjaChvYmosIHRoaXMuc3RhdGUuZmlsdGVyLCB7XG4gICAgICAgIHNlYXJjaGFibGVQcm9wczogdGhpcy5zZWFyY2hhYmxlUHJvcHMsXG4gICAgICAgIGNiOiB0aGlzLnNlYXJjaEFuZEZpbHRlckNiXG4gICAgICB9KSk7XG5cbiAgICB2YXIgY29sdW1uS2V5ID0gT2JqZWN0LmdldE93blByb3BlcnR5TmFtZXModGhpcy5zdGF0ZS5jb2xTb3J0RGlycylbMF07XG4gICAgdmFyIHNvcnREaXIgPSB0aGlzLnN0YXRlLmNvbFNvcnREaXJzW2NvbHVtbktleV07XG4gICAgdmFyIHNvcnRlZCA9IF8uc29ydEJ5KGZpbHRlcmVkLCBjb2x1bW5LZXkpO1xuICAgIGlmKHNvcnREaXIgPT09IFNvcnRUeXBlcy5BU0Mpe1xuICAgICAgc29ydGVkID0gc29ydGVkLnJldmVyc2UoKTtcbiAgICB9XG5cbiAgICByZXR1cm4gc29ydGVkO1xuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgdmFyIGRhdGEgPSB0aGlzLnNvcnRBbmRGaWx0ZXIodGhpcy5wcm9wcy5ub2RlUmVjb3Jkcyk7XG4gICAgdmFyIGxvZ2lucyA9IHRoaXMucHJvcHMubG9naW5zO1xuICAgIHZhciBvbkxvZ2luQ2xpY2sgPSB0aGlzLnByb3BzLm9uTG9naW5DbGljaztcblxuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1ub2RlcyBncnYtcGFnZVwiPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4IGdydi1oZWFkZXJcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LWNvbHVtblwiPjwvZGl2PlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWZsZXgtY29sdW1uXCI+XG4gICAgICAgICAgICA8aDE+IE5vZGVzIDwvaDE+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZmxleC1jb2x1bW5cIj5cbiAgICAgICAgICAgIDxJbnB1dFNlYXJjaCB2YWx1ZT17dGhpcy5maWx0ZXJ9IG9uQ2hhbmdlPXt0aGlzLm9uRmlsdGVyQ2hhbmdlfS8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgIDwvZGl2PlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgIHtcbiAgICAgICAgICAgIGRhdGEubGVuZ3RoID09PSAwICYmIHRoaXMuc3RhdGUuZmlsdGVyLmxlbmd0aCA+IDAgPyA8RW1wdHlJbmRpY2F0b3IgdGV4dD1cIk5vIG1hdGNoaW5nIG5vZGVzIGZvdW5kLlwiLz4gOlxuXG4gICAgICAgICAgICA8VGFibGUgcm93Q291bnQ9e2RhdGEubGVuZ3RofSBjbGFzc05hbWU9XCJ0YWJsZS1zdHJpcGVkIGdydi1ub2Rlcy10YWJsZVwiPlxuICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgY29sdW1uS2V5PVwiaG9zdG5hbWVcIlxuICAgICAgICAgICAgICAgIGhlYWRlcj17XG4gICAgICAgICAgICAgICAgICA8U29ydEhlYWRlckNlbGxcbiAgICAgICAgICAgICAgICAgICAgc29ydERpcj17dGhpcy5zdGF0ZS5jb2xTb3J0RGlycy5ob3N0bmFtZX1cbiAgICAgICAgICAgICAgICAgICAgb25Tb3J0Q2hhbmdlPXt0aGlzLm9uU29ydENoYW5nZX1cbiAgICAgICAgICAgICAgICAgICAgdGl0bGU9XCJOb2RlXCJcbiAgICAgICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgICAgfVxuICAgICAgICAgICAgICAgIGNlbGw9ezxUZXh0Q2VsbCBkYXRhPXtkYXRhfS8+IH1cbiAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICAgIGNvbHVtbktleT1cImFkZHJcIlxuICAgICAgICAgICAgICAgIGhlYWRlcj17XG4gICAgICAgICAgICAgICAgICA8U29ydEhlYWRlckNlbGxcbiAgICAgICAgICAgICAgICAgICAgc29ydERpcj17dGhpcy5zdGF0ZS5jb2xTb3J0RGlycy5hZGRyfVxuICAgICAgICAgICAgICAgICAgICBvblNvcnRDaGFuZ2U9e3RoaXMub25Tb3J0Q2hhbmdlfVxuICAgICAgICAgICAgICAgICAgICB0aXRsZT1cIklQXCJcbiAgICAgICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgICAgfVxuXG4gICAgICAgICAgICAgICAgY2VsbD17PFRleHRDZWxsIGRhdGE9e2RhdGF9Lz4gfVxuICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgY29sdW1uS2V5PVwidGFnc1wiXG4gICAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD48L0NlbGw+IH1cbiAgICAgICAgICAgICAgICBjZWxsPXs8VGFnQ2VsbCBkYXRhPXtkYXRhfS8+IH1cbiAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICAgIGNvbHVtbktleT1cInJvbGVzXCJcbiAgICAgICAgICAgICAgICBvbkxvZ2luQ2xpY2s9e29uTG9naW5DbGlja31cbiAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPkxvZ2luIGFzPC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgY2VsbD17PExvZ2luQ2VsbCBkYXRhPXtkYXRhfSBsb2dpbnM9e2xvZ2luc30vPiB9XG4gICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICA8L1RhYmxlPlxuICAgICAgICAgIH1cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApXG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IE5vZGVMaXN0O1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvbm9kZXMvbm9kZUxpc3QuanN4XG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHsgTGluayB9ID0gcmVxdWlyZSgncmVhY3Qtcm91dGVyJyk7XG52YXIge25vZGVIb3N0TmFtZUJ5U2VydmVySWR9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvbm9kZXMvZ2V0dGVycycpO1xudmFyIHtkaXNwbGF5RGF0ZUZvcm1hdH0gPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG52YXIge0NlbGx9ID0gcmVxdWlyZSgnYXBwL2NvbXBvbmVudHMvdGFibGUuanN4Jyk7XG52YXIgbW9tZW50ID0gIHJlcXVpcmUoJ21vbWVudCcpO1xuXG5jb25zdCBEYXRlQ3JlYXRlZENlbGwgPSAoeyByb3dJbmRleCwgZGF0YSwgLi4ucHJvcHMgfSkgPT4ge1xuICBsZXQgY3JlYXRlZCA9IGRhdGFbcm93SW5kZXhdLmNyZWF0ZWQ7XG4gIGxldCBkaXNwbGF5RGF0ZSA9IG1vbWVudChjcmVhdGVkKS5mb3JtYXQoZGlzcGxheURhdGVGb3JtYXQpO1xuICByZXR1cm4gKFxuICAgIDxDZWxsIHsuLi5wcm9wc30+XG4gICAgICB7IGRpc3BsYXlEYXRlIH1cbiAgICA8L0NlbGw+XG4gIClcbn07XG5cbmNvbnN0IER1cmF0aW9uQ2VsbCA9ICh7IHJvd0luZGV4LCBkYXRhLCAuLi5wcm9wcyB9KSA9PiB7XG4gIGxldCBjcmVhdGVkID0gZGF0YVtyb3dJbmRleF0uY3JlYXRlZDtcbiAgbGV0IGxhc3RBY3RpdmUgPSBkYXRhW3Jvd0luZGV4XS5sYXN0QWN0aXZlO1xuXG4gIGxldCBlbmQgPSBtb21lbnQoY3JlYXRlZCk7XG4gIGxldCBub3cgPSBtb21lbnQobGFzdEFjdGl2ZSk7XG4gIGxldCBkdXJhdGlvbiA9IG1vbWVudC5kdXJhdGlvbihub3cuZGlmZihlbmQpKTtcbiAgbGV0IGRpc3BsYXlEYXRlID0gZHVyYXRpb24uaHVtYW5pemUoKTtcblxuICByZXR1cm4gKFxuICAgIDxDZWxsIHsuLi5wcm9wc30+XG4gICAgICB7IGRpc3BsYXlEYXRlIH1cbiAgICA8L0NlbGw+XG4gIClcbn07XG5cbmNvbnN0IFNpbmdsZVVzZXJDZWxsID0gKHsgcm93SW5kZXgsIGRhdGEsIC4uLnByb3BzIH0pID0+IHtcbiAgcmV0dXJuIChcbiAgICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgICAgPHNwYW4gY2xhc3NOYW1lPVwiZ3J2LXNlc3Npb25zLXVzZXIgbGFiZWwgbGFiZWwtZGVmYXVsdFwiPntkYXRhW3Jvd0luZGV4XS5sb2dpbn08L3NwYW4+XG4gICAgPC9DZWxsPlxuICApXG59O1xuXG5jb25zdCBVc2Vyc0NlbGwgPSAoeyByb3dJbmRleCwgZGF0YSwgLi4ucHJvcHMgfSkgPT4ge1xuICBsZXQgJHVzZXJzID0gZGF0YVtyb3dJbmRleF0ucGFydGllcy5tYXAoKGl0ZW0sIGl0ZW1JbmRleCk9PlxuICAgICg8c3BhbiBrZXk9e2l0ZW1JbmRleH0gY2xhc3NOYW1lPVwiZ3J2LXNlc3Npb25zLXVzZXIgbGFiZWwgbGFiZWwtZGVmYXVsdFwiPntpdGVtLnVzZXJ9PC9zcGFuPilcbiAgKVxuXG4gIHJldHVybiAoXG4gICAgPENlbGwgey4uLnByb3BzfT5cbiAgICAgIDxkaXY+XG4gICAgICAgIHskdXNlcnN9XG4gICAgICA8L2Rpdj5cbiAgICA8L0NlbGw+XG4gIClcbn07XG5cbmNvbnN0IEJ1dHRvbkNlbGwgPSAoeyByb3dJbmRleCwgZGF0YSwgLi4ucHJvcHMgfSkgPT4ge1xuICBsZXQgeyBzZXNzaW9uVXJsLCBhY3RpdmUgfSA9IGRhdGFbcm93SW5kZXhdO1xuICBsZXQgW2FjdGlvblRleHQsIGFjdGlvbkNsYXNzXSA9IGFjdGl2ZSA/IFsnam9pbicsICdidG4td2FybmluZyddIDogWydwbGF5JywgJ2J0bi1wcmltYXJ5J107XG4gIHJldHVybiAoXG4gICAgPENlbGwgey4uLnByb3BzfT5cbiAgICAgIDxMaW5rIHRvPXtzZXNzaW9uVXJsfSBjbGFzc05hbWU9e1wiYnRuIFwiICthY3Rpb25DbGFzcysgXCIgYnRuLXhzXCJ9IHR5cGU9XCJidXR0b25cIj57YWN0aW9uVGV4dH08L0xpbms+XG4gICAgPC9DZWxsPlxuICApXG59XG5cbmNvbnN0IE5vZGVDZWxsID0gKHsgcm93SW5kZXgsIGRhdGEsIC4uLnByb3BzIH0pID0+IHtcbiAgbGV0IHtzZXJ2ZXJJZH0gPSBkYXRhW3Jvd0luZGV4XTtcbiAgbGV0IGhvc3RuYW1lID0gcmVhY3Rvci5ldmFsdWF0ZShub2RlSG9zdE5hbWVCeVNlcnZlcklkKHNlcnZlcklkKSkgfHwgJ3Vua25vd24nO1xuXG4gIHJldHVybiAoXG4gICAgPENlbGwgey4uLnByb3BzfT5cbiAgICAgIHtob3N0bmFtZX1cbiAgICA8L0NlbGw+XG4gIClcbn1cblxuZXhwb3J0IGRlZmF1bHQgQnV0dG9uQ2VsbDtcblxuZXhwb3J0IHtcbiAgQnV0dG9uQ2VsbCxcbiAgVXNlcnNDZWxsLFxuICBEdXJhdGlvbkNlbGwsXG4gIERhdGVDcmVhdGVkQ2VsbCxcbiAgU2luZ2xlVXNlckNlbGwsXG4gIE5vZGVDZWxsXG59O1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvc2Vzc2lvbnMvbGlzdEl0ZW1zLmpzeFxuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxuaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfQVBQX0lOSVQ6IG51bGwsXG4gIFRMUFRfQVBQX0ZBSUxFRDogbnVsbCxcbiAgVExQVF9BUFBfUkVBRFk6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hcHAvYWN0aW9uVHlwZXMuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG52YXIgeyBTdG9yZSwgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcblxudmFyIHsgVExQVF9BUFBfSU5JVCwgVExQVF9BUFBfRkFJTEVELCBUTFBUX0FQUF9SRUFEWSB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG52YXIgaW5pdFN0YXRlID0gdG9JbW11dGFibGUoe1xuICBpc1JlYWR5OiBmYWxzZSxcbiAgaXNJbml0aWFsaXppbmc6IGZhbHNlLFxuICBpc0ZhaWxlZDogZmFsc2Vcbn0pO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG5cbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiBpbml0U3RhdGUuc2V0KCdpc0luaXRpYWxpemluZycsIHRydWUpO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX0FQUF9JTklULCAoKT0+IGluaXRTdGF0ZS5zZXQoJ2lzSW5pdGlhbGl6aW5nJywgdHJ1ZSkpO1xuICAgIHRoaXMub24oVExQVF9BUFBfUkVBRFksKCk9PiBpbml0U3RhdGUuc2V0KCdpc1JlYWR5JywgdHJ1ZSkpO1xuICAgIHRoaXMub24oVExQVF9BUFBfRkFJTEVELCgpPT4gaW5pdFN0YXRlLnNldCgnaXNGYWlsZWQnLCB0cnVlKSk7XG4gIH1cbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hcHAvYXBwU3RvcmUuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5pbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVExQVF9DVVJSRU5UX1NFU1NJT05fT1BFTjogbnVsbCxcbiAgVExQVF9DVVJSRU5UX1NFU1NJT05fQ0xPU0U6IG51bGwgIFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2N1cnJlbnRTZXNzaW9uL2FjdGlvblR5cGVzLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCdhcHAvc2VydmljZXMvc2Vzc2lvbicpO1xudmFyIGFwaSA9IHJlcXVpcmUoJ2FwcC9zZXJ2aWNlcy9hcGknKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG52YXIgZ2V0dGVycyA9IHJlcXVpcmUoJy4vZ2V0dGVycycpO1xudmFyIHNlc3Npb25Nb2R1bGUgPSByZXF1aXJlKCcuLy4uL3Nlc3Npb25zJyk7XG5cbmNvbnN0IGxvZ2dlciA9IHJlcXVpcmUoJ2FwcC9jb21tb24vbG9nZ2VyJykuY3JlYXRlKCdDdXJyZW50IFNlc3Npb24nKTtcbmNvbnN0IHsgVExQVF9DVVJSRU5UX1NFU1NJT05fT1BFTiwgVExQVF9DVVJSRU5UX1NFU1NJT05fQ0xPU0UgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuY29uc3QgYWN0aW9ucyA9IHtcblxuICBjbG9zZSgpe1xuICAgIGxldCB7aXNOZXdTZXNzaW9ufSA9IHJlYWN0b3IuZXZhbHVhdGUoZ2V0dGVycy5jdXJyZW50U2Vzc2lvbik7XG5cbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfQ1VSUkVOVF9TRVNTSU9OX0NMT1NFKTtcblxuICAgIGlmKGlzTmV3U2Vzc2lvbil7XG4gICAgICBzZXNzaW9uLmdldEhpc3RvcnkoKS5wdXNoKGNmZy5yb3V0ZXMubm9kZXMpO1xuICAgIH1lbHNle1xuICAgICAgc2Vzc2lvbi5nZXRIaXN0b3J5KCkucHVzaChjZmcucm91dGVzLnNlc3Npb25zKTtcbiAgICB9XG4gIH0sXG5cbiAgcmVzaXplKHcsIGgpe1xuICAgIC8vIHNvbWUgbWluIHZhbHVlc1xuICAgIHcgPSB3IDwgNSA/IDUgOiB3O1xuICAgIGggPSBoIDwgNSA/IDUgOiBoO1xuXG4gICAgbGV0IHJlcURhdGEgPSB7IHRlcm1pbmFsX3BhcmFtczogeyB3LCBoIH0gfTtcbiAgICBsZXQge3NpZH0gPSByZWFjdG9yLmV2YWx1YXRlKGdldHRlcnMuY3VycmVudFNlc3Npb24pO1xuXG4gICAgbG9nZ2VyLmluZm8oJ3Jlc2l6ZScsIGB3OiR7d30gYW5kIGg6JHtofWApO1xuICAgIGFwaS5wdXQoY2ZnLmFwaS5nZXRUZXJtaW5hbFNlc3Npb25Vcmwoc2lkKSwgcmVxRGF0YSlcbiAgICAgIC5kb25lKCgpPT4gbG9nZ2VyLmluZm8oJ3Jlc2l6ZWQnKSlcbiAgICAgIC5mYWlsKChlcnIpPT4gbG9nZ2VyLmVycm9yKCdmYWlsZWQgdG8gcmVzaXplJywgZXJyKSk7XG4gIH0sXG5cbiAgb3BlblNlc3Npb24oc2lkKXtcbiAgICBsb2dnZXIuaW5mbygnYXR0ZW1wdCB0byBvcGVuIHNlc3Npb24nLCB7c2lkfSk7XG4gICAgc2Vzc2lvbk1vZHVsZS5hY3Rpb25zLmZldGNoU2Vzc2lvbihzaWQpXG4gICAgICAuZG9uZSgoKT0+e1xuICAgICAgICBsZXQgc1ZpZXcgPSByZWFjdG9yLmV2YWx1YXRlKHNlc3Npb25Nb2R1bGUuZ2V0dGVycy5zZXNzaW9uVmlld0J5SWQoc2lkKSk7XG4gICAgICAgIGxldCB7IHNlcnZlcklkLCBsb2dpbiB9ID0gc1ZpZXc7XG4gICAgICAgIGxvZ2dlci5pbmZvKCdvcGVuIHNlc3Npb24nLCAnT0snKTtcbiAgICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX0NVUlJFTlRfU0VTU0lPTl9PUEVOLCB7XG4gICAgICAgICAgICBzZXJ2ZXJJZCxcbiAgICAgICAgICAgIGxvZ2luLFxuICAgICAgICAgICAgc2lkLFxuICAgICAgICAgICAgaXNOZXdTZXNzaW9uOiBmYWxzZVxuICAgICAgICAgIH0pO1xuICAgICAgfSlcbiAgICAgIC5mYWlsKChlcnIpPT57XG4gICAgICAgIGxvZ2dlci5lcnJvcignb3BlbiBzZXNzaW9uJywgZXJyKTtcbiAgICAgICAgc2Vzc2lvbi5nZXRIaXN0b3J5KCkucHVzaChjZmcucm91dGVzLnBhZ2VOb3RGb3VuZCk7XG4gICAgICB9KVxuICB9LFxuXG4gIGNyZWF0ZU5ld1Nlc3Npb24oc2VydmVySWQsIGxvZ2luKXtcbiAgICBsZXQgZGF0YSA9IHsgJ3Nlc3Npb24nOiB7J3Rlcm1pbmFsX3BhcmFtcyc6IHsndyc6IDQ1LCAnaCc6IDV9LCBsb2dpbn19XG4gICAgYXBpLnBvc3QoY2ZnLmFwaS5zaXRlU2Vzc2lvblBhdGgsIGRhdGEpLnRoZW4oanNvbj0+e1xuICAgICAgbGV0IHNpZCA9IGpzb24uc2Vzc2lvbi5pZDtcbiAgICAgIGxldCByb3V0ZVVybCA9IGNmZy5nZXRBY3RpdmVTZXNzaW9uUm91dGVVcmwoc2lkKTtcbiAgICAgIGxldCBoaXN0b3J5ID0gc2Vzc2lvbi5nZXRIaXN0b3J5KCk7XG5cbiAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9DVVJSRU5UX1NFU1NJT05fT1BFTiwge1xuICAgICAgIHNlcnZlcklkLFxuICAgICAgIGxvZ2luLFxuICAgICAgIHNpZCxcbiAgICAgICBpc05ld1Nlc3Npb246IHRydWVcbiAgICAgIH0pO1xuXG4gICAgICBoaXN0b3J5LnB1c2gocm91dGVVcmwpO1xuICAgfSk7XG5cbiAgfVxufVxuXG5leHBvcnQgZGVmYXVsdCBhY3Rpb25zO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvY3VycmVudFNlc3Npb24vYWN0aW9ucy5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG52YXIgeyBUTFBUX0NVUlJFTlRfU0VTU0lPTl9PUEVOLCBUTFBUX0NVUlJFTlRfU0VTU0lPTl9DTE9TRSB9ICA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKG51bGwpO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX0NVUlJFTlRfU0VTU0lPTl9PUEVOLCBzZXRDdXJyZW50U2Vzc2lvbik7XG4gICAgdGhpcy5vbihUTFBUX0NVUlJFTlRfU0VTU0lPTl9DTE9TRSwgY2xvc2UpO1xuICB9XG59KVxuXG5mdW5jdGlvbiBjbG9zZSgpe1xuICByZXR1cm4gdG9JbW11dGFibGUobnVsbCk7XG59XG5cbmZ1bmN0aW9uIHNldEN1cnJlbnRTZXNzaW9uKHN0YXRlLCB7c2VydmVySWQsIGxvZ2luLCBzaWQsIGlzTmV3U2Vzc2lvbn0gKXtcbiAgcmV0dXJuIHRvSW1tdXRhYmxlKHtcbiAgICBzZXJ2ZXJJZCxcbiAgICBsb2dpbixcbiAgICBzaWQsXG4gICAgaXNOZXdTZXNzaW9uXG4gIH0pO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvY3VycmVudFNlc3Npb24vY3VycmVudFNlc3Npb25TdG9yZS5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cbm1vZHVsZS5leHBvcnRzLmdldHRlcnMgPSByZXF1aXJlKCcuL2dldHRlcnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGlvbnMgPSByZXF1aXJlKCcuL2FjdGlvbnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGl2ZVRlcm1TdG9yZSA9IHJlcXVpcmUoJy4vY3VycmVudFNlc3Npb25TdG9yZScpO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvY3VycmVudFNlc3Npb24vaW5kZXguanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbmltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX0RJQUxPR19TRUxFQ1RfTk9ERV9TSE9XOiBudWxsLFxuICBUTFBUX0RJQUxPR19TRUxFQ1RfTk9ERV9DTE9TRTogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvYWN0aW9uVHlwZXMuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7IFRMUFRfRElBTE9HX1NFTEVDVF9OT0RFX1NIT1csIFRMUFRfRElBTE9HX1NFTEVDVF9OT0RFX0NMT1NFIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbnZhciBhY3Rpb25zID0ge1xuICBzaG93U2VsZWN0Tm9kZURpYWxvZygpe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9ESUFMT0dfU0VMRUNUX05PREVfU0hPVyk7XG4gIH0sXG5cbiAgY2xvc2VTZWxlY3ROb2RlRGlhbG9nKCl7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX0RJQUxPR19TRUxFQ1RfTk9ERV9DTE9TRSk7XG4gIH1cbn1cblxuZXhwb3J0IGRlZmF1bHQgYWN0aW9ucztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvYWN0aW9ucy5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG5cbnZhciB7IFRMUFRfRElBTE9HX1NFTEVDVF9OT0RFX1NIT1csIFRMUFRfRElBTE9HX1NFTEVDVF9OT0RFX0NMT1NFIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IFN0b3JlKHtcblxuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKHtcbiAgICAgIGlzU2VsZWN0Tm9kZURpYWxvZ09wZW46IGZhbHNlXG4gICAgfSk7XG4gIH0sXG5cbiAgaW5pdGlhbGl6ZSgpIHtcbiAgICB0aGlzLm9uKFRMUFRfRElBTE9HX1NFTEVDVF9OT0RFX1NIT1csIHNob3dTZWxlY3ROb2RlRGlhbG9nKTtcbiAgICB0aGlzLm9uKFRMUFRfRElBTE9HX1NFTEVDVF9OT0RFX0NMT1NFLCBjbG9zZVNlbGVjdE5vZGVEaWFsb2cpO1xuICB9XG59KVxuXG5mdW5jdGlvbiBzaG93U2VsZWN0Tm9kZURpYWxvZyhzdGF0ZSl7XG4gIHJldHVybiBzdGF0ZS5zZXQoJ2lzU2VsZWN0Tm9kZURpYWxvZ09wZW4nLCB0cnVlKTtcbn1cblxuZnVuY3Rpb24gY2xvc2VTZWxlY3ROb2RlRGlhbG9nKHN0YXRlKXtcbiAgcmV0dXJuIHN0YXRlLnNldCgnaXNTZWxlY3ROb2RlRGlhbG9nT3BlbicsIGZhbHNlKTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvZGlhbG9nU3RvcmUuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbmltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX05PREVTX1JFQ0VJVkU6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9hY3Rpb25UeXBlcy5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxuaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfTk9USUZJQ0FUSU9OU19BREQ6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9ub3RpZmljYXRpb25zL2FjdGlvblR5cGVzLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG5pbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVExQVF9SRVNUX0FQSV9TVEFSVDogbnVsbCxcbiAgVExQVF9SRVNUX0FQSV9TVUNDRVNTOiBudWxsLFxuICBUTFBUX1JFU1RfQVBJX0ZBSUw6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2FjdGlvblR5cGVzLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG5pbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVFJZSU5HX1RPX1NJR05fVVA6IG51bGwsXG4gIFRSWUlOR19UT19MT0dJTjogbnVsbCxcbiAgRkVUQ0hJTkdfSU5WSVRFOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9jb25zdGFudHMuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbmltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX1NUT1JFRF9TRVNTSU5TX0ZJTFRFUl9TRVRfUkFOR0U6IG51bGwsXG4gIFRMUFRfU1RPUkVEX1NFU1NJTlNfRklMVEVSX1NFVF9TVEFUVVM6IG51bGwsXG4gIFRMUFRfU1RPUkVEX1NFU1NJTlNfRklMVEVSX1JFQ0VJVkVfTU9SRTogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3N0b3JlZFNlc3Npb25zRmlsdGVyL2FjdGlvblR5cGVzLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgeyBUTFBUX1JFQ0VJVkVfVVNFUiwgVExQVF9SRUNFSVZFX1VTRVJfSU5WSVRFIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG52YXIgeyBUUllJTkdfVE9fU0lHTl9VUCwgVFJZSU5HX1RPX0xPR0lOLCBGRVRDSElOR19JTlZJVEV9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvcmVzdEFwaS9jb25zdGFudHMnKTtcbnZhciByZXN0QXBpQWN0aW9ucyA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3Jlc3RBcGkvYWN0aW9ucycpO1xudmFyIGF1dGggPSByZXF1aXJlKCdhcHAvc2VydmljZXMvYXV0aCcpO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCdhcHAvc2VydmljZXMvc2Vzc2lvbicpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcbnZhciBhcGkgPSByZXF1aXJlKCdhcHAvc2VydmljZXMvYXBpJyk7XG5cbmV4cG9ydCBkZWZhdWx0IHtcblxuICBmZXRjaEludml0ZShpbnZpdGVUb2tlbil7XG4gICAgdmFyIHBhdGggPSBjZmcuYXBpLmdldEludml0ZVVybChpbnZpdGVUb2tlbik7XG4gICAgcmVzdEFwaUFjdGlvbnMuc3RhcnQoRkVUQ0hJTkdfSU5WSVRFKTtcbiAgICBhcGkuZ2V0KHBhdGgpLmRvbmUoaW52aXRlPT57XG4gICAgICByZXN0QXBpQWN0aW9ucy5zdWNjZXNzKEZFVENISU5HX0lOVklURSk7XG4gICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfUkVDRUlWRV9VU0VSX0lOVklURSwgaW52aXRlKTtcbiAgICB9KS5cbiAgICBmYWlsKChlcnIpPT57XG4gICAgICByZXN0QXBpQWN0aW9ucy5mYWlsKEZFVENISU5HX0lOVklURSwgZXJyLnJlc3BvbnNlSlNPTi5tZXNzYWdlKTtcbiAgICB9KTtcbiAgfSxcblxuICBlbnN1cmVVc2VyKG5leHRTdGF0ZSwgcmVwbGFjZSwgY2Ipe1xuICAgIGF1dGguZW5zdXJlVXNlcigpXG4gICAgICAuZG9uZSgodXNlckRhdGEpPT4ge1xuICAgICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfUkVDRUlWRV9VU0VSLCB1c2VyRGF0YS51c2VyICk7XG4gICAgICAgIGNiKCk7XG4gICAgICB9KVxuICAgICAgLmZhaWwoKCk9PntcbiAgICAgICAgbGV0IG5ld0xvY2F0aW9uID0ge1xuICAgICAgICAgICAgcGF0aG5hbWU6IGNmZy5yb3V0ZXMubG9naW4sXG4gICAgICAgICAgICBzdGF0ZToge1xuICAgICAgICAgICAgICByZWRpcmVjdFRvOiBuZXh0U3RhdGUubG9jYXRpb24ucGF0aG5hbWVcbiAgICAgICAgICAgIH1cbiAgICAgICAgICB9O1xuXG4gICAgICAgIHJlcGxhY2UobmV3TG9jYXRpb24pO1xuICAgICAgICBjYigpO1xuICAgICAgfSk7XG4gIH0sXG5cbiAgc2lnblVwKHtuYW1lLCBwc3csIHRva2VuLCBpbnZpdGVUb2tlbn0pe1xuICAgIHJlc3RBcGlBY3Rpb25zLnN0YXJ0KFRSWUlOR19UT19TSUdOX1VQKTtcbiAgICBhdXRoLnNpZ25VcChuYW1lLCBwc3csIHRva2VuLCBpbnZpdGVUb2tlbilcbiAgICAgIC5kb25lKChzZXNzaW9uRGF0YSk9PntcbiAgICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFQ0VJVkVfVVNFUiwgc2Vzc2lvbkRhdGEudXNlcik7XG4gICAgICAgIHJlc3RBcGlBY3Rpb25zLnN1Y2Nlc3MoVFJZSU5HX1RPX1NJR05fVVApO1xuICAgICAgICBzZXNzaW9uLmdldEhpc3RvcnkoKS5wdXNoKHtwYXRobmFtZTogY2ZnLnJvdXRlcy5hcHB9KTtcbiAgICAgIH0pXG4gICAgICAuZmFpbCgoZXJyKT0+e1xuICAgICAgICByZXN0QXBpQWN0aW9ucy5mYWlsKFRSWUlOR19UT19TSUdOX1VQLCBlcnIucmVzcG9uc2VKU09OLm1lc3NhZ2UgfHwgJ2ZhaWxlZCB0byBzaW5nIHVwJyk7XG4gICAgICB9KTtcbiAgfSxcblxuICBsb2dpbih7dXNlciwgcGFzc3dvcmQsIHRva2VuLCBwcm92aWRlcn0sIHJlZGlyZWN0KXtcbiAgICBpZihwcm92aWRlcil7XG4gICAgICBsZXQgZnVsbFBhdGggPSBjZmcuZ2V0RnVsbFVybChyZWRpcmVjdCk7XG4gICAgICB3aW5kb3cubG9jYXRpb24gPSBjZmcuYXBpLmdldFNzb1VybChmdWxsUGF0aCwgcHJvdmlkZXIpO1xuICAgICAgcmV0dXJuO1xuICAgIH1cblxuICAgIHJlc3RBcGlBY3Rpb25zLnN0YXJ0KFRSWUlOR19UT19MT0dJTik7XG4gICAgYXV0aC5sb2dpbih1c2VyLCBwYXNzd29yZCwgdG9rZW4pXG4gICAgICAuZG9uZSgoc2Vzc2lvbkRhdGEpPT57XG4gICAgICAgIHJlc3RBcGlBY3Rpb25zLnN1Y2Nlc3MoVFJZSU5HX1RPX0xPR0lOKTtcbiAgICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFQ0VJVkVfVVNFUiwgc2Vzc2lvbkRhdGEudXNlcik7XG4gICAgICAgIHNlc3Npb24uZ2V0SGlzdG9yeSgpLnB1c2goe3BhdGhuYW1lOiByZWRpcmVjdH0pO1xuICAgICAgfSlcbiAgICAgIC5mYWlsKChlcnIpPT4gcmVzdEFwaUFjdGlvbnMuZmFpbChUUllJTkdfVE9fTE9HSU4sIGVyci5yZXNwb25zZUpTT04ubWVzc2FnZSkpXG4gICAgfVxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvdXNlci9hY3Rpb25zLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG5tb2R1bGUuZXhwb3J0cy5nZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3Rpb25zID0gcmVxdWlyZSgnLi9hY3Rpb25zJyk7XG5tb2R1bGUuZXhwb3J0cy5ub2RlU3RvcmUgPSByZXF1aXJlKCcuL3VzZXJTdG9yZScpO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvdXNlci9pbmRleC5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG52YXIgIHsgVExQVF9SRUNFSVZFX1VTRVIgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKG51bGwpO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX1JFQ0VJVkVfVVNFUiwgcmVjZWl2ZVVzZXIpXG4gIH1cblxufSlcblxuZnVuY3Rpb24gcmVjZWl2ZVVzZXIoc3RhdGUsIHVzZXIpe1xuICByZXR1cm4gdG9JbW11dGFibGUodXNlcik7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy91c2VyL3VzZXJTdG9yZS5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIGFwaSA9IHJlcXVpcmUoJy4vYXBpJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnLi4vY29uZmlnJyk7XG5cbmNvbnN0IGFwaVV0aWxzID0ge1xuICAgIGZpbHRlclNlc3Npb25zKHtzdGFydCwgZW5kLCBzaWQsIGxpbWl0LCBvcmRlcj0tMX0pe1xuICAgICAgbGV0IHBhcmFtcyA9IHtcbiAgICAgICAgc3RhcnQ6IHN0YXJ0LnRvSVNPU3RyaW5nKCksXG4gICAgICAgIGVuZCxcbiAgICAgICAgb3JkZXIsXG4gICAgICAgIGxpbWl0XG4gICAgICB9XG5cbiAgICAgIGlmKHNpZCl7XG4gICAgICAgIHBhcmFtcy5zZXNzaW9uX2lkID0gc2lkO1xuICAgICAgfVxuXG4gICAgICByZXR1cm4gYXBpLmdldChjZmcuYXBpLmdldEZldGNoU2Vzc2lvbnNVcmwocGFyYW1zKSlcbiAgICB9XG59XG5cbm1vZHVsZS5leHBvcnRzID0gYXBpVXRpbHM7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvc2VydmljZXMvYXBpVXRpbHMuanNcbiAqKi8iLCIvKipcbiAqIENvcHlyaWdodCAyMDEzLTIwMTUsIEZhY2Vib29rLCBJbmMuXG4gKiBBbGwgcmlnaHRzIHJlc2VydmVkLlxuICpcbiAqIFRoaXMgc291cmNlIGNvZGUgaXMgbGljZW5zZWQgdW5kZXIgdGhlIEJTRC1zdHlsZSBsaWNlbnNlIGZvdW5kIGluIHRoZVxuICogTElDRU5TRSBmaWxlIGluIHRoZSByb290IGRpcmVjdG9yeSBvZiB0aGlzIHNvdXJjZSB0cmVlLiBBbiBhZGRpdGlvbmFsIGdyYW50XG4gKiBvZiBwYXRlbnQgcmlnaHRzIGNhbiBiZSBmb3VuZCBpbiB0aGUgUEFURU5UUyBmaWxlIGluIHRoZSBzYW1lIGRpcmVjdG9yeS5cbiAqXG4gKiBAcHJvdmlkZXNNb2R1bGUgQ1NTQ29yZVxuICogQHR5cGVjaGVja3NcbiAqL1xuXG4ndXNlIHN0cmljdCc7XG5cbnZhciBpbnZhcmlhbnQgPSByZXF1aXJlKCcuL2ludmFyaWFudCcpO1xuXG4vKipcbiAqIFRoZSBDU1NDb3JlIG1vZHVsZSBzcGVjaWZpZXMgdGhlIEFQSSAoYW5kIGltcGxlbWVudHMgbW9zdCBvZiB0aGUgbWV0aG9kcylcbiAqIHRoYXQgc2hvdWxkIGJlIHVzZWQgd2hlbiBkZWFsaW5nIHdpdGggdGhlIGRpc3BsYXkgb2YgZWxlbWVudHMgKHZpYSB0aGVpclxuICogQ1NTIGNsYXNzZXMgYW5kIHZpc2liaWxpdHkgb24gc2NyZWVuLiBJdCBpcyBhbiBBUEkgZm9jdXNlZCBvbiBtdXRhdGluZyB0aGVcbiAqIGRpc3BsYXkgYW5kIG5vdCByZWFkaW5nIGl0IGFzIG5vIGxvZ2ljYWwgc3RhdGUgc2hvdWxkIGJlIGVuY29kZWQgaW4gdGhlXG4gKiBkaXNwbGF5IG9mIGVsZW1lbnRzLlxuICovXG5cbnZhciBDU1NDb3JlID0ge1xuXG4gIC8qKlxuICAgKiBBZGRzIHRoZSBjbGFzcyBwYXNzZWQgaW4gdG8gdGhlIGVsZW1lbnQgaWYgaXQgZG9lc24ndCBhbHJlYWR5IGhhdmUgaXQuXG4gICAqXG4gICAqIEBwYXJhbSB7RE9NRWxlbWVudH0gZWxlbWVudCB0aGUgZWxlbWVudCB0byBzZXQgdGhlIGNsYXNzIG9uXG4gICAqIEBwYXJhbSB7c3RyaW5nfSBjbGFzc05hbWUgdGhlIENTUyBjbGFzc05hbWVcbiAgICogQHJldHVybiB7RE9NRWxlbWVudH0gdGhlIGVsZW1lbnQgcGFzc2VkIGluXG4gICAqL1xuICBhZGRDbGFzczogZnVuY3Rpb24gKGVsZW1lbnQsIGNsYXNzTmFtZSkge1xuICAgICEhL1xccy8udGVzdChjbGFzc05hbWUpID8gcHJvY2Vzcy5lbnYuTk9ERV9FTlYgIT09ICdwcm9kdWN0aW9uJyA/IGludmFyaWFudChmYWxzZSwgJ0NTU0NvcmUuYWRkQ2xhc3MgdGFrZXMgb25seSBhIHNpbmdsZSBjbGFzcyBuYW1lLiBcIiVzXCIgY29udGFpbnMgJyArICdtdWx0aXBsZSBjbGFzc2VzLicsIGNsYXNzTmFtZSkgOiBpbnZhcmlhbnQoZmFsc2UpIDogdW5kZWZpbmVkO1xuXG4gICAgaWYgKGNsYXNzTmFtZSkge1xuICAgICAgaWYgKGVsZW1lbnQuY2xhc3NMaXN0KSB7XG4gICAgICAgIGVsZW1lbnQuY2xhc3NMaXN0LmFkZChjbGFzc05hbWUpO1xuICAgICAgfSBlbHNlIGlmICghQ1NTQ29yZS5oYXNDbGFzcyhlbGVtZW50LCBjbGFzc05hbWUpKSB7XG4gICAgICAgIGVsZW1lbnQuY2xhc3NOYW1lID0gZWxlbWVudC5jbGFzc05hbWUgKyAnICcgKyBjbGFzc05hbWU7XG4gICAgICB9XG4gICAgfVxuICAgIHJldHVybiBlbGVtZW50O1xuICB9LFxuXG4gIC8qKlxuICAgKiBSZW1vdmVzIHRoZSBjbGFzcyBwYXNzZWQgaW4gZnJvbSB0aGUgZWxlbWVudFxuICAgKlxuICAgKiBAcGFyYW0ge0RPTUVsZW1lbnR9IGVsZW1lbnQgdGhlIGVsZW1lbnQgdG8gc2V0IHRoZSBjbGFzcyBvblxuICAgKiBAcGFyYW0ge3N0cmluZ30gY2xhc3NOYW1lIHRoZSBDU1MgY2xhc3NOYW1lXG4gICAqIEByZXR1cm4ge0RPTUVsZW1lbnR9IHRoZSBlbGVtZW50IHBhc3NlZCBpblxuICAgKi9cbiAgcmVtb3ZlQ2xhc3M6IGZ1bmN0aW9uIChlbGVtZW50LCBjbGFzc05hbWUpIHtcbiAgICAhIS9cXHMvLnRlc3QoY2xhc3NOYW1lKSA/IHByb2Nlc3MuZW52Lk5PREVfRU5WICE9PSAncHJvZHVjdGlvbicgPyBpbnZhcmlhbnQoZmFsc2UsICdDU1NDb3JlLnJlbW92ZUNsYXNzIHRha2VzIG9ubHkgYSBzaW5nbGUgY2xhc3MgbmFtZS4gXCIlc1wiIGNvbnRhaW5zICcgKyAnbXVsdGlwbGUgY2xhc3Nlcy4nLCBjbGFzc05hbWUpIDogaW52YXJpYW50KGZhbHNlKSA6IHVuZGVmaW5lZDtcblxuICAgIGlmIChjbGFzc05hbWUpIHtcbiAgICAgIGlmIChlbGVtZW50LmNsYXNzTGlzdCkge1xuICAgICAgICBlbGVtZW50LmNsYXNzTGlzdC5yZW1vdmUoY2xhc3NOYW1lKTtcbiAgICAgIH0gZWxzZSBpZiAoQ1NTQ29yZS5oYXNDbGFzcyhlbGVtZW50LCBjbGFzc05hbWUpKSB7XG4gICAgICAgIGVsZW1lbnQuY2xhc3NOYW1lID0gZWxlbWVudC5jbGFzc05hbWUucmVwbGFjZShuZXcgUmVnRXhwKCcoXnxcXFxccyknICsgY2xhc3NOYW1lICsgJyg/OlxcXFxzfCQpJywgJ2cnKSwgJyQxJykucmVwbGFjZSgvXFxzKy9nLCAnICcpIC8vIG11bHRpcGxlIHNwYWNlcyB0byBvbmVcbiAgICAgICAgLnJlcGxhY2UoL15cXHMqfFxccyokL2csICcnKTsgLy8gdHJpbSB0aGUgZW5kc1xuICAgICAgfVxuICAgIH1cbiAgICByZXR1cm4gZWxlbWVudDtcbiAgfSxcblxuICAvKipcbiAgICogSGVscGVyIHRvIGFkZCBvciByZW1vdmUgYSBjbGFzcyBmcm9tIGFuIGVsZW1lbnQgYmFzZWQgb24gYSBjb25kaXRpb24uXG4gICAqXG4gICAqIEBwYXJhbSB7RE9NRWxlbWVudH0gZWxlbWVudCB0aGUgZWxlbWVudCB0byBzZXQgdGhlIGNsYXNzIG9uXG4gICAqIEBwYXJhbSB7c3RyaW5nfSBjbGFzc05hbWUgdGhlIENTUyBjbGFzc05hbWVcbiAgICogQHBhcmFtIHsqfSBib29sIGNvbmRpdGlvbiB0byB3aGV0aGVyIHRvIGFkZCBvciByZW1vdmUgdGhlIGNsYXNzXG4gICAqIEByZXR1cm4ge0RPTUVsZW1lbnR9IHRoZSBlbGVtZW50IHBhc3NlZCBpblxuICAgKi9cbiAgY29uZGl0aW9uQ2xhc3M6IGZ1bmN0aW9uIChlbGVtZW50LCBjbGFzc05hbWUsIGJvb2wpIHtcbiAgICByZXR1cm4gKGJvb2wgPyBDU1NDb3JlLmFkZENsYXNzIDogQ1NTQ29yZS5yZW1vdmVDbGFzcykoZWxlbWVudCwgY2xhc3NOYW1lKTtcbiAgfSxcblxuICAvKipcbiAgICogVGVzdHMgd2hldGhlciB0aGUgZWxlbWVudCBoYXMgdGhlIGNsYXNzIHNwZWNpZmllZC5cbiAgICpcbiAgICogQHBhcmFtIHtET01Ob2RlfERPTVdpbmRvd30gZWxlbWVudCB0aGUgZWxlbWVudCB0byBzZXQgdGhlIGNsYXNzIG9uXG4gICAqIEBwYXJhbSB7c3RyaW5nfSBjbGFzc05hbWUgdGhlIENTUyBjbGFzc05hbWVcbiAgICogQHJldHVybiB7Ym9vbGVhbn0gdHJ1ZSBpZiB0aGUgZWxlbWVudCBoYXMgdGhlIGNsYXNzLCBmYWxzZSBpZiBub3RcbiAgICovXG4gIGhhc0NsYXNzOiBmdW5jdGlvbiAoZWxlbWVudCwgY2xhc3NOYW1lKSB7XG4gICAgISEvXFxzLy50ZXN0KGNsYXNzTmFtZSkgPyBwcm9jZXNzLmVudi5OT0RFX0VOViAhPT0gJ3Byb2R1Y3Rpb24nID8gaW52YXJpYW50KGZhbHNlLCAnQ1NTLmhhc0NsYXNzIHRha2VzIG9ubHkgYSBzaW5nbGUgY2xhc3MgbmFtZS4nKSA6IGludmFyaWFudChmYWxzZSkgOiB1bmRlZmluZWQ7XG4gICAgaWYgKGVsZW1lbnQuY2xhc3NMaXN0KSB7XG4gICAgICByZXR1cm4gISFjbGFzc05hbWUgJiYgZWxlbWVudC5jbGFzc0xpc3QuY29udGFpbnMoY2xhc3NOYW1lKTtcbiAgICB9XG4gICAgcmV0dXJuICgnICcgKyBlbGVtZW50LmNsYXNzTmFtZSArICcgJykuaW5kZXhPZignICcgKyBjbGFzc05hbWUgKyAnICcpID4gLTE7XG4gIH1cblxufTtcblxubW9kdWxlLmV4cG9ydHMgPSBDU1NDb3JlO1xuXG5cbi8qKioqKioqKioqKioqKioqKlxuICoqIFdFQlBBQ0sgRk9PVEVSXG4gKiogLi9+L2ZianMvbGliL0NTU0NvcmUuanNcbiAqKiBtb2R1bGUgaWQgPSAyODZcbiAqKiBtb2R1bGUgY2h1bmtzID0gMVxuICoqLyIsIm1vZHVsZS5leHBvcnRzID0gcmVxdWlyZSgncmVhY3QvbGliL1JlYWN0Q1NTVHJhbnNpdGlvbkdyb3VwJyk7XG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiAuL34vcmVhY3QtYWRkb25zLWNzcy10cmFuc2l0aW9uLWdyb3VwL2luZGV4LmpzXG4gKiogbW9kdWxlIGlkID0gMzEwXG4gKiogbW9kdWxlIGNodW5rcyA9IDFcbiAqKi8iLCIvKlxuICogIFRoZSBNSVQgTGljZW5zZSAoTUlUKVxuICogIENvcHlyaWdodCAoYykgMjAxNSBSeWFuIEZsb3JlbmNlLCBNaWNoYWVsIEphY2tzb25cbiAqICBQZXJtaXNzaW9uIGlzIGhlcmVieSBncmFudGVkLCBmcmVlIG9mIGNoYXJnZSwgdG8gYW55IHBlcnNvbiBvYnRhaW5pbmcgYSBjb3B5IG9mIHRoaXMgc29mdHdhcmUgYW5kIGFzc29jaWF0ZWQgZG9jdW1lbnRhdGlvbiBmaWxlcyAodGhlIFwiU29mdHdhcmVcIiksIHRvIGRlYWwgaW4gdGhlIFNvZnR3YXJlIHdpdGhvdXQgcmVzdHJpY3Rpb24sIGluY2x1ZGluZyB3aXRob3V0IGxpbWl0YXRpb24gdGhlIHJpZ2h0cyB0byB1c2UsIGNvcHksIG1vZGlmeSwgbWVyZ2UsIHB1Ymxpc2gsIGRpc3RyaWJ1dGUsIHN1YmxpY2Vuc2UsIGFuZC9vciBzZWxsIGNvcGllcyBvZiB0aGUgU29mdHdhcmUsIGFuZCB0byBwZXJtaXQgcGVyc29ucyB0byB3aG9tIHRoZSBTb2Z0d2FyZSBpcyBmdXJuaXNoZWQgdG8gZG8gc28sIHN1YmplY3QgdG8gdGhlIGZvbGxvd2luZyBjb25kaXRpb25zOlxuICogIFRoZSBhYm92ZSBjb3B5cmlnaHQgbm90aWNlIGFuZCB0aGlzIHBlcm1pc3Npb24gbm90aWNlIHNoYWxsIGJlIGluY2x1ZGVkIGluIGFsbCBjb3BpZXMgb3Igc3Vic3RhbnRpYWwgcG9ydGlvbnMgb2YgdGhlIFNvZnR3YXJlLlxuICogIFRIRSBTT0ZUV0FSRSBJUyBQUk9WSURFRCBcIkFTIElTXCIsIFdJVEhPVVQgV0FSUkFOVFkgT0YgQU5ZIEtJTkQsIEVYUFJFU1MgT1IgSU1QTElFRCwgSU5DTFVESU5HIEJVVCBOT1QgTElNSVRFRCBUTyBUSEUgV0FSUkFOVElFUyBPRiBNRVJDSEFOVEFCSUxJVFksIEZJVE5FU1MgRk9SIEEgUEFSVElDVUxBUiBQVVJQT1NFIEFORCBOT05JTkZSSU5HRU1FTlQuIElOIE5PIEVWRU5UIFNIQUxMIFRIRSBBVVRIT1JTIE9SIENPUFlSSUdIVCBIT0xERVJTIEJFIExJQUJMRSBGT1IgQU5ZIENMQUlNLCBEQU1BR0VTIE9SIE9USEVSIExJQUJJTElUWSwgV0hFVEhFUiBJTiBBTiBBQ1RJT04gT0YgQ09OVFJBQ1QsIFRPUlQgT1IgT1RIRVJXSVNFLCBBUklTSU5HIEZST00sIE9VVCBPRiBPUiBJTiBDT05ORUNUSU9OIFdJVEggVEhFIFNPRlRXQVJFIE9SIFRIRSBVU0UgT1IgT1RIRVIgREVBTElOR1MgSU4gVEhFIFNPRlRXQVJFLlxuKi9cblxuaW1wb3J0IGludmFyaWFudCBmcm9tICdpbnZhcmlhbnQnXG5cbmZ1bmN0aW9uIGVzY2FwZVJlZ0V4cChzdHJpbmcpIHtcbiAgcmV0dXJuIHN0cmluZy5yZXBsYWNlKC9bLiorP14ke30oKXxbXFxdXFxcXF0vZywgJ1xcXFwkJicpXG59XG5cbmZ1bmN0aW9uIGVzY2FwZVNvdXJjZShzdHJpbmcpIHtcbiAgcmV0dXJuIGVzY2FwZVJlZ0V4cChzdHJpbmcpLnJlcGxhY2UoL1xcLysvZywgJy8rJylcbn1cblxuZnVuY3Rpb24gX2NvbXBpbGVQYXR0ZXJuKHBhdHRlcm4pIHtcbiAgbGV0IHJlZ2V4cFNvdXJjZSA9ICcnO1xuICBjb25zdCBwYXJhbU5hbWVzID0gW107XG4gIGNvbnN0IHRva2VucyA9IFtdO1xuXG4gIGxldCBtYXRjaCwgbGFzdEluZGV4ID0gMCwgbWF0Y2hlciA9IC86KFthLXpBLVpfJF1bYS16QS1aMC05XyRdKil8XFwqXFwqfFxcKnxcXCh8XFwpL2dcbiAgLyplc2xpbnQgbm8tY29uZC1hc3NpZ246IDAqL1xuICB3aGlsZSAoKG1hdGNoID0gbWF0Y2hlci5leGVjKHBhdHRlcm4pKSkge1xuICAgIGlmIChtYXRjaC5pbmRleCAhPT0gbGFzdEluZGV4KSB7XG4gICAgICB0b2tlbnMucHVzaChwYXR0ZXJuLnNsaWNlKGxhc3RJbmRleCwgbWF0Y2guaW5kZXgpKVxuICAgICAgcmVnZXhwU291cmNlICs9IGVzY2FwZVNvdXJjZShwYXR0ZXJuLnNsaWNlKGxhc3RJbmRleCwgbWF0Y2guaW5kZXgpKVxuICAgIH1cblxuICAgIGlmIChtYXRjaFsxXSkge1xuICAgICAgcmVnZXhwU291cmNlICs9ICcoW14vPyNdKyknO1xuICAgICAgcGFyYW1OYW1lcy5wdXNoKG1hdGNoWzFdKTtcbiAgICB9IGVsc2UgaWYgKG1hdGNoWzBdID09PSAnKionKSB7XG4gICAgICByZWdleHBTb3VyY2UgKz0gJyhbXFxcXHNcXFxcU10qKSdcbiAgICAgIHBhcmFtTmFtZXMucHVzaCgnc3BsYXQnKTtcbiAgICB9IGVsc2UgaWYgKG1hdGNoWzBdID09PSAnKicpIHtcbiAgICAgIHJlZ2V4cFNvdXJjZSArPSAnKFtcXFxcc1xcXFxTXSo/KSdcbiAgICAgIHBhcmFtTmFtZXMucHVzaCgnc3BsYXQnKTtcbiAgICB9IGVsc2UgaWYgKG1hdGNoWzBdID09PSAnKCcpIHtcbiAgICAgIHJlZ2V4cFNvdXJjZSArPSAnKD86JztcbiAgICB9IGVsc2UgaWYgKG1hdGNoWzBdID09PSAnKScpIHtcbiAgICAgIHJlZ2V4cFNvdXJjZSArPSAnKT8nO1xuICAgIH1cblxuICAgIHRva2Vucy5wdXNoKG1hdGNoWzBdKTtcblxuICAgIGxhc3RJbmRleCA9IG1hdGNoZXIubGFzdEluZGV4O1xuICB9XG5cbiAgaWYgKGxhc3RJbmRleCAhPT0gcGF0dGVybi5sZW5ndGgpIHtcbiAgICB0b2tlbnMucHVzaChwYXR0ZXJuLnNsaWNlKGxhc3RJbmRleCwgcGF0dGVybi5sZW5ndGgpKVxuICAgIHJlZ2V4cFNvdXJjZSArPSBlc2NhcGVTb3VyY2UocGF0dGVybi5zbGljZShsYXN0SW5kZXgsIHBhdHRlcm4ubGVuZ3RoKSlcbiAgfVxuXG4gIHJldHVybiB7XG4gICAgcGF0dGVybixcbiAgICByZWdleHBTb3VyY2UsXG4gICAgcGFyYW1OYW1lcyxcbiAgICB0b2tlbnNcbiAgfVxufVxuXG5jb25zdCBDb21waWxlZFBhdHRlcm5zQ2FjaGUgPSB7fVxuXG5leHBvcnQgZnVuY3Rpb24gY29tcGlsZVBhdHRlcm4ocGF0dGVybikge1xuICBpZiAoIShwYXR0ZXJuIGluIENvbXBpbGVkUGF0dGVybnNDYWNoZSkpXG4gICAgQ29tcGlsZWRQYXR0ZXJuc0NhY2hlW3BhdHRlcm5dID0gX2NvbXBpbGVQYXR0ZXJuKHBhdHRlcm4pXG5cbiAgcmV0dXJuIENvbXBpbGVkUGF0dGVybnNDYWNoZVtwYXR0ZXJuXVxufVxuXG4vKipcbiAqIEF0dGVtcHRzIHRvIG1hdGNoIGEgcGF0dGVybiBvbiB0aGUgZ2l2ZW4gcGF0aG5hbWUuIFBhdHRlcm5zIG1heSB1c2VcbiAqIHRoZSBmb2xsb3dpbmcgc3BlY2lhbCBjaGFyYWN0ZXJzOlxuICpcbiAqIC0gOnBhcmFtTmFtZSAgICAgTWF0Y2hlcyBhIFVSTCBzZWdtZW50IHVwIHRvIHRoZSBuZXh0IC8sID8sIG9yICMuIFRoZVxuICogICAgICAgICAgICAgICAgICBjYXB0dXJlZCBzdHJpbmcgaXMgY29uc2lkZXJlZCBhIFwicGFyYW1cIlxuICogLSAoKSAgICAgICAgICAgICBXcmFwcyBhIHNlZ21lbnQgb2YgdGhlIFVSTCB0aGF0IGlzIG9wdGlvbmFsXG4gKiAtICogICAgICAgICAgICAgIENvbnN1bWVzIChub24tZ3JlZWR5KSBhbGwgY2hhcmFjdGVycyB1cCB0byB0aGUgbmV4dFxuICogICAgICAgICAgICAgICAgICBjaGFyYWN0ZXIgaW4gdGhlIHBhdHRlcm4sIG9yIHRvIHRoZSBlbmQgb2YgdGhlIFVSTCBpZlxuICogICAgICAgICAgICAgICAgICB0aGVyZSBpcyBub25lXG4gKiAtICoqICAgICAgICAgICAgIENvbnN1bWVzIChncmVlZHkpIGFsbCBjaGFyYWN0ZXJzIHVwIHRvIHRoZSBuZXh0IGNoYXJhY3RlclxuICogICAgICAgICAgICAgICAgICBpbiB0aGUgcGF0dGVybiwgb3IgdG8gdGhlIGVuZCBvZiB0aGUgVVJMIGlmIHRoZXJlIGlzIG5vbmVcbiAqXG4gKiBUaGUgcmV0dXJuIHZhbHVlIGlzIGFuIG9iamVjdCB3aXRoIHRoZSBmb2xsb3dpbmcgcHJvcGVydGllczpcbiAqXG4gKiAtIHJlbWFpbmluZ1BhdGhuYW1lXG4gKiAtIHBhcmFtTmFtZXNcbiAqIC0gcGFyYW1WYWx1ZXNcbiAqL1xuZXhwb3J0IGZ1bmN0aW9uIG1hdGNoUGF0dGVybihwYXR0ZXJuLCBwYXRobmFtZSkge1xuICAvLyBNYWtlIGxlYWRpbmcgc2xhc2hlcyBjb25zaXN0ZW50IGJldHdlZW4gcGF0dGVybiBhbmQgcGF0aG5hbWUuXG4gIGlmIChwYXR0ZXJuLmNoYXJBdCgwKSAhPT0gJy8nKSB7XG4gICAgcGF0dGVybiA9IGAvJHtwYXR0ZXJufWBcbiAgfVxuICBpZiAocGF0aG5hbWUuY2hhckF0KDApICE9PSAnLycpIHtcbiAgICBwYXRobmFtZSA9IGAvJHtwYXRobmFtZX1gXG4gIH1cblxuICBsZXQgeyByZWdleHBTb3VyY2UsIHBhcmFtTmFtZXMsIHRva2VucyB9ID0gY29tcGlsZVBhdHRlcm4ocGF0dGVybilcblxuICByZWdleHBTb3VyY2UgKz0gJy8qJyAvLyBDYXB0dXJlIHBhdGggc2VwYXJhdG9yc1xuXG4gIC8vIFNwZWNpYWwtY2FzZSBwYXR0ZXJucyBsaWtlICcqJyBmb3IgY2F0Y2gtYWxsIHJvdXRlcy5cbiAgY29uc3QgY2FwdHVyZVJlbWFpbmluZyA9IHRva2Vuc1t0b2tlbnMubGVuZ3RoIC0gMV0gIT09ICcqJ1xuXG4gIGlmIChjYXB0dXJlUmVtYWluaW5nKSB7XG4gICAgLy8gVGhpcyB3aWxsIG1hdGNoIG5ld2xpbmVzIGluIHRoZSByZW1haW5pbmcgcGF0aC5cbiAgICByZWdleHBTb3VyY2UgKz0gJyhbXFxcXHNcXFxcU10qPyknXG4gIH1cblxuICBjb25zdCBtYXRjaCA9IHBhdGhuYW1lLm1hdGNoKG5ldyBSZWdFeHAoJ14nICsgcmVnZXhwU291cmNlICsgJyQnLCAnaScpKVxuXG4gIGxldCByZW1haW5pbmdQYXRobmFtZSwgcGFyYW1WYWx1ZXNcbiAgaWYgKG1hdGNoICE9IG51bGwpIHtcbiAgICBpZiAoY2FwdHVyZVJlbWFpbmluZykge1xuICAgICAgcmVtYWluaW5nUGF0aG5hbWUgPSBtYXRjaC5wb3AoKVxuICAgICAgY29uc3QgbWF0Y2hlZFBhdGggPVxuICAgICAgICBtYXRjaFswXS5zdWJzdHIoMCwgbWF0Y2hbMF0ubGVuZ3RoIC0gcmVtYWluaW5nUGF0aG5hbWUubGVuZ3RoKVxuXG4gICAgICAvLyBJZiB3ZSBkaWRuJ3QgbWF0Y2ggdGhlIGVudGlyZSBwYXRobmFtZSwgdGhlbiBtYWtlIHN1cmUgdGhhdCB0aGUgbWF0Y2hcbiAgICAgIC8vIHdlIGRpZCBnZXQgZW5kcyBhdCBhIHBhdGggc2VwYXJhdG9yIChwb3RlbnRpYWxseSB0aGUgb25lIHdlIGFkZGVkXG4gICAgICAvLyBhYm92ZSBhdCB0aGUgYmVnaW5uaW5nIG9mIHRoZSBwYXRoLCBpZiB0aGUgYWN0dWFsIG1hdGNoIHdhcyBlbXB0eSkuXG4gICAgICBpZiAoXG4gICAgICAgIHJlbWFpbmluZ1BhdGhuYW1lICYmXG4gICAgICAgIG1hdGNoZWRQYXRoLmNoYXJBdChtYXRjaGVkUGF0aC5sZW5ndGggLSAxKSAhPT0gJy8nXG4gICAgICApIHtcbiAgICAgICAgcmV0dXJuIHtcbiAgICAgICAgICByZW1haW5pbmdQYXRobmFtZTogbnVsbCxcbiAgICAgICAgICBwYXJhbU5hbWVzLFxuICAgICAgICAgIHBhcmFtVmFsdWVzOiBudWxsXG4gICAgICAgIH1cbiAgICAgIH1cbiAgICB9IGVsc2Uge1xuICAgICAgLy8gSWYgdGhpcyBtYXRjaGVkIGF0IGFsbCwgdGhlbiB0aGUgbWF0Y2ggd2FzIHRoZSBlbnRpcmUgcGF0aG5hbWUuXG4gICAgICByZW1haW5pbmdQYXRobmFtZSA9ICcnXG4gICAgfVxuXG4gICAgcGFyYW1WYWx1ZXMgPSBtYXRjaC5zbGljZSgxKS5tYXAoXG4gICAgICB2ID0+IHYgIT0gbnVsbCA/IGRlY29kZVVSSUNvbXBvbmVudCh2KSA6IHZcbiAgICApXG4gIH0gZWxzZSB7XG4gICAgcmVtYWluaW5nUGF0aG5hbWUgPSBwYXJhbVZhbHVlcyA9IG51bGxcbiAgfVxuXG4gIHJldHVybiB7XG4gICAgcmVtYWluaW5nUGF0aG5hbWUsXG4gICAgcGFyYW1OYW1lcyxcbiAgICBwYXJhbVZhbHVlc1xuICB9XG59XG5cbmV4cG9ydCBmdW5jdGlvbiBnZXRQYXJhbU5hbWVzKHBhdHRlcm4pIHtcbiAgcmV0dXJuIGNvbXBpbGVQYXR0ZXJuKHBhdHRlcm4pLnBhcmFtTmFtZXNcbn1cblxuZXhwb3J0IGZ1bmN0aW9uIGdldFBhcmFtcyhwYXR0ZXJuLCBwYXRobmFtZSkge1xuICBjb25zdCB7IHBhcmFtTmFtZXMsIHBhcmFtVmFsdWVzIH0gPSBtYXRjaFBhdHRlcm4ocGF0dGVybiwgcGF0aG5hbWUpXG5cbiAgaWYgKHBhcmFtVmFsdWVzICE9IG51bGwpIHtcbiAgICByZXR1cm4gcGFyYW1OYW1lcy5yZWR1Y2UoZnVuY3Rpb24gKG1lbW8sIHBhcmFtTmFtZSwgaW5kZXgpIHtcbiAgICAgIG1lbW9bcGFyYW1OYW1lXSA9IHBhcmFtVmFsdWVzW2luZGV4XVxuICAgICAgcmV0dXJuIG1lbW9cbiAgICB9LCB7fSlcbiAgfVxuXG4gIHJldHVybiBudWxsXG59XG5cbi8qKlxuICogUmV0dXJucyBhIHZlcnNpb24gb2YgdGhlIGdpdmVuIHBhdHRlcm4gd2l0aCBwYXJhbXMgaW50ZXJwb2xhdGVkLiBUaHJvd3NcbiAqIGlmIHRoZXJlIGlzIGEgZHluYW1pYyBzZWdtZW50IG9mIHRoZSBwYXR0ZXJuIGZvciB3aGljaCB0aGVyZSBpcyBubyBwYXJhbS5cbiAqL1xuZXhwb3J0IGZ1bmN0aW9uIGZvcm1hdFBhdHRlcm4ocGF0dGVybiwgcGFyYW1zKSB7XG4gIHBhcmFtcyA9IHBhcmFtcyB8fCB7fVxuXG4gIGNvbnN0IHsgdG9rZW5zIH0gPSBjb21waWxlUGF0dGVybihwYXR0ZXJuKVxuICBsZXQgcGFyZW5Db3VudCA9IDAsIHBhdGhuYW1lID0gJycsIHNwbGF0SW5kZXggPSAwXG5cbiAgbGV0IHRva2VuLCBwYXJhbU5hbWUsIHBhcmFtVmFsdWVcbiAgZm9yIChsZXQgaSA9IDAsIGxlbiA9IHRva2Vucy5sZW5ndGg7IGkgPCBsZW47ICsraSkge1xuICAgIHRva2VuID0gdG9rZW5zW2ldXG5cbiAgICBpZiAodG9rZW4gPT09ICcqJyB8fCB0b2tlbiA9PT0gJyoqJykge1xuICAgICAgcGFyYW1WYWx1ZSA9IEFycmF5LmlzQXJyYXkocGFyYW1zLnNwbGF0KSA/IHBhcmFtcy5zcGxhdFtzcGxhdEluZGV4KytdIDogcGFyYW1zLnNwbGF0XG5cbiAgICAgIGludmFyaWFudChcbiAgICAgICAgcGFyYW1WYWx1ZSAhPSBudWxsIHx8IHBhcmVuQ291bnQgPiAwLFxuICAgICAgICAnTWlzc2luZyBzcGxhdCAjJXMgZm9yIHBhdGggXCIlc1wiJyxcbiAgICAgICAgc3BsYXRJbmRleCwgcGF0dGVyblxuICAgICAgKVxuXG4gICAgICBpZiAocGFyYW1WYWx1ZSAhPSBudWxsKVxuICAgICAgICBwYXRobmFtZSArPSBlbmNvZGVVUkkocGFyYW1WYWx1ZSlcbiAgICB9IGVsc2UgaWYgKHRva2VuID09PSAnKCcpIHtcbiAgICAgIHBhcmVuQ291bnQgKz0gMVxuICAgIH0gZWxzZSBpZiAodG9rZW4gPT09ICcpJykge1xuICAgICAgcGFyZW5Db3VudCAtPSAxXG4gICAgfSBlbHNlIGlmICh0b2tlbi5jaGFyQXQoMCkgPT09ICc6Jykge1xuICAgICAgcGFyYW1OYW1lID0gdG9rZW4uc3Vic3RyaW5nKDEpXG4gICAgICBwYXJhbVZhbHVlID0gcGFyYW1zW3BhcmFtTmFtZV1cblxuICAgICAgaW52YXJpYW50KFxuICAgICAgICBwYXJhbVZhbHVlICE9IG51bGwgfHwgcGFyZW5Db3VudCA+IDAsXG4gICAgICAgICdNaXNzaW5nIFwiJXNcIiBwYXJhbWV0ZXIgZm9yIHBhdGggXCIlc1wiJyxcbiAgICAgICAgcGFyYW1OYW1lLCBwYXR0ZXJuXG4gICAgICApXG5cbiAgICAgIGlmIChwYXJhbVZhbHVlICE9IG51bGwpXG4gICAgICAgIHBhdGhuYW1lICs9IGVuY29kZVVSSUNvbXBvbmVudChwYXJhbVZhbHVlKVxuICAgIH0gZWxzZSB7XG4gICAgICBwYXRobmFtZSArPSB0b2tlblxuICAgIH1cbiAgfVxuXG4gIHJldHVybiBwYXRobmFtZS5yZXBsYWNlKC9cXC8rL2csICcvJylcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21tb24vcGF0dGVyblV0aWxzLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgRXZlbnRFbWl0dGVyID0gcmVxdWlyZSgnZXZlbnRzJykuRXZlbnRFbWl0dGVyO1xuXG5jb25zdCBsb2dnZXIgPSByZXF1aXJlKCcuL2xvZ2dlcicpLmNyZWF0ZSgnVHR5RXZlbnRzJyk7XG5cbmNsYXNzIFR0eUV2ZW50cyBleHRlbmRzIEV2ZW50RW1pdHRlciB7XG5cbiAgY29uc3RydWN0b3IoKXtcbiAgICBzdXBlcigpO1xuICAgIHRoaXMuc29ja2V0ID0gbnVsbDtcbiAgfVxuXG4gIGNvbm5lY3QoY29ublN0cil7XG4gICAgdGhpcy5zb2NrZXQgPSBuZXcgV2ViU29ja2V0KGNvbm5TdHIsICdwcm90bycpO1xuXG4gICAgdGhpcy5zb2NrZXQub25vcGVuID0gKCkgPT4ge1xuICAgICAgbG9nZ2VyLmluZm8oJ1R0eSBldmVudCBzdHJlYW0gaXMgb3BlbicpO1xuICAgIH1cblxuICAgIHRoaXMuc29ja2V0Lm9ubWVzc2FnZSA9IChldmVudCkgPT4ge1xuICAgICAgdHJ5XG4gICAgICB7XG4gICAgICAgIGxldCBqc29uID0gSlNPTi5wYXJzZShldmVudC5kYXRhKTtcbiAgICAgICAgdGhpcy5lbWl0KCdkYXRhJywganNvbi5zZXNzaW9uKTtcbiAgICAgIH1cbiAgICAgIGNhdGNoKGVycil7XG4gICAgICAgIGxvZ2dlci5lcnJvcignZmFpbGVkIHRvIHBhcnNlIGV2ZW50IHN0cmVhbSBkYXRhJywgZXJyKTtcbiAgICAgIH1cbiAgICB9O1xuXG4gICAgdGhpcy5zb2NrZXQub25jbG9zZSA9ICgpID0+IHtcbiAgICAgIGxvZ2dlci5pbmZvKCdUdHkgZXZlbnQgc3RyZWFtIGlzIGNsb3NlZCcpO1xuICAgIH07XG4gIH1cblxuICBkaXNjb25uZWN0KCl7XG4gICAgdGhpcy5zb2NrZXQuY2xvc2UoKTtcbiAgfVxuXG59XG5cbm1vZHVsZS5leHBvcnRzID0gVHR5RXZlbnRzO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbW1vbi90dHlFdmVudHMuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciBUdHkgPSByZXF1aXJlKCdhcHAvY29tbW9uL3R0eScpO1xudmFyIGFwaSA9IHJlcXVpcmUoJ2FwcC9zZXJ2aWNlcy9hcGknKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG52YXIge3Nob3dFcnJvcn0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9ub3RpZmljYXRpb25zL2FjdGlvbnMnKTtcbnZhciBCdWZmZXIgPSByZXF1aXJlKCdidWZmZXIvJykuQnVmZmVyO1xuXG5jb25zdCBsb2dnZXIgPSByZXF1aXJlKCdhcHAvY29tbW9uL2xvZ2dlcicpLmNyZWF0ZSgnVHR5UGxheWVyJyk7XG5jb25zdCBTVFJFQU1fU1RBUlRfSU5ERVggPSAxO1xuY29uc3QgUFJFX0ZFVENIX0JVRl9TSVpFID0gNTAwMDtcblxuZnVuY3Rpb24gaGFuZGxlQWpheEVycm9yKGVycil7XG4gIHNob3dFcnJvcignVW5hYmxlIHRvIHJldHJpZXZlIHNlc3Npb24gaW5mbycpO1xuICBsb2dnZXIuZXJyb3IoJ2ZldGNoaW5nIHNlc3Npb24gbGVuZ3RoJywgZXJyKTtcbn1cblxuY2xhc3MgVHR5UGxheWVyIGV4dGVuZHMgVHR5IHtcbiAgY29uc3RydWN0b3Ioe3NpZH0pe1xuICAgIHN1cGVyKHt9KTtcbiAgICB0aGlzLnNpZCA9IHNpZDtcbiAgICB0aGlzLmN1cnJlbnQgPSBTVFJFQU1fU1RBUlRfSU5ERVg7XG4gICAgdGhpcy5sZW5ndGggPSAtMTtcbiAgICB0aGlzLnR0eVN0cmVhbSA9IG5ldyBBcnJheSgpO1xuICAgIHRoaXMuaXNQbGF5aW5nID0gZmFsc2U7XG4gICAgdGhpcy5pc0Vycm9yID0gZmFsc2U7XG4gICAgdGhpcy5pc1JlYWR5ID0gZmFsc2U7XG4gICAgdGhpcy5pc0xvYWRpbmcgPSB0cnVlO1xuICB9XG5cbiAgc2VuZCgpe1xuICB9XG5cbiAgcmVzaXplKCl7XG4gIH1cblxuICBnZXREaW1lbnNpb25zKCl7XG4gICAgbGV0IGNodW5rSW5mbyA9IHRoaXMudHR5U3RyZWFtW3RoaXMuY3VycmVudC0xXTtcbiAgICBpZihjaHVua0luZm8pe1xuICAgICAgIHJldHVybiB7XG4gICAgICAgICB3OiBjaHVua0luZm8udyxcbiAgICAgICAgIGg6IGNodW5rSW5mby5oXG4gICAgICAgfVxuICAgIH1lbHNle1xuICAgICAgcmV0dXJuIHt3OiB1bmRlZmluZWQsIGg6IHVuZGVmaW5lZH07XG4gICAgfVxuICB9XG5cbiAgX2hhbmRsZUVycm9yKCl7XG5cbiAgfVxuXG4gIF9mZXRjaEV2ZW50cygpe1xuXG5cbiAgICBhcGkuZ2V0KGAvdjEvd2ViYXBpL3NpdGVzLy1jdXJyZW50LS9zZXNzaW9ucy8ke3RoaXMuc2lkfS9ldmVudHNgKTtcbiAgICBhcGkuZ2V0KGAvdjEvd2ViYXBpL3NpdGVzLy1jdXJyZW50LS9zZXNzaW9ucy8ke3RoaXMuc2lkfS9zdHJlYW1gKTtcbiAgICAgIC8vLnRoZW4odGhpcy5faW5pdEV2ZW50cy5iaW5kKHRoaXMpKVxuICAgICAgLy8uZmFpbChoYW5kbGVBamF4RXJyb3IpXG5cblxuXG5cbiAgfVxuXG4gIGNvbm5lY3QoKXtcbiAgICB0aGlzLl9zZXRTdGF0dXNGbGFnKHtpc0xvYWRpbmc6IHRydWV9KTtcblxuICAgIHRoaXMuX2ZldGNoRXZlbnRzKCk7XG4gICAgYXBpLmdldChjZmcuYXBpLmdldEZldGNoU2Vzc2lvbkxlbmd0aFVybCh0aGlzLnNpZCkpXG4gICAgICAuZG9uZSgoZGF0YSk9PntcbiAgICAgICAgLypcbiAgICAgICAgKiB0ZW1wb3JhcnkgaG90Zml4IHRvIGJhY2stZW5kIGlzc3VlIHJlbGF0ZWQgdG8gc2Vzc2lvbiBjaHVua3Mgc3RhcnRpbmcgYXRcbiAgICAgICAgKiBpbmRleD0xIGFuZCBlbmRpbmcgYXQgaW5kZXg9bGVuZ3RoKzFcbiAgICAgICAgKiovXG4gICAgICAgIHRoaXMubGVuZ3RoID0gZGF0YS5jb3VudCsxO1xuICAgICAgICB0aGlzLl9zZXRTdGF0dXNGbGFnKHtpc1JlYWR5OiB0cnVlfSk7XG4gICAgICB9KVxuICAgICAgLmZhaWwoKGVycik9PntcbiAgICAgICAgaGFuZGxlQWpheEVycm9yKGVycik7XG4gICAgICB9KVxuICAgICAgLmFsd2F5cygoKT0+e1xuICAgICAgICB0aGlzLl9jaGFuZ2UoKTtcbiAgICAgIH0pO1xuXG4gICAgdGhpcy5fY2hhbmdlKCk7XG4gIH1cblxuICBtb3ZlKG5ld1Bvcyl7XG4gICAgaWYoIXRoaXMuaXNSZWFkeSl7XG4gICAgICByZXR1cm47XG4gICAgfVxuXG4gICAgaWYobmV3UG9zID09PSB1bmRlZmluZWQpe1xuICAgICAgbmV3UG9zID0gdGhpcy5jdXJyZW50ICsgMTtcbiAgICB9XG5cbiAgICBpZihuZXdQb3MgPiB0aGlzLmxlbmd0aCl7XG4gICAgICBuZXdQb3MgPSB0aGlzLmxlbmd0aDtcbiAgICAgIHRoaXMuc3RvcCgpO1xuICAgIH1cblxuICAgIGlmKG5ld1BvcyA9PT0gMCl7XG4gICAgICBuZXdQb3MgPSBTVFJFQU1fU1RBUlRfSU5ERVg7XG4gICAgfVxuXG4gICAgaWYodGhpcy5jdXJyZW50IDwgbmV3UG9zKXtcbiAgICAgIHRoaXMuX3Nob3dDaHVuayh0aGlzLmN1cnJlbnQsIG5ld1Bvcyk7XG4gICAgfWVsc2V7XG4gICAgICB0aGlzLmVtaXQoJ3Jlc2V0Jyk7XG4gICAgICB0aGlzLl9zaG93Q2h1bmsoU1RSRUFNX1NUQVJUX0lOREVYLCBuZXdQb3MpO1xuICAgIH1cblxuICAgIHRoaXMuX2NoYW5nZSgpO1xuICB9XG5cbiAgc3RvcCgpe1xuICAgIHRoaXMuaXNQbGF5aW5nID0gZmFsc2U7XG4gICAgdGhpcy50aW1lciA9IGNsZWFySW50ZXJ2YWwodGhpcy50aW1lcik7XG4gICAgdGhpcy5fY2hhbmdlKCk7XG4gIH1cblxuICBwbGF5KCl7XG4gICAgaWYodGhpcy5pc1BsYXlpbmcpe1xuICAgICAgcmV0dXJuO1xuICAgIH1cblxuICAgIHRoaXMuaXNQbGF5aW5nID0gdHJ1ZTtcblxuICAgIC8vIHN0YXJ0IGZyb20gdGhlIGJlZ2lubmluZyBpZiBhdCB0aGUgZW5kXG4gICAgaWYodGhpcy5jdXJyZW50ID09PSB0aGlzLmxlbmd0aCl7XG4gICAgICB0aGlzLmN1cnJlbnQgPSBTVFJFQU1fU1RBUlRfSU5ERVg7XG4gICAgICB0aGlzLmVtaXQoJ3Jlc2V0Jyk7XG4gICAgfVxuXG4gICAgdGhpcy50aW1lciA9IHNldEludGVydmFsKHRoaXMubW92ZS5iaW5kKHRoaXMpLCAxNTApO1xuICAgIHRoaXMuX2NoYW5nZSgpO1xuICB9XG5cbiAgX3Nob3VsZEZldGNoKHN0YXJ0LCBlbmQpe1xuICAgIGZvcih2YXIgaSA9IHN0YXJ0OyBpIDwgZW5kOyBpKyspe1xuICAgICAgaWYodGhpcy50dHlTdHJlYW1baV0gPT09IHVuZGVmaW5lZCl7XG4gICAgICAgIHJldHVybiB0cnVlO1xuICAgICAgfVxuICAgIH1cblxuICAgIHJldHVybiBmYWxzZTtcbiAgfVxuXG4gIF9mZXRjaChzdGFydCwgZW5kKXtcbiAgICBlbmQgPSBlbmQgKyBQUkVfRkVUQ0hfQlVGX1NJWkU7XG4gICAgZW5kID0gZW5kID4gdGhpcy5sZW5ndGggPyB0aGlzLmxlbmd0aCA6IGVuZDtcblxuICAgIHRoaXMuX3NldFN0YXR1c0ZsYWcoe2lzTG9hZGluZzogdHJ1ZSB9KTtcblxuICAgIHJldHVybiBhcGkuZ2V0KGNmZy5hcGkuZ2V0RmV0Y2hTZXNzaW9uQ2h1bmtVcmwoe3NpZDogdGhpcy5zaWQsIHN0YXJ0LCBlbmR9KSkuXG4gICAgICBkb25lKChyZXNwb25zZSk9PntcbiAgICAgICAgZm9yKHZhciBpID0gMDsgaSA8IGVuZC1zdGFydDsgaSsrKXtcbiAgICAgICAgICBsZXQge2RhdGEsIGRlbGF5LCB0ZXJtOiB7aCwgd319ID0gcmVzcG9uc2UuY2h1bmtzW2ldO1xuICAgICAgICAgIGRhdGEgPSBuZXcgQnVmZmVyKGRhdGEsICdiYXNlNjQnKS50b1N0cmluZygndXRmOCcpO1xuICAgICAgICAgIHRoaXMudHR5U3RyZWFtW3N0YXJ0K2ldID0geyBkYXRhLCBkZWxheSwgdywgaCB9O1xuICAgICAgICB9XG5cbiAgICAgICAgdGhpcy5fc2V0U3RhdHVzRmxhZyh7aXNSZWFkeTogdHJ1ZSB9KTtcbiAgICAgIH0pXG4gICAgICAuZmFpbCgoZXJyKT0+e1xuICAgICAgICBoYW5kbGVBamF4RXJyb3IoZXJyKTtcbiAgICAgICAgdGhpcy5fc2V0U3RhdHVzRmxhZyh7aXNFcnJvcjogdHJ1ZSB9KTtcbiAgICAgIH0pXG4gIH1cblxuICBfZGlzcGxheShzdGFydCwgZW5kKXtcbiAgICBsZXQgc3RyZWFtID0gdGhpcy50dHlTdHJlYW07XG4gICAgbGV0IGk7XG4gICAgbGV0IHRtcCA9IFt7XG4gICAgICBkYXRhOiBbc3RyZWFtW3N0YXJ0XS5kYXRhXSxcbiAgICAgIHc6IHN0cmVhbVtzdGFydF0udyxcbiAgICAgIGg6IHN0cmVhbVtzdGFydF0uaFxuICAgIH1dO1xuXG4gICAgbGV0IGN1ciA9IHRtcFswXTtcblxuICAgIGZvcihpID0gc3RhcnQrMTsgaSA8IGVuZDsgaSsrKXtcbiAgICAgIGlmKGN1ci53ID09PSBzdHJlYW1baV0udyAmJiBjdXIuaCA9PT0gc3RyZWFtW2ldLmgpe1xuICAgICAgICBjdXIuZGF0YS5wdXNoKHN0cmVhbVtpXS5kYXRhKVxuICAgICAgfWVsc2V7XG4gICAgICAgIGN1ciA9e1xuICAgICAgICAgIGRhdGE6IFtzdHJlYW1baV0uZGF0YV0sXG4gICAgICAgICAgdzogc3RyZWFtW2ldLncsXG4gICAgICAgICAgaDogc3RyZWFtW2ldLmhcbiAgICAgICAgfTtcblxuICAgICAgICB0bXAucHVzaChjdXIpO1xuICAgICAgfVxuICAgIH1cblxuICAgIGZvcihpID0gMDsgaSA8IHRtcC5sZW5ndGg7IGkgKyspe1xuICAgICAgbGV0IHN0ciA9IHRtcFtpXS5kYXRhLmpvaW4oJycpO1xuICAgICAgbGV0IHtoLCB3fSA9IHRtcFtpXTtcbiAgICAgIHRoaXMuZW1pdCgncmVzaXplJywge2gsIHd9KTtcbiAgICAgIHRoaXMuZW1pdCgnZGF0YScsIHN0cik7XG4gICAgfVxuXG4gICAgdGhpcy5jdXJyZW50ID0gZW5kO1xuICB9XG5cbiAgX3Nob3dDaHVuayhzdGFydCwgZW5kKXtcbiAgICBpZih0aGlzLl9zaG91bGRGZXRjaChzdGFydCwgZW5kKSl7XG4gICAgICB0aGlzLl9mZXRjaChzdGFydCwgZW5kKS50aGVuKCgpPT5cbiAgICAgICAgdGhpcy5fZGlzcGxheShzdGFydCwgZW5kKSk7XG4gICAgfWVsc2V7XG4gICAgICB0aGlzLl9kaXNwbGF5KHN0YXJ0LCBlbmQpO1xuICAgIH1cbiAgfVxuXG4gIF9zZXRTdGF0dXNGbGFnKG5ld1N0YXR1cyl7XG4gICAgbGV0IHtpc1JlYWR5PWZhbHNlLCBpc0Vycm9yPWZhbHNlLCBpc0xvYWRpbmc9ZmFsc2UgfSA9IG5ld1N0YXR1cztcblxuICAgIHRoaXMuaXNSZWFkeSA9IGlzUmVhZHk7XG4gICAgdGhpcy5pc0Vycm9yID0gaXNFcnJvcjtcbiAgICB0aGlzLmlzTG9hZGluZyA9IGlzTG9hZGluZztcbiAgfVxuXG4gIF9jaGFuZ2UoKXtcbiAgICB0aGlzLmVtaXQoJ2NoYW5nZScpO1xuICB9XG59XG5cbmV4cG9ydCBkZWZhdWx0IFR0eVBsYXllcjtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21tb24vdHR5UGxheWVyLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIE5hdkxlZnRCYXIgPSByZXF1aXJlKCcuL25hdkxlZnRCYXInKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7YWN0aW9ucywgZ2V0dGVyc30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9hcHAnKTtcbnZhciBTZWxlY3ROb2RlRGlhbG9nID0gcmVxdWlyZSgnLi9zZWxlY3ROb2RlRGlhbG9nLmpzeCcpO1xudmFyIE5vdGlmaWNhdGlvbkhvc3QgPSByZXF1aXJlKCcuL25vdGlmaWNhdGlvbkhvc3QuanN4Jyk7XG5cbnZhciBBcHAgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIGFwcDogZ2V0dGVycy5hcHBTdGF0ZVxuICAgIH1cbiAgfSxcblxuICBjb21wb25lbnRXaWxsTW91bnQoKXtcbiAgICBhY3Rpb25zLmluaXRBcHAoKTtcbiAgICB0aGlzLnJlZnJlc2hJbnRlcnZhbCA9IHNldEludGVydmFsKGFjdGlvbnMuZmV0Y2hOb2Rlc0FuZFNlc3Npb25zLCAzNTAwMCk7XG4gIH0sXG5cbiAgY29tcG9uZW50V2lsbFVubW91bnQ6IGZ1bmN0aW9uKCkge1xuICAgIGNsZWFySW50ZXJ2YWwodGhpcy5yZWZyZXNoSW50ZXJ2YWwpO1xuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgaWYodGhpcy5zdGF0ZS5hcHAuaXNJbml0aWFsaXppbmcpe1xuICAgICAgcmV0dXJuIG51bGw7XG4gICAgfVxuXG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXRscHQgZ3J2LWZsZXggZ3J2LWZsZXgtcm93XCI+XG4gICAgICAgIDxTZWxlY3ROb2RlRGlhbG9nLz5cbiAgICAgICAgPE5vdGlmaWNhdGlvbkhvc3QvPlxuICAgICAgICB7dGhpcy5wcm9wcy5DdXJyZW50U2Vzc2lvbkhvc3R9XG4gICAgICAgIDxOYXZMZWZ0QmFyLz5cbiAgICAgICAge3RoaXMucHJvcHMuY2hpbGRyZW59XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KVxuXG5tb2R1bGUuZXhwb3J0cyA9IEFwcDtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2FwcC5qc3hcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIge25vZGVIb3N0TmFtZUJ5U2VydmVySWR9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvbm9kZXMvZ2V0dGVycycpO1xudmFyIFR0eVRlcm1pbmFsID0gcmVxdWlyZSgnLi8uLi90ZXJtaW5hbC5qc3gnKTtcbnZhciBTZXNzaW9uTGVmdFBhbmVsID0gcmVxdWlyZSgnLi9zZXNzaW9uTGVmdFBhbmVsJyk7XG5cbnZhciBBY3RpdmVTZXNzaW9uID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIGxldCB7bG9naW4sIHBhcnRpZXMsIHNlcnZlcklkfSA9IHRoaXMucHJvcHM7XG4gICAgbGV0IHNlcnZlckxhYmVsVGV4dCA9ICcnO1xuICAgIGlmKHNlcnZlcklkKXtcbiAgICAgIGxldCBob3N0bmFtZSA9IHJlYWN0b3IuZXZhbHVhdGUobm9kZUhvc3ROYW1lQnlTZXJ2ZXJJZChzZXJ2ZXJJZCkpO1xuICAgICAgc2VydmVyTGFiZWxUZXh0ID0gYCR7bG9naW59QCR7aG9zdG5hbWV9YDtcbiAgICB9XG5cbiAgICByZXR1cm4gKFxuICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1jdXJyZW50LXNlc3Npb25cIj5cbiAgICAgICA8U2Vzc2lvbkxlZnRQYW5lbCBwYXJ0aWVzPXtwYXJ0aWVzfS8+XG4gICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtY3VycmVudC1zZXNzaW9uLXNlcnZlci1pbmZvXCI+XG4gICAgICAgICA8aDM+e3NlcnZlckxhYmVsVGV4dH08L2gzPlxuICAgICAgIDwvZGl2PlxuICAgICAgIDxUdHlUZXJtaW5hbCByZWY9XCJ0dHlDbW50SW5zdGFuY2VcIiB7Li4udGhpcy5wcm9wc30gLz5cbiAgICAgPC9kaXY+XG4gICAgICk7XG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IEFjdGl2ZVNlc3Npb247XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9jdXJyZW50U2Vzc2lvbi9hY3RpdmVTZXNzaW9uLmpzeFxuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7Z2V0dGVycywgYWN0aW9uc30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9jdXJyZW50U2Vzc2lvbi8nKTtcbnZhciBTZXNzaW9uUGxheWVyID0gcmVxdWlyZSgnLi9zZXNzaW9uUGxheWVyLmpzeCcpO1xudmFyIEFjdGl2ZVNlc3Npb24gPSByZXF1aXJlKCcuL2FjdGl2ZVNlc3Npb24uanN4Jyk7XG5cbnZhciBDdXJyZW50U2Vzc2lvbkhvc3QgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIGN1cnJlbnRTZXNzaW9uOiBnZXR0ZXJzLmN1cnJlbnRTZXNzaW9uXG4gICAgfVxuICB9LFxuXG4gIGNvbXBvbmVudERpZE1vdW50KCl7XG4gICAgdmFyIHsgc2lkIH0gPSB0aGlzLnByb3BzLnBhcmFtcztcbiAgICBpZighdGhpcy5zdGF0ZS5jdXJyZW50U2Vzc2lvbil7XG4gICAgICBhY3Rpb25zLm9wZW5TZXNzaW9uKHNpZCk7XG4gICAgfVxuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgdmFyIGN1cnJlbnRTZXNzaW9uID0gdGhpcy5zdGF0ZS5jdXJyZW50U2Vzc2lvbjtcbiAgICBpZighY3VycmVudFNlc3Npb24pe1xuICAgICAgcmV0dXJuIG51bGw7XG4gICAgfVxuXG4gICAgaWYoY3VycmVudFNlc3Npb24uaXNOZXdTZXNzaW9uIHx8IGN1cnJlbnRTZXNzaW9uLmFjdGl2ZSl7XG4gICAgICByZXR1cm4gPEFjdGl2ZVNlc3Npb24gey4uLmN1cnJlbnRTZXNzaW9ufS8+O1xuICAgIH1cblxuICAgIHJldHVybiA8U2Vzc2lvblBsYXllciB7Li4uY3VycmVudFNlc3Npb259Lz47XG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IEN1cnJlbnRTZXNzaW9uSG9zdDtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2N1cnJlbnRTZXNzaW9uL21haW4uanN4XG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIFJlYWN0U2xpZGVyID0gcmVxdWlyZSgncmVhY3Qtc2xpZGVyJyk7XG52YXIgVHR5UGxheWVyID0gcmVxdWlyZSgnYXBwL2NvbW1vbi90dHlQbGF5ZXInKVxudmFyIFRlcm1pbmFsID0gcmVxdWlyZSgnYXBwL2NvbW1vbi90ZXJtaW5hbCcpO1xudmFyIFNlc3Npb25MZWZ0UGFuZWwgPSByZXF1aXJlKCcuL3Nlc3Npb25MZWZ0UGFuZWwuanN4Jyk7XG5cbmNsYXNzIE15VGVybWluYWwgZXh0ZW5kcyBUZXJtaW5hbHtcbiAgY29uc3RydWN0b3IodHR5LCBlbCl7XG4gICAgc3VwZXIoe2VsfSk7XG4gICAgdGhpcy50dHkgPSB0dHk7XG4gIH1cblxuICBjb25uZWN0KCl7XG4gICAgdGhpcy50dHkuY29ubmVjdCgpO1xuICB9XG59XG5cbnZhciBUZXJtaW5hbFBsYXllciA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBjb21wb25lbnREaWRNb3VudDogZnVuY3Rpb24oKSB7XG4gICAgdGhpcy50ZXJtaW5hbCA9IG5ldyBNeVRlcm1pbmFsKHRoaXMucHJvcHMudHR5LCB0aGlzLnJlZnMuY29udGFpbmVyKTtcbiAgICB0aGlzLnRlcm1pbmFsLm9wZW4oKTtcbiAgfSxcblxuICBjb21wb25lbnRXaWxsVW5tb3VudDogZnVuY3Rpb24oKSB7XG4gICAgdGhpcy50ZXJtaW5hbC5kZXN0cm95KCk7XG4gIH0sXG5cbiAgc2hvdWxkQ29tcG9uZW50VXBkYXRlOiBmdW5jdGlvbigpIHtcbiAgICByZXR1cm4gZmFsc2U7XG4gIH0sXG5cbiAgcmVuZGVyKCkge1xuICAgIHJldHVybiAoIDxkaXYgcmVmPVwiY29udGFpbmVyXCI+ICA8L2Rpdj4gKTtcbiAgfVxufSk7XG5cbnZhciBTZXNzaW9uUGxheWVyID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICBjYWxjdWxhdGVTdGF0ZSgpe1xuICAgIHJldHVybiB7XG4gICAgICBsZW5ndGg6IHRoaXMudHR5Lmxlbmd0aCxcbiAgICAgIG1pbjogMSxcbiAgICAgIGlzUGxheWluZzogdGhpcy50dHkuaXNQbGF5aW5nLFxuICAgICAgY3VycmVudDogdGhpcy50dHkuY3VycmVudCxcbiAgICAgIGNhblBsYXk6IHRoaXMudHR5Lmxlbmd0aCA+IDFcbiAgICB9O1xuICB9LFxuXG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICB2YXIgc2lkID0gdGhpcy5wcm9wcy5zaWQ7XG4gICAgdGhpcy50dHkgPSBuZXcgVHR5UGxheWVyKHtzaWR9KTtcbiAgICByZXR1cm4gdGhpcy5jYWxjdWxhdGVTdGF0ZSgpO1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50KCkge1xuICAgIHRoaXMudHR5LnN0b3AoKTtcbiAgICB0aGlzLnR0eS5yZW1vdmVBbGxMaXN0ZW5lcnMoKTtcbiAgfSxcblxuICBjb21wb25lbnREaWRNb3VudCgpIHtcbiAgICB0aGlzLnR0eS5vbignY2hhbmdlJywgKCk9PntcbiAgICAgIHZhciBuZXdTdGF0ZSA9IHRoaXMuY2FsY3VsYXRlU3RhdGUoKTtcbiAgICAgIHRoaXMuc2V0U3RhdGUobmV3U3RhdGUpO1xuICAgIH0pO1xuXG4gICAgdGhpcy50dHkucGxheSgpO1xuICB9LFxuXG4gIHRvZ2dsZVBsYXlTdG9wKCl7XG4gICAgaWYodGhpcy5zdGF0ZS5pc1BsYXlpbmcpe1xuICAgICAgdGhpcy50dHkuc3RvcCgpO1xuICAgIH1lbHNle1xuICAgICAgdGhpcy50dHkucGxheSgpO1xuICAgIH1cbiAgfSxcblxuICBtb3ZlKHZhbHVlKXtcbiAgICB0aGlzLnR0eS5tb3ZlKHZhbHVlKTtcbiAgfSxcblxuICBvbkJlZm9yZUNoYW5nZSgpe1xuICAgIHRoaXMudHR5LnN0b3AoKTtcbiAgfSxcblxuICBvbkFmdGVyQ2hhbmdlKHZhbHVlKXtcbiAgICB0aGlzLnR0eS5wbGF5KCk7XG4gICAgdGhpcy50dHkubW92ZSh2YWx1ZSk7XG4gIH0sXG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICB2YXIge2lzUGxheWluZ30gPSB0aGlzLnN0YXRlO1xuXG4gICAgcmV0dXJuIChcbiAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtY3VycmVudC1zZXNzaW9uIGdydi1zZXNzaW9uLXBsYXllclwiPlxuICAgICAgIDxTZXNzaW9uTGVmdFBhbmVsLz5cbiAgICAgICA8VGVybWluYWxQbGF5ZXIgcmVmPVwidGVybVwiIHR0eT17dGhpcy50dHl9IHNjcm9sbGJhY2s9ezB9IC8+XG4gICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtc2Vzc2lvbi1wbGF5ZXItY29udHJvbHNcIj5cbiAgICAgICAgIDxidXR0b24gY2xhc3NOYW1lPVwiYnRuXCIgb25DbGljaz17dGhpcy50b2dnbGVQbGF5U3RvcH0+XG4gICAgICAgICAgIHsgaXNQbGF5aW5nID8gPGkgY2xhc3NOYW1lPVwiZmEgZmEtc3RvcFwiPjwvaT4gOiAgPGkgY2xhc3NOYW1lPVwiZmEgZmEtcGxheVwiPjwvaT4gfVxuICAgICAgICAgPC9idXR0b24+XG4gICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LWNvbHVtblwiPlxuICAgICAgICAgICA8UmVhY3RTbGlkZXJcbiAgICAgICAgICAgICAgbWluPXt0aGlzLnN0YXRlLm1pbn1cbiAgICAgICAgICAgICAgbWF4PXt0aGlzLnN0YXRlLmxlbmd0aH1cbiAgICAgICAgICAgICAgdmFsdWU9e3RoaXMuc3RhdGUuY3VycmVudH1cbiAgICAgICAgICAgICAgb25DaGFuZ2U9e3RoaXMubW92ZX1cbiAgICAgICAgICAgICAgZGVmYXVsdFZhbHVlPXsxfVxuICAgICAgICAgICAgICB3aXRoQmFyc1xuICAgICAgICAgICAgICBjbGFzc05hbWU9XCJncnYtc2xpZGVyXCIgLz5cbiAgICAgICAgIDwvZGl2PlxuICAgICAgICA8L2Rpdj5cbiAgICAgPC9kaXY+XG4gICAgICk7XG4gIH1cbn0pO1xuXG5cblxuZXhwb3J0IGRlZmF1bHQgU2Vzc2lvblBsYXllcjtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2N1cnJlbnRTZXNzaW9uL3Nlc3Npb25QbGF5ZXIuanN4XG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyICQgPSByZXF1aXJlKCdqUXVlcnknKTtcbnZhciBtb21lbnQgPSByZXF1aXJlKCdtb21lbnQnKTtcbnZhciB7ZGVib3VuY2V9ID0gcmVxdWlyZSgnXycpO1xuXG52YXIgRGF0ZVJhbmdlUGlja2VyID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIGdldERhdGVzKCl7XG4gICAgdmFyIHN0YXJ0RGF0ZSA9ICQodGhpcy5yZWZzLmRwUGlja2VyMSkuZGF0ZXBpY2tlcignZ2V0RGF0ZScpO1xuICAgIHZhciBlbmREYXRlID0gJCh0aGlzLnJlZnMuZHBQaWNrZXIyKS5kYXRlcGlja2VyKCdnZXREYXRlJyk7XG4gICAgcmV0dXJuIFtzdGFydERhdGUsIG1vbWVudChlbmREYXRlKS5lbmRPZignZGF5JykudG9EYXRlKCldO1xuICB9LFxuXG4gIHNldERhdGVzKHtzdGFydERhdGUsIGVuZERhdGV9KXtcbiAgICAkKHRoaXMucmVmcy5kcFBpY2tlcjEpLmRhdGVwaWNrZXIoJ3NldERhdGUnLCBzdGFydERhdGUpO1xuICAgICQodGhpcy5yZWZzLmRwUGlja2VyMikuZGF0ZXBpY2tlcignc2V0RGF0ZScsIGVuZERhdGUpO1xuICB9LFxuXG4gIGdldERlZmF1bHRQcm9wcygpIHtcbiAgICAgcmV0dXJuIHtcbiAgICAgICBzdGFydERhdGU6IG1vbWVudCgpLnN0YXJ0T2YoJ21vbnRoJykudG9EYXRlKCksXG4gICAgICAgZW5kRGF0ZTogbW9tZW50KCkuZW5kT2YoJ21vbnRoJykudG9EYXRlKCksXG4gICAgICAgb25DaGFuZ2U6ICgpPT57fVxuICAgICB9O1xuICAgfSxcblxuICBjb21wb25lbnRXaWxsVW5tb3VudCgpe1xuICAgICQodGhpcy5yZWZzLmRwKS5kYXRlcGlja2VyKCdkZXN0cm95Jyk7XG4gIH0sXG5cbiAgY29tcG9uZW50V2lsbFJlY2VpdmVQcm9wcyhuZXdQcm9wcyl7XG4gICAgdmFyIFtzdGFydERhdGUsIGVuZERhdGVdID0gdGhpcy5nZXREYXRlcygpO1xuICAgIGlmKCEoaXNTYW1lKHN0YXJ0RGF0ZSwgbmV3UHJvcHMuc3RhcnREYXRlKSAmJlxuICAgICAgICAgIGlzU2FtZShlbmREYXRlLCBuZXdQcm9wcy5lbmREYXRlKSkpe1xuICAgICAgICB0aGlzLnNldERhdGVzKG5ld1Byb3BzKTtcbiAgICAgIH1cbiAgfSxcblxuICBzaG91bGRDb21wb25lbnRVcGRhdGUoKXtcbiAgICByZXR1cm4gZmFsc2U7XG4gIH0sXG5cbiAgY29tcG9uZW50RGlkTW91bnQoKXtcbiAgICB0aGlzLm9uQ2hhbmdlID0gZGVib3VuY2UodGhpcy5vbkNoYW5nZSwgMSk7XG4gICAgJCh0aGlzLnJlZnMucmFuZ2VQaWNrZXIpLmRhdGVwaWNrZXIoe1xuICAgICAgdG9kYXlCdG46ICdsaW5rZWQnLFxuICAgICAga2V5Ym9hcmROYXZpZ2F0aW9uOiBmYWxzZSxcbiAgICAgIGZvcmNlUGFyc2U6IGZhbHNlLFxuICAgICAgY2FsZW5kYXJXZWVrczogdHJ1ZSxcbiAgICAgIGF1dG9jbG9zZTogdHJ1ZVxuICAgIH0pLm9uKCdjaGFuZ2VEYXRlJywgdGhpcy5vbkNoYW5nZSk7XG5cbiAgICB0aGlzLnNldERhdGVzKHRoaXMucHJvcHMpO1xuICB9LFxuXG4gIG9uQ2hhbmdlKCl7XG4gICAgdmFyIFtzdGFydERhdGUsIGVuZERhdGVdID0gdGhpcy5nZXREYXRlcygpXG4gICAgaWYoIShpc1NhbWUoc3RhcnREYXRlLCB0aGlzLnByb3BzLnN0YXJ0RGF0ZSkgJiZcbiAgICAgICAgICBpc1NhbWUoZW5kRGF0ZSwgdGhpcy5wcm9wcy5lbmREYXRlKSkpe1xuICAgICAgICB0aGlzLnByb3BzLm9uQ2hhbmdlKHtzdGFydERhdGUsIGVuZERhdGV9KTtcbiAgICB9XG4gIH0sXG5cbiAgcmVuZGVyKCkge1xuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1kYXRlcGlja2VyIGlucHV0LWdyb3VwIGlucHV0LWRhdGVyYW5nZVwiIHJlZj1cInJhbmdlUGlja2VyXCI+XG4gICAgICAgIDxpbnB1dCByZWY9XCJkcFBpY2tlcjFcIiB0eXBlPVwidGV4dFwiIGNsYXNzTmFtZT1cImlucHV0LXNtIGZvcm0tY29udHJvbFwiIG5hbWU9XCJzdGFydFwiIC8+XG4gICAgICAgIDxzcGFuIGNsYXNzTmFtZT1cImlucHV0LWdyb3VwLWFkZG9uXCI+dG88L3NwYW4+XG4gICAgICAgIDxpbnB1dCByZWY9XCJkcFBpY2tlcjJcIiB0eXBlPVwidGV4dFwiIGNsYXNzTmFtZT1cImlucHV0LXNtIGZvcm0tY29udHJvbFwiIG5hbWU9XCJlbmRcIiAvPlxuICAgICAgPC9kaXY+XG4gICAgKTtcbiAgfVxufSk7XG5cbmZ1bmN0aW9uIGlzU2FtZShkYXRlMSwgZGF0ZTIpe1xuICByZXR1cm4gbW9tZW50KGRhdGUxKS5pc1NhbWUoZGF0ZTIsICdkYXknKTtcbn1cblxuLyoqXG4qIENhbGVuZGFyIE5hdlxuKi9cbnZhciBDYWxlbmRhck5hdiA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICByZW5kZXIoKSB7XG4gICAgbGV0IHt2YWx1ZX0gPSB0aGlzLnByb3BzO1xuICAgIGxldCBkaXNwbGF5VmFsdWUgPSBtb21lbnQodmFsdWUpLmZvcm1hdCgnTU1NIERvLCBZWVlZJyk7XG5cbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9e1wiZ3J2LWNhbGVuZGFyLW5hdiBcIiArIHRoaXMucHJvcHMuY2xhc3NOYW1lfSA+XG4gICAgICAgIDxidXR0b24gb25DbGljaz17dGhpcy5tb3ZlLmJpbmQodGhpcywgLTEpfSBjbGFzc05hbWU9XCJidG4gYnRuLW91dGxpbmUgYnRuLWxpbmtcIj48aSBjbGFzc05hbWU9XCJmYSBmYS1jaGV2cm9uLWxlZnRcIj48L2k+PC9idXR0b24+XG4gICAgICAgIDxzcGFuIGNsYXNzTmFtZT1cInRleHQtbXV0ZWRcIj57ZGlzcGxheVZhbHVlfTwvc3Bhbj5cbiAgICAgICAgPGJ1dHRvbiBvbkNsaWNrPXt0aGlzLm1vdmUuYmluZCh0aGlzLCAxKX0gY2xhc3NOYW1lPVwiYnRuIGJ0bi1vdXRsaW5lIGJ0bi1saW5rXCI+PGkgY2xhc3NOYW1lPVwiZmEgZmEtY2hldnJvbi1yaWdodFwiPjwvaT48L2J1dHRvbj5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH0sXG5cbiAgbW92ZShhdCl7XG4gICAgbGV0IHt2YWx1ZX0gPSB0aGlzLnByb3BzO1xuICAgIGxldCBuZXdWYWx1ZSA9IG1vbWVudCh2YWx1ZSkuYWRkKGF0LCAnd2VlaycpLnRvRGF0ZSgpO1xuICAgIHRoaXMucHJvcHMub25WYWx1ZUNoYW5nZShuZXdWYWx1ZSk7XG4gIH1cbn0pO1xuXG5DYWxlbmRhck5hdi5nZXR3ZWVrUmFuZ2UgPSBmdW5jdGlvbih2YWx1ZSl7XG4gIGxldCBzdGFydERhdGUgPSBtb21lbnQodmFsdWUpLnN0YXJ0T2YoJ21vbnRoJykudG9EYXRlKCk7XG4gIGxldCBlbmREYXRlID0gbW9tZW50KHZhbHVlKS5lbmRPZignbW9udGgnKS50b0RhdGUoKTtcbiAgcmV0dXJuIFtzdGFydERhdGUsIGVuZERhdGVdO1xufVxuXG5leHBvcnQgZGVmYXVsdCBEYXRlUmFuZ2VQaWNrZXI7XG5leHBvcnQge0NhbGVuZGFyTmF2LCBEYXRlUmFuZ2VQaWNrZXJ9O1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvZGF0ZVBpY2tlci5qc3hcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbm1vZHVsZS5leHBvcnRzLkFwcCA9IHJlcXVpcmUoJy4vYXBwLmpzeCcpO1xubW9kdWxlLmV4cG9ydHMuTG9naW4gPSByZXF1aXJlKCcuL2xvZ2luLmpzeCcpO1xubW9kdWxlLmV4cG9ydHMuTmV3VXNlciA9IHJlcXVpcmUoJy4vbmV3VXNlci5qc3gnKTtcbm1vZHVsZS5leHBvcnRzLk5vZGVzID0gcmVxdWlyZSgnLi9ub2Rlcy9tYWluLmpzeCcpO1xubW9kdWxlLmV4cG9ydHMuU2Vzc2lvbnMgPSByZXF1aXJlKCcuL3Nlc3Npb25zL21haW4uanN4Jyk7XG5tb2R1bGUuZXhwb3J0cy5DdXJyZW50U2Vzc2lvbkhvc3QgPSByZXF1aXJlKCcuL2N1cnJlbnRTZXNzaW9uL21haW4uanN4Jyk7XG5tb2R1bGUuZXhwb3J0cy5FcnJvclBhZ2UgPSByZXF1aXJlKCcuL21zZ1BhZ2UuanN4JykuRXJyb3JQYWdlO1xubW9kdWxlLmV4cG9ydHMuTm90Rm91bmQgPSByZXF1aXJlKCcuL21zZ1BhZ2UuanN4JykuTm90Rm91bmQ7XG5tb2R1bGUuZXhwb3J0cy5NZXNzYWdlUGFnZSA9IHJlcXVpcmUoJy4vbXNnUGFnZS5qc3gnKS5NZXNzYWdlUGFnZTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2luZGV4LmpzeFxuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciAkID0gcmVxdWlyZSgnalF1ZXJ5Jyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgTGlua2VkU3RhdGVNaXhpbiA9IHJlcXVpcmUoJ3JlYWN0LWFkZG9ucy1saW5rZWQtc3RhdGUtbWl4aW4nKTtcbnZhciB7YWN0aW9ucywgZ2V0dGVyc30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy91c2VyJyk7XG52YXIgR29vZ2xlQXV0aEluZm8gPSByZXF1aXJlKCcuL2dvb2dsZUF1dGhMb2dvJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xudmFyIHtUZWxlcG9ydExvZ299ID0gcmVxdWlyZSgnLi9pY29ucy5qc3gnKTtcbnZhciB7UFJPVklERVJfR09PR0xFfSA9IHJlcXVpcmUoJ2FwcC9zZXJ2aWNlcy9hdXRoJyk7XG5cbnZhciBMb2dpbklucHV0Rm9ybSA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBtaXhpbnM6IFtMaW5rZWRTdGF0ZU1peGluXSxcblxuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIHVzZXI6ICcnLFxuICAgICAgcGFzc3dvcmQ6ICcnLFxuICAgICAgdG9rZW46ICcnLFxuICAgICAgcHJvdmlkZXI6IG51bGxcbiAgICB9XG4gIH0sXG5cblxuICBvbkxvZ2luKGUpe1xuICAgIGUucHJldmVudERlZmF1bHQoKTtcbiAgICBpZiAodGhpcy5pc1ZhbGlkKCkpIHtcbiAgICAgIHRoaXMucHJvcHMub25DbGljayh0aGlzLnN0YXRlKTtcbiAgICB9XG4gIH0sXG5cbiAgb25Mb2dpbldpdGhHb29nbGU6IGZ1bmN0aW9uKGUpIHtcbiAgICBlLnByZXZlbnREZWZhdWx0KCk7XG4gICAgdGhpcy5zdGF0ZS5wcm92aWRlciA9IFBST1ZJREVSX0dPT0dMRTtcbiAgICB0aGlzLnByb3BzLm9uQ2xpY2sodGhpcy5zdGF0ZSk7XG4gIH0sXG5cbiAgaXNWYWxpZDogZnVuY3Rpb24oKSB7XG4gICAgdmFyICRmb3JtID0gJCh0aGlzLnJlZnMuZm9ybSk7XG4gICAgcmV0dXJuICRmb3JtLmxlbmd0aCA9PT0gMCB8fCAkZm9ybS52YWxpZCgpO1xuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICBsZXQge2lzUHJvY2Vzc2luZywgaXNGYWlsZWQsIG1lc3NhZ2UgfSA9IHRoaXMucHJvcHMuYXR0ZW1wO1xuICAgIGxldCBwcm92aWRlcnMgPSBjZmcuZ2V0QXV0aFByb3ZpZGVycygpO1xuICAgIGxldCB1c2VHb29nbGUgPSBwcm92aWRlcnMuaW5kZXhPZihQUk9WSURFUl9HT09HTEUpICE9PSAtMTtcblxuICAgIHJldHVybiAoXG4gICAgICA8Zm9ybSByZWY9XCJmb3JtXCIgY2xhc3NOYW1lPVwiZ3J2LWxvZ2luLWlucHV0LWZvcm1cIj5cbiAgICAgICAgPGgzPiBXZWxjb21lIHRvIFRlbGVwb3J0IDwvaDM+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiXCI+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXQgYXV0b0ZvY3VzIHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3VzZXInKX0gY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sIHJlcXVpcmVkXCIgcGxhY2Vob2xkZXI9XCJVc2VyIG5hbWVcIiBuYW1lPVwidXNlck5hbWVcIiAvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0IHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3Bhc3N3b3JkJyl9IHR5cGU9XCJwYXNzd29yZFwiIG5hbWU9XCJwYXNzd29yZFwiIGNsYXNzTmFtZT1cImZvcm0tY29udHJvbCByZXF1aXJlZFwiIHBsYWNlaG9sZGVyPVwiUGFzc3dvcmRcIi8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXQgYXV0b0NvbXBsZXRlPVwib2ZmXCIgdmFsdWVMaW5rPXt0aGlzLmxpbmtTdGF0ZSgndG9rZW4nKX0gY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sIHJlcXVpcmVkXCIgbmFtZT1cInRva2VuXCIgcGxhY2Vob2xkZXI9XCJUd28gZmFjdG9yIHRva2VuIChHb29nbGUgQXV0aGVudGljYXRvcilcIi8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGJ1dHRvbiBvbkNsaWNrPXt0aGlzLm9uTG9naW59IGRpc2FibGVkPXtpc1Byb2Nlc3Npbmd9IHR5cGU9XCJzdWJtaXRcIiBjbGFzc05hbWU9XCJidG4gYnRuLXByaW1hcnkgYmxvY2sgZnVsbC13aWR0aCBtLWJcIj5Mb2dpbjwvYnV0dG9uPlxuICAgICAgICAgIHsgdXNlR29vZ2xlID8gPGJ1dHRvbiBvbkNsaWNrPXt0aGlzLm9uTG9naW5XaXRoR29vZ2xlfSB0eXBlPVwic3VibWl0XCIgY2xhc3NOYW1lPVwiYnRuIGJ0bi1kYW5nZXIgYmxvY2sgZnVsbC13aWR0aCBtLWJcIj5XaXRoIEdvb2dsZTwvYnV0dG9uPiA6IG51bGwgfVxuICAgICAgICAgIHsgaXNGYWlsZWQgPyAoPGxhYmVsIGNsYXNzTmFtZT1cImVycm9yXCI+e21lc3NhZ2V9PC9sYWJlbD4pIDogbnVsbCB9XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9mb3JtPlxuICAgICk7XG4gIH1cbn0pXG5cbnZhciBMb2dpbiA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBtaXhpbnM6IFtyZWFjdG9yLlJlYWN0TWl4aW5dLFxuXG4gIGdldERhdGFCaW5kaW5ncygpIHtcbiAgICByZXR1cm4ge1xuICAgICAgYXR0ZW1wOiBnZXR0ZXJzLmxvZ2luQXR0ZW1wXG4gICAgfVxuICB9LFxuXG4gIG9uQ2xpY2soaW5wdXREYXRhKXtcbiAgICB2YXIgbG9jID0gdGhpcy5wcm9wcy5sb2NhdGlvbjtcbiAgICB2YXIgcmVkaXJlY3QgPSBjZmcucm91dGVzLmFwcDtcblxuICAgIGlmKGxvYy5zdGF0ZSAmJiBsb2Muc3RhdGUucmVkaXJlY3RUbyl7XG4gICAgICByZWRpcmVjdCA9IGxvYy5zdGF0ZS5yZWRpcmVjdFRvO1xuICAgIH1cblxuICAgIGFjdGlvbnMubG9naW4oaW5wdXREYXRhLCByZWRpcmVjdCk7XG4gIH0sXG5cbiAgcmVuZGVyKCkge1xuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1sb2dpbiB0ZXh0LWNlbnRlclwiPlxuICAgICAgICA8VGVsZXBvcnRMb2dvLz5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtY29udGVudCBncnYtZmxleFwiPlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWZsZXgtY29sdW1uXCI+XG4gICAgICAgICAgICA8TG9naW5JbnB1dEZvcm0gYXR0ZW1wPXt0aGlzLnN0YXRlLmF0dGVtcH0gb25DbGljaz17dGhpcy5vbkNsaWNrfS8+XG4gICAgICAgICAgICA8R29vZ2xlQXV0aEluZm8vPlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbG9naW4taW5mb1wiPlxuICAgICAgICAgICAgICA8aSBjbGFzc05hbWU9XCJmYSBmYS1xdWVzdGlvblwiPjwvaT5cbiAgICAgICAgICAgICAgPHN0cm9uZz5OZXcgQWNjb3VudCBvciBmb3Jnb3QgcGFzc3dvcmQ/PC9zdHJvbmc+XG4gICAgICAgICAgICAgIDxkaXY+QXNrIGZvciBhc3Npc3RhbmNlIGZyb20geW91ciBDb21wYW55IGFkbWluaXN0cmF0b3I8L2Rpdj5cbiAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IExvZ2luO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvbG9naW4uanN4XG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xudmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7IEluZGV4TGluayB9ID0gcmVxdWlyZSgncmVhY3Qtcm91dGVyJyk7XG52YXIgZ2V0dGVycyA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3VzZXIvZ2V0dGVycycpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcbnZhciB7VXNlckljb259ID0gcmVxdWlyZSgnLi9pY29ucy5qc3gnKTtcblxudmFyIG1lbnVJdGVtcyA9IFtcbiAge2ljb246ICdmYSBmYS1zaGFyZS1hbHQnLCB0bzogY2ZnLnJvdXRlcy5ub2RlcywgdGl0bGU6ICdOb2Rlcyd9LFxuICB7aWNvbjogJ2ZhICBmYS1ncm91cCcsIHRvOiBjZmcucm91dGVzLnNlc3Npb25zLCB0aXRsZTogJ1Nlc3Npb25zJ31cbl07XG5cbnZhciBOYXZMZWZ0QmFyID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICByZW5kZXI6IGZ1bmN0aW9uKCl7XG4gICAgdmFyIHtuYW1lfSA9IHJlYWN0b3IuZXZhbHVhdGUoZ2V0dGVycy51c2VyKTtcbiAgICB2YXIgaXRlbXMgPSBtZW51SXRlbXMubWFwKChpLCBpbmRleCk9PntcbiAgICAgIHZhciBjbGFzc05hbWUgPSB0aGlzLmNvbnRleHQucm91dGVyLmlzQWN0aXZlKGkudG8pID8gJ2FjdGl2ZScgOiAnJztcbiAgICAgIHJldHVybiAoXG4gICAgICAgIDxsaSBrZXk9e2luZGV4fSBjbGFzc05hbWU9e2NsYXNzTmFtZX0gdGl0bGU9e2kudGl0bGV9PlxuICAgICAgICAgIDxJbmRleExpbmsgdG89e2kudG99PlxuICAgICAgICAgICAgPGkgY2xhc3NOYW1lPXtpLmljb259IC8+XG4gICAgICAgICAgPC9JbmRleExpbms+XG4gICAgICAgIDwvbGk+XG4gICAgICApO1xuICAgIH0pO1xuXG4gICAgaXRlbXMucHVzaCgoXG4gICAgICA8bGkga2V5PXtpdGVtcy5sZW5ndGh9IHRpdGxlPVwiaGVscFwiPlxuICAgICAgICA8YSBocmVmPXtjZmcuaGVscFVybH0gdGFyZ2V0PVwiX2JsYW5rXCI+XG4gICAgICAgICAgPGkgY2xhc3NOYW1lPVwiZmEgZmEtcXVlc3Rpb25cIiAvPlxuICAgICAgICA8L2E+XG4gICAgICA8L2xpPikpO1xuXG4gICAgaXRlbXMucHVzaCgoXG4gICAgICA8bGkga2V5PXtpdGVtcy5sZW5ndGh9IHRpdGxlPVwibG9nb3V0XCI+XG4gICAgICAgIDxhIGhyZWY9e2NmZy5yb3V0ZXMubG9nb3V0fT5cbiAgICAgICAgICA8aSBjbGFzc05hbWU9XCJmYSBmYS1zaWduLW91dFwiIHN0eWxlPXt7bWFyZ2luUmlnaHQ6IDB9fT48L2k+XG4gICAgICAgIDwvYT5cbiAgICAgIDwvbGk+XG4gICAgKSk7XG5cbiAgICByZXR1cm4gKFxuICAgICAgPG5hdiBjbGFzc05hbWU9J2dydi1uYXYgbmF2YmFyLWRlZmF1bHQnIHJvbGU9J25hdmlnYXRpb24nPlxuICAgICAgICA8dWwgY2xhc3NOYW1lPSduYXYgdGV4dC1jZW50ZXInIGlkPSdzaWRlLW1lbnUnPlxuICAgICAgICAgIDxsaT5cbiAgICAgICAgICAgIDxVc2VySWNvbiBuYW1lPXtuYW1lfSAvPlxuICAgICAgICAgIDwvbGk+XG4gICAgICAgICAge2l0ZW1zfVxuICAgICAgICA8L3VsPlxuICAgICAgPC9uYXY+XG4gICAgKTtcbiAgfVxufSk7XG5cbk5hdkxlZnRCYXIuY29udGV4dFR5cGVzID0ge1xuICByb3V0ZXI6IFJlYWN0LlByb3BUeXBlcy5vYmplY3QuaXNSZXF1aXJlZFxufVxuXG5tb2R1bGUuZXhwb3J0cyA9IE5hdkxlZnRCYXI7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9uYXZMZWZ0QmFyLmpzeFxuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciAkID0gcmVxdWlyZSgnalF1ZXJ5Jyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIge2FjdGlvbnMsIGdldHRlcnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvdXNlcicpO1xudmFyIExpbmtlZFN0YXRlTWl4aW4gPSByZXF1aXJlKCdyZWFjdC1hZGRvbnMtbGlua2VkLXN0YXRlLW1peGluJyk7XG52YXIgR29vZ2xlQXV0aEluZm8gPSByZXF1aXJlKCcuL2dvb2dsZUF1dGhMb2dvJyk7XG52YXIge0Vycm9yUGFnZSwgRXJyb3JUeXBlc30gPSByZXF1aXJlKCcuL21zZ1BhZ2UnKTtcbnZhciB7VGVsZXBvcnRMb2dvfSA9IHJlcXVpcmUoJy4vaWNvbnMuanN4Jyk7XG5cbnZhciBJbnZpdGVJbnB1dEZvcm0gPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbTGlua2VkU3RhdGVNaXhpbl0sXG5cbiAgY29tcG9uZW50RGlkTW91bnQoKXtcbiAgICAkKHRoaXMucmVmcy5mb3JtKS52YWxpZGF0ZSh7XG4gICAgICBydWxlczp7XG4gICAgICAgIHBhc3N3b3JkOntcbiAgICAgICAgICBtaW5sZW5ndGg6IDYsXG4gICAgICAgICAgcmVxdWlyZWQ6IHRydWVcbiAgICAgICAgfSxcbiAgICAgICAgcGFzc3dvcmRDb25maXJtZWQ6e1xuICAgICAgICAgIHJlcXVpcmVkOiB0cnVlLFxuICAgICAgICAgIGVxdWFsVG86IHRoaXMucmVmcy5wYXNzd29yZFxuICAgICAgICB9XG4gICAgICB9LFxuXG4gICAgICBtZXNzYWdlczoge1xuICBcdFx0XHRwYXNzd29yZENvbmZpcm1lZDoge1xuICBcdFx0XHRcdG1pbmxlbmd0aDogJC52YWxpZGF0b3IuZm9ybWF0KCdFbnRlciBhdCBsZWFzdCB7MH0gY2hhcmFjdGVycycpLFxuICBcdFx0XHRcdGVxdWFsVG86ICdFbnRlciB0aGUgc2FtZSBwYXNzd29yZCBhcyBhYm92ZSdcbiAgXHRcdFx0fVxuICAgICAgfVxuICAgIH0pXG4gIH0sXG5cbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB7XG4gICAgICBuYW1lOiB0aGlzLnByb3BzLmludml0ZS51c2VyLFxuICAgICAgcHN3OiAnJyxcbiAgICAgIHBzd0NvbmZpcm1lZDogJycsXG4gICAgICB0b2tlbjogJydcbiAgICB9XG4gIH0sXG5cbiAgb25DbGljayhlKSB7XG4gICAgZS5wcmV2ZW50RGVmYXVsdCgpO1xuICAgIGlmICh0aGlzLmlzVmFsaWQoKSkge1xuICAgICAgYWN0aW9ucy5zaWduVXAoe1xuICAgICAgICBuYW1lOiB0aGlzLnN0YXRlLm5hbWUsXG4gICAgICAgIHBzdzogdGhpcy5zdGF0ZS5wc3csXG4gICAgICAgIHRva2VuOiB0aGlzLnN0YXRlLnRva2VuLFxuICAgICAgICBpbnZpdGVUb2tlbjogdGhpcy5wcm9wcy5pbnZpdGUuaW52aXRlX3Rva2VufSk7XG4gICAgfVxuICB9LFxuXG4gIGlzVmFsaWQoKSB7XG4gICAgdmFyICRmb3JtID0gJCh0aGlzLnJlZnMuZm9ybSk7XG4gICAgcmV0dXJuICRmb3JtLmxlbmd0aCA9PT0gMCB8fCAkZm9ybS52YWxpZCgpO1xuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICBsZXQge2lzUHJvY2Vzc2luZywgaXNGYWlsZWQsIG1lc3NhZ2UgfSA9IHRoaXMucHJvcHMuYXR0ZW1wO1xuICAgIHJldHVybiAoXG4gICAgICA8Zm9ybSByZWY9XCJmb3JtXCIgY2xhc3NOYW1lPVwiZ3J2LWludml0ZS1pbnB1dC1mb3JtXCI+XG4gICAgICAgIDxoMz4gR2V0IHN0YXJ0ZWQgd2l0aCBUZWxlcG9ydCA8L2gzPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0XG4gICAgICAgICAgICAgIGRpc2FibGVkXG4gICAgICAgICAgICAgIHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ25hbWUnKX1cbiAgICAgICAgICAgICAgbmFtZT1cInVzZXJOYW1lXCJcbiAgICAgICAgICAgICAgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sIHJlcXVpcmVkXCJcbiAgICAgICAgICAgICAgcGxhY2Vob2xkZXI9XCJVc2VyIG5hbWVcIi8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXRcbiAgICAgICAgICAgICAgYXV0b0ZvY3VzXG4gICAgICAgICAgICAgIHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3BzdycpfVxuICAgICAgICAgICAgICByZWY9XCJwYXNzd29yZFwiXG4gICAgICAgICAgICAgIHR5cGU9XCJwYXNzd29yZFwiXG4gICAgICAgICAgICAgIG5hbWU9XCJwYXNzd29yZFwiXG4gICAgICAgICAgICAgIGNsYXNzTmFtZT1cImZvcm0tY29udHJvbFwiXG4gICAgICAgICAgICAgIHBsYWNlaG9sZGVyPVwiUGFzc3dvcmRcIiAvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0XG4gICAgICAgICAgICAgIHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3Bzd0NvbmZpcm1lZCcpfVxuICAgICAgICAgICAgICB0eXBlPVwicGFzc3dvcmRcIlxuICAgICAgICAgICAgICBuYW1lPVwicGFzc3dvcmRDb25maXJtZWRcIlxuICAgICAgICAgICAgICBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2xcIlxuICAgICAgICAgICAgICBwbGFjZWhvbGRlcj1cIlBhc3N3b3JkIGNvbmZpcm1cIi8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXRcbiAgICAgICAgICAgICAgYXV0b0NvbXBsZXRlPVwib2ZmXCJcbiAgICAgICAgICAgICAgbmFtZT1cInRva2VuXCJcbiAgICAgICAgICAgICAgdmFsdWVMaW5rPXt0aGlzLmxpbmtTdGF0ZSgndG9rZW4nKX1cbiAgICAgICAgICAgICAgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sIHJlcXVpcmVkXCJcbiAgICAgICAgICAgICAgcGxhY2Vob2xkZXI9XCJUd28gZmFjdG9yIHRva2VuIChHb29nbGUgQXV0aGVudGljYXRvcilcIiAvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxidXR0b24gdHlwZT1cInN1Ym1pdFwiIGRpc2FibGVkPXtpc1Byb2Nlc3Npbmd9IGNsYXNzTmFtZT1cImJ0biBidG4tcHJpbWFyeSBibG9jayBmdWxsLXdpZHRoIG0tYlwiIG9uQ2xpY2s9e3RoaXMub25DbGlja30gPlNpZ24gdXA8L2J1dHRvbj5cbiAgICAgICAgICB7IGlzRmFpbGVkID8gKDxsYWJlbCBjbGFzc05hbWU9XCJlcnJvclwiPnttZXNzYWdlfTwvbGFiZWw+KSA6IG51bGwgfVxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZm9ybT5cbiAgICApO1xuICB9XG59KVxuXG52YXIgSW52aXRlID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW3JlYWN0b3IuUmVhY3RNaXhpbl0sXG5cbiAgZ2V0RGF0YUJpbmRpbmdzKCkge1xuICAgIHJldHVybiB7XG4gICAgICBpbnZpdGU6IGdldHRlcnMuaW52aXRlLFxuICAgICAgYXR0ZW1wOiBnZXR0ZXJzLmF0dGVtcCxcbiAgICAgIGZldGNoaW5nSW52aXRlOiBnZXR0ZXJzLmZldGNoaW5nSW52aXRlXG4gICAgfVxuICB9LFxuXG4gIGNvbXBvbmVudERpZE1vdW50KCl7XG4gICAgYWN0aW9ucy5mZXRjaEludml0ZSh0aGlzLnByb3BzLnBhcmFtcy5pbnZpdGVUb2tlbik7XG4gIH0sXG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICBsZXQge2ZldGNoaW5nSW52aXRlLCBpbnZpdGUsIGF0dGVtcH0gPSB0aGlzLnN0YXRlO1xuXG4gICAgaWYoZmV0Y2hpbmdJbnZpdGUuaXNGYWlsZWQpe1xuICAgICAgcmV0dXJuIDxFcnJvclBhZ2UgdHlwZT17RXJyb3JUeXBlcy5FWFBJUkVEX0lOVklURX0vPlxuICAgIH1cblxuICAgIGlmKCFpbnZpdGUpIHtcbiAgICAgIHJldHVybiBudWxsO1xuICAgIH1cblxuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1pbnZpdGUgdGV4dC1jZW50ZXJcIj5cbiAgICAgICAgPFRlbGVwb3J0TG9nby8+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWNvbnRlbnQgZ3J2LWZsZXhcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LWNvbHVtblwiPlxuICAgICAgICAgICAgPEludml0ZUlucHV0Rm9ybSBhdHRlbXA9e2F0dGVtcH0gaW52aXRlPXtpbnZpdGUudG9KUygpfS8+XG4gICAgICAgICAgICA8R29vZ2xlQXV0aEluZm8vPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWZsZXgtY29sdW1uIGdydi1pbnZpdGUtYmFyY29kZVwiPlxuICAgICAgICAgICAgPGg0PlNjYW4gYmFyIGNvZGUgZm9yIGF1dGggdG9rZW4gPGJyLz4gPHNtYWxsPlNjYW4gYmVsb3cgdG8gZ2VuZXJhdGUgeW91ciB0d28gZmFjdG9yIHRva2VuPC9zbWFsbD48L2g0PlxuICAgICAgICAgICAgPGltZyBjbGFzc05hbWU9XCJpbWctdGh1bWJuYWlsXCIgc3JjPXsgYGRhdGE6aW1hZ2UvcG5nO2Jhc2U2NCwke2ludml0ZS5nZXQoJ3FyJyl9YCB9IC8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9kaXY+XG4gICAgKTtcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gSW52aXRlO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvbmV3VXNlci5qc3hcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgdXNlckdldHRlcnMgPSByZXF1aXJlKCdhcHAvbW9kdWxlcy91c2VyL2dldHRlcnMnKTtcbnZhciBub2RlR2V0dGVycyA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL25vZGVzL2dldHRlcnMnKTtcbnZhciBOb2RlTGlzdCA9IHJlcXVpcmUoJy4vbm9kZUxpc3QuanN4Jyk7XG5cbnZhciBOb2RlcyA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBtaXhpbnM6IFtyZWFjdG9yLlJlYWN0TWl4aW5dLFxuXG4gIGdldERhdGFCaW5kaW5ncygpIHtcbiAgICByZXR1cm4ge1xuICAgICAgbm9kZVJlY29yZHM6IG5vZGVHZXR0ZXJzLm5vZGVMaXN0VmlldyxcbiAgICAgIHVzZXI6IHVzZXJHZXR0ZXJzLnVzZXJcbiAgICB9XG4gIH0sXG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICB2YXIgbm9kZVJlY29yZHMgPSB0aGlzLnN0YXRlLm5vZGVSZWNvcmRzO1xuICAgIHZhciBsb2dpbnMgPSB0aGlzLnN0YXRlLnVzZXIubG9naW5zO1xuICAgIHJldHVybiAoIDxOb2RlTGlzdCBub2RlUmVjb3Jkcz17bm9kZVJlY29yZHN9IGxvZ2lucz17bG9naW5zfS8+ICk7XG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IE5vZGVzO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvbm9kZXMvbWFpbi5qc3hcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgUHVyZVJlbmRlck1peGluID0gcmVxdWlyZSgncmVhY3QtYWRkb25zLXB1cmUtcmVuZGVyLW1peGluJyk7XG52YXIge2xhc3RNZXNzYWdlfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL25vdGlmaWNhdGlvbnMvZ2V0dGVycycpO1xudmFyIHtUb2FzdENvbnRhaW5lciwgVG9hc3RNZXNzYWdlfSA9IHJlcXVpcmUoXCJyZWFjdC10b2FzdHJcIik7XG52YXIgVG9hc3RNZXNzYWdlRmFjdG9yeSA9IFJlYWN0LmNyZWF0ZUZhY3RvcnkoVG9hc3RNZXNzYWdlLmFuaW1hdGlvbik7XG5cbmNvbnN0IGFuaW1hdGlvbk9wdGlvbnMgPSB7XG4gIHNob3dBbmltYXRpb246ICdhbmltYXRlZCBmYWRlSW4nLFxuICBoaWRlQW5pbWF0aW9uOiAnYW5pbWF0ZWQgZmFkZU91dCdcbn1cblxudmFyIE5vdGlmaWNhdGlvbkhvc3QgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbXG4gICAgcmVhY3Rvci5SZWFjdE1peGluLCBQdXJlUmVuZGVyTWl4aW5cbiAgXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHttc2c6IGxhc3RNZXNzYWdlfVxuICB9LFxuXG4gIHVwZGF0ZShtc2cpIHtcbiAgICBpZiAobXNnKSB7XG4gICAgICBpZiAobXNnLmlzRXJyb3IpIHtcbiAgICAgICAgdGhpcy5yZWZzLmNvbnRhaW5lci5lcnJvcihtc2cudGV4dCwgbXNnLnRpdGxlLCBhbmltYXRpb25PcHRpb25zKTtcbiAgICAgIH0gZWxzZSBpZiAobXNnLmlzV2FybmluZykge1xuICAgICAgICB0aGlzLnJlZnMuY29udGFpbmVyLndhcm5pbmcobXNnLnRleHQsIG1zZy50aXRsZSwgYW5pbWF0aW9uT3B0aW9ucyk7XG4gICAgICB9IGVsc2UgaWYgKG1zZy5pc1N1Y2Nlc3MpIHtcbiAgICAgICAgdGhpcy5yZWZzLmNvbnRhaW5lci5zdWNjZXNzKG1zZy50ZXh0LCBtc2cudGl0bGUsIGFuaW1hdGlvbk9wdGlvbnMpO1xuICAgICAgfSBlbHNlIHtcbiAgICAgICAgdGhpcy5yZWZzLmNvbnRhaW5lci5pbmZvKG1zZy50ZXh0LCBtc2cudGl0bGUsIGFuaW1hdGlvbk9wdGlvbnMpO1xuICAgICAgfVxuICAgIH1cbiAgfSxcblxuICBjb21wb25lbnREaWRNb3VudCgpIHtcbiAgICByZWFjdG9yLm9ic2VydmUobGFzdE1lc3NhZ2UsIHRoaXMudXBkYXRlKVxuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50KCkge1xuICAgIHJlYWN0b3IudW5vYnNlcnZlKGxhc3RNZXNzYWdlLCB0aGlzLnVwZGF0ZSk7XG4gIH0sXG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICByZXR1cm4gKFxuICAgICAgICA8VG9hc3RDb250YWluZXJcbiAgICAgICAgICByZWY9XCJjb250YWluZXJcIiB0b2FzdE1lc3NhZ2VGYWN0b3J5PXtUb2FzdE1lc3NhZ2VGYWN0b3J5fSBjbGFzc05hbWU9XCJ0b2FzdC10b3AtcmlnaHRcIi8+XG4gICAgKTtcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gTm90aWZpY2F0aW9uSG9zdDtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL25vdGlmaWNhdGlvbkhvc3QuanN4XG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHtnZXR0ZXJzfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2RpYWxvZ3MnKTtcbnZhciB7Y2xvc2VTZWxlY3ROb2RlRGlhbG9nfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL2RpYWxvZ3MvYWN0aW9ucycpO1xudmFyIE5vZGVMaXN0ID0gcmVxdWlyZSgnLi9ub2Rlcy9ub2RlTGlzdC5qc3gnKTtcbnZhciBjdXJyZW50U2Vzc2lvbkdldHRlcnMgPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9jdXJyZW50U2Vzc2lvbi9nZXR0ZXJzJyk7XG52YXIgbm9kZUdldHRlcnMgPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9ub2Rlcy9nZXR0ZXJzJyk7XG52YXIgJCA9IHJlcXVpcmUoJ2pRdWVyeScpO1xuXG52YXIgU2VsZWN0Tm9kZURpYWxvZyA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBtaXhpbnM6IFtyZWFjdG9yLlJlYWN0TWl4aW5dLFxuXG4gIGdldERhdGFCaW5kaW5ncygpIHtcbiAgICByZXR1cm4ge1xuICAgICAgZGlhbG9nczogZ2V0dGVycy5kaWFsb2dzXG4gICAgfVxuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICByZXR1cm4gdGhpcy5zdGF0ZS5kaWFsb2dzLmlzU2VsZWN0Tm9kZURpYWxvZ09wZW4gPyA8RGlhbG9nLz4gOiBudWxsO1xuICB9XG59KTtcblxudmFyIERpYWxvZyA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBvbkxvZ2luQ2xpY2soc2VydmVySWQpe1xuICAgIGlmKFNlbGVjdE5vZGVEaWFsb2cub25TZXJ2ZXJDaGFuZ2VDYWxsQmFjayl7XG4gICAgICBTZWxlY3ROb2RlRGlhbG9nLm9uU2VydmVyQ2hhbmdlQ2FsbEJhY2soe3NlcnZlcklkfSk7XG4gICAgfVxuXG4gICAgY2xvc2VTZWxlY3ROb2RlRGlhbG9nKCk7XG4gIH0sXG5cbiAgY29tcG9uZW50V2lsbFVubW91bnQoKXtcbiAgICAkKCcubW9kYWwnKS5tb2RhbCgnaGlkZScpO1xuICB9LFxuXG4gIGNvbXBvbmVudERpZE1vdW50KCl7XG4gICAgJCgnLm1vZGFsJykubW9kYWwoJ3Nob3cnKTtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgdmFyIGFjdGl2ZVNlc3Npb24gPSByZWFjdG9yLmV2YWx1YXRlKGN1cnJlbnRTZXNzaW9uR2V0dGVycy5jdXJyZW50U2Vzc2lvbikgfHwge307XG4gICAgdmFyIG5vZGVSZWNvcmRzID0gcmVhY3Rvci5ldmFsdWF0ZShub2RlR2V0dGVycy5ub2RlTGlzdFZpZXcpO1xuICAgIHZhciBsb2dpbnMgPSBbYWN0aXZlU2Vzc2lvbi5sb2dpbl07XG5cbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJtb2RhbCBmYWRlIGdydi1kaWFsb2ctc2VsZWN0LW5vZGVcIiB0YWJJbmRleD17LTF9IHJvbGU9XCJkaWFsb2dcIj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJtb2RhbC1kaWFsb2dcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIm1vZGFsLWNvbnRlbnRcIj5cbiAgICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwibW9kYWwtaGVhZGVyXCI+XG4gICAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwibW9kYWwtYm9keVwiPlxuICAgICAgICAgICAgICA8Tm9kZUxpc3Qgbm9kZVJlY29yZHM9e25vZGVSZWNvcmRzfSBsb2dpbnM9e2xvZ2luc30gb25Mb2dpbkNsaWNrPXt0aGlzLm9uTG9naW5DbGlja30vPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIm1vZGFsLWZvb3RlclwiPlxuICAgICAgICAgICAgICA8YnV0dG9uIG9uQ2xpY2s9e2Nsb3NlU2VsZWN0Tm9kZURpYWxvZ30gdHlwZT1cImJ1dHRvblwiIGNsYXNzTmFtZT1cImJ0biBidG4tcHJpbWFyeVwiPlxuICAgICAgICAgICAgICAgIENsb3NlXG4gICAgICAgICAgICAgIDwvYnV0dG9uPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9kaXY+XG4gICAgKTtcbiAgfVxufSk7XG5cblNlbGVjdE5vZGVEaWFsb2cub25TZXJ2ZXJDaGFuZ2VDYWxsQmFjayA9ICgpPT57fTtcblxubW9kdWxlLmV4cG9ydHMgPSBTZWxlY3ROb2RlRGlhbG9nO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvc2VsZWN0Tm9kZURpYWxvZy5qc3hcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIge1RhYmxlLCBDb2x1bW4sIENlbGwsIFRleHRDZWxsLCBFbXB0eUluZGljYXRvcn0gPSByZXF1aXJlKCdhcHAvY29tcG9uZW50cy90YWJsZS5qc3gnKTtcbnZhciB7QnV0dG9uQ2VsbCwgVXNlcnNDZWxsLCBOb2RlQ2VsbCwgRGF0ZUNyZWF0ZWRDZWxsfSA9IHJlcXVpcmUoJy4vbGlzdEl0ZW1zJyk7XG5cbnZhciBBY3RpdmVTZXNzaW9uTGlzdCA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICBsZXQgZGF0YSA9IHRoaXMucHJvcHMuZGF0YS5maWx0ZXIoaXRlbSA9PiBpdGVtLmFjdGl2ZSk7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXNlc3Npb25zLWFjdGl2ZVwiPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1oZWFkZXJcIj5cbiAgICAgICAgICA8aDE+IEFjdGl2ZSBTZXNzaW9ucyA8L2gxPlxuICAgICAgICA8L2Rpdj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtY29udGVudFwiPlxuICAgICAgICAgIHtkYXRhLmxlbmd0aCA9PT0gMCA/IDxFbXB0eUluZGljYXRvciB0ZXh0PVwiWW91IGhhdmUgbm8gYWN0aXZlIHNlc3Npb25zLlwiLz4gOlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICAgICAgPFRhYmxlIHJvd0NvdW50PXtkYXRhLmxlbmd0aH0gY2xhc3NOYW1lPVwidGFibGUtc3RyaXBlZFwiPlxuICAgICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICAgIGNvbHVtbktleT1cInNpZFwiXG4gICAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPiBTZXNzaW9uIElEIDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17PFRleHRDZWxsIGRhdGE9e2RhdGF9Lz4gfVxuICAgICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD4gPC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXtcbiAgICAgICAgICAgICAgICAgICAgPEJ1dHRvbkNlbGwgZGF0YT17ZGF0YX0gLz5cbiAgICAgICAgICAgICAgICAgIH1cbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+IE5vZGUgPC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXs8Tm9kZUNlbGwgZGF0YT17ZGF0YX0gLz4gfVxuICAgICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICAgICAgY29sdW1uS2V5PVwiY3JlYXRlZFwiXG4gICAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPiBDcmVhdGVkIDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17PERhdGVDcmVhdGVkQ2VsbCBkYXRhPXtkYXRhfS8+IH1cbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+IFVzZXJzIDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17PFVzZXJzQ2VsbCBkYXRhPXtkYXRhfSAvPiB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgPC9UYWJsZT5cbiAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIH1cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApXG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IEFjdGl2ZVNlc3Npb25MaXN0O1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvc2Vzc2lvbnMvYWN0aXZlU2Vzc2lvbkxpc3QuanN4XG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHtzZXNzaW9uc1ZpZXd9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvc2Vzc2lvbnMvZ2V0dGVycycpO1xudmFyIHtmaWx0ZXJ9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvc3RvcmVkU2Vzc2lvbnNGaWx0ZXIvZ2V0dGVycycpO1xudmFyIFN0b3JlZFNlc3Npb25MaXN0ID0gcmVxdWlyZSgnLi9zdG9yZWRTZXNzaW9uTGlzdC5qc3gnKTtcbnZhciBBY3RpdmVTZXNzaW9uTGlzdCA9IHJlcXVpcmUoJy4vYWN0aXZlU2Vzc2lvbkxpc3QuanN4Jyk7XG5cbnZhciBTZXNzaW9ucyA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIGRhdGE6IHNlc3Npb25zVmlldyxcbiAgICAgIHN0b3JlZFNlc3Npb25zRmlsdGVyOiBmaWx0ZXJcbiAgICB9XG4gIH0sXG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICBsZXQge2RhdGEsIHN0b3JlZFNlc3Npb25zRmlsdGVyfSA9IHRoaXMuc3RhdGU7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXNlc3Npb25zIGdydi1wYWdlXCI+XG4gICAgICAgIDxBY3RpdmVTZXNzaW9uTGlzdCBkYXRhPXtkYXRhfS8+XG4gICAgICAgIDxociBjbGFzc05hbWU9XCJncnYtZGl2aWRlclwiLz5cbiAgICAgICAgPFN0b3JlZFNlc3Npb25MaXN0IGRhdGE9e2RhdGF9IGZpbHRlcj17c3RvcmVkU2Vzc2lvbnNGaWx0ZXJ9Lz5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IFNlc3Npb25zO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvc2Vzc2lvbnMvbWFpbi5qc3hcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIge2FjdGlvbnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvc3RvcmVkU2Vzc2lvbnNGaWx0ZXInKTtcbnZhciBJbnB1dFNlYXJjaCA9IHJlcXVpcmUoJy4vLi4vaW5wdXRTZWFyY2guanN4Jyk7XG52YXIge1RhYmxlLCBDb2x1bW4sIENlbGwsIFRleHRDZWxsLCBTb3J0SGVhZGVyQ2VsbCwgU29ydFR5cGVzLCBFbXB0eUluZGljYXRvcn0gPSByZXF1aXJlKCdhcHAvY29tcG9uZW50cy90YWJsZS5qc3gnKTtcbnZhciB7QnV0dG9uQ2VsbCwgU2luZ2xlVXNlckNlbGwsIERhdGVDcmVhdGVkQ2VsbH0gPSByZXF1aXJlKCcuL2xpc3RJdGVtcycpO1xudmFyIHtEYXRlUmFuZ2VQaWNrZXJ9ID0gcmVxdWlyZSgnLi8uLi9kYXRlUGlja2VyLmpzeCcpO1xudmFyIG1vbWVudCA9ICByZXF1aXJlKCdtb21lbnQnKTtcbnZhciB7aXNNYXRjaH0gPSByZXF1aXJlKCdhcHAvY29tbW9uL29iamVjdFV0aWxzJyk7XG52YXIgXyA9IHJlcXVpcmUoJ18nKTtcbnZhciB7ZGlzcGxheURhdGVGb3JtYXR9ID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xuXG52YXIgQXJjaGl2ZWRTZXNzaW9ucyA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBnZXRJbml0aWFsU3RhdGUoKXtcbiAgICB0aGlzLnNlYXJjaGFibGVQcm9wcyA9IFsnc2VydmVySXAnLCAnY3JlYXRlZCcsICdzaWQnLCAnbG9naW4nXTtcbiAgICByZXR1cm4geyBmaWx0ZXI6ICcnLCBjb2xTb3J0RGlyczoge2NyZWF0ZWQ6ICdBU0MnfX07XG4gIH0sXG5cbiAgY29tcG9uZW50V2lsbE1vdW50KCl7XG4gICAgc2V0VGltZW91dCgoKT0+YWN0aW9ucy5mZXRjaCgpLCAwKTtcbiAgfSxcblxuICBjb21wb25lbnRXaWxsVW5tb3VudCgpe1xuICAgIGFjdGlvbnMucmVtb3ZlU3RvcmVkU2Vzc2lvbnMoKTtcbiAgfSxcblxuICBvbkZpbHRlckNoYW5nZSh2YWx1ZSl7XG4gICAgdGhpcy5zdGF0ZS5maWx0ZXIgPSB2YWx1ZTtcbiAgICB0aGlzLnNldFN0YXRlKHRoaXMuc3RhdGUpO1xuICB9LFxuXG4gIG9uU29ydENoYW5nZShjb2x1bW5LZXksIHNvcnREaXIpIHtcbiAgICB0aGlzLnN0YXRlLmNvbFNvcnREaXJzID0geyBbY29sdW1uS2V5XTogc29ydERpciB9O1xuICAgIHRoaXMuc2V0U3RhdGUodGhpcy5zdGF0ZSk7XG4gIH0sXG5cbiAgb25SYW5nZVBpY2tlckNoYW5nZSh7c3RhcnREYXRlLCBlbmREYXRlfSl7XG4gICAgYWN0aW9ucy5zZXRUaW1lUmFuZ2Uoc3RhcnREYXRlLCBlbmREYXRlKTtcbiAgfSxcblxuICBzZWFyY2hBbmRGaWx0ZXJDYih0YXJnZXRWYWx1ZSwgc2VhcmNoVmFsdWUsIHByb3BOYW1lKXtcbiAgICBpZihwcm9wTmFtZSA9PT0gJ2NyZWF0ZWQnKXtcbiAgICAgIHZhciBkaXNwbGF5RGF0ZSA9IG1vbWVudCh0YXJnZXRWYWx1ZSkuZm9ybWF0KGRpc3BsYXlEYXRlRm9ybWF0KS50b0xvY2FsZVVwcGVyQ2FzZSgpO1xuICAgICAgcmV0dXJuIGRpc3BsYXlEYXRlLmluZGV4T2Yoc2VhcmNoVmFsdWUpICE9PSAtMTtcbiAgICB9XG4gIH0sXG5cbiAgc29ydEFuZEZpbHRlcihkYXRhKXtcbiAgICB2YXIgZmlsdGVyZWQgPSBkYXRhLmZpbHRlcihvYmo9PlxuICAgICAgaXNNYXRjaChvYmosIHRoaXMuc3RhdGUuZmlsdGVyLCB7XG4gICAgICAgIHNlYXJjaGFibGVQcm9wczogdGhpcy5zZWFyY2hhYmxlUHJvcHMsXG4gICAgICAgIGNiOiB0aGlzLnNlYXJjaEFuZEZpbHRlckNiXG4gICAgICB9KSk7XG5cbiAgICB2YXIgY29sdW1uS2V5ID0gT2JqZWN0LmdldE93blByb3BlcnR5TmFtZXModGhpcy5zdGF0ZS5jb2xTb3J0RGlycylbMF07XG4gICAgdmFyIHNvcnREaXIgPSB0aGlzLnN0YXRlLmNvbFNvcnREaXJzW2NvbHVtbktleV07XG4gICAgdmFyIHNvcnRlZCA9IF8uc29ydEJ5KGZpbHRlcmVkLCBjb2x1bW5LZXkpO1xuICAgIGlmKHNvcnREaXIgPT09IFNvcnRUeXBlcy5BU0Mpe1xuICAgICAgc29ydGVkID0gc29ydGVkLnJldmVyc2UoKTtcbiAgICB9XG5cbiAgICByZXR1cm4gc29ydGVkO1xuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgbGV0IHtzdGFydCwgZW5kLCBzdGF0dXN9ID0gdGhpcy5wcm9wcy5maWx0ZXI7XG4gICAgbGV0IGRhdGEgPSB0aGlzLnByb3BzLmRhdGEuZmlsdGVyKFxuICAgICAgaXRlbSA9PiAhaXRlbS5hY3RpdmUgJiYgbW9tZW50KGl0ZW0uY3JlYXRlZCkuaXNCZXR3ZWVuKHN0YXJ0LCBlbmQpKTtcblxuICAgIGRhdGEgPSB0aGlzLnNvcnRBbmRGaWx0ZXIoZGF0YSk7XG5cbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtc2Vzc2lvbnMtc3RvcmVkXCI+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWhlYWRlclwiPlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWZsZXhcIj5cbiAgICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWZsZXgtY29sdW1uXCI+PC9kaXY+XG4gICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LWNvbHVtblwiPlxuICAgICAgICAgICAgICA8aDE+IEFyY2hpdmVkIFNlc3Npb25zIDwvaDE+XG4gICAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWZsZXgtY29sdW1uXCI+XG4gICAgICAgICAgICAgIDxJbnB1dFNlYXJjaCB2YWx1ZT17dGhpcy5maWx0ZXJ9IG9uQ2hhbmdlPXt0aGlzLm9uRmlsdGVyQ2hhbmdlfS8+XG4gICAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4XCI+XG4gICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LXJvd1wiPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LXJvd1wiPlxuICAgICAgICAgICAgICA8RGF0ZVJhbmdlUGlja2VyIHN0YXJ0RGF0ZT17c3RhcnR9IGVuZERhdGU9e2VuZH0gb25DaGFuZ2U9e3RoaXMub25SYW5nZVBpY2tlckNoYW5nZX0vPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LXJvd1wiPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICA8L2Rpdj5cbiAgICAgICAgPC9kaXY+XG5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtY29udGVudFwiPlxuICAgICAgICAgIHtkYXRhLmxlbmd0aCA9PT0gMCAmJiAhc3RhdHVzLmlzTG9hZGluZyA/IDxFbXB0eUluZGljYXRvciB0ZXh0PVwiTm8gbWF0Y2hpbmcgYXJjaGl2ZWQgc2Vzc2lvbnMgZm91bmQuXCIvPiA6XG4gICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgICAgICA8VGFibGUgcm93Q291bnQ9e2RhdGEubGVuZ3RofSBjbGFzc05hbWU9XCJ0YWJsZS1zdHJpcGVkXCI+XG4gICAgICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICAgICAgY29sdW1uS2V5PVwic2lkXCJcbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+IFNlc3Npb24gSUQgPC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXs8VGV4dENlbGwgZGF0YT17ZGF0YX0vPiB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPiA8L0NlbGw+IH1cbiAgICAgICAgICAgICAgICAgIGNlbGw9e1xuICAgICAgICAgICAgICAgICAgICA8QnV0dG9uQ2VsbCBkYXRhPXtkYXRhfSAvPlxuICAgICAgICAgICAgICAgICAgfVxuICAgICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICAgICAgY29sdW1uS2V5PVwiY3JlYXRlZFwiXG4gICAgICAgICAgICAgICAgICBoZWFkZXI9e1xuICAgICAgICAgICAgICAgICAgICA8U29ydEhlYWRlckNlbGxcbiAgICAgICAgICAgICAgICAgICAgICBzb3J0RGlyPXt0aGlzLnN0YXRlLmNvbFNvcnREaXJzLmNyZWF0ZWR9XG4gICAgICAgICAgICAgICAgICAgICAgb25Tb3J0Q2hhbmdlPXt0aGlzLm9uU29ydENoYW5nZX1cbiAgICAgICAgICAgICAgICAgICAgICB0aXRsZT1cIkNyZWF0ZWRcIlxuICAgICAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICAgICAgfVxuICAgICAgICAgICAgICAgICAgY2VsbD17PERhdGVDcmVhdGVkQ2VsbCBkYXRhPXtkYXRhfS8+IH1cbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+IFVzZXIgPC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXs8U2luZ2xlVXNlckNlbGwgZGF0YT17ZGF0YX0vPiB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgPC9UYWJsZT5cbiAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIH1cbiAgICAgICAgPC9kaXY+XG4gICAgICAgIHtcbiAgICAgICAgICBzdGF0dXMuaGFzTW9yZSA/XG4gICAgICAgICAgICAoPGRpdiBjbGFzc05hbWU9XCJncnYtZm9vdGVyXCI+XG4gICAgICAgICAgICAgIDxidXR0b24gZGlzYWJsZWQ9e3N0YXR1cy5pc0xvYWRpbmd9IGNsYXNzTmFtZT1cImJ0biBidG4tcHJpbWFyeSBidG4tb3V0bGluZVwiIG9uQ2xpY2s9e2FjdGlvbnMuZmV0Y2hNb3JlfT5cbiAgICAgICAgICAgICAgICA8c3Bhbj5Mb2FkIG1vcmUuLi48L3NwYW4+XG4gICAgICAgICAgICAgIDwvYnV0dG9uPlxuICAgICAgICAgICAgPC9kaXY+KSA6IG51bGxcbiAgICAgICAgfVxuICAgICAgPC9kaXY+XG4gICAgKVxuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBBcmNoaXZlZFNlc3Npb25zO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvc2Vzc2lvbnMvc3RvcmVkU2Vzc2lvbkxpc3QuanN4XG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcbnZhciBzZXNzaW9uID0gcmVxdWlyZSgnYXBwL3NlcnZpY2VzL3Nlc3Npb24nKTtcbnZhciBUZXJtaW5hbCA9IHJlcXVpcmUoJ2FwcC9jb21tb24vdGVybWluYWwnKTtcbnZhciB7dXBkYXRlU2Vzc2lvbn0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9zZXNzaW9ucy9hY3Rpb25zJyk7XG5cbnZhciBUdHlUZXJtaW5hbCA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBjb21wb25lbnREaWRNb3VudDogZnVuY3Rpb24oKSB7XG4gICAgbGV0IHtzZXJ2ZXJJZCwgbG9naW4sIHNpZCwgcm93cywgY29sc30gPSB0aGlzLnByb3BzO1xuICAgIGxldCB7dG9rZW59ID0gc2Vzc2lvbi5nZXRVc2VyRGF0YSgpO1xuICAgIGxldCB1cmwgPSBjZmcuYXBpLmdldFR0eVVybCgpO1xuXG4gICAgbGV0IG9wdGlvbnMgPSB7XG4gICAgICB0dHk6IHtcbiAgICAgICAgc2VydmVySWQsIGxvZ2luLCBzaWQsIHRva2VuLCB1cmxcbiAgICAgIH0sXG4gICAgIHJvd3MsXG4gICAgIGNvbHMsXG4gICAgIGVsOiB0aGlzLnJlZnMuY29udGFpbmVyXG4gICAgfVxuXG4gICAgdGhpcy50ZXJtaW5hbCA9IG5ldyBUZXJtaW5hbChvcHRpb25zKTtcbiAgICB0aGlzLnRlcm1pbmFsLnR0eUV2ZW50cy5vbignZGF0YScsIHVwZGF0ZVNlc3Npb24pO1xuICAgIHRoaXMudGVybWluYWwub3BlbigpO1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50OiBmdW5jdGlvbigpIHtcbiAgICB0aGlzLnRlcm1pbmFsLmRlc3Ryb3koKTtcbiAgfSxcblxuICBzaG91bGRDb21wb25lbnRVcGRhdGU6IGZ1bmN0aW9uKCkge1xuICAgIHJldHVybiBmYWxzZTtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuICggPGRpdiByZWY9XCJjb250YWluZXJcIj4gIDwvZGl2PiApO1xuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBUdHlUZXJtaW5hbDtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL3Rlcm1pbmFsLmpzeFxuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciByZW5kZXIgPSByZXF1aXJlKCdyZWFjdC1kb20nKS5yZW5kZXI7XG52YXIgeyBSb3V0ZXIsIFJvdXRlLCBSZWRpcmVjdCB9ID0gcmVxdWlyZSgncmVhY3Qtcm91dGVyJyk7XG52YXIgeyBBcHAsIExvZ2luLCBOb2RlcywgU2Vzc2lvbnMsIE5ld1VzZXIsIEN1cnJlbnRTZXNzaW9uSG9zdCwgTWVzc2FnZVBhZ2UsIE5vdEZvdW5kIH0gPSByZXF1aXJlKCcuL2NvbXBvbmVudHMnKTtcbnZhciB7ZW5zdXJlVXNlcn0gPSByZXF1aXJlKCcuL21vZHVsZXMvdXNlci9hY3Rpb25zJyk7XG52YXIgYXV0aCA9IHJlcXVpcmUoJy4vc2VydmljZXMvYXV0aCcpO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCcuL3NlcnZpY2VzL3Nlc3Npb24nKTtcbnZhciBjZmcgPSByZXF1aXJlKCcuL2NvbmZpZycpO1xuXG5yZXF1aXJlKCcuL21vZHVsZXMnKTtcblxuLy8gaW5pdCBzZXNzaW9uXG5zZXNzaW9uLmluaXQoKTtcblxuY2ZnLmluaXQod2luZG93LkdSVl9DT05GSUcpO1xuXG5yZW5kZXIoKFxuICA8Um91dGVyIGhpc3Rvcnk9e3Nlc3Npb24uZ2V0SGlzdG9yeSgpfT5cbiAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5tc2dzfSBjb21wb25lbnQ9e01lc3NhZ2VQYWdlfS8+XG4gICAgPFJvdXRlIHBhdGg9e2NmZy5yb3V0ZXMubG9naW59IGNvbXBvbmVudD17TG9naW59Lz5cbiAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5sb2dvdXR9IG9uRW50ZXI9e2F1dGgubG9nb3V0fS8+XG4gICAgPFJvdXRlIHBhdGg9e2NmZy5yb3V0ZXMubmV3VXNlcn0gY29tcG9uZW50PXtOZXdVc2VyfS8+XG4gICAgPFJlZGlyZWN0IGZyb209e2NmZy5yb3V0ZXMuYXBwfSB0bz17Y2ZnLnJvdXRlcy5ub2Rlc30vPlxuICAgIDxSb3V0ZSBwYXRoPXtjZmcucm91dGVzLmFwcH0gY29tcG9uZW50PXtBcHB9IG9uRW50ZXI9e2Vuc3VyZVVzZXJ9ID5cbiAgICAgIDxSb3V0ZSBwYXRoPXtjZmcucm91dGVzLm5vZGVzfSBjb21wb25lbnQ9e05vZGVzfS8+XG4gICAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5hY3RpdmVTZXNzaW9ufSBjb21wb25lbnRzPXt7Q3VycmVudFNlc3Npb25Ib3N0OiBDdXJyZW50U2Vzc2lvbkhvc3R9fS8+XG4gICAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5zZXNzaW9uc30gY29tcG9uZW50PXtTZXNzaW9uc30vPlxuICAgIDwvUm91dGU+XG4gICAgPFJvdXRlIHBhdGg9XCIqXCIgY29tcG9uZW50PXtOb3RGb3VuZH0gLz5cbiAgPC9Sb3V0ZXI+XG4pLCBkb2N1bWVudC5nZXRFbGVtZW50QnlJZChcImFwcFwiKSk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvaW5kZXguanN4XG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIge2ZldGNoU2Vzc2lvbnN9ID0gcmVxdWlyZSgnLi8uLi9zZXNzaW9ucy9hY3Rpb25zJyk7XG52YXIge2ZldGNoTm9kZXN9ID0gcmVxdWlyZSgnLi8uLi9ub2Rlcy9hY3Rpb25zJyk7XG52YXIgJCA9IHJlcXVpcmUoJ2pRdWVyeScpO1xuXG5jb25zdCB7IFRMUFRfQVBQX0lOSVQsIFRMUFRfQVBQX0ZBSUxFRCwgVExQVF9BUFBfUkVBRFkgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuY29uc3QgYWN0aW9ucyA9IHtcblxuICBpbml0QXBwKCkge1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9BUFBfSU5JVCk7ICAgIFxuICAgIGFjdGlvbnMuZmV0Y2hOb2Rlc0FuZFNlc3Npb25zKClcbiAgICAgIC5kb25lKCgpPT4gcmVhY3Rvci5kaXNwYXRjaChUTFBUX0FQUF9SRUFEWSkgKVxuICAgICAgLmZhaWwoKCk9PiByZWFjdG9yLmRpc3BhdGNoKFRMUFRfQVBQX0ZBSUxFRCkgKTtcbiAgfSxcblxuICBmZXRjaE5vZGVzQW5kU2Vzc2lvbnMoKSB7XG4gICAgcmV0dXJuICQud2hlbihmZXRjaE5vZGVzKCksIGZldGNoU2Vzc2lvbnMoKSk7XG4gIH1cbn1cblxuZXhwb3J0IGRlZmF1bHQgYWN0aW9ucztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2FwcC9hY3Rpb25zLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG5jb25zdCBhcHBTdGF0ZSA9IFtbJ3RscHQnXSwgYXBwPT4gYXBwLnRvSlMoKV07XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgYXBwU3RhdGVcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2FwcC9nZXR0ZXJzLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xubW9kdWxlLmV4cG9ydHMuZ2V0dGVycyA9IHJlcXVpcmUoJy4vZ2V0dGVycycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aW9ucyA9IHJlcXVpcmUoJy4vYWN0aW9ucycpO1xubW9kdWxlLmV4cG9ydHMuYXBwU3RvcmUgPSByZXF1aXJlKCcuL2FwcFN0b3JlJyk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hcHAvaW5kZXguanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbmNvbnN0IGRpYWxvZ3MgPSBbWyd0bHB0X2RpYWxvZ3MnXSwgc3RhdGU9PiBzdGF0ZS50b0pTKCldO1xuXG5leHBvcnQgZGVmYXVsdCB7XG4gIGRpYWxvZ3Ncbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvZ2V0dGVycy5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxubW9kdWxlLmV4cG9ydHMuZ2V0dGVycyA9IHJlcXVpcmUoJy4vZ2V0dGVycycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aW9ucyA9IHJlcXVpcmUoJy4vYWN0aW9ucycpO1xubW9kdWxlLmV4cG9ydHMuZGlhbG9nU3RvcmUgPSByZXF1aXJlKCcuL2RpYWxvZ1N0b3JlJyk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9kaWFsb2dzL2luZGV4LmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG5yZWFjdG9yLnJlZ2lzdGVyU3RvcmVzKHtcbiAgJ3RscHQnOiByZXF1aXJlKCcuL2FwcC9hcHBTdG9yZScpLFxuICAndGxwdF9kaWFsb2dzJzogcmVxdWlyZSgnLi9kaWFsb2dzL2RpYWxvZ1N0b3JlJyksXG4gICd0bHB0X2N1cnJlbnRfc2Vzc2lvbic6IHJlcXVpcmUoJy4vY3VycmVudFNlc3Npb24vY3VycmVudFNlc3Npb25TdG9yZScpLFxuICAndGxwdF91c2VyJzogcmVxdWlyZSgnLi91c2VyL3VzZXJTdG9yZScpLFxuICAndGxwdF91c2VyX2ludml0ZSc6IHJlcXVpcmUoJy4vdXNlci91c2VySW52aXRlU3RvcmUnKSxcbiAgJ3RscHRfbm9kZXMnOiByZXF1aXJlKCcuL25vZGVzL25vZGVTdG9yZScpLFxuICAndGxwdF9yZXN0X2FwaSc6IHJlcXVpcmUoJy4vcmVzdEFwaS9yZXN0QXBpU3RvcmUnKSxcbiAgJ3RscHRfc2Vzc2lvbnMnOiByZXF1aXJlKCcuL3Nlc3Npb25zL3Nlc3Npb25TdG9yZScpLFxuICAndGxwdF9zdG9yZWRfc2Vzc2lvbnNfZmlsdGVyJzogcmVxdWlyZSgnLi9zdG9yZWRTZXNzaW9uc0ZpbHRlci9zdG9yZWRTZXNzaW9uRmlsdGVyU3RvcmUnKSxcbiAgJ3RscHRfbm90aWZpY2F0aW9ucyc6IHJlcXVpcmUoJy4vbm90aWZpY2F0aW9ucy9ub3RpZmljYXRpb25TdG9yZScpXG59KTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2luZGV4LmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgeyBUTFBUX05PREVTX1JFQ0VJVkUgfSAgPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG52YXIgYXBpID0gcmVxdWlyZSgnYXBwL3NlcnZpY2VzL2FwaScpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcbnZhciB7c2hvd0Vycm9yfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL25vdGlmaWNhdGlvbnMvYWN0aW9ucycpO1xuXG5jb25zdCBsb2dnZXIgPSByZXF1aXJlKCdhcHAvY29tbW9uL2xvZ2dlcicpLmNyZWF0ZSgnTW9kdWxlcy9Ob2RlcycpO1xuXG5leHBvcnQgZGVmYXVsdCB7XG4gIGZldGNoTm9kZXMoKXtcbiAgICBhcGkuZ2V0KGNmZy5hcGkubm9kZXNQYXRoKS5kb25lKChkYXRhPVtdKT0+e1xuICAgICAgdmFyIG5vZGVBcnJheSA9IGRhdGEubm9kZXMubWFwKGl0ZW09Pml0ZW0ubm9kZSk7XG4gICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfTk9ERVNfUkVDRUlWRSwgbm9kZUFycmF5KTtcbiAgICB9KS5mYWlsKChlcnIpPT57XG4gICAgICBzaG93RXJyb3IoJ1VuYWJsZSB0byByZXRyaWV2ZSBsaXN0IG9mIG5vZGVzJyk7XG4gICAgICBsb2dnZXIuZXJyb3IoJ2ZldGNoTm9kZXMnLCBlcnIpO1xuICAgIH0pXG4gIH1cbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2FjdGlvbnMuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyICB7IFRMUFRfTk9ERVNfUkVDRUlWRSB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gdG9JbW11dGFibGUoW10pO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX05PREVTX1JFQ0VJVkUsIHJlY2VpdmVOb2RlcylcbiAgfVxufSlcblxuZnVuY3Rpb24gcmVjZWl2ZU5vZGVzKHN0YXRlLCBub2RlQXJyYXkpe1xuICByZXR1cm4gdG9JbW11dGFibGUobm9kZUFycmF5KTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL25vZGVTdG9yZS5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxuZXhwb3J0IGNvbnN0IGxhc3RNZXNzYWdlID1cbiAgWyBbJ3RscHRfbm90aWZpY2F0aW9ucyddLCBub3RpZmljYXRpb25zID0+IG5vdGlmaWNhdGlvbnMubGFzdCgpIF07XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9ub3RpZmljYXRpb25zL2dldHRlcnMuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbmltcG9ydCB7IFN0b3JlLCBJbW11dGFibGUgfSBmcm9tICdudWNsZWFyLWpzJztcbmltcG9ydCB7VExQVF9OT1RJRklDQVRJT05TX0FERH0gZnJvbSAnLi9hY3Rpb25UeXBlcyc7XG5cbmV4cG9ydCBkZWZhdWx0IFN0b3JlKHtcbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiBuZXcgSW1tdXRhYmxlLk9yZGVyZWRNYXAoKTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9OT1RJRklDQVRJT05TX0FERCwgYWRkTm90aWZpY2F0aW9uKTtcbiAgfVxufSk7XG5cbmZ1bmN0aW9uIGFkZE5vdGlmaWNhdGlvbihzdGF0ZSwgbWVzc2FnZSkge1xuICByZXR1cm4gc3RhdGUuc2V0KHN0YXRlLnNpemUsIG1lc3NhZ2UpO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvbm90aWZpY2F0aW9ucy9ub3RpZmljYXRpb25TdG9yZS5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xuXG52YXIge1xuICBUTFBUX1JFU1RfQVBJX1NUQVJULFxuICBUTFBUX1JFU1RfQVBJX1NVQ0NFU1MsXG4gIFRMUFRfUkVTVF9BUElfRkFJTCB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCB7XG5cbiAgc3RhcnQocmVxVHlwZSl7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFU1RfQVBJX1NUQVJULCB7dHlwZTogcmVxVHlwZX0pO1xuICB9LFxuXG4gIGZhaWwocmVxVHlwZSwgbWVzc2FnZSl7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFU1RfQVBJX0ZBSUwsICB7dHlwZTogcmVxVHlwZSwgbWVzc2FnZX0pO1xuICB9LFxuXG4gIHN1Y2Nlc3MocmVxVHlwZSl7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFU1RfQVBJX1NVQ0NFU1MsIHt0eXBlOiByZXFUeXBlfSk7XG4gIH1cblxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9hY3Rpb25zLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgZGVmYXVsdE9iaiA9IHtcbiAgaXNQcm9jZXNzaW5nOiBmYWxzZSxcbiAgaXNFcnJvcjogZmFsc2UsXG4gIGlzU3VjY2VzczogZmFsc2UsXG4gIG1lc3NhZ2U6ICcnXG59XG5cbmNvbnN0IHJlcXVlc3RTdGF0dXMgPSAocmVxVHlwZSkgPT4gIFsgWyd0bHB0X3Jlc3RfYXBpJywgcmVxVHlwZV0sIChhdHRlbXApID0+IHtcbiAgcmV0dXJuIGF0dGVtcCA/IGF0dGVtcC50b0pTKCkgOiBkZWZhdWx0T2JqO1xuIH1cbl07XG5cbmV4cG9ydCBkZWZhdWx0IHsgIHJlcXVlc3RTdGF0dXMgIH07XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2dldHRlcnMuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyIHtcbiAgVExQVF9SRVNUX0FQSV9TVEFSVCxcbiAgVExQVF9SRVNUX0FQSV9TVUNDRVNTLFxuICBUTFBUX1JFU1RfQVBJX0ZBSUwgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKHt9KTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9SRVNUX0FQSV9TVEFSVCwgc3RhcnQpO1xuICAgIHRoaXMub24oVExQVF9SRVNUX0FQSV9GQUlMLCBmYWlsKTtcbiAgICB0aGlzLm9uKFRMUFRfUkVTVF9BUElfU1VDQ0VTUywgc3VjY2Vzcyk7XG4gIH1cbn0pXG5cbmZ1bmN0aW9uIHN0YXJ0KHN0YXRlLCByZXF1ZXN0KXtcbiAgcmV0dXJuIHN0YXRlLnNldChyZXF1ZXN0LnR5cGUsIHRvSW1tdXRhYmxlKHtpc1Byb2Nlc3Npbmc6IHRydWV9KSk7XG59XG5cbmZ1bmN0aW9uIGZhaWwoc3RhdGUsIHJlcXVlc3Qpe1xuICByZXR1cm4gc3RhdGUuc2V0KHJlcXVlc3QudHlwZSwgdG9JbW11dGFibGUoe2lzRmFpbGVkOiB0cnVlLCBtZXNzYWdlOiByZXF1ZXN0Lm1lc3NhZ2V9KSk7XG59XG5cbmZ1bmN0aW9uIHN1Y2Nlc3Moc3RhdGUsIHJlcXVlc3Qpe1xuICByZXR1cm4gc3RhdGUuc2V0KHJlcXVlc3QudHlwZSwgdG9JbW11dGFibGUoe2lzU3VjY2VzczogdHJ1ZX0pKTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvcmVzdEFwaVN0b3JlLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG5tb2R1bGUuZXhwb3J0cy5nZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3Rpb25zID0gcmVxdWlyZSgnLi9hY3Rpb25zJyk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9zZXNzaW9ucy9pbmRleC5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG52YXIgeyBUTFBUX1NFU1NJTlNfUkVDRUlWRSwgVExQVF9TRVNTSU5TX1VQREFURSwgVExQVF9TRVNTSU5TX1JFTU9WRV9TVE9SRUQgfSAgPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IFN0b3JlKHtcbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB0b0ltbXV0YWJsZSh7fSk7XG4gIH0sXG5cbiAgaW5pdGlhbGl6ZSgpIHtcbiAgICB0aGlzLm9uKFRMUFRfU0VTU0lOU19SRUNFSVZFLCByZWNlaXZlU2Vzc2lvbnMpO1xuICAgIHRoaXMub24oVExQVF9TRVNTSU5TX1VQREFURSwgdXBkYXRlU2Vzc2lvbik7XG4gICAgdGhpcy5vbihUTFBUX1NFU1NJTlNfUkVNT1ZFX1NUT1JFRCwgcmVtb3ZlU3RvcmVkU2Vzc2lvbnMpO1xuICB9XG59KVxuXG5mdW5jdGlvbiByZW1vdmVTdG9yZWRTZXNzaW9ucyhzdGF0ZSl7XG4gIHJldHVybiBzdGF0ZS53aXRoTXV0YXRpb25zKHN0YXRlID0+IHtcbiAgICBzdGF0ZS52YWx1ZVNlcSgpLmZvckVhY2goaXRlbT0+IHtcbiAgICAgIGlmKGl0ZW0uZ2V0KCdhY3RpdmUnKSAhPT0gdHJ1ZSl7XG4gICAgICAgIHN0YXRlLnJlbW92ZShpdGVtLmdldCgnaWQnKSk7XG4gICAgICB9XG4gICAgfSk7XG4gIH0pO1xufVxuXG5mdW5jdGlvbiB1cGRhdGVTZXNzaW9uKHN0YXRlLCBqc29uKXtcbiAgcmV0dXJuIHN0YXRlLnNldChqc29uLmlkLCB0b0ltbXV0YWJsZShqc29uKSk7XG59XG5cbmZ1bmN0aW9uIHJlY2VpdmVTZXNzaW9ucyhzdGF0ZSwganNvbkFycmF5KXtcbiAganNvbkFycmF5ID0ganNvbkFycmF5IHx8IFtdO1xuXG4gIHJldHVybiBzdGF0ZS53aXRoTXV0YXRpb25zKHN0YXRlID0+IHtcbiAgICBqc29uQXJyYXkuZm9yRWFjaCgoaXRlbSkgPT4ge1xuICAgICAgaXRlbS5jcmVhdGVkID0gbmV3IERhdGUoaXRlbS5jcmVhdGVkKTtcbiAgICAgIGl0ZW0ubGFzdF9hY3RpdmUgPSBuZXcgRGF0ZShpdGVtLmxhc3RfYWN0aXZlKTtcbiAgICAgIHN0YXRlLnNldChpdGVtLmlkLCB0b0ltbXV0YWJsZShpdGVtKSlcbiAgICB9KVxuICB9KTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL3Nlc3Npb25TdG9yZS5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHtmaWx0ZXJ9ID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG52YXIge21heFNlc3Npb25Mb2FkU2l6ZX0gPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG52YXIgbW9tZW50ID0gcmVxdWlyZSgnbW9tZW50Jyk7XG52YXIgYXBpVXRpbHMgPSByZXF1aXJlKCdhcHAvc2VydmljZXMvYXBpVXRpbHMnKTtcblxudmFyIHtzaG93RXJyb3J9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvbm90aWZpY2F0aW9ucy9hY3Rpb25zJyk7XG5cbmNvbnN0IGxvZ2dlciA9IHJlcXVpcmUoJ2FwcC9jb21tb24vbG9nZ2VyJykuY3JlYXRlKCdNb2R1bGVzL1Nlc3Npb25zJyk7XG5cbmNvbnN0IHtcbiAgVExQVF9TVE9SRURfU0VTU0lOU19GSUxURVJfU0VUX1JBTkdFLFxuICBUTFBUX1NUT1JFRF9TRVNTSU5TX0ZJTFRFUl9TRVRfU1RBVFVTIH0gID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5jb25zdCB7VExQVF9TRVNTSU5TX1JFQ0VJVkUsIFRMUFRfU0VTU0lOU19SRU1PVkVfU1RPUkVEIH0gID0gcmVxdWlyZSgnLi8uLi9zZXNzaW9ucy9hY3Rpb25UeXBlcycpO1xuXG4vKipcbiogRHVlIHRvIGN1cnJlbnQgbGltaXRhdGlvbnMgb2YgdGhlIGJhY2tlbmQgQVBJLCB0aGUgZmlsdGVyaW5nIGxvZ2ljIGZvciB0aGUgQXJjaGl2ZWQgbGlzdCBvZiBTZXNzaW9uXG4qIHdvcmtzIGFzIGZvbGxvd3M6XG4qICAxKSBlYWNoIHRpbWUgYSBuZXcgZGF0ZSByYW5nZSBpcyBzZXQsIGFsbCBwcmV2aW91c2x5IHJldHJpZXZlZCBpbmFjdGl2ZSBzZXNzaW9ucyBnZXQgZGVsZXRlZC5cbiogIDIpIGhhc01vcmUgZmxhZyB3aWxsIGJlIGRldGVybWluZSBhZnRlciBhIGNvbnNlcXVlbnQgZmV0Y2ggcmVxdWVzdCB3aXRoIG5ldyBkYXRlIHJhbmdlIHZhbHVlcy5cbiovXG5cbmNvbnN0IGFjdGlvbnMgPSB7XG5cbiAgZmV0Y2goKXtcbiAgICBsZXQgeyBlbmQgfSA9IHJlYWN0b3IuZXZhbHVhdGUoZmlsdGVyKTtcbiAgICBfZmV0Y2goZW5kKTtcbiAgfSxcblxuICBmZXRjaE1vcmUoKXtcbiAgICBsZXQge3N0YXR1cywgZW5kIH0gPSByZWFjdG9yLmV2YWx1YXRlKGZpbHRlcik7XG4gICAgaWYoc3RhdHVzLmhhc01vcmUgPT09IHRydWUgJiYgIXN0YXR1cy5pc0xvYWRpbmcpe1xuICAgICAgX2ZldGNoKGVuZCwgc3RhdHVzLnNpZCk7XG4gICAgfVxuICB9LFxuXG4gIHJlbW92ZVN0b3JlZFNlc3Npb25zKCl7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1NFU1NJTlNfUkVNT1ZFX1NUT1JFRCk7XG4gIH0sXG5cbiAgc2V0VGltZVJhbmdlKHN0YXJ0LCBlbmQpe1xuICAgIHJlYWN0b3IuYmF0Y2goKCk9PntcbiAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9TVE9SRURfU0VTU0lOU19GSUxURVJfU0VUX1JBTkdFLCB7c3RhcnQsIGVuZCwgaGFzTW9yZTogZmFsc2V9KTtcbiAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9TRVNTSU5TX1JFTU9WRV9TVE9SRUQpO1xuICAgICAgX2ZldGNoKGVuZCk7XG4gICAgfSk7XG4gIH1cbn1cblxuZnVuY3Rpb24gX2ZldGNoKGVuZCwgc2lkKXtcbiAgbGV0IHN0YXR1cyA9IHtcbiAgICBoYXNNb3JlOiBmYWxzZSxcbiAgICBpc0xvYWRpbmc6IHRydWVcbiAgfVxuXG4gIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9TVE9SRURfU0VTU0lOU19GSUxURVJfU0VUX1NUQVRVUywgc3RhdHVzKTtcblxuICBsZXQgc3RhcnQgPSBlbmQgfHwgbmV3IERhdGUoKTtcbiAgbGV0IHBhcmFtcyA9IHtcbiAgICBvcmRlcjogLTEsXG4gICAgbGltaXQ6IG1heFNlc3Npb25Mb2FkU2l6ZSxcbiAgICBzdGFydCxcbiAgICBzaWRcbiAgfTtcblxuICByZXR1cm4gYXBpVXRpbHMuZmlsdGVyU2Vzc2lvbnMocGFyYW1zKS5kb25lKChqc29uKSA9PiB7XG4gICAgbGV0IHtzZXNzaW9uc30gPSBqc29uO1xuICAgIGxldCB7c3RhcnR9ID0gcmVhY3Rvci5ldmFsdWF0ZShmaWx0ZXIpO1xuXG4gICAgc3RhdHVzLmhhc01vcmUgPSBmYWxzZTtcbiAgICBzdGF0dXMuaXNMb2FkaW5nID0gZmFsc2U7XG5cbiAgICBpZiAoc2Vzc2lvbnMubGVuZ3RoID09PSBtYXhTZXNzaW9uTG9hZFNpemUpIHtcbiAgICAgIGxldCB7aWQsIGNyZWF0ZWR9ID0gc2Vzc2lvbnNbc2Vzc2lvbnMubGVuZ3RoLTFdO1xuICAgICAgc3RhdHVzLnNpZCA9IGlkO1xuICAgICAgc3RhdHVzLmhhc01vcmUgPSBtb21lbnQoc3RhcnQpLmlzQmVmb3JlKGNyZWF0ZWQpO1xuXG4gICAgICAvKipcbiAgICAgICogcmVtb3ZlIGF0IGxlYXN0IDEgaXRlbSBiZWZvcmUgc3RvcmluZyB0aGUgc2Vzc2lvbnMsIHRoaXMgd2F5IHdlIGVuc3VyZSB0aGF0XG4gICAgICAqIHRoZXJlIHdpbGwgYmUgYWx3YXlzIGF0IGxlYXN0IG9uZSBpdGVtIG9uIHRoZSBuZXh0ICdmZXRjaE1vcmUnIHJlcXVlc3QuXG4gICAgICAqL1xuICAgICAgc2Vzc2lvbnMgPSBzZXNzaW9ucy5zbGljZSgwLCBtYXhTZXNzaW9uTG9hZFNpemUtMSk7XG4gICAgfVxuXG4gICAgcmVhY3Rvci5iYXRjaCgoKT0+e1xuICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1NFU1NJTlNfUkVDRUlWRSwgc2Vzc2lvbnMpO1xuICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1NUT1JFRF9TRVNTSU5TX0ZJTFRFUl9TRVRfU1RBVFVTLCBzdGF0dXMpO1xuICAgIH0pO1xuXG4gIH0pXG4gIC5mYWlsKChlcnIpPT57XG4gICAgc2hvd0Vycm9yKCdVbmFibGUgdG8gcmV0cmlldmUgbGlzdCBvZiBzZXNzaW9ucycpO1xuICAgIGxvZ2dlci5lcnJvcignZmV0Y2hpbmcgZmlsdGVyZWQgc2V0IG9mIHNlc3Npb25zJywgZXJyKTtcbiAgfSk7XG5cbn1cblxuZXhwb3J0IGRlZmF1bHQgYWN0aW9ucztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3N0b3JlZFNlc3Npb25zRmlsdGVyL2FjdGlvbnMuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5tb2R1bGUuZXhwb3J0cy5nZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3Rpb25zID0gcmVxdWlyZSgnLi9hY3Rpb25zJyk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9zdG9yZWRTZXNzaW9uc0ZpbHRlci9pbmRleC5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG52YXIgbW9tZW50ID0gcmVxdWlyZSgnbW9tZW50Jyk7XG5cbnZhciB7XG4gIFRMUFRfU1RPUkVEX1NFU1NJTlNfRklMVEVSX1NFVF9SQU5HRSxcbiAgVExQVF9TVE9SRURfU0VTU0lOU19GSUxURVJfU0VUX1NUQVRVUyB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcblxuICAgIGxldCBlbmQgPSBtb21lbnQobmV3IERhdGUoKSkuZW5kT2YoJ2RheScpLnRvRGF0ZSgpO1xuICAgIGxldCBzdGFydCA9IG1vbWVudChlbmQpLnN1YnRyYWN0KDMsICdkYXknKS5zdGFydE9mKCdkYXknKS50b0RhdGUoKTtcbiAgICBsZXQgc3RhdGUgPSB7XG4gICAgICBzdGFydCxcbiAgICAgIGVuZCxcbiAgICAgIHN0YXR1czoge1xuICAgICAgICBpc0xvYWRpbmc6IGZhbHNlLFxuICAgICAgICBoYXNNb3JlOiBmYWxzZVxuICAgICAgfVxuICAgIH1cblxuICAgIHJldHVybiB0b0ltbXV0YWJsZShzdGF0ZSk7XG4gIH0sXG5cbiAgaW5pdGlhbGl6ZSgpIHtcbiAgICB0aGlzLm9uKFRMUFRfU1RPUkVEX1NFU1NJTlNfRklMVEVSX1NFVF9SQU5HRSwgc2V0UmFuZ2UpO1xuICAgIHRoaXMub24oVExQVF9TVE9SRURfU0VTU0lOU19GSUxURVJfU0VUX1NUQVRVUywgc2V0U3RhdHVzKTtcbiAgfVxufSlcblxuZnVuY3Rpb24gc2V0U3RhdHVzKHN0YXRlLCBzdGF0dXMpe1xuICByZXR1cm4gc3RhdGUubWVyZ2VJbihbJ3N0YXR1cyddLCBzdGF0dXMpO1xufVxuXG5mdW5jdGlvbiBzZXRSYW5nZShzdGF0ZSwgbmV3U3RhdGUpe1xuICByZXR1cm4gc3RhdGUubWVyZ2UobmV3U3RhdGUpO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvc3RvcmVkU2Vzc2lvbnNGaWx0ZXIvc3RvcmVkU2Vzc2lvbkZpbHRlclN0b3JlLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgeyBTdG9yZSwgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcbnZhciAgeyBUTFBUX1JFQ0VJVkVfVVNFUl9JTlZJVEUgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKG51bGwpO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX1JFQ0VJVkVfVVNFUl9JTlZJVEUsIHJlY2VpdmVJbnZpdGUpXG4gIH1cbn0pXG5cbmZ1bmN0aW9uIHJlY2VpdmVJbnZpdGUoc3RhdGUsIGludml0ZSl7XG4gIHJldHVybiB0b0ltbXV0YWJsZShpbnZpdGUpO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvdXNlci91c2VySW52aXRlU3RvcmUuanNcbiAqKi8iLCIvKipcbiAqIENvcHlyaWdodCAyMDEzLTIwMTUsIEZhY2Vib29rLCBJbmMuXG4gKiBBbGwgcmlnaHRzIHJlc2VydmVkLlxuICpcbiAqIFRoaXMgc291cmNlIGNvZGUgaXMgbGljZW5zZWQgdW5kZXIgdGhlIEJTRC1zdHlsZSBsaWNlbnNlIGZvdW5kIGluIHRoZVxuICogTElDRU5TRSBmaWxlIGluIHRoZSByb290IGRpcmVjdG9yeSBvZiB0aGlzIHNvdXJjZSB0cmVlLiBBbiBhZGRpdGlvbmFsIGdyYW50XG4gKiBvZiBwYXRlbnQgcmlnaHRzIGNhbiBiZSBmb3VuZCBpbiB0aGUgUEFURU5UUyBmaWxlIGluIHRoZSBzYW1lIGRpcmVjdG9yeS5cbiAqXG4gKiBAdHlwZWNoZWNrc1xuICogQHByb3ZpZGVzTW9kdWxlIFJlYWN0Q1NTVHJhbnNpdGlvbkdyb3VwXG4gKi9cblxuJ3VzZSBzdHJpY3QnO1xuXG52YXIgUmVhY3QgPSByZXF1aXJlKCcuL1JlYWN0Jyk7XG5cbnZhciBhc3NpZ24gPSByZXF1aXJlKCcuL09iamVjdC5hc3NpZ24nKTtcblxudmFyIFJlYWN0VHJhbnNpdGlvbkdyb3VwID0gcmVxdWlyZSgnLi9SZWFjdFRyYW5zaXRpb25Hcm91cCcpO1xudmFyIFJlYWN0Q1NTVHJhbnNpdGlvbkdyb3VwQ2hpbGQgPSByZXF1aXJlKCcuL1JlYWN0Q1NTVHJhbnNpdGlvbkdyb3VwQ2hpbGQnKTtcblxuZnVuY3Rpb24gY3JlYXRlVHJhbnNpdGlvblRpbWVvdXRQcm9wVmFsaWRhdG9yKHRyYW5zaXRpb25UeXBlKSB7XG4gIHZhciB0aW1lb3V0UHJvcE5hbWUgPSAndHJhbnNpdGlvbicgKyB0cmFuc2l0aW9uVHlwZSArICdUaW1lb3V0JztcbiAgdmFyIGVuYWJsZWRQcm9wTmFtZSA9ICd0cmFuc2l0aW9uJyArIHRyYW5zaXRpb25UeXBlO1xuXG4gIHJldHVybiBmdW5jdGlvbiAocHJvcHMpIHtcbiAgICAvLyBJZiB0aGUgdHJhbnNpdGlvbiBpcyBlbmFibGVkXG4gICAgaWYgKHByb3BzW2VuYWJsZWRQcm9wTmFtZV0pIHtcbiAgICAgIC8vIElmIG5vIHRpbWVvdXQgZHVyYXRpb24gaXMgcHJvdmlkZWRcbiAgICAgIGlmIChwcm9wc1t0aW1lb3V0UHJvcE5hbWVdID09IG51bGwpIHtcbiAgICAgICAgcmV0dXJuIG5ldyBFcnJvcih0aW1lb3V0UHJvcE5hbWUgKyAnIHdhc25cXCd0IHN1cHBsaWVkIHRvIFJlYWN0Q1NTVHJhbnNpdGlvbkdyb3VwOiAnICsgJ3RoaXMgY2FuIGNhdXNlIHVucmVsaWFibGUgYW5pbWF0aW9ucyBhbmQgd29uXFwndCBiZSBzdXBwb3J0ZWQgaW4gJyArICdhIGZ1dHVyZSB2ZXJzaW9uIG9mIFJlYWN0LiBTZWUgJyArICdodHRwczovL2ZiLm1lL3JlYWN0LWFuaW1hdGlvbi10cmFuc2l0aW9uLWdyb3VwLXRpbWVvdXQgZm9yIG1vcmUgJyArICdpbmZvcm1hdGlvbi4nKTtcblxuICAgICAgICAvLyBJZiB0aGUgZHVyYXRpb24gaXNuJ3QgYSBudW1iZXJcbiAgICAgIH0gZWxzZSBpZiAodHlwZW9mIHByb3BzW3RpbWVvdXRQcm9wTmFtZV0gIT09ICdudW1iZXInKSB7XG4gICAgICAgICAgcmV0dXJuIG5ldyBFcnJvcih0aW1lb3V0UHJvcE5hbWUgKyAnIG11c3QgYmUgYSBudW1iZXIgKGluIG1pbGxpc2Vjb25kcyknKTtcbiAgICAgICAgfVxuICAgIH1cbiAgfTtcbn1cblxudmFyIFJlYWN0Q1NTVHJhbnNpdGlvbkdyb3VwID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICBkaXNwbGF5TmFtZTogJ1JlYWN0Q1NTVHJhbnNpdGlvbkdyb3VwJyxcblxuICBwcm9wVHlwZXM6IHtcbiAgICB0cmFuc2l0aW9uTmFtZTogUmVhY3RDU1NUcmFuc2l0aW9uR3JvdXBDaGlsZC5wcm9wVHlwZXMubmFtZSxcblxuICAgIHRyYW5zaXRpb25BcHBlYXI6IFJlYWN0LlByb3BUeXBlcy5ib29sLFxuICAgIHRyYW5zaXRpb25FbnRlcjogUmVhY3QuUHJvcFR5cGVzLmJvb2wsXG4gICAgdHJhbnNpdGlvbkxlYXZlOiBSZWFjdC5Qcm9wVHlwZXMuYm9vbCxcbiAgICB0cmFuc2l0aW9uQXBwZWFyVGltZW91dDogY3JlYXRlVHJhbnNpdGlvblRpbWVvdXRQcm9wVmFsaWRhdG9yKCdBcHBlYXInKSxcbiAgICB0cmFuc2l0aW9uRW50ZXJUaW1lb3V0OiBjcmVhdGVUcmFuc2l0aW9uVGltZW91dFByb3BWYWxpZGF0b3IoJ0VudGVyJyksXG4gICAgdHJhbnNpdGlvbkxlYXZlVGltZW91dDogY3JlYXRlVHJhbnNpdGlvblRpbWVvdXRQcm9wVmFsaWRhdG9yKCdMZWF2ZScpXG4gIH0sXG5cbiAgZ2V0RGVmYXVsdFByb3BzOiBmdW5jdGlvbiAoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIHRyYW5zaXRpb25BcHBlYXI6IGZhbHNlLFxuICAgICAgdHJhbnNpdGlvbkVudGVyOiB0cnVlLFxuICAgICAgdHJhbnNpdGlvbkxlYXZlOiB0cnVlXG4gICAgfTtcbiAgfSxcblxuICBfd3JhcENoaWxkOiBmdW5jdGlvbiAoY2hpbGQpIHtcbiAgICAvLyBXZSBuZWVkIHRvIHByb3ZpZGUgdGhpcyBjaGlsZEZhY3Rvcnkgc28gdGhhdFxuICAgIC8vIFJlYWN0Q1NTVHJhbnNpdGlvbkdyb3VwQ2hpbGQgY2FuIHJlY2VpdmUgdXBkYXRlcyB0byBuYW1lLCBlbnRlciwgYW5kXG4gICAgLy8gbGVhdmUgd2hpbGUgaXQgaXMgbGVhdmluZy5cbiAgICByZXR1cm4gUmVhY3QuY3JlYXRlRWxlbWVudChSZWFjdENTU1RyYW5zaXRpb25Hcm91cENoaWxkLCB7XG4gICAgICBuYW1lOiB0aGlzLnByb3BzLnRyYW5zaXRpb25OYW1lLFxuICAgICAgYXBwZWFyOiB0aGlzLnByb3BzLnRyYW5zaXRpb25BcHBlYXIsXG4gICAgICBlbnRlcjogdGhpcy5wcm9wcy50cmFuc2l0aW9uRW50ZXIsXG4gICAgICBsZWF2ZTogdGhpcy5wcm9wcy50cmFuc2l0aW9uTGVhdmUsXG4gICAgICBhcHBlYXJUaW1lb3V0OiB0aGlzLnByb3BzLnRyYW5zaXRpb25BcHBlYXJUaW1lb3V0LFxuICAgICAgZW50ZXJUaW1lb3V0OiB0aGlzLnByb3BzLnRyYW5zaXRpb25FbnRlclRpbWVvdXQsXG4gICAgICBsZWF2ZVRpbWVvdXQ6IHRoaXMucHJvcHMudHJhbnNpdGlvbkxlYXZlVGltZW91dFxuICAgIH0sIGNoaWxkKTtcbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uICgpIHtcbiAgICByZXR1cm4gUmVhY3QuY3JlYXRlRWxlbWVudChSZWFjdFRyYW5zaXRpb25Hcm91cCwgYXNzaWduKHt9LCB0aGlzLnByb3BzLCB7IGNoaWxkRmFjdG9yeTogdGhpcy5fd3JhcENoaWxkIH0pKTtcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gUmVhY3RDU1NUcmFuc2l0aW9uR3JvdXA7XG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiAuL34vcmVhY3QvbGliL1JlYWN0Q1NTVHJhbnNpdGlvbkdyb3VwLmpzXG4gKiogbW9kdWxlIGlkID0gMzg2XG4gKiogbW9kdWxlIGNodW5rcyA9IDFcbiAqKi8iLCIvKipcbiAqIENvcHlyaWdodCAyMDEzLTIwMTUsIEZhY2Vib29rLCBJbmMuXG4gKiBBbGwgcmlnaHRzIHJlc2VydmVkLlxuICpcbiAqIFRoaXMgc291cmNlIGNvZGUgaXMgbGljZW5zZWQgdW5kZXIgdGhlIEJTRC1zdHlsZSBsaWNlbnNlIGZvdW5kIGluIHRoZVxuICogTElDRU5TRSBmaWxlIGluIHRoZSByb290IGRpcmVjdG9yeSBvZiB0aGlzIHNvdXJjZSB0cmVlLiBBbiBhZGRpdGlvbmFsIGdyYW50XG4gKiBvZiBwYXRlbnQgcmlnaHRzIGNhbiBiZSBmb3VuZCBpbiB0aGUgUEFURU5UUyBmaWxlIGluIHRoZSBzYW1lIGRpcmVjdG9yeS5cbiAqXG4gKiBAdHlwZWNoZWNrc1xuICogQHByb3ZpZGVzTW9kdWxlIFJlYWN0Q1NTVHJhbnNpdGlvbkdyb3VwQ2hpbGRcbiAqL1xuXG4ndXNlIHN0cmljdCc7XG5cbnZhciBSZWFjdCA9IHJlcXVpcmUoJy4vUmVhY3QnKTtcbnZhciBSZWFjdERPTSA9IHJlcXVpcmUoJy4vUmVhY3RET00nKTtcblxudmFyIENTU0NvcmUgPSByZXF1aXJlKCdmYmpzL2xpYi9DU1NDb3JlJyk7XG52YXIgUmVhY3RUcmFuc2l0aW9uRXZlbnRzID0gcmVxdWlyZSgnLi9SZWFjdFRyYW5zaXRpb25FdmVudHMnKTtcblxudmFyIG9ubHlDaGlsZCA9IHJlcXVpcmUoJy4vb25seUNoaWxkJyk7XG5cbi8vIFdlIGRvbid0IHJlbW92ZSB0aGUgZWxlbWVudCBmcm9tIHRoZSBET00gdW50aWwgd2UgcmVjZWl2ZSBhbiBhbmltYXRpb25lbmQgb3Jcbi8vIHRyYW5zaXRpb25lbmQgZXZlbnQuIElmIHRoZSB1c2VyIHNjcmV3cyB1cCBhbmQgZm9yZ2V0cyB0byBhZGQgYW4gYW5pbWF0aW9uXG4vLyB0aGVpciBub2RlIHdpbGwgYmUgc3R1Y2sgaW4gdGhlIERPTSBmb3JldmVyLCBzbyB3ZSBkZXRlY3QgaWYgYW4gYW5pbWF0aW9uXG4vLyBkb2VzIG5vdCBzdGFydCBhbmQgaWYgaXQgZG9lc24ndCwgd2UganVzdCBjYWxsIHRoZSBlbmQgbGlzdGVuZXIgaW1tZWRpYXRlbHkuXG52YXIgVElDSyA9IDE3O1xuXG52YXIgUmVhY3RDU1NUcmFuc2l0aW9uR3JvdXBDaGlsZCA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgZGlzcGxheU5hbWU6ICdSZWFjdENTU1RyYW5zaXRpb25Hcm91cENoaWxkJyxcblxuICBwcm9wVHlwZXM6IHtcbiAgICBuYW1lOiBSZWFjdC5Qcm9wVHlwZXMub25lT2ZUeXBlKFtSZWFjdC5Qcm9wVHlwZXMuc3RyaW5nLCBSZWFjdC5Qcm9wVHlwZXMuc2hhcGUoe1xuICAgICAgZW50ZXI6IFJlYWN0LlByb3BUeXBlcy5zdHJpbmcsXG4gICAgICBsZWF2ZTogUmVhY3QuUHJvcFR5cGVzLnN0cmluZyxcbiAgICAgIGFjdGl2ZTogUmVhY3QuUHJvcFR5cGVzLnN0cmluZ1xuICAgIH0pLCBSZWFjdC5Qcm9wVHlwZXMuc2hhcGUoe1xuICAgICAgZW50ZXI6IFJlYWN0LlByb3BUeXBlcy5zdHJpbmcsXG4gICAgICBlbnRlckFjdGl2ZTogUmVhY3QuUHJvcFR5cGVzLnN0cmluZyxcbiAgICAgIGxlYXZlOiBSZWFjdC5Qcm9wVHlwZXMuc3RyaW5nLFxuICAgICAgbGVhdmVBY3RpdmU6IFJlYWN0LlByb3BUeXBlcy5zdHJpbmcsXG4gICAgICBhcHBlYXI6IFJlYWN0LlByb3BUeXBlcy5zdHJpbmcsXG4gICAgICBhcHBlYXJBY3RpdmU6IFJlYWN0LlByb3BUeXBlcy5zdHJpbmdcbiAgICB9KV0pLmlzUmVxdWlyZWQsXG5cbiAgICAvLyBPbmNlIHdlIHJlcXVpcmUgdGltZW91dHMgdG8gYmUgc3BlY2lmaWVkLCB3ZSBjYW4gcmVtb3ZlIHRoZVxuICAgIC8vIGJvb2xlYW4gZmxhZ3MgKGFwcGVhciBldGMuKSBhbmQganVzdCBhY2NlcHQgYSBudW1iZXJcbiAgICAvLyBvciBhIGJvb2wgZm9yIHRoZSB0aW1lb3V0IGZsYWdzIChhcHBlYXJUaW1lb3V0IGV0Yy4pXG4gICAgYXBwZWFyOiBSZWFjdC5Qcm9wVHlwZXMuYm9vbCxcbiAgICBlbnRlcjogUmVhY3QuUHJvcFR5cGVzLmJvb2wsXG4gICAgbGVhdmU6IFJlYWN0LlByb3BUeXBlcy5ib29sLFxuICAgIGFwcGVhclRpbWVvdXQ6IFJlYWN0LlByb3BUeXBlcy5udW1iZXIsXG4gICAgZW50ZXJUaW1lb3V0OiBSZWFjdC5Qcm9wVHlwZXMubnVtYmVyLFxuICAgIGxlYXZlVGltZW91dDogUmVhY3QuUHJvcFR5cGVzLm51bWJlclxuICB9LFxuXG4gIHRyYW5zaXRpb246IGZ1bmN0aW9uIChhbmltYXRpb25UeXBlLCBmaW5pc2hDYWxsYmFjaywgdXNlclNwZWNpZmllZERlbGF5KSB7XG4gICAgdmFyIG5vZGUgPSBSZWFjdERPTS5maW5kRE9NTm9kZSh0aGlzKTtcblxuICAgIGlmICghbm9kZSkge1xuICAgICAgaWYgKGZpbmlzaENhbGxiYWNrKSB7XG4gICAgICAgIGZpbmlzaENhbGxiYWNrKCk7XG4gICAgICB9XG4gICAgICByZXR1cm47XG4gICAgfVxuXG4gICAgdmFyIGNsYXNzTmFtZSA9IHRoaXMucHJvcHMubmFtZVthbmltYXRpb25UeXBlXSB8fCB0aGlzLnByb3BzLm5hbWUgKyAnLScgKyBhbmltYXRpb25UeXBlO1xuICAgIHZhciBhY3RpdmVDbGFzc05hbWUgPSB0aGlzLnByb3BzLm5hbWVbYW5pbWF0aW9uVHlwZSArICdBY3RpdmUnXSB8fCBjbGFzc05hbWUgKyAnLWFjdGl2ZSc7XG4gICAgdmFyIHRpbWVvdXQgPSBudWxsO1xuXG4gICAgdmFyIGVuZExpc3RlbmVyID0gZnVuY3Rpb24gKGUpIHtcbiAgICAgIGlmIChlICYmIGUudGFyZ2V0ICE9PSBub2RlKSB7XG4gICAgICAgIHJldHVybjtcbiAgICAgIH1cblxuICAgICAgY2xlYXJUaW1lb3V0KHRpbWVvdXQpO1xuXG4gICAgICBDU1NDb3JlLnJlbW92ZUNsYXNzKG5vZGUsIGNsYXNzTmFtZSk7XG4gICAgICBDU1NDb3JlLnJlbW92ZUNsYXNzKG5vZGUsIGFjdGl2ZUNsYXNzTmFtZSk7XG5cbiAgICAgIFJlYWN0VHJhbnNpdGlvbkV2ZW50cy5yZW1vdmVFbmRFdmVudExpc3RlbmVyKG5vZGUsIGVuZExpc3RlbmVyKTtcblxuICAgICAgLy8gVXN1YWxseSB0aGlzIG9wdGlvbmFsIGNhbGxiYWNrIGlzIHVzZWQgZm9yIGluZm9ybWluZyBhbiBvd25lciBvZlxuICAgICAgLy8gYSBsZWF2ZSBhbmltYXRpb24gYW5kIHRlbGxpbmcgaXQgdG8gcmVtb3ZlIHRoZSBjaGlsZC5cbiAgICAgIGlmIChmaW5pc2hDYWxsYmFjaykge1xuICAgICAgICBmaW5pc2hDYWxsYmFjaygpO1xuICAgICAgfVxuICAgIH07XG5cbiAgICBDU1NDb3JlLmFkZENsYXNzKG5vZGUsIGNsYXNzTmFtZSk7XG5cbiAgICAvLyBOZWVkIHRvIGRvIHRoaXMgdG8gYWN0dWFsbHkgdHJpZ2dlciBhIHRyYW5zaXRpb24uXG4gICAgdGhpcy5xdWV1ZUNsYXNzKGFjdGl2ZUNsYXNzTmFtZSk7XG5cbiAgICAvLyBJZiB0aGUgdXNlciBzcGVjaWZpZWQgYSB0aW1lb3V0IGRlbGF5LlxuICAgIGlmICh1c2VyU3BlY2lmaWVkRGVsYXkpIHtcbiAgICAgIC8vIENsZWFuLXVwIHRoZSBhbmltYXRpb24gYWZ0ZXIgdGhlIHNwZWNpZmllZCBkZWxheVxuICAgICAgdGltZW91dCA9IHNldFRpbWVvdXQoZW5kTGlzdGVuZXIsIHVzZXJTcGVjaWZpZWREZWxheSk7XG4gICAgICB0aGlzLnRyYW5zaXRpb25UaW1lb3V0cy5wdXNoKHRpbWVvdXQpO1xuICAgIH0gZWxzZSB7XG4gICAgICAvLyBERVBSRUNBVEVEOiB0aGlzIGxpc3RlbmVyIHdpbGwgYmUgcmVtb3ZlZCBpbiBhIGZ1dHVyZSB2ZXJzaW9uIG9mIHJlYWN0XG4gICAgICBSZWFjdFRyYW5zaXRpb25FdmVudHMuYWRkRW5kRXZlbnRMaXN0ZW5lcihub2RlLCBlbmRMaXN0ZW5lcik7XG4gICAgfVxuICB9LFxuXG4gIHF1ZXVlQ2xhc3M6IGZ1bmN0aW9uIChjbGFzc05hbWUpIHtcbiAgICB0aGlzLmNsYXNzTmFtZVF1ZXVlLnB1c2goY2xhc3NOYW1lKTtcblxuICAgIGlmICghdGhpcy50aW1lb3V0KSB7XG4gICAgICB0aGlzLnRpbWVvdXQgPSBzZXRUaW1lb3V0KHRoaXMuZmx1c2hDbGFzc05hbWVRdWV1ZSwgVElDSyk7XG4gICAgfVxuICB9LFxuXG4gIGZsdXNoQ2xhc3NOYW1lUXVldWU6IGZ1bmN0aW9uICgpIHtcbiAgICBpZiAodGhpcy5pc01vdW50ZWQoKSkge1xuICAgICAgdGhpcy5jbGFzc05hbWVRdWV1ZS5mb3JFYWNoKENTU0NvcmUuYWRkQ2xhc3MuYmluZChDU1NDb3JlLCBSZWFjdERPTS5maW5kRE9NTm9kZSh0aGlzKSkpO1xuICAgIH1cbiAgICB0aGlzLmNsYXNzTmFtZVF1ZXVlLmxlbmd0aCA9IDA7XG4gICAgdGhpcy50aW1lb3V0ID0gbnVsbDtcbiAgfSxcblxuICBjb21wb25lbnRXaWxsTW91bnQ6IGZ1bmN0aW9uICgpIHtcbiAgICB0aGlzLmNsYXNzTmFtZVF1ZXVlID0gW107XG4gICAgdGhpcy50cmFuc2l0aW9uVGltZW91dHMgPSBbXTtcbiAgfSxcblxuICBjb21wb25lbnRXaWxsVW5tb3VudDogZnVuY3Rpb24gKCkge1xuICAgIGlmICh0aGlzLnRpbWVvdXQpIHtcbiAgICAgIGNsZWFyVGltZW91dCh0aGlzLnRpbWVvdXQpO1xuICAgIH1cbiAgICB0aGlzLnRyYW5zaXRpb25UaW1lb3V0cy5mb3JFYWNoKGZ1bmN0aW9uICh0aW1lb3V0KSB7XG4gICAgICBjbGVhclRpbWVvdXQodGltZW91dCk7XG4gICAgfSk7XG4gIH0sXG5cbiAgY29tcG9uZW50V2lsbEFwcGVhcjogZnVuY3Rpb24gKGRvbmUpIHtcbiAgICBpZiAodGhpcy5wcm9wcy5hcHBlYXIpIHtcbiAgICAgIHRoaXMudHJhbnNpdGlvbignYXBwZWFyJywgZG9uZSwgdGhpcy5wcm9wcy5hcHBlYXJUaW1lb3V0KTtcbiAgICB9IGVsc2Uge1xuICAgICAgZG9uZSgpO1xuICAgIH1cbiAgfSxcblxuICBjb21wb25lbnRXaWxsRW50ZXI6IGZ1bmN0aW9uIChkb25lKSB7XG4gICAgaWYgKHRoaXMucHJvcHMuZW50ZXIpIHtcbiAgICAgIHRoaXMudHJhbnNpdGlvbignZW50ZXInLCBkb25lLCB0aGlzLnByb3BzLmVudGVyVGltZW91dCk7XG4gICAgfSBlbHNlIHtcbiAgICAgIGRvbmUoKTtcbiAgICB9XG4gIH0sXG5cbiAgY29tcG9uZW50V2lsbExlYXZlOiBmdW5jdGlvbiAoZG9uZSkge1xuICAgIGlmICh0aGlzLnByb3BzLmxlYXZlKSB7XG4gICAgICB0aGlzLnRyYW5zaXRpb24oJ2xlYXZlJywgZG9uZSwgdGhpcy5wcm9wcy5sZWF2ZVRpbWVvdXQpO1xuICAgIH0gZWxzZSB7XG4gICAgICBkb25lKCk7XG4gICAgfVxuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24gKCkge1xuICAgIHJldHVybiBvbmx5Q2hpbGQodGhpcy5wcm9wcy5jaGlsZHJlbik7XG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IFJlYWN0Q1NTVHJhbnNpdGlvbkdyb3VwQ2hpbGQ7XG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiAuL34vcmVhY3QvbGliL1JlYWN0Q1NTVHJhbnNpdGlvbkdyb3VwQ2hpbGQuanNcbiAqKiBtb2R1bGUgaWQgPSAzODdcbiAqKiBtb2R1bGUgY2h1bmtzID0gMVxuICoqLyIsIihmdW5jdGlvbihob3N0KSB7XG5cbiAgdmFyIHByb3BlcnRpZXMgPSB7XG4gICAgYnJvd3NlcjogW1xuICAgICAgWy9tc2llIChbXFwuXFxfXFxkXSspLywgXCJpZVwiXSxcbiAgICAgIFsvdHJpZGVudFxcLy4qP3J2OihbXFwuXFxfXFxkXSspLywgXCJpZVwiXSxcbiAgICAgIFsvZmlyZWZveFxcLyhbXFwuXFxfXFxkXSspLywgXCJmaXJlZm94XCJdLFxuICAgICAgWy9jaHJvbWVcXC8oW1xcLlxcX1xcZF0rKS8sIFwiY2hyb21lXCJdLFxuICAgICAgWy92ZXJzaW9uXFwvKFtcXC5cXF9cXGRdKykuKj9zYWZhcmkvLCBcInNhZmFyaVwiXSxcbiAgICAgIFsvbW9iaWxlIHNhZmFyaSAoW1xcLlxcX1xcZF0rKS8sIFwic2FmYXJpXCJdLFxuICAgICAgWy9hbmRyb2lkLio/dmVyc2lvblxcLyhbXFwuXFxfXFxkXSspLio/c2FmYXJpLywgXCJjb20uYW5kcm9pZC5icm93c2VyXCJdLFxuICAgICAgWy9jcmlvc1xcLyhbXFwuXFxfXFxkXSspLio/c2FmYXJpLywgXCJjaHJvbWVcIl0sXG4gICAgICBbL29wZXJhLywgXCJvcGVyYVwiXSxcbiAgICAgIFsvb3BlcmFcXC8oW1xcLlxcX1xcZF0rKS8sIFwib3BlcmFcIl0sXG4gICAgICBbL29wZXJhIChbXFwuXFxfXFxkXSspLywgXCJvcGVyYVwiXSxcbiAgICAgIFsvb3BlcmEgbWluaS4qP3ZlcnNpb25cXC8oW1xcLlxcX1xcZF0rKS8sIFwib3BlcmEubWluaVwiXSxcbiAgICAgIFsvb3Bpb3NcXC8oW2EtelxcLlxcX1xcZF0rKS8sIFwib3BlcmFcIl0sXG4gICAgICBbL2JsYWNrYmVycnkvLCBcImJsYWNrYmVycnlcIl0sXG4gICAgICBbL2JsYWNrYmVycnkuKj92ZXJzaW9uXFwvKFtcXC5cXF9cXGRdKykvLCBcImJsYWNrYmVycnlcIl0sXG4gICAgICBbL2JiXFxkKy4qP3ZlcnNpb25cXC8oW1xcLlxcX1xcZF0rKS8sIFwiYmxhY2tiZXJyeVwiXSxcbiAgICAgIFsvcmltLio/dmVyc2lvblxcLyhbXFwuXFxfXFxkXSspLywgXCJibGFja2JlcnJ5XCJdLFxuICAgICAgWy9pY2V3ZWFzZWxcXC8oW1xcLlxcX1xcZF0rKS8sIFwiaWNld2Vhc2VsXCJdLFxuICAgICAgWy9lZGdlXFwvKFtcXC5cXGRdKykvLCBcImVkZ2VcIl1cbiAgICBdLFxuICAgIG9zOiBbXG4gICAgICBbL2xpbnV4ICgpKFthLXpcXC5cXF9cXGRdKykvLCBcImxpbnV4XCJdLFxuICAgICAgWy9tYWMgb3MgeC8sIFwibWFjb3NcIl0sXG4gICAgICBbL21hYyBvcyB4Lio/KFtcXC5cXF9cXGRdKykvLCBcIm1hY29zXCJdLFxuICAgICAgWy9vcyAoW1xcLlxcX1xcZF0rKSBsaWtlIG1hYyBvcy8sIFwiaW9zXCJdLFxuICAgICAgWy9vcGVuYnNkICgpKFthLXpcXC5cXF9cXGRdKykvLCBcIm9wZW5ic2RcIl0sXG4gICAgICBbL2FuZHJvaWQvLCBcImFuZHJvaWRcIl0sXG4gICAgICBbL2FuZHJvaWQgKFthLXpcXC5cXF9cXGRdKyk7LywgXCJhbmRyb2lkXCJdLFxuICAgICAgWy9tb3ppbGxhXFwvW2EtelxcLlxcX1xcZF0rIFxcKCg/Om1vYmlsZSl8KD86dGFibGV0KS8sIFwiZmlyZWZveG9zXCJdLFxuICAgICAgWy93aW5kb3dzXFxzKig/Om50KT9cXHMqKFtcXC5cXF9cXGRdKykvLCBcIndpbmRvd3NcIl0sXG4gICAgICBbL3dpbmRvd3MgcGhvbmUuKj8oW1xcLlxcX1xcZF0rKS8sIFwid2luZG93cy5waG9uZVwiXSxcbiAgICAgIFsvd2luZG93cyBtb2JpbGUvLCBcIndpbmRvd3MubW9iaWxlXCJdLFxuICAgICAgWy9ibGFja2JlcnJ5LywgXCJibGFja2JlcnJ5b3NcIl0sXG4gICAgICBbL2JiXFxkKy8sIFwiYmxhY2tiZXJyeW9zXCJdLFxuICAgICAgWy9yaW0uKj9vc1xccyooW1xcLlxcX1xcZF0rKS8sIFwiYmxhY2tiZXJyeW9zXCJdXG4gICAgXSxcbiAgICBkZXZpY2U6IFtcbiAgICAgIFsvaXBhZC8sIFwiaXBhZFwiXSxcbiAgICAgIFsvaXBob25lLywgXCJpcGhvbmVcIl0sXG4gICAgICBbL2x1bWlhLywgXCJsdW1pYVwiXSxcbiAgICAgIFsvaHRjLywgXCJodGNcIl0sXG4gICAgICBbL25leHVzLywgXCJuZXh1c1wiXSxcbiAgICAgIFsvZ2FsYXh5IG5leHVzLywgXCJnYWxheHkubmV4dXNcIl0sXG4gICAgICBbL25va2lhLywgXCJub2tpYVwiXSxcbiAgICAgIFsvIGd0XFwtLywgXCJnYWxheHlcIl0sXG4gICAgICBbLyBzbVxcLS8sIFwiZ2FsYXh5XCJdLFxuICAgICAgWy94Ym94LywgXCJ4Ym94XCJdLFxuICAgICAgWy8oPzpiYlxcZCspfCg/OmJsYWNrYmVycnkpfCg/OiByaW0gKS8sIFwiYmxhY2tiZXJyeVwiXVxuICAgIF1cbiAgfTtcblxuICB2YXIgVU5LTk9XTiA9IFwiVW5rbm93blwiO1xuXG4gIHZhciBwcm9wZXJ0eU5hbWVzID0gT2JqZWN0LmtleXMocHJvcGVydGllcyk7XG5cbiAgZnVuY3Rpb24gU25pZmZyKCkge1xuICAgIHZhciBzZWxmID0gdGhpcztcblxuICAgIHByb3BlcnR5TmFtZXMuZm9yRWFjaChmdW5jdGlvbihwcm9wZXJ0eU5hbWUpIHtcbiAgICAgIHNlbGZbcHJvcGVydHlOYW1lXSA9IHtcbiAgICAgICAgbmFtZTogVU5LTk9XTixcbiAgICAgICAgdmVyc2lvbjogW10sXG4gICAgICAgIHZlcnNpb25TdHJpbmc6IFVOS05PV05cbiAgICAgIH07XG4gICAgfSk7XG4gIH1cblxuICBmdW5jdGlvbiBkZXRlcm1pbmVQcm9wZXJ0eShzZWxmLCBwcm9wZXJ0eU5hbWUsIHVzZXJBZ2VudCkge1xuICAgIHByb3BlcnRpZXNbcHJvcGVydHlOYW1lXS5mb3JFYWNoKGZ1bmN0aW9uKHByb3BlcnR5TWF0Y2hlcikge1xuICAgICAgdmFyIHByb3BlcnR5UmVnZXggPSBwcm9wZXJ0eU1hdGNoZXJbMF07XG4gICAgICB2YXIgcHJvcGVydHlWYWx1ZSA9IHByb3BlcnR5TWF0Y2hlclsxXTtcblxuICAgICAgdmFyIG1hdGNoID0gdXNlckFnZW50Lm1hdGNoKHByb3BlcnR5UmVnZXgpO1xuXG4gICAgICBpZiAobWF0Y2gpIHtcbiAgICAgICAgc2VsZltwcm9wZXJ0eU5hbWVdLm5hbWUgPSBwcm9wZXJ0eVZhbHVlO1xuXG4gICAgICAgIGlmIChtYXRjaFsyXSkge1xuICAgICAgICAgIHNlbGZbcHJvcGVydHlOYW1lXS52ZXJzaW9uU3RyaW5nID0gbWF0Y2hbMl07XG4gICAgICAgICAgc2VsZltwcm9wZXJ0eU5hbWVdLnZlcnNpb24gPSBbXTtcbiAgICAgICAgfSBlbHNlIGlmIChtYXRjaFsxXSkge1xuICAgICAgICAgIHNlbGZbcHJvcGVydHlOYW1lXS52ZXJzaW9uU3RyaW5nID0gbWF0Y2hbMV0ucmVwbGFjZSgvXy9nLCBcIi5cIik7XG4gICAgICAgICAgc2VsZltwcm9wZXJ0eU5hbWVdLnZlcnNpb24gPSBwYXJzZVZlcnNpb24obWF0Y2hbMV0pO1xuICAgICAgICB9IGVsc2Uge1xuICAgICAgICAgIHNlbGZbcHJvcGVydHlOYW1lXS52ZXJzaW9uU3RyaW5nID0gVU5LTk9XTjtcbiAgICAgICAgICBzZWxmW3Byb3BlcnR5TmFtZV0udmVyc2lvbiA9IFtdO1xuICAgICAgICB9XG4gICAgICB9XG4gICAgfSk7XG4gIH1cblxuICBmdW5jdGlvbiBwYXJzZVZlcnNpb24odmVyc2lvblN0cmluZykge1xuICAgIHJldHVybiB2ZXJzaW9uU3RyaW5nLnNwbGl0KC9bXFwuX10vKS5tYXAoZnVuY3Rpb24odmVyc2lvblBhcnQpIHtcbiAgICAgIHJldHVybiBwYXJzZUludCh2ZXJzaW9uUGFydCk7XG4gICAgfSk7XG4gIH1cblxuICBTbmlmZnIucHJvdG90eXBlLnNuaWZmID0gZnVuY3Rpb24odXNlckFnZW50U3RyaW5nKSB7XG4gICAgdmFyIHNlbGYgPSB0aGlzO1xuICAgIHZhciB1c2VyQWdlbnQgPSAodXNlckFnZW50U3RyaW5nIHx8IG5hdmlnYXRvci51c2VyQWdlbnQgfHwgXCJcIikudG9Mb3dlckNhc2UoKTtcblxuICAgIHByb3BlcnR5TmFtZXMuZm9yRWFjaChmdW5jdGlvbihwcm9wZXJ0eU5hbWUpIHtcbiAgICAgIGRldGVybWluZVByb3BlcnR5KHNlbGYsIHByb3BlcnR5TmFtZSwgdXNlckFnZW50KTtcbiAgICB9KTtcbiAgfTtcblxuXG4gIGlmICh0eXBlb2YgbW9kdWxlICE9PSAndW5kZWZpbmVkJyAmJiBtb2R1bGUuZXhwb3J0cykge1xuICAgIG1vZHVsZS5leHBvcnRzID0gU25pZmZyO1xuICB9IGVsc2Uge1xuICAgIGhvc3QuU25pZmZyID0gbmV3IFNuaWZmcigpO1xuICAgIGhvc3QuU25pZmZyLnNuaWZmKG5hdmlnYXRvci51c2VyQWdlbnQpO1xuICB9XG59KSh0aGlzKTtcblxuXG5cbi8qKioqKioqKioqKioqKioqKlxuICoqIFdFQlBBQ0sgRk9PVEVSXG4gKiogLi9+L3NuaWZmci9zcmMvc25pZmZyLmpzXG4gKiogbW9kdWxlIGlkID0gNDM0XG4gKiogbW9kdWxlIGNodW5rcyA9IDFcbiAqKi8iLCI7XG52YXIgc3ByaXRlID0gcmVxdWlyZShcIi9ob21lL2Frb250c2V2b3kvZ28vc3JjL2dpdGh1Yi5jb20vZ3Jhdml0YXRpb25hbC90ZWxlcG9ydC93ZWIvbm9kZV9tb2R1bGVzL3N2Zy1zcHJpdGUtbG9hZGVyL2xpYi93ZWIvZ2xvYmFsLXNwcml0ZVwiKTs7XG52YXIgaW1hZ2UgPSBcIjxzeW1ib2wgdmlld0JveD1cXFwiMCAwIDM0MCAxMDBcXFwiIGlkPVxcXCJncnYtdGxwdC1sb2dvLWZ1bGxcXFwiIHhtbG5zOnhsaW5rPVxcXCJodHRwOi8vd3d3LnczLm9yZy8xOTk5L3hsaW5rXFxcIj4gPGc+IDxnIGlkPVxcXCJncnYtdGxwdC1sb2dvLWZ1bGxfTGF5ZXJfMlxcXCI+IDxnPiA8Zz4gPHBhdGggZD1cXFwibTQ3LjY3MTAwMSwyMS40NDRjLTcuMzk2LDAgLTE0LjEwMjAwMSwzLjAwNzk5OSAtMTguOTYwMDAzLDcuODY2MDAxYy00Ljg1Njk5OCw0Ljg1Njk5OCAtNy44NjU5OTksMTEuNTYzIC03Ljg2NTk5OSwxOC45NTk5OTljMCw3LjM5NiAzLjAwODAwMSwxNC4xMDEwMDIgNy44NjU5OTksMTguOTU3OTk2czExLjU2NDAwMyw3Ljg2NTAwNSAxOC45NjAwMDMsNy44NjUwMDVzMTQuMTAyMDAxLC0zLjAwODAwMyAxOC45NTg5OTYsLTcuODY1MDA1czcuODY1MDA1LC0xMS41NjE5OTYgNy44NjUwMDUsLTE4Ljk1Nzk5NnMtMy4wMDgwMDMsLTE0LjEwNCAtNy44NjUwMDUsLTE4Ljk1OTk5OWMtNC44NTc5OTQsLTQuODU4MDAyIC0xMS41NjI5OTYsLTcuODY2MDAxIC0xOC45NTg5OTYsLTcuODY2MDAxem0xMS4zODY5OTcsMTkuNTA5OTk4aC04LjIxMzk5N3YyMy4xODAwMDRoLTYuMzQ0MDAydi0yMy4xODAwMDRoLTguMjE1di01LjYxMmgyMi43NzI5OTl2NS42MTJsMCwwelxcXCIvPiA8L2c+IDxnPiA8cGF0aCBkPVxcXCJtOTIuNzgyOTk3LDYzLjM1NzAwMmMtMC4wOTg5OTksLTAuMzcxMDAyIC0wLjMyMDk5OSwtMC43MDkgLTAuNjQ2OTk2LC0wLjk0MjAwMWwtNC41NjIwMDQsLTMuOTU4bC00LjU2MTk5NiwtMy45NTcwMDFjMC4xNjMwMDIsLTAuODg3MDAxIDAuMjY3OTk4LC0xLjgwNSAwLjMzMTAwMSwtMi43MzZjMC4wNjM5OTUsLTAuOTMxIDAuMDg2OTk4LC0xLjg3NDAwMSAwLjA4Njk5OCwtMi44MDVjMCwtMC45MzI5OTkgLTAuMDIyMDAzLC0xLjg3NSAtMC4wODY5OTgsLTIuODA2OTk5Yy0wLjA2MzAwNCwtMC45MzE5OTkgLTAuMTY3OTk5LC0xLjg1MTAwMiAtMC4zMzEwMDEsLTIuNzM2bDQuNTYxOTk2LC0zLjk1NzAwMWw0LjU2MjAwNCwtMy45NThjMC4zMjU5OTYsLTAuMjMyOTk4IDAuNTQ4OTk2LC0wLjU3IDAuNjQ2OTk2LC0wLjk0MjAwMWMwLjA5OTAwNywtMC4zNzI5OTcgMC4wNzUwMDUsLTAuNzc4OTk5IC0wLjA4Nzk5NywtMS4xNTNjLTAuOTMxOTk5LC0yLjg2MiAtMi4xOTk5OTcsLTUuNjU1OTk4IC0zLjczMTAwMywtOC4yOTljLTEuNTMwOTk4LC0yLjY0MTk5OCAtMy4zMjE5OTksLTUuMTMyOTk4IC01LjMwMTk5NCwtNy4zOTA5OTljLTAuMjc4OTk5LC0wLjMyNiAtMC42MTcwMDQsLTAuNTQ4IC0wLjk3ODAwNCwtMC42NDZjLTAuMzYwMDAxLC0wLjA5ODk5OSAtMC43NDQ5OTUsLTAuMDc0OTk5IC0xLjExNjk5NywwLjA4N2wtNS43NTA5OTksMi4wMDIwMDFsLTUuNzQ5MDAxLDIuMDAwOTk5Yy0xLjQxOTk5OCwtMS4xNjQgLTIuOTMzOTk4LC0yLjIxMSAtNC41MjIwMDMsLTMuMTM2OTk5Yy0xLjU4OTk5NiwtMC45MjUwMDEgLTMuMjUzOTk4LC0xLjcyODAwMSAtNC45Nzc5OTcsLTIuNDA0MDAxbC0xLjEzOTk5OSwtNS45NTlsLTEuMTQwOTk5LC01Ljk1OWMtMC4wNjksLTAuMzczIC0wLjI2ODAwNSwtMC43MzMgLTAuNTQ3MDA1LC0xLjAxM2MtMC4yNzg5OTksLTAuMjggLTAuNjQwOTk5LC0wLjQ3OCAtMS4wMzY5OTUsLTAuNTI0Yy0yLjk4MDAwMywtMC42MDUgLTYuMDA3MDA0LC0wLjkwOCAtOS4wMzMwMDUsLTAuOTA4cy02LjA1Mjk5OCwwLjMwMiAtOS4wMzI5OTcsMC45MDhjLTAuMzk2LDAuMDQ2IC0wLjc1NjAwMSwwLjI0NTAwMSAtMS4wMzYwMDMsMC41MjRjLTAuMjc4OTk5LDAuMjc5IC0wLjQ3Nzk5NywwLjY0IC0wLjU0Njk5NywxLjAxM2wtMS4xNDEwMDMsNS45NTlsLTEuMTQwOTk5LDUuOTYwMDAxYy0xLjcyMywwLjY3NTk5OSAtMy40MTA5OTksMS40NzkgLTUuMDEyMDAxLDIuNDAzOTk5Yy0xLjU5OTk5OCwwLjkyNDk5OSAtMy4xMTI5OTksMS45NzMgLTQuNDg3LDMuMTM2OTk5bC01Ljc1LC0yLjAwMDk5OWwtNS43NSwtMi4wMDE5OTljLTAuMzcyLC0wLjE2NDAwMSAtMC43NTU5OTksLTAuMTg3IC0xLjExNjk5OSwtMC4wODgwMDFjLTAuMzYxLDAuMSAtMC42OTkwMDEsMC4zMiAtMC45NzgwMDEsMC42NDZjLTEuOTc5LDIuMjU5MDAxIC0zLjc3MSw0Ljc1IC01LjMwMiw3LjM5MjAwMmMtMS41MywyLjY0MTk5OCAtMi43OTksNS40MzY5OTYgLTMuNzMsOC4yOTljLTAuMTYzLDAuMzcyOTk3IC0wLjE4NywwLjc4MDk5OCAtMC4wODcwMDEsMS4xNTE5OTdjMC4wOTksMC4zNzIwMDIgMC4zMjAwMDEsMC43MTAwMDMgMC42NDYwMDEsMC45NDMwMDFsNC41NjMsMy45NTcwMDFsNC41NjIsMy45NThjLTAuMTYzLDAuODg0OTk4IC0wLjI2OCwxLjgwNDAwMSAtMC4zMzEwMDEsMi43MzUwMDFjLTAuMDYzOTk5LDAuOTMxOTk5IC0wLjA4Nzk5OSwxLjg3NSAtMC4wODc5OTksMi44MDZzMC4wMjMwMDEsMS44NzUgMC4wODcsMi44MDZjMC4wNjQwMDEsMC45MzE5OTkgMC4xNjgwMDEsMS44NTEwMDIgMC4zMzIwMDEsMi43MzUwMDFsLTQuNTYyLDMuOTU3MDAxbC00LjU2MiwzLjk1OWMtMC4zMjUsMC4yMzEwMDMgLTAuNTQ3LDAuNTY5IC0wLjY0NiwwLjk0MjAwMWMtMC4wOTksMC4zNzA5OTUgLTAuMDc2LDAuNzc4OTk5IDAuMDg3LDEuMTUwMDAyYzAuOTMxLDIuODY0OTk4IDIuMiw1LjY1Nzk5NyAzLjczLDguMzAwOTk1YzEuNTMxLDIuNjQyOTk4IDMuMzIzLDUuMTMzMDAzIDUuMzAyLDcuMzkxOTk4YzAuMjgwMDAxLDAuMzI1MDA1IDAuNjE4LDAuNTQ4MDA0IDAuOTc4MDAxLDAuNjQ2MDA0YzAuMzYxLDAuMDk5OTk4IDAuNzQ0OTk5LDAuMDc0OTk3IDEuMTE4LC0wLjA4Nzk5N2w1Ljc1LC0yLjAwMzAwNmw1Ljc0OTk5OCwtMi4wMDA5OTljMS4zNzMwMDEsMS4xNjQwMDEgMi44ODYwMDIsMi4yMTMwMDUgNC40ODcwMDMsMy4xMzljMS42MDA5OTgsMC45MjQwMDQgMy4yODg5OTgsMS43MjgwMDQgNS4wMTA5OTgsMi40MDEwMDFsMS4xNDA5OTksNS45NjE5OThsMS4xNDEwMDMsNS45NTljMC4wNywwLjM3MjAwMiAwLjI2Nzk5OCwwLjczMzAwMiAwLjU0NzAwMSwxLjAxNGMwLjI3ODk5OSwwLjI3OTAwNyAwLjY0MDk5OSwwLjQ3OTAwNCAxLjAzNTk5OSwwLjUyMjAwM2MxLjQ4OTk5OCwwLjI3OCAyLjk3OSwwLjUwMDk5OSA0LjQ4MDk5OSwwLjY1MTAwMWMxLjUwMDk5OSwwLjE1MiAzLjAxNDk5OSwwLjIzMjAwMiA0LjU1MTk5OCwwLjIzMjAwMnMzLjA0OTAwNCwtMC4wODAwMDIgNC41NTEwMDMsLTAuMjMyMDAyYzEuNTAxOTk5LC0wLjE1MDAwMiAyLjk5MDk5NywtMC4zNzMwMDEgNC40Nzk5OTYsLTAuNjUxMDAxYzAuMzk2MDA0LC0wLjA0NDk5OCAwLjc1NzAwNCwtMC4yNDM5OTYgMS4wMzcwMDMsLTAuNTIyMDAzYzAuMjc5OTk5LC0wLjI3ODk5OSAwLjQ3Njk5NywtMC42NDE5OTggMC41NDcwMDUsLTEuMDE0bDEuMTQwOTk5LC01Ljk1OWwxLjE0MDk5OSwtNS45NjE5OThjMS43MjMsLTAuNjc0OTk1IDMuMzg3MDAxLC0xLjQ3Nzk5NyA0Ljk3Njk5NywtMi40MDEwMDFjMS41ODgwMDUsLTAuOTI1OTk1IDMuMTAzMDA0LC0xLjk3NDk5OCA0LjUyMjAwMywtMy4xMzlsNS43NSwyLjAwMDk5OWw1Ljc1LDIuMDAzMDA2YzAuMzczMDAxLDAuMTYyOTk0IDAuNzU2OTk2LDAuMTg1OTk3IDEuMTE3OTk2LDAuMDg3OTk3YzAuMzYwMDAxLC0wLjA5ODk5OSAwLjY5ODAwNiwtMC4zMiAwLjk3ODAwNCwtMC42NDYwMDRjMS45Nzg5OTYsLTIuMjU4OTk1IDMuNzcwOTk2LC00Ljc0OTAwMSA1LjMwMTk5NCwtNy4zOTE5OThjMS41MzEwMDYsLTIuNjQyOTk4IDIuODAwMDAzLC01LjQzNjk5NiAzLjczMTAwMywtOC4zMDA5OTVjMC4xNjQwMDEsLTAuMzY4MDA0IDAuMTg4MDA0LC0wLjc3ODAwOCAwLjA4Nzk5NywtMS4xNTAwMDJ6bS0yNC4yMzc5OTksNS43ODc5OTRjLTUuMzQ4LDUuMzQ5MDA3IC0xMi43MzE5OTUsOC42NjAwMDQgLTIwLjg3NSw4LjY2MDAwNGMtOC4xNDM5OTcsMCAtMTUuNTI2OTk3LC0zLjMxMjAwNCAtMjAuODc1LC04LjY2MDAwNHMtOC42NTk5OTgsLTEyLjczMDk5NSAtOC42NTk5OTgsLTIwLjg3NDk5NmMwLC04LjE0NDAwMSAzLjMxMiwtMTUuNTI3IDguNjYxMDAxLC0yMC44NzU5OTljNS4zNDgsLTUuMzQ4MDAxIDEyLjczMTk5OCwtOC42NjEwMDEgMjAuODc1OTk5LC04LjY2MTAwMWM4LjE0MzAwMiwwIDE1LjUyNTk5NywzLjMxMiAyMC44NzQ5OTYsOC42NjEwMDFjNS4zNDgsNS4zNDg5OTkgOC42NjEwMDMsMTIuNzMxOTk4IDguNjYxMDAzLDIwLjg3NTk5OWMtMC4wMDA5OTksOC4xNDE5OTggLTMuMzE0MDAzLDE1LjUyNTk5NyAtOC42NjMwMDIsMjAuODc0OTk2elxcXCIvPiA8L2c+IDwvZz4gPC9nPiA8Zz4gPHBhdGggZD1cXFwibTExOS43NzMwMDMsMzAuODYxaC0xMy4wMjAwMDR2LTYuODQxaDMzLjU5OTk5OHY2Ljg0MWgtMTMuMDIwMDA0djM1LjYzOTk5OWgtNy41NTk5OXYtMzUuNjM5OTk5bDAsMHpcXFwiLz4gPHBhdGggZD1cXFwibTE0My45NTMwMDMsNTQuNjIwOTk4YzAuMjM5OTksMi4xNiAxLjA4MDAwMiwzLjg0IDIuNTIwMDA0LDUuMDM5OTk3czMuMTc5OTkzLDEuODAwMDAzIDUuMjE5OTg2LDEuODAwMDAzYzEuODAwMDAzLDAgMy4zMDkwMDYsLTAuMzY4OTk2IDQuNTMwMDE0LC0xLjExMDAwMWMxLjIxOTk4NiwtMC43Mzg5OTggMi4yODk5OTMsLTEuNjY4OTk5IDMuMjA5OTkxLC0yLjc5MDAwMWw1LjE2MDAwNCwzLjkwMDAwMmMtMS42ODAwMDgsMi4wODAwMDIgLTMuNTYxMDA1LDMuNTYxMDA1IC01LjYzOTk5OSw0LjQ0MDAwMmMtMi4wODAwMDIsMC44Nzg5OTggLTQuMjYwMDEsMS4zMTkgLTYuNTQwMDA5LDEuMzE5Yy0yLjE1OTk4OCwwIC00LjE5OTk5NywtMC4zNTkwMDEgLTYuMTE5OTk1LC0xLjA4MDAwMmMtMS45MTk5OTgsLTAuNzIwMDAxIC0zLjU4MDk5NCwtMS43Mzg5OTggLTQuOTc5OTk2LC0zLjA1OTk5OGMtMS40MDEwMDEsLTEuMzIwMDA3IC0yLjUxMTAwMiwtMi45MTAwMDQgLTMuMzMwMDAyLC00Ljc3MTAwNGMtMC44MjAwMDcsLTEuODU4OTk3IC0xLjIyOTk5NiwtMy45Mjk5OTYgLTEuMjI5OTk2LC02LjIwOTk5OWMwLC0yLjI3ODk5OSAwLjQwOTk4OCwtNC4zNDk5OTggMS4yMjk5OTYsLTYuMjA5OTk5YzAuODE5LC0xLjg1OTAwMSAxLjkyOTAwMSwtMy40NDkwMDEgMy4zMzAwMDIsLTQuNzdjMS4zOTkwMDIsLTEuMzIgMy4wNTk5OTgsLTIuMzQgNC45Nzk5OTYsLTMuMDYxMDAxYzEuOTE5OTk4LC0wLjcxOTk5NyAzLjk2MDAwNywtMS4wNzg5OTkgNi4xMTk5OTUsLTEuMDc4OTk5YzIsMCAzLjgzMDAwMiwwLjM1MTAwMiA1LjQ5MDAwNSwxLjA0OTk5OWMxLjY1ODk5NywwLjcwMDAwMSAzLjA4MDAwMiwxLjcwOTk5OSA0LjI1OTk5NSwzLjAyODk5OWMxLjE4MDAwOCwxLjMyIDIuMTAwMDA2LDIuOTUxIDIuNzYwMDEsNC44OTEwMDNjMC42NTk5ODgsMS45Mzk5OTkgMC45ODk5OSw0LjE2OTk5OCAwLjk4OTk5LDYuNjg4OTk5djEuOThoLTIxLjk1OTk5MWwwLDAuMDAyOTk4em0xNC43NTk5OTUsLTUuMzk5OTk4Yy0wLjA0MSwtMi4xMTg5OTkgLTAuNjk5OTk3LC0zLjc4OTAwMSAtMS45Nzk5OTYsLTUuMDEwMDAyYy0xLjI4MTAwNiwtMS4yMTk5OTcgLTMuMDU5OTk4LC0xLjgyOTk5OCAtNS4zMzk5OTYsLTEuODI5OTk4Yy0yLjE2MDAwNCwwIC0zLjg3MDAxLDAuNjIwOTk4IC01LjEzMDAwNSwxLjg2MDAwMWMtMS4yNTk5OTUsMS4yMzk5OTggLTIuMDMxMDA2LDIuODk5OTk4IC0yLjMwOTk5OCw0Ljk3OWgxNC43NTk5OTVsMCwwLjAwMDk5OXpcXFwiLz4gPHBhdGggZD1cXFwibTE3Mi43NTMwMDYsMjEuMTQxMDAxaDcuMTk5OTk3djQ1LjM1OTk5OWgtNy4xOTk5OTd2LTQ1LjM1OTk5OWwwLDB6XFxcIi8+IDxwYXRoIGQ9XFxcIm0xOTMuOTkyMDA0LDU0LjYyMDk5OGMwLjIzOTk5LDIuMTYgMS4wODAwMDIsMy44NCAyLjUxOTk4OSw1LjAzOTk5N2MxLjQ0MDAwMiwxLjIwMDAwNSAzLjE4MSwxLjgwMDAwMyA1LjIyMTAwOCwxLjgwMDAwM2MxLjgwMDAwMywwIDMuMzA5MDA2LC0wLjM2ODk5NiA0LjUyODk5MiwtMS4xMTAwMDFjMS4yMjEwMDgsLTAuNzM4OTk4IDIuMjkwMDA5LC0xLjY2ODk5OSAzLjIxMTAxNCwtMi43OTAwMDFsNS4xNTk5ODgsMy45MDAwMDJjLTEuNjgxLDIuMDgwMDAyIC0zLjU2MDk4OSwzLjU2MTAwNSAtNS42NDA5OTEsNC40NDAwMDJjLTIuMDgwMDAyLDAuODc4OTk4IC00LjI2MDAxLDEuMzE5IC02LjU0MDAwOSwxLjMxOWMtMi4xNTg5OTcsMCAtNC4xOTk5OTcsLTAuMzU5MDAxIC02LjExOTk5NSwtMS4wODAwMDJjLTEuOTE5OTk4LC0wLjcyMDAwMSAtMy41ODAwMDIsLTEuNzM4OTk4IC00Ljk3OTAwNCwtMy4wNTk5OThjLTEuNDAxMDAxLC0xLjMyMDAwNyAtMi41MTEwMDIsLTIuOTEwMDA0IC0zLjMzMDAwMiwtNC43NzEwMDRjLTAuODE5OTkyLC0xLjg1ODk5NyAtMS4yMjg5ODksLTMuOTI5OTk2IC0xLjIyODk4OSwtNi4yMDk5OTljMCwtMi4yNzg5OTkgMC40MDg5OTcsLTQuMzQ5OTk4IDEuMjI4OTg5LC02LjIwOTk5OWMwLjgxOSwtMS44NTkwMDEgMS45MjkwMDEsLTMuNDQ5MDAxIDMuMzMwMDAyLC00Ljc3YzEuMzk5MDAyLC0xLjMyIDMuMDU5OTk4LC0yLjM0IDQuOTc5MDA0LC0zLjA2MTAwMWMxLjkxOTk5OCwtMC43MTk5OTcgMy45NjA5OTksLTEuMDc4OTk5IDYuMTE5OTk1LC0xLjA3ODk5OWMyLDAgMy44MzAwMDIsMC4zNTEwMDIgNS40OTAwMDUsMS4wNDk5OTljMS42NTg5OTcsMC43MDAwMDEgMy4wNzg5OTUsMS43MDk5OTkgNC4yNTk5OTUsMy4wMjg5OTljMS4xODAwMDgsMS4zMiAyLjEwMDk5OCwyLjk1MSAyLjc2MTAwMiw0Ljg5MTAwM2MwLjY2MDAwNCwxLjkzOTk5OSAwLjk4ODk5OCw0LjE2OTk5OCAwLjk4ODk5OCw2LjY4ODk5OXYxLjk4aC0yMS45NTk5OTFsMCwwLjAwMjk5OHptMTQuNzU5OTk1LC01LjM5OTk5OGMtMC4wMzk5OTMsLTIuMTE4OTk5IC0wLjY5OTAwNSwtMy43ODkwMDEgLTEuOTc5MDA0LC01LjAxMDAwMmMtMS4yNzk5OTksLTEuMjE5OTk3IC0zLjA1OTk5OCwtMS44Mjk5OTggLTUuMzQwOTg4LC0xLjgyOTk5OGMtMi4xNTkwMTIsMCAtMy44NjkwMDMsMC42MjA5OTggLTUuMTI5MDEzLDEuODYwMDAxYy0xLjI1OTk5NSwxLjIzOTk5OCAtMi4wMzA5OTEsMi44OTk5OTggLTIuMzEwOTg5LDQuOTc5aDE0Ljc1OTk5NWwwLDAuMDAwOTk5elxcXCIvPiA8cGF0aCBkPVxcXCJtMjIyLjY3MTk5NywzNy43MDFoNi44Mzk5OTZ2NC4zMTloMC4xMjAwMWMxLjAzOTk5MywtMS43NTg5OTkgMi40Mzg5OTUsLTMuMDM5MDAxIDQuMTk5OTk3LC0zLjg0YzEuNzU5OTk1LC0wLjc5OTk5OSAzLjY2MDAwNCwtMS4xOTkwMDEgNS42OTkwMDUsLTEuMTk5MDAxYzIuMTk4OTksMCA0LjE3OTk5MywwLjM4OTk5OSA1LjkzOTk4NywxLjE3MDAwMmMxLjc2MDAxLDAuNzc4OTk5IDMuMjYwMDI1LDEuODUwOTk4IDQuNTAwMDE1LDMuMjA5OTk5YzEuMjM5MDE0LDEuMzYwMDAxIDIuMTc5OTkzLDIuOTU5OTk5IDIuODIwMDA3LDQuNzk5OTk5YzAuNjM5OTg0LDEuODQgMC45NTk5OTEsMy44MiAwLjk1OTk5MSw1LjkzODk5OWMwLDIuMTIxMDAyIC0wLjMzOTk5Niw0LjEwMTAwMiAtMS4wMTk5ODksNS45NDAwMDJjLTAuNjgyMDA3LDEuODQwMDA0IC0xLjYzMTAxMiwzLjQ0MDAwMiAtMi44NTEwMTMsNC44MDAwMDNjLTEuMjIxMDA4LDEuMzU5OTkzIC0yLjY5MDAwMiwyLjQzIC00LjQxMDAwNCwzLjIwOTk5OXMtMy42MDA5OTgsMS4xNjk5OTggLTUuNjM5OTk5LDEuMTY5OTk4Yy0xLjM2MDAwMSwwIC0yLjU2MTAwNSwtMC4xNDA5OTkgLTMuNjAwMDA2LC0wLjQyMDAwNmMtMS4wNDEsLTAuMjc5OTkxIC0xLjk2MDk5OSwtMC42Mzk5OTIgLTIuNzYxMDAyLC0xLjA3OTk5NGMtMC43OTk5ODgsLTAuNDM5MDAzIC0xLjQ3ODk4OSwtMC45MDkwMDQgLTIuMDM5OTkzLC0xLjQxMDAwNGMtMC41NjEwMDUsLTAuNDk5MDAxIC0xLjAyMDAwNCwtMC45ODg5OTggLTEuMzgwMDA1LC0xLjQ2OTk5NGgtMC4xODF2MTcuMzM5OTk2aC03LjE5ODk5di00Mi40NzlsMC4wMDI5OTEsMHptMjMuODgwMDA1LDE0LjQwMDAwMmMwLC0xLjExOTAwMyAtMC4xOTAwMDIsLTIuMTk5MDAxIC0wLjU2OSwtMy4yMzkwMDJjLTAuMzgwOTk3LC0xLjA0MDAwMSAtMC45NDA5OTQsLTEuOTU5OTk5IC0xLjY4MSwtMi43NjA5OThjLTAuNzQwOTk3LC0wLjc5OTAwNCAtMS42MzAwMDUsLTEuNDM5MDAzIC0yLjY2OTk5OCwtMS45MjAwMDJjLTEuMDQwMDA5LC0wLjQ3OSAtMi4yMjAwMDEsLTAuNzIwMDAxIC0zLjU0MDAwOSwtMC43MjAwMDFzLTIuNSwwLjI0MDAwMiAtMy41Mzk5OTMsMC43MjAwMDFjLTEuMDQwMDA5LDAuNDggLTEuOTMxLDEuMTIwOTk4IC0yLjY2OTk5OCwxLjkyMDAwMmMtMC43NDA5OTcsMC44MDA5OTkgLTEuMzAwMDAzLDEuNzIwOTk3IC0xLjY4MSwyLjc2MDk5OGMtMC4zODAwMDUsMS4wNDAwMDEgLTAuNTY5LDIuMTE5OTk5IC0wLjU2OSwzLjIzOTAwMmMwLDEuMTIwOTk4IDAuMTg4OTk1LDIuMjAwOTk2IDAuNTY5LDMuMjM5OTk4YzAuMzgwOTk3LDEuMDQxIDAuOTM4OTk1LDEuOTYwOTk1IDEuNjgxLDIuNzU5OTk4YzAuNzM4OTk4LDAuODAxMDAzIDEuNjI5OTksMS40NDAwMDIgMi42Njk5OTgsMS45MTk5OThjMS4wMzk5OTMsMC40ODAwMDMgMi4yMjAwMDEsMC43MjEwMDEgMy41Mzk5OTMsMC43MjEwMDFzMi41LC0wLjIzOTk5OCAzLjU0MDAwOSwtMC43MjEwMDFjMS4wMzk5OTMsLTAuNDc4OTk2IDEuOTI5MDAxLC0xLjExODk5NiAyLjY2OTk5OCwtMS45MTk5OThjMC43Mzg5OTgsLTAuNzk5MDA0IDEuMzAwMDAzLC0xLjcxODk5OCAxLjY4MSwtMi43NTk5OThjMC4zNzc5OTEsLTEuMDM5MDAxIDAuNTY5LC0yLjExODk5OSAwLjU2OSwtMy4yMzk5OTh6XFxcIi8+IDxwYXRoIGQ9XFxcIm0yNTkuMDMxMDA2LDUyLjEwMTAwMmMwLC0yLjI3OTAwMyAwLjQxMDAwNCwtNC4zNTAwMDIgMS4yMzAwMTEsLTYuMjEwMDAzYzAuODE3OTkzLC0xLjg1ODk5NyAxLjkyODk4NiwtMy40NDg5OTcgMy4zMjk5ODcsLTQuNzdjMS4zOTgwMSwtMS4zMiAzLjA1OTAyMSwtMi4zNCA0Ljk3OTAwNCwtMy4wNjA5OTdjMS45MjAwMTMsLTAuNzIwMDAxIDMuOTU5OTkxLC0xLjA3OTAwMiA2LjExOTk5NSwtMS4wNzkwMDJzNC4xOTkwMDUsMC4zNTkwMDEgNi4xMTkwMTksMS4wNzkwMDJjMS45MTk5ODMsMC43MjA5OTcgMy41Nzk5ODcsMS43Mzk5OTggNC45Nzk5OCwzLjA2MDk5N3MyLjUxMDAxLDIuOTEgMy4zMzAwMTcsNC43N2MwLjgxOTk3NywxLjg2MDAwMSAxLjIyOTk4LDMuOTMxIDEuMjI5OTgsNi4yMTAwMDNjMCwyLjI3OTk5OSAtMC40MTAwMDQsNC4zNTA5OTggLTEuMjI5OTgsNi4yMTAwMDNjLTAuODIwMDA3LDEuODYwMDAxIC0xLjkzMDAyMywzLjQ0OTk5NyAtMy4zMzAwMTcsNC43NzA5OTZzLTMuMDYxMDA1LDIuMzQwMDA0IC00Ljk3OTk4LDMuMDU5OTk4Yy0xLjkyMDAxMywwLjcyMTAwMSAtMy45NTkwMTUsMS4wODAwMDIgLTYuMTE5MDE5LDEuMDgwMDAycy00LjE5OTk4MiwtMC4zNTkwMDEgLTYuMTE5OTk1LC0xLjA4MDAwMmMtMS45MjA5OSwtMC43MTk5OTQgLTMuNTgwOTk0LC0xLjczODk5OCAtNC45NzkwMDQsLTMuMDU5OTk4Yy0xLjQwMTAwMSwtMS4zMiAtMi41MTE5OTMsLTIuOTA5OTk2IC0zLjMyOTk4NywtNC43NzA5OTZjLTAuODIwMDA3LC0xLjg2MDAwNCAtMS4yMzAwMTEsLTMuOTMwMDA0IC0xLjIzMDAxMSwtNi4yMTAwMDN6bTcuMTk5MDA1LDBjMCwxLjEyMDk5OCAwLjE4ODk5NSwyLjIwMDk5NiAwLjU3MDAwNywzLjIzOTk5OGMwLjM4MDAwNSwxLjA0MSAwLjkzODk5NSwxLjk2MDk5NSAxLjY3OTk5MywyLjc1OTk5OGMwLjczOTk5LDAuODAxMDAzIDEuNjMwMDA1LDEuNDQwMDAyIDIuNjcwMDEzLDEuOTE5OTk4YzEuMDQwOTg1LDAuNDgwMDAzIDIuMjIwOTc4LDAuNzIxMDAxIDMuNTQwOTg1LDAuNzIxMDAxczIuNDk4OTkzLC0wLjIzOTk5OCAzLjUzOTAwMSwtMC43MjEwMDFjMS4wNDA5ODUsLTAuNDc4OTk2IDEuOTI5OTkzLC0xLjExODk5NiAyLjY3MDAxMywtMS45MTk5OThjMC43Mzk5OSwtMC43OTkwMDQgMS4zMDA5OTUsLTEuNzE4OTk4IDEuNjgxOTc2LC0yLjc1OTk5OGMwLjM3ODk5OCwtMS4wMzkwMDEgMC41NjgwMjQsLTIuMTE4OTk5IDAuNTY4MDI0LC0zLjIzOTk5OGMwLC0xLjExOTAwMyAtMC4xODkwMjYsLTIuMTk5MDAxIC0wLjU2ODAyNCwtMy4yMzkwMDJjLTAuMzgwOTgxLC0xLjA0MDAwMSAtMC45NDA5NzksLTEuOTU5OTk5IC0xLjY4MTk3NiwtMi43NjA5OThjLTAuNzQwMDIxLC0wLjc5OTAwNCAtMS42MjkwMjgsLTEuNDM5MDAzIC0yLjY3MDAxMywtMS45MjAwMDJjLTEuMDQwMDA5LC0wLjQ3OSAtMi4yMTg5OTQsLTAuNzIwMDAxIC0zLjUzOTAwMSwtMC43MjAwMDFzLTIuNSwwLjI0MDAwMiAtMy41NDA5ODUsMC43MjAwMDFjLTEuMDQwMDA5LDAuNDggLTEuOTMwMDIzLDEuMTIwOTk4IC0yLjY3MDAxMywxLjkyMDAwMmMtMC43Mzk5OSwwLjgwMDk5OSAtMS4yOTk5ODgsMS43MjA5OTcgLTEuNjc5OTkzLDIuNzYwOTk4Yy0wLjM4MDAwNSwxLjAzOTAwMSAtMC41NzAwMDcsMi4xMTg5OTkgLTAuNTcwMDA3LDMuMjM5MDAyelxcXCIvPiA8cGF0aCBkPVxcXCJtMjk3LjA3MDAwNywzNy43MDFoNy4yMDA5ODl2NC41NjAwMDFoMC4xMTkwMTljMC43OTg5ODEsLTEuNjggMS45Mzg5OTUsLTIuOTc5IDMuNDE5OTgzLC0zLjg5OTAwMnMzLjE3OTk5MywtMS4zODAwMDEgNS4xMDAwMDYsLTEuMzgwMDAxYzAuNDM4OTk1LDAgMC44NzEwMDIsMC4wNDAwMDEgMS4yOTA5ODUsMC4xMTkwMDNjMC40MjAwMTMsMC4wODA5OTcgMC44NTAwMDYsMC4xODEgMS4yODkwMDEsMC4zMDA5OTl2Ni45NTk5OTljLTAuNTk5OTc2LC0wLjE2IC0xLjE4ODk5NSwtMC4yOTAwMDEgLTEuNzY5OTg5LC0wLjM5MDk5OWMtMC41Nzk5ODcsLTAuMDk4OTk5IC0xLjE0OTk5NCwtMC4xNDkwMDIgLTEuNzEwOTk5LC0wLjE0OTAwMmMtMS42Nzk5OTMsMCAtMy4wMjg5OTIsMC4zMTAwMDEgLTQuMDQ5MDExLDAuOTNjLTEuMDE5OTg5LDAuNjIxMDAyIC0xLjgwMDk5NSwxLjMzMDAwMiAtMi4zMzk5OTYsMi4xMzAwMDFjLTAuNTQwOTg1LDAuODAwOTk5IC0wLjg5OTk5NCwxLjYwMTAwMiAtMS4wNzk5ODcsMi40MDAwMDJjLTAuMTgwMDIzLDAuODAwOTk5IC0wLjI3MDAyLDEuMzk5OTk4IC0wLjI3MDAyLDEuNzk5OTk5djE1LjQxOTk5OGgtNy4yMDA5ODl2LTI4LjgwMDk5OWwwLjAwMTAwNywwelxcXCIvPiA8cGF0aCBkPVxcXCJtMzE3LjA0OTAxMSw0My44MjA5OTl2LTYuMTE5OTk5aDUuOTQwOTc5di04LjM0aDcuMTk5MDA1djguMzRoNy45MjAwMTN2Ni4xMTk5OTloLTcuOTIwMDEzdjEyLjYwMDAwMmMwLDEuNDM5OTk5IDAuMjcwMDIsMi41Nzk5OTggMC44MTEwMDUsMy40MjAwMDJjMC41MzkwMDEsMC44Mzk5OTYgMS42MDkwMDksMS4yNTk5OTUgMy4yMDkwMTUsMS4yNTk5OTVjMC42NDA5OTEsMCAxLjMzOTk5NiwtMC4wNjkgMi4xMDE5OSwtMC4yMDk5OTljMC43NTc5OTYsLTAuMTM5OTk5IDEuMzU5MDA5LC0wLjM2OTAwMyAxLjc5ODk4MSwtMC42ODkwMDN2Ni4wNjAwMDVjLTAuNzU5OTc5LDAuMzYwMDAxIC0xLjY4ODk5NSwwLjYwODk5NCAtMi43ODg5NzEsMC43NWMtMS4xMDIwMiwwLjEzOTk5OSAtMi4wNzAwMDcsMC4yMDk5OTkgLTIuOTEwMDA0LDAuMjA5OTk5Yy0xLjkyMDAxMywwIC0zLjQ5MDAyMSwtMC4yMDk5OTkgLTQuNzEwOTk5LC0wLjYzMDAwNXMtMi4xODAwMjMsLTEuMDU5OTk4IC0yLjg3ODk5OCwtMS45MTk5OThjLTAuNzAxMDE5LC0wLjg1OTAwMSAtMS4xODIwMDcsLTEuOTMgLTEuNDQxMDEsLTMuMjA5OTkxYy0wLjI2MDAxLC0xLjI3OTAwNyAtMC4zODkwMDgsLTIuNzYwMDEgLTAuMzg5MDA4LC00LjQ0MDAwMnYtMTMuMjAxMDA0aC01Ljk0MTk4NmwwLDB6XFxcIi8+IDwvZz4gPGc+IDxwYXRoIGQ9XFxcIm0xMTkuMTk0LDg2LjI5NTk5OGgzLjU4Nzk5N2MwLjM0NjAwMSwwIDAuNjg5MDAzLDAuMDQxIDEuMDI3LDAuMTI0MDAxYzAuMzM4MDA1LDAuMDgyMDAxIDAuNjM5LDAuMjE3MDAzIDAuOTAzLDAuNDAyYzAuMjY0LDAuMTg3MDA0IDAuNDc5MDA0LDAuNDI3MDAyIDAuNjQ0MDA1LDAuNzIyczAuMjQ2OTk0LDAuNjUwMDAyIDAuMjQ2OTk0LDEuMDY2MDAyYzAsMC41MTk5OTcgLTAuMTQ2OTk2LDAuOTQ3OTk4IC0wLjQ0MTk5NCwxLjI4NzAwM2MtMC4yOTUwMDYsMC4zMzc5OTcgLTAuNjgxLDAuNTc5OTk0IC0xLjE1NzAwNSwwLjcyNzk5N3YwLjAyNjAwMWMwLjI4NjAwMywwLjAzMzk5NyAwLjU1MzAwMSwwLjExMzk5OCAwLjgwMDAwMywwLjIzOTk5OGMwLjI0NzAwMiwwLjEyNTk5OSAwLjQ1NzAwMSwwLjI4NjAwMyAwLjYyOTk5NywwLjQ4MDAwM2MwLjE3MzAwNCwwLjE5NSAwLjMxMDAwNSwwLjQyMDk5OCAwLjQwOTAwNCwwLjY3Njk5NHMwLjE0OTk5NCwwLjUzMDAwNiAwLjE0OTk5NCwwLjgyNTAwNWMwLDAuNTAyOTk4IC0wLjA5OTk5OCwwLjkyMDk5OCAtMC4yOTg5OTYsMS4yNTQ5OTdjLTAuMTk4OTk3LDAuMzMzIC0wLjQ2MDk5OSwwLjYwMzAwNCAtMC43ODYwMDMsMC44MDZjLTAuMzI0OTk3LDAuMjA0MDAyIC0wLjY5Nzk5OCwwLjM0ODk5OSAtMS4xMTc5OTYsMC40MzYwMDVzLTAuODQ4LDAuMTI5OTk3IC0xLjI4MDk5OCwwLjEyOTk5N2gtMy4zMTUwMDJ2LTkuMjA0MDAybDAsMHptMS42MzgsMy43NDQwMDNoMS40OTUwMDNjMC41NDU5OTgsMCAwLjk1NTk5NCwtMC4xMDYwMDMgMS4yMjg5OTYsLTAuMzE4MDAxYzAuMjczMDAzLC0wLjIxMjk5NyAwLjQwODk5NywtMC40OTE5OTcgMC40MDg5OTcsLTAuODM4OTk3YzAsLTAuMzk4MDAzIC0wLjE0MDk5OSwtMC42OTUgLTAuNDIxOTk3LC0wLjg5MTAwNmMtMC4yODE5OTgsLTAuMTk0IC0wLjczNDAwMSwtMC4yOTIgLTEuMzU4MDAyLC0wLjI5MmgtMS4zNTE5OTd2Mi4zNDAwMDRsLTAuMDAwOTk5LDB6bTAsNC4wNTZoMS41MDc5OTZjMC4yMDgsMCAwLjQzMTAwNywtMC4wMTMgMC42NjkwMDYsLTAuMDM5MDAxYzAuMjM3OTk5LC0wLjAyNTAwMiAwLjQ1NzAwMSwtMC4wODU5OTkgMC42NTY5OTgsLTAuMTgxOTk5YzAuMTk4OTk3LC0wLjA5NjAwMSAwLjM2Mzk5OCwtMC4yMzEwMDMgMC40OTQwMDMsLTAuNDA4OTk3YzAuMTI5OTk3LC0wLjE3ODAwMSAwLjE5NSwtMC40MTgwMDcgMC4xOTUsLTAuNzIyYzAsLTAuNDg1MDAxIC0wLjE1ODAwNSwtMC44MjMwMDYgLTAuNDc1MDA2LC0xLjAxNGMtMC4zMTU5OTQsLTAuMTkxMDAyIC0wLjgwNzk5OSwtMC4yODYwMDMgLTEuNDc1OTk4LC0wLjI4NjAwM2gtMS41NzI5OTh2Mi42NTJsMC4wMDA5OTksMHpcXFwiLz4gPHBhdGggZD1cXFwibTEzMC44NTQ5OTYsOTEuNTYwOTk3bC0zLjQ1Nzk5MywtNS4yNjQ5OTloMi4wNTQwMDFsMi4yNjE5OTMsMy42NjZsMi4yODgwMSwtMy42NjZoMS45NDk5OTdsLTMuNDU4MDA4LDUuMjY0OTk5djMuOTM5MDAzaC0xLjYzOHYtMy45MzkwMDNsMCwwelxcXCIvPiA8cGF0aCBkPVxcXCJtMTUwLjc5Njk5Nyw5NC44MjM5OTdjLTEuMTM2MDAyLDAuNjA2MDAzIC0yLjQwNDk5OSwwLjkxMDAwNCAtMy44MDg5OSwwLjkxMDAwNGMtMC43MTEwMTQsMCAtMS4zNjMwMDcsLTAuMTE0OTk4IC0xLjk1NzAwMSwtMC4zNDUwMDFzLTEuMTA1MDExLC0wLjU1NSAtMS41MzQwMTIsLTAuOTc1OTk4Yy0wLjQyOTAwMSwtMC40MjAwMDYgLTAuNzY0OTk5LC0wLjkyNTAwMyAtMS4wMDY5ODksLTEuNTE0Yy0wLjI0MzAxMSwtMC41OTAwMDQgLTAuMzYzOTk4LC0xLjI0NDAwMyAtMC4zNjM5OTgsLTEuOTY0MDA1YzAsLTAuNzM2IDAuMTIwOTg3LC0xLjQwNDk5OSAwLjM2Mzk5OCwtMi4wMDc5OTZzMC41Nzg5OTUsLTEuMTE2MDA1IDEuMDA2OTg5LC0xLjU0MWMwLjQyOTAwMSwtMC40MjQwMDQgMC45NDAwMDIsLTAuNzUwOTk5IDEuNTM0MDEyLC0wLjk4MTAwM2MwLjU5Mzk5NCwtMC4yMjg5OTYgMS4yNDU5ODcsLTAuMzQ1MDAxIDEuOTU3MDAxLC0wLjM0NTAwMWMwLjcwMTk5NiwwIDEuMzYwMDAxLDAuMDg0OTk5IDEuOTc1OTk4LDAuMjU0MDA1YzAuNjE0OTksMC4xNjg5OTkgMS4xNjYsMC40NzEwMDEgMS42NTEwMDEsMC45MDNsLTEuMjA5LDEuMjIzYy0wLjI5NTAxMywtMC4yODYwMDMgLTAuNjUyMDA4LC0wLjUwODAwMyAtMS4wNzIwMDYsLTAuNjYzMDAyYy0wLjQyMTAwNSwtMC4xNTU5OTggLTAuODY1MDA1LC0wLjIzNDAwMSAtMS4zMzI5OTMsLTAuMjM0MDAxYy0wLjQ3NzAwNSwwIC0wLjkwODAwNSwwLjA4NDk5OSAtMS4yOTQwMDYsMC4yNTM5OThjLTAuMzg0OTk1LDAuMTY5MDA2IC0wLjcxNjk5NSwwLjQwMiAtMC45OTQwMDMsMC43MDEwMDRjLTAuMjc2OTkzLDAuMjk5OTk1IC0wLjQ5MjAwNCwwLjY0ODAwMyAtMC42NDM5OTcsMS4wNDY5OTdjLTAuMTUxOTkzLDAuMzk4MDAzIC0wLjIyNzk5NywwLjgyODAwMyAtMC4yMjc5OTcsMS4yODcwMDNjMCwwLjQ5Mzk5NiAwLjA3NjAwNCwwLjk0ODk5NyAwLjIyNzk5NywxLjM2NDk5OGMwLjE1MTAwMSwwLjQxNiAwLjM2NTk5NywwLjc3NTAwMiAwLjY0Mzk5NywxLjA3OTAwMmMwLjI3NzAwOCwwLjMwMzAwMSAwLjYwOTAwOSwwLjU0MSAwLjk5NDAwMywwLjcxNDk5NmMwLjM4NjAwMiwwLjE3MzAwNCAwLjgxNzAwMSwwLjI2MDAwMiAxLjI5NDAwNiwwLjI2MDAwMmMwLjQxNiwwIDAuODA3OTk5LC0wLjAzOTAwMSAxLjE3NTk5NSwtMC4xMTY5OTdjMC4zNjc5OTYsLTAuMDc4MDAzIDAuNjk0OTkyLC0wLjE5OTAwNSAwLjk4MTAwMywtMC4zNjI5OTl2LTIuMTcxMDA1aC0xLjg4NTAxdi0xLjQ4MDk5NWgzLjUyMzAxdjQuNzA0OTk0bDAuMDAwOTkyLDB6XFxcIi8+IDxwYXRoIGQ9XFxcIm0xNTMuNzIyLDg2LjI5NTk5OGgzLjE5Nzk5OGMwLjQ0MjAwMSwwIDAuODY5MDAzLDAuMDQxIDEuMjc5OTk5LDAuMTI0MDAxYzAuNDEyMDAzLDAuMDgyMDAxIDAuNzc4LDAuMjIzIDEuMDk4OTk5LDAuNDIyMDA1YzAuMzIwMDA3LDAuMTk4OTk3IDAuNTc2MDA0LDAuNDY3OTk1IDAuNzY2OTk4LDAuODA2OTk5YzAuMTkwMDAyLDAuMzM3OTk3IDAuMjg2MDExLDAuNzY2OTk4IDAuMjg2MDExLDEuMjg1OTk1YzAsMC42Njc5OTkgLTAuMTg0OTk4LDEuMjI3MDA1IC0wLjU1MzAwOSwxLjY3ODAwMWMtMC4zNjkwMDMsMC40NTAwMDUgLTAuODk0OTg5LDAuNzIzOTk5IC0xLjU4MDAwMiwwLjgxODAwMWwyLjQ0NTAwNyw0LjA2OWgtMS45NzU5OThsLTIuMTMyMDA0LC0zLjkwMDAwMmgtMS4xOTU5OTl2My45MDAwMDJoLTEuNjM4di05LjIwNDAwMmwwLDB6bTIuOTEyMDAzLDMuOTAwMDAyYzAuMjMzOTk0LDAgMC40NjgwMDIsLTAuMDExMDAyIDAuNzAxOTk2LC0wLjAzMjk5N2MwLjIzNDAwOSwtMC4wMjEwMDQgMC40NDc5OTgsLTAuMDczMDA2IDAuNjQzOTk3LC0wLjE1NDk5OWMwLjE5NTAwNywtMC4wODMgMC4zNTI5OTcsLTAuMjA4IDAuNDczOTk5LC0wLjM3NzAwN2MwLjEyMjAwOSwtMC4xNjg5OTkgMC4xODIwMDcsLTAuNDA0OTk5IDAuMTgyMDA3LC0wLjcwOWMwLC0wLjI2ODk5NyAtMC4wNTYsLTAuNDg1MDAxIC0wLjE2OTAwNiwtMC42NDg5OTRjLTAuMTEyOTkxLC0wLjE2NTAwMSAtMC4yNTk5OTUsLTAuMjg4MDAyIC0wLjQ0MjAwMSwtMC4zNzEwMDJjLTAuMTgxOTkyLC0wLjA4MjAwMSAtMC4zODM5ODcsLTAuMTM3MDAxIC0wLjYwMzk4OSwtMC4xNjIwMDNjLTAuMjIxMDA4LC0wLjAyNjAwMSAtMC40MzYwMDUsLTAuMDM5MDAxIC0wLjY0NDAxMiwtMC4wMzkwMDFoLTEuNDE2OTkydjIuNDk2MDAyaDEuMjc0MDAybDAsLTAuMDAwOTk5elxcXCIvPiA8cGF0aCBkPVxcXCJtMTY1Ljg3NjAwNyw4Ni4yOTU5OThoMS40MTY5OTJsMy45NjYwMDMsOS4yMDQwMDJoLTEuODcyMDA5bC0wLjg1Nzk4NiwtMi4xMDYwMDNoLTMuOTkxMDEzbC0wLjgzMjAwMSwyLjEwNjAwM2gtMS44MzI5OTNsNC4wMDMwMDYsLTkuMjA0MDAyem0yLjA4MDk5NCw1LjY5NGwtMS40MTcwMDcsLTMuNzQzOTk2bC0xLjQ0Mjk5MywzLjc0Mzk5NmgyLjg2MDAwMWwwLDB6XFxcIi8+IDxwYXRoIGQ9XFxcIm0xNzEuNDAxMDAxLDg2LjI5NTk5OGgxLjg4NDk5NWwyLjUwOTAwMyw2Ljk1NTAwMmwyLjU4NzAwNiwtNi45NTUwMDJoMS43Njc5OWwtMy43MTY5OTUsOS4yMDQwMDJoLTEuNDE2OTkybC0zLjYxNTAwNSwtOS4yMDQwMDJ6XFxcIi8+IDxwYXRoIGQ9XFxcIm0xODIuMDg3MDA2LDg2LjI5NTk5OGgxLjYzOHY5LjIwNDAwMmgtMS42Mzh2LTkuMjA0MDAybDAsMHpcXFwiLz4gPHBhdGggZD1cXFwibTE4OC42MTMwMDcsODcuNzc4aC0yLjgyMDk5OXYtMS40ODIwMDJoNy4yNzk5OTl2MS40ODIwMDJoLTIuODIwOTk5djcuNzIyaC0xLjYzOHYtNy43MjJsMCwwelxcXCIvPiA8cGF0aCBkPVxcXCJtMTk2Ljk1OSw4Ni4yOTU5OThoMS40MTcwMDdsMy45NjU5ODgsOS4yMDQwMDJoLTEuODczMDAxbC0wLjg1Njk5NSwtMi4xMDYwMDNoLTMuOTkwOTk3bC0wLjgzMzAwOCwyLjEwNjAwM2gtMS44MzI5OTNsNC4wMDM5OTgsLTkuMjA0MDAyem0yLjA4MDAwMiw1LjY5NGwtMS40MTcwMDcsLTMuNzQzOTk2bC0xLjQ0MjAwMSwzLjc0Mzk5NmgyLjg1OTAwOWwwLDB6XFxcIi8+IDxwYXRoIGQ9XFxcIm0yMDUuMDQ0OTk4LDg3Ljc3OGgtMi44MTk5OTJ2LTEuNDgyMDAyaDcuMjc4OTkydjEuNDgyMDAyaC0yLjgxOTk5MnY3LjcyMmgtMS42MzkwMDh2LTcuNzIybDAsMHpcXFwiLz4gPHBhdGggZD1cXFwibTIxMS41NzAwMDcsODYuMjk1OTk4aDEuNjM4OTkydjkuMjA0MDAyaC0xLjYzODk5MnYtOS4yMDQwMDJsMCwwelxcXCIvPiA8cGF0aCBkPVxcXCJtMjE1LjcxODk5NCw5MC45MzY5OTZjMCwtMC43MzYgMC4xMjEwMDIsLTEuNDA0OTk5IDAuMzYyOTkxLC0yLjAwNzk5NnMwLjU3ODAwMywtMS4xMTU5OTcgMS4wMDgwMTEsLTEuNTQxYzAuNDI5MDAxLC0wLjQyNDAwNCAwLjkzODk5NSwtMC43NTA5OTkgMS41MzI5OSwtMC45ODEwMDNjMC41OTQwMDksLTAuMjI4OTk2IDEuMjQ2MDAyLC0wLjM0NTAwMSAxLjk1NzAwMSwtMC4zNDUwMDFjMC43MTkwMDksLTAuMDA3OTk2IDEuMzc4MDA2LDAuMDk4MDA3IDEuOTc3MDA1LDAuMzE5YzAuNTk3OTkyLDAuMjIxMDAxIDEuMTEyOTkxLDAuNTQ0MDA2IDEuNTQ2OTk3LDAuOTY4MDAyYzAuNDMyOTk5LDAuNDI1MDAzIDAuNzcwOTk2LDAuOTM3MDA0IDEuMDE0MDA4LDEuNTM0MDA0YzAuMjQxOTg5LDAuNTk4OTk5IDAuMzYyOTkxLDEuMjY1OTk5IDAuMzYyOTkxLDIuMDAxOTk5YzAsMC43MjAwMDEgLTAuMTIxMDAyLDEuMzc0MDAxIC0wLjM2Mjk5MSwxLjk2Mjk5N2MtMC4yNDIwMDQsMC41OTAwMDQgLTAuNTgxMDA5LDEuMDk3IC0xLjAxNDAwOCwxLjUyMTAwNGMtMC40MzQwMDYsMC40MjQ5OTUgLTAuOTQ5MDA1LDAuNzU1OTk3IC0xLjU0Njk5NywwLjk5Mzk5NmMtMC41OTg5OTksMC4yMzc5OTkgLTEuMjU3OTk2LDAuMzYyIC0xLjk3NzAwNSwwLjM3MTAwMmMtMC43MTA5OTksMCAtMS4zNjI5OTEsLTAuMTE0OTk4IC0xLjk1NzAwMSwtMC4zNDUwMDFzLTEuMTAzOTg5LC0wLjU1NSAtMS41MzI5OSwtMC45NzU5OThjLTAuNDMwMDA4LC0wLjQyMDAwNiAtMC43NjYwMDYsLTAuOTI1MDAzIC0xLjAwODAxMSwtMS41MTRjLTAuMjQxOTg5LC0wLjU4ODAwNSAtMC4zNjI5OTEsLTEuMjQzMDA0IC0wLjM2Mjk5MSwtMS45NjIwMDZ6bTEuNzE1MDEyLC0wLjEwMzk5NmMwLDAuNDk0MDAzIDAuMDc2MDA0LDAuOTQ4OTk3IDAuMjI5MDA0LDEuMzY0OTk4YzAuMTQ5OTk0LDAuNDE2IDAuMzY1MDA1LDAuNzc1MDAyIDAuNjQzMDA1LDEuMDc5MDAyYzAuMjc2OTkzLDAuMzAzMDAxIDAuNjA4OTk0LDAuNTQxIDAuOTkzOTg4LDAuNzE0OTk2YzAuMzg3MDA5LDAuMTczMDA0IDAuODE3MDAxLDAuMjYwMDAyIDEuMjk1MDEzLDAuMjYwMDAyYzAuNDc2OTksMCAwLjkwODk5NywtMC4wODY5OTggMS4yOTg5OTYsLTAuMjYwMDAyYzAuMzkwOTkxLC0wLjE3Mzk5NiAwLjcyNDk5MSwtMC40MTE5OTUgMS4wMDE5OTksLTAuNzE0OTk2YzAuMjc2OTkzLC0wLjMwNDAwMSAwLjQ5MDk5NywtMC42NjMwMDIgMC42NDMwMDUsLTEuMDc5MDAyYzAuMTUxOTkzLC0wLjQxNiAwLjIyODk4OSwtMC44NzA5OTUgMC4yMjg5ODksLTEuMzY0OTk4YzAsLTAuNDU5IC0wLjA3NTk4OSwtMC44ODkgLTAuMjI4OTg5LC0xLjI4NzAwM2MtMC4xNTEwMDEsLTAuMzk3OTk1IC0wLjM2NTAwNSwtMC43NDY5OTQgLTAuNjQzMDA1LC0xLjA0Njk5N2MtMC4yNzcwMDgsLTAuMjk5MDA0IC0wLjYxMTAwOCwtMC41MzE5OTggLTEuMDAxOTk5LC0wLjcwMTAwNGMtMC4zODk5OTksLTAuMTY4OTk5IC0wLjgyMjAwNiwtMC4yNTM5OTggLTEuMjk4OTk2LC0wLjI1Mzk5OGMtMC40NzgwMTIsMCAtMC45MDgwMDUsMC4wODQ5OTkgLTEuMjk1MDEzLDAuMjUzOTk4Yy0wLjM4NDk5NSwwLjE2OTAwNiAtMC43MTY5OTUsMC40MDIgLTAuOTkzOTg4LDAuNzAxMDA0Yy0wLjI3NzAwOCwwLjMwMDAwMyAtMC40OTIwMDQsMC42NDgwMDMgLTAuNjQzMDA1LDEuMDQ2OTk3Yy0wLjE1MzAxNSwwLjM5ODAwMyAtMC4yMjkwMDQsMC44MjgwMDMgLTAuMjI5MDA0LDEuMjg3MDAzelxcXCIvPiA8cGF0aCBkPVxcXCJtMjI4LjAyOTAwNyw4Ni4yOTU5OThoMi4xNzA5OWw0LjQ1OSw2LjgzODAwNWgwLjAyNjAwMXYtNi44MzgwMDVoMS42MzcwMDl2OS4yMDQwMDJoLTIuMDc5MDFsLTQuNTUwMDAzLC03LjA1ODk5OGgtMC4wMjU5ODZ2Ny4wNTg5OThoLTEuNjM4di05LjIwNDAwMmwwLDB6XFxcIi8+IDxwYXRoIGQ9XFxcIm0yNDIuMzQxOTk1LDg2LjI5NTk5OGgxLjQxNzAwN2wzLjk2NjAwMyw5LjIwNDAwMmgtMS44NzMwMDFsLTAuODU3MDEsLTIuMTA2MDAzaC0zLjk5MDk5N2wtMC44MzI5OTMsMi4xMDYwMDNoLTEuODMzMDA4bDQuMDAzOTk4LC05LjIwNDAwMnptMi4wODAwMDIsNS42OTRsLTEuNDE2OTkyLC0zLjc0Mzk5NmwtMS40NDIwMDEsMy43NDM5OTZoMi44NTg5OTRsMCwwelxcXCIvPiA8cGF0aCBkPVxcXCJtMjQ5LjczODAwNyw4Ni4yOTU5OThoMS42Mzg5OTJ2Ny43MjJoMy45MTIwMDN2MS40ODIwMDJoLTUuNTUwOTk1di05LjIwNDAwMmwwLDB6XFxcIi8+IDwvZz4gPC9nPiA8L3N5bWJvbD5cIjtcbm1vZHVsZS5leHBvcnRzID0gc3ByaXRlLmFkZChpbWFnZSwgXCJncnYtdGxwdC1sb2dvLWZ1bGxcIik7XG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiAuL3NyYy9hc3NldHMvaW1nL3N2Zy9ncnYtdGxwdC1sb2dvLWZ1bGwuc3ZnXG4gKiogbW9kdWxlIGlkID0gNDM5XG4gKiogbW9kdWxlIGNodW5rcyA9IDFcbiAqKi8iLCJ2YXIgU3ByaXRlID0gcmVxdWlyZSgnLi9zcHJpdGUnKTtcbnZhciBnbG9iYWxTcHJpdGUgPSBuZXcgU3ByaXRlKCk7XG5cbmlmIChkb2N1bWVudC5ib2R5KSB7XG4gIGdsb2JhbFNwcml0ZS5lbGVtID0gZ2xvYmFsU3ByaXRlLnJlbmRlcihkb2N1bWVudC5ib2R5KTtcbn0gZWxzZSB7XG4gIGRvY3VtZW50LmFkZEV2ZW50TGlzdGVuZXIoJ0RPTUNvbnRlbnRMb2FkZWQnLCBmdW5jdGlvbiAoKSB7XG4gICAgZ2xvYmFsU3ByaXRlLmVsZW0gPSBnbG9iYWxTcHJpdGUucmVuZGVyKGRvY3VtZW50LmJvZHkpO1xuICB9LCBmYWxzZSk7XG59XG5cbm1vZHVsZS5leHBvcnRzID0gZ2xvYmFsU3ByaXRlO1xuXG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiAuL34vc3ZnLXNwcml0ZS1sb2FkZXIvbGliL3dlYi9nbG9iYWwtc3ByaXRlLmpzXG4gKiogbW9kdWxlIGlkID0gNDQwXG4gKiogbW9kdWxlIGNodW5rcyA9IDFcbiAqKi8iLCJ2YXIgU25pZmZyID0gcmVxdWlyZSgnc25pZmZyJyk7XG5cbi8qKlxuICogTGlzdCBvZiBTVkcgYXR0cmlidXRlcyB0byBmaXggdXJsIHRhcmdldCBpbiB0aGVtXG4gKiBAdHlwZSB7c3RyaW5nW119XG4gKi9cbnZhciBmaXhBdHRyaWJ1dGVzID0gW1xuICAnY2xpcFBhdGgnLFxuICAnY29sb3JQcm9maWxlJyxcbiAgJ3NyYycsXG4gICdjdXJzb3InLFxuICAnZmlsbCcsXG4gICdmaWx0ZXInLFxuICAnbWFya2VyJyxcbiAgJ21hcmtlclN0YXJ0JyxcbiAgJ21hcmtlck1pZCcsXG4gICdtYXJrZXJFbmQnLFxuICAnbWFzaycsXG4gICdzdHJva2UnXG5dO1xuXG4vKipcbiAqIFF1ZXJ5IHRvIGZpbmQnZW1cbiAqIEB0eXBlIHtzdHJpbmd9XG4gKi9cbnZhciBmaXhBdHRyaWJ1dGVzUXVlcnkgPSAnWycgKyBmaXhBdHRyaWJ1dGVzLmpvaW4oJ10sWycpICsgJ10nO1xuLyoqXG4gKiBAdHlwZSB7UmVnRXhwfVxuICovXG52YXIgVVJJX0ZVTkNfUkVHRVggPSAvXnVybFxcKCguKilcXCkkLztcblxuLyoqXG4gKiBDb252ZXJ0IGFycmF5LWxpa2UgdG8gYXJyYXlcbiAqIEBwYXJhbSB7T2JqZWN0fSBhcnJheUxpa2VcbiAqIEByZXR1cm5zIHtBcnJheS48Kj59XG4gKi9cbmZ1bmN0aW9uIGFycmF5RnJvbShhcnJheUxpa2UpIHtcbiAgcmV0dXJuIEFycmF5LnByb3RvdHlwZS5zbGljZS5jYWxsKGFycmF5TGlrZSwgMCk7XG59XG5cbi8qKlxuICogSGFuZGxlcyBmb3JiaWRkZW4gc3ltYm9scyB3aGljaCBjYW5ub3QgYmUgZGlyZWN0bHkgdXNlZCBpbnNpZGUgYXR0cmlidXRlcyB3aXRoIHVybCguLi4pIGNvbnRlbnQuXG4gKiBBZGRzIGxlYWRpbmcgc2xhc2ggZm9yIHRoZSBicmFja2V0c1xuICogQHBhcmFtIHtzdHJpbmd9IHVybFxuICogQHJldHVybiB7c3RyaW5nfSBlbmNvZGVkIHVybFxuICovXG5mdW5jdGlvbiBlbmNvZGVVcmxGb3JFbWJlZGRpbmcodXJsKSB7XG4gIHJldHVybiB1cmwucmVwbGFjZSgvXFwofFxcKS9nLCBcIlxcXFwkJlwiKTtcbn1cblxuLyoqXG4gKiBSZXBsYWNlcyBwcmVmaXggaW4gYHVybCgpYCBmdW5jdGlvbnNcbiAqIEBwYXJhbSB7RWxlbWVudH0gc3ZnXG4gKiBAcGFyYW0ge3N0cmluZ30gY3VycmVudFVybFByZWZpeFxuICogQHBhcmFtIHtzdHJpbmd9IG5ld1VybFByZWZpeFxuICovXG5mdW5jdGlvbiBiYXNlVXJsV29ya0Fyb3VuZChzdmcsIGN1cnJlbnRVcmxQcmVmaXgsIG5ld1VybFByZWZpeCkge1xuICB2YXIgbm9kZXMgPSBzdmcucXVlcnlTZWxlY3RvckFsbChmaXhBdHRyaWJ1dGVzUXVlcnkpO1xuXG4gIGlmICghbm9kZXMpIHtcbiAgICByZXR1cm47XG4gIH1cblxuICBhcnJheUZyb20obm9kZXMpLmZvckVhY2goZnVuY3Rpb24gKG5vZGUpIHtcbiAgICBpZiAoIW5vZGUuYXR0cmlidXRlcykge1xuICAgICAgcmV0dXJuO1xuICAgIH1cblxuICAgIGFycmF5RnJvbShub2RlLmF0dHJpYnV0ZXMpLmZvckVhY2goZnVuY3Rpb24gKGF0dHJpYnV0ZSkge1xuICAgICAgdmFyIGF0dHJpYnV0ZU5hbWUgPSBhdHRyaWJ1dGUubG9jYWxOYW1lLnRvTG93ZXJDYXNlKCk7XG5cbiAgICAgIGlmIChmaXhBdHRyaWJ1dGVzLmluZGV4T2YoYXR0cmlidXRlTmFtZSkgIT09IC0xKSB7XG4gICAgICAgIHZhciBtYXRjaCA9IFVSSV9GVU5DX1JFR0VYLmV4ZWMobm9kZS5nZXRBdHRyaWJ1dGUoYXR0cmlidXRlTmFtZSkpO1xuXG4gICAgICAgIC8vIERvIG5vdCB0b3VjaCB1cmxzIHdpdGggdW5leHBlY3RlZCBwcmVmaXhcbiAgICAgICAgaWYgKG1hdGNoICYmIG1hdGNoWzFdLmluZGV4T2YoY3VycmVudFVybFByZWZpeCkgPT09IDApIHtcbiAgICAgICAgICB2YXIgcmVmZXJlbmNlVXJsID0gZW5jb2RlVXJsRm9yRW1iZWRkaW5nKG5ld1VybFByZWZpeCArIG1hdGNoWzFdLnNwbGl0KGN1cnJlbnRVcmxQcmVmaXgpWzFdKTtcbiAgICAgICAgICBub2RlLnNldEF0dHJpYnV0ZShhdHRyaWJ1dGVOYW1lLCAndXJsKCcgKyByZWZlcmVuY2VVcmwgKyAnKScpO1xuICAgICAgICB9XG4gICAgICB9XG4gICAgfSk7XG4gIH0pO1xufVxuXG4vKipcbiAqIEJlY2F1c2Ugb2YgRmlyZWZveCBidWcgIzM1MzU3NSBncmFkaWVudHMgYW5kIHBhdHRlcm5zIGRvbid0IHdvcmsgaWYgdGhleSBhcmUgd2l0aGluIGEgc3ltYm9sLlxuICogVG8gd29ya2Fyb3VuZCB0aGlzIHdlIG1vdmUgdGhlIGdyYWRpZW50IGRlZmluaXRpb24gb3V0c2lkZSB0aGUgc3ltYm9sIGVsZW1lbnRcbiAqIEBzZWUgaHR0cHM6Ly9idWd6aWxsYS5tb3ppbGxhLm9yZy9zaG93X2J1Zy5jZ2k/aWQ9MzUzNTc1XG4gKiBAcGFyYW0ge0VsZW1lbnR9IHN2Z1xuICovXG52YXIgRmlyZWZveFN5bWJvbEJ1Z1dvcmthcm91bmQgPSBmdW5jdGlvbiAoc3ZnKSB7XG4gIHZhciBkZWZzID0gc3ZnLnF1ZXJ5U2VsZWN0b3IoJ2RlZnMnKTtcblxuICB2YXIgbW92ZVRvRGVmc0VsZW1zID0gc3ZnLnF1ZXJ5U2VsZWN0b3JBbGwoJ3N5bWJvbCBsaW5lYXJHcmFkaWVudCwgc3ltYm9sIHJhZGlhbEdyYWRpZW50LCBzeW1ib2wgcGF0dGVybicpO1xuICBmb3IgKHZhciBpID0gMCwgbGVuID0gbW92ZVRvRGVmc0VsZW1zLmxlbmd0aDsgaSA8IGxlbjsgaSsrKSB7XG4gICAgZGVmcy5hcHBlbmRDaGlsZChtb3ZlVG9EZWZzRWxlbXNbaV0pO1xuICB9XG59O1xuXG4vKipcbiAqIEB0eXBlIHtzdHJpbmd9XG4gKi9cbnZhciBERUZBVUxUX1VSSV9QUkVGSVggPSAnIyc7XG5cbi8qKlxuICogQHR5cGUge3N0cmluZ31cbiAqL1xudmFyIHhMaW5rSHJlZiA9ICd4bGluazpocmVmJztcbi8qKlxuICogQHR5cGUge3N0cmluZ31cbiAqL1xudmFyIHhMaW5rTlMgPSAnaHR0cDovL3d3dy53My5vcmcvMTk5OS94bGluayc7XG4vKipcbiAqIEB0eXBlIHtzdHJpbmd9XG4gKi9cbnZhciBzdmdPcGVuaW5nID0gJzxzdmcgeG1sbnM9XCJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2Z1wiIHhtbG5zOnhsaW5rPVwiJyArIHhMaW5rTlMgKyAnXCInO1xuLyoqXG4gKiBAdHlwZSB7c3RyaW5nfVxuICovXG52YXIgc3ZnQ2xvc2luZyA9ICc8L3N2Zz4nO1xuLyoqXG4gKiBAdHlwZSB7c3RyaW5nfVxuICovXG52YXIgY29udGVudFBsYWNlSG9sZGVyID0gJ3tjb250ZW50fSc7XG5cbi8qKlxuICogUmVwcmVzZW50YXRpb24gb2YgU1ZHIHNwcml0ZVxuICogQGNvbnN0cnVjdG9yXG4gKi9cbmZ1bmN0aW9uIFNwcml0ZSgpIHtcbiAgdmFyIGJhc2VFbGVtZW50ID0gZG9jdW1lbnQuZ2V0RWxlbWVudHNCeVRhZ05hbWUoJ2Jhc2UnKVswXTtcbiAgdmFyIGN1cnJlbnRVcmwgPSB3aW5kb3cubG9jYXRpb24uaHJlZi5zcGxpdCgnIycpWzBdO1xuICB2YXIgYmFzZVVybCA9IGJhc2VFbGVtZW50ICYmIGJhc2VFbGVtZW50LmhyZWY7XG4gIHRoaXMudXJsUHJlZml4ID0gYmFzZVVybCAmJiBiYXNlVXJsICE9PSBjdXJyZW50VXJsID8gY3VycmVudFVybCArIERFRkFVTFRfVVJJX1BSRUZJWCA6IERFRkFVTFRfVVJJX1BSRUZJWDtcblxuICB2YXIgc25pZmZyID0gbmV3IFNuaWZmcigpO1xuICBzbmlmZnIuc25pZmYoKTtcbiAgdGhpcy5icm93c2VyID0gc25pZmZyLmJyb3dzZXI7XG4gIHRoaXMuY29udGVudCA9IFtdO1xuXG4gIGlmICh0aGlzLmJyb3dzZXIubmFtZSAhPT0gJ2llJyAmJiBiYXNlVXJsKSB7XG4gICAgd2luZG93LmFkZEV2ZW50TGlzdGVuZXIoJ3Nwcml0ZUxvYWRlckxvY2F0aW9uVXBkYXRlZCcsIGZ1bmN0aW9uIChlKSB7XG4gICAgICB2YXIgY3VycmVudFByZWZpeCA9IHRoaXMudXJsUHJlZml4O1xuICAgICAgdmFyIG5ld1VybFByZWZpeCA9IGUuZGV0YWlsLm5ld1VybC5zcGxpdChERUZBVUxUX1VSSV9QUkVGSVgpWzBdICsgREVGQVVMVF9VUklfUFJFRklYO1xuICAgICAgYmFzZVVybFdvcmtBcm91bmQodGhpcy5zdmcsIGN1cnJlbnRQcmVmaXgsIG5ld1VybFByZWZpeCk7XG4gICAgICB0aGlzLnVybFByZWZpeCA9IG5ld1VybFByZWZpeDtcblxuICAgICAgaWYgKHRoaXMuYnJvd3Nlci5uYW1lID09PSAnZmlyZWZveCcgfHwgdGhpcy5icm93c2VyLm5hbWUgPT09ICdlZGdlJyB8fCB0aGlzLmJyb3dzZXIubmFtZSA9PT0gJ2Nocm9tZScgJiYgdGhpcy5icm93c2VyLnZlcnNpb25bMF0gPj0gNDkpIHtcbiAgICAgICAgdmFyIG5vZGVzID0gYXJyYXlGcm9tKGRvY3VtZW50LnF1ZXJ5U2VsZWN0b3JBbGwoJ3VzZVsqfGhyZWZdJykpO1xuICAgICAgICBub2Rlcy5mb3JFYWNoKGZ1bmN0aW9uIChub2RlKSB7XG4gICAgICAgICAgdmFyIGhyZWYgPSBub2RlLmdldEF0dHJpYnV0ZSh4TGlua0hyZWYpO1xuICAgICAgICAgIGlmIChocmVmICYmIGhyZWYuaW5kZXhPZihjdXJyZW50UHJlZml4KSA9PT0gMCkge1xuICAgICAgICAgICAgbm9kZS5zZXRBdHRyaWJ1dGVOUyh4TGlua05TLCB4TGlua0hyZWYsIG5ld1VybFByZWZpeCArIGhyZWYuc3BsaXQoREVGQVVMVF9VUklfUFJFRklYKVsxXSk7XG4gICAgICAgICAgfVxuICAgICAgICB9KTtcbiAgICAgIH1cbiAgICB9LmJpbmQodGhpcykpO1xuICB9XG59XG5cblNwcml0ZS5zdHlsZXMgPSBbJ3Bvc2l0aW9uOmFic29sdXRlJywgJ3dpZHRoOjAnLCAnaGVpZ2h0OjAnLCAndmlzaWJpbGl0eTpoaWRkZW4nXTtcblxuU3ByaXRlLnNwcml0ZVRlbXBsYXRlID0gc3ZnT3BlbmluZyArICcgc3R5bGU9XCInKyBTcHJpdGUuc3R5bGVzLmpvaW4oJzsnKSArJ1wiPjxkZWZzPicgKyBjb250ZW50UGxhY2VIb2xkZXIgKyAnPC9kZWZzPicgKyBzdmdDbG9zaW5nO1xuU3ByaXRlLnN5bWJvbFRlbXBsYXRlID0gc3ZnT3BlbmluZyArICc+JyArIGNvbnRlbnRQbGFjZUhvbGRlciArIHN2Z0Nsb3Npbmc7XG5cbi8qKlxuICogQHR5cGUge0FycmF5PFN0cmluZz59XG4gKi9cblNwcml0ZS5wcm90b3R5cGUuY29udGVudCA9IG51bGw7XG5cbi8qKlxuICogQHBhcmFtIHtTdHJpbmd9IGNvbnRlbnRcbiAqIEBwYXJhbSB7U3RyaW5nfSBpZFxuICovXG5TcHJpdGUucHJvdG90eXBlLmFkZCA9IGZ1bmN0aW9uIChjb250ZW50LCBpZCkge1xuICBpZiAodGhpcy5zdmcpIHtcbiAgICB0aGlzLmFwcGVuZFN5bWJvbChjb250ZW50KTtcbiAgfVxuXG4gIHRoaXMuY29udGVudC5wdXNoKGNvbnRlbnQpO1xuXG4gIHJldHVybiBERUZBVUxUX1VSSV9QUkVGSVggKyBpZDtcbn07XG5cbi8qKlxuICpcbiAqIEBwYXJhbSBjb250ZW50XG4gKiBAcGFyYW0gdGVtcGxhdGVcbiAqIEByZXR1cm5zIHtFbGVtZW50fVxuICovXG5TcHJpdGUucHJvdG90eXBlLndyYXBTVkcgPSBmdW5jdGlvbiAoY29udGVudCwgdGVtcGxhdGUpIHtcbiAgdmFyIHN2Z1N0cmluZyA9IHRlbXBsYXRlLnJlcGxhY2UoY29udGVudFBsYWNlSG9sZGVyLCBjb250ZW50KTtcblxuICB2YXIgc3ZnID0gbmV3IERPTVBhcnNlcigpLnBhcnNlRnJvbVN0cmluZyhzdmdTdHJpbmcsICdpbWFnZS9zdmcreG1sJykuZG9jdW1lbnRFbGVtZW50O1xuXG4gIGlmICh0aGlzLmJyb3dzZXIubmFtZSAhPT0gJ2llJyAmJiB0aGlzLnVybFByZWZpeCkge1xuICAgIGJhc2VVcmxXb3JrQXJvdW5kKHN2ZywgREVGQVVMVF9VUklfUFJFRklYLCB0aGlzLnVybFByZWZpeCk7XG4gIH1cblxuICByZXR1cm4gc3ZnO1xufTtcblxuU3ByaXRlLnByb3RvdHlwZS5hcHBlbmRTeW1ib2wgPSBmdW5jdGlvbiAoY29udGVudCkge1xuICB2YXIgc3ltYm9sID0gdGhpcy53cmFwU1ZHKGNvbnRlbnQsIFNwcml0ZS5zeW1ib2xUZW1wbGF0ZSkuY2hpbGROb2Rlc1swXTtcblxuICB0aGlzLnN2Zy5xdWVyeVNlbGVjdG9yKCdkZWZzJykuYXBwZW5kQ2hpbGQoc3ltYm9sKTtcbiAgaWYgKHRoaXMuYnJvd3Nlci5uYW1lID09PSAnZmlyZWZveCcpIHtcbiAgICBGaXJlZm94U3ltYm9sQnVnV29ya2Fyb3VuZCh0aGlzLnN2Zyk7XG4gIH1cbn07XG5cbi8qKlxuICogQHJldHVybnMge1N0cmluZ31cbiAqL1xuU3ByaXRlLnByb3RvdHlwZS50b1N0cmluZyA9IGZ1bmN0aW9uICgpIHtcbiAgdmFyIHdyYXBwZXIgPSBkb2N1bWVudC5jcmVhdGVFbGVtZW50KCdkaXYnKTtcbiAgd3JhcHBlci5hcHBlbmRDaGlsZCh0aGlzLnJlbmRlcigpKTtcbiAgcmV0dXJuIHdyYXBwZXIuaW5uZXJIVE1MO1xufTtcblxuLyoqXG4gKiBAcGFyYW0ge0hUTUxFbGVtZW50fSBbdGFyZ2V0XVxuICogQHBhcmFtIHtCb29sZWFufSBbcHJlcGVuZD10cnVlXVxuICogQHJldHVybnMge0hUTUxFbGVtZW50fSBSZW5kZXJlZCBzcHJpdGUgbm9kZVxuICovXG5TcHJpdGUucHJvdG90eXBlLnJlbmRlciA9IGZ1bmN0aW9uICh0YXJnZXQsIHByZXBlbmQpIHtcbiAgdGFyZ2V0ID0gdGFyZ2V0IHx8IG51bGw7XG4gIHByZXBlbmQgPSB0eXBlb2YgcHJlcGVuZCA9PT0gJ2Jvb2xlYW4nID8gcHJlcGVuZCA6IHRydWU7XG5cbiAgdmFyIHN2ZyA9IHRoaXMud3JhcFNWRyh0aGlzLmNvbnRlbnQuam9pbignJyksIFNwcml0ZS5zcHJpdGVUZW1wbGF0ZSk7XG5cbiAgaWYgKHRoaXMuYnJvd3Nlci5uYW1lID09PSAnZmlyZWZveCcpIHtcbiAgICBGaXJlZm94U3ltYm9sQnVnV29ya2Fyb3VuZChzdmcpO1xuICB9XG5cbiAgaWYgKHRhcmdldCkge1xuICAgIGlmIChwcmVwZW5kICYmIHRhcmdldC5jaGlsZE5vZGVzWzBdKSB7XG4gICAgICB0YXJnZXQuaW5zZXJ0QmVmb3JlKHN2ZywgdGFyZ2V0LmNoaWxkTm9kZXNbMF0pO1xuICAgIH0gZWxzZSB7XG4gICAgICB0YXJnZXQuYXBwZW5kQ2hpbGQoc3ZnKTtcbiAgICB9XG4gIH1cblxuICB0aGlzLnN2ZyA9IHN2ZztcblxuICByZXR1cm4gc3ZnO1xufTtcblxubW9kdWxlLmV4cG9ydHMgPSBTcHJpdGU7XG5cblxuXG4vKioqKioqKioqKioqKioqKipcbiAqKiBXRUJQQUNLIEZPT1RFUlxuICoqIC4vfi9zdmctc3ByaXRlLWxvYWRlci9saWIvd2ViL3Nwcml0ZS5qc1xuICoqIG1vZHVsZSBpZCA9IDQ0MVxuICoqIG1vZHVsZSBjaHVua3MgPSAxXG4gKiovIiwibW9kdWxlLmV4cG9ydHMgPSBUZXJtaW5hbDtcblxuXG4vKioqKioqKioqKioqKioqKipcbiAqKiBXRUJQQUNLIEZPT1RFUlxuICoqIGV4dGVybmFsIFwiVGVybWluYWxcIlxuICoqIG1vZHVsZSBpZCA9IDQ0M1xuICoqIG1vZHVsZSBjaHVua3MgPSAxXG4gKiovIl0sInNvdXJjZVJvb3QiOiIifQ==