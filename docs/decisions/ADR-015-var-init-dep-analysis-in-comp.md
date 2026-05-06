# ADR-015: Var-init dependency analysis at the comp layer

**Status:** accepted
**Date:** 2026-05-06

## Context

Go's package-init rule says a `var` whose initializer transitively reads
another `var` must be initialized after the read target. mvm originally
ran this analysis at the parser layer in `goparser/stmt.go` via
`splitAndSortVarDecls` / `sortByDeps` / `collectIdents`: a token-shape
walker that followed bare-Ident references and free-function calls by
name.

That approach missed every dispatch flavour that does not leave a free
identifier at the call site:

- method calls (`R{}.First()` -- `First` is preceded by Period),
- qualified imports (`pkg.Func()` -- same),
- function values stored in a var,
- interface dispatch.

The token walker had no way to distinguish a method on a value from a
field selector, and the parser does not yet have full type info, so
fixing it at the parser layer would mean reimplementing most of the
compiler's symbol resolution. Real packages (`github.com/google/uuid`)
hit these cases routinely, so the parser-side approach was always going
to be incomplete.

## Decision

Move var-init dependency analysis into `comp/`. Use the bytecode the
compiler emits as the source of truth: every `GetGlobal A`, `CallImm A`,
and `GoCallImm A` already encodes the slot the call or read targets,
and method dispatch compiles to `GetGlobal(method-func-slot)` followed
by `vm.Call`. Tracking these at emit time gives an exact direct-ref
set per scope; a fixed-point pass converges to the transitive Var-only
read set.

The design has three pieces:

1. **Compile-order pre-walk** (`varCompileOrder`). A small token-level
   sibling-Ident walk in `comp` (using `goparser.WalkIdents`) topo-sorts
   var-decls so any var whose RHS reads a sibling's name compiles after
   it. Required because mvm's RHS type inference only sets a var's Type
   when its own init compiles. This walk is intentionally weaker than the
   old parser-side analysis: it follows direct Ident refs only, not
   transitive function-body reads.

2. **Per-buffer emit** (`Compile` Phase 1). Each var-init compiles into
   its own buffer by swapping `c.Code = nil`. A scope-stack field
   `Compiler.emitReads` is pushed/popped around each compile; `c.emit()`
   records every slot-ref opcode into the top scope's accumulator. The
   same mechanism pushes/pops around func bodies via `lang.Label` /
   `<name>_end` in `generate`.

3. **Fixed-point + topo sort** (`expandReads`, `topoSortVarBufs`). After
   all bodies and buffers are emitted, `expandReads` runs a fixed-point
   that, for each accumulator, expands Func slot entries by unioning
   the callee's `Reads`. The result is a transitive Var-only read set
   per func and per buffer. `topoSortVarBufs` then orders the buffers
   so any buffer producing a slot read by another comes first; the
   sorted prefix is prepended to `c.Code` and Func entry-points in
   `c.Data` are shifted by the prefix length.

Both topo passes share `kahnTopoSort`. Inline `var f = func(){...}`
literals are tracked per-buffer via `varBuf.funcSyms` so their
buffer-relative entry-point shifts by the buffer's offset rather than
the full prefix length.

## Consequences

**Easier:**
- Method dispatch, qualified imports, and direct-call analysis all just
  work, because they all compile to `GetGlobal` of a known slot followed
  by `Call` (or directly to `CallImm`). No special token-shape pattern
  matching.
- `goparser/stmt.go` shrinks: the var-block expander is a 12-line
  function with no dep analysis.
- Adding new dispatch flavours (e.g. a future closure-call opcode) only
  needs `isSlotRefOp` updated, not a new walker.
- The dest-classification heuristic (first N `GetGlobal` before trailing
  `SetS` are destination pushes, not value reads) goes away -- emit-time
  knows directly that a `GetGlobal` is producing a settable reference,
  and `varDeclSlots` derives `produces` from the LHS up front.

**Harder:**
- Adds an emit-time hook in `c.emit()` and a scope stack on the
  `Compiler`. The hot path adds a single slice-len check per emit
  (cheap; the path that actually records only fires for three opcodes).
- Compile() is more involved: buffer-swap, snapshot/diff Func symbols
  to find inline-literal newcomers, fixed-point, prepend, shift. Roughly
  100 lines of orchestration, replacing ~70 lines of token-shape
  analysis in goparser.
- Two flavours of init order coexist: the compile-order topo (sibling
  Ident refs) is needed for type inference, the runtime topo
  (bytecode-derived reads) is needed for correctness. Most cases agree;
  they only diverge when a var reads another transitively through a
  function call.

## Acknowledged limitations

- Interface method dispatch (vtable lookup) still cannot be statically
  resolved; this is fundamental to runtime polymorphism.
- Function values fetched from a runtime container (`m["x"]()`) likewise.

These were unreachable for the parser-side approach too, so this is no
regression.

## Supersedes

This ADR refines [ADR-004](ADR-004-lazy-fixpoint.md): the var-init
ordering responsibility moves from goparser's Phase 2 setup to comp's
Compile.
