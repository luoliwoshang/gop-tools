// Copyright 2022 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"log"

	"golang.org/x/tools/gopls/goxls"
)

func main() {
	log.Println("goxls starting...")
	goxls.Main(goxls.FlagsDebug)
}
