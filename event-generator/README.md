# K8s Event Generator

Generates "realistic" Kubernetes events at a controllable rate to evaluate the [OpenTelemetry `k8s_events` receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8seventsreceiver) and `k8s_objects` receiver.

## Phases

1. **Generation**: creates events at the target rate for a fixed duration
2. **Cleanup**: deletes all generated events (labeled `app.kubernetes.io/managed-by=event-generator`)

## Flags

| Flag              | Default   | Description                                    |
| ----------------- | --------- | ---------------------------------------------- |
| `--target-rate`   | `10`      | Events per second                              |
| `--warning-ratio` | `0.2`     | Fraction of Warning vs Normal events (0.0–1.0) |
| `--duration`      | `10m`     | How long to run the generator                  |
| `--namespace`     | `default` | Namespace to create events in                  |
| `--workers`       | `5`       | Concurrent event-creation goroutines           |

## Local run (requires kubeconfig)

```bash
go run . --target-rate=5 --duration=1m
```

## Deploy as K8s Job

```bash
# Build & push image
docker build -t <your-registry>/event-generator:latest .
docker push <your-registry>/event-generator:latest

# Update image in k8s/job.yaml, then:
kubectl apply -f k8s/job.yaml

# Watch logs
kubectl logs -f job/event-generator

# Manual cleanup (if job was interrupted)
kubectl delete events -l app.kubernetes.io/managed-by=event-generator
```
