# Invariants

A module at the Exec stage is well formed when it satisfies the rules of [Code IR](../code-ir/invariants.md) and
those of this chapter. The module of the abstract machine with its compositions and every module that conversion from
the Flow stage produces are verified, and so is every module that a pass produces; a backend prints only a verified
module.

## The Stage

- Every operation that a function calls belongs to the Exec dialect or the Action dialect. No operation of the Flow
  dialect remains: conversion from the Flow stage replaces each by a primitive, and verification at the Exec stage
  reports one that is left.

## Cleanups

The cleanup of a `Protect` ends normally and changes nothing but its own locals and the state that operations
restore:

- it holds no `Exit`, `Break`, `Continue`, `Return`, `Loop`, `Protect` or `Assert`;
- it assigns only the locals that it declares, and no register;
- it reads no register;
- it calls only operations, never a function, and no operation that may end the parse or that reads or writes a
  register.
