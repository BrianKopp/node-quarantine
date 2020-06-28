package node

import (
	"encoding/json"
	"fmt"

	"time"

	"github.com/briankopp/node-quarantine/pkg/config"
	"github.com/rs/zerolog/log"
	v1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

// NewNodeClient creates a new node client from dependencies
func NewNodeClient(nodeAPI corev1.NodeInterface, config config.Settings) Client {
	return &KubernetesClient{
		nodes:  nodeAPI,
		config: config,
	}
}

type cordonPatch struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

// ListNodes lists kubernetes nodes, filtered for unready, unschedulable, or new nodes
func (m *KubernetesClient) ListNodes() (*v1.NodeList, error) {
	nodes, err := m.nodes.List(metaV1.ListOptions{LabelSelector: m.config.LabelSelector})
	if err != nil {
		log.Error().Err(err).Msg("error listing nodes")
		return nil, err
	}
	return filterNodeList(nodes, time.Now()), nil
}

// CordonNode cordons a node
func (m *KubernetesClient) CordonNode(name string) error {
	if m.config.DryRun {
		log.Info().Msg(fmt.Sprintf("DRY RUN - would have cordoned node %v", name))
		return nil
	}

	patches := []cordonPatch{
		{
			Op:    "replace",
			Path:  "/spec/unschedulable",
			Value: "true",
		},
	}
	payloadBytes, err := json.Marshal(patches)

	if err != nil {
		log.Error().Err(err).Msg("error marshalling json payload")
		return err
	}

	_, err = m.nodes.Patch(name, types.JSONPatchType, payloadBytes)
	if err != nil {
		log.Error().Err(err).Str("nodeName", name).Msg("error cordoning node")
		return err
	}

	log.Info().Str("nodeName", name).Msg("successfully cordoned node")
	return nil
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
			log.Info().Str("node", node.Name).Msg("node excluded since node phase not Running")
			remove = true
		}

		if node.Spec.Unschedulable {
			log.Info().Str("node", node.Name).Msg("node excluded since unschedulable")
			remove = true
		}

		if node.CreationTimestamp.After(now.Add(-5 * time.Minute)) {
			log.Info().Str("node", node.Name).Msg("node excluded since CreationTimestamp younger than 5m")
			remove = true
		}

		if remove {
			nodes.Items = append(nodes.Items[:i], nodes.Items[i+1:]...)
		}
	}
	return nodes
}
