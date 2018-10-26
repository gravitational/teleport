var grvlib =
/******/ (function(modules) { // webpackBootstrap
/******/ 	// The module cache
/******/ 	var installedModules = {};
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
/******/ 			Object.defineProperty(exports, name, {
/******/ 				configurable: false,
/******/ 				enumerable: true,
/******/ 				get: getter
/******/ 			});
/******/ 		}
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
/******/ 	__webpack_require__.p = "/";
/******/
/******/ 	// Load entry module and return exports
/******/ 	return __webpack_require__(__webpack_require__.s = 1);
/******/ })
/************************************************************************/
/******/ ([
/* 0 */
/***/ (function(module, exports) {

module.exports = $;

/***/ }),
/* 1 */
/***/ (function(module, exports, __webpack_require__) {

module.exports = __webpack_require__(2);


/***/ }),
/* 2 */
/***/ (function(module, exports, __webpack_require__) {

"use strict";


Object.defineProperty(exports, "__esModule", {
  value: true
});

var _topNav = __webpack_require__(3);

var _topNav2 = _interopRequireDefault(_topNav);

var _secondaryNav = __webpack_require__(4);

var _secondaryNav2 = _interopRequireDefault(_secondaryNav);

var _sideNav = __webpack_require__(5);

var _sideNav2 = _interopRequireDefault(_sideNav);

var _buttons = __webpack_require__(6);

__webpack_require__(7);

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

var lib = {
  TopNav: _topNav2.default,
  SecondaryNav: _secondaryNav2.default,
  SideNav: _sideNav2.default,
  buttonRipple: _buttons.buttonRipple,
  buttonSmoothScroll: _buttons.buttonSmoothScroll
}; // modules
exports.default = lib;

/***/ }),
/* 3 */
/***/ (function(module, exports, __webpack_require__) {

"use strict";


Object.defineProperty(exports, "__esModule", {
  value: true
});

var _createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }(); // dependancies


var _jquery = __webpack_require__(0);

var _jquery2 = _interopRequireDefault(_jquery);

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

/**
 * Top Navigation
 * @class
 * @param {String} id - The id of the navigation element
 * @param {Boolean} pinned - pin the navigation on scroll
 * @returns {Object}
 */

var TopNav = function () {
  function TopNav() {
    var settings = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : { id: '', pinned: false };

    _classCallCheck(this, TopNav);

    var id = settings.id,
        pinned = settings.pinned;

    var elementId = id || '#top-nav';

    // register elements & vars
    this.$window = (0, _jquery2.default)(window);
    this.$nav = (0, _jquery2.default)(elementId);
    this.$trigger = this.$nav.find('#top-nav-trigger');
    this.$close = this.$nav.find('#top-nav-close');
    this.$menu = this.$nav.find('#top-nav-menu');
    this.$cta = this.$nav.find('#top-nav-cta');
    this.$dropDownButtons = this.$nav.find('.top-nav-button.has-dropdown');
    this.$overlays = this.$nav.find('.top-nav-dropdown-overlay');
    this.currentPath = window.location.pathname;

    // activate event listeners
    this.activateDropdownMenus();
    this.activateMobileMenu();
    this.updateCta();

    // pin top nav
    if (pinned) {
      this.pinTopNav();
    }
  }

  _createClass(TopNav, [{
    key: 'activateDropdownMenus',
    value: function activateDropdownMenus() {
      // listen for dropdown button click
      this.$dropDownButtons.on('click', function (e) {
        e.stopImmediatePropagation();
        var $button = (0, _jquery2.default)(e.currentTarget);
        var $dropdown = $button.find('.top-nav-dropdown');
        var $overlay = $button.find('.top-nav-dropdown-overlay');

        $button.toggleClass('is-active');
        $overlay.toggleClass('is-hidden');
        $dropdown.toggleClass('is-hidden');
      });

      // close menus when overlay is clicked
      this.$overlays.on('click', function (e) {
        e.stopImmediatePropagation();
        var $overlay = (0, _jquery2.default)(e.currentTarget);
        var $dropdown = $overlay.siblings('.top-nav-dropdown');
        var $button = $overlay.parent();

        $button.toggleClass('is-active');
        $dropdown.toggleClass('is-hidden');
        $overlay.toggleClass('is-hidden');
      });
    }
  }, {
    key: 'activateMobileMenu',
    value: function activateMobileMenu() {
      var _this = this;

      this.$trigger.on('click', function (e) {
        e.preventDefault();

        _this.$trigger.addClass('is-hidden');
        _this.$close.addClass('is-visible');
        _this.$menu.addClass('is-visible');
      });

      this.$close.on('click', function (e) {
        e.preventDefault();

        _this.$trigger.removeClass('is-hidden');
        _this.$close.removeClass('is-visible');
        _this.$menu.removeClass('is-visible');
      });
    }
  }, {
    key: 'pinTopNav',
    value: function pinTopNav() {
      var _this2 = this;

      if (this.$window[0].pageYOffset > 2) {
        this.$nav.addClass("is-fixed");
      }

      this.$window.on("scroll", function () {
        if (_this2.$window[0].pageYOffset > 200) {
          _this2.$nav.addClass("is-fixed");
        } else {
          _this2.$nav.removeClass("is-fixed");
        }
      });
    }
  }, {
    key: 'updateCta',
    value: function updateCta() {
      // change cta to teleport demo on teleport pages
      if (this.$cta.length && this.currentPath.includes('/teleport/')) {
        this.$cta.attr('href', '/teleport/demo/');
        this.$cta.text('Demo Teleport');
      }

      // change cta to telekube demo on telekube pages
      if (this.$cta.length && this.currentPath.includes('/gravity/')) {
        this.$cta.attr('href', '/gravity/demo/');
        this.$cta.text('Demo Gravity');
      }
    }
  }]);

  return TopNav;
}();

exports.default = TopNav;

/***/ }),
/* 4 */
/***/ (function(module, exports, __webpack_require__) {

"use strict";


Object.defineProperty(exports, "__esModule", {
  value: true
});

var _createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }(); // dependancies


var _jquery = __webpack_require__(0);

var _jquery2 = _interopRequireDefault(_jquery);

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

var SecondaryNav = function () {
  function SecondaryNav() {
    var settings = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : { id: '', pinned: false };

    _classCallCheck(this, SecondaryNav);

    var id = settings.id,
        pinned = settings.pinned;

    var elementId = id || '#secondary-nav';

    // register elements & vars
    this.$window = (0, _jquery2.default)(window);
    this.$secondaryNav = (0, _jquery2.default)(elementId);
    this.$trigger = this.$secondaryNav.find('#secondary-nav-trigger');
    this.$close = this.$secondaryNav.find('#secondary-nav-close');
    this.$menu = this.$secondaryNav.find('#secondary-nav-menu');
    this.$buttons = this.$secondaryNav.find('.secondary-nav-button');
    this.currentPath = window.location.pathname;
    this.productName = this.$secondaryNav.data('name');

    // activate navigation
    this.activateMenuHighlights();
    this.activateMobileMenu();

    // pin nav
    if (pinned) {
      this.pinSecondaryNav();
    }
  }

  _createClass(SecondaryNav, [{
    key: 'activateMenuHighlights',
    value: function activateMenuHighlights() {
      var that = this;

      this.$buttons.each(function (index, el) {
        var $button = (0, _jquery2.default)(el);
        var href = $button.attr('href');
        var path = href.replace(/\.\.\//g, '');
        var paths = path.split('/');

        if (that.currentPath === '/' + path) {
          $button.addClass('is-active');
        } else if (that.currentPath.includes('/' + path) && paths.length >= 3) {
          $button.addClass('is-active');
        }
      });
    }
  }, {
    key: 'activateMobileMenu',
    value: function activateMobileMenu() {
      var _this = this;

      this.$trigger.on('click', function (e) {
        e.preventDefault();

        _this.$trigger.addClass('is-hidden');
        _this.$close.addClass('is-visible');
        _this.$menu.addClass('is-visible');
      });

      this.$close.on('click', function (e) {
        e.preventDefault();

        _this.$trigger.removeClass('is-hidden');
        _this.$close.removeClass('is-visible');
        _this.$menu.removeClass('is-visible');
      });
    }
  }, {
    key: 'pinSecondaryNav',
    value: function pinSecondaryNav() {
      var _this2 = this;

      if (this.$window[0].pageYOffset > 2) {
        this.$secondaryNav.addClass("is-fixed");
      }

      this.$window.on("scroll", function () {
        if (_this2.$window[0].pageYOffset > 200) {
          _this2.$secondaryNav.addClass("is-fixed");
        } else {
          _this2.$secondaryNav.removeClass("is-fixed");
        }
      });
    }
  }]);

  return SecondaryNav;
}();

exports.default = SecondaryNav;

/***/ }),
/* 5 */
/***/ (function(module, exports, __webpack_require__) {

"use strict";


Object.defineProperty(exports, "__esModule", {
  value: true
});

var _createClass = function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; }();

var _jquery = __webpack_require__(0);

var _jquery2 = _interopRequireDefault(_jquery);

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

var SideNav = function () {
  function SideNav() {
    var settings = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : { id: '', pinned: false };

    _classCallCheck(this, SideNav);

    var id = settings.id,
        pinned = settings.pinned;

    var elementId = id || '#side-nav';

    this.$window = (0, _jquery2.default)(window);
    this.$nav = (0, _jquery2.default)(elementId);
    this.$trigger = (0, _jquery2.default)('#side-nav-trigger');
    this.$close = (0, _jquery2.default)('#side-nav-close');
    this.$menu = (0, _jquery2.default)('#side-nav-menu');
    this.$buttons = (0, _jquery2.default)('.side-nav-buttons a');
    this.$secondaryButtons = (0, _jquery2.default)('.side-nav-secondary-buttons a');
    this.currentPath = window.location.pathname;

    // BIND METHODS
    this.closeMenu = this.closeMenu.bind(this);
    this.openMenu = this.openMenu.bind(this);

    if (this.$nav.length) {
      this.activateMenuHighlights();
      this.activateMobileMenu();

      if (pinned) {
        this.pinSideNav();
      }
    }
  }

  _createClass(SideNav, [{
    key: 'activateMenuHighlights',
    value: function activateMenuHighlights() {
      var that = this;

      this.$buttons.each(function (index, el) {
        var $button = (0, _jquery2.default)(el);
        var href = $button.attr('href');
        var path = href.replace(/\.\.\//g, '');

        if (that.currentPath === '/resources/' && path === 'resources/') {
          $button.addClass('is-active');
        } else if (that.currentPath.includes(path) && path !== 'resources/') {
          $button.addClass('is-active');
        }
      });
    }
  }, {
    key: 'pinSideNav',
    value: function pinSideNav() {
      var _this = this;

      if (this.$window[0].pageYOffset > 2) {
        this.$nav.addClass("is-fixed");
      }

      this.$window.on("scroll", function () {
        if (_this.$window[0].pageYOffset > 80) {
          _this.$nav.addClass("is-fixed");
        } else {
          _this.$nav.removeClass("is-fixed");
        }
      });
    }
  }, {
    key: 'closeMenu',
    value: function closeMenu(e) {
      if (e) {
        e.stopPropagation();
      }

      this.$nav.removeClass('is-active');
      this.$trigger.removeClass('is-hidden');
      this.$close.removeClass('is-visible');
      this.$menu.removeClass('is-visible');
    }
  }, {
    key: 'openMenu',
    value: function openMenu(e) {
      if (e) {
        e.stopPropagation();
      }

      this.$nav.addClass('is-active');
      this.$trigger.addClass('is-hidden');
      this.$close.addClass('is-visible');
      this.$menu.addClass('is-visible');
    }
  }, {
    key: 'activateMobileMenu',
    value: function activateMobileMenu() {
      this.$trigger.on('click', this.openMenu);
      this.$close.on('click', this.closeMenu);
      this.$secondaryButtons.on('click', this.closeMenu);
    }
  }]);

  return SideNav;
}();

exports.default = SideNav;

/***/ }),
/* 6 */
/***/ (function(module, exports, __webpack_require__) {

"use strict";


Object.defineProperty(exports, "__esModule", {
  value: true
});
exports.buttonSmoothScroll = exports.buttonRipple = undefined;

var _jquery = __webpack_require__(0);

var _jquery2 = _interopRequireDefault(_jquery);

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { default: obj }; }

/**
 * Button Ripple
 * @function
 * @param {String} buttonClass - The class name for button '.button'
 * @description - you can add a "data-ripple-color" attribute to the
 *                button to manually set the ripple color.
 */

var buttonRipple = function buttonRipple(buttonClass) {
  var buttonClassName = buttonClass || '.button';
  var $buttons = (0, _jquery2.default)(buttonClassName);

  $buttons.on('click', function (e) {
    (0, _jquery2.default)('.button-ripple').remove();

    // BUTTON
    var button = (0, _jquery2.default)(this);
    var buttonHeight = button.height();

    // FIND POSITION
    var btnOffset = button.offset();
    var xPos = e.pageX - btnOffset.left;
    var yPos = e.pageY - btnOffset.top;

    // CREATE RIPPLE CONTAINER
    var $ripple = (0, _jquery2.default)('<div/>');
    $ripple.addClass('button-ripple');

    // SET SIZE FOR RIPPLE CONTAINER
    $ripple.css('height', buttonHeight);
    $ripple.css('width', buttonHeight);

    // SET POSITION FOR RIPPLE CONTAINER
    $ripple.css({
      top: yPos - $ripple.height() / 2,
      left: xPos - $ripple.width() / 2,
      background: button.data('ripple-color')
    }).appendTo(button);

    // REMOVE ELEMENT AFTER 2 SECS
    setTimeout(function () {
      $ripple.remove();
    }, 2000);
  });
};

var buttonSmoothScroll = function buttonSmoothScroll(offset) {
  offset = offset || 132;
  // Select all links with hashes
  (0, _jquery2.default)('a[href*="#"]')
  // Remove links that don't actually link to anything
  .not('[href="#"]').not('[href="#0"]').click(function () {
    // On-page links
    if (location.pathname.replace(/^\//, '') == this.pathname.replace(/^\//, '') && location.hostname == this.hostname) {
      // Figure out element to scroll to
      var target = (0, _jquery2.default)(this.hash);
      target = target.length ? target : (0, _jquery2.default)('[name=' + this.hash.slice(1) + ']');
      // Does a scroll target exist?
      if (target.length) {
        // Only prevent default if animation is actually gonna happen
        (0, _jquery2.default)('html, body').animate({
          scrollTop: target.offset().top - offset
        }, 1000, function () {
          // Callback after animation
          // Must change focus!
          var $target = (0, _jquery2.default)(target);
          $target.focus();
          if ($target.is(":focus")) {
            // Checking if the target was focused
            return false;
          } else {
            $target.attr('tabindex', '-1'); // Adding tabindex for elements not focusable
            $target.focus(); // Set focus again
          }
        });
      }
    }
  });
};

exports.buttonRipple = buttonRipple;
exports.buttonSmoothScroll = buttonSmoothScroll;

/***/ }),
/* 7 */
/***/ (function(module, exports) {

// removed by extract-text-webpack-plugin

/***/ })
/******/ ])["default"];