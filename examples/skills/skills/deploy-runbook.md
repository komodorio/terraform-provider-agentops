---
description: How to safely roll out a service
---

# Deploy runbook

Roll out a service change in stages, verifying health at each step before
widening the blast radius.

## Steps

1. Confirm the change is merged and the image is built.
2. Deploy to the canary and watch error rate + latency for 10 minutes.
3. If healthy, roll out to 25%, then 50%, then 100%, pausing at each step.
4. If any step regresses, roll back to the previous version immediately.

## Rollback

Redeploy the previously running version and confirm the error rate returns to
baseline before investigating the failed change.
