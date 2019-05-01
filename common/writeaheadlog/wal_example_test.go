package writeaheadlog

import (
	"fmt"
	"os"
	"path/filepath"
)

func ExampleWal() {
	testDir := tempDir("ExampleWal")
	_ = os.Mkdir(testDir, 0777)
	// create a new wal
	wal, txns, err := New(filepath.Join(testDir, "test.wal"))
	if len(txns) != 0 {
		fmt.Printf("Unexpected transaction number. Got %d, Expect %d\n", len(txns), 0)
		return
	}
	if err != nil {
		fmt.Printf("Cannot new wal: %v\n", err)
		return
	}
	// struct to be modified
	p := person{name: "jacky"}
	newName := "wyx"
	op := Operation{
		Name: "person_rename",
		Data: []byte(newName),
	}
	t, err := wal.NewTransaction([]Operation{op})
	fmt.Println("Add a new transaction")
	if err != nil {
		fmt.Printf("new transaction error: %v\n", err)
		return
	}
	p.name = newName
	<-t.InitComplete
	if t.InitErr != nil {
		fmt.Println(t.InitErr)
		return
	}
	commitDone := t.Commit()
	if err := <-commitDone; err != nil {
		fmt.Println("error commit")
		return
	}
	err = wal.Close()
	fmt.Printf("Close: %v\n", err)

	// reopen
	recoveredWal, recoveredTxn, err := New(filepath.Join(testDir, "test.wal"))
	if err != nil {
		fmt.Println("reopen wal error:", err)
		return
	}
	if len(recoveredTxn) != 1 {
		fmt.Printf("expected recovered txn 1, got %d\n", len(recoveredTxn))
		return
	}
	for _, txn := range recoveredTxn {
		for _, op := range txn.Operations {
			p.name = string(op.Data)
			err := p.saveToDisk(filepath.Join(testDir, "jacky_person.dat"))
			if err != nil {
				fmt.Printf("failed save to disk: %v", err)
				return
			}
		}
		txn.Release()
	}
	err = recoveredWal.Close()
	if err != nil {
		fmt.Printf("wal doesn't close clean: %v", err)
	}

	// Output:
	// Add a new transaction
	// Close: wal closed with 1 unfinished transactions
}
