package rlp

import "fmt"

type struct1 struct {
	Name string
	Age  uint
}

type struct2 struct {
	name []byte
	age  []byte
}

var personTest = struct1{"mzhang", 24}
var exampleWriter struct2

func (s *struct2) Write(p []byte) (n int, err error) {
	// it will get the encoded list header first
	// then get the encoded data content
	if p[0] < 0xBF {
		nameLength := int(p[0] - 0x80)
		for i := 1; i <= nameLength; i++ {
			s.name = append(s.name, p[i])
		}

		for i := nameLength + 1; i < len(p); i++ {
			s.age = append(s.age, p[i])
		}

		return len(p) - 1, nil
	}

	// list header, ignore
	return 0, nil

}

func ExampleEncoder() {
	err := Encode(&exampleWriter, personTest)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(exampleWriter.name)
	fmt.Println(exampleWriter.age)
	// Output:
	// [109 122 104 97 110 103]
	// [24]
}
