import type {
  InfiniteData,
  UseInfiniteQueryResult,
} from '@tanstack/react-query';
import { useCallback, useMemo, useState, type ReactNode } from 'react';
import { Link } from 'react-router';
import styled from 'styled-components';

import { Alert } from 'design/Alert';
import Box from 'design/Box';
import ButtonIcon from 'design/ButtonIcon';
import Flex from 'design/Flex';
import { CircleArrowLeft } from 'design/Icon';
import { Indicator } from 'design/Indicator';
import { getErrorMessage } from 'shared/utils/error';

interface ItemResponse<TData> {
  items?: TData[];
}

interface PaginatedListProps<TResponse extends ItemResponse<TData>, TData> {
  query: UseInfiniteQueryResult<InfiniteData<TResponse>>;
  rowRenderer: (item: TData) => ReactNode;
}

export function PaginatedList<TResponse extends ItemResponse<TData>, TData>({
  query,
  rowRenderer,
}: PaginatedListProps<TResponse, TData>) {
  const [page, setPage] = useState(0);

  const pages = query.data?.pages.length ?? 0;

  const items = useMemo(() => {
    if (!query.data) {
      return [];
    }

    return query.data.pages[page]?.items?.map(rowRenderer) ?? [];
  }, [page, query.data, rowRenderer]);

  const handlePreviousPage = useCallback(() => {
    if (page === 0) {
      return;
    }

    setPage(prevPage => prevPage - 1);
  }, [page]);

  const handleNextPage = useCallback(async () => {
    if (query.isFetchingNextPage) {
      return;
    }

    if (!query.data?.pages[page + 1]) {
      await query.fetchNextPage();
    }

    setPage(prevPage => prevPage + 1);
  }, [query, page]);

  if (query.isPending) {
    return (
      <Container alignItems="center" justifyContent="center" py={4}>
        <Indicator delay="none" />
      </Container>
    );
  }

  if (query.isError) {
    return <Alert kind="danger">{getErrorMessage(query.error)}</Alert>;
  }

  if (items.length === 0) {
    return (
      <Container>
        <Box textAlign="center" p={4}>
          No items found.
        </Box>
      </Container>
    );
  }

  return (
    <Container>
      <Flex flexDirection="column" gap={0} flex={1}>
        {items}
      </Flex>

      <Footer>
        <Box color="text.slightlyMuted" fontWeight="400">
          Page {page + 1}
        </Box>

        <Spacer />

        <ButtonIcon
          aria-label="Previous page"
          disabled={page === 0}
          onClick={handlePreviousPage}
        >
          <CircleArrowLeft />
        </ButtonIcon>

        <ButtonIcon
          aria-label="Next page"
          disabled={
            (page === pages - 1 && !query.hasNextPage) ||
            query.isFetchingNextPage
          }
          onClick={() => {
            void handleNextPage();
          }}
          ml={2}
        >
          <CircleArrowLeft style={{ transform: 'rotate(180deg)' }} />
        </ButtonIcon>
      </Footer>
    </Container>
  );
}

const Spacer = styled.div`
  flex: 1;
`;

const Footer = styled.div`
  display: flex;
  align-items: center;
  padding: ${p => p.theme.space[2]}px ${p => p.theme.space[3]}px;
`;

export const ItemLink = styled(Link)`
  display: flex;
  padding: ${p => p.theme.space[2]}px ${p => p.theme.space[3]}px;
  text-decoration: none;
  color: ${p => p.theme.colors.text.main};
  // avoid the border from changing color on hover by mixing the equivalent tonal.neutral alpha into the page background
  border-bottom: 1px solid
    color-mix(
      in srgb,
      ${p => p.theme.colors.text.main} 7%,
      ${p => p.theme.colors.levels.sunken}
    );

  &:hover {
    background-color: ${p => p.theme.colors.interactive.tonal.neutral[0]};
  }
`;

const Container = styled(Flex)`
  border: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[0]};
  border-radius: 8px;
  flex-direction: column;
  gap: 0;
  margin-top: ${p => p.theme.space[2]}px;
  overflow: hidden;
`;
