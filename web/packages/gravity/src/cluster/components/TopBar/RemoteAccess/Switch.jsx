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

import React from 'react'
import styled from 'styled-components';
import PropTypes from 'prop-types';
import { darken, fade } from 'design/theme/utils/colorManipulator';

export default function Switch({checked, onChange, ...styles}) {
  const [ focused, { onFocused, onBlur } ] = useFocus();
  const [ hovered, { onMouseEnter, onMouseLeave } ] = useHover();
  return (
    <StyledSwitch
      {...styles}
      checked={checked}
      focused={focused}
      hovered={hovered}
      onMouseEnter={onMouseEnter}
      onMouseLeave={onMouseLeave}
    >
      <input type="checkbox" onFocus={onFocused} onBlur={onBlur} checked={checked} onChange={onChange}/>
      <span className="slider" />
    </StyledSwitch>
  )
}

Switch.propTypes = {
  onChange: PropTypes.func.isRequired,
  checked: PropTypes.bool.isRequired,
}

function fromTheme({theme, checked, focused, hovered}){
  const color = checked ? theme.colors.secondary.main : theme.colors.disabled;
  const brightColor = fade(color, 0.4);

  let sliderColor = color;
  if(focused, hovered){
    sliderColor = darken(color, 0.2)
  }

  return {
    color: brightColor,
    backgroundColor: brightColor,
    '.slider': {
      backgroundColor: sliderColor,
      boxShadow: checked ? 'rgba(0, 191, 165, 0.24) 0px 0px 0px, rgba(0, 191, 165, 0.56) 0px 4px 16px' : 'none'
    }
  }
}

const StyledSwitch = styled.label`
  position: relative;
  display: inline-flex;
  align-items: center;
  width: 36px;
  height: 18px;
  border-radius: 20px;
  cursor: pointer;

  input {
    width: 100%;
    height: 100%;
    cursor: inherit;
    margin: 0;
    opacity: 0;
    padding: 0;
    position: absolute;
  }

  input:checked + .slider {
    transform: translateX(calc(36px - 18px));
  }

  .slider {
    position: absolute;
    height: 18px;
    width: 18px;
    border-radius: 50%;
    transition: .4s;
  }

  ${fromTheme}
`

const useFocus = () => {
  const [focused, set] = React.useState(false)
  return [
    focused,
    {
      onFocus: () => set(true),
      onBlur: () => set(false),
    },
  ]
}

const useHover = () => {
  const [hovered, set] = React.useState(false)
  return [
    hovered,
    {
      onMouseEnter: () => set(true),
      onMouseLeave: () => set(false),
    },
  ]
}