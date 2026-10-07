# OTel Collector Kubernetes Events Evaluation

In this repository I want to evaluate if the `k8s_events` reciver paired with `k8s_leader_elector` works well and how `k8s_objects` compares against it.

The repository includes all configuration files needed to run the otel collectors and monitor them.
It also includes an event-generate writting in Go. It generates "realistic" Kubernetes events at a controllable rate.

Why is this test of importants? Becuase `k8s_objects` is essentailly the generic and better implementation of `k8s_events`. Also `k8s_events` was standing close to beeing deprecated, and is still in `alpha` while `k8s_objects` is in `beta` and has a more stable api.
`k8s_events` has native features like a deduplication of events, while `k8s_objects` has better and more fine grained query support, for example instead of pulling all events and then filitering for only warning events, `k8s_objects` can direclty query the api server to only return warning events.
But most importantly `k8s_events` automatically parses the event into a readable and proper log. With `k8s_objects` this would have to happen via OpenTelemetry Transformation Language (OTTL) which may be slower and more resource intensive.

## Requirements for the adoption of `k8s_objects`

- [ ] A performance gain is seen from adapting the query for only WARNING events
- [ ] A major performance degration form manuall parsing is not observed
- [ ] No context, style or other information is lost in the end log compared to `k8s_events`
- [ ] The deduplication can be achived differently or is deemed not important

## Prerequisits

- Prometheus CRDs
- Cert-manager
- OTel Operator
- VictoriaOperator
  - VictoriaMetrics Single
  - VictoriaLogs Single
  - VictoriaMetrics Agent with cAdvisor Scrape
- Grafana
- Event Generator

## Steps

Practial Evaluation:

- [x] Research a realistic event creation ammount and warning ratio
  - => event creation is rather small, with 300 events/s on moderate clusters as far as I know. This means a load test is not really needed since 300-1000events/s shouldn't be an issue for otel collector. this shifts the performance test rather to an efficency test, and checks on the memory and cpu consumption.
- [x] Write Event Generator
- [x] Deploy Prerequistis on Gardener
- [x] Prepare Dashbaord for load test
- [ ] Run load tests
  - [ ] `k8s_events`
    - [ ] Include all events
    - [ ] Filter out Normal events
  - [ ] `k8s_objects`
    - [ ] All Events with proper parsing
    - [ ] Query api server for WARNING events only with proper parsing
- [ ] Export Result graphs

Formal Evaluation:

- [ ] Compare maturity of `k8s_events` and `k8s_objects` (Theses: `k8s_events` needs overhaul and proper SemConV)
- [ ] Compare log output quallity

## Quick Start

- **Namespace**

  ```bash
  kubectl create namespace monitoring
  ```

- **Prometheus CRDs**

  ```bash
    helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
    helm install my-prometheus-operator-crds prometheus-community/prometheus-operator-crds \
    --version 32.0.1 \
    --values ./k8s/prometheus-crds-values.yaml \
    --namespace monitoring
  ```

- **Cert-Manager**

  ```bash
  helm install \
    cert-manager oci://quay.io/jetstack/charts/cert-manager \
    --namespace cert-manager \
    --create-namespace \
    --version v1.21.2 \
    --values k8s/cert-manager-values.yaml
  ```

> [!WARNING]
> Cert Manager values are tuned for a Gardener Shoot Cluster. Double-check if those values apply to you.

- **OpenTelemetry Operator**

  ```bash
  helm repo add opentelemetry-helm https://open-telemetry.github.io/opentelemetry-helm-charts
  helm install my-opentelemetry-operator opentelemetry-helm/opentelemetry-operator \
    --version 0.123.1 \
    --values k8s/otel-operator-values.yaml \
    --namespace monitoring
  ```

- **VictoriaMetrics Operator**

  ```bash
  helm repo add victoriametrics https://victoriametrics.github.io/helm-charts/
  helm install my-victoria-metrics-operator victoriametrics/victoria-metrics-operator \
    --version 0.68.1 \
    --values k8s/vm-operator-values.yaml \
    --namespace monitoring
  ```

- **VictoriaMetrics Components**

  ```bash
  kubectl apply -f ./k8s/vm-single.yaml
  kubectl apply -f ./k8s/vm-agent.yaml # Also includes scrape config for cAdvisor
  kubectl apply -f ./k8s/vl-single.yaml
  ```

- **Grafana**

  ```bash
  helm install grafana oci://ghcr.io/grafana-community/helm-charts/grafana \
  --version 13.2.8 \
  --values ./k8s/grafana-values.yaml \
  --namespace monitoring
  ```

- **Access Grafana**
  Retrive Grafana `admin` user password:

  ```bash
  kubectl get secret --namespace monitoring grafana -o jsonpath="{.data.admin-password}" | base64 --decode ; echo
  ```

- **OpenTelemetry K8s Events Collector**

  ```bash
  kubectl apply -f ./k8s/otel-collector-events.yaml
  ```

- **OpenTelemetry K8s Objects Collector**

  ```bash
  kubectl apply -f ./k8s/otel-collector-objects.yaml
  ```
