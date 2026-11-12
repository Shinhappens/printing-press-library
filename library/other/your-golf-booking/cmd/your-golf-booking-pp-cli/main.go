// Copyright 2026 Printing Press Library contributors. Licensed under Apache-2.0. See LICENSE.

package main

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/other/your-golf-booking/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
