package reconciliation

import (
	"math"
	"testing"
)

func TestMinCostAssignment_2x2(t *testing.T) {
	// Minimize diagonal (0,0) and (1,1)
	cost := [][]float64{
		{1, 10},
		{10, 1},
	}
	m := minCostAssignment(cost)
	if m[0] != 0 || m[1] != 1 {
		t.Fatalf("assignment %+v", m)
	}
}

func TestHungarianPairsFromRectangular(t *testing.T) {
	left := []int{0, 1}
	right := []int{10, 11}
	w := func(li, ri int) float64 {
		if li == 0 && ri == 11 {
			return 1.0
		}
		if li == 1 && ri == 10 {
			return 1.0
		}
		return 0.1
	}
	pairs := hungarianPairsFromRectangular(left, right, w, -1e9)
	if len(pairs) != 2 {
		t.Fatalf("pairs %v", pairs)
	}
}

func TestMaxWeightBipartiteSquareSkipsDummy(t *testing.T) {
	profit := [][]float64{
		{0.9, 0.1},
		{0.2, 0.8},
	}
	m := maxWeightBipartiteSquare(profit, 2, 2, -1e6)
	if len(m) != 2 {
		t.Fatalf("len %d", len(m))
	}
	sum := profit[0][m[0]] + profit[1][m[1]]
	if math.Abs(sum-1.7) > 1e-6 {
		t.Fatalf("sum %v", sum)
	}
}
