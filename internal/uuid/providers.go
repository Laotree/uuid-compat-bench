package uuid

import (
	guuid "github.com/google/uuid"
	"uuid"
)

type Google struct{}

func (Google) Name() string { return "google" }

func (Google) New() [16]byte {
	u, err := guuid.NewRandom()
	if err != nil {
		panic(err)
	}
	return u
}

func (Google) NewV7() [16]byte {
	u, err := guuid.NewV7()
	if err != nil {
		panic(err)
	}
	return u
}

func (Google) Parse(s string) ([16]byte, error) {
	u, err := guuid.Parse(s)
	return u, err
}

func (Google) String(b [16]byte) string {
	return guuid.UUID(b).String()
}

func (Google) InsertValue(b [16]byte) any {
	return guuid.UUID(b)
}

func (Google) ScanValue(b *[16]byte) any {
	return (*guuid.UUID)(b)
}

type Stdlib struct{}

func (Stdlib) Name() string { return "stdlib" }

func (Stdlib) New() [16]byte {
	return uuid.New()
}

func (Stdlib) NewV7() [16]byte {
	return uuid.NewV7()
}

func (Stdlib) Parse(s string) ([16]byte, error) {
	u, err := uuid.Parse(s)
	return u, err
}

func (Stdlib) String(b [16]byte) string {
	return uuid.UUID(b).String()
}

func (Stdlib) InsertValue(b [16]byte) any {
	return uuid.UUID(b)
}

func (Stdlib) ScanValue(b *[16]byte) any {
	return (*uuid.UUID)(b)
}
