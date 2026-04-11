package ejabberd

import "context"

type Client interface {
	Ready(ctx context.Context) error
	Close() error
}
type Stub struct{}

func NewStub() *Stub                        { return &Stub{} }
func (s *Stub) Ready(context.Context) error { return nil }
func (s *Stub) Close() error                { return nil }
