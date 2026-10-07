package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	labelKey   = "app.kubernetes.io/managed-by"
	labelValue = "event-generator"
)

var eventCounter atomic.Int64

// --- Configuration ---

type config struct {
	targetRate   float64
	warningRatio float64
	duration     time.Duration
	namespace    string
	workers      int
}

func parseFlags() config {
	var cfg config
	flag.Float64Var(&cfg.targetRate, "target-rate", 10, "Target events per second")
	flag.Float64Var(&cfg.warningRatio, "warning-ratio", 0.2, "Ratio of warning events (0.0-1.0)")
	flag.DurationVar(&cfg.duration, "duration", 10*time.Minute, "How long to run the generator")
	flag.StringVar(&cfg.namespace, "namespace", "default", "Namespace to create events in")
	flag.IntVar(&cfg.workers, "workers", 5, "Number of concurrent event-creation workers")
	flag.Parse()

	if cfg.warningRatio < 0 || cfg.warningRatio > 1 {
		fmt.Fprintln(os.Stderr, "error: --warning-ratio must be between 0.0 and 1.0")
		os.Exit(1)
	}
	if cfg.targetRate <= 0 {
		fmt.Fprintln(os.Stderr, "error: --target-rate must be positive")
		os.Exit(1)
	}
	return cfg
}

// --- Stats ---

type stats struct {
	created  atomic.Int64
	failed   atomic.Int64
	normal   atomic.Int64
	warnings atomic.Int64
	deleted  atomic.Int64
}

// --- Event templates ---

type eventTemplate struct {
	reason     string
	message    string
	component  string
	kind       string
	apiVersion string
}

var normalTemplates = []eventTemplate{
	{"Scheduled", "Successfully assigned {namespace}/{name} to {node}", "default-scheduler", "Pod", "v1"},
	{"Pulling", "Pulling image \"{image}\"", "kubelet", "Pod", "v1"},
	{"Pulled", "Successfully pulled image \"{image}\" in 1.234s", "kubelet", "Pod", "v1"},
	{"Created", "Created container app", "kubelet", "Pod", "v1"},
	{"Started", "Started container app", "kubelet", "Pod", "v1"},
	{"ScalingReplicaSet", "Scaled up replica set {name} to 3 replicas", "deployment-controller", "Deployment", "apps/v1"},
	{"SuccessfulCreate", "Created pod: {name}", "replicaset-controller", "ReplicaSet", "apps/v1"},
}

var warningTemplates = []eventTemplate{
	{"BackOff", "Back-off restarting failed container app in pod {name}", "kubelet", "Pod", "v1"},
	{"Failed", "Error: ImagePullBackOff for container app", "kubelet", "Pod", "v1"},
	{"Unhealthy", "Readiness probe failed: HTTP probe failed with statuscode: 503", "kubelet", "Pod", "v1"},
	{"FailedScheduling", "0/5 nodes are available: 3 Insufficient cpu, 2 Insufficient memory", "default-scheduler", "Pod", "v1"},
	{"OOMKilling", "Memory cgroup out of memory: Killed process 12345 (app)", "kernel-monitor", "Pod", "v1"},
}

// --- Fake cluster objects for realistic event references ---

var (
	podNames  []string
	nodeNames = []string{"node-0.internal", "node-1.internal", "node-2.internal", "node-3.internal", "node-4.internal"}
	deplNames []string
	rsNames   []string
	appNames  = []string{"web", "api", "worker", "cache", "db", "gateway", "auth", "search", "metrics", "queue"}
	images    = []string{
		"nginx:1.25", "redis:7.2", "postgres:16", "node:20-alpine",
		"python:3.12-slim", "golang:1.22", "ubuntu:22.04", "busybox:latest",
		"grafana/grafana:10.0", "prom/prometheus:v2.48",
	}
)

func init() {
	for _, app := range appNames {
		deplName := app + "-deployment"
		deplNames = append(deplNames, deplName)
		rsName := fmt.Sprintf("%s-%s", deplName, randString(10))
		rsNames = append(rsNames, rsName)
		for j := 0; j < 5; j++ {
			podNames = append(podNames, fmt.Sprintf("%s-%s", rsName, randString(5)))
		}
	}
}

func randString(n int) string {
	const chars = "bcdfghjklmnpqrstvwxz2456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[rand.IntN(len(chars))]
	}
	return string(b)
}

// --- Kubernetes client ---

func newClient() (kubernetes.Interface, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			home, _ := os.UserHomeDir()
			kubeconfig = home + "/.kube/config"
		}
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("cannot create config: %w", err)
		}
	}
	cfg.QPS = 200
	cfg.Burst = 300
	return kubernetes.NewForConfig(cfg)
}

// --- Main ---

func main() {
	cfg := parseFlags()

	client, err := newClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create k8s client: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var s stats
	start := time.Now()

	fmt.Println("=== K8s Event Generator ===")
	fmt.Printf("Namespace:      %s\n", cfg.namespace)
	fmt.Printf("Rate:           %.1f events/s\n", cfg.targetRate)
	fmt.Printf("Warning ratio:  %.0f%%\n", cfg.warningRatio*100)
	fmt.Printf("Duration:       %s\n", cfg.duration)
	fmt.Printf("Workers:        %d\n", cfg.workers)

	// Run generation
	fmt.Printf("\n--- Generating events (%s) ---\n", cfg.duration)
	runGeneration(ctx, client, cfg, &s)

	// Phase 3: Cleanup — remove all generated events (uses fresh context so SIGINT doesn't skip cleanup)
	fmt.Println("\n--- Cleanup ---")
	cleanup(context.Background(), client, cfg, &s)

	// Print brief stats
	elapsed := time.Since(start)
	fmt.Println("\n=== Stats ===")
	fmt.Printf("Duration:  %s\n", elapsed.Round(time.Second))
	fmt.Printf("Created:   %d (normal: %d, warning: %d)\n",
		s.created.Load(), s.normal.Load(), s.warnings.Load())
	fmt.Printf("Failed:    %d\n", s.failed.Load())
	fmt.Printf("Deleted:   %d\n", s.deleted.Load())
	if elapsed.Seconds() > 0 {
		fmt.Printf("Avg rate:  %.1f events/s\n", float64(s.created.Load())/elapsed.Seconds())
	}
}

// --- Event generation ---

func runGeneration(ctx context.Context, client kubernetes.Interface, cfg config, s *stats) {
	var wg sync.WaitGroup
	work := make(chan struct{}, cfg.workers*2)

	for i := 0; i < cfg.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range work {
				createEvent(ctx, client, cfg, s)
			}
		}()
	}

	phaseStart := time.Now()
	lastLog := phaseStart
	interval := time.Duration(float64(time.Second) / cfg.targetRate)

	for {
		elapsed := time.Since(phaseStart)
		if elapsed >= cfg.duration {
			break
		}

		select {
		case work <- struct{}{}:
		case <-ctx.Done():
			close(work)
			wg.Wait()
			return
		}

		if time.Since(lastLog) >= 10*time.Second {
			fmt.Printf("  [%s] rate=%.1f/s created=%d errors=%d\n",
				elapsed.Round(time.Second), cfg.targetRate, s.created.Load(), s.failed.Load())
			lastLog = time.Now()
		}

		time.Sleep(interval)
	}

	close(work)
	wg.Wait()
}

// --- Event creation ---

func createEvent(ctx context.Context, client kubernetes.Interface, cfg config, s *stats) {
	isWarning := rand.Float64() < cfg.warningRatio

	var tmpl eventTemplate
	if isWarning {
		tmpl = warningTemplates[rand.IntN(len(warningTemplates))]
	} else {
		tmpl = normalTemplates[rand.IntN(len(normalTemplates))]
	}

	objName, objNs := pickObject(tmpl.kind, cfg.namespace)
	now := metav1.Now()
	id := eventCounter.Add(1)

	msg := tmpl.message
	msg = strings.ReplaceAll(msg, "{name}", objName)
	msg = strings.ReplaceAll(msg, "{namespace}", cfg.namespace)
	msg = strings.ReplaceAll(msg, "{node}", nodeNames[rand.IntN(len(nodeNames))])
	msg = strings.ReplaceAll(msg, "{image}", images[rand.IntN(len(images))])

	evtType := corev1.EventTypeNormal
	if isWarning {
		evtType = corev1.EventTypeWarning
	}

	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("evtgen.%06d", id),
			Namespace: cfg.namespace,
			Labels: map[string]string{
				labelKey: labelValue,
			},
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:       tmpl.kind,
			Namespace:  objNs,
			Name:       objName,
			APIVersion: tmpl.apiVersion,
		},
		Reason:         tmpl.reason,
		Message:        msg,
		Type:           evtType,
		Count:          1,
		FirstTimestamp: now,
		LastTimestamp:  now,
		Source: corev1.EventSource{
			Component: tmpl.component,
			Host:      nodeNames[rand.IntN(len(nodeNames))],
		},
	}

	_, err := client.CoreV1().Events(cfg.namespace).Create(ctx, event, metav1.CreateOptions{})
	if err != nil {
		s.failed.Add(1)
		return
	}

	s.created.Add(1)
	if isWarning {
		s.warnings.Add(1)
	} else {
		s.normal.Add(1)
	}
}

func pickObject(kind, namespace string) (name, ns string) {
	switch kind {
	case "Pod":
		return podNames[rand.IntN(len(podNames))], namespace
	case "Node":
		return nodeNames[rand.IntN(len(nodeNames))], ""
	case "Deployment":
		return deplNames[rand.IntN(len(deplNames))], namespace
	case "ReplicaSet":
		return rsNames[rand.IntN(len(rsNames))], namespace
	case "Endpoints":
		return appNames[rand.IntN(len(appNames))], namespace
	default:
		return "unknown", namespace
	}
}

// --- Cleanup ---

func cleanup(ctx context.Context, client kubernetes.Interface, cfg config, s *stats) {
	selector := fmt.Sprintf("%s=%s", labelKey, labelValue)
	err := client.CoreV1().Events(cfg.namespace).DeleteCollection(ctx,
		metav1.DeleteOptions{},
		metav1.ListOptions{LabelSelector: selector},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cleanup failed: %v\n", err)
		return
	}
	s.deleted.Store(s.created.Load())
	fmt.Printf("  Deleted %d events\n", s.deleted.Load())
}
