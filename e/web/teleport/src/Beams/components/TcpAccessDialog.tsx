import {
  ButtonSecondary,
  Flex,
  P2,
  Subtitle2,
} from '@gravitational/design-system';

import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';

import { CodeBlock } from 'e-teleport/Beams/BeamsQuickstart/components/CodeBlock';

export function TcpAccessDialog({
  appName,
  vnetUrl,
  onClose,
}: {
  appName: string;
  vnetUrl: string;
  onClose: () => void;
}) {
  return (
    <Dialog open onClose={onClose}>
      <DialogHeader>
        <DialogTitle>Accessing a published TCP app</DialogTitle>
      </DialogHeader>
      <DialogContent width="540px">
        <Flex flexDirection="column" gap={4}>
          <Flex flexDirection="column" gap={2}>
            <Subtitle2>Using a local proxy</Subtitle2>
            <P2>Run the following in your terminal:</P2>
            <CodeBlock command={`tsh proxy app ${appName}`} />
            <P2>
              This starts a local proxy, where you can connect your client to
              the address it prints.
            </P2>
          </Flex>

          <Flex flexDirection="column" gap={2}>
            <Subtitle2>Using Teleport VNet</Subtitle2>
            <P2>Run the following in your terminal:</P2>
            <CodeBlock command="tsh vnet" />
            <P2>Then, connect to your TCP application at:</P2>
            <CodeBlock command={vnetUrl} prompt="" />
          </Flex>

          <Flex flexDirection="column" gap={2}>
            <Subtitle2>Using Teleport Connect</Subtitle2>
            <P2>
              Alternatively, use Teleport Connect to start virtual network
              emulation using VNet and manage running local proxies.
            </P2>
          </Flex>
        </Flex>
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={onClose}>Close</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
