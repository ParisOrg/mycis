# Cross-Framework Assessment — Revised Implementation Plan

## Context

This document revises the initial implementation plan for the cross-framework assessment feature. The original plan's core design decision — using `control_record` as the sync primitive — is sound: it avoids duplication, is non-breaking, and leverages an existing model. However, several gaps need to be addressed before implementation to avoid production issues.

This document captures what to change and why, so the implementation can proceed with full context.

---

## 1. Critical Issues to Fix Before MVP

### 1.1 The `CHECK (item_a_id < item_b_id)` constraint is fragile

**Issue:** Ordering random UUID v4 values lexicographically to deduplicate `(A,B)` and `(B,A)` works in Postgres but is semantically fragile. It breaks if the project later migrates to UUID v7 or ULID, and the constraint leaks an implementation detail (ordering) into the schema.

**Recommendation:**
- Remove the `CHECK (item_a_id < item_b_id)` constraint.
- Handle normalization at the application layer via an `upsert_mapping(a, b, relation)` function that swaps arguments before insert when the relation is symmetric.
- This also enables directional relations (see 1.2).

---

### 1.2 `subset_of` is directional — the current schema treats it as symmetric

**Issue:** This is the most serious latent bug in the plan. If ISO 27001 8.8 is `subset_of` NIST CSF ID.RA-01, then:
- Validating CSF ID.RA-01 does **not** validate ISO 8.8 (the superset does not imply the subset).
- Validating ISO 8.8 contributes to CSF ID.RA-01 but does not validate it alone (other items may be needed to cover the full superset).

Consequently, `subset_of` relations **cannot** share a `control_record`. They belong to a different category than `equivalent`.

**Recommendation:** Define three distinct relations with clear behaviour:

| Relation | Shares `control_record`? | Propagation | Semantic |
|---|---|---|---|
| `equivalent` | Yes | Bidirectional | Same requirement expressed differently |
| `subset_of` (A ⊂ B) | No | None. UI shows "contributes to B" | A alone is insufficient for B |
| `related` | No | None | Informational only, cross-links in UI |

**For the MVP:** only handle `equivalent`. Store `subset_of` and `related` in the database (official seeds contain them), but ignore them in grouping and propagation logic. Activate them in a later phase once the "contributes to" UI is designed.

Also: remove the ordering `CHECK` and make the pair ordering semantically meaningful for `subset_of` (`a ⊂ b`, not the inverse).

---

### 1.3 Union-find on `equivalent` creates unwanted large groups

**Issue:** Consider: ISO 27001 5.15 ≡ NIST CSF PR.AA-01, and NIST CSF PR.AA-01 ≡ CIS v8 6.1, and CIS v8 6.1 ≡ DORA Art. 9.4. By transitivity, all four end up in the same equivalence group. In practice, official crosswalks are never perfectly transitive — ISO 5.15 and DORA Art. 9.4 may cover slightly different aspects despite sharing a pivot.

**Result:** users validate one item and see propagation to items that are not truly equivalent in their interpretation. Trust in the feature erodes.

**Recommendation:** Two mitigations, ideally both:
1. Add a `confidence` or `strength` field on `control_mappings` (values: `strong`, `partial`). Only `strong` + `equivalent` enters union-find. Official seeds annotate this.
2. Limit transitive depth to 1 hop by default, configurable. Two items merge only if they have a **direct** mapping, not via a pivot.

**For the MVP:** use the 1-hop limit only. It is simpler, more predictable, and easier to explain to users ("these two items are marked equivalent by NIST").

---

### 1.4 Synchronous propagation in transaction will not scale

**Issue:** In `AssessmentItemService.Update()`, looping over siblings with per-sibling `UPDATE` + audit log produces:
- 5 updates + 5 audit logs per HTTP request for a 5-framework equivalence.
- 250+ writes in a single transaction for a bulk update of 50 items.
- Audit log pollution — the real event is "user validated ISO 5.15, which propagated to 4 siblings", not 5 independent events.

**Recommendation:**
- Single audit log per user action with a payload listing affected items: `action: "validated_with_propagation", primary_item_id: ..., propagated_to: [ids]`.
- Batch `UPDATE` in one SQL query: `UPDATE assessment_items SET status = $1, ... WHERE control_record_id = $2 AND id != $3`.
- Keep the transaction synchronous for the MVP (consistency > performance). Move to async only if latency issues appear.

---

### 1.5 Concurrent update handling is missing

**Issue:** Scenario: Alice validates ISO 5.15 at 10:00:00. Bob is marking NIST PR.AA-01 (sibling) as `blocked` at 10:00:01. The plan does not specify the outcome.

**Options:**
1. **Last write wins:** simple, but Alice never knows her action was overwritten.
2. **Optimistic locking** via `updated_at` or a `version` column: the second save fails, the UI prompts a reload.
3. **Divergence detection at propagation time:** if a sibling was modified by another user within X seconds, do not overwrite, log a warning, display a "divergent" badge in the UI.

**Recommendation for MVP:** option 2 (optimistic locking). It is standard Go/Postgres and prevents hours of debugging phantom data.

---

## 2. Structural Model Issues

### 2.1 `is_cross_framework` as a boolean creates technical debt

**Issue:** A boolean locks the model into a "simple vs cross" dichotomy. Future differentiation ("cross by user" vs "cross by official seed" vs "partial per-item opt-in cross") will require migration.

**Recommendation:**
- Do not store the flag. Derive it from `COUNT(*) FROM assessment_frameworks WHERE assessment_id = X > 1`. An assessment is cross-framework iff it has more than one framework. No drift possible.
- If a flag is needed for indexing, use a generated column or maintain it via trigger / application layer.

---

### 2.2 `framework_id` on `assessments` becomes ambiguous

**Issue:** Keeping `framework_id` as the "primary framework for display" is a trap. Any future query `WHERE a.framework_id = X` intended to filter assessments for framework X will silently miss cross-assessments that contain X but have Y as primary.

**Recommendation:** Two options:
1. Rename to `primary_framework_id` explicitly, add a SQL comment documenting the ambiguity.
2. **Better:** drop the column, add `is_primary BOOLEAN` on `assessment_frameworks`. Display logic picks the row with `is_primary = true`. One-time data migration cost; model consistency gain.

---

### 2.3 No invalidation strategy

**Issue:** The status matrix says `not_started` is not auto-propagated. But the plan does not address:
- What happens when a user modifies shared evidence on the `control_record` after validation?
- What happens when a `framework_item` is updated (seed update) during an active cycle?

Without handling this, "validate once" silently becomes "validate once, drift silently".

**Recommendation:**
- Add `evidence_version` or `last_evidence_update_at` on `control_record`.
- At validation time, snapshot this version on each sibling's `assessment_item`.
- If evidence changes after validation, flip affected siblings to `needs_review` or display an "evidence updated since validation" badge.

Not required for MVP, but track it as a known issue.

---

## 3. Phase 2 — Seeds: Practical Pitfalls

### 3.1 Official crosswalks are not in exploitable public formats

| Crosswalk | Source | Difficulty |
|---|---|---|
| NIST CSF ↔ ISO 27001 | NIST publishes an Informative Reference PDF/XLSX | Parseable, not trivial |
| ENISA NIS2 ↔ NIST CSF | Dense PDF with annexes | Hard |
| DORA | No public official crosswalk — requires EBA work or consulting output | Hard |
| CIS v8 ↔ ISO 27001 | CIS publishes a free mapping XLSX | Parseable |
| ISO 27001 ↔ ISO 27002 | ~1-to-1 | Trivially scriptable |

**Recommendation:** Plan a separate "official crosswalk ingestion" step, distinct from YAML seeding. Ideally, a script that takes the source XLSX and generates the YAML — versioned, reproducible. Otherwise the team maintains 6 YAML files by hand when CSF v3 ships.

### 3.2 YAML seed format should include provenance

```yaml
source: "iso27001-2022-x-nist-csf-2-0"
source_url: "https://csrc.nist.gov/..."
source_version: "2024-02"
retrieved_at: "2026-04-15"
mappings:
  - ...
```

Without this, no one in 18 months will know whether the mappings are current.

---

## 4. Phase 5 — UI: What's Missing

### 4.1 No "who propagated what" indicator

**Issue:** If I own NIST PR.AA-01 and it flips to `validated` without my action, I need to see why. Otherwise owners will panic or contest.

**Recommendation:** Minimal UI — a badge on the item row: *"Validated via ISO 27001 5.15 by Alice on 2026-04-19"*.

### 4.2 Matrix view needs filters from day one

**Issue:** For a cross-assessment of ISO + CSF + NIS2 + DORA + CIS over ~300 items, the matrix is ~300×5 = 1500 cells. Unusable above 50 items without filters.

**Recommendation:** v1 must ship with filters by status, owner, group/tag, and "mapped only" vs "not mapped".

### 4.3 Export

**Issue:** Auditors consume this data. It will be requested the day after shipping.

**Recommendation:** Ship CSV/XLSX export of the matrix in v1.

---

## 5. Revised Delivery Order

The original MVP (steps 1–4) bundles too much for a first ship. Splitting propagation from creation allows testing the data model in production with read-only cross-assessments before enabling sync.

| Step | Deliverable | Rationale |
|---|---|---|
| 1 | Schema migration (`control_mappings`, `assessment_frameworks`, drop `is_cross_framework`) | Foundation |
| 2 | `seed-mappings` CLI + one file (ISO 27001 ↔ ISO 27002, trivial) | Validates seed pipeline without depending on official crosswalks |
| 3 | `CreateCrossAssessment` service + form, **no propagation** | Can create a cross-assessment, see deduplicated items, no sync |
| 4 | `equivalent`-only propagation + optimistic locking + batched audit log | Core feature, shipped in isolation so it can be rolled back |
| 5 | Badges + framework filter chip | Visibility in detail view |
| 6 | ISO ↔ CSF seed (most demanded crosswalk) | First real crosswalk |
| 7 | Matrix view + CSV export | Executive overview |
| 8 | Remaining seeds (NIS2, DORA, CIS, GDPR) | Coverage expansion |
| 9 | Cycle compatibility | Full lifecycle support |
| 10 | `subset_of` and `related` in UI | Enhancement |

---

## 6. Open Questions — Decide Before Coding

1. **Who can create a manual mapping?** The schema has `source: 'seed' | 'manual'` but no permission model. Admin only? Per tenant? Reviewer role?
   - *Proposed MVP stance:* admin only.

2. **Are mappings global or per-tenant?** If a client wants to override an official mapping because they interpret ISO differently, what happens?
   - *Proposed MVP stance:* global mappings, no override, documented limitation.

3. **What happens to historical `assessment_items` when a new mapping is added?** If ISO 5.15 ↔ CSF PR.AA-01 is added today and a cross-assessment already exists, is it reconfigured retroactively?
   - *Proposed stance:* no retroactive changes, unless an explicit "re-sync mappings" action is triggered from the UI.

4. **DORA / NIS2 are not control frameworks** — they are regulations with obligations. Mapping them item-to-item against ISO is simplistic. Compliance-savvy users will contest equivalences.
   - *Recommendation:* add a UI disclaimer for regulatory mappings, or a distinct `regulatory_aligned` relation separate from `equivalent`.

---

*Last updated: 2026-04-19*
