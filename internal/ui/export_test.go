package ui

// StateForTest returns a copy of the model's shared state.
func (m *Model) StateForTest() State { return m.state }
