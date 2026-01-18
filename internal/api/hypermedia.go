package api

// LinksFor generates HAL-style links based on simulation state.
// Pure function: state -> links (domain layer, unit testable).
func LinksFor(state SimulationState) map[string]string {
	// sprintIsActive returns true if the simulation has an active sprint.
	sprintIsActive := func(s SimulationState) bool {
		_, ok := s.CurrentSprintOption.Get()
		return ok
	}

	links := map[string]string{
		"self": "/simulations/" + state.ID,
	}

	// Assign link available whenever backlog has tickets (UC11: sprint planning)
	if state.BacklogCount > 0 {
		links["assign"] = "/simulations/" + state.ID + "/assignments"
	}

	if sprintIsActive(state) {
		links["tick"] = "/simulations/" + state.ID + "/tick"
	} else {
		links["start-sprint"] = "/simulations/" + state.ID + "/sprints"
	}

	return links
}
