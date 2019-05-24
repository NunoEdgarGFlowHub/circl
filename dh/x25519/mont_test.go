package x25519

import (
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/cloudflare/circl/internal/conv"
	"github.com/cloudflare/circl/internal/test"
	fp "github.com/cloudflare/circl/math/fp25519"
)

func TestMul24(t *testing.T) {
	var x, z fp.Elt
	numTests := 1 << 10
	A24 := big.NewInt(a24)
	prime := fp.P()
	p := conv.BytesLe2BigInt(prime[:])

	for i := 0; i < numTests; i++ {
		_, _ = rand.Read(x[:])
		mulA24(&z, &x)
		fp.Modp(&z)
		got := conv.BytesLe2BigInt(z[:])

		xx := conv.BytesLe2BigInt(x[:])
		want := xx.Mul(xx, A24).Mod(xx, p)

		if got.Cmp(want) != 0 {
			test.ReportError(t, got, want, x)
		}
	}
}

// Montgomery point doubling in projective (X:Z) coordintates.
func doubleBig(work [4]*big.Int, A24, p *big.Int) {
	x1, z1 := work[0], work[1]
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
func diffAddBig(work [4]*big.Int, mu, p *big.Int, b uint) {
	x1, z1, x2, z2 := work[0], work[1], work[2], work[3]
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

func TestCurve(t *testing.T) {
	numTests := 1 << 9
	var work [4]fp.Elt
	var bigWork [4]*big.Int

	A24 := big.NewInt(a24)
	prime := fp.P()
	p := conv.BytesLe2BigInt(prime[:])

	t.Run("double", func(t *testing.T) {
		for i := 0; i < numTests; i++ {
			for j := range work {
				_, _ = rand.Read(work[j][:])
				bigWork[j] = conv.BytesLe2BigInt(work[j][:])
			}

			double(&work)
			fp.Modp(&work[0])
			fp.Modp(&work[1])
			got0 := conv.BytesLe2BigInt(work[0][:])
			got1 := conv.BytesLe2BigInt(work[1][:])

			doubleBig(bigWork, A24, p)
			want0 := bigWork[0]
			want1 := bigWork[1]

			if got0.Cmp(want0) != 0 {
				test.ReportError(t, got0, want0, work)
			}
			if got1.Cmp(want1) != 0 {
				test.ReportError(t, got1, want1, work)
			}
		}
	})

	t.Run("diffAdd", func(t *testing.T) {
		var mu fp.Elt
		for i := 0; i < numTests; i++ {
			for j := range work {
				_, _ = rand.Read(work[j][:])
				bigWork[j] = conv.BytesLe2BigInt(work[j][:])
			}
			_, _ = rand.Read(mu[:])
			bigMu := conv.BytesLe2BigInt(mu[:])
			b := uint(mu[0] & 1)

			difAdd(&work, &mu, b)

			diffAddBig(bigWork, bigMu, p, b)

			for j := range work {
				fp.Modp(&work[j])
				got := conv.BytesLe2BigInt(work[j][:])
				want := bigWork[j]
				if got.Cmp(want) != 0 {
					test.ReportError(t, got, want, work, mu, b)
				}
			}
		}
	})

	t.Run("ladder", func(t *testing.T) {
		var workLadder [5]fp.Elt
		var bigWorkLadder [5]*big.Int
		for i := 0; i < numTests; i++ {
			for j := range workLadder {
				_, _ = rand.Read(workLadder[j][:])
				bigWorkLadder[j] = conv.BytesLe2BigInt(workLadder[j][:])
			}
			b := uint(workLadder[0][0] & 1)

			ladderStep(&workLadder, b)

			ladderStepBig(bigWorkLadder, A24, p, b)

			for j := range workLadder {
				fp.Modp(&workLadder[j])
				got := conv.BytesLe2BigInt(workLadder[j][:])
				want := bigWorkLadder[j]
				if got.Cmp(want) != 0 {
					test.ReportError(t, got, want, workLadder, b)
				}
			}
		}
	})
}
