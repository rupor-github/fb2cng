package kfx

import (
	"testing"
)

// TestIonDataLoaded verifies that ion data was loaded successfully and completely.
func TestIonDataLoaded(t *testing.T) {
	// Verify StyleList loaded
	t.Run("StyleList", func(t *testing.T) {
		if len(defaultStyleListEntries) == 0 {
			t.Fatal("StyleList is empty")
		}
		t.Logf("StyleList: %d entries loaded", len(defaultStyleListEntries))

		// Verify all entries have required fields
		for i, e := range defaultStyleListEntries {
			if e.Key == "" {
				t.Errorf("StyleList[%d] has empty Key", i)
			}
			if e.Class == "" {
				t.Errorf("StyleList[%d] has empty Class", i)
			}
		}
	})

	// Verify StyleMap loaded
	t.Run("StyleMap", func(t *testing.T) {
		if len(defaultStyleMapEntries) == 0 {
			t.Fatal("StyleMap is empty")
		}
		t.Logf("StyleMap: %d entries loaded", len(defaultStyleMapEntries))

		// Count entries with various fields populated to verify loading
		withHTMLTag := 0
		withHTMLAttr := 0
		withProperty := 0
		withValue := 0
		withUnit := 0
		withValueType := 0
		withDisplay := 0
		withCSSStyles := 0
		withTransformer := 0
		withIgnoreHTML := 0

		for _, e := range defaultStyleMapEntries {
			if e.Key.Tag != "" {
				withHTMLTag++
			}
			if e.Key.Attr != "" {
				withHTMLAttr++
			}
			if e.Property != "" {
				withProperty++
			}
			if e.Value != "" {
				withValue++
			}
			if e.Unit != "" {
				withUnit++
			}
			if e.ValueType != "" {
				withValueType++
			}
			if e.Display != "" {
				withDisplay++
			}
			if len(e.CSSStyles) > 0 {
				withCSSStyles++
			}
			if e.Transformer != "" {
				withTransformer++
			}
			if e.IgnoreHTML {
				withIgnoreHTML++
			}
		}

		t.Logf("  with Key.Tag: %d", withHTMLTag)
		t.Logf("  with Key.Attr: %d", withHTMLAttr)
		t.Logf("  with Property: %d", withProperty)
		t.Logf("  with Value: %d", withValue)
		t.Logf("  with Unit: %d", withUnit)
		t.Logf("  with ValueType: %d", withValueType)
		t.Logf("  with Display: %d", withDisplay)
		t.Logf("  with CSSStyles: %d", withCSSStyles)
		t.Logf("  with Transformer: %d", withTransformer)
		t.Logf("  with IgnoreHTML=true: %d", withIgnoreHTML)
	})

	// Verify IgnorablePatterns loaded
	t.Run("IgnorablePatterns", func(t *testing.T) {
		if len(defaultIgnorablePatterns) == 0 {
			t.Fatal("IgnorablePatterns is empty")
		}
		t.Logf("IgnorablePatterns: %d entries loaded", len(defaultIgnorablePatterns))

		// Verify entries have data
		withTag := 0
		withStyle := 0
		withValue := 0
		withUnit := 0

		for _, e := range defaultIgnorablePatterns {
			if e.Tag != "" {
				withTag++
			}
			if e.Style != "" {
				withStyle++
			}
			if e.Value != "" {
				withValue++
			}
			if e.Unit != "" {
				withUnit++
			}
		}

		t.Logf("  with Tag: %d", withTag)
		t.Logf("  with Style: %d", withStyle)
		t.Logf("  with Value: %d", withValue)
		t.Logf("  with Unit: %d", withUnit)
	})
}
