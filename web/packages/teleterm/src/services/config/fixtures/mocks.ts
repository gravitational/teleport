import { AppConfig, ConfigService } from 'teleterm/services/config';

export function createMockConfigService(
  providedValues: AppConfig
): ConfigService {
  const values = { ...providedValues };
  return {
    get(key) {
      return { value: values[key], metadata: { isStored: false } };
    },
    set(key, value) {
      values[key] = value;
    },
    getStoredConfigErrors() {
      return [];
    },
  };
}
