/******/ (function(modules) { // webpackBootstrap
/******/ 	// install a JSONP callback for chunk loading
/******/ 	function webpackJsonpCallback(data) {
/******/ 		var chunkIds = data[0];
/******/ 		var moreModules = data[1];
/******/ 		var executeModules = data[2];
/******/
/******/ 		// add "moreModules" to the modules object,
/******/ 		// then flag all "chunkIds" as loaded and fire callback
/******/ 		var moduleId, chunkId, i = 0, resolves = [];
/******/ 		for(;i < chunkIds.length; i++) {
/******/ 			chunkId = chunkIds[i];
/******/ 			if(installedChunks[chunkId]) {
/******/ 				resolves.push(installedChunks[chunkId][0]);
/******/ 			}
/******/ 			installedChunks[chunkId] = 0;
/******/ 		}
/******/ 		for(moduleId in moreModules) {
/******/ 			if(Object.prototype.hasOwnProperty.call(moreModules, moduleId)) {
/******/ 				modules[moduleId] = moreModules[moduleId];
/******/ 			}
/******/ 		}
/******/ 		if(parentJsonpFunction) parentJsonpFunction(data);
/******/
/******/ 		while(resolves.length) {
/******/ 			resolves.shift()();
/******/ 		}
/******/
/******/ 		// add entry modules from loaded chunk to deferred list
/******/ 		deferredModules.push.apply(deferredModules, executeModules || []);
/******/
/******/ 		// run deferred modules when all chunks ready
/******/ 		return checkDeferredModules();
/******/ 	};
/******/ 	function checkDeferredModules() {
/******/ 		var result;
/******/ 		for(var i = 0; i < deferredModules.length; i++) {
/******/ 			var deferredModule = deferredModules[i];
/******/ 			var fulfilled = true;
/******/ 			for(var j = 1; j < deferredModule.length; j++) {
/******/ 				var depId = deferredModule[j];
/******/ 				if(installedChunks[depId] !== 0) fulfilled = false;
/******/ 			}
/******/ 			if(fulfilled) {
/******/ 				deferredModules.splice(i--, 1);
/******/ 				result = __webpack_require__(__webpack_require__.s = deferredModule[0]);
/******/ 			}
/******/ 		}
/******/ 		return result;
/******/ 	}
/******/
/******/ 	// The module cache
/******/ 	var installedModules = {};
/******/
/******/ 	// object to store loaded and loading chunks
/******/ 	// undefined = chunk not loaded, null = chunk preloaded/prefetched
/******/ 	// Promise = chunk loading, 0 = chunk loaded
/******/ 	var installedChunks = {
/******/ 		0: 0
/******/ 	};
/******/
/******/ 	var deferredModules = [];
/******/
/******/ 	// The require function
/******/ 	function __webpack_require__(moduleId) {
/******/
/******/ 		// Check if module is in cache
/******/ 		if(installedModules[moduleId]) {
/******/ 			return installedModules[moduleId].exports;
/******/ 		}
/******/ 		// Create a new module (and put it into the cache)
/******/ 		var module = installedModules[moduleId] = {
/******/ 			i: moduleId,
/******/ 			l: false,
/******/ 			exports: {}
/******/ 		};
/******/
/******/ 		// Execute the module function
/******/ 		modules[moduleId].call(module.exports, module, module.exports, __webpack_require__);
/******/
/******/ 		// Flag the module as loaded
/******/ 		module.l = true;
/******/
/******/ 		// Return the exports of the module
/******/ 		return module.exports;
/******/ 	}
/******/
/******/
/******/ 	// expose the modules object (__webpack_modules__)
/******/ 	__webpack_require__.m = modules;
/******/
/******/ 	// expose the module cache
/******/ 	__webpack_require__.c = installedModules;
/******/
/******/ 	// define getter function for harmony exports
/******/ 	__webpack_require__.d = function(exports, name, getter) {
/******/ 		if(!__webpack_require__.o(exports, name)) {
/******/ 			Object.defineProperty(exports, name, { enumerable: true, get: getter });
/******/ 		}
/******/ 	};
/******/
/******/ 	// define __esModule on exports
/******/ 	__webpack_require__.r = function(exports) {
/******/ 		if(typeof Symbol !== 'undefined' && Symbol.toStringTag) {
/******/ 			Object.defineProperty(exports, Symbol.toStringTag, { value: 'Module' });
/******/ 		}
/******/ 		Object.defineProperty(exports, '__esModule', { value: true });
/******/ 	};
/******/
/******/ 	// create a fake namespace object
/******/ 	// mode & 1: value is a module id, require it
/******/ 	// mode & 2: merge all properties of value into the ns
/******/ 	// mode & 4: return value when already ns object
/******/ 	// mode & 8|1: behave like require
/******/ 	__webpack_require__.t = function(value, mode) {
/******/ 		if(mode & 1) value = __webpack_require__(value);
/******/ 		if(mode & 8) return value;
/******/ 		if((mode & 4) && typeof value === 'object' && value && value.__esModule) return value;
/******/ 		var ns = Object.create(null);
/******/ 		__webpack_require__.r(ns);
/******/ 		Object.defineProperty(ns, 'default', { enumerable: true, value: value });
/******/ 		if(mode & 2 && typeof value != 'string') for(var key in value) __webpack_require__.d(ns, key, function(key) { return value[key]; }.bind(null, key));
/******/ 		return ns;
/******/ 	};
/******/
/******/ 	// getDefaultExport function for compatibility with non-harmony modules
/******/ 	__webpack_require__.n = function(module) {
/******/ 		var getter = module && module.__esModule ?
/******/ 			function getDefault() { return module['default']; } :
/******/ 			function getModuleExports() { return module; };
/******/ 		__webpack_require__.d(getter, 'a', getter);
/******/ 		return getter;
/******/ 	};
/******/
/******/ 	// Object.prototype.hasOwnProperty.call
/******/ 	__webpack_require__.o = function(object, property) { return Object.prototype.hasOwnProperty.call(object, property); };
/******/
/******/ 	// __webpack_public_path__
/******/ 	__webpack_require__.p = "/web/app";
/******/
/******/ 	var jsonpArray = window["webpackJsonp"] = window["webpackJsonp"] || [];
/******/ 	var oldJsonpFunction = jsonpArray.push.bind(jsonpArray);
/******/ 	jsonpArray.push = webpackJsonpCallback;
/******/ 	jsonpArray = jsonpArray.slice();
/******/ 	for(var i = 0; i < jsonpArray.length; i++) webpackJsonpCallback(jsonpArray[i]);
/******/ 	var parentJsonpFunction = oldJsonpFunction;
/******/
/******/
/******/ 	// add entry module to deferred list
/******/ 	deferredModules.push([0,1]);
/******/ 	// run deferred modules when ready
/******/ 	return checkDeferredModules();
/******/ })
/************************************************************************/
/******/ ({

/***/ 0:
/***/ (function(module, exports, __webpack_require__) {

module.exports = __webpack_require__("rVcD");


/***/ }),

/***/ "7WIf":
/***/ (function(module, exports, __webpack_require__) {

// extracted by mini-css-extract-plugin

/***/ }),

/***/ "BBHA":
/***/ (function(module, exports, __webpack_require__) {

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
module.exports.getters = __webpack_require__("F+qq");
module.exports.actions = __webpack_require__("VW/0");

/***/ }),

/***/ "CtRu":
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* unused harmony export fetchStoredSession */
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "b", function() { return fetchSiteEvents; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "a", function() { return fetchActiveSessions; });
/* harmony import */ var app_reactor__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__("xSHT");
/* harmony import */ var app_services_api__WEBPACK_IMPORTED_MODULE_1__ = __webpack_require__("Z9Rw");
/* harmony import */ var app_config__WEBPACK_IMPORTED_MODULE_2__ = __webpack_require__("LMli");
/* harmony import */ var moment__WEBPACK_IMPORTED_MODULE_3__ = __webpack_require__("wd/R");
/* harmony import */ var moment__WEBPACK_IMPORTED_MODULE_3___default = /*#__PURE__*/__webpack_require__.n(moment__WEBPACK_IMPORTED_MODULE_3__);
/* harmony import */ var app_flux_app_getters__WEBPACK_IMPORTED_MODULE_4__ = __webpack_require__("i3tb");
/* harmony import */ var app_lib_logger__WEBPACK_IMPORTED_MODULE_5__ = __webpack_require__("lZJN");
/* harmony import */ var _actionTypes__WEBPACK_IMPORTED_MODULE_6__ = __webpack_require__("zMbK");
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









var logger = app_lib_logger__WEBPACK_IMPORTED_MODULE_5__[/* default */ "a"].create('Modules/Sessions');

function fetchStoredSession(sid, siteId) {
  siteId = siteId || app_reactor__WEBPACK_IMPORTED_MODULE_0__[/* default */ "a"].evaluate(app_flux_app_getters__WEBPACK_IMPORTED_MODULE_4__[/* default */ "a"].siteId);
  return app_services_api__WEBPACK_IMPORTED_MODULE_1__[/* default */ "a"].get(app_config__WEBPACK_IMPORTED_MODULE_2__[/* default */ "a"].api.getSessionEventsUrl({ siteId: siteId, sid: sid })).then(function (json) {
    if (json && json.events) {
      app_reactor__WEBPACK_IMPORTED_MODULE_0__[/* default */ "a"].dispatch(_actionTypes__WEBPACK_IMPORTED_MODULE_6__[/* RECEIVE_SITE_EVENTS */ "b"], { siteId: siteId, json: json.events });
    }
  });
}

function fetchSiteEvents(start, end) {
  // default values
  start = start || moment__WEBPACK_IMPORTED_MODULE_3___default()(new Date()).endOf('day').toDate();
  end = end || moment__WEBPACK_IMPORTED_MODULE_3___default()(end).subtract(3, 'day').startOf('day').toDate();

  start = start.toISOString();
  end = end.toISOString();

  var siteId = app_reactor__WEBPACK_IMPORTED_MODULE_0__[/* default */ "a"].evaluate(app_flux_app_getters__WEBPACK_IMPORTED_MODULE_4__[/* default */ "a"].siteId);
  return app_services_api__WEBPACK_IMPORTED_MODULE_1__[/* default */ "a"].get(app_config__WEBPACK_IMPORTED_MODULE_2__[/* default */ "a"].api.getSiteEventsFilterUrl({ start: start, end: end, siteId: siteId })).done(function (json) {
    if (json && json.events) {
      app_reactor__WEBPACK_IMPORTED_MODULE_0__[/* default */ "a"].dispatch(_actionTypes__WEBPACK_IMPORTED_MODULE_6__[/* RECEIVE_SITE_EVENTS */ "b"], { siteId: siteId, json: json.events });
    }
  }).fail(function (err) {
    logger.error('fetchSiteEvents', err);
  });
}

function fetchActiveSessions() {
  var siteId = app_reactor__WEBPACK_IMPORTED_MODULE_0__[/* default */ "a"].evaluate(app_flux_app_getters__WEBPACK_IMPORTED_MODULE_4__[/* default */ "a"].siteId);
  return app_services_api__WEBPACK_IMPORTED_MODULE_1__[/* default */ "a"].get(app_config__WEBPACK_IMPORTED_MODULE_2__[/* default */ "a"].api.getFetchSessionsUrl(siteId)).done(function (json) {
    var sessions = json.sessions || [];
    app_reactor__WEBPACK_IMPORTED_MODULE_0__[/* default */ "a"].dispatch(_actionTypes__WEBPACK_IMPORTED_MODULE_6__[/* RECEIVE_ACTIVE_SESSIONS */ "a"], { siteId: siteId, json: sessions });
  }).fail(function (err) {
    logger.error('fetchActiveSessions', err);
  });
}

/***/ }),

/***/ "F+qq":
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
__webpack_require__.r(__webpack_exports__);
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

var filter = [['tlpt_sessions_filter'], function (filter) {
  return filter.toJS();
}];

/* harmony default export */ __webpack_exports__["default"] = ({
  filter: filter
});

/***/ }),

/***/ "KdfW":
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "a", function() { return KeysEnum; });
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

var KeysEnum = {
  TOKEN: 'grv_teleport_token',
  TOKEN_RENEW: 'grv_teleport_token_renew'
};

var storage = {
  clear: function clear() {
    window.localStorage.clear();
  },
  subscribe: function subscribe(fn) {
    window.addEventListener('storage', fn);
  },
  unsubscribe: function unsubscribe(fn) {
    window.removeEventListener('storage', fn);
  },
  setBearerToken: function setBearerToken(token) {
    window.localStorage.setItem(KeysEnum.TOKEN, JSON.stringify(token));
  },
  getBearerToken: function getBearerToken() {
    var item = window.localStorage.getItem(KeysEnum.TOKEN);
    if (item) {
      return JSON.parse(item);
    }

    return null;
  },
  broadcast: function broadcast(messageType, messageBody) {
    window.localStorage.setItem(messageType, messageBody);
    window.localStorage.removeItem(messageType);
  }
};

/* harmony default export */ __webpack_exports__["b"] = (storage);

/***/ }),

/***/ "LMli":
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var app_lib_patternUtils__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__("cIpc");
/* harmony import */ var jQuery__WEBPACK_IMPORTED_MODULE_1__ = __webpack_require__("K5fH");
/* harmony import */ var jQuery__WEBPACK_IMPORTED_MODULE_1___default = /*#__PURE__*/__webpack_require__.n(jQuery__WEBPACK_IMPORTED_MODULE_1__);
/* harmony import */ var _services_utils__WEBPACK_IMPORTED_MODULE_2__ = __webpack_require__("gOk0");
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





var baseUrl = Object(_services_utils__WEBPACK_IMPORTED_MODULE_2__[/* isTestEnv */ "a"])() ? 'localhost' : window.location.origin;

var cfg = {

  baseUrl: baseUrl,

  helpUrl: 'https://gravitational.com/teleport/docs/quickstart/',

  maxSessionLoadSize: 50,

  displayDateFormat: 'MM/DD/YYYY HH:mm:ss',

  auth: {},

  canJoinSessions: true,

  routes: {
    app: '/web',
    login: '/web/login',
    nodes: '/web/nodes',
    currentSession: '/web/cluster/:siteId/sessions/:sid',
    sessions: '/web/sessions',
    newUser: '/web/newuser/:inviteToken',
    error: '/web/msg/error(/:type)',
    info: '/web/msg/info(/:type)',
    pageNotFound: '/web/notfound',
    terminal: '/web/cluster/:siteId/node/:serverId/:login(/:sid)',
    player: '/web/player/cluster/:siteId/sid/:sid',
    webApi: '/v1/webapi/*',
    settingsBase: '/web/settings',
    settingsAccount: '/web/settings/account'
  },

  api: {
    scp: '/v1/webapi/sites/:siteId/nodes/:serverId/:login/scp?location=:location&filename=:filename',
    ssoOidc: '/v1/webapi/oidc/login/web?redirect_url=:redirect&connector_id=:providerName',
    ssoSaml: '/v1/webapi/saml/sso?redirect_url=:redirect&connector_id=:providerName',
    renewTokenPath: '/v1/webapi/sessions/renew',
    sessionPath: '/v1/webapi/sessions',
    userContextPath: '/v1/webapi/user/context',
    userStatusPath: '/v1/webapi/user/status',
    invitePath: '/v1/webapi/users/invites/:inviteToken',
    createUserPath: '/v1/webapi/users',
    changeUserPasswordPath: '/v1/webapi/users/password',
    u2fCreateUserChallengePath: '/v1/webapi/u2f/signuptokens/:inviteToken',
    u2fCreateUserPath: '/v1/webapi/u2f/users',
    u2fSessionChallengePath: '/v1/webapi/u2f/signrequest',
    u2fChangePassChallengePath: '/v1/webapi/u2f/password/changerequest',
    u2fChangePassPath: '/v1/webapi/u2f/password',
    u2fSessionPath: '/v1/webapi/u2f/sessions',
    sitesBasePath: '/v1/webapi/sites',
    sitePath: '/v1/webapi/sites/:siteId',
    nodesPath: '/v1/webapi/sites/:siteId/nodes',
    siteSessionPath: '/v1/webapi/sites/:siteId/sessions',
    sessionEventsPath: '/v1/webapi/sites/:siteId/sessions/:sid/events',
    siteEventSessionFilterPath: '/v1/webapi/sites/:siteId/sessions',
    siteEventsFilterPath: '/v1/webapi/sites/:siteId/events?event=session.start&event=session.end&from=:start&to=:end',
    ttyWsAddr: ':fqdm/v1/webapi/sites/:cluster/connect?access_token=:token&params=:params',

    getSiteUrl: function getSiteUrl(siteId) {
      return Object(app_lib_patternUtils__WEBPACK_IMPORTED_MODULE_0__[/* formatPattern */ "a"])(cfg.api.sitePath, { siteId: siteId });
    },
    getSiteNodesUrl: function getSiteNodesUrl() {
      var siteId = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : '-current-';

      return Object(app_lib_patternUtils__WEBPACK_IMPORTED_MODULE_0__[/* formatPattern */ "a"])(cfg.api.nodesPath, { siteId: siteId });
    },
    getSiteSessionUrl: function getSiteSessionUrl() {
      var siteId = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : '-current-';

      return Object(app_lib_patternUtils__WEBPACK_IMPORTED_MODULE_0__[/* formatPattern */ "a"])(cfg.api.siteSessionPath, { siteId: siteId });
    },
    getSsoUrl: function getSsoUrl(providerUrl, providerName, redirect) {
      return cfg.baseUrl + Object(app_lib_patternUtils__WEBPACK_IMPORTED_MODULE_0__[/* formatPattern */ "a"])(providerUrl, { redirect: redirect, providerName: providerName });
    },
    getSiteEventsFilterUrl: function getSiteEventsFilterUrl(_ref) {
      var start = _ref.start,
          end = _ref.end,
          siteId = _ref.siteId;

      return Object(app_lib_patternUtils__WEBPACK_IMPORTED_MODULE_0__[/* formatPattern */ "a"])(cfg.api.siteEventsFilterPath, { start: start, end: end, siteId: siteId });
    },
    getSessionEventsUrl: function getSessionEventsUrl(_ref2) {
      var sid = _ref2.sid,
          siteId = _ref2.siteId;

      return Object(app_lib_patternUtils__WEBPACK_IMPORTED_MODULE_0__[/* formatPattern */ "a"])(cfg.api.sessionEventsPath, { sid: sid, siteId: siteId });
    },
    getScpUrl: function getScpUrl(_ref3) {
      var siteId = _ref3.siteId,
          serverId = _ref3.serverId,
          login = _ref3.login,
          location = _ref3.location,
          filename = _ref3.filename;

      return Object(app_lib_patternUtils__WEBPACK_IMPORTED_MODULE_0__[/* formatPattern */ "a"])(cfg.api.scp, { siteId: siteId, serverId: serverId, login: login, location: location, filename: filename });
    },
    getFetchSessionsUrl: function getFetchSessionsUrl(siteId) {
      return Object(app_lib_patternUtils__WEBPACK_IMPORTED_MODULE_0__[/* formatPattern */ "a"])(cfg.api.siteEventSessionFilterPath, { siteId: siteId });
    },
    getFetchSessionUrl: function getFetchSessionUrl(_ref4) {
      var sid = _ref4.sid,
          siteId = _ref4.siteId;

      return Object(app_lib_patternUtils__WEBPACK_IMPORTED_MODULE_0__[/* formatPattern */ "a"])(cfg.api.siteSessionPath + '/:sid', { sid: sid, siteId: siteId });
    },
    getInviteUrl: function getInviteUrl(inviteToken) {
      return Object(app_lib_patternUtils__WEBPACK_IMPORTED_MODULE_0__[/* formatPattern */ "a"])(cfg.api.invitePath, { inviteToken: inviteToken });
    },
    getU2fCreateUserChallengeUrl: function getU2fCreateUserChallengeUrl(inviteToken) {
      return Object(app_lib_patternUtils__WEBPACK_IMPORTED_MODULE_0__[/* formatPattern */ "a"])(cfg.api.u2fCreateUserChallengePath, { inviteToken: inviteToken });
    }
  },

  getPlayerUrl: function getPlayerUrl(_ref5) {
    var siteId = _ref5.siteId,
        sid = _ref5.sid;

    return Object(app_lib_patternUtils__WEBPACK_IMPORTED_MODULE_0__[/* formatPattern */ "a"])(cfg.routes.player, { siteId: siteId, sid: sid });
  },
  getTerminalLoginUrl: function getTerminalLoginUrl(_ref6) {
    var siteId = _ref6.siteId,
        serverId = _ref6.serverId,
        login = _ref6.login,
        sid = _ref6.sid;

    if (!sid) {
      var url = this.stripOptionalParams(cfg.routes.terminal);
      return Object(app_lib_patternUtils__WEBPACK_IMPORTED_MODULE_0__[/* formatPattern */ "a"])(url, { siteId: siteId, serverId: serverId, login: login });
    }

    return Object(app_lib_patternUtils__WEBPACK_IMPORTED_MODULE_0__[/* formatPattern */ "a"])(cfg.routes.terminal, { siteId: siteId, serverId: serverId, login: login, sid: sid });
  },
  getCurrentSessionRouteUrl: function getCurrentSessionRouteUrl(_ref7) {
    var sid = _ref7.sid,
        siteId = _ref7.siteId;

    return Object(app_lib_patternUtils__WEBPACK_IMPORTED_MODULE_0__[/* formatPattern */ "a"])(cfg.routes.currentSession, { sid: sid, siteId: siteId });
  },
  getAuthProviders: function getAuthProviders() {
    return cfg.auth && cfg.auth.providers ? cfg.auth.providers : [];
  },
  getAuth2faType: function getAuth2faType() {
    return cfg.auth ? cfg.auth.second_factor : null;
  },
  getU2fAppId: function getU2fAppId() {
    return cfg.auth && cfg.auth.u2f ? cfg.auth.u2f.app_id : null;
  },
  getWsHostName: function getWsHostName() {
    var prefix = location.protocol === 'https:' ? 'wss://' : 'ws://';
    var hostport = location.hostname + (location.port ? ':' + location.port : '');
    return '' + prefix + hostport;
  },
  init: function init() {
    var config = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : {};

    jQuery__WEBPACK_IMPORTED_MODULE_1___default.a.extend(true, this, config);
  },
  stripOptionalParams: function stripOptionalParams(pattern) {
    return pattern.replace(/\(.*\)/, '');
  }
};

/* harmony default export */ __webpack_exports__["a"] = (cfg);

/***/ }),

/***/ "LYgY":
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "f", function() { return TRYING_TO_SIGN_UP; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "e", function() { return TRYING_TO_LOGIN; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "a", function() { return FETCHING_INVITE; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "c", function() { return TRYING_TO_INIT_APP; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "d", function() { return TRYING_TO_INIT_SETTINGS; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "b", function() { return TRYING_TO_CHANGE_PSW; });
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

var TRYING_TO_SIGN_UP = 'TRYING_TO_SIGN_UP';
var TRYING_TO_LOGIN = 'TRYING_TO_LOGIN';
var FETCHING_INVITE = 'FETCHING_INVITE';
var TRYING_TO_INIT_APP = 'TRYING_TO_INIT_APP';
var TRYING_TO_INIT_SETTINGS = 'TRYING_TO_INIT_SETTINGS';
var TRYING_TO_CHANGE_PSW = 'TRYING_TO_CHANGE_PSW';

/***/ }),

/***/ "OcN7":
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "a", function() { return TLPT_STORED_SESSINS_FILTER_SET_RANGE; });
/* unused harmony export TLPT_STORED_SESSINS_FILTER_SET_STATUS */
/* unused harmony export TLPT_STORED_SESSINS_FILTER_RECEIVE_MORE */
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

var TLPT_STORED_SESSINS_FILTER_SET_RANGE = 'TLPT_STORED_SESSINS_FILTER_SET_RANGE';
var TLPT_STORED_SESSINS_FILTER_SET_STATUS = 'TLPT_STORED_SESSINS_FILTER_SET_STATUS';
var TLPT_STORED_SESSINS_FILTER_RECEIVE_MORE = 'TLPT_STORED_SESSINS_FILTER_RECEIVE_MORE';

/***/ }),

/***/ "PVWJ":
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "a", function() { return TrackRec; });
/* harmony import */ var nuclear_js__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__("L7e8");
/* harmony import */ var nuclear_js__WEBPACK_IMPORTED_MODULE_0___default = /*#__PURE__*/__webpack_require__.n(nuclear_js__WEBPACK_IMPORTED_MODULE_0__);
/* harmony import */ var _actionTypes__WEBPACK_IMPORTED_MODULE_1__ = __webpack_require__("tGXY");
/* harmony import */ var immutable__WEBPACK_IMPORTED_MODULE_2__ = __webpack_require__("JPcv");
/* harmony import */ var immutable__WEBPACK_IMPORTED_MODULE_2___default = /*#__PURE__*/__webpack_require__.n(immutable__WEBPACK_IMPORTED_MODULE_2__);
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





var TrackRec = new immutable__WEBPACK_IMPORTED_MODULE_2__["Record"]({
  isProcessing: false,
  isFailed: false,
  isSuccess: false,
  message: ''
});

/* harmony default export */ __webpack_exports__["b"] = (Object(nuclear_js__WEBPACK_IMPORTED_MODULE_0__["Store"])({
  getInitialState: function getInitialState() {
    return Object(nuclear_js__WEBPACK_IMPORTED_MODULE_0__["toImmutable"])({});
  },
  initialize: function initialize() {
    this.on(_actionTypes__WEBPACK_IMPORTED_MODULE_1__[/* START */ "c"], start);
    this.on(_actionTypes__WEBPACK_IMPORTED_MODULE_1__[/* FAIL */ "b"], fail);
    this.on(_actionTypes__WEBPACK_IMPORTED_MODULE_1__[/* SUCCESS */ "d"], success);
    this.on(_actionTypes__WEBPACK_IMPORTED_MODULE_1__[/* CLEAR */ "a"], clear);
  }
}));

function start(state, request) {
  return state.set(request.type, new TrackRec({ isProcessing: true }));
}

function fail(state, request) {
  return state.set(request.type, new TrackRec({ isFailed: true, message: request.message }));
}

function success(state, request) {
  return state.set(request.type, new TrackRec({ isSuccess: true, message: request.message }));
}

function clear(state, request) {
  return state.set(request.type, new TrackRec());
}

/***/ }),

/***/ "RnhZ":
/***/ (function(module, exports, __webpack_require__) {

var map = {
	"./af": "K/tc",
	"./af.js": "K/tc",
	"./ar": "jnO4",
	"./ar-dz": "o1bE",
	"./ar-dz.js": "o1bE",
	"./ar-kw": "Qj4J",
	"./ar-kw.js": "Qj4J",
	"./ar-ly": "HP3h",
	"./ar-ly.js": "HP3h",
	"./ar-ma": "CoRJ",
	"./ar-ma.js": "CoRJ",
	"./ar-sa": "gjCT",
	"./ar-sa.js": "gjCT",
	"./ar-tn": "bYM6",
	"./ar-tn.js": "bYM6",
	"./ar.js": "jnO4",
	"./az": "SFxW",
	"./az.js": "SFxW",
	"./be": "H8ED",
	"./be.js": "H8ED",
	"./bg": "hKrs",
	"./bg.js": "hKrs",
	"./bm": "p/rL",
	"./bm.js": "p/rL",
	"./bn": "kEOa",
	"./bn.js": "kEOa",
	"./bo": "0mo+",
	"./bo.js": "0mo+",
	"./br": "aIdf",
	"./br.js": "aIdf",
	"./bs": "JVSJ",
	"./bs.js": "JVSJ",
	"./ca": "1xZ4",
	"./ca.js": "1xZ4",
	"./cs": "PA2r",
	"./cs.js": "PA2r",
	"./cv": "A+xa",
	"./cv.js": "A+xa",
	"./cy": "l5ep",
	"./cy.js": "l5ep",
	"./da": "DxQv",
	"./da.js": "DxQv",
	"./de": "tGlX",
	"./de-at": "s+uk",
	"./de-at.js": "s+uk",
	"./de-ch": "u3GI",
	"./de-ch.js": "u3GI",
	"./de.js": "tGlX",
	"./dv": "WYrj",
	"./dv.js": "WYrj",
	"./el": "jUeY",
	"./el.js": "jUeY",
	"./en-au": "Dmvi",
	"./en-au.js": "Dmvi",
	"./en-ca": "OIYi",
	"./en-ca.js": "OIYi",
	"./en-gb": "Oaa7",
	"./en-gb.js": "Oaa7",
	"./en-ie": "4dOw",
	"./en-ie.js": "4dOw",
	"./en-nz": "b1Dy",
	"./en-nz.js": "b1Dy",
	"./eo": "Zduo",
	"./eo.js": "Zduo",
	"./es": "iYuL",
	"./es-do": "CjzT",
	"./es-do.js": "CjzT",
	"./es-us": "Vclq",
	"./es-us.js": "Vclq",
	"./es.js": "iYuL",
	"./et": "7BjC",
	"./et.js": "7BjC",
	"./eu": "D/JM",
	"./eu.js": "D/JM",
	"./fa": "jfSC",
	"./fa.js": "jfSC",
	"./fi": "gekB",
	"./fi.js": "gekB",
	"./fo": "ByF4",
	"./fo.js": "ByF4",
	"./fr": "nyYc",
	"./fr-ca": "2fjn",
	"./fr-ca.js": "2fjn",
	"./fr-ch": "Dkky",
	"./fr-ch.js": "Dkky",
	"./fr.js": "nyYc",
	"./fy": "cRix",
	"./fy.js": "cRix",
	"./gd": "9rRi",
	"./gd.js": "9rRi",
	"./gl": "iEDd",
	"./gl.js": "iEDd",
	"./gom-latn": "DKr+",
	"./gom-latn.js": "DKr+",
	"./gu": "4MV3",
	"./gu.js": "4MV3",
	"./he": "x6pH",
	"./he.js": "x6pH",
	"./hi": "3E1r",
	"./hi.js": "3E1r",
	"./hr": "S6ln",
	"./hr.js": "S6ln",
	"./hu": "WxRl",
	"./hu.js": "WxRl",
	"./hy-am": "1rYy",
	"./hy-am.js": "1rYy",
	"./id": "UDhR",
	"./id.js": "UDhR",
	"./is": "BVg3",
	"./is.js": "BVg3",
	"./it": "bpih",
	"./it.js": "bpih",
	"./ja": "B55N",
	"./ja.js": "B55N",
	"./jv": "tUCv",
	"./jv.js": "tUCv",
	"./ka": "IBtZ",
	"./ka.js": "IBtZ",
	"./kk": "bXm7",
	"./kk.js": "bXm7",
	"./km": "6B0Y",
	"./km.js": "6B0Y",
	"./kn": "PpIw",
	"./kn.js": "PpIw",
	"./ko": "Ivi+",
	"./ko.js": "Ivi+",
	"./ky": "lgnt",
	"./ky.js": "lgnt",
	"./lb": "RAwQ",
	"./lb.js": "RAwQ",
	"./lo": "sp3z",
	"./lo.js": "sp3z",
	"./lt": "JvlW",
	"./lt.js": "JvlW",
	"./lv": "uXwI",
	"./lv.js": "uXwI",
	"./me": "KTz0",
	"./me.js": "KTz0",
	"./mi": "aIsn",
	"./mi.js": "aIsn",
	"./mk": "aQkU",
	"./mk.js": "aQkU",
	"./ml": "AvvY",
	"./ml.js": "AvvY",
	"./mr": "Ob0Z",
	"./mr.js": "Ob0Z",
	"./ms": "6+QB",
	"./ms-my": "ZAMP",
	"./ms-my.js": "ZAMP",
	"./ms.js": "6+QB",
	"./my": "honF",
	"./my.js": "honF",
	"./nb": "bOMt",
	"./nb.js": "bOMt",
	"./ne": "OjkT",
	"./ne.js": "OjkT",
	"./nl": "+s0g",
	"./nl-be": "2ykv",
	"./nl-be.js": "2ykv",
	"./nl.js": "+s0g",
	"./nn": "uEye",
	"./nn.js": "uEye",
	"./pa-in": "8/+R",
	"./pa-in.js": "8/+R",
	"./pl": "jVdC",
	"./pl.js": "jVdC",
	"./pt": "8mBD",
	"./pt-br": "0tRk",
	"./pt-br.js": "0tRk",
	"./pt.js": "8mBD",
	"./ro": "lyxo",
	"./ro.js": "lyxo",
	"./ru": "lXzo",
	"./ru.js": "lXzo",
	"./sd": "Z4QM",
	"./sd.js": "Z4QM",
	"./se": "//9w",
	"./se.js": "//9w",
	"./si": "7aV9",
	"./si.js": "7aV9",
	"./sk": "e+ae",
	"./sk.js": "e+ae",
	"./sl": "gVVK",
	"./sl.js": "gVVK",
	"./sq": "yPMs",
	"./sq.js": "yPMs",
	"./sr": "zx6S",
	"./sr-cyrl": "E+lV",
	"./sr-cyrl.js": "E+lV",
	"./sr.js": "zx6S",
	"./ss": "Ur1D",
	"./ss.js": "Ur1D",
	"./sv": "X709",
	"./sv.js": "X709",
	"./sw": "dNwA",
	"./sw.js": "dNwA",
	"./ta": "PeUW",
	"./ta.js": "PeUW",
	"./te": "XLvN",
	"./te.js": "XLvN",
	"./tet": "V2x9",
	"./tet.js": "V2x9",
	"./th": "EOgW",
	"./th.js": "EOgW",
	"./tl-ph": "Dzi0",
	"./tl-ph.js": "Dzi0",
	"./tlh": "z3Vd",
	"./tlh.js": "z3Vd",
	"./tr": "DoHr",
	"./tr.js": "DoHr",
	"./tzl": "z1FC",
	"./tzl.js": "z1FC",
	"./tzm": "wQk9",
	"./tzm-latn": "tT3J",
	"./tzm-latn.js": "tT3J",
	"./tzm.js": "wQk9",
	"./uk": "raLr",
	"./uk.js": "raLr",
	"./ur": "UpQW",
	"./ur.js": "UpQW",
	"./uz": "Loxo",
	"./uz-latn": "AQ68",
	"./uz-latn.js": "AQ68",
	"./uz.js": "Loxo",
	"./vi": "KSF8",
	"./vi.js": "KSF8",
	"./x-pseudo": "/X5v",
	"./x-pseudo.js": "/X5v",
	"./yo": "fzPg",
	"./yo.js": "fzPg",
	"./zh-cn": "XDpg",
	"./zh-cn.js": "XDpg",
	"./zh-hk": "SatO",
	"./zh-hk.js": "SatO",
	"./zh-tw": "kOpN",
	"./zh-tw.js": "kOpN"
};


function webpackContext(req) {
	var id = webpackContextResolve(req);
	return __webpack_require__(id);
}
function webpackContextResolve(req) {
	var id = map[req];
	if(!(id + 1)) { // check for number or string
		var e = new Error("Cannot find module '" + req + "'");
		e.code = 'MODULE_NOT_FOUND';
		throw e;
	}
	return id;
}
webpackContext.keys = function webpackContextKeys() {
	return Object.keys(map);
};
webpackContext.resolve = webpackContextResolve;
module.exports = webpackContext;
webpackContext.id = "RnhZ";

/***/ }),

/***/ "VW/0":
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
__webpack_require__.r(__webpack_exports__);
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "fetchSiteEventsWithinTimeRange", function() { return fetchSiteEventsWithinTimeRange; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "setTimeRange", function() { return setTimeRange; });
/* harmony import */ var app_reactor__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__("xSHT");
/* harmony import */ var _getters__WEBPACK_IMPORTED_MODULE_1__ = __webpack_require__("F+qq");
/* harmony import */ var _sessions_actions__WEBPACK_IMPORTED_MODULE_2__ = __webpack_require__("CtRu");
/* harmony import */ var _actionTypes__WEBPACK_IMPORTED_MODULE_3__ = __webpack_require__("OcN7");
/* harmony import */ var app_lib_logger__WEBPACK_IMPORTED_MODULE_4__ = __webpack_require__("lZJN");
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






var logger = app_lib_logger__WEBPACK_IMPORTED_MODULE_4__[/* default */ "a"].create('Modules/Sessions');

function fetchSiteEventsWithinTimeRange() {
  var _reactor$evaluate = app_reactor__WEBPACK_IMPORTED_MODULE_0__[/* default */ "a"].evaluate(_getters__WEBPACK_IMPORTED_MODULE_1__["default"].filter),
      start = _reactor$evaluate.start,
      end = _reactor$evaluate.end;

  return _fetch(start, end);
}

function setTimeRange(start, end) {
  app_reactor__WEBPACK_IMPORTED_MODULE_0__[/* default */ "a"].batch(function () {
    app_reactor__WEBPACK_IMPORTED_MODULE_0__[/* default */ "a"].dispatch(_actionTypes__WEBPACK_IMPORTED_MODULE_3__[/* TLPT_STORED_SESSINS_FILTER_SET_RANGE */ "a"], { start: start, end: end });
    _fetch(start, end);
  });
}

function _fetch(start, end) {
  return Object(_sessions_actions__WEBPACK_IMPORTED_MODULE_2__[/* fetchSiteEvents */ "b"])(start, end).fail(function (err) {
    logger.error('fetching filtered set of sessions', err);
  });
}

/***/ }),

/***/ "Z9Rw":
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var jQuery__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__("K5fH");
/* harmony import */ var jQuery__WEBPACK_IMPORTED_MODULE_0___default = /*#__PURE__*/__webpack_require__.n(jQuery__WEBPACK_IMPORTED_MODULE_0__);
/* harmony import */ var _localStorage__WEBPACK_IMPORTED_MODULE_1__ = __webpack_require__("KdfW");
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




var api = {
  put: function put(path, data) {
    return api.ajax({ url: path, data: JSON.stringify(data), type: 'PUT' });
  },
  post: function post(path, data) {
    return api.ajax({ url: path, data: JSON.stringify(data), type: 'POST' });
  },
  delete: function _delete(path, data) {
    return api.ajax({ url: path, data: JSON.stringify(data), type: 'DELETE' });
  },
  get: function get(path) {
    return api.ajax({ url: path });
  },
  ajax: function ajax(cfg) {
    var _this = this;

    var defaultCfg = {
      cache: false,
      type: 'GET',
      dataType: 'json',
      contentType: 'application/json; charset=utf-8',
      beforeSend: function beforeSend(xhr) {
        return _this.setAuthHeaders(xhr);
      }
    };

    return jQuery__WEBPACK_IMPORTED_MODULE_0___default.a.ajax(jQuery__WEBPACK_IMPORTED_MODULE_0___default.a.extend({}, defaultCfg, cfg));
  },
  getErrorText: function getErrorText(err) {
    var msg = 'Unknown error';

    if (err instanceof Error) {
      return err.message || msg;
    }

    if (err.responseJSON && err.responseJSON.message) {
      return err.responseJSON.message;
    }

    if (err.responseJSON && err.responseJSON.error) {
      return err.responseJSON.error.message || msg;
    }

    if (err.responseText) {
      return err.responseText;
    }

    return msg;
  },
  setAuthHeaders: function setAuthHeaders(xhr) {
    var accessToken = this.getAccessToken();
    var csrfToken = this.getXCSRFToken();
    xhr.setRequestHeader('X-CSRF-Token', csrfToken);
    xhr.setRequestHeader('Authorization', 'Bearer ' + accessToken);
  },
  setNoCacheHeaders: function setNoCacheHeaders(xhr) {
    xhr.setRequestHeader('cache-control', 'max-age=0');
    xhr.setRequestHeader('expires', '0');
    xhr.setRequestHeader('pragma', 'no-cache');
  },
  getAccessToken: function getAccessToken() {
    var bearerToken = _localStorage__WEBPACK_IMPORTED_MODULE_1__[/* default */ "b"].getBearerToken() || {};
    return bearerToken.accessToken;
  },
  getXCSRFToken: function getXCSRFToken() {
    var metaTag = document.querySelector('[name=grv_csrf_token]');
    return metaTag ? metaTag.content : '';
  }
};

/* harmony default export */ __webpack_exports__["a"] = (api);

/***/ }),

/***/ "cIpc":
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* unused harmony export compilePattern */
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "b", function() { return matchPattern; });
/* unused harmony export getParamNames */
/* unused harmony export getParams */
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "a", function() { return formatPattern; });
/* harmony import */ var invariant__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__("QLaP");
/* harmony import */ var invariant__WEBPACK_IMPORTED_MODULE_0___default = /*#__PURE__*/__webpack_require__.n(invariant__WEBPACK_IMPORTED_MODULE_0__);
/*
 *  The MIT License (MIT)
 *  Copyright (c) 2015 Ryan Florence, Michael Jackson
 *  Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:
 *  The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.
 *  THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/



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

      invariant__WEBPACK_IMPORTED_MODULE_0___default()(paramValue != null || parenCount > 0, 'Missing splat #%s for path "%s"', splatIndex, pattern);

      if (paramValue != null) pathname += encodeURI(paramValue);
    } else if (token === '(') {
      parenCount += 1;
    } else if (token === ')') {
      parenCount -= 1;
    } else if (token.charAt(0) === ':') {
      paramName = token.substring(1);
      paramValue = params[paramName];

      invariant__WEBPACK_IMPORTED_MODULE_0___default()(paramValue != null || parenCount > 0, 'Missing "%s" parameter for path "%s"', paramName, pattern);

      if (paramValue != null) pathname += encodeURIComponent(paramValue);
    } else {
      pathname += token;
    }
  }

  return pathname.replace(/\/+/g, '/');
}

/***/ }),

/***/ "gOk0":
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* WEBPACK VAR INJECTION */(function(process) {/* unused harmony export isDevEnv */
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "a", function() { return isTestEnv; });
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

var isDevEnv = function isDevEnv() {
    return "production" === 'development';
};
var isTestEnv = function isTestEnv() {
    return process.env.NODE_ENV_TYPE === 'test';
};
/* WEBPACK VAR INJECTION */}.call(this, __webpack_require__("8oxB")))

/***/ }),

/***/ "i3tb":
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var app_flux_status_getters__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__("oHDm");
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



/* harmony default export */ __webpack_exports__["a"] = ({
  initAttempt: app_flux_status_getters__WEBPACK_IMPORTED_MODULE_0__[/* initAppAttempt */ "c"],
  siteId: ['tlpt', 'siteId']
});

/***/ }),

/***/ "lZJN":
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
var _createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

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

  _createClass(Logger, [{
    key: 'log',
    value: function log() {
      var _window$console;

      var level = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : 'log';

      for (var _len = arguments.length, args = Array(_len > 1 ? _len - 1 : 0), _key = 1; _key < _len; _key++) {
        args[_key - 1] = arguments[_key];
      }

      (_window$console = window.console)[level].apply(_window$console, ['%c[' + this.name + ']', 'color: blue;'].concat(args));
    }
  }, {
    key: 'trace',
    value: function trace() {
      for (var _len2 = arguments.length, args = Array(_len2), _key2 = 0; _key2 < _len2; _key2++) {
        args[_key2] = arguments[_key2];
      }

      this.log.apply(this, ['trace'].concat(args));
    }
  }, {
    key: 'warn',
    value: function warn() {
      for (var _len3 = arguments.length, args = Array(_len3), _key3 = 0; _key3 < _len3; _key3++) {
        args[_key3] = arguments[_key3];
      }

      this.log.apply(this, ['warn'].concat(args));
    }
  }, {
    key: 'info',
    value: function info() {
      for (var _len4 = arguments.length, args = Array(_len4), _key4 = 0; _key4 < _len4; _key4++) {
        args[_key4] = arguments[_key4];
      }

      this.log.apply(this, ['info'].concat(args));
    }
  }, {
    key: 'error',
    value: function error() {
      for (var _len5 = arguments.length, args = Array(_len5), _key5 = 0; _key5 < _len5; _key5++) {
        args[_key5] = arguments[_key5];
      }

      this.log.apply(this, ['error'].concat(args));
    }
  }]);

  return Logger;
}();

/* harmony default export */ __webpack_exports__["a"] = ({
  create: function create() {
    for (var _len6 = arguments.length, args = Array(_len6), _key6 = 0; _key6 < _len6; _key6++) {
      args[_key6] = arguments[_key6];
    }

    return new (Function.prototype.bind.apply(Logger, [null].concat(args)))();
  }
});

/***/ }),

/***/ "oHDm":
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "e", function() { return makeGetter; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "c", function() { return initAppAttempt; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "d", function() { return loginAttempt; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "b", function() { return fetchInviteAttempt; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "f", function() { return signupAttempt; });
/* unused harmony export initSettingsAttempt */
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "a", function() { return changePasswordAttempt; });
/* harmony import */ var _statusStore__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__("PVWJ");
/* harmony import */ var _constants__WEBPACK_IMPORTED_MODULE_1__ = __webpack_require__("LYgY");
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




var STORE_NAME = 'tlpt_status';

var makeGetter = function makeGetter(reqType) {
  return [[STORE_NAME, reqType], function (rec) {
    return rec || new _statusStore__WEBPACK_IMPORTED_MODULE_0__[/* TrackRec */ "a"]();
  }];
};

var initAppAttempt = makeGetter(_constants__WEBPACK_IMPORTED_MODULE_1__[/* TRYING_TO_INIT_APP */ "c"]);
var loginAttempt = makeGetter(_constants__WEBPACK_IMPORTED_MODULE_1__[/* TRYING_TO_LOGIN */ "e"]);
var fetchInviteAttempt = makeGetter(_constants__WEBPACK_IMPORTED_MODULE_1__[/* FETCHING_INVITE */ "a"]);
var signupAttempt = makeGetter(_constants__WEBPACK_IMPORTED_MODULE_1__[/* TRYING_TO_SIGN_UP */ "f"]);
var initSettingsAttempt = makeGetter(_constants__WEBPACK_IMPORTED_MODULE_1__[/* TRYING_TO_INIT_SETTINGS */ "d"]);
var changePasswordAttempt = makeGetter(_constants__WEBPACK_IMPORTED_MODULE_1__[/* TRYING_TO_CHANGE_PSW */ "b"]);

/***/ }),

/***/ "rVcD":
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
__webpack_require__.r(__webpack_exports__);

// EXTERNAL MODULE: ./node_modules/react/react.js
var react = __webpack_require__("sbe7");
var react_default = /*#__PURE__*/__webpack_require__.n(react);

// EXTERNAL MODULE: ./node_modules/react-dom/index.js
var react_dom = __webpack_require__("i8i4");
var react_dom_default = /*#__PURE__*/__webpack_require__.n(react_dom);

// EXTERNAL MODULE: ./node_modules/react-hot-loader/index.js
var react_hot_loader = __webpack_require__("0cfB");

// EXTERNAL MODULE: ./node_modules/react-router/lib/index.js
var lib = __webpack_require__("l2sx");

// EXTERNAL MODULE: ./node_modules/nuclear-js-react-addons/build/index.js
var build = __webpack_require__("sIaI");

// EXTERNAL MODULE: ./src/app/config.js
var config = __webpack_require__("LMli");

// EXTERNAL MODULE: ./src/app/lib/patternUtils.js
var patternUtils = __webpack_require__("cIpc");

// CONCATENATED MODULE: ./src/app/services/history.js
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





var _inst = null;

var history_history = {
  original: function original() {
    return _inst;
  },
  init: function init() {
    var history = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : lib["browserHistory"];

    _inst = history;
  },
  push: function push(route) {
    var withRefresh = arguments.length > 1 && arguments[1] !== undefined ? arguments[1] : false;

    route = this.ensureSafeRoute(route);
    if (withRefresh) {
      this._pageRefresh(route);
    } else {
      _inst.push(route);
    }
  },
  goBack: function goBack(number) {
    this.original().goBack(number);
  },
  goToLogin: function goToLogin(rememberLocation) {
    var url = config["a" /* default */].routes.login;
    if (rememberLocation) {
      var currentLoc = _inst.getCurrentLocation();
      var redirectUrl = _inst.createHref(currentLoc);
      redirectUrl = this.ensureSafeRoute(redirectUrl);
      redirectUrl = this.ensureBaseUrl(redirectUrl);
      url = url + '?redirect_uri=' + redirectUrl;
    }

    this._pageRefresh(url);
  },
  getRedirectParam: function getRedirectParam() {
    var loc = this.original().getCurrentLocation();
    if (loc.query && loc.query.redirect_uri) {
      return loc.query.redirect_uri;
    }

    return '';
  },
  ensureSafeRoute: function ensureSafeRoute(url) {
    url = this._canPush(url) ? url : config["a" /* default */].routes.app;
    return url;
  },
  ensureBaseUrl: function ensureBaseUrl(url) {
    url = url || '';
    if (url.indexOf(config["a" /* default */].baseUrl) !== 0) {
      url = config["a" /* default */].baseUrl + url;
    }

    return url;
  },
  getRoutes: function getRoutes() {
    return Object.getOwnPropertyNames(config["a" /* default */].routes).map(function (p) {
      return config["a" /* default */].routes[p];
    });
  },
  _canPush: function _canPush(route) {
    route = route || '';
    var routes = this.getRoutes();
    if (route.indexOf(config["a" /* default */].baseUrl) === 0) {
      route = route.replace(config["a" /* default */].baseUrl, '');
    }

    return routes.some(history_match(route));
  },
  _pageRefresh: function _pageRefresh(route) {
    window.location.href = this.ensureBaseUrl(route);
  }
};

var history_match = function match(url) {
  return function (route) {
    var _matchPattern = Object(patternUtils["b" /* matchPattern */])(route, url),
        remainingPathname = _matchPattern.remainingPathname;

    return remainingPathname !== null && remainingPathname.length === 0;
  };
};

/* harmony default export */ var services_history = (history_history);
// EXTERNAL MODULE: ./src/app/reactor.js
var app_reactor = __webpack_require__("xSHT");

// EXTERNAL MODULE: ./src/assets/js/jquery.js
var jquery = __webpack_require__("K5fH");
var jquery_default = /*#__PURE__*/__webpack_require__.n(jquery);

// EXTERNAL MODULE: ./src/assets/js/jquery-validate.js
var jquery_validate = __webpack_require__("q1Yt");

// EXTERNAL MODULE: ./src/app/services/api.js
var api = __webpack_require__("Z9Rw");

// EXTERNAL MODULE: ./node_modules/u2f-api-polyfill/u2f-api-polyfill.js
var u2f_api_polyfill = __webpack_require__("KDbw");

// CONCATENATED MODULE: ./src/app/services/auth.js
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





// This puts it in window.u2f


var auth = {
  login: function login(email, password, token) {
    var data = {
      user: email,
      pass: password,
      second_factor_token: token
    };

    return api["a" /* default */].post(config["a" /* default */].api.sessionPath, data, false);
  },
  loginWithU2f: function loginWithU2f(name, password) {
    var data = {
      user: name,
      pass: password
    };

    return api["a" /* default */].post(config["a" /* default */].api.u2fSessionChallengePath, data, false).then(function (data) {
      var deferred = jquery_default.a.Deferred();

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

        api["a" /* default */].post(config["a" /* default */].api.u2fSessionPath, response, false).then(function (data) {
          deferred.resolve(data);
        }).fail(function (data) {
          deferred.reject(data);
        });
      });

      return deferred.promise();
    });
  },
  acceptInvite: function acceptInvite(name, password, token, inviteToken) {
    var data = {
      invite_token: inviteToken,
      pass: password,
      second_factor_token: token,
      user: name
    };

    return api["a" /* default */].post(config["a" /* default */].api.createUserPath, data, false);
  },
  acceptInviteWithU2f: function acceptInviteWithU2f(name, password, inviteToken) {
    return api["a" /* default */].get(config["a" /* default */].api.getU2fCreateUserChallengeUrl(inviteToken)).then(function (data) {
      var deferred = jquery_default.a.Deferred();
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

        api["a" /* default */].post(config["a" /* default */].api.u2fCreateUserPath, response, false).then(function (data) {
          deferred.resolve(data);
        }).fail(function (err) {
          deferred.reject(err);
        });
      });

      return deferred.promise();
    });
  },
  changePassword: function changePassword(oldPass, newPass, token) {
    var data = {
      old_password: window.btoa(oldPass),
      new_password: window.btoa(newPass),
      second_factor_token: token
    };

    return api["a" /* default */].put(config["a" /* default */].api.changeUserPasswordPath, data);
  },
  changePasswordWithU2f: function changePasswordWithU2f(oldPass, newPass) {
    var data = {
      user: name,
      pass: oldPass
    };

    return api["a" /* default */].post(config["a" /* default */].api.u2fChangePassChallengePath, data).then(function (data) {
      var deferred = jquery_default.a.Deferred();

      window.u2f.sign(data.appId, data.challenge, [data], function (res) {
        if (res.errorCode) {
          var err = auth._getU2fErr(res.errorCode);
          deferred.reject(err);
          return;
        }

        var data = {
          new_password: window.btoa(newPass),
          u2f_sign_response: res
        };

        api["a" /* default */].put(config["a" /* default */].api.changeUserPasswordPath, data).then(function (data) {
          deferred.resolve(data);
        }).fail(function (data) {
          deferred.reject(data);
        });
      });

      return deferred.promise();
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

    var message = 'Please check your U2F settings, make sure it is plugged in and you are using the supported browser.\nU2F error: ' + errorMsg;

    return {
      responseJSON: {
        message: message
      }
    };
  }
};

/* harmony default export */ var services_auth = (auth);
// EXTERNAL MODULE: ./src/app/lib/logger.js
var logger = __webpack_require__("lZJN");

// EXTERNAL MODULE: ./src/app/services/localStorage.js
var localStorage = __webpack_require__("KdfW");

// CONCATENATED MODULE: ./src/app/services/session.js
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








var EMPTY_TOKEN_CONTENT_LENGTH = 20;
var TOKEN_CHECKER_INTERVAL = 15 * 1000; //  every 15 sec
var session_logger = logger["a" /* default */].create('services/sessions');

var BearerToken = function BearerToken(json) {
  _classCallCheck(this, BearerToken);

  this.accessToken = json.token;
  this.expiresIn = json.expires_in;
  this.created = new Date().getTime();
};

var sesstionCheckerTimerId = null;

var session_session = {
  logout: function logout() {
    var rememberLocation = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : false;

    api["a" /* default */].delete(config["a" /* default */].api.sessionPath).always(function () {
      services_history.goToLogin(rememberLocation);
    });

    this.clear();
  },
  clear: function clear() {
    this._stopSessionChecker();
    localStorage["b" /* default */].unsubscribe(receiveMessage);
    localStorage["b" /* default */].setBearerToken(null);
    localStorage["b" /* default */].clear();
  },
  ensureSession: function ensureSession() {
    var _this = this;

    var rememberLocation = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : false;

    this._stopSessionChecker();
    this._ensureLocalStorageSubscription();

    var token = this._getBearerToken();
    if (!token) {
      this.logout(rememberLocation);
      return jquery_default.a.Deferred().reject();
    }

    if (this._shouldRenewToken()) {
      return this._renewToken().done(this._startSessionChecker.bind(this)).fail(function () {
        return _this.logout(rememberLocation);
      });
    } else {
      this._startSessionChecker();
      return jquery_default.a.Deferred().resolve(token);
    }
  },
  _getBearerToken: function _getBearerToken() {
    var token = null;
    try {
      token = this._extractBearerTokenFromHtml();
      if (token) {
        localStorage["b" /* default */].setBearerToken(token);
      } else {
        token = localStorage["b" /* default */].getBearerToken();
      }
    } catch (err) {
      session_logger.error('Cannot find bearer token', err);
    }

    return token;
  },
  _extractBearerTokenFromHtml: function _extractBearerTokenFromHtml() {
    var el = document.querySelector('[name=grv_bearer_token]');
    var token = null;
    if (el !== null) {
      var encodedToken = el.content || '';
      if (encodedToken.length > EMPTY_TOKEN_CONTENT_LENGTH) {
        var decoded = window.atob(encodedToken);
        var json = JSON.parse(decoded);
        token = new BearerToken(json);
      }

      // remove initial data from HTML as it will be renewed with a time
      el.parentNode.removeChild(el);
    }

    return token;
  },
  _shouldRenewToken: function _shouldRenewToken() {
    if (this._getIsRenewing()) {
      return false;
    }

    return this._timeLeft() < TOKEN_CHECKER_INTERVAL * 1.5;
  },
  _shouldCheckStatus: function _shouldCheckStatus() {
    if (this._getIsRenewing()) {
      return false;
    }

    /* 
    * double the threshold value for slow connections to avoid 
    * access-denied response due to concurrent renew token request 
    * made from other tab
    */
    return this._timeLeft() > TOKEN_CHECKER_INTERVAL * 2;
  },
  _renewToken: function _renewToken() {
    var _this2 = this;

    this._setAndBroadcastIsRenewing(true);
    return api["a" /* default */].post(config["a" /* default */].api.renewTokenPath).then(this._receiveBearerToken.bind(this)).always(function () {
      _this2._setAndBroadcastIsRenewing(false);
    });
  },
  _receiveBearerToken: function _receiveBearerToken(json) {
    var token = new BearerToken(json);
    localStorage["b" /* default */].setBearerToken(token);
  },
  _fetchStatus: function _fetchStatus() {
    var _this3 = this;

    api["a" /* default */].get(config["a" /* default */].api.userStatusPath).fail(function (err) {
      // indicates that session is no longer valid (caused by server restarts or updates)
      if (err.status == 403) {
        _this3.logout();
      }
    });
  },
  _setAndBroadcastIsRenewing: function _setAndBroadcastIsRenewing(value) {
    this._setIsRenewing(value);
    localStorage["b" /* default */].broadcast(localStorage["a" /* KeysEnum */].TOKEN_RENEW, value);
  },
  _setIsRenewing: function _setIsRenewing(value) {
    this._isRenewing = value;
  },
  _getIsRenewing: function _getIsRenewing() {
    return !!this._isRenewing;
  },
  _timeLeft: function _timeLeft() {
    var token = this._getBearerToken();
    if (!token) {
      return 0;
    }

    var expiresIn = token.expiresIn,
        created = token.created;

    if (!created || !expiresIn) {
      return 0;
    }

    expiresIn = expiresIn * 1000;
    var delta = created + expiresIn - new Date().getTime();
    return delta;
  },


  // detects localStorage changes from other tabs
  _ensureLocalStorageSubscription: function _ensureLocalStorageSubscription() {
    localStorage["b" /* default */].subscribe(receiveMessage);
  },
  _startSessionChecker: function _startSessionChecker() {
    var _this4 = this;

    this._stopSessionChecker();
    sesstionCheckerTimerId = setInterval(function () {
      // calling ensureSession() will again invoke _startSessionChecker              
      _this4.ensureSession();

      // check if server has a valid session in case of server restarts
      if (_this4._shouldCheckStatus()) {
        _this4._fetchStatus();
      }
    }, TOKEN_CHECKER_INTERVAL);
  },
  _stopSessionChecker: function _stopSessionChecker() {
    clearInterval(sesstionCheckerTimerId);
    sesstionCheckerTimerId = null;
  }
};

function receiveMessage(event) {
  var key = event.key,
      newValue = event.newValue;

  // check if local storage has been cleared from another tab

  if (localStorage["b" /* default */].getBearerToken() === null) {
    session_session.logout();
  }

  // renewToken has been invoked from another tab
  if (key === localStorage["a" /* KeysEnum */].TOKEN_RENEW && !!newValue) {
    session_session._setIsRenewing(JSON.parse(newValue));
  }
}

/* harmony default export */ var services_session = (session_session);
// EXTERNAL MODULE: ./src/app/flux/status/actionTypes.js
var actionTypes = __webpack_require__("tGXY");

// EXTERNAL MODULE: ./src/app/flux/status/constants.js
var constants = __webpack_require__("LYgY");

// CONCATENATED MODULE: ./src/app/flux/status/actions.js
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





function makeStatus(reqType) {
  return {
    start: function start() {
      app_reactor["a" /* default */].dispatch(actionTypes["c" /* START */], { type: reqType });
    },
    success: function success(message) {
      app_reactor["a" /* default */].dispatch(actionTypes["d" /* SUCCESS */], { type: reqType, message: message });
    },
    fail: function fail(message) {
      app_reactor["a" /* default */].dispatch(actionTypes["b" /* FAIL */], { type: reqType, message: message });
    },
    clear: function clear() {
      app_reactor["a" /* default */].dispatch(actionTypes["a" /* CLEAR */], { type: reqType });
    }
  };
}

var initAppStatus = makeStatus(constants["c" /* TRYING_TO_INIT_APP */]);
var loginStatus = makeStatus(constants["e" /* TRYING_TO_LOGIN */]);
var fetchInviteStatus = makeStatus(constants["a" /* FETCHING_INVITE */]);
var signupStatus = makeStatus(constants["f" /* TRYING_TO_SIGN_UP */]);
var initSettingsStatus = makeStatus(constants["d" /* TRYING_TO_INIT_SETTINGS */]);
var changePasswordStatus = makeStatus(constants["b" /* TRYING_TO_CHANGE_PSW */]);
// CONCATENATED MODULE: ./src/app/flux/user/actionTypes.js
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

var RECEIVE_USER = 'TLPT_RECEIVE_USER';
var RECEIVE_INVITE = 'TLPT_RECEIVE_USER_INVITE';
// CONCATENATED MODULE: ./src/app/flux/user/actions.js
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











var actions_logger = logger["a" /* default */].create('flux/user/actions');

var actions = {
  fetchInvite: function fetchInvite(inviteToken) {
    var path = config["a" /* default */].api.getInviteUrl(inviteToken);
    fetchInviteStatus.start();
    api["a" /* default */].get(path).done(function (invite) {
      fetchInviteStatus.success();
      app_reactor["a" /* default */].dispatch(RECEIVE_INVITE, invite);
    }).fail(function (err) {
      var msg = api["a" /* default */].getErrorText(err);
      fetchInviteStatus.fail(msg);
    });
  },
  ensureUser: function ensureUser(nextState, replace, cb) {
    services_session.ensureSession(true).done(function () {
      cb();
    });
  },
  acceptInvite: function acceptInvite(name, psw, token, inviteToken) {
    var promise = services_auth.acceptInvite(name, psw, token, inviteToken);
    actions._handleAcceptInvitePromise(promise);
  },
  acceptInviteWithU2f: function acceptInviteWithU2f(name, psw, inviteToken) {
    var promise = services_auth.acceptInviteWithU2f(name, psw, inviteToken);
    return actions._handleAcceptInvitePromise(promise);
  },
  loginWithSso: function loginWithSso(providerName, providerUrl) {
    var entryUrl = this._getEntryRoute();
    services_history.push(config["a" /* default */].api.getSsoUrl(providerUrl, providerName, entryUrl), true);
  },
  loginWithU2f: function loginWithU2f(user, password) {
    var promise = services_auth.loginWithU2f(user, password);
    actions._handleLoginPromise(promise);
  },
  login: function login(user, password, token) {
    var promise = services_auth.login(user, password, token);
    actions._handleLoginPromise(promise);
  },
  logout: function logout() {
    services_session.logout();
  },
  _handleAcceptInvitePromise: function _handleAcceptInvitePromise(promise) {
    signupStatus.start();
    return promise.done(function () {
      services_history.push(config["a" /* default */].routes.app, true);
    }).fail(function (err) {
      var msg = api["a" /* default */].getErrorText(err);
      actions_logger.error('accept invite', err);
      signupStatus.fail(msg);
    });
  },
  _handleLoginPromise: function _handleLoginPromise(promise) {
    var _this = this;

    loginStatus.start();
    promise.done(function () {
      var url = _this._getEntryRoute();
      services_history.push(url, true);
    }).fail(function (err) {
      var msg = api["a" /* default */].getErrorText(err);
      actions_logger.error('login', err);
      loginStatus.fail(msg);
    });
  },
  _getEntryRoute: function _getEntryRoute() {
    var entryUrl = services_history.getRedirectParam();
    if (entryUrl) {
      entryUrl = services_history.ensureSafeRoute(entryUrl);
    } else {
      entryUrl = config["a" /* default */].routes.app;
    }

    return services_history.ensureBaseUrl(entryUrl);
  }
};

/* harmony default export */ var user_actions = (actions);
// EXTERNAL MODULE: ./src/app/flux/status/getters.js
var getters = __webpack_require__("oHDm");

// CONCATENATED MODULE: ./src/app/flux/user/index.js
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




var STORE_NAME = 'tlpt_user';

function getUser() {
  return app_reactor["a" /* default */].evaluate([STORE_NAME]);
}

var user_invite = [['tlpt_user_invite'], function (invite) {
  return invite;
}];
var userName = [STORE_NAME, 'name'];

var user_getters = {
  userName: userName,
  invite: user_invite,
  pswChangeAttempt: getters["a" /* changePasswordAttempt */],
  loginAttemp: getters["d" /* loginAttempt */],
  attemp: getters["f" /* signupAttempt */],
  fetchingInvite: getters["b" /* fetchInviteAttempt */]
};
// CONCATENATED MODULE: ./src/app/components/user/googleAuthLogo.jsx
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



var googleAuthLogo_GoogleAuthInfo = function GoogleAuthInfo() {
  return react_default.a.createElement(
    "div",
    { className: "grv-google-auth text-left" },
    react_default.a.createElement("div", { className: "grv-icon-google-auth" }),
    react_default.a.createElement(
      "strong",
      null,
      "Google Authenticator"
    ),
    react_default.a.createElement(
      "div",
      null,
      "Download",
      react_default.a.createElement(
        "a",
        { href: "https://support.google.com/accounts/answer/1066447?hl=en" },
        react_default.a.createElement(
          "span",
          null,
          " Google Authenticator "
        )
      ),
      "on your phone to access your two factor token"
    )
  );
};

/* harmony default export */ var googleAuthLogo = (googleAuthLogo_GoogleAuthInfo);
// EXTERNAL MODULE: ./node_modules/classnames/index.js
var classnames = __webpack_require__("TSYQ");
var classnames_default = /*#__PURE__*/__webpack_require__.n(classnames);

// EXTERNAL MODULE: ./src/assets/img/grv-tlpt-logo-full.svg
var grv_tlpt_logo_full = __webpack_require__("TglN");

// EXTERNAL MODULE: ./src/assets/img/grv-icon-close.svg
var grv_icon_close = __webpack_require__("Y9yc");

// CONCATENATED MODULE: ./src/app/components/icons.jsx
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






var icons_TeleportLogo = function TeleportLogo() {
  return react_default.a.createElement(
    'svg',
    { className: 'grv-icon-logo-tlpt' },
    react_default.a.createElement('use', { xlinkHref: "#" + grv_tlpt_logo_full["a" /* default */].id })
  );
};

var icons_CloseIcon = function CloseIcon() {
  return react_default.a.createElement(
    'svg',
    { className: 'grv-icon-close' },
    react_default.a.createElement('use', { xlinkHref: "#" + grv_icon_close["a" /* default */].id })
  );
};

var icons_UserIcon = function UserIcon(_ref) {
  var _ref$name = _ref.name,
      name = _ref$name === undefined ? '' : _ref$name,
      isDark = _ref.isDark;

  var iconClass = classnames_default()('grv-icon-user', {
    '--dark': isDark
  });

  return react_default.a.createElement(
    'div',
    { title: name, className: iconClass },
    react_default.a.createElement(
      'span',
      null,
      react_default.a.createElement(
        'strong',
        null,
        name[0]
      )
    )
  );
};


// CONCATENATED MODULE: ./src/app/components/documentTitle.jsx
var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

var _createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function documentTitle_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

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



var DocumentTitle = function (_React$Component) {
  _inherits(DocumentTitle, _React$Component);

  function DocumentTitle() {
    documentTitle_classCallCheck(this, DocumentTitle);

    return _possibleConstructorReturn(this, (DocumentTitle.__proto__ || Object.getPrototypeOf(DocumentTitle)).apply(this, arguments));
  }

  _createClass(DocumentTitle, [{
    key: 'componentDidUpdate',
    value: function componentDidUpdate(prevProps) {
      if (prevProps.title !== this.props.title) {
        this.setTitle(this.props.title);
      }
    }
  }, {
    key: 'componentDidMount',
    value: function componentDidMount() {
      this.setTitle(this.props.title);
    }
  }, {
    key: 'getTitle',
    value: function getTitle() {
      return document.title;
    }
  }, {
    key: 'setTitle',
    value: function setTitle(title) {
      document.title = title;
    }
  }, {
    key: 'render',
    value: function render() {
      return this.props.children;
    }
  }]);

  return DocumentTitle;
}(react_default.a.Component);

var documentTitle_withDocTitle = function withDocTitle(title, component) {
  var _class, _temp;

  return _temp = _class = function (_React$Component2) {
    _inherits(WithWindowTitle, _React$Component2);

    function WithWindowTitle() {
      documentTitle_classCallCheck(this, WithWindowTitle);

      return _possibleConstructorReturn(this, (WithWindowTitle.__proto__ || Object.getPrototypeOf(WithWindowTitle)).apply(this, arguments));
    }

    _createClass(WithWindowTitle, [{
      key: 'componentDidMount',
      value: function componentDidMount() {
        this.setTitle(title);
      }
    }, {
      key: 'setTitle',
      value: function setTitle(title) {
        document.title = title;
      }
    }, {
      key: 'render',
      value: function render() {
        return react_default.a.createElement(component, _extends({}, this.props));
      }
    }]);

    return WithWindowTitle;
  }(react_default.a.Component), _class.displayName = 'withDocTitleWrapper', _temp;
};
// CONCATENATED MODULE: ./src/app/components/user/items.js

var U2F_ERROR_CODES_URL = 'https://developers.yubico.com/U2F/Libraries/Client_error_codes.html';

var items_ErrorMessage = function ErrorMessage(_ref) {
  var message = _ref.message;

  message = message || '';
  if (message.indexOf('U2F') !== -1) {
    return react_default.a.createElement(
      'label',
      { className: 'grv-invite-login-error' },
      message,
      react_default.a.createElement('br', null),
      react_default.a.createElement(
        'small',
        { className: 'grv-invite-login-error-u2f-codes' },
        react_default.a.createElement(
          'span',
          null,
          'click ',
          react_default.a.createElement(
            'a',
            { target: '_blank', href: U2F_ERROR_CODES_URL },
            'here'
          ),
          ' to learn more about U2F error codes'
        )
      )
    );
  }

  return react_default.a.createElement(
    'label',
    { className: 'error' },
    message,
    ' '
  );
};
// CONCATENATED MODULE: ./src/app/services/enums.js
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

var AuthProviderTypeEnum = {
  OIDC: 'oidc',
  SAML: 'saml',
  GITHUB: 'github'
};

var RestRespCodeEnum = {
  FORBIDDEN: 403
};

var Auth2faTypeEnum = {
  UTF: 'u2f',
  OTP: 'otp',
  DISABLED: 'off'
};

var AuthTypeEnum = {
  LOCAL: 'local',
  SSO: 'sso'
};
// CONCATENATED MODULE: ./src/app/components/user/ssoBtnList.jsx




var ssoBtnList_guessProviderBtnClass = function guessProviderBtnClass(name, type) {
  name = name.toLowerCase();

  if (name.indexOf('microsoft') !== -1) {
    return 'btn-microsoft';
  }

  if (name.indexOf('bitbucket') !== -1) {
    return 'btn-bitbucket';
  }

  if (name.indexOf('google') !== -1) {
    return 'btn-google';
  }

  if (name.indexOf('github') !== -1 || type === AuthProviderTypeEnum.GITHUB) {
    return 'btn-github';
  }

  if (type === AuthProviderTypeEnum.OIDC) {
    return 'btn-openid';
  }

  return '--unknown';
};

var ssoBtnList_SsoBtnList = function SsoBtnList(_ref) {
  var providers = _ref.providers,
      prefixText = _ref.prefixText,
      isDisabled = _ref.isDisabled,
      _onClick = _ref.onClick;

  var $btns = providers.map(function (item, index) {
    var name = item.name,
        type = item.type,
        displayName = item.displayName;

    displayName = displayName || name;
    var title = prefixText + ' ' + displayName;
    var providerBtnClass = ssoBtnList_guessProviderBtnClass(displayName, type);
    var btnClass = 'btn grv-user-btn-sso full-width ' + providerBtnClass;
    return react_default.a.createElement(
      'button',
      { key: index,
        disabled: isDisabled,
        className: btnClass,
        onClick: function onClick(e) {
          e.preventDefault();_onClick(item);
        } },
      react_default.a.createElement(
        'div',
        { className: '--sso-icon' },
        react_default.a.createElement('span', { className: 'fa' })
      ),
      react_default.a.createElement(
        'span',
        null,
        title
      )
    );
  });

  if ($btns.length === 0) {
    return react_default.a.createElement(
      'h4',
      null,
      ' You have no SSO providers configured '
    );
  }

  return react_default.a.createElement(
    'div',
    null,
    ' ',
    $btns,
    ' '
  );
};


// CONCATENATED MODULE: ./src/app/components/user/login.jsx
var login_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function _defineProperty(obj, key, value) { if (key in obj) { Object.defineProperty(obj, key, { value: value, enumerable: true, configurable: true, writable: true }); } else { obj[key] = value; } return obj; }

function login_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function login_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function login_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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















var login_Login = function (_React$Component) {
  login_inherits(Login, _React$Component);

  function Login() {
    var _ref;

    var _temp, _this, _ret;

    login_classCallCheck(this, Login);

    for (var _len = arguments.length, args = Array(_len), _key = 0; _key < _len; _key++) {
      args[_key] = arguments[_key];
    }

    return _ret = (_temp = (_this = login_possibleConstructorReturn(this, (_ref = Login.__proto__ || Object.getPrototypeOf(Login)).call.apply(_ref, [this].concat(args))), _this), _this.onLoginWithSso = function (ssoProvider) {
      user_actions.loginWithSso(ssoProvider.name, ssoProvider.url);
    }, _this.onLoginWithU2f = function (username, password) {
      user_actions.loginWithU2f(username, password);
    }, _this.onLogin = function (username, password, token) {
      user_actions.login(username, password, token);
    }, _temp), login_possibleConstructorReturn(_this, _ret);
  }

  login_createClass(Login, [{
    key: 'render',
    value: function render() {
      var attemp = this.props.attemp;

      var authProviders = config["a" /* default */].getAuthProviders();
      var auth2faType = config["a" /* default */].getAuth2faType();

      return react_default.a.createElement(
        'div',
        { className: 'grv-login text-center' },
        react_default.a.createElement(icons_TeleportLogo, null),
        react_default.a.createElement(
          'div',
          { className: 'grv-content grv-flex' },
          react_default.a.createElement(
            'div',
            { className: 'grv-flex-column' },
            react_default.a.createElement(login_LoginInputForm, {
              authProviders: authProviders,
              auth2faType: auth2faType,
              onLoginWithSso: this.onLoginWithSso,
              onLoginWithU2f: this.onLoginWithU2f,
              onLogin: this.onLogin,
              attemp: attemp
            }),
            react_default.a.createElement(login_LoginFooter, { auth2faType: auth2faType })
          )
        )
      );
    }
  }]);

  return Login;
}(react_default.a.Component);

var login_LoginInputForm = function (_React$Component2) {
  login_inherits(LoginInputForm, _React$Component2);

  function LoginInputForm(props) {
    login_classCallCheck(this, LoginInputForm);

    var _this2 = login_possibleConstructorReturn(this, (LoginInputForm.__proto__ || Object.getPrototypeOf(LoginInputForm)).call(this, props));

    _this2.onLogin = function (e) {
      e.preventDefault();
      if (_this2.isValid()) {
        var _this2$state = _this2.state,
            user = _this2$state.user,
            password = _this2$state.password,
            token = _this2$state.token;

        _this2.props.onLogin(user, password, token);
      }
    };

    _this2.onLoginWithU2f = function (e) {
      e.preventDefault();
      if (_this2.isValid()) {
        var _this2$state2 = _this2.state,
            user = _this2$state2.user,
            password = _this2$state2.password;

        _this2.props.onLoginWithU2f(user, password);
      }
    };

    _this2.onLoginWithSso = function (ssoProvider) {
      _this2.props.onLoginWithSso(ssoProvider);
    };

    _this2.onChangeState = function (propName, value) {
      _this2.setState(_defineProperty({}, propName, value));
    };

    _this2.state = {
      user: '',
      password: '',
      token: ''
    };
    return _this2;
  }

  login_createClass(LoginInputForm, [{
    key: 'isValid',
    value: function isValid() {
      var $form = jquery_default()(this.refs.form);
      return $form.length === 0 || $form.valid();
    }
  }, {
    key: 'needs2fa',
    value: function needs2fa() {
      return !!this.props.auth2faType && this.props.auth2faType !== Auth2faTypeEnum.DISABLED;
    }
  }, {
    key: 'needsSso',
    value: function needsSso() {
      return this.props.authProviders && this.props.authProviders.length > 0;
    }
  }, {
    key: 'render2faFields',
    value: function render2faFields() {
      var _this3 = this;

      if (!this.needs2fa() || this.props.auth2faType !== Auth2faTypeEnum.OTP) {
        return null;
      }

      return react_default.a.createElement(
        'div',
        { className: 'form-group' },
        react_default.a.createElement('input', {
          autoComplete: 'off',
          value: this.state.token,
          onChange: function onChange(e) {
            return _this3.onChangeState('token', e.target.value);
          },
          className: 'form-control required',
          name: 'token',
          placeholder: 'Two factor token (Google Authenticator)' })
      );
    }
  }, {
    key: 'renderNameAndPassFields',
    value: function renderNameAndPassFields() {
      var _this4 = this;

      return react_default.a.createElement(
        'div',
        null,
        react_default.a.createElement(
          'div',
          { className: 'form-group' },
          react_default.a.createElement('input', {
            autoFocus: true,
            value: this.state.user,
            onChange: function onChange(e) {
              return _this4.onChangeState('user', e.target.value);
            },
            className: 'form-control required',
            placeholder: 'User name',
            name: 'userName' })
        ),
        react_default.a.createElement(
          'div',
          { className: 'form-group' },
          react_default.a.createElement('input', {
            value: this.state.password,
            onChange: function onChange(e) {
              return _this4.onChangeState('password', e.target.value);
            },
            type: 'password',
            name: 'password',
            className: 'form-control required',
            placeholder: 'Password' })
        )
      );
    }
  }, {
    key: 'renderLoginBtn',
    value: function renderLoginBtn() {
      var isProcessing = this.props.attemp.isProcessing;

      var $helpBlock = isProcessing && this.props.auth2faType === Auth2faTypeEnum.UTF ? react_default.a.createElement(
        'div',
        { className: 'help-block' },
        'Insert your U2F key and press the button on the key'
      ) : null;

      var onClick = this.props.auth2faType === Auth2faTypeEnum.UTF ? this.onLoginWithU2f : this.onLogin;

      return react_default.a.createElement(
        'div',
        null,
        react_default.a.createElement(
          'button',
          {
            onClick: onClick,
            disabled: isProcessing,
            type: 'submit',
            className: 'btn btn-primary block full-width m-b' },
          'Login'
        ),
        $helpBlock
      );
    }
  }, {
    key: 'renderSsoBtns',
    value: function renderSsoBtns() {
      var _props = this.props,
          authProviders = _props.authProviders,
          attemp = _props.attemp;

      if (!this.needsSso()) {
        return null;
      }

      return react_default.a.createElement(ssoBtnList_SsoBtnList, {
        prefixText: 'Login with ',
        isDisabled: attemp.isProcessing,
        providers: authProviders,
        onClick: this.onLoginWithSso });
    }
  }, {
    key: 'render',
    value: function render() {
      var _props$attemp = this.props.attemp,
          isFailed = _props$attemp.isFailed,
          message = _props$attemp.message;

      var $error = isFailed ? react_default.a.createElement(items_ErrorMessage, { message: message }) : null;

      var hasAnyAuth = !!config["a" /* default */].auth;

      return react_default.a.createElement(
        'div',
        null,
        react_default.a.createElement(
          'form',
          { ref: 'form', className: 'grv-login-input-form' },
          react_default.a.createElement(
            'h3',
            null,
            ' Welcome to Teleport '
          ),
          !hasAnyAuth ? react_default.a.createElement(
            'div',
            null,
            ' You have no authentication options configured '
          ) : react_default.a.createElement(
            'div',
            null,
            this.renderNameAndPassFields(),
            this.render2faFields(),
            this.renderLoginBtn(),
            this.renderSsoBtns()
          )
        ),
        $error
      );
    }
  }]);

  return LoginInputForm;
}(react_default.a.Component);

login_LoginInputForm.propTypes = {
  authProviders: react_default.a.PropTypes.array,
  auth2faType: react_default.a.PropTypes.string,
  onLoginWithSso: react_default.a.PropTypes.func.isRequired,
  onLoginWithU2f: react_default.a.PropTypes.func.isRequired,
  onLogin: react_default.a.PropTypes.func.isRequired,
  attemp: react_default.a.PropTypes.object.isRequired
};
var login_LoginFooter = function LoginFooter(_ref2) {
  var auth2faType = _ref2.auth2faType;

  var $googleHint = auth2faType === Auth2faTypeEnum.OTP ? react_default.a.createElement(googleAuthLogo, null) : null;
  return react_default.a.createElement(
    'div',
    null,
    $googleHint,
    react_default.a.createElement(
      'div',
      { className: 'grv-login-info' },
      react_default.a.createElement('i', { className: 'fa fa-question' }),
      react_default.a.createElement(
        'strong',
        null,
        'New Account or forgot password?'
      ),
      react_default.a.createElement(
        'div',
        null,
        'Ask for assistance from your Company administrator'
      )
    )
  );
};

function login_mapStateToProps() {
  return {
    attemp: user_getters.loginAttemp
  };
}

/* harmony default export */ var user_login = (documentTitle_withDocTitle("Login", Object(build["connect"])(login_mapStateToProps)(login_Login)));
// CONCATENATED MODULE: ./src/app/components/msgPage.jsx
var msgPage_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

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




var MSG_INFO_LOGIN_SUCCESS = 'Login successful';
var MSG_ERROR_LOGIN_FAILED = 'Login unsuccessful. Please try again, if the problem persists, contact your system administrator.';
var MSG_ERROR_DEFAULT = 'Internal Error';
var MSG_ERROR_NOT_FOUND = '404 Not Found';
var MSG_ERROR_NOT_FOUND_DETAILS = 'Looks like the page you are looking for isn\'t here any longer.';
var MSG_ERROR_EXPIRED_INVITE = 'Invite code has expired.';
var MSG_ERROR_EXPIRED_INVITE_DETAILS = 'Looks like your invite code isn\'t valid anymore.';
var MSG_ERROR_ACCESS_DENIED = 'Access denied';

var ErrorPageEnum = {
  FAILED_TO_LOGIN: 'login_failed',
  EXPIRED_INVITE: 'expired_invite',
  NOT_FOUND: 'not_found',
  ACCESS_DENIED: 'access_denied'
};

var InfoPageEnum = {
  LOGIN_SUCCESS: 'login_success'
};

var InfoPage = documentTitle_withDocTitle("Info", function (_ref) {
  var params = _ref.params;
  var type = params.type;

  if (type === InfoPageEnum.LOGIN_SUCCESS) {
    return react_default.a.createElement(msgPage_SuccessfulLogin, null);
  }

  return null;
});

var ErrorPage = documentTitle_withDocTitle("Error", function (_ref2) {
  var params = _ref2.params,
      location = _ref2.location;
  var type = params.type;

  var details = location.query.details;
  switch (type) {
    case ErrorPageEnum.FAILED_TO_LOGIN:
      return react_default.a.createElement(msgPage_LoginFailed, { message: details });
    case ErrorPageEnum.EXPIRED_INVITE:
      return react_default.a.createElement(msgPage_ExpiredLink, null);
    case ErrorPageEnum.NOT_FOUND:
      return react_default.a.createElement(msgPage_NotFound, null);
    case ErrorPageEnum.ACCESS_DENIED:
      return react_default.a.createElement(msgPage_AccessDenied, { message: details });
    default:
      return react_default.a.createElement(msgPage_Failed, { message: details });
  }
});

var msgPage_Box = function Box(props) {
  return react_default.a.createElement(
    'div',
    { className: 'grv-msg-page' },
    react_default.a.createElement(
      'div',
      { className: 'grv-header' },
      react_default.a.createElement('i', { className: props.iconClass })
    ),
    props.children
  );
};

var msgPage_ErrorBox = function ErrorBox(props) {
  return react_default.a.createElement(msgPage_Box, msgPage_extends({ iconClass: 'fa fa-exclamation-triangle' }, props));
};

var msgPage_ErrorBoxDetails = function ErrorBoxDetails(_ref3) {
  var _ref3$message = _ref3.message,
      message = _ref3$message === undefined ? '' : _ref3$message;
  return react_default.a.createElement(
    'div',
    { className: 'm-t text-muted' },
    react_default.a.createElement(
      'small',
      { className: 'grv-msg-page-details-text' },
      message
    ),
    react_default.a.createElement(
      'p',
      null,
      react_default.a.createElement(
        'small',
        { className: 'contact-section' },
        'If you believe this is an issue with Teleport, please ',
        react_default.a.createElement(
          'a',
          { href: 'https://github.com/gravitational/teleport/issues/new' },
          'create a GitHub issue.'
        )
      )
    )
  );
};

var msgPage_NotFound = function NotFound() {
  return react_default.a.createElement(
    msgPage_ErrorBox,
    null,
    react_default.a.createElement(
      'h1',
      null,
      MSG_ERROR_NOT_FOUND
    ),
    react_default.a.createElement(msgPage_ErrorBoxDetails, { message: MSG_ERROR_NOT_FOUND_DETAILS })
  );
};

var NotFoundPage = documentTitle_withDocTitle("Not Found", msgPage_NotFound);

var msgPage_AccessDenied = function AccessDenied(_ref4) {
  var message = _ref4.message;
  return react_default.a.createElement(
    msgPage_Box,
    { iconClass: 'fa fa-frown-o' },
    react_default.a.createElement(
      'h1',
      null,
      MSG_ERROR_ACCESS_DENIED
    ),
    react_default.a.createElement(msgPage_ErrorBoxDetails, { message: message })
  );
};

var msgPage_Failed = function Failed(_ref5) {
  var message = _ref5.message;
  return react_default.a.createElement(
    msgPage_ErrorBox,
    null,
    react_default.a.createElement(
      'h1',
      null,
      MSG_ERROR_DEFAULT
    ),
    react_default.a.createElement(msgPage_ErrorBoxDetails, { message: message })
  );
};

var msgPage_ExpiredLink = function ExpiredLink() {
  return react_default.a.createElement(
    msgPage_ErrorBox,
    null,
    react_default.a.createElement(
      'h1',
      null,
      MSG_ERROR_EXPIRED_INVITE
    ),
    react_default.a.createElement(msgPage_ErrorBoxDetails, { message: MSG_ERROR_EXPIRED_INVITE_DETAILS })
  );
};

var msgPage_LoginFailed = function LoginFailed(_ref6) {
  var message = _ref6.message;
  return react_default.a.createElement(
    msgPage_ErrorBox,
    null,
    react_default.a.createElement(
      'h1',
      null,
      MSG_ERROR_LOGIN_FAILED
    ),
    react_default.a.createElement(msgPage_ErrorBoxDetails, { message: message })
  );
};

var msgPage_SuccessfulLogin = function SuccessfulLogin() {
  return react_default.a.createElement(
    msgPage_Box,
    { iconClass: 'fa fa-check-circle m-b-md' },
    react_default.a.createElement(
      'h1',
      null,
      MSG_INFO_LOGIN_SUCCESS
    ),
    react_default.a.createElement(
      'p',
      { className: 'm-t', style: successfulLoginStyles },
      'You have successfully signed into your account. You can close this window and continue using the product.'
    )
  );
};

var successfulLoginStyles = {
  textAlign: "center",
  maxWidth: "500px",
  marginLeft: "auto",
  marginRight: "auto"
};


// CONCATENATED MODULE: ./src/app/components/user/invite.jsx
var invite_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function invite_defineProperty(obj, key, value) { if (key in obj) { Object.defineProperty(obj, key, { value: value, enumerable: true, configurable: true, writable: true }); } else { obj[key] = value; } return obj; }

function invite_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function invite_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function invite_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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















var U2F_HELP_URL = 'https://support.google.com/accounts/answer/6103523?hl=en';

var invite_needs2fa = function needs2fa(auth2faType) {
  return !!auth2faType && auth2faType !== Auth2faTypeEnum.DISABLED;
};

var invite_Invite = function (_React$Component) {
  invite_inherits(Invite, _React$Component);

  function Invite() {
    var _ref;

    var _temp, _this, _ret;

    invite_classCallCheck(this, Invite);

    for (var _len = arguments.length, args = Array(_len), _key = 0; _key < _len; _key++) {
      args[_key] = arguments[_key];
    }

    return _ret = (_temp = (_this = invite_possibleConstructorReturn(this, (_ref = Invite.__proto__ || Object.getPrototypeOf(Invite)).call.apply(_ref, [this].concat(args))), _this), _this.onSubmitWithU2f = function (username, password) {
      user_actions.acceptInviteWithU2f(username, password, _this.props.params.inviteToken);
    }, _this.onSubmit = function (username, password, token) {
      user_actions.acceptInvite(username, password, token, _this.props.params.inviteToken);
    }, _temp), invite_possibleConstructorReturn(_this, _ret);
  }

  invite_createClass(Invite, [{
    key: 'componentDidMount',
    value: function componentDidMount() {
      user_actions.fetchInvite(this.props.params.inviteToken);
    }
  }, {
    key: 'render',
    value: function render() {
      var _props = this.props,
          fetchingInvite = _props.fetchingInvite,
          invite = _props.invite,
          attemp = _props.attemp;

      var auth2faType = config["a" /* default */].getAuth2faType();

      if (fetchingInvite.isFailed) {
        return react_default.a.createElement(msgPage_ExpiredLink, null);
      }

      if (!invite) {
        return null;
      }

      var containerClass = classnames_default()('grv-invite-content grv-flex', {
        '---with-2fa-data': invite_needs2fa(auth2faType)
      });

      return react_default.a.createElement(
        'div',
        { className: 'grv-invite text-center' },
        react_default.a.createElement(icons_TeleportLogo, null),
        react_default.a.createElement(
          'div',
          { className: containerClass },
          react_default.a.createElement(
            'div',
            { className: 'grv-flex-column' },
            react_default.a.createElement(invite_InviteInputForm, {
              auth2faType: auth2faType,
              attemp: attemp,
              invite: invite,
              onSubmitWithU2f: this.onSubmitWithU2f,
              onSubmit: this.onSubmit
            }),
            react_default.a.createElement(invite_InviteFooter, { auth2faType: auth2faType })
          ),
          react_default.a.createElement(invite_Invite2faData, {
            auth2faType: auth2faType,
            qr: invite.qr })
        )
      );
    }
  }]);

  return Invite;
}(react_default.a.Component);

var invite_InviteInputForm = function (_React$Component2) {
  invite_inherits(InviteInputForm, _React$Component2);

  function InviteInputForm(props) {
    invite_classCallCheck(this, InviteInputForm);

    var _this2 = invite_possibleConstructorReturn(this, (InviteInputForm.__proto__ || Object.getPrototypeOf(InviteInputForm)).call(this, props));

    _this2.onSubmit = function (e) {
      e.preventDefault();
      if (_this2.isValid()) {
        var _this2$state = _this2.state,
            userName = _this2$state.userName,
            password = _this2$state.password,
            token = _this2$state.token;

        _this2.props.onSubmit(userName, password, token);
      }
    };

    _this2.onSubmitWithU2f = function (e) {
      e.preventDefault();
      if (_this2.isValid()) {
        var _this2$state2 = _this2.state,
            userName = _this2$state2.userName,
            password = _this2$state2.password;

        _this2.props.onSubmitWithU2f(userName, password);
      }
    };

    _this2.onChangeState = function (propName, value) {
      _this2.setState(invite_defineProperty({}, propName, value));
    };

    _this2.state = {
      userName: _this2.props.invite.user,
      password: '',
      passwordConfirmed: '',
      token: ''
    };
    return _this2;
  }

  invite_createClass(InviteInputForm, [{
    key: 'componentDidMount',
    value: function componentDidMount() {
      jquery_default()(this.refs.form).validate({
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
            minlength: jquery_default.a.validator.format('Enter at least {0} characters'),
            equalTo: 'Enter the same password as above'
          }
        }
      });
    }
  }, {
    key: 'isValid',
    value: function isValid() {
      var $form = jquery_default()(this.refs.form);
      return $form.length === 0 || $form.valid();
    }
  }, {
    key: 'renderNameAndPassFields',
    value: function renderNameAndPassFields() {
      var _this3 = this;

      return react_default.a.createElement(
        'div',
        null,
        react_default.a.createElement(
          'div',
          { className: 'form-group' },
          react_default.a.createElement('input', {
            disabled: true,
            value: this.state.userName,
            onChange: function onChange(e) {
              return _this3.onChangeState('userName', e.target.value);
            },
            className: 'form-control required',
            placeholder: 'User name',
            name: 'userName' })
        ),
        react_default.a.createElement(
          'div',
          { className: 'form-group' },
          react_default.a.createElement('input', {
            value: this.state.password,
            onChange: function onChange(e) {
              return _this3.onChangeState('password', e.target.value);
            },
            autoFocus: true,
            ref: 'password',
            type: 'password',
            name: 'password',
            className: 'form-control required',
            placeholder: 'Password' })
        ),
        react_default.a.createElement(
          'div',
          { className: 'form-group' },
          react_default.a.createElement('input', {
            value: this.state.passwordConfirmed,
            onChange: function onChange(e) {
              return _this3.onChangeState('passwordConfirmed', e.target.value);
            },
            type: 'password',
            name: 'passwordConfirmed',
            className: 'form-control',
            placeholder: 'Password confirm' })
        )
      );
    }
  }, {
    key: 'render2faFields',
    value: function render2faFields() {
      var _this4 = this;

      var auth2faType = this.props.auth2faType;

      if (invite_needs2fa(auth2faType) && auth2faType === Auth2faTypeEnum.OTP) {
        return react_default.a.createElement(
          'div',
          { className: 'form-group' },
          react_default.a.createElement('input', {
            autoComplete: 'off',
            value: this.state.token,
            onChange: function onChange(e) {
              return _this4.onChangeState('token', e.target.value);
            },
            className: 'form-control required',
            name: 'token',
            placeholder: 'Two factor token (Google Authenticator)' })
        );
      }

      return null;
    }
  }, {
    key: 'renderSubmitBtn',
    value: function renderSubmitBtn() {
      var isProcessing = this.props.attemp.isProcessing;

      var $helpBlock = isProcessing && this.props.auth2faType === Auth2faTypeEnum.UTF ? react_default.a.createElement(
        'div',
        { className: 'help-block' },
        'Insert your U2F key and press the button on the key'
      ) : null;

      var onClick = this.props.auth2faType === Auth2faTypeEnum.UTF ? this.onSubmitWithU2f : this.onSubmit;

      return react_default.a.createElement(
        'div',
        null,
        react_default.a.createElement(
          'button',
          {
            onClick: onClick,
            disabled: isProcessing,
            type: 'submit',
            className: 'btn btn-primary block full-width m-b' },
          'Sign up'
        ),
        $helpBlock
      );
    }
  }, {
    key: 'render',
    value: function render() {
      var _props$attemp = this.props.attemp,
          isFailed = _props$attemp.isFailed,
          message = _props$attemp.message;

      var $error = isFailed ? react_default.a.createElement(items_ErrorMessage, { message: message }) : null;
      return react_default.a.createElement(
        'form',
        { ref: 'form', className: 'grv-invite-input-form' },
        react_default.a.createElement(
          'h3',
          null,
          ' Get started with Teleport '
        ),
        this.renderNameAndPassFields(),
        this.render2faFields(),
        this.renderSubmitBtn(),
        $error
      );
    }
  }]);

  return InviteInputForm;
}(react_default.a.Component);

invite_InviteInputForm.propTypes = {
  auth2faType: react_default.a.PropTypes.string,
  authType: react_default.a.PropTypes.string,
  onSubmitWithU2f: react_default.a.PropTypes.func.isRequired,
  onSubmit: react_default.a.PropTypes.func.isRequired,
  attemp: react_default.a.PropTypes.object.isRequired
};
var invite_Invite2faData = function Invite2faData(_ref2) {
  var auth2faType = _ref2.auth2faType,
      qr = _ref2.qr;

  if (!invite_needs2fa(auth2faType)) {
    return null;
  }

  if (auth2faType === Auth2faTypeEnum.OTP) {
    return react_default.a.createElement(
      'div',
      { className: 'grv-flex-column grv-invite-barcode' },
      react_default.a.createElement(
        'h4',
        null,
        'Scan bar code for auth token ',
        react_default.a.createElement('br', null),
        react_default.a.createElement(
          'small',
          null,
          'Scan below to generate your two factor token'
        )
      ),
      react_default.a.createElement('img', { className: 'img-thumbnail', src: 'data:image/png;base64,' + qr })
    );
  }

  if (auth2faType === Auth2faTypeEnum.UTF) {
    return react_default.a.createElement(
      'div',
      { className: 'grv-flex-column' },
      react_default.a.createElement(
        'h3',
        null,
        'Insert your U2F key '
      ),
      react_default.a.createElement(
        'div',
        { className: 'm-t-md' },
        'Press the button on the U2F key after you press the sign up button'
      ),
      react_default.a.createElement(
        'div',
        { className: 'm-t text-muted' },
        react_default.a.createElement(
          'small',
          null,
          'Click',
          react_default.a.createElement(
            'a',
            { a: true, target: '_blank', href: U2F_HELP_URL },
            ' here '
          ),
          'to learn more about U2F 2-Step Verification.'
        )
      )
    );
  }

  return null;
};

var invite_InviteFooter = function InviteFooter(_ref3) {
  var auth2faType = _ref3.auth2faType;

  var $googleHint = auth2faType === Auth2faTypeEnum.OTP ? react_default.a.createElement(googleAuthLogo, null) : null;
  return react_default.a.createElement(
    'div',
    null,
    $googleHint
  );
};

function invite_mapStateToProps() {
  return {
    invite: user_getters.invite,
    attemp: user_getters.attemp,
    fetchingInvite: user_getters.fetchingInvite
  };
}

/* harmony default export */ var components_user_invite = (documentTitle_withDocTitle("Invite", Object(build["connect"])(invite_mapStateToProps)(invite_Invite)));
// CONCATENATED MODULE: ./src/app/routes.jsx
function _toConsumableArray(arr) { if (Array.isArray(arr)) { for (var i = 0, arr2 = Array(arr.length); i < arr.length; i++) { arr2[i] = arr[i]; } return arr2; } else { return Array.from(arr); } }

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






function addRoutes() {
    var routesToAdd = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : [];

    return [{ path: config["a" /* default */].routes.error, component: ErrorPage }, { path: config["a" /* default */].routes.info, component: InfoPage }, { path: config["a" /* default */].routes.login, component: user_login }, { path: config["a" /* default */].routes.newUser, component: components_user_invite }, { path: config["a" /* default */].routes.app, onEnter: function onEnter(localtion, replace) {
            return replace(config["a" /* default */].routes.nodes);
        } }].concat(_toConsumableArray(routesToAdd), [{ path: '*', component: NotFoundPage }]);
}
// CONCATENATED MODULE: ./src/app/flux/userAcl/getters.js
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

var userAcl = ['tlpt_user_acl'];

/* harmony default export */ var userAcl_getters = ({
  userAcl: userAcl
});
// CONCATENATED MODULE: ./src/app/flux/nodes/getters.js
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

var getters_siteNodes = [['tlpt_nodes'], ['tlpt', 'siteId'], function (nodeStore, siteId) {
  return nodeStore.getSiteServers(siteId);
}];

/* harmony default export */ var nodes_getters = ({
  siteNodes: getters_siteNodes
});
// EXTERNAL MODULE: ./src/app/flux/app/getters.js
var app_getters = __webpack_require__("i3tb");

// EXTERNAL MODULE: ./node_modules/lodash/lodash.js
var lodash = __webpack_require__("LvDl");

// CONCATENATED MODULE: ./src/app/lib/objectUtils.js
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

var uuid = {
  3: /^[0-9A-F]{8}-[0-9A-F]{4}-3[0-9A-F]{3}-[0-9A-F]{4}-[0-9A-F]{12}$/i,
  4: /^[0-9A-F]{8}-[0-9A-F]{4}-4[0-9A-F]{3}-[89AB][0-9A-F]{3}-[0-9A-F]{12}$/i,
  5: /^[0-9A-F]{8}-[0-9A-F]{4}-5[0-9A-F]{3}-[89AB][0-9A-F]{3}-[0-9A-F]{12}$/i,
  all: /^[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12}$/i
};

var PORT_REGEX = /:\d+$/;

function parseIp(addr) {
  addr = addr || '';
  return addr.replace(PORT_REGEX, '');
}

function isMatch(obj, searchValue, _ref) {
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
}

function isUUID(str) {
  var version = arguments.length > 1 && arguments[1] !== undefined ? arguments[1] : 'all';

  var pattern = uuid[version];
  return pattern && pattern.test(str);
}
// CONCATENATED MODULE: ./src/app/components/inputSearch.jsx
var inputSearch_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function inputSearch_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function inputSearch_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function inputSearch_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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





var inputSearch_InputSearch = function (_React$Component) {
  inputSearch_inherits(InputSearch, _React$Component);

  function InputSearch(props) {
    inputSearch_classCallCheck(this, InputSearch);

    var _this = inputSearch_possibleConstructorReturn(this, (InputSearch.__proto__ || Object.getPrototypeOf(InputSearch)).call(this, props));

    _this.onChange = function (e) {
      _this.setState({ value: e.target.value });
      _this.debouncedNotify();
    };

    _this.debouncedNotify = Object(lodash["debounce"])(function () {
      _this.props.onChange(_this.state.value);
    }, 200);

    var value = props.value || '';

    _this.state = {
      value: value
    };
    return _this;
  }

  inputSearch_createClass(InputSearch, [{
    key: 'componentDidMount',
    value: function componentDidMount() {
      // set cursor
      var $el = react_dom_default.a.findDOMNode(this);
      if ($el) {
        var $input = $el.querySelector('input');
        var length = $input.value.length;
        $input.selectionEnd = length;
        $input.selectionStart = length;
      }
    }
  }, {
    key: 'render',
    value: function render() {
      var _props = this.props,
          _props$className = _props.className,
          className = _props$className === undefined ? '' : _props$className,
          _props$autoFocus = _props.autoFocus,
          autoFocus = _props$autoFocus === undefined ? false : _props$autoFocus;

      className = 'grv-search input-group-sm ' + className;

      return react_default.a.createElement(
        'div',
        { className: className },
        react_default.a.createElement('input', { placeholder: 'Search...', className: 'form-control',
          autoFocus: autoFocus,
          value: this.state.value,
          onChange: this.onChange })
      );
    }
  }]);

  return InputSearch;
}(react_default.a.Component);

/* harmony default export */ var inputSearch = (inputSearch_InputSearch);
// CONCATENATED MODULE: ./src/app/components/inputSshServer.jsx
var _slicedToArray = function () { function sliceIterator(arr, i) { var _arr = []; var _n = true; var _d = false; var _e = undefined; try { for (var _i = arr[Symbol.iterator](), _s; !(_n = (_s = _i.next()).done); _n = true) { _arr.push(_s.value); if (i && _arr.length === i) break; } } catch (err) { _d = true; _e = err; } finally { try { if (!_n && _i["return"]) _i["return"](); } finally { if (_d) throw _e; } } return _arr; } return function (arr, i) { if (Array.isArray(arr)) { return arr; } else if (Symbol.iterator in Object(arr)) { return sliceIterator(arr, i); } else { throw new TypeError("Invalid attempt to destructure non-iterable instance"); } }; }();

var inputSshServer_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function inputSshServer_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function inputSshServer_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function inputSshServer_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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




var SSH_STR_REGEX = /(^\w+@(\w|\.|-)+(:\d+)*$)|(^$)/;
var PLACEHOLDER_TEXT = 'login@host';
var DEFAULT_HISTORY_INDEX = -1;

var KeyEnum = {
  UP: 38,
  DOWN: 40
};

var inputSshServer_InputSshServer = function (_React$Component) {
  inputSshServer_inherits(InputSshServer, _React$Component);

  function InputSshServer() {
    var _ref;

    var _temp, _this, _ret;

    inputSshServer_classCallCheck(this, InputSshServer);

    for (var _len = arguments.length, args = Array(_len), _key = 0; _key < _len; _key++) {
      args[_key] = arguments[_key];
    }

    return _ret = (_temp = (_this = inputSshServer_possibleConstructorReturn(this, (_ref = InputSshServer.__proto__ || Object.getPrototypeOf(InputSshServer)).call.apply(_ref, [this].concat(args))), _this), _this.prevLoginIndex = DEFAULT_HISTORY_INDEX, _this.state = {
      hasErrors: false,
      value: ''
    }, _this.onChange = function (e) {
      var value = e.target.value;
      var isValid = _this.isValid(value);
      if (isValid && _this.state.hasErrors === true) {
        _this.setState({
          hasErrors: false,
          value: value
        });
      }

      _this.setState({ value: value });
    }, _this.onKeyUp = function (e) {
      if (_this.getPrevLogins().length === 0) {
        return;
      }

      var dir = 0;
      var keyCode = e.which;
      if (keyCode === KeyEnum.UP) {
        dir = 1;
      }

      if (keyCode === KeyEnum.DOWN) {
        dir = -1;
      }

      if (dir === 0) {
        return;
      }

      var login = _this.getNextLogin(dir);
      _this.setState({ value: login });
    }, _this.onKeyPress = function (e) {
      var value = e.target.value;
      var isValid = _this.isValid(value);
      if ((e.key === 'Enter' || e.type === 'click') && value) {
        _this.setState({ hasErrors: !isValid });
        if (isValid) {
          var _value$split = value.split('@'),
              _value$split2 = _slicedToArray(_value$split, 2),
              login = _value$split2[0],
              host = _value$split2[1];

          _this.props.onEnter(login, host);
        }
      }
    }, _temp), inputSshServer_possibleConstructorReturn(_this, _ret);
  }

  inputSshServer_createClass(InputSshServer, [{
    key: 'setDefaultPrevIndex',
    value: function setDefaultPrevIndex() {
      this.prevLoginIndex = DEFAULT_HISTORY_INDEX;
    }
  }, {
    key: 'componentWillReceiveProps',
    value: function componentWillReceiveProps(nextProps) {
      if (nextProps.clusterId !== this.props.clusterId || nextProps.sshHistory !== this.props.sshHistory) {
        this.setDefaultPrevIndex();
        this.setState({ value: '' });
      }
    }
  }, {
    key: 'getPrevLogins',
    value: function getPrevLogins() {
      var _props = this.props,
          sshHistory = _props.sshHistory,
          clusterId = _props.clusterId;

      return sshHistory.getPrevLogins(clusterId);
    }
  }, {
    key: 'getNextLogin',
    value: function getNextLogin(dir) {
      var logins = this.getPrevLogins();

      if (logins.length === 0) {
        return '';
      }

      var index = this.prevLoginIndex + dir;

      if (index < 0) {
        this.setDefaultPrevIndex();
        return '';
      }

      if (index >= logins.length) {
        index = this.prevLoginIndex;
      } else {
        this.prevLoginIndex = index;
      }

      return logins[this.prevLoginIndex];
    }
  }, {
    key: 'isValid',
    value: function isValid(value) {
      var match = SSH_STR_REGEX.exec(value);
      return !!match;
    }
  }, {
    key: 'render',
    value: function render() {
      var _this2 = this;

      var _state = this.state,
          value = _state.value,
          hasErrors = _state.hasErrors;
      var _props$autoFocus = this.props.autoFocus,
          autoFocus = _props$autoFocus === undefined ? false : _props$autoFocus;

      var className = classnames_default()('grv-sshserver-input', { '--error': hasErrors });
      return react_default.a.createElement(
        'div',
        { className: className },
        react_default.a.createElement(
          'div',
          { className: 'm-l input-group-sm', title: 'login to SSH server' },
          react_default.a.createElement('input', { ref: function ref(e) {
              _this2.inputRef = e;
            }, className: 'form-control',
            placeholder: PLACEHOLDER_TEXT,
            value: value,
            autoFocus: autoFocus,
            onChange: this.onChange,
            onKeyUp: this.onKeyUp,
            onKeyPress: this.onKeyPress
          })
        ),
        react_default.a.createElement(
          'label',
          { className: 'm-l grv-sshserver-input-errors' },
          ' Invalid format '
        )
      );
    }
  }]);

  return InputSshServer;
}(react_default.a.Component);

inputSshServer_InputSshServer.propTypes = {
  sshHistory: react["PropTypes"].object.isRequired,
  clusterId: react["PropTypes"].string.isRequired,
  onEnter: react["PropTypes"].func.isRequired
};
/* harmony default export */ var inputSshServer = (inputSshServer_InputSshServer);
// CONCATENATED MODULE: ./src/app/components/table/table.jsx
var table_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

var table_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function table_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function table_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function table_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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



var table_TableTextCell = function TableTextCell(_ref) {
  var rowIndex = _ref.rowIndex,
      data = _ref.data,
      columnKey = _ref.columnKey,
      props = _objectWithoutProperties(_ref, ['rowIndex', 'data', 'columnKey']);

  return react_default.a.createElement(
    table_TableCell,
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

var table_SortIndicator = function SortIndicator(_ref2) {
  var sortDir = _ref2.sortDir;

  var cls = 'grv-table-indicator-sort fa fa-sort';
  if (sortDir === SortTypes.DESC) {
    cls += '-desc';
  }

  if (sortDir === SortTypes.ASC) {
    cls += '-asc';
  }

  return react_default.a.createElement('i', { className: cls });
};

/**
* Sort Header Cell
*/

var table_SortHeaderCell = function (_React$Component) {
  table_inherits(SortHeaderCell, _React$Component);

  function SortHeaderCell() {
    var _ref3;

    var _temp, _this, _ret;

    table_classCallCheck(this, SortHeaderCell);

    for (var _len = arguments.length, args = Array(_len), _key = 0; _key < _len; _key++) {
      args[_key] = arguments[_key];
    }

    return _ret = (_temp = (_this = table_possibleConstructorReturn(this, (_ref3 = SortHeaderCell.__proto__ || Object.getPrototypeOf(SortHeaderCell)).call.apply(_ref3, [this].concat(args))), _this), _this.onSortChange = function (e) {
      e.preventDefault();
      if (_this.props.onSortChange) {
        // default
        var newDir = SortTypes.DESC;
        if (_this.props.sortDir) {
          newDir = _this.props.sortDir === SortTypes.DESC ? SortTypes.ASC : SortTypes.DESC;
        }
        _this.props.onSortChange(_this.props.columnKey, newDir);
      }
    }, _temp), table_possibleConstructorReturn(_this, _ret);
  }

  table_createClass(SortHeaderCell, [{
    key: 'render',
    value: function render() {
      var _props = this.props,
          sortDir = _props.sortDir,
          title = _props.title,
          props = _objectWithoutProperties(_props, ['sortDir', 'title']);

      return react_default.a.createElement(
        table_TableCell,
        props,
        react_default.a.createElement(
          'a',
          { onClick: this.onSortChange },
          title
        ),
        react_default.a.createElement(table_SortIndicator, { sortDir: sortDir })
      );
    }
  }]);

  return SortHeaderCell;
}(react_default.a.Component);

/**
* Default Cell
*/


var table_TableCell = function TableCell(props) {
  var isHeader = props.isHeader,
      children = props.children,
      _props$className = props.className,
      className = _props$className === undefined ? '' : _props$className;

  className = 'grv-table-cell ' + className;
  return isHeader ? react_default.a.createElement(
    'th',
    { className: className },
    children
  ) : react_default.a.createElement(
    'td',
    null,
    children
  );
};

/**
* Table
*/

var table_Table = function (_React$Component2) {
  table_inherits(Table, _React$Component2);

  function Table() {
    table_classCallCheck(this, Table);

    return table_possibleConstructorReturn(this, (Table.__proto__ || Object.getPrototypeOf(Table)).apply(this, arguments));
  }

  table_createClass(Table, [{
    key: 'renderHeader',
    value: function renderHeader(children) {
      var _this3 = this;

      var data = this.props.data;

      var cells = children.map(function (item, index) {
        return _this3.renderCell(item.props.header, table_extends({
          index: index,
          key: index,
          isHeader: true,
          data: data
        }, item.props));
      });

      return react_default.a.createElement(
        'thead',
        { className: 'grv-table-header' },
        react_default.a.createElement(
          'tr',
          null,
          cells
        )
      );
    }
  }, {
    key: 'renderBody',
    value: function renderBody(children) {
      var _this4 = this;

      var data = this.props.data;

      var count = this.props.rowCount;
      var rows = [];
      for (var i = 0; i < count; i++) {
        var cells = children.map(function (item, index) {
          return _this4.renderCell(item.props.cell, table_extends({
            rowIndex: i,
            key: index,
            isHeader: false,
            data: data
          }, item.props));
        });

        rows.push(react_default.a.createElement(
          'tr',
          { key: i },
          cells
        ));
      }

      return react_default.a.createElement(
        'tbody',
        null,
        rows
      );
    }
  }, {
    key: 'renderCell',
    value: function renderCell(cell, cellProps) {
      var content = null;
      if (react_default.a.isValidElement(cell)) {
        content = react_default.a.cloneElement(cell, cellProps);
      } else if (typeof cell === 'function') {
        content = cell(cellProps);
      }

      return content;
    }
  }, {
    key: 'render',
    value: function render() {
      var children = [];
      react_default.a.Children.forEach(this.props.children, function (child) {
        if (child == null) {
          return;
        }

        if (!child.props._isColumn) {
          throw 'Should be Column';
        }

        children.push(child);
      });

      var tableClass = 'table grv-table ' + this.props.className;

      return react_default.a.createElement(
        'table',
        { className: tableClass },
        this.renderHeader(children),
        this.renderBody(children)
      );
    }
  }]);

  return Table;
}(react_default.a.Component);

var Column = function (_React$Component3) {
  table_inherits(Column, _React$Component3);

  function Column() {
    table_classCallCheck(this, Column);

    return table_possibleConstructorReturn(this, (Column.__proto__ || Object.getPrototypeOf(Column)).apply(this, arguments));
  }

  table_createClass(Column, [{
    key: 'render',
    value: function render() {
      throw new Error('Component <Column /> should never render');
    }
  }]);

  return Column;
}(react_default.a.Component);

Column.defaultProps = {
  _isColumn: true
};


var table_EmptyIndicator = function EmptyIndicator(_ref4) {
  var text = _ref4.text;
  return react_default.a.createElement(
    'div',
    { className: 'grv-table-indicator-empty text-muted' },
    react_default.a.createElement(
      'span',
      null,
      text
    )
  );
};

/* harmony default export */ var table = (table_Table);

// CONCATENATED MODULE: ./src/app/flux/sites/getters.js
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

var getters_sites = [['tlpt_sites'], function (siteList) {
  return siteList.filter(onlyOnline).toArray();
}];

/* harmony default export */ var sites_getters = ({
  sites: getters_sites
});
// CONCATENATED MODULE: ./src/app/components/dropdown.jsx
var dropdown_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function dropdown_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function dropdown_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function dropdown_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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






var dropdown_DropDown = function (_React$Component) {
  dropdown_inherits(DropDown, _React$Component);

  function DropDown() {
    var _ref;

    var _temp, _this, _ret;

    dropdown_classCallCheck(this, DropDown);

    for (var _len = arguments.length, args = Array(_len), _key = 0; _key < _len; _key++) {
      args[_key] = arguments[_key];
    }

    return _ret = (_temp = (_this = dropdown_possibleConstructorReturn(this, (_ref = DropDown.__proto__ || Object.getPrototypeOf(DropDown)).call.apply(_ref, [this].concat(args))), _this), _this.onClick = function (event) {
      event.preventDefault();
      var options = _this.props.options;

      var index = jquery_default()(event.target).parent().index();
      var option = options[index];
      var value = Object(lodash["isObject"])(option) ? option.value : option;

      _this.props.onChange(value);
    }, _temp), dropdown_possibleConstructorReturn(_this, _ret);
  }

  dropdown_createClass(DropDown, [{
    key: 'renderOption',
    value: function renderOption(option, index) {
      var displayValue = Object(lodash["isObject"])(option) ? option.label : option;
      return react_default.a.createElement(
        'li',
        { key: index },
        react_default.a.createElement(
          'a',
          { href: '#' },
          displayValue
        )
      );
    }
  }, {
    key: 'getDisplayValue',
    value: function getDisplayValue(value) {
      var _props$options = this.props.options,
          options = _props$options === undefined ? [] : _props$options;

      for (var i = 0; i < options.length; i++) {
        var op = options[i];
        if (Object(lodash["isObject"])(op) && op.value === value) {
          return op.label;
        }

        if (op === value) {
          return value;
        }
      }

      return null;
    }
  }, {
    key: 'render',
    value: function render() {
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

      var valueClass = classnames_default()('grv-dropdown-value', {
        'text-muted': !hiddenValue
      });

      var mainClass = 'grv-dropdown ' + className;

      var btnClass = classnames_default()('btn btn-default full-width dropdown-toggle', {
        'btn-sm': size === 'sm'
      });

      var menuClass = classnames_default()('dropdown-menu', {
        'pull-right': align === 'right'
      });

      var $menu = options.length > 0 ? react_default.a.createElement(
        'ul',
        { onClick: this.onClick, className: menuClass },
        $options
      ) : null;

      return react_default.a.createElement(
        'div',
        { className: mainClass },
        react_default.a.createElement(
          'div',
          { className: 'dropdown' },
          react_default.a.createElement(
            'div',
            { className: btnClass,
              type: 'button',
              'data-toggle': 'dropdown' },
            react_default.a.createElement(
              'div',
              { className: valueClass },
              react_default.a.createElement(
                'span',
                { style: { textOverflow: "ellipsis", overflow: "hidden" } },
                displayValue
              ),
              react_default.a.createElement('span', { className: 'caret m-l-sm' })
            )
          ),
          $menu
        ),
        react_default.a.createElement('input', { className: classRules,
          value: hiddenValue,
          type: 'hidden',
          ref: 'input',
          name: name })
      );
    }
  }]);

  return DropDown;
}(react_default.a.Component);

/* harmony default export */ var dropdown = (dropdown_DropDown);
// CONCATENATED MODULE: ./src/app/flux/nodes/actionTypes.js
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

var TLPT_NODES_RECEIVE = 'TLPT_NODES_RECEIVE';
// CONCATENATED MODULE: ./src/app/flux/nodes/actions.js






var nodes_actions_logger = logger["a" /* default */].create('Modules/Nodes');

function fetchNodes() {
  var siteId = app_reactor["a" /* default */].evaluate(app_getters["a" /* default */].siteId);
  return api["a" /* default */].get(config["a" /* default */].api.getSiteNodesUrl(siteId)).then(function (res) {
    return res.items || [];
  }).done(function (items) {
    return app_reactor["a" /* default */].dispatch(TLPT_NODES_RECEIVE, items);
  }).fail(function (err) {
    return nodes_actions_logger.error('fetchNodes', err);
  });
}
// CONCATENATED MODULE: ./src/app/flux/app/actionTypes.js
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

var SET_SITE_ID = 'TLPT_APP_SET_SITE_ID';
var ADD_NAV_ITEM = 'TLPT_APP_ADD_NAV_ITEM';
// CONCATENATED MODULE: ./src/app/flux/sites/actionTypes.js
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

var RECEIVE_CLUSTERS = 'TLPT_CLUSTER_RECEIVE';
// CONCATENATED MODULE: ./src/app/flux/userAcl/actionTypes.js
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

var RECEIVE_USERACL = 'TLPT_USERACL_RECEIVE';
// EXTERNAL MODULE: ./src/app/flux/sessions/actions.js
var sessions_actions = __webpack_require__("CtRu");

// CONCATENATED MODULE: ./src/app/flux/app/actions.js
function actions_toConsumableArray(arr) { if (Array.isArray(arr)) { for (var i = 0, arr2 = Array(arr.length); i < arr.length; i++) { arr2[i] = arr[i]; } return arr2; } else { return Array.from(arr); } }

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














var app_actions_logger = logger["a" /* default */].create('flux/app');

function addNavItem(item) {
  app_reactor["a" /* default */].dispatch(ADD_NAV_ITEM, item);
}

function setSiteId(siteId) {
  app_reactor["a" /* default */].dispatch(SET_SITE_ID, siteId);
}

function initApp(siteId, featureActivator) {
  initAppStatus.start();
  // get the list of available clusters
  return fetchInitData(siteId).done(function () {
    featureActivator.onload();
    initAppStatus.success();
  }).fail(function (err) {
    var msg = api["a" /* default */].getErrorText(err);
    initAppStatus.fail(msg);
  });
}

function fetchInitData(siteId) {
  return jquery_default.a.when(fetchSites(), fetchUserContext()).then(function (masterSiteId) {
    var selectedCluster = siteId || masterSiteId;
    setSiteId(selectedCluster);
    return jquery_default.a.when(fetchNodes(), Object(sessions_actions["a" /* fetchActiveSessions */])());
  });
}

function fetchSites() {
  return api["a" /* default */].get(config["a" /* default */].api.sitesBasePath).then(function (json) {
    var trusted = json.trusted || [];
    var allClusters = [json.current].concat(actions_toConsumableArray(trusted));
    app_reactor["a" /* default */].dispatch(RECEIVE_CLUSTERS, allClusters);
    return json.current.name;
  }).fail(function (err) {
    app_actions_logger.error('fetchSites', err);
  });
}

function fetchUserContext() {
  return api["a" /* default */].get(config["a" /* default */].api.userContextPath).done(function (json) {
    app_reactor["a" /* default */].dispatch(RECEIVE_USER, { name: json.userName, authType: json.authType });
    app_reactor["a" /* default */].dispatch(RECEIVE_USERACL, json.userAcl);
    app_actions_logger.info("Teleport ver:", json.version);
  });
}
// CONCATENATED MODULE: ./src/app/components/clusterSelector.jsx
var clusterSelector_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function clusterSelector_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function clusterSelector_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function clusterSelector_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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











var clusterSelector_ClusterSelector = function (_React$Component) {
  clusterSelector_inherits(ClusterSelector, _React$Component);

  function ClusterSelector() {
    var _ref;

    var _temp, _this, _ret;

    clusterSelector_classCallCheck(this, ClusterSelector);

    for (var _len = arguments.length, args = Array(_len), _key = 0; _key < _len; _key++) {
      args[_key] = arguments[_key];
    }

    return _ret = (_temp = (_this = clusterSelector_possibleConstructorReturn(this, (_ref = ClusterSelector.__proto__ || Object.getPrototypeOf(ClusterSelector)).call.apply(_ref, [this].concat(args))), _this), _this.onChangeSite = function (value) {
      setSiteId(value);
      fetchNodes();
      Object(sessions_actions["a" /* fetchActiveSessions */])();
    }, _temp), clusterSelector_possibleConstructorReturn(_this, _ret);
  }

  clusterSelector_createClass(ClusterSelector, [{
    key: 'render',
    value: function render() {
      var _props = this.props,
          sites = _props.sites,
          siteId = _props.siteId;

      var siteOptions = sites.map(function (s) {
        return {
          label: s.name,
          value: s.name
        };
      });

      if (siteOptions.length === 1 && isUUID(siteOptions[0].value)) {
        siteOptions[0].label = location.hostname;
      }

      return react_default.a.createElement(
        'div',
        { className: 'grv-clusters-selector' },
        react_default.a.createElement(
          'div',
          { className: 'm-r-sm' },
          'Cluster:'
        ),
        react_default.a.createElement(dropdown, {
          className: 'm-r-sm',
          size: 'sm',
          align: 'right',
          onChange: this.onChangeSite,
          value: siteId,
          options: siteOptions
        })
      );
    }
  }]);

  return ClusterSelector;
}(react_default.a.Component);

function clusterSelector_mapStateToProps() {
  return {
    sites: sites_getters.sites,
    siteId: app_getters["a" /* default */].siteId
  };
}

/* harmony default export */ var clusterSelector = (Object(build["connect"])(clusterSelector_mapStateToProps)(clusterSelector_ClusterSelector));
// CONCATENATED MODULE: ./src/app/components/table/pagedTable.jsx
var pagedTable_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

/*
Copyright 2018 Gravitational, Inc.

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





var PagedTable = react_default.a.createClass({
  displayName: 'PagedTable',
  onPrev: function onPrev() {
    var _state = this.state,
        startFrom = _state.startFrom,
        pageSize = _state.pageSize;


    startFrom = startFrom - pageSize;

    if (startFrom < 0) {
      startFrom = 0;
    }

    this.setState({
      startFrom: startFrom
    });
  },
  onNext: function onNext() {
    var _props$data = this.props.data,
        data = _props$data === undefined ? [] : _props$data;
    var _state2 = this.state,
        startFrom = _state2.startFrom,
        pageSize = _state2.pageSize;

    var newStartFrom = startFrom + pageSize;

    if (newStartFrom < data.length) {
      newStartFrom = startFrom + pageSize;
      this.setState({
        startFrom: newStartFrom
      });
    }
  },
  getInitialState: function getInitialState() {
    var _props$pageSize = this.props.pageSize,
        pageSize = _props$pageSize === undefined ? 7 : _props$pageSize;

    return {
      startFrom: 0,
      pageSize: pageSize
    };
  },
  componentWillReceiveProps: function componentWillReceiveProps(newProps) {
    var newData = newProps.data || [];
    var oldData = this.props.data || [];
    // if data length changes, reset paging
    if (newData.length !== oldData.length) {
      this.setState({ startFrom: 0 });
    }
  },
  render: function render() {
    var _state3 = this.state,
        startFrom = _state3.startFrom,
        pageSize = _state3.pageSize;
    var _props = this.props,
        _props$data2 = _props.data,
        data = _props$data2 === undefined ? [] : _props$data2,
        _props$tableClass = _props.tableClass,
        tableClass = _props$tableClass === undefined ? '' : _props$tableClass,
        _props$className = _props.className,
        className = _props$className === undefined ? '' : _props$className;

    var totalRows = data.length;

    var endAt = 0;
    var pagedData = data;

    if (data.length > 0) {
      endAt = startFrom + (pageSize > data.length ? data.length : pageSize);

      if (endAt > data.length) {
        endAt = data.length;
      }

      pagedData = data.slice(startFrom, endAt);
    }

    var tableProps = pagedTable_extends({}, this.props, {
      rowCount: pagedData.length,
      data: pagedData
    });

    var infoProps = {
      pageSize: pageSize,
      startFrom: startFrom,
      endAt: endAt,
      totalRows: totalRows
    };

    return react_default.a.createElement(
      'div',
      { className: className },
      react_default.a.createElement(pagedTable_PageInfo, pagedTable_extends({}, infoProps, { onPrev: this.onPrev, onNext: this.onNext })),
      react_default.a.createElement(
        'div',
        { className: tableClass },
        react_default.a.createElement(table_Table, tableProps)
      ),
      react_default.a.createElement(pagedTable_PageInfo, pagedTable_extends({}, infoProps, { onPrev: this.onPrev, onNext: this.onNext }))
    );
  }
});

var pagedTable_PageInfo = function PageInfo(props) {
  var startFrom = props.startFrom,
      endAt = props.endAt,
      totalRows = props.totalRows,
      onPrev = props.onPrev,
      onNext = props.onNext,
      pageSize = props.pageSize;


  var shouldBeDisplayed = totalRows > pageSize;

  if (!shouldBeDisplayed) {
    return null;
  }

  var prevBtnClass = classnames_default()('btn btn-white', {
    'disabled': startFrom === 0
  });

  var nextBtnClass = classnames_default()('btn btn-white', {
    'disabled': endAt === totalRows
  });

  return react_default.a.createElement(
    'div',
    { className: 'm-b-sm grv-table-paged-info' },
    react_default.a.createElement(
      'span',
      { className: 'm-r-sm' },
      react_default.a.createElement(
        'span',
        { className: 'text-muted' },
        'Showing '
      ),
      react_default.a.createElement(
        'span',
        { className: 'font-bold' },
        startFrom + 1
      ),
      react_default.a.createElement(
        'span',
        { className: 'text-muted' },
        ' to '
      ),
      react_default.a.createElement(
        'span',
        { className: 'font-bold' },
        endAt
      ),
      react_default.a.createElement(
        'span',
        { className: 'text-muted' },
        ' of '
      ),
      react_default.a.createElement(
        'span',
        { className: 'font-bold' },
        totalRows
      )
    ),
    react_default.a.createElement(
      'div',
      { className: 'btn-group btn-group-sm' },
      react_default.a.createElement(
        'a',
        { onClick: onPrev, className: prevBtnClass, type: 'button' },
        'Prev'
      ),
      react_default.a.createElement(
        'a',
        { onClick: onNext, className: nextBtnClass, type: 'button' },
        'Next'
      )
    )
  );
};

/* harmony default export */ var pagedTable = (PagedTable);


// CONCATENATED MODULE: ./src/app/components/nodes/nodeList.jsx
var nodeList_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function nodeList_defineProperty(obj, key, value) { if (key in obj) { Object.defineProperty(obj, key, { value: value, enumerable: true, configurable: true, writable: true }); } else { obj[key] = value; } return obj; }

function nodeList_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function nodeList_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function nodeList_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

function nodeList_objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }

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













var nodeList_EmptyValue = function EmptyValue(_ref) {
  var _ref$text = _ref.text,
      text = _ref$text === undefined ? 'Empty' : _ref$text;
  return react_default.a.createElement(
    'small',
    { className: 'text-muted' },
    react_default.a.createElement(
      'span',
      null,
      text
    )
  );
};

var nodeList_TagCell = function TagCell(_ref2) {
  var rowIndex = _ref2.rowIndex,
      data = _ref2.data,
      props = nodeList_objectWithoutProperties(_ref2, ['rowIndex', 'data']);

  var tags = data[rowIndex].tags;

  var $content = tags.map(function (item, index) {
    return react_default.a.createElement(
      'span',
      { key: index, title: item.name + ':' + item.value, className: 'label label-default grv-nodes-table-label' },
      item.name,
      ' ',
      react_default.a.createElement('li', { className: 'fa fa-long-arrow-right m-r-xs' }),
      item.value
    );
  });

  if ($content.length === 0) {
    $content = react_default.a.createElement(nodeList_EmptyValue, { text: 'No assigned labels' });
  }

  return react_default.a.createElement(
    table_TableCell,
    props,
    $content
  );
};

var nodeList_LoginCell = function (_React$Component) {
  nodeList_inherits(LoginCell, _React$Component);

  function LoginCell() {
    var _ref3;

    var _temp, _this, _ret;

    nodeList_classCallCheck(this, LoginCell);

    for (var _len = arguments.length, args = Array(_len), _key = 0; _key < _len; _key++) {
      args[_key] = arguments[_key];
    }

    return _ret = (_temp = (_this = nodeList_possibleConstructorReturn(this, (_ref3 = LoginCell.__proto__ || Object.getPrototypeOf(LoginCell)).call.apply(_ref3, [this].concat(args))), _this), _this.onKeyPress = function (e) {
      if (e.key === 'Enter' && e.target.value) {
        var url = _this.makeUrl(e.target.value);
        services_history.push(url);
      }
    }, _this.onShowLoginsClick = function (e) {
      e.stopPropagation();
      e.nativeEvent.stopImmediatePropagation();
      _this.refs.customLogin.focus();
    }, _temp), nodeList_possibleConstructorReturn(_this, _ret);
  }

  nodeList_createClass(LoginCell, [{
    key: 'makeUrl',
    value: function makeUrl(login) {
      var _props = this.props,
          data = _props.data,
          rowIndex = _props.rowIndex;
      var _data$rowIndex = data[rowIndex],
          siteId = _data$rowIndex.siteId,
          id = _data$rowIndex.id;

      return config["a" /* default */].getTerminalLoginUrl({
        siteId: siteId,
        serverId: id,
        login: login
      });
    }
  }, {
    key: 'render',
    value: function render() {
      var _props2 = this.props,
          logins = _props2.logins,
          props = nodeList_objectWithoutProperties(_props2, ['logins']);

      var $lis = [];
      var defaultLogin = logins[0] || '';
      var defaultTermUrl = this.makeUrl(defaultLogin);

      for (var i = 0; i < logins.length; i++) {
        var termUrl = this.makeUrl(logins[i]);
        $lis.push(react_default.a.createElement(
          'li',
          { key: i },
          react_default.a.createElement(
            lib["Link"],
            { to: termUrl },
            logins[i]
          )
        ));
      }

      return react_default.a.createElement(
        table_TableCell,
        props,
        react_default.a.createElement(
          'div',
          { style: { display: "flex", justifyContent: "flex-end" } },
          logins.length === 0 && react_default.a.createElement(nodeList_EmptyValue, { text: 'No assigned logins' }),
          logins.length > 0 && react_default.a.createElement(
            'div',
            { style: { display: "flex" }, className: 'btn-group' },
            react_default.a.createElement(
              lib["Link"],
              { className: 'btn btn-xs btn-primary', to: defaultTermUrl },
              defaultLogin
            ),
            react_default.a.createElement(
              'button',
              { 'data-toggle': 'dropdown',
                onClick: this.onShowLoginsClick,
                className: 'btn btn-default btn-xs dropdown-toggle' },
              react_default.a.createElement('span', { className: 'caret' })
            ),
            react_default.a.createElement(
              'ul',
              { className: 'dropdown-menu pull-right' },
              react_default.a.createElement(
                'li',
                null,
                react_default.a.createElement(
                  'div',
                  { className: 'input-group-sm grv-nodes-custom-login' },
                  react_default.a.createElement('input', { className: 'form-control', ref: 'customLogin',
                    placeholder: 'Enter login name...',
                    onKeyPress: this.onKeyPress,
                    autoFocus: true
                  })
                )
              ),
              $lis
            )
          )
        )
      );
    }
  }]);

  return LoginCell;
}(react_default.a.Component);

var nodeList_NodeList = function (_React$Component2) {
  nodeList_inherits(NodeList, _React$Component2);

  function NodeList(props) {
    nodeList_classCallCheck(this, NodeList);

    var _this2 = nodeList_possibleConstructorReturn(this, (NodeList.__proto__ || Object.getPrototypeOf(NodeList)).call(this, props));

    _this2.storageKey = 'NodeList';
    _this2.searchableProps = ['addr', 'hostname', 'tags'];

    _this2.onSortChange = function (columnKey, sortDir) {
      _this2.state.colSortDirs = nodeList_defineProperty({}, columnKey, sortDir);
      _this2.setState(_this2.state);
    };

    _this2.onFilterChange = function (value) {
      _this2.state.filter = value;
      _this2.setState(_this2.state);
    };

    _this2.onSshInputEnter = function (login, host) {
      var url = config["a" /* default */].getTerminalLoginUrl({
        siteId: _this2.props.siteId,
        serverId: host,
        login: login
      });

      services_history.push(url);
    };

    if (props.storage) {
      _this2.state = props.storage.findByKey(_this2.storageKey);
    }

    if (!_this2.state) {
      _this2.state = { filter: '', colSortDirs: { hostname: SortTypes.DESC } };
    }
    return _this2;
  }

  nodeList_createClass(NodeList, [{
    key: 'componentWillUnmount',
    value: function componentWillUnmount() {
      if (this.props.storage) {
        this.props.storage.save(this.storageKey, this.state);
      }
    }
  }, {
    key: 'searchAndFilterCb',
    value: function searchAndFilterCb(targetValue, searchValue, propName) {
      if (propName === 'tags') {
        return targetValue.some(function (item) {
          var name = item.name,
              value = item.value;

          return name.toLocaleUpperCase().indexOf(searchValue) !== -1 || value.toLocaleUpperCase().indexOf(searchValue) !== -1;
        });
      }
    }
  }, {
    key: 'sortAndFilter',
    value: function sortAndFilter(data) {
      var _this3 = this;

      var colSortDirs = this.state.colSortDirs;

      var filtered = data.filter(function (obj) {
        return isMatch(obj, _this3.state.filter, {
          searchableProps: _this3.searchableProps,
          cb: _this3.searchAndFilterCb
        });
      });

      var columnKey = Object.getOwnPropertyNames(colSortDirs)[0];
      var sortDir = colSortDirs[columnKey];
      var sorted = Object(lodash["sortBy"])(filtered, columnKey);
      if (sortDir === SortTypes.ASC) {
        sorted = sorted.reverse();
      }

      return sorted;
    }
  }, {
    key: 'render',
    value: function render() {
      var _props3 = this.props,
          sshHistory = _props3.sshHistory,
          siteId = _props3.siteId,
          nodeRecords = _props3.nodeRecords,
          logins = _props3.logins,
          onLoginClick = _props3.onLoginClick;

      var searchValue = this.state.filter;
      var data = this.sortAndFilter(nodeRecords);
      return react_default.a.createElement(
        'div',
        { className: 'grv-nodes m-t' },
        react_default.a.createElement(
          'div',
          { className: 'grv-flex grv-header', style: { justifyContent: "space-between" } },
          react_default.a.createElement(
            'h2',
            { className: 'text-center no-margins' },
            ' Nodes '
          ),
          react_default.a.createElement(
            'div',
            { className: 'grv-flex' },
            react_default.a.createElement(clusterSelector, null),
            react_default.a.createElement(inputSearch, { autoFocus: true, value: searchValue, onChange: this.onFilterChange }),
            react_default.a.createElement(inputSshServer, {
              clusterId: siteId,
              sshHistory: sshHistory,
              onEnter: this.onSshInputEnter })
          )
        ),
        react_default.a.createElement(
          'div',
          { className: 'm-t' },
          data.length === 0 && this.state.filter.length > 0 ? react_default.a.createElement(table_EmptyIndicator, { text: 'No matching nodes found' }) : react_default.a.createElement(
            PagedTable,
            { className: 'grv-nodes-table', tableClass: 'table-striped', data: data, pageSize: 100 },
            react_default.a.createElement(Column, {
              columnKey: 'hostname',
              header: react_default.a.createElement(table_SortHeaderCell, {
                sortDir: this.state.colSortDirs.hostname,
                onSortChange: this.onSortChange,
                title: 'Hostname'
              }),
              cell: react_default.a.createElement(table_TableTextCell, null)
            }),
            react_default.a.createElement(Column, {
              columnKey: 'addr',
              header: react_default.a.createElement(table_SortHeaderCell, {
                sortDir: this.state.colSortDirs.addr,
                onSortChange: this.onSortChange,
                title: 'Address'
              }),
              cell: react_default.a.createElement(table_TableTextCell, null)
            }),
            react_default.a.createElement(Column, {
              header: react_default.a.createElement(
                table_TableCell,
                null,
                'Labels'
              ),
              cell: react_default.a.createElement(nodeList_TagCell, null)
            }),
            react_default.a.createElement(Column, {
              className: 'grv-nodes-table-login',
              onLoginClick: onLoginClick,
              header: react_default.a.createElement(
                table_TableCell,
                null,
                'Login as'
              ),
              cell: react_default.a.createElement(nodeList_LoginCell, { logins: logins })
            })
          )
        )
      );
    }
  }]);

  return NodeList;
}(react_default.a.Component);

/* harmony default export */ var nodeList = (nodeList_NodeList);
// EXTERNAL MODULE: ./node_modules/nuclear-js/dist/nuclear.js
var nuclear = __webpack_require__("L7e8");

// EXTERNAL MODULE: ./node_modules/immutable/dist/immutable.js
var immutable = __webpack_require__("JPcv");

// CONCATENATED MODULE: ./src/app/flux/sshHistory/actionTypes.js
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
var ADD_ITEM = 'TLPT_SSH_HISTORY';
// CONCATENATED MODULE: ./src/app/flux/sshHistory/store.js
var store_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function store_defineProperty(obj, key, value) { if (key in obj) { Object.defineProperty(obj, key, { value: value, enumerable: true, configurable: true, writable: true }); } else { obj[key] = value; } return obj; }

function store_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function store_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function store_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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





var store_STORE_NAME = 'tlpt_ssh_history';

var SshHistoryRec = function (_Record) {
  store_inherits(SshHistoryRec, _Record);

  function SshHistoryRec(params) {
    store_classCallCheck(this, SshHistoryRec);

    return store_possibleConstructorReturn(this, (SshHistoryRec.__proto__ || Object.getPrototypeOf(SshHistoryRec)).call(this, params));
  }

  store_createClass(SshHistoryRec, [{
    key: 'getPrevLogins',
    value: function getPrevLogins(siteId) {
      return this.clusterLogins.get(siteId) || [];
    }
  }, {
    key: 'addLoginString',
    value: function addLoginString(_ref) {
      var login = _ref.login,
          serverId = _ref.serverId,
          siteId = _ref.siteId;

      var logins = this.getIn(['clusterLogins', siteId]);
      if (!logins) {
        logins = [];
      }

      var loginStr = login + '@' + serverId;
      var exists = logins.some(function (i) {
        return i === loginStr;
      });

      if (exists) {
        return this;
      }

      logins.unshift(loginStr);
      return this.setIn(['clusterLogins', siteId], logins);
    }
  }]);

  return SshHistoryRec;
}(Object(immutable["Record"])({
  clusterLogins: new immutable["Map"]()
}));

var store_store = Object(nuclear["Store"])({
  getInitialState: function getInitialState() {
    return new SshHistoryRec();
  },
  initialize: function initialize() {
    this.on(ADD_ITEM, function (state, params) {
      return state.addLoginString(params);
    });
  }
});

var store_register = function register(reactor) {
  reactor.registerStores(store_defineProperty({}, store_STORE_NAME, store_store));
};

var store_getters = {
  store: [store_STORE_NAME]
};
// CONCATENATED MODULE: ./src/app/flux/misc/store.js
function misc_store_defineProperty(obj, key, value) { if (key in obj) { Object.defineProperty(obj, key, { value: value, enumerable: true, configurable: true, writable: true }); } else { obj[key] = value; } return obj; }

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




var SET = 'MISC_SET';
var misc_store_STORE_NAME = 'tlpt_misc';

// stores any temporary data
var misc_store_store = Object(nuclear["Store"])({
  getInitialState: function getInitialState() {
    return new nuclear["Immutable"].Map();
  },
  initialize: function initialize() {
    this.on(SET, function (state, _ref) {
      var key = _ref.key,
          payload = _ref.payload;
      return state.set(key, payload);
    });
  }
});

var misc_store_register = function register(reactor) {
  reactor.registerStores(misc_store_defineProperty({}, misc_store_STORE_NAME, misc_store_store));
};

var store_storage = {
  save: function save(key, payload) {
    app_reactor["a" /* default */].dispatch(SET, { key: key, payload: payload });
  },
  findByKey: function findByKey(key) {
    return app_reactor["a" /* default */].evaluate([misc_store_STORE_NAME, key]);
  }
};
// CONCATENATED MODULE: ./src/app/components/withStorage.jsx
var withStorage_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

var withStorage_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function withStorage_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function withStorage_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function withStorage_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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




var withStorage_withStorage = function withStorage(component) {
  var _class, _temp;

  return _temp = _class = function (_React$Component) {
    withStorage_inherits(WithTmpStorageWrapper, _React$Component);

    function WithTmpStorageWrapper(props, context) {
      withStorage_classCallCheck(this, WithTmpStorageWrapper);

      return withStorage_possibleConstructorReturn(this, (WithTmpStorageWrapper.__proto__ || Object.getPrototypeOf(WithTmpStorageWrapper)).call(this, props, context));
    }

    withStorage_createClass(WithTmpStorageWrapper, [{
      key: 'render',
      value: function render() {
        var props = this.props;
        return react_default.a.createElement(component, withStorage_extends({}, props, {
          storage: store_storage
        }));
      }
    }]);

    return WithTmpStorageWrapper;
  }(react_default.a.Component), _class.displayName = 'withTmpStorageWrapper', _temp;
};

/* harmony default export */ var components_withStorage = (withStorage_withStorage);
// CONCATENATED MODULE: ./src/app/components/nodes/main.jsx
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











var main_Nodes = function Nodes(props) {
  var siteNodes = props.siteNodes,
      sshHistory = props.sshHistory,
      aclStore = props.aclStore,
      sites = props.sites,
      siteId = props.siteId,
      storage = props.storage;

  var logins = aclStore.getSshLogins().toJS();
  var nodeRecords = siteNodes.toJS();
  var title = siteId + ' \xB7 Nodes';
  return react_default.a.createElement(
    'div',
    { className: 'grv-page' },
    react_default.a.createElement(
      DocumentTitle,
      { title: title },
      react_default.a.createElement(nodeList, {
        sshHistory: sshHistory,
        storage: storage,
        siteId: siteId,
        sites: sites,
        nodeRecords: nodeRecords,
        logins: logins
      })
    )
  );
};

function main_mapStateToProps() {
  return {
    siteId: app_getters["a" /* default */].siteId,
    siteNodes: nodes_getters.siteNodes,
    aclStore: userAcl_getters.userAcl,
    sshHistory: store_getters.store
  };
}

var NodesWithStorage = components_withStorage(main_Nodes);

/* harmony default export */ var main = (Object(build["connect"])(main_mapStateToProps)(NodesWithStorage));
// CONCATENATED MODULE: ./src/app/components/indicator.jsx
var indicator_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function indicator_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function indicator_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function indicator_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }



var WHEN_TO_DISPLAY = 100; // 0.2s;

var indicator_Indicator = function (_React$Component) {
  indicator_inherits(Indicator, _React$Component);

  function Indicator(props) {
    indicator_classCallCheck(this, Indicator);

    var _this = indicator_possibleConstructorReturn(this, (Indicator.__proto__ || Object.getPrototypeOf(Indicator)).call(this, props));

    _this._timer = null;
    _this.state = {
      canDisplay: false
    };
    return _this;
  }

  indicator_createClass(Indicator, [{
    key: 'componentDidMount',
    value: function componentDidMount() {
      var _this2 = this;

      this._timer = setTimeout(function () {
        _this2.setState({
          canDisplay: true
        });
      }, WHEN_TO_DISPLAY);
    }
  }, {
    key: 'componentWillUnmount',
    value: function componentWillUnmount() {
      clearTimeout(this._timer);
    }
  }, {
    key: 'render',
    value: function render() {
      var _props$type = this.props.type,
          type = _props$type === undefined ? 'bounce' : _props$type;


      if (!this.state.canDisplay) {
        return null;
      }

      if (type === 'bounce') {
        return react_default.a.createElement(indicator_ThreeBounce, null);
      }
    }
  }]);

  return Indicator;
}(react_default.a.Component);

var indicator_ThreeBounce = function ThreeBounce() {
  return react_default.a.createElement(
    'div',
    { className: 'grv-spinner sk-spinner sk-spinner-three-bounce' },
    react_default.a.createElement('div', { className: 'sk-bounce1' }),
    react_default.a.createElement('div', { className: 'sk-bounce2' }),
    react_default.a.createElement('div', { className: 'sk-bounce3' })
  );
};

/* harmony default export */ var indicator = (indicator_Indicator);
// CONCATENATED MODULE: ./src/app/components/withFeature.jsx
var withFeature_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

var withFeature_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function withFeature_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function withFeature_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function withFeature_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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








var withFeature_logger = logger["a" /* default */].create('components/withFeature');

var withFeature_withFeature = function withFeature(feature) {
  return function (component) {
    var _class, _temp;

    return _temp = _class = function (_React$Component) {
      withFeature_inherits(WithFeatureWrapper, _React$Component);

      function WithFeatureWrapper(props, context) {
        withFeature_classCallCheck(this, WithFeatureWrapper);

        var _this = withFeature_possibleConstructorReturn(this, (WithFeatureWrapper.__proto__ || Object.getPrototypeOf(WithFeatureWrapper)).call(this, props, context));

        _this._unsubscribeFn = null;
        return _this;
      }

      withFeature_createClass(WithFeatureWrapper, [{
        key: 'componentDidMount',
        value: function componentDidMount() {
          var _this2 = this;

          try {
            this._unsubscribeFn = app_reactor["a" /* default */].observe(feature.initAttemptGetter, function () {
              _this2.setState({});
            });

            app_reactor["a" /* default */].batch(function () {
              feature.componentDidMount();
            });
          } catch (err) {
            withFeature_logger.error('failed to initialize a feature', err);
          }
        }
      }, {
        key: 'componentWillUnmount',
        value: function componentWillUnmount() {
          this._unsubscribeFn();
        }
      }, {
        key: 'render',
        value: function render() {
          if (feature.isProcessing()) {
            return react_default.a.createElement(indicator, { delay: 'long', type: 'bounce' });
          }

          if (feature.isFailed()) {
            var errorText = feature.getErrorText();
            if (feature.getErrorCode() === RestRespCodeEnum.FORBIDDEN) {
              return react_default.a.createElement(msgPage_AccessDenied, { message: errorText });
            }
            return react_default.a.createElement(msgPage_Failed, { message: errorText });
          }

          if (!feature.wasInitialized()) {
            return null;
          }

          var props = this.props;
          return react_default.a.createElement(component, withFeature_extends({}, props, {
            feature: feature
          }));
        }
      }]);

      return WithFeatureWrapper;
    }(react_default.a.Component), _class.displayName = 'withFeatureWrapper', _temp;
  };
};

/* harmony default export */ var components_withFeature = (withFeature_withFeature);
// CONCATENATED MODULE: ./src/app/featureBase.js
var featureBase_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function featureBase_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }










var _featureId = 0;

var ensureActionType = function ensureActionType(actionType) {
  if (!actionType) {
    ++_featureId;
    return 'TRYING_TO_INIT_FEATURE_' + _featureId;
  }

  return actionType;
};

var featureBase_FeatureBase = function () {
  function FeatureBase(actionType) {
    featureBase_classCallCheck(this, FeatureBase);

    actionType = ensureActionType(actionType);
    this.initStatus = makeStatus(ensureActionType(actionType));
    this.initAttemptGetter = Object(getters["e" /* makeGetter */])(actionType);
  }

  featureBase_createClass(FeatureBase, [{
    key: 'preload',
    value: function preload() {
      return jquery_default.a.Deferred().resolve();
    }
  }, {
    key: 'onload',
    value: function onload() {}
  }, {
    key: 'startProcessing',
    value: function startProcessing() {
      this.initStatus.start();
    }
  }, {
    key: 'stopProcessing',
    value: function stopProcessing() {
      this.initStatus.success();
    }
  }, {
    key: 'isReady',
    value: function isReady() {
      return this._getInitAttempt().isSuccess;
    }
  }, {
    key: 'isProcessing',
    value: function isProcessing() {
      return this._getInitAttempt().isProcessing;
    }
  }, {
    key: 'isFailed',
    value: function isFailed() {
      return this._getInitAttempt().isFailed;
    }
  }, {
    key: 'wasInitialized',
    value: function wasInitialized() {
      var attempt = this._getInitAttempt();
      return attempt.isFailed || attempt.isProcessing || attempt.isSuccess;
    }
  }, {
    key: 'componentDidMount',
    value: function componentDidMount() {}
  }, {
    key: 'getErrorText',
    value: function getErrorText() {
      var _getInitAttempt2 = this._getInitAttempt(),
          message = _getInitAttempt2.message;

      return Object(lodash["isObject"])(message) ? message.text : message;
    }
  }, {
    key: 'getErrorCode',
    value: function getErrorCode() {
      var _getInitAttempt3 = this._getInitAttempt(),
          message = _getInitAttempt3.message;

      return Object(lodash["isObject"])(message) ? message.code : null;
    }
  }, {
    key: 'handleAccesDenied',
    value: function handleAccesDenied() {
      this.handleError(new Error('Access Denied'));
    }
  }, {
    key: 'handleError',
    value: function handleError(err) {
      var message = api["a" /* default */].getErrorText(err);
      if (err.status === RestRespCodeEnum.FORBIDDEN) {
        message = {
          code: RestRespCodeEnum.FORBIDDEN,
          text: message
        };
      }

      this.initStatus.fail(message);
    }
  }, {
    key: 'withMe',
    value: function withMe(component) {
      return components_withFeature(this)(component);
    }
  }, {
    key: '_getInitAttempt',
    value: function _getInitAttempt() {
      return app_reactor["a" /* default */].evaluate(this.initAttemptGetter);
    }
  }]);

  return FeatureBase;
}();

/* harmony default export */ var featureBase = (featureBase_FeatureBase);
// CONCATENATED MODULE: ./src/app/flux/terminal/getters.js
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

/* harmony default export */ var terminal_getters = ({
  store: ['tlpt_terminal']
});
// CONCATENATED MODULE: ./src/app/flux/fileTransfer/actionTypes.jsx
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

var OPEN = '/flux/fileTransfer:open';
var CLOSE = '/flux/fileTransfer:close';
var ADD = '/flux/fileTransfer:add';
var REMOVE = '/flux/fileTransfer:remove';
var UPDATE_STATUS = '/flux/fileTransfer:status';
// CONCATENATED MODULE: ./src/app/flux/fileTransfer/store.js
var fileTransfer_store_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

var store_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

function store_objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }

function fileTransfer_store_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function fileTransfer_store_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function fileTransfer_store_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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






var store_defaultStatus = {
  isFailed: false,
  isProcessing: false,
  isCompleted: false,
  error: ""
};

var store_File = function (_Record) {
  fileTransfer_store_inherits(File, _Record);

  function File(props) {
    fileTransfer_store_classCallCheck(this, File);

    props = store_extends({}, props, {
      id: new Date().getTime() + props.name
    });
    return fileTransfer_store_possibleConstructorReturn(this, (File.__proto__ || Object.getPrototypeOf(File)).call(this, props));
  }

  return File;
}(Object(immutable["Record"])(store_extends({
  id: '',
  url: '',
  name: '',
  isUpload: '',
  blob: null
}, store_defaultStatus)));

var store_FileTransferStore = function (_Record2) {
  fileTransfer_store_inherits(FileTransferStore, _Record2);

  function FileTransferStore(params) {
    fileTransfer_store_classCallCheck(this, FileTransferStore);

    return fileTransfer_store_possibleConstructorReturn(this, (FileTransferStore.__proto__ || Object.getPrototypeOf(FileTransferStore)).call(this, params));
  }

  fileTransfer_store_createClass(FileTransferStore, [{
    key: 'open',
    value: function open(_ref) {
      var isUpload = _ref.isUpload,
          siteId = _ref.siteId,
          serverId = _ref.serverId,
          login = _ref.login;

      return this.merge({
        isOpen: true,
        isUpload: isUpload,
        siteId: siteId,
        serverId: serverId,
        login: login
      });
    }
  }, {
    key: 'close',
    value: function close() {
      return new FileTransferStore();
    }
  }, {
    key: 'makeUrl',
    value: function makeUrl(location, filename) {
      var siteId = this.siteId,
          serverId = this.serverId,
          login = this.login;


      var url = config["a" /* default */].api.getScpUrl({
        siteId: siteId,
        serverId: serverId,
        login: login,
        location: location,
        filename: filename
      });

      return url;
    }
  }, {
    key: 'removeFile',
    value: function removeFile(id) {
      var files = this.files.delete(id);
      return this.set('files', files);
    }
  }, {
    key: 'addFile',
    value: function addFile(_ref2) {
      var location = _ref2.location,
          name = _ref2.name,
          blob = _ref2.blob,
          isUpload = _ref2.isUpload;

      var url = this.makeUrl(location, name);
      var file = new store_File({
        url: url,
        name: name,
        isUpload: isUpload,
        blob: blob
      });

      return this.update('files', function (files) {
        return files.set(file.id, file);
      });
    }
  }, {
    key: 'updateStatus',
    value: function updateStatus(_ref3) {
      var id = _ref3.id,
          rest = store_objectWithoutProperties(_ref3, ['id']);

      var file = this.files.get(id);
      var status = store_extends({}, store_defaultStatus, rest);

      file = file.merge(status);
      return this.update('files', function (files) {
        return files.set(id, file);
      });
    }
  }, {
    key: 'isTransfering',
    value: function isTransfering() {
      return this.files.some(function (f) {
        return f.isProcessing === true;
      });
    }
  }]);

  return FileTransferStore;
}(Object(immutable["Record"])({
  isOpen: false,
  isUpload: false,
  siteId: '',
  serverId: '',
  login: '',
  files: new immutable["OrderedMap"]()
}));

/* harmony default export */ var fileTransfer_store = (Object(nuclear["Store"])({
  getInitialState: function getInitialState() {
    return new store_FileTransferStore();
  },
  initialize: function initialize() {
    this.on(OPEN, function (state, json) {
      return state.open(json);
    });
    this.on(CLOSE, function (state) {
      return state.close();
    });
    this.on(ADD, function (state, json) {
      return state.addFile(json);
    });
    this.on(REMOVE, function (state, id) {
      return state.removeFile(id);
    });
    this.on(UPDATE_STATUS, function (state, json) {
      return state.updateStatus(json);
    });
  }
}));
// CONCATENATED MODULE: ./src/app/flux/fileTransfer/index.js
function fileTransfer_defineProperty(obj, key, value) { if (key in obj) { Object.defineProperty(obj, key, { value: value, enumerable: true, configurable: true, writable: true }); } else { obj[key] = value; } return obj; }

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




var fileTransfer_STORE_NAME = 'tlpt_files';

function getFileTransfer() {
  return app_reactor["a" /* default */].evaluate([fileTransfer_STORE_NAME]);
}

var fileTransfer_register = function register(reactor) {
  reactor.registerStores(fileTransfer_defineProperty({}, fileTransfer_STORE_NAME, fileTransfer_store));
};

var fileTransfer_getters = {
  store: [fileTransfer_STORE_NAME]
};
// CONCATENATED MODULE: ./src/app/flux/nodes/nodeStore.js
var nodeStore_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

var nodeStore_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

function nodeStore_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function nodeStore_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function nodeStore_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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






var nodeStore_ServerRec = function (_Record) {
  nodeStore_inherits(ServerRec, _Record);

  function ServerRec(props) {
    nodeStore_classCallCheck(this, ServerRec);

    var tags = new immutable["List"](Object(nuclear["toImmutable"])(props.tags));
    return nodeStore_possibleConstructorReturn(this, (ServerRec.__proto__ || Object.getPrototypeOf(ServerRec)).call(this, nodeStore_extends({}, props, {
      tags: tags
    })));
  }

  return ServerRec;
}(Object(immutable["Record"])({
  id: '',
  siteId: '',
  hostname: '',
  tags: new immutable["List"](),
  addr: ''
}));

var nodeStore_NodeStoreRec = function (_Record2) {
  nodeStore_inherits(NodeStoreRec, _Record2);

  function NodeStoreRec() {
    nodeStore_classCallCheck(this, NodeStoreRec);

    return nodeStore_possibleConstructorReturn(this, (NodeStoreRec.__proto__ || Object.getPrototypeOf(NodeStoreRec)).apply(this, arguments));
  }

  nodeStore_createClass(NodeStoreRec, [{
    key: 'findServer',
    value: function findServer(serverId) {
      return this.servers.find(function (s) {
        return s.id === serverId;
      });
    }
  }, {
    key: 'getSiteServers',
    value: function getSiteServers(siteId) {
      return this.servers.filter(function (s) {
        return s.siteId === siteId;
      });
    }
  }, {
    key: 'addSiteServers',
    value: function addSiteServers(jsonItems) {
      var list = new immutable["List"]().withMutations(function (state) {
        jsonItems.forEach(function (item) {
          return state.push(new nodeStore_ServerRec(item));
        });
        return state;
      });

      return list.equals(this.servers) ? this : this.set('servers', list);
    }
  }]);

  return NodeStoreRec;
}(Object(immutable["Record"])({
  servers: new immutable["List"]()
}));

function getNodeStore() {
  return app_reactor["a" /* default */].evaluate(['tlpt_nodes']);
}

/* harmony default export */ var nodeStore = (Object(nuclear["Store"])({
  getInitialState: function getInitialState() {
    return new nodeStore_NodeStoreRec();
  },
  initialize: function initialize() {
    this.on(TLPT_NODES_RECEIVE, function (state, items) {
      return state.addSiteServers(items);
    });
  }
}));
// EXTERNAL MODULE: ./node_modules/moment/moment.js
var moment = __webpack_require__("wd/R");
var moment_default = /*#__PURE__*/__webpack_require__.n(moment);

// CONCATENATED MODULE: ./src/app/lib/term/enums.js
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

var EventTypeEnum = {
  START: 'session.start',
  JOIN: 'session.join',
  END: 'session.end',
  PRINT: 'print',
  RESIZE: 'resize'
};

var TermEventEnum = {
  RESIZE: 'terminal.resize',
  CLOSE: 'terminal.close',
  RESET: 'terminal.reset',
  DATA: 'terminal.data',
  CONN_CLOSE: 'connection.close'
};

var StatusCodeEnum = {
  NORMAL: 1000
};
// CONCATENATED MODULE: ./src/app/flux/sessions/getters.js
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








/*
** Getters
*/
var activeSessionList = [['tlpt_sessions_active'], ['tlpt', 'siteId'], function (sessionList, siteId) {
  sessionList = sessionList.filter(function (n) {
    return n.get('siteId') === siteId;
  });
  return sessionList.valueSeq().map(createActiveListItem).toJS();
}];

var storedSessionList = [['tlpt_sessions_archived'], ['tlpt', 'siteId'], function (sessionList, siteId) {
  sessionList = sessionList.filter(function (n) {
    return n.get('siteId') === siteId;
  });
  return sessionList.valueSeq().map(createStoredListItem).toJS();
}];

var getters_nodeIpById = function nodeIpById(sid) {
  return ['tlpt_sessions_events', sid, EventTypeEnum.START, 'addr.local'];
};
var storedSessionById = function storedSessionById(sid) {
  return ['tlpt_sessions_archived', sid];
};
var activeSessionById = function activeSessionById(sid) {
  return ['tlpt_sessions_active', sid];
};
var activePartiesById = function activePartiesById(sid) {
  return [['tlpt_sessions_active', sid, 'parties'], function (parties) {
    return parties ? parties.toJS() : [];
  }];
};

// creates a list of stored sessions which involves collecting the data from other stores
function createStoredListItem(session) {
  var sid = session.get('id');
  var siteId = session.siteId,
      nodeIp = session.nodeIp,
      created = session.created,
      server_id = session.server_id,
      parties = session.parties,
      last_active = session.last_active;

  var duration = moment_default()(last_active).diff(created);
  var nodeDisplayText = getNodeIpDisplayText(siteId, server_id, nodeIp);
  var createdDisplayText = getCreatedDisplayText(created);
  var sessionUrl = config["a" /* default */].getPlayerUrl({
    sid: sid,
    siteId: siteId
  });

  return {
    active: false,
    parties: createParties(parties),
    sid: sid,
    duration: duration,
    siteId: siteId,
    sessionUrl: sessionUrl,
    created: created,
    createdDisplayText: createdDisplayText,
    nodeDisplayText: nodeDisplayText,
    lastActive: last_active
  };
}

// creates a list of active sessions which involves collecting the data from other stores
function createActiveListItem(session) {
  var sid = session.get('id');
  var parties = createParties(session.parties);
  var siteId = session.siteId,
      created = session.created,
      login = session.login,
      last_active = session.last_active,
      server_id = session.server_id;

  var duration = moment_default()(last_active).diff(created);
  var nodeIp = app_reactor["a" /* default */].evaluate(getters_nodeIpById(sid));
  var nodeDisplayText = getNodeIpDisplayText(siteId, server_id, nodeIp);
  var createdDisplayText = getCreatedDisplayText(created);
  var sessionUrl = config["a" /* default */].getTerminalLoginUrl({
    sid: sid,
    siteId: siteId,
    login: login,
    serverId: server_id
  });

  return {
    active: true,
    parties: parties,
    sid: sid,
    duration: duration,
    siteId: siteId,
    sessionUrl: sessionUrl,
    created: created,
    createdDisplayText: createdDisplayText,
    nodeDisplayText: nodeDisplayText,
    lastActive: last_active
  };
}

function createParties(partyRecs) {
  var parties = partyRecs.toJS();
  return parties.map(function (p) {
    var ip = parseIp(p.serverIp);
    return p.user + ' [' + ip + ']';
  });
}

function getCreatedDisplayText(date) {
  return moment_default()(date).format(config["a" /* default */].displayDateFormat);
}

function getNodeIpDisplayText(siteId, serverId, serverIp) {
  var server = getNodeStore().findServer(serverId);
  var ipAddress = parseIp(serverIp);

  var displayText = ipAddress;
  if (server && server.hostname) {
    displayText = server.hostname;
    if (ipAddress) {
      displayText = displayText + ' [' + ipAddress + ']';
    }
  }

  return displayText;
}

/* harmony default export */ var sessions_getters = ({
  storedSessionList: storedSessionList,
  activeSessionList: activeSessionList,
  activeSessionById: activeSessionById,
  activePartiesById: activePartiesById,
  storedSessionById: storedSessionById,
  createStoredListItem: createStoredListItem
});
// CONCATENATED MODULE: ./src/app/flux/terminal/actionTypes.js
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
var TLPT_TERMINAL_INIT = 'TLPT_TERMINAL_INIT';
var TLPT_TERMINAL_CLOSE = 'TLPT_TERMINAL_CLOSE';
var TLPT_TERMINAL_SET_STATUS = 'TLPT_TERMINAL_SET_STATUS';
// CONCATENATED MODULE: ./src/app/flux/sshHistory/actions.js
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



var actions_saveSshLogin = function saveSshLogin(_ref) {
    var login = _ref.login,
        serverId = _ref.serverId,
        siteId = _ref.siteId;

    app_reactor["a" /* default */].dispatch(ADD_ITEM, { login: login, serverId: serverId, siteId: siteId });
};
// CONCATENATED MODULE: ./src/app/flux/terminal/actions.js
var actions_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

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










var terminal_actions_logger = logger["a" /* default */].create('flux/terminal');

var actions_setStatus = function setStatus(json) {
  return app_reactor["a" /* default */].dispatch(TLPT_TERMINAL_SET_STATUS, json);
};

function initStore(params) {
  var serverId = params.serverId;

  var server = getNodeStore().findServer(serverId);
  var hostname = server ? server.hostname : '';
  app_reactor["a" /* default */].dispatch(TLPT_TERMINAL_INIT, actions_extends({}, params, {
    hostname: hostname
  }));
}

function createSid(routeParams) {
  var login = routeParams.login,
      siteId = routeParams.siteId;

  var data = {
    session: {
      terminal_params: {
        w: 45,
        h: 5
      },
      login: login
    }
  };

  return api["a" /* default */].post(config["a" /* default */].api.getSiteSessionUrl(siteId), data);
}

function initTerminal(routeParams) {
  terminal_actions_logger.info('attempt to open a terminal', routeParams);

  var sid = routeParams.sid;


  actions_setStatus({ isLoading: true });

  if (sid) {
    var activeSession = app_reactor["a" /* default */].evaluate(sessions_getters.activeSessionById(sid));
    if (activeSession) {
      // init store with existing sid
      initStore(routeParams);
      actions_setStatus({ isReady: true });
    } else {
      actions_setStatus({ isNotFound: true });
    }

    return;
  }

  createSid(routeParams).done(function (json) {
    var sid = json.session.id;
    var newRouteParams = actions_extends({}, routeParams, {
      sid: sid
    });
    initStore(newRouteParams);
    actions_setStatus({ isReady: true });
    updateRoute(newRouteParams);

    actions_saveSshLogin(routeParams);
  }).fail(function (err) {
    var errorText = api["a" /* default */].getErrorText(err);
    actions_setStatus({ isError: true, errorText: errorText });
  });
}

function actions_close() {
  app_reactor["a" /* default */].dispatch(TLPT_TERMINAL_CLOSE);
  services_history.push(config["a" /* default */].routes.nodes);
}

function updateRoute(newRouteParams) {
  var routeUrl = config["a" /* default */].getTerminalLoginUrl(newRouteParams);
  services_history.push(routeUrl);
}
// CONCATENATED MODULE: ./src/app/flux/userAcl/store.js
var userAcl_store_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function userAcl_store_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function userAcl_store_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function userAcl_store_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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






// sort logins by making 'root' as the first in the list
var sortLogins = function sortLogins(loginList) {
  var index = loginList.indexOf('root');
  if (index !== -1) {
    loginList = loginList.remove(index);
    return loginList.sort().unshift('root');
  }

  return loginList;
};

var Access = new immutable["Record"]({
  list: false,
  read: false,
  edit: false,
  create: false,
  remove: false
});

var store_AccessListRec = function (_Record) {
  userAcl_store_inherits(AccessListRec, _Record);

  function AccessListRec() {
    var json = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : {};

    userAcl_store_classCallCheck(this, AccessListRec);

    var map = Object(nuclear["toImmutable"])(json);
    var sshLogins = new immutable["List"](map.get('sshLogins'));
    var params = {
      sshLogins: sortLogins(sshLogins),
      authConnectors: new Access(map.get('authConnectors')),
      trustedClusters: new Access(map.get('trustedClusters')),
      roles: new Access(map.get('roles')),
      sessions: new Access(map.get('sessions'))
    };

    return userAcl_store_possibleConstructorReturn(this, (AccessListRec.__proto__ || Object.getPrototypeOf(AccessListRec)).call(this, params));
  }

  userAcl_store_createClass(AccessListRec, [{
    key: 'getSessionAccess',
    value: function getSessionAccess() {
      return this.get('sessions');
    }
  }, {
    key: 'getRoleAccess',
    value: function getRoleAccess() {
      return this.get('roles');
    }
  }, {
    key: 'getConnectorAccess',
    value: function getConnectorAccess() {
      return this.get('authConnectors');
    }
  }, {
    key: 'getClusterAccess',
    value: function getClusterAccess() {
      return this.get('trustedClusters');
    }
  }, {
    key: 'getSshLogins',
    value: function getSshLogins() {
      return this.get('sshLogins');
    }
  }]);

  return AccessListRec;
}(Object(immutable["Record"])({
  authConnectors: new Access(),
  trustedClusters: new Access(),
  roles: new Access(),
  sessions: new Access(),
  sshLogins: new immutable["List"]()
}));

function getAcl() {
  return app_reactor["a" /* default */].evaluate(['tlpt_user_acl']);
}

/* harmony default export */ var userAcl_store = (Object(nuclear["Store"])({
  getInitialState: function getInitialState() {
    return new store_AccessListRec();
  },
  initialize: function initialize() {
    this.on(RECEIVE_USERACL, function (state, json) {
      return new store_AccessListRec(json);
    });
  }
}));
// CONCATENATED MODULE: ./src/app/flux/player/actions.js
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





function actions_open(siteId, sid) {
  var routeUrl = config["a" /* default */].getPlayerUrl({ siteId: siteId, sid: sid });
  services_history.push(routeUrl);
}

function player_actions_close() {
  var canListSessions = getAcl().getSessionAccess().read;
  var redirect = canListSessions ? config["a" /* default */].routes.sessions : config["a" /* default */].routes.app;
  services_history.push(redirect);
}
// CONCATENATED MODULE: ./src/app/flux/fileTransfer/actions.js
var fileTransfer_actions_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

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




function addFile(json) {
  app_reactor["a" /* default */].dispatch(ADD, json);
}

function removeFile(id) {
  app_reactor["a" /* default */].dispatch(REMOVE, id);
}

function updateStatus(json) {
  app_reactor["a" /* default */].dispatch(UPDATE_STATUS, json);
}

function openUploadDialog(params) {
  var json = fileTransfer_actions_extends({}, params, {
    isUpload: true
  });
  app_reactor["a" /* default */].dispatch(OPEN, json);
}

function openDownloadDialog(params) {
  var json = fileTransfer_actions_extends({}, params, {
    isUpload: false
  });
  app_reactor["a" /* default */].dispatch(OPEN, json);
}

function closeDialog() {
  app_reactor["a" /* default */].dispatch(CLOSE, {});
}
// EXTERNAL MODULE: ./node_modules/hoist-non-react-statics/index.js
var hoist_non_react_statics = __webpack_require__("/LiH");
var hoist_non_react_statics_default = /*#__PURE__*/__webpack_require__.n(hoist_non_react_statics);

// CONCATENATED MODULE: ./src/app/components/connect.jsx
var connect_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

var connect_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function connect_defineProperty(obj, key, value) { if (key in obj) { Object.defineProperty(obj, key, { value: value, enumerable: true, configurable: true, writable: true }); } else { obj[key] = value; } return obj; }

function connect_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function connect_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function connect_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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




var reactorShape = react["PropTypes"].shape({
  dispatch: react["PropTypes"].func.isRequired,
  evaluate: react["PropTypes"].func.isRequired,
  evaluateToJS: react["PropTypes"].func.isRequired,
  observe: react["PropTypes"].func.isRequired
});

function getDisplayName(WrappedComponent) {
  return WrappedComponent.displayName || WrappedComponent.name || 'Component';
}

function connect(mapFluxToProps, mapStateToProps) {
  mapStateToProps = mapStateToProps ? mapStateToProps : function () {
    return {};
  };
  return function wrapWithConnect(WrappedComponent) {
    var Connect = function (_Component) {
      connect_inherits(Connect, _Component);

      function Connect(props, context) {
        connect_classCallCheck(this, Connect);

        var _this = connect_possibleConstructorReturn(this, (Connect.__proto__ || Object.getPrototypeOf(Connect)).call(this, props, context));

        _this.reactor = props.reactor || context.reactor;
        _this.unsubscribeFns = [];
        _this.updatePropMap(props);
        return _this;
      }

      connect_createClass(Connect, [{
        key: 'resubscribe',
        value: function resubscribe(props) {
          this.unsubscribe();
          this.updatePropMap(props);
          this.updateState();
          this.subscribe();
        }
      }, {
        key: 'componentWillMount',
        value: function componentWillMount() {
          this.updateState();
          this.subscribe(this.props);
        }
      }, {
        key: 'componentWillUnmount',
        value: function componentWillUnmount() {
          this.unsubscribe();
        }
      }, {
        key: 'updatePropMap',
        value: function updatePropMap(props) {
          this.propMap = mapFluxToProps ? mapFluxToProps(props) : {};
        }
      }, {
        key: 'updateState',
        value: function updateState() {
          var propMap = this.propMap;
          var stateToSet = {};

          for (var key in propMap) {
            var getter = propMap[key];
            stateToSet[key] = this.reactor.evaluate(getter);
          }

          this.setState(stateToSet);
        }
      }, {
        key: 'subscribe',
        value: function subscribe() {
          var _this2 = this;

          var propMap = this.propMap;

          var _loop = function _loop(key) {
            var getter = propMap[key];
            var unsubscribeFn = _this2.reactor.observe(getter, function (val) {
              _this2.setState(connect_defineProperty({}, key, val));
            });

            _this2.unsubscribeFns.push(unsubscribeFn);
          };

          for (var key in propMap) {
            _loop(key);
          }
        }
      }, {
        key: 'unsubscribe',
        value: function unsubscribe() {
          if (this.unsubscribeFns.length === 0) {
            return;
          }

          while (this.unsubscribeFns.length > 0) {
            this.unsubscribeFns.shift()();
          }
        }
      }, {
        key: 'render',
        value: function render() {
          var stateProps = mapStateToProps(this.props);
          return Object(react["createElement"])(WrappedComponent, connect_extends({
            reactor: this.reactor
          }, stateProps, this.props, this.state));
        }
      }]);

      return Connect;
    }(react["Component"]);

    Connect.displayName = 'Connect(' + getDisplayName(WrappedComponent) + ')';
    Connect.WrappedComponent = WrappedComponent;
    Connect.contextTypes = {
      reactor: reactorShape
    };
    Connect.propTypes = {
      reactor: reactorShape
    };

    return hoist_non_react_statics_default()(Connect, WrappedComponent);
  };
}
// CONCATENATED MODULE: ./src/app/components/terminal/terminalActionBar.jsx
var terminalActionBar_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function terminalActionBar_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function terminalActionBar_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function terminalActionBar_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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










var closeTextStyle = {
  width: "30px",
  height: "30px",
  display: "block",
  margin: "0 auto"
};

var terminalActionBar_ActionBar = function (_React$Component) {
  terminalActionBar_inherits(ActionBar, _React$Component);

  function ActionBar() {
    var _ref;

    var _temp, _this, _ret;

    terminalActionBar_classCallCheck(this, ActionBar);

    for (var _len = arguments.length, args = Array(_len), _key = 0; _key < _len; _key++) {
      args[_key] = arguments[_key];
    }

    return _ret = (_temp = (_this = terminalActionBar_possibleConstructorReturn(this, (_ref = ActionBar.__proto__ || Object.getPrototypeOf(ActionBar)).call.apply(_ref, [this].concat(args))), _this), _this.openFileTransferDialog = function (isUpload) {
      var _this$props = _this.props,
          store = _this$props.store,
          params = _this$props.params;
      // disable actions if file transfer dialog is open

      if (store.isOpen) {
        return;
      }

      if (isUpload) {
        openUploadDialog(params);
      } else {
        openDownloadDialog(params);
      }
    }, _this.close = function () {
      actions_close();
    }, _this.openUploadDialog = function () {
      _this.openFileTransferDialog(true);
    }, _this.openDownloadDialog = function () {
      _this.openFileTransferDialog(false);
    }, _temp), terminalActionBar_possibleConstructorReturn(_this, _ret);
  }

  terminalActionBar_createClass(ActionBar, [{
    key: 'componentWillUnmount',
    value: function componentWillUnmount() {
      closeDialog();
    }
  }, {
    key: 'render',
    value: function render() {
      var _props = this.props,
          children = _props.children,
          store = _props.store;
      var isOpen = store.isOpen;


      var fileTransferClass = classnames_default()('grv-terminal-actions-files', isOpen && '--isOpen');

      return react_default.a.createElement(
        'div',
        { className: 'grv-terminal-actions' },
        react_default.a.createElement(
          'div',
          { title: 'Close', style: closeTextStyle, onClick: this.close },
          react_default.a.createElement(icons_CloseIcon, null)
        ),
        react_default.a.createElement(
          'div',
          { className: 'grv-terminal-actions-participans' },
          children
        ),
        react_default.a.createElement(
          'div',
          { className: fileTransferClass },
          react_default.a.createElement(
            'a',
            { title: 'Download files',
              className: 'grv-terminal-actions-files-btn m-b-sm',
              onClick: this.openDownloadDialog },
            react_default.a.createElement('i', { className: 'fa fa-download' })
          ),
          react_default.a.createElement(
            'a',
            { title: 'Upload files',
              className: 'grv-terminal-actions-files-btn',
              onClick: this.openUploadDialog },
            react_default.a.createElement('i', { className: 'fa fa-upload' })
          )
        )
      );
    }
  }]);

  return ActionBar;
}(react_default.a.Component);

function terminalActionBar_mapStateToProps() {
  return {
    store: fileTransfer_getters.store
  };
}

/* harmony default export */ var terminalActionBar = (connect(terminalActionBar_mapStateToProps)(Object(lib["withRouter"])(terminalActionBar_ActionBar)));
// EXTERNAL MODULE: ./node_modules/react-addons-css-transition-group/index.js
var react_addons_css_transition_group = __webpack_require__("J21I");
var react_addons_css_transition_group_default = /*#__PURE__*/__webpack_require__.n(react_addons_css_transition_group);

// CONCATENATED MODULE: ./src/app/components/terminal/terminalPartyList.jsx
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







var terminalPartyList_PartyList = function PartyList(props) {
  var parties = props.parties || [];
  var userIcons = parties.map(function (item, index) {
    return react_default.a.createElement(
      'div',
      { key: index, className: 'animated m-t' },
      react_default.a.createElement(icons_UserIcon, { colorIndex: index,
        isDark: true,
        name: item.user })
    );
  });

  return react_default.a.createElement(
    react_addons_css_transition_group_default.a,
    { className: 'm-t', component: 'div',
      transitionEnterTimeout: 500,
      transitionLeaveTimeout: 500,
      transitionName: {
        enter: "fadeIn",
        leave: "fadeOut"
      } },
    react_default.a.createElement('hr', { className: 'grv-divider m-t' }),
    userIcons
  );
};

function terminalPartyList_mapStateToProps(props) {
  return {
    parties: sessions_getters.activePartiesById(props.sid)
  };
}

/* harmony default export */ var terminalPartyList = (Object(build["connect"])(terminalPartyList_mapStateToProps)(terminalPartyList_PartyList));
// EXTERNAL MODULE: ./node_modules/xterm/dist/xterm.js
var xterm = __webpack_require__("Q92V");
var xterm_default = /*#__PURE__*/__webpack_require__.n(xterm);

// EXTERNAL MODULE: ./node_modules/events/events.js
var events_events = __webpack_require__("+qE3");

// EXTERNAL MODULE: ./node_modules/buffer/index.js
var buffer = __webpack_require__("tjlA");
var buffer_default = /*#__PURE__*/__webpack_require__.n(buffer);

// CONCATENATED MODULE: ./src/app/lib/term/protobuf.js
var protobuf_slicedToArray = function () { function sliceIterator(arr, i) { var _arr = []; var _n = true; var _d = false; var _e = undefined; try { for (var _i = arr[Symbol.iterator](), _s; !(_n = (_s = _i.next()).done); _n = true) { _arr.push(_s.value); if (i && _arr.length === i) break; } } catch (err) { _d = true; _e = err; } finally { try { if (!_n && _i["return"]) _i["return"](); } finally { if (_d) throw _e; } } return _arr; } return function (arr, i) { if (Array.isArray(arr)) { return arr; } else if (Symbol.iterator in Object(arr)) { return sliceIterator(arr, i); } else { throw new TypeError("Invalid attempt to destructure non-iterable instance"); } }; }();

var protobuf_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function protobuf_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

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



/**
 * convenience constant equal to 2^32.
 */
var TWO_TO_32 = 4294967296;

var MessageTypeEnum = {
  RAW: 'r',
  AUDIT: 'a',
  SESSION_END: 'c',
  RESIZE: 'w'
};

var messageFields = {
  payload: {
    code: 0x1a
  },

  version: {
    code: 10,
    length: 1,
    values: {
      v1: 49
    }
  },

  type: {
    length: 1,
    code: 0x12,
    values: {
      resize: MessageTypeEnum.RESIZE.charCodeAt(0),
      data: MessageTypeEnum.RAW.charCodeAt(0),
      event: MessageTypeEnum.AUDIT.charCodeAt(0),
      close: MessageTypeEnum.SESSION_END.charCodeAt(0)
    }
  }
};

var protobuf_Protobuf = function () {
  function Protobuf() {
    protobuf_classCallCheck(this, Protobuf);
  }

  protobuf_createClass(Protobuf, [{
    key: 'encode',
    value: function encode(messageType, message) {
      var buffer = [];
      this.encodeVersion(buffer);
      this.encodeType(buffer, messageType);
      this.encodePayload(buffer, message);
      return buffer;
    }
  }, {
    key: 'encodeResizeMessage',
    value: function encodeResizeMessage(message) {
      return this.encode(messageFields.type.values.resize, message);
    }
  }, {
    key: 'encodeRawMessage',
    value: function encodeRawMessage(message) {
      return this.encode(messageFields.type.values.data, message);
    }
  }, {
    key: 'encodePayload',
    value: function encodePayload(buffer, text) {
      // set type
      buffer.push(messageFields.payload.code);

      // encode payload
      var uintArray = this._textToUintArray(text);
      this.encodeVarint(buffer, uintArray.length);
      for (var i = 0; i < uintArray.length; i++) {
        buffer.push(uintArray[i]);
      }
    }
  }, {
    key: 'encodeVersion',
    value: function encodeVersion(buffer) {
      buffer[0] = messageFields.version.code;
      buffer[1] = messageFields.version.length;
      buffer[2] = messageFields.version.values.v1;
    }
  }, {
    key: 'encodeType',
    value: function encodeType(buffer, typeValue) {
      buffer[3] = messageFields.type.code;
      buffer[4] = messageFields.type.length;
      buffer[5] = typeValue;
    }
  }, {
    key: 'encodeVarint',
    value: function encodeVarint(buffer, value) {
      var lowBits = value >>> 0;
      var highBits = Math.floor((value - lowBits) / TWO_TO_32) >>> 0;
      while (highBits > 0 || lowBits > 127) {
        buffer.push(lowBits & 0x7f | 0x80);
        lowBits = (lowBits >>> 7 | highBits << 25) >>> 0;
        highBits = highBits >>> 7;
      }

      buffer.push(lowBits);
    }
  }, {
    key: 'decode',
    value: function decode(uintArray) {
      var version = this.decodeVersion(uintArray);
      var type = this.decodeType(uintArray);
      var payload = this.decodePayload(uintArray);
      return {
        version: version,
        type: type,
        payload: payload
      };
    }
  }, {
    key: 'decodeVersion',
    value: function decodeVersion(uintArray) {
      if (uintArray[0] === messageFields.version.code && uintArray[1] === messageFields.version.length) {
        return String.fromCharCode(uintArray[2]);
      }

      throw new Error("invalid version field");
    }
  }, {
    key: 'decodeType',
    value: function decodeType(uintArray) {
      if (uintArray[3] === messageFields.type.code && uintArray[4] === messageFields.type.length) {
        return String.fromCharCode(uintArray[5]);
      }
      throw new Error("invalid type field");
    }
  }, {
    key: 'decodePayload',
    value: function decodePayload(uintArray) {
      if (!uintArray[6]) {
        return "";
      }

      if (uintArray[6] !== messageFields.payload.code) {
        throw new Error("invalid payload field");
      }

      var rawPayloadField = uintArray.slice(7);

      var _decodeVarint = this.decodeVarint(rawPayloadField),
          _decodeVarint2 = protobuf_slicedToArray(_decodeVarint, 2),
          startsAt = _decodeVarint2[0],
          payloadLength = _decodeVarint2[1];

      var payloadBytes = rawPayloadField.slice(startsAt, startsAt + payloadLength);
      return this._uintArrayToText(payloadBytes);
    }
  }, {
    key: 'decodeVarint',
    value: function decodeVarint(uintArray) {
      var x = 0;
      var s = 0;
      for (var i = 0; i < uintArray.length; i++) {
        var b = uintArray[i];
        if (b < 0x80) {
          if (i > 9 || i == 9 && b > 1) {
            throw new Error("unable to decode varint: overflow");
          }
          return [i + 1, x | b << s];
        }
        x = x | b & 0x7f << s;
        s = s + 7;
      }

      throw new Error("unable to decode varint: empty array");
    }
  }, {
    key: '_textToUintArray',
    value: function _textToUintArray(text) {
      return buffer_default.a.Buffer(text);
    }
  }, {
    key: '_uintArrayToText',
    value: function _uintArrayToText(uintArray) {
      // use native TextDecoder if supported
      if (window.TextDecoder) {
        return new TextDecoder("utf-8").decode(uintArray);
      } else {
        return buffer_default.a.Buffer(uintArray).toString();
      }
    }
  }]);

  return Protobuf;
}();

// Polyfill for Uint8Array.slice for IE and Safari
if (!Uint8Array.prototype.slice) {
  Object.defineProperty(Uint8Array.prototype, 'slice', {
    value: Array.prototype.slice
  });
}
// CONCATENATED MODULE: ./src/app/lib/term/tty.js
var tty_slicedToArray = function () { function sliceIterator(arr, i) { var _arr = []; var _n = true; var _d = false; var _e = undefined; try { for (var _i = arr[Symbol.iterator](), _s; !(_n = (_s = _i.next()).done); _n = true) { _arr.push(_s.value); if (i && _arr.length === i) break; } } catch (err) { _d = true; _e = err; } finally { try { if (!_n && _i["return"]) _i["return"](); } finally { if (_d) throw _e; } } return _arr; } return function (arr, i) { if (Array.isArray(arr)) { return arr; } else if (Symbol.iterator in Object(arr)) { return sliceIterator(arr, i); } else { throw new TypeError("Invalid attempt to destructure non-iterable instance"); } }; }();

var tty_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

var tty_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function tty_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function tty_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function tty_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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






var tty_logger = logger["a" /* default */].create('Tty');

var defaultOptions = {
  buffered: true
};

var tty_Tty = function (_EventEmitter) {
  tty_inherits(Tty, _EventEmitter);

  function Tty(addressResolver) {
    var props = arguments.length > 1 && arguments[1] !== undefined ? arguments[1] : {};

    tty_classCallCheck(this, Tty);

    var _this = tty_possibleConstructorReturn(this, (Tty.__proto__ || Object.getPrototypeOf(Tty)).call(this));

    _this.socket = null;
    _this._buffered = true;
    _this._addressResolver = null;

    var options = tty_extends({}, defaultOptions, props);

    _this._addressResolver = addressResolver;
    _this._buffered = options.buffered;
    _this._proto = new protobuf_Protobuf();
    _this._onOpenConnection = _this._onOpenConnection.bind(_this);
    _this._onCloseConnection = _this._onCloseConnection.bind(_this);
    _this._onMessage = _this._onMessage.bind(_this);
    return _this;
  }

  tty_createClass(Tty, [{
    key: 'disconnect',
    value: function disconnect() {
      var reasonCode = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : StatusCodeEnum.NORMAL;

      if (this.socket !== null) {
        this.socket.close(reasonCode);
      }
    }
  }, {
    key: 'connect',
    value: function connect(w, h) {
      var connStr = this._addressResolver.getConnStr(w, h);
      this.socket = new WebSocket(connStr);
      this.socket.binaryType = 'arraybuffer';
      this.socket.onopen = this._onOpenConnection;
      this.socket.onmessage = this._onMessage;
      this.socket.onclose = this._onCloseConnection;
    }
  }, {
    key: 'send',
    value: function send(data) {
      var msg = this._proto.encodeRawMessage(data);
      var bytearray = new Uint8Array(msg);
      this.socket.send(bytearray.buffer);
    }
  }, {
    key: 'requestResize',
    value: function requestResize(w, h) {
      tty_logger.info('requesting new screen size', 'w:' + w + ' and h:' + h);
      var data = JSON.stringify({
        event: EventTypeEnum.RESIZE,
        width: w,
        height: h,
        size: w + ':' + h
      });

      var encoded = this._proto.encodeResizeMessage(data);
      var bytearray = new Uint8Array(encoded);
      this.socket.send(bytearray.buffer);
    }
  }, {
    key: '_flushBuffer',
    value: function _flushBuffer() {
      this.emit(TermEventEnum.DATA, this._attachSocketBuffer);
      this._attachSocketBuffer = null;
      clearTimeout(this._attachSocketBufferTimer);
      this._attachSocketBufferTimer = null;
    }
  }, {
    key: '_pushToBuffer',
    value: function _pushToBuffer(data) {
      if (this._attachSocketBuffer) {
        this._attachSocketBuffer += data;
      } else {
        this._attachSocketBuffer = data;
        setTimeout(this._flushBuffer.bind(this), 10);
      }
    }
  }, {
    key: '_onOpenConnection',
    value: function _onOpenConnection() {
      this.emit('open');
      tty_logger.info('websocket is open');
    }
  }, {
    key: '_onCloseConnection',
    value: function _onCloseConnection(e) {
      this.socket.onopen = null;
      this.socket.onmessage = null;
      this.socket.onclose = null;
      this.socket = null;
      this.emit(TermEventEnum.CONN_CLOSE, e);
      tty_logger.info('websocket is closed');
    }
  }, {
    key: '_onMessage',
    value: function _onMessage(ev) {
      try {
        var uintArray = new Uint8Array(ev.data);
        var msg = this._proto.decode(uintArray);
        switch (msg.type) {
          case MessageTypeEnum.AUDIT:
            this._processAuditPayload(msg.payload);
            break;
          case MessageTypeEnum.SESSION_END:
            this.emit(TermEventEnum.CLOSE, msg.payload);
            break;
          case MessageTypeEnum.RAW:
            if (this._buffered) {
              this._pushToBuffer(msg.payload);
            } else {
              this.emit(TermEventEnum.DATA, msg.payload);
            }
            break;
          default:
            throw Error('unknown message type', msg.type);
        }
      } catch (err) {
        tty_logger.error('failed to parse incoming message.', err);
      }
    }
  }, {
    key: '_processAuditPayload',
    value: function _processAuditPayload(payload) {
      var event = JSON.parse(payload);
      if (event.event === EventTypeEnum.RESIZE) {
        var _event$size$split = event.size.split(':'),
            _event$size$split2 = tty_slicedToArray(_event$size$split, 2),
            w = _event$size$split2[0],
            h = _event$size$split2[1];

        w = Number(w);
        h = Number(h);
        this.emit(TermEventEnum.RESIZE, { w: w, h: h });
      }
    }
  }]);

  return Tty;
}(events_events["EventEmitter"]);

/* harmony default export */ var tty = (tty_Tty);
// CONCATENATED MODULE: ./src/app/lib/term/terminal.js
var terminal_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function terminal_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

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






var terminal_logger = logger["a" /* default */].create('lib/term/terminal');
var DISCONNECT_TXT = 'disconnected';
var GRV_CLASS = 'grv-terminal';
var WINDOW_RESIZE_DEBOUNCE_DELAY = 200;

/**
 * TtyTerminal is a wrapper on top of xtermjs that handles connections
 * and resize events
 */

var terminal_TtyTerminal = function () {
  function TtyTerminal(options) {
    terminal_classCallCheck(this, TtyTerminal);

    var addressResolver = options.addressResolver,
        el = options.el,
        _options$scrollBack = options.scrollBack,
        scrollBack = _options$scrollBack === undefined ? 1000 : _options$scrollBack;

    this._el = el;
    this.tty = new tty(addressResolver);
    this.scrollBack = scrollBack;
    this.rows = undefined;
    this.cols = undefined;
    this.term = null;
    this.debouncedResize = Object(lodash["debounce"])(this._requestResize.bind(this), WINDOW_RESIZE_DEBOUNCE_DELAY);
  }

  terminal_createClass(TtyTerminal, [{
    key: 'open',
    value: function open() {
      var _this = this;

      this._el.classList.add(GRV_CLASS);

      // render xtermjs with default values
      this.term = new xterm_default.a({
        cols: 15,
        rows: 5,
        scrollback: this.scrollBack,
        cursorBlink: false
      });

      this.term.open(this._el);

      // fit xterm to available space
      this.resize(this.cols, this.rows);

      // subscribe to xtermjs output
      this.term.on('data', function (data) {
        _this.tty.send(data);
      });

      // subscribe to window resize events
      window.addEventListener('resize', this.debouncedResize);

      // subscribe to tty
      this.tty.on(TermEventEnum.RESET, this.reset.bind(this));
      this.tty.on(TermEventEnum.CONN_CLOSE, this._processClose.bind(this));
      this.tty.on(TermEventEnum.DATA, this._processData.bind(this));

      // subscribe tty resize event (used by session player)
      this.tty.on(TermEventEnum.RESIZE, function (_ref) {
        var h = _ref.h,
            w = _ref.w;
        return _this.resize(w, h);
      });

      this.connect();
    }
  }, {
    key: 'connect',
    value: function connect() {
      this.tty.connect(this.cols, this.rows);
    }
  }, {
    key: 'destroy',
    value: function destroy() {
      window.removeEventListener('resize', this.debouncedResize);
      this._disconnect();
      if (this.term !== null) {
        this.term.destroy();
        this.term.removeAllListeners();
      }

      this._el.innerHTML = null;
      this._el.classList.remove(GRV_CLASS);
    }
  }, {
    key: 'reset',
    value: function reset() {
      this.term.reset();
    }
  }, {
    key: 'resize',
    value: function resize(cols, rows) {
      try {
        // if not defined, use the size of the container
        if (!Object(lodash["isInteger"])(cols) || !Object(lodash["isInteger"])(rows)) {
          var dim = this._getDimensions();
          cols = dim.cols;
          rows = dim.rows;
        }

        if (cols === this.cols && rows === this.rows) {
          return;
        }

        this.cols = cols;
        this.rows = rows;
        this.term.resize(cols, rows);
      } catch (err) {
        terminal_logger.error('xterm.resize', { w: cols, h: rows }, err);
        this.term.reset();
      }
    }
  }, {
    key: '_processData',
    value: function _processData(data) {
      try {
        this.term.write(data);
      } catch (err) {
        terminal_logger.error('xterm.write', data, err);
        // recover xtermjs by resetting it
        this.term.reset();
      }
    }
  }, {
    key: '_processClose',
    value: function _processClose(e) {
      var reason = e.reason;

      var displayText = DISCONNECT_TXT;
      if (reason) {
        displayText = displayText + ': ' + reason;
      }

      displayText = '\x1B[31m' + displayText + '\x1B[m\r\n';
      this.term.write(displayText);
    }
  }, {
    key: '_disconnect',
    value: function _disconnect() {
      this.tty.disconnect();
      this.tty.removeAllListeners();
    }
  }, {
    key: '_requestResize',
    value: function _requestResize() {
      var _getDimensions2 = this._getDimensions(),
          cols = _getDimensions2.cols,
          rows = _getDimensions2.rows;

      if (!Object(lodash["isInteger"])(cols) || !Object(lodash["isInteger"])(rows)) {
        terminal_logger.info('unable to calculate terminal dimensions (container might be hidden) ' + cols + ':' + rows);
        return;
      }

      // ensure min size
      var w = cols < 5 ? 5 : cols;
      var h = rows < 5 ? 5 : rows;

      this.resize(w, h);
      this.tty.requestResize(w, h);
    }
  }, {
    key: '_getDimensions',
    value: function _getDimensions() {
      var $terminal = this._el.querySelector('.terminal');
      var $fakeRow = document.createElement('div');
      $fakeRow.innerHTML = '<span>&nbsp;</span>';

      $terminal.appendChild($fakeRow);

      var fakeColHeight = $fakeRow.getBoundingClientRect().height;
      var fakeColWidth = $fakeRow.firstElementChild.getBoundingClientRect().width;
      var width = this._el.clientWidth;
      var height = this._el.clientHeight;
      var cols = Math.floor(width / fakeColWidth);
      var rows = Math.floor(height / fakeColHeight);

      $terminal.removeChild($fakeRow);
      return { cols: cols, rows: rows };
    }
  }]);

  return TtyTerminal;
}();

/* harmony default export */ var terminal = (terminal_TtyTerminal);
// CONCATENATED MODULE: ./src/app/lib/term/ttyAddressResolver.js
var ttyAddressResolver_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

var ttyAddressResolver_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function ttyAddressResolver_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

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



var ttyAddressResolver_AddressResolver = function () {
  function AddressResolver(params) {
    ttyAddressResolver_classCallCheck(this, AddressResolver);

    this._params = {
      login: null,
      target: function target() {
        throw Error('target method is not provided');
      },
      sid: null,
      clusterName: null,
      ttyUrl: null
    };

    this._params = ttyAddressResolver_extends({}, params);
  }

  ttyAddressResolver_createClass(AddressResolver, [{
    key: 'getConnStr',
    value: function getConnStr(w, h) {
      var _params = this._params,
          getTarget = _params.getTarget,
          ttyUrl = _params.ttyUrl,
          login = _params.login,
          sid = _params.sid;

      var params = JSON.stringify(ttyAddressResolver_extends({}, getTarget(), {
        login: login,
        sid: sid,
        term: { h: h, w: w }
      }));

      var encoded = window.encodeURI(params);
      return this.format(ttyUrl).replace(':params', encoded);
    }
  }, {
    key: 'format',
    value: function format(url) {
      return url.replace(':fqdm', config["a" /* default */].getWsHostName()).replace(':token', this._params.token).replace(':cluster', this._params.cluster).replace(':sid', this._params.sid);
    }
  }]);

  return AddressResolver;
}();

/* harmony default export */ var ttyAddressResolver = (ttyAddressResolver_AddressResolver);
// CONCATENATED MODULE: ./src/app/components/terminal/terminal.jsx
var terminal_terminal_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function terminal_terminal_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function terminal_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function terminal_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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






var terminal_Terminal = function (_React$Component) {
  terminal_inherits(Terminal, _React$Component);

  function Terminal() {
    terminal_terminal_classCallCheck(this, Terminal);

    return terminal_possibleConstructorReturn(this, (Terminal.__proto__ || Object.getPrototypeOf(Terminal)).apply(this, arguments));
  }

  terminal_terminal_createClass(Terminal, [{
    key: 'componentDidMount',
    value: function componentDidMount() {
      var _props = this.props,
          onSessionEnd = _props.onSessionEnd,
          ttyParams = _props.ttyParams,
          title = _props.title;

      var addressResolver = new ttyAddressResolver(ttyParams);
      this.terminal = new terminal({
        el: this.refs.container,
        addressResolver: addressResolver
      });

      this.terminal.open();
      this.terminal.tty.on(TermEventEnum.CLOSE, onSessionEnd);

      document.title = title;
    }
  }, {
    key: 'componentWillUnmount',
    value: function componentWillUnmount() {
      this.terminal.destroy();
    }
  }, {
    key: 'shouldComponentUpdate',
    value: function shouldComponentUpdate() {
      return false;
    }
  }, {
    key: 'focus',
    value: function focus() {
      this.terminal.term.focus();
    }
  }, {
    key: 'render',
    value: function render() {
      return react_default.a.createElement('div', { ref: 'container' });
    }
  }]);

  return Terminal;
}(react_default.a.Component);
// CONCATENATED MODULE: ./src/app/components/files/items.jsx
/*
Copyright 2018 Gravitational, Inc.

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




var items_Text = function Text(props) {
  return react_default.a.createElement(
    'div',
    { className: classnames_default()('grv-file-transfer-text', props.className) },
    props.children
  );
};
// CONCATENATED MODULE: ./src/app/components/files/download.jsx
var download_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function download_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function download_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function download_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

/*
Copyright 2018 Gravitational, Inc.

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




var download_FileDownloadSelector = function (_React$Component) {
  download_inherits(FileDownloadSelector, _React$Component);

  function FileDownloadSelector() {
    var _ref;

    var _temp, _this, _ret;

    download_classCallCheck(this, FileDownloadSelector);

    for (var _len = arguments.length, args = Array(_len), _key = 0; _key < _len; _key++) {
      args[_key] = arguments[_key];
    }

    return _ret = (_temp = (_this = download_possibleConstructorReturn(this, (_ref = FileDownloadSelector.__proto__ || Object.getPrototypeOf(FileDownloadSelector)).call.apply(_ref, [this].concat(args))), _this), _this.state = {
      path: '~/'
    }, _this.onChangePath = function (e) {
      _this.setState({
        path: e.target.value
      });
    }, _this.onDownload = function () {
      if (_this.isValidPath(_this.state.path)) {
        _this.props.onDownload(_this.state.path);
      }
    }, _this.onKeyDown = function (event) {
      if (event.key === 'Enter') {
        event.preventDefault();
        event.stopPropagation();
        _this.onDownload();
      }
    }, _temp), download_possibleConstructorReturn(_this, _ret);
  }

  download_createClass(FileDownloadSelector, [{
    key: 'isValidPath',
    value: function isValidPath(path) {
      return path && path[path.length - 1] !== '/';
    }
  }, {
    key: 'moveCaretAtEnd',
    value: function moveCaretAtEnd(e) {
      var tmp = e.target.value;
      e.target.value = '';
      e.target.value = tmp;
    }
  }, {
    key: 'render',
    value: function render() {
      var _this2 = this;

      var path = this.state.path;

      var isBtnDisabled = !this.isValidPath(path);
      return react_default.a.createElement(
        'div',
        { className: 'grv-file-transfer-header m-b' },
        react_default.a.createElement(
          items_Text,
          { className: 'm-b' },
          react_default.a.createElement(
            'h4',
            null,
            'SCP DOWNLOAD'
          )
        ),
        react_default.a.createElement(
          items_Text,
          { className: 'm-b-xs' },
          'File path'
        ),
        react_default.a.createElement(
          'div',
          { className: 'grv-file-transfer-download' },
          react_default.a.createElement('input', { onChange: this.onChangePath,
            ref: function ref(e) {
              return _this2.inputRef = e;
            },
            value: path,
            className: 'grv-file-transfer-input m-r-sm',
            autoFocus: true,
            onFocus: this.moveCaretAtEnd,
            onKeyDown: this.onKeyDown
          }),
          react_default.a.createElement(
            'button',
            { className: 'btn btn-sm grv-file-transfer-btn',
              style: { width: "105px" },
              disabled: isBtnDisabled,
              onClick: this.onDownload },
            'Download'
          )
        )
      );
    }
  }]);

  return FileDownloadSelector;
}(react_default.a.Component);
download_FileDownloadSelector.propTypes = {
  onDownload: react["PropTypes"].func.isRequired
};
// CONCATENATED MODULE: ./src/app/components/files/upload.jsx
var upload_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function upload_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function upload_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function upload_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

/*
Copyright 2018 Gravitational, Inc.

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




var upload_FileUploadSelector = function (_React$Component) {
  upload_inherits(FileUploadSelector, _React$Component);

  function FileUploadSelector() {
    var _ref;

    var _temp, _this, _ret;

    upload_classCallCheck(this, FileUploadSelector);

    for (var _len = arguments.length, args = Array(_len), _key = 0; _key < _len; _key++) {
      args[_key] = arguments[_key];
    }

    return _ret = (_temp = (_this = upload_possibleConstructorReturn(this, (_ref = FileUploadSelector.__proto__ || Object.getPrototypeOf(FileUploadSelector)).call.apply(_ref, [this].concat(args))), _this), _this.state = {
      files: [],
      remoteLocation: "~/"
    }, _this.onFileSelected = function (e) {
      _this.addFiles([], e.target.files);
      _this.inputRef.focus();
    }, _this.onFilePathChanged = function (e) {
      _this.setState({
        remoteLocation: e.target.value
      });
    }, _this.onUpload = function () {
      var _this$state = _this.state,
          files = _this$state.files,
          remoteLocation = _this$state.remoteLocation;

      for (var i = 0; i < files.length; i++) {
        _this.props.onUpload(remoteLocation, files[i].name, files[i]);
      }

      _this.setState({ files: [] });
      _this.setFocus();
    }, _this.onOpenFilePicker = function () {
      // reset all selected files
      _this.fileSelectorRef.value = "";
      _this.fileSelectorRef.click();
    }, _this.onDrop = function (e) {
      e.preventDefault();
      e.stopPropagation();
      _this.addFiles(_this.state.files, e.dataTransfer.files);
      _this.setFocus();
    }, _this.onKeyDown = function (event) {
      if (event.key === 'Enter') {
        event.preventDefault();
        event.stopPropagation();
        _this.onOpenFilePicker();
      }
    }, _temp), upload_possibleConstructorReturn(_this, _ret);
  }

  upload_createClass(FileUploadSelector, [{
    key: 'componentWillUnmount',
    value: function componentWillUnmount() {
      document.removeEventListener('drop', this.onDocumentDrop);
      document.removeEventListener('dragover', this.preventDefault);
    }
  }, {
    key: 'componentDidMount',
    value: function componentDidMount() {
      document.addEventListener('dragover', this.preventDefault, false);
      document.addEventListener('drop', this.onDocumentDrop, false);
    }
  }, {
    key: 'preventDefault',
    value: function preventDefault(e) {
      e.preventDefault();
    }
  }, {
    key: 'onDocumentDrop',
    value: function onDocumentDrop(e) {
      if (this.refDropzone && this.refDropzone.contains(e.target)) {
        return;
      }

      e.preventDefault();
      e.dataTransfer.effectAllowed = 'none';
      e.dataTransfer.dropEffect = 'none';
    }
  }, {
    key: 'setFocus',
    value: function setFocus() {
      this.inputRef.focus();
    }
  }, {
    key: 'moveCaretAtEnd',
    value: function moveCaretAtEnd(e) {
      var tmp = e.target.value;
      e.target.value = '';
      e.target.value = tmp;
    }
  }, {
    key: 'addFiles',
    value: function addFiles(files) {
      var blobs = arguments.length > 1 && arguments[1] !== undefined ? arguments[1] : [];

      for (var i = 0; i < blobs.length; i++) {
        files.push(blobs[i]);
      }

      this.setState({
        files: files
      });
    }
  }, {
    key: 'render',
    value: function render() {
      var _this2 = this;

      var _state = this.state,
          remoteLocation = _state.remoteLocation,
          files = _state.files;

      var isDldBtnDisabled = !remoteLocation || files.length === 0;
      var hasFiles = files.length > 0;

      return react_default.a.createElement(
        'div',
        { className: 'grv-file-transfer-header m-b' },
        react_default.a.createElement(
          items_Text,
          { className: 'm-b' },
          react_default.a.createElement(
            'h4',
            null,
            'SCP UPLOAD'
          )
        ),
        react_default.a.createElement(
          'div',
          { className: 'grv-file-transfer-upload' },
          react_default.a.createElement(
            'div',
            { className: 'grv-file-transfer-upload-selected-files',
              ref: function ref(e) {
                return _this2.refDropzone = e;
              },
              onDragOver: function onDragOver(e) {
                return e.preventDefault();
              },
              onDrop: this.onDrop
            },
            !hasFiles && react_default.a.createElement(
              'div',
              null,
              react_default.a.createElement(
                'a',
                { onClick: this.onOpenFilePicker },
                'Select files'
              ),
              ' to upload or drag & drop them here'
            ),
            hasFiles && react_default.a.createElement(
              'div',
              null,
              react_default.a.createElement(
                'a',
                { onClick: this.onOpenFilePicker },
                ' ',
                files.length,
                ' files selected '
              )
            )
          ),
          react_default.a.createElement(
            items_Text,
            { className: 'm-b-xs m-t' },
            'Upload destination'
          ),
          react_default.a.createElement(
            'div',
            { style: { display: "flex" } },
            react_default.a.createElement('input', { className: 'grv-file-transfer-input m-r-sm',
              ref: function ref(e) {
                return _this2.inputRef = e;
              },
              value: remoteLocation,
              autoFocus: true,
              onFocus: this.moveCaretAtEnd,
              onChange: this.onFilePathChanged,
              onKeyDown: this.onKeyDown
            }),
            react_default.a.createElement(
              'button',
              { className: 'btn btn-sm grv-file-transfer-btn',
                style: { width: "105px" },
                disabled: isDldBtnDisabled,
                onClick: this.onUpload },
              'Upload'
            )
          ),
          react_default.a.createElement('input', { ref: function ref(e) {
              return _this2.fileSelectorRef = e;
            }, type: 'file',
            multiple: true,
            style: { display: "none" },
            accept: '*.*',
            name: 'file',
            onChange: this.onFileSelected
          })
        )
      );
    }
  }]);

  return FileUploadSelector;
}(react_default.a.Component);
upload_FileUploadSelector.propTypes = {
  onUpload: react["PropTypes"].func.isRequired
};
// CONCATENATED MODULE: ./src/app/services/fileTransfer.js
var fileTransfer_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function fileTransfer_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function fileTransfer_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function fileTransfer_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }





var fileTransfer_logger = logger["a" /* default */].create('api/fileTransfer');

var REQ_FAILED_TXT = 'Network request failed';

var Transfer = function (_EventEmitter) {
  fileTransfer_inherits(Transfer, _EventEmitter);

  function Transfer() {
    fileTransfer_classCallCheck(this, Transfer);

    var _this = fileTransfer_possibleConstructorReturn(this, (Transfer.__proto__ || Object.getPrototypeOf(Transfer)).call(this));

    _this._xhr = new XMLHttpRequest();
    var xhr = _this._xhr;

    xhr.onload = function () {
      var status = xhr.status;

      if (status === 200) {
        _this.handleSuccess(xhr);
        return;
      }

      _this.handleError(xhr);
    };

    xhr.onerror = function () {
      _this.emit('error', new Error(REQ_FAILED_TXT));
    };

    xhr.ontimeout = function () {
      _this.emit('error', new Error(REQ_FAILED_TXT));
    };

    xhr.onabort = function () {
      _this.emit('error', new DOMException('Aborted', 'AbortError'));
    };
    return _this;
  }

  fileTransfer_createClass(Transfer, [{
    key: 'abort',
    value: function abort() {
      this._xhr.abort();
    }
  }, {
    key: 'onProgress',
    value: function onProgress(cb) {
      this.on('progress', cb);
    }
  }, {
    key: 'onCompleted',
    value: function onCompleted(cb) {
      this.on('completed', cb);
    }
  }, {
    key: 'onError',
    value: function onError(cb) {
      this.on('error', cb);
    }
  }, {
    key: 'handleSuccess',
    value: function handleSuccess() {
      throw Error('not implemented');
    }
  }, {
    key: 'handleError',
    value: function handleError(xhr) {
      var errText = fileTransfer_getErrorText(xhr.response);
      this.emit('error', new Error(errText));
    }
  }, {
    key: 'handleProgress',
    value: function handleProgress(e) {
      var progress = 0;
      // if Content-Length is present
      if (e.lengthComputable) {
        progress = Math.round(e.loaded / e.total * 100);
      } else {
        var done = e.position || e.loaded;
        var total = e.totalSize || e.total;
        progress = Math.floor(done / total * 1000) / 10;
      }

      this.emit('progress', progress);
    }
  }]);

  return Transfer;
}(events_events["EventEmitter"]);

var fileTransfer_Uploader = function (_Transfer) {
  fileTransfer_inherits(Uploader, _Transfer);

  function Uploader() {
    fileTransfer_classCallCheck(this, Uploader);

    return fileTransfer_possibleConstructorReturn(this, (Uploader.__proto__ || Object.getPrototypeOf(Uploader)).call(this));
  }

  fileTransfer_createClass(Uploader, [{
    key: 'handleSuccess',
    value: function handleSuccess() {
      this.emit('completed');
    }
  }, {
    key: 'do',
    value: function _do(url, blob) {
      var _this3 = this;

      this._xhr.upload.addEventListener('progress', function (e) {
        _this3.handleProgress(e);
      });

      this._xhr.open('post', url, true);
      api["a" /* default */].setAuthHeaders(this._xhr);
      api["a" /* default */].setNoCacheHeaders(this._xhr);
      this._xhr.send(blob);
    }
  }]);

  return Uploader;
}(Transfer);

var fileTransfer_Downloader = function (_Transfer2) {
  fileTransfer_inherits(Downloader, _Transfer2);

  function Downloader() {
    fileTransfer_classCallCheck(this, Downloader);

    return fileTransfer_possibleConstructorReturn(this, (Downloader.__proto__ || Object.getPrototypeOf(Downloader)).call(this));
  }

  fileTransfer_createClass(Downloader, [{
    key: 'do',
    value: function _do(url) {
      var _this5 = this;

      this._xhr.open('get', url, true);
      this._xhr.onprogress = function (e) {
        _this5.handleProgress(e);
      };

      api["a" /* default */].setAuthHeaders(this._xhr);
      this._xhr.responseType = 'blob';
      this._xhr.send();
    }
  }, {
    key: 'handleSuccess',
    value: function handleSuccess(xhr) {
      var fileName = getDispositionFileName(xhr);
      if (!fileName) {
        this.emit('error', new Error("Bad response"));
      } else {
        this.emit('completed', {
          fileName: fileName,
          blob: xhr.response
        });
      }
    }

    // parses blob response to get an error text

  }, {
    key: 'handleError',
    value: function handleError(xhr) {
      var _this6 = this;

      var reader = new FileReader();

      reader.onerror = function (err) {
        _this6.emit('error', err);
      };

      reader.onload = function () {
        var text = fileTransfer_getErrorText(reader.result);
        _this6.emit('error', new Error(text));
      };

      reader.readAsText(xhr.response);
    }
  }]);

  return Downloader;
}(Transfer);

function getDispositionFileName(xhr) {
  var fileName = "";
  var disposition = xhr.getResponseHeader("Content-Disposition");
  if (disposition) {
    var filenameRegex = /filename[^;=\n]*=((['"]).*?\2|[^;\n]*)/;
    var matches = filenameRegex.exec(disposition);
    if (matches != null && matches[1]) {
      fileName = matches[1].replace(/['"]/g, '');
    }
  }

  return decodeURIComponent(fileName);
}

// TODO: as backend may return errors in different
// formats, look at different JSON structures to retreive the error message
function fileTransfer_getErrorText(response, responseText) {
  var errText = 'Bad request';
  if (!response) {
    return responseText || errText;
  }

  try {
    var json = JSON.parse(response);
    if (json.message) {
      return json.message;
    }

    if (responseText) {
      return responseText;
    }
  } catch (err) {
    fileTransfer_logger.error('faild to parse error message', err);
  }

  return errText;
}
// CONCATENATED MODULE: ./src/app/components/files/withHttpRequest.jsx
var withHttpRequest_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

var withHttpRequest_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function withHttpRequest_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function withHttpRequest_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function withHttpRequest_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

/*
Copyright 2018 Gravitational, Inc.

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




var withHttpRequest_withHttpRequest = function withHttpRequest(httpCtor) {
  return function (component) {
    var _class, _temp;

    return _temp = _class = function (_React$Component) {
      withHttpRequest_inherits(WithHttpRequestWrapper, _React$Component);

      function WithHttpRequestWrapper(props, context) {
        withHttpRequest_classCallCheck(this, WithHttpRequestWrapper);

        var _this = withHttpRequest_possibleConstructorReturn(this, (WithHttpRequestWrapper.__proto__ || Object.getPrototypeOf(WithHttpRequestWrapper)).call(this, props, context));

        _this.state = {
          progress: "0",
          response: null
        };

        _this.onRemove = function () {
          removeFile(_this.fileId);
        };

        _this.http = new httpCtor();
        _this.fileId = props.file.id;
        _this.fileBlob = props.file.blob;
        _this.fileUrl = props.file.url;
        return _this;
      }

      withHttpRequest_createClass(WithHttpRequestWrapper, [{
        key: 'componentWillUnmount',
        value: function componentWillUnmount() {
          this.http.removeAllListeners();
          this.http.abort();
        }
      }, {
        key: 'componentDidMount',
        value: function componentDidMount() {
          var _this2 = this;

          var handleProgress = function handleProgress(completed) {
            _this2.setState({ progress: completed });
          };

          var handleCompleted = function handleCompleted(response) {
            _this2.state.response = response;
            updateStatus({
              id: _this2.fileId,
              isCompleted: true
            });
          };

          var handleFailed = function handleFailed(err) {
            updateStatus({
              id: _this2.fileId,
              isFailed: true,
              error: err.message
            });
          };

          updateStatus({
            id: this.fileId,
            isProcessing: true
          });

          this.http.onProgress(handleProgress);
          this.http.onCompleted(handleCompleted);
          this.http.onError(handleFailed);
          this.http.do(this.fileUrl, this.fileBlob);
        }
      }, {
        key: 'render',
        value: function render() {
          var _state = this.state,
              response = _state.response,
              progress = _state.progress;

          return react_default.a.createElement(component, withHttpRequest_extends({}, this.props, {
            onRemove: this.onRemove,
            httpResponse: response,
            httpProgress: progress
          }));
        }
      }]);

      return WithHttpRequestWrapper;
    }(react_default.a.Component), _class.displayName = 'WithHttpRequestWrapper', _temp;
  };
};

/* harmony default export */ var files_withHttpRequest = (withHttpRequest_withHttpRequest);
// CONCATENATED MODULE: ./src/app/components/files/fileTransfer.jsx
var files_fileTransfer_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function files_fileTransfer_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function files_fileTransfer_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function files_fileTransfer_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

/*
Copyright 2018 Gravitational, Inc.

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







var fileTransfer_FileTransfer = function FileTransfer(_ref) {
  var files = _ref.files;

  if (files.length === 0) {
    return null;
  }

  var $files = files.map(function (file) {
    var key = file.id;
    return file.isUpload ? react_default.a.createElement(FileToSend, { key: key, file: file }) : react_default.a.createElement(FileToReceive, { key: key, file: file });
  });

  return react_default.a.createElement(
    'div',
    { className: 'm-t-sm' },
    react_default.a.createElement('div', { className: 'grv-file-transfer-header m-b-sm' }),
    react_default.a.createElement(
      'div',
      { className: 'grv-file-transfer-file-list-cols' },
      react_default.a.createElement(
        items_Text,
        null,
        ' File '
      ),
      react_default.a.createElement(
        items_Text,
        null,
        'Status '
      ),
      react_default.a.createElement(
        'div',
        null,
        ' '
      )
    ),
    react_default.a.createElement(
      'div',
      { className: 'grv-file-transfer-content' },
      react_default.a.createElement(
        'div',
        { className: 'grv-file-transfer-file-list' },
        $files
      )
    )
  );
};

var fileTransfer_File = function (_Component) {
  files_fileTransfer_inherits(File, _Component);

  function File() {
    var _ref2;

    var _temp, _this, _ret;

    files_fileTransfer_classCallCheck(this, File);

    for (var _len = arguments.length, args = Array(_len), _key = 0; _key < _len; _key++) {
      args[_key] = arguments[_key];
    }

    return _ret = (_temp = (_this = files_fileTransfer_possibleConstructorReturn(this, (_ref2 = File.__proto__ || Object.getPrototypeOf(File)).call.apply(_ref2, [this].concat(args))), _this), _this.savedToDisk = false, _this.onRemove = function () {
      _this.props.onRemove();
    }, _temp), files_fileTransfer_possibleConstructorReturn(_this, _ret);
  }

  files_fileTransfer_createClass(File, [{
    key: 'componentDidUpdate',
    value: function componentDidUpdate() {
      var _props$file = this.props.file,
          isCompleted = _props$file.isCompleted,
          isUpload = _props$file.isUpload;

      if (isCompleted && !isUpload) {
        this.saveToDisk(this.props.httpResponse);
      }
    }
  }, {
    key: 'saveToDisk',
    value: function saveToDisk(_ref3) {
      var fileName = _ref3.fileName,
          blob = _ref3.blob;

      if (this.savedToDisk) {
        return;
      }

      this.savedToDisk = true;

      // if IE11
      if (window.navigator.msSaveOrOpenBlob) {
        window.navigator.msSaveOrOpenBlob(blob, fileName);
        return;
      }

      var a = document.createElement("a");
      a.href = window.URL.createObjectURL(blob);
      a.download = fileName;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
    }
  }, {
    key: 'render',
    value: function render() {
      var _props$file2 = this.props.file,
          name = _props$file2.name,
          isFailed = _props$file2.isFailed,
          isProcessing = _props$file2.isProcessing,
          isCompleted = _props$file2.isCompleted,
          error = _props$file2.error;
      var httpProgress = this.props.httpProgress;


      var className = classnames_default()("grv-file-transfer-file-list-item", isFailed && "--failed", isProcessing && "--processing", isCompleted && "--completed");

      return react_default.a.createElement(
        'div',
        { className: className },
        react_default.a.createElement(
          'div',
          { className: 'grv-file-transfer-file-path' },
          name,
          isFailed && react_default.a.createElement(
            'div',
            null,
            ' ',
            error,
            ' '
          )
        ),
        react_default.a.createElement(
          'div',
          { className: 'grv-file-transfer-file-status' },
          isFailed && react_default.a.createElement(
            'div',
            null,
            'failed'
          ),
          isProcessing && react_default.a.createElement(
            'div',
            null,
            httpProgress,
            '%'
          ),
          isCompleted && react_default.a.createElement(
            items_Text,
            null,
            'completed'
          )
        ),
        isProcessing && react_default.a.createElement(
          'div',
          { className: 'grv-file-transfer-file-close' },
          react_default.a.createElement(
            'a',
            { onClick: this.onRemove },
            'cancel'
          )
        )
      );
    }
  }]);

  return File;
}(react["Component"]);

fileTransfer_File.propTypes = {
  file: react["PropTypes"].object.isRequired,
  onRemove: react["PropTypes"].func.isRequired,
  httpResponse: react["PropTypes"].object
};
var FileToSend = files_withHttpRequest(fileTransfer_Uploader)(fileTransfer_File);
var FileToReceive = files_withHttpRequest(fileTransfer_Downloader)(fileTransfer_File);
// CONCATENATED MODULE: ./src/app/components/files/index.jsx
var files_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function files_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function files_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function files_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

/*
Copyright 2018 Gravitational, Inc.

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






var files_FileTransferDialog = function (_Component) {
  files_inherits(FileTransferDialog, _Component);

  function FileTransferDialog() {
    var _ref;

    var _temp, _this, _ret;

    files_classCallCheck(this, FileTransferDialog);

    for (var _len = arguments.length, args = Array(_len), _key = 0; _key < _len; _key++) {
      args[_key] = arguments[_key];
    }

    return _ret = (_temp = (_this = files_possibleConstructorReturn(this, (_ref = FileTransferDialog.__proto__ || Object.getPrototypeOf(FileTransferDialog)).call.apply(_ref, [this].concat(args))), _this), _this.onDownload = function (location) {
      _this.transfer(location, location, false);
    }, _this.onUpload = function (location, filename, blob) {
      _this.transfer(location, filename, true, blob);
    }, _this.onKeyDown = function (e) {
      // escape
      if (e.keyCode !== 27) {
        return;
      }

      e.preventDefault();
      e.stopPropagation();

      _this.onClose();
    }, _this.onClose = function () {
      var isTransfering = _this.props.store.isTransfering();
      if (!isTransfering) {
        _this.props.onClose();
      }

      if (isTransfering && window.confirm("Are you sure you want to cancel file transfers?")) {
        _this.props.onClose();
      }
    }, _temp), files_possibleConstructorReturn(_this, _ret);
  }

  files_createClass(FileTransferDialog, [{
    key: 'transfer',
    value: function transfer(location, name, isUpload) {
      var blob = arguments.length > 3 && arguments[3] !== undefined ? arguments[3] : [];

      this.props.onTransfer({
        location: location,
        name: name,
        isUpload: isUpload,
        blob: blob
      });
    }
  }, {
    key: 'render',
    value: function render() {
      var store = this.props.store;

      if (!store.isOpen) {
        return null;
      }

      var files = store.files,
          isUpload = store.isUpload;

      var latestFirst = files.toArray().reverse();
      return react_default.a.createElement(
        'div',
        { className: 'grv-file-transfer p-sm', onKeyDown: this.onKeyDown },
        !isUpload && react_default.a.createElement(download_FileDownloadSelector, { onDownload: this.onDownload }),
        isUpload && react_default.a.createElement(upload_FileUploadSelector, { onUpload: this.onUpload }),
        react_default.a.createElement(fileTransfer_FileTransfer, { files: latestFirst }),
        react_default.a.createElement(
          'div',
          { className: 'grv-file-transfer-footer' },
          react_default.a.createElement(
            'button',
            { onClick: this.onClose,
              className: 'btn btn-sm  grv-file-transfer-btn' },
            'Close'
          )
        )
      );
    }
  }]);

  return FileTransferDialog;
}(react["Component"]);
files_FileTransferDialog.propTypes = {
  store: react["PropTypes"].object.isRequired,
  onTransfer: react["PropTypes"].func.isRequired,
  onClose: react["PropTypes"].func.isRequired
};
// CONCATENATED MODULE: ./src/app/components/terminal/index.jsx
var terminal_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

var components_terminal_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function components_terminal_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function components_terminal_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function components_terminal_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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














var terminal_Page = function (_React$Component) {
  components_terminal_inherits(Page, _React$Component);

  function Page(props) {
    components_terminal_classCallCheck(this, Page);

    var _this = components_terminal_possibleConstructorReturn(this, (Page.__proto__ || Object.getPrototypeOf(Page)).call(this, props));

    _this.startNew = function () {
      var newRouteParams = terminal_extends({}, _this.props.routeParams, {
        sid: undefined
      });

      updateRoute(newRouteParams);
      initTerminal(newRouteParams);
    };

    _this.replay = function () {
      var _this$props$routePara = _this.props.routeParams,
          siteId = _this$props$routePara.siteId,
          sid = _this$props$routePara.sid;

      actions_open(siteId, sid);
    };

    _this.onCloseFileTransfer = function () {
      closeDialog();
      if (_this.termRef) {
        _this.termRef.focus();
      }
    };

    return _this;
  }

  components_terminal_createClass(Page, [{
    key: 'componentDidMount',
    value: function componentDidMount() {
      var _this2 = this;

      setTimeout(function () {
        return initTerminal(_this2.props.routeParams);
      }, 0);
    }
  }, {
    key: 'render',
    value: function render() {
      var _this3 = this;

      var _props = this.props,
          termStore = _props.termStore,
          fileStore = _props.fileStore;
      var status = termStore.status,
          sid = termStore.sid;

      var title = termStore.getServerLabel();

      var $content = null;
      var $leftPanelContent = null;

      if (status.isLoading) {
        $content = react_default.a.createElement(indicator, { type: 'bounce' });
      }

      if (status.isError) {
        $content = react_default.a.createElement(terminal_ErrorIndicator, { text: status.errorText });
      }

      if (status.isNotFound) {
        $content = react_default.a.createElement(terminal_SidNotFoundError, {
          onReplay: this.replay,
          onNew: this.startNew });
      }

      if (status.isReady) {
        var ttyParams = termStore.getTtyParams();
        $content = react_default.a.createElement(terminal_Terminal, { ref: function ref(e) {
            return _this3.termRef = e;
          },
          title: title,
          onSessionEnd: actions_close,
          ttyParams: ttyParams });
        $leftPanelContent = react_default.a.createElement(terminalPartyList, { sid: sid });
      }

      return react_default.a.createElement(
        'div',
        null,
        react_default.a.createElement(files_FileTransferDialog, {
          store: fileStore,
          onClose: this.onCloseFileTransfer,
          onTransfer: addFile
        }),
        react_default.a.createElement(
          'div',
          { className: 'grv-terminalhost' },
          react_default.a.createElement(
            terminalActionBar,
            null,
            $leftPanelContent
          ),
          react_default.a.createElement(
            'div',
            { className: 'grv-terminalhost-server-info' },
            react_default.a.createElement(
              'h3',
              null,
              title
            )
          ),
          $content
        )
      );
    }
  }]);

  return Page;
}(react_default.a.Component);

var terminal_ErrorIndicator = function ErrorIndicator(_ref) {
  var text = _ref.text;
  return react_default.a.createElement(
    'div',
    { className: 'grv-terminalhost-indicator-error' },
    react_default.a.createElement('i', { className: 'fa fa-exclamation-triangle fa-3x text-warning' }),
    react_default.a.createElement(
      'div',
      { className: 'm-l' },
      react_default.a.createElement(
        'strong',
        null,
        'Connection error'
      ),
      react_default.a.createElement(
        'div',
        null,
        react_default.a.createElement(
          'small',
          null,
          text
        )
      )
    )
  );
};

var terminal_SidNotFoundError = function SidNotFoundError(_ref2) {
  var onNew = _ref2.onNew,
      onReplay = _ref2.onReplay;
  return react_default.a.createElement(
    'div',
    { className: 'grv-terminalhost-indicator-error' },
    react_default.a.createElement(
      'div',
      { className: 'text-center' },
      react_default.a.createElement(
        'strong',
        null,
        'The session is no longer active'
      ),
      react_default.a.createElement(
        'div',
        { className: 'm-t' },
        react_default.a.createElement(
          'button',
          { onClick: onNew, className: 'btn btn-sm btn-primary m-r' },
          ' Start New '
        ),
        react_default.a.createElement(
          'button',
          { onClick: onReplay, className: 'btn btn-sm btn-primary' },
          ' Replay '
        )
      )
    )
  );
};

function terminal_mapStateToProps() {
  return {
    termStore: terminal_getters.store,
    fileStore: fileTransfer_getters.store
  };
}

/* harmony default export */ var components_terminal = (Object(build["connect"])(terminal_mapStateToProps)(terminal_Page));
// CONCATENATED MODULE: ./src/app/features/featureSsh.js
var featureSsh_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function featureSsh_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function featureSsh_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function featureSsh_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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







var sshRoutes = [{
  path: config["a" /* default */].routes.nodes,
  title: "Nodes",
  component: main
}, {
  path: config["a" /* default */].routes.terminal,
  title: "Terminal",
  components: {
    CurrentSessionHost: components_terminal
  }
}];

var sshNavItem = {
  icon: 'fa fa-share-alt',
  to: config["a" /* default */].routes.nodes,
  title: 'Nodes'
};

var featureSsh_SshFeature = function (_FeatureBase) {
  featureSsh_inherits(SshFeature, _FeatureBase);

  function SshFeature(routes) {
    featureSsh_classCallCheck(this, SshFeature);

    var _this = featureSsh_possibleConstructorReturn(this, (SshFeature.__proto__ || Object.getPrototypeOf(SshFeature)).call(this));

    routes.push.apply(routes, sshRoutes);
    return _this;
  }

  featureSsh_createClass(SshFeature, [{
    key: 'onload',
    value: function onload() {
      addNavItem(sshNavItem);
    }
  }]);

  return SshFeature;
}(featureBase);

/* harmony default export */ var featureSsh = (featureSsh_SshFeature);
// EXTERNAL MODULE: ./src/app/flux/storedSessionsFilter/actions.js
var storedSessionsFilter_actions = __webpack_require__("VW/0");

// EXTERNAL MODULE: ./src/app/flux/storedSessionsFilter/getters.js
var storedSessionsFilter_getters = __webpack_require__("F+qq");

// CONCATENATED MODULE: ./src/app/components/dataProvider.jsx
var dataProvider_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function dataProvider_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function dataProvider_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function dataProvider_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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



var DEFAULT_INTERVAL = 3000; // every 3 sec

var DataProvider = function (_Component) {
  dataProvider_inherits(DataProvider, _Component);

  function DataProvider(props) {
    dataProvider_classCallCheck(this, DataProvider);

    var _this = dataProvider_possibleConstructorReturn(this, (DataProvider.__proto__ || Object.getPrototypeOf(DataProvider)).call(this, props));

    _this._timerId = null;
    _this._request = null;

    _this._intervalTime = props.time || DEFAULT_INTERVAL;
    return _this;
  }

  dataProvider_createClass(DataProvider, [{
    key: 'fetch',
    value: function fetch() {
      var _this2 = this;

      // do not refetch if still in progress
      if (this._request) {
        return;
      }

      this._request = this.props.onFetch().always(function () {
        _this2._request = null;
      });
    }
  }, {
    key: 'componentDidMount',
    value: function componentDidMount() {
      this.fetch();
      this._timerId = setInterval(this.fetch.bind(this), this._intervalTime);
    }
  }, {
    key: 'componentWillUnmount',
    value: function componentWillUnmount() {
      clearInterval(this._timerId);
    }
  }, {
    key: 'render',
    value: function render() {
      return null;
    }
  }]);

  return DataProvider;
}(react["Component"]);

/* harmony default export */ var dataProvider = (DataProvider);
// EXTERNAL MODULE: ./src/app/flux/storedSessionsFilter/index.js
var flux_storedSessionsFilter = __webpack_require__("BBHA");

// CONCATENATED MODULE: ./src/app/components/layout.jsx
var layout_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

function layout_objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }

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



var styles = {
  flex: {
    display: 'flex'
  },

  justify: {
    start: {
      justifyContent: 'flex-start'
    },

    end: {
      justifyContent: 'flex-end'
    },

    between: {
      justifyContent: 'space-between'
    }
  },

  align: {
    center: {
      alignItems: 'center'
    },

    start: {
      alignItems: 'flex-start'
    },

    end: {
      alignItems: 'flex-end'
    },

    baseline: {
      alignItems: 'baseline'
    }
  },

  dir: {

    row: {
      flexDirection: 'row'
    },

    col: {
      flexDirection: 'column'
    }
  }
};

var getStyle = function getStyle(_ref) {
  var _ref$dir = _ref.dir,
      dir = _ref$dir === undefined ? 'col' : _ref$dir,
      _ref$align = _ref.align,
      align = _ref$align === undefined ? 'start' : _ref$align,
      _ref$justify = _ref.justify,
      justify = _ref$justify === undefined ? 'start' : _ref$justify,
      _ref$style = _ref.style,
      style = _ref$style === undefined ? {} : _ref$style;

  return layout_extends({}, style, styles.flex, styles.dir[dir], styles.justify[justify], styles.align[align]);
};

var layout_Flex = function Flex(_ref2) {
  var _ref2$className = _ref2.className,
      className = _ref2$className === undefined ? '' : _ref2$className,
      children = _ref2.children,
      props = layout_objectWithoutProperties(_ref2, ['className', 'children']);

  return react_default.a.createElement(
    'div',
    { className: className, style: getStyle(props) },
    children
  );
};

/* harmony default export */ var layout = ({
  Flex: layout_Flex
});
// EXTERNAL MODULE: ./node_modules/react-overlays/lib/index.js
var react_overlays_lib = __webpack_require__("X2Fi");

// CONCATENATED MODULE: ./src/app/components/overlayTrigger.jsx
var overlayTrigger_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function overlayTrigger_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function overlayTrigger_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function overlayTrigger_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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





var triggerType = react_default.a.PropTypes.oneOf(['click', 'hover', 'focus']);

var propTypes = {

  trigger: react_default.a.PropTypes.oneOfType([triggerType, react_default.a.PropTypes.arrayOf(triggerType)]),

  delay: react_default.a.PropTypes.number,

  delayShow: react_default.a.PropTypes.number,

  delayHide: react_default.a.PropTypes.number,

  defaultOverlayShown: react_default.a.PropTypes.bool,

  overlay: react_default.a.PropTypes.node.isRequired,

  onBlur: react_default.a.PropTypes.func,

  onClick: react_default.a.PropTypes.func,

  onFocus: react_default.a.PropTypes.func,

  onMouseOut: react_default.a.PropTypes.func,

  onMouseOver: react_default.a.PropTypes.func,

  target: react_default.a.PropTypes.oneOf([null]),

  onHide: react_default.a.PropTypes.oneOf([null]),

  show: react_default.a.PropTypes.oneOf([null])
};

var defaultProps = {
  defaultOverlayShown: false,
  trigger: ['hover', 'focus']
};

var overlayTrigger_OverlayTrigger = function (_React$Component) {
  overlayTrigger_inherits(OverlayTrigger, _React$Component);

  function OverlayTrigger(props, context) {
    overlayTrigger_classCallCheck(this, OverlayTrigger);

    var _this = overlayTrigger_possibleConstructorReturn(this, (OverlayTrigger.__proto__ || Object.getPrototypeOf(OverlayTrigger)).call(this, props, context));

    _this.getElement = _this.getElement.bind(_this);
    _this.handleToggle = _this.handleToggle.bind(_this);
    _this.handleHide = _this.handleHide.bind(_this);
    _this.state = {
      show: props.defaultOverlayShown
    };
    return _this;
  }

  overlayTrigger_createClass(OverlayTrigger, [{
    key: 'handleToggle',
    value: function handleToggle() {
      if (this.state.show) {
        this.hide();
      } else {
        this.show();
      }
    }
  }, {
    key: 'handleHide',
    value: function handleHide() {
      this.hide();
    }
  }, {
    key: 'show',
    value: function show() {
      this.setState({ show: true });
    }
  }, {
    key: 'hide',
    value: function hide() {
      this.setState({ show: false });
    }
  }, {
    key: 'getElement',
    value: function getElement() {
      return react_dom_default.a.findDOMNode(this);
    }
  }, {
    key: 'render',
    value: function render() {
      var _this2 = this;

      var _props = this.props,
          _props$container = _props.container,
          container = _props$container === undefined ? this : _props$container,
          placement = _props.placement,
          overlay = _props.overlay;

      return react_default.a.createElement(
        'div',
        { onClick: this.handleToggle },
        this.props.children,
        react_default.a.createElement(
          react_overlays_lib["Overlay"],
          {
            rootClose: true,
            placement: placement,
            show: this.state.show,
            onHide: this.handleHide,
            target: function target() {
              return _this2.getElement();
            },
            container: container },
          overlay
        )
      );
    }
  }]);

  return OverlayTrigger;
}(react_default.a.Component);

overlayTrigger_OverlayTrigger.propTypes = propTypes;
overlayTrigger_OverlayTrigger.defaultProps = defaultProps;

/* harmony default export */ var overlayTrigger = (overlayTrigger_OverlayTrigger);
// CONCATENATED MODULE: ./src/app/components/moreButton.jsx
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





var moreButton_classes = {
  'btn grv-btn-details': true
};

var moreButton_MoreButton = function MoreButton(props) {
  return react_default.a.createElement(
    'button',
    { className: classnames_default()(props.className, moreButton_classes) },
    react_default.a.createElement(
      'span',
      null,
      '\u2026'
    )
  );
};

moreButton_MoreButton.WithOverlay = function (props) {
  return react_default.a.createElement(
    overlayTrigger,
    props,
    react_default.a.createElement(moreButton_MoreButton, null)
  );
};

/* harmony default export */ var moreButton = (moreButton_MoreButton);
// CONCATENATED MODULE: ./src/app/components/popover.jsx
var popover_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

var popover_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function popover_defineProperty(obj, key, value) { if (key in obj) { Object.defineProperty(obj, key, { value: value, enumerable: true, configurable: true, writable: true }); } else { obj[key] = value; } return obj; }

function popover_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function popover_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function popover_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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




var popover_propTypes = {
  placement: react_default.a.PropTypes.oneOf(['top', 'right', 'bottom', 'left']),
  positionTop: react_default.a.PropTypes.oneOfType([react_default.a.PropTypes.number, react_default.a.PropTypes.string]),
  positionLeft: react_default.a.PropTypes.oneOfType([react_default.a.PropTypes.number, react_default.a.PropTypes.string]),
  arrowOffsetTop: react_default.a.PropTypes.oneOfType([react_default.a.PropTypes.number, react_default.a.PropTypes.string]),
  arrowOffsetLeft: react_default.a.PropTypes.oneOfType([react_default.a.PropTypes.number, react_default.a.PropTypes.string]),
  title: react_default.a.PropTypes.node
};

var popover_defaultProps = {
  placement: 'right'
};

var popover_Popover = function (_React$Component) {
  popover_inherits(Popover, _React$Component);

  function Popover() {
    popover_classCallCheck(this, Popover);

    return popover_possibleConstructorReturn(this, (Popover.__proto__ || Object.getPrototypeOf(Popover)).apply(this, arguments));
  }

  popover_createClass(Popover, [{
    key: 'render',
    value: function render() {
      var _props = this.props,
          placement = _props.placement,
          positionTop = _props.positionTop,
          positionLeft = _props.positionLeft,
          arrowOffsetTop = _props.arrowOffsetTop,
          arrowOffsetLeft = _props.arrowOffsetLeft,
          title = _props.title,
          className = _props.className,
          style = _props.style,
          children = _props.children;


      var classes = popover_defineProperty({
        'popover': true
      }, placement, true);

      var outerStyle = popover_extends({
        display: 'block',
        top: positionTop,
        left: positionLeft
      }, style);

      var arrowStyle = {
        top: arrowOffsetTop,
        left: arrowOffsetLeft
      };

      return react_default.a.createElement(
        'div',
        {
          role: 'tooltip',
          className: classnames_default()(className, classes),
          style: outerStyle },
        react_default.a.createElement('div', { className: 'arrow', style: arrowStyle }),
        title && react_default.a.createElement(
          'h3',
          { className: 'popover-title' },
          title
        ),
        react_default.a.createElement(
          'div',
          { className: 'popover-content' },
          children
        )
      );
    }
  }]);

  return Popover;
}(react_default.a.Component);

popover_Popover.propTypes = popover_propTypes;
popover_Popover.defaultProps = popover_defaultProps;

/* harmony default export */ var popover = (popover_Popover);
// CONCATENATED MODULE: ./src/app/components/sessions/listItems.jsx
function listItems_objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }

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










var listItems_DateCreatedCell = function DateCreatedCell(_ref) {
  var rowIndex = _ref.rowIndex,
      data = _ref.data,
      props = listItems_objectWithoutProperties(_ref, ['rowIndex', 'data']);

  var createdDisplayText = data[rowIndex].createdDisplayText;

  return react_default.a.createElement(
    table_TableCell,
    props,
    createdDisplayText
  );
};

var listItems_DurationCell = function DurationCell(_ref2) {
  var rowIndex = _ref2.rowIndex,
      data = _ref2.data,
      props = listItems_objectWithoutProperties(_ref2, ['rowIndex', 'data']);

  var duration = data[rowIndex].duration;

  var displayDate = moment_default.a.duration(duration).humanize();
  return react_default.a.createElement(
    table_TableCell,
    props,
    displayDate
  );
};

var listItems_SingleUserCell = function SingleUserCell(_ref3) {
  var rowIndex = _ref3.rowIndex,
      data = _ref3.data,
      props = listItems_objectWithoutProperties(_ref3, ['rowIndex', 'data']);

  var user = data[rowIndex].user;

  return react_default.a.createElement(
    table_TableCell,
    props,
    react_default.a.createElement(
      'span',
      { className: 'grv-sessions-user label label-default' },
      user
    )
  );
};

var listItems_UsersCell = function UsersCell(_ref4) {
  var rowIndex = _ref4.rowIndex,
      data = _ref4.data,
      props = listItems_objectWithoutProperties(_ref4, ['rowIndex', 'data']);

  var _data$rowIndex = data[rowIndex],
      parties = _data$rowIndex.parties,
      user = _data$rowIndex.user;

  var $users = react_default.a.createElement(
    'div',
    { className: 'grv-sessions-user' },
    user
  );

  if (parties.length > 0) {
    $users = parties.map(function (item, itemIndex) {
      return react_default.a.createElement(
        'div',
        { key: itemIndex, className: 'grv-sessions-user' },
        item
      );
    });
  }

  return react_default.a.createElement(
    table_TableCell,
    props,
    react_default.a.createElement(
      'div',
      null,
      $users
    )
  );
};

var listItems_sessionInfo = function sessionInfo(sid) {
  return react_default.a.createElement(
    popover,
    { className: 'grv-sessions-stored-details' },
    react_default.a.createElement(
      'div',
      null,
      sid
    )
  );
};

var listItems_SessionIdCell = function SessionIdCell(_ref5) {
  var rowIndex = _ref5.rowIndex,
      canJoin = _ref5.canJoin,
      data = _ref5.data,
      container = _ref5.container,
      props = listItems_objectWithoutProperties(_ref5, ['rowIndex', 'canJoin', 'data', 'container']);

  var _data$rowIndex2 = data[rowIndex],
      sessionUrl = _data$rowIndex2.sessionUrl,
      active = _data$rowIndex2.active,
      sid = _data$rowIndex2.sid;

  var isDisabled = active && !canJoin;
  var sidShort = sid.slice(0, 8);
  var actionText = active ? 'join' : 'play';

  var btnClass = classnames_default()('btn btn-xs m-r-sm', {
    'btn-primary': !active,
    'btn-warning': active,
    'disabled': isDisabled
  });

  return react_default.a.createElement(
    table_TableCell,
    props,
    react_default.a.createElement(
      layout.Flex,
      { dir: 'row', align: 'center' },
      isDisabled && react_default.a.createElement(
        'button',
        { className: btnClass },
        actionText
      ),
      !isDisabled && react_default.a.createElement(
        lib["Link"],
        { to: sessionUrl, className: btnClass, type: 'button' },
        actionText
      ),
      react_default.a.createElement(
        'span',
        { style: { width: "75px" } },
        sidShort
      ),
      react_default.a.createElement(moreButton.WithOverlay, {
        trigger: 'click',
        placement: 'bottom',
        container: container,
        overlay: listItems_sessionInfo(sid) })
    )
  );
};

var listItems_NodeCell = function NodeCell(_ref6) {
  var rowIndex = _ref6.rowIndex,
      data = _ref6.data,
      props = listItems_objectWithoutProperties(_ref6, ['rowIndex', 'data']);

  var nodeDisplayText = data[rowIndex].nodeDisplayText;

  return react_default.a.createElement(
    table_TableCell,
    props,
    nodeDisplayText
  );
};


// CONCATENATED MODULE: ./src/app/components/datePicker.jsx
var datePicker_slicedToArray = function () { function sliceIterator(arr, i) { var _arr = []; var _n = true; var _d = false; var _e = undefined; try { for (var _i = arr[Symbol.iterator](), _s; !(_n = (_s = _i.next()).done); _n = true) { _arr.push(_s.value); if (i && _arr.length === i) break; } } catch (err) { _d = true; _e = err; } finally { try { if (!_n && _i["return"]) _i["return"](); } finally { if (_d) throw _e; } } return _arr; } return function (arr, i) { if (Array.isArray(arr)) { return arr; } else if (Symbol.iterator in Object(arr)) { return sliceIterator(arr, i); } else { throw new TypeError("Invalid attempt to destructure non-iterable instance"); } }; }();

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






var DateRangePicker = react_default.a.createClass({
  displayName: 'DateRangePicker',
  getDates: function getDates() {
    var startDate = jquery_default()(this.refs.dpPicker1).datepicker('getDate');
    var endDate = jquery_default()(this.refs.dpPicker2).datepicker('getDate');
    return [startDate, moment_default()(endDate).endOf('day').toDate()];
  },
  setDates: function setDates(_ref) {
    var startDate = _ref.startDate,
        endDate = _ref.endDate;

    jquery_default()(this.refs.dpPicker1).datepicker('setDate', startDate);
    jquery_default()(this.refs.dpPicker2).datepicker('setDate', endDate);
  },
  getDefaultProps: function getDefaultProps() {
    return {
      startDate: moment_default()().startOf('month').toDate(),
      endDate: moment_default()().endOf('month').toDate(),
      onChange: function onChange() {}
    };
  },
  componentWillUnmount: function componentWillUnmount() {
    jquery_default()(this.refs.dp).datepicker('destroy');
  },
  componentWillReceiveProps: function componentWillReceiveProps(newProps) {
    var _getDates = this.getDates(),
        _getDates2 = datePicker_slicedToArray(_getDates, 2),
        startDate = _getDates2[0],
        endDate = _getDates2[1];

    if (!(isSame(startDate, newProps.startDate) && isSame(endDate, newProps.endDate))) {
      this.setDates(newProps);
    }
  },
  shouldComponentUpdate: function shouldComponentUpdate() {
    return false;
  },
  componentDidMount: function componentDidMount() {
    this.onChange = Object(lodash["debounce"])(this.onChange, 1);
    jquery_default()(this.refs.rangePicker).datepicker({
      todayBtn: 'linked',
      keyboardNavigation: false,
      forceParse: false,
      calendarWeeks: true,
      autoclose: true
    });

    this.setDates(this.props);

    jquery_default()(this.refs.rangePicker).datepicker().on('changeDate', this.onChange);
  },
  onChange: function onChange() {
    var _getDates3 = this.getDates(),
        _getDates4 = datePicker_slicedToArray(_getDates3, 2),
        startDate = _getDates4[0],
        endDate = _getDates4[1];

    if (!(isSame(startDate, this.props.startDate) && isSame(endDate, this.props.endDate))) {
      this.props.onChange({ startDate: startDate, endDate: endDate });
    }
  },
  render: function render() {
    return react_default.a.createElement(
      'div',
      { className: 'grv-datepicker input-group input-group-sm input-daterange', ref: 'rangePicker' },
      react_default.a.createElement('input', { ref: 'dpPicker1', type: 'text', className: 'input-sm form-control', name: 'start' }),
      react_default.a.createElement(
        'span',
        { className: 'input-group-addon' },
        'to'
      ),
      react_default.a.createElement('input', { ref: 'dpPicker2', type: 'text', className: 'input-sm form-control', name: 'end' })
    );
  }
});

function isSame(date1, date2) {
  return moment_default()(date1).isSame(date2, 'day');
}

/* harmony default export */ var datePicker = (DateRangePicker);
// CONCATENATED MODULE: ./src/app/components/sessions/sessionList.jsx
var sessionList_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function sessionList_toConsumableArray(arr) { if (Array.isArray(arr)) { for (var i = 0, arr2 = Array(arr.length); i < arr.length; i++) { arr2[i] = arr[i]; } return arr2; } else { return Array.from(arr); } }

function sessionList_defineProperty(obj, key, value) { if (key in obj) { Object.defineProperty(obj, key, { value: value, enumerable: true, configurable: true, writable: true }); } else { obj[key] = value; } return obj; }

function sessionList_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function sessionList_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function sessionList_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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













var sessionList_SessionList = function (_React$Component) {
  sessionList_inherits(SessionList, _React$Component);

  function SessionList(props) {
    sessionList_classCallCheck(this, SessionList);

    var _this = sessionList_possibleConstructorReturn(this, (SessionList.__proto__ || Object.getPrototypeOf(SessionList)).call(this, props));

    _this.searchableProps = ['nodeDisplayText', 'createdDisplayText', 'sid', 'parties'];
    _this._mounted = false;

    _this.onSearchChange = function (value) {
      _this.state.searchValue = value;
      _this.setState(_this.state);
    };

    _this.onSortChange = function (columnKey, sortDir) {
      _this.state.colSortDirs = sessionList_defineProperty({}, columnKey, sortDir);
      _this.setState(_this.state);
    };

    _this.onRangePickerChange = function (_ref) {
      var startDate = _ref.startDate,
          endDate = _ref.endDate;

      /**
      * as date picker uses timeouts its important to ensure that
      * component is still mounted when data picker triggers an update
      */
      if (_this._mounted) {
        flux_storedSessionsFilter["actions"].setTimeRange(startDate, endDate);
      }
    };

    if (props.storage) {
      _this.state = props.storage.findByKey('SessionList');
    }

    if (!_this.state) {
      _this.state = { searchValue: '', colSortDirs: { created: 'ASC' } };
    }
    return _this;
  }

  sessionList_createClass(SessionList, [{
    key: 'componentDidMount',
    value: function componentDidMount() {
      this._mounted = true;
    }
  }, {
    key: 'componentWillUnmount',
    value: function componentWillUnmount() {
      this._mounted = false;
      if (this.props.storage) {
        this.props.storage.save('SessionList', this.state);
      }
    }
  }, {
    key: 'searchAndFilterCb',
    value: function searchAndFilterCb(targetValue, searchValue, propName) {
      if (propName === 'parties') {
        targetValue = targetValue || [];
        return targetValue.join('').toLocaleUpperCase().indexOf(searchValue) !== -1;
      }
    }
  }, {
    key: 'sortAndFilter',
    value: function sortAndFilter(data) {
      var _this2 = this;

      var filtered = data.filter(function (obj) {
        return isMatch(obj, _this2.state.searchValue, {
          searchableProps: _this2.searchableProps,
          cb: _this2.searchAndFilterCb
        });
      });

      var columnKey = Object.getOwnPropertyNames(this.state.colSortDirs)[0];
      var sortDir = this.state.colSortDirs[columnKey];
      var sorted = Object(lodash["sortBy"])(filtered, columnKey);
      if (sortDir === SortTypes.ASC) {
        sorted = sorted.reverse();
      }

      return sorted;
    }
  }, {
    key: 'render',
    value: function render() {
      var _props = this.props,
          filter = _props.filter,
          storedSessions = _props.storedSessions,
          activeSessions = _props.activeSessions;
      var start = filter.start,
          end = filter.end;

      var canJoin = config["a" /* default */].canJoinSessions;
      var searchValue = this.state.searchValue;

      var stored = storedSessions.filter(function (item) {
        return moment_default()(item.created).isBetween(start, end);
      });

      var active = activeSessions.filter(function (item) {
        return item.parties.length > 0;
      }).filter(function (item) {
        return moment_default()(item.created).isBetween(start, end);
      });

      stored = this.sortAndFilter(stored);
      active = this.sortAndFilter(active);

      // always display active sessions first
      var data = [].concat(sessionList_toConsumableArray(active), sessionList_toConsumableArray(stored));
      return react_default.a.createElement(
        'div',
        { className: 'grv-sessions-stored m-t' },
        react_default.a.createElement(
          'div',
          { className: 'grv-header' },
          react_default.a.createElement(
            'div',
            { className: 'grv-flex m-b-md', style: { justifyContent: "space-between" } },
            react_default.a.createElement(
              'div',
              { className: 'grv-flex' },
              react_default.a.createElement(
                'h2',
                { className: 'text-center' },
                ' Sessions '
              )
            ),
            react_default.a.createElement(
              'div',
              { className: 'grv-flex' },
              react_default.a.createElement(clusterSelector, null),
              react_default.a.createElement(inputSearch, { autoFocus: true, value: searchValue, onChange: this.onSearchChange }),
              react_default.a.createElement(
                'div',
                { className: 'm-l-sm' },
                react_default.a.createElement(datePicker, { startDate: start, endDate: end, onChange: this.onRangePickerChange })
              )
            )
          )
        ),
        react_default.a.createElement(
          'div',
          { className: 'grv-content' },
          data.length === 0 ? react_default.a.createElement(table_EmptyIndicator, { text: 'No matching sessions found' }) : react_default.a.createElement(
            table_Table,
            {
              data: data,
              rowCount: data.length },
            react_default.a.createElement(Column, {
              header: react_default.a.createElement(
                table_TableCell,
                { className: 'grv-sessions-col-sid' },
                ' Session ID '
              ),
              cell: react_default.a.createElement(listItems_SessionIdCell, { canJoin: canJoin, container: this })
            }),
            react_default.a.createElement(Column, {
              header: react_default.a.createElement(
                table_TableCell,
                null,
                ' User '
              ),
              cell: react_default.a.createElement(listItems_UsersCell, null)
            }),
            react_default.a.createElement(Column, {
              columnKey: 'nodeIp',
              header: react_default.a.createElement(
                table_TableCell,
                { className: 'grv-sessions-stored-col-ip' },
                'Node'
              ),
              cell: react_default.a.createElement(listItems_NodeCell, null)
            }),
            react_default.a.createElement(Column, {
              columnKey: 'created',
              header: react_default.a.createElement(table_SortHeaderCell, {
                sortDir: this.state.colSortDirs.created,
                onSortChange: this.onSortChange,
                title: 'Created (UTC)'
              }),
              cell: react_default.a.createElement(listItems_DateCreatedCell, null)
            }),
            react_default.a.createElement(Column, {
              columnKey: 'duration',
              header: react_default.a.createElement(table_SortHeaderCell, {
                sortDir: this.state.colSortDirs.duration,
                onSortChange: this.onSortChange,
                title: 'Duration'
              }),
              cell: react_default.a.createElement(listItems_DurationCell, null)
            })
          )
        )
      );
    }
  }]);

  return SessionList;
}(react_default.a.Component);

/* harmony default export */ var sessionList = (sessionList_SessionList);
// CONCATENATED MODULE: ./src/app/components/sessions/main.jsx
var main_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function main_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function main_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function main_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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












var main_Sessions = function (_React$Component) {
  main_inherits(Sessions, _React$Component);

  function Sessions() {
    var _ref;

    var _temp, _this, _ret;

    main_classCallCheck(this, Sessions);

    for (var _len = arguments.length, args = Array(_len), _key = 0; _key < _len; _key++) {
      args[_key] = arguments[_key];
    }

    return _ret = (_temp = (_this = main_possibleConstructorReturn(this, (_ref = Sessions.__proto__ || Object.getPrototypeOf(Sessions)).call.apply(_ref, [this].concat(args))), _this), _this.refresh = function () {
      return Object(storedSessionsFilter_actions["fetchSiteEventsWithinTimeRange"])();
    }, _temp), main_possibleConstructorReturn(_this, _ret);
  }

  main_createClass(Sessions, [{
    key: 'render',
    value: function render() {
      var _props = this.props,
          siteId = _props.siteId,
          storedSessions = _props.storedSessions,
          activeSessions = _props.activeSessions,
          storedSessionsFilter = _props.storedSessionsFilter;

      var title = siteId + ' \xB7 Sessions';
      return react_default.a.createElement(
        DocumentTitle,
        { title: title },
        react_default.a.createElement(
          'div',
          { className: 'grv-page grv-sessions' },
          react_default.a.createElement(sessionList, {
            storage: this.props.storage,
            activeSessions: activeSessions,
            storedSessions: storedSessions,
            filter: storedSessionsFilter
          }),
          react_default.a.createElement(dataProvider, { onFetch: this.refresh })
        )
      );
    }
  }]);

  return Sessions;
}(react_default.a.Component);

function main_mapFluxToProps() {
  return {
    siteId: app_getters["a" /* default */].siteId,
    activeSessions: sessions_getters.activeSessionList,
    storedSessions: sessions_getters.storedSessionList,
    storedSessionsFilter: storedSessionsFilter_getters["default"].filter
  };
}

var SessionsWithStorage = components_withStorage(main_Sessions);

/* harmony default export */ var sessions_main = (connect(main_mapFluxToProps)(SessionsWithStorage));
// EXTERNAL MODULE: ./node_modules/perfect-scrollbar/jquery.js
var perfect_scrollbar_jquery = __webpack_require__("Se41");
var perfect_scrollbar_jquery_default = /*#__PURE__*/__webpack_require__.n(perfect_scrollbar_jquery);

// EXTERNAL MODULE: ./node_modules/react-slider/react-slider.js
var react_slider = __webpack_require__("uz9f");
var react_slider_default = /*#__PURE__*/__webpack_require__.n(react_slider);

// CONCATENATED MODULE: ./src/app/lib/term/ttyPlayer.js
var ttyPlayer_slicedToArray = function () { function sliceIterator(arr, i) { var _arr = []; var _n = true; var _d = false; var _e = undefined; try { for (var _i = arr[Symbol.iterator](), _s; !(_n = (_s = _i.next()).done); _n = true) { _arr.push(_s.value); if (i && _arr.length === i) break; } } catch (err) { _d = true; _e = err; } finally { try { if (!_n && _i["return"]) _i["return"](); } finally { if (_d) throw _e; } } return _arr; } return function (arr, i) { if (Array.isArray(arr)) { return arr; } else if (Symbol.iterator in Object(arr)) { return sliceIterator(arr, i); } else { throw new TypeError("Invalid attempt to destructure non-iterable instance"); } }; }();

var ttyPlayer_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function ttyPlayer_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function ttyPlayer_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

function ttyPlayer_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

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








var ttyPlayer_logger = logger["a" /* default */].create('TtyPlayer');
var STREAM_START_INDEX = 0;
var URL_PREFIX_EVENTS = '/events';
var PLAY_SPEED = 5;
var Buffer = buffer_default.a.Buffer;

var MAX_SIZE = 5242880; // 5mg

var ttyPlayer_EventProvider = function () {
  function EventProvider(_ref) {
    var url = _ref.url;

    ttyPlayer_classCallCheck(this, EventProvider);

    this.url = url;
    this.events = [];
  }

  ttyPlayer_createClass(EventProvider, [{
    key: 'getDuration',
    value: function getDuration() {
      var eventCount = this.events.length;
      if (eventCount === 0) {
        return 0;
      }

      return this.events[eventCount - 1].msNormalized;
    }
  }, {
    key: 'init',
    value: function init() {
      var _this = this;

      return this._fetchEvents().then(function (events) {
        _this.events = events;
        var printEvents = _this.events.filter(onlyPrintEvents);
        if (printEvents.length === 0) {
          return;
        }

        return _this._fetchContent(printEvents).then(function (buffer) {
          _this._populatePrintEvents(buffer, printEvents);
        });
      });
    }
  }, {
    key: '_fetchEvents',
    value: function _fetchEvents() {
      var _this2 = this;

      var url = this.url + URL_PREFIX_EVENTS;
      return api["a" /* default */].get(url).then(function (json) {
        if (!json.events) {
          return [];
        }

        return _this2._createEvents(json.events);
      });
    }
  }, {
    key: '_fetchContent',
    value: function _fetchContent(events) {
      // calclulate the size of the session in bytes to know how many
      // chunks to load due to maximum chunk size.
      var offset = events[0].offset;
      var end = events.length - 1;
      var totalSize = events[end].offset - offset + events[end].bytes;
      var chunkCount = Math.ceil(totalSize / MAX_SIZE);

      // create a fetch request for each chunk
      var promises = [];
      for (var i = 0; i < chunkCount; i++) {
        var url = this.url + '/stream?offset=' + offset + '&bytes=' + MAX_SIZE;
        promises.push(api["a" /* default */].ajax({
          url: url,
          processData: true,
          dataType: 'text'
        }));

        offset = offset + MAX_SIZE;
      }

      // fetch all session chunks and then merge them in one
      return jquery_default.a.when.apply(jquery_default.a, promises).then(function () {
        for (var _len = arguments.length, responses = Array(_len), _key = 0; _key < _len; _key++) {
          responses[_key] = arguments[_key];
        }

        responses = promises.length === 1 ? [[responses]] : responses;
        var allBytes = responses.reduce(function (byteStr, r) {
          return byteStr + r[0];
        }, '');
        return new Buffer(allBytes);
      });
    }
  }, {
    key: '_populatePrintEvents',
    value: function _populatePrintEvents(buffer, events) {
      var byteStrOffset = events[0].bytes;
      events[0].data = buffer.slice(0, byteStrOffset).toString('utf8');
      for (var i = 1; i < events.length; i++) {
        var bytes = events[i].bytes;

        events[i].data = buffer.slice(byteStrOffset, byteStrOffset + bytes).toString('utf8');
        byteStrOffset += bytes;
      }
    }
  }, {
    key: '_createEvents',
    value: function _createEvents(json) {
      var w = void 0,
          h = void 0;
      var events = [];

      // filter print events and ensure that each has the right screen size and valid values
      for (var i = 0; i < json.length; i++) {
        var _json$i = json[i],
            ms = _json$i.ms,
            event = _json$i.event,
            offset = _json$i.offset,
            time = _json$i.time,
            bytes = _json$i.bytes;

        // grab new screen size for the next events

        if (event === EventTypeEnum.RESIZE || event === EventTypeEnum.START) {
          var _json$i$size$split = json[i].size.split(':');

          var _json$i$size$split2 = ttyPlayer_slicedToArray(_json$i$size$split, 2);

          w = _json$i$size$split2[0];
          h = _json$i$size$split2[1];
        }

        // session has ended, stop here
        if (event === EventTypeEnum.END) {
          var start = new Date(events[0].time);
          var end = new Date(time);
          var duration = end.getTime() - start.getTime();
          events.push({
            eventType: event,
            ms: duration,
            time: new Date(time)
          });

          break;
        }

        // process only PRINT events
        if (event !== EventTypeEnum.PRINT) {
          continue;
        }

        events.push({
          eventType: EventTypeEnum.PRINT,
          ms: ms,
          bytes: bytes,
          offset: offset,
          data: null,
          w: Number(w),
          h: Number(h),
          time: new Date(time)
        });
      }

      return this._normalizeEventsByTime(events);
    }
  }, {
    key: '_normalizeEventsByTime',
    value: function _normalizeEventsByTime(events) {
      if (!events || events.length === 0) {
        return [];
      }

      events.forEach(function (e) {
        e.displayTime = formatDisplayTime(e.ms);
        e.ms = e.ms > 0 ? Math.floor(e.ms / 10) : 0;
        e.msNormalized = e.ms;
      });

      var cur = events[0];
      var tmp = [];
      for (var i = 1; i < events.length; i++) {
        var sameSize = cur.w === events[i].w && cur.h === events[i].h;
        var delay = events[i].ms - cur.ms;

        // merge events with tiny delay
        if (delay < 2 && sameSize) {
          cur.bytes += events[i].bytes;
          continue;
        }

        // avoid long delays between chunks
        events[i].msNormalized = cur.msNormalized + shortenTime(delay);

        tmp.push(cur);
        cur = events[i];
      }

      if (tmp.indexOf(cur) === -1) {
        tmp.push(cur);
      }

      return tmp;
    }
  }]);

  return EventProvider;
}();

function formatDisplayTime(ms) {
  if (ms <= 0) {
    return '00:00';
  }

  var totalSec = Math.floor(ms / 1000);
  var totalDays = totalSec % 31536000 % 86400;
  var h = Math.floor(totalDays / 3600);
  var m = Math.floor(totalDays % 3600 / 60);
  var s = totalDays % 3600 % 60;

  m = m > 9 ? m : '0' + m;
  s = s > 9 ? s : '0' + s;
  h = h > 0 ? h + ':' : '';

  return '' + h + m + ':' + s;
}

function shortenTime(value) {
  if (value >= 25 && value < 50) {
    return 25;
  } else if (value >= 50 && value < 100) {
    return 50;
  } else if (value >= 100) {
    return 100;
  } else {
    return value;
  }
}

function onlyPrintEvents(e) {
  return e.eventType === EventTypeEnum.PRINT;
}

var ttyPlayer_TtyPlayer = function (_Tty) {
  ttyPlayer_inherits(TtyPlayer, _Tty);

  function TtyPlayer(_ref2) {
    var url = _ref2.url;

    ttyPlayer_classCallCheck(this, TtyPlayer);

    var _this3 = ttyPlayer_possibleConstructorReturn(this, (TtyPlayer.__proto__ || Object.getPrototypeOf(TtyPlayer)).call(this, {}));

    _this3.currentEventIndex = 0;
    _this3.current = 0;
    _this3.duration = 0;
    _this3.isPlaying = false;
    _this3.isError = false;
    _this3.isReady = false;
    _this3.isLoading = true;
    _this3.errText = '';

    _this3._posToEventIndexMap = [];
    _this3._eventProvider = new ttyPlayer_EventProvider({ url: url });
    return _this3;
  }

  // override


  ttyPlayer_createClass(TtyPlayer, [{
    key: 'send',
    value: function send() {}

    // override

  }, {
    key: 'connect',
    value: function connect() {
      var _this4 = this;

      this._setStatusFlag({ isLoading: true });
      this._eventProvider.init().then(function () {
        _this4._init();
        _this4._setStatusFlag({ isReady: true });
      }).fail(function (err) {
        ttyPlayer_logger.error('unable to init event provider', err);
        _this4.handleError(err);
      }).always(this._change.bind(this));

      this._change();
    }
  }, {
    key: 'handleError',
    value: function handleError(err) {
      this._setStatusFlag({
        isError: true,
        errText: api["a" /* default */].getErrorText(err)
      });
    }
  }, {
    key: '_init',
    value: function _init() {
      var _this5 = this;

      this.duration = this._eventProvider.getDuration();
      this._eventProvider.events.forEach(function (item) {
        return _this5._posToEventIndexMap.push(item.msNormalized);
      });
    }
  }, {
    key: 'move',
    value: function move(newPos) {
      if (!this.isReady) {
        return;
      }

      if (newPos === undefined) {
        newPos = this.current + 1;
      }

      if (newPos < 0) {
        newPos = 0;
      }

      if (newPos > this.duration) {
        this.stop();
      }

      var newEventIndex = this._getEventIndex(newPos) + 1;

      if (newEventIndex === this.currentEventIndex) {
        this.current = newPos;
        this._change();
        return;
      }

      var isRewind = this.currentEventIndex > newEventIndex;

      try {
        // we cannot playback the content within terminal so instead:
        // 1. tell terminal to reset.
        // 2. tell terminal to render 1 huge chunk that has everything up to current
        // location.
        if (isRewind) {
          this.emit(TermEventEnum.RESET);
        }

        var from = isRewind ? 0 : this.currentEventIndex;
        var to = newEventIndex;
        var events = this._eventProvider.events.slice(from, to);
        var printEvents = events.filter(onlyPrintEvents);

        this._display(printEvents);
        this.currentEventIndex = newEventIndex;
        this.current = newPos;
        this._change();
      } catch (err) {
        ttyPlayer_logger.error('move', err);
        this.handleError(err);
      }
    }
  }, {
    key: 'stop',
    value: function stop() {
      this.isPlaying = false;
      this.timer = clearInterval(this.timer);
      this._change();
    }
  }, {
    key: 'play',
    value: function play() {
      if (this.isPlaying) {
        return;
      }

      this.isPlaying = true;

      // start from the beginning if at the end
      if (this.current >= this.duration) {
        this.current = STREAM_START_INDEX;
        this.emit(TermEventEnum.RESET);
      }

      this.timer = setInterval(this.move.bind(this), PLAY_SPEED);
      this._change();
    }
  }, {
    key: 'getCurrentTime',
    value: function getCurrentTime() {
      if (this.currentEventIndex) {
        var displayTime = this._eventProvider.events[this.currentEventIndex - 1].displayTime;

        return displayTime;
      } else {
        return '--:--';
      }
    }
  }, {
    key: 'getEventCount',
    value: function getEventCount() {
      return this._eventProvider.events.length;
    }
  }, {
    key: '_display',
    value: function _display(events) {
      if (!events || events.length === 0) {
        return;
      }

      var groups = [{
        data: [events[0].data],
        w: events[0].w,
        h: events[0].h
      }];

      var cur = groups[0];

      // group events by screen size and construct 1 chunk of data per group
      for (var i = 1; i < events.length; i++) {
        if (cur.w === events[i].w && cur.h === events[i].h) {
          cur.data.push(events[i].data);
        } else {
          cur = {
            data: [events[i].data],
            w: events[i].w,
            h: events[i].h
          };

          groups.push(cur);
        }
      }

      // render each group
      for (var _i = 0; _i < groups.length; _i++) {
        var str = groups[_i].data.join('');
        var _groups$_i = groups[_i],
            h = _groups$_i.h,
            w = _groups$_i.w;

        if (str.length > 0) {
          this.emit(TermEventEnum.RESIZE, { h: h, w: w });
          this.emit(TermEventEnum.DATA, str);
        }
      }
    }
  }, {
    key: '_setStatusFlag',
    value: function _setStatusFlag(newStatus) {
      var _newStatus$isReady = newStatus.isReady,
          isReady = _newStatus$isReady === undefined ? false : _newStatus$isReady,
          _newStatus$isError = newStatus.isError,
          isError = _newStatus$isError === undefined ? false : _newStatus$isError,
          _newStatus$isLoading = newStatus.isLoading,
          isLoading = _newStatus$isLoading === undefined ? false : _newStatus$isLoading,
          _newStatus$errText = newStatus.errText,
          errText = _newStatus$errText === undefined ? '' : _newStatus$errText;


      this.isReady = isReady;
      this.isError = isError;
      this.isLoading = isLoading;
      this.errText = errText;
    }
  }, {
    key: '_getEventIndex',
    value: function _getEventIndex(num) {
      var arr = this._posToEventIndexMap;
      var low = 0;
      var hi = arr.length - 1;

      while (hi - low > 1) {
        var mid = Math.floor((low + hi) / 2);
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
    }
  }, {
    key: '_change',
    value: function _change() {
      this.emit('change');
    }
  }]);

  return TtyPlayer;
}(tty);

/* harmony default export */ var ttyPlayer = (ttyPlayer_TtyPlayer);

// CONCATENATED MODULE: ./src/app/components/player/items.jsx
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



var items_ErrorIndicator = function ErrorIndicator(_ref) {
  var text = _ref.text;
  return react_default.a.createElement(
    "div",
    { className: "grv-terminalhost-indicator-error" },
    react_default.a.createElement("i", { className: "fa fa-exclamation-triangle fa-3x text-warning" }),
    react_default.a.createElement(
      "div",
      { className: "m-l" },
      react_default.a.createElement(
        "strong",
        null,
        text || "Error"
      )
    )
  );
};

var items_WarningIndicator = function WarningIndicator(_ref2) {
  var text = _ref2.text;
  return react_default.a.createElement(
    "div",
    { className: "grv-terminalhost-indicator-error" },
    react_default.a.createElement(
      "h3",
      null,
      text
    )
  );
};
// CONCATENATED MODULE: ./src/app/components/player/player.jsx
var player_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

var _get = function get(object, property, receiver) { if (object === null) object = Function.prototype; var desc = Object.getOwnPropertyDescriptor(object, property); if (desc === undefined) { var parent = Object.getPrototypeOf(object); if (parent === null) { return undefined; } else { return get(parent, property, receiver); } } else if ("value" in desc) { return desc.value; } else { var getter = desc.get; if (getter === undefined) { return undefined; } return getter.call(receiver); } };

function player_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function player_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function player_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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










perfect_scrollbar_jquery_default()(jquery_default.a);

var player_Terminal = function (_GrvTerminal) {
  player_inherits(Terminal, _GrvTerminal);

  function Terminal(tty, el) {
    player_classCallCheck(this, Terminal);

    var _this = player_possibleConstructorReturn(this, (Terminal.__proto__ || Object.getPrototypeOf(Terminal)).call(this, { el: el, scrollBack: 1000 }));

    _this.tty = tty;
    return _this;
  }

  player_createClass(Terminal, [{
    key: 'connect',
    value: function connect() {}
  }, {
    key: 'open',
    value: function open() {
      _get(Terminal.prototype.__proto__ || Object.getPrototypeOf(Terminal.prototype), 'open', this).call(this);
      jquery_default()(this._el).perfectScrollbar();
    }
  }, {
    key: 'resize',
    value: function resize(cols, rows) {
      // ensure that cursor is visible as xterm hides it on blur event
      this.term.cursorState = 1;
      _get(Terminal.prototype.__proto__ || Object.getPrototypeOf(Terminal.prototype), 'resize', this).call(this, cols, rows);
      jquery_default()(this._el).perfectScrollbar('update');
    }
  }, {
    key: 'destroy',
    value: function destroy() {
      _get(Terminal.prototype.__proto__ || Object.getPrototypeOf(Terminal.prototype), 'destroy', this).call(this);
      jquery_default()(this._el).perfectScrollbar('destroy');
    }
  }, {
    key: '_disconnect',
    value: function _disconnect() {}
  }, {
    key: '_requestResize',
    value: function _requestResize() {}
  }]);

  return Terminal;
}(terminal);

var player_Content = function (_React$Component) {
  player_inherits(Content, _React$Component);

  function Content() {
    player_classCallCheck(this, Content);

    return player_possibleConstructorReturn(this, (Content.__proto__ || Object.getPrototypeOf(Content)).apply(this, arguments));
  }

  player_createClass(Content, [{
    key: 'componentDidMount',
    value: function componentDidMount() {
      var tty = this.props.tty;
      this.terminal = new player_Terminal(tty, this.refs.container);
      this.terminal.open();
    }
  }, {
    key: 'componentWillUnmount',
    value: function componentWillUnmount() {
      this.terminal.destroy();
    }
  }, {
    key: 'render',
    value: function render() {
      var isLoading = this.props.tty.isLoading;
      // need to hide the terminal cursor while fetching for events
      var style = {
        visibility: isLoading ? "hidden" : "initial"
      };

      return react_default.a.createElement('div', { style: style, ref: 'container' });
    }
  }]);

  return Content;
}(react_default.a.Component);

player_Content.propTypes = {
  tty: react_default.a.PropTypes.object.isRequired
};

var player_ControlPanel = function (_React$Component2) {
  player_inherits(ControlPanel, _React$Component2);

  function ControlPanel() {
    player_classCallCheck(this, ControlPanel);

    return player_possibleConstructorReturn(this, (ControlPanel.__proto__ || Object.getPrototypeOf(ControlPanel)).apply(this, arguments));
  }

  player_createClass(ControlPanel, [{
    key: 'componentDidMount',
    value: function componentDidMount() {
      var el = react_dom_default.a.findDOMNode(this);
      var btn = el.querySelector('.grv-session-player-controls button');
      btn && btn.focus();
    }
  }, {
    key: 'render',
    value: function render() {
      var _props = this.props,
          isPlaying = _props.isPlaying,
          min = _props.min,
          max = _props.max,
          value = _props.value,
          onChange = _props.onChange,
          onToggle = _props.onToggle,
          time = _props.time;

      var btnClass = isPlaying ? 'fa fa-stop' : 'fa fa-play';
      return react_default.a.createElement(
        'div',
        { className: 'grv-session-player-controls' },
        react_default.a.createElement(
          'button',
          { className: 'btn', onClick: onToggle },
          react_default.a.createElement('i', { className: btnClass })
        ),
        react_default.a.createElement(
          'div',
          { className: 'grv-session-player-controls-time' },
          time
        ),
        react_default.a.createElement(
          'div',
          { className: 'grv-flex-column' },
          react_default.a.createElement(react_slider_default.a, {
            min: min,
            max: max,
            value: value,
            onChange: onChange,
            defaultValue: 1,
            withBars: true,
            className: 'grv-slider' })
        )
      );
    }
  }]);

  return ControlPanel;
}(react_default.a.Component);

var player_Player = function (_React$Component3) {
  player_inherits(Player, _React$Component3);

  function Player(props) {
    player_classCallCheck(this, Player);

    var _this4 = player_possibleConstructorReturn(this, (Player.__proto__ || Object.getPrototypeOf(Player)).call(this, props));

    _this4.updateState = function () {
      var newState = _this4.calculateState();
      _this4.setState(newState);
    };

    _this4.onTogglePlayStop = function () {
      if (_this4.state.isPlaying) {
        _this4.tty.stop();
      } else {
        _this4.tty.play();
      }
    };

    _this4.onMove = function (value) {
      _this4.tty.move(value);
    };

    var url = _this4.props.url;

    _this4.tty = new ttyPlayer_TtyPlayer({ url: url });
    _this4.state = _this4.calculateState();
    return _this4;
  }

  player_createClass(Player, [{
    key: 'calculateState',
    value: function calculateState() {
      return {
        eventCount: this.tty.getEventCount(),
        duration: this.tty.duration,
        min: 1,
        time: this.tty.getCurrentTime(),
        isLoading: this.tty.isLoading,
        isPlaying: this.tty.isPlaying,
        isError: this.tty.isError,
        errText: this.tty.errText,
        current: this.tty.current,
        canPlay: this.tty.length > 1
      };
    }
  }, {
    key: 'componentDidMount',
    value: function componentDidMount() {
      this.tty.on('change', this.updateState);
      this.tty.connect();
      this.tty.play();
    }
  }, {
    key: 'componentWillUnmount',
    value: function componentWillUnmount() {
      this.tty.stop();
      this.tty.removeAllListeners();
    }
  }, {
    key: 'render',
    value: function render() {
      var _state = this.state,
          isPlaying = _state.isPlaying,
          isLoading = _state.isLoading,
          isError = _state.isError,
          errText = _state.errText,
          time = _state.time,
          min = _state.min,
          duration = _state.duration,
          current = _state.current,
          eventCount = _state.eventCount;


      if (isError) {
        return react_default.a.createElement(items_ErrorIndicator, { text: errText });
      }

      if (!isLoading && eventCount === 0) {
        return react_default.a.createElement(items_WarningIndicator, { text: 'The recording for this session is not available.' });
      }

      return react_default.a.createElement(
        'div',
        { className: 'grv-session-player-content' },
        react_default.a.createElement(player_Content, { tty: this.tty }),
        isLoading && react_default.a.createElement(indicator, null),
        eventCount > 0 && react_default.a.createElement(player_ControlPanel, {
          isPlaying: isPlaying,
          time: time,
          min: min,
          max: duration,
          value: current,
          onToggle: this.onTogglePlayStop,
          onChange: this.onMove })
      );
    }
  }]);

  return Player;
}(react_default.a.Component);
// CONCATENATED MODULE: ./src/app/components/player/playerHost.jsx
var playerHost_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function playerHost_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function playerHost_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function playerHost_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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








var playerHost_PlayerHost = function (_React$Component) {
  playerHost_inherits(PlayerHost, _React$Component);

  function PlayerHost() {
    playerHost_classCallCheck(this, PlayerHost);

    return playerHost_possibleConstructorReturn(this, (PlayerHost.__proto__ || Object.getPrototypeOf(PlayerHost)).apply(this, arguments));
  }

  playerHost_createClass(PlayerHost, [{
    key: 'componentWillMount',
    value: function componentWillMount() {
      var _props$params = this.props.params,
          sid = _props$params.sid,
          siteId = _props$params.siteId;

      this.url = config["a" /* default */].api.getFetchSessionUrl({ siteId: siteId, sid: sid });
    }
  }, {
    key: 'render',
    value: function render() {
      if (!this.url) {
        return null;
      }

      var siteId = this.props.params.siteId;

      var title = siteId + ' \xB7 Player';
      return react_default.a.createElement(
        DocumentTitle,
        { title: title },
        react_default.a.createElement(
          'div',
          { className: 'grv-terminalhost grv-session-player' },
          react_default.a.createElement(
            'div',
            { className: 'grv-session-player-actions m-t-md' },
            react_default.a.createElement(
              'div',
              { title: 'Close', style: playerHost_closeTextStyle, onClick: player_actions_close },
              react_default.a.createElement(icons_CloseIcon, null)
            )
          ),
          react_default.a.createElement(player_Player, { url: this.url })
        )
      );
    }
  }]);

  return PlayerHost;
}(react_default.a.Component);

var playerHost_closeTextStyle = {
  width: "30px",
  height: "30px",
  display: "block",
  margin: "0 auto"
};

/* harmony default export */ var playerHost = (playerHost_PlayerHost);
// CONCATENATED MODULE: ./src/app/features/featureAudit.js
var featureAudit_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function featureAudit_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function featureAudit_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function featureAudit_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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










var auditNavItem = {
  icon: 'fa  fa-group',
  to: config["a" /* default */].routes.sessions,
  title: 'Sessions'
};

var featureAudit_AuditFeature = function (_FeatureBase) {
  featureAudit_inherits(AuditFeature, _FeatureBase);

  featureAudit_createClass(AuditFeature, [{
    key: 'componentDidMount',
    value: function componentDidMount() {
      this.init();
    }
  }, {
    key: 'init',
    value: function init() {
      var _this2 = this;

      if (!this.wasInitialized()) {
        app_reactor["a" /* default */].batch(function () {
          _this2.startProcessing();
          Object(storedSessionsFilter_actions["fetchSiteEventsWithinTimeRange"])().done(_this2.stopProcessing.bind(_this2)).fail(_this2.handleError.bind(_this2));
        });
      }
    }
  }]);

  function AuditFeature(routes) {
    featureAudit_classCallCheck(this, AuditFeature);

    var _this = featureAudit_possibleConstructorReturn(this, (AuditFeature.__proto__ || Object.getPrototypeOf(AuditFeature)).call(this));

    var auditRoutes = [{
      path: config["a" /* default */].routes.sessions,
      title: "Sessions",
      component: _this.withMe(sessions_main)
    }, {
      path: config["a" /* default */].routes.player,
      title: "Player",
      components: {
        CurrentSessionHost: playerHost
      }
    }];

    routes.push.apply(routes, auditRoutes);
    return _this;
  }

  featureAudit_createClass(AuditFeature, [{
    key: 'onload',
    value: function onload() {
      var sessAccess = getAcl().getSessionAccess();
      if (sessAccess.list) {
        addNavItem(auditNavItem);
        this.init();
      }
    }
  }]);

  return AuditFeature;
}(featureBase);

/* harmony default export */ var featureAudit = (featureAudit_AuditFeature);
// CONCATENATED MODULE: ./src/app/featureActivator.js
var featureActivator_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function featureActivator_toConsumableArray(arr) { if (Array.isArray(arr)) { for (var i = 0, arr2 = Array(arr.length); i < arr.length; i++) { arr2[i] = arr[i]; } return arr2; } else { return Array.from(arr); } }

function featureActivator_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

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



var featureActivator_logger = logger["a" /* default */].create('featureActivator');

/**
 * Invokes methods on a group of registered features. 
 * 
 */

var featureActivator_FeactureActivator = function () {
  function FeactureActivator() {
    featureActivator_classCallCheck(this, FeactureActivator);

    this._features = [];
  }

  featureActivator_createClass(FeactureActivator, [{
    key: 'register',
    value: function register(feature) {
      if (!feature) {
        throw Error('Feature is undefined');
      }

      this._features.push(feature);
    }

    /**
     * to be called during app initialization. Becomes useful if feature wants to be
     * part of app initialization flow. 
     */

  }, {
    key: 'preload',
    value: function preload(context) {
      var promises = this._features.map(function (f) {
        var featurePromise = jquery_default.a.Deferred();
        // feature should handle failed promises thus always resolve.
        f.init(context).always(function () {
          featurePromise.resolve();
        });

        return featurePromise;
      });

      return jquery_default.a.when.apply(jquery_default.a, featureActivator_toConsumableArray(promises));
    }
  }, {
    key: 'onload',
    value: function onload(context) {
      var _this = this;

      this._features.forEach(function (f) {
        _this._invokeOnload(f, context);
      });
    }
  }, {
    key: 'getFirstAvailable',
    value: function getFirstAvailable() {
      return this._features.find(function (f) {
        return !f.isFailed();
      });
    }
  }, {
    key: 'getFeatures',
    value: function getFeatures() {
      return this._features;
    }
  }, {
    key: '_invokeOnload',
    value: function _invokeOnload(f) {
      try {
        for (var _len = arguments.length, props = Array(_len > 1 ? _len - 1 : 0), _key = 1; _key < _len; _key++) {
          props[_key - 1] = arguments[_key];
        }

        f.onload.apply(f, props);
      } catch (err) {
        featureActivator_logger.error('failed to invoke feature onload()', err);
      }
    }
  }]);

  return FeactureActivator;
}();

/* harmony default export */ var app_featureActivator = (featureActivator_FeactureActivator);
// CONCATENATED MODULE: ./src/app/flux/settings/getters.js
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

/* harmony default export */ var settings_getters = ({
  store: ['tlpt_settings']
});
// CONCATENATED MODULE: ./src/app/components/settings/main.jsx
var settings_main_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function settings_main_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function settings_main_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function settings_main_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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






var main_Separator = function Separator() {
  return react_default.a.createElement('div', { className: 'grv-settings-header-line-solid m-t-sm m-b-sm' });
};

var main_Settings = function (_React$Component) {
  settings_main_inherits(Settings, _React$Component);

  function Settings() {
    settings_main_classCallCheck(this, Settings);

    return settings_main_possibleConstructorReturn(this, (Settings.__proto__ || Object.getPrototypeOf(Settings)).apply(this, arguments));
  }

  settings_main_createClass(Settings, [{
    key: 'renderHeaderItem',
    value: function renderHeaderItem(item, key) {
      var to = item.to,
          isIndex = item.isIndex,
          title = item.title;

      var className = this.context.router.isActive(to, isIndex) ? "active" : "";
      return react_default.a.createElement(
        'li',
        { key: key, className: className },
        react_default.a.createElement(
          lib["Link"],
          { to: to },
          react_default.a.createElement(
            'h2',
            { className: 'm-b-xxs' },
            title
          )
        ),
        react_default.a.createElement(main_Separator, null)
      );
    }
  }, {
    key: 'render',
    value: function render() {
      var store = this.props.store;

      var $headerItems = store.getNavItems().map(this.renderHeaderItem.bind(this));

      if (!store.isReady()) {
        return null;
      }

      return react_default.a.createElement(
        'div',
        { className: 'grv-page grv-settings' },
        react_default.a.createElement(
          'ul',
          { className: 'grv-settings-header-menu' },
          $headerItems
        ),
        $headerItems.length > 0 && react_default.a.createElement(main_Separator, null),
        this.props.children
      );
    }
  }]);

  return Settings;
}(react_default.a.Component);

main_Settings.contextTypes = {
  router: react["PropTypes"].object.isRequired
};


function settings_main_mapStateToProps() {
  return {
    store: settings_getters.store
  };
}

/* harmony default export */ var settings_main = (Object(build["connect"])(settings_main_mapStateToProps)(main_Settings));
// CONCATENATED MODULE: ./src/app/flux/settings/actionTypes.js
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

var INIT = 'SETTINGS_INIT';
var actionTypes_ADD_NAV_ITEM = 'SETTINGS_ADD_NAV_ITEM';
var SET_RES_TO_DELETE = 'SETTINGS_SET_RES_TO_DELETE';
// CONCATENATED MODULE: ./src/app/flux/settings/actions.js
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






function actions_addNavItem(navItem) {
  app_reactor["a" /* default */].dispatch(actionTypes_ADD_NAV_ITEM, navItem);
}

function initSettings(featureActivator) {
  // init only once
  var store = app_reactor["a" /* default */].evaluate(settings_getters.store);
  if (store.isReady()) {
    return;
  }

  featureActivator.onload();
  app_reactor["a" /* default */].dispatch(INIT, {});
  initSettingsStatus.success();
}
// CONCATENATED MODULE: ./src/app/components/settings/index.jsx
var settings_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function settings_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function settings_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function settings_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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






var settings_SettingsIndex = function (_React$Component) {
  settings_inherits(SettingsIndex, _React$Component);

  function SettingsIndex() {
    settings_classCallCheck(this, SettingsIndex);

    return settings_possibleConstructorReturn(this, (SettingsIndex.__proto__ || Object.getPrototypeOf(SettingsIndex)).apply(this, arguments));
  }

  settings_createClass(SettingsIndex, [{
    key: 'componentDidMount',
    value: function componentDidMount() {
      var route = this.getAvailableRoute();
      if (route) {
        this.props.router.replace({ pathname: route });
      }
    }
  }, {
    key: 'getAvailableRoute',
    value: function getAvailableRoute() {
      var items = this.props.store.getNavItems();
      if (items && items[0]) {
        return items[0].to;
      }

      return null;
    }
  }, {
    key: 'render',
    value: function render() {
      return react_default.a.createElement(msgPage_AccessDenied, null);
    }
  }]);

  return SettingsIndex;
}(react_default.a.Component);

settings_SettingsIndex.propTypes = {
  router: react_default.a.PropTypes.object.isRequired,
  store: react_default.a.PropTypes.object.isRequired,
  location: react_default.a.PropTypes.object.isRequired
};


function settings_mapStateToProps() {
  return {
    store: settings_getters.store
  };
}

/* harmony default export */ var components_settings = (connect(settings_mapStateToProps)(settings_SettingsIndex));
// CONCATENATED MODULE: ./src/app/features/settings/featureSettings.js
var featureSettings_get = function get(object, property, receiver) { if (object === null) object = Function.prototype; var desc = Object.getOwnPropertyDescriptor(object, property); if (desc === undefined) { var parent = Object.getPrototypeOf(object); if (parent === null) { return undefined; } else { return get(parent, property, receiver); } } else if ("value" in desc) { return desc.value; } else { var getter = desc.get; if (getter === undefined) { return undefined; } return getter.call(receiver); } };

var featureSettings_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function featureSettings_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function featureSettings_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function featureSettings_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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











var settingsNavItem = {
  icon: 'fa fa-wrench',
  to: config["a" /* default */].routes.settingsBase,
  title: 'Settings'

  /**
   * Describes nested features within Settings
   */
};var SettingsFeatureBase = function (_FeatureBase) {
  featureSettings_inherits(SettingsFeatureBase, _FeatureBase);

  function SettingsFeatureBase(props) {
    featureSettings_classCallCheck(this, SettingsFeatureBase);

    return featureSettings_possibleConstructorReturn(this, (SettingsFeatureBase.__proto__ || Object.getPrototypeOf(SettingsFeatureBase)).call(this, props));
  }

  featureSettings_createClass(SettingsFeatureBase, [{
    key: 'isEnabled',
    value: function isEnabled() {
      return true;
    }
  }]);

  return SettingsFeatureBase;
}(featureBase);

var featureSettings_SettingsFeature = function (_FeatureBase2) {
  featureSettings_inherits(SettingsFeature, _FeatureBase2);

  featureSettings_createClass(SettingsFeature, [{
    key: 'addChild',
    value: function addChild(feature) {
      if (!(feature instanceof SettingsFeatureBase)) {
        throw Error('feature must implement SettingsFeatureBase');
      }

      this.featureActivator.register(feature);
    }
  }]);

  function SettingsFeature(routes) {
    featureSettings_classCallCheck(this, SettingsFeature);

    var _this2 = featureSettings_possibleConstructorReturn(this, (SettingsFeature.__proto__ || Object.getPrototypeOf(SettingsFeature)).call(this, constants["d" /* TRYING_TO_INIT_SETTINGS */]));

    _this2.featureActivator = new app_featureActivator();
    _this2.childRoutes = [];

    var settingsRoutes = {
      path: config["a" /* default */].routes.settingsBase,
      component: featureSettings_get(SettingsFeature.prototype.__proto__ || Object.getPrototypeOf(SettingsFeature.prototype), 'withMe', _this2).call(_this2, settings_main),
      indexRoute: {
        // need index component to handle default redirect to available nested feature
        component: components_settings
      },
      childRoutes: _this2.childRoutes
    };

    routes.push(settingsRoutes);
    return _this2;
  }

  featureSettings_createClass(SettingsFeature, [{
    key: 'componentDidMount',
    value: function componentDidMount() {
      try {
        initSettings(this.featureActivator);
      } catch (err) {
        this.handleError(err);
      }
    }
  }, {
    key: 'onload',
    value: function onload() {
      var features = this.featureActivator.getFeatures();
      var some = features.some(function (f) {
        return f.isEnabled();
      });
      if (some) {
        addNavItem(settingsNavItem);
      }
    }
  }]);

  return SettingsFeature;
}(featureBase);

/* harmony default export */ var featureSettings = (featureSettings_SettingsFeature);
// CONCATENATED MODULE: ./src/app/features/index.js
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






// CONCATENATED MODULE: ./src/app/features/settings/flags.js
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



function isAccountEnabled() {
  return getUser().isSso() == false;
}
// CONCATENATED MODULE: ./src/app/components/alerts.jsx
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




var alerts_Danger = function Danger(props) {
  return react_default.a.createElement(
    'div',
    { className: classnames_default()("grv-alert grv-alert-danger", props.className) },
    props.children
  );
};

var alerts_Info = function Info(props) {
  return react_default.a.createElement(
    'div',
    { className: classnames_default()("grv-alert grv-alert-info", props.className) },
    props.children
  );
};

var alerts_Success = function Success(props) {
  return react_default.a.createElement(
    'div',
    { className: classnames_default()("grv-alert grv-alert-success", props.className) },
    ' ',
    react_default.a.createElement('i', { className: 'fa fa-check m-r-xs' }),
    ' ',
    props.children
  );
};
// CONCATENATED MODULE: ./src/app/flux/settingsAccount/actions.js
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







var settingsAccount_actions_logger = logger["a" /* default */].create("flux/settingsAccount/actions");

function actions_changePasswordWithU2f(oldPsw, newPsw) {
  var promise = services_auth.changePasswordWithU2f(oldPsw, newPsw);
  _handleChangePasswordPromise(promise);
}

function actions_changePassword(oldPass, newPass, token) {
  var promise = services_auth.changePassword(oldPass, newPass, token);
  _handleChangePasswordPromise(promise);
}

function resetPasswordChangeAttempt() {
  changePasswordStatus.clear();
}

function _handleChangePasswordPromise(promise) {
  changePasswordStatus.start();
  return promise.done(function () {
    changePasswordStatus.success();
  }).fail(function (err) {
    var msg = api["a" /* default */].getErrorText(err);
    settingsAccount_actions_logger.error("change password", err);
    changePasswordStatus.fail(msg);
    // logout a user in case of access denied error 
    // (most likely a user exceeded a number of allowed attempts)
    if (err.status == 403) {
      services_session.logout();
    }
  });
}
// CONCATENATED MODULE: ./src/app/components/settings/accountTab.jsx
var accountTab_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

var accountTab_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function accountTab_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function accountTab_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function accountTab_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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












var accountTab_Separator = function Separator() {
  return react_default.a.createElement('div', { className: 'grv-settings-header-line-solid m-t-sm m-b-sm' });
};

var accountTab_Label = function Label(_ref) {
  var text = _ref.text;
  return react_default.a.createElement(
    'label',
    { style: { width: "150px", fontWeight: "normal" }, className: ' m-t-xs' },
    ' ',
    text,
    ' '
  );
};

var accountTab_defaultState = {
  oldPass: '',
  newPass: '',
  newPassConfirmed: '',
  token: ''
};

var accountTab_AccountTab = function (_React$Component) {
  accountTab_inherits(AccountTab, _React$Component);

  function AccountTab() {
    var _ref2;

    var _temp, _this, _ret;

    accountTab_classCallCheck(this, AccountTab);

    for (var _len = arguments.length, args = Array(_len), _key = 0; _key < _len; _key++) {
      args[_key] = arguments[_key];
    }

    return _ret = (_temp = (_this = accountTab_possibleConstructorReturn(this, (_ref2 = AccountTab.__proto__ || Object.getPrototypeOf(AccountTab)).call.apply(_ref2, [this].concat(args))), _this), _this.hasBeenClicked = false, _this.state = accountTab_extends({}, accountTab_defaultState), _this.onClick = function (e) {
      e.preventDefault();
      if (_this.isValid()) {
        var _this$state = _this.state,
            oldPass = _this$state.oldPass,
            newPass = _this$state.newPass,
            token = _this$state.token;

        _this.hasBeenClicked = true;
        if (_this.props.auth2faType === Auth2faTypeEnum.UTF) {
          _this.props.onChangePassWithU2f(oldPass, newPass);
        } else {
          _this.props.onChangePass(oldPass, newPass, token);
        }
      }
    }, _this.onKeyPress = function (e) {
      if (e.key === 'Enter' && e.target.value) {
        _this.onClick(e);
      }
    }, _temp), accountTab_possibleConstructorReturn(_this, _ret);
  }

  accountTab_createClass(AccountTab, [{
    key: 'componentDidMount',
    value: function componentDidMount() {
      jquery_default()(this.refs.form).validate({
        rules: {
          newPass: {
            minlength: 6,
            required: true
          },
          newPassConfirmed: {
            required: true,
            equalTo: this.refs.newPass
          }
        },
        messages: {
          passwordConfirmed: {
            minlength: jquery_default.a.validator.format('Enter at least {0} characters'),
            equalTo: 'Enter the same password as above'
          }
        }
      });
    }
  }, {
    key: 'componentWillUnmount',
    value: function componentWillUnmount() {
      this.props.onDestory && this.props.onDestory();
    }
  }, {
    key: 'isValid',
    value: function isValid() {
      var $form = jquery_default()(this.refs.form);
      return $form.length === 0 || $form.valid();
    }
  }, {
    key: 'componentWillReceiveProps',
    value: function componentWillReceiveProps(nextProps) {
      var isSuccess = nextProps.attempt.isSuccess;

      if (isSuccess && this.hasBeenClicked) {
        // reset all input fields on success
        this.hasBeenClicked = false;
        this.setState(accountTab_defaultState);
      }
    }
  }, {
    key: 'isU2f',
    value: function isU2f() {
      return this.props.auth2faType === Auth2faTypeEnum.UTF;
    }
  }, {
    key: 'isOtp',
    value: function isOtp() {
      return this.props.auth2faType === Auth2faTypeEnum.OTP;
    }
  }, {
    key: 'render',
    value: function render() {
      var _this2 = this;

      var isOtpEnabled = this.isOtp();
      var _props$attempt = this.props.attempt,
          isFailed = _props$attempt.isFailed,
          isProcessing = _props$attempt.isProcessing,
          isSuccess = _props$attempt.isSuccess,
          message = _props$attempt.message;
      var _state = this.state,
          oldPass = _state.oldPass,
          newPass = _state.newPass,
          newPassConfirmed = _state.newPassConfirmed;

      var waitForU2fKeyResponse = isProcessing && this.isU2f();

      return react_default.a.createElement(
        'div',
        { title: 'Change Password', className: 'm-t-sm grv-settings-account' },
        react_default.a.createElement(
          'h3',
          { className: 'no-margins' },
          'Change Password'
        ),
        react_default.a.createElement(accountTab_Separator, null),
        react_default.a.createElement(
          'div',
          { className: 'm-b m-l-xl', style: { maxWidth: "500px" } },
          react_default.a.createElement(
            'form',
            { ref: 'form', onKeyPress: this.onKeyPress },
            react_default.a.createElement(
              'div',
              null,
              isFailed && react_default.a.createElement(
                alerts_Danger,
                { className: 'm-b-sm' },
                ' ',
                message,
                ' '
              ),
              isSuccess && react_default.a.createElement(
                alerts_Success,
                { className: 'm-b-sm' },
                ' Your password has been changed '
              ),
              waitForU2fKeyResponse && react_default.a.createElement(
                alerts_Info,
                { className: 'm-b-sm' },
                ' Insert your U2F key and press the button on the key '
              )
            ),
            react_default.a.createElement(
              layout.Flex,
              { dir: 'row', className: 'm-t' },
              react_default.a.createElement(accountTab_Label, { text: 'Current Password:' }),
              react_default.a.createElement(
                'div',
                { style: { flex: "1" } },
                react_default.a.createElement('input', {
                  autoFocus: true,
                  type: 'password',
                  value: oldPass,
                  onChange: function onChange(e) {
                    return _this2.setState({
                      oldPass: e.target.value
                    });
                  },
                  className: 'form-control required' })
              )
            ),
            isOtpEnabled && react_default.a.createElement(
              layout.Flex,
              { dir: 'row', className: 'm-t-sm' },
              react_default.a.createElement(accountTab_Label, { text: '2nd factor token:' }),
              react_default.a.createElement(
                'div',
                { style: { flex: "1" } },
                react_default.a.createElement('input', { autoComplete: 'off',
                  style: { width: "100px" },
                  value: this.state.token,
                  onChange: function onChange(e) {
                    return _this2.setState({
                      'token': e.target.value
                    });
                  },
                  className: 'form-control required', name: 'token'
                })
              )
            ),
            react_default.a.createElement(
              layout.Flex,
              { dir: 'row', className: 'm-t-lg' },
              react_default.a.createElement(accountTab_Label, { text: 'New Password:' }),
              react_default.a.createElement(
                'div',
                { style: { flex: "1" } },
                react_default.a.createElement('input', {
                  value: newPass,
                  onChange: function onChange(e) {
                    return _this2.setState({
                      newPass: e.target.value
                    });
                  },
                  ref: 'newPass',
                  type: 'password',
                  name: 'newPass',
                  className: 'form-control'
                })
              )
            ),
            react_default.a.createElement(
              layout.Flex,
              { dir: 'row', className: 'm-t-sm' },
              react_default.a.createElement(accountTab_Label, { text: 'Confirm Password:' }),
              react_default.a.createElement(
                'div',
                { style: { flex: "1" } },
                react_default.a.createElement('input', {
                  type: 'password',
                  value: newPassConfirmed,
                  onChange: function onChange(e) {
                    return _this2.setState({
                      newPassConfirmed: e.target.value
                    });
                  },
                  name: 'newPassConfirmed',
                  className: 'form-control'
                })
              )
            )
          )
        ),
        react_default.a.createElement(
          'button',
          { disabled: isProcessing, onClick: this.onClick, type: 'submit', className: 'btn btn-sm btn-primary block' },
          'Update'
        )
      );
    }
  }]);

  return AccountTab;
}(react_default.a.Component);

accountTab_AccountTab.propTypes = {
  attempt: react_default.a.PropTypes.object.isRequired,
  onChangePass: react_default.a.PropTypes.func.isRequired,
  onChangePassWithU2f: react_default.a.PropTypes.func.isRequired
};


function accountTab_mapFluxToProps() {
  return {
    attempt: user_getters.pswChangeAttempt
  };
}

function accountTab_mapStateToProps() {
  return {
    auth2faType: config["a" /* default */].getAuth2faType(),
    onChangePass: actions_changePassword,
    onChangePassWithU2f: actions_changePasswordWithU2f,
    onDestory: resetPasswordChangeAttempt
  };
}

/* harmony default export */ var accountTab = (connect(accountTab_mapFluxToProps, accountTab_mapStateToProps)(accountTab_AccountTab));
// CONCATENATED MODULE: ./src/app/features/settings/featureSettingsAccount.js
var featureSettingsAccount_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function featureSettingsAccount_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function featureSettingsAccount_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function featureSettingsAccount_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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








var featureUrl = config["a" /* default */].routes.settingsAccount;

var featureSettingsAccount_AccountFeature = function (_SettingsFeatureBase) {
  featureSettingsAccount_inherits(AccountFeature, _SettingsFeatureBase);

  function AccountFeature(routes) {
    featureSettingsAccount_classCallCheck(this, AccountFeature);

    var _this = featureSettingsAccount_possibleConstructorReturn(this, (AccountFeature.__proto__ || Object.getPrototypeOf(AccountFeature)).call(this));

    var route = {
      path: featureUrl,
      component: _this.withMe(documentTitle_withDocTitle('Account', accountTab))
    };

    routes.push(route);
    return _this;
  }

  featureSettingsAccount_createClass(AccountFeature, [{
    key: 'isEnabled',
    value: function isEnabled() {
      return isAccountEnabled();
    }
  }, {
    key: 'init',
    value: function init() {
      if (!this.wasInitialized()) {
        this.stopProcessing();
      }
    }
  }, {
    key: 'onload',
    value: function onload() {
      if (!this.isEnabled()) {
        return;
      }

      var navItem = {
        to: featureUrl,
        title: "Account"
      };

      actions_addNavItem(navItem);
      this.init();
    }
  }]);

  return AccountFeature;
}(SettingsFeatureBase);

/* harmony default export */ var featureSettingsAccount = (featureSettingsAccount_AccountFeature);
// CONCATENATED MODULE: ./src/app/features/settings/index.js
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




/**
 * Adds nested feature to given Settings feature
 * @param {*instance of Settings feature} settings 
 */
var append = function append(settings, fctor) {
  var f = new fctor(settings.childRoutes);
  settings.addChild(f);
};

var settings_createSettings = function createSettings(routes) {
  var settings = new featureSettings(routes);
  append(settings, featureSettingsAccount);
  return settings;
};
// CONCATENATED MODULE: ./src/app/services/browser.js
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

function detectPlatform() {
  var userAgent = window.navigator.userAgent;
  return {
    isWin: userAgent.indexOf('Windows') >= 0,
    isMac: userAgent.indexOf('Macintosh') >= 0,
    isLinux: userAgent.indexOf('Linux') >= 0
  };
}

var platform = detectPlatform();
// CONCATENATED MODULE: ./src/app/flux/app/appStore.js
var appStore_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function appStore_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function appStore_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function appStore_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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





var AppRec = function (_Record) {
  appStore_inherits(AppRec, _Record);

  function AppRec(props) {
    appStore_classCallCheck(this, AppRec);

    return appStore_possibleConstructorReturn(this, (AppRec.__proto__ || Object.getPrototypeOf(AppRec)).call(this, props));
  }

  appStore_createClass(AppRec, [{
    key: 'setSiteId',
    value: function setSiteId(siteId) {
      return this.set('siteId', siteId);
    }
  }, {
    key: 'getClusterName',
    value: function getClusterName() {
      return this.get('siteId');
    }
  }, {
    key: 'getNavItems',
    value: function getNavItems() {
      return this.navItems.toJS();
    }
  }, {
    key: 'addNavItem',
    value: function addNavItem(navItem) {
      return this.set('navItems', this.navItems.push(navItem));
    }
  }]);

  return AppRec;
}(Object(immutable["Record"])({
  siteId: null,
  navItems: new immutable["List"]()
}));

function getStore() {
  return app_reactor["a" /* default */].evaluate(['tlpt']);
}

/* harmony default export */ var appStore = (Object(nuclear["Store"])({
  getInitialState: function getInitialState() {
    return new AppRec();
  },
  initialize: function initialize() {
    this.on(SET_SITE_ID, function (state, siteId) {
      return state.setSiteId(siteId);
    });
    this.on(ADD_NAV_ITEM, function (state, navItem) {
      return state.addNavItem(navItem);
    });
  }
}));
// CONCATENATED MODULE: ./src/app/components/navLeftBar.jsx
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








function NavLeftBar(props) {
  var items = getStore().getNavItems();
  var name = getUser().getName();
  var $items = items.map(function (i, index) {
    var className = props.router.isActive(i.to) ? 'active' : '';
    return react_default.a.createElement(
      'li',
      { key: index, className: className, title: i.title },
      react_default.a.createElement(
        lib["IndexLink"],
        { to: i.to },
        react_default.a.createElement('i', { className: i.icon })
      )
    );
  });

  $items.push(react_default.a.createElement(
    'li',
    { key: $items.length, title: 'help' },
    react_default.a.createElement(
      'a',
      { href: config["a" /* default */].helpUrl, target: '_blank' },
      react_default.a.createElement('i', { className: 'fa fa-question' })
    )
  ));

  $items.push(react_default.a.createElement(
    'li',
    { key: $items.length, title: 'logout' },
    react_default.a.createElement(
      'a',
      { href: '#', onClick: user_actions.logout },
      react_default.a.createElement('i', { className: 'fa fa-sign-out', style: { marginRight: 0 } })
    )
  ));

  return react_default.a.createElement(
    'nav',
    { className: 'grv-nav navbar-default', role: 'navigation' },
    react_default.a.createElement(
      'ul',
      { className: 'nav text-center', id: 'side-menu' },
      react_default.a.createElement(
        'li',
        null,
        react_default.a.createElement(icons_UserIcon, { name: name })
      ),
      $items
    )
  );
}

NavLeftBar.propTypes = {
  router: react_default.a.PropTypes.object.isRequired
};
// CONCATENATED MODULE: ./src/app/components/app.jsx
var app_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function app_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function app_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function app_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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












var app_App = function (_Component) {
  app_inherits(App, _Component);

  function App() {
    var _ref;

    var _temp, _this, _ret;

    app_classCallCheck(this, App);

    for (var _len = arguments.length, args = Array(_len), _key = 0; _key < _len; _key++) {
      args[_key] = arguments[_key];
    }

    return _ret = (_temp = (_this = app_possibleConstructorReturn(this, (_ref = App.__proto__ || Object.getPrototypeOf(App)).call.apply(_ref, [this].concat(args))), _this), _this.onRefresh = function () {
      return Object(sessions_actions["a" /* fetchActiveSessions */])();
    }, _temp), app_possibleConstructorReturn(_this, _ret);
  }

  app_createClass(App, [{
    key: 'render',
    value: function render() {
      var _props = this.props,
          router = _props.router,
          initAttempt = _props.initAttempt;
      var isProcessing = initAttempt.isProcessing,
          isSuccess = initAttempt.isSuccess,
          isFailed = initAttempt.isFailed,
          message = initAttempt.message;


      if (isProcessing) {
        return react_default.a.createElement(indicator, { type: 'bounce' });
      }

      if (isFailed) {
        return react_default.a.createElement(msgPage_Failed, { message: message });
      }

      var className = classnames_default()('grv-tlpt grv-flex grv-flex-row', {
        '--isLinux': platform.isLinux,
        '--isWin': platform.isWin,
        '--isMac': platform.isMac
      });

      if (isSuccess) {
        return react_default.a.createElement(
          'div',
          { className: className },
          react_default.a.createElement(dataProvider, { onFetch: this.onRefresh, time: 3000 }),
          this.props.CurrentSessionHost,
          react_default.a.createElement(NavLeftBar, { router: router }),
          this.props.children
        );
      }

      return null;
    }
  }]);

  return App;
}(react["Component"]);

function app_mapStateToProps() {
  return {
    initAttempt: app_getters["a" /* default */].initAttempt
  };
}

/* harmony default export */ var app = (Object(build["connect"])(app_mapStateToProps)(app_App));
// EXTERNAL MODULE: ./src/styles/grv.scss
var grv = __webpack_require__("7WIf");

// CONCATENATED MODULE: ./src/app/flux/terminal/store.js
var terminal_store_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function terminal_store_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function terminal_store_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function terminal_store_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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







var TermStatusRec = new immutable["Record"]({
  isReady: false,
  isLoading: false,
  isNotFound: false,
  isError: false,
  errorText: undefined
});

var store_TermRec = function (_Record) {
  terminal_store_inherits(TermRec, _Record);

  function TermRec() {
    terminal_store_classCallCheck(this, TermRec);

    return terminal_store_possibleConstructorReturn(this, (TermRec.__proto__ || Object.getPrototypeOf(TermRec)).apply(this, arguments));
  }

  terminal_store_createClass(TermRec, [{
    key: 'getClusterName',
    value: function getClusterName() {
      return this.siteId;
    }
  }, {
    key: 'getTtyParams',
    value: function getTtyParams() {
      var _localStorage$getBear = localStorage["b" /* default */].getBearerToken(),
          accessToken = _localStorage$getBear.accessToken;

      var server_id = this.serverId;
      return {
        login: this.login,
        sid: this.sid,
        token: accessToken,
        ttyUrl: config["a" /* default */].api.ttyWsAddr,
        cluster: this.siteId,
        getTarget: function getTarget() {
          return { server_id: server_id };
        }
      };
    }
  }, {
    key: 'getServerLabel',
    value: function getServerLabel() {
      if (this.hostname && this.login) {
        return this.login + '@' + this.hostname;
      }

      if (this.serverId && this.login) {
        return this.login + '@' + this.serverId;
      }

      return '';
    }
  }]);

  return TermRec;
}(Object(immutable["Record"])({
  status: TermStatusRec(),
  hostname: null,
  login: null,
  siteId: null,
  serverId: null,
  sid: null
}));

/* harmony default export */ var terminal_store = (Object(nuclear["Store"])({
  getInitialState: function getInitialState() {
    return new store_TermRec();
  },
  initialize: function initialize() {
    this.on(TLPT_TERMINAL_INIT, store_init);
    this.on(TLPT_TERMINAL_CLOSE, store_close);
    this.on(TLPT_TERMINAL_SET_STATUS, changeStatus);
  }
}));

function store_close() {
  return new store_TermRec();
}

function store_init(state, json) {
  return new store_TermRec(json);
}

function changeStatus(state, status) {
  return state.setIn(['status'], new TermStatusRec(status));
}
// CONCATENATED MODULE: ./src/app/flux/settings/store.js
var settings_store_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function settings_store_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function settings_store_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function settings_store_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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





var SettingsRec = function (_Record) {
  settings_store_inherits(SettingsRec, _Record);

  function SettingsRec(params) {
    settings_store_classCallCheck(this, SettingsRec);

    return settings_store_possibleConstructorReturn(this, (SettingsRec.__proto__ || Object.getPrototypeOf(SettingsRec)).call(this, params));
  }

  settings_store_createClass(SettingsRec, [{
    key: 'isReady',
    value: function isReady() {
      return this.isInitialized;
    }
  }, {
    key: 'getNavItems',
    value: function getNavItems() {
      return this.navItems.toJS();
    }
  }, {
    key: 'addNavItem',
    value: function addNavItem(navItem) {
      return this.set('navItems', this.navItems.push(navItem));
    }
  }]);

  return SettingsRec;
}(Object(immutable["Record"])({
  isInitialized: false,
  navItems: new immutable["List"]()
}));

/* harmony default export */ var settings_store = (Object(nuclear["Store"])({
  getInitialState: function getInitialState() {
    return new SettingsRec();
  },
  initialize: function initialize() {
    this.on(INIT, function (state) {
      return state.set('isInitialized', true);
    });
    this.on(actionTypes_ADD_NAV_ITEM, function (state, navItem) {
      return state.addNavItem(navItem);
    });
  }
}));
// EXTERNAL MODULE: ./src/app/flux/status/statusStore.js
var statusStore = __webpack_require__("PVWJ");

// CONCATENATED MODULE: ./src/app/flux/user/userStore.js
var userStore_createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

function userStore_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function userStore_possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function userStore_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

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






var userStore_UserRec = function (_Record) {
  userStore_inherits(UserRec, _Record);

  function UserRec(params) {
    userStore_classCallCheck(this, UserRec);

    return userStore_possibleConstructorReturn(this, (UserRec.__proto__ || Object.getPrototypeOf(UserRec)).call(this, params));
  }

  userStore_createClass(UserRec, [{
    key: 'isSso',
    value: function isSso() {
      return this.get('authType') === AuthTypeEnum.SSO;
    }
  }, {
    key: 'getName',
    value: function getName() {
      return this.get('name');
    }
  }]);

  return UserRec;
}(Object(immutable["Record"])({
  name: '',
  authType: ''
}));

/* harmony default export */ var userStore = (Object(nuclear["Store"])({
  getInitialState: function getInitialState() {
    return Object(nuclear["toImmutable"])(null);
  },
  initialize: function initialize() {
    this.on(RECEIVE_USER, receiveUser);
  }
}));

function receiveUser(state, json) {
  return new userStore_UserRec(json);
}
// CONCATENATED MODULE: ./src/app/flux/user/userInviteStore.js
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





var userInviteStore_Invite = new immutable["Record"]({
  invite_token: '',
  user: '',
  qr: ''
});

/* harmony default export */ var userInviteStore = (Object(nuclear["Store"])({
  getInitialState: function getInitialState() {
    return Object(nuclear["toImmutable"])(null);
  },
  initialize: function initialize() {
    this.on(RECEIVE_INVITE, receiveInvite);
  }
}));

function receiveInvite(state, json) {
  return new userInviteStore_Invite(json);
}
// CONCATENATED MODULE: ./src/app/flux/sites/siteStore.js
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





var Site = Object(immutable["Record"])({
  name: null,
  status: false
});

/* harmony default export */ var siteStore = (Object(nuclear["Store"])({
  getInitialState: function getInitialState() {
    return new immutable["List"]();
  },
  initialize: function initialize() {
    this.on(RECEIVE_CLUSTERS, receiveSites);
  }
}));

function receiveSites(state, json) {
  return Object(nuclear["toImmutable"])(json).map(function (o) {
    return new Site(o);
  });
}
// EXTERNAL MODULE: ./src/app/flux/sessions/actionTypes.js
var sessions_actionTypes = __webpack_require__("zMbK");

// CONCATENATED MODULE: ./src/app/flux/sessions/eventStore.js
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




/* harmony default export */ var eventStore = (Object(nuclear["Store"])({
  getInitialState: function getInitialState() {
    return Object(nuclear["toImmutable"])({});
  },
  initialize: function initialize() {
    this.on(sessions_actionTypes["b" /* RECEIVE_SITE_EVENTS */], receive);
  }
}));

function receive(state, _ref) {
  var json = _ref.json;

  var jsonEvents = json || [];
  return state.withMutations(function (state) {
    jsonEvents.forEach(function (item) {
      var sid = item.sid,
          event = item.event;


      if (!state.has(sid)) {
        state.set(sid, Object(nuclear["toImmutable"])({}));
      }

      state.setIn([sid, event], Object(nuclear["toImmutable"])(item));
    });
  });
}
// CONCATENATED MODULE: ./src/app/flux/sessions/archivedSessionStore.js
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






var StoredSessionRec = Object(immutable["Record"])({
  id: undefined,
  user: undefined,
  created: undefined,
  nodeIp: undefined,
  last_active: undefined,
  server_id: undefined,
  siteId: undefined,
  parties: Object(immutable["List"])()
});

/* harmony default export */ var archivedSessionStore = (Object(nuclear["Store"])({
  getInitialState: function getInitialState() {
    return Object(nuclear["toImmutable"])({});
  },
  initialize: function initialize() {
    this.on(sessions_actionTypes["b" /* RECEIVE_SITE_EVENTS */], archivedSessionStore_receive);
  }
}));

// uses events to build stored session objects
function archivedSessionStore_receive(state, _ref) {
  var siteId = _ref.siteId,
      json = _ref.json;

  var jsonEvents = json || [];
  var tmp = {};
  return state.withMutations(function (state) {
    jsonEvents.forEach(function (item) {
      if (item.event !== EventTypeEnum.START && item.event !== EventTypeEnum.END) {
        return;
      }

      var sid = item.sid,
          user = item.user,
          time = item.time,
          event = item.event,
          server_id = item.server_id;


      tmp[sid] = tmp[sid] || {};
      tmp[sid].id = sid;
      tmp[sid].user = user;
      tmp[sid].siteId = siteId;

      if (event === EventTypeEnum.START) {
        tmp[sid].created = time;
        tmp[sid].server_id = server_id;
        tmp[sid].nodeIp = item['addr.local'];
        tmp[sid].parties = [{
          user: user,
          serverIp: item['addr.remote']
        }];
      }

      // update the store only with new items
      if (event === EventTypeEnum.END && !state.has(sid)) {
        tmp[sid].last_active = time;
        state.set(sid, new StoredSessionRec(Object(nuclear["toImmutable"])(tmp[sid])));
      }
    });
  });
}
// CONCATENATED MODULE: ./src/app/flux/sessions/activeSessionStore.js
var activeSessionStore_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

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






var ActiveSessionRec = Object(immutable["Record"])({
  id: undefined,
  namespace: undefined,
  login: undefined,
  active: undefined,
  created: undefined,
  last_active: undefined,
  server_id: undefined,
  siteId: undefined,
  parties: Object(immutable["List"])()
});

var PartyRecord = Object(immutable["Record"])({
  user: undefined,
  serverIp: undefined,
  serverId: undefined
});

var activeSessionStore_defaultState = function defaultState() {
  return Object(nuclear["toImmutable"])({});
};

/* harmony default export */ var activeSessionStore = (Object(nuclear["Store"])({
  getInitialState: function getInitialState() {
    return activeSessionStore_defaultState();
  },
  initialize: function initialize() {
    this.on(sessions_actionTypes["a" /* RECEIVE_ACTIVE_SESSIONS */], activeSessionStore_receive);
  }
}));

function activeSessionStore_receive(state, _ref) {
  var siteId = _ref.siteId,
      json = _ref.json;

  var jsonArray = json || [];
  var newState = activeSessionStore_defaultState().withMutations(function (newState) {
    return jsonArray.filter(function (item) {
      return item.active === true;
    }).forEach(function (item) {
      var rec = createSessionRec(siteId, item);
      newState.set(rec.id, rec);
    });
  });

  return newState.equals(state) ? state : newState;
}

function createSessionRec(siteId, json) {
  var parties = activeSessionStore_createParties(json.parties);
  var rec = new ActiveSessionRec(Object(nuclear["toImmutable"])(activeSessionStore_extends({}, json, {
    siteId: siteId,
    parties: parties
  })));

  return rec;
}

function activeSessionStore_createParties(jsonArray) {
  jsonArray = jsonArray || [];
  var list = new immutable["List"]();
  return list.withMutations(function (list) {
    jsonArray.forEach(function (item) {
      var party = new PartyRecord({
        user: item.user,
        serverIp: item.remote_addr,
        serverId: item.server_id
      });

      list.push(party);
    });
  });
}
// EXTERNAL MODULE: ./src/app/flux/storedSessionsFilter/actionTypes.js
var storedSessionsFilter_actionTypes = __webpack_require__("OcN7");

// CONCATENATED MODULE: ./src/app/flux/storedSessionsFilter/storedSessionFilterStore.js
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





/* harmony default export */ var storedSessionFilterStore = (Object(nuclear["Store"])({
  getInitialState: function getInitialState() {
    var end = moment_default()(new Date()).endOf('day').toDate();
    var start = moment_default()(end).subtract(3, 'day').startOf('day').toDate();
    var state = {
      start: start,
      end: end
    };

    return Object(nuclear["toImmutable"])(state);
  },
  initialize: function initialize() {
    this.on(storedSessionsFilter_actionTypes["a" /* TLPT_STORED_SESSINS_FILTER_SET_RANGE */], setRange);
  }
}));

function setRange(state, newState) {
  return state.merge(newState);
}
// CONCATENATED MODULE: ./src/app/flux/index.js
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



















store_register(app_reactor["a" /* default */]);
misc_store_register(app_reactor["a" /* default */]);
fileTransfer_register(app_reactor["a" /* default */]);

app_reactor["a" /* default */].registerStores({
  'tlpt_settings': settings_store,
  'tlpt': appStore,
  'tlpt_terminal': terminal_store,
  'tlpt_nodes': nodeStore,
  'tlpt_user': userStore,
  'tlpt_user_invite': userInviteStore,
  'tlpt_user_acl': userAcl_store,
  'tlpt_sites': siteStore,
  'tlpt_status': statusStore["b" /* default */],
  'tlpt_sessions_events': eventStore,
  'tlpt_sessions_archived': archivedSessionStore,
  'tlpt_sessions_active': activeSessionStore,
  'tlpt_sessions_filter': storedSessionFilterStore
});
// EXTERNAL MODULE: ./node_modules/bootstrap-sass/assets/javascripts/bootstrap.js
var bootstrap = __webpack_require__("VSY+");

// EXTERNAL MODULE: ./src/assets/js/datepicker.js
var datepicker = __webpack_require__("hQgP");

// CONCATENATED MODULE: ./src/app/vendor.js
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






// CONCATENATED MODULE: ./src/app/index.jsx
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


















config["a" /* default */].init(window.GRV_CONFIG);
services_history.init();

var featureRoutes = [];
var src_app_featureActivator = new app_featureActivator();

src_app_featureActivator.register(new featureSsh(featureRoutes));
src_app_featureActivator.register(new featureAudit(featureRoutes));
src_app_featureActivator.register(settings_createSettings(featureRoutes));

var app_onEnterApp = function onEnterApp(nextState) {
  var siteId = nextState.params.siteId;

  initApp(siteId, src_app_featureActivator);
};

var app_routes = [{
  path: config["a" /* default */].routes.app,
  onEnter: user_actions.ensureUser,
  component: app,
  childRoutes: [{
    onEnter: app_onEnterApp,
    childRoutes: featureRoutes
  }]
}];

var app_Root = function Root() {
  return react_default.a.createElement(
    build["Provider"],
    { reactor: app_reactor["a" /* default */] },
    react_default.a.createElement(lib["Router"], { history: services_history.original(), routes: addRoutes(app_routes) })
  );
};

/* harmony default export */ var src_app = (app_Root);
// CONCATENATED MODULE: ./src/boot.js
/*
Copyright 2018 Gravitational, Inc.

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






var boot_render = function render(Component) {
  react_dom_default.a.render(react_default.a.createElement(
    react_hot_loader["AppContainer"],
    null,
    react_default.a.createElement(Component, null)
  ), document.getElementById('app'));
};

boot_render(src_app);

if (false) {}

/***/ }),

/***/ "tGXY":
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "c", function() { return START; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "d", function() { return SUCCESS; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "b", function() { return FAIL; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "a", function() { return CLEAR; });
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

var START = 'TLPT_STATUS_START';
var SUCCESS = 'TLPT_STATUS_SUCCESS';
var FAIL = 'TLPT_STATUS_FAIL';
var CLEAR = 'TLPT_STATUS_CLEAR';

/***/ }),

/***/ "xSHT":
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var nuclear_js__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__("L7e8");
/* harmony import */ var nuclear_js__WEBPACK_IMPORTED_MODULE_0___default = /*#__PURE__*/__webpack_require__.n(nuclear_js__WEBPACK_IMPORTED_MODULE_0__);
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



var CSS = 'color: blue';

// reactor options
var options = {
  debug: "production" === 'development'
};

var logger = {
  dispatchStart: function dispatchStart(reactorState, actionType, payload) {
    window.console.log('%creactor.dispatch("' + actionType + '", ', CSS, payload, ')');
  },


  dispatchError: function dispatchError(reactorState, error) {
    window.console.debug('Dispatch error: ' + error);
  },

  dispatchEnd: function dispatchEnd(reactorState, state, dirtyStores) {
    var stateChanges = state.filter(function (val, key) {
      return dirtyStores.contains(key);
    });
    window.console.log('%cupdated store -> ', CSS, stateChanges.toJS());
  }
};

if (options.debug) {
  options.logger = logger;
}

var reactor = new nuclear_js__WEBPACK_IMPORTED_MODULE_0__["Reactor"](options);
window.reactor = reactor;

/* harmony default export */ __webpack_exports__["a"] = (reactor);

/***/ }),

/***/ "zMbK":
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "a", function() { return RECEIVE_ACTIVE_SESSIONS; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "b", function() { return RECEIVE_SITE_EVENTS; });
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

var RECEIVE_ACTIVE_SESSIONS = 'TLPT_SESSIONS_RECEIVE_ACTIVE';
var RECEIVE_SITE_EVENTS = 'TLPT_SESSIONS_RECEIVE_EVENTS';

/***/ })

/******/ });