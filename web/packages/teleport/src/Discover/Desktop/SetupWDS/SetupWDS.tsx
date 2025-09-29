import { H3, Subtitle3 } from "design/Text";
import { JSX, Suspense, useEffect, useState } from "react";
import Validation, { Validator } from "shared/components/Validation";
import { ActionButtons, Header, HeaderSubtitle, LabelsCreater, ResourceKind, StyledBox, TextIcon } from "teleport/Discover/Shared";
import { ResourceLabel } from "teleport/services/agents";
import * as Icons from 'design/Icon';
import { ButtonSecondary } from "design/Button";
import { Alert, Box } from "design";
import { useDiscover } from "teleport/Discover/useDiscover";
import { TextSelectCopyMulti } from "shared/components/TextSelectCopy";
import { useJoinTokenSuspender } from "teleport/Discover/Shared/useJoinTokenSuspender";
import cfg from "teleport/config";
import { PingTeleportProvider, usePingTeleport } from "teleport/Discover/Shared/PingTeleportContext";
import { useShowHint } from "teleport/Discover/Shared/useShowHint";
import { Node } from 'teleport/services/nodes';
import { AgentStepProps } from "teleport/Discover/types";
import { Desktop } from "teleport/services/desktops";

export default function Container(props: AgentStepProps) {
  const [labels, setLabels] = useState<ResourceLabel[]>([]);
  const [showScript, setShowScript] = useState(false);

  function toggleShowScript(validator: Validator) {
    if (!validator.validate()) {
      return;
    }
    setShowScript(!showScript);
  }

  return (
    <>
      <Suspense
        fallback={
          <>
        <Heading />
          <StepOne
            labels={labels}
            setLabels={setLabels}
            onShowScript={toggleShowScript}
            showScript={showScript}
            onPrev={props.prevStep}
            processing={true}
          />
          </>
        }
      >
        <Heading />
        <StepOne
          labels={labels}
          setLabels={setLabels}
          onShowScript={toggleShowScript}
          showScript={showScript}
          onPrev={props.prevStep}
        />
        {showScript && <StepTwo {...props} labels={labels}/>}
      </Suspense>
    </>
  );
}

const Heading = () => (
  <>
    <Header>Setup Windows Desktop Service</Header>
    <HeaderSubtitle>
      Install and configure the Teleport Windows Desktop Service
    </HeaderSubtitle>
  </>
);

interface StepOneProps {
  labels: ResourceLabel[];
  setLabels(l: ResourceLabel[]): void;
  error?: Error;
  onShowScript(validator: Validator): void;
  showScript: boolean;
  processing?: boolean;
  onPrev(): void 
}

function StepOne({labels, setLabels, error, onShowScript, showScript, processing, onPrev}: StepOneProps) {
  const nextLabelTxt = labels.length
    ? 'Finish Adding Labels'
    : 'Skip Adding Labels';

  return (
    <>
      <StyledBox mb={5}>
        <header>
          <H3>Step 1 (Optional)</H3>
          <Subtitle3>Add labels that will be used to filter discovered desktops</Subtitle3>
        </header>
        <Validation>
            {({ validator }) => (
              <>
                <LabelsCreater
                  labels={labels}
                  setLabels={setLabels}
                  isLabelOptional={true}
                  noDuplicateKey={true}
                  disableBtns={showScript && !error}
                />
                {error && (
                  <TextIcon mt={2} mb={3}>
                    <Icons.Warning
                      size="medium"
                      ml={1}
                      mr={2}
                      color="error.main"
                    />
                    Encountered Error: {error.message}
                  </TextIcon>
                )}
                <Box mt={3}>
                  <ButtonSecondary
                    width="200px"
                    type="submit"
                    onClick={() => onShowScript(validator)}
                    disabled={processing}
                  >
                    {showScript && !error ? 'Edit Labels' : nextLabelTxt}
                  </ButtonSecondary>
                </Box>
              </>
            )}
        </Validation>
      </StyledBox>
      {(!showScript || processing || error) && (
        <ActionButtons
          onProceed={() => null}
          disableProceed={true}
          onPrev={onPrev}
        />
      )}
    </>
  );
}

function StepTwo(props: AgentStepProps & { labels: ResourceLabel[] }) {
  // Fetch join token
  const { joinToken } = useJoinTokenSuspender({
    resourceKinds: [ResourceKind.Desktop],
    suggestedLabels: props.labels,
  });

  // Starts resource querying interval
  // const { result, active } = usePingTeleport<Desktop>(joinToken);

  // TEST
  const [result, setResult] = useState(false);
  const [active, setActive] = useState(true);
  useEffect(() => {
    const timer = window.setTimeout(() => {
      setResult(true);
    }, 2000);

    return () => window.clearTimeout(timer);
  }, []);

  function handleNextStep() {
    // TEST
    props.updateAgentMeta({
      ...props.agentMeta,
      desktop: {
        kind: 'windows_desktop',
        os: 'windows',
        name: 'name',
        addr: '127.0.0.1',
        labels: [],
        logins: [],
      }
    });

    props.nextStep();
  }

  // Show connection hint after timeout
  const showHint = useShowHint(active);

  let hint: JSX.Element;
  if (showHint && !result) {
    hint = (
      <Alert kind="warning">
        We&apos;re still looking for your Windows Desktop Service.
      </Alert>
    )
  } else if (result) {
    hint = (
      <Alert kind="success">
        Successfully detected your new Teleport instance.
      </Alert>
    );
  } else {
    hint = (
      <Alert kind="neutral" icon={Icons.Restore}>
        After running the command above, we&apos;ll automatically detect your
        new Teleport instance.
      </Alert>
    );
  }

  return (
    <>
      {joinToken && (
        <>
          <StyledBox mb={5}>
            <header>
              <H3>Step 2</H3>
              <Subtitle3 mb={3}>
                Run the following command on the server you want to add
              </Subtitle3>
            </header>
            <TextSelectCopyMulti
              lines={[{ text: createBashCommand(joinToken.id) }]}
            />
          </StyledBox>
          {hint}
          <ActionButtons
            onProceed={handleNextStep}
            onPrev={props.prevStep}
            disableProceed={!result}
          />
        </>
      )}
    </>
  )
}

function createBashCommand(tokenId: string) {
  return `sudo bash -c "$(curl -fsSL ${cfg.getNodeScriptUrl(tokenId)})"`;
}