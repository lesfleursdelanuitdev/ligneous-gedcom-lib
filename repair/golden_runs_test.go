package repair

type goldenRun struct {
	name string
	f    func([]byte) (*Result, error)
}

// goldenRuns are the calls that must reproduce v0.2.1's Repair exactly.
func goldenRuns() []goldenRun {
	return []goldenRun{{"Repair", Repair}}
}
