package huly

import "strings"

// Class and id constants (Huly uses package:type:Name refs).
const (
	ClassIssue       = "tracker:class:Issue"
	ClassProject     = "tracker:class:Project"
	ClassMilestone   = "tracker:class:Milestone"
	ClassComponent   = "tracker:class:Component"
	ClassIssueStatus = "tracker:class:IssueStatus"

	TxCreateDoc = "core:class:TxCreateDoc"
	TxUpdateDoc = "core:class:TxUpdateDoc"
	TxRemoveDoc = "core:class:TxRemoveDoc"

	SpaceTx    = "core:space:Tx"
	IDNoParent = "tracker:ids:NoParent"

	CollectionSubIssues = "subIssues"
)

// Priority mirrors tracker IssuePriority enum order.
type Priority int

const (
	NoPriority Priority = iota
	Urgent
	High
	Medium
	Low
)

var priorityNames = []string{"NoPriority", "Urgent", "High", "Medium", "Low"}

func (p Priority) String() string {
	if int(p) < 0 || int(p) >= len(priorityNames) {
		return "NoPriority"
	}
	return priorityNames[p]
}

// PriorityFromName resolves a case-insensitive name to a Priority.
func PriorityFromName(name string) (Priority, bool) {
	for i, n := range priorityNames {
		if strings.EqualFold(n, name) {
			return Priority(i), true
		}
	}
	return NoPriority, false
}

// Project is a tracker project (a Space).
type Project struct {
	ID         string `json:"_id"`
	Class      string `json:"_class"`
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
	Sequence   int    `json:"sequence"`
}

// Issue is a tracker issue (an AttachedDoc).
type Issue struct {
	ID              string `json:"_id"`
	Class           string `json:"_class"`
	Space           string `json:"space"`
	Title           string `json:"title"`
	Description     string `json:"description,omitempty"`
	Status          string `json:"status,omitempty"`
	Priority        int    `json:"priority"`
	Assignee        string `json:"assignee,omitempty"`
	Component       string `json:"component,omitempty"`
	Milestone       string `json:"milestone,omitempty"`
	Number          int    `json:"number,omitempty"`
	Identifier      string `json:"identifier,omitempty"`
	AttachedTo      string `json:"attachedTo,omitempty"`
	AttachedToClass string `json:"attachedToClass,omitempty"`
	Collection      string `json:"collection,omitempty"`
}

// Milestone is a tracker milestone.
type Milestone struct {
	ID         string `json:"_id"`
	Class      string `json:"_class"`
	Space      string `json:"space"`
	Label      string `json:"label"`
	Status     int    `json:"status"`
	TargetDate int64  `json:"targetDate,omitempty"`
}

// Component is a tracker component.
type Component struct {
	ID          string `json:"_id"`
	Class       string `json:"_class"`
	Space       string `json:"space"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Lead        string `json:"lead,omitempty"`
}

// IssueStatus is a per-project workflow state.
type IssueStatus struct {
	ID       string `json:"_id"`
	Space    string `json:"space"`
	Name     string `json:"name"`
	Category string `json:"category,omitempty"`
}
