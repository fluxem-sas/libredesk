package email

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/abhinavxd/libredesk/internal/attachment"
	cmodels "github.com/abhinavxd/libredesk/internal/conversation/models"
	"github.com/abhinavxd/libredesk/internal/inbox"
	imodels "github.com/abhinavxd/libredesk/internal/inbox/models"
	"github.com/abhinavxd/libredesk/internal/stringutil"
	"github.com/svix/svix-webhooks/go"
	"github.com/volatiletech/null/v9"
	"github.com/zerodha/logf"
)

const resendAPIBaseURL = "https://api.resend.com"

type resendSendEmailRequest struct {
	From        string                 `json:"from"`
	To          []string               `json:"to"`
	CC          []string               `json:"cc,omitempty"`
	BCC         []string               `json:"bcc,omitempty"`
	ReplyTo     string                 `json:"reply_to,omitempty"`
	Subject     string                 `json:"subject"`
	HTML        string                 `json:"html,omitempty"`
	Text        string                 `json:"text,omitempty"`
	Headers     map[string]string      `json:"headers,omitempty"`
	Attachments []resendSendAttachment `json:"attachments,omitempty"`
}

type resendSendAttachment struct {
	Filename    string `json:"filename"`
	Content     string `json:"content"`
	ContentType string `json:"content_type,omitempty"`
	ContentID   string `json:"content_id,omitempty"`
}

type resendSendEmailResponse struct {
	ID string `json:"id"`
}

type ResendWebhookEvent struct {
	Type      string                 `json:"type"`
	CreatedAt time.Time              `json:"created_at"`
	Data      ResendWebhookEventData `json:"data"`
}

type ResendWebhookEventData struct {
	EmailID   string   `json:"email_id"`
	CreatedAt string   `json:"created_at"`
	From      string   `json:"from"`
	To        []string `json:"to"`
	CC        []string `json:"cc"`
	BCC       []string `json:"bcc"`
	MessageID string   `json:"message_id"`
	Subject   string   `json:"subject"`
}

type resendReceivedEmail struct {
	Object      string            `json:"object"`
	ID          string            `json:"id"`
	To          []string          `json:"to"`
	From        string            `json:"from"`
	CreatedAt   string            `json:"created_at"`
	Subject     string            `json:"subject"`
	HTML        string            `json:"html"`
	HTMLFormat  string            `json:"html_format"`
	Text        string            `json:"text"`
	Headers     map[string]string `json:"headers"`
	BCC         []string          `json:"bcc"`
	CC          []string          `json:"cc"`
	ReplyTo     []string          `json:"reply_to"`
	MessageID   string            `json:"message_id"`
	Attachments []struct {
		ID                 string `json:"id"`
		Filename           string `json:"filename"`
		ContentType        string `json:"content_type"`
		ContentDisposition string `json:"content_disposition"`
		ContentID          string `json:"content_id"`
		Size               int    `json:"size"`
	} `json:"attachments"`
}

type resendListAttachmentsResponse struct {
	Object  string                     `json:"object"`
	HasMore bool                       `json:"has_more"`
	Data    []resendReceivedAttachment `json:"data"`
}

type resendReceivedAttachment struct {
	ID                 string `json:"id"`
	Filename           string `json:"filename"`
	Size               int    `json:"size"`
	ContentType        string `json:"content_type"`
	ContentDisposition string `json:"content_disposition"`
	ContentID          string `json:"content_id"`
	DownloadURL        string `json:"download_url"`
	ExpiresAt          string `json:"expires_at"`
}

func (e *Email) sendViaResend(m cmodels.OutboundMessage) error {
	if e.resend == nil || e.resend.APIKey == "" {
		return fmt.Errorf("resend is not configured for inbox %d", e.Identifier())
	}

	headers := map[string]string{}

	emailAddress, err := stringutil.ExtractEmail(m.From)
	if err != nil {
		e.lo.Error("failed to extract email address from the 'from' header", "error", err)
		return fmt.Errorf("failed to extract email address from 'From' header: %w", err)
	}
	headers[headerLibredeskLoopPrevention] = emailAddress

	if rt := resolveReplyTo(m.ReplyTo, e.replyTo, emailAddress, m.ConversationUUID, e.enablePlusAddressing); rt != "" {
		headers["Reply-To"] = rt
	}

	for key, value := range e.headers {
		headers[key] = value
	}

	if m.InReplyTo != "" {
		headers[headerInReplyTo] = "<" + strings.Trim(m.InReplyTo, " <>") + ">"
	}

	if m.SourceID != "" {
		headers[headerMessageID] = fmt.Sprintf("<%s>", strings.Trim(m.SourceID, " <>"))
	}

	if len(m.References) > 0 {
		refs := make([]string, 0, len(m.References))
		for _, ref := range m.References {
			ref = strings.TrimSpace(strings.Trim(ref, "<>"))
			if ref == "" {
				continue
			}
			refs = append(refs, "<"+ref+">")
		}
		if len(refs) > 0 {
			headers[headerReferences] = strings.Join(refs, " ")
		}
	}

	if m.ConversationUUID != "" {
		headers[headerLibredeskConversationID] = m.ConversationUUID
	}

	payload := resendSendEmailRequest{
		From:    m.From,
		To:      m.To,
		CC:      m.CC,
		BCC:     m.BCC,
		Subject: m.Subject,
		Headers: headers,
	}

	if replyTo := headers["Reply-To"]; replyTo != "" {
		payload.ReplyTo = replyTo
	}

	switch m.ContentType {
	case "plain":
		payload.Text = m.Content
	default:
		payload.HTML = m.Content
		if m.AltContent != "" {
			payload.Text = m.AltContent
		} else if m.TextContent != "" {
			payload.Text = m.TextContent
		}
	}

	payload.Attachments = make([]resendSendAttachment, 0, len(m.Attachments))
	for _, file := range m.Attachments {
		att := resendSendAttachment{
			Filename:    file.Name,
			Content:     base64.StdEncoding.EncodeToString(file.Content),
			ContentType: file.ContentType,
		}
		if file.ContentID != "" {
			att.ContentID = file.ContentID
		}
		payload.Attachments = append(payload.Attachments, att)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var resp resendSendEmailResponse
	if err := resendDoJSON(ctx, e.resend.APIKey, http.MethodPost, resendAPIBaseURL+"/emails", payload, &resp); err != nil {
		return err
	}

	return nil
}

func VerifyResendWebhook(payload []byte, headers map[string]string, secret string) error {
	wh, err := svix.NewWebhook(secret)
	if err != nil {
		return fmt.Errorf("creating svix webhook verifier: %w", err)
	}

	msgHeaders := http.Header{}
	msgHeaders.Set("svix-id", headers["svix-id"])
	msgHeaders.Set("svix-timestamp", headers["svix-timestamp"])
	msgHeaders.Set("svix-signature", headers["svix-signature"])

	if err := wh.Verify(payload, msgHeaders); err != nil {
		return fmt.Errorf("verifying resend webhook signature: %w", err)
	}

	return nil
}

func MatchResendInbox(cfg imodels.Config, recipients []string) bool {
	candidates := make([]string, 0, 2)
	if fromAddr, ok := extractParsedEmail(cfg.From); ok {
		candidates = append(candidates, fromAddr)
	}
	if replyToAddr, ok := extractParsedEmail(cfg.ReplyTo); ok {
		candidates = append(candidates, replyToAddr)
	}

	for _, recipient := range recipients {
		normalized, ok := extractParsedEmail(recipient)
		if !ok {
			continue
		}
		base := stripPlusAddress(normalized)
		for _, candidate := range candidates {
			if normalized == candidate || base == candidate {
				return true
			}
		}
	}

	return false
}

func ProcessResendReceivedEmail(ctx context.Context, event ResendWebhookEvent, inboxID int, cfg imodels.Config, msgStore inbox.MessageStore, userStore inbox.UserStore, lo *logf.Logger) error {
	if cfg.Resend == nil || cfg.Resend.APIKey == "" {
		return fmt.Errorf("resend api key is missing")
	}

	messageID := normalizeMessageID(event.Data.MessageID)
	if messageID == "" {
		return nil
	}

	exists, err := msgStore.MessageExists(messageID)
	if err != nil {
		return fmt.Errorf("checking if resend message exists in DB: %w", err)
	}
	if exists {
		return nil
	}

	fromAddress, _ := extractParsedEmail(event.Data.From)
	if fromAddress != "" {
		blocked, err := userStore.IsEmailBlocked(fromAddress)
		if err != nil {
			return fmt.Errorf("checking if email is blocked: %w", err)
		}
		if blocked {
			lo.Info("contact email is blocked dropping incoming resend email", "email", fromAddress, "inbox_id", inboxID)
			return nil
		}
	}

	receivedEmail, err := getResendReceivedEmail(ctx, cfg.Resend.APIKey, event.Data.EmailID)
	if err != nil {
		return err
	}

	if receivedEmail.MessageID != "" {
		messageID = normalizeMessageID(receivedEmail.MessageID)
	}
	if messageID == "" {
		return nil
	}

	headers := lowerHeaderKeys(receivedEmail.Headers)
	fromHeader := receivedEmail.From
	if headers["from"] != "" {
		fromHeader = headers["from"]
	}

	contactFirstName, contactLastName, contactEmail := parseIncomingContact(fromHeader)
	if contactEmail == "" {
		contactEmail = fromAddress
	}
	if contactEmail == "" {
		return nil
	}

	meta, err := json.Marshal(map[string]any{
		"from":    normalizeEmails([]string{fromHeader}),
		"cc":      normalizeEmails(receivedEmail.CC),
		"bcc":     normalizeEmails(receivedEmail.BCC),
		"to":      normalizeEmails(receivedEmail.To),
		"subject": firstNonEmpty(receivedEmail.Subject, event.Data.Subject),
	})
	if err != nil {
		return fmt.Errorf("marshalling resend meta: %w", err)
	}

	incomingMsg := cmodels.IncomingMessage{
		Channel: ChannelEmail,
		InboxID: inboxID,
		Contact: cmodels.IncomingContact{
			FirstName: contactFirstName,
			LastName:  contactLastName,
			Email:     null.StringFrom(strings.ToLower(contactEmail)),
		},
		Subject:   firstNonEmpty(receivedEmail.Subject, event.Data.Subject),
		SourceID:  null.StringFrom(messageID),
		Meta:      meta,
		InReplyTo: normalizeMessageID(headers[strings.ToLower(headerInReplyTo)]),
	}

	if receivedEmail.HTML != "" {
		incomingMsg.Content = receivedEmail.HTML
		incomingMsg.ContentType = cmodels.ContentTypeHTML
	} else {
		incomingMsg.Content = receivedEmail.Text
		incomingMsg.ContentType = cmodels.ContentTypeText
	}

	if refs := headers[strings.ToLower(headerReferences)]; refs != "" {
		for _, ref := range strings.Fields(refs) {
			ref = normalizeMessageID(ref)
			if ref != "" {
				incomingMsg.References = append(incomingMsg.References, ref)
			}
		}
	}

	incomingMsg.ConversationUUIDFromReplyTo = extractConversationUUIDFromRecipients(receivedEmail.To)
	if incomingMsg.ConversationUUIDFromReplyTo == "" {
		incomingMsg.ConversationUUIDFromReplyTo = extractConversationUUIDFromRecipients(receivedEmail.CC)
	}

	attachments, err := listResendAttachments(ctx, cfg.Resend.APIKey, event.Data.EmailID)
	if err != nil {
		return err
	}
	for _, att := range attachments {
		content, err := downloadAttachment(ctx, att.DownloadURL)
		if err != nil {
			return err
		}
		disposition := attachment.DispositionAttachment
		if att.ContentDisposition == attachment.DispositionInline || att.ContentID != "" {
			disposition = attachment.DispositionInline
		}
		incomingMsg.Attachments = append(incomingMsg.Attachments, attachment.Attachment{
			Name:        att.Filename,
			Content:     content,
			ContentType: att.ContentType,
			ContentID:   att.ContentID,
			Size:        len(content),
			Disposition: disposition,
		})
	}

	incomingMsg.Content = stringutil.SanitizeUTF8(incomingMsg.Content)
	incomingMsg.Subject = stringutil.SanitizeUTF8(incomingMsg.Subject)
	incomingMsg.Contact.FirstName = stringutil.SanitizeUTF8(incomingMsg.Contact.FirstName)
	incomingMsg.Contact.LastName = stringutil.SanitizeUTF8(incomingMsg.Contact.LastName)

	return msgStore.EnqueueIncoming(incomingMsg)
}

func getResendReceivedEmail(ctx context.Context, apiKey, emailID string) (resendReceivedEmail, error) {
	var out resendReceivedEmail
	endpoint := resendAPIBaseURL + "/emails/receiving/" + url.PathEscape(emailID) + "?html_format=cid"
	if err := resendDoJSON(ctx, apiKey, http.MethodGet, endpoint, nil, &out); err != nil {
		return out, err
	}
	return out, nil
}

func listResendAttachments(ctx context.Context, apiKey, emailID string) ([]resendReceivedAttachment, error) {
	var out resendListAttachmentsResponse
	endpoint := resendAPIBaseURL + "/emails/receiving/" + url.PathEscape(emailID) + "/attachments"
	if err := resendDoJSON(ctx, apiKey, http.MethodGet, endpoint, nil, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

func downloadAttachment(ctx context.Context, downloadURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating attachment download request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading resend attachment: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return nil, fmt.Errorf("downloading resend attachment failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading resend attachment body: %w", err)
	}
	return content, nil
}

func resendDoJSON(ctx context.Context, apiKey, method, endpoint string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshalling resend payload: %w", err)
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return fmt.Errorf("creating resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending resend request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading resend response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("resend request failed: %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}
	if out == nil || len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("unmarshalling resend response: %w", err)
	}
	return nil
}

func normalizeMessageID(messageID string) string {
	return strings.TrimSpace(strings.Trim(messageID, "<>"))
}

func lowerHeaderKeys(headers map[string]string) map[string]string {
	out := make(map[string]string, len(headers))
	for key, value := range headers {
		out[strings.ToLower(strings.TrimSpace(key))] = value
	}
	return out
}

func normalizeEmails(addresses []string) []string {
	out := make([]string, 0, len(addresses))
	for _, addr := range addresses {
		if parsed, ok := extractParsedEmail(addr); ok {
			out = append(out, parsed)
		}
	}
	return out
}

func extractParsedEmail(input string) (string, bool) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", false
	}
	if addr, err := mail.ParseAddress(input); err == nil && addr.Address != "" {
		return strings.ToLower(addr.Address), true
	}
	return strings.ToLower(strings.Trim(input, "<>")), strings.Contains(input, "@")
}

func stripPlusAddress(emailAddr string) string {
	parts := strings.SplitN(emailAddr, "@", 2)
	if len(parts) != 2 {
		return emailAddr
	}
	local := strings.SplitN(parts[0], "+", 2)[0]
	return local + "@" + parts[1]
}

func extractConversationUUIDFromRecipients(recipients []string) string {
	for _, recipient := range recipients {
		if uuid := stringutil.ExtractConvUUID(recipient); uuid != "" {
			return uuid
		}
	}
	return ""
}

func parseIncomingContact(from string) (string, string, string) {
	if addr, err := mail.ParseAddress(from); err == nil {
		emailAddr := strings.ToLower(addr.Address)
		name := strings.TrimSpace(addr.Name)
		parts := strings.Fields(name)
		switch len(parts) {
		case 0:
			local := strings.SplitN(emailAddr, "@", 2)[0]
			return local, "", emailAddr
		case 1:
			return parts[0], "", emailAddr
		default:
			return parts[0], strings.Join(parts[1:], " "), emailAddr
		}
	}
	if emailAddr, ok := extractParsedEmail(from); ok {
		local := strings.SplitN(emailAddr, "@", 2)[0]
		return local, "", emailAddr
	}
	return "", "", ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
