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
	Raft — это алгоритм консенсуса, предназначенный для управления реплицированными журналами в
распределённых системах. Он был разработан как более понятная альтернатива Paxos и используется
для обеспечения согласованности данных между узлами.
  Raft решает три ключевые задачи:
 1. Выбор лидера (Leader Election): Один узел выбирается лидером, который управляет записями в журнале.
 2. Лог репликация (Log Replication): Лидер принимает команды и рассылает их другим узлам для синхронизации.
 3. Безопасность и отказоустойчивость (Safety and Fault Tolerance): Гарантирует, что данные остаются согласованными даже при сбоях.
    Raft обеспечивает strong consistency, что делает его удобным для распределённых систем, которым нужна предсказуемость и защита от разделения сети (network partitioning).

  Библиотека go-libp2p-consensus — это модуль для libp2p, который позволяет внедрять механизмы консенсуса (включая Raft)
в одноранговые (peer-to-peer) сети. Он предоставляет абстрактный интерфейс, который можно реализовать для
различных алгоритмов консенсуса, включая Raft, PoW, PoS и BFT-системы.Основные характеристики go-libp2p-consensus:
- Абстракция над алгоритмами консенсуса
- Позволяет использовать Raft или любой другой алгоритм (например, PoS).
- Интеграция с libp2p
- Поддерживает работу в децентрализованных системах без центрального координатора.
- Гибкость
- Разработчики могут реализовывать свою логику консенсуса, расширяя интерфейсы библиотеки.
- Работа в P2P-среде
- Отличается от классического Raft тем, что адаптирован под динамически изменяющиеся сети.
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
