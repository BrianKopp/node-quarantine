package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/briankopp/node-quarantine/pkg/config"
	"github.com/briankopp/node-quarantine/pkg/node"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	labelSelectorPtr := flag.String("node-labels", "", "node labels to select, e.g. group=quarantine")
	thresholdPtr := flag.Float64("threshold", 0.5, "utilization below which node will be marked as underutilized, fraction, e.g. 0.5")
	unusedAgePtr := flag.Int("unneeded-time", 600, "the amount of time (s) a node must be underutilized before it is considered to be cordoned")
	evaluationPtr := flag.Int("evaluation-period", 30, "the time (s) between evaluations")
	errorBackoffPtr := flag.Int("error-backoff", 300, "the time (s) between evaluations if an error occurs")
	cordonDelayPtr := flag.Int("cordon-backoff", 120, "the sleep time (s) after cordoning a node")
	debugPtr := flag.Bool("debug", false, "use debug logs")
	flag.Parse()

	config := config.Settings{
		LabelSelector:        *labelSelectorPtr,
		UtilizationThreshold: *thresholdPtr,
		UnusedAge:            time.Duration(*unusedAgePtr) * time.Second,
		EvaluationPeriod:     time.Duration(*evaluationPtr) * time.Second,
		DelayAfterError:      time.Duration(*errorBackoffPtr) * time.Second,
		DelayAfterCordon:     time.Duration(*cordonDelayPtr) * time.Second,
	}

	// set up logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	if *debugPtr {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
	log.Info().
		Str("nodeLabels", config.LabelSelector).
		Float64("utilizationThreshold", config.UtilizationThreshold).
		Msg("starting node-quarantine")

	// get in cluster config
	clusterConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Error().Err(err).Msg("error getting in cluster config")
		os.Exit(1)
	}

	// create the client
	clientSet, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		log.Error().Err(err).Msg("error getting kubernetes client set")
		os.Exit(1)
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
		log.Debug().Msg("begin evaluation loop")
		didCordon, err := runSingleEvaluation(evaluator, nodeClient, config)
		if err != nil {
			log.Error().Err(err).Msg("error running evaluation")
			time.Sleep(config.DelayAfterError)
		} else if didCordon {
			log.Info().Msg("did cordon node")
			time.Sleep(config.DelayAfterCordon)
		} else {
			log.Info().Msg("did not cordon node")
			time.Sleep(config.EvaluationPeriod)
		}
	}
}

func runSingleEvaluation(evaluator node.Evaluator, nodeClient node.Client, config config.Settings) (bool, error) {
	nodes, err := nodeClient.ListNodes()
	if err != nil {
		log.Error().Err(err).Msg("error listing nodes")
		return false, err
	}
	utilizations := node.GetNodeUtilizations(nodes)
	log.Info().Msg("acquired node utilizations")
	for _, util := range utilizations {
		log.Info().Msg(fmt.Sprintf("utilization for %v - %v", util.Name, util.MaxUtilization))
	}
	evaluator.UpdateUnderUtilizedNodes(utilizations, time.Now())

	candidate := evaluator.GetCordonCandidate(time.Now())
	if candidate == nil {
		return false, nil
	}

	err = nodeClient.CordonNode(*candidate)
	if err != nil {
		log.Error().Err(err).Msg("error cordoning nodes")
		return false, err
	}

	evaluator.DidCordonNode(*candidate)
	return true, nil
}
