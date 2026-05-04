// Package source defines the interface every upstream data source must
// satisfy. Phoenix Fire MapServer is one. Scanner+Whisper will be another.
// The ingester does not know or care which.
package source

import (
	"context"

	"github.com/abusedmindset/phoenix-feed/internal/model"
)

// Source is one upstream that produces incident data.
type Source interface {
	// Name is the stable identifier stored in incidents.source. It must not
	// change for the lifetime of the system; it forms part of the composite
	// primary key and is exposed in API responses.
	Name() string

	// ParserVersion identifies the parser used by this Source. Bump this when
	// field handling, decoding, or normalization changes meaningfully so that
	// historical poll rows record which parser produced them.
	ParserVersion() string

	// Poll executes one fetch + parse cycle. Implementations must NOT retry
	// internally; the ingester loop handles backoff. Implementations should
	// always return a PollResult (never nil), with Err set on failure so the
	// outer audit can record latency / status code regardless.
	Poll(ctx context.Context) model.PollResult
}
