import Client from './client';
import { arrayBufFirst2Pngs } from './fixtures';

const tdpClient = new Client('wss://socketAddr', 'username');

test('only the first png frame causes a "connect" event to be emitted', () => {
  let i = 0;
  let connectEmitted = false;

  // Set up listener to check for connect event.
  tdpClient.on('connect', () => {
    connectEmitted = true;
    expect(i).toEqual(0); // check only emitted on the first frame
  });

  arrayBufFirst2Pngs.forEach(pngFrame => {
    tdpClient.processMessage(pngFrame);
    i++;
  });

  expect(connectEmitted).toEqual(true); // check that connect was emitted at all
});
