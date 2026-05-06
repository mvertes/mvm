# ADR-004: Two-phase compilation with pre-allocated slots

**Status:** accepted (revised; var-init ordering refined by ADR-015)
**Date:** 2024-01-15 (revised 2026-03, 2026-05)

## Context

Go allows out-of-order top-level declarations: a function can reference a
type or variable declared later in the source. A single-pass compiler
cannot resolve forward references without either a pre-pass or a retry
mechanism.

The original design used a retry (fixpoint) loop in both phases. Phase 2
retries required rollback machinery (`savedSlots`, code/data length
checkpoints) that was fragile and hard to reason about.

## Decision

The compiler uses a two-phase approach. Only Phase 1 retries; Phase 2
runs straight through with no retries.

**Phase 1 -- Declarations.** `ScanDecls` splits the source into top-level
declarations. Each is passed to `ParseDecl`, which resolves `package`,
`import`, `const`, `type`, and `var` (type registration only) and
registers function and method signatures via `registerFunc` without
parsing bodies. Declarations that fail with `ErrUndefined` are retried
until convergence. Rollback is lightweight: only `SymTracker` keys are
deleted from the symbol table (no code or data is emitted in Phase 1).

**Phase 2 -- Code generation.** After Phase 1:

1. `goparser.expandVarBlocks` flattens `var(...)` blocks into individual
   declarations; init-order analysis itself runs at the comp layer
   (see [ADR-015](ADR-015-var-init-dep-analysis-in-comp.md)).
2. `allocGlobalSlots` pre-assigns a `Data` slot for every `Var` and
   `Func` symbol, so code generation never encounters `UnsetAddr`.
3. Each var initializer compiles into its own bytecode buffer (in a
   compile-order topo so RHS type inference sees its deps' Types),
   while emit-time tracking records the slot references each buffer
   makes.
4. Func bodies and top-level statements compile into `c.Code` in
   source order.
5. A fixed-point on the recorded direct-refs converts them to
   transitive Var-only read sets; the var-init buffers are
   topologically sorted by those reads and prepended to `c.Code`,
   with Func entry-points shifted by the prefix length.

Because all symbols have pre-allocated indices and var ordering is
derived from emitted bytecode, Phase 2 needs no retries and no
rollback machinery.

## Consequences

**Easier:**
- No rollback machinery in Phase 2 -- `savedSlots`, code/data length
  checkpoints, and string cache cleanup are all removed.
- Bytecode-derived var ordering covers method calls, qualified imports,
  and indirect dispatch (see [ADR-015](ADR-015-var-init-dep-analysis-in-comp.md))
  -- something the previous parser-side token walker could not do.
- Works naturally with incremental REPL evaluation.
- Method signatures are registered in Phase 1 alongside plain functions,
  improving forward reference coverage.

**Harder:**
- Phase 1 worst case is still O(n^2) in the number of declarations
  (each round resolves at least one). Acceptable for typical program sizes.
- The dependency analysis adds an emit-time hook on `c.emit()` plus a
  fixed-point pass over the recorded direct-refs. Both run once per
  compile and are linear in code size.
