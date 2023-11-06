import React from 'react'
import styled from 'styled-components'
import Box from "design/Box"
import * as Icons from 'design/Icon';
import { ButtonPrimary, ButtonSecondary } from "design/Button"
import Flex from "design/Flex"
import Text from 'design/Text';

export const ByobCta = () => {
  return (
    <CtaContainer mb="4">
      <Flex justifyContent="space-between">
        <Flex mr="4" alignItems="center">
          <Icons.Server size="medium" mr="3" />
          <Box>
            <Text bold>Bring Your Own (BYOB) S3</Text>
            <Text>Provides the ability to store and view session recordings and audit logs on AWS S3
              buckets that are managed outside of Teleport Cloud. </Text>
          </Box>
        </Flex>
        <Flex alignItems="center">
          {/* TODO: onclick */}
          <ButtonPrimary width="200px" mr="2">Manage Data Storage</ButtonPrimary>
          <ButtonSecondary width="170px">Learn More</ButtonSecondary>
        </Flex>
      </Flex>
    </CtaContainer>
  )
}

const CtaContainer = styled(Box)`
background-color: ${props => props.theme.colors.spotBackground[0]};
padding: ${props => `${props.theme.space[3]}px`};
border: 1px solid ${props => props.theme.colors.spotBackground[2]};
border-radius: ${props => `${props.theme.space[2]}px`};
`