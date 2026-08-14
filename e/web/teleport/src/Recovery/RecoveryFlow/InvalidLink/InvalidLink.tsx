import { Card, H1, P1 } from 'design';

export default function InvalidLink() {
  return (
    <Card
      width="540px"
      color="text.main"
      p={6}
      bg="levels.elevated"
      mt={6}
      mx="auto"
    >
      <H1 textAlign="center" mb={3}>
        Invalid Recovery Link
      </H1>
      <P1 textAlign="center">This recovery link is invalid or has expired.</P1>
      <P1 textAlign="center">
        If you believe this is a mistake, please contact us at:
        support@goteleport.com
      </P1>
    </Card>
  );
}
