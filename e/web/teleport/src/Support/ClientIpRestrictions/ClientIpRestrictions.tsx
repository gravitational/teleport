import {
  ChangeEvent,
  ReactNode,
  useCallback,
  useEffect,
  useState,
} from 'react';
import { useTheme } from 'styled-components';

import { Alert as BaseAlert } from 'design/Alert';
import { AlertProps } from 'design/Alert/Alert';
import Box from 'design/Box';
import { Button } from 'design/Button';
import Flex from 'design/Flex';
import { ListAddCheck } from 'design/Icon';
import { Indicator } from 'design/Indicator';
import { TextArea } from 'design/TextArea';
import { Theme } from 'design/theme';
import { HoverTooltip } from 'design/Tooltip';
import {
  InfoExternalTextLink,
  InfoGuideButton,
  InfoParagraph,
  InfoUl,
} from 'shared/components/SlidingSidePanel/InfoGuide';
import { useAsync } from 'shared/hooks/useAsync';

import useTeleportE from 'e-teleport/useTeleportE';
import { SupportSectionCard } from 'teleport/Support/Support';

const EDITOR_PLACEHOLDER = `Enter one CIDR block per line, e.g.:
100.20.56.0/32
104.18.0.0/16`;

export const ClientIpRestrictions = ({ clusterId }: { clusterId: string }) => {
  const theme = useTheme();

  const ctx = useTeleportE();
  const access = ctx.storeUser.geClientIpRestrictionAccess();

  const canEdit = access.edit && access.create;

  const [editing, setEditing] = useState(false);
  const [allowList, setAllowList] = useState<string[]>([]);
  const [showSuccess, setShowSuccess] = useState(false);
  const [error, setError] = useState<string>(null);

  const [listAttempt, listRun] = useAsync(
    useCallback(async () => {
      const res =
        await ctx.clientIpRestrictionsService.fetchClientIpRestrictions(
          clusterId
        );
      setAllowList(res);
      return res;
    }, [clusterId, ctx.clientIpRestrictionsService])
  );

  useEffect(() => {
    if (!listAttempt.status && clusterId && access.list) {
      listRun().then(([, error]) => {
        setError(error?.message);
      });
    }
  }, [listAttempt.status, listRun, clusterId, access.list]);

  const editorContent = allowList?.join('\n');

  const handleChange = (e: ChangeEvent<HTMLInputElement>) => {
    const val = e.target.value;
    setAllowList(val?.split('\n'));
  };

  const [saveAttempt, saveRun] = useAsync(async () => {
    await ctx.clientIpRestrictionsService.saveClientIpRestrictions(
      clusterId,
      allowList.filter(s => s.trim() !== '')
    );
  });
  const handleSave = async () => {
    const [, error] = await saveRun();
    if (!error) {
      setEditing(false);
      setShowSuccess(true);
    }
    setError(error?.message);
  };

  const loading =
    listAttempt.status === 'processing' || saveAttempt.status === 'processing';

  if (!access.list) {
    return null;
  }

  return (
    <SupportSectionCard
      title="IP Allowlist"
      icon={<ListAddCheck />}
      titleAction={<InfoGuideButton config={{ guide: <InfoGuide /> }} />}
      gridColumn="1 / -1"
      transition="0.2s"
    >
      <Flex flex="1" data-testid="text-editor-container">
        <TextArea
          disabled={!editing || loading}
          bg={!editing ? 'buttons.bgDisabled' : 'levels.sunken'}
          value={editorContent}
          placeholder={EDITOR_PLACEHOLDER}
          style={{
            maxHeight: 240,
            minHeight: 36,
            height: getEditorHeight(editorContent, theme),
          }}
          onChange={handleChange}
          name="allowlist"
        />
      </Flex>
      <Box mt="3">
        {!editing && (
          <HoverTooltip
            tipContent={
              canEdit
                ? ''
                : 'Insufficient permissions. Reach out to your Teleport administrator'
            }
          >
            <Button
              disabled={!!error || loading || !canEdit}
              onClick={() => setEditing(true)}
              role="button"
            >
              {loading ? <Indicator size="small" delay="none" /> : 'Edit'}
            </Button>
          </HoverTooltip>
        )}
        {editing && (
          <Button disabled={loading} onClick={handleSave} role="button">
            Save
          </Button>
        )}
      </Box>
      {error && (
        <Alert kind="danger" onDismiss={() => setError(null)}>
          {error}
        </Alert>
      )}
      {!error && showSuccess && (
        <Alert kind="success" onDismiss={() => setShowSuccess(false)}>
          Allowlist updated
        </Alert>
      )}
    </SupportSectionCard>
  );
};

const getEditorHeight = (editorContent: string, theme: Theme): number => {
  const editorPadding = theme.space[4];
  const editorPxPerLine = theme.fontSizes[7];
  const qtyLines =
    editorContent === ''
      ? EDITOR_PLACEHOLDER.split('\n').length
      : editorContent?.split('\n').length;
  return qtyLines * editorPxPerLine + editorPadding;
};

const InfoGuide = () => (
  <Box>
    <InfoParagraph>
      <b>Client IP Allowlist</b> restricts access to your Teleport Cloud
      cluster, allowing traffic only from the specified network ranges (CIDR
      blocks).
    </InfoParagraph>

    <InfoParagraph>Changes are recorded in the audit log.</InfoParagraph>

    <InfoParagraph>Add one CIDR block per line, without commas.</InfoParagraph>

    <InfoParagraph>
      Changes take effect in 5–15 minutes and will terminate existing
      connections. Maintenance windows are ignored.
    </InfoParagraph>

    <InfoParagraph>
      <b>Limitations and Notes</b>
      <InfoUl>
        <li>
          Misconfiguration can block all access to your cluster. Make sure to
          include your current network before saving changes.
        </li>
        <li>
          Teleport does not auto-add third-party service ranges. Add all
          required CIDRs explicitly.
        </li>
        <li>
          The allowlist applies to Teleport Cloud access; it does not replace
          your organization’s network/firewall policies.
        </li>
        <li>
          Up to 256 CIDR blocks are allowed. If you need more,{' '}
          <InfoExternalTextLink href="https://support.goteleport.com">
            create a Support Ticket
          </InfoExternalTextLink>
        </li>
      </InfoUl>
    </InfoParagraph>

    <InfoParagraph>
      Not sure what a CIDR block is? See{' '}
      <InfoExternalTextLink href="https://en.wikipedia.org/wiki/Classless_Inter-Domain_Routing">
        this short primer
      </InfoExternalTextLink>
      .
    </InfoParagraph>
  </Box>
);

const Alert = ({
  kind,
  children,
  ...rest
}: {
  kind: 'danger' | 'success';
  children: ReactNode;
} & AlertProps) => (
  <BaseAlert kind={kind} dismissible mb="0" mt="3" {...rest}>
    {children}
  </BaseAlert>
);
