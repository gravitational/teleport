---
authors: Ryan Clark (ryan.clark@goteleport.com)
state: draft
---

# RFD 0206 - Stricter linting for the frontend

## Required Approvers

* Engineering: (@ravicious || @kimlisa || @avatus) && @zmb3

## What

Add strict type checking with TypeScript to the frontend codebase, as well as enabling stricter, type checked
ESLint rules.

## Why

The frontend codebase is pretty relaxed when it comes to type checking, which can cause bugs to surface during runtime
instead of during compile time.

Enabling TypeScript's strict mode could help catch potential issues earlier in the development process, as well as
helping to improve code readability and maintainability.

ESLint's TypeScript plugin offers strict rules that also use type checking to catch even more potential issues, further
helping to improve code quality.

Together, these changes can help to improve the overall quality of the codebase, reduce potential bugs from happening at
runtime, as well as improve the development experience.

## Current issues

At the moment, the frontend codebase wouldn't catch a lot of potential issues, such as
trying to access a property or a method on a variable that is `undefined` or `null`:

```typescript
let x: string | undefined;

if (x.length > 0) {
  // do something
}
```

This would currently compile, but would throw an error at runtime if `x` is `undefined`.

With strict type checking, this would throw an error at compile time, as `x` is `undefined` and doesn't have a `split`
method.

```typescript
let x: string | undefined;

if (x.length > 0) { // error TS18048: 'x' is possibly 'undefined'.
  // do something
}
```

## TypeScript's strict mode

TypeScript's strict mode is a combination of multiple flags that enable stricter type checking:

### strictNullChecks

When `strictNullChecks` is not enabled, `false`, `null` and `undefined` are effectively ignored by the TypeScript
compiler. This can cause issues like
the one above, where a variable that is `undefined` is accessed as if it is a string.

Another example is when searching an array for a result. The return type of `Array.prototype.find()` when
`strictNullChecks` is not enabled is `T`,
which results in the type checker assuming that a result is always found.

```typescript
const users = [
  { name: 'Lisa', githubUsername: 'kimlisa' },
  { name: 'Michael', githubUsername: 'avatus' },
  { name: 'Rafal', githubUsername: 'ravicious' },
  { name: 'Ryan', githubUsername: 'ryanclark' },
];

const zac = users.find(user => user.name === 'Zac');

console.log(`Zac's GitHub username is ${zac.githubUsername}`);
```

When `strictNullChecks` is enabled, `null` and `undefined` are treated as their own, distinct types. This means that
TypeScript will throw a type error if you try to access a property or method on a variable that is `null` or
`undefined`.

```typescript
const users = [
  { name: 'Lisa', githubUsername: 'kimlisa' },
  { name: 'Michael', githubUsername: 'avatus' },
  { name: 'Rafal', githubUsername: 'ravicious' },
  { name: 'Ryan', githubUsername: 'ryanclark' },
];

const zac = users.find(user => user.name === 'Zac');

console.log(`Zac's GitHub username is ${zac.githubUsername}`); // error TS18048: 'zac' is possibly 'undefined'.
```

### strictBindCallApply

This rule ensures that any arguments passed to `Function.prototype.bind`, `Function.prototype.call` and
`Function.prototype.apply` are checked against the function's type signature.

We don't really use these methods a lot in the codebase, but if we do, this rule will help to ensure that the arguments
passed to them are valid.

```typescript
function fn(x: string) {
  return parseInt(x);
}

const n1 = fn.call(undefined, "10");

const n2 = fn.call(undefined, false); // error TS2345: Argument of type 'boolean' is not assignable to parameter of type 'string'.
```

### strictBuiltinIteratorReturn

Similar to the above, this is a rule for something we don't use a lot, but if we do, it will help to ensure that the
return type of the iterator is checked against the function's type signature, instead
of being `any`.

### strictFunctionTypes

This flag enables stricter checking of function types.

For example, currently, if you have a function that is expecting a string, with a custom type signature that allows
either a number or a string, TypeScript would allow you to pass a number to the function.

```typescript
function fn(x: string) {
  console.log("Hello, " + x.toLowerCase());
}

type StringOrNumberFunc = (ns: string | number) => void;

let func: StringOrNumberFunc = fn;

func(10);
````

This could lead to bugs surfacing due to a type mismatch, with no enforcement. This could have happened during a
refactor, where the function signature was changed to accept a string, but the type signature was not updated.

With `strictFunctionTypes` enabled, TypeScript will throw an error if you try to pass a number to a function that is
expecting a string.

```typescript
function fn(x: string) {
  console.log("Hello, " + x.toLowerCase());
}

type StringOrNumberFunc = (ns: string | number) => void;

let func: StringOrNumberFunc = fn;
// Type '(x: string) => void' is not assignable to type 'StringOrNumberFunc'.
//  Types of parameters 'x' and 'ns' are incompatible.
//  Type 'string | number' is not assignable to type 'string'.
//  Type 'number' is not assignable to type 'string'.
````

### noImplicitAny

`any` is a dangerous type, as it can lead to runtime errors if the type is not what you expect. This rule stops
TypeScript from inferring `any` when it
cannot determine the type of an expression.

Currently, code like this would compile:

```typescript
function fn(s) {
  console.log(s.subtr(3));
}

fn(42);
```

This would throw an error at runtime, as `s` is a number and doesn't have a `subtr` method.

`noImplicitAny` would throw an error at compile time, as `s` is not typed and TypeScript cannot infer the type.

```typescript
function fn(s) { // error TS7006: Parameter 's' implicitly has an 'any' type.
  console.log(s.subtr(3));
}
````

### noImplicitThis

This can help catch common pitfalls when using `this` in functions. Currently, if you have a function that is expecting
`this` to be a certain type, but it is not, TypeScript will not throw an error.

```typescript
class Rectangle {
  width: number;
  height: number;

  constructor(width: number, height: number) {
    this.width = width;
    this.height = height;
  }

  getAreaFunction() {
    return function() {
      return this.width * this.height; // `this` is not equal `Rectangle` here due to being inside a `function`
    }
  }
}
```

This could cause a bug at runtime, as `this` is not what it was expected to be.

`noImplicitThis` would throw an error at compile time, as `this` is not typed and TypeScript cannot infer the type.

```typescript
class Rectangle {
  width: number;
  height: number;

  constructor(width: number, height: number) {
    this.width = width;
    this.height = height;
  }

  getAreaFunction() {
    return function() {
      return this.width * this.height;
      // 'this' implicitly has type 'any' because it does not have a type annotation.
      // 'this' implicitly has type 'any' because it does not have a type annotation.
    };
  }
}
```

### useUnknownInCatchVariables

Currently, in a `try`/`catch` block, `error` is typed as `any`, which can lead to runtime errors if the type is not what
you expect.

Errors that are thrown during execution can be anything - `Error`, an object, a string, etc. This can lead to bugs
surfacing at runtime if
an unknown error is thrown.

`useUnknownInCatchVariables` would throw an error at compile time, as `error` would be typed as `unknown`, which is a
safer type than `any`.

```typescript
try {
  // do something
} catch (error) {
  console.log(error.message); // error TS18046: 'error' is of type 'unknown'.

  if (error instanceof Error) {
    console.log(error.message); // this is fine as it's checking if `error` is an instance of `Error`, so it has a `message` property
  }
}
```

## ESLint's strict rules

Without going into the same level of detail as above, ESLint's strict type checked rules can offer a lot of benefits to
the
codebase, such as:

### @typescript-eslint/await-thenable

This avoids using `await` on a non-promise, which can lead to runtime errors if the value is not a promise.

### @typescript-eslint/no-explicit-any

This prevents the use of `any` in the codebase, which can lead to runtime errors if the type is not what you expect.

### @typescript-eslint/no-unnecessary-condition

This prevents asserting conditions that are type checked to always be `true` or `false`.

### @typescript-eslint/no-unnecessary-type-assertion

This prevents type assertions when the type is already what is being asserted. This can reduce the amount of type
assertions in the codebase (which can be a possible introduction of bugs) and improve readability.

### @typescript-eslint/no-misused-promises

This prevents some common pitfalls when using promises, such as forgetting to await for a promise to resolve, or using a
promise in a place where it is not expected.

### @typescript-eslint/no-non-null-assertion

This prevents usage of `!` in the codebase to assert to the compiler that a value is not `null` or `undefined`. This can
lead to runtime errors if the value is actually `null` or `undefined`. Instead of using `!`, the code should be
refactored to
ensure that the value is not `null` or `undefined` before using it.

### and more...

There are a lot of other rules in ESLint's TypeScript plugin that can help to improve the quality of the codebase. The
full list [is here](https://github.com/typescript-eslint/typescript-eslint/blob/main/packages/eslint-plugin/src/configs/strict-type-checked.ts).

There are also [stylistic type checked rules](https://github.com/typescript-eslint/typescript-eslint/blob/main/packages/eslint-plugin/src/configs/stylistic-type-checked.ts)
which can help to improve the consistency and readability of the codebase.

## Migration plan

These rules cannot be enabled all at once, or even one-by-one, as there would be thousands of errors which would be hard
to fix in one PR and backport, and could also potentially disrupt other pull requests in flight.

Instead, we can move the existing codebase over to a legacy directory in one large swoop, and then enable the strict
changes for the new directories, left in the same place as the old codebase was. We would then move code over from
the legacy folder to the new folder, fixing the strict errors as we go. Backporting changes should be relatively easy,
as the diff to existing code should be small (ideally only import changes to point to the new codebase).

Due to the way the TypeScript compiler works, it is not possible to have strictly typed code import non-strictly typed
code, without also strictly checking the non-strictly typed code. This means that we cannot have a mix of strict and
non-strict code in the same directory.

It is possible for the non-strict code to import strictly typed code, so as we migrate parts of the codebase over, the
existing codebase can import and use the code from the new location.

When moving code over, instead of deleting the legacy file once finished, the file should instead export all the
exports from the new file, marked with a `@deprecated` comment. This will minimize the disruptions when updating 
Teleport E, as import changes will not break the E build, as E can still point to the legacy locations.

Once the migration is complete, we would be left with the codebase being strictly typed.

Whilst we introduce a new design component system (outlined
in [RFD 196](https://github.com/gravitational/teleport/pull/53157)) (something else that would also require the
separation of code into a new directory during a migration), it
is a great opportunity to also add strict type checking.

### Current directory structure

```
- e
  - web
    - teleport
- web
  - packages
    - design
    - shared
    - teleport
    - teleterm
```

### Proposed directory structure

```
- e
  - web
    - new
    - teleport
- web
  - packages
    - legacy
      - design
      - shared
      - teleport
      - teleterm

    - design
    - shared
    - teleport
    - teleterm
```

### Importing between codebases

All the existing code can import the new code, through imports such as `@gravitational/teleport/services/auth`, etc. As services,
pages and components are moved over, the import paths would be updated to point to where they live in the new
folders, and the old code would be removed.

### Migration process

- Identify a part of the codebase to migrate over
- Copy the code over from the legacy directory to the new directory
- Update the import paths in the new codebase to point to the new directory
- Fix the strict type errors in the new codebase
- Move the components over to use the new design system (if applicable)
- Update the stories and tests for the migrated code
- Run the part of the web test plan for major releases against the migrated code
  - If there is not a section for the migrated code, create a new section in the web test plan for the migrated code 
    (only necessary when moving over pages, not services or types etc)
- Pull request the changes
- Backport the changes

To avoid disruption to developer's actual work, this work could be done as a side project or palette cleanser, maybe on 
a Friday afternoon or during some downtime.

### A joint effort

It would be nice to have everyone involved in the migration process, so that everyone can learn about the new codebase,
stricter linting and type checking, and how to work with it.

An offsite could be a great way to kick off the migration process, where we can all work together to migrate a part of the codebase over, 
as well as support each other in the process.

This would be tracked on a GitHub project, so everyone can see what is being worked on, and what is left to do.

### Downsides

- The codebase is split between two different directories, which could be confusing for developers who are used to the old
  structure.
    - Detailed documentation and training sessions can help mitigate this, so everyone is on the same page.
    - Each part of the codebase that needs to be migrated over can be tracked in a GitHub project, and people can take
      ownership of parts of the codebase to migrate over, so that it doesn't fall on one person to do all the work.
    - This can be a great opportunity for developers to learn about parts of the codebase they
      are not familiar with, and can help to improve the overall quality of the codebase.
    - This can also help developers improve their TypeScript skills, as they will be working with strict type
      checking and ESLint rules.
- The migration would take a while
    - This is true, but the outcome is worth it - there aren't a lot of opportunities to introduce strict type checking
      into a codebase, and the move to the new design system presents a great opportunity to do so.
