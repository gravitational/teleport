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
import styled, {keyframes} from 'styled-components';
import { Box, Flex, Text } from 'design';
import theme from 'design/theme';

export function CircleGraph(props){
  const { title, subtitles=[], current, ...rest} = props;

  const $subtitles = subtitles.map( (value, index) => (
    <Text key={index} color="text.primary" mx="2" typography="body2">
      {value}
    </Text>
  ))

  return (
    <Container bg="primary.main" borderRadius="3" {...rest}>
      <Text brtr={8} brtl={8} as={Flex} color="text.primary" typography="h5" height="56px" bg="primary.light" alignItems="center" px="3">
        {title}
      </Text>
      <Chart>
        <Graph viewBox="0 0 36 36">
          <GraphBackground
            d="M18 2.0845
              a 15.9155 15.9155 0 0 1 0 31.831
              a 15.9155 15.9155 0 0 1 0 -31.831"
            strokeDasharray="80, 80"
          />
          <GraphProgress
            current={current}
            strokeDasharray={`${current * .80}, 80`}
            d="M18 2.0845
              a 15.9155 15.9155 0 0 1 0 31.831"
          />
        </Graph>
        <CurrentPecentage>{current}%</CurrentPecentage>
      </Chart>
      <Flex px="2" pb="3" mt="1" alignItems="center" justifyContent="center">
        {$subtitles}
      </Flex>
    </Container>
  );
}

const stroke = props => {
  const { colors } = props.theme;
  if(props.current < 50) {
    return {stroke: colors.success}
  }

  if(props.current < 80) {
    return {stroke: colors.warning}
  }

  return {stroke: colors.danger}
}

const progress = keyframes`
  0% {stroke-dasharray: 0 100;}
`

const Chart = styled.div`
  box-sizing: border-box;
  height: 176px;
  min-width: 200px;
  padding-top: 32px;
  position: relative;
`

const Graph = styled.svg`
  display: block;
  margin: 0 auto;
  max-width: 80%;
  max-height: 120px;
  transform: rotate(215deg);
`
const GraphBackground = styled.path`
  fill: none;
  stroke: ${props => props.theme.colors.primary.dark};
  stroke-width: 4;
  stroke-linecap: round;
`

const GraphProgress = styled.path`
  stroke-linecap: round;
  animation: ${progress} 1s ease-in;
  fill: none;
  stroke-width: 4;
  ${stroke}
  transition: all 1s;
`
const CurrentPecentage = styled.h3`
  font-size: 28px;
  font-weight: 300;
  margin: 0;
  line-height: 48px;
  position: absolute;
  margin-top: -20px;
  top: 50%;
  left: 0;
  right: 0;
  text-align: center;
`

const Container = styled(Box)`
  overflow: hidden;
  box-shadow: 0 0 32px rgba(0, 0, 0, .12), 0 8px 32px rgba(0, 0, 0, .24);
  max-width: 250px;
  flex-shrink: 0;
  flex-grow: 1;
`

CircleGraph.defaultProps = {
  theme: theme,
  title: 'Title',
  current: 0,
  avg: 0,
  high: 0,
  nodes: 0,
}

CircleGraph.displayName = 'CircleGraph';

export default CircleGraph;