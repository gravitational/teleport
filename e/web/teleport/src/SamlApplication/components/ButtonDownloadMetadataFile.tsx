import { useCallback } from 'react';

import { Box, ButtonBorder, LabelInput } from 'design';
import { Download } from 'design/Icon';
import { useAsync } from 'shared/hooks/useAsync';
import { saveOnDisk } from 'shared/utils/saveOnDisk';

import useTeleportE from 'e-teleport/useTeleportE';

export function ButtonDownloadMetadataFile() {
  const { idpService } = useTeleportE();
  const [saveMetadataXmlAttempt, runSaveMetadataXml] = useAsync(
    useCallback(async () => {
      const resp = await idpService.getMetadataXml();
      saveOnDisk(resp, 'teleport-saml-idp-metadata.xml', 'application/xml');
    }, [idpService])
  );

  const labelText = 'Failed to fetch SAML IdP metadata file';
  return (
    <Box>
      {saveMetadataXmlAttempt.status === 'error' && (
        <LabelInput hasError={true}>{labelText}</LabelInput>
      )}
      <ButtonBorder
        gap={2}
        onClick={runSaveMetadataXml}
        size="medium"
        px={3}
        textTransform="none"
        disabled={saveMetadataXmlAttempt.status === 'processing'}
      >
        Download Metadata File
        <Download size="medium" />
      </ButtonBorder>
    </Box>
  );
}
