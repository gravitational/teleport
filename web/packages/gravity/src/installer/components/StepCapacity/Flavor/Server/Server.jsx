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
import { ServerVarEnums } from 'gravity/services/enums';
import { values } from 'lodash';
import FieldMount from './FieldMount';
import FieldInterface from './FieldInterface';
import { Box, LabelInput, Text, Flex } from 'design';

export function Server({ hostname, vars, onSetVars, onRemoveVars, role, ...styles }){

  React.useEffect(() => {
    function cleanup(){
      onRemoveVars({ role, hostname });
    }

    return cleanup;
  }, [])

  const varValues = React.useMemo(() => ({
    ip: null,
    mounts: {}
  }), []);


  function notify() {
    onSetVars({
      role,
      hostname,
      ip: varValues.ip,
      mounts: values(varValues.mounts)
    })
  }

  function onChangeIp(ip){
    varValues.ip = ip;
    notify()
  }

  function onSetMount({ value, name }){
    varValues.mounts[name] = {
      value,
      name
    }

    notify()
  }

  const $vars = vars.map( (v, index) => {
    if(v.type === ServerVarEnums.INTERFACE){
      const { value, options } = v;
      return (
        <FieldInterface
          key={index}
          {...varBoxProps}
          maxWidth="200px"
          defaultValue={ value }
          options={options}
          onChange={onChangeIp}
        />
      )
    }

    if(v.type === ServerVarEnums.MOUNT){
      const { value, name } = v;
      return (
        <FieldMount
          key={index}
          {...varBoxProps}
          defaultValue={value}
          name={name}
          onChange={onSetMount}
        />
      )
    }

    return null;
  })

  return (
    <StyledServer {...styles}>
      <Box mr="4">
        <LabelInput>
          Hostname
        </LabelInput>
        <Text typography="h5">
          {hostname}
        </Text>
      </Box>
      <Flex flexWrap="wrap" flex="1" justifyContent="flex-end">
        {$vars}
      </Flex>
    </StyledServer>
  )
}

const StyledServer = styled(Flex)`
  border-top: 1px solid ${ ({ theme }) => theme.colors.primary.dark };
`

const varBoxProps = {
  ml: "3",
  flex: "1",
  minWidth: "180px",
}




