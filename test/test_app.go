// Copyright IBM Corp. All Rights Reserved.
//
// SPDX-License-Identifier: Apache-2.0
//

package test

import (
	"encoding/asn1"
	"time"

	"github.com/SmartBFT-Go/consensus/pkg/consensus"
	"github.com/SmartBFT-Go/consensus/pkg/types"
	"github.com/SmartBFT-Go/consensus/pkg/wal"
	"github.com/SmartBFT-Go/consensus/smartbftprotos"
	"github.com/golang/protobuf/proto"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type App struct {
	ID          uint64
	Delivered   chan *AppRecord
	Consensus   *consensus.Consensus
	Setup       func()
	Node        *Node
	logLevel    zap.AtomicLevel
	latestMD    *smartbftprotos.ViewMetadata
	clock       *time.Ticker
	secondClock *time.Ticker
	timeChannel chan time.Time
}

func (a *App) Mute() {
	a.logLevel.SetLevel(zapcore.PanicLevel)
}

func (a *App) UnMute() {
	a.logLevel.SetLevel(zapcore.DebugLevel)
}

func (a *App) Submit(req Request) {
	a.Consensus.SubmitRequest(req.ToBytes())
}

func (a *App) setTimeChannel(timeChannel chan time.Time) {
	a.timeChannel = timeChannel
}

func (a *App) Sync() (smartbftprotos.ViewMetadata, uint64) {
	panic("implement me")
}

func (a *App) Restart() {
	a.Node.Lock()
	defer a.Node.Unlock()
	a.Consensus.Stop()
	a.Setup()
	a.Consensus.Start()
}

func (a *App) Disconnect() {
	a.Node.Lock()
	defer a.Node.Unlock()
	a.Node.lossProbability = 1
}

func (a *App) Connect() {
	a.Node.Lock()
	defer a.Node.Unlock()
	a.Node.lossProbability = 0
}

func (a *App) RequestID(req []byte) types.RequestInfo {
	txn := requestFromBytes(req)
	return types.RequestInfo{
		ClientID: txn.ClientID,
		ID:       txn.ID,
	}
}

func (a *App) VerifyProposal(proposal types.Proposal) ([]types.RequestInfo, error) {
	blockData := BatchFromBytes(proposal.Payload)
	requests := make([]types.RequestInfo, 0)
	for _, t := range blockData.Requests {
		req := requestFromBytes(t)
		reqInfo := types.RequestInfo{ID: req.ID, ClientID: req.ClientID}
		requests = append(requests, reqInfo)
	}
	return requests, nil
}

func (a *App) VerifyRequest(val []byte) (types.RequestInfo, error) {
	req := requestFromBytes(val)
	return types.RequestInfo{ID: req.ID, ClientID: req.ClientID}, nil
}

func (a *App) VerifyConsenterSig(signature types.Signature, prop types.Proposal) error {
	return nil
}

func (a *App) VerifySignature(signature types.Signature) error {
	return nil
}

func (a *App) VerificationSequence() uint64 {
	return 0
}

func (a *App) Sign([]byte) []byte {
	return nil
}

func (a *App) SignProposal(types.Proposal) *types.Signature {
	return &types.Signature{Id: a.ID}
}

func (a *App) AssembleProposal(metadata []byte, requests [][]byte) (nextProp types.Proposal, remainder [][]byte) {
	return types.Proposal{
		Payload:  Batch{Requests: requests}.ToBytes(),
		Metadata: metadata,
	}, nil
}

func (a *App) Deliver(proposal types.Proposal, signature []types.Signature) {
	a.Delivered <- &AppRecord{
		Metadata: proposal.Metadata,
		Batch:    BatchFromBytes(proposal.Payload),
	}
	a.latestMD = &smartbftprotos.ViewMetadata{}
	proto.Unmarshal(proposal.Metadata, a.latestMD)
}

type Request struct {
	ClientID string
	ID       string
}

func (txn Request) ToBytes() []byte {
	rawTxn, err := asn1.Marshal(txn)
	if err != nil {
		panic(err)
	}
	return rawTxn
}

func requestFromBytes(req []byte) *Request {
	var r Request
	asn1.Unmarshal(req, &r)
	return &r
}

type Batch struct {
	Requests [][]byte
}

func (b Batch) ToBytes() []byte {
	rawBlock, err := asn1.Marshal(b)
	if err != nil {
		panic(err)
	}
	return rawBlock
}

func BatchFromBytes(rawBlock []byte) *Batch {
	var block Batch
	asn1.Unmarshal(rawBlock, &block)
	return &block
}

type AppRecord struct {
	Batch    *Batch
	Metadata []byte
}

func newNode(id uint64, network Network, testName string) *App {
	logConfig := zap.NewDevelopmentConfig()
	logger, _ := logConfig.Build()
	logger = logger.With(zap.String("t", testName)).With(zap.Int64("id", int64(id)))

	app := &App{
		clock:     time.NewTicker(time.Second),
		ID:        id,
		Delivered: make(chan *AppRecord, 100),
		logLevel:  logConfig.Level,
		latestMD:  &smartbftprotos.ViewMetadata{},
	}

	wal := &wal.EphemeralWAL{}

	app.Setup = func() {
		c := &consensus.Consensus{
			Scheduler:         app.clock.C,
			SelfID:            id,
			Logger:            logger.Sugar(),
			WAL:               wal,
			Metadata:          *app.latestMD,
			Verifier:          app,
			Signer:            app,
			RequestInspector:  app,
			Assembler:         app,
			Synchronizer:      app,
			Application:       app,
			BatchSize:         10,
			BatchTimeout:      time.Millisecond,
			WALInitialContent: wal.ReadAll(),
			LastProposal:      types.Proposal{},
			LastSignatures:    []types.Signature{},
		}
		if app.timeChannel != nil {
			c.Scheduler = app.timeChannel
		}
		network.AddOrUpdateNode(id, c)
		c.Comm = network[id]
		app.Consensus = c
	}
	app.Setup()
	app.Node = network[id]
	return app
}
