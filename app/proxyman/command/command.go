package command

import (
	"context"
	"strings"
	"sync"

	grpc "google.golang.org/grpc"

	core "github.com/v2fly/v2ray-core/v5"
	"github.com/v2fly/v2ray-core/v5/common"
	"github.com/v2fly/v2ray-core/v5/common/serial"
	"github.com/v2fly/v2ray-core/v5/features/inbound"
	"github.com/v2fly/v2ray-core/v5/features/outbound"
	"github.com/v2fly/v2ray-core/v5/proxy"
)

// InboundOperation is the interface for operations that applies to inbound handlers.
type InboundOperation interface {
	// ApplyInbound applies this operation to the given inbound handler.
	ApplyInbound(context.Context, inbound.Handler) error
}

// OutboundOperation is the interface for operations that applies to outbound handlers.
type OutboundOperation interface {
	// ApplyOutbound applies this operation to the given outbound handler.
	ApplyOutbound(context.Context, outbound.Handler) error
}

func getInbound(handler inbound.Handler) (proxy.Inbound, error) {
	gi, ok := handler.(proxy.GetInbound)
	if !ok {
		return nil, newError("can't get inbound proxy from handler.")
	}
	return gi.GetInbound(), nil
}

// ApplyInbound implements InboundOperation.
func (op *AddUserOperation) ApplyInbound(ctx context.Context, handler inbound.Handler) error {
	p, err := getInbound(handler)
	if err != nil {
		return err
	}
	um, ok := p.(proxy.UserManager)
	if !ok {
		return newError("proxy is not a UserManager")
	}
	mUser, err := op.User.ToMemoryUser()
	if err != nil {
		return newError("failed to parse user").Base(err)
	}
	return um.AddUser(ctx, mUser)
}

// ApplyInbound implements InboundOperation.
func (op *RemoveUserOperation) ApplyInbound(ctx context.Context, handler inbound.Handler) error {
	p, err := getInbound(handler)
	if err != nil {
		return err
	}
	um, ok := p.(proxy.UserManager)
	if !ok {
		return newError("proxy is not a UserManager")
	}
	return um.RemoveUser(ctx, op.Email)
}

type handlerServer struct {
	s   *core.Instance
	ihm inbound.Manager
	ohm outbound.Manager

	inboundAccess         sync.RWMutex
	taggedInboundRequest  map[string]*AddInboundRequest
	outboundAccess        sync.RWMutex
	taggedOutboundRequest map[string]*AddOutboundRequest
}

func (s *handlerServer) ListInboundConf(ctx context.Context, request *ListInboundConfRequest) (*ListInboundConfResponse, error) {
	s.inboundAccess.Lock()
	defer s.inboundAccess.Unlock()

	var (
		result ListInboundConfResponse
	)

	for _, inbound := range s.taggedInboundRequest {
		if request.Prefix != "" && !strings.HasPrefix(inbound.Inbound.Tag, request.Prefix) {
			continue
		}
		result.Inbounds = append(result.Inbounds, inbound.Inbound)
	}

	return &result, nil
}

func (s *handlerServer) ListOutboundConf(ctx context.Context, request *ListOutboundConfRequest) (*ListOutboundConfResponse, error) {
	s.outboundAccess.Lock()
	defer s.outboundAccess.Unlock()

	var (
		result ListOutboundConfResponse
	)

	for _, outbound := range s.taggedOutboundRequest {
		if request.Prefix != "" && !strings.HasPrefix(outbound.Outbound.Tag, request.Prefix) {
			continue
		}
		result.Outbounds = append(result.Outbounds, outbound.Outbound)
	}

	return &result, nil
}

func (s *handlerServer) GetInboundConf(ctx context.Context, request *GetInboundConfRequest) (*GetInboundConfResponse, error) {
	s.inboundAccess.Lock()
	defer s.inboundAccess.Unlock()

	req := s.taggedInboundRequest[request.GetTag()]
	if req == nil {
		return nil, newError("inbound not found: ", request.GetTag())
	}

	return &GetInboundConfResponse{Inbound: req.Inbound}, nil
}

func (s *handlerServer) GetOutboundConf(ctx context.Context, request *GetOutboundConfRequest) (*GetOutboundConfResponse, error) {
	s.outboundAccess.Lock()
	defer s.outboundAccess.Unlock()

	req := s.taggedOutboundRequest[request.GetTag()]
	if req == nil {
		return nil, newError("outbound not found: ", request.GetTag())
	}

	return &GetOutboundConfResponse{Outbound: req.Outbound}, nil
}

func (s *handlerServer) AddInbound(ctx context.Context, request *AddInboundRequest) (*AddInboundResponse, error) {
	if err := core.AddInboundHandler(s.s, request.Inbound); err != nil {
		return nil, err
	}

	s.inboundAccess.Lock()
	if request.Inbound != nil && request.Inbound.Tag != "" {
		s.taggedInboundRequest[request.Inbound.Tag] = request
	}
	s.inboundAccess.Unlock()

	return &AddInboundResponse{}, nil
}

func (s *handlerServer) RemoveInbound(ctx context.Context, request *RemoveInboundRequest) (*RemoveInboundResponse, error) {
	s.inboundAccess.Lock()
	if request.Tag != "" {
		delete(s.taggedInboundRequest, request.Tag)
	}
	s.inboundAccess.Unlock()
	return &RemoveInboundResponse{}, s.ihm.RemoveHandler(ctx, request.Tag)
}

func (s *handlerServer) AlterInbound(ctx context.Context, request *AlterInboundRequest) (*AlterInboundResponse, error) {
	rawOperation, err := serial.GetInstanceOf(request.Operation)
	if err != nil {
		return nil, newError("unknown operation").Base(err)
	}
	operation, ok := rawOperation.(InboundOperation)
	if !ok {
		return nil, newError("not an inbound operation")
	}

	handler, err := s.ihm.GetHandler(ctx, request.Tag)
	if err != nil {
		return nil, newError("failed to get handler: ", request.Tag).Base(err)
	}

	return &AlterInboundResponse{}, operation.ApplyInbound(ctx, handler)
}

func (s *handlerServer) AddOutbound(ctx context.Context, request *AddOutboundRequest) (*AddOutboundResponse, error) {
	if err := core.AddOutboundHandler(s.s, request.Outbound); err != nil {
		return nil, err
	}

	s.outboundAccess.Lock()
	if request.Outbound != nil && request.Outbound.Tag != "" {
		s.taggedOutboundRequest[request.Outbound.Tag] = request
	}
	s.outboundAccess.Unlock()

	return &AddOutboundResponse{}, nil
}

func (s *handlerServer) RemoveOutbound(ctx context.Context, request *RemoveOutboundRequest) (*RemoveOutboundResponse, error) {
	s.outboundAccess.Lock()
	if request.Tag != "" {
		delete(s.taggedOutboundRequest, request.Tag)
	}
	s.outboundAccess.Unlock()
	return &RemoveOutboundResponse{}, s.ohm.RemoveHandler(ctx, request.Tag)
}

func (s *handlerServer) AlterOutbound(ctx context.Context, request *AlterOutboundRequest) (*AlterOutboundResponse, error) {
	rawOperation, err := serial.GetInstanceOf(request.Operation)
	if err != nil {
		return nil, newError("unknown operation").Base(err)
	}
	operation, ok := rawOperation.(OutboundOperation)
	if !ok {
		return nil, newError("not an outbound operation")
	}

	handler := s.ohm.GetHandler(request.Tag)
	return &AlterOutboundResponse{}, operation.ApplyOutbound(ctx, handler)
}

func (s *handlerServer) mustEmbedUnimplementedHandlerServiceServer() {}

type service struct {
	v *core.Instance
}

func (s *service) Register(server *grpc.Server) {
	hs := &handlerServer{
		s: s.v,
	}
	common.Must(s.v.RequireFeatures(func(im inbound.Manager, om outbound.Manager) {
		hs.ihm = im
		hs.ohm = om
		hs.taggedInboundRequest = make(map[string]*AddInboundRequest)
		hs.taggedOutboundRequest = make(map[string]*AddOutboundRequest)
	}))
	RegisterHandlerServiceServer(server, hs)
}

func init() {
	common.Must(common.RegisterConfig((*Config)(nil), func(ctx context.Context, cfg interface{}) (interface{}, error) {
		s := core.MustFromContext(ctx)
		return &service{v: s}, nil
	}))
}
