/**
 * pipe takes an array of unary functions as an argument. It returns a function that accepts a value
 * that's going to be passed through the supplied functions.
 *
 * @example
 * // Without pipe.
 * const add1ThenDouble = (x) => double(add1(x));
 *
 * // With pipe.
 * const add1ThenDouble = pipe(add1, double);
 */
export const pipe =
  <PipeSubject>(...fns: Array<(pipeSubject: PipeSubject) => PipeSubject>) =>
  (x: PipeSubject) =>
    fns.reduce((v, f) => f(v), x);
