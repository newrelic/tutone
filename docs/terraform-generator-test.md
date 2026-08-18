# Tutone Terraform Generator — Test Plan

**Branch:** `tutone-changes-terraform` ([github.com/newrelic/tutone](https://github.com/newrelic/tutone/tree/tutone-changes-terraform))  
**Epic:** [NR-576792](https://new-relic.atlassian.net/browse/NR-576792)  
**Author:** Bhoomika R S

---

## What this tests

The `tutone-changes-terraform` branch adds a `terraform` generator to tutone that produces production-grade Terraform provider files directly from the NerdGraph GraphQL schema.

Running `tutone generate` from the `terraform-provider-newrelic` root generates **5 files per resource**:

| File | Action |
|---|---|
| `newrelic/resource_newrelic_<name>.go` | Full CRUD scaffold — CreateContext, ReadContext, UpdateContext, DeleteContext |
| `newrelic/structures_newrelic_<name>.go` | expand/flatten functions with real field mappings |
| `newrelic/resource_newrelic_<name>_test.go` | Acceptance test scaffold (created once, never overwritten) |
| `newrelic/provider_registration.txt` | Copy-paste line for `provider_newrelic.go` ResourcesMap |
| `newrelic/provider_newrelic.go` | Auto-patched — new ResourcesMap entry inserted alphabetically |

---

## Prerequisites

- Go 1.20+
- Access to `terraform-provider-newrelic` repo locally
- New Relic API key with Customer schema access
- Local clone of `tutone` on branch `tutone-changes-terraform`

```bash
# Clone tutone on the feature branch
git clone https://github.com/newrelic/tutone.git
cd tutone
git checkout tutone-changes-terraform
cd ..
```

---

## Setup

### 1. Copy the test config into the terraform provider

```bash
cp /path/to/tutone/configs/tutone-terraform-test.yml \
   /path/to/terraform-provider-newrelic/.tutone.yml
```

### 2. Navigate to the terraform provider root

```bash
cd /path/to/terraform-provider-newrelic
```

> **Important:** all `tutone` commands must be run from the terraform provider root so that `path: newrelic/` resolves to `terraform-provider-newrelic/newrelic/`.

---

## Test 1 — Standard CRUD resource (`alert_muting_rule`)

Tests the baseline generation path: existing NerdGraph mutation, int ID, direct read.

### Run

```bash
export NEW_RELIC_API_KEY=<your-key>

# Fetch schema (only needed once)
go run /path/to/tutone/cmd/tutone/main.go fetch -s schema.json -c .tutone.yml

# Generate
go run /path/to/tutone/cmd/tutone/main.go generate --config .tutone.yml --package alertmutingrule
```

### Expected output

```
newrelic/resource_newrelic_alert_muting_rule.go      ← overwritten
newrelic/structures_newrelic_alert_muting_rule.go    ← overwritten
newrelic/provider_registration.txt                   ← created
```

> `resource_newrelic_alert_muting_rule_test.go` is **skipped** — already exists (scaffold-once behaviour).  
> `provider_newrelic.go` patch is **skipped** — entry already exists.

### Verify

```bash
# 1. Check TUTONE:STATUS banner
head -6 newrelic/resource_newrelic_alert_muting_rule.go

# 2. Confirm snake_case attribute names (not camelCase)
grep '"action_on_muting_rule_window_ended"' newrelic/resource_newrelic_alert_muting_rule.go

# 3. Confirm correct function name casing
grep 'func resourceNewRelicAlertMutingRule()' newrelic/resource_newrelic_alert_muting_rule.go

# 4. Confirm client accessor is correct
grep 'client.Alerts.' newrelic/resource_newrelic_alert_muting_rule.go

# 5. No unused strconv import (id_type: int → strconv used for serializeIDs)
head -20 newrelic/resource_newrelic_alert_muting_rule.go | grep strconv
```

### Pass criteria

- [ ] `TUTONE:STATUS` banner present in first 6 lines
- [ ] Attribute names are `snake_case` (e.g. `action_on_muting_rule_window_ended`)
- [ ] Function name is `resourceNewRelicAlertMutingRule` (capital R in Relic)
- [ ] Client calls use `client.Alerts.CreateMutingRuleWithContext`
- [ ] `strconv` imported (id_type is int)
- [ ] No `ValidateFunc` on `TypeList` fields

---

## Test 2 — New import: pathpoint (100% automation test)

Tests all 4 automation gaps:
- **Gap 1:** multi-arg create (`pathpoint` + `scope` inputs)
- **Gap 2:** pointer fields (`*PathPointKpiTimeWindowInput`)
- **Gap 3:** custom scalar mapping (`EpochMilliseconds` → `schema.TypeInt`)
- **Gap 4:** non-standard CRUD args (delete by GUID only, update without accountID)

> Requires `newrelic-client-go` pathpoint package. If [PR #1443](https://github.com/newrelic/newrelic-client-go/pull/1443) is not yet merged:
> ```bash
> go get github.com/newrelic/newrelic-client-go/v2@main
> ```

### Run

```bash
go run /path/to/tutone/cmd/tutone/main.go generate --config .tutone.yml --package pathpoint
```

### Expected output

```
newrelic/resource_newrelic_pathpoint_flow.go         ← created
newrelic/structures_newrelic_pathpoint_flow.go       ← created
newrelic/resource_newrelic_pathpoint_flow_test.go    ← created (scaffold)
newrelic/provider_registration.txt                   ← appended
newrelic/provider_newrelic.go                        ← patched
```

### Verify

```bash
# 1. TUTONE:STATUS must be FULLY AUTOMATED
head -6 newrelic/resource_newrelic_pathpoint_flow.go
head -6 newrelic/structures_newrelic_pathpoint_flow.go

# 2. Zero TUTONE:MANUAL markers
grep -c 'TUTONE:MANUAL' newrelic/resource_newrelic_pathpoint_flow.go
grep -c 'TUTONE:MANUAL' newrelic/structures_newrelic_pathpoint_flow.go

# 3. Gap 1 — two separate expand calls in Create
grep -A5 'PathPointCreateWithContext' newrelic/resource_newrelic_pathpoint_flow.go

# 4. Gap 2 — pointer nil guard in flatten
grep 'result\..*!= nil' newrelic/structures_newrelic_pathpoint_flow.go

# 5. Gap 3 — EpochMilliseconds uses schema.TypeInt (not TypeString)
grep -A3 'epoch_milliseconds\|EpochMilliseconds' newrelic/resource_newrelic_pathpoint_flow.go

# 6. Gap 4 — Delete passes only id (no accountID)
grep -A3 'PathPointDeleteWithContext' newrelic/resource_newrelic_pathpoint_flow.go

# 7. provider_newrelic.go patched
grep 'pathpoint_flow' newrelic/provider_newrelic.go
```

### Pass criteria

- [ ] `TUTONE:STATUS ✅ FULLY AUTOMATED` in both files
- [ ] `grep -c 'TUTONE:MANUAL'` returns `0` for both files
- [ ] Create calls `PathPointCreateWithContext(ctx, accountID, pathpointInput, scopeInput)`
- [ ] EpochMilliseconds field has `Type: schema.TypeInt`
- [ ] Pointer fields have `result.Field != nil` nil guard in flatten
- [ ] Delete call is `PathPointDeleteWithContext(ctx, id)` — no accountID
- [ ] `"newrelic_pathpoint_flow": resourceNewRelicPathpointFlow()` in `provider_newrelic.go`

---

## Test 3 — Build check

```bash
go build ./newrelic/... 2>&1
```

### Pass criteria

- [ ] Zero compile errors on the generated pathpoint files
- [ ] `strconv imported but not used` does NOT appear
- [ ] No `ValidateFunc` type mismatch errors

---

## Test 4 — Regression: existing tests still pass

```bash
go test ./newrelic/... -run TestUnit -count=1 2>&1 | tail -5
```

### Pass criteria

- [ ] All existing unit tests pass
- [ ] No new test failures introduced by generated files

---

## What to report back

Please share results for each test using this table:

| Test | Pass | Notes |
|---|---|---|
| Test 1 — alertmutingrule generates | ☐ | |
| Test 1 — snake_case attribute names | ☐ | |
| Test 1 — correct function casing | ☐ | |
| Test 2 — pathpoint generates (0 MANUAL markers) | ☐ | |
| Test 2 — Gap 1 multi-arg create | ☐ | |
| Test 2 — Gap 2 pointer nil guard | ☐ | |
| Test 2 — Gap 3 EpochMilliseconds TypeInt | ☐ | |
| Test 2 — Gap 4 delete without accountID | ☐ | |
| Test 3 — build passes | ☐ | |
| Test 4 — existing tests pass | ☐ | |

Any compile errors or unexpected output — please attach the relevant file content.

---

## Config reference

Full test config: [`configs/tutone-terraform-test.yml`](../configs/tutone-terraform-test.yml)  
Template dir: [`templates/terraform/`](../templates/terraform/)
