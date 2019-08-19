// Copyright IBM Corp. All Rights Reserved.
//
// SPDX-License-Identifier: Apache-2.0
//

package bft

import (
	"encoding/asn1"
	"math"
	"sort"
	"sync/atomic"

	"sync"

	"time"

	"github.com/SmartBFT-Go/consensus/pkg/api"
	"github.com/SmartBFT-Go/consensus/pkg/types"
	protos "github.com/SmartBFT-Go/consensus/smartbftprotos"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
)

func viewNumber(m *protos.Message) uint64 {
	if pp := m.GetPrePrepare(); pp != nil {
		return pp.GetView()
	}

	if prp := m.GetPrepare(); prp != nil {
		return prp.GetView()
	}

	if cmt := m.GetCommit(); cmt != nil {
		return cmt.GetView()
	}

	return math.MaxUint64
}

func proposalSequence(m *protos.Message) uint64 {
	if pp := m.GetPrePrepare(); pp != nil {
		return pp.Seq
	}

	if prp := m.GetPrepare(); prp != nil {
		return prp.Seq
	}

	if cmt := m.GetCommit(); cmt != nil {
		return cmt.Seq
	}

	return math.MaxUint64
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func MarshalOrPanic(msg proto.Message) []byte {
	b, err := proto.Marshal(msg)
	if err != nil {
		panic(err)
	}
	return b
}

func getLeaderID(view uint64, N uint64, nodes []uint64) uint64 {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i] < nodes[j]
	})
	return nodes[view%N]
}

type vote struct {
	*protos.Message
	sender uint64
}

type voteSet struct {
	validVote func(voter uint64, message *protos.Message) bool
	voted     map[uint64]struct{}
	votes     chan *vote
}

func (vs *voteSet) clear(n uint64) {
	// Drain the votes channel
	for len(vs.votes) > 0 {
		<-vs.votes
	}

	vs.voted = make(map[uint64]struct{}, n)
	vs.votes = make(chan *vote, n)
}

func (vs *voteSet) registerVote(voter uint64, message *protos.Message) {
	if !vs.validVote(voter, message) {
		return
	}

	_, hasVoted := vs.voted[voter]
	if hasVoted {
		// Received double vote
		return
	}

	vs.voted[voter] = struct{}{}
	vs.votes <- &vote{Message: message, sender: voter}
}

type incMsg struct {
	*protos.Message
	sender uint64
}

type TBSPrepare struct {
	View   int64
	Seq    int64
	Digest string
}

func (tbsp TBSPrepare) ToBytes() []byte {
	bytes, err := asn1.Marshal(tbsp)
	if err != nil {
		panic(errors.Errorf("failed marshaling prepare %v: %v", tbsp, err))
	}
	return bytes
}

// computeQuorum calculates the quorums size Q, given a cluster size N.
//
// The calculation satisfies the following:
// Given a cluster size of N nodes, which tolerates f failures according to:
//    f = argmax ( N >= 3f+1 )
// Q is the size of the quorum such that:
//    any two subsets q1, q2 of size Q, intersect in at least f+1 nodes.
//
// Note that this is different from N-f (the number of correct nodes), when N=3f+3. That is, we have two extra nodes
// above the minimum required to tolerate f failures.
func computeQuorum(N uint64) (Q int, F int) {
	F = int((int(N) - 1) / 3)
	Q = int(math.Ceil((float64(N) + float64(F) + 1) / 2.0))
	return
}

// InFlightData records proposals that are in-flight,
// as well as their corresponding prepares.
type InFlightData struct {
	v atomic.Value
}

type inFlightProposalData struct {
	proposal *types.Proposal
	prepares map[uint64]*protos.Prepare
}

// InFlightData returns an in-flight proposal or nil if there is no such.
func (ifp *InFlightData) InFlightProposal() *types.Proposal {
	fetched := ifp.v.Load()
	if fetched == nil {
		return nil
	}

	data := fetched.(inFlightProposalData)
	return data.proposal
}

func (ifp *InFlightData) InFlightPrepares() map[uint64]*protos.Prepare {
	fetched := ifp.v.Load()
	if fetched == nil {
		return nil
	}
	data := fetched.(inFlightProposalData)
	return data.prepares
}

// Store stores an in-flight proposal.
func (ifp *InFlightData) StoreProposal(prop types.Proposal) {
	p := prop
	ifp.v.Store(inFlightProposalData{proposal: &p})
}

func (ifp *InFlightData) StorePrepares(view, seq uint64, signatures map[uint64][]byte) {
	prop := ifp.InFlightProposal()
	if prop == nil {
		panic("stored prepares but proposal is not initialized")
	}

	dig := prop.Digest()
	prepares := make(map[uint64]*protos.Prepare)
	for signer, sig := range signatures {
		prepare := &protos.Prepare{
			Seq:       seq,
			View:      view,
			Digest:    dig,
			Signature: sig,
		}
		prepares[signer] = prepare
	}
	p := prop
	ifp.v.Store(inFlightProposalData{proposal: p, prepares: prepares})
}

type ProposalMaker struct {
	N               uint64
	SelfID          uint64
	Decider         Decider
	FailureDetector FailureDetector
	Sync            api.Synchronizer
	Logger          api.Logger
	Comm            Comm
	Verifier        api.Verifier
	Signer          api.Signer
	State           State

	restoreOnceFromWAL sync.Once
}

func (pm *ProposalMaker) NewProposer(leader, proposalSequence, viewNum uint64, quorumSize int) Proposer {
	view := &View{
		N:                pm.N,
		LeaderID:         leader,
		SelfID:           pm.SelfID,
		Quorum:           quorumSize,
		Number:           viewNum,
		Decider:          pm.Decider,
		FailureDetector:  pm.FailureDetector,
		Sync:             pm.Sync,
		Logger:           pm.Logger,
		Comm:             pm.Comm,
		Verifier:         pm.Verifier,
		Signer:           pm.Signer,
		ProposalSequence: proposalSequence,
		State:            pm.State,
	}

	pm.restoreOnceFromWAL.Do(func() {
		err := pm.State.Restore(view)
		if err != nil {
			pm.Logger.Panicf("Failed restoring view from WAL: %v", err)
		}
	})

	if proposalSequence > view.ProposalSequence {
		view.ProposalSequence = proposalSequence
	}

	if viewNum > view.Number {
		view.Number = viewNum
	}

	return view
}

type TickDemultiplexer struct {
	stopChan chan struct{}
	stopOnce sync.Once
	running  sync.WaitGroup
	In       <-chan time.Time
	Out      []chan<- time.Time
}

func (td *TickDemultiplexer) Start() {
	td.stopChan = make(chan struct{})
	td.running.Add(1)
	go func() {
		defer td.running.Done()
		for {
			select {
			case <-td.stopChan:
				return
			case now := <-td.In:
				td.tick(now)
			}
		}
	}()
}

func (td *TickDemultiplexer) tick(now time.Time) {
	var wg sync.WaitGroup
	wg.Add(len(td.Out))
	for _, out := range td.Out {
		go func(out chan<- time.Time) {
			defer wg.Done()
			out <- now
		}(out)
	}
	wg.Wait()
}

func (td *TickDemultiplexer) Stop() {
	td.stopOnce.Do(func() {
		close(td.stopChan)
		td.running.Wait()
	})
}
