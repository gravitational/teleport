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

import React from 'react';
import PropTypes from 'prop-types';
import createReactClass from 'create-react-class';

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

var isArray =
  Array.isArray ||
  function (x) {
    return Object.prototype.toString.call(x) === '[object Array]';
  };

// undoEnsureArray(ensureArray(x)) === x

const ReactSlider = createReactClass({
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
    defaultValue: PropTypes.oneOfType([
      PropTypes.number,
      PropTypes.arrayOf(PropTypes.number),
    ]),

    /**
     * Like `defaultValue` but for [controlled components](http://facebook.github.io/react/docs/forms.html#controlled-components).
     */
    value: PropTypes.oneOfType([
      PropTypes.number,
      PropTypes.arrayOf(PropTypes.number),
    ]),

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
     *  Callback called when the slider is clicked (handle or bars).
     *  Receives the value at the clicked position as argument.
     */
    onSliderClick: PropTypes.func,
  },

  getDefaultProps: function () {
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
      invert: false,
    };
  },

  getInitialState: function () {
    var value = this._or(
      ensureArray(this.props.value),
      ensureArray(this.props.defaultValue)
    );

    // reused throughout the component to store results of iterations over `value`
    this.tempArray = value.slice();

    // array for storing resize timeouts ids
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
      zIndices: zIndices,
    };
  },

  // Keep the internal `value` consistent with an outside `value` if present.
  // This basically allows the slider to be a controlled component.
  UNSAFE_componentWillReceiveProps: function (newProps) {
    var value = this._or(ensureArray(newProps.value), this.state.value);

    // ensure the array keeps the same size as `value`
    this.tempArray = value.slice();

    for (var i = 0; i < value.length; i++) {
      this.state.value[i] = this._trimAlignValue(value[i], newProps);
    }
    if (this.state.value.length > value.length)
      this.state.value.length = value.length;

    // If an upperBound has not yet been determined (due to the component being hidden
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
  _or: function (value, defaultValue) {
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
          window.console.warn(
            this.constructor.displayName +
              ': Number of values does not match number of children.'
          );
        }
        return linspace(this.props.min, this.props.max, count);
    }
  },

  componentDidMount: function () {
    window.addEventListener('resize', this._handleResize);
    this._resize();
  },

  componentWillUnmount: function () {
    this._clearPendingResizeTimeouts();
    window.removeEventListener('resize', this._handleResize);
  },

  getValue: function () {
    return undoEnsureArray(this.state.value);
  },

  _resize: function () {
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
      sliderStart: this.props.invert ? sliderMax : sliderMin,
    });
  },

  _handleResize: function () {
    // setTimeout of 0 gives element enough time to have assumed its new size if it is being resized
    var resizeTimeout = window.setTimeout(
      function () {
        // drop this timeout from pendingResizeTimeouts to reduce memory usage
        this.pendingResizeTimeouts.shift();
        this._resize();
      }.bind(this),
      0
    );

    this.pendingResizeTimeouts.push(resizeTimeout);
  },

  // clear all pending timeouts to avoid error messages after unmounting
  _clearPendingResizeTimeouts: function () {
    do {
      var nextTimeout = this.pendingResizeTimeouts.shift();

      clearTimeout(nextTimeout);
    } while (this.pendingResizeTimeouts.length);
  },

  // calculates the offset of a handle in pixels based on its value.
  _calcOffset: function (value) {
    var range = this.props.max - this.props.min;
    if (range === 0) {
      return 0;
    }
    var ratio = (value - this.props.min) / range;
    return ratio * this.state.upperBound;
  },

  // calculates the value corresponding to a given pixel offset, i.e. the inverse of `_calcOffset`.
  _calcValue: function (offset) {
    var ratio = offset / this.state.upperBound;
    return ratio * (this.props.max - this.props.min) + this.props.min;
  },

  _buildHandleStyle: function (offset, i) {
    var style = {
      position: 'absolute',
      willChange: this.state.index >= 0 ? this._posMinKey() : '',
      zIndex: this.state.zIndices.indexOf(i) + 1,
    };
    style[this._posMinKey()] = offset + 'px';
    return style;
  },

  _buildBarStyle: function (min, max) {
    var obj = {
      position: 'absolute',
      willChange:
        this.state.index >= 0
          ? this._posMinKey() + ',' + this._posMaxKey()
          : '',
    };
    obj[this._posMinKey()] = min;
    obj[this._posMaxKey()] = max;
    return obj;
  },

  _getClosestIndex: function (pixelOffset) {
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

  _calcOffsetFromPosition: function (position) {
    var pixelOffset = position - this.state.sliderStart;
    if (this.props.invert) pixelOffset = this.state.sliderLength - pixelOffset;
    pixelOffset -= this.state.handleSize / 2;
    return pixelOffset;
  },

  // Snaps the nearest handle to the value corresponding to `position` and calls `callback` with that handle's index.
  _forceValueFromPosition: function (position, callback) {
    var pixelOffset = this._calcOffsetFromPosition(position);
    var closestIndex = this._getClosestIndex(pixelOffset);
    var nextValue = this._trimAlignValue(this._calcValue(pixelOffset));

    var value = this.state.value.slice(); // Clone this.state.value since we'll modify it temporarily
    value[closestIndex] = nextValue;

    // Prevents the slider from shrinking below `props.minDistance`
    for (var i = 0; i < value.length - 1; i += 1) {
      if (value[i + 1] - value[i] < this.props.minDistance) return;
    }

    this.setState({ value: value }, callback.bind(this, closestIndex));
  },

  _getMousePosition: function (e) {
    return [e['page' + this._axisKey()], e['page' + this._orthogonalAxisKey()]];
  },

  _getTouchPosition: function (e) {
    var touch = e.touches[0];
    return [
      touch['page' + this._axisKey()],
      touch['page' + this._orthogonalAxisKey()],
    ];
  },

  _getKeyDownEventMap: function () {
    return {
      keydown: this._onKeyDown,
      focusout: this._onBlur,
    };
  },

  _getMouseEventMap: function () {
    return {
      mousemove: this._onMouseMove,
      mouseup: this._onMouseUp,
    };
  },

  _getTouchEventMap: function () {
    return {
      touchmove: this._onTouchMove,
      touchend: this._onTouchEnd,
    };
  },

  // create the `keydown` handler for the i-th handle
  _createOnKeyDown: function (i) {
    return function (e) {
      if (this.props.disabled) return;
      this._start(i);
      this._addHandlers(this._getKeyDownEventMap());
      pauseEvent(e);
    }.bind(this);
  },

  // create the `mousedown` handler for the i-th handle
  _createOnMouseDown: function (i) {
    return function (e) {
      if (this.props.disabled) return;
      var position = this._getMousePosition(e);
      this._start(i, position[0]);
      this._addHandlers(this._getMouseEventMap());
      pauseEvent(e);
    }.bind(this);
  },

  // create the `touchstart` handler for the i-th handle
  _createOnTouchStart: function (i) {
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

  _addHandlers: function (eventMap) {
    for (var key in eventMap) {
      document.addEventListener(key, eventMap[key], false);
    }
  },

  _removeHandlers: function (eventMap) {
    for (var key in eventMap) {
      document.removeEventListener(key, eventMap[key], false);
    }
  },

  _start: function (i, position) {
    var activeEl = document.activeElement;
    var handleRef = this['handle' + i];
    // if activeElement is body window will lost focus in IE9
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
        startPosition:
          position !== undefined ? position : prevState.startPosition,
        index: i,
        zIndices: zIndices,
      };
    });
  },

  _onMouseUp: function () {
    this._onEnd(this._getMouseEventMap());
  },

  _onTouchEnd: function () {
    this._onEnd(this._getTouchEventMap());
  },

  _onBlur: function () {
    this._onEnd(this._getKeyDownEventMap());
  },

  _onEnd: function (eventMap) {
    this._removeHandlers(eventMap);
    this.setState(
      { index: -1 },
      this._fireChangeEvent.bind(this, 'onAfterChange')
    );
  },

  _onMouseMove: function (e) {
    var position = this._getMousePosition(e);
    var diffPosition = this._getDiffPosition(position[0]);
    var newValue = this._getValueFromPosition(diffPosition);
    this._move(newValue);
  },

  _onTouchMove: function (e) {
    if (e.touches.length > 1) return;

    var position = this._getTouchPosition(e);

    if (typeof this.isScrolling === 'undefined') {
      var diffMainDir = position[0] - this.startPosition[0];
      var diffScrollDir = position[1] - this.startPosition[1];
      this.isScrolling = Math.abs(diffScrollDir) > Math.abs(diffMainDir);
    }

    if (this.isScrolling) {
      this.setState({ index: -1 });
      return;
    }

    pauseEvent(e);

    var diffPosition = this._getDiffPosition(position[0]);
    var newValue = this._getValueFromPosition(diffPosition);

    this._move(newValue);
  },

  _onKeyDown: function (e) {
    if (e.ctrlKey || e.shiftKey || e.altKey) return;
    switch (e.key) {
      case 'ArrowLeft':
      case 'ArrowUp':
        e.preventDefault();
        return this._moveDownOneStep();
      case 'ArrowRight':
      case 'ArrowDown':
        e.preventDefault();
        return this._moveUpOneStep();
      case 'Home':
        return this._move(this.props.min);
      case 'End':
        return this._move(this.props.max);
      default:
        return;
    }
  },

  _moveUpOneStep: function () {
    var oldValue = this.state.value[this.state.index];
    var newValue = oldValue + this.props.step;
    this._move(Math.min(newValue, this.props.max));
  },

  _moveDownOneStep: function () {
    var oldValue = this.state.value[this.state.index];
    var newValue = oldValue - this.props.step;
    this._move(Math.max(newValue, this.props.min));
  },

  _getValueFromPosition: function (position) {
    var diffValue =
      (position / (this.state.sliderLength - this.state.handleSize)) *
      (this.props.max - this.props.min);
    return this._trimAlignValue(this.state.startValue + diffValue);
  },

  _getDiffPosition: function (position) {
    var diffPosition = position - this.state.startPosition;
    if (this.props.invert) diffPosition *= -1;
    return diffPosition;
  },

  _move: function (newValue) {
    this.hasMoved = true;

    var props = this.props;
    var state = this.state;
    var index = state.index;

    var value = state.value;
    var length = value.length;
    var oldValue = value[index];

    var minDistance = props.minDistance;

    // if "pearling" (= handles pushing each other) is disabled,
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

    value[index] = newValue;

    // if "pearling" is enabled, let the current handle push the pre- and succeeding handles.
    if (props.pearling && length > 1) {
      if (newValue > oldValue) {
        this._pushSucceeding(value, minDistance, index);
        this._trimSucceeding(length, value, minDistance, props.max);
      } else if (newValue < oldValue) {
        this._pushPreceding(value, minDistance, index);
        this._trimPreceding(length, value, minDistance, props.min);
      }
    }

    // Normally you would use `shouldComponentUpdate`, but since the slider is a low-level component,
    // the extra complexity might be worth the extra performance.
    if (newValue !== oldValue) {
      this.setState(
        { value: value },
        this._fireChangeEvent.bind(this, 'onChange')
      );
    }
  },

  _pushSucceeding: function (value, minDistance, index) {
    var i, padding;
    for (
      i = index, padding = value[i] + minDistance;
      value[i + 1] != null && padding > value[i + 1];
      i++, padding = value[i] + minDistance
    ) {
      value[i + 1] = this._alignValue(padding);
    }
  },

  _trimSucceeding: function (length, nextValue, minDistance, max) {
    for (var i = 0; i < length; i++) {
      var padding = max - i * minDistance;
      if (nextValue[length - 1 - i] > padding) {
        nextValue[length - 1 - i] = padding;
      }
    }
  },

  _pushPreceding: function (value, minDistance, index) {
    var i, padding;
    for (
      i = index, padding = value[i] - minDistance;
      value[i - 1] != null && padding < value[i - 1];
      i--, padding = value[i] - minDistance
    ) {
      value[i - 1] = this._alignValue(padding);
    }
  },

  _trimPreceding: function (length, nextValue, minDistance, min) {
    for (var i = 0; i < length; i++) {
      var padding = min + i * minDistance;
      if (nextValue[i] < padding) {
        nextValue[i] = padding;
      }
    }
  },

  _axisKey: function () {
    var orientation = this.props.orientation;
    if (orientation === 'horizontal') return 'X';
    if (orientation === 'vertical') return 'Y';
  },

  _orthogonalAxisKey: function () {
    var orientation = this.props.orientation;
    if (orientation === 'horizontal') return 'Y';
    if (orientation === 'vertical') return 'X';
  },

  _posMinKey: function () {
    var orientation = this.props.orientation;
    if (orientation === 'horizontal')
      return this.props.invert ? 'right' : 'left';
    if (orientation === 'vertical') return this.props.invert ? 'bottom' : 'top';
  },

  _posMaxKey: function () {
    var orientation = this.props.orientation;
    if (orientation === 'horizontal')
      return this.props.invert ? 'left' : 'right';
    if (orientation === 'vertical') return this.props.invert ? 'top' : 'bottom';
  },

  _sizeKey: function () {
    var orientation = this.props.orientation;
    if (orientation === 'horizontal') return 'clientWidth';
    if (orientation === 'vertical') return 'clientHeight';
  },

  _trimAlignValue: function (val, props) {
    return this._alignValue(this._trimValue(val, props), props);
  },

  _trimValue: function (val, props) {
    props = props || this.props;

    if (val <= props.min) val = props.min;
    if (val >= props.max) val = props.max;

    return val;
  },

  _alignValue: function (val, props) {
    props = props || this.props;

    var valModStep = (val - props.min) % props.step;
    var alignValue = val - valModStep;

    if (Math.abs(valModStep) * 2 >= props.step) {
      alignValue += valModStep > 0 ? props.step : -props.step;
    }

    return parseFloat(alignValue.toFixed(5));
  },

  _renderHandle: function (style, child, i) {
    var self = this;
    var className =
      this.props.handleClassName +
      ' ' +
      (this.props.handleClassName + '-' + i) +
      ' ' +
      (this.state.index === i ? this.props.handleActiveClassName : '');

    return React.createElement(
      'div',
      {
        ref: function (r) {
          self['handle' + i] = r;
        },
        key: 'handle' + i,
        className: className,
        style: style,
        onMouseDown: this._createOnMouseDown(i),
        onTouchStart: this._createOnTouchStart(i),
        onFocus: this._createOnKeyDown(i),
        tabIndex: 0,
        role: 'slider',
        'aria-valuenow': this.state.value[i],
        'aria-valuemin': this.props.min,
        'aria-valuemax': this.props.max,
        'aria-label': isArray(this.props.ariaLabel)
          ? this.props.ariaLabel[i]
          : this.props.ariaLabel,
        'aria-valuetext': this.props.ariaValuetext,
      },
      child
    );
  },

  _renderHandles: function (offset) {
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

  _renderBar: function (i, offsetFrom, offsetTo) {
    var self = this;
    return React.createElement('div', {
      key: 'bar' + i,
      ref: function (r) {
        self['bar' + i] = r;
      },
      className:
        this.props.barClassName + ' ' + this.props.barClassName + '-' + i,
      style: this._buildBarStyle(offsetFrom, this.state.upperBound - offsetTo),
    });
  },

  _renderBars: function (offset) {
    var bars = [];
    var lastIndex = offset.length - 1;

    bars.push(this._renderBar(0, 0, offset[0]));

    for (var i = 0; i < lastIndex; i++) {
      bars.push(this._renderBar(i + 1, offset[i], offset[i + 1]));
    }

    bars.push(
      this._renderBar(lastIndex + 1, offset[lastIndex], this.state.upperBound)
    );

    return bars;
  },

  _onSliderMouseDown: function (e) {
    if (this.props.disabled) return;
    this.hasMoved = false;
    if (!this.props.snapDragDisabled) {
      var position = this._getMousePosition(e);
      this._forceValueFromPosition(
        position[0],
        function (i) {
          this._start(i, position[0]);
          this._fireChangeEvent('onChange');
          this._addHandlers(this._getMouseEventMap());
        }.bind(this)
      );
    }

    pauseEvent(e);
  },

  _onSliderClick: function (e) {
    if (this.props.disabled) return;

    if (this.props.onSliderClick && !this.hasMoved) {
      var position = this._getMousePosition(e);
      var valueAtPos = this._trimAlignValue(
        this._calcValue(this._calcOffsetFromPosition(position[0]))
      );
      this.props.onSliderClick(valueAtPos);
    }
  },

  _fireChangeEvent: function (event) {
    if (this.props[event]) {
      this.props[event](undoEnsureArray(this.state.value));
    }
  },

  render: function () {
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

    return React.createElement(
      'div',
      {
        ref: function (r) {
          self.slider = r;
        },
        style: { position: 'relative' },
        className: props.className + (props.disabled ? ' disabled' : ''),
        onMouseDown: this._onSliderMouseDown,
        onClick: this._onSliderClick,
      },
      bars,
      handles
    );
  },
});

export default ReactSlider;
