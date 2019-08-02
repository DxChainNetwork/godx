package ethdb

import (
	"github.com/deckarep/golang-set"
	"runtime"
	"sync"
	"testing"
	"time"
)

/**
@size: integer of size capacity. size = 0 create as default
@return: the value return by the constructor of memory_database.go
*/
func creatememdb(t *testing.T, size int, dat []struct {
	key   string
	value string
}) *MemDatabase {
	var mem *MemDatabase
	if size <= 0 {
		mem = NewMemDatabase()
	} else {
		mem = NewMemDatabaseWithCap(size)
	}

	if dat != nil {
		for _, it := range dat {
			if err := mem.Put([]byte(it.key), []byte(it.value)); err != nil {
				t.Errorf("add fail")
			}
		}
	}
	return mem
}

/**
Test create database with different parameter, which call out NewMemDatabase() or NewMemDatabaseWithCap(size)
*/
func TestCreatdb(t *testing.T) {
	_ = creatememdb(t, 0, nil)
	_ = creatememdb(t, 10, nil)
	_ = creatememdb(t, 0, testdata)
	_ = creatememdb(t, 20, testdata)
}

/**
@param mem: reference of memdb
@param expected: expected entry the database should contain
Check if the database totally match the expected content
*/
func checkMem(t *testing.T, mem *MemDatabase, expected []struct {
	key   string
	value string
}) {
	keys := mem.Keys()
	set := mapset.NewSet()
	// add kesy to set, for testing if the 'keys()' return the same keys as expected
	for _, it := range keys {
		set.Add(string(it))
	}
	countlength := 0 // to record the number of expected entry should be in the db
	for idx, it := range expected {
		if idx%5 == 0 {
			continue
		} // hard code: choose the 5* to delete, just for generating test case
		if !set.Contains(it.key) {
			t.Errorf("memdb does not contain the expected key")
		}
		if memVal, err := mem.Get([]byte(it.key)); err != nil {
			t.Errorf("Calling 'Get' cause err info")
		} else if string(memVal) != it.value {
			t.Errorf("Value does not match the expected")
		}
		countlength++
	}
	if mem.Len() != countlength {
		t.Errorf("the length of the database does not match the expected")
	}
}

/**
@param start: start position to grab data from large data set
@param c: chanel for recording the accomplishment of the test function, also used for exiting while timeout
*/
func threadDelete(t *testing.T, start int, mem *MemDatabase, c chan string) {
	//var queue []struct{key string; value string}
	var queue = make(map[string]string)
	for i := start; i < len(delset) && i < start+20; i++ {
		runtime.Gosched()
		if contains, err := mem.Has([]byte(delset[i].key)); err != nil {
			t.Errorf("calling Has fail")
		} else if contains == true {
			// contain the value, can be deleted
			if err = mem.Delete([]byte(delset[i].key)); err != nil {
				t.Errorf("fail to delete element in memdb")
			}
		} else if contains == false {
			// does not contain the value, add to
			queue[delset[i].key] = delset[i].value
		}
	}

	qlength := len(queue)
	for qlength != 0 {
		runtime.Gosched()
		for key := range queue {
			if contains, err := mem.Has([]byte(key)); err != nil {
				t.Errorf("calling 'Has' fail")
			} else if contains == true {
				// contain the value, can be deleted
				if err = mem.Delete([]byte(key)); err != nil {
					t.Errorf("fail to delete element in memdb")
				}
				delete(queue, key)
				break // need to refresh iterator
			} else if contains == false {
				// does not contain the value, continue
				continue
			}
		}
		qlength = len(queue)
	}

	c <- "Done"
}

/**
@param c: chanel for recording the complishment of the test function, also used for exiting while timeout
*/
func threadAdd(t *testing.T, start int, mem *MemDatabase, c chan string) {
	for i := start; i < len(largedata) && i < start+100; i++ {
		runtime.Gosched()
		if err := mem.Put([]byte(largedata[i].key), []byte(largedata[i].value)); err != nil {
			t.Errorf("put fail")
		}
	}
	c <- "Done"
}

/**
@param input: start position of input in largedataset
@param del: start position of deletion in the deldataset
Multi-threading write batch at the same time, to test if the database synchronized
*/
func writeBatch(t *testing.T, mem *MemDatabase, input int, del int, group *sync.WaitGroup) {
	batch := mem.NewBatch()
	for i := input; i < len(largedata) && i < input+100; i++ {
		runtime.Gosched()
		if err := batch.Put([]byte(largedata[i].key), []byte(largedata[i].value)); err != nil {
			t.Errorf("cannot add to the batch")
		}
	}
	for j := del; j < len(delset) && j < del+20; j++ {
		runtime.Gosched()
		if err := batch.Delete([]byte(delset[j].key)); err != nil {
			t.Errorf("cannot delete from the batch")
		}
	}
	if err := batch.Write(); err != nil {
		t.Errorf("batch write to database fail")
	}
	_ = batch.ValueSize()
	batch.Reset()
	if batch.ValueSize() != 0 {
		t.Errorf("reset does not work as expected")
	}
	group.Done()
}

/**
Process function for testing concurrency.
20 Threads are going to work at the same time, including adding and deleting in the
database, test if after executing, the database remain as the expected
*/
func TestConcur(t *testing.T) {
	mem := creatememdb(t, 0, nil)
	chs := make(chan string, 20)
	for i := 0; i < 10; i++ {
		//chs] = make(chan string)
		go threadAdd(t, i*100, mem, chs)
		go threadDelete(t, 200-(i+1)*20, mem, chs)
	}

	// start a time count to measure the time
	// problem: go has no Daemon mechanism, porblem may occur if the timer cannot start as a thread if dead lock happen
	timeout := make(chan bool, 1)
	go func() {
		time.Sleep(3 * time.Second)
		timeout <- true
	}()

	counter := 0
	loop := true
	for loop {
		select {
		case <-chs:
			counter++
			if counter >= 20 {
				loop = false
			}
		case <-timeout:
			if counter < 20 {
				t.Errorf("writing time out, threads may not be synchronized")
				return
			} else {
				loop = false
			}
		}
	}
	checkMem(t, mem, largedata)
	mem.Close()
}

/**
Test writing batch in conccurrency approach.
*/
func TestBatchConcur(t *testing.T) {
	var group sync.WaitGroup
	group.Add(10)
	mem := creatememdb(t, 0, nil)
	for i := 0; i < 10; i++ {
		go writeBatch(t, mem, i*100, i*20, &group)
	}
	group.Wait()
	checkMem(t, mem, largedata)
}
