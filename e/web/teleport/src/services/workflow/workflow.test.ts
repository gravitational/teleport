import Workflow from './workflow';
import api from 'teleport/services/api';

test('fetch access requests', async () => {
  // Test null response.
  jest.spyOn(api, 'get').mockResolvedValue(null);

  const workflow = new Workflow();
  const response = await workflow.fetchAccessRequests({});

  expect(response).not.toBeNull();
  expect(response).toHaveLength(0);
});
