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
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestBasic(t *testing.T) {
	t.Parallel()
	network := make(Network)
	defer network.Shutdown()

	testDir, err := ioutil.TempDir("", t.Name())
	assert.NoErrorf(t, err, "generate temporary test dir")
	defer os.RemoveAll(testDir)

	n1 := newNode(1, network, t.Name(), testDir)
	n2 := newNode(2, network, t.Name(), testDir)
	n3 := newNode(3, network, t.Name(), testDir)
	n4 := newNode(4, network, t.Name(), testDir)

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

	n1 := newNode(1, network, t.Name(), testDir)
	n2 := newNode(2, network, t.Name(), testDir)
	n3 := newNode(3, network, t.Name(), testDir)
	n4 := newNode(4, network, t.Name(), testDir)

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

	n0 := newNode(0, network, t.Name(), testDir)
	n1 := newNode(1, network, t.Name(), testDir)
	n2 := newNode(2, network, t.Name(), testDir)
	n3 := newNode(3, network, t.Name(), testDir)

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

	n0 := newNode(0, network, t.Name(), testDir)
	n1 := newNode(1, network, t.Name(), testDir)
	n2 := newNode(2, network, t.Name(), testDir)
	n3 := newNode(3, network, t.Name(), testDir)

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

func TestLeaderInPartitionWithHealing(t *testing.T) {
	t.Parallel()

	network := make(Network)
	defer network.Shutdown()

	testDir, err := ioutil.TempDir("", t.Name())
	assert.NoErrorf(t, err, "generate temporary test dir")
	defer os.RemoveAll(testDir)

	logConfig := zap.NewDevelopmentConfig()
	logger, _ := logConfig.Build()
	logger = logger.With(zap.String("t", t.Name())).With(zap.String("id", "TEST"))
	sugaredLogger := logger.Sugar()

	n0 := newNode(0, network, t.Name(), testDir)
	n1 := newNode(1, network, t.Name(), testDir)
	n2 := newNode(2, network, t.Name(), testDir)
	n3 := newNode(3, network, t.Name(), testDir)

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
	sugaredLogger.Infof("Disconnected n0")

	n1.Submit(Request{ID: "3", ClientID: "alice"}) // submit to other nodes
	n2.Submit(Request{ID: "3", ClientID: "alice"})
	n3.Submit(Request{ID: "3", ClientID: "alice"})

	data1Tx3 := <-n1.Delivered
	data2 = <-n2.Delivered
	data3 = <-n3.Delivered
	assert.Equal(t, data1Tx3, data2)
	assert.Equal(t, data2, data3)

	n1.Submit(Request{ID: "4", ClientID: "alice"})
	n2.Submit(Request{ID: "4", ClientID: "alice"})
	n3.Submit(Request{ID: "4", ClientID: "alice"})

	data1Tx4 := <-n1.Delivered
	data2 = <-n2.Delivered
	data3 = <-n3.Delivered
	assert.Equal(t, data1Tx4, data2)
	assert.Equal(t, data2, data3)

	n0.Connect() // partition heals, leader should eventually sync and deliver
	sugaredLogger.Infof("Connected n0")

	data0 = <-n0.Delivered
	assert.Equal(t, data1Tx3, data0)

	data0 = <-n0.Delivered
	assert.Equal(t, data1Tx4, data0)
}

func TestMultiLeadersPartition(t *testing.T) {
	t.Parallel()
	network := make(Network)
	defer network.Shutdown()

	testDir, err := ioutil.TempDir("", t.Name())
	assert.NoErrorf(t, err, "generate temporary test dir")
	defer os.RemoveAll(testDir)

	n0 := newNode(0, network, t.Name(), testDir)
	n1 := newNode(1, network, t.Name(), testDir)
	n2 := newNode(2, network, t.Name(), testDir)
	n3 := newNode(3, network, t.Name(), testDir)
	n4 := newNode(4, network, t.Name(), testDir)
	n5 := newNode(5, network, t.Name(), testDir)
	n6 := newNode(6, network, t.Name(), testDir)

	start := time.Now()
	n2.viewChangeTime = make(chan time.Time, 1)
	n3.viewChangeTime = make(chan time.Time, 1)
	n4.viewChangeTime = make(chan time.Time, 1)
	n5.viewChangeTime = make(chan time.Time, 1)
	n6.viewChangeTime = make(chan time.Time, 1)
	n2.viewChangeTime <- start
	n3.viewChangeTime <- start
	n4.viewChangeTime <- start
	n5.viewChangeTime <- start
	n6.viewChangeTime <- start
	n2.Setup()
	n3.Setup()
	n4.Setup()
	n5.Setup()
	n6.Setup()

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

	done := make(chan struct{})
	defer close(done)
	// Accelerate the time for a view change timeout
	go func() {
		var i int
		for {
			i++
			select {
			case <-done:
				return
			case <-time.After(time.Millisecond * 100):
				n2.viewChangeTime <- time.Now().Add(time.Second * time.Duration(10*i))
				n3.viewChangeTime <- time.Now().Add(time.Second * time.Duration(10*i))
				n4.viewChangeTime <- time.Now().Add(time.Second * time.Duration(10*i))
				n5.viewChangeTime <- time.Now().Add(time.Second * time.Duration(10*i))
				n6.viewChangeTime <- time.Now().Add(time.Second * time.Duration(10*i))
			}
		}
	}()

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

func TestHeartbeatTimeoutCausesViewChange(t *testing.T) {
	t.Parallel()
	network := make(Network)
	defer network.Shutdown()

	testDir, err := ioutil.TempDir("", t.Name())
	assert.NoErrorf(t, err, "generate temporary test dir")
	defer os.RemoveAll(testDir)

	n0 := newNode(0, network, t.Name(), testDir)
	n1 := newNode(1, network, t.Name(), testDir)
	n2 := newNode(2, network, t.Name(), testDir)
	n3 := newNode(3, network, t.Name(), testDir)

	start := time.Now()
	n1.heartbeatTime = make(chan time.Time, 1)
	n2.heartbeatTime = make(chan time.Time, 1)
	n3.heartbeatTime = make(chan time.Time, 1)
	n1.heartbeatTime <- start
	n2.heartbeatTime <- start
	n3.heartbeatTime <- start
	n1.Setup()
	n2.Setup()
	n3.Setup()

	// wait for the new leader to finish the view change before submitting
	done := make(chan struct{})
	viewChangeWG := sync.WaitGroup{}
	viewChangeWG.Add(3)
	for _, n := range network {
		baseLogger := n.app.Consensus.Logger.(*zap.SugaredLogger).Desugar()
		n.app.Consensus.Logger = baseLogger.WithOptions(zap.Hooks(func(entry zapcore.Entry) error {
			if strings.Contains(entry.Message, "ViewChanged, the new view is 1") {
				viewChangeWG.Done()
			}
			return nil
		})).Sugar()
	}

	n0.Consensus.Start()

	n0.Disconnect() // leader in partition

	n1.Consensus.Start()
	n2.Consensus.Start()
	n3.Consensus.Start()

	// Accelerate the time until a view change because of heartbeat timeout
	go func() {
		var i int
		for {
			i++
			select {
			case <-done:
				return
			case <-time.After(time.Millisecond * 100):
				n1.heartbeatTime <- time.Now().Add(time.Second * time.Duration(10*i))
				n2.heartbeatTime <- time.Now().Add(time.Second * time.Duration(10*i))
				n3.heartbeatTime <- time.Now().Add(time.Second * time.Duration(10*i))
			}
		}
	}()

	viewChangeWG.Wait()
	close(done)

	n1.Submit(Request{ID: "1", ClientID: "alice"}) // submit to new leader
	n2.Submit(Request{ID: "1", ClientID: "alice"}) // submit to follower
	n3.Submit(Request{ID: "1", ClientID: "alice"}) // submit to follower

	data1 := <-n1.Delivered
	data2 := <-n2.Delivered
	data3 := <-n3.Delivered

	assert.Equal(t, data1, data2)
	assert.Equal(t, data2, data3)
}

func TestMultiViewChangeWithNoRequestsTimeout(t *testing.T) {
	t.Parallel()
	network := make(Network)
	defer network.Shutdown()

	testDir, err := ioutil.TempDir("", t.Name())
	assert.NoErrorf(t, err, "generate temporary test dir")
	defer os.RemoveAll(testDir)

	n0 := newNode(0, network, t.Name(), testDir)
	n1 := newNode(1, network, t.Name(), testDir)
	n2 := newNode(2, network, t.Name(), testDir)
	n3 := newNode(3, network, t.Name(), testDir)
	n4 := newNode(4, network, t.Name(), testDir)
	n5 := newNode(5, network, t.Name(), testDir)
	n6 := newNode(6, network, t.Name(), testDir)

	start := time.Now()
	for _, n := range network {
		n.app.heartbeatTime = make(chan time.Time, 1)
		n.app.heartbeatTime <- start
		n.app.viewChangeTime = make(chan time.Time, 1)
		n.app.viewChangeTime <- start
		n.app.Setup()
	}

	// wait for the new leader to finish the view change before submitting
	done := make(chan struct{})
	viewChangeWG := sync.WaitGroup{}
	viewChangeWG.Add(5)
	for _, n := range network {
		baseLogger := n.app.Consensus.Logger.(*zap.SugaredLogger).Desugar()
		n.app.Consensus.Logger = baseLogger.WithOptions(zap.Hooks(func(entry zapcore.Entry) error {
			if strings.Contains(entry.Message, "ViewChanged, the new view is 2") {
				viewChangeWG.Done()
			}
			return nil
		})).Sugar()
	}

	n0.Consensus.Start()
	n1.Consensus.Start()

	n0.Disconnect() // leader in partition
	n1.Disconnect() // next leader in partition

	n2.Consensus.Start()
	n3.Consensus.Start()
	n4.Consensus.Start()
	n5.Consensus.Start()
	n6.Consensus.Start()

	// Accelerate the time until a view change
	go func() {
		var i int
		for {
			i++
			select {
			case <-done:
				return
			case <-time.After(time.Millisecond * 100):
				for _, n := range network {
					n.app.heartbeatTime <- time.Now().Add(time.Second * time.Duration(2*i))
					n.app.viewChangeTime <- time.Now().Add(time.Second * time.Duration(10*i))
				}
			}
		}
	}()

	viewChangeWG.Wait()
	close(done)

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

	n0 := newNode(0, network, t.Name(), testDir)
	n1 := newNode(1, network, t.Name(), testDir)
	n2 := newNode(2, network, t.Name(), testDir)
	n3 := newNode(3, network, t.Name(), testDir)

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

func TestLeaderCatchingUpAfterViewChange(t *testing.T) {
	t.Parallel()
	network := make(Network)
	defer network.Shutdown()

	testDir, err := ioutil.TempDir("", t.Name())
	assert.NoErrorf(t, err, "generate temporary test dir")
	defer os.RemoveAll(testDir)

	n0 := newNode(0, network, t.Name(), testDir)
	n1 := newNode(1, network, t.Name(), testDir)
	n2 := newNode(2, network, t.Name(), testDir)
	n3 := newNode(3, network, t.Name(), testDir)

	n0.Consensus.Start()
	n1.Consensus.Start()
	n2.Consensus.Start()
	n3.Consensus.Start()

	n0.Submit(Request{ID: "1", ClientID: "alice"}) // submit to leader

	data0Seq1 := <-n0.Delivered
	data1Seq1 := <-n1.Delivered
	data2Seq1 := <-n2.Delivered
	data3Seq1 := <-n3.Delivered
	assert.Equal(t, data0Seq1, data1Seq1)
	assert.Equal(t, data1Seq1, data2Seq1)
	assert.Equal(t, data2Seq1, data3Seq1)

	n0.Disconnect() // leader in partition

	n1.Submit(Request{ID: "2", ClientID: "alice"}) // submit to new leader
	n2.Submit(Request{ID: "2", ClientID: "alice"})
	n3.Submit(Request{ID: "2", ClientID: "alice"})

	data1Seq2 := <-n1.Delivered
	data2Seq2 := <-n2.Delivered
	data3Seq2 := <-n3.Delivered
	assert.Equal(t, data1Seq2, data2Seq2)
	assert.Equal(t, data2Seq2, data3Seq2)

	n0.Connect() // old leader woke up

	// We create new batches until it catches up
	for reqID := 3; reqID < 100; reqID++ {
		n1.Submit(Request{ID: fmt.Sprintf("%d", reqID), ClientID: "alice"})
		n2.Submit(Request{ID: fmt.Sprintf("%d", reqID), ClientID: "alice"})
		<-n1.Delivered // Wait for new leader to commit
		<-n2.Delivered // Wait for follower to commit
		caughtUp := waitForCatchup(reqID, n0.Delivered)
		if caughtUp {
			return
		}
	}
	t.Fatalf("Didn't catch up")
}

func TestRestartAfterViewChangeAndRestoreNewView(t *testing.T) {
	t.Parallel()
	network := make(Network)
	defer network.Shutdown()

	testDir, err := ioutil.TempDir("", t.Name())
	assert.NoErrorf(t, err, "generate temporary test dir")
	defer os.RemoveAll(testDir)

	n0 := newNode(0, network, t.Name(), testDir)
	n1 := newNode(1, network, t.Name(), testDir)
	n2 := newNode(2, network, t.Name(), testDir)
	n3 := newNode(3, network, t.Name(), testDir)

	start := time.Now()
	for _, n := range network {
		n.app.heartbeatTime = make(chan time.Time, 1)
		n.app.heartbeatTime <- start
		n.app.Setup()
	}

	// wait for a view change to occur
	done := make(chan struct{})
	viewChangeWG := sync.WaitGroup{}
	viewChangeWG.Add(2)
	baseLogger1 := n1.Consensus.Logger.(*zap.SugaredLogger).Desugar()
	n1.Consensus.Logger = baseLogger1.WithOptions(zap.Hooks(func(entry zapcore.Entry) error {
		if strings.Contains(entry.Message, "ViewChanged, the new view is 1") {
			viewChangeWG.Done()
		}
		return nil
	})).Sugar()
	baseLogger3 := n3.Consensus.Logger.(*zap.SugaredLogger).Desugar()
	n3.Consensus.Logger = baseLogger3.WithOptions(zap.Hooks(func(entry zapcore.Entry) error {
		if strings.Contains(entry.Message, "ViewChanged, the new view is 1") {
			viewChangeWG.Done()
		}
		return nil
	})).Sugar()

	n0.Consensus.Start()
	n1.Consensus.Start()
	n2.Consensus.Start()
	n3.Consensus.Start()

	n0.Disconnect()

	// Accelerate the time until a view change
	go func() {
		var i int
		for {
			i++
			select {
			case <-done:
				return
			case <-time.After(time.Millisecond * 100):
				for _, n := range network {
					n.app.heartbeatTime <- time.Now().Add(time.Second * time.Duration(2*i))
				}
			}
		}
	}()

	viewChangeWG.Wait()
	close(done)

	// restart new leader and a follower, will restore from new view
	n1.Restart()
	n3.Restart()

	n1.Submit(Request{ID: "1", ClientID: "alice"})
	n2.Submit(Request{ID: "1", ClientID: "alice"})
	n3.Submit(Request{ID: "1", ClientID: "alice"})

	data1 := <-n1.Delivered
	data2 := <-n2.Delivered
	data3 := <-n3.Delivered

	assert.Equal(t, data1, data2)
	assert.Equal(t, data2, data3)

}

func TestRestoringViewChange(t *testing.T) {
	t.Parallel()
	network := make(Network)
	defer network.Shutdown()

	testDir, err := ioutil.TempDir("", t.Name())
	assert.NoErrorf(t, err, "generate temporary test dir")
	defer os.RemoveAll(testDir)

	n0 := newNode(0, network, t.Name(), testDir)
	n1 := newNode(1, network, t.Name(), testDir)
	n2 := newNode(2, network, t.Name(), testDir)
	n3 := newNode(3, network, t.Name(), testDir)
	n4 := newNode(4, network, t.Name(), testDir)
	n5 := newNode(5, network, t.Name(), testDir)
	n6 := newNode(6, network, t.Name(), testDir)

	start := time.Now()
	for _, n := range network {
		n.app.heartbeatTime = make(chan time.Time, 1)
		n.app.heartbeatTime <- start
		n.app.viewChangeTime = make(chan time.Time, 1)
		n.app.viewChangeTime <- start
	}

	done := make(chan struct{})
	viewChangeFinishWG := sync.WaitGroup{}
	viewChangeFinishWG.Add(1)
	viewChangeFinishOnce := sync.Once{}
	viewChangeWG := sync.WaitGroup{}
	viewChangeWG.Add(1)
	viewChangeOnce := sync.Once{}
	baseLogger := n6.logger.Desugar()
	n6.logger = baseLogger.WithOptions(zap.Hooks(func(entry zapcore.Entry) error {
		if strings.Contains(entry.Message, "Node 6 sent view data msg, with next view 1, to the new leader 1") {
			viewChangeOnce.Do(func() {
				viewChangeWG.Done()
			})
		}
		if strings.Contains(entry.Message, "ViewChanged, the new view is 2") {
			viewChangeFinishOnce.Do(func() {
				viewChangeFinishWG.Done()
			})
		}
		return nil
	})).Sugar()

	for _, n := range network {
		n.app.Setup()
	}

	n0.Consensus.Start()
	n1.Consensus.Start()

	n0.Disconnect() // leader in partition
	n1.Disconnect() // next leader in partition

	n2.Consensus.Start()
	n3.Consensus.Start()
	n4.Consensus.Start()
	n5.Consensus.Start()
	n6.Consensus.Start()

	// Accelerate the time until a view change
	go func() {
		var i int
		for {
			i++
			select {
			case <-done:
				return
			case <-time.After(time.Millisecond * 100):
				for _, n := range network {
					n.app.heartbeatTime <- time.Now().Add(time.Second * time.Duration(2*i))
					n.app.viewChangeTime <- time.Now().Add(time.Second * time.Duration(2*i))
				}
			}
		}
	}()

	viewChangeWG.Wait()
	n6.Disconnect()
	n6.Restart()
	n6.Connect()

	viewChangeFinishWG.Wait()
	close(done)

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

func TestLeaderForwarding(t *testing.T) {
	t.Parallel()
	network := make(Network)
	defer network.Shutdown()

	testDir, err := ioutil.TempDir("", t.Name())
	assert.NoErrorf(t, err, "generate temporary test dir")
	defer os.RemoveAll(testDir)

	n0 := newNode(0, network, t.Name(), testDir)
	n1 := newNode(1, network, t.Name(), testDir)
	n2 := newNode(2, network, t.Name(), testDir)
	n3 := newNode(3, network, t.Name(), testDir)

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

	n0 := newNode(0, network, t.Name(), testDir)
	n1 := newNode(1, network, t.Name(), testDir)
	n2 := newNode(2, network, t.Name(), testDir)
	n3 := newNode(3, network, t.Name(), testDir)

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
	t.Fatalf("Didn't catch up")
}

func TestCatchingUpWithSyncAssisted(t *testing.T) {
	t.Parallel()
	network := make(Network)
	defer network.Shutdown()

	testDir, err := ioutil.TempDir("", t.Name())
	assert.NoErrorf(t, err, "generate temporary test dir")
	defer os.RemoveAll(testDir)

	n0 := newNode(0, network, t.Name(), testDir)
	n1 := newNode(1, network, t.Name(), testDir)
	n2 := newNode(2, network, t.Name(), testDir)
	n3 := newNode(3, network, t.Name(), testDir)

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
	t.Fatalf("Didn't catch up")
}

func TestCatchingUpWithSyncAutonomous(t *testing.T) {
	t.Parallel()
	network := make(Network)
	defer network.Shutdown()

	testDir, err := ioutil.TempDir("", t.Name())
	assert.NoErrorf(t, err, "generate temporary test dir")
	defer os.RemoveAll(testDir)

	n0 := newNode(0, network, t.Name(), testDir)
	n1 := newNode(1, network, t.Name(), testDir)
	n2 := newNode(2, network, t.Name(), testDir)
	n3 := newNode(3, network, t.Name(), testDir)

	var detectedSequenceGap uint32

	baseLogger := n3.Consensus.Logger.(*zap.SugaredLogger).Desugar()
	n3.Consensus.Logger = baseLogger.WithOptions(zap.Hooks(func(entry zapcore.Entry) error {
		if strings.Contains(entry.Message, "Leader's sequence is 10 and ours is 1") {
			atomic.StoreUint32(&detectedSequenceGap, 1)
		}
		return nil
	})).Sugar()

	start := time.Now()
	n0.heartbeatTime = make(chan time.Time, 1)
	n0.heartbeatTime <- start
	n3.heartbeatTime = make(chan time.Time, 1)
	n3.heartbeatTime <- start
	n3.viewChangeTime = make(chan time.Time, 1)
	n3.viewChangeTime <- start
	n0.Setup()
	n3.Setup()

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

	done := make(chan struct{})
	// Accelerate the time for n3 so it will suspect the leader and view change.
	go func() {
		var i int
		for {
			i++
			select {
			case <-done:
				return
			case <-time.After(time.Millisecond * 100):
				n0.heartbeatTime <- time.Now().Add(time.Second * time.Duration(10*i))
				n3.heartbeatTime <- time.Now().Add(time.Second * time.Duration(10*i))
				n3.viewChangeTime <- time.Now().Add(time.Minute * time.Duration(10*i))
			}
		}
	}()

	for i := 1; i <= 10; i++ {
		select {
		case <-n3.Delivered:
		case <-time.After(time.Second * 10):
			t.Fatalf("Didn't catch up within a timely period")
		}
	}

	close(done)
	assert.Equal(t, uint32(0), atomic.LoadUint32(&detectedSequenceGap))
}

func TestFollowerStateTransfer(t *testing.T) {
	// Scenario: the leader (n0) is disconnected and so there is a view change
	// a follower (n6) is also disconnected and misses the view change
	// after the follower reconnects and gets a view change timeout is calls sync
	// where it collects state transfer requests and sees that there was a view change

	t.Parallel()
	network := make(Network)
	defer network.Shutdown()

	testDir, err := ioutil.TempDir("", t.Name())
	assert.NoErrorf(t, err, "generate temporary test dir")
	defer os.RemoveAll(testDir)

	n0 := newNode(0, network, t.Name(), testDir)
	n1 := newNode(1, network, t.Name(), testDir)
	n2 := newNode(2, network, t.Name(), testDir)
	n3 := newNode(3, network, t.Name(), testDir)
	n4 := newNode(4, network, t.Name(), testDir)
	n5 := newNode(5, network, t.Name(), testDir)
	n6 := newNode(6, network, t.Name(), testDir)

	start := time.Now()
	for _, n := range network {
		n.app.heartbeatTime = make(chan time.Time, 1)
		n.app.heartbeatTime <- start
		n.app.viewChangeTime = make(chan time.Time, 1)
		n.app.viewChangeTime <- start
	}

	syncedWG := sync.WaitGroup{}
	syncedWG.Add(1)
	baseLogger6 := n6.logger.Desugar()
	n6.logger = baseLogger6.WithOptions(zap.Hooks(func(entry zapcore.Entry) error {
		if strings.Contains(entry.Message, "The collected state is with view 1 and sequence 1") {
			syncedWG.Done()
		}
		return nil
	})).Sugar()

	viewChangeWG := sync.WaitGroup{}
	viewChangeWG.Add(1)
	baseLogger1 := n1.logger.Desugar()
	n1.logger = baseLogger1.WithOptions(zap.Hooks(func(entry zapcore.Entry) error {
		if strings.Contains(entry.Message, "ViewChanged, the new view is 1") {
			viewChangeWG.Done()
		}
		return nil
	})).Sugar()

	for _, n := range network {
		n.app.Setup()
	}

	n0.Consensus.Start()
	n0.Disconnect() // leader in partition
	n6.Consensus.Start()
	n6.Disconnect() // follower in partition

	n1.Consensus.Start()
	n2.Consensus.Start()
	n3.Consensus.Start()
	n4.Consensus.Start()
	n5.Consensus.Start()

	// Accelerate the time until a view change
	done := make(chan struct{})
	go func() {
		var i int
		for {
			i++
			select {
			case <-done:
				return
			case <-time.After(time.Millisecond * 100):
				for _, n := range network {
					n.app.heartbeatTime <- time.Now().Add(time.Second * time.Duration(2*i))
					n.app.viewChangeTime <- time.Now().Add(time.Second * time.Duration(2*i))
				}
			}
		}
	}()

	viewChangeWG.Wait()
	n6.Connect()
	syncedWG.Wait()
	close(done)

	n1.Submit(Request{ID: "1", ClientID: "alice"}) // submit to new leader
	n2.Submit(Request{ID: "1", ClientID: "alice"})
	n3.Submit(Request{ID: "1", ClientID: "alice"})
	n4.Submit(Request{ID: "1", ClientID: "alice"})
	n5.Submit(Request{ID: "1", ClientID: "alice"})
	n6.Submit(Request{ID: "1", ClientID: "alice"})

	data1 := <-n1.Delivered
	data2 := <-n2.Delivered
	data3 := <-n3.Delivered
	data4 := <-n4.Delivered
	data5 := <-n5.Delivered
	data6 := <-n6.Delivered

	assert.Equal(t, data1, data2)
	assert.Equal(t, data2, data3)
	assert.Equal(t, data3, data4)
	assert.Equal(t, data4, data5)
	assert.Equal(t, data5, data6)
	assert.Equal(t, data6, data2)

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
