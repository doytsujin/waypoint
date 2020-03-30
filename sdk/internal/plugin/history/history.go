package history

import (
	"context"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	"github.com/mitchellh/devflow/sdk/component"
	"github.com/mitchellh/devflow/sdk/history"
	"github.com/mitchellh/devflow/sdk/history/convert"
	"github.com/mitchellh/devflow/sdk/internal-shared/mapper"
	pb "github.com/mitchellh/devflow/sdk/proto"
)

// HistoryPlugin implements plugin.Plugin (specifically GRPCPlugin) for
// the History component type.
type HistoryPlugin struct {
	plugin.NetRPCUnsupportedPlugin

	Impl    history.Client // Impl is the concrete implementation
	Mappers []*mapper.Func // Mappers
	Logger  hclog.Logger   // Logger
}

func (p *HistoryPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	pb.RegisterHistoryServiceServer(s, &historyServer{
		Impl:    p.Impl,
		Mappers: p.Mappers,
		Logger:  p.Logger,
	})
	return nil
}

func (p *HistoryPlugin) GRPCClient(
	ctx context.Context,
	broker *plugin.GRPCBroker,
	c *grpc.ClientConn,
) (interface{}, error) {
	return &historyClient{
		client:  pb.NewHistoryServiceClient(c),
		logger:  p.Logger,
		mappers: p.Mappers,
	}, nil
}

// historyClient is an implementation of component.History that
// communicates over gRPC.
type historyClient struct {
	client  pb.HistoryServiceClient
	logger  hclog.Logger
	mappers []*mapper.Func
}

func (c *historyClient) Deployments(
	ctx context.Context,
	cfg *history.Lookup,
) ([]component.Deployment, error) {
	// Call it
	resp, err := c.client.Deployments(ctx, &pb.History_LookupRequest{})
	if err != nil {
		return nil, err
	}

	result, err := convert.Component(
		mapper.Set(c.mappers),
		resp.Results,
		cfg.Type,
		(*component.Deployment)(nil),
	)
	if err != nil {
		return nil, err
	}

	return result.([]component.Deployment), nil
}

// historyServer is a gRPC server that the client talks to and calls a
// real implementation of the component.
type historyServer struct {
	Impl    history.Client
	Mappers []*mapper.Func
	Logger  hclog.Logger
}

func (s *historyServer) Deployments(
	ctx context.Context,
	args *pb.History_LookupRequest,
) (*pb.History_LookupResponse, error) {
	result, err := s.Impl.Deployments(ctx, nil)
	if err != nil {
		return nil, err
	}

	encoded, err := component.ProtoAnySlice(result)
	if err != nil {
		return nil, err
	}

	return &pb.History_LookupResponse{Results: encoded}, nil
}

var (
	_ plugin.Plugin           = (*HistoryPlugin)(nil)
	_ plugin.GRPCPlugin       = (*HistoryPlugin)(nil)
	_ pb.HistoryServiceServer = (*historyServer)(nil)
	_ history.Client          = (*historyClient)(nil)
)
