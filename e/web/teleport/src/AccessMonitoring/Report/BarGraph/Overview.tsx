import styled from 'styled-components';

interface OverviewProps {
  counts: Map<string, number>;
  colors: (params: { id: string }) => string;
  selected: string;
  onSelect: (id: string) => void;
}

const Container = styled.div`
  display: flex;
  padding: 0 16px;
  gap: 4px;
  flex-wrap: wrap;
`;

const Item = styled.div<{ selected?: boolean }>`
  cursor: pointer;
  border-radius: 7px;
  padding: 4px 8px;
  display: flex;
  align-items: center;
  gap: 8px;
  background: ${p => (p.selected ? p.theme.colors.spotBackground[2] : 'none')};
  user-select: none;

  &:hover {
    background: ${p =>
      p.selected
        ? p.theme.colors.spotBackground[1]
        : p.theme.colors.spotBackground[0]};
  }
`;

const ItemColor = styled.div`
  width: 16px;
  height: 16px;
  border-radius: 4px;
`;

const ItemDetails = styled.div`
  display: flex;
  align-items: center;
  gap: 16px;
`;

const ItemName = styled.div`
  font-weight: bold;
`;

const ItemValue = styled.div`
  color: ${p => p.theme.colors.text.muted};
`;

export function Overview(props: OverviewProps) {
  // sort props.counts by value
  const sortedCounts = Array.from(props.counts.entries()).sort(
    (a, b) => b[1] - a[1]
  );

  const items = sortedCounts.map(([key, value]) => (
    <Item
      key={key}
      selected={props.selected === key}
      onClick={() => props.onSelect(key)}
    >
      <ItemColor style={{ background: props.colors({ id: key }) }} />

      <ItemDetails>
        <ItemName>{key}</ItemName>

        <ItemValue>{value}</ItemValue>
      </ItemDetails>
    </Item>
  ));

  return <Container>{items}</Container>;
}
