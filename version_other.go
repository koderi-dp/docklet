//go:build !windows

package main

import "os"

func versionOutput() *os.File { return os.Stdout }
