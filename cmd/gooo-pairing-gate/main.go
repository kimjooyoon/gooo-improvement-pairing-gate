package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kimjooyoon/gooo-improvement-pairing-gate/internal/gate"
)

func main() {
	options := gate.Options{}
	flag.StringVar(&options.Source, "source", ".", "input source directory")
	flag.StringVar(&options.Fixture, "fixture", "testdata/canonical-fixtures.json", "fixture JSON path")
	flag.StringVar(&options.Metacode, "metacode", ".gooo/pairing-gate.gooo", "authoritative .gooo metacode path")
	flag.StringVar(&options.Output, "output", "", "empty caller-owned output directory")
	flag.Parse()
	if err := gate.Run(options); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
