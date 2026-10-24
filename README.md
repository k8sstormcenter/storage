# Conformance evidence: kubescape-operator 1.41.0-duckling87

Chart under test: `kubescape-operator-1.41.0-duckling87` (node-agent `v0.1.0-rogue105`, storage `rc-rogue-rebase-e467e88f`, both pinned by digest). Workflow `release-conformance.yaml` on the node-agent fork, fresh kind cluster per test, chart installed from the public Helm index, Prometheus and Alertmanager installed, one job per test.

| run | lane | tests run | passed | failed | note |
|---|---|---|---|---|---|
| 40093532095 | kindnet | 48 | 48 | 0 | Test_15 job stopped before the cluster test, see below |
| 40093534027 | cilium | 48 | 48 | 0 | same Test_15 pre-check failure |
| 40095776365 | kindnet, Test_15 only | 1 | 0 | 1 | `Test_15_CompletedApBecomesReadyAgain` |

Layout per run: `run.json` (job list with conclusions and timestamps), `test-output/<test>.log` (the `go test -v` output of that job), `logs/<test>.log` (the node-agent log tail and the storage log tail captured after the test). Timestamps and the runner prefix were stripped from every line; nothing else was edited.

## Test_15

The workflow's default test list names `Test_15_CompletedApCannotBecomeReadyAgain`; the test binary shipped in the node-agent companion image names it `Test_15_CompletedApBecomesReadyAgain`. The "Check the requested test exists" step therefore failed in both lanes before any cluster work. The rerun under the binary's name failed inside the test:

```
component_test.go:802
expected: "ready"
actual  : "completed"
```

The test creates a ContainerProfile with status `completed`, patches the status back to a learning state through the API, and expects the new status to be stored. The storage returned `completed`. `pkg/registry/file/containerprofile_processor.go` (PreSave) resets an incoming `learning` status to `completed` when the stored consolidated profile is already `completed`; that guard looks the stored profile up under the plural resource prefix. Before the prefix migration the stored profile lived under the singular prefix, so the lookup never found it and the guard never fired; after the migration it does. The guard itself predates the rebase. The two test lines encode opposite expectations of this transition, which is a contract to settle, not a crash.

## Storage log summary

Across all 97 test jobs the storage log carried no error line and a single warning class, `skipping host cleanup: node discovery failed` (the storage service account has no `list nodes` right in the test cluster), 96 occurrences per lane. No prefix migration ran: every cluster was fresh. No `single-writer`, `acquisition` or write-gate lines appeared.
