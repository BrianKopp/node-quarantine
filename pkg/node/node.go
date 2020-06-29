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
	GetNodeUtilizations(nodes *v1.NodeList) ([]Utilization, error)
}

// kubernetesClient calls the kubernetes API
type kubernetesClient struct {
	nodes  corev1.NodeInterface
	pods   corev1.PodInterface
	config config.Settings
}

// NewNodeClient creates a new node client from dependencies
func NewNodeClient(nodeAPI corev1.NodeInterface, config config.Settings) Client {
	return &kubernetesClient{
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
func (m *kubernetesClient) ListNodes() (*v1.NodeList, error) {
	nodes, err := m.nodes.List(metaV1.ListOptions{LabelSelector: m.config.LabelSelector})
	if err != nil {
		log.Error().Err(err).Msg("error listing nodes")
		return nil, err
	}
	return filterNodeList(nodes, time.Now()), nil
}

// getPodsOnNode gets pods on node
func (m *kubernetesClient) getPodsOnNode(name string) (*v1.PodList, error) {
	pods, err := m.pods.List(metaV1.ListOptions{FieldSelector: fmt.Sprintf("spec.nodeName=%v", name)})
	if err != nil {
		return nil, err
	}
	return pods, nil
}

// CordonNode cordons a node
func (m *kubernetesClient) CordonNode(name string) error {
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

// GetNodeUtilizations calculates the node utilizations. Note this
// is slightly more conservative than cluster autoscaler, which excludes
// daemonsets and mirrored pods
func (m *kubernetesClient) GetNodeUtilizations(nodes *v1.NodeList) ([]Utilization, error) {
	nodeUtils := []Utilization{}

	for _, node := range nodes.Items {
		pods, err := m.getPodsOnNode(node.Name)
		if err != nil {
			log.Error().Err(err).Str("node", node.Name).Msg("error getting pods on node, couldnt calculate resource usage")
			continue
		}

		cpuTotal := float64(0)
		memTotal := float64(0)
		for _, pod := range pods.Items {
			for _, c := range pod.Spec.Containers {
				cpuRequests := c.Resources.Requests.Cpu()
				if cpuRequests != nil {
					cpuTotal += float64(cpuRequests.MilliValue())
				}

				memRequests := c.Resources.Requests.Memory()
				if memRequests != nil {
					memTotal += float64(memRequests.MilliValue())
				}
			}
		}

		util := float64(1)
		cpuAllocatable := node.Status.Allocatable.Cpu()
		if cpuAllocatable != nil {
			cpuAllocMilli := float64(cpuAllocatable.MilliValue())
			cpuUtil := cpuTotal / cpuAllocMilli
			if cpuUtil < util {
				util = cpuUtil
			}
		}
		memAllocatable := node.Status.Allocatable.Memory()
		if memAllocatable != nil {
			memAllocMilli := float64(memAllocatable.MilliValue())
			memUtil := memTotal / memAllocMilli
			if memUtil < util {
				util = memUtil
			}
		}

		nodeUtils = append(nodeUtils, Utilization{Name: node.Name, MaxUtilization: util})
	}

	return nodeUtils, nil
}

// filterNodeList takes a list of nodes and removes those nodes that meet the following conditions
// * are not scheduleable
// * were created within the last five minutes
func filterNodeList(nodes *v1.NodeList, now time.Time) *v1.NodeList {
	length := len(nodes.Items)
	for i := length - 1; i >= 0; i-- {
		node := nodes.Items[i]
		remove := false

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
