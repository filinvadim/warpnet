package node

import (
	"context"
	"fmt"
	"github.com/filinvadim/warpnet/database"
	"github.com/libp2p/go-libp2p/core/host"
	"io"
	"log"
	"os"
	"time"

	"github.com/hashicorp/raft"
	"github.com/libp2p/go-libp2p-raft"
)

type ExampleFSM struct{}

func (f *ExampleFSM) Apply(log *raft.Log) interface{} {
	fmt.Printf("Applied log: %s\n", string(log.Data))
	return nil
}

func (f *ExampleFSM) Snapshot() (raft.FSMSnapshot, error) {
	return nil, nil
}

func (f *ExampleFSM) Restore(snapshot io.ReadCloser) error {
	return nil
}

func NewRaft(ctx context.Context, node host.Host, consRepo database.ConsensusRepo) {
	transport, err := libp2praft.NewLibp2pTransport(node, time.Second*30)
	if err != nil {
		log.Fatalf("Failed to create raft transport: %v", err)
	}

	// Создаём FSM (Finite State Machine)
	fsm := &ExampleFSM{}

	// Настраиваем хранилище Raft
	store := consRepo

	// Настраиваем снапшоты
	//snapshots, err := raft.NewFileSnapshotStore(consRepo.SnapshotPath(), 5, os.Stdout)
	//if err != nil {
	//	log.Fatalf("Failed to create snapshot store: %v", err)
	//}

	// Создаём конфигурацию Raft
	config := createRaftConfig(node.ID().String())

	// Создаём узел Raft
	raftNode, err := raft.NewRaft(config, fsm, store, store, store, transport)
	if err != nil {
		log.Fatalf("Failed to create raft node: %v", err)
	}

	// Добавляем себя в кластер
	raftNode.BootstrapCluster(raft.Configuration{
		Servers: []raft.Server{
			{
				ID:      config.LocalID,
				Address: transport.LocalAddr(),
			},
		},
	})

	log.Printf("Raft node started with ID: %s", config.LocalID)
	select {}
}

func createRaftConfig(nodeId string) *raft.Config {
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(nodeId)
	config.HeartbeatTimeout = 2 * time.Second
	config.ElectionTimeout = 2 * time.Second
	config.LeaderLeaseTimeout = 2 * time.Second
	return config
}
