import { useCallback, useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router';

import { useAsync } from 'shared/hooks/useAsync';

import cfg from 'e-teleport/config';
import useTeleportE from 'e-teleport/useTeleportE';
import { AuthConnectorEditorContent } from 'teleport/AuthConnectors/AuthConnectorEditor';
import { KindAuthConnectors } from 'teleport/services/resources';

import templates from '../templates';

export function AuthConnectorEditor({ isNew = false }) {
  const { connectorType, connectorName } = useParams<{
    connectorType: KindAuthConnectors;
    connectorName: string;
  }>();
  const ctx = useTeleportE();
  const navigate = useNavigate();

  if (!connectorType || (!isNew && !connectorName)) {
    return null;
  }

  const [content, setContent] = useState(templates[connectorType]);
  const [initialContent, setInitialContent] = useState(
    templates[connectorType]
  );

  const [fetchAttempt, fetchConnector] = useAsync(
    useCallback(async () => {
      if (!isNew) {
        const res = await ctx.resourceService.fetchConnector(
          connectorType,
          connectorName
        );
        setContent(res.content);
        setInitialContent(res.content);
      }
      return;
    }, [connectorName, connectorType, ctx.resourceService, isNew])
  );

  const [saveAttempt, saveConnector] = useAsync(async () => {
    if (isNew) {
      await ctx.resourceService
        .createConnector(connectorType, content)
        .then(() => navigate(cfg.oss.routes.sso));
    } else {
      await ctx.resourceService
        .updateConnector(connectorType, connectorName, content)
        .then(() => navigate(cfg.oss.routes.sso));
    }
  });

  const isSaveDisabled =
    saveAttempt.status === 'processing' || content === initialContent;

  useEffect(() => {
    if (fetchAttempt.status !== 'success') {
      fetchConnector();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const backButtonRoute = isNew
    ? cfg.routes.ssoNewConnectorList
    : cfg.oss.routes.sso;

  let title;
  if (isNew) {
    switch (connectorType) {
      case 'github':
        title = 'Creating new Auth Connector (GitHub): ';
        break;
      case 'saml':
        title = 'Creating new Auth Connector (SAML): ';
        break;
      case 'oidc':
        title = 'Creating new Auth Connector (OIDC): ';
        break;
    }
  } else {
    title = `Editing Auth Connector: ${connectorName}`;
  }

  return (
    <AuthConnectorEditorContent
      title={title}
      content={content}
      backButtonRoute={backButtonRoute}
      isSaveDisabled={isSaveDisabled}
      saveAttempt={saveAttempt}
      fetchAttempt={fetchAttempt}
      onSave={saveConnector}
      onCancel={() => navigate(backButtonRoute)}
      setContent={setContent}
    />
  );
}
