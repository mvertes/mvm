package main

// Limitation #1 from ADR-015: interface method dispatch goes through a
// runtime vtable. Iface.First() compiles to vm.IfaceCall (carrying a
// methodID, no slot operand), so the var-init dep walker doesn't know
// FromIface reaches Table through R.First.
//
// To make the failure observable rather than masked by topo-luck, the
// init also gives Table a compile-order dep on a sibling Sentinel.
// Without the missing edge, Kahn schedules FromIface before Table, and
// FromIface ends up reading byte 0.
import "fmt"

type Reader interface{ First() byte }

type R struct{}

func (R) First() byte { return Table[0] }

func computeTable(b byte) [256]byte { return [256]byte{0xaa, b + 0xa9} }

var (
	Iface     Reader = R{}
	FromIface        = Iface.First()
	Sentinel  byte   = 1
	Table            = computeTable(Sentinel)
)

func main() {
	fmt.Printf("Table=%x FromIface=%x\n", Table[0], FromIface)
}

// skip: interface method dispatch is opaque to bytecode-derived var-init analysis (ADR-015).
// Output:
// Table=aa FromIface=aa
