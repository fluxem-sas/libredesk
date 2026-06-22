package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/abhinavxd/libredesk/internal/integrations/slack/models"
)

const (
	slackAPIBase   = "https://slack.com/api/"
	oauthAuthorize = "https://slack.com/oauth/v2/authorize"
	defaultScopes  = "chat:write,channels:read,groups:read"
)

// Client wraps Slack HTTP API calls.
type Client struct {
	httpClient *http.Client
}

// NewClient creates a Slack API client.
func NewClient(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
	}
}

// AuthorizeURL builds the Slack OAuth authorize URL.
func AuthorizeURL(clientID, redirectURI, state string) string {
	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("scope", defaultScopes)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	return oauthAuthorize + "?" + q.Encode()
}

// ExchangeOAuthCode exchanges an authorization code for a bot token.
func (c *Client) ExchangeOAuthCode(ctx context.Context, clientID, clientSecret, code, redirectURI string) (models.OAuthAccessResponse, error) {
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, slackAPIBase+"oauth.v2.access", strings.NewReader(form.Encode()))
	if err != nil {
		return models.OAuthAccessResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var resp models.OAuthAccessResponse
	if err := c.doJSON(req, &resp); err != nil {
		return resp, err
	}
	if !resp.OK {
		if resp.Error == "" {
			resp.Error = "oauth_failed"
		}
		return resp, fmt.Errorf("slack oauth: %s", resp.Error)
	}
	return resp, nil
}

// ListChannels returns public and private channels the bot can access.
func (c *Client) ListChannels(ctx context.Context, botToken string) ([]models.Channel, error) {
	var (
		channels []models.Channel
		cursor   string
	)

	for {
		q := url.Values{}
		q.Set("types", "public_channel,private_channel")
		q.Set("exclude_archived", "true")
		q.Set("limit", "200")
		if cursor != "" {
			q.Set("cursor", cursor)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, slackAPIBase+"conversations.list?"+q.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+botToken)

		var resp models.ConversationsListResponse
		if err := c.doJSON(req, &resp); err != nil {
			return nil, err
		}
		if !resp.OK {
			if resp.Error == "" {
				resp.Error = "conversations_list_failed"
			}
			return nil, fmt.Errorf("slack conversations.list: %s", resp.Error)
		}

		for _, ch := range resp.Channels {
			channels = append(channels, models.Channel{ID: ch.ID, Name: ch.Name})
		}

		cursor = resp.ResponseMetadata.NextCursor
		if cursor == "" {
			break
		}
	}

	return channels, nil
}

// PostMessage sends a message to a Slack channel.
func (c *Client) PostMessage(ctx context.Context, botToken, channelID string, blocks []map[string]any, text string) error {
	body := map[string]any{
		"channel": channelID,
		"text":    text,
	}
	if len(blocks) > 0 {
		body["blocks"] = blocks
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, slackAPIBase+"chat.postMessage", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+botToken)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	var resp models.PostMessageResponse
	if err := c.doJSON(req, &resp); err != nil {
		return err
	}
	if !resp.OK {
		if resp.Error == "" {
			resp.Error = "post_message_failed"
		}
		return fmt.Errorf("slack chat.postMessage: %s", resp.Error)
	}
	return nil
}

func (c *Client) doJSON(req *http.Request, out any) error {
	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	data, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode >= 400 {
		return fmt.Errorf("slack api http %d: %s", res.StatusCode, string(data))
	}
	return json.Unmarshal(data, out)
}
