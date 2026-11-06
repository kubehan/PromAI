package main

import "testing"

func TestRiskLevelFor(t *testing.T) {
	cases := map[string]string{
		"df -h":                       "low",
		"ps aux":                      "low",
		"tail -n 50 app.log":          "low",
		"mkdir -p /opt/x":             "medium",
		"systemctl restart nginx":     "medium",
		"rm -rf /tmp/*":               "high",
		"kill -9 1234":                "high",
		"dd if=/dev/zero of=/dev/sda": "high",
		"cat /etc/passwd":             "low",
	}
	for in, want := range cases {
		if got := riskLevelFor(in); got != want {
			t.Errorf("riskLevelFor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeRisk(t *testing.T) {
	if got := normalizeRisk("low", "rm -rf /tmp/x"); got != "high" {
		t.Errorf("normalizeRisk high hard-stop: got %q", got)
	}
	if got := normalizeRisk("high", "df -h"); got != "high" {
		t.Errorf("LLM high should win: got %q", got)
	}
	if got := normalizeRisk("medium", "echo hi"); got != "medium" {
		t.Errorf("LLM medium should win: got %q", got)
	}
	if got := normalizeRisk("", "df -h"); got != "low" {
		t.Errorf("heuristic fallback: got %q", got)
	}
	if got := normalizeRisk("", "mv a b"); got != "medium" {
		t.Errorf("heuristic fallback medium: got %q", got)
	}
}

func TestParsePlan(t *testing.T) {
	fenced := "```json\n{\"analysis\":\"磁盘可能已满\",\"steps\":[{\"command\":\"df -h\",\"purpose\":\"查看磁盘\",\"risk\":\"low\"}],\"done\":false}\n```"
	var plan agentPlan
	if err := parsePlan(fenced, &plan); err != nil {
		t.Fatalf("parsePlan error: %v", err)
	}
	if plan.Analysis != "磁盘可能已满" || len(plan.Steps) != 1 || plan.Steps[0].Command != "df -h" {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if plan.Done {
		t.Fatal("done should be false")
	}

	var done agentPlan
	if err := parsePlan(`{"analysis":"已解决","steps":[],"done":true}`, &done); err != nil {
		t.Fatalf("parse done plan error: %v", err)
	}
	if !done.Done || len(done.Steps) != 0 {
		t.Fatalf("unexpected done plan: %+v", done)
	}

	if err := parsePlan("no json here", &plan); err == nil {
		t.Fatal("expected error for non-json input")
	}
}
