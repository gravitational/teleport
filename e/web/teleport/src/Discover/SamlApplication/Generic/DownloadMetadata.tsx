import { IdpMetadata } from 'e-teleport/SamlApplication/components/IdpMetadata';
import {
  ActionButtons,
  Header,
  HeaderSubtitle,
} from 'teleport/Discover/Shared';
import { useDiscover } from 'teleport/Discover/useDiscover';

export function DownloadMetadata() {
  const { prevStep, nextStep, isUpdateFlow } = useDiscover();
  return (
    <>
      <Header>
        Configure Service Provider with Teleport's Identity Provider Metadata
      </Header>
      <HeaderSubtitle>
        In order to use Teleport as an Identity Provider for your SAML
        application, you must configure your service provider to recognize
        Teleport's IdP metadata.
      </HeaderSubtitle>
      <IdpMetadata />
      {/* TODO(sshah): update useDiscover to return prevStep based on updateFlow */}
      <ActionButtons
        onProceed={nextStep}
        onPrev={isUpdateFlow ? null : prevStep}
      />
    </>
  );
}
