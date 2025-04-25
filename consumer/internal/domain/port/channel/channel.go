package channel

import (
	"context"
)

type Channel interface {
	Send(ctx context.Context, body, to string) error
}
