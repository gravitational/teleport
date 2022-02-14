// paginateData breaks the data array up into chunks the length of pageSize
export default function paginateData(
  data = [],
  pageSize = 10
): Array<Array<any>> {
  const pageCount = Math.ceil(data.length / pageSize);
  const pages = [];

  for (let i = 0; i < pageCount; i++) {
    const start = i * pageSize;
    const page = data.slice(start, start + pageSize);
    pages.push(page);
  }

  // If there are no items, place an empty page inside pages
  if (pages.length === 0) {
    pages[0] = [];
  }

  return pages;
}
