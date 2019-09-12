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
import ReactSelect from 'react-select';
import styled from 'styled-components';

export default function Select({hasError, ...props}){
  return (
    <StyledSelect hasError={hasError}>
      <ReactSelect
        className="react-select-container"
        classNamePrefix="react-select"
        clearable={false}
        placeholder="Select..."
        {...props}
      />
    </StyledSelect>
  )
}

const StyledSelect = styled.div`
  .react-select__control,
  .react-select__control--is-focused {
    min-height: 40px;
    ${ ({ hasError, theme}) => {
      if(hasError){
        return {
          borderRadius: 'inherit !important',
          borderWidth: '2px !important',
          border: `2px solid ${theme.colors.error.main}  !important`,
        }
      }
    }}
  }

  .react-select-container {
    box-shadow: inset 0 2px 4px rgba(0,0,0,.24);
    box-sizing: border-box;
    border: none;
    display: block;
    font-size: 16px;
    outline: none;
    width: 100%;
    color: rgba(0,0,0,0.87);
    background-color: #FFFFFF;
    margin-bottom: 0px;
    border-radius: 4px;
  }

  .react-select__menu{
    margin-top: 0px;
  }

  react-select__menu-list {
  }

  .react-select__indicator-separator{
    display: none;
  }

  .react-select__control {
    &:hover {
      border-color: transparent;
    }
  }

  .react-select__control--is-focused {
    background-color: transparent;
    border-color: transparent;
    border-radius: 4px;
    border-style: solid;
    border-width: 1px;
    box-shadow: none;
  }

  .react-select__option--is-selected{
    //background-color: white;
    background-color: #cfd8dc;
    color: inherit;
  }
 `