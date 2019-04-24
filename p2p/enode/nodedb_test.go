package enode

import (
	"bytes"
	"math/rand"
	"reflect"
	"testing"
)

/*
	Test the function makeKey to check if the correct key-blob can be generated based on
	the node id and the field
*/

func TestMakeKey(t *testing.T) {
	id := idGenerator()
	field := "UDP"
	result := addPrefixSuffix(dbItemPrefix, id, &field)

	tables := []struct {
		id     ID
		field  string
		result []byte
	}{
		{ID{}, "IP", []byte{'I', 'P'}},
		{toArray(id), field, result},
	}

	for _, table := range tables {
		returnResult := makeKey(table.id, table.field)
		if !bytes.Equal(table.result, returnResult) {
			t.Errorf("Input ID: %s, Input Field: %s. Got result: %s Expected: %s",
				table.id, table.field, returnResult, table.result)
		}
	}
}

/*
	Test the function splitKey to check if the correct node id and field can be retrieved
*/

func TestSplitKey(t *testing.T) {
	id := idGenerator()
	field := "Discover"
	dbkey := addPrefixSuffix(dbItemPrefix, id, &field)

	tables := []struct {
		key   []byte
		id    ID
		field string
	}{
		{[]byte("TCP"), ID{}, "TCP"},
		{dbkey, toArray(id), field},
	}

	for _, table := range tables {
		id, field := splitKey(table.key)
		if field != table.field {
			t.Errorf("Database Key: %s Got Field: %s Expected Field: %s",
				table.key, field, table.field)
		}

		if !reflect.DeepEqual(id, table.id) {
			t.Errorf("Database Key: %s Got ID %s Expected ID: %s",
				table.key, id, table.id)
		}
	}
}

/*

 _____ _   _ _______ ______ _____  _   _          _        ______ _    _ _   _  _____ _______ _____ ____  _   _
|_   _| \ | |__   __|  ____|  __ \| \ | |   /\   | |      |  ____| |  | | \ | |/ ____|__   __|_   _/ __ \| \ | |
  | | |  \| |  | |  | |__  | |__) |  \| |  /  \  | |      | |__  | |  | |  \| | |       | |    | || |  | |  \| |
  | | | . ` |  | |  |  __| |  _  /| . ` | / /\ \ | |      |  __| | |  | | . ` | |       | |    | || |  | | . ` |
 _| |_| |\  |  | |  | |____| | \ \| |\  |/ ____ \| |____  | |    | |__| | |\  | |____   | |   _| || |__| | |\  |
|_____|_| \_|  |_|  |______|_|  \_\_| \_/_/    \_\______| |_|     \____/|_| \_|\_____|  |_|  |_____\____/|_| \_|


*/

func addPrefixSuffix(prefix string, data []byte, field *string) []byte {
	bytePrefix := []byte(prefix)
	result := append(bytePrefix, data...)
	if field != nil {
		result = append(result, *field...)
	}
	return result
}

func idGenerator() []byte {
	id := make([]byte, 32)
	rand.Read(id)
	return id
}

func toArray(slice []byte) [32]byte {
	var arr [32]byte
	copy(arr[:], slice)
	return arr
}
