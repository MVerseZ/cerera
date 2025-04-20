package types

import (
	"fmt"
	"testing"
)

func testBase(t *testing.T) {
	var a, _ = Decode("test")
	fmt.Println(a)
}
