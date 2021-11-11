package main

import (
	"cmd/internal/addr2line"
	"cmd/internal/buildid"
	"cmd/internal/cover"
	"cmd/internal/doc"
	"cmd/internal/pprof"
	"cmd/internal/trace"
	"os"
	"path/filepath"
)

func main() {
	cmd := filepath.Base(os.Args[0])
	switch cmd {
	case "addr2line":
		addr2line.Main()
	case "buildid":
		buildid.Main()
	case "cover":
		cover.Main()
	case "doc":
		doc.Main()
	case "pprof":
		pprof.Main()
	case "trace":
		trace.Main()
	default:
		os.Stderr.WriteString("Golang multicall binary\n")
		if cmd != "multicall" {
			os.Stderr.WriteString(`unknown command "` + os.Args[0] + `"` + "\n")
			os.Exit(64) // EX_USAGE
		}
	}
}
