package main

import (
	"strings"
	"testing"
)

func benchmarkPathologicalFind(b *testing.B, n, m int) {
	corpus := make([]string, n)
	for i := 0; i < len(corpus); i++ {
		corpus[i] = strings.Repeat("m", m-2) + "oo"
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		searcher := newSearcher("moo")
		for _, s := range corpus {
			searcher.append(s)
		}
		b.ReportMetric(float64(searcher.batchCount/b.N), "jobs/op")
		searcher.rankedResults(maxResults)
	}
}

func BenchmarkPathologicalFind1000(b *testing.B)    { benchmarkPathologicalFind(b, 1000, 100) }
func BenchmarkPathologicalFind5000(b *testing.B)    { benchmarkPathologicalFind(b, 5000, 100) }
func BenchmarkPathologicalFind10000(b *testing.B)   { benchmarkPathologicalFind(b, 10000, 100) }
func BenchmarkPathologicalFind50000(b *testing.B)   { benchmarkPathologicalFind(b, 50000, 100) }
func BenchmarkPathologicalFind100000(b *testing.B)  { benchmarkPathologicalFind(b, 100000, 100) }
func BenchmarkPathologicalFind500000(b *testing.B)  { benchmarkPathologicalFind(b, 500000, 100) }
func BenchmarkPathologicalFind1000000(b *testing.B) { benchmarkPathologicalFind(b, 1000000, 100) }
