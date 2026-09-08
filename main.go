package main

import (
	"os"

	"github.com/tdeshazo/repoctx/internal/cli"
)

func main() { cli.Main(os.Args[1:]) }
