package xcurve

import (
	"crypto/rand"
	"math/big"
	mrand "math/rand"
	"testing"

	"github.com/cloudflare/circl/internal/conv"
	"github.com/cloudflare/circl/internal/test"
	fp255 "github.com/cloudflare/circl/math/fp25519"
	"github.com/cloudflare/circl/math/fp448"
)

// Montgomery point doubling in projective (X:Z) coordintates.
func doubleBig(x1, z1, A24, p *big.Int) {
	A, B, C := big.NewInt(0), big.NewInt(0), big.NewInt(0)
	A.Add(x1, z1).Mod(A, p)
	B.Sub(x1, z1).Mod(B, p)
	A.Mul(A, A)
	B.Mul(B, B)
	C.Sub(A, B)
	x1.Mul(A, B).Mod(x1, p)
	z1.Mul(C, A24).Add(z1, B).Mul(z1, C).Mod(z1, p)
}

// Equation 7 at https://eprint.iacr.org/2017/264
func diffAddBig(work [5]*big.Int, p *big.Int, b uint) {
	mu, x1, z1, x2, z2 := work[0], work[1], work[2], work[3], work[4]
	A, B := big.NewInt(0), big.NewInt(0)
	if b != 0 {
		t := new(big.Int)
		t.Set(x1)
		x1.Set(x2)
		x2.Set(t)
		t.Set(z1)
		z1.Set(z2)
		z2.Set(t)
	}
	A.Add(x1, z1)
	B.Sub(x1, z1)
	B.Mul(B, mu).Mod(B, p)
	x1.Add(A, B).Mod(x1, p)
	z1.Sub(A, B).Mod(z1, p)
	x1.Mul(x1, x1).Mul(x1, z2).Mod(x1, p)
	z1.Mul(z1, z1).Mul(z1, x2).Mod(z1, p)
	mu.Mod(mu, p)
	x2.Mod(x2, p)
	z2.Mod(z2, p)
}

func ladderStepBig(work [5]*big.Int, A24, p *big.Int, b uint) {
	x1 := work[0]
	x2, z2 := work[1], work[2]
	x3, z3 := work[3], work[4]
	A, B, C, D := big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)
	DA, CB, E := big.NewInt(0), big.NewInt(0), big.NewInt(0)
	A.Add(x2, z2).Mod(A, p)
	B.Sub(x2, z2).Mod(B, p)
	C.Add(x3, z3).Mod(C, p)
	D.Sub(x3, z3).Mod(D, p)
	DA.Mul(D, A).Mod(DA, p)
	CB.Mul(C, B).Mod(CB, p)
	if b != 0 {
		t := new(big.Int)
		t.Set(A)
		A.Set(C)
		C.Set(t)
		t.Set(B)
		B.Set(D)
		D.Set(t)
	}
	AA := A.Mul(A, A).Mod(A, p)
	BB := B.Mul(B, B).Mod(B, p)
	E.Sub(AA, BB)
	x1.Mod(x1, p)
	x2.Mul(AA, BB).Mod(x2, p)
	z2.Mul(E, A24).Add(z2, BB).Mul(z2, E).Mod(z2, p)
	x3.Add(DA, CB)
	z3.Sub(DA, CB)
	x3.Mul(x3, x3).Mod(x3, p)
	z3.Mul(z3, z3).Mul(z3, x1).Mod(z3, p)
}

func TestCurve255(t *testing.T) {
	numTests := 1 << 9
	p := big.NewInt(1)
	p.Lsh(p, 255).Sub(p, big.NewInt(19))
	A24 := big.NewInt(121666)

	t.Run("mulA24", func(t *testing.T) {
		var x, z fp255.Elt
		for i := 0; i < numTests; i++ {
			_, _ = rand.Read(x[:])
			bigX := conv.BytesLe2BigInt(x[:])
			c255.mulA24(&z, &x)
			got := conv.BytesLe2BigInt(z[:])
			got.Mod(got, p)

			want := bigX.Mul(bigX, A24).Mod(bigX, p)

			if got.Cmp(want) != 0 {
				test.ReportError(t, got, want, x)
			}
		}
	})

	t.Run("double", func(t *testing.T) {
		var x, z fp255.Elt
		for i := 0; i < numTests; i++ {
			_, _ = rand.Read(x[:])
			_, _ = rand.Read(z[:])

			bigX := conv.BytesLe2BigInt(x[:])
			bigZ := conv.BytesLe2BigInt(z[:])
			c255.double(&x, &z)
			got0 := conv.BytesLe2BigInt(x[:])
			got1 := conv.BytesLe2BigInt(z[:])
			got0.Mod(got0, p)
			got1.Mod(got1, p)

			doubleBig(bigX, bigZ, A24, p)
			want0 := bigX
			want1 := bigZ

			if got0.Cmp(want0) != 0 {
				test.ReportError(t, got0, want0, x, z)
			}
			if got1.Cmp(want1) != 0 {
				test.ReportError(t, got1, want1, x, z)
			}
		}
	})

	t.Run("diffAdd", func(t *testing.T) {
		var w [5]fp255.Elt
		bigWork := [5]*big.Int{}
		for i := 0; i < numTests; i++ {
			for j := range w {
				_, _ = rand.Read(w[j][:])
				bigWork[j] = conv.BytesLe2BigInt(w[j][:])
			}
			b := uint(w[0][0] & 1)

			c255.difAdd(&w, b)

			diffAddBig(bigWork, p, b)

			for j := range w {
				got := conv.BytesLe2BigInt(w[j][:])
				got.Mod(got, p)
				want := bigWork[j]
				if got.Cmp(want) != 0 {
					test.ReportError(t, got, want, w, b)
				}
			}
		}
	})

	t.Run("ladderStep", func(t *testing.T) {
		var w [5]fp255.Elt
		bigWork := [5]*big.Int{}
		for i := 0; i < numTests; i++ {
			for j := range w {
				_, _ = mrand.Read(w[j][:])
				bigWork[j] = conv.BytesLe2BigInt(w[j][:])
			}
			b := uint(w[0][0] & 1)

			c255.ladderStep(&w, b)

			ladderStepBig(bigWork, A24, p, b)

			for j := range bigWork {
				got := conv.BytesLe2BigInt(w[j][:])
				got.Mod(got, p)
				want := bigWork[j]
				if got.Cmp(want) != 0 {
					test.ReportError(t, got, want, w, b)
				}
			}
		}
	})
}

func TestCurve448(t *testing.T) {
	numTests := 1 << 9
	p := big.NewInt(1)
	p.Lsh(p, 224)
	p.Sub(p, new(big.Int).SetInt64(1))
	p.Lsh(p, 224)
	p.Sub(p, new(big.Int).SetInt64(1))
	A24 := big.NewInt(39082)

	t.Run("mulA24", func(t *testing.T) {
		var x, z fp448.Elt
		for i := 0; i < numTests; i++ {
			_, _ = rand.Read(x[:])
			bigX := conv.BytesLe2BigInt(x[:])
			c448.mulA24(&z, &x)
			got := conv.BytesLe2BigInt(z[:])
			got.Mod(got, p)

			want := bigX.Mul(bigX, A24).Mod(bigX, p)

			if got.Cmp(want) != 0 {
				test.ReportError(t, got, want, x)
			}
		}
	})

	t.Run("double", func(t *testing.T) {
		var x, z fp448.Elt
		for i := 0; i < numTests; i++ {
			_, _ = rand.Read(x[:])
			_, _ = rand.Read(z[:])

			bigX := conv.BytesLe2BigInt(x[:])
			bigZ := conv.BytesLe2BigInt(z[:])
			c448.double(&x, &z)
			got0 := conv.BytesLe2BigInt(x[:])
			got1 := conv.BytesLe2BigInt(z[:])
			got0.Mod(got0, p)
			got1.Mod(got1, p)

			doubleBig(bigX, bigZ, A24, p)
			want0 := bigX
			want1 := bigZ

			if got0.Cmp(want0) != 0 {
				test.ReportError(t, got0, want0, x, z)
			}
			if got1.Cmp(want1) != 0 {
				test.ReportError(t, got1, want1, x, z)
			}
		}
	})

	t.Run("diffAdd", func(t *testing.T) {
		var w [5]fp448.Elt
		bigWork := [5]*big.Int{}
		for i := 0; i < numTests; i++ {
			for j := range w {
				_, _ = rand.Read(w[j][:])
				bigWork[j] = conv.BytesLe2BigInt(w[j][:])
			}
			b := uint(w[0][0] & 1)

			c448.difAdd(&w, b)

			diffAddBig(bigWork, p, b)

			for j := range bigWork {
				got := conv.BytesLe2BigInt(w[j][:])
				got.Mod(got, p)
				want := bigWork[j]
				if got.Cmp(want) != 0 {
					test.ReportError(t, got, want, w, b)
				}
			}
		}
	})

	t.Run("ladderStep", func(t *testing.T) {
		var w [5]fp448.Elt
		bigWork := [5]*big.Int{}
		for i := 0; i < numTests; i++ {
			for j := range w {
				_, _ = rand.Read(w[j][:])
				bigWork[j] = conv.BytesLe2BigInt(w[j][:])
			}
			b := uint(w[0][0] & 1)

			c448.ladderStep(&w, b)

			ladderStepBig(bigWork, A24, p, b)

			for j := range bigWork {
				got := conv.BytesLe2BigInt(w[j][:])
				got.Mod(got, p)
				want := bigWork[j]
				if got.Cmp(want) != 0 {
					test.ReportError(t, got, want, w, b)
				}
			}
		}
	})
}
