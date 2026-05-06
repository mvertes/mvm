package main

// Regression: FromTable's init calls computeUsingTable() which reads Table,
// so Table must be initialized first even though FromTable appears earlier
// in source order.
import "fmt"

func computeUsingTable() byte { return Table[0] }

var (
	FromTable = computeUsingTable()
	Table     = [256]byte{0xaa, 0xbb, 0xcc, 0xdd}
)

func main() {
	fmt.Printf("Table=%x FromTable=%x\n", Table[0], FromTable)
}

// Output:
// Table=aa FromTable=aa
