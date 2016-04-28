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
	
	  TtyPlayer.prototype.connect = function connect() {
	    var _this = this;
	
	    this._setStatusFlag({ isLoading: true });
	
	    api.get('/v1/webapi/sites/-curent-/sessions/' + this.sid + '/events');
	
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
	
	function receiveSessions(state) {
	  var jsonArray = arguments.length <= 1 || arguments[1] === undefined ? [] : arguments[1];
	
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
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJzb3VyY2VzIjpbIndlYnBhY2s6Ly8vLi9zcmMvYXBwL3JlYWN0b3IuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb25maWcuanMiLCJ3ZWJwYWNrOi8vL2V4dGVybmFsIFwialF1ZXJ5XCIiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21tb24vbG9nZ2VyLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvc2VydmljZXMvYXBpLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvc2VydmljZXMvc2Vzc2lvbi5qcyIsIndlYnBhY2s6Ly8vZXh0ZXJuYWwgXCJfXCIiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2ljb25zLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvbXNnUGFnZS5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL3RhYmxlLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvbm90aWZpY2F0aW9ucy9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9jdXJyZW50U2Vzc2lvbi9nZXR0ZXJzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9zZXNzaW9ucy9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvYWN0aW9ucy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc3RvcmVkU2Vzc2lvbnNGaWx0ZXIvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvdXNlci9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvdXNlci9nZXR0ZXJzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvc2VydmljZXMvYXV0aC5qcyIsIndlYnBhY2s6Ly8vLi9+L2V2ZW50cy9ldmVudHMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21tb24vb2JqZWN0VXRpbHMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21tb24vdGVybWluYWwuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21tb24vdHR5LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9jdXJyZW50U2Vzc2lvbi9zZXNzaW9uTGVmdFBhbmVsLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvZ29vZ2xlQXV0aExvZ28uanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9pbnB1dFNlYXJjaC5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL25vZGVzL25vZGVMaXN0LmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvc2Vzc2lvbnMvbGlzdEl0ZW1zLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvYXBwL2FjdGlvblR5cGVzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hcHAvYXBwU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2N1cnJlbnRTZXNzaW9uL2FjdGlvblR5cGVzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9jdXJyZW50U2Vzc2lvbi9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9jdXJyZW50U2Vzc2lvbi9jdXJyZW50U2Vzc2lvblN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9jdXJyZW50U2Vzc2lvbi9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvZGlhbG9ncy9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvZGlhbG9ncy9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9kaWFsb2dzL2RpYWxvZ1N0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvbm90aWZpY2F0aW9ucy9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9hY3Rpb25UeXBlcy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9jb25zdGFudHMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3N0b3JlZFNlc3Npb25zRmlsdGVyL2FjdGlvblR5cGVzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy91c2VyL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvdXNlclN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvc2VydmljZXMvYXBpVXRpbHMuanMiLCJ3ZWJwYWNrOi8vLy4vfi9mYmpzL2xpYi9DU1NDb3JlLmpzIiwid2VicGFjazovLy8uL34vcmVhY3QtYWRkb25zLWNzcy10cmFuc2l0aW9uLWdyb3VwL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tbW9uL3BhdHRlcm5VdGlscy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbW1vbi90dHlFdmVudHMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21tb24vdHR5UGxheWVyLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9hcHAuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9jdXJyZW50U2Vzc2lvbi9hY3RpdmVTZXNzaW9uLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvY3VycmVudFNlc3Npb24vbWFpbi5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2N1cnJlbnRTZXNzaW9uL3Nlc3Npb25QbGF5ZXIuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9kYXRlUGlja2VyLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvaW5kZXguanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9sb2dpbi5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL25hdkxlZnRCYXIuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9uZXdVc2VyLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvbm9kZXMvbWFpbi5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL25vdGlmaWNhdGlvbkhvc3QuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9zZWxlY3ROb2RlRGlhbG9nLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvc2Vzc2lvbnMvYWN0aXZlU2Vzc2lvbkxpc3QuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9zZXNzaW9ucy9tYWluLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvc2Vzc2lvbnMvc3RvcmVkU2Vzc2lvbkxpc3QuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy90ZXJtaW5hbC5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9pbmRleC5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2FwcC9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9hcHAvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvYXBwL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9kaWFsb2dzL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2RpYWxvZ3MvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9ub2RlU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL25vdGlmaWNhdGlvbnMvZ2V0dGVycy5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvbm90aWZpY2F0aW9ucy9ub3RpZmljYXRpb25TdG9yZS5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvcmVzdEFwaVN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9zZXNzaW9ucy9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvc2Vzc2lvblN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9zdG9yZWRTZXNzaW9uc0ZpbHRlci9hY3Rpb25zLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9zdG9yZWRTZXNzaW9uc0ZpbHRlci9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvc3RvcmVkU2Vzc2lvbnNGaWx0ZXIvc3RvcmVkU2Vzc2lvbkZpbHRlclN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy91c2VyL3VzZXJJbnZpdGVTdG9yZS5qcyIsIndlYnBhY2s6Ly8vLi9+L3JlYWN0L2xpYi9SZWFjdENTU1RyYW5zaXRpb25Hcm91cC5qcyIsIndlYnBhY2s6Ly8vLi9+L3JlYWN0L2xpYi9SZWFjdENTU1RyYW5zaXRpb25Hcm91cENoaWxkLmpzIiwid2VicGFjazovLy8uL34vc25pZmZyL3NyYy9zbmlmZnIuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2Fzc2V0cy9pbWcvc3ZnL2dydi10bHB0LWxvZ28tZnVsbC5zdmciLCJ3ZWJwYWNrOi8vLy4vfi9zdmctc3ByaXRlLWxvYWRlci9saWIvd2ViL2dsb2JhbC1zcHJpdGUuanMiLCJ3ZWJwYWNrOi8vLy4vfi9zdmctc3ByaXRlLWxvYWRlci9saWIvd2ViL3Nwcml0ZS5qcyIsIndlYnBhY2s6Ly8vZXh0ZXJuYWwgXCJUZXJtaW5hbFwiIl0sIm5hbWVzIjpbXSwibWFwcGluZ3MiOiI7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7O3NDQWdCd0IsRUFBWTs7QUFFcEMsS0FBSSxPQUFPLEdBQUcsSUFBSSxDQUFDOzs7QUFHbkIsS0FBSSxLQUFLLEdBQUcsTUFBTSxDQUFDLFNBQVMsQ0FBQztBQUM3QixLQUFHLEtBQUssSUFBSSxLQUFLLENBQUMsTUFBTSxDQUFDLElBQUksQ0FBQyxNQUFNLEtBQUssQ0FBQyxFQUFDO0FBQ3pDLFVBQU8sR0FBRyxLQUFLLENBQUM7RUFDakI7O0FBRUQsS0FBTSxPQUFPLEdBQUcsdUJBQVk7QUFDMUIsUUFBSyxFQUFFLE9BQU87RUFDZixDQUFDOztBQUVGLE9BQU0sQ0FBQyxPQUFPLEdBQUcsT0FBTyxDQUFDOztzQkFFVixPQUFPOzs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7OztnQkNoQkEsbUJBQU8sQ0FBQyxHQUF5QixDQUFDOztLQUFuRCxhQUFhLFlBQWIsYUFBYTs7QUFDbEIsS0FBSSxDQUFDLEdBQUcsbUJBQU8sQ0FBQyxFQUFRLENBQUMsQ0FBQzs7QUFFMUIsS0FBSSxHQUFHLEdBQUc7O0FBRVIsVUFBTyxFQUFFLE1BQU0sQ0FBQyxRQUFRLENBQUMsTUFBTTs7QUFFL0IsVUFBTyxFQUFFLG9EQUFvRDs7QUFF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QTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7QUFDQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0Esa0JBQWlCO0FBQ2pCO0FBQ0E7QUFDQTtBQUNBOztBQUVBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQSxvQkFBbUIsU0FBUztBQUM1QjtBQUNBO0FBQ0E7QUFDQSxJQUFHO0FBQ0g7QUFDQTtBQUNBLGdCQUFlLFNBQVM7QUFDeEI7O0FBRUE7QUFDQTtBQUNBLGdCQUFlLFNBQVM7QUFDeEI7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBLE1BQUs7QUFDTDtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBOztBQUVBOztBQUVBO0FBQ0E7QUFDQTs7QUFFQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUEsSUFBRztBQUNILHFCQUFvQixTQUFTO0FBQzdCO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7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O0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBLE1BQUs7O0FBRUw7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0EsSUFBRzs7QUFFSDtBQUNBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7O0FBRUE7QUFDQTs7QUFFQTs7QUFFQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7O0FBRUE7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0EsTUFBSztBQUNMO0FBQ0E7QUFDQTtBQUNBLElBQUc7O0FBRUg7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7QUFDQSxJQUFHOztBQUVIO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBLElBQUc7O0FBRUg7QUFDQTtBQUNBO0FBQ0EsSUFBRzs7QUFFSDtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQSxNQUFLO0FBQ0wsSUFBRzs7QUFFSDtBQUNBO0FBQ0E7QUFDQSxNQUFLO0FBQ0w7QUFDQTtBQUNBLElBQUc7O0FBRUg7QUFDQTtBQUNBO0FBQ0EsTUFBSztBQUNMO0FBQ0E7QUFDQSxJQUFHOztBQUVIO0FBQ0E7QUFDQTtBQUNBLE1BQUs7QUFDTDtBQUNBO0FBQ0EsSUFBRzs7QUFFSDtBQUNBO0FBQ0E7QUFDQSxFQUFDOztBQUVELCtDOzs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7O0FDcEtBOztBQUVBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQSxnQ0FBK0I7QUFDL0I7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFD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QUVBOztBQUVBO0FBQ0E7O0FBRUE7Ozs7Ozs7O0FDeFBBLDJCIiwiZmlsZSI6ImFwcC5qcyIsInNvdXJjZXNDb250ZW50IjpbIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxuaW1wb3J0IHsgUmVhY3RvciB9IGZyb20gJ251Y2xlYXItanMnXG5cbmxldCBlbmFibGVkID0gdHJ1ZTtcblxuLy8gdGVtcG9yYXJ5IHdvcmthcm91bmQgdG8gZGlzYWJsZSBkZWJ1ZyBpbmZvIGR1cmluZyB1bml0LXRlc3RzXG5sZXQga2FybWEgPSB3aW5kb3cuX19rYXJtYV9fO1xuaWYoa2FybWEgJiYga2FybWEuY29uZmlnLmFyZ3MubGVuZ3RoID09PSAxKXtcbiAgZW5hYmxlZCA9IGZhbHNlO1xufVxuXG5jb25zdCByZWFjdG9yID0gbmV3IFJlYWN0b3Ioe1xuICBkZWJ1ZzogZW5hYmxlZFxufSlcblxud2luZG93LnJlYWN0b3IgPSByZWFjdG9yO1xuXG5leHBvcnQgZGVmYXVsdCByZWFjdG9yXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvcmVhY3Rvci5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxubGV0IHtmb3JtYXRQYXR0ZXJufSA9IHJlcXVpcmUoJ2FwcC9jb21tb24vcGF0dGVyblV0aWxzJyk7XG5sZXQgJCA9IHJlcXVpcmUoJ2pRdWVyeScpO1xuXG5sZXQgY2ZnID0ge1xuXG4gIGJhc2VVcmw6IHdpbmRvdy5sb2NhdGlvbi5vcmlnaW4sXG5cbiAgaGVscFVybDogJ2h0dHA6Ly9ncmF2aXRhdGlvbmFsLmNvbS90ZWxlcG9ydC9kb2NzL3F1aWNrc3RhcnQvJyxcblxuICBtYXhTZXNzaW9uTG9hZFNpemU6IDUwLFxuXG4gIGRpc3BsYXlEYXRlRm9ybWF0OiAnbCBMVFMgWicsXG5cbiAgYXV0aDoge1xuICAgIG9pZGNfY29ubmVjdG9yczogW11cbiAgfSxcblxuICByb3V0ZXM6IHtcbiAgICBhcHA6ICcvd2ViJyxcbiAgICBsb2dvdXQ6ICcvd2ViL2xvZ291dCcsXG4gICAgbG9naW46ICcvd2ViL2xvZ2luJyxcbiAgICBub2RlczogJy93ZWIvbm9kZXMnLFxuICAgIGFjdGl2ZVNlc3Npb246ICcvd2ViL3Nlc3Npb25zLzpzaWQnLFxuICAgIG5ld1VzZXI6ICcvd2ViL25ld3VzZXIvOmludml0ZVRva2VuJyxcbiAgICBzZXNzaW9uczogJy93ZWIvc2Vzc2lvbnMnLFxuICAgIG1zZ3M6ICcvd2ViL21zZy86dHlwZSgvOnN1YlR5cGUpJyxcbiAgICBwYWdlTm90Rm91bmQ6ICcvd2ViL25vdGZvdW5kJ1xuICB9LFxuXG4gIGFwaToge1xuICAgIHNzbzogJy92MS93ZWJhcGkvb2lkYy9sb2dpbi93ZWI/cmVkaXJlY3RfdXJsPTpyZWRpcmVjdCZjb25uZWN0b3JfaWQ9OnByb3ZpZGVyJyxcbiAgICByZW5ld1Rva2VuUGF0aDonL3YxL3dlYmFwaS9zZXNzaW9ucy9yZW5ldycsXG4gICAgbm9kZXNQYXRoOiAnL3YxL3dlYmFwaS9zaXRlcy8tY3VycmVudC0vbm9kZXMnLFxuICAgIHNlc3Npb25QYXRoOiAnL3YxL3dlYmFwaS9zZXNzaW9ucycsXG4gICAgc2l0ZVNlc3Npb25QYXRoOiAnL3YxL3dlYmFwaS9zaXRlcy8tY3VycmVudC0vc2Vzc2lvbnMnLFxuICAgIGludml0ZVBhdGg6ICcvdjEvd2ViYXBpL3VzZXJzL2ludml0ZXMvOmludml0ZVRva2VuJyxcbiAgICBjcmVhdGVVc2VyUGF0aDogJy92MS93ZWJhcGkvdXNlcnMnLFxuICAgIHNlc3Npb25DaHVuazogJy92MS93ZWJhcGkvc2l0ZXMvLWN1cnJlbnQtL3Nlc3Npb25zLzpzaWQvY2h1bmtzP3N0YXJ0PTpzdGFydCZlbmQ9OmVuZCcsXG4gICAgc2Vzc2lvbkNodW5rQ291bnRQYXRoOiAnL3YxL3dlYmFwaS9zaXRlcy8tY3VycmVudC0vc2Vzc2lvbnMvOnNpZC9jaHVua3Njb3VudCcsXG4gICAgc2l0ZUV2ZW50U2Vzc2lvbkZpbHRlclBhdGg6IGAvdjEvd2ViYXBpL3NpdGVzLy1jdXJyZW50LS9zZXNzaW9ucz9maWx0ZXI9OmZpbHRlcmAsXG5cbiAgICBnZXRTc29VcmwocmVkaXJlY3QsIHByb3ZpZGVyKXtcbiAgICAgIHJldHVybiBjZmcuYmFzZVVybCArIGZvcm1hdFBhdHRlcm4oY2ZnLmFwaS5zc28sIHtyZWRpcmVjdCwgcHJvdmlkZXJ9KTtcbiAgICB9LFxuXG4gICAgZ2V0RmV0Y2hTZXNzaW9uQ2h1bmtVcmwoe3NpZCwgc3RhcnQsIGVuZH0pe1xuICAgICAgcmV0dXJuIGZvcm1hdFBhdHRlcm4oY2ZnLmFwaS5zZXNzaW9uQ2h1bmssIHtzaWQsIHN0YXJ0LCBlbmR9KTtcbiAgICB9LFxuXG4gICAgZ2V0RmV0Y2hTZXNzaW9uTGVuZ3RoVXJsKHNpZCl7XG4gICAgICByZXR1cm4gZm9ybWF0UGF0dGVybihjZmcuYXBpLnNlc3Npb25DaHVua0NvdW50UGF0aCwge3NpZH0pO1xuICAgIH0sXG5cbiAgICBnZXRGZXRjaFNlc3Npb25zVXJsKGFyZ3Mpe1xuICAgICAgdmFyIGZpbHRlciA9IEpTT04uc3RyaW5naWZ5KGFyZ3MpO1xuICAgICAgcmV0dXJuIGZvcm1hdFBhdHRlcm4oY2ZnLmFwaS5zaXRlRXZlbnRTZXNzaW9uRmlsdGVyUGF0aCwge2ZpbHRlcn0pO1xuICAgIH0sXG5cbiAgICBnZXRGZXRjaFNlc3Npb25Vcmwoc2lkKXtcbiAgICAgIHJldHVybiBmb3JtYXRQYXR0ZXJuKGNmZy5hcGkuc2l0ZVNlc3Npb25QYXRoKycvOnNpZCcsIHtzaWR9KTtcbiAgICB9LFxuXG4gICAgZ2V0VGVybWluYWxTZXNzaW9uVXJsKHNpZCl7XG4gICAgICByZXR1cm4gZm9ybWF0UGF0dGVybihjZmcuYXBpLnNpdGVTZXNzaW9uUGF0aCsnLzpzaWQnLCB7c2lkfSk7XG4gICAgfSxcblxuICAgIGdldEludml0ZVVybChpbnZpdGVUb2tlbil7XG4gICAgICByZXR1cm4gZm9ybWF0UGF0dGVybihjZmcuYXBpLmludml0ZVBhdGgsIHtpbnZpdGVUb2tlbn0pO1xuICAgIH0sXG5cbiAgICBnZXRFdmVudFN0cmVhbUNvbm5TdHIoKXtcbiAgICAgIHZhciBob3N0bmFtZSA9IGdldFdzSG9zdE5hbWUoKTtcbiAgICAgIHJldHVybiBgJHtob3N0bmFtZX0vdjEvd2ViYXBpL3NpdGVzLy1jdXJyZW50LWA7XG4gICAgfSxcblxuICAgIGdldFR0eVVybCgpe1xuICAgICAgdmFyIGhvc3RuYW1lID0gZ2V0V3NIb3N0TmFtZSgpO1xuICAgICAgcmV0dXJuIGAke2hvc3RuYW1lfS92MS93ZWJhcGkvc2l0ZXMvLWN1cnJlbnQtYDtcbiAgICB9XG5cblxuICB9LFxuXG4gIGdldEZ1bGxVcmwodXJsKXtcbiAgICByZXR1cm4gY2ZnLmJhc2VVcmwgKyB1cmw7XG4gIH0sXG5cbiAgZ2V0QWN0aXZlU2Vzc2lvblJvdXRlVXJsKHNpZCl7XG4gICAgcmV0dXJuIGZvcm1hdFBhdHRlcm4oY2ZnLnJvdXRlcy5hY3RpdmVTZXNzaW9uLCB7c2lkfSk7XG4gIH0sXG5cbiAgZ2V0QXV0aFByb3ZpZGVycygpe1xuICAgIHJldHVybiBjZmcuYXV0aC5vaWRjX2Nvbm5lY3RvcnM7XG4gIH0sXG5cbiAgaW5pdChjb25maWc9e30pe1xuICAgICQuZXh0ZW5kKHRydWUsIHRoaXMsIGNvbmZpZyk7XG4gIH1cbn1cblxuZXhwb3J0IGRlZmF1bHQgY2ZnO1xuXG5mdW5jdGlvbiBnZXRXc0hvc3ROYW1lKCl7XG4gIHZhciBwcmVmaXggPSBsb2NhdGlvbi5wcm90b2NvbCA9PSBcImh0dHBzOlwiP1wid3NzOi8vXCI6XCJ3czovL1wiO1xuICB2YXIgaG9zdHBvcnQgPSBsb2NhdGlvbi5ob3N0bmFtZSsobG9jYXRpb24ucG9ydCA/ICc6Jytsb2NhdGlvbi5wb3J0OiAnJyk7XG4gIHJldHVybiBgJHtwcmVmaXh9JHtob3N0cG9ydH1gO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbmZpZy5qc1xuICoqLyIsIm1vZHVsZS5leHBvcnRzID0galF1ZXJ5O1xuXG5cbi8qKioqKioqKioqKioqKioqKlxuICoqIFdFQlBBQ0sgRk9PVEVSXG4gKiogZXh0ZXJuYWwgXCJqUXVlcnlcIlxuICoqIG1vZHVsZSBpZCA9IDIyXG4gKiogbW9kdWxlIGNodW5rcyA9IDFcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbmNsYXNzIExvZ2dlciB7XG4gIGNvbnN0cnVjdG9yKG5hbWU9J2RlZmF1bHQnKSB7XG4gICAgdGhpcy5uYW1lID0gbmFtZTtcbiAgfVxuXG4gIGxvZyhsZXZlbD0nbG9nJywgLi4uYXJncykge1xuICAgIGNvbnNvbGVbbGV2ZWxdKGAlY1ske3RoaXMubmFtZX1dYCwgYGNvbG9yOiBibHVlO2AsIC4uLmFyZ3MpO1xuICB9XG5cbiAgdHJhY2UoLi4uYXJncykge1xuICAgIHRoaXMubG9nKCd0cmFjZScsIC4uLmFyZ3MpO1xuICB9XG5cbiAgd2FybiguLi5hcmdzKSB7XG4gICAgdGhpcy5sb2coJ3dhcm4nLCAuLi5hcmdzKTtcbiAgfVxuXG4gIGluZm8oLi4uYXJncykge1xuICAgIHRoaXMubG9nKCdpbmZvJywgLi4uYXJncyk7XG4gIH1cblxuICBlcnJvciguLi5hcmdzKSB7XG4gICAgdGhpcy5sb2coJ2Vycm9yJywgLi4uYXJncyk7XG4gIH1cbn1cblxuZXhwb3J0IGRlZmF1bHQge1xuICBjcmVhdGU6ICguLi5hcmdzKSA9PiBuZXcgTG9nZ2VyKC4uLmFyZ3MpXG59O1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbW1vbi9sb2dnZXIuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciAkID0gcmVxdWlyZShcImpRdWVyeVwiKTtcbnZhciBzZXNzaW9uID0gcmVxdWlyZSgnLi9zZXNzaW9uJyk7XG5cbmNvbnN0IGFwaSA9IHtcblxuICBwdXQocGF0aCwgZGF0YSwgd2l0aFRva2VuKXtcbiAgICByZXR1cm4gYXBpLmFqYXgoe3VybDogcGF0aCwgZGF0YTogSlNPTi5zdHJpbmdpZnkoZGF0YSksIHR5cGU6ICdQVVQnfSwgd2l0aFRva2VuKTtcbiAgfSxcblxuICBwb3N0KHBhdGgsIGRhdGEsIHdpdGhUb2tlbil7XG4gICAgcmV0dXJuIGFwaS5hamF4KHt1cmw6IHBhdGgsIGRhdGE6IEpTT04uc3RyaW5naWZ5KGRhdGEpLCB0eXBlOiAnUE9TVCd9LCB3aXRoVG9rZW4pO1xuICB9LFxuXG4gIGdldChwYXRoKXtcbiAgICByZXR1cm4gYXBpLmFqYXgoe3VybDogcGF0aH0pO1xuICB9LFxuXG4gIGFqYXgoY2ZnLCB3aXRoVG9rZW4gPSB0cnVlKXtcbiAgICB2YXIgZGVmYXVsdENmZyA9IHtcbiAgICAgIHR5cGU6IFwiR0VUXCIsXG4gICAgICBkYXRhVHlwZTogXCJqc29uXCIsXG4gICAgICBiZWZvcmVTZW5kOiBmdW5jdGlvbih4aHIpIHtcbiAgICAgICAgaWYod2l0aFRva2VuKXtcbiAgICAgICAgICB2YXIgeyB0b2tlbiB9ID0gc2Vzc2lvbi5nZXRVc2VyRGF0YSgpO1xuICAgICAgICAgIHhoci5zZXRSZXF1ZXN0SGVhZGVyKCdBdXRob3JpemF0aW9uJywnQmVhcmVyICcgKyB0b2tlbik7XG4gICAgICAgIH1cbiAgICAgICB9XG4gICAgfVxuXG4gICAgcmV0dXJuICQuYWpheCgkLmV4dGVuZCh7fSwgZGVmYXVsdENmZywgY2ZnKSk7XG4gIH1cbn1cblxubW9kdWxlLmV4cG9ydHMgPSBhcGk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvc2VydmljZXMvYXBpLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgeyBicm93c2VySGlzdG9yeSwgY3JlYXRlTWVtb3J5SGlzdG9yeSB9ID0gcmVxdWlyZSgncmVhY3Qtcm91dGVyJyk7XG5cbmNvbnN0IGxvZ2dlciA9IHJlcXVpcmUoJ2FwcC9jb21tb24vbG9nZ2VyJykuY3JlYXRlKCdzZXJ2aWNlcy9zZXNzaW9ucycpO1xuY29uc3QgQVVUSF9LRVlfREFUQSA9ICdhdXRoRGF0YSc7XG5cbnZhciBfaGlzdG9yeSA9IGNyZWF0ZU1lbW9yeUhpc3RvcnkoKTtcblxudmFyIHNlc3Npb24gPSB7XG5cbiAgaW5pdChoaXN0b3J5PWJyb3dzZXJIaXN0b3J5KXtcbiAgICBfaGlzdG9yeSA9IGhpc3Rvcnk7XG4gIH0sXG5cbiAgZ2V0SGlzdG9yeSgpe1xuICAgIHJldHVybiBfaGlzdG9yeTtcbiAgfSxcblxuICBzZXRVc2VyRGF0YSh1c2VyRGF0YSl7XG4gICAgbG9jYWxTdG9yYWdlLnNldEl0ZW0oQVVUSF9LRVlfREFUQSwgSlNPTi5zdHJpbmdpZnkodXNlckRhdGEpKTtcbiAgfSxcblxuICBnZXRVc2VyRGF0YSgpe1xuICAgIHZhciBpdGVtID0gbG9jYWxTdG9yYWdlLmdldEl0ZW0oQVVUSF9LRVlfREFUQSk7XG4gICAgaWYoaXRlbSl7XG4gICAgICByZXR1cm4gSlNPTi5wYXJzZShpdGVtKTtcbiAgICB9XG5cbiAgICAvLyBmb3Igc3NvIHVzZS1jYXNlcywgdHJ5IHRvIGdyYWIgdGhlIHRva2VuIGZyb20gSFRNTFxuICAgIHZhciBoaWRkZW5EaXYgPSBkb2N1bWVudC5nZXRFbGVtZW50QnlJZChcImJlYXJlcl90b2tlblwiKTtcbiAgICBpZihoaWRkZW5EaXYgIT09IG51bGwgKXtcbiAgICAgIHRyeXtcbiAgICAgICAgbGV0IGpzb24gPSB3aW5kb3cuYXRvYihoaWRkZW5EaXYudGV4dENvbnRlbnQpO1xuICAgICAgICBsZXQgdXNlckRhdGEgPSBKU09OLnBhcnNlKGpzb24pO1xuICAgICAgICBpZih1c2VyRGF0YS50b2tlbil7XG4gICAgICAgICAgLy8gcHV0IGl0IGludG8gdGhlIHNlc3Npb25cbiAgICAgICAgICB0aGlzLnNldFVzZXJEYXRhKHVzZXJEYXRhKTtcbiAgICAgICAgICAvLyByZW1vdmUgdGhlIGVsZW1lbnRcbiAgICAgICAgICBoaWRkZW5EaXYucmVtb3ZlKCk7XG4gICAgICAgICAgcmV0dXJuIHVzZXJEYXRhO1xuICAgICAgICB9XG4gICAgICB9Y2F0Y2goZXJyKXtcbiAgICAgICAgbG9nZ2VyLmVycm9yKCdlcnJvciBwYXJzaW5nIFNTTyB0b2tlbjonLCBlcnIpO1xuICAgICAgfVxuICAgIH1cblxuICAgIHJldHVybiB7fTtcbiAgfSxcblxuICBjbGVhcigpe1xuICAgIGxvY2FsU3RvcmFnZS5jbGVhcigpXG4gIH1cblxufVxuXG5tb2R1bGUuZXhwb3J0cyA9IHNlc3Npb247XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvc2VydmljZXMvc2Vzc2lvbi5qc1xuICoqLyIsIm1vZHVsZS5leHBvcnRzID0gXztcblxuXG4vKioqKioqKioqKioqKioqKipcbiAqKiBXRUJQQUNLIEZPT1RFUlxuICoqIGV4dGVybmFsIFwiX1wiXG4gKiogbW9kdWxlIGlkID0gNDNcbiAqKiBtb2R1bGUgY2h1bmtzID0gMVxuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciBsb2dvU3ZnID0gcmVxdWlyZSgnYXNzZXRzL2ltZy9zdmcvZ3J2LXRscHQtbG9nby1mdWxsLnN2ZycpO1xudmFyIGNsYXNzbmFtZXMgPSByZXF1aXJlKCdjbGFzc25hbWVzJyk7XG5cbmNvbnN0IFRlbGVwb3J0TG9nbyA9ICgpID0+IChcbiAgPHN2ZyBjbGFzc05hbWU9XCJncnYtaWNvbi1sb2dvLXRscHRcIj48dXNlIHhsaW5rSHJlZj17bG9nb1N2Z30vPjwvc3ZnPlxuKVxuXG5jb25zdCBVc2VySWNvbiA9ICh7bmFtZT0nJywgaXNEYXJrfSk9PntcbiAgbGV0IGljb25DbGFzcyA9IGNsYXNzbmFtZXMoJ2dydi1pY29uLXVzZXInLCB7XG4gICAgJy0tZGFyaycgOiBpc0RhcmtcbiAgfSk7XG5cbiAgcmV0dXJuIChcbiAgICA8ZGl2IHRpdGxlPXtuYW1lfSBjbGFzc05hbWU9e2ljb25DbGFzc30+XG4gICAgICA8c3Bhbj5cbiAgICAgICAgPHN0cm9uZz57bmFtZVswXX08L3N0cm9uZz5cbiAgICAgIDwvc3Bhbj5cbiAgICA8L2Rpdj5cbiAgKVxufTtcblxuZXhwb3J0IHtUZWxlcG9ydExvZ28sIFVzZXJJY29ufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvaWNvbnMuanN4XG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xuXG5jb25zdCBNU0dfSU5GT19MT0dJTl9TVUNDRVNTID0gJ0xvZ2luIHdhcyBzdWNjZXNzZnVsLCB5b3UgY2FuIGNsb3NlIHRoaXMgd2luZG93IGFuZCBjb250aW51ZSB1c2luZyB0c2guJztcbmNvbnN0IE1TR19FUlJPUl9MT0dJTl9GQUlMRUQgPSAnTG9naW4gdW5zdWNjZXNzZnVsLiBQbGVhc2UgdHJ5IGFnYWluLCBpZiB0aGUgcHJvYmxlbSBwZXJzaXN0cywgY29udGFjdCB5b3VyIHN5c3RlbSBhZG1pbmlzdGF0b3IuJztcbmNvbnN0IE1TR19FUlJPUl9ERUZBVUxUID0gJ1dob29wcywgc29tZXRoaW5nIHdlbnQgd3JvbmcuJztcblxuY29uc3QgTVNHX0VSUk9SX05PVF9GT1VORCA9ICdXaG9vcHMsIHdlIGNhbm5vdCBmaW5kIHRoYXQuJztcbmNvbnN0IE1TR19FUlJPUl9OT1RfRk9VTkRfREVUQUlMUyA9IGBMb29rcyBsaWtlIHRoZSBwYWdlIHlvdSBhcmUgbG9va2luZyBmb3IgaXNuJ3QgaGVyZSBhbnkgbG9uZ2VyLmA7XG5cbmNvbnN0IE1TR19FUlJPUl9FWFBJUkVEX0lOVklURSA9ICdJbnZpdGUgY29kZSBoYXMgZXhwaXJlZC4nO1xuY29uc3QgTVNHX0VSUk9SX0VYUElSRURfSU5WSVRFX0RFVEFJTFMgPSBgTG9va3MgbGlrZSB5b3VyIGludml0ZSBjb2RlIGlzbid0IHZhbGlkIGFueW1vcmUuYDtcblxuY29uc3QgTXNnVHlwZSA9IHtcbiAgSU5GTzogJ2luZm8nLFxuICBFUlJPUjogJ2Vycm9yJ1xufVxuXG5jb25zdCBFcnJvclR5cGVzID0ge1xuICBGQUlMRURfVE9fTE9HSU46ICdsb2dpbl9mYWlsZWQnLFxuICBFWFBJUkVEX0lOVklURTogJ2V4cGlyZWRfaW52aXRlJyxcbiAgTk9UX0ZPVU5EOiAnbm90X2ZvdW5kJ1xufTtcblxuY29uc3QgSW5mb1R5cGVzID0ge1xuICBMT0dJTl9TVUNDRVNTOiAnbG9naW5fc3VjY2Vzcydcbn07XG5cbnZhciBNZXNzYWdlUGFnZSA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgcmVuZGVyKCl7XG4gICAgbGV0IHt0eXBlLCBzdWJUeXBlfSA9IHRoaXMucHJvcHMucGFyYW1zO1xuICAgIGlmKHR5cGUgPT09IE1zZ1R5cGUuRVJST1Ipe1xuICAgICAgcmV0dXJuIDxFcnJvclBhZ2UgdHlwZT17c3ViVHlwZX0vPlxuICAgIH1cblxuICAgIGlmKHR5cGUgPT09IE1zZ1R5cGUuSU5GTyl7XG4gICAgICByZXR1cm4gPEluZm9QYWdlIHR5cGU9e3N1YlR5cGV9Lz5cbiAgICB9XG5cbiAgICByZXR1cm4gbnVsbDtcbiAgfVxufSk7XG5cbnZhciBFcnJvclBhZ2UgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIHJlbmRlcigpIHtcbiAgICBsZXQge3R5cGV9ID0gdGhpcy5wcm9wcztcbiAgICBsZXQgbXNnQm9keSA9IChcbiAgICAgIDxkaXY+XG4gICAgICAgIDxoMT57TVNHX0VSUk9SX0RFRkFVTFR9PC9oMT5cbiAgICAgIDwvZGl2PlxuICAgICk7XG5cbiAgICBpZih0eXBlID09PSBFcnJvclR5cGVzLkZBSUxFRF9UT19MT0dJTil7XG4gICAgICBtc2dCb2R5ID0gKFxuICAgICAgICA8ZGl2PlxuICAgICAgICAgIDxoMT57TVNHX0VSUk9SX0xPR0lOX0ZBSUxFRH08L2gxPlxuICAgICAgICA8L2Rpdj5cbiAgICAgIClcbiAgICB9XG5cbiAgICBpZih0eXBlID09PSBFcnJvclR5cGVzLkVYUElSRURfSU5WSVRFKXtcbiAgICAgIG1zZ0JvZHkgPSAoXG4gICAgICAgIDxkaXY+XG4gICAgICAgICAgPGgxPntNU0dfRVJST1JfRVhQSVJFRF9JTlZJVEV9PC9oMT5cbiAgICAgICAgICA8ZGl2PntNU0dfRVJST1JfRVhQSVJFRF9JTlZJVEVfREVUQUlMU308L2Rpdj5cbiAgICAgICAgPC9kaXY+XG4gICAgICApXG4gICAgfVxuXG4gICAgaWYoIHR5cGUgPT09IEVycm9yVHlwZXMuTk9UX0ZPVU5EKXtcbiAgICAgIG1zZ0JvZHkgPSAoXG4gICAgICAgIDxkaXY+XG4gICAgICAgICAgPGgxPntNU0dfRVJST1JfTk9UX0ZPVU5EfTwvaDE+XG4gICAgICAgICAgPGRpdj57TVNHX0VSUk9SX05PVF9GT1VORF9ERVRBSUxTfTwvZGl2PlxuICAgICAgICA8L2Rpdj5cbiAgICAgICk7XG4gICAgfVxuXG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LW1zZy1wYWdlXCI+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWhlYWRlclwiPjxpIGNsYXNzTmFtZT1cImZhIGZhLWZyb3duLW9cIj48L2k+IDwvZGl2PlxuICAgICAgICB7bXNnQm9keX1cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJjb250YWN0LXNlY3Rpb25cIj5JZiB5b3UgYmVsaWV2ZSB0aGlzIGlzIGFuIGlzc3VlIHdpdGggVGVsZXBvcnQsIHBsZWFzZSA8YSBocmVmPVwiaHR0cHM6Ly9naXRodWIuY29tL2dyYXZpdGF0aW9uYWwvdGVsZXBvcnQvaXNzdWVzL25ld1wiPmNyZWF0ZSBhIEdpdEh1YiBpc3N1ZS48L2E+PC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KVxuXG52YXIgSW5mb1BhZ2UgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIHJlbmRlcigpIHtcbiAgICBsZXQge3R5cGV9ID0gdGhpcy5wcm9wcztcbiAgICBsZXQgbXNnQm9keSA9IG51bGw7XG5cbiAgICBpZih0eXBlID09PSBJbmZvVHlwZXMuTE9HSU5fU1VDQ0VTUyl7XG4gICAgICBtc2dCb2R5ID0gKFxuICAgICAgICA8ZGl2PlxuICAgICAgICAgIDxoMT57TVNHX0lORk9fTE9HSU5fU1VDQ0VTU308L2gxPlxuICAgICAgICA8L2Rpdj5cbiAgICAgICk7XG4gICAgfVxuXG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LW1zZy1wYWdlXCI+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWhlYWRlclwiPjxpIGNsYXNzTmFtZT1cImZhIGZhLXNtaWxlLW9cIj48L2k+IDwvZGl2PlxuICAgICAgICB7bXNnQm9keX1cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pXG5cbnZhciBOb3RGb3VuZCA9ICgpID0+IChcbiAgPEVycm9yUGFnZSB0eXBlPXtFcnJvclR5cGVzLk5PVF9GT1VORH0vPlxuKVxuXG5leHBvcnQge0Vycm9yUGFnZSwgSW5mb1BhZ2UsIE5vdEZvdW5kLCBFcnJvclR5cGVzLCBNZXNzYWdlUGFnZX07XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9tc2dQYWdlLmpzeFxuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcblxuY29uc3QgR3J2VGFibGVUZXh0Q2VsbCA9ICh7cm93SW5kZXgsIGRhdGEsIGNvbHVtbktleSwgLi4ucHJvcHN9KSA9PiAoXG4gIDxHcnZUYWJsZUNlbGwgey4uLnByb3BzfT5cbiAgICB7ZGF0YVtyb3dJbmRleF1bY29sdW1uS2V5XX1cbiAgPC9HcnZUYWJsZUNlbGw+XG4pO1xuXG4vKipcbiogU29ydCBpbmRpY2F0b3IgdXNlZCBieSBTb3J0SGVhZGVyQ2VsbFxuKi9cbmNvbnN0IFNvcnRUeXBlcyA9IHtcbiAgQVNDOiAnQVNDJyxcbiAgREVTQzogJ0RFU0MnXG59O1xuXG5jb25zdCBTb3J0SW5kaWNhdG9yID0gKHtzb3J0RGlyfSk9PntcbiAgbGV0IGNscyA9ICdncnYtdGFibGUtaW5kaWNhdG9yLXNvcnQgZmEgZmEtc29ydCdcbiAgaWYoc29ydERpciA9PT0gU29ydFR5cGVzLkRFU0Mpe1xuICAgIGNscyArPSAnLWRlc2MnXG4gIH1cblxuICBpZiggc29ydERpciA9PT0gU29ydFR5cGVzLkFTQyl7XG4gICAgY2xzICs9ICctYXNjJ1xuICB9XG5cbiAgcmV0dXJuICg8aSBjbGFzc05hbWU9e2Nsc30+PC9pPik7XG59O1xuXG4vKipcbiogU29ydCBIZWFkZXIgQ2VsbFxuKi9cbnZhciBTb3J0SGVhZGVyQ2VsbCA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgcmVuZGVyKCkge1xuICAgIHZhciB7c29ydERpciwgdGl0bGUsIC4uLnByb3BzfSA9IHRoaXMucHJvcHM7XG5cbiAgICByZXR1cm4gKFxuICAgICAgPEdydlRhYmxlQ2VsbCB7Li4ucHJvcHN9PlxuICAgICAgICA8YSBvbkNsaWNrPXt0aGlzLm9uU29ydENoYW5nZX0+XG4gICAgICAgICAge3RpdGxlfVxuICAgICAgICA8L2E+XG4gICAgICAgIDxTb3J0SW5kaWNhdG9yIHNvcnREaXI9e3NvcnREaXJ9Lz5cbiAgICAgIDwvR3J2VGFibGVDZWxsPlxuICAgICk7XG4gIH0sXG5cbiAgb25Tb3J0Q2hhbmdlKGUpIHtcbiAgICBlLnByZXZlbnREZWZhdWx0KCk7XG4gICAgaWYodGhpcy5wcm9wcy5vblNvcnRDaGFuZ2UpIHtcbiAgICAgIC8vIGRlZmF1bHRcbiAgICAgIGxldCBuZXdEaXIgPSBTb3J0VHlwZXMuREVTQztcbiAgICAgIGlmKHRoaXMucHJvcHMuc29ydERpcil7XG4gICAgICAgIG5ld0RpciA9IHRoaXMucHJvcHMuc29ydERpciA9PT0gU29ydFR5cGVzLkRFU0MgPyBTb3J0VHlwZXMuQVNDIDogU29ydFR5cGVzLkRFU0M7XG4gICAgICB9XG4gICAgICB0aGlzLnByb3BzLm9uU29ydENoYW5nZSh0aGlzLnByb3BzLmNvbHVtbktleSwgbmV3RGlyKTtcbiAgICB9XG4gIH1cbn0pO1xuXG4vKipcbiogRGVmYXVsdCBDZWxsXG4qL1xudmFyIEdydlRhYmxlQ2VsbCA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgcmVuZGVyKCl7XG4gICAgdmFyIHByb3BzID0gdGhpcy5wcm9wcztcbiAgICByZXR1cm4gcHJvcHMuaXNIZWFkZXIgPyA8dGgga2V5PXtwcm9wcy5rZXl9IGNsYXNzTmFtZT1cImdydi10YWJsZS1jZWxsXCI+e3Byb3BzLmNoaWxkcmVufTwvdGg+IDogPHRkIGtleT17cHJvcHMua2V5fT57cHJvcHMuY2hpbGRyZW59PC90ZD47XG4gIH1cbn0pO1xuXG4vKipcbiogVGFibGVcbiovXG52YXIgR3J2VGFibGUgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgcmVuZGVySGVhZGVyKGNoaWxkcmVuKXtcbiAgICB2YXIgY2VsbHMgPSBjaGlsZHJlbi5tYXAoKGl0ZW0sIGluZGV4KT0+e1xuICAgICAgcmV0dXJuIHRoaXMucmVuZGVyQ2VsbChpdGVtLnByb3BzLmhlYWRlciwge2luZGV4LCBrZXk6IGluZGV4LCBpc0hlYWRlcjogdHJ1ZSwgLi4uaXRlbS5wcm9wc30pO1xuICAgIH0pXG5cbiAgICByZXR1cm4gPHRoZWFkIGNsYXNzTmFtZT1cImdydi10YWJsZS1oZWFkZXJcIj48dHI+e2NlbGxzfTwvdHI+PC90aGVhZD5cbiAgfSxcblxuICByZW5kZXJCb2R5KGNoaWxkcmVuKXtcbiAgICB2YXIgY291bnQgPSB0aGlzLnByb3BzLnJvd0NvdW50O1xuICAgIHZhciByb3dzID0gW107XG4gICAgZm9yKHZhciBpID0gMDsgaSA8IGNvdW50OyBpICsrKXtcbiAgICAgIHZhciBjZWxscyA9IGNoaWxkcmVuLm1hcCgoaXRlbSwgaW5kZXgpPT57XG4gICAgICAgIHJldHVybiB0aGlzLnJlbmRlckNlbGwoaXRlbS5wcm9wcy5jZWxsLCB7cm93SW5kZXg6IGksIGtleTogaW5kZXgsIGlzSGVhZGVyOiBmYWxzZSwgLi4uaXRlbS5wcm9wc30pO1xuICAgICAgfSlcblxuICAgICAgcm93cy5wdXNoKDx0ciBrZXk9e2l9PntjZWxsc308L3RyPik7XG4gICAgfVxuXG4gICAgcmV0dXJuIDx0Ym9keT57cm93c308L3Rib2R5PjtcbiAgfSxcblxuICByZW5kZXJDZWxsKGNlbGwsIGNlbGxQcm9wcyl7XG4gICAgdmFyIGNvbnRlbnQgPSBudWxsO1xuICAgIGlmIChSZWFjdC5pc1ZhbGlkRWxlbWVudChjZWxsKSkge1xuICAgICAgIGNvbnRlbnQgPSBSZWFjdC5jbG9uZUVsZW1lbnQoY2VsbCwgY2VsbFByb3BzKTtcbiAgICAgfSBlbHNlIGlmICh0eXBlb2YgY2VsbCA9PT0gJ2Z1bmN0aW9uJykge1xuICAgICAgIGNvbnRlbnQgPSBjZWxsKGNlbGxQcm9wcyk7XG4gICAgIH1cblxuICAgICByZXR1cm4gY29udGVudDtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgdmFyIGNoaWxkcmVuID0gW107XG4gICAgUmVhY3QuQ2hpbGRyZW4uZm9yRWFjaCh0aGlzLnByb3BzLmNoaWxkcmVuLCAoY2hpbGQpID0+IHtcbiAgICAgIGlmIChjaGlsZCA9PSBudWxsKSB7XG4gICAgICAgIHJldHVybjtcbiAgICAgIH1cblxuICAgICAgaWYoY2hpbGQudHlwZS5kaXNwbGF5TmFtZSAhPT0gJ0dydlRhYmxlQ29sdW1uJyl7XG4gICAgICAgIHRocm93ICdTaG91bGQgYmUgR3J2VGFibGVDb2x1bW4nO1xuICAgICAgfVxuXG4gICAgICBjaGlsZHJlbi5wdXNoKGNoaWxkKTtcbiAgICB9KTtcblxuICAgIHZhciB0YWJsZUNsYXNzID0gJ3RhYmxlIGdydi10YWJsZSAnICsgdGhpcy5wcm9wcy5jbGFzc05hbWU7XG5cbiAgICByZXR1cm4gKFxuICAgICAgPHRhYmxlIGNsYXNzTmFtZT17dGFibGVDbGFzc30+XG4gICAgICAgIHt0aGlzLnJlbmRlckhlYWRlcihjaGlsZHJlbil9XG4gICAgICAgIHt0aGlzLnJlbmRlckJvZHkoY2hpbGRyZW4pfVxuICAgICAgPC90YWJsZT5cbiAgICApO1xuICB9XG59KVxuXG52YXIgR3J2VGFibGVDb2x1bW4gPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgdGhyb3cgbmV3IEVycm9yKCdDb21wb25lbnQgPEdydlRhYmxlQ29sdW1uIC8+IHNob3VsZCBuZXZlciByZW5kZXInKTtcbiAgfVxufSlcblxuY29uc3QgRW1wdHlJbmRpY2F0b3IgPSAoe3RleHR9KSA9PiAoXG4gIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXRhYmxlLWluZGljYXRvci1lbXB0eSB0ZXh0LWNlbnRlciB0ZXh0LW11dGVkXCI+PHNwYW4+e3RleHR9PC9zcGFuPjwvZGl2PlxuKVxuXG5leHBvcnQgZGVmYXVsdCBHcnZUYWJsZTtcbmV4cG9ydCB7XG4gIEdydlRhYmxlQ29sdW1uIGFzIENvbHVtbixcbiAgR3J2VGFibGUgYXMgVGFibGUsXG4gIEdydlRhYmxlQ2VsbCBhcyBDZWxsLFxuICBHcnZUYWJsZVRleHRDZWxsIGFzIFRleHRDZWxsLFxuICBTb3J0SGVhZGVyQ2VsbCxcbiAgU29ydEluZGljYXRvcixcbiAgU29ydFR5cGVzLFxuICBFbXB0eUluZGljYXRvcn07XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy90YWJsZS5qc3hcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbmNvbnN0IG5vZGVIb3N0TmFtZUJ5U2VydmVySWQgPSAoc2VydmVySWQpID0+IFsgWyd0bHB0X25vZGVzJ10sIChub2RlcykgPT57XG4gIGxldCBzZXJ2ZXIgPSBub2Rlcy5maW5kKGl0ZW09PiBpdGVtLmdldCgnaWQnKSA9PT0gc2VydmVySWQpO1xuICByZXR1cm4gIXNlcnZlciA/ICcnIDogc2VydmVyLmdldCgnaG9zdG5hbWUnKTtcbn1dO1xuXG5jb25zdCBub2RlTGlzdFZpZXcgPSBbIFsndGxwdF9ub2RlcyddLCAobm9kZXMpID0+e1xuICAgIHJldHVybiBub2Rlcy5tYXAoKGl0ZW0pPT57XG4gICAgICB2YXIgc2VydmVySWQgPSBpdGVtLmdldCgnaWQnKTtcbiAgICAgIHJldHVybiB7XG4gICAgICAgIGlkOiBzZXJ2ZXJJZCxcbiAgICAgICAgaG9zdG5hbWU6IGl0ZW0uZ2V0KCdob3N0bmFtZScpLFxuICAgICAgICB0YWdzOiBnZXRUYWdzKGl0ZW0pLFxuICAgICAgICBhZGRyOiBpdGVtLmdldCgnYWRkcicpXG4gICAgICB9XG4gICAgfSkudG9KUygpO1xuIH1cbl07XG5cbmZ1bmN0aW9uIGdldFRhZ3Mobm9kZSl7XG4gIHZhciBhbGxMYWJlbHMgPSBbXTtcbiAgdmFyIGxhYmVscyA9IG5vZGUuZ2V0KCdsYWJlbHMnKTtcblxuICBpZihsYWJlbHMpe1xuICAgIGxhYmVscy5lbnRyeVNlcSgpLnRvQXJyYXkoKS5mb3JFYWNoKGl0ZW09PntcbiAgICAgIGFsbExhYmVscy5wdXNoKHtcbiAgICAgICAgcm9sZTogaXRlbVswXSxcbiAgICAgICAgdmFsdWU6IGl0ZW1bMV1cbiAgICAgIH0pO1xuICAgIH0pO1xuICB9XG5cbiAgbGFiZWxzID0gbm9kZS5nZXQoJ2NtZF9sYWJlbHMnKTtcblxuICBpZihsYWJlbHMpe1xuICAgIGxhYmVscy5lbnRyeVNlcSgpLnRvQXJyYXkoKS5mb3JFYWNoKGl0ZW09PntcbiAgICAgIGFsbExhYmVscy5wdXNoKHtcbiAgICAgICAgcm9sZTogaXRlbVswXSxcbiAgICAgICAgdmFsdWU6IGl0ZW1bMV0uZ2V0KCdyZXN1bHQnKSxcbiAgICAgICAgdG9vbHRpcDogaXRlbVsxXS5nZXQoJ2NvbW1hbmQnKVxuICAgICAgfSk7XG4gICAgfSk7XG4gIH1cblxuICByZXR1cm4gYWxsTGFiZWxzO1xufVxuXG5leHBvcnQgZGVmYXVsdCB7XG4gIG5vZGVMaXN0VmlldyxcbiAgbm9kZUhvc3ROYW1lQnlTZXJ2ZXJJZFxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvZ2V0dGVycy5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHsgVExQVF9OT1RJRklDQVRJT05TX0FERCB9ICA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuXG4gIHNob3dFcnJvcih0ZXh0LCB0aXRsZT0nRVJST1InKXtcbiAgICBkaXNwYXRjaCh7aXNFcnJvcjogdHJ1ZSwgdGV4dDogdGV4dCwgdGl0bGV9KTtcbiAgfSxcblxuICBzaG93U3VjY2Vzcyh0ZXh0LCB0aXRsZT0nU1VDQ0VTUycpe1xuICAgIGRpc3BhdGNoKHtpc1N1Y2Nlc3M6dHJ1ZSwgdGV4dDogdGV4dCwgdGl0bGV9KTtcbiAgfSxcblxuICBzaG93SW5mbyh0ZXh0LCB0aXRsZT0nSU5GTycpe1xuICAgIGRpc3BhdGNoKHtpc0luZm86dHJ1ZSwgdGV4dDogdGV4dCwgdGl0bGV9KTtcbiAgfSxcblxuICBzaG93V2FybmluZyh0ZXh0LCB0aXRsZT0nV0FSTklORycpe1xuICAgIGRpc3BhdGNoKHtpc1dhcm5pbmc6IHRydWUsIHRleHQ6IHRleHQsIHRpdGxlfSk7XG4gIH1cblxufVxuXG5mdW5jdGlvbiBkaXNwYXRjaChtc2cpe1xuICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfTk9USUZJQ0FUSU9OU19BREQsIG1zZyk7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9ub3RpZmljYXRpb25zL2FjdGlvbnMuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciB7Y3JlYXRlVmlld30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9zZXNzaW9ucy9nZXR0ZXJzJyk7XG5cbmNvbnN0IGN1cnJlbnRTZXNzaW9uID0gWyBbJ3RscHRfY3VycmVudF9zZXNzaW9uJ10sIFsndGxwdF9zZXNzaW9ucyddLFxuKGN1cnJlbnQsIHNlc3Npb25zKSA9PiB7XG4gICAgaWYoIWN1cnJlbnQpe1xuICAgICAgcmV0dXJuIG51bGw7XG4gICAgfVxuXG4gICAgLypcbiAgICAqIGFjdGl2ZSBzZXNzaW9uIG5lZWRzIHRvIGhhdmUgaXRzIG93biB2aWV3IGFzIGFuIGFjdHVhbCBzZXNzaW9uIG1pZ2h0IG5vdFxuICAgICogZXhpc3QgYXQgdGhpcyBwb2ludC4gRm9yIGV4YW1wbGUsIHVwb24gY3JlYXRpbmcgYSBuZXcgc2Vzc2lvbiB3ZSBuZWVkIHRvIGtub3dcbiAgICAqIGxvZ2luIGFuZCBzZXJ2ZXJJZC4gSXQgd2lsbCBiZSBzaW1wbGlmaWVkIG9uY2Ugc2VydmVyIEFQSSBnZXRzIGV4dGVuZGVkLlxuICAgICovXG4gICAgbGV0IGN1clNlc3Npb25WaWV3ID0ge1xuICAgICAgaXNOZXdTZXNzaW9uOiBjdXJyZW50LmdldCgnaXNOZXdTZXNzaW9uJyksXG4gICAgICBub3RGb3VuZDogY3VycmVudC5nZXQoJ25vdEZvdW5kJyksXG4gICAgICBhZGRyOiBjdXJyZW50LmdldCgnYWRkcicpLFxuICAgICAgc2VydmVySWQ6IGN1cnJlbnQuZ2V0KCdzZXJ2ZXJJZCcpLFxuICAgICAgc2VydmVySXA6IHVuZGVmaW5lZCxcbiAgICAgIGxvZ2luOiBjdXJyZW50LmdldCgnbG9naW4nKSxcbiAgICAgIHNpZDogY3VycmVudC5nZXQoJ3NpZCcpLFxuICAgICAgY29sczogdW5kZWZpbmVkLFxuICAgICAgcm93czogdW5kZWZpbmVkXG4gICAgfTtcblxuICAgIC8qXG4gICAgKiBpbiBjYXNlIGlmIHNlc3Npb24gYWxyZWFkeSBleGlzdHMsIGdldCBpdHMgdmlldyBkYXRhIChmb3IgZXhhbXBsZSwgd2hlbiBqb2luaW5nIGFuIGV4aXN0aW5nIHNlc3Npb24pXG4gICAgKi9cbiAgICBpZihzZXNzaW9ucy5oYXMoY3VyU2Vzc2lvblZpZXcuc2lkKSl7XG4gICAgICBsZXQgZXhpc3RpbmcgPSBjcmVhdGVWaWV3KHNlc3Npb25zLmdldChjdXJTZXNzaW9uVmlldy5zaWQpKTtcblxuICAgICAgY3VyU2Vzc2lvblZpZXcucGFydGllcyA9IGV4aXN0aW5nLnBhcnRpZXM7XG4gICAgICBjdXJTZXNzaW9uVmlldy5zZXJ2ZXJJcCA9IGV4aXN0aW5nLnNlcnZlcklwO1xuICAgICAgY3VyU2Vzc2lvblZpZXcuc2VydmVySWQgPSBleGlzdGluZy5zZXJ2ZXJJZDtcbiAgICAgIGN1clNlc3Npb25WaWV3LmFjdGl2ZSA9IGV4aXN0aW5nLmFjdGl2ZTtcbiAgICAgIGN1clNlc3Npb25WaWV3LmNvbHMgPSBleGlzdGluZy5jb2xzO1xuICAgICAgY3VyU2Vzc2lvblZpZXcucm93cyA9IGV4aXN0aW5nLnJvd3M7XG4gICAgfVxuXG4gICAgcmV0dXJuIGN1clNlc3Npb25WaWV3O1xuICB9XG5dO1xuXG5leHBvcnQgZGVmYXVsdCB7XG4gIGN1cnJlbnRTZXNzaW9uXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9jdXJyZW50U2Vzc2lvbi9nZXR0ZXJzLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG5pbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVExQVF9TRVNTSU5TX1JFQ0VJVkU6IG51bGwsXG4gIFRMUFRfU0VTU0lOU19VUERBVEU6IG51bGwsXG4gIFRMUFRfU0VTU0lOU19SRU1PVkVfU1RPUkVEOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvYWN0aW9uVHlwZXMuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciBhcGkgPSByZXF1aXJlKCdhcHAvc2VydmljZXMvYXBpJyk7XG52YXIgYXBpVXRpbHMgPSByZXF1aXJlKCdhcHAvc2VydmljZXMvYXBpVXRpbHMnKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG52YXIge3Nob3dFcnJvcn0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9ub3RpZmljYXRpb25zL2FjdGlvbnMnKTtcblxuY29uc3QgbG9nZ2VyID0gcmVxdWlyZSgnYXBwL2NvbW1vbi9sb2dnZXInKS5jcmVhdGUoJ01vZHVsZXMvU2Vzc2lvbnMnKTtcbmNvbnN0IHsgVExQVF9TRVNTSU5TX1JFQ0VJVkUsIFRMUFRfU0VTU0lOU19VUERBVEUgfSAgPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmNvbnN0IGFjdGlvbnMgPSB7XG5cbiAgZmV0Y2hTZXNzaW9uKHNpZCl7XG4gICAgcmV0dXJuIGFwaS5nZXQoY2ZnLmFwaS5nZXRGZXRjaFNlc3Npb25Vcmwoc2lkKSkudGhlbihqc29uPT57XG4gICAgICBpZihqc29uICYmIGpzb24uc2Vzc2lvbil7XG4gICAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9TRVNTSU5TX1VQREFURSwganNvbi5zZXNzaW9uKTtcbiAgICAgIH1cbiAgICB9KTtcbiAgfSxcblxuICBmZXRjaFNlc3Npb25zKHtlbmQsIHNpZCwgbGltaXQ9Y2ZnLm1heFNlc3Npb25Mb2FkU2l6ZX09e30pe1xuICAgIGxldCBzdGFydCA9IGVuZCB8fCBuZXcgRGF0ZSgpO1xuICAgIGxldCBwYXJhbXMgPSB7XG4gICAgICBvcmRlcjogLTEsXG4gICAgICBsaW1pdCxcbiAgICAgIHN0YXJ0LFxuICAgICAgc2lkXG4gICAgfTtcblxuICAgIHJldHVybiBhcGlVdGlscy5maWx0ZXJTZXNzaW9ucyhwYXJhbXMpXG4gICAgICAuZG9uZSgoanNvbikgPT4ge1xuICAgICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfU0VTU0lOU19SRUNFSVZFLCBqc29uLnNlc3Npb25zKTtcbiAgICAgIH0pXG4gICAgICAuZmFpbCgoZXJyKT0+e1xuICAgICAgICBzaG93RXJyb3IoJ1VuYWJsZSB0byByZXRyaWV2ZSBsaXN0IG9mIHNlc3Npb25zJyk7XG4gICAgICAgIGxvZ2dlci5lcnJvcignZmV0Y2hTZXNzaW9ucycsIGVycik7XG4gICAgICB9KTtcbiAgfSxcblxuICB1cGRhdGVTZXNzaW9uKGpzb24pe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9TRVNTSU5TX1VQREFURSwganNvbik7XG4gIH1cbn1cblxuZXhwb3J0IGRlZmF1bHQgYWN0aW9ucztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL2FjdGlvbnMuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciB7IHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xuXG5jb25zdCBzZXNzaW9uc0J5U2VydmVyID0gKHNlcnZlcklkKSA9PiBbWyd0bHB0X3Nlc3Npb25zJ10sIChzZXNzaW9ucykgPT57XG4gIHJldHVybiBzZXNzaW9ucy52YWx1ZVNlcSgpLmZpbHRlcihpdGVtPT57XG4gICAgdmFyIHBhcnRpZXMgPSBpdGVtLmdldCgncGFydGllcycpIHx8IHRvSW1tdXRhYmxlKFtdKTtcbiAgICB2YXIgaGFzU2VydmVyID0gcGFydGllcy5maW5kKGl0ZW0yPT4gaXRlbTIuZ2V0KCdzZXJ2ZXJfaWQnKSA9PT0gc2VydmVySWQpO1xuICAgIHJldHVybiBoYXNTZXJ2ZXI7XG4gIH0pLnRvTGlzdCgpO1xufV1cblxuY29uc3Qgc2Vzc2lvbnNWaWV3ID0gW1sndGxwdF9zZXNzaW9ucyddLCAoc2Vzc2lvbnMpID0+e1xuICByZXR1cm4gc2Vzc2lvbnMudmFsdWVTZXEoKS5tYXAoY3JlYXRlVmlldykudG9KUygpO1xufV07XG5cbmNvbnN0IHNlc3Npb25WaWV3QnlJZCA9IChzaWQpPT4gW1sndGxwdF9zZXNzaW9ucycsIHNpZF0sIChzZXNzaW9uKT0+e1xuICBpZighc2Vzc2lvbil7XG4gICAgcmV0dXJuIG51bGw7XG4gIH1cblxuICByZXR1cm4gY3JlYXRlVmlldyhzZXNzaW9uKTtcbn1dO1xuXG5jb25zdCBwYXJ0aWVzQnlTZXNzaW9uSWQgPSAoc2lkKSA9PlxuIFtbJ3RscHRfc2Vzc2lvbnMnLCBzaWQsICdwYXJ0aWVzJ10sIChwYXJ0aWVzKSA9PntcblxuICBpZighcGFydGllcyl7XG4gICAgcmV0dXJuIFtdO1xuICB9XG5cbiAgdmFyIGxhc3RBY3RpdmVVc3JOYW1lID0gZ2V0TGFzdEFjdGl2ZVVzZXIocGFydGllcykuZ2V0KCd1c2VyJyk7XG5cbiAgcmV0dXJuIHBhcnRpZXMubWFwKGl0ZW09PntcbiAgICB2YXIgdXNlciA9IGl0ZW0uZ2V0KCd1c2VyJyk7XG4gICAgcmV0dXJuIHtcbiAgICAgIHVzZXI6IGl0ZW0uZ2V0KCd1c2VyJyksXG4gICAgICBzZXJ2ZXJJcDogaXRlbS5nZXQoJ3JlbW90ZV9hZGRyJyksXG4gICAgICBzZXJ2ZXJJZDogaXRlbS5nZXQoJ3NlcnZlcl9pZCcpLFxuICAgICAgaXNBY3RpdmU6IGxhc3RBY3RpdmVVc3JOYW1lID09PSB1c2VyXG4gICAgfVxuICB9KS50b0pTKCk7XG59XTtcblxuZnVuY3Rpb24gZ2V0TGFzdEFjdGl2ZVVzZXIocGFydGllcyl7XG4gIHJldHVybiBwYXJ0aWVzLnNvcnRCeShpdGVtPT4gbmV3IERhdGUoaXRlbS5nZXQoJ2xhc3RBY3RpdmUnKSkpLmxhc3QoKTtcbn1cblxuZnVuY3Rpb24gY3JlYXRlVmlldyhzZXNzaW9uKXtcbiAgdmFyIHNpZCA9IHNlc3Npb24uZ2V0KCdpZCcpO1xuICB2YXIgc2VydmVySXAsIHNlcnZlcklkO1xuICB2YXIgcGFydGllcyA9IHJlYWN0b3IuZXZhbHVhdGUocGFydGllc0J5U2Vzc2lvbklkKHNpZCkpO1xuXG4gIGlmKHBhcnRpZXMubGVuZ3RoID4gMCl7XG4gICAgc2VydmVySXAgPSBwYXJ0aWVzWzBdLnNlcnZlcklwO1xuICAgIHNlcnZlcklkID0gcGFydGllc1swXS5zZXJ2ZXJJZDtcbiAgfVxuXG4gIHJldHVybiB7XG4gICAgc2lkOiBzaWQsXG4gICAgc2Vzc2lvblVybDogY2ZnLmdldEFjdGl2ZVNlc3Npb25Sb3V0ZVVybChzaWQpLFxuICAgIHNlcnZlcklwLFxuICAgIHNlcnZlcklkLFxuICAgIGFjdGl2ZTogc2Vzc2lvbi5nZXQoJ2FjdGl2ZScpLFxuICAgIGNyZWF0ZWQ6IHNlc3Npb24uZ2V0KCdjcmVhdGVkJyksXG4gICAgbGFzdEFjdGl2ZTogc2Vzc2lvbi5nZXQoJ2xhc3RfYWN0aXZlJyksXG4gICAgbG9naW46IHNlc3Npb24uZ2V0KCdsb2dpbicpLFxuICAgIHBhcnRpZXM6IHBhcnRpZXMsXG4gICAgY29sczogc2Vzc2lvbi5nZXRJbihbJ3Rlcm1pbmFsX3BhcmFtcycsICd3J10pLFxuICAgIHJvd3M6IHNlc3Npb24uZ2V0SW4oWyd0ZXJtaW5hbF9wYXJhbXMnLCAnaCddKVxuICB9XG59XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgcGFydGllc0J5U2Vzc2lvbklkLFxuICBzZXNzaW9uc0J5U2VydmVyLFxuICBzZXNzaW9uc1ZpZXcsXG4gIHNlc3Npb25WaWV3QnlJZCxcbiAgY3JlYXRlVmlldyxcbiAgY291bnQ6IFtbJ3RscHRfc2Vzc2lvbnMnXSwgc2Vzc2lvbnMgPT4gc2Vzc2lvbnMuc2l6ZSBdXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9zZXNzaW9ucy9nZXR0ZXJzLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG5jb25zdCBmaWx0ZXIgPSBbWyd0bHB0X3N0b3JlZF9zZXNzaW9uc19maWx0ZXInXSwgKGZpbHRlcikgPT57XG4gIHJldHVybiBmaWx0ZXIudG9KUygpO1xufV07XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgZmlsdGVyXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9zdG9yZWRTZXNzaW9uc0ZpbHRlci9nZXR0ZXJzLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG5pbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVExQVF9SRUNFSVZFX1VTRVI6IG51bGwsXG4gIFRMUFRfUkVDRUlWRV9VU0VSX0lOVklURTogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvYWN0aW9uVHlwZXMuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciB7VFJZSU5HX1RPX0xPR0lOLCBUUllJTkdfVE9fU0lHTl9VUCwgRkVUQ0hJTkdfSU5WSVRFfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3Jlc3RBcGkvY29uc3RhbnRzJyk7XG52YXIge3JlcXVlc3RTdGF0dXN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvcmVzdEFwaS9nZXR0ZXJzJyk7XG5cbmNvbnN0IGludml0ZSA9IFsgWyd0bHB0X3VzZXJfaW52aXRlJ10sIChpbnZpdGUpID0+IGludml0ZSBdO1xuXG5jb25zdCB1c2VyID0gWyBbJ3RscHRfdXNlciddLCAoY3VycmVudFVzZXIpID0+IHtcbiAgICBpZighY3VycmVudFVzZXIpe1xuICAgICAgcmV0dXJuIG51bGw7XG4gICAgfVxuXG4gICAgdmFyIG5hbWUgPSBjdXJyZW50VXNlci5nZXQoJ25hbWUnKSB8fCAnJztcbiAgICB2YXIgc2hvcnREaXNwbGF5TmFtZSA9IG5hbWVbMF0gfHwgJyc7XG5cbiAgICByZXR1cm4ge1xuICAgICAgbmFtZSxcbiAgICAgIHNob3J0RGlzcGxheU5hbWUsXG4gICAgICBsb2dpbnM6IGN1cnJlbnRVc2VyLmdldCgnYWxsb3dlZF9sb2dpbnMnKS50b0pTKClcbiAgICB9XG4gIH1cbl07XG5cbmV4cG9ydCBkZWZhdWx0IHtcbiAgdXNlcixcbiAgaW52aXRlLFxuICBsb2dpbkF0dGVtcDogcmVxdWVzdFN0YXR1cyhUUllJTkdfVE9fTE9HSU4pLFxuICBhdHRlbXA6IHJlcXVlc3RTdGF0dXMoVFJZSU5HX1RPX1NJR05fVVApLFxuICBmZXRjaGluZ0ludml0ZTogcmVxdWVzdFN0YXR1cyhGRVRDSElOR19JTlZJVEUpXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy91c2VyL2dldHRlcnMuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciBhcGkgPSByZXF1aXJlKCcuL2FwaScpO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCcuL3Nlc3Npb24nKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG52YXIgJCA9IHJlcXVpcmUoJ2pRdWVyeScpO1xuXG5jb25zdCBQUk9WSURFUl9HT09HTEUgPSAnZ29vZ2xlJztcblxuY29uc3QgcmVmcmVzaFJhdGUgPSA2MDAwMCAqIDU7IC8vIDUgbWluXG5cbnZhciByZWZyZXNoVG9rZW5UaW1lcklkID0gbnVsbDtcblxudmFyIGF1dGggPSB7XG5cbiAgc2lnblVwKG5hbWUsIHBhc3N3b3JkLCB0b2tlbiwgaW52aXRlVG9rZW4pe1xuICAgIHZhciBkYXRhID0ge3VzZXI6IG5hbWUsIHBhc3M6IHBhc3N3b3JkLCBzZWNvbmRfZmFjdG9yX3Rva2VuOiB0b2tlbiwgaW52aXRlX3Rva2VuOiBpbnZpdGVUb2tlbn07XG4gICAgcmV0dXJuIGFwaS5wb3N0KGNmZy5hcGkuY3JlYXRlVXNlclBhdGgsIGRhdGEpXG4gICAgICAudGhlbigodXNlcik9PntcbiAgICAgICAgc2Vzc2lvbi5zZXRVc2VyRGF0YSh1c2VyKTtcbiAgICAgICAgYXV0aC5fc3RhcnRUb2tlblJlZnJlc2hlcigpO1xuICAgICAgICByZXR1cm4gdXNlcjtcbiAgICAgIH0pO1xuICB9LFxuXG4gIGxvZ2luKG5hbWUsIHBhc3N3b3JkLCB0b2tlbil7XG4gICAgYXV0aC5fc3RvcFRva2VuUmVmcmVzaGVyKCk7XG4gICAgc2Vzc2lvbi5jbGVhcigpO1xuICAgIHJldHVybiBhdXRoLl9sb2dpbihuYW1lLCBwYXNzd29yZCwgdG9rZW4pLmRvbmUoYXV0aC5fc3RhcnRUb2tlblJlZnJlc2hlcik7XG4gIH0sXG5cbiAgZW5zdXJlVXNlcigpe1xuICAgIHZhciB1c2VyRGF0YSA9IHNlc3Npb24uZ2V0VXNlckRhdGEoKTtcbiAgICBpZih1c2VyRGF0YS50b2tlbil7XG4gICAgICAvLyByZWZyZXNoIHRpbWVyIHdpbGwgbm90IGJlIHNldCBpbiBjYXNlIG9mIGJyb3dzZXIgcmVmcmVzaCBldmVudFxuICAgICAgaWYoYXV0aC5fZ2V0UmVmcmVzaFRva2VuVGltZXJJZCgpID09PSBudWxsKXtcbiAgICAgICAgcmV0dXJuIGF1dGguX3JlZnJlc2hUb2tlbigpLmRvbmUoYXV0aC5fc3RhcnRUb2tlblJlZnJlc2hlcik7XG4gICAgICB9XG5cbiAgICAgIHJldHVybiAkLkRlZmVycmVkKCkucmVzb2x2ZSh1c2VyRGF0YSk7XG4gICAgfVxuXG4gICAgcmV0dXJuICQuRGVmZXJyZWQoKS5yZWplY3QoKTtcbiAgfSxcblxuICBsb2dvdXQoKXtcbiAgICBhdXRoLl9zdG9wVG9rZW5SZWZyZXNoZXIoKTtcbiAgICBzZXNzaW9uLmNsZWFyKCk7XG4gICAgYXV0aC5fcmVkaXJlY3QoKTtcbiAgfSxcblxuICBfcmVkaXJlY3QoKXtcbiAgICB3aW5kb3cubG9jYXRpb24gPSBjZmcucm91dGVzLmxvZ2luO1xuICB9LFxuXG4gIF9zdGFydFRva2VuUmVmcmVzaGVyKCl7XG4gICAgcmVmcmVzaFRva2VuVGltZXJJZCA9IHNldEludGVydmFsKGF1dGguX3JlZnJlc2hUb2tlbiwgcmVmcmVzaFJhdGUpO1xuICB9LFxuXG4gIF9zdG9wVG9rZW5SZWZyZXNoZXIoKXtcbiAgICBjbGVhckludGVydmFsKHJlZnJlc2hUb2tlblRpbWVySWQpO1xuICAgIHJlZnJlc2hUb2tlblRpbWVySWQgPSBudWxsO1xuICB9LFxuXG4gIF9nZXRSZWZyZXNoVG9rZW5UaW1lcklkKCl7XG4gICAgcmV0dXJuIHJlZnJlc2hUb2tlblRpbWVySWQ7XG4gIH0sXG5cbiAgX3JlZnJlc2hUb2tlbigpe1xuICAgIHJldHVybiBhcGkucG9zdChjZmcuYXBpLnJlbmV3VG9rZW5QYXRoKS50aGVuKGRhdGE9PntcbiAgICAgIHNlc3Npb24uc2V0VXNlckRhdGEoZGF0YSk7XG4gICAgICByZXR1cm4gZGF0YTtcbiAgICB9KS5mYWlsKCgpPT57XG4gICAgICBhdXRoLmxvZ291dCgpO1xuICAgIH0pO1xuICB9LFxuXG4gIF9sb2dpbihuYW1lLCBwYXNzd29yZCwgdG9rZW4pe1xuICAgIHZhciBkYXRhID0ge1xuICAgICAgdXNlcjogbmFtZSxcbiAgICAgIHBhc3M6IHBhc3N3b3JkLFxuICAgICAgc2Vjb25kX2ZhY3Rvcl90b2tlbjogdG9rZW5cbiAgICB9O1xuXG4gICAgcmV0dXJuIGFwaS5wb3N0KGNmZy5hcGkuc2Vzc2lvblBhdGgsIGRhdGEsIGZhbHNlKS50aGVuKGRhdGE9PntcbiAgICAgIHNlc3Npb24uc2V0VXNlckRhdGEoZGF0YSk7XG4gICAgICByZXR1cm4gZGF0YTtcbiAgICB9KTtcbiAgfVxufVxuXG5tb2R1bGUuZXhwb3J0cyA9IGF1dGg7XG5tb2R1bGUuZXhwb3J0cy5QUk9WSURFUl9HT09HTEUgPSBQUk9WSURFUl9HT09HTEU7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvc2VydmljZXMvYXV0aC5qc1xuICoqLyIsIi8vIENvcHlyaWdodCBKb3llbnQsIEluYy4gYW5kIG90aGVyIE5vZGUgY29udHJpYnV0b3JzLlxuLy9cbi8vIFBlcm1pc3Npb24gaXMgaGVyZWJ5IGdyYW50ZWQsIGZyZWUgb2YgY2hhcmdlLCB0byBhbnkgcGVyc29uIG9idGFpbmluZyBhXG4vLyBjb3B5IG9mIHRoaXMgc29mdHdhcmUgYW5kIGFzc29jaWF0ZWQgZG9jdW1lbnRhdGlvbiBmaWxlcyAodGhlXG4vLyBcIlNvZnR3YXJlXCIpLCB0byBkZWFsIGluIHRoZSBTb2Z0d2FyZSB3aXRob3V0IHJlc3RyaWN0aW9uLCBpbmNsdWRpbmdcbi8vIHdpdGhvdXQgbGltaXRhdGlvbiB0aGUgcmlnaHRzIHRvIHVzZSwgY29weSwgbW9kaWZ5LCBtZXJnZSwgcHVibGlzaCxcbi8vIGRpc3RyaWJ1dGUsIHN1YmxpY2Vuc2UsIGFuZC9vciBzZWxsIGNvcGllcyBvZiB0aGUgU29mdHdhcmUsIGFuZCB0byBwZXJtaXRcbi8vIHBlcnNvbnMgdG8gd2hvbSB0aGUgU29mdHdhcmUgaXMgZnVybmlzaGVkIHRvIGRvIHNvLCBzdWJqZWN0IHRvIHRoZVxuLy8gZm9sbG93aW5nIGNvbmRpdGlvbnM6XG4vL1xuLy8gVGhlIGFib3ZlIGNvcHlyaWdodCBub3RpY2UgYW5kIHRoaXMgcGVybWlzc2lvbiBub3RpY2Ugc2hhbGwgYmUgaW5jbHVkZWRcbi8vIGluIGFsbCBjb3BpZXMgb3Igc3Vic3RhbnRpYWwgcG9ydGlvbnMgb2YgdGhlIFNvZnR3YXJlLlxuLy9cbi8vIFRIRSBTT0ZUV0FSRSBJUyBQUk9WSURFRCBcIkFTIElTXCIsIFdJVEhPVVQgV0FSUkFOVFkgT0YgQU5ZIEtJTkQsIEVYUFJFU1Ncbi8vIE9SIElNUExJRUQsIElOQ0xVRElORyBCVVQgTk9UIExJTUlURUQgVE8gVEhFIFdBUlJBTlRJRVMgT0Zcbi8vIE1FUkNIQU5UQUJJTElUWSwgRklUTkVTUyBGT1IgQSBQQVJUSUNVTEFSIFBVUlBPU0UgQU5EIE5PTklORlJJTkdFTUVOVC4gSU5cbi8vIE5PIEVWRU5UIFNIQUxMIFRIRSBBVVRIT1JTIE9SIENPUFlSSUdIVCBIT0xERVJTIEJFIExJQUJMRSBGT1IgQU5ZIENMQUlNLFxuLy8gREFNQUdFUyBPUiBPVEhFUiBMSUFCSUxJVFksIFdIRVRIRVIgSU4gQU4gQUNUSU9OIE9GIENPTlRSQUNULCBUT1JUIE9SXG4vLyBPVEhFUldJU0UsIEFSSVNJTkcgRlJPTSwgT1VUIE9GIE9SIElOIENPTk5FQ1RJT04gV0lUSCBUSEUgU09GVFdBUkUgT1IgVEhFXG4vLyBVU0UgT1IgT1RIRVIgREVBTElOR1MgSU4gVEhFIFNPRlRXQVJFLlxuXG5mdW5jdGlvbiBFdmVudEVtaXR0ZXIoKSB7XG4gIHRoaXMuX2V2ZW50cyA9IHRoaXMuX2V2ZW50cyB8fCB7fTtcbiAgdGhpcy5fbWF4TGlzdGVuZXJzID0gdGhpcy5fbWF4TGlzdGVuZXJzIHx8IHVuZGVmaW5lZDtcbn1cbm1vZHVsZS5leHBvcnRzID0gRXZlbnRFbWl0dGVyO1xuXG4vLyBCYWNrd2FyZHMtY29tcGF0IHdpdGggbm9kZSAwLjEwLnhcbkV2ZW50RW1pdHRlci5FdmVudEVtaXR0ZXIgPSBFdmVudEVtaXR0ZXI7XG5cbkV2ZW50RW1pdHRlci5wcm90b3R5cGUuX2V2ZW50cyA9IHVuZGVmaW5lZDtcbkV2ZW50RW1pdHRlci5wcm90b3R5cGUuX21heExpc3RlbmVycyA9IHVuZGVmaW5lZDtcblxuLy8gQnkgZGVmYXVsdCBFdmVudEVtaXR0ZXJzIHdpbGwgcHJpbnQgYSB3YXJuaW5nIGlmIG1vcmUgdGhhbiAxMCBsaXN0ZW5lcnMgYXJlXG4vLyBhZGRlZCB0byBpdC4gVGhpcyBpcyBhIHVzZWZ1bCBkZWZhdWx0IHdoaWNoIGhlbHBzIGZpbmRpbmcgbWVtb3J5IGxlYWtzLlxuRXZlbnRFbWl0dGVyLmRlZmF1bHRNYXhMaXN0ZW5lcnMgPSAxMDtcblxuLy8gT2J2aW91c2x5IG5vdCBhbGwgRW1pdHRlcnMgc2hvdWxkIGJlIGxpbWl0ZWQgdG8gMTAuIFRoaXMgZnVuY3Rpb24gYWxsb3dzXG4vLyB0aGF0IHRvIGJlIGluY3JlYXNlZC4gU2V0IHRvIHplcm8gZm9yIHVubGltaXRlZC5cbkV2ZW50RW1pdHRlci5wcm90b3R5cGUuc2V0TWF4TGlzdGVuZXJzID0gZnVuY3Rpb24obikge1xuICBpZiAoIWlzTnVtYmVyKG4pIHx8IG4gPCAwIHx8IGlzTmFOKG4pKVxuICAgIHRocm93IFR5cGVFcnJvcignbiBtdXN0IGJlIGEgcG9zaXRpdmUgbnVtYmVyJyk7XG4gIHRoaXMuX21heExpc3RlbmVycyA9IG47XG4gIHJldHVybiB0aGlzO1xufTtcblxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5lbWl0ID0gZnVuY3Rpb24odHlwZSkge1xuICB2YXIgZXIsIGhhbmRsZXIsIGxlbiwgYXJncywgaSwgbGlzdGVuZXJzO1xuXG4gIGlmICghdGhpcy5fZXZlbnRzKVxuICAgIHRoaXMuX2V2ZW50cyA9IHt9O1xuXG4gIC8vIElmIHRoZXJlIGlzIG5vICdlcnJvcicgZXZlbnQgbGlzdGVuZXIgdGhlbiB0aHJvdy5cbiAgaWYgKHR5cGUgPT09ICdlcnJvcicpIHtcbiAgICBpZiAoIXRoaXMuX2V2ZW50cy5lcnJvciB8fFxuICAgICAgICAoaXNPYmplY3QodGhpcy5fZXZlbnRzLmVycm9yKSAmJiAhdGhpcy5fZXZlbnRzLmVycm9yLmxlbmd0aCkpIHtcbiAgICAgIGVyID0gYXJndW1lbnRzWzFdO1xuICAgICAgaWYgKGVyIGluc3RhbmNlb2YgRXJyb3IpIHtcbiAgICAgICAgdGhyb3cgZXI7IC8vIFVuaGFuZGxlZCAnZXJyb3InIGV2ZW50XG4gICAgICB9XG4gICAgICB0aHJvdyBUeXBlRXJyb3IoJ1VuY2F1Z2h0LCB1bnNwZWNpZmllZCBcImVycm9yXCIgZXZlbnQuJyk7XG4gICAgfVxuICB9XG5cbiAgaGFuZGxlciA9IHRoaXMuX2V2ZW50c1t0eXBlXTtcblxuICBpZiAoaXNVbmRlZmluZWQoaGFuZGxlcikpXG4gICAgcmV0dXJuIGZhbHNlO1xuXG4gIGlmIChpc0Z1bmN0aW9uKGhhbmRsZXIpKSB7XG4gICAgc3dpdGNoIChhcmd1bWVudHMubGVuZ3RoKSB7XG4gICAgICAvLyBmYXN0IGNhc2VzXG4gICAgICBjYXNlIDE6XG4gICAgICAgIGhhbmRsZXIuY2FsbCh0aGlzKTtcbiAgICAgICAgYnJlYWs7XG4gICAgICBjYXNlIDI6XG4gICAgICAgIGhhbmRsZXIuY2FsbCh0aGlzLCBhcmd1bWVudHNbMV0pO1xuICAgICAgICBicmVhaztcbiAgICAgIGNhc2UgMzpcbiAgICAgICAgaGFuZGxlci5jYWxsKHRoaXMsIGFyZ3VtZW50c1sxXSwgYXJndW1lbnRzWzJdKTtcbiAgICAgICAgYnJlYWs7XG4gICAgICAvLyBzbG93ZXJcbiAgICAgIGRlZmF1bHQ6XG4gICAgICAgIGxlbiA9IGFyZ3VtZW50cy5sZW5ndGg7XG4gICAgICAgIGFyZ3MgPSBuZXcgQXJyYXkobGVuIC0gMSk7XG4gICAgICAgIGZvciAoaSA9IDE7IGkgPCBsZW47IGkrKylcbiAgICAgICAgICBhcmdzW2kgLSAxXSA9IGFyZ3VtZW50c1tpXTtcbiAgICAgICAgaGFuZGxlci5hcHBseSh0aGlzLCBhcmdzKTtcbiAgICB9XG4gIH0gZWxzZSBpZiAoaXNPYmplY3QoaGFuZGxlcikpIHtcbiAgICBsZW4gPSBhcmd1bWVudHMubGVuZ3RoO1xuICAgIGFyZ3MgPSBuZXcgQXJyYXkobGVuIC0gMSk7XG4gICAgZm9yIChpID0gMTsgaSA8IGxlbjsgaSsrKVxuICAgICAgYXJnc1tpIC0gMV0gPSBhcmd1bWVudHNbaV07XG5cbiAgICBsaXN0ZW5lcnMgPSBoYW5kbGVyLnNsaWNlKCk7XG4gICAgbGVuID0gbGlzdGVuZXJzLmxlbmd0aDtcbiAgICBmb3IgKGkgPSAwOyBpIDwgbGVuOyBpKyspXG4gICAgICBsaXN0ZW5lcnNbaV0uYXBwbHkodGhpcywgYXJncyk7XG4gIH1cblxuICByZXR1cm4gdHJ1ZTtcbn07XG5cbkV2ZW50RW1pdHRlci5wcm90b3R5cGUuYWRkTGlzdGVuZXIgPSBmdW5jdGlvbih0eXBlLCBsaXN0ZW5lcikge1xuICB2YXIgbTtcblxuICBpZiAoIWlzRnVuY3Rpb24obGlzdGVuZXIpKVxuICAgIHRocm93IFR5cGVFcnJvcignbGlzdGVuZXIgbXVzdCBiZSBhIGZ1bmN0aW9uJyk7XG5cbiAgaWYgKCF0aGlzLl9ldmVudHMpXG4gICAgdGhpcy5fZXZlbnRzID0ge307XG5cbiAgLy8gVG8gYXZvaWQgcmVjdXJzaW9uIGluIHRoZSBjYXNlIHRoYXQgdHlwZSA9PT0gXCJuZXdMaXN0ZW5lclwiISBCZWZvcmVcbiAgLy8gYWRkaW5nIGl0IHRvIHRoZSBsaXN0ZW5lcnMsIGZpcnN0IGVtaXQgXCJuZXdMaXN0ZW5lclwiLlxuICBpZiAodGhpcy5fZXZlbnRzLm5ld0xpc3RlbmVyKVxuICAgIHRoaXMuZW1pdCgnbmV3TGlzdGVuZXInLCB0eXBlLFxuICAgICAgICAgICAgICBpc0Z1bmN0aW9uKGxpc3RlbmVyLmxpc3RlbmVyKSA/XG4gICAgICAgICAgICAgIGxpc3RlbmVyLmxpc3RlbmVyIDogbGlzdGVuZXIpO1xuXG4gIGlmICghdGhpcy5fZXZlbnRzW3R5cGVdKVxuICAgIC8vIE9wdGltaXplIHRoZSBjYXNlIG9mIG9uZSBsaXN0ZW5lci4gRG9uJ3QgbmVlZCB0aGUgZXh0cmEgYXJyYXkgb2JqZWN0LlxuICAgIHRoaXMuX2V2ZW50c1t0eXBlXSA9IGxpc3RlbmVyO1xuICBlbHNlIGlmIChpc09iamVjdCh0aGlzLl9ldmVudHNbdHlwZV0pKVxuICAgIC8vIElmIHdlJ3ZlIGFscmVhZHkgZ290IGFuIGFycmF5LCBqdXN0IGFwcGVuZC5cbiAgICB0aGlzLl9ldmVudHNbdHlwZV0ucHVzaChsaXN0ZW5lcik7XG4gIGVsc2VcbiAgICAvLyBBZGRpbmcgdGhlIHNlY29uZCBlbGVtZW50LCBuZWVkIHRvIGNoYW5nZSB0byBhcnJheS5cbiAgICB0aGlzLl9ldmVudHNbdHlwZV0gPSBbdGhpcy5fZXZlbnRzW3R5cGVdLCBsaXN0ZW5lcl07XG5cbiAgLy8gQ2hlY2sgZm9yIGxpc3RlbmVyIGxlYWtcbiAgaWYgKGlzT2JqZWN0KHRoaXMuX2V2ZW50c1t0eXBlXSkgJiYgIXRoaXMuX2V2ZW50c1t0eXBlXS53YXJuZWQpIHtcbiAgICB2YXIgbTtcbiAgICBpZiAoIWlzVW5kZWZpbmVkKHRoaXMuX21heExpc3RlbmVycykpIHtcbiAgICAgIG0gPSB0aGlzLl9tYXhMaXN0ZW5lcnM7XG4gICAgfSBlbHNlIHtcbiAgICAgIG0gPSBFdmVudEVtaXR0ZXIuZGVmYXVsdE1heExpc3RlbmVycztcbiAgICB9XG5cbiAgICBpZiAobSAmJiBtID4gMCAmJiB0aGlzLl9ldmVudHNbdHlwZV0ubGVuZ3RoID4gbSkge1xuICAgICAgdGhpcy5fZXZlbnRzW3R5cGVdLndhcm5lZCA9IHRydWU7XG4gICAgICBjb25zb2xlLmVycm9yKCcobm9kZSkgd2FybmluZzogcG9zc2libGUgRXZlbnRFbWl0dGVyIG1lbW9yeSAnICtcbiAgICAgICAgICAgICAgICAgICAgJ2xlYWsgZGV0ZWN0ZWQuICVkIGxpc3RlbmVycyBhZGRlZC4gJyArXG4gICAgICAgICAgICAgICAgICAgICdVc2UgZW1pdHRlci5zZXRNYXhMaXN0ZW5lcnMoKSB0byBpbmNyZWFzZSBsaW1pdC4nLFxuICAgICAgICAgICAgICAgICAgICB0aGlzLl9ldmVudHNbdHlwZV0ubGVuZ3RoKTtcbiAgICAgIGlmICh0eXBlb2YgY29uc29sZS50cmFjZSA9PT0gJ2Z1bmN0aW9uJykge1xuICAgICAgICAvLyBub3Qgc3VwcG9ydGVkIGluIElFIDEwXG4gICAgICAgIGNvbnNvbGUudHJhY2UoKTtcbiAgICAgIH1cbiAgICB9XG4gIH1cblxuICByZXR1cm4gdGhpcztcbn07XG5cbkV2ZW50RW1pdHRlci5wcm90b3R5cGUub24gPSBFdmVudEVtaXR0ZXIucHJvdG90eXBlLmFkZExpc3RlbmVyO1xuXG5FdmVudEVtaXR0ZXIucHJvdG90eXBlLm9uY2UgPSBmdW5jdGlvbih0eXBlLCBsaXN0ZW5lcikge1xuICBpZiAoIWlzRnVuY3Rpb24obGlzdGVuZXIpKVxuICAgIHRocm93IFR5cGVFcnJvcignbGlzdGVuZXIgbXVzdCBiZSBhIGZ1bmN0aW9uJyk7XG5cbiAgdmFyIGZpcmVkID0gZmFsc2U7XG5cbiAgZnVuY3Rpb24gZygpIHtcbiAgICB0aGlzLnJlbW92ZUxpc3RlbmVyKHR5cGUsIGcpO1xuXG4gICAgaWYgKCFmaXJlZCkge1xuICAgICAgZmlyZWQgPSB0cnVlO1xuICAgICAgbGlzdGVuZXIuYXBwbHkodGhpcywgYXJndW1lbnRzKTtcbiAgICB9XG4gIH1cblxuICBnLmxpc3RlbmVyID0gbGlzdGVuZXI7XG4gIHRoaXMub24odHlwZSwgZyk7XG5cbiAgcmV0dXJuIHRoaXM7XG59O1xuXG4vLyBlbWl0cyBhICdyZW1vdmVMaXN0ZW5lcicgZXZlbnQgaWZmIHRoZSBsaXN0ZW5lciB3YXMgcmVtb3ZlZFxuRXZlbnRFbWl0dGVyLnByb3RvdHlwZS5yZW1vdmVMaXN0ZW5lciA9IGZ1bmN0aW9uKHR5cGUsIGxpc3RlbmVyKSB7XG4gIHZhciBsaXN0LCBwb3NpdGlvbiwgbGVuZ3RoLCBpO1xuXG4gIGlmICghaXNGdW5jdGlvbihsaXN0ZW5lcikpXG4gICAgdGhyb3cgVHlwZUVycm9yKCdsaXN0ZW5lciBtdXN0IGJlIGEgZnVuY3Rpb24nKTtcblxuICBpZiAoIXRoaXMuX2V2ZW50cyB8fCAhdGhpcy5fZXZlbnRzW3R5cGVdKVxuICAgIHJldHVybiB0aGlzO1xuXG4gIGxpc3QgPSB0aGlzLl9ldmVudHNbdHlwZV07XG4gIGxlbmd0aCA9IGxpc3QubGVuZ3RoO1xuICBwb3NpdGlvbiA9IC0xO1xuXG4gIGlmIChsaXN0ID09PSBsaXN0ZW5lciB8fFxuICAgICAgKGlzRnVuY3Rpb24obGlzdC5saXN0ZW5lcikgJiYgbGlzdC5saXN0ZW5lciA9PT0gbGlzdGVuZXIpKSB7XG4gICAgZGVsZXRlIHRoaXMuX2V2ZW50c1t0eXBlXTtcbiAgICBpZiAodGhpcy5fZXZlbnRzLnJlbW92ZUxpc3RlbmVyKVxuICAgICAgdGhpcy5lbWl0KCdyZW1vdmVMaXN0ZW5lcicsIHR5cGUsIGxpc3RlbmVyKTtcblxuICB9IGVsc2UgaWYgKGlzT2JqZWN0KGxpc3QpKSB7XG4gICAgZm9yIChpID0gbGVuZ3RoOyBpLS0gPiAwOykge1xuICAgICAgaWYgKGxpc3RbaV0gPT09IGxpc3RlbmVyIHx8XG4gICAgICAgICAgKGxpc3RbaV0ubGlzdGVuZXIgJiYgbGlzdFtpXS5saXN0ZW5lciA9PT0gbGlzdGVuZXIpKSB7XG4gICAgICAgIHBvc2l0aW9uID0gaTtcbiAgICAgICAgYnJlYWs7XG4gICAgICB9XG4gICAgfVxuXG4gICAgaWYgKHBvc2l0aW9uIDwgMClcbiAgICAgIHJldHVybiB0aGlzO1xuXG4gICAgaWYgKGxpc3QubGVuZ3RoID09PSAxKSB7XG4gICAgICBsaXN0Lmxlbmd0aCA9IDA7XG4gICAgICBkZWxldGUgdGhpcy5fZXZlbnRzW3R5cGVdO1xuICAgIH0gZWxzZSB7XG4gICAgICBsaXN0LnNwbGljZShwb3NpdGlvbiwgMSk7XG4gICAgfVxuXG4gICAgaWYgKHRoaXMuX2V2ZW50cy5yZW1vdmVMaXN0ZW5lcilcbiAgICAgIHRoaXMuZW1pdCgncmVtb3ZlTGlzdGVuZXInLCB0eXBlLCBsaXN0ZW5lcik7XG4gIH1cblxuICByZXR1cm4gdGhpcztcbn07XG5cbkV2ZW50RW1pdHRlci5wcm90b3R5cGUucmVtb3ZlQWxsTGlzdGVuZXJzID0gZnVuY3Rpb24odHlwZSkge1xuICB2YXIga2V5LCBsaXN0ZW5lcnM7XG5cbiAgaWYgKCF0aGlzLl9ldmVudHMpXG4gICAgcmV0dXJuIHRoaXM7XG5cbiAgLy8gbm90IGxpc3RlbmluZyBmb3IgcmVtb3ZlTGlzdGVuZXIsIG5vIG5lZWQgdG8gZW1pdFxuICBpZiAoIXRoaXMuX2V2ZW50cy5yZW1vdmVMaXN0ZW5lcikge1xuICAgIGlmIChhcmd1bWVudHMubGVuZ3RoID09PSAwKVxuICAgICAgdGhpcy5fZXZlbnRzID0ge307XG4gICAgZWxzZSBpZiAodGhpcy5fZXZlbnRzW3R5cGVdKVxuICAgICAgZGVsZXRlIHRoaXMuX2V2ZW50c1t0eXBlXTtcbiAgICByZXR1cm4gdGhpcztcbiAgfVxuXG4gIC8vIGVtaXQgcmVtb3ZlTGlzdGVuZXIgZm9yIGFsbCBsaXN0ZW5lcnMgb24gYWxsIGV2ZW50c1xuICBpZiAoYXJndW1lbnRzLmxlbmd0aCA9PT0gMCkge1xuICAgIGZvciAoa2V5IGluIHRoaXMuX2V2ZW50cykge1xuICAgICAgaWYgKGtleSA9PT0gJ3JlbW92ZUxpc3RlbmVyJykgY29udGludWU7XG4gICAgICB0aGlzLnJlbW92ZUFsbExpc3RlbmVycyhrZXkpO1xuICAgIH1cbiAgICB0aGlzLnJlbW92ZUFsbExpc3RlbmVycygncmVtb3ZlTGlzdGVuZXInKTtcbiAgICB0aGlzLl9ldmVudHMgPSB7fTtcbiAgICByZXR1cm4gdGhpcztcbiAgfVxuXG4gIGxpc3RlbmVycyA9IHRoaXMuX2V2ZW50c1t0eXBlXTtcblxuICBpZiAoaXNGdW5jdGlvbihsaXN0ZW5lcnMpKSB7XG4gICAgdGhpcy5yZW1vdmVMaXN0ZW5lcih0eXBlLCBsaXN0ZW5lcnMpO1xuICB9IGVsc2Uge1xuICAgIC8vIExJRk8gb3JkZXJcbiAgICB3aGlsZSAobGlzdGVuZXJzLmxlbmd0aClcbiAgICAgIHRoaXMucmVtb3ZlTGlzdGVuZXIodHlwZSwgbGlzdGVuZXJzW2xpc3RlbmVycy5sZW5ndGggLSAxXSk7XG4gIH1cbiAgZGVsZXRlIHRoaXMuX2V2ZW50c1t0eXBlXTtcblxuICByZXR1cm4gdGhpcztcbn07XG5cbkV2ZW50RW1pdHRlci5wcm90b3R5cGUubGlzdGVuZXJzID0gZnVuY3Rpb24odHlwZSkge1xuICB2YXIgcmV0O1xuICBpZiAoIXRoaXMuX2V2ZW50cyB8fCAhdGhpcy5fZXZlbnRzW3R5cGVdKVxuICAgIHJldCA9IFtdO1xuICBlbHNlIGlmIChpc0Z1bmN0aW9uKHRoaXMuX2V2ZW50c1t0eXBlXSkpXG4gICAgcmV0ID0gW3RoaXMuX2V2ZW50c1t0eXBlXV07XG4gIGVsc2VcbiAgICByZXQgPSB0aGlzLl9ldmVudHNbdHlwZV0uc2xpY2UoKTtcbiAgcmV0dXJuIHJldDtcbn07XG5cbkV2ZW50RW1pdHRlci5saXN0ZW5lckNvdW50ID0gZnVuY3Rpb24oZW1pdHRlciwgdHlwZSkge1xuICB2YXIgcmV0O1xuICBpZiAoIWVtaXR0ZXIuX2V2ZW50cyB8fCAhZW1pdHRlci5fZXZlbnRzW3R5cGVdKVxuICAgIHJldCA9IDA7XG4gIGVsc2UgaWYgKGlzRnVuY3Rpb24oZW1pdHRlci5fZXZlbnRzW3R5cGVdKSlcbiAgICByZXQgPSAxO1xuICBlbHNlXG4gICAgcmV0ID0gZW1pdHRlci5fZXZlbnRzW3R5cGVdLmxlbmd0aDtcbiAgcmV0dXJuIHJldDtcbn07XG5cbmZ1bmN0aW9uIGlzRnVuY3Rpb24oYXJnKSB7XG4gIHJldHVybiB0eXBlb2YgYXJnID09PSAnZnVuY3Rpb24nO1xufVxuXG5mdW5jdGlvbiBpc051bWJlcihhcmcpIHtcbiAgcmV0dXJuIHR5cGVvZiBhcmcgPT09ICdudW1iZXInO1xufVxuXG5mdW5jdGlvbiBpc09iamVjdChhcmcpIHtcbiAgcmV0dXJuIHR5cGVvZiBhcmcgPT09ICdvYmplY3QnICYmIGFyZyAhPT0gbnVsbDtcbn1cblxuZnVuY3Rpb24gaXNVbmRlZmluZWQoYXJnKSB7XG4gIHJldHVybiBhcmcgPT09IHZvaWQgMDtcbn1cblxuXG5cbi8qKioqKioqKioqKioqKioqKlxuICoqIFdFQlBBQ0sgRk9PVEVSXG4gKiogLi9+L2V2ZW50cy9ldmVudHMuanNcbiAqKiBtb2R1bGUgaWQgPSA5OVxuICoqIG1vZHVsZSBjaHVua3MgPSAxXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG5tb2R1bGUuZXhwb3J0cy5pc01hdGNoID0gZnVuY3Rpb24ob2JqLCBzZWFyY2hWYWx1ZSwge3NlYXJjaGFibGVQcm9wcywgY2J9KSB7XG4gIHNlYXJjaFZhbHVlID0gc2VhcmNoVmFsdWUudG9Mb2NhbGVVcHBlckNhc2UoKTtcbiAgbGV0IHByb3BOYW1lcyA9IHNlYXJjaGFibGVQcm9wcyB8fCBPYmplY3QuZ2V0T3duUHJvcGVydHlOYW1lcyhvYmopO1xuICBmb3IgKGxldCBpID0gMDsgaSA8IHByb3BOYW1lcy5sZW5ndGg7IGkrKykge1xuICAgIGxldCB0YXJnZXRWYWx1ZSA9IG9ialtwcm9wTmFtZXNbaV1dO1xuICAgIGlmICh0YXJnZXRWYWx1ZSkge1xuICAgICAgaWYodHlwZW9mIGNiID09PSAnZnVuY3Rpb24nKXtcbiAgICAgICAgbGV0IHJlc3VsdCA9IGNiKHRhcmdldFZhbHVlLCBzZWFyY2hWYWx1ZSwgcHJvcE5hbWVzW2ldKTtcbiAgICAgICAgaWYocmVzdWx0ID09PSB0cnVlKXtcbiAgICAgICAgICByZXR1cm4gcmVzdWx0O1xuICAgICAgICB9XG4gICAgICB9XG5cbiAgICAgIGlmICh0YXJnZXRWYWx1ZS50b1N0cmluZygpLnRvTG9jYWxlVXBwZXJDYXNlKCkuaW5kZXhPZihzZWFyY2hWYWx1ZSkgIT09IC0xKSB7XG4gICAgICAgIHJldHVybiB0cnVlO1xuICAgICAgfVxuICAgIH1cbiAgfVxuXG4gIHJldHVybiBmYWxzZTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21tb24vb2JqZWN0VXRpbHMuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciBUZXJtID0gcmVxdWlyZSgnVGVybWluYWwnKTtcbnZhciBUdHkgPSByZXF1aXJlKCcuL3R0eScpO1xudmFyIFR0eUV2ZW50cyA9IHJlcXVpcmUoJy4vdHR5RXZlbnRzJyk7XG52YXIge2RlYm91bmNlLCBpc051bWJlcn0gPSByZXF1aXJlKCdfJyk7XG5cbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG52YXIgYXBpID0gcmVxdWlyZSgnYXBwL3NlcnZpY2VzL2FwaScpO1xudmFyIGxvZ2dlciA9IHJlcXVpcmUoJ2FwcC9jb21tb24vbG9nZ2VyJykuY3JlYXRlKCd0ZXJtaW5hbCcpO1xudmFyICQgPSByZXF1aXJlKCdqUXVlcnknKTtcblxuVGVybS5jb2xvcnNbMjU2XSA9ICcjMjUyMzIzJztcblxuY29uc3QgRElTQ09OTkVDVF9UWFQgPSAnXFx4MWJbMzFtZGlzY29ubmVjdGVkXFx4MWJbbVxcclxcbic7XG5jb25zdCBDT05ORUNURURfVFhUID0gJ0Nvbm5lY3RlZCFcXHJcXG4nO1xuY29uc3QgR1JWX0NMQVNTID0gJ2dydi10ZXJtaW5hbCc7XG5cbmNsYXNzIFR0eVRlcm1pbmFsIHtcbiAgY29uc3RydWN0b3Iob3B0aW9ucyl7XG4gICAgbGV0IHtcbiAgICAgIHR0eSxcbiAgICAgIGNvbHMsXG4gICAgICByb3dzLFxuICAgICAgc2Nyb2xsQmFjayA9IDEwMDAgfSA9IG9wdGlvbnM7XG5cbiAgICB0aGlzLnR0eVBhcmFtcyA9IHR0eTtcbiAgICB0aGlzLnR0eSA9IG5ldyBUdHkoKTtcbiAgICB0aGlzLnR0eUV2ZW50cyA9IG5ldyBUdHlFdmVudHMoKTtcblxuICAgIHRoaXMuc2Nyb2xsQmFjayA9IHNjcm9sbEJhY2tcbiAgICB0aGlzLnJvd3MgPSByb3dzO1xuICAgIHRoaXMuY29scyA9IGNvbHM7XG4gICAgdGhpcy50ZXJtID0gbnVsbDtcbiAgICB0aGlzLl9lbCA9IG9wdGlvbnMuZWw7XG5cbiAgICB0aGlzLmRlYm91bmNlZFJlc2l6ZSA9IGRlYm91bmNlKHRoaXMuX3JlcXVlc3RSZXNpemUuYmluZCh0aGlzKSwgMjAwKTtcbiAgfVxuXG4gIG9wZW4oKSB7XG4gICAgJCh0aGlzLl9lbCkuYWRkQ2xhc3MoR1JWX0NMQVNTKTtcblxuICAgIHRoaXMudGVybSA9IG5ldyBUZXJtKHtcbiAgICAgIGNvbHM6IDE1LFxuICAgICAgcm93czogNSxcbiAgICAgIHNjcm9sbGJhY2s6IHRoaXMuc2Nyb2xsYmFjayxcbiAgICAgIHVzZVN0eWxlOiB0cnVlLFxuICAgICAgc2NyZWVuS2V5czogdHJ1ZSxcbiAgICAgIGN1cnNvckJsaW5rOiB0cnVlXG4gICAgfSk7XG5cbiAgICB0aGlzLnRlcm0ub3Blbih0aGlzLl9lbCk7XG5cbiAgICB0aGlzLnJlc2l6ZSh0aGlzLmNvbHMsIHRoaXMucm93cyk7XG5cbiAgICAvLyB0ZXJtIGV2ZW50c1xuICAgIHRoaXMudGVybS5vbignZGF0YScsIChkYXRhKSA9PiB0aGlzLnR0eS5zZW5kKGRhdGEpKTtcblxuICAgIC8vIHR0eVxuICAgIHRoaXMudHR5Lm9uKCdyZXNpemUnLCAoe2gsIHd9KT0+IHRoaXMucmVzaXplKHcsIGgpKTtcbiAgICB0aGlzLnR0eS5vbigncmVzZXQnLCAoKT0+IHRoaXMudGVybS5yZXNldCgpKTtcbiAgICB0aGlzLnR0eS5vbignb3BlbicsICgpPT4gdGhpcy50ZXJtLndyaXRlKENPTk5FQ1RFRF9UWFQpKTtcbiAgICB0aGlzLnR0eS5vbignY2xvc2UnLCAoKT0+IHRoaXMudGVybS53cml0ZShESVNDT05ORUNUX1RYVCkpO1xuICAgIHRoaXMudHR5Lm9uKCdkYXRhJywgKGRhdGEpID0+IHtcbiAgICAgIHRyeXtcbiAgICAgICAgdGhpcy50ZXJtLndyaXRlKGRhdGEpO1xuICAgICAgfWNhdGNoKGVycil7XG4gICAgICAgIGNvbnNvbGUuZXJyb3IoZXJyKTtcbiAgICAgIH1cbiAgICB9KTtcblxuICAgIC8vIHR0eUV2ZW50c1xuICAgIHRoaXMudHR5RXZlbnRzLm9uKCdkYXRhJywgdGhpcy5faGFuZGxlVHR5RXZlbnRzRGF0YS5iaW5kKHRoaXMpKTtcbiAgICB0aGlzLmNvbm5lY3QoKTtcbiAgICB3aW5kb3cuYWRkRXZlbnRMaXN0ZW5lcigncmVzaXplJywgdGhpcy5kZWJvdW5jZWRSZXNpemUpO1xuICB9XG5cbiAgY29ubmVjdCgpe1xuICAgIHRoaXMudHR5LmNvbm5lY3QodGhpcy5fZ2V0VHR5Q29ublN0cigpKTtcbiAgICB0aGlzLnR0eUV2ZW50cy5jb25uZWN0KHRoaXMuX2dldFR0eUV2ZW50c0Nvbm5TdHIoKSk7XG4gIH1cblxuICBkZXN0cm95KCkge1xuICAgIGlmKHRoaXMudHR5ICE9PSBudWxsKXtcbiAgICAgIHRoaXMudHR5LmRpc2Nvbm5lY3QoKTtcbiAgICB9XG5cbiAgICBpZih0aGlzLnR0eUV2ZW50cyAhPT0gbnVsbCl7XG4gICAgICB0aGlzLnR0eUV2ZW50cy5kaXNjb25uZWN0KCk7XG4gICAgICB0aGlzLnR0eUV2ZW50cy5yZW1vdmVBbGxMaXN0ZW5lcnMoKTtcbiAgICB9XG5cbiAgICBpZih0aGlzLnRlcm0gIT09IG51bGwpe1xuICAgICAgdGhpcy50ZXJtLmRlc3Ryb3koKTtcbiAgICAgIHRoaXMudGVybS5yZW1vdmVBbGxMaXN0ZW5lcnMoKTtcbiAgICB9XG5cbiAgICAkKHRoaXMuX2VsKS5lbXB0eSgpLnJlbW92ZUNsYXNzKEdSVl9DTEFTUyk7XG5cbiAgICB3aW5kb3cucmVtb3ZlRXZlbnRMaXN0ZW5lcigncmVzaXplJywgdGhpcy5kZWJvdW5jZWRSZXNpemUpO1xuICB9XG5cbiAgcmVzaXplKGNvbHMsIHJvd3MpIHtcbiAgICAvLyBpZiBub3QgZGVmaW5lZCwgdXNlIHRoZSBzaXplIG9mIHRoZSBjb250YWluZXJcbiAgICBpZighaXNOdW1iZXIoY29scykgfHwgIWlzTnVtYmVyKHJvd3MpKXtcbiAgICAgIGxldCBkaW0gPSB0aGlzLl9nZXREaW1lbnNpb25zKCk7XG4gICAgICBjb2xzID0gZGltLmNvbHM7XG4gICAgICByb3dzID0gZGltLnJvd3M7XG4gICAgfVxuXG4gICAgdGhpcy5jb2xzID0gY29scztcbiAgICB0aGlzLnJvd3MgPSByb3dzO1xuICAgIHRoaXMudGVybS5yZXNpemUodGhpcy5jb2xzLCB0aGlzLnJvd3MpO1xuICB9XG5cbiAgX3JlcXVlc3RSZXNpemUoKXtcbiAgICBsZXQge2NvbHMsIHJvd3N9ID0gdGhpcy5fZ2V0RGltZW5zaW9ucygpO1xuICAgIGxldCB3ID0gY29scztcbiAgICBsZXQgaCA9IHJvd3M7XG5cbiAgICAvLyBzb21lIG1pbiB2YWx1ZXNcbiAgICB3ID0gdyA8IDUgPyA1IDogdztcbiAgICBoID0gaCA8IDUgPyA1IDogaDtcblxuICAgIGxldCB7c2lkIH0gPSB0aGlzLnR0eVBhcmFtcztcbiAgICBsZXQgcmVxRGF0YSA9IHsgdGVybWluYWxfcGFyYW1zOiB7IHcsIGggfSB9O1xuXG4gICAgbG9nZ2VyLmluZm8oJ3Jlc2l6ZScsIGB3OiR7d30gYW5kIGg6JHtofWApO1xuICAgIGFwaS5wdXQoY2ZnLmFwaS5nZXRUZXJtaW5hbFNlc3Npb25Vcmwoc2lkKSwgcmVxRGF0YSlcbiAgICAgIC5kb25lKCgpPT4gbG9nZ2VyLmluZm8oJ3Jlc2l6ZWQnKSlcbiAgICAgIC5mYWlsKChlcnIpPT4gbG9nZ2VyLmVycm9yKCdmYWlsZWQgdG8gcmVzaXplJywgZXJyKSk7XG4gIH1cblxuICBfaGFuZGxlVHR5RXZlbnRzRGF0YShkYXRhKXtcbiAgICBpZihkYXRhICYmIGRhdGEudGVybWluYWxfcGFyYW1zKXtcbiAgICAgIGxldCB7dywgaH0gPSBkYXRhLnRlcm1pbmFsX3BhcmFtcztcbiAgICAgIGlmKGggIT09IHRoaXMucm93cyB8fCB3ICE9PSB0aGlzLmNvbHMpe1xuICAgICAgICB0aGlzLnJlc2l6ZSh3LCBoKTtcbiAgICAgIH1cbiAgICB9XG4gIH1cblxuICBfZ2V0RGltZW5zaW9ucygpe1xuICAgIGxldCAkY29udGFpbmVyID0gJCh0aGlzLl9lbCk7XG4gICAgbGV0IGZha2VSb3cgPSAkKCc8ZGl2PjxzcGFuPiZuYnNwOzwvc3Bhbj48L2Rpdj4nKTtcblxuICAgICRjb250YWluZXIuZmluZCgnLnRlcm1pbmFsJykuYXBwZW5kKGZha2VSb3cpO1xuICAgIC8vIGdldCBkaXYgaGVpZ2h0XG4gICAgbGV0IGZha2VDb2xIZWlnaHQgPSBmYWtlUm93WzBdLmdldEJvdW5kaW5nQ2xpZW50UmVjdCgpLmhlaWdodDtcbiAgICAvLyBnZXQgc3BhbiB3aWR0aFxuICAgIGxldCBmYWtlQ29sV2lkdGggPSBmYWtlUm93LmNoaWxkcmVuKCkuZmlyc3QoKVswXS5nZXRCb3VuZGluZ0NsaWVudFJlY3QoKS53aWR0aDtcblxuICAgIGxldCB3aWR0aCA9ICRjb250YWluZXJbMF0uY2xpZW50V2lkdGg7XG4gICAgbGV0IGhlaWdodCA9ICRjb250YWluZXJbMF0uY2xpZW50SGVpZ2h0O1xuXG4gICAgbGV0IGNvbHMgPSBNYXRoLmZsb29yKHdpZHRoIC8gKGZha2VDb2xXaWR0aCkpO1xuICAgIGxldCByb3dzID0gTWF0aC5mbG9vcihoZWlnaHQgLyAoZmFrZUNvbEhlaWdodCkpO1xuICAgIGZha2VSb3cucmVtb3ZlKCk7XG5cbiAgICByZXR1cm4ge2NvbHMsIHJvd3N9O1xuICB9XG5cbiAgX2dldFR0eUV2ZW50c0Nvbm5TdHIoKXtcbiAgICBsZXQge3NpZCwgdXJsLCB0b2tlbiB9ID0gdGhpcy50dHlQYXJhbXM7XG4gICAgcmV0dXJuIGAke3VybH0vc2Vzc2lvbnMvJHtzaWR9L2V2ZW50cy9zdHJlYW0/YWNjZXNzX3Rva2VuPSR7dG9rZW59YDtcbiAgfVxuXG4gIF9nZXRUdHlDb25uU3RyKCl7XG4gICAgbGV0IHtzZXJ2ZXJJZCwgbG9naW4sIHNpZCwgdXJsLCB0b2tlbiB9ID0gdGhpcy50dHlQYXJhbXM7XG4gICAgdmFyIHBhcmFtcyA9IHtcbiAgICAgIHNlcnZlcl9pZDogc2VydmVySWQsXG4gICAgICBsb2dpbixcbiAgICAgIHNpZCxcbiAgICAgIHRlcm06IHtcbiAgICAgICAgaDogdGhpcy5yb3dzLFxuICAgICAgICB3OiB0aGlzLmNvbHNcbiAgICAgIH1cbiAgICB9XG5cbiAgICB2YXIganNvbiA9IEpTT04uc3RyaW5naWZ5KHBhcmFtcyk7XG4gICAgdmFyIGpzb25FbmNvZGVkID0gd2luZG93LmVuY29kZVVSSShqc29uKTtcblxuICAgIHJldHVybiBgJHt1cmx9L2Nvbm5lY3Q/YWNjZXNzX3Rva2VuPSR7dG9rZW59JnBhcmFtcz0ke2pzb25FbmNvZGVkfWA7XG4gIH1cblxufVxuXG5tb2R1bGUuZXhwb3J0cyA9IFR0eVRlcm1pbmFsO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbW1vbi90ZXJtaW5hbC5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIEV2ZW50RW1pdHRlciA9IHJlcXVpcmUoJ2V2ZW50cycpLkV2ZW50RW1pdHRlcjtcbnZhciBCdWZmZXIgPSByZXF1aXJlKCdidWZmZXIvJykuQnVmZmVyO1xuXG5jbGFzcyBUdHkgZXh0ZW5kcyBFdmVudEVtaXR0ZXIge1xuXG4gIGNvbnN0cnVjdG9yKCl7XG4gICAgc3VwZXIoKTtcbiAgICB0aGlzLnNvY2tldCA9IG51bGw7XG4gIH1cblxuICBkaXNjb25uZWN0KCl7XG4gICAgdGhpcy5zb2NrZXQuY2xvc2UoKTtcbiAgfVxuXG4gIHJlY29ubmVjdChvcHRpb25zKXtcbiAgICB0aGlzLmRpc2Nvbm5lY3QoKTtcbiAgICB0aGlzLnNvY2tldC5vbm9wZW4gPSBudWxsO1xuICAgIHRoaXMuc29ja2V0Lm9ubWVzc2FnZSA9IG51bGw7XG4gICAgdGhpcy5zb2NrZXQub25jbG9zZSA9IG51bGw7XG5cbiAgICB0aGlzLmNvbm5lY3Qob3B0aW9ucyk7XG4gIH1cblxuICBjb25uZWN0KGNvbm5TdHIpe1xuICAgIHRoaXMuc29ja2V0ID0gbmV3IFdlYlNvY2tldChjb25uU3RyLCAncHJvdG8nKTtcblxuICAgIHRoaXMuc29ja2V0Lm9ub3BlbiA9ICgpID0+IHtcbiAgICAgIHRoaXMuZW1pdCgnb3BlbicpO1xuICAgIH1cblxuICAgIHRoaXMuc29ja2V0Lm9ubWVzc2FnZSA9IChlKT0+e1xuICAgICAgbGV0IGRhdGEgPSBuZXcgQnVmZmVyKGUuZGF0YSwgJ2Jhc2U2NCcpLnRvU3RyaW5nKCd1dGY4Jyk7XG4gICAgICB0aGlzLmVtaXQoJ2RhdGEnLCBkYXRhKTtcbiAgICB9XG5cbiAgICB0aGlzLnNvY2tldC5vbmNsb3NlID0gKCk9PntcbiAgICAgIHRoaXMuZW1pdCgnY2xvc2UnKTtcbiAgICB9XG4gIH1cblxuICBzZW5kKGRhdGEpe1xuICAgIHRoaXMuc29ja2V0LnNlbmQoZGF0YSk7XG4gIH1cbn1cblxubW9kdWxlLmV4cG9ydHMgPSBUdHk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tbW9uL3R0eS5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciB7YWN0aW9uc30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9jdXJyZW50U2Vzc2lvbi8nKTtcbnZhciB7VXNlckljb259ID0gcmVxdWlyZSgnLi8uLi9pY29ucy5qc3gnKTtcbnZhciBSZWFjdENTU1RyYW5zaXRpb25Hcm91cCA9IHJlcXVpcmUoJ3JlYWN0LWFkZG9ucy1jc3MtdHJhbnNpdGlvbi1ncm91cCcpO1xuXG5jb25zdCBTZXNzaW9uTGVmdFBhbmVsID0gKHtwYXJ0aWVzfSkgPT4ge1xuICBwYXJ0aWVzID0gcGFydGllcyB8fCBbXTtcbiAgbGV0IHVzZXJJY29ucyA9IHBhcnRpZXMubWFwKChpdGVtLCBpbmRleCk9PihcbiAgICA8bGkga2V5PXtpbmRleH0gY2xhc3NOYW1lPVwiYW5pbWF0ZWRcIj48VXNlckljb24gY29sb3JJbmRleD17aW5kZXh9IGlzRGFyaz17dHJ1ZX0gbmFtZT17aXRlbS51c2VyfS8+PC9saT5cbiAgKSk7XG5cbiAgcmV0dXJuIChcbiAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi10ZXJtaW5hbC1wYXJ0aWNpcGFuc1wiPlxuICAgICAgPHVsIGNsYXNzTmFtZT1cIm5hdlwiPlxuICAgICAgICA8bGkgdGl0bGU9XCJDbG9zZVwiPlxuICAgICAgICAgIDxidXR0b24gb25DbGljaz17YWN0aW9ucy5jbG9zZX0gY2xhc3NOYW1lPVwiYnRuIGJ0bi1kYW5nZXIgYnRuLWNpcmNsZVwiIHR5cGU9XCJidXR0b25cIj5cbiAgICAgICAgICAgIDxpIGNsYXNzTmFtZT1cImZhIGZhLXRpbWVzXCI+PC9pPlxuICAgICAgICAgIDwvYnV0dG9uPlxuICAgICAgICA8L2xpPlxuICAgICAgPC91bD5cbiAgICAgIHsgdXNlckljb25zLmxlbmd0aCA+IDAgPyA8aHIgY2xhc3NOYW1lPVwiZ3J2LWRpdmlkZXJcIi8+IDogbnVsbCB9XG4gICAgICA8UmVhY3RDU1NUcmFuc2l0aW9uR3JvdXAgY2xhc3NOYW1lPVwibmF2XCIgY29tcG9uZW50PSd1bCdcbiAgICAgICAgdHJhbnNpdGlvbkVudGVyVGltZW91dD17NTAwfVxuICAgICAgICB0cmFuc2l0aW9uTGVhdmVUaW1lb3V0PXs1MDB9XG4gICAgICAgIHRyYW5zaXRpb25OYW1lPXt7XG4gICAgICAgICAgZW50ZXI6IFwiZmFkZUluXCIsXG4gICAgICAgICAgbGVhdmU6IFwiZmFkZU91dFwiXG4gICAgICAgIH19PlxuICAgICAgICB7dXNlckljb25zfVxuICAgICAgPC9SZWFjdENTU1RyYW5zaXRpb25Hcm91cD5cbiAgICA8L2Rpdj5cbiAgKVxufTtcblxubW9kdWxlLmV4cG9ydHMgPSBTZXNzaW9uTGVmdFBhbmVsO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvY3VycmVudFNlc3Npb24vc2Vzc2lvbkxlZnRQYW5lbC5qc3hcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG5cbnZhciBHb29nbGVBdXRoSW5mbyA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgcmVuZGVyKCkge1xuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1nb29nbGUtYXV0aFwiPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1pY29uLWdvb2dsZS1hdXRoXCI+PC9kaXY+XG4gICAgICAgIDxzdHJvbmc+R29vZ2xlIEF1dGhlbnRpY2F0b3I8L3N0cm9uZz5cbiAgICAgICAgPGRpdj5Eb3dubG9hZCA8YSBocmVmPVwiaHR0cHM6Ly9zdXBwb3J0Lmdvb2dsZS5jb20vYWNjb3VudHMvYW5zd2VyLzEwNjY0NDc/aGw9ZW5cIj5Hb29nbGUgQXV0aGVudGljYXRvcjwvYT4gb24geW91ciBwaG9uZSB0byBhY2Nlc3MgeW91ciB0d28gZmFjdG9yIHRva2VuPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KVxuXG5tb2R1bGUuZXhwb3J0cyA9IEdvb2dsZUF1dGhJbmZvO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvZ29vZ2xlQXV0aExvZ28uanN4XG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHtkZWJvdW5jZX0gPSByZXF1aXJlKCdfJyk7XG5cbnZhciBJbnB1dFNlYXJjaCA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBnZXRJbml0aWFsU3RhdGUoKXtcbiAgICB0aGlzLmRlYm91bmNlZE5vdGlmeSA9IGRlYm91bmNlKCgpPT57ICAgICAgICBcbiAgICAgICAgdGhpcy5wcm9wcy5vbkNoYW5nZSh0aGlzLnN0YXRlLnZhbHVlKTtcbiAgICB9LCAyMDApO1xuXG4gICAgcmV0dXJuIHt2YWx1ZTogdGhpcy5wcm9wcy52YWx1ZX07XG4gIH0sXG5cbiAgb25DaGFuZ2UoZSl7XG4gICAgdGhpcy5zZXRTdGF0ZSh7dmFsdWU6IGUudGFyZ2V0LnZhbHVlfSk7XG4gICAgdGhpcy5kZWJvdW5jZWROb3RpZnkoKTtcbiAgfSxcblxuICBjb21wb25lbnREaWRNb3VudCgpIHtcbiAgfSxcblxuICBjb21wb25lbnRXaWxsVW5tb3VudCgpIHtcbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1zZWFyY2hcIj5cbiAgICAgICAgPGlucHV0IHBsYWNlaG9sZGVyPVwiU2VhcmNoLi4uXCIgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sIGlucHV0LXNtXCJcbiAgICAgICAgICB2YWx1ZT17dGhpcy5zdGF0ZS52YWx1ZX1cbiAgICAgICAgICBvbkNoYW5nZT17dGhpcy5vbkNoYW5nZX0gLz5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IElucHV0U2VhcmNoO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvaW5wdXRTZWFyY2guanN4XG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIElucHV0U2VhcmNoID0gcmVxdWlyZSgnLi8uLi9pbnB1dFNlYXJjaC5qc3gnKTtcbnZhciB7VGFibGUsIENvbHVtbiwgQ2VsbCwgU29ydEhlYWRlckNlbGwsIFNvcnRUeXBlcywgRW1wdHlJbmRpY2F0b3J9ID0gcmVxdWlyZSgnYXBwL2NvbXBvbmVudHMvdGFibGUuanN4Jyk7XG52YXIge2NyZWF0ZU5ld1Nlc3Npb259ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvY3VycmVudFNlc3Npb24vYWN0aW9ucycpO1xuXG52YXIgXyA9IHJlcXVpcmUoJ18nKTtcbnZhciB7aXNNYXRjaH0gPSByZXF1aXJlKCdhcHAvY29tbW9uL29iamVjdFV0aWxzJyk7XG5cbmNvbnN0IFRleHRDZWxsID0gKHtyb3dJbmRleCwgZGF0YSwgY29sdW1uS2V5LCAuLi5wcm9wc30pID0+IChcbiAgPENlbGwgey4uLnByb3BzfT5cbiAgICB7ZGF0YVtyb3dJbmRleF1bY29sdW1uS2V5XX1cbiAgPC9DZWxsPlxuKTtcblxuY29uc3QgVGFnQ2VsbCA9ICh7cm93SW5kZXgsIGRhdGEsIC4uLnByb3BzfSkgPT4gKFxuICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgIHsgZGF0YVtyb3dJbmRleF0udGFncy5tYXAoKGl0ZW0sIGluZGV4KSA9PlxuICAgICAgKDxzcGFuIGtleT17aW5kZXh9IGNsYXNzTmFtZT1cImxhYmVsIGxhYmVsLWRlZmF1bHRcIj5cbiAgICAgICAge2l0ZW0ucm9sZX0gPGxpIGNsYXNzTmFtZT1cImZhIGZhLWxvbmctYXJyb3ctcmlnaHRcIj48L2xpPlxuICAgICAgICB7aXRlbS52YWx1ZX1cbiAgICAgIDwvc3Bhbj4pXG4gICAgKSB9XG4gIDwvQ2VsbD5cbik7XG5cbmNvbnN0IExvZ2luQ2VsbCA9ICh7bG9naW5zLCBvbkxvZ2luQ2xpY2ssIHJvd0luZGV4LCBkYXRhLCAuLi5wcm9wc30pID0+IHtcbiAgaWYoIWxvZ2lucyB8fGxvZ2lucy5sZW5ndGggPT09IDApe1xuICAgIHJldHVybiA8Q2VsbCB7Li4ucHJvcHN9IC8+O1xuICB9XG5cbiAgdmFyIHNlcnZlcklkID0gZGF0YVtyb3dJbmRleF0uaWQ7XG4gIHZhciAkbGlzID0gW107XG5cbiAgZnVuY3Rpb24gb25DbGljayhpKXtcbiAgICB2YXIgbG9naW4gPSBsb2dpbnNbaV07XG4gICAgaWYob25Mb2dpbkNsaWNrKXtcbiAgICAgIHJldHVybiAoKT0+IG9uTG9naW5DbGljayhzZXJ2ZXJJZCwgbG9naW4pO1xuICAgIH1lbHNle1xuICAgICAgcmV0dXJuICgpID0+IGNyZWF0ZU5ld1Nlc3Npb24oc2VydmVySWQsIGxvZ2luKTtcbiAgICB9XG4gIH1cblxuICBmb3IodmFyIGkgPSAwOyBpIDwgbG9naW5zLmxlbmd0aDsgaSsrKXtcbiAgICAkbGlzLnB1c2goPGxpIGtleT17aX0+PGEgb25DbGljaz17b25DbGljayhpKX0+e2xvZ2luc1tpXX08L2E+PC9saT4pO1xuICB9XG5cbiAgcmV0dXJuIChcbiAgICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJidG4tZ3JvdXBcIj5cbiAgICAgICAgPGJ1dHRvbiB0eXBlPVwiYnV0dG9uXCIgb25DbGljaz17b25DbGljaygwKX0gY2xhc3NOYW1lPVwiYnRuIGJ0bi14cyBidG4tcHJpbWFyeVwiPntsb2dpbnNbMF19PC9idXR0b24+XG4gICAgICAgIHtcbiAgICAgICAgICAkbGlzLmxlbmd0aCA+IDEgPyAoXG4gICAgICAgICAgICAgIFtcbiAgICAgICAgICAgICAgICA8YnV0dG9uIGtleT17MH0gZGF0YS10b2dnbGU9XCJkcm9wZG93blwiIGNsYXNzTmFtZT1cImJ0biBidG4tZGVmYXVsdCBidG4teHMgZHJvcGRvd24tdG9nZ2xlXCIgYXJpYS1leHBhbmRlZD1cInRydWVcIj5cbiAgICAgICAgICAgICAgICAgIDxzcGFuIGNsYXNzTmFtZT1cImNhcmV0XCI+PC9zcGFuPlxuICAgICAgICAgICAgICAgIDwvYnV0dG9uPixcbiAgICAgICAgICAgICAgICA8dWwga2V5PXsxfSBjbGFzc05hbWU9XCJkcm9wZG93bi1tZW51XCI+XG4gICAgICAgICAgICAgICAgICB7JGxpc31cbiAgICAgICAgICAgICAgICA8L3VsPlxuICAgICAgICAgICAgICBdIClcbiAgICAgICAgICAgIDogbnVsbFxuICAgICAgICB9XG4gICAgICA8L2Rpdj5cbiAgICA8L0NlbGw+XG4gIClcbn07XG5cbnZhciBOb2RlTGlzdCA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBnZXRJbml0aWFsU3RhdGUoLypwcm9wcyovKXtcbiAgICB0aGlzLnNlYXJjaGFibGVQcm9wcyA9IFsnYWRkcicsICdob3N0bmFtZScsICd0YWdzJ107XG4gICAgcmV0dXJuIHsgZmlsdGVyOiAnJywgY29sU29ydERpcnM6IHtob3N0bmFtZTogU29ydFR5cGVzLkRFU0N9IH07XG4gIH0sXG5cbiAgb25Tb3J0Q2hhbmdlKGNvbHVtbktleSwgc29ydERpcikge1xuICAgIHRoaXMuc3RhdGUuY29sU29ydERpcnMgPSB7IFtjb2x1bW5LZXldOiBzb3J0RGlyIH07XG4gICAgdGhpcy5zZXRTdGF0ZSh0aGlzLnN0YXRlKTtcbiAgfSxcblxuICBvbkZpbHRlckNoYW5nZSh2YWx1ZSl7XG4gICAgdGhpcy5zdGF0ZS5maWx0ZXIgPSB2YWx1ZTtcbiAgICB0aGlzLnNldFN0YXRlKHRoaXMuc3RhdGUpO1xuICB9LFxuXG4gIHNlYXJjaEFuZEZpbHRlckNiKHRhcmdldFZhbHVlLCBzZWFyY2hWYWx1ZSwgcHJvcE5hbWUpe1xuICAgIGlmKHByb3BOYW1lID09PSAndGFncycpe1xuICAgICAgcmV0dXJuIHRhcmdldFZhbHVlLnNvbWUoKGl0ZW0pID0+IHtcbiAgICAgICAgbGV0IHtyb2xlLCB2YWx1ZX0gPSBpdGVtO1xuICAgICAgICByZXR1cm4gcm9sZS50b0xvY2FsZVVwcGVyQ2FzZSgpLmluZGV4T2Yoc2VhcmNoVmFsdWUpICE9PS0xIHx8XG4gICAgICAgICAgdmFsdWUudG9Mb2NhbGVVcHBlckNhc2UoKS5pbmRleE9mKHNlYXJjaFZhbHVlKSAhPT0tMTtcbiAgICAgIH0pO1xuICAgIH1cbiAgfSxcblxuICBzb3J0QW5kRmlsdGVyKGRhdGEpe1xuICAgIHZhciBmaWx0ZXJlZCA9IGRhdGEuZmlsdGVyKG9iaj0+IGlzTWF0Y2gob2JqLCB0aGlzLnN0YXRlLmZpbHRlciwge1xuICAgICAgICBzZWFyY2hhYmxlUHJvcHM6IHRoaXMuc2VhcmNoYWJsZVByb3BzLFxuICAgICAgICBjYjogdGhpcy5zZWFyY2hBbmRGaWx0ZXJDYlxuICAgICAgfSkpO1xuXG4gICAgdmFyIGNvbHVtbktleSA9IE9iamVjdC5nZXRPd25Qcm9wZXJ0eU5hbWVzKHRoaXMuc3RhdGUuY29sU29ydERpcnMpWzBdO1xuICAgIHZhciBzb3J0RGlyID0gdGhpcy5zdGF0ZS5jb2xTb3J0RGlyc1tjb2x1bW5LZXldO1xuICAgIHZhciBzb3J0ZWQgPSBfLnNvcnRCeShmaWx0ZXJlZCwgY29sdW1uS2V5KTtcbiAgICBpZihzb3J0RGlyID09PSBTb3J0VHlwZXMuQVNDKXtcbiAgICAgIHNvcnRlZCA9IHNvcnRlZC5yZXZlcnNlKCk7XG4gICAgfVxuXG4gICAgcmV0dXJuIHNvcnRlZDtcbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIHZhciBkYXRhID0gdGhpcy5zb3J0QW5kRmlsdGVyKHRoaXMucHJvcHMubm9kZVJlY29yZHMpO1xuICAgIHZhciBsb2dpbnMgPSB0aGlzLnByb3BzLmxvZ2lucztcbiAgICB2YXIgb25Mb2dpbkNsaWNrID0gdGhpcy5wcm9wcy5vbkxvZ2luQ2xpY2s7XG5cbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbm9kZXMgZ3J2LXBhZ2VcIj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZmxleCBncnYtaGVhZGVyXCI+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZmxleC1jb2x1bW5cIj48L2Rpdj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LWNvbHVtblwiPlxuICAgICAgICAgICAgPGgxPiBOb2RlcyA8L2gxPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWZsZXgtY29sdW1uXCI+XG4gICAgICAgICAgICA8SW5wdXRTZWFyY2ggdmFsdWU9e3RoaXMuZmlsdGVyfSBvbkNoYW5nZT17dGhpcy5vbkZpbHRlckNoYW5nZX0vPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICA8L2Rpdj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICB7XG4gICAgICAgICAgICBkYXRhLmxlbmd0aCA9PT0gMCAmJiB0aGlzLnN0YXRlLmZpbHRlci5sZW5ndGggPiAwID8gPEVtcHR5SW5kaWNhdG9yIHRleHQ9XCJObyBtYXRjaGluZyBub2RlcyBmb3VuZC5cIi8+IDpcblxuICAgICAgICAgICAgPFRhYmxlIHJvd0NvdW50PXtkYXRhLmxlbmd0aH0gY2xhc3NOYW1lPVwidGFibGUtc3RyaXBlZCBncnYtbm9kZXMtdGFibGVcIj5cbiAgICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICAgIGNvbHVtbktleT1cImhvc3RuYW1lXCJcbiAgICAgICAgICAgICAgICBoZWFkZXI9e1xuICAgICAgICAgICAgICAgICAgPFNvcnRIZWFkZXJDZWxsXG4gICAgICAgICAgICAgICAgICAgIHNvcnREaXI9e3RoaXMuc3RhdGUuY29sU29ydERpcnMuaG9zdG5hbWV9XG4gICAgICAgICAgICAgICAgICAgIG9uU29ydENoYW5nZT17dGhpcy5vblNvcnRDaGFuZ2V9XG4gICAgICAgICAgICAgICAgICAgIHRpdGxlPVwiTm9kZVwiXG4gICAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICAgIH1cbiAgICAgICAgICAgICAgICBjZWxsPXs8VGV4dENlbGwgZGF0YT17ZGF0YX0vPiB9XG4gICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJhZGRyXCJcbiAgICAgICAgICAgICAgICBoZWFkZXI9e1xuICAgICAgICAgICAgICAgICAgPFNvcnRIZWFkZXJDZWxsXG4gICAgICAgICAgICAgICAgICAgIHNvcnREaXI9e3RoaXMuc3RhdGUuY29sU29ydERpcnMuYWRkcn1cbiAgICAgICAgICAgICAgICAgICAgb25Tb3J0Q2hhbmdlPXt0aGlzLm9uU29ydENoYW5nZX1cbiAgICAgICAgICAgICAgICAgICAgdGl0bGU9XCJJUFwiXG4gICAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICAgIH1cblxuICAgICAgICAgICAgICAgIGNlbGw9ezxUZXh0Q2VsbCBkYXRhPXtkYXRhfS8+IH1cbiAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICAgIGNvbHVtbktleT1cInRhZ3NcIlxuICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+PC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgY2VsbD17PFRhZ0NlbGwgZGF0YT17ZGF0YX0vPiB9XG4gICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJyb2xlc1wiXG4gICAgICAgICAgICAgICAgb25Mb2dpbkNsaWNrPXtvbkxvZ2luQ2xpY2t9XG4gICAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD5Mb2dpbiBhczwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgIGNlbGw9ezxMb2dpbkNlbGwgZGF0YT17ZGF0YX0gbG9naW5zPXtsb2dpbnN9Lz4gfVxuICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgPC9UYWJsZT5cbiAgICAgICAgICB9XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9kaXY+XG4gICAgKVxuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBOb2RlTGlzdDtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL25vZGVzL25vZGVMaXN0LmpzeFxuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7IExpbmsgfSA9IHJlcXVpcmUoJ3JlYWN0LXJvdXRlcicpO1xudmFyIHtub2RlSG9zdE5hbWVCeVNlcnZlcklkfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL25vZGVzL2dldHRlcnMnKTtcbnZhciB7ZGlzcGxheURhdGVGb3JtYXR9ID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xudmFyIHtDZWxsfSA9IHJlcXVpcmUoJ2FwcC9jb21wb25lbnRzL3RhYmxlLmpzeCcpO1xudmFyIG1vbWVudCA9ICByZXF1aXJlKCdtb21lbnQnKTtcblxuY29uc3QgRGF0ZUNyZWF0ZWRDZWxsID0gKHsgcm93SW5kZXgsIGRhdGEsIC4uLnByb3BzIH0pID0+IHtcbiAgbGV0IGNyZWF0ZWQgPSBkYXRhW3Jvd0luZGV4XS5jcmVhdGVkO1xuICBsZXQgZGlzcGxheURhdGUgPSBtb21lbnQoY3JlYXRlZCkuZm9ybWF0KGRpc3BsYXlEYXRlRm9ybWF0KTtcbiAgcmV0dXJuIChcbiAgICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgICAgeyBkaXNwbGF5RGF0ZSB9XG4gICAgPC9DZWxsPlxuICApXG59O1xuXG5jb25zdCBEdXJhdGlvbkNlbGwgPSAoeyByb3dJbmRleCwgZGF0YSwgLi4ucHJvcHMgfSkgPT4ge1xuICBsZXQgY3JlYXRlZCA9IGRhdGFbcm93SW5kZXhdLmNyZWF0ZWQ7XG4gIGxldCBsYXN0QWN0aXZlID0gZGF0YVtyb3dJbmRleF0ubGFzdEFjdGl2ZTtcblxuICBsZXQgZW5kID0gbW9tZW50KGNyZWF0ZWQpO1xuICBsZXQgbm93ID0gbW9tZW50KGxhc3RBY3RpdmUpO1xuICBsZXQgZHVyYXRpb24gPSBtb21lbnQuZHVyYXRpb24obm93LmRpZmYoZW5kKSk7XG4gIGxldCBkaXNwbGF5RGF0ZSA9IGR1cmF0aW9uLmh1bWFuaXplKCk7XG5cbiAgcmV0dXJuIChcbiAgICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgICAgeyBkaXNwbGF5RGF0ZSB9XG4gICAgPC9DZWxsPlxuICApXG59O1xuXG5jb25zdCBTaW5nbGVVc2VyQ2VsbCA9ICh7IHJvd0luZGV4LCBkYXRhLCAuLi5wcm9wcyB9KSA9PiB7XG4gIHJldHVybiAoXG4gICAgPENlbGwgey4uLnByb3BzfT5cbiAgICAgIDxzcGFuIGNsYXNzTmFtZT1cImdydi1zZXNzaW9ucy11c2VyIGxhYmVsIGxhYmVsLWRlZmF1bHRcIj57ZGF0YVtyb3dJbmRleF0ubG9naW59PC9zcGFuPlxuICAgIDwvQ2VsbD5cbiAgKVxufTtcblxuY29uc3QgVXNlcnNDZWxsID0gKHsgcm93SW5kZXgsIGRhdGEsIC4uLnByb3BzIH0pID0+IHtcbiAgbGV0ICR1c2VycyA9IGRhdGFbcm93SW5kZXhdLnBhcnRpZXMubWFwKChpdGVtLCBpdGVtSW5kZXgpPT5cbiAgICAoPHNwYW4ga2V5PXtpdGVtSW5kZXh9IGNsYXNzTmFtZT1cImdydi1zZXNzaW9ucy11c2VyIGxhYmVsIGxhYmVsLWRlZmF1bHRcIj57aXRlbS51c2VyfTwvc3Bhbj4pXG4gIClcblxuICByZXR1cm4gKFxuICAgIDxDZWxsIHsuLi5wcm9wc30+XG4gICAgICA8ZGl2PlxuICAgICAgICB7JHVzZXJzfVxuICAgICAgPC9kaXY+XG4gICAgPC9DZWxsPlxuICApXG59O1xuXG5jb25zdCBCdXR0b25DZWxsID0gKHsgcm93SW5kZXgsIGRhdGEsIC4uLnByb3BzIH0pID0+IHtcbiAgbGV0IHsgc2Vzc2lvblVybCwgYWN0aXZlIH0gPSBkYXRhW3Jvd0luZGV4XTtcbiAgbGV0IFthY3Rpb25UZXh0LCBhY3Rpb25DbGFzc10gPSBhY3RpdmUgPyBbJ2pvaW4nLCAnYnRuLXdhcm5pbmcnXSA6IFsncGxheScsICdidG4tcHJpbWFyeSddO1xuICByZXR1cm4gKFxuICAgIDxDZWxsIHsuLi5wcm9wc30+XG4gICAgICA8TGluayB0bz17c2Vzc2lvblVybH0gY2xhc3NOYW1lPXtcImJ0biBcIiArYWN0aW9uQ2xhc3MrIFwiIGJ0bi14c1wifSB0eXBlPVwiYnV0dG9uXCI+e2FjdGlvblRleHR9PC9MaW5rPlxuICAgIDwvQ2VsbD5cbiAgKVxufVxuXG5jb25zdCBOb2RlQ2VsbCA9ICh7IHJvd0luZGV4LCBkYXRhLCAuLi5wcm9wcyB9KSA9PiB7XG4gIGxldCB7c2VydmVySWR9ID0gZGF0YVtyb3dJbmRleF07XG4gIGxldCBob3N0bmFtZSA9IHJlYWN0b3IuZXZhbHVhdGUobm9kZUhvc3ROYW1lQnlTZXJ2ZXJJZChzZXJ2ZXJJZCkpIHx8ICd1bmtub3duJztcblxuICByZXR1cm4gKFxuICAgIDxDZWxsIHsuLi5wcm9wc30+XG4gICAgICB7aG9zdG5hbWV9XG4gICAgPC9DZWxsPlxuICApXG59XG5cbmV4cG9ydCBkZWZhdWx0IEJ1dHRvbkNlbGw7XG5cbmV4cG9ydCB7XG4gIEJ1dHRvbkNlbGwsXG4gIFVzZXJzQ2VsbCxcbiAgRHVyYXRpb25DZWxsLFxuICBEYXRlQ3JlYXRlZENlbGwsXG4gIFNpbmdsZVVzZXJDZWxsLFxuICBOb2RlQ2VsbFxufTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL3Nlc3Npb25zL2xpc3RJdGVtcy5qc3hcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbmltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX0FQUF9JTklUOiBudWxsLFxuICBUTFBUX0FQUF9GQUlMRUQ6IG51bGwsXG4gIFRMUFRfQVBQX1JFQURZOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvYXBwL2FjdGlvblR5cGVzLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xudmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG5cbnZhciB7IFRMUFRfQVBQX0lOSVQsIFRMUFRfQVBQX0ZBSUxFRCwgVExQVF9BUFBfUkVBRFkgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxudmFyIGluaXRTdGF0ZSA9IHRvSW1tdXRhYmxlKHtcbiAgaXNSZWFkeTogZmFsc2UsXG4gIGlzSW5pdGlhbGl6aW5nOiBmYWxzZSxcbiAgaXNGYWlsZWQ6IGZhbHNlXG59KTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuXG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gaW5pdFN0YXRlLnNldCgnaXNJbml0aWFsaXppbmcnLCB0cnVlKTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9BUFBfSU5JVCwgKCk9PiBpbml0U3RhdGUuc2V0KCdpc0luaXRpYWxpemluZycsIHRydWUpKTtcbiAgICB0aGlzLm9uKFRMUFRfQVBQX1JFQURZLCgpPT4gaW5pdFN0YXRlLnNldCgnaXNSZWFkeScsIHRydWUpKTtcbiAgICB0aGlzLm9uKFRMUFRfQVBQX0ZBSUxFRCwoKT0+IGluaXRTdGF0ZS5zZXQoJ2lzRmFpbGVkJywgdHJ1ZSkpO1xuICB9XG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvYXBwL2FwcFN0b3JlLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfQ1VSUkVOVF9TRVNTSU9OX09QRU46IG51bGwsXG4gIFRMUFRfQ1VSUkVOVF9TRVNTSU9OX0NMT1NFOiBudWxsICBcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9jdXJyZW50U2Vzc2lvbi9hY3Rpb25UeXBlcy5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciBzZXNzaW9uID0gcmVxdWlyZSgnYXBwL3NlcnZpY2VzL3Nlc3Npb24nKTtcbnZhciBhcGkgPSByZXF1aXJlKCdhcHAvc2VydmljZXMvYXBpJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xudmFyIGdldHRlcnMgPSByZXF1aXJlKCcuL2dldHRlcnMnKTtcbnZhciBzZXNzaW9uTW9kdWxlID0gcmVxdWlyZSgnLi8uLi9zZXNzaW9ucycpO1xuXG5jb25zdCBsb2dnZXIgPSByZXF1aXJlKCdhcHAvY29tbW9uL2xvZ2dlcicpLmNyZWF0ZSgnQ3VycmVudCBTZXNzaW9uJyk7XG5jb25zdCB7IFRMUFRfQ1VSUkVOVF9TRVNTSU9OX09QRU4sIFRMUFRfQ1VSUkVOVF9TRVNTSU9OX0NMT1NFIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmNvbnN0IGFjdGlvbnMgPSB7XG5cbiAgY2xvc2UoKXtcbiAgICBsZXQge2lzTmV3U2Vzc2lvbn0gPSByZWFjdG9yLmV2YWx1YXRlKGdldHRlcnMuY3VycmVudFNlc3Npb24pO1xuXG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX0NVUlJFTlRfU0VTU0lPTl9DTE9TRSk7XG5cbiAgICBpZihpc05ld1Nlc3Npb24pe1xuICAgICAgc2Vzc2lvbi5nZXRIaXN0b3J5KCkucHVzaChjZmcucm91dGVzLm5vZGVzKTtcbiAgICB9ZWxzZXtcbiAgICAgIHNlc3Npb24uZ2V0SGlzdG9yeSgpLnB1c2goY2ZnLnJvdXRlcy5zZXNzaW9ucyk7XG4gICAgfVxuICB9LFxuXG4gIHJlc2l6ZSh3LCBoKXtcbiAgICAvLyBzb21lIG1pbiB2YWx1ZXNcbiAgICB3ID0gdyA8IDUgPyA1IDogdztcbiAgICBoID0gaCA8IDUgPyA1IDogaDtcblxuICAgIGxldCByZXFEYXRhID0geyB0ZXJtaW5hbF9wYXJhbXM6IHsgdywgaCB9IH07XG4gICAgbGV0IHtzaWR9ID0gcmVhY3Rvci5ldmFsdWF0ZShnZXR0ZXJzLmN1cnJlbnRTZXNzaW9uKTtcblxuICAgIGxvZ2dlci5pbmZvKCdyZXNpemUnLCBgdzoke3d9IGFuZCBoOiR7aH1gKTtcbiAgICBhcGkucHV0KGNmZy5hcGkuZ2V0VGVybWluYWxTZXNzaW9uVXJsKHNpZCksIHJlcURhdGEpXG4gICAgICAuZG9uZSgoKT0+IGxvZ2dlci5pbmZvKCdyZXNpemVkJykpXG4gICAgICAuZmFpbCgoZXJyKT0+IGxvZ2dlci5lcnJvcignZmFpbGVkIHRvIHJlc2l6ZScsIGVycikpO1xuICB9LFxuXG4gIG9wZW5TZXNzaW9uKHNpZCl7XG4gICAgbG9nZ2VyLmluZm8oJ2F0dGVtcHQgdG8gb3BlbiBzZXNzaW9uJywge3NpZH0pO1xuICAgIHNlc3Npb25Nb2R1bGUuYWN0aW9ucy5mZXRjaFNlc3Npb24oc2lkKVxuICAgICAgLmRvbmUoKCk9PntcbiAgICAgICAgbGV0IHNWaWV3ID0gcmVhY3Rvci5ldmFsdWF0ZShzZXNzaW9uTW9kdWxlLmdldHRlcnMuc2Vzc2lvblZpZXdCeUlkKHNpZCkpO1xuICAgICAgICBsZXQgeyBzZXJ2ZXJJZCwgbG9naW4gfSA9IHNWaWV3O1xuICAgICAgICBsb2dnZXIuaW5mbygnb3BlbiBzZXNzaW9uJywgJ09LJyk7XG4gICAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9DVVJSRU5UX1NFU1NJT05fT1BFTiwge1xuICAgICAgICAgICAgc2VydmVySWQsXG4gICAgICAgICAgICBsb2dpbixcbiAgICAgICAgICAgIHNpZCxcbiAgICAgICAgICAgIGlzTmV3U2Vzc2lvbjogZmFsc2VcbiAgICAgICAgICB9KTtcbiAgICAgIH0pXG4gICAgICAuZmFpbCgoZXJyKT0+e1xuICAgICAgICBsb2dnZXIuZXJyb3IoJ29wZW4gc2Vzc2lvbicsIGVycik7XG4gICAgICAgIHNlc3Npb24uZ2V0SGlzdG9yeSgpLnB1c2goY2ZnLnJvdXRlcy5wYWdlTm90Rm91bmQpO1xuICAgICAgfSlcbiAgfSxcblxuICBjcmVhdGVOZXdTZXNzaW9uKHNlcnZlcklkLCBsb2dpbil7XG4gICAgbGV0IGRhdGEgPSB7ICdzZXNzaW9uJzogeyd0ZXJtaW5hbF9wYXJhbXMnOiB7J3cnOiA0NSwgJ2gnOiA1fSwgbG9naW59fVxuICAgIGFwaS5wb3N0KGNmZy5hcGkuc2l0ZVNlc3Npb25QYXRoLCBkYXRhKS50aGVuKGpzb249PntcbiAgICAgIGxldCBzaWQgPSBqc29uLnNlc3Npb24uaWQ7XG4gICAgICBsZXQgcm91dGVVcmwgPSBjZmcuZ2V0QWN0aXZlU2Vzc2lvblJvdXRlVXJsKHNpZCk7XG4gICAgICBsZXQgaGlzdG9yeSA9IHNlc3Npb24uZ2V0SGlzdG9yeSgpO1xuXG4gICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfQ1VSUkVOVF9TRVNTSU9OX09QRU4sIHtcbiAgICAgICBzZXJ2ZXJJZCxcbiAgICAgICBsb2dpbixcbiAgICAgICBzaWQsXG4gICAgICAgaXNOZXdTZXNzaW9uOiB0cnVlXG4gICAgICB9KTtcblxuICAgICAgaGlzdG9yeS5wdXNoKHJvdXRlVXJsKTtcbiAgIH0pO1xuXG4gIH1cbn1cblxuZXhwb3J0IGRlZmF1bHQgYWN0aW9ucztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2N1cnJlbnRTZXNzaW9uL2FjdGlvbnMuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyIHsgVExQVF9DVVJSRU5UX1NFU1NJT05fT1BFTiwgVExQVF9DVVJSRU5UX1NFU1NJT05fQ0xPU0UgfSAgPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IFN0b3JlKHtcbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB0b0ltbXV0YWJsZShudWxsKTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9DVVJSRU5UX1NFU1NJT05fT1BFTiwgc2V0Q3VycmVudFNlc3Npb24pO1xuICAgIHRoaXMub24oVExQVF9DVVJSRU5UX1NFU1NJT05fQ0xPU0UsIGNsb3NlKTtcbiAgfVxufSlcblxuZnVuY3Rpb24gY2xvc2UoKXtcbiAgcmV0dXJuIHRvSW1tdXRhYmxlKG51bGwpO1xufVxuXG5mdW5jdGlvbiBzZXRDdXJyZW50U2Vzc2lvbihzdGF0ZSwge3NlcnZlcklkLCBsb2dpbiwgc2lkLCBpc05ld1Nlc3Npb259ICl7XG4gIHJldHVybiB0b0ltbXV0YWJsZSh7XG4gICAgc2VydmVySWQsXG4gICAgbG9naW4sXG4gICAgc2lkLFxuICAgIGlzTmV3U2Vzc2lvblxuICB9KTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2N1cnJlbnRTZXNzaW9uL2N1cnJlbnRTZXNzaW9uU3RvcmUuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5tb2R1bGUuZXhwb3J0cy5nZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3Rpb25zID0gcmVxdWlyZSgnLi9hY3Rpb25zJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3RpdmVUZXJtU3RvcmUgPSByZXF1aXJlKCcuL2N1cnJlbnRTZXNzaW9uU3RvcmUnKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2N1cnJlbnRTZXNzaW9uL2luZGV4LmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG5pbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVExQVF9ESUFMT0dfU0VMRUNUX05PREVfU0hPVzogbnVsbCxcbiAgVExQVF9ESUFMT0dfU0VMRUNUX05PREVfQ0xPU0U6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9kaWFsb2dzL2FjdGlvblR5cGVzLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgeyBUTFBUX0RJQUxPR19TRUxFQ1RfTk9ERV9TSE9XLCBUTFBUX0RJQUxPR19TRUxFQ1RfTk9ERV9DTE9TRSB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG52YXIgYWN0aW9ucyA9IHtcbiAgc2hvd1NlbGVjdE5vZGVEaWFsb2coKXtcbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfRElBTE9HX1NFTEVDVF9OT0RFX1NIT1cpO1xuICB9LFxuXG4gIGNsb3NlU2VsZWN0Tm9kZURpYWxvZygpe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9ESUFMT0dfU0VMRUNUX05PREVfQ0xPU0UpO1xuICB9XG59XG5cbmV4cG9ydCBkZWZhdWx0IGFjdGlvbnM7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9kaWFsb2dzL2FjdGlvbnMuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xuXG52YXIgeyBUTFBUX0RJQUxPR19TRUxFQ1RfTk9ERV9TSE9XLCBUTFBUX0RJQUxPR19TRUxFQ1RfTk9ERV9DTE9TRSB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG5cbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB0b0ltbXV0YWJsZSh7XG4gICAgICBpc1NlbGVjdE5vZGVEaWFsb2dPcGVuOiBmYWxzZVxuICAgIH0pO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX0RJQUxPR19TRUxFQ1RfTk9ERV9TSE9XLCBzaG93U2VsZWN0Tm9kZURpYWxvZyk7XG4gICAgdGhpcy5vbihUTFBUX0RJQUxPR19TRUxFQ1RfTk9ERV9DTE9TRSwgY2xvc2VTZWxlY3ROb2RlRGlhbG9nKTtcbiAgfVxufSlcblxuZnVuY3Rpb24gc2hvd1NlbGVjdE5vZGVEaWFsb2coc3RhdGUpe1xuICByZXR1cm4gc3RhdGUuc2V0KCdpc1NlbGVjdE5vZGVEaWFsb2dPcGVuJywgdHJ1ZSk7XG59XG5cbmZ1bmN0aW9uIGNsb3NlU2VsZWN0Tm9kZURpYWxvZyhzdGF0ZSl7XG4gIHJldHVybiBzdGF0ZS5zZXQoJ2lzU2VsZWN0Tm9kZURpYWxvZ09wZW4nLCBmYWxzZSk7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9kaWFsb2dzL2RpYWxvZ1N0b3JlLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG5pbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVExQVF9OT0RFU19SRUNFSVZFOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvYWN0aW9uVHlwZXMuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbmltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUTFBUX05PVElGSUNBVElPTlNfQUREOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvbm90aWZpY2F0aW9ucy9hY3Rpb25UeXBlcy5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxuaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfUkVTVF9BUElfU1RBUlQ6IG51bGwsXG4gIFRMUFRfUkVTVF9BUElfU1VDQ0VTUzogbnVsbCxcbiAgVExQVF9SRVNUX0FQSV9GQUlMOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9hY3Rpb25UeXBlcy5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxuaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRSWUlOR19UT19TSUdOX1VQOiBudWxsLFxuICBUUllJTkdfVE9fTE9HSU46IG51bGwsXG4gIEZFVENISU5HX0lOVklURTogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvY29uc3RhbnRzLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG5pbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVExQVF9TVE9SRURfU0VTU0lOU19GSUxURVJfU0VUX1JBTkdFOiBudWxsLFxuICBUTFBUX1NUT1JFRF9TRVNTSU5TX0ZJTFRFUl9TRVRfU1RBVFVTOiBudWxsLFxuICBUTFBUX1NUT1JFRF9TRVNTSU5TX0ZJTFRFUl9SRUNFSVZFX01PUkU6IG51bGxcbn0pXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9zdG9yZWRTZXNzaW9uc0ZpbHRlci9hY3Rpb25UeXBlcy5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHsgVExQVF9SRUNFSVZFX1VTRVIsIFRMUFRfUkVDRUlWRV9VU0VSX0lOVklURSB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xudmFyIHsgVFJZSU5HX1RPX1NJR05fVVAsIFRSWUlOR19UT19MT0dJTiwgRkVUQ0hJTkdfSU5WSVRFfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3Jlc3RBcGkvY29uc3RhbnRzJyk7XG52YXIgcmVzdEFwaUFjdGlvbnMgPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9yZXN0QXBpL2FjdGlvbnMnKTtcbnZhciBhdXRoID0gcmVxdWlyZSgnYXBwL3NlcnZpY2VzL2F1dGgnKTtcbnZhciBzZXNzaW9uID0gcmVxdWlyZSgnYXBwL3NlcnZpY2VzL3Nlc3Npb24nKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG52YXIgYXBpID0gcmVxdWlyZSgnYXBwL3NlcnZpY2VzL2FwaScpO1xuXG5leHBvcnQgZGVmYXVsdCB7XG5cbiAgZmV0Y2hJbnZpdGUoaW52aXRlVG9rZW4pe1xuICAgIHZhciBwYXRoID0gY2ZnLmFwaS5nZXRJbnZpdGVVcmwoaW52aXRlVG9rZW4pO1xuICAgIHJlc3RBcGlBY3Rpb25zLnN0YXJ0KEZFVENISU5HX0lOVklURSk7XG4gICAgYXBpLmdldChwYXRoKS5kb25lKGludml0ZT0+e1xuICAgICAgcmVzdEFwaUFjdGlvbnMuc3VjY2VzcyhGRVRDSElOR19JTlZJVEUpO1xuICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFQ0VJVkVfVVNFUl9JTlZJVEUsIGludml0ZSk7XG4gICAgfSkuXG4gICAgZmFpbCgoZXJyKT0+e1xuICAgICAgcmVzdEFwaUFjdGlvbnMuZmFpbChGRVRDSElOR19JTlZJVEUsIGVyci5yZXNwb25zZUpTT04ubWVzc2FnZSk7XG4gICAgfSk7XG4gIH0sXG5cbiAgZW5zdXJlVXNlcihuZXh0U3RhdGUsIHJlcGxhY2UsIGNiKXtcbiAgICBhdXRoLmVuc3VyZVVzZXIoKVxuICAgICAgLmRvbmUoKHVzZXJEYXRhKT0+IHtcbiAgICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1JFQ0VJVkVfVVNFUiwgdXNlckRhdGEudXNlciApO1xuICAgICAgICBjYigpO1xuICAgICAgfSlcbiAgICAgIC5mYWlsKCgpPT57XG4gICAgICAgIGxldCBuZXdMb2NhdGlvbiA9IHtcbiAgICAgICAgICAgIHBhdGhuYW1lOiBjZmcucm91dGVzLmxvZ2luLFxuICAgICAgICAgICAgc3RhdGU6IHtcbiAgICAgICAgICAgICAgcmVkaXJlY3RUbzogbmV4dFN0YXRlLmxvY2F0aW9uLnBhdGhuYW1lXG4gICAgICAgICAgICB9XG4gICAgICAgICAgfTtcblxuICAgICAgICByZXBsYWNlKG5ld0xvY2F0aW9uKTtcbiAgICAgICAgY2IoKTtcbiAgICAgIH0pO1xuICB9LFxuXG4gIHNpZ25VcCh7bmFtZSwgcHN3LCB0b2tlbiwgaW52aXRlVG9rZW59KXtcbiAgICByZXN0QXBpQWN0aW9ucy5zdGFydChUUllJTkdfVE9fU0lHTl9VUCk7XG4gICAgYXV0aC5zaWduVXAobmFtZSwgcHN3LCB0b2tlbiwgaW52aXRlVG9rZW4pXG4gICAgICAuZG9uZSgoc2Vzc2lvbkRhdGEpPT57XG4gICAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRUNFSVZFX1VTRVIsIHNlc3Npb25EYXRhLnVzZXIpO1xuICAgICAgICByZXN0QXBpQWN0aW9ucy5zdWNjZXNzKFRSWUlOR19UT19TSUdOX1VQKTtcbiAgICAgICAgc2Vzc2lvbi5nZXRIaXN0b3J5KCkucHVzaCh7cGF0aG5hbWU6IGNmZy5yb3V0ZXMuYXBwfSk7XG4gICAgICB9KVxuICAgICAgLmZhaWwoKGVycik9PntcbiAgICAgICAgcmVzdEFwaUFjdGlvbnMuZmFpbChUUllJTkdfVE9fU0lHTl9VUCwgZXJyLnJlc3BvbnNlSlNPTi5tZXNzYWdlIHx8ICdmYWlsZWQgdG8gc2luZyB1cCcpO1xuICAgICAgfSk7XG4gIH0sXG5cbiAgbG9naW4oe3VzZXIsIHBhc3N3b3JkLCB0b2tlbiwgcHJvdmlkZXJ9LCByZWRpcmVjdCl7XG4gICAgaWYocHJvdmlkZXIpe1xuICAgICAgbGV0IGZ1bGxQYXRoID0gY2ZnLmdldEZ1bGxVcmwocmVkaXJlY3QpO1xuICAgICAgd2luZG93LmxvY2F0aW9uID0gY2ZnLmFwaS5nZXRTc29VcmwoZnVsbFBhdGgsIHByb3ZpZGVyKTtcbiAgICAgIHJldHVybjtcbiAgICB9XG5cbiAgICByZXN0QXBpQWN0aW9ucy5zdGFydChUUllJTkdfVE9fTE9HSU4pO1xuICAgIGF1dGgubG9naW4odXNlciwgcGFzc3dvcmQsIHRva2VuKVxuICAgICAgLmRvbmUoKHNlc3Npb25EYXRhKT0+e1xuICAgICAgICByZXN0QXBpQWN0aW9ucy5zdWNjZXNzKFRSWUlOR19UT19MT0dJTik7XG4gICAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRUNFSVZFX1VTRVIsIHNlc3Npb25EYXRhLnVzZXIpO1xuICAgICAgICBzZXNzaW9uLmdldEhpc3RvcnkoKS5wdXNoKHtwYXRobmFtZTogcmVkaXJlY3R9KTtcbiAgICAgIH0pXG4gICAgICAuZmFpbCgoZXJyKT0+IHJlc3RBcGlBY3Rpb25zLmZhaWwoVFJZSU5HX1RPX0xPR0lOLCBlcnIucmVzcG9uc2VKU09OLm1lc3NhZ2UpKVxuICAgIH1cbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvYWN0aW9ucy5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxubW9kdWxlLmV4cG9ydHMuZ2V0dGVycyA9IHJlcXVpcmUoJy4vZ2V0dGVycycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aW9ucyA9IHJlcXVpcmUoJy4vYWN0aW9ucycpO1xubW9kdWxlLmV4cG9ydHMubm9kZVN0b3JlID0gcmVxdWlyZSgnLi91c2VyU3RvcmUnKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3VzZXIvaW5kZXguanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyICB7IFRMUFRfUkVDRUlWRV9VU0VSIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IFN0b3JlKHtcbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB0b0ltbXV0YWJsZShudWxsKTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9SRUNFSVZFX1VTRVIsIHJlY2VpdmVVc2VyKVxuICB9XG5cbn0pXG5cbmZ1bmN0aW9uIHJlY2VpdmVVc2VyKHN0YXRlLCB1c2VyKXtcbiAgcmV0dXJuIHRvSW1tdXRhYmxlKHVzZXIpO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvdXNlci91c2VyU3RvcmUuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciBhcGkgPSByZXF1aXJlKCcuL2FwaScpO1xudmFyIGNmZyA9IHJlcXVpcmUoJy4uL2NvbmZpZycpO1xuXG5jb25zdCBhcGlVdGlscyA9IHtcbiAgICBmaWx0ZXJTZXNzaW9ucyh7c3RhcnQsIGVuZCwgc2lkLCBsaW1pdCwgb3JkZXI9LTF9KXtcbiAgICAgIGxldCBwYXJhbXMgPSB7XG4gICAgICAgIHN0YXJ0OiBzdGFydC50b0lTT1N0cmluZygpLFxuICAgICAgICBlbmQsXG4gICAgICAgIG9yZGVyLFxuICAgICAgICBsaW1pdFxuICAgICAgfVxuXG4gICAgICBpZihzaWQpe1xuICAgICAgICBwYXJhbXMuc2Vzc2lvbl9pZCA9IHNpZDtcbiAgICAgIH1cblxuICAgICAgcmV0dXJuIGFwaS5nZXQoY2ZnLmFwaS5nZXRGZXRjaFNlc3Npb25zVXJsKHBhcmFtcykpXG4gICAgfVxufVxuXG5tb2R1bGUuZXhwb3J0cyA9IGFwaVV0aWxzO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL3NlcnZpY2VzL2FwaVV0aWxzLmpzXG4gKiovIiwiLyoqXG4gKiBDb3B5cmlnaHQgMjAxMy0yMDE1LCBGYWNlYm9vaywgSW5jLlxuICogQWxsIHJpZ2h0cyByZXNlcnZlZC5cbiAqXG4gKiBUaGlzIHNvdXJjZSBjb2RlIGlzIGxpY2Vuc2VkIHVuZGVyIHRoZSBCU0Qtc3R5bGUgbGljZW5zZSBmb3VuZCBpbiB0aGVcbiAqIExJQ0VOU0UgZmlsZSBpbiB0aGUgcm9vdCBkaXJlY3Rvcnkgb2YgdGhpcyBzb3VyY2UgdHJlZS4gQW4gYWRkaXRpb25hbCBncmFudFxuICogb2YgcGF0ZW50IHJpZ2h0cyBjYW4gYmUgZm91bmQgaW4gdGhlIFBBVEVOVFMgZmlsZSBpbiB0aGUgc2FtZSBkaXJlY3RvcnkuXG4gKlxuICogQHByb3ZpZGVzTW9kdWxlIENTU0NvcmVcbiAqIEB0eXBlY2hlY2tzXG4gKi9cblxuJ3VzZSBzdHJpY3QnO1xuXG52YXIgaW52YXJpYW50ID0gcmVxdWlyZSgnLi9pbnZhcmlhbnQnKTtcblxuLyoqXG4gKiBUaGUgQ1NTQ29yZSBtb2R1bGUgc3BlY2lmaWVzIHRoZSBBUEkgKGFuZCBpbXBsZW1lbnRzIG1vc3Qgb2YgdGhlIG1ldGhvZHMpXG4gKiB0aGF0IHNob3VsZCBiZSB1c2VkIHdoZW4gZGVhbGluZyB3aXRoIHRoZSBkaXNwbGF5IG9mIGVsZW1lbnRzICh2aWEgdGhlaXJcbiAqIENTUyBjbGFzc2VzIGFuZCB2aXNpYmlsaXR5IG9uIHNjcmVlbi4gSXQgaXMgYW4gQVBJIGZvY3VzZWQgb24gbXV0YXRpbmcgdGhlXG4gKiBkaXNwbGF5IGFuZCBub3QgcmVhZGluZyBpdCBhcyBubyBsb2dpY2FsIHN0YXRlIHNob3VsZCBiZSBlbmNvZGVkIGluIHRoZVxuICogZGlzcGxheSBvZiBlbGVtZW50cy5cbiAqL1xuXG52YXIgQ1NTQ29yZSA9IHtcblxuICAvKipcbiAgICogQWRkcyB0aGUgY2xhc3MgcGFzc2VkIGluIHRvIHRoZSBlbGVtZW50IGlmIGl0IGRvZXNuJ3QgYWxyZWFkeSBoYXZlIGl0LlxuICAgKlxuICAgKiBAcGFyYW0ge0RPTUVsZW1lbnR9IGVsZW1lbnQgdGhlIGVsZW1lbnQgdG8gc2V0IHRoZSBjbGFzcyBvblxuICAgKiBAcGFyYW0ge3N0cmluZ30gY2xhc3NOYW1lIHRoZSBDU1MgY2xhc3NOYW1lXG4gICAqIEByZXR1cm4ge0RPTUVsZW1lbnR9IHRoZSBlbGVtZW50IHBhc3NlZCBpblxuICAgKi9cbiAgYWRkQ2xhc3M6IGZ1bmN0aW9uIChlbGVtZW50LCBjbGFzc05hbWUpIHtcbiAgICAhIS9cXHMvLnRlc3QoY2xhc3NOYW1lKSA/IHByb2Nlc3MuZW52Lk5PREVfRU5WICE9PSAncHJvZHVjdGlvbicgPyBpbnZhcmlhbnQoZmFsc2UsICdDU1NDb3JlLmFkZENsYXNzIHRha2VzIG9ubHkgYSBzaW5nbGUgY2xhc3MgbmFtZS4gXCIlc1wiIGNvbnRhaW5zICcgKyAnbXVsdGlwbGUgY2xhc3Nlcy4nLCBjbGFzc05hbWUpIDogaW52YXJpYW50KGZhbHNlKSA6IHVuZGVmaW5lZDtcblxuICAgIGlmIChjbGFzc05hbWUpIHtcbiAgICAgIGlmIChlbGVtZW50LmNsYXNzTGlzdCkge1xuICAgICAgICBlbGVtZW50LmNsYXNzTGlzdC5hZGQoY2xhc3NOYW1lKTtcbiAgICAgIH0gZWxzZSBpZiAoIUNTU0NvcmUuaGFzQ2xhc3MoZWxlbWVudCwgY2xhc3NOYW1lKSkge1xuICAgICAgICBlbGVtZW50LmNsYXNzTmFtZSA9IGVsZW1lbnQuY2xhc3NOYW1lICsgJyAnICsgY2xhc3NOYW1lO1xuICAgICAgfVxuICAgIH1cbiAgICByZXR1cm4gZWxlbWVudDtcbiAgfSxcblxuICAvKipcbiAgICogUmVtb3ZlcyB0aGUgY2xhc3MgcGFzc2VkIGluIGZyb20gdGhlIGVsZW1lbnRcbiAgICpcbiAgICogQHBhcmFtIHtET01FbGVtZW50fSBlbGVtZW50IHRoZSBlbGVtZW50IHRvIHNldCB0aGUgY2xhc3Mgb25cbiAgICogQHBhcmFtIHtzdHJpbmd9IGNsYXNzTmFtZSB0aGUgQ1NTIGNsYXNzTmFtZVxuICAgKiBAcmV0dXJuIHtET01FbGVtZW50fSB0aGUgZWxlbWVudCBwYXNzZWQgaW5cbiAgICovXG4gIHJlbW92ZUNsYXNzOiBmdW5jdGlvbiAoZWxlbWVudCwgY2xhc3NOYW1lKSB7XG4gICAgISEvXFxzLy50ZXN0KGNsYXNzTmFtZSkgPyBwcm9jZXNzLmVudi5OT0RFX0VOViAhPT0gJ3Byb2R1Y3Rpb24nID8gaW52YXJpYW50KGZhbHNlLCAnQ1NTQ29yZS5yZW1vdmVDbGFzcyB0YWtlcyBvbmx5IGEgc2luZ2xlIGNsYXNzIG5hbWUuIFwiJXNcIiBjb250YWlucyAnICsgJ211bHRpcGxlIGNsYXNzZXMuJywgY2xhc3NOYW1lKSA6IGludmFyaWFudChmYWxzZSkgOiB1bmRlZmluZWQ7XG5cbiAgICBpZiAoY2xhc3NOYW1lKSB7XG4gICAgICBpZiAoZWxlbWVudC5jbGFzc0xpc3QpIHtcbiAgICAgICAgZWxlbWVudC5jbGFzc0xpc3QucmVtb3ZlKGNsYXNzTmFtZSk7XG4gICAgICB9IGVsc2UgaWYgKENTU0NvcmUuaGFzQ2xhc3MoZWxlbWVudCwgY2xhc3NOYW1lKSkge1xuICAgICAgICBlbGVtZW50LmNsYXNzTmFtZSA9IGVsZW1lbnQuY2xhc3NOYW1lLnJlcGxhY2UobmV3IFJlZ0V4cCgnKF58XFxcXHMpJyArIGNsYXNzTmFtZSArICcoPzpcXFxcc3wkKScsICdnJyksICckMScpLnJlcGxhY2UoL1xccysvZywgJyAnKSAvLyBtdWx0aXBsZSBzcGFjZXMgdG8gb25lXG4gICAgICAgIC5yZXBsYWNlKC9eXFxzKnxcXHMqJC9nLCAnJyk7IC8vIHRyaW0gdGhlIGVuZHNcbiAgICAgIH1cbiAgICB9XG4gICAgcmV0dXJuIGVsZW1lbnQ7XG4gIH0sXG5cbiAgLyoqXG4gICAqIEhlbHBlciB0byBhZGQgb3IgcmVtb3ZlIGEgY2xhc3MgZnJvbSBhbiBlbGVtZW50IGJhc2VkIG9uIGEgY29uZGl0aW9uLlxuICAgKlxuICAgKiBAcGFyYW0ge0RPTUVsZW1lbnR9IGVsZW1lbnQgdGhlIGVsZW1lbnQgdG8gc2V0IHRoZSBjbGFzcyBvblxuICAgKiBAcGFyYW0ge3N0cmluZ30gY2xhc3NOYW1lIHRoZSBDU1MgY2xhc3NOYW1lXG4gICAqIEBwYXJhbSB7Kn0gYm9vbCBjb25kaXRpb24gdG8gd2hldGhlciB0byBhZGQgb3IgcmVtb3ZlIHRoZSBjbGFzc1xuICAgKiBAcmV0dXJuIHtET01FbGVtZW50fSB0aGUgZWxlbWVudCBwYXNzZWQgaW5cbiAgICovXG4gIGNvbmRpdGlvbkNsYXNzOiBmdW5jdGlvbiAoZWxlbWVudCwgY2xhc3NOYW1lLCBib29sKSB7XG4gICAgcmV0dXJuIChib29sID8gQ1NTQ29yZS5hZGRDbGFzcyA6IENTU0NvcmUucmVtb3ZlQ2xhc3MpKGVsZW1lbnQsIGNsYXNzTmFtZSk7XG4gIH0sXG5cbiAgLyoqXG4gICAqIFRlc3RzIHdoZXRoZXIgdGhlIGVsZW1lbnQgaGFzIHRoZSBjbGFzcyBzcGVjaWZpZWQuXG4gICAqXG4gICAqIEBwYXJhbSB7RE9NTm9kZXxET01XaW5kb3d9IGVsZW1lbnQgdGhlIGVsZW1lbnQgdG8gc2V0IHRoZSBjbGFzcyBvblxuICAgKiBAcGFyYW0ge3N0cmluZ30gY2xhc3NOYW1lIHRoZSBDU1MgY2xhc3NOYW1lXG4gICAqIEByZXR1cm4ge2Jvb2xlYW59IHRydWUgaWYgdGhlIGVsZW1lbnQgaGFzIHRoZSBjbGFzcywgZmFsc2UgaWYgbm90XG4gICAqL1xuICBoYXNDbGFzczogZnVuY3Rpb24gKGVsZW1lbnQsIGNsYXNzTmFtZSkge1xuICAgICEhL1xccy8udGVzdChjbGFzc05hbWUpID8gcHJvY2Vzcy5lbnYuTk9ERV9FTlYgIT09ICdwcm9kdWN0aW9uJyA/IGludmFyaWFudChmYWxzZSwgJ0NTUy5oYXNDbGFzcyB0YWtlcyBvbmx5IGEgc2luZ2xlIGNsYXNzIG5hbWUuJykgOiBpbnZhcmlhbnQoZmFsc2UpIDogdW5kZWZpbmVkO1xuICAgIGlmIChlbGVtZW50LmNsYXNzTGlzdCkge1xuICAgICAgcmV0dXJuICEhY2xhc3NOYW1lICYmIGVsZW1lbnQuY2xhc3NMaXN0LmNvbnRhaW5zKGNsYXNzTmFtZSk7XG4gICAgfVxuICAgIHJldHVybiAoJyAnICsgZWxlbWVudC5jbGFzc05hbWUgKyAnICcpLmluZGV4T2YoJyAnICsgY2xhc3NOYW1lICsgJyAnKSA+IC0xO1xuICB9XG5cbn07XG5cbm1vZHVsZS5leHBvcnRzID0gQ1NTQ29yZTtcblxuXG4vKioqKioqKioqKioqKioqKipcbiAqKiBXRUJQQUNLIEZPT1RFUlxuICoqIC4vfi9mYmpzL2xpYi9DU1NDb3JlLmpzXG4gKiogbW9kdWxlIGlkID0gMjg2XG4gKiogbW9kdWxlIGNodW5rcyA9IDFcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cyA9IHJlcXVpcmUoJ3JlYWN0L2xpYi9SZWFjdENTU1RyYW5zaXRpb25Hcm91cCcpO1xuXG5cbi8qKioqKioqKioqKioqKioqKlxuICoqIFdFQlBBQ0sgRk9PVEVSXG4gKiogLi9+L3JlYWN0LWFkZG9ucy1jc3MtdHJhbnNpdGlvbi1ncm91cC9pbmRleC5qc1xuICoqIG1vZHVsZSBpZCA9IDMxMFxuICoqIG1vZHVsZSBjaHVua3MgPSAxXG4gKiovIiwiLypcbiAqICBUaGUgTUlUIExpY2Vuc2UgKE1JVClcbiAqICBDb3B5cmlnaHQgKGMpIDIwMTUgUnlhbiBGbG9yZW5jZSwgTWljaGFlbCBKYWNrc29uXG4gKiAgUGVybWlzc2lvbiBpcyBoZXJlYnkgZ3JhbnRlZCwgZnJlZSBvZiBjaGFyZ2UsIHRvIGFueSBwZXJzb24gb2J0YWluaW5nIGEgY29weSBvZiB0aGlzIHNvZnR3YXJlIGFuZCBhc3NvY2lhdGVkIGRvY3VtZW50YXRpb24gZmlsZXMgKHRoZSBcIlNvZnR3YXJlXCIpLCB0byBkZWFsIGluIHRoZSBTb2Z0d2FyZSB3aXRob3V0IHJlc3RyaWN0aW9uLCBpbmNsdWRpbmcgd2l0aG91dCBsaW1pdGF0aW9uIHRoZSByaWdodHMgdG8gdXNlLCBjb3B5LCBtb2RpZnksIG1lcmdlLCBwdWJsaXNoLCBkaXN0cmlidXRlLCBzdWJsaWNlbnNlLCBhbmQvb3Igc2VsbCBjb3BpZXMgb2YgdGhlIFNvZnR3YXJlLCBhbmQgdG8gcGVybWl0IHBlcnNvbnMgdG8gd2hvbSB0aGUgU29mdHdhcmUgaXMgZnVybmlzaGVkIHRvIGRvIHNvLCBzdWJqZWN0IHRvIHRoZSBmb2xsb3dpbmcgY29uZGl0aW9uczpcbiAqICBUaGUgYWJvdmUgY29weXJpZ2h0IG5vdGljZSBhbmQgdGhpcyBwZXJtaXNzaW9uIG5vdGljZSBzaGFsbCBiZSBpbmNsdWRlZCBpbiBhbGwgY29waWVzIG9yIHN1YnN0YW50aWFsIHBvcnRpb25zIG9mIHRoZSBTb2Z0d2FyZS5cbiAqICBUSEUgU09GVFdBUkUgSVMgUFJPVklERUQgXCJBUyBJU1wiLCBXSVRIT1VUIFdBUlJBTlRZIE9GIEFOWSBLSU5ELCBFWFBSRVNTIE9SIElNUExJRUQsIElOQ0xVRElORyBCVVQgTk9UIExJTUlURUQgVE8gVEhFIFdBUlJBTlRJRVMgT0YgTUVSQ0hBTlRBQklMSVRZLCBGSVRORVNTIEZPUiBBIFBBUlRJQ1VMQVIgUFVSUE9TRSBBTkQgTk9OSU5GUklOR0VNRU5ULiBJTiBOTyBFVkVOVCBTSEFMTCBUSEUgQVVUSE9SUyBPUiBDT1BZUklHSFQgSE9MREVSUyBCRSBMSUFCTEUgRk9SIEFOWSBDTEFJTSwgREFNQUdFUyBPUiBPVEhFUiBMSUFCSUxJVFksIFdIRVRIRVIgSU4gQU4gQUNUSU9OIE9GIENPTlRSQUNULCBUT1JUIE9SIE9USEVSV0lTRSwgQVJJU0lORyBGUk9NLCBPVVQgT0YgT1IgSU4gQ09OTkVDVElPTiBXSVRIIFRIRSBTT0ZUV0FSRSBPUiBUSEUgVVNFIE9SIE9USEVSIERFQUxJTkdTIElOIFRIRSBTT0ZUV0FSRS5cbiovXG5cbmltcG9ydCBpbnZhcmlhbnQgZnJvbSAnaW52YXJpYW50J1xuXG5mdW5jdGlvbiBlc2NhcGVSZWdFeHAoc3RyaW5nKSB7XG4gIHJldHVybiBzdHJpbmcucmVwbGFjZSgvWy4qKz9eJHt9KCl8W1xcXVxcXFxdL2csICdcXFxcJCYnKVxufVxuXG5mdW5jdGlvbiBlc2NhcGVTb3VyY2Uoc3RyaW5nKSB7XG4gIHJldHVybiBlc2NhcGVSZWdFeHAoc3RyaW5nKS5yZXBsYWNlKC9cXC8rL2csICcvKycpXG59XG5cbmZ1bmN0aW9uIF9jb21waWxlUGF0dGVybihwYXR0ZXJuKSB7XG4gIGxldCByZWdleHBTb3VyY2UgPSAnJztcbiAgY29uc3QgcGFyYW1OYW1lcyA9IFtdO1xuICBjb25zdCB0b2tlbnMgPSBbXTtcblxuICBsZXQgbWF0Y2gsIGxhc3RJbmRleCA9IDAsIG1hdGNoZXIgPSAvOihbYS16QS1aXyRdW2EtekEtWjAtOV8kXSopfFxcKlxcKnxcXCp8XFwofFxcKS9nXG4gIC8qZXNsaW50IG5vLWNvbmQtYXNzaWduOiAwKi9cbiAgd2hpbGUgKChtYXRjaCA9IG1hdGNoZXIuZXhlYyhwYXR0ZXJuKSkpIHtcbiAgICBpZiAobWF0Y2guaW5kZXggIT09IGxhc3RJbmRleCkge1xuICAgICAgdG9rZW5zLnB1c2gocGF0dGVybi5zbGljZShsYXN0SW5kZXgsIG1hdGNoLmluZGV4KSlcbiAgICAgIHJlZ2V4cFNvdXJjZSArPSBlc2NhcGVTb3VyY2UocGF0dGVybi5zbGljZShsYXN0SW5kZXgsIG1hdGNoLmluZGV4KSlcbiAgICB9XG5cbiAgICBpZiAobWF0Y2hbMV0pIHtcbiAgICAgIHJlZ2V4cFNvdXJjZSArPSAnKFteLz8jXSspJztcbiAgICAgIHBhcmFtTmFtZXMucHVzaChtYXRjaFsxXSk7XG4gICAgfSBlbHNlIGlmIChtYXRjaFswXSA9PT0gJyoqJykge1xuICAgICAgcmVnZXhwU291cmNlICs9ICcoW1xcXFxzXFxcXFNdKiknXG4gICAgICBwYXJhbU5hbWVzLnB1c2goJ3NwbGF0Jyk7XG4gICAgfSBlbHNlIGlmIChtYXRjaFswXSA9PT0gJyonKSB7XG4gICAgICByZWdleHBTb3VyY2UgKz0gJyhbXFxcXHNcXFxcU10qPyknXG4gICAgICBwYXJhbU5hbWVzLnB1c2goJ3NwbGF0Jyk7XG4gICAgfSBlbHNlIGlmIChtYXRjaFswXSA9PT0gJygnKSB7XG4gICAgICByZWdleHBTb3VyY2UgKz0gJyg/Oic7XG4gICAgfSBlbHNlIGlmIChtYXRjaFswXSA9PT0gJyknKSB7XG4gICAgICByZWdleHBTb3VyY2UgKz0gJyk/JztcbiAgICB9XG5cbiAgICB0b2tlbnMucHVzaChtYXRjaFswXSk7XG5cbiAgICBsYXN0SW5kZXggPSBtYXRjaGVyLmxhc3RJbmRleDtcbiAgfVxuXG4gIGlmIChsYXN0SW5kZXggIT09IHBhdHRlcm4ubGVuZ3RoKSB7XG4gICAgdG9rZW5zLnB1c2gocGF0dGVybi5zbGljZShsYXN0SW5kZXgsIHBhdHRlcm4ubGVuZ3RoKSlcbiAgICByZWdleHBTb3VyY2UgKz0gZXNjYXBlU291cmNlKHBhdHRlcm4uc2xpY2UobGFzdEluZGV4LCBwYXR0ZXJuLmxlbmd0aCkpXG4gIH1cblxuICByZXR1cm4ge1xuICAgIHBhdHRlcm4sXG4gICAgcmVnZXhwU291cmNlLFxuICAgIHBhcmFtTmFtZXMsXG4gICAgdG9rZW5zXG4gIH1cbn1cblxuY29uc3QgQ29tcGlsZWRQYXR0ZXJuc0NhY2hlID0ge31cblxuZXhwb3J0IGZ1bmN0aW9uIGNvbXBpbGVQYXR0ZXJuKHBhdHRlcm4pIHtcbiAgaWYgKCEocGF0dGVybiBpbiBDb21waWxlZFBhdHRlcm5zQ2FjaGUpKVxuICAgIENvbXBpbGVkUGF0dGVybnNDYWNoZVtwYXR0ZXJuXSA9IF9jb21waWxlUGF0dGVybihwYXR0ZXJuKVxuXG4gIHJldHVybiBDb21waWxlZFBhdHRlcm5zQ2FjaGVbcGF0dGVybl1cbn1cblxuLyoqXG4gKiBBdHRlbXB0cyB0byBtYXRjaCBhIHBhdHRlcm4gb24gdGhlIGdpdmVuIHBhdGhuYW1lLiBQYXR0ZXJucyBtYXkgdXNlXG4gKiB0aGUgZm9sbG93aW5nIHNwZWNpYWwgY2hhcmFjdGVyczpcbiAqXG4gKiAtIDpwYXJhbU5hbWUgICAgIE1hdGNoZXMgYSBVUkwgc2VnbWVudCB1cCB0byB0aGUgbmV4dCAvLCA/LCBvciAjLiBUaGVcbiAqICAgICAgICAgICAgICAgICAgY2FwdHVyZWQgc3RyaW5nIGlzIGNvbnNpZGVyZWQgYSBcInBhcmFtXCJcbiAqIC0gKCkgICAgICAgICAgICAgV3JhcHMgYSBzZWdtZW50IG9mIHRoZSBVUkwgdGhhdCBpcyBvcHRpb25hbFxuICogLSAqICAgICAgICAgICAgICBDb25zdW1lcyAobm9uLWdyZWVkeSkgYWxsIGNoYXJhY3RlcnMgdXAgdG8gdGhlIG5leHRcbiAqICAgICAgICAgICAgICAgICAgY2hhcmFjdGVyIGluIHRoZSBwYXR0ZXJuLCBvciB0byB0aGUgZW5kIG9mIHRoZSBVUkwgaWZcbiAqICAgICAgICAgICAgICAgICAgdGhlcmUgaXMgbm9uZVxuICogLSAqKiAgICAgICAgICAgICBDb25zdW1lcyAoZ3JlZWR5KSBhbGwgY2hhcmFjdGVycyB1cCB0byB0aGUgbmV4dCBjaGFyYWN0ZXJcbiAqICAgICAgICAgICAgICAgICAgaW4gdGhlIHBhdHRlcm4sIG9yIHRvIHRoZSBlbmQgb2YgdGhlIFVSTCBpZiB0aGVyZSBpcyBub25lXG4gKlxuICogVGhlIHJldHVybiB2YWx1ZSBpcyBhbiBvYmplY3Qgd2l0aCB0aGUgZm9sbG93aW5nIHByb3BlcnRpZXM6XG4gKlxuICogLSByZW1haW5pbmdQYXRobmFtZVxuICogLSBwYXJhbU5hbWVzXG4gKiAtIHBhcmFtVmFsdWVzXG4gKi9cbmV4cG9ydCBmdW5jdGlvbiBtYXRjaFBhdHRlcm4ocGF0dGVybiwgcGF0aG5hbWUpIHtcbiAgLy8gTWFrZSBsZWFkaW5nIHNsYXNoZXMgY29uc2lzdGVudCBiZXR3ZWVuIHBhdHRlcm4gYW5kIHBhdGhuYW1lLlxuICBpZiAocGF0dGVybi5jaGFyQXQoMCkgIT09ICcvJykge1xuICAgIHBhdHRlcm4gPSBgLyR7cGF0dGVybn1gXG4gIH1cbiAgaWYgKHBhdGhuYW1lLmNoYXJBdCgwKSAhPT0gJy8nKSB7XG4gICAgcGF0aG5hbWUgPSBgLyR7cGF0aG5hbWV9YFxuICB9XG5cbiAgbGV0IHsgcmVnZXhwU291cmNlLCBwYXJhbU5hbWVzLCB0b2tlbnMgfSA9IGNvbXBpbGVQYXR0ZXJuKHBhdHRlcm4pXG5cbiAgcmVnZXhwU291cmNlICs9ICcvKicgLy8gQ2FwdHVyZSBwYXRoIHNlcGFyYXRvcnNcblxuICAvLyBTcGVjaWFsLWNhc2UgcGF0dGVybnMgbGlrZSAnKicgZm9yIGNhdGNoLWFsbCByb3V0ZXMuXG4gIGNvbnN0IGNhcHR1cmVSZW1haW5pbmcgPSB0b2tlbnNbdG9rZW5zLmxlbmd0aCAtIDFdICE9PSAnKidcblxuICBpZiAoY2FwdHVyZVJlbWFpbmluZykge1xuICAgIC8vIFRoaXMgd2lsbCBtYXRjaCBuZXdsaW5lcyBpbiB0aGUgcmVtYWluaW5nIHBhdGguXG4gICAgcmVnZXhwU291cmNlICs9ICcoW1xcXFxzXFxcXFNdKj8pJ1xuICB9XG5cbiAgY29uc3QgbWF0Y2ggPSBwYXRobmFtZS5tYXRjaChuZXcgUmVnRXhwKCdeJyArIHJlZ2V4cFNvdXJjZSArICckJywgJ2knKSlcblxuICBsZXQgcmVtYWluaW5nUGF0aG5hbWUsIHBhcmFtVmFsdWVzXG4gIGlmIChtYXRjaCAhPSBudWxsKSB7XG4gICAgaWYgKGNhcHR1cmVSZW1haW5pbmcpIHtcbiAgICAgIHJlbWFpbmluZ1BhdGhuYW1lID0gbWF0Y2gucG9wKClcbiAgICAgIGNvbnN0IG1hdGNoZWRQYXRoID1cbiAgICAgICAgbWF0Y2hbMF0uc3Vic3RyKDAsIG1hdGNoWzBdLmxlbmd0aCAtIHJlbWFpbmluZ1BhdGhuYW1lLmxlbmd0aClcblxuICAgICAgLy8gSWYgd2UgZGlkbid0IG1hdGNoIHRoZSBlbnRpcmUgcGF0aG5hbWUsIHRoZW4gbWFrZSBzdXJlIHRoYXQgdGhlIG1hdGNoXG4gICAgICAvLyB3ZSBkaWQgZ2V0IGVuZHMgYXQgYSBwYXRoIHNlcGFyYXRvciAocG90ZW50aWFsbHkgdGhlIG9uZSB3ZSBhZGRlZFxuICAgICAgLy8gYWJvdmUgYXQgdGhlIGJlZ2lubmluZyBvZiB0aGUgcGF0aCwgaWYgdGhlIGFjdHVhbCBtYXRjaCB3YXMgZW1wdHkpLlxuICAgICAgaWYgKFxuICAgICAgICByZW1haW5pbmdQYXRobmFtZSAmJlxuICAgICAgICBtYXRjaGVkUGF0aC5jaGFyQXQobWF0Y2hlZFBhdGgubGVuZ3RoIC0gMSkgIT09ICcvJ1xuICAgICAgKSB7XG4gICAgICAgIHJldHVybiB7XG4gICAgICAgICAgcmVtYWluaW5nUGF0aG5hbWU6IG51bGwsXG4gICAgICAgICAgcGFyYW1OYW1lcyxcbiAgICAgICAgICBwYXJhbVZhbHVlczogbnVsbFxuICAgICAgICB9XG4gICAgICB9XG4gICAgfSBlbHNlIHtcbiAgICAgIC8vIElmIHRoaXMgbWF0Y2hlZCBhdCBhbGwsIHRoZW4gdGhlIG1hdGNoIHdhcyB0aGUgZW50aXJlIHBhdGhuYW1lLlxuICAgICAgcmVtYWluaW5nUGF0aG5hbWUgPSAnJ1xuICAgIH1cblxuICAgIHBhcmFtVmFsdWVzID0gbWF0Y2guc2xpY2UoMSkubWFwKFxuICAgICAgdiA9PiB2ICE9IG51bGwgPyBkZWNvZGVVUklDb21wb25lbnQodikgOiB2XG4gICAgKVxuICB9IGVsc2Uge1xuICAgIHJlbWFpbmluZ1BhdGhuYW1lID0gcGFyYW1WYWx1ZXMgPSBudWxsXG4gIH1cblxuICByZXR1cm4ge1xuICAgIHJlbWFpbmluZ1BhdGhuYW1lLFxuICAgIHBhcmFtTmFtZXMsXG4gICAgcGFyYW1WYWx1ZXNcbiAgfVxufVxuXG5leHBvcnQgZnVuY3Rpb24gZ2V0UGFyYW1OYW1lcyhwYXR0ZXJuKSB7XG4gIHJldHVybiBjb21waWxlUGF0dGVybihwYXR0ZXJuKS5wYXJhbU5hbWVzXG59XG5cbmV4cG9ydCBmdW5jdGlvbiBnZXRQYXJhbXMocGF0dGVybiwgcGF0aG5hbWUpIHtcbiAgY29uc3QgeyBwYXJhbU5hbWVzLCBwYXJhbVZhbHVlcyB9ID0gbWF0Y2hQYXR0ZXJuKHBhdHRlcm4sIHBhdGhuYW1lKVxuXG4gIGlmIChwYXJhbVZhbHVlcyAhPSBudWxsKSB7XG4gICAgcmV0dXJuIHBhcmFtTmFtZXMucmVkdWNlKGZ1bmN0aW9uIChtZW1vLCBwYXJhbU5hbWUsIGluZGV4KSB7XG4gICAgICBtZW1vW3BhcmFtTmFtZV0gPSBwYXJhbVZhbHVlc1tpbmRleF1cbiAgICAgIHJldHVybiBtZW1vXG4gICAgfSwge30pXG4gIH1cblxuICByZXR1cm4gbnVsbFxufVxuXG4vKipcbiAqIFJldHVybnMgYSB2ZXJzaW9uIG9mIHRoZSBnaXZlbiBwYXR0ZXJuIHdpdGggcGFyYW1zIGludGVycG9sYXRlZC4gVGhyb3dzXG4gKiBpZiB0aGVyZSBpcyBhIGR5bmFtaWMgc2VnbWVudCBvZiB0aGUgcGF0dGVybiBmb3Igd2hpY2ggdGhlcmUgaXMgbm8gcGFyYW0uXG4gKi9cbmV4cG9ydCBmdW5jdGlvbiBmb3JtYXRQYXR0ZXJuKHBhdHRlcm4sIHBhcmFtcykge1xuICBwYXJhbXMgPSBwYXJhbXMgfHwge31cblxuICBjb25zdCB7IHRva2VucyB9ID0gY29tcGlsZVBhdHRlcm4ocGF0dGVybilcbiAgbGV0IHBhcmVuQ291bnQgPSAwLCBwYXRobmFtZSA9ICcnLCBzcGxhdEluZGV4ID0gMFxuXG4gIGxldCB0b2tlbiwgcGFyYW1OYW1lLCBwYXJhbVZhbHVlXG4gIGZvciAobGV0IGkgPSAwLCBsZW4gPSB0b2tlbnMubGVuZ3RoOyBpIDwgbGVuOyArK2kpIHtcbiAgICB0b2tlbiA9IHRva2Vuc1tpXVxuXG4gICAgaWYgKHRva2VuID09PSAnKicgfHwgdG9rZW4gPT09ICcqKicpIHtcbiAgICAgIHBhcmFtVmFsdWUgPSBBcnJheS5pc0FycmF5KHBhcmFtcy5zcGxhdCkgPyBwYXJhbXMuc3BsYXRbc3BsYXRJbmRleCsrXSA6IHBhcmFtcy5zcGxhdFxuXG4gICAgICBpbnZhcmlhbnQoXG4gICAgICAgIHBhcmFtVmFsdWUgIT0gbnVsbCB8fCBwYXJlbkNvdW50ID4gMCxcbiAgICAgICAgJ01pc3Npbmcgc3BsYXQgIyVzIGZvciBwYXRoIFwiJXNcIicsXG4gICAgICAgIHNwbGF0SW5kZXgsIHBhdHRlcm5cbiAgICAgIClcblxuICAgICAgaWYgKHBhcmFtVmFsdWUgIT0gbnVsbClcbiAgICAgICAgcGF0aG5hbWUgKz0gZW5jb2RlVVJJKHBhcmFtVmFsdWUpXG4gICAgfSBlbHNlIGlmICh0b2tlbiA9PT0gJygnKSB7XG4gICAgICBwYXJlbkNvdW50ICs9IDFcbiAgICB9IGVsc2UgaWYgKHRva2VuID09PSAnKScpIHtcbiAgICAgIHBhcmVuQ291bnQgLT0gMVxuICAgIH0gZWxzZSBpZiAodG9rZW4uY2hhckF0KDApID09PSAnOicpIHtcbiAgICAgIHBhcmFtTmFtZSA9IHRva2VuLnN1YnN0cmluZygxKVxuICAgICAgcGFyYW1WYWx1ZSA9IHBhcmFtc1twYXJhbU5hbWVdXG5cbiAgICAgIGludmFyaWFudChcbiAgICAgICAgcGFyYW1WYWx1ZSAhPSBudWxsIHx8IHBhcmVuQ291bnQgPiAwLFxuICAgICAgICAnTWlzc2luZyBcIiVzXCIgcGFyYW1ldGVyIGZvciBwYXRoIFwiJXNcIicsXG4gICAgICAgIHBhcmFtTmFtZSwgcGF0dGVyblxuICAgICAgKVxuXG4gICAgICBpZiAocGFyYW1WYWx1ZSAhPSBudWxsKVxuICAgICAgICBwYXRobmFtZSArPSBlbmNvZGVVUklDb21wb25lbnQocGFyYW1WYWx1ZSlcbiAgICB9IGVsc2Uge1xuICAgICAgcGF0aG5hbWUgKz0gdG9rZW5cbiAgICB9XG4gIH1cblxuICByZXR1cm4gcGF0aG5hbWUucmVwbGFjZSgvXFwvKy9nLCAnLycpXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tbW9uL3BhdHRlcm5VdGlscy5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIEV2ZW50RW1pdHRlciA9IHJlcXVpcmUoJ2V2ZW50cycpLkV2ZW50RW1pdHRlcjtcblxuY29uc3QgbG9nZ2VyID0gcmVxdWlyZSgnLi9sb2dnZXInKS5jcmVhdGUoJ1R0eUV2ZW50cycpO1xuXG5jbGFzcyBUdHlFdmVudHMgZXh0ZW5kcyBFdmVudEVtaXR0ZXIge1xuXG4gIGNvbnN0cnVjdG9yKCl7XG4gICAgc3VwZXIoKTtcbiAgICB0aGlzLnNvY2tldCA9IG51bGw7XG4gIH1cblxuICBjb25uZWN0KGNvbm5TdHIpe1xuICAgIHRoaXMuc29ja2V0ID0gbmV3IFdlYlNvY2tldChjb25uU3RyLCAncHJvdG8nKTtcblxuICAgIHRoaXMuc29ja2V0Lm9ub3BlbiA9ICgpID0+IHtcbiAgICAgIGxvZ2dlci5pbmZvKCdUdHkgZXZlbnQgc3RyZWFtIGlzIG9wZW4nKTtcbiAgICB9XG5cbiAgICB0aGlzLnNvY2tldC5vbm1lc3NhZ2UgPSAoZXZlbnQpID0+IHtcbiAgICAgIHRyeVxuICAgICAge1xuICAgICAgICBsZXQganNvbiA9IEpTT04ucGFyc2UoZXZlbnQuZGF0YSk7XG4gICAgICAgIHRoaXMuZW1pdCgnZGF0YScsIGpzb24uc2Vzc2lvbik7XG4gICAgICB9XG4gICAgICBjYXRjaChlcnIpe1xuICAgICAgICBsb2dnZXIuZXJyb3IoJ2ZhaWxlZCB0byBwYXJzZSBldmVudCBzdHJlYW0gZGF0YScsIGVycik7XG4gICAgICB9XG4gICAgfTtcblxuICAgIHRoaXMuc29ja2V0Lm9uY2xvc2UgPSAoKSA9PiB7XG4gICAgICBsb2dnZXIuaW5mbygnVHR5IGV2ZW50IHN0cmVhbSBpcyBjbG9zZWQnKTtcbiAgICB9O1xuICB9XG5cbiAgZGlzY29ubmVjdCgpe1xuICAgIHRoaXMuc29ja2V0LmNsb3NlKCk7XG4gIH1cblxufVxuXG5tb2R1bGUuZXhwb3J0cyA9IFR0eUV2ZW50cztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21tb24vdHR5RXZlbnRzLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgVHR5ID0gcmVxdWlyZSgnYXBwL2NvbW1vbi90dHknKTtcbnZhciBhcGkgPSByZXF1aXJlKCdhcHAvc2VydmljZXMvYXBpJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xudmFyIHtzaG93RXJyb3J9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvbm90aWZpY2F0aW9ucy9hY3Rpb25zJyk7XG52YXIgQnVmZmVyID0gcmVxdWlyZSgnYnVmZmVyLycpLkJ1ZmZlcjtcblxuY29uc3QgbG9nZ2VyID0gcmVxdWlyZSgnYXBwL2NvbW1vbi9sb2dnZXInKS5jcmVhdGUoJ1R0eVBsYXllcicpO1xuY29uc3QgU1RSRUFNX1NUQVJUX0lOREVYID0gMTtcbmNvbnN0IFBSRV9GRVRDSF9CVUZfU0laRSA9IDUwMDA7XG5cbmZ1bmN0aW9uIGhhbmRsZUFqYXhFcnJvcihlcnIpe1xuICBzaG93RXJyb3IoJ1VuYWJsZSB0byByZXRyaWV2ZSBzZXNzaW9uIGluZm8nKTtcbiAgbG9nZ2VyLmVycm9yKCdmZXRjaGluZyBzZXNzaW9uIGxlbmd0aCcsIGVycik7XG59XG5cbmNsYXNzIFR0eVBsYXllciBleHRlbmRzIFR0eSB7XG4gIGNvbnN0cnVjdG9yKHtzaWR9KXtcbiAgICBzdXBlcih7fSk7XG4gICAgdGhpcy5zaWQgPSBzaWQ7XG4gICAgdGhpcy5jdXJyZW50ID0gU1RSRUFNX1NUQVJUX0lOREVYO1xuICAgIHRoaXMubGVuZ3RoID0gLTE7XG4gICAgdGhpcy50dHlTdHJlYW0gPSBuZXcgQXJyYXkoKTtcbiAgICB0aGlzLmlzUGxheWluZyA9IGZhbHNlO1xuICAgIHRoaXMuaXNFcnJvciA9IGZhbHNlO1xuICAgIHRoaXMuaXNSZWFkeSA9IGZhbHNlO1xuICAgIHRoaXMuaXNMb2FkaW5nID0gdHJ1ZTtcbiAgfVxuXG4gIHNlbmQoKXtcbiAgfVxuXG4gIHJlc2l6ZSgpe1xuICB9XG5cbiAgZ2V0RGltZW5zaW9ucygpe1xuICAgIGxldCBjaHVua0luZm8gPSB0aGlzLnR0eVN0cmVhbVt0aGlzLmN1cnJlbnQtMV07XG4gICAgaWYoY2h1bmtJbmZvKXtcbiAgICAgICByZXR1cm4ge1xuICAgICAgICAgdzogY2h1bmtJbmZvLncsXG4gICAgICAgICBoOiBjaHVua0luZm8uaFxuICAgICAgIH1cbiAgICB9ZWxzZXtcbiAgICAgIHJldHVybiB7dzogdW5kZWZpbmVkLCBoOiB1bmRlZmluZWR9O1xuICAgIH1cbiAgfVxuXG4gIGNvbm5lY3QoKXtcbiAgICB0aGlzLl9zZXRTdGF0dXNGbGFnKHtpc0xvYWRpbmc6IHRydWV9KTtcblxuXG4gICAgYXBpLmdldChgL3YxL3dlYmFwaS9zaXRlcy8tY3VyZW50LS9zZXNzaW9ucy8ke3RoaXMuc2lkfS9ldmVudHNgKTtcblxuICAgIGFwaS5nZXQoY2ZnLmFwaS5nZXRGZXRjaFNlc3Npb25MZW5ndGhVcmwodGhpcy5zaWQpKVxuICAgICAgLmRvbmUoKGRhdGEpPT57XG4gICAgICAgIC8qXG4gICAgICAgICogdGVtcG9yYXJ5IGhvdGZpeCB0byBiYWNrLWVuZCBpc3N1ZSByZWxhdGVkIHRvIHNlc3Npb24gY2h1bmtzIHN0YXJ0aW5nIGF0XG4gICAgICAgICogaW5kZXg9MSBhbmQgZW5kaW5nIGF0IGluZGV4PWxlbmd0aCsxXG4gICAgICAgICoqL1xuICAgICAgICB0aGlzLmxlbmd0aCA9IGRhdGEuY291bnQrMTtcbiAgICAgICAgdGhpcy5fc2V0U3RhdHVzRmxhZyh7aXNSZWFkeTogdHJ1ZX0pO1xuICAgICAgfSlcbiAgICAgIC5mYWlsKChlcnIpPT57XG4gICAgICAgIGhhbmRsZUFqYXhFcnJvcihlcnIpO1xuICAgICAgfSlcbiAgICAgIC5hbHdheXMoKCk9PntcbiAgICAgICAgdGhpcy5fY2hhbmdlKCk7XG4gICAgICB9KTtcblxuICAgIHRoaXMuX2NoYW5nZSgpO1xuICB9XG5cbiAgbW92ZShuZXdQb3Mpe1xuICAgIGlmKCF0aGlzLmlzUmVhZHkpe1xuICAgICAgcmV0dXJuO1xuICAgIH1cblxuICAgIGlmKG5ld1BvcyA9PT0gdW5kZWZpbmVkKXtcbiAgICAgIG5ld1BvcyA9IHRoaXMuY3VycmVudCArIDE7XG4gICAgfVxuXG4gICAgaWYobmV3UG9zID4gdGhpcy5sZW5ndGgpe1xuICAgICAgbmV3UG9zID0gdGhpcy5sZW5ndGg7XG4gICAgICB0aGlzLnN0b3AoKTtcbiAgICB9XG5cbiAgICBpZihuZXdQb3MgPT09IDApe1xuICAgICAgbmV3UG9zID0gU1RSRUFNX1NUQVJUX0lOREVYO1xuICAgIH1cblxuICAgIGlmKHRoaXMuY3VycmVudCA8IG5ld1Bvcyl7XG4gICAgICB0aGlzLl9zaG93Q2h1bmsodGhpcy5jdXJyZW50LCBuZXdQb3MpO1xuICAgIH1lbHNle1xuICAgICAgdGhpcy5lbWl0KCdyZXNldCcpO1xuICAgICAgdGhpcy5fc2hvd0NodW5rKFNUUkVBTV9TVEFSVF9JTkRFWCwgbmV3UG9zKTtcbiAgICB9XG5cbiAgICB0aGlzLl9jaGFuZ2UoKTtcbiAgfVxuXG4gIHN0b3AoKXtcbiAgICB0aGlzLmlzUGxheWluZyA9IGZhbHNlO1xuICAgIHRoaXMudGltZXIgPSBjbGVhckludGVydmFsKHRoaXMudGltZXIpO1xuICAgIHRoaXMuX2NoYW5nZSgpO1xuICB9XG5cbiAgcGxheSgpe1xuICAgIGlmKHRoaXMuaXNQbGF5aW5nKXtcbiAgICAgIHJldHVybjtcbiAgICB9XG5cbiAgICB0aGlzLmlzUGxheWluZyA9IHRydWU7XG5cbiAgICAvLyBzdGFydCBmcm9tIHRoZSBiZWdpbm5pbmcgaWYgYXQgdGhlIGVuZFxuICAgIGlmKHRoaXMuY3VycmVudCA9PT0gdGhpcy5sZW5ndGgpe1xuICAgICAgdGhpcy5jdXJyZW50ID0gU1RSRUFNX1NUQVJUX0lOREVYO1xuICAgICAgdGhpcy5lbWl0KCdyZXNldCcpO1xuICAgIH1cblxuICAgIHRoaXMudGltZXIgPSBzZXRJbnRlcnZhbCh0aGlzLm1vdmUuYmluZCh0aGlzKSwgMTUwKTtcbiAgICB0aGlzLl9jaGFuZ2UoKTtcbiAgfVxuXG4gIF9zaG91bGRGZXRjaChzdGFydCwgZW5kKXtcbiAgICBmb3IodmFyIGkgPSBzdGFydDsgaSA8IGVuZDsgaSsrKXtcbiAgICAgIGlmKHRoaXMudHR5U3RyZWFtW2ldID09PSB1bmRlZmluZWQpe1xuICAgICAgICByZXR1cm4gdHJ1ZTtcbiAgICAgIH1cbiAgICB9XG5cbiAgICByZXR1cm4gZmFsc2U7XG4gIH1cblxuICBfZmV0Y2goc3RhcnQsIGVuZCl7XG4gICAgZW5kID0gZW5kICsgUFJFX0ZFVENIX0JVRl9TSVpFO1xuICAgIGVuZCA9IGVuZCA+IHRoaXMubGVuZ3RoID8gdGhpcy5sZW5ndGggOiBlbmQ7XG5cbiAgICB0aGlzLl9zZXRTdGF0dXNGbGFnKHtpc0xvYWRpbmc6IHRydWUgfSk7XG5cbiAgICByZXR1cm4gYXBpLmdldChjZmcuYXBpLmdldEZldGNoU2Vzc2lvbkNodW5rVXJsKHtzaWQ6IHRoaXMuc2lkLCBzdGFydCwgZW5kfSkpLlxuICAgICAgZG9uZSgocmVzcG9uc2UpPT57XG4gICAgICAgIGZvcih2YXIgaSA9IDA7IGkgPCBlbmQtc3RhcnQ7IGkrKyl7XG4gICAgICAgICAgbGV0IHtkYXRhLCBkZWxheSwgdGVybToge2gsIHd9fSA9IHJlc3BvbnNlLmNodW5rc1tpXTtcbiAgICAgICAgICBkYXRhID0gbmV3IEJ1ZmZlcihkYXRhLCAnYmFzZTY0JykudG9TdHJpbmcoJ3V0ZjgnKTtcbiAgICAgICAgICB0aGlzLnR0eVN0cmVhbVtzdGFydCtpXSA9IHsgZGF0YSwgZGVsYXksIHcsIGggfTtcbiAgICAgICAgfVxuXG4gICAgICAgIHRoaXMuX3NldFN0YXR1c0ZsYWcoe2lzUmVhZHk6IHRydWUgfSk7XG4gICAgICB9KVxuICAgICAgLmZhaWwoKGVycik9PntcbiAgICAgICAgaGFuZGxlQWpheEVycm9yKGVycik7XG4gICAgICAgIHRoaXMuX3NldFN0YXR1c0ZsYWcoe2lzRXJyb3I6IHRydWUgfSk7XG4gICAgICB9KVxuICB9XG5cbiAgX2Rpc3BsYXkoc3RhcnQsIGVuZCl7XG4gICAgbGV0IHN0cmVhbSA9IHRoaXMudHR5U3RyZWFtO1xuICAgIGxldCBpO1xuICAgIGxldCB0bXAgPSBbe1xuICAgICAgZGF0YTogW3N0cmVhbVtzdGFydF0uZGF0YV0sXG4gICAgICB3OiBzdHJlYW1bc3RhcnRdLncsXG4gICAgICBoOiBzdHJlYW1bc3RhcnRdLmhcbiAgICB9XTtcblxuICAgIGxldCBjdXIgPSB0bXBbMF07XG5cbiAgICBmb3IoaSA9IHN0YXJ0KzE7IGkgPCBlbmQ7IGkrKyl7XG4gICAgICBpZihjdXIudyA9PT0gc3RyZWFtW2ldLncgJiYgY3VyLmggPT09IHN0cmVhbVtpXS5oKXtcbiAgICAgICAgY3VyLmRhdGEucHVzaChzdHJlYW1baV0uZGF0YSlcbiAgICAgIH1lbHNle1xuICAgICAgICBjdXIgPXtcbiAgICAgICAgICBkYXRhOiBbc3RyZWFtW2ldLmRhdGFdLFxuICAgICAgICAgIHc6IHN0cmVhbVtpXS53LFxuICAgICAgICAgIGg6IHN0cmVhbVtpXS5oXG4gICAgICAgIH07XG5cbiAgICAgICAgdG1wLnB1c2goY3VyKTtcbiAgICAgIH1cbiAgICB9XG5cbiAgICBmb3IoaSA9IDA7IGkgPCB0bXAubGVuZ3RoOyBpICsrKXtcbiAgICAgIGxldCBzdHIgPSB0bXBbaV0uZGF0YS5qb2luKCcnKTtcbiAgICAgIGxldCB7aCwgd30gPSB0bXBbaV07XG4gICAgICB0aGlzLmVtaXQoJ3Jlc2l6ZScsIHtoLCB3fSk7XG4gICAgICB0aGlzLmVtaXQoJ2RhdGEnLCBzdHIpO1xuICAgIH1cblxuICAgIHRoaXMuY3VycmVudCA9IGVuZDtcbiAgfVxuXG4gIF9zaG93Q2h1bmsoc3RhcnQsIGVuZCl7XG4gICAgaWYodGhpcy5fc2hvdWxkRmV0Y2goc3RhcnQsIGVuZCkpe1xuICAgICAgdGhpcy5fZmV0Y2goc3RhcnQsIGVuZCkudGhlbigoKT0+XG4gICAgICAgIHRoaXMuX2Rpc3BsYXkoc3RhcnQsIGVuZCkpO1xuICAgIH1lbHNle1xuICAgICAgdGhpcy5fZGlzcGxheShzdGFydCwgZW5kKTtcbiAgICB9XG4gIH1cblxuICBfc2V0U3RhdHVzRmxhZyhuZXdTdGF0dXMpe1xuICAgIGxldCB7aXNSZWFkeT1mYWxzZSwgaXNFcnJvcj1mYWxzZSwgaXNMb2FkaW5nPWZhbHNlIH0gPSBuZXdTdGF0dXM7XG5cbiAgICB0aGlzLmlzUmVhZHkgPSBpc1JlYWR5O1xuICAgIHRoaXMuaXNFcnJvciA9IGlzRXJyb3I7XG4gICAgdGhpcy5pc0xvYWRpbmcgPSBpc0xvYWRpbmc7XG4gIH1cblxuICBfY2hhbmdlKCl7XG4gICAgdGhpcy5lbWl0KCdjaGFuZ2UnKTtcbiAgfVxufVxuXG5leHBvcnQgZGVmYXVsdCBUdHlQbGF5ZXI7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tbW9uL3R0eVBsYXllci5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciBOYXZMZWZ0QmFyID0gcmVxdWlyZSgnLi9uYXZMZWZ0QmFyJyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIge2FjdGlvbnMsIGdldHRlcnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvYXBwJyk7XG52YXIgU2VsZWN0Tm9kZURpYWxvZyA9IHJlcXVpcmUoJy4vc2VsZWN0Tm9kZURpYWxvZy5qc3gnKTtcbnZhciBOb3RpZmljYXRpb25Ib3N0ID0gcmVxdWlyZSgnLi9ub3RpZmljYXRpb25Ib3N0LmpzeCcpO1xuXG52YXIgQXBwID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW3JlYWN0b3IuUmVhY3RNaXhpbl0sXG5cbiAgZ2V0RGF0YUJpbmRpbmdzKCkge1xuICAgIHJldHVybiB7XG4gICAgICBhcHA6IGdldHRlcnMuYXBwU3RhdGVcbiAgICB9XG4gIH0sXG5cbiAgY29tcG9uZW50V2lsbE1vdW50KCl7XG4gICAgYWN0aW9ucy5pbml0QXBwKCk7XG4gICAgdGhpcy5yZWZyZXNoSW50ZXJ2YWwgPSBzZXRJbnRlcnZhbChhY3Rpb25zLmZldGNoTm9kZXNBbmRTZXNzaW9ucywgMzUwMDApO1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50OiBmdW5jdGlvbigpIHtcbiAgICBjbGVhckludGVydmFsKHRoaXMucmVmcmVzaEludGVydmFsKTtcbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIGlmKHRoaXMuc3RhdGUuYXBwLmlzSW5pdGlhbGl6aW5nKXtcbiAgICAgIHJldHVybiBudWxsO1xuICAgIH1cblxuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi10bHB0IGdydi1mbGV4IGdydi1mbGV4LXJvd1wiPlxuICAgICAgICA8U2VsZWN0Tm9kZURpYWxvZy8+XG4gICAgICAgIDxOb3RpZmljYXRpb25Ib3N0Lz5cbiAgICAgICAge3RoaXMucHJvcHMuQ3VycmVudFNlc3Npb25Ib3N0fVxuICAgICAgICA8TmF2TGVmdEJhci8+XG4gICAgICAgIHt0aGlzLnByb3BzLmNoaWxkcmVufVxuICAgICAgPC9kaXY+XG4gICAgKTtcbiAgfVxufSlcblxubW9kdWxlLmV4cG9ydHMgPSBBcHA7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9hcHAuanN4XG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHtub2RlSG9zdE5hbWVCeVNlcnZlcklkfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL25vZGVzL2dldHRlcnMnKTtcbnZhciBUdHlUZXJtaW5hbCA9IHJlcXVpcmUoJy4vLi4vdGVybWluYWwuanN4Jyk7XG52YXIgU2Vzc2lvbkxlZnRQYW5lbCA9IHJlcXVpcmUoJy4vc2Vzc2lvbkxlZnRQYW5lbCcpO1xuXG52YXIgQWN0aXZlU2Vzc2lvbiA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICBsZXQge2xvZ2luLCBwYXJ0aWVzLCBzZXJ2ZXJJZH0gPSB0aGlzLnByb3BzO1xuICAgIGxldCBzZXJ2ZXJMYWJlbFRleHQgPSAnJztcbiAgICBpZihzZXJ2ZXJJZCl7XG4gICAgICBsZXQgaG9zdG5hbWUgPSByZWFjdG9yLmV2YWx1YXRlKG5vZGVIb3N0TmFtZUJ5U2VydmVySWQoc2VydmVySWQpKTtcbiAgICAgIHNlcnZlckxhYmVsVGV4dCA9IGAke2xvZ2lufUAke2hvc3RuYW1lfWA7XG4gICAgfVxuXG4gICAgcmV0dXJuIChcbiAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtY3VycmVudC1zZXNzaW9uXCI+XG4gICAgICAgPFNlc3Npb25MZWZ0UGFuZWwgcGFydGllcz17cGFydGllc30vPlxuICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWN1cnJlbnQtc2Vzc2lvbi1zZXJ2ZXItaW5mb1wiPlxuICAgICAgICAgPGgzPntzZXJ2ZXJMYWJlbFRleHR9PC9oMz5cbiAgICAgICA8L2Rpdj5cbiAgICAgICA8VHR5VGVybWluYWwgcmVmPVwidHR5Q21udEluc3RhbmNlXCIgey4uLnRoaXMucHJvcHN9IC8+XG4gICAgIDwvZGl2PlxuICAgICApO1xuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBBY3RpdmVTZXNzaW9uO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvY3VycmVudFNlc3Npb24vYWN0aXZlU2Vzc2lvbi5qc3hcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIge2dldHRlcnMsIGFjdGlvbnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvY3VycmVudFNlc3Npb24vJyk7XG52YXIgU2Vzc2lvblBsYXllciA9IHJlcXVpcmUoJy4vc2Vzc2lvblBsYXllci5qc3gnKTtcbnZhciBBY3RpdmVTZXNzaW9uID0gcmVxdWlyZSgnLi9hY3RpdmVTZXNzaW9uLmpzeCcpO1xuXG52YXIgQ3VycmVudFNlc3Npb25Ib3N0ID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW3JlYWN0b3IuUmVhY3RNaXhpbl0sXG5cbiAgZ2V0RGF0YUJpbmRpbmdzKCkge1xuICAgIHJldHVybiB7XG4gICAgICBjdXJyZW50U2Vzc2lvbjogZ2V0dGVycy5jdXJyZW50U2Vzc2lvblxuICAgIH1cbiAgfSxcblxuICBjb21wb25lbnREaWRNb3VudCgpe1xuICAgIHZhciB7IHNpZCB9ID0gdGhpcy5wcm9wcy5wYXJhbXM7XG4gICAgaWYoIXRoaXMuc3RhdGUuY3VycmVudFNlc3Npb24pe1xuICAgICAgYWN0aW9ucy5vcGVuU2Vzc2lvbihzaWQpO1xuICAgIH1cbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIHZhciBjdXJyZW50U2Vzc2lvbiA9IHRoaXMuc3RhdGUuY3VycmVudFNlc3Npb247XG4gICAgaWYoIWN1cnJlbnRTZXNzaW9uKXtcbiAgICAgIHJldHVybiBudWxsO1xuICAgIH1cblxuICAgIGlmKGN1cnJlbnRTZXNzaW9uLmlzTmV3U2Vzc2lvbiB8fCBjdXJyZW50U2Vzc2lvbi5hY3RpdmUpe1xuICAgICAgcmV0dXJuIDxBY3RpdmVTZXNzaW9uIHsuLi5jdXJyZW50U2Vzc2lvbn0vPjtcbiAgICB9XG5cbiAgICByZXR1cm4gPFNlc3Npb25QbGF5ZXIgey4uLmN1cnJlbnRTZXNzaW9ufS8+O1xuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBDdXJyZW50U2Vzc2lvbkhvc3Q7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9jdXJyZW50U2Vzc2lvbi9tYWluLmpzeFxuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciBSZWFjdFNsaWRlciA9IHJlcXVpcmUoJ3JlYWN0LXNsaWRlcicpO1xudmFyIFR0eVBsYXllciA9IHJlcXVpcmUoJ2FwcC9jb21tb24vdHR5UGxheWVyJylcbnZhciBUZXJtaW5hbCA9IHJlcXVpcmUoJ2FwcC9jb21tb24vdGVybWluYWwnKTtcbnZhciBTZXNzaW9uTGVmdFBhbmVsID0gcmVxdWlyZSgnLi9zZXNzaW9uTGVmdFBhbmVsLmpzeCcpO1xuXG5jbGFzcyBNeVRlcm1pbmFsIGV4dGVuZHMgVGVybWluYWx7XG4gIGNvbnN0cnVjdG9yKHR0eSwgZWwpe1xuICAgIHN1cGVyKHtlbH0pO1xuICAgIHRoaXMudHR5ID0gdHR5O1xuICB9XG5cbiAgY29ubmVjdCgpe1xuICAgIHRoaXMudHR5LmNvbm5lY3QoKTtcbiAgfVxufVxuXG52YXIgVGVybWluYWxQbGF5ZXIgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgY29tcG9uZW50RGlkTW91bnQ6IGZ1bmN0aW9uKCkge1xuICAgIHRoaXMudGVybWluYWwgPSBuZXcgTXlUZXJtaW5hbCh0aGlzLnByb3BzLnR0eSwgdGhpcy5yZWZzLmNvbnRhaW5lcik7XG4gICAgdGhpcy50ZXJtaW5hbC5vcGVuKCk7XG4gIH0sXG5cbiAgY29tcG9uZW50V2lsbFVubW91bnQ6IGZ1bmN0aW9uKCkge1xuICAgIHRoaXMudGVybWluYWwuZGVzdHJveSgpO1xuICB9LFxuXG4gIHNob3VsZENvbXBvbmVudFVwZGF0ZTogZnVuY3Rpb24oKSB7XG4gICAgcmV0dXJuIGZhbHNlO1xuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICByZXR1cm4gKCA8ZGl2IHJlZj1cImNvbnRhaW5lclwiPiAgPC9kaXY+ICk7XG4gIH1cbn0pO1xuXG52YXIgU2Vzc2lvblBsYXllciA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgY2FsY3VsYXRlU3RhdGUoKXtcbiAgICByZXR1cm4ge1xuICAgICAgbGVuZ3RoOiB0aGlzLnR0eS5sZW5ndGgsXG4gICAgICBtaW46IDEsXG4gICAgICBpc1BsYXlpbmc6IHRoaXMudHR5LmlzUGxheWluZyxcbiAgICAgIGN1cnJlbnQ6IHRoaXMudHR5LmN1cnJlbnQsXG4gICAgICBjYW5QbGF5OiB0aGlzLnR0eS5sZW5ndGggPiAxXG4gICAgfTtcbiAgfSxcblxuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgdmFyIHNpZCA9IHRoaXMucHJvcHMuc2lkO1xuICAgIHRoaXMudHR5ID0gbmV3IFR0eVBsYXllcih7c2lkfSk7XG4gICAgcmV0dXJuIHRoaXMuY2FsY3VsYXRlU3RhdGUoKTtcbiAgfSxcblxuICBjb21wb25lbnRXaWxsVW5tb3VudCgpIHtcbiAgICB0aGlzLnR0eS5zdG9wKCk7XG4gICAgdGhpcy50dHkucmVtb3ZlQWxsTGlzdGVuZXJzKCk7XG4gIH0sXG5cbiAgY29tcG9uZW50RGlkTW91bnQoKSB7XG4gICAgdGhpcy50dHkub24oJ2NoYW5nZScsICgpPT57XG4gICAgICB2YXIgbmV3U3RhdGUgPSB0aGlzLmNhbGN1bGF0ZVN0YXRlKCk7XG4gICAgICB0aGlzLnNldFN0YXRlKG5ld1N0YXRlKTtcbiAgICB9KTtcblxuICAgIHRoaXMudHR5LnBsYXkoKTtcbiAgfSxcblxuICB0b2dnbGVQbGF5U3RvcCgpe1xuICAgIGlmKHRoaXMuc3RhdGUuaXNQbGF5aW5nKXtcbiAgICAgIHRoaXMudHR5LnN0b3AoKTtcbiAgICB9ZWxzZXtcbiAgICAgIHRoaXMudHR5LnBsYXkoKTtcbiAgICB9XG4gIH0sXG5cbiAgbW92ZSh2YWx1ZSl7XG4gICAgdGhpcy50dHkubW92ZSh2YWx1ZSk7XG4gIH0sXG5cbiAgb25CZWZvcmVDaGFuZ2UoKXtcbiAgICB0aGlzLnR0eS5zdG9wKCk7XG4gIH0sXG5cbiAgb25BZnRlckNoYW5nZSh2YWx1ZSl7XG4gICAgdGhpcy50dHkucGxheSgpO1xuICAgIHRoaXMudHR5Lm1vdmUodmFsdWUpO1xuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgdmFyIHtpc1BsYXlpbmd9ID0gdGhpcy5zdGF0ZTtcblxuICAgIHJldHVybiAoXG4gICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWN1cnJlbnQtc2Vzc2lvbiBncnYtc2Vzc2lvbi1wbGF5ZXJcIj5cbiAgICAgICA8U2Vzc2lvbkxlZnRQYW5lbC8+XG4gICAgICAgPFRlcm1pbmFsUGxheWVyIHJlZj1cInRlcm1cIiB0dHk9e3RoaXMudHR5fSBzY3JvbGxiYWNrPXswfSAvPlxuICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXNlc3Npb24tcGxheWVyLWNvbnRyb2xzXCI+XG4gICAgICAgICA8YnV0dG9uIGNsYXNzTmFtZT1cImJ0blwiIG9uQ2xpY2s9e3RoaXMudG9nZ2xlUGxheVN0b3B9PlxuICAgICAgICAgICB7IGlzUGxheWluZyA/IDxpIGNsYXNzTmFtZT1cImZhIGZhLXN0b3BcIj48L2k+IDogIDxpIGNsYXNzTmFtZT1cImZhIGZhLXBsYXlcIj48L2k+IH1cbiAgICAgICAgIDwvYnV0dG9uPlxuICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZmxleC1jb2x1bW5cIj5cbiAgICAgICAgICAgPFJlYWN0U2xpZGVyXG4gICAgICAgICAgICAgIG1pbj17dGhpcy5zdGF0ZS5taW59XG4gICAgICAgICAgICAgIG1heD17dGhpcy5zdGF0ZS5sZW5ndGh9XG4gICAgICAgICAgICAgIHZhbHVlPXt0aGlzLnN0YXRlLmN1cnJlbnR9XG4gICAgICAgICAgICAgIG9uQ2hhbmdlPXt0aGlzLm1vdmV9XG4gICAgICAgICAgICAgIGRlZmF1bHRWYWx1ZT17MX1cbiAgICAgICAgICAgICAgd2l0aEJhcnNcbiAgICAgICAgICAgICAgY2xhc3NOYW1lPVwiZ3J2LXNsaWRlclwiIC8+XG4gICAgICAgICA8L2Rpdj5cbiAgICAgICAgPC9kaXY+XG4gICAgIDwvZGl2PlxuICAgICApO1xuICB9XG59KTtcblxuXG5cbmV4cG9ydCBkZWZhdWx0IFNlc3Npb25QbGF5ZXI7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9jdXJyZW50U2Vzc2lvbi9zZXNzaW9uUGxheWVyLmpzeFxuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciAkID0gcmVxdWlyZSgnalF1ZXJ5Jyk7XG52YXIgbW9tZW50ID0gcmVxdWlyZSgnbW9tZW50Jyk7XG52YXIge2RlYm91bmNlfSA9IHJlcXVpcmUoJ18nKTtcblxudmFyIERhdGVSYW5nZVBpY2tlciA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBnZXREYXRlcygpe1xuICAgIHZhciBzdGFydERhdGUgPSAkKHRoaXMucmVmcy5kcFBpY2tlcjEpLmRhdGVwaWNrZXIoJ2dldERhdGUnKTtcbiAgICB2YXIgZW5kRGF0ZSA9ICQodGhpcy5yZWZzLmRwUGlja2VyMikuZGF0ZXBpY2tlcignZ2V0RGF0ZScpO1xuICAgIHJldHVybiBbc3RhcnREYXRlLCBtb21lbnQoZW5kRGF0ZSkuZW5kT2YoJ2RheScpLnRvRGF0ZSgpXTtcbiAgfSxcblxuICBzZXREYXRlcyh7c3RhcnREYXRlLCBlbmREYXRlfSl7XG4gICAgJCh0aGlzLnJlZnMuZHBQaWNrZXIxKS5kYXRlcGlja2VyKCdzZXREYXRlJywgc3RhcnREYXRlKTtcbiAgICAkKHRoaXMucmVmcy5kcFBpY2tlcjIpLmRhdGVwaWNrZXIoJ3NldERhdGUnLCBlbmREYXRlKTtcbiAgfSxcblxuICBnZXREZWZhdWx0UHJvcHMoKSB7XG4gICAgIHJldHVybiB7XG4gICAgICAgc3RhcnREYXRlOiBtb21lbnQoKS5zdGFydE9mKCdtb250aCcpLnRvRGF0ZSgpLFxuICAgICAgIGVuZERhdGU6IG1vbWVudCgpLmVuZE9mKCdtb250aCcpLnRvRGF0ZSgpLFxuICAgICAgIG9uQ2hhbmdlOiAoKT0+e31cbiAgICAgfTtcbiAgIH0sXG5cbiAgY29tcG9uZW50V2lsbFVubW91bnQoKXtcbiAgICAkKHRoaXMucmVmcy5kcCkuZGF0ZXBpY2tlcignZGVzdHJveScpO1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxSZWNlaXZlUHJvcHMobmV3UHJvcHMpe1xuICAgIHZhciBbc3RhcnREYXRlLCBlbmREYXRlXSA9IHRoaXMuZ2V0RGF0ZXMoKTtcbiAgICBpZighKGlzU2FtZShzdGFydERhdGUsIG5ld1Byb3BzLnN0YXJ0RGF0ZSkgJiZcbiAgICAgICAgICBpc1NhbWUoZW5kRGF0ZSwgbmV3UHJvcHMuZW5kRGF0ZSkpKXtcbiAgICAgICAgdGhpcy5zZXREYXRlcyhuZXdQcm9wcyk7XG4gICAgICB9XG4gIH0sXG5cbiAgc2hvdWxkQ29tcG9uZW50VXBkYXRlKCl7XG4gICAgcmV0dXJuIGZhbHNlO1xuICB9LFxuXG4gIGNvbXBvbmVudERpZE1vdW50KCl7XG4gICAgdGhpcy5vbkNoYW5nZSA9IGRlYm91bmNlKHRoaXMub25DaGFuZ2UsIDEpO1xuICAgICQodGhpcy5yZWZzLnJhbmdlUGlja2VyKS5kYXRlcGlja2VyKHtcbiAgICAgIHRvZGF5QnRuOiAnbGlua2VkJyxcbiAgICAgIGtleWJvYXJkTmF2aWdhdGlvbjogZmFsc2UsXG4gICAgICBmb3JjZVBhcnNlOiBmYWxzZSxcbiAgICAgIGNhbGVuZGFyV2Vla3M6IHRydWUsXG4gICAgICBhdXRvY2xvc2U6IHRydWVcbiAgICB9KS5vbignY2hhbmdlRGF0ZScsIHRoaXMub25DaGFuZ2UpO1xuXG4gICAgdGhpcy5zZXREYXRlcyh0aGlzLnByb3BzKTtcbiAgfSxcblxuICBvbkNoYW5nZSgpe1xuICAgIHZhciBbc3RhcnREYXRlLCBlbmREYXRlXSA9IHRoaXMuZ2V0RGF0ZXMoKVxuICAgIGlmKCEoaXNTYW1lKHN0YXJ0RGF0ZSwgdGhpcy5wcm9wcy5zdGFydERhdGUpICYmXG4gICAgICAgICAgaXNTYW1lKGVuZERhdGUsIHRoaXMucHJvcHMuZW5kRGF0ZSkpKXtcbiAgICAgICAgdGhpcy5wcm9wcy5vbkNoYW5nZSh7c3RhcnREYXRlLCBlbmREYXRlfSk7XG4gICAgfVxuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZGF0ZXBpY2tlciBpbnB1dC1ncm91cCBpbnB1dC1kYXRlcmFuZ2VcIiByZWY9XCJyYW5nZVBpY2tlclwiPlxuICAgICAgICA8aW5wdXQgcmVmPVwiZHBQaWNrZXIxXCIgdHlwZT1cInRleHRcIiBjbGFzc05hbWU9XCJpbnB1dC1zbSBmb3JtLWNvbnRyb2xcIiBuYW1lPVwic3RhcnRcIiAvPlxuICAgICAgICA8c3BhbiBjbGFzc05hbWU9XCJpbnB1dC1ncm91cC1hZGRvblwiPnRvPC9zcGFuPlxuICAgICAgICA8aW5wdXQgcmVmPVwiZHBQaWNrZXIyXCIgdHlwZT1cInRleHRcIiBjbGFzc05hbWU9XCJpbnB1dC1zbSBmb3JtLWNvbnRyb2xcIiBuYW1lPVwiZW5kXCIgLz5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pO1xuXG5mdW5jdGlvbiBpc1NhbWUoZGF0ZTEsIGRhdGUyKXtcbiAgcmV0dXJuIG1vbWVudChkYXRlMSkuaXNTYW1lKGRhdGUyLCAnZGF5Jyk7XG59XG5cbi8qKlxuKiBDYWxlbmRhciBOYXZcbiovXG52YXIgQ2FsZW5kYXJOYXYgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgcmVuZGVyKCkge1xuICAgIGxldCB7dmFsdWV9ID0gdGhpcy5wcm9wcztcbiAgICBsZXQgZGlzcGxheVZhbHVlID0gbW9tZW50KHZhbHVlKS5mb3JtYXQoJ01NTSBEbywgWVlZWScpO1xuXG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPXtcImdydi1jYWxlbmRhci1uYXYgXCIgKyB0aGlzLnByb3BzLmNsYXNzTmFtZX0gPlxuICAgICAgICA8YnV0dG9uIG9uQ2xpY2s9e3RoaXMubW92ZS5iaW5kKHRoaXMsIC0xKX0gY2xhc3NOYW1lPVwiYnRuIGJ0bi1vdXRsaW5lIGJ0bi1saW5rXCI+PGkgY2xhc3NOYW1lPVwiZmEgZmEtY2hldnJvbi1sZWZ0XCI+PC9pPjwvYnV0dG9uPlxuICAgICAgICA8c3BhbiBjbGFzc05hbWU9XCJ0ZXh0LW11dGVkXCI+e2Rpc3BsYXlWYWx1ZX08L3NwYW4+XG4gICAgICAgIDxidXR0b24gb25DbGljaz17dGhpcy5tb3ZlLmJpbmQodGhpcywgMSl9IGNsYXNzTmFtZT1cImJ0biBidG4tb3V0bGluZSBidG4tbGlua1wiPjxpIGNsYXNzTmFtZT1cImZhIGZhLWNoZXZyb24tcmlnaHRcIj48L2k+PC9idXR0b24+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9LFxuXG4gIG1vdmUoYXQpe1xuICAgIGxldCB7dmFsdWV9ID0gdGhpcy5wcm9wcztcbiAgICBsZXQgbmV3VmFsdWUgPSBtb21lbnQodmFsdWUpLmFkZChhdCwgJ3dlZWsnKS50b0RhdGUoKTtcbiAgICB0aGlzLnByb3BzLm9uVmFsdWVDaGFuZ2UobmV3VmFsdWUpO1xuICB9XG59KTtcblxuQ2FsZW5kYXJOYXYuZ2V0d2Vla1JhbmdlID0gZnVuY3Rpb24odmFsdWUpe1xuICBsZXQgc3RhcnREYXRlID0gbW9tZW50KHZhbHVlKS5zdGFydE9mKCdtb250aCcpLnRvRGF0ZSgpO1xuICBsZXQgZW5kRGF0ZSA9IG1vbWVudCh2YWx1ZSkuZW5kT2YoJ21vbnRoJykudG9EYXRlKCk7XG4gIHJldHVybiBbc3RhcnREYXRlLCBlbmREYXRlXTtcbn1cblxuZXhwb3J0IGRlZmF1bHQgRGF0ZVJhbmdlUGlja2VyO1xuZXhwb3J0IHtDYWxlbmRhck5hdiwgRGF0ZVJhbmdlUGlja2VyfTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2RhdGVQaWNrZXIuanN4XG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG5tb2R1bGUuZXhwb3J0cy5BcHAgPSByZXF1aXJlKCcuL2FwcC5qc3gnKTtcbm1vZHVsZS5leHBvcnRzLkxvZ2luID0gcmVxdWlyZSgnLi9sb2dpbi5qc3gnKTtcbm1vZHVsZS5leHBvcnRzLk5ld1VzZXIgPSByZXF1aXJlKCcuL25ld1VzZXIuanN4Jyk7XG5tb2R1bGUuZXhwb3J0cy5Ob2RlcyA9IHJlcXVpcmUoJy4vbm9kZXMvbWFpbi5qc3gnKTtcbm1vZHVsZS5leHBvcnRzLlNlc3Npb25zID0gcmVxdWlyZSgnLi9zZXNzaW9ucy9tYWluLmpzeCcpO1xubW9kdWxlLmV4cG9ydHMuQ3VycmVudFNlc3Npb25Ib3N0ID0gcmVxdWlyZSgnLi9jdXJyZW50U2Vzc2lvbi9tYWluLmpzeCcpO1xubW9kdWxlLmV4cG9ydHMuRXJyb3JQYWdlID0gcmVxdWlyZSgnLi9tc2dQYWdlLmpzeCcpLkVycm9yUGFnZTtcbm1vZHVsZS5leHBvcnRzLk5vdEZvdW5kID0gcmVxdWlyZSgnLi9tc2dQYWdlLmpzeCcpLk5vdEZvdW5kO1xubW9kdWxlLmV4cG9ydHMuTWVzc2FnZVBhZ2UgPSByZXF1aXJlKCcuL21zZ1BhZ2UuanN4JykuTWVzc2FnZVBhZ2U7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9pbmRleC5qc3hcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgJCA9IHJlcXVpcmUoJ2pRdWVyeScpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIExpbmtlZFN0YXRlTWl4aW4gPSByZXF1aXJlKCdyZWFjdC1hZGRvbnMtbGlua2VkLXN0YXRlLW1peGluJyk7XG52YXIge2FjdGlvbnMsIGdldHRlcnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvdXNlcicpO1xudmFyIEdvb2dsZUF1dGhJbmZvID0gcmVxdWlyZSgnLi9nb29nbGVBdXRoTG9nbycpO1xudmFyIGNmZyA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcbnZhciB7VGVsZXBvcnRMb2dvfSA9IHJlcXVpcmUoJy4vaWNvbnMuanN4Jyk7XG52YXIge1BST1ZJREVSX0dPT0dMRX0gPSByZXF1aXJlKCdhcHAvc2VydmljZXMvYXV0aCcpO1xuXG52YXIgTG9naW5JbnB1dEZvcm0gPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbTGlua2VkU3RhdGVNaXhpbl0sXG5cbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB7XG4gICAgICB1c2VyOiAnJyxcbiAgICAgIHBhc3N3b3JkOiAnJyxcbiAgICAgIHRva2VuOiAnJyxcbiAgICAgIHByb3ZpZGVyOiBudWxsXG4gICAgfVxuICB9LFxuXG5cbiAgb25Mb2dpbihlKXtcbiAgICBlLnByZXZlbnREZWZhdWx0KCk7XG4gICAgaWYgKHRoaXMuaXNWYWxpZCgpKSB7XG4gICAgICB0aGlzLnByb3BzLm9uQ2xpY2sodGhpcy5zdGF0ZSk7XG4gICAgfVxuICB9LFxuXG4gIG9uTG9naW5XaXRoR29vZ2xlOiBmdW5jdGlvbihlKSB7XG4gICAgZS5wcmV2ZW50RGVmYXVsdCgpO1xuICAgIHRoaXMuc3RhdGUucHJvdmlkZXIgPSBQUk9WSURFUl9HT09HTEU7XG4gICAgdGhpcy5wcm9wcy5vbkNsaWNrKHRoaXMuc3RhdGUpO1xuICB9LFxuXG4gIGlzVmFsaWQ6IGZ1bmN0aW9uKCkge1xuICAgIHZhciAkZm9ybSA9ICQodGhpcy5yZWZzLmZvcm0pO1xuICAgIHJldHVybiAkZm9ybS5sZW5ndGggPT09IDAgfHwgJGZvcm0udmFsaWQoKTtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgbGV0IHtpc1Byb2Nlc3NpbmcsIGlzRmFpbGVkLCBtZXNzYWdlIH0gPSB0aGlzLnByb3BzLmF0dGVtcDtcbiAgICBsZXQgcHJvdmlkZXJzID0gY2ZnLmdldEF1dGhQcm92aWRlcnMoKTtcbiAgICBsZXQgdXNlR29vZ2xlID0gcHJvdmlkZXJzLmluZGV4T2YoUFJPVklERVJfR09PR0xFKSAhPT0gLTE7XG5cbiAgICByZXR1cm4gKFxuICAgICAgPGZvcm0gcmVmPVwiZm9ybVwiIGNsYXNzTmFtZT1cImdydi1sb2dpbi1pbnB1dC1mb3JtXCI+XG4gICAgICAgIDxoMz4gV2VsY29tZSB0byBUZWxlcG9ydCA8L2gzPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0IGF1dG9Gb2N1cyB2YWx1ZUxpbms9e3RoaXMubGlua1N0YXRlKCd1c2VyJyl9IGNsYXNzTmFtZT1cImZvcm0tY29udHJvbCByZXF1aXJlZFwiIHBsYWNlaG9sZGVyPVwiVXNlciBuYW1lXCIgbmFtZT1cInVzZXJOYW1lXCIgLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImZvcm0tZ3JvdXBcIj5cbiAgICAgICAgICAgIDxpbnB1dCB2YWx1ZUxpbms9e3RoaXMubGlua1N0YXRlKCdwYXNzd29yZCcpfSB0eXBlPVwicGFzc3dvcmRcIiBuYW1lPVwicGFzc3dvcmRcIiBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2wgcmVxdWlyZWRcIiBwbGFjZWhvbGRlcj1cIlBhc3N3b3JkXCIvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0IGF1dG9Db21wbGV0ZT1cIm9mZlwiIHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3Rva2VuJyl9IGNsYXNzTmFtZT1cImZvcm0tY29udHJvbCByZXF1aXJlZFwiIG5hbWU9XCJ0b2tlblwiIHBsYWNlaG9sZGVyPVwiVHdvIGZhY3RvciB0b2tlbiAoR29vZ2xlIEF1dGhlbnRpY2F0b3IpXCIvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxidXR0b24gb25DbGljaz17dGhpcy5vbkxvZ2lufSBkaXNhYmxlZD17aXNQcm9jZXNzaW5nfSB0eXBlPVwic3VibWl0XCIgY2xhc3NOYW1lPVwiYnRuIGJ0bi1wcmltYXJ5IGJsb2NrIGZ1bGwtd2lkdGggbS1iXCI+TG9naW48L2J1dHRvbj5cbiAgICAgICAgICB7IHVzZUdvb2dsZSA/IDxidXR0b24gb25DbGljaz17dGhpcy5vbkxvZ2luV2l0aEdvb2dsZX0gdHlwZT1cInN1Ym1pdFwiIGNsYXNzTmFtZT1cImJ0biBidG4tZGFuZ2VyIGJsb2NrIGZ1bGwtd2lkdGggbS1iXCI+V2l0aCBHb29nbGU8L2J1dHRvbj4gOiBudWxsIH1cbiAgICAgICAgICB7IGlzRmFpbGVkID8gKDxsYWJlbCBjbGFzc05hbWU9XCJlcnJvclwiPnttZXNzYWdlfTwvbGFiZWw+KSA6IG51bGwgfVxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZm9ybT5cbiAgICApO1xuICB9XG59KVxuXG52YXIgTG9naW4gPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIGF0dGVtcDogZ2V0dGVycy5sb2dpbkF0dGVtcFxuICAgIH1cbiAgfSxcblxuICBvbkNsaWNrKGlucHV0RGF0YSl7XG4gICAgdmFyIGxvYyA9IHRoaXMucHJvcHMubG9jYXRpb247XG4gICAgdmFyIHJlZGlyZWN0ID0gY2ZnLnJvdXRlcy5hcHA7XG5cbiAgICBpZihsb2Muc3RhdGUgJiYgbG9jLnN0YXRlLnJlZGlyZWN0VG8pe1xuICAgICAgcmVkaXJlY3QgPSBsb2Muc3RhdGUucmVkaXJlY3RUbztcbiAgICB9XG5cbiAgICBhY3Rpb25zLmxvZ2luKGlucHV0RGF0YSwgcmVkaXJlY3QpO1xuICB9LFxuXG4gIHJlbmRlcigpIHtcbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbG9naW4gdGV4dC1jZW50ZXJcIj5cbiAgICAgICAgPFRlbGVwb3J0TG9nby8+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWNvbnRlbnQgZ3J2LWZsZXhcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LWNvbHVtblwiPlxuICAgICAgICAgICAgPExvZ2luSW5wdXRGb3JtIGF0dGVtcD17dGhpcy5zdGF0ZS5hdHRlbXB9IG9uQ2xpY2s9e3RoaXMub25DbGlja30vPlxuICAgICAgICAgICAgPEdvb2dsZUF1dGhJbmZvLz5cbiAgICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWxvZ2luLWluZm9cIj5cbiAgICAgICAgICAgICAgPGkgY2xhc3NOYW1lPVwiZmEgZmEtcXVlc3Rpb25cIj48L2k+XG4gICAgICAgICAgICAgIDxzdHJvbmc+TmV3IEFjY291bnQgb3IgZm9yZ290IHBhc3N3b3JkPzwvc3Ryb25nPlxuICAgICAgICAgICAgICA8ZGl2PkFzayBmb3IgYXNzaXN0YW5jZSBmcm9tIHlvdXIgQ29tcGFueSBhZG1pbmlzdHJhdG9yPC9kaXY+XG4gICAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBMb2dpbjtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2xvZ2luLmpzeFxuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cbnZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgeyBJbmRleExpbmsgfSA9IHJlcXVpcmUoJ3JlYWN0LXJvdXRlcicpO1xudmFyIGdldHRlcnMgPSByZXF1aXJlKCdhcHAvbW9kdWxlcy91c2VyL2dldHRlcnMnKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG52YXIge1VzZXJJY29ufSA9IHJlcXVpcmUoJy4vaWNvbnMuanN4Jyk7XG5cbnZhciBtZW51SXRlbXMgPSBbXG4gIHtpY29uOiAnZmEgZmEtc2hhcmUtYWx0JywgdG86IGNmZy5yb3V0ZXMubm9kZXMsIHRpdGxlOiAnTm9kZXMnfSxcbiAge2ljb246ICdmYSAgZmEtZ3JvdXAnLCB0bzogY2ZnLnJvdXRlcy5zZXNzaW9ucywgdGl0bGU6ICdTZXNzaW9ucyd9XG5dO1xuXG52YXIgTmF2TGVmdEJhciA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgcmVuZGVyOiBmdW5jdGlvbigpe1xuICAgIHZhciB7bmFtZX0gPSByZWFjdG9yLmV2YWx1YXRlKGdldHRlcnMudXNlcik7XG4gICAgdmFyIGl0ZW1zID0gbWVudUl0ZW1zLm1hcCgoaSwgaW5kZXgpPT57XG4gICAgICB2YXIgY2xhc3NOYW1lID0gdGhpcy5jb250ZXh0LnJvdXRlci5pc0FjdGl2ZShpLnRvKSA/ICdhY3RpdmUnIDogJyc7XG4gICAgICByZXR1cm4gKFxuICAgICAgICA8bGkga2V5PXtpbmRleH0gY2xhc3NOYW1lPXtjbGFzc05hbWV9IHRpdGxlPXtpLnRpdGxlfT5cbiAgICAgICAgICA8SW5kZXhMaW5rIHRvPXtpLnRvfT5cbiAgICAgICAgICAgIDxpIGNsYXNzTmFtZT17aS5pY29ufSAvPlxuICAgICAgICAgIDwvSW5kZXhMaW5rPlxuICAgICAgICA8L2xpPlxuICAgICAgKTtcbiAgICB9KTtcblxuICAgIGl0ZW1zLnB1c2goKFxuICAgICAgPGxpIGtleT17aXRlbXMubGVuZ3RofSB0aXRsZT1cImhlbHBcIj5cbiAgICAgICAgPGEgaHJlZj17Y2ZnLmhlbHBVcmx9IHRhcmdldD1cIl9ibGFua1wiPlxuICAgICAgICAgIDxpIGNsYXNzTmFtZT1cImZhIGZhLXF1ZXN0aW9uXCIgLz5cbiAgICAgICAgPC9hPlxuICAgICAgPC9saT4pKTtcblxuICAgIGl0ZW1zLnB1c2goKFxuICAgICAgPGxpIGtleT17aXRlbXMubGVuZ3RofSB0aXRsZT1cImxvZ291dFwiPlxuICAgICAgICA8YSBocmVmPXtjZmcucm91dGVzLmxvZ291dH0+XG4gICAgICAgICAgPGkgY2xhc3NOYW1lPVwiZmEgZmEtc2lnbi1vdXRcIiBzdHlsZT17e21hcmdpblJpZ2h0OiAwfX0+PC9pPlxuICAgICAgICA8L2E+XG4gICAgICA8L2xpPlxuICAgICkpO1xuXG4gICAgcmV0dXJuIChcbiAgICAgIDxuYXYgY2xhc3NOYW1lPSdncnYtbmF2IG5hdmJhci1kZWZhdWx0JyByb2xlPSduYXZpZ2F0aW9uJz5cbiAgICAgICAgPHVsIGNsYXNzTmFtZT0nbmF2IHRleHQtY2VudGVyJyBpZD0nc2lkZS1tZW51Jz5cbiAgICAgICAgICA8bGk+XG4gICAgICAgICAgICA8VXNlckljb24gbmFtZT17bmFtZX0gLz5cbiAgICAgICAgICA8L2xpPlxuICAgICAgICAgIHtpdGVtc31cbiAgICAgICAgPC91bD5cbiAgICAgIDwvbmF2PlxuICAgICk7XG4gIH1cbn0pO1xuXG5OYXZMZWZ0QmFyLmNvbnRleHRUeXBlcyA9IHtcbiAgcm91dGVyOiBSZWFjdC5Qcm9wVHlwZXMub2JqZWN0LmlzUmVxdWlyZWRcbn1cblxubW9kdWxlLmV4cG9ydHMgPSBOYXZMZWZ0QmFyO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvbmF2TGVmdEJhci5qc3hcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgJCA9IHJlcXVpcmUoJ2pRdWVyeScpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHthY3Rpb25zLCBnZXR0ZXJzfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3VzZXInKTtcbnZhciBMaW5rZWRTdGF0ZU1peGluID0gcmVxdWlyZSgncmVhY3QtYWRkb25zLWxpbmtlZC1zdGF0ZS1taXhpbicpO1xudmFyIEdvb2dsZUF1dGhJbmZvID0gcmVxdWlyZSgnLi9nb29nbGVBdXRoTG9nbycpO1xudmFyIHtFcnJvclBhZ2UsIEVycm9yVHlwZXN9ID0gcmVxdWlyZSgnLi9tc2dQYWdlJyk7XG52YXIge1RlbGVwb3J0TG9nb30gPSByZXF1aXJlKCcuL2ljb25zLmpzeCcpO1xuXG52YXIgSW52aXRlSW5wdXRGb3JtID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW0xpbmtlZFN0YXRlTWl4aW5dLFxuXG4gIGNvbXBvbmVudERpZE1vdW50KCl7XG4gICAgJCh0aGlzLnJlZnMuZm9ybSkudmFsaWRhdGUoe1xuICAgICAgcnVsZXM6e1xuICAgICAgICBwYXNzd29yZDp7XG4gICAgICAgICAgbWlubGVuZ3RoOiA2LFxuICAgICAgICAgIHJlcXVpcmVkOiB0cnVlXG4gICAgICAgIH0sXG4gICAgICAgIHBhc3N3b3JkQ29uZmlybWVkOntcbiAgICAgICAgICByZXF1aXJlZDogdHJ1ZSxcbiAgICAgICAgICBlcXVhbFRvOiB0aGlzLnJlZnMucGFzc3dvcmRcbiAgICAgICAgfVxuICAgICAgfSxcblxuICAgICAgbWVzc2FnZXM6IHtcbiAgXHRcdFx0cGFzc3dvcmRDb25maXJtZWQ6IHtcbiAgXHRcdFx0XHRtaW5sZW5ndGg6ICQudmFsaWRhdG9yLmZvcm1hdCgnRW50ZXIgYXQgbGVhc3QgezB9IGNoYXJhY3RlcnMnKSxcbiAgXHRcdFx0XHRlcXVhbFRvOiAnRW50ZXIgdGhlIHNhbWUgcGFzc3dvcmQgYXMgYWJvdmUnXG4gIFx0XHRcdH1cbiAgICAgIH1cbiAgICB9KVxuICB9LFxuXG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4ge1xuICAgICAgbmFtZTogdGhpcy5wcm9wcy5pbnZpdGUudXNlcixcbiAgICAgIHBzdzogJycsXG4gICAgICBwc3dDb25maXJtZWQ6ICcnLFxuICAgICAgdG9rZW46ICcnXG4gICAgfVxuICB9LFxuXG4gIG9uQ2xpY2soZSkge1xuICAgIGUucHJldmVudERlZmF1bHQoKTtcbiAgICBpZiAodGhpcy5pc1ZhbGlkKCkpIHtcbiAgICAgIGFjdGlvbnMuc2lnblVwKHtcbiAgICAgICAgbmFtZTogdGhpcy5zdGF0ZS5uYW1lLFxuICAgICAgICBwc3c6IHRoaXMuc3RhdGUucHN3LFxuICAgICAgICB0b2tlbjogdGhpcy5zdGF0ZS50b2tlbixcbiAgICAgICAgaW52aXRlVG9rZW46IHRoaXMucHJvcHMuaW52aXRlLmludml0ZV90b2tlbn0pO1xuICAgIH1cbiAgfSxcblxuICBpc1ZhbGlkKCkge1xuICAgIHZhciAkZm9ybSA9ICQodGhpcy5yZWZzLmZvcm0pO1xuICAgIHJldHVybiAkZm9ybS5sZW5ndGggPT09IDAgfHwgJGZvcm0udmFsaWQoKTtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgbGV0IHtpc1Byb2Nlc3NpbmcsIGlzRmFpbGVkLCBtZXNzYWdlIH0gPSB0aGlzLnByb3BzLmF0dGVtcDtcbiAgICByZXR1cm4gKFxuICAgICAgPGZvcm0gcmVmPVwiZm9ybVwiIGNsYXNzTmFtZT1cImdydi1pbnZpdGUtaW5wdXQtZm9ybVwiPlxuICAgICAgICA8aDM+IEdldCBzdGFydGVkIHdpdGggVGVsZXBvcnQgPC9oMz5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImZvcm0tZ3JvdXBcIj5cbiAgICAgICAgICAgIDxpbnB1dFxuICAgICAgICAgICAgICBkaXNhYmxlZFxuICAgICAgICAgICAgICB2YWx1ZUxpbms9e3RoaXMubGlua1N0YXRlKCduYW1lJyl9XG4gICAgICAgICAgICAgIG5hbWU9XCJ1c2VyTmFtZVwiXG4gICAgICAgICAgICAgIGNsYXNzTmFtZT1cImZvcm0tY29udHJvbCByZXF1aXJlZFwiXG4gICAgICAgICAgICAgIHBsYWNlaG9sZGVyPVwiVXNlciBuYW1lXCIvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0XG4gICAgICAgICAgICAgIGF1dG9Gb2N1c1xuICAgICAgICAgICAgICB2YWx1ZUxpbms9e3RoaXMubGlua1N0YXRlKCdwc3cnKX1cbiAgICAgICAgICAgICAgcmVmPVwicGFzc3dvcmRcIlxuICAgICAgICAgICAgICB0eXBlPVwicGFzc3dvcmRcIlxuICAgICAgICAgICAgICBuYW1lPVwicGFzc3dvcmRcIlxuICAgICAgICAgICAgICBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2xcIlxuICAgICAgICAgICAgICBwbGFjZWhvbGRlcj1cIlBhc3N3b3JkXCIgLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImZvcm0tZ3JvdXBcIj5cbiAgICAgICAgICAgIDxpbnB1dFxuICAgICAgICAgICAgICB2YWx1ZUxpbms9e3RoaXMubGlua1N0YXRlKCdwc3dDb25maXJtZWQnKX1cbiAgICAgICAgICAgICAgdHlwZT1cInBhc3N3b3JkXCJcbiAgICAgICAgICAgICAgbmFtZT1cInBhc3N3b3JkQ29uZmlybWVkXCJcbiAgICAgICAgICAgICAgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sXCJcbiAgICAgICAgICAgICAgcGxhY2Vob2xkZXI9XCJQYXNzd29yZCBjb25maXJtXCIvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0XG4gICAgICAgICAgICAgIGF1dG9Db21wbGV0ZT1cIm9mZlwiXG4gICAgICAgICAgICAgIG5hbWU9XCJ0b2tlblwiXG4gICAgICAgICAgICAgIHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3Rva2VuJyl9XG4gICAgICAgICAgICAgIGNsYXNzTmFtZT1cImZvcm0tY29udHJvbCByZXF1aXJlZFwiXG4gICAgICAgICAgICAgIHBsYWNlaG9sZGVyPVwiVHdvIGZhY3RvciB0b2tlbiAoR29vZ2xlIEF1dGhlbnRpY2F0b3IpXCIgLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8YnV0dG9uIHR5cGU9XCJzdWJtaXRcIiBkaXNhYmxlZD17aXNQcm9jZXNzaW5nfSBjbGFzc05hbWU9XCJidG4gYnRuLXByaW1hcnkgYmxvY2sgZnVsbC13aWR0aCBtLWJcIiBvbkNsaWNrPXt0aGlzLm9uQ2xpY2t9ID5TaWduIHVwPC9idXR0b24+XG4gICAgICAgICAgeyBpc0ZhaWxlZCA/ICg8bGFiZWwgY2xhc3NOYW1lPVwiZXJyb3JcIj57bWVzc2FnZX08L2xhYmVsPikgOiBudWxsIH1cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Zvcm0+XG4gICAgKTtcbiAgfVxufSlcblxudmFyIEludml0ZSA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBtaXhpbnM6IFtyZWFjdG9yLlJlYWN0TWl4aW5dLFxuXG4gIGdldERhdGFCaW5kaW5ncygpIHtcbiAgICByZXR1cm4ge1xuICAgICAgaW52aXRlOiBnZXR0ZXJzLmludml0ZSxcbiAgICAgIGF0dGVtcDogZ2V0dGVycy5hdHRlbXAsXG4gICAgICBmZXRjaGluZ0ludml0ZTogZ2V0dGVycy5mZXRjaGluZ0ludml0ZVxuICAgIH1cbiAgfSxcblxuICBjb21wb25lbnREaWRNb3VudCgpe1xuICAgIGFjdGlvbnMuZmV0Y2hJbnZpdGUodGhpcy5wcm9wcy5wYXJhbXMuaW52aXRlVG9rZW4pO1xuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgbGV0IHtmZXRjaGluZ0ludml0ZSwgaW52aXRlLCBhdHRlbXB9ID0gdGhpcy5zdGF0ZTtcblxuICAgIGlmKGZldGNoaW5nSW52aXRlLmlzRmFpbGVkKXtcbiAgICAgIHJldHVybiA8RXJyb3JQYWdlIHR5cGU9e0Vycm9yVHlwZXMuRVhQSVJFRF9JTlZJVEV9Lz5cbiAgICB9XG5cbiAgICBpZighaW52aXRlKSB7XG4gICAgICByZXR1cm4gbnVsbDtcbiAgICB9XG5cbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtaW52aXRlIHRleHQtY2VudGVyXCI+XG4gICAgICAgIDxUZWxlcG9ydExvZ28vPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1jb250ZW50IGdydi1mbGV4XCI+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZmxleC1jb2x1bW5cIj5cbiAgICAgICAgICAgIDxJbnZpdGVJbnB1dEZvcm0gYXR0ZW1wPXthdHRlbXB9IGludml0ZT17aW52aXRlLnRvSlMoKX0vPlxuICAgICAgICAgICAgPEdvb2dsZUF1dGhJbmZvLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LWNvbHVtbiBncnYtaW52aXRlLWJhcmNvZGVcIj5cbiAgICAgICAgICAgIDxoND5TY2FuIGJhciBjb2RlIGZvciBhdXRoIHRva2VuIDxici8+IDxzbWFsbD5TY2FuIGJlbG93IHRvIGdlbmVyYXRlIHlvdXIgdHdvIGZhY3RvciB0b2tlbjwvc21hbGw+PC9oND5cbiAgICAgICAgICAgIDxpbWcgY2xhc3NOYW1lPVwiaW1nLXRodW1ibmFpbFwiIHNyYz17IGBkYXRhOmltYWdlL3BuZztiYXNlNjQsJHtpbnZpdGUuZ2V0KCdxcicpfWAgfSAvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IEludml0ZTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL25ld1VzZXIuanN4XG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHVzZXJHZXR0ZXJzID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvdXNlci9nZXR0ZXJzJyk7XG52YXIgbm9kZUdldHRlcnMgPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9ub2Rlcy9nZXR0ZXJzJyk7XG52YXIgTm9kZUxpc3QgPSByZXF1aXJlKCcuL25vZGVMaXN0LmpzeCcpO1xuXG52YXIgTm9kZXMgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIG5vZGVSZWNvcmRzOiBub2RlR2V0dGVycy5ub2RlTGlzdFZpZXcsXG4gICAgICB1c2VyOiB1c2VyR2V0dGVycy51c2VyXG4gICAgfVxuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgdmFyIG5vZGVSZWNvcmRzID0gdGhpcy5zdGF0ZS5ub2RlUmVjb3JkcztcbiAgICB2YXIgbG9naW5zID0gdGhpcy5zdGF0ZS51c2VyLmxvZ2lucztcbiAgICByZXR1cm4gKCA8Tm9kZUxpc3Qgbm9kZVJlY29yZHM9e25vZGVSZWNvcmRzfSBsb2dpbnM9e2xvZ2luc30vPiApO1xuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBOb2RlcztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL25vZGVzL21haW4uanN4XG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIFB1cmVSZW5kZXJNaXhpbiA9IHJlcXVpcmUoJ3JlYWN0LWFkZG9ucy1wdXJlLXJlbmRlci1taXhpbicpO1xudmFyIHtsYXN0TWVzc2FnZX0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9ub3RpZmljYXRpb25zL2dldHRlcnMnKTtcbnZhciB7VG9hc3RDb250YWluZXIsIFRvYXN0TWVzc2FnZX0gPSByZXF1aXJlKFwicmVhY3QtdG9hc3RyXCIpO1xudmFyIFRvYXN0TWVzc2FnZUZhY3RvcnkgPSBSZWFjdC5jcmVhdGVGYWN0b3J5KFRvYXN0TWVzc2FnZS5hbmltYXRpb24pO1xuXG5jb25zdCBhbmltYXRpb25PcHRpb25zID0ge1xuICBzaG93QW5pbWF0aW9uOiAnYW5pbWF0ZWQgZmFkZUluJyxcbiAgaGlkZUFuaW1hdGlvbjogJ2FuaW1hdGVkIGZhZGVPdXQnXG59XG5cbnZhciBOb3RpZmljYXRpb25Ib3N0ID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW1xuICAgIHJlYWN0b3IuUmVhY3RNaXhpbiwgUHVyZVJlbmRlck1peGluXG4gIF0sXG5cbiAgZ2V0RGF0YUJpbmRpbmdzKCkge1xuICAgIHJldHVybiB7bXNnOiBsYXN0TWVzc2FnZX1cbiAgfSxcblxuICB1cGRhdGUobXNnKSB7XG4gICAgaWYgKG1zZykge1xuICAgICAgaWYgKG1zZy5pc0Vycm9yKSB7XG4gICAgICAgIHRoaXMucmVmcy5jb250YWluZXIuZXJyb3IobXNnLnRleHQsIG1zZy50aXRsZSwgYW5pbWF0aW9uT3B0aW9ucyk7XG4gICAgICB9IGVsc2UgaWYgKG1zZy5pc1dhcm5pbmcpIHtcbiAgICAgICAgdGhpcy5yZWZzLmNvbnRhaW5lci53YXJuaW5nKG1zZy50ZXh0LCBtc2cudGl0bGUsIGFuaW1hdGlvbk9wdGlvbnMpO1xuICAgICAgfSBlbHNlIGlmIChtc2cuaXNTdWNjZXNzKSB7XG4gICAgICAgIHRoaXMucmVmcy5jb250YWluZXIuc3VjY2Vzcyhtc2cudGV4dCwgbXNnLnRpdGxlLCBhbmltYXRpb25PcHRpb25zKTtcbiAgICAgIH0gZWxzZSB7XG4gICAgICAgIHRoaXMucmVmcy5jb250YWluZXIuaW5mbyhtc2cudGV4dCwgbXNnLnRpdGxlLCBhbmltYXRpb25PcHRpb25zKTtcbiAgICAgIH1cbiAgICB9XG4gIH0sXG5cbiAgY29tcG9uZW50RGlkTW91bnQoKSB7XG4gICAgcmVhY3Rvci5vYnNlcnZlKGxhc3RNZXNzYWdlLCB0aGlzLnVwZGF0ZSlcbiAgfSxcblxuICBjb21wb25lbnRXaWxsVW5tb3VudCgpIHtcbiAgICByZWFjdG9yLnVub2JzZXJ2ZShsYXN0TWVzc2FnZSwgdGhpcy51cGRhdGUpO1xuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgcmV0dXJuIChcbiAgICAgICAgPFRvYXN0Q29udGFpbmVyXG4gICAgICAgICAgcmVmPVwiY29udGFpbmVyXCIgdG9hc3RNZXNzYWdlRmFjdG9yeT17VG9hc3RNZXNzYWdlRmFjdG9yeX0gY2xhc3NOYW1lPVwidG9hc3QtdG9wLXJpZ2h0XCIvPlxuICAgICk7XG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IE5vdGlmaWNhdGlvbkhvc3Q7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9ub3RpZmljYXRpb25Ib3N0LmpzeFxuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7Z2V0dGVyc30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9kaWFsb2dzJyk7XG52YXIge2Nsb3NlU2VsZWN0Tm9kZURpYWxvZ30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9kaWFsb2dzL2FjdGlvbnMnKTtcbnZhciBOb2RlTGlzdCA9IHJlcXVpcmUoJy4vbm9kZXMvbm9kZUxpc3QuanN4Jyk7XG52YXIgY3VycmVudFNlc3Npb25HZXR0ZXJzID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvY3VycmVudFNlc3Npb24vZ2V0dGVycycpO1xudmFyIG5vZGVHZXR0ZXJzID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvbm9kZXMvZ2V0dGVycycpO1xudmFyICQgPSByZXF1aXJlKCdqUXVlcnknKTtcblxudmFyIFNlbGVjdE5vZGVEaWFsb2cgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIGRpYWxvZ3M6IGdldHRlcnMuZGlhbG9nc1xuICAgIH1cbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuIHRoaXMuc3RhdGUuZGlhbG9ncy5pc1NlbGVjdE5vZGVEaWFsb2dPcGVuID8gPERpYWxvZy8+IDogbnVsbDtcbiAgfVxufSk7XG5cbnZhciBEaWFsb2cgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgb25Mb2dpbkNsaWNrKHNlcnZlcklkKXtcbiAgICBpZihTZWxlY3ROb2RlRGlhbG9nLm9uU2VydmVyQ2hhbmdlQ2FsbEJhY2spe1xuICAgICAgU2VsZWN0Tm9kZURpYWxvZy5vblNlcnZlckNoYW5nZUNhbGxCYWNrKHtzZXJ2ZXJJZH0pO1xuICAgIH1cblxuICAgIGNsb3NlU2VsZWN0Tm9kZURpYWxvZygpO1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxVbm1vdW50KCl7XG4gICAgJCgnLm1vZGFsJykubW9kYWwoJ2hpZGUnKTtcbiAgfSxcblxuICBjb21wb25lbnREaWRNb3VudCgpe1xuICAgICQoJy5tb2RhbCcpLm1vZGFsKCdzaG93Jyk7XG4gIH0sXG5cbiAgcmVuZGVyKCkge1xuICAgIHZhciBhY3RpdmVTZXNzaW9uID0gcmVhY3Rvci5ldmFsdWF0ZShjdXJyZW50U2Vzc2lvbkdldHRlcnMuY3VycmVudFNlc3Npb24pIHx8IHt9O1xuICAgIHZhciBub2RlUmVjb3JkcyA9IHJlYWN0b3IuZXZhbHVhdGUobm9kZUdldHRlcnMubm9kZUxpc3RWaWV3KTtcbiAgICB2YXIgbG9naW5zID0gW2FjdGl2ZVNlc3Npb24ubG9naW5dO1xuXG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwibW9kYWwgZmFkZSBncnYtZGlhbG9nLXNlbGVjdC1ub2RlXCIgdGFiSW5kZXg9ey0xfSByb2xlPVwiZGlhbG9nXCI+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwibW9kYWwtZGlhbG9nXCI+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJtb2RhbC1jb250ZW50XCI+XG4gICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIm1vZGFsLWhlYWRlclwiPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIm1vZGFsLWJvZHlcIj5cbiAgICAgICAgICAgICAgPE5vZGVMaXN0IG5vZGVSZWNvcmRzPXtub2RlUmVjb3Jkc30gbG9naW5zPXtsb2dpbnN9IG9uTG9naW5DbGljaz17dGhpcy5vbkxvZ2luQ2xpY2t9Lz5cbiAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJtb2RhbC1mb290ZXJcIj5cbiAgICAgICAgICAgICAgPGJ1dHRvbiBvbkNsaWNrPXtjbG9zZVNlbGVjdE5vZGVEaWFsb2d9IHR5cGU9XCJidXR0b25cIiBjbGFzc05hbWU9XCJidG4gYnRuLXByaW1hcnlcIj5cbiAgICAgICAgICAgICAgICBDbG9zZVxuICAgICAgICAgICAgICA8L2J1dHRvbj5cbiAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pO1xuXG5TZWxlY3ROb2RlRGlhbG9nLm9uU2VydmVyQ2hhbmdlQ2FsbEJhY2sgPSAoKT0+e307XG5cbm1vZHVsZS5leHBvcnRzID0gU2VsZWN0Tm9kZURpYWxvZztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL3NlbGVjdE5vZGVEaWFsb2cuanN4XG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHtUYWJsZSwgQ29sdW1uLCBDZWxsLCBUZXh0Q2VsbCwgRW1wdHlJbmRpY2F0b3J9ID0gcmVxdWlyZSgnYXBwL2NvbXBvbmVudHMvdGFibGUuanN4Jyk7XG52YXIge0J1dHRvbkNlbGwsIFVzZXJzQ2VsbCwgTm9kZUNlbGwsIERhdGVDcmVhdGVkQ2VsbH0gPSByZXF1aXJlKCcuL2xpc3RJdGVtcycpO1xuXG52YXIgQWN0aXZlU2Vzc2lvbkxpc3QgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgbGV0IGRhdGEgPSB0aGlzLnByb3BzLmRhdGEuZmlsdGVyKGl0ZW0gPT4gaXRlbS5hY3RpdmUpO1xuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1zZXNzaW9ucy1hY3RpdmVcIj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtaGVhZGVyXCI+XG4gICAgICAgICAgPGgxPiBBY3RpdmUgU2Vzc2lvbnMgPC9oMT5cbiAgICAgICAgPC9kaXY+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWNvbnRlbnRcIj5cbiAgICAgICAgICB7ZGF0YS5sZW5ndGggPT09IDAgPyA8RW1wdHlJbmRpY2F0b3IgdGV4dD1cIllvdSBoYXZlIG5vIGFjdGl2ZSBzZXNzaW9ucy5cIi8+IDpcbiAgICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiXCI+XG4gICAgICAgICAgICAgIDxUYWJsZSByb3dDb3VudD17ZGF0YS5sZW5ndGh9IGNsYXNzTmFtZT1cInRhYmxlLXN0cmlwZWRcIj5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJzaWRcIlxuICAgICAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD4gU2Vzc2lvbiBJRCA8L0NlbGw+IH1cbiAgICAgICAgICAgICAgICAgIGNlbGw9ezxUZXh0Q2VsbCBkYXRhPXtkYXRhfS8+IH1cbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+IDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17XG4gICAgICAgICAgICAgICAgICAgIDxCdXR0b25DZWxsIGRhdGE9e2RhdGF9IC8+XG4gICAgICAgICAgICAgICAgICB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPiBOb2RlIDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17PE5vZGVDZWxsIGRhdGE9e2RhdGF9IC8+IH1cbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICAgIGNvbHVtbktleT1cImNyZWF0ZWRcIlxuICAgICAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD4gQ3JlYXRlZCA8L0NlbGw+IH1cbiAgICAgICAgICAgICAgICAgIGNlbGw9ezxEYXRlQ3JlYXRlZENlbGwgZGF0YT17ZGF0YX0vPiB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPiBVc2VycyA8L0NlbGw+IH1cbiAgICAgICAgICAgICAgICAgIGNlbGw9ezxVc2Vyc0NlbGwgZGF0YT17ZGF0YX0gLz4gfVxuICAgICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgIDwvVGFibGU+XG4gICAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICB9XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9kaXY+XG4gICAgKVxuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBBY3RpdmVTZXNzaW9uTGlzdDtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL3Nlc3Npb25zL2FjdGl2ZVNlc3Npb25MaXN0LmpzeFxuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7c2Vzc2lvbnNWaWV3fSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3Nlc3Npb25zL2dldHRlcnMnKTtcbnZhciB7ZmlsdGVyfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3N0b3JlZFNlc3Npb25zRmlsdGVyL2dldHRlcnMnKTtcbnZhciBTdG9yZWRTZXNzaW9uTGlzdCA9IHJlcXVpcmUoJy4vc3RvcmVkU2Vzc2lvbkxpc3QuanN4Jyk7XG52YXIgQWN0aXZlU2Vzc2lvbkxpc3QgPSByZXF1aXJlKCcuL2FjdGl2ZVNlc3Npb25MaXN0LmpzeCcpO1xuXG52YXIgU2Vzc2lvbnMgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIG1peGluczogW3JlYWN0b3IuUmVhY3RNaXhpbl0sXG5cbiAgZ2V0RGF0YUJpbmRpbmdzKCkge1xuICAgIHJldHVybiB7XG4gICAgICBkYXRhOiBzZXNzaW9uc1ZpZXcsXG4gICAgICBzdG9yZWRTZXNzaW9uc0ZpbHRlcjogZmlsdGVyXG4gICAgfVxuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgbGV0IHtkYXRhLCBzdG9yZWRTZXNzaW9uc0ZpbHRlcn0gPSB0aGlzLnN0YXRlO1xuICAgIHJldHVybiAoXG4gICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1zZXNzaW9ucyBncnYtcGFnZVwiPlxuICAgICAgICA8QWN0aXZlU2Vzc2lvbkxpc3QgZGF0YT17ZGF0YX0vPlxuICAgICAgICA8aHIgY2xhc3NOYW1lPVwiZ3J2LWRpdmlkZXJcIi8+XG4gICAgICAgIDxTdG9yZWRTZXNzaW9uTGlzdCBkYXRhPXtkYXRhfSBmaWx0ZXI9e3N0b3JlZFNlc3Npb25zRmlsdGVyfS8+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBTZXNzaW9ucztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL3Nlc3Npb25zL21haW4uanN4XG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIHthY3Rpb25zfSA9IHJlcXVpcmUoJ2FwcC9tb2R1bGVzL3N0b3JlZFNlc3Npb25zRmlsdGVyJyk7XG52YXIgSW5wdXRTZWFyY2ggPSByZXF1aXJlKCcuLy4uL2lucHV0U2VhcmNoLmpzeCcpO1xudmFyIHtUYWJsZSwgQ29sdW1uLCBDZWxsLCBUZXh0Q2VsbCwgU29ydEhlYWRlckNlbGwsIFNvcnRUeXBlcywgRW1wdHlJbmRpY2F0b3J9ID0gcmVxdWlyZSgnYXBwL2NvbXBvbmVudHMvdGFibGUuanN4Jyk7XG52YXIge0J1dHRvbkNlbGwsIFNpbmdsZVVzZXJDZWxsLCBEYXRlQ3JlYXRlZENlbGx9ID0gcmVxdWlyZSgnLi9saXN0SXRlbXMnKTtcbnZhciB7RGF0ZVJhbmdlUGlja2VyfSA9IHJlcXVpcmUoJy4vLi4vZGF0ZVBpY2tlci5qc3gnKTtcbnZhciBtb21lbnQgPSAgcmVxdWlyZSgnbW9tZW50Jyk7XG52YXIge2lzTWF0Y2h9ID0gcmVxdWlyZSgnYXBwL2NvbW1vbi9vYmplY3RVdGlscycpO1xudmFyIF8gPSByZXF1aXJlKCdfJyk7XG52YXIge2Rpc3BsYXlEYXRlRm9ybWF0fSA9IHJlcXVpcmUoJ2FwcC9jb25maWcnKTtcblxudmFyIEFyY2hpdmVkU2Vzc2lvbnMgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgZ2V0SW5pdGlhbFN0YXRlKCl7XG4gICAgdGhpcy5zZWFyY2hhYmxlUHJvcHMgPSBbJ3NlcnZlcklwJywgJ2NyZWF0ZWQnLCAnc2lkJywgJ2xvZ2luJ107XG4gICAgcmV0dXJuIHsgZmlsdGVyOiAnJywgY29sU29ydERpcnM6IHtjcmVhdGVkOiAnQVNDJ319O1xuICB9LFxuXG4gIGNvbXBvbmVudFdpbGxNb3VudCgpe1xuICAgIHNldFRpbWVvdXQoKCk9PmFjdGlvbnMuZmV0Y2goKSwgMCk7XG4gIH0sXG5cbiAgY29tcG9uZW50V2lsbFVubW91bnQoKXtcbiAgICBhY3Rpb25zLnJlbW92ZVN0b3JlZFNlc3Npb25zKCk7XG4gIH0sXG5cbiAgb25GaWx0ZXJDaGFuZ2UodmFsdWUpe1xuICAgIHRoaXMuc3RhdGUuZmlsdGVyID0gdmFsdWU7XG4gICAgdGhpcy5zZXRTdGF0ZSh0aGlzLnN0YXRlKTtcbiAgfSxcblxuICBvblNvcnRDaGFuZ2UoY29sdW1uS2V5LCBzb3J0RGlyKSB7XG4gICAgdGhpcy5zdGF0ZS5jb2xTb3J0RGlycyA9IHsgW2NvbHVtbktleV06IHNvcnREaXIgfTtcbiAgICB0aGlzLnNldFN0YXRlKHRoaXMuc3RhdGUpO1xuICB9LFxuXG4gIG9uUmFuZ2VQaWNrZXJDaGFuZ2Uoe3N0YXJ0RGF0ZSwgZW5kRGF0ZX0pe1xuICAgIGFjdGlvbnMuc2V0VGltZVJhbmdlKHN0YXJ0RGF0ZSwgZW5kRGF0ZSk7XG4gIH0sXG5cbiAgc2VhcmNoQW5kRmlsdGVyQ2IodGFyZ2V0VmFsdWUsIHNlYXJjaFZhbHVlLCBwcm9wTmFtZSl7XG4gICAgaWYocHJvcE5hbWUgPT09ICdjcmVhdGVkJyl7XG4gICAgICB2YXIgZGlzcGxheURhdGUgPSBtb21lbnQodGFyZ2V0VmFsdWUpLmZvcm1hdChkaXNwbGF5RGF0ZUZvcm1hdCkudG9Mb2NhbGVVcHBlckNhc2UoKTtcbiAgICAgIHJldHVybiBkaXNwbGF5RGF0ZS5pbmRleE9mKHNlYXJjaFZhbHVlKSAhPT0gLTE7XG4gICAgfVxuICB9LFxuXG4gIHNvcnRBbmRGaWx0ZXIoZGF0YSl7XG4gICAgdmFyIGZpbHRlcmVkID0gZGF0YS5maWx0ZXIob2JqPT5cbiAgICAgIGlzTWF0Y2gob2JqLCB0aGlzLnN0YXRlLmZpbHRlciwge1xuICAgICAgICBzZWFyY2hhYmxlUHJvcHM6IHRoaXMuc2VhcmNoYWJsZVByb3BzLFxuICAgICAgICBjYjogdGhpcy5zZWFyY2hBbmRGaWx0ZXJDYlxuICAgICAgfSkpO1xuXG4gICAgdmFyIGNvbHVtbktleSA9IE9iamVjdC5nZXRPd25Qcm9wZXJ0eU5hbWVzKHRoaXMuc3RhdGUuY29sU29ydERpcnMpWzBdO1xuICAgIHZhciBzb3J0RGlyID0gdGhpcy5zdGF0ZS5jb2xTb3J0RGlyc1tjb2x1bW5LZXldO1xuICAgIHZhciBzb3J0ZWQgPSBfLnNvcnRCeShmaWx0ZXJlZCwgY29sdW1uS2V5KTtcbiAgICBpZihzb3J0RGlyID09PSBTb3J0VHlwZXMuQVNDKXtcbiAgICAgIHNvcnRlZCA9IHNvcnRlZC5yZXZlcnNlKCk7XG4gICAgfVxuXG4gICAgcmV0dXJuIHNvcnRlZDtcbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIGxldCB7c3RhcnQsIGVuZCwgc3RhdHVzfSA9IHRoaXMucHJvcHMuZmlsdGVyO1xuICAgIGxldCBkYXRhID0gdGhpcy5wcm9wcy5kYXRhLmZpbHRlcihcbiAgICAgIGl0ZW0gPT4gIWl0ZW0uYWN0aXZlICYmIG1vbWVudChpdGVtLmNyZWF0ZWQpLmlzQmV0d2VlbihzdGFydCwgZW5kKSk7XG5cbiAgICBkYXRhID0gdGhpcy5zb3J0QW5kRmlsdGVyKGRhdGEpO1xuXG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LXNlc3Npb25zLXN0b3JlZFwiPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1oZWFkZXJcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4XCI+XG4gICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LWNvbHVtblwiPjwvZGl2PlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZmxleC1jb2x1bW5cIj5cbiAgICAgICAgICAgICAgPGgxPiBBcmNoaXZlZCBTZXNzaW9ucyA8L2gxPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LWNvbHVtblwiPlxuICAgICAgICAgICAgICA8SW5wdXRTZWFyY2ggdmFsdWU9e3RoaXMuZmlsdGVyfSBvbkNoYW5nZT17dGhpcy5vbkZpbHRlckNoYW5nZX0vPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZmxleFwiPlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZmxleC1yb3dcIj5cbiAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZmxleC1yb3dcIj5cbiAgICAgICAgICAgICAgPERhdGVSYW5nZVBpY2tlciBzdGFydERhdGU9e3N0YXJ0fSBlbmREYXRlPXtlbmR9IG9uQ2hhbmdlPXt0aGlzLm9uUmFuZ2VQaWNrZXJDaGFuZ2V9Lz5cbiAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZmxleC1yb3dcIj5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgPC9kaXY+XG4gICAgICAgIDwvZGl2PlxuXG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWNvbnRlbnRcIj5cbiAgICAgICAgICB7ZGF0YS5sZW5ndGggPT09IDAgJiYgIXN0YXR1cy5pc0xvYWRpbmcgPyA8RW1wdHlJbmRpY2F0b3IgdGV4dD1cIk5vIG1hdGNoaW5nIGFyY2hpdmVkIHNlc3Npb25zIGZvdW5kLlwiLz4gOlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICAgICAgPFRhYmxlIHJvd0NvdW50PXtkYXRhLmxlbmd0aH0gY2xhc3NOYW1lPVwidGFibGUtc3RyaXBlZFwiPlxuICAgICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICAgIGNvbHVtbktleT1cInNpZFwiXG4gICAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPiBTZXNzaW9uIElEIDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17PFRleHRDZWxsIGRhdGE9e2RhdGF9Lz4gfVxuICAgICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgICAgPENvbHVtblxuICAgICAgICAgICAgICAgICAgaGVhZGVyPXs8Q2VsbD4gPC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXtcbiAgICAgICAgICAgICAgICAgICAgPEJ1dHRvbkNlbGwgZGF0YT17ZGF0YX0gLz5cbiAgICAgICAgICAgICAgICAgIH1cbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICAgIGNvbHVtbktleT1cImNyZWF0ZWRcIlxuICAgICAgICAgICAgICAgICAgaGVhZGVyPXtcbiAgICAgICAgICAgICAgICAgICAgPFNvcnRIZWFkZXJDZWxsXG4gICAgICAgICAgICAgICAgICAgICAgc29ydERpcj17dGhpcy5zdGF0ZS5jb2xTb3J0RGlycy5jcmVhdGVkfVxuICAgICAgICAgICAgICAgICAgICAgIG9uU29ydENoYW5nZT17dGhpcy5vblNvcnRDaGFuZ2V9XG4gICAgICAgICAgICAgICAgICAgICAgdGl0bGU9XCJDcmVhdGVkXCJcbiAgICAgICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgICAgIH1cbiAgICAgICAgICAgICAgICAgIGNlbGw9ezxEYXRlQ3JlYXRlZENlbGwgZGF0YT17ZGF0YX0vPiB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPiBVc2VyIDwvQ2VsbD4gfVxuICAgICAgICAgICAgICAgICAgY2VsbD17PFNpbmdsZVVzZXJDZWxsIGRhdGE9e2RhdGF9Lz4gfVxuICAgICAgICAgICAgICAgIC8+XG4gICAgICAgICAgICAgIDwvVGFibGU+XG4gICAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICB9XG4gICAgICAgIDwvZGl2PlxuICAgICAgICB7XG4gICAgICAgICAgc3RhdHVzLmhhc01vcmUgP1xuICAgICAgICAgICAgKDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWZvb3RlclwiPlxuICAgICAgICAgICAgICA8YnV0dG9uIGRpc2FibGVkPXtzdGF0dXMuaXNMb2FkaW5nfSBjbGFzc05hbWU9XCJidG4gYnRuLXByaW1hcnkgYnRuLW91dGxpbmVcIiBvbkNsaWNrPXthY3Rpb25zLmZldGNoTW9yZX0+XG4gICAgICAgICAgICAgICAgPHNwYW4+TG9hZCBtb3JlLi4uPC9zcGFuPlxuICAgICAgICAgICAgICA8L2J1dHRvbj5cbiAgICAgICAgICAgIDwvZGl2PikgOiBudWxsXG4gICAgICAgIH1cbiAgICAgIDwvZGl2PlxuICAgIClcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gQXJjaGl2ZWRTZXNzaW9ucztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL3Nlc3Npb25zL3N0b3JlZFNlc3Npb25MaXN0LmpzeFxuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG52YXIgc2Vzc2lvbiA9IHJlcXVpcmUoJ2FwcC9zZXJ2aWNlcy9zZXNzaW9uJyk7XG52YXIgVGVybWluYWwgPSByZXF1aXJlKCdhcHAvY29tbW9uL3Rlcm1pbmFsJyk7XG52YXIge3VwZGF0ZVNlc3Npb259ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvc2Vzc2lvbnMvYWN0aW9ucycpO1xuXG52YXIgVHR5VGVybWluYWwgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgY29tcG9uZW50RGlkTW91bnQ6IGZ1bmN0aW9uKCkge1xuICAgIGxldCB7c2VydmVySWQsIGxvZ2luLCBzaWQsIHJvd3MsIGNvbHN9ID0gdGhpcy5wcm9wcztcbiAgICBsZXQge3Rva2VufSA9IHNlc3Npb24uZ2V0VXNlckRhdGEoKTtcbiAgICBsZXQgdXJsID0gY2ZnLmFwaS5nZXRUdHlVcmwoKTtcblxuICAgIGxldCBvcHRpb25zID0ge1xuICAgICAgdHR5OiB7XG4gICAgICAgIHNlcnZlcklkLCBsb2dpbiwgc2lkLCB0b2tlbiwgdXJsXG4gICAgICB9LFxuICAgICByb3dzLFxuICAgICBjb2xzLFxuICAgICBlbDogdGhpcy5yZWZzLmNvbnRhaW5lclxuICAgIH1cblxuICAgIHRoaXMudGVybWluYWwgPSBuZXcgVGVybWluYWwob3B0aW9ucyk7XG4gICAgdGhpcy50ZXJtaW5hbC50dHlFdmVudHMub24oJ2RhdGEnLCB1cGRhdGVTZXNzaW9uKTtcbiAgICB0aGlzLnRlcm1pbmFsLm9wZW4oKTtcbiAgfSxcblxuICBjb21wb25lbnRXaWxsVW5tb3VudDogZnVuY3Rpb24oKSB7XG4gICAgdGhpcy50ZXJtaW5hbC5kZXN0cm95KCk7XG4gIH0sXG5cbiAgc2hvdWxkQ29tcG9uZW50VXBkYXRlOiBmdW5jdGlvbigpIHtcbiAgICByZXR1cm4gZmFsc2U7XG4gIH0sXG5cbiAgcmVuZGVyKCkge1xuICAgIHJldHVybiAoIDxkaXYgcmVmPVwiY29udGFpbmVyXCI+ICA8L2Rpdj4gKTtcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gVHR5VGVybWluYWw7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy90ZXJtaW5hbC5qc3hcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgcmVuZGVyID0gcmVxdWlyZSgncmVhY3QtZG9tJykucmVuZGVyO1xudmFyIHsgUm91dGVyLCBSb3V0ZSwgUmVkaXJlY3QgfSA9IHJlcXVpcmUoJ3JlYWN0LXJvdXRlcicpO1xudmFyIHsgQXBwLCBMb2dpbiwgTm9kZXMsIFNlc3Npb25zLCBOZXdVc2VyLCBDdXJyZW50U2Vzc2lvbkhvc3QsIE1lc3NhZ2VQYWdlLCBOb3RGb3VuZCB9ID0gcmVxdWlyZSgnLi9jb21wb25lbnRzJyk7XG52YXIge2Vuc3VyZVVzZXJ9ID0gcmVxdWlyZSgnLi9tb2R1bGVzL3VzZXIvYWN0aW9ucycpO1xudmFyIGF1dGggPSByZXF1aXJlKCcuL3NlcnZpY2VzL2F1dGgnKTtcbnZhciBzZXNzaW9uID0gcmVxdWlyZSgnLi9zZXJ2aWNlcy9zZXNzaW9uJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnLi9jb25maWcnKTtcblxucmVxdWlyZSgnLi9tb2R1bGVzJyk7XG5cbi8vIGluaXQgc2Vzc2lvblxuc2Vzc2lvbi5pbml0KCk7XG5cbmNmZy5pbml0KHdpbmRvdy5HUlZfQ09ORklHKTtcblxucmVuZGVyKChcbiAgPFJvdXRlciBoaXN0b3J5PXtzZXNzaW9uLmdldEhpc3RvcnkoKX0+XG4gICAgPFJvdXRlIHBhdGg9e2NmZy5yb3V0ZXMubXNnc30gY29tcG9uZW50PXtNZXNzYWdlUGFnZX0vPlxuICAgIDxSb3V0ZSBwYXRoPXtjZmcucm91dGVzLmxvZ2lufSBjb21wb25lbnQ9e0xvZ2lufS8+XG4gICAgPFJvdXRlIHBhdGg9e2NmZy5yb3V0ZXMubG9nb3V0fSBvbkVudGVyPXthdXRoLmxvZ291dH0vPlxuICAgIDxSb3V0ZSBwYXRoPXtjZmcucm91dGVzLm5ld1VzZXJ9IGNvbXBvbmVudD17TmV3VXNlcn0vPlxuICAgIDxSZWRpcmVjdCBmcm9tPXtjZmcucm91dGVzLmFwcH0gdG89e2NmZy5yb3V0ZXMubm9kZXN9Lz5cbiAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5hcHB9IGNvbXBvbmVudD17QXBwfSBvbkVudGVyPXtlbnN1cmVVc2VyfSA+XG4gICAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5ub2Rlc30gY29tcG9uZW50PXtOb2Rlc30vPlxuICAgICAgPFJvdXRlIHBhdGg9e2NmZy5yb3V0ZXMuYWN0aXZlU2Vzc2lvbn0gY29tcG9uZW50cz17e0N1cnJlbnRTZXNzaW9uSG9zdDogQ3VycmVudFNlc3Npb25Ib3N0fX0vPlxuICAgICAgPFJvdXRlIHBhdGg9e2NmZy5yb3V0ZXMuc2Vzc2lvbnN9IGNvbXBvbmVudD17U2Vzc2lvbnN9Lz5cbiAgICA8L1JvdXRlPlxuICAgIDxSb3V0ZSBwYXRoPVwiKlwiIGNvbXBvbmVudD17Tm90Rm91bmR9IC8+XG4gIDwvUm91dGVyPlxuKSwgZG9jdW1lbnQuZ2V0RWxlbWVudEJ5SWQoXCJhcHBcIikpO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2luZGV4LmpzeFxuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHtmZXRjaFNlc3Npb25zfSA9IHJlcXVpcmUoJy4vLi4vc2Vzc2lvbnMvYWN0aW9ucycpO1xudmFyIHtmZXRjaE5vZGVzfSA9IHJlcXVpcmUoJy4vLi4vbm9kZXMvYWN0aW9ucycpO1xudmFyICQgPSByZXF1aXJlKCdqUXVlcnknKTtcblxuY29uc3QgeyBUTFBUX0FQUF9JTklULCBUTFBUX0FQUF9GQUlMRUQsIFRMUFRfQVBQX1JFQURZIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmNvbnN0IGFjdGlvbnMgPSB7XG5cbiAgaW5pdEFwcCgpIHtcbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfQVBQX0lOSVQpOyAgICBcbiAgICBhY3Rpb25zLmZldGNoTm9kZXNBbmRTZXNzaW9ucygpXG4gICAgICAuZG9uZSgoKT0+IHJlYWN0b3IuZGlzcGF0Y2goVExQVF9BUFBfUkVBRFkpIClcbiAgICAgIC5mYWlsKCgpPT4gcmVhY3Rvci5kaXNwYXRjaChUTFBUX0FQUF9GQUlMRUQpICk7XG4gIH0sXG5cbiAgZmV0Y2hOb2Rlc0FuZFNlc3Npb25zKCkge1xuICAgIHJldHVybiAkLndoZW4oZmV0Y2hOb2RlcygpLCBmZXRjaFNlc3Npb25zKCkpO1xuICB9XG59XG5cbmV4cG9ydCBkZWZhdWx0IGFjdGlvbnM7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hcHAvYWN0aW9ucy5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxuY29uc3QgYXBwU3RhdGUgPSBbWyd0bHB0J10sIGFwcD0+IGFwcC50b0pTKCldO1xuXG5leHBvcnQgZGVmYXVsdCB7XG4gIGFwcFN0YXRlXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9hcHAvZ2V0dGVycy5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cbm1vZHVsZS5leHBvcnRzLmdldHRlcnMgPSByZXF1aXJlKCcuL2dldHRlcnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGlvbnMgPSByZXF1aXJlKCcuL2FjdGlvbnMnKTtcbm1vZHVsZS5leHBvcnRzLmFwcFN0b3JlID0gcmVxdWlyZSgnLi9hcHBTdG9yZScpO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvYXBwL2luZGV4LmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG5jb25zdCBkaWFsb2dzID0gW1sndGxwdF9kaWFsb2dzJ10sIHN0YXRlPT4gc3RhdGUudG9KUygpXTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICBkaWFsb2dzXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9kaWFsb2dzL2dldHRlcnMuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbm1vZHVsZS5leHBvcnRzLmdldHRlcnMgPSByZXF1aXJlKCcuL2dldHRlcnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGlvbnMgPSByZXF1aXJlKCcuL2FjdGlvbnMnKTtcbm1vZHVsZS5leHBvcnRzLmRpYWxvZ1N0b3JlID0gcmVxdWlyZSgnLi9kaWFsb2dTdG9yZScpO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvZGlhbG9ncy9pbmRleC5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xucmVhY3Rvci5yZWdpc3RlclN0b3Jlcyh7XG4gICd0bHB0JzogcmVxdWlyZSgnLi9hcHAvYXBwU3RvcmUnKSxcbiAgJ3RscHRfZGlhbG9ncyc6IHJlcXVpcmUoJy4vZGlhbG9ncy9kaWFsb2dTdG9yZScpLFxuICAndGxwdF9jdXJyZW50X3Nlc3Npb24nOiByZXF1aXJlKCcuL2N1cnJlbnRTZXNzaW9uL2N1cnJlbnRTZXNzaW9uU3RvcmUnKSxcbiAgJ3RscHRfdXNlcic6IHJlcXVpcmUoJy4vdXNlci91c2VyU3RvcmUnKSxcbiAgJ3RscHRfdXNlcl9pbnZpdGUnOiByZXF1aXJlKCcuL3VzZXIvdXNlckludml0ZVN0b3JlJyksXG4gICd0bHB0X25vZGVzJzogcmVxdWlyZSgnLi9ub2Rlcy9ub2RlU3RvcmUnKSxcbiAgJ3RscHRfcmVzdF9hcGknOiByZXF1aXJlKCcuL3Jlc3RBcGkvcmVzdEFwaVN0b3JlJyksXG4gICd0bHB0X3Nlc3Npb25zJzogcmVxdWlyZSgnLi9zZXNzaW9ucy9zZXNzaW9uU3RvcmUnKSxcbiAgJ3RscHRfc3RvcmVkX3Nlc3Npb25zX2ZpbHRlcic6IHJlcXVpcmUoJy4vc3RvcmVkU2Vzc2lvbnNGaWx0ZXIvc3RvcmVkU2Vzc2lvbkZpbHRlclN0b3JlJyksXG4gICd0bHB0X25vdGlmaWNhdGlvbnMnOiByZXF1aXJlKCcuL25vdGlmaWNhdGlvbnMvbm90aWZpY2F0aW9uU3RvcmUnKVxufSk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9pbmRleC5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHsgVExQVF9OT0RFU19SRUNFSVZFIH0gID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xudmFyIGFwaSA9IHJlcXVpcmUoJ2FwcC9zZXJ2aWNlcy9hcGknKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG52YXIge3Nob3dFcnJvcn0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9ub3RpZmljYXRpb25zL2FjdGlvbnMnKTtcblxuY29uc3QgbG9nZ2VyID0gcmVxdWlyZSgnYXBwL2NvbW1vbi9sb2dnZXInKS5jcmVhdGUoJ01vZHVsZXMvTm9kZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuICBmZXRjaE5vZGVzKCl7XG4gICAgYXBpLmdldChjZmcuYXBpLm5vZGVzUGF0aCkuZG9uZSgoZGF0YT1bXSk9PntcbiAgICAgIHZhciBub2RlQXJyYXkgPSBkYXRhLm5vZGVzLm1hcChpdGVtPT5pdGVtLm5vZGUpO1xuICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX05PREVTX1JFQ0VJVkUsIG5vZGVBcnJheSk7XG4gICAgfSkuZmFpbCgoZXJyKT0+e1xuICAgICAgc2hvd0Vycm9yKCdVbmFibGUgdG8gcmV0cmlldmUgbGlzdCBvZiBub2RlcycpO1xuICAgICAgbG9nZ2VyLmVycm9yKCdmZXRjaE5vZGVzJywgZXJyKTtcbiAgICB9KVxuICB9XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9hY3Rpb25zLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgeyBTdG9yZSwgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcbnZhciAgeyBUTFBUX05PREVTX1JFQ0VJVkUgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKFtdKTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9OT0RFU19SRUNFSVZFLCByZWNlaXZlTm9kZXMpXG4gIH1cbn0pXG5cbmZ1bmN0aW9uIHJlY2VpdmVOb2RlcyhzdGF0ZSwgbm9kZUFycmF5KXtcbiAgcmV0dXJuIHRvSW1tdXRhYmxlKG5vZGVBcnJheSk7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9ub2RlU3RvcmUuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbmV4cG9ydCBjb25zdCBsYXN0TWVzc2FnZSA9XG4gIFsgWyd0bHB0X25vdGlmaWNhdGlvbnMnXSwgbm90aWZpY2F0aW9ucyA9PiBub3RpZmljYXRpb25zLmxhc3QoKSBdO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvbm90aWZpY2F0aW9ucy9nZXR0ZXJzLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG5pbXBvcnQgeyBTdG9yZSwgSW1tdXRhYmxlIH0gZnJvbSAnbnVjbGVhci1qcyc7XG5pbXBvcnQge1RMUFRfTk9USUZJQ0FUSU9OU19BRER9IGZyb20gJy4vYWN0aW9uVHlwZXMnO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gbmV3IEltbXV0YWJsZS5PcmRlcmVkTWFwKCk7XG4gIH0sXG5cbiAgaW5pdGlhbGl6ZSgpIHtcbiAgICB0aGlzLm9uKFRMUFRfTk9USUZJQ0FUSU9OU19BREQsIGFkZE5vdGlmaWNhdGlvbik7XG4gIH1cbn0pO1xuXG5mdW5jdGlvbiBhZGROb3RpZmljYXRpb24oc3RhdGUsIG1lc3NhZ2UpIHtcbiAgcmV0dXJuIHN0YXRlLnNldChzdGF0ZS5zaXplLCBtZXNzYWdlKTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL25vdGlmaWNhdGlvbnMvbm90aWZpY2F0aW9uU3RvcmUuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcblxudmFyIHtcbiAgVExQVF9SRVNUX0FQSV9TVEFSVCxcbiAgVExQVF9SRVNUX0FQSV9TVUNDRVNTLFxuICBUTFBUX1JFU1RfQVBJX0ZBSUwgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQge1xuXG4gIHN0YXJ0KHJlcVR5cGUpe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRVNUX0FQSV9TVEFSVCwge3R5cGU6IHJlcVR5cGV9KTtcbiAgfSxcblxuICBmYWlsKHJlcVR5cGUsIG1lc3NhZ2Upe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRVNUX0FQSV9GQUlMLCAge3R5cGU6IHJlcVR5cGUsIG1lc3NhZ2V9KTtcbiAgfSxcblxuICBzdWNjZXNzKHJlcVR5cGUpe1xuICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRVNUX0FQSV9TVUNDRVNTLCB7dHlwZTogcmVxVHlwZX0pO1xuICB9XG5cbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvYWN0aW9ucy5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIGRlZmF1bHRPYmogPSB7XG4gIGlzUHJvY2Vzc2luZzogZmFsc2UsXG4gIGlzRXJyb3I6IGZhbHNlLFxuICBpc1N1Y2Nlc3M6IGZhbHNlLFxuICBtZXNzYWdlOiAnJ1xufVxuXG5jb25zdCByZXF1ZXN0U3RhdHVzID0gKHJlcVR5cGUpID0+ICBbIFsndGxwdF9yZXN0X2FwaScsIHJlcVR5cGVdLCAoYXR0ZW1wKSA9PiB7XG4gIHJldHVybiBhdHRlbXAgPyBhdHRlbXAudG9KUygpIDogZGVmYXVsdE9iajtcbiB9XG5dO1xuXG5leHBvcnQgZGVmYXVsdCB7ICByZXF1ZXN0U3RhdHVzICB9O1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9nZXR0ZXJzLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgeyBTdG9yZSwgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcbnZhciB7XG4gIFRMUFRfUkVTVF9BUElfU1RBUlQsXG4gIFRMUFRfUkVTVF9BUElfU1VDQ0VTUyxcbiAgVExQVF9SRVNUX0FQSV9GQUlMIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IFN0b3JlKHtcbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB0b0ltbXV0YWJsZSh7fSk7XG4gIH0sXG5cbiAgaW5pdGlhbGl6ZSgpIHtcbiAgICB0aGlzLm9uKFRMUFRfUkVTVF9BUElfU1RBUlQsIHN0YXJ0KTtcbiAgICB0aGlzLm9uKFRMUFRfUkVTVF9BUElfRkFJTCwgZmFpbCk7XG4gICAgdGhpcy5vbihUTFBUX1JFU1RfQVBJX1NVQ0NFU1MsIHN1Y2Nlc3MpO1xuICB9XG59KVxuXG5mdW5jdGlvbiBzdGFydChzdGF0ZSwgcmVxdWVzdCl7XG4gIHJldHVybiBzdGF0ZS5zZXQocmVxdWVzdC50eXBlLCB0b0ltbXV0YWJsZSh7aXNQcm9jZXNzaW5nOiB0cnVlfSkpO1xufVxuXG5mdW5jdGlvbiBmYWlsKHN0YXRlLCByZXF1ZXN0KXtcbiAgcmV0dXJuIHN0YXRlLnNldChyZXF1ZXN0LnR5cGUsIHRvSW1tdXRhYmxlKHtpc0ZhaWxlZDogdHJ1ZSwgbWVzc2FnZTogcmVxdWVzdC5tZXNzYWdlfSkpO1xufVxuXG5mdW5jdGlvbiBzdWNjZXNzKHN0YXRlLCByZXF1ZXN0KXtcbiAgcmV0dXJuIHN0YXRlLnNldChyZXF1ZXN0LnR5cGUsIHRvSW1tdXRhYmxlKHtpc1N1Y2Nlc3M6IHRydWV9KSk7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL3Jlc3RBcGlTdG9yZS5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxubW9kdWxlLmV4cG9ydHMuZ2V0dGVycyA9IHJlcXVpcmUoJy4vZ2V0dGVycycpO1xubW9kdWxlLmV4cG9ydHMuYWN0aW9ucyA9IHJlcXVpcmUoJy4vYWN0aW9ucycpO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvc2Vzc2lvbnMvaW5kZXguanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5cbnZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyIHsgVExQVF9TRVNTSU5TX1JFQ0VJVkUsIFRMUFRfU0VTU0lOU19VUERBVEUsIFRMUFRfU0VTU0lOU19SRU1PVkVfU1RPUkVEIH0gID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gdG9JbW11dGFibGUoe30pO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX1NFU1NJTlNfUkVDRUlWRSwgcmVjZWl2ZVNlc3Npb25zKTtcbiAgICB0aGlzLm9uKFRMUFRfU0VTU0lOU19VUERBVEUsIHVwZGF0ZVNlc3Npb24pO1xuICAgIHRoaXMub24oVExQVF9TRVNTSU5TX1JFTU9WRV9TVE9SRUQsIHJlbW92ZVN0b3JlZFNlc3Npb25zKTtcbiAgfVxufSlcblxuZnVuY3Rpb24gcmVtb3ZlU3RvcmVkU2Vzc2lvbnMoc3RhdGUpe1xuICByZXR1cm4gc3RhdGUud2l0aE11dGF0aW9ucyhzdGF0ZSA9PiB7XG4gICAgc3RhdGUudmFsdWVTZXEoKS5mb3JFYWNoKGl0ZW09PiB7XG4gICAgICBpZihpdGVtLmdldCgnYWN0aXZlJykgIT09IHRydWUpe1xuICAgICAgICBzdGF0ZS5yZW1vdmUoaXRlbS5nZXQoJ2lkJykpO1xuICAgICAgfVxuICAgIH0pO1xuICB9KTtcbn1cblxuZnVuY3Rpb24gdXBkYXRlU2Vzc2lvbihzdGF0ZSwganNvbil7XG4gIHJldHVybiBzdGF0ZS5zZXQoanNvbi5pZCwgdG9JbW11dGFibGUoanNvbikpO1xufVxuXG5mdW5jdGlvbiByZWNlaXZlU2Vzc2lvbnMoc3RhdGUsIGpzb25BcnJheT1bXSl7XG4gIHJldHVybiBzdGF0ZS53aXRoTXV0YXRpb25zKHN0YXRlID0+IHtcbiAgICBqc29uQXJyYXkuZm9yRWFjaCgoaXRlbSkgPT4ge1xuICAgICAgaXRlbS5jcmVhdGVkID0gbmV3IERhdGUoaXRlbS5jcmVhdGVkKTtcbiAgICAgIGl0ZW0ubGFzdF9hY3RpdmUgPSBuZXcgRGF0ZShpdGVtLmxhc3RfYWN0aXZlKTtcbiAgICAgIHN0YXRlLnNldChpdGVtLmlkLCB0b0ltbXV0YWJsZShpdGVtKSlcbiAgICB9KVxuICB9KTtcbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Nlc3Npb25zL3Nlc3Npb25TdG9yZS5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xudmFyIHtmaWx0ZXJ9ID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG52YXIge21heFNlc3Npb25Mb2FkU2l6ZX0gPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG52YXIgbW9tZW50ID0gcmVxdWlyZSgnbW9tZW50Jyk7XG52YXIgYXBpVXRpbHMgPSByZXF1aXJlKCdhcHAvc2VydmljZXMvYXBpVXRpbHMnKTtcblxudmFyIHtzaG93RXJyb3J9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvbm90aWZpY2F0aW9ucy9hY3Rpb25zJyk7XG5cbmNvbnN0IGxvZ2dlciA9IHJlcXVpcmUoJ2FwcC9jb21tb24vbG9nZ2VyJykuY3JlYXRlKCdNb2R1bGVzL1Nlc3Npb25zJyk7XG5cbmNvbnN0IHtcbiAgVExQVF9TVE9SRURfU0VTU0lOU19GSUxURVJfU0VUX1JBTkdFLFxuICBUTFBUX1NUT1JFRF9TRVNTSU5TX0ZJTFRFUl9TRVRfU1RBVFVTIH0gID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5jb25zdCB7VExQVF9TRVNTSU5TX1JFQ0VJVkUsIFRMUFRfU0VTU0lOU19SRU1PVkVfU1RPUkVEIH0gID0gcmVxdWlyZSgnLi8uLi9zZXNzaW9ucy9hY3Rpb25UeXBlcycpO1xuXG4vKipcbiogRHVlIHRvIGN1cnJlbnQgbGltaXRhdGlvbnMgb2YgdGhlIGJhY2tlbmQgQVBJLCB0aGUgZmlsdGVyaW5nIGxvZ2ljIGZvciB0aGUgQXJjaGl2ZWQgbGlzdCBvZiBTZXNzaW9uXG4qIHdvcmtzIGFzIGZvbGxvd3M6XG4qICAxKSBlYWNoIHRpbWUgYSBuZXcgZGF0ZSByYW5nZSBpcyBzZXQsIGFsbCBwcmV2aW91c2x5IHJldHJpZXZlZCBpbmFjdGl2ZSBzZXNzaW9ucyBnZXQgZGVsZXRlZC5cbiogIDIpIGhhc01vcmUgZmxhZyB3aWxsIGJlIGRldGVybWluZSBhZnRlciBhIGNvbnNlcXVlbnQgZmV0Y2ggcmVxdWVzdCB3aXRoIG5ldyBkYXRlIHJhbmdlIHZhbHVlcy5cbiovXG5cbmNvbnN0IGFjdGlvbnMgPSB7XG5cbiAgZmV0Y2goKXtcbiAgICBsZXQgeyBlbmQgfSA9IHJlYWN0b3IuZXZhbHVhdGUoZmlsdGVyKTtcbiAgICBfZmV0Y2goZW5kKTtcbiAgfSxcblxuICBmZXRjaE1vcmUoKXtcbiAgICBsZXQge3N0YXR1cywgZW5kIH0gPSByZWFjdG9yLmV2YWx1YXRlKGZpbHRlcik7XG4gICAgaWYoc3RhdHVzLmhhc01vcmUgPT09IHRydWUgJiYgIXN0YXR1cy5pc0xvYWRpbmcpe1xuICAgICAgX2ZldGNoKGVuZCwgc3RhdHVzLnNpZCk7XG4gICAgfVxuICB9LFxuXG4gIHJlbW92ZVN0b3JlZFNlc3Npb25zKCl7XG4gICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1NFU1NJTlNfUkVNT1ZFX1NUT1JFRCk7XG4gIH0sXG5cbiAgc2V0VGltZVJhbmdlKHN0YXJ0LCBlbmQpe1xuICAgIHJlYWN0b3IuYmF0Y2goKCk9PntcbiAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9TVE9SRURfU0VTU0lOU19GSUxURVJfU0VUX1JBTkdFLCB7c3RhcnQsIGVuZCwgaGFzTW9yZTogZmFsc2V9KTtcbiAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9TRVNTSU5TX1JFTU9WRV9TVE9SRUQpO1xuICAgICAgX2ZldGNoKGVuZCk7XG4gICAgfSk7XG4gIH1cbn1cblxuZnVuY3Rpb24gX2ZldGNoKGVuZCwgc2lkKXtcbiAgbGV0IHN0YXR1cyA9IHtcbiAgICBoYXNNb3JlOiBmYWxzZSxcbiAgICBpc0xvYWRpbmc6IHRydWVcbiAgfVxuXG4gIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9TVE9SRURfU0VTU0lOU19GSUxURVJfU0VUX1NUQVRVUywgc3RhdHVzKTtcblxuICBsZXQgc3RhcnQgPSBlbmQgfHwgbmV3IERhdGUoKTtcbiAgbGV0IHBhcmFtcyA9IHtcbiAgICBvcmRlcjogLTEsXG4gICAgbGltaXQ6IG1heFNlc3Npb25Mb2FkU2l6ZSxcbiAgICBzdGFydCxcbiAgICBzaWRcbiAgfTtcblxuICByZXR1cm4gYXBpVXRpbHMuZmlsdGVyU2Vzc2lvbnMocGFyYW1zKS5kb25lKChqc29uKSA9PiB7XG4gICAgbGV0IHtzZXNzaW9uc30gPSBqc29uO1xuICAgIGxldCB7c3RhcnR9ID0gcmVhY3Rvci5ldmFsdWF0ZShmaWx0ZXIpO1xuXG4gICAgc3RhdHVzLmhhc01vcmUgPSBmYWxzZTtcbiAgICBzdGF0dXMuaXNMb2FkaW5nID0gZmFsc2U7XG5cbiAgICBpZiAoc2Vzc2lvbnMubGVuZ3RoID09PSBtYXhTZXNzaW9uTG9hZFNpemUpIHtcbiAgICAgIGxldCB7aWQsIGNyZWF0ZWR9ID0gc2Vzc2lvbnNbc2Vzc2lvbnMubGVuZ3RoLTFdO1xuICAgICAgc3RhdHVzLnNpZCA9IGlkO1xuICAgICAgc3RhdHVzLmhhc01vcmUgPSBtb21lbnQoc3RhcnQpLmlzQmVmb3JlKGNyZWF0ZWQpO1xuXG4gICAgICAvKipcbiAgICAgICogcmVtb3ZlIGF0IGxlYXN0IDEgaXRlbSBiZWZvcmUgc3RvcmluZyB0aGUgc2Vzc2lvbnMsIHRoaXMgd2F5IHdlIGVuc3VyZSB0aGF0XG4gICAgICAqIHRoZXJlIHdpbGwgYmUgYWx3YXlzIGF0IGxlYXN0IG9uZSBpdGVtIG9uIHRoZSBuZXh0ICdmZXRjaE1vcmUnIHJlcXVlc3QuXG4gICAgICAqL1xuICAgICAgc2Vzc2lvbnMgPSBzZXNzaW9ucy5zbGljZSgwLCBtYXhTZXNzaW9uTG9hZFNpemUtMSk7XG4gICAgfVxuXG4gICAgcmVhY3Rvci5iYXRjaCgoKT0+e1xuICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1NFU1NJTlNfUkVDRUlWRSwgc2Vzc2lvbnMpO1xuICAgICAgcmVhY3Rvci5kaXNwYXRjaChUTFBUX1NUT1JFRF9TRVNTSU5TX0ZJTFRFUl9TRVRfU1RBVFVTLCBzdGF0dXMpO1xuICAgIH0pO1xuXG4gIH0pXG4gIC5mYWlsKChlcnIpPT57XG4gICAgc2hvd0Vycm9yKCdVbmFibGUgdG8gcmV0cmlldmUgbGlzdCBvZiBzZXNzaW9ucycpO1xuICAgIGxvZ2dlci5lcnJvcignZmV0Y2hpbmcgZmlsdGVyZWQgc2V0IG9mIHNlc3Npb25zJywgZXJyKTtcbiAgfSk7XG5cbn1cblxuZXhwb3J0IGRlZmF1bHQgYWN0aW9ucztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3N0b3JlZFNlc3Npb25zRmlsdGVyL2FjdGlvbnMuanNcbiAqKi8iLCIvKlxuQ29weXJpZ2h0IDIwMTUgR3Jhdml0YXRpb25hbCwgSW5jLlxuXG5MaWNlbnNlZCB1bmRlciB0aGUgQXBhY2hlIExpY2Vuc2UsIFZlcnNpb24gMi4wICh0aGUgXCJMaWNlbnNlXCIpO1xueW91IG1heSBub3QgdXNlIHRoaXMgZmlsZSBleGNlcHQgaW4gY29tcGxpYW5jZSB3aXRoIHRoZSBMaWNlbnNlLlxuWW91IG1heSBvYnRhaW4gYSBjb3B5IG9mIHRoZSBMaWNlbnNlIGF0XG5cbiAgICBodHRwOi8vd3d3LmFwYWNoZS5vcmcvbGljZW5zZXMvTElDRU5TRS0yLjBcblxuVW5sZXNzIHJlcXVpcmVkIGJ5IGFwcGxpY2FibGUgbGF3IG9yIGFncmVlZCB0byBpbiB3cml0aW5nLCBzb2Z0d2FyZVxuZGlzdHJpYnV0ZWQgdW5kZXIgdGhlIExpY2Vuc2UgaXMgZGlzdHJpYnV0ZWQgb24gYW4gXCJBUyBJU1wiIEJBU0lTLFxuV0lUSE9VVCBXQVJSQU5USUVTIE9SIENPTkRJVElPTlMgT0YgQU5ZIEtJTkQsIGVpdGhlciBleHByZXNzIG9yIGltcGxpZWQuXG5TZWUgdGhlIExpY2Vuc2UgZm9yIHRoZSBzcGVjaWZpYyBsYW5ndWFnZSBnb3Zlcm5pbmcgcGVybWlzc2lvbnMgYW5kXG5saW1pdGF0aW9ucyB1bmRlciB0aGUgTGljZW5zZS5cbiovXG5tb2R1bGUuZXhwb3J0cy5nZXR0ZXJzID0gcmVxdWlyZSgnLi9nZXR0ZXJzJyk7XG5tb2R1bGUuZXhwb3J0cy5hY3Rpb25zID0gcmVxdWlyZSgnLi9hY3Rpb25zJyk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9zdG9yZWRTZXNzaW9uc0ZpbHRlci9pbmRleC5qc1xuICoqLyIsIi8qXG5Db3B5cmlnaHQgMjAxNSBHcmF2aXRhdGlvbmFsLCBJbmMuXG5cbkxpY2Vuc2VkIHVuZGVyIHRoZSBBcGFjaGUgTGljZW5zZSwgVmVyc2lvbiAyLjAgKHRoZSBcIkxpY2Vuc2VcIik7XG55b3UgbWF5IG5vdCB1c2UgdGhpcyBmaWxlIGV4Y2VwdCBpbiBjb21wbGlhbmNlIHdpdGggdGhlIExpY2Vuc2UuXG5Zb3UgbWF5IG9idGFpbiBhIGNvcHkgb2YgdGhlIExpY2Vuc2UgYXRcblxuICAgIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuXG5Vbmxlc3MgcmVxdWlyZWQgYnkgYXBwbGljYWJsZSBsYXcgb3IgYWdyZWVkIHRvIGluIHdyaXRpbmcsIHNvZnR3YXJlXG5kaXN0cmlidXRlZCB1bmRlciB0aGUgTGljZW5zZSBpcyBkaXN0cmlidXRlZCBvbiBhbiBcIkFTIElTXCIgQkFTSVMsXG5XSVRIT1VUIFdBUlJBTlRJRVMgT1IgQ09ORElUSU9OUyBPRiBBTlkgS0lORCwgZWl0aGVyIGV4cHJlc3Mgb3IgaW1wbGllZC5cblNlZSB0aGUgTGljZW5zZSBmb3IgdGhlIHNwZWNpZmljIGxhbmd1YWdlIGdvdmVybmluZyBwZXJtaXNzaW9ucyBhbmRcbmxpbWl0YXRpb25zIHVuZGVyIHRoZSBMaWNlbnNlLlxuKi9cblxudmFyIHsgU3RvcmUsIHRvSW1tdXRhYmxlIH0gPSByZXF1aXJlKCdudWNsZWFyLWpzJyk7XG52YXIgbW9tZW50ID0gcmVxdWlyZSgnbW9tZW50Jyk7XG5cbnZhciB7XG4gIFRMUFRfU1RPUkVEX1NFU1NJTlNfRklMVEVSX1NFVF9SQU5HRSxcbiAgVExQVF9TVE9SRURfU0VTU0lOU19GSUxURVJfU0VUX1NUQVRVUyB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcblxuICAgIGxldCBlbmQgPSBtb21lbnQobmV3IERhdGUoKSkuZW5kT2YoJ2RheScpLnRvRGF0ZSgpO1xuICAgIGxldCBzdGFydCA9IG1vbWVudChlbmQpLnN1YnRyYWN0KDMsICdkYXknKS5zdGFydE9mKCdkYXknKS50b0RhdGUoKTtcbiAgICBsZXQgc3RhdGUgPSB7XG4gICAgICBzdGFydCxcbiAgICAgIGVuZCxcbiAgICAgIHN0YXR1czoge1xuICAgICAgICBpc0xvYWRpbmc6IGZhbHNlLFxuICAgICAgICBoYXNNb3JlOiBmYWxzZVxuICAgICAgfVxuICAgIH1cblxuICAgIHJldHVybiB0b0ltbXV0YWJsZShzdGF0ZSk7XG4gIH0sXG5cbiAgaW5pdGlhbGl6ZSgpIHtcbiAgICB0aGlzLm9uKFRMUFRfU1RPUkVEX1NFU1NJTlNfRklMVEVSX1NFVF9SQU5HRSwgc2V0UmFuZ2UpO1xuICAgIHRoaXMub24oVExQVF9TVE9SRURfU0VTU0lOU19GSUxURVJfU0VUX1NUQVRVUywgc2V0U3RhdHVzKTtcbiAgfVxufSlcblxuZnVuY3Rpb24gc2V0U3RhdHVzKHN0YXRlLCBzdGF0dXMpe1xuICByZXR1cm4gc3RhdGUubWVyZ2VJbihbJ3N0YXR1cyddLCBzdGF0dXMpO1xufVxuXG5mdW5jdGlvbiBzZXRSYW5nZShzdGF0ZSwgbmV3U3RhdGUpe1xuICByZXR1cm4gc3RhdGUubWVyZ2UobmV3U3RhdGUpO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvc3RvcmVkU2Vzc2lvbnNGaWx0ZXIvc3RvcmVkU2Vzc2lvbkZpbHRlclN0b3JlLmpzXG4gKiovIiwiLypcbkNvcHlyaWdodCAyMDE1IEdyYXZpdGF0aW9uYWwsIEluYy5cblxuTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbnlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbllvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuXG4gICAgaHR0cDovL3d3dy5hcGFjaGUub3JnL2xpY2Vuc2VzL0xJQ0VOU0UtMi4wXG5cblVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbmRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbldJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxubGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4qL1xuXG52YXIgeyBTdG9yZSwgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcbnZhciAgeyBUTFBUX1JFQ0VJVkVfVVNFUl9JTlZJVEUgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKG51bGwpO1xuICB9LFxuXG4gIGluaXRpYWxpemUoKSB7XG4gICAgdGhpcy5vbihUTFBUX1JFQ0VJVkVfVVNFUl9JTlZJVEUsIHJlY2VpdmVJbnZpdGUpXG4gIH1cbn0pXG5cbmZ1bmN0aW9uIHJlY2VpdmVJbnZpdGUoc3RhdGUsIGludml0ZSl7XG4gIHJldHVybiB0b0ltbXV0YWJsZShpbnZpdGUpO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvdXNlci91c2VySW52aXRlU3RvcmUuanNcbiAqKi8iLCIvKipcbiAqIENvcHlyaWdodCAyMDEzLTIwMTUsIEZhY2Vib29rLCBJbmMuXG4gKiBBbGwgcmlnaHRzIHJlc2VydmVkLlxuICpcbiAqIFRoaXMgc291cmNlIGNvZGUgaXMgbGljZW5zZWQgdW5kZXIgdGhlIEJTRC1zdHlsZSBsaWNlbnNlIGZvdW5kIGluIHRoZVxuICogTElDRU5TRSBmaWxlIGluIHRoZSByb290IGRpcmVjdG9yeSBvZiB0aGlzIHNvdXJjZSB0cmVlLiBBbiBhZGRpdGlvbmFsIGdyYW50XG4gKiBvZiBwYXRlbnQgcmlnaHRzIGNhbiBiZSBmb3VuZCBpbiB0aGUgUEFURU5UUyBmaWxlIGluIHRoZSBzYW1lIGRpcmVjdG9yeS5cbiAqXG4gKiBAdHlwZWNoZWNrc1xuICogQHByb3ZpZGVzTW9kdWxlIFJlYWN0Q1NTVHJhbnNpdGlvbkdyb3VwXG4gKi9cblxuJ3VzZSBzdHJpY3QnO1xuXG52YXIgUmVhY3QgPSByZXF1aXJlKCcuL1JlYWN0Jyk7XG5cbnZhciBhc3NpZ24gPSByZXF1aXJlKCcuL09iamVjdC5hc3NpZ24nKTtcblxudmFyIFJlYWN0VHJhbnNpdGlvbkdyb3VwID0gcmVxdWlyZSgnLi9SZWFjdFRyYW5zaXRpb25Hcm91cCcpO1xudmFyIFJlYWN0Q1NTVHJhbnNpdGlvbkdyb3VwQ2hpbGQgPSByZXF1aXJlKCcuL1JlYWN0Q1NTVHJhbnNpdGlvbkdyb3VwQ2hpbGQnKTtcblxuZnVuY3Rpb24gY3JlYXRlVHJhbnNpdGlvblRpbWVvdXRQcm9wVmFsaWRhdG9yKHRyYW5zaXRpb25UeXBlKSB7XG4gIHZhciB0aW1lb3V0UHJvcE5hbWUgPSAndHJhbnNpdGlvbicgKyB0cmFuc2l0aW9uVHlwZSArICdUaW1lb3V0JztcbiAgdmFyIGVuYWJsZWRQcm9wTmFtZSA9ICd0cmFuc2l0aW9uJyArIHRyYW5zaXRpb25UeXBlO1xuXG4gIHJldHVybiBmdW5jdGlvbiAocHJvcHMpIHtcbiAgICAvLyBJZiB0aGUgdHJhbnNpdGlvbiBpcyBlbmFibGVkXG4gICAgaWYgKHByb3BzW2VuYWJsZWRQcm9wTmFtZV0pIHtcbiAgICAgIC8vIElmIG5vIHRpbWVvdXQgZHVyYXRpb24gaXMgcHJvdmlkZWRcbiAgICAgIGlmIChwcm9wc1t0aW1lb3V0UHJvcE5hbWVdID09IG51bGwpIHtcbiAgICAgICAgcmV0dXJuIG5ldyBFcnJvcih0aW1lb3V0UHJvcE5hbWUgKyAnIHdhc25cXCd0IHN1cHBsaWVkIHRvIFJlYWN0Q1NTVHJhbnNpdGlvbkdyb3VwOiAnICsgJ3RoaXMgY2FuIGNhdXNlIHVucmVsaWFibGUgYW5pbWF0aW9ucyBhbmQgd29uXFwndCBiZSBzdXBwb3J0ZWQgaW4gJyArICdhIGZ1dHVyZSB2ZXJzaW9uIG9mIFJlYWN0LiBTZWUgJyArICdodHRwczovL2ZiLm1lL3JlYWN0LWFuaW1hdGlvbi10cmFuc2l0aW9uLWdyb3VwLXRpbWVvdXQgZm9yIG1vcmUgJyArICdpbmZvcm1hdGlvbi4nKTtcblxuICAgICAgICAvLyBJZiB0aGUgZHVyYXRpb24gaXNuJ3QgYSBudW1iZXJcbiAgICAgIH0gZWxzZSBpZiAodHlwZW9mIHByb3BzW3RpbWVvdXRQcm9wTmFtZV0gIT09ICdudW1iZXInKSB7XG4gICAgICAgICAgcmV0dXJuIG5ldyBFcnJvcih0aW1lb3V0UHJvcE5hbWUgKyAnIG11c3QgYmUgYSBudW1iZXIgKGluIG1pbGxpc2Vjb25kcyknKTtcbiAgICAgICAgfVxuICAgIH1cbiAgfTtcbn1cblxudmFyIFJlYWN0Q1NTVHJhbnNpdGlvbkdyb3VwID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICBkaXNwbGF5TmFtZTogJ1JlYWN0Q1NTVHJhbnNpdGlvbkdyb3VwJyxcblxuICBwcm9wVHlwZXM6IHtcbiAgICB0cmFuc2l0aW9uTmFtZTogUmVhY3RDU1NUcmFuc2l0aW9uR3JvdXBDaGlsZC5wcm9wVHlwZXMubmFtZSxcblxuICAgIHRyYW5zaXRpb25BcHBlYXI6IFJlYWN0LlByb3BUeXBlcy5ib29sLFxuICAgIHRyYW5zaXRpb25FbnRlcjogUmVhY3QuUHJvcFR5cGVzLmJvb2wsXG4gICAgdHJhbnNpdGlvbkxlYXZlOiBSZWFjdC5Qcm9wVHlwZXMuYm9vbCxcbiAgICB0cmFuc2l0aW9uQXBwZWFyVGltZW91dDogY3JlYXRlVHJhbnNpdGlvblRpbWVvdXRQcm9wVmFsaWRhdG9yKCdBcHBlYXInKSxcbiAgICB0cmFuc2l0aW9uRW50ZXJUaW1lb3V0OiBjcmVhdGVUcmFuc2l0aW9uVGltZW91dFByb3BWYWxpZGF0b3IoJ0VudGVyJyksXG4gICAgdHJhbnNpdGlvbkxlYXZlVGltZW91dDogY3JlYXRlVHJhbnNpdGlvblRpbWVvdXRQcm9wVmFsaWRhdG9yKCdMZWF2ZScpXG4gIH0sXG5cbiAgZ2V0RGVmYXVsdFByb3BzOiBmdW5jdGlvbiAoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIHRyYW5zaXRpb25BcHBlYXI6IGZhbHNlLFxuICAgICAgdHJhbnNpdGlvbkVudGVyOiB0cnVlLFxuICAgICAgdHJhbnNpdGlvbkxlYXZlOiB0cnVlXG4gICAgfTtcbiAgfSxcblxuICBfd3JhcENoaWxkOiBmdW5jdGlvbiAoY2hpbGQpIHtcbiAgICAvLyBXZSBuZWVkIHRvIHByb3ZpZGUgdGhpcyBjaGlsZEZhY3Rvcnkgc28gdGhhdFxuICAgIC8vIFJlYWN0Q1NTVHJhbnNpdGlvbkdyb3VwQ2hpbGQgY2FuIHJlY2VpdmUgdXBkYXRlcyB0byBuYW1lLCBlbnRlciwgYW5kXG4gICAgLy8gbGVhdmUgd2hpbGUgaXQgaXMgbGVhdmluZy5cbiAgICByZXR1cm4gUmVhY3QuY3JlYXRlRWxlbWVudChSZWFjdENTU1RyYW5zaXRpb25Hcm91cENoaWxkLCB7XG4gICAgICBuYW1lOiB0aGlzLnByb3BzLnRyYW5zaXRpb25OYW1lLFxuICAgICAgYXBwZWFyOiB0aGlzLnByb3BzLnRyYW5zaXRpb25BcHBlYXIsXG4gICAgICBlbnRlcjogdGhpcy5wcm9wcy50cmFuc2l0aW9uRW50ZXIsXG4gICAgICBsZWF2ZTogdGhpcy5wcm9wcy50cmFuc2l0aW9uTGVhdmUsXG4gICAgICBhcHBlYXJUaW1lb3V0OiB0aGlzLnByb3BzLnRyYW5zaXRpb25BcHBlYXJUaW1lb3V0LFxuICAgICAgZW50ZXJUaW1lb3V0OiB0aGlzLnByb3BzLnRyYW5zaXRpb25FbnRlclRpbWVvdXQsXG4gICAgICBsZWF2ZVRpbWVvdXQ6IHRoaXMucHJvcHMudHJhbnNpdGlvbkxlYXZlVGltZW91dFxuICAgIH0sIGNoaWxkKTtcbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uICgpIHtcbiAgICByZXR1cm4gUmVhY3QuY3JlYXRlRWxlbWVudChSZWFjdFRyYW5zaXRpb25Hcm91cCwgYXNzaWduKHt9LCB0aGlzLnByb3BzLCB7IGNoaWxkRmFjdG9yeTogdGhpcy5fd3JhcENoaWxkIH0pKTtcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gUmVhY3RDU1NUcmFuc2l0aW9uR3JvdXA7XG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiAuL34vcmVhY3QvbGliL1JlYWN0Q1NTVHJhbnNpdGlvbkdyb3VwLmpzXG4gKiogbW9kdWxlIGlkID0gMzg2XG4gKiogbW9kdWxlIGNodW5rcyA9IDFcbiAqKi8iLCIvKipcbiAqIENvcHlyaWdodCAyMDEzLTIwMTUsIEZhY2Vib29rLCBJbmMuXG4gKiBBbGwgcmlnaHRzIHJlc2VydmVkLlxuICpcbiAqIFRoaXMgc291cmNlIGNvZGUgaXMgbGljZW5zZWQgdW5kZXIgdGhlIEJTRC1zdHlsZSBsaWNlbnNlIGZvdW5kIGluIHRoZVxuICogTElDRU5TRSBmaWxlIGluIHRoZSByb290IGRpcmVjdG9yeSBvZiB0aGlzIHNvdXJjZSB0cmVlLiBBbiBhZGRpdGlvbmFsIGdyYW50XG4gKiBvZiBwYXRlbnQgcmlnaHRzIGNhbiBiZSBmb3VuZCBpbiB0aGUgUEFURU5UUyBmaWxlIGluIHRoZSBzYW1lIGRpcmVjdG9yeS5cbiAqXG4gKiBAdHlwZWNoZWNrc1xuICogQHByb3ZpZGVzTW9kdWxlIFJlYWN0Q1NTVHJhbnNpdGlvbkdyb3VwQ2hpbGRcbiAqL1xuXG4ndXNlIHN0cmljdCc7XG5cbnZhciBSZWFjdCA9IHJlcXVpcmUoJy4vUmVhY3QnKTtcbnZhciBSZWFjdERPTSA9IHJlcXVpcmUoJy4vUmVhY3RET00nKTtcblxudmFyIENTU0NvcmUgPSByZXF1aXJlKCdmYmpzL2xpYi9DU1NDb3JlJyk7XG52YXIgUmVhY3RUcmFuc2l0aW9uRXZlbnRzID0gcmVxdWlyZSgnLi9SZWFjdFRyYW5zaXRpb25FdmVudHMnKTtcblxudmFyIG9ubHlDaGlsZCA9IHJlcXVpcmUoJy4vb25seUNoaWxkJyk7XG5cbi8vIFdlIGRvbid0IHJlbW92ZSB0aGUgZWxlbWVudCBmcm9tIHRoZSBET00gdW50aWwgd2UgcmVjZWl2ZSBhbiBhbmltYXRpb25lbmQgb3Jcbi8vIHRyYW5zaXRpb25lbmQgZXZlbnQuIElmIHRoZSB1c2VyIHNjcmV3cyB1cCBhbmQgZm9yZ2V0cyB0byBhZGQgYW4gYW5pbWF0aW9uXG4vLyB0aGVpciBub2RlIHdpbGwgYmUgc3R1Y2sgaW4gdGhlIERPTSBmb3JldmVyLCBzbyB3ZSBkZXRlY3QgaWYgYW4gYW5pbWF0aW9uXG4vLyBkb2VzIG5vdCBzdGFydCBhbmQgaWYgaXQgZG9lc24ndCwgd2UganVzdCBjYWxsIHRoZSBlbmQgbGlzdGVuZXIgaW1tZWRpYXRlbHkuXG52YXIgVElDSyA9IDE3O1xuXG52YXIgUmVhY3RDU1NUcmFuc2l0aW9uR3JvdXBDaGlsZCA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgZGlzcGxheU5hbWU6ICdSZWFjdENTU1RyYW5zaXRpb25Hcm91cENoaWxkJyxcblxuICBwcm9wVHlwZXM6IHtcbiAgICBuYW1lOiBSZWFjdC5Qcm9wVHlwZXMub25lT2ZUeXBlKFtSZWFjdC5Qcm9wVHlwZXMuc3RyaW5nLCBSZWFjdC5Qcm9wVHlwZXMuc2hhcGUoe1xuICAgICAgZW50ZXI6IFJlYWN0LlByb3BUeXBlcy5zdHJpbmcsXG4gICAgICBsZWF2ZTogUmVhY3QuUHJvcFR5cGVzLnN0cmluZyxcbiAgICAgIGFjdGl2ZTogUmVhY3QuUHJvcFR5cGVzLnN0cmluZ1xuICAgIH0pLCBSZWFjdC5Qcm9wVHlwZXMuc2hhcGUoe1xuICAgICAgZW50ZXI6IFJlYWN0LlByb3BUeXBlcy5zdHJpbmcsXG4gICAgICBlbnRlckFjdGl2ZTogUmVhY3QuUHJvcFR5cGVzLnN0cmluZyxcbiAgICAgIGxlYXZlOiBSZWFjdC5Qcm9wVHlwZXMuc3RyaW5nLFxuICAgICAgbGVhdmVBY3RpdmU6IFJlYWN0LlByb3BUeXBlcy5zdHJpbmcsXG4gICAgICBhcHBlYXI6IFJlYWN0LlByb3BUeXBlcy5zdHJpbmcsXG4gICAgICBhcHBlYXJBY3RpdmU6IFJlYWN0LlByb3BUeXBlcy5zdHJpbmdcbiAgICB9KV0pLmlzUmVxdWlyZWQsXG5cbiAgICAvLyBPbmNlIHdlIHJlcXVpcmUgdGltZW91dHMgdG8gYmUgc3BlY2lmaWVkLCB3ZSBjYW4gcmVtb3ZlIHRoZVxuICAgIC8vIGJvb2xlYW4gZmxhZ3MgKGFwcGVhciBldGMuKSBhbmQganVzdCBhY2NlcHQgYSBudW1iZXJcbiAgICAvLyBvciBhIGJvb2wgZm9yIHRoZSB0aW1lb3V0IGZsYWdzIChhcHBlYXJUaW1lb3V0IGV0Yy4pXG4gICAgYXBwZWFyOiBSZWFjdC5Qcm9wVHlwZXMuYm9vbCxcbiAgICBlbnRlcjogUmVhY3QuUHJvcFR5cGVzLmJvb2wsXG4gICAgbGVhdmU6IFJlYWN0LlByb3BUeXBlcy5ib29sLFxuICAgIGFwcGVhclRpbWVvdXQ6IFJlYWN0LlByb3BUeXBlcy5udW1iZXIsXG4gICAgZW50ZXJUaW1lb3V0OiBSZWFjdC5Qcm9wVHlwZXMubnVtYmVyLFxuICAgIGxlYXZlVGltZW91dDogUmVhY3QuUHJvcFR5cGVzLm51bWJlclxuICB9LFxuXG4gIHRyYW5zaXRpb246IGZ1bmN0aW9uIChhbmltYXRpb25UeXBlLCBmaW5pc2hDYWxsYmFjaywgdXNlclNwZWNpZmllZERlbGF5KSB7XG4gICAgdmFyIG5vZGUgPSBSZWFjdERPTS5maW5kRE9NTm9kZSh0aGlzKTtcblxuICAgIGlmICghbm9kZSkge1xuICAgICAgaWYgKGZpbmlzaENhbGxiYWNrKSB7XG4gICAgICAgIGZpbmlzaENhbGxiYWNrKCk7XG4gICAgICB9XG4gICAgICByZXR1cm47XG4gICAgfVxuXG4gICAgdmFyIGNsYXNzTmFtZSA9IHRoaXMucHJvcHMubmFtZVthbmltYXRpb25UeXBlXSB8fCB0aGlzLnByb3BzLm5hbWUgKyAnLScgKyBhbmltYXRpb25UeXBlO1xuICAgIHZhciBhY3RpdmVDbGFzc05hbWUgPSB0aGlzLnByb3BzLm5hbWVbYW5pbWF0aW9uVHlwZSArICdBY3RpdmUnXSB8fCBjbGFzc05hbWUgKyAnLWFjdGl2ZSc7XG4gICAgdmFyIHRpbWVvdXQgPSBudWxsO1xuXG4gICAgdmFyIGVuZExpc3RlbmVyID0gZnVuY3Rpb24gKGUpIHtcbiAgICAgIGlmIChlICYmIGUudGFyZ2V0ICE9PSBub2RlKSB7XG4gICAgICAgIHJldHVybjtcbiAgICAgIH1cblxuICAgICAgY2xlYXJUaW1lb3V0KHRpbWVvdXQpO1xuXG4gICAgICBDU1NDb3JlLnJlbW92ZUNsYXNzKG5vZGUsIGNsYXNzTmFtZSk7XG4gICAgICBDU1NDb3JlLnJlbW92ZUNsYXNzKG5vZGUsIGFjdGl2ZUNsYXNzTmFtZSk7XG5cbiAgICAgIFJlYWN0VHJhbnNpdGlvbkV2ZW50cy5yZW1vdmVFbmRFdmVudExpc3RlbmVyKG5vZGUsIGVuZExpc3RlbmVyKTtcblxuICAgICAgLy8gVXN1YWxseSB0aGlzIG9wdGlvbmFsIGNhbGxiYWNrIGlzIHVzZWQgZm9yIGluZm9ybWluZyBhbiBvd25lciBvZlxuICAgICAgLy8gYSBsZWF2ZSBhbmltYXRpb24gYW5kIHRlbGxpbmcgaXQgdG8gcmVtb3ZlIHRoZSBjaGlsZC5cbiAgICAgIGlmIChmaW5pc2hDYWxsYmFjaykge1xuICAgICAgICBmaW5pc2hDYWxsYmFjaygpO1xuICAgICAgfVxuICAgIH07XG5cbiAgICBDU1NDb3JlLmFkZENsYXNzKG5vZGUsIGNsYXNzTmFtZSk7XG5cbiAgICAvLyBOZWVkIHRvIGRvIHRoaXMgdG8gYWN0dWFsbHkgdHJpZ2dlciBhIHRyYW5zaXRpb24uXG4gICAgdGhpcy5xdWV1ZUNsYXNzKGFjdGl2ZUNsYXNzTmFtZSk7XG5cbiAgICAvLyBJZiB0aGUgdXNlciBzcGVjaWZpZWQgYSB0aW1lb3V0IGRlbGF5LlxuICAgIGlmICh1c2VyU3BlY2lmaWVkRGVsYXkpIHtcbiAgICAgIC8vIENsZWFuLXVwIHRoZSBhbmltYXRpb24gYWZ0ZXIgdGhlIHNwZWNpZmllZCBkZWxheVxuICAgICAgdGltZW91dCA9IHNldFRpbWVvdXQoZW5kTGlzdGVuZXIsIHVzZXJTcGVjaWZpZWREZWxheSk7XG4gICAgICB0aGlzLnRyYW5zaXRpb25UaW1lb3V0cy5wdXNoKHRpbWVvdXQpO1xuICAgIH0gZWxzZSB7XG4gICAgICAvLyBERVBSRUNBVEVEOiB0aGlzIGxpc3RlbmVyIHdpbGwgYmUgcmVtb3ZlZCBpbiBhIGZ1dHVyZSB2ZXJzaW9uIG9mIHJlYWN0XG4gICAgICBSZWFjdFRyYW5zaXRpb25FdmVudHMuYWRkRW5kRXZlbnRMaXN0ZW5lcihub2RlLCBlbmRMaXN0ZW5lcik7XG4gICAgfVxuICB9LFxuXG4gIHF1ZXVlQ2xhc3M6IGZ1bmN0aW9uIChjbGFzc05hbWUpIHtcbiAgICB0aGlzLmNsYXNzTmFtZVF1ZXVlLnB1c2goY2xhc3NOYW1lKTtcblxuICAgIGlmICghdGhpcy50aW1lb3V0KSB7XG4gICAgICB0aGlzLnRpbWVvdXQgPSBzZXRUaW1lb3V0KHRoaXMuZmx1c2hDbGFzc05hbWVRdWV1ZSwgVElDSyk7XG4gICAgfVxuICB9LFxuXG4gIGZsdXNoQ2xhc3NOYW1lUXVldWU6IGZ1bmN0aW9uICgpIHtcbiAgICBpZiAodGhpcy5pc01vdW50ZWQoKSkge1xuICAgICAgdGhpcy5jbGFzc05hbWVRdWV1ZS5mb3JFYWNoKENTU0NvcmUuYWRkQ2xhc3MuYmluZChDU1NDb3JlLCBSZWFjdERPTS5maW5kRE9NTm9kZSh0aGlzKSkpO1xuICAgIH1cbiAgICB0aGlzLmNsYXNzTmFtZVF1ZXVlLmxlbmd0aCA9IDA7XG4gICAgdGhpcy50aW1lb3V0ID0gbnVsbDtcbiAgfSxcblxuICBjb21wb25lbnRXaWxsTW91bnQ6IGZ1bmN0aW9uICgpIHtcbiAgICB0aGlzLmNsYXNzTmFtZVF1ZXVlID0gW107XG4gICAgdGhpcy50cmFuc2l0aW9uVGltZW91dHMgPSBbXTtcbiAgfSxcblxuICBjb21wb25lbnRXaWxsVW5tb3VudDogZnVuY3Rpb24gKCkge1xuICAgIGlmICh0aGlzLnRpbWVvdXQpIHtcbiAgICAgIGNsZWFyVGltZW91dCh0aGlzLnRpbWVvdXQpO1xuICAgIH1cbiAgICB0aGlzLnRyYW5zaXRpb25UaW1lb3V0cy5mb3JFYWNoKGZ1bmN0aW9uICh0aW1lb3V0KSB7XG4gICAgICBjbGVhclRpbWVvdXQodGltZW91dCk7XG4gICAgfSk7XG4gIH0sXG5cbiAgY29tcG9uZW50V2lsbEFwcGVhcjogZnVuY3Rpb24gKGRvbmUpIHtcbiAgICBpZiAodGhpcy5wcm9wcy5hcHBlYXIpIHtcbiAgICAgIHRoaXMudHJhbnNpdGlvbignYXBwZWFyJywgZG9uZSwgdGhpcy5wcm9wcy5hcHBlYXJUaW1lb3V0KTtcbiAgICB9IGVsc2Uge1xuICAgICAgZG9uZSgpO1xuICAgIH1cbiAgfSxcblxuICBjb21wb25lbnRXaWxsRW50ZXI6IGZ1bmN0aW9uIChkb25lKSB7XG4gICAgaWYgKHRoaXMucHJvcHMuZW50ZXIpIHtcbiAgICAgIHRoaXMudHJhbnNpdGlvbignZW50ZXInLCBkb25lLCB0aGlzLnByb3BzLmVudGVyVGltZW91dCk7XG4gICAgfSBlbHNlIHtcbiAgICAgIGRvbmUoKTtcbiAgICB9XG4gIH0sXG5cbiAgY29tcG9uZW50V2lsbExlYXZlOiBmdW5jdGlvbiAoZG9uZSkge1xuICAgIGlmICh0aGlzLnByb3BzLmxlYXZlKSB7XG4gICAgICB0aGlzLnRyYW5zaXRpb24oJ2xlYXZlJywgZG9uZSwgdGhpcy5wcm9wcy5sZWF2ZVRpbWVvdXQpO1xuICAgIH0gZWxzZSB7XG4gICAgICBkb25lKCk7XG4gICAgfVxuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24gKCkge1xuICAgIHJldHVybiBvbmx5Q2hpbGQodGhpcy5wcm9wcy5jaGlsZHJlbik7XG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IFJlYWN0Q1NTVHJhbnNpdGlvbkdyb3VwQ2hpbGQ7XG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiAuL34vcmVhY3QvbGliL1JlYWN0Q1NTVHJhbnNpdGlvbkdyb3VwQ2hpbGQuanNcbiAqKiBtb2R1bGUgaWQgPSAzODdcbiAqKiBtb2R1bGUgY2h1bmtzID0gMVxuICoqLyIsIihmdW5jdGlvbihob3N0KSB7XG5cbiAgdmFyIHByb3BlcnRpZXMgPSB7XG4gICAgYnJvd3NlcjogW1xuICAgICAgWy9tc2llIChbXFwuXFxfXFxkXSspLywgXCJpZVwiXSxcbiAgICAgIFsvdHJpZGVudFxcLy4qP3J2OihbXFwuXFxfXFxkXSspLywgXCJpZVwiXSxcbiAgICAgIFsvZmlyZWZveFxcLyhbXFwuXFxfXFxkXSspLywgXCJmaXJlZm94XCJdLFxuICAgICAgWy9jaHJvbWVcXC8oW1xcLlxcX1xcZF0rKS8sIFwiY2hyb21lXCJdLFxuICAgICAgWy92ZXJzaW9uXFwvKFtcXC5cXF9cXGRdKykuKj9zYWZhcmkvLCBcInNhZmFyaVwiXSxcbiAgICAgIFsvbW9iaWxlIHNhZmFyaSAoW1xcLlxcX1xcZF0rKS8sIFwic2FmYXJpXCJdLFxuICAgICAgWy9hbmRyb2lkLio/dmVyc2lvblxcLyhbXFwuXFxfXFxkXSspLio/c2FmYXJpLywgXCJjb20uYW5kcm9pZC5icm93c2VyXCJdLFxuICAgICAgWy9jcmlvc1xcLyhbXFwuXFxfXFxkXSspLio/c2FmYXJpLywgXCJjaHJvbWVcIl0sXG4gICAgICBbL29wZXJhLywgXCJvcGVyYVwiXSxcbiAgICAgIFsvb3BlcmFcXC8oW1xcLlxcX1xcZF0rKS8sIFwib3BlcmFcIl0sXG4gICAgICBbL29wZXJhIChbXFwuXFxfXFxkXSspLywgXCJvcGVyYVwiXSxcbiAgICAgIFsvb3BlcmEgbWluaS4qP3ZlcnNpb25cXC8oW1xcLlxcX1xcZF0rKS8sIFwib3BlcmEubWluaVwiXSxcbiAgICAgIFsvb3Bpb3NcXC8oW2EtelxcLlxcX1xcZF0rKS8sIFwib3BlcmFcIl0sXG4gICAgICBbL2JsYWNrYmVycnkvLCBcImJsYWNrYmVycnlcIl0sXG4gICAgICBbL2JsYWNrYmVycnkuKj92ZXJzaW9uXFwvKFtcXC5cXF9cXGRdKykvLCBcImJsYWNrYmVycnlcIl0sXG4gICAgICBbL2JiXFxkKy4qP3ZlcnNpb25cXC8oW1xcLlxcX1xcZF0rKS8sIFwiYmxhY2tiZXJyeVwiXSxcbiAgICAgIFsvcmltLio/dmVyc2lvblxcLyhbXFwuXFxfXFxkXSspLywgXCJibGFja2JlcnJ5XCJdLFxuICAgICAgWy9pY2V3ZWFzZWxcXC8oW1xcLlxcX1xcZF0rKS8sIFwiaWNld2Vhc2VsXCJdLFxuICAgICAgWy9lZGdlXFwvKFtcXC5cXGRdKykvLCBcImVkZ2VcIl1cbiAgICBdLFxuICAgIG9zOiBbXG4gICAgICBbL2xpbnV4ICgpKFthLXpcXC5cXF9cXGRdKykvLCBcImxpbnV4XCJdLFxuICAgICAgWy9tYWMgb3MgeC8sIFwibWFjb3NcIl0sXG4gICAgICBbL21hYyBvcyB4Lio/KFtcXC5cXF9cXGRdKykvLCBcIm1hY29zXCJdLFxuICAgICAgWy9vcyAoW1xcLlxcX1xcZF0rKSBsaWtlIG1hYyBvcy8sIFwiaW9zXCJdLFxuICAgICAgWy9vcGVuYnNkICgpKFthLXpcXC5cXF9cXGRdKykvLCBcIm9wZW5ic2RcIl0sXG4gICAgICBbL2FuZHJvaWQvLCBcImFuZHJvaWRcIl0sXG4gICAgICBbL2FuZHJvaWQgKFthLXpcXC5cXF9cXGRdKyk7LywgXCJhbmRyb2lkXCJdLFxuICAgICAgWy9tb3ppbGxhXFwvW2EtelxcLlxcX1xcZF0rIFxcKCg/Om1vYmlsZSl8KD86dGFibGV0KS8sIFwiZmlyZWZveG9zXCJdLFxuICAgICAgWy93aW5kb3dzXFxzKig/Om50KT9cXHMqKFtcXC5cXF9cXGRdKykvLCBcIndpbmRvd3NcIl0sXG4gICAgICBbL3dpbmRvd3MgcGhvbmUuKj8oW1xcLlxcX1xcZF0rKS8sIFwid2luZG93cy5waG9uZVwiXSxcbiAgICAgIFsvd2luZG93cyBtb2JpbGUvLCBcIndpbmRvd3MubW9iaWxlXCJdLFxuICAgICAgWy9ibGFja2JlcnJ5LywgXCJibGFja2JlcnJ5b3NcIl0sXG4gICAgICBbL2JiXFxkKy8sIFwiYmxhY2tiZXJyeW9zXCJdLFxuICAgICAgWy9yaW0uKj9vc1xccyooW1xcLlxcX1xcZF0rKS8sIFwiYmxhY2tiZXJyeW9zXCJdXG4gICAgXSxcbiAgICBkZXZpY2U6IFtcbiAgICAgIFsvaXBhZC8sIFwiaXBhZFwiXSxcbiAgICAgIFsvaXBob25lLywgXCJpcGhvbmVcIl0sXG4gICAgICBbL2x1bWlhLywgXCJsdW1pYVwiXSxcbiAgICAgIFsvaHRjLywgXCJodGNcIl0sXG4gICAgICBbL25leHVzLywgXCJuZXh1c1wiXSxcbiAgICAgIFsvZ2FsYXh5IG5leHVzLywgXCJnYWxheHkubmV4dXNcIl0sXG4gICAgICBbL25va2lhLywgXCJub2tpYVwiXSxcbiAgICAgIFsvIGd0XFwtLywgXCJnYWxheHlcIl0sXG4gICAgICBbLyBzbVxcLS8sIFwiZ2FsYXh5XCJdLFxuICAgICAgWy94Ym94LywgXCJ4Ym94XCJdLFxuICAgICAgWy8oPzpiYlxcZCspfCg/OmJsYWNrYmVycnkpfCg/OiByaW0gKS8sIFwiYmxhY2tiZXJyeVwiXVxuICAgIF1cbiAgfTtcblxuICB2YXIgVU5LTk9XTiA9IFwiVW5rbm93blwiO1xuXG4gIHZhciBwcm9wZXJ0eU5hbWVzID0gT2JqZWN0LmtleXMocHJvcGVydGllcyk7XG5cbiAgZnVuY3Rpb24gU25pZmZyKCkge1xuICAgIHZhciBzZWxmID0gdGhpcztcblxuICAgIHByb3BlcnR5TmFtZXMuZm9yRWFjaChmdW5jdGlvbihwcm9wZXJ0eU5hbWUpIHtcbiAgICAgIHNlbGZbcHJvcGVydHlOYW1lXSA9IHtcbiAgICAgICAgbmFtZTogVU5LTk9XTixcbiAgICAgICAgdmVyc2lvbjogW10sXG4gICAgICAgIHZlcnNpb25TdHJpbmc6IFVOS05PV05cbiAgICAgIH07XG4gICAgfSk7XG4gIH1cblxuICBmdW5jdGlvbiBkZXRlcm1pbmVQcm9wZXJ0eShzZWxmLCBwcm9wZXJ0eU5hbWUsIHVzZXJBZ2VudCkge1xuICAgIHByb3BlcnRpZXNbcHJvcGVydHlOYW1lXS5mb3JFYWNoKGZ1bmN0aW9uKHByb3BlcnR5TWF0Y2hlcikge1xuICAgICAgdmFyIHByb3BlcnR5UmVnZXggPSBwcm9wZXJ0eU1hdGNoZXJbMF07XG4gICAgICB2YXIgcHJvcGVydHlWYWx1ZSA9IHByb3BlcnR5TWF0Y2hlclsxXTtcblxuICAgICAgdmFyIG1hdGNoID0gdXNlckFnZW50Lm1hdGNoKHByb3BlcnR5UmVnZXgpO1xuXG4gICAgICBpZiAobWF0Y2gpIHtcbiAgICAgICAgc2VsZltwcm9wZXJ0eU5hbWVdLm5hbWUgPSBwcm9wZXJ0eVZhbHVlO1xuXG4gICAgICAgIGlmIChtYXRjaFsyXSkge1xuICAgICAgICAgIHNlbGZbcHJvcGVydHlOYW1lXS52ZXJzaW9uU3RyaW5nID0gbWF0Y2hbMl07XG4gICAgICAgICAgc2VsZltwcm9wZXJ0eU5hbWVdLnZlcnNpb24gPSBbXTtcbiAgICAgICAgfSBlbHNlIGlmIChtYXRjaFsxXSkge1xuICAgICAgICAgIHNlbGZbcHJvcGVydHlOYW1lXS52ZXJzaW9uU3RyaW5nID0gbWF0Y2hbMV0ucmVwbGFjZSgvXy9nLCBcIi5cIik7XG4gICAgICAgICAgc2VsZltwcm9wZXJ0eU5hbWVdLnZlcnNpb24gPSBwYXJzZVZlcnNpb24obWF0Y2hbMV0pO1xuICAgICAgICB9IGVsc2Uge1xuICAgICAgICAgIHNlbGZbcHJvcGVydHlOYW1lXS52ZXJzaW9uU3RyaW5nID0gVU5LTk9XTjtcbiAgICAgICAgICBzZWxmW3Byb3BlcnR5TmFtZV0udmVyc2lvbiA9IFtdO1xuICAgICAgICB9XG4gICAgICB9XG4gICAgfSk7XG4gIH1cblxuICBmdW5jdGlvbiBwYXJzZVZlcnNpb24odmVyc2lvblN0cmluZykge1xuICAgIHJldHVybiB2ZXJzaW9uU3RyaW5nLnNwbGl0KC9bXFwuX10vKS5tYXAoZnVuY3Rpb24odmVyc2lvblBhcnQpIHtcbiAgICAgIHJldHVybiBwYXJzZUludCh2ZXJzaW9uUGFydCk7XG4gICAgfSk7XG4gIH1cblxuICBTbmlmZnIucHJvdG90eXBlLnNuaWZmID0gZnVuY3Rpb24odXNlckFnZW50U3RyaW5nKSB7XG4gICAgdmFyIHNlbGYgPSB0aGlzO1xuICAgIHZhciB1c2VyQWdlbnQgPSAodXNlckFnZW50U3RyaW5nIHx8IG5hdmlnYXRvci51c2VyQWdlbnQgfHwgXCJcIikudG9Mb3dlckNhc2UoKTtcblxuICAgIHByb3BlcnR5TmFtZXMuZm9yRWFjaChmdW5jdGlvbihwcm9wZXJ0eU5hbWUpIHtcbiAgICAgIGRldGVybWluZVByb3BlcnR5KHNlbGYsIHByb3BlcnR5TmFtZSwgdXNlckFnZW50KTtcbiAgICB9KTtcbiAgfTtcblxuXG4gIGlmICh0eXBlb2YgbW9kdWxlICE9PSAndW5kZWZpbmVkJyAmJiBtb2R1bGUuZXhwb3J0cykge1xuICAgIG1vZHVsZS5leHBvcnRzID0gU25pZmZyO1xuICB9IGVsc2Uge1xuICAgIGhvc3QuU25pZmZyID0gbmV3IFNuaWZmcigpO1xuICAgIGhvc3QuU25pZmZyLnNuaWZmKG5hdmlnYXRvci51c2VyQWdlbnQpO1xuICB9XG59KSh0aGlzKTtcblxuXG5cbi8qKioqKioqKioqKioqKioqKlxuICoqIFdFQlBBQ0sgRk9PVEVSXG4gKiogLi9+L3NuaWZmci9zcmMvc25pZmZyLmpzXG4gKiogbW9kdWxlIGlkID0gNDM0XG4gKiogbW9kdWxlIGNodW5rcyA9IDFcbiAqKi8iLCI7XG52YXIgc3ByaXRlID0gcmVxdWlyZShcIi9ob21lL2Frb250c2V2b3kvZ28vc3JjL2dpdGh1Yi5jb20vZ3Jhdml0YXRpb25hbC90ZWxlcG9ydC93ZWIvbm9kZV9tb2R1bGVzL3N2Zy1zcHJpdGUtbG9hZGVyL2xpYi93ZWIvZ2xvYmFsLXNwcml0ZVwiKTs7XG52YXIgaW1hZ2UgPSBcIjxzeW1ib2wgdmlld0JveD1cXFwiMCAwIDM0MCAxMDBcXFwiIGlkPVxcXCJncnYtdGxwdC1sb2dvLWZ1bGxcXFwiIHhtbG5zOnhsaW5rPVxcXCJodHRwOi8vd3d3LnczLm9yZy8xOTk5L3hsaW5rXFxcIj4gPGc+IDxnIGlkPVxcXCJncnYtdGxwdC1sb2dvLWZ1bGxfTGF5ZXJfMlxcXCI+IDxnPiA8Zz4gPHBhdGggZD1cXFwibTQ3LjY3MTAwMSwyMS40NDRjLTcuMzk2LDAgLTE0LjEwMjAwMSwzLjAwNzk5OSAtMTguOTYwMDAzLDcuODY2MDAxYy00Ljg1Njk5OCw0Ljg1Njk5OCAtNy44NjU5OTksMTEuNTYzIC03Ljg2NTk5OSwxOC45NTk5OTljMCw3LjM5NiAzLjAwODAwMSwxNC4xMDEwMDIgNy44NjU5OTksMTguOTU3OTk2czExLjU2NDAwMyw3Ljg2NTAwNSAxOC45NjAwMDMsNy44NjUwMDVzMTQuMTAyMDAxLC0zLjAwODAwMyAxOC45NTg5OTYsLTcuODY1MDA1czcuODY1MDA1LC0xMS41NjE5OTYgNy44NjUwMDUsLTE4Ljk1Nzk5NnMtMy4wMDgwMDMsLTE0LjEwNCAtNy44NjUwMDUsLTE4Ljk1OTk5OWMtNC44NTc5OTQsLTQuODU4MDAyIC0xMS41NjI5OTYsLTcuODY2MDAxIC0xOC45NTg5OTYsLTcuODY2MDAxem0xMS4zODY5OTcsMTkuNTA5OTk4aC04LjIxMzk5N3YyMy4xODAwMDRoLTYuMzQ0MDAydi0yMy4xODAwMDRoLTguMjE1di01LjYxMmgyMi43NzI5OTl2NS42MTJsMCwwelxcXCIvPiA8L2c+IDxnPiA8cGF0aCBkPVxcXCJtOTIuNzgyOTk3LDYzLjM1NzAwMmMtMC4wOTg5OTksLTAuMzcxMDAyIC0wLjMyMDk5OSwtMC43MDkgLTAuNjQ2OTk2LC0wLjk0MjAwMWwtNC41NjIwMDQsLTMuOTU4bC00LjU2MTk5NiwtMy45NTcwMDFjMC4xNjMwMDIsLTAuODg3MDAxIDAuMjY3OTk4LC0xLjgwNSAwLjMzMTAwMSwtMi43MzZjMC4wNjM5OTUsLTAuOTMxIDAuMDg2OTk4LC0xLjg3NDAwMSAwLjA4Njk5OCwtMi44MDVjMCwtMC45MzI5OTkgLTAuMDIyMDAzLC0xLjg3NSAtMC4wODY5OTgsLTIuODA2OTk5Yy0wLjA2MzAwNCwtMC45MzE5OTkgLTAuMTY3OTk5LC0xLjg1MTAwMiAtMC4zMzEwMDEsLTIuNzM2bDQuNTYxOTk2LC0zLjk1NzAwMWw0LjU2MjAwNCwtMy45NThjMC4zMjU5OTYsLTAuMjMyOTk4IDAuNTQ4OTk2LC0wLjU3IDAuNjQ2OTk2LC0wLjk0MjAwMWMwLjA5OTAwNywtMC4zNzI5OTcgMC4wNzUwMDUsLTAuNzc4OTk5IC0wLjA4Nzk5NywtMS4xNTNjLTAuOTMxOTk5LC0yLjg2MiAtMi4xOTk5OTcsLTUuNjU1OTk4IC0zLjczMTAwMywtOC4yOTljLTEuNTMwOTk4LC0yLjY0MTk5OCAtMy4zMjE5OTksLTUuMTMyOTk4IC01LjMwMTk5NCwtNy4zOTA5OTljLTAuMjc4OTk5LC0wLjMyNiAtMC42MTcwMDQsLTAuNTQ4IC0wLjk3ODAwNCwtMC42NDZjLTAuMzYwMDAxLC0wLjA5ODk5OSAtMC43NDQ5OTUsLTAuMDc0OTk5IC0xLjExNjk5NywwLjA4N2wtNS43NTA5OTksMi4wMDIwMDFsLTUuNzQ5MDAxLDIuMDAwOTk5Yy0xLjQxOTk5OCwtMS4xNjQgLTIuOTMzOTk4LC0yLjIxMSAtNC41MjIwMDMsLTMuMTM2OTk5Yy0xLjU4OTk5NiwtMC45MjUwMDEgLTMuMjUzOTk4LC0xLjcyODAwMSAtNC45Nzc5OTcsLTIuNDA0MDAxbC0xLjEzOTk5OSwtNS45NTlsLTEuMTQwOTk5LC01Ljk1OWMtMC4wNjksLTAuMzczIC0wLjI2ODAwNSwtMC43MzMgLTAuNTQ3MDA1LC0xLjAxM2MtMC4yNzg5OTksLTAuMjggLTAuNjQwOTk5LC0wLjQ3OCAtMS4wMzY5OTUsLTAuNTI0Yy0yLjk4MDAwMywtMC42MDUgLTYuMDA3MDA0LC0wLjkwOCAtOS4wMzMwMDUsLTAuOTA4cy02LjA1Mjk5OCwwLjMwMiAtOS4wMzI5OTcsMC45MDhjLTAuMzk2LDAuMDQ2IC0wLjc1NjAwMSwwLjI0NTAwMSAtMS4wMzYwMDMsMC41MjRjLTAuMjc4OTk5LDAuMjc5IC0wLjQ3Nzk5NywwLjY0IC0wLjU0Njk5NywxLjAxM2wtMS4xNDEwMDMsNS45NTlsLTEuMTQwOTk5LDUuOTYwMDAxYy0xLjcyMywwLjY3NTk5OSAtMy40MTA5OTksMS40NzkgLTUuMDEyMDAxLDIuNDAzOTk5Yy0xLjU5OTk5OCwwLjkyNDk5OSAtMy4xMTI5OTksMS45NzMgLTQuNDg3LDMuMTM2OTk5bC01Ljc1LC0yLjAwMDk5OWwtNS43NSwtMi4wMDE5OTljLTAuMzcyLC0wLjE2NDAwMSAtMC43NTU5OTksLTAuMTg3IC0xLjExNjk5OSwtMC4wODgwMDFjLTAuMzYxLDAuMSAtMC42OTkwMDEsMC4zMiAtMC45NzgwMDEsMC42NDZjLTEuOTc5LDIuMjU5MDAxIC0zLjc3MSw0Ljc1IC01LjMwMiw3LjM5MjAwMmMtMS41MywyLjY0MTk5OCAtMi43OTksNS40MzY5OTYgLTMuNzMsOC4yOTljLTAuMTYzLDAuMzcyOTk3IC0wLjE4NywwLjc4MDk5OCAtMC4wODcwMDEsMS4xNTE5OTdjMC4wOTksMC4zNzIwMDIgMC4zMjAwMDEsMC43MTAwMDMgMC42NDYwMDEsMC45NDMwMDFsNC41NjMsMy45NTcwMDFsNC41NjIsMy45NThjLTAuMTYzLDAuODg0OTk4IC0wLjI2OCwxLjgwNDAwMSAtMC4zMzEwMDEsMi43MzUwMDFjLTAuMDYzOTk5LDAuOTMxOTk5IC0wLjA4Nzk5OSwxLjg3NSAtMC4wODc5OTksMi44MDZzMC4wMjMwMDEsMS44NzUgMC4wODcsMi44MDZjMC4wNjQwMDEsMC45MzE5OTkgMC4xNjgwMDEsMS44NTEwMDIgMC4zMzIwMDEsMi43MzUwMDFsLTQuNTYyLDMuOTU3MDAxbC00LjU2MiwzLjk1OWMtMC4zMjUsMC4yMzEwMDMgLTAuNTQ3LDAuNTY5IC0wLjY0NiwwLjk0MjAwMWMtMC4wOTksMC4zNzA5OTUgLTAuMDc2LDAuNzc4OTk5IDAuMDg3LDEuMTUwMDAyYzAuOTMxLDIuODY0OTk4IDIuMiw1LjY1Nzk5NyAzLjczLDguMzAwOTk1YzEuNTMxLDIuNjQyOTk4IDMuMzIzLDUuMTMzMDAzIDUuMzAyLDcuMzkxOTk4YzAuMjgwMDAxLDAuMzI1MDA1IDAuNjE4LDAuNTQ4MDA0IDAuOTc4MDAxLDAuNjQ2MDA0YzAuMzYxLDAuMDk5OTk4IDAuNzQ0OTk5LDAuMDc0OTk3IDEuMTE4LC0wLjA4Nzk5N2w1Ljc1LC0yLjAwMzAwNmw1Ljc0OTk5OCwtMi4wMDA5OTljMS4zNzMwMDEsMS4xNjQwMDEgMi44ODYwMDIsMi4yMTMwMDUgNC40ODcwMDMsMy4xMzljMS42MDA5OTgsMC45MjQwMDQgMy4yODg5OTgsMS43MjgwMDQgNS4wMTA5OTgsMi40MDEwMDFsMS4xNDA5OTksNS45NjE5OThsMS4xNDEwMDMsNS45NTljMC4wNywwLjM3MjAwMiAwLjI2Nzk5OCwwLjczMzAwMiAwLjU0NzAwMSwxLjAxNGMwLjI3ODk5OSwwLjI3OTAwNyAwLjY0MDk5OSwwLjQ3OTAwNCAxLjAzNTk5OSwwLjUyMjAwM2MxLjQ4OTk5OCwwLjI3OCAyLjk3OSwwLjUwMDk5OSA0LjQ4MDk5OSwwLjY1MTAwMWMxLjUwMDk5OSwwLjE1MiAzLjAxNDk5OSwwLjIzMjAwMiA0LjU1MTk5OCwwLjIzMjAwMnMzLjA0OTAwNCwtMC4wODAwMDIgNC41NTEwMDMsLTAuMjMyMDAyYzEuNTAxOTk5LC0wLjE1MDAwMiAyLjk5MDk5NywtMC4zNzMwMDEgNC40Nzk5OTYsLTAuNjUxMDAxYzAuMzk2MDA0LC0wLjA0NDk5OCAwLjc1NzAwNCwtMC4yNDM5OTYgMS4wMzcwMDMsLTAuNTIyMDAzYzAuMjc5OTk5LC0wLjI3ODk5OSAwLjQ3Njk5NywtMC42NDE5OTggMC41NDcwMDUsLTEuMDE0bDEuMTQwOTk5LC01Ljk1OWwxLjE0MDk5OSwtNS45NjE5OThjMS43MjMsLTAuNjc0OTk1IDMuMzg3MDAxLC0xLjQ3Nzk5NyA0Ljk3Njk5NywtMi40MDEwMDFjMS41ODgwMDUsLTAuOTI1OTk1IDMuMTAzMDA0LC0xLjk3NDk5OCA0LjUyMjAwMywtMy4xMzlsNS43NSwyLjAwMDk5OWw1Ljc1LDIuMDAzMDA2YzAuMzczMDAxLDAuMTYyOTk0IDAuNzU2OTk2LDAuMTg1OTk3IDEuMTE3OTk2LDAuMDg3OTk3YzAuMzYwMDAxLC0wLjA5ODk5OSAwLjY5ODAwNiwtMC4zMiAwLjk3ODAwNCwtMC42NDYwMDRjMS45Nzg5OTYsLTIuMjU4OTk1IDMuNzcwOTk2LC00Ljc0OTAwMSA1LjMwMTk5NCwtNy4zOTE5OThjMS41MzEwMDYsLTIuNjQyOTk4IDIuODAwMDAzLC01LjQzNjk5NiAzLjczMTAwMywtOC4zMDA5OTVjMC4xNjQwMDEsLTAuMzY4MDA0IDAuMTg4MDA0LC0wLjc3ODAwOCAwLjA4Nzk5NywtMS4xNTAwMDJ6bS0yNC4yMzc5OTksNS43ODc5OTRjLTUuMzQ4LDUuMzQ5MDA3IC0xMi43MzE5OTUsOC42NjAwMDQgLTIwLjg3NSw4LjY2MDAwNGMtOC4xNDM5OTcsMCAtMTUuNTI2OTk3LC0zLjMxMjAwNCAtMjAuODc1LC04LjY2MDAwNHMtOC42NTk5OTgsLTEyLjczMDk5NSAtOC42NTk5OTgsLTIwLjg3NDk5NmMwLC04LjE0NDAwMSAzLjMxMiwtMTUuNTI3IDguNjYxMDAxLC0yMC44NzU5OTljNS4zNDgsLTUuMzQ4MDAxIDEyLjczMTk5OCwtOC42NjEwMDEgMjAuODc1OTk5LC04LjY2MTAwMWM4LjE0MzAwMiwwIDE1LjUyNTk5NywzLjMxMiAyMC44NzQ5OTYsOC42NjEwMDFjNS4zNDgsNS4zNDg5OTkgOC42NjEwMDMsMTIuNzMxOTk4IDguNjYxMDAzLDIwLjg3NTk5OWMtMC4wMDA5OTksOC4xNDE5OTggLTMuMzE0MDAzLDE1LjUyNTk5NyAtOC42NjMwMDIsMjAuODc0OTk2elxcXCIvPiA8L2c+IDwvZz4gPC9nPiA8Zz4gPHBhdGggZD1cXFwibTExOS43NzMwMDMsMzAuODYxaC0xMy4wMjAwMDR2LTYuODQxaDMzLjU5OTk5OHY2Ljg0MWgtMTMuMDIwMDA0djM1LjYzOTk5OWgtNy41NTk5OXYtMzUuNjM5OTk5bDAsMHpcXFwiLz4gPHBhdGggZD1cXFwibTE0My45NTMwMDMsNTQuNjIwOTk4YzAuMjM5OTksMi4xNiAxLjA4MDAwMiwzLjg0IDIuNTIwMDA0LDUuMDM5OTk3czMuMTc5OTkzLDEuODAwMDAzIDUuMjE5OTg2LDEuODAwMDAzYzEuODAwMDAzLDAgMy4zMDkwMDYsLTAuMzY4OTk2IDQuNTMwMDE0LC0xLjExMDAwMWMxLjIxOTk4NiwtMC43Mzg5OTggMi4yODk5OTMsLTEuNjY4OTk5IDMuMjA5OTkxLC0yLjc5MDAwMWw1LjE2MDAwNCwzLjkwMDAwMmMtMS42ODAwMDgsMi4wODAwMDIgLTMuNTYxMDA1LDMuNTYxMDA1IC01LjYzOTk5OSw0LjQ0MDAwMmMtMi4wODAwMDIsMC44Nzg5OTggLTQuMjYwMDEsMS4zMTkgLTYuNTQwMDA5LDEuMzE5Yy0yLjE1OTk4OCwwIC00LjE5OTk5NywtMC4zNTkwMDEgLTYuMTE5OTk1LC0xLjA4MDAwMmMtMS45MTk5OTgsLTAuNzIwMDAxIC0zLjU4MDk5NCwtMS43Mzg5OTggLTQuOTc5OTk2LC0zLjA1OTk5OGMtMS40MDEwMDEsLTEuMzIwMDA3IC0yLjUxMTAwMiwtMi45MTAwMDQgLTMuMzMwMDAyLC00Ljc3MTAwNGMtMC44MjAwMDcsLTEuODU4OTk3IC0xLjIyOTk5NiwtMy45Mjk5OTYgLTEuMjI5OTk2LC02LjIwOTk5OWMwLC0yLjI3ODk5OSAwLjQwOTk4OCwtNC4zNDk5OTggMS4yMjk5OTYsLTYuMjA5OTk5YzAuODE5LC0xLjg1OTAwMSAxLjkyOTAwMSwtMy40NDkwMDEgMy4zMzAwMDIsLTQuNzdjMS4zOTkwMDIsLTEuMzIgMy4wNTk5OTgsLTIuMzQgNC45Nzk5OTYsLTMuMDYxMDAxYzEuOTE5OTk4LC0wLjcxOTk5NyAzLjk2MDAwNywtMS4wNzg5OTkgNi4xMTk5OTUsLTEuMDc4OTk5YzIsMCAzLjgzMDAwMiwwLjM1MTAwMiA1LjQ5MDAwNSwxLjA0OTk5OWMxLjY1ODk5NywwLjcwMDAwMSAzLjA4MDAwMiwxLjcwOTk5OSA0LjI1OTk5NSwzLjAyODk5OWMxLjE4MDAwOCwxLjMyIDIuMTAwMDA2LDIuOTUxIDIuNzYwMDEsNC44OTEwMDNjMC42NTk5ODgsMS45Mzk5OTkgMC45ODk5OSw0LjE2OTk5OCAwLjk4OTk5LDYuNjg4OTk5djEuOThoLTIxLjk1OTk5MWwwLDAuMDAyOTk4em0xNC43NTk5OTUsLTUuMzk5OTk4Yy0wLjA0MSwtMi4xMTg5OTkgLTAuNjk5OTk3LC0zLjc4OTAwMSAtMS45Nzk5OTYsLTUuMDEwMDAyYy0xLjI4MTAwNiwtMS4yMTk5OTcgLTMuMDU5OTk4LC0xLjgyOTk5OCAtNS4zMzk5OTYsLTEuODI5OTk4Yy0yLjE2MDAwNCwwIC0zLjg3MDAxLDAuNjIwOTk4IC01LjEzMDAwNSwxLjg2MDAwMWMtMS4yNTk5OTUsMS4yMzk5OTggLTIuMDMxMDA2LDIuODk5OTk4IC0yLjMwOTk5OCw0Ljk3OWgxNC43NTk5OTVsMCwwLjAwMDk5OXpcXFwiLz4gPHBhdGggZD1cXFwibTE3Mi43NTMwMDYsMjEuMTQxMDAxaDcuMTk5OTk3djQ1LjM1OTk5OWgtNy4xOTk5OTd2LTQ1LjM1OTk5OWwwLDB6XFxcIi8+IDxwYXRoIGQ9XFxcIm0xOTMuOTkyMDA0LDU0LjYyMDk5OGMwLjIzOTk5LDIuMTYgMS4wODAwMDIsMy44NCAyLjUxOTk4OSw1LjAzOTk5N2MxLjQ0MDAwMiwxLjIwMDAwNSAzLjE4MSwxLjgwMDAwMyA1LjIyMTAwOCwxLjgwMDAwM2MxLjgwMDAwMywwIDMuMzA5MDA2LC0wLjM2ODk5NiA0LjUyODk5MiwtMS4xMTAwMDFjMS4yMjEwMDgsLTAuNzM4OTk4IDIuMjkwMDA5LC0xLjY2ODk5OSAzLjIxMTAxNCwtMi43OTAwMDFsNS4xNTk5ODgsMy45MDAwMDJjLTEuNjgxLDIuMDgwMDAyIC0zLjU2MDk4OSwzLjU2MTAwNSAtNS42NDA5OTEsNC40NDAwMDJjLTIuMDgwMDAyLDAuODc4OTk4IC00LjI2MDAxLDEuMzE5IC02LjU0MDAwOSwxLjMxOWMtMi4xNTg5OTcsMCAtNC4xOTk5OTcsLTAuMzU5MDAxIC02LjExOTk5NSwtMS4wODAwMDJjLTEuOTE5OTk4LC0wLjcyMDAwMSAtMy41ODAwMDIsLTEuNzM4OTk4IC00Ljk3OTAwNCwtMy4wNTk5OThjLTEuNDAxMDAxLC0xLjMyMDAwNyAtMi41MTEwMDIsLTIuOTEwMDA0IC0zLjMzMDAwMiwtNC43NzEwMDRjLTAuODE5OTkyLC0xLjg1ODk5NyAtMS4yMjg5ODksLTMuOTI5OTk2IC0xLjIyODk4OSwtNi4yMDk5OTljMCwtMi4yNzg5OTkgMC40MDg5OTcsLTQuMzQ5OTk4IDEuMjI4OTg5LC02LjIwOTk5OWMwLjgxOSwtMS44NTkwMDEgMS45MjkwMDEsLTMuNDQ5MDAxIDMuMzMwMDAyLC00Ljc3YzEuMzk5MDAyLC0xLjMyIDMuMDU5OTk4LC0yLjM0IDQuOTc5MDA0LC0zLjA2MTAwMWMxLjkxOTk5OCwtMC43MTk5OTcgMy45NjA5OTksLTEuMDc4OTk5IDYuMTE5OTk1LC0xLjA3ODk5OWMyLDAgMy44MzAwMDIsMC4zNTEwMDIgNS40OTAwMDUsMS4wNDk5OTljMS42NTg5OTcsMC43MDAwMDEgMy4wNzg5OTUsMS43MDk5OTkgNC4yNTk5OTUsMy4wMjg5OTljMS4xODAwMDgsMS4zMiAyLjEwMDk5OCwyLjk1MSAyLjc2MTAwMiw0Ljg5MTAwM2MwLjY2MDAwNCwxLjkzOTk5OSAwLjk4ODk5OCw0LjE2OTk5OCAwLjk4ODk5OCw2LjY4ODk5OXYxLjk4aC0yMS45NTk5OTFsMCwwLjAwMjk5OHptMTQuNzU5OTk1LC01LjM5OTk5OGMtMC4wMzk5OTMsLTIuMTE4OTk5IC0wLjY5OTAwNSwtMy43ODkwMDEgLTEuOTc5MDA0LC01LjAxMDAwMmMtMS4yNzk5OTksLTEuMjE5OTk3IC0zLjA1OTk5OCwtMS44Mjk5OTggLTUuMzQwOTg4LC0xLjgyOTk5OGMtMi4xNTkwMTIsMCAtMy44NjkwMDMsMC42MjA5OTggLTUuMTI5MDEzLDEuODYwMDAxYy0xLjI1OTk5NSwxLjIzOTk5OCAtMi4wMzA5OTEsMi44OTk5OTggLTIuMzEwOTg5LDQuOTc5aDE0Ljc1OTk5NWwwLDAuMDAwOTk5elxcXCIvPiA8cGF0aCBkPVxcXCJtMjIyLjY3MTk5NywzNy43MDFoNi44Mzk5OTZ2NC4zMTloMC4xMjAwMWMxLjAzOTk5MywtMS43NTg5OTkgMi40Mzg5OTUsLTMuMDM5MDAxIDQuMTk5OTk3LC0zLjg0YzEuNzU5OTk1LC0wLjc5OTk5OSAzLjY2MDAwNCwtMS4xOTkwMDEgNS42OTkwMDUsLTEuMTk5MDAxYzIuMTk4OTksMCA0LjE3OTk5MywwLjM4OTk5OSA1LjkzOTk4NywxLjE3MDAwMmMxLjc2MDAxLDAuNzc4OTk5IDMuMjYwMDI1LDEuODUwOTk4IDQuNTAwMDE1LDMuMjA5OTk5YzEuMjM5MDE0LDEuMzYwMDAxIDIuMTc5OTkzLDIuOTU5OTk5IDIuODIwMDA3LDQuNzk5OTk5YzAuNjM5OTg0LDEuODQgMC45NTk5OTEsMy44MiAwLjk1OTk5MSw1LjkzODk5OWMwLDIuMTIxMDAyIC0wLjMzOTk5Niw0LjEwMTAwMiAtMS4wMTk5ODksNS45NDAwMDJjLTAuNjgyMDA3LDEuODQwMDA0IC0xLjYzMTAxMiwzLjQ0MDAwMiAtMi44NTEwMTMsNC44MDAwMDNjLTEuMjIxMDA4LDEuMzU5OTkzIC0yLjY5MDAwMiwyLjQzIC00LjQxMDAwNCwzLjIwOTk5OXMtMy42MDA5OTgsMS4xNjk5OTggLTUuNjM5OTk5LDEuMTY5OTk4Yy0xLjM2MDAwMSwwIC0yLjU2MTAwNSwtMC4xNDA5OTkgLTMuNjAwMDA2LC0wLjQyMDAwNmMtMS4wNDEsLTAuMjc5OTkxIC0xLjk2MDk5OSwtMC42Mzk5OTIgLTIuNzYxMDAyLC0xLjA3OTk5NGMtMC43OTk5ODgsLTAuNDM5MDAzIC0xLjQ3ODk4OSwtMC45MDkwMDQgLTIuMDM5OTkzLC0xLjQxMDAwNGMtMC41NjEwMDUsLTAuNDk5MDAxIC0xLjAyMDAwNCwtMC45ODg5OTggLTEuMzgwMDA1LC0xLjQ2OTk5NGgtMC4xODF2MTcuMzM5OTk2aC03LjE5ODk5di00Mi40NzlsMC4wMDI5OTEsMHptMjMuODgwMDA1LDE0LjQwMDAwMmMwLC0xLjExOTAwMyAtMC4xOTAwMDIsLTIuMTk5MDAxIC0wLjU2OSwtMy4yMzkwMDJjLTAuMzgwOTk3LC0xLjA0MDAwMSAtMC45NDA5OTQsLTEuOTU5OTk5IC0xLjY4MSwtMi43NjA5OThjLTAuNzQwOTk3LC0wLjc5OTAwNCAtMS42MzAwMDUsLTEuNDM5MDAzIC0yLjY2OTk5OCwtMS45MjAwMDJjLTEuMDQwMDA5LC0wLjQ3OSAtMi4yMjAwMDEsLTAuNzIwMDAxIC0zLjU0MDAwOSwtMC43MjAwMDFzLTIuNSwwLjI0MDAwMiAtMy41Mzk5OTMsMC43MjAwMDFjLTEuMDQwMDA5LDAuNDggLTEuOTMxLDEuMTIwOTk4IC0yLjY2OTk5OCwxLjkyMDAwMmMtMC43NDA5OTcsMC44MDA5OTkgLTEuMzAwMDAzLDEuNzIwOTk3IC0xLjY4MSwyLjc2MDk5OGMtMC4zODAwMDUsMS4wNDAwMDEgLTAuNTY5LDIuMTE5OTk5IC0wLjU2OSwzLjIzOTAwMmMwLDEuMTIwOTk4IDAuMTg4OTk1LDIuMjAwOTk2IDAuNTY5LDMuMjM5OTk4YzAuMzgwOTk3LDEuMDQxIDAuOTM4OTk1LDEuOTYwOTk1IDEuNjgxLDIuNzU5OTk4YzAuNzM4OTk4LDAuODAxMDAzIDEuNjI5OTksMS40NDAwMDIgMi42Njk5OTgsMS45MTk5OThjMS4wMzk5OTMsMC40ODAwMDMgMi4yMjAwMDEsMC43MjEwMDEgMy41Mzk5OTMsMC43MjEwMDFzMi41LC0wLjIzOTk5OCAzLjU0MDAwOSwtMC43MjEwMDFjMS4wMzk5OTMsLTAuNDc4OTk2IDEuOTI5MDAxLC0xLjExODk5NiAyLjY2OTk5OCwtMS45MTk5OThjMC43Mzg5OTgsLTAuNzk5MDA0IDEuMzAwMDAzLC0xLjcxODk5OCAxLjY4MSwtMi43NTk5OThjMC4zNzc5OTEsLTEuMDM5MDAxIDAuNTY5LC0yLjExODk5OSAwLjU2OSwtMy4yMzk5OTh6XFxcIi8+IDxwYXRoIGQ9XFxcIm0yNTkuMDMxMDA2LDUyLjEwMTAwMmMwLC0yLjI3OTAwMyAwLjQxMDAwNCwtNC4zNTAwMDIgMS4yMzAwMTEsLTYuMjEwMDAzYzAuODE3OTkzLC0xLjg1ODk5NyAxLjkyODk4NiwtMy40NDg5OTcgMy4zMjk5ODcsLTQuNzdjMS4zOTgwMSwtMS4zMiAzLjA1OTAyMSwtMi4zNCA0Ljk3OTAwNCwtMy4wNjA5OTdjMS45MjAwMTMsLTAuNzIwMDAxIDMuOTU5OTkxLC0xLjA3OTAwMiA2LjExOTk5NSwtMS4wNzkwMDJzNC4xOTkwMDUsMC4zNTkwMDEgNi4xMTkwMTksMS4wNzkwMDJjMS45MTk5ODMsMC43MjA5OTcgMy41Nzk5ODcsMS43Mzk5OTggNC45Nzk5OCwzLjA2MDk5N3MyLjUxMDAxLDIuOTEgMy4zMzAwMTcsNC43N2MwLjgxOTk3NywxLjg2MDAwMSAxLjIyOTk4LDMuOTMxIDEuMjI5OTgsNi4yMTAwMDNjMCwyLjI3OTk5OSAtMC40MTAwMDQsNC4zNTA5OTggLTEuMjI5OTgsNi4yMTAwMDNjLTAuODIwMDA3LDEuODYwMDAxIC0xLjkzMDAyMywzLjQ0OTk5NyAtMy4zMzAwMTcsNC43NzA5OTZzLTMuMDYxMDA1LDIuMzQwMDA0IC00Ljk3OTk4LDMuMDU5OTk4Yy0xLjkyMDAxMywwLjcyMTAwMSAtMy45NTkwMTUsMS4wODAwMDIgLTYuMTE5MDE5LDEuMDgwMDAycy00LjE5OTk4MiwtMC4zNTkwMDEgLTYuMTE5OTk1LC0xLjA4MDAwMmMtMS45MjA5OSwtMC43MTk5OTQgLTMuNTgwOTk0LC0xLjczODk5OCAtNC45NzkwMDQsLTMuMDU5OTk4Yy0xLjQwMTAwMSwtMS4zMiAtMi41MTE5OTMsLTIuOTA5OTk2IC0zLjMyOTk4NywtNC43NzA5OTZjLTAuODIwMDA3LC0xLjg2MDAwNCAtMS4yMzAwMTEsLTMuOTMwMDA0IC0xLjIzMDAxMSwtNi4yMTAwMDN6bTcuMTk5MDA1LDBjMCwxLjEyMDk5OCAwLjE4ODk5NSwyLjIwMDk5NiAwLjU3MDAwNywzLjIzOTk5OGMwLjM4MDAwNSwxLjA0MSAwLjkzODk5NSwxLjk2MDk5NSAxLjY3OTk5MywyLjc1OTk5OGMwLjczOTk5LDAuODAxMDAzIDEuNjMwMDA1LDEuNDQwMDAyIDIuNjcwMDEzLDEuOTE5OTk4YzEuMDQwOTg1LDAuNDgwMDAzIDIuMjIwOTc4LDAuNzIxMDAxIDMuNTQwOTg1LDAuNzIxMDAxczIuNDk4OTkzLC0wLjIzOTk5OCAzLjUzOTAwMSwtMC43MjEwMDFjMS4wNDA5ODUsLTAuNDc4OTk2IDEuOTI5OTkzLC0xLjExODk5NiAyLjY3MDAxMywtMS45MTk5OThjMC43Mzk5OSwtMC43OTkwMDQgMS4zMDA5OTUsLTEuNzE4OTk4IDEuNjgxOTc2LC0yLjc1OTk5OGMwLjM3ODk5OCwtMS4wMzkwMDEgMC41NjgwMjQsLTIuMTE4OTk5IDAuNTY4MDI0LC0zLjIzOTk5OGMwLC0xLjExOTAwMyAtMC4xODkwMjYsLTIuMTk5MDAxIC0wLjU2ODAyNCwtMy4yMzkwMDJjLTAuMzgwOTgxLC0xLjA0MDAwMSAtMC45NDA5NzksLTEuOTU5OTk5IC0xLjY4MTk3NiwtMi43NjA5OThjLTAuNzQwMDIxLC0wLjc5OTAwNCAtMS42MjkwMjgsLTEuNDM5MDAzIC0yLjY3MDAxMywtMS45MjAwMDJjLTEuMDQwMDA5LC0wLjQ3OSAtMi4yMTg5OTQsLTAuNzIwMDAxIC0zLjUzOTAwMSwtMC43MjAwMDFzLTIuNSwwLjI0MDAwMiAtMy41NDA5ODUsMC43MjAwMDFjLTEuMDQwMDA5LDAuNDggLTEuOTMwMDIzLDEuMTIwOTk4IC0yLjY3MDAxMywxLjkyMDAwMmMtMC43Mzk5OSwwLjgwMDk5OSAtMS4yOTk5ODgsMS43MjA5OTcgLTEuNjc5OTkzLDIuNzYwOTk4Yy0wLjM4MDAwNSwxLjAzOTAwMSAtMC41NzAwMDcsMi4xMTg5OTkgLTAuNTcwMDA3LDMuMjM5MDAyelxcXCIvPiA8cGF0aCBkPVxcXCJtMjk3LjA3MDAwNywzNy43MDFoNy4yMDA5ODl2NC41NjAwMDFoMC4xMTkwMTljMC43OTg5ODEsLTEuNjggMS45Mzg5OTUsLTIuOTc5IDMuNDE5OTgzLC0zLjg5OTAwMnMzLjE3OTk5MywtMS4zODAwMDEgNS4xMDAwMDYsLTEuMzgwMDAxYzAuNDM4OTk1LDAgMC44NzEwMDIsMC4wNDAwMDEgMS4yOTA5ODUsMC4xMTkwMDNjMC40MjAwMTMsMC4wODA5OTcgMC44NTAwMDYsMC4xODEgMS4yODkwMDEsMC4zMDA5OTl2Ni45NTk5OTljLTAuNTk5OTc2LC0wLjE2IC0xLjE4ODk5NSwtMC4yOTAwMDEgLTEuNzY5OTg5LC0wLjM5MDk5OWMtMC41Nzk5ODcsLTAuMDk4OTk5IC0xLjE0OTk5NCwtMC4xNDkwMDIgLTEuNzEwOTk5LC0wLjE0OTAwMmMtMS42Nzk5OTMsMCAtMy4wMjg5OTIsMC4zMTAwMDEgLTQuMDQ5MDExLDAuOTNjLTEuMDE5OTg5LDAuNjIxMDAyIC0xLjgwMDk5NSwxLjMzMDAwMiAtMi4zMzk5OTYsMi4xMzAwMDFjLTAuNTQwOTg1LDAuODAwOTk5IC0wLjg5OTk5NCwxLjYwMTAwMiAtMS4wNzk5ODcsMi40MDAwMDJjLTAuMTgwMDIzLDAuODAwOTk5IC0wLjI3MDAyLDEuMzk5OTk4IC0wLjI3MDAyLDEuNzk5OTk5djE1LjQxOTk5OGgtNy4yMDA5ODl2LTI4LjgwMDk5OWwwLjAwMTAwNywwelxcXCIvPiA8cGF0aCBkPVxcXCJtMzE3LjA0OTAxMSw0My44MjA5OTl2LTYuMTE5OTk5aDUuOTQwOTc5di04LjM0aDcuMTk5MDA1djguMzRoNy45MjAwMTN2Ni4xMTk5OTloLTcuOTIwMDEzdjEyLjYwMDAwMmMwLDEuNDM5OTk5IDAuMjcwMDIsMi41Nzk5OTggMC44MTEwMDUsMy40MjAwMDJjMC41MzkwMDEsMC44Mzk5OTYgMS42MDkwMDksMS4yNTk5OTUgMy4yMDkwMTUsMS4yNTk5OTVjMC42NDA5OTEsMCAxLjMzOTk5NiwtMC4wNjkgMi4xMDE5OSwtMC4yMDk5OTljMC43NTc5OTYsLTAuMTM5OTk5IDEuMzU5MDA5LC0wLjM2OTAwMyAxLjc5ODk4MSwtMC42ODkwMDN2Ni4wNjAwMDVjLTAuNzU5OTc5LDAuMzYwMDAxIC0xLjY4ODk5NSwwLjYwODk5NCAtMi43ODg5NzEsMC43NWMtMS4xMDIwMiwwLjEzOTk5OSAtMi4wNzAwMDcsMC4yMDk5OTkgLTIuOTEwMDA0LDAuMjA5OTk5Yy0xLjkyMDAxMywwIC0zLjQ5MDAyMSwtMC4yMDk5OTkgLTQuNzEwOTk5LC0wLjYzMDAwNXMtMi4xODAwMjMsLTEuMDU5OTk4IC0yLjg3ODk5OCwtMS45MTk5OThjLTAuNzAxMDE5LC0wLjg1OTAwMSAtMS4xODIwMDcsLTEuOTMgLTEuNDQxMDEsLTMuMjA5OTkxYy0wLjI2MDAxLC0xLjI3OTAwNyAtMC4zODkwMDgsLTIuNzYwMDEgLTAuMzg5MDA4LC00LjQ0MDAwMnYtMTMuMjAxMDA0aC01Ljk0MTk4NmwwLDB6XFxcIi8+IDwvZz4gPGc+IDxwYXRoIGQ9XFxcIm0xMTkuMTk0LDg2LjI5NTk5OGgzLjU4Nzk5N2MwLjM0NjAwMSwwIDAuNjg5MDAzLDAuMDQxIDEuMDI3LDAuMTI0MDAxYzAuMzM4MDA1LDAuMDgyMDAxIDAuNjM5LDAuMjE3MDAzIDAuOTAzLDAuNDAyYzAuMjY0LDAuMTg3MDA0IDAuNDc5MDA0LDAuNDI3MDAyIDAuNjQ0MDA1LDAuNzIyczAuMjQ2OTk0LDAuNjUwMDAyIDAuMjQ2OTk0LDEuMDY2MDAyYzAsMC41MTk5OTcgLTAuMTQ2OTk2LDAuOTQ3OTk4IC0wLjQ0MTk5NCwxLjI4NzAwM2MtMC4yOTUwMDYsMC4zMzc5OTcgLTAuNjgxLDAuNTc5OTk0IC0xLjE1NzAwNSwwLjcyNzk5N3YwLjAyNjAwMWMwLjI4NjAwMywwLjAzMzk5NyAwLjU1MzAwMSwwLjExMzk5OCAwLjgwMDAwMywwLjIzOTk5OGMwLjI0NzAwMiwwLjEyNTk5OSAwLjQ1NzAwMSwwLjI4NjAwMyAwLjYyOTk5NywwLjQ4MDAwM2MwLjE3MzAwNCwwLjE5NSAwLjMxMDAwNSwwLjQyMDk5OCAwLjQwOTAwNCwwLjY3Njk5NHMwLjE0OTk5NCwwLjUzMDAwNiAwLjE0OTk5NCwwLjgyNTAwNWMwLDAuNTAyOTk4IC0wLjA5OTk5OCwwLjkyMDk5OCAtMC4yOTg5OTYsMS4yNTQ5OTdjLTAuMTk4OTk3LDAuMzMzIC0wLjQ2MDk5OSwwLjYwMzAwNCAtMC43ODYwMDMsMC44MDZjLTAuMzI0OTk3LDAuMjA0MDAyIC0wLjY5Nzk5OCwwLjM0ODk5OSAtMS4xMTc5OTYsMC40MzYwMDVzLTAuODQ4LDAuMTI5OTk3IC0xLjI4MDk5OCwwLjEyOTk5N2gtMy4zMTUwMDJ2LTkuMjA0MDAybDAsMHptMS42MzgsMy43NDQwMDNoMS40OTUwMDNjMC41NDU5OTgsMCAwLjk1NTk5NCwtMC4xMDYwMDMgMS4yMjg5OTYsLTAuMzE4MDAxYzAuMjczMDAzLC0wLjIxMjk5NyAwLjQwODk5NywtMC40OTE5OTcgMC40MDg5OTcsLTAuODM4OTk3YzAsLTAuMzk4MDAzIC0wLjE0MDk5OSwtMC42OTUgLTAuNDIxOTk3LC0wLjg5MTAwNmMtMC4yODE5OTgsLTAuMTk0IC0wLjczNDAwMSwtMC4yOTIgLTEuMzU4MDAyLC0wLjI5MmgtMS4zNTE5OTd2Mi4zNDAwMDRsLTAuMDAwOTk5LDB6bTAsNC4wNTZoMS41MDc5OTZjMC4yMDgsMCAwLjQzMTAwNywtMC4wMTMgMC42NjkwMDYsLTAuMDM5MDAxYzAuMjM3OTk5LC0wLjAyNTAwMiAwLjQ1NzAwMSwtMC4wODU5OTkgMC42NTY5OTgsLTAuMTgxOTk5YzAuMTk4OTk3LC0wLjA5NjAwMSAwLjM2Mzk5OCwtMC4yMzEwMDMgMC40OTQwMDMsLTAuNDA4OTk3YzAuMTI5OTk3LC0wLjE3ODAwMSAwLjE5NSwtMC40MTgwMDcgMC4xOTUsLTAuNzIyYzAsLTAuNDg1MDAxIC0wLjE1ODAwNSwtMC44MjMwMDYgLTAuNDc1MDA2LC0xLjAxNGMtMC4zMTU5OTQsLTAuMTkxMDAyIC0wLjgwNzk5OSwtMC4yODYwMDMgLTEuNDc1OTk4LC0wLjI4NjAwM2gtMS41NzI5OTh2Mi42NTJsMC4wMDA5OTksMHpcXFwiLz4gPHBhdGggZD1cXFwibTEzMC44NTQ5OTYsOTEuNTYwOTk3bC0zLjQ1Nzk5MywtNS4yNjQ5OTloMi4wNTQwMDFsMi4yNjE5OTMsMy42NjZsMi4yODgwMSwtMy42NjZoMS45NDk5OTdsLTMuNDU4MDA4LDUuMjY0OTk5djMuOTM5MDAzaC0xLjYzOHYtMy45MzkwMDNsMCwwelxcXCIvPiA8cGF0aCBkPVxcXCJtMTUwLjc5Njk5Nyw5NC44MjM5OTdjLTEuMTM2MDAyLDAuNjA2MDAzIC0yLjQwNDk5OSwwLjkxMDAwNCAtMy44MDg5OSwwLjkxMDAwNGMtMC43MTEwMTQsMCAtMS4zNjMwMDcsLTAuMTE0OTk4IC0xLjk1NzAwMSwtMC4zNDUwMDFzLTEuMTA1MDExLC0wLjU1NSAtMS41MzQwMTIsLTAuOTc1OTk4Yy0wLjQyOTAwMSwtMC40MjAwMDYgLTAuNzY0OTk5LC0wLjkyNTAwMyAtMS4wMDY5ODksLTEuNTE0Yy0wLjI0MzAxMSwtMC41OTAwMDQgLTAuMzYzOTk4LC0xLjI0NDAwMyAtMC4zNjM5OTgsLTEuOTY0MDA1YzAsLTAuNzM2IDAuMTIwOTg3LC0xLjQwNDk5OSAwLjM2Mzk5OCwtMi4wMDc5OTZzMC41Nzg5OTUsLTEuMTE2MDA1IDEuMDA2OTg5LC0xLjU0MWMwLjQyOTAwMSwtMC40MjQwMDQgMC45NDAwMDIsLTAuNzUwOTk5IDEuNTM0MDEyLC0wLjk4MTAwM2MwLjU5Mzk5NCwtMC4yMjg5OTYgMS4yNDU5ODcsLTAuMzQ1MDAxIDEuOTU3MDAxLC0wLjM0NTAwMWMwLjcwMTk5NiwwIDEuMzYwMDAxLDAuMDg0OTk5IDEuOTc1OTk4LDAuMjU0MDA1YzAuNjE0OTksMC4xNjg5OTkgMS4xNjYsMC40NzEwMDEgMS42NTEwMDEsMC45MDNsLTEuMjA5LDEuMjIzYy0wLjI5NTAxMywtMC4yODYwMDMgLTAuNjUyMDA4LC0wLjUwODAwMyAtMS4wNzIwMDYsLTAuNjYzMDAyYy0wLjQyMTAwNSwtMC4xNTU5OTggLTAuODY1MDA1LC0wLjIzNDAwMSAtMS4zMzI5OTMsLTAuMjM0MDAxYy0wLjQ3NzAwNSwwIC0wLjkwODAwNSwwLjA4NDk5OSAtMS4yOTQwMDYsMC4yNTM5OThjLTAuMzg0OTk1LDAuMTY5MDA2IC0wLjcxNjk5NSwwLjQwMiAtMC45OTQwMDMsMC43MDEwMDRjLTAuMjc2OTkzLDAuMjk5OTk1IC0wLjQ5MjAwNCwwLjY0ODAwMyAtMC42NDM5OTcsMS4wNDY5OTdjLTAuMTUxOTkzLDAuMzk4MDAzIC0wLjIyNzk5NywwLjgyODAwMyAtMC4yMjc5OTcsMS4yODcwMDNjMCwwLjQ5Mzk5NiAwLjA3NjAwNCwwLjk0ODk5NyAwLjIyNzk5NywxLjM2NDk5OGMwLjE1MTAwMSwwLjQxNiAwLjM2NTk5NywwLjc3NTAwMiAwLjY0Mzk5NywxLjA3OTAwMmMwLjI3NzAwOCwwLjMwMzAwMSAwLjYwOTAwOSwwLjU0MSAwLjk5NDAwMywwLjcxNDk5NmMwLjM4NjAwMiwwLjE3MzAwNCAwLjgxNzAwMSwwLjI2MDAwMiAxLjI5NDAwNiwwLjI2MDAwMmMwLjQxNiwwIDAuODA3OTk5LC0wLjAzOTAwMSAxLjE3NTk5NSwtMC4xMTY5OTdjMC4zNjc5OTYsLTAuMDc4MDAzIDAuNjk0OTkyLC0wLjE5OTAwNSAwLjk4MTAwMywtMC4zNjI5OTl2LTIuMTcxMDA1aC0xLjg4NTAxdi0xLjQ4MDk5NWgzLjUyMzAxdjQuNzA0OTk0bDAuMDAwOTkyLDB6XFxcIi8+IDxwYXRoIGQ9XFxcIm0xNTMuNzIyLDg2LjI5NTk5OGgzLjE5Nzk5OGMwLjQ0MjAwMSwwIDAuODY5MDAzLDAuMDQxIDEuMjc5OTk5LDAuMTI0MDAxYzAuNDEyMDAzLDAuMDgyMDAxIDAuNzc4LDAuMjIzIDEuMDk4OTk5LDAuNDIyMDA1YzAuMzIwMDA3LDAuMTk4OTk3IDAuNTc2MDA0LDAuNDY3OTk1IDAuNzY2OTk4LDAuODA2OTk5YzAuMTkwMDAyLDAuMzM3OTk3IDAuMjg2MDExLDAuNzY2OTk4IDAuMjg2MDExLDEuMjg1OTk1YzAsMC42Njc5OTkgLTAuMTg0OTk4LDEuMjI3MDA1IC0wLjU1MzAwOSwxLjY3ODAwMWMtMC4zNjkwMDMsMC40NTAwMDUgLTAuODk0OTg5LDAuNzIzOTk5IC0xLjU4MDAwMiwwLjgxODAwMWwyLjQ0NTAwNyw0LjA2OWgtMS45NzU5OThsLTIuMTMyMDA0LC0zLjkwMDAwMmgtMS4xOTU5OTl2My45MDAwMDJoLTEuNjM4di05LjIwNDAwMmwwLDB6bTIuOTEyMDAzLDMuOTAwMDAyYzAuMjMzOTk0LDAgMC40NjgwMDIsLTAuMDExMDAyIDAuNzAxOTk2LC0wLjAzMjk5N2MwLjIzNDAwOSwtMC4wMjEwMDQgMC40NDc5OTgsLTAuMDczMDA2IDAuNjQzOTk3LC0wLjE1NDk5OWMwLjE5NTAwNywtMC4wODMgMC4zNTI5OTcsLTAuMjA4IDAuNDczOTk5LC0wLjM3NzAwN2MwLjEyMjAwOSwtMC4xNjg5OTkgMC4xODIwMDcsLTAuNDA0OTk5IDAuMTgyMDA3LC0wLjcwOWMwLC0wLjI2ODk5NyAtMC4wNTYsLTAuNDg1MDAxIC0wLjE2OTAwNiwtMC42NDg5OTRjLTAuMTEyOTkxLC0wLjE2NTAwMSAtMC4yNTk5OTUsLTAuMjg4MDAyIC0wLjQ0MjAwMSwtMC4zNzEwMDJjLTAuMTgxOTkyLC0wLjA4MjAwMSAtMC4zODM5ODcsLTAuMTM3MDAxIC0wLjYwMzk4OSwtMC4xNjIwMDNjLTAuMjIxMDA4LC0wLjAyNjAwMSAtMC40MzYwMDUsLTAuMDM5MDAxIC0wLjY0NDAxMiwtMC4wMzkwMDFoLTEuNDE2OTkydjIuNDk2MDAyaDEuMjc0MDAybDAsLTAuMDAwOTk5elxcXCIvPiA8cGF0aCBkPVxcXCJtMTY1Ljg3NjAwNyw4Ni4yOTU5OThoMS40MTY5OTJsMy45NjYwMDMsOS4yMDQwMDJoLTEuODcyMDA5bC0wLjg1Nzk4NiwtMi4xMDYwMDNoLTMuOTkxMDEzbC0wLjgzMjAwMSwyLjEwNjAwM2gtMS44MzI5OTNsNC4wMDMwMDYsLTkuMjA0MDAyem0yLjA4MDk5NCw1LjY5NGwtMS40MTcwMDcsLTMuNzQzOTk2bC0xLjQ0Mjk5MywzLjc0Mzk5NmgyLjg2MDAwMWwwLDB6XFxcIi8+IDxwYXRoIGQ9XFxcIm0xNzEuNDAxMDAxLDg2LjI5NTk5OGgxLjg4NDk5NWwyLjUwOTAwMyw2Ljk1NTAwMmwyLjU4NzAwNiwtNi45NTUwMDJoMS43Njc5OWwtMy43MTY5OTUsOS4yMDQwMDJoLTEuNDE2OTkybC0zLjYxNTAwNSwtOS4yMDQwMDJ6XFxcIi8+IDxwYXRoIGQ9XFxcIm0xODIuMDg3MDA2LDg2LjI5NTk5OGgxLjYzOHY5LjIwNDAwMmgtMS42Mzh2LTkuMjA0MDAybDAsMHpcXFwiLz4gPHBhdGggZD1cXFwibTE4OC42MTMwMDcsODcuNzc4aC0yLjgyMDk5OXYtMS40ODIwMDJoNy4yNzk5OTl2MS40ODIwMDJoLTIuODIwOTk5djcuNzIyaC0xLjYzOHYtNy43MjJsMCwwelxcXCIvPiA8cGF0aCBkPVxcXCJtMTk2Ljk1OSw4Ni4yOTU5OThoMS40MTcwMDdsMy45NjU5ODgsOS4yMDQwMDJoLTEuODczMDAxbC0wLjg1Njk5NSwtMi4xMDYwMDNoLTMuOTkwOTk3bC0wLjgzMzAwOCwyLjEwNjAwM2gtMS44MzI5OTNsNC4wMDM5OTgsLTkuMjA0MDAyem0yLjA4MDAwMiw1LjY5NGwtMS40MTcwMDcsLTMuNzQzOTk2bC0xLjQ0MjAwMSwzLjc0Mzk5NmgyLjg1OTAwOWwwLDB6XFxcIi8+IDxwYXRoIGQ9XFxcIm0yMDUuMDQ0OTk4LDg3Ljc3OGgtMi44MTk5OTJ2LTEuNDgyMDAyaDcuMjc4OTkydjEuNDgyMDAyaC0yLjgxOTk5MnY3LjcyMmgtMS42MzkwMDh2LTcuNzIybDAsMHpcXFwiLz4gPHBhdGggZD1cXFwibTIxMS41NzAwMDcsODYuMjk1OTk4aDEuNjM4OTkydjkuMjA0MDAyaC0xLjYzODk5MnYtOS4yMDQwMDJsMCwwelxcXCIvPiA8cGF0aCBkPVxcXCJtMjE1LjcxODk5NCw5MC45MzY5OTZjMCwtMC43MzYgMC4xMjEwMDIsLTEuNDA0OTk5IDAuMzYyOTkxLC0yLjAwNzk5NnMwLjU3ODAwMywtMS4xMTU5OTcgMS4wMDgwMTEsLTEuNTQxYzAuNDI5MDAxLC0wLjQyNDAwNCAwLjkzODk5NSwtMC43NTA5OTkgMS41MzI5OSwtMC45ODEwMDNjMC41OTQwMDksLTAuMjI4OTk2IDEuMjQ2MDAyLC0wLjM0NTAwMSAxLjk1NzAwMSwtMC4zNDUwMDFjMC43MTkwMDksLTAuMDA3OTk2IDEuMzc4MDA2LDAuMDk4MDA3IDEuOTc3MDA1LDAuMzE5YzAuNTk3OTkyLDAuMjIxMDAxIDEuMTEyOTkxLDAuNTQ0MDA2IDEuNTQ2OTk3LDAuOTY4MDAyYzAuNDMyOTk5LDAuNDI1MDAzIDAuNzcwOTk2LDAuOTM3MDA0IDEuMDE0MDA4LDEuNTM0MDA0YzAuMjQxOTg5LDAuNTk4OTk5IDAuMzYyOTkxLDEuMjY1OTk5IDAuMzYyOTkxLDIuMDAxOTk5YzAsMC43MjAwMDEgLTAuMTIxMDAyLDEuMzc0MDAxIC0wLjM2Mjk5MSwxLjk2Mjk5N2MtMC4yNDIwMDQsMC41OTAwMDQgLTAuNTgxMDA5LDEuMDk3IC0xLjAxNDAwOCwxLjUyMTAwNGMtMC40MzQwMDYsMC40MjQ5OTUgLTAuOTQ5MDA1LDAuNzU1OTk3IC0xLjU0Njk5NywwLjk5Mzk5NmMtMC41OTg5OTksMC4yMzc5OTkgLTEuMjU3OTk2LDAuMzYyIC0xLjk3NzAwNSwwLjM3MTAwMmMtMC43MTA5OTksMCAtMS4zNjI5OTEsLTAuMTE0OTk4IC0xLjk1NzAwMSwtMC4zNDUwMDFzLTEuMTAzOTg5LC0wLjU1NSAtMS41MzI5OSwtMC45NzU5OThjLTAuNDMwMDA4LC0wLjQyMDAwNiAtMC43NjYwMDYsLTAuOTI1MDAzIC0xLjAwODAxMSwtMS41MTRjLTAuMjQxOTg5LC0wLjU4ODAwNSAtMC4zNjI5OTEsLTEuMjQzMDA0IC0wLjM2Mjk5MSwtMS45NjIwMDZ6bTEuNzE1MDEyLC0wLjEwMzk5NmMwLDAuNDk0MDAzIDAuMDc2MDA0LDAuOTQ4OTk3IDAuMjI5MDA0LDEuMzY0OTk4YzAuMTQ5OTk0LDAuNDE2IDAuMzY1MDA1LDAuNzc1MDAyIDAuNjQzMDA1LDEuMDc5MDAyYzAuMjc2OTkzLDAuMzAzMDAxIDAuNjA4OTk0LDAuNTQxIDAuOTkzOTg4LDAuNzE0OTk2YzAuMzg3MDA5LDAuMTczMDA0IDAuODE3MDAxLDAuMjYwMDAyIDEuMjk1MDEzLDAuMjYwMDAyYzAuNDc2OTksMCAwLjkwODk5NywtMC4wODY5OTggMS4yOTg5OTYsLTAuMjYwMDAyYzAuMzkwOTkxLC0wLjE3Mzk5NiAwLjcyNDk5MSwtMC40MTE5OTUgMS4wMDE5OTksLTAuNzE0OTk2YzAuMjc2OTkzLC0wLjMwNDAwMSAwLjQ5MDk5NywtMC42NjMwMDIgMC42NDMwMDUsLTEuMDc5MDAyYzAuMTUxOTkzLC0wLjQxNiAwLjIyODk4OSwtMC44NzA5OTUgMC4yMjg5ODksLTEuMzY0OTk4YzAsLTAuNDU5IC0wLjA3NTk4OSwtMC44ODkgLTAuMjI4OTg5LC0xLjI4NzAwM2MtMC4xNTEwMDEsLTAuMzk3OTk1IC0wLjM2NTAwNSwtMC43NDY5OTQgLTAuNjQzMDA1LC0xLjA0Njk5N2MtMC4yNzcwMDgsLTAuMjk5MDA0IC0wLjYxMTAwOCwtMC41MzE5OTggLTEuMDAxOTk5LC0wLjcwMTAwNGMtMC4zODk5OTksLTAuMTY4OTk5IC0wLjgyMjAwNiwtMC4yNTM5OTggLTEuMjk4OTk2LC0wLjI1Mzk5OGMtMC40NzgwMTIsMCAtMC45MDgwMDUsMC4wODQ5OTkgLTEuMjk1MDEzLDAuMjUzOTk4Yy0wLjM4NDk5NSwwLjE2OTAwNiAtMC43MTY5OTUsMC40MDIgLTAuOTkzOTg4LDAuNzAxMDA0Yy0wLjI3NzAwOCwwLjMwMDAwMyAtMC40OTIwMDQsMC42NDgwMDMgLTAuNjQzMDA1LDEuMDQ2OTk3Yy0wLjE1MzAxNSwwLjM5ODAwMyAtMC4yMjkwMDQsMC44MjgwMDMgLTAuMjI5MDA0LDEuMjg3MDAzelxcXCIvPiA8cGF0aCBkPVxcXCJtMjI4LjAyOTAwNyw4Ni4yOTU5OThoMi4xNzA5OWw0LjQ1OSw2LjgzODAwNWgwLjAyNjAwMXYtNi44MzgwMDVoMS42MzcwMDl2OS4yMDQwMDJoLTIuMDc5MDFsLTQuNTUwMDAzLC03LjA1ODk5OGgtMC4wMjU5ODZ2Ny4wNTg5OThoLTEuNjM4di05LjIwNDAwMmwwLDB6XFxcIi8+IDxwYXRoIGQ9XFxcIm0yNDIuMzQxOTk1LDg2LjI5NTk5OGgxLjQxNzAwN2wzLjk2NjAwMyw5LjIwNDAwMmgtMS44NzMwMDFsLTAuODU3MDEsLTIuMTA2MDAzaC0zLjk5MDk5N2wtMC44MzI5OTMsMi4xMDYwMDNoLTEuODMzMDA4bDQuMDAzOTk4LC05LjIwNDAwMnptMi4wODAwMDIsNS42OTRsLTEuNDE2OTkyLC0zLjc0Mzk5NmwtMS40NDIwMDEsMy43NDM5OTZoMi44NTg5OTRsMCwwelxcXCIvPiA8cGF0aCBkPVxcXCJtMjQ5LjczODAwNyw4Ni4yOTU5OThoMS42Mzg5OTJ2Ny43MjJoMy45MTIwMDN2MS40ODIwMDJoLTUuNTUwOTk1di05LjIwNDAwMmwwLDB6XFxcIi8+IDwvZz4gPC9nPiA8L3N5bWJvbD5cIjtcbm1vZHVsZS5leHBvcnRzID0gc3ByaXRlLmFkZChpbWFnZSwgXCJncnYtdGxwdC1sb2dvLWZ1bGxcIik7XG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiAuL3NyYy9hc3NldHMvaW1nL3N2Zy9ncnYtdGxwdC1sb2dvLWZ1bGwuc3ZnXG4gKiogbW9kdWxlIGlkID0gNDM5XG4gKiogbW9kdWxlIGNodW5rcyA9IDFcbiAqKi8iLCJ2YXIgU3ByaXRlID0gcmVxdWlyZSgnLi9zcHJpdGUnKTtcbnZhciBnbG9iYWxTcHJpdGUgPSBuZXcgU3ByaXRlKCk7XG5cbmlmIChkb2N1bWVudC5ib2R5KSB7XG4gIGdsb2JhbFNwcml0ZS5lbGVtID0gZ2xvYmFsU3ByaXRlLnJlbmRlcihkb2N1bWVudC5ib2R5KTtcbn0gZWxzZSB7XG4gIGRvY3VtZW50LmFkZEV2ZW50TGlzdGVuZXIoJ0RPTUNvbnRlbnRMb2FkZWQnLCBmdW5jdGlvbiAoKSB7XG4gICAgZ2xvYmFsU3ByaXRlLmVsZW0gPSBnbG9iYWxTcHJpdGUucmVuZGVyKGRvY3VtZW50LmJvZHkpO1xuICB9LCBmYWxzZSk7XG59XG5cbm1vZHVsZS5leHBvcnRzID0gZ2xvYmFsU3ByaXRlO1xuXG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiAuL34vc3ZnLXNwcml0ZS1sb2FkZXIvbGliL3dlYi9nbG9iYWwtc3ByaXRlLmpzXG4gKiogbW9kdWxlIGlkID0gNDQwXG4gKiogbW9kdWxlIGNodW5rcyA9IDFcbiAqKi8iLCJ2YXIgU25pZmZyID0gcmVxdWlyZSgnc25pZmZyJyk7XG5cbi8qKlxuICogTGlzdCBvZiBTVkcgYXR0cmlidXRlcyB0byBmaXggdXJsIHRhcmdldCBpbiB0aGVtXG4gKiBAdHlwZSB7c3RyaW5nW119XG4gKi9cbnZhciBmaXhBdHRyaWJ1dGVzID0gW1xuICAnY2xpcFBhdGgnLFxuICAnY29sb3JQcm9maWxlJyxcbiAgJ3NyYycsXG4gICdjdXJzb3InLFxuICAnZmlsbCcsXG4gICdmaWx0ZXInLFxuICAnbWFya2VyJyxcbiAgJ21hcmtlclN0YXJ0JyxcbiAgJ21hcmtlck1pZCcsXG4gICdtYXJrZXJFbmQnLFxuICAnbWFzaycsXG4gICdzdHJva2UnXG5dO1xuXG4vKipcbiAqIFF1ZXJ5IHRvIGZpbmQnZW1cbiAqIEB0eXBlIHtzdHJpbmd9XG4gKi9cbnZhciBmaXhBdHRyaWJ1dGVzUXVlcnkgPSAnWycgKyBmaXhBdHRyaWJ1dGVzLmpvaW4oJ10sWycpICsgJ10nO1xuLyoqXG4gKiBAdHlwZSB7UmVnRXhwfVxuICovXG52YXIgVVJJX0ZVTkNfUkVHRVggPSAvXnVybFxcKCguKilcXCkkLztcblxuLyoqXG4gKiBDb252ZXJ0IGFycmF5LWxpa2UgdG8gYXJyYXlcbiAqIEBwYXJhbSB7T2JqZWN0fSBhcnJheUxpa2VcbiAqIEByZXR1cm5zIHtBcnJheS48Kj59XG4gKi9cbmZ1bmN0aW9uIGFycmF5RnJvbShhcnJheUxpa2UpIHtcbiAgcmV0dXJuIEFycmF5LnByb3RvdHlwZS5zbGljZS5jYWxsKGFycmF5TGlrZSwgMCk7XG59XG5cbi8qKlxuICogSGFuZGxlcyBmb3JiaWRkZW4gc3ltYm9scyB3aGljaCBjYW5ub3QgYmUgZGlyZWN0bHkgdXNlZCBpbnNpZGUgYXR0cmlidXRlcyB3aXRoIHVybCguLi4pIGNvbnRlbnQuXG4gKiBBZGRzIGxlYWRpbmcgc2xhc2ggZm9yIHRoZSBicmFja2V0c1xuICogQHBhcmFtIHtzdHJpbmd9IHVybFxuICogQHJldHVybiB7c3RyaW5nfSBlbmNvZGVkIHVybFxuICovXG5mdW5jdGlvbiBlbmNvZGVVcmxGb3JFbWJlZGRpbmcodXJsKSB7XG4gIHJldHVybiB1cmwucmVwbGFjZSgvXFwofFxcKS9nLCBcIlxcXFwkJlwiKTtcbn1cblxuLyoqXG4gKiBSZXBsYWNlcyBwcmVmaXggaW4gYHVybCgpYCBmdW5jdGlvbnNcbiAqIEBwYXJhbSB7RWxlbWVudH0gc3ZnXG4gKiBAcGFyYW0ge3N0cmluZ30gY3VycmVudFVybFByZWZpeFxuICogQHBhcmFtIHtzdHJpbmd9IG5ld1VybFByZWZpeFxuICovXG5mdW5jdGlvbiBiYXNlVXJsV29ya0Fyb3VuZChzdmcsIGN1cnJlbnRVcmxQcmVmaXgsIG5ld1VybFByZWZpeCkge1xuICB2YXIgbm9kZXMgPSBzdmcucXVlcnlTZWxlY3RvckFsbChmaXhBdHRyaWJ1dGVzUXVlcnkpO1xuXG4gIGlmICghbm9kZXMpIHtcbiAgICByZXR1cm47XG4gIH1cblxuICBhcnJheUZyb20obm9kZXMpLmZvckVhY2goZnVuY3Rpb24gKG5vZGUpIHtcbiAgICBpZiAoIW5vZGUuYXR0cmlidXRlcykge1xuICAgICAgcmV0dXJuO1xuICAgIH1cblxuICAgIGFycmF5RnJvbShub2RlLmF0dHJpYnV0ZXMpLmZvckVhY2goZnVuY3Rpb24gKGF0dHJpYnV0ZSkge1xuICAgICAgdmFyIGF0dHJpYnV0ZU5hbWUgPSBhdHRyaWJ1dGUubG9jYWxOYW1lLnRvTG93ZXJDYXNlKCk7XG5cbiAgICAgIGlmIChmaXhBdHRyaWJ1dGVzLmluZGV4T2YoYXR0cmlidXRlTmFtZSkgIT09IC0xKSB7XG4gICAgICAgIHZhciBtYXRjaCA9IFVSSV9GVU5DX1JFR0VYLmV4ZWMobm9kZS5nZXRBdHRyaWJ1dGUoYXR0cmlidXRlTmFtZSkpO1xuXG4gICAgICAgIC8vIERvIG5vdCB0b3VjaCB1cmxzIHdpdGggdW5leHBlY3RlZCBwcmVmaXhcbiAgICAgICAgaWYgKG1hdGNoICYmIG1hdGNoWzFdLmluZGV4T2YoY3VycmVudFVybFByZWZpeCkgPT09IDApIHtcbiAgICAgICAgICB2YXIgcmVmZXJlbmNlVXJsID0gZW5jb2RlVXJsRm9yRW1iZWRkaW5nKG5ld1VybFByZWZpeCArIG1hdGNoWzFdLnNwbGl0KGN1cnJlbnRVcmxQcmVmaXgpWzFdKTtcbiAgICAgICAgICBub2RlLnNldEF0dHJpYnV0ZShhdHRyaWJ1dGVOYW1lLCAndXJsKCcgKyByZWZlcmVuY2VVcmwgKyAnKScpO1xuICAgICAgICB9XG4gICAgICB9XG4gICAgfSk7XG4gIH0pO1xufVxuXG4vKipcbiAqIEJlY2F1c2Ugb2YgRmlyZWZveCBidWcgIzM1MzU3NSBncmFkaWVudHMgYW5kIHBhdHRlcm5zIGRvbid0IHdvcmsgaWYgdGhleSBhcmUgd2l0aGluIGEgc3ltYm9sLlxuICogVG8gd29ya2Fyb3VuZCB0aGlzIHdlIG1vdmUgdGhlIGdyYWRpZW50IGRlZmluaXRpb24gb3V0c2lkZSB0aGUgc3ltYm9sIGVsZW1lbnRcbiAqIEBzZWUgaHR0cHM6Ly9idWd6aWxsYS5tb3ppbGxhLm9yZy9zaG93X2J1Zy5jZ2k/aWQ9MzUzNTc1XG4gKiBAcGFyYW0ge0VsZW1lbnR9IHN2Z1xuICovXG52YXIgRmlyZWZveFN5bWJvbEJ1Z1dvcmthcm91bmQgPSBmdW5jdGlvbiAoc3ZnKSB7XG4gIHZhciBkZWZzID0gc3ZnLnF1ZXJ5U2VsZWN0b3IoJ2RlZnMnKTtcblxuICB2YXIgbW92ZVRvRGVmc0VsZW1zID0gc3ZnLnF1ZXJ5U2VsZWN0b3JBbGwoJ3N5bWJvbCBsaW5lYXJHcmFkaWVudCwgc3ltYm9sIHJhZGlhbEdyYWRpZW50LCBzeW1ib2wgcGF0dGVybicpO1xuICBmb3IgKHZhciBpID0gMCwgbGVuID0gbW92ZVRvRGVmc0VsZW1zLmxlbmd0aDsgaSA8IGxlbjsgaSsrKSB7XG4gICAgZGVmcy5hcHBlbmRDaGlsZChtb3ZlVG9EZWZzRWxlbXNbaV0pO1xuICB9XG59O1xuXG4vKipcbiAqIEB0eXBlIHtzdHJpbmd9XG4gKi9cbnZhciBERUZBVUxUX1VSSV9QUkVGSVggPSAnIyc7XG5cbi8qKlxuICogQHR5cGUge3N0cmluZ31cbiAqL1xudmFyIHhMaW5rSHJlZiA9ICd4bGluazpocmVmJztcbi8qKlxuICogQHR5cGUge3N0cmluZ31cbiAqL1xudmFyIHhMaW5rTlMgPSAnaHR0cDovL3d3dy53My5vcmcvMTk5OS94bGluayc7XG4vKipcbiAqIEB0eXBlIHtzdHJpbmd9XG4gKi9cbnZhciBzdmdPcGVuaW5nID0gJzxzdmcgeG1sbnM9XCJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2Z1wiIHhtbG5zOnhsaW5rPVwiJyArIHhMaW5rTlMgKyAnXCInO1xuLyoqXG4gKiBAdHlwZSB7c3RyaW5nfVxuICovXG52YXIgc3ZnQ2xvc2luZyA9ICc8L3N2Zz4nO1xuLyoqXG4gKiBAdHlwZSB7c3RyaW5nfVxuICovXG52YXIgY29udGVudFBsYWNlSG9sZGVyID0gJ3tjb250ZW50fSc7XG5cbi8qKlxuICogUmVwcmVzZW50YXRpb24gb2YgU1ZHIHNwcml0ZVxuICogQGNvbnN0cnVjdG9yXG4gKi9cbmZ1bmN0aW9uIFNwcml0ZSgpIHtcbiAgdmFyIGJhc2VFbGVtZW50ID0gZG9jdW1lbnQuZ2V0RWxlbWVudHNCeVRhZ05hbWUoJ2Jhc2UnKVswXTtcbiAgdmFyIGN1cnJlbnRVcmwgPSB3aW5kb3cubG9jYXRpb24uaHJlZi5zcGxpdCgnIycpWzBdO1xuICB2YXIgYmFzZVVybCA9IGJhc2VFbGVtZW50ICYmIGJhc2VFbGVtZW50LmhyZWY7XG4gIHRoaXMudXJsUHJlZml4ID0gYmFzZVVybCAmJiBiYXNlVXJsICE9PSBjdXJyZW50VXJsID8gY3VycmVudFVybCArIERFRkFVTFRfVVJJX1BSRUZJWCA6IERFRkFVTFRfVVJJX1BSRUZJWDtcblxuICB2YXIgc25pZmZyID0gbmV3IFNuaWZmcigpO1xuICBzbmlmZnIuc25pZmYoKTtcbiAgdGhpcy5icm93c2VyID0gc25pZmZyLmJyb3dzZXI7XG4gIHRoaXMuY29udGVudCA9IFtdO1xuXG4gIGlmICh0aGlzLmJyb3dzZXIubmFtZSAhPT0gJ2llJyAmJiBiYXNlVXJsKSB7XG4gICAgd2luZG93LmFkZEV2ZW50TGlzdGVuZXIoJ3Nwcml0ZUxvYWRlckxvY2F0aW9uVXBkYXRlZCcsIGZ1bmN0aW9uIChlKSB7XG4gICAgICB2YXIgY3VycmVudFByZWZpeCA9IHRoaXMudXJsUHJlZml4O1xuICAgICAgdmFyIG5ld1VybFByZWZpeCA9IGUuZGV0YWlsLm5ld1VybC5zcGxpdChERUZBVUxUX1VSSV9QUkVGSVgpWzBdICsgREVGQVVMVF9VUklfUFJFRklYO1xuICAgICAgYmFzZVVybFdvcmtBcm91bmQodGhpcy5zdmcsIGN1cnJlbnRQcmVmaXgsIG5ld1VybFByZWZpeCk7XG4gICAgICB0aGlzLnVybFByZWZpeCA9IG5ld1VybFByZWZpeDtcblxuICAgICAgaWYgKHRoaXMuYnJvd3Nlci5uYW1lID09PSAnZmlyZWZveCcgfHwgdGhpcy5icm93c2VyLm5hbWUgPT09ICdlZGdlJyB8fCB0aGlzLmJyb3dzZXIubmFtZSA9PT0gJ2Nocm9tZScgJiYgdGhpcy5icm93c2VyLnZlcnNpb25bMF0gPj0gNDkpIHtcbiAgICAgICAgdmFyIG5vZGVzID0gYXJyYXlGcm9tKGRvY3VtZW50LnF1ZXJ5U2VsZWN0b3JBbGwoJ3VzZVsqfGhyZWZdJykpO1xuICAgICAgICBub2Rlcy5mb3JFYWNoKGZ1bmN0aW9uIChub2RlKSB7XG4gICAgICAgICAgdmFyIGhyZWYgPSBub2RlLmdldEF0dHJpYnV0ZSh4TGlua0hyZWYpO1xuICAgICAgICAgIGlmIChocmVmICYmIGhyZWYuaW5kZXhPZihjdXJyZW50UHJlZml4KSA9PT0gMCkge1xuICAgICAgICAgICAgbm9kZS5zZXRBdHRyaWJ1dGVOUyh4TGlua05TLCB4TGlua0hyZWYsIG5ld1VybFByZWZpeCArIGhyZWYuc3BsaXQoREVGQVVMVF9VUklfUFJFRklYKVsxXSk7XG4gICAgICAgICAgfVxuICAgICAgICB9KTtcbiAgICAgIH1cbiAgICB9LmJpbmQodGhpcykpO1xuICB9XG59XG5cblNwcml0ZS5zdHlsZXMgPSBbJ3Bvc2l0aW9uOmFic29sdXRlJywgJ3dpZHRoOjAnLCAnaGVpZ2h0OjAnLCAndmlzaWJpbGl0eTpoaWRkZW4nXTtcblxuU3ByaXRlLnNwcml0ZVRlbXBsYXRlID0gc3ZnT3BlbmluZyArICcgc3R5bGU9XCInKyBTcHJpdGUuc3R5bGVzLmpvaW4oJzsnKSArJ1wiPjxkZWZzPicgKyBjb250ZW50UGxhY2VIb2xkZXIgKyAnPC9kZWZzPicgKyBzdmdDbG9zaW5nO1xuU3ByaXRlLnN5bWJvbFRlbXBsYXRlID0gc3ZnT3BlbmluZyArICc+JyArIGNvbnRlbnRQbGFjZUhvbGRlciArIHN2Z0Nsb3Npbmc7XG5cbi8qKlxuICogQHR5cGUge0FycmF5PFN0cmluZz59XG4gKi9cblNwcml0ZS5wcm90b3R5cGUuY29udGVudCA9IG51bGw7XG5cbi8qKlxuICogQHBhcmFtIHtTdHJpbmd9IGNvbnRlbnRcbiAqIEBwYXJhbSB7U3RyaW5nfSBpZFxuICovXG5TcHJpdGUucHJvdG90eXBlLmFkZCA9IGZ1bmN0aW9uIChjb250ZW50LCBpZCkge1xuICBpZiAodGhpcy5zdmcpIHtcbiAgICB0aGlzLmFwcGVuZFN5bWJvbChjb250ZW50KTtcbiAgfVxuXG4gIHRoaXMuY29udGVudC5wdXNoKGNvbnRlbnQpO1xuXG4gIHJldHVybiBERUZBVUxUX1VSSV9QUkVGSVggKyBpZDtcbn07XG5cbi8qKlxuICpcbiAqIEBwYXJhbSBjb250ZW50XG4gKiBAcGFyYW0gdGVtcGxhdGVcbiAqIEByZXR1cm5zIHtFbGVtZW50fVxuICovXG5TcHJpdGUucHJvdG90eXBlLndyYXBTVkcgPSBmdW5jdGlvbiAoY29udGVudCwgdGVtcGxhdGUpIHtcbiAgdmFyIHN2Z1N0cmluZyA9IHRlbXBsYXRlLnJlcGxhY2UoY29udGVudFBsYWNlSG9sZGVyLCBjb250ZW50KTtcblxuICB2YXIgc3ZnID0gbmV3IERPTVBhcnNlcigpLnBhcnNlRnJvbVN0cmluZyhzdmdTdHJpbmcsICdpbWFnZS9zdmcreG1sJykuZG9jdW1lbnRFbGVtZW50O1xuXG4gIGlmICh0aGlzLmJyb3dzZXIubmFtZSAhPT0gJ2llJyAmJiB0aGlzLnVybFByZWZpeCkge1xuICAgIGJhc2VVcmxXb3JrQXJvdW5kKHN2ZywgREVGQVVMVF9VUklfUFJFRklYLCB0aGlzLnVybFByZWZpeCk7XG4gIH1cblxuICByZXR1cm4gc3ZnO1xufTtcblxuU3ByaXRlLnByb3RvdHlwZS5hcHBlbmRTeW1ib2wgPSBmdW5jdGlvbiAoY29udGVudCkge1xuICB2YXIgc3ltYm9sID0gdGhpcy53cmFwU1ZHKGNvbnRlbnQsIFNwcml0ZS5zeW1ib2xUZW1wbGF0ZSkuY2hpbGROb2Rlc1swXTtcblxuICB0aGlzLnN2Zy5xdWVyeVNlbGVjdG9yKCdkZWZzJykuYXBwZW5kQ2hpbGQoc3ltYm9sKTtcbiAgaWYgKHRoaXMuYnJvd3Nlci5uYW1lID09PSAnZmlyZWZveCcpIHtcbiAgICBGaXJlZm94U3ltYm9sQnVnV29ya2Fyb3VuZCh0aGlzLnN2Zyk7XG4gIH1cbn07XG5cbi8qKlxuICogQHJldHVybnMge1N0cmluZ31cbiAqL1xuU3ByaXRlLnByb3RvdHlwZS50b1N0cmluZyA9IGZ1bmN0aW9uICgpIHtcbiAgdmFyIHdyYXBwZXIgPSBkb2N1bWVudC5jcmVhdGVFbGVtZW50KCdkaXYnKTtcbiAgd3JhcHBlci5hcHBlbmRDaGlsZCh0aGlzLnJlbmRlcigpKTtcbiAgcmV0dXJuIHdyYXBwZXIuaW5uZXJIVE1MO1xufTtcblxuLyoqXG4gKiBAcGFyYW0ge0hUTUxFbGVtZW50fSBbdGFyZ2V0XVxuICogQHBhcmFtIHtCb29sZWFufSBbcHJlcGVuZD10cnVlXVxuICogQHJldHVybnMge0hUTUxFbGVtZW50fSBSZW5kZXJlZCBzcHJpdGUgbm9kZVxuICovXG5TcHJpdGUucHJvdG90eXBlLnJlbmRlciA9IGZ1bmN0aW9uICh0YXJnZXQsIHByZXBlbmQpIHtcbiAgdGFyZ2V0ID0gdGFyZ2V0IHx8IG51bGw7XG4gIHByZXBlbmQgPSB0eXBlb2YgcHJlcGVuZCA9PT0gJ2Jvb2xlYW4nID8gcHJlcGVuZCA6IHRydWU7XG5cbiAgdmFyIHN2ZyA9IHRoaXMud3JhcFNWRyh0aGlzLmNvbnRlbnQuam9pbignJyksIFNwcml0ZS5zcHJpdGVUZW1wbGF0ZSk7XG5cbiAgaWYgKHRoaXMuYnJvd3Nlci5uYW1lID09PSAnZmlyZWZveCcpIHtcbiAgICBGaXJlZm94U3ltYm9sQnVnV29ya2Fyb3VuZChzdmcpO1xuICB9XG5cbiAgaWYgKHRhcmdldCkge1xuICAgIGlmIChwcmVwZW5kICYmIHRhcmdldC5jaGlsZE5vZGVzWzBdKSB7XG4gICAgICB0YXJnZXQuaW5zZXJ0QmVmb3JlKHN2ZywgdGFyZ2V0LmNoaWxkTm9kZXNbMF0pO1xuICAgIH0gZWxzZSB7XG4gICAgICB0YXJnZXQuYXBwZW5kQ2hpbGQoc3ZnKTtcbiAgICB9XG4gIH1cblxuICB0aGlzLnN2ZyA9IHN2ZztcblxuICByZXR1cm4gc3ZnO1xufTtcblxubW9kdWxlLmV4cG9ydHMgPSBTcHJpdGU7XG5cblxuXG4vKioqKioqKioqKioqKioqKipcbiAqKiBXRUJQQUNLIEZPT1RFUlxuICoqIC4vfi9zdmctc3ByaXRlLWxvYWRlci9saWIvd2ViL3Nwcml0ZS5qc1xuICoqIG1vZHVsZSBpZCA9IDQ0MVxuICoqIG1vZHVsZSBjaHVua3MgPSAxXG4gKiovIiwibW9kdWxlLmV4cG9ydHMgPSBUZXJtaW5hbDtcblxuXG4vKioqKioqKioqKioqKioqKipcbiAqKiBXRUJQQUNLIEZPT1RFUlxuICoqIGV4dGVybmFsIFwiVGVybWluYWxcIlxuICoqIG1vZHVsZSBpZCA9IDQ0M1xuICoqIG1vZHVsZSBjaHVua3MgPSAxXG4gKiovIl0sInNvdXJjZVJvb3QiOiIifQ==