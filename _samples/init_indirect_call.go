package main

// Limitation #2 from ADR-015: a function value fetched at runtime via an
// opaque path is invisible to the analyzer. Src.Fn() returns the real
// `first` via interface dispatch; the trailing call lands on a vm.Call
// (no slot operand). Neither hop surfaces a slot ref to first, so
// FromFn's read set never grows to include Table.
//
// As with init_iface.go, Table is given a sibling-dep on Sentinel so
// the missing edge actually changes the topo order.
import "fmt"

type FnSource interface{ Fn() func() byte }

type S struct{}

func (S) Fn() func() byte { return first }

func first() byte { return Table[0] }

func computeTable(b byte) [256]byte { return [256]byte{0xaa, b + 0xa9} }

var (
	Src      FnSource = S{}
	FromFn            = Src.Fn()()
	Sentinel byte     = 1
	Table             = computeTable(Sentinel)
)

func main() {
	fmt.Printf("Table=%x FromFn=%x\n", Table[0], FromFn)
}

// skip: indirect call through a func value fetched at runtime is opaque to bytecode-derived var-init analysis (ADR-015).
// Output:
// Table=aa FromFn=aa
