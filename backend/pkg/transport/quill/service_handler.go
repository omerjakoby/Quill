package quill

import (
	"context"
	"quill/pkg/domain"
	"time"
)

// TODO input validation: verify thread_id, msg_id correlation between users and email, a user can't update email that isn't his ect
// TODO add support for receiving error codes from the domain layer so it will replace the general internal error
// ServiceHandler orchestrates DTO ↔ domain mapping, invoking services, and constructing DTO replies.
type ServiceHandler struct {
	authSvc  AuthService
	emailSvc EmailService
	keySvc   KeyService
}

// NewServiceHandler constructs the service-layer handler using existing services.
func NewServiceHandler(as AuthService, es EmailService, ks KeyService) *ServiceHandler {
	return &ServiceHandler{authSvc: as, emailSvc: es, keySvc: ks}
}

// HandleAuth authenticates the client and returns AuthAckPayload or an ErrorPayload
func (s *ServiceHandler) HandleAuth(ctx context.Context, payload AuthPayload) (AuthAckPayload, *ErrorPayload) {
	err := s.authSvc.Authenticate(ctx, payload.Credentials.Token)
	if err != nil {
		return AuthAckPayload{}, &ErrorPayload{Code: ErrorCodeInvalidToken, Message: err.Error(), Context: PacketTypeAuth}
	}
	//TODO add a check to validate email address from the ctx context and make sure its for the right userID
	//TODO remove the hardcoded expiresIn and have an actual logical amount (need to think what it is)
	sess := SessionInfo{ExpiresIn: 3600, Identity: payload.Credentials.Token}
	return AuthAckPayload{Accepted: true, Session: sess}, nil
}

// HandleFetchKeys looks up public keys and returns KeyResponsePayload or an ErrorPayload
func (s *ServiceHandler) HandleFetchKeys(ctx context.Context, payload FetchKeysPayload) (KeyResponsePayload, *ErrorPayload) {
	req := domain.FetchKeysRequest{Query: payload.Query}
	res, err := s.keySvc.FetchKeys(ctx, req)
	if err != nil {
		return KeyResponsePayload{}, &ErrorPayload{Code: ErrorCodeInternalServerError, Message: err.Error(), Context: PacketTypeFetchKeys}
	}
	return KeyResponsePayload{Email: res.Identity, PublicKey: res.PublicKey, Expires: res.Expires.Format(time.RFC3339)}, nil
}

// HandleSendEmail processes a send request and returns SendEmailAckPayload or an ErrorPayload
func (s *ServiceHandler) HandleSendEmail(ctx context.Context, payload SendEmailPayload) (SendEmailAckPayload, *ErrorPayload) {
	var threadID *string
	if payload.ThreadID != "" {
		threadID = &payload.ThreadID
	}

	// Convert quill.Attachment slice to domain.Attachment slice
	domainAttachments := make([]domain.Attachment, len(payload.Attachments))
	for i, att := range payload.Attachments {
		domainAttachments[i] = domain.Attachment{
			Filename: att.Filename,
			Mimetype: att.Mimetype,
			Link:     att.Link,
		}
	}

	// Handle optional SendOptions fields as pointers
	var expiresInSeconds *int
	if payload.Options.ExpiresInSeconds != 0 { // Assuming 0 means not set for ExpiresInSeconds
		expiresInSeconds = &payload.Options.ExpiresInSeconds
	}

	var oneTime *bool
	if payload.Options.OneTime { // Assuming false means not set for OneTime if it's typically omitted
		oneTime = &payload.Options.OneTime
	}
	dReq := domain.SendEmailRequest{
		MessageID:   payload.MessageID,
		ThreadID:    threadID,
		From:        payload.From,
		To:          payload.To,
		CC:          payload.Cc,
		BCC:         payload.Bcc,
		Subject:     payload.Subject,
		Body:        domain.EmailBody{Text: payload.Body.Text, HTML: payload.Body.HTML},
		Attachments: domainAttachments,
		Options: domain.SendEmailOptions{
			ExpiresInSeconds: expiresInSeconds,
			OneTime:          oneTime,
		},
	}
	res, err := s.emailSvc.SendEmail(ctx, dReq)
	if err != nil {
		return SendEmailAckPayload{}, &ErrorPayload{Code: ErrorCodeInternalServerError, Message: err.Error(), Context: PacketTypeSendEmail}
	}

	//TODO ITAMAR handle external delivery if needed. the res contains the queued_for field which is a list of addresses that need to be sent to

	return SendEmailAckPayload{Status: StatusOK, MessageID: res.MessageID, DeliveredTo: res.DeliveredTo, QueuedFor: res.QueuedFor}, nil
}

// HandleFetchEmail processes a fetch request and returns FetchEmailResponsePayload or an ErrorPayload
func (s *ServiceHandler) HandleFetchEmail(ctx context.Context, payload FetchEmailPayload) (FetchEmailResponsePayload, *ErrorPayload) {
	dReq := domain.FetchEmailRequest{Mode: domain.FetchEmailMode(payload.Mode)}
	if payload.Folder != "" {
		dReq.Folder = &payload.Folder
	}
	if payload.ThreadID != "" {
		dReq.ThreadID = &payload.ThreadID
	}
	// Set pagination and filters
	dReq.Limit = &payload.Limit
	dReq.Offset = &payload.Offset
	dReq.Filters = convertFilters(payload.Filters)

	// Call domain service
	res, err := s.emailSvc.FetchEmail(ctx, dReq)
	if err != nil {
		return FetchEmailResponsePayload{}, &ErrorPayload{Code: ErrorCodeInternalServerError, Message: err.Error(), Context: PacketTypeFetchEmail}
	}

	// Map based on mode
	if payload.Mode == string(domain.FetchModeOverview) {
		return mapFetchOverview(res), nil
	} else if payload.Mode == string(domain.FetchModeThread) {
		return mapFetchThread(res, payload.ThreadID), nil
	} else {
		return FetchEmailResponsePayload{}, &ErrorPayload{Code: ErrorCodeInvalidMode, Message: "Invalid Fetch Mode", Context: PacketTypeFetchEmail}
	}
}

// HandleUpdateEmail processes an update request and returns UpdateEmailAckPayload or an ErrorPayload
func (s *ServiceHandler) HandleUpdateEmail(ctx context.Context, payload UpdateEmailPayload) (UpdateEmailAckPayload, *ErrorPayload) {
	dReq := domain.UpdateEmailRequest{MessageIDs: payload.MessageIDs}
	hasUpdates := false

	if payload.Updates.Folder != "" {
		dReq.Folder = &payload.Updates.Folder
		hasUpdates = true
	}
	if payload.Updates.Category != "" {
		dReq.Category = &payload.Updates.Category
		hasUpdates = true
	}
	if payload.Updates.Flags.IsRead != nil ||
		payload.Updates.Flags.IsStarred != nil ||
		payload.Updates.Flags.IsDeleted != nil {

		dReq.Flags = domain.UpdateEmailFlags{
			IsRead:    payload.Updates.Flags.IsRead,
			IsStarred: payload.Updates.Flags.IsStarred,
			IsDeleted: payload.Updates.Flags.IsDeleted,
		}
		hasUpdates = true
	}

	if !hasUpdates {
		return UpdateEmailAckPayload{}, &ErrorPayload{Code: ErrorCodeInvalidPayload, Message: "No updates provided in payload", Context: PacketTypeUpdateEmail}
	}

	res, err := s.emailSvc.UpdateEmail(ctx, dReq)
	if err != nil {
		return UpdateEmailAckPayload{}, &ErrorPayload{Code: ErrorCodeInternalServerError, Message: err.Error(), Context: PacketTypeUpdateEmail}
	}
	return mapUpdateResult(res), nil
}

// convertFilters transforms DTO EmailFilters to domain.FetchEmailFilters
func convertFilters(f *EmailFilters) *domain.FetchEmailFilters {
	if f == nil {
		return nil
	}
	var df domain.FetchEmailFilters
	if f.Search != nil {
		df.Search = &domain.SearchFilter{Keywords: f.Search.Keywords, ExactPhrase: f.Search.ExactPhrase, From: f.Search.From, To: f.Search.To}
	}
	if f.Flags != nil {
		df.Flags = &domain.FlagFilter{HasAttachments: f.Flags.HasAttachments, IsRead: f.Flags.IsRead, IsStarred: f.Flags.IsStarred}
	}
	if f.DateRange != nil {
		after, _ := time.Parse(time.RFC3339, f.DateRange.After)
		before, _ := time.Parse(time.RFC3339, f.DateRange.Before)
		df.DateRange = &domain.DateRangeFilter{After: &after, Before: &before}
	}
	return &df
}

// mapFetchOverview maps overview mode results into DTO
func mapFetchOverview(res domain.FetchEmailResult) FetchEmailResponsePayload {
	out := FetchEmailResponsePayload{
		Status:       StatusOK,
		Mode:         string(domain.FetchModeOverview),
		Limit:        res.Limit,
		Offset:       res.Offset,
		TotalThreads: res.TotalThreads,
	}
	for _, thr := range res.ThreadOverviews {
		latest := thr.LatestMessage
		dSummary := MessageSummary{
			ID:        latest.ID,
			ThreadID:  latest.ThreadID,
			From:      latest.From,
			Subject:   latest.Subject,
			Timestamp: latest.Timestamp.Format(time.RFC3339),
			Snippet:   latest.Snippet,
			Flags: struct {
				HasAttachments bool `json:"has_attachments"`
				IsStarred      bool `json:"is_starred"`
			}{
				HasAttachments: latest.Flags.HasAttachments,
				IsStarred:      latest.Flags.IsStarred,
			},
		}
		out.Threads = append(out.Threads, ThreadOverview{
			ThreadID:      thr.ThreadID,
			LatestMessage: dSummary,
			Count:         thr.Count,
			UnreadCount:   thr.UnreadCount,
		})
	}
	return out
}

// mapFetchThread maps thread mode results into DTO
func mapFetchThread(res domain.FetchEmailResult, threadID string) FetchEmailResponsePayload {
	out := FetchEmailResponsePayload{
		Status:        StatusOK,
		Mode:          string(domain.FetchModeThread),
		ThreadID:      threadID,
		TotalMessages: res.TotalMessages,
		Limit:         res.Limit,
		Offset:        res.Offset,
	}
	for _, msg := range res.Messages {
		body := MessageBody{Text: msg.Body.Text, HTML: msg.Body.HTML}
		atts := make([]MessageAttachment, len(msg.Attachments))
		for i, a := range msg.Attachments {
			atts[i] = MessageAttachment{Filename: a.Filename, Mimetype: a.Mimetype, Link: a.Link}
		}
		dFlags := struct {
			HasAttachments bool `json:"has_attachments"`
			IsRead         bool `json:"is_read"`
			IsStarred      bool `json:"is_starred"`
		}{
			HasAttachments: msg.Flags.HasAttachments,
			IsRead:         msg.Flags.IsRead,
			IsStarred:      msg.Flags.IsStarred,
		}
		out.Messages = append(out.Messages, MessageDTO{
			ID:          msg.ID,
			From:        msg.From,
			To:          msg.To,
			Cc:          msg.CC,
			Bcc:         msg.BCC,
			Subject:     msg.Subject,
			Body:        body,
			Attachments: atts,
			Timestamp:   msg.Timestamp.Format(time.RFC3339),
			Flags:       dFlags,
		})
	}
	return out
}

// mapUpdateResult builds a DTO from domain.UpdateEmailResult
func mapUpdateResult(res domain.UpdateEmailResult) UpdateEmailAckPayload {
	var results []UpdateResultItem
	for _, item := range res.Results {
		var errDto *ErrorPayload
		if item.Error != nil {
			errDto = &ErrorPayload{Code: item.Error.Code, Message: item.Error.Message, Context: PacketTypeUpdateEmail}
		}
		results = append(results, UpdateResultItem{
			MessageID: item.MessageID,
			Status:    item.Status,
			Error:     errDto,
		})
	}
	return UpdateEmailAckPayload{Results: results}
}
