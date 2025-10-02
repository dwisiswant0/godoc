package godoc_test

import (
	"testing"

	"go.dw1.io/godoc"
)

func BenchmarkLoadPackage(b *testing.B) {
	g := godoc.New()
	for b.Loop() {
		result, err := g.Load("fmt", "", "")
		if err != nil {
			b.Fatal(err)
		}
		_ = result
	}
}

func BenchmarkLoadSymbol(b *testing.B) {
	g := godoc.New()
	for b.Loop() {
		result, err := g.Load("fmt", "Printf", "")
		if err != nil {
			b.Fatal(err)
		}
		_ = result
	}
}

func BenchmarkLoadType(b *testing.B) {
	g := godoc.New()
	for b.Loop() {
		result, err := g.Load("fmt", "Stringer", "")
		if err != nil {
			b.Fatal(err)
		}
		_ = result
	}
}

func BenchmarkLoadMethod(b *testing.B) {
	g := godoc.New()
	for b.Loop() {
		result, err := g.Load("net/http", "Request.ParseForm", "")
		if err != nil {
			b.Fatal(err)
		}
		_ = result
	}
}

func BenchmarkLoadPackageNoCache(b *testing.B) {
	for b.Loop() {
		g := godoc.New() // New instance each time, no cache benefit
		result, err := g.Load("fmt", "", "")
		if err != nil {
			b.Fatal(err)
		}
		_ = result
	}
}

func BenchmarkLoadRemotePackage(b *testing.B) {
	g := godoc.New()
	for b.Loop() {
		result, err := g.Load("github.com/stretchr/testify/assert", "", "")
		if err != nil {
			b.Skip("Remote load failed:", err)
		}
		_ = result
	}
}

func BenchmarkLoadAndMarshal(b *testing.B) {
	g := godoc.New()
	for b.Loop() {
		result, err := g.Load("fmt", "", "")
		if err != nil {
			b.Fatal(err)
		}
		_ = result.Text()
		_ = result.HTML()
	}
}

func BenchmarkResultText(b *testing.B) {
	g := godoc.New()

	result, err := g.Load("fmt", "", "")
	if err != nil {
		b.Fatal(err)
	}

	for b.Loop() {
		_ = result.Text()
	}
}

func BenchmarkResultHTML(b *testing.B) {
	g := godoc.New()

	result, err := g.Load("fmt", "", "")
	if err != nil {
		b.Fatal(err)
	}

	for b.Loop() {
		_ = result.HTML()
	}
}

func BenchmarkConcurrentLoad(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		g := godoc.New()
		for pb.Next() {
			result, err := g.Load("fmt", "", "")
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})
}
