import React from 'react';
import PropTypes from 'prop-types';
import styled from 'styled-components';
import { Flex, Text } from 'design';

const LicenseTextArea = React.forwardRef((props, ref) => {
  const {text, ...styles} = props;
  return (
    <Flex bg="grey.50" alignItems="start" style={{borderRadius: "6px"}} {...styles} >
      <StyledCmd ref={ref} typography="body2" px="3" py="3" color="text.onLight">{text}</StyledCmd>
    </Flex>
  )
})

const StyledCmd = styled(Text)`
  font-family: ${({theme})=> theme.fonts.mono};
  word-break: break-all;
  word-wrap: break-word;
  border-radius: 6px;
  white-space: pre;
  word-break: break-all;
  word-wrap: break-word;
`;

LicenseTextArea.propTypes = {
  ...Flex.propTypes,
  cmd: PropTypes.string
}

export default LicenseTextArea;