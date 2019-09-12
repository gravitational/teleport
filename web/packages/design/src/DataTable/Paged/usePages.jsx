/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';

export default function usePages({ pageSize = 7, data }){
  const [ startFrom, setFrom ] = React.useState(0);

  // set current page to 0 when data source length changes
  React.useEffect(() => {
    setFrom(0);
  }, [data.length]);

  function onPrev(){
    let prevPage = startFrom - pageSize;
    if(prevPage < 0){
      prevPage = 0;
    }
    setFrom(prevPage);
  }

  function onNext(){
    let nextPage = startFrom + pageSize;
    if(nextPage < data.length){
      nextPage = startFrom + pageSize;
      setFrom(nextPage);
    }
  }

  const totalRows = data.length;

  let endAt = 0;
  let pagedData = data;

  if (data.length > 0){
    endAt = startFrom + (pageSize > data.length ? data.length : pageSize);
    if(endAt > data.length){
      endAt = data.length;
    }

    pagedData = data.slice(startFrom, endAt);
  }

  return {
    pageSize,
    startFrom,
    endAt,
    totalRows,
    data: pagedData,
    hasPages: totalRows > pageSize,
    onNext,
    onPrev
  }
}