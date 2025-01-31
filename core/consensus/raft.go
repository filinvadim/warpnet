package consensus

import (
	"context"
	"github.com/filinvadim/warpnet/database"
	consensus "github.com/libp2p/go-libp2p-consensus"
	"github.com/libp2p/go-libp2p/core/host"
	"log"
	"os"
	"time"

	"github.com/hashicorp/raft"
	"github.com/libp2p/go-libp2p-raft"
)

/*
	Raft is a consensus algorithm designed for managing replicated logs in distributed systems.
	It was developed as a more understandable alternative to Paxos and is used to ensure data consistency across nodes.

  Raft solves three key tasks:
  1. **Leader Election**: One node is elected as the leader, responsible for managing log entries.
  2. **Log Replication**: The leader accepts commands and distributes them to other nodes for synchronization.
  3. **Safety and Fault Tolerance**: Ensures that data remains consistent even in the event of failures.

  Raft provides **strong consistency**, making it suitable for distributed systems that require predictability
  and protection against network partitioning.

  The **go-libp2p-consensus** library is a module for libp2p that enables the integration of consensus mechanisms
  (including Raft) into peer-to-peer (P2P) networks. It provides an abstract interface that can be implemented
  for various consensus algorithms, including Raft, PoW, PoS, and BFT-based systems.

  ### **Key Features of go-libp2p-consensus:**
  - **Consensus Algorithm Abstraction**
    - Supports Raft and other algorithms (e.g., PoS).
  - **Integration with libp2p**
    - Designed for decentralized systems without a central coordinator.
  - **Flexibility**
    - Developers can implement custom consensus logic by extending the library's interfaces.
  - **Optimized for P2P Environments**
    - Unlike traditional Raft, it is adapted for dynamically changing networks.
*/

type (
	Consensus = libp2praft.Consensus
	State     = consensus.State
)

// NewRaft TODO not used for now
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
	cons := libp2praft.NewConsensus(&state)
	// Получаем FSM через Consensus
	fsm := cons.FSM()

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
