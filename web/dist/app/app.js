webpackJsonp([1],{

/***/ 0:
/***/ function(module, exports, __webpack_require__) {

	module.exports = __webpack_require__(186);


/***/ },

/***/ 16:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _nuclearJs = __webpack_require__(30);
	
	var reactor = new _nuclearJs.Reactor({
	  debug: true
	});
	
	window.reactor = reactor;
	
	exports['default'] = reactor;
	module.exports = exports['default'];

/***/ },

/***/ 22:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(142);
	
	var formatPattern = _require.formatPattern;
	
	var cfg = {
	
	  baseUrl: window.location.origin,
	
	  api: {
	    nodesPath: '/nodes',
	    sessionPath: '/v1/webapi/sessions',
	    invitePath: '/v1/webapi/users/invites/:inviteToken',
	    createUserPath: '/v1/webapi/users',
	    getInviteUrl: function getInviteUrl(inviteToken) {
	      return formatPattern(cfg.api.invitePath, { inviteToken: inviteToken });
	    }
	  },
	
	  routes: {
	    app: '/web',
	    logout: '/web/logout',
	    login: '/web/login',
	    nodes: '/web/nodes',
	    newUser: '/web/newuser/:inviteToken',
	    sessions: '/web/sessions'
	  }
	
	};
	
	exports['default'] = cfg;
	module.exports = exports['default'];

/***/ },

/***/ 35:
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

/***/ 38:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var _require = __webpack_require__(42);
	
	var browserHistory = _require.browserHistory;
	
	var AUTH_KEY_DATA = 'authData';
	
	var _history = null;
	
	var session = {
	
	  init: function init() {
	    var history = arguments.length <= 0 || arguments[0] === undefined ? browserHistory : arguments[0];
	
	    _history = history;
	  },
	
	  getHistory: function getHistory() {
	    return _history;
	  },
	
	  setUserData: function setUserData(userData) {
	    sessionStorage.setItem(AUTH_KEY_DATA, JSON.stringify(userData));
	  },
	
	  getUserData: function getUserData() {
	    var item = sessionStorage.getItem(AUTH_KEY_DATA);
	    if (item) {
	      return JSON.parse(item);
	    }
	
	    return {};
	  },
	
	  clear: function clear() {
	    sessionStorage.clear();
	  }
	
	};
	
	module.exports = session;

/***/ },

/***/ 51:
/***/ function(module, exports) {

	module.exports = jQuery;

/***/ },

/***/ 52:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var $ = __webpack_require__(51);
	var session = __webpack_require__(38);
	
	var api = {
	
	  post: function post(path, data) {
	    return api.ajax({ url: path, data: JSON.stringify(data), type: 'POST' }, false);
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

/***/ 82:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var api = __webpack_require__(52);
	var session = __webpack_require__(38);
	var cfg = __webpack_require__(22);
	var $ = __webpack_require__(51);
	
	var refreshRate = 60000 * 1; // 1 min
	
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
	    if (session.getUserData()) {
	      // refresh timer will not be set in case of browser refresh event
	      if (auth._getRefreshTokenTimerId() === null) {
	        return auth._login().done(auth._startTokenRefresher);
	      }
	
	      return $.Deferred().resolve();
	    }
	
	    return $.Deferred().reject();
	  },
	
	  logout: function logout() {
	    auth._stopTokenRefresher();
	    return session.clear();
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
	    auth._login().fail(function () {
	      auth.logout();
	      window.location.reload();
	    });
	  },
	
	  _login: function _login(name, password, token) {
	    var data = {
	      user: name,
	      pass: password,
	      second_factor_token: token
	    };
	
	    return api.post(cfg.api.sessionPath, data).then(function (data) {
	      session.setUserData(data);
	      return data;
	    });
	  }
	};
	
	module.exports = auth;

/***/ },

/***/ 83:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(35);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_RECEIVE_USER_INVITE: null
	});
	module.exports = exports['default'];

/***/ },

/***/ 84:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(30);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(83);
	
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

/***/ 85:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(35);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_RECEIVE_NODES: null
	});
	module.exports = exports['default'];

/***/ },

/***/ 86:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(30);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(85);
	
	var TLPT_RECEIVE_NODES = _require2.TLPT_RECEIVE_NODES;
	exports['default'] = Store({
	  getInitialState: function getInitialState() {
	    return toImmutable({});
	  },
	
	  initialize: function initialize() {
	    this.on(TLPT_RECEIVE_NODES, receiveNodes);
	  }
	});
	
	function receiveNodes(state, nodeArrayData) {
	  return state.withMutations(function (state) {
	    nodeArrayData.forEach(function (item) {
	      state.set(item.id, toImmutable(item));
	    });
	  });
	}
	module.exports = exports['default'];

/***/ },

/***/ 87:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(35);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_REST_API_START: null,
	  TLPT_REST_API_SUCCESS: null,
	  TLPT_REST_API_FAIL: null
	});
	module.exports = exports['default'];

/***/ },

/***/ 88:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(35);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TRYING_TO_SIGN_UP: null
	});
	module.exports = exports['default'];

/***/ },

/***/ 89:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _keymirror = __webpack_require__(35);
	
	var _keymirror2 = _interopRequireDefault(_keymirror);
	
	exports['default'] = _keymirror2['default']({
	  TLPT_RECEIVE_USER: null
	});
	module.exports = exports['default'];

/***/ },

/***/ 90:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(153);
	module.exports.actions = __webpack_require__(152);
	module.exports.nodeStore = __webpack_require__(91);
	
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

/***/ 91:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(30);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(89);
	
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

/***/ 103:
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

/***/ 142:
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

/***/ 143:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var reactor = __webpack_require__(16);
	reactor.registerStores({
	  'tlpt_user': __webpack_require__(91),
	  'tlpt_nodes': __webpack_require__(86),
	  'tlpt_invite': __webpack_require__(84),
	  'tlpt_rest_api': __webpack_require__(151)
	});

/***/ },

/***/ 144:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(16);
	
	var _require = __webpack_require__(83);
	
	var TLPT_RECEIVE_USER_INVITE = _require.TLPT_RECEIVE_USER_INVITE;
	
	var api = __webpack_require__(52);
	var cfg = __webpack_require__(22);
	
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

/***/ 145:
/***/ function(module, exports, __webpack_require__) {

	/*eslint no-undef: 0,  no-unused-vars: 0, no-debugger:0*/
	
	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(88);
	
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

/***/ 146:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(145);
	module.exports.actions = __webpack_require__(144);
	module.exports.nodeStore = __webpack_require__(84);

/***/ },

/***/ 147:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(16);
	
	var _require = __webpack_require__(85);
	
	var TLPT_RECEIVE_NODES = _require.TLPT_RECEIVE_NODES;
	
	var api = __webpack_require__(52);
	var cfg = __webpack_require__(22);
	
	exports['default'] = {
	  fetchNodes: function fetchNodes() {
	    api.get(cfg.api.nodes).done(function (nodes) {
	      reactor.dispatch(TLPT_RECEIVE_NODES, nodes);
	    });
	  }
	};
	module.exports = exports['default'];

/***/ },

/***/ 148:
/***/ function(module, exports) {

	//var sort = require('app/common/sort');
	
	'use strict';
	
	exports.__esModule = true;
	var nodeListView = [['tlpt_nodes'], function (nodes) {
	  return nodes.valueSeq().map(function (item) {
	    return {
	      count: item.get('count'),
	      ip: item.get('ip'),
	      tags: ['tag1', 'tag2', 'tag3'],
	      roles: ['r1', 'r2', 'r3']
	    };
	  }).toJS();
	}];
	
	exports['default'] = {
	  nodeListView: nodeListView
	};
	module.exports = exports['default'];

/***/ },

/***/ 149:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	module.exports.getters = __webpack_require__(148);
	module.exports.actions = __webpack_require__(147);
	module.exports.nodeStore = __webpack_require__(86);
	
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

/***/ 150:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(16);
	
	var _require = __webpack_require__(87);
	
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

/***/ 151:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _require = __webpack_require__(30);
	
	var Store = _require.Store;
	var toImmutable = _require.toImmutable;
	
	var _require2 = __webpack_require__(87);
	
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

/***/ 152:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	var reactor = __webpack_require__(16);
	
	var _require = __webpack_require__(89);
	
	var TLPT_RECEIVE_USER = _require.TLPT_RECEIVE_USER;
	
	var _require2 = __webpack_require__(88);
	
	var TRYING_TO_SIGN_UP = _require2.TRYING_TO_SIGN_UP;
	
	var restApiActions = __webpack_require__(150);
	var auth = __webpack_require__(82);
	var session = __webpack_require__(38);
	var cfg = __webpack_require__(22);
	
	exports['default'] = {
	
	  signUp: function signUp(_ref) {
	    var name = _ref.name;
	    var psw = _ref.psw;
	    var token = _ref.token;
	    var inviteToken = _ref.inviteToken;
	
	    restApiActions.start(TRYING_TO_SIGN_UP);
	    auth.signUp(name, psw, token, inviteToken).done(function (user) {
	      reactor.dispatch(TLPT_RECEIVE_USER, user);
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
	
	    auth.login(user, password, token).done(function (user) {
	      reactor.dispatch(TLPT_RECEIVE_USER, user);
	      session.getHistory().push({ pathname: redirect });
	    }).fail(function () {});
	  }
	};
	module.exports = exports['default'];

/***/ },

/***/ 153:
/***/ function(module, exports) {

	"use strict";

/***/ },

/***/ 178:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	var NavLeftBar = __webpack_require__(181);
	var cfg = __webpack_require__(22);
	
	var App = React.createClass({
	  displayName: 'App',
	
	  render: function render() {
	    return React.createElement(
	      'div',
	      { className: 'grv' },
	      React.createElement(NavLeftBar, null),
	      React.createElement(
	        'div',
	        { className: 'row', style: { marginLeft: "70px" } },
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
	                'span',
	                { className: 'm-r-sm text-muted welcome-message' },
	                'Welcome to Gravitational Portal'
	              )
	            ),
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
	        { style: { 'marginLeft': '100px' } },
	        this.props.children
	      )
	    );
	  }
	});
	
	module.exports = App;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "app.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 179:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	module.exports.App = __webpack_require__(178);
	module.exports.Login = __webpack_require__(180);
	module.exports.NewUser = __webpack_require__(182);
	module.exports.Nodes = __webpack_require__(183);
	module.exports.Sessions = __webpack_require__(184);
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "index.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 180:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };
	
	var React = __webpack_require__(5);
	var $ = __webpack_require__(51);
	var reactor = __webpack_require__(16);
	var LinkedStateMixin = __webpack_require__(57);
	
	var _require = __webpack_require__(90);
	
	var actions = _require.actions;
	
	var GoogleAuthInfo = __webpack_require__(103);
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
	      { className: 'grv grv-login text-center' },
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

/***/ 181:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	
	var _require = __webpack_require__(42);
	
	var Router = _require.Router;
	var IndexLink = _require.IndexLink;
	var History = _require.History;
	
	var cfg = __webpack_require__(22);
	
	var menuItems = [{ icon: 'fa fa fa-sitemap', to: cfg.routes.nodes, title: 'Nodes' }, { icon: 'fa fa-hdd-o', to: cfg.routes.sessions, title: 'Sessions' }];
	
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

/***/ 182:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	var $ = __webpack_require__(51);
	var reactor = __webpack_require__(16);
	
	var _require = __webpack_require__(146);
	
	var actions = _require.actions;
	var getters = _require.getters;
	
	var userModule = __webpack_require__(90);
	var LinkedStateMixin = __webpack_require__(57);
	var GoogleAuthInfo = __webpack_require__(103);
	
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
	      { className: 'grv grv-invite text-center' },
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

/***/ 183:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }
	
	var React = __webpack_require__(5);
	var reactor = __webpack_require__(16);
	
	var _require = __webpack_require__(149);
	
	var getters = _require.getters;
	var actions = _require.actions;
	
	var _require2 = __webpack_require__(185);
	
	var Table = _require2.Table;
	var Column = _require2.Column;
	var Cell = _require2.Cell;
	
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
	
	var Nodes = React.createClass({
	  displayName: 'Nodes',
	
	  mixins: [reactor.ReactMixin],
	
	  getDataBindings: function getDataBindings() {
	    return {
	      nodeRecords: getters.nodeListView
	    };
	  },
	
	  componentDidMount: function componentDidMount() {
	    actions.fetchNodes();
	  },
	
	  renderRows: function renderRows() {},
	
	  render: function render() {
	    var data = this.state.nodeRecords;
	    return React.createElement(
	      'div',
	      null,
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
	              { rowCount: data.length },
	              React.createElement(Column, {
	                columnKey: 'count',
	                header: React.createElement(
	                  Cell,
	                  null,
	                  ' Sessions '
	                ),
	                cell: React.createElement(TextCell, { data: data })
	              }),
	              React.createElement(Column, {
	                columnKey: 'ip',
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
	                cell: React.createElement(TextCell, { data: data })
	              }),
	              React.createElement(Column, {
	                columnKey: 'roles',
	                header: React.createElement(
	                  Cell,
	                  null,
	                  'Login as'
	                ),
	                cell: React.createElement(TextCell, { data: data })
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

/***/ 184:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	var reactor = __webpack_require__(16);
	var Nodes = React.createClass({
	  displayName: 'Nodes',
	
	  render: function render() {
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
	              'table',
	              { className: 'table table-striped' },
	              React.createElement(
	                'thead',
	                null,
	                React.createElement(
	                  'tr',
	                  null,
	                  React.createElement(
	                    'th',
	                    null,
	                    'Node'
	                  ),
	                  React.createElement(
	                    'th',
	                    null,
	                    'Status'
	                  ),
	                  React.createElement(
	                    'th',
	                    null,
	                    'Labels'
	                  ),
	                  React.createElement(
	                    'th',
	                    null,
	                    'CPU'
	                  ),
	                  React.createElement(
	                    'th',
	                    null,
	                    'RAM'
	                  ),
	                  React.createElement(
	                    'th',
	                    null,
	                    'OS'
	                  ),
	                  React.createElement(
	                    'th',
	                    null,
	                    ' Last Heartbeat '
	                  )
	                )
	              ),
	              React.createElement('tbody', null)
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

/***/ 185:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	exports.__esModule = true;
	
	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };
	
	var React = __webpack_require__(5);
	
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
	        return _this2.renderCell(item.props.cell, _extends({ rowIndex: i, key: i, isHeader: false }, item.props));
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
	
	    return React.createElement(
	      'table',
	      { className: 'table table-bordered' },
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
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "table.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 186:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	var render = __webpack_require__(102).render;
	
	var _require = __webpack_require__(42);
	
	var Router = _require.Router;
	var Route = _require.Route;
	var Redirect = _require.Redirect;
	var IndexRoute = _require.IndexRoute;
	var browserHistory = _require.browserHistory;
	
	var _require2 = __webpack_require__(179);
	
	var App = _require2.App;
	var Login = _require2.Login;
	var Nodes = _require2.Nodes;
	var Sessions = _require2.Sessions;
	var NewUser = _require2.NewUser;
	
	var auth = __webpack_require__(82);
	var session = __webpack_require__(38);
	var cfg = __webpack_require__(22);
	
	__webpack_require__(143);
	
	// init session
	session.init();
	
	function requiresAuth(nextState, replace, cb) {
	  auth.ensureUser().done(function () {
	    return cb();
	  }).fail(function () {
	    replace({ redirectTo: nextState.location.pathname }, cfg.routes.login);
	    cb();
	  });
	}
	
	function handleLogout(nextState, replace, cb) {
	  auth.logout();
	  // going back will hit requireAuth handler which will redirect it to the login page
	  session.getHistory().goBack();
	}
	
	render(React.createElement(
	  Router,
	  { history: session.getHistory() },
	  React.createElement(Route, { path: cfg.routes.login, component: Login }),
	  React.createElement(Route, { path: cfg.routes.logout, onEnter: handleLogout }),
	  React.createElement(Route, { path: cfg.routes.newUser, component: NewUser }),
	  React.createElement(
	    Route,
	    { path: cfg.routes.app, component: App, onEnter: requiresAuth },
	    React.createElement(Route, { path: cfg.routes.nodes, component: Nodes }),
	    React.createElement(Route, { path: cfg.routes.sessions, component: Sessions })
	  )
	), document.getElementById("app"));
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "index.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ }

});
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJzb3VyY2VzIjpbIndlYnBhY2s6Ly8vLi9zcmMvYXBwL3JlYWN0b3IuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb25maWcuanMiLCJ3ZWJwYWNrOi8vLy4vfi9rZXltaXJyb3IvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9zZXNzaW9uLmpzIiwid2VicGFjazovLy9leHRlcm5hbCBcImpRdWVyeVwiIiwid2VicGFjazovLy8uL3NyYy9hcHAvc2VydmljZXMvYXBpLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvYXV0aC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2FjdGlvblR5cGVzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvaW52aXRlU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2FjdGlvblR5cGVzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9ub2Rlcy9ub2RlU3RvcmUuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvYWN0aW9uVHlwZXMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvY29uc3RhbnRzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy91c2VyL2FjdGlvblR5cGVzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy91c2VyL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy91c2VyL3VzZXJTdG9yZS5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvZ29vZ2xlQXV0aC5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21tb24vcGF0dGVyblV0aWxzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9pbmRleC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9nZXR0ZXJzLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvaW5kZXguanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2dldHRlcnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2luZGV4LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvcmVzdEFwaVN0b3JlLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvbW9kdWxlcy91c2VyL2FjdGlvbnMuanMiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2FwcC5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL2luZGV4LmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvbG9naW4uanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9uYXZMZWZ0QmFyLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvbmV3VXNlci5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL25vZGVzL21haW4uanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9zZXNzaW9ucy9tYWluLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvdGFibGUuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvaW5kZXguanN4Il0sIm5hbWVzIjpbXSwibWFwcGluZ3MiOiI7Ozs7Ozs7Ozs7Ozs7Ozs7O3NDQUF3QixFQUFZOztBQUVwQyxLQUFNLE9BQU8sR0FBRyx1QkFBWTtBQUMxQixRQUFLLEVBQUUsSUFB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LElBQUMsSUFBSSxFQUFFLEdBQUcsQ0FBQyxNQUFNLENBQUMsT0FBUSxFQUFDLFNBQVMsRUFBRSxPQUFRLEdBQUU7R0FFdEQ7QUFBQyxVQUFLO09BQUMsSUFBSSxFQUFFLEdBQUcsQ0FBQyxNQUFNLENBQUMsR0FBSSxFQUFDLFNBQVMsRUFBRSxHQUFJLEVBQUMsT0FBTyxFQUFFLFlBQWE7S0FDakUsb0JBQUMsS0FBSyxJQUFDLElBQUksRUFBRSxHQUFHLENBQUMsTUFBTSxDQUFDLEtBQU0sRUFBQyxTQUFTLEVBQUUsS0FBTSxHQUFFO0tBQ2xELG9CQUFDLEtBQUssSUFBQyxJQUFJLEVBQUUsR0FBRyxDQUFDLE1BQU0sQ0FBQyxRQUFTLEVBQUMsU0FBUyxFQUFFLFFBQVMsR0FBRTtJQUNsRDtFQUNELEVBQ1IsUUFBUSxDQUFDLGNBQWMsQ0FBQyxLQUFLLENBQUMsQ0FBQyxDIiwiZmlsZSI6ImFwcC5qcyIsInNvdXJjZXNDb250ZW50IjpbImltcG9ydCB7IFJlYWN0b3IgfSBmcm9tICdudWNsZWFyLWpzJ1xuXG5jb25zdCByZWFjdG9yID0gbmV3IFJlYWN0b3Ioe1xuICBkZWJ1ZzogdHJ1ZVxufSlcblxud2luZG93LnJlYWN0b3IgPSByZWFjdG9yO1xuXG5leHBvcnQgZGVmYXVsdCByZWFjdG9yXG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvcmVhY3Rvci5qc1xuICoqLyIsImxldCB7Zm9ybWF0UGF0dGVybn0gPSByZXF1aXJlKCdhcHAvY29tbW9uL3BhdHRlcm5VdGlscycpO1xuXG5sZXQgY2ZnID0ge1xuXG4gIGJhc2VVcmw6IHdpbmRvdy5sb2NhdGlvbi5vcmlnaW4sXG5cbiAgYXBpOiB7XG4gICAgbm9kZXNQYXRoOiAnL25vZGVzJyxcbiAgICBzZXNzaW9uUGF0aDogJy92MS93ZWJhcGkvc2Vzc2lvbnMnLFxuICAgIGludml0ZVBhdGg6ICcvdjEvd2ViYXBpL3VzZXJzL2ludml0ZXMvOmludml0ZVRva2VuJyxcbiAgICBjcmVhdGVVc2VyUGF0aDogJy92MS93ZWJhcGkvdXNlcnMnLFxuICAgIGdldEludml0ZVVybDogKGludml0ZVRva2VuKSA9PiB7XG4gICAgICByZXR1cm4gZm9ybWF0UGF0dGVybihjZmcuYXBpLmludml0ZVBhdGgsIHtpbnZpdGVUb2tlbn0pO1xuICAgIH1cbiAgfSxcblxuICByb3V0ZXM6IHtcbiAgICBhcHA6ICcvd2ViJyxcbiAgICBsb2dvdXQ6ICcvd2ViL2xvZ291dCcsXG4gICAgbG9naW46ICcvd2ViL2xvZ2luJyxcbiAgICBub2RlczogJy93ZWIvbm9kZXMnLFxuICAgIG5ld1VzZXI6ICcvd2ViL25ld3VzZXIvOmludml0ZVRva2VuJyxcbiAgICBzZXNzaW9uczogJy93ZWIvc2Vzc2lvbnMnXG4gIH1cblxufVxuXG5leHBvcnQgZGVmYXVsdCBjZmc7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29uZmlnLmpzXG4gKiovIiwiLyoqXG4gKiBDb3B5cmlnaHQgMjAxMy0yMDE0IEZhY2Vib29rLCBJbmMuXG4gKlxuICogTGljZW5zZWQgdW5kZXIgdGhlIEFwYWNoZSBMaWNlbnNlLCBWZXJzaW9uIDIuMCAodGhlIFwiTGljZW5zZVwiKTtcbiAqIHlvdSBtYXkgbm90IHVzZSB0aGlzIGZpbGUgZXhjZXB0IGluIGNvbXBsaWFuY2Ugd2l0aCB0aGUgTGljZW5zZS5cbiAqIFlvdSBtYXkgb2J0YWluIGEgY29weSBvZiB0aGUgTGljZW5zZSBhdFxuICpcbiAqIGh0dHA6Ly93d3cuYXBhY2hlLm9yZy9saWNlbnNlcy9MSUNFTlNFLTIuMFxuICpcbiAqIFVubGVzcyByZXF1aXJlZCBieSBhcHBsaWNhYmxlIGxhdyBvciBhZ3JlZWQgdG8gaW4gd3JpdGluZywgc29mdHdhcmVcbiAqIGRpc3RyaWJ1dGVkIHVuZGVyIHRoZSBMaWNlbnNlIGlzIGRpc3RyaWJ1dGVkIG9uIGFuIFwiQVMgSVNcIiBCQVNJUyxcbiAqIFdJVEhPVVQgV0FSUkFOVElFUyBPUiBDT05ESVRJT05TIE9GIEFOWSBLSU5ELCBlaXRoZXIgZXhwcmVzcyBvciBpbXBsaWVkLlxuICogU2VlIHRoZSBMaWNlbnNlIGZvciB0aGUgc3BlY2lmaWMgbGFuZ3VhZ2UgZ292ZXJuaW5nIHBlcm1pc3Npb25zIGFuZFxuICogbGltaXRhdGlvbnMgdW5kZXIgdGhlIExpY2Vuc2UuXG4gKlxuICovXG5cblwidXNlIHN0cmljdFwiO1xuXG4vKipcbiAqIENvbnN0cnVjdHMgYW4gZW51bWVyYXRpb24gd2l0aCBrZXlzIGVxdWFsIHRvIHRoZWlyIHZhbHVlLlxuICpcbiAqIEZvciBleGFtcGxlOlxuICpcbiAqICAgdmFyIENPTE9SUyA9IGtleU1pcnJvcih7Ymx1ZTogbnVsbCwgcmVkOiBudWxsfSk7XG4gKiAgIHZhciBteUNvbG9yID0gQ09MT1JTLmJsdWU7XG4gKiAgIHZhciBpc0NvbG9yVmFsaWQgPSAhIUNPTE9SU1tteUNvbG9yXTtcbiAqXG4gKiBUaGUgbGFzdCBsaW5lIGNvdWxkIG5vdCBiZSBwZXJmb3JtZWQgaWYgdGhlIHZhbHVlcyBvZiB0aGUgZ2VuZXJhdGVkIGVudW0gd2VyZVxuICogbm90IGVxdWFsIHRvIHRoZWlyIGtleXMuXG4gKlxuICogICBJbnB1dDogIHtrZXkxOiB2YWwxLCBrZXkyOiB2YWwyfVxuICogICBPdXRwdXQ6IHtrZXkxOiBrZXkxLCBrZXkyOiBrZXkyfVxuICpcbiAqIEBwYXJhbSB7b2JqZWN0fSBvYmpcbiAqIEByZXR1cm4ge29iamVjdH1cbiAqL1xudmFyIGtleU1pcnJvciA9IGZ1bmN0aW9uKG9iaikge1xuICB2YXIgcmV0ID0ge307XG4gIHZhciBrZXk7XG4gIGlmICghKG9iaiBpbnN0YW5jZW9mIE9iamVjdCAmJiAhQXJyYXkuaXNBcnJheShvYmopKSkge1xuICAgIHRocm93IG5ldyBFcnJvcigna2V5TWlycm9yKC4uLik6IEFyZ3VtZW50IG11c3QgYmUgYW4gb2JqZWN0LicpO1xuICB9XG4gIGZvciAoa2V5IGluIG9iaikge1xuICAgIGlmICghb2JqLmhhc093blByb3BlcnR5KGtleSkpIHtcbiAgICAgIGNvbnRpbnVlO1xuICAgIH1cbiAgICByZXRba2V5XSA9IGtleTtcbiAgfVxuICByZXR1cm4gcmV0O1xufTtcblxubW9kdWxlLmV4cG9ydHMgPSBrZXlNaXJyb3I7XG5cblxuXG4vKioqKioqKioqKioqKioqKipcbiAqKiBXRUJQQUNLIEZPT1RFUlxuICoqIC4vfi9rZXltaXJyb3IvaW5kZXguanNcbiAqKiBtb2R1bGUgaWQgPSAzNVxuICoqIG1vZHVsZSBjaHVua3MgPSAxXG4gKiovIiwidmFyIHsgYnJvd3Nlckhpc3RvcnkgfSA9IHJlcXVpcmUoJ3JlYWN0LXJvdXRlcicpO1xuXG5jb25zdCBBVVRIX0tFWV9EQVRBID0gJ2F1dGhEYXRhJztcblxudmFyIF9oaXN0b3J5ID0gbnVsbDtcblxudmFyIHNlc3Npb24gPSB7XG5cbiAgaW5pdChoaXN0b3J5PWJyb3dzZXJIaXN0b3J5KXtcbiAgICBfaGlzdG9yeSA9IGhpc3Rvcnk7XG4gIH0sXG5cbiAgZ2V0SGlzdG9yeSgpe1xuICAgIHJldHVybiBfaGlzdG9yeTtcbiAgfSxcblxuICBzZXRVc2VyRGF0YSh1c2VyRGF0YSl7XG4gICAgc2Vzc2lvblN0b3JhZ2Uuc2V0SXRlbShBVVRIX0tFWV9EQVRBLCBKU09OLnN0cmluZ2lmeSh1c2VyRGF0YSkpO1xuICB9LFxuXG4gIGdldFVzZXJEYXRhKCl7XG4gICAgdmFyIGl0ZW0gPSBzZXNzaW9uU3RvcmFnZS5nZXRJdGVtKEFVVEhfS0VZX0RBVEEpO1xuICAgIGlmKGl0ZW0pe1xuICAgICAgcmV0dXJuIEpTT04ucGFyc2UoaXRlbSk7XG4gICAgfVxuXG4gICAgcmV0dXJuIHt9O1xuICB9LFxuXG4gIGNsZWFyKCl7XG4gICAgc2Vzc2lvblN0b3JhZ2UuY2xlYXIoKVxuICB9XG5cbn1cblxubW9kdWxlLmV4cG9ydHMgPSBzZXNzaW9uO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL3Nlc3Npb24uanNcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cyA9IGpRdWVyeTtcblxuXG4vKioqKioqKioqKioqKioqKipcbiAqKiBXRUJQQUNLIEZPT1RFUlxuICoqIGV4dGVybmFsIFwialF1ZXJ5XCJcbiAqKiBtb2R1bGUgaWQgPSA1MVxuICoqIG1vZHVsZSBjaHVua3MgPSAxXG4gKiovIiwidmFyICQgPSByZXF1aXJlKFwialF1ZXJ5XCIpO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCdhcHAvc2Vzc2lvbicpO1xuXG5jb25zdCBhcGkgPSB7XG5cbiAgcG9zdChwYXRoLCBkYXRhKXtcbiAgICByZXR1cm4gYXBpLmFqYXgoe3VybDogcGF0aCwgZGF0YTogSlNPTi5zdHJpbmdpZnkoZGF0YSksIHR5cGU6ICdQT1NUJ30sIGZhbHNlKTtcbiAgfSxcblxuICBnZXQocGF0aCl7XG4gICAgcmV0dXJuIGFwaS5hamF4KHt1cmw6IHBhdGh9KTtcbiAgfSxcblxuICBhamF4KGNmZywgd2l0aFRva2VuID0gdHJ1ZSl7XG4gICAgdmFyIGRlZmF1bHRDZmcgPSB7XG4gICAgICB0eXBlOiBcIkdFVFwiLFxuICAgICAgZGF0YVR5cGU6IFwianNvblwiLFxuICAgICAgYmVmb3JlU2VuZDogZnVuY3Rpb24oeGhyKSB7XG4gICAgICAgIGlmKHdpdGhUb2tlbil7XG4gICAgICAgICAgdmFyIHsgdG9rZW4gfSA9IHNlc3Npb24uZ2V0VXNlckRhdGEoKTtcbiAgICAgICAgICB4aHIuc2V0UmVxdWVzdEhlYWRlcignQXV0aG9yaXphdGlvbicsJ0JlYXJlciAnICsgdG9rZW4pO1xuICAgICAgICB9XG4gICAgICAgfVxuICAgIH1cblxuICAgIHJldHVybiAkLmFqYXgoJC5leHRlbmQoe30sIGRlZmF1bHRDZmcsIGNmZykpO1xuICB9XG59XG5cbm1vZHVsZS5leHBvcnRzID0gYXBpO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL3NlcnZpY2VzL2FwaS5qc1xuICoqLyIsInZhciBhcGkgPSByZXF1aXJlKCcuL3NlcnZpY2VzL2FwaScpO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCcuL3Nlc3Npb24nKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG52YXIgJCA9IHJlcXVpcmUoJ2pRdWVyeScpO1xuXG5jb25zdCByZWZyZXNoUmF0ZSA9IDYwMDAwICogMTsgLy8gMSBtaW5cblxudmFyIHJlZnJlc2hUb2tlblRpbWVySWQgPSBudWxsO1xuXG52YXIgYXV0aCA9IHtcblxuICBzaWduVXAobmFtZSwgcGFzc3dvcmQsIHRva2VuLCBpbnZpdGVUb2tlbil7XG4gICAgdmFyIGRhdGEgPSB7dXNlcjogbmFtZSwgcGFzczogcGFzc3dvcmQsIHNlY29uZF9mYWN0b3JfdG9rZW46IHRva2VuLCBpbnZpdGVfdG9rZW46IGludml0ZVRva2VufTtcbiAgICByZXR1cm4gYXBpLnBvc3QoY2ZnLmFwaS5jcmVhdGVVc2VyUGF0aCwgZGF0YSlcbiAgICAgIC50aGVuKCh1c2VyKT0+e1xuICAgICAgICBzZXNzaW9uLnNldFVzZXJEYXRhKHVzZXIpO1xuICAgICAgICBhdXRoLl9zdGFydFRva2VuUmVmcmVzaGVyKCk7XG4gICAgICAgIHJldHVybiB1c2VyO1xuICAgICAgfSk7XG4gIH0sXG5cbiAgbG9naW4obmFtZSwgcGFzc3dvcmQsIHRva2VuKXtcbiAgICBhdXRoLl9zdG9wVG9rZW5SZWZyZXNoZXIoKTtcbiAgICByZXR1cm4gYXV0aC5fbG9naW4obmFtZSwgcGFzc3dvcmQsIHRva2VuKS5kb25lKGF1dGguX3N0YXJ0VG9rZW5SZWZyZXNoZXIpO1xuICB9LFxuXG4gIGVuc3VyZVVzZXIoKXtcbiAgICBpZihzZXNzaW9uLmdldFVzZXJEYXRhKCkpe1xuICAgICAgLy8gcmVmcmVzaCB0aW1lciB3aWxsIG5vdCBiZSBzZXQgaW4gY2FzZSBvZiBicm93c2VyIHJlZnJlc2ggZXZlbnRcbiAgICAgIGlmKGF1dGguX2dldFJlZnJlc2hUb2tlblRpbWVySWQoKSA9PT0gbnVsbCl7XG4gICAgICAgIHJldHVybiBhdXRoLl9sb2dpbigpLmRvbmUoYXV0aC5fc3RhcnRUb2tlblJlZnJlc2hlcik7XG4gICAgICB9XG5cbiAgICAgIHJldHVybiAkLkRlZmVycmVkKCkucmVzb2x2ZSgpO1xuICAgIH1cblxuICAgIHJldHVybiAkLkRlZmVycmVkKCkucmVqZWN0KCk7XG4gIH0sXG5cbiAgbG9nb3V0KCl7XG4gICAgYXV0aC5fc3RvcFRva2VuUmVmcmVzaGVyKCk7XG4gICAgcmV0dXJuIHNlc3Npb24uY2xlYXIoKTtcbiAgfSxcblxuICBfc3RhcnRUb2tlblJlZnJlc2hlcigpe1xuICAgIHJlZnJlc2hUb2tlblRpbWVySWQgPSBzZXRJbnRlcnZhbChhdXRoLl9yZWZyZXNoVG9rZW4sIHJlZnJlc2hSYXRlKTtcbiAgfSxcblxuICBfc3RvcFRva2VuUmVmcmVzaGVyKCl7XG4gICAgY2xlYXJJbnRlcnZhbChyZWZyZXNoVG9rZW5UaW1lcklkKTtcbiAgICByZWZyZXNoVG9rZW5UaW1lcklkID0gbnVsbDtcbiAgfSxcblxuICBfZ2V0UmVmcmVzaFRva2VuVGltZXJJZCgpe1xuICAgIHJldHVybiByZWZyZXNoVG9rZW5UaW1lcklkO1xuICB9LFxuXG4gIF9yZWZyZXNoVG9rZW4oKXtcbiAgICBhdXRoLl9sb2dpbigpLmZhaWwoKCk9PntcbiAgICAgIGF1dGgubG9nb3V0KCk7XG4gICAgICB3aW5kb3cubG9jYXRpb24ucmVsb2FkKCk7XG4gICAgfSlcbiAgfSxcblxuICBfbG9naW4obmFtZSwgcGFzc3dvcmQsIHRva2VuKXtcbiAgICB2YXIgZGF0YSA9IHtcbiAgICAgIHVzZXI6IG5hbWUsXG4gICAgICBwYXNzOiBwYXNzd29yZCxcbiAgICAgIHNlY29uZF9mYWN0b3JfdG9rZW46IHRva2VuXG4gICAgfTtcblxuICAgIHJldHVybiBhcGkucG9zdChjZmcuYXBpLnNlc3Npb25QYXRoLCBkYXRhKS50aGVuKGRhdGE9PntcbiAgICAgIHNlc3Npb24uc2V0VXNlckRhdGEoZGF0YSk7XG4gICAgICByZXR1cm4gZGF0YTtcbiAgICB9KTtcblxuICB9XG59XG5cbm1vZHVsZS5leHBvcnRzID0gYXV0aDtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9hdXRoLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfUkVDRUlWRV9VU0VSX0lOVklURTogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9hY3Rpb25UeXBlcy5qc1xuICoqLyIsInZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyICB7IFRMUFRfUkVDRUlWRV9VU0VSX0lOVklURSB9ID0gcmVxdWlyZSgnLi9hY3Rpb25UeXBlcycpO1xuXG5leHBvcnQgZGVmYXVsdCBTdG9yZSh7XG4gIGdldEluaXRpYWxTdGF0ZSgpIHtcbiAgICByZXR1cm4gdG9JbW11dGFibGUobnVsbCk7XG4gIH0sXG5cbiAgaW5pdGlhbGl6ZSgpIHtcbiAgICB0aGlzLm9uKFRMUFRfUkVDRUlWRV9VU0VSX0lOVklURSwgcmVjZWl2ZUludml0ZSlcbiAgfVxufSlcblxuZnVuY3Rpb24gcmVjZWl2ZUludml0ZShzdGF0ZSwgaW52aXRlKXtcbiAgcmV0dXJuIHRvSW1tdXRhYmxlKGludml0ZSk7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvaW52aXRlU3RvcmUuanNcbiAqKi8iLCJpbXBvcnQga2V5TWlycm9yIGZyb20gJ2tleW1pcnJvcidcblxuZXhwb3J0IGRlZmF1bHQga2V5TWlycm9yKHtcbiAgVExQVF9SRUNFSVZFX05PREVTOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvYWN0aW9uVHlwZXMuanNcbiAqKi8iLCJ2YXIgeyBTdG9yZSwgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcbnZhciAgeyBUTFBUX1JFQ0VJVkVfTk9ERVMgfSA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcblxuZXhwb3J0IGRlZmF1bHQgU3RvcmUoe1xuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHRvSW1tdXRhYmxlKHt9KTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9SRUNFSVZFX05PREVTLCByZWNlaXZlTm9kZXMpXG4gIH1cbn0pXG5cbmZ1bmN0aW9uIHJlY2VpdmVOb2RlcyhzdGF0ZSwgbm9kZUFycmF5RGF0YSl7XG4gIHJldHVybiBzdGF0ZS53aXRoTXV0YXRpb25zKHN0YXRlID0+IHtcbiAgICBub2RlQXJyYXlEYXRhLmZvckVhY2goKGl0ZW0pID0+IHtcbiAgICAgICAgc3RhdGUuc2V0KGl0ZW0uaWQsIHRvSW1tdXRhYmxlKGl0ZW0pKVxuICAgICAgfSlcbiAgIH0pO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvbm9kZVN0b3JlLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfUkVTVF9BUElfU1RBUlQ6IG51bGwsXG4gIFRMUFRfUkVTVF9BUElfU1VDQ0VTUzogbnVsbCxcbiAgVExQVF9SRVNUX0FQSV9GQUlMOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvcmVzdEFwaS9hY3Rpb25UeXBlcy5qc1xuICoqLyIsImltcG9ydCBrZXlNaXJyb3IgZnJvbSAna2V5bWlycm9yJ1xuXG5leHBvcnQgZGVmYXVsdCBrZXlNaXJyb3Ioe1xuICBUUllJTkdfVE9fU0lHTl9VUDogbnVsbFxufSlcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL3Jlc3RBcGkvY29uc3RhbnRzLmpzXG4gKiovIiwiaW1wb3J0IGtleU1pcnJvciBmcm9tICdrZXltaXJyb3InXG5cbmV4cG9ydCBkZWZhdWx0IGtleU1pcnJvcih7XG4gIFRMUFRfUkVDRUlWRV9VU0VSOiBudWxsXG59KVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvdXNlci9hY3Rpb25UeXBlcy5qc1xuICoqLyIsIm1vZHVsZS5leHBvcnRzLmdldHRlcnMgPSByZXF1aXJlKCcuL2dldHRlcnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGlvbnMgPSByZXF1aXJlKCcuL2FjdGlvbnMnKTtcbm1vZHVsZS5leHBvcnRzLm5vZGVTdG9yZSA9IHJlcXVpcmUoJy4vdXNlclN0b3JlJyk7XG5cbi8vIG5vZGVzOiBbe1wiaWRcIjpcIngyMjBcIixcImFkZHJcIjpcIjAuMC4wLjA6MzAyMlwiLFwiaG9zdG5hbWVcIjpcIngyMjBcIixcImxhYmVsc1wiOm51bGwsXCJjbWRfbGFiZWxzXCI6bnVsbH1dXG5cblxuLy8gc2Vzc2lvbnM6IFt7XCJpZFwiOlwiMDc2MzA2MzYtYmIzZC00MGUxLWIwODYtNjBiMmNhZTIxYWM0XCIsXCJwYXJ0aWVzXCI6W3tcImlkXCI6XCI4OWY3NjJhMy03NDI5LTRjN2EtYTkxMy03NjY0OTNmZTdjOGFcIixcInNpdGVcIjpcIjEyNy4wLjAuMTozNzUxNFwiLFwidXNlclwiOlwiYWtvbnRzZXZveVwiLFwic2VydmVyX2FkZHJcIjpcIjAuMC4wLjA6MzAyMlwiLFwibGFzdF9hY3RpdmVcIjpcIjIwMTYtMDItMjJUMTQ6Mzk6MjAuOTMxMjA1MzUtMDU6MDBcIn1dfV1cblxuLypcbmxldCBUb2RvUmVjb3JkID0gSW1tdXRhYmxlLlJlY29yZCh7XG4gICAgaWQ6IDAsXG4gICAgZGVzY3JpcHRpb246IFwiXCIsXG4gICAgY29tcGxldGVkOiBmYWxzZVxufSk7XG4qL1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvdXNlci9pbmRleC5qc1xuICoqLyIsInZhciB7IFN0b3JlLCB0b0ltbXV0YWJsZSB9ID0gcmVxdWlyZSgnbnVjbGVhci1qcycpO1xudmFyICB7IFRMUFRfUkVDRUlWRV9VU0VSIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IFN0b3JlKHtcbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB0b0ltbXV0YWJsZShudWxsKTtcbiAgfSxcblxuICBpbml0aWFsaXplKCkge1xuICAgIHRoaXMub24oVExQVF9SRUNFSVZFX1VTRVIsIHJlY2VpdmVVc2VyKVxuICB9XG5cbn0pXG5cbmZ1bmN0aW9uIHJlY2VpdmVVc2VyKHN0YXRlLCB1c2VyKXtcbiAgcmV0dXJuIHRvSW1tdXRhYmxlKHVzZXIpO1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvdXNlci91c2VyU3RvcmUuanNcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xuXG52YXIgR29vZ2xlQXV0aEluZm8gPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIHJlbmRlcigpIHtcbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZ29vZ2xlLWF1dGhcIj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtZ29vZ2xlLWF1dGgtaWNvblwiPjwvZGl2PlxuICAgICAgICA8c3Ryb25nPkdvb2dsZSBBdXRoZW50aWNhdG9yPC9zdHJvbmc+XG4gICAgICAgIDxkaXY+RG93bmxvYWQgPGEgaHJlZj1cImh0dHBzOi8vc3VwcG9ydC5nb29nbGUuY29tL2FjY291bnRzL2Fuc3dlci8xMDY2NDQ3P2hsPWVuXCI+R29vZ2xlIEF1dGhlbnRpY2F0b3I8L2E+IG9uIHlvdXIgcGhvbmUgdG8gYWNjZXNzIHlvdXIgdHdvIGZhY3RvcnkgdG9rZW48L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pXG5cbm1vZHVsZS5leHBvcnRzID0gR29vZ2xlQXV0aEluZm87XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9nb29nbGVBdXRoLmpzeFxuICoqLyIsIi8qXG4gKiAgVGhlIE1JVCBMaWNlbnNlIChNSVQpXG4gKiAgQ29weXJpZ2h0IChjKSAyMDE1IFJ5YW4gRmxvcmVuY2UsIE1pY2hhZWwgSmFja3NvblxuICogIFBlcm1pc3Npb24gaXMgaGVyZWJ5IGdyYW50ZWQsIGZyZWUgb2YgY2hhcmdlLCB0byBhbnkgcGVyc29uIG9idGFpbmluZyBhIGNvcHkgb2YgdGhpcyBzb2Z0d2FyZSBhbmQgYXNzb2NpYXRlZCBkb2N1bWVudGF0aW9uIGZpbGVzICh0aGUgXCJTb2Z0d2FyZVwiKSwgdG8gZGVhbCBpbiB0aGUgU29mdHdhcmUgd2l0aG91dCByZXN0cmljdGlvbiwgaW5jbHVkaW5nIHdpdGhvdXQgbGltaXRhdGlvbiB0aGUgcmlnaHRzIHRvIHVzZSwgY29weSwgbW9kaWZ5LCBtZXJnZSwgcHVibGlzaCwgZGlzdHJpYnV0ZSwgc3VibGljZW5zZSwgYW5kL29yIHNlbGwgY29waWVzIG9mIHRoZSBTb2Z0d2FyZSwgYW5kIHRvIHBlcm1pdCBwZXJzb25zIHRvIHdob20gdGhlIFNvZnR3YXJlIGlzIGZ1cm5pc2hlZCB0byBkbyBzbywgc3ViamVjdCB0byB0aGUgZm9sbG93aW5nIGNvbmRpdGlvbnM6XG4gKiAgVGhlIGFib3ZlIGNvcHlyaWdodCBub3RpY2UgYW5kIHRoaXMgcGVybWlzc2lvbiBub3RpY2Ugc2hhbGwgYmUgaW5jbHVkZWQgaW4gYWxsIGNvcGllcyBvciBzdWJzdGFudGlhbCBwb3J0aW9ucyBvZiB0aGUgU29mdHdhcmUuXG4gKiAgVEhFIFNPRlRXQVJFIElTIFBST1ZJREVEIFwiQVMgSVNcIiwgV0lUSE9VVCBXQVJSQU5UWSBPRiBBTlkgS0lORCwgRVhQUkVTUyBPUiBJTVBMSUVELCBJTkNMVURJTkcgQlVUIE5PVCBMSU1JVEVEIFRPIFRIRSBXQVJSQU5USUVTIE9GIE1FUkNIQU5UQUJJTElUWSwgRklUTkVTUyBGT1IgQSBQQVJUSUNVTEFSIFBVUlBPU0UgQU5EIE5PTklORlJJTkdFTUVOVC4gSU4gTk8gRVZFTlQgU0hBTEwgVEhFIEFVVEhPUlMgT1IgQ09QWVJJR0hUIEhPTERFUlMgQkUgTElBQkxFIEZPUiBBTlkgQ0xBSU0sIERBTUFHRVMgT1IgT1RIRVIgTElBQklMSVRZLCBXSEVUSEVSIElOIEFOIEFDVElPTiBPRiBDT05UUkFDVCwgVE9SVCBPUiBPVEhFUldJU0UsIEFSSVNJTkcgRlJPTSwgT1VUIE9GIE9SIElOIENPTk5FQ1RJT04gV0lUSCBUSEUgU09GVFdBUkUgT1IgVEhFIFVTRSBPUiBPVEhFUiBERUFMSU5HUyBJTiBUSEUgU09GVFdBUkUuXG4qL1xuXG5pbXBvcnQgaW52YXJpYW50IGZyb20gJ2ludmFyaWFudCdcblxuZnVuY3Rpb24gZXNjYXBlUmVnRXhwKHN0cmluZykge1xuICByZXR1cm4gc3RyaW5nLnJlcGxhY2UoL1suKis/XiR7fSgpfFtcXF1cXFxcXS9nLCAnXFxcXCQmJylcbn1cblxuZnVuY3Rpb24gZXNjYXBlU291cmNlKHN0cmluZykge1xuICByZXR1cm4gZXNjYXBlUmVnRXhwKHN0cmluZykucmVwbGFjZSgvXFwvKy9nLCAnLysnKVxufVxuXG5mdW5jdGlvbiBfY29tcGlsZVBhdHRlcm4ocGF0dGVybikge1xuICBsZXQgcmVnZXhwU291cmNlID0gJyc7XG4gIGNvbnN0IHBhcmFtTmFtZXMgPSBbXTtcbiAgY29uc3QgdG9rZW5zID0gW107XG5cbiAgbGV0IG1hdGNoLCBsYXN0SW5kZXggPSAwLCBtYXRjaGVyID0gLzooW2EtekEtWl8kXVthLXpBLVowLTlfJF0qKXxcXCpcXCp8XFwqfFxcKHxcXCkvZ1xuICAvKmVzbGludCBuby1jb25kLWFzc2lnbjogMCovXG4gIHdoaWxlICgobWF0Y2ggPSBtYXRjaGVyLmV4ZWMocGF0dGVybikpKSB7XG4gICAgaWYgKG1hdGNoLmluZGV4ICE9PSBsYXN0SW5kZXgpIHtcbiAgICAgIHRva2Vucy5wdXNoKHBhdHRlcm4uc2xpY2UobGFzdEluZGV4LCBtYXRjaC5pbmRleCkpXG4gICAgICByZWdleHBTb3VyY2UgKz0gZXNjYXBlU291cmNlKHBhdHRlcm4uc2xpY2UobGFzdEluZGV4LCBtYXRjaC5pbmRleCkpXG4gICAgfVxuXG4gICAgaWYgKG1hdGNoWzFdKSB7XG4gICAgICByZWdleHBTb3VyY2UgKz0gJyhbXi8/I10rKSc7XG4gICAgICBwYXJhbU5hbWVzLnB1c2gobWF0Y2hbMV0pO1xuICAgIH0gZWxzZSBpZiAobWF0Y2hbMF0gPT09ICcqKicpIHtcbiAgICAgIHJlZ2V4cFNvdXJjZSArPSAnKFtcXFxcc1xcXFxTXSopJ1xuICAgICAgcGFyYW1OYW1lcy5wdXNoKCdzcGxhdCcpO1xuICAgIH0gZWxzZSBpZiAobWF0Y2hbMF0gPT09ICcqJykge1xuICAgICAgcmVnZXhwU291cmNlICs9ICcoW1xcXFxzXFxcXFNdKj8pJ1xuICAgICAgcGFyYW1OYW1lcy5wdXNoKCdzcGxhdCcpO1xuICAgIH0gZWxzZSBpZiAobWF0Y2hbMF0gPT09ICcoJykge1xuICAgICAgcmVnZXhwU291cmNlICs9ICcoPzonO1xuICAgIH0gZWxzZSBpZiAobWF0Y2hbMF0gPT09ICcpJykge1xuICAgICAgcmVnZXhwU291cmNlICs9ICcpPyc7XG4gICAgfVxuXG4gICAgdG9rZW5zLnB1c2gobWF0Y2hbMF0pO1xuXG4gICAgbGFzdEluZGV4ID0gbWF0Y2hlci5sYXN0SW5kZXg7XG4gIH1cblxuICBpZiAobGFzdEluZGV4ICE9PSBwYXR0ZXJuLmxlbmd0aCkge1xuICAgIHRva2Vucy5wdXNoKHBhdHRlcm4uc2xpY2UobGFzdEluZGV4LCBwYXR0ZXJuLmxlbmd0aCkpXG4gICAgcmVnZXhwU291cmNlICs9IGVzY2FwZVNvdXJjZShwYXR0ZXJuLnNsaWNlKGxhc3RJbmRleCwgcGF0dGVybi5sZW5ndGgpKVxuICB9XG5cbiAgcmV0dXJuIHtcbiAgICBwYXR0ZXJuLFxuICAgIHJlZ2V4cFNvdXJjZSxcbiAgICBwYXJhbU5hbWVzLFxuICAgIHRva2Vuc1xuICB9XG59XG5cbmNvbnN0IENvbXBpbGVkUGF0dGVybnNDYWNoZSA9IHt9XG5cbmV4cG9ydCBmdW5jdGlvbiBjb21waWxlUGF0dGVybihwYXR0ZXJuKSB7XG4gIGlmICghKHBhdHRlcm4gaW4gQ29tcGlsZWRQYXR0ZXJuc0NhY2hlKSlcbiAgICBDb21waWxlZFBhdHRlcm5zQ2FjaGVbcGF0dGVybl0gPSBfY29tcGlsZVBhdHRlcm4ocGF0dGVybilcblxuICByZXR1cm4gQ29tcGlsZWRQYXR0ZXJuc0NhY2hlW3BhdHRlcm5dXG59XG5cbi8qKlxuICogQXR0ZW1wdHMgdG8gbWF0Y2ggYSBwYXR0ZXJuIG9uIHRoZSBnaXZlbiBwYXRobmFtZS4gUGF0dGVybnMgbWF5IHVzZVxuICogdGhlIGZvbGxvd2luZyBzcGVjaWFsIGNoYXJhY3RlcnM6XG4gKlxuICogLSA6cGFyYW1OYW1lICAgICBNYXRjaGVzIGEgVVJMIHNlZ21lbnQgdXAgdG8gdGhlIG5leHQgLywgPywgb3IgIy4gVGhlXG4gKiAgICAgICAgICAgICAgICAgIGNhcHR1cmVkIHN0cmluZyBpcyBjb25zaWRlcmVkIGEgXCJwYXJhbVwiXG4gKiAtICgpICAgICAgICAgICAgIFdyYXBzIGEgc2VnbWVudCBvZiB0aGUgVVJMIHRoYXQgaXMgb3B0aW9uYWxcbiAqIC0gKiAgICAgICAgICAgICAgQ29uc3VtZXMgKG5vbi1ncmVlZHkpIGFsbCBjaGFyYWN0ZXJzIHVwIHRvIHRoZSBuZXh0XG4gKiAgICAgICAgICAgICAgICAgIGNoYXJhY3RlciBpbiB0aGUgcGF0dGVybiwgb3IgdG8gdGhlIGVuZCBvZiB0aGUgVVJMIGlmXG4gKiAgICAgICAgICAgICAgICAgIHRoZXJlIGlzIG5vbmVcbiAqIC0gKiogICAgICAgICAgICAgQ29uc3VtZXMgKGdyZWVkeSkgYWxsIGNoYXJhY3RlcnMgdXAgdG8gdGhlIG5leHQgY2hhcmFjdGVyXG4gKiAgICAgICAgICAgICAgICAgIGluIHRoZSBwYXR0ZXJuLCBvciB0byB0aGUgZW5kIG9mIHRoZSBVUkwgaWYgdGhlcmUgaXMgbm9uZVxuICpcbiAqIFRoZSByZXR1cm4gdmFsdWUgaXMgYW4gb2JqZWN0IHdpdGggdGhlIGZvbGxvd2luZyBwcm9wZXJ0aWVzOlxuICpcbiAqIC0gcmVtYWluaW5nUGF0aG5hbWVcbiAqIC0gcGFyYW1OYW1lc1xuICogLSBwYXJhbVZhbHVlc1xuICovXG5leHBvcnQgZnVuY3Rpb24gbWF0Y2hQYXR0ZXJuKHBhdHRlcm4sIHBhdGhuYW1lKSB7XG4gIC8vIE1ha2UgbGVhZGluZyBzbGFzaGVzIGNvbnNpc3RlbnQgYmV0d2VlbiBwYXR0ZXJuIGFuZCBwYXRobmFtZS5cbiAgaWYgKHBhdHRlcm4uY2hhckF0KDApICE9PSAnLycpIHtcbiAgICBwYXR0ZXJuID0gYC8ke3BhdHRlcm59YFxuICB9XG4gIGlmIChwYXRobmFtZS5jaGFyQXQoMCkgIT09ICcvJykge1xuICAgIHBhdGhuYW1lID0gYC8ke3BhdGhuYW1lfWBcbiAgfVxuXG4gIGxldCB7IHJlZ2V4cFNvdXJjZSwgcGFyYW1OYW1lcywgdG9rZW5zIH0gPSBjb21waWxlUGF0dGVybihwYXR0ZXJuKVxuXG4gIHJlZ2V4cFNvdXJjZSArPSAnLyonIC8vIENhcHR1cmUgcGF0aCBzZXBhcmF0b3JzXG5cbiAgLy8gU3BlY2lhbC1jYXNlIHBhdHRlcm5zIGxpa2UgJyonIGZvciBjYXRjaC1hbGwgcm91dGVzLlxuICBjb25zdCBjYXB0dXJlUmVtYWluaW5nID0gdG9rZW5zW3Rva2Vucy5sZW5ndGggLSAxXSAhPT0gJyonXG5cbiAgaWYgKGNhcHR1cmVSZW1haW5pbmcpIHtcbiAgICAvLyBUaGlzIHdpbGwgbWF0Y2ggbmV3bGluZXMgaW4gdGhlIHJlbWFpbmluZyBwYXRoLlxuICAgIHJlZ2V4cFNvdXJjZSArPSAnKFtcXFxcc1xcXFxTXSo/KSdcbiAgfVxuXG4gIGNvbnN0IG1hdGNoID0gcGF0aG5hbWUubWF0Y2gobmV3IFJlZ0V4cCgnXicgKyByZWdleHBTb3VyY2UgKyAnJCcsICdpJykpXG5cbiAgbGV0IHJlbWFpbmluZ1BhdGhuYW1lLCBwYXJhbVZhbHVlc1xuICBpZiAobWF0Y2ggIT0gbnVsbCkge1xuICAgIGlmIChjYXB0dXJlUmVtYWluaW5nKSB7XG4gICAgICByZW1haW5pbmdQYXRobmFtZSA9IG1hdGNoLnBvcCgpXG4gICAgICBjb25zdCBtYXRjaGVkUGF0aCA9XG4gICAgICAgIG1hdGNoWzBdLnN1YnN0cigwLCBtYXRjaFswXS5sZW5ndGggLSByZW1haW5pbmdQYXRobmFtZS5sZW5ndGgpXG5cbiAgICAgIC8vIElmIHdlIGRpZG4ndCBtYXRjaCB0aGUgZW50aXJlIHBhdGhuYW1lLCB0aGVuIG1ha2Ugc3VyZSB0aGF0IHRoZSBtYXRjaFxuICAgICAgLy8gd2UgZGlkIGdldCBlbmRzIGF0IGEgcGF0aCBzZXBhcmF0b3IgKHBvdGVudGlhbGx5IHRoZSBvbmUgd2UgYWRkZWRcbiAgICAgIC8vIGFib3ZlIGF0IHRoZSBiZWdpbm5pbmcgb2YgdGhlIHBhdGgsIGlmIHRoZSBhY3R1YWwgbWF0Y2ggd2FzIGVtcHR5KS5cbiAgICAgIGlmIChcbiAgICAgICAgcmVtYWluaW5nUGF0aG5hbWUgJiZcbiAgICAgICAgbWF0Y2hlZFBhdGguY2hhckF0KG1hdGNoZWRQYXRoLmxlbmd0aCAtIDEpICE9PSAnLydcbiAgICAgICkge1xuICAgICAgICByZXR1cm4ge1xuICAgICAgICAgIHJlbWFpbmluZ1BhdGhuYW1lOiBudWxsLFxuICAgICAgICAgIHBhcmFtTmFtZXMsXG4gICAgICAgICAgcGFyYW1WYWx1ZXM6IG51bGxcbiAgICAgICAgfVxuICAgICAgfVxuICAgIH0gZWxzZSB7XG4gICAgICAvLyBJZiB0aGlzIG1hdGNoZWQgYXQgYWxsLCB0aGVuIHRoZSBtYXRjaCB3YXMgdGhlIGVudGlyZSBwYXRobmFtZS5cbiAgICAgIHJlbWFpbmluZ1BhdGhuYW1lID0gJydcbiAgICB9XG5cbiAgICBwYXJhbVZhbHVlcyA9IG1hdGNoLnNsaWNlKDEpLm1hcChcbiAgICAgIHYgPT4gdiAhPSBudWxsID8gZGVjb2RlVVJJQ29tcG9uZW50KHYpIDogdlxuICAgIClcbiAgfSBlbHNlIHtcbiAgICByZW1haW5pbmdQYXRobmFtZSA9IHBhcmFtVmFsdWVzID0gbnVsbFxuICB9XG5cbiAgcmV0dXJuIHtcbiAgICByZW1haW5pbmdQYXRobmFtZSxcbiAgICBwYXJhbU5hbWVzLFxuICAgIHBhcmFtVmFsdWVzXG4gIH1cbn1cblxuZXhwb3J0IGZ1bmN0aW9uIGdldFBhcmFtTmFtZXMocGF0dGVybikge1xuICByZXR1cm4gY29tcGlsZVBhdHRlcm4ocGF0dGVybikucGFyYW1OYW1lc1xufVxuXG5leHBvcnQgZnVuY3Rpb24gZ2V0UGFyYW1zKHBhdHRlcm4sIHBhdGhuYW1lKSB7XG4gIGNvbnN0IHsgcGFyYW1OYW1lcywgcGFyYW1WYWx1ZXMgfSA9IG1hdGNoUGF0dGVybihwYXR0ZXJuLCBwYXRobmFtZSlcblxuICBpZiAocGFyYW1WYWx1ZXMgIT0gbnVsbCkge1xuICAgIHJldHVybiBwYXJhbU5hbWVzLnJlZHVjZShmdW5jdGlvbiAobWVtbywgcGFyYW1OYW1lLCBpbmRleCkge1xuICAgICAgbWVtb1twYXJhbU5hbWVdID0gcGFyYW1WYWx1ZXNbaW5kZXhdXG4gICAgICByZXR1cm4gbWVtb1xuICAgIH0sIHt9KVxuICB9XG5cbiAgcmV0dXJuIG51bGxcbn1cblxuLyoqXG4gKiBSZXR1cm5zIGEgdmVyc2lvbiBvZiB0aGUgZ2l2ZW4gcGF0dGVybiB3aXRoIHBhcmFtcyBpbnRlcnBvbGF0ZWQuIFRocm93c1xuICogaWYgdGhlcmUgaXMgYSBkeW5hbWljIHNlZ21lbnQgb2YgdGhlIHBhdHRlcm4gZm9yIHdoaWNoIHRoZXJlIGlzIG5vIHBhcmFtLlxuICovXG5leHBvcnQgZnVuY3Rpb24gZm9ybWF0UGF0dGVybihwYXR0ZXJuLCBwYXJhbXMpIHtcbiAgcGFyYW1zID0gcGFyYW1zIHx8IHt9XG5cbiAgY29uc3QgeyB0b2tlbnMgfSA9IGNvbXBpbGVQYXR0ZXJuKHBhdHRlcm4pXG4gIGxldCBwYXJlbkNvdW50ID0gMCwgcGF0aG5hbWUgPSAnJywgc3BsYXRJbmRleCA9IDBcblxuICBsZXQgdG9rZW4sIHBhcmFtTmFtZSwgcGFyYW1WYWx1ZVxuICBmb3IgKGxldCBpID0gMCwgbGVuID0gdG9rZW5zLmxlbmd0aDsgaSA8IGxlbjsgKytpKSB7XG4gICAgdG9rZW4gPSB0b2tlbnNbaV1cblxuICAgIGlmICh0b2tlbiA9PT0gJyonIHx8IHRva2VuID09PSAnKionKSB7XG4gICAgICBwYXJhbVZhbHVlID0gQXJyYXkuaXNBcnJheShwYXJhbXMuc3BsYXQpID8gcGFyYW1zLnNwbGF0W3NwbGF0SW5kZXgrK10gOiBwYXJhbXMuc3BsYXRcblxuICAgICAgaW52YXJpYW50KFxuICAgICAgICBwYXJhbVZhbHVlICE9IG51bGwgfHwgcGFyZW5Db3VudCA+IDAsXG4gICAgICAgICdNaXNzaW5nIHNwbGF0ICMlcyBmb3IgcGF0aCBcIiVzXCInLFxuICAgICAgICBzcGxhdEluZGV4LCBwYXR0ZXJuXG4gICAgICApXG5cbiAgICAgIGlmIChwYXJhbVZhbHVlICE9IG51bGwpXG4gICAgICAgIHBhdGhuYW1lICs9IGVuY29kZVVSSShwYXJhbVZhbHVlKVxuICAgIH0gZWxzZSBpZiAodG9rZW4gPT09ICcoJykge1xuICAgICAgcGFyZW5Db3VudCArPSAxXG4gICAgfSBlbHNlIGlmICh0b2tlbiA9PT0gJyknKSB7XG4gICAgICBwYXJlbkNvdW50IC09IDFcbiAgICB9IGVsc2UgaWYgKHRva2VuLmNoYXJBdCgwKSA9PT0gJzonKSB7XG4gICAgICBwYXJhbU5hbWUgPSB0b2tlbi5zdWJzdHJpbmcoMSlcbiAgICAgIHBhcmFtVmFsdWUgPSBwYXJhbXNbcGFyYW1OYW1lXVxuXG4gICAgICBpbnZhcmlhbnQoXG4gICAgICAgIHBhcmFtVmFsdWUgIT0gbnVsbCB8fCBwYXJlbkNvdW50ID4gMCxcbiAgICAgICAgJ01pc3NpbmcgXCIlc1wiIHBhcmFtZXRlciBmb3IgcGF0aCBcIiVzXCInLFxuICAgICAgICBwYXJhbU5hbWUsIHBhdHRlcm5cbiAgICAgIClcblxuICAgICAgaWYgKHBhcmFtVmFsdWUgIT0gbnVsbClcbiAgICAgICAgcGF0aG5hbWUgKz0gZW5jb2RlVVJJQ29tcG9uZW50KHBhcmFtVmFsdWUpXG4gICAgfSBlbHNlIHtcbiAgICAgIHBhdGhuYW1lICs9IHRva2VuXG4gICAgfVxuICB9XG5cbiAgcmV0dXJuIHBhdGhuYW1lLnJlcGxhY2UoL1xcLysvZywgJy8nKVxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbW1vbi9wYXR0ZXJuVXRpbHMuanNcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG5yZWFjdG9yLnJlZ2lzdGVyU3RvcmVzKHtcbiAgJ3RscHRfdXNlcic6IHJlcXVpcmUoJy4vdXNlci91c2VyU3RvcmUnKSxcbiAgJ3RscHRfbm9kZXMnOiByZXF1aXJlKCcuL25vZGVzL25vZGVTdG9yZScpLFxuICAndGxwdF9pbnZpdGUnOiByZXF1aXJlKCcuL2ludml0ZS9pbnZpdGVTdG9yZScpLFxuICAndGxwdF9yZXN0X2FwaSc6IHJlcXVpcmUoJy4vcmVzdEFwaS9yZXN0QXBpU3RvcmUnKVxufSk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9pbmRleC5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7IFRMUFRfUkVDRUlWRV9VU0VSX0lOVklURSB9ICA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcbnZhciBhcGkgPSByZXF1aXJlKCdhcHAvc2VydmljZXMvYXBpJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xuXG5leHBvcnQgZGVmYXVsdCB7XG4gIGZldGNoSW52aXRlKGludml0ZVRva2VuKXtcbiAgICB2YXIgcGF0aCA9IGNmZy5hcGkuZ2V0SW52aXRlVXJsKGludml0ZVRva2VuKTtcbiAgICBhcGkuZ2V0KHBhdGgpLmRvbmUoaW52aXRlPT57XG4gICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfUkVDRUlWRV9VU0VSX0lOVklURSwgaW52aXRlKTtcbiAgICB9KTtcbiAgfVxufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvaW52aXRlL2FjdGlvbnMuanNcbiAqKi8iLCIvKmVzbGludCBuby11bmRlZjogMCwgIG5vLXVudXNlZC12YXJzOiAwLCBuby1kZWJ1Z2dlcjowKi9cblxudmFyIHtUUllJTkdfVE9fU0lHTl9VUH0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9yZXN0QXBpL2NvbnN0YW50cycpO1xuXG5jb25zdCBpbnZpdGUgPSBbIFsndGxwdF9pbnZpdGUnXSwgKGludml0ZSkgPT4ge1xuICByZXR1cm4gaW52aXRlO1xuIH1cbl07XG5cbmNvbnN0IGF0dGVtcCA9IFsgWyd0bHB0X3Jlc3RfYXBpJywgVFJZSU5HX1RPX1NJR05fVVBdLCAoYXR0ZW1wKSA9PiB7XG4gIHZhciBkZWZhdWx0T2JqID0ge1xuICAgIGlzUHJvY2Vzc2luZzogZmFsc2UsXG4gICAgaXNFcnJvcjogZmFsc2UsXG4gICAgaXNTdWNjZXNzOiBmYWxzZSxcbiAgICBtZXNzYWdlOiAnJ1xuICB9XG5cbiAgcmV0dXJuIGF0dGVtcCA/IGF0dGVtcC50b0pTKCkgOiBkZWZhdWx0T2JqO1xuICBcbiB9XG5dO1xuXG5leHBvcnQgZGVmYXVsdCB7XG4gIGludml0ZSxcbiAgYXR0ZW1wXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9pbnZpdGUvZ2V0dGVycy5qc1xuICoqLyIsIm1vZHVsZS5leHBvcnRzLmdldHRlcnMgPSByZXF1aXJlKCcuL2dldHRlcnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGlvbnMgPSByZXF1aXJlKCcuL2FjdGlvbnMnKTtcbm1vZHVsZS5leHBvcnRzLm5vZGVTdG9yZSA9IHJlcXVpcmUoJy4vaW52aXRlU3RvcmUnKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL2ludml0ZS9pbmRleC5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7IFRMUFRfUkVDRUlWRV9OT0RFUyB9ICA9IHJlcXVpcmUoJy4vYWN0aW9uVHlwZXMnKTtcbnZhciBhcGkgPSByZXF1aXJlKCdhcHAvc2VydmljZXMvYXBpJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xuXG5leHBvcnQgZGVmYXVsdCB7XG4gIGZldGNoTm9kZXMoKXtcbiAgICBhcGkuZ2V0KGNmZy5hcGkubm9kZXMpLmRvbmUobm9kZXM9PntcbiAgICAgIHJlYWN0b3IuZGlzcGF0Y2goVExQVF9SRUNFSVZFX05PREVTLCBub2Rlcyk7XG4gICAgfSk7XG4gIH1cbn1cblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9tb2R1bGVzL25vZGVzL2FjdGlvbnMuanNcbiAqKi8iLCIvL3ZhciBzb3J0ID0gcmVxdWlyZSgnYXBwL2NvbW1vbi9zb3J0Jyk7XG5cbmNvbnN0IG5vZGVMaXN0VmlldyA9IFsgWyd0bHB0X25vZGVzJ10sIChub2RlcykgPT57XG4gICAgcmV0dXJuIG5vZGVzLnZhbHVlU2VxKCkubWFwKChpdGVtKT0+e1xuICAgICAgcmV0dXJuIHtcbiAgICAgICAgY291bnQ6IGl0ZW0uZ2V0KCdjb3VudCcpLFxuICAgICAgICBpcDogaXRlbS5nZXQoJ2lwJyksXG4gICAgICAgIHRhZ3M6IFsndGFnMScsICd0YWcyJywgJ3RhZzMnXSxcbiAgICAgICAgcm9sZXM6IFsncjEnLCAncjInLCAncjMnXVxuICAgICAgfVxuICAgIH0pLnRvSlMoKTtcbiB9XG5dO1xuXG5leHBvcnQgZGVmYXVsdCB7XG4gIG5vZGVMaXN0Vmlld1xufVxuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvZ2V0dGVycy5qc1xuICoqLyIsIm1vZHVsZS5leHBvcnRzLmdldHRlcnMgPSByZXF1aXJlKCcuL2dldHRlcnMnKTtcbm1vZHVsZS5leHBvcnRzLmFjdGlvbnMgPSByZXF1aXJlKCcuL2FjdGlvbnMnKTtcbm1vZHVsZS5leHBvcnRzLm5vZGVTdG9yZSA9IHJlcXVpcmUoJy4vbm9kZVN0b3JlJyk7XG5cbi8vIG5vZGVzOiBbe1wiaWRcIjpcIngyMjBcIixcImFkZHJcIjpcIjAuMC4wLjA6MzAyMlwiLFwiaG9zdG5hbWVcIjpcIngyMjBcIixcImxhYmVsc1wiOm51bGwsXCJjbWRfbGFiZWxzXCI6bnVsbH1dXG5cblxuLy8gc2Vzc2lvbnM6IFt7XCJpZFwiOlwiMDc2MzA2MzYtYmIzZC00MGUxLWIwODYtNjBiMmNhZTIxYWM0XCIsXCJwYXJ0aWVzXCI6W3tcImlkXCI6XCI4OWY3NjJhMy03NDI5LTRjN2EtYTkxMy03NjY0OTNmZTdjOGFcIixcInNpdGVcIjpcIjEyNy4wLjAuMTozNzUxNFwiLFwidXNlclwiOlwiYWtvbnRzZXZveVwiLFwic2VydmVyX2FkZHJcIjpcIjAuMC4wLjA6MzAyMlwiLFwibGFzdF9hY3RpdmVcIjpcIjIwMTYtMDItMjJUMTQ6Mzk6MjAuOTMxMjA1MzUtMDU6MDBcIn1dfV1cblxuLypcbmxldCBUb2RvUmVjb3JkID0gSW1tdXRhYmxlLlJlY29yZCh7XG4gICAgaWQ6IDAsXG4gICAgZGVzY3JpcHRpb246IFwiXCIsXG4gICAgY29tcGxldGVkOiBmYWxzZVxufSk7XG4qL1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL21vZHVsZXMvbm9kZXMvaW5kZXguanNcbiAqKi8iLCJ2YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG5cbnZhciB7XG4gIFRMUFRfUkVTVF9BUElfU1RBUlQsXG4gIFRMUFRfUkVTVF9BUElfU1VDQ0VTUyxcbiAgVExQVF9SRVNUX0FQSV9GQUlMIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IHtcblxuICBzdGFydChyZXFUeXBlKXtcbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfUkVTVF9BUElfU1RBUlQsIHt0eXBlOiByZXFUeXBlfSk7XG4gIH0sXG5cbiAgZmFpbChyZXFUeXBlLCBtZXNzYWdlKXtcbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfUkVTVF9BUElfRkFJTCwgIHt0eXBlOiByZXFUeXBlLCBtZXNzYWdlfSk7XG4gIH0sXG5cbiAgc3VjY2VzcyhyZXFUeXBlKXtcbiAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfUkVTVF9BUElfU1VDQ0VTUywge3R5cGU6IHJlcVR5cGV9KTtcbiAgfVxuXG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL2FjdGlvbnMuanNcbiAqKi8iLCJ2YXIgeyBTdG9yZSwgdG9JbW11dGFibGUgfSA9IHJlcXVpcmUoJ251Y2xlYXItanMnKTtcbnZhciB7XG4gIFRMUFRfUkVTVF9BUElfU1RBUlQsXG4gIFRMUFRfUkVTVF9BUElfU1VDQ0VTUyxcbiAgVExQVF9SRVNUX0FQSV9GQUlMIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG5cbmV4cG9ydCBkZWZhdWx0IFN0b3JlKHtcbiAgZ2V0SW5pdGlhbFN0YXRlKCkge1xuICAgIHJldHVybiB0b0ltbXV0YWJsZSh7fSk7XG4gIH0sXG5cbiAgaW5pdGlhbGl6ZSgpIHtcbiAgICB0aGlzLm9uKFRMUFRfUkVTVF9BUElfU1RBUlQsIHN0YXJ0KTtcbiAgICB0aGlzLm9uKFRMUFRfUkVTVF9BUElfRkFJTCwgZmFpbCk7XG4gICAgdGhpcy5vbihUTFBUX1JFU1RfQVBJX1NVQ0NFU1MsIHN1Y2Nlc3MpO1xuICB9XG59KVxuXG5mdW5jdGlvbiBzdGFydChzdGF0ZSwgcmVxdWVzdCl7XG4gIHJldHVybiBzdGF0ZS5zZXQocmVxdWVzdC50eXBlLCB0b0ltbXV0YWJsZSh7aXNQcm9jZXNzaW5nOiB0cnVlfSkpO1xufVxuXG5mdW5jdGlvbiBmYWlsKHN0YXRlLCByZXF1ZXN0KXtcbiAgcmV0dXJuIHN0YXRlLnNldChyZXF1ZXN0LnR5cGUsIHRvSW1tdXRhYmxlKHtpc0ZhaWxlZDogdHJ1ZSwgbWVzc2FnZTogcmVxdWVzdC5tZXNzYWdlfSkpO1xufVxuXG5mdW5jdGlvbiBzdWNjZXNzKHN0YXRlLCByZXF1ZXN0KXtcbiAgcmV0dXJuIHN0YXRlLnNldChyZXF1ZXN0LnR5cGUsIHRvSW1tdXRhYmxlKHtpc1N1Y2Nlc3M6IHRydWV9KSk7XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy9yZXN0QXBpL3Jlc3RBcGlTdG9yZS5qc1xuICoqLyIsInZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7IFRMUFRfUkVDRUlWRV9VU0VSIH0gPSByZXF1aXJlKCcuL2FjdGlvblR5cGVzJyk7XG52YXIgeyBUUllJTkdfVE9fU0lHTl9VUH0gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9yZXN0QXBpL2NvbnN0YW50cycpO1xudmFyIHJlc3RBcGlBY3Rpb25zID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvcmVzdEFwaS9hY3Rpb25zJyk7XG52YXIgYXV0aCA9IHJlcXVpcmUoJ2FwcC9hdXRoJyk7XG52YXIgc2Vzc2lvbiA9IHJlcXVpcmUoJ2FwcC9zZXNzaW9uJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xuXG5leHBvcnQgZGVmYXVsdCB7XG5cbiAgc2lnblVwKHtuYW1lLCBwc3csIHRva2VuLCBpbnZpdGVUb2tlbn0pe1xuICAgIHJlc3RBcGlBY3Rpb25zLnN0YXJ0KFRSWUlOR19UT19TSUdOX1VQKTtcbiAgICBhdXRoLnNpZ25VcChuYW1lLCBwc3csIHRva2VuLCBpbnZpdGVUb2tlbilcbiAgICAgIC5kb25lKCh1c2VyKT0+e1xuICAgICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfUkVDRUlWRV9VU0VSLCB1c2VyKTtcbiAgICAgICAgcmVzdEFwaUFjdGlvbnMuc3VjY2VzcyhUUllJTkdfVE9fU0lHTl9VUCk7XG4gICAgICAgIHNlc3Npb24uZ2V0SGlzdG9yeSgpLnB1c2goe3BhdGhuYW1lOiBjZmcucm91dGVzLmFwcH0pO1xuICAgICAgfSlcbiAgICAgIC5mYWlsKCgpPT57XG4gICAgICAgIHJlc3RBcGlBY3Rpb25zLmZhaWwoVFJZSU5HX1RPX1NJR05fVVAsICdmYWlsZWQgdG8gc2luZyB1cCcpO1xuICAgICAgfSk7XG4gIH0sXG5cbiAgbG9naW4oe3VzZXIsIHBhc3N3b3JkLCB0b2tlbn0sIHJlZGlyZWN0KXtcbiAgICAgIGF1dGgubG9naW4odXNlciwgcGFzc3dvcmQsIHRva2VuKVxuICAgICAgICAuZG9uZSgodXNlcik9PntcbiAgICAgICAgICByZWFjdG9yLmRpc3BhdGNoKFRMUFRfUkVDRUlWRV9VU0VSLCB1c2VyKTtcbiAgICAgICAgICBzZXNzaW9uLmdldEhpc3RvcnkoKS5wdXNoKHtwYXRobmFtZTogcmVkaXJlY3R9KTtcbiAgICAgICAgfSlcbiAgICAgICAgLmZhaWwoKCk9PntcbiAgICAgICAgfSlcbiAgICB9XG59XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvbW9kdWxlcy91c2VyL2FjdGlvbnMuanNcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyIE5hdkxlZnRCYXIgPSByZXF1aXJlKCcuL25hdkxlZnRCYXInKTtcbnZhciBjZmcgPSByZXF1aXJlKCdhcHAvY29uZmlnJyk7XG5cbnZhciBBcHAgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2XCI+XG4gICAgICAgIDxOYXZMZWZ0QmFyLz5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJyb3dcIiBzdHlsZT17e21hcmdpbkxlZnQ6IFwiNzBweFwifX0+XG4gICAgICAgICAgPG5hdiBjbGFzc05hbWU9XCJcIiByb2xlPVwibmF2aWdhdGlvblwiIHN0eWxlPXt7IG1hcmdpbkJvdHRvbTogMCB9fT5cbiAgICAgICAgICAgIDx1bCBjbGFzc05hbWU9XCJuYXYgbmF2YmFyLXRvcC1saW5rcyBuYXZiYXItcmlnaHRcIj5cbiAgICAgICAgICAgICAgPGxpPlxuICAgICAgICAgICAgICAgIDxzcGFuIGNsYXNzTmFtZT1cIm0tci1zbSB0ZXh0LW11dGVkIHdlbGNvbWUtbWVzc2FnZVwiPlxuICAgICAgICAgICAgICAgICAgV2VsY29tZSB0byBHcmF2aXRhdGlvbmFsIFBvcnRhbFxuICAgICAgICAgICAgICAgIDwvc3Bhbj5cbiAgICAgICAgICAgICAgPC9saT5cbiAgICAgICAgICAgICAgPGxpPlxuICAgICAgICAgICAgICAgIDxhIGhyZWY9e2NmZy5yb3V0ZXMubG9nb3V0fT5cbiAgICAgICAgICAgICAgICAgIDxpIGNsYXNzTmFtZT1cImZhIGZhLXNpZ24tb3V0XCI+PC9pPlxuICAgICAgICAgICAgICAgICAgTG9nIG91dFxuICAgICAgICAgICAgICAgIDwvYT5cbiAgICAgICAgICAgICAgPC9saT5cbiAgICAgICAgICAgIDwvdWw+XG4gICAgICAgICAgPC9uYXY+XG4gICAgICAgIDwvZGl2PlxuICAgICAgICA8ZGl2IHN0eWxlPXt7ICdtYXJnaW5MZWZ0JzogJzEwMHB4JyB9fT5cbiAgICAgICAgICB7dGhpcy5wcm9wcy5jaGlsZHJlbn1cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KVxuXG5tb2R1bGUuZXhwb3J0cyA9IEFwcDtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2FwcC5qc3hcbiAqKi8iLCJtb2R1bGUuZXhwb3J0cy5BcHAgPSByZXF1aXJlKCcuL2FwcC5qc3gnKTtcbm1vZHVsZS5leHBvcnRzLkxvZ2luID0gcmVxdWlyZSgnLi9sb2dpbi5qc3gnKTtcbm1vZHVsZS5leHBvcnRzLk5ld1VzZXIgPSByZXF1aXJlKCcuL25ld1VzZXIuanN4Jyk7XG5tb2R1bGUuZXhwb3J0cy5Ob2RlcyA9IHJlcXVpcmUoJy4vbm9kZXMvbWFpbi5qc3gnKTtcbm1vZHVsZS5leHBvcnRzLlNlc3Npb25zID0gcmVxdWlyZSgnLi9zZXNzaW9ucy9tYWluLmpzeCcpO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvaW5kZXguanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciAkID0gcmVxdWlyZSgnalF1ZXJ5Jyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgTGlua2VkU3RhdGVNaXhpbiA9IHJlcXVpcmUoJ3JlYWN0LWFkZG9ucy1saW5rZWQtc3RhdGUtbWl4aW4nKTtcbnZhciB7YWN0aW9uc30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy91c2VyJyk7XG52YXIgR29vZ2xlQXV0aEluZm8gPSByZXF1aXJlKCcuL2dvb2dsZUF1dGgnKTtcbnZhciBMb2dpbklucHV0Rm9ybSA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBtaXhpbnM6IFtMaW5rZWRTdGF0ZU1peGluXSxcblxuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIHVzZXI6ICcnLFxuICAgICAgcGFzc3dvcmQ6ICcnLFxuICAgICAgdG9rZW46ICcnXG4gICAgfVxuICB9LFxuXG4gIG9uQ2xpY2s6IGZ1bmN0aW9uKGUpIHtcbiAgICBlLnByZXZlbnREZWZhdWx0KCk7XG4gICAgaWYgKHRoaXMuaXNWYWxpZCgpKSB7XG4gICAgICBhY3Rpb25zLmxvZ2luKHsgLi4udGhpcy5zdGF0ZX0sICcvd2ViJyk7XG4gICAgfVxuICB9LFxuXG4gIGlzVmFsaWQ6IGZ1bmN0aW9uKCkge1xuICAgIHZhciAkZm9ybSA9ICQodGhpcy5yZWZzLmZvcm0pO1xuICAgIHJldHVybiAkZm9ybS5sZW5ndGggPT09IDAgfHwgJGZvcm0udmFsaWQoKTtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuIChcbiAgICAgIDxmb3JtIHJlZj1cImZvcm1cIiBjbGFzc05hbWU9XCJncnYtbG9naW4taW5wdXQtZm9ybVwiPlxuICAgICAgICA8aDM+IFdlbGNvbWUgdG8gVGVsZXBvcnQgPC9oMz5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImZvcm0tZ3JvdXBcIj5cbiAgICAgICAgICAgIDxpbnB1dCB2YWx1ZUxpbms9e3RoaXMubGlua1N0YXRlKCd1c2VyJyl9IGNsYXNzTmFtZT1cImZvcm0tY29udHJvbCByZXF1aXJlZFwiIHBsYWNlaG9sZGVyPVwiVXNlciBuYW1lXCIgbmFtZT1cInVzZXJOYW1lXCIgLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImZvcm0tZ3JvdXBcIj5cbiAgICAgICAgICAgIDxpbnB1dCB2YWx1ZUxpbms9e3RoaXMubGlua1N0YXRlKCdwYXNzd29yZCcpfSB0eXBlPVwicGFzc3dvcmRcIiBuYW1lPVwicGFzc3dvcmRcIiBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2wgcmVxdWlyZWRcIiBwbGFjZWhvbGRlcj1cIlBhc3N3b3JkXCIvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgPGlucHV0IHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3Rva2VuJyl9IGNsYXNzTmFtZT1cImZvcm0tY29udHJvbCByZXF1aXJlZFwiIG5hbWU9XCJ0b2tlblwiIHBsYWNlaG9sZGVyPVwiVHdvIGZhY3RvciB0b2tlbiAoR29vZ2xlIEF1dGhlbnRpY2F0b3IpXCIvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxidXR0b24gdHlwZT1cInN1Ym1pdFwiIGNsYXNzTmFtZT1cImJ0biBidG4tcHJpbWFyeSBibG9jayBmdWxsLXdpZHRoIG0tYlwiIG9uQ2xpY2s9e3RoaXMub25DbGlja30+TG9naW48L2J1dHRvbj5cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Zvcm0+XG4gICAgKTtcbiAgfVxufSlcblxudmFyIExvZ2luID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuXG4gIG1peGluczogW3JlYWN0b3IuUmVhY3RNaXhpbl0sXG5cbiAgZ2V0RGF0YUJpbmRpbmdzKCkge1xuICAgIHJldHVybiB7XG4gIC8vICAgIHVzZXJSZXF1ZXN0OiBnZXR0ZXJzLnVzZXJSZXF1ZXN0XG4gICAgfVxuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgdmFyIGlzUHJvY2Vzc2luZyA9IGZhbHNlOy8vdGhpcy5zdGF0ZS51c2VyUmVxdWVzdC5nZXQoJ2lzTG9hZGluZycpO1xuICAgIHZhciBpc0Vycm9yID0gZmFsc2U7Ly90aGlzLnN0YXRlLnVzZXJSZXF1ZXN0LmdldCgnaXNFcnJvcicpO1xuXG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2IGdydi1sb2dpbiB0ZXh0LWNlbnRlclwiPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1sb2dvLXRwcnRcIj48L2Rpdj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtY29udGVudCBncnYtZmxleFwiPlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWZsZXgtY29sdW1uXCI+XG4gICAgICAgICAgICA8TG9naW5JbnB1dEZvcm0vPlxuICAgICAgICAgICAgPEdvb2dsZUF1dGhJbmZvLz5cbiAgICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWxvZ2luLWluZm9cIj5cbiAgICAgICAgICAgICAgPGkgY2xhc3NOYW1lPVwiZmEgZmEtcXVlc3Rpb25cIj48L2k+XG4gICAgICAgICAgICAgIDxzdHJvbmc+TmV3IEFjY291bnQgb3IgZm9yZ290IHBhc3N3b3JkPzwvc3Ryb25nPlxuICAgICAgICAgICAgICA8ZGl2PkFzayBmb3IgYXNzaXN0YW5jZSBmcm9tIHlvdXIgQ29tcGFueSBhZG1pbmlzdHJhdG9yPC9kaXY+XG4gICAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBMb2dpbjtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL2xvZ2luLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgeyBSb3V0ZXIsIEluZGV4TGluaywgSGlzdG9yeSB9ID0gcmVxdWlyZSgncmVhY3Qtcm91dGVyJyk7XG52YXIgY2ZnID0gcmVxdWlyZSgnYXBwL2NvbmZpZycpO1xuXG52YXIgbWVudUl0ZW1zID0gW1xuICB7aWNvbjogJ2ZhIGZhIGZhLXNpdGVtYXAnLCB0bzogY2ZnLnJvdXRlcy5ub2RlcywgdGl0bGU6ICdOb2Rlcyd9LFxuICB7aWNvbjogJ2ZhIGZhLWhkZC1vJywgdG86IGNmZy5yb3V0ZXMuc2Vzc2lvbnMsIHRpdGxlOiAnU2Vzc2lvbnMnfVxuXTtcblxudmFyIE5hdkxlZnRCYXIgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpe1xuICAgIHZhciBpdGVtcyA9IG1lbnVJdGVtcy5tYXAoKGksIGluZGV4KT0+e1xuICAgICAgdmFyIGNsYXNzTmFtZSA9IHRoaXMuY29udGV4dC5yb3V0ZXIuaXNBY3RpdmUoaS50bykgPyAnYWN0aXZlJyA6ICcnO1xuICAgICAgcmV0dXJuIChcbiAgICAgICAgPGxpIGtleT17aW5kZXh9IGNsYXNzTmFtZT17Y2xhc3NOYW1lfT5cbiAgICAgICAgICA8SW5kZXhMaW5rIHRvPXtpLnRvfT5cbiAgICAgICAgICAgIDxpIGNsYXNzTmFtZT17aS5pY29ufSB0aXRsZT17aS50aXRsZX0vPlxuICAgICAgICAgIDwvSW5kZXhMaW5rPlxuICAgICAgICA8L2xpPlxuICAgICAgKTtcbiAgICB9KTtcblxuICAgIHJldHVybiAoXG4gICAgICA8bmF2IGNsYXNzTmFtZT0nJyByb2xlPSduYXZpZ2F0aW9uJyBzdHlsZT17e3dpZHRoOiAnNjBweCcsIGZsb2F0OiAnbGVmdCcsIHBvc2l0aW9uOiAnYWJzb2x1dGUnfX0+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPScnPlxuICAgICAgICAgIDx1bCBjbGFzc05hbWU9J25hdiAxbWV0aXNtZW51JyBpZD0nc2lkZS1tZW51Jz5cbiAgICAgICAgICAgIHtpdGVtc31cbiAgICAgICAgICA8L3VsPlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvbmF2PlxuICAgICk7XG4gIH1cbn0pO1xuXG5OYXZMZWZ0QmFyLmNvbnRleHRUeXBlcyA9IHtcbiAgcm91dGVyOiBSZWFjdC5Qcm9wVHlwZXMub2JqZWN0LmlzUmVxdWlyZWRcbn1cblxubW9kdWxlLmV4cG9ydHMgPSBOYXZMZWZ0QmFyO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvbmF2TGVmdEJhci5qc3hcbiAqKi8iLCJ2YXIgUmVhY3QgPSByZXF1aXJlKCdyZWFjdCcpO1xudmFyICQgPSByZXF1aXJlKCdqUXVlcnknKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciB7YWN0aW9ucywgZ2V0dGVyc30gPSByZXF1aXJlKCdhcHAvbW9kdWxlcy9pbnZpdGUnKTtcbnZhciB1c2VyTW9kdWxlID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvdXNlcicpO1xudmFyIExpbmtlZFN0YXRlTWl4aW4gPSByZXF1aXJlKCdyZWFjdC1hZGRvbnMtbGlua2VkLXN0YXRlLW1peGluJyk7XG52YXIgR29vZ2xlQXV0aEluZm8gPSByZXF1aXJlKCcuL2dvb2dsZUF1dGgnKTtcblxudmFyIEludml0ZUlucHV0Rm9ybSA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBtaXhpbnM6IFtMaW5rZWRTdGF0ZU1peGluXSxcblxuICBjb21wb25lbnREaWRNb3VudCgpe1xuICAgICQodGhpcy5yZWZzLmZvcm0pLnZhbGlkYXRlKHtcbiAgICAgIHJ1bGVzOntcbiAgICAgICAgcGFzc3dvcmQ6e1xuICAgICAgICAgIG1pbmxlbmd0aDogNSxcbiAgICAgICAgICByZXF1aXJlZDogdHJ1ZVxuICAgICAgICB9LFxuICAgICAgICBwYXNzd29yZENvbmZpcm1lZDp7XG4gICAgICAgICAgcmVxdWlyZWQ6IHRydWUsXG4gICAgICAgICAgZXF1YWxUbzogdGhpcy5yZWZzLnBhc3N3b3JkXG4gICAgICAgIH1cbiAgICAgIH0sXG5cbiAgICAgIG1lc3NhZ2VzOiB7XG4gIFx0XHRcdHBhc3N3b3JkQ29uZmlybWVkOiB7XG4gIFx0XHRcdFx0bWlubGVuZ3RoOiAkLnZhbGlkYXRvci5mb3JtYXQoJ0VudGVyIGF0IGxlYXN0IHswfSBjaGFyYWN0ZXJzJyksXG4gIFx0XHRcdFx0ZXF1YWxUbzogJ0VudGVyIHRoZSBzYW1lIHBhc3N3b3JkIGFzIGFib3ZlJ1xuICBcdFx0XHR9XG4gICAgICB9XG4gICAgfSlcbiAgfSxcblxuICBnZXRJbml0aWFsU3RhdGUoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIG5hbWU6IHRoaXMucHJvcHMuaW52aXRlLnVzZXIsXG4gICAgICBwc3c6ICcnLFxuICAgICAgcHN3Q29uZmlybWVkOiAnJyxcbiAgICAgIHRva2VuOiAnJ1xuICAgIH1cbiAgfSxcblxuICBvbkNsaWNrKGUpIHtcbiAgICBlLnByZXZlbnREZWZhdWx0KCk7XG4gICAgaWYgKHRoaXMuaXNWYWxpZCgpKSB7XG4gICAgICB1c2VyTW9kdWxlLmFjdGlvbnMuc2lnblVwKHtcbiAgICAgICAgbmFtZTogdGhpcy5zdGF0ZS5uYW1lLFxuICAgICAgICBwc3c6IHRoaXMuc3RhdGUucHN3LFxuICAgICAgICB0b2tlbjogdGhpcy5zdGF0ZS50b2tlbixcbiAgICAgICAgaW52aXRlVG9rZW46IHRoaXMucHJvcHMuaW52aXRlLmludml0ZV90b2tlbn0pO1xuICAgIH1cbiAgfSxcblxuICBpc1ZhbGlkKCkge1xuICAgIHZhciAkZm9ybSA9ICQodGhpcy5yZWZzLmZvcm0pO1xuICAgIHJldHVybiAkZm9ybS5sZW5ndGggPT09IDAgfHwgJGZvcm0udmFsaWQoKTtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgcmV0dXJuIChcbiAgICAgIDxmb3JtIHJlZj1cImZvcm1cIiBjbGFzc05hbWU9XCJncnYtaW52aXRlLWlucHV0LWZvcm1cIj5cbiAgICAgICAgPGgzPiBHZXQgc3RhcnRlZCB3aXRoIFRlbGVwb3J0IDwvaDM+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiXCI+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXRcbiAgICAgICAgICAgICAgdmFsdWVMaW5rPXt0aGlzLmxpbmtTdGF0ZSgnbmFtZScpfVxuICAgICAgICAgICAgICBuYW1lPVwidXNlck5hbWVcIlxuICAgICAgICAgICAgICBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2wgcmVxdWlyZWRcIlxuICAgICAgICAgICAgICBwbGFjZWhvbGRlcj1cIlVzZXIgbmFtZVwiLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImZvcm0tZ3JvdXBcIj5cbiAgICAgICAgICAgIDxpbnB1dFxuICAgICAgICAgICAgICB2YWx1ZUxpbms9e3RoaXMubGlua1N0YXRlKCdwc3cnKX1cbiAgICAgICAgICAgICAgcmVmPVwicGFzc3dvcmRcIlxuICAgICAgICAgICAgICB0eXBlPVwicGFzc3dvcmRcIlxuICAgICAgICAgICAgICBuYW1lPVwicGFzc3dvcmRcIlxuICAgICAgICAgICAgICBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2xcIlxuICAgICAgICAgICAgICBwbGFjZWhvbGRlcj1cIlBhc3N3b3JkXCIgLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImZvcm0tZ3JvdXAgZ3J2LVwiPlxuICAgICAgICAgICAgPGlucHV0XG4gICAgICAgICAgICAgIHZhbHVlTGluaz17dGhpcy5saW5rU3RhdGUoJ3Bzd0NvbmZpcm1lZCcpfVxuICAgICAgICAgICAgICB0eXBlPVwicGFzc3dvcmRcIlxuICAgICAgICAgICAgICBuYW1lPVwicGFzc3dvcmRDb25maXJtZWRcIlxuICAgICAgICAgICAgICBjbGFzc05hbWU9XCJmb3JtLWNvbnRyb2xcIlxuICAgICAgICAgICAgICBwbGFjZWhvbGRlcj1cIlBhc3N3b3JkIGNvbmZpcm1cIi8+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICA8aW5wdXRcbiAgICAgICAgICAgICAgbmFtZT1cInRva2VuXCJcbiAgICAgICAgICAgICAgdmFsdWVMaW5rPXt0aGlzLmxpbmtTdGF0ZSgndG9rZW4nKX1cbiAgICAgICAgICAgICAgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sIHJlcXVpcmVkXCJcbiAgICAgICAgICAgICAgcGxhY2Vob2xkZXI9XCJUd28gZmFjdG9yIHRva2VuIChHb29nbGUgQXV0aGVudGljYXRvcilcIiAvPlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDxidXR0b24gdHlwZT1cInN1Ym1pdFwiIGRpc2FibGVkPXt0aGlzLnByb3BzLmF0dGVtcC5pc1Byb2Nlc3Npbmd9IGNsYXNzTmFtZT1cImJ0biBidG4tcHJpbWFyeSBibG9jayBmdWxsLXdpZHRoIG0tYlwiIG9uQ2xpY2s9e3RoaXMub25DbGlja30gPlNpZ24gdXA8L2J1dHRvbj5cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Zvcm0+XG4gICAgKTtcbiAgfVxufSlcblxudmFyIEludml0ZSA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBtaXhpbnM6IFtyZWFjdG9yLlJlYWN0TWl4aW5dLFxuXG4gIGdldERhdGFCaW5kaW5ncygpIHtcbiAgICByZXR1cm4ge1xuICAgICAgaW52aXRlOiBnZXR0ZXJzLmludml0ZSxcbiAgICAgIGF0dGVtcDogZ2V0dGVycy5hdHRlbXBcbiAgICB9XG4gIH0sXG5cbiAgY29tcG9uZW50RGlkTW91bnQoKXtcbiAgICBhY3Rpb25zLmZldGNoSW52aXRlKHRoaXMucHJvcHMucGFyYW1zLmludml0ZVRva2VuKTtcbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIGlmKCF0aGlzLnN0YXRlLmludml0ZSkge1xuICAgICAgcmV0dXJuIG51bGw7XG4gICAgfVxuXG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2IGdydi1pbnZpdGUgdGV4dC1jZW50ZXJcIj5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJncnYtbG9nby10cHJ0XCI+PC9kaXY+XG4gICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2LWNvbnRlbnQgZ3J2LWZsZXhcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LWNvbHVtblwiPlxuICAgICAgICAgICAgPEludml0ZUlucHV0Rm9ybSBhdHRlbXA9e3RoaXMuc3RhdGUuYXR0ZW1wfSBpbnZpdGU9e3RoaXMuc3RhdGUuaW52aXRlLnRvSlMoKX0vPlxuICAgICAgICAgICAgPEdvb2dsZUF1dGhJbmZvLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImdydi1mbGV4LWNvbHVtblwiPlxuICAgICAgICAgICAgPGg0PlNjYW4gYmFyIGNvZGUgZm9yIGF1dGggdG9rZW4gPGJyLz4gPHNtYWxsPlNjYW4gYmVsb3cgdG8gZ2VuZXJhdGUgeW91ciB0d28gZmFjdG9yIHRva2VuPC9zbWFsbD48L2g0PlxuICAgICAgICAgICAgPGltZyBjbGFzc05hbWU9XCJpbWctdGh1bWJuYWlsXCIgc3JjPXsgYGRhdGE6aW1hZ2UvcG5nO2Jhc2U2NCwke3RoaXMuc3RhdGUuaW52aXRlLmdldCgncXInKX1gIH0gLz5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgPC9kaXY+XG4gICAgICA8L2Rpdj5cbiAgICApO1xuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBJbnZpdGU7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9uZXdVc2VyLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIge2dldHRlcnMsIGFjdGlvbnN9ID0gcmVxdWlyZSgnYXBwL21vZHVsZXMvbm9kZXMnKTtcbnZhciB7VGFibGUsIENvbHVtbiwgQ2VsbH0gPSByZXF1aXJlKCdhcHAvY29tcG9uZW50cy90YWJsZS5qc3gnKTtcblxuY29uc3QgVGV4dENlbGwgPSAoe3Jvd0luZGV4LCBkYXRhLCBjb2x1bW5LZXksIC4uLnByb3BzfSkgPT4gKFxuICA8Q2VsbCB7Li4ucHJvcHN9PlxuICAgIHtkYXRhW3Jvd0luZGV4XVtjb2x1bW5LZXldfVxuICA8L0NlbGw+XG4pO1xuXG52YXIgTm9kZXMgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgICAgIG5vZGVSZWNvcmRzOiBnZXR0ZXJzLm5vZGVMaXN0Vmlld1xuICAgIH1cbiAgfSxcblxuICBjb21wb25lbnREaWRNb3VudCgpe1xuICAgIGFjdGlvbnMuZmV0Y2hOb2RlcygpO1xuICB9LFxuXG4gIHJlbmRlclJvd3MoKXtcbiAgfSxcblxuICByZW5kZXI6IGZ1bmN0aW9uKCkge1xuICAgIHZhciBkYXRhID0gdGhpcy5zdGF0ZS5ub2RlUmVjb3JkcztcbiAgICByZXR1cm4gKFxuICAgICAgPGRpdj5cbiAgICAgICAgPGgxPiBOb2RlcyA8L2gxPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiXCI+XG4gICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgICAgICA8VGFibGUgcm93Q291bnQ9e2RhdGEubGVuZ3RofT5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJjb3VudFwiXG4gICAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPiBTZXNzaW9ucyA8L0NlbGw+IH1cbiAgICAgICAgICAgICAgICAgIGNlbGw9ezxUZXh0Q2VsbCBkYXRhPXtkYXRhfS8+IH1cbiAgICAgICAgICAgICAgICAvPlxuICAgICAgICAgICAgICAgIDxDb2x1bW5cbiAgICAgICAgICAgICAgICAgIGNvbHVtbktleT1cImlwXCJcbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+IE5vZGUgPC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXs8VGV4dENlbGwgZGF0YT17ZGF0YX0vPiB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJ0YWdzXCJcbiAgICAgICAgICAgICAgICAgIGhlYWRlcj17PENlbGw+PC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXs8VGV4dENlbGwgZGF0YT17ZGF0YX0vPiB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgICA8Q29sdW1uXG4gICAgICAgICAgICAgICAgICBjb2x1bW5LZXk9XCJyb2xlc1wiXG4gICAgICAgICAgICAgICAgICBoZWFkZXI9ezxDZWxsPkxvZ2luIGFzPC9DZWxsPiB9XG4gICAgICAgICAgICAgICAgICBjZWxsPXs8VGV4dENlbGwgZGF0YT17ZGF0YX0vPiB9XG4gICAgICAgICAgICAgICAgLz5cbiAgICAgICAgICAgICAgPC9UYWJsZT5cbiAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgIClcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gTm9kZXM7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9ub2Rlcy9tYWluLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG52YXIgTm9kZXMgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXY+XG4gICAgICAgIDxoMT4gU2Vzc2lvbnMhPC9oMT5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICAgICAgPHRhYmxlIGNsYXNzTmFtZT1cInRhYmxlIHRhYmxlLXN0cmlwZWRcIj5cbiAgICAgICAgICAgICAgICA8dGhlYWQ+XG4gICAgICAgICAgICAgICAgICA8dHI+XG4gICAgICAgICAgICAgICAgICAgIDx0aD5Ob2RlPC90aD5cbiAgICAgICAgICAgICAgICAgICAgPHRoPlN0YXR1czwvdGg+XG4gICAgICAgICAgICAgICAgICAgIDx0aD5MYWJlbHM8L3RoPlxuICAgICAgICAgICAgICAgICAgICAgIDx0aD5DUFU8L3RoPlxuICAgICAgICAgICAgICAgICAgICAgIDx0aD5SQU08L3RoPlxuICAgICAgICAgICAgICAgICAgICAgIDx0aD5PUzwvdGg+XG4gICAgICAgICAgICAgICAgICAgICAgPHRoPiBMYXN0IEhlYXJ0YmVhdCA8L3RoPlxuICAgICAgICAgICAgICAgICAgICA8L3RyPlxuICAgICAgICAgICAgICAgICAgPC90aGVhZD5cbiAgICAgICAgICAgICAgICA8dGJvZHk+PC90Ym9keT5cbiAgICAgICAgICAgICAgPC90YWJsZT5cbiAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgIDwvZGl2PlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgIClcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gTm9kZXM7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvY29tcG9uZW50cy9zZXNzaW9ucy9tYWluLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG5cbnZhciBHcnZUYWJsZUNlbGwgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIHJlbmRlcigpe1xuICAgIHZhciBwcm9wcyA9IHRoaXMucHJvcHM7XG4gICAgcmV0dXJuIHByb3BzLmlzSGVhZGVyID8gPHRoIGtleT17cHJvcHMua2V5fT57cHJvcHMuY2hpbGRyZW59PC90aD4gOiA8dGQga2V5PXtwcm9wcy5rZXl9Pntwcm9wcy5jaGlsZHJlbn08L3RkPjtcbiAgfVxufSk7XG5cbnZhciBHcnZUYWJsZSA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICByZW5kZXJIZWFkZXIoY2hpbGRyZW4pe1xuICAgIHZhciBjZWxscyA9IGNoaWxkcmVuLm1hcCgoaXRlbSwgaW5kZXgpPT57XG4gICAgICByZXR1cm4gdGhpcy5yZW5kZXJDZWxsKGl0ZW0ucHJvcHMuaGVhZGVyLCB7aW5kZXgsIGtleTogaW5kZXgsIGlzSGVhZGVyOiB0cnVlLCAuLi5pdGVtLnByb3BzfSk7XG4gICAgfSlcblxuICAgIHJldHVybiA8dGhlYWQ+PHRyPntjZWxsc308L3RyPjwvdGhlYWQ+XG4gIH0sXG5cbiAgcmVuZGVyQm9keShjaGlsZHJlbil7XG4gICAgdmFyIGNvdW50ID0gdGhpcy5wcm9wcy5yb3dDb3VudDtcbiAgICB2YXIgcm93cyA9IFtdO1xuICAgIGZvcih2YXIgaSA9IDA7IGkgPCBjb3VudDsgaSArKyl7XG4gICAgICB2YXIgY2VsbHMgPSBjaGlsZHJlbi5tYXAoKGl0ZW0sIGluZGV4KT0+e1xuICAgICAgICByZXR1cm4gdGhpcy5yZW5kZXJDZWxsKGl0ZW0ucHJvcHMuY2VsbCwge3Jvd0luZGV4OiBpLCBrZXk6IGksIGlzSGVhZGVyOiBmYWxzZSwgLi4uaXRlbS5wcm9wc30pO1xuICAgICAgfSlcblxuICAgICAgcm93cy5wdXNoKDx0ciBrZXk9e2l9PntjZWxsc308L3RyPik7XG4gICAgfVxuXG4gICAgcmV0dXJuIDx0Ym9keT57cm93c308L3Rib2R5PjtcbiAgfSxcblxuICByZW5kZXJDZWxsKGNlbGwsIGNlbGxQcm9wcyl7XG4gICAgdmFyIGNvbnRlbnQgPSBudWxsO1xuICAgIGlmIChSZWFjdC5pc1ZhbGlkRWxlbWVudChjZWxsKSkge1xuICAgICAgIGNvbnRlbnQgPSBSZWFjdC5jbG9uZUVsZW1lbnQoY2VsbCwgY2VsbFByb3BzKTtcbiAgICAgfSBlbHNlIGlmICh0eXBlb2YgcHJvcHMuY2VsbCA9PT0gJ2Z1bmN0aW9uJykge1xuICAgICAgIGNvbnRlbnQgPSBjZWxsKGNlbGxQcm9wcyk7XG4gICAgIH1cblxuICAgICByZXR1cm4gY29udGVudDtcbiAgfSxcblxuICByZW5kZXIoKSB7XG4gICAgdmFyIGNoaWxkcmVuID0gW107XG4gICAgUmVhY3QuQ2hpbGRyZW4uZm9yRWFjaCh0aGlzLnByb3BzLmNoaWxkcmVuLCAoY2hpbGQsIGluZGV4KSA9PiB7XG4gICAgICBpZiAoY2hpbGQgPT0gbnVsbCkge1xuICAgICAgICByZXR1cm47XG4gICAgICB9XG5cbiAgICAgIGlmKGNoaWxkLnR5cGUuZGlzcGxheU5hbWUgIT09ICdHcnZUYWJsZUNvbHVtbicpe1xuICAgICAgICB0aHJvdyAnU2hvdWxkIGJlIEdydlRhYmxlQ29sdW1uJztcbiAgICAgIH1cblxuICAgICAgY2hpbGRyZW4ucHVzaChjaGlsZCk7XG4gICAgfSk7XG5cbiAgICByZXR1cm4gKFxuICAgICAgPHRhYmxlIGNsYXNzTmFtZT1cInRhYmxlIHRhYmxlLWJvcmRlcmVkXCI+XG4gICAgICAgIHt0aGlzLnJlbmRlckhlYWRlcihjaGlsZHJlbil9XG4gICAgICAgIHt0aGlzLnJlbmRlckJvZHkoY2hpbGRyZW4pfVxuICAgICAgPC90YWJsZT5cbiAgICApO1xuICB9XG59KVxuXG52YXIgR3J2VGFibGVDb2x1bW4gPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgdGhyb3cgbmV3IEVycm9yKCdDb21wb25lbnQgPEdydlRhYmxlQ29sdW1uIC8+IHNob3VsZCBuZXZlciByZW5kZXInKTtcbiAgfVxufSlcblxuZXhwb3J0IGRlZmF1bHQgR3J2VGFibGU7XG5leHBvcnQge0dydlRhYmxlQ29sdW1uIGFzIENvbHVtbiwgR3J2VGFibGUgYXMgVGFibGUsIEdydlRhYmxlQ2VsbCBhcyBDZWxsfTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL3RhYmxlLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgcmVuZGVyID0gcmVxdWlyZSgncmVhY3QtZG9tJykucmVuZGVyO1xudmFyIHsgUm91dGVyLCBSb3V0ZSwgUmVkaXJlY3QsIEluZGV4Um91dGUsIGJyb3dzZXJIaXN0b3J5IH0gPSByZXF1aXJlKCdyZWFjdC1yb3V0ZXInKTtcbnZhciB7IEFwcCwgTG9naW4sIE5vZGVzLCBTZXNzaW9ucywgTmV3VXNlciB9ID0gcmVxdWlyZSgnLi9jb21wb25lbnRzJyk7XG52YXIgYXV0aCA9IHJlcXVpcmUoJy4vYXV0aCcpO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCcuL3Nlc3Npb24nKTtcbnZhciBjZmcgPSByZXF1aXJlKCcuL2NvbmZpZycpO1xuXG5yZXF1aXJlKCcuL21vZHVsZXMnKTtcblxuLy8gaW5pdCBzZXNzaW9uXG5zZXNzaW9uLmluaXQoKTtcblxuZnVuY3Rpb24gcmVxdWlyZXNBdXRoKG5leHRTdGF0ZSwgcmVwbGFjZSwgY2IpIHtcbiAgYXV0aC5lbnN1cmVVc2VyKClcbiAgICAuZG9uZSgoKT0+IGNiKCkpXG4gICAgLmZhaWwoKCk9PntcbiAgICAgIHJlcGxhY2Uoe3JlZGlyZWN0VG86IG5leHRTdGF0ZS5sb2NhdGlvbi5wYXRobmFtZSB9LCBjZmcucm91dGVzLmxvZ2luKTtcbiAgICAgIGNiKCk7XG4gICAgfSk7XG59XG5cbmZ1bmN0aW9uIGhhbmRsZUxvZ291dChuZXh0U3RhdGUsIHJlcGxhY2UsIGNiKXtcbiAgYXV0aC5sb2dvdXQoKTtcbiAgLy8gZ29pbmcgYmFjayB3aWxsIGhpdCByZXF1aXJlQXV0aCBoYW5kbGVyIHdoaWNoIHdpbGwgcmVkaXJlY3QgaXQgdG8gdGhlIGxvZ2luIHBhZ2VcbiAgc2Vzc2lvbi5nZXRIaXN0b3J5KCkuZ29CYWNrKCk7XG59XG5cbnJlbmRlcigoXG4gIDxSb3V0ZXIgaGlzdG9yeT17c2Vzc2lvbi5nZXRIaXN0b3J5KCl9PlxuICAgIDxSb3V0ZSBwYXRoPXtjZmcucm91dGVzLmxvZ2lufSBjb21wb25lbnQ9e0xvZ2lufS8+XG4gICAgPFJvdXRlIHBhdGg9e2NmZy5yb3V0ZXMubG9nb3V0fSBvbkVudGVyPXtoYW5kbGVMb2dvdXR9Lz5cbiAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5uZXdVc2VyfSBjb21wb25lbnQ9e05ld1VzZXJ9Lz5cblxuICAgIDxSb3V0ZSBwYXRoPXtjZmcucm91dGVzLmFwcH0gY29tcG9uZW50PXtBcHB9IG9uRW50ZXI9e3JlcXVpcmVzQXV0aH0+XG4gICAgICA8Um91dGUgcGF0aD17Y2ZnLnJvdXRlcy5ub2Rlc30gY29tcG9uZW50PXtOb2Rlc30vPlxuICAgICAgPFJvdXRlIHBhdGg9e2NmZy5yb3V0ZXMuc2Vzc2lvbnN9IGNvbXBvbmVudD17U2Vzc2lvbnN9Lz5cbiAgICA8L1JvdXRlPlxuICA8L1JvdXRlcj5cbiksIGRvY3VtZW50LmdldEVsZW1lbnRCeUlkKFwiYXBwXCIpKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9pbmRleC5qc3hcbiAqKi8iXSwic291cmNlUm9vdCI6IiJ9