import { Header, HeaderSubtitle } from "teleport/Discover/Shared";
import { DesktopInstance, DynamicWindowsDesktop } from "./DynamicWindowsDesktop";
import { ActionButtonPrimary, ActionButtonSecondary } from "teleport/Account/Header";
import { useState } from "react";
import Flex from "design/Flex";
import Box from "design/Box";
import { useDiscover } from "teleport/Discover/useDiscover";

export default function Container() {
  const { nextStep } = useDiscover();
  const [desktops, setDesktops] = useState<DesktopInstance[]>([]);

  const addNewDesktop = () => {
    if (desktops.length < 5) {
      const newDesktop: DesktopInstance = {
        id: crypto.randomUUID(),
        // TEST
        desktopUri: 'desktop.example.com',
        desktopPort: '3389',
        labels: [],
        connectionState: 'idle'
      };
      setDesktops(prev => [...prev, newDesktop]);
    }
  };

  const updateDesktop = (id: string, updates: Partial<DesktopInstance>) => {
    setDesktops(prev => prev.map(desktop => 
      desktop.id === id ? { ...desktop, ...updates } : desktop
    ));
  };

  const removeDesktop = (id: string) => {
    setDesktops(prev => prev.filter(desktop => desktop.id !== id));
  };

  const finishText = desktops.length ? "And Add Desktops" : "Without Adding Desktops";

  const finishFlow = () => {
    nextStep();
  };

  return (
    <>
      <Heading />
      <Flex gap={2} flexDirection='column'>
        {desktops.map(desktop => (
          <DynamicWindowsDesktop
            key={desktop.id}
            desktopInstance={desktop}
            onUpdate={updateDesktop}
            onRemove={removeDesktop}
          />
        ))}
      </Flex>
      <Box>
          <ActionButtonSecondary
            mt={2}
            mr={2}
            onClick={addNewDesktop}
            disabled={desktops.length >= 5}
          >
            Add Desktop Resource
          </ActionButtonSecondary>
        <ActionButtonPrimary
          mt={4}
          onClick={finishFlow}
        >
          Finish {finishText}
        </ActionButtonPrimary>
      </Box>
    </>
  );
}

const Heading = () => (
  <>
    <Header>Connect Windows Desktops</Header>
    <HeaderSubtitle>
      Optionally, you can also add up to 5 Dynamic Windows Desktop resources for your Windows Desktop Service to discover.
    </HeaderSubtitle>
  </>
);