(window["webpackJsonp"] = window["webpackJsonp"] || []).push([[2],{

/***/ "8edf":
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
__webpack_require__.r(__webpack_exports__);
/* WEBPACK VAR INJECTION */(function(module) {/* harmony import */ var react_hot_loader__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__("g0WC");
/* harmony import */ var react_hot_loader__WEBPACK_IMPORTED_MODULE_0___default = /*#__PURE__*/__webpack_require__.n(react_hot_loader__WEBPACK_IMPORTED_MODULE_0__);
/* harmony import */ var react__WEBPACK_IMPORTED_MODULE_1__ = __webpack_require__("ERkP");
/* harmony import */ var react__WEBPACK_IMPORTED_MODULE_1___default = /*#__PURE__*/__webpack_require__.n(react__WEBPACK_IMPORTED_MODULE_1__);
/* harmony import */ var gravity_config__WEBPACK_IMPORTED_MODULE_2__ = __webpack_require__("20nU");
/* harmony import */ var gravity_components_Router__WEBPACK_IMPORTED_MODULE_3__ = __webpack_require__("5bvh");
/* harmony import */ var _components_Installer__WEBPACK_IMPORTED_MODULE_4__ = __webpack_require__("uxHf");
/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/






function InstallerApp() {
  return react__WEBPACK_IMPORTED_MODULE_1___default.a.createElement(gravity_components_Router__WEBPACK_IMPORTED_MODULE_3__[/* Switch */ "e"], null, react__WEBPACK_IMPORTED_MODULE_1___default.a.createElement(gravity_components_Router__WEBPACK_IMPORTED_MODULE_3__[/* Route */ "c"], {
    title: "Installer",
    path: gravity_config__WEBPACK_IMPORTED_MODULE_2__[/* default */ "a"].routes.installerApp,
    component: _components_Installer__WEBPACK_IMPORTED_MODULE_4__[/* default */ "a"]
  }), react__WEBPACK_IMPORTED_MODULE_1___default.a.createElement(gravity_components_Router__WEBPACK_IMPORTED_MODULE_3__[/* Route */ "c"], {
    title: "Installer",
    path: gravity_config__WEBPACK_IMPORTED_MODULE_2__[/* default */ "a"].routes.installerCluster,
    component: _components_Installer__WEBPACK_IMPORTED_MODULE_4__[/* default */ "a"]
  }));
}

/* harmony default export */ __webpack_exports__["default"] = (Object(react_hot_loader__WEBPACK_IMPORTED_MODULE_0__["hot"])(module)(InstallerApp));
/* WEBPACK VAR INJECTION */}.call(this, __webpack_require__("cyaT")(module)))

/***/ }),

/***/ "SEMh":
/***/ (function(module, exports, __webpack_require__) {

var __WEBPACK_AMD_DEFINE_FACTORY__, __WEBPACK_AMD_DEFINE_ARRAY__, __WEBPACK_AMD_DEFINE_RESULT__;function _typeof(obj) { if (typeof Symbol === "function" && typeof Symbol.iterator === "symbol") { _typeof = function _typeof(obj) { return typeof obj; }; } else { _typeof = function _typeof(obj) { return obj && typeof Symbol === "function" && obj.constructor === Symbol && obj !== Symbol.prototype ? "symbol" : typeof obj; }; } return _typeof(obj); }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
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
The MIT License (MIT)

Copyright (c) 2014 Michal Powaga

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.

*/

/*global define */
(function (root, factory) {
  if (true) {
    !(__WEBPACK_AMD_DEFINE_ARRAY__ = [__webpack_require__("ERkP"), __webpack_require__("aWzz"), __webpack_require__("Y3fD")], __WEBPACK_AMD_DEFINE_FACTORY__ = (factory),
				__WEBPACK_AMD_DEFINE_RESULT__ = (typeof __WEBPACK_AMD_DEFINE_FACTORY__ === 'function' ?
				(__WEBPACK_AMD_DEFINE_FACTORY__.apply(exports, __WEBPACK_AMD_DEFINE_ARRAY__)) : __WEBPACK_AMD_DEFINE_FACTORY__),
				__WEBPACK_AMD_DEFINE_RESULT__ !== undefined && (module.exports = __WEBPACK_AMD_DEFINE_RESULT__));
  } else {}
})(this, function (React, PropTypes, createReactClass) {
  /**
   * To prevent text selection while dragging.
   * http://stackoverflow.com/questions/5429827/how-can-i-prevent-text-element-selection-with-cursor-drag
   */
  function pauseEvent(e) {
    if (e.stopPropagation) e.stopPropagation();
    if (e.preventDefault) e.preventDefault();
    return false;
  }

  function stopPropagation(e) {
    if (e.stopPropagation) e.stopPropagation();
  }
  /**
   * Spreads `count` values equally between `min` and `max`.
   */


  function linspace(min, max, count) {
    var range = (max - min) / (count - 1);
    var res = [];

    for (var i = 0; i < count; i++) {
      res.push(min + range * i);
    }

    return res;
  }

  function ensureArray(x) {
    return x == null ? [] : Array.isArray(x) ? x : [x];
  }

  function undoEnsureArray(x) {
    return x != null && x.length === 1 ? x[0] : x;
  }

  var isArray = Array.isArray || function (x) {
    return Object.prototype.toString.call(x) === '[object Array]';
  }; // undoEnsureArray(ensureArray(x)) === x


  var ReactSlider = createReactClass({
    displayName: 'ReactSlider',
    propTypes: {
      /**
       * The minimum value of the slider.
       */
      min: PropTypes.number,

      /**
       * The maximum value of the slider.
       */
      max: PropTypes.number,

      /**
       * Value to be added or subtracted on each step the slider makes.
       * Must be greater than zero.
       * `max - min` should be evenly divisible by the step value.
       */
      step: PropTypes.number,

      /**
       * The minimal distance between any pair of handles.
       * Must be positive, but zero means they can sit on top of each other.
       */
      minDistance: PropTypes.number,

      /**
       * Determines the initial positions of the handles and the number of handles if the component has no children.
       *
       * If a number is passed a slider with one handle will be rendered.
       * If an array is passed each value will determine the position of one handle.
       * The values in the array must be sorted.
       * If the component has children, the length of the array must match the number of children.
       */
      defaultValue: PropTypes.oneOfType([PropTypes.number, PropTypes.arrayOf(PropTypes.number)]),

      /**
       * Like `defaultValue` but for [controlled components](http://facebook.github.io/react/docs/forms.html#controlled-components).
       */
      value: PropTypes.oneOfType([PropTypes.number, PropTypes.arrayOf(PropTypes.number)]),

      /**
       * Determines whether the slider moves horizontally (from left to right) or vertically (from top to bottom).
       */
      orientation: PropTypes.oneOf(['horizontal', 'vertical']),

      /**
       * The css class set on the slider node.
       */
      className: PropTypes.string,

      /**
       * The css class set on each handle node.
       *
       * In addition each handle will receive a numbered css class of the form `${handleClassName}-${i}`,
       * e.g. `handle-0`, `handle-1`, ...
       */
      handleClassName: PropTypes.string,

      /**
       * The css class set on the handle that is currently being moved.
       */
      handleActiveClassName: PropTypes.string,

      /**
       * If `true` bars between the handles will be rendered.
       */
      withBars: PropTypes.bool,

      /**
       * The css class set on the bars between the handles.
       * In addition bar fragment will receive a numbered css class of the form `${barClassName}-${i}`,
       * e.g. `bar-0`, `bar-1`, ...
       */
      barClassName: PropTypes.string,

      /**
       * If `true` the active handle will push other handles
       * within the constraints of `min`, `max`, `step` and `minDistance`.
       */
      pearling: PropTypes.bool,

      /**
       * If `true` the handles can't be moved.
       */
      disabled: PropTypes.bool,

      /**
       * Disables handle move when clicking the slider bar
       */
      snapDragDisabled: PropTypes.bool,

      /**
       * Inverts the slider.
       */
      invert: PropTypes.bool,

      /**
       * Callback called before starting to move a handle.
       */
      onBeforeChange: PropTypes.func,

      /**
       * Callback called on every value change.
       */
      onChange: PropTypes.func,

      /**
       * Callback called only after moving a handle has ended.
       */
      onAfterChange: PropTypes.func,

      /**
       *  Callback called when the the slider is clicked (handle or bars).
       *  Receives the value at the clicked position as argument.
       */
      onSliderClick: PropTypes.func
    },
    getDefaultProps: function getDefaultProps() {
      return {
        min: 0,
        max: 100,
        step: 1,
        minDistance: 0,
        defaultValue: 0,
        orientation: 'horizontal',
        className: 'slider',
        handleClassName: 'handle',
        handleActiveClassName: 'active',
        barClassName: 'bar',
        withBars: false,
        pearling: false,
        disabled: false,
        snapDragDisabled: false,
        invert: false
      };
    },
    getInitialState: function getInitialState() {
      var value = this._or(ensureArray(this.props.value), ensureArray(this.props.defaultValue)); // reused throughout the component to store results of iterations over `value`


      this.tempArray = value.slice(); // array for storing resize timeouts ids

      this.pendingResizeTimeouts = [];
      var zIndices = [];

      for (var i = 0; i < value.length; i++) {
        value[i] = this._trimAlignValue(value[i], this.props);
        zIndices.push(i);
      }

      return {
        index: -1,
        upperBound: 0,
        sliderLength: 0,
        value: value,
        zIndices: zIndices
      };
    },
    // Keep the internal `value` consistent with an outside `value` if present.
    // This basically allows the slider to be a controlled component.
    UNSAFE_componentWillReceiveProps: function UNSAFE_componentWillReceiveProps(newProps) {
      var value = this._or(ensureArray(newProps.value), this.state.value); // ensure the array keeps the same size as `value`


      this.tempArray = value.slice();

      for (var i = 0; i < value.length; i++) {
        this.state.value[i] = this._trimAlignValue(value[i], newProps);
      }

      if (this.state.value.length > value.length) this.state.value.length = value.length; // If an upperBound has not yet been determined (due to the component being hidden
      // during the mount event, or during the last resize), then calculate it now

      if (this.state.upperBound === 0) {
        this._resize();
      }
    },
    // Check if the arity of `value` or `defaultValue` matches the number of children (= number of custom handles).
    // If no custom handles are provided, just returns `value` if present and `defaultValue` otherwise.
    // If custom handles are present but neither `value` nor `defaultValue` are applicable the handles are spread out
    // equally.
    // TODO: better name? better solution?
    _or: function _or(value, defaultValue) {
      var count = React.Children.count(this.props.children);

      switch (count) {
        case 0:
          return value.length > 0 ? value : defaultValue;

        case value.length:
          return value;

        case defaultValue.length:
          return defaultValue;

        default:
          if (value.length !== count || defaultValue.length !== count) {
            window.console.warn(this.constructor.displayName + ": Number of values does not match number of children.");
          }

          return linspace(this.props.min, this.props.max, count);
      }
    },
    componentDidMount: function componentDidMount() {
      window.addEventListener('resize', this._handleResize);

      this._resize();
    },
    componentWillUnmount: function componentWillUnmount() {
      this._clearPendingResizeTimeouts();

      window.removeEventListener('resize', this._handleResize);
    },
    getValue: function getValue() {
      return undoEnsureArray(this.state.value);
    },
    _resize: function _resize() {
      var slider = this.slider;
      var handle = this.handle0;
      var rect = slider.getBoundingClientRect();

      var size = this._sizeKey();

      var sliderMax = rect[this._posMaxKey()];

      var sliderMin = rect[this._posMinKey()];

      this.setState({
        upperBound: slider[size] - handle[size],
        sliderLength: Math.abs(sliderMax - sliderMin),
        handleSize: handle[size],
        sliderStart: this.props.invert ? sliderMax : sliderMin
      });
    },
    _handleResize: function _handleResize() {
      // setTimeout of 0 gives element enough time to have assumed its new size if it is being resized
      var resizeTimeout = window.setTimeout(function () {
        // drop this timeout from pendingResizeTimeouts to reduce memory usage
        this.pendingResizeTimeouts.shift();

        this._resize();
      }.bind(this), 0);
      this.pendingResizeTimeouts.push(resizeTimeout);
    },
    // clear all pending timeouts to avoid error messages after unmounting
    _clearPendingResizeTimeouts: function _clearPendingResizeTimeouts() {
      do {
        var nextTimeout = this.pendingResizeTimeouts.shift();
        clearTimeout(nextTimeout);
      } while (this.pendingResizeTimeouts.length);
    },
    // calculates the offset of a handle in pixels based on its value.
    _calcOffset: function _calcOffset(value) {
      var range = this.props.max - this.props.min;

      if (range === 0) {
        return 0;
      }

      var ratio = (value - this.props.min) / range;
      return ratio * this.state.upperBound;
    },
    // calculates the value corresponding to a given pixel offset, i.e. the inverse of `_calcOffset`.
    _calcValue: function _calcValue(offset) {
      var ratio = offset / this.state.upperBound;
      return ratio * (this.props.max - this.props.min) + this.props.min;
    },
    _buildHandleStyle: function _buildHandleStyle(offset, i) {
      var style = {
        position: 'absolute',
        willChange: this.state.index >= 0 ? this._posMinKey() : '',
        zIndex: this.state.zIndices.indexOf(i) + 1
      };
      style[this._posMinKey()] = offset + 'px';
      return style;
    },
    _buildBarStyle: function _buildBarStyle(min, max) {
      var obj = {
        position: 'absolute',
        willChange: this.state.index >= 0 ? this._posMinKey() + ',' + this._posMaxKey() : ''
      };
      obj[this._posMinKey()] = min;
      obj[this._posMaxKey()] = max;
      return obj;
    },
    _getClosestIndex: function _getClosestIndex(pixelOffset) {
      var minDist = Number.MAX_VALUE;
      var closestIndex = -1;
      var value = this.state.value;
      var l = value.length;

      for (var i = 0; i < l; i++) {
        var offset = this._calcOffset(value[i]);

        var dist = Math.abs(pixelOffset - offset);

        if (dist < minDist) {
          minDist = dist;
          closestIndex = i;
        }
      }

      return closestIndex;
    },
    _calcOffsetFromPosition: function _calcOffsetFromPosition(position) {
      var pixelOffset = position - this.state.sliderStart;
      if (this.props.invert) pixelOffset = this.state.sliderLength - pixelOffset;
      pixelOffset -= this.state.handleSize / 2;
      return pixelOffset;
    },
    // Snaps the nearest handle to the value corresponding to `position` and calls `callback` with that handle's index.
    _forceValueFromPosition: function _forceValueFromPosition(position, callback) {
      var pixelOffset = this._calcOffsetFromPosition(position);

      var closestIndex = this._getClosestIndex(pixelOffset);

      var nextValue = this._trimAlignValue(this._calcValue(pixelOffset));

      var value = this.state.value.slice(); // Clone this.state.value since we'll modify it temporarily

      value[closestIndex] = nextValue; // Prevents the slider from shrinking below `props.minDistance`

      for (var i = 0; i < value.length - 1; i += 1) {
        if (value[i + 1] - value[i] < this.props.minDistance) return;
      }

      this.setState({
        value: value
      }, callback.bind(this, closestIndex));
    },
    _getMousePosition: function _getMousePosition(e) {
      return [e['page' + this._axisKey()], e['page' + this._orthogonalAxisKey()]];
    },
    _getTouchPosition: function _getTouchPosition(e) {
      var touch = e.touches[0];
      return [touch['page' + this._axisKey()], touch['page' + this._orthogonalAxisKey()]];
    },
    _getKeyDownEventMap: function _getKeyDownEventMap() {
      return {
        'keydown': this._onKeyDown,
        'focusout': this._onBlur
      };
    },
    _getMouseEventMap: function _getMouseEventMap() {
      return {
        'mousemove': this._onMouseMove,
        'mouseup': this._onMouseUp
      };
    },
    _getTouchEventMap: function _getTouchEventMap() {
      return {
        'touchmove': this._onTouchMove,
        'touchend': this._onTouchEnd
      };
    },
    // create the `keydown` handler for the i-th handle
    _createOnKeyDown: function _createOnKeyDown(i) {
      return function (e) {
        if (this.props.disabled) return;

        this._start(i);

        this._addHandlers(this._getKeyDownEventMap());

        pauseEvent(e);
      }.bind(this);
    },
    // create the `mousedown` handler for the i-th handle
    _createOnMouseDown: function _createOnMouseDown(i) {
      return function (e) {
        if (this.props.disabled) return;

        var position = this._getMousePosition(e);

        this._start(i, position[0]);

        this._addHandlers(this._getMouseEventMap());

        pauseEvent(e);
      }.bind(this);
    },
    // create the `touchstart` handler for the i-th handle
    _createOnTouchStart: function _createOnTouchStart(i) {
      return function (e) {
        if (this.props.disabled || e.touches.length > 1) return;

        var position = this._getTouchPosition(e);

        this.startPosition = position;
        this.isScrolling = undefined; // don't know yet if the user is trying to scroll

        this._start(i, position[0]);

        this._addHandlers(this._getTouchEventMap());

        stopPropagation(e);
      }.bind(this);
    },
    _addHandlers: function _addHandlers(eventMap) {
      for (var key in eventMap) {
        document.addEventListener(key, eventMap[key], false);
      }
    },
    _removeHandlers: function _removeHandlers(eventMap) {
      for (var key in eventMap) {
        document.removeEventListener(key, eventMap[key], false);
      }
    },
    _start: function _start(i, position) {
      var activeEl = document.activeElement;
      var handleRef = this['handle' + i]; // if activeElement is body window will lost focus in IE9

      if (activeEl && activeEl != document.body && activeEl != handleRef) {
        activeEl.blur && activeEl.blur();
      }

      this.hasMoved = false;

      this._fireChangeEvent('onBeforeChange');

      var zIndices = this.state.zIndices;
      zIndices.splice(zIndices.indexOf(i), 1); // remove wherever the element is

      zIndices.push(i); // add to end

      this.setState(function (prevState) {
        return {
          startValue: this.state.value[i],
          startPosition: position !== undefined ? position : prevState.startPosition,
          index: i,
          zIndices: zIndices
        };
      });
    },
    _onMouseUp: function _onMouseUp() {
      this._onEnd(this._getMouseEventMap());
    },
    _onTouchEnd: function _onTouchEnd() {
      this._onEnd(this._getTouchEventMap());
    },
    _onBlur: function _onBlur() {
      this._onEnd(this._getKeyDownEventMap());
    },
    _onEnd: function _onEnd(eventMap) {
      this._removeHandlers(eventMap);

      this.setState({
        index: -1
      }, this._fireChangeEvent.bind(this, 'onAfterChange'));
    },
    _onMouseMove: function _onMouseMove(e) {
      var position = this._getMousePosition(e);

      var diffPosition = this._getDiffPosition(position[0]);

      var newValue = this._getValueFromPosition(diffPosition);

      this._move(newValue);
    },
    _onTouchMove: function _onTouchMove(e) {
      if (e.touches.length > 1) return;

      var position = this._getTouchPosition(e);

      if (typeof this.isScrolling === 'undefined') {
        var diffMainDir = position[0] - this.startPosition[0];
        var diffScrollDir = position[1] - this.startPosition[1];
        this.isScrolling = Math.abs(diffScrollDir) > Math.abs(diffMainDir);
      }

      if (this.isScrolling) {
        this.setState({
          index: -1
        });
        return;
      }

      pauseEvent(e);

      var diffPosition = this._getDiffPosition(position[0]);

      var newValue = this._getValueFromPosition(diffPosition);

      this._move(newValue);
    },
    _onKeyDown: function _onKeyDown(e) {
      if (e.ctrlKey || e.shiftKey || e.altKey) return;

      switch (e.key) {
        case "ArrowLeft":
        case "ArrowUp":
          e.preventDefault();
          return this._moveDownOneStep();

        case "ArrowRight":
        case "ArrowDown":
          e.preventDefault();
          return this._moveUpOneStep();

        case "Home":
          return this._move(this.props.min);

        case "End":
          return this._move(this.props.max);

        default:
          return;
      }
    },
    _moveUpOneStep: function _moveUpOneStep() {
      var oldValue = this.state.value[this.state.index];
      var newValue = oldValue + this.props.step;

      this._move(Math.min(newValue, this.props.max));
    },
    _moveDownOneStep: function _moveDownOneStep() {
      var oldValue = this.state.value[this.state.index];
      var newValue = oldValue - this.props.step;

      this._move(Math.max(newValue, this.props.min));
    },
    _getValueFromPosition: function _getValueFromPosition(position) {
      var diffValue = position / (this.state.sliderLength - this.state.handleSize) * (this.props.max - this.props.min);
      return this._trimAlignValue(this.state.startValue + diffValue);
    },
    _getDiffPosition: function _getDiffPosition(position) {
      var diffPosition = position - this.state.startPosition;
      if (this.props.invert) diffPosition *= -1;
      return diffPosition;
    },
    _move: function _move(newValue) {
      this.hasMoved = true;
      var props = this.props;
      var state = this.state;
      var index = state.index;
      var value = state.value;
      var length = value.length;
      var oldValue = value[index];
      var minDistance = props.minDistance; // if "pearling" (= handles pushing each other) is disabled,
      // prevent the handle from getting closer than `minDistance` to the previous or next handle.

      if (!props.pearling) {
        if (index > 0) {
          var valueBefore = value[index - 1];

          if (newValue < valueBefore + minDistance) {
            newValue = valueBefore + minDistance;
          }
        }

        if (index < length - 1) {
          var valueAfter = value[index + 1];

          if (newValue > valueAfter - minDistance) {
            newValue = valueAfter - minDistance;
          }
        }
      }

      value[index] = newValue; // if "pearling" is enabled, let the current handle push the pre- and succeeding handles.

      if (props.pearling && length > 1) {
        if (newValue > oldValue) {
          this._pushSucceeding(value, minDistance, index);

          this._trimSucceeding(length, value, minDistance, props.max);
        } else if (newValue < oldValue) {
          this._pushPreceding(value, minDistance, index);

          this._trimPreceding(length, value, minDistance, props.min);
        }
      } // Normally you would use `shouldComponentUpdate`, but since the slider is a low-level component,
      // the extra complexity might be worth the extra performance.


      if (newValue !== oldValue) {
        this.setState({
          value: value
        }, this._fireChangeEvent.bind(this, 'onChange'));
      }
    },
    _pushSucceeding: function _pushSucceeding(value, minDistance, index) {
      var i, padding;

      for (i = index, padding = value[i] + minDistance; value[i + 1] != null && padding > value[i + 1]; i++, padding = value[i] + minDistance) {
        value[i + 1] = this._alignValue(padding);
      }
    },
    _trimSucceeding: function _trimSucceeding(length, nextValue, minDistance, max) {
      for (var i = 0; i < length; i++) {
        var padding = max - i * minDistance;

        if (nextValue[length - 1 - i] > padding) {
          nextValue[length - 1 - i] = padding;
        }
      }
    },
    _pushPreceding: function _pushPreceding(value, minDistance, index) {
      var i, padding;

      for (i = index, padding = value[i] - minDistance; value[i - 1] != null && padding < value[i - 1]; i--, padding = value[i] - minDistance) {
        value[i - 1] = this._alignValue(padding);
      }
    },
    _trimPreceding: function _trimPreceding(length, nextValue, minDistance, min) {
      for (var i = 0; i < length; i++) {
        var padding = min + i * minDistance;

        if (nextValue[i] < padding) {
          nextValue[i] = padding;
        }
      }
    },
    _axisKey: function _axisKey() {
      var orientation = this.props.orientation;
      if (orientation === 'horizontal') return 'X';
      if (orientation === 'vertical') return 'Y';
    },
    _orthogonalAxisKey: function _orthogonalAxisKey() {
      var orientation = this.props.orientation;
      if (orientation === 'horizontal') return 'Y';
      if (orientation === 'vertical') return 'X';
    },
    _posMinKey: function _posMinKey() {
      var orientation = this.props.orientation;
      if (orientation === 'horizontal') return this.props.invert ? 'right' : 'left';
      if (orientation === 'vertical') return this.props.invert ? 'bottom' : 'top';
    },
    _posMaxKey: function _posMaxKey() {
      var orientation = this.props.orientation;
      if (orientation === 'horizontal') return this.props.invert ? 'left' : 'right';
      if (orientation === 'vertical') return this.props.invert ? 'top' : 'bottom';
    },
    _sizeKey: function _sizeKey() {
      var orientation = this.props.orientation;
      if (orientation === 'horizontal') return 'clientWidth';
      if (orientation === 'vertical') return 'clientHeight';
    },
    _trimAlignValue: function _trimAlignValue(val, props) {
      return this._alignValue(this._trimValue(val, props), props);
    },
    _trimValue: function _trimValue(val, props) {
      props = props || this.props;
      if (val <= props.min) val = props.min;
      if (val >= props.max) val = props.max;
      return val;
    },
    _alignValue: function _alignValue(val, props) {
      props = props || this.props;
      var valModStep = (val - props.min) % props.step;
      var alignValue = val - valModStep;

      if (Math.abs(valModStep) * 2 >= props.step) {
        alignValue += valModStep > 0 ? props.step : -props.step;
      }

      return parseFloat(alignValue.toFixed(5));
    },
    _renderHandle: function _renderHandle(style, child, i) {
      var self = this;
      var className = this.props.handleClassName + ' ' + (this.props.handleClassName + '-' + i) + ' ' + (this.state.index === i ? this.props.handleActiveClassName : '');
      return React.createElement('div', {
        ref: function ref(r) {
          self['handle' + i] = r;
        },
        key: 'handle' + i,
        className: className,
        style: style,
        onMouseDown: this._createOnMouseDown(i),
        onTouchStart: this._createOnTouchStart(i),
        onFocus: this._createOnKeyDown(i),
        tabIndex: 0,
        role: "slider",
        "aria-valuenow": this.state.value[i],
        "aria-valuemin": this.props.min,
        "aria-valuemax": this.props.max,
        "aria-label": isArray(this.props.ariaLabel) ? this.props.ariaLabel[i] : this.props.ariaLabel,
        "aria-valuetext": this.props.ariaValuetext
      }, child);
    },
    _renderHandles: function _renderHandles(offset) {
      var length = offset.length;
      var styles = this.tempArray;

      for (var i = 0; i < length; i++) {
        styles[i] = this._buildHandleStyle(offset[i], i);
      }

      var res = [];
      var renderHandle = this._renderHandle;

      if (React.Children.count(this.props.children) > 0) {
        React.Children.forEach(this.props.children, function (child, i) {
          res[i] = renderHandle(styles[i], child, i);
        });
      } else {
        for (i = 0; i < length; i++) {
          res[i] = renderHandle(styles[i], null, i);
        }
      }

      return res;
    },
    _renderBar: function _renderBar(i, offsetFrom, offsetTo) {
      var self = this;
      return React.createElement('div', {
        key: 'bar' + i,
        ref: function ref(r) {
          self['bar' + i] = r;
        },
        className: this.props.barClassName + ' ' + this.props.barClassName + '-' + i,
        style: this._buildBarStyle(offsetFrom, this.state.upperBound - offsetTo)
      });
    },
    _renderValueComponent: function _renderValueComponent() {
      var _this$props = this.props,
          valueComponent = _this$props.valueComponent,
          max = _this$props.max,
          min = _this$props.min,
          value = _this$props.value;

      if (React.isValidElement(valueComponent)) {
        var _this$state = this.state,
            handleSize = _this$state.handleSize,
            upperBound = _this$state.upperBound,
            sliderLength = _this$state.sliderLength;
        var newProps = {
          handleSize: handleSize,
          upperBound: upperBound,
          max: max,
          min: min,
          value: value,
          sliderLength: sliderLength
        };
        return React.cloneElement(valueComponent, newProps);
      }

      return null;
    },
    _renderBars: function _renderBars(offset) {
      var bars = [];
      var lastIndex = offset.length - 1;
      bars.push(this._renderBar(0, 0, offset[0]));

      for (var i = 0; i < lastIndex; i++) {
        bars.push(this._renderBar(i + 1, offset[i], offset[i + 1]));
      }

      bars.push(this._renderBar(lastIndex + 1, offset[lastIndex], this.state.upperBound));
      return bars;
    },
    _onSliderMouseDown: function _onSliderMouseDown(e) {
      if (this.props.disabled) return;
      this.hasMoved = false;

      if (!this.props.snapDragDisabled) {
        var position = this._getMousePosition(e);

        this._forceValueFromPosition(position[0], function (i) {
          this._start(i, position[0]);

          this._fireChangeEvent('onChange');

          this._addHandlers(this._getMouseEventMap());
        }.bind(this));
      }

      pauseEvent(e);
    },
    _onSliderClick: function _onSliderClick(e) {
      if (this.props.disabled) return;

      if (this.props.onSliderClick && !this.hasMoved) {
        var position = this._getMousePosition(e);

        var valueAtPos = this._trimAlignValue(this._calcValue(this._calcOffsetFromPosition(position[0])));

        this.props.onSliderClick(valueAtPos);
      }
    },
    _fireChangeEvent: function _fireChangeEvent(event) {
      if (this.props[event]) {
        this.props[event](undoEnsureArray(this.state.value));
      }
    },
    render: function render() {
      var self = this;
      var state = this.state;
      var props = this.props;
      var offset = this.tempArray;
      var value = state.value;
      var l = value.length;

      for (var i = 0; i < l; i++) {
        offset[i] = this._calcOffset(value[i], i);
      }

      var bars = props.withBars ? this._renderBars(offset) : null;

      var handles = this._renderHandles(offset);

      var $values = this._renderValueComponent();

      return React.createElement('div', {
        ref: function ref(r) {
          self.slider = r;
        },
        style: {
          position: 'relative'
        },
        className: props.className + (props.disabled ? ' disabled' : ''),
        onMouseDown: this._onSliderMouseDown,
        onClick: this._onSliderClick
      }, bars, $values, handles);
    }
  });
  return ReactSlider;
});

/***/ }),

/***/ "uxHf":
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";

// EXTERNAL MODULE: /web-apps/node_modules/react/index.js
var react = __webpack_require__("ERkP");
var react_default = /*#__PURE__*/__webpack_require__.n(react);

// EXTERNAL MODULE: ../design/src/index.js + 20 modules
var design_src = __webpack_require__("+HtU");

// EXTERNAL MODULE: ../shared/components/Validation/index.js + 1 modules
var Validation = __webpack_require__("Bvo6");

// EXTERNAL MODULE: ../design/src/CardError/index.js + 1 modules
var CardError = __webpack_require__("rMID");

// EXTERNAL MODULE: /web-apps/node_modules/jquery/dist/jquery.js
var jquery = __webpack_require__("GtyH");
var jquery_default = /*#__PURE__*/__webpack_require__.n(jquery);

// EXTERNAL MODULE: ./src/services/api.js
var api = __webpack_require__("dCQc");

// EXTERNAL MODULE: ./src/config.js
var src_config = __webpack_require__("20nU");

// EXTERNAL MODULE: ./src/services/applications/index.js + 3 modules
var applications = __webpack_require__("FFWk");

// EXTERNAL MODULE: ./src/services/operations/index.js + 3 modules
var services_operations = __webpack_require__("FYaq");

// EXTERNAL MODULE: /web-apps/node_modules/lodash/lodash.js
var lodash = __webpack_require__("nsO7");

// CONCATENATED MODULE: ./src/installer/services/makeFlavors.js
function _slicedToArray(arr, i) { return _arrayWithHoles(arr) || _iterableToArrayLimit(arr, i) || _nonIterableRest(); }

function _nonIterableRest() { throw new TypeError("Invalid attempt to destructure non-iterable instance"); }

function _iterableToArrayLimit(arr, i) { if (!(Symbol.iterator in Object(arr) || Object.prototype.toString.call(arr) === "[object Arguments]")) { return; } var _arr = []; var _n = true; var _d = false; var _e = undefined; try { for (var _i = arr[Symbol.iterator](), _s; !(_n = (_s = _i.next()).done); _n = true) { _arr.push(_s.value); if (i && _arr.length === i) break; } } catch (err) { _d = true; _e = err; } finally { try { if (!_n && _i["return"] != null) _i["return"](); } finally { if (_d) throw _e; } } return _arr; }

function _arrayWithHoles(arr) { if (Array.isArray(arr)) return arr; }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

function makeFlavors(flavorsJson, app, operation) {
  // create a map from node profile array
  var nodeProfiles = Object(lodash["keyBy"])(app.nodeProfiles, function (i) {
    return i.name;
  });

  var _at = Object(lodash["at"])(flavorsJson, ['items', 'default', 'title']),
      _at2 = _slicedToArray(_at, 3),
      flavorItems = _at2[0],
      defaultFlavor = _at2[1],
      prompt = _at2[2]; // flavor options


  var options = Object(lodash["map"])(flavorItems, function (item) {
    var description = item.description,
        name = item.name,
        flavorProfiles = item.profiles; // create new profile objects from node profiles, operation agent, and flavor profiles

    var profiles = Object(lodash["keys"])(flavorProfiles).map(function (key) {
      var _flavorProfiles$key = flavorProfiles[key],
          instance_types = _flavorProfiles$key.instance_types,
          instance_type = _flavorProfiles$key.instance_type,
          count = _flavorProfiles$key.count;
      var _nodeProfiles$key = nodeProfiles[key],
          description = _nodeProfiles$key.description,
          requirementsText = _nodeProfiles$key.requirementsText;
      var instructions = Object(lodash["at"])(operation, "details.agents.".concat(key, ".instructions"))[0];
      return {
        name: key,
        instanceTypes: instance_types,
        instanceTypeFixed: instance_type,
        instructions: instructions,
        count: count,
        description: description,
        requirementsText: requirementsText
      };
    });
    return {
      name: name,
      isDefault: name === defaultFlavor,
      title: description || name,
      profiles: profiles
    };
  });
  return {
    options: options,
    prompt: prompt
  };
}
// EXTERNAL MODULE: ./src/services/enums.js
var enums = __webpack_require__("6eIW");

// CONCATENATED MODULE: ./src/installer/services/makeAgentServer.js
function _toConsumableArray(arr) { return _arrayWithoutHoles(arr) || _iterableToArray(arr) || _nonIterableSpread(); }

function _nonIterableSpread() { throw new TypeError("Invalid attempt to spread non-iterable instance"); }

function _iterableToArray(iter) { if (Symbol.iterator in Object(iter) || Object.prototype.toString.call(iter) === "[object Arguments]") return Array.from(iter); }

function _arrayWithoutHoles(arr) { if (Array.isArray(arr)) { for (var i = 0, arr2 = new Array(arr.length); i < arr.length; i++) { arr2[i] = arr[i]; } return arr2; } }

function makeAgentServer_slicedToArray(arr, i) { return makeAgentServer_arrayWithHoles(arr) || makeAgentServer_iterableToArrayLimit(arr, i) || makeAgentServer_nonIterableRest(); }

function makeAgentServer_nonIterableRest() { throw new TypeError("Invalid attempt to destructure non-iterable instance"); }

function makeAgentServer_iterableToArrayLimit(arr, i) { if (!(Symbol.iterator in Object(arr) || Object.prototype.toString.call(arr) === "[object Arguments]")) { return; } var _arr = []; var _n = true; var _d = false; var _e = undefined; try { for (var _i = arr[Symbol.iterator](), _s; !(_n = (_s = _i.next()).done); _n = true) { _arr.push(_s.value); if (i && _arr.length === i) break; } } catch (err) { _d = true; _e = err; } finally { try { if (!_n && _i["return"] != null) _i["return"](); } finally { if (_d) throw _e; } } return _arr; }

function makeAgentServer_arrayWithHoles(arr) { if (Array.isArray(arr)) return arr; }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/


function makeAgentServers(json) {
  var _at = Object(lodash["at"])(json, ['servers']),
      _at2 = makeAgentServer_slicedToArray(_at, 1),
      servers = _at2[0];

  var agentServers = Object(lodash["map"])(servers, function (srv) {
    var mountVars = makeMountVars(srv);
    var interfaceVars = makeInterfaceVars(srv);
    var vars = [].concat(_toConsumableArray(interfaceVars), _toConsumableArray(mountVars));
    return {
      role: srv.role,
      hostname: srv.hostname,
      vars: vars,
      os: srv.os
    };
  });
  return Object(lodash["sortBy"])(agentServers, function (s) {
    return s.hostname;
  });
}

function makeMountVars(json) {
  var mounts = Object(lodash["map"])(json.mounts, function (mnt) {
    return {
      name: mnt.name,
      type: enums["l" /* ServerVarEnums */].MOUNT,
      value: mnt.source,
      options: []
    };
  });
  return Object(lodash["sortBy"])(mounts, function (m) {
    return m.name;
  });
}

function makeInterfaceVars(json) {
  var defaultValue = json['advertise_addr'];
  var options = Object(lodash["values"])(json.interfaces).map(function (value) {
    return value['ipv4_addr'];
  }).sort();
  return [{
    type: enums["l" /* ServerVarEnums */].INTERFACE,
    value: defaultValue || options[0],
    options: options
  }];
}
// CONCATENATED MODULE: ./src/installer/services/installer.js
/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/







var installer_service = {
  fetchAgentReport: function fetchAgentReport(_ref) {
    var siteId = _ref.siteId,
        opId = _ref.opId;
    return api["b" /* default */].get(src_config["a" /* default */].getOperationAgentUrl(siteId, opId)).then(function (data) {
      return makeAgentServers(data);
    });
  },
  verifyOnPrem: function verifyOnPrem(request) {
    var siteId = request.siteId,
        opId = request.opId;
    return api["b" /* default */].post(src_config["a" /* default */].operationPrecheckPath(siteId, opId), request);
  },
  startInstall: function startInstall(request) {
    var siteId = request.siteId,
        opId = request.opId;
    return api["b" /* default */].post(src_config["a" /* default */].getOperationStartUrl(siteId, opId), request);
  },
  fetchClusterDetails: function fetchClusterDetails(siteId) {
    return jquery_default.a.when( // fetch operation
    services_operations["c" /* default */].fetchOps(siteId), // fetch cluster app
    installer_service.fetchClusterApp(siteId), // fetch flavors
    api["b" /* default */].get(src_config["a" /* default */].getSiteFlavorsUrl(siteId))).then(function () {
      for (var _len = arguments.length, responses = new Array(_len), _key = 0; _key < _len; _key++) {
        responses[_key] = arguments[_key];
      }

      var operations = responses[0],
          app = responses[1],
          flavorsJson = responses[2];
      var operation = operations.find(function (o) {
        return o.type === services_operations["a" /* OpTypeEnum */].OPERATION_INSTALL;
      });
      var flavors = makeFlavors(flavorsJson, app, operation);
      return {
        app: app,
        flavors: flavors,
        operation: operation
      };
    });
  },
  createCluster: function createCluster(request) {
    return installer_service.verifyClusterName(request.domain_name).then(function () {
      var url = src_config["a" /* default */].getSiteUrl({});
      return api["b" /* default */].post(url, request).then(function (json) {
        return json.site_domain;
      });
    });
  },
  setDeploymentType: function setDeploymentType(license, app_package) {
    var request = {
      license: license,
      app_package: app_package
    };
    return api["b" /* default */].post(src_config["a" /* default */].api.licenseValidationPath, request).then(function () {
      return license;
    });
  },
  verifyClusterName: function verifyClusterName(name) {
    return api["b" /* default */].get(src_config["a" /* default */].getCheckDomainNameUrl(name)).then(function (data) {
      data = data || [];

      if (data.length > 0) {
        return jquery_default.a.Deferred().reject(new Error("Cluster \"".concat(name, "\" already exists")));
      }
    });
  },
  fetchApp: function fetchApp() {
    return applications["a" /* default */].fetchApplication.apply(applications["a" /* default */], arguments);
  },
  fetchClusterApp: function fetchClusterApp(siteId) {
    return api["b" /* default */].get(src_config["a" /* default */].getSiteUrl({
      siteId: siteId,
      shallow: false
    })).then(function (json) {
      return Object(applications["b" /* makeApplication */])(json.app);
    });
  }
};
/* harmony default export */ var installer = (installer_service);
// CONCATENATED MODULE: ./src/installer/services/index.js
/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/



var ServiceContext = react_default.a.createContext(installer);

function Provider(_ref) {
  var _ref$value = _ref.value,
      value = _ref$value === void 0 ? installer : _ref$value,
      children = _ref.children;
  return react_default.a.createElement(ServiceContext.Provider, {
    value: value,
    children: children
  });
}

function useServices() {
  return react_default.a.useContext(ServiceContext);
}

/* harmony default export */ var services = (installer);

// EXTERNAL MODULE: /web-apps/node_modules/styled-components/dist/styled-components.browser.esm.js
var styled_components_browser_esm = __webpack_require__("j/s1");

// CONCATENATED MODULE: ./src/installer/components/Layout.jsx
function _extends() { _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; }; return _extends.apply(this, arguments); }

function _objectWithoutProperties(source, excluded) { if (source == null) return {}; var target = _objectWithoutPropertiesLoose(source, excluded); var key, i; if (Object.getOwnPropertySymbols) { var sourceSymbolKeys = Object.getOwnPropertySymbols(source); for (i = 0; i < sourceSymbolKeys.length; i++) { key = sourceSymbolKeys[i]; if (excluded.indexOf(key) >= 0) continue; if (!Object.prototype.propertyIsEnumerable.call(source, key)) continue; target[key] = source[key]; } } return target; }

function _objectWithoutPropertiesLoose(source, excluded) { if (source == null) return {}; var target = {}; var sourceKeys = Object.keys(source); var key, i; for (i = 0; i < sourceKeys.length; i++) { key = sourceKeys[i]; if (excluded.indexOf(key) >= 0) continue; target[key] = source[key]; } return target; }

function _templateObject() {
  var data = _taggedTemplateLiteral(["\n  position: absolute;\n  margin: 0 auto;\n  width: 100%;\n  height: 100%;\n  color: ", ";\n"]);

  _templateObject = function _templateObject() {
    return data;
  };

  return data;
}

function _taggedTemplateLiteral(strings, raw) { if (!raw) { raw = strings.slice(0); } return Object.freeze(Object.defineProperties(strings, { raw: { value: Object.freeze(raw) } })); }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/



var AppLayout = Object(styled_components_browser_esm["c" /* default */])(design_src["l" /* Flex */])(_templateObject(), function (_ref) {
  var theme = _ref.theme;
  return theme.colors.primary.contrastText;
});
function StepLayout(_ref2) {
  var title = _ref2.title,
      children = _ref2.children,
      styles = _objectWithoutProperties(_ref2, ["title", "children"]);

  return react_default.a.createElement(design_src["l" /* Flex */], _extends({
    flexDirection: "column"
  }, styles), title && react_default.a.createElement(design_src["u" /* Text */], {
    mb: "4",
    typography: "h1",
    style: {
      flexShrink: "0"
    }
  }, " ", title, " "), children);
}
// EXTERNAL MODULE: /web-apps/node_modules/prop-types/index.js
var prop_types = __webpack_require__("aWzz");
var prop_types_default = /*#__PURE__*/__webpack_require__.n(prop_types);

// EXTERNAL MODULE: ../design/src/system/index.js + 2 modules
var system = __webpack_require__("xM/J");

// CONCATENATED MODULE: ./src/components/CheckBox/CheckBox.jsx
function CheckBox_templateObject() {
  var data = CheckBox_taggedTemplateLiteral(["\n  display: inline-flex;\n  align-items: center;\n  position: relative;\n  padding-left: 30px;\n  margin-bottom: 12px;\n  cursor: pointer;\n  user-select: none;\n\n  /* Hide the browser's default checkbox */\n  input {\n    position: absolute;\n    opacity: 0;\n    cursor: pointer;\n    height: 0;\n    width: 0;\n  }\n\n  /* Create a custom checkbox */\n  .checkmark {\n    position: absolute;\n    top: 0;\n    left: 0;\n    height: 20px;\n    width: 20px;\n    background-color: #eee;\n  }\n\n  /* On mouse-over, add a grey background color */\n  &:hover input ~ .checkmark {\n    background-color: #ccc;\n  }\n\n  /* When the checkbox is checked, add a blue background */\n  input:checked ~ .checkmark {\n    background-color: #2196F3;\n  }\n\n  /* Create the checkmark/indicator (hidden when not checked) */\n  .checkmark:after {\n    content: \"\";\n    position: absolute;\n    display: none;\n  }\n\n  /* Show the checkmark when checked */\n  input:checked ~ .checkmark:after {\n    display: block;\n  }\n\n  /* Style the checkmark/indicator */\n  .checkmark:after {\n    left: 6px;\n    top: 2px;\n    width: 5px;\n    height: 10px;\n    border: solid white;\n    border-width: 0 3px 3px 0;\n    transform: rotate(45deg);\n  }\n  ", "\n  ", "\n"]);

  CheckBox_templateObject = function _templateObject() {
    return data;
  };

  return data;
}

function CheckBox_taggedTemplateLiteral(strings, raw) { if (!raw) { raw = strings.slice(0); } return Object.freeze(Object.defineProperties(strings, { raw: { value: Object.freeze(raw) } })); }

function CheckBox_objectWithoutProperties(source, excluded) { if (source == null) return {}; var target = CheckBox_objectWithoutPropertiesLoose(source, excluded); var key, i; if (Object.getOwnPropertySymbols) { var sourceSymbolKeys = Object.getOwnPropertySymbols(source); for (i = 0; i < sourceSymbolKeys.length; i++) { key = sourceSymbolKeys[i]; if (excluded.indexOf(key) >= 0) continue; if (!Object.prototype.propertyIsEnumerable.call(source, key)) continue; target[key] = source[key]; } } return target; }

function CheckBox_objectWithoutPropertiesLoose(source, excluded) { if (source == null) return {}; var target = {}; var sourceKeys = Object.keys(source); var key, i; for (i = 0; i < sourceKeys.length; i++) { key = sourceKeys[i]; if (excluded.indexOf(key) >= 0) continue; target[key] = source[key]; } return target; }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/



function Checkbox(_ref) {
  var value = _ref.value,
      onChange = _ref.onChange,
      label = _ref.label,
      styles = CheckBox_objectWithoutProperties(_ref, ["value", "onChange", "label"]);

  function onClick() {
    onChange(!value);
  }

  return react_default.a.createElement(StyledLabel, styles, label, react_default.a.createElement("input", {
    type: "checkbox",
    checked: value === true,
    onChange: onClick
  }), react_default.a.createElement("span", {
    className: "checkmark"
  }));
}
var StyledLabel = styled_components_browser_esm["c" /* default */].label(CheckBox_templateObject(), system["u" /* space */], system["f" /* color */]);
// CONCATENATED MODULE: ./src/components/CheckBox/index.js
/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/* harmony default export */ var CheckBox = (Checkbox);
// EXTERNAL MODULE: ../design/src/Image/index.js + 1 modules
var Image = __webpack_require__("GXDk");

// EXTERNAL MODULE: ../design/src/assets/images/gravity-logo.svg
var gravity_logo = __webpack_require__("GGeb");
var gravity_logo_default = /*#__PURE__*/__webpack_require__.n(gravity_logo);

// CONCATENATED MODULE: ./src/installer/components/Logo.jsx
function Logo_extends() { Logo_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; }; return Logo_extends.apply(this, arguments); }

function Logo_objectWithoutProperties(source, excluded) { if (source == null) return {}; var target = Logo_objectWithoutPropertiesLoose(source, excluded); var key, i; if (Object.getOwnPropertySymbols) { var sourceSymbolKeys = Object.getOwnPropertySymbols(source); for (i = 0; i < sourceSymbolKeys.length; i++) { key = sourceSymbolKeys[i]; if (excluded.indexOf(key) >= 0) continue; if (!Object.prototype.propertyIsEnumerable.call(source, key)) continue; target[key] = source[key]; } } return target; }

function Logo_objectWithoutPropertiesLoose(source, excluded) { if (source == null) return {}; var target = {}; var sourceKeys = Object.keys(source); var key, i; for (i = 0; i < sourceKeys.length; i++) { key = sourceKeys[i]; if (excluded.indexOf(key) >= 0) continue; target[key] = source[key]; } return target; }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/



function Logo(_ref) {
  var src = _ref.src,
      rest = Logo_objectWithoutProperties(_ref, ["src"]);

  var logoSrc = src || gravity_logo_default.a;
  return react_default.a.createElement(Image["a" /* default */], Logo_extends({
    mr: "8",
    my: "3",
    width: "auto",
    maxWidth: "120px",
    maxHeight: "40px",
    src: logoSrc
  }, rest));
}
// CONCATENATED MODULE: ./src/installer/components/Eula/Eula.jsx
function Eula_templateObject() {
  var data = Eula_taggedTemplateLiteral(["\n  border-radius: 6px;\n  min-height: 200px;\n  overflow: auto;\n  white-space: pre;\n  word-break: break-all;\n  word-wrap: break-word;\n"]);

  Eula_templateObject = function _templateObject() {
    return data;
  };

  return data;
}

function Eula_taggedTemplateLiteral(strings, raw) { if (!raw) { raw = strings.slice(0); } return Object.freeze(Object.defineProperties(strings, { raw: { value: Object.freeze(raw) } })); }

function Eula_slicedToArray(arr, i) { return Eula_arrayWithHoles(arr) || Eula_iterableToArrayLimit(arr, i) || Eula_nonIterableRest(); }

function Eula_nonIterableRest() { throw new TypeError("Invalid attempt to destructure non-iterable instance"); }

function Eula_iterableToArrayLimit(arr, i) { if (!(Symbol.iterator in Object(arr) || Object.prototype.toString.call(arr) === "[object Arguments]")) { return; } var _arr = []; var _n = true; var _d = false; var _e = undefined; try { for (var _i = arr[Symbol.iterator](), _s; !(_n = (_s = _i.next()).done); _n = true) { _arr.push(_s.value); if (i && _arr.length === i) break; } } catch (err) { _d = true; _e = err; } finally { try { if (!_n && _i["return"] != null) _i["return"](); } finally { if (_d) throw _e; } } return _arr; }

function Eula_arrayWithHoles(arr) { if (Array.isArray(arr)) return arr; }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/







function Eula(props) {
  var onAccept = props.onAccept,
      app = props.app,
      config = props.config;
  var eula = app.eula,
      displayName = app.displayName,
      logo = app.logo;
  var eulaAgreeText = config.eulaAgreeText,
      eulaHeaderText = config.eulaHeaderText,
      eulaContentLabelText = config.eulaContentLabelText;
  var headerText = eulaHeaderText.replace('{0}', displayName);

  var _React$useState = react_default.a.useState(false),
      _React$useState2 = Eula_slicedToArray(_React$useState, 2),
      accepted = _React$useState2[0],
      setAccepted = _React$useState2[1];

  function onToggleAccepted(value) {
    setAccepted(value);
  }

  return react_default.a.createElement(AppLayout, {
    flexDirection: "column",
    px: "40px",
    py: "40px"
  }, react_default.a.createElement(design_src["l" /* Flex */], {
    alignItems: "center",
    mb: "8"
  }, react_default.a.createElement(Logo, {
    src: logo
  }), react_default.a.createElement(design_src["u" /* Text */], {
    typography: "h2"
  }, " ", headerText, " ")), react_default.a.createElement(StepLayout, {
    title: eulaContentLabelText,
    overflow: "auto"
  }, react_default.a.createElement(StyledAgreement, {
    flex: "1",
    px: "2",
    py: "2",
    mb: "4",
    as: design_src["l" /* Flex */],
    typography: "body2",
    mono: true,
    bg: "light",
    color: "text.onLight"
  }, eula), react_default.a.createElement(CheckBox, {
    mb: "10",
    value: accepted,
    onChange: onToggleAccepted,
    label: eulaAgreeText
  }), react_default.a.createElement(design_src["f" /* ButtonPrimary */], {
    width: "200px",
    onClick: onAccept,
    disabled: !accepted
  }, "Accept AGREEMENT")));
}
Eula.propTypes = {
  onAccept: prop_types_default.a.func.isRequired,
  app: prop_types_default.a.object.isRequired,
  config: prop_types_default.a.object.isRequired
};
var StyledAgreement = Object(styled_components_browser_esm["c" /* default */])(design_src["u" /* Text */])(Eula_templateObject());
// CONCATENATED MODULE: ./src/installer/components/Eula/index.js
/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/* harmony default export */ var components_Eula = (Eula);
// EXTERNAL MODULE: ./src/lib/stores/index.js + 3 modules
var stores = __webpack_require__("HRSe");

// CONCATENATED MODULE: ./src/installer/components/store.js
function _typeof(obj) { if (typeof Symbol === "function" && typeof Symbol.iterator === "symbol") { _typeof = function _typeof(obj) { return typeof obj; }; } else { _typeof = function _typeof(obj) { return obj && typeof Symbol === "function" && obj.constructor === Symbol && obj !== Symbol.prototype ? "symbol" : typeof obj; }; } return _typeof(obj); }

function store_slicedToArray(arr, i) { return store_arrayWithHoles(arr) || store_iterableToArrayLimit(arr, i) || store_nonIterableRest(); }

function store_nonIterableRest() { throw new TypeError("Invalid attempt to destructure non-iterable instance"); }

function store_iterableToArrayLimit(arr, i) { if (!(Symbol.iterator in Object(arr) || Object.prototype.toString.call(arr) === "[object Arguments]")) { return; } var _arr = []; var _n = true; var _d = false; var _e = undefined; try { for (var _i = arr[Symbol.iterator](), _s; !(_n = (_s = _i.next()).done); _n = true) { _arr.push(_s.value); if (i && _arr.length === i) break; } } catch (err) { _d = true; _e = err; } finally { try { if (!_n && _i["return"] != null) _i["return"](); } finally { if (_d) throw _e; } } return _arr; }

function store_arrayWithHoles(arr) { if (Array.isArray(arr)) return arr; }

function ownKeys(object, enumerableOnly) { var keys = Object.keys(object); if (Object.getOwnPropertySymbols) { var symbols = Object.getOwnPropertySymbols(object); if (enumerableOnly) symbols = symbols.filter(function (sym) { return Object.getOwnPropertyDescriptor(object, sym).enumerable; }); keys.push.apply(keys, symbols); } return keys; }

function _objectSpread(target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i] != null ? arguments[i] : {}; if (i % 2) { ownKeys(source, true).forEach(function (key) { _defineProperty(target, key, source[key]); }); } else if (Object.getOwnPropertyDescriptors) { Object.defineProperties(target, Object.getOwnPropertyDescriptors(source)); } else { ownKeys(source).forEach(function (key) { Object.defineProperty(target, key, Object.getOwnPropertyDescriptor(source, key)); }); } } return target; }

function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function _defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } }

function _createClass(Constructor, protoProps, staticProps) { if (protoProps) _defineProperties(Constructor.prototype, protoProps); if (staticProps) _defineProperties(Constructor, staticProps); return Constructor; }

function _possibleConstructorReturn(self, call) { if (call && (_typeof(call) === "object" || typeof call === "function")) { return call; } return _assertThisInitialized(self); }

function _getPrototypeOf(o) { _getPrototypeOf = Object.setPrototypeOf ? Object.getPrototypeOf : function _getPrototypeOf(o) { return o.__proto__ || Object.getPrototypeOf(o); }; return _getPrototypeOf(o); }

function _assertThisInitialized(self) { if (self === void 0) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return self; }

function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function"); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, writable: true, configurable: true } }); if (superClass) _setPrototypeOf(subClass, superClass); }

function _setPrototypeOf(o, p) { _setPrototypeOf = Object.setPrototypeOf || function _setPrototypeOf(o, p) { o.__proto__ = p; return o; }; return _setPrototypeOf(o, p); }

function _defineProperty(obj, key, value) { if (key in obj) { Object.defineProperty(obj, key, { value: value, enumerable: true, configurable: true, writable: true }); } else { obj[key] = value; } return obj; }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/





var StepEnum = {
  LICENSE: 'license',
  NEW_APP: 'new_app',
  PROVISION: 'provision',
  PROGRESS: 'progress',
  USER: 'user'
};
var defaultServiceSubnet = '10.100.0.0/16';
var defaultPodSubnet = '10.244.0.0/16';

var store_InstallerStore =
/*#__PURE__*/
function (_Store) {
  _inherits(InstallerStore, _Store);

  function InstallerStore() {
    var _getPrototypeOf2;

    var _this;

    _classCallCheck(this, InstallerStore);

    for (var _len = arguments.length, args = new Array(_len), _key = 0; _key < _len; _key++) {
      args[_key] = arguments[_key];
    }

    _this = _possibleConstructorReturn(this, (_getPrototypeOf2 = _getPrototypeOf(InstallerStore)).call.apply(_getPrototypeOf2, [this].concat(args)));

    _defineProperty(_assertThisInitialized(_this), "state", {
      // Current installation step
      step: '',
      // Defined installation steps
      stepOptions: [],
      // License required for installation
      license: null,
      // Store status
      status: 'loading',
      // Installer config which has app custom installer settings
      config: Object(lodash["merge"])({}, src_config["a" /* default */].modules.installer),
      // Indicates of user accepted EULA agreement
      eulaAccepted: false,
      // Application data
      app: {},
      // Install operation data
      operation: null,
      // Cluster tags
      tags: {},
      // Entered cluster name
      clusterName: '',
      // Service subnet
      serviceSubnet: defaultServiceSubnet,
      // Pod subnet
      podSubnet: defaultPodSubnet,
      // Available app flavors
      flavors: null,
      // Parameters for selected flavor and connected servers
      provision: {
        profiles: {},
        servers: []
      },
      // Joined onprem servers
      agentServers: []
    });

    _defineProperty(_assertThisInitialized(_this), "acceptEula", function () {
      _this.setState({
        eulaAccepted: true
      });
    });

    return _this;
  }

  _createClass(InstallerStore, [{
    key: "setError",
    value: function setError(err) {
      this.setState({
        status: 'error',
        statusText: err.message
      });
    }
  }, {
    key: "setLicense",
    value: function setLicense(license) {
      this.setState({
        license: license,
        step: StepEnum.NEW_APP
      });
    }
  }, {
    key: "setClusterTags",
    value: function setClusterTags(tags) {
      this.setState({
        tags: _objectSpread({}, tags)
      });
    }
  }, {
    key: "setStepProgress",
    value: function setStepProgress() {
      this.setState({
        step: StepEnum.PROGRESS
      });
    }
  }, {
    key: "setOnpremSubnets",
    value: function setOnpremSubnets(serviceSubnet, podSubnet) {
      this.setState({
        serviceSubnet: serviceSubnet,
        podSubnet: podSubnet
      });
    }
  }, {
    key: "setClusterName",
    value: function setClusterName(clusterName) {
      this.setState({
        clusterName: clusterName
      });
    }
  }, {
    key: "makeOnpremRequest",
    value: function makeOnpremRequest() {
      var _this$state = this.state,
          clusterName = _this$state.clusterName,
          license = _this$state.license,
          tags = _this$state.tags,
          serviceSubnet = _this$state.serviceSubnet,
          podSubnet = _this$state.podSubnet;
      var packageId = this.state.app.packageId;
      var request = {
        app_package: packageId,
        domain_name: clusterName,
        provider: null,
        license: license,
        labels: tags
      };
      request.provider = _defineProperty({
        provisioner: enums["h" /* ProviderEnum */].ONPREM
      }, enums["h" /* ProviderEnum */].ONPREM, {
        pod_cidr: podSubnet,
        service_cidr: serviceSubnet
      });
      return request;
    }
  }, {
    key: "makeAgentRequest",
    value: function makeAgentRequest() {
      var _this$state$operation = this.state.operation,
          siteId = _this$state$operation.siteId,
          opId = _this$state$operation.id;
      return {
        siteId: siteId,
        opId: opId
      };
    }
  }, {
    key: "makeStartInstallRequest",
    value: function makeStartInstallRequest() {
      var _this2 = this;

      var request = {
        siteId: this.state.operation.siteId,
        opId: this.state.operation.id,
        profiles: {},
        servers: []
      };
      Object(lodash["keys"])(this.state.provision.profiles).forEach(function (key) {
        var _this2$state$provisio = _this2.state.provision.profiles[key],
            instanceType = _this2$state$provisio.instanceType,
            count = _this2$state$provisio.count;
        request.profiles[key] = {
          instance_type: instanceType,
          count: count
        };
      });
      var serverMap = this.state.provision.servers;
      Object(lodash["keys"])(serverMap).map(function (role) {
        Object(lodash["values"])(serverMap[role]).map(function (server) {
          var os = server.os;
          var role = server.role;
          var system_state = null;
          var advertise_ip = server.ip;
          var hostname = server.hostname;
          var mounts = Object(lodash["map"])(server.mounts, function (mount) {
            return {
              name: mount.name,
              source: mount.value
            };
          });
          request.servers.push({
            os: os,
            role: role,
            system_state: system_state,
            advertise_ip: advertise_ip,
            hostname: hostname,
            mounts: mounts
          });
        });
      });
      return request;
    }
  }, {
    key: "initWithApp",
    value: function initWithApp(app) {
      var step = StepEnum.LICENSE;
      var stepOptions = [{
        value: StepEnum.LICENSE,
        title: 'License'
      }, {
        value: StepEnum.NEW_APP,
        title: 'Cluster name'
      }, {
        value: StepEnum.PROVISION,
        title: 'Capacity'
      }, {
        value: StepEnum.PROGRESS,
        title: 'Installation'
      }, {
        value: StepEnum.USER,
        title: 'Create Admin'
      }]; // remove license step

      if (!app.licenseRequired) {
        stepOptions.shift();
        step = StepEnum.NEW_APP;
      } // remove bandwagon step


      if (app.bandwagon) {
        stepOptions.unshift();
      }

      var _at = Object(lodash["at"])(app, ['config.modules.installer', 'config.agentReport']),
          _at2 = store_slicedToArray(_at, 2),
          installerConfig = _at2[0],
          agentReportConfig = _at2[1]; // TODO: fixme
      // overrides default agent report config


      Object(lodash["merge"])(src_config["a" /* default */], {
        agentReportConfig: agentReportConfig
      });
      var config = Object(lodash["merge"])(this.state.config, installerConfig);
      this.setState({
        status: 'ready',
        stepOptions: stepOptions,
        app: app,
        step: step,
        config: config
      });
    }
  }, {
    key: "initWithCluster",
    value: function initWithCluster(details) {
      var app = details.app,
          operation = details.operation,
          flavors = details.flavors;
      var step = mapOpStateToStep(operation.state);
      this.initWithApp(app);
      this.setState({
        flavors: flavors,
        step: step,
        operation: operation,
        eulaAccepted: true
      });
    }
  }, {
    key: "setProvisionProfiles",
    value: function setProvisionProfiles(profiles) {
      var provisitProfiles = {};
      Object(lodash["forEach"])(profiles, function (p) {
        provisitProfiles[p.name] = {
          count: p.count
        };
      });

      var provision = _objectSpread({}, this.state.provision, {
        profiles: provisitProfiles
      });

      this.setState({
        provision: provision
      });
    }
  }, {
    key: "setAgentServers",
    value: function setAgentServers(agentServers) {
      this.setState({
        agentServers: agentServers
      });
    }
  }, {
    key: "setServerVars",
    value: function setServerVars(_ref) {
      var role = _ref.role,
          hostname = _ref.hostname,
          ip = _ref.ip,
          mounts = _ref.mounts;
      Object(lodash["set"])(this.state.provision, ['servers', role, hostname], {
        role: role,
        hostname: hostname,
        ip: ip,
        mounts: mounts
      });
      this.setState(_objectSpread({}, this.state.provision));
    }
  }, {
    key: "removeServerVars",
    value: function removeServerVars(_ref2) {
      var role = _ref2.role,
          hostname = _ref2.hostname;
      Object(lodash["unset"])(this.state.provision, ['servers', role, hostname]);
      this.setState(_objectSpread({}, this.state.provision));
    }
  }]);

  return InstallerStore;
}(stores["a" /* Store */]);


var installerContext = react_default.a.createContext({});

function mapOpStateToStep(state) {
  var step;

  switch (state) {
    case enums["f" /* OpStateEnum */].CREATED:
    case enums["f" /* OpStateEnum */].INSTALL_INITIATED:
    case enums["f" /* OpStateEnum */].INSTALL_PRECHECKS:
    case enums["f" /* OpStateEnum */].INSTALL_SETTING_CLUSTER_PLAN:
      step = StepEnum.PROVISION;
      break;

    default:
      step = StepEnum.PROGRESS;
  }

  return step;
}

var store_Provider = installerContext.Provider;
function useInstallerContext() {
  return react_default.a.useContext(installerContext);
}
function useInstallerStore() {
  var store = useInstallerContext();
  return Object(stores["b" /* useStore */])(store);
}
// EXTERNAL MODULE: ./src/components/nuclear.js
var nuclear = __webpack_require__("+Yr/");

// CONCATENATED MODULE: ./src/flux/opProgress/getters.js
/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
var progressById = function progressById(opId) {
  return [['opProgress', opId], function (progressMap) {
    if (!progressMap) {
      return null;
    }

    var siteId = progressMap.get('site_id');
    var state = progressMap.get('state');
    return {
      siteId: siteId,
      opId: opId,
      step: progressMap.get('step'),
      isProcessing: state === 'in_progress',
      isCompleted: state === 'completed',
      isError: state === 'failed',
      message: progressMap.get('message'),
      crashReportUrl: progressMap.get('crashReportUrl'),
      siteUrl: progressMap.get('siteUrl')
    };
  }];
};

/* harmony default export */ var getters = ({
  progressById: progressById
});
// EXTERNAL MODULE: ./src/components/LogViewer/index.js + 3 modules
var LogViewer = __webpack_require__("gnIC");

// EXTERNAL MODULE: ./src/components/AjaxPoller/index.js + 1 modules
var AjaxPoller = __webpack_require__("Q4Eh");

// EXTERNAL MODULE: ./src/reactor.js
var reactor = __webpack_require__("DEM/");

// EXTERNAL MODULE: ./src/flux/opProgress/actionTypes.js
var actionTypes = __webpack_require__("isOX");

// CONCATENATED MODULE: ./src/flux/opProgress/actions.js
/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/




function fetchOpProgress(siteId, opId) {
  var url = src_config["a" /* default */].getOperationProgressUrl(siteId, opId);
  return api["b" /* default */].get(url).then(function (data) {
    reactor["a" /* default */].dispatch(actionTypes["a" /* OP_PROGRESS_RECEIVE */], data);
  });
}
// EXTERNAL MODULE: /web-apps/node_modules/react-router/es/generatePath.js
var generatePath = __webpack_require__("LTrZ");

// CONCATENATED MODULE: ./src/installer/components/StepProgress/InstallLogsProvider/InstallLogsProvider.jsx
function InstallLogsProvider_typeof(obj) { if (typeof Symbol === "function" && typeof Symbol.iterator === "symbol") { InstallLogsProvider_typeof = function _typeof(obj) { return typeof obj; }; } else { InstallLogsProvider_typeof = function _typeof(obj) { return obj && typeof Symbol === "function" && obj.constructor === Symbol && obj !== Symbol.prototype ? "symbol" : typeof obj; }; } return InstallLogsProvider_typeof(obj); }

function InstallLogsProvider_classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function InstallLogsProvider_defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ("value" in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } }

function InstallLogsProvider_createClass(Constructor, protoProps, staticProps) { if (protoProps) InstallLogsProvider_defineProperties(Constructor.prototype, protoProps); if (staticProps) InstallLogsProvider_defineProperties(Constructor, staticProps); return Constructor; }

function InstallLogsProvider_possibleConstructorReturn(self, call) { if (call && (InstallLogsProvider_typeof(call) === "object" || typeof call === "function")) { return call; } return InstallLogsProvider_assertThisInitialized(self); }

function InstallLogsProvider_assertThisInitialized(self) { if (self === void 0) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return self; }

function InstallLogsProvider_getPrototypeOf(o) { InstallLogsProvider_getPrototypeOf = Object.setPrototypeOf ? Object.getPrototypeOf : function _getPrototypeOf(o) { return o.__proto__ || Object.getPrototypeOf(o); }; return InstallLogsProvider_getPrototypeOf(o); }

function InstallLogsProvider_inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function"); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, writable: true, configurable: true } }); if (superClass) InstallLogsProvider_setPrototypeOf(subClass, superClass); }

function InstallLogsProvider_setPrototypeOf(o, p) { InstallLogsProvider_setPrototypeOf = Object.setPrototypeOf || function _setPrototypeOf(o, p) { o.__proto__ = p; return o; }; return InstallLogsProvider_setPrototypeOf(o, p); }

function InstallLogsProvider_defineProperty(obj, key, value) { if (key in obj) { Object.defineProperty(obj, key, { value: value, enumerable: true, configurable: true, writable: true }); } else { obj[key] = value; } return obj; }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/






var InstallLogsProvider =
/*#__PURE__*/
function (_React$Component) {
  InstallLogsProvider_inherits(InstallLogsProvider, _React$Component);

  function InstallLogsProvider(props) {
    var _this;

    InstallLogsProvider_classCallCheck(this, InstallLogsProvider);

    _this = InstallLogsProvider_possibleConstructorReturn(this, InstallLogsProvider_getPrototypeOf(InstallLogsProvider).call(this, props));
    _this.socket = null;
    return _this;
  }

  InstallLogsProvider_createClass(InstallLogsProvider, [{
    key: "componentWillReceiveProps",
    value: function componentWillReceiveProps(nextProps) {
      var _this$props = this.props,
          siteId = _this$props.siteId,
          opId = _this$props.opId;

      if (nextProps.opId !== opId) {
        this.connect(siteId, nextProps.opId);
      }
    }
  }, {
    key: "componentDidMount",
    value: function componentDidMount() {
      var _this$props2 = this.props,
          siteId = _this$props2.siteId,
          opId = _this$props2.opId;
      this.connect(siteId, opId);
    }
  }, {
    key: "componentWillUnmount",
    value: function componentWillUnmount() {
      this.disconnect();
    }
  }, {
    key: "disconnect",
    value: function disconnect() {
      if (this.socket) {
        this.socket.close();
      }
    }
  }, {
    key: "onLoading",
    value: function onLoading(value) {
      if (this.props.onLoading) {
        this.props.onLoading(value);
      }
    }
  }, {
    key: "onError",
    value: function onError(err) {
      if (this.props.onError) {
        this.props.onError(err);
      }
    }
  }, {
    key: "onData",
    value: function onData(data) {
      if (this.props.onData) {
        this.props.onData(data.trim() + '\n');
      }
    }
  }, {
    key: "connect",
    value: function connect(siteId, opId) {
      var _this2 = this;

      this.disconnect();
      this.onLoading(true);
      this.socket = createLogStreamer(siteId, opId);

      this.socket.onopen = function () {
        _this2.onLoading(false);
      };

      this.socket.onerror = function () {
        _this2.onError();
      };

      this.socket.onclose = function () {};

      this.socket.onmessage = function (e) {
        _this2.onData(e.data);
      };
    }
  }, {
    key: "render",
    value: function render() {
      return null;
    }
  }]);

  return InstallLogsProvider;
}(react_default.a.Component);

InstallLogsProvider_defineProperty(InstallLogsProvider, "propTypes", {
  siteId: prop_types_default.a.string.isRequired,
  opId: prop_types_default.a.string.isRequired,
  onLoading: prop_types_default.a.func,
  onError: prop_types_default.a.func,
  onData: prop_types_default.a.func
});



function createLogStreamer(siteId, opId) {
  var token = Object(api["c" /* getAccessToken */])();
  var hostport = location.hostname + (location.port ? ':' + location.port : '');
  var hostname = "wss://".concat(hostport);
  var url = Object(generatePath["a" /* default */])(src_config["a" /* default */].api.operationLogsPath, {
    siteId: siteId,
    token: token,
    opId: opId
  });
  return new WebSocket(hostname + url);
}
// CONCATENATED MODULE: ./src/installer/components/StepProgress/InstallLogsProvider/index.js
/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/* harmony default export */ var StepProgress_InstallLogsProvider = (InstallLogsProvider);
// EXTERNAL MODULE: ../design/src/Icon/index.js
var Icon = __webpack_require__("+aPP");

// CONCATENATED MODULE: ./src/installer/components/StepProgress/TogglePanel.jsx
function TogglePanel_templateObject() {
  var data = TogglePanel_taggedTemplateLiteral(["\n  flex-grow: 0;\n  cursor: pointer;\n  // prevent text selection on accidental double click\n  -webkit-user-select: none;\n  -moz-user-select: none;\n  -khtml-user-select: none;\n  -ms-user-select: none;\n"]);

  TogglePanel_templateObject = function _templateObject() {
    return data;
  };

  return data;
}

function TogglePanel_taggedTemplateLiteral(strings, raw) { if (!raw) { raw = strings.slice(0); } return Object.freeze(Object.defineProperties(strings, { raw: { value: Object.freeze(raw) } })); }

function TogglePanel_extends() { TogglePanel_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; }; return TogglePanel_extends.apply(this, arguments); }

function TogglePanel_objectWithoutProperties(source, excluded) { if (source == null) return {}; var target = TogglePanel_objectWithoutPropertiesLoose(source, excluded); var key, i; if (Object.getOwnPropertySymbols) { var sourceSymbolKeys = Object.getOwnPropertySymbols(source); for (i = 0; i < sourceSymbolKeys.length; i++) { key = sourceSymbolKeys[i]; if (excluded.indexOf(key) >= 0) continue; if (!Object.prototype.propertyIsEnumerable.call(source, key)) continue; target[key] = source[key]; } } return target; }

function TogglePanel_objectWithoutPropertiesLoose(source, excluded) { if (source == null) return {}; var target = {}; var sourceKeys = Object.keys(source); var key, i; for (i = 0; i < sourceKeys.length; i++) { key = sourceKeys[i]; if (excluded.indexOf(key) >= 0) continue; target[key] = source[key]; } return target; }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/




function ExpandPanel(_ref) {
  var children = _ref.children,
      title = _ref.title,
      expanded = _ref.expanded,
      onToggle = _ref.onToggle,
      styles = TogglePanel_objectWithoutProperties(_ref, ["children", "title", "expanded", "onToggle"]);

  var IconCmpt = expanded ? Icon["b" /* ArrowUp */] : Icon["a" /* ArrowDown */];
  return react_default.a.createElement(design_src["l" /* Flex */], TogglePanel_extends({
    width: "100%",
    flexDirection: "column",
    bg: "primary.light"
  }, styles), react_default.a.createElement(StyledHeader, {
    height: "50px",
    pl: "3",
    pr: "2",
    py: "2",
    flex: "1",
    bg: "primary.main",
    alignItems: "center",
    justifyContent: "space-between",
    onClick: onToggle
  }, react_default.a.createElement(design_src["u" /* Text */], {
    typography: "subtitle1",
    caps: true
  }, title), react_default.a.createElement(design_src["c" /* ButtonIcon */], {
    onClick: onToggle
  }, react_default.a.createElement(IconCmpt, null))), children);
}
var StyledHeader = Object(styled_components_browser_esm["c" /* default */])(design_src["l" /* Flex */])(TogglePanel_templateObject());
// CONCATENATED MODULE: ./src/installer/components/StepProgress/ProgressBar.jsx
function ProgressBar_templateObject() {
  var data = ProgressBar_taggedTemplateLiteral(["\n  align-items: center;\n  flex-shrink: 0;\n\n  background-color: ", ";\n  border-radius: 12px;\n  > span {\n    border-radius: 12px;\n    ", "\n  }\n"]);

  ProgressBar_templateObject = function _templateObject() {
    return data;
  };

  return data;
}

function ProgressBar_taggedTemplateLiteral(strings, raw) { if (!raw) { raw = strings.slice(0); } return Object.freeze(Object.defineProperties(strings, { raw: { value: Object.freeze(raw) } })); }

function ProgressBar_extends() { ProgressBar_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; }; return ProgressBar_extends.apply(this, arguments); }

function ProgressBar_objectWithoutProperties(source, excluded) { if (source == null) return {}; var target = ProgressBar_objectWithoutPropertiesLoose(source, excluded); var key, i; if (Object.getOwnPropertySymbols) { var sourceSymbolKeys = Object.getOwnPropertySymbols(source); for (i = 0; i < sourceSymbolKeys.length; i++) { key = sourceSymbolKeys[i]; if (excluded.indexOf(key) >= 0) continue; if (!Object.prototype.propertyIsEnumerable.call(source, key)) continue; target[key] = source[key]; } } return target; }

function ProgressBar_objectWithoutPropertiesLoose(source, excluded) { if (source == null) return {}; var target = {}; var sourceKeys = Object.keys(source); var key, i; for (i = 0; i < sourceKeys.length; i++) { key = sourceKeys[i]; if (excluded.indexOf(key) >= 0) continue; target[key] = source[key]; } return target; }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/




function ProgressBar(props) {
  var _props$value = props.value,
      value = _props$value === void 0 ? 0 : _props$value,
      styles = ProgressBar_objectWithoutProperties(props, ["value"]);

  return react_default.a.createElement(StyledProgressBar, ProgressBar_extends({
    height: "18px"
  }, styles, {
    value: value,
    isCompleted: false
  }), react_default.a.createElement("span", null));
}
ProgressBar.propTypes = {
  value: prop_types_default.a.number.isRequired
};
var StyledProgressBar = Object(styled_components_browser_esm["c" /* default */])(design_src["l" /* Flex */])(ProgressBar_templateObject(), function (_ref) {
  var theme = _ref.theme;
  return theme.colors.light;
}, function (_ref2) {
  var theme = _ref2.theme,
      value = _ref2.value;
  return "\n      height: 100%;\n      width: ".concat(value, "%;\n      background-color: ").concat(theme.colors.success, ";\n    ");
});
// EXTERNAL MODULE: ../design/src/Icon/Icon.jsx
var Icon_Icon = __webpack_require__("oLQf");

// CONCATENATED MODULE: ./src/installer/components/StepProgress/ProgressDescription.jsx
function _templateObject2() {
  var data = ProgressDescription_taggedTemplateLiteral(["\n  ", "\n\n  animation: anim-rotate 2s infinite linear;\n  color: #fff;\n  display: inline-block;\n  opacity: .87;\n  text-shadow: 0 0 .25em rgba(255,255,255, .3);\n\n  @keyframes anim-rotate {\n    0% {\n      transform: rotate(0);\n    }\n    100% {\n      transform: rotate(360deg);\n    }\n  }\n"]);

  _templateObject2 = function _templateObject2() {
    return data;
  };

  return data;
}

function ProgressDescription_templateObject() {
  var data = ProgressDescription_taggedTemplateLiteral(["\n  border-right: 1px solid ", ";\n\n  &:last-child{\n    border-right: none;\n  }\n"]);

  ProgressDescription_templateObject = function _templateObject() {
    return data;
  };

  return data;
}

function ProgressDescription_taggedTemplateLiteral(strings, raw) { if (!raw) { raw = strings.slice(0); } return Object.freeze(Object.defineProperties(strings, { raw: { value: Object.freeze(raw) } })); }

function ProgressDescription_extends() { ProgressDescription_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; }; return ProgressDescription_extends.apply(this, arguments); }

function ProgressDescription_objectWithoutProperties(source, excluded) { if (source == null) return {}; var target = ProgressDescription_objectWithoutPropertiesLoose(source, excluded); var key, i; if (Object.getOwnPropertySymbols) { var sourceSymbolKeys = Object.getOwnPropertySymbols(source); for (i = 0; i < sourceSymbolKeys.length; i++) { key = sourceSymbolKeys[i]; if (excluded.indexOf(key) >= 0) continue; if (!Object.prototype.propertyIsEnumerable.call(source, key)) continue; target[key] = source[key]; } } return target; }

function ProgressDescription_objectWithoutPropertiesLoose(source, excluded) { if (source == null) return {}; var target = {}; var sourceKeys = Object.keys(source); var key, i; for (i = 0; i < sourceKeys.length; i++) { key = sourceKeys[i]; if (excluded.indexOf(key) >= 0) continue; target[key] = source[key]; } return target; }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/





function ProgressDescription(props) {
  var _props$step = props.step,
      step = _props$step === void 0 ? 0 : _props$step,
      _props$steps = props.steps,
      steps = _props$steps === void 0 ? [] : _props$steps,
      styles = ProgressDescription_objectWithoutProperties(props, ["step", "steps"]);

  var items = steps.map(function (name, index) {
    return {
      isCompleted: step > index,
      isProcessing: step === index,
      name: name
    };
  });
  var groupItems1 = items.slice(0, 3);
  var groupItems2 = items.slice(3, 6);
  var groupItems3 = items.slice(6, 10);
  return react_default.a.createElement(design_src["l" /* Flex */], ProgressDescription_extends({
    bg: "primary.light",
    justifyCofntent: "space-between"
  }, styles), react_default.a.createElement(Group, {
    IconComponent: Icon_Icon["Q" /* SettingsInputComposite */],
    title: "Gathering Instances",
    items: groupItems1
  }), react_default.a.createElement(Group, {
    IconComponent: Icon_Icon["u" /* Equalizer */],
    title: "Configure and install",
    items: groupItems2
  }), react_default.a.createElement(Group, {
    IconComponent: Icon_Icon["E" /* ListAddCheck */],
    title: "Finalizing install",
    items: groupItems3
  }));
}
ProgressDescription.propTypes = {
  steps: prop_types_default.a.array.isRequired,
  step: prop_types_default.a.number.isRequired
};

function Group(_ref) {
  var title = _ref.title,
      items = _ref.items,
      IconComponent = _ref.IconComponent;
  var $items = items.map(function (item) {
    return react_default.a.createElement(Item, ProgressDescription_extends({
      key: item.name
    }, item));
  });
  return react_default.a.createElement(StyledGroup, {
    flexDirection: "column",
    p: "4",
    flex: "1"
  }, react_default.a.createElement(design_src["l" /* Flex */], {
    as: design_src["u" /* Text */],
    mb: "4",
    typography: "h3",
    alignItems: "center"
  }, react_default.a.createElement(IconComponent, {
    mr: "3",
    fontSize: "24px",
    width: "50px",
    style: {
      textAlign: "center"
    }
  }), title), react_default.a.createElement(design_src["l" /* Flex */], {
    flexDirection: "column"
  }, $items));
}

function Item(_ref2) {
  var isCompleted = _ref2.isCompleted,
      isProcessing = _ref2.isProcessing,
      name = _ref2.name;

  var IconCmpt = function IconCmpt() {
    return null;
  };

  if (isCompleted) {
    IconCmpt = Icon_Icon["i" /* CircleCheck */];
  }

  if (isProcessing) {
    IconCmpt = StyledSpinner;
  }

  return react_default.a.createElement(design_src["l" /* Flex */], {
    as: design_src["u" /* Text */],
    typography: "h5",
    my: "3",
    alignItems: "center",
    style: {
      position: "relative"
    }
  }, react_default.a.createElement("div", {
    style: {
      position: "absolute"
    }
  }, react_default.a.createElement(IconCmpt, {
    ml: "3",
    fontSize: "20px"
  })), react_default.a.createElement(design_src["u" /* Text */], {
    ml: "9"
  }, name));
}

var StyledGroup = Object(styled_components_browser_esm["c" /* default */])(design_src["l" /* Flex */])(ProgressDescription_templateObject(), function (_ref3) {
  var theme = _ref3.theme;
  return theme.colors.primary.dark;
});
var StyledSpinner = Object(styled_components_browser_esm["c" /* default */])(Icon_Icon["X" /* Spinner */])(_templateObject2(), function (_ref4) {
  var _ref4$fontSize = _ref4.fontSize,
      fontSize = _ref4$fontSize === void 0 ? "32px" : _ref4$fontSize;
  return "\n    font-size: ".concat(fontSize, ";\n    height: ").concat(fontSize, ";\n    width: ").concat(fontSize, ";\n  ");
});
// CONCATENATED MODULE: ./src/installer/components/StepProgress/Completed.jsx
function Completed_templateObject() {
  var data = Completed_taggedTemplateLiteral(["\n  text-align: center;\n\n"]);

  Completed_templateObject = function _templateObject() {
    return data;
  };

  return data;
}

function Completed_taggedTemplateLiteral(strings, raw) { if (!raw) { raw = strings.slice(0); } return Object.freeze(Object.defineProperties(strings, { raw: { value: Object.freeze(raw) } })); }

function Completed_extends() { Completed_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; }; return Completed_extends.apply(this, arguments); }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/





function Completed(props) {
  var completeInstallUrl = src_config["a" /* default */].getInstallerLastStepUrl(props.siteId);
  return react_default.a.createElement(StyledCompleted, Completed_extends({
    flexDirection: "column",
    bg: "light",
    py: "5",
    px: "10",
    color: "text.onLight"
  }, props), react_default.a.createElement(Icon_Icon["i" /* CircleCheck */], {
    mb: "5",
    color: "success",
    fontSize: "100px"
  }), react_default.a.createElement(design_src["b" /* Box */], {
    as: design_src["u" /* Text */],
    typography: "h5",
    mb: "8"
  }, "The application has been installed successfully. Please continue and configure your application to finish the setup process."), react_default.a.createElement(design_src["f" /* ButtonPrimary */], {
    as: "a",
    href: completeInstallUrl,
    size: "large"
  }, "Continue & finish setup"));
}
var StyledCompleted = Object(styled_components_browser_esm["c" /* default */])(design_src["i" /* Card */])(Completed_templateObject());
// EXTERNAL MODULE: ./src/services/downloader.js
var downloader = __webpack_require__("wc0V");

// CONCATENATED MODULE: ./src/installer/components/StepProgress/Failed.jsx
function Failed_templateObject() {
  var data = Failed_taggedTemplateLiteral(["\n  text-align: center;\n"]);

  Failed_templateObject = function _templateObject() {
    return data;
  };

  return data;
}

function Failed_taggedTemplateLiteral(strings, raw) { if (!raw) { raw = strings.slice(0); } return Object.freeze(Object.defineProperties(strings, { raw: { value: Object.freeze(raw) } })); }

function Failed_extends() { Failed_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; }; return Failed_extends.apply(this, arguments); }

function Failed_objectWithoutProperties(source, excluded) { if (source == null) return {}; var target = Failed_objectWithoutPropertiesLoose(source, excluded); var key, i; if (Object.getOwnPropertySymbols) { var sourceSymbolKeys = Object.getOwnPropertySymbols(source); for (i = 0; i < sourceSymbolKeys.length; i++) { key = sourceSymbolKeys[i]; if (excluded.indexOf(key) >= 0) continue; if (!Object.prototype.propertyIsEnumerable.call(source, key)) continue; target[key] = source[key]; } } return target; }

function Failed_objectWithoutPropertiesLoose(source, excluded) { if (source == null) return {}; var target = {}; var sourceKeys = Object.keys(source); var key, i; for (i = 0; i < sourceKeys.length; i++) { key = sourceKeys[i]; if (excluded.indexOf(key) >= 0) continue; target[key] = source[key]; } return target; }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/





function Failed(_ref) {
  var tarballUrl = _ref.tarballUrl,
      styles = Failed_objectWithoutProperties(_ref, ["tarballUrl"]);

  function onClick() {
    location.href = Object(downloader["b" /* makeDownloadable */])(tarballUrl);
  }

  return react_default.a.createElement(Failed_StyledCompleted, Failed_extends({
    flexDirection: "column",
    bg: "light",
    py: "5",
    px: "10",
    color: "text.onLight"
  }, styles), react_default.a.createElement(Icon_Icon["gb" /* Warning */], {
    mb: "5",
    color: "error.main",
    fontSize: "100px"
  }), react_default.a.createElement(design_src["b" /* Box */], {
    as: design_src["u" /* Text */],
    typography: "h5",
    mb: "8"
  }, "Something went wrong with the install. We've attached a tarball which has diagnostic logs that our team will need to review. We sincerely apologize for any inconvenience"), react_default.a.createElement(design_src["h" /* ButtonWarning */], {
    size: "large",
    onClick: onClick
  }, "Download tarball"));
}
var Failed_StyledCompleted = Object(styled_components_browser_esm["c" /* default */])(design_src["i" /* Card */])(Failed_templateObject());
// CONCATENATED MODULE: ./src/installer/components/StepProgress/StepProgress.jsx
function StepProgress_extends() { StepProgress_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; }; return StepProgress_extends.apply(this, arguments); }

function StepProgress_slicedToArray(arr, i) { return StepProgress_arrayWithHoles(arr) || StepProgress_iterableToArrayLimit(arr, i) || StepProgress_nonIterableRest(); }

function StepProgress_nonIterableRest() { throw new TypeError("Invalid attempt to destructure non-iterable instance"); }

function StepProgress_iterableToArrayLimit(arr, i) { if (!(Symbol.iterator in Object(arr) || Object.prototype.toString.call(arr) === "[object Arguments]")) { return; } var _arr = []; var _n = true; var _d = false; var _e = undefined; try { for (var _i = arr[Symbol.iterator](), _s; !(_n = (_s = _i.next()).done); _n = true) { _arr.push(_s.value); if (i && _arr.length === i) break; } } catch (err) { _d = true; _e = err; } finally { try { if (!_n && _i["return"] != null) _i["return"](); } finally { if (_d) throw _e; } } return _arr; }

function StepProgress_arrayWithHoles(arr) { if (Array.isArray(arr)) return arr; }

function StepProgress_objectWithoutProperties(source, excluded) { if (source == null) return {}; var target = StepProgress_objectWithoutPropertiesLoose(source, excluded); var key, i; if (Object.getOwnPropertySymbols) { var sourceSymbolKeys = Object.getOwnPropertySymbols(source); for (i = 0; i < sourceSymbolKeys.length; i++) { key = sourceSymbolKeys[i]; if (excluded.indexOf(key) >= 0) continue; if (!Object.prototype.propertyIsEnumerable.call(source, key)) continue; target[key] = source[key]; } } return target; }

function StepProgress_objectWithoutPropertiesLoose(source, excluded) { if (source == null) return {}; var target = {}; var sourceKeys = Object.keys(source); var key, i; for (i = 0; i < sourceKeys.length; i++) { key = sourceKeys[i]; if (excluded.indexOf(key) >= 0) continue; target[key] = source[key]; } return target; }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/















function StepProgress(props) {
  var progress = props.progress,
      logProvider = props.logProvider,
      styles = StepProgress_objectWithoutProperties(props, ["progress", "logProvider"]);

  var _React$useState = react_default.a.useState(false),
      _React$useState2 = StepProgress_slicedToArray(_React$useState, 2),
      showLogs = _React$useState2[0],
      toggleLogs = _React$useState2[1];

  function onToggleLogs() {
    toggleLogs(!showLogs);
  }

  var isError = progress.isError,
      isCompleted = progress.isCompleted,
      step = progress.step,
      siteId = progress.siteId,
      crashReportUrl = progress.crashReportUrl;
  var progressValue = 100 / PROGRESS_STATE_STRINGS.length * (step + 1);
  var isInstalling = !(isError || isCompleted);
  var title = isInstalling ? "Installation" : '';
  return react_default.a.createElement(StepLayout, StepProgress_extends({
    title: title,
    height: "100%"
  }, styles), isCompleted && react_default.a.createElement(Completed, {
    siteId: siteId
  }), isError && react_default.a.createElement(Failed, {
    tarballUrl: crashReportUrl
  }), isInstalling && react_default.a.createElement(react_default.a.Fragment, null, react_default.a.createElement(ProgressBar, {
    mb: "4",
    value: progressValue
  }), react_default.a.createElement(ProgressDescription, {
    step: step,
    steps: PROGRESS_STATE_STRINGS
  })), react_default.a.createElement(ExpandPanel, {
    mt: "4",
    title: "Executable Logs",
    expanded: showLogs,
    onToggle: onToggleLogs,
    height: showLogs ? "100%" : "auto"
  }, react_default.a.createElement(design_src["l" /* Flex */], {
    pt: "2",
    px: "2",
    minHeight: "400px",
    height: "100%",
    bg: "bgTerminal",
    style: {
      display: showLogs ? 'inherit' : 'none'
    }
  }, react_default.a.createElement(LogViewer["a" /* default */], {
    autoScroll: true,
    provider: logProvider
  }))));
} // state provider

/* harmony default export */ var StepProgress_StepProgress = (function (props) {
  var _useInstallerContext = useInstallerContext(),
      state = _useInstallerContext.state;

  var _state$operation = state.operation,
      siteId = _state$operation.siteId,
      id = _state$operation.id;
  var progress = Object(nuclear["b" /* useFluxStore */])(getters.progressById(id)); // poll operation progress status

  function onFetchProgress() {
    return fetchOpProgress(siteId, id);
  } // creates web socket connection and streams install logs


  var $provider = react_default.a.createElement(StepProgress_InstallLogsProvider, {
    siteId: siteId,
    opId: id
  });
  return react_default.a.createElement(react_default.a.Fragment, null, progress && react_default.a.createElement(StepProgress, StepProgress_extends({}, props, {
    progress: progress,
    logProvider: $provider
  })), react_default.a.createElement(AjaxPoller["a" /* default */], {
    time: POLL_INTERVAL,
    onFetch: onFetchProgress
  }));
});
var POLL_INTERVAL = 3000; // every 5 sec

var PROGRESS_STATE_STRINGS = ['Provisioning Instances', 'Connecting to instances', 'Verifying instances', 'Preparing configuration', 'Installing dependencies', 'Installing platform', 'Installing application', 'Verifying application', 'Connecting to application'];
// CONCATENATED MODULE: ./src/installer/components/StepProgress/index.js
/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/* harmony default export */ var components_StepProgress = (StepProgress_StepProgress);
// EXTERNAL MODULE: ./src/installer/components/StepCapacity/FlavorSelector/Slider.jsx
var Slider = __webpack_require__("SEMh");
var Slider_default = /*#__PURE__*/__webpack_require__.n(Slider);

// CONCATENATED MODULE: ./src/installer/components/StepCapacity/FlavorSelector/FlavorSelector.jsx
function FlavorSelector_templateObject() {
  var data = FlavorSelector_taggedTemplateLiteral(["\n  .grv-installer-provision-flavors-range {\n    padding-top: 15px;\n    margin-top: 45px;\n    font-size: 13px;\n    color: #969696;\n  }\n\n  .grv-slider-value {\n    width: 3px;\n    height: 15px;\n    top: -4px;\n    z-index: 1;\n    background: #DDD;\n  }\n\n  .grv-slider-value-desc {\n    top: -20px;\n  }\n\n  .grv-slider-value-desc:first-child{\n    margin-left: 0 !important;\n    text-align: start !important;\n  }\n\n  .grv-slider-value-desc:last-child{\n    right: 0;\n    width: auto !important;\n    text-align: right !important;\n  }\n\n  .grv-slider {\n    margin-top: 16px;\n    height: 50px;\n  }\n\n  .grv-slider .bar {\n    height: 6px;\n    border-radius: 10px;\n  }\n\n  .grv-slider .handle {\n    width: 20px;\n    height: 20px;\n    left: -10px;\n    top: -7px;\n    border-radius: 14px;\n    background: ", ";\n    box-shadow: rgba(0, 0, 0, 0.2) 0px 1px 3px 1px, rgba(0, 0, 0, 0.14) 0px 2px 2px 0px, rgba(0, 0, 0, 0.12) 0px 3px 1px -2px;\n  }\n\n  .grv-slider .handle:after {\n  }\n\n  .grv-slider .bar-0 {\n    background: none repeat scroll 0 0 ", ";\n    box-shadow: none;\n  }\n\n  .grv-slider .bar-1 {\n    background-color: white;\n  }\n\n  .grv-slider .grv-installer-provision-flavors-handle {\n    width: 50px;\n    text-align: center;\n    position: absolute;\n    font-size: 13px;\n    margin-top: -30px;\n    margin-left: -13px;\n    border-radius: 15%;\n  }\n"]);

  FlavorSelector_templateObject = function _templateObject() {
    return data;
  };

  return data;
}

function FlavorSelector_taggedTemplateLiteral(strings, raw) { if (!raw) { raw = strings.slice(0); } return Object.freeze(Object.defineProperties(strings, { raw: { value: Object.freeze(raw) } })); }

function FlavorSelector_ownKeys(object, enumerableOnly) { var keys = Object.keys(object); if (Object.getOwnPropertySymbols) { var symbols = Object.getOwnPropertySymbols(object); if (enumerableOnly) symbols = symbols.filter(function (sym) { return Object.getOwnPropertyDescriptor(object, sym).enumerable; }); keys.push.apply(keys, symbols); } return keys; }

function FlavorSelector_objectSpread(target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i] != null ? arguments[i] : {}; if (i % 2) { FlavorSelector_ownKeys(source, true).forEach(function (key) { FlavorSelector_defineProperty(target, key, source[key]); }); } else if (Object.getOwnPropertyDescriptors) { Object.defineProperties(target, Object.getOwnPropertyDescriptors(source)); } else { FlavorSelector_ownKeys(source).forEach(function (key) { Object.defineProperty(target, key, Object.getOwnPropertyDescriptor(source, key)); }); } } return target; }

function FlavorSelector_defineProperty(obj, key, value) { if (key in obj) { Object.defineProperty(obj, key, { value: value, enumerable: true, configurable: true, writable: true }); } else { obj[key] = value; } return obj; }

function FlavorSelector_extends() { FlavorSelector_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; }; return FlavorSelector_extends.apply(this, arguments); }

function FlavorSelector_objectWithoutProperties(source, excluded) { if (source == null) return {}; var target = FlavorSelector_objectWithoutPropertiesLoose(source, excluded); var key, i; if (Object.getOwnPropertySymbols) { var sourceSymbolKeys = Object.getOwnPropertySymbols(source); for (i = 0; i < sourceSymbolKeys.length; i++) { key = sourceSymbolKeys[i]; if (excluded.indexOf(key) >= 0) continue; if (!Object.prototype.propertyIsEnumerable.call(source, key)) continue; target[key] = source[key]; } } return target; }

function FlavorSelector_objectWithoutPropertiesLoose(source, excluded) { if (source == null) return {}; var target = {}; var sourceKeys = Object.keys(source); var key, i; for (i = 0; i < sourceKeys.length; i++) { key = sourceKeys[i]; if (excluded.indexOf(key) >= 0) continue; target[key] = source[key]; } return target; }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/





function FlavorSelector(props) {
  var current = props.current,
      options = props.options,
      onChange = props.onChange,
      rest = FlavorSelector_objectWithoutProperties(props, ["current", "options", "onChange"]);

  var total = options.length;

  if (total < 2) {
    return null;
  }

  function onSliderChange(value) {
    onChange(value - 1);
  }

  return react_default.a.createElement(StyledFlavorBox, FlavorSelector_extends({
    mb: "10"
  }, rest), react_default.a.createElement(Slider_default.a, {
    options: options,
    valueComponent: react_default.a.createElement(FlavorValueComponent, {
      options: options
    }),
    min: 1,
    max: total,
    value: current + 1,
    onChange: onSliderChange,
    defaultValue: 1,
    withBars: true,
    className: "grv-slider"
  }));
}
FlavorSelector.propTypes = {
  current: prop_types_default.a.number.isRequired,
  options: prop_types_default.a.array
};

function Value(_ref) {
  var offset = _ref.offset,
      marginLeft = _ref.marginLeft;
  var props = {
    className: 'grv-slider-value',
    style: {
      position: 'absolute',
      left: "".concat(offset, "px"),
      marginLeft: "".concat(marginLeft, "px")
    }
  };
  return react_default.a.createElement("div", props);
}

function ValueDesc(_ref2) {
  var offset = _ref2.offset,
      width = _ref2.width,
      marginLeft = _ref2.marginLeft,
      text = _ref2.text;
  var props = {
    className: 'grv-slider-value-desc',
    style: {
      width: "".concat(width, "px"),
      position: 'absolute',
      marginLeft: "".concat(width / -2 + marginLeft, "px"),
      left: "".concat(offset, "px"),
      textAlign: 'center'
    }
  };
  return react_default.a.createElement("div", props, react_default.a.createElement("span", null, text), react_default.a.createElement("br", null));
}

function FlavorValueComponent(props) {
  var options = props.options,
      handleSize = props.handleSize,
      upperBound = props.upperBound,
      sliderLength = props.sliderLength;
  var $vals = [];
  var $descriptions = [];
  var count = options.length - 1;
  var widthWithHandle = upperBound / count;
  var widthWithoutHandle = sliderLength / count;
  var marginLeft = handleSize / 2;

  for (var i = 0; i < options.length; i++) {
    var offset = widthWithHandle * i;
    var label = options[i].label;
    var valueProps = {
      key: 'value_' + i,
      offset: offset,
      marginLeft: marginLeft
    };

    var descProps = FlavorSelector_objectSpread({}, valueProps, {
      key: 'desc_' + i,
      width: widthWithoutHandle,
      text: label
    });

    $vals.push(react_default.a.createElement(Value, valueProps));
    $descriptions.push(react_default.a.createElement(ValueDesc, descProps));
  }

  return react_default.a.createElement("div", null, $vals, react_default.a.createElement("div", {
    className: "grv-installer-provision-flavors-range",
    style: {
      position: 'absolute',
      width: '100%'
    }
  }, $descriptions));
}

var StyledFlavorBox = Object(styled_components_browser_esm["c" /* default */])(design_src["b" /* Box */])(FlavorSelector_templateObject(), function (_ref3) {
  var theme = _ref3.theme;
  return theme.colors.success;
}, function (_ref4) {
  var theme = _ref4.theme;
  return theme.colors.success;
});
// CONCATENATED MODULE: ./src/installer/components/StepCapacity/FlavorSelector/index.js
/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/* harmony default export */ var StepCapacity_FlavorSelector = (FlavorSelector);
// EXTERNAL MODULE: ./src/components/CmdText/index.js + 1 modules
var CmdText = __webpack_require__("3SkG");

// EXTERNAL MODULE: ../shared/components/FieldInput/index.js + 1 modules
var FieldInput = __webpack_require__("tT4w");

// CONCATENATED MODULE: ./src/installer/components/StepCapacity/Flavor/Server/FieldMount.jsx
function FieldMount_extends() { FieldMount_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; }; return FieldMount_extends.apply(this, arguments); }

function FieldMount_slicedToArray(arr, i) { return FieldMount_arrayWithHoles(arr) || FieldMount_iterableToArrayLimit(arr, i) || FieldMount_nonIterableRest(); }

function FieldMount_nonIterableRest() { throw new TypeError("Invalid attempt to destructure non-iterable instance"); }

function FieldMount_iterableToArrayLimit(arr, i) { if (!(Symbol.iterator in Object(arr) || Object.prototype.toString.call(arr) === "[object Arguments]")) { return; } var _arr = []; var _n = true; var _d = false; var _e = undefined; try { for (var _i = arr[Symbol.iterator](), _s; !(_n = (_s = _i.next()).done); _n = true) { _arr.push(_s.value); if (i && _arr.length === i) break; } } catch (err) { _d = true; _e = err; } finally { try { if (!_n && _i["return"] != null) _i["return"](); } finally { if (_d) throw _e; } } return _arr; }

function FieldMount_arrayWithHoles(arr) { if (Array.isArray(arr)) return arr; }

function FieldMount_objectWithoutProperties(source, excluded) { if (source == null) return {}; var target = FieldMount_objectWithoutPropertiesLoose(source, excluded); var key, i; if (Object.getOwnPropertySymbols) { var sourceSymbolKeys = Object.getOwnPropertySymbols(source); for (i = 0; i < sourceSymbolKeys.length; i++) { key = sourceSymbolKeys[i]; if (excluded.indexOf(key) >= 0) continue; if (!Object.prototype.propertyIsEnumerable.call(source, key)) continue; target[key] = source[key]; } } return target; }

function FieldMount_objectWithoutPropertiesLoose(source, excluded) { if (source == null) return {}; var target = {}; var sourceKeys = Object.keys(source); var key, i; for (i = 0; i < sourceKeys.length; i++) { key = sourceKeys[i]; if (excluded.indexOf(key) >= 0) continue; target[key] = source[key]; } return target; }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/




function FieldMount(_ref) {
  var defaultValue = _ref.defaultValue,
      name = _ref.name,
      onChange = _ref.onChange,
      styles = FieldMount_objectWithoutProperties(_ref, ["defaultValue", "name", "onChange"]);

  var _React$useState = react_default.a.useState(defaultValue),
      _React$useState2 = FieldMount_slicedToArray(_React$useState, 2),
      value = _React$useState2[0],
      setValue = _React$useState2[1]; // notify parent about current value


  react_default.a.useEffect(function () {
    onChange({
      name: name,
      value: value
    });
  }, [value]); // pick up this field title from web config

  var title = react_default.a.useMemo(function () {
    var mountCfg = src_config["a" /* default */].getAgentDeviceMount(name);
    var title = mountCfg.labelText || name;
    return Object(lodash["capitalize"])(title);
  }, [name]);

  function onFieldChange(e) {
    setValue(e.target.value);
  }

  return react_default.a.createElement(FieldInput["a" /* default */], FieldMount_extends({
    mb: "3"
  }, styles, {
    value: value,
    label: title,
    rule: required("".concat(title, " is required")),
    onChange: onFieldChange
  }));
}

var required = function required(message) {
  return function (value) {
    return function () {
      return {
        valid: !!value,
        message: message
      };
    };
  };
};
// EXTERNAL MODULE: ../shared/components/FieldSelect/index.js + 3 modules
var FieldSelect = __webpack_require__("53l0");

// CONCATENATED MODULE: ./src/installer/components/StepCapacity/Flavor/Server/FieldInterface.jsx
function FieldInterface_extends() { FieldInterface_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; }; return FieldInterface_extends.apply(this, arguments); }

function FieldInterface_slicedToArray(arr, i) { return FieldInterface_arrayWithHoles(arr) || FieldInterface_iterableToArrayLimit(arr, i) || FieldInterface_nonIterableRest(); }

function FieldInterface_nonIterableRest() { throw new TypeError("Invalid attempt to destructure non-iterable instance"); }

function FieldInterface_iterableToArrayLimit(arr, i) { if (!(Symbol.iterator in Object(arr) || Object.prototype.toString.call(arr) === "[object Arguments]")) { return; } var _arr = []; var _n = true; var _d = false; var _e = undefined; try { for (var _i = arr[Symbol.iterator](), _s; !(_n = (_s = _i.next()).done); _n = true) { _arr.push(_s.value); if (i && _arr.length === i) break; } } catch (err) { _d = true; _e = err; } finally { try { if (!_n && _i["return"] != null) _i["return"](); } finally { if (_d) throw _e; } } return _arr; }

function FieldInterface_arrayWithHoles(arr) { if (Array.isArray(arr)) return arr; }

function FieldInterface_objectWithoutProperties(source, excluded) { if (source == null) return {}; var target = FieldInterface_objectWithoutPropertiesLoose(source, excluded); var key, i; if (Object.getOwnPropertySymbols) { var sourceSymbolKeys = Object.getOwnPropertySymbols(source); for (i = 0; i < sourceSymbolKeys.length; i++) { key = sourceSymbolKeys[i]; if (excluded.indexOf(key) >= 0) continue; if (!Object.prototype.propertyIsEnumerable.call(source, key)) continue; target[key] = source[key]; } } return target; }

function FieldInterface_objectWithoutPropertiesLoose(source, excluded) { if (source == null) return {}; var target = {}; var sourceKeys = Object.keys(source); var key, i; for (i = 0; i < sourceKeys.length; i++) { key = sourceKeys[i]; if (excluded.indexOf(key) >= 0) continue; target[key] = source[key]; } return target; }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/



function InterfaceVariable(props) {
  var defaultValue = props.defaultValue,
      onChange = props.onChange,
      options = props.options,
      styles = FieldInterface_objectWithoutProperties(props, ["defaultValue", "onChange", "options"]);

  var _React$useState = react_default.a.useState(defaultValue),
      _React$useState2 = FieldInterface_slicedToArray(_React$useState, 2),
      value = _React$useState2[0],
      setValue = _React$useState2[1];

  react_default.a.useEffect(function () {
    onChange(value);
  }, [value]); // pick up this field title from web config

  var _React$useMemo = react_default.a.useMemo(function () {
    var ipCfg = src_config["a" /* default */].getAgentDeviceIpv4();
    var label = ipCfg.labelText || 'IP Address';
    var selectOptions = options.map(function (item) {
      return {
        value: item,
        label: item
      };
    });
    return {
      label: label,
      selectOptions: selectOptions
    };
  }, []),
      label = _React$useMemo.label,
      selectOptions = _React$useMemo.selectOptions;

  function onChangeSelect(option) {
    setValue(option.value);
  }

  return react_default.a.createElement(FieldSelect["a" /* default */], FieldInterface_extends({
    mb: "3"
  }, styles, {
    rule: FieldInterface_required("".concat(label, " is required")),
    label: label,
    value: {
      value: value,
      label: value
    },
    options: selectOptions,
    onChange: onChangeSelect
  }));
}

var FieldInterface_required = function required(message) {
  return function (option) {
    return function () {
      return {
        valid: option && option.value,
        message: message
      };
    };
  };
};
// CONCATENATED MODULE: ./src/installer/components/StepCapacity/Flavor/Server/Server.jsx
function Server_templateObject() {
  var data = Server_taggedTemplateLiteral(["\n  border-top: 1px solid ", ";\n"]);

  Server_templateObject = function _templateObject() {
    return data;
  };

  return data;
}

function Server_taggedTemplateLiteral(strings, raw) { if (!raw) { raw = strings.slice(0); } return Object.freeze(Object.defineProperties(strings, { raw: { value: Object.freeze(raw) } })); }

function Server_extends() { Server_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; }; return Server_extends.apply(this, arguments); }

function Server_objectWithoutProperties(source, excluded) { if (source == null) return {}; var target = Server_objectWithoutPropertiesLoose(source, excluded); var key, i; if (Object.getOwnPropertySymbols) { var sourceSymbolKeys = Object.getOwnPropertySymbols(source); for (i = 0; i < sourceSymbolKeys.length; i++) { key = sourceSymbolKeys[i]; if (excluded.indexOf(key) >= 0) continue; if (!Object.prototype.propertyIsEnumerable.call(source, key)) continue; target[key] = source[key]; } } return target; }

function Server_objectWithoutPropertiesLoose(source, excluded) { if (source == null) return {}; var target = {}; var sourceKeys = Object.keys(source); var key, i; for (i = 0; i < sourceKeys.length; i++) { key = sourceKeys[i]; if (excluded.indexOf(key) >= 0) continue; target[key] = source[key]; } return target; }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/







function Server(_ref) {
  var hostname = _ref.hostname,
      vars = _ref.vars,
      onSetVars = _ref.onSetVars,
      onRemoveVars = _ref.onRemoveVars,
      role = _ref.role,
      styles = Server_objectWithoutProperties(_ref, ["hostname", "vars", "onSetVars", "onRemoveVars", "role"]);

  react_default.a.useEffect(function () {
    function cleanup() {
      onRemoveVars({
        role: role,
        hostname: hostname
      });
    }

    return cleanup;
  }, []);
  var varValues = react_default.a.useMemo(function () {
    return {
      ip: null,
      mounts: {}
    };
  }, []);

  function notify() {
    onSetVars({
      role: role,
      hostname: hostname,
      ip: varValues.ip,
      mounts: Object(lodash["values"])(varValues.mounts)
    });
  }

  function onChangeIp(ip) {
    varValues.ip = ip;
    notify();
  }

  function onSetMount(_ref2) {
    var value = _ref2.value,
        name = _ref2.name;
    varValues.mounts[name] = {
      value: value,
      name: name
    };
    notify();
  }

  var $vars = vars.map(function (v, index) {
    if (v.type === enums["l" /* ServerVarEnums */].INTERFACE) {
      var value = v.value,
          options = v.options;
      return react_default.a.createElement(InterfaceVariable, Server_extends({
        key: index
      }, varBoxProps, {
        maxWidth: "200px",
        defaultValue: value,
        options: options,
        onChange: onChangeIp
      }));
    }

    if (v.type === enums["l" /* ServerVarEnums */].MOUNT) {
      var _value = v.value,
          name = v.name;
      return react_default.a.createElement(FieldMount, Server_extends({
        key: index
      }, varBoxProps, {
        defaultValue: _value,
        name: name,
        onChange: onSetMount
      }));
    }

    return null;
  });
  return react_default.a.createElement(StyledServer, styles, react_default.a.createElement(design_src["b" /* Box */], {
    mr: "4"
  }, react_default.a.createElement(design_src["q" /* LabelInput */], null, "Hostname"), react_default.a.createElement(design_src["u" /* Text */], {
    typography: "h5"
  }, hostname)), react_default.a.createElement(design_src["l" /* Flex */], {
    flexWrap: "wrap",
    flex: "1",
    justifyContent: "flex-end"
  }, $vars));
}
var StyledServer = Object(styled_components_browser_esm["c" /* default */])(design_src["l" /* Flex */])(Server_templateObject(), function (_ref3) {
  var theme = _ref3.theme;
  return theme.colors.primary.dark;
});
var varBoxProps = {
  ml: "3",
  flex: "1",
  minWidth: "180px"
};
// CONCATENATED MODULE: ./src/installer/components/StepCapacity/Flavor/Server/index.js
/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/


// CONCATENATED MODULE: ./src/installer/components/StepCapacity/Flavor/Profile.jsx
/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/




function ProfileOnprem(props) {
  var name = props.name,
      servers = props.servers,
      count = props.count,
      description = props.description,
      instructions = props.instructions,
      requirementsText = props.requirementsText,
      onSetServerVars = props.onSetServerVars,
      onRemoveServerVars = props.onRemoveServerVars,
      mb = props.mb;
  var $servers = servers.map(function (server) {
    return react_default.a.createElement(Server, {
      mx: -4,
      px: "4",
      pt: "3",
      key: name + server.hostname,
      role: server.role,
      hostname: server.hostname,
      vars: server.vars,
      onSetVars: onSetServerVars,
      onRemoveVars: onRemoveServerVars
    });
  });
  return react_default.a.createElement(design_src["i" /* Card */], {
    as: design_src["l" /* Flex */],
    bg: "primary.light",
    px: "4",
    py: "3",
    mb: mb,
    flexDirection: "column"
  }, react_default.a.createElement(design_src["l" /* Flex */], {
    alignItems: "center"
  }, react_default.a.createElement(design_src["r" /* LabelState */], {
    shadow: true,
    width: "100px",
    mr: "6",
    py: "2",
    fontSize: "2",
    style: {
      flexShrink: '0'
    }
  }, labelText(count)), react_default.a.createElement(design_src["l" /* Flex */], {
    flexDirection: "colum",
    flexWrap: "wrap",
    alignItems: "baseline"
  }, react_default.a.createElement(design_src["u" /* Text */], {
    typography: "h3",
    mr: "4"
  }, description), react_default.a.createElement(design_src["u" /* Text */], {
    as: "span",
    typography: "h6"
  }, "REQUIREMENTS - ".concat(requirementsText)))), react_default.a.createElement(design_src["q" /* LabelInput */], {
    mt: "3"
  }, "Copy and paste the command below into terminal. Your server will automatically appear in the list"), react_default.a.createElement(CmdText["a" /* default */], {
    cmd: instructions
  }), react_default.a.createElement(design_src["b" /* Box */], {
    mt: "4"
  }, $servers));
}

function labelText(count) {
  var nodes = count > 1 ? 'nodes' : 'node';
  return "".concat(count, " ").concat(nodes);
}
// EXTERNAL MODULE: ../design/src/Alert/index.jsx + 1 modules
var Alert = __webpack_require__("9TLm");

// EXTERNAL MODULE: ../shared/hooks/index.js + 1 modules
var hooks = __webpack_require__("nVph");

// CONCATENATED MODULE: ./src/installer/components/StepCapacity/Flavor/Flavor.jsx
function Flavor_extends() { Flavor_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; }; return Flavor_extends.apply(this, arguments); }

function Flavor_objectWithoutProperties(source, excluded) { if (source == null) return {}; var target = Flavor_objectWithoutPropertiesLoose(source, excluded); var key, i; if (Object.getOwnPropertySymbols) { var sourceSymbolKeys = Object.getOwnPropertySymbols(source); for (i = 0; i < sourceSymbolKeys.length; i++) { key = sourceSymbolKeys[i]; if (excluded.indexOf(key) >= 0) continue; if (!Object.prototype.propertyIsEnumerable.call(source, key)) continue; target[key] = source[key]; } } return target; }

function Flavor_objectWithoutPropertiesLoose(source, excluded) { if (source == null) return {}; var target = {}; var sourceKeys = Object.keys(source); var key, i; for (i = 0; i < sourceKeys.length; i++) { key = sourceKeys[i]; if (excluded.indexOf(key) >= 0) continue; target[key] = source[key]; } return target; }

function Flavor_slicedToArray(arr, i) { return Flavor_arrayWithHoles(arr) || Flavor_iterableToArrayLimit(arr, i) || Flavor_nonIterableRest(); }

function Flavor_nonIterableRest() { throw new TypeError("Invalid attempt to destructure non-iterable instance"); }

function Flavor_iterableToArrayLimit(arr, i) { if (!(Symbol.iterator in Object(arr) || Object.prototype.toString.call(arr) === "[object Arguments]")) { return; } var _arr = []; var _n = true; var _d = false; var _e = undefined; try { for (var _i = arr[Symbol.iterator](), _s; !(_n = (_s = _i.next()).done); _n = true) { _arr.push(_s.value); if (i && _arr.length === i) break; } } catch (err) { _d = true; _e = err; } finally { try { if (!_n && _i["return"] != null) _i["return"](); } finally { if (_d) throw _e; } } return _arr; }

function Flavor_arrayWithHoles(arr) { if (Array.isArray(arr)) return arr; }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/









var Flavor_POLL_INTERVAL = 3000; // every 5 sec

function Flavor(props) {
  var servers = props.servers,
      profiles = props.profiles,
      store = props.store;

  var _useAttempt = Object(hooks["a" /* useAttempt */])(),
      _useAttempt2 = Flavor_slicedToArray(_useAttempt, 2),
      attempt = _useAttempt2[0],
      attemptActions = _useAttempt2[1];

  var service = useServices();

  function onSetServerVars(_ref) {
    var role = _ref.role,
        hostname = _ref.hostname,
        ip = _ref.ip,
        mounts = _ref.mounts;
    store.setServerVars({
      role: role,
      hostname: hostname,
      ip: ip,
      mounts: mounts
    });
  }

  function onRemoveServerVars(_ref2) {
    var role = _ref2.role,
        hostname = _ref2.hostname;
    store.removeServerVars({
      role: role,
      hostname: hostname
    });
  }

  var $reqItems = profiles.map(function (profile) {
    var instructions = profile.instructions,
        requirementsText = profile.requirementsText,
        count = profile.count,
        name = profile.name,
        description = profile.description;
    var profileServers = servers.filter(function (s) {
      return s.role === name;
    });
    return react_default.a.createElement(ProfileOnprem, {
      mb: "4",
      servers: profileServers,
      key: name,
      requirementsText: requirementsText,
      instructions: instructions,
      count: count,
      name: name,
      description: description,
      onSetServerVars: onSetServerVars,
      onRemoveServerVars: onRemoveServerVars
    });
  });

  function onContinue() {
    var request = store.makeStartInstallRequest();
    attemptActions.start();
    service.startInstall(request).done(function () {
      store.setStepProgress();
    }).fail(function (err) {
      attemptActions.error(err);
    });
  }

  function onFetchAgentReport() {
    var request = store.makeAgentRequest();
    return service.fetchAgentReport(request).then(function (agentServers) {
      store.setAgentServers(agentServers);
    }).fail(function () {
      store.setAgentServers([]);
    });
  }

  function onVerfiy() {
    var request = store.makeStartInstallRequest();
    attemptActions.start();
    service.verifyOnPrem(request).done(function () {
      attemptActions.stop('Verified!');
    }).fail(function (err) {
      attemptActions.error(err);
    });
  }

  var btnDisabled = attempt.isProcessing;
  return react_default.a.createElement(Validation["a" /* default */], null, react_default.a.createElement(react_default.a.Fragment, null, attempt.isFailed && react_default.a.createElement(Alert["a" /* Danger */], null, attempt.message), attempt.isSuccess && react_default.a.createElement(Alert["c" /* Success */], null, attempt.message), $reqItems, react_default.a.createElement(design_src["l" /* Flex */], {
    mt: "60px"
  }, react_default.a.createElement(ButtonValidate, {
    width: "200px",
    mr: "3",
    disabled: btnDisabled,
    onClick: onContinue
  }, "Continue"), react_default.a.createElement(ButtonValidate, {
    disabled: btnDisabled,
    onClick: onVerfiy,
    kind: "secondary"
  }, "Verify"))), react_default.a.createElement(AjaxPoller["a" /* default */], {
    time: Flavor_POLL_INTERVAL,
    onFetch: onFetchAgentReport
  }));
}
Flavor.propTypes = {
  store: prop_types_default.a.object.isRequired,
  profiles: prop_types_default.a.array.isRequired
};

function ButtonValidate(_ref3) {
  var onClick = _ref3.onClick,
      rest = Flavor_objectWithoutProperties(_ref3, ["onClick"]);

  var validator = Object(Validation["c" /* useValidation */])();

  function onContinue() {
    validator.validate() && onClick();
  }

  return react_default.a.createElement(design_src["f" /* ButtonPrimary */], Flavor_extends({
    onClick: onContinue
  }, rest));
}
// CONCATENATED MODULE: ./src/installer/components/StepCapacity/Flavor/index.js
/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/* harmony default export */ var StepCapacity_Flavor = (Flavor);
// CONCATENATED MODULE: ./src/installer/components/StepCapacity/StepCapacity.jsx
function StepCapacity_slicedToArray(arr, i) { return StepCapacity_arrayWithHoles(arr) || StepCapacity_iterableToArrayLimit(arr, i) || StepCapacity_nonIterableRest(); }

function StepCapacity_nonIterableRest() { throw new TypeError("Invalid attempt to destructure non-iterable instance"); }

function StepCapacity_iterableToArrayLimit(arr, i) { if (!(Symbol.iterator in Object(arr) || Object.prototype.toString.call(arr) === "[object Arguments]")) { return; } var _arr = []; var _n = true; var _d = false; var _e = undefined; try { for (var _i = arr[Symbol.iterator](), _s; !(_n = (_s = _i.next()).done); _n = true) { _arr.push(_s.value); if (i && _arr.length === i) break; } } catch (err) { _d = true; _e = err; } finally { try { if (!_n && _i["return"] != null) _i["return"](); } finally { if (_d) throw _e; } } return _arr; }

function StepCapacity_arrayWithHoles(arr) { if (Array.isArray(arr)) return arr; }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/






function StepCapacity() {
  var store = useInstallerContext();
  var flavorOptions = store.state.flavors.options;
  var agentServers = store.state.agentServers; // selected flavor

  var _React$useState = react_default.a.useState(function () {
    var index = Object(lodash["findIndex"])(flavorOptions, function (f) {
      return f.isDefault === true;
    });
    return index !== -1 ? index : 0;
  }),
      _React$useState2 = StepCapacity_slicedToArray(_React$useState, 2),
      selectedFlavor = _React$useState2[0],
      setSelectedFlavor = _React$useState2[1]; // slider options


  var sliderOptions = react_default.a.useMemo(function () {
    return Object(lodash["map"])(store.state.flavors.options, function (f) {
      return {
        value: f.name,
        label: f.title
      };
    });
  }); // profiles of selected flavor

  var profiles = react_default.a.useMemo(function () {
    if (flavorOptions[selectedFlavor]) {
      var p = flavorOptions[selectedFlavor].profiles; // set new profile to configure from given flavor

      store.setProvisionProfiles(p);
      return p;
    }

    return [];
  }, [selectedFlavor]);

  function onChangeFlavor(index) {
    setSelectedFlavor(index);
  }

  return react_default.a.createElement(StepLayout, {
    title: store.state.flavors.prompt || "Review Infrastructure Requirements"
  }, react_default.a.createElement(StepCapacity_FlavorSelector, {
    current: selectedFlavor,
    options: sliderOptions,
    onChange: onChangeFlavor
  }), react_default.a.createElement(StepCapacity_Flavor, {
    servers: agentServers,
    key: selectedFlavor,
    profiles: profiles,
    store: store
  }));
}
// CONCATENATED MODULE: ./src/installer/components/StepCapacity/index.js
/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/* harmony default export */ var components_StepCapacity = (StepCapacity);
// CONCATENATED MODULE: ./src/installer/components/StepList/StepList.jsx
function StepList_templateObject2() {
  var data = StepList_taggedTemplateLiteral(["\n  min-width: 500px;\n  display: flex;\n  align-items: center;\n  flex-shrink: 0;\n  flex-wrap: wrap;\n  flex: 1;\n"]);

  StepList_templateObject2 = function _templateObject2() {
    return data;
  };

  return data;
}

function StepList_templateObject() {
  var data = StepList_taggedTemplateLiteral(["\n  position: relative;\n  &:last-child{\n    margin-right: 0;\n  }\n\n  ", "\n"]);

  StepList_templateObject = function _templateObject() {
    return data;
  };

  return data;
}

function StepList_taggedTemplateLiteral(strings, raw) { if (!raw) { raw = strings.slice(0); } return Object.freeze(Object.defineProperties(strings, { raw: { value: Object.freeze(raw) } })); }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/



function StepList(_ref) {
  var options = _ref.options,
      value = _ref.value;
  var $steps = options.map(function (option, index) {
    return react_default.a.createElement(StepList_StepListItem, {
      active: value === option.value,
      title: "".concat(index + 1, ". ").concat(option.title),
      key: option.value
    });
  });
  return react_default.a.createElement(StyledStepList, {
    bold: true,
    children: $steps
  });
}

var StepList_StepListItem = function StepListItem(_ref2) {
  var title = _ref2.title,
      active = _ref2.active;
  return react_default.a.createElement(StyledTabItem, {
    color: "text.primary",
    active: active,
    typography: "h3",
    mr: 5,
    py: "2"
  }, title);
};

var StyledTabItem = Object(styled_components_browser_esm["c" /* default */])(design_src["u" /* Text */])(StepList_templateObject(), function (_ref3) {
  var active = _ref3.active,
      theme = _ref3.theme;

  if (active) {
    return "\n        &:after {\n          background-color: ".concat(theme.colors.accent, ";\n          content: \"\";\n          position: absolute;\n          bottom: 0;\n          left: 0;\n          width: 100%;\n          height: 4px;\n      }\n    ");
  }
});
var StyledStepList = styled_components_browser_esm["c" /* default */].div(StepList_templateObject2());
// CONCATENATED MODULE: ./src/installer/components/StepList/index.js
/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/* harmony default export */ var components_StepList = (StepList);
// EXTERNAL MODULE: ./src/services/history.js
var services_history = __webpack_require__("sRt+");

// EXTERNAL MODULE: ../design/src/ButtonIcon/index.js + 1 modules
var ButtonIcon = __webpack_require__("Z6Fm");

// CONCATENATED MODULE: ./src/installer/components/StepProvider/AdvancedOptions/ClusterTags/Tag.jsx
function Tag_extends() { Tag_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; }; return Tag_extends.apply(this, arguments); }

function Tag_objectWithoutProperties(source, excluded) { if (source == null) return {}; var target = Tag_objectWithoutPropertiesLoose(source, excluded); var key, i; if (Object.getOwnPropertySymbols) { var sourceSymbolKeys = Object.getOwnPropertySymbols(source); for (i = 0; i < sourceSymbolKeys.length; i++) { key = sourceSymbolKeys[i]; if (excluded.indexOf(key) >= 0) continue; if (!Object.prototype.propertyIsEnumerable.call(source, key)) continue; target[key] = source[key]; } } return target; }

function Tag_objectWithoutPropertiesLoose(source, excluded) { if (source == null) return {}; var target = {}; var sourceKeys = Object.keys(source); var key, i; for (i = 0; i < sourceKeys.length; i++) { key = sourceKeys[i]; if (excluded.indexOf(key) >= 0) continue; target[key] = source[key]; } return target; }

function Tag_templateObject() {
  var data = Tag_taggedTemplateLiteral(["\n  max-width: 200px;\n  overflow: auto;\n  display: flex;\n  align-items: center;\n  background: ", ";\n  border-radius: 10px;\n  > span {\n    white-space: nowrap;\n    overflow: hidden;\n    text-overflow: ellipsis;\n  }\n\n  ", "{\n    color: ", ";\n    border-radius: 50%;\n    font-size: 14px;\n    min-width: 10px;\n  }\n\n  ", "\n  ", "\n  ", "\n"]);

  Tag_templateObject = function _templateObject() {
    return data;
  };

  return data;
}

function Tag_taggedTemplateLiteral(strings, raw) { if (!raw) { raw = strings.slice(0); } return Object.freeze(Object.defineProperties(strings, { raw: { value: Object.freeze(raw) } })); }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/





var StyledTag = styled_components_browser_esm["c" /* default */].div(Tag_templateObject(), function (props) {
  return props.theme.colors.primary.dark;
}, Icon["G" /* default */], function (_ref) {
  var theme = _ref.theme;
  return theme.colors.text.primary;
}, system["w" /* typography */], system["f" /* color */], system["u" /* space */]);
function Tag(_ref2) {
  var name = _ref2.name,
      value = _ref2.value,
      onClick = _ref2.onClick,
      styles = Tag_objectWithoutProperties(_ref2, ["name", "value", "onClick"]);

  function onIconClick() {
    onClick(name);
  }

  var text = value ? "".concat(name, ": ").concat(value) : name;
  return react_default.a.createElement(StyledTag, Tag_extends({
    typography: "body2"
  }, styles, {
    bg: "primary.dark",
    color: "primary.contrastText",
    pl: "2",
    pr: "1"
  }), react_default.a.createElement("span", {
    title: text
  }, text), react_default.a.createElement(ButtonIcon["a" /* default */], {
    size: 0,
    onClick: onIconClick,
    ml: "1",
    bg: "primary.light"
  }, react_default.a.createElement(Icon["k" /* Close */], null)));
}
// CONCATENATED MODULE: ./src/installer/components/StepProvider/AdvancedOptions/ClusterTags/ClusterTags.jsx
function ClusterTags_ownKeys(object, enumerableOnly) { var keys = Object.keys(object); if (Object.getOwnPropertySymbols) { var symbols = Object.getOwnPropertySymbols(object); if (enumerableOnly) symbols = symbols.filter(function (sym) { return Object.getOwnPropertyDescriptor(object, sym).enumerable; }); keys.push.apply(keys, symbols); } return keys; }

function ClusterTags_objectSpread(target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i] != null ? arguments[i] : {}; if (i % 2) { ClusterTags_ownKeys(source, true).forEach(function (key) { ClusterTags_defineProperty(target, key, source[key]); }); } else if (Object.getOwnPropertyDescriptors) { Object.defineProperties(target, Object.getOwnPropertyDescriptors(source)); } else { ClusterTags_ownKeys(source).forEach(function (key) { Object.defineProperty(target, key, Object.getOwnPropertyDescriptor(source, key)); }); } } return target; }

function ClusterTags_defineProperty(obj, key, value) { if (key in obj) { Object.defineProperty(obj, key, { value: value, enumerable: true, configurable: true, writable: true }); } else { obj[key] = value; } return obj; }

function ClusterTags_slicedToArray(arr, i) { return ClusterTags_arrayWithHoles(arr) || ClusterTags_iterableToArrayLimit(arr, i) || ClusterTags_nonIterableRest(); }

function ClusterTags_nonIterableRest() { throw new TypeError("Invalid attempt to destructure non-iterable instance"); }

function ClusterTags_iterableToArrayLimit(arr, i) { if (!(Symbol.iterator in Object(arr) || Object.prototype.toString.call(arr) === "[object Arguments]")) { return; } var _arr = []; var _n = true; var _d = false; var _e = undefined; try { for (var _i = arr[Symbol.iterator](), _s; !(_n = (_s = _i.next()).done); _n = true) { _arr.push(_s.value); if (i && _arr.length === i) break; } } catch (err) { _d = true; _e = err; } finally { try { if (!_n && _i["return"] != null) _i["return"](); } finally { if (_d) throw _e; } } return _arr; }

function ClusterTags_arrayWithHoles(arr) { if (Array.isArray(arr)) return arr; }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/



var ENTER_KEY = 13;

function ClusterTags(_ref) {
  var onChange = _ref.onChange;

  var _React$useState = react_default.a.useState(''),
      _React$useState2 = ClusterTags_slicedToArray(_React$useState, 2),
      value = _React$useState2[0],
      setValue = _React$useState2[1];

  var _React$useState3 = react_default.a.useState({}),
      _React$useState4 = ClusterTags_slicedToArray(_React$useState3, 2),
      tags = _React$useState4[0],
      setTags = _React$useState4[1]; // notify parent about the change


  react_default.a.useEffect(function () {
    onChange(tags);
  }, [tags]);

  function onChangeValue(e) {
    setValue(e.target.value);
  }

  function onAddTags() {
    if (value) {
      var tagsToAdd = {};
      parseTags(value).forEach(function (t) {
        tagsToAdd[t.key] = t.value;
      });
      setTags(ClusterTags_objectSpread({}, tags, {}, tagsToAdd));
      setValue('');
    }
  }

  function onKeyDown(e) {
    if (e.which === ENTER_KEY) {
      onAddTags();
    }
  }

  function onDelete(key) {
    delete tags[key];
    setTags(ClusterTags_objectSpread({}, tags));
  }

  return react_default.a.createElement(design_src["b" /* Box */], null, react_default.a.createElement(design_src["q" /* LabelInput */], null, "Create cluster labels"), react_default.a.createElement(design_src["l" /* Flex */], {
    mb: "4"
  }, react_default.a.createElement(design_src["o" /* Input */], {
    mr: "3",
    value: value,
    onKeyDown: onKeyDown,
    onChange: onChangeValue,
    autoComplete: "off",
    placeholder: "key:value, key:value, ..."
  }), react_default.a.createElement(design_src["f" /* ButtonPrimary */], {
    onClick: onAddTags
  }, "Create")), react_default.a.createElement(LabelList, {
    tags: tags,
    onDelete: onDelete
  }));
}

function LabelList(_ref2) {
  var tags = _ref2.tags,
      onDelete = _ref2.onDelete;
  var $tags = Object.keys(tags).map(function (key) {
    return react_default.a.createElement(Tag, {
      mr: "2",
      mb: "2",
      key: key,
      name: key,
      value: tags[key],
      onClick: function onClick() {
        return onDelete(key);
      }
    });
  });
  return react_default.a.createElement(design_src["l" /* Flex */], {
    flexWrap: "wrap"
  }, $tags);
}

function parseTags(str) {
  return str.split(',').map(function (t) {
    var _t$split = t.split(':'),
        _t$split2 = ClusterTags_slicedToArray(_t$split, 2),
        key = _t$split2[0],
        value = _t$split2[1]; // remove spaces


    key = key ? key.trim() : key;
    value = value ? value.trim() : value;
    return {
      key: key,
      value: value
    };
  }).filter(function (tag) {
    return tag.value && tag.key;
  });
}

/* harmony default export */ var ClusterTags_ClusterTags = (ClusterTags);
// CONCATENATED MODULE: ./src/installer/components/StepProvider/AdvancedOptions/ClusterTags/index.js
/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/* harmony default export */ var AdvancedOptions_ClusterTags = (ClusterTags_ClusterTags);
// CONCATENATED MODULE: ./src/installer/components/StepProvider/AdvancedOptions/AdvancedOptions.jsx
function AdvancedOptions_templateObject() {
  var data = AdvancedOptions_taggedTemplateLiteral(["\n  cursor: pointer;\n  // prevent text selection on accidental double click\n  -webkit-user-select: none;\n  -moz-user-select: none;\n  -khtml-user-select: none;\n  -ms-user-select: none;\n"]);

  AdvancedOptions_templateObject = function _templateObject() {
    return data;
  };

  return data;
}

function AdvancedOptions_taggedTemplateLiteral(strings, raw) { if (!raw) { raw = strings.slice(0); } return Object.freeze(Object.defineProperties(strings, { raw: { value: Object.freeze(raw) } })); }

function AdvancedOptions_extends() { AdvancedOptions_extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; }; return AdvancedOptions_extends.apply(this, arguments); }

function AdvancedOptions_slicedToArray(arr, i) { return AdvancedOptions_arrayWithHoles(arr) || AdvancedOptions_iterableToArrayLimit(arr, i) || AdvancedOptions_nonIterableRest(); }

function AdvancedOptions_nonIterableRest() { throw new TypeError("Invalid attempt to destructure non-iterable instance"); }

function AdvancedOptions_iterableToArrayLimit(arr, i) { if (!(Symbol.iterator in Object(arr) || Object.prototype.toString.call(arr) === "[object Arguments]")) { return; } var _arr = []; var _n = true; var _d = false; var _e = undefined; try { for (var _i = arr[Symbol.iterator](), _s; !(_n = (_s = _i.next()).done); _n = true) { _arr.push(_s.value); if (i && _arr.length === i) break; } } catch (err) { _d = true; _e = err; } finally { try { if (!_n && _i["return"] != null) _i["return"](); } finally { if (_d) throw _e; } } return _arr; }

function AdvancedOptions_arrayWithHoles(arr) { if (Array.isArray(arr)) return arr; }

function AdvancedOptions_objectWithoutProperties(source, excluded) { if (source == null) return {}; var target = AdvancedOptions_objectWithoutPropertiesLoose(source, excluded); var key, i; if (Object.getOwnPropertySymbols) { var sourceSymbolKeys = Object.getOwnPropertySymbols(source); for (i = 0; i < sourceSymbolKeys.length; i++) { key = sourceSymbolKeys[i]; if (excluded.indexOf(key) >= 0) continue; if (!Object.prototype.propertyIsEnumerable.call(source, key)) continue; target[key] = source[key]; } } return target; }

function AdvancedOptions_objectWithoutPropertiesLoose(source, excluded) { if (source == null) return {}; var target = {}; var sourceKeys = Object.keys(source); var key, i; for (i = 0; i < sourceKeys.length; i++) { key = sourceKeys[i]; if (excluded.indexOf(key) >= 0) continue; target[key] = source[key]; } return target; }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/






function AdvancedOptions(_ref) {
  var children = _ref.children,
      onChangeTags = _ref.onChangeTags,
      styles = AdvancedOptions_objectWithoutProperties(_ref, ["children", "onChangeTags"]);

  var _React$useState = react_default.a.useState(false),
      _React$useState2 = AdvancedOptions_slicedToArray(_React$useState, 2),
      isExpanded = _React$useState2[0],
      setExpanded = _React$useState2[1];

  function onToggle() {
    setExpanded(!isExpanded);
  }

  var IconCmpt = isExpanded ? Icon["b" /* ArrowUp */] : Icon["a" /* ArrowDown */];
  return react_default.a.createElement(design_src["l" /* Flex */], AdvancedOptions_extends({
    width: "100%",
    flexDirection: "column",
    bg: "primary.light"
  }, styles), react_default.a.createElement(AdvancedOptions_StyledHeader, {
    height: "50px",
    pl: "3",
    pr: "2",
    py: "2",
    flex: "1",
    bg: "primary.main",
    alignItems: "center",
    justifyContent: "space-between",
    onClick: onToggle
  }, react_default.a.createElement(design_src["u" /* Text */], {
    typography: "subtitle1",
    caps: true
  }, "Additional Options"), react_default.a.createElement(design_src["c" /* ButtonIcon */], {
    onClick: onToggle
  }, react_default.a.createElement(IconCmpt, null))), isExpanded && react_default.a.createElement(design_src["b" /* Box */], {
    p: "3"
  }, children, react_default.a.createElement(AdvancedOptions_ClusterTags, {
    onChange: onChangeTags
  })));
}

var AdvancedOptions_StyledHeader = Object(styled_components_browser_esm["c" /* default */])(design_src["l" /* Flex */])(AdvancedOptions_templateObject());
/* harmony default export */ var AdvancedOptions_AdvancedOptions = (AdvancedOptions);
// EXTERNAL MODULE: ./src/lib/paramUtils.js
var paramUtils = __webpack_require__("Qwgj");

// CONCATENATED MODULE: ./src/installer/components/StepProvider/AdvancedOptions/Subnets/Subnets.jsx
function Subnets_objectWithoutProperties(source, excluded) { if (source == null) return {}; var target = Subnets_objectWithoutPropertiesLoose(source, excluded); var key, i; if (Object.getOwnPropertySymbols) { var sourceSymbolKeys = Object.getOwnPropertySymbols(source); for (i = 0; i < sourceSymbolKeys.length; i++) { key = sourceSymbolKeys[i]; if (excluded.indexOf(key) >= 0) continue; if (!Object.prototype.propertyIsEnumerable.call(source, key)) continue; target[key] = source[key]; } } return target; }

function Subnets_objectWithoutPropertiesLoose(source, excluded) { if (source == null) return {}; var target = {}; var sourceKeys = Object.keys(source); var key, i; for (i = 0; i < sourceKeys.length; i++) { key = sourceKeys[i]; if (excluded.indexOf(key) >= 0) continue; target[key] = source[key]; } return target; }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/




var POD_HOST_NUM = 65534;
var INVALID_SUBNET = 'Invalid CIDR format';
var VALIDATION_POD_SUBNET_MIN = "Range cannot be less than ".concat(POD_HOST_NUM);
function Subnets(_ref) {
  var onChange = _ref.onChange,
      podSubnet = _ref.podSubnet,
      serviceSubnet = _ref.serviceSubnet,
      styles = Subnets_objectWithoutProperties(_ref, ["onChange", "podSubnet", "serviceSubnet"]);

  function onChangePodnet(e) {
    onChange({
      podSubnet: e.target.value,
      serviceSubnet: serviceSubnet
    });
  }

  function onChangeServiceSubnet(e) {
    onChange({
      podSubnet: podSubnet,
      serviceSubnet: e.target.value
    });
  }

  return react_default.a.createElement(design_src["l" /* Flex */], styles, react_default.a.createElement(FieldInput["a" /* default */], {
    flex: "1",
    autoComplete: "off",
    label: "Service Subnet",
    mr: "3",
    onChange: onChangeServiceSubnet,
    placeholder: "10.0.0.0/16",
    rule: Subnets_validCidr,
    value: serviceSubnet
  }), react_default.a.createElement(FieldInput["a" /* default */], {
    flex: "1",
    autoComplete: "off",
    label: "Pod Subnet",
    onChange: onChangePodnet,
    placeholder: "10.0.0.0/16",
    rule: Subnets_validPod,
    value: podSubnet
  }));
}

var Subnets_validCidr = function validCidr(value) {
  return function () {
    return {
      valid: Object(paramUtils["a" /* parseCidr */])(value) !== null,
      message: INVALID_SUBNET
    };
  };
};

var Subnets_validPod = function validPod(value) {
  return function () {
    var result = Object(paramUtils["a" /* parseCidr */])(value);

    if (result && result.totalHost <= POD_HOST_NUM) {
      return {
        valid: false,
        message: VALIDATION_POD_SUBNET_MIN
      };
    }

    return {
      valid: result !== null,
      message: INVALID_SUBNET
    };
  };
};
// CONCATENATED MODULE: ./src/installer/components/StepProvider/AdvancedOptions/Subnets/index.js
/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/* harmony default export */ var AdvancedOptions_Subnets = (Subnets);
// CONCATENATED MODULE: ./src/installer/components/StepProvider/AdvancedOptions/index.js
/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/


/* harmony default export */ var StepProvider_AdvancedOptions = (AdvancedOptions_AdvancedOptions);

// CONCATENATED MODULE: ./src/installer/components/StepProvider/StepProvider.jsx
function StepProvider_slicedToArray(arr, i) { return StepProvider_arrayWithHoles(arr) || StepProvider_iterableToArrayLimit(arr, i) || StepProvider_nonIterableRest(); }

function StepProvider_nonIterableRest() { throw new TypeError("Invalid attempt to destructure non-iterable instance"); }

function StepProvider_iterableToArrayLimit(arr, i) { if (!(Symbol.iterator in Object(arr) || Object.prototype.toString.call(arr) === "[object Arguments]")) { return; } var _arr = []; var _n = true; var _d = false; var _e = undefined; try { for (var _i = arr[Symbol.iterator](), _s; !(_n = (_s = _i.next()).done); _n = true) { _arr.push(_s.value); if (i && _arr.length === i) break; } } catch (err) { _d = true; _e = err; } finally { try { if (!_n && _i["return"] != null) _i["return"](); } finally { if (_d) throw _e; } } return _arr; }

function StepProvider_arrayWithHoles(arr) { if (Array.isArray(arr)) return arr; }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/












function StepProvider() {
  var store = useInstallerContext();
  var _store$state = store.state,
      clusterName = _store$state.clusterName,
      serviceSubnet = _store$state.serviceSubnet,
      podSubnet = _store$state.podSubnet;
  var validator = Object(Validation["c" /* useValidation */])();

  var _useAttempt = Object(hooks["a" /* useAttempt */])(),
      _useAttempt2 = StepProvider_slicedToArray(_useAttempt, 2),
      attempt = _useAttempt2[0],
      attemptActions = _useAttempt2[1];

  var isFailed = attempt.isFailed,
      isProcessing = attempt.isProcessing,
      message = attempt.message;

  function onChangeName(name) {
    store.setClusterName(name);
  }

  function onStart(request) {
    return installer.createCluster(request).then(function (clusterName) {
      services_history["a" /* default */].push(src_config["a" /* default */].getInstallerProvisionUrl(clusterName), true);
    });
  }

  function onChangeSubnets(_ref) {
    var podSubnet = _ref.podSubnet,
        serviceSubnet = _ref.serviceSubnet;
    store.setOnpremSubnets(serviceSubnet, podSubnet);
  }

  function onChangeTags(tags) {
    store.setClusterTags(tags);
  }

  function onContinue() {
    if (validator.validate()) {
      attemptActions.start();
      var request = store.makeOnpremRequest();
      onStart(request).fail(function (err) {
        return attemptActions.error(err);
      });
    }
  }

  return react_default.a.createElement(StepLayout, {
    title: "Name your cluster"
  }, react_default.a.createElement(FieldInput["a" /* default */], {
    placeholder: "prod.example.com",
    autoFocus: true,
    rule: StepProvider_required,
    value: clusterName,
    onChange: function onChange(e) {
      return onChangeName(e.target.value);
    },
    label: "Cluster Name"
  }), react_default.a.createElement(design_src["b" /* Box */], null, isFailed && react_default.a.createElement(Alert["a" /* Danger */], {
    mb: "4"
  }, message), react_default.a.createElement(StepProvider_AdvancedOptions, {
    onChangeTags: onChangeTags
  }, react_default.a.createElement(AdvancedOptions_Subnets, {
    serviceSubnet: serviceSubnet,
    podSubnet: podSubnet,
    onChange: onChangeSubnets
  })), react_default.a.createElement(design_src["f" /* ButtonPrimary */], {
    disabled: isProcessing,
    mt: "6",
    width: "200px",
    onClick: onContinue
  }, "Continue")));
}

var StepProvider_required = function required(value) {
  return function () {
    return {
      valid: !!value,
      message: 'Cluster name is required'
    };
  };
};
// CONCATENATED MODULE: ./src/installer/components/StepProvider/index.js
/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/* harmony default export */ var components_StepProvider = (StepProvider);
// CONCATENATED MODULE: ./src/installer/components/StepLicense/StepLicense.jsx
function StepLicense_templateObject() {
  var data = StepLicense_taggedTemplateLiteral(["\n  border-radius: 6px;\n  min-height: 200px;\n  overflow: auto;\n  white-space: pre;\n  word-break: break-all;\n  word-wrap: break-word;\n"]);

  StepLicense_templateObject = function _templateObject() {
    return data;
  };

  return data;
}

function StepLicense_taggedTemplateLiteral(strings, raw) { if (!raw) { raw = strings.slice(0); } return Object.freeze(Object.defineProperties(strings, { raw: { value: Object.freeze(raw) } })); }

function StepLicense_slicedToArray(arr, i) { return StepLicense_arrayWithHoles(arr) || StepLicense_iterableToArrayLimit(arr, i) || StepLicense_nonIterableRest(); }

function StepLicense_nonIterableRest() { throw new TypeError("Invalid attempt to destructure non-iterable instance"); }

function StepLicense_iterableToArrayLimit(arr, i) { if (!(Symbol.iterator in Object(arr) || Object.prototype.toString.call(arr) === "[object Arguments]")) { return; } var _arr = []; var _n = true; var _d = false; var _e = undefined; try { for (var _i = arr[Symbol.iterator](), _s; !(_n = (_s = _i.next()).done); _n = true) { _arr.push(_s.value); if (i && _arr.length === i) break; } } catch (err) { _d = true; _e = err; } finally { try { if (!_n && _i["return"] != null) _i["return"](); } finally { if (_d) throw _e; } } return _arr; }

function StepLicense_arrayWithHoles(arr) { if (Array.isArray(arr)) return arr; }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/








function StepLicense(_ref) {
  var store = _ref.store;

  var _useAttempt = Object(hooks["a" /* useAttempt */])(),
      _useAttempt2 = StepLicense_slicedToArray(_useAttempt, 2),
      attempt = _useAttempt2[0],
      attemptActions = _useAttempt2[1];

  var _React$useState = react_default.a.useState(''),
      _React$useState2 = StepLicense_slicedToArray(_React$useState, 2),
      license = _React$useState2[0],
      setLicense = _React$useState2[1];

  var licenseHeaderText = store.state.config.licenseHeaderText;
  var isProcessing = attempt.isProcessing,
      isFailed = attempt.isFailed,
      message = attempt.message;
  var btnDisabled = isProcessing || !license;
  var service = useServices();

  function onContinue() {
    attemptActions["do"](function () {
      return service.setDeploymentType(license, store.state.app.packageId);
    }).done(function () {
      store.setLicense(license);
    });
  }

  return react_default.a.createElement(StepLayout, {
    title: licenseHeaderText
  }, isFailed && react_default.a.createElement(Alert["a" /* Danger */], null, " ", message), react_default.a.createElement(StyledLicense, {
    as: "textarea",
    px: "2",
    py: "2",
    mb: "4",
    value: license,
    autoComplete: "off",
    onChange: function onChange(e) {
      return setLicense(e.target.value);
    },
    typography: "body1",
    mono: true,
    bg: "light",
    placeholder: "Insert your license key here",
    color: "text.onLight"
  }), react_default.a.createElement(design_src["f" /* ButtonPrimary */], {
    width: "200px",
    disabled: btnDisabled,
    onClick: onContinue
  }, "Continue"));
}
StepLicense.propTypes = {
  label: prop_types_default.a.string.isRequired
};
var StyledLicense = Object(styled_components_browser_esm["c" /* default */])(design_src["u" /* Text */])(StepLicense_templateObject());
/* harmony default export */ var StepLicense_StepLicense = (StepLicense);
// CONCATENATED MODULE: ./src/installer/components/StepLicense/index.js
/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/* harmony default export */ var components_StepLicense = (StepLicense_StepLicense);
// CONCATENATED MODULE: ./src/installer/components/Description/Description.jsx
function Description_templateObject() {
  var data = Description_taggedTemplateLiteral(["\n  white-space: pre-line;\n\n  .ul{\n    padding-left: 10px\n  }\n"]);

  Description_templateObject = function _templateObject() {
    return data;
  };

  return data;
}

function Description_taggedTemplateLiteral(strings, raw) { if (!raw) { raw = strings.slice(0); } return Object.freeze(Object.defineProperties(strings, { raw: { value: Object.freeze(raw) } })); }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/





function Description(_ref) {
  var store = _ref.store;
  var _store$state = store.state,
      step = _store$state.step,
      config = _store$state.config;
  var licenseUserHintText = config.licenseUserHintText,
      prereqUserHintText = config.prereqUserHintText,
      provisionUserHintText = config.provisionUserHintText,
      progressUserHintText = config.progressUserHintText;
  var hintTexts = {};
  hintTexts[StepEnum.LICENSE] = licenseUserHintText;
  hintTexts[StepEnum.NEW_APP] = prereqUserHintText;
  hintTexts[StepEnum.PROVISION] = provisionUserHintText;
  hintTexts[StepEnum.PROGRESS] = progressUserHintText;
  var text = hintTexts[step] || 'Your custom text here';
  return react_default.a.createElement(StyledHint, {
    flexDirection: "column",
    py: "10",
    px: "5",
    maxWidth: "600px"
  }, react_default.a.createElement(design_src["u" /* Text */], {
    typography: "h3",
    mb: "4"
  }, "About this step"), react_default.a.createElement(design_src["u" /* Text */], {
    typography: "paragraph",
    dangerouslySetInnerHTML: {
      __html: text
    }
  }));
}
Description.style = {
  whiteSpace: 'pre-line'
};
Description.propTypes = {
  store: prop_types_default.a.object.isRequired
};
var StyledHint = Object(styled_components_browser_esm["c" /* default */])(design_src["l" /* Flex */])(Description_templateObject());
// CONCATENATED MODULE: ./src/installer/components/Description/index.js
/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/* harmony default export */ var components_Description = (Description);
// CONCATENATED MODULE: ./src/installer/components/Installer.jsx
/* unused harmony export Installer */
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "a", function() { return Container; });
function Installer_slicedToArray(arr, i) { return Installer_arrayWithHoles(arr) || Installer_iterableToArrayLimit(arr, i) || Installer_nonIterableRest(); }

function Installer_nonIterableRest() { throw new TypeError("Invalid attempt to destructure non-iterable instance"); }

function Installer_iterableToArrayLimit(arr, i) { if (!(Symbol.iterator in Object(arr) || Object.prototype.toString.call(arr) === "[object Arguments]")) { return; } var _arr = []; var _n = true; var _d = false; var _e = undefined; try { for (var _i = arr[Symbol.iterator](), _s; !(_n = (_s = _i.next()).done); _n = true) { _arr.push(_s.value); if (i && _arr.length === i) break; } } catch (err) { _d = true; _e = err; } finally { try { if (!_n && _i["return"] != null) _i["return"](); } finally { if (_d) throw _e; } } return _arr; }

function Installer_arrayWithHoles(arr) { if (Array.isArray(arr)) return arr; }

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/















function Installer(props) {
  var store = useInstallerStore();
  var service = useServices();
  var repository = props.repository,
      name = props.name,
      version = props.version,
      siteId = props.siteId;
  var _store$state = store.state,
      step = _store$state.step,
      stepOptions = _store$state.stepOptions,
      config = _store$state.config,
      app = _store$state.app,
      eulaAccepted = _store$state.eulaAccepted,
      status = _store$state.status,
      statusText = _store$state.statusText;
  react_default.a.useEffect(function () {
    if (!siteId) {
      service.fetchApp(name, repository, version).then(function (app) {
        return store.initWithApp(app);
      }).fail(function (err) {
        return store.setError(err);
      });
    } else {
      service.fetchClusterDetails(siteId).then(function (response) {
        store.initWithCluster(response);
      }).fail(function (err) {
        return store.setError(err);
      });
    }
  }, []);

  if (status === 'error') {
    return react_default.a.createElement(CardError["a" /* Failed */], {
      message: statusText
    });
  }

  if (status !== 'ready') {
    return react_default.a.createElement(AppLayout, {
      alignItems: "center",
      justifyContent: "center"
    }, react_default.a.createElement(design_src["n" /* Indicator */], null));
  }

  if (app.eula && !eulaAccepted) {
    return react_default.a.createElement(components_Eula, {
      onAccept: store.acceptEula,
      config: config,
      app: app
    });
  }

  var logoSrc = app.logo;
  return react_default.a.createElement(Validation["a" /* default */], null, react_default.a.createElement(AppLayout, null, react_default.a.createElement(design_src["l" /* Flex */], {
    flex: "1",
    px: "8",
    py: "10",
    mr: "4",
    mb: "5",
    justifyContent: "flex-end",
    style: {
      overflow: 'auto'
    }
  }, react_default.a.createElement(design_src["l" /* Flex */], {
    flexDirection: "column",
    flex: "1",
    maxWidth: "1000px"
  }, react_default.a.createElement(design_src["l" /* Flex */], {
    mb: "10",
    alignItems: "center",
    flexWrap: "wrap"
  }, react_default.a.createElement(Logo, {
    src: logoSrc
  }), react_default.a.createElement(components_StepList, {
    value: step,
    options: stepOptions
  })), step === StepEnum.NEW_APP && react_default.a.createElement(components_StepProvider, null), step === StepEnum.LICENSE && react_default.a.createElement(components_StepLicense, {
    store: store
  }), step === StepEnum.PROVISION && react_default.a.createElement(components_StepCapacity, null), step === StepEnum.PROGRESS && react_default.a.createElement(components_StepProgress, null))), react_default.a.createElement(design_src["l" /* Flex */], {
    flex: "0 0 30%",
    bg: "primary.main"
  }, react_default.a.createElement(components_Description, {
    store: store
  }))));
}
function Container(_ref) {
  var match = _ref.match,
      service = _ref.service,
      store = _ref.store;
  var _match$params = match.params,
      siteId = _match$params.siteId,
      repository = _match$params.repository,
      name = _match$params.name,
      version = _match$params.version;

  var _React$useState = react_default.a.useState(function () {
    return store || new store_InstallerStore();
  }),
      _React$useState2 = Installer_slicedToArray(_React$useState, 1),
      installerStore = _React$useState2[0];

  var props = {
    siteId: siteId,
    repository: repository,
    name: name,
    version: version
  };
  return react_default.a.createElement(Provider, {
    value: service
  }, react_default.a.createElement(store_Provider, {
    value: installerStore
  }, react_default.a.createElement(Installer, props)));
}

/***/ })

}]);