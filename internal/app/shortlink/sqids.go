package shortlink

import (
	"sync"

	"github.com/sqids/sqids-go"
)

var (
	sq   *sqids.Sqids
	once sync.Once
)

func getSqids() *sqids.Sqids {
	once.Do(func() {
		var err error
		sq, err = sqids.New(sqids.Options{
			Alphabet:  "k3G7QAe51FCsiWrNOYBUwM6XzZvdLT4j9JhyHKg2cVbxfERq0mSoI8lDpunPat",
			MinLength: 3,
		})
		if err != nil {
			panic("sqids init failed: " + err.Error())
		}
	})
	return sq
}

func SqidsEncode(id uint64) (string, error) {
	return getSqids().Encode([]uint64{id})
}
