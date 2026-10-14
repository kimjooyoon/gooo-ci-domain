package main

import (
	domain "github.com/kimjooyoon/gooo-ci-domain"
	"os"
)

func main() { os.Exit(domain.Main(os.Args[1:], os.Stdout, os.Stderr)) }
