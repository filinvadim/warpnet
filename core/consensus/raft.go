package consensus

import (
	"context"
	"github.com/filinvadim/warpnet/database"
	"github.com/libp2p/go-libp2p/core/host"
	"log"
	"os"
	"time"

	"github.com/hashicorp/raft"
	"github.com/libp2p/go-libp2p-raft"
)

func NewRaft(
	ctx context.Context,
	node host.Host,
	consRepo *database.ConsensusRepo,
	isBootstrap bool,
) {
	var (
		err           error
		logStore      raft.LogStore
		stableStore   raft.StableStore
		snapshotStore raft.SnapshotStore
	)

	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(node.ID().String())

	if isBootstrap {
		logStore = raft.NewInmemStore()              // In-memory хранилище логов
		stableStore = raft.NewInmemStore()           // In-memory стабильное хранилище
		snapshotStore = raft.NewInmemSnapshotStore() // Хранилище снапшотов
	} else {
		logStore = consRepo
		stableStore = consRepo
		snapshotStore, err = raft.NewFileSnapshotStore(consRepo.SnapshotPath(), 5, os.Stdout)
		if err != nil {
			log.Fatalf("Failed to create snapshot store: %v", err)
		}
	}

	transport, err := libp2praft.NewLibp2pTransport(node, time.Second*30)
	if err != nil {
		log.Fatalf("Failed to create raft transport: %v", err)
	}

	state := map[string]string{}
	consensus := libp2praft.NewConsensus(&state)
	// Получаем FSM через Consensus
	fsm := consensus.FSM()

	// Создаём узел Raft
	raftNode, err := raft.NewRaft(
		config,
		fsm,
		logStore, stableStore,
		snapshotStore,
		transport,
	)
	if err != nil {
		log.Fatalf("Failed to create raft node: %v", err)
	}

	raftNode.BootstrapCluster(raft.Configuration{
		Servers: []raft.Server{
			{
				ID:      config.LocalID,
				Address: transport.LocalAddr(),
			},
		},
	})
	defer func() {
		wait := raftNode.Shutdown()
		if wait != nil && wait.Error() != nil {
			log.Fatalf("failed to shutdown raft: %v", wait.Error())
		}
	}()

	log.Printf("Raft node started with ID: %s", config.LocalID)
	select {}
}
