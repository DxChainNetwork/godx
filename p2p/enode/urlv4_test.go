package enode

import (
	"testing"
)

/*
	Test the function parseComplete to check if the Node with correct information
	can be returned based on the enode url
*/

func TestParseComplete(t *testing.T) {
	tables := []struct {
		rawurl string
		error  error
	}{
		{"enode://6f8a80d14311c39f35f516fa664deaaaa13e85b2f7493f37f6144d86991ec01293730" +
			"7647bd3b9a82abe2974e1407241d54947bbb39763a4cac9f77166ad92a0@10.3.58.6:30303?discport=30301", nil},
	}

	for _, table := range tables {
		_, err := parseComplete(table.rawurl)
		if err != nil {
			t.Fatalf("Error Parsing URL %s: %s", table.rawurl, err)
		}
		//fmt.Printf("%T \n", node)
		//fmt.Printf("%v \n", node.id)
		//fmt.Printf("%v \n", node.r)
	}
}
