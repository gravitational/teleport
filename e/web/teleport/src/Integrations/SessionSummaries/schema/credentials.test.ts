import { cloudCredentials, selfHostedCredentials } from './credentials';

describe('cloud', () => {
  describe('bedrock integration', () => {
    it('validates bedrock integration mode', () => {
      const input = {
        accessMethod: 'bedrock',
        bedrockMode: 'integration',
        integrationName: 'test-integration',
        region: 'us-east-1',
      };

      const result = cloudCredentials.safeParse(input);

      expect(result.success).toBe(true);
    });

    it('rejects missing region', () => {
      const input = {
        accessMethod: 'bedrock',
        bedrockMode: 'integration',
        integrationName: 'my-integration',
        region: '',
      };

      const result = cloudCredentials.safeParse(input);

      expect(result.success).toBe(false);
      expect(result.error.issues[0].message).toBe('Region is required');
    });

    it('rejects missing integrationName', () => {
      const input = {
        accessMethod: 'bedrock',
        bedrockMode: 'integration',
        integrationName: '',
        region: 'us-east-1',
      };

      const result = cloudCredentials.safeParse(input);

      expect(result.success).toBe(false);
      expect(result.error.issues[0].message).toBe(
        'Integration name is required'
      );
    });
  });

  describe('teleport', () => {
    it('validates teleport access method', () => {
      const input = {
        accessMethod: 'teleport',
      };

      const result = cloudCredentials.safeParse(input);

      expect(result.success).toBe(true);
    });
  });

  describe('OpenAI', () => {
    it('validates with new key', () => {
      const input = {
        accessMethod: 'openai_api',
        apiKeyMode: 'new',
        apiKey: 'sk-' + 'a'.repeat(50),
      };

      const result = cloudCredentials.safeParse(input);

      expect(result.success).toBe(true);
    });

    it('validates with existing secret', () => {
      const input = {
        accessMethod: 'openai_api',
        apiKeyMode: 'existing',
        secretName: 'my-openai-secret',
      };

      const result = cloudCredentials.safeParse(input);

      expect(result.success).toBe(true);
    });

    it('rejects api key without sk- prefix', () => {
      const input = {
        accessMethod: 'openai_api',
        apiKeyMode: 'new',
        apiKey: 'invalid-key-' + 'a'.repeat(50),
      };

      const result = cloudCredentials.safeParse(input);

      expect(result.success).toBe(false);
      expect(result.error.issues[0].message).toBe(
        'OpenAI API keys must start with "sk-"'
      );
    });

    it('rejects api key that is too short', () => {
      const input = {
        accessMethod: 'openai_api',
        apiKeyMode: 'new',
        apiKey: 'sk-short',
      };

      const result = cloudCredentials.safeParse(input);

      expect(result.success).toBe(false);
      expect(result.error.issues[0].message).toBe('Invalid OpenAI API key');
    });

    it('rejects missing secretName for existing key mode', () => {
      const input = {
        accessMethod: 'openai_api',
        apiKeyMode: 'existing',
        secretName: '',
      };

      const result = cloudCredentials.safeParse(input);

      expect(result.success).toBe(false);
      expect(result.error.issues[0].message).toBe('Secret name is required');
    });
  });

  describe('OpenAI compatible API', () => {
    it('validates with new key', () => {
      const input = {
        accessMethod: 'openai_compatible',
        apiKeyMode: 'new',
        apiKey: 'some-api-key',
        apiUrl: 'https://api.example.com/v1',
      };

      const result = cloudCredentials.safeParse(input);

      expect(result.success).toBe(true);
    });

    it('validates with existing secret', () => {
      const input = {
        accessMethod: 'openai_compatible',
        apiKeyMode: 'existing',
        secretName: 'my-secret',
        apiUrl: 'https://api.example.com/v1',
      };

      const result = cloudCredentials.safeParse(input);

      expect(result.success).toBe(true);
    });

    it('rejects invalid URL', () => {
      const input = {
        accessMethod: 'openai_compatible',
        apiKeyMode: 'new',
        apiUrl: 'not-a-valid-url',
      };

      const result = cloudCredentials.safeParse(input);

      expect(result.success).toBe(false);
      expect(result.error.issues[0].message).toBe('Must be a valid URL');
    });
  });
});

describe('self hosted', () => {
  describe('bedrock', () => {
    it('validates integration mode', () => {
      const input = {
        accessMethod: 'bedrock',
        bedrockMode: 'integration',
        integrationName: 'test-integration',
        region: 'us-west-2',
      };

      const result = selfHostedCredentials.safeParse(input);

      expect(result.success).toBe(true);
    });

    it('validates direct mode', () => {
      const input = {
        accessMethod: 'bedrock',
        bedrockMode: 'direct',
        region: 'eu-west-1',
      };

      const result = selfHostedCredentials.safeParse(input);

      expect(result.success).toBe(true);
    });

    it('validates inference_profile mode', () => {
      const input = {
        accessMethod: 'bedrock',
        bedrockMode: 'inference_profile',
        region: 'ap-northeast-1',
      };

      const result = selfHostedCredentials.safeParse(input);

      expect(result.success).toBe(true);
    });

    it('rejects missing region for direct mode', () => {
      const input = {
        accessMethod: 'bedrock',
        bedrockMode: 'direct',
        region: '',
      };

      const result = selfHostedCredentials.safeParse(input);

      expect(result.success).toBe(false);
      expect(result.error.issues[0].message).toBe('Region is required');
    });

    it('rejects missing integrationName for integration mode', () => {
      const input = {
        accessMethod: 'bedrock',
        bedrockMode: 'integration',
        integrationName: '',
        region: 'us-east-1',
      };

      const result = selfHostedCredentials.safeParse(input);

      expect(result.success).toBe(false);
      expect(result.error.issues[0].message).toBe(
        'Integration name is required'
      );
    });
  });

  describe('OpenAI', () => {
    it('validates with new key', () => {
      const input = {
        accessMethod: 'openai_api',
        apiKeyMode: 'new',
        apiKey: 'sk-' + 'x'.repeat(50),
      };

      const result = selfHostedCredentials.safeParse(input);

      expect(result.success).toBe(true);
    });

    it('rejects api key without sk- prefix', () => {
      const input = {
        accessMethod: 'openai_api',
        apiKeyMode: 'new',
        apiKey: 'invalid-key-' + 'x'.repeat(50),
      };

      const result = selfHostedCredentials.safeParse(input);

      expect(result.success).toBe(false);
      expect(result.error.issues[0].message).toBe(
        'OpenAI API keys must start with "sk-"'
      );
    });

    it('rejects api key that is too short', () => {
      const input = {
        accessMethod: 'openai_api',
        apiKeyMode: 'new',
        apiKey: 'sk-short',
      };

      const result = selfHostedCredentials.safeParse(input);

      expect(result.success).toBe(false);
      expect(result.error.issues[0].message).toBe('Invalid OpenAI API key');
    });

    it('rejects missing secretName for existing key mode', () => {
      const input = {
        accessMethod: 'openai_api',
        apiKeyMode: 'existing',
        secretName: '',
      };

      const result = selfHostedCredentials.safeParse(input);

      expect(result.success).toBe(false);
      expect(result.error.issues[0].message).toBe('Secret name is required');
    });
  });

  describe('OpenAI compatible API', () => {
    it('validates with new key and optional apiKey', () => {
      const input = {
        accessMethod: 'openai_compatible',
        apiKeyMode: 'new',
        apiUrl: 'http://localhost:8080/v1',
      };

      const result = selfHostedCredentials.safeParse(input);

      expect(result.success).toBe(true);
    });

    it('rejects invalid URL', () => {
      const input = {
        accessMethod: 'openai_compatible',
        apiKeyMode: 'new',
        apiUrl: 'not-a-valid-url',
      };

      const result = selfHostedCredentials.safeParse(input);

      expect(result.success).toBe(false);
      expect(result.error.issues[0].message).toBe('Must be a valid URL');
    });

    it('rejects missing secretName for existing key mode', () => {
      const input = {
        accessMethod: 'openai_compatible',
        apiKeyMode: 'existing',
        secretName: '',
        apiUrl: 'https://api.example.com/v1',
      };

      const result = selfHostedCredentials.safeParse(input);

      expect(result.success).toBe(false);
      expect(result.error.issues[0].message).toBe('Secret name is required');
    });
  });
});
