package service

import (
	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/pick"
)

// UserProfile chooses default use cases without changing their scoring weights.
type UserProfile struct {
	Slug           string   `json:"slug"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	UseCaseSlugs   []string `json:"use_case_slugs"`
	DefaultUseCase string   `json:"default_use_case"`
}

// UserProfiles returns fresh values so callers cannot mutate the built-in sets.
func (p *ProfileService) UserProfiles() []UserProfile {
	return []UserProfile{
		{Slug: "software_engineering", Name: "Software Engineering", Description: "Build, review and maintain software.", DefaultUseCase: "simple_implementation", UseCaseSlugs: []string{"simple_implementation", "balanced_implementation", "complex_implementation", "review", "ui_ux", "planning", "orchestration", "research"}},
		{Slug: "marketing", Name: "Marketing", Description: "Create content, research audiences and plan campaigns.", DefaultUseCase: "content_drafting", UseCaseSlugs: []string{"content_drafting", "content_editing", "market_research", "campaign_planning", "marketing_analysis"}},
		{Slug: "general", Name: "General", Description: "Research, writing, planning and everyday tasks.", DefaultUseCase: "research", UseCaseSlugs: []string{"research", "content_drafting", "content_editing", "planning", "simple_action_execution", "complex_action_execution", "financial_work"}},
	}
}

var useCaseDescriptions = map[string]string{
	"simple_implementation":    "Small code changes where cost and speed matter most.",
	"balanced_implementation":  "Everyday feature work with balanced quality, cost and speed.",
	"complex_implementation":   "Difficult implementation requiring reasoning and planning.",
	"review":                   "Review code for correctness, instructions and security concerns.",
	"ui_ux":                    "Implement and evaluate interfaces using code and visual evidence.",
	"planning":                 "Break down complex work and reason about dependencies.",
	"orchestration":            "Coordinate tasks and tools with attention to cost and latency.",
	"research":                 "Find information, combine evidence and answer research questions.",
	"simple_action_execution":  "Carry out straightforward tasks with tools.",
	"complex_action_execution": "Carry out tasks with multiple tool interactions.",
	"financial_work":           "Analyse financial information and supporting evidence.",
	"content_drafting":         "Draft campaign copy, emails and social content from a brief.",
	"content_editing":          "Refine tone, structure and adherence to a content brief.",
	"market_research":          "Research audiences, competitors and market context.",
	"campaign_planning":        "Develop campaign plans, messaging and channel priorities.",
	"marketing_analysis":       "Interpret campaign data, spreadsheets and performance trends.",
}

func useCaseEvidenceNote(slug string) string {
	switch slug {
	case "content_drafting", "content_editing", "market_research", "campaign_planning", "marketing_analysis":
		return "Uses general capability benchmarks; marketing outcomes and brand voice are not directly measured."
	default:
		return ""
	}
}

// Existing custom use cases keep precedence over newly introduced presets.
// Legacy built-ins retain their historical read-only/precedence behavior.
func builtinUseCase(slug string, customs config.ProfilesTOML) bool {
	if _, ok := pick.Profiles[slug]; !ok {
		return false
	}
	if _, custom := customs[slug]; custom {
		switch slug {
		case "content_drafting", "content_editing", "market_research", "campaign_planning", "marketing_analysis":
			return false
		}
	}
	return true
}

func isNewUseCase(slug string) bool {
	switch slug {
	case "content_drafting", "content_editing", "market_research", "campaign_planning", "marketing_analysis":
		return true
	default:
		return false
	}
}
