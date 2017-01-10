webpackJsonp([0],{

/***/ 0:
/***/ function(module, exports, __webpack_require__) {

	module.exports = __webpack_require__(1);


/***/ },

/***/ 1:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

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

	var React = __webpack_require__(2);
	var render = __webpack_require__(154).render;

	var _require = __webpack_require__(155),
	    Router = _require.Router,
	    Route = _require.Route,
	    Redirect = _require.Redirect;

	var _require2 = __webpack_require__(212),
	    App = _require2.App,
	    Login = _require2.Login,
	    Nodes = _require2.Nodes,
	    Sessions = _require2.Sessions,
	    NewUser = _require2.NewUser,
	    CurrentSessionHost = _require2.CurrentSessionHost,
	    MessagePage = _require2.MessagePage,
	    NotFound = _require2.NotFound;

	var _require3 = __webpack_require__(371),
	    ensureUser = _require3.ensureUser;

	var _require4 = __webpack_require__(226),
	    initApp = _require4.initApp;

	var _require5 = __webpack_require__(386),
	    openSession = _require5.openSession;

	var session = __webpack_require__(229);
	var cfg = __webpack_require__(217);

	__webpack_require__(450);

	// init session
	session.init();

	cfg.init(window.GRV_CONFIG);

	render(React.createElement(
	  Router,
	  { history: session.getHistory() },
	  React.createElement(Route, { path: cfg.routes.msgs, component: MessagePage }),
	  React.createElement(Route, { path: cfg.routes.login, component: Login }),
	  React.createElement(Route, { path: cfg.routes.newUser, component: NewUser }),
	  React.createElement(Redirect, { from: cfg.routes.app, to: cfg.routes.nodes }),
	  React.createElement(
	    Route,
	    { path: cfg.routes.app, onEnter: ensureUser, component: App },
	    React.createElement(
	      Route,
	      { path: cfg.routes.app, onEnter: initApp },
	      React.createElement(Route, { path: cfg.routes.sessions, component: Sessions }),
	      React.createElement(Route, { path: cfg.routes.nodes, component: Nodes }),
	      React.createElement(Route, { path: cfg.routes.currentSession, onEnter: openSession, components: { CurrentSessionHost: CurrentSessionHost } })
	    )
	  ),
	  React.createElement(Route, { path: '*', component: NotFound })
	), document.getElementById("app"));

/***/ },

/***/ 212:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

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

	module.exports.App = __webpack_require__(213);
	module.exports.Login = __webpack_require__(365);
	module.exports.NewUser = __webpack_require__(377);
	module.exports.Nodes = __webpack_require__(379);
	module.exports.Sessions = __webpack_require__(394);
	module.exports.CurrentSessionHost = __webpack_require__(403);
	module.exports.ErrorPage = __webpack_require__(378).ErrorPage;
	module.exports.NotFound = __webpack_require__(378).NotFound;
	module.exports.MessagePage = __webpack_require__(378).MessagePage;

/***/ },

/***/ 213:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

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

	var React = __webpack_require__(2);
	var NavLeftBar = __webpack_require__(214);
	var reactor = __webpack_require__(215);

	var _require = __webpack_require__(347),
	    getters = _require.getters;

	var _require2 = __webpack_require__(226),
	    checkIfValidUser = _require2.checkIfValidUser,
	    refresh = _require2.refresh;

	var NotificationHost = __webpack_require__(349);
	var Timer = __webpack_require__(364);

	var App = React.createClass({
	  displayName: 'App',


	  mixins: [reactor.ReactMixin],

	  getDataBindings: function getDataBindings() {
	    return {
	      appStatus: getters.appStatus
	    };
	  },
	  render: function render() {
	    if (this.state.appStatus.isInitializing) {
	      return null;
	    }

	    return React.createElement(
	      'div',
	      { className: 'grv-tlpt grv-flex grv-flex-row' },
	      React.createElement(Timer, { onTimeout: checkIfValidUser, interval: 10000 }),
	      React.createElement(Timer, { onTimeout: refresh, interval: 4000 }),
	      React.createElement(NotificationHost, null),
	      this.props.CurrentSessionHost,
	      React.createElement(NavLeftBar, null),
	      this.props.children
	    );
	  }
	});

	module.exports = App;

/***/ },

/***/ 214:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

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
	var React = __webpack_require__(2);
	var reactor = __webpack_require__(215);
	var cfg = __webpack_require__(217);
	var userGetters = __webpack_require__(222);

	var _require = __webpack_require__(155),
	    IndexLink = _require.IndexLink;

	var _require2 = __webpack_require__(226),
	    logoutUser = _require2.logoutUser;

	var _require3 = __webpack_require__(341),
	    UserIcon = _require3.UserIcon;

	var menuItems = [{ icon: 'fa fa-share-alt', to: cfg.routes.nodes, title: 'Nodes' }, { icon: 'fa  fa-group', to: cfg.routes.sessions, title: 'Sessions' }];

	var NavLeftBar = React.createClass({
	  displayName: 'NavLeftBar',
	  render: function render() {
	    var _this = this;

	    var _reactor$evaluate = reactor.evaluate(userGetters.user),
	        name = _reactor$evaluate.name;

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
	        { href: '#', onClick: logoutUser },
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

/***/ },

/***/ 215:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _nuclearJs = __webpack_require__(216);

	var __DEV__ = ("production") === 'development'; /*
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

	var reactor = new _nuclearJs.Reactor({
	  debug: __DEV__
	});

	window.reactor = reactor;

	exports.default = reactor;
	module.exports = exports['default'];

/***/ },

/***/ 217:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

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

	var _require = __webpack_require__(218),
	    formatPattern = _require.formatPattern;

	var $ = __webpack_require__(219);

	var cfg = {

	  baseUrl: window.location.origin,

	  helpUrl: 'https://gravitational.com/teleport/docs/quickstart/',

	  maxSessionLoadSize: 50,

	  displayDateFormat: 'l LTS Z',

	  auth: {
	    oidc_connectors: [],
	    u2f_appid: ""
	  },

	  routes: {
	    app: '/web',
	    login: '/web/login',
	    nodes: '/web/nodes',
	    currentSession: '/web/cluster/:siteId/sessions/:sid',
	    sessions: '/web/sessions',
	    newUser: '/web/newuser/:inviteToken',
	    msgs: '/web/msg/:type(/:subType)',
	    pageNotFound: '/web/notfound'
	  },

	  api: {
	    sso: '/v1/webapi/oidc/login/web?redirect_url=:redirect&connector_id=:provider',
	    renewTokenPath: '/v1/webapi/sessions/renew',
	    sessionPath: '/v1/webapi/sessions',
	    userStatus: '/v1/webapi/user/status',
	    invitePath: '/v1/webapi/users/invites/:inviteToken',
	    createUserPath: '/v1/webapi/users',
	    u2fCreateUserChallengePath: '/webapi/u2f/signuptokens/:inviteToken',
	    u2fCreateUserPath: '/webapi/u2f/users',
	    u2fSessionChallengePath: '/webapi/u2f/signrequest',
	    u2fSessionPath: '/webapi/u2f/sessions',
	    sitesBasePath: '/v1/webapi/sites',
	    sitePath: '/v1/webapi/sites/:siteId',
	    nodesPath: '/v1/webapi/sites/:siteId/nodes',
	    siteSessionPath: '/v1/webapi/sites/:siteId/sessions',
	    sessionEventsPath: '/v1/webapi/sites/:siteId/sessions/:sid/events',
	    siteEventSessionFilterPath: '/v1/webapi/sites/:siteId/sessions',
	    siteEventsFilterPath: '/v1/webapi/sites/:siteId/events?event=session.start&event=session.end&from=:start&to=:end',

	    getSiteUrl: function getSiteUrl(siteId) {
	      return formatPattern(cfg.api.sitePath, { siteId: siteId });
	    },
	    getSiteNodesUrl: function getSiteNodesUrl() {
	      var siteId = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : '-current-';

	      return formatPattern(cfg.api.nodesPath, { siteId: siteId });
	    },
	    getSiteSessionUrl: function getSiteSessionUrl() {
	      var siteId = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : '-current-';

	      return formatPattern(cfg.api.siteSessionPath, { siteId: siteId });
	    },
	    getSsoUrl: function getSsoUrl(redirect, provider) {
	      return cfg.baseUrl + formatPattern(cfg.api.sso, { redirect: redirect, provider: provider });
	    },
	    getSiteEventsFilterUrl: function getSiteEventsFilterUrl(_ref) {
	      var start = _ref.start,
	          end = _ref.end,
	          siteId = _ref.siteId;

	      return formatPattern(cfg.api.siteEventsFilterPath, { start: start, end: end, siteId: siteId });
	    },
	    getSessionEventsUrl: function getSessionEventsUrl(_ref2) {
	      var sid = _ref2.sid,
	          siteId = _ref2.siteId;

	      return formatPattern(cfg.api.sessionEventsPath, { sid: sid, siteId: siteId });
	    },
	    getFetchSessionsUrl: function getFetchSessionsUrl(siteId) {
	      return formatPattern(cfg.api.siteEventSessionFilterPath, { siteId: siteId });
	    },
	    getFetchSessionUrl: function getFetchSessionUrl(_ref3) {
	      var sid = _ref3.sid,
	          siteId = _ref3.siteId;

	      return formatPattern(cfg.api.siteSessionPath + '/:sid', { sid: sid, siteId: siteId });
	    },
	    getInviteUrl: function getInviteUrl(inviteToken) {
	      return formatPattern(cfg.api.invitePath, { inviteToken: inviteToken });
	    },
	    getU2fCreateUserChallengeUrl: function getU2fCreateUserChallengeUrl(inviteToken) {
	      return formatPattern(cfg.api.u2fCreateUserChallengePath, { inviteToken: inviteToken });
	    }
	  },

	  getFullUrl: function getFullUrl(url) {
	    return cfg.baseUrl + url;
	  },
	  getCurrentSessionRouteUrl: function getCurrentSessionRouteUrl(_ref4) {
	    var sid = _ref4.sid,
	        siteId = _ref4.siteId;

	    return formatPattern(cfg.routes.currentSession, { sid: sid, siteId: siteId });
	  },
	  getAuthProviders: function getAuthProviders() {
	    return cfg.auth.oidc_connectors;
	  },
	  getU2fAppId: function getU2fAppId() {
	    return cfg.auth.u2f_appid;
	  },
	  init: function init() {
	    var config = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : {};

	    $.extend(true, this, config);
	  }
	};

	exports.default = cfg;
	module.exports = exports['default'];

/***/ },

/***/ 218:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.compilePattern = compilePattern;
	exports.matchPattern = matchPattern;
	exports.getParamNames = getParamNames;
	exports.getParams = getParams;
	exports.formatPattern = formatPattern;

	var _invariant = __webpack_require__(159);

	var _invariant2 = _interopRequireDefault(_invariant);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function escapeRegExp(string) {
	  return string.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
	} /*
	   *  The MIT License (MIT)
	   *  Copyright (c) 2015 Ryan Florence, Michael Jackson
	   *  Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:
	   *  The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.
	   *  THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
	  */

	function escapeSource(string) {
	  return escapeRegExp(string).replace(/\/+/g, '/+');
	}

	function _compilePattern(pattern) {
	  var regexpSource = '';
	  var paramNames = [];
	  var tokens = [];

	  var match = void 0,
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

	  var _compilePattern2 = compilePattern(pattern),
	      regexpSource = _compilePattern2.regexpSource,
	      paramNames = _compilePattern2.paramNames,
	      tokens = _compilePattern2.tokens;

	  regexpSource += '/*'; // Capture path separators

	  // Special-case patterns like '*' for catch-all routes.
	  var captureRemaining = tokens[tokens.length - 1] !== '*';

	  if (captureRemaining) {
	    // This will match newlines in the remaining path.
	    regexpSource += '([\\s\\S]*?)';
	  }

	  var match = pathname.match(new RegExp('^' + regexpSource + '$', 'i'));

	  var remainingPathname = void 0,
	      paramValues = void 0;
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
	  var _matchPattern = matchPattern(pattern, pathname),
	      paramNames = _matchPattern.paramNames,
	      paramValues = _matchPattern.paramValues;

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

	  var _compilePattern3 = compilePattern(pattern),
	      tokens = _compilePattern3.tokens;

	  var parenCount = 0,
	      pathname = '',
	      splatIndex = 0;

	  var token = void 0,
	      paramName = void 0,
	      paramValue = void 0;
	  for (var i = 0, len = tokens.length; i < len; ++i) {
	    token = tokens[i];

	    if (token === '*' || token === '**') {
	      paramValue = Array.isArray(params.splat) ? params.splat[splatIndex++] : params.splat;

	      (0, _invariant2.default)(paramValue != null || parenCount > 0, 'Missing splat #%s for path "%s"', splatIndex, pattern);

	      if (paramValue != null) pathname += encodeURI(paramValue);
	    } else if (token === '(') {
	      parenCount += 1;
	    } else if (token === ')') {
	      parenCount -= 1;
	    } else if (token.charAt(0) === ':') {
	      paramName = token.substring(1);
	      paramValue = params[paramName];

	      (0, _invariant2.default)(paramValue != null || parenCount > 0, 'Missing "%s" parameter for path "%s"', paramName, pattern);

	      if (paramValue != null) pathname += encodeURIComponent(paramValue);
	    } else {
	      pathname += token;
	    }
	  }

	  return pathname.replace(/\/+/g, '/');
	}

/***/ },

/***/ 222:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

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

	var _require = __webpack_require__(223),
	    TRYING_TO_LOGIN = _require.TRYING_TO_LOGIN,
	    TRYING_TO_SIGN_UP = _require.TRYING_TO_SIGN_UP,
	    FETCHING_INVITE = _require.FETCHING_INVITE;

	var _require2 = __webpack_require__(225),
	    requestStatus = _require2.requestStatus;

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

	exports.default = {
	  user: user,
	  invite: invite,
	  loginAttemp: requestStatus(TRYING_TO_LOGIN),
	  attemp: requestStatus(TRYING_TO_SIGN_UP),
	  fetchingInvite: requestStatus(FETCHING_INVITE)
	};
	module.exports = exports['default'];

/***/ },

/***/ 223:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _keymirror = __webpack_require__(224);

	var _keymirror2 = _interopRequireDefault(_keymirror);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	exports.default = (0, _keymirror2.default)({
	  TRYING_TO_SIGN_UP: null,
	  TRYING_TO_LOGIN: null,
	  FETCHING_INVITE: null
	}); /*
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

	module.exports = exports['default'];

/***/ },

/***/ 225:
/***/ function(module, exports) {

	'use strict';

	exports.__esModule = true;
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

	exports.default = { requestStatus: requestStatus };
	module.exports = exports['default'];

/***/ },

/***/ 226:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _reactor = __webpack_require__(215);

	var _reactor2 = _interopRequireDefault(_reactor);

	var _auth = __webpack_require__(227);

	var _auth2 = _interopRequireDefault(_auth);

	var _actions = __webpack_require__(232);

	var _actionTypes = __webpack_require__(234);

	var _actionTypes2 = __webpack_require__(235);

	var _api = __webpack_require__(228);

	var _api2 = _interopRequireDefault(_api);

	var _config = __webpack_require__(217);

	var _config2 = _interopRequireDefault(_config);

	var _actions2 = __webpack_require__(236);

	var _actions3 = __webpack_require__(239);

	var _jQuery = __webpack_require__(219);

	var _jQuery2 = _interopRequireDefault(_jQuery);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

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

	var logger = __webpack_require__(230).create('flux/app');

	var actions = {
	  setSiteId: function setSiteId(siteId) {
	    _reactor2.default.dispatch(_actionTypes.TLPT_APP_SET_SITE_ID, siteId);
	  },
	  initApp: function initApp(nextState, replace, cb) {
	    var siteId = nextState.params.siteId;

	    _reactor2.default.dispatch(_actionTypes.TLPT_APP_INIT);
	    // get the list of available clusters
	    actions.fetchSites().then(function (masterSiteId) {
	      siteId = siteId || masterSiteId;
	      _reactor2.default.dispatch(_actionTypes.TLPT_APP_SET_SITE_ID, siteId);
	      // fetch nodes and active sessions 
	      _jQuery2.default.when((0, _actions2.fetchNodes)(), (0, _actions3.fetchActiveSessions)()).done(function () {
	        _reactor2.default.dispatch(_actionTypes.TLPT_APP_READY);
	        cb();
	      });
	    }).fail(function () {
	      _reactor2.default.dispatch(_actionTypes.TLPT_APP_FAILED);
	      cb();
	    });
	  },
	  refresh: function refresh() {
	    actions.fetchSites();
	    (0, _actions3.fetchActiveSessions)();
	    (0, _actions2.fetchNodes)();
	  },
	  fetchSites: function fetchSites() {
	    return _api2.default.get(_config2.default.api.sitesBasePath).then(function (json) {
	      var masterSiteId = null;
	      var sites = json.sites;
	      if (sites) {
	        masterSiteId = sites[0].name;
	      }

	      _reactor2.default.dispatch(_actionTypes2.TLPT_SITES_RECEIVE, sites);

	      return masterSiteId;
	    }).fail(function (err) {
	      (0, _actions.showError)('Unable to retrieve list of clusters ');
	      logger.error('fetchSites', err);
	    });
	  },
	  resetApp: function resetApp() {
	    // set to 'loading state' to notify subscribers
	    _reactor2.default.dispatch(_actionTypes.TLPT_APP_INIT);
	    // reset  reactor
	    _reactor2.default.reset();
	  },
	  checkIfValidUser: function checkIfValidUser() {
	    _api2.default.get(_config2.default.api.userStatus).fail(function (err) {
	      if (err.status == 403) {
	        actions.logoutUser();
	      }
	    });
	  },
	  logoutUser: function logoutUser() {
	    actions.resetApp();
	    _auth2.default.logout();
	  }
	};

	window.actions = actions;

	exports.default = actions;
	module.exports = exports['default'];

/***/ },

/***/ 227:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

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

	var api = __webpack_require__(228);
	var session = __webpack_require__(229);
	var cfg = __webpack_require__(217);
	var $ = __webpack_require__(219);
	var logger = __webpack_require__(230).create('services/auth');
	__webpack_require__(231); // This puts it in window.u2f

	var PROVIDER_GOOGLE = 'google';

	var SECOND_FACTOR_TYPE_HOTP = 'hotp';
	var SECOND_FACTOR_TYPE_OIDC = 'oidc';
	var SECOND_FACTOR_TYPE_U2F = 'u2f';

	var CHECK_TOKEN_REFRESH_RATE = 10 * 1000; // 10 sec

	var refreshTokenTimerId = null;

	var auth = {
	  signUp: function signUp(name, password, token, inviteToken) {
	    var data = { user: name, pass: password, second_factor_token: token, invite_token: inviteToken };
	    return api.post(cfg.api.createUserPath, data).then(function (data) {
	      session.setUserData(data);
	      auth._startTokenRefresher();
	      return data;
	    });
	  },
	  u2fSignUp: function u2fSignUp(name, password, inviteToken) {
	    return api.get(cfg.api.getU2fCreateUserChallengeUrl(inviteToken)).then(function (data) {
	      var deferred = $.Deferred();

	      window.u2f.register(data.appId, [data], [], function (res) {
	        if (res.errorCode) {
	          var err = auth._getU2fErr(res.errorCode);
	          deferred.reject(err);
	          return;
	        }

	        var response = {
	          user: name,
	          pass: password,
	          u2f_register_response: res,
	          invite_token: inviteToken
	        };
	        api.post(cfg.api.u2fCreateUserPath, response, false).then(function (data) {
	          session.setUserData(data);
	          auth._startTokenRefresher();
	          deferred.resolve(data);
	        }).fail(function (data) {
	          deferred.reject(data);
	        });
	      });

	      return deferred.promise();
	    });
	  },
	  login: function login(name, password, token) {
	    var _this = this;

	    auth._stopTokenRefresher();
	    session.clear();

	    var data = {
	      user: name,
	      pass: password,
	      second_factor_token: token
	    };

	    return api.post(cfg.api.sessionPath, data, false).then(function (data) {
	      session.setUserData(data);
	      _this._startTokenRefresher();
	      return data;
	    });
	  },
	  u2fLogin: function u2fLogin(name, password) {
	    auth._stopTokenRefresher();
	    session.clear();

	    var data = {
	      user: name,
	      pass: password
	    };

	    return api.post(cfg.api.u2fSessionChallengePath, data, false).then(function (data) {
	      var deferred = $.Deferred();

	      window.u2f.sign(data.appId, data.challenge, [data], function (res) {
	        if (res.errorCode) {
	          var err = auth._getU2fErr(res.errorCode);
	          deferred.reject(err);
	          return;
	        }

	        var response = {
	          user: name,
	          u2f_sign_response: res
	        };
	        api.post(cfg.api.u2fSessionPath, response, false).then(function (data) {
	          session.setUserData(data);
	          auth._startTokenRefresher();
	          deferred.resolve(data);
	        }).fail(function (data) {
	          deferred.reject(data);
	        });
	      });

	      return deferred.promise();
	    });
	  },
	  ensureUser: function ensureUser() {
	    this._stopTokenRefresher();

	    var userData = session.getUserData();

	    if (!userData.token) {
	      return $.Deferred().reject();
	    }

	    if (this._shouldRefreshToken(userData)) {
	      return this._refreshToken().done(this._startTokenRefresher);
	    }

	    this._startTokenRefresher();
	    return $.Deferred().resolve(userData);
	  },
	  logout: function logout() {
	    logger.info('logout()');
	    api.delete(cfg.api.sessionPath).always(function () {
	      auth._redirect();
	    });
	    session.clear();
	    auth._stopTokenRefresher();
	  },
	  _redirect: function _redirect() {
	    window.location = cfg.routes.login;
	  },
	  _shouldRefreshToken: function _shouldRefreshToken(_ref) {
	    var expires_in = _ref.expires_in,
	        created = _ref.created;

	    if (!created || !expires_in) {
	      return true;
	    }

	    if (expires_in < 0) {
	      expires_in = expires_in * -1;
	    }

	    expires_in = expires_in * 1000;

	    var delta = created + expires_in - new Date().getTime();

	    return delta < expires_in * 0.33;
	  },
	  _startTokenRefresher: function _startTokenRefresher() {
	    refreshTokenTimerId = setInterval(auth.ensureUser.bind(auth), CHECK_TOKEN_REFRESH_RATE);
	  },
	  _stopTokenRefresher: function _stopTokenRefresher() {
	    clearInterval(refreshTokenTimerId);
	    refreshTokenTimerId = null;
	  },
	  _refreshToken: function _refreshToken() {
	    return api.post(cfg.api.renewTokenPath).then(function (data) {
	      session.setUserData(data);
	      return data;
	    }).fail(function () {
	      auth.logout();
	    });
	  },
	  _getU2fErr: function _getU2fErr(errorCode) {
	    var errorMsg = "";
	    // lookup error message...
	    for (var msg in window.u2f.ErrorCodes) {
	      if (window.u2f.ErrorCodes[msg] == errorCode) {
	        errorMsg = msg;
	      }
	    }
	    return { responseJSON: { message: "U2F Error: " + errorMsg } };
	  }
	};

	module.exports = auth;
	module.exports.PROVIDER_GOOGLE = PROVIDER_GOOGLE;
	module.exports.SECOND_FACTOR_TYPE_HOTP = SECOND_FACTOR_TYPE_HOTP;
	module.exports.SECOND_FACTOR_TYPE_OIDC = SECOND_FACTOR_TYPE_OIDC;
	module.exports.SECOND_FACTOR_TYPE_U2F = SECOND_FACTOR_TYPE_U2F;

/***/ },

/***/ 228:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

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

	var $ = __webpack_require__(219);
	var session = __webpack_require__(229);

	var api = {
	  put: function put(path, data, withToken) {
	    return api.ajax({ url: path, data: JSON.stringify(data), type: 'PUT' }, withToken);
	  },
	  post: function post(path, data, withToken) {
	    return api.ajax({ url: path, data: JSON.stringify(data), type: 'POST' }, withToken);
	  },
	  delete: function _delete(path, data, withToken) {
	    return api.ajax({ url: path, data: JSON.stringify(data), type: 'DELETE' }, withToken);
	  },
	  get: function get(path) {
	    return api.ajax({ url: path });
	  },
	  ajax: function ajax(cfg) {
	    var withToken = arguments.length > 1 && arguments[1] !== undefined ? arguments[1] : true;

	    var defaultCfg = {
	      // to avoid caching in IE browsers
	      // (implicitly disabling caching adds a timestamp to each ajax requestStatus)
	      cache: false,
	      type: "GET",
	      dataType: "json",
	      beforeSend: function beforeSend(xhr) {
	        if (withToken) {
	          var _session$getUserData = session.getUserData(),
	              token = _session$getUserData.token;

	          xhr.setRequestHeader('Authorization', 'Bearer ' + token);
	        }
	      }
	    };

	    return $.ajax($.extend({}, defaultCfg, cfg));
	  }
	};

	module.exports = api;

/***/ },

/***/ 229:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

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

	var _require = __webpack_require__(155),
	    browserHistory = _require.browserHistory,
	    createMemoryHistory = _require.createMemoryHistory;

	var $ = __webpack_require__(219);

	var logger = __webpack_require__(230).create('services/sessions');
	var AUTH_KEY_DATA = 'authData';

	var _history = createMemoryHistory();

	var UserData = function UserData(json) {
	  $.extend(this, json);
	  this.created = new Date().getTime();
	};

	var session = {
	  init: function init() {
	    var history = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : browserHistory;

	    _history = history;
	  },
	  getHistory: function getHistory() {
	    return _history;
	  },
	  setUserData: function setUserData(data) {
	    var userData = new UserData(data);
	    localStorage.setItem(AUTH_KEY_DATA, JSON.stringify(userData));
	    return userData;
	  },
	  getUserData: function getUserData() {
	    try {
	      var item = localStorage.getItem(AUTH_KEY_DATA);
	      if (item) {
	        return JSON.parse(item);
	      }

	      // for sso use-cases, try to grab the token from HTML
	      var hiddenDiv = document.getElementById("bearer_token");
	      if (hiddenDiv !== null) {
	        var json = window.atob(hiddenDiv.textContent);
	        var data = JSON.parse(json);
	        if (data.token) {
	          // put it into the session
	          var userData = this.setUserData(data);
	          // remove the element
	          hiddenDiv.remove();
	          return userData;
	        }
	      }
	    } catch (err) {
	      logger.error('error trying to read user auth data:', err);
	    }

	    return {};
	  },
	  clear: function clear() {
	    localStorage.clear();
	  }
	};

	module.exports = session;

/***/ },

/***/ 230:
/***/ function(module, exports) {

	'use strict';

	exports.__esModule = true;

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

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

	var Logger = function () {
	  function Logger() {
	    var name = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : 'default';

	    _classCallCheck(this, Logger);

	    this.name = name;
	  }

	  Logger.prototype.log = function log() {
	    var _console;

	    var level = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : 'log';

	    for (var _len = arguments.length, args = Array(_len > 1 ? _len - 1 : 0), _key = 1; _key < _len; _key++) {
	      args[_key - 1] = arguments[_key];
	    }

	    (_console = console)[level].apply(_console, ['%c[' + this.name + ']', 'color: blue;'].concat(args));
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
	}();

	exports.default = {
	  create: function create() {
	    for (var _len6 = arguments.length, args = Array(_len6), _key6 = 0; _key6 < _len6; _key6++) {
	      args[_key6] = arguments[_key6];
	    }

	    return new (Function.prototype.bind.apply(Logger, [null].concat(args)))();
	  }
	};
	module.exports = exports['default'];

/***/ },

/***/ 231:
/***/ function(module, exports) {

	
	//Copyright 2014-2015 Google Inc. All rights reserved.

	//Use of this source code is governed by a BSD-style
	//license that can be found in the LICENSE file or at
	//https://developers.google.com/open-source/licenses/bsd

	/**
	 * @fileoverview The U2F api.
	 */
	'use strict';

	(function (){
	  var isChrome = 'chrome' in window && window.navigator.userAgent.indexOf('Edge') < 0;
	  if ('u2f' in window || !isChrome) {
	    return;
	  }

	  /** Namespace for the U2F api.
	   * @type {Object}
	   */
	  var u2f = window.u2f = {};

	  /**
	   * FIDO U2F Javascript API Version
	   * @number
	   */
	  var js_api_version;

	  /**
	   * The U2F extension id
	   * @const {string}
	   */
	  // The Chrome packaged app extension ID.
	  // Uncomment this if you want to deploy a server instance that uses
	  // the package Chrome app and does not require installing the U2F Chrome extension.
	   u2f.EXTENSION_ID = 'kmendfapggjehodndflmmgagdbamhnfd';
	  // The U2F Chrome extension ID.
	  // Uncomment this if you want to deploy a server instance that uses
	  // the U2F Chrome extension to authenticate.
	  // u2f.EXTENSION_ID = 'pfboblefjcgdjicmnffhdgionmgcdmne';


	  /**
	   * Message types for messsages to/from the extension
	   * @const
	   * @enum {string}
	   */
	  u2f.MessageTypes = {
	      'U2F_REGISTER_REQUEST': 'u2f_register_request',
	      'U2F_REGISTER_RESPONSE': 'u2f_register_response',
	      'U2F_SIGN_REQUEST': 'u2f_sign_request',
	      'U2F_SIGN_RESPONSE': 'u2f_sign_response',
	      'U2F_GET_API_VERSION_REQUEST': 'u2f_get_api_version_request',
	      'U2F_GET_API_VERSION_RESPONSE': 'u2f_get_api_version_response'
	  };


	  /**
	   * Response status codes
	   * @const
	   * @enum {number}
	   */
	  u2f.ErrorCodes = {
	      'OK': 0,
	      'OTHER_ERROR': 1,
	      'BAD_REQUEST': 2,
	      'CONFIGURATION_UNSUPPORTED': 3,
	      'DEVICE_INELIGIBLE': 4,
	      'TIMEOUT': 5
	  };


	  /**
	   * A message for registration requests
	   * @typedef {{
	   *   type: u2f.MessageTypes,
	   *   appId: ?string,
	   *   timeoutSeconds: ?number,
	   *   requestId: ?number
	   * }}
	   */
	  u2f.U2fRequest;


	  /**
	   * A message for registration responses
	   * @typedef {{
	   *   type: u2f.MessageTypes,
	   *   responseData: (u2f.Error | u2f.RegisterResponse | u2f.SignResponse),
	   *   requestId: ?number
	   * }}
	   */
	  u2f.U2fResponse;


	  /**
	   * An error object for responses
	   * @typedef {{
	   *   errorCode: u2f.ErrorCodes,
	   *   errorMessage: ?string
	   * }}
	   */
	  u2f.Error;

	  /**
	   * Data object for a single sign request.
	   * @typedef {enum {BLUETOOTH_RADIO, BLUETOOTH_LOW_ENERGY, USB, NFC}}
	   */
	  u2f.Transport;


	  /**
	   * Data object for a single sign request.
	   * @typedef {Array<u2f.Transport>}
	   */
	  u2f.Transports;

	  /**
	   * Data object for a single sign request.
	   * @typedef {{
	   *   version: string,
	   *   challenge: string,
	   *   keyHandle: string,
	   *   appId: string
	   * }}
	   */
	  u2f.SignRequest;


	  /**
	   * Data object for a sign response.
	   * @typedef {{
	   *   keyHandle: string,
	   *   signatureData: string,
	   *   clientData: string
	   * }}
	   */
	  u2f.SignResponse;


	  /**
	   * Data object for a registration request.
	   * @typedef {{
	   *   version: string,
	   *   challenge: string
	   * }}
	   */
	  u2f.RegisterRequest;


	  /**
	   * Data object for a registration response.
	   * @typedef {{
	   *   version: string,
	   *   keyHandle: string,
	   *   transports: Transports,
	   *   appId: string
	   * }}
	   */
	  u2f.RegisterResponse;


	  /**
	   * Data object for a registered key.
	   * @typedef {{
	   *   version: string,
	   *   keyHandle: string,
	   *   transports: ?Transports,
	   *   appId: ?string
	   * }}
	   */
	  u2f.RegisteredKey;


	  /**
	   * Data object for a get API register response.
	   * @typedef {{
	   *   js_api_version: number
	   * }}
	   */
	  u2f.GetJsApiVersionResponse;


	  //Low level MessagePort API support

	  /**
	   * Sets up a MessagePort to the U2F extension using the
	   * available mechanisms.
	   * @param {function((MessagePort|u2f.WrappedChromeRuntimePort_))} callback
	   */
	  u2f.getMessagePort = function(callback) {
	    if (typeof chrome != 'undefined' && chrome.runtime) {
	      // The actual message here does not matter, but we need to get a reply
	      // for the callback to run. Thus, send an empty signature request
	      // in order to get a failure response.
	      var msg = {
	          type: u2f.MessageTypes.U2F_SIGN_REQUEST,
	          signRequests: []
	      };
	      chrome.runtime.sendMessage(u2f.EXTENSION_ID, msg, function() {
	        if (!chrome.runtime.lastError) {
	          // We are on a whitelisted origin and can talk directly
	          // with the extension.
	          u2f.getChromeRuntimePort_(callback);
	        } else {
	          // chrome.runtime was available, but we couldn't message
	          // the extension directly, use iframe
	          u2f.getIframePort_(callback);
	        }
	      });
	    } else if (u2f.isAndroidChrome_()) {
	      u2f.getAuthenticatorPort_(callback);
	    } else if (u2f.isIosChrome_()) {
	      u2f.getIosPort_(callback);
	    } else {
	      // chrome.runtime was not available at all, which is normal
	      // when this origin doesn't have access to any extensions.
	      u2f.getIframePort_(callback);
	    }
	  };

	  /**
	   * Detect chrome running on android based on the browser's useragent.
	   * @private
	   */
	  u2f.isAndroidChrome_ = function() {
	    var userAgent = navigator.userAgent;
	    return userAgent.indexOf('Chrome') != -1 &&
	    userAgent.indexOf('Android') != -1;
	  };

	  /**
	   * Detect chrome running on iOS based on the browser's platform.
	   * @private
	   */
	  u2f.isIosChrome_ = function() {
	    return ["iPhone", "iPad", "iPod"].indexOf(navigator.platform) > -1;
	  };

	  /**
	   * Connects directly to the extension via chrome.runtime.connect.
	   * @param {function(u2f.WrappedChromeRuntimePort_)} callback
	   * @private
	   */
	  u2f.getChromeRuntimePort_ = function(callback) {
	    var port = chrome.runtime.connect(u2f.EXTENSION_ID,
	        {'includeTlsChannelId': true});
	    setTimeout(function() {
	      callback(new u2f.WrappedChromeRuntimePort_(port));
	    }, 0);
	  };

	  /**
	   * Return a 'port' abstraction to the Authenticator app.
	   * @param {function(u2f.WrappedAuthenticatorPort_)} callback
	   * @private
	   */
	  u2f.getAuthenticatorPort_ = function(callback) {
	    setTimeout(function() {
	      callback(new u2f.WrappedAuthenticatorPort_());
	    }, 0);
	  };

	  /**
	   * Return a 'port' abstraction to the iOS client app.
	   * @param {function(u2f.WrappedIosPort_)} callback
	   * @private
	   */
	  u2f.getIosPort_ = function(callback) {
	    setTimeout(function() {
	      callback(new u2f.WrappedIosPort_());
	    }, 0);
	  };

	  /**
	   * A wrapper for chrome.runtime.Port that is compatible with MessagePort.
	   * @param {Port} port
	   * @constructor
	   * @private
	   */
	  u2f.WrappedChromeRuntimePort_ = function(port) {
	    this.port_ = port;
	  };

	  /**
	   * Format and return a sign request compliant with the JS API version supported by the extension.
	   * @param {Array<u2f.SignRequest>} signRequests
	   * @param {number} timeoutSeconds
	   * @param {number} reqId
	   * @return {Object}
	   */
	  u2f.formatSignRequest_ =
	    function(appId, challenge, registeredKeys, timeoutSeconds, reqId) {
	    if (js_api_version === undefined || js_api_version < 1.1) {
	      // Adapt request to the 1.0 JS API
	      var signRequests = [];
	      for (var i = 0; i < registeredKeys.length; i++) {
	        signRequests[i] = {
	            version: registeredKeys[i].version,
	            challenge: challenge,
	            keyHandle: registeredKeys[i].keyHandle,
	            appId: appId
	        };
	      }
	      return {
	        type: u2f.MessageTypes.U2F_SIGN_REQUEST,
	        signRequests: signRequests,
	        timeoutSeconds: timeoutSeconds,
	        requestId: reqId
	      };
	    }
	    // JS 1.1 API
	    return {
	      type: u2f.MessageTypes.U2F_SIGN_REQUEST,
	      appId: appId,
	      challenge: challenge,
	      registeredKeys: registeredKeys,
	      timeoutSeconds: timeoutSeconds,
	      requestId: reqId
	    };
	  };

	  /**
	   * Format and return a register request compliant with the JS API version supported by the extension..
	   * @param {Array<u2f.SignRequest>} signRequests
	   * @param {Array<u2f.RegisterRequest>} signRequests
	   * @param {number} timeoutSeconds
	   * @param {number} reqId
	   * @return {Object}
	   */
	  u2f.formatRegisterRequest_ =
	    function(appId, registeredKeys, registerRequests, timeoutSeconds, reqId) {
	    if (js_api_version === undefined || js_api_version < 1.1) {
	      // Adapt request to the 1.0 JS API
	      for (var i = 0; i < registerRequests.length; i++) {
	        registerRequests[i].appId = appId;
	      }
	      var signRequests = [];
	      for (var i = 0; i < registeredKeys.length; i++) {
	        signRequests[i] = {
	            version: registeredKeys[i].version,
	            challenge: registerRequests[0],
	            keyHandle: registeredKeys[i].keyHandle,
	            appId: appId
	        };
	      }
	      return {
	        type: u2f.MessageTypes.U2F_REGISTER_REQUEST,
	        signRequests: signRequests,
	        registerRequests: registerRequests,
	        timeoutSeconds: timeoutSeconds,
	        requestId: reqId
	      };
	    }
	    // JS 1.1 API
	    return {
	      type: u2f.MessageTypes.U2F_REGISTER_REQUEST,
	      appId: appId,
	      registerRequests: registerRequests,
	      registeredKeys: registeredKeys,
	      timeoutSeconds: timeoutSeconds,
	      requestId: reqId
	    };
	  };


	  /**
	   * Posts a message on the underlying channel.
	   * @param {Object} message
	   */
	  u2f.WrappedChromeRuntimePort_.prototype.postMessage = function(message) {
	    this.port_.postMessage(message);
	  };


	  /**
	   * Emulates the HTML 5 addEventListener interface. Works only for the
	   * onmessage event, which is hooked up to the chrome.runtime.Port.onMessage.
	   * @param {string} eventName
	   * @param {function({data: Object})} handler
	   */
	  u2f.WrappedChromeRuntimePort_.prototype.addEventListener =
	      function(eventName, handler) {
	    var name = eventName.toLowerCase();
	    if (name == 'message' || name == 'onmessage') {
	      this.port_.onMessage.addListener(function(message) {
	        // Emulate a minimal MessageEvent object
	        handler({'data': message});
	      });
	    } else {
	      console.error('WrappedChromeRuntimePort only supports onMessage');
	    }
	  };

	  /**
	   * Wrap the Authenticator app with a MessagePort interface.
	   * @constructor
	   * @private
	   */
	  u2f.WrappedAuthenticatorPort_ = function() {
	    this.requestId_ = -1;
	    this.requestObject_ = null;
	  }

	  /**
	   * Launch the Authenticator intent.
	   * @param {Object} message
	   */
	  u2f.WrappedAuthenticatorPort_.prototype.postMessage = function(message) {
	    var intentUrl =
	      u2f.WrappedAuthenticatorPort_.INTENT_URL_BASE_ +
	      ';S.request=' + encodeURIComponent(JSON.stringify(message)) +
	      ';end';
	    document.location = intentUrl;
	  };

	  /**
	   * Tells what type of port this is.
	   * @return {String} port type
	   */
	  u2f.WrappedAuthenticatorPort_.prototype.getPortType = function() {
	    return "WrappedAuthenticatorPort_";
	  };


	  /**
	   * Emulates the HTML 5 addEventListener interface.
	   * @param {string} eventName
	   * @param {function({data: Object})} handler
	   */
	  u2f.WrappedAuthenticatorPort_.prototype.addEventListener = function(eventName, handler) {
	    var name = eventName.toLowerCase();
	    if (name == 'message') {
	      var self = this;
	      /* Register a callback to that executes when
	       * chrome injects the response. */
	      window.addEventListener(
	          'message', self.onRequestUpdate_.bind(self, handler), false);
	    } else {
	      console.error('WrappedAuthenticatorPort only supports message');
	    }
	  };

	  /**
	   * Callback invoked  when a response is received from the Authenticator.
	   * @param function({data: Object}) callback
	   * @param {Object} message message Object
	   */
	  u2f.WrappedAuthenticatorPort_.prototype.onRequestUpdate_ =
	      function(callback, message) {
	    var messageObject = JSON.parse(message.data);
	    var intentUrl = messageObject['intentURL'];

	    var errorCode = messageObject['errorCode'];
	    var responseObject = null;
	    if (messageObject.hasOwnProperty('data')) {
	      responseObject = /** @type {Object} */ (
	          JSON.parse(messageObject['data']));
	    }

	    callback({'data': responseObject});
	  };

	  /**
	   * Base URL for intents to Authenticator.
	   * @const
	   * @private
	   */
	  u2f.WrappedAuthenticatorPort_.INTENT_URL_BASE_ =
	    'intent:#Intent;action=com.google.android.apps.authenticator.AUTHENTICATE';

	  /**
	   * Wrap the iOS client app with a MessagePort interface.
	   * @constructor
	   * @private
	   */
	  u2f.WrappedIosPort_ = function() {};

	  /**
	   * Launch the iOS client app request
	   * @param {Object} message
	   */
	  u2f.WrappedIosPort_.prototype.postMessage = function(message) {
	    var str = JSON.stringify(message);
	    var url = "u2f://auth?" + encodeURI(str);
	    location.replace(url);
	  };

	  /**
	   * Tells what type of port this is.
	   * @return {String} port type
	   */
	  u2f.WrappedIosPort_.prototype.getPortType = function() {
	    return "WrappedIosPort_";
	  };

	  /**
	   * Emulates the HTML 5 addEventListener interface.
	   * @param {string} eventName
	   * @param {function({data: Object})} handler
	   */
	  u2f.WrappedIosPort_.prototype.addEventListener = function(eventName, handler) {
	    var name = eventName.toLowerCase();
	    if (name !== 'message') {
	      console.error('WrappedIosPort only supports message');
	    }
	  };

	  /**
	   * Sets up an embedded trampoline iframe, sourced from the extension.
	   * @param {function(MessagePort)} callback
	   * @private
	   */
	  u2f.getIframePort_ = function(callback) {
	    // Create the iframe
	    var iframeOrigin = 'chrome-extension://' + u2f.EXTENSION_ID;
	    var iframe = document.createElement('iframe');
	    iframe.src = iframeOrigin + '/u2f-comms.html';
	    iframe.setAttribute('style', 'display:none');
	    document.body.appendChild(iframe);

	    var channel = new MessageChannel();
	    var ready = function(message) {
	      if (message.data == 'ready') {
	        channel.port1.removeEventListener('message', ready);
	        callback(channel.port1);
	      } else {
	        console.error('First event on iframe port was not "ready"');
	      }
	    };
	    channel.port1.addEventListener('message', ready);
	    channel.port1.start();

	    iframe.addEventListener('load', function() {
	      // Deliver the port to the iframe and initialize
	      iframe.contentWindow.postMessage('init', iframeOrigin, [channel.port2]);
	    });
	  };


	  //High-level JS API

	  /**
	   * Default extension response timeout in seconds.
	   * @const
	   */
	  u2f.EXTENSION_TIMEOUT_SEC = 30;

	  /**
	   * A singleton instance for a MessagePort to the extension.
	   * @type {MessagePort|u2f.WrappedChromeRuntimePort_}
	   * @private
	   */
	  u2f.port_ = null;

	  /**
	   * Callbacks waiting for a port
	   * @type {Array<function((MessagePort|u2f.WrappedChromeRuntimePort_))>}
	   * @private
	   */
	  u2f.waitingForPort_ = [];

	  /**
	   * A counter for requestIds.
	   * @type {number}
	   * @private
	   */
	  u2f.reqCounter_ = 0;

	  /**
	   * A map from requestIds to client callbacks
	   * @type {Object.<number,(function((u2f.Error|u2f.RegisterResponse))
	   *                       |function((u2f.Error|u2f.SignResponse)))>}
	   * @private
	   */
	  u2f.callbackMap_ = {};

	  /**
	   * Creates or retrieves the MessagePort singleton to use.
	   * @param {function((MessagePort|u2f.WrappedChromeRuntimePort_))} callback
	   * @private
	   */
	  u2f.getPortSingleton_ = function(callback) {
	    if (u2f.port_) {
	      callback(u2f.port_);
	    } else {
	      if (u2f.waitingForPort_.length == 0) {
	        u2f.getMessagePort(function(port) {
	          u2f.port_ = port;
	          u2f.port_.addEventListener('message',
	              /** @type {function(Event)} */ (u2f.responseHandler_));

	          // Careful, here be async callbacks. Maybe.
	          while (u2f.waitingForPort_.length)
	            u2f.waitingForPort_.shift()(u2f.port_);
	        });
	      }
	      u2f.waitingForPort_.push(callback);
	    }
	  };

	  /**
	   * Handles response messages from the extension.
	   * @param {MessageEvent.<u2f.Response>} message
	   * @private
	   */
	  u2f.responseHandler_ = function(message) {
	    var response = message.data;
	    var reqId = response['requestId'];
	    if (!reqId || !u2f.callbackMap_[reqId]) {
	      console.error('Unknown or missing requestId in response.');
	      return;
	    }
	    var cb = u2f.callbackMap_[reqId];
	    delete u2f.callbackMap_[reqId];
	    cb(response['responseData']);
	  };

	  /**
	   * Dispatches an array of sign requests to available U2F tokens.
	   * If the JS API version supported by the extension is unknown, it first sends a
	   * message to the extension to find out the supported API version and then it sends
	   * the sign request.
	   * @param {string=} appId
	   * @param {string=} challenge
	   * @param {Array<u2f.RegisteredKey>} registeredKeys
	   * @param {function((u2f.Error|u2f.SignResponse))} callback
	   * @param {number=} opt_timeoutSeconds
	   */
	  u2f.sign = function(appId, challenge, registeredKeys, callback, opt_timeoutSeconds) {
	    if (js_api_version === undefined) {
	      // Send a message to get the extension to JS API version, then send the actual sign request.
	      u2f.getApiVersion(
	          function (response) {
	            js_api_version = response['js_api_version'] === undefined ? 0 : response['js_api_version'];
	            console.log("Extension JS API Version: ", js_api_version);
	            u2f.sendSignRequest(appId, challenge, registeredKeys, callback, opt_timeoutSeconds);
	          });
	    } else {
	      // We know the JS API version. Send the actual sign request in the supported API version.
	      u2f.sendSignRequest(appId, challenge, registeredKeys, callback, opt_timeoutSeconds);
	    }
	  };

	  /**
	   * Dispatches an array of sign requests to available U2F tokens.
	   * @param {string=} appId
	   * @param {string=} challenge
	   * @param {Array<u2f.RegisteredKey>} registeredKeys
	   * @param {function((u2f.Error|u2f.SignResponse))} callback
	   * @param {number=} opt_timeoutSeconds
	   */
	  u2f.sendSignRequest = function(appId, challenge, registeredKeys, callback, opt_timeoutSeconds) {
	    u2f.getPortSingleton_(function(port) {
	      var reqId = ++u2f.reqCounter_;
	      u2f.callbackMap_[reqId] = callback;
	      var timeoutSeconds = (typeof opt_timeoutSeconds !== 'undefined' ?
	          opt_timeoutSeconds : u2f.EXTENSION_TIMEOUT_SEC);
	      var req = u2f.formatSignRequest_(appId, challenge, registeredKeys, timeoutSeconds, reqId);
	      port.postMessage(req);
	    });
	  };

	  /**
	   * Dispatches register requests to available U2F tokens. An array of sign
	   * requests identifies already registered tokens.
	   * If the JS API version supported by the extension is unknown, it first sends a
	   * message to the extension to find out the supported API version and then it sends
	   * the register request.
	   * @param {string=} appId
	   * @param {Array<u2f.RegisterRequest>} registerRequests
	   * @param {Array<u2f.RegisteredKey>} registeredKeys
	   * @param {function((u2f.Error|u2f.RegisterResponse))} callback
	   * @param {number=} opt_timeoutSeconds
	   */
	  u2f.register = function(appId, registerRequests, registeredKeys, callback, opt_timeoutSeconds) {
	    if (js_api_version === undefined) {
	      // Send a message to get the extension to JS API version, then send the actual register request.
	      u2f.getApiVersion(
	          function (response) {
	            js_api_version = response['js_api_version'] === undefined ? 0: response['js_api_version'];
	            console.log("Extension JS API Version: ", js_api_version);
	            u2f.sendRegisterRequest(appId, registerRequests, registeredKeys,
	                callback, opt_timeoutSeconds);
	          });
	    } else {
	      // We know the JS API version. Send the actual register request in the supported API version.
	      u2f.sendRegisterRequest(appId, registerRequests, registeredKeys,
	          callback, opt_timeoutSeconds);
	    }
	  };

	  /**
	   * Dispatches register requests to available U2F tokens. An array of sign
	   * requests identifies already registered tokens.
	   * @param {string=} appId
	   * @param {Array<u2f.RegisterRequest>} registerRequests
	   * @param {Array<u2f.RegisteredKey>} registeredKeys
	   * @param {function((u2f.Error|u2f.RegisterResponse))} callback
	   * @param {number=} opt_timeoutSeconds
	   */
	  u2f.sendRegisterRequest = function(appId, registerRequests, registeredKeys, callback, opt_timeoutSeconds) {
	    u2f.getPortSingleton_(function(port) {
	      var reqId = ++u2f.reqCounter_;
	      u2f.callbackMap_[reqId] = callback;
	      var timeoutSeconds = (typeof opt_timeoutSeconds !== 'undefined' ?
	          opt_timeoutSeconds : u2f.EXTENSION_TIMEOUT_SEC);
	      var req = u2f.formatRegisterRequest_(
	          appId, registeredKeys, registerRequests, timeoutSeconds, reqId);
	      port.postMessage(req);
	    });
	  };


	  /**
	   * Dispatches a message to the extension to find out the supported
	   * JS API version.
	   * If the user is on a mobile phone and is thus using Google Authenticator instead
	   * of the Chrome extension, don't send the request and simply return 0.
	   * @param {function((u2f.Error|u2f.GetJsApiVersionResponse))} callback
	   * @param {number=} opt_timeoutSeconds
	   */
	  u2f.getApiVersion = function(callback, opt_timeoutSeconds) {
	   u2f.getPortSingleton_(function(port) {
	     // If we are using Android Google Authenticator or iOS client app,
	     // do not fire an intent to ask which JS API version to use.
	     if (port.getPortType) {
	       var apiVersion;
	       switch (port.getPortType()) {
	         case 'WrappedIosPort_':
	         case 'WrappedAuthenticatorPort_':
	           apiVersion = 1.1;
	           break;

	         default:
	           apiVersion = 0;
	           break;
	       }
	       callback({ 'js_api_version': apiVersion });
	       return;
	     }
	      var reqId = ++u2f.reqCounter_;
	      u2f.callbackMap_[reqId] = callback;
	      var req = {
	        type: u2f.MessageTypes.U2F_GET_API_VERSION_REQUEST,
	        timeoutSeconds: (typeof opt_timeoutSeconds !== 'undefined' ?
	            opt_timeoutSeconds : u2f.EXTENSION_TIMEOUT_SEC),
	        requestId: reqId
	      };
	      port.postMessage(req);
	    });
	  };
	})();


/***/ },

/***/ 232:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
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

	var reactor = __webpack_require__(215);

	var _require = __webpack_require__(233),
	    TLPT_NOTIFICATIONS_ADD = _require.TLPT_NOTIFICATIONS_ADD;

	exports.default = {
	  showError: function showError() {
	    var title = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : 'Error';

	    dispatch({ isError: true, title: title });
	  },
	  showSuccess: function showSuccess() {
	    var title = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : 'SUCCESS';

	    dispatch({ isSuccess: true, title: title });
	  },
	  showInfo: function showInfo(text) {
	    var title = arguments.length > 1 && arguments[1] !== undefined ? arguments[1] : 'INFO';

	    dispatch({ isInfo: true, text: text, title: title });
	  },
	  showWarning: function showWarning(text) {
	    var title = arguments.length > 1 && arguments[1] !== undefined ? arguments[1] : 'WARNING';

	    dispatch({ isWarning: true, text: text, title: title });
	  }
	};


	function dispatch(msg) {
	  reactor.dispatch(TLPT_NOTIFICATIONS_ADD, msg);
	}
	module.exports = exports['default'];

/***/ },

/***/ 233:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _keymirror = __webpack_require__(224);

	var _keymirror2 = _interopRequireDefault(_keymirror);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	exports.default = (0, _keymirror2.default)({
	  TLPT_NOTIFICATIONS_ADD: null
	}); /*
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

	module.exports = exports['default'];

/***/ },

/***/ 234:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _keymirror = __webpack_require__(224);

	var _keymirror2 = _interopRequireDefault(_keymirror);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	exports.default = (0, _keymirror2.default)({
	  TLPT_APP_INIT: null,
	  TLPT_APP_FAILED: null,
	  TLPT_APP_READY: null,
	  TLPT_APP_SET_SITE_ID: null
	}); /*
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

	module.exports = exports['default'];

/***/ },

/***/ 235:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _keymirror = __webpack_require__(224);

	var _keymirror2 = _interopRequireDefault(_keymirror);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	exports.default = (0, _keymirror2.default)({
	  TLPT_SITES_RECEIVE: null
	}); /*
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

	module.exports = exports['default'];

/***/ },

/***/ 236:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	var reactor = __webpack_require__(215);

	var _require = __webpack_require__(237),
	    TLPT_NODES_RECEIVE = _require.TLPT_NODES_RECEIVE;

	var api = __webpack_require__(228);
	var cfg = __webpack_require__(217);

	var _require2 = __webpack_require__(232),
	    showError = _require2.showError;

	var appGetters = __webpack_require__(238);

	var logger = __webpack_require__(230).create('Modules/Nodes');

	exports.default = {
	  fetchNodes: function fetchNodes() {
	    var siteId = reactor.evaluate(appGetters.siteId);
	    return api.get(cfg.api.getSiteNodesUrl(siteId)).done(function () {
	      var data = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : [];

	      var nodeArray = data.nodes.map(function (item) {
	        return item.node;
	      });

	      nodeArray.forEach(function (item) {
	        return item.siteId = siteId;
	      });
	      reactor.dispatch(TLPT_NODES_RECEIVE, { siteId: siteId, nodeArray: nodeArray });
	    }).fail(function (err) {
	      showError('Unable to retrieve list of nodes');
	      logger.error('fetchNodes', err);
	    });
	  }
	};
	module.exports = exports['default'];

/***/ },

/***/ 237:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _keymirror = __webpack_require__(224);

	var _keymirror2 = _interopRequireDefault(_keymirror);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	exports.default = (0, _keymirror2.default)({
	  TLPT_NODES_RECEIVE: null
	}); /*
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

	module.exports = exports['default'];

/***/ },

/***/ 238:
/***/ function(module, exports) {

	'use strict';

	exports.__esModule = true;
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

	var appStatus = [['tlpt', 'status'], function (app) {
	  return app.toJS();
	}];

	exports.default = {
	  appStatus: appStatus,
	  siteId: ['tlpt', 'siteId']
	};
	module.exports = exports['default'];

/***/ },

/***/ 239:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
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

	var reactor = __webpack_require__(215);
	var api = __webpack_require__(228);
	var cfg = __webpack_require__(217);

	var _require = __webpack_require__(232),
	    showError = _require.showError;

	var moment = __webpack_require__(240);
	var appGetters = __webpack_require__(238);

	var logger = __webpack_require__(230).create('Modules/Sessions');

	var _require2 = __webpack_require__(340),
	    TLPT_SESSIONS_UPDATE = _require2.TLPT_SESSIONS_UPDATE,
	    TLPT_SESSIONS_UPDATE_WITH_EVENTS = _require2.TLPT_SESSIONS_UPDATE_WITH_EVENTS,
	    TLPT_SESSIONS_RECEIVE = _require2.TLPT_SESSIONS_RECEIVE;

	var actions = {
	  fetchStoredSession: function fetchStoredSession(sid) {
	    var siteId = reactor.evaluate(appGetters.siteId);
	    return api.get(cfg.api.getSessionEventsUrl({ siteId: siteId, sid: sid })).then(function (json) {
	      if (json && json.events) {
	        reactor.dispatch(TLPT_SESSIONS_UPDATE_WITH_EVENTS, { siteId: siteId, jsonEvents: json.events });
	      }
	    });
	  },
	  fetchSiteEvents: function fetchSiteEvents(start, end) {
	    // default values
	    start = start || moment(new Date()).endOf('day').toDate();
	    end = end || moment(end).subtract(3, 'day').startOf('day').toDate();

	    start = start.toISOString();
	    end = end.toISOString();

	    var siteId = reactor.evaluate(appGetters.siteId);
	    return api.get(cfg.api.getSiteEventsFilterUrl({ start: start, end: end, siteId: siteId })).done(function (json) {
	      if (json && json.events) {
	        reactor.dispatch(TLPT_SESSIONS_UPDATE_WITH_EVENTS, { siteId: siteId, jsonEvents: json.events });
	      }
	    }).fail(function (err) {
	      showError('Unable to retrieve site events');
	      logger.error('fetchSiteEvents', err);
	    });
	  },
	  fetchActiveSessions: function fetchActiveSessions() {
	    var siteId = reactor.evaluate(appGetters.siteId);
	    return api.get(cfg.api.getFetchSessionsUrl(siteId)).done(function (json) {
	      var sessions = json.sessions || [];
	      sessions.forEach(function (s) {
	        return s.siteId = siteId;
	      });
	      reactor.dispatch(TLPT_SESSIONS_RECEIVE, sessions);
	    }).fail(function (err) {
	      showError('Unable to retrieve list of sessions');
	      logger.error('fetchActiveSessions', err);
	    });
	  },
	  updateSession: function updateSession(json) {
	    reactor.dispatch(TLPT_SESSIONS_UPDATE, json);
	  }
	};

	exports.default = actions;
	module.exports = exports['default'];

/***/ },

/***/ 340:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _keymirror = __webpack_require__(224);

	var _keymirror2 = _interopRequireDefault(_keymirror);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	exports.default = (0, _keymirror2.default)({
	  TLPT_SESSIONS_RECEIVE: null,
	  TLPT_SESSIONS_UPDATE: null,
	  TLPT_SESSIONS_UPDATE_WITH_EVENTS: null
	}); /*
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

	module.exports = exports['default'];

/***/ },

/***/ 341:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
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

	var React = __webpack_require__(2);
	var logoSvg = __webpack_require__(342);
	var classnames = __webpack_require__(346);

	var TeleportLogo = function TeleportLogo() {
	  return React.createElement(
	    'svg',
	    { className: 'grv-icon-logo-tlpt' },
	    React.createElement('use', { xlinkHref: logoSvg })
	  );
	};

	var UserIcon = function UserIcon(_ref) {
	  var _ref$name = _ref.name,
	      name = _ref$name === undefined ? '' : _ref$name,
	      isDark = _ref.isDark;

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

/***/ },

/***/ 342:
/***/ function(module, exports, __webpack_require__) {

	;
	var sprite = __webpack_require__(343);;
	var image = "<symbol viewBox=\"0 0 340 100\" id=\"grv-tlpt-logo-full\" xmlns:xlink=\"http://www.w3.org/1999/xlink\"> <g> <g id=\"grv-tlpt-logo-full_Layer_2\"> <g> <g> <path d=\"m47.671001,21.444c-7.396,0 -14.102001,3.007999 -18.960003,7.866001c-4.856998,4.856998 -7.865999,11.563 -7.865999,18.959999c0,7.396 3.008001,14.101002 7.865999,18.957996s11.564003,7.865005 18.960003,7.865005s14.102001,-3.008003 18.958996,-7.865005s7.865005,-11.561996 7.865005,-18.957996s-3.008003,-14.104 -7.865005,-18.959999c-4.857994,-4.858002 -11.562996,-7.866001 -18.958996,-7.866001zm11.386997,19.509998h-8.213997v23.180004h-6.344002v-23.180004h-8.215v-5.612h22.772999v5.612l0,0z\"/> </g> <g> <path d=\"m92.782997,63.357002c-0.098999,-0.371002 -0.320999,-0.709 -0.646996,-0.942001l-4.562004,-3.958l-4.561996,-3.957001c0.163002,-0.887001 0.267998,-1.805 0.331001,-2.736c0.063995,-0.931 0.086998,-1.874001 0.086998,-2.805c0,-0.932999 -0.022003,-1.875 -0.086998,-2.806999c-0.063004,-0.931999 -0.167999,-1.851002 -0.331001,-2.736l4.561996,-3.957001l4.562004,-3.958c0.325996,-0.232998 0.548996,-0.57 0.646996,-0.942001c0.099007,-0.372997 0.075005,-0.778999 -0.087997,-1.153c-0.931999,-2.862 -2.199997,-5.655998 -3.731003,-8.299c-1.530998,-2.641998 -3.321999,-5.132998 -5.301994,-7.390999c-0.278999,-0.326 -0.617004,-0.548 -0.978004,-0.646c-0.360001,-0.098999 -0.744995,-0.074999 -1.116997,0.087l-5.750999,2.002001l-5.749001,2.000999c-1.419998,-1.164 -2.933998,-2.211 -4.522003,-3.136999c-1.589996,-0.925001 -3.253998,-1.728001 -4.977997,-2.404001l-1.139999,-5.959l-1.140999,-5.959c-0.069,-0.373 -0.268005,-0.733 -0.547005,-1.013c-0.278999,-0.28 -0.640999,-0.478 -1.036995,-0.524c-2.980003,-0.605 -6.007004,-0.908 -9.033005,-0.908s-6.052998,0.302 -9.032997,0.908c-0.396,0.046 -0.756001,0.245001 -1.036003,0.524c-0.278999,0.279 -0.477997,0.64 -0.546997,1.013l-1.141003,5.959l-1.140999,5.960001c-1.723,0.675999 -3.410999,1.479 -5.012001,2.403999c-1.599998,0.924999 -3.112999,1.973 -4.487,3.136999l-5.75,-2.000999l-5.75,-2.001999c-0.372,-0.164001 -0.755999,-0.187 -1.116999,-0.088001c-0.361,0.1 -0.699001,0.32 -0.978001,0.646c-1.979,2.259001 -3.771,4.75 -5.302,7.392002c-1.53,2.641998 -2.799,5.436996 -3.73,8.299c-0.163,0.372997 -0.187,0.780998 -0.087001,1.151997c0.099,0.372002 0.320001,0.710003 0.646001,0.943001l4.563,3.957001l4.562,3.958c-0.163,0.884998 -0.268,1.804001 -0.331001,2.735001c-0.063999,0.931999 -0.087999,1.875 -0.087999,2.806s0.023001,1.875 0.087,2.806c0.064001,0.931999 0.168001,1.851002 0.332001,2.735001l-4.562,3.957001l-4.562,3.959c-0.325,0.231003 -0.547,0.569 -0.646,0.942001c-0.099,0.370995 -0.076,0.778999 0.087,1.150002c0.931,2.864998 2.2,5.657997 3.73,8.300995c1.531,2.642998 3.323,5.133003 5.302,7.391998c0.280001,0.325005 0.618,0.548004 0.978001,0.646004c0.361,0.099998 0.744999,0.074997 1.118,-0.087997l5.75,-2.003006l5.749998,-2.000999c1.373001,1.164001 2.886002,2.213005 4.487003,3.139c1.600998,0.924004 3.288998,1.728004 5.010998,2.401001l1.140999,5.961998l1.141003,5.959c0.07,0.372002 0.267998,0.733002 0.547001,1.014c0.278999,0.279007 0.640999,0.479004 1.035999,0.522003c1.489998,0.278 2.979,0.500999 4.480999,0.651001c1.500999,0.152 3.014999,0.232002 4.551998,0.232002s3.049004,-0.080002 4.551003,-0.232002c1.501999,-0.150002 2.990997,-0.373001 4.479996,-0.651001c0.396004,-0.044998 0.757004,-0.243996 1.037003,-0.522003c0.279999,-0.278999 0.476997,-0.641998 0.547005,-1.014l1.140999,-5.959l1.140999,-5.961998c1.723,-0.674995 3.387001,-1.477997 4.976997,-2.401001c1.588005,-0.925995 3.103004,-1.974998 4.522003,-3.139l5.75,2.000999l5.75,2.003006c0.373001,0.162994 0.756996,0.185997 1.117996,0.087997c0.360001,-0.098999 0.698006,-0.32 0.978004,-0.646004c1.978996,-2.258995 3.770996,-4.749001 5.301994,-7.391998c1.531006,-2.642998 2.800003,-5.436996 3.731003,-8.300995c0.164001,-0.368004 0.188004,-0.778008 0.087997,-1.150002zm-24.237999,5.787994c-5.348,5.349007 -12.731995,8.660004 -20.875,8.660004c-8.143997,0 -15.526997,-3.312004 -20.875,-8.660004s-8.659998,-12.730995 -8.659998,-20.874996c0,-8.144001 3.312,-15.527 8.661001,-20.875999c5.348,-5.348001 12.731998,-8.661001 20.875999,-8.661001c8.143002,0 15.525997,3.312 20.874996,8.661001c5.348,5.348999 8.661003,12.731998 8.661003,20.875999c-0.000999,8.141998 -3.314003,15.525997 -8.663002,20.874996z\"/> </g> </g> </g> <g> <path d=\"m119.773003,30.861h-13.020004v-6.841h33.599998v6.841h-13.020004v35.639999h-7.55999v-35.639999l0,0z\"/> <path d=\"m143.953003,54.620998c0.23999,2.16 1.080002,3.84 2.520004,5.039997s3.179993,1.800003 5.219986,1.800003c1.800003,0 3.309006,-0.368996 4.530014,-1.110001c1.219986,-0.738998 2.289993,-1.668999 3.209991,-2.790001l5.160004,3.900002c-1.680008,2.080002 -3.561005,3.561005 -5.639999,4.440002c-2.080002,0.878998 -4.26001,1.319 -6.540009,1.319c-2.159988,0 -4.199997,-0.359001 -6.119995,-1.080002c-1.919998,-0.720001 -3.580994,-1.738998 -4.979996,-3.059998c-1.401001,-1.320007 -2.511002,-2.910004 -3.330002,-4.771004c-0.820007,-1.858997 -1.229996,-3.929996 -1.229996,-6.209999c0,-2.278999 0.409988,-4.349998 1.229996,-6.209999c0.819,-1.859001 1.929001,-3.449001 3.330002,-4.77c1.399002,-1.32 3.059998,-2.34 4.979996,-3.061001c1.919998,-0.719997 3.960007,-1.078999 6.119995,-1.078999c2,0 3.830002,0.351002 5.490005,1.049999c1.658997,0.700001 3.080002,1.709999 4.259995,3.028999c1.180008,1.32 2.100006,2.951 2.76001,4.891003c0.659988,1.939999 0.98999,4.169998 0.98999,6.688999v1.98h-21.959991l0,0.002998zm14.759995,-5.399998c-0.041,-2.118999 -0.699997,-3.789001 -1.979996,-5.010002c-1.281006,-1.219997 -3.059998,-1.829998 -5.339996,-1.829998c-2.160004,0 -3.87001,0.620998 -5.130005,1.860001c-1.259995,1.239998 -2.031006,2.899998 -2.309998,4.979h14.759995l0,0.000999z\"/> <path d=\"m172.753006,21.141001h7.199997v45.359999h-7.199997v-45.359999l0,0z\"/> <path d=\"m193.992004,54.620998c0.23999,2.16 1.080002,3.84 2.519989,5.039997c1.440002,1.200005 3.181,1.800003 5.221008,1.800003c1.800003,0 3.309006,-0.368996 4.528992,-1.110001c1.221008,-0.738998 2.290009,-1.668999 3.211014,-2.790001l5.159988,3.900002c-1.681,2.080002 -3.560989,3.561005 -5.640991,4.440002c-2.080002,0.878998 -4.26001,1.319 -6.540009,1.319c-2.158997,0 -4.199997,-0.359001 -6.119995,-1.080002c-1.919998,-0.720001 -3.580002,-1.738998 -4.979004,-3.059998c-1.401001,-1.320007 -2.511002,-2.910004 -3.330002,-4.771004c-0.819992,-1.858997 -1.228989,-3.929996 -1.228989,-6.209999c0,-2.278999 0.408997,-4.349998 1.228989,-6.209999c0.819,-1.859001 1.929001,-3.449001 3.330002,-4.77c1.399002,-1.32 3.059998,-2.34 4.979004,-3.061001c1.919998,-0.719997 3.960999,-1.078999 6.119995,-1.078999c2,0 3.830002,0.351002 5.490005,1.049999c1.658997,0.700001 3.078995,1.709999 4.259995,3.028999c1.180008,1.32 2.100998,2.951 2.761002,4.891003c0.660004,1.939999 0.988998,4.169998 0.988998,6.688999v1.98h-21.959991l0,0.002998zm14.759995,-5.399998c-0.039993,-2.118999 -0.699005,-3.789001 -1.979004,-5.010002c-1.279999,-1.219997 -3.059998,-1.829998 -5.340988,-1.829998c-2.159012,0 -3.869003,0.620998 -5.129013,1.860001c-1.259995,1.239998 -2.030991,2.899998 -2.310989,4.979h14.759995l0,0.000999z\"/> <path d=\"m222.671997,37.701h6.839996v4.319h0.12001c1.039993,-1.758999 2.438995,-3.039001 4.199997,-3.84c1.759995,-0.799999 3.660004,-1.199001 5.699005,-1.199001c2.19899,0 4.179993,0.389999 5.939987,1.170002c1.76001,0.778999 3.260025,1.850998 4.500015,3.209999c1.239014,1.360001 2.179993,2.959999 2.820007,4.799999c0.639984,1.84 0.959991,3.82 0.959991,5.938999c0,2.121002 -0.339996,4.101002 -1.019989,5.940002c-0.682007,1.840004 -1.631012,3.440002 -2.851013,4.800003c-1.221008,1.359993 -2.690002,2.43 -4.410004,3.209999s-3.600998,1.169998 -5.639999,1.169998c-1.360001,0 -2.561005,-0.140999 -3.600006,-0.420006c-1.041,-0.279991 -1.960999,-0.639992 -2.761002,-1.079994c-0.799988,-0.439003 -1.478989,-0.909004 -2.039993,-1.410004c-0.561005,-0.499001 -1.020004,-0.988998 -1.380005,-1.469994h-0.181v17.339996h-7.19899v-42.479l0.002991,0zm23.880005,14.400002c0,-1.119003 -0.190002,-2.199001 -0.569,-3.239002c-0.380997,-1.040001 -0.940994,-1.959999 -1.681,-2.760998c-0.740997,-0.799004 -1.630005,-1.439003 -2.669998,-1.920002c-1.040009,-0.479 -2.220001,-0.720001 -3.540009,-0.720001s-2.5,0.240002 -3.539993,0.720001c-1.040009,0.48 -1.931,1.120998 -2.669998,1.920002c-0.740997,0.800999 -1.300003,1.720997 -1.681,2.760998c-0.380005,1.040001 -0.569,2.119999 -0.569,3.239002c0,1.120998 0.188995,2.200996 0.569,3.239998c0.380997,1.041 0.938995,1.960995 1.681,2.759998c0.738998,0.801003 1.62999,1.440002 2.669998,1.919998c1.039993,0.480003 2.220001,0.721001 3.539993,0.721001s2.5,-0.239998 3.540009,-0.721001c1.039993,-0.478996 1.929001,-1.118996 2.669998,-1.919998c0.738998,-0.799004 1.300003,-1.718998 1.681,-2.759998c0.377991,-1.039001 0.569,-2.118999 0.569,-3.239998z\"/> <path d=\"m259.031006,52.101002c0,-2.279003 0.410004,-4.350002 1.230011,-6.210003c0.817993,-1.858997 1.928986,-3.448997 3.329987,-4.77c1.39801,-1.32 3.059021,-2.34 4.979004,-3.060997c1.920013,-0.720001 3.959991,-1.079002 6.119995,-1.079002s4.199005,0.359001 6.119019,1.079002c1.919983,0.720997 3.579987,1.739998 4.97998,3.060997s2.51001,2.91 3.330017,4.77c0.819977,1.860001 1.22998,3.931 1.22998,6.210003c0,2.279999 -0.410004,4.350998 -1.22998,6.210003c-0.820007,1.860001 -1.930023,3.449997 -3.330017,4.770996s-3.061005,2.340004 -4.97998,3.059998c-1.920013,0.721001 -3.959015,1.080002 -6.119019,1.080002s-4.199982,-0.359001 -6.119995,-1.080002c-1.92099,-0.719994 -3.580994,-1.738998 -4.979004,-3.059998c-1.401001,-1.32 -2.511993,-2.909996 -3.329987,-4.770996c-0.820007,-1.860004 -1.230011,-3.930004 -1.230011,-6.210003zm7.199005,0c0,1.120998 0.188995,2.200996 0.570007,3.239998c0.380005,1.041 0.938995,1.960995 1.679993,2.759998c0.73999,0.801003 1.630005,1.440002 2.670013,1.919998c1.040985,0.480003 2.220978,0.721001 3.540985,0.721001s2.498993,-0.239998 3.539001,-0.721001c1.040985,-0.478996 1.929993,-1.118996 2.670013,-1.919998c0.73999,-0.799004 1.300995,-1.718998 1.681976,-2.759998c0.378998,-1.039001 0.568024,-2.118999 0.568024,-3.239998c0,-1.119003 -0.189026,-2.199001 -0.568024,-3.239002c-0.380981,-1.040001 -0.940979,-1.959999 -1.681976,-2.760998c-0.740021,-0.799004 -1.629028,-1.439003 -2.670013,-1.920002c-1.040009,-0.479 -2.218994,-0.720001 -3.539001,-0.720001s-2.5,0.240002 -3.540985,0.720001c-1.040009,0.48 -1.930023,1.120998 -2.670013,1.920002c-0.73999,0.800999 -1.299988,1.720997 -1.679993,2.760998c-0.380005,1.039001 -0.570007,2.118999 -0.570007,3.239002z\"/> <path d=\"m297.070007,37.701h7.200989v4.560001h0.119019c0.798981,-1.68 1.938995,-2.979 3.419983,-3.899002s3.179993,-1.380001 5.100006,-1.380001c0.438995,0 0.871002,0.040001 1.290985,0.119003c0.420013,0.080997 0.850006,0.181 1.289001,0.300999v6.959999c-0.599976,-0.16 -1.188995,-0.290001 -1.769989,-0.390999c-0.579987,-0.098999 -1.149994,-0.149002 -1.710999,-0.149002c-1.679993,0 -3.028992,0.310001 -4.049011,0.93c-1.019989,0.621002 -1.800995,1.330002 -2.339996,2.130001c-0.540985,0.800999 -0.899994,1.601002 -1.079987,2.400002c-0.180023,0.800999 -0.27002,1.399998 -0.27002,1.799999v15.419998h-7.200989v-28.800999l0.001007,0z\"/> <path d=\"m317.049011,43.820999v-6.119999h5.940979v-8.34h7.199005v8.34h7.920013v6.119999h-7.920013v12.600002c0,1.439999 0.27002,2.579998 0.811005,3.420002c0.539001,0.839996 1.609009,1.259995 3.209015,1.259995c0.640991,0 1.339996,-0.069 2.10199,-0.209999c0.757996,-0.139999 1.359009,-0.369003 1.798981,-0.689003v6.060005c-0.759979,0.360001 -1.688995,0.608994 -2.788971,0.75c-1.10202,0.139999 -2.070007,0.209999 -2.910004,0.209999c-1.920013,0 -3.490021,-0.209999 -4.710999,-0.630005s-2.180023,-1.059998 -2.878998,-1.919998c-0.701019,-0.859001 -1.182007,-1.93 -1.44101,-3.209991c-0.26001,-1.279007 -0.389008,-2.76001 -0.389008,-4.440002v-13.201004h-5.941986l0,0z\"/> </g> <g> <path d=\"m119.194,86.295998h3.587997c0.346001,0 0.689003,0.041 1.027,0.124001c0.338005,0.082001 0.639,0.217003 0.903,0.402c0.264,0.187004 0.479004,0.427002 0.644005,0.722s0.246994,0.650002 0.246994,1.066002c0,0.519997 -0.146996,0.947998 -0.441994,1.287003c-0.295006,0.337997 -0.681,0.579994 -1.157005,0.727997v0.026001c0.286003,0.033997 0.553001,0.113998 0.800003,0.239998c0.247002,0.125999 0.457001,0.286003 0.629997,0.480003c0.173004,0.195 0.310005,0.420998 0.409004,0.676994s0.149994,0.530006 0.149994,0.825005c0,0.502998 -0.099998,0.920998 -0.298996,1.254997c-0.198997,0.333 -0.460999,0.603004 -0.786003,0.806c-0.324997,0.204002 -0.697998,0.348999 -1.117996,0.436005s-0.848,0.129997 -1.280998,0.129997h-3.315002v-9.204002l0,0zm1.638,3.744003h1.495003c0.545998,0 0.955994,-0.106003 1.228996,-0.318001c0.273003,-0.212997 0.408997,-0.491997 0.408997,-0.838997c0,-0.398003 -0.140999,-0.695 -0.421997,-0.891006c-0.281998,-0.194 -0.734001,-0.292 -1.358002,-0.292h-1.351997v2.340004l-0.000999,0zm0,4.056h1.507996c0.208,0 0.431007,-0.013 0.669006,-0.039001c0.237999,-0.025002 0.457001,-0.085999 0.656998,-0.181999c0.198997,-0.096001 0.363998,-0.231003 0.494003,-0.408997c0.129997,-0.178001 0.195,-0.418007 0.195,-0.722c0,-0.485001 -0.158005,-0.823006 -0.475006,-1.014c-0.315994,-0.191002 -0.807999,-0.286003 -1.475998,-0.286003h-1.572998v2.652l0.000999,0z\"/> <path d=\"m130.854996,91.560997l-3.457993,-5.264999h2.054001l2.261993,3.666l2.28801,-3.666h1.949997l-3.458008,5.264999v3.939003h-1.638v-3.939003l0,0z\"/> <path d=\"m150.796997,94.823997c-1.136002,0.606003 -2.404999,0.910004 -3.80899,0.910004c-0.711014,0 -1.363007,-0.114998 -1.957001,-0.345001s-1.105011,-0.555 -1.534012,-0.975998c-0.429001,-0.420006 -0.764999,-0.925003 -1.006989,-1.514c-0.243011,-0.590004 -0.363998,-1.244003 -0.363998,-1.964005c0,-0.736 0.120987,-1.404999 0.363998,-2.007996s0.578995,-1.116005 1.006989,-1.541c0.429001,-0.424004 0.940002,-0.750999 1.534012,-0.981003c0.593994,-0.228996 1.245987,-0.345001 1.957001,-0.345001c0.701996,0 1.360001,0.084999 1.975998,0.254005c0.61499,0.168999 1.166,0.471001 1.651001,0.903l-1.209,1.223c-0.295013,-0.286003 -0.652008,-0.508003 -1.072006,-0.663002c-0.421005,-0.155998 -0.865005,-0.234001 -1.332993,-0.234001c-0.477005,0 -0.908005,0.084999 -1.294006,0.253998c-0.384995,0.169006 -0.716995,0.402 -0.994003,0.701004c-0.276993,0.299995 -0.492004,0.648003 -0.643997,1.046997c-0.151993,0.398003 -0.227997,0.828003 -0.227997,1.287003c0,0.493996 0.076004,0.948997 0.227997,1.364998c0.151001,0.416 0.365997,0.775002 0.643997,1.079002c0.277008,0.303001 0.609009,0.541 0.994003,0.714996c0.386002,0.173004 0.817001,0.260002 1.294006,0.260002c0.416,0 0.807999,-0.039001 1.175995,-0.116997c0.367996,-0.078003 0.694992,-0.199005 0.981003,-0.362999v-2.171005h-1.88501v-1.480995h3.52301v4.704994l0.000992,0z\"/> <path d=\"m153.722,86.295998h3.197998c0.442001,0 0.869003,0.041 1.279999,0.124001c0.412003,0.082001 0.778,0.223 1.098999,0.422005c0.320007,0.198997 0.576004,0.467995 0.766998,0.806999c0.190002,0.337997 0.286011,0.766998 0.286011,1.285995c0,0.667999 -0.184998,1.227005 -0.553009,1.678001c-0.369003,0.450005 -0.894989,0.723999 -1.580002,0.818001l2.445007,4.069h-1.975998l-2.132004,-3.900002h-1.195999v3.900002h-1.638v-9.204002l0,0zm2.912003,3.900002c0.233994,0 0.468002,-0.011002 0.701996,-0.032997c0.234009,-0.021004 0.447998,-0.073006 0.643997,-0.154999c0.195007,-0.083 0.352997,-0.208 0.473999,-0.377007c0.122009,-0.168999 0.182007,-0.404999 0.182007,-0.709c0,-0.268997 -0.056,-0.485001 -0.169006,-0.648994c-0.112991,-0.165001 -0.259995,-0.288002 -0.442001,-0.371002c-0.181992,-0.082001 -0.383987,-0.137001 -0.603989,-0.162003c-0.221008,-0.026001 -0.436005,-0.039001 -0.644012,-0.039001h-1.416992v2.496002h1.274002l0,-0.000999z\"/> <path d=\"m165.876007,86.295998h1.416992l3.966003,9.204002h-1.872009l-0.857986,-2.106003h-3.991013l-0.832001,2.106003h-1.832993l4.003006,-9.204002zm2.080994,5.694l-1.417007,-3.743996l-1.442993,3.743996h2.860001l0,0z\"/> <path d=\"m171.401001,86.295998h1.884995l2.509003,6.955002l2.587006,-6.955002h1.76799l-3.716995,9.204002h-1.416992l-3.615005,-9.204002z\"/> <path d=\"m182.087006,86.295998h1.638v9.204002h-1.638v-9.204002l0,0z\"/> <path d=\"m188.613007,87.778h-2.820999v-1.482002h7.279999v1.482002h-2.820999v7.722h-1.638v-7.722l0,0z\"/> <path d=\"m196.959,86.295998h1.417007l3.965988,9.204002h-1.873001l-0.856995,-2.106003h-3.990997l-0.833008,2.106003h-1.832993l4.003998,-9.204002zm2.080002,5.694l-1.417007,-3.743996l-1.442001,3.743996h2.859009l0,0z\"/> <path d=\"m205.044998,87.778h-2.819992v-1.482002h7.278992v1.482002h-2.819992v7.722h-1.639008v-7.722l0,0z\"/> <path d=\"m211.570007,86.295998h1.638992v9.204002h-1.638992v-9.204002l0,0z\"/> <path d=\"m215.718994,90.936996c0,-0.736 0.121002,-1.404999 0.362991,-2.007996s0.578003,-1.115997 1.008011,-1.541c0.429001,-0.424004 0.938995,-0.750999 1.53299,-0.981003c0.594009,-0.228996 1.246002,-0.345001 1.957001,-0.345001c0.719009,-0.007996 1.378006,0.098007 1.977005,0.319c0.597992,0.221001 1.112991,0.544006 1.546997,0.968002c0.432999,0.425003 0.770996,0.937004 1.014008,1.534004c0.241989,0.598999 0.362991,1.265999 0.362991,2.001999c0,0.720001 -0.121002,1.374001 -0.362991,1.962997c-0.242004,0.590004 -0.581009,1.097 -1.014008,1.521004c-0.434006,0.424995 -0.949005,0.755997 -1.546997,0.993996c-0.598999,0.237999 -1.257996,0.362 -1.977005,0.371002c-0.710999,0 -1.362991,-0.114998 -1.957001,-0.345001s-1.103989,-0.555 -1.53299,-0.975998c-0.430008,-0.420006 -0.766006,-0.925003 -1.008011,-1.514c-0.241989,-0.588005 -0.362991,-1.243004 -0.362991,-1.962006zm1.715012,-0.103996c0,0.494003 0.076004,0.948997 0.229004,1.364998c0.149994,0.416 0.365005,0.775002 0.643005,1.079002c0.276993,0.303001 0.608994,0.541 0.993988,0.714996c0.387009,0.173004 0.817001,0.260002 1.295013,0.260002c0.47699,0 0.908997,-0.086998 1.298996,-0.260002c0.390991,-0.173996 0.724991,-0.411995 1.001999,-0.714996c0.276993,-0.304001 0.490997,-0.663002 0.643005,-1.079002c0.151993,-0.416 0.228989,-0.870995 0.228989,-1.364998c0,-0.459 -0.075989,-0.889 -0.228989,-1.287003c-0.151001,-0.397995 -0.365005,-0.746994 -0.643005,-1.046997c-0.277008,-0.299004 -0.611008,-0.531998 -1.001999,-0.701004c-0.389999,-0.168999 -0.822006,-0.253998 -1.298996,-0.253998c-0.478012,0 -0.908005,0.084999 -1.295013,0.253998c-0.384995,0.169006 -0.716995,0.402 -0.993988,0.701004c-0.277008,0.300003 -0.492004,0.648003 -0.643005,1.046997c-0.153015,0.398003 -0.229004,0.828003 -0.229004,1.287003z\"/> <path d=\"m228.029007,86.295998h2.17099l4.459,6.838005h0.026001v-6.838005h1.637009v9.204002h-2.07901l-4.550003,-7.058998h-0.025986v7.058998h-1.638v-9.204002l0,0z\"/> <path d=\"m242.341995,86.295998h1.417007l3.966003,9.204002h-1.873001l-0.85701,-2.106003h-3.990997l-0.832993,2.106003h-1.833008l4.003998,-9.204002zm2.080002,5.694l-1.416992,-3.743996l-1.442001,3.743996h2.858994l0,0z\"/> <path d=\"m249.738007,86.295998h1.638992v7.722h3.912003v1.482002h-5.550995v-9.204002l0,0z\"/> </g> </g> </symbol>";
	module.exports = sprite.add(image, "grv-tlpt-logo-full");

/***/ },

/***/ 343:
/***/ function(module, exports, __webpack_require__) {

	var Sprite = __webpack_require__(344);
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

/***/ 344:
/***/ function(module, exports, __webpack_require__) {

	var Sniffr = __webpack_require__(345);

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

/***/ 345:
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

/***/ 347:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

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
	module.exports.getters = __webpack_require__(238);
	module.exports.actions = __webpack_require__(226);
	module.exports.appStore = __webpack_require__(348);

/***/ },

/***/ 348:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

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
	var _require = __webpack_require__(216),
	    Store = _require.Store,
	    toImmutable = _require.toImmutable;

	var _require2 = __webpack_require__(234),
	    TLPT_APP_INIT = _require2.TLPT_APP_INIT,
	    TLPT_APP_FAILED = _require2.TLPT_APP_FAILED,
	    TLPT_APP_READY = _require2.TLPT_APP_READY,
	    TLPT_APP_SET_SITE_ID = _require2.TLPT_APP_SET_SITE_ID;

	var defaultStatus = toImmutable({
	  isReady: false,
	  isInitializing: false,
	  isFailed: false
	});

	exports.default = Store({
	  getInitialState: function getInitialState() {
	    return toImmutable({
	      siteId: undefined,
	      status: defaultStatus
	    });
	  },
	  initialize: function initialize() {
	    this.on(TLPT_APP_INIT, function (state) {
	      return state.set('status', defaultStatus.set('isInitializing', true));
	    });
	    this.on(TLPT_APP_READY, function (state) {
	      return state.set('status', defaultStatus.set('isReady', true));
	    });
	    this.on(TLPT_APP_FAILED, function (state) {
	      return state.set('status', defaultStatus.set('isFailed', true));
	    });
	    this.on(TLPT_APP_SET_SITE_ID, function (state, siteId) {
	      return state.set('siteId', siteId);
	    });
	  }
	});
	module.exports = exports['default'];

/***/ },

/***/ 349:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

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

	var React = __webpack_require__(2);
	var reactor = __webpack_require__(215);
	var PureRenderMixin = __webpack_require__(350);

	var _require = __webpack_require__(353),
	    lastMessage = _require.lastMessage;

	var _require2 = __webpack_require__(354),
	    ToastContainer = _require2.ToastContainer,
	    ToastMessage = _require2.ToastMessage;

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
	      ref: 'container', preventDuplicates: true, toastMessageFactory: ToastMessageFactory, className: 'toast-top-right' });
	  }
	});

	module.exports = NotificationHost;

/***/ },

/***/ 353:
/***/ function(module, exports) {

	'use strict';

	exports.__esModule = true;
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

	var lastMessage = exports.lastMessage = [['tlpt_notifications'], function (notifications) {
	    return notifications.last();
	}];

/***/ },

/***/ 364:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

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

	var React = __webpack_require__(2);

	var Timer = React.createClass({
	  displayName: 'Timer',
	  shouldComponentUpdate: function shouldComponentUpdate() {
	    return false;
	  },
	  componentWillMount: function componentWillMount() {
	    var _props = this.props,
	        onTimeout = _props.onTimeout,
	        _props$interval = _props.interval,
	        interval = _props$interval === undefined ? 2500 : _props$interval;

	    onTimeout();
	    this.refreshInterval = setInterval(onTimeout, interval);
	  },
	  componentWillUnmount: function componentWillUnmount() {
	    clearInterval(this.refreshInterval);
	  },
	  render: function render() {
	    return null;
	  }
	});

	module.exports = Timer;

/***/ },

/***/ 365:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

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

	var React = __webpack_require__(2);
	var $ = __webpack_require__(219);
	var reactor = __webpack_require__(215);
	var LinkedStateMixin = __webpack_require__(366);

	var _require = __webpack_require__(370),
	    actions = _require.actions,
	    getters = _require.getters;

	var GoogleAuthInfo = __webpack_require__(376);
	var cfg = __webpack_require__(217);

	var _require2 = __webpack_require__(341),
	    TeleportLogo = _require2.TeleportLogo;

	var _require3 = __webpack_require__(227),
	    SECOND_FACTOR_TYPE_HOTP = _require3.SECOND_FACTOR_TYPE_HOTP,
	    SECOND_FACTOR_TYPE_OIDC = _require3.SECOND_FACTOR_TYPE_OIDC,
	    SECOND_FACTOR_TYPE_U2F = _require3.SECOND_FACTOR_TYPE_U2F;

	var LoginInputForm = React.createClass({
	  displayName: 'LoginInputForm',


	  mixins: [LinkedStateMixin],

	  getInitialState: function getInitialState() {
	    return {
	      user: '',
	      password: '',
	      token: '',
	      provider: null,
	      secondFactorType: SECOND_FACTOR_TYPE_HOTP
	    };
	  },
	  onLogin: function onLogin(e) {
	    e.preventDefault();
	    this.state.secondFactorType = SECOND_FACTOR_TYPE_HOTP;
	    // token field is required for Google Authenticator
	    $('input[name=token]').addClass("required");
	    if (this.isValid()) {
	      this.props.onClick(this.state);
	    }
	  },


	  providerLogin: function providerLogin(provider) {
	    var self = this;
	    return function (e) {
	      e.preventDefault();
	      self.state.secondFactorType = SECOND_FACTOR_TYPE_OIDC;
	      self.state.provider = provider.id;
	      self.props.onClick(self.state);
	    };
	  },

	  onLoginWithU2f: function onLoginWithU2f(e) {
	    e.preventDefault();
	    this.state.secondFactorType = SECOND_FACTOR_TYPE_U2F;
	    // token field not required for U2F
	    $('input[name=token]').removeClass("required");
	    if (this.isValid()) {
	      this.props.onClick(this.state);
	    }
	  },

	  isValid: function isValid() {
	    var $form = $(this.refs.form);
	    return $form.length === 0 || $form.valid();
	  },

	  render: function render() {
	    var _this = this;

	    var _props$attemp = this.props.attemp,
	        isProcessing = _props$attemp.isProcessing,
	        isFailed = _props$attemp.isFailed,
	        message = _props$attemp.message;

	    var providers = cfg.getAuthProviders();
	    var useU2f = !!cfg.getU2fAppId();

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
	        useU2f ? React.createElement(
	          'button',
	          { onClick: this.onLoginWithU2f, disabled: isProcessing, type: 'submit', className: 'btn btn-primary block full-width m-b' },
	          'Login with U2F'
	        ) : null,
	        providers.map(function (provider) {
	          return React.createElement(
	            'button',
	            { onClick: _this.providerLogin(provider), type: 'submit', className: 'btn btn-danger block full-width m-b' },
	            'With ',
	            provider.display
	          );
	        }),
	        isProcessing && this.state.secondFactorType == SECOND_FACTOR_TYPE_U2F ? React.createElement(
	          'label',
	          { className: 'help-block' },
	          'Insert your U2F key and press the button on the key'
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

/***/ },

/***/ 370:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

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

	module.exports.getters = __webpack_require__(222);
	module.exports.actions = __webpack_require__(371);
	module.exports.nodeStore = __webpack_require__(375);

/***/ },

/***/ 371:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
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

	var reactor = __webpack_require__(215);

	var _require = __webpack_require__(372),
	    TLPT_RECEIVE_USER = _require.TLPT_RECEIVE_USER,
	    TLPT_RECEIVE_USER_INVITE = _require.TLPT_RECEIVE_USER_INVITE;

	var _require2 = __webpack_require__(223),
	    TRYING_TO_SIGN_UP = _require2.TRYING_TO_SIGN_UP,
	    TRYING_TO_LOGIN = _require2.TRYING_TO_LOGIN,
	    FETCHING_INVITE = _require2.FETCHING_INVITE;

	var restApiActions = __webpack_require__(373);
	var auth = __webpack_require__(227);
	var session = __webpack_require__(229);
	var cfg = __webpack_require__(217);
	var api = __webpack_require__(228);

	var _require3 = __webpack_require__(227),
	    SECOND_FACTOR_TYPE_OIDC = _require3.SECOND_FACTOR_TYPE_OIDC,
	    SECOND_FACTOR_TYPE_U2F = _require3.SECOND_FACTOR_TYPE_U2F;

	var actions = {
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
	    var name = _ref.name,
	        psw = _ref.psw,
	        token = _ref.token,
	        inviteToken = _ref.inviteToken,
	        secondFactorType = _ref.secondFactorType;

	    restApiActions.start(TRYING_TO_SIGN_UP);

	    var onSuccess = function onSuccess(sessionData) {
	      reactor.dispatch(TLPT_RECEIVE_USER, sessionData.user);
	      restApiActions.success(TRYING_TO_SIGN_UP);
	      session.getHistory().push({ pathname: cfg.routes.app });
	    };

	    var onFailure = function onFailure(err) {
	      var msg = err.responseJSON ? err.responseJSON.message : 'Failed to sign up';
	      restApiActions.fail(TRYING_TO_SIGN_UP, msg);
	    };

	    if (secondFactorType == SECOND_FACTOR_TYPE_U2F) {
	      auth.u2fSignUp(name, psw, inviteToken).done(onSuccess).fail(onFailure);
	    } else {
	      auth.signUp(name, psw, token, inviteToken).done(onSuccess).fail(onFailure);
	    }
	  },
	  login: function login(_ref2, redirect) {
	    var user = _ref2.user,
	        password = _ref2.password,
	        token = _ref2.token,
	        provider = _ref2.provider,
	        secondFactorType = _ref2.secondFactorType;

	    if (secondFactorType == SECOND_FACTOR_TYPE_OIDC) {
	      var fullPath = cfg.getFullUrl(redirect);
	      window.location = cfg.api.getSsoUrl(fullPath, provider);
	      return;
	    }

	    restApiActions.start(TRYING_TO_LOGIN);

	    var onSuccess = function onSuccess(sessionData) {
	      restApiActions.success(TRYING_TO_LOGIN);
	      reactor.dispatch(TLPT_RECEIVE_USER, sessionData.user);
	      session.getHistory().push({ pathname: redirect });
	    };

	    var onFailure = function onFailure(err) {
	      var msg = err.responseJSON ? err.responseJSON.message : 'Error';
	      restApiActions.fail(TRYING_TO_LOGIN, msg);
	    };

	    if (secondFactorType == SECOND_FACTOR_TYPE_U2F) {
	      auth.u2fLogin(user, password).done(onSuccess).fail(onFailure);
	    } else {
	      auth.login(user, password, token).done(onSuccess).fail(onFailure);
	    }
	  }
	};

	exports.default = actions;
	module.exports = exports['default'];

/***/ },

/***/ 372:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _keymirror = __webpack_require__(224);

	var _keymirror2 = _interopRequireDefault(_keymirror);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	exports.default = (0, _keymirror2.default)({
	  TLPT_RECEIVE_USER: null,
	  TLPT_RECEIVE_USER_INVITE: null
	}); /*
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

	module.exports = exports['default'];

/***/ },

/***/ 373:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
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

	var reactor = __webpack_require__(215);

	var _require = __webpack_require__(374),
	    TLPT_REST_API_START = _require.TLPT_REST_API_START,
	    TLPT_REST_API_SUCCESS = _require.TLPT_REST_API_SUCCESS,
	    TLPT_REST_API_FAIL = _require.TLPT_REST_API_FAIL;

	exports.default = {
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

/***/ 374:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _keymirror = __webpack_require__(224);

	var _keymirror2 = _interopRequireDefault(_keymirror);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	exports.default = (0, _keymirror2.default)({
	  TLPT_REST_API_START: null,
	  TLPT_REST_API_SUCCESS: null,
	  TLPT_REST_API_FAIL: null
	}); /*
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

	module.exports = exports['default'];

/***/ },

/***/ 375:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

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

	var _require = __webpack_require__(216),
	    Store = _require.Store,
	    toImmutable = _require.toImmutable;

	var _require2 = __webpack_require__(372),
	    TLPT_RECEIVE_USER = _require2.TLPT_RECEIVE_USER;

	exports.default = Store({
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

/***/ 376:
/***/ function(module, exports, __webpack_require__) {

	"use strict";

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

	var React = __webpack_require__(2);

	var GoogleAuthInfo = React.createClass({
	  displayName: "GoogleAuthInfo",
	  render: function render() {
	    return React.createElement(
	      "div",
	      { className: "grv-google-auth text-left" },
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

/***/ },

/***/ 377:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

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

	var React = __webpack_require__(2);
	var $ = __webpack_require__(219);
	var reactor = __webpack_require__(215);

	var _require = __webpack_require__(370),
	    actions = _require.actions,
	    getters = _require.getters;

	var LinkedStateMixin = __webpack_require__(366);
	var GoogleAuthInfo = __webpack_require__(376);

	var _require2 = __webpack_require__(378),
	    ErrorPage = _require2.ErrorPage,
	    ErrorTypes = _require2.ErrorTypes;

	var _require3 = __webpack_require__(341),
	    TeleportLogo = _require3.TeleportLogo;

	var _require4 = __webpack_require__(227),
	    SECOND_FACTOR_TYPE_HOTP = _require4.SECOND_FACTOR_TYPE_HOTP,
	    SECOND_FACTOR_TYPE_U2F = _require4.SECOND_FACTOR_TYPE_U2F;

	var cfg = __webpack_require__(217);

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
	      token: '',
	      secondFactorType: SECOND_FACTOR_TYPE_HOTP
	    };
	  },
	  onClick: function onClick(e) {
	    e.preventDefault();
	    if (this.isValid()) {
	      actions.signUp({
	        name: this.state.name,
	        psw: this.state.psw,
	        token: this.state.token,
	        inviteToken: this.props.invite.invite_token,
	        secondFactorType: this.state.secondFactorType });
	    }
	  },
	  isValid: function isValid() {
	    var $form = $(this.refs.form);
	    return $form.length === 0 || $form.valid();
	  },
	  onSecondFactorTypeChanged: function onSecondFactorTypeChanged(e) {
	    var type = e.currentTarget.value;
	    this.setState({
	      secondFactorType: type
	    });
	    this.props.set2FType(type);
	  },
	  render: function render() {
	    var _props$attemp = this.props.attemp,
	        isProcessing = _props$attemp.isProcessing,
	        isFailed = _props$attemp.isFailed,
	        message = _props$attemp.message;

	    var useU2f = !!cfg.getU2fAppId();
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
	        useU2f ? React.createElement(
	          'div',
	          { className: 'form-group' },
	          React.createElement('input', {
	            type: 'radio',
	            value: SECOND_FACTOR_TYPE_HOTP,
	            checked: this.state.secondFactorType == SECOND_FACTOR_TYPE_HOTP,
	            onChange: this.onSecondFactorTypeChanged }),
	          'Google Authenticator',
	          React.createElement('input', {
	            type: 'radio',
	            value: SECOND_FACTOR_TYPE_U2F,
	            checked: this.state.secondFactorType == SECOND_FACTOR_TYPE_U2F,
	            onChange: this.onSecondFactorTypeChanged }),
	          'U2F'
	        ) : null,
	        this.state.secondFactorType == SECOND_FACTOR_TYPE_HOTP ? React.createElement(
	          'div',
	          { className: 'form-group' },
	          React.createElement('input', {
	            autoComplete: 'off',
	            name: 'token',
	            valueLink: this.linkState('token'),
	            className: 'form-control required',
	            placeholder: 'Two factor token (Google Authenticator)' })
	        ) : null,
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
	  getInitialState: function getInitialState() {
	    return {
	      secondFactorType: SECOND_FACTOR_TYPE_HOTP
	    };
	  },
	  setSecondFactorType: function setSecondFactorType(type) {
	    this.setState({
	      secondFactorType: type
	    });
	  },


	  render: function render() {
	    var _state = this.state,
	        fetchingInvite = _state.fetchingInvite,
	        invite = _state.invite,
	        attemp = _state.attemp;


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
	          React.createElement(InviteInputForm, { attemp: attemp, invite: invite.toJS(), set2FType: this.setSecondFactorType }),
	          React.createElement(GoogleAuthInfo, null)
	        ),
	        this.state.secondFactorType == SECOND_FACTOR_TYPE_HOTP ? React.createElement(
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
	        ) : React.createElement(
	          'div',
	          { className: 'grv-flex-column' },
	          React.createElement(
	            'h4',
	            null,
	            'Insert your U2F key ',
	            React.createElement('br', null),
	            ' ',
	            React.createElement(
	              'small',
	              null,
	              'Press the button on the U2F key after you press the sign up button'
	            )
	          )
	        )
	      )
	    );
	  }
	});

	module.exports = Invite;

/***/ },

/***/ 378:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
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
	    var _props$params = this.props.params,
	        type = _props$params.type,
	        subType = _props$params.subType;

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

/***/ },

/***/ 379:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

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

	var React = __webpack_require__(2);
	var reactor = __webpack_require__(215);
	var userGetters = __webpack_require__(222);
	var nodeGetters = __webpack_require__(380);
	var NodeList = __webpack_require__(381);
	var ClusterContent = __webpack_require__(391);

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
	    var _state = this.state,
	        nodeRecords = _state.nodeRecords,
	        user = _state.user,
	        sites = _state.sites,
	        siteId = _state.siteId;
	    var logins = user.logins;

	    return React.createElement(
	      ClusterContent,
	      null,
	      React.createElement(NodeList, {
	        siteId: siteId,
	        sites: sites,
	        nodeRecords: nodeRecords,
	        logins: logins
	      })
	    );
	  }
	});

	module.exports = Nodes;

/***/ },

/***/ 380:
/***/ function(module, exports) {

	'use strict';

	exports.__esModule = true;
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

	var nodeHostNameByServerId = function nodeHostNameByServerId(serverId) {
	  return [['tlpt_nodes'], function (nodeList) {
	    var server = nodeList.find(function (item) {
	      return item.get('id') === serverId;
	    });
	    return !server ? '' : server.get('hostname');
	  }];
	};

	var nodeListView = [['tlpt_nodes'], ['tlpt', 'siteId'], function (nodeList, siteId) {
	  nodeList = nodeList.filter(function (n) {
	    return n.get('siteId') === siteId;
	  });
	  if (!nodeList) {
	    return [];
	  }

	  return nodeList.map(function (item) {
	    var serverId = item.get('id');
	    return {
	      id: serverId,
	      siteId: item.get('siteId'),
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

	exports.default = {
	  nodeListView: nodeListView,
	  nodeHostNameByServerId: nodeHostNameByServerId
	};
	module.exports = exports['default'];

/***/ },

/***/ 381:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }

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

	var React = __webpack_require__(2);
	var InputSearch = __webpack_require__(382);

	var _require = __webpack_require__(385),
	    Table = _require.Table,
	    Column = _require.Column,
	    Cell = _require.Cell,
	    SortHeaderCell = _require.SortHeaderCell,
	    SortTypes = _require.SortTypes,
	    EmptyIndicator = _require.EmptyIndicator;

	var _require2 = __webpack_require__(386),
	    createNewSession = _require2.createNewSession;

	var _ = __webpack_require__(383);

	var _require3 = __webpack_require__(390),
	    isMatch = _require3.isMatch;

	var TextCell = function TextCell(_ref) {
	  var rowIndex = _ref.rowIndex,
	      data = _ref.data,
	      columnKey = _ref.columnKey,
	      props = _objectWithoutProperties(_ref, ['rowIndex', 'data', 'columnKey']);

	  return React.createElement(
	    Cell,
	    props,
	    data[rowIndex][columnKey]
	  );
	};

	var TagCell = function TagCell(_ref2) {
	  var rowIndex = _ref2.rowIndex,
	      data = _ref2.data,
	      props = _objectWithoutProperties(_ref2, ['rowIndex', 'data']);

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
	  var logins = _ref3.logins,
	      onLoginClick = _ref3.onLoginClick,
	      rowIndex = _ref3.rowIndex,
	      data = _ref3.data,
	      props = _objectWithoutProperties(_ref3, ['logins', 'onLoginClick', 'rowIndex', 'data']);

	  if (!logins || logins.length === 0) {
	    return React.createElement(Cell, props);
	  }

	  var _data$rowIndex = data[rowIndex],
	      id = _data$rowIndex.id,
	      siteId = _data$rowIndex.siteId;

	  var $lis = [];

	  function onClick(i) {
	    var login = logins[i];
	    if (onLoginClick) {
	      return function () {
	        return onLoginClick(id, login);
	      };
	    } else {
	      return function () {
	        return createNewSession(siteId, id, login);
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
	  getInitialState: function getInitialState() {
	    this.searchableProps = ['addr', 'hostname', 'tags'];
	    return {
	      filter: '',
	      colSortDirs: { hostname: SortTypes.DESC }
	    };
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
	        var role = item.role,
	            value = item.value;

	        return role.toLocaleUpperCase().indexOf(searchValue) !== -1 || value.toLocaleUpperCase().indexOf(searchValue) !== -1;
	      });
	    }
	  },
	  sortAndFilter: function sortAndFilter(data) {
	    var _this = this;

	    var colSortDirs = this.state.colSortDirs;

	    var filtered = data.filter(function (obj) {
	      return isMatch(obj, _this.state.filter, {
	        searchableProps: _this.searchableProps,
	        cb: _this.searchAndFilterCb
	      });
	    });

	    var columnKey = Object.getOwnPropertyNames(colSortDirs)[0];
	    var sortDir = colSortDirs[columnKey];
	    var sorted = _.sortBy(filtered, columnKey);
	    if (sortDir === SortTypes.ASC) {
	      sorted = sorted.reverse();
	    }

	    return sorted;
	  },
	  render: function render() {
	    var _props = this.props,
	        nodeRecords = _props.nodeRecords,
	        logins = _props.logins,
	        onLoginClick = _props.onLoginClick;

	    var data = this.sortAndFilter(nodeRecords);
	    return React.createElement(
	      'div',
	      { className: 'grv-nodes' },
	      React.createElement(
	        'div',
	        { className: 'grv-flex grv-header m-t', style: { justifyContent: "space-between" } },
	        React.createElement(
	          'h2',
	          { className: 'text-center no-margins' },
	          ' Nodes '
	        ),
	        React.createElement(
	          'div',
	          { className: 'grv-flex' },
	          React.createElement(InputSearch, { value: this.filter, onChange: this.onFilterChange })
	        )
	      ),
	      React.createElement(
	        'div',
	        { className: 'm-t' },
	        data.length === 0 && this.state.filter.length > 0 ? React.createElement(EmptyIndicator, { text: 'No matching nodes found' }) : React.createElement(
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

	exports.default = NodeList;
	module.exports = exports['default'];

/***/ },

/***/ 382:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

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

	var React = __webpack_require__(2);

	var _require = __webpack_require__(383),
	    debounce = _require.debounce;

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
	    var _props$className = this.props.className,
	        className = _props$className === undefined ? '' : _props$className;

	    className = 'grv-search ' + className;

	    return React.createElement(
	      'div',
	      { className: className },
	      React.createElement('input', { placeholder: 'Search...', className: 'form-control input-sm',
	        value: this.state.value,
	        onChange: this.onChange })
	    );
	  }
	});

	module.exports = InputSearch;

/***/ },

/***/ 385:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }

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

	var React = __webpack_require__(2);

	var GrvTableTextCell = function GrvTableTextCell(_ref) {
	  var rowIndex = _ref.rowIndex,
	      data = _ref.data,
	      columnKey = _ref.columnKey,
	      props = _objectWithoutProperties(_ref, ['rowIndex', 'data', 'columnKey']);

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
	    var _props = this.props,
	        sortDir = _props.sortDir,
	        title = _props.title,
	        props = _objectWithoutProperties(_props, ['sortDir', 'title']);

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
	    var _props2 = this.props,
	        isHeader = _props2.isHeader,
	        children = _props2.children,
	        _props2$className = _props2.className,
	        className = _props2$className === undefined ? '' : _props2$className;

	    className = 'grv-table-cell ' + className;
	    return isHeader ? React.createElement(
	      'th',
	      { className: className },
	      children
	    ) : React.createElement(
	      'td',
	      null,
	      children
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
	    { className: 'grv-table-indicator-empty text-muted' },
	    React.createElement(
	      'span',
	      null,
	      text
	    )
	  );
	};

	exports.default = GrvTable;
	exports.Column = GrvTableColumn;
	exports.Table = GrvTable;
	exports.Cell = GrvTableCell;
	exports.TextCell = GrvTableTextCell;
	exports.SortHeaderCell = SortHeaderCell;
	exports.SortIndicator = SortIndicator;
	exports.SortTypes = SortTypes;
	exports.EmptyIndicator = EmptyIndicator;

/***/ },

/***/ 386:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
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
	var reactor = __webpack_require__(215);
	var session = __webpack_require__(229);
	var api = __webpack_require__(228);
	var cfg = __webpack_require__(217);
	var getters = __webpack_require__(387);

	var _require = __webpack_require__(239),
	    fetchStoredSession = _require.fetchStoredSession,
	    updateSession = _require.updateSession;

	var sessionGetters = __webpack_require__(388);
	var $ = __webpack_require__(219);

	var logger = __webpack_require__(230).create('Current Session');

	var _require2 = __webpack_require__(389),
	    TLPT_CURRENT_SESSION_OPEN = _require2.TLPT_CURRENT_SESSION_OPEN,
	    TLPT_CURRENT_SESSION_CLOSE = _require2.TLPT_CURRENT_SESSION_CLOSE;

	var actions = {
	  createNewSession: function createNewSession(siteId, serverId, login) {
	    var data = { 'session': { 'terminal_params': { 'w': 45, 'h': 5 }, login: login } };
	    api.post(cfg.api.getSiteSessionUrl(siteId), data).then(function (json) {
	      var sid = json.session.id;
	      var routeUrl = cfg.getCurrentSessionRouteUrl({ siteId: siteId, sid: sid });
	      var history = session.getHistory();

	      reactor.dispatch(TLPT_CURRENT_SESSION_OPEN, {
	        siteId: siteId,
	        serverId: serverId,
	        login: login,
	        sid: sid,
	        isNewSession: true
	      });

	      history.push(routeUrl);
	    });
	  },
	  openSession: function openSession(nextState) {
	    var sid = nextState.params.sid;

	    var currentSession = reactor.evaluate(getters.currentSession);
	    if (currentSession) {
	      return;
	    }

	    logger.info('attempt to open session', { sid: sid });
	    $.when(fetchStoredSession(sid)).done(function () {
	      var sView = reactor.evaluate(sessionGetters.sessionViewById(sid));
	      if (!sView) {
	        return;
	      }

	      var serverId = sView.serverId,
	          login = sView.login,
	          siteId = sView.siteId;

	      logger.info('open session', 'OK');
	      reactor.dispatch(TLPT_CURRENT_SESSION_OPEN, {
	        siteId: siteId,
	        serverId: serverId,
	        login: login,
	        sid: sid,
	        isNewSession: false
	      });
	    }).fail(function (err) {
	      logger.error('open session', err);
	    });
	  },
	  close: function close() {
	    var _reactor$evaluate = reactor.evaluate(getters.currentSession),
	        isNewSession = _reactor$evaluate.isNewSession;

	    reactor.dispatch(TLPT_CURRENT_SESSION_CLOSE);

	    if (isNewSession) {
	      session.getHistory().push(cfg.routes.nodes);
	    } else {
	      session.getHistory().push(cfg.routes.sessions);
	    }
	  },
	  processSessionFromEventStream: function processSessionFromEventStream(siteId) {
	    return function (data) {
	      data.events.forEach(function (item) {
	        if (item.event === 'session.end') {
	          actions.close();
	        }
	      });

	      var session = data.session;

	      session.siteId = siteId;
	      updateSession(data.session);
	    };
	  }
	};

	exports.default = actions;
	module.exports = exports['default'];

/***/ },

/***/ 387:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

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

	var _require = __webpack_require__(388),
	    createView = _require.createView;

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
	    serverId: current.get('serverId'),
	    login: current.get('login'),
	    sid: current.get('sid'),
	    siteId: current.get('siteId'),
	    cols: undefined,
	    rows: undefined
	  };

	  /*
	  * in case if session already exists, get its view data (for example, when joining an existing session)
	  */
	  if (sessions.has(curSessionView.sid)) {
	    var existing = createView(sessions.get(curSessionView.sid));

	    curSessionView.parties = existing.parties;
	    curSessionView.serverId = existing.serverId;
	    curSessionView.active = existing.active;
	    curSessionView.cols = existing.cols;
	    curSessionView.rows = existing.rows;
	    curSessionView.siteId = existing.siteId;
	  }

	  return curSessionView;
	}];

	exports.default = {
	  currentSession: currentSession
	};
	module.exports = exports['default'];

/***/ },

/***/ 388:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
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

	var moment = __webpack_require__(240);
	var reactor = __webpack_require__(215);
	var cfg = __webpack_require__(217);

	var sessionsView = [['tlpt_sessions'], ['tlpt', 'siteId'], function (sessionList, siteId) {
	  sessionList = sessionList.filter(function (n) {
	    return n.get('siteId') === siteId;
	  });
	  return sessionList.valueSeq().map(createView).toJS();
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

	    return parties.map(function (item) {
	      return {
	        user: item.get('user'),
	        serverIp: item.get('remote_addr'),
	        serverId: item.get('server_id')
	      };
	    }).toJS();
	  }];
	};

	function createView(session) {
	  var sid = session.get('id');
	  var serverIp;
	  var parties = reactor.evaluate(partiesBySessionId(sid));

	  if (parties.length > 0) {
	    serverIp = parties[0].serverIp;
	  }

	  var created = new Date(session.get('created'));
	  var lastActive = new Date(session.get('last_active'));
	  var duration = moment(created).diff(lastActive);
	  var siteId = session.get('siteId');

	  return {
	    parties: parties,
	    sid: sid,
	    created: created,
	    lastActive: lastActive,
	    duration: duration,
	    serverIp: serverIp,
	    siteId: siteId,
	    stored: session.get('stored'),
	    serverId: session.get('server_id'),
	    clientIp: session.get('clientIp'),
	    nodeIp: session.get('nodeIp'),
	    active: session.get('active'),
	    user: session.get('user'),
	    login: session.get('login'),
	    sessionUrl: cfg.getCurrentSessionRouteUrl({ sid: sid, siteId: siteId }),
	    cols: session.getIn(['terminal_params', 'w']),
	    rows: session.getIn(['terminal_params', 'h'])
	  };
	}

	exports.default = {
	  partiesBySessionId: partiesBySessionId,
	  sessionsView: sessionsView,
	  sessionViewById: sessionViewById,
	  createView: createView,
	  count: [['tlpt_sessions'], function (sessions) {
	    return sessions.size;
	  }]
	};
	module.exports = exports['default'];

/***/ },

/***/ 389:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _keymirror = __webpack_require__(224);

	var _keymirror2 = _interopRequireDefault(_keymirror);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	exports.default = (0, _keymirror2.default)({
	  TLPT_CURRENT_SESSION_OPEN: null,
	  TLPT_CURRENT_SESSION_CLOSE: null
	}); /*
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

	module.exports = exports['default'];

/***/ },

/***/ 390:
/***/ function(module, exports) {

	'use strict';

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

	module.exports.isMatch = function (obj, searchValue, _ref) {
	  var searchableProps = _ref.searchableProps,
	      cb = _ref.cb;

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

/***/ 391:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _reactor = __webpack_require__(215);

	var _reactor2 = _interopRequireDefault(_reactor);

	var _getters = __webpack_require__(392);

	var _getters2 = _interopRequireDefault(_getters);

	var _getters3 = __webpack_require__(238);

	var _getters4 = _interopRequireDefault(_getters3);

	var _dropdown = __webpack_require__(393);

	var _dropdown2 = _interopRequireDefault(_dropdown);

	var _actions = __webpack_require__(226);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

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

	var YOUR_CSR_TEXT = 'Your cluster';

	var PageWithHeader = _react2.default.createClass({
	  displayName: 'PageWithHeader',


	  mixins: [_reactor2.default.ReactMixin],

	  getDataBindings: function getDataBindings() {
	    return {
	      sites: _getters2.default.sites,
	      siteId: _getters4.default.siteId
	    };
	  },
	  onChangeSite: function onChangeSite(value) {
	    (0, _actions.setSiteId)(value);
	    (0, _actions.refresh)();
	  },
	  render: function render() {
	    var _state = this.state,
	        sites = _state.sites,
	        siteId = _state.siteId;

	    var siteOptions = sites.map(function (s, index) {
	      return {
	        label: index === 0 ? YOUR_CSR_TEXT : s.name,
	        value: s.name
	      };
	    });

	    return _react2.default.createElement(
	      'div',
	      { className: 'grv-page' },
	      _react2.default.createElement(_dropdown2.default, {
	        className: 'grv-page-header-clusters-selector m-t-sm',
	        size: 'sm',
	        align: 'right',
	        onChange: this.onChangeSite,
	        value: siteId,
	        options: siteOptions
	      }),
	      this.props.children
	    );
	  }
	});

	exports.default = PageWithHeader;
	module.exports = exports['default'];

/***/ },

/***/ 392:
/***/ function(module, exports) {

	'use strict';

	exports.__esModule = true;
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

	var SiteStatusEnum = {
	  ONLINE: 'online',
	  OFFLINE: 'offline'
	};

	var onlyOnline = function onlyOnline(s) {
	  return s.status === SiteStatusEnum.ONLINE;
	};

	var sites = [['tlpt_sites'], function (siteList) {
	  return siteList.filter(onlyOnline).toArray();
	}];

	exports.default = {
	  sites: sites
	};
	module.exports = exports['default'];

/***/ },

/***/ 393:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

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

	var React = __webpack_require__(2);
	var $ = __webpack_require__(219);

	var _require = __webpack_require__(383),
	    isObject = _require.isObject;

	var classnames = __webpack_require__(346);

	var DropDown = React.createClass({
	  displayName: 'DropDown',
	  onClick: function onClick(event) {
	    event.preventDefault();
	    var options = this.props.options;

	    var index = $(event.target).parent().index();
	    var option = options[index];
	    var value = isObject(option) ? option.value : option;

	    this.props.onChange(value);
	  },
	  renderOption: function renderOption(option, index) {
	    var displayValue = isObject(option) ? option.label : option;
	    return React.createElement(
	      'li',
	      { key: index },
	      React.createElement(
	        'a',
	        { href: '#' },
	        displayValue
	      )
	    );
	  },
	  getDisplayValue: function getDisplayValue(value) {
	    var _props$options = this.props.options,
	        options = _props$options === undefined ? [] : _props$options;

	    for (var i = 0; i < options.length; i++) {
	      var op = options[i];
	      if (isObject(op) && op.value === value) {
	        return op.label;
	      }

	      if (op === value) {
	        return value;
	      }
	    }

	    return null;
	  },
	  render: function render() {
	    var _props = this.props,
	        options = _props.options,
	        value = _props.value,
	        classRules = _props.classRules,
	        _props$className = _props.className,
	        className = _props$className === undefined ? '' : _props$className,
	        name = _props.name,
	        _props$size = _props.size,
	        size = _props$size === undefined ? 'default' : _props$size,
	        _props$align = _props.align,
	        align = _props$align === undefined ? 'left' : _props$align;

	    var $options = options.map(this.renderOption);
	    var hiddenValue = value;
	    var displayValue = this.getDisplayValue(value);

	    displayValue = displayValue || 'Select...';

	    var valueClass = classnames('grv-dropdown-value', {
	      'text-muted': !hiddenValue
	    });

	    var mainClass = 'grv-dropdown ' + className;

	    var btnClass = classnames('btn btn-default full-width dropdown-toggle', {
	      'btn-sm': size === 'sm'
	    });

	    var menuClass = classnames('dropdown-menu', {
	      'pull-right': align === 'right'
	    });

	    var $menu = options.length > 0 ? React.createElement(
	      'ul',
	      { onClick: this.onClick, className: menuClass },
	      $options
	    ) : null;

	    return React.createElement(
	      'div',
	      { className: mainClass },
	      React.createElement(
	        'div',
	        { className: 'dropdown' },
	        React.createElement(
	          'div',
	          { className: btnClass, type: 'button', 'data-toggle': 'dropdown', 'aria-haspopup': 'true', 'aria-expanded': 'true' },
	          React.createElement(
	            'div',
	            { className: valueClass },
	            React.createElement(
	              'span',
	              { style: { textOverflow: "ellipsis", overflow: "hidden" } },
	              displayValue
	            ),
	            React.createElement('span', { className: 'caret m-l-sm' })
	          )
	        ),
	        $menu
	      ),
	      React.createElement('input', { className: classRules, value: hiddenValue, type: 'hidden', ref: 'input', name: name })
	    );
	  }
	});

	module.exports = DropDown;

/***/ },

/***/ 394:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

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

	var React = __webpack_require__(2);
	var reactor = __webpack_require__(215);

	var _require = __webpack_require__(395),
	    fetchStoredSession = _require.fetchStoredSession;

	var _require2 = __webpack_require__(388),
	    sessionsView = _require2.sessionsView;

	var _require3 = __webpack_require__(396),
	    filter = _require3.filter;

	var StoredSessionList = __webpack_require__(398);
	var ActiveSessionList = __webpack_require__(402);
	var Timer = __webpack_require__(364);
	var ClusterContent = __webpack_require__(391);

	var Sessions = React.createClass({
	  displayName: 'Sessions',

	  mixins: [reactor.ReactMixin],

	  getDataBindings: function getDataBindings() {
	    return {
	      data: sessionsView,
	      storedSessionsFilter: filter
	    };
	  },
	  refresh: function refresh() {
	    fetchStoredSession();
	  },


	  render: function render() {
	    var _state = this.state,
	        data = _state.data,
	        storedSessionsFilter = _state.storedSessionsFilter;

	    return React.createElement(
	      ClusterContent,
	      null,
	      React.createElement(
	        'div',
	        { className: 'grv-sessions' },
	        React.createElement(Timer, { onTimeout: this.refresh }),
	        React.createElement(ActiveSessionList, { data: data }),
	        React.createElement('div', { className: 'm-t-lg' }),
	        React.createElement(StoredSessionList, { data: data, filter: storedSessionsFilter })
	      )
	    );
	  }
	});

	module.exports = Sessions;

/***/ },

/***/ 395:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
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

	var reactor = __webpack_require__(215);

	var _require = __webpack_require__(396),
	    filter = _require.filter;

	var _require2 = __webpack_require__(239),
	    fetchSiteEvents = _require2.fetchSiteEvents;

	var _require3 = __webpack_require__(232),
	    showError = _require3.showError;

	var logger = __webpack_require__(230).create('Modules/Sessions');

	var _require4 = __webpack_require__(397),
	    TLPT_STORED_SESSINS_FILTER_SET_RANGE = _require4.TLPT_STORED_SESSINS_FILTER_SET_RANGE;

	var actions = {
	  fetchStoredSession: function fetchStoredSession() {
	    var _reactor$evaluate = reactor.evaluate(filter),
	        start = _reactor$evaluate.start,
	        end = _reactor$evaluate.end;

	    _fetch(start, end);
	  },
	  setTimeRange: function setTimeRange(start, end) {
	    reactor.batch(function () {
	      reactor.dispatch(TLPT_STORED_SESSINS_FILTER_SET_RANGE, { start: start, end: end });
	      _fetch(start, end);
	    });
	  }
	};

	function _fetch(start, end) {
	  return fetchSiteEvents(start, end).fail(function (err) {
	    showError('Unable to retrieve list of sessions for a given time range');
	    logger.error('fetching filtered set of sessions', err);
	  });
	}

	exports.default = actions;
	module.exports = exports['default'];

/***/ },

/***/ 396:
/***/ function(module, exports) {

	'use strict';

	exports.__esModule = true;
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

	var filter = [['tlpt_stored_sessions_filter'], function (filter) {
	  return filter.toJS();
	}];

	exports.default = {
	  filter: filter
	};
	module.exports = exports['default'];

/***/ },

/***/ 397:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _keymirror = __webpack_require__(224);

	var _keymirror2 = _interopRequireDefault(_keymirror);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	exports.default = (0, _keymirror2.default)({
	  TLPT_STORED_SESSINS_FILTER_SET_RANGE: null,
	  TLPT_STORED_SESSINS_FILTER_SET_STATUS: null,
	  TLPT_STORED_SESSINS_FILTER_RECEIVE_MORE: null
	}); /*
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

	module.exports = exports['default'];

/***/ },

/***/ 398:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

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

	var _ = __webpack_require__(383);
	var React = __webpack_require__(2);
	var moment = __webpack_require__(240);
	var InputSearch = __webpack_require__(382);

	var _require = __webpack_require__(390),
	    isMatch = _require.isMatch;

	var _require2 = __webpack_require__(217),
	    displayDateFormat = _require2.displayDateFormat;

	var _require3 = __webpack_require__(399),
	    actions = _require3.actions;

	var _require4 = __webpack_require__(385),
	    Table = _require4.Table,
	    Column = _require4.Column,
	    Cell = _require4.Cell,
	    TextCell = _require4.TextCell,
	    SortHeaderCell = _require4.SortHeaderCell,
	    SortTypes = _require4.SortTypes,
	    EmptyIndicator = _require4.EmptyIndicator;

	var _require5 = __webpack_require__(400),
	    ButtonCell = _require5.ButtonCell,
	    SingleUserCell = _require5.SingleUserCell,
	    DateCreatedCell = _require5.DateCreatedCell,
	    DurationCell = _require5.DurationCell;

	var _require6 = __webpack_require__(401),
	    DateRangePicker = _require6.DateRangePicker;

	var StoredSessions = React.createClass({
	  displayName: 'StoredSessions',
	  getInitialState: function getInitialState() {
	    this.searchableProps = ['nodeIp', 'created', 'sid', 'login'];
	    return { filter: '', colSortDirs: { created: 'ASC' } };
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
	    var startDate = _ref.startDate,
	        endDate = _ref.endDate;

	    /**
	    * as date picker uses timeouts its important to ensure that
	    * component is still mounted when data picker triggers an update
	    */
	    if (this.isMounted()) {
	      actions.setTimeRange(startDate, endDate);
	    }
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
	    var _props$filter = this.props.filter,
	        start = _props$filter.start,
	        end = _props$filter.end;

	    var data = this.props.data.filter(function (item) {
	      return item.stored && moment(item.created).isBetween(start, end);
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
	          { className: 'grv-flex m-b-md', style: { justifyContent: "space-between" } },
	          React.createElement(
	            'h2',
	            { className: 'text-center' },
	            ' Archived Sessions '
	          ),
	          React.createElement(
	            'div',
	            { className: 'grv-flex' },
	            React.createElement(DateRangePicker, { className: 'm-r', startDate: start, endDate: end, onChange: this.onRangePickerChange }),
	            React.createElement(InputSearch, { value: this.filter, onChange: this.onFilterChange })
	          )
	        )
	      ),
	      React.createElement(
	        'div',
	        { className: 'grv-content' },
	        data.length === 0 ? React.createElement(EmptyIndicator, { text: 'No matching archived sessions found' }) : React.createElement(
	          'div',
	          { className: '' },
	          React.createElement(
	            Table,
	            { rowCount: data.length, className: 'table-striped' },
	            React.createElement(Column, {
	              header: React.createElement(Cell, null),
	              cell: React.createElement(ButtonCell, { data: data })
	            }),
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
	              columnKey: 'nodeIp',
	              header: React.createElement(SortHeaderCell, {
	                sortDir: this.state.colSortDirs.nodeIp,
	                onSortChange: this.onSortChange,
	                title: 'Node IP',
	                className: 'grv-sessions-stored-col-ip'
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
	              columnKey: 'duration',
	              header: React.createElement(SortHeaderCell, {
	                sortDir: this.state.colSortDirs.duration,
	                onSortChange: this.onSortChange,
	                title: 'Duration'
	              }),
	              cell: React.createElement(DurationCell, { data: data })
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

	module.exports = StoredSessions;

/***/ },

/***/ 399:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

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
	module.exports.getters = __webpack_require__(396);
	module.exports.actions = __webpack_require__(395);

/***/ },

/***/ 400:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }

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

	var React = __webpack_require__(2);
	var reactor = __webpack_require__(215);

	var _require = __webpack_require__(155),
	    Link = _require.Link;

	var _require2 = __webpack_require__(380),
	    nodeHostNameByServerId = _require2.nodeHostNameByServerId;

	var _require3 = __webpack_require__(217),
	    displayDateFormat = _require3.displayDateFormat;

	var _require4 = __webpack_require__(385),
	    Cell = _require4.Cell;

	var moment = __webpack_require__(240);

	var DateCreatedCell = function DateCreatedCell(_ref) {
	  var rowIndex = _ref.rowIndex,
	      data = _ref.data,
	      props = _objectWithoutProperties(_ref, ['rowIndex', 'data']);

	  var created = data[rowIndex].created;
	  var displayDate = moment(created).format(displayDateFormat);
	  return React.createElement(
	    Cell,
	    props,
	    displayDate
	  );
	};

	var DurationCell = function DurationCell(_ref2) {
	  var rowIndex = _ref2.rowIndex,
	      data = _ref2.data,
	      props = _objectWithoutProperties(_ref2, ['rowIndex', 'data']);

	  var duration = data[rowIndex].duration;

	  var displayDate = moment.duration(duration).humanize();
	  return React.createElement(
	    Cell,
	    props,
	    displayDate
	  );
	};

	var SingleUserCell = function SingleUserCell(_ref3) {
	  var rowIndex = _ref3.rowIndex,
	      data = _ref3.data,
	      props = _objectWithoutProperties(_ref3, ['rowIndex', 'data']);

	  var user = data[rowIndex].user;

	  return React.createElement(
	    Cell,
	    props,
	    React.createElement(
	      'span',
	      { className: 'grv-sessions-user label label-default' },
	      user
	    )
	  );
	};

	var UsersCell = function UsersCell(_ref4) {
	  var rowIndex = _ref4.rowIndex,
	      data = _ref4.data,
	      props = _objectWithoutProperties(_ref4, ['rowIndex', 'data']);

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
	  var rowIndex = _ref5.rowIndex,
	      data = _ref5.data,
	      props = _objectWithoutProperties(_ref5, ['rowIndex', 'data']);

	  var _data$rowIndex = data[rowIndex],
	      sessionUrl = _data$rowIndex.sessionUrl,
	      active = _data$rowIndex.active;

	  var _ref6 = active ? ['join', 'btn-warning'] : ['play', 'btn-primary'],
	      actionText = _ref6[0],
	      actionClass = _ref6[1];

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
	  var rowIndex = _ref7.rowIndex,
	      data = _ref7.data,
	      props = _objectWithoutProperties(_ref7, ['rowIndex', 'data']);

	  var serverId = data[rowIndex].serverId;

	  var hostname = reactor.evaluate(nodeHostNameByServerId(serverId)) || 'unknown';

	  return React.createElement(
	    Cell,
	    props,
	    hostname
	  );
	};

	exports.default = ButtonCell;
	exports.ButtonCell = ButtonCell;
	exports.UsersCell = UsersCell;
	exports.DurationCell = DurationCell;
	exports.DateCreatedCell = DateCreatedCell;
	exports.SingleUserCell = SingleUserCell;
	exports.NodeCell = NodeCell;

/***/ },

/***/ 401:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
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

	var React = __webpack_require__(2);
	var $ = __webpack_require__(219);
	var moment = __webpack_require__(240);

	var _require = __webpack_require__(383),
	    debounce = _require.debounce;

	var DateRangePicker = React.createClass({
	  displayName: 'DateRangePicker',
	  getDates: function getDates() {
	    var startDate = $(this.refs.dpPicker1).datepicker('getDate');
	    var endDate = $(this.refs.dpPicker2).datepicker('getDate');
	    return [startDate, moment(endDate).endOf('day').toDate()];
	  },
	  setDates: function setDates(_ref) {
	    var startDate = _ref.startDate,
	        endDate = _ref.endDate;

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
	    var _getDates = this.getDates(),
	        startDate = _getDates[0],
	        endDate = _getDates[1];

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
	    var _getDates2 = this.getDates(),
	        startDate = _getDates2[0],
	        endDate = _getDates2[1];

	    if (!(isSame(startDate, this.props.startDate) && isSame(endDate, this.props.endDate))) {
	      this.props.onChange({ startDate: startDate, endDate: endDate });
	    }
	  },
	  render: function render() {
	    return React.createElement(
	      'div',
	      { className: 'grv-datepicker input-group input-daterange m-r', ref: 'rangePicker' },
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

	exports.default = DateRangePicker;
	exports.CalendarNav = CalendarNav;
	exports.DateRangePicker = DateRangePicker;

/***/ },

/***/ 402:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	/*
	Copyright 2015 Gravitational, Inc.

	Licensed under the Apache License, Version 2.0 (the "License");
	You may obtain a copy of the License at
	you may not use this file except in compliance with the License.

	    http://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/

	var React = __webpack_require__(2);

	var _require = __webpack_require__(385),
	    Table = _require.Table,
	    Column = _require.Column,
	    Cell = _require.Cell,
	    TextCell = _require.TextCell,
	    EmptyIndicator = _require.EmptyIndicator;

	var _require2 = __webpack_require__(400),
	    ButtonCell = _require2.ButtonCell,
	    UsersCell = _require2.UsersCell,
	    NodeCell = _require2.NodeCell,
	    DateCreatedCell = _require2.DateCreatedCell;

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
	          'h2',
	          { className: '' },
	          ' Active Sessions '
	        )
	      ),
	      React.createElement(
	        'div',
	        { className: 'grv-content m-t' },
	        data.length === 0 ? React.createElement(EmptyIndicator, { text: 'You have no active sessions' }) : React.createElement(
	          'div',
	          { className: '' },
	          React.createElement(
	            Table,
	            { rowCount: data.length, className: 'table-striped' },
	            React.createElement(Column, {
	              header: React.createElement(Cell, null),
	              cell: React.createElement(ButtonCell, { data: data })
	            }),
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
	                'Node'
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
	                ' User '
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

/***/ },

/***/ 403:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

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

	var React = __webpack_require__(2);
	var reactor = __webpack_require__(215);

	var _require = __webpack_require__(404),
	    getters = _require.getters;

	var SessionPlayer = __webpack_require__(406);
	var ActiveSession = __webpack_require__(449);
	var cfg = __webpack_require__(217);

	var CurrentSessionHost = React.createClass({
	  displayName: 'CurrentSessionHost',


	  mixins: [reactor.ReactMixin],

	  getDataBindings: function getDataBindings() {
	    return {
	      currentSession: getters.currentSession
	    };
	  },
	  render: function render() {
	    var currentSession = this.state.currentSession;

	    if (!currentSession) {
	      return null;
	    }

	    if (currentSession.isNewSession || currentSession.active) {
	      return React.createElement(ActiveSession, currentSession);
	    }

	    var sid = currentSession.sid,
	        siteId = currentSession.siteId;

	    var url = cfg.api.getFetchSessionUrl({ siteId: siteId, sid: sid });

	    return React.createElement(SessionPlayer, { url: url });
	  }
	});

	module.exports = CurrentSessionHost;

/***/ },

/***/ 404:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

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
	module.exports.getters = __webpack_require__(387);
	module.exports.actions = __webpack_require__(386);
	module.exports.activeTermStore = __webpack_require__(405);

/***/ },

/***/ 405:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

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

	var _require = __webpack_require__(216),
	    Store = _require.Store,
	    toImmutable = _require.toImmutable;

	var _require2 = __webpack_require__(389),
	    TLPT_CURRENT_SESSION_OPEN = _require2.TLPT_CURRENT_SESSION_OPEN,
	    TLPT_CURRENT_SESSION_CLOSE = _require2.TLPT_CURRENT_SESSION_CLOSE;

	exports.default = Store({
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
	  var siteId = _ref.siteId,
	      serverId = _ref.serverId,
	      login = _ref.login,
	      sid = _ref.sid,
	      isNewSession = _ref.isNewSession;

	  return toImmutable({
	    siteId: siteId,
	    serverId: serverId,
	    login: login,
	    sid: sid,
	    isNewSession: isNewSession
	  });
	}
	module.exports = exports['default'];

/***/ },

/***/ 406:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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

	var React = __webpack_require__(2);
	var ReactSlider = __webpack_require__(407);

	var _require = __webpack_require__(408),
	    TtyPlayer = _require.TtyPlayer;

	var Terminal = __webpack_require__(416);
	var SessionLeftPanel = __webpack_require__(420);
	var $ = __webpack_require__(219);
	__webpack_require__(427)($);

	var Term = function (_Terminal) {
	  _inherits(Term, _Terminal);

	  function Term(tty, el) {
	    _classCallCheck(this, Term);

	    var _this = _possibleConstructorReturn(this, _Terminal.call(this, { el: el, scrollBack: 0 }));

	    _this.tty = tty;
	    return _this;
	  }

	  Term.prototype.connect = function connect() {
	    this.tty.connect();
	  };

	  Term.prototype.open = function open() {
	    _Terminal.prototype.open.call(this);
	    $(this._el).perfectScrollbar();
	  };

	  Term.prototype.resize = function resize(cols, rows) {
	    if (cols === this.cols && rows === this.rows) {
	      return;
	    }

	    _Terminal.prototype.resize.call(this, cols, rows);
	    $(this._el).perfectScrollbar('update');
	  };

	  Term.prototype._disconnect = function _disconnect() {};

	  Term.prototype._requestResize = function _requestResize() {};

	  return Term;
	}(Terminal);

	var SessionPlayer = React.createClass({
	  displayName: 'SessionPlayer',
	  calculateState: function calculateState() {
	    return {
	      length: this.tty.length,
	      min: 1,
	      time: this.tty.getCurrentTime(),
	      isPlaying: this.tty.isPlaying,
	      current: this.tty.current,
	      canPlay: this.tty.length > 1
	    };
	  },
	  getInitialState: function getInitialState() {
	    var url = this.props.url;

	    this.tty = new TtyPlayer({ url: url });
	    return this.calculateState();
	  },
	  componentDidMount: function componentDidMount() {
	    this.terminal = new Term(this.tty, this.refs.container);
	    this.terminal.open();

	    this.tty.on('change', this.updateState);
	    this.tty.play();
	  },
	  updateState: function updateState() {
	    var newState = this.calculateState();
	    this.setState(newState);
	  },
	  componentWillUnmount: function componentWillUnmount() {
	    this.tty.stop();
	    this.tty.removeAllListeners();
	    this.terminal.destroy();
	    $(this.refs.container).perfectScrollbar('destroy');
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
	    var _state = this.state,
	        isPlaying = _state.isPlaying,
	        time = _state.time;


	    return React.createElement(
	      'div',
	      { className: 'grv-current-session grv-session-player' },
	      React.createElement(SessionLeftPanel, null),
	      React.createElement('div', { ref: 'container' }),
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
	          { className: 'grv-session-player-controls-time' },
	          time
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

	exports.default = SessionPlayer;
	module.exports = exports['default'];

/***/ },

/***/ 408:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

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

	var Tty = __webpack_require__(409);
	var api = __webpack_require__(228);

	var _require = __webpack_require__(232),
	    showError = _require.showError;

	var $ = __webpack_require__(219);
	var Buffer = __webpack_require__(411).Buffer;

	var logger = __webpack_require__(230).create('TtyPlayer');
	var STREAM_START_INDEX = 0;
	var PRE_FETCH_BUF_SIZE = 150;
	var URL_PREFIX_EVENTS = '/events';
	var PLAY_SPEED = 5;

	function handleAjaxError(err) {
	  showError('Unable to retrieve session info');
	  logger.error('fetching recorded session info', err);
	}

	var EventProvider = function () {
	  function EventProvider(_ref) {
	    var url = _ref.url;

	    _classCallCheck(this, EventProvider);

	    this.url = url;
	    this.buffSize = PRE_FETCH_BUF_SIZE;
	    this.events = [];
	  }

	  EventProvider.prototype.getLength = function getLength() {
	    return this.events.length;
	  };

	  EventProvider.prototype.getCurrentEventTime = function getCurrentEventTime() {};

	  EventProvider.prototype.getLengthInTime = function getLengthInTime() {
	    var length = this.events.length;
	    if (length === 0) {
	      return 0;
	    }

	    return this.events[length - 1].msNormalized;
	  };

	  EventProvider.prototype.init = function init() {
	    var _this = this;

	    return api.get(this.url + URL_PREFIX_EVENTS).done(function (data) {
	      _this._createPrintEvents(data.events);
	      _this._normalizeEventsByTime();
	    });
	  };

	  EventProvider.prototype.getEventsWithByteStream = function getEventsWithByteStream(start, end) {
	    var _this2 = this;

	    try {
	      if (this._shouldFetch(start, end)) {
	        // TODO: add buffering logic, as for now, load everything
	        return this._fetch().then(this.processByteStream.bind(this, start, this.getLength())).then(function () {
	          return _this2.events.slice(start, end);
	        });
	      } else {
	        return $.Deferred().resolve(this.events.slice(start, end));
	      }
	    } catch (err) {
	      return $.Deferred().reject(err);
	    }
	  };

	  EventProvider.prototype.processByteStream = function processByteStream(start, end, byteStr) {
	    var byteStrOffset = this.events[start].bytes;
	    this.events[start].data = byteStr.slice(0, byteStrOffset).toString('utf8');
	    for (var i = start + 1; i < end; i++) {
	      var bytes = this.events[i].bytes;

	      this.events[i].data = byteStr.slice(byteStrOffset, byteStrOffset + bytes).toString('utf8');
	      byteStrOffset += bytes;
	    }
	  };

	  EventProvider.prototype._shouldFetch = function _shouldFetch(start, end) {
	    for (var i = start; i < end; i++) {
	      if (this.events[i].data === null) {
	        return true;
	      }
	    }

	    return false;
	  };

	  EventProvider.prototype._fetch = function _fetch() {
	    var end = this.events.length - 1;
	    var offset = this.events[0].offset;
	    var bytes = this.events[end].offset - offset + this.events[end].bytes;
	    var url = this.url + '/stream?offset=' + offset + '&bytes=' + bytes;
	    return api.ajax({ url: url, processData: true, dataType: 'text' }).then(function (response) {
	      return new Buffer(response);
	    });
	  };

	  EventProvider.prototype._formatDisplayTime = function _formatDisplayTime(ms) {
	    if (ms < 0) {
	      return '0:0';
	    }

	    var totalSec = Math.floor(ms / 1000);
	    var h = Math.floor(totalSec % 31536000 % 86400 / 3600);
	    var m = Math.floor(totalSec % 31536000 % 86400 % 3600 / 60);
	    var s = totalSec % 31536000 % 86400 % 3600 % 60;

	    m = m > 9 ? m : '0' + m;
	    s = s > 9 ? s : '0' + s;
	    h = h > 0 ? h + ':' : '';

	    return '' + h + m + ':' + s;
	  };

	  EventProvider.prototype._createPrintEvents = function _createPrintEvents(json) {
	    var w = void 0,
	        h = void 0;
	    var events = [];

	    // filter print events and ensure that each event has the right screen size and valid values
	    for (var i = 0; i < json.length; i++) {
	      var _json$i = json[i],
	          ms = _json$i.ms,
	          event = _json$i.event,
	          offset = _json$i.offset,
	          time = _json$i.time,
	          bytes = _json$i.bytes;

	      // grab new screen size for the next events

	      if (event === 'resize' || event === 'session.start') {
	        var _json$i$size$split = json[i].size.split(':');

	        w = _json$i$size$split[0];
	        h = _json$i$size$split[1];
	      }

	      if (event !== 'print') {
	        continue;
	      }

	      var displayTime = this._formatDisplayTime(ms);

	      // use smaller numbers
	      ms = ms > 0 ? Math.floor(ms / 10) : 0;

	      events.push({
	        displayTime: displayTime,
	        ms: ms,
	        msNormalized: ms,
	        bytes: bytes,
	        offset: offset,
	        data: null,
	        w: Number(w),
	        h: Number(h),
	        time: new Date(time)
	      });
	    }

	    this.events = events;
	  };

	  EventProvider.prototype._normalizeEventsByTime = function _normalizeEventsByTime() {
	    var events = this.events;
	    var cur = events[0];
	    var tmp = [];
	    for (var i = 1; i < events.length; i++) {
	      var sameSize = cur.w === events[i].w && cur.h === events[i].h;
	      var delay = events[i].ms - cur.ms;

	      // merge events with tiny delay
	      if (delay < 2 && sameSize) {
	        cur.bytes += events[i].bytes;
	        cur.msNormalized += delay;
	        continue;
	      }

	      // avoid long delays between chunks
	      if (delay >= 25 && delay < 50) {
	        events[i].msNormalized = cur.msNormalized + 25;
	      } else if (delay >= 50 && delay < 100) {
	        events[i].msNormalized = cur.msNormalized + 50;
	      } else if (delay >= 100) {
	        events[i].msNormalized = cur.msNormalized + 100;
	      } else {
	        events[i].msNormalized = cur.msNormalized + delay;
	      }

	      tmp.push(cur);
	      cur = events[i];
	    }

	    if (tmp.indexOf(cur) === -1) {
	      tmp.push(cur);
	    }

	    this.events = tmp;
	  };

	  return EventProvider;
	}();

	var TtyPlayer = function (_Tty) {
	  _inherits(TtyPlayer, _Tty);

	  function TtyPlayer(_ref2) {
	    var url = _ref2.url;

	    _classCallCheck(this, TtyPlayer);

	    var _this3 = _possibleConstructorReturn(this, _Tty.call(this, {}));

	    _this3.currentEventIndex = 0;
	    _this3.current = 0;
	    _this3.length = -1;
	    _this3.isPlaying = false;
	    _this3.isError = false;
	    _this3.isReady = false;
	    _this3.isLoading = true;

	    _this3._posToEventIndexMap = [];
	    _this3._eventProvider = new EventProvider({ url: url });
	    return _this3;
	  }

	  TtyPlayer.prototype.send = function send() {};

	  TtyPlayer.prototype.resize = function resize() {};

	  TtyPlayer.prototype.connect = function connect() {
	    this._setStatusFlag({ isLoading: true });
	    this._eventProvider.init().done(this._init.bind(this)).fail(handleAjaxError).always(this._change.bind(this));

	    this._change();
	  };

	  TtyPlayer.prototype._init = function _init() {
	    var _this4 = this;

	    this.length = this._eventProvider.getLengthInTime();
	    this._eventProvider.events.forEach(function (item) {
	      return _this4._posToEventIndexMap.push(item.msNormalized);
	    });
	    this._setStatusFlag({ isReady: true });
	  };

	  TtyPlayer.prototype.move = function move(newPos) {
	    var _this5 = this;

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

	    if (newPos < 0) {
	      newPos = 0;
	    }

	    var newEventIndex = this._getEventIndex(newPos) + 1;

	    if (newEventIndex === this.currentEventIndex) {
	      this.current = newPos;
	      this._change();
	      return;
	    }

	    try {
	      var isRewind = this.currentEventIndex > newEventIndex;
	      if (isRewind) {
	        this.emit('reset');
	      }

	      this._showChunk(isRewind ? 0 : this.currentEventIndex, newEventIndex).then(function () {
	        _this5.currentEventIndex = newEventIndex;
	        _this5.current = newPos;
	        _this5._change();
	      });
	    } catch (err) {
	      logger.error('move', err);
	    }
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

	    this.timer = setInterval(this.move.bind(this), PLAY_SPEED);
	    this._change();
	  };

	  TtyPlayer.prototype.getCurrentTime = function getCurrentTime() {
	    if (this.currentEventIndex) {
	      var displayTime = this._eventProvider.events[this.currentEventIndex - 1].displayTime;

	      return displayTime;
	    } else {
	      return '';
	    }
	  };

	  TtyPlayer.prototype._showChunk = function _showChunk(start, end) {
	    var _this6 = this;

	    this._setStatusFlag({ isLoading: true });
	    return this._eventProvider.getEventsWithByteStream(start, end).done(function (events) {
	      _this6._setStatusFlag({ isReady: true });
	      _this6._display(events);
	    }).fail(function (err) {
	      _this6._setStatusFlag({ isError: true });
	      handleAjaxError(err);
	    });
	  };

	  TtyPlayer.prototype._display = function _display(stream) {
	    var i = void 0;
	    var tmp = [{
	      data: [stream[0].data],
	      w: stream[0].w,
	      h: stream[0].h
	    }];

	    var cur = tmp[0];

	    for (i = 1; i < stream.length; i++) {
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
	      var _tmp$i = tmp[i],
	          h = _tmp$i.h,
	          w = _tmp$i.w;

	      if (str.length > 0) {
	        this.emit('resize', { h: h, w: w });
	        this.emit('data', str);
	      }
	    }
	  };

	  TtyPlayer.prototype._setStatusFlag = function _setStatusFlag(newStatus) {
	    var _newStatus$isReady = newStatus.isReady,
	        isReady = _newStatus$isReady === undefined ? false : _newStatus$isReady,
	        _newStatus$isError = newStatus.isError,
	        isError = _newStatus$isError === undefined ? false : _newStatus$isError,
	        _newStatus$isLoading = newStatus.isLoading,
	        isLoading = _newStatus$isLoading === undefined ? false : _newStatus$isLoading;

	    this.isReady = isReady;
	    this.isError = isError;
	    this.isLoading = isLoading;
	  };

	  TtyPlayer.prototype._getEventIndex = function _getEventIndex(num) {
	    var arr = this._posToEventIndexMap;
	    var mid;
	    var low = 0;
	    var hi = arr.length - 1;

	    while (hi - low > 1) {
	      mid = Math.floor((low + hi) / 2);
	      if (arr[mid] < num) {
	        low = mid;
	      } else {
	        hi = mid;
	      }
	    }

	    if (num - arr[low] <= arr[hi] - num) {
	      return low;
	    }

	    return hi;
	  };

	  TtyPlayer.prototype._change = function _change() {
	    this.emit('change');
	  };

	  return TtyPlayer;
	}(Tty);

	exports.default = TtyPlayer;
	exports.EventProvider = EventProvider;
	exports.TtyPlayer = TtyPlayer;
	exports.Buffer = Buffer;

/***/ },

/***/ 409:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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

	var EventEmitter = __webpack_require__(410).EventEmitter;

	var Tty = function (_EventEmitter) {
	  _inherits(Tty, _EventEmitter);

	  function Tty() {
	    _classCallCheck(this, Tty);

	    var _this = _possibleConstructorReturn(this, _EventEmitter.call(this));

	    _this.socket = null;
	    return _this;
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
	    var _this2 = this;

	    this.socket = new WebSocket(connStr);

	    this.socket.onopen = function () {
	      _this2.emit('open');
	    };

	    this.socket.onmessage = function (e) {
	      _this2.emit('data', e.data);
	    };

	    this.socket.onclose = function () {
	      _this2.emit('close');
	    };
	  };

	  Tty.prototype.send = function send(data) {
	    this.socket.send(data);
	  };

	  return Tty;
	}(EventEmitter);

	module.exports = Tty;

/***/ },

/***/ 410:
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

/***/ 416:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

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

	var Term = __webpack_require__(417);
	var Tty = __webpack_require__(409);
	var TtyEvents = __webpack_require__(419);

	var _require = __webpack_require__(383),
	    debounce = _require.debounce,
	    isNumber = _require.isNumber;

	var api = __webpack_require__(228);
	var logger = __webpack_require__(230).create('terminal');
	var $ = __webpack_require__(219);

	Term.colors[256] = '#252323';

	var DISCONNECT_TXT = '\x1b[31mdisconnected\x1b[m\r\n';
	var CONNECTED_TXT = 'Connected!\r\n';
	var GRV_CLASS = 'grv-terminal';
	var WINDOW_RESIZE_DEBOUNCE_DELAY = 100;

	var TtyTerminal = function () {
	  function TtyTerminal(options) {
	    _classCallCheck(this, TtyTerminal);

	    var tty = options.tty,
	        _options$scrollBack = options.scrollBack,
	        scrollBack = _options$scrollBack === undefined ? 1000 : _options$scrollBack;


	    this.ttyParams = tty;
	    this.tty = new Tty();
	    this.ttyEvents = new TtyEvents();

	    this.scrollBack = scrollBack;
	    this.rows = undefined;
	    this.cols = undefined;
	    this.term = null;
	    this._el = options.el;

	    this.debouncedResize = debounce(this._requestResize.bind(this), WINDOW_RESIZE_DEBOUNCE_DELAY);
	  }

	  TtyTerminal.prototype.open = function open() {
	    var _this = this;

	    $(this._el).addClass(GRV_CLASS);

	    // render termjs with default values (will be used to calculate the character size)
	    this.term = new Term({
	      cols: 15,
	      rows: 5,
	      scrollback: this.scrollBack,
	      useStyle: true,
	      screenKeys: true,
	      cursorBlink: true
	    });

	    this.term.open(this._el);

	    // resize to available space (by given container)
	    this.resize(this.cols, this.rows);

	    // subscribe termjs events
	    this.term.on('data', function (data) {
	      return _this.tty.send(data);
	    });

	    // subscribe to tty events
	    this.tty.on('resize', function (_ref) {
	      var h = _ref.h,
	          w = _ref.w;
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
	    this.tty.on('data', this._processData.bind(this));

	    this.connect();
	    window.addEventListener('resize', this.debouncedResize);
	  };

	  TtyTerminal.prototype.connect = function connect() {
	    this.tty.connect(this._getTtyConnStr());
	    this.ttyEvents.connect(this._getTtyEventsConnStr());
	  };

	  TtyTerminal.prototype.destroy = function destroy() {
	    this._disconnect();

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

	    if (cols === this.cols && rows === this.rows) {
	      return;
	    }

	    this.cols = cols;
	    this.rows = rows;
	    this.term.resize(this.cols, this.rows);
	  };

	  TtyTerminal.prototype._processData = function _processData(data) {
	    try {
	      data = this._ensureScreenSize(data);
	      this.term.write(data);
	    } catch (err) {
	      logger.error({
	        w: this.cols,
	        h: this.rows,
	        text: 'failed to resize termjs',
	        data: data,
	        err: err
	      });
	    }
	  };

	  TtyTerminal.prototype._ensureScreenSize = function _ensureScreenSize(data) {
	    /**
	    * for better sync purposes, the screen values are inserted to the end of the chunk
	    * with the following format: '\0NUMBER:NUMBER'
	    */
	    var pos = data.lastIndexOf('\0');
	    if (pos !== -1) {
	      var length = data.length - pos;
	      if (length > 2 && length < 10) {
	        var tmp = data.substr(pos + 1);

	        var _tmp$split = tmp.split(':'),
	            w = _tmp$split[0],
	            h = _tmp$split[1];

	        if ($.isNumeric(w) && $.isNumeric(h)) {
	          w = Number(w);
	          h = Number(h);

	          if (w < 500 && h < 500) {
	            data = data.slice(0, pos);
	            this.resize(w, h);
	          }
	        }
	      }
	    }

	    return data;
	  };

	  TtyTerminal.prototype._disconnect = function _disconnect() {
	    if (this.tty !== null) {
	      this.tty.disconnect();
	    }

	    if (this.ttyEvents !== null) {
	      this.ttyEvents.disconnect();
	      this.ttyEvents.removeAllListeners();
	    }
	  };

	  TtyTerminal.prototype._requestResize = function _requestResize() {
	    var _getDimensions2 = this._getDimensions(),
	        cols = _getDimensions2.cols,
	        rows = _getDimensions2.rows;

	    var w = cols;
	    var h = rows;

	    // some min values
	    w = w < 5 ? 5 : w;
	    h = h < 5 ? 5 : h;

	    var _ttyParams = this.ttyParams,
	        sid = _ttyParams.sid,
	        url = _ttyParams.url;

	    var reqData = { terminal_params: { w: w, h: h } };

	    logger.info('request new screen size', 'w:' + w + ' and h:' + h);

	    api.put(url + '/sessions/' + sid, reqData).done(function () {
	      return logger.info('new screen size requested');
	    }).fail(function (err) {
	      return logger.error('request new screen size', err);
	    });
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
	    var _ttyParams2 = this.ttyParams,
	        sid = _ttyParams2.sid,
	        url = _ttyParams2.url,
	        token = _ttyParams2.token;

	    var urlPrefix = getWsHostName();
	    return '' + urlPrefix + url + '/sessions/' + sid + '/events/stream?access_token=' + token;
	  };

	  TtyTerminal.prototype._getTtyConnStr = function _getTtyConnStr() {
	    var _ttyParams3 = this.ttyParams,
	        serverId = _ttyParams3.serverId,
	        login = _ttyParams3.login,
	        sid = _ttyParams3.sid,
	        url = _ttyParams3.url,
	        token = _ttyParams3.token;

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
	    var urlPrefix = getWsHostName();

	    return '' + urlPrefix + url + '/connect?access_token=' + token + '&params=' + jsonEncoded;
	  };

	  return TtyTerminal;
	}();

	function getWsHostName() {
	  var prefix = location.protocol == "https:" ? "wss://" : "ws://";
	  var hostport = location.hostname + (location.port ? ':' + location.port : '');
	  return '' + prefix + hostport;
	}

	module.exports = TtyTerminal;

/***/ },

/***/ 419:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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

	var EventEmitter = __webpack_require__(410).EventEmitter;

	var logger = __webpack_require__(230).create('TtyEvents');

	var TtyEvents = function (_EventEmitter) {
	  _inherits(TtyEvents, _EventEmitter);

	  function TtyEvents() {
	    _classCallCheck(this, TtyEvents);

	    var _this = _possibleConstructorReturn(this, _EventEmitter.call(this));

	    _this.socket = null;
	    return _this;
	  }

	  TtyEvents.prototype.connect = function connect(connStr) {
	    var _this2 = this;

	    this.socket = new WebSocket(connStr);

	    this.socket.onopen = function () {
	      logger.info('Tty event stream is open');
	    };

	    this.socket.onmessage = function (event) {
	      try {
	        var json = JSON.parse(event.data);
	        _this2.emit('data', json);
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
	}(EventEmitter);

	module.exports = TtyEvents;

/***/ },

/***/ 420:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

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

	var React = __webpack_require__(2);

	var _require = __webpack_require__(404),
	    actions = _require.actions;

	var _require2 = __webpack_require__(341),
	    UserIcon = _require2.UserIcon;

	var ReactCSSTransitionGroup = __webpack_require__(421);

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
	          React.createElement(
	            'span',
	            null,
	            '\u2716'
	          )
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

/***/ },

/***/ 449:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

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

	var React = __webpack_require__(2);
	var reactor = __webpack_require__(215);

	var _require = __webpack_require__(380),
	    nodeHostNameByServerId = _require.nodeHostNameByServerId;

	var SessionLeftPanel = __webpack_require__(420);
	var cfg = __webpack_require__(217);
	var session = __webpack_require__(229);
	var Terminal = __webpack_require__(416);

	var _require2 = __webpack_require__(386),
	    processSessionFromEventStream = _require2.processSessionFromEventStream;

	var ActiveSession = React.createClass({
	  displayName: 'ActiveSession',
	  render: function render() {
	    var _props = this.props,
	        login = _props.login,
	        parties = _props.parties,
	        serverId = _props.serverId;

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

	var TtyTerminal = React.createClass({
	  displayName: 'TtyTerminal',
	  componentDidMount: function componentDidMount() {
	    var _props2 = this.props,
	        serverId = _props2.serverId,
	        siteId = _props2.siteId,
	        login = _props2.login,
	        sid = _props2.sid,
	        rows = _props2.rows,
	        cols = _props2.cols;

	    var _session$getUserData = session.getUserData(),
	        token = _session$getUserData.token;

	    var url = cfg.api.getSiteUrl(siteId);

	    var options = {
	      tty: {
	        serverId: serverId, login: login, sid: sid, token: token, url: url
	      },
	      rows: rows,
	      cols: cols,
	      el: this.refs.container
	    };

	    this.terminal = new Terminal(options);
	    this.terminal.ttyEvents.on('data', processSessionFromEventStream(siteId));
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

	module.exports = ActiveSession;

/***/ },

/***/ 450:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	var _reactor = __webpack_require__(215);

	var _reactor2 = _interopRequireDefault(_reactor);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	_reactor2.default.registerStores({
	  'tlpt': __webpack_require__(348),
	  'tlpt_current_session': __webpack_require__(405),
	  'tlpt_user': __webpack_require__(375),
	  'tlpt_sites': __webpack_require__(451),
	  'tlpt_user_invite': __webpack_require__(453),
	  'tlpt_nodes': __webpack_require__(454),
	  'tlpt_rest_api': __webpack_require__(455),
	  'tlpt_sessions': __webpack_require__(456),
	  'tlpt_stored_sessions_filter': __webpack_require__(457),
	  'tlpt_notifications': __webpack_require__(458)
	}); /*
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

/***/ },

/***/ 451:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _nuclearJs = __webpack_require__(216);

	var _actionTypes = __webpack_require__(235);

	var _immutable = __webpack_require__(452);

	var Site = (0, _immutable.Record)({
	  name: null,
	  status: false
	}); /*
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

	exports.default = (0, _nuclearJs.Store)({
	  getInitialState: function getInitialState() {
	    return new _immutable.List();
	  },
	  initialize: function initialize() {
	    this.on(_actionTypes.TLPT_SITES_RECEIVE, receiveSites);
	  }
	});


	function receiveSites(state, json) {
	  return (0, _nuclearJs.toImmutable)(json).map(function (o) {
	    return new Site(o);
	  });
	}
	module.exports = exports['default'];

/***/ },

/***/ 452:
/***/ function(module, exports, __webpack_require__) {

	/**
	 *  Copyright (c) 2014-2015, Facebook, Inc.
	 *  All rights reserved.
	 *
	 *  This source code is licensed under the BSD-style license found in the
	 *  LICENSE file in the root directory of this source tree. An additional grant
	 *  of patent rights can be found in the PATENTS file in the same directory.
	 */

	(function (global, factory) {
	   true ? module.exports = factory() :
	  typeof define === 'function' && define.amd ? define(factory) :
	  (global.Immutable = factory());
	}(this, function () { 'use strict';var SLICE$0 = Array.prototype.slice;

	  function createClass(ctor, superClass) {
	    if (superClass) {
	      ctor.prototype = Object.create(superClass.prototype);
	    }
	    ctor.prototype.constructor = ctor;
	  }

	  function Iterable(value) {
	      return isIterable(value) ? value : Seq(value);
	    }


	  createClass(KeyedIterable, Iterable);
	    function KeyedIterable(value) {
	      return isKeyed(value) ? value : KeyedSeq(value);
	    }


	  createClass(IndexedIterable, Iterable);
	    function IndexedIterable(value) {
	      return isIndexed(value) ? value : IndexedSeq(value);
	    }


	  createClass(SetIterable, Iterable);
	    function SetIterable(value) {
	      return isIterable(value) && !isAssociative(value) ? value : SetSeq(value);
	    }



	  function isIterable(maybeIterable) {
	    return !!(maybeIterable && maybeIterable[IS_ITERABLE_SENTINEL]);
	  }

	  function isKeyed(maybeKeyed) {
	    return !!(maybeKeyed && maybeKeyed[IS_KEYED_SENTINEL]);
	  }

	  function isIndexed(maybeIndexed) {
	    return !!(maybeIndexed && maybeIndexed[IS_INDEXED_SENTINEL]);
	  }

	  function isAssociative(maybeAssociative) {
	    return isKeyed(maybeAssociative) || isIndexed(maybeAssociative);
	  }

	  function isOrdered(maybeOrdered) {
	    return !!(maybeOrdered && maybeOrdered[IS_ORDERED_SENTINEL]);
	  }

	  Iterable.isIterable = isIterable;
	  Iterable.isKeyed = isKeyed;
	  Iterable.isIndexed = isIndexed;
	  Iterable.isAssociative = isAssociative;
	  Iterable.isOrdered = isOrdered;

	  Iterable.Keyed = KeyedIterable;
	  Iterable.Indexed = IndexedIterable;
	  Iterable.Set = SetIterable;


	  var IS_ITERABLE_SENTINEL = '@@__IMMUTABLE_ITERABLE__@@';
	  var IS_KEYED_SENTINEL = '@@__IMMUTABLE_KEYED__@@';
	  var IS_INDEXED_SENTINEL = '@@__IMMUTABLE_INDEXED__@@';
	  var IS_ORDERED_SENTINEL = '@@__IMMUTABLE_ORDERED__@@';

	  // Used for setting prototype methods that IE8 chokes on.
	  var DELETE = 'delete';

	  // Constants describing the size of trie nodes.
	  var SHIFT = 5; // Resulted in best performance after ______?
	  var SIZE = 1 << SHIFT;
	  var MASK = SIZE - 1;

	  // A consistent shared value representing "not set" which equals nothing other
	  // than itself, and nothing that could be provided externally.
	  var NOT_SET = {};

	  // Boolean references, Rough equivalent of `bool &`.
	  var CHANGE_LENGTH = { value: false };
	  var DID_ALTER = { value: false };

	  function MakeRef(ref) {
	    ref.value = false;
	    return ref;
	  }

	  function SetRef(ref) {
	    ref && (ref.value = true);
	  }

	  // A function which returns a value representing an "owner" for transient writes
	  // to tries. The return value will only ever equal itself, and will not equal
	  // the return of any subsequent call of this function.
	  function OwnerID() {}

	  // http://jsperf.com/copy-array-inline
	  function arrCopy(arr, offset) {
	    offset = offset || 0;
	    var len = Math.max(0, arr.length - offset);
	    var newArr = new Array(len);
	    for (var ii = 0; ii < len; ii++) {
	      newArr[ii] = arr[ii + offset];
	    }
	    return newArr;
	  }

	  function ensureSize(iter) {
	    if (iter.size === undefined) {
	      iter.size = iter.__iterate(returnTrue);
	    }
	    return iter.size;
	  }

	  function wrapIndex(iter, index) {
	    // This implements "is array index" which the ECMAString spec defines as:
	    //
	    //     A String property name P is an array index if and only if
	    //     ToString(ToUint32(P)) is equal to P and ToUint32(P) is not equal
	    //     to 2^32−1.
	    //
	    // http://www.ecma-international.org/ecma-262/6.0/#sec-array-exotic-objects
	    if (typeof index !== 'number') {
	      var uint32Index = index >>> 0; // N >>> 0 is shorthand for ToUint32
	      if ('' + uint32Index !== index || uint32Index === 4294967295) {
	        return NaN;
	      }
	      index = uint32Index;
	    }
	    return index < 0 ? ensureSize(iter) + index : index;
	  }

	  function returnTrue() {
	    return true;
	  }

	  function wholeSlice(begin, end, size) {
	    return (begin === 0 || (size !== undefined && begin <= -size)) &&
	      (end === undefined || (size !== undefined && end >= size));
	  }

	  function resolveBegin(begin, size) {
	    return resolveIndex(begin, size, 0);
	  }

	  function resolveEnd(end, size) {
	    return resolveIndex(end, size, size);
	  }

	  function resolveIndex(index, size, defaultIndex) {
	    return index === undefined ?
	      defaultIndex :
	      index < 0 ?
	        Math.max(0, size + index) :
	        size === undefined ?
	          index :
	          Math.min(size, index);
	  }

	  /* global Symbol */

	  var ITERATE_KEYS = 0;
	  var ITERATE_VALUES = 1;
	  var ITERATE_ENTRIES = 2;

	  var REAL_ITERATOR_SYMBOL = typeof Symbol === 'function' && Symbol.iterator;
	  var FAUX_ITERATOR_SYMBOL = '@@iterator';

	  var ITERATOR_SYMBOL = REAL_ITERATOR_SYMBOL || FAUX_ITERATOR_SYMBOL;


	  function Iterator(next) {
	      this.next = next;
	    }

	    Iterator.prototype.toString = function() {
	      return '[Iterator]';
	    };


	  Iterator.KEYS = ITERATE_KEYS;
	  Iterator.VALUES = ITERATE_VALUES;
	  Iterator.ENTRIES = ITERATE_ENTRIES;

	  Iterator.prototype.inspect =
	  Iterator.prototype.toSource = function () { return this.toString(); }
	  Iterator.prototype[ITERATOR_SYMBOL] = function () {
	    return this;
	  };


	  function iteratorValue(type, k, v, iteratorResult) {
	    var value = type === 0 ? k : type === 1 ? v : [k, v];
	    iteratorResult ? (iteratorResult.value = value) : (iteratorResult = {
	      value: value, done: false
	    });
	    return iteratorResult;
	  }

	  function iteratorDone() {
	    return { value: undefined, done: true };
	  }

	  function hasIterator(maybeIterable) {
	    return !!getIteratorFn(maybeIterable);
	  }

	  function isIterator(maybeIterator) {
	    return maybeIterator && typeof maybeIterator.next === 'function';
	  }

	  function getIterator(iterable) {
	    var iteratorFn = getIteratorFn(iterable);
	    return iteratorFn && iteratorFn.call(iterable);
	  }

	  function getIteratorFn(iterable) {
	    var iteratorFn = iterable && (
	      (REAL_ITERATOR_SYMBOL && iterable[REAL_ITERATOR_SYMBOL]) ||
	      iterable[FAUX_ITERATOR_SYMBOL]
	    );
	    if (typeof iteratorFn === 'function') {
	      return iteratorFn;
	    }
	  }

	  function isArrayLike(value) {
	    return value && typeof value.length === 'number';
	  }

	  createClass(Seq, Iterable);
	    function Seq(value) {
	      return value === null || value === undefined ? emptySequence() :
	        isIterable(value) ? value.toSeq() : seqFromValue(value);
	    }

	    Seq.of = function(/*...values*/) {
	      return Seq(arguments);
	    };

	    Seq.prototype.toSeq = function() {
	      return this;
	    };

	    Seq.prototype.toString = function() {
	      return this.__toString('Seq {', '}');
	    };

	    Seq.prototype.cacheResult = function() {
	      if (!this._cache && this.__iterateUncached) {
	        this._cache = this.entrySeq().toArray();
	        this.size = this._cache.length;
	      }
	      return this;
	    };

	    // abstract __iterateUncached(fn, reverse)

	    Seq.prototype.__iterate = function(fn, reverse) {
	      return seqIterate(this, fn, reverse, true);
	    };

	    // abstract __iteratorUncached(type, reverse)

	    Seq.prototype.__iterator = function(type, reverse) {
	      return seqIterator(this, type, reverse, true);
	    };



	  createClass(KeyedSeq, Seq);
	    function KeyedSeq(value) {
	      return value === null || value === undefined ?
	        emptySequence().toKeyedSeq() :
	        isIterable(value) ?
	          (isKeyed(value) ? value.toSeq() : value.fromEntrySeq()) :
	          keyedSeqFromValue(value);
	    }

	    KeyedSeq.prototype.toKeyedSeq = function() {
	      return this;
	    };



	  createClass(IndexedSeq, Seq);
	    function IndexedSeq(value) {
	      return value === null || value === undefined ? emptySequence() :
	        !isIterable(value) ? indexedSeqFromValue(value) :
	        isKeyed(value) ? value.entrySeq() : value.toIndexedSeq();
	    }

	    IndexedSeq.of = function(/*...values*/) {
	      return IndexedSeq(arguments);
	    };

	    IndexedSeq.prototype.toIndexedSeq = function() {
	      return this;
	    };

	    IndexedSeq.prototype.toString = function() {
	      return this.__toString('Seq [', ']');
	    };

	    IndexedSeq.prototype.__iterate = function(fn, reverse) {
	      return seqIterate(this, fn, reverse, false);
	    };

	    IndexedSeq.prototype.__iterator = function(type, reverse) {
	      return seqIterator(this, type, reverse, false);
	    };



	  createClass(SetSeq, Seq);
	    function SetSeq(value) {
	      return (
	        value === null || value === undefined ? emptySequence() :
	        !isIterable(value) ? indexedSeqFromValue(value) :
	        isKeyed(value) ? value.entrySeq() : value
	      ).toSetSeq();
	    }

	    SetSeq.of = function(/*...values*/) {
	      return SetSeq(arguments);
	    };

	    SetSeq.prototype.toSetSeq = function() {
	      return this;
	    };



	  Seq.isSeq = isSeq;
	  Seq.Keyed = KeyedSeq;
	  Seq.Set = SetSeq;
	  Seq.Indexed = IndexedSeq;

	  var IS_SEQ_SENTINEL = '@@__IMMUTABLE_SEQ__@@';

	  Seq.prototype[IS_SEQ_SENTINEL] = true;



	  createClass(ArraySeq, IndexedSeq);
	    function ArraySeq(array) {
	      this._array = array;
	      this.size = array.length;
	    }

	    ArraySeq.prototype.get = function(index, notSetValue) {
	      return this.has(index) ? this._array[wrapIndex(this, index)] : notSetValue;
	    };

	    ArraySeq.prototype.__iterate = function(fn, reverse) {
	      var array = this._array;
	      var maxIndex = array.length - 1;
	      for (var ii = 0; ii <= maxIndex; ii++) {
	        if (fn(array[reverse ? maxIndex - ii : ii], ii, this) === false) {
	          return ii + 1;
	        }
	      }
	      return ii;
	    };

	    ArraySeq.prototype.__iterator = function(type, reverse) {
	      var array = this._array;
	      var maxIndex = array.length - 1;
	      var ii = 0;
	      return new Iterator(function() 
	        {return ii > maxIndex ?
	          iteratorDone() :
	          iteratorValue(type, ii, array[reverse ? maxIndex - ii++ : ii++])}
	      );
	    };



	  createClass(ObjectSeq, KeyedSeq);
	    function ObjectSeq(object) {
	      var keys = Object.keys(object);
	      this._object = object;
	      this._keys = keys;
	      this.size = keys.length;
	    }

	    ObjectSeq.prototype.get = function(key, notSetValue) {
	      if (notSetValue !== undefined && !this.has(key)) {
	        return notSetValue;
	      }
	      return this._object[key];
	    };

	    ObjectSeq.prototype.has = function(key) {
	      return this._object.hasOwnProperty(key);
	    };

	    ObjectSeq.prototype.__iterate = function(fn, reverse) {
	      var object = this._object;
	      var keys = this._keys;
	      var maxIndex = keys.length - 1;
	      for (var ii = 0; ii <= maxIndex; ii++) {
	        var key = keys[reverse ? maxIndex - ii : ii];
	        if (fn(object[key], key, this) === false) {
	          return ii + 1;
	        }
	      }
	      return ii;
	    };

	    ObjectSeq.prototype.__iterator = function(type, reverse) {
	      var object = this._object;
	      var keys = this._keys;
	      var maxIndex = keys.length - 1;
	      var ii = 0;
	      return new Iterator(function()  {
	        var key = keys[reverse ? maxIndex - ii : ii];
	        return ii++ > maxIndex ?
	          iteratorDone() :
	          iteratorValue(type, key, object[key]);
	      });
	    };

	  ObjectSeq.prototype[IS_ORDERED_SENTINEL] = true;


	  createClass(IterableSeq, IndexedSeq);
	    function IterableSeq(iterable) {
	      this._iterable = iterable;
	      this.size = iterable.length || iterable.size;
	    }

	    IterableSeq.prototype.__iterateUncached = function(fn, reverse) {
	      if (reverse) {
	        return this.cacheResult().__iterate(fn, reverse);
	      }
	      var iterable = this._iterable;
	      var iterator = getIterator(iterable);
	      var iterations = 0;
	      if (isIterator(iterator)) {
	        var step;
	        while (!(step = iterator.next()).done) {
	          if (fn(step.value, iterations++, this) === false) {
	            break;
	          }
	        }
	      }
	      return iterations;
	    };

	    IterableSeq.prototype.__iteratorUncached = function(type, reverse) {
	      if (reverse) {
	        return this.cacheResult().__iterator(type, reverse);
	      }
	      var iterable = this._iterable;
	      var iterator = getIterator(iterable);
	      if (!isIterator(iterator)) {
	        return new Iterator(iteratorDone);
	      }
	      var iterations = 0;
	      return new Iterator(function()  {
	        var step = iterator.next();
	        return step.done ? step : iteratorValue(type, iterations++, step.value);
	      });
	    };



	  createClass(IteratorSeq, IndexedSeq);
	    function IteratorSeq(iterator) {
	      this._iterator = iterator;
	      this._iteratorCache = [];
	    }

	    IteratorSeq.prototype.__iterateUncached = function(fn, reverse) {
	      if (reverse) {
	        return this.cacheResult().__iterate(fn, reverse);
	      }
	      var iterator = this._iterator;
	      var cache = this._iteratorCache;
	      var iterations = 0;
	      while (iterations < cache.length) {
	        if (fn(cache[iterations], iterations++, this) === false) {
	          return iterations;
	        }
	      }
	      var step;
	      while (!(step = iterator.next()).done) {
	        var val = step.value;
	        cache[iterations] = val;
	        if (fn(val, iterations++, this) === false) {
	          break;
	        }
	      }
	      return iterations;
	    };

	    IteratorSeq.prototype.__iteratorUncached = function(type, reverse) {
	      if (reverse) {
	        return this.cacheResult().__iterator(type, reverse);
	      }
	      var iterator = this._iterator;
	      var cache = this._iteratorCache;
	      var iterations = 0;
	      return new Iterator(function()  {
	        if (iterations >= cache.length) {
	          var step = iterator.next();
	          if (step.done) {
	            return step;
	          }
	          cache[iterations] = step.value;
	        }
	        return iteratorValue(type, iterations, cache[iterations++]);
	      });
	    };




	  // # pragma Helper functions

	  function isSeq(maybeSeq) {
	    return !!(maybeSeq && maybeSeq[IS_SEQ_SENTINEL]);
	  }

	  var EMPTY_SEQ;

	  function emptySequence() {
	    return EMPTY_SEQ || (EMPTY_SEQ = new ArraySeq([]));
	  }

	  function keyedSeqFromValue(value) {
	    var seq =
	      Array.isArray(value) ? new ArraySeq(value).fromEntrySeq() :
	      isIterator(value) ? new IteratorSeq(value).fromEntrySeq() :
	      hasIterator(value) ? new IterableSeq(value).fromEntrySeq() :
	      typeof value === 'object' ? new ObjectSeq(value) :
	      undefined;
	    if (!seq) {
	      throw new TypeError(
	        'Expected Array or iterable object of [k, v] entries, '+
	        'or keyed object: ' + value
	      );
	    }
	    return seq;
	  }

	  function indexedSeqFromValue(value) {
	    var seq = maybeIndexedSeqFromValue(value);
	    if (!seq) {
	      throw new TypeError(
	        'Expected Array or iterable object of values: ' + value
	      );
	    }
	    return seq;
	  }

	  function seqFromValue(value) {
	    var seq = maybeIndexedSeqFromValue(value) ||
	      (typeof value === 'object' && new ObjectSeq(value));
	    if (!seq) {
	      throw new TypeError(
	        'Expected Array or iterable object of values, or keyed object: ' + value
	      );
	    }
	    return seq;
	  }

	  function maybeIndexedSeqFromValue(value) {
	    return (
	      isArrayLike(value) ? new ArraySeq(value) :
	      isIterator(value) ? new IteratorSeq(value) :
	      hasIterator(value) ? new IterableSeq(value) :
	      undefined
	    );
	  }

	  function seqIterate(seq, fn, reverse, useKeys) {
	    var cache = seq._cache;
	    if (cache) {
	      var maxIndex = cache.length - 1;
	      for (var ii = 0; ii <= maxIndex; ii++) {
	        var entry = cache[reverse ? maxIndex - ii : ii];
	        if (fn(entry[1], useKeys ? entry[0] : ii, seq) === false) {
	          return ii + 1;
	        }
	      }
	      return ii;
	    }
	    return seq.__iterateUncached(fn, reverse);
	  }

	  function seqIterator(seq, type, reverse, useKeys) {
	    var cache = seq._cache;
	    if (cache) {
	      var maxIndex = cache.length - 1;
	      var ii = 0;
	      return new Iterator(function()  {
	        var entry = cache[reverse ? maxIndex - ii : ii];
	        return ii++ > maxIndex ?
	          iteratorDone() :
	          iteratorValue(type, useKeys ? entry[0] : ii - 1, entry[1]);
	      });
	    }
	    return seq.__iteratorUncached(type, reverse);
	  }

	  function fromJS(json, converter) {
	    return converter ?
	      fromJSWith(converter, json, '', {'': json}) :
	      fromJSDefault(json);
	  }

	  function fromJSWith(converter, json, key, parentJSON) {
	    if (Array.isArray(json)) {
	      return converter.call(parentJSON, key, IndexedSeq(json).map(function(v, k)  {return fromJSWith(converter, v, k, json)}));
	    }
	    if (isPlainObj(json)) {
	      return converter.call(parentJSON, key, KeyedSeq(json).map(function(v, k)  {return fromJSWith(converter, v, k, json)}));
	    }
	    return json;
	  }

	  function fromJSDefault(json) {
	    if (Array.isArray(json)) {
	      return IndexedSeq(json).map(fromJSDefault).toList();
	    }
	    if (isPlainObj(json)) {
	      return KeyedSeq(json).map(fromJSDefault).toMap();
	    }
	    return json;
	  }

	  function isPlainObj(value) {
	    return value && (value.constructor === Object || value.constructor === undefined);
	  }

	  /**
	   * An extension of the "same-value" algorithm as [described for use by ES6 Map
	   * and Set](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Map#Key_equality)
	   *
	   * NaN is considered the same as NaN, however -0 and 0 are considered the same
	   * value, which is different from the algorithm described by
	   * [`Object.is`](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Object/is).
	   *
	   * This is extended further to allow Objects to describe the values they
	   * represent, by way of `valueOf` or `equals` (and `hashCode`).
	   *
	   * Note: because of this extension, the key equality of Immutable.Map and the
	   * value equality of Immutable.Set will differ from ES6 Map and Set.
	   *
	   * ### Defining custom values
	   *
	   * The easiest way to describe the value an object represents is by implementing
	   * `valueOf`. For example, `Date` represents a value by returning a unix
	   * timestamp for `valueOf`:
	   *
	   *     var date1 = new Date(1234567890000); // Fri Feb 13 2009 ...
	   *     var date2 = new Date(1234567890000);
	   *     date1.valueOf(); // 1234567890000
	   *     assert( date1 !== date2 );
	   *     assert( Immutable.is( date1, date2 ) );
	   *
	   * Note: overriding `valueOf` may have other implications if you use this object
	   * where JavaScript expects a primitive, such as implicit string coercion.
	   *
	   * For more complex types, especially collections, implementing `valueOf` may
	   * not be performant. An alternative is to implement `equals` and `hashCode`.
	   *
	   * `equals` takes another object, presumably of similar type, and returns true
	   * if the it is equal. Equality is symmetrical, so the same result should be
	   * returned if this and the argument are flipped.
	   *
	   *     assert( a.equals(b) === b.equals(a) );
	   *
	   * `hashCode` returns a 32bit integer number representing the object which will
	   * be used to determine how to store the value object in a Map or Set. You must
	   * provide both or neither methods, one must not exist without the other.
	   *
	   * Also, an important relationship between these methods must be upheld: if two
	   * values are equal, they *must* return the same hashCode. If the values are not
	   * equal, they might have the same hashCode; this is called a hash collision,
	   * and while undesirable for performance reasons, it is acceptable.
	   *
	   *     if (a.equals(b)) {
	   *       assert( a.hashCode() === b.hashCode() );
	   *     }
	   *
	   * All Immutable collections implement `equals` and `hashCode`.
	   *
	   */
	  function is(valueA, valueB) {
	    if (valueA === valueB || (valueA !== valueA && valueB !== valueB)) {
	      return true;
	    }
	    if (!valueA || !valueB) {
	      return false;
	    }
	    if (typeof valueA.valueOf === 'function' &&
	        typeof valueB.valueOf === 'function') {
	      valueA = valueA.valueOf();
	      valueB = valueB.valueOf();
	      if (valueA === valueB || (valueA !== valueA && valueB !== valueB)) {
	        return true;
	      }
	      if (!valueA || !valueB) {
	        return false;
	      }
	    }
	    if (typeof valueA.equals === 'function' &&
	        typeof valueB.equals === 'function' &&
	        valueA.equals(valueB)) {
	      return true;
	    }
	    return false;
	  }

	  function deepEqual(a, b) {
	    if (a === b) {
	      return true;
	    }

	    if (
	      !isIterable(b) ||
	      a.size !== undefined && b.size !== undefined && a.size !== b.size ||
	      a.__hash !== undefined && b.__hash !== undefined && a.__hash !== b.__hash ||
	      isKeyed(a) !== isKeyed(b) ||
	      isIndexed(a) !== isIndexed(b) ||
	      isOrdered(a) !== isOrdered(b)
	    ) {
	      return false;
	    }

	    if (a.size === 0 && b.size === 0) {
	      return true;
	    }

	    var notAssociative = !isAssociative(a);

	    if (isOrdered(a)) {
	      var entries = a.entries();
	      return b.every(function(v, k)  {
	        var entry = entries.next().value;
	        return entry && is(entry[1], v) && (notAssociative || is(entry[0], k));
	      }) && entries.next().done;
	    }

	    var flipped = false;

	    if (a.size === undefined) {
	      if (b.size === undefined) {
	        if (typeof a.cacheResult === 'function') {
	          a.cacheResult();
	        }
	      } else {
	        flipped = true;
	        var _ = a;
	        a = b;
	        b = _;
	      }
	    }

	    var allEqual = true;
	    var bSize = b.__iterate(function(v, k)  {
	      if (notAssociative ? !a.has(v) :
	          flipped ? !is(v, a.get(k, NOT_SET)) : !is(a.get(k, NOT_SET), v)) {
	        allEqual = false;
	        return false;
	      }
	    });

	    return allEqual && a.size === bSize;
	  }

	  createClass(Repeat, IndexedSeq);

	    function Repeat(value, times) {
	      if (!(this instanceof Repeat)) {
	        return new Repeat(value, times);
	      }
	      this._value = value;
	      this.size = times === undefined ? Infinity : Math.max(0, times);
	      if (this.size === 0) {
	        if (EMPTY_REPEAT) {
	          return EMPTY_REPEAT;
	        }
	        EMPTY_REPEAT = this;
	      }
	    }

	    Repeat.prototype.toString = function() {
	      if (this.size === 0) {
	        return 'Repeat []';
	      }
	      return 'Repeat [ ' + this._value + ' ' + this.size + ' times ]';
	    };

	    Repeat.prototype.get = function(index, notSetValue) {
	      return this.has(index) ? this._value : notSetValue;
	    };

	    Repeat.prototype.includes = function(searchValue) {
	      return is(this._value, searchValue);
	    };

	    Repeat.prototype.slice = function(begin, end) {
	      var size = this.size;
	      return wholeSlice(begin, end, size) ? this :
	        new Repeat(this._value, resolveEnd(end, size) - resolveBegin(begin, size));
	    };

	    Repeat.prototype.reverse = function() {
	      return this;
	    };

	    Repeat.prototype.indexOf = function(searchValue) {
	      if (is(this._value, searchValue)) {
	        return 0;
	      }
	      return -1;
	    };

	    Repeat.prototype.lastIndexOf = function(searchValue) {
	      if (is(this._value, searchValue)) {
	        return this.size;
	      }
	      return -1;
	    };

	    Repeat.prototype.__iterate = function(fn, reverse) {
	      for (var ii = 0; ii < this.size; ii++) {
	        if (fn(this._value, ii, this) === false) {
	          return ii + 1;
	        }
	      }
	      return ii;
	    };

	    Repeat.prototype.__iterator = function(type, reverse) {var this$0 = this;
	      var ii = 0;
	      return new Iterator(function() 
	        {return ii < this$0.size ? iteratorValue(type, ii++, this$0._value) : iteratorDone()}
	      );
	    };

	    Repeat.prototype.equals = function(other) {
	      return other instanceof Repeat ?
	        is(this._value, other._value) :
	        deepEqual(other);
	    };


	  var EMPTY_REPEAT;

	  function invariant(condition, error) {
	    if (!condition) throw new Error(error);
	  }

	  createClass(Range, IndexedSeq);

	    function Range(start, end, step) {
	      if (!(this instanceof Range)) {
	        return new Range(start, end, step);
	      }
	      invariant(step !== 0, 'Cannot step a Range by 0');
	      start = start || 0;
	      if (end === undefined) {
	        end = Infinity;
	      }
	      step = step === undefined ? 1 : Math.abs(step);
	      if (end < start) {
	        step = -step;
	      }
	      this._start = start;
	      this._end = end;
	      this._step = step;
	      this.size = Math.max(0, Math.ceil((end - start) / step - 1) + 1);
	      if (this.size === 0) {
	        if (EMPTY_RANGE) {
	          return EMPTY_RANGE;
	        }
	        EMPTY_RANGE = this;
	      }
	    }

	    Range.prototype.toString = function() {
	      if (this.size === 0) {
	        return 'Range []';
	      }
	      return 'Range [ ' +
	        this._start + '...' + this._end +
	        (this._step !== 1 ? ' by ' + this._step : '') +
	      ' ]';
	    };

	    Range.prototype.get = function(index, notSetValue) {
	      return this.has(index) ?
	        this._start + wrapIndex(this, index) * this._step :
	        notSetValue;
	    };

	    Range.prototype.includes = function(searchValue) {
	      var possibleIndex = (searchValue - this._start) / this._step;
	      return possibleIndex >= 0 &&
	        possibleIndex < this.size &&
	        possibleIndex === Math.floor(possibleIndex);
	    };

	    Range.prototype.slice = function(begin, end) {
	      if (wholeSlice(begin, end, this.size)) {
	        return this;
	      }
	      begin = resolveBegin(begin, this.size);
	      end = resolveEnd(end, this.size);
	      if (end <= begin) {
	        return new Range(0, 0);
	      }
	      return new Range(this.get(begin, this._end), this.get(end, this._end), this._step);
	    };

	    Range.prototype.indexOf = function(searchValue) {
	      var offsetValue = searchValue - this._start;
	      if (offsetValue % this._step === 0) {
	        var index = offsetValue / this._step;
	        if (index >= 0 && index < this.size) {
	          return index
	        }
	      }
	      return -1;
	    };

	    Range.prototype.lastIndexOf = function(searchValue) {
	      return this.indexOf(searchValue);
	    };

	    Range.prototype.__iterate = function(fn, reverse) {
	      var maxIndex = this.size - 1;
	      var step = this._step;
	      var value = reverse ? this._start + maxIndex * step : this._start;
	      for (var ii = 0; ii <= maxIndex; ii++) {
	        if (fn(value, ii, this) === false) {
	          return ii + 1;
	        }
	        value += reverse ? -step : step;
	      }
	      return ii;
	    };

	    Range.prototype.__iterator = function(type, reverse) {
	      var maxIndex = this.size - 1;
	      var step = this._step;
	      var value = reverse ? this._start + maxIndex * step : this._start;
	      var ii = 0;
	      return new Iterator(function()  {
	        var v = value;
	        value += reverse ? -step : step;
	        return ii > maxIndex ? iteratorDone() : iteratorValue(type, ii++, v);
	      });
	    };

	    Range.prototype.equals = function(other) {
	      return other instanceof Range ?
	        this._start === other._start &&
	        this._end === other._end &&
	        this._step === other._step :
	        deepEqual(this, other);
	    };


	  var EMPTY_RANGE;

	  createClass(Collection, Iterable);
	    function Collection() {
	      throw TypeError('Abstract');
	    }


	  createClass(KeyedCollection, Collection);function KeyedCollection() {}

	  createClass(IndexedCollection, Collection);function IndexedCollection() {}

	  createClass(SetCollection, Collection);function SetCollection() {}


	  Collection.Keyed = KeyedCollection;
	  Collection.Indexed = IndexedCollection;
	  Collection.Set = SetCollection;

	  var imul =
	    typeof Math.imul === 'function' && Math.imul(0xffffffff, 2) === -2 ?
	    Math.imul :
	    function imul(a, b) {
	      a = a | 0; // int
	      b = b | 0; // int
	      var c = a & 0xffff;
	      var d = b & 0xffff;
	      // Shift by 0 fixes the sign on the high part.
	      return (c * d) + ((((a >>> 16) * d + c * (b >>> 16)) << 16) >>> 0) | 0; // int
	    };

	  // v8 has an optimization for storing 31-bit signed numbers.
	  // Values which have either 00 or 11 as the high order bits qualify.
	  // This function drops the highest order bit in a signed number, maintaining
	  // the sign bit.
	  function smi(i32) {
	    return ((i32 >>> 1) & 0x40000000) | (i32 & 0xBFFFFFFF);
	  }

	  function hash(o) {
	    if (o === false || o === null || o === undefined) {
	      return 0;
	    }
	    if (typeof o.valueOf === 'function') {
	      o = o.valueOf();
	      if (o === false || o === null || o === undefined) {
	        return 0;
	      }
	    }
	    if (o === true) {
	      return 1;
	    }
	    var type = typeof o;
	    if (type === 'number') {
	      if (o !== o || o === Infinity) {
	        return 0;
	      }
	      var h = o | 0;
	      if (h !== o) {
	        h ^= o * 0xFFFFFFFF;
	      }
	      while (o > 0xFFFFFFFF) {
	        o /= 0xFFFFFFFF;
	        h ^= o;
	      }
	      return smi(h);
	    }
	    if (type === 'string') {
	      return o.length > STRING_HASH_CACHE_MIN_STRLEN ? cachedHashString(o) : hashString(o);
	    }
	    if (typeof o.hashCode === 'function') {
	      return o.hashCode();
	    }
	    if (type === 'object') {
	      return hashJSObj(o);
	    }
	    if (typeof o.toString === 'function') {
	      return hashString(o.toString());
	    }
	    throw new Error('Value type ' + type + ' cannot be hashed.');
	  }

	  function cachedHashString(string) {
	    var hash = stringHashCache[string];
	    if (hash === undefined) {
	      hash = hashString(string);
	      if (STRING_HASH_CACHE_SIZE === STRING_HASH_CACHE_MAX_SIZE) {
	        STRING_HASH_CACHE_SIZE = 0;
	        stringHashCache = {};
	      }
	      STRING_HASH_CACHE_SIZE++;
	      stringHashCache[string] = hash;
	    }
	    return hash;
	  }

	  // http://jsperf.com/hashing-strings
	  function hashString(string) {
	    // This is the hash from JVM
	    // The hash code for a string is computed as
	    // s[0] * 31 ^ (n - 1) + s[1] * 31 ^ (n - 2) + ... + s[n - 1],
	    // where s[i] is the ith character of the string and n is the length of
	    // the string. We "mod" the result to make it between 0 (inclusive) and 2^31
	    // (exclusive) by dropping high bits.
	    var hash = 0;
	    for (var ii = 0; ii < string.length; ii++) {
	      hash = 31 * hash + string.charCodeAt(ii) | 0;
	    }
	    return smi(hash);
	  }

	  function hashJSObj(obj) {
	    var hash;
	    if (usingWeakMap) {
	      hash = weakMap.get(obj);
	      if (hash !== undefined) {
	        return hash;
	      }
	    }

	    hash = obj[UID_HASH_KEY];
	    if (hash !== undefined) {
	      return hash;
	    }

	    if (!canDefineProperty) {
	      hash = obj.propertyIsEnumerable && obj.propertyIsEnumerable[UID_HASH_KEY];
	      if (hash !== undefined) {
	        return hash;
	      }

	      hash = getIENodeHash(obj);
	      if (hash !== undefined) {
	        return hash;
	      }
	    }

	    hash = ++objHashUID;
	    if (objHashUID & 0x40000000) {
	      objHashUID = 0;
	    }

	    if (usingWeakMap) {
	      weakMap.set(obj, hash);
	    } else if (isExtensible !== undefined && isExtensible(obj) === false) {
	      throw new Error('Non-extensible objects are not allowed as keys.');
	    } else if (canDefineProperty) {
	      Object.defineProperty(obj, UID_HASH_KEY, {
	        'enumerable': false,
	        'configurable': false,
	        'writable': false,
	        'value': hash
	      });
	    } else if (obj.propertyIsEnumerable !== undefined &&
	               obj.propertyIsEnumerable === obj.constructor.prototype.propertyIsEnumerable) {
	      // Since we can't define a non-enumerable property on the object
	      // we'll hijack one of the less-used non-enumerable properties to
	      // save our hash on it. Since this is a function it will not show up in
	      // `JSON.stringify` which is what we want.
	      obj.propertyIsEnumerable = function() {
	        return this.constructor.prototype.propertyIsEnumerable.apply(this, arguments);
	      };
	      obj.propertyIsEnumerable[UID_HASH_KEY] = hash;
	    } else if (obj.nodeType !== undefined) {
	      // At this point we couldn't get the IE `uniqueID` to use as a hash
	      // and we couldn't use a non-enumerable property to exploit the
	      // dontEnum bug so we simply add the `UID_HASH_KEY` on the node
	      // itself.
	      obj[UID_HASH_KEY] = hash;
	    } else {
	      throw new Error('Unable to set a non-enumerable property on object.');
	    }

	    return hash;
	  }

	  // Get references to ES5 object methods.
	  var isExtensible = Object.isExtensible;

	  // True if Object.defineProperty works as expected. IE8 fails this test.
	  var canDefineProperty = (function() {
	    try {
	      Object.defineProperty({}, '@', {});
	      return true;
	    } catch (e) {
	      return false;
	    }
	  }());

	  // IE has a `uniqueID` property on DOM nodes. We can construct the hash from it
	  // and avoid memory leaks from the IE cloneNode bug.
	  function getIENodeHash(node) {
	    if (node && node.nodeType > 0) {
	      switch (node.nodeType) {
	        case 1: // Element
	          return node.uniqueID;
	        case 9: // Document
	          return node.documentElement && node.documentElement.uniqueID;
	      }
	    }
	  }

	  // If possible, use a WeakMap.
	  var usingWeakMap = typeof WeakMap === 'function';
	  var weakMap;
	  if (usingWeakMap) {
	    weakMap = new WeakMap();
	  }

	  var objHashUID = 0;

	  var UID_HASH_KEY = '__immutablehash__';
	  if (typeof Symbol === 'function') {
	    UID_HASH_KEY = Symbol(UID_HASH_KEY);
	  }

	  var STRING_HASH_CACHE_MIN_STRLEN = 16;
	  var STRING_HASH_CACHE_MAX_SIZE = 255;
	  var STRING_HASH_CACHE_SIZE = 0;
	  var stringHashCache = {};

	  function assertNotInfinite(size) {
	    invariant(
	      size !== Infinity,
	      'Cannot perform this action with an infinite size.'
	    );
	  }

	  createClass(Map, KeyedCollection);

	    // @pragma Construction

	    function Map(value) {
	      return value === null || value === undefined ? emptyMap() :
	        isMap(value) && !isOrdered(value) ? value :
	        emptyMap().withMutations(function(map ) {
	          var iter = KeyedIterable(value);
	          assertNotInfinite(iter.size);
	          iter.forEach(function(v, k)  {return map.set(k, v)});
	        });
	    }

	    Map.of = function() {var keyValues = SLICE$0.call(arguments, 0);
	      return emptyMap().withMutations(function(map ) {
	        for (var i = 0; i < keyValues.length; i += 2) {
	          if (i + 1 >= keyValues.length) {
	            throw new Error('Missing value for key: ' + keyValues[i]);
	          }
	          map.set(keyValues[i], keyValues[i + 1]);
	        }
	      });
	    };

	    Map.prototype.toString = function() {
	      return this.__toString('Map {', '}');
	    };

	    // @pragma Access

	    Map.prototype.get = function(k, notSetValue) {
	      return this._root ?
	        this._root.get(0, undefined, k, notSetValue) :
	        notSetValue;
	    };

	    // @pragma Modification

	    Map.prototype.set = function(k, v) {
	      return updateMap(this, k, v);
	    };

	    Map.prototype.setIn = function(keyPath, v) {
	      return this.updateIn(keyPath, NOT_SET, function()  {return v});
	    };

	    Map.prototype.remove = function(k) {
	      return updateMap(this, k, NOT_SET);
	    };

	    Map.prototype.deleteIn = function(keyPath) {
	      return this.updateIn(keyPath, function()  {return NOT_SET});
	    };

	    Map.prototype.update = function(k, notSetValue, updater) {
	      return arguments.length === 1 ?
	        k(this) :
	        this.updateIn([k], notSetValue, updater);
	    };

	    Map.prototype.updateIn = function(keyPath, notSetValue, updater) {
	      if (!updater) {
	        updater = notSetValue;
	        notSetValue = undefined;
	      }
	      var updatedValue = updateInDeepMap(
	        this,
	        forceIterator(keyPath),
	        notSetValue,
	        updater
	      );
	      return updatedValue === NOT_SET ? undefined : updatedValue;
	    };

	    Map.prototype.clear = function() {
	      if (this.size === 0) {
	        return this;
	      }
	      if (this.__ownerID) {
	        this.size = 0;
	        this._root = null;
	        this.__hash = undefined;
	        this.__altered = true;
	        return this;
	      }
	      return emptyMap();
	    };

	    // @pragma Composition

	    Map.prototype.merge = function(/*...iters*/) {
	      return mergeIntoMapWith(this, undefined, arguments);
	    };

	    Map.prototype.mergeWith = function(merger) {var iters = SLICE$0.call(arguments, 1);
	      return mergeIntoMapWith(this, merger, iters);
	    };

	    Map.prototype.mergeIn = function(keyPath) {var iters = SLICE$0.call(arguments, 1);
	      return this.updateIn(
	        keyPath,
	        emptyMap(),
	        function(m ) {return typeof m.merge === 'function' ?
	          m.merge.apply(m, iters) :
	          iters[iters.length - 1]}
	      );
	    };

	    Map.prototype.mergeDeep = function(/*...iters*/) {
	      return mergeIntoMapWith(this, deepMerger, arguments);
	    };

	    Map.prototype.mergeDeepWith = function(merger) {var iters = SLICE$0.call(arguments, 1);
	      return mergeIntoMapWith(this, deepMergerWith(merger), iters);
	    };

	    Map.prototype.mergeDeepIn = function(keyPath) {var iters = SLICE$0.call(arguments, 1);
	      return this.updateIn(
	        keyPath,
	        emptyMap(),
	        function(m ) {return typeof m.mergeDeep === 'function' ?
	          m.mergeDeep.apply(m, iters) :
	          iters[iters.length - 1]}
	      );
	    };

	    Map.prototype.sort = function(comparator) {
	      // Late binding
	      return OrderedMap(sortFactory(this, comparator));
	    };

	    Map.prototype.sortBy = function(mapper, comparator) {
	      // Late binding
	      return OrderedMap(sortFactory(this, comparator, mapper));
	    };

	    // @pragma Mutability

	    Map.prototype.withMutations = function(fn) {
	      var mutable = this.asMutable();
	      fn(mutable);
	      return mutable.wasAltered() ? mutable.__ensureOwner(this.__ownerID) : this;
	    };

	    Map.prototype.asMutable = function() {
	      return this.__ownerID ? this : this.__ensureOwner(new OwnerID());
	    };

	    Map.prototype.asImmutable = function() {
	      return this.__ensureOwner();
	    };

	    Map.prototype.wasAltered = function() {
	      return this.__altered;
	    };

	    Map.prototype.__iterator = function(type, reverse) {
	      return new MapIterator(this, type, reverse);
	    };

	    Map.prototype.__iterate = function(fn, reverse) {var this$0 = this;
	      var iterations = 0;
	      this._root && this._root.iterate(function(entry ) {
	        iterations++;
	        return fn(entry[1], entry[0], this$0);
	      }, reverse);
	      return iterations;
	    };

	    Map.prototype.__ensureOwner = function(ownerID) {
	      if (ownerID === this.__ownerID) {
	        return this;
	      }
	      if (!ownerID) {
	        this.__ownerID = ownerID;
	        this.__altered = false;
	        return this;
	      }
	      return makeMap(this.size, this._root, ownerID, this.__hash);
	    };


	  function isMap(maybeMap) {
	    return !!(maybeMap && maybeMap[IS_MAP_SENTINEL]);
	  }

	  Map.isMap = isMap;

	  var IS_MAP_SENTINEL = '@@__IMMUTABLE_MAP__@@';

	  var MapPrototype = Map.prototype;
	  MapPrototype[IS_MAP_SENTINEL] = true;
	  MapPrototype[DELETE] = MapPrototype.remove;
	  MapPrototype.removeIn = MapPrototype.deleteIn;


	  // #pragma Trie Nodes



	    function ArrayMapNode(ownerID, entries) {
	      this.ownerID = ownerID;
	      this.entries = entries;
	    }

	    ArrayMapNode.prototype.get = function(shift, keyHash, key, notSetValue) {
	      var entries = this.entries;
	      for (var ii = 0, len = entries.length; ii < len; ii++) {
	        if (is(key, entries[ii][0])) {
	          return entries[ii][1];
	        }
	      }
	      return notSetValue;
	    };

	    ArrayMapNode.prototype.update = function(ownerID, shift, keyHash, key, value, didChangeSize, didAlter) {
	      var removed = value === NOT_SET;

	      var entries = this.entries;
	      var idx = 0;
	      for (var len = entries.length; idx < len; idx++) {
	        if (is(key, entries[idx][0])) {
	          break;
	        }
	      }
	      var exists = idx < len;

	      if (exists ? entries[idx][1] === value : removed) {
	        return this;
	      }

	      SetRef(didAlter);
	      (removed || !exists) && SetRef(didChangeSize);

	      if (removed && entries.length === 1) {
	        return; // undefined
	      }

	      if (!exists && !removed && entries.length >= MAX_ARRAY_MAP_SIZE) {
	        return createNodes(ownerID, entries, key, value);
	      }

	      var isEditable = ownerID && ownerID === this.ownerID;
	      var newEntries = isEditable ? entries : arrCopy(entries);

	      if (exists) {
	        if (removed) {
	          idx === len - 1 ? newEntries.pop() : (newEntries[idx] = newEntries.pop());
	        } else {
	          newEntries[idx] = [key, value];
	        }
	      } else {
	        newEntries.push([key, value]);
	      }

	      if (isEditable) {
	        this.entries = newEntries;
	        return this;
	      }

	      return new ArrayMapNode(ownerID, newEntries);
	    };




	    function BitmapIndexedNode(ownerID, bitmap, nodes) {
	      this.ownerID = ownerID;
	      this.bitmap = bitmap;
	      this.nodes = nodes;
	    }

	    BitmapIndexedNode.prototype.get = function(shift, keyHash, key, notSetValue) {
	      if (keyHash === undefined) {
	        keyHash = hash(key);
	      }
	      var bit = (1 << ((shift === 0 ? keyHash : keyHash >>> shift) & MASK));
	      var bitmap = this.bitmap;
	      return (bitmap & bit) === 0 ? notSetValue :
	        this.nodes[popCount(bitmap & (bit - 1))].get(shift + SHIFT, keyHash, key, notSetValue);
	    };

	    BitmapIndexedNode.prototype.update = function(ownerID, shift, keyHash, key, value, didChangeSize, didAlter) {
	      if (keyHash === undefined) {
	        keyHash = hash(key);
	      }
	      var keyHashFrag = (shift === 0 ? keyHash : keyHash >>> shift) & MASK;
	      var bit = 1 << keyHashFrag;
	      var bitmap = this.bitmap;
	      var exists = (bitmap & bit) !== 0;

	      if (!exists && value === NOT_SET) {
	        return this;
	      }

	      var idx = popCount(bitmap & (bit - 1));
	      var nodes = this.nodes;
	      var node = exists ? nodes[idx] : undefined;
	      var newNode = updateNode(node, ownerID, shift + SHIFT, keyHash, key, value, didChangeSize, didAlter);

	      if (newNode === node) {
	        return this;
	      }

	      if (!exists && newNode && nodes.length >= MAX_BITMAP_INDEXED_SIZE) {
	        return expandNodes(ownerID, nodes, bitmap, keyHashFrag, newNode);
	      }

	      if (exists && !newNode && nodes.length === 2 && isLeafNode(nodes[idx ^ 1])) {
	        return nodes[idx ^ 1];
	      }

	      if (exists && newNode && nodes.length === 1 && isLeafNode(newNode)) {
	        return newNode;
	      }

	      var isEditable = ownerID && ownerID === this.ownerID;
	      var newBitmap = exists ? newNode ? bitmap : bitmap ^ bit : bitmap | bit;
	      var newNodes = exists ? newNode ?
	        setIn(nodes, idx, newNode, isEditable) :
	        spliceOut(nodes, idx, isEditable) :
	        spliceIn(nodes, idx, newNode, isEditable);

	      if (isEditable) {
	        this.bitmap = newBitmap;
	        this.nodes = newNodes;
	        return this;
	      }

	      return new BitmapIndexedNode(ownerID, newBitmap, newNodes);
	    };




	    function HashArrayMapNode(ownerID, count, nodes) {
	      this.ownerID = ownerID;
	      this.count = count;
	      this.nodes = nodes;
	    }

	    HashArrayMapNode.prototype.get = function(shift, keyHash, key, notSetValue) {
	      if (keyHash === undefined) {
	        keyHash = hash(key);
	      }
	      var idx = (shift === 0 ? keyHash : keyHash >>> shift) & MASK;
	      var node = this.nodes[idx];
	      return node ? node.get(shift + SHIFT, keyHash, key, notSetValue) : notSetValue;
	    };

	    HashArrayMapNode.prototype.update = function(ownerID, shift, keyHash, key, value, didChangeSize, didAlter) {
	      if (keyHash === undefined) {
	        keyHash = hash(key);
	      }
	      var idx = (shift === 0 ? keyHash : keyHash >>> shift) & MASK;
	      var removed = value === NOT_SET;
	      var nodes = this.nodes;
	      var node = nodes[idx];

	      if (removed && !node) {
	        return this;
	      }

	      var newNode = updateNode(node, ownerID, shift + SHIFT, keyHash, key, value, didChangeSize, didAlter);
	      if (newNode === node) {
	        return this;
	      }

	      var newCount = this.count;
	      if (!node) {
	        newCount++;
	      } else if (!newNode) {
	        newCount--;
	        if (newCount < MIN_HASH_ARRAY_MAP_SIZE) {
	          return packNodes(ownerID, nodes, newCount, idx);
	        }
	      }

	      var isEditable = ownerID && ownerID === this.ownerID;
	      var newNodes = setIn(nodes, idx, newNode, isEditable);

	      if (isEditable) {
	        this.count = newCount;
	        this.nodes = newNodes;
	        return this;
	      }

	      return new HashArrayMapNode(ownerID, newCount, newNodes);
	    };




	    function HashCollisionNode(ownerID, keyHash, entries) {
	      this.ownerID = ownerID;
	      this.keyHash = keyHash;
	      this.entries = entries;
	    }

	    HashCollisionNode.prototype.get = function(shift, keyHash, key, notSetValue) {
	      var entries = this.entries;
	      for (var ii = 0, len = entries.length; ii < len; ii++) {
	        if (is(key, entries[ii][0])) {
	          return entries[ii][1];
	        }
	      }
	      return notSetValue;
	    };

	    HashCollisionNode.prototype.update = function(ownerID, shift, keyHash, key, value, didChangeSize, didAlter) {
	      if (keyHash === undefined) {
	        keyHash = hash(key);
	      }

	      var removed = value === NOT_SET;

	      if (keyHash !== this.keyHash) {
	        if (removed) {
	          return this;
	        }
	        SetRef(didAlter);
	        SetRef(didChangeSize);
	        return mergeIntoNode(this, ownerID, shift, keyHash, [key, value]);
	      }

	      var entries = this.entries;
	      var idx = 0;
	      for (var len = entries.length; idx < len; idx++) {
	        if (is(key, entries[idx][0])) {
	          break;
	        }
	      }
	      var exists = idx < len;

	      if (exists ? entries[idx][1] === value : removed) {
	        return this;
	      }

	      SetRef(didAlter);
	      (removed || !exists) && SetRef(didChangeSize);

	      if (removed && len === 2) {
	        return new ValueNode(ownerID, this.keyHash, entries[idx ^ 1]);
	      }

	      var isEditable = ownerID && ownerID === this.ownerID;
	      var newEntries = isEditable ? entries : arrCopy(entries);

	      if (exists) {
	        if (removed) {
	          idx === len - 1 ? newEntries.pop() : (newEntries[idx] = newEntries.pop());
	        } else {
	          newEntries[idx] = [key, value];
	        }
	      } else {
	        newEntries.push([key, value]);
	      }

	      if (isEditable) {
	        this.entries = newEntries;
	        return this;
	      }

	      return new HashCollisionNode(ownerID, this.keyHash, newEntries);
	    };




	    function ValueNode(ownerID, keyHash, entry) {
	      this.ownerID = ownerID;
	      this.keyHash = keyHash;
	      this.entry = entry;
	    }

	    ValueNode.prototype.get = function(shift, keyHash, key, notSetValue) {
	      return is(key, this.entry[0]) ? this.entry[1] : notSetValue;
	    };

	    ValueNode.prototype.update = function(ownerID, shift, keyHash, key, value, didChangeSize, didAlter) {
	      var removed = value === NOT_SET;
	      var keyMatch = is(key, this.entry[0]);
	      if (keyMatch ? value === this.entry[1] : removed) {
	        return this;
	      }

	      SetRef(didAlter);

	      if (removed) {
	        SetRef(didChangeSize);
	        return; // undefined
	      }

	      if (keyMatch) {
	        if (ownerID && ownerID === this.ownerID) {
	          this.entry[1] = value;
	          return this;
	        }
	        return new ValueNode(ownerID, this.keyHash, [key, value]);
	      }

	      SetRef(didChangeSize);
	      return mergeIntoNode(this, ownerID, shift, hash(key), [key, value]);
	    };



	  // #pragma Iterators

	  ArrayMapNode.prototype.iterate =
	  HashCollisionNode.prototype.iterate = function (fn, reverse) {
	    var entries = this.entries;
	    for (var ii = 0, maxIndex = entries.length - 1; ii <= maxIndex; ii++) {
	      if (fn(entries[reverse ? maxIndex - ii : ii]) === false) {
	        return false;
	      }
	    }
	  }

	  BitmapIndexedNode.prototype.iterate =
	  HashArrayMapNode.prototype.iterate = function (fn, reverse) {
	    var nodes = this.nodes;
	    for (var ii = 0, maxIndex = nodes.length - 1; ii <= maxIndex; ii++) {
	      var node = nodes[reverse ? maxIndex - ii : ii];
	      if (node && node.iterate(fn, reverse) === false) {
	        return false;
	      }
	    }
	  }

	  ValueNode.prototype.iterate = function (fn, reverse) {
	    return fn(this.entry);
	  }

	  createClass(MapIterator, Iterator);

	    function MapIterator(map, type, reverse) {
	      this._type = type;
	      this._reverse = reverse;
	      this._stack = map._root && mapIteratorFrame(map._root);
	    }

	    MapIterator.prototype.next = function() {
	      var type = this._type;
	      var stack = this._stack;
	      while (stack) {
	        var node = stack.node;
	        var index = stack.index++;
	        var maxIndex;
	        if (node.entry) {
	          if (index === 0) {
	            return mapIteratorValue(type, node.entry);
	          }
	        } else if (node.entries) {
	          maxIndex = node.entries.length - 1;
	          if (index <= maxIndex) {
	            return mapIteratorValue(type, node.entries[this._reverse ? maxIndex - index : index]);
	          }
	        } else {
	          maxIndex = node.nodes.length - 1;
	          if (index <= maxIndex) {
	            var subNode = node.nodes[this._reverse ? maxIndex - index : index];
	            if (subNode) {
	              if (subNode.entry) {
	                return mapIteratorValue(type, subNode.entry);
	              }
	              stack = this._stack = mapIteratorFrame(subNode, stack);
	            }
	            continue;
	          }
	        }
	        stack = this._stack = this._stack.__prev;
	      }
	      return iteratorDone();
	    };


	  function mapIteratorValue(type, entry) {
	    return iteratorValue(type, entry[0], entry[1]);
	  }

	  function mapIteratorFrame(node, prev) {
	    return {
	      node: node,
	      index: 0,
	      __prev: prev
	    };
	  }

	  function makeMap(size, root, ownerID, hash) {
	    var map = Object.create(MapPrototype);
	    map.size = size;
	    map._root = root;
	    map.__ownerID = ownerID;
	    map.__hash = hash;
	    map.__altered = false;
	    return map;
	  }

	  var EMPTY_MAP;
	  function emptyMap() {
	    return EMPTY_MAP || (EMPTY_MAP = makeMap(0));
	  }

	  function updateMap(map, k, v) {
	    var newRoot;
	    var newSize;
	    if (!map._root) {
	      if (v === NOT_SET) {
	        return map;
	      }
	      newSize = 1;
	      newRoot = new ArrayMapNode(map.__ownerID, [[k, v]]);
	    } else {
	      var didChangeSize = MakeRef(CHANGE_LENGTH);
	      var didAlter = MakeRef(DID_ALTER);
	      newRoot = updateNode(map._root, map.__ownerID, 0, undefined, k, v, didChangeSize, didAlter);
	      if (!didAlter.value) {
	        return map;
	      }
	      newSize = map.size + (didChangeSize.value ? v === NOT_SET ? -1 : 1 : 0);
	    }
	    if (map.__ownerID) {
	      map.size = newSize;
	      map._root = newRoot;
	      map.__hash = undefined;
	      map.__altered = true;
	      return map;
	    }
	    return newRoot ? makeMap(newSize, newRoot) : emptyMap();
	  }

	  function updateNode(node, ownerID, shift, keyHash, key, value, didChangeSize, didAlter) {
	    if (!node) {
	      if (value === NOT_SET) {
	        return node;
	      }
	      SetRef(didAlter);
	      SetRef(didChangeSize);
	      return new ValueNode(ownerID, keyHash, [key, value]);
	    }
	    return node.update(ownerID, shift, keyHash, key, value, didChangeSize, didAlter);
	  }

	  function isLeafNode(node) {
	    return node.constructor === ValueNode || node.constructor === HashCollisionNode;
	  }

	  function mergeIntoNode(node, ownerID, shift, keyHash, entry) {
	    if (node.keyHash === keyHash) {
	      return new HashCollisionNode(ownerID, keyHash, [node.entry, entry]);
	    }

	    var idx1 = (shift === 0 ? node.keyHash : node.keyHash >>> shift) & MASK;
	    var idx2 = (shift === 0 ? keyHash : keyHash >>> shift) & MASK;

	    var newNode;
	    var nodes = idx1 === idx2 ?
	      [mergeIntoNode(node, ownerID, shift + SHIFT, keyHash, entry)] :
	      ((newNode = new ValueNode(ownerID, keyHash, entry)), idx1 < idx2 ? [node, newNode] : [newNode, node]);

	    return new BitmapIndexedNode(ownerID, (1 << idx1) | (1 << idx2), nodes);
	  }

	  function createNodes(ownerID, entries, key, value) {
	    if (!ownerID) {
	      ownerID = new OwnerID();
	    }
	    var node = new ValueNode(ownerID, hash(key), [key, value]);
	    for (var ii = 0; ii < entries.length; ii++) {
	      var entry = entries[ii];
	      node = node.update(ownerID, 0, undefined, entry[0], entry[1]);
	    }
	    return node;
	  }

	  function packNodes(ownerID, nodes, count, excluding) {
	    var bitmap = 0;
	    var packedII = 0;
	    var packedNodes = new Array(count);
	    for (var ii = 0, bit = 1, len = nodes.length; ii < len; ii++, bit <<= 1) {
	      var node = nodes[ii];
	      if (node !== undefined && ii !== excluding) {
	        bitmap |= bit;
	        packedNodes[packedII++] = node;
	      }
	    }
	    return new BitmapIndexedNode(ownerID, bitmap, packedNodes);
	  }

	  function expandNodes(ownerID, nodes, bitmap, including, node) {
	    var count = 0;
	    var expandedNodes = new Array(SIZE);
	    for (var ii = 0; bitmap !== 0; ii++, bitmap >>>= 1) {
	      expandedNodes[ii] = bitmap & 1 ? nodes[count++] : undefined;
	    }
	    expandedNodes[including] = node;
	    return new HashArrayMapNode(ownerID, count + 1, expandedNodes);
	  }

	  function mergeIntoMapWith(map, merger, iterables) {
	    var iters = [];
	    for (var ii = 0; ii < iterables.length; ii++) {
	      var value = iterables[ii];
	      var iter = KeyedIterable(value);
	      if (!isIterable(value)) {
	        iter = iter.map(function(v ) {return fromJS(v)});
	      }
	      iters.push(iter);
	    }
	    return mergeIntoCollectionWith(map, merger, iters);
	  }

	  function deepMerger(existing, value, key) {
	    return existing && existing.mergeDeep && isIterable(value) ?
	      existing.mergeDeep(value) :
	      is(existing, value) ? existing : value;
	  }

	  function deepMergerWith(merger) {
	    return function(existing, value, key)  {
	      if (existing && existing.mergeDeepWith && isIterable(value)) {
	        return existing.mergeDeepWith(merger, value);
	      }
	      var nextValue = merger(existing, value, key);
	      return is(existing, nextValue) ? existing : nextValue;
	    };
	  }

	  function mergeIntoCollectionWith(collection, merger, iters) {
	    iters = iters.filter(function(x ) {return x.size !== 0});
	    if (iters.length === 0) {
	      return collection;
	    }
	    if (collection.size === 0 && !collection.__ownerID && iters.length === 1) {
	      return collection.constructor(iters[0]);
	    }
	    return collection.withMutations(function(collection ) {
	      var mergeIntoMap = merger ?
	        function(value, key)  {
	          collection.update(key, NOT_SET, function(existing )
	            {return existing === NOT_SET ? value : merger(existing, value, key)}
	          );
	        } :
	        function(value, key)  {
	          collection.set(key, value);
	        }
	      for (var ii = 0; ii < iters.length; ii++) {
	        iters[ii].forEach(mergeIntoMap);
	      }
	    });
	  }

	  function updateInDeepMap(existing, keyPathIter, notSetValue, updater) {
	    var isNotSet = existing === NOT_SET;
	    var step = keyPathIter.next();
	    if (step.done) {
	      var existingValue = isNotSet ? notSetValue : existing;
	      var newValue = updater(existingValue);
	      return newValue === existingValue ? existing : newValue;
	    }
	    invariant(
	      isNotSet || (existing && existing.set),
	      'invalid keyPath'
	    );
	    var key = step.value;
	    var nextExisting = isNotSet ? NOT_SET : existing.get(key, NOT_SET);
	    var nextUpdated = updateInDeepMap(
	      nextExisting,
	      keyPathIter,
	      notSetValue,
	      updater
	    );
	    return nextUpdated === nextExisting ? existing :
	      nextUpdated === NOT_SET ? existing.remove(key) :
	      (isNotSet ? emptyMap() : existing).set(key, nextUpdated);
	  }

	  function popCount(x) {
	    x = x - ((x >> 1) & 0x55555555);
	    x = (x & 0x33333333) + ((x >> 2) & 0x33333333);
	    x = (x + (x >> 4)) & 0x0f0f0f0f;
	    x = x + (x >> 8);
	    x = x + (x >> 16);
	    return x & 0x7f;
	  }

	  function setIn(array, idx, val, canEdit) {
	    var newArray = canEdit ? array : arrCopy(array);
	    newArray[idx] = val;
	    return newArray;
	  }

	  function spliceIn(array, idx, val, canEdit) {
	    var newLen = array.length + 1;
	    if (canEdit && idx + 1 === newLen) {
	      array[idx] = val;
	      return array;
	    }
	    var newArray = new Array(newLen);
	    var after = 0;
	    for (var ii = 0; ii < newLen; ii++) {
	      if (ii === idx) {
	        newArray[ii] = val;
	        after = -1;
	      } else {
	        newArray[ii] = array[ii + after];
	      }
	    }
	    return newArray;
	  }

	  function spliceOut(array, idx, canEdit) {
	    var newLen = array.length - 1;
	    if (canEdit && idx === newLen) {
	      array.pop();
	      return array;
	    }
	    var newArray = new Array(newLen);
	    var after = 0;
	    for (var ii = 0; ii < newLen; ii++) {
	      if (ii === idx) {
	        after = 1;
	      }
	      newArray[ii] = array[ii + after];
	    }
	    return newArray;
	  }

	  var MAX_ARRAY_MAP_SIZE = SIZE / 4;
	  var MAX_BITMAP_INDEXED_SIZE = SIZE / 2;
	  var MIN_HASH_ARRAY_MAP_SIZE = SIZE / 4;

	  createClass(List, IndexedCollection);

	    // @pragma Construction

	    function List(value) {
	      var empty = emptyList();
	      if (value === null || value === undefined) {
	        return empty;
	      }
	      if (isList(value)) {
	        return value;
	      }
	      var iter = IndexedIterable(value);
	      var size = iter.size;
	      if (size === 0) {
	        return empty;
	      }
	      assertNotInfinite(size);
	      if (size > 0 && size < SIZE) {
	        return makeList(0, size, SHIFT, null, new VNode(iter.toArray()));
	      }
	      return empty.withMutations(function(list ) {
	        list.setSize(size);
	        iter.forEach(function(v, i)  {return list.set(i, v)});
	      });
	    }

	    List.of = function(/*...values*/) {
	      return this(arguments);
	    };

	    List.prototype.toString = function() {
	      return this.__toString('List [', ']');
	    };

	    // @pragma Access

	    List.prototype.get = function(index, notSetValue) {
	      index = wrapIndex(this, index);
	      if (index >= 0 && index < this.size) {
	        index += this._origin;
	        var node = listNodeFor(this, index);
	        return node && node.array[index & MASK];
	      }
	      return notSetValue;
	    };

	    // @pragma Modification

	    List.prototype.set = function(index, value) {
	      return updateList(this, index, value);
	    };

	    List.prototype.remove = function(index) {
	      return !this.has(index) ? this :
	        index === 0 ? this.shift() :
	        index === this.size - 1 ? this.pop() :
	        this.splice(index, 1);
	    };

	    List.prototype.insert = function(index, value) {
	      return this.splice(index, 0, value);
	    };

	    List.prototype.clear = function() {
	      if (this.size === 0) {
	        return this;
	      }
	      if (this.__ownerID) {
	        this.size = this._origin = this._capacity = 0;
	        this._level = SHIFT;
	        this._root = this._tail = null;
	        this.__hash = undefined;
	        this.__altered = true;
	        return this;
	      }
	      return emptyList();
	    };

	    List.prototype.push = function(/*...values*/) {
	      var values = arguments;
	      var oldSize = this.size;
	      return this.withMutations(function(list ) {
	        setListBounds(list, 0, oldSize + values.length);
	        for (var ii = 0; ii < values.length; ii++) {
	          list.set(oldSize + ii, values[ii]);
	        }
	      });
	    };

	    List.prototype.pop = function() {
	      return setListBounds(this, 0, -1);
	    };

	    List.prototype.unshift = function(/*...values*/) {
	      var values = arguments;
	      return this.withMutations(function(list ) {
	        setListBounds(list, -values.length);
	        for (var ii = 0; ii < values.length; ii++) {
	          list.set(ii, values[ii]);
	        }
	      });
	    };

	    List.prototype.shift = function() {
	      return setListBounds(this, 1);
	    };

	    // @pragma Composition

	    List.prototype.merge = function(/*...iters*/) {
	      return mergeIntoListWith(this, undefined, arguments);
	    };

	    List.prototype.mergeWith = function(merger) {var iters = SLICE$0.call(arguments, 1);
	      return mergeIntoListWith(this, merger, iters);
	    };

	    List.prototype.mergeDeep = function(/*...iters*/) {
	      return mergeIntoListWith(this, deepMerger, arguments);
	    };

	    List.prototype.mergeDeepWith = function(merger) {var iters = SLICE$0.call(arguments, 1);
	      return mergeIntoListWith(this, deepMergerWith(merger), iters);
	    };

	    List.prototype.setSize = function(size) {
	      return setListBounds(this, 0, size);
	    };

	    // @pragma Iteration

	    List.prototype.slice = function(begin, end) {
	      var size = this.size;
	      if (wholeSlice(begin, end, size)) {
	        return this;
	      }
	      return setListBounds(
	        this,
	        resolveBegin(begin, size),
	        resolveEnd(end, size)
	      );
	    };

	    List.prototype.__iterator = function(type, reverse) {
	      var index = 0;
	      var values = iterateList(this, reverse);
	      return new Iterator(function()  {
	        var value = values();
	        return value === DONE ?
	          iteratorDone() :
	          iteratorValue(type, index++, value);
	      });
	    };

	    List.prototype.__iterate = function(fn, reverse) {
	      var index = 0;
	      var values = iterateList(this, reverse);
	      var value;
	      while ((value = values()) !== DONE) {
	        if (fn(value, index++, this) === false) {
	          break;
	        }
	      }
	      return index;
	    };

	    List.prototype.__ensureOwner = function(ownerID) {
	      if (ownerID === this.__ownerID) {
	        return this;
	      }
	      if (!ownerID) {
	        this.__ownerID = ownerID;
	        return this;
	      }
	      return makeList(this._origin, this._capacity, this._level, this._root, this._tail, ownerID, this.__hash);
	    };


	  function isList(maybeList) {
	    return !!(maybeList && maybeList[IS_LIST_SENTINEL]);
	  }

	  List.isList = isList;

	  var IS_LIST_SENTINEL = '@@__IMMUTABLE_LIST__@@';

	  var ListPrototype = List.prototype;
	  ListPrototype[IS_LIST_SENTINEL] = true;
	  ListPrototype[DELETE] = ListPrototype.remove;
	  ListPrototype.setIn = MapPrototype.setIn;
	  ListPrototype.deleteIn =
	  ListPrototype.removeIn = MapPrototype.removeIn;
	  ListPrototype.update = MapPrototype.update;
	  ListPrototype.updateIn = MapPrototype.updateIn;
	  ListPrototype.mergeIn = MapPrototype.mergeIn;
	  ListPrototype.mergeDeepIn = MapPrototype.mergeDeepIn;
	  ListPrototype.withMutations = MapPrototype.withMutations;
	  ListPrototype.asMutable = MapPrototype.asMutable;
	  ListPrototype.asImmutable = MapPrototype.asImmutable;
	  ListPrototype.wasAltered = MapPrototype.wasAltered;



	    function VNode(array, ownerID) {
	      this.array = array;
	      this.ownerID = ownerID;
	    }

	    // TODO: seems like these methods are very similar

	    VNode.prototype.removeBefore = function(ownerID, level, index) {
	      if (index === level ? 1 << level : 0 || this.array.length === 0) {
	        return this;
	      }
	      var originIndex = (index >>> level) & MASK;
	      if (originIndex >= this.array.length) {
	        return new VNode([], ownerID);
	      }
	      var removingFirst = originIndex === 0;
	      var newChild;
	      if (level > 0) {
	        var oldChild = this.array[originIndex];
	        newChild = oldChild && oldChild.removeBefore(ownerID, level - SHIFT, index);
	        if (newChild === oldChild && removingFirst) {
	          return this;
	        }
	      }
	      if (removingFirst && !newChild) {
	        return this;
	      }
	      var editable = editableVNode(this, ownerID);
	      if (!removingFirst) {
	        for (var ii = 0; ii < originIndex; ii++) {
	          editable.array[ii] = undefined;
	        }
	      }
	      if (newChild) {
	        editable.array[originIndex] = newChild;
	      }
	      return editable;
	    };

	    VNode.prototype.removeAfter = function(ownerID, level, index) {
	      if (index === (level ? 1 << level : 0) || this.array.length === 0) {
	        return this;
	      }
	      var sizeIndex = ((index - 1) >>> level) & MASK;
	      if (sizeIndex >= this.array.length) {
	        return this;
	      }

	      var newChild;
	      if (level > 0) {
	        var oldChild = this.array[sizeIndex];
	        newChild = oldChild && oldChild.removeAfter(ownerID, level - SHIFT, index);
	        if (newChild === oldChild && sizeIndex === this.array.length - 1) {
	          return this;
	        }
	      }

	      var editable = editableVNode(this, ownerID);
	      editable.array.splice(sizeIndex + 1);
	      if (newChild) {
	        editable.array[sizeIndex] = newChild;
	      }
	      return editable;
	    };



	  var DONE = {};

	  function iterateList(list, reverse) {
	    var left = list._origin;
	    var right = list._capacity;
	    var tailPos = getTailOffset(right);
	    var tail = list._tail;

	    return iterateNodeOrLeaf(list._root, list._level, 0);

	    function iterateNodeOrLeaf(node, level, offset) {
	      return level === 0 ?
	        iterateLeaf(node, offset) :
	        iterateNode(node, level, offset);
	    }

	    function iterateLeaf(node, offset) {
	      var array = offset === tailPos ? tail && tail.array : node && node.array;
	      var from = offset > left ? 0 : left - offset;
	      var to = right - offset;
	      if (to > SIZE) {
	        to = SIZE;
	      }
	      return function()  {
	        if (from === to) {
	          return DONE;
	        }
	        var idx = reverse ? --to : from++;
	        return array && array[idx];
	      };
	    }

	    function iterateNode(node, level, offset) {
	      var values;
	      var array = node && node.array;
	      var from = offset > left ? 0 : (left - offset) >> level;
	      var to = ((right - offset) >> level) + 1;
	      if (to > SIZE) {
	        to = SIZE;
	      }
	      return function()  {
	        do {
	          if (values) {
	            var value = values();
	            if (value !== DONE) {
	              return value;
	            }
	            values = null;
	          }
	          if (from === to) {
	            return DONE;
	          }
	          var idx = reverse ? --to : from++;
	          values = iterateNodeOrLeaf(
	            array && array[idx], level - SHIFT, offset + (idx << level)
	          );
	        } while (true);
	      };
	    }
	  }

	  function makeList(origin, capacity, level, root, tail, ownerID, hash) {
	    var list = Object.create(ListPrototype);
	    list.size = capacity - origin;
	    list._origin = origin;
	    list._capacity = capacity;
	    list._level = level;
	    list._root = root;
	    list._tail = tail;
	    list.__ownerID = ownerID;
	    list.__hash = hash;
	    list.__altered = false;
	    return list;
	  }

	  var EMPTY_LIST;
	  function emptyList() {
	    return EMPTY_LIST || (EMPTY_LIST = makeList(0, 0, SHIFT));
	  }

	  function updateList(list, index, value) {
	    index = wrapIndex(list, index);

	    if (index !== index) {
	      return list;
	    }

	    if (index >= list.size || index < 0) {
	      return list.withMutations(function(list ) {
	        index < 0 ?
	          setListBounds(list, index).set(0, value) :
	          setListBounds(list, 0, index + 1).set(index, value)
	      });
	    }

	    index += list._origin;

	    var newTail = list._tail;
	    var newRoot = list._root;
	    var didAlter = MakeRef(DID_ALTER);
	    if (index >= getTailOffset(list._capacity)) {
	      newTail = updateVNode(newTail, list.__ownerID, 0, index, value, didAlter);
	    } else {
	      newRoot = updateVNode(newRoot, list.__ownerID, list._level, index, value, didAlter);
	    }

	    if (!didAlter.value) {
	      return list;
	    }

	    if (list.__ownerID) {
	      list._root = newRoot;
	      list._tail = newTail;
	      list.__hash = undefined;
	      list.__altered = true;
	      return list;
	    }
	    return makeList(list._origin, list._capacity, list._level, newRoot, newTail);
	  }

	  function updateVNode(node, ownerID, level, index, value, didAlter) {
	    var idx = (index >>> level) & MASK;
	    var nodeHas = node && idx < node.array.length;
	    if (!nodeHas && value === undefined) {
	      return node;
	    }

	    var newNode;

	    if (level > 0) {
	      var lowerNode = node && node.array[idx];
	      var newLowerNode = updateVNode(lowerNode, ownerID, level - SHIFT, index, value, didAlter);
	      if (newLowerNode === lowerNode) {
	        return node;
	      }
	      newNode = editableVNode(node, ownerID);
	      newNode.array[idx] = newLowerNode;
	      return newNode;
	    }

	    if (nodeHas && node.array[idx] === value) {
	      return node;
	    }

	    SetRef(didAlter);

	    newNode = editableVNode(node, ownerID);
	    if (value === undefined && idx === newNode.array.length - 1) {
	      newNode.array.pop();
	    } else {
	      newNode.array[idx] = value;
	    }
	    return newNode;
	  }

	  function editableVNode(node, ownerID) {
	    if (ownerID && node && ownerID === node.ownerID) {
	      return node;
	    }
	    return new VNode(node ? node.array.slice() : [], ownerID);
	  }

	  function listNodeFor(list, rawIndex) {
	    if (rawIndex >= getTailOffset(list._capacity)) {
	      return list._tail;
	    }
	    if (rawIndex < 1 << (list._level + SHIFT)) {
	      var node = list._root;
	      var level = list._level;
	      while (node && level > 0) {
	        node = node.array[(rawIndex >>> level) & MASK];
	        level -= SHIFT;
	      }
	      return node;
	    }
	  }

	  function setListBounds(list, begin, end) {
	    // Sanitize begin & end using this shorthand for ToInt32(argument)
	    // http://www.ecma-international.org/ecma-262/6.0/#sec-toint32
	    if (begin !== undefined) {
	      begin = begin | 0;
	    }
	    if (end !== undefined) {
	      end = end | 0;
	    }
	    var owner = list.__ownerID || new OwnerID();
	    var oldOrigin = list._origin;
	    var oldCapacity = list._capacity;
	    var newOrigin = oldOrigin + begin;
	    var newCapacity = end === undefined ? oldCapacity : end < 0 ? oldCapacity + end : oldOrigin + end;
	    if (newOrigin === oldOrigin && newCapacity === oldCapacity) {
	      return list;
	    }

	    // If it's going to end after it starts, it's empty.
	    if (newOrigin >= newCapacity) {
	      return list.clear();
	    }

	    var newLevel = list._level;
	    var newRoot = list._root;

	    // New origin might need creating a higher root.
	    var offsetShift = 0;
	    while (newOrigin + offsetShift < 0) {
	      newRoot = new VNode(newRoot && newRoot.array.length ? [undefined, newRoot] : [], owner);
	      newLevel += SHIFT;
	      offsetShift += 1 << newLevel;
	    }
	    if (offsetShift) {
	      newOrigin += offsetShift;
	      oldOrigin += offsetShift;
	      newCapacity += offsetShift;
	      oldCapacity += offsetShift;
	    }

	    var oldTailOffset = getTailOffset(oldCapacity);
	    var newTailOffset = getTailOffset(newCapacity);

	    // New size might need creating a higher root.
	    while (newTailOffset >= 1 << (newLevel + SHIFT)) {
	      newRoot = new VNode(newRoot && newRoot.array.length ? [newRoot] : [], owner);
	      newLevel += SHIFT;
	    }

	    // Locate or create the new tail.
	    var oldTail = list._tail;
	    var newTail = newTailOffset < oldTailOffset ?
	      listNodeFor(list, newCapacity - 1) :
	      newTailOffset > oldTailOffset ? new VNode([], owner) : oldTail;

	    // Merge Tail into tree.
	    if (oldTail && newTailOffset > oldTailOffset && newOrigin < oldCapacity && oldTail.array.length) {
	      newRoot = editableVNode(newRoot, owner);
	      var node = newRoot;
	      for (var level = newLevel; level > SHIFT; level -= SHIFT) {
	        var idx = (oldTailOffset >>> level) & MASK;
	        node = node.array[idx] = editableVNode(node.array[idx], owner);
	      }
	      node.array[(oldTailOffset >>> SHIFT) & MASK] = oldTail;
	    }

	    // If the size has been reduced, there's a chance the tail needs to be trimmed.
	    if (newCapacity < oldCapacity) {
	      newTail = newTail && newTail.removeAfter(owner, 0, newCapacity);
	    }

	    // If the new origin is within the tail, then we do not need a root.
	    if (newOrigin >= newTailOffset) {
	      newOrigin -= newTailOffset;
	      newCapacity -= newTailOffset;
	      newLevel = SHIFT;
	      newRoot = null;
	      newTail = newTail && newTail.removeBefore(owner, 0, newOrigin);

	    // Otherwise, if the root has been trimmed, garbage collect.
	    } else if (newOrigin > oldOrigin || newTailOffset < oldTailOffset) {
	      offsetShift = 0;

	      // Identify the new top root node of the subtree of the old root.
	      while (newRoot) {
	        var beginIndex = (newOrigin >>> newLevel) & MASK;
	        if (beginIndex !== (newTailOffset >>> newLevel) & MASK) {
	          break;
	        }
	        if (beginIndex) {
	          offsetShift += (1 << newLevel) * beginIndex;
	        }
	        newLevel -= SHIFT;
	        newRoot = newRoot.array[beginIndex];
	      }

	      // Trim the new sides of the new root.
	      if (newRoot && newOrigin > oldOrigin) {
	        newRoot = newRoot.removeBefore(owner, newLevel, newOrigin - offsetShift);
	      }
	      if (newRoot && newTailOffset < oldTailOffset) {
	        newRoot = newRoot.removeAfter(owner, newLevel, newTailOffset - offsetShift);
	      }
	      if (offsetShift) {
	        newOrigin -= offsetShift;
	        newCapacity -= offsetShift;
	      }
	    }

	    if (list.__ownerID) {
	      list.size = newCapacity - newOrigin;
	      list._origin = newOrigin;
	      list._capacity = newCapacity;
	      list._level = newLevel;
	      list._root = newRoot;
	      list._tail = newTail;
	      list.__hash = undefined;
	      list.__altered = true;
	      return list;
	    }
	    return makeList(newOrigin, newCapacity, newLevel, newRoot, newTail);
	  }

	  function mergeIntoListWith(list, merger, iterables) {
	    var iters = [];
	    var maxSize = 0;
	    for (var ii = 0; ii < iterables.length; ii++) {
	      var value = iterables[ii];
	      var iter = IndexedIterable(value);
	      if (iter.size > maxSize) {
	        maxSize = iter.size;
	      }
	      if (!isIterable(value)) {
	        iter = iter.map(function(v ) {return fromJS(v)});
	      }
	      iters.push(iter);
	    }
	    if (maxSize > list.size) {
	      list = list.setSize(maxSize);
	    }
	    return mergeIntoCollectionWith(list, merger, iters);
	  }

	  function getTailOffset(size) {
	    return size < SIZE ? 0 : (((size - 1) >>> SHIFT) << SHIFT);
	  }

	  createClass(OrderedMap, Map);

	    // @pragma Construction

	    function OrderedMap(value) {
	      return value === null || value === undefined ? emptyOrderedMap() :
	        isOrderedMap(value) ? value :
	        emptyOrderedMap().withMutations(function(map ) {
	          var iter = KeyedIterable(value);
	          assertNotInfinite(iter.size);
	          iter.forEach(function(v, k)  {return map.set(k, v)});
	        });
	    }

	    OrderedMap.of = function(/*...values*/) {
	      return this(arguments);
	    };

	    OrderedMap.prototype.toString = function() {
	      return this.__toString('OrderedMap {', '}');
	    };

	    // @pragma Access

	    OrderedMap.prototype.get = function(k, notSetValue) {
	      var index = this._map.get(k);
	      return index !== undefined ? this._list.get(index)[1] : notSetValue;
	    };

	    // @pragma Modification

	    OrderedMap.prototype.clear = function() {
	      if (this.size === 0) {
	        return this;
	      }
	      if (this.__ownerID) {
	        this.size = 0;
	        this._map.clear();
	        this._list.clear();
	        return this;
	      }
	      return emptyOrderedMap();
	    };

	    OrderedMap.prototype.set = function(k, v) {
	      return updateOrderedMap(this, k, v);
	    };

	    OrderedMap.prototype.remove = function(k) {
	      return updateOrderedMap(this, k, NOT_SET);
	    };

	    OrderedMap.prototype.wasAltered = function() {
	      return this._map.wasAltered() || this._list.wasAltered();
	    };

	    OrderedMap.prototype.__iterate = function(fn, reverse) {var this$0 = this;
	      return this._list.__iterate(
	        function(entry ) {return entry && fn(entry[1], entry[0], this$0)},
	        reverse
	      );
	    };

	    OrderedMap.prototype.__iterator = function(type, reverse) {
	      return this._list.fromEntrySeq().__iterator(type, reverse);
	    };

	    OrderedMap.prototype.__ensureOwner = function(ownerID) {
	      if (ownerID === this.__ownerID) {
	        return this;
	      }
	      var newMap = this._map.__ensureOwner(ownerID);
	      var newList = this._list.__ensureOwner(ownerID);
	      if (!ownerID) {
	        this.__ownerID = ownerID;
	        this._map = newMap;
	        this._list = newList;
	        return this;
	      }
	      return makeOrderedMap(newMap, newList, ownerID, this.__hash);
	    };


	  function isOrderedMap(maybeOrderedMap) {
	    return isMap(maybeOrderedMap) && isOrdered(maybeOrderedMap);
	  }

	  OrderedMap.isOrderedMap = isOrderedMap;

	  OrderedMap.prototype[IS_ORDERED_SENTINEL] = true;
	  OrderedMap.prototype[DELETE] = OrderedMap.prototype.remove;



	  function makeOrderedMap(map, list, ownerID, hash) {
	    var omap = Object.create(OrderedMap.prototype);
	    omap.size = map ? map.size : 0;
	    omap._map = map;
	    omap._list = list;
	    omap.__ownerID = ownerID;
	    omap.__hash = hash;
	    return omap;
	  }

	  var EMPTY_ORDERED_MAP;
	  function emptyOrderedMap() {
	    return EMPTY_ORDERED_MAP || (EMPTY_ORDERED_MAP = makeOrderedMap(emptyMap(), emptyList()));
	  }

	  function updateOrderedMap(omap, k, v) {
	    var map = omap._map;
	    var list = omap._list;
	    var i = map.get(k);
	    var has = i !== undefined;
	    var newMap;
	    var newList;
	    if (v === NOT_SET) { // removed
	      if (!has) {
	        return omap;
	      }
	      if (list.size >= SIZE && list.size >= map.size * 2) {
	        newList = list.filter(function(entry, idx)  {return entry !== undefined && i !== idx});
	        newMap = newList.toKeyedSeq().map(function(entry ) {return entry[0]}).flip().toMap();
	        if (omap.__ownerID) {
	          newMap.__ownerID = newList.__ownerID = omap.__ownerID;
	        }
	      } else {
	        newMap = map.remove(k);
	        newList = i === list.size - 1 ? list.pop() : list.set(i, undefined);
	      }
	    } else {
	      if (has) {
	        if (v === list.get(i)[1]) {
	          return omap;
	        }
	        newMap = map;
	        newList = list.set(i, [k, v]);
	      } else {
	        newMap = map.set(k, list.size);
	        newList = list.set(list.size, [k, v]);
	      }
	    }
	    if (omap.__ownerID) {
	      omap.size = newMap.size;
	      omap._map = newMap;
	      omap._list = newList;
	      omap.__hash = undefined;
	      return omap;
	    }
	    return makeOrderedMap(newMap, newList);
	  }

	  createClass(ToKeyedSequence, KeyedSeq);
	    function ToKeyedSequence(indexed, useKeys) {
	      this._iter = indexed;
	      this._useKeys = useKeys;
	      this.size = indexed.size;
	    }

	    ToKeyedSequence.prototype.get = function(key, notSetValue) {
	      return this._iter.get(key, notSetValue);
	    };

	    ToKeyedSequence.prototype.has = function(key) {
	      return this._iter.has(key);
	    };

	    ToKeyedSequence.prototype.valueSeq = function() {
	      return this._iter.valueSeq();
	    };

	    ToKeyedSequence.prototype.reverse = function() {var this$0 = this;
	      var reversedSequence = reverseFactory(this, true);
	      if (!this._useKeys) {
	        reversedSequence.valueSeq = function()  {return this$0._iter.toSeq().reverse()};
	      }
	      return reversedSequence;
	    };

	    ToKeyedSequence.prototype.map = function(mapper, context) {var this$0 = this;
	      var mappedSequence = mapFactory(this, mapper, context);
	      if (!this._useKeys) {
	        mappedSequence.valueSeq = function()  {return this$0._iter.toSeq().map(mapper, context)};
	      }
	      return mappedSequence;
	    };

	    ToKeyedSequence.prototype.__iterate = function(fn, reverse) {var this$0 = this;
	      var ii;
	      return this._iter.__iterate(
	        this._useKeys ?
	          function(v, k)  {return fn(v, k, this$0)} :
	          ((ii = reverse ? resolveSize(this) : 0),
	            function(v ) {return fn(v, reverse ? --ii : ii++, this$0)}),
	        reverse
	      );
	    };

	    ToKeyedSequence.prototype.__iterator = function(type, reverse) {
	      if (this._useKeys) {
	        return this._iter.__iterator(type, reverse);
	      }
	      var iterator = this._iter.__iterator(ITERATE_VALUES, reverse);
	      var ii = reverse ? resolveSize(this) : 0;
	      return new Iterator(function()  {
	        var step = iterator.next();
	        return step.done ? step :
	          iteratorValue(type, reverse ? --ii : ii++, step.value, step);
	      });
	    };

	  ToKeyedSequence.prototype[IS_ORDERED_SENTINEL] = true;


	  createClass(ToIndexedSequence, IndexedSeq);
	    function ToIndexedSequence(iter) {
	      this._iter = iter;
	      this.size = iter.size;
	    }

	    ToIndexedSequence.prototype.includes = function(value) {
	      return this._iter.includes(value);
	    };

	    ToIndexedSequence.prototype.__iterate = function(fn, reverse) {var this$0 = this;
	      var iterations = 0;
	      return this._iter.__iterate(function(v ) {return fn(v, iterations++, this$0)}, reverse);
	    };

	    ToIndexedSequence.prototype.__iterator = function(type, reverse) {
	      var iterator = this._iter.__iterator(ITERATE_VALUES, reverse);
	      var iterations = 0;
	      return new Iterator(function()  {
	        var step = iterator.next();
	        return step.done ? step :
	          iteratorValue(type, iterations++, step.value, step)
	      });
	    };



	  createClass(ToSetSequence, SetSeq);
	    function ToSetSequence(iter) {
	      this._iter = iter;
	      this.size = iter.size;
	    }

	    ToSetSequence.prototype.has = function(key) {
	      return this._iter.includes(key);
	    };

	    ToSetSequence.prototype.__iterate = function(fn, reverse) {var this$0 = this;
	      return this._iter.__iterate(function(v ) {return fn(v, v, this$0)}, reverse);
	    };

	    ToSetSequence.prototype.__iterator = function(type, reverse) {
	      var iterator = this._iter.__iterator(ITERATE_VALUES, reverse);
	      return new Iterator(function()  {
	        var step = iterator.next();
	        return step.done ? step :
	          iteratorValue(type, step.value, step.value, step);
	      });
	    };



	  createClass(FromEntriesSequence, KeyedSeq);
	    function FromEntriesSequence(entries) {
	      this._iter = entries;
	      this.size = entries.size;
	    }

	    FromEntriesSequence.prototype.entrySeq = function() {
	      return this._iter.toSeq();
	    };

	    FromEntriesSequence.prototype.__iterate = function(fn, reverse) {var this$0 = this;
	      return this._iter.__iterate(function(entry ) {
	        // Check if entry exists first so array access doesn't throw for holes
	        // in the parent iteration.
	        if (entry) {
	          validateEntry(entry);
	          var indexedIterable = isIterable(entry);
	          return fn(
	            indexedIterable ? entry.get(1) : entry[1],
	            indexedIterable ? entry.get(0) : entry[0],
	            this$0
	          );
	        }
	      }, reverse);
	    };

	    FromEntriesSequence.prototype.__iterator = function(type, reverse) {
	      var iterator = this._iter.__iterator(ITERATE_VALUES, reverse);
	      return new Iterator(function()  {
	        while (true) {
	          var step = iterator.next();
	          if (step.done) {
	            return step;
	          }
	          var entry = step.value;
	          // Check if entry exists first so array access doesn't throw for holes
	          // in the parent iteration.
	          if (entry) {
	            validateEntry(entry);
	            var indexedIterable = isIterable(entry);
	            return iteratorValue(
	              type,
	              indexedIterable ? entry.get(0) : entry[0],
	              indexedIterable ? entry.get(1) : entry[1],
	              step
	            );
	          }
	        }
	      });
	    };


	  ToIndexedSequence.prototype.cacheResult =
	  ToKeyedSequence.prototype.cacheResult =
	  ToSetSequence.prototype.cacheResult =
	  FromEntriesSequence.prototype.cacheResult =
	    cacheResultThrough;


	  function flipFactory(iterable) {
	    var flipSequence = makeSequence(iterable);
	    flipSequence._iter = iterable;
	    flipSequence.size = iterable.size;
	    flipSequence.flip = function()  {return iterable};
	    flipSequence.reverse = function () {
	      var reversedSequence = iterable.reverse.apply(this); // super.reverse()
	      reversedSequence.flip = function()  {return iterable.reverse()};
	      return reversedSequence;
	    };
	    flipSequence.has = function(key ) {return iterable.includes(key)};
	    flipSequence.includes = function(key ) {return iterable.has(key)};
	    flipSequence.cacheResult = cacheResultThrough;
	    flipSequence.__iterateUncached = function (fn, reverse) {var this$0 = this;
	      return iterable.__iterate(function(v, k)  {return fn(k, v, this$0) !== false}, reverse);
	    }
	    flipSequence.__iteratorUncached = function(type, reverse) {
	      if (type === ITERATE_ENTRIES) {
	        var iterator = iterable.__iterator(type, reverse);
	        return new Iterator(function()  {
	          var step = iterator.next();
	          if (!step.done) {
	            var k = step.value[0];
	            step.value[0] = step.value[1];
	            step.value[1] = k;
	          }
	          return step;
	        });
	      }
	      return iterable.__iterator(
	        type === ITERATE_VALUES ? ITERATE_KEYS : ITERATE_VALUES,
	        reverse
	      );
	    }
	    return flipSequence;
	  }


	  function mapFactory(iterable, mapper, context) {
	    var mappedSequence = makeSequence(iterable);
	    mappedSequence.size = iterable.size;
	    mappedSequence.has = function(key ) {return iterable.has(key)};
	    mappedSequence.get = function(key, notSetValue)  {
	      var v = iterable.get(key, NOT_SET);
	      return v === NOT_SET ?
	        notSetValue :
	        mapper.call(context, v, key, iterable);
	    };
	    mappedSequence.__iterateUncached = function (fn, reverse) {var this$0 = this;
	      return iterable.__iterate(
	        function(v, k, c)  {return fn(mapper.call(context, v, k, c), k, this$0) !== false},
	        reverse
	      );
	    }
	    mappedSequence.__iteratorUncached = function (type, reverse) {
	      var iterator = iterable.__iterator(ITERATE_ENTRIES, reverse);
	      return new Iterator(function()  {
	        var step = iterator.next();
	        if (step.done) {
	          return step;
	        }
	        var entry = step.value;
	        var key = entry[0];
	        return iteratorValue(
	          type,
	          key,
	          mapper.call(context, entry[1], key, iterable),
	          step
	        );
	      });
	    }
	    return mappedSequence;
	  }


	  function reverseFactory(iterable, useKeys) {
	    var reversedSequence = makeSequence(iterable);
	    reversedSequence._iter = iterable;
	    reversedSequence.size = iterable.size;
	    reversedSequence.reverse = function()  {return iterable};
	    if (iterable.flip) {
	      reversedSequence.flip = function () {
	        var flipSequence = flipFactory(iterable);
	        flipSequence.reverse = function()  {return iterable.flip()};
	        return flipSequence;
	      };
	    }
	    reversedSequence.get = function(key, notSetValue) 
	      {return iterable.get(useKeys ? key : -1 - key, notSetValue)};
	    reversedSequence.has = function(key )
	      {return iterable.has(useKeys ? key : -1 - key)};
	    reversedSequence.includes = function(value ) {return iterable.includes(value)};
	    reversedSequence.cacheResult = cacheResultThrough;
	    reversedSequence.__iterate = function (fn, reverse) {var this$0 = this;
	      return iterable.__iterate(function(v, k)  {return fn(v, k, this$0)}, !reverse);
	    };
	    reversedSequence.__iterator =
	      function(type, reverse)  {return iterable.__iterator(type, !reverse)};
	    return reversedSequence;
	  }


	  function filterFactory(iterable, predicate, context, useKeys) {
	    var filterSequence = makeSequence(iterable);
	    if (useKeys) {
	      filterSequence.has = function(key ) {
	        var v = iterable.get(key, NOT_SET);
	        return v !== NOT_SET && !!predicate.call(context, v, key, iterable);
	      };
	      filterSequence.get = function(key, notSetValue)  {
	        var v = iterable.get(key, NOT_SET);
	        return v !== NOT_SET && predicate.call(context, v, key, iterable) ?
	          v : notSetValue;
	      };
	    }
	    filterSequence.__iterateUncached = function (fn, reverse) {var this$0 = this;
	      var iterations = 0;
	      iterable.__iterate(function(v, k, c)  {
	        if (predicate.call(context, v, k, c)) {
	          iterations++;
	          return fn(v, useKeys ? k : iterations - 1, this$0);
	        }
	      }, reverse);
	      return iterations;
	    };
	    filterSequence.__iteratorUncached = function (type, reverse) {
	      var iterator = iterable.__iterator(ITERATE_ENTRIES, reverse);
	      var iterations = 0;
	      return new Iterator(function()  {
	        while (true) {
	          var step = iterator.next();
	          if (step.done) {
	            return step;
	          }
	          var entry = step.value;
	          var key = entry[0];
	          var value = entry[1];
	          if (predicate.call(context, value, key, iterable)) {
	            return iteratorValue(type, useKeys ? key : iterations++, value, step);
	          }
	        }
	      });
	    }
	    return filterSequence;
	  }


	  function countByFactory(iterable, grouper, context) {
	    var groups = Map().asMutable();
	    iterable.__iterate(function(v, k)  {
	      groups.update(
	        grouper.call(context, v, k, iterable),
	        0,
	        function(a ) {return a + 1}
	      );
	    });
	    return groups.asImmutable();
	  }


	  function groupByFactory(iterable, grouper, context) {
	    var isKeyedIter = isKeyed(iterable);
	    var groups = (isOrdered(iterable) ? OrderedMap() : Map()).asMutable();
	    iterable.__iterate(function(v, k)  {
	      groups.update(
	        grouper.call(context, v, k, iterable),
	        function(a ) {return (a = a || [], a.push(isKeyedIter ? [k, v] : v), a)}
	      );
	    });
	    var coerce = iterableClass(iterable);
	    return groups.map(function(arr ) {return reify(iterable, coerce(arr))});
	  }


	  function sliceFactory(iterable, begin, end, useKeys) {
	    var originalSize = iterable.size;

	    // Sanitize begin & end using this shorthand for ToInt32(argument)
	    // http://www.ecma-international.org/ecma-262/6.0/#sec-toint32
	    if (begin !== undefined) {
	      begin = begin | 0;
	    }
	    if (end !== undefined) {
	      if (end === Infinity) {
	        end = originalSize;
	      } else {
	        end = end | 0;
	      }
	    }

	    if (wholeSlice(begin, end, originalSize)) {
	      return iterable;
	    }

	    var resolvedBegin = resolveBegin(begin, originalSize);
	    var resolvedEnd = resolveEnd(end, originalSize);

	    // begin or end will be NaN if they were provided as negative numbers and
	    // this iterable's size is unknown. In that case, cache first so there is
	    // a known size and these do not resolve to NaN.
	    if (resolvedBegin !== resolvedBegin || resolvedEnd !== resolvedEnd) {
	      return sliceFactory(iterable.toSeq().cacheResult(), begin, end, useKeys);
	    }

	    // Note: resolvedEnd is undefined when the original sequence's length is
	    // unknown and this slice did not supply an end and should contain all
	    // elements after resolvedBegin.
	    // In that case, resolvedSize will be NaN and sliceSize will remain undefined.
	    var resolvedSize = resolvedEnd - resolvedBegin;
	    var sliceSize;
	    if (resolvedSize === resolvedSize) {
	      sliceSize = resolvedSize < 0 ? 0 : resolvedSize;
	    }

	    var sliceSeq = makeSequence(iterable);

	    // If iterable.size is undefined, the size of the realized sliceSeq is
	    // unknown at this point unless the number of items to slice is 0
	    sliceSeq.size = sliceSize === 0 ? sliceSize : iterable.size && sliceSize || undefined;

	    if (!useKeys && isSeq(iterable) && sliceSize >= 0) {
	      sliceSeq.get = function (index, notSetValue) {
	        index = wrapIndex(this, index);
	        return index >= 0 && index < sliceSize ?
	          iterable.get(index + resolvedBegin, notSetValue) :
	          notSetValue;
	      }
	    }

	    sliceSeq.__iterateUncached = function(fn, reverse) {var this$0 = this;
	      if (sliceSize === 0) {
	        return 0;
	      }
	      if (reverse) {
	        return this.cacheResult().__iterate(fn, reverse);
	      }
	      var skipped = 0;
	      var isSkipping = true;
	      var iterations = 0;
	      iterable.__iterate(function(v, k)  {
	        if (!(isSkipping && (isSkipping = skipped++ < resolvedBegin))) {
	          iterations++;
	          return fn(v, useKeys ? k : iterations - 1, this$0) !== false &&
	                 iterations !== sliceSize;
	        }
	      });
	      return iterations;
	    };

	    sliceSeq.__iteratorUncached = function(type, reverse) {
	      if (sliceSize !== 0 && reverse) {
	        return this.cacheResult().__iterator(type, reverse);
	      }
	      // Don't bother instantiating parent iterator if taking 0.
	      var iterator = sliceSize !== 0 && iterable.__iterator(type, reverse);
	      var skipped = 0;
	      var iterations = 0;
	      return new Iterator(function()  {
	        while (skipped++ < resolvedBegin) {
	          iterator.next();
	        }
	        if (++iterations > sliceSize) {
	          return iteratorDone();
	        }
	        var step = iterator.next();
	        if (useKeys || type === ITERATE_VALUES) {
	          return step;
	        } else if (type === ITERATE_KEYS) {
	          return iteratorValue(type, iterations - 1, undefined, step);
	        } else {
	          return iteratorValue(type, iterations - 1, step.value[1], step);
	        }
	      });
	    }

	    return sliceSeq;
	  }


	  function takeWhileFactory(iterable, predicate, context) {
	    var takeSequence = makeSequence(iterable);
	    takeSequence.__iterateUncached = function(fn, reverse) {var this$0 = this;
	      if (reverse) {
	        return this.cacheResult().__iterate(fn, reverse);
	      }
	      var iterations = 0;
	      iterable.__iterate(function(v, k, c) 
	        {return predicate.call(context, v, k, c) && ++iterations && fn(v, k, this$0)}
	      );
	      return iterations;
	    };
	    takeSequence.__iteratorUncached = function(type, reverse) {var this$0 = this;
	      if (reverse) {
	        return this.cacheResult().__iterator(type, reverse);
	      }
	      var iterator = iterable.__iterator(ITERATE_ENTRIES, reverse);
	      var iterating = true;
	      return new Iterator(function()  {
	        if (!iterating) {
	          return iteratorDone();
	        }
	        var step = iterator.next();
	        if (step.done) {
	          return step;
	        }
	        var entry = step.value;
	        var k = entry[0];
	        var v = entry[1];
	        if (!predicate.call(context, v, k, this$0)) {
	          iterating = false;
	          return iteratorDone();
	        }
	        return type === ITERATE_ENTRIES ? step :
	          iteratorValue(type, k, v, step);
	      });
	    };
	    return takeSequence;
	  }


	  function skipWhileFactory(iterable, predicate, context, useKeys) {
	    var skipSequence = makeSequence(iterable);
	    skipSequence.__iterateUncached = function (fn, reverse) {var this$0 = this;
	      if (reverse) {
	        return this.cacheResult().__iterate(fn, reverse);
	      }
	      var isSkipping = true;
	      var iterations = 0;
	      iterable.__iterate(function(v, k, c)  {
	        if (!(isSkipping && (isSkipping = predicate.call(context, v, k, c)))) {
	          iterations++;
	          return fn(v, useKeys ? k : iterations - 1, this$0);
	        }
	      });
	      return iterations;
	    };
	    skipSequence.__iteratorUncached = function(type, reverse) {var this$0 = this;
	      if (reverse) {
	        return this.cacheResult().__iterator(type, reverse);
	      }
	      var iterator = iterable.__iterator(ITERATE_ENTRIES, reverse);
	      var skipping = true;
	      var iterations = 0;
	      return new Iterator(function()  {
	        var step, k, v;
	        do {
	          step = iterator.next();
	          if (step.done) {
	            if (useKeys || type === ITERATE_VALUES) {
	              return step;
	            } else if (type === ITERATE_KEYS) {
	              return iteratorValue(type, iterations++, undefined, step);
	            } else {
	              return iteratorValue(type, iterations++, step.value[1], step);
	            }
	          }
	          var entry = step.value;
	          k = entry[0];
	          v = entry[1];
	          skipping && (skipping = predicate.call(context, v, k, this$0));
	        } while (skipping);
	        return type === ITERATE_ENTRIES ? step :
	          iteratorValue(type, k, v, step);
	      });
	    };
	    return skipSequence;
	  }


	  function concatFactory(iterable, values) {
	    var isKeyedIterable = isKeyed(iterable);
	    var iters = [iterable].concat(values).map(function(v ) {
	      if (!isIterable(v)) {
	        v = isKeyedIterable ?
	          keyedSeqFromValue(v) :
	          indexedSeqFromValue(Array.isArray(v) ? v : [v]);
	      } else if (isKeyedIterable) {
	        v = KeyedIterable(v);
	      }
	      return v;
	    }).filter(function(v ) {return v.size !== 0});

	    if (iters.length === 0) {
	      return iterable;
	    }

	    if (iters.length === 1) {
	      var singleton = iters[0];
	      if (singleton === iterable ||
	          isKeyedIterable && isKeyed(singleton) ||
	          isIndexed(iterable) && isIndexed(singleton)) {
	        return singleton;
	      }
	    }

	    var concatSeq = new ArraySeq(iters);
	    if (isKeyedIterable) {
	      concatSeq = concatSeq.toKeyedSeq();
	    } else if (!isIndexed(iterable)) {
	      concatSeq = concatSeq.toSetSeq();
	    }
	    concatSeq = concatSeq.flatten(true);
	    concatSeq.size = iters.reduce(
	      function(sum, seq)  {
	        if (sum !== undefined) {
	          var size = seq.size;
	          if (size !== undefined) {
	            return sum + size;
	          }
	        }
	      },
	      0
	    );
	    return concatSeq;
	  }


	  function flattenFactory(iterable, depth, useKeys) {
	    var flatSequence = makeSequence(iterable);
	    flatSequence.__iterateUncached = function(fn, reverse) {
	      var iterations = 0;
	      var stopped = false;
	      function flatDeep(iter, currentDepth) {var this$0 = this;
	        iter.__iterate(function(v, k)  {
	          if ((!depth || currentDepth < depth) && isIterable(v)) {
	            flatDeep(v, currentDepth + 1);
	          } else if (fn(v, useKeys ? k : iterations++, this$0) === false) {
	            stopped = true;
	          }
	          return !stopped;
	        }, reverse);
	      }
	      flatDeep(iterable, 0);
	      return iterations;
	    }
	    flatSequence.__iteratorUncached = function(type, reverse) {
	      var iterator = iterable.__iterator(type, reverse);
	      var stack = [];
	      var iterations = 0;
	      return new Iterator(function()  {
	        while (iterator) {
	          var step = iterator.next();
	          if (step.done !== false) {
	            iterator = stack.pop();
	            continue;
	          }
	          var v = step.value;
	          if (type === ITERATE_ENTRIES) {
	            v = v[1];
	          }
	          if ((!depth || stack.length < depth) && isIterable(v)) {
	            stack.push(iterator);
	            iterator = v.__iterator(type, reverse);
	          } else {
	            return useKeys ? step : iteratorValue(type, iterations++, v, step);
	          }
	        }
	        return iteratorDone();
	      });
	    }
	    return flatSequence;
	  }


	  function flatMapFactory(iterable, mapper, context) {
	    var coerce = iterableClass(iterable);
	    return iterable.toSeq().map(
	      function(v, k)  {return coerce(mapper.call(context, v, k, iterable))}
	    ).flatten(true);
	  }


	  function interposeFactory(iterable, separator) {
	    var interposedSequence = makeSequence(iterable);
	    interposedSequence.size = iterable.size && iterable.size * 2 -1;
	    interposedSequence.__iterateUncached = function(fn, reverse) {var this$0 = this;
	      var iterations = 0;
	      iterable.__iterate(function(v, k) 
	        {return (!iterations || fn(separator, iterations++, this$0) !== false) &&
	        fn(v, iterations++, this$0) !== false},
	        reverse
	      );
	      return iterations;
	    };
	    interposedSequence.__iteratorUncached = function(type, reverse) {
	      var iterator = iterable.__iterator(ITERATE_VALUES, reverse);
	      var iterations = 0;
	      var step;
	      return new Iterator(function()  {
	        if (!step || iterations % 2) {
	          step = iterator.next();
	          if (step.done) {
	            return step;
	          }
	        }
	        return iterations % 2 ?
	          iteratorValue(type, iterations++, separator) :
	          iteratorValue(type, iterations++, step.value, step);
	      });
	    };
	    return interposedSequence;
	  }


	  function sortFactory(iterable, comparator, mapper) {
	    if (!comparator) {
	      comparator = defaultComparator;
	    }
	    var isKeyedIterable = isKeyed(iterable);
	    var index = 0;
	    var entries = iterable.toSeq().map(
	      function(v, k)  {return [k, v, index++, mapper ? mapper(v, k, iterable) : v]}
	    ).toArray();
	    entries.sort(function(a, b)  {return comparator(a[3], b[3]) || a[2] - b[2]}).forEach(
	      isKeyedIterable ?
	      function(v, i)  { entries[i].length = 2; } :
	      function(v, i)  { entries[i] = v[1]; }
	    );
	    return isKeyedIterable ? KeyedSeq(entries) :
	      isIndexed(iterable) ? IndexedSeq(entries) :
	      SetSeq(entries);
	  }


	  function maxFactory(iterable, comparator, mapper) {
	    if (!comparator) {
	      comparator = defaultComparator;
	    }
	    if (mapper) {
	      var entry = iterable.toSeq()
	        .map(function(v, k)  {return [v, mapper(v, k, iterable)]})
	        .reduce(function(a, b)  {return maxCompare(comparator, a[1], b[1]) ? b : a});
	      return entry && entry[0];
	    } else {
	      return iterable.reduce(function(a, b)  {return maxCompare(comparator, a, b) ? b : a});
	    }
	  }

	  function maxCompare(comparator, a, b) {
	    var comp = comparator(b, a);
	    // b is considered the new max if the comparator declares them equal, but
	    // they are not equal and b is in fact a nullish value.
	    return (comp === 0 && b !== a && (b === undefined || b === null || b !== b)) || comp > 0;
	  }


	  function zipWithFactory(keyIter, zipper, iters) {
	    var zipSequence = makeSequence(keyIter);
	    zipSequence.size = new ArraySeq(iters).map(function(i ) {return i.size}).min();
	    // Note: this a generic base implementation of __iterate in terms of
	    // __iterator which may be more generically useful in the future.
	    zipSequence.__iterate = function(fn, reverse) {
	      /* generic:
	      var iterator = this.__iterator(ITERATE_ENTRIES, reverse);
	      var step;
	      var iterations = 0;
	      while (!(step = iterator.next()).done) {
	        iterations++;
	        if (fn(step.value[1], step.value[0], this) === false) {
	          break;
	        }
	      }
	      return iterations;
	      */
	      // indexed:
	      var iterator = this.__iterator(ITERATE_VALUES, reverse);
	      var step;
	      var iterations = 0;
	      while (!(step = iterator.next()).done) {
	        if (fn(step.value, iterations++, this) === false) {
	          break;
	        }
	      }
	      return iterations;
	    };
	    zipSequence.__iteratorUncached = function(type, reverse) {
	      var iterators = iters.map(function(i )
	        {return (i = Iterable(i), getIterator(reverse ? i.reverse() : i))}
	      );
	      var iterations = 0;
	      var isDone = false;
	      return new Iterator(function()  {
	        var steps;
	        if (!isDone) {
	          steps = iterators.map(function(i ) {return i.next()});
	          isDone = steps.some(function(s ) {return s.done});
	        }
	        if (isDone) {
	          return iteratorDone();
	        }
	        return iteratorValue(
	          type,
	          iterations++,
	          zipper.apply(null, steps.map(function(s ) {return s.value}))
	        );
	      });
	    };
	    return zipSequence
	  }


	  // #pragma Helper Functions

	  function reify(iter, seq) {
	    return isSeq(iter) ? seq : iter.constructor(seq);
	  }

	  function validateEntry(entry) {
	    if (entry !== Object(entry)) {
	      throw new TypeError('Expected [K, V] tuple: ' + entry);
	    }
	  }

	  function resolveSize(iter) {
	    assertNotInfinite(iter.size);
	    return ensureSize(iter);
	  }

	  function iterableClass(iterable) {
	    return isKeyed(iterable) ? KeyedIterable :
	      isIndexed(iterable) ? IndexedIterable :
	      SetIterable;
	  }

	  function makeSequence(iterable) {
	    return Object.create(
	      (
	        isKeyed(iterable) ? KeyedSeq :
	        isIndexed(iterable) ? IndexedSeq :
	        SetSeq
	      ).prototype
	    );
	  }

	  function cacheResultThrough() {
	    if (this._iter.cacheResult) {
	      this._iter.cacheResult();
	      this.size = this._iter.size;
	      return this;
	    } else {
	      return Seq.prototype.cacheResult.call(this);
	    }
	  }

	  function defaultComparator(a, b) {
	    return a > b ? 1 : a < b ? -1 : 0;
	  }

	  function forceIterator(keyPath) {
	    var iter = getIterator(keyPath);
	    if (!iter) {
	      // Array might not be iterable in this environment, so we need a fallback
	      // to our wrapped type.
	      if (!isArrayLike(keyPath)) {
	        throw new TypeError('Expected iterable or array-like: ' + keyPath);
	      }
	      iter = getIterator(Iterable(keyPath));
	    }
	    return iter;
	  }

	  createClass(Record, KeyedCollection);

	    function Record(defaultValues, name) {
	      var hasInitialized;

	      var RecordType = function Record(values) {
	        if (values instanceof RecordType) {
	          return values;
	        }
	        if (!(this instanceof RecordType)) {
	          return new RecordType(values);
	        }
	        if (!hasInitialized) {
	          hasInitialized = true;
	          var keys = Object.keys(defaultValues);
	          setProps(RecordTypePrototype, keys);
	          RecordTypePrototype.size = keys.length;
	          RecordTypePrototype._name = name;
	          RecordTypePrototype._keys = keys;
	          RecordTypePrototype._defaultValues = defaultValues;
	        }
	        this._map = Map(values);
	      };

	      var RecordTypePrototype = RecordType.prototype = Object.create(RecordPrototype);
	      RecordTypePrototype.constructor = RecordType;

	      return RecordType;
	    }

	    Record.prototype.toString = function() {
	      return this.__toString(recordName(this) + ' {', '}');
	    };

	    // @pragma Access

	    Record.prototype.has = function(k) {
	      return this._defaultValues.hasOwnProperty(k);
	    };

	    Record.prototype.get = function(k, notSetValue) {
	      if (!this.has(k)) {
	        return notSetValue;
	      }
	      var defaultVal = this._defaultValues[k];
	      return this._map ? this._map.get(k, defaultVal) : defaultVal;
	    };

	    // @pragma Modification

	    Record.prototype.clear = function() {
	      if (this.__ownerID) {
	        this._map && this._map.clear();
	        return this;
	      }
	      var RecordType = this.constructor;
	      return RecordType._empty || (RecordType._empty = makeRecord(this, emptyMap()));
	    };

	    Record.prototype.set = function(k, v) {
	      if (!this.has(k)) {
	        throw new Error('Cannot set unknown key "' + k + '" on ' + recordName(this));
	      }
	      if (this._map && !this._map.has(k)) {
	        var defaultVal = this._defaultValues[k];
	        if (v === defaultVal) {
	          return this;
	        }
	      }
	      var newMap = this._map && this._map.set(k, v);
	      if (this.__ownerID || newMap === this._map) {
	        return this;
	      }
	      return makeRecord(this, newMap);
	    };

	    Record.prototype.remove = function(k) {
	      if (!this.has(k)) {
	        return this;
	      }
	      var newMap = this._map && this._map.remove(k);
	      if (this.__ownerID || newMap === this._map) {
	        return this;
	      }
	      return makeRecord(this, newMap);
	    };

	    Record.prototype.wasAltered = function() {
	      return this._map.wasAltered();
	    };

	    Record.prototype.__iterator = function(type, reverse) {var this$0 = this;
	      return KeyedIterable(this._defaultValues).map(function(_, k)  {return this$0.get(k)}).__iterator(type, reverse);
	    };

	    Record.prototype.__iterate = function(fn, reverse) {var this$0 = this;
	      return KeyedIterable(this._defaultValues).map(function(_, k)  {return this$0.get(k)}).__iterate(fn, reverse);
	    };

	    Record.prototype.__ensureOwner = function(ownerID) {
	      if (ownerID === this.__ownerID) {
	        return this;
	      }
	      var newMap = this._map && this._map.__ensureOwner(ownerID);
	      if (!ownerID) {
	        this.__ownerID = ownerID;
	        this._map = newMap;
	        return this;
	      }
	      return makeRecord(this, newMap, ownerID);
	    };


	  var RecordPrototype = Record.prototype;
	  RecordPrototype[DELETE] = RecordPrototype.remove;
	  RecordPrototype.deleteIn =
	  RecordPrototype.removeIn = MapPrototype.removeIn;
	  RecordPrototype.merge = MapPrototype.merge;
	  RecordPrototype.mergeWith = MapPrototype.mergeWith;
	  RecordPrototype.mergeIn = MapPrototype.mergeIn;
	  RecordPrototype.mergeDeep = MapPrototype.mergeDeep;
	  RecordPrototype.mergeDeepWith = MapPrototype.mergeDeepWith;
	  RecordPrototype.mergeDeepIn = MapPrototype.mergeDeepIn;
	  RecordPrototype.setIn = MapPrototype.setIn;
	  RecordPrototype.update = MapPrototype.update;
	  RecordPrototype.updateIn = MapPrototype.updateIn;
	  RecordPrototype.withMutations = MapPrototype.withMutations;
	  RecordPrototype.asMutable = MapPrototype.asMutable;
	  RecordPrototype.asImmutable = MapPrototype.asImmutable;


	  function makeRecord(likeRecord, map, ownerID) {
	    var record = Object.create(Object.getPrototypeOf(likeRecord));
	    record._map = map;
	    record.__ownerID = ownerID;
	    return record;
	  }

	  function recordName(record) {
	    return record._name || record.constructor.name || 'Record';
	  }

	  function setProps(prototype, names) {
	    try {
	      names.forEach(setProp.bind(undefined, prototype));
	    } catch (error) {
	      // Object.defineProperty failed. Probably IE8.
	    }
	  }

	  function setProp(prototype, name) {
	    Object.defineProperty(prototype, name, {
	      get: function() {
	        return this.get(name);
	      },
	      set: function(value) {
	        invariant(this.__ownerID, 'Cannot set on an immutable record.');
	        this.set(name, value);
	      }
	    });
	  }

	  createClass(Set, SetCollection);

	    // @pragma Construction

	    function Set(value) {
	      return value === null || value === undefined ? emptySet() :
	        isSet(value) && !isOrdered(value) ? value :
	        emptySet().withMutations(function(set ) {
	          var iter = SetIterable(value);
	          assertNotInfinite(iter.size);
	          iter.forEach(function(v ) {return set.add(v)});
	        });
	    }

	    Set.of = function(/*...values*/) {
	      return this(arguments);
	    };

	    Set.fromKeys = function(value) {
	      return this(KeyedIterable(value).keySeq());
	    };

	    Set.prototype.toString = function() {
	      return this.__toString('Set {', '}');
	    };

	    // @pragma Access

	    Set.prototype.has = function(value) {
	      return this._map.has(value);
	    };

	    // @pragma Modification

	    Set.prototype.add = function(value) {
	      return updateSet(this, this._map.set(value, true));
	    };

	    Set.prototype.remove = function(value) {
	      return updateSet(this, this._map.remove(value));
	    };

	    Set.prototype.clear = function() {
	      return updateSet(this, this._map.clear());
	    };

	    // @pragma Composition

	    Set.prototype.union = function() {var iters = SLICE$0.call(arguments, 0);
	      iters = iters.filter(function(x ) {return x.size !== 0});
	      if (iters.length === 0) {
	        return this;
	      }
	      if (this.size === 0 && !this.__ownerID && iters.length === 1) {
	        return this.constructor(iters[0]);
	      }
	      return this.withMutations(function(set ) {
	        for (var ii = 0; ii < iters.length; ii++) {
	          SetIterable(iters[ii]).forEach(function(value ) {return set.add(value)});
	        }
	      });
	    };

	    Set.prototype.intersect = function() {var iters = SLICE$0.call(arguments, 0);
	      if (iters.length === 0) {
	        return this;
	      }
	      iters = iters.map(function(iter ) {return SetIterable(iter)});
	      var originalSet = this;
	      return this.withMutations(function(set ) {
	        originalSet.forEach(function(value ) {
	          if (!iters.every(function(iter ) {return iter.includes(value)})) {
	            set.remove(value);
	          }
	        });
	      });
	    };

	    Set.prototype.subtract = function() {var iters = SLICE$0.call(arguments, 0);
	      if (iters.length === 0) {
	        return this;
	      }
	      iters = iters.map(function(iter ) {return SetIterable(iter)});
	      var originalSet = this;
	      return this.withMutations(function(set ) {
	        originalSet.forEach(function(value ) {
	          if (iters.some(function(iter ) {return iter.includes(value)})) {
	            set.remove(value);
	          }
	        });
	      });
	    };

	    Set.prototype.merge = function() {
	      return this.union.apply(this, arguments);
	    };

	    Set.prototype.mergeWith = function(merger) {var iters = SLICE$0.call(arguments, 1);
	      return this.union.apply(this, iters);
	    };

	    Set.prototype.sort = function(comparator) {
	      // Late binding
	      return OrderedSet(sortFactory(this, comparator));
	    };

	    Set.prototype.sortBy = function(mapper, comparator) {
	      // Late binding
	      return OrderedSet(sortFactory(this, comparator, mapper));
	    };

	    Set.prototype.wasAltered = function() {
	      return this._map.wasAltered();
	    };

	    Set.prototype.__iterate = function(fn, reverse) {var this$0 = this;
	      return this._map.__iterate(function(_, k)  {return fn(k, k, this$0)}, reverse);
	    };

	    Set.prototype.__iterator = function(type, reverse) {
	      return this._map.map(function(_, k)  {return k}).__iterator(type, reverse);
	    };

	    Set.prototype.__ensureOwner = function(ownerID) {
	      if (ownerID === this.__ownerID) {
	        return this;
	      }
	      var newMap = this._map.__ensureOwner(ownerID);
	      if (!ownerID) {
	        this.__ownerID = ownerID;
	        this._map = newMap;
	        return this;
	      }
	      return this.__make(newMap, ownerID);
	    };


	  function isSet(maybeSet) {
	    return !!(maybeSet && maybeSet[IS_SET_SENTINEL]);
	  }

	  Set.isSet = isSet;

	  var IS_SET_SENTINEL = '@@__IMMUTABLE_SET__@@';

	  var SetPrototype = Set.prototype;
	  SetPrototype[IS_SET_SENTINEL] = true;
	  SetPrototype[DELETE] = SetPrototype.remove;
	  SetPrototype.mergeDeep = SetPrototype.merge;
	  SetPrototype.mergeDeepWith = SetPrototype.mergeWith;
	  SetPrototype.withMutations = MapPrototype.withMutations;
	  SetPrototype.asMutable = MapPrototype.asMutable;
	  SetPrototype.asImmutable = MapPrototype.asImmutable;

	  SetPrototype.__empty = emptySet;
	  SetPrototype.__make = makeSet;

	  function updateSet(set, newMap) {
	    if (set.__ownerID) {
	      set.size = newMap.size;
	      set._map = newMap;
	      return set;
	    }
	    return newMap === set._map ? set :
	      newMap.size === 0 ? set.__empty() :
	      set.__make(newMap);
	  }

	  function makeSet(map, ownerID) {
	    var set = Object.create(SetPrototype);
	    set.size = map ? map.size : 0;
	    set._map = map;
	    set.__ownerID = ownerID;
	    return set;
	  }

	  var EMPTY_SET;
	  function emptySet() {
	    return EMPTY_SET || (EMPTY_SET = makeSet(emptyMap()));
	  }

	  createClass(OrderedSet, Set);

	    // @pragma Construction

	    function OrderedSet(value) {
	      return value === null || value === undefined ? emptyOrderedSet() :
	        isOrderedSet(value) ? value :
	        emptyOrderedSet().withMutations(function(set ) {
	          var iter = SetIterable(value);
	          assertNotInfinite(iter.size);
	          iter.forEach(function(v ) {return set.add(v)});
	        });
	    }

	    OrderedSet.of = function(/*...values*/) {
	      return this(arguments);
	    };

	    OrderedSet.fromKeys = function(value) {
	      return this(KeyedIterable(value).keySeq());
	    };

	    OrderedSet.prototype.toString = function() {
	      return this.__toString('OrderedSet {', '}');
	    };


	  function isOrderedSet(maybeOrderedSet) {
	    return isSet(maybeOrderedSet) && isOrdered(maybeOrderedSet);
	  }

	  OrderedSet.isOrderedSet = isOrderedSet;

	  var OrderedSetPrototype = OrderedSet.prototype;
	  OrderedSetPrototype[IS_ORDERED_SENTINEL] = true;

	  OrderedSetPrototype.__empty = emptyOrderedSet;
	  OrderedSetPrototype.__make = makeOrderedSet;

	  function makeOrderedSet(map, ownerID) {
	    var set = Object.create(OrderedSetPrototype);
	    set.size = map ? map.size : 0;
	    set._map = map;
	    set.__ownerID = ownerID;
	    return set;
	  }

	  var EMPTY_ORDERED_SET;
	  function emptyOrderedSet() {
	    return EMPTY_ORDERED_SET || (EMPTY_ORDERED_SET = makeOrderedSet(emptyOrderedMap()));
	  }

	  createClass(Stack, IndexedCollection);

	    // @pragma Construction

	    function Stack(value) {
	      return value === null || value === undefined ? emptyStack() :
	        isStack(value) ? value :
	        emptyStack().unshiftAll(value);
	    }

	    Stack.of = function(/*...values*/) {
	      return this(arguments);
	    };

	    Stack.prototype.toString = function() {
	      return this.__toString('Stack [', ']');
	    };

	    // @pragma Access

	    Stack.prototype.get = function(index, notSetValue) {
	      var head = this._head;
	      index = wrapIndex(this, index);
	      while (head && index--) {
	        head = head.next;
	      }
	      return head ? head.value : notSetValue;
	    };

	    Stack.prototype.peek = function() {
	      return this._head && this._head.value;
	    };

	    // @pragma Modification

	    Stack.prototype.push = function(/*...values*/) {
	      if (arguments.length === 0) {
	        return this;
	      }
	      var newSize = this.size + arguments.length;
	      var head = this._head;
	      for (var ii = arguments.length - 1; ii >= 0; ii--) {
	        head = {
	          value: arguments[ii],
	          next: head
	        };
	      }
	      if (this.__ownerID) {
	        this.size = newSize;
	        this._head = head;
	        this.__hash = undefined;
	        this.__altered = true;
	        return this;
	      }
	      return makeStack(newSize, head);
	    };

	    Stack.prototype.pushAll = function(iter) {
	      iter = IndexedIterable(iter);
	      if (iter.size === 0) {
	        return this;
	      }
	      assertNotInfinite(iter.size);
	      var newSize = this.size;
	      var head = this._head;
	      iter.reverse().forEach(function(value ) {
	        newSize++;
	        head = {
	          value: value,
	          next: head
	        };
	      });
	      if (this.__ownerID) {
	        this.size = newSize;
	        this._head = head;
	        this.__hash = undefined;
	        this.__altered = true;
	        return this;
	      }
	      return makeStack(newSize, head);
	    };

	    Stack.prototype.pop = function() {
	      return this.slice(1);
	    };

	    Stack.prototype.unshift = function(/*...values*/) {
	      return this.push.apply(this, arguments);
	    };

	    Stack.prototype.unshiftAll = function(iter) {
	      return this.pushAll(iter);
	    };

	    Stack.prototype.shift = function() {
	      return this.pop.apply(this, arguments);
	    };

	    Stack.prototype.clear = function() {
	      if (this.size === 0) {
	        return this;
	      }
	      if (this.__ownerID) {
	        this.size = 0;
	        this._head = undefined;
	        this.__hash = undefined;
	        this.__altered = true;
	        return this;
	      }
	      return emptyStack();
	    };

	    Stack.prototype.slice = function(begin, end) {
	      if (wholeSlice(begin, end, this.size)) {
	        return this;
	      }
	      var resolvedBegin = resolveBegin(begin, this.size);
	      var resolvedEnd = resolveEnd(end, this.size);
	      if (resolvedEnd !== this.size) {
	        // super.slice(begin, end);
	        return IndexedCollection.prototype.slice.call(this, begin, end);
	      }
	      var newSize = this.size - resolvedBegin;
	      var head = this._head;
	      while (resolvedBegin--) {
	        head = head.next;
	      }
	      if (this.__ownerID) {
	        this.size = newSize;
	        this._head = head;
	        this.__hash = undefined;
	        this.__altered = true;
	        return this;
	      }
	      return makeStack(newSize, head);
	    };

	    // @pragma Mutability

	    Stack.prototype.__ensureOwner = function(ownerID) {
	      if (ownerID === this.__ownerID) {
	        return this;
	      }
	      if (!ownerID) {
	        this.__ownerID = ownerID;
	        this.__altered = false;
	        return this;
	      }
	      return makeStack(this.size, this._head, ownerID, this.__hash);
	    };

	    // @pragma Iteration

	    Stack.prototype.__iterate = function(fn, reverse) {
	      if (reverse) {
	        return this.reverse().__iterate(fn);
	      }
	      var iterations = 0;
	      var node = this._head;
	      while (node) {
	        if (fn(node.value, iterations++, this) === false) {
	          break;
	        }
	        node = node.next;
	      }
	      return iterations;
	    };

	    Stack.prototype.__iterator = function(type, reverse) {
	      if (reverse) {
	        return this.reverse().__iterator(type);
	      }
	      var iterations = 0;
	      var node = this._head;
	      return new Iterator(function()  {
	        if (node) {
	          var value = node.value;
	          node = node.next;
	          return iteratorValue(type, iterations++, value);
	        }
	        return iteratorDone();
	      });
	    };


	  function isStack(maybeStack) {
	    return !!(maybeStack && maybeStack[IS_STACK_SENTINEL]);
	  }

	  Stack.isStack = isStack;

	  var IS_STACK_SENTINEL = '@@__IMMUTABLE_STACK__@@';

	  var StackPrototype = Stack.prototype;
	  StackPrototype[IS_STACK_SENTINEL] = true;
	  StackPrototype.withMutations = MapPrototype.withMutations;
	  StackPrototype.asMutable = MapPrototype.asMutable;
	  StackPrototype.asImmutable = MapPrototype.asImmutable;
	  StackPrototype.wasAltered = MapPrototype.wasAltered;


	  function makeStack(size, head, ownerID, hash) {
	    var map = Object.create(StackPrototype);
	    map.size = size;
	    map._head = head;
	    map.__ownerID = ownerID;
	    map.__hash = hash;
	    map.__altered = false;
	    return map;
	  }

	  var EMPTY_STACK;
	  function emptyStack() {
	    return EMPTY_STACK || (EMPTY_STACK = makeStack(0));
	  }

	  /**
	   * Contributes additional methods to a constructor
	   */
	  function mixin(ctor, methods) {
	    var keyCopier = function(key ) { ctor.prototype[key] = methods[key]; };
	    Object.keys(methods).forEach(keyCopier);
	    Object.getOwnPropertySymbols &&
	      Object.getOwnPropertySymbols(methods).forEach(keyCopier);
	    return ctor;
	  }

	  Iterable.Iterator = Iterator;

	  mixin(Iterable, {

	    // ### Conversion to other types

	    toArray: function() {
	      assertNotInfinite(this.size);
	      var array = new Array(this.size || 0);
	      this.valueSeq().__iterate(function(v, i)  { array[i] = v; });
	      return array;
	    },

	    toIndexedSeq: function() {
	      return new ToIndexedSequence(this);
	    },

	    toJS: function() {
	      return this.toSeq().map(
	        function(value ) {return value && typeof value.toJS === 'function' ? value.toJS() : value}
	      ).__toJS();
	    },

	    toJSON: function() {
	      return this.toSeq().map(
	        function(value ) {return value && typeof value.toJSON === 'function' ? value.toJSON() : value}
	      ).__toJS();
	    },

	    toKeyedSeq: function() {
	      return new ToKeyedSequence(this, true);
	    },

	    toMap: function() {
	      // Use Late Binding here to solve the circular dependency.
	      return Map(this.toKeyedSeq());
	    },

	    toObject: function() {
	      assertNotInfinite(this.size);
	      var object = {};
	      this.__iterate(function(v, k)  { object[k] = v; });
	      return object;
	    },

	    toOrderedMap: function() {
	      // Use Late Binding here to solve the circular dependency.
	      return OrderedMap(this.toKeyedSeq());
	    },

	    toOrderedSet: function() {
	      // Use Late Binding here to solve the circular dependency.
	      return OrderedSet(isKeyed(this) ? this.valueSeq() : this);
	    },

	    toSet: function() {
	      // Use Late Binding here to solve the circular dependency.
	      return Set(isKeyed(this) ? this.valueSeq() : this);
	    },

	    toSetSeq: function() {
	      return new ToSetSequence(this);
	    },

	    toSeq: function() {
	      return isIndexed(this) ? this.toIndexedSeq() :
	        isKeyed(this) ? this.toKeyedSeq() :
	        this.toSetSeq();
	    },

	    toStack: function() {
	      // Use Late Binding here to solve the circular dependency.
	      return Stack(isKeyed(this) ? this.valueSeq() : this);
	    },

	    toList: function() {
	      // Use Late Binding here to solve the circular dependency.
	      return List(isKeyed(this) ? this.valueSeq() : this);
	    },


	    // ### Common JavaScript methods and properties

	    toString: function() {
	      return '[Iterable]';
	    },

	    __toString: function(head, tail) {
	      if (this.size === 0) {
	        return head + tail;
	      }
	      return head + ' ' + this.toSeq().map(this.__toStringMapper).join(', ') + ' ' + tail;
	    },


	    // ### ES6 Collection methods (ES6 Array and Map)

	    concat: function() {var values = SLICE$0.call(arguments, 0);
	      return reify(this, concatFactory(this, values));
	    },

	    includes: function(searchValue) {
	      return this.some(function(value ) {return is(value, searchValue)});
	    },

	    entries: function() {
	      return this.__iterator(ITERATE_ENTRIES);
	    },

	    every: function(predicate, context) {
	      assertNotInfinite(this.size);
	      var returnValue = true;
	      this.__iterate(function(v, k, c)  {
	        if (!predicate.call(context, v, k, c)) {
	          returnValue = false;
	          return false;
	        }
	      });
	      return returnValue;
	    },

	    filter: function(predicate, context) {
	      return reify(this, filterFactory(this, predicate, context, true));
	    },

	    find: function(predicate, context, notSetValue) {
	      var entry = this.findEntry(predicate, context);
	      return entry ? entry[1] : notSetValue;
	    },

	    forEach: function(sideEffect, context) {
	      assertNotInfinite(this.size);
	      return this.__iterate(context ? sideEffect.bind(context) : sideEffect);
	    },

	    join: function(separator) {
	      assertNotInfinite(this.size);
	      separator = separator !== undefined ? '' + separator : ',';
	      var joined = '';
	      var isFirst = true;
	      this.__iterate(function(v ) {
	        isFirst ? (isFirst = false) : (joined += separator);
	        joined += v !== null && v !== undefined ? v.toString() : '';
	      });
	      return joined;
	    },

	    keys: function() {
	      return this.__iterator(ITERATE_KEYS);
	    },

	    map: function(mapper, context) {
	      return reify(this, mapFactory(this, mapper, context));
	    },

	    reduce: function(reducer, initialReduction, context) {
	      assertNotInfinite(this.size);
	      var reduction;
	      var useFirst;
	      if (arguments.length < 2) {
	        useFirst = true;
	      } else {
	        reduction = initialReduction;
	      }
	      this.__iterate(function(v, k, c)  {
	        if (useFirst) {
	          useFirst = false;
	          reduction = v;
	        } else {
	          reduction = reducer.call(context, reduction, v, k, c);
	        }
	      });
	      return reduction;
	    },

	    reduceRight: function(reducer, initialReduction, context) {
	      var reversed = this.toKeyedSeq().reverse();
	      return reversed.reduce.apply(reversed, arguments);
	    },

	    reverse: function() {
	      return reify(this, reverseFactory(this, true));
	    },

	    slice: function(begin, end) {
	      return reify(this, sliceFactory(this, begin, end, true));
	    },

	    some: function(predicate, context) {
	      return !this.every(not(predicate), context);
	    },

	    sort: function(comparator) {
	      return reify(this, sortFactory(this, comparator));
	    },

	    values: function() {
	      return this.__iterator(ITERATE_VALUES);
	    },


	    // ### More sequential methods

	    butLast: function() {
	      return this.slice(0, -1);
	    },

	    isEmpty: function() {
	      return this.size !== undefined ? this.size === 0 : !this.some(function()  {return true});
	    },

	    count: function(predicate, context) {
	      return ensureSize(
	        predicate ? this.toSeq().filter(predicate, context) : this
	      );
	    },

	    countBy: function(grouper, context) {
	      return countByFactory(this, grouper, context);
	    },

	    equals: function(other) {
	      return deepEqual(this, other);
	    },

	    entrySeq: function() {
	      var iterable = this;
	      if (iterable._cache) {
	        // We cache as an entries array, so we can just return the cache!
	        return new ArraySeq(iterable._cache);
	      }
	      var entriesSequence = iterable.toSeq().map(entryMapper).toIndexedSeq();
	      entriesSequence.fromEntrySeq = function()  {return iterable.toSeq()};
	      return entriesSequence;
	    },

	    filterNot: function(predicate, context) {
	      return this.filter(not(predicate), context);
	    },

	    findEntry: function(predicate, context, notSetValue) {
	      var found = notSetValue;
	      this.__iterate(function(v, k, c)  {
	        if (predicate.call(context, v, k, c)) {
	          found = [k, v];
	          return false;
	        }
	      });
	      return found;
	    },

	    findKey: function(predicate, context) {
	      var entry = this.findEntry(predicate, context);
	      return entry && entry[0];
	    },

	    findLast: function(predicate, context, notSetValue) {
	      return this.toKeyedSeq().reverse().find(predicate, context, notSetValue);
	    },

	    findLastEntry: function(predicate, context, notSetValue) {
	      return this.toKeyedSeq().reverse().findEntry(predicate, context, notSetValue);
	    },

	    findLastKey: function(predicate, context) {
	      return this.toKeyedSeq().reverse().findKey(predicate, context);
	    },

	    first: function() {
	      return this.find(returnTrue);
	    },

	    flatMap: function(mapper, context) {
	      return reify(this, flatMapFactory(this, mapper, context));
	    },

	    flatten: function(depth) {
	      return reify(this, flattenFactory(this, depth, true));
	    },

	    fromEntrySeq: function() {
	      return new FromEntriesSequence(this);
	    },

	    get: function(searchKey, notSetValue) {
	      return this.find(function(_, key)  {return is(key, searchKey)}, undefined, notSetValue);
	    },

	    getIn: function(searchKeyPath, notSetValue) {
	      var nested = this;
	      // Note: in an ES6 environment, we would prefer:
	      // for (var key of searchKeyPath) {
	      var iter = forceIterator(searchKeyPath);
	      var step;
	      while (!(step = iter.next()).done) {
	        var key = step.value;
	        nested = nested && nested.get ? nested.get(key, NOT_SET) : NOT_SET;
	        if (nested === NOT_SET) {
	          return notSetValue;
	        }
	      }
	      return nested;
	    },

	    groupBy: function(grouper, context) {
	      return groupByFactory(this, grouper, context);
	    },

	    has: function(searchKey) {
	      return this.get(searchKey, NOT_SET) !== NOT_SET;
	    },

	    hasIn: function(searchKeyPath) {
	      return this.getIn(searchKeyPath, NOT_SET) !== NOT_SET;
	    },

	    isSubset: function(iter) {
	      iter = typeof iter.includes === 'function' ? iter : Iterable(iter);
	      return this.every(function(value ) {return iter.includes(value)});
	    },

	    isSuperset: function(iter) {
	      iter = typeof iter.isSubset === 'function' ? iter : Iterable(iter);
	      return iter.isSubset(this);
	    },

	    keyOf: function(searchValue) {
	      return this.findKey(function(value ) {return is(value, searchValue)});
	    },

	    keySeq: function() {
	      return this.toSeq().map(keyMapper).toIndexedSeq();
	    },

	    last: function() {
	      return this.toSeq().reverse().first();
	    },

	    lastKeyOf: function(searchValue) {
	      return this.toKeyedSeq().reverse().keyOf(searchValue);
	    },

	    max: function(comparator) {
	      return maxFactory(this, comparator);
	    },

	    maxBy: function(mapper, comparator) {
	      return maxFactory(this, comparator, mapper);
	    },

	    min: function(comparator) {
	      return maxFactory(this, comparator ? neg(comparator) : defaultNegComparator);
	    },

	    minBy: function(mapper, comparator) {
	      return maxFactory(this, comparator ? neg(comparator) : defaultNegComparator, mapper);
	    },

	    rest: function() {
	      return this.slice(1);
	    },

	    skip: function(amount) {
	      return this.slice(Math.max(0, amount));
	    },

	    skipLast: function(amount) {
	      return reify(this, this.toSeq().reverse().skip(amount).reverse());
	    },

	    skipWhile: function(predicate, context) {
	      return reify(this, skipWhileFactory(this, predicate, context, true));
	    },

	    skipUntil: function(predicate, context) {
	      return this.skipWhile(not(predicate), context);
	    },

	    sortBy: function(mapper, comparator) {
	      return reify(this, sortFactory(this, comparator, mapper));
	    },

	    take: function(amount) {
	      return this.slice(0, Math.max(0, amount));
	    },

	    takeLast: function(amount) {
	      return reify(this, this.toSeq().reverse().take(amount).reverse());
	    },

	    takeWhile: function(predicate, context) {
	      return reify(this, takeWhileFactory(this, predicate, context));
	    },

	    takeUntil: function(predicate, context) {
	      return this.takeWhile(not(predicate), context);
	    },

	    valueSeq: function() {
	      return this.toIndexedSeq();
	    },


	    // ### Hashable Object

	    hashCode: function() {
	      return this.__hash || (this.__hash = hashIterable(this));
	    }


	    // ### Internal

	    // abstract __iterate(fn, reverse)

	    // abstract __iterator(type, reverse)
	  });

	  // var IS_ITERABLE_SENTINEL = '@@__IMMUTABLE_ITERABLE__@@';
	  // var IS_KEYED_SENTINEL = '@@__IMMUTABLE_KEYED__@@';
	  // var IS_INDEXED_SENTINEL = '@@__IMMUTABLE_INDEXED__@@';
	  // var IS_ORDERED_SENTINEL = '@@__IMMUTABLE_ORDERED__@@';

	  var IterablePrototype = Iterable.prototype;
	  IterablePrototype[IS_ITERABLE_SENTINEL] = true;
	  IterablePrototype[ITERATOR_SYMBOL] = IterablePrototype.values;
	  IterablePrototype.__toJS = IterablePrototype.toArray;
	  IterablePrototype.__toStringMapper = quoteString;
	  IterablePrototype.inspect =
	  IterablePrototype.toSource = function() { return this.toString(); };
	  IterablePrototype.chain = IterablePrototype.flatMap;
	  IterablePrototype.contains = IterablePrototype.includes;

	  mixin(KeyedIterable, {

	    // ### More sequential methods

	    flip: function() {
	      return reify(this, flipFactory(this));
	    },

	    mapEntries: function(mapper, context) {var this$0 = this;
	      var iterations = 0;
	      return reify(this,
	        this.toSeq().map(
	          function(v, k)  {return mapper.call(context, [k, v], iterations++, this$0)}
	        ).fromEntrySeq()
	      );
	    },

	    mapKeys: function(mapper, context) {var this$0 = this;
	      return reify(this,
	        this.toSeq().flip().map(
	          function(k, v)  {return mapper.call(context, k, v, this$0)}
	        ).flip()
	      );
	    }

	  });

	  var KeyedIterablePrototype = KeyedIterable.prototype;
	  KeyedIterablePrototype[IS_KEYED_SENTINEL] = true;
	  KeyedIterablePrototype[ITERATOR_SYMBOL] = IterablePrototype.entries;
	  KeyedIterablePrototype.__toJS = IterablePrototype.toObject;
	  KeyedIterablePrototype.__toStringMapper = function(v, k)  {return JSON.stringify(k) + ': ' + quoteString(v)};



	  mixin(IndexedIterable, {

	    // ### Conversion to other types

	    toKeyedSeq: function() {
	      return new ToKeyedSequence(this, false);
	    },


	    // ### ES6 Collection methods (ES6 Array and Map)

	    filter: function(predicate, context) {
	      return reify(this, filterFactory(this, predicate, context, false));
	    },

	    findIndex: function(predicate, context) {
	      var entry = this.findEntry(predicate, context);
	      return entry ? entry[0] : -1;
	    },

	    indexOf: function(searchValue) {
	      var key = this.keyOf(searchValue);
	      return key === undefined ? -1 : key;
	    },

	    lastIndexOf: function(searchValue) {
	      var key = this.lastKeyOf(searchValue);
	      return key === undefined ? -1 : key;
	    },

	    reverse: function() {
	      return reify(this, reverseFactory(this, false));
	    },

	    slice: function(begin, end) {
	      return reify(this, sliceFactory(this, begin, end, false));
	    },

	    splice: function(index, removeNum /*, ...values*/) {
	      var numArgs = arguments.length;
	      removeNum = Math.max(removeNum | 0, 0);
	      if (numArgs === 0 || (numArgs === 2 && !removeNum)) {
	        return this;
	      }
	      // If index is negative, it should resolve relative to the size of the
	      // collection. However size may be expensive to compute if not cached, so
	      // only call count() if the number is in fact negative.
	      index = resolveBegin(index, index < 0 ? this.count() : this.size);
	      var spliced = this.slice(0, index);
	      return reify(
	        this,
	        numArgs === 1 ?
	          spliced :
	          spliced.concat(arrCopy(arguments, 2), this.slice(index + removeNum))
	      );
	    },


	    // ### More collection methods

	    findLastIndex: function(predicate, context) {
	      var entry = this.findLastEntry(predicate, context);
	      return entry ? entry[0] : -1;
	    },

	    first: function() {
	      return this.get(0);
	    },

	    flatten: function(depth) {
	      return reify(this, flattenFactory(this, depth, false));
	    },

	    get: function(index, notSetValue) {
	      index = wrapIndex(this, index);
	      return (index < 0 || (this.size === Infinity ||
	          (this.size !== undefined && index > this.size))) ?
	        notSetValue :
	        this.find(function(_, key)  {return key === index}, undefined, notSetValue);
	    },

	    has: function(index) {
	      index = wrapIndex(this, index);
	      return index >= 0 && (this.size !== undefined ?
	        this.size === Infinity || index < this.size :
	        this.indexOf(index) !== -1
	      );
	    },

	    interpose: function(separator) {
	      return reify(this, interposeFactory(this, separator));
	    },

	    interleave: function(/*...iterables*/) {
	      var iterables = [this].concat(arrCopy(arguments));
	      var zipped = zipWithFactory(this.toSeq(), IndexedSeq.of, iterables);
	      var interleaved = zipped.flatten(true);
	      if (zipped.size) {
	        interleaved.size = zipped.size * iterables.length;
	      }
	      return reify(this, interleaved);
	    },

	    keySeq: function() {
	      return Range(0, this.size);
	    },

	    last: function() {
	      return this.get(-1);
	    },

	    skipWhile: function(predicate, context) {
	      return reify(this, skipWhileFactory(this, predicate, context, false));
	    },

	    zip: function(/*, ...iterables */) {
	      var iterables = [this].concat(arrCopy(arguments));
	      return reify(this, zipWithFactory(this, defaultZipper, iterables));
	    },

	    zipWith: function(zipper/*, ...iterables */) {
	      var iterables = arrCopy(arguments);
	      iterables[0] = this;
	      return reify(this, zipWithFactory(this, zipper, iterables));
	    }

	  });

	  IndexedIterable.prototype[IS_INDEXED_SENTINEL] = true;
	  IndexedIterable.prototype[IS_ORDERED_SENTINEL] = true;



	  mixin(SetIterable, {

	    // ### ES6 Collection methods (ES6 Array and Map)

	    get: function(value, notSetValue) {
	      return this.has(value) ? value : notSetValue;
	    },

	    includes: function(value) {
	      return this.has(value);
	    },


	    // ### More sequential methods

	    keySeq: function() {
	      return this.valueSeq();
	    }

	  });

	  SetIterable.prototype.has = IterablePrototype.includes;
	  SetIterable.prototype.contains = SetIterable.prototype.includes;


	  // Mixin subclasses

	  mixin(KeyedSeq, KeyedIterable.prototype);
	  mixin(IndexedSeq, IndexedIterable.prototype);
	  mixin(SetSeq, SetIterable.prototype);

	  mixin(KeyedCollection, KeyedIterable.prototype);
	  mixin(IndexedCollection, IndexedIterable.prototype);
	  mixin(SetCollection, SetIterable.prototype);


	  // #pragma Helper functions

	  function keyMapper(v, k) {
	    return k;
	  }

	  function entryMapper(v, k) {
	    return [k, v];
	  }

	  function not(predicate) {
	    return function() {
	      return !predicate.apply(this, arguments);
	    }
	  }

	  function neg(predicate) {
	    return function() {
	      return -predicate.apply(this, arguments);
	    }
	  }

	  function quoteString(value) {
	    return typeof value === 'string' ? JSON.stringify(value) : String(value);
	  }

	  function defaultZipper() {
	    return arrCopy(arguments);
	  }

	  function defaultNegComparator(a, b) {
	    return a < b ? 1 : a > b ? -1 : 0;
	  }

	  function hashIterable(iterable) {
	    if (iterable.size === Infinity) {
	      return 0;
	    }
	    var ordered = isOrdered(iterable);
	    var keyed = isKeyed(iterable);
	    var h = ordered ? 1 : 0;
	    var size = iterable.__iterate(
	      keyed ?
	        ordered ?
	          function(v, k)  { h = 31 * h + hashMerge(hash(v), hash(k)) | 0; } :
	          function(v, k)  { h = h + hashMerge(hash(v), hash(k)) | 0; } :
	        ordered ?
	          function(v ) { h = 31 * h + hash(v) | 0; } :
	          function(v ) { h = h + hash(v) | 0; }
	    );
	    return murmurHashOfSize(size, h);
	  }

	  function murmurHashOfSize(size, h) {
	    h = imul(h, 0xCC9E2D51);
	    h = imul(h << 15 | h >>> -15, 0x1B873593);
	    h = imul(h << 13 | h >>> -13, 5);
	    h = (h + 0xE6546B64 | 0) ^ size;
	    h = imul(h ^ h >>> 16, 0x85EBCA6B);
	    h = imul(h ^ h >>> 13, 0xC2B2AE35);
	    h = smi(h ^ h >>> 16);
	    return h;
	  }

	  function hashMerge(a, b) {
	    return a ^ b + 0x9E3779B9 + (a << 6) + (a >> 2) | 0; // int
	  }

	  var Immutable = {

	    Iterable: Iterable,

	    Seq: Seq,
	    Collection: Collection,
	    Map: Map,
	    OrderedMap: OrderedMap,
	    List: List,
	    Stack: Stack,
	    Set: Set,
	    OrderedSet: OrderedSet,

	    Record: Record,
	    Range: Range,
	    Repeat: Repeat,

	    is: is,
	    fromJS: fromJS

	  };

	  return Immutable;

	}));

/***/ },

/***/ 453:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

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

	var _require = __webpack_require__(216),
	    Store = _require.Store,
	    toImmutable = _require.toImmutable;

	var _require2 = __webpack_require__(372),
	    TLPT_RECEIVE_USER_INVITE = _require2.TLPT_RECEIVE_USER_INVITE;

	exports.default = Store({
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

/***/ 454:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

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

	var _require = __webpack_require__(216),
	    Store = _require.Store,
	    toImmutable = _require.toImmutable;

	var _require2 = __webpack_require__(237),
	    TLPT_NODES_RECEIVE = _require2.TLPT_NODES_RECEIVE;

	exports.default = Store({
	  getInitialState: function getInitialState() {
	    return toImmutable([]);
	  },
	  initialize: function initialize() {
	    this.on(TLPT_NODES_RECEIVE, receiveNodes);
	  }
	});


	function receiveNodes(state, _ref) {
	  var siteId = _ref.siteId,
	      nodeArray = _ref.nodeArray;

	  return state.filter(function (o) {
	    return o.get('siteId') !== siteId;
	  }).concat(toImmutable(nodeArray));
	}
	module.exports = exports['default'];

/***/ },

/***/ 455:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

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

	var _require = __webpack_require__(216),
	    Store = _require.Store,
	    toImmutable = _require.toImmutable;

	var _require2 = __webpack_require__(374),
	    TLPT_REST_API_START = _require2.TLPT_REST_API_START,
	    TLPT_REST_API_SUCCESS = _require2.TLPT_REST_API_SUCCESS,
	    TLPT_REST_API_FAIL = _require2.TLPT_REST_API_FAIL;

	exports.default = Store({
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

/***/ 456:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

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

	var _require = __webpack_require__(216),
	    Store = _require.Store,
	    toImmutable = _require.toImmutable;

	var _require2 = __webpack_require__(340),
	    TLPT_SESSIONS_RECEIVE = _require2.TLPT_SESSIONS_RECEIVE,
	    TLPT_SESSIONS_UPDATE = _require2.TLPT_SESSIONS_UPDATE,
	    TLPT_SESSIONS_UPDATE_WITH_EVENTS = _require2.TLPT_SESSIONS_UPDATE_WITH_EVENTS;

	var PORT_REGEX = /:\d+$/;

	exports.default = Store({
	  getInitialState: function getInitialState() {
	    return toImmutable({});
	  },
	  initialize: function initialize() {
	    this.on(TLPT_SESSIONS_UPDATE_WITH_EVENTS, updateSessionWithEvents);
	    this.on(TLPT_SESSIONS_RECEIVE, receiveSessions);
	    this.on(TLPT_SESSIONS_UPDATE, updateSession);
	  }
	});


	function getIp(addr) {
	  addr = addr || '';
	  return addr.replace(PORT_REGEX, '');
	}

	function updateSessionWithEvents(state, _ref) {
	  var _ref$jsonEvents = _ref.jsonEvents,
	      jsonEvents = _ref$jsonEvents === undefined ? [] : _ref$jsonEvents,
	      siteId = _ref.siteId;

	  return state.withMutations(function (state) {
	    jsonEvents.forEach(function (item) {
	      if (item.event !== 'session.start' && item.event !== 'session.end') {
	        return;
	      }

	      // check if record already exists
	      var session = state.get(item.sid);
	      if (!session) {
	        session = {};
	      } else {
	        session = session.toJS();
	      }

	      session.id = item.sid;
	      session.user = item.user;

	      if (item.event === 'session.start') {
	        session.created = item.time;
	        session.nodeIp = getIp(item['addr.local']);
	        session.clientIp = getIp(item['addr.remote']);
	      }

	      if (item.event === 'session.end') {
	        session.last_active = item.time;
	        session.active = false;
	        session.stored = true;
	      }

	      session.siteId = siteId;
	      state.set(session.id, toImmutable(session));
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
	      if (!state.getIn([item.id, 'stored'])) {
	        state.set(item.id, toImmutable(item));
	      }
	    });
	  });
	}
	module.exports = exports['default'];

/***/ },

/***/ 457:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

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

	var _require = __webpack_require__(216),
	    Store = _require.Store,
	    toImmutable = _require.toImmutable;

	var moment = __webpack_require__(240);

	var _require2 = __webpack_require__(397),
	    TLPT_STORED_SESSINS_FILTER_SET_RANGE = _require2.TLPT_STORED_SESSINS_FILTER_SET_RANGE;

	exports.default = Store({
	  getInitialState: function getInitialState() {

	    var end = moment(new Date()).endOf('day').toDate();
	    var start = moment(end).subtract(3, 'day').startOf('day').toDate();
	    var state = {
	      start: start,
	      end: end
	    };

	    return toImmutable(state);
	  },
	  initialize: function initialize() {
	    this.on(TLPT_STORED_SESSINS_FILTER_SET_RANGE, setRange);
	  }
	});


	function setRange(state, newState) {
	  return state.merge(newState);
	}
	module.exports = exports['default'];

/***/ },

/***/ 458:
/***/ function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _nuclearJs = __webpack_require__(216);

	var _actionTypes = __webpack_require__(233);

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

	exports.default = (0, _nuclearJs.Store)({
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

/***/ }

});