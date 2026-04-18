package service

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"mycis/internal/db"
)

func TestAssessmentControlRecordsRemainIsolatedAcrossAssessmentsAndCycles(t *testing.T) {
	h := newIntegrationHarness(t)

	if err := h.services.Frameworks.SeedFramework(h.ctx, "cis-v8-1", false); err != nil {
		t.Fatal(err)
	}

	admin := h.createAdmin(t)
	owner := h.createUser(t, "Owner", "owner@example.com", false)
	reviewer := h.createUser(t, "Reviewer", "reviewer@example.com", false)
	framework := h.onlyFramework(t)

	startDate := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	dueDate := time.Date(2026, time.April, 30, 0, 0, 0, 0, time.UTC)

	assessmentOne, err := h.services.Assessments.CreateAssessment(h.ctx, admin, CreateAssessmentInput{
		FrameworkID: framework.ID,
		Name:        "Assessment One",
		Scope:       "Production",
		StartDate:   startDate,
		DueDate:     dueDate,
	})
	if err != nil {
		t.Fatal(err)
	}

	itemsOne, err := h.services.Assessments.ListAssessmentItems(h.ctx, assessmentOne.ID.String(), AssessmentItemFilters{})
	if err != nil {
		t.Fatal(err)
	}
	if len(itemsOne) == 0 {
		t.Fatal("expected seeded assessment items")
	}

	sourceItem := itemsOne[0]
	notes := "Documented as not applicable in the original assessment"
	if err := h.services.Items.Update(h.ctx, admin, UpdateItemInput{
		ID:             sourceItem.ID,
		OwnerUserID:    &owner.ID,
		ReviewerUserID: &reviewer.ID,
		Status:         db.AssessmentItemStatusNotApplicable,
		Priority:       sourceItem.Priority,
		DueDate:        sourceItem.DueDate,
		Notes:          &notes,
	}); err != nil {
		t.Fatal(err)
	}

	if err := h.services.Items.AddComment(h.ctx, owner, sourceItem.ID.String(), "Owner note"); err != nil {
		t.Fatal(err)
	}
	if err := h.services.Items.AddEvidenceLink(h.ctx, owner, sourceItem.ID.String(), "Evidence", "https://example.com/evidence"); err != nil {
		t.Fatal(err)
	}

	assessmentTwo, err := h.services.Assessments.CreateAssessment(h.ctx, admin, CreateAssessmentInput{
		FrameworkID: framework.ID,
		Name:        "Assessment Two",
		Scope:       "Production",
		StartDate:   startDate.AddDate(0, 1, 0),
		DueDate:     dueDate.AddDate(0, 1, 0),
	})
	if err != nil {
		t.Fatal(err)
	}

	itemsTwo, err := h.services.Assessments.ListAssessmentItems(h.ctx, assessmentTwo.ID.String(), AssessmentItemFilters{})
	if err != nil {
		t.Fatal(err)
	}
	secondItem := findAssessmentItem(t, itemsTwo, sourceItem.FrameworkItemID)
	if secondItem.OwnerUserID.Valid {
		t.Fatalf("expected owner to be reset, got %v", secondItem.OwnerUserID)
	}
	if secondItem.ReviewerUserID.Valid {
		t.Fatalf("expected reviewer to be reset, got %v", secondItem.ReviewerUserID)
	}
	if secondItem.Status != db.AssessmentItemStatusNotStarted {
		t.Fatalf("expected status reset to not_started, got %s", secondItem.Status)
	}
	if secondItem.Notes != "" {
		t.Fatalf("expected notes to reset, got %q", secondItem.Notes)
	}

	secondDetail, err := h.services.Items.GetDetail(h.ctx, secondItem.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(secondDetail.Comments) != 0 {
		t.Fatalf("expected no comments on new assessment, got %d", len(secondDetail.Comments))
	}
	if len(secondDetail.Evidence) != 0 {
		t.Fatalf("expected no evidence on new assessment, got %d", len(secondDetail.Evidence))
	}

	cycle, err := h.services.Assessments.CreateCycleFromPrevious(h.ctx, admin, CreateCycleInput{
		PreviousAssessmentID: assessmentOne.ID,
		Name:                 "Assessment One Cycle 2",
		Scope:                "Production",
		StartDate:            startDate.AddDate(0, 2, 0),
		DueDate:              dueDate.AddDate(0, 2, 0),
	})
	if err != nil {
		t.Fatal(err)
	}

	cycleItems, err := h.services.Assessments.ListAssessmentItems(h.ctx, cycle.ID.String(), AssessmentItemFilters{})
	if err != nil {
		t.Fatal(err)
	}
	cycleItem := findAssessmentItem(t, cycleItems, sourceItem.FrameworkItemID)
	if !cycleItem.OwnerUserID.Valid || cycleItem.OwnerUserID.Bytes != owner.ID {
		t.Fatalf("expected owner to carry forward, got %v", cycleItem.OwnerUserID)
	}
	if !cycleItem.ReviewerUserID.Valid || cycleItem.ReviewerUserID.Bytes != reviewer.ID {
		t.Fatalf("expected reviewer to carry forward, got %v", cycleItem.ReviewerUserID)
	}
	if cycleItem.Status != db.AssessmentItemStatusNotStarted {
		t.Fatalf("expected cycle status reset to not_started, got %s", cycleItem.Status)
	}
	if cycleItem.Notes != "" {
		t.Fatalf("expected cycle notes to reset, got %q", cycleItem.Notes)
	}

	cycleDetail, err := h.services.Items.GetDetail(h.ctx, cycleItem.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if cycleDetail.Item.IsNotApplicable {
		t.Fatal("expected cycle not_applicable flag to reset")
	}
	if len(cycleDetail.Comments) != 0 {
		t.Fatalf("expected no cycle comments, got %d", len(cycleDetail.Comments))
	}
	if len(cycleDetail.Evidence) != 0 {
		t.Fatalf("expected no cycle evidence, got %d", len(cycleDetail.Evidence))
	}

	sourceDetail, err := h.services.Items.GetDetail(h.ctx, sourceItem.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if !sourceDetail.Item.IsNotApplicable {
		t.Fatal("expected original assessment to remain not applicable")
	}
	if len(sourceDetail.Comments) != 1 {
		t.Fatalf("expected original assessment comments to remain, got %d", len(sourceDetail.Comments))
	}
	if len(sourceDetail.Evidence) != 1 {
		t.Fatalf("expected original assessment evidence to remain, got %d", len(sourceDetail.Evidence))
	}
}

func TestForceReseedUsesOnlyActiveFrameworkRowsForNewAssessmentsAndCycles(t *testing.T) {
	h := newIntegrationHarness(t)

	slug := fmt.Sprintf("force-reseed-%d", time.Now().UnixNano())
	h.writeFrameworkSeed(t, slug, testFrameworkSeedYAML(slug, "1.0.0", "1.1"))
	if err := h.services.Frameworks.SeedFramework(h.ctx, slug, false); err != nil {
		t.Fatal(err)
	}

	admin := h.createAdmin(t)
	framework := h.onlyFramework(t)

	startDate := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	dueDate := time.Date(2026, time.April, 30, 0, 0, 0, 0, time.UTC)

	originalAssessment, err := h.services.Assessments.CreateAssessment(h.ctx, admin, CreateAssessmentInput{
		FrameworkID: framework.ID,
		Name:        "Original Assessment",
		Scope:       "Production",
		StartDate:   startDate,
		DueDate:     dueDate,
	})
	if err != nil {
		t.Fatal(err)
	}

	originalItems, err := h.services.Assessments.ListAssessmentItems(h.ctx, originalAssessment.ID.String(), AssessmentItemFilters{})
	if err != nil {
		t.Fatal(err)
	}
	if codes := assessmentItemCodes(originalItems); len(codes) != 1 || codes[0] != "1.1" {
		t.Fatalf("expected original assessment to contain [1.1], got %v", codes)
	}

	h.writeFrameworkSeed(t, slug, testFrameworkSeedYAML(slug, "1.0.0", "1.2"))
	if err := h.services.Frameworks.SeedFramework(h.ctx, slug, true); err != nil {
		t.Fatal(err)
	}

	reseededAssessment, err := h.services.Assessments.CreateAssessment(h.ctx, admin, CreateAssessmentInput{
		FrameworkID: framework.ID,
		Name:        "Reseeded Assessment",
		Scope:       "Production",
		StartDate:   startDate.AddDate(0, 1, 0),
		DueDate:     dueDate.AddDate(0, 1, 0),
	})
	if err != nil {
		t.Fatal(err)
	}

	reseededItems, err := h.services.Assessments.ListAssessmentItems(h.ctx, reseededAssessment.ID.String(), AssessmentItemFilters{})
	if err != nil {
		t.Fatal(err)
	}
	if codes := assessmentItemCodes(reseededItems); len(codes) != 1 || codes[0] != "1.2" {
		t.Fatalf("expected new assessment to contain only active item [1.2], got %v", codes)
	}

	cycle, err := h.services.Assessments.CreateCycleFromPrevious(h.ctx, admin, CreateCycleInput{
		PreviousAssessmentID: originalAssessment.ID,
		Name:                 "Cycle After Reseed",
		Scope:                "Production",
		StartDate:            startDate.AddDate(0, 2, 0),
		DueDate:              dueDate.AddDate(0, 2, 0),
	})
	if err != nil {
		t.Fatal(err)
	}

	cycleItems, err := h.services.Assessments.ListAssessmentItems(h.ctx, cycle.ID.String(), AssessmentItemFilters{})
	if err != nil {
		t.Fatal(err)
	}
	if codes := assessmentItemCodes(cycleItems); len(codes) != 1 || codes[0] != "1.2" {
		t.Fatalf("expected cycle to contain only active item [1.2], got %v", codes)
	}

	originalItemsAfterReseed, err := h.services.Assessments.ListAssessmentItems(h.ctx, originalAssessment.ID.String(), AssessmentItemFilters{})
	if err != nil {
		t.Fatal(err)
	}
	if codes := assessmentItemCodes(originalItemsAfterReseed); len(codes) != 1 || codes[0] != "1.1" {
		t.Fatalf("expected historical assessment to keep [1.1], got %v", codes)
	}
}

func TestSeedFrameworkPreservesYamlOrderForNonNumericCodes(t *testing.T) {
	h := newIntegrationHarness(t)

	slug := fmt.Sprintf("ordered-%d", time.Now().UnixNano())
	h.writeFrameworkSeed(t, slug, testOrderedFrameworkSeedYAML(slug))
	if err := h.services.Frameworks.SeedFramework(h.ctx, slug, false); err != nil {
		t.Fatal(err)
	}

	admin := h.createAdmin(t)
	framework := h.onlyFramework(t)

	groups, err := h.services.Frameworks.ListGroupsByFramework(h.ctx, framework.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	groupCodes := make([]string, 0, len(groups))
	for _, group := range groups {
		groupCodes = append(groupCodes, group.Code)
	}
	if want := []string{"ZZ", "AA", "MM"}; !slices.Equal(groupCodes, want) {
		t.Fatalf("unexpected group order: got %v want %v", groupCodes, want)
	}

	assessment, err := h.services.Assessments.CreateAssessment(h.ctx, admin, CreateAssessmentInput{
		FrameworkID: framework.ID,
		Name:        "Ordered Assessment",
		Scope:       "Production",
		StartDate:   time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		DueDate:     time.Date(2026, time.April, 30, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}

	items, err := h.services.Assessments.ListAssessmentItems(h.ctx, assessment.ID.String(), AssessmentItemFilters{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := assessmentItemCodes(items), []string{"ZZ-two", "AA-one", "MM-three", "AA-nine"}; !slices.Equal(got, want) {
		t.Fatalf("unexpected assessment item order: got %v want %v", got, want)
	}
}

func TestNISTCSF20SeedUsesOnlyActiveCoreItems(t *testing.T) {
	h := newIntegrationHarness(t)

	if err := h.services.Frameworks.SeedFramework(h.ctx, "nist-csf-2-0", false); err != nil {
		t.Fatal(err)
	}

	admin := h.createAdmin(t)
	framework := h.onlyFramework(t)

	assessment, err := h.services.Assessments.CreateAssessment(h.ctx, admin, CreateAssessmentInput{
		FrameworkID: framework.ID,
		Name:        "NIST Assessment",
		Scope:       "Production",
		StartDate:   time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		DueDate:     time.Date(2026, time.April, 30, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}

	items, err := h.services.Assessments.ListAssessmentItems(h.ctx, assessment.ID.String(), AssessmentItemFilters{})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(items); got != 106 {
		t.Fatalf("expected 106 active NIST CSF 2.0 items, got %d", got)
	}

	codes := assessmentItemCodes(items)
	for _, withdrawn := range []string{
		"PR.DS-03",
		"PR.DS-04",
		"PR.DS-05",
		"PR.DS-12",
		"DE.CM-04",
		"DE.CM-05",
		"DE.CM-07",
		"DE.CM-08",
		"DE.AE-01",
		"DE.AE-05",
		"RS.AN-01",
		"RS.AN-02",
		"RS.AN-04",
		"RS.AN-05",
		"RS.CO-01",
		"RC.CO-01",
		"RC.CO-02",
	} {
		if slices.Contains(codes, withdrawn) {
			t.Fatalf("expected withdrawn item %s to be absent from new assessments", withdrawn)
		}
	}

	for _, active := range []string{"GV.OC-01", "PR.IR-04", "RS.CO-02", "RC.CO-04"} {
		if !slices.Contains(codes, active) {
			t.Fatalf("expected active item %s to be present", active)
		}
	}
}

func TestRepairFrameworkItemReferencesRebindsOnlySafeAssessments(t *testing.T) {
	h := newIntegrationHarness(t)

	admin := h.createAdmin(t)

	framework, err := h.queries.CreateFramework(h.ctx, db.CreateFrameworkParams{
		Slug:    "repair-framework",
		Label:   "Repair Framework",
		Version: "1.0.0",
		Status:  "active",
	})
	if err != nil {
		t.Fatal(err)
	}

	group, err := h.queries.CreateFrameworkGroup(h.ctx, db.CreateFrameworkGroupParams{
		FrameworkID: framework.ID,
		Code:        "GRP",
		Title:       "Group",
		Summary:     "Group summary",
		Description: "Group description",
		SortOrder:   1,
	})
	if err != nil {
		t.Fatal(err)
	}

	legacyItem, err := h.queries.CreateFrameworkItem(h.ctx, db.CreateFrameworkItemParams{
		FrameworkID:      framework.ID,
		FrameworkGroupID: group.ID,
		Code:             "LEGACY-01",
		Title:            "Legacy item",
		Summary:          "Legacy item",
		Description:      "Legacy item",
		SortOrder:        1,
		AssetClass:       "Systems",
		SecurityFunction: "RESPOND",
		Tags:             []string{"legacy"},
	})
	if err != nil {
		t.Fatal(err)
	}

	safeAssessment, err := h.services.Assessments.CreateAssessment(h.ctx, admin, CreateAssessmentInput{
		FrameworkID: framework.ID,
		Name:        "Safe Assessment",
		Scope:       "Production",
		StartDate:   time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		DueDate:     time.Date(2026, time.April, 30, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}

	canonicalItem, err := h.queries.CreateFrameworkItem(h.ctx, db.CreateFrameworkItemParams{
		FrameworkID:      framework.ID,
		FrameworkGroupID: group.ID,
		Code:             "CANONICAL-01",
		Title:            "Canonical item",
		Summary:          "Canonical item",
		Description:      "Canonical item",
		SortOrder:        2,
		AssetClass:       "Systems",
		SecurityFunction: "RESPOND",
		Tags:             []string{"canonical"},
	})
	if err != nil {
		t.Fatal(err)
	}

	conflictingAssessment, err := h.services.Assessments.CreateAssessment(h.ctx, admin, CreateAssessmentInput{
		FrameworkID: framework.ID,
		Name:        "Conflicting Assessment",
		Scope:       "Production",
		StartDate:   time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC),
		DueDate:     time.Date(2026, time.May, 31, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := repairFrameworkItemReferences(h.ctx, h.queries, framework.ID, []frameworkItemRepairRule{
		{
			LegacyCode:    "LEGACY-01",
			CanonicalCode: "CANONICAL-01",
		},
	}); err != nil {
		t.Fatal(err)
	}

	safeItems, err := h.services.Assessments.ListAssessmentItems(h.ctx, safeAssessment.ID.String(), AssessmentItemFilters{})
	if err != nil {
		t.Fatal(err)
	}
	if got := assessmentItemCodes(safeItems); len(got) != 1 || got[0] != "CANONICAL-01" {
		t.Fatalf("expected safe assessment to rebind to CANONICAL-01, got %v", got)
	}
	safeControlRecord, err := h.queries.GetControlRecordByAssessmentItem(h.ctx, safeItems[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if safeControlRecord.FrameworkItemID != canonicalItem.ID {
		t.Fatalf("expected safe control record to point at canonical item, got %s want %s", safeControlRecord.FrameworkItemID, canonicalItem.ID)
	}

	conflictingItems, err := h.services.Assessments.ListAssessmentItems(h.ctx, conflictingAssessment.ID.String(), AssessmentItemFilters{})
	if err != nil {
		t.Fatal(err)
	}
	if got := assessmentItemCodes(conflictingItems); !slices.Equal(got, []string{"LEGACY-01", "CANONICAL-01"}) {
		t.Fatalf("expected conflicting assessment to keep both items, got %v", got)
	}
	conflictingLegacyItem := findAssessmentItemByCode(t, conflictingItems, "LEGACY-01")
	conflictingControlRecord, err := h.queries.GetControlRecordByAssessmentItem(h.ctx, conflictingLegacyItem.ID)
	if err != nil {
		t.Fatal(err)
	}
	if conflictingControlRecord.FrameworkItemID != legacyItem.ID {
		t.Fatalf("expected conflicting legacy control record to stay on legacy item, got %s want %s", conflictingControlRecord.FrameworkItemID, legacyItem.ID)
	}
}

func TestUpdateClearsNotesAndNotApplicableFlag(t *testing.T) {
	h := newIntegrationHarness(t)

	if err := h.services.Frameworks.SeedFramework(h.ctx, "cis-v8-1", false); err != nil {
		t.Fatal(err)
	}

	admin := h.createAdmin(t)
	framework := h.onlyFramework(t)

	assessment, err := h.services.Assessments.CreateAssessment(h.ctx, admin, CreateAssessmentInput{
		FrameworkID: framework.ID,
		Name:        "Assessment",
		Scope:       "Production",
		StartDate:   time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		DueDate:     time.Date(2026, time.April, 30, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}

	items, err := h.services.Assessments.ListAssessmentItems(h.ctx, assessment.ID.String(), AssessmentItemFilters{})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 {
		t.Fatal("expected seeded assessment items")
	}

	item := items[0]
	notes := "Legacy note"
	if err := h.services.Items.Update(h.ctx, admin, UpdateItemInput{
		ID:       item.ID,
		Status:   db.AssessmentItemStatusNotApplicable,
		Priority: item.Priority,
		DueDate:  item.DueDate,
		Notes:    &notes,
	}); err != nil {
		t.Fatal(err)
	}

	if err := h.services.Items.Update(h.ctx, admin, UpdateItemInput{
		ID:       item.ID,
		Status:   db.AssessmentItemStatusInProgress,
		Priority: item.Priority,
		DueDate:  item.DueDate,
	}); err != nil {
		t.Fatal(err)
	}

	detail, err := h.services.Items.GetDetail(h.ctx, item.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if detail.Item.IsNotApplicable {
		t.Fatal("expected not-applicable flag to be cleared")
	}
	if detail.Item.Notes != "" {
		t.Fatalf("expected notes to be cleared, got %q", detail.Item.Notes)
	}
}
