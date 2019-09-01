// Copyright IBM Corp. All Rights Reserved.
//
// SPDX-License-Identifier: Apache-2.0
//

package test

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBasic(t *testing.T) {
	t.Parallel()
	network := make(Network)
	defer network.Shutdown()

	testDir, err := ioutil.TempDir("", t.Name())
	assert.NoErrorf(t, err, "generate temporary test dir")
	defer os.RemoveAll(testDir)

	n1 := newNode(1, 4, network, t.Name(), testDir)
	n2 := newNode(2, 4, network, t.Name(), testDir)
	n3 := newNode(3, 4, network, t.Name(), testDir)
	n4 := newNode(4, 4, network, t.Name(), testDir)

	n1.Consensus.Start()
	n2.Consensus.Start()
	n3.Consensus.Start()
	n4.Consensus.Start()

	n1.Submit(Request{ID: "1", ClientID: "alice"})
	n1.Submit(Request{ID: "2", ClientID: "alice"})
	n1.Submit(Request{ID: "3", ClientID: "alice"})
	n1.Submit(Request{ID: "3", ClientID: "alice"})

	data1 := <-n1.Delivered
	data2 := <-n2.Delivered
	data3 := <-n3.Delivered
	data4 := <-n4.Delivered

	assert.Equal(t, data1, data2)
	assert.Equal(t, data3, data4)
	assert.Equal(t, data1, data4)
}

func TestRestartFollowers(t *testing.T) {
	t.Parallel()
	network := make(Network)
	defer network.Shutdown()

	testDir, err := ioutil.TempDir("", t.Name())
	assert.NoErrorf(t, err, "generate temporary test dir")
	defer os.RemoveAll(testDir)

	n1 := newNode(1, 4, network, t.Name(), testDir)
	n2 := newNode(2, 4, network, t.Name(), testDir)
	n3 := newNode(3, 4, network, t.Name(), testDir)
	n4 := newNode(4, 4, network, t.Name(), testDir)

	n1.Consensus.Start()
	n2.Consensus.Start()
	n3.Consensus.Start()
	n4.Consensus.Start()

	n1.Submit(Request{ID: "1", ClientID: "alice"})

	n2.Restart()

	data1 := <-n1.Delivered
	data2 := <-n2.Delivered
	data3 := <-n3.Delivered
	data4 := <-n4.Delivered

	assert.Equal(t, data1, data2)
	assert.Equal(t, data3, data4)
	assert.Equal(t, data1, data4)

	n3.Restart()
	n1.Submit(Request{ID: "2", ClientID: "alice"})
	n4.Restart()

	data1 = <-n1.Delivered
	data2 = <-n2.Delivered
	data3 = <-n3.Delivered
	data4 = <-n4.Delivered
	assert.Equal(t, data1, data2)
	assert.Equal(t, data3, data4)
	assert.Equal(t, data1, data4)
}

func TestLeaderInPartition(t *testing.T) {
	t.Parallel()
	network := make(Network)
	defer network.Shutdown()

	testDir, err := ioutil.TempDir("", t.Name())
	assert.NoErrorf(t, err, "generate temporary test dir")
	defer os.RemoveAll(testDir)

	n0 := newNode(0, 4, network, t.Name(), testDir)
	n1 := newNode(1, 4, network, t.Name(), testDir)
	n2 := newNode(2, 4, network, t.Name(), testDir)
	n3 := newNode(3, 4, network, t.Name(), testDir)

	n0.Consensus.Start()
	n1.Consensus.Start()
	n2.Consensus.Start()
	n3.Consensus.Start()

	n0.Disconnect() // leader in partition

	n1.Submit(Request{ID: "1", ClientID: "alice"}) // submit to other nodes
	n2.Submit(Request{ID: "1", ClientID: "alice"})
	n3.Submit(Request{ID: "1", ClientID: "alice"})

	data1 := <-n1.Delivered
	data2 := <-n2.Delivered
	data3 := <-n3.Delivered
	assert.Equal(t, data1, data2)
	assert.Equal(t, data2, data3)
}

func TestAfterDecisionLeaderInPartition(t *testing.T) {
	t.Parallel()
	network := make(Network)
	defer network.Shutdown()

	testDir, err := ioutil.TempDir("", t.Name())
	assert.NoErrorf(t, err, "generate temporary test dir")
	defer os.RemoveAll(testDir)

	n0 := newNode(0, 4, network, t.Name(), testDir)
	n1 := newNode(1, 4, network, t.Name(), testDir)
	n2 := newNode(2, 4, network, t.Name(), testDir)
	n3 := newNode(3, 4, network, t.Name(), testDir)

	n0.Consensus.Start()
	n1.Consensus.Start()
	n2.Consensus.Start()
	n3.Consensus.Start()

	n0.Submit(Request{ID: "1", ClientID: "alice"}) // submit to leader

	data0 := <-n0.Delivered
	data1 := <-n1.Delivered
	data2 := <-n2.Delivered
	data3 := <-n3.Delivered
	assert.Equal(t, data0, data1)
	assert.Equal(t, data1, data2)
	assert.Equal(t, data2, data3)

	n0.Submit(Request{ID: "2", ClientID: "alice"})

	data0 = <-n0.Delivered
	data1 = <-n1.Delivered
	data2 = <-n2.Delivered
	data3 = <-n3.Delivered
	assert.Equal(t, data0, data1)
	assert.Equal(t, data1, data2)
	assert.Equal(t, data2, data3)

	n0.Disconnect() // leader in partition

	n1.Submit(Request{ID: "3", ClientID: "alice"}) // submit to other nodes
	n2.Submit(Request{ID: "3", ClientID: "alice"})
	n3.Submit(Request{ID: "3", ClientID: "alice"})

	data1 = <-n1.Delivered
	data2 = <-n2.Delivered
	data3 = <-n3.Delivered
	assert.Equal(t, data1, data2)
	assert.Equal(t, data2, data3)

	n1.Submit(Request{ID: "4", ClientID: "alice"})
	n2.Submit(Request{ID: "4", ClientID: "alice"})
	n3.Submit(Request{ID: "4", ClientID: "alice"})

	data1 = <-n1.Delivered
	data2 = <-n2.Delivered
	data3 = <-n3.Delivered
	assert.Equal(t, data1, data2)
	assert.Equal(t, data2, data3)
}

func TestMultiLeadersPartition(t *testing.T) {
	t.Parallel()
	network := make(Network)
	defer network.Shutdown()

	testDir, err := ioutil.TempDir("", t.Name())
	assert.NoErrorf(t, err, "generate temporary test dir")
	defer os.RemoveAll(testDir)

	n0 := newNode(0, 7, network, t.Name(), testDir)
	n1 := newNode(1, 7, network, t.Name(), testDir)
	n2 := newNode(2, 7, network, t.Name(), testDir)
	n3 := newNode(3, 7, network, t.Name(), testDir)
	n4 := newNode(4, 7, network, t.Name(), testDir)
	n5 := newNode(5, 7, network, t.Name(), testDir)
	n6 := newNode(6, 7, network, t.Name(), testDir)

	n0.Consensus.Start()
	n1.Consensus.Start()

	n0.Disconnect() // leader in partition
	n1.Disconnect() // next leader in partition

	n2.Consensus.Start()
	n3.Consensus.Start()
	n4.Consensus.Start()
	n5.Consensus.Start()
	n6.Consensus.Start()

	n2.Submit(Request{ID: "1", ClientID: "alice"}) // submit to new leader
	n3.Submit(Request{ID: "1", ClientID: "alice"}) // submit to follower
	n4.Submit(Request{ID: "1", ClientID: "alice"})
	n5.Submit(Request{ID: "1", ClientID: "alice"})
	n6.Submit(Request{ID: "1", ClientID: "alice"})

	data2 := <-n2.Delivered
	data3 := <-n3.Delivered
	data4 := <-n4.Delivered
	data5 := <-n5.Delivered
	data6 := <-n6.Delivered

	assert.Equal(t, data2, data3)
	assert.Equal(t, data3, data4)
	assert.Equal(t, data4, data5)
	assert.Equal(t, data5, data6)
	assert.Equal(t, data6, data2)

}

func TestCatchingUpWithViewChange(t *testing.T) {
	t.Parallel()
	network := make(Network)
	defer network.Shutdown()

	testDir, err := ioutil.TempDir("", t.Name())
	assert.NoErrorf(t, err, "generate temporary test dir")
	defer os.RemoveAll(testDir)

	n0 := newNode(0, 4, network, t.Name(), testDir)
	n1 := newNode(1, 4, network, t.Name(), testDir)
	n2 := newNode(2, 4, network, t.Name(), testDir)
	n3 := newNode(3, 4, network, t.Name(), testDir)

	n0.Consensus.Start()
	n1.Consensus.Start()
	n2.Consensus.Start()
	n3.Consensus.Start()

	n3.Disconnect() // will need to catch up

	n0.Submit(Request{ID: "1", ClientID: "alice"}) // submit to leader

	data0 := <-n0.Delivered
	data1 := <-n1.Delivered
	data2 := <-n2.Delivered

	assert.Equal(t, data0, data1)
	assert.Equal(t, data1, data2)

	n3.Connect()
	n0.Disconnect() // leader in partition

	n1.Submit(Request{ID: "2", ClientID: "alice"}) // submit to other nodes
	n2.Submit(Request{ID: "2", ClientID: "alice"})
	n3.Submit(Request{ID: "2", ClientID: "alice"})

	data3 := <-n3.Delivered // from catch up
	assert.Equal(t, data2, data3)

	data1 = <-n1.Delivered
	data2 = <-n2.Delivered
	data3 = <-n3.Delivered

	assert.Equal(t, data1, data2)
	assert.Equal(t, data2, data3)
}

func TestLeaderForwarding(t *testing.T) {
	t.Parallel()
	network := make(Network)
	defer network.Shutdown()

	testDir, err := ioutil.TempDir("", t.Name())
	assert.NoErrorf(t, err, "generate temporary test dir")
	defer os.RemoveAll(testDir)

	n0 := newNode(0, 4, network, t.Name(), testDir)
	n1 := newNode(1, 4, network, t.Name(), testDir)
	n2 := newNode(2, 4, network, t.Name(), testDir)
	n3 := newNode(3, 4, network, t.Name(), testDir)

	n0.Consensus.Start()
	n1.Consensus.Start()
	n2.Consensus.Start()
	n3.Consensus.Start()

	n1.Submit(Request{ID: "1", ClientID: "alice"})
	n2.Submit(Request{ID: "2", ClientID: "bob"})
	n3.Submit(Request{ID: "3", ClientID: "carol"})

	numBatchesCreated := countCommittedBatches(n0)

	committedBatches := make([][]AppRecord, 3)
	for nodeIndex, n := range []*App{n1, n2, n3} {
		committedBatches = append(committedBatches, make([]AppRecord, numBatchesCreated))
		for i := 0; i < numBatchesCreated; i++ {
			record := <-n.Delivered
			committedBatches[nodeIndex] = append(committedBatches[nodeIndex], *record)
		}
	}

	assert.Equal(t, committedBatches[0], committedBatches[1])
	assert.Equal(t, committedBatches[0], committedBatches[2])
}

func TestLeaderExclusion(t *testing.T) {
	// Scenario: The leader doesn't send messages to n3,
	// but it should detect this and sync.
	t.Parallel()
	network := make(Network)
	defer network.Shutdown()

	testDir, err := ioutil.TempDir("", t.Name())
	assert.NoErrorf(t, err, "generate temporary test dir")
	defer os.RemoveAll(testDir)

	n0 := newNode(0, 4, network, t.Name(), testDir)
	n1 := newNode(1, 4, network, t.Name(), testDir)
	n2 := newNode(2, 4, network, t.Name(), testDir)
	n3 := newNode(3, 4, network, t.Name(), testDir)

	n0.DisconnectFrom(3)

	n0.Consensus.Start()
	n1.Consensus.Start()
	n2.Consensus.Start()
	n3.Consensus.Start()

	// We create new batches until the disconnected node catches up the quorum.
	for reqID := 1; reqID < 100; reqID++ {
		n1.Submit(Request{ID: fmt.Sprintf("%d", reqID), ClientID: "alice"})
		<-n1.Delivered // Wait for follower to commit
		caughtUp := waitForCatchup(reqID, n3.Delivered)
		if caughtUp {
			return
		}
	}
	t.Log("Didn't catch up")
}

func TestCatchingUpWithSync(t *testing.T) {
	t.Parallel()
	network := make(Network)
	defer network.Shutdown()

	testDir, err := ioutil.TempDir("", "test-leader-forwarding")
	assert.NoErrorf(t, err, "generate temporary test dir")
	defer os.RemoveAll(testDir)

	n0 := newNode(0, 4, network, t.Name(), testDir)
	n1 := newNode(1, 4, network, t.Name(), testDir)
	n2 := newNode(2, 4, network, t.Name(), testDir)
	n3 := newNode(3, 4, network, t.Name(), testDir)

	n0.Consensus.Start()
	n1.Consensus.Start()
	n2.Consensus.Start()
	n3.Consensus.Start()

	n3.Disconnect() // will need to catch up

	for i := 1; i <= 10; i++ {
		n0.Submit(Request{ID: fmt.Sprintf("%d", i), ClientID: "alice"})
		<-n0.Delivered // Wait for leader to commit
		<-n1.Delivered // Wait for follower to commit
		<-n2.Delivered // Wait for follower to commit
	}

	n3.Connect()

	// We create new batches until it catches up the quorum.
	for reqID := 11; reqID < 100; reqID++ {
		n1.Submit(Request{ID: fmt.Sprintf("%d", reqID), ClientID: "alice"})
		<-n1.Delivered // Wait for follower to commit
		caughtUp := waitForCatchup(reqID, n3.Delivered)
		if caughtUp {
			return
		}
	}
	t.Log("Didn't catch up")
}

func countCommittedBatches(n *App) int {
	var numBatchesCreated int
	for {
		select {
		case <-n.Delivered:
			numBatchesCreated++
		case <-time.After(time.Millisecond * 500):
			return numBatchesCreated
		}
	}
}

func requestIDFromBatch(record *AppRecord) int {
	n, _ := strconv.ParseInt(requestFromBytes(record.Batch.Requests[0]).ID, 10, 32)
	return int(n)
}

func waitForCatchup(targetReqID int, out chan *AppRecord) bool {
	for {
		select {
		case record := <-out:
			if requestIDFromBatch(record) == targetReqID {
				return true
			}
		case <-time.After(time.Millisecond * 100):
			return false
		}
	}
}
