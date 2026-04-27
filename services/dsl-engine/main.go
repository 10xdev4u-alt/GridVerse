package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
)

type Reading struct {
	MeterID       string  `json:"meter_id"`
	Profile       string  `json:"profile"`
	Timestamp     string  `json:"timestamp"`
	ConsumptionKW float64 `json:"consumption_kw"`
	ProductionKW  float64 `json:"production_kw"`
	Voltage       float64 `json:"voltage"`
	Status        string  `json:"status"`
}

type Rule struct {
	Name     string
	When     Condition
	Actions  []Action
	Priority int
}

type Action struct {
	Type     string
	Percent  float64
	Duration time.Duration
}

type Condition interface {
	Evaluate(state *MeterState) bool
}

type AndCondition struct{ Left, Right Condition }
type OrCondition struct{ Left, Right Condition }
type Comparison struct {
	Field string
	Op    string
	Value float64
}
type InRange struct {
	Field string
	Low   float64
	High  float64
}
type StringMatch struct {
	Field string
	Value string
}

func (c *AndCondition) Evaluate(s *MeterState) bool  { return c.Left.Evaluate(s) && c.Right.Evaluate(s) }
func (c *OrCondition) Evaluate(s *MeterState) bool   { return c.Left.Evaluate(s) || c.Right.Evaluate(s) }
func (c *Comparison) Evaluate(s *MeterState) bool {
	val := s.getField(c.Field)
	switch c.Op {
	case ">": return val > c.Value
	case "<": return val < c.Value
	case ">=": return val >= c.Value
	case "<=": return val <= c.Value
	case "==": return val == c.Value
	}
	return false
}
func (c *InRange) Evaluate(s *MeterState) bool {
	val := s.getField(c.Field)
	return val >= c.Low && val <= c.High
}
func (c *StringMatch) Evaluate(s *MeterState) bool {
	return s.getStringField(c.Field) == c.Value
}

type MeterState struct {
	MeterID       string
	Profile       string
	ConsumptionKW float64
	ProductionKW  float64
	Voltage       float64
	Status        string
	Demand        float64
	Capacity      float64
	TimeHour      float64
}

func (s *MeterState) getField(name string) float64 {
	switch name {
	case "demand": return s.Demand
	case "capacity": return s.Capacity
	case "consumption": return s.ConsumptionKW
	case "production": return s.ProductionKW
	case "time.hour": return s.TimeHour
	case "voltage": return s.Voltage
	}
	return 0
}

func (s *MeterState) getStringField(name string) string {
	switch name {
	case "meter.profile": return s.Profile
	case "status": return s.Status
	}
	return ""
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" { return v }
	return def
}

func getEnvFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil { return f }
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil { return b }
	}
	return def
}

func main() {
	natsURL := getEnv("NATS_URL", "nats://nats:4222")
	rulesFile := getEnv("RULES_FILE", "/rules/default.rules")
	evalInterval := getEnvFloat("EVAL_INTERVAL", 5)
	dryRun := getEnvBool("DRY_RUN", false)

	nc, err := nats.Connect(natsURL,
		nats.ReconnectWait(2*time.Second),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()
	log.Printf("DSL Engine connected to NATS at %s", natsURL)
	if dryRun {
		log.Println("DRY RUN MODE — no commands will be published")
	}

	var mu sync.RWMutex
	meters := make(map[string]*MeterState)
	capacity := 100.0

	// Subscribe to meter readings to build state
	nc.Subscribe("meter.*.readings", func(msg *nats.Msg) {
		var r Reading
		if err := json.Unmarshal(msg.Data, &r); err != nil { return }
		mu.Lock()
		meters[r.MeterID] = &MeterState{
			MeterID: r.MeterID, Profile: r.Profile,
			ConsumptionKW: r.ConsumptionKW, ProductionKW: r.ProductionKW,
			Voltage: r.Voltage, Status: r.Status,
			Capacity: capacity, TimeHour: float64(time.Now().Hour()),
		}
		mu.Unlock()
	})

	// Subscribe to edge aggregated to get demand
	nc.Subscribe("edge.aggregated", func(msg *nats.Msg) {
		var a struct {
			AvgConsumption float64 `json:"avg_consumption"`
			AvgProduction  float64 `json:"avg_production"`
		}
		if err := json.Unmarshal(msg.Data, &a); err != nil { return }
		mu.Lock()
		for _, m := range meters {
			m.Demand = a.AvgConsumption
		}
		mu.Unlock()
	})

	// Parse initial rules
	var rules []Rule
	if data, err := os.ReadFile(rulesFile); err == nil {
		rules, err = ParseRules(string(data))
		if err != nil {
			log.Printf("Failed to parse rules: %v", err)
		} else {
			log.Printf("Loaded %d rules from %s", len(rules), rulesFile)
		}
	} else {
		log.Printf("No rules file found at %s, using defaults", rulesFile)
		rules = defaultRules()
	}

	// Hot-reload watcher
	lastMod := time.Now()
	go func() {
		for {
			time.Sleep(5 * time.Second)
			if info, err := os.Stat(rulesFile); err == nil {
				if info.ModTime().After(lastMod) {
					lastMod = info.ModTime()
					newRules, err := ParseRules(string(func() []byte {
						d, _ := os.ReadFile(rulesFile); return d
					}()))
					if err == nil {
						mu.Lock()
						rules = newRules
						mu.Unlock()
						log.Printf("Hot-reloaded %d rules", len(rules))
					}
				}
			}
		}
	}()

	// Evaluation loop
	ticker := time.NewTicker(time.Duration(evalInterval) * time.Second)
	defer ticker.Stop()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			mu.Lock()
			handled := make(map[string]bool)
			for _, rule := range sortByPriority(rules) {
				for id, meter := range meters {
					if handled[id] { continue }
					meter.TimeHour = float64(time.Now().Hour())
					if rule.When.Evaluate(meter) {
						for _, action := range rule.Actions {
							cmd := buildCommand(action)
							log.Printf("Rule %q → %s: %+v", rule.Name, id, cmd)
							if !dryRun {
								data, _ := json.Marshal(cmd)
								nc.Publish(fmt.Sprintf("meter.%s.commands", id), data)
							}
						}
						handled[id] = true
					}
				}
			}
			mu.Unlock()

		case <-sig:
			log.Println("Shutting down DSL Engine...")
			return
		}
	}
}

func buildCommand(a Action) map[string]interface{} {
	cmd := map[string]interface{}{"command": a.Type}
	if a.Percent > 0 { cmd["percent"] = a.Percent }
	if a.Duration > 0 { cmd["duration"] = a.Duration.String() }
	return cmd
}

func sortByPriority(rules []Rule) []Rule {
	sorted := make([]Rule, len(rules))
	copy(sorted, rules)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].Priority > sorted[j].Priority {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return sorted
}

func defaultRules() []Rule {
	return []Rule{
		{
			Name: "critical-protect",
			When: &OrCondition{
				Left:  &StringMatch{Field: "meter.profile", Value: "hospital"},
				Right: &StringMatch{Field: "meter.profile", Value: "datacenter"},
			},
			Actions:  []Action{{Type: "protect"}},
			Priority: 0,
		},
		{
			Name: "peak-shed-residential",
			When: &AndCondition{
				Left: &AndCondition{
					Left:  &Comparison{Field: "demand", Op: ">", Value: 85},
					Right: &InRange{Field: "time.hour", Low: 17, High: 21},
				},
				Right: &StringMatch{Field: "meter.profile", Value: "residential"},
			},
			Actions:  []Action{{Type: "shed", Percent: 10, Duration: 30 * time.Minute}},
			Priority: 2,
		},
		{
			Name: "curtail-solar-overload",
			When: &AndCondition{
				Left:  &Comparison{Field: "production", Op: ">", Value: 110},
				Right: &StringMatch{Field: "meter.profile", Value: "solar-panel"},
			},
			Actions:  []Action{{Type: "curtail", Percent: 20}},
			Priority: 3,
		},
	}
}

// Parser
type token struct {
	typ string
	val string
}

func tokenize(input string) []token {
	var tokens []token
	input = strings.ReplaceAll(input, "\r\n", "\n")
	i := 0
	for i < len(input) {
		ch := input[i]
		switch {
		case ch == ' ' || ch == '\t' || ch == '\n':
			i++
		case ch == '#':
			for i < len(input) && input[i] != '\n' { i++ }
		case ch == '"':
			j := i + 1
			for j < len(input) && input[j] != '"' { j++ }
			tokens = append(tokens, token{"STRING", input[i+1 : j]})
			i = j + 1
		case ch == '{': tokens = append(tokens, token{"LBRACE", "{"}); i++
		case ch == '}': tokens = append(tokens, token{"RBRACE", "}"}); i++
		case ch == '[': tokens = append(tokens, token{"LBRACKET", "["}); i++
		case ch == ']': tokens = append(tokens, token{"RBRACKET", "]"}); i++
		case ch == '(': tokens = append(tokens, token{"LPAREN", "("}); i++
		case ch == ')': tokens = append(tokens, token{"RPAREN", ")"}); i++
		case ch == ':': tokens = append(tokens, token{"COLON", ":"}); i++
		case ch == ',': tokens = append(tokens, token{"COMMA", ","}); i++
		case ch == '%': tokens = append(tokens, token{"PERCENT", "%"}); i++
		case ch == '>' && i+1 < len(input) && input[i+1] == '=':
			tokens = append(tokens, token{"COMP_OP", ">="}); i += 2
		case ch == '<' && i+1 < len(input) && input[i+1] == '=':
			tokens = append(tokens, token{"COMP_OP", "<="}); i += 2
		case ch == '=' && i+1 < len(input) && input[i+1] == '=':
			tokens = append(tokens, token{"COMP_OP", "=="}); i += 2
		case ch == '>': tokens = append(tokens, token{"COMP_OP", ">"}); i++
		case ch == '<': tokens = append(tokens, token{"COMP_OP", "<"}); i++
		case ch == '.' && i+1 < len(input) && input[i+1] == '.':
			tokens = append(tokens, token{"RANGE", ".."}); i += 2
		case ch >= '0' && ch <= '9':
			j := i
			for j < len(input) && (input[j] >= '0' && input[j] <= '9' || input[j] == '.') {
				// Stop if dot is part of a range (..)
				if input[j] == '.' && j+1 < len(input) && input[j+1] == '.' { break }
				j++
			}
			tokens = append(tokens, token{"NUMBER", input[i:j]})
			i = j
		case ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch == '_':
			j := i
			for j < len(input) && (input[j] >= 'a' && input[j] <= 'z' || input[j] >= 'A' && input[j] <= 'Z' || input[j] == '_' || input[j] == '.') { j++ }
			word := input[i:j]
			switch word {
			case "rule": tokens = append(tokens, token{"RULE", word})
			case "when": tokens = append(tokens, token{"WHEN", word})
			case "action": tokens = append(tokens, token{"ACTION", word})
			case "priority": tokens = append(tokens, token{"PRIORITY", word})
			case "AND": tokens = append(tokens, token{"AND", word})
			case "OR": tokens = append(tokens, token{"OR", word})
			case "IN": tokens = append(tokens, token{"IN", word})
			default: tokens = append(tokens, token{"IDENT", word})
			}
			i = j
		default:
			i++
		}
	}
	return tokens
}

type parser struct {
	tokens []token
	pos    int
}

func (p *parser) peek() token {
	if p.pos < len(p.tokens) { return p.tokens[p.pos] }
	return token{"EOF", ""}
}

func (p *parser) consume() token {
	t := p.peek()
	if p.pos < len(p.tokens) { p.pos++ }
	return t
}

func (p *parser) expect(typ string) token {
	t := p.consume()
	if t.typ != typ { panic(fmt.Sprintf("expected %s, got %s(%s)", typ, t.typ, t.val)) }
	return t
}

func ParseRules(input string) ([]Rule, error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Parse error: %v", r)
		}
	}()
	p := &parser{tokens: tokenize(input)}
	var rules []Rule
	for p.peek().typ != "EOF" {
		rules = append(rules, parseRule(p))
	}
	return rules, nil
}

func parseRule(p *parser) Rule {
	p.expect("RULE")
	name := p.expect("STRING").val
	p.expect("LBRACE")
	var when Condition
	var actions []Action
	priority := 99

	for p.peek().typ != "RBRACE" && p.peek().typ != "EOF" {
		switch p.peek().typ {
		case "WHEN":
			p.consume()
			p.expect("COLON")
			when = parseCondition(p)
		case "ACTION":
			p.consume()
			p.expect("COLON")
			actions = append(actions, parseAction(p))
		case "PRIORITY":
			p.consume()
			p.expect("COLON")
			priority = parseInt(p.expect("NUMBER").val)
		default:
			p.consume()
		}
	}
	p.expect("RBRACE")
	return Rule{Name: name, When: when, Actions: actions, Priority: priority}
}

func parseCondition(p *parser) Condition {
	left := parseExpr(p)
	for p.peek().typ == "AND" || p.peek().typ == "OR" {
		op := p.consume().typ
		right := parseExpr(p)
		if op == "AND" {
			left = &AndCondition{Left: left, Right: right}
		} else {
			left = &OrCondition{Left: left, Right: right}
		}
	}
	return left
}

func parseExpr(p *parser) Condition {
	field := p.expect("IDENT").val

	if p.peek().typ == "IN" {
		p.consume()
		p.expect("LBRACKET")
		low := parseFloat(p.expect("NUMBER").val)
		p.expect("RANGE")
		high := parseFloat(p.expect("NUMBER").val)
		p.expect("RBRACKET")
		return &InRange{Field: field, Low: low, High: high}
	}

	if p.peek().typ == "COMP_OP" {
		op := p.consume().val
		if p.peek().typ == "STRING" {
			val := p.consume().val
			return &StringMatch{Field: field, Value: val}
		}
		val := parseFloat(p.expect("NUMBER").val)
		return &Comparison{Field: field, Op: op, Value: val}
	}

	// Bare field check (for string match like meter.profile == "x")
	if p.peek().typ == "STRING" {
		// This handles the case: meter.profile == "residential" where == was already consumed
		return &StringMatch{Field: field, Value: p.consume().val}
	}

	return &Comparison{Field: field, Op: ">", Value: 0}
}

func parseAction(p *parser) Action {
	name := p.expect("IDENT").val
	p.expect("LPAREN")
	a := Action{Type: name}
	for p.peek().typ != "RPAREN" && p.peek().typ != "EOF" {
		key := p.expect("IDENT").val
		p.expect("COLON")
		if key == "percent" {
			a.Percent = parseFloat(p.expect("NUMBER").val)
		} else if key == "duration" {
			val := p.expect("NUMBER").val
			unit := p.expect("IDENT").val
			d, err := time.ParseDuration(val + unit)
			if err == nil { a.Duration = d }
		} else {
			p.consume() // skip value
		}
		if p.peek().typ == "COMMA" { p.consume() }
	}
	p.expect("RPAREN")
	return a
}

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func parseInt(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

// Helper for hot-reload file read
func readFile(path string) string {
	d, _ := os.ReadFile(path)
	return string(d)
}

// Unused stubs to satisfy imports
var _ = math.Abs
var _ = readFile