import { fireEvent, render, screen, waitFor } from 'design/utils/testing';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { ContextProvider } from 'teleport';

import { ButtonDownloadMetadataFile } from './ButtonDownloadMetadataFile';

test('button download metadata file', async () => {
  const ctx = createTeleportContextE();
  jest.spyOn(console, 'error').mockImplementation();
  const fakeFetchXml = jest.fn();
  ctx.idpService.getMetadataXml = fakeFetchXml;
  render(
    <ContextProvider ctx={ctx}>
      <ButtonDownloadMetadataFile />
    </ContextProvider>
  );

  const downloadButton = screen.getByRole('button', {
    name: /Download Metadata File/i,
  });
  fireEvent.click(downloadButton);
  expect(fakeFetchXml).toHaveBeenCalled();

  // test error label
  ctx.idpService.getMetadataXml = jest.fn().mockRejectedValue(() => {
    throw new Error('request error');
  });
  fireEvent.click(downloadButton);
  await waitFor(() => {
    expect(
      screen.getByText('Failed to fetch SAML IdP metadata file')
    ).toBeInTheDocument();
  });
});
