package seed

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type Document struct {
	Framework Framework `yaml:"framework"`
	Groups    []Group   `yaml:"groups"`
	Items     []Item    `yaml:"items"`
}

type Framework struct {
	Slug    string `yaml:"slug"`
	Label   string `yaml:"label"`
	Version string `yaml:"version"`
}

type Group struct {
	Code        string `yaml:"code"`
	Title       string `yaml:"title"`
	Summary     string `yaml:"summary"`
	Description string `yaml:"description"`
}

type Item struct {
	GroupCode        string   `yaml:"group_code"`
	Code             string   `yaml:"code"`
	Title            string   `yaml:"title"`
	Summary          string   `yaml:"summary"`
	Description      string   `yaml:"description"`
	AssetClass       string   `yaml:"asset_class"`
	SecurityFunction string   `yaml:"security_function"`
	Tags             []string `yaml:"tags"`
}

func LoadDocument(data []byte) (Document, error) {
	var doc Document
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return Document{}, fmt.Errorf("unmarshal seed yaml: %w", err)
	}

	if err := doc.Validate(); err != nil {
		return Document{}, err
	}

	return doc, nil
}

func (d Document) Validate() error {
	if strings.TrimSpace(d.Framework.Slug) == "" || strings.TrimSpace(d.Framework.Label) == "" || strings.TrimSpace(d.Framework.Version) == "" {
		return errors.New("framework slug, label, and version are required")
	}

	groupCodes := make(map[string]struct{}, len(d.Groups))
	for _, group := range d.Groups {
		if strings.TrimSpace(group.Code) == "" || strings.TrimSpace(group.Title) == "" || strings.TrimSpace(group.Summary) == "" {
			return fmt.Errorf("group %q has missing required fields", group.Code)
		}
		if _, exists := groupCodes[group.Code]; exists {
			return fmt.Errorf("duplicate group code %q", group.Code)
		}
		groupCodes[group.Code] = struct{}{}
	}

	itemCodes := make(map[string]struct{}, len(d.Items))
	for _, item := range d.Items {
		if strings.TrimSpace(item.GroupCode) == "" ||
			strings.TrimSpace(item.Code) == "" ||
			strings.TrimSpace(item.Title) == "" ||
			strings.TrimSpace(item.Summary) == "" ||
			strings.TrimSpace(item.AssetClass) == "" ||
			strings.TrimSpace(item.SecurityFunction) == "" {
			return fmt.Errorf("item %q has missing required fields", item.Code)
		}
		if _, ok := groupCodes[item.GroupCode]; !ok {
			return fmt.Errorf("item %q references unknown group %q", item.Code, item.GroupCode)
		}
		if _, exists := itemCodes[item.Code]; exists {
			return fmt.Errorf("duplicate item code %q", item.Code)
		}
		itemCodes[item.Code] = struct{}{}
	}

	return nil
}

func NormalizeDescription(description string) string {
	return strings.Join(strings.Fields(description), " ")
}

func SummarizeDescription(description string) string {
	trimmed := strings.Join(strings.Fields(description), " ")
	if trimmed == "" {
		return ""
	}

	if idx := strings.Index(trimmed, ". "); idx >= 0 {
		trimmed = trimmed[:idx+1]
	}
	if len(trimmed) > 220 {
		trimmed = strings.TrimSpace(trimmed[:220])
		trimmed = strings.TrimRight(trimmed, ",;:-")
		trimmed += "..."
	}

	return trimmed
}
