package main

// Regression: FromMethod's init calls First() on R{}, which reads Table.
// Init order must place Table first; comp's bytecode-derived analysis sees
// the GetGlobal of First's func slot followed by vm.Call and follows the
// method's reads transitively.
import "fmt"

type R struct{}

func (R) First() byte { return Table[0] }

var (
	FromMethod = R{}.First()
	Table      = [256]byte{0xaa, 0xbb}
)

func main() {
	fmt.Printf("Table=%x FromMethod=%x\n", Table[0], FromMethod)
}

// Output:
// Table=aa FromMethod=aa
