import {useHistory, useParams} from 'react-router';

import {ButtonPrimary, Flex, H2} from 'design';

import cfg, { UrlDesktopParams } from 'teleport/config';
import Dialog from "design/Dialog";

export function LinuxDesktopSelector() {
  const { username, desktopName, clusterId } = useParams<UrlDesktopParams>();

  const history = useHistory();

  let sessionUrl = (sessionName: string) =>
    cfg.getLinuxDesktopRoute({
      username,
      desktopName,
      clusterId,
      sessionName,
    });

  return (
    <Dialog dialogCss={() => ({ width: '400px' })} open={true}>
      <Flex flexDirection={"column"} alignItems="center" mb={4}>
        <H2>Select session</H2>
        <ButtonPrimary onClick={()=>history.push(sessionUrl('first'))}>First available</ButtonPrimary>
      </Flex>
    </Dialog>
  );
}
