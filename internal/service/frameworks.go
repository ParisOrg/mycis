package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mycis/internal/db"
	"mycis/internal/seed"
)

type FrameworkService struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

// Repair maps are limited to safe one-to-one rebinding. Legacy many-to-one
// consolidation needs explicit merge semantics and is intentionally unsupported.
var frameworkItemRepairMaps = map[string]map[string]string{
	"nist-csf-2-0": {
		"PR.DS-04": "PR.IR-04",
	},
}

func (s *FrameworkService) ListFrameworks(ctx context.Context) ([]db.ListFrameworksWithCountsRow, error) {
	return s.queries.ListFrameworksWithCounts(ctx)
}

func (s *FrameworkService) ListGroupsByFramework(ctx context.Context, frameworkID string) ([]db.ListFrameworkGroupsByFrameworkRow, error) {
	id, err := uuidFromString(frameworkID)
	if err != nil {
		return nil, err
	}
	return s.queries.ListFrameworkGroupsByFramework(ctx, id)
}

func (s *FrameworkService) SeedFramework(ctx context.Context, slug string, force bool) error {
	path := filepath.Join("seed", "frameworks", slug+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read seed file: %w", err)
	}

	doc, err := seed.LoadDocument(data)
	if err != nil {
		return err
	}

	_, err = withTx(ctx, s.pool, func(q *db.Queries) (struct{}, error) {
		existing, err := q.GetFrameworkBySlugVersion(ctx, db.GetFrameworkBySlugVersionParams{
			Slug:    doc.Framework.Slug,
			Version: doc.Framework.Version,
		})
		if err == nil {
			if !force {
				return struct{}{}, fmt.Errorf("%w: framework %s %s already exists", ErrConflict, doc.Framework.Slug, doc.Framework.Version)
			}
			// Force-reseed: keep historical framework rows for old assessments, but deactivate
			// anything removed from the current document so new assessments and cycles only
			// use the active framework definition.
			groupMap, err := upsertFrameworkGroups(ctx, q, existing.ID, doc.Groups)
			if err != nil {
				return struct{}{}, err
			}
			if err := upsertFrameworkItems(ctx, q, existing.ID, groupMap, doc.Items); err != nil {
				return struct{}{}, err
			}
			if repairMap, ok := frameworkItemRepairMaps[doc.Framework.Slug]; ok {
				if err := repairFrameworkItemReferences(ctx, q, existing.ID, repairMap); err != nil {
					return struct{}{}, err
				}
			}
			if err := q.DeactivateMissingFrameworkItems(ctx, db.DeactivateMissingFrameworkItemsParams{
				FrameworkID: existing.ID,
				Codes:       frameworkItemCodes(doc.Items),
			}); err != nil {
				return struct{}{}, fmt.Errorf("deactivate framework items: %w", err)
			}
			if err := q.DeactivateMissingFrameworkGroups(ctx, db.DeactivateMissingFrameworkGroupsParams{
				FrameworkID: existing.ID,
				Codes:       frameworkGroupCodes(doc.Groups),
			}); err != nil {
				return struct{}{}, fmt.Errorf("deactivate framework groups: %w", err)
			}
			return struct{}{}, nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return struct{}{}, fmt.Errorf("check framework: %w", err)
		}

		// Fresh insert — no existing framework record.
		framework, err := q.CreateFramework(ctx, db.CreateFrameworkParams{
			Slug:    doc.Framework.Slug,
			Label:   doc.Framework.Label,
			Version: doc.Framework.Version,
			Status:  "active",
		})
		if err != nil {
			return struct{}{}, fmt.Errorf("create framework: %w", err)
		}

		groupMap, err := createFrameworkGroups(ctx, q, framework.ID, doc.Groups)
		if err != nil {
			return struct{}{}, err
		}
		if err := createFrameworkItems(ctx, q, framework.ID, groupMap, doc.Items); err != nil {
			return struct{}{}, err
		}

		return struct{}{}, nil
	})

	return err
}

func upsertFrameworkGroups(ctx context.Context, q *db.Queries, frameworkID uuid.UUID, groups []seed.Group) (map[string]uuid.UUID, error) {
	groupMap := make(map[string]uuid.UUID, len(groups))
	for i, group := range groups {
		updated, err := q.UpsertFrameworkGroup(ctx, db.UpsertFrameworkGroupParams{
			FrameworkID: frameworkID,
			Code:        group.Code,
			Title:       group.Title,
			Summary:     group.Summary,
			Description: group.Description,
			SortOrder:   int32(i + 1),
		})
		if err != nil {
			return nil, fmt.Errorf("upsert framework group %s: %w", group.Code, err)
		}
		groupMap[group.Code] = updated.ID
	}
	return groupMap, nil
}

func upsertFrameworkItems(ctx context.Context, q *db.Queries, frameworkID uuid.UUID, groupMap map[string]uuid.UUID, items []seed.Item) error {
	for i, item := range items {
		groupID, ok := groupMap[item.GroupCode]
		if !ok {
			return fmt.Errorf("%w: missing framework group %s", ErrInvalidInput, item.GroupCode)
		}
		if _, err := q.UpsertFrameworkItem(ctx, db.UpsertFrameworkItemParams{
			FrameworkID:      frameworkID,
			FrameworkGroupID: groupID,
			Code:             item.Code,
			Title:            item.Title,
			Summary:          item.Summary,
			Description:      item.Description,
			SortOrder:        int32(i + 1),
			AssetClass:       item.AssetClass,
			SecurityFunction: item.SecurityFunction,
			Tags:             item.Tags,
		}); err != nil {
			return fmt.Errorf("upsert framework item %s: %w", item.Code, err)
		}
	}
	return nil
}

func createFrameworkGroups(ctx context.Context, q *db.Queries, frameworkID uuid.UUID, groups []seed.Group) (map[string]uuid.UUID, error) {
	groupMap := make(map[string]uuid.UUID, len(groups))
	for i, group := range groups {
		created, err := q.CreateFrameworkGroup(ctx, db.CreateFrameworkGroupParams{
			FrameworkID: frameworkID,
			Code:        group.Code,
			Title:       group.Title,
			Summary:     group.Summary,
			Description: group.Description,
			SortOrder:   int32(i + 1),
		})
		if err != nil {
			return nil, fmt.Errorf("create framework group %s: %w", group.Code, err)
		}
		groupMap[group.Code] = created.ID
	}
	return groupMap, nil
}

func createFrameworkItems(ctx context.Context, q *db.Queries, frameworkID uuid.UUID, groupMap map[string]uuid.UUID, items []seed.Item) error {
	for i, item := range items {
		groupID, ok := groupMap[item.GroupCode]
		if !ok {
			return fmt.Errorf("%w: missing framework group %s", ErrInvalidInput, item.GroupCode)
		}
		if _, err := q.CreateFrameworkItem(ctx, db.CreateFrameworkItemParams{
			FrameworkID:      frameworkID,
			FrameworkGroupID: groupID,
			Code:             item.Code,
			Title:            item.Title,
			Summary:          item.Summary,
			Description:      item.Description,
			SortOrder:        int32(i + 1),
			AssetClass:       item.AssetClass,
			SecurityFunction: item.SecurityFunction,
			Tags:             item.Tags,
		}); err != nil {
			return fmt.Errorf("create framework item %s: %w", item.Code, err)
		}
	}
	return nil
}

func frameworkGroupCodes(groups []seed.Group) []string {
	codes := make([]string, 0, len(groups))
	for _, group := range groups {
		codes = append(codes, group.Code)
	}
	return codes
}

func frameworkItemCodes(items []seed.Item) []string {
	codes := make([]string, 0, len(items))
	for _, item := range items {
		codes = append(codes, item.Code)
	}
	return codes
}

func repairFrameworkItemReferences(ctx context.Context, q *db.Queries, frameworkID uuid.UUID, repairMap map[string]string) error {
	for oldCode, newCode := range repairMap {
		oldItem, err := q.GetFrameworkItemByCode(ctx, db.GetFrameworkItemByCodeParams{
			FrameworkID: frameworkID,
			Code:        oldCode,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return fmt.Errorf("get legacy framework item %s: %w", oldCode, err)
		}

		newItem, err := q.GetFrameworkItemByCode(ctx, db.GetFrameworkItemByCodeParams{
			FrameworkID: frameworkID,
			Code:        newCode,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return fmt.Errorf("get canonical framework item %s: %w", newCode, err)
		}

		assessmentIDs, err := q.ListAssessmentIDsEligibleForFrameworkItemRebind(ctx, db.ListAssessmentIDsEligibleForFrameworkItemRebindParams{
			OldFrameworkItemID: oldItem.ID,
			NewFrameworkItemID: newItem.ID,
		})
		if err != nil {
			return fmt.Errorf("list eligible assessments for %s -> %s: %w", oldCode, newCode, err)
		}
		if len(assessmentIDs) == 0 {
			continue
		}

		if err := q.RebindControlRecordsFrameworkItem(ctx, db.RebindControlRecordsFrameworkItemParams{
			OldFrameworkItemID: oldItem.ID,
			NewFrameworkItemID: newItem.ID,
			AssessmentIds:      assessmentIDs,
		}); err != nil {
			return fmt.Errorf("rebind control records for %s -> %s: %w", oldCode, newCode, err)
		}

		if err := q.RebindAssessmentItemsFrameworkItem(ctx, db.RebindAssessmentItemsFrameworkItemParams{
			OldFrameworkItemID: oldItem.ID,
			NewFrameworkItemID: newItem.ID,
			AssessmentIds:      assessmentIDs,
		}); err != nil {
			return fmt.Errorf("rebind assessment items for %s -> %s: %w", oldCode, newCode, err)
		}
	}

	return nil
}
