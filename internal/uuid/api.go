package uuid

type Provider interface {
	Name() string
	New() [16]byte
	NewV7() [16]byte
	Parse(s string) ([16]byte, error)
	String(b [16]byte) string
	InsertValue(b [16]byte) any
	ScanValue(b *[16]byte) any
}

var All = []Provider{Google{}, Stdlib{}}

func Pairs() [][2]Provider {
	return [][2]Provider{
		{Google{}, Google{}},
		{Stdlib{}, Stdlib{}},
		{Google{}, Stdlib{}},
		{Stdlib{}, Google{}},
	}
}

func PairName(p, c Provider) string {
	return p.Name() + " -> " + c.Name()
}
