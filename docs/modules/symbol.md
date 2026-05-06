# symbol

> Scoped symbol table for variables, types, functions, and labels.

## Overview

The `symbol` package provides `SymMap`, a flat map from scoped names to
`Symbol` entries. It is shared between the parser and compiler: the parser
populates it during parsing; the compiler reads it during code generation
to resolve addresses and types.

## Key types and functions

- **`SymMap`** (type `map[string]*Symbol`) -- symbol table. Keys are
  scoped names like `main/foo/x` or `0/int` (scope `0` for builtins).
- **`Symbol`** -- a table entry:
  - `Kind` -- one of `Value`, `Type`, `Label`, `Const`, `Var`,
    `LocalVar`, `Func`, `Pkg`, `Builtin`, `Generic`. `Var` is for
    global data; `LocalVar` is for frame-relative locals; `Generic`
    marks a generic function or type template.
  - `Index` -- address in the VM data segment or frame.
  - `Type` -- `*vm.Type` for the symbol's runtime type.
  - `Captured` / `FreeVars` -- closure capture metadata.
  - `RecvName` -- for method symbols: raw receiver variable name, cached
    from Phase 1 so `parseFunc` can re-use it in Phase 2.
  - `InNames` / `OutNames` -- raw input/output parameter names for func
    symbols, cached during Phase 1 signature parsing so Phase 2 does not
    re-parse the signature.
  - `Data any` -- opaque payload. Used by `Generic` symbols to store a
    `*genericTemplate` (type params, raw token stream, func-or-type flag).
    Nil for all other kinds.
  - `Reads map[int]bool` -- for `Func` symbols only: the set of global
    Data-slot indices the function body reads transitively. Populated
    in two stages by the compiler -- emit-time tracking captures
    direct slot references as `c.emit()` runs; a fixed-point pass
    afterwards expands callee Func entries into their transitive Var
    slots. Used by `comp` to topo-sort var-init buffers
    (see [ADR-015](../decisions/ADR-015-var-init-dep-analysis-in-comp.md)).
- **`Kind`** (int enum) -- symbol classification.
- **`Get(name, scope string) (*Symbol, string, bool)`** -- lookup by
  walking from the innermost scope outward.
- **`MethodByName(sym, name) (*Symbol, []int)`** -- find a method on a
  type, including promoted methods from embedded fields.
- **`Package`** -- package descriptor with `Path`, `Bin` (binary flag),
  and `Values map[string]vm.Value` for exported symbols.
- **`BinPkg(m map[string]reflect.Value, name string) *Package`** -- creates
  a binary package from a map of reflect values (used for stdlib wrappers).
- **`Init()`** -- populates builtin types (`int`, `string`, `bool`, ...),
  values (`nil`, `true`, `false`, `iota`), and builtin functions (`print`,
  `println`, `len`, `cap`, `append`, `copy`, `delete`, `new`, `make`,
  `panic`, `recover`, `trap`) with `Kind: Builtin`. The compiler emits
  dedicated opcodes for each builtin.

## Internal design

Symbol lookup walks the scope hierarchy: given scope `main/foo/for0` and
name `x`, it tries `main/foo/for0/x`, then `main/foo/x`, then `main/x`,
then `0/x` (builtins). First match wins.

Method lookup traverses embedded field chains to find promoted methods,
returning the field index path needed to reach the receiver.

### MethodByName ambiguity heuristic

For Var/LocalVar/Value receivers whose `Type.Name` is empty (the
common case for mvm-created struct types and pointer-to-struct
results from field access), `MethodByName` searches the symbol table
for a Type entry whose `Rtype` matches and uses *that* key as the
type name to construct the method lookup. The same `Rtype` can appear
under multiple keys:

- the unqualified short name `T` (from the user's type decl),
- a package-qualified alias `pkgpath.T` written by `importSrc`,
- an anonymous-struct stringification like `struct { F int }`
  (from `zeroInitLocals` registering names for var init).

Methods are registered under the short receiver name (e.g. `*T.M`), so
lookups through the qualified or stringified key would miss. To handle
this, `MethodByName` iterates candidate keys and prefers the one that
*also* has a registered method (`k.M` or `*k.M` is in the map). If
none has a registered method, it falls back to the first match -- this
preserves prior behavior for receivers that genuinely have no methods.

This is a heuristic. The planned canonical-pkg-qualified-symbol-keys
refactor will replace it: every type and method gets stored under
`pkgpath.T` / `pkgpath.*T.M` with short names becoming a
scope-visibility concern, eliminating the ambiguity at the root.

## Dependencies

- `vm/` -- `Type`, `Value` structures.
