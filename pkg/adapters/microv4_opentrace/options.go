package microv4_opentrace

import (
	"context"
	"github.com/liuhailove/gmiter/core/base"
	"github.com/opentracing/opentracing-go"
	"go-micro.dev/v4/client"
	"go-micro.dev/v4/server"
)

type (
	Option func(*options)

	options struct {
		clientResourceExtract func(context.Context, client.Request) string
		serverResourceExtract func(context.Context, server.Request) string

		streamClientResourceExtract func(context.Context, client.Request) string
		streamServerResourceExtract func(server.Stream) string

		clientBlockFallback func(context.Context, client.Request, *base.BlockError) error
		serverBlockFallback func(context.Context, server.Request, *base.BlockError) error

		streamClientBlockFallback func(context.Context, client.Request, *base.BlockError) (client.Stream, error)
		streamServerBlockFallback func(server.Stream, *base.BlockError) server.Stream

		// tracer 链路追踪Tracer
		tracer opentracing.Tracer
	}
)

// WithClientResourceExtractor sets the resource extractor of unary client request.
// The second string parameter is the full method name of current invocation.
func WithClientResourceExtractor(fn func(ctx context.Context, request client.Request) string) Option {
	return func(o *options) {
		o.clientResourceExtract = fn
	}
}

// WithServerResourceExtractor sets the resource extractor of unary server request.
func WithServerResourceExtractor(fn func(ctx context.Context, request server.Request) string) Option {
	return func(o *options) {
		o.serverResourceExtract = fn
	}
}

// WithStreamClientResourceExtractor sets the resource extractor of stream client request.
func WithStreamClientResourceExtractor(fn func(ctx context.Context, request client.Request) string) Option {
	return func(o *options) {
		o.streamClientResourceExtract = fn
	}
}

// WithStreamServerResourceExtractor sets the resource extractor of stream server request.
func WithStreamServerResourceExtractor(fn func(stream server.Stream) string) Option {
	return func(o *options) {
		o.streamServerResourceExtract = fn
	}
}

// WithClientBlockFallback sets the block fallback handler of unary client request.
// The second string parameter is the full method name of current invocation.
func WithClientBlockFallback(fn func(context.Context, client.Request, *base.BlockError) error) Option {
	return func(o *options) {
		o.clientBlockFallback = fn
	}
}

// WithServerBlockFallback sets the block fallback handler of unary server request.
func WithServerBlockFallback(fn func(context.Context, server.Request, *base.BlockError) error) Option {
	return func(o *options) {
		o.serverBlockFallback = fn
	}
}

// WithStreamClientBlockFallback sets the block fallback handler of stream client request.
func WithStreamClientBlockFallback(fn func(context.Context, client.Request, *base.BlockError) (client.Stream, error)) Option {
	return func(o *options) {
		o.streamClientBlockFallback = fn
	}
}

// WithStreamServerBlockFallback sets the block fallback handler of stream server request.
func WithStreamServerBlockFallback(fn func(server.Stream, *base.BlockError) server.Stream) Option {
	return func(opts *options) {
		opts.streamServerBlockFallback = fn
	}
}

func WithOpenTracer(tracer opentracing.Tracer) Option {
	return func(opts *options) {
		opts.tracer = tracer
	}
}

func evaluateOptions(opts []Option) *options {
	optCopy := &options{}
	for _, o := range opts {
		o(optCopy)
	}
	return optCopy
}
