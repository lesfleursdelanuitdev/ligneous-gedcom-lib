package reconciliation

// DefaultOptions returns API-aligned defaults (soft matching on, Hungarian off).
func DefaultOptions() *Options {
	return &Options{
		EnableSoftIndividualMatching: true,
		SoftMinAlignScore:            0.74,
		SoftMinHintScore:             0.55,
		MaxSoftComparisonsPerSide:    40,
		UseHungarianAssignment:       false,
		HungarianMaxMatrix:           64,
	}
}
