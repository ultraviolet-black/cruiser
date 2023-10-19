package server

import (
	"context"
	"errors"
	"io"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type GrpcMethodBackendFactory func(stream grpc.ServerStream) (GrpcMethodBackend, error)

type GrpcMethodBackend interface {
	Begin(context.Context, metadata.MD) error
	Request(context.Context, []byte) error
	Response(context.Context) (metadata.MD, []byte, error)
	End(context.Context)
}

type grpcHandler struct {
	methodBackendFactory GrpcMethodBackendFactory
	grpcServer           *grpc.Server
}

func (g *grpcHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.grpcServer.ServeHTTP(w, r)
}

func (g *grpcHandler) Handle(srv interface{}, stream grpc.ServerStream) error {

	methodBackend, err := g.methodBackendFactory(stream)

	if err != nil {

		return err

	}

	ctx := stream.Context()

	defer methodBackend.End(ctx)

	receiveCh := wrapGrpcFlow(stream, methodBackend, grpcReceiveFrom)
	sendCh := wrapGrpcFlow(stream, methodBackend, grpcSendFrom)

flow:
	for i := 0; i < 4; i++ {
		select {

		case receiveErr, ok := <-receiveCh:
			if !ok {
				receiveCh = nil
				continue flow
			}
			if receiveErr != nil {
				return receiveErr
			}

		case sendErr, ok := <-sendCh:
			if !ok {
				sendCh = nil
				continue flow
			}
			if sendErr != nil {
				return sendErr
			}

		}
	}

	return nil

}

func wrapGrpcFlow(stream grpc.ServerStream, methodBackend GrpcMethodBackend, flow func(grpc.ServerStream, GrpcMethodBackend) error) <-chan error {

	errCh := make(chan error)

	go func() {

		defer close(errCh)

		errCh <- flow(stream, methodBackend)

	}()

	return errCh

}

func grpcReceiveFrom(stream grpc.ServerStream, methodBackend GrpcMethodBackend) error {

	frame := newFrame(nil)

	ctx := stream.Context()

	incomingMetadata, hasIncomingMetadata := metadata.FromIncomingContext(ctx)

	for i := 0; ; i++ {

		if i == 0 && hasIncomingMetadata {
			if err := methodBackend.Begin(ctx, incomingMetadata); err != nil {
				return wrapGrpcError(err)
			}
		}

		if err := stream.RecvMsg(frame); err != nil {

			if errors.Is(err, io.EOF) {
				break
			}

			return err

		}

		if err := methodBackend.Request(ctx, frame.payload); err != nil {

			if errors.Is(err, io.EOF) {
				break
			}

			return wrapGrpcError(err)

		}

	}

	return nil

}

func grpcSendFrom(stream grpc.ServerStream, methodBackend GrpcMethodBackend) error {

	frame := newFrame(nil)

	ctx := stream.Context()

	for i := 0; ; i++ {

		outgoingMetadata, payload, err := methodBackend.Response(ctx)

		isEOF := err == io.EOF

		if i == 0 && outgoingMetadata != nil {
			if err := stream.SetHeader(outgoingMetadata); err != nil {
				return status.Errorf(codes.Internal, "Unable to set header on server stream: %s", err.Error())
			}
		}

		if err != nil && err != io.EOF {
			return wrapGrpcError(err)
		}

		frame.payload = payload

		if err := stream.SendMsg(frame); err != nil {
			return status.Errorf(codes.Internal, "Error sending result to server stream: %s", err.Error())
		}

		if isEOF {
			break
		}

	}

	return nil

}

func wrapGrpcError(err error) error {
	_, ok := status.FromError(err)
	if ok {
		return err
	}

	return status.Errorf(codes.Internal, "Internal error: %s", err.Error())
}
