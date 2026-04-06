package seed

import "testing"

func TestLoadDocumentRejectsDuplicateItemCodes(t *testing.T) {
	data := []byte(`
framework:
  slug: cis-v8-1
  label: CIS Controls
  version: 8.1.2
groups:
  - code: "1"
    title: "Group"
    summary: "Summary"
items:
  - group_code: "1"
    code: "1.1"
    title: "Item A"
    summary: "Summary A"
    asset_class: "Devices"
    security_function: "Identify"
    tags: ["ig1"]
  - group_code: "1"
    code: "1.1"
    title: "Item B"
    summary: "Summary B"
    asset_class: "Devices"
    security_function: "Detect"
    tags: ["ig2"]
`)

	if _, err := LoadDocument(data); err == nil {
		t.Fatal("expected duplicate item code validation error")
	}
}

func TestSummarizeDescriptionCapsLength(t *testing.T) {
	text := "First sentence has the detail we want. Second sentence should never appear in the summary because the function should stop at the first sentence."
	got := SummarizeDescription(text)
	want := "First sentence has the detail we want."
	if got != want {
		t.Fatalf("unexpected summary: %q", got)
	}
}
