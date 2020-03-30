/* eslint react/no-danger: 0 */

import React from 'react';
import { Box, Flex, ButtonWarning, Text } from 'design';
import Dialog, { DialogContent, DialogFooter } from 'design/DialogConfirmation';
import { Warning } from 'design/Icon';

type LicenseWarningProps = {
  html: string;
  onClose: () => void;
};

export default function LicenseWarning(props: LicenseWarningProps) {
  return (
    <Dialog
      disableEscapeKeyDown={true}
      onClose={props.onClose}
      open={true}
      bg="white"
      dialogCss={dialogCss}
    >
      <Box width="540px">
        <DialogContent>
          <Flex alignItems="center">
            <Warning mr="2" fontSize={36} color="warning" />
            <Text typography="h1">Warning!</Text>
          </Flex>
          <Text typography="paragraph" mt="2" mb="4">
            <div dangerouslySetInnerHTML={{ __html: props.html }} />
          </Text>
        </DialogContent>
        <DialogFooter>
          <ButtonWarning mr="3" onClick={props.onClose}>
            Disregard and continue
          </ButtonWarning>
        </DialogFooter>
      </Box>
    </Dialog>
  );
}

const dialogCss = ({ theme }: any): any => ({
  a: {
    color: theme.colors.light,
  },
});
