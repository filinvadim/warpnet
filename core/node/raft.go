package node

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

func NewRaft(ctx context.Context, node host.Host, consRepo *database.ConsensusRepo) {
	transport, err := libp2praft.NewLibp2pTransport(node, time.Second*30)
	if err != nil {
		log.Fatalf("Failed to create raft transport: %v", err)
	}

	// Настраиваем хранилище Raft
	store := consRepo

	// Настраиваем снапшоты
	snapshotStore, err := raft.NewFileSnapshotStore(consRepo.SnapshotPath(), 5, os.Stdout)
	if err != nil {
		log.Fatalf("Failed to create snapshot store: %v", err)
	}

	// Создаём конфигурацию Raft
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(node.ID().String())
	config.HeartbeatTimeout = 2 * time.Second
	config.ElectionTimeout = 2 * time.Second
	config.LeaderLeaseTimeout = 2 * time.Second

	// Создаём узел Raft
	raftNode, err := raft.NewRaft(
		config,
		&libp2praft.FSM{},
		store, store,
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
