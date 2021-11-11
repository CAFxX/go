// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package buildid

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: go tool buildid [-w] file\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func Main() {
	log.SetPrefix("buildid: ")
	log.SetFlags(0)
	flag.Usage = usage
	var wflag = flag.Bool("w", false, "write build ID")
	flag.Parse()
	if flag.NArg() != 1 {
		usage()
	}

	file := flag.Arg(0)
	id, err := ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}
	if !*wflag {
		fmt.Printf("%s\n", id)
		return
	}

	// Keep in sync with src/cmd/go/internal/work/buildid.go:updateBuildID

	f, err := os.Open(file)
	if err != nil {
		log.Fatal(err)
	}
	matches, hash, err := FindAndHash(f, id, 0)
	f.Close()
	if err != nil {
		log.Fatal(err)
	}

	newID := id[:strings.LastIndex(id, "/")] + "/" + HashToString(hash)
	if len(newID) != len(id) {
		log.Fatalf("%s: build ID length mismatch %q vs %q", file, id, newID)
	}

	if len(matches) == 0 {
		return
	}

	f, err = os.OpenFile(file, os.O_RDWR, 0)
	if err != nil {
		log.Fatal(err)
	}
	if err := Rewrite(f, matches, newID); err != nil {
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
}
