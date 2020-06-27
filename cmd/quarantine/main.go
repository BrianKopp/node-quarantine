package main

import (
	"fmt"
	"time"

	"github.com/briankopp/node-quarantine/pkg/config"
	"github.com/briankopp/node-quarantine/pkg/node"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	// get settings
	config := config.NewFromEnv()

	// get in cluster config
	clusterConfig, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// create the client
	clientSet, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		panic(err.Error())
	}

	// set up the evaluator and node client, and kick them off
	evaluator := node.NewEvaluator(config)
	nodeClient := node.NewNodeClient(clientSet.CoreV1().Nodes(), config)
	go func() {
		runForever(evaluator, nodeClient, config)
	}()

	// TODO add liveness & readiness checks, and metrics endpoint for prometheus
}

func runForever(evaluator node.Evaluator, nodeClient node.Client, config config.Settings) {
	for {
		didRemove, err := runSingleEvaluation(evaluator, nodeClient, config)
		if err != nil {
			fmt.Printf("error running evaluation, %v", err)
			time.Sleep(config.DelayAfterError)
		} else if didRemove {
			time.Sleep(config.DelayAfterCordon)
		} else {
			time.Sleep(config.EvaluationPeriod)
		}
	}
}

func runSingleEvaluation(evaluator node.Evaluator, nodeClient node.Client, config config.Settings) (bool, error) {
	nodes, err := nodeClient.ListNodes()
	if err != nil {
		return false, err
	}
	utilizations := node.GetNodeUtilizations(nodes)
	evaluator.UpdateUnderUtilizedNodes(utilizations, time.Now())

	candidate := evaluator.GetCordonCandidate(time.Now())
	if candidate == nil {
		fmt.Print("no candidate for cordon")
		return false, nil
	}

	err = nodeClient.CordonNode(*candidate)
	if err != nil {
		return false, err
	}

	evaluator.DidCordonNode(*candidate)
	return true, nil
}
