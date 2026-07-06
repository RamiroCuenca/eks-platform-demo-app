# Running the load tests in-cluster

The k6 scripts in the parent directory run as Kubernetes Jobs in the
`loadtest` namespace — the service is deliberately cluster-internal (no
Ingress/LoadBalancer), so the load generator goes to the service instead of
the service coming to the internet. The namespace itself, its default-deny
floor and the demo-side ingress allow are owned by the
[gitops repo](https://github.com/RamiroCuenca/eks-platform-gitops); this
directory carries only what the generator itself needs.

These manifests are applied manually around a load-test session rather than
reconciled by ArgoCD — a load test that re-fires on every sync is not a
feature.

```sh
# 1. Scripts travel as a ConfigMap, from the same files k6 runs locally —
#    one source of truth, no manifest-embedded copies to drift.
kubectl -n loadtest create configmap k6-scripts \
  --from-file=loadtest/http.js --from-file=loadtest/enqueue.js

# 2. Egress allow (DNS-only namespace otherwise), then the profile you want.
kubectl apply -f loadtest/k8s/cnp-allow-k6-egress.yaml
kubectl apply -f loadtest/k8s/job-http.yaml      # HPA / Karpenter path
kubectl apply -f loadtest/k8s/job-enqueue.yaml   # KEDA queue path

# 3. Watch: k6 summary in the Job logs; scaling in the go-demo-slo dashboard.
kubectl -n loadtest logs -f job/k6-http

# Re-runs: Jobs are immutable — delete and re-apply.
kubectl -n loadtest delete job k6-http k6-enqueue --ignore-not-found
```

| Profile | Script | Exercises |
|---|---|---|
| `job-http.yaml` | `http.js` | `/db` + `/cache` → server CPU → HPA scale-out → Karpenter node provisioning |
| `job-enqueue.yaml` | `enqueue.js` | `/enqueue` → Redis list backlog → KEDA worker scale-out (from zero) |

Thresholds inside the scripts (`http_req_failed < 5%`, `p95 < 1.5s`) are the
same values the platform's Prometheus alerts fire on, so a failed load gate
and a firing alert corroborate each other.
