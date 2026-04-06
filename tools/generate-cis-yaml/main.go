package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
	"gopkg.in/yaml.v3"

	"mycis/internal/seed"
)

func main() {
	var (
		inputPath  = flag.String("input", "", "path to CIS workbook")
		outputPath = flag.String("output", filepath.Join("seed", "frameworks", "cis-v8-1.yaml"), "path to output yaml")
		slug       = flag.String("slug", "cis-v8-1", "framework slug")
		label      = flag.String("label", "CIS Controls", "framework label")
		version    = flag.String("version", "8.1.2", "framework version")
		sheet      = flag.String("sheet", "Controls v8.1.2", "worksheet name")
	)
	flag.Parse()

	if *inputPath == "" {
		fmt.Fprintln(os.Stderr, "-input is required")
		os.Exit(1)
	}

	doc, err := buildDocument(*inputPath, *sheet, *slug, *label, *version)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Dir(*outputPath), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	file, err := os.Create(*outputPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)
	if err := encoder.Encode(doc); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func buildDocument(inputPath, sheetName, slug, label, version string) (seed.Document, error) {
	workbook, err := excelize.OpenFile(inputPath)
	if err != nil {
		return seed.Document{}, fmt.Errorf("open workbook: %w", err)
	}
	defer workbook.Close()

	rows, err := workbook.GetRows(sheetName)
	if err != nil {
		return seed.Document{}, fmt.Errorf("read sheet %q: %w", sheetName, err)
	}

	doc := seed.Document{
		Framework: seed.Framework{
			Slug:    slug,
			Label:   label,
			Version: version,
		},
	}

	groups := make(map[string]seed.Group)

	for index, row := range rows {
		if index == 0 || len(row) == 0 {
			continue
		}

		controlCode := normalizeCode(cell(row, 0))
		safeguardCode := normalizeCode(cell(row, 1))
		title := strings.TrimSpace(cell(row, 4))
		description := strings.TrimSpace(cell(row, 5))
		if controlCode == "" || title == "" {
			continue
		}

		if safeguardCode == "" {
			groups[controlCode] = seed.Group{
				Code:        controlCode,
				Title:       title,
				Summary:     seed.SummarizeDescription(description),
				Description: seed.NormalizeDescription(description),
			}
			continue
		}

		doc.Items = append(doc.Items, seed.Item{
			GroupCode:        controlCode,
			Code:             safeguardCode,
			Title:            title,
			Summary:          seed.SummarizeDescription(description),
			Description:      seed.NormalizeDescription(description),
			AssetClass:       strings.TrimSpace(cell(row, 2)),
			SecurityFunction: strings.TrimSpace(cell(row, 3)),
			Tags:             collectTags(row),
		})
	}

	for _, group := range groups {
		doc.Groups = append(doc.Groups, group)
	}

	sortGroups(doc.Groups)
	sortItems(doc.Items)
	return doc, doc.Validate()
}

func cell(row []string, index int) string {
	if index >= len(row) {
		return ""
	}
	return row[index]
}

func normalizeCode(value string) string {
	clean := strings.ReplaceAll(strings.TrimSpace(value), "\u00a0", "")
	clean = strings.ReplaceAll(clean, " ", "")
	return clean
}

func collectTags(row []string) []string {
	var tags []string
	for offset, tag := range []string{"ig1", "ig2", "ig3"} {
		if strings.EqualFold(strings.TrimSpace(cell(row, 6+offset)), "x") {
			tags = append(tags, tag)
		}
	}
	return tags
}

func sortGroups(groups []seed.Group) {
	sort.Slice(groups, func(i, j int) bool {
		return compareCodes(groups[i].Code, groups[j].Code) < 0
	})
}

func sortItems(items []seed.Item) {
	sort.Slice(items, func(i, j int) bool {
		return compareCodes(items[i].Code, items[j].Code) < 0
	})
}

func compareCodes(left, right string) int {
	leftParts := strings.Split(left, ".")
	rightParts := strings.Split(right, ".")
	for i := 0; i < len(leftParts) || i < len(rightParts); i++ {
		leftPart := numericPart(leftParts, i)
		rightPart := numericPart(rightParts, i)
		if leftPart == rightPart {
			continue
		}
		if leftPart < rightPart {
			return -1
		}
		return 1
	}
	return 0
}

func numericPart(parts []string, index int) int {
	if index >= len(parts) {
		return 0
	}
	value, err := strconv.Atoi(parts[index])
	if err != nil {
		return 0
	}
	return value
}
