// pkg/transport/quill/service_handler.go
package quill

import (
	"context"
)

// authService and EmailService are your existing interfaces
// authService authenticates tokens, EmailService handles domain logic.

type ServiceHandler struct {
	authSvc  AuthService
	emailSvc EmailService
	keySvc   KeyService
}

// NewServiceHandler constructs the service-layer handler using existing services.
func NewServiceHandler(as AuthService, ms EmailService, ks KeyService) *ServiceHandler {
	return &ServiceHandler{
		authSvc:  as,
		emailSvc: ms,
		keySvc:   ks,
	}
}

// HandleSend processes SEND_EMAIL packets once authenticated.
func (s *ServiceHandler) HandleSendEmail(ctx context.Context, pkt *Packet) *Packet {
	// TODO: unmarshal payload into DomainSendRequest
	// req := parseSendRequest(pkt.Payload)
	// result, err := s.emailSvc.Send(ctx, req)
	// return buildSendEmailAck(result, err)
	return nil
}

// HandleFetch processes FETCH_EMAIL packets once authenticated.
func (s *ServiceHandler) HandleFetchEmail(ctx context.Context, pkt *Packet) *Packet {
	// TODO: unmarshal payload into DomainFetchRequest
	// res, err := s.emailSvc.Fetch(ctx, req)
	// return buildFetchEmailResponse(res, err)
	return nil
}

// HandleUpdate processes UPDATE_EMAIL packets once authenticated.
func (s *ServiceHandler) HandleUpdateEmail(ctx context.Context, pkt *Packet) *Packet {
	// TODO: unmarshal payload into update request
	// err := s.emailSvc.Update(ctx, req)
	// return buildUpdateEmailAck(err)
	return nil
}

func (s *ServiceHandler) HandleFetchKeys(ctx context.Context, pkt *Packet) *Packet {
	// TODO: actually do the function code
	return nil
}
