package reconciliation

import (
	"math"
)

// minCostAssignment solves the square assignment problem (minimize sum of chosen costs).
// cost must be n×n. Returns matchColForRow[i] = assigned column index for row i.
// Implemented via min-cost max-flow (successive shortest augmenting path with Bellman–Ford),
// suitable for n up to a few hundred (soft-match buckets are capped much smaller).
func minCostAssignment(cost [][]float64) []int {
	n := len(cost)
	if n == 0 {
		return nil
	}
	for _, row := range cost {
		if len(row) != n {
			return nil
		}
	}
	flow := newMinCostFlowSquare(cost)
	match := flow.solve()
	out := make([]int, n)
	for i := range out {
		out[i] = -1
	}
	for i := 0; i < n; i++ {
		j := match[i]
		if j >= 0 && j < n {
			out[i] = j
		}
	}
	return out
}

type mcfEdge struct {
	to   int
	rev  int
	cap  int
	cost float64
}

type minCostFlowSquare struct {
	n int
	g [][]mcfEdge
}

func newMinCostFlowSquare(cost [][]float64) *minCostFlowSquare {
	n := len(cost)
	// Vertices: S=0, rows 1..n, cols n+1..2n, T=2*n+1
	V := 2*n + 2
	S := 0
	T := V - 1
	g := make([][]mcfEdge, V)
	add := func(from, to, cap int, c float64) {
		fwd := len(g[to])
		rev := len(g[from])
		g[from] = append(g[from], mcfEdge{to, fwd, cap, c})
		g[to] = append(g[to], mcfEdge{from, rev, 0, -c})
	}
	for i := 0; i < n; i++ {
		add(S, 1+i, 1, 0)
	}
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			add(1+i, 1+n+j, 1, cost[i][j])
		}
	}
	for j := 0; j < n; j++ {
		add(1+n+j, T, 1, 0)
	}
	return &minCostFlowSquare{n: n, g: g}
}

func (m *minCostFlowSquare) solve() []int {
	n := m.n
	V := len(m.g)
	S := 0
	T := V - 1
	for k := 0; k < n; k++ {
		dist := make([]float64, V)
		prevV := make([]int, V)
		prevE := make([]int, V)
		for i := range dist {
			dist[i] = math.Inf(1)
			prevV[i] = -1
		}
		dist[S] = 0
		// Bellman–Ford (V−1 rounds) on residual; costs can go negative on reverse edges.
		for iter := 0; iter < V-1; iter++ {
			for v := 0; v < V; v++ {
				if math.IsInf(dist[v], 1) {
					continue
				}
				for ei, e := range m.g[v] {
					if e.cap == 0 {
						continue
					}
					if dist[e.to] > dist[v]+e.cost+1e-15 {
						dist[e.to] = dist[v] + e.cost
						prevV[e.to] = v
						prevE[e.to] = ei
					}
				}
			}
		}
		if math.IsInf(dist[T], 1) {
			break
		}
		// Augment one unit S->T
		for v := T; v != S; v = prevV[v] {
			pv := prevV[v]
			pe := prevE[v]
			edge := &m.g[pv][pe]
			edge.cap--
			rev := &m.g[edge.to][edge.rev]
			rev.cap++
		}
	}
	// Read matching from row edges with zero residual on forward row->col edges.
	match := make([]int, n)
	for i := range match {
		match[i] = -1
	}
	for i := 0; i < n; i++ {
		rowV := 1 + i
		for _, e := range m.g[rowV] {
			if e.to >= 1+n && e.to <= 2*n {
				rev := m.g[e.to][e.rev]
				if rev.cap == 1 { // flow pushed on forward
					match[i] = e.to - (1 + n)
					break
				}
			}
		}
	}
	return match
}

// maxWeightBipartiteSquare pads a nL×nR profit matrix to square n×n with dummyProfit (very low)
// so dummy assignments are avoided when real edges exist.
func maxWeightBipartiteSquare(profit [][]float64, nL, nR int, dummyProfit float64) []int {
	if nL == 0 || nR == 0 {
		return nil
	}
	n := nL
	if nR > n {
		n = nR
	}
	maxP := dummyProfit
	for i := 0; i < nL; i++ {
		for j := 0; j < nR; j++ {
			if profit[i][j] > maxP {
				maxP = profit[i][j]
			}
		}
	}
	// cost = maxP - profit; higher profit => lower cost. Dummy cells get very low profit.
	cost := make([][]float64, n)
	for i := range cost {
		cost[i] = make([]float64, n)
		for j := range cost[i] {
			var p float64
			if i < nL && j < nR {
				p = profit[i][j]
			} else {
				p = dummyProfit
			}
			cost[i][j] = maxP - p
		}
	}
	return minCostAssignment(cost)
}

// hungarianPairsFromRectangular runs Hungarian on leftIdx × rightIdx using weight(li, ri int) float64.
// Skips pairs with weight <= dummyProfit. Returns (leftSliceIndex, rightSliceIndex) pairs.
func hungarianPairsFromRectangular(
	leftIdx []int,
	rightIdx []int,
	weight func(li, ri int) float64,
	dummyProfit float64,
) [][2]int {
	nL := len(leftIdx)
	nR := len(rightIdx)
	if nL == 0 || nR == 0 {
		return nil
	}
	profit := make([][]float64, nL)
	for i := 0; i < nL; i++ {
		profit[i] = make([]float64, nR)
		for j := 0; j < nR; j++ {
			profit[i][j] = weight(leftIdx[i], rightIdx[j])
		}
	}
	match := maxWeightBipartiteSquare(profit, nL, nR, dummyProfit)
	var out [][2]int
	for i := 0; i < nL; i++ {
		j := match[i]
		if j < 0 || j >= nR {
			continue
		}
		w := profit[i][j]
		if w <= dummyProfit+1e-12 {
			continue
		}
		out = append(out, [2]int{leftIdx[i], rightIdx[j]})
	}
	return out
}
