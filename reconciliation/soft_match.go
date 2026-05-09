package reconciliation

import (
	"sort"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/enricher"
)

type scoredPair struct {
	li, ri int
	score  float64
	card   MatchScorecard
}

// runSoftIndividualMatching fills alignments for unmatched individuals using stages 3–5.
// matchedLeft/matchedRight use indiKey; alignIndividual must mark those keys.
// Returns hint scorecards per still-unmatched right individual key (indiKey).
func runSoftIndividualMatching(
	left, right *enricher.EnrichedDocument,
	opt *Options,
	plan *MergePlan,
	matchedLeft, matchedRight map[string]bool,
	alignIndividual func(l, r *enricher.EnrichedIndividual, sc MatchScorecard, conf AlignmentConfidence),
) map[string][]MatchScorecard {
	profile := coalesceProfile(opt)
	alignMin, hintMin, maxCmp := 0.74, 0.55, 40
	useHungarian := false
	hMax := 64
	if opt != nil {
		alignMin = opt.softMinAlignScore()
		hintMin = opt.softMinHintScore()
		maxCmp = opt.maxSoftComparisonsPerSide()
		useHungarian = opt.UseHungarianAssignment
		hMax = opt.hungarianMaxMatrix()
	}

	var leftIdx, rightIdx []int
	for i := range left.Individuals {
		if matchedLeft[indiKey(&left.Individuals[i])] {
			continue
		}
		leftIdx = append(leftIdx, i)
	}
	for i := range right.Individuals {
		if matchedRight[indiKey(&right.Individuals[i])] {
			continue
		}
		rightIdx = append(rightIdx, i)
	}
	if len(leftIdx) == 0 || len(rightIdx) == 0 {
		return nil
	}

	rightBuckets := make(map[string][]int)
	for _, i := range rightIdx {
		k := softBlockingKey(right, &right.Individuals[i])
		rightBuckets[k] = append(rightBuckets[k], i)
	}
	leftBuckets := make(map[string][]int)
	for _, i := range leftIdx {
		k := softBlockingKey(left, &left.Individuals[i])
		leftBuckets[k] = append(leftBuckets[k], i)
	}

	var pairs []scoredPair
	seen := make(map[[2]int]struct{})
	addScores := func(li, ri int) {
		if _, ok := seen[[2]int{li, ri}]; ok {
			return
		}
		seen[[2]int{li, ri}] = struct{}{}
		l := &left.Individuals[li]
		r := &right.Individuals[ri]
		card := scoreSoftIndividualPair(left, right, l, r, profile)
		pairs = append(pairs, scoredPair{li: li, ri: ri, score: card.Score, card: card})
	}

	if useHungarian {
		var keys []string
		for k := range leftBuckets {
			if len(rightBuckets[k]) > 0 {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		dummyProfit := -1e6
		for _, bk := range keys {
			L := append([]int(nil), leftBuckets[bk]...)
			R := append([]int(nil), rightBuckets[bk]...)
			if len(L) == 0 || len(R) == 0 {
				continue
			}
			// Cap for O(n³); deterministic trim by index order
			if len(L) > hMax {
				L = L[:hMax]
			}
			if len(R) > hMax {
				R = R[:hMax]
			}
			sort.Ints(L)
			sort.Ints(R)
			hPairs := hungarianPairsFromRectangular(L, R, func(li, ri int) float64 {
				l := &left.Individuals[li]
				r := &right.Individuals[ri]
				return scoreSoftIndividualPair(left, right, l, r, profile).Score
			}, dummyProfit)
			for _, pr := range hPairs {
				addScores(pr[0], pr[1])
			}
		}
		// Cross-bucket fallback: same as legacy capped search for pairs not yet seen
		for _, li := range leftIdx {
			bk := softBlockingKey(left, &left.Individuals[li])
			candidates := append([]int(nil), rightBuckets[bk]...)
			if len(candidates) < maxCmp {
				for _, ri := range rightIdx {
					if len(candidates) >= maxCmp {
						break
					}
					dup := false
					for _, c := range candidates {
						if c == ri {
							dup = true
							break
						}
					}
					if !dup {
						candidates = append(candidates, ri)
					}
				}
			}
			if len(candidates) > maxCmp {
				candidates = candidates[:maxCmp]
			}
			for _, ri := range candidates {
				addScores(li, ri)
			}
		}
	} else {
		for _, li := range leftIdx {
			bk := softBlockingKey(left, &left.Individuals[li])
			candidates := append([]int(nil), rightBuckets[bk]...)
			if len(candidates) < maxCmp {
				for _, ri := range rightIdx {
					if len(candidates) >= maxCmp {
						break
					}
					dup := false
					for _, c := range candidates {
						if c == ri {
							dup = true
							break
						}
					}
					if !dup {
						candidates = append(candidates, ri)
					}
				}
			}
			if len(candidates) > maxCmp {
				candidates = candidates[:maxCmp]
			}
			for _, ri := range candidates {
				if matchedRight[indiKey(&right.Individuals[ri])] {
					continue
				}
				addScores(li, ri)
			}
		}
	}

	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].score != pairs[j].score {
			return pairs[i].score > pairs[j].score
		}
		if pairs[i].li != pairs[j].li {
			return pairs[i].li < pairs[j].li
		}
		return pairs[i].ri < pairs[j].ri
	})

	for _, p := range pairs {
		l := &left.Individuals[p.li]
		r := &right.Individuals[p.ri]
		if matchedLeft[indiKey(l)] || matchedRight[indiKey(r)] {
			continue
		}
		if p.score < alignMin {
			continue
		}
		conf := ConfidenceMedium
		if p.card.Stage == StageNameDate {
			conf = ConfidenceHigh
		}
		if p.card.Stage == StageFuzzy {
			conf = ConfidenceLow
		}
		alignIndividual(l, r, p.card, conf)
	}

	hintsMap := make(map[string][]MatchScorecard)
	for _, ri := range rightIdx {
		r := &right.Individuals[ri]
		if matchedRight[indiKey(r)] {
			continue
		}
		var hints []MatchScorecard
		for _, li := range leftIdx {
			l := &left.Individuals[li]
			if matchedLeft[indiKey(l)] {
				continue
			}
			card := scoreSoftIndividualPair(left, right, l, r, profile)
			if card.Score >= hintMin && card.Score < alignMin {
				hints = append(hints, card)
			}
		}
		sort.Slice(hints, func(i, j int) bool { return hints[i].Score > hints[j].Score })
		if len(hints) > 3 {
			hints = hints[:3]
		}
		if len(hints) > 0 {
			hintsMap[indiKey(r)] = hints
		}
	}
	return hintsMap
}
