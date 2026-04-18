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

type frameworkItemRepairRule struct {
	LegacyCode    string
	CanonicalCode string
}

// Repair rules are limited to safe one-to-one rebinding. Legacy many-to-one
// consolidation needs explicit merge semantics and is intentionally unsupported.
var frameworkItemRepairRules = map[string][]frameworkItemRepairRule{
	"nist-csf-2-0": {
		{
			LegacyCode:    "PR.DS-04",
			CanonicalCode: "PR.IR-04",
		},
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
		framework, existed, err := ensureFrameworkForSeed(ctx, q, doc.Framework, force)
		if err != nil {
			return struct{}{}, err
		}

		groupMap, err := persistFrameworkGroups(ctx, q, framework.ID, doc.Groups)
		if err != nil {
			return struct{}{}, err
		}
		if err := persistFrameworkItems(ctx, q, framework.ID, groupMap, doc.Items); err != nil {
			return struct{}{}, err
		}
		if !existed {
			return struct{}{}, nil
		}

		// Force-reseed: keep historical framework rows for old assessments, but deactivate
		// anything removed from the current document so new assessments and cycles only
		// use the active framework definition.
		if err := repairFrameworkItemReferences(ctx, q, framework.ID, frameworkItemRepairRules[doc.Framework.Slug]); err != nil {
			return struct{}{}, err
		}
		if err := q.DeactivateMissingFrameworkItems(ctx, db.DeactivateMissingFrameworkItemsParams{
			FrameworkID: framework.ID,
			Codes:       frameworkItemCodes(doc.Items),
		}); err != nil {
			return struct{}{}, fmt.Errorf("deactivate framework items: %w", err)
		}
		if err := q.DeactivateMissingFrameworkGroups(ctx, db.DeactivateMissingFrameworkGroupsParams{
			FrameworkID: framework.ID,
			Codes:       frameworkGroupCodes(doc.Groups),
		}); err != nil {
			return struct{}{}, fmt.Errorf("deactivate framework groups: %w", err)
		}
		return struct{}{}, nil
	})

	return err
}

func ensureFrameworkForSeed(ctx context.Context, q *db.Queries, framework seed.Framework, force bool) (db.Framework, bool, error) {
	existing, err := q.GetFrameworkBySlugVersion(ctx, db.GetFrameworkBySlugVersionParams{
		Slug:    framework.Slug,
		Version: framework.Version,
	})
	if err == nil {
		if !force {
			return db.Framework{}, false, fmt.Errorf("%w: framework %s %s already exists", ErrConflict, framework.Slug, framework.Version)
		}
		return existing, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return db.Framework{}, false, fmt.Errorf("check framework: %w", err)
	}

	created, err := q.CreateFramework(ctx, db.CreateFrameworkParams{
		Slug:    framework.Slug,
		Label:   framework.Label,
		Version: framework.Version,
		Status:  "active",
	})
	if err != nil {
		return db.Framework{}, false, fmt.Errorf("create framework: %w", err)
	}
	return created, false, nil
}

func persistFrameworkGroups(ctx context.Context, q *db.Queries, frameworkID uuid.UUID, groups []seed.Group) (map[string]uuid.UUID, error) {
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

func persistFrameworkItems(ctx context.Context, q *db.Queries, frameworkID uuid.UUID, groupMap map[string]uuid.UUID, items []seed.Item) error {
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

func repairFrameworkItemReferences(ctx context.Context, q *db.Queries, frameworkID uuid.UUID, rules []frameworkItemRepairRule) error {
	for _, rule := range rules {
		oldItem, err := q.GetFrameworkItemByCode(ctx, db.GetFrameworkItemByCodeParams{
			FrameworkID: frameworkID,
			Code:        rule.LegacyCode,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return fmt.Errorf("get legacy framework item %s: %w", rule.LegacyCode, err)
		}

		newItem, err := q.GetFrameworkItemByCode(ctx, db.GetFrameworkItemByCodeParams{
			FrameworkID: frameworkID,
			Code:        rule.CanonicalCode,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return fmt.Errorf("get canonical framework item %s: %w", rule.CanonicalCode, err)
		}

		if err := q.RebindFrameworkItemReferences(ctx, db.RebindFrameworkItemReferencesParams{
			OldFrameworkItemID: oldItem.ID,
			NewFrameworkItemID: newItem.ID,
		}); err != nil {
			return fmt.Errorf("rebind framework item references for %s -> %s: %w", rule.LegacyCode, rule.CanonicalCode, err)
		}
	}

	return nil
}
