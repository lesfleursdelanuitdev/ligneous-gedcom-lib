package enricher

// extractRepositories processes top-level REPO records and builds
// source-repository junction links.
func (e *enricherState) extractRepositories(ed *EnrichedDocument) {
	// Phase 1: Top-level repositories
	for _, repoRec := range e.doc.Repositories {
		if repoRec.Xref == "" {
			continue
		}

		address := repoRec.ChildValue("ADDR")
		city := ""
		state := ""
		country := ""
		if addrRec := repoRec.FirstChildByTag("ADDR"); addrRec != nil {
			city = addrRec.ChildValue("CITY")
			state = addrRec.ChildValue("STAE")
			country = addrRec.ChildValue("CTRY")
		}

		idx := len(ed.Repositories)
		ed.Repositories = append(ed.Repositories, EnrichedRepository{
			Xref:    repoRec.Xref,
			Name:    repoRec.ChildValue("NAME"),
			Address: address,
			City:    city,
			State:   state,
			Country: country,
			Phone:   repoRec.ChildValue("PHON"),
			Email:   repoRec.ChildValue("EMAIL"),
			Website: repoRec.ChildValue("WWW"),
		})
		e.repoXrefIndex[repoRec.Xref] = idx
	}

	// Phase 2: Build source-repository junctions
	for srcIdx, src := range ed.Sources {
		if src.RepositoryXref == "" {
			continue
		}
		repoIdx, ok := e.repoXrefIndex[src.RepositoryXref]
		if !ok {
			continue
		}
		ed.SourceRepositories = append(ed.SourceRepositories, SourceRepositoryLink{
			SourceIndex:     srcIdx,
			RepositoryIndex: repoIdx,
			CallNumber:      src.CallNumber,
		})
	}
}
