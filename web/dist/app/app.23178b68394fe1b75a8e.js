webpackJsonp([0],[
/* 0 */
/***/ (function(module, exports, __webpack_require__) {

	module.exports = __webpack_require__(1);


/***/ }),
/* 1 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _reactDom = __webpack_require__(31);

	var _reactRouter = __webpack_require__(164);

	var _nuclearJsReactAddons = __webpack_require__(219);

	var _history = __webpack_require__(226);

	var _history2 = _interopRequireDefault(_history);

	var _config = __webpack_require__(228);

	var _config2 = _interopRequireDefault(_config);

	var _reactor = __webpack_require__(233);

	var _reactor2 = _interopRequireDefault(_reactor);

	var _routes = __webpack_require__(235);

	var _features = __webpack_require__(268);

	var Features = _interopRequireWildcard(_features);

	var _settings = __webpack_require__(525);

	var _featureActivator = __webpack_require__(519);

	var _featureActivator2 = _interopRequireDefault(_featureActivator);

	var _actions = __webpack_require__(284);

	var _app = __webpack_require__(530);

	var _app2 = _interopRequireDefault(_app);

	var _actions2 = __webpack_require__(239);

	__webpack_require__(534);

	function _interopRequireWildcard(obj) { if (obj && obj.__esModule) { return obj; } else { var newObj = {}; if (obj != null) { for (var key in obj) { if (Object.prototype.hasOwnProperty.call(obj, key)) newObj[key] = obj[key]; } } newObj.default = obj; return newObj; } }

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	_config2.default.init(window.GRV_CONFIG); /*
	                                          Copyright 2015 Gravitational, Inc.
	                                          
	                                          Licensed under the Apache License, Version 2.0 (the "License");
	                                          you may not use this file except in compliance with the License.
	                                          You may obtain a copy of the License at
	                                          
	                                              http://www.apache.org/licenses/LICENSE-2.0
	                                          
	                                          Unless required by applicable law or agreed to in writing, software
	                                          distributed under the License is distributed on an "AS IS" BASIS,
	                                          WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                          See the License for the specific language governing permissions and
	                                          limitations under the License.
	                                          */

	_history2.default.init();

	var featureRoutes = [];
	var featureActivator = new _featureActivator2.default();

	featureActivator.register(new Features.Ssh(featureRoutes));
	featureActivator.register(new Features.Audit(featureRoutes));
	featureActivator.register((0, _settings.createSettings)(featureRoutes));

	var onEnterApp = function onEnterApp(nextState) {
	  var siteId = nextState.params.siteId;

	  (0, _actions.initApp)(siteId, featureActivator);
	};

	var routes = [{
	  path: _config2.default.routes.app,
	  onEnter: _actions2.ensureUser,
	  component: _app2.default,
	  childRoutes: [{
	    onEnter: onEnterApp,
	    childRoutes: featureRoutes
	  }]
	}];

	(0, _reactDom.render)(_react2.default.createElement(
	  _nuclearJsReactAddons.Provider,
	  { reactor: _reactor2.default },
	  _react2.default.createElement(_reactRouter.Router, { history: _history2.default.original(), routes: (0, _routes.addRoutes)(routes) })
	), document.getElementById("app"));

/***/ }),
/* 2 */,
/* 3 */,
/* 4 */,
/* 5 */,
/* 6 */,
/* 7 */,
/* 8 */,
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
/* 22 */,
/* 23 */,
/* 24 */,
/* 25 */,
/* 26 */,
/* 27 */,
/* 28 */,
/* 29 */,
/* 30 */,
/* 31 */,
/* 32 */,
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
/* 43 */,
/* 44 */,
/* 45 */,
/* 46 */,
/* 47 */,
/* 48 */,
/* 49 */,
/* 50 */,
/* 51 */,
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
/* 68 */,
/* 69 */,
/* 70 */,
/* 71 */,
/* 72 */,
/* 73 */,
/* 74 */,
/* 75 */,
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
/* 99 */,
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
/* 212 */,
/* 213 */,
/* 214 */,
/* 215 */,
/* 216 */,
/* 217 */,
/* 218 */,
/* 219 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

	var _connect = __webpack_require__(220);

	var _connect2 = _interopRequireDefault(_connect);

	var _Provider = __webpack_require__(222);

	var _Provider2 = _interopRequireDefault(_Provider);

	var _nuclearMixin = __webpack_require__(223);

	var _nuclearMixin2 = _interopRequireDefault(_nuclearMixin);

	var _provideReactor = __webpack_require__(224);

	var _provideReactor2 = _interopRequireDefault(_provideReactor);

	var _nuclearComponent = __webpack_require__(225);

	var _nuclearComponent2 = _interopRequireDefault(_nuclearComponent);

	exports.connect = _connect2['default'];
	exports.Provider = _Provider2['default'];
	exports.nuclearMixin = _nuclearMixin2['default'];
	exports.provideReactor = _provideReactor2['default'];
	exports.nuclearComponent = _nuclearComponent2['default'];

/***/ }),
/* 220 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

	exports['default'] = connect;

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError('Cannot call a class as a function'); } }

	function _inherits(subClass, superClass) { if (typeof superClass !== 'function' && superClass !== null) { throw new TypeError('Super expression must either be null or a function, not ' + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

	var _react = __webpack_require__(2);

	var _reactorShape = __webpack_require__(221);

	var _reactorShape2 = _interopRequireDefault(_reactorShape);

	var _hoistNonReactStatics = __webpack_require__(188);

	var _hoistNonReactStatics2 = _interopRequireDefault(_hoistNonReactStatics);

	function getDisplayName(WrappedComponent) {
	  return WrappedComponent.displayName || WrappedComponent.name || 'Component';
	}

	function connect(mapStateToProps) {
	  return function wrapWithConnect(WrappedComponent) {
	    var Connect = (function (_Component) {
	      _inherits(Connect, _Component);

	      function Connect(props, context) {
	        _classCallCheck(this, Connect);

	        _Component.call(this, props, context);
	        this.reactor = props.reactor || context.reactor;
	        this.unsubscribeFns = [];
	        this.updatePropMap(props);
	      }

	      Connect.prototype.resubscribe = function resubscribe(props) {
	        this.unsubscribe();
	        this.updatePropMap(props);
	        this.updateState();
	        this.subscribe();
	      };

	      Connect.prototype.componentWillMount = function componentWillMount() {
	        this.updateState();
	      };

	      Connect.prototype.componentDidMount = function componentDidMount() {
	        this.subscribe(this.props);
	      };

	      Connect.prototype.componentWillUnmount = function componentWillUnmount() {
	        this.unsubscribe();
	      };

	      Connect.prototype.updatePropMap = function updatePropMap(props) {
	        this.propMap = mapStateToProps ? mapStateToProps(props) : {};
	      };

	      Connect.prototype.updateState = function updateState() {
	        var propMap = this.propMap;
	        var stateToSet = {};

	        for (var key in propMap) {
	          var getter = propMap[key];
	          stateToSet[key] = this.reactor.evaluate(getter);
	        }

	        this.setState(stateToSet);
	      };

	      Connect.prototype.subscribe = function subscribe() {
	        var _this = this;

	        var propMap = this.propMap;

	        var _loop = function (key) {
	          var getter = propMap[key];
	          var unsubscribeFn = _this.reactor.observe(getter, function (val) {
	            var _setState;

	            _this.setState((_setState = {}, _setState[key] = val, _setState));
	          });

	          _this.unsubscribeFns.push(unsubscribeFn);
	        };

	        for (var key in propMap) {
	          _loop(key);
	        }
	      };

	      Connect.prototype.unsubscribe = function unsubscribe() {
	        if (this.unsubscribeFns.length === 0) {
	          return;
	        }

	        while (this.unsubscribeFns.length > 0) {
	          this.unsubscribeFns.shift()();
	        }
	      };

	      Connect.prototype.render = function render() {
	        return _react.createElement(WrappedComponent, _extends({
	          reactor: this.reactor
	        }, this.props, this.state));
	      };

	      return Connect;
	    })(_react.Component);

	    Connect.displayName = 'Connect(' + getDisplayName(WrappedComponent) + ')';
	    Connect.WrappedComponent = WrappedComponent;
	    Connect.contextTypes = {
	      reactor: _reactorShape2['default']
	    };
	    Connect.propTypes = {
	      reactor: _reactorShape2['default']
	    };

	    return _hoistNonReactStatics2['default'](Connect, WrappedComponent);
	  };
	}

	module.exports = exports['default'];

/***/ }),
/* 221 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _react = __webpack_require__(2);

	exports['default'] = _react.PropTypes.shape({
	  dispatch: _react.PropTypes.func.isRequired,
	  evaluate: _react.PropTypes.func.isRequired,
	  evaluateToJS: _react.PropTypes.func.isRequired,
	  observe: _react.PropTypes.func.isRequired
	});
	module.exports = exports['default'];

/***/ }),
/* 222 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError('Cannot call a class as a function'); } }

	function _inherits(subClass, superClass) { if (typeof superClass !== 'function' && superClass !== null) { throw new TypeError('Super expression must either be null or a function, not ' + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

	var _react = __webpack_require__(2);

	var _reactorShape = __webpack_require__(221);

	var _reactorShape2 = _interopRequireDefault(_reactorShape);

	var Provider = (function (_Component) {
	  _inherits(Provider, _Component);

	  Provider.prototype.getChildContext = function getChildContext() {
	    return {
	      reactor: this.reactor
	    };
	  };

	  function Provider(props, context) {
	    _classCallCheck(this, Provider);

	    _Component.call(this, props, context);
	    this.reactor = props.reactor;
	  }

	  Provider.prototype.render = function render() {
	    return _react.Children.only(this.props.children);
	  };

	  return Provider;
	})(_react.Component);

	exports['default'] = Provider;

	Provider.propTypes = {
	  reactor: _reactorShape2['default'].isRequired,
	  children: _react.PropTypes.element.isRequired
	};

	Provider.childContextTypes = {
	  reactor: _reactorShape2['default'].isRequired
	};
	module.exports = exports['default'];

/***/ }),
/* 223 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _react = __webpack_require__(2);

	/**
	 * Iterate on an object
	 */
	function each(obj, fn) {
	  for (var key in obj) {
	    if (obj.hasOwnProperty(key)) {
	      fn(obj[key], key);
	    }
	  }
	}

	/**
	 * Returns a mapping of the getDataBinding keys to
	 * the reactor values
	 */
	function getState(reactor, data) {
	  var state = {};
	  each(data, function (value, key) {
	    state[key] = reactor.evaluate(value);
	  });
	  return state;
	}

	/**
	 * Mixin expecting a context.reactor on the component
	 *
	 * Should be used if a higher level component has been
	 * wrapped with provideReactor
	 * @type {Object}
	 */
	exports['default'] = {
	  contextTypes: {
	    reactor: _react.PropTypes.object.isRequired
	  },

	  getInitialState: function getInitialState() {
	    if (!this.getDataBindings) {
	      return null;
	    }
	    return getState(this.context.reactor, this.getDataBindings());
	  },

	  componentDidMount: function componentDidMount() {
	    if (!this.getDataBindings) {
	      return;
	    }
	    var component = this;
	    component.__nuclearUnwatchFns = [];
	    each(this.getDataBindings(), function (getter, key) {
	      var unwatchFn = component.context.reactor.observe(getter, function (val) {
	        var newState = {};
	        newState[key] = val;
	        component.setState(newState);
	      });

	      component.__nuclearUnwatchFns.push(unwatchFn);
	    });
	  },

	  componentWillUnmount: function componentWillUnmount() {
	    if (!this.__nuclearUnwatchFns) {
	      return;
	    }
	    while (this.__nuclearUnwatchFns.length) {
	      this.__nuclearUnwatchFns.shift()();
	    }
	  }
	};
	module.exports = exports['default'];

/***/ }),
/* 224 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports['default'] = provideReactor;

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _hoistNonReactStatics = __webpack_require__(188);

	var _hoistNonReactStatics2 = _interopRequireDefault(_hoistNonReactStatics);

	var _objectAssign = __webpack_require__(4);

	var _objectAssign2 = _interopRequireDefault(_objectAssign);

	function createComponent(Component, additionalContextTypes) {
	  var componentName = Component.displayName || Component.name;
	  var childContextTypes = _objectAssign2['default']({
	    reactor: _react2['default'].PropTypes.object.isRequired
	  }, additionalContextTypes || {});

	  var ReactorProvider = _react2['default'].createClass({
	    displayName: 'ReactorProvider(' + componentName + ')',

	    propTypes: {
	      reactor: _react2['default'].PropTypes.object.isRequired
	    },

	    childContextTypes: childContextTypes,

	    getChildContext: function getChildContext() {
	      var childContext = {
	        reactor: this.props.reactor
	      };
	      if (additionalContextTypes) {
	        Object.keys(additionalContextTypes).forEach(function (key) {
	          childContext[key] = this.props[key];
	        }, this);
	      }
	      return childContext;
	    },

	    render: function render() {
	      return _react2['default'].createElement(Component, this.props);
	    }
	  });

	  _hoistNonReactStatics2['default'](ReactorProvider, Component);

	  return ReactorProvider;
	}

	/**
	 * Provides reactor prop to all children as React context
	 *
	 * Example:
	 *   var WrappedComponent = provideReactor(Component, {
	 *     foo: React.PropTypes.string
	 *   });
	 *
	 * Also supports the decorator pattern:
	 *   @provideReactor({
	 *     foo: React.PropTypes.string
	 *   })
	 *   class BaseComponent extends React.Component {
	 *     render() {
	 *       return <div/>;
	 *     }
	 *   }
	 *
	 * @method provideReactor
	 * @param {React.Component} [Component] component to wrap
	 * @param {object} additionalContextTypes Additional contextTypes to add
	 * @returns {React.Component|Function} returns function if using decorator pattern
	 */

	function provideReactor(Component, additionalContextTypes) {
	  console.warn('`provideReactor` is deprecated, use `<Provider reactor={reactor} />` instead');
	  // support decorator pattern
	  if (arguments.length === 0 || typeof arguments[0] !== 'function') {
	    additionalContextTypes = arguments[0];
	    return function connectToReactorDecorator(ComponentToDecorate) {
	      return createComponent(ComponentToDecorate, additionalContextTypes);
	    };
	  }

	  return createComponent.apply(null, arguments);
	}

	module.exports = exports['default'];

/***/ }),
/* 225 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports['default'] = nuclearComponent;

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

	var _connect = __webpack_require__(220);

	var _connect2 = _interopRequireDefault(_connect);

	/**
	 * Provides dataBindings + reactor
	 * as props to wrapped component
	 *
	 * Example:
	 *   var WrappedComponent = nuclearComponent(Component, function(props) {
	 *     return { counter: 'counter' };
	 *   );
	 *
	 * Also supports the decorator pattern:
	 *   @nuclearComponent((props) => {
	 *     return { counter: 'counter' }
	 *   })
	 *   class BaseComponent extends React.Component {
	 *     render() {
	 *       const { counter, reactor } = this.props;
	 *       return <div/>;
	 *     }
	 *   }
	 *
	 * @method nuclearComponent
	 * @param {React.Component} [Component] component to wrap
	 * @param {Function} getDataBindings function which returns dataBindings to listen for data change
	 * @returns {React.Component|Function} returns function if using decorator pattern
	 */

	function nuclearComponent(Component, getDataBindings) {
	  console.warn('nuclearComponent is deprecated, use `connect()` instead');
	  // support decorator pattern
	  // detect all React Components because they have a render method
	  if (arguments.length === 0 || !Component.prototype.render) {
	    // Component here is the getDataBindings Function
	    return _connect2['default'](Component);
	  }

	  return _connect2['default'](getDataBindings)(Component);
	}

	module.exports = exports['default'];

/***/ }),
/* 226 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _reactRouter = __webpack_require__(164);

	var _patternUtils = __webpack_require__(227);

	var _config = __webpack_require__(228);

	var _config2 = _interopRequireDefault(_config);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	var _inst = null; /*
	                  Copyright 2015 Gravitational, Inc.
	                  
	                  Licensed under the Apache License, Version 2.0 (the "License");
	                  you may not use this file except in compliance with the License.
	                  You may obtain a copy of the License at
	                  
	                      http://www.apache.org/licenses/LICENSE-2.0
	                  
	                  Unless required by applicable law or agreed to in writing, software
	                  distributed under the License is distributed on an "AS IS" BASIS,
	                  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                  See the License for the specific language governing permissions and
	                  limitations under the License.
	                  */

	var history = {
	  original: function original() {
	    return _inst;
	  },
	  init: function init() {
	    var history = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : _reactRouter.browserHistory;

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
	    var url = _config2.default.routes.login;
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
	    url = this._canPush(url) ? url : _config2.default.routes.app;
	    return url;
	  },
	  ensureBaseUrl: function ensureBaseUrl(url) {
	    url = url || '';
	    if (url.indexOf(_config2.default.baseUrl) !== 0) {
	      url = _config2.default.baseUrl + url;
	    }

	    return url;
	  },
	  getRoutes: function getRoutes() {
	    return Object.getOwnPropertyNames(_config2.default.routes).map(function (p) {
	      return _config2.default.routes[p];
	    });
	  },
	  _canPush: function _canPush(route) {
	    route = route || '';
	    var routes = this.getRoutes();
	    if (route.indexOf(_config2.default.baseUrl) === 0) {
	      route = route.replace(_config2.default.baseUrl, '');
	    }

	    return routes.some(match(route));
	  },
	  _pageRefresh: function _pageRefresh(route) {
	    window.location.href = this.ensureBaseUrl(route);
	  }
	};

	var match = function match(url) {
	  return function (route) {
	    var _matchPattern = (0, _patternUtils.matchPattern)(route, url),
	        remainingPathname = _matchPattern.remainingPathname;

	    return remainingPathname !== null && remainingPathname.length === 0;
	  };
	};

	exports.default = history;
	module.exports = exports['default'];

/***/ }),
/* 227 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.compilePattern = compilePattern;
	exports.matchPattern = matchPattern;
	exports.getParamNames = getParamNames;
	exports.getParams = getParams;
	exports.formatPattern = formatPattern;

	var _invariant = __webpack_require__(168);

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

/***/ }),
/* 228 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _patternUtils = __webpack_require__(227);

	var _jQuery = __webpack_require__(229);

	var _jQuery2 = _interopRequireDefault(_jQuery);

	var _utils = __webpack_require__(232);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	var baseUrl = (0, _utils.isTestEnv)() ? 'localhost' : window.location.origin; /*
	                                                                              Copyright 2015 Gravitational, Inc.
	                                                                              
	                                                                              Licensed under the Apache License, Version 2.0 (the "License");
	                                                                              you may not use this file except in compliance with the License.
	                                                                              You may obtain a copy of the License at
	                                                                              
	                                                                                  http://www.apache.org/licenses/LICENSE-2.0
	                                                                              
	                                                                              Unless required by applicable law or agreed to in writing, software
	                                                                              distributed under the License is distributed on an "AS IS" BASIS,
	                                                                              WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                              See the License for the specific language governing permissions and
	                                                                              limitations under the License.
	                                                                              */

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
	    ttyEventWsAddr: ':fqdm/v1/webapi/sites/:cluster/sessions/:sid/events/stream?access_token=:token',
	    ttyResizeUrl: '/v1/webapi/sites/:cluster/sessions/:sid',

	    getSiteUrl: function getSiteUrl(siteId) {
	      return (0, _patternUtils.formatPattern)(cfg.api.sitePath, { siteId: siteId });
	    },
	    getSiteNodesUrl: function getSiteNodesUrl() {
	      var siteId = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : '-current-';

	      return (0, _patternUtils.formatPattern)(cfg.api.nodesPath, { siteId: siteId });
	    },
	    getSiteSessionUrl: function getSiteSessionUrl() {
	      var siteId = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : '-current-';

	      return (0, _patternUtils.formatPattern)(cfg.api.siteSessionPath, { siteId: siteId });
	    },
	    getSsoUrl: function getSsoUrl(providerUrl, providerName, redirect) {
	      return cfg.baseUrl + (0, _patternUtils.formatPattern)(providerUrl, { redirect: redirect, providerName: providerName });
	    },
	    getSiteEventsFilterUrl: function getSiteEventsFilterUrl(_ref) {
	      var start = _ref.start,
	          end = _ref.end,
	          siteId = _ref.siteId;

	      return (0, _patternUtils.formatPattern)(cfg.api.siteEventsFilterPath, { start: start, end: end, siteId: siteId });
	    },
	    getSessionEventsUrl: function getSessionEventsUrl(_ref2) {
	      var sid = _ref2.sid,
	          siteId = _ref2.siteId;

	      return (0, _patternUtils.formatPattern)(cfg.api.sessionEventsPath, { sid: sid, siteId: siteId });
	    },
	    getFetchSessionsUrl: function getFetchSessionsUrl(siteId) {
	      return (0, _patternUtils.formatPattern)(cfg.api.siteEventSessionFilterPath, { siteId: siteId });
	    },
	    getFetchSessionUrl: function getFetchSessionUrl(_ref3) {
	      var sid = _ref3.sid,
	          siteId = _ref3.siteId;

	      return (0, _patternUtils.formatPattern)(cfg.api.siteSessionPath + '/:sid', { sid: sid, siteId: siteId });
	    },
	    getInviteUrl: function getInviteUrl(inviteToken) {
	      return (0, _patternUtils.formatPattern)(cfg.api.invitePath, { inviteToken: inviteToken });
	    },
	    getU2fCreateUserChallengeUrl: function getU2fCreateUserChallengeUrl(inviteToken) {
	      return (0, _patternUtils.formatPattern)(cfg.api.u2fCreateUserChallengePath, { inviteToken: inviteToken });
	    }
	  },

	  getPlayerUrl: function getPlayerUrl(_ref4) {
	    var siteId = _ref4.siteId,
	        sid = _ref4.sid;

	    return (0, _patternUtils.formatPattern)(cfg.routes.player, { siteId: siteId, sid: sid });
	  },
	  getTerminalLoginUrl: function getTerminalLoginUrl(_ref5) {
	    var siteId = _ref5.siteId,
	        serverId = _ref5.serverId,
	        login = _ref5.login,
	        sid = _ref5.sid;

	    if (!sid) {
	      var url = this.stripOptionalParams(cfg.routes.terminal);
	      return (0, _patternUtils.formatPattern)(url, { siteId: siteId, serverId: serverId, login: login });
	    }

	    return (0, _patternUtils.formatPattern)(cfg.routes.terminal, { siteId: siteId, serverId: serverId, login: login, sid: sid });
	  },
	  getCurrentSessionRouteUrl: function getCurrentSessionRouteUrl(_ref6) {
	    var sid = _ref6.sid,
	        siteId = _ref6.siteId;

	    return (0, _patternUtils.formatPattern)(cfg.routes.currentSession, { sid: sid, siteId: siteId });
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

	    _jQuery2.default.extend(true, this, config);
	  },
	  stripOptionalParams: function stripOptionalParams(pattern) {
	    return pattern.replace(/\(.*\)/, '');
	  }
	};

	exports.default = cfg;
	module.exports = exports['default'];

/***/ }),
/* 229 */,
/* 230 */,
/* 231 */,
/* 232 */
/***/ (function(module, exports, __webpack_require__) {

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

	var isDevEnv = exports.isDevEnv = function isDevEnv() {
	    return ("production") === 'development';
	};
	var isTestEnv = exports.isTestEnv = function isTestEnv() {
	    return ("production") === 'test';
	};

/***/ }),
/* 233 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _nuclearJs = __webpack_require__(234);

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

/***/ }),
/* 234 */,
/* 235 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.addRoutes = addRoutes;

	var _config = __webpack_require__(228);

	var _config2 = _interopRequireDefault(_config);

	var _login = __webpack_require__(236);

	var _login2 = _interopRequireDefault(_login);

	var _invite = __webpack_require__(265);

	var _invite2 = _interopRequireDefault(_invite);

	var _msgPage = __webpack_require__(266);

	var Message = _interopRequireWildcard(_msgPage);

	var _documentTitle = __webpack_require__(267);

	var _documentTitle2 = _interopRequireDefault(_documentTitle);

	function _interopRequireWildcard(obj) { if (obj && obj.__esModule) { return obj; } else { var newObj = {}; if (obj != null) { for (var key in obj) { if (Object.prototype.hasOwnProperty.call(obj, key)) newObj[key] = obj[key]; } } newObj.default = obj; return newObj; } }

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function addRoutes() {
	  var routesToAdd = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : [];

	  return [{
	    component: _documentTitle2.default,
	    childRoutes: [{ path: _config2.default.routes.error, title: "Error", component: Message.ErrorPage }, { path: _config2.default.routes.info, title: "Info", component: Message.InfoPage }, { path: _config2.default.routes.login, title: "Login", component: _login2.default }, { path: _config2.default.routes.newUser, component: _invite2.default }, { path: _config2.default.routes.app, onEnter: function onEnter(localtion, replace) {
	        return replace(_config2.default.routes.nodes);
	      } }].concat(routesToAdd, [{ path: '*', component: Message.NotFound }])
	  }];
	} /*
	  Copyright 2015 Gravitational, Inc.
	  
	  Licensed under the Apache License, Version 2.0 (the "License");
	  you may not use this file except in compliance with the License.
	  You may obtain a copy of the License at
	  
	      http://www.apache.org/licenses/LICENSE-2.0
	  
	  Unless required by applicable law or agreed to in writing, software
	  distributed under the License is distributed on an "AS IS" BASIS,
	  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	  See the License for the specific language governing permissions and
	  limitations under the License.
	  */

/***/ }),
/* 236 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.LoginInputForm = exports.Login = undefined;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _nuclearJsReactAddons = __webpack_require__(219);

	var _jQuery = __webpack_require__(229);

	var _jQuery2 = _interopRequireDefault(_jQuery);

	__webpack_require__(237);

	var _actions = __webpack_require__(239);

	var _actions2 = _interopRequireDefault(_actions);

	var _user = __webpack_require__(250);

	var _googleAuthLogo = __webpack_require__(254);

	var _googleAuthLogo2 = _interopRequireDefault(_googleAuthLogo);

	var _config = __webpack_require__(228);

	var _config2 = _interopRequireDefault(_config);

	var _items = __webpack_require__(255);

	var _icons = __webpack_require__(256);

	var _ssoBtnList = __webpack_require__(263);

	var _enums = __webpack_require__(264);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               limitations under the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               */

	var Login = exports.Login = function (_React$Component) {
	  _inherits(Login, _React$Component);

	  function Login() {
	    var _temp, _this, _ret;

	    _classCallCheck(this, Login);

	    for (var _len = arguments.length, args = Array(_len), _key = 0; _key < _len; _key++) {
	      args[_key] = arguments[_key];
	    }

	    return _ret = (_temp = (_this = _possibleConstructorReturn(this, _React$Component.call.apply(_React$Component, [this].concat(args))), _this), _this.onLoginWithSso = function (ssoProvider) {
	      _actions2.default.loginWithSso(ssoProvider.name, ssoProvider.url);
	    }, _this.onLoginWithU2f = function (username, password) {
	      _actions2.default.loginWithU2f(username, password);
	    }, _this.onLogin = function (username, password, token) {
	      _actions2.default.login(username, password, token);
	    }, _temp), _possibleConstructorReturn(_this, _ret);
	  }

	  Login.prototype.render = function render() {
	    var attemp = this.props.attemp;

	    var authProviders = _config2.default.getAuthProviders();
	    var auth2faType = _config2.default.getAuth2faType();

	    return _react2.default.createElement(
	      'div',
	      { className: 'grv-login text-center' },
	      _react2.default.createElement(_icons.TeleportLogo, null),
	      _react2.default.createElement(
	        'div',
	        { className: 'grv-content grv-flex' },
	        _react2.default.createElement(
	          'div',
	          { className: 'grv-flex-column' },
	          _react2.default.createElement(LoginInputForm, {
	            authProviders: authProviders,
	            auth2faType: auth2faType,
	            onLoginWithSso: this.onLoginWithSso,
	            onLoginWithU2f: this.onLoginWithU2f,
	            onLogin: this.onLogin,
	            attemp: attemp
	          }),
	          _react2.default.createElement(LoginFooter, { auth2faType: auth2faType })
	        )
	      )
	    );
	  };

	  return Login;
	}(_react2.default.Component);

	var LoginInputForm = exports.LoginInputForm = function (_React$Component2) {
	  _inherits(LoginInputForm, _React$Component2);

	  function LoginInputForm(props) {
	    _classCallCheck(this, LoginInputForm);

	    var _this2 = _possibleConstructorReturn(this, _React$Component2.call(this, props));

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
	      var _this2$setState;

	      _this2.setState((_this2$setState = {}, _this2$setState[propName] = value, _this2$setState));
	    };

	    _this2.state = {
	      user: '',
	      password: '',
	      token: ''
	    };
	    return _this2;
	  }

	  LoginInputForm.prototype.isValid = function isValid() {
	    var $form = (0, _jQuery2.default)(this.refs.form);
	    return $form.length === 0 || $form.valid();
	  };

	  LoginInputForm.prototype.needs2fa = function needs2fa() {
	    return !!this.props.auth2faType && this.props.auth2faType !== _enums.Auth2faTypeEnum.DISABLED;
	  };

	  LoginInputForm.prototype.needsSso = function needsSso() {
	    return this.props.authProviders && this.props.authProviders.length > 0;
	  };

	  LoginInputForm.prototype.render2faFields = function render2faFields() {
	    var _this3 = this;

	    if (!this.needs2fa() || this.props.auth2faType !== _enums.Auth2faTypeEnum.OTP) {
	      return null;
	    }

	    return _react2.default.createElement(
	      'div',
	      { className: 'form-group' },
	      _react2.default.createElement('input', {
	        autoComplete: 'off',
	        value: this.state.token,
	        onChange: function onChange(e) {
	          return _this3.onChangeState('token', e.target.value);
	        },
	        className: 'form-control required',
	        name: 'token',
	        placeholder: 'Two factor token (Google Authenticator)' })
	    );
	  };

	  LoginInputForm.prototype.renderNameAndPassFields = function renderNameAndPassFields() {
	    var _this4 = this;

	    return _react2.default.createElement(
	      'div',
	      null,
	      _react2.default.createElement(
	        'div',
	        { className: 'form-group' },
	        _react2.default.createElement('input', {
	          autoFocus: true,
	          value: this.state.user,
	          onChange: function onChange(e) {
	            return _this4.onChangeState('user', e.target.value);
	          },
	          className: 'form-control required',
	          placeholder: 'User name',
	          name: 'userName' })
	      ),
	      _react2.default.createElement(
	        'div',
	        { className: 'form-group' },
	        _react2.default.createElement('input', {
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
	  };

	  LoginInputForm.prototype.renderLoginBtn = function renderLoginBtn() {
	    var isProcessing = this.props.attemp.isProcessing;

	    var $helpBlock = isProcessing && this.props.auth2faType === _enums.Auth2faTypeEnum.UTF ? _react2.default.createElement(
	      'div',
	      { className: 'help-block' },
	      'Insert your U2F key and press the button on the key'
	    ) : null;

	    var onClick = this.props.auth2faType === _enums.Auth2faTypeEnum.UTF ? this.onLoginWithU2f : this.onLogin;

	    return _react2.default.createElement(
	      'div',
	      null,
	      _react2.default.createElement(
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
	  };

	  LoginInputForm.prototype.renderSsoBtns = function renderSsoBtns() {
	    var _props = this.props,
	        authProviders = _props.authProviders,
	        attemp = _props.attemp;

	    if (!this.needsSso()) {
	      return null;
	    }

	    return _react2.default.createElement(_ssoBtnList.SsoBtnList, {
	      prefixText: 'Login with ',
	      isDisabled: attemp.isProcessing,
	      providers: authProviders,
	      onClick: this.onLoginWithSso });
	  };

	  LoginInputForm.prototype.render = function render() {
	    var _props$attemp = this.props.attemp,
	        isFailed = _props$attemp.isFailed,
	        message = _props$attemp.message;

	    var $error = isFailed ? _react2.default.createElement(_items.ErrorMessage, { message: message }) : null;

	    var hasAnyAuth = !!_config2.default.auth;

	    return _react2.default.createElement(
	      'div',
	      null,
	      _react2.default.createElement(
	        'form',
	        { ref: 'form', className: 'grv-login-input-form' },
	        _react2.default.createElement(
	          'h3',
	          null,
	          ' Welcome to Teleport '
	        ),
	        !hasAnyAuth ? _react2.default.createElement(
	          'div',
	          null,
	          ' You have no authentication options configured '
	        ) : _react2.default.createElement(
	          'div',
	          null,
	          this.renderNameAndPassFields(),
	          this.render2faFields(),
	          this.renderLoginBtn(),
	          this.renderSsoBtns(),
	          $error
	        )
	      )
	    );
	  };

	  return LoginInputForm;
	}(_react2.default.Component);

	LoginInputForm.propTypes = {
	  authProviders: _react2.default.PropTypes.array,
	  auth2faType: _react2.default.PropTypes.string,
	  onLoginWithSso: _react2.default.PropTypes.func.isRequired,
	  onLoginWithU2f: _react2.default.PropTypes.func.isRequired,
	  onLogin: _react2.default.PropTypes.func.isRequired,
	  attemp: _react2.default.PropTypes.object.isRequired
	};


	var LoginFooter = function LoginFooter(_ref) {
	  var auth2faType = _ref.auth2faType;

	  var $googleHint = auth2faType === _enums.Auth2faTypeEnum.OTP ? _react2.default.createElement(_googleAuthLogo2.default, null) : null;
	  return _react2.default.createElement(
	    'div',
	    null,
	    $googleHint,
	    _react2.default.createElement(
	      'div',
	      { className: 'grv-login-info' },
	      _react2.default.createElement('i', { className: 'fa fa-question' }),
	      _react2.default.createElement(
	        'strong',
	        null,
	        'New Account or forgot password?'
	      ),
	      _react2.default.createElement(
	        'div',
	        null,
	        'Ask for assistance from your Company administrator'
	      )
	    )
	  );
	};

	function mapStateToProps() {
	  return {
	    attemp: _user.getters.loginAttemp
	  };
	}

	exports.default = (0, _nuclearJsReactAddons.connect)(mapStateToProps)(Login);

/***/ }),
/* 237 */,
/* 238 */,
/* 239 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _reactor = __webpack_require__(233);

	var _reactor2 = _interopRequireDefault(_reactor);

	var _auth = __webpack_require__(240);

	var _auth2 = _interopRequireDefault(_auth);

	var _history = __webpack_require__(226);

	var _history2 = _interopRequireDefault(_history);

	var _session = __webpack_require__(244);

	var _session2 = _interopRequireDefault(_session);

	var _config = __webpack_require__(228);

	var _config2 = _interopRequireDefault(_config);

	var _api = __webpack_require__(241);

	var _api2 = _interopRequireDefault(_api);

	var _logger = __webpack_require__(245);

	var _logger2 = _interopRequireDefault(_logger);

	var _actions = __webpack_require__(246);

	var status = _interopRequireWildcard(_actions);

	var _actionTypes = __webpack_require__(249);

	function _interopRequireWildcard(obj) { if (obj && obj.__esModule) { return obj; } else { var newObj = {}; if (obj != null) { for (var key in obj) { if (Object.prototype.hasOwnProperty.call(obj, key)) newObj[key] = obj[key]; } } newObj.default = obj; return newObj; } }

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	var logger = _logger2.default.create('flux/user/actions'); /*
	                                                           Copyright 2015 Gravitational, Inc.
	                                                           
	                                                           Licensed under the Apache License, Version 2.0 (the "License");
	                                                           you may not use this file except in compliance with the License.
	                                                           You may obtain a copy of the License at
	                                                           
	                                                               http://www.apache.org/licenses/LICENSE-2.0
	                                                           
	                                                           Unless required by applicable law or agreed to in writing, software
	                                                           distributed under the License is distributed on an "AS IS" BASIS,
	                                                           WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                           See the License for the specific language governing permissions and
	                                                           limitations under the License.
	                                                           */

	var actions = {
	  fetchInvite: function fetchInvite(inviteToken) {
	    var path = _config2.default.api.getInviteUrl(inviteToken);
	    status.fetchInviteStatus.start();
	    _api2.default.get(path).done(function (invite) {
	      status.fetchInviteStatus.success();
	      _reactor2.default.dispatch(_actionTypes.RECEIVE_INVITE, invite);
	    }).fail(function (err) {
	      var msg = _api2.default.getErrorText(err);
	      status.fetchInviteStatus.fail(msg);
	    });
	  },
	  ensureUser: function ensureUser(nextState, replace, cb) {
	    _session2.default.ensureSession(true).done(function () {
	      cb();
	    });
	  },
	  acceptInvite: function acceptInvite(name, psw, token, inviteToken) {
	    var promise = _auth2.default.acceptInvite(name, psw, token, inviteToken);
	    actions._handleAcceptInvitePromise(promise);
	  },
	  acceptInviteWithU2f: function acceptInviteWithU2f(name, psw, inviteToken) {
	    var promise = _auth2.default.acceptInviteWithU2f(name, psw, inviteToken);
	    return actions._handleAcceptInvitePromise(promise);
	  },
	  loginWithSso: function loginWithSso(providerName, providerUrl) {
	    var entryUrl = this._getEntryRoute();
	    _history2.default.push(_config2.default.api.getSsoUrl(providerUrl, providerName, entryUrl), true);
	  },
	  loginWithU2f: function loginWithU2f(user, password) {
	    var promise = _auth2.default.loginWithU2f(user, password);
	    actions._handleLoginPromise(promise);
	  },
	  login: function login(user, password, token) {
	    var promise = _auth2.default.login(user, password, token);
	    actions._handleLoginPromise(promise);
	  },
	  logout: function logout() {
	    _session2.default.logout();
	  },
	  changePasswordWithU2f: function changePasswordWithU2f(oldPsw, newPsw) {
	    var promise = _auth2.default.changePasswordWithU2f(oldPsw, newPsw);
	    actions._handleChangePasswordPromise(promise);
	  },
	  changePassword: function changePassword(oldPass, newPass, token) {
	    var promise = _auth2.default.changePassword(oldPass, newPass, token);
	    actions._handleChangePasswordPromise(promise);
	  },
	  resetPasswordChangeAttempt: function resetPasswordChangeAttempt() {
	    status.changePasswordStatus.clear();
	  },
	  _handleChangePasswordPromise: function _handleChangePasswordPromise(promise) {
	    status.changePasswordStatus.start();
	    return promise.done(function () {
	      status.changePasswordStatus.success();
	    }).fail(function (err) {
	      var msg = _api2.default.getErrorText(err);
	      logger.error('change password', err);
	      status.changePasswordStatus.fail(msg);
	    });
	  },
	  _handleAcceptInvitePromise: function _handleAcceptInvitePromise(promise) {
	    status.signupStatus.start();
	    return promise.done(function () {
	      _history2.default.push(_config2.default.routes.app, true);
	    }).fail(function (err) {
	      var msg = _api2.default.getErrorText(err);
	      logger.error('accept invite', err);
	      status.signupStatus.fail(msg);
	    });
	  },
	  _handleLoginPromise: function _handleLoginPromise(promise) {
	    var _this = this;

	    status.loginStatus.start();
	    promise.done(function () {
	      var url = _this._getEntryRoute();
	      _history2.default.push(url, true);
	    }).fail(function (err) {
	      var msg = _api2.default.getErrorText(err);
	      logger.error('login', err);
	      status.loginStatus.fail(msg);
	    });
	  },
	  _getEntryRoute: function _getEntryRoute() {
	    var entryUrl = _history2.default.getRedirectParam();
	    if (entryUrl) {
	      entryUrl = _history2.default.ensureSafeRoute(entryUrl);
	    } else {
	      entryUrl = _config2.default.routes.app;
	    }

	    return _history2.default.ensureBaseUrl(entryUrl);
	  }
	};

	exports.default = actions;
	module.exports = exports['default'];

/***/ }),
/* 240 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _api = __webpack_require__(241);

	var _api2 = _interopRequireDefault(_api);

	var _config = __webpack_require__(228);

	var _config2 = _interopRequireDefault(_config);

	var _jQuery = __webpack_require__(229);

	var _jQuery2 = _interopRequireDefault(_jQuery);

	__webpack_require__(243);

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

	var auth = {
	  login: function login(email, password, token) {
	    var data = {
	      user: email,
	      pass: password,
	      second_factor_token: token
	    };

	    return _api2.default.post(_config2.default.api.sessionPath, data, false);
	  },
	  loginWithU2f: function loginWithU2f(name, password) {
	    var data = {
	      user: name,
	      pass: password
	    };

	    return _api2.default.post(_config2.default.api.u2fSessionChallengePath, data, false).then(function (data) {
	      var deferred = _jQuery2.default.Deferred();

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

	        _api2.default.post(_config2.default.api.u2fSessionPath, response, false).then(function (data) {
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

	    return _api2.default.post(_config2.default.api.createUserPath, data, false);
	  },
	  acceptInviteWithU2f: function acceptInviteWithU2f(name, password, inviteToken) {
	    return _api2.default.get(_config2.default.api.getU2fCreateUserChallengeUrl(inviteToken)).then(function (data) {
	      var deferred = _jQuery2.default.Deferred();
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

	        _api2.default.post(_config2.default.api.u2fCreateUserPath, response, false).then(function (data) {
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

	    return _api2.default.put(_config2.default.api.changeUserPasswordPath, data);
	  },
	  changePasswordWithU2f: function changePasswordWithU2f(oldPass, newPass) {
	    var data = {
	      user: name,
	      pass: oldPass
	    };

	    return _api2.default.post(_config2.default.api.u2fChangePassChallengePath, data).then(function (data) {
	      var deferred = _jQuery2.default.Deferred();

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

	        _api2.default.put(_config2.default.api.changeUserPasswordPath, data).then(function (data) {
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

	// This puts it in window.u2f
	exports.default = auth;
	module.exports = exports['default'];

/***/ }),
/* 241 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _jQuery = __webpack_require__(229);

	var _jQuery2 = _interopRequireDefault(_jQuery);

	var _localStorage = __webpack_require__(242);

	var _localStorage2 = _interopRequireDefault(_localStorage);

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
	      cache: false,
	      type: 'GET',
	      contentType: 'application/json; charset=utf-8',
	      dataType: 'json',
	      beforeSend: function beforeSend(xhr) {
	        xhr.setRequestHeader('X-CSRF-Token', getXCSRFToken());
	        if (withToken) {
	          var bearerToken = _localStorage2.default.getBearerToken() || {};
	          var accessToken = bearerToken.accessToken;

	          xhr.setRequestHeader('Authorization', 'Bearer ' + accessToken);
	        }
	      }
	    };

	    return _jQuery2.default.ajax(_jQuery2.default.extend({}, defaultCfg, cfg));
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
	  }
	};

	var getXCSRFToken = function getXCSRFToken() {
	  var metaTag = document.querySelector('[name=grv_csrf_token]');
	  return metaTag ? metaTag.content : '';
	};

	exports.default = api;
	module.exports = exports['default'];

/***/ }),
/* 242 */
/***/ (function(module, exports) {

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

	var KeysEnum = exports.KeysEnum = {
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

	exports.default = storage;

/***/ }),
/* 243 */
/***/ (function(module, exports) {

	
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


/***/ }),
/* 244 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.BearerToken = undefined;

	var _logger = __webpack_require__(245);

	var _logger2 = _interopRequireDefault(_logger);

	var _config = __webpack_require__(228);

	var _config2 = _interopRequireDefault(_config);

	var _jQuery = __webpack_require__(229);

	var _jQuery2 = _interopRequireDefault(_jQuery);

	var _history = __webpack_require__(226);

	var _history2 = _interopRequireDefault(_history);

	var _localStorage = __webpack_require__(242);

	var _localStorage2 = _interopRequireDefault(_localStorage);

	var _api = __webpack_require__(241);

	var _api2 = _interopRequireDefault(_api);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } } /*
	                                                                                                                                                          Copyright 2015 Gravitational, Inc.
	                                                                                                                                                          
	                                                                                                                                                          Licensed under the Apache License, Version 2.0 (the "License");
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
	var logger = _logger2.default.create('services/sessions');

	var BearerToken = exports.BearerToken = function BearerToken(json) {
	  _classCallCheck(this, BearerToken);

	  this.accessToken = json.token;
	  this.expiresIn = json.expires_in;
	  this.created = new Date().getTime();
	};

	var sesstionCheckerTimerId = null;

	var session = {
	  logout: function logout() {
	    var rememberLocation = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : false;

	    _api2.default.delete(_config2.default.api.sessionPath).always(function () {
	      _history2.default.goToLogin(rememberLocation);
	    });

	    this.clear();
	  },
	  clear: function clear() {
	    this._stopSessionChecker();
	    _localStorage2.default.unsubscribe(receiveMessage);
	    _localStorage2.default.setBearerToken(null);
	    _localStorage2.default.clear();
	  },
	  ensureSession: function ensureSession() {
	    var _this = this;

	    var rememberLocation = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : false;

	    this._stopSessionChecker();
	    this._ensureLocalStorageSubscription();

	    var token = this._getBearerToken();
	    if (!token) {
	      this.logout(rememberLocation);
	      return _jQuery2.default.Deferred().reject();
	    }

	    if (this._shouldRenewToken()) {
	      return this._renewToken().done(this._startSessionChecker.bind(this)).fail(function () {
	        return _this.logout(rememberLocation);
	      });
	    } else {
	      this._startSessionChecker();
	      return _jQuery2.default.Deferred().resolve(token);
	    }
	  },
	  _getBearerToken: function _getBearerToken() {
	    var token = null;
	    try {
	      token = this._extractBearerTokenFromHtml();
	      if (token) {
	        _localStorage2.default.setBearerToken(token);
	      } else {
	        token = _localStorage2.default.getBearerToken();
	      }
	    } catch (err) {
	      logger.error('Cannot find bearer token', err);
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
	    return _api2.default.post(_config2.default.api.renewTokenPath).then(this._receiveBearerToken.bind(this)).always(function () {
	      _this2._setAndBroadcastIsRenewing(false);
	    });
	  },
	  _receiveBearerToken: function _receiveBearerToken(json) {
	    var token = new BearerToken(json);
	    _localStorage2.default.setBearerToken(token);
	  },
	  _fetchStatus: function _fetchStatus() {
	    var _this3 = this;

	    _api2.default.get(_config2.default.api.userStatusPath).fail(function (err) {
	      // indicates that session is no longer valid (caused by server restarts or updates)
	      if (err.status == 403) {
	        _this3.logout();
	      }
	    });
	  },
	  _setAndBroadcastIsRenewing: function _setAndBroadcastIsRenewing(value) {
	    this._setIsRenewing(value);
	    _localStorage2.default.broadcast(_localStorage.KeysEnum.TOKEN_RENEW, value);
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
	    _localStorage2.default.subscribe(receiveMessage);
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

	  if (_localStorage2.default.getBearerToken() === null) {
	    session.logout();
	  }

	  // renewToken has been invoked from another tab
	  if (key === _localStorage.KeysEnum.TOKEN_RENEW && !!newValue) {
	    session._setIsRenewing(JSON.parse(newValue));
	  }
	}

	exports.default = session;

/***/ }),
/* 245 */
/***/ (function(module, exports) {

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

/***/ }),
/* 246 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.changePasswordStatus = exports.initSettingsStatus = exports.signupStatus = exports.fetchInviteStatus = exports.loginStatus = exports.initAppStatus = undefined;
	exports.makeStatus = makeStatus;

	var _reactor = __webpack_require__(233);

	var _reactor2 = _interopRequireDefault(_reactor);

	var _actionTypes = __webpack_require__(247);

	var AT = _interopRequireWildcard(_actionTypes);

	var _constants = __webpack_require__(248);

	var RT = _interopRequireWildcard(_constants);

	function _interopRequireWildcard(obj) { if (obj && obj.__esModule) { return obj; } else { var newObj = {}; if (obj != null) { for (var key in obj) { if (Object.prototype.hasOwnProperty.call(obj, key)) newObj[key] = obj[key]; } } newObj.default = obj; return newObj; } }

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function makeStatus(reqType) {
	  return {
	    start: function start() {
	      _reactor2.default.dispatch(AT.START, { type: reqType });
	    },
	    success: function success(message) {
	      _reactor2.default.dispatch(AT.SUCCESS, { type: reqType, message: message });
	    },
	    fail: function fail(message) {
	      _reactor2.default.dispatch(AT.FAIL, { type: reqType, message: message });
	    },
	    clear: function clear() {
	      _reactor2.default.dispatch(AT.CLEAR, { type: reqType });
	    }
	  };
	} /*
	  Copyright 2015 Gravitational, Inc.
	  
	  Licensed under the Apache License, Version 2.0 (the "License");
	  you may not use this file except in compliance with the License.
	  You may obtain a copy of the License at
	  
	      http://www.apache.org/licenses/LICENSE-2.0
	  
	  Unless required by applicable law or agreed to in writing, software
	  distributed under the License is distributed on an "AS IS" BASIS,
	  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	  See the License for the specific language governing permissions and
	  limitations under the License.
	  */

	var initAppStatus = exports.initAppStatus = makeStatus(RT.TRYING_TO_INIT_APP);
	var loginStatus = exports.loginStatus = makeStatus(RT.TRYING_TO_LOGIN);
	var fetchInviteStatus = exports.fetchInviteStatus = makeStatus(RT.FETCHING_INVITE);
	var signupStatus = exports.signupStatus = makeStatus(RT.TRYING_TO_SIGN_UP);
	var initSettingsStatus = exports.initSettingsStatus = makeStatus(RT.TRYING_TO_INIT_SETTINGS);
	var changePasswordStatus = exports.changePasswordStatus = makeStatus(RT.TRYING_TO_CHANGE_PSW);

/***/ }),
/* 247 */
/***/ (function(module, exports) {

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

	var START = exports.START = 'TLPT_STATUS_START';
	var SUCCESS = exports.SUCCESS = 'TLPT_STATUS_SUCCESS';
	var FAIL = exports.FAIL = 'TLPT_STATUS_FAIL';
	var CLEAR = exports.CLEAR = 'TLPT_STATUS_CLEAR';

/***/ }),
/* 248 */
/***/ (function(module, exports) {

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

	var TRYING_TO_SIGN_UP = exports.TRYING_TO_SIGN_UP = 'TRYING_TO_SIGN_UP';
	var TRYING_TO_LOGIN = exports.TRYING_TO_LOGIN = 'TRYING_TO_LOGIN';
	var FETCHING_INVITE = exports.FETCHING_INVITE = 'FETCHING_INVITE';
	var TRYING_TO_INIT_APP = exports.TRYING_TO_INIT_APP = 'TRYING_TO_INIT_APP';
	var TRYING_TO_INIT_SETTINGS = exports.TRYING_TO_INIT_SETTINGS = 'TRYING_TO_INIT_SETTINGS';
	var TRYING_TO_CHANGE_PSW = exports.TRYING_TO_CHANGE_PSW = 'TRYING_TO_CHANGE_PSW';

/***/ }),
/* 249 */
/***/ (function(module, exports) {

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

	var RECEIVE_USER = exports.RECEIVE_USER = 'TLPT_RECEIVE_USER';
	var RECEIVE_INVITE = exports.RECEIVE_INVITE = 'TLPT_RECEIVE_USER_INVITE';

/***/ }),
/* 250 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.getters = undefined;
	exports.getUser = getUser;

	var _reactor = __webpack_require__(233);

	var _reactor2 = _interopRequireDefault(_reactor);

	var _getters = __webpack_require__(251);

	var stsGetters = _interopRequireWildcard(_getters);

	function _interopRequireWildcard(obj) { if (obj && obj.__esModule) { return obj; } else { var newObj = {}; if (obj != null) { for (var key in obj) { if (Object.prototype.hasOwnProperty.call(obj, key)) newObj[key] = obj[key]; } } newObj.default = obj; return newObj; } }

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

	var STORE_NAME = 'tlpt_user';

	function getUser() {
	  return _reactor2.default.evaluate([STORE_NAME]);
	}

	var invite = [['tlpt_user_invite'], function (invite) {
	  return invite;
	}];
	var userName = [STORE_NAME, 'name'];

	var getters = exports.getters = {
	  userName: userName,
	  invite: invite,
	  pswChangeAttempt: stsGetters.changePasswordAttempt,
	  loginAttemp: stsGetters.loginAttempt,
	  attemp: stsGetters.signupAttempt,
	  fetchingInvite: stsGetters.fetchInviteAttempt
	};

/***/ }),
/* 251 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.changePasswordAttempt = exports.initSettingsAttempt = exports.signupAttempt = exports.fetchInviteAttempt = exports.loginAttempt = exports.initAppAttempt = exports.makeGetter = undefined;

	var _statusStore = __webpack_require__(252);

	var _constants = __webpack_require__(248);

	var RT = _interopRequireWildcard(_constants);

	function _interopRequireWildcard(obj) { if (obj && obj.__esModule) { return obj; } else { var newObj = {}; if (obj != null) { for (var key in obj) { if (Object.prototype.hasOwnProperty.call(obj, key)) newObj[key] = obj[key]; } } newObj.default = obj; return newObj; } }

	/*
	Copyright 2015 Gravitational, Inc.

	Licensed under the Apache License, Version 2.0 (the "License");
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

	var makeGetter = exports.makeGetter = function makeGetter(reqType) {
	  return [[STORE_NAME, reqType], function (rec) {
	    return rec || new _statusStore.TrackRec();
	  }];
	};

	var initAppAttempt = exports.initAppAttempt = makeGetter(RT.TRYING_TO_INIT_APP);
	var loginAttempt = exports.loginAttempt = makeGetter(RT.TRYING_TO_LOGIN);
	var fetchInviteAttempt = exports.fetchInviteAttempt = makeGetter(RT.FETCHING_INVITE);
	var signupAttempt = exports.signupAttempt = makeGetter(RT.TRYING_TO_SIGN_UP);
	var initSettingsAttempt = exports.initSettingsAttempt = makeGetter(RT.TRYING_TO_INIT_SETTINGS);
	var changePasswordAttempt = exports.changePasswordAttempt = makeGetter(RT.TRYING_TO_CHANGE_PSW);

/***/ }),
/* 252 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.TrackRec = undefined;

	var _nuclearJs = __webpack_require__(234);

	var _actionTypes = __webpack_require__(247);

	var AT = _interopRequireWildcard(_actionTypes);

	var _immutable = __webpack_require__(253);

	function _interopRequireWildcard(obj) { if (obj && obj.__esModule) { return obj; } else { var newObj = {}; if (obj != null) { for (var key in obj) { if (Object.prototype.hasOwnProperty.call(obj, key)) newObj[key] = obj[key]; } } newObj.default = obj; return newObj; } }

	var TrackRec = exports.TrackRec = new _immutable.Record({
	  isProcessing: false,
	  isFailed: false,
	  isSuccess: false,
	  message: ''
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
	    return (0, _nuclearJs.toImmutable)({});
	  },
	  initialize: function initialize() {
	    this.on(AT.START, start);
	    this.on(AT.FAIL, fail);
	    this.on(AT.SUCCESS, success);
	    this.on(AT.CLEAR, clear);
	  }
	});


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
/* 253 */
/***/ (function(module, exports, __webpack_require__) {

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

/***/ }),
/* 254 */
/***/ (function(module, exports, __webpack_require__) {

	"use strict";

	exports.__esModule = true;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	var GoogleAuthInfo = function GoogleAuthInfo() {
	  return _react2.default.createElement(
	    "div",
	    { className: "grv-google-auth text-left" },
	    _react2.default.createElement("div", { className: "grv-icon-google-auth" }),
	    _react2.default.createElement(
	      "strong",
	      null,
	      "Google Authenticator"
	    ),
	    _react2.default.createElement(
	      "div",
	      null,
	      "Download",
	      _react2.default.createElement(
	        "a",
	        { href: "https://support.google.com/accounts/answer/1066447?hl=en" },
	        _react2.default.createElement(
	          "span",
	          null,
	          " Google Authenticator "
	        )
	      ),
	      "on your phone to access your two factor token"
	    )
	  );
	}; /*
	   Copyright 2015 Gravitational, Inc.
	   
	   Licensed under the Apache License, Version 2.0 (the "License");
	   you may not use this file except in compliance with the License.
	   You may obtain a copy of the License at
	   
	       http://www.apache.org/licenses/LICENSE-2.0
	   
	   Unless required by applicable law or agreed to in writing, software
	   distributed under the License is distributed on an "AS IS" BASIS,
	   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	   See the License for the specific language governing permissions and
	   limitations under the License.
	   */

	exports.default = GoogleAuthInfo;
	module.exports = exports['default'];

/***/ }),
/* 255 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.ErrorMessage = undefined;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	var U2F_ERROR_CODES_URL = 'https://developers.yubico.com/U2F/Libraries/Client_error_codes.html';

	var ErrorMessage = exports.ErrorMessage = function ErrorMessage(_ref) {
	  var message = _ref.message;

	  message = message || '';
	  if (message.indexOf('U2F') !== -1) {
	    return _react2.default.createElement(
	      'label',
	      { className: 'grv-invite-login-error' },
	      message,
	      _react2.default.createElement('br', null),
	      _react2.default.createElement(
	        'small',
	        { className: 'grv-invite-login-error-u2f-codes' },
	        _react2.default.createElement(
	          'span',
	          null,
	          'click ',
	          _react2.default.createElement(
	            'a',
	            { target: '_blank', href: U2F_ERROR_CODES_URL },
	            'here'
	          ),
	          ' to learn more about U2F error codes'
	        )
	      )
	    );
	  }

	  return _react2.default.createElement(
	    'label',
	    { className: 'error' },
	    message,
	    ' '
	  );
	};

/***/ }),
/* 256 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.CloseIcon = exports.UserIcon = exports.TeleportLogo = undefined;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _classnames = __webpack_require__(257);

	var _classnames2 = _interopRequireDefault(_classnames);

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

	var logoSvg = __webpack_require__(258);
	var closeSvg = __webpack_require__(262);

	var TeleportLogo = function TeleportLogo() {
	  return _react2.default.createElement(
	    'svg',
	    { className: 'grv-icon-logo-tlpt' },
	    _react2.default.createElement('use', { xlinkHref: logoSvg })
	  );
	};

	var CloseIcon = function CloseIcon() {
	  return _react2.default.createElement(
	    'svg',
	    { className: 'grv-icon-close' },
	    _react2.default.createElement('use', { xlinkHref: closeSvg })
	  );
	};

	var UserIcon = function UserIcon(_ref) {
	  var _ref$name = _ref.name,
	      name = _ref$name === undefined ? '' : _ref$name,
	      isDark = _ref.isDark;

	  var iconClass = (0, _classnames2.default)('grv-icon-user', {
	    '--dark': isDark
	  });

	  return _react2.default.createElement(
	    'div',
	    { title: name, className: iconClass },
	    _react2.default.createElement(
	      'span',
	      null,
	      _react2.default.createElement(
	        'strong',
	        null,
	        name[0]
	      )
	    )
	  );
	};

	exports.TeleportLogo = TeleportLogo;
	exports.UserIcon = UserIcon;
	exports.CloseIcon = CloseIcon;

/***/ }),
/* 257 */,
/* 258 */
/***/ (function(module, exports, __webpack_require__) {

	;
	var sprite = __webpack_require__(259);;
	var image = "<symbol viewBox=\"0 0 340 100\" id=\"grv-tlpt-logo-full\" xmlns:xlink=\"http://www.w3.org/1999/xlink\"> <g> <g id=\"grv-tlpt-logo-full_Layer_2\"> <g> <g> <path d=\"m47.671001,21.444c-7.396,0 -14.102001,3.007999 -18.960003,7.866001c-4.856998,4.856998 -7.865999,11.563 -7.865999,18.959999c0,7.396 3.008001,14.101002 7.865999,18.957996s11.564003,7.865005 18.960003,7.865005s14.102001,-3.008003 18.958996,-7.865005s7.865005,-11.561996 7.865005,-18.957996s-3.008003,-14.104 -7.865005,-18.959999c-4.857994,-4.858002 -11.562996,-7.866001 -18.958996,-7.866001zm11.386997,19.509998h-8.213997v23.180004h-6.344002v-23.180004h-8.215v-5.612h22.772999v5.612l0,0z\"/> </g> <g> <path d=\"m92.782997,63.357002c-0.098999,-0.371002 -0.320999,-0.709 -0.646996,-0.942001l-4.562004,-3.958l-4.561996,-3.957001c0.163002,-0.887001 0.267998,-1.805 0.331001,-2.736c0.063995,-0.931 0.086998,-1.874001 0.086998,-2.805c0,-0.932999 -0.022003,-1.875 -0.086998,-2.806999c-0.063004,-0.931999 -0.167999,-1.851002 -0.331001,-2.736l4.561996,-3.957001l4.562004,-3.958c0.325996,-0.232998 0.548996,-0.57 0.646996,-0.942001c0.099007,-0.372997 0.075005,-0.778999 -0.087997,-1.153c-0.931999,-2.862 -2.199997,-5.655998 -3.731003,-8.299c-1.530998,-2.641998 -3.321999,-5.132998 -5.301994,-7.390999c-0.278999,-0.326 -0.617004,-0.548 -0.978004,-0.646c-0.360001,-0.098999 -0.744995,-0.074999 -1.116997,0.087l-5.750999,2.002001l-5.749001,2.000999c-1.419998,-1.164 -2.933998,-2.211 -4.522003,-3.136999c-1.589996,-0.925001 -3.253998,-1.728001 -4.977997,-2.404001l-1.139999,-5.959l-1.140999,-5.959c-0.069,-0.373 -0.268005,-0.733 -0.547005,-1.013c-0.278999,-0.28 -0.640999,-0.478 -1.036995,-0.524c-2.980003,-0.605 -6.007004,-0.908 -9.033005,-0.908s-6.052998,0.302 -9.032997,0.908c-0.396,0.046 -0.756001,0.245001 -1.036003,0.524c-0.278999,0.279 -0.477997,0.64 -0.546997,1.013l-1.141003,5.959l-1.140999,5.960001c-1.723,0.675999 -3.410999,1.479 -5.012001,2.403999c-1.599998,0.924999 -3.112999,1.973 -4.487,3.136999l-5.75,-2.000999l-5.75,-2.001999c-0.372,-0.164001 -0.755999,-0.187 -1.116999,-0.088001c-0.361,0.1 -0.699001,0.32 -0.978001,0.646c-1.979,2.259001 -3.771,4.75 -5.302,7.392002c-1.53,2.641998 -2.799,5.436996 -3.73,8.299c-0.163,0.372997 -0.187,0.780998 -0.087001,1.151997c0.099,0.372002 0.320001,0.710003 0.646001,0.943001l4.563,3.957001l4.562,3.958c-0.163,0.884998 -0.268,1.804001 -0.331001,2.735001c-0.063999,0.931999 -0.087999,1.875 -0.087999,2.806s0.023001,1.875 0.087,2.806c0.064001,0.931999 0.168001,1.851002 0.332001,2.735001l-4.562,3.957001l-4.562,3.959c-0.325,0.231003 -0.547,0.569 -0.646,0.942001c-0.099,0.370995 -0.076,0.778999 0.087,1.150002c0.931,2.864998 2.2,5.657997 3.73,8.300995c1.531,2.642998 3.323,5.133003 5.302,7.391998c0.280001,0.325005 0.618,0.548004 0.978001,0.646004c0.361,0.099998 0.744999,0.074997 1.118,-0.087997l5.75,-2.003006l5.749998,-2.000999c1.373001,1.164001 2.886002,2.213005 4.487003,3.139c1.600998,0.924004 3.288998,1.728004 5.010998,2.401001l1.140999,5.961998l1.141003,5.959c0.07,0.372002 0.267998,0.733002 0.547001,1.014c0.278999,0.279007 0.640999,0.479004 1.035999,0.522003c1.489998,0.278 2.979,0.500999 4.480999,0.651001c1.500999,0.152 3.014999,0.232002 4.551998,0.232002s3.049004,-0.080002 4.551003,-0.232002c1.501999,-0.150002 2.990997,-0.373001 4.479996,-0.651001c0.396004,-0.044998 0.757004,-0.243996 1.037003,-0.522003c0.279999,-0.278999 0.476997,-0.641998 0.547005,-1.014l1.140999,-5.959l1.140999,-5.961998c1.723,-0.674995 3.387001,-1.477997 4.976997,-2.401001c1.588005,-0.925995 3.103004,-1.974998 4.522003,-3.139l5.75,2.000999l5.75,2.003006c0.373001,0.162994 0.756996,0.185997 1.117996,0.087997c0.360001,-0.098999 0.698006,-0.32 0.978004,-0.646004c1.978996,-2.258995 3.770996,-4.749001 5.301994,-7.391998c1.531006,-2.642998 2.800003,-5.436996 3.731003,-8.300995c0.164001,-0.368004 0.188004,-0.778008 0.087997,-1.150002zm-24.237999,5.787994c-5.348,5.349007 -12.731995,8.660004 -20.875,8.660004c-8.143997,0 -15.526997,-3.312004 -20.875,-8.660004s-8.659998,-12.730995 -8.659998,-20.874996c0,-8.144001 3.312,-15.527 8.661001,-20.875999c5.348,-5.348001 12.731998,-8.661001 20.875999,-8.661001c8.143002,0 15.525997,3.312 20.874996,8.661001c5.348,5.348999 8.661003,12.731998 8.661003,20.875999c-0.000999,8.141998 -3.314003,15.525997 -8.663002,20.874996z\"/> </g> </g> </g> <g> <path d=\"m119.773003,30.861h-13.020004v-6.841h33.599998v6.841h-13.020004v35.639999h-7.55999v-35.639999l0,0z\"/> <path d=\"m143.953003,54.620998c0.23999,2.16 1.080002,3.84 2.520004,5.039997s3.179993,1.800003 5.219986,1.800003c1.800003,0 3.309006,-0.368996 4.530014,-1.110001c1.219986,-0.738998 2.289993,-1.668999 3.209991,-2.790001l5.160004,3.900002c-1.680008,2.080002 -3.561005,3.561005 -5.639999,4.440002c-2.080002,0.878998 -4.26001,1.319 -6.540009,1.319c-2.159988,0 -4.199997,-0.359001 -6.119995,-1.080002c-1.919998,-0.720001 -3.580994,-1.738998 -4.979996,-3.059998c-1.401001,-1.320007 -2.511002,-2.910004 -3.330002,-4.771004c-0.820007,-1.858997 -1.229996,-3.929996 -1.229996,-6.209999c0,-2.278999 0.409988,-4.349998 1.229996,-6.209999c0.819,-1.859001 1.929001,-3.449001 3.330002,-4.77c1.399002,-1.32 3.059998,-2.34 4.979996,-3.061001c1.919998,-0.719997 3.960007,-1.078999 6.119995,-1.078999c2,0 3.830002,0.351002 5.490005,1.049999c1.658997,0.700001 3.080002,1.709999 4.259995,3.028999c1.180008,1.32 2.100006,2.951 2.76001,4.891003c0.659988,1.939999 0.98999,4.169998 0.98999,6.688999v1.98h-21.959991l0,0.002998zm14.759995,-5.399998c-0.041,-2.118999 -0.699997,-3.789001 -1.979996,-5.010002c-1.281006,-1.219997 -3.059998,-1.829998 -5.339996,-1.829998c-2.160004,0 -3.87001,0.620998 -5.130005,1.860001c-1.259995,1.239998 -2.031006,2.899998 -2.309998,4.979h14.759995l0,0.000999z\"/> <path d=\"m172.753006,21.141001h7.199997v45.359999h-7.199997v-45.359999l0,0z\"/> <path d=\"m193.992004,54.620998c0.23999,2.16 1.080002,3.84 2.519989,5.039997c1.440002,1.200005 3.181,1.800003 5.221008,1.800003c1.800003,0 3.309006,-0.368996 4.528992,-1.110001c1.221008,-0.738998 2.290009,-1.668999 3.211014,-2.790001l5.159988,3.900002c-1.681,2.080002 -3.560989,3.561005 -5.640991,4.440002c-2.080002,0.878998 -4.26001,1.319 -6.540009,1.319c-2.158997,0 -4.199997,-0.359001 -6.119995,-1.080002c-1.919998,-0.720001 -3.580002,-1.738998 -4.979004,-3.059998c-1.401001,-1.320007 -2.511002,-2.910004 -3.330002,-4.771004c-0.819992,-1.858997 -1.228989,-3.929996 -1.228989,-6.209999c0,-2.278999 0.408997,-4.349998 1.228989,-6.209999c0.819,-1.859001 1.929001,-3.449001 3.330002,-4.77c1.399002,-1.32 3.059998,-2.34 4.979004,-3.061001c1.919998,-0.719997 3.960999,-1.078999 6.119995,-1.078999c2,0 3.830002,0.351002 5.490005,1.049999c1.658997,0.700001 3.078995,1.709999 4.259995,3.028999c1.180008,1.32 2.100998,2.951 2.761002,4.891003c0.660004,1.939999 0.988998,4.169998 0.988998,6.688999v1.98h-21.959991l0,0.002998zm14.759995,-5.399998c-0.039993,-2.118999 -0.699005,-3.789001 -1.979004,-5.010002c-1.279999,-1.219997 -3.059998,-1.829998 -5.340988,-1.829998c-2.159012,0 -3.869003,0.620998 -5.129013,1.860001c-1.259995,1.239998 -2.030991,2.899998 -2.310989,4.979h14.759995l0,0.000999z\"/> <path d=\"m222.671997,37.701h6.839996v4.319h0.12001c1.039993,-1.758999 2.438995,-3.039001 4.199997,-3.84c1.759995,-0.799999 3.660004,-1.199001 5.699005,-1.199001c2.19899,0 4.179993,0.389999 5.939987,1.170002c1.76001,0.778999 3.260025,1.850998 4.500015,3.209999c1.239014,1.360001 2.179993,2.959999 2.820007,4.799999c0.639984,1.84 0.959991,3.82 0.959991,5.938999c0,2.121002 -0.339996,4.101002 -1.019989,5.940002c-0.682007,1.840004 -1.631012,3.440002 -2.851013,4.800003c-1.221008,1.359993 -2.690002,2.43 -4.410004,3.209999s-3.600998,1.169998 -5.639999,1.169998c-1.360001,0 -2.561005,-0.140999 -3.600006,-0.420006c-1.041,-0.279991 -1.960999,-0.639992 -2.761002,-1.079994c-0.799988,-0.439003 -1.478989,-0.909004 -2.039993,-1.410004c-0.561005,-0.499001 -1.020004,-0.988998 -1.380005,-1.469994h-0.181v17.339996h-7.19899v-42.479l0.002991,0zm23.880005,14.400002c0,-1.119003 -0.190002,-2.199001 -0.569,-3.239002c-0.380997,-1.040001 -0.940994,-1.959999 -1.681,-2.760998c-0.740997,-0.799004 -1.630005,-1.439003 -2.669998,-1.920002c-1.040009,-0.479 -2.220001,-0.720001 -3.540009,-0.720001s-2.5,0.240002 -3.539993,0.720001c-1.040009,0.48 -1.931,1.120998 -2.669998,1.920002c-0.740997,0.800999 -1.300003,1.720997 -1.681,2.760998c-0.380005,1.040001 -0.569,2.119999 -0.569,3.239002c0,1.120998 0.188995,2.200996 0.569,3.239998c0.380997,1.041 0.938995,1.960995 1.681,2.759998c0.738998,0.801003 1.62999,1.440002 2.669998,1.919998c1.039993,0.480003 2.220001,0.721001 3.539993,0.721001s2.5,-0.239998 3.540009,-0.721001c1.039993,-0.478996 1.929001,-1.118996 2.669998,-1.919998c0.738998,-0.799004 1.300003,-1.718998 1.681,-2.759998c0.377991,-1.039001 0.569,-2.118999 0.569,-3.239998z\"/> <path d=\"m259.031006,52.101002c0,-2.279003 0.410004,-4.350002 1.230011,-6.210003c0.817993,-1.858997 1.928986,-3.448997 3.329987,-4.77c1.39801,-1.32 3.059021,-2.34 4.979004,-3.060997c1.920013,-0.720001 3.959991,-1.079002 6.119995,-1.079002s4.199005,0.359001 6.119019,1.079002c1.919983,0.720997 3.579987,1.739998 4.97998,3.060997s2.51001,2.91 3.330017,4.77c0.819977,1.860001 1.22998,3.931 1.22998,6.210003c0,2.279999 -0.410004,4.350998 -1.22998,6.210003c-0.820007,1.860001 -1.930023,3.449997 -3.330017,4.770996s-3.061005,2.340004 -4.97998,3.059998c-1.920013,0.721001 -3.959015,1.080002 -6.119019,1.080002s-4.199982,-0.359001 -6.119995,-1.080002c-1.92099,-0.719994 -3.580994,-1.738998 -4.979004,-3.059998c-1.401001,-1.32 -2.511993,-2.909996 -3.329987,-4.770996c-0.820007,-1.860004 -1.230011,-3.930004 -1.230011,-6.210003zm7.199005,0c0,1.120998 0.188995,2.200996 0.570007,3.239998c0.380005,1.041 0.938995,1.960995 1.679993,2.759998c0.73999,0.801003 1.630005,1.440002 2.670013,1.919998c1.040985,0.480003 2.220978,0.721001 3.540985,0.721001s2.498993,-0.239998 3.539001,-0.721001c1.040985,-0.478996 1.929993,-1.118996 2.670013,-1.919998c0.73999,-0.799004 1.300995,-1.718998 1.681976,-2.759998c0.378998,-1.039001 0.568024,-2.118999 0.568024,-3.239998c0,-1.119003 -0.189026,-2.199001 -0.568024,-3.239002c-0.380981,-1.040001 -0.940979,-1.959999 -1.681976,-2.760998c-0.740021,-0.799004 -1.629028,-1.439003 -2.670013,-1.920002c-1.040009,-0.479 -2.218994,-0.720001 -3.539001,-0.720001s-2.5,0.240002 -3.540985,0.720001c-1.040009,0.48 -1.930023,1.120998 -2.670013,1.920002c-0.73999,0.800999 -1.299988,1.720997 -1.679993,2.760998c-0.380005,1.039001 -0.570007,2.118999 -0.570007,3.239002z\"/> <path d=\"m297.070007,37.701h7.200989v4.560001h0.119019c0.798981,-1.68 1.938995,-2.979 3.419983,-3.899002s3.179993,-1.380001 5.100006,-1.380001c0.438995,0 0.871002,0.040001 1.290985,0.119003c0.420013,0.080997 0.850006,0.181 1.289001,0.300999v6.959999c-0.599976,-0.16 -1.188995,-0.290001 -1.769989,-0.390999c-0.579987,-0.098999 -1.149994,-0.149002 -1.710999,-0.149002c-1.679993,0 -3.028992,0.310001 -4.049011,0.93c-1.019989,0.621002 -1.800995,1.330002 -2.339996,2.130001c-0.540985,0.800999 -0.899994,1.601002 -1.079987,2.400002c-0.180023,0.800999 -0.27002,1.399998 -0.27002,1.799999v15.419998h-7.200989v-28.800999l0.001007,0z\"/> <path d=\"m317.049011,43.820999v-6.119999h5.940979v-8.34h7.199005v8.34h7.920013v6.119999h-7.920013v12.600002c0,1.439999 0.27002,2.579998 0.811005,3.420002c0.539001,0.839996 1.609009,1.259995 3.209015,1.259995c0.640991,0 1.339996,-0.069 2.10199,-0.209999c0.757996,-0.139999 1.359009,-0.369003 1.798981,-0.689003v6.060005c-0.759979,0.360001 -1.688995,0.608994 -2.788971,0.75c-1.10202,0.139999 -2.070007,0.209999 -2.910004,0.209999c-1.920013,0 -3.490021,-0.209999 -4.710999,-0.630005s-2.180023,-1.059998 -2.878998,-1.919998c-0.701019,-0.859001 -1.182007,-1.93 -1.44101,-3.209991c-0.26001,-1.279007 -0.389008,-2.76001 -0.389008,-4.440002v-13.201004h-5.941986l0,0z\"/> </g> <g> <path d=\"m119.194,86.295998h3.587997c0.346001,0 0.689003,0.041 1.027,0.124001c0.338005,0.082001 0.639,0.217003 0.903,0.402c0.264,0.187004 0.479004,0.427002 0.644005,0.722s0.246994,0.650002 0.246994,1.066002c0,0.519997 -0.146996,0.947998 -0.441994,1.287003c-0.295006,0.337997 -0.681,0.579994 -1.157005,0.727997v0.026001c0.286003,0.033997 0.553001,0.113998 0.800003,0.239998c0.247002,0.125999 0.457001,0.286003 0.629997,0.480003c0.173004,0.195 0.310005,0.420998 0.409004,0.676994s0.149994,0.530006 0.149994,0.825005c0,0.502998 -0.099998,0.920998 -0.298996,1.254997c-0.198997,0.333 -0.460999,0.603004 -0.786003,0.806c-0.324997,0.204002 -0.697998,0.348999 -1.117996,0.436005s-0.848,0.129997 -1.280998,0.129997h-3.315002v-9.204002l0,0zm1.638,3.744003h1.495003c0.545998,0 0.955994,-0.106003 1.228996,-0.318001c0.273003,-0.212997 0.408997,-0.491997 0.408997,-0.838997c0,-0.398003 -0.140999,-0.695 -0.421997,-0.891006c-0.281998,-0.194 -0.734001,-0.292 -1.358002,-0.292h-1.351997v2.340004l-0.000999,0zm0,4.056h1.507996c0.208,0 0.431007,-0.013 0.669006,-0.039001c0.237999,-0.025002 0.457001,-0.085999 0.656998,-0.181999c0.198997,-0.096001 0.363998,-0.231003 0.494003,-0.408997c0.129997,-0.178001 0.195,-0.418007 0.195,-0.722c0,-0.485001 -0.158005,-0.823006 -0.475006,-1.014c-0.315994,-0.191002 -0.807999,-0.286003 -1.475998,-0.286003h-1.572998v2.652l0.000999,0z\"/> <path d=\"m130.854996,91.560997l-3.457993,-5.264999h2.054001l2.261993,3.666l2.28801,-3.666h1.949997l-3.458008,5.264999v3.939003h-1.638v-3.939003l0,0z\"/> <path d=\"m150.796997,94.823997c-1.136002,0.606003 -2.404999,0.910004 -3.80899,0.910004c-0.711014,0 -1.363007,-0.114998 -1.957001,-0.345001s-1.105011,-0.555 -1.534012,-0.975998c-0.429001,-0.420006 -0.764999,-0.925003 -1.006989,-1.514c-0.243011,-0.590004 -0.363998,-1.244003 -0.363998,-1.964005c0,-0.736 0.120987,-1.404999 0.363998,-2.007996s0.578995,-1.116005 1.006989,-1.541c0.429001,-0.424004 0.940002,-0.750999 1.534012,-0.981003c0.593994,-0.228996 1.245987,-0.345001 1.957001,-0.345001c0.701996,0 1.360001,0.084999 1.975998,0.254005c0.61499,0.168999 1.166,0.471001 1.651001,0.903l-1.209,1.223c-0.295013,-0.286003 -0.652008,-0.508003 -1.072006,-0.663002c-0.421005,-0.155998 -0.865005,-0.234001 -1.332993,-0.234001c-0.477005,0 -0.908005,0.084999 -1.294006,0.253998c-0.384995,0.169006 -0.716995,0.402 -0.994003,0.701004c-0.276993,0.299995 -0.492004,0.648003 -0.643997,1.046997c-0.151993,0.398003 -0.227997,0.828003 -0.227997,1.287003c0,0.493996 0.076004,0.948997 0.227997,1.364998c0.151001,0.416 0.365997,0.775002 0.643997,1.079002c0.277008,0.303001 0.609009,0.541 0.994003,0.714996c0.386002,0.173004 0.817001,0.260002 1.294006,0.260002c0.416,0 0.807999,-0.039001 1.175995,-0.116997c0.367996,-0.078003 0.694992,-0.199005 0.981003,-0.362999v-2.171005h-1.88501v-1.480995h3.52301v4.704994l0.000992,0z\"/> <path d=\"m153.722,86.295998h3.197998c0.442001,0 0.869003,0.041 1.279999,0.124001c0.412003,0.082001 0.778,0.223 1.098999,0.422005c0.320007,0.198997 0.576004,0.467995 0.766998,0.806999c0.190002,0.337997 0.286011,0.766998 0.286011,1.285995c0,0.667999 -0.184998,1.227005 -0.553009,1.678001c-0.369003,0.450005 -0.894989,0.723999 -1.580002,0.818001l2.445007,4.069h-1.975998l-2.132004,-3.900002h-1.195999v3.900002h-1.638v-9.204002l0,0zm2.912003,3.900002c0.233994,0 0.468002,-0.011002 0.701996,-0.032997c0.234009,-0.021004 0.447998,-0.073006 0.643997,-0.154999c0.195007,-0.083 0.352997,-0.208 0.473999,-0.377007c0.122009,-0.168999 0.182007,-0.404999 0.182007,-0.709c0,-0.268997 -0.056,-0.485001 -0.169006,-0.648994c-0.112991,-0.165001 -0.259995,-0.288002 -0.442001,-0.371002c-0.181992,-0.082001 -0.383987,-0.137001 -0.603989,-0.162003c-0.221008,-0.026001 -0.436005,-0.039001 -0.644012,-0.039001h-1.416992v2.496002h1.274002l0,-0.000999z\"/> <path d=\"m165.876007,86.295998h1.416992l3.966003,9.204002h-1.872009l-0.857986,-2.106003h-3.991013l-0.832001,2.106003h-1.832993l4.003006,-9.204002zm2.080994,5.694l-1.417007,-3.743996l-1.442993,3.743996h2.860001l0,0z\"/> <path d=\"m171.401001,86.295998h1.884995l2.509003,6.955002l2.587006,-6.955002h1.76799l-3.716995,9.204002h-1.416992l-3.615005,-9.204002z\"/> <path d=\"m182.087006,86.295998h1.638v9.204002h-1.638v-9.204002l0,0z\"/> <path d=\"m188.613007,87.778h-2.820999v-1.482002h7.279999v1.482002h-2.820999v7.722h-1.638v-7.722l0,0z\"/> <path d=\"m196.959,86.295998h1.417007l3.965988,9.204002h-1.873001l-0.856995,-2.106003h-3.990997l-0.833008,2.106003h-1.832993l4.003998,-9.204002zm2.080002,5.694l-1.417007,-3.743996l-1.442001,3.743996h2.859009l0,0z\"/> <path d=\"m205.044998,87.778h-2.819992v-1.482002h7.278992v1.482002h-2.819992v7.722h-1.639008v-7.722l0,0z\"/> <path d=\"m211.570007,86.295998h1.638992v9.204002h-1.638992v-9.204002l0,0z\"/> <path d=\"m215.718994,90.936996c0,-0.736 0.121002,-1.404999 0.362991,-2.007996s0.578003,-1.115997 1.008011,-1.541c0.429001,-0.424004 0.938995,-0.750999 1.53299,-0.981003c0.594009,-0.228996 1.246002,-0.345001 1.957001,-0.345001c0.719009,-0.007996 1.378006,0.098007 1.977005,0.319c0.597992,0.221001 1.112991,0.544006 1.546997,0.968002c0.432999,0.425003 0.770996,0.937004 1.014008,1.534004c0.241989,0.598999 0.362991,1.265999 0.362991,2.001999c0,0.720001 -0.121002,1.374001 -0.362991,1.962997c-0.242004,0.590004 -0.581009,1.097 -1.014008,1.521004c-0.434006,0.424995 -0.949005,0.755997 -1.546997,0.993996c-0.598999,0.237999 -1.257996,0.362 -1.977005,0.371002c-0.710999,0 -1.362991,-0.114998 -1.957001,-0.345001s-1.103989,-0.555 -1.53299,-0.975998c-0.430008,-0.420006 -0.766006,-0.925003 -1.008011,-1.514c-0.241989,-0.588005 -0.362991,-1.243004 -0.362991,-1.962006zm1.715012,-0.103996c0,0.494003 0.076004,0.948997 0.229004,1.364998c0.149994,0.416 0.365005,0.775002 0.643005,1.079002c0.276993,0.303001 0.608994,0.541 0.993988,0.714996c0.387009,0.173004 0.817001,0.260002 1.295013,0.260002c0.47699,0 0.908997,-0.086998 1.298996,-0.260002c0.390991,-0.173996 0.724991,-0.411995 1.001999,-0.714996c0.276993,-0.304001 0.490997,-0.663002 0.643005,-1.079002c0.151993,-0.416 0.228989,-0.870995 0.228989,-1.364998c0,-0.459 -0.075989,-0.889 -0.228989,-1.287003c-0.151001,-0.397995 -0.365005,-0.746994 -0.643005,-1.046997c-0.277008,-0.299004 -0.611008,-0.531998 -1.001999,-0.701004c-0.389999,-0.168999 -0.822006,-0.253998 -1.298996,-0.253998c-0.478012,0 -0.908005,0.084999 -1.295013,0.253998c-0.384995,0.169006 -0.716995,0.402 -0.993988,0.701004c-0.277008,0.300003 -0.492004,0.648003 -0.643005,1.046997c-0.153015,0.398003 -0.229004,0.828003 -0.229004,1.287003z\"/> <path d=\"m228.029007,86.295998h2.17099l4.459,6.838005h0.026001v-6.838005h1.637009v9.204002h-2.07901l-4.550003,-7.058998h-0.025986v7.058998h-1.638v-9.204002l0,0z\"/> <path d=\"m242.341995,86.295998h1.417007l3.966003,9.204002h-1.873001l-0.85701,-2.106003h-3.990997l-0.832993,2.106003h-1.833008l4.003998,-9.204002zm2.080002,5.694l-1.416992,-3.743996l-1.442001,3.743996h2.858994l0,0z\"/> <path d=\"m249.738007,86.295998h1.638992v7.722h3.912003v1.482002h-5.550995v-9.204002l0,0z\"/> </g> </g> </symbol>";
	module.exports = sprite.add(image, "grv-tlpt-logo-full");

/***/ }),
/* 259 */
/***/ (function(module, exports, __webpack_require__) {

	var Sprite = __webpack_require__(260);
	var globalSprite = new Sprite();

	if (document.body) {
	  globalSprite.elem = globalSprite.render(document.body);
	} else {
	  document.addEventListener('DOMContentLoaded', function () {
	    globalSprite.elem = globalSprite.render(document.body);
	  }, false);
	}

	module.exports = globalSprite;


/***/ }),
/* 260 */
/***/ (function(module, exports, __webpack_require__) {

	var Sniffr = __webpack_require__(261);

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


/***/ }),
/* 261 */
/***/ (function(module, exports) {

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


/***/ }),
/* 262 */
/***/ (function(module, exports, __webpack_require__) {

	;
	var sprite = __webpack_require__(259);;
	var image = "<symbol viewBox=\"0 0 90.000000 90.000000\" id=\"grv-icon-close\" xmlns:svg=\"http://www.w3.org/2000/svg\"> <g> <title>Layer 1</title> <g id=\"grv-icon-close_svg_-\" transform=\"translate(0,95) scale(0.10000000149011612,-0.10000000149011612) \"> <path id=\"grv-icon-close_svg_2\" d=\"m329,932c-217,-57 -359,-280 -321,-504c17,-100 54,-172 126,-244c72,-72 144,-109 244,-126c89,-15 190,-1 272,39c71,34 169,132 203,203c79,163 52,362 -66,495c-114,131 -286,182 -458,137zm78,-344l43,-42l44,43c29,29 50,42 60,38c24,-9 20,-19 -29,-67l-45,-44l47,-48c40,-41 44,-49 33,-63c-12,-14 -19,-11 -62,32l-48,47l-48,-47c-44,-43 -50,-46 -62,-32c-12,14 -7,22 33,63l47,47l-46,46c-43,43 -50,69 -21,69c6,0 31,-19 54,-42z\"/> </g> </g> </symbol>";
	module.exports = sprite.add(image, "grv-icon-close");

/***/ }),
/* 263 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.SsoBtnList = undefined;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _enums = __webpack_require__(264);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	var guessProviderBtnClass = function guessProviderBtnClass(name, type) {
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

	  if (name.indexOf('github') !== -1 || type === _enums.AuthProviderTypeEnum.GITHUB) {
	    return 'btn-github';
	  }

	  if (type === _enums.AuthProviderTypeEnum.OIDC) {
	    return 'btn-openid';
	  }

	  return '--unknown';
	};

	var SsoBtnList = function SsoBtnList(_ref) {
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
	    var providerBtnClass = guessProviderBtnClass(displayName, type);
	    var btnClass = 'btn grv-user-btn-sso full-width ' + providerBtnClass;
	    return _react2.default.createElement(
	      'button',
	      { key: index,
	        disabled: isDisabled,
	        className: btnClass,
	        onClick: function onClick(e) {
	          e.preventDefault();_onClick(item);
	        } },
	      _react2.default.createElement(
	        'div',
	        { className: '--sso-icon' },
	        _react2.default.createElement('span', { className: 'fa' })
	      ),
	      _react2.default.createElement(
	        'span',
	        null,
	        title
	      )
	    );
	  });

	  if ($btns.length === 0) {
	    return _react2.default.createElement(
	      'h4',
	      null,
	      ' You have no SSO providers configured '
	    );
	  }

	  return _react2.default.createElement(
	    'div',
	    null,
	    ' ',
	    $btns,
	    ' '
	  );
	};

	exports.SsoBtnList = SsoBtnList;

/***/ }),
/* 264 */
/***/ (function(module, exports) {

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

	var AuthProviderTypeEnum = exports.AuthProviderTypeEnum = {
	  OIDC: 'oidc',
	  SAML: 'saml',
	  GITHUB: 'github'
	};

	var RestRespCodeEnum = exports.RestRespCodeEnum = {
	  FORBIDDEN: 403
	};

	var Auth2faTypeEnum = exports.Auth2faTypeEnum = {
	  UTF: 'u2f',
	  OTP: 'otp',
	  DISABLED: 'off'
	};

	var AuthTypeEnum = exports.AuthTypeEnum = {
	  LOCAL: 'local',
	  SSO: 'sso'
	};

/***/ }),
/* 265 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.InviteInputForm = exports.Invite = undefined;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _jQuery = __webpack_require__(229);

	var _jQuery2 = _interopRequireDefault(_jQuery);

	var _classnames = __webpack_require__(257);

	var _classnames2 = _interopRequireDefault(_classnames);

	var _nuclearJsReactAddons = __webpack_require__(219);

	var _config = __webpack_require__(228);

	var _config2 = _interopRequireDefault(_config);

	var _actions = __webpack_require__(239);

	var _actions2 = _interopRequireDefault(_actions);

	var _user = __webpack_require__(250);

	var _enums = __webpack_require__(264);

	var _msgPage = __webpack_require__(266);

	var _icons = __webpack_require__(256);

	var _googleAuthLogo = __webpack_require__(254);

	var _googleAuthLogo2 = _interopRequireDefault(_googleAuthLogo);

	var _items = __webpack_require__(255);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
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

	var needs2fa = function needs2fa(auth2faType) {
	  return !!auth2faType && auth2faType !== _enums.Auth2faTypeEnum.DISABLED;
	};

	var Invite = exports.Invite = function (_React$Component) {
	  _inherits(Invite, _React$Component);

	  function Invite() {
	    var _temp, _this, _ret;

	    _classCallCheck(this, Invite);

	    for (var _len = arguments.length, args = Array(_len), _key = 0; _key < _len; _key++) {
	      args[_key] = arguments[_key];
	    }

	    return _ret = (_temp = (_this = _possibleConstructorReturn(this, _React$Component.call.apply(_React$Component, [this].concat(args))), _this), _this.onSubmitWithU2f = function (username, password) {
	      _actions2.default.acceptInviteWithU2f(username, password, _this.props.params.inviteToken);
	    }, _this.onSubmit = function (username, password, token) {
	      _actions2.default.acceptInvite(username, password, token, _this.props.params.inviteToken);
	    }, _temp), _possibleConstructorReturn(_this, _ret);
	  }

	  Invite.prototype.componentDidMount = function componentDidMount() {
	    _actions2.default.fetchInvite(this.props.params.inviteToken);
	  };

	  Invite.prototype.render = function render() {
	    var _props = this.props,
	        fetchingInvite = _props.fetchingInvite,
	        invite = _props.invite,
	        attemp = _props.attemp;

	    var auth2faType = _config2.default.getAuth2faType();

	    if (fetchingInvite.isFailed) {
	      return _react2.default.createElement(_msgPage.ExpiredLink, null);
	    }

	    if (!invite) {
	      return null;
	    }

	    var containerClass = (0, _classnames2.default)('grv-invite-content grv-flex', {
	      '---with-2fa-data': needs2fa(auth2faType)
	    });

	    return _react2.default.createElement(
	      'div',
	      { className: 'grv-invite text-center' },
	      _react2.default.createElement(_icons.TeleportLogo, null),
	      _react2.default.createElement(
	        'div',
	        { className: containerClass },
	        _react2.default.createElement(
	          'div',
	          { className: 'grv-flex-column' },
	          _react2.default.createElement(InviteInputForm, {
	            auth2faType: auth2faType,
	            attemp: attemp,
	            invite: invite,
	            onSubmitWithU2f: this.onSubmitWithU2f,
	            onSubmit: this.onSubmit
	          }),
	          _react2.default.createElement(InviteFooter, { auth2faType: auth2faType })
	        ),
	        _react2.default.createElement(Invite2faData, {
	          auth2faType: auth2faType,
	          qr: invite.qr })
	      )
	    );
	  };

	  return Invite;
	}(_react2.default.Component);

	var InviteInputForm = exports.InviteInputForm = function (_React$Component2) {
	  _inherits(InviteInputForm, _React$Component2);

	  function InviteInputForm(props) {
	    _classCallCheck(this, InviteInputForm);

	    var _this2 = _possibleConstructorReturn(this, _React$Component2.call(this, props));

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
	      var _this2$setState;

	      _this2.setState((_this2$setState = {}, _this2$setState[propName] = value, _this2$setState));
	    };

	    _this2.state = {
	      userName: _this2.props.invite.user,
	      password: '',
	      passwordConfirmed: '',
	      token: ''
	    };
	    return _this2;
	  }

	  InviteInputForm.prototype.componentDidMount = function componentDidMount() {
	    (0, _jQuery2.default)(this.refs.form).validate({
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
	          minlength: _jQuery2.default.validator.format('Enter at least {0} characters'),
	          equalTo: 'Enter the same password as above'
	        }
	      }
	    });
	  };

	  InviteInputForm.prototype.isValid = function isValid() {
	    var $form = (0, _jQuery2.default)(this.refs.form);
	    return $form.length === 0 || $form.valid();
	  };

	  InviteInputForm.prototype.renderNameAndPassFields = function renderNameAndPassFields() {
	    var _this3 = this;

	    return _react2.default.createElement(
	      'div',
	      null,
	      _react2.default.createElement(
	        'div',
	        { className: 'form-group' },
	        _react2.default.createElement('input', {
	          disabled: true,
	          autoFocus: true,
	          value: this.state.userName,
	          onChange: function onChange(e) {
	            return _this3.onChangeState('userName', e.target.value);
	          },
	          className: 'form-control required',
	          placeholder: 'User name',
	          name: 'userName' })
	      ),
	      _react2.default.createElement(
	        'div',
	        { className: 'form-group' },
	        _react2.default.createElement('input', {
	          value: this.state.password,
	          onChange: function onChange(e) {
	            return _this3.onChangeState('password', e.target.value);
	          },
	          ref: 'password',
	          type: 'password',
	          name: 'password',
	          className: 'form-control required',
	          placeholder: 'Password' })
	      ),
	      _react2.default.createElement(
	        'div',
	        { className: 'form-group' },
	        _react2.default.createElement('input', {
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
	  };

	  InviteInputForm.prototype.render2faFields = function render2faFields() {
	    var _this4 = this;

	    var auth2faType = this.props.auth2faType;

	    if (needs2fa(auth2faType) && auth2faType === _enums.Auth2faTypeEnum.OTP) {
	      return _react2.default.createElement(
	        'div',
	        { className: 'form-group' },
	        _react2.default.createElement('input', {
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
	  };

	  InviteInputForm.prototype.renderSubmitBtn = function renderSubmitBtn() {
	    var isProcessing = this.props.attemp.isProcessing;

	    var $helpBlock = isProcessing && this.props.auth2faType === _enums.Auth2faTypeEnum.UTF ? _react2.default.createElement(
	      'div',
	      { className: 'help-block' },
	      'Insert your U2F key and press the button on the key'
	    ) : null;

	    var onClick = this.props.auth2faType === _enums.Auth2faTypeEnum.UTF ? this.onSubmitWithU2f : this.onSubmit;

	    return _react2.default.createElement(
	      'div',
	      null,
	      _react2.default.createElement(
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
	  };

	  InviteInputForm.prototype.render = function render() {
	    var _props$attemp = this.props.attemp,
	        isFailed = _props$attemp.isFailed,
	        message = _props$attemp.message;

	    var $error = isFailed ? _react2.default.createElement(_items.ErrorMessage, { message: message }) : null;
	    return _react2.default.createElement(
	      'form',
	      { ref: 'form', className: 'grv-invite-input-form' },
	      _react2.default.createElement(
	        'h3',
	        null,
	        ' Get started with Teleport '
	      ),
	      this.renderNameAndPassFields(),
	      this.render2faFields(),
	      this.renderSubmitBtn(),
	      $error
	    );
	  };

	  return InviteInputForm;
	}(_react2.default.Component);

	InviteInputForm.propTypes = {
	  auth2faType: _react2.default.PropTypes.string,
	  authType: _react2.default.PropTypes.string,
	  onSubmitWithU2f: _react2.default.PropTypes.func.isRequired,
	  onSubmit: _react2.default.PropTypes.func.isRequired,
	  attemp: _react2.default.PropTypes.object.isRequired
	};


	var Invite2faData = function Invite2faData(_ref) {
	  var auth2faType = _ref.auth2faType,
	      qr = _ref.qr;

	  if (!needs2fa(auth2faType)) {
	    return null;
	  }

	  if (auth2faType === _enums.Auth2faTypeEnum.OTP) {
	    return _react2.default.createElement(
	      'div',
	      { className: 'grv-flex-column grv-invite-barcode' },
	      _react2.default.createElement(
	        'h4',
	        null,
	        'Scan bar code for auth token ',
	        _react2.default.createElement('br', null),
	        _react2.default.createElement(
	          'small',
	          null,
	          'Scan below to generate your two factor token'
	        )
	      ),
	      _react2.default.createElement('img', { className: 'img-thumbnail', src: 'data:image/png;base64,' + qr })
	    );
	  }

	  if (auth2faType === _enums.Auth2faTypeEnum.UTF) {
	    return _react2.default.createElement(
	      'div',
	      { className: 'grv-flex-column' },
	      _react2.default.createElement(
	        'h3',
	        null,
	        'Insert your U2F key '
	      ),
	      _react2.default.createElement(
	        'div',
	        { className: 'm-t-md' },
	        'Press the button on the U2F key after you press the sign up button'
	      ),
	      _react2.default.createElement(
	        'div',
	        { className: 'm-t text-muted' },
	        _react2.default.createElement(
	          'small',
	          null,
	          'Click',
	          _react2.default.createElement(
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

	var InviteFooter = function InviteFooter(_ref2) {
	  var auth2faType = _ref2.auth2faType;

	  var $googleHint = auth2faType === _enums.Auth2faTypeEnum.OTP ? _react2.default.createElement(_googleAuthLogo2.default, null) : null;
	  return _react2.default.createElement(
	    'div',
	    null,
	    $googleHint
	  );
	};

	function mapStateToProps() {
	  return {
	    invite: _user.getters.invite,
	    attemp: _user.getters.attemp,
	    fetchingInvite: _user.getters.fetchingInvite
	  };
	}

	exports.default = (0, _nuclearJsReactAddons.connect)(mapStateToProps)(Invite);

/***/ }),
/* 266 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.ExpiredLink = exports.AccessDenied = exports.Failed = exports.NotFound = exports.InfoPage = exports.ErrorPage = exports.MSG_ERROR_ACCESS_DENIED = exports.MSG_ERROR_EXPIRED_INVITE_DETAILS = exports.MSG_ERROR_EXPIRED_INVITE = exports.MSG_ERROR_NOT_FOUND_DETAILS = exports.MSG_ERROR_NOT_FOUND = exports.MSG_ERROR_DEFAULT = exports.MSG_ERROR_LOGIN_FAILED = exports.MSG_INFO_LOGIN_SUCCESS = undefined;

	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; }; /*
	                                                                                                                                                                                                                                                                  Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                  
	                                                                                                                                                                                                                                                                  Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                  you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                  You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                  
	                                                                                                                                                                                                                                                                      http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                  
	                                                                                                                                                                                                                                                                  Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                  distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                  See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                  limitations under the License.
	                                                                                                                                                                                                                                                                  */

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	var MSG_INFO_LOGIN_SUCCESS = exports.MSG_INFO_LOGIN_SUCCESS = 'Login was successful, you can close this window and continue using tsh.';
	var MSG_ERROR_LOGIN_FAILED = exports.MSG_ERROR_LOGIN_FAILED = 'Login unsuccessful. Please try again, if the problem persists, contact your system administrator.';
	var MSG_ERROR_DEFAULT = exports.MSG_ERROR_DEFAULT = 'Internal Error';
	var MSG_ERROR_NOT_FOUND = exports.MSG_ERROR_NOT_FOUND = '404 Not Found';
	var MSG_ERROR_NOT_FOUND_DETAILS = exports.MSG_ERROR_NOT_FOUND_DETAILS = 'Looks like the page you are looking for isn\'t here any longer.';
	var MSG_ERROR_EXPIRED_INVITE = exports.MSG_ERROR_EXPIRED_INVITE = 'Invite code has expired.';
	var MSG_ERROR_EXPIRED_INVITE_DETAILS = exports.MSG_ERROR_EXPIRED_INVITE_DETAILS = 'Looks like your invite code isn\'t valid anymore.';
	var MSG_ERROR_ACCESS_DENIED = exports.MSG_ERROR_ACCESS_DENIED = 'Access denied';

	var ErrorPageEnum = {
	  FAILED_TO_LOGIN: 'login_failed',
	  EXPIRED_INVITE: 'expired_invite',
	  NOT_FOUND: 'not_found',
	  ACCESS_DENIED: 'access_denied'
	};

	var InfoPageEnum = {
	  LOGIN_SUCCESS: 'login_success'
	};

	var InfoPage = function InfoPage(_ref) {
	  var params = _ref.params;
	  var type = params.type;

	  if (type === InfoPageEnum.LOGIN_SUCCESS) {
	    return _react2.default.createElement(SuccessfulLogin, null);
	  }

	  return _react2.default.createElement(InfoBox, null);
	};

	var ErrorPage = function ErrorPage(_ref2) {
	  var params = _ref2.params,
	      location = _ref2.location;
	  var type = params.type;

	  var details = location.query.details;
	  switch (type) {
	    case ErrorPageEnum.FAILED_TO_LOGIN:
	      return _react2.default.createElement(LoginFailed, { message: details });
	    case ErrorPageEnum.EXPIRED_INVITE:
	      return _react2.default.createElement(ExpiredLink, null);
	    case ErrorPageEnum.NOT_FOUND:
	      return _react2.default.createElement(NotFound, null);
	    case ErrorPageEnum.ACCESS_DENIED:
	      return _react2.default.createElement(AccessDenied, { message: details });
	    default:
	      return _react2.default.createElement(Failed, { message: details });
	  }
	};

	var Box = function Box(props) {
	  return _react2.default.createElement(
	    'div',
	    { className: 'grv-msg-page' },
	    _react2.default.createElement(
	      'div',
	      { className: 'grv-header' },
	      _react2.default.createElement('i', { className: props.iconClass })
	    ),
	    props.children
	  );
	};

	var InfoBox = function InfoBox(props) {
	  return _react2.default.createElement(Box, _extends({ iconClass: 'fa fa-smile-o' }, props));
	};

	var ErrorBox = function ErrorBox(props) {
	  return _react2.default.createElement(Box, _extends({ iconClass: 'fa fa-frown-o' }, props));
	};

	var ErrorBoxDetails = function ErrorBoxDetails(_ref3) {
	  var _ref3$message = _ref3.message,
	      message = _ref3$message === undefined ? '' : _ref3$message;
	  return _react2.default.createElement(
	    'div',
	    { className: 'm-t text-muted' },
	    _react2.default.createElement(
	      'small',
	      { className: 'grv-msg-page-details-text' },
	      message
	    ),
	    _react2.default.createElement(
	      'p',
	      null,
	      _react2.default.createElement(
	        'small',
	        { className: 'contact-section' },
	        'If you believe this is an issue with Teleport, please ',
	        _react2.default.createElement(
	          'a',
	          { href: 'https://github.com/gravitational/teleport/issues/new' },
	          'create a GitHub issue.'
	        )
	      )
	    )
	  );
	};

	var NotFound = function NotFound() {
	  return _react2.default.createElement(
	    ErrorBox,
	    null,
	    _react2.default.createElement(
	      'h1',
	      null,
	      MSG_ERROR_NOT_FOUND
	    ),
	    _react2.default.createElement(ErrorBoxDetails, { message: MSG_ERROR_NOT_FOUND_DETAILS })
	  );
	};

	var AccessDenied = function AccessDenied(_ref4) {
	  var message = _ref4.message;
	  return _react2.default.createElement(
	    Box,
	    { iconClass: 'fa fa-frown-o' },
	    _react2.default.createElement(
	      'h1',
	      null,
	      MSG_ERROR_ACCESS_DENIED
	    ),
	    _react2.default.createElement(ErrorBoxDetails, { message: message })
	  );
	};

	var Failed = function Failed(_ref5) {
	  var message = _ref5.message;
	  return _react2.default.createElement(
	    ErrorBox,
	    null,
	    _react2.default.createElement(
	      'h1',
	      null,
	      MSG_ERROR_DEFAULT
	    ),
	    _react2.default.createElement(ErrorBoxDetails, { message: message })
	  );
	};

	var ExpiredLink = function ExpiredLink() {
	  return _react2.default.createElement(
	    ErrorBox,
	    null,
	    _react2.default.createElement(
	      'h1',
	      null,
	      MSG_ERROR_EXPIRED_INVITE
	    ),
	    _react2.default.createElement(ErrorBoxDetails, { message: MSG_ERROR_EXPIRED_INVITE_DETAILS })
	  );
	};

	var LoginFailed = function LoginFailed(_ref6) {
	  var message = _ref6.message;
	  return _react2.default.createElement(
	    ErrorBox,
	    null,
	    _react2.default.createElement(
	      'h1',
	      null,
	      MSG_ERROR_LOGIN_FAILED
	    ),
	    _react2.default.createElement(ErrorBoxDetails, { message: message })
	  );
	};

	var SuccessfulLogin = function SuccessfulLogin() {
	  return _react2.default.createElement(
	    InfoBox,
	    null,
	    _react2.default.createElement(
	      'h1',
	      null,
	      MSG_INFO_LOGIN_SUCCESS
	    )
	  );
	};

	exports.ErrorPage = ErrorPage;
	exports.InfoPage = InfoPage;
	exports.NotFound = NotFound;
	exports.Failed = Failed;
	exports.AccessDenied = AccessDenied;
	exports.ExpiredLink = ExpiredLink;

/***/ }),
/* 267 */
/***/ (function(module, exports) {

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

	var DEFAULT_TITLE = 'Teleport by Gravitational';

	var DocumentTitle = function DocumentTitle(props) {
	  var title = DEFAULT_TITLE;
	  var routes = props.routes || [];
	  for (var i = routes.length - 1; i > 0; i--) {
	    if (routes[i].title) {
	      title = routes[i].title;
	      break;
	    }
	  }

	  document.title = title;

	  return props.children;
	};

	exports.default = DocumentTitle;
	module.exports = exports['default'];

/***/ }),
/* 268 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.Settings = exports.Audit = exports.Ssh = undefined;

	var _featureSsh = __webpack_require__(269);

	var _featureSsh2 = _interopRequireDefault(_featureSsh);

	var _featureAudit = __webpack_require__(420);

	var _featureAudit2 = _interopRequireDefault(_featureAudit);

	var _featureSettings = __webpack_require__(518);

	var _featureSettings2 = _interopRequireDefault(_featureSettings);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	exports.Ssh = _featureSsh2.default;
	exports.Audit = _featureAudit2.default;
	exports.Settings = _featureSettings2.default; /*
	                                              Copyright 2015 Gravitational, Inc.
	                                              
	                                              Licensed under the Apache License, Version 2.0 (the "License");
	                                              you may not use this file except in compliance with the License.
	                                              You may obtain a copy of the License at
	                                              
	                                                  http://www.apache.org/licenses/LICENSE-2.0
	                                              
	                                              Unless required by applicable law or agreed to in writing, software
	                                              distributed under the License is distributed on an "AS IS" BASIS,
	                                              WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                              See the License for the specific language governing permissions and
	                                              limitations under the License.
	                                              */

/***/ }),
/* 269 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _config = __webpack_require__(228);

	var _config2 = _interopRequireDefault(_config);

	var _main = __webpack_require__(270);

	var _main2 = _interopRequireDefault(_main);

	var _featureBase = __webpack_require__(393);

	var _featureBase2 = _interopRequireDefault(_featureBase);

	var _actions = __webpack_require__(284);

	var _terminalHost = __webpack_require__(396);

	var _terminalHost2 = _interopRequireDefault(_terminalHost);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
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
	  path: _config2.default.routes.nodes,
	  title: "Nodes",
	  component: _main2.default
	}, {
	  path: _config2.default.routes.terminal,
	  title: "Terminal",
	  components: {
	    CurrentSessionHost: _terminalHost2.default
	  }
	}];

	var sshNavItem = {
	  icon: 'fa fa-share-alt',
	  to: _config2.default.routes.nodes,
	  title: 'Nodes'
	};

	var SshFeature = function (_FeatureBase) {
	  _inherits(SshFeature, _FeatureBase);

	  function SshFeature(routes) {
	    _classCallCheck(this, SshFeature);

	    var _this = _possibleConstructorReturn(this, _FeatureBase.call(this));

	    routes.push.apply(routes, sshRoutes);
	    return _this;
	  }

	  SshFeature.prototype.onload = function onload() {
	    (0, _actions.addNavItem)(sshNavItem);
	  };

	  return SshFeature;
	}(_featureBase2.default);

	exports.default = SshFeature;
	module.exports = exports['default'];

/***/ }),
/* 270 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _nuclearJsReactAddons = __webpack_require__(219);

	var _getters = __webpack_require__(271);

	var _getters2 = _interopRequireDefault(_getters);

	var _getters3 = __webpack_require__(272);

	var _getters4 = _interopRequireDefault(_getters3);

	var _getters5 = __webpack_require__(273);

	var _getters6 = _interopRequireDefault(_getters5);

	var _nodeList = __webpack_require__(274);

	var _nodeList2 = _interopRequireDefault(_nodeList);

	var _withStorage = __webpack_require__(391);

	var _withStorage2 = _interopRequireDefault(_withStorage);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	var Nodes = function Nodes(props) {
	  var siteNodes = props.siteNodes,
	      aclStore = props.aclStore,
	      sites = props.sites,
	      siteId = props.siteId,
	      storage = props.storage;

	  var logins = aclStore.getSshLogins().toJS();
	  var nodeRecords = siteNodes.toJS();
	  return _react2.default.createElement(
	    'div',
	    { className: 'grv-page' },
	    _react2.default.createElement(_nodeList2.default, {
	      storage: storage,
	      siteId: siteId,
	      sites: sites,
	      nodeRecords: nodeRecords,
	      logins: logins
	    })
	  );
	}; /*
	   Copyright 2015 Gravitational, Inc.
	   
	   Licensed under the Apache License, Version 2.0 (the "License");
	   you may not use this file except in compliance with the License.
	   You may obtain a copy of the License at
	   
	       http://www.apache.org/licenses/LICENSE-2.0
	   
	   Unless required by applicable law or agreed to in writing, software
	   distributed under the License is distributed on an "AS IS" BASIS,
	   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	   See the License for the specific language governing permissions and
	   limitations under the License.
	   */

	function mapStateToProps() {
	  return {
	    siteId: _getters6.default.siteId,
	    siteNodes: _getters4.default.siteNodes,
	    aclStore: _getters2.default.userAcl
	  };
	}

	var NodesWithStorage = (0, _withStorage2.default)(Nodes);

	exports.default = (0, _nuclearJsReactAddons.connect)(mapStateToProps)(NodesWithStorage);
	module.exports = exports['default'];

/***/ }),
/* 271 */
/***/ (function(module, exports) {

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

	var userAcl = ['tlpt_user_acl'];

	exports.default = {
	  userAcl: userAcl
	};
	module.exports = exports['default'];

/***/ }),
/* 272 */
/***/ (function(module, exports) {

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

	var siteNodes = [['tlpt_nodes'], ['tlpt', 'siteId'], function (nodeStore, siteId) {
	  return nodeStore.getSiteServers(siteId);
	}];

	exports.default = {
	  siteNodes: siteNodes
	};
	module.exports = exports['default'];

/***/ }),
/* 273 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _getters = __webpack_require__(251);

	exports.default = {
	  initAttempt: _getters.initAppAttempt,
	  siteId: ['tlpt', 'siteId']
	}; /*
	   Copyright 2015 Gravitational, Inc.
	   
	   Licensed under the Apache License, Version 2.0 (the "License");
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

/***/ }),
/* 274 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _reactRouter = __webpack_require__(164);

	var _lodash = __webpack_require__(275);

	var _objectUtils = __webpack_require__(277);

	var _inputSearch = __webpack_require__(278);

	var _inputSearch2 = _interopRequireDefault(_inputSearch);

	var _inputSshServer = __webpack_require__(279);

	var _inputSshServer2 = _interopRequireDefault(_inputSshServer);

	var _table = __webpack_require__(280);

	var _clusterSelector = __webpack_require__(281);

	var _clusterSelector2 = _interopRequireDefault(_clusterSelector);

	var _config = __webpack_require__(228);

	var _config2 = _interopRequireDefault(_config);

	var _history = __webpack_require__(226);

	var _history2 = _interopRequireDefault(_history);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; } /*
	                                                                                                                                                                                                                             Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                             
	                                                                                                                                                                                                                             Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                             you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                             You may obtain a copy of the License at
	                                                                                                                                                                                                                             
	                                                                                                                                                                                                                                 http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                             
	                                                                                                                                                                                                                             Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                             distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                             WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                             See the License for the specific language governing permissions and
	                                                                                                                                                                                                                             limitations under the License.
	                                                                                                                                                                                                                             */

	var EmptyValue = function EmptyValue(_ref) {
	  var _ref$text = _ref.text,
	      text = _ref$text === undefined ? 'Empty' : _ref$text;
	  return _react2.default.createElement(
	    'small',
	    { className: 'text-muted' },
	    _react2.default.createElement(
	      'span',
	      null,
	      text
	    )
	  );
	};

	var TagCell = function TagCell(_ref2) {
	  var rowIndex = _ref2.rowIndex,
	      data = _ref2.data,
	      props = _objectWithoutProperties(_ref2, ['rowIndex', 'data']);

	  var tags = data[rowIndex].tags;

	  var $content = tags.map(function (item, index) {
	    return _react2.default.createElement(
	      'span',
	      { key: index, title: item.name + ':' + item.value, className: 'label label-default grv-nodes-table-label' },
	      item.name,
	      ' ',
	      _react2.default.createElement('li', { className: 'fa fa-long-arrow-right m-r-xs' }),
	      item.value
	    );
	  });

	  if ($content.length === 0) {
	    $content = _react2.default.createElement(EmptyValue, { text: 'No assigned labels' });
	  }

	  return _react2.default.createElement(
	    _table.Cell,
	    props,
	    $content
	  );
	};

	var LoginCell = function (_React$Component) {
	  _inherits(LoginCell, _React$Component);

	  function LoginCell() {
	    var _temp, _this, _ret;

	    _classCallCheck(this, LoginCell);

	    for (var _len = arguments.length, args = Array(_len), _key = 0; _key < _len; _key++) {
	      args[_key] = arguments[_key];
	    }

	    return _ret = (_temp = (_this = _possibleConstructorReturn(this, _React$Component.call.apply(_React$Component, [this].concat(args))), _this), _this.onKeyPress = function (e) {
	      if (e.key === 'Enter' && e.target.value) {
	        var url = _this.makeUrl(e.target.value);
	        _history2.default.push(url);
	      }
	    }, _this.onShowLoginsClick = function () {
	      _this.refs.customLogin.focus();
	    }, _temp), _possibleConstructorReturn(_this, _ret);
	  }

	  LoginCell.prototype.makeUrl = function makeUrl(login) {
	    var _props = this.props,
	        data = _props.data,
	        rowIndex = _props.rowIndex;
	    var _data$rowIndex = data[rowIndex],
	        siteId = _data$rowIndex.siteId,
	        hostname = _data$rowIndex.hostname;

	    return _config2.default.getTerminalLoginUrl({
	      siteId: siteId,
	      serverId: hostname,
	      login: login
	    });
	  };

	  LoginCell.prototype.render = function render() {
	    var _props2 = this.props,
	        logins = _props2.logins,
	        props = _objectWithoutProperties(_props2, ['logins']);

	    var $lis = [];
	    var defaultLogin = logins[0] || '';
	    var defaultTermUrl = this.makeUrl(defaultLogin);

	    for (var i = 0; i < logins.length; i++) {
	      var termUrl = this.makeUrl(logins[i]);
	      $lis.push(_react2.default.createElement(
	        'li',
	        { key: i },
	        _react2.default.createElement(
	          _reactRouter.Link,
	          { to: termUrl },
	          logins[i]
	        )
	      ));
	    }

	    return _react2.default.createElement(
	      _table.Cell,
	      props,
	      _react2.default.createElement(
	        'div',
	        { style: { display: "flex" } },
	        _react2.default.createElement(
	          'div',
	          { style: { display: "flex" }, className: 'btn-group' },
	          logins.length > 0 && _react2.default.createElement(
	            _reactRouter.Link,
	            { className: 'btn btn-xs btn-primary', to: defaultTermUrl },
	            defaultLogin
	          ),
	          logins.length === 0 && _react2.default.createElement(
	            'div',
	            { className: 'btn btn-xs btn-white' },
	            _react2.default.createElement(
	              'span',
	              { className: 'text-muted' },
	              ' Empty '
	            )
	          ),
	          _react2.default.createElement(
	            'button',
	            { 'data-toggle': 'dropdown',
	              onClick: this.onShowLoginsClick,
	              className: 'btn btn-default btn-xs dropdown-toggle' },
	            _react2.default.createElement('span', { className: 'caret' })
	          ),
	          _react2.default.createElement(
	            'ul',
	            { className: 'dropdown-menu pull-right' },
	            _react2.default.createElement(
	              'li',
	              null,
	              _react2.default.createElement(
	                'div',
	                { className: 'input-group-sm grv-nodes-custom-login' },
	                _react2.default.createElement('input', { className: 'form-control', ref: 'customLogin',
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
	  };

	  return LoginCell;
	}(_react2.default.Component);

	var NodeList = function (_React$Component2) {
	  _inherits(NodeList, _React$Component2);

	  function NodeList(props) {
	    _classCallCheck(this, NodeList);

	    var _this2 = _possibleConstructorReturn(this, _React$Component2.call(this, props));

	    _this2.storageKey = 'NodeList';
	    _this2.searchableProps = ['addr', 'hostname', 'tags'];

	    _this2.onSortChange = function (columnKey, sortDir) {
	      var _this2$state$colSortD;

	      _this2.state.colSortDirs = (_this2$state$colSortD = {}, _this2$state$colSortD[columnKey] = sortDir, _this2$state$colSortD);
	      _this2.setState(_this2.state);
	    };

	    _this2.onFilterChange = function (value) {
	      _this2.state.filter = value;
	      _this2.setState(_this2.state);
	    };

	    _this2.onSshInputEnter = function (login, host) {
	      var url = _config2.default.getTerminalLoginUrl({
	        siteId: _this2.props.siteId,
	        serverId: host,
	        login: login
	      });

	      _history2.default.push(url);
	    };

	    if (props.storage) {
	      _this2.state = props.storage.findByKey(_this2.storageKey);
	    }

	    if (!_this2.state) {
	      _this2.state = { filter: '', colSortDirs: { hostname: _table.SortTypes.DESC } };
	    }
	    return _this2;
	  }

	  NodeList.prototype.componentWillUnmount = function componentWillUnmount() {
	    if (this.props.storage) {
	      this.props.storage.save(this.storageKey, this.state);
	    }
	  };

	  NodeList.prototype.searchAndFilterCb = function searchAndFilterCb(targetValue, searchValue, propName) {
	    if (propName === 'tags') {
	      return targetValue.some(function (item) {
	        var name = item.name,
	            value = item.value;

	        return name.toLocaleUpperCase().indexOf(searchValue) !== -1 || value.toLocaleUpperCase().indexOf(searchValue) !== -1;
	      });
	    }
	  };

	  NodeList.prototype.sortAndFilter = function sortAndFilter(data) {
	    var _this3 = this;

	    var colSortDirs = this.state.colSortDirs;

	    var filtered = data.filter(function (obj) {
	      return (0, _objectUtils.isMatch)(obj, _this3.state.filter, {
	        searchableProps: _this3.searchableProps,
	        cb: _this3.searchAndFilterCb
	      });
	    });

	    var columnKey = Object.getOwnPropertyNames(colSortDirs)[0];
	    var sortDir = colSortDirs[columnKey];
	    var sorted = (0, _lodash.sortBy)(filtered, columnKey);
	    if (sortDir === _table.SortTypes.ASC) {
	      sorted = sorted.reverse();
	    }

	    return sorted;
	  };

	  NodeList.prototype.render = function render() {
	    var _props3 = this.props,
	        nodeRecords = _props3.nodeRecords,
	        logins = _props3.logins,
	        onLoginClick = _props3.onLoginClick;

	    var searchValue = this.state.filter;
	    var data = this.sortAndFilter(nodeRecords);
	    return _react2.default.createElement(
	      'div',
	      { className: 'grv-nodes m-t' },
	      _react2.default.createElement(
	        'div',
	        { className: 'grv-flex grv-header', style: { justifyContent: "space-between" } },
	        _react2.default.createElement(
	          'h2',
	          { className: 'text-center no-margins' },
	          ' Nodes '
	        ),
	        _react2.default.createElement(
	          'div',
	          { className: 'grv-flex' },
	          _react2.default.createElement(_clusterSelector2.default, null),
	          _react2.default.createElement(_inputSearch2.default, { value: searchValue, onChange: this.onFilterChange }),
	          _react2.default.createElement(_inputSshServer2.default, { onEnter: this.onSshInputEnter })
	        )
	      ),
	      _react2.default.createElement(
	        'div',
	        { className: 'm-t' },
	        data.length === 0 && this.state.filter.length > 0 ? _react2.default.createElement(_table.EmptyIndicator, { text: 'No matching nodes found' }) : _react2.default.createElement(
	          _table.Table,
	          { rowCount: data.length, className: 'table-striped grv-nodes-table' },
	          _react2.default.createElement(_table.Column, {
	            columnKey: 'hostname',
	            header: _react2.default.createElement(_table.SortHeaderCell, {
	              sortDir: this.state.colSortDirs.hostname,
	              onSortChange: this.onSortChange,
	              title: 'Hostname'
	            }),
	            cell: _react2.default.createElement(_table.TextCell, { data: data })
	          }),
	          _react2.default.createElement(_table.Column, {
	            columnKey: 'addr',
	            header: _react2.default.createElement(_table.SortHeaderCell, {
	              sortDir: this.state.colSortDirs.addr,
	              onSortChange: this.onSortChange,
	              title: 'Address'
	            }),
	            cell: _react2.default.createElement(_table.TextCell, { data: data })
	          }),
	          _react2.default.createElement(_table.Column, {
	            header: _react2.default.createElement(
	              _table.Cell,
	              null,
	              'Labels'
	            ),
	            cell: _react2.default.createElement(TagCell, { data: data })
	          }),
	          _react2.default.createElement(_table.Column, {
	            onLoginClick: onLoginClick,
	            header: _react2.default.createElement(
	              _table.Cell,
	              null,
	              'Login as'
	            ),
	            cell: _react2.default.createElement(LoginCell, { data: data, logins: logins })
	          })
	        )
	      )
	    );
	  };

	  return NodeList;
	}(_react2.default.Component);

	exports.default = NodeList;
	module.exports = exports['default'];

/***/ }),
/* 275 */,
/* 276 */,
/* 277 */
/***/ (function(module, exports) {

	'use strict';

	exports.__esModule = true;
	exports.parseIp = parseIp;
	exports.isMatch = isMatch;
	exports.isUUID = isUUID;
	/*
	Copyright 2015 Gravitational, Inc.

	Licensed under the Apache License, Version 2.0 (the "License");
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

/***/ }),
/* 278 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _reactDom = __webpack_require__(31);

	var _reactDom2 = _interopRequireDefault(_reactDom);

	var _lodash = __webpack_require__(275);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               limitations under the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               */

	var InputSearch = function (_React$Component) {
	  _inherits(InputSearch, _React$Component);

	  function InputSearch(props) {
	    _classCallCheck(this, InputSearch);

	    var _this = _possibleConstructorReturn(this, _React$Component.call(this, props));

	    _this.onChange = function (e) {
	      _this.setState({ value: e.target.value });
	      _this.debouncedNotify();
	    };

	    _this.debouncedNotify = (0, _lodash.debounce)(function () {
	      _this.props.onChange(_this.state.value);
	    }, 200);

	    var value = props.value || '';

	    _this.state = {
	      value: value
	    };
	    return _this;
	  }

	  InputSearch.prototype.componentDidMount = function componentDidMount() {
	    // set cursor
	    var $el = _reactDom2.default.findDOMNode(this);
	    if ($el) {
	      var $input = $el.querySelector('input');
	      var length = $input.value.length;
	      $input.selectionEnd = length;
	      $input.selectionStart = length;
	    }
	  };

	  InputSearch.prototype.render = function render() {
	    var _props$className = this.props.className,
	        className = _props$className === undefined ? '' : _props$className;

	    className = 'grv-search input-group-sm ' + className;

	    return _react2.default.createElement(
	      'div',
	      { className: className },
	      _react2.default.createElement('input', { placeholder: 'Search...', className: 'form-control',
	        autoFocus: true,
	        value: this.state.value,
	        onChange: this.onChange })
	    );
	  };

	  return InputSearch;
	}(_react2.default.Component);

	exports.default = InputSearch;
	module.exports = exports['default'];

/***/ }),
/* 279 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _classnames = __webpack_require__(257);

	var _classnames2 = _interopRequireDefault(_classnames);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               limitations under the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               */

	var SSH_STR_REGEX = /(^\w+\@(\w|\.|\-)+(:\d+)*$)|(^$)/;
	var PLACEHOLDER_TEXT = 'login@host';

	var InputSshServer = function (_React$Component) {
	  _inherits(InputSshServer, _React$Component);

	  function InputSshServer() {
	    var _temp, _this, _ret;

	    _classCallCheck(this, InputSshServer);

	    for (var _len = arguments.length, args = Array(_len), _key = 0; _key < _len; _key++) {
	      args[_key] = arguments[_key];
	    }

	    return _ret = (_temp = (_this = _possibleConstructorReturn(this, _React$Component.call.apply(_React$Component, [this].concat(args))), _this), _this.state = {
	      hasErrors: false
	    }, _this.onChange = function (e) {
	      var value = e.target.value;
	      var isValid = _this.isValid(value);
	      if (isValid && _this.state.hasErrors === true) {
	        _this.setState({ hasErrors: false });
	      }
	    }, _this.onKeyPress = function (e) {
	      var value = e.target.value;
	      var isValid = _this.isValid(value);
	      if ((e.key === 'Enter' || e.type === 'click') && value) {
	        _this.setState({ hasErrors: !isValid });
	        if (isValid) {
	          var _value$split = value.split('@'),
	              login = _value$split[0],
	              host = _value$split[1];

	          _this.props.onEnter(login, host);
	        }
	      }
	    }, _temp), _possibleConstructorReturn(_this, _ret);
	  }

	  InputSshServer.prototype.isValid = function isValid(value) {
	    var match = SSH_STR_REGEX.exec(value);
	    return !!match;
	  };

	  InputSshServer.prototype.render = function render() {
	    var className = (0, _classnames2.default)('grv-sshserver-input', { '--error': this.state.hasErrors });
	    return _react2.default.createElement(
	      'div',
	      { className: className },
	      _react2.default.createElement(
	        'div',
	        { className: 'm-l input-group input-group-sm', title: 'login to SSH server' },
	        _react2.default.createElement('input', { className: 'form-control',
	          placeholder: PLACEHOLDER_TEXT,
	          onChange: this.onChange,
	          onKeyPress: this.onKeyPress
	        }),
	        _react2.default.createElement(
	          'span',
	          { className: 'input-group-btn' },
	          _react2.default.createElement(
	            'button',
	            { className: 'btn btn-sm btn-white', onClick: this.onKeyPress },
	            _react2.default.createElement('i', { className: 'fa fa-terminal text-muted' })
	          )
	        )
	      ),
	      _react2.default.createElement(
	        'label',
	        { className: 'm-l grv-sshserver-input-errors' },
	        ' Invalid format '
	      )
	    );
	  };

	  return InputSshServer;
	}(_react2.default.Component);

	exports.default = InputSshServer;
	module.exports = exports['default'];

/***/ }),
/* 280 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.EmptyIndicator = exports.SortTypes = exports.SortIndicator = exports.SortHeaderCell = exports.TextCell = exports.Cell = exports.Table = exports.Column = undefined;

	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; } /*
	                                                                                                                                                                                                                             Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                             
	                                                                                                                                                                                                                             Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                             you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                             You may obtain a copy of the License at
	                                                                                                                                                                                                                             
	                                                                                                                                                                                                                                 http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                             
	                                                                                                                                                                                                                             Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                             distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                             WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                             See the License for the specific language governing permissions and
	                                                                                                                                                                                                                             limitations under the License.
	                                                                                                                                                                                                                             */

	var GrvTableTextCell = function GrvTableTextCell(_ref) {
	  var rowIndex = _ref.rowIndex,
	      data = _ref.data,
	      columnKey = _ref.columnKey,
	      props = _objectWithoutProperties(_ref, ['rowIndex', 'data', 'columnKey']);

	  return _react2.default.createElement(
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

	  return _react2.default.createElement('i', { className: cls });
	};

	/**
	* Sort Header Cell
	*/
	var SortHeaderCell = _react2.default.createClass({
	  displayName: 'SortHeaderCell',
	  render: function render() {
	    var _props = this.props,
	        sortDir = _props.sortDir,
	        title = _props.title,
	        props = _objectWithoutProperties(_props, ['sortDir', 'title']);

	    return _react2.default.createElement(
	      GrvTableCell,
	      props,
	      _react2.default.createElement(
	        'a',
	        { onClick: this.onSortChange },
	        title
	      ),
	      _react2.default.createElement(SortIndicator, { sortDir: sortDir })
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
	var GrvTableCell = _react2.default.createClass({
	  displayName: 'GrvTableCell',
	  render: function render() {
	    var _props2 = this.props,
	        isHeader = _props2.isHeader,
	        children = _props2.children,
	        _props2$className = _props2.className,
	        className = _props2$className === undefined ? '' : _props2$className;

	    className = 'grv-table-cell ' + className;
	    return isHeader ? _react2.default.createElement(
	      'th',
	      { className: className },
	      children
	    ) : _react2.default.createElement(
	      'td',
	      null,
	      children
	    );
	  }
	});

	/**
	* Table
	*/
	var GrvTable = _react2.default.createClass({
	  displayName: 'GrvTable',
	  renderHeader: function renderHeader(children) {
	    var _this = this;

	    var cells = children.map(function (item, index) {
	      return _this.renderCell(item.props.header, _extends({ index: index, key: index, isHeader: true }, item.props));
	    });

	    return _react2.default.createElement(
	      'thead',
	      { className: 'grv-table-header' },
	      _react2.default.createElement(
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

	      rows.push(_react2.default.createElement(
	        'tr',
	        { key: i },
	        cells
	      ));
	    }

	    return _react2.default.createElement(
	      'tbody',
	      null,
	      rows
	    );
	  },
	  renderCell: function renderCell(cell, cellProps) {
	    var content = null;
	    if (_react2.default.isValidElement(cell)) {
	      content = _react2.default.cloneElement(cell, cellProps);
	    } else if (typeof cell === 'function') {
	      content = cell(cellProps);
	    }

	    return content;
	  },
	  render: function render() {
	    var children = [];
	    _react2.default.Children.forEach(this.props.children, function (child) {
	      if (child == null) {
	        return;
	      }

	      if (child.type.displayName !== 'GrvTableColumn') {
	        throw 'Should be GrvTableColumn';
	      }

	      children.push(child);
	    });

	    var tableClass = 'table grv-table ' + this.props.className;

	    return _react2.default.createElement(
	      'table',
	      { className: tableClass },
	      this.renderHeader(children),
	      this.renderBody(children)
	    );
	  }
	});

	var GrvTableColumn = _react2.default.createClass({
	  displayName: 'GrvTableColumn',

	  render: function render() {
	    throw new Error('Component <GrvTableColumn /> should never render');
	  }
	});

	var EmptyIndicator = function EmptyIndicator(_ref3) {
	  var text = _ref3.text;
	  return _react2.default.createElement(
	    'div',
	    { className: 'grv-table-indicator-empty text-muted' },
	    _react2.default.createElement(
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

/***/ }),
/* 281 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _reactor = __webpack_require__(233);

	var _reactor2 = _interopRequireDefault(_reactor);

	var _getters = __webpack_require__(282);

	var _getters2 = _interopRequireDefault(_getters);

	var _getters3 = __webpack_require__(273);

	var _getters4 = _interopRequireDefault(_getters3);

	var _dropdown = __webpack_require__(283);

	var _dropdown2 = _interopRequireDefault(_dropdown);

	var _actions = __webpack_require__(284);

	var _objectUtils = __webpack_require__(277);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	var ClusterSelector = _react2.default.createClass({
	  displayName: 'ClusterSelector',


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


	    var siteOptions = sites.map(function (s) {
	      return {
	        label: s.name,
	        value: s.name
	      };
	    });

	    if (siteOptions.length === 1 && (0, _objectUtils.isUUID)(siteOptions[0].value)) {
	      siteOptions[0].label = location.hostname;
	    }

	    return _react2.default.createElement(
	      'div',
	      { className: 'grv-clusters-selector' },
	      _react2.default.createElement(
	        'div',
	        { className: 'm-r-sm' },
	        'Cluster:'
	      ),
	      _react2.default.createElement(_dropdown2.default, {
	        className: 'm-r-sm',
	        size: 'sm',
	        align: 'right',
	        onChange: this.onChangeSite,
	        value: siteId,
	        options: siteOptions
	      })
	    );
	  }
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

	exports.default = ClusterSelector;
	module.exports = exports['default'];

/***/ }),
/* 282 */
/***/ (function(module, exports) {

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

/***/ }),
/* 283 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _jQuery = __webpack_require__(229);

	var _jQuery2 = _interopRequireDefault(_jQuery);

	var _lodash = __webpack_require__(275);

	var _classnames = __webpack_require__(257);

	var _classnames2 = _interopRequireDefault(_classnames);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               limitations under the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               */

	var DropDown = function (_React$Component) {
	  _inherits(DropDown, _React$Component);

	  function DropDown() {
	    var _temp, _this, _ret;

	    _classCallCheck(this, DropDown);

	    for (var _len = arguments.length, args = Array(_len), _key = 0; _key < _len; _key++) {
	      args[_key] = arguments[_key];
	    }

	    return _ret = (_temp = (_this = _possibleConstructorReturn(this, _React$Component.call.apply(_React$Component, [this].concat(args))), _this), _this.onClick = function (event) {
	      event.preventDefault();
	      var options = _this.props.options;

	      var index = (0, _jQuery2.default)(event.target).parent().index();
	      var option = options[index];
	      var value = (0, _lodash.isObject)(option) ? option.value : option;

	      _this.props.onChange(value);
	    }, _temp), _possibleConstructorReturn(_this, _ret);
	  }

	  DropDown.prototype.renderOption = function renderOption(option, index) {
	    var displayValue = (0, _lodash.isObject)(option) ? option.label : option;
	    return _react2.default.createElement(
	      'li',
	      { key: index },
	      _react2.default.createElement(
	        'a',
	        { href: '#' },
	        displayValue
	      )
	    );
	  };

	  DropDown.prototype.getDisplayValue = function getDisplayValue(value) {
	    var _props$options = this.props.options,
	        options = _props$options === undefined ? [] : _props$options;

	    for (var i = 0; i < options.length; i++) {
	      var op = options[i];
	      if ((0, _lodash.isObject)(op) && op.value === value) {
	        return op.label;
	      }

	      if (op === value) {
	        return value;
	      }
	    }

	    return null;
	  };

	  DropDown.prototype.render = function render() {
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

	    var valueClass = (0, _classnames2.default)('grv-dropdown-value', {
	      'text-muted': !hiddenValue
	    });

	    var mainClass = 'grv-dropdown ' + className;

	    var btnClass = (0, _classnames2.default)('btn btn-default full-width dropdown-toggle', {
	      'btn-sm': size === 'sm'
	    });

	    var menuClass = (0, _classnames2.default)('dropdown-menu', {
	      'pull-right': align === 'right'
	    });

	    var $menu = options.length > 0 ? _react2.default.createElement(
	      'ul',
	      { onClick: this.onClick, className: menuClass },
	      $options
	    ) : null;

	    return _react2.default.createElement(
	      'div',
	      { className: mainClass },
	      _react2.default.createElement(
	        'div',
	        { className: 'dropdown' },
	        _react2.default.createElement(
	          'div',
	          { className: btnClass, type: 'button', 'data-toggle': 'dropdown', 'aria-haspopup': 'true', 'aria-expanded': 'true' },
	          _react2.default.createElement(
	            'div',
	            { className: valueClass },
	            _react2.default.createElement(
	              'span',
	              { style: { textOverflow: "ellipsis", overflow: "hidden" } },
	              displayValue
	            ),
	            _react2.default.createElement('span', { className: 'caret m-l-sm' })
	          )
	        ),
	        $menu
	      ),
	      _react2.default.createElement('input', { className: classRules, value: hiddenValue, type: 'hidden', ref: 'input', name: name })
	    );
	  };

	  return DropDown;
	}(_react2.default.Component);

	exports.default = DropDown;
	module.exports = exports['default'];

/***/ }),
/* 284 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.addNavItem = addNavItem;
	exports.setSiteId = setSiteId;
	exports.initApp = initApp;
	exports.refresh = refresh;
	exports.fetchInitData = fetchInitData;
	exports.fetchSites = fetchSites;
	exports.fetchUserContext = fetchUserContext;

	var _jQuery = __webpack_require__(229);

	var _jQuery2 = _interopRequireDefault(_jQuery);

	var _reactor = __webpack_require__(233);

	var _reactor2 = _interopRequireDefault(_reactor);

	var _actionTypes = __webpack_require__(285);

	var _actionTypes2 = __webpack_require__(286);

	var _actionTypes3 = __webpack_require__(249);

	var _actionTypes4 = __webpack_require__(287);

	var _api = __webpack_require__(241);

	var _api2 = _interopRequireDefault(_api);

	var _config = __webpack_require__(228);

	var _config2 = _interopRequireDefault(_config);

	var _actions = __webpack_require__(246);

	var _actions2 = __webpack_require__(288);

	var _actions3 = __webpack_require__(290);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	var logger = __webpack_require__(245).create('flux/app'); /*
	                                                           Copyright 2015 Gravitational, Inc.
	                                                           
	                                                           Licensed under the Apache License, Version 2.0 (the "License");
	                                                           you may not use this file except in compliance with the License.
	                                                           You may obtain a copy of the License at
	                                                           
	                                                               http://www.apache.org/licenses/LICENSE-2.0
	                                                           
	                                                           Unless required by applicable law or agreed to in writing, software
	                                                           distributed under the License is distributed on an "AS IS" BASIS,
	                                                           WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                           See the License for the specific language governing permissions and
	                                                           limitations under the License.
	                                                           */

	function addNavItem(item) {
	  _reactor2.default.dispatch(_actionTypes.ADD_NAV_ITEM, item);
	}

	function setSiteId(siteId) {
	  _reactor2.default.dispatch(_actionTypes.SET_SITE_ID, siteId);
	}

	function initApp(siteId, featureActivator) {
	  _actions.initAppStatus.start();
	  // get the list of available clusters        
	  return fetchInitData(siteId).done(function () {
	    featureActivator.onload();
	    _actions.initAppStatus.success();
	  }).fail(function (err) {
	    var msg = _api2.default.getErrorText(err);
	    _actions.initAppStatus.fail(msg);
	  });
	}

	function refresh() {
	  return _jQuery2.default.when((0, _actions3.fetchActiveSessions)(), (0, _actions2.fetchNodes)());
	}

	function fetchInitData(siteId) {
	  return _jQuery2.default.when(fetchSites(), fetchUserContext()).then(function (masterSiteId) {
	    var selectedCluster = siteId || masterSiteId;
	    setSiteId(selectedCluster);
	    return _jQuery2.default.when((0, _actions2.fetchNodes)(), (0, _actions3.fetchActiveSessions)());
	  });
	}

	function fetchSites() {
	  return _api2.default.get(_config2.default.api.sitesBasePath).then(function (json) {
	    var masterSiteId = null;
	    var sites = json.sites;
	    if (sites) {
	      masterSiteId = sites[0].name;
	    }

	    _reactor2.default.dispatch(_actionTypes2.RECEIVE_CLUSTERS, sites);

	    return masterSiteId;
	  }).fail(function (err) {
	    logger.error('fetchSites', err);
	  });
	}

	function fetchUserContext() {
	  return _api2.default.get(_config2.default.api.userContextPath).done(function (json) {
	    _reactor2.default.dispatch(_actionTypes3.RECEIVE_USER, { name: json.userName, authType: json.authType });
	    _reactor2.default.dispatch(_actionTypes4.RECEIVE_USERACL, json.userAcl);
	  });
	}

/***/ }),
/* 285 */
/***/ (function(module, exports) {

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

	var SET_SITE_ID = exports.SET_SITE_ID = 'TLPT_APP_SET_SITE_ID';
	var ADD_NAV_ITEM = exports.ADD_NAV_ITEM = 'TLPT_APP_ADD_NAV_ITEM';

/***/ }),
/* 286 */
/***/ (function(module, exports) {

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

	var RECEIVE_CLUSTERS = exports.RECEIVE_CLUSTERS = 'TLPT_CLUSTER_RECEIVE';

/***/ }),
/* 287 */
/***/ (function(module, exports) {

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

	var RECEIVE_USERACL = exports.RECEIVE_USERACL = 'TLPT_USERACL_RECEIVE';

/***/ }),
/* 288 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _reactor = __webpack_require__(233);

	var _reactor2 = _interopRequireDefault(_reactor);

	var _actionTypes = __webpack_require__(289);

	var _api = __webpack_require__(241);

	var _api2 = _interopRequireDefault(_api);

	var _config = __webpack_require__(228);

	var _config2 = _interopRequireDefault(_config);

	var _getters = __webpack_require__(273);

	var _getters2 = _interopRequireDefault(_getters);

	var _logger = __webpack_require__(245);

	var _logger2 = _interopRequireDefault(_logger);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	var logger = _logger2.default.create('Modules/Nodes');

	exports.default = {
	  fetchNodes: function fetchNodes() {
	    var siteId = _reactor2.default.evaluate(_getters2.default.siteId);
	    return _api2.default.get(_config2.default.api.getSiteNodesUrl(siteId)).then(function (res) {
	      return res.items || [];
	    }).done(function (items) {
	      return _reactor2.default.dispatch(_actionTypes.TLPT_NODES_RECEIVE, items);
	    }).fail(function (err) {
	      return logger.error('fetchNodes', err);
	    });
	  }
	};
	module.exports = exports['default'];

/***/ }),
/* 289 */
/***/ (function(module, exports) {

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

	var TLPT_NODES_RECEIVE = exports.TLPT_NODES_RECEIVE = 'TLPT_NODES_RECEIVE';

/***/ }),
/* 290 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _reactor = __webpack_require__(233);

	var _reactor2 = _interopRequireDefault(_reactor);

	var _api = __webpack_require__(241);

	var _api2 = _interopRequireDefault(_api);

	var _config = __webpack_require__(228);

	var _config2 = _interopRequireDefault(_config);

	var _moment = __webpack_require__(291);

	var _moment2 = _interopRequireDefault(_moment);

	var _getters = __webpack_require__(273);

	var _getters2 = _interopRequireDefault(_getters);

	var _logger = __webpack_require__(245);

	var _logger2 = _interopRequireDefault(_logger);

	var _actionTypes = __webpack_require__(390);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	var logger = _logger2.default.create('Modules/Sessions'); /*
	                                                          Copyright 2015 Gravitational, Inc.
	                                                          
	                                                          Licensed under the Apache License, Version 2.0 (the "License");
	                                                          you may not use this file except in compliance with the License.
	                                                          You may obtain a copy of the License at
	                                                          
	                                                              http://www.apache.org/licenses/LICENSE-2.0
	                                                          
	                                                          Unless required by applicable law or agreed to in writing, software
	                                                          distributed under the License is distributed on an "AS IS" BASIS,
	                                                          WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                          See the License for the specific language governing permissions and
	                                                          limitations under the License.
	                                                          */

	var actions = {
	  fetchStoredSession: function fetchStoredSession(sid, siteId) {
	    siteId = siteId || _reactor2.default.evaluate(_getters2.default.siteId);
	    return _api2.default.get(_config2.default.api.getSessionEventsUrl({ siteId: siteId, sid: sid })).then(function (json) {
	      if (json && json.events) {
	        _reactor2.default.dispatch(_actionTypes.RECEIVE_SITE_EVENTS, { siteId: siteId, json: json.events });
	      }
	    });
	  },
	  fetchSiteEvents: function fetchSiteEvents(start, end) {
	    // default values
	    start = start || (0, _moment2.default)(new Date()).endOf('day').toDate();
	    end = end || (0, _moment2.default)(end).subtract(3, 'day').startOf('day').toDate();

	    start = start.toISOString();
	    end = end.toISOString();

	    var siteId = _reactor2.default.evaluate(_getters2.default.siteId);
	    return _api2.default.get(_config2.default.api.getSiteEventsFilterUrl({ start: start, end: end, siteId: siteId })).done(function (json) {
	      if (json && json.events) {
	        _reactor2.default.dispatch(_actionTypes.RECEIVE_SITE_EVENTS, { siteId: siteId, json: json.events });
	      }
	    }).fail(function (err) {
	      logger.error('fetchSiteEvents', err);
	    });
	  },
	  fetchActiveSessions: function fetchActiveSessions() {
	    var siteId = _reactor2.default.evaluate(_getters2.default.siteId);
	    return _api2.default.get(_config2.default.api.getFetchSessionsUrl(siteId)).done(function (json) {
	      var sessions = json.sessions || [];
	      _reactor2.default.dispatch(_actionTypes.RECEIVE_ACTIVE_SESSIONS, { siteId: siteId, json: sessions });
	    }).fail(function (err) {
	      logger.error('fetchActiveSessions', err);
	    });
	  },
	  updateSession: function updateSession(_ref) {
	    var siteId = _ref.siteId,
	        json = _ref.json;

	    _reactor2.default.dispatch(_actionTypes.UPDATE_ACTIVE_SESSION, { siteId: siteId, json: json });
	  }
	};

	exports.default = actions;
	module.exports = exports['default'];

/***/ }),
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
/* 310 */,
/* 311 */,
/* 312 */,
/* 313 */,
/* 314 */,
/* 315 */,
/* 316 */,
/* 317 */,
/* 318 */,
/* 319 */,
/* 320 */,
/* 321 */,
/* 322 */,
/* 323 */,
/* 324 */,
/* 325 */,
/* 326 */,
/* 327 */,
/* 328 */,
/* 329 */,
/* 330 */,
/* 331 */,
/* 332 */,
/* 333 */,
/* 334 */,
/* 335 */,
/* 336 */,
/* 337 */,
/* 338 */,
/* 339 */,
/* 340 */,
/* 341 */,
/* 342 */,
/* 343 */,
/* 344 */,
/* 345 */,
/* 346 */,
/* 347 */,
/* 348 */,
/* 349 */,
/* 350 */,
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
/* 386 */,
/* 387 */,
/* 388 */,
/* 389 */,
/* 390 */
/***/ (function(module, exports) {

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

	var RECEIVE_ACTIVE_SESSIONS = exports.RECEIVE_ACTIVE_SESSIONS = 'TLPT_SESSIONS_RECEIVE_ACTIVE';
	var UPDATE_ACTIVE_SESSION = exports.UPDATE_ACTIVE_SESSION = 'TLPT_SESSIONS_UPDATE_ACTIVE';
	var RECEIVE_SITE_EVENTS = exports.RECEIVE_SITE_EVENTS = 'TLPT_SESSIONS_RECEIVE_EVENTS';

/***/ }),
/* 391 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _store = __webpack_require__(392);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               limitations under the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               */

	var withStorage = function withStorage(component) {
	  var _class, _temp;

	  return _temp = _class = function (_React$Component) {
	    _inherits(WithTmpStorageWrapper, _React$Component);

	    function WithTmpStorageWrapper(props, context) {
	      _classCallCheck(this, WithTmpStorageWrapper);

	      return _possibleConstructorReturn(this, _React$Component.call(this, props, context));
	    }

	    WithTmpStorageWrapper.prototype.render = function render() {
	      var props = this.props;
	      return _react2.default.createElement(component, _extends({}, props, {
	        storage: _store.storage
	      }));
	    };

	    return WithTmpStorageWrapper;
	  }(_react2.default.Component), _class.displayName = 'withTmpStorageWrapper', _temp;
	};

	exports.default = withStorage;
	module.exports = exports['default'];

/***/ }),
/* 392 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.storage = undefined;

	var _reactor$registerStor; /*
	                           Copyright 2015 Gravitational, Inc.
	                           
	                           Licensed under the Apache License, Version 2.0 (the "License");
	                           you may not use this file except in compliance with the License.
	                           You may obtain a copy of the License at
	                           
	                               http://www.apache.org/licenses/LICENSE-2.0
	                           
	                           Unless required by applicable law or agreed to in writing, software
	                           distributed under the License is distributed on an "AS IS" BASIS,
	                           WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                           See the License for the specific language governing permissions and
	                           limitations under the License.
	                           */

	var _nuclearJs = __webpack_require__(234);

	var _reactor = __webpack_require__(233);

	var _reactor2 = _interopRequireDefault(_reactor);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	var SET = 'MISC_SET';
	var STORE_NAME = 'tlpt_misc';

	// stores any temporary data
	var store = (0, _nuclearJs.Store)({
	  getInitialState: function getInitialState() {
	    return new _nuclearJs.Immutable.Map();
	  },
	  initialize: function initialize() {
	    this.on(SET, function (state, _ref) {
	      var key = _ref.key,
	          payload = _ref.payload;
	      return state.set(key, payload);
	    });
	  }
	});

	_reactor2.default.registerStores((_reactor$registerStor = {}, _reactor$registerStor[STORE_NAME] = store, _reactor$registerStor));

	var storage = exports.storage = {
	  save: function save(key, payload) {
	    _reactor2.default.dispatch(SET, { key: key, payload: payload });
	  },
	  findByKey: function findByKey(key) {
	    return _reactor2.default.evaluate([STORE_NAME, key]);
	  }
	};

/***/ }),
/* 393 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _jQuery = __webpack_require__(229);

	var _jQuery2 = _interopRequireDefault(_jQuery);

	var _reactor = __webpack_require__(233);

	var _reactor2 = _interopRequireDefault(_reactor);

	var _lodash = __webpack_require__(275);

	var _withFeature = __webpack_require__(394);

	var _withFeature2 = _interopRequireDefault(_withFeature);

	var _api = __webpack_require__(241);

	var _api2 = _interopRequireDefault(_api);

	var _enums = __webpack_require__(264);

	var _actions = __webpack_require__(246);

	var _getters = __webpack_require__(251);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	var _featureId = 0;

	var ensureActionType = function ensureActionType(actionType) {
	  if (!actionType) {
	    ++_featureId;
	    return 'TRYING_TO_INIT_FEATURE_' + _featureId;
	  }

	  return actionType;
	};

	var FeatureBase = function () {
	  function FeatureBase(actionType) {
	    _classCallCheck(this, FeatureBase);

	    actionType = ensureActionType(actionType);
	    this.initStatus = (0, _actions.makeStatus)(ensureActionType(actionType));
	    this.initAttemptGetter = (0, _getters.makeGetter)(actionType);
	  }

	  FeatureBase.prototype.preload = function preload() {
	    return _jQuery2.default.Deferred().resolve();
	  };

	  FeatureBase.prototype.onload = function onload() {};

	  FeatureBase.prototype.startProcessing = function startProcessing() {
	    this.initStatus.start();
	  };

	  FeatureBase.prototype.stopProcessing = function stopProcessing() {
	    this.initStatus.success();
	  };

	  FeatureBase.prototype.isReady = function isReady() {
	    return this._getInitAttempt().isSuccess;
	  };

	  FeatureBase.prototype.isProcessing = function isProcessing() {
	    return this._getInitAttempt().isProcessing;
	  };

	  FeatureBase.prototype.isFailed = function isFailed() {
	    return this._getInitAttempt().isFailed;
	  };

	  FeatureBase.prototype.wasInitialized = function wasInitialized() {
	    var attempt = this._getInitAttempt();
	    return attempt.isFailed || attempt.isProcessing || attempt.isSuccess;
	  };

	  FeatureBase.prototype.componentDidMount = function componentDidMount() {};

	  FeatureBase.prototype.getErrorText = function getErrorText() {
	    var _getInitAttempt2 = this._getInitAttempt(),
	        message = _getInitAttempt2.message;

	    return (0, _lodash.isObject)(message) ? message.text : message;
	  };

	  FeatureBase.prototype.getErrorCode = function getErrorCode() {
	    var _getInitAttempt3 = this._getInitAttempt(),
	        message = _getInitAttempt3.message;

	    return (0, _lodash.isObject)(message) ? message.code : null;
	  };

	  FeatureBase.prototype.handleAccesDenied = function handleAccesDenied() {
	    this.handleError(new Error('Access Denied'));
	  };

	  FeatureBase.prototype.handleError = function handleError(err) {
	    var message = _api2.default.getErrorText(err);
	    if (err.status === _enums.RestRespCodeEnum.FORBIDDEN) {
	      message = {
	        code: _enums.RestRespCodeEnum.FORBIDDEN,
	        text: message
	      };
	    }

	    this.initStatus.fail(message);
	  };

	  FeatureBase.prototype.withMe = function withMe(component) {
	    return (0, _withFeature2.default)(this)(component);
	  };

	  FeatureBase.prototype._getInitAttempt = function _getInitAttempt() {
	    return _reactor2.default.evaluate(this.initAttemptGetter);
	  };

	  return FeatureBase;
	}();

	exports.default = FeatureBase;
	module.exports = exports['default'];

/***/ }),
/* 394 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _reactor = __webpack_require__(233);

	var _reactor2 = _interopRequireDefault(_reactor);

	var _indicator = __webpack_require__(395);

	var _indicator2 = _interopRequireDefault(_indicator);

	var _enums = __webpack_require__(264);

	var _msgPage = __webpack_require__(266);

	var Messages = _interopRequireWildcard(_msgPage);

	var _logger = __webpack_require__(245);

	var _logger2 = _interopRequireDefault(_logger);

	function _interopRequireWildcard(obj) { if (obj && obj.__esModule) { return obj; } else { var newObj = {}; if (obj != null) { for (var key in obj) { if (Object.prototype.hasOwnProperty.call(obj, key)) newObj[key] = obj[key]; } } newObj.default = obj; return newObj; } }

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               limitations under the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               */

	var logger = _logger2.default.create('components/withFeature');

	var withFeature = function withFeature(feature) {
	  return function (component) {
	    var _class, _temp;

	    return _temp = _class = function (_React$Component) {
	      _inherits(WithFeatureWrapper, _React$Component);

	      function WithFeatureWrapper(props, context) {
	        _classCallCheck(this, WithFeatureWrapper);

	        var _this = _possibleConstructorReturn(this, _React$Component.call(this, props, context));

	        _this._unsubscribeFn = null;
	        return _this;
	      }

	      WithFeatureWrapper.prototype.componentDidMount = function componentDidMount() {
	        var _this2 = this;

	        try {
	          this._unsubscribeFn = _reactor2.default.observe(feature.initAttemptGetter, function () {
	            _this2.setState({});
	          });

	          _reactor2.default.batch(function () {
	            feature.componentDidMount();
	          });
	        } catch (err) {
	          logger.error('failed to initialize a feature', err);
	        }
	      };

	      WithFeatureWrapper.prototype.componentWillUnmount = function componentWillUnmount() {
	        this._unsubscribeFn();
	      };

	      WithFeatureWrapper.prototype.render = function render() {
	        if (feature.isProcessing()) {
	          return _react2.default.createElement(_indicator2.default, { delay: 'long', type: 'bounce' });
	        }

	        if (feature.isFailed()) {
	          var errorText = feature.getErrorText();
	          if (feature.getErrorCode() === _enums.RestRespCodeEnum.FORBIDDEN) {
	            return _react2.default.createElement(Messages.AccessDenied, { message: errorText });
	          }
	          return _react2.default.createElement(Messages.Failed, { message: errorText });
	        }

	        if (!feature.wasInitialized()) {
	          return null;
	        }

	        var props = this.props;
	        return _react2.default.createElement(component, _extends({}, props, {
	          feature: feature
	        }));
	      };

	      return WithFeatureWrapper;
	    }(_react2.default.Component), _class.displayName = 'withFeatureWrapper', _temp;
	  };
	};

	exports.default = withFeature;
	module.exports = exports['default'];

/***/ }),
/* 395 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

	var WHEN_TO_DISPLAY = 100; // 0.2s;

	var Indicator = function (_React$Component) {
	  _inherits(Indicator, _React$Component);

	  function Indicator(props) {
	    _classCallCheck(this, Indicator);

	    var _this = _possibleConstructorReturn(this, _React$Component.call(this, props));

	    _this._timer = null;
	    _this.state = {
	      canDisplay: false
	    };
	    return _this;
	  }

	  Indicator.prototype.componentDidMount = function componentDidMount() {
	    var _this2 = this;

	    this._timer = setTimeout(function () {
	      _this2.setState({
	        canDisplay: true
	      });
	    }, WHEN_TO_DISPLAY);
	  };

	  Indicator.prototype.componentWillUnmount = function componentWillUnmount() {
	    clearTimeout(this._timer);
	  };

	  Indicator.prototype.render = function render() {
	    var _props$type = this.props.type,
	        type = _props$type === undefined ? 'bounce' : _props$type;


	    if (!this.state.canDisplay) {
	      return null;
	    }

	    if (type === 'bounce') {
	      return _react2.default.createElement(ThreeBounce, null);
	    }
	  };

	  return Indicator;
	}(_react2.default.Component);

	var ThreeBounce = function ThreeBounce() {
	  return _react2.default.createElement(
	    'div',
	    { className: 'grv-spinner sk-spinner sk-spinner-three-bounce' },
	    _react2.default.createElement('div', { className: 'sk-bounce1' }),
	    _react2.default.createElement('div', { className: 'sk-bounce2' }),
	    _react2.default.createElement('div', { className: 'sk-bounce3' })
	  );
	};

	exports.default = Indicator;
	module.exports = exports['default'];

/***/ }),
/* 396 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _nuclearJsReactAddons = __webpack_require__(219);

	var _enums = __webpack_require__(397);

	var _terminal = __webpack_require__(398);

	var _terminal2 = _interopRequireDefault(_terminal);

	var _getters = __webpack_require__(403);

	var _getters2 = _interopRequireDefault(_getters);

	var _ttyAddressResolver = __webpack_require__(404);

	var _ttyAddressResolver2 = _interopRequireDefault(_ttyAddressResolver);

	var _actions = __webpack_require__(405);

	var _actions2 = __webpack_require__(290);

	var _actions3 = __webpack_require__(409);

	var playerActions = _interopRequireWildcard(_actions3);

	var _partyListPanel = __webpack_require__(411);

	var _partyListPanel2 = _interopRequireDefault(_partyListPanel);

	var _indicator = __webpack_require__(395);

	var _indicator2 = _interopRequireDefault(_indicator);

	var _terminalPartyList = __webpack_require__(412);

	var _terminalPartyList2 = _interopRequireDefault(_terminalPartyList);

	function _interopRequireWildcard(obj) { if (obj && obj.__esModule) { return obj; } else { var newObj = {}; if (obj != null) { for (var key in obj) { if (Object.prototype.hasOwnProperty.call(obj, key)) newObj[key] = obj[key]; } } newObj.default = obj; return newObj; } }

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               limitations under the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               */

	var TerminalHost = function (_React$Component) {
	  _inherits(TerminalHost, _React$Component);

	  function TerminalHost(props) {
	    _classCallCheck(this, TerminalHost);

	    var _this = _possibleConstructorReturn(this, _React$Component.call(this, props));

	    _this.startNew = function () {
	      var newRouteParams = _extends({}, _this.props.routeParams, {
	        sid: undefined
	      });

	      (0, _actions.updateRoute)(newRouteParams);
	      (0, _actions.initTerminal)(newRouteParams);
	    };

	    _this.replay = function () {
	      var _this$props$routePara = _this.props.routeParams,
	          siteId = _this$props$routePara.siteId,
	          sid = _this$props$routePara.sid;

	      playerActions.open(siteId, sid);
	    };

	    return _this;
	  }

	  TerminalHost.prototype.componentDidMount = function componentDidMount() {
	    var _this2 = this;

	    setTimeout(function () {
	      return (0, _actions.initTerminal)(_this2.props.routeParams);
	    }, 0);
	  };

	  TerminalHost.prototype.render = function render() {
	    var store = this.props.store;
	    var status = store.status,
	        sid = store.sid;

	    var serverLabel = store.getServerLabel();

	    var $content = null;
	    var $leftPanelContent = null;

	    if (status.isLoading) {
	      $content = _react2.default.createElement(_indicator2.default, { type: 'bounce' });
	    }

	    if (status.isError) {
	      $content = _react2.default.createElement(ErrorIndicator, { text: status.errorText });
	    }

	    if (status.isNotFound) {
	      $content = _react2.default.createElement(SidNotFoundError, {
	        onReplay: this.replay,
	        onNew: this.startNew });
	    }

	    if (status.isReady) {
	      document.title = serverLabel;
	      $content = _react2.default.createElement(TerminalContainer, { store: store });
	      $leftPanelContent = _react2.default.createElement(_terminalPartyList2.default, { sid: sid });
	    }

	    return _react2.default.createElement(
	      'div',
	      { className: 'grv-terminalhost' },
	      _react2.default.createElement(
	        _partyListPanel2.default,
	        { onClose: _actions.close },
	        $leftPanelContent
	      ),
	      _react2.default.createElement(
	        'div',
	        { className: 'grv-terminalhost-server-info' },
	        _react2.default.createElement(
	          'h3',
	          null,
	          serverLabel
	        )
	      ),
	      $content
	    );
	  };

	  return TerminalHost;
	}(_react2.default.Component);

	var TerminalContainer = function (_React$Component2) {
	  _inherits(TerminalContainer, _React$Component2);

	  function TerminalContainer() {
	    _classCallCheck(this, TerminalContainer);

	    return _possibleConstructorReturn(this, _React$Component2.apply(this, arguments));
	  }

	  TerminalContainer.prototype.componentDidMount = function componentDidMount() {
	    var options = this.props.store.getTtyParams();
	    var addressResolver = new _ttyAddressResolver2.default(options);
	    this.terminal = new _terminal2.default({
	      el: this.refs.container,
	      addressResolver: addressResolver
	    });

	    this.terminal.ttyEvents.on('data', this.receiveEvents.bind(this));
	    this.terminal.open();
	  };

	  TerminalContainer.prototype.componentWillUnmount = function componentWillUnmount() {
	    this.terminal.destroy();
	  };

	  TerminalContainer.prototype.shouldComponentUpdate = function shouldComponentUpdate() {
	    return false;
	  };

	  TerminalContainer.prototype.render = function render() {
	    return _react2.default.createElement('div', { ref: 'container' });
	  };

	  TerminalContainer.prototype.receiveEvents = function receiveEvents(data) {
	    var hasEnded = data.events.some(function (item) {
	      return item.event === _enums.EventTypeEnum.END;
	    });
	    if (hasEnded) {
	      (0, _actions.close)();
	    }

	    // update participant list
	    (0, _actions2.updateSession)({
	      siteId: this.props.store.getClusterName(),
	      json: data.session
	    });
	  };

	  return TerminalContainer;
	}(_react2.default.Component);

	var ErrorIndicator = function ErrorIndicator(_ref) {
	  var text = _ref.text;
	  return _react2.default.createElement(
	    'div',
	    { className: 'grv-terminalhost-indicator-error' },
	    _react2.default.createElement('i', { className: 'fa fa-exclamation-triangle fa-3x text-warning' }),
	    _react2.default.createElement(
	      'div',
	      { className: 'm-l' },
	      _react2.default.createElement(
	        'strong',
	        null,
	        'Connection error'
	      ),
	      _react2.default.createElement(
	        'div',
	        null,
	        _react2.default.createElement(
	          'small',
	          null,
	          text
	        )
	      )
	    )
	  );
	};

	var SidNotFoundError = function SidNotFoundError(_ref2) {
	  var onNew = _ref2.onNew,
	      onReplay = _ref2.onReplay;
	  return _react2.default.createElement(
	    'div',
	    { className: 'grv-terminalhost-indicator-error' },
	    _react2.default.createElement(
	      'div',
	      { className: 'text-center' },
	      _react2.default.createElement(
	        'strong',
	        null,
	        'The session is no longer active'
	      ),
	      _react2.default.createElement(
	        'div',
	        { className: 'm-t' },
	        _react2.default.createElement(
	          'button',
	          { onClick: onNew, className: 'btn btn-sm btn-primary m-r' },
	          ' Start New '
	        ),
	        _react2.default.createElement(
	          'button',
	          { onClick: onReplay, className: 'btn btn-sm btn-primary' },
	          ' Replay '
	        )
	      )
	    )
	  );
	};

	function mapStateToProps() {
	  return {
	    store: _getters2.default.store
	  };
	}

	exports.default = (0, _nuclearJsReactAddons.connect)(mapStateToProps)(TerminalHost);
	module.exports = exports['default'];

/***/ }),
/* 397 */
/***/ (function(module, exports) {

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

	var EventTypeEnum = exports.EventTypeEnum = {
	  START: 'session.start',
	  END: 'session.end',
	  PRINT: 'print',
	  RESIZE: 'resize'
	};

	var StatusCodeEnum = exports.StatusCodeEnum = {
	  NORMAL: 1000
	};

/***/ }),
/* 398 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _xterm = __webpack_require__(399);

	var _xterm2 = _interopRequireDefault(_xterm);

	var _tty = __webpack_require__(400);

	var _tty2 = _interopRequireDefault(_tty);

	var _ttyEvents = __webpack_require__(402);

	var _ttyEvents2 = _interopRequireDefault(_ttyEvents);

	var _lodash = __webpack_require__(275);

	var _logger = __webpack_require__(245);

	var _logger2 = _interopRequireDefault(_logger);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } } /*
	                                                                                                                                                          Copyright 2015 Gravitational, Inc.
	                                                                                                                                                          
	                                                                                                                                                          Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                          you may not use this file except in compliance with the License.
	                                                                                                                                                          You may obtain a copy of the License at
	                                                                                                                                                          
	                                                                                                                                                              http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                          
	                                                                                                                                                          Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                          distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                          WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                          See the License for the specific language governing permissions and
	                                                                                                                                                          limitations under the License.
	                                                                                                                                                          */


	var logger = _logger2.default.create('lib/term/terminal');
	var DISCONNECT_TXT = 'disconnected';
	var GRV_CLASS = 'grv-terminal';
	var WINDOW_RESIZE_DEBOUNCE_DELAY = 200;

	/**
	 * TtyTerminal is a wrapper on top of xtermjs that handles connections
	 * and resize events
	 */

	var TtyTerminal = function () {
	  function TtyTerminal(options) {
	    _classCallCheck(this, TtyTerminal);

	    var addressResolver = options.addressResolver,
	        el = options.el,
	        _options$scrollBack = options.scrollBack,
	        scrollBack = _options$scrollBack === undefined ? 1000 : _options$scrollBack;

	    this._el = el;
	    this.tty = new _tty2.default(addressResolver);
	    this.ttyEvents = new _ttyEvents2.default(addressResolver);
	    this.scrollBack = scrollBack;
	    this.rows = undefined;
	    this.cols = undefined;
	    this.term = null;
	    this.debouncedResize = (0, _lodash.debounce)(this._requestResize.bind(this), WINDOW_RESIZE_DEBOUNCE_DELAY);
	  }

	  TtyTerminal.prototype.open = function open() {
	    var _this = this;

	    this._el.classList.add(GRV_CLASS);

	    // render xtermjs with default values
	    this.term = new _xterm2.default({
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
	    this.tty.on('reset', this.reset.bind(this));
	    this.tty.on('close', this._processClose.bind(this));
	    this.tty.on('data', this._processData.bind(this));

	    // subscribe tty resize event (used by session player)
	    this.tty.on('resize', function (_ref) {
	      var h = _ref.h,
	          w = _ref.w;
	      return _this.resize(w, h);
	    });
	    // subscribe to session resize events (triggered by other participants)
	    this.ttyEvents.on('resize', function (_ref2) {
	      var h = _ref2.h,
	          w = _ref2.w;
	      return _this.resize(w, h);
	    });

	    this.connect();
	  };

	  TtyTerminal.prototype.connect = function connect() {
	    this.tty.connect(this.cols, this.rows);
	    this.ttyEvents.connect();
	  };

	  TtyTerminal.prototype.destroy = function destroy() {
	    window.removeEventListener('resize', this.debouncedResize);
	    this._disconnect();
	    if (this.term !== null) {
	      this.term.destroy();
	      this.term.removeAllListeners();
	    }

	    this._el.innerHTML = null;
	    this._el.classList.remove(GRV_CLASS);
	  };

	  TtyTerminal.prototype.reset = function reset() {
	    this.term.reset();
	  };

	  TtyTerminal.prototype.resize = function resize(cols, rows) {
	    try {
	      // if not defined, use the size of the container
	      if (!(0, _lodash.isNumber)(cols) || !(0, _lodash.isNumber)(rows)) {
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
	      logger.error('xterm.resize', { w: cols, h: rows }, err);
	      this.term.reset();
	    }
	  };

	  TtyTerminal.prototype._processData = function _processData(data) {
	    try {
	      this.term.write(data);
	    } catch (err) {
	      logger.error('xterm.write', data, err);
	      // recover xtermjs by resetting it
	      this.term.reset();
	    }
	  };

	  TtyTerminal.prototype._processClose = function _processClose(e) {
	    var reason = e.reason;

	    var displayText = DISCONNECT_TXT;
	    if (reason) {
	      displayText = displayText + ': ' + reason;
	    }

	    displayText = '\x1B[31m' + displayText + '\x1B[m\r\n';
	    this.term.write(displayText);
	  };

	  TtyTerminal.prototype._disconnect = function _disconnect() {
	    this.tty.disconnect();
	    this.tty.removeAllListeners();
	    this.ttyEvents.disconnect();
	    this.ttyEvents.removeAllListeners();
	  };

	  TtyTerminal.prototype._requestResize = function _requestResize() {
	    var _getDimensions2 = this._getDimensions(),
	        cols = _getDimensions2.cols,
	        rows = _getDimensions2.rows;
	    // ensure min size


	    var w = cols < 5 ? 5 : cols;
	    var h = rows < 5 ? 5 : rows;

	    this.resize(w, h);
	    this.tty.requestResize(w, h);
	  };

	  TtyTerminal.prototype._getDimensions = function _getDimensions() {
	    var parentElementStyle = window.getComputedStyle(this.term.element.parentElement);
	    var parentElementHeight = parseInt(parentElementStyle.getPropertyValue('height'));
	    var parentElementWidth = Math.max(0, parseInt(parentElementStyle.getPropertyValue('width')) /*- 17*/);
	    var elementStyle = window.getComputedStyle(this.term.element);
	    var elementPaddingVer = parseInt(elementStyle.getPropertyValue('padding-top')) + parseInt(elementStyle.getPropertyValue('padding-bottom'));
	    var elementPaddingHor = parseInt(elementStyle.getPropertyValue('padding-right')) + parseInt(elementStyle.getPropertyValue('padding-left'));
	    var availableHeight = parentElementHeight - elementPaddingVer;
	    var availableWidth = parentElementWidth - elementPaddingHor;
	    var subjectRow = this.term.rowContainer.firstElementChild;
	    var contentBuffer = subjectRow.innerHTML;

	    subjectRow.style.display = 'inline';
	    // common character for measuring width, although on monospace
	    subjectRow.innerHTML = 'W';

	    var characterWidth = subjectRow.getBoundingClientRect().width;
	    // revert style before calculating height, since they differ.
	    subjectRow.style.display = '';

	    var characterHeight = parseInt(subjectRow.offsetHeight);
	    subjectRow.innerHTML = contentBuffer;

	    var rows = parseInt(availableHeight / characterHeight);
	    var cols = parseInt(availableWidth / characterWidth);
	    return { cols: cols, rows: rows };
	  };

	  return TtyTerminal;
	}();

	exports.default = TtyTerminal;
	module.exports = exports['default'];

/***/ }),
/* 399 */,
/* 400 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

	var _events = __webpack_require__(401);

	var _enums = __webpack_require__(397);

	var _api = __webpack_require__(241);

	var _api2 = _interopRequireDefault(_api);

	var _logger = __webpack_require__(245);

	var _logger2 = _interopRequireDefault(_logger);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               limitations under the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               */

	var logger = _logger2.default.create('Tty');

	var defaultOptions = {
	  buffered: true
	};

	var Tty = function (_EventEmitter) {
	  _inherits(Tty, _EventEmitter);

	  function Tty(addressResolver) {
	    var props = arguments.length > 1 && arguments[1] !== undefined ? arguments[1] : {};

	    _classCallCheck(this, Tty);

	    var _this = _possibleConstructorReturn(this, _EventEmitter.call(this));

	    _this.socket = null;
	    _this._buffered = true;
	    _this._addressResolver = null;

	    var options = _extends({}, defaultOptions, props);

	    _this._addressResolver = addressResolver;
	    _this._buffered = options.buffered;
	    _this._onOpenConnection = _this._onOpenConnection.bind(_this);
	    _this._onCloseConnection = _this._onCloseConnection.bind(_this);
	    _this._onReceiveData = _this._onReceiveData.bind(_this);
	    return _this;
	  }

	  Tty.prototype.disconnect = function disconnect() {
	    var reasonCode = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : _enums.StatusCodeEnum.NORMAL;

	    if (this.socket !== null) {
	      this.socket.close(reasonCode);
	    }
	  };

	  Tty.prototype.connect = function connect(w, h) {
	    var connStr = this._addressResolver.getConnStr(w, h);
	    this.socket = new WebSocket(connStr);
	    this.socket.onopen = this._onOpenConnection;
	    this.socket.onmessage = this._onReceiveData;
	    this.socket.onclose = this._onCloseConnection;
	  };

	  Tty.prototype.send = function send(data) {
	    this.socket.send(data);
	  };

	  Tty.prototype.requestResize = function requestResize(w, h) {
	    var url = this._addressResolver.getResizeReqUrl();
	    var payload = {
	      terminal_params: { w: w, h: h }
	    };

	    logger.info('requesting new screen size', 'w:' + w + ' and h:' + h);
	    return _api2.default.put(url, payload).fail(function (err) {
	      return logger.error('requestResize', err);
	    });
	  };

	  Tty.prototype._flushBuffer = function _flushBuffer() {
	    this.emit('data', this._attachSocketBuffer);
	    this._attachSocketBuffer = null;
	    clearTimeout(this._attachSocketBufferTimer);
	    this._attachSocketBufferTimer = null;
	  };

	  Tty.prototype._pushToBuffer = function _pushToBuffer(data) {
	    if (this._attachSocketBuffer) {
	      this._attachSocketBuffer += data;
	    } else {
	      this._attachSocketBuffer = data;
	      setTimeout(this._flushBuffer.bind(this), 10);
	    }
	  };

	  Tty.prototype._onOpenConnection = function _onOpenConnection() {
	    this.emit('open');
	    logger.info('websocket is open');
	  };

	  Tty.prototype._onCloseConnection = function _onCloseConnection(e) {
	    this.socket.onopen = null;
	    this.socket.onmessage = null;
	    this.socket.onclose = null;
	    this.socket = null;
	    this.emit('close', e);
	    logger.info('websocket is closed');
	  };

	  Tty.prototype._onReceiveData = function _onReceiveData(ev) {
	    if (this._buffered) {
	      this._pushToBuffer(ev.data);
	    } else {
	      this.emit('data', ev.data);
	    }
	  };

	  return Tty;
	}(_events.EventEmitter);

	exports.default = Tty;
	module.exports = exports['default'];

/***/ }),
/* 401 */
/***/ (function(module, exports) {

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


/***/ }),
/* 402 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _events = __webpack_require__(401);

	var _lodash = __webpack_require__(275);

	var _enums = __webpack_require__(397);

	var _logger = __webpack_require__(245);

	var _logger2 = _interopRequireDefault(_logger);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               limitations under the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               */

	var logger = _logger2.default.create('TtyEvents');

	var TtyEvents = function (_EventEmitter) {
	  _inherits(TtyEvents, _EventEmitter);

	  function TtyEvents(addressResolver) {
	    _classCallCheck(this, TtyEvents);

	    var _this = _possibleConstructorReturn(this, _EventEmitter.call(this));

	    _this.socket = null;
	    _this._addressResolver = null;

	    _this._addressResolver = addressResolver;
	    return _this;
	  }

	  TtyEvents.prototype.connect = function connect() {
	    var connStr = this._addressResolver.getEventProviderConnStr();
	    this.socket = new WebSocket(connStr);
	    this.socket.onmessage = this._onReceiveMessage.bind(this);
	    this.socket.onclose = this._onCloseConnection.bind(this);
	    this.socket.onopen = function () {
	      logger.info('websocket is open');
	    };
	  };

	  TtyEvents.prototype.disconnect = function disconnect() {
	    var reasonCode = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : _enums.StatusCodeEnum.NORMAL;

	    if (this.socket !== null) {
	      this.socket.close(reasonCode);
	    }
	  };

	  TtyEvents.prototype._onCloseConnection = function _onCloseConnection(e) {
	    this.socket.onmessage = null;
	    this.socket.onopen = null;
	    this.socket.onclose = null;
	    this.emit('close', e);
	    logger.info('websocket is closed');
	  };

	  TtyEvents.prototype._onReceiveMessage = function _onReceiveMessage(message) {
	    try {
	      var json = JSON.parse(message.data);
	      this._processResize(json.events);
	      this.emit('data', json);
	    } catch (err) {
	      logger.error('failed to parse event stream data', err);
	    }
	  };

	  TtyEvents.prototype._processResize = function _processResize(events) {
	    events = events || [];
	    // filter resize events 
	    var resizes = events.filter(function (item) {
	      return item.event === _enums.EventTypeEnum.RESIZE;
	    });

	    (0, _lodash.sortBy)(resizes, ['ms']);

	    if (resizes.length > 0) {
	      // get values from the last resize event
	      var _resizes$size$split = resizes[resizes.length - 1].size.split(':'),
	          w = _resizes$size$split[0],
	          h = _resizes$size$split[1];

	      w = Number(w);
	      h = Number(h);
	      this.emit('resize', { w: w, h: h });
	    }
	  };

	  return TtyEvents;
	}(_events.EventEmitter);

	exports.default = TtyEvents;
	module.exports = exports['default'];

/***/ }),
/* 403 */
/***/ (function(module, exports) {

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

	exports.default = {
	  store: ['tlpt_terminal']
	};
	module.exports = exports['default'];

/***/ }),
/* 404 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

	var _config = __webpack_require__(228);

	var _config2 = _interopRequireDefault(_config);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } } /*
	                                                                                                                                                          Copyright 2015 Gravitational, Inc.
	                                                                                                                                                          
	                                                                                                                                                          Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                          you may not use this file except in compliance with the License.
	                                                                                                                                                          You may obtain a copy of the License at
	                                                                                                                                                          
	                                                                                                                                                              http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                          
	                                                                                                                                                          Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                          distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                          WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                          See the License for the specific language governing permissions and
	                                                                                                                                                          limitations under the License.
	                                                                                                                                                          */

	var AddressResolver = function () {
	  function AddressResolver(params) {
	    _classCallCheck(this, AddressResolver);

	    this._params = {
	      login: null,
	      target: function target() {
	        throw Error('target method is not provided');
	      },
	      sid: null,
	      clusterName: null,
	      ttyUrl: null,
	      ttyEventUrl: null,
	      ttyResizeUrl: null
	    };

	    this._params = _extends({}, params);
	  }

	  AddressResolver.prototype.getConnStr = function getConnStr(w, h) {
	    var _params = this._params,
	        getTarget = _params.getTarget,
	        ttyUrl = _params.ttyUrl,
	        login = _params.login,
	        sid = _params.sid;

	    var params = JSON.stringify(_extends({}, getTarget(), {
	      login: login,
	      sid: sid,
	      term: { h: h, w: w }
	    }));

	    var encoded = window.encodeURI(params);
	    return this.format(ttyUrl).replace(':params', encoded);
	  };

	  AddressResolver.prototype.getEventProviderConnStr = function getEventProviderConnStr() {
	    return this.format(this._params.ttyEventUrl);
	  };

	  AddressResolver.prototype.getResizeReqUrl = function getResizeReqUrl() {
	    return this.format(this._params.ttyResizeUrl);
	  };

	  AddressResolver.prototype.format = function format(url) {
	    return url.replace(':fqdm', _config2.default.getWsHostName()).replace(':token', this._params.token).replace(':cluster', this._params.cluster).replace(':sid', this._params.sid);
	  };

	  return AddressResolver;
	}();

	exports.default = AddressResolver;
	module.exports = exports['default'];

/***/ }),
/* 405 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.updateRoute = exports.close = exports.initTerminal = undefined;

	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; }; /*
	                                                                                                                                                                                                                                                                  Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                  
	                                                                                                                                                                                                                                                                  Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                  you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                  You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                  
	                                                                                                                                                                                                                                                                      http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                  
	                                                                                                                                                                                                                                                                  Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                  distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                  See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                  limitations under the License.
	                                                                                                                                                                                                                                                                  */


	var _reactor = __webpack_require__(233);

	var _reactor2 = _interopRequireDefault(_reactor);

	var _history = __webpack_require__(226);

	var _history2 = _interopRequireDefault(_history);

	var _api = __webpack_require__(241);

	var _api2 = _interopRequireDefault(_api);

	var _config = __webpack_require__(228);

	var _config2 = _interopRequireDefault(_config);

	var _logger = __webpack_require__(245);

	var _logger2 = _interopRequireDefault(_logger);

	var _nodeStore = __webpack_require__(406);

	var _getters = __webpack_require__(407);

	var _getters2 = _interopRequireDefault(_getters);

	var _actionTypes = __webpack_require__(408);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	var logger = _logger2.default.create('flux/terminal');

	var setStatus = function setStatus(json) {
	  return _reactor2.default.dispatch(_actionTypes.TLPT_TERMINAL_SET_STATUS, json);
	};

	var initStore = function initStore(params) {
	  var serverId = params.serverId;

	  var server = (0, _nodeStore.getNodeStore)().findServer(serverId);
	  var hostname = server ? server.hostname : '';
	  _reactor2.default.dispatch(_actionTypes.TLPT_TERMINAL_INIT, _extends({}, params, {
	    hostname: hostname
	  }));
	};

	var createSid = function createSid(routeParams) {
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

	  return _api2.default.post(_config2.default.api.getSiteSessionUrl(siteId), data);
	};

	var initTerminal = exports.initTerminal = function initTerminal(routeParams) {
	  logger.info('attempt to open a terminal', routeParams);

	  var sid = routeParams.sid;


	  setStatus({ isLoading: true });

	  if (sid) {
	    var activeSession = _reactor2.default.evaluate(_getters2.default.activeSessionById(sid));
	    if (activeSession) {
	      // init store with existing sid
	      initStore(routeParams);
	      setStatus({ isReady: true });
	    } else {
	      setStatus({ isNotFound: true });
	    }

	    return;
	  }

	  createSid(routeParams).done(function (json) {
	    var sid = json.session.id;
	    var newRouteParams = _extends({}, routeParams, {
	      sid: sid
	    });
	    initStore(newRouteParams);
	    setStatus({ isReady: true });
	    updateRoute(newRouteParams);
	  }).fail(function (err) {
	    var errorText = _api2.default.getErrorText(err);
	    setStatus({ isError: true, errorText: errorText });
	  });
	};

	var close = exports.close = function close() {
	  _reactor2.default.dispatch(_actionTypes.TLPT_TERMINAL_CLOSE);
	  _history2.default.push(_config2.default.routes.nodes);
	};

	var updateRoute = exports.updateRoute = function updateRoute(newRouteParams) {
	  var routeUrl = _config2.default.getTerminalLoginUrl(newRouteParams);
	  _history2.default.push(routeUrl);
	};

/***/ }),
/* 406 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.ServerRec = undefined;

	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

	exports.getNodeStore = getNodeStore;

	var _reactor = __webpack_require__(233);

	var _reactor2 = _interopRequireDefault(_reactor);

	var _nuclearJs = __webpack_require__(234);

	var _immutable = __webpack_require__(253);

	var _actionTypes = __webpack_require__(289);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               limitations under the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               */

	var ServerRec = exports.ServerRec = function (_Record) {
	  _inherits(ServerRec, _Record);

	  function ServerRec(props) {
	    _classCallCheck(this, ServerRec);

	    var tags = new _immutable.List((0, _nuclearJs.toImmutable)(props.tags));
	    return _possibleConstructorReturn(this, _Record.call(this, _extends({}, props, {
	      tags: tags
	    })));
	  }

	  return ServerRec;
	}((0, _immutable.Record)({
	  id: '',
	  siteId: '',
	  hostname: '',
	  tags: new _immutable.List(),
	  addr: ''
	}));

	var NodeStoreRec = function (_Record2) {
	  _inherits(NodeStoreRec, _Record2);

	  function NodeStoreRec() {
	    _classCallCheck(this, NodeStoreRec);

	    return _possibleConstructorReturn(this, _Record2.apply(this, arguments));
	  }

	  NodeStoreRec.prototype.findServer = function findServer(serverId) {
	    return this.servers.find(function (s) {
	      return s.id === serverId;
	    });
	  };

	  NodeStoreRec.prototype.getSiteServers = function getSiteServers(siteId) {
	    return this.servers.filter(function (s) {
	      return s.siteId === siteId;
	    });
	  };

	  NodeStoreRec.prototype.addSiteServers = function addSiteServers(jsonItems) {
	    var list = new _immutable.List().withMutations(function (state) {
	      jsonItems.forEach(function (item) {
	        return state.push(new ServerRec(item));
	      });
	      return state;
	    });

	    return list.equals(this.servers) ? this : this.set('servers', list);
	  };

	  return NodeStoreRec;
	}((0, _immutable.Record)({
	  servers: new _immutable.List()
	}));

	function getNodeStore() {
	  return _reactor2.default.evaluate(['tlpt_nodes']);
	}

	exports.default = (0, _nuclearJs.Store)({
	  getInitialState: function getInitialState() {
	    return new NodeStoreRec();
	  },
	  initialize: function initialize() {
	    this.on(_actionTypes.TLPT_NODES_RECEIVE, function (state, items) {
	      return state.addSiteServers(items);
	    });
	  }
	});

/***/ }),
/* 407 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _moment = __webpack_require__(291);

	var _moment2 = _interopRequireDefault(_moment);

	var _config = __webpack_require__(228);

	var _config2 = _interopRequireDefault(_config);

	var _enums = __webpack_require__(397);

	var _reactor = __webpack_require__(233);

	var _reactor2 = _interopRequireDefault(_reactor);

	var _objectUtils = __webpack_require__(277);

	var _nodeStore = __webpack_require__(406);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	/*
	** Getters
	*/
	/*
	Copyright 2015 Gravitational, Inc.

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

	    http://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
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

	var nodeIpById = function nodeIpById(sid) {
	  return ['tlpt_sessions_events', sid, _enums.EventTypeEnum.START, 'addr.local'];
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

	  var duration = (0, _moment2.default)(last_active).diff(created);
	  var nodeDisplayText = getNodeIpDisplayText(siteId, server_id, nodeIp);
	  var createdDisplayText = getCreatedDisplayText(created);
	  var sessionUrl = _config2.default.getPlayerUrl({
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

	  var duration = (0, _moment2.default)(last_active).diff(created);
	  var nodeIp = _reactor2.default.evaluate(nodeIpById(sid));
	  var nodeDisplayText = getNodeIpDisplayText(siteId, server_id, nodeIp);
	  var createdDisplayText = getCreatedDisplayText(created);
	  var sessionUrl = _config2.default.getTerminalLoginUrl({
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
	    var ip = (0, _objectUtils.parseIp)(p.serverIp);
	    return p.user + ' [' + ip + ']';
	  });
	}

	function getCreatedDisplayText(date) {
	  return (0, _moment2.default)(date).format(_config2.default.displayDateFormat);
	}

	function getNodeIpDisplayText(siteId, serverId, serverIp) {
	  var server = (0, _nodeStore.getNodeStore)().findServer(serverId);
	  var ipAddress = (0, _objectUtils.parseIp)(serverIp);

	  var displayText = ipAddress;
	  if (server && server.hostname) {
	    displayText = server.hostname;
	    if (ipAddress) {
	      displayText = displayText + ' [' + ipAddress + ']';
	    }
	  }

	  return displayText;
	}

	exports.default = {
	  storedSessionList: storedSessionList,
	  activeSessionList: activeSessionList,
	  activeSessionById: activeSessionById,
	  activePartiesById: activePartiesById,
	  storedSessionById: storedSessionById,
	  createStoredListItem: createStoredListItem
	};
	module.exports = exports['default'];

/***/ }),
/* 408 */
/***/ (function(module, exports) {

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
	var TLPT_TERMINAL_INIT = exports.TLPT_TERMINAL_INIT = 'TLPT_TERMINAL_INIT';
	var TLPT_TERMINAL_CLOSE = exports.TLPT_TERMINAL_CLOSE = 'TLPT_TERMINAL_CLOSE';
	var TLPT_TERMINAL_SET_STATUS = exports.TLPT_TERMINAL_SET_STATUS = 'TLPT_TERMINAL_SET_STATUS';

/***/ }),
/* 409 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.open = open;
	exports.close = close;

	var _history = __webpack_require__(226);

	var _history2 = _interopRequireDefault(_history);

	var _config = __webpack_require__(228);

	var _config2 = _interopRequireDefault(_config);

	var _store = __webpack_require__(410);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function open(siteId, sid) {
	  var routeUrl = _config2.default.getPlayerUrl({ siteId: siteId, sid: sid });
	  _history2.default.push(routeUrl);
	} /*
	  Copyright 2015 Gravitational, Inc.
	  
	  Licensed under the Apache License, Version 2.0 (the "License");
	  you may not use this file except in compliance with the License.
	  You may obtain a copy of the License at
	  
	      http://www.apache.org/licenses/LICENSE-2.0
	  
	  Unless required by applicable law or agreed to in writing, software
	  distributed under the License is distributed on an "AS IS" BASIS,
	  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	  See the License for the specific language governing permissions and
	  limitations under the License.
	  */

	function close() {
	  var canListSessions = (0, _store.getAcl)().getSessionAccess().read;
	  var redirect = canListSessions ? _config2.default.routes.sessions : _config2.default.routes.app;
	  _history2.default.push(redirect);
	}

/***/ }),
/* 410 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.getAcl = getAcl;

	var _reactor = __webpack_require__(233);

	var _reactor2 = _interopRequireDefault(_reactor);

	var _nuclearJs = __webpack_require__(234);

	var _immutable = __webpack_require__(253);

	var _actionTypes = __webpack_require__(287);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
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

	var Access = new _immutable.Record({
	  list: false,
	  read: false,
	  edit: false,
	  create: false,
	  remove: false
	});

	var AccessListRec = function (_Record) {
	  _inherits(AccessListRec, _Record);

	  function AccessListRec() {
	    var json = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : {};

	    _classCallCheck(this, AccessListRec);

	    var map = (0, _nuclearJs.toImmutable)(json);
	    var sshLogins = new _immutable.List(map.get('sshLogins'));
	    var params = {
	      sshLogins: sortLogins(sshLogins),
	      authConnectors: new Access(map.get('authConnectors')),
	      trustedClusters: new Access(map.get('trustedClusters')),
	      roles: new Access(map.get('roles')),
	      sessions: new Access(map.get('sessions'))
	    };

	    return _possibleConstructorReturn(this, _Record.call(this, params));
	  }

	  AccessListRec.prototype.getSessionAccess = function getSessionAccess() {
	    return this.get('sessions');
	  };

	  AccessListRec.prototype.getRoleAccess = function getRoleAccess() {
	    return this.get('roles');
	  };

	  AccessListRec.prototype.getConnectorAccess = function getConnectorAccess() {
	    return this.get('authConnectors');
	  };

	  AccessListRec.prototype.getClusterAccess = function getClusterAccess() {
	    return this.get('trustedClusters');
	  };

	  AccessListRec.prototype.getSshLogins = function getSshLogins() {
	    return this.get('sshLogins');
	  };

	  return AccessListRec;
	}((0, _immutable.Record)({
	  authConnectors: new Access(),
	  trustedClusters: new Access(),
	  roles: new Access(),
	  sessions: new Access(),
	  sshLogins: new _immutable.List()
	}));

	function getAcl() {
	  return _reactor2.default.evaluate(['tlpt_user_acl']);
	}

	exports.default = (0, _nuclearJs.Store)({
	  getInitialState: function getInitialState() {
	    return new AccessListRec();
	  },
	  initialize: function initialize() {
	    this.on(_actionTypes.RECEIVE_USERACL, function (state, json) {
	      return new AccessListRec(json);
	    });
	  }
	});

/***/ }),
/* 411 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _icons = __webpack_require__(256);

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

	var closeTextStyle = {
	  width: "30px",
	  height: "30px",
	  display: "block",
	  margin: "0 auto"
	};

	var PartyListPanel = function PartyListPanel(_ref) {
	  var onClose = _ref.onClose,
	      children = _ref.children;

	  return _react2.default.createElement(
	    'div',
	    { className: 'grv-terminal-participans' },
	    _react2.default.createElement(
	      'ul',
	      { className: 'nav' },
	      _react2.default.createElement(
	        'li',
	        { title: 'Close' },
	        _react2.default.createElement(
	          'div',
	          { style: closeTextStyle, onClick: onClose },
	          _react2.default.createElement(_icons.CloseIcon, null)
	        )
	      )
	    ),
	    children ? _react2.default.createElement('hr', { className: 'grv-divider' }) : null,
	    children
	  );
	};

	exports.default = PartyListPanel;
	module.exports = exports['default'];

/***/ }),
/* 412 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _reactAddonsCssTransitionGroup = __webpack_require__(413);

	var _reactAddonsCssTransitionGroup2 = _interopRequireDefault(_reactAddonsCssTransitionGroup);

	var _nuclearJsReactAddons = __webpack_require__(219);

	var _getters = __webpack_require__(407);

	var _getters2 = _interopRequireDefault(_getters);

	var _icons = __webpack_require__(256);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	var PartyList = function PartyList(props) {
	  var parties = props.parties || [];
	  var userIcons = parties.map(function (item, index) {
	    return _react2.default.createElement(
	      'li',
	      { key: index, className: 'animated' },
	      _react2.default.createElement(_icons.UserIcon, { colorIndex: index,
	        isDark: true,
	        name: item.user })
	    );
	  });

	  return _react2.default.createElement(
	    _reactAddonsCssTransitionGroup2.default,
	    { className: 'nav', component: 'ul',
	      transitionEnterTimeout: 500,
	      transitionLeaveTimeout: 500,
	      transitionName: {
	        enter: "fadeIn",
	        leave: "fadeOut"
	      } },
	    userIcons
	  );
	}; /*
	   Copyright 2015 Gravitational, Inc.
	   
	   Licensed under the Apache License, Version 2.0 (the "License");
	   you may not use this file except in compliance with the License.
	   You may obtain a copy of the License at
	   
	       http://www.apache.org/licenses/LICENSE-2.0
	   
	   Unless required by applicable law or agreed to in writing, software
	   distributed under the License is distributed on an "AS IS" BASIS,
	   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	   See the License for the specific language governing permissions and
	   limitations under the License.
	   */

	function mapStateToProps(props) {
	  return {
	    parties: _getters2.default.activePartiesById(props.sid)
	  };
	}

	exports.default = (0, _nuclearJsReactAddons.connect)(mapStateToProps)(PartyList);
	module.exports = exports['default'];

/***/ }),
/* 413 */,
/* 414 */,
/* 415 */,
/* 416 */,
/* 417 */,
/* 418 */,
/* 419 */,
/* 420 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _config = __webpack_require__(228);

	var _config2 = _interopRequireDefault(_config);

	var _featureBase = __webpack_require__(393);

	var _featureBase2 = _interopRequireDefault(_featureBase);

	var _actions = __webpack_require__(284);

	var _main = __webpack_require__(421);

	var _main2 = _interopRequireDefault(_main);

	var _playerHost = __webpack_require__(486);

	var _playerHost2 = _interopRequireDefault(_playerHost);

	var _reactor = __webpack_require__(233);

	var _reactor2 = _interopRequireDefault(_reactor);

	var _actions2 = __webpack_require__(423);

	var _store = __webpack_require__(410);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
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
	  to: _config2.default.routes.sessions,
	  title: 'Sessions'
	};

	var AuditFeature = function (_FeatureBase) {
	  _inherits(AuditFeature, _FeatureBase);

	  AuditFeature.prototype.componentDidMount = function componentDidMount() {
	    this.init();
	  };

	  AuditFeature.prototype.init = function init() {
	    var _this2 = this;

	    if (!this.wasInitialized()) {
	      _reactor2.default.batch(function () {
	        _this2.startProcessing();
	        (0, _actions2.fetchSiteEventsWithinTimeRange)().done(_this2.stopProcessing.bind(_this2)).fail(_this2.handleError.bind(_this2));
	      });
	    }
	  };

	  function AuditFeature(routes) {
	    _classCallCheck(this, AuditFeature);

	    var _this = _possibleConstructorReturn(this, _FeatureBase.call(this));

	    var auditRoutes = [{
	      path: _config2.default.routes.sessions,
	      title: "Stored Sessions",
	      component: _this.withMe(_main2.default)
	    }, {
	      path: _config2.default.routes.player,
	      title: "Player",
	      components: {
	        CurrentSessionHost: _playerHost2.default
	      }
	    }];

	    routes.push.apply(routes, auditRoutes);
	    return _this;
	  }

	  AuditFeature.prototype.onload = function onload() {
	    var sessAccess = (0, _store.getAcl)().getSessionAccess();
	    if (sessAccess.list) {
	      (0, _actions.addNavItem)(auditNavItem);
	      this.init();
	    }
	  };

	  return AuditFeature;
	}(_featureBase2.default);

	exports.default = AuditFeature;
	module.exports = exports['default'];

/***/ }),
/* 421 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _connect = __webpack_require__(422);

	var _connect2 = _interopRequireDefault(_connect);

	var _actions = __webpack_require__(423);

	var _getters = __webpack_require__(407);

	var _getters2 = __webpack_require__(424);

	var _dataProvider = __webpack_require__(426);

	var _dataProvider2 = _interopRequireDefault(_dataProvider);

	var _sessionList = __webpack_require__(427);

	var _sessionList2 = _interopRequireDefault(_sessionList);

	var _withStorage = __webpack_require__(391);

	var _withStorage2 = _interopRequireDefault(_withStorage);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               limitations under the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               */

	var Sessions = function (_React$Component) {
	  _inherits(Sessions, _React$Component);

	  function Sessions() {
	    var _temp, _this, _ret;

	    _classCallCheck(this, Sessions);

	    for (var _len = arguments.length, args = Array(_len), _key = 0; _key < _len; _key++) {
	      args[_key] = arguments[_key];
	    }

	    return _ret = (_temp = (_this = _possibleConstructorReturn(this, _React$Component.call.apply(_React$Component, [this].concat(args))), _this), _this.refresh = function () {
	      return (0, _actions.fetchSiteEventsWithinTimeRange)();
	    }, _temp), _possibleConstructorReturn(_this, _ret);
	  }

	  Sessions.prototype.render = function render() {
	    var _props = this.props,
	        storedSessions = _props.storedSessions,
	        activeSessions = _props.activeSessions,
	        storedSessionsFilter = _props.storedSessionsFilter;

	    return _react2.default.createElement(
	      'div',
	      { className: 'grv-page grv-sessions' },
	      _react2.default.createElement(_sessionList2.default, {
	        storage: this.props.storage,
	        activeSessions: activeSessions,
	        storedSessions: storedSessions,
	        filter: storedSessionsFilter
	      }),
	      _react2.default.createElement(_dataProvider2.default, { onFetch: this.refresh })
	    );
	  };

	  return Sessions;
	}(_react2.default.Component);

	function mapFluxToProps() {
	  return {
	    activeSessions: _getters.activeSessionList,
	    storedSessions: _getters.storedSessionList,
	    storedSessionsFilter: _getters2.filter
	  };
	}

	var SessionsWithStorage = (0, _withStorage2.default)(Sessions);

	exports.default = (0, _connect2.default)(mapFluxToProps)(SessionsWithStorage);
	module.exports = exports['default'];

/***/ }),
/* 422 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

	exports.default = connect;

	var _react = __webpack_require__(2);

	var _hoistNonReactStatics = __webpack_require__(188);

	var _hoistNonReactStatics2 = _interopRequireDefault(_hoistNonReactStatics);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               limitations under the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               */

	var reactorShape = _react.PropTypes.shape({
	  dispatch: _react.PropTypes.func.isRequired,
	  evaluate: _react.PropTypes.func.isRequired,
	  evaluateToJS: _react.PropTypes.func.isRequired,
	  observe: _react.PropTypes.func.isRequired
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
	      _inherits(Connect, _Component);

	      function Connect(props, context) {
	        _classCallCheck(this, Connect);

	        var _this = _possibleConstructorReturn(this, _Component.call(this, props, context));

	        _this.reactor = props.reactor || context.reactor;
	        _this.unsubscribeFns = [];
	        _this.updatePropMap(props);
	        return _this;
	      }

	      Connect.prototype.resubscribe = function resubscribe(props) {
	        this.unsubscribe();
	        this.updatePropMap(props);
	        this.updateState();
	        this.subscribe();
	      };

	      Connect.prototype.componentWillMount = function componentWillMount() {
	        this.updateState();
	        this.subscribe(this.props);
	      };

	      Connect.prototype.componentWillUnmount = function componentWillUnmount() {
	        this.unsubscribe();
	      };

	      Connect.prototype.updatePropMap = function updatePropMap(props) {
	        this.propMap = mapFluxToProps ? mapFluxToProps(props) : {};
	      };

	      Connect.prototype.updateState = function updateState() {
	        var propMap = this.propMap;
	        var stateToSet = {};

	        for (var key in propMap) {
	          var getter = propMap[key];
	          stateToSet[key] = this.reactor.evaluate(getter);
	        }

	        this.setState(stateToSet);
	      };

	      Connect.prototype.subscribe = function subscribe() {
	        var _this2 = this;

	        var propMap = this.propMap;

	        var _loop = function _loop(key) {
	          var getter = propMap[key];
	          var unsubscribeFn = _this2.reactor.observe(getter, function (val) {
	            var _this2$setState;

	            _this2.setState((_this2$setState = {}, _this2$setState[key] = val, _this2$setState));
	          });

	          _this2.unsubscribeFns.push(unsubscribeFn);
	        };

	        for (var key in propMap) {
	          _loop(key);
	        }
	      };

	      Connect.prototype.unsubscribe = function unsubscribe() {
	        if (this.unsubscribeFns.length === 0) {
	          return;
	        }

	        while (this.unsubscribeFns.length > 0) {
	          this.unsubscribeFns.shift()();
	        }
	      };

	      Connect.prototype.render = function render() {
	        var stateProps = mapStateToProps(this.props);
	        return (0, _react.createElement)(WrappedComponent, _extends({
	          reactor: this.reactor
	        }, stateProps, this.props, this.state));
	      };

	      return Connect;
	    }(_react.Component);

	    Connect.displayName = 'Connect(' + getDisplayName(WrappedComponent) + ')';
	    Connect.WrappedComponent = WrappedComponent;
	    Connect.contextTypes = {
	      reactor: reactorShape
	    };
	    Connect.propTypes = {
	      reactor: reactorShape
	    };

	    return (0, _hoistNonReactStatics2.default)(Connect, WrappedComponent);
	  };
	}
	module.exports = exports['default'];

/***/ }),
/* 423 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _reactor = __webpack_require__(233);

	var _reactor2 = _interopRequireDefault(_reactor);

	var _getters = __webpack_require__(424);

	var _actions = __webpack_require__(290);

	var _actionTypes = __webpack_require__(425);

	var _logger = __webpack_require__(245);

	var _logger2 = _interopRequireDefault(_logger);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	var logger = _logger2.default.create('Modules/Sessions'); /*
	                                                          Copyright 2015 Gravitational, Inc.
	                                                          
	                                                          Licensed under the Apache License, Version 2.0 (the "License");
	                                                          you may not use this file except in compliance with the License.
	                                                          You may obtain a copy of the License at
	                                                          
	                                                              http://www.apache.org/licenses/LICENSE-2.0
	                                                          
	                                                          Unless required by applicable law or agreed to in writing, software
	                                                          distributed under the License is distributed on an "AS IS" BASIS,
	                                                          WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                          See the License for the specific language governing permissions and
	                                                          limitations under the License.
	                                                          */

	var actions = {
	  fetchSiteEventsWithinTimeRange: function fetchSiteEventsWithinTimeRange() {
	    var _reactor$evaluate = _reactor2.default.evaluate(_getters.filter),
	        start = _reactor$evaluate.start,
	        end = _reactor$evaluate.end;

	    return _fetch(start, end);
	  },
	  setTimeRange: function setTimeRange(start, end) {
	    _reactor2.default.batch(function () {
	      _reactor2.default.dispatch(_actionTypes.TLPT_STORED_SESSINS_FILTER_SET_RANGE, { start: start, end: end });
	      _fetch(start, end);
	    });
	  }
	};

	function _fetch(start, end) {
	  return (0, _actions.fetchSiteEvents)(start, end).fail(function (err) {
	    logger.error('fetching filtered set of sessions', err);
	  });
	}

	exports.default = actions;
	module.exports = exports['default'];

/***/ }),
/* 424 */
/***/ (function(module, exports) {

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

	var filter = [['tlpt_sessions_filter'], function (filter) {
	  return filter.toJS();
	}];

	exports.default = {
	  filter: filter
	};
	module.exports = exports['default'];

/***/ }),
/* 425 */
/***/ (function(module, exports) {

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

	var TLPT_STORED_SESSINS_FILTER_SET_RANGE = exports.TLPT_STORED_SESSINS_FILTER_SET_RANGE = 'TLPT_STORED_SESSINS_FILTER_SET_RANGE';
	var TLPT_STORED_SESSINS_FILTER_SET_STATUS = exports.TLPT_STORED_SESSINS_FILTER_SET_STATUS = 'TLPT_STORED_SESSINS_FILTER_SET_STATUS';
	var TLPT_STORED_SESSINS_FILTER_RECEIVE_MORE = exports.TLPT_STORED_SESSINS_FILTER_RECEIVE_MORE = 'TLPT_STORED_SESSINS_FILTER_RECEIVE_MORE';

/***/ }),
/* 426 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _react = __webpack_require__(2);

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
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
	  _inherits(DataProvider, _Component);

	  function DataProvider(props) {
	    _classCallCheck(this, DataProvider);

	    var _this = _possibleConstructorReturn(this, _Component.call(this, props));

	    _this._timerId = null;
	    _this._request = null;

	    _this._intervalTime = props.time || DEFAULT_INTERVAL;
	    return _this;
	  }

	  DataProvider.prototype.fetch = function fetch() {
	    var _this2 = this;

	    // do not refetch if still in progress
	    if (this._request) {
	      return;
	    }

	    this._request = this.props.onFetch().always(function () {
	      _this2._request = null;
	    });
	  };

	  DataProvider.prototype.componentDidMount = function componentDidMount() {
	    this.fetch();
	    this._timerId = setInterval(this.fetch.bind(this), this._intervalTime);
	  };

	  DataProvider.prototype.componentWillUnmount = function componentWillUnmount() {
	    clearInterval(this._timerId);
	  };

	  DataProvider.prototype.render = function render() {
	    return null;
	  };

	  return DataProvider;
	}(_react.Component);

	exports.default = DataProvider;
	module.exports = exports['default'];

/***/ }),
/* 427 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _lodash = __webpack_require__(275);

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _moment = __webpack_require__(291);

	var _moment2 = _interopRequireDefault(_moment);

	var _inputSearch = __webpack_require__(278);

	var _inputSearch2 = _interopRequireDefault(_inputSearch);

	var _objectUtils = __webpack_require__(277);

	var _storedSessionsFilter = __webpack_require__(428);

	var _table = __webpack_require__(280);

	var _listItems = __webpack_require__(429);

	var _datePicker = __webpack_require__(485);

	var _datePicker2 = _interopRequireDefault(_datePicker);

	var _clusterSelector = __webpack_require__(281);

	var _clusterSelector2 = _interopRequireDefault(_clusterSelector);

	var _config = __webpack_require__(228);

	var _config2 = _interopRequireDefault(_config);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               limitations under the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               */

	var SessionList = function (_React$Component) {
	  _inherits(SessionList, _React$Component);

	  function SessionList(props) {
	    _classCallCheck(this, SessionList);

	    var _this = _possibleConstructorReturn(this, _React$Component.call(this, props));

	    _this.searchableProps = ['nodeDisplayText', 'createdDisplayText', 'sid', 'parties'];
	    _this._mounted = false;

	    _this.onSearchChange = function (value) {
	      _this.state.searchValue = value;
	      _this.setState(_this.state);
	    };

	    _this.onSortChange = function (columnKey, sortDir) {
	      var _this$state$colSortDi;

	      _this.state.colSortDirs = (_this$state$colSortDi = {}, _this$state$colSortDi[columnKey] = sortDir, _this$state$colSortDi);
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
	        _storedSessionsFilter.actions.setTimeRange(startDate, endDate);
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

	  SessionList.prototype.componentDidMount = function componentDidMount() {
	    this._mounted = true;
	  };

	  SessionList.prototype.componentWillUnmount = function componentWillUnmount() {
	    this._mounted = false;
	    if (this.props.storage) {
	      this.props.storage.save('SessionList', this.state);
	    }
	  };

	  SessionList.prototype.searchAndFilterCb = function searchAndFilterCb(targetValue, searchValue, propName) {
	    if (propName === 'parties') {
	      targetValue = targetValue || [];
	      return targetValue.join('').toLocaleUpperCase().indexOf(searchValue) !== -1;
	    }
	  };

	  SessionList.prototype.sortAndFilter = function sortAndFilter(data) {
	    var _this2 = this;

	    var filtered = data.filter(function (obj) {
	      return (0, _objectUtils.isMatch)(obj, _this2.state.searchValue, {
	        searchableProps: _this2.searchableProps,
	        cb: _this2.searchAndFilterCb
	      });
	    });

	    var columnKey = Object.getOwnPropertyNames(this.state.colSortDirs)[0];
	    var sortDir = this.state.colSortDirs[columnKey];
	    var sorted = (0, _lodash.sortBy)(filtered, columnKey);
	    if (sortDir === _table.SortTypes.ASC) {
	      sorted = sorted.reverse();
	    }

	    return sorted;
	  };

	  SessionList.prototype.render = function render() {
	    var _props = this.props,
	        filter = _props.filter,
	        storedSessions = _props.storedSessions,
	        activeSessions = _props.activeSessions;
	    var start = filter.start,
	        end = filter.end;

	    var canJoin = _config2.default.canJoinSessions;
	    var searchValue = this.state.searchValue;

	    var stored = storedSessions.filter(function (item) {
	      return (0, _moment2.default)(item.created).isBetween(start, end);
	    });

	    var active = activeSessions.filter(function (item) {
	      return item.parties.length > 0;
	    }).filter(function (item) {
	      return (0, _moment2.default)(item.created).isBetween(start, end);
	    });

	    stored = this.sortAndFilter(stored);
	    active = this.sortAndFilter(active);

	    // always display active sessions first    
	    var data = [].concat(active, stored);
	    return _react2.default.createElement(
	      'div',
	      { className: 'grv-sessions-stored m-t' },
	      _react2.default.createElement(
	        'div',
	        { className: 'grv-header' },
	        _react2.default.createElement(
	          'div',
	          { className: 'grv-flex m-b-md', style: { justifyContent: "space-between" } },
	          _react2.default.createElement(
	            'div',
	            { className: 'grv-flex' },
	            _react2.default.createElement(
	              'h2',
	              { className: 'text-center' },
	              ' Sessions '
	            )
	          ),
	          _react2.default.createElement(
	            'div',
	            { className: 'grv-flex' },
	            _react2.default.createElement(_clusterSelector2.default, null),
	            _react2.default.createElement(_inputSearch2.default, { value: searchValue, onChange: this.onSearchChange }),
	            _react2.default.createElement(
	              'div',
	              { className: 'm-l-sm' },
	              _react2.default.createElement(_datePicker2.default, { startDate: start, endDate: end, onChange: this.onRangePickerChange })
	            )
	          )
	        )
	      ),
	      _react2.default.createElement(
	        'div',
	        { className: 'grv-content' },
	        data.length === 0 ? _react2.default.createElement(_table.EmptyIndicator, { text: 'No matching sessions found' }) : _react2.default.createElement(
	          _table.Table,
	          { rowCount: data.length },
	          _react2.default.createElement(_table.Column, {
	            header: _react2.default.createElement(
	              _table.Cell,
	              { className: 'grv-sessions-col-sid' },
	              ' Session ID '
	            ),
	            cell: _react2.default.createElement(_listItems.SessionIdCell, { canJoin: canJoin, data: data, container: this })
	          }),
	          _react2.default.createElement(_table.Column, {
	            header: _react2.default.createElement(
	              _table.Cell,
	              null,
	              ' User '
	            ),
	            cell: _react2.default.createElement(_listItems.UsersCell, { data: data })
	          }),
	          _react2.default.createElement(_table.Column, {
	            columnKey: 'nodeIp',
	            header: _react2.default.createElement(
	              _table.Cell,
	              { className: 'grv-sessions-stored-col-ip' },
	              'Node'
	            ),
	            cell: _react2.default.createElement(_listItems.NodeCell, { data: data })
	          }),
	          _react2.default.createElement(_table.Column, {
	            columnKey: 'created',
	            header: _react2.default.createElement(_table.SortHeaderCell, {
	              sortDir: this.state.colSortDirs.created,
	              onSortChange: this.onSortChange,
	              title: 'Created (UTC)'
	            }),
	            cell: _react2.default.createElement(_listItems.DateCreatedCell, { data: data })
	          }),
	          _react2.default.createElement(_table.Column, {
	            columnKey: 'duration',
	            header: _react2.default.createElement(_table.SortHeaderCell, {
	              sortDir: this.state.colSortDirs.duration,
	              onSortChange: this.onSortChange,
	              title: 'Duration'
	            }),
	            cell: _react2.default.createElement(_listItems.DurationCell, { data: data })
	          })
	        )
	      )
	    );
	  };

	  return SessionList;
	}(_react2.default.Component);

	exports.default = SessionList;
	module.exports = exports['default'];

/***/ }),
/* 428 */
/***/ (function(module, exports, __webpack_require__) {

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
	module.exports.getters = __webpack_require__(424);
	module.exports.actions = __webpack_require__(423);

/***/ }),
/* 429 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.NodeCell = exports.SingleUserCell = exports.DateCreatedCell = exports.DurationCell = exports.UsersCell = exports.SessionIdCell = undefined;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _reactRouter = __webpack_require__(164);

	var _table = __webpack_require__(280);

	var _moment = __webpack_require__(291);

	var _moment2 = _interopRequireDefault(_moment);

	var _layout = __webpack_require__(430);

	var _layout2 = _interopRequireDefault(_layout);

	var _moreButton = __webpack_require__(431);

	var _moreButton2 = _interopRequireDefault(_moreButton);

	var _popover = __webpack_require__(484);

	var _popover2 = _interopRequireDefault(_popover);

	var _classnames = __webpack_require__(257);

	var _classnames2 = _interopRequireDefault(_classnames);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; } /*
	                                                                                                                                                                                                                             Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                             
	                                                                                                                                                                                                                             Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                             you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                             You may obtain a copy of the License at
	                                                                                                                                                                                                                             
	                                                                                                                                                                                                                                 http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                             
	                                                                                                                                                                                                                             Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                             distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                             WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                             See the License for the specific language governing permissions and
	                                                                                                                                                                                                                             limitations under the License.
	                                                                                                                                                                                                                             */

	var DateCreatedCell = function DateCreatedCell(_ref) {
	  var rowIndex = _ref.rowIndex,
	      data = _ref.data,
	      props = _objectWithoutProperties(_ref, ['rowIndex', 'data']);

	  var createdDisplayText = data[rowIndex].createdDisplayText;

	  return _react2.default.createElement(
	    _table.Cell,
	    props,
	    createdDisplayText
	  );
	};

	var DurationCell = function DurationCell(_ref2) {
	  var rowIndex = _ref2.rowIndex,
	      data = _ref2.data,
	      props = _objectWithoutProperties(_ref2, ['rowIndex', 'data']);

	  var duration = data[rowIndex].duration;

	  var displayDate = _moment2.default.duration(duration).humanize();
	  return _react2.default.createElement(
	    _table.Cell,
	    props,
	    displayDate
	  );
	};

	var SingleUserCell = function SingleUserCell(_ref3) {
	  var rowIndex = _ref3.rowIndex,
	      data = _ref3.data,
	      props = _objectWithoutProperties(_ref3, ['rowIndex', 'data']);

	  var user = data[rowIndex].user;

	  return _react2.default.createElement(
	    _table.Cell,
	    props,
	    _react2.default.createElement(
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

	  var _data$rowIndex = data[rowIndex],
	      parties = _data$rowIndex.parties,
	      user = _data$rowIndex.user;

	  var $users = _react2.default.createElement(
	    'div',
	    { className: 'grv-sessions-user' },
	    user
	  );

	  if (parties.length > 0) {
	    $users = parties.map(function (item, itemIndex) {
	      return _react2.default.createElement(
	        'div',
	        { key: itemIndex, className: 'grv-sessions-user' },
	        item
	      );
	    });
	  }

	  return _react2.default.createElement(
	    _table.Cell,
	    props,
	    _react2.default.createElement(
	      'div',
	      null,
	      $users
	    )
	  );
	};

	var sessionInfo = function sessionInfo(sid) {
	  return _react2.default.createElement(
	    _popover2.default,
	    { className: 'grv-sessions-stored-details' },
	    _react2.default.createElement(
	      'div',
	      null,
	      sid
	    )
	  );
	};

	var SessionIdCell = function SessionIdCell(_ref5) {
	  var rowIndex = _ref5.rowIndex,
	      canJoin = _ref5.canJoin,
	      data = _ref5.data,
	      container = _ref5.container,
	      props = _objectWithoutProperties(_ref5, ['rowIndex', 'canJoin', 'data', 'container']);

	  var _data$rowIndex2 = data[rowIndex],
	      sessionUrl = _data$rowIndex2.sessionUrl,
	      active = _data$rowIndex2.active,
	      sid = _data$rowIndex2.sid;

	  var isDisabled = active && !canJoin;
	  var sidShort = sid.slice(0, 8);
	  var actionText = active ? 'join' : 'play';

	  var btnClass = (0, _classnames2.default)('btn btn-xs m-r-sm', {
	    'btn-primary': !active,
	    'btn-warning': active,
	    'disabled': isDisabled
	  });

	  return _react2.default.createElement(
	    _table.Cell,
	    props,
	    _react2.default.createElement(
	      _layout2.default.Flex,
	      { dir: 'row', align: 'center' },
	      isDisabled && _react2.default.createElement(
	        'button',
	        { className: btnClass },
	        actionText
	      ),
	      !isDisabled && _react2.default.createElement(
	        _reactRouter.Link,
	        { to: sessionUrl, className: btnClass, type: 'button' },
	        actionText
	      ),
	      _react2.default.createElement(
	        'span',
	        { style: { width: "75px" } },
	        sidShort
	      ),
	      _react2.default.createElement(_moreButton2.default.WithOverlay, {
	        trigger: 'click',
	        placement: 'bottom',
	        container: container,
	        overlay: sessionInfo(sid) })
	    )
	  );
	};

	var NodeCell = function NodeCell(_ref6) {
	  var rowIndex = _ref6.rowIndex,
	      data = _ref6.data,
	      props = _objectWithoutProperties(_ref6, ['rowIndex', 'data']);

	  var nodeDisplayText = data[rowIndex].nodeDisplayText;

	  return _react2.default.createElement(
	    _table.Cell,
	    props,
	    nodeDisplayText
	  );
	};

	exports.SessionIdCell = SessionIdCell;
	exports.UsersCell = UsersCell;
	exports.DurationCell = DurationCell;
	exports.DateCreatedCell = DateCreatedCell;
	exports.SingleUserCell = SingleUserCell;
	exports.NodeCell = NodeCell;

/***/ }),
/* 430 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; }; /*
	                                                                                                                                                                                                                                                                  Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                  
	                                                                                                                                                                                                                                                                  Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                  you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                  You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                  
	                                                                                                                                                                                                                                                                      http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                  
	                                                                                                                                                                                                                                                                  Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                  distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                  See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                  limitations under the License.
	                                                                                                                                                                                                                                                                  */

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _objectWithoutProperties(obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; }

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

	  return _extends({}, style, styles.flex, styles.dir[dir], styles.justify[justify], styles.align[align]);
	};

	var Flex = function Flex(_ref2) {
	  var _ref2$className = _ref2.className,
	      className = _ref2$className === undefined ? '' : _ref2$className,
	      children = _ref2.children,
	      props = _objectWithoutProperties(_ref2, ['className', 'children']);

	  return _react2.default.createElement(
	    'div',
	    { className: className, style: getStyle(props) },
	    children
	  );
	};

	exports.default = {
	  Flex: Flex
	};
	module.exports = exports['default'];

/***/ }),
/* 431 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _classnames = __webpack_require__(257);

	var _classnames2 = _interopRequireDefault(_classnames);

	var _overlayTrigger = __webpack_require__(432);

	var _overlayTrigger2 = _interopRequireDefault(_overlayTrigger);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	var classes = {
	  'btn grv-btn-details': true
	}; /*
	   Copyright 2015 Gravitational, Inc.
	   
	   Licensed under the Apache License, Version 2.0 (the "License");
	   you may not use this file except in compliance with the License.
	   You may obtain a copy of the License at
	   
	       http://www.apache.org/licenses/LICENSE-2.0
	   
	   Unless required by applicable law or agreed to in writing, software
	   distributed under the License is distributed on an "AS IS" BASIS,
	   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	   See the License for the specific language governing permissions and
	   limitations under the License.
	   */

	var MoreButton = function MoreButton(props) {
	  return _react2.default.createElement(
	    'button',
	    { className: (0, _classnames2.default)(props.className, classes) },
	    _react2.default.createElement(
	      'span',
	      null,
	      '\u2026'
	    )
	  );
	};

	MoreButton.WithOverlay = function (props) {
	  return _react2.default.createElement(
	    _overlayTrigger2.default,
	    props,
	    _react2.default.createElement(MoreButton, null)
	  );
	};

	exports.default = MoreButton;
	module.exports = exports['default'];

/***/ }),
/* 432 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _reactDom = __webpack_require__(31);

	var _reactDom2 = _interopRequireDefault(_reactDom);

	var _reactOverlays = __webpack_require__(433);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               limitations under the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               */

	var triggerType = _react2.default.PropTypes.oneOf(['click', 'hover', 'focus']);

	var propTypes = {

	  trigger: _react2.default.PropTypes.oneOfType([triggerType, _react2.default.PropTypes.arrayOf(triggerType)]),

	  delay: _react2.default.PropTypes.number,

	  delayShow: _react2.default.PropTypes.number,

	  delayHide: _react2.default.PropTypes.number,

	  defaultOverlayShown: _react2.default.PropTypes.bool,

	  overlay: _react2.default.PropTypes.node.isRequired,

	  onBlur: _react2.default.PropTypes.func,

	  onClick: _react2.default.PropTypes.func,

	  onFocus: _react2.default.PropTypes.func,

	  onMouseOut: _react2.default.PropTypes.func,

	  onMouseOver: _react2.default.PropTypes.func,

	  target: _react2.default.PropTypes.oneOf([null]),

	  onHide: _react2.default.PropTypes.oneOf([null]),

	  show: _react2.default.PropTypes.oneOf([null])
	};

	var defaultProps = {
	  defaultOverlayShown: false,
	  trigger: ['hover', 'focus']
	};

	var OverlayTrigger = function (_React$Component) {
	  _inherits(OverlayTrigger, _React$Component);

	  function OverlayTrigger(props, context) {
	    _classCallCheck(this, OverlayTrigger);

	    var _this = _possibleConstructorReturn(this, _React$Component.call(this, props, context));

	    _this.getElement = _this.getElement.bind(_this);
	    _this.handleToggle = _this.handleToggle.bind(_this);
	    _this.handleHide = _this.handleHide.bind(_this);
	    _this.state = {
	      show: props.defaultOverlayShown
	    };
	    return _this;
	  }

	  OverlayTrigger.prototype.handleToggle = function handleToggle() {
	    if (this.state.show) {
	      this.hide();
	    } else {
	      this.show();
	    }
	  };

	  OverlayTrigger.prototype.handleHide = function handleHide() {
	    this.hide();
	  };

	  OverlayTrigger.prototype.show = function show() {
	    this.setState({ show: true });
	  };

	  OverlayTrigger.prototype.hide = function hide() {
	    this.setState({ show: false });
	  };

	  OverlayTrigger.prototype.getElement = function getElement() {
	    return _reactDom2.default.findDOMNode(this);
	  };

	  OverlayTrigger.prototype.render = function render() {
	    var _this2 = this;

	    var _props = this.props,
	        _props$container = _props.container,
	        container = _props$container === undefined ? this : _props$container,
	        placement = _props.placement,
	        overlay = _props.overlay;

	    return _react2.default.createElement(
	      'div',
	      { onClick: this.handleToggle },
	      this.props.children,
	      _react2.default.createElement(
	        _reactOverlays.Overlay,
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
	  };

	  return OverlayTrigger;
	}(_react2.default.Component);

	OverlayTrigger.propTypes = propTypes;
	OverlayTrigger.defaultProps = defaultProps;

	exports.default = OverlayTrigger;
	module.exports = exports['default'];

/***/ }),
/* 433 */,
/* 434 */,
/* 435 */,
/* 436 */,
/* 437 */,
/* 438 */,
/* 439 */,
/* 440 */,
/* 441 */,
/* 442 */,
/* 443 */,
/* 444 */,
/* 445 */,
/* 446 */,
/* 447 */,
/* 448 */,
/* 449 */,
/* 450 */,
/* 451 */,
/* 452 */,
/* 453 */,
/* 454 */,
/* 455 */,
/* 456 */,
/* 457 */,
/* 458 */,
/* 459 */,
/* 460 */,
/* 461 */,
/* 462 */,
/* 463 */,
/* 464 */,
/* 465 */,
/* 466 */,
/* 467 */,
/* 468 */,
/* 469 */,
/* 470 */,
/* 471 */,
/* 472 */,
/* 473 */,
/* 474 */,
/* 475 */,
/* 476 */,
/* 477 */,
/* 478 */,
/* 479 */,
/* 480 */,
/* 481 */,
/* 482 */,
/* 483 */,
/* 484 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

	var _classnames = __webpack_require__(257);

	var _classnames2 = _interopRequireDefault(_classnames);

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               limitations under the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               */

	var propTypes = {
	  placement: _react2.default.PropTypes.oneOf(['top', 'right', 'bottom', 'left']),
	  positionTop: _react2.default.PropTypes.oneOfType([_react2.default.PropTypes.number, _react2.default.PropTypes.string]),
	  positionLeft: _react2.default.PropTypes.oneOfType([_react2.default.PropTypes.number, _react2.default.PropTypes.string]),
	  arrowOffsetTop: _react2.default.PropTypes.oneOfType([_react2.default.PropTypes.number, _react2.default.PropTypes.string]),
	  arrowOffsetLeft: _react2.default.PropTypes.oneOfType([_react2.default.PropTypes.number, _react2.default.PropTypes.string]),
	  title: _react2.default.PropTypes.node
	};

	var defaultProps = {
	  placement: 'right'
	};

	var Popover = function (_React$Component) {
	  _inherits(Popover, _React$Component);

	  function Popover() {
	    _classCallCheck(this, Popover);

	    return _possibleConstructorReturn(this, _React$Component.apply(this, arguments));
	  }

	  Popover.prototype.render = function render() {
	    var _classes;

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


	    var classes = (_classes = {
	      'popover': true
	    }, _classes[placement] = true, _classes);

	    var outerStyle = _extends({
	      display: 'block',
	      top: positionTop,
	      left: positionLeft
	    }, style);

	    var arrowStyle = {
	      top: arrowOffsetTop,
	      left: arrowOffsetLeft
	    };

	    return _react2.default.createElement(
	      'div',
	      {
	        role: 'tooltip',
	        className: (0, _classnames2.default)(className, classes),
	        style: outerStyle },
	      _react2.default.createElement('div', { className: 'arrow', style: arrowStyle }),
	      title && _react2.default.createElement(
	        'h3',
	        { className: 'popover-title' },
	        title
	      ),
	      _react2.default.createElement(
	        'div',
	        { className: 'popover-content' },
	        children
	      )
	    );
	  };

	  return Popover;
	}(_react2.default.Component);

	Popover.propTypes = propTypes;
	Popover.defaultProps = defaultProps;

	exports.default = Popover;
	module.exports = exports['default'];

/***/ }),
/* 485 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _jQuery = __webpack_require__(229);

	var _jQuery2 = _interopRequireDefault(_jQuery);

	var _moment = __webpack_require__(291);

	var _moment2 = _interopRequireDefault(_moment);

	var _lodash = __webpack_require__(275);

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

	var DateRangePicker = _react2.default.createClass({
	  displayName: 'DateRangePicker',
	  getDates: function getDates() {
	    var startDate = (0, _jQuery2.default)(this.refs.dpPicker1).datepicker('getDate');
	    var endDate = (0, _jQuery2.default)(this.refs.dpPicker2).datepicker('getDate');
	    return [startDate, (0, _moment2.default)(endDate).endOf('day').toDate()];
	  },
	  setDates: function setDates(_ref) {
	    var startDate = _ref.startDate,
	        endDate = _ref.endDate;

	    (0, _jQuery2.default)(this.refs.dpPicker1).datepicker('setDate', startDate);
	    (0, _jQuery2.default)(this.refs.dpPicker2).datepicker('setDate', endDate);
	  },
	  getDefaultProps: function getDefaultProps() {
	    return {
	      startDate: (0, _moment2.default)().startOf('month').toDate(),
	      endDate: (0, _moment2.default)().endOf('month').toDate(),
	      onChange: function onChange() {}
	    };
	  },
	  componentWillUnmount: function componentWillUnmount() {
	    (0, _jQuery2.default)(this.refs.dp).datepicker('destroy');
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
	    this.onChange = (0, _lodash.debounce)(this.onChange, 1);
	    (0, _jQuery2.default)(this.refs.rangePicker).datepicker({
	      todayBtn: 'linked',
	      keyboardNavigation: false,
	      forceParse: false,
	      calendarWeeks: true,
	      autoclose: true
	    });

	    this.setDates(this.props);

	    (0, _jQuery2.default)(this.refs.rangePicker).datepicker().on('changeDate', this.onChange);
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
	    return _react2.default.createElement(
	      'div',
	      { className: 'grv-datepicker input-group input-group-sm input-daterange', ref: 'rangePicker' },
	      _react2.default.createElement('input', { ref: 'dpPicker1', type: 'text', className: 'input-sm form-control', name: 'start' }),
	      _react2.default.createElement(
	        'span',
	        { className: 'input-group-addon' },
	        'to'
	      ),
	      _react2.default.createElement('input', { ref: 'dpPicker2', type: 'text', className: 'input-sm form-control', name: 'end' })
	    );
	  }
	});

	function isSame(date1, date2) {
	  return (0, _moment2.default)(date1).isSame(date2, 'day');
	}

	exports.default = DateRangePicker;
	module.exports = exports['default'];

/***/ }),
/* 486 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _actions = __webpack_require__(409);

	var _player = __webpack_require__(487);

	var _partyListPanel = __webpack_require__(411);

	var _partyListPanel2 = _interopRequireDefault(_partyListPanel);

	var _config = __webpack_require__(228);

	var _config2 = _interopRequireDefault(_config);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               limitations under the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               */

	var PlayerHost = function (_React$Component) {
	  _inherits(PlayerHost, _React$Component);

	  function PlayerHost() {
	    _classCallCheck(this, PlayerHost);

	    return _possibleConstructorReturn(this, _React$Component.apply(this, arguments));
	  }

	  PlayerHost.prototype.componentWillMount = function componentWillMount() {
	    var _props$params = this.props.params,
	        sid = _props$params.sid,
	        siteId = _props$params.siteId;

	    this.url = _config2.default.api.getFetchSessionUrl({ siteId: siteId, sid: sid });
	  };

	  PlayerHost.prototype.render = function render() {
	    if (!this.url) {
	      return null;
	    }
	    return _react2.default.createElement(
	      'div',
	      { className: 'grv-terminalhost grv-session-player' },
	      _react2.default.createElement(_partyListPanel2.default, { onClose: _actions.close }),
	      _react2.default.createElement(_player.Player, { url: this.url })
	    );
	  };

	  return PlayerHost;
	}(_react2.default.Component);

	exports.default = PlayerHost;
	module.exports = exports['default'];

/***/ }),
/* 487 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.Player = undefined;

	var _jQuery = __webpack_require__(229);

	var _jQuery2 = _interopRequireDefault(_jQuery);

	var _jquery = __webpack_require__(488);

	var _jquery2 = _interopRequireDefault(_jquery);

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _reactDom = __webpack_require__(31);

	var _reactDom2 = _interopRequireDefault(_reactDom);

	var _reactSlider = __webpack_require__(510);

	var _reactSlider2 = _interopRequireDefault(_reactSlider);

	var _terminal = __webpack_require__(398);

	var _terminal2 = _interopRequireDefault(_terminal);

	var _ttyPlayer = __webpack_require__(511);

	var _indicator = __webpack_require__(395);

	var _indicator2 = _interopRequireDefault(_indicator);

	var _items = __webpack_require__(517);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               limitations under the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               */


	(0, _jquery2.default)(_jQuery2.default);

	var Terminal = function (_GrvTerminal) {
	  _inherits(Terminal, _GrvTerminal);

	  function Terminal(tty, el) {
	    _classCallCheck(this, Terminal);

	    var _this = _possibleConstructorReturn(this, _GrvTerminal.call(this, { el: el, scrollBack: 1000 }));

	    _this.tty = tty;
	    return _this;
	  }

	  Terminal.prototype.connect = function connect() {};

	  Terminal.prototype.open = function open() {
	    _GrvTerminal.prototype.open.call(this);
	    (0, _jQuery2.default)(this._el).perfectScrollbar();
	  };

	  Terminal.prototype.resize = function resize(cols, rows) {
	    // ensure that cursor is visible as xterm hides it on blur event
	    this.term.cursorState = 1;
	    _GrvTerminal.prototype.resize.call(this, cols, rows);
	    (0, _jQuery2.default)(this._el).perfectScrollbar('update');
	  };

	  Terminal.prototype.destroy = function destroy() {
	    _GrvTerminal.prototype.destroy.call(this);
	    (0, _jQuery2.default)(this._el).perfectScrollbar('destroy');
	  };

	  Terminal.prototype._disconnect = function _disconnect() {};

	  Terminal.prototype._requestResize = function _requestResize() {};

	  return Terminal;
	}(_terminal2.default);

	var Content = function (_React$Component) {
	  _inherits(Content, _React$Component);

	  function Content() {
	    _classCallCheck(this, Content);

	    return _possibleConstructorReturn(this, _React$Component.apply(this, arguments));
	  }

	  Content.prototype.componentDidMount = function componentDidMount() {
	    var tty = this.props.tty;
	    this.terminal = new Terminal(tty, this.refs.container);
	    this.terminal.open();
	  };

	  Content.prototype.componentWillUnmount = function componentWillUnmount() {
	    this.terminal.destroy();
	  };

	  Content.prototype.render = function render() {
	    var isLoading = this.props.tty.isLoading;
	    // need to hide the terminal cursor while fetching for events
	    var style = {
	      visibility: isLoading ? "hidden" : "initial"
	    };

	    return _react2.default.createElement('div', { style: style, ref: 'container' });
	  };

	  return Content;
	}(_react2.default.Component);

	Content.propTypes = {
	  tty: _react2.default.PropTypes.object.isRequired
	};

	var ControlPanel = function (_React$Component2) {
	  _inherits(ControlPanel, _React$Component2);

	  function ControlPanel() {
	    _classCallCheck(this, ControlPanel);

	    return _possibleConstructorReturn(this, _React$Component2.apply(this, arguments));
	  }

	  ControlPanel.prototype.componentDidMount = function componentDidMount() {
	    var el = _reactDom2.default.findDOMNode(this);
	    var btn = el.querySelector('.grv-session-player-controls button');
	    btn && btn.focus();
	  };

	  ControlPanel.prototype.render = function render() {
	    var _props = this.props,
	        isPlaying = _props.isPlaying,
	        min = _props.min,
	        max = _props.max,
	        value = _props.value,
	        onChange = _props.onChange,
	        onToggle = _props.onToggle,
	        time = _props.time;

	    var btnClass = isPlaying ? 'fa fa-stop' : 'fa fa-play';
	    return _react2.default.createElement(
	      'div',
	      { className: 'grv-session-player-controls' },
	      _react2.default.createElement(
	        'button',
	        { className: 'btn', onClick: onToggle },
	        _react2.default.createElement('i', { className: btnClass })
	      ),
	      _react2.default.createElement(
	        'div',
	        { className: 'grv-session-player-controls-time' },
	        time
	      ),
	      _react2.default.createElement(
	        'div',
	        { className: 'grv-flex-column' },
	        _react2.default.createElement(_reactSlider2.default, {
	          min: min,
	          max: max,
	          value: value,
	          onChange: onChange,
	          defaultValue: 1,
	          withBars: true,
	          className: 'grv-slider' })
	      )
	    );
	  };

	  return ControlPanel;
	}(_react2.default.Component);

	var Player = exports.Player = function (_React$Component3) {
	  _inherits(Player, _React$Component3);

	  function Player(props) {
	    _classCallCheck(this, Player);

	    var _this4 = _possibleConstructorReturn(this, _React$Component3.call(this, props));

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

	    _this4.tty = new _ttyPlayer.TtyPlayer({ url: url });
	    _this4.state = _this4.calculateState();
	    return _this4;
	  }

	  Player.prototype.calculateState = function calculateState() {
	    return {
	      eventCount: this.tty.getEventCount(),
	      length: this.tty.length,
	      min: 1,
	      time: this.tty.getCurrentTime(),
	      isLoading: this.tty.isLoading,
	      isPlaying: this.tty.isPlaying,
	      isError: this.tty.isError,
	      errText: this.tty.errText,
	      current: this.tty.current,
	      canPlay: this.tty.length > 1
	    };
	  };

	  Player.prototype.componentDidMount = function componentDidMount() {
	    this.tty.on('change', this.updateState);
	    this.tty.connect();
	    this.tty.play();
	  };

	  Player.prototype.componentWillUnmount = function componentWillUnmount() {
	    this.tty.stop();
	    this.tty.removeAllListeners();
	  };

	  Player.prototype.render = function render() {
	    var _state = this.state,
	        isPlaying = _state.isPlaying,
	        isLoading = _state.isLoading,
	        isError = _state.isError,
	        errText = _state.errText,
	        time = _state.time,
	        min = _state.min,
	        length = _state.length,
	        current = _state.current,
	        eventCount = _state.eventCount;


	    if (isError) {
	      return _react2.default.createElement(_items.ErrorIndicator, { text: errText });
	    }

	    if (!isLoading && eventCount === 0) {
	      return _react2.default.createElement(_items.WarningIndicator, { text: 'The recording for this session is not available.' });
	    }

	    return _react2.default.createElement(
	      'div',
	      { className: 'grv-session-player-content' },
	      _react2.default.createElement(Content, { tty: this.tty }),
	      isLoading && _react2.default.createElement(_indicator2.default, null),
	      eventCount > 0 && _react2.default.createElement(ControlPanel, {
	        isPlaying: isPlaying,
	        time: time,
	        min: min,
	        max: length,
	        value: current,
	        onToggle: this.onTogglePlayStop,
	        onChange: this.onMove })
	    );
	  };

	  return Player;
	}(_react2.default.Component);

/***/ }),
/* 488 */,
/* 489 */,
/* 490 */,
/* 491 */,
/* 492 */,
/* 493 */,
/* 494 */,
/* 495 */,
/* 496 */,
/* 497 */,
/* 498 */,
/* 499 */,
/* 500 */,
/* 501 */,
/* 502 */,
/* 503 */,
/* 504 */,
/* 505 */,
/* 506 */,
/* 507 */,
/* 508 */,
/* 509 */,
/* 510 */,
/* 511 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.Buffer = exports.TtyPlayer = exports.EventProvider = exports.MAX_SIZE = undefined;

	var _jQuery = __webpack_require__(229);

	var _jQuery2 = _interopRequireDefault(_jQuery);

	var _buffer = __webpack_require__(512);

	var _buffer2 = _interopRequireDefault(_buffer);

	var _api = __webpack_require__(241);

	var _api2 = _interopRequireDefault(_api);

	var _tty = __webpack_require__(400);

	var _tty2 = _interopRequireDefault(_tty);

	var _enums = __webpack_require__(397);

	var _logger = __webpack_require__(245);

	var _logger2 = _interopRequireDefault(_logger);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } } /*
	                                                                                                                                                          Copyright 2015 Gravitational, Inc.
	                                                                                                                                                          
	                                                                                                                                                          Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                          you may not use this file except in compliance with the License.
	                                                                                                                                                          You may obtain a copy of the License at
	                                                                                                                                                          
	                                                                                                                                                              http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                          
	                                                                                                                                                          Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                          distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                          WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                          See the License for the specific language governing permissions and
	                                                                                                                                                          limitations under the License.
	                                                                                                                                                          */

	var logger = _logger2.default.create('TtyPlayer');
	var STREAM_START_INDEX = 0;
	var URL_PREFIX_EVENTS = '/events';
	var PLAY_SPEED = 5;
	var Buffer = _buffer2.default.Buffer;

	var MAX_SIZE = exports.MAX_SIZE = 5242880; // 5mg

	var EventProvider = exports.EventProvider = function () {
	  function EventProvider(_ref) {
	    var url = _ref.url;

	    _classCallCheck(this, EventProvider);

	    this.url = url;
	    this.events = [];
	  }

	  EventProvider.prototype.getLengthInTime = function getLengthInTime() {
	    var length = this.events.length;
	    if (length === 0) {
	      return 0;
	    }

	    return this.events[length - 1].msNormalized;
	  };

	  EventProvider.prototype.init = function init() {
	    var _this = this;

	    var url = this.url + URL_PREFIX_EVENTS;
	    return _api2.default.get(url).then(function (json) {
	      if (!json.events) {
	        return;
	      }

	      var events = _this._createPrintEvents(json.events);
	      if (events.length === 0) {
	        return;
	      }

	      _this.events = _this._normalizeEventsByTime(events);
	      return _this._fetchBytes();
	    });
	  };

	  EventProvider.prototype._fetchBytes = function _fetchBytes() {
	    var _this2 = this;

	    // need to calclulate the size of the session in bytes to know how many 
	    // chunks to load due to maximum chunk size.
	    var offset = this.events[0].offset;
	    var end = this.events.length - 1;
	    var totalSize = this.events[end].offset - offset + this.events[end].bytes;
	    var chunkCount = Math.ceil(totalSize / MAX_SIZE);
	    var promises = [];
	    for (var i = 0; i < chunkCount; i++) {
	      var url = this.url + '/stream?offset=' + offset + '&bytes=' + MAX_SIZE;
	      promises.push(_api2.default.ajax({
	        url: url,
	        processData: true,
	        dataType: 'text'
	      }));

	      offset = offset + MAX_SIZE;
	    }

	    // wait for all chunks to load and then merge all in one
	    return _jQuery2.default.when.apply(_jQuery2.default, promises).then(function () {
	      for (var _len = arguments.length, responses = Array(_len), _key = 0; _key < _len; _key++) {
	        responses[_key] = arguments[_key];
	      }

	      responses = promises.length === 1 ? [[responses]] : responses;
	      var allBytes = responses.reduce(function (byteStr, r) {
	        return byteStr + r[0];
	      }, '');
	      return new Buffer(allBytes);
	    }).then(function (buffer) {
	      return _this2._processByteStream(buffer);
	    });
	  };

	  EventProvider.prototype._processByteStream = function _processByteStream(buffer) {
	    var byteStrOffset = this.events[0].bytes;
	    this.events[0].data = buffer.slice(0, byteStrOffset).toString('utf8');
	    for (var i = 1; i < this.events.length; i++) {
	      var bytes = this.events[i].bytes;

	      this.events[i].data = buffer.slice(byteStrOffset, byteStrOffset + bytes).toString('utf8');
	      byteStrOffset += bytes;
	    }
	  };

	  EventProvider.prototype._createPrintEvents = function _createPrintEvents(json) {
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

	      if (event === _enums.EventTypeEnum.RESIZE || event === _enums.EventTypeEnum.START) {
	        var _json$i$size$split = json[i].size.split(':');

	        w = _json$i$size$split[0];
	        h = _json$i$size$split[1];
	      }

	      // session has ended, stop here
	      if (event === _enums.EventTypeEnum.END) {
	        break;
	      }

	      // process only PRINT events      
	      if (event !== _enums.EventTypeEnum.PRINT) {
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

	    return events;
	  };

	  EventProvider.prototype._normalizeEventsByTime = function _normalizeEventsByTime(events) {
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

	    return tmp;
	  };

	  EventProvider.prototype._formatDisplayTime = function _formatDisplayTime(ms) {
	    if (ms < 0) {
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
	  };

	  return EventProvider;
	}();

	var TtyPlayer = exports.TtyPlayer = function (_Tty) {
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
	    _this3.errText = '';

	    _this3._posToEventIndexMap = [];
	    _this3._eventProvider = new EventProvider({ url: url });
	    return _this3;
	  }

	  // override


	  TtyPlayer.prototype.send = function send() {};

	  // override


	  TtyPlayer.prototype.resize = function resize() {};

	  // override


	  TtyPlayer.prototype.connect = function connect() {
	    var _this4 = this;

	    this._setStatusFlag({ isLoading: true });
	    this._eventProvider.init().then(function () {
	      _this4._init();
	      _this4._setStatusFlag({ isReady: true });
	    }).fail(function (err) {
	      logger.error('unable to init event provider', err);
	      _this4.handleError(err);
	    }).always(this._change.bind(this));

	    this._change();
	  };

	  TtyPlayer.prototype.handleError = function handleError(err) {
	    this._setStatusFlag({
	      isError: true,
	      errText: _api2.default.getErrorText(err)
	    });
	  };

	  TtyPlayer.prototype._init = function _init() {
	    var _this5 = this;

	    this.length = this._eventProvider.getLengthInTime();
	    this._eventProvider.events.forEach(function (item) {
	      return _this5._posToEventIndexMap.push(item.msNormalized);
	    });
	  };

	  TtyPlayer.prototype.move = function move(newPos) {
	    if (!this.isReady) {
	      return;
	    }

	    if (newPos === undefined) {
	      newPos = this.current + 1;
	    }

	    if (newPos < 0) {
	      newPos = 0;
	    }

	    if (newPos > this.length) {
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
	        this.emit('reset');
	      }

	      var from = isRewind ? 0 : this.currentEventIndex;
	      var to = newEventIndex;
	      var events = this._eventProvider.events.slice(from, to);

	      this._display(events);
	      this.currentEventIndex = newEventIndex;
	      this.current = newPos;
	      this._change();
	    } catch (err) {
	      logger.error('move', err);
	      this.handleError(err);
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
	      return '--:--';
	    }
	  };

	  TtyPlayer.prototype.getEventCount = function getEventCount() {
	    return this._eventProvider.events.length;
	  };

	  TtyPlayer.prototype._display = function _display(events) {
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
	        isLoading = _newStatus$isLoading === undefined ? false : _newStatus$isLoading,
	        _newStatus$errText = newStatus.errText,
	        errText = _newStatus$errText === undefined ? '' : _newStatus$errText;

	    this.isReady = isReady;
	    this.isError = isError;
	    this.isLoading = isLoading;
	    this.errText = errText;
	  };

	  TtyPlayer.prototype._getEventIndex = function _getEventIndex(num) {
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
	  };

	  TtyPlayer.prototype._change = function _change() {
	    this.emit('change');
	  };

	  return TtyPlayer;
	}(_tty2.default);

	exports.default = TtyPlayer;
	exports.Buffer = Buffer;

/***/ }),
/* 512 */,
/* 513 */,
/* 514 */,
/* 515 */,
/* 516 */,
/* 517 */
/***/ (function(module, exports, __webpack_require__) {

	"use strict";

	exports.__esModule = true;
	exports.WarningIndicator = exports.ErrorIndicator = undefined;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	var ErrorIndicator = exports.ErrorIndicator = function ErrorIndicator(_ref) {
	  var text = _ref.text;
	  return _react2.default.createElement(
	    "div",
	    { className: "grv-terminalhost-indicator-error" },
	    _react2.default.createElement("i", { className: "fa fa-exclamation-triangle fa-3x text-warning" }),
	    _react2.default.createElement(
	      "div",
	      { className: "m-l" },
	      _react2.default.createElement(
	        "strong",
	        null,
	        text || "Error"
	      )
	    )
	  );
	}; /*
	   Copyright 2015 Gravitational, Inc.
	   
	   Licensed under the Apache License, Version 2.0 (the "License");
	   you may not use this file except in compliance with the License.
	   You may obtain a copy of the License at
	   
	       http://www.apache.org/licenses/LICENSE-2.0
	   
	   Unless required by applicable law or agreed to in writing, software
	   distributed under the License is distributed on an "AS IS" BASIS,
	   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	   See the License for the specific language governing permissions and
	   limitations under the License.
	   */

	var WarningIndicator = exports.WarningIndicator = function WarningIndicator(_ref2) {
	  var text = _ref2.text;
	  return _react2.default.createElement(
	    "div",
	    { className: "grv-terminalhost-indicator-error" },
	    _react2.default.createElement(
	      "h3",
	      null,
	      text
	    )
	  );
	};

/***/ }),
/* 518 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.SettingsFeatureBase = undefined;

	var _featureBase = __webpack_require__(393);

	var _featureBase2 = _interopRequireDefault(_featureBase);

	var _featureActivator = __webpack_require__(519);

	var _featureActivator2 = _interopRequireDefault(_featureActivator);

	var _actions = __webpack_require__(284);

	var _config = __webpack_require__(228);

	var _config2 = _interopRequireDefault(_config);

	var _main = __webpack_require__(520);

	var _main2 = _interopRequireDefault(_main);

	var _actions2 = __webpack_require__(522);

	var _settings = __webpack_require__(524);

	var _settings2 = _interopRequireDefault(_settings);

	var _constants = __webpack_require__(248);

	var API = _interopRequireWildcard(_constants);

	function _interopRequireWildcard(obj) { if (obj && obj.__esModule) { return obj; } else { var newObj = {}; if (obj != null) { for (var key in obj) { if (Object.prototype.hasOwnProperty.call(obj, key)) newObj[key] = obj[key]; } } newObj.default = obj; return newObj; } }

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
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
	  to: _config2.default.routes.settingsBase,
	  title: 'Settings'

	  /**
	   * Describes nested features within Settings
	   */
	};
	var SettingsFeatureBase = exports.SettingsFeatureBase = function (_FeatureBase) {
	  _inherits(SettingsFeatureBase, _FeatureBase);

	  function SettingsFeatureBase(props) {
	    _classCallCheck(this, SettingsFeatureBase);

	    return _possibleConstructorReturn(this, _FeatureBase.call(this, props));
	  }

	  SettingsFeatureBase.prototype.isEnabled = function isEnabled() {
	    return true;
	  };

	  return SettingsFeatureBase;
	}(_featureBase2.default);

	var SettingsFeature = function (_FeatureBase2) {
	  _inherits(SettingsFeature, _FeatureBase2);

	  SettingsFeature.prototype.addChild = function addChild(feature) {
	    if (!(feature instanceof SettingsFeatureBase)) {
	      throw Error('feature must implement SettingsFeatureBase');
	    }

	    this.featureActivator.register(feature);
	  };

	  function SettingsFeature(routes) {
	    _classCallCheck(this, SettingsFeature);

	    var _this2 = _possibleConstructorReturn(this, _FeatureBase2.call(this, API.TRYING_TO_INIT_SETTINGS));

	    _this2.featureActivator = new _featureActivator2.default();
	    _this2.childRoutes = [];

	    var settingsRoutes = {
	      path: _config2.default.routes.settingsBase,
	      title: 'Settings',
	      component: _FeatureBase2.prototype.withMe.call(_this2, _main2.default),
	      indexRoute: {
	        // need index component to handle default redirect to available nested feature
	        component: _settings2.default
	      },
	      childRoutes: _this2.childRoutes
	    };

	    routes.push(settingsRoutes);
	    return _this2;
	  }

	  SettingsFeature.prototype.componentDidMount = function componentDidMount() {
	    try {
	      (0, _actions2.initSettings)(this.featureActivator);
	    } catch (err) {
	      this.handleError(err);
	    }
	  };

	  SettingsFeature.prototype.onload = function onload() {
	    var features = this.featureActivator.getFeatures();
	    var some = features.some(function (f) {
	      return f.isEnabled();
	    });
	    if (some) {
	      (0, _actions.addNavItem)(settingsNavItem);
	    }
	  };

	  return SettingsFeature;
	}(_featureBase2.default);

	exports.default = SettingsFeature;

/***/ }),
/* 519 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _jQuery = __webpack_require__(229);

	var _jQuery2 = _interopRequireDefault(_jQuery);

	var _logger = __webpack_require__(245);

	var _logger2 = _interopRequireDefault(_logger);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } } /*
	                                                                                                                                                          Copyright 2015 Gravitational, Inc.
	                                                                                                                                                          
	                                                                                                                                                          Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                          you may not use this file except in compliance with the License.
	                                                                                                                                                          You may obtain a copy of the License at
	                                                                                                                                                          
	                                                                                                                                                              http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                          
	                                                                                                                                                          Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                          distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                          WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                          See the License for the specific language governing permissions and
	                                                                                                                                                          limitations under the License.
	                                                                                                                                                          */

	var logger = _logger2.default.create('featureActivator');

	/**
	 * Invokes methods on a group of registered features. 
	 * 
	 */

	var FeactureActivator = function () {
	  function FeactureActivator() {
	    _classCallCheck(this, FeactureActivator);

	    this._features = [];
	  }

	  FeactureActivator.prototype.register = function register(feature) {
	    if (!feature) {
	      throw Error('Feature is undefined');
	    }

	    this._features.push(feature);
	  };

	  /**
	   * to be called during app initialization. Becomes useful if feature wants to be
	   * part of app initialization flow. 
	   */


	  FeactureActivator.prototype.preload = function preload(context) {
	    var promises = this._features.map(function (f) {
	      var featurePromise = _jQuery2.default.Deferred();
	      // feature should handle failed promises thus always resolve.
	      f.init(context).always(function () {
	        featurePromise.resolve();
	      });

	      return featurePromise;
	    });

	    return _jQuery2.default.when.apply(_jQuery2.default, promises);
	  };

	  FeactureActivator.prototype.onload = function onload(context) {
	    var _this = this;

	    this._features.forEach(function (f) {
	      _this._invokeOnload(f, context);
	    });
	  };

	  FeactureActivator.prototype.getFirstAvailable = function getFirstAvailable() {
	    return this._features.find(function (f) {
	      return !f.isFailed();
	    });
	  };

	  FeactureActivator.prototype.getFeatures = function getFeatures() {
	    return this._features;
	  };

	  FeactureActivator.prototype._invokeOnload = function _invokeOnload(f) {
	    try {
	      for (var _len = arguments.length, props = Array(_len > 1 ? _len - 1 : 0), _key = 1; _key < _len; _key++) {
	        props[_key - 1] = arguments[_key];
	      }

	      f.onload.apply(f, props);
	    } catch (err) {
	      logger.error('failed to invoke feature onload()', err);
	    }
	  };

	  return FeactureActivator;
	}();

	exports.default = FeactureActivator;
	module.exports = exports['default'];

/***/ }),
/* 520 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _nuclearJsReactAddons = __webpack_require__(219);

	var _reactRouter = __webpack_require__(164);

	var _getters = __webpack_require__(521);

	var _getters2 = _interopRequireDefault(_getters);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               limitations under the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               */

	var Separator = function Separator() {
	  return _react2.default.createElement('div', { className: 'grv-settings-header-line-solid m-t-sm m-b-sm' });
	};

	var Settings = function (_React$Component) {
	  _inherits(Settings, _React$Component);

	  function Settings() {
	    _classCallCheck(this, Settings);

	    return _possibleConstructorReturn(this, _React$Component.apply(this, arguments));
	  }

	  Settings.prototype.renderHeaderItem = function renderHeaderItem(item, key) {
	    var to = item.to,
	        isIndex = item.isIndex,
	        title = item.title;

	    var className = this.context.router.isActive(to, isIndex) ? "active" : "";
	    return _react2.default.createElement(
	      'li',
	      { key: key, className: className },
	      _react2.default.createElement(
	        _reactRouter.Link,
	        { to: to },
	        _react2.default.createElement(
	          'h2',
	          { className: 'm-b-xxs' },
	          title
	        )
	      ),
	      _react2.default.createElement(Separator, null)
	    );
	  };

	  Settings.prototype.render = function render() {
	    var store = this.props.store;

	    var $headerItems = store.getNavItems().map(this.renderHeaderItem.bind(this));

	    if (!store.isReady()) {
	      return null;
	    }

	    return _react2.default.createElement(
	      'div',
	      { className: 'grv-page grv-settings' },
	      _react2.default.createElement(
	        'ul',
	        { className: 'grv-settings-header-menu' },
	        $headerItems
	      ),
	      $headerItems.length > 0 && _react2.default.createElement(Separator, null),
	      this.props.children
	    );
	  };

	  return Settings;
	}(_react2.default.Component);

	Settings.contextTypes = {
	  router: _react.PropTypes.object.isRequired
	};


	function mapStateToProps() {
	  return {
	    store: _getters2.default.store
	  };
	}

	exports.default = (0, _nuclearJsReactAddons.connect)(mapStateToProps)(Settings);
	module.exports = exports['default'];

/***/ }),
/* 521 */
/***/ (function(module, exports) {

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

	exports.default = {
	  store: ['tlpt_settings']
	};
	module.exports = exports['default'];

/***/ }),
/* 522 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.addNavItem = addNavItem;
	exports.initSettings = initSettings;

	var _reactor = __webpack_require__(233);

	var _reactor2 = _interopRequireDefault(_reactor);

	var _getters = __webpack_require__(521);

	var _getters2 = _interopRequireDefault(_getters);

	var _actionTypes = __webpack_require__(523);

	var AT = _interopRequireWildcard(_actionTypes);

	var _actions = __webpack_require__(246);

	function _interopRequireWildcard(obj) { if (obj && obj.__esModule) { return obj; } else { var newObj = {}; if (obj != null) { for (var key in obj) { if (Object.prototype.hasOwnProperty.call(obj, key)) newObj[key] = obj[key]; } } newObj.default = obj; return newObj; } }

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

	function addNavItem(navItem) {
	  _reactor2.default.dispatch(AT.ADD_NAV_ITEM, navItem);
	}

	function initSettings(featureActivator) {
	  // init only once
	  var store = _reactor2.default.evaluate(_getters2.default.store);
	  if (store.isReady()) {
	    return;
	  }

	  featureActivator.onload();
	  _reactor2.default.dispatch(AT.INIT, {});
	  _actions.initSettingsStatus.success();
	}

/***/ }),
/* 523 */
/***/ (function(module, exports) {

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

	var INIT = exports.INIT = 'SETTINGS_INIT';
	var ADD_NAV_ITEM = exports.ADD_NAV_ITEM = 'SETTINGS_ADD_NAV_ITEM';
	var SET_RES_TO_DELETE = exports.SET_RES_TO_DELETE = 'SETTINGS_SET_RES_TO_DELETE';

/***/ }),
/* 524 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _connect = __webpack_require__(422);

	var _connect2 = _interopRequireDefault(_connect);

	var _msgPage = __webpack_require__(266);

	var Messages = _interopRequireWildcard(_msgPage);

	var _getters = __webpack_require__(521);

	var _getters2 = _interopRequireDefault(_getters);

	function _interopRequireWildcard(obj) { if (obj && obj.__esModule) { return obj; } else { var newObj = {}; if (obj != null) { for (var key in obj) { if (Object.prototype.hasOwnProperty.call(obj, key)) newObj[key] = obj[key]; } } newObj.default = obj; return newObj; } }

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               limitations under the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               */

	var SettingsIndex = function (_React$Component) {
	  _inherits(SettingsIndex, _React$Component);

	  function SettingsIndex() {
	    _classCallCheck(this, SettingsIndex);

	    return _possibleConstructorReturn(this, _React$Component.apply(this, arguments));
	  }

	  SettingsIndex.prototype.componentDidMount = function componentDidMount() {
	    var route = this.getAvailableRoute();
	    if (route) {
	      this.props.router.replace({ pathname: route });
	    }
	  };

	  SettingsIndex.prototype.getAvailableRoute = function getAvailableRoute() {
	    var items = this.props.store.getNavItems();
	    if (items && items[0]) {
	      return items[0].to;
	    }

	    return null;
	  };

	  SettingsIndex.prototype.render = function render() {
	    return _react2.default.createElement(Messages.AccessDenied, null);
	  };

	  return SettingsIndex;
	}(_react2.default.Component);

	SettingsIndex.propTypes = {
	  router: _react2.default.PropTypes.object.isRequired,
	  store: _react2.default.PropTypes.object.isRequired,
	  location: _react2.default.PropTypes.object.isRequired
	};


	function mapStateToProps() {
	  return {
	    store: _getters2.default.store
	  };
	}

	exports.default = (0, _connect2.default)(mapStateToProps)(SettingsIndex);
	module.exports = exports['default'];

/***/ }),
/* 525 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.createSettings = exports.append = undefined;

	var _featureSettingsAccount = __webpack_require__(526);

	var _featureSettingsAccount2 = _interopRequireDefault(_featureSettingsAccount);

	var _featureSettings = __webpack_require__(518);

	var _featureSettings2 = _interopRequireDefault(_featureSettings);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	/**
	 * Adds nested feature to given Settings feature
	 * @param {*instance of Settings feature} settings 
	 */
	/*
	Copyright 2015 Gravitational, Inc.

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

	    http://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/

	var append = exports.append = function append(settings, fctor) {
	  var f = new fctor(settings.childRoutes);
	  settings.addChild(f);
	};

	var createSettings = exports.createSettings = function createSettings(routes) {
	  var settings = new _featureSettings2.default(routes);
	  append(settings, _featureSettingsAccount2.default);
	  return settings;
	};

/***/ }),
/* 526 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _flags = __webpack_require__(527);

	var featureFlags = _interopRequireWildcard(_flags);

	var _featureSettings = __webpack_require__(518);

	var _config = __webpack_require__(228);

	var _config2 = _interopRequireDefault(_config);

	var _actions = __webpack_require__(522);

	var _accountTab = __webpack_require__(528);

	var _accountTab2 = _interopRequireDefault(_accountTab);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _interopRequireWildcard(obj) { if (obj && obj.__esModule) { return obj; } else { var newObj = {}; if (obj != null) { for (var key in obj) { if (Object.prototype.hasOwnProperty.call(obj, key)) newObj[key] = obj[key]; } } newObj.default = obj; return newObj; } }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               limitations under the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               */

	var featureUrl = _config2.default.routes.settingsAccount;

	var AccountFeature = function (_SettingsFeatureBase) {
	  _inherits(AccountFeature, _SettingsFeatureBase);

	  function AccountFeature(routes) {
	    _classCallCheck(this, AccountFeature);

	    var _this = _possibleConstructorReturn(this, _SettingsFeatureBase.call(this));

	    var route = {
	      title: 'Account',
	      path: featureUrl,
	      component: _this.withMe(_accountTab2.default)
	    };

	    routes.push(route);
	    return _this;
	  }

	  AccountFeature.prototype.isEnabled = function isEnabled() {
	    return featureFlags.isAccountEnabled();
	  };

	  AccountFeature.prototype.init = function init() {
	    if (!this.wasInitialized()) {
	      this.stopProcessing();
	    }
	  };

	  AccountFeature.prototype.onload = function onload() {
	    if (!this.isEnabled()) {
	      return;
	    }

	    var navItem = {
	      to: featureUrl,
	      title: "Account"
	    };

	    (0, _actions.addNavItem)(navItem);
	    this.init();
	  };

	  return AccountFeature;
	}(_featureSettings.SettingsFeatureBase);

	exports.default = AccountFeature;
	module.exports = exports['default'];

/***/ }),
/* 527 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.isAccountEnabled = isAccountEnabled;

	var _user = __webpack_require__(250);

	function isAccountEnabled() {
	  return (0, _user.getUser)().isSso() == false;
	} /*
	  Copyright 2015 Gravitational, Inc.
	  
	  Licensed under the Apache License, Version 2.0 (the "License");
	  you may not use this file except in compliance with the License.
	  You may obtain a copy of the License at
	  
	      http://www.apache.org/licenses/LICENSE-2.0
	  
	  Unless required by applicable law or agreed to in writing, software
	  distributed under the License is distributed on an "AS IS" BASIS,
	  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	  See the License for the specific language governing permissions and
	  limitations under the License.
	  */

/***/ }),
/* 528 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

	var _jQuery = __webpack_require__(229);

	var _jQuery2 = _interopRequireDefault(_jQuery);

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _connect = __webpack_require__(422);

	var _connect2 = _interopRequireDefault(_connect);

	var _config = __webpack_require__(228);

	var _config2 = _interopRequireDefault(_config);

	var _enums = __webpack_require__(264);

	var _alerts = __webpack_require__(529);

	var Alerts = _interopRequireWildcard(_alerts);

	var _user = __webpack_require__(250);

	var _actions = __webpack_require__(239);

	var _layout = __webpack_require__(430);

	var _layout2 = _interopRequireDefault(_layout);

	function _interopRequireWildcard(obj) { if (obj && obj.__esModule) { return obj; } else { var newObj = {}; if (obj != null) { for (var key in obj) { if (Object.prototype.hasOwnProperty.call(obj, key)) newObj[key] = obj[key]; } } newObj.default = obj; return newObj; } }

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               limitations under the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               */

	var Separator = function Separator() {
	  return _react2.default.createElement('div', { className: 'grv-settings-header-line-solid m-t-sm m-b-sm' });
	};

	var Label = function Label(_ref) {
	  var text = _ref.text;
	  return _react2.default.createElement(
	    'label',
	    { style: { width: "150px", fontWeight: "normal" }, className: ' m-t-xs' },
	    ' ',
	    text,
	    ' '
	  );
	};

	var defaultState = {
	  oldPass: '',
	  newPass: '',
	  newPassConfirmed: '',
	  token: ''
	};

	var AccountTab = function (_React$Component) {
	  _inherits(AccountTab, _React$Component);

	  function AccountTab() {
	    var _temp, _this, _ret;

	    _classCallCheck(this, AccountTab);

	    for (var _len = arguments.length, args = Array(_len), _key = 0; _key < _len; _key++) {
	      args[_key] = arguments[_key];
	    }

	    return _ret = (_temp = (_this = _possibleConstructorReturn(this, _React$Component.call.apply(_React$Component, [this].concat(args))), _this), _this.hasBeenClicked = false, _this.state = _extends({}, defaultState), _this.onClick = function (e) {
	      e.preventDefault();
	      if (_this.isValid()) {
	        var _this$state = _this.state,
	            oldPass = _this$state.oldPass,
	            newPass = _this$state.newPass,
	            token = _this$state.token;

	        _this.hasBeenClicked = true;
	        if (_this.props.auth2faType === _enums.Auth2faTypeEnum.UTF) {
	          _this.props.onChangePassWithU2f(oldPass, newPass);
	        } else {
	          _this.props.onChangePass(oldPass, newPass, token);
	        }
	      }
	    }, _temp), _possibleConstructorReturn(_this, _ret);
	  }

	  AccountTab.prototype.componentDidMount = function componentDidMount() {
	    (0, _jQuery2.default)(this.refs.form).validate({
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
	          minlength: _jQuery2.default.validator.format('Enter at least {0} characters'),
	          equalTo: 'Enter the same password as above'
	        }
	      }
	    });
	  };

	  AccountTab.prototype.componentWillUnmount = function componentWillUnmount() {
	    this.props.onDestory && this.props.onDestory();
	  };

	  AccountTab.prototype.isValid = function isValid() {
	    var $form = (0, _jQuery2.default)(this.refs.form);
	    return $form.length === 0 || $form.valid();
	  };

	  AccountTab.prototype.componentWillReceiveProps = function componentWillReceiveProps(nextProps) {
	    var isSuccess = nextProps.attempt.isSuccess;

	    if (isSuccess && this.hasBeenClicked) {
	      // reset all input fields on success
	      this.hasBeenClicked = false;
	      this.setState(defaultState);
	    }
	  };

	  AccountTab.prototype.isU2f = function isU2f() {
	    return this.props.auth2faType === _enums.Auth2faTypeEnum.UTF;
	  };

	  AccountTab.prototype.isOtp = function isOtp() {
	    return this.props.auth2faType === _enums.Auth2faTypeEnum.OTP;
	  };

	  AccountTab.prototype.render = function render() {
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

	    return _react2.default.createElement(
	      'div',
	      { title: 'Change Password', className: 'm-t-sm grv-settings-account' },
	      _react2.default.createElement(
	        'h3',
	        { className: 'no-margins' },
	        'Change Password'
	      ),
	      _react2.default.createElement(Separator, null),
	      _react2.default.createElement(
	        'div',
	        { className: 'm-b m-l-xl', style: { maxWidth: "500px" } },
	        _react2.default.createElement(
	          'form',
	          { ref: 'form' },
	          _react2.default.createElement(
	            'div',
	            null,
	            isFailed && _react2.default.createElement(
	              Alerts.Danger,
	              { className: 'm-b-sm' },
	              ' ',
	              message,
	              ' '
	            ),
	            isSuccess && _react2.default.createElement(
	              Alerts.Success,
	              { className: 'm-b-sm' },
	              ' Your password has been changed '
	            ),
	            waitForU2fKeyResponse && _react2.default.createElement(
	              Alerts.Info,
	              { className: 'm-b-sm' },
	              ' Insert your U2F key and press the button on the key '
	            )
	          ),
	          _react2.default.createElement(
	            _layout2.default.Flex,
	            { dir: 'row', className: 'm-t' },
	            _react2.default.createElement(Label, { text: 'Current Password:' }),
	            _react2.default.createElement(
	              'div',
	              { style: { flex: "1" } },
	              _react2.default.createElement('input', {
	                autoFocus: true,
	                type: 'password',
	                value: oldPass,
	                onChange: function onChange(e) {
	                  return _this2.setState({
	                    oldPass: e.target.value
	                  });
	                },
	                className: 'form-control required',
	                placeholder: '' })
	            )
	          ),
	          isOtpEnabled && _react2.default.createElement(
	            _layout2.default.Flex,
	            { dir: 'row', className: 'm-t-sm' },
	            _react2.default.createElement(Label, { text: '2nd factor token:' }),
	            _react2.default.createElement(
	              'div',
	              { style: { flex: "1" } },
	              _react2.default.createElement('input', { autoComplete: 'off',
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
	          _react2.default.createElement(
	            _layout2.default.Flex,
	            { dir: 'row', className: 'm-t-lg' },
	            _react2.default.createElement(Label, { text: 'New Password:' }),
	            _react2.default.createElement(
	              'div',
	              { style: { flex: "1" } },
	              _react2.default.createElement('input', {
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
	          _react2.default.createElement(
	            _layout2.default.Flex,
	            { dir: 'row', className: 'm-t-sm' },
	            _react2.default.createElement(Label, { text: 'Confirm Password:' }),
	            _react2.default.createElement(
	              'div',
	              { style: { flex: "1" } },
	              _react2.default.createElement('input', {
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
	      _react2.default.createElement(
	        'button',
	        { disabled: isProcessing, onClick: this.onClick, type: 'submit', className: 'btn btn-sm btn-primary block' },
	        'Update'
	      )
	    );
	  };

	  return AccountTab;
	}(_react2.default.Component);

	AccountTab.propTypes = {
	  attempt: _react2.default.PropTypes.object.isRequired,
	  onChangePass: _react2.default.PropTypes.func.isRequired,
	  onChangePassWithU2f: _react2.default.PropTypes.func.isRequired
	};


	function mapFluxToProps() {
	  return {
	    attempt: _user.getters.pswChangeAttempt
	  };
	}

	function mapStateToProps() {
	  return {
	    auth2faType: _config2.default.getAuth2faType(),
	    onChangePass: _actions.changePassword,
	    onChangePassWithU2f: _actions.changePasswordWithU2f,
	    onDestory: _actions.resetPasswordChangeAttempt
	  };
	}

	exports.default = (0, _connect2.default)(mapFluxToProps, mapStateToProps)(AccountTab);
	module.exports = exports['default'];

/***/ }),
/* 529 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.Success = exports.Info = exports.Danger = undefined;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _classnames = __webpack_require__(257);

	var _classnames2 = _interopRequireDefault(_classnames);

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

	var Danger = exports.Danger = function Danger(props) {
	  return _react2.default.createElement(
	    'div',
	    { className: (0, _classnames2.default)("grv-alert grv-alert-danger", props.className) },
	    props.children
	  );
	};

	var Info = exports.Info = function Info(props) {
	  return _react2.default.createElement(
	    'div',
	    { className: (0, _classnames2.default)("grv-alert grv-alert-info", props.className) },
	    props.children
	  );
	};

	var Success = exports.Success = function Success(props) {
	  return _react2.default.createElement(
	    'div',
	    { className: (0, _classnames2.default)("grv-alert grv-alert-success", props.className) },
	    ' ',
	    _react2.default.createElement('i', { className: 'fa fa-check m-r-xs' }),
	    ' ',
	    props.children
	  );
	};

/***/ }),
/* 530 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _classnames = __webpack_require__(257);

	var _classnames2 = _interopRequireDefault(_classnames);

	var _nuclearJsReactAddons = __webpack_require__(219);

	var _getters = __webpack_require__(273);

	var _getters2 = _interopRequireDefault(_getters);

	var _browser = __webpack_require__(531);

	var _actions = __webpack_require__(284);

	var _navLeftBar = __webpack_require__(532);

	var _navLeftBar2 = _interopRequireDefault(_navLeftBar);

	var _dataProvider = __webpack_require__(426);

	var _dataProvider2 = _interopRequireDefault(_dataProvider);

	var _msgPage = __webpack_require__(266);

	var _indicator = __webpack_require__(395);

	var _indicator2 = _interopRequireDefault(_indicator);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               limitations under the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               */

	var App = function (_Component) {
	  _inherits(App, _Component);

	  function App() {
	    _classCallCheck(this, App);

	    return _possibleConstructorReturn(this, _Component.apply(this, arguments));
	  }

	  App.prototype.render = function render() {
	    var _props = this.props,
	        router = _props.router,
	        initAttempt = _props.initAttempt;
	    var isProcessing = initAttempt.isProcessing,
	        isSuccess = initAttempt.isSuccess,
	        isFailed = initAttempt.isFailed,
	        message = initAttempt.message;


	    if (isProcessing) {
	      return _react2.default.createElement(_indicator2.default, { type: 'bounce' });
	    }

	    if (isFailed) {
	      return _react2.default.createElement(_msgPage.Failed, { message: message });
	    }

	    var className = (0, _classnames2.default)('grv-tlpt grv-flex grv-flex-row', {
	      '--isLinux': _browser.platform.isLinux,
	      '--isWin': _browser.platform.isWin,
	      '--isMac': _browser.platform.isMac
	    });

	    if (isSuccess) {
	      return _react2.default.createElement(
	        'div',
	        { className: className },
	        _react2.default.createElement(_dataProvider2.default, { onFetch: _actions.refresh, time: 4000 }),
	        this.props.CurrentSessionHost,
	        _react2.default.createElement(_navLeftBar2.default, { router: router }),
	        this.props.children
	      );
	    }

	    return null;
	  };

	  return App;
	}(_react.Component);

	function mapStateToProps() {
	  return {
	    initAttempt: _getters2.default.initAttempt
	  };
	}

	exports.default = (0, _nuclearJsReactAddons.connect)(mapStateToProps)(App);
	module.exports = exports['default'];

/***/ }),
/* 531 */
/***/ (function(module, exports) {

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

	function detectPlatform() {
	  var userAgent = window.navigator.userAgent;
	  return {
	    isWin: userAgent.indexOf('Windows') >= 0,
	    isMac: userAgent.indexOf('Macintosh') >= 0,
	    isLinux: userAgent.indexOf('Linux') >= 0
	  };
	}

	var platform = exports.platform = detectPlatform();

/***/ }),
/* 532 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.default = NavLeftBar;

	var _react = __webpack_require__(2);

	var _react2 = _interopRequireDefault(_react);

	var _config = __webpack_require__(228);

	var _config2 = _interopRequireDefault(_config);

	var _user = __webpack_require__(250);

	var UserFlux = _interopRequireWildcard(_user);

	var _appStore = __webpack_require__(533);

	var AppStore = _interopRequireWildcard(_appStore);

	var _actions = __webpack_require__(239);

	var _reactRouter = __webpack_require__(164);

	var _icons = __webpack_require__(256);

	function _interopRequireWildcard(obj) { if (obj && obj.__esModule) { return obj; } else { var newObj = {}; if (obj != null) { for (var key in obj) { if (Object.prototype.hasOwnProperty.call(obj, key)) newObj[key] = obj[key]; } } newObj.default = obj; return newObj; } }

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function NavLeftBar(props) {
	  var items = AppStore.getStore().getNavItems();
	  var name = UserFlux.getUser().getName();
	  var $items = items.map(function (i, index) {
	    var className = props.router.isActive(i.to) ? 'active' : '';
	    return _react2.default.createElement(
	      'li',
	      { key: index, className: className, title: i.title },
	      _react2.default.createElement(
	        _reactRouter.IndexLink,
	        { to: i.to },
	        _react2.default.createElement('i', { className: i.icon })
	      )
	    );
	  });

	  $items.push(_react2.default.createElement(
	    'li',
	    { key: $items.length, title: 'help' },
	    _react2.default.createElement(
	      'a',
	      { href: _config2.default.helpUrl, target: '_blank' },
	      _react2.default.createElement('i', { className: 'fa fa-question' })
	    )
	  ));

	  $items.push(_react2.default.createElement(
	    'li',
	    { key: $items.length, title: 'logout' },
	    _react2.default.createElement(
	      'a',
	      { href: '#', onClick: _actions.logout },
	      _react2.default.createElement('i', { className: 'fa fa-sign-out', style: { marginRight: 0 } })
	    )
	  ));

	  return _react2.default.createElement(
	    'nav',
	    { className: 'grv-nav navbar-default', role: 'navigation' },
	    _react2.default.createElement(
	      'ul',
	      { className: 'nav text-center', id: 'side-menu' },
	      _react2.default.createElement(
	        'li',
	        null,
	        _react2.default.createElement(_icons.UserIcon, { name: name })
	      ),
	      $items
	    )
	  );
	} /*
	  Copyright 2015 Gravitational, Inc.
	  
	  Licensed under the Apache License, Version 2.0 (the "License");
	  you may not use this file except in compliance with the License.
	  You may obtain a copy of the License at
	  
	      http://www.apache.org/licenses/LICENSE-2.0
	  
	  Unless required by applicable law or agreed to in writing, software
	  distributed under the License is distributed on an "AS IS" BASIS,
	  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	  See the License for the specific language governing permissions and
	  limitations under the License.
	  */


	NavLeftBar.propTypes = {
	  router: _react2.default.PropTypes.object.isRequired
	};
	module.exports = exports['default'];

/***/ }),
/* 533 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.getStore = getStore;

	var _reactor = __webpack_require__(233);

	var _reactor2 = _interopRequireDefault(_reactor);

	var _nuclearJs = __webpack_require__(234);

	var _immutable = __webpack_require__(253);

	var _actionTypes = __webpack_require__(285);

	var AT = _interopRequireWildcard(_actionTypes);

	function _interopRequireWildcard(obj) { if (obj && obj.__esModule) { return obj; } else { var newObj = {}; if (obj != null) { for (var key in obj) { if (Object.prototype.hasOwnProperty.call(obj, key)) newObj[key] = obj[key]; } } newObj.default = obj; return newObj; } }

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
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
	  _inherits(AppRec, _Record);

	  function AppRec(props) {
	    _classCallCheck(this, AppRec);

	    return _possibleConstructorReturn(this, _Record.call(this, props));
	  }

	  AppRec.prototype.setSiteId = function setSiteId(siteId) {
	    return this.set('siteId', siteId);
	  };

	  AppRec.prototype.getClusterName = function getClusterName() {
	    return this.get('siteId');
	  };

	  AppRec.prototype.getNavItems = function getNavItems() {
	    return this.navItems.toJS();
	  };

	  AppRec.prototype.addNavItem = function addNavItem(navItem) {
	    return this.set('navItems', this.navItems.push(navItem));
	  };

	  return AppRec;
	}((0, _immutable.Record)({
	  siteId: null,
	  navItems: new _immutable.List()
	}));

	function getStore() {
	  return _reactor2.default.evaluate(['tlpt']);
	}

	exports.default = (0, _nuclearJs.Store)({
	  getInitialState: function getInitialState() {
	    return new AppRec();
	  },
	  initialize: function initialize() {
	    this.on(AT.SET_SITE_ID, function (state, siteId) {
	      return state.setSiteId(siteId);
	    });
	    this.on(AT.ADD_NAV_ITEM, function (state, navItem) {
	      return state.addNavItem(navItem);
	    });
	  }
	});

/***/ }),
/* 534 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	var _reactor = __webpack_require__(233);

	var _reactor2 = _interopRequireDefault(_reactor);

	var _store = __webpack_require__(535);

	var _store2 = _interopRequireDefault(_store);

	var _store3 = __webpack_require__(410);

	var _store4 = _interopRequireDefault(_store3);

	var _appStore = __webpack_require__(533);

	var _appStore2 = _interopRequireDefault(_appStore);

	var _nodeStore = __webpack_require__(406);

	var _nodeStore2 = _interopRequireDefault(_nodeStore);

	var _store5 = __webpack_require__(536);

	var _store6 = _interopRequireDefault(_store5);

	var _statusStore = __webpack_require__(252);

	var _statusStore2 = _interopRequireDefault(_statusStore);

	__webpack_require__(392);

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

	_reactor2.default.registerStores({
	  'tlpt_settings': _store6.default,
	  'tlpt': _appStore2.default,
	  'tlpt_terminal': _store2.default,
	  'tlpt_nodes': _nodeStore2.default,
	  'tlpt_user': __webpack_require__(537),
	  'tlpt_user_invite': __webpack_require__(538),
	  'tlpt_user_acl': _store4.default,
	  'tlpt_sites': __webpack_require__(539),
	  'tlpt_status': _statusStore2.default,
	  'tlpt_sessions_events': __webpack_require__(540),
	  'tlpt_sessions_archived': __webpack_require__(541),
	  'tlpt_sessions_active': __webpack_require__(542),
	  'tlpt_sessions_filter': __webpack_require__(543),
	  'tlpt_notifications': __webpack_require__(544)
	});

/***/ }),
/* 535 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;
	exports.TermRec = undefined;

	var _nuclearJs = __webpack_require__(234);

	var _immutable = __webpack_require__(253);

	var _config = __webpack_require__(228);

	var _config2 = _interopRequireDefault(_config);

	var _localStorage = __webpack_require__(242);

	var _localStorage2 = _interopRequireDefault(_localStorage);

	var _actionTypes = __webpack_require__(408);

	function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               limitations under the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               */


	var TermStatusRec = new _immutable.Record({
	  isReady: false,
	  isLoading: false,
	  isNotFound: false,
	  isError: false,
	  errorText: undefined
	});

	var TermRec = exports.TermRec = function (_Record) {
	  _inherits(TermRec, _Record);

	  function TermRec() {
	    _classCallCheck(this, TermRec);

	    return _possibleConstructorReturn(this, _Record.apply(this, arguments));
	  }

	  TermRec.prototype.getClusterName = function getClusterName() {
	    return this.siteId;
	  };

	  TermRec.prototype.getTtyParams = function getTtyParams() {
	    var _localStorage$getBear = _localStorage2.default.getBearerToken(),
	        accessToken = _localStorage$getBear.accessToken;

	    var server_id = this.serverId;
	    return {
	      login: this.login,
	      sid: this.sid,
	      token: accessToken,
	      ttyUrl: _config2.default.api.ttyWsAddr,
	      ttyEventUrl: _config2.default.api.ttyEventWsAddr,
	      ttyResizeUrl: _config2.default.api.ttyResizeUrl,
	      cluster: this.siteId,
	      getTarget: function getTarget() {
	        return { server_id: server_id };
	      }
	    };
	  };

	  TermRec.prototype.getServerLabel = function getServerLabel() {
	    if (this.hostname && this.login) {
	      return this.login + '@' + this.hostname;
	    }

	    if (this.serverId && this.login) {
	      return this.login + '@' + this.serverId;
	    }

	    return '';
	  };

	  return TermRec;
	}((0, _immutable.Record)({
	  status: TermStatusRec(),
	  hostname: null,
	  login: null,
	  siteId: null,
	  serverId: null,
	  sid: null
	}));

	exports.default = (0, _nuclearJs.Store)({
	  getInitialState: function getInitialState() {
	    return new TermRec();
	  },
	  initialize: function initialize() {
	    this.on(_actionTypes.TLPT_TERMINAL_INIT, init);
	    this.on(_actionTypes.TLPT_TERMINAL_CLOSE, close);
	    this.on(_actionTypes.TLPT_TERMINAL_SET_STATUS, changeStatus);
	  }
	});


	function close() {
	  return new TermRec();
	}

	function init(state, json) {
	  return new TermRec(json);
	}

	function changeStatus(state, status) {
	  return state.setIn(['status'], new TermStatusRec(status));
	}

/***/ }),
/* 536 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _nuclearJs = __webpack_require__(234);

	var _immutable = __webpack_require__(253);

	var _actionTypes = __webpack_require__(523);

	var AT = _interopRequireWildcard(_actionTypes);

	function _interopRequireWildcard(obj) { if (obj && obj.__esModule) { return obj; } else { var newObj = {}; if (obj != null) { for (var key in obj) { if (Object.prototype.hasOwnProperty.call(obj, key)) newObj[key] = obj[key]; } } newObj.default = obj; return newObj; } }

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
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
	  _inherits(SettingsRec, _Record);

	  function SettingsRec(params) {
	    _classCallCheck(this, SettingsRec);

	    return _possibleConstructorReturn(this, _Record.call(this, params));
	  }

	  SettingsRec.prototype.isReady = function isReady() {
	    return this.isInitialized;
	  };

	  SettingsRec.prototype.getNavItems = function getNavItems() {
	    return this.navItems.toJS();
	  };

	  SettingsRec.prototype.addNavItem = function addNavItem(navItem) {
	    return this.set('navItems', this.navItems.push(navItem));
	  };

	  return SettingsRec;
	}((0, _immutable.Record)({
	  isInitialized: false,
	  navItems: new _immutable.List()
	}));

	exports.default = (0, _nuclearJs.Store)({
	  getInitialState: function getInitialState() {
	    return new SettingsRec();
	  },
	  initialize: function initialize() {
	    this.on(AT.INIT, function (state) {
	      return state.set('isInitialized', true);
	    });
	    this.on(AT.ADD_NAV_ITEM, function (state, navItem) {
	      return state.addNavItem(navItem);
	    });
	  }
	});
	module.exports = exports['default'];

/***/ }),
/* 537 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _nuclearJs = __webpack_require__(234);

	var _immutable = __webpack_require__(253);

	var _actionTypes = __webpack_require__(249);

	var _enums = __webpack_require__(264);

	function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

	function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

	function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; } /*
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               limitations under the License.
	                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               */

	var UserRec = function (_Record) {
	  _inherits(UserRec, _Record);

	  function UserRec(params) {
	    _classCallCheck(this, UserRec);

	    return _possibleConstructorReturn(this, _Record.call(this, params));
	  }

	  UserRec.prototype.isSso = function isSso() {
	    return this.get('authType') === _enums.AuthTypeEnum.SSO;
	  };

	  UserRec.prototype.getName = function getName() {
	    return this.get('name');
	  };

	  return UserRec;
	}((0, _immutable.Record)({
	  name: '',
	  authType: ''
	}));

	exports.default = (0, _nuclearJs.Store)({
	  getInitialState: function getInitialState() {
	    return (0, _nuclearJs.toImmutable)(null);
	  },
	  initialize: function initialize() {
	    this.on(_actionTypes.RECEIVE_USER, receiveUser);
	  }
	});


	function receiveUser(state, json) {
	  return new UserRec(json);
	}
	module.exports = exports['default'];

/***/ }),
/* 538 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _nuclearJs = __webpack_require__(234);

	var _actionTypes = __webpack_require__(249);

	var _immutable = __webpack_require__(253);

	var Invite = new _immutable.Record({
	  invite_token: '',
	  user: '',
	  qr: ''
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
	    return (0, _nuclearJs.toImmutable)(null);
	  },
	  initialize: function initialize() {
	    this.on(_actionTypes.RECEIVE_INVITE, receiveInvite);
	  }
	});


	function receiveInvite(state, json) {
	  return new Invite(json);
	}
	module.exports = exports['default'];

/***/ }),
/* 539 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _nuclearJs = __webpack_require__(234);

	var _actionTypes = __webpack_require__(286);

	var _immutable = __webpack_require__(253);

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
	    this.on(_actionTypes.RECEIVE_CLUSTERS, receiveSites);
	  }
	});


	function receiveSites(state, json) {
	  return (0, _nuclearJs.toImmutable)(json).map(function (o) {
	    return new Site(o);
	  });
	}
	module.exports = exports['default'];

/***/ }),
/* 540 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _nuclearJs = __webpack_require__(234);

	var _actionTypes = __webpack_require__(390);

	/*
	Copyright 2015 Gravitational, Inc.

	Licensed under the Apache License, Version 2.0 (the "License");
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
	    return (0, _nuclearJs.toImmutable)({});
	  },
	  initialize: function initialize() {
	    this.on(_actionTypes.RECEIVE_SITE_EVENTS, receive);
	  }
	});


	function receive(state, _ref) {
	  var json = _ref.json;

	  var jsonEvents = json || [];
	  return state.withMutations(function (state) {
	    jsonEvents.forEach(function (item) {
	      var sid = item.sid,
	          event = item.event;


	      if (!state.has(sid)) {
	        state.set(sid, (0, _nuclearJs.toImmutable)({}));
	      }

	      state.setIn([sid, event], (0, _nuclearJs.toImmutable)(item));
	    });
	  });
	}
	module.exports = exports['default'];

/***/ }),
/* 541 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _nuclearJs = __webpack_require__(234);

	var _immutable = __webpack_require__(253);

	var _actionTypes = __webpack_require__(390);

	var _enums = __webpack_require__(397);

	/*
	Copyright 2015 Gravitational, Inc.

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

	    http://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
	*/

	var StoredSessionRec = (0, _immutable.Record)({
	  id: undefined,
	  user: undefined,
	  created: undefined,
	  nodeIp: undefined,
	  last_active: undefined,
	  server_id: undefined,
	  siteId: undefined,
	  parties: (0, _immutable.List)()
	});

	exports.default = (0, _nuclearJs.Store)({
	  getInitialState: function getInitialState() {
	    return (0, _nuclearJs.toImmutable)({});
	  },
	  initialize: function initialize() {
	    this.on(_actionTypes.RECEIVE_SITE_EVENTS, receive);
	  }
	});

	// uses events to build stored session objects

	function receive(state, _ref) {
	  var siteId = _ref.siteId,
	      json = _ref.json;

	  var jsonEvents = json || [];
	  var tmp = {};
	  return state.withMutations(function (state) {
	    jsonEvents.forEach(function (item) {
	      if (item.event !== _enums.EventTypeEnum.START && item.event !== _enums.EventTypeEnum.END) {
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

	      if (event === _enums.EventTypeEnum.START) {
	        tmp[sid].created = time;
	        tmp[sid].server_id = server_id;
	        tmp[sid].nodeIp = item['addr.local'];
	        tmp[sid].parties = [{
	          user: user,
	          serverIp: item['addr.remote']
	        }];
	      }

	      // update the store only with new items
	      if (event === _enums.EventTypeEnum.END && !state.has(sid)) {
	        tmp[sid].last_active = time;
	        state.set(sid, new StoredSessionRec((0, _nuclearJs.toImmutable)(tmp[sid])));
	      }
	    });
	  });
	}
	module.exports = exports['default'];

/***/ }),
/* 542 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; }; /*
	                                                                                                                                                                                                                                                                  Copyright 2015 Gravitational, Inc.
	                                                                                                                                                                                                                                                                  
	                                                                                                                                                                                                                                                                  Licensed under the Apache License, Version 2.0 (the "License");
	                                                                                                                                                                                                                                                                  you may not use this file except in compliance with the License.
	                                                                                                                                                                                                                                                                  You may obtain a copy of the License at
	                                                                                                                                                                                                                                                                  
	                                                                                                                                                                                                                                                                      http://www.apache.org/licenses/LICENSE-2.0
	                                                                                                                                                                                                                                                                  
	                                                                                                                                                                                                                                                                  Unless required by applicable law or agreed to in writing, software
	                                                                                                                                                                                                                                                                  distributed under the License is distributed on an "AS IS" BASIS,
	                                                                                                                                                                                                                                                                  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	                                                                                                                                                                                                                                                                  See the License for the specific language governing permissions and
	                                                                                                                                                                                                                                                                  limitations under the License.
	                                                                                                                                                                                                                                                                  */

	var _nuclearJs = __webpack_require__(234);

	var _immutable = __webpack_require__(253);

	var _actionTypes = __webpack_require__(390);

	var ActiveSessionRec = (0, _immutable.Record)({
	  id: undefined,
	  namespace: undefined,
	  login: undefined,
	  active: undefined,
	  created: undefined,
	  last_active: undefined,
	  server_id: undefined,
	  siteId: undefined,
	  parties: (0, _immutable.List)()
	});

	var PartyRecord = (0, _immutable.Record)({
	  user: undefined,
	  serverIp: undefined,
	  serverId: undefined
	});

	var defaultState = function defaultState() {
	  return (0, _nuclearJs.toImmutable)({});
	};

	exports.default = (0, _nuclearJs.Store)({
	  getInitialState: function getInitialState() {
	    return defaultState();
	  },
	  initialize: function initialize() {
	    this.on(_actionTypes.RECEIVE_ACTIVE_SESSIONS, receive);
	    this.on(_actionTypes.UPDATE_ACTIVE_SESSION, updateSession);
	  }
	});


	function updateSession(state, _ref) {
	  var siteId = _ref.siteId,
	      json = _ref.json;

	  var rec = createSessionRec(siteId, json);
	  return rec.equals(state.get(rec.id)) ? state : state.set(rec.id, rec);
	}

	function receive(state, _ref2) {
	  var siteId = _ref2.siteId,
	      json = _ref2.json;

	  var jsonArray = json || [];
	  var newState = defaultState().withMutations(function (newState) {
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
	  var parties = createParties(json.parties);
	  var rec = new ActiveSessionRec((0, _nuclearJs.toImmutable)(_extends({}, json, {
	    siteId: siteId,
	    parties: parties
	  })));

	  return rec;
	}

	function createParties(jsonArray) {
	  jsonArray = jsonArray || [];
	  var list = new _immutable.List();
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
	module.exports = exports['default'];

/***/ }),
/* 543 */
/***/ (function(module, exports, __webpack_require__) {

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

	var _require = __webpack_require__(234),
	    Store = _require.Store,
	    toImmutable = _require.toImmutable;

	var moment = __webpack_require__(291);

	var _require2 = __webpack_require__(425),
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

/***/ }),
/* 544 */
/***/ (function(module, exports, __webpack_require__) {

	'use strict';

	exports.__esModule = true;

	var _nuclearJs = __webpack_require__(234);

	var _actionTypes = __webpack_require__(545);

	/*
	Copyright 2015 Gravitational, Inc.

	Licensed under the Apache License, Version 2.0 (the "License");
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

/***/ }),
/* 545 */
/***/ (function(module, exports) {

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

	var TLPT_NOTIFICATIONS_ADD = exports.TLPT_NOTIFICATIONS_ADD = 'TLPT_NOTIFICATIONS_ADD';

/***/ })
]);