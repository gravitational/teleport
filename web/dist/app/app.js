webpackJsonp([1],{

/***/ 0:
/***/ function(module, exports, __webpack_require__) {

	module.exports = __webpack_require__(154);


/***/ },

/***/ 32:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	exports.__esModule = true;
	
	var _nuclearJs = __webpack_require__(82);
	
	var reactor = new _nuclearJs.Reactor({
	  debug: true
	});
	
	exports['default'] = reactor;
	module.exports = exports['default'];

/***/ },

/***/ 45:
/***/ function(module, exports) {

	module.exports = jQuery;

/***/ },

/***/ 46:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var createHistory = __webpack_require__(136);
	
	var AUTH_KEY_DATA = 'authData';
	
	var _history = null;
	
	var session = {
	
	  init: function init() {
	    _history = createHistory();
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

/***/ 119:
/***/ function(module, exports, __webpack_require__) {

	'use strict';
	
	var api = __webpack_require__(120);
	var session = __webpack_require__(46);
	var $ = __webpack_require__(45);
	
	var refreshRate = 60000 * 1; // 1 min
	
	var refreshTokenTimerId = null;
	
	var auth = {
	
	  login: function login(email, password) {
	    auth._stopTokenRefresher();
	    return auth._login(email, password).done(auth._startTokenRefresher);
	  },
	
	  ensureUser: function ensureUser() {
	    if (session.getUserData().user) {
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
	
	  _login: function _login(email, password) {
	    return api.login(email, password).then(function (data) {
	      session.setUserData(data);
	      return data;
	    });
	  }
	};
	
	module.exports = auth;

/***/ },

/***/ 120:
/***/ function(module, exports, __webpack_require__) {

	"use strict";
	
	var $ = __webpack_require__(45);
	var session = __webpack_require__(46);
	
	var BASE_URL = window.location.origin;
	
	var api = {
	  login: function login(username, password) {
	    var options = {
	      url: BASE_URL + "/portal/v1/sessions",
	      type: "POST",
	      dataType: "json",
	      beforeSend: function beforeSend(xhr) {
	        if (!!username && !!password) {
	          xhr.setRequestHeader("Authorization", "Basic " + btoa(username + ":" + password));
	        } else {
	          var _session$getUserData = session.getUserData();
	
	          var token = _session$getUserData.token;
	
	          xhr.setRequestHeader('Authorization', 'Bearer ' + token);
	        }
	      }
	    };
	
	    return $.ajax(options).then(function (json) {
	      return {
	        user: {
	          name: json.user.name,
	          accountId: json.user.account_id,
	          siteId: json.user.site_id
	        },
	        token: json.token
	      };
	    });
	  }
	};
	
	module.exports = api;

/***/ },

/***/ 136:
/***/ function(module, exports, __webpack_require__) {

	/* WEBPACK VAR INJECTION */(function(process) {'use strict';
	
	exports.__esModule = true;
	
	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };
	
	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }
	
	var _invariant = __webpack_require__(9);
	
	var _invariant2 = _interopRequireDefault(_invariant);
	
	var _Actions = __webpack_require__(25);
	
	var _ExecutionEnvironment = __webpack_require__(34);
	
	var _DOMUtils = __webpack_require__(48);
	
	var _DOMStateStorage = __webpack_require__(78);
	
	var _createDOMHistory = __webpack_require__(79);
	
	var _createDOMHistory2 = _interopRequireDefault(_createDOMHistory);
	
	/**
	 * Creates and returns a history object that uses HTML5's history API
	 * (pushState, replaceState, and the popstate event) to manage history.
	 * This is the recommended method of managing history in browsers because
	 * it provides the cleanest URLs.
	 *
	 * Note: In browsers that do not support the HTML5 history API full
	 * page reloads will be used to preserve URLs.
	 */
	function createBrowserHistory() {
	  var options = arguments.length <= 0 || arguments[0] === undefined ? {} : arguments[0];
	
	  !_ExecutionEnvironment.canUseDOM ? process.env.NODE_ENV !== 'production' ? _invariant2['default'](false, 'Browser history needs a DOM') : _invariant2['default'](false) : undefined;
	
	  var forceRefresh = options.forceRefresh;
	
	  var isSupported = _DOMUtils.supportsHistory();
	  var useRefresh = !isSupported || forceRefresh;
	
	  function getCurrentLocation(historyState) {
	    historyState = historyState || window.history.state || {};
	
	    var path = _DOMUtils.getWindowPath();
	    var _historyState = historyState;
	    var key = _historyState.key;
	
	    var state = undefined;
	    if (key) {
	      state = _DOMStateStorage.readState(key);
	    } else {
	      state = null;
	      key = history.createKey();
	
	      if (isSupported) window.history.replaceState(_extends({}, historyState, { key: key }), null, path);
	    }
	
	    return history.createLocation(path, state, undefined, key);
	  }
	
	  function startPopStateListener(_ref) {
	    var transitionTo = _ref.transitionTo;
	
	    function popStateListener(event) {
	      if (event.state === undefined) return; // Ignore extraneous popstate events in WebKit.
	
	      transitionTo(getCurrentLocation(event.state));
	    }
	
	    _DOMUtils.addEventListener(window, 'popstate', popStateListener);
	
	    return function () {
	      _DOMUtils.removeEventListener(window, 'popstate', popStateListener);
	    };
	  }
	
	  function finishTransition(location) {
	    var basename = location.basename;
	    var pathname = location.pathname;
	    var search = location.search;
	    var hash = location.hash;
	    var state = location.state;
	    var action = location.action;
	    var key = location.key;
	
	    if (action === _Actions.POP) return; // Nothing to do.
	
	    _DOMStateStorage.saveState(key, state);
	
	    var path = (basename || '') + pathname + search + hash;
	    var historyState = {
	      key: key
	    };
	
	    if (action === _Actions.PUSH) {
	      if (useRefresh) {
	        window.location.href = path;
	        return false; // Prevent location update.
	      } else {
	          window.history.pushState(historyState, null, path);
	        }
	    } else {
	      // REPLACE
	      if (useRefresh) {
	        window.location.replace(path);
	        return false; // Prevent location update.
	      } else {
	          window.history.replaceState(historyState, null, path);
	        }
	    }
	  }
	
	  var history = _createDOMHistory2['default'](_extends({}, options, {
	    getCurrentLocation: getCurrentLocation,
	    finishTransition: finishTransition,
	    saveState: _DOMStateStorage.saveState
	  }));
	
	  var listenerCount = 0,
	      stopPopStateListener = undefined;
	
	  function listenBefore(listener) {
	    if (++listenerCount === 1) stopPopStateListener = startPopStateListener(history);
	
	    var unlisten = history.listenBefore(listener);
	
	    return function () {
	      unlisten();
	
	      if (--listenerCount === 0) stopPopStateListener();
	    };
	  }
	
	  function listen(listener) {
	    if (++listenerCount === 1) stopPopStateListener = startPopStateListener(history);
	
	    var unlisten = history.listen(listener);
	
	    return function () {
	      unlisten();
	
	      if (--listenerCount === 0) stopPopStateListener();
	    };
	  }
	
	  // deprecated
	  function registerTransitionHook(hook) {
	    if (++listenerCount === 1) stopPopStateListener = startPopStateListener(history);
	
	    history.registerTransitionHook(hook);
	  }
	
	  // deprecated
	  function unregisterTransitionHook(hook) {
	    history.unregisterTransitionHook(hook);
	
	    if (--listenerCount === 0) stopPopStateListener();
	  }
	
	  return _extends({}, history, {
	    listenBefore: listenBefore,
	    listen: listen,
	    registerTransitionHook: registerTransitionHook,
	    unregisterTransitionHook: unregisterTransitionHook
	  });
	}
	
	exports['default'] = createBrowserHistory;
	module.exports = exports['default'];
	/* WEBPACK VAR INJECTION */}.call(exports, __webpack_require__(1)))

/***/ },

/***/ 148:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	var NavLeftBar = __webpack_require__(150);
	
	var App = React.createClass({
	  displayName: 'App',
	
	  render: function render() {
	    return React.createElement(
	      'div',
	      { className: 'grv' },
	      React.createElement(NavLeftBar, null),
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

/***/ 149:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	var $ = __webpack_require__(45);
	var reactor = __webpack_require__(32);
	
	var Login = React.createClass({
	  displayName: 'Login',
	
	  mixins: [reactor.ReactMixin],
	
	  getDataBindings: function getDataBindings() {
	    return {
	      //    userRequest: getters.userRequest
	    };
	  },
	
	  handleSubmit: function handleSubmit(e) {
	    e.preventDefault();
	    if (this.isValid()) {
	      var loc = this.props.location;
	      var email = this.refs.email.value;
	      var pass = this.refs.pass.value;
	      var redirect = '/web';
	
	      if (loc.state && loc.state.redirectTo) {
	        redirect = loc.state.redirectTo;
	      }
	
	      //actions.login(email, pass, redirect);
	    }
	  },
	
	  isValid: function isValid() {
	    var $form = $(".loginscreen form");
	    return $form.length === 0 || $form.valid();
	  },
	
	  render: function render() {
	    var isProcessing = false; //this.state.userRequest.get('isLoading');
	    var isError = false; //this.state.userRequest.get('isError');
	
	    return React.createElement(
	      'div',
	      { className: 'middle-box text-center loginscreen' },
	      React.createElement(
	        'form',
	        null,
	        React.createElement(
	          'div',
	          null,
	          React.createElement(
	            'h1',
	            { className: 'logo-name' },
	            'G'
	          )
	        ),
	        React.createElement(
	          'h3',
	          null,
	          ' Welcome to Gravitational'
	        ),
	        React.createElement(
	          'p',
	          null,
	          ' Login in.'
	        ),
	        React.createElement(
	          'div',
	          { className: 'm-t', role: 'form', onSubmit: this.handleSubmit },
	          React.createElement(
	            'div',
	            { className: 'form-group' },
	            React.createElement('input', { type: 'email', ref: 'email', name: 'email', className: 'form-control required', placeholder: 'Username', required: true })
	          ),
	          React.createElement(
	            'div',
	            { className: 'form-group' },
	            React.createElement('input', { type: 'password', ref: 'pass', name: 'password', className: 'form-control required', placeholder: 'Password', required: true })
	          ),
	          React.createElement(
	            'button',
	            { type: 'submit', onClick: this.handleSubmit, className: 'btn btn-primary block full-width m-b', disabled: isProcessing },
	            'Login'
	          )
	        )
	      )
	    );
	  }
	});
	
	module.exports = Login;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "login.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 150:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	var Router = __webpack_require__(36);
	var IndexLink = Router.IndexLink;
	var History = __webpack_require__(36).History;
	
	var menuItems = [{ icon: "fa fa fa-sitemap", to: '/web/nodes', title: "Nodes" }, { icon: "fa fa-hdd-o", to: '/web/sessions', title: "Sessions" }];
	
	var NavLeftBar = React.createClass({
	  displayName: 'NavLeftBar',
	
	  mixins: [History],
	
	  render: function render() {
	    var self = this;
	    var items = menuItems.map(function (i, index) {
	      var className = self.history.isActive(i.to) ? "active" : "";
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
	      { className: '', role: 'navigation', style: { width: '60px', float: 'left' } },
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
	
	module.exports = NavLeftBar;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "navLeftBar.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 151:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	var $ = __webpack_require__(45);
	var reactor = __webpack_require__(32);
	
	var Invite = React.createClass({
	  displayName: 'Invite',
	
	  handleSubmit: function handleSubmit(e) {
	    e.preventDefault();
	    if (this.isValid()) {
	      var loc = this.props.location;
	      var email = this.refs.email.value;
	      var pass = this.refs.pass.value;
	      var redirect = '/web';
	
	      if (loc.state && loc.state.redirectTo) {
	        redirect = loc.state.redirectTo;
	      }
	
	      //actions.login(email, pass, redirect);
	    }
	  },
	
	  isValid: function isValid() {
	    var $form = $(".loginscreen form");
	    return $form.length === 0 || $form.valid();
	  },
	
	  render: function render() {
	    var isProcessing = false; //this.state.userRequest.get('isLoading');
	    var isError = false; //this.state.userRequest.get('isError');
	
	    return React.createElement(
	      'div',
	      { className: 'middle-box text-center loginscreen  animated fadeInDown' },
	      React.createElement(
	        'div',
	        null,
	        React.createElement(
	          'div',
	          null,
	          React.createElement(
	            'h1',
	            { className: 'logo-name' },
	            'G'
	          )
	        ),
	        React.createElement(
	          'h3',
	          null,
	          'Welcome to Gravity'
	        ),
	        React.createElement(
	          'p',
	          null,
	          'Create password.'
	        ),
	        React.createElement(
	          'div',
	          { align: 'left' },
	          React.createElement(
	            'font',
	            { color: 'white' },
	            '1) Create and enter a new password',
	            React.createElement('br', null),
	            '2) Install Google Authenticator on your smartphone',
	            React.createElement('br', null),
	            '3) Open Google Authenticator and create a new account using provided barcode',
	            React.createElement('br', null),
	            '4) Generate Authenticator token and enter it below',
	            React.createElement('br', null)
	          )
	        ),
	        React.createElement(
	          'form',
	          { className: 'm-t', role: 'form', action: '/web/finishnewuser', method: 'POST' },
	          React.createElement(
	            'div',
	            { className: 'form-group' },
	            React.createElement('input', { type: 'hidden', name: 'token', className: 'form-control' })
	          ),
	          React.createElement(
	            'div',
	            { className: 'form-group' },
	            React.createElement('input', { type: 'test', name: 'username', disabled: true, className: 'form-control', placeholder: 'Username', required: '' })
	          ),
	          React.createElement(
	            'div',
	            { className: 'form-group' },
	            React.createElement('input', { type: 'password', name: 'password', id: 'password', className: 'form-control', placeholder: 'Password', required: '', onchange: 'checkPasswords()' })
	          ),
	          React.createElement(
	            'div',
	            { className: 'form-group' },
	            React.createElement('input', { type: 'password', name: 'password_confirm', id: 'password_confirm', className: 'form-control', placeholder: 'Confirm password', required: '', onchange: 'checkPasswords()' })
	          ),
	          React.createElement(
	            'div',
	            { className: 'form-group' },
	            React.createElement('input', { type: 'test', name: 'hotp_token', id: 'hotp_token', className: 'form-control', placeholder: 'hotp token', required: '' })
	          ),
	          React.createElement(
	            'button',
	            { type: 'submit', className: 'btn btn-primary block full-width m-b' },
	            'Confirm'
	          )
	        )
	      )
	    );
	  }
	});
	
	module.exports = Invite;
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "newUser.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ },

/***/ 152:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	var reactor = __webpack_require__(32);
	var Nodes = React.createClass({
	  displayName: 'Nodes',
	
	  render: function render() {
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

/***/ 153:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	var reactor = __webpack_require__(32);
	var Nodes = React.createClass({
	  displayName: 'Nodes',
	
	  render: function render() {
	    return React.createElement(
	      'div',
	      null,
	      React.createElement(
	        'h1',
	        null,
	        ' Sessions '
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

/***/ 154:
/***/ function(module, exports, __webpack_require__) {

	/* REACT HOT LOADER */ if (false) { (function () { var ReactHotAPI = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-api/modules/index.js"), RootInstanceProvider = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/RootInstanceProvider.js"), ReactMount = require("react/lib/ReactMount"), React = require("react"); module.makeHot = module.hot.data ? module.hot.data.makeHot : ReactHotAPI(function () { return RootInstanceProvider.getRootInstances(ReactMount); }, React); })(); } try { (function () {
	
	'use strict';
	
	var React = __webpack_require__(5);
	
	var _require = __webpack_require__(36);
	
	var Router = _require.Router;
	var Route = _require.Route;
	var Redirect = _require.Redirect;
	var IndexRoute = _require.IndexRoute;
	
	var render = __webpack_require__(83).render;
	
	// route component modules
	var App = __webpack_require__(148);
	var Login = __webpack_require__(149);
	var Nodes = __webpack_require__(152);
	var Sessions = __webpack_require__(153);
	var NewUser = __webpack_require__(151);
	var auth = __webpack_require__(119);
	var session = __webpack_require__(46);
	
	// init session
	session.init();
	
	function requireAuth(nextState, replace, cb) {
	  auth.ensureUser().done(function () {
	    return cb();
	  }).fail(function () {
	    replace({ redirectTo: nextState.location.pathname }, '/web/login');
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
	  React.createElement(Route, { path: '/web/login', component: Login }),
	  React.createElement(Route, { path: '/web/logout', onEnter: handleLogout }),
	  React.createElement(Route, { path: '/web/newuser', component: NewUser }),
	  React.createElement(
	    Route,
	    { path: '/web', component: App },
	    React.createElement(Route, { path: 'nodes', component: Nodes }),
	    React.createElement(Route, { path: 'sessions', component: Sessions })
	  )
	), document.getElementById("app"));
	
	/* REACT HOT LOADER */ }).call(this); } finally { if (false) { (function () { var foundReactClasses = module.hot.data && module.hot.data.foundReactClasses || false; if (module.exports && module.makeHot) { var makeExportsHot = require("/home/akontsevoy/go/src/github.com/gravitational/teleport/web/node_modules/react-hot-loader/makeExportsHot.js"); if (makeExportsHot(module, require("react"))) { foundReactClasses = true; } var shouldAcceptModule = true && foundReactClasses; if (shouldAcceptModule) { module.hot.accept(function (err) { if (err) { console.error("Cannot not apply hot update to " + "index.jsx" + ": " + err.message); } }); } } module.hot.dispose(function (data) { data.makeHot = module.makeHot; data.foundReactClasses = foundReactClasses; }); })(); } }

/***/ }

});
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJzb3VyY2VzIjpbIndlYnBhY2s6Ly8vLi9zcmMvYXBwL3JlYWN0b3IuanMiLCJ3ZWJwYWNrOi8vL2V4dGVybmFsIFwialF1ZXJ5XCIiLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9zZXNzaW9uLmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvYXV0aC5qcyIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL3NlcnZpY2VzL2FwaS5qcyIsIndlYnBhY2s6Ly8vLi9+L2hpc3RvcnkvbGliL2NyZWF0ZUJyb3dzZXJIaXN0b3J5LmpzIiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9hcHAuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9sb2dpbi5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL25hdkxlZnRCYXIuanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvY29tcG9uZW50cy9uZXdVc2VyLmpzeCIsIndlYnBhY2s6Ly8vLi9zcmMvYXBwL2NvbXBvbmVudHMvbm9kZXMvbWFpbi5qc3giLCJ3ZWJwYWNrOi8vLy4vc3JjL2FwcC9jb21wb25lbnRzL3Nlc3Npb25zL21haW4uanN4Iiwid2VicGFjazovLy8uL3NyYy9hcHAvaW5kZXguanN4Il0sIm5hbWVzIjpbXSwibWFwcGluZ3MiOiI7Ozs7Ozs7Ozs7Ozs7Ozs7O3NDQUF3QixFQUFZOztBQUVwQyxLQUFNLE9BQU8sR0FBRyx1QkFBWTtBQUMxQixRQUFLLEVBQUUsSUFBSTtFQUNaLENBQUM7O3NCQUVhLE9BQU87Ozs7Ozs7O0FDTnRCLHlCOzs7Ozs7Ozs7QUNBQSxLQUFJLGFBQWEsR0FBRyxtQkFBTyxDQUFDLEdBQWtDLENBQUMsQ0FBQzs7QUFFaEUsS0FBTSxhQUFhLEdBQUcsVUFBVSxDQUFDOztBQUVqQyxLQUFJLFFBQVEsR0FBRyxJQUFJLENBQUM7O0FBRXBCLEtBQUksT0FBTyxHQUFHOztBQUVaLE9BQUksa0JBQUU7QUFDSixhQUFRLEdBQUcsYUFBYSxFQUFFLENBQUM7SUFDNUI7O0FBRUQsYUFBVSx3QkFBRTtBQUNWLFlBQU8sUUFBUSxDQUFDO0lBQ2pCOztBQUVELGNBQVcsdUJBQUMsUUFBUSxFQUFDO0FBQ25CLG1CQUFjLENBQUMsT0FBTyxDQUFDLGFBQWEsRUFBRSxJQUFJLENBQUMsU0FBUyxDQUFDLFFBQVEsQ0FBQyxDQUFDLENBQUM7SUFDakU7O0FBRUQsY0FBVyx5QkFBRTtBQUNYLFNBQUksSUFBSSxHQUFHLGNBQWMsQ0FBQyxPQUFPLENBQUMsYUFBYSxDQUFDLENBQUM7QUFDakQsU0FBRyxJQUFJLEVBQUM7QUFDTixjQUFPLElBQUksQ0FBQyxLQUFLLENBQUMsSUFBSSxDQUFDLENBQUM7TUFDekI7O0FBRUQsWUFBTyxFQUFFLENBQUM7SUFDWDs7QUFFRCxRQUFLLG1CQUFFO0FBQ0wsbUJBQWMsQ0FBQyxLQUFLLEVBQUU7SUFDdkI7O0VBRUY7O0FBRUQsT0FBTSxDQUFDLE9BQU8sR0FBRyxPQUFPLEM7Ozs7Ozs7OztBQ25DeEIsS0FBSSxHQUFHLEdBQUcsbUJBQU8sQ0FBQyxHQUFnQixDQUFDLENBQUM7QUFDcEMsS0FBSSxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxFQUFXLENBQUMsQ0FBQztBQUNuQyxLQUFJLENBQUMsR0FBRyxtQkFBTyxDQUFDLEVBQVEsQ0FBQyxDQUFDOztBQUUxQixLQUFNLFdBQVcsR0FBRyxLQUFLLEdBQUcsQ0FBQyxDQUFDOztBQUU5QixLQUFJLG1CQUFtQixHQUFHLElBQUksQ0FBQzs7QUFFL0IsS0FBSSxJQUFJLEdBQUc7O0FBRVQsUUFBSyxpQkFBQyxLQUFLLEVBQUUsUUFBUSxFQUFDO0FBQ3BCLFNBQUksQ0FBQyxtQkFBbUIsRUFBRSxDQUFDO0FBQzNCLFlBQU8sSUFBSSxDQUFDLE1BQU0sQ0FBQyxLQUFLLEVBQUUsUUFBUSxDQUFDLENBQUMsSUFBSSxDQUFDLElBQUksQ0FBQyxvQkFBb0IsQ0FBQyxDQUFDO0lBQ3JFOztBQUVELGFBQVUsd0JBQUU7QUFDVixTQUFHLE9BQU8sQ0FBQyxXQUFXLEVBQUUsQ0FBQyxJQUFJLEVBQUM7O0FBRTVCLFdBQUcsSUFBSSxDQUFDLHVCQUF1QixFQUFFLEtBQUssSUFBSSxFQUFDO0FBQ3pDLGdCQUFPLElBQUksQ0FBQyxNQUFNLEVBQUUsQ0FBQyxJQUFJLENBQUMsSUFBSSxDQUFDLG9CQUFvQixDQUFDLENBQUM7UUFDdEQ7O0FBRUQsY0FBTyxDQUFDLENBQUMsUUFBUSxFQUFFLENBQUMsT0FBTyxFQUFFLENBQUM7TUFDL0I7O0FBRUQsWUFBTyxDQUFDLENBQUMsUUFBUSxFQUFFLENBQUMsTUFBTSxFQUFFLENBQUM7SUFDOUI7O0FBRUQsU0FBTSxvQkFBRTtBQUNOLFNBQUksQ0FBQyxtQkFBbUIsRUFBRSxDQUFDO0FBQzNCLFlBQU8sT0FBTyxDQUFDLEtBQUssRUFBRSxDQUFDO0lBQ3hCOztBQUVELHVCQUFvQixrQ0FBRTtBQUNwQix3QkFBbUIsR0FBRyxXQUFXLENBQUMsSUFBSSxDQUFDLGFBQWEsRUFBRSxXQUFXLENBQUMsQ0FBQztJQUNwRTs7QUFFRCxzQkFBbUIsaUNBQUU7QUFDbkIsa0JBQWEsQ0FBQyxtQkFBbUIsQ0FBQyxDQUFDO0FBQ25DLHdCQUFtQixHQUFHLElBQUksQ0FBQztJQUM1Qjs7QUFFRCwwQkFBdUIscUNBQUU7QUFDdkIsWUFBTyxtQkFBbUIsQ0FBQztJQUM1Qjs7QUFFRCxnQkFBYSwyQkFBRTtBQUNiLFNBQUksQ0FBQyxNQUFNLEVBQUUsQ0FBQyxJQUFJLENBQUMsWUFBSTtBQUNyQixXQUFJLENBQUMsTUFBTSxFQUFFLENBQUM7QUFDZCxhQUFNLENBQUMsUUFBUSxDQUFDLE1BQU0sRUFBRSxDQUFDO01BQzFCLENBQUM7SUFDSDs7QUFFRCxTQUFNLGtCQUFDLEtBQUssRUFBRSxRQUFRLEVBQUM7QUFDckIsWUFBTyxHQUFHLENBQUMsS0FBSyxDQUFDLEtBQUssRUFBRSxRQUFRLENBQUMsQ0FBQyxJQUFJLENBQUMsY0FBSSxFQUFFO0FBQzNDLGNBQU8sQ0FBQyxXQUFXLENBQUMsSUFBSSxDQUFDLENBQUM7QUFDMUIsY0FBTyxJQUFJLENBQUM7TUFDYixDQUFDLENBQUM7SUFDSjtFQUNGOztBQUVELE9BQU0sQ0FBQyxPQUFPLEdBQUcsSUFBSSxDOzs7Ozs7Ozs7QUM3RHJCLEtBQUksQ0FBQyxHQUFHLG1CQUFPLENBQUMsRUFBUSxDQUFDLENBQUM7QUFDMUIsS0FBSSxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxFQUFjLENBQUMsQ0FBQzs7QUFFdEMsS0FBTSxRQUFRLEdBQUcsTUFBTSxDQUFDLFFBQVEsQ0FBQyxNQUFNLENBQUM7O0FBRXhDLEtBQU0sR0FBRyxHQUFHO0FBQ1YsUUFBSyxpQkFBQyxRQUFRLEVBQUUsUUFBUSxFQUFDO0FBQ3ZCLFNBQUksT0FBTyxHQUFHO0FBQ1osVUFBRyxFQUFFLFFBQVEsR0FBRyxxQkFBcUI7QUFDckMsV0FBSSxFQUFFLE1BQU07QUFDWixlQUFRLEVBQUUsTUFBTTtBQUNoQixpQkFBVSxFQUFFLG9CQUFTLEdBQUcsRUFBRTtBQUN4QixhQUFHLENBQUMsQ0FBQyxRQUFRLElBQUksQ0FBQyxDQUFDLFFBQVEsRUFBQztBQUMxQixjQUFHLENBQUMsZ0JBQWdCLENBQUUsZUFBZSxFQUFFLFFBQVEsR0FBRyxJQUFJLENBQUMsUUFBUSxHQUFHLEdBQUcsR0FBRyxRQUFRLENBQUMsQ0FBQyxDQUFDO1VBQ3BGLE1BQUk7c0NBQ2EsT0FBTyxDQUFDLFdBQVcsRUFBRTs7ZUFBL0IsS0FBSyx3QkFBTCxLQUFLOztBQUNYLGNBQUcsQ0FBQyxnQkFBZ0IsQ0FBQyxlQUFlLEVBQUMsU0FBUyxHQUFHLEtBQUssQ0FBQyxDQUFDO1VBQ3pEO1FBQ0Y7TUFDRjs7QUFFRCxZQUFPLENBQUMsQ0FBQyxJQUFJLENBQUMsT0FBTyxDQUFDLENBQUMsSUFBSSxDQUFDLGNBQUksRUFBSTtBQUNsQyxjQUFPO0FBQ0wsYUFBSSxFQUFFO0FBQ0osZUFBSSxFQUFFLElBQUksQ0FBQyxJQUFJLENBQUMsSUFBSTtBQUNwQixvQkFBUyxFQUFFLElBQUksQ0FBQyxJQUFJLENBQUMsVUFBVTtBQUMvQixpQkFBTSxFQUFFLElBQUksQ0FBQyxJQUFJLENBQUMsT0FBTztVQUMxQjtBQUNELGNBQUssRUFBRSxJQUFJLENBQUMsS0FBSztRQUNsQjtNQUNGLENBQUM7SUFDSDtFQUNGOztBQUVELE9BQU0sQ0FBQyxPQUFPLEdBQUcsR0FBRyxDOzs7Ozs7O0FDbENwQjs7QUFFQTs7QUFFQSxvREFBbUQsZ0JBQWdCLHNCQUFzQixPQUFPLDJCQUEyQiwwQkFBMEIseURBQXlELDJCQUEyQixFQUFFLEVBQUUsRUFBRSxlQUFlOztBQUU5UCx1Q0FBc0MsdUNBQXVDLGtCQUFrQjs7QUFFL0Y7O0FBRUE7O0FBRUE7O0FBRUE7O0FBRUE7O0FBRUE7O0FBRUE7O0FBRUE7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQSx5RUFBd0U7O0FBRXhFOztBQUVBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0EsTUFBSztBQUNMO0FBQ0E7O0FBRUEsK0RBQThELGlCQUFpQixXQUFXO0FBQzFGOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTs7QUFFQTtBQUNBLDZDQUE0Qzs7QUFFNUM7QUFDQTs7QUFFQTs7QUFFQTtBQUNBO0FBQ0E7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7QUFDQTtBQUNBO0FBQ0E7QUFDQTtBQUNBOztBQUVBLHlDQUF3Qzs7QUFFeEM7O0FBRUE7QUFDQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBO0FBQ0Esc0JBQXFCO0FBQ3JCLFFBQU87QUFDUDtBQUNBO0FBQ0EsTUFBSztBQUNMO0FBQ0E7QUFDQTtBQUNBLHNCQUFxQjtBQUNyQixRQUFPO0FBQ1A7QUFDQTtBQUNBO0FBQ0E7O0FBRUEsMERBQXlEO0FBQ3pEO0FBQ0E7QUFDQTtBQUNBLElBQUc7O0FBRUg7QUFDQTs7QUFFQTtBQUNBOztBQUVBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7O0FBRUE7QUFDQTs7QUFFQTtBQUNBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUE7QUFDQTtBQUNBOztBQUVBO0FBQ0E7O0FBRUEscUJBQW9CO0FBQ3BCO0FBQ0E7QUFDQTtBQUNBO0FBQ0EsSUFBRztBQUNIOztBQUVBO0FBQ0EscUM7Ozs7Ozs7Ozs7OztBQzNLQSxLQUFJLEtBQUssR0FBRyxtQkFBTyxDQUFDLENBQU8sQ0FBQyxDQUFDO0FBQzdCLEtBQUksVUFBVSxHQUFHLG1CQUFPLENBQUMsR0FBYyxDQUFDLENBQUM7O0FBRXpDLEtBQUksR0FBRyxHQUFHLEtBQUssQ0FBQyxXQUFXLENBQUM7OztBQUMxQixTQUFNLEVBQUUsa0JBQVU7QUFDaEIsWUFDRTs7U0FBSyxTQUFTLEVBQUMsS0FBSztPQUNsQixvQkFBQyxVQUFVLE9BQUU7T0FDYjs7V0FBSyxLQUFLLEVBQUUsRUFBQyxZQUFZLEVBQUUsT0FBTyxFQUFFO1NBQ2pDLElBQUksQ0FBQyxLQUFLLENBQUMsUUFBUTtRQUNoQjtNQUNGLENBQ047SUFDSDtFQUNGLENBQUM7O0FBRUYsT0FBTSxDQUFDLE9BQU8sR0FBRyxHQUFHLEM7Ozs7Ozs7Ozs7Ozs7QUNoQnBCLEtBQUksS0FBSyxHQUFHLG1CQUFPLENBQUMsQ0FBTyxDQUFDLENBQUM7QUFDN0IsS0FBSSxDQUFDLEdBQUcsbUJBQU8sQ0FBQyxFQUFRLENBQUMsQ0FBQztBQUMxQixLQUFJLE9BQU8sR0FBRyxtQkFBTyxDQUFDLEVBQWEsQ0FBQyxDQUFDOztBQUVyQyxLQUFJLEtBQUssR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFFNUIsU0FBTSxFQUFFLENBQUMsT0FBTyxDQUFDLFVBQVUsQ0FBQzs7QUFFNUIsa0JBQWUsNkJBQUc7QUFDaEIsWUFBTzs7TUFFTjtJQUNGOztBQUVELGVBQVksRUFBRSxzQkFBUyxDQUFDLEVBQUM7QUFDdkIsTUFBQyxDQUFDLGNBQWMsRUFBRSxDQUFDO0FBQ25CLFNBQUcsSUFBSSxDQUFDLE9BQU8sRUFBRSxFQUFDO0FBQ2hCLFdBQUksR0FBRyxHQUFHLElBQUksQ0FBQyxLQUFLLENBQUMsUUFBUSxDQUFDO0FBQzlCLFdBQUksS0FBSyxHQUFHLElBQUksQ0FBQyxJQUFJLENBQUMsS0FBSyxDQUFDLEtBQUssQ0FBQztBQUNsQyxXQUFJLElBQUksR0FBRyxJQUFJLENBQUMsSUFBSSxDQUFDLElBQUksQ0FBQyxLQUFLLENBQUM7QUFDaEMsV0FBSSxRQUFRLEdBQUcsTUFBTSxDQUFDOztBQUV0QixXQUFHLEdBQUcsQ0FBQyxLQUFLLElBQUksR0FBRyxDQUFDLEtBQUssQ0FBQyxVQUFVLEVBQUM7QUFDbkMsaUJBQVEsR0FBRyxHQUFHLENBQUMsS0FBSyxDQUFDLFVBQVUsQ0FBQztRQUNqQzs7O01BR0Y7SUFDRjs7QUFFRCxVQUFPLEVBQUUsbUJBQVU7QUFDaEIsU0FBSSxLQUFLLEdBQUcsQ0FBQyxDQUFDLG1CQUFtQixDQUFDLENBQUM7QUFDbkMsWUFBTyxLQUFLLENBQUMsTUFBTSxLQUFLLENBQUMsSUFBSSxLQUFLLENBQUMsS0FBSyxFQUFFLENBQUM7SUFDN0M7O0FBRUQsU0FBTSxFQUFFLGtCQUFXO0FBQ2pCLFNBQUksWUFBWSxHQUFHLEtBQUssQ0FBQztBQUN6QixTQUFJLE9BQU8sR0FBRyxLQUFLLENBQUM7O0FBRXBCLFlBQ0U7O1NBQUssU0FBUyxFQUFDLG9DQUFvQztPQUNqRDs7O1NBQ0U7OztXQUNFOztlQUFJLFNBQVMsRUFBQyxXQUFXOztZQUFPO1VBQzVCO1NBQ047Ozs7VUFBa0M7U0FDbEM7Ozs7VUFBaUI7U0FDakI7O2FBQUssU0FBUyxFQUFDLEtBQUssRUFBQyxJQUFJLEVBQUMsTUFBTSxFQUFDLFFBQVEsRUFBRSxJQUFJLENBQUMsWUFBYTtXQUMzRDs7ZUFBSyxTQUFTLEVBQUMsWUFBWTthQUN6QiwrQkFBTyxJQUFJLEVBQUMsT0FBTyxFQUFDLEdBQUcsRUFBQyxPQUFPLEVBQUMsSUFBSSxFQUFDLE9BQU8sRUFBQyxTQUFTLEVBQUMsdUJBQXVCLEVBQUMsV0FBVyxFQUFDLFVBQVUsRUFBQyxRQUFRLFNBQUc7WUFDN0c7V0FDTjs7ZUFBSyxTQUFTLEVBQUMsWUFBWTthQUN6QiwrQkFBTyxJQUFJLEVBQUMsVUFBVSxFQUFDLEdBQUcsRUFBQyxNQUFNLEVBQUMsSUFBSSxFQUFDLFVBQVUsRUFBQyxTQUFTLEVBQUMsdUJBQXVCLEVBQUMsV0FBVyxFQUFDLFVBQVUsRUFBQyxRQUFRLFNBQUc7WUFDbEg7V0FDTjs7ZUFBUSxJQUFJLEVBQUMsUUFBUSxFQUFDLE9BQU8sRUFBRyxJQUFJLENBQUMsWUFBYSxFQUFDLFNBQVMsRUFBQyxzQ0FBc0MsRUFBQyxRQUFRLEVBQUUsWUFBYTs7WUFBZTtVQUN0STtRQUNEO01BQ0gsQ0FDTjtJQUNIO0VBQ0YsQ0FBQyxDQUFDOztBQUVILE9BQU0sQ0FBQyxPQUFPLEdBQUcsS0FBSyxDOzs7Ozs7Ozs7Ozs7O0FDOUR0QixLQUFJLEtBQUssR0FBRyxtQkFBTyxDQUFDLENBQU8sQ0FBQyxDQUFDO0FBQzdCLEtBQUksTUFBTSxHQUFHLG1CQUFPLENBQUMsRUFBYyxDQUFDLENBQUM7QUFDckMsS0FBSSxTQUFTLEdBQUcsTUFBTSxDQUFDLFNBQVMsQ0FBQztBQUNqQyxLQUFJLE9BQU8sR0FBRyxtQkFBTyxDQUFDLEVBQWMsQ0FBQyxDQUFDLE9BQU8sQ0FBQzs7QUFFOUMsS0FBSSxTQUFTLEdBQUcsQ0FDZCxFQUFDLElBQUksRUFBRSxrQkFBa0IsRUFBRSxFQUFFLGNBQWMsRUFBRSxLQUFLLEVBQUUsT0FBTyxFQUFDLEVBQzVELEVBQUMsSUFBSSxFQUFFLGFBQWEsRUFBRSxFQUFFLGlCQUFpQixFQUFFLEtBQUssRUFBRSxVQUFVLEVBQUMsQ0FDOUQsQ0FBQzs7QUFFRixLQUFJLFVBQVUsR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFFakMsU0FBTSxFQUFFLENBQUUsT0FBTyxDQUFFOztBQUVuQixTQUFNLEVBQUUsa0JBQVU7QUFDaEIsU0FBSSxJQUFJLEdBQUcsSUFBSSxDQUFDO0FBQ2hCLFNBQUksS0FBSyxHQUFHLFNBQVMsQ0FBQyxHQUFHLENBQUMsVUFBUyxDQUFDLEVBQUUsS0FBSyxFQUFDO0FBQzFDLFdBQUksU0FBUyxHQUFHLElBQUksQ0FBQyxPQUFPLENBQUMsUUFBUSxDQUFDLENBQUMsQ0FBQyxFQUFFLENBQUMsR0FBRyxRQUFRLEdBQUcsRUFBRSxDQUFDO0FBQzVELGNBQ0U7O1dBQUksR0FBRyxFQUFFLEtBQU0sRUFBQyxTQUFTLEVBQUUsU0FBVTtTQUNuQztBQUFDLG9CQUFTO2FBQUMsRUFBRSxFQUFFLENBQUMsQ0FBQyxFQUFHO1dBQ2xCLDJCQUFHLFNBQVMsRUFBRSxDQUFDLENBQUMsSUFBSyxFQUFDLEtBQUssRUFBRSxDQUFDLENBQUMsS0FBTSxHQUFFO1VBQzdCO1FBQ1QsQ0FDTDtNQUNILENBQUMsQ0FBQzs7QUFFSCxZQUNFOztTQUFLLFNBQVMsRUFBQyxFQUFFLEVBQUMsSUFBSSxFQUFDLFlBQVksRUFBQyxLQUFLLEVBQUUsRUFBQyxLQUFLLEVBQUUsTUFBTSxFQUFFLEtBQUssRUFBRSxNQUFNLEVBQUU7T0FDeEU7O1dBQUssU0FBUyxFQUFDLEVBQUU7U0FDZjs7YUFBSSxTQUFTLEVBQUMsZ0JBQWdCLEVBQUMsRUFBRSxFQUFDLFdBQVc7V0FDMUMsS0FBSztVQUNIO1FBQ0Q7TUFDRixDQUNOO0lBQ0g7RUFDRixDQUFDLENBQUM7O0FBRUgsT0FBTSxDQUFDLE9BQU8sR0FBRyxVQUFVLEM7Ozs7Ozs7Ozs7Ozs7QUN2QzNCLEtBQUksS0FBSyxHQUFHLG1CQUFPLENBQUMsQ0FBTyxDQUFDLENBQUM7QUFDN0IsS0FBSSxDQUFDLEdBQUcsbUJBQU8sQ0FBQyxFQUFRLENBQUMsQ0FBQztBQUMxQixLQUFJLE9BQU8sR0FBRyxtQkFBTyxDQUFDLEVBQWEsQ0FBQyxDQUFDOztBQUVyQyxLQUFJLE1BQU0sR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFFN0IsZUFBWSxFQUFFLHNCQUFTLENBQUMsRUFBRTtBQUN4QixNQUFDLENBQUMsY0FBYyxFQUFFLENBQUM7QUFDbkIsU0FBSSxJQUFJLENBQUMsT0FBTyxFQUFFLEVBQUU7QUFDbEIsV0FBSSxHQUFHLEdBQUcsSUFBSSxDQUFDLEtBQUssQ0FBQyxRQUFRLENBQUM7QUFDOUIsV0FBSSxLQUFLLEdBQUcsSUFBSSxDQUFDLElBQUksQ0FBQyxLQUFLLENBQUMsS0FBSyxDQUFDO0FBQ2xDLFdBQUksSUFBSSxHQUFHLElBQUksQ0FBQyxJQUFJLENBQUMsSUFBSSxDQUFDLEtBQUssQ0FBQztBQUNoQyxXQUFJLFFBQVEsR0FBRyxNQUFNLENBQUM7O0FBRXRCLFdBQUksR0FBRyxDQUFDLEtBQUssSUFBSSxHQUFHLENBQUMsS0FBSyxDQUFDLFVBQVUsRUFBRTtBQUNyQyxpQkFBUSxHQUFHLEdBQUcsQ0FBQyxLQUFLLENBQUMsVUFBVSxDQUFDO1FBQ2pDOzs7TUFHRjtJQUNGOztBQUVELFVBQU8sRUFBRSxtQkFBVztBQUNsQixTQUFJLEtBQUssR0FBRyxDQUFDLENBQUMsbUJBQW1CLENBQUMsQ0FBQztBQUNuQyxZQUFPLEtBQUssQ0FBQyxNQUFNLEtBQUssQ0FBQyxJQUFJLEtBQUssQ0FBQyxLQUFLLEVBQUUsQ0FBQztJQUM1Qzs7QUFFRCxTQUFNLEVBQUUsa0JBQVc7QUFDakIsU0FBSSxZQUFZLEdBQUcsS0FBSyxDQUFDO0FBQ3pCLFNBQUksT0FBTyxHQUFHLEtBQUssQ0FBQzs7QUFFcEIsWUFDRTs7U0FBSyxTQUFTLEVBQUMseURBQXlEO09BQ3RFOzs7U0FDRTs7O1dBQ0U7O2VBQUksU0FBUyxFQUFDLFdBQVc7O1lBQU87VUFDNUI7U0FDTjs7OztVQUEyQjtTQUMzQjs7OztVQUF1QjtTQUN2Qjs7YUFBSyxLQUFLLEVBQUMsTUFBTTtXQUNmOztlQUFNLEtBQUssRUFBQyxPQUFPOzthQUVqQiwrQkFBUzs7YUFFVCwrQkFBUzs7YUFFVCwrQkFBUzs7YUFFVCwrQkFBUztZQUNKO1VBQ0g7U0FDTjs7YUFBTSxTQUFTLEVBQUMsS0FBSyxFQUFDLElBQUksRUFBQyxNQUFNLEVBQUMsTUFBTSxFQUFDLG9CQUFvQixFQUFDLE1BQU0sRUFBQyxNQUFNO1dBQ3pFOztlQUFLLFNBQVMsRUFBQyxZQUFZO2FBQ3pCLCtCQUFPLElBQUksRUFBQyxRQUFRLEVBQUMsSUFBSSxFQUFDLE9BQU8sRUFBQyxTQUFTLEVBQUMsY0FBYyxHQUFFO1lBQ3hEO1dBQ047O2VBQUssU0FBUyxFQUFDLFlBQVk7YUFDekIsK0JBQU8sSUFBSSxFQUFDLE1BQU0sRUFBQyxJQUFJLEVBQUMsVUFBVSxFQUFDLFFBQVEsUUFBQyxTQUFTLEVBQUMsY0FBYyxFQUFDLFdBQVcsRUFBQyxVQUFVLEVBQUMsUUFBUSxFQUFDLEVBQUUsR0FBRTtZQUNyRztXQUNOOztlQUFLLFNBQVMsRUFBQyxZQUFZO2FBQ3pCLCtCQUFPLElBQUksRUFBQyxVQUFVLEVBQUMsSUFBSSxFQUFDLFVBQVUsRUFBQyxFQUFFLEVBQUMsVUFBVSxFQUFDLFNBQVMsRUFBQyxjQUFjLEVBQUMsV0FBVyxFQUFDLFVBQVUsRUFBQyxRQUFRLEVBQUMsRUFBRSxFQUFDLFFBQVEsRUFBQyxrQkFBa0IsR0FBRTtZQUMxSTtXQUNOOztlQUFLLFNBQVMsRUFBQyxZQUFZO2FBQ3pCLCtCQUFPLElBQUksRUFBQyxVQUFVLEVBQUMsSUFBSSxFQUFDLGtCQUFrQixFQUFDLEVBQUUsRUFBQyxrQkFBa0IsRUFBQyxTQUFTLEVBQUMsY0FBYyxFQUFDLFdBQVcsRUFBQyxrQkFBa0IsRUFBQyxRQUFRLEVBQUMsRUFBRSxFQUFDLFFBQVEsRUFBQyxrQkFBa0IsR0FBRTtZQUNsSztXQUNOOztlQUFLLFNBQVMsRUFBQyxZQUFZO2FBQ3pCLCtCQUFPLElBQUksRUFBQyxNQUFNLEVBQUMsSUFBSSxFQUFDLFlBQVksRUFBQyxFQUFFLEVBQUMsWUFBWSxFQUFDLFNBQVMsRUFBQyxjQUFjLEVBQUMsV0FBVyxFQUFDLFlBQVksRUFBQyxRQUFRLEVBQUMsRUFBRSxHQUFFO1lBQ2hIO1dBQ047O2VBQVEsSUFBSSxFQUFDLFFBQVEsRUFBQyxTQUFTLEVBQUMsc0NBQXNDOztZQUFpQjtVQUNsRjtRQUNIO01BQ0YsQ0FDTjtJQUNIO0VBQ0YsQ0FBQyxDQUFDOztBQUVILE9BQU0sQ0FBQyxPQUFPLEdBQUcsTUFBTSxDOzs7Ozs7Ozs7Ozs7O0FDM0V2QixLQUFJLEtBQUssR0FBRyxtQkFBTyxDQUFDLENBQU8sQ0FBQyxDQUFDO0FBQzdCLEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsRUFBYSxDQUFDLENBQUM7QUFDckMsS0FBSSxLQUFLLEdBQUcsS0FBSyxDQUFDLFdBQVcsQ0FBQzs7O0FBQzVCLFNBQU0sRUFBRSxrQkFBVztBQUNqQixZQUNFOzs7T0FDRTs7OztRQUFnQjtPQUNoQjs7V0FBSyxTQUFTLEVBQUMsRUFBRTtTQUNmOzthQUFLLFNBQVMsRUFBQyxFQUFFO1dBQ2Y7O2VBQUssU0FBUyxFQUFDLEVBQUU7YUFDZjs7aUJBQU8sU0FBUyxFQUFDLHFCQUFxQjtlQUNwQzs7O2lCQUNFOzs7bUJBQ0U7Ozs7b0JBQWE7bUJBQ2I7Ozs7b0JBQWU7bUJBQ2Y7Ozs7b0JBQWU7bUJBQ2I7Ozs7b0JBQVk7bUJBQ1o7Ozs7b0JBQVk7bUJBQ1o7Ozs7b0JBQVc7bUJBQ1g7Ozs7b0JBQXlCO2tCQUN0QjtnQkFDQztlQUNWLGtDQUFlO2NBQ1Q7WUFDSjtVQUNGO1FBQ0Y7TUFDRixDQUNQO0lBQ0Y7RUFDRixDQUFDLENBQUM7O0FBRUgsT0FBTSxDQUFDLE9BQU8sR0FBRyxLQUFLLEM7Ozs7Ozs7Ozs7Ozs7QUNoQ3RCLEtBQUksS0FBSyxHQUFHLG1CQUFPLENBQUMsQ0FBTyxDQUFDLENBQUM7QUFDN0IsS0FBSSxPQUFPLEdBQUcsbUJBQU8sQ0FBQyxFQUFhLENBQUMsQ0FBQztBQUNyQyxLQUFJLEtBQUssR0FBRyxLQUFLLENBQUMsV0FBVyxDQUFDOzs7QUFDNUIsU0FBTSxFQUFFLGtCQUFXO0FBQ2pCLFlBQ0U7OztPQUNFOzs7O1FBQW1CO09BQ25COztXQUFLLFNBQVMsRUFBQyxFQUFFO1NBQ2Y7O2FBQUssU0FBUyxFQUFDLEVBQUU7V0FDZjs7ZUFBSyxTQUFTLEVBQUMsRUFBRTthQUNmOztpQkFBTyxTQUFTLEVBQUMscUJBQXFCO2VBQ3BDOzs7aUJBQ0U7OzttQkFDRTs7OztvQkFBYTttQkFDYjs7OztvQkFBZTttQkFDZjs7OztvQkFBZTttQkFDYjs7OztvQkFBWTttQkFDWjs7OztvQkFBWTttQkFDWjs7OztvQkFBVzttQkFDWDs7OztvQkFBeUI7a0JBQ3RCO2dCQUNDO2VBQ1Ysa0NBQWU7Y0FDVDtZQUNKO1VBQ0Y7UUFDRjtNQUNGLENBQ1A7SUFDRjtFQUNGLENBQUMsQ0FBQzs7QUFFSCxPQUFNLENBQUMsT0FBTyxHQUFHLEtBQUssQzs7Ozs7Ozs7Ozs7OztBQ2hDdEIsS0FBSSxLQUFLLEdBQUcsbUJBQU8sQ0FBQyxDQUFPLENBQUMsQ0FBQzs7Z0JBQ2lCLG1CQUFPLENBQUMsRUFBYyxDQUFDOztLQUEvRCxNQUFNLFlBQU4sTUFBTTtLQUFFLEtBQUssWUFBTCxLQUFLO0tBQUUsUUFBUSxZQUFSLFFBQVE7S0FBRSxVQUFVLFlBQVYsVUFBVTs7QUFDekMsS0FBSSxNQUFNLEdBQUcsbUJBQU8sQ0FBQyxFQUFXLENBQUMsQ0FBQyxNQUFNLENBQUM7OztBQUd6QyxLQUFJLEdBQUcsR0FBRyxtQkFBTyxDQUFDLEdBQXNCLENBQUMsQ0FBQztBQUMxQyxLQUFJLEtBQUssR0FBRyxtQkFBTyxDQUFDLEdBQXdCLENBQUMsQ0FBQztBQUM5QyxLQUFJLEtBQUssR0FBRyxtQkFBTyxDQUFDLEdBQTZCLENBQUMsQ0FBQztBQUNuRCxLQUFJLFFBQVEsR0FBRyxtQkFBTyxDQUFDLEdBQWdDLENBQUMsQ0FBQztBQUN6RCxLQUFJLE9BQU8sR0FBRyxtQkFBTyxDQUFDLEdBQTBCLENBQUMsQ0FBQztBQUNsRCxLQUFJLElBQUksR0FBRyxtQkFBTyxDQUFDLEdBQVEsQ0FBQyxDQUFDO0FBQzdCLEtBQUksT0FBTyxHQUFHLG1CQUFPLENBQUMsRUFBVyxDQUFDLENBQUM7OztBQUduQyxRQUFPLENBQUMsSUFBSSxFQUFFLENBQUM7O0FBRWYsVUFBUyxXQUFXLENBQUMsU0FBUyxFQUFFLE9BQU8sRUFBRSxFQUFFLEVBQUU7QUFDM0MsT0FBSSxDQUFDLFVBQVUsRUFBRSxDQUNkLElBQUksQ0FBQztZQUFLLEVBQUUsRUFBRTtJQUFBLENBQUMsQ0FDZixJQUFJLENBQUMsWUFBSTtBQUNSLFlBQU8sQ0FBQyxFQUFDLFVBQVUsRUFBRSxTQUFTLENBQUMsUUFBUSxDQUFDLFFBQVEsRUFBRSxFQUFFLFlBQVksQ0FBRSxDQUFDO0FBQ25FLE9BQUUsRUFBRSxDQUFDO0lBQ04sQ0FBQyxDQUFDO0VBQ047O0FBRUQsVUFBUyxZQUFZLENBQUMsU0FBUyxFQUFFLE9BQU8sRUFBRSxFQUFFLEVBQUM7QUFDM0MsT0FBSSxDQUFDLE1BQU0sRUFBRSxDQUFDOztBQUVkLFVBQU8sQ0FBQyxVQUFVLEVBQUUsQ0FBQyxNQUFNLEVBQUUsQ0FBQztFQUMvQjs7QUFFRCxPQUFNLENBQ0o7QUFBQyxTQUFNO0tBQUMsT0FBTyxFQUFFLE9BQU8sQ0FBQyxVQUFVLEVBQUc7R0FDcEMsb0JBQUMsS0FBSyxJQUFDLElBQUksRUFBQyxZQUFZLEVBQUMsU0FBUyxFQUFFLEtBQU0sR0FBRTtHQUM1QyxvQkFBQyxLQUFLLElBQUMsSUFBSSxFQUFDLGFBQWEsRUFBQyxPQUFPLEVBQUUsWUFBYSxHQUFFO0dBQ2xELG9CQUFDLEtBQUssSUFBQyxJQUFJLEVBQUMsY0FBYyxFQUFDLFNBQVMsRUFBRSxPQUFRLEdBQUU7R0FDaEQ7QUFBQyxVQUFLO09BQUMsSUFBSSxFQUFDLE1BQU0sRUFBQyxTQUFTLEVBQUUsR0FBSTtLQUNoQyxvQkFBQyxLQUFLLElBQUMsSUFBSSxFQUFDLE9BQU8sRUFBQyxTQUFTLEVBQUUsS0FBTSxHQUFFO0tBQ3ZDLG9CQUFDLEtBQUssSUFBQyxJQUFJLEVBQUMsVUFBVSxFQUFDLFNBQVMsRUFBRSxRQUFTLEdBQUU7SUFDdkM7RUFDRCxFQUNSLFFBQVEsQ0FBQyxjQUFjLENBQUMsS0FBSyxDQUFDLENBQUMsQyIsImZpbGUiOiJhcHAuanMiLCJzb3VyY2VzQ29udGVudCI6WyJpbXBvcnQgeyBSZWFjdG9yIH0gZnJvbSAnbnVjbGVhci1qcydcblxuY29uc3QgcmVhY3RvciA9IG5ldyBSZWFjdG9yKHtcbiAgZGVidWc6IHRydWVcbn0pXG5cbmV4cG9ydCBkZWZhdWx0IHJlYWN0b3JcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9yZWFjdG9yLmpzXG4gKiovIiwibW9kdWxlLmV4cG9ydHMgPSBqUXVlcnk7XG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiBleHRlcm5hbCBcImpRdWVyeVwiXG4gKiogbW9kdWxlIGlkID0gNDVcbiAqKiBtb2R1bGUgY2h1bmtzID0gMVxuICoqLyIsInZhciBjcmVhdGVIaXN0b3J5ID0gcmVxdWlyZSgnaGlzdG9yeS9saWIvY3JlYXRlQnJvd3Nlckhpc3RvcnknKTtcblxuY29uc3QgQVVUSF9LRVlfREFUQSA9ICdhdXRoRGF0YSc7XG5cbnZhciBfaGlzdG9yeSA9IG51bGw7XG5cbnZhciBzZXNzaW9uID0ge1xuXG4gIGluaXQoKXtcbiAgICBfaGlzdG9yeSA9IGNyZWF0ZUhpc3RvcnkoKTtcbiAgfSxcblxuICBnZXRIaXN0b3J5KCl7XG4gICAgcmV0dXJuIF9oaXN0b3J5O1xuICB9LFxuXG4gIHNldFVzZXJEYXRhKHVzZXJEYXRhKXtcbiAgICBzZXNzaW9uU3RvcmFnZS5zZXRJdGVtKEFVVEhfS0VZX0RBVEEsIEpTT04uc3RyaW5naWZ5KHVzZXJEYXRhKSk7XG4gIH0sXG5cbiAgZ2V0VXNlckRhdGEoKXtcbiAgICB2YXIgaXRlbSA9IHNlc3Npb25TdG9yYWdlLmdldEl0ZW0oQVVUSF9LRVlfREFUQSk7XG4gICAgaWYoaXRlbSl7XG4gICAgICByZXR1cm4gSlNPTi5wYXJzZShpdGVtKTtcbiAgICB9XG5cbiAgICByZXR1cm4ge307XG4gIH0sXG5cbiAgY2xlYXIoKXtcbiAgICBzZXNzaW9uU3RvcmFnZS5jbGVhcigpXG4gIH1cblxufVxuXG5tb2R1bGUuZXhwb3J0cyA9IHNlc3Npb247XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvc2Vzc2lvbi5qc1xuICoqLyIsInZhciBhcGkgPSByZXF1aXJlKCcuL3NlcnZpY2VzL2FwaScpO1xudmFyIHNlc3Npb24gPSByZXF1aXJlKCcuL3Nlc3Npb24nKTtcbnZhciAkID0gcmVxdWlyZSgnalF1ZXJ5Jyk7XG5cbmNvbnN0IHJlZnJlc2hSYXRlID0gNjAwMDAgKiAxOyAvLyAxIG1pblxuXG52YXIgcmVmcmVzaFRva2VuVGltZXJJZCA9IG51bGw7XG5cbnZhciBhdXRoID0ge1xuXG4gIGxvZ2luKGVtYWlsLCBwYXNzd29yZCl7XG4gICAgYXV0aC5fc3RvcFRva2VuUmVmcmVzaGVyKCk7XG4gICAgcmV0dXJuIGF1dGguX2xvZ2luKGVtYWlsLCBwYXNzd29yZCkuZG9uZShhdXRoLl9zdGFydFRva2VuUmVmcmVzaGVyKTtcbiAgfSxcblxuICBlbnN1cmVVc2VyKCl7XG4gICAgaWYoc2Vzc2lvbi5nZXRVc2VyRGF0YSgpLnVzZXIpe1xuICAgICAgLy8gcmVmcmVzaCB0aW1lciB3aWxsIG5vdCBiZSBzZXQgaW4gY2FzZSBvZiBicm93c2VyIHJlZnJlc2ggZXZlbnRcbiAgICAgIGlmKGF1dGguX2dldFJlZnJlc2hUb2tlblRpbWVySWQoKSA9PT0gbnVsbCl7XG4gICAgICAgIHJldHVybiBhdXRoLl9sb2dpbigpLmRvbmUoYXV0aC5fc3RhcnRUb2tlblJlZnJlc2hlcik7XG4gICAgICB9XG5cbiAgICAgIHJldHVybiAkLkRlZmVycmVkKCkucmVzb2x2ZSgpO1xuICAgIH1cblxuICAgIHJldHVybiAkLkRlZmVycmVkKCkucmVqZWN0KCk7XG4gIH0sXG5cbiAgbG9nb3V0KCl7XG4gICAgYXV0aC5fc3RvcFRva2VuUmVmcmVzaGVyKCk7XG4gICAgcmV0dXJuIHNlc3Npb24uY2xlYXIoKTtcbiAgfSxcblxuICBfc3RhcnRUb2tlblJlZnJlc2hlcigpe1xuICAgIHJlZnJlc2hUb2tlblRpbWVySWQgPSBzZXRJbnRlcnZhbChhdXRoLl9yZWZyZXNoVG9rZW4sIHJlZnJlc2hSYXRlKTtcbiAgfSxcblxuICBfc3RvcFRva2VuUmVmcmVzaGVyKCl7XG4gICAgY2xlYXJJbnRlcnZhbChyZWZyZXNoVG9rZW5UaW1lcklkKTtcbiAgICByZWZyZXNoVG9rZW5UaW1lcklkID0gbnVsbDtcbiAgfSxcblxuICBfZ2V0UmVmcmVzaFRva2VuVGltZXJJZCgpe1xuICAgIHJldHVybiByZWZyZXNoVG9rZW5UaW1lcklkO1xuICB9LFxuXG4gIF9yZWZyZXNoVG9rZW4oKXtcbiAgICBhdXRoLl9sb2dpbigpLmZhaWwoKCk9PntcbiAgICAgIGF1dGgubG9nb3V0KCk7XG4gICAgICB3aW5kb3cubG9jYXRpb24ucmVsb2FkKCk7XG4gICAgfSlcbiAgfSxcblxuICBfbG9naW4oZW1haWwsIHBhc3N3b3JkKXtcbiAgICByZXR1cm4gYXBpLmxvZ2luKGVtYWlsLCBwYXNzd29yZCkudGhlbihkYXRhPT57XG4gICAgICBzZXNzaW9uLnNldFVzZXJEYXRhKGRhdGEpO1xuICAgICAgcmV0dXJuIGRhdGE7XG4gICAgfSk7XG4gIH1cbn1cblxubW9kdWxlLmV4cG9ydHMgPSBhdXRoO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2F1dGguanNcbiAqKi8iLCJ2YXIgJCA9IHJlcXVpcmUoXCJqUXVlcnlcIik7XG52YXIgc2Vzc2lvbiA9IHJlcXVpcmUoJy4vLi4vc2Vzc2lvbicpO1xuXG5jb25zdCBCQVNFX1VSTCA9IHdpbmRvdy5sb2NhdGlvbi5vcmlnaW47XG5cbmNvbnN0IGFwaSA9IHtcbiAgbG9naW4odXNlcm5hbWUsIHBhc3N3b3JkKXtcbiAgICB2YXIgb3B0aW9ucyA9IHtcbiAgICAgIHVybDogQkFTRV9VUkwgKyBcIi9wb3J0YWwvdjEvc2Vzc2lvbnNcIixcbiAgICAgIHR5cGU6IFwiUE9TVFwiLFxuICAgICAgZGF0YVR5cGU6IFwianNvblwiLFxuICAgICAgYmVmb3JlU2VuZDogZnVuY3Rpb24oeGhyKSB7XG4gICAgICAgIGlmKCEhdXNlcm5hbWUgJiYgISFwYXNzd29yZCl7XG4gICAgICAgICAgeGhyLnNldFJlcXVlc3RIZWFkZXIgKFwiQXV0aG9yaXphdGlvblwiLCBcIkJhc2ljIFwiICsgYnRvYSh1c2VybmFtZSArIFwiOlwiICsgcGFzc3dvcmQpKTtcbiAgICAgICAgfWVsc2V7XG4gICAgICAgICAgdmFyIHsgdG9rZW4gfSA9IHNlc3Npb24uZ2V0VXNlckRhdGEoKTtcbiAgICAgICAgICB4aHIuc2V0UmVxdWVzdEhlYWRlcignQXV0aG9yaXphdGlvbicsJ0JlYXJlciAnICsgdG9rZW4pO1xuICAgICAgICB9XG4gICAgICB9XG4gICAgfVxuXG4gICAgcmV0dXJuICQuYWpheChvcHRpb25zKS50aGVuKGpzb24gPT4ge1xuICAgICAgcmV0dXJuIHtcbiAgICAgICAgdXNlcjoge1xuICAgICAgICAgIG5hbWU6IGpzb24udXNlci5uYW1lLFxuICAgICAgICAgIGFjY291bnRJZDoganNvbi51c2VyLmFjY291bnRfaWQsXG4gICAgICAgICAgc2l0ZUlkOiBqc29uLnVzZXIuc2l0ZV9pZFxuICAgICAgICB9LFxuICAgICAgICB0b2tlbjoganNvbi50b2tlblxuICAgICAgfVxuICAgIH0pXG4gIH1cbn1cblxubW9kdWxlLmV4cG9ydHMgPSBhcGk7XG5cblxuXG4vKiogV0VCUEFDSyBGT09URVIgKipcbiAqKiAuL3NyYy9hcHAvc2VydmljZXMvYXBpLmpzXG4gKiovIiwiJ3VzZSBzdHJpY3QnO1xuXG5leHBvcnRzLl9fZXNNb2R1bGUgPSB0cnVlO1xuXG52YXIgX2V4dGVuZHMgPSBPYmplY3QuYXNzaWduIHx8IGZ1bmN0aW9uICh0YXJnZXQpIHsgZm9yICh2YXIgaSA9IDE7IGkgPCBhcmd1bWVudHMubGVuZ3RoOyBpKyspIHsgdmFyIHNvdXJjZSA9IGFyZ3VtZW50c1tpXTsgZm9yICh2YXIga2V5IGluIHNvdXJjZSkgeyBpZiAoT2JqZWN0LnByb3RvdHlwZS5oYXNPd25Qcm9wZXJ0eS5jYWxsKHNvdXJjZSwga2V5KSkgeyB0YXJnZXRba2V5XSA9IHNvdXJjZVtrZXldOyB9IH0gfSByZXR1cm4gdGFyZ2V0OyB9O1xuXG5mdW5jdGlvbiBfaW50ZXJvcFJlcXVpcmVEZWZhdWx0KG9iaikgeyByZXR1cm4gb2JqICYmIG9iai5fX2VzTW9kdWxlID8gb2JqIDogeyAnZGVmYXVsdCc6IG9iaiB9OyB9XG5cbnZhciBfaW52YXJpYW50ID0gcmVxdWlyZSgnaW52YXJpYW50Jyk7XG5cbnZhciBfaW52YXJpYW50MiA9IF9pbnRlcm9wUmVxdWlyZURlZmF1bHQoX2ludmFyaWFudCk7XG5cbnZhciBfQWN0aW9ucyA9IHJlcXVpcmUoJy4vQWN0aW9ucycpO1xuXG52YXIgX0V4ZWN1dGlvbkVudmlyb25tZW50ID0gcmVxdWlyZSgnLi9FeGVjdXRpb25FbnZpcm9ubWVudCcpO1xuXG52YXIgX0RPTVV0aWxzID0gcmVxdWlyZSgnLi9ET01VdGlscycpO1xuXG52YXIgX0RPTVN0YXRlU3RvcmFnZSA9IHJlcXVpcmUoJy4vRE9NU3RhdGVTdG9yYWdlJyk7XG5cbnZhciBfY3JlYXRlRE9NSGlzdG9yeSA9IHJlcXVpcmUoJy4vY3JlYXRlRE9NSGlzdG9yeScpO1xuXG52YXIgX2NyZWF0ZURPTUhpc3RvcnkyID0gX2ludGVyb3BSZXF1aXJlRGVmYXVsdChfY3JlYXRlRE9NSGlzdG9yeSk7XG5cbi8qKlxuICogQ3JlYXRlcyBhbmQgcmV0dXJucyBhIGhpc3Rvcnkgb2JqZWN0IHRoYXQgdXNlcyBIVE1MNSdzIGhpc3RvcnkgQVBJXG4gKiAocHVzaFN0YXRlLCByZXBsYWNlU3RhdGUsIGFuZCB0aGUgcG9wc3RhdGUgZXZlbnQpIHRvIG1hbmFnZSBoaXN0b3J5LlxuICogVGhpcyBpcyB0aGUgcmVjb21tZW5kZWQgbWV0aG9kIG9mIG1hbmFnaW5nIGhpc3RvcnkgaW4gYnJvd3NlcnMgYmVjYXVzZVxuICogaXQgcHJvdmlkZXMgdGhlIGNsZWFuZXN0IFVSTHMuXG4gKlxuICogTm90ZTogSW4gYnJvd3NlcnMgdGhhdCBkbyBub3Qgc3VwcG9ydCB0aGUgSFRNTDUgaGlzdG9yeSBBUEkgZnVsbFxuICogcGFnZSByZWxvYWRzIHdpbGwgYmUgdXNlZCB0byBwcmVzZXJ2ZSBVUkxzLlxuICovXG5mdW5jdGlvbiBjcmVhdGVCcm93c2VySGlzdG9yeSgpIHtcbiAgdmFyIG9wdGlvbnMgPSBhcmd1bWVudHMubGVuZ3RoIDw9IDAgfHwgYXJndW1lbnRzWzBdID09PSB1bmRlZmluZWQgPyB7fSA6IGFyZ3VtZW50c1swXTtcblxuICAhX0V4ZWN1dGlvbkVudmlyb25tZW50LmNhblVzZURPTSA/IHByb2Nlc3MuZW52Lk5PREVfRU5WICE9PSAncHJvZHVjdGlvbicgPyBfaW52YXJpYW50MlsnZGVmYXVsdCddKGZhbHNlLCAnQnJvd3NlciBoaXN0b3J5IG5lZWRzIGEgRE9NJykgOiBfaW52YXJpYW50MlsnZGVmYXVsdCddKGZhbHNlKSA6IHVuZGVmaW5lZDtcblxuICB2YXIgZm9yY2VSZWZyZXNoID0gb3B0aW9ucy5mb3JjZVJlZnJlc2g7XG5cbiAgdmFyIGlzU3VwcG9ydGVkID0gX0RPTVV0aWxzLnN1cHBvcnRzSGlzdG9yeSgpO1xuICB2YXIgdXNlUmVmcmVzaCA9ICFpc1N1cHBvcnRlZCB8fCBmb3JjZVJlZnJlc2g7XG5cbiAgZnVuY3Rpb24gZ2V0Q3VycmVudExvY2F0aW9uKGhpc3RvcnlTdGF0ZSkge1xuICAgIGhpc3RvcnlTdGF0ZSA9IGhpc3RvcnlTdGF0ZSB8fCB3aW5kb3cuaGlzdG9yeS5zdGF0ZSB8fCB7fTtcblxuICAgIHZhciBwYXRoID0gX0RPTVV0aWxzLmdldFdpbmRvd1BhdGgoKTtcbiAgICB2YXIgX2hpc3RvcnlTdGF0ZSA9IGhpc3RvcnlTdGF0ZTtcbiAgICB2YXIga2V5ID0gX2hpc3RvcnlTdGF0ZS5rZXk7XG5cbiAgICB2YXIgc3RhdGUgPSB1bmRlZmluZWQ7XG4gICAgaWYgKGtleSkge1xuICAgICAgc3RhdGUgPSBfRE9NU3RhdGVTdG9yYWdlLnJlYWRTdGF0ZShrZXkpO1xuICAgIH0gZWxzZSB7XG4gICAgICBzdGF0ZSA9IG51bGw7XG4gICAgICBrZXkgPSBoaXN0b3J5LmNyZWF0ZUtleSgpO1xuXG4gICAgICBpZiAoaXNTdXBwb3J0ZWQpIHdpbmRvdy5oaXN0b3J5LnJlcGxhY2VTdGF0ZShfZXh0ZW5kcyh7fSwgaGlzdG9yeVN0YXRlLCB7IGtleToga2V5IH0pLCBudWxsLCBwYXRoKTtcbiAgICB9XG5cbiAgICByZXR1cm4gaGlzdG9yeS5jcmVhdGVMb2NhdGlvbihwYXRoLCBzdGF0ZSwgdW5kZWZpbmVkLCBrZXkpO1xuICB9XG5cbiAgZnVuY3Rpb24gc3RhcnRQb3BTdGF0ZUxpc3RlbmVyKF9yZWYpIHtcbiAgICB2YXIgdHJhbnNpdGlvblRvID0gX3JlZi50cmFuc2l0aW9uVG87XG5cbiAgICBmdW5jdGlvbiBwb3BTdGF0ZUxpc3RlbmVyKGV2ZW50KSB7XG4gICAgICBpZiAoZXZlbnQuc3RhdGUgPT09IHVuZGVmaW5lZCkgcmV0dXJuOyAvLyBJZ25vcmUgZXh0cmFuZW91cyBwb3BzdGF0ZSBldmVudHMgaW4gV2ViS2l0LlxuXG4gICAgICB0cmFuc2l0aW9uVG8oZ2V0Q3VycmVudExvY2F0aW9uKGV2ZW50LnN0YXRlKSk7XG4gICAgfVxuXG4gICAgX0RPTVV0aWxzLmFkZEV2ZW50TGlzdGVuZXIod2luZG93LCAncG9wc3RhdGUnLCBwb3BTdGF0ZUxpc3RlbmVyKTtcblxuICAgIHJldHVybiBmdW5jdGlvbiAoKSB7XG4gICAgICBfRE9NVXRpbHMucmVtb3ZlRXZlbnRMaXN0ZW5lcih3aW5kb3csICdwb3BzdGF0ZScsIHBvcFN0YXRlTGlzdGVuZXIpO1xuICAgIH07XG4gIH1cblxuICBmdW5jdGlvbiBmaW5pc2hUcmFuc2l0aW9uKGxvY2F0aW9uKSB7XG4gICAgdmFyIGJhc2VuYW1lID0gbG9jYXRpb24uYmFzZW5hbWU7XG4gICAgdmFyIHBhdGhuYW1lID0gbG9jYXRpb24ucGF0aG5hbWU7XG4gICAgdmFyIHNlYXJjaCA9IGxvY2F0aW9uLnNlYXJjaDtcbiAgICB2YXIgaGFzaCA9IGxvY2F0aW9uLmhhc2g7XG4gICAgdmFyIHN0YXRlID0gbG9jYXRpb24uc3RhdGU7XG4gICAgdmFyIGFjdGlvbiA9IGxvY2F0aW9uLmFjdGlvbjtcbiAgICB2YXIga2V5ID0gbG9jYXRpb24ua2V5O1xuXG4gICAgaWYgKGFjdGlvbiA9PT0gX0FjdGlvbnMuUE9QKSByZXR1cm47IC8vIE5vdGhpbmcgdG8gZG8uXG5cbiAgICBfRE9NU3RhdGVTdG9yYWdlLnNhdmVTdGF0ZShrZXksIHN0YXRlKTtcblxuICAgIHZhciBwYXRoID0gKGJhc2VuYW1lIHx8ICcnKSArIHBhdGhuYW1lICsgc2VhcmNoICsgaGFzaDtcbiAgICB2YXIgaGlzdG9yeVN0YXRlID0ge1xuICAgICAga2V5OiBrZXlcbiAgICB9O1xuXG4gICAgaWYgKGFjdGlvbiA9PT0gX0FjdGlvbnMuUFVTSCkge1xuICAgICAgaWYgKHVzZVJlZnJlc2gpIHtcbiAgICAgICAgd2luZG93LmxvY2F0aW9uLmhyZWYgPSBwYXRoO1xuICAgICAgICByZXR1cm4gZmFsc2U7IC8vIFByZXZlbnQgbG9jYXRpb24gdXBkYXRlLlxuICAgICAgfSBlbHNlIHtcbiAgICAgICAgICB3aW5kb3cuaGlzdG9yeS5wdXNoU3RhdGUoaGlzdG9yeVN0YXRlLCBudWxsLCBwYXRoKTtcbiAgICAgICAgfVxuICAgIH0gZWxzZSB7XG4gICAgICAvLyBSRVBMQUNFXG4gICAgICBpZiAodXNlUmVmcmVzaCkge1xuICAgICAgICB3aW5kb3cubG9jYXRpb24ucmVwbGFjZShwYXRoKTtcbiAgICAgICAgcmV0dXJuIGZhbHNlOyAvLyBQcmV2ZW50IGxvY2F0aW9uIHVwZGF0ZS5cbiAgICAgIH0gZWxzZSB7XG4gICAgICAgICAgd2luZG93Lmhpc3RvcnkucmVwbGFjZVN0YXRlKGhpc3RvcnlTdGF0ZSwgbnVsbCwgcGF0aCk7XG4gICAgICAgIH1cbiAgICB9XG4gIH1cblxuICB2YXIgaGlzdG9yeSA9IF9jcmVhdGVET01IaXN0b3J5MlsnZGVmYXVsdCddKF9leHRlbmRzKHt9LCBvcHRpb25zLCB7XG4gICAgZ2V0Q3VycmVudExvY2F0aW9uOiBnZXRDdXJyZW50TG9jYXRpb24sXG4gICAgZmluaXNoVHJhbnNpdGlvbjogZmluaXNoVHJhbnNpdGlvbixcbiAgICBzYXZlU3RhdGU6IF9ET01TdGF0ZVN0b3JhZ2Uuc2F2ZVN0YXRlXG4gIH0pKTtcblxuICB2YXIgbGlzdGVuZXJDb3VudCA9IDAsXG4gICAgICBzdG9wUG9wU3RhdGVMaXN0ZW5lciA9IHVuZGVmaW5lZDtcblxuICBmdW5jdGlvbiBsaXN0ZW5CZWZvcmUobGlzdGVuZXIpIHtcbiAgICBpZiAoKytsaXN0ZW5lckNvdW50ID09PSAxKSBzdG9wUG9wU3RhdGVMaXN0ZW5lciA9IHN0YXJ0UG9wU3RhdGVMaXN0ZW5lcihoaXN0b3J5KTtcblxuICAgIHZhciB1bmxpc3RlbiA9IGhpc3RvcnkubGlzdGVuQmVmb3JlKGxpc3RlbmVyKTtcblxuICAgIHJldHVybiBmdW5jdGlvbiAoKSB7XG4gICAgICB1bmxpc3RlbigpO1xuXG4gICAgICBpZiAoLS1saXN0ZW5lckNvdW50ID09PSAwKSBzdG9wUG9wU3RhdGVMaXN0ZW5lcigpO1xuICAgIH07XG4gIH1cblxuICBmdW5jdGlvbiBsaXN0ZW4obGlzdGVuZXIpIHtcbiAgICBpZiAoKytsaXN0ZW5lckNvdW50ID09PSAxKSBzdG9wUG9wU3RhdGVMaXN0ZW5lciA9IHN0YXJ0UG9wU3RhdGVMaXN0ZW5lcihoaXN0b3J5KTtcblxuICAgIHZhciB1bmxpc3RlbiA9IGhpc3RvcnkubGlzdGVuKGxpc3RlbmVyKTtcblxuICAgIHJldHVybiBmdW5jdGlvbiAoKSB7XG4gICAgICB1bmxpc3RlbigpO1xuXG4gICAgICBpZiAoLS1saXN0ZW5lckNvdW50ID09PSAwKSBzdG9wUG9wU3RhdGVMaXN0ZW5lcigpO1xuICAgIH07XG4gIH1cblxuICAvLyBkZXByZWNhdGVkXG4gIGZ1bmN0aW9uIHJlZ2lzdGVyVHJhbnNpdGlvbkhvb2soaG9vaykge1xuICAgIGlmICgrK2xpc3RlbmVyQ291bnQgPT09IDEpIHN0b3BQb3BTdGF0ZUxpc3RlbmVyID0gc3RhcnRQb3BTdGF0ZUxpc3RlbmVyKGhpc3RvcnkpO1xuXG4gICAgaGlzdG9yeS5yZWdpc3RlclRyYW5zaXRpb25Ib29rKGhvb2spO1xuICB9XG5cbiAgLy8gZGVwcmVjYXRlZFxuICBmdW5jdGlvbiB1bnJlZ2lzdGVyVHJhbnNpdGlvbkhvb2soaG9vaykge1xuICAgIGhpc3RvcnkudW5yZWdpc3RlclRyYW5zaXRpb25Ib29rKGhvb2spO1xuXG4gICAgaWYgKC0tbGlzdGVuZXJDb3VudCA9PT0gMCkgc3RvcFBvcFN0YXRlTGlzdGVuZXIoKTtcbiAgfVxuXG4gIHJldHVybiBfZXh0ZW5kcyh7fSwgaGlzdG9yeSwge1xuICAgIGxpc3RlbkJlZm9yZTogbGlzdGVuQmVmb3JlLFxuICAgIGxpc3RlbjogbGlzdGVuLFxuICAgIHJlZ2lzdGVyVHJhbnNpdGlvbkhvb2s6IHJlZ2lzdGVyVHJhbnNpdGlvbkhvb2ssXG4gICAgdW5yZWdpc3RlclRyYW5zaXRpb25Ib29rOiB1bnJlZ2lzdGVyVHJhbnNpdGlvbkhvb2tcbiAgfSk7XG59XG5cbmV4cG9ydHNbJ2RlZmF1bHQnXSA9IGNyZWF0ZUJyb3dzZXJIaXN0b3J5O1xubW9kdWxlLmV4cG9ydHMgPSBleHBvcnRzWydkZWZhdWx0J107XG5cblxuLyoqKioqKioqKioqKioqKioqXG4gKiogV0VCUEFDSyBGT09URVJcbiAqKiAuL34vaGlzdG9yeS9saWIvY3JlYXRlQnJvd3Nlckhpc3RvcnkuanNcbiAqKiBtb2R1bGUgaWQgPSAxMzZcbiAqKiBtb2R1bGUgY2h1bmtzID0gMVxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgTmF2TGVmdEJhciA9IHJlcXVpcmUoJy4vbmF2TGVmdEJhcicpO1xuXG52YXIgQXBwID0gUmVhY3QuY3JlYXRlQ2xhc3Moe1xuICByZW5kZXI6IGZ1bmN0aW9uKCl7XG4gICAgcmV0dXJuIChcbiAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZ3J2XCI+XG4gICAgICAgIDxOYXZMZWZ0QmFyLz5cbiAgICAgICAgPGRpdiBzdHlsZT17eydtYXJnaW5MZWZ0JzogJzEwMHB4J319PlxuICAgICAgICAgIHt0aGlzLnByb3BzLmNoaWxkcmVufVxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pXG5cbm1vZHVsZS5leHBvcnRzID0gQXBwO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvYXBwLmpzeFxuICoqLyIsInZhciBSZWFjdCA9IHJlcXVpcmUoJ3JlYWN0Jyk7XG52YXIgJCA9IHJlcXVpcmUoJ2pRdWVyeScpO1xudmFyIHJlYWN0b3IgPSByZXF1aXJlKCdhcHAvcmVhY3RvcicpO1xuXG52YXIgTG9naW4gPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgbWl4aW5zOiBbcmVhY3Rvci5SZWFjdE1peGluXSxcblxuICBnZXREYXRhQmluZGluZ3MoKSB7XG4gICAgcmV0dXJuIHtcbiAgLy8gICAgdXNlclJlcXVlc3Q6IGdldHRlcnMudXNlclJlcXVlc3RcbiAgICB9XG4gIH0sXG5cbiAgaGFuZGxlU3VibWl0OiBmdW5jdGlvbihlKXtcbiAgICBlLnByZXZlbnREZWZhdWx0KCk7XG4gICAgaWYodGhpcy5pc1ZhbGlkKCkpe1xuICAgICAgdmFyIGxvYyA9IHRoaXMucHJvcHMubG9jYXRpb247XG4gICAgICB2YXIgZW1haWwgPSB0aGlzLnJlZnMuZW1haWwudmFsdWU7XG4gICAgICB2YXIgcGFzcyA9IHRoaXMucmVmcy5wYXNzLnZhbHVlO1xuICAgICAgdmFyIHJlZGlyZWN0ID0gJy93ZWInO1xuXG4gICAgICBpZihsb2Muc3RhdGUgJiYgbG9jLnN0YXRlLnJlZGlyZWN0VG8pe1xuICAgICAgICByZWRpcmVjdCA9IGxvYy5zdGF0ZS5yZWRpcmVjdFRvO1xuICAgICAgfVxuXG4gICAgICAvL2FjdGlvbnMubG9naW4oZW1haWwsIHBhc3MsIHJlZGlyZWN0KTtcbiAgICB9XG4gIH0sXG5cbiAgaXNWYWxpZDogZnVuY3Rpb24oKXtcbiAgICAgdmFyICRmb3JtID0gJChcIi5sb2dpbnNjcmVlbiBmb3JtXCIpO1xuICAgICByZXR1cm4gJGZvcm0ubGVuZ3RoID09PSAwIHx8ICRmb3JtLnZhbGlkKCk7XG4gIH0sXG5cbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICB2YXIgaXNQcm9jZXNzaW5nID0gZmFsc2U7Ly90aGlzLnN0YXRlLnVzZXJSZXF1ZXN0LmdldCgnaXNMb2FkaW5nJyk7XG4gICAgdmFyIGlzRXJyb3IgPSBmYWxzZTsvL3RoaXMuc3RhdGUudXNlclJlcXVlc3QuZ2V0KCdpc0Vycm9yJyk7XG5cbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJtaWRkbGUtYm94IHRleHQtY2VudGVyIGxvZ2luc2NyZWVuXCI+XG4gICAgICAgIDxmb3JtPlxuICAgICAgICAgIDxkaXY+XG4gICAgICAgICAgICA8aDEgY2xhc3NOYW1lPVwibG9nby1uYW1lXCI+RzwvaDE+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPGgzPiBXZWxjb21lIHRvIEdyYXZpdGF0aW9uYWw8L2gzPlxuICAgICAgICAgIDxwPiBMb2dpbiBpbi48L3A+XG4gICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJtLXRcIiByb2xlPVwiZm9ybVwiIG9uU3VibWl0PXt0aGlzLmhhbmRsZVN1Ym1pdH0+XG4gICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImZvcm0tZ3JvdXBcIj5cbiAgICAgICAgICAgICAgPGlucHV0IHR5cGU9XCJlbWFpbFwiIHJlZj1cImVtYWlsXCIgbmFtZT1cImVtYWlsXCIgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sIHJlcXVpcmVkXCIgcGxhY2Vob2xkZXI9XCJVc2VybmFtZVwiIHJlcXVpcmVkIC8+XG4gICAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgICA8aW5wdXQgdHlwZT1cInBhc3N3b3JkXCIgcmVmPVwicGFzc1wiIG5hbWU9XCJwYXNzd29yZFwiIGNsYXNzTmFtZT1cImZvcm0tY29udHJvbCByZXF1aXJlZFwiIHBsYWNlaG9sZGVyPVwiUGFzc3dvcmRcIiByZXF1aXJlZCAvPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgICA8YnV0dG9uIHR5cGU9XCJzdWJtaXRcIiBvbkNsaWNrPSB7dGhpcy5oYW5kbGVTdWJtaXR9IGNsYXNzTmFtZT1cImJ0biBidG4tcHJpbWFyeSBibG9jayBmdWxsLXdpZHRoIG0tYlwiIGRpc2FibGVkPXtpc1Byb2Nlc3Npbmd9PkxvZ2luPC9idXR0b24+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgIDwvZm9ybT5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IExvZ2luO1xuXG5cblxuLyoqIFdFQlBBQ0sgRk9PVEVSICoqXG4gKiogLi9zcmMvYXBwL2NvbXBvbmVudHMvbG9naW4uanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciBSb3V0ZXIgPSByZXF1aXJlKCdyZWFjdC1yb3V0ZXInKTtcbnZhciBJbmRleExpbmsgPSBSb3V0ZXIuSW5kZXhMaW5rO1xudmFyIEhpc3RvcnkgPSByZXF1aXJlKCdyZWFjdC1yb3V0ZXInKS5IaXN0b3J5O1xuXG52YXIgbWVudUl0ZW1zID0gWyAgXG4gIHtpY29uOiBcImZhIGZhIGZhLXNpdGVtYXBcIiwgdG86IGAvd2ViL25vZGVzYCwgdGl0bGU6IFwiTm9kZXNcIn0sXG4gIHtpY29uOiBcImZhIGZhLWhkZC1vXCIsIHRvOiBgL3dlYi9zZXNzaW9uc2AsIHRpdGxlOiBcIlNlc3Npb25zXCJ9XG5dO1xuXG52YXIgTmF2TGVmdEJhciA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcblxuICBtaXhpbnM6IFsgSGlzdG9yeSBdLFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKXtcbiAgICB2YXIgc2VsZiA9IHRoaXM7XG4gICAgdmFyIGl0ZW1zID0gbWVudUl0ZW1zLm1hcChmdW5jdGlvbihpLCBpbmRleCl7XG4gICAgICB2YXIgY2xhc3NOYW1lID0gc2VsZi5oaXN0b3J5LmlzQWN0aXZlKGkudG8pID8gXCJhY3RpdmVcIiA6IFwiXCI7XG4gICAgICByZXR1cm4gKFxuICAgICAgICA8bGkga2V5PXtpbmRleH0gY2xhc3NOYW1lPXtjbGFzc05hbWV9PlxuICAgICAgICAgIDxJbmRleExpbmsgdG89e2kudG99PlxuICAgICAgICAgICAgPGkgY2xhc3NOYW1lPXtpLmljb259IHRpdGxlPXtpLnRpdGxlfS8+XG4gICAgICAgICAgPC9JbmRleExpbms+XG4gICAgICAgIDwvbGk+XG4gICAgICApO1xuICAgIH0pO1xuXG4gICAgcmV0dXJuIChcbiAgICAgIDxuYXYgY2xhc3NOYW1lPVwiXCIgcm9sZT1cIm5hdmlnYXRpb25cIiBzdHlsZT17e3dpZHRoOiAnNjBweCcsIGZsb2F0OiAnbGVmdCd9fT5cbiAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJcIj5cbiAgICAgICAgICA8dWwgY2xhc3NOYW1lPVwibmF2IDFtZXRpc21lbnVcIiBpZD1cInNpZGUtbWVudVwiPlxuICAgICAgICAgICAge2l0ZW1zfVxuICAgICAgICAgIDwvdWw+XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9uYXY+XG4gICAgKTtcbiAgfVxufSk7XG5cbm1vZHVsZS5leHBvcnRzID0gTmF2TGVmdEJhcjtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL25hdkxlZnRCYXIuanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciAkID0gcmVxdWlyZSgnalF1ZXJ5Jyk7XG52YXIgcmVhY3RvciA9IHJlcXVpcmUoJ2FwcC9yZWFjdG9yJyk7XG5cbnZhciBJbnZpdGUgPSBSZWFjdC5jcmVhdGVDbGFzcyh7XG5cbiAgaGFuZGxlU3VibWl0OiBmdW5jdGlvbihlKSB7XG4gICAgZS5wcmV2ZW50RGVmYXVsdCgpO1xuICAgIGlmICh0aGlzLmlzVmFsaWQoKSkge1xuICAgICAgdmFyIGxvYyA9IHRoaXMucHJvcHMubG9jYXRpb247XG4gICAgICB2YXIgZW1haWwgPSB0aGlzLnJlZnMuZW1haWwudmFsdWU7XG4gICAgICB2YXIgcGFzcyA9IHRoaXMucmVmcy5wYXNzLnZhbHVlO1xuICAgICAgdmFyIHJlZGlyZWN0ID0gJy93ZWInO1xuXG4gICAgICBpZiAobG9jLnN0YXRlICYmIGxvYy5zdGF0ZS5yZWRpcmVjdFRvKSB7XG4gICAgICAgIHJlZGlyZWN0ID0gbG9jLnN0YXRlLnJlZGlyZWN0VG87XG4gICAgICB9XG5cbiAgICAgIC8vYWN0aW9ucy5sb2dpbihlbWFpbCwgcGFzcywgcmVkaXJlY3QpO1xuICAgIH1cbiAgfSxcblxuICBpc1ZhbGlkOiBmdW5jdGlvbigpIHtcbiAgICB2YXIgJGZvcm0gPSAkKFwiLmxvZ2luc2NyZWVuIGZvcm1cIik7XG4gICAgcmV0dXJuICRmb3JtLmxlbmd0aCA9PT0gMCB8fCAkZm9ybS52YWxpZCgpO1xuICB9LFxuXG4gIHJlbmRlcjogZnVuY3Rpb24oKSB7XG4gICAgdmFyIGlzUHJvY2Vzc2luZyA9IGZhbHNlOyAvL3RoaXMuc3RhdGUudXNlclJlcXVlc3QuZ2V0KCdpc0xvYWRpbmcnKTtcbiAgICB2YXIgaXNFcnJvciA9IGZhbHNlOyAvL3RoaXMuc3RhdGUudXNlclJlcXVlc3QuZ2V0KCdpc0Vycm9yJyk7XG5cbiAgICByZXR1cm4gKFxuICAgICAgPGRpdiBjbGFzc05hbWU9XCJtaWRkbGUtYm94IHRleHQtY2VudGVyIGxvZ2luc2NyZWVuICBhbmltYXRlZCBmYWRlSW5Eb3duXCI+XG4gICAgICAgIDxkaXY+XG4gICAgICAgICAgPGRpdj5cbiAgICAgICAgICAgIDxoMSBjbGFzc05hbWU9XCJsb2dvLW5hbWVcIj5HPC9oMT5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8aDM+V2VsY29tZSB0byBHcmF2aXR5PC9oMz5cbiAgICAgICAgICA8cD5DcmVhdGUgcGFzc3dvcmQuPC9wPlxuICAgICAgICAgIDxkaXYgYWxpZ249XCJsZWZ0XCI+XG4gICAgICAgICAgICA8Zm9udCBjb2xvcj1cIndoaXRlXCI+XG4gICAgICAgICAgICAgIDEpIENyZWF0ZSBhbmQgZW50ZXIgYSBuZXcgcGFzc3dvcmRcbiAgICAgICAgICAgICAgPGJyPjwvYnI+XG4gICAgICAgICAgICAgIDIpIEluc3RhbGwgR29vZ2xlIEF1dGhlbnRpY2F0b3Igb24geW91ciBzbWFydHBob25lXG4gICAgICAgICAgICAgIDxicj48L2JyPlxuICAgICAgICAgICAgICAzKSBPcGVuIEdvb2dsZSBBdXRoZW50aWNhdG9yIGFuZCBjcmVhdGUgYSBuZXcgYWNjb3VudCB1c2luZyBwcm92aWRlZCBiYXJjb2RlXG4gICAgICAgICAgICAgIDxicj48L2JyPlxuICAgICAgICAgICAgICA0KSBHZW5lcmF0ZSBBdXRoZW50aWNhdG9yIHRva2VuIGFuZCBlbnRlciBpdCBiZWxvd1xuICAgICAgICAgICAgICA8YnI+PC9icj5cbiAgICAgICAgICAgIDwvZm9udD5cbiAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICA8Zm9ybSBjbGFzc05hbWU9XCJtLXRcIiByb2xlPVwiZm9ybVwiIGFjdGlvbj1cIi93ZWIvZmluaXNobmV3dXNlclwiIG1ldGhvZD1cIlBPU1RcIj5cbiAgICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgICA8aW5wdXQgdHlwZT1cImhpZGRlblwiIG5hbWU9XCJ0b2tlblwiIGNsYXNzTmFtZT1cImZvcm0tY29udHJvbFwiLz5cbiAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICAgIDxpbnB1dCB0eXBlPVwidGVzdFwiIG5hbWU9XCJ1c2VybmFtZVwiIGRpc2FibGVkIGNsYXNzTmFtZT1cImZvcm0tY29udHJvbFwiIHBsYWNlaG9sZGVyPVwiVXNlcm5hbWVcIiByZXF1aXJlZD1cIlwiLz5cbiAgICAgICAgICAgIDwvZGl2PlxuICAgICAgICAgICAgPGRpdiBjbGFzc05hbWU9XCJmb3JtLWdyb3VwXCI+XG4gICAgICAgICAgICAgIDxpbnB1dCB0eXBlPVwicGFzc3dvcmRcIiBuYW1lPVwicGFzc3dvcmRcIiBpZD1cInBhc3N3b3JkXCIgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sXCIgcGxhY2Vob2xkZXI9XCJQYXNzd29yZFwiIHJlcXVpcmVkPVwiXCIgb25jaGFuZ2U9XCJjaGVja1Bhc3N3b3JkcygpXCIvPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cImZvcm0tZ3JvdXBcIj5cbiAgICAgICAgICAgICAgPGlucHV0IHR5cGU9XCJwYXNzd29yZFwiIG5hbWU9XCJwYXNzd29yZF9jb25maXJtXCIgaWQ9XCJwYXNzd29yZF9jb25maXJtXCIgY2xhc3NOYW1lPVwiZm9ybS1jb250cm9sXCIgcGxhY2Vob2xkZXI9XCJDb25maXJtIHBhc3N3b3JkXCIgcmVxdWlyZWQ9XCJcIiBvbmNoYW5nZT1cImNoZWNrUGFzc3dvcmRzKClcIi8+XG4gICAgICAgICAgICA8L2Rpdj5cbiAgICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiZm9ybS1ncm91cFwiPlxuICAgICAgICAgICAgICA8aW5wdXQgdHlwZT1cInRlc3RcIiBuYW1lPVwiaG90cF90b2tlblwiIGlkPVwiaG90cF90b2tlblwiIGNsYXNzTmFtZT1cImZvcm0tY29udHJvbFwiIHBsYWNlaG9sZGVyPVwiaG90cCB0b2tlblwiIHJlcXVpcmVkPVwiXCIvPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgICA8YnV0dG9uIHR5cGU9XCJzdWJtaXRcIiBjbGFzc05hbWU9XCJidG4gYnRuLXByaW1hcnkgYmxvY2sgZnVsbC13aWR0aCBtLWJcIj5Db25maXJtPC9idXR0b24+XG4gICAgICAgICAgPC9mb3JtPlxuICAgICAgICA8L2Rpdj5cbiAgICAgIDwvZGl2PlxuICAgICk7XG4gIH1cbn0pO1xuXG5tb2R1bGUuZXhwb3J0cyA9IEludml0ZTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL25ld1VzZXIuanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciBOb2RlcyA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICByZXR1cm4gKFxuICAgICAgPGRpdj5cbiAgICAgICAgPGgxPiBOb2RlcyA8L2gxPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiXCI+XG4gICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgICAgICA8dGFibGUgY2xhc3NOYW1lPVwidGFibGUgdGFibGUtc3RyaXBlZFwiPlxuICAgICAgICAgICAgICAgIDx0aGVhZD5cbiAgICAgICAgICAgICAgICAgIDx0cj5cbiAgICAgICAgICAgICAgICAgICAgPHRoPk5vZGU8L3RoPlxuICAgICAgICAgICAgICAgICAgICA8dGg+U3RhdHVzPC90aD5cbiAgICAgICAgICAgICAgICAgICAgPHRoPkxhYmVsczwvdGg+XG4gICAgICAgICAgICAgICAgICAgICAgPHRoPkNQVTwvdGg+XG4gICAgICAgICAgICAgICAgICAgICAgPHRoPlJBTTwvdGg+XG4gICAgICAgICAgICAgICAgICAgICAgPHRoPk9TPC90aD5cbiAgICAgICAgICAgICAgICAgICAgICA8dGg+IExhc3QgSGVhcnRiZWF0IDwvdGg+XG4gICAgICAgICAgICAgICAgICAgIDwvdHI+XG4gICAgICAgICAgICAgICAgICA8L3RoZWFkPlxuICAgICAgICAgICAgICAgIDx0Ym9keT48L3Rib2R5PlxuICAgICAgICAgICAgICA8L3RhYmxlPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9kaXY+XG4gICAgKVxuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBOb2RlcztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL25vZGVzL21haW4uanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciByZWFjdG9yID0gcmVxdWlyZSgnYXBwL3JlYWN0b3InKTtcbnZhciBOb2RlcyA9IFJlYWN0LmNyZWF0ZUNsYXNzKHtcbiAgcmVuZGVyOiBmdW5jdGlvbigpIHtcbiAgICByZXR1cm4gKFxuICAgICAgPGRpdj5cbiAgICAgICAgPGgxPiBTZXNzaW9ucyA8L2gxPlxuICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgIDxkaXYgY2xhc3NOYW1lPVwiXCI+XG4gICAgICAgICAgICA8ZGl2IGNsYXNzTmFtZT1cIlwiPlxuICAgICAgICAgICAgICA8dGFibGUgY2xhc3NOYW1lPVwidGFibGUgdGFibGUtc3RyaXBlZFwiPlxuICAgICAgICAgICAgICAgIDx0aGVhZD5cbiAgICAgICAgICAgICAgICAgIDx0cj5cbiAgICAgICAgICAgICAgICAgICAgPHRoPk5vZGU8L3RoPlxuICAgICAgICAgICAgICAgICAgICA8dGg+U3RhdHVzPC90aD5cbiAgICAgICAgICAgICAgICAgICAgPHRoPkxhYmVsczwvdGg+XG4gICAgICAgICAgICAgICAgICAgICAgPHRoPkNQVTwvdGg+XG4gICAgICAgICAgICAgICAgICAgICAgPHRoPlJBTTwvdGg+XG4gICAgICAgICAgICAgICAgICAgICAgPHRoPk9TPC90aD5cbiAgICAgICAgICAgICAgICAgICAgICA8dGg+IExhc3QgSGVhcnRiZWF0IDwvdGg+XG4gICAgICAgICAgICAgICAgICAgIDwvdHI+XG4gICAgICAgICAgICAgICAgICA8L3RoZWFkPlxuICAgICAgICAgICAgICAgIDx0Ym9keT48L3Rib2R5PlxuICAgICAgICAgICAgICA8L3RhYmxlPlxuICAgICAgICAgICAgPC9kaXY+XG4gICAgICAgICAgPC9kaXY+XG4gICAgICAgIDwvZGl2PlxuICAgICAgPC9kaXY+XG4gICAgKVxuICB9XG59KTtcblxubW9kdWxlLmV4cG9ydHMgPSBOb2RlcztcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9jb21wb25lbnRzL3Nlc3Npb25zL21haW4uanN4XG4gKiovIiwidmFyIFJlYWN0ID0gcmVxdWlyZSgncmVhY3QnKTtcbnZhciB7IFJvdXRlciwgUm91dGUsIFJlZGlyZWN0LCBJbmRleFJvdXRlIH0gPSByZXF1aXJlKCdyZWFjdC1yb3V0ZXInKTtcbnZhciByZW5kZXIgPSByZXF1aXJlKCdyZWFjdC1kb20nKS5yZW5kZXI7XG5cbi8vIHJvdXRlIGNvbXBvbmVudCBtb2R1bGVzXG52YXIgQXBwID0gcmVxdWlyZSgnLi9jb21wb25lbnRzL2FwcC5qc3gnKTtcbnZhciBMb2dpbiA9IHJlcXVpcmUoJy4vY29tcG9uZW50cy9sb2dpbi5qc3gnKTtcbnZhciBOb2RlcyA9IHJlcXVpcmUoJy4vY29tcG9uZW50cy9ub2Rlcy9tYWluLmpzeCcpO1xudmFyIFNlc3Npb25zID0gcmVxdWlyZSgnLi9jb21wb25lbnRzL3Nlc3Npb25zL21haW4uanN4Jyk7XG52YXIgTmV3VXNlciA9IHJlcXVpcmUoJy4vY29tcG9uZW50cy9uZXdVc2VyLmpzeCcpO1xudmFyIGF1dGggPSByZXF1aXJlKCcuL2F1dGgnKTtcbnZhciBzZXNzaW9uID0gcmVxdWlyZSgnLi9zZXNzaW9uJyk7XG5cbi8vIGluaXQgc2Vzc2lvblxuc2Vzc2lvbi5pbml0KCk7XG5cbmZ1bmN0aW9uIHJlcXVpcmVBdXRoKG5leHRTdGF0ZSwgcmVwbGFjZSwgY2IpIHtcbiAgYXV0aC5lbnN1cmVVc2VyKClcbiAgICAuZG9uZSgoKT0+IGNiKCkpXG4gICAgLmZhaWwoKCk9PntcbiAgICAgIHJlcGxhY2Uoe3JlZGlyZWN0VG86IG5leHRTdGF0ZS5sb2NhdGlvbi5wYXRobmFtZSB9LCAnL3dlYi9sb2dpbicgKTtcbiAgICAgIGNiKCk7XG4gICAgfSk7XG59XG5cbmZ1bmN0aW9uIGhhbmRsZUxvZ291dChuZXh0U3RhdGUsIHJlcGxhY2UsIGNiKXtcbiAgYXV0aC5sb2dvdXQoKTtcbiAgLy8gZ29pbmcgYmFjayB3aWxsIGhpdCByZXF1aXJlQXV0aCBoYW5kbGVyIHdoaWNoIHdpbGwgcmVkaXJlY3QgaXQgdG8gdGhlIGxvZ2luIHBhZ2VcbiAgc2Vzc2lvbi5nZXRIaXN0b3J5KCkuZ29CYWNrKCk7XG59XG5cbnJlbmRlcigoXG4gIDxSb3V0ZXIgaGlzdG9yeT17c2Vzc2lvbi5nZXRIaXN0b3J5KCl9PlxuICAgIDxSb3V0ZSBwYXRoPVwiL3dlYi9sb2dpblwiIGNvbXBvbmVudD17TG9naW59Lz5cbiAgICA8Um91dGUgcGF0aD1cIi93ZWIvbG9nb3V0XCIgb25FbnRlcj17aGFuZGxlTG9nb3V0fS8+XG4gICAgPFJvdXRlIHBhdGg9XCIvd2ViL25ld3VzZXJcIiBjb21wb25lbnQ9e05ld1VzZXJ9Lz5cbiAgICA8Um91dGUgcGF0aD1cIi93ZWJcIiBjb21wb25lbnQ9e0FwcH0+XG4gICAgICA8Um91dGUgcGF0aD1cIm5vZGVzXCIgY29tcG9uZW50PXtOb2Rlc30vPlxuICAgICAgPFJvdXRlIHBhdGg9XCJzZXNzaW9uc1wiIGNvbXBvbmVudD17U2Vzc2lvbnN9Lz5cbiAgICA8L1JvdXRlPlxuICA8L1JvdXRlcj5cbiksIGRvY3VtZW50LmdldEVsZW1lbnRCeUlkKFwiYXBwXCIpKTtcblxuXG5cbi8qKiBXRUJQQUNLIEZPT1RFUiAqKlxuICoqIC4vc3JjL2FwcC9pbmRleC5qc3hcbiAqKi8iXSwic291cmNlUm9vdCI6IiJ9