import { ResourceKind } from "../Shared";
import { PingTeleportProvider } from "../Shared/PingTeleportContext";
import { PING_INTERVAL } from "./config"

export function DesktopWrapper(props: DesktopWrapperProps) {
  return (
    <PingTeleportProvider
      interval={PING_INTERVAL}
      resourceKind={ResourceKind.Server}
    >
      {props.children}
    </PingTeleportProvider>
  );
}

interface DesktopWrapperProps {
  children: React.ReactNode;
}