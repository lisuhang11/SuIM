#!/usr/bin/env bash
set -euo pipefail

# Apply the full SuIM workload. Review secret.yml and infra.yml first.
cd "$(dirname "$0")"

echo "1/6 namespace"
kubectl apply -f namespace.yml
echo "2/6 secret (update CHANGE_ME values before production use)"
kubectl apply -f secret.yml
echo "3/6 configmap"
kubectl apply -f configmap.yml
echo "4/6 external infrastructure endpoints"
kubectl apply -f infra.yml
echo "5/6 services"
kubectl apply -f apps/
echo "6/6 ingress"
kubectl apply -f ingress.yml

echo "waiting for rollout..."
kubectl -n suim rollout status deployment --timeout=300s
