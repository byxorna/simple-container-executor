package main

import (
	"fmt"

	"github.com/dustin/go-humanize"
)

// this type implements the Value interface, needed by flags :)
type bytesFmt int64

// returns the string representation of a bytesFmt type
func (b *bytesFmt) String() string {
	return fmt.Sprintf("%d", *b)
}

// given a string like 128MB, parse into int64 bytes
func (b *bytesFmt) Set(v string) error {
	i, err := humanize.ParseBytes(v)
	if err != nil {
		return err
	}
	*b = bytesFmt(i)
	return nil
}
