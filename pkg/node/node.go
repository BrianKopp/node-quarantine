package node

import (
	"time"

	"github.com/briankopp/node-quarantine/pkg/config"
	v1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// Client handles all node operations
type Client interface {
	ListNodes() (*v1.NodeList, error)
	CordonNode(name string) error
}

// KubernetesClient calls the kubernetes API
type KubernetesClient struct {
	nodes  corev1.NodeInterface
	config config.Settings
}

func NewNodeClient(nodeApi corev1.NodeInterface, config config.Settings) Client {
	return &KubernetesClient{
		nodes:  nodeApi,
		config: config,
	}
}

// ListNodes lists kubernetes nodes, filtered for unready, unschedulable, or new nodes
func (m *KubernetesClient) ListNodes() (*v1.NodeList, error) {
	nodes, err := m.nodes.List(metaV1.ListOptions{LabelSelector: m.config.LabelSelector})
	if err != nil {
		return nil, err
	}
	return filterNodeList(nodes, time.Now()), nil
}

func (m *KubernetesClient) CordonNode(name string) error {
	return nil // TODO
}

// filterNodeList takes a list of nodes and removes those nodes that meet the following conditions
// * are not ready
// * are not scheduleable
// * were created within the last five minutes
func filterNodeList(nodes *v1.NodeList, now time.Time) *v1.NodeList {
	length := len(nodes.Items)
	for i := length - 1; i >= 0; i-- {
		node := nodes.Items[i]
		remove := false

		if node.Status.Phase != v1.NodeRunning {
			remove = true
		}

		if node.Spec.Unschedulable {
			remove = true
		}

		if node.CreationTimestamp.After(now.Add(-5 * time.Minute)) {
			remove = true
		}

		if remove {
			nodes.Items = append(nodes.Items[:i], nodes.Items[i+1:]...)
		}
	}
	return nodes
}
