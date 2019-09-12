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

import React from 'react';
import styled from 'styled-components';
import PropTypes from 'prop-types';
import Slider from './Slider';
import { Box } from 'design';

export default function FlavorSelector(props) {
  const { current, options, onChange, ...rest } = props;
  const total = options.length;

  if(total < 2){
    return null;
  }

  function onSliderChange(value){
    onChange(value-1);
  }

  return (
    <StyledFlavorBox mb="10" {...rest}>
      <Slider
        options={options}
        valueComponent={<FlavorValueComponent options={options}/>}
        min={1}
        max={total}
        value={current+1}
        onChange={onSliderChange}
        defaultValue={1}
        withBars={true}
        className="grv-slider"/>
    </StyledFlavorBox>
  );
}

FlavorSelector.propTypes = {
  current: PropTypes.number.isRequired,
  options: PropTypes.array
}

function Value({offset, marginLeft}) {
  const props = {
    className: 'grv-slider-value',
    style: {
      position: 'absolute',
      left: `${offset}px`,
      marginLeft: `${marginLeft}px`
    }
  }

  return ( <div {...props} /> );
}

function ValueDesc ({offset, width, marginLeft, text}) {
  const props = {
    className: 'grv-slider-value-desc',
    style: {
      width: `${width}px`,
      position: 'absolute',
      marginLeft:`${(width/-2) + marginLeft}px`,
      left: `${offset}px`,
      textAlign: 'center'
    }
  }

  return (
    <div {...props} >
      <span>{text}</span><br/>
    </div>
  )
}

function FlavorValueComponent(props) {
  const {options, handleSize, upperBound, sliderLength} = props;

  const $vals = [];
  const $descriptions = [];
  const count = options.length - 1;

  const widthWithHandle = upperBound / count;
  const widthWithoutHandle = sliderLength / count;
  const marginLeft = handleSize / 2;

  for(var i = 0; i < options.length; i++){
    let offset = widthWithHandle * i;
    let { label } = options[i];

    let valueProps = {
      key: 'value_' + i,
      offset,
      marginLeft
    }

    let descProps = {
      ...valueProps,
      key: 'desc_' + i,
      width: widthWithoutHandle,
      text: label,
    }

    $vals.push(<Value {...valueProps}/>);
    $descriptions.push(<ValueDesc {...descProps}/>);
  }

  return (
    <div>
      {$vals}
      <div className="grv-installer-provision-flavors-range" style={{position: 'absolute', width: '100%'}}>{$descriptions}</div>
    </div>
  );
}

const StyledFlavorBox = styled(Box)`
  .grv-installer-provision-flavors-range {
    padding-top: 15px;
    margin-top: 45px;
    font-size: 13px;
    color: #969696;
  }

  .grv-slider-value {
    width: 3px;
    height: 15px;
    top: -4px;
    z-index: 1;
    background: #DDD;
  }

  .grv-slider-value-desc {
    top: -20px;
  }

  .grv-slider-value-desc:first-child{
    margin-left: 0 !important;
    text-align: start !important;
  }

  .grv-slider-value-desc:last-child{
    right: 0;
    width: auto !important;
    text-align: right !important;
  }

  .grv-slider {
    margin-top: 16px;
    height: 50px;
  }

  .grv-slider .bar {
    height: 6px;
    border-radius: 10px;
  }

  .grv-slider .handle {
    width: 20px;
    height: 20px;
    left: -10px;
    top: -7px;
    border-radius: 14px;
    background: ${ ({theme}) => theme.colors.success};
    box-shadow: rgba(0, 0, 0, 0.2) 0px 1px 3px 1px, rgba(0, 0, 0, 0.14) 0px 2px 2px 0px, rgba(0, 0, 0, 0.12) 0px 3px 1px -2px;
  }

  .grv-slider .handle:after {
  }

  .grv-slider .bar-0 {
    background: none repeat scroll 0 0 ${ ({theme}) => theme.colors.success};
    box-shadow: none;
  }

  .grv-slider .bar-1 {
    background-color: white;
  }

  .grv-slider .grv-installer-provision-flavors-handle {
    width: 50px;
    text-align: center;
    position: absolute;
    font-size: 13px;
    margin-top: -30px;
    margin-left: -13px;
    border-radius: 15%;
  }
`