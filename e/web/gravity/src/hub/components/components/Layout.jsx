import React from 'react';
import styled from 'styled-components';
import { Flex, Text } from 'design';

const FeatureHeader = styled(Flex)`
  flex-shrink: 0;
`

FeatureHeader.defaultProps = {
  my: 4,
}

const FeatureHeaderTitle = props => (
  <Text typography="h1" as="h1"  {...props} />
)

const FeatureBox = styled(Flex)`
  overflow: auto;
  width: 100%;
  height: 100%;
  ::after { content: ' '; padding-bottom: 24px; }
`

FeatureBox.defaultProps = {
  px: 6,
  flexDirection: "column"
}

const AppVerticalSplit = styled.div`
  position: absolute;
  width: 100%;
  height: 100%;
  display: flex;
`

const AppHorizontalSplit = styled.div`
  display: flex;
  flex-direction: column;
  width: 100%
`

export {
  AppHorizontalSplit,
  AppVerticalSplit,
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle
}