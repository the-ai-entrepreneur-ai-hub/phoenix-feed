package main

import (
	"testing"
	"time"
)

func TestCapBackoff(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want time.Duration
	}{
		{0, 0},
		{30 * time.Second, 30 * time.Second},
		{90 * time.Second, 90 * time.Second},
		{2 * time.Minute, 90 * time.Second},
		{5 * time.Minute, 90 * time.Second},
	}
	for _, c := range cases {
		if got := capBackoff(c.in); got != c.want {
			t.Errorf("capBackoff(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}
