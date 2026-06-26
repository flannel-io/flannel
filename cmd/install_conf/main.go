// Copyright (c) 2026 Flannel Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This is a install tool for multus plugins
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/flannel-io/flannel/pkg/utils"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: %s <src> <dest>\n", os.Args[0])
		os.Exit(1)
	}

	src := os.Args[1]
	dest := os.Args[2]
	destDir := filepath.Dir(dest)
	destFileName := filepath.Base(dest)

	if err := utils.CopyFileAtomic(src, destDir, "."+destFileName+".tmp", destFileName); err != nil {
		fmt.Fprintf(os.Stderr, "error installing %q: %v\n", src, err)
		os.Exit(1)
	}
}
