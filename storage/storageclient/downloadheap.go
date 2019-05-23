// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

// downloadSegmentHeap is a heap that is sorted first by file priority, then by
// the start time of the download, and finally by the index of the segment.  As
// downloads are queued, they are added to the downloadSegmentHeap. As resources
// become available to execute downloads, segments are pulled off of the heap and
// distributed to workers.
type downloadSegmentHeap []*unfinishedDownloadSegment

func (dch downloadSegmentHeap) Len() int {
	return len(dch)
}

func (dch downloadSegmentHeap) Less(i, j int) bool {
	// First sort by priority.
	if dch[i].staticPriority != dch[j].staticPriority {
		return dch[i].staticPriority > dch[j].staticPriority
	}
	// For equal priority, sort by start time.
	if dch[i].download.staticStartTime != dch[j].download.staticStartTime {
		return dch[i].download.staticStartTime.Before(dch[j].download.staticStartTime)
	}
	// For equal start time (typically meaning it's the same file), sort by
	// chunkIndex.
	//
	// NOTE: To prevent deadlocks when acquiring memory and using writers that
	// will streamline / order different chunks, we must make sure that we sort
	// by chunkIndex such that the earlier chunks are selected first from the
	// heap.
	return dch[i].staticSegmentIndex < dch[j].staticSegmentIndex
}

func (dch downloadSegmentHeap) Swap(i, j int) {
	dch[i], dch[j] = dch[j], dch[i]
}

func (dch *downloadSegmentHeap) Push(x interface{}) {
	*dch = append(*dch, x.(*unfinishedDownloadSegment))
}

func (dch *downloadSegmentHeap) Pop() interface{} {
	old := *dch
	n := len(old)
	x := old[n-1]
	*dch = old[0 : n-1]
	return x
}
