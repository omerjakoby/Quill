// pkg/transport/quill/service_handler.go
package quill

import (
	"context"
)

// authService and EmailService are your existing interfaces
// authService authenticates tokens, EmailService handles domain logic.

type ServiceHandler struct {
	authSvc    AuthService
	emailSvc EmailService
}

// NewServiceHandler constructs the service-layer handler using existing services.
func NewServiceHandler(as AuthService, ms EmailService) *ServiceHandler {
	return &ServiceHandler{
		authSvc:    as,
		emailSvc: ms,
	}
}

// HandleSend processes SEND_EMAIL packets once authenticated.
func (s *ServiceHandler) HandleSend(ctx context.Context, pkt *Packet) *Packet {
	// TODO: unmarshal payload into DomainSendRequest
	// req := parseSendRequest(pkt.Payload)
	// result, err := s.emailSvc.Send(ctx, req)
	// return buildSendEmailAck(result, err)
	return nil
}

// HandleFetch processes FETCH_EMAIL packets once authenticated.
func (s *ServiceHandler) HandleFetch(ctx context.Context, pkt *Packet) *Packet {
	// TODO: unmarshal payload into DomainFetchRequest
	// res, err := s.emailSvc.Fetch(ctx, req)
	// return buildFetchEmailResponse(res, err)
	return nil
}

// HandleUpdate processes UPDATE_EMAIL packets once authenticated.
func (s *ServiceHandler) HandleUpdate(ctx context.Context, pkt *Packet) *Packet {
	// TODO: unmarshal payload into update request
	// err := s.emailSvc.Update(ctx, req)
	// return buildUpdateEmailAck(err)
	return nil
}
