package test

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	klog "k8s.io/klog/v2"

	extProcPb "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"

	"inference.networking.x-k8s.io/llm-instance-gateway/pkg/ext-proc/backend"
	"inference.networking.x-k8s.io/llm-instance-gateway/pkg/ext-proc/handlers"
	"inference.networking.x-k8s.io/llm-instance-gateway/pkg/ext-proc/scheduling"
)

func StartExtProc(port int, refreshPodsInterval, refreshMetricsInterval time.Duration, pods []*backend.PodMetrics) *grpc.Server {
	ps := make(backend.PodSet)
	pms := make(map[backend.Pod]*backend.PodMetrics)
	for _, pod := range pods {
		ps[pod.Pod] = true
		pms[pod.Pod] = pod
	}
	pmc := &backend.FakePodMetricsClient{Res: pms}
	pp := backend.NewProvider(pmc, &backend.K8sDatastore{Pods: &sync.Map{}})
	if err := pp.Init(refreshPodsInterval, refreshMetricsInterval); err != nil {
		klog.Fatalf("failed to initialize: %v", err)
	}
	return startExtProc(port, pp)
}

// startExtProc starts an extProc server with fake pods.
func startExtProc(port int, pp *backend.Provider) *grpc.Server {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		klog.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()

	extProcPb.RegisterExternalProcessorServer(s, handlers.NewServer(pp, scheduling.NewScheduler(pp), "target-pod"))

	klog.Infof("Starting gRPC server on port :%v", port)
	reflection.Register(s)
	go s.Serve(lis)
	return s
}

func GenerateRequest(model string) *extProcPb.ProcessingRequest {
	j := map[string]interface{}{
		"model":       model,
		"prompt":      "hello",
		"max_tokens":  100,
		"temperature": 0,
	}

	llmReq, err := json.Marshal(j)
	if err != nil {
		klog.Fatal(err)
	}
	req := &extProcPb.ProcessingRequest{
		Request: &extProcPb.ProcessingRequest_RequestBody{
			RequestBody: &extProcPb.HttpBody{Body: llmReq},
		},
	}
	return req
}

func FakePod(index int) backend.Pod {
	address := fmt.Sprintf("address-%v", index)
	pod := backend.Pod{
		Name:    fmt.Sprintf("pod-%v", index),
		Address: address,
	}
	return pod
}
