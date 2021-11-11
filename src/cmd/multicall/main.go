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
	switch filepath.Base(os.Args[0]) {
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
	case "multicall":
		os.Stderr.WriteString("Golang multicall binary\n")
		return
	default:
		panic(`unknown command "` + os.Args[0] + `"`)
	}
}
