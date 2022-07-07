// Copyright (c) 2022 RFull Development
// This source code is managed under the MIT license. See LICENSE in the project root.
package main

import (
	"fmt"
	"os"

	"github.com/ngv-jp/libgo-http/conv"
)

func main() {
	c := conv.NewHttpHeaderConverter()
	r, e := c.Output()
	if e != nil {
		os.Exit(1)
	}
	fmt.Println(r)
}
