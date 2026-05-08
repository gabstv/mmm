package mmmvet_test

import (
	"testing"

	"github.com/gabstv/mmm/analysis/mmmvet"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestMmmvet(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, mmmvet.Analyzer,
		"safe", "allocwarn", "assignwarn", "pinok", "arenaptrsafe",
	)
}
