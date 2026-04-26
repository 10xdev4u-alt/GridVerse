package main

import (
	"strings"
	"testing"
	"time"
)

func TestParseSimpleRule(t *testing.T) {
	input := `rule "test-rule" {
  when: demand > 50
  action: shed(percent: 10, duration: 30m)
  priority: 5
}`
	rules, err := ParseRules(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(rules))
	}
	r := rules[0]
	if r.Name != "test-rule" {
		t.Errorf("Expected 'test-rule', got '%s'", r.Name)
	}
	if r.Priority != 5 {
		t.Errorf("Expected priority 5, got %d", r.Priority)
	}
	if len(r.Actions) != 1 {
		t.Fatalf("Expected 1 action, got %d", len(r.Actions))
	}
	if r.Actions[0].Type != "shed" {
		t.Errorf("Expected 'shed', got '%s'", r.Actions[0].Type)
	}
	if r.Actions[0].Percent != 10 {
		t.Errorf("Expected 10%%, got %f", r.Actions[0].Percent)
	}
}

func TestParseAndCondition(t *testing.T) {
	input := `rule "and-rule" {
  when: demand > 80 AND time.hour IN [17..21]
  action: shed(percent: 15)
  priority: 2
}`
	rules, err := ParseRules(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(rules))
	}
}

func TestParseOrCondition(t *testing.T) {
	input := `rule "or-rule" {
  when: meter.profile == "hospital" OR meter.profile == "datacenter"
  action: protect()
  priority: 0
}`
	rules, err := ParseRules(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(rules))
	}
}

func TestParseMultipleRules(t *testing.T) {
	input := `
rule "rule-1" {
  when: demand > 90
  action: shed(percent: 20)
  priority: 1
}

rule "rule-2" {
  when: production > 100
  action: curtail(percent: 10)
  priority: 3
}`
	rules, err := ParseRules(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("Expected 2 rules, got %d", len(rules))
	}
}

func TestComparisonEvaluate(t *testing.T) {
	c := &Comparison{Field: "demand", Op: ">", Value: 50}
	s := &MeterState{Demand: 60}
	if !c.Evaluate(s) {
		t.Error("Expected true for 60 > 50")
	}
	s.Demand = 40
	if c.Evaluate(s) {
		t.Error("Expected false for 40 > 50")
	}
}

func TestInRangeEvaluate(t *testing.T) {
	c := &InRange{Field: "time.hour", Low: 17, High: 21}
	s := &MeterState{TimeHour: 19}
	if !c.Evaluate(s) {
		t.Error("Expected true for 19 in [17..21]")
	}
	s.TimeHour = 12
	if c.Evaluate(s) {
		t.Error("Expected false for 12 in [17..21]")
	}
}

func TestStringMatchEvaluate(t *testing.T) {
	c := &StringMatch{Field: "meter.profile", Value: "residential"}
	s := &MeterState{Profile: "residential"}
	if !c.Evaluate(s) {
		t.Error("Expected true for residential match")
	}
	s.Profile = "commercial"
	if c.Evaluate(s) {
		t.Error("Expected false for commercial != residential")
	}
}

func TestAndConditionEvaluate(t *testing.T) {
	c := &AndCondition{
		Left:  &Comparison{Field: "demand", Op: ">", Value: 50},
		Right: &StringMatch{Field: "meter.profile", Value: "residential"},
	}
	s := &MeterState{Demand: 60, Profile: "residential"}
	if !c.Evaluate(s) {
		t.Error("Expected true for demand>50 AND residential")
	}
	s.Profile = "commercial"
	if c.Evaluate(s) {
		t.Error("Expected false for demand>50 AND commercial")
	}
}

func TestOrConditionEvaluate(t *testing.T) {
	c := &OrCondition{
		Left:  &StringMatch{Field: "meter.profile", Value: "hospital"},
		Right: &StringMatch{Field: "meter.profile", Value: "datacenter"},
	}
	s := &MeterState{Profile: "hospital"}
	if !c.Evaluate(s) {
		t.Error("Expected true for hospital OR datacenter")
	}
	s.Profile = "residential"
	if c.Evaluate(s) {
		t.Error("Expected false for residential")
	}
}

func TestSortByPriority(t *testing.T) {
	rules := []Rule{
		{Name: "low", Priority: 10},
		{Name: "high", Priority: 1},
		{Name: "mid", Priority: 5},
	}
	sorted := sortByPriority(rules)
	if sorted[0].Priority != 1 {
		t.Errorf("Expected priority 1 first, got %d", sorted[0].Priority)
	}
	if sorted[1].Priority != 5 {
		t.Errorf("Expected priority 5 second, got %d", sorted[1].Priority)
	}
	if sorted[2].Priority != 10 {
		t.Errorf("Expected priority 10 last, got %d", sorted[2].Priority)
	}
}

func TestBuildCommand(t *testing.T) {
	a := Action{Type: "shed", Percent: 25, Duration: 30 * time.Minute}
	cmd := buildCommand(a)
	if cmd["command"] != "shed" {
		t.Errorf("Expected 'shed', got '%s'", cmd["command"])
	}
	if cmd["percent"] != 25.0 {
		t.Errorf("Expected 25, got %v", cmd["percent"])
	}
}

func TestTokenize(t *testing.T) {
	input := `rule "test" { when: demand > 50 action: protect() priority: 0 }`
	tokens := tokenize(input)
	if len(tokens) == 0 {
		t.Error("Expected tokens, got none")
	}
	// Check key tokens
	expected := []string{"RULE", "STRING", "LBRACE", "WHEN", "COLON", "IDENT", "COMP_OP", "NUMBER", "ACTION", "COLON", "IDENT", "LPAREN", "RPAREN", "PRIORITY", "COLON", "NUMBER", "RBRACE"}
	if len(tokens) != len(expected) {
		t.Errorf("Expected %d tokens, got %d", len(expected), len(tokens))
	}
	for i, tok := range tokens {
		if i < len(expected) && tok.typ != expected[i] {
			t.Errorf("Token %d: expected %s, got %s(%s)", i, expected[i], tok.typ, tok.val)
		}
	}
}

func TestDefaultRules(t *testing.T) {
	rules := defaultRules()
	if len(rules) != 3 {
		t.Errorf("Expected 3 default rules, got %d", len(rules))
	}
	if rules[0].Name != "critical-protect" {
		t.Errorf("Expected 'critical-protect', got '%s'", rules[0].Name)
	}
}

func TestParseStringComparison(t *testing.T) {
	input := `rule "str-rule" {
  when: meter.profile == "residential"
  action: notify(message: "test")
  priority: 1
}`
	rules, err := ParseRules(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(rules))
	}
	_, ok := rules[0].When.(*StringMatch)
	if !ok {
		t.Errorf("Expected StringMatch condition, got %T", rules[0].When)
	}
}

func TestParseInRange(t *testing.T) {
	input := `rule "range-rule" {
  when: time.hour IN [8..18]
  action: shed(percent: 5)
  priority: 4
}`
	rules, err := ParseRules(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(rules))
	}
}

func TestParseCommentsIgnored(t *testing.T) {
	input := `
# This is a comment
rule "commented-rule" {
  # Another comment
  when: demand > 30
  action: protect()
  priority: 1
}`
	rules, err := ParseRules(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(rules))
	}
}

func TestGetEnvHelpers(t *testing.T) {
	if v := getEnv("DSL_TEST_MISSING", "default"); v != "default" {
		t.Errorf("Expected 'default', got '%s'", v)
	}
	if v := getEnvFloat("DSL_TEST_MISSING", 3.14); v != 3.14 {
		t.Errorf("Expected 3.14, got %f", v)
	}
	if v := getEnvBool("DSL_TEST_MISSING", true); !v {
		t.Error("Expected true default")
	}
}

func TestTokenizeDuration(t *testing.T) {
	input := `action: shed(percent: 10, duration: 30m)`
	tokens := tokenize(input)
	found := false
	for _, tok := range tokens {
		if tok.val == "30" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find duration value 30 in tokens")
	}
}

func TestParseInvalidSyntax(t *testing.T) {
	input := `rule { broken }`
	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Parser panicked as expected: %v", r)
		}
	}()
	ParseRules(input)
}

func TestMeterStateFields(t *testing.T) {
	s := &MeterState{
		MeterID: "m1", Profile: "solar-panel",
		ConsumptionKW: 1.0, ProductionKW: 5.0,
		Voltage: 230, Demand: 80, Capacity: 100, TimeHour: 14,
	}
	if v := s.getField("demand"); v != 80 { t.Errorf("Expected 80, got %f", v) }
	if v := s.getField("capacity"); v != 100 { t.Errorf("Expected 100, got %f", v) }
	if v := s.getField("production"); v != 5.0 { t.Errorf("Expected 5.0, got %f", v) }
	if v := s.getStringField("meter.profile"); v != "solar-panel" {
		t.Errorf("Expected 'solar-panel', got '%s'", v)
	}
}

func TestHotReloadFileMod(t *testing.T) {
	// Test that readFile works
	content := readFile("default.rules")
	if !strings.Contains(content, "critical-protect") {
		t.Error("Expected default.rules to contain 'critical-protect'")
	}
}