# OTel Collector Kubernetes Events Evaluation

In this repository I want to evaluate if the `k8s_events` reciver paired with `k8s_leader_elector` works well and how `k8s_objects` compares against it.

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
- KSM
- cAdvisor
- VictoriaOperator
  - VictoriaMetrics Single
  - VictoriaMetrics Agent
- Grafana
- OTel Operator
- Event Generator

## Steps

Practial Evaluation:

- [ ] Research a realistic event creation ammount and warning ratio
  - =>
- [ ] Write Event Generator
- [ ] Deploy Prerequistis on Gardener
- [ ] Prepare Dashbaord for load test
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
