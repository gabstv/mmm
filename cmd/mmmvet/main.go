package main

import (
	"github.com/gabstv/mmm/analysis/mmmvet"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(mmmvet.Analyzer) }
