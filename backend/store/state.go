package store

import "sync"

// State holds in-memory workshop progress. Always starts fresh — no replay.
type State struct {
	mu           sync.RWMutex
	meta         *Metadata
	activeStepID string
	completed    map[string]bool // set of completed step IDs
}

// NewState creates fresh state with the first accessible step active.
func NewState(meta *Metadata) *State {
	s := &State{
		meta:      meta,
		completed: make(map[string]bool),
	}
	if len(meta.Steps) > 0 {
		s.activeStepID = meta.Steps[0].ID
	}
	return s
}

// ActiveStepID returns the currently active step.
func (s *State) ActiveStepID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeStepID
}

// SetActiveStep sets the active step (called on navigate).
func (s *State) SetActiveStep(stepID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeStepID = stepID
}

// IsCompleted returns whether a step has been validated successfully.
func (s *State) IsCompleted(stepID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.completed[stepID]
}

// MarkCompleted marks a step as completed and updates accessible steps.
func (s *State) MarkCompleted(stepID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.completed[stepID] = true
}

// CompletedSteps returns all completed step IDs.
func (s *State) CompletedSteps() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, 0, len(s.completed))
	for id := range s.completed {
		result = append(result, id)
	}
	return result
}

// Accessible returns whether a step can be navigated to under the current nav mode.
func (s *State) Accessible(stepID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.accessible(stepID)
}

// accessible is the unlocked internal implementation.
func (s *State) accessible(stepID string) bool {
	nav := s.meta.Workshop.Navigation
	switch nav {
	case "free":
		// All steps always accessible
		_, ok := s.meta.StepsByID[stepID]
		return ok

	case "linear":
		// Only steps up to (and including) the first uncompleted step are accessible.
		// I.e.: step N is accessible if all steps before N are completed.
		for _, step := range s.meta.Steps {
			if step.ID == stepID {
				return true // reached target before finding uncompleted prior step
			}
			if !s.completed[step.ID] {
				return false // blocked by uncompleted prior step
			}
		}
		return false

	case "guided":
		meta, ok := s.meta.StepsByID[stepID]
		if !ok {
			return false
		}
		// Check requires
		for _, req := range meta.Requires {
			if !s.completed[req] {
				return false
			}
		}
		// Check group ordering (groups unlock when all steps in prior group complete)
		// Simple implementation: find all steps in groups that appear before this step's group
		// For now, treat guided like free if no group complexity needed
		// TODO: full group ordering enforcement post-MVP
		return true

	default:
		return false
	}
}
