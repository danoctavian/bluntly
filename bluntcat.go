package main

import (
	"fmt"
	"runtime"
	"github.com/danoctavian/bluntly/node"
)

func main() {
	// Use all processor cores.
	runtime.GOMAXPROCS(runtime.NumCPU())

	fmt.Println("running bluntly..")
	_ ,err := node.NewNode(nil)

	if err != nil {
		fmt.Println("failed to create a node: ", err)
	}
}