import { useEffect, useRef } from 'react';

// From https://usehooks-ts.com/react-hook/use-unmount

export function useUnmount(func: () => void) {
  const funcRef = useRef(func);

  funcRef.current = func;

  useEffect(
    () => () => {
      funcRef.current();
    },
    []
  );
}
