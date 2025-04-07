package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	log "github.com/sirupsen/logrus"
	"strconv"
)

type metricsClient struct {
	address, nodeID string
	isBootstrapNode bool
}

func NewMetricsClient(
	address, nodeID string,
	isBootstrap bool,
) *metricsClient {
	return &metricsClient{address, nodeID, isBootstrap}
}

// PushStatusOnline - inform PushGateway about node status
func (m *metricsClient) PushStatusOnline() {
	if m.address == "" {
		return
	}
	if m.nodeID == "" {
		log.Errorf("failed to push status online, no node ID")
		return
	}
	onlineMetric := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "node_online_status",
		Help: "1 if node is online, 0 otherwise.",
	})

	onlineMetric.Set(1)

	err := push.New(m.address, "status").
		Collector(onlineMetric).
		Grouping("node_id", m.nodeID).
		Grouping("is_bootstrap", strconv.FormatBool(m.isBootstrapNode)).
		Push()
	if err != nil {
		log.Errorf("failed to push online metric: %v", err)
	}
}
