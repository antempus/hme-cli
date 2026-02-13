package icloud

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	BaseURLV1 = "https://p68-maildomainws.icloud.com/v1/hme"
	BaseURLV2 = "https://p68-maildomainws.icloud.com/v2/hme"
)

type Client struct {
	httpClient *http.Client
}

type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
	cookies string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	newReq := req.Clone(req.Context())
	for k, v := range t.headers {
		newReq.Header.Set(k, v)
	}
	if t.cookies != "" {
		newReq.Header.Set("Cookie", t.cookies)
	}
	return t.base.RoundTrip(newReq)
}

func NewClient(rawCookies string) (*Client, error) {
	headers := map[string]string{
		"Connection":         "keep-alive",
		"Pragma":             "no-cache",
		"Cache-Control":      "no-cache",
		"User-Agent":         "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36",
		"Content-Type":       "application/json",
		"Accept":             "*/*",
		"Sec-GPC":            "1",
		"Origin":             "https://www.icloud.com",
		"Sec-Fetch-Site":     "same-site",
		"Sec-Fetch-Mode":     "cors",
		"Sec-Fetch-Dest":     "empty",
		"Referer":            "https://www.icloud.com/",
		"Accept-Language":    "en-US,en-GB;q=0.9,en;q=0.8,cs;q=0.7",
		"sec-ch-ua":          `"Brave";v="141", "Not?A_Brand";v="8", "Chromium";v="141"`,
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": `"macOS"`,
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &headerTransport{
			base:    http.DefaultTransport,
			headers: headers,
			cookies: string(rawCookies),
		},
	}

	return &Client{httpClient: client}, nil
}

type EmailList struct {
	Success bool `json:"success"`
	Result  struct {
		ForwardToEmails []string `json:"forwardToEmails"`

		HmeEmails []struct {
			Origin          string `json:"origin"`
			AnonymousID     string `json:"anonymousId"`
			Domain          string `json:"domain"`
			ForwardToEmail  string `json:"forwardToEmail"`
			Hme             string `json:"hme"`
			Label           string `json:"label"`
			Note            string `json:"note"`
			CreateTimestamp int64  `json:"createTimestamp"`
			IsActive        bool   `json:"isActive"`
			RecipientMailID string `json:"recipientMailId"`
			OriginAppName   string `json:"originAppName,omitempty"`
			AppBundleID     string `json:"appBundleId,omitempty"`
		} `json:"hmeEmails"`

		SelectedForwardTo string `json:"selectedForwardTo"`
	} `json:"result"`
}

type GenerateRequest struct {
	LangCode string `json:"langCode"`
}

type DeactivateReactivateRequest struct {
	AnonymousID string `json:"anonymousId"`
}

type ReserveRequest struct {
	HME   string `json:"hme"`
	Label string `json:"label"`
	Note  string `json:"note"`
}

type APIResponse struct {
	Success bool            `json:"success"`
	Reason  string          `json:"reason,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

func (r *APIResponse) GetError() error {
	if r.Success {
		return nil
	}
	errMsg := "unknown API error"
	if len(r.Error) > 0 {
		var errObj struct {
			ErrorMessage string `json:"errorMessage"`
		}
		if err := json.Unmarshal(r.Error, &errObj); err == nil && errObj.ErrorMessage != "" {
			errMsg = errObj.ErrorMessage
		} else if r.Reason != "" {
			errMsg = r.Reason
		}
	} else if r.Reason != "" {
		errMsg = r.Reason
	}
	return errors.New(errMsg)
}

type GenerateResponse struct {
	APIResponse
	Result struct {
		HME string `json:"hme"`
	} `json:"result"`
}

type DeactivateReactivateResponse struct {
	APIResponse
	Result struct {
		Message string `json:"message"`
	} `json:"result"`
}

type ReserveResponse struct {
	APIResponse
	Result struct {
		HME struct {
			HME   string `json:"hme"`
			Label string `json:"label"`
		} `json:"hme"`
	} `json:"result"`
}

func (c *Client) doPostRequest(endpoint string, payload any, target any) error {
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to build request payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to initialize http request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected http status %d: %s", resp.StatusCode, string(respBodyBytes))
	}

	if err := json.Unmarshal(respBodyBytes, target); err != nil {
		dump := string(respBodyBytes)
		if len(dump) > 1024 {
			dump = dump[:1024] + " ... (truncated)"
		}
		return fmt.Errorf("json parse error: %w\nraw body: %s", err, dump)
	}

	return nil
}

func (c *Client) ListEmails() (*EmailList, error) {
	req, err := http.NewRequest(http.MethodGet, BaseURLV2+"/list", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("api connection error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid status code: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	var emailList EmailList
	if err := json.NewDecoder(resp.Body).Decode(&emailList); err != nil {
		return nil, fmt.Errorf("failed to parse json: %w", err)
	}

	if !emailList.Success {
		return nil, fmt.Errorf("api returned success=false")
	}

	return &emailList, nil
}

func (c *Client) GenerateEmail() (*GenerateResponse, error) {
	payload := GenerateRequest{LangCode: "en-us"}

	var target GenerateResponse

	err := c.doPostRequest(BaseURLV1+"/generate", payload, &target)
	return &target, err
}

func (c *Client) ReserveEmail(email, label, note string) (*ReserveResponse, error) {
	payload := ReserveRequest{
		HME:   email,
		Label: label,
		Note:  note,
	}

	var target ReserveResponse

	err := c.doPostRequest(BaseURLV1+"/reserve", payload, &target)
	return &target, err
}

func (c *Client) GenerateOne(label, note string) (string, error) {
	genResp, err := c.GenerateEmail()
	if err != nil {
		return "", fmt.Errorf("generation critical error: %w", err)
	}

	if err := genResp.GetError(); err != nil {
		return "", fmt.Errorf("failed to generate email: %v", err)
	}

	email := genResp.Result.HME
	if email == "" {
		return "", errors.New("api returned success but hme field is empty")
	}

	reserveResp, err := c.ReserveEmail(email, label, note)
	if err != nil {
		return "", fmt.Errorf("reservation critical error: %w", err)
	}

	if err := reserveResp.GetError(); err != nil {
		return "", fmt.Errorf("failed to reserve email %q: %v", email, err)
	}

	return email, nil
}

func (c *Client) DeactivateEmail(anonymousID string) error {
	payload := DeactivateReactivateRequest{AnonymousID: anonymousID}

	var target DeactivateReactivateResponse

	err := c.doPostRequest(BaseURLV1+"/deactivate", payload, &target)
	if err != nil {
		return fmt.Errorf("requesting deactivate failed: %w", err)
	}

	if err := target.GetError(); err != nil {
		return fmt.Errorf("failed to deactivate email: %v", err)
	}

	return nil
}

func (c *Client) ReactivateEmail(anonymousID string) error {
	payload := DeactivateReactivateRequest{AnonymousID: anonymousID}
	var target DeactivateReactivateResponse

	err := c.doPostRequest(BaseURLV1+"/reactivate", payload, &target)
	if err != nil {
		return fmt.Errorf("requesting reactivate failed: %w", err)
	}

	if err := target.GetError(); err != nil {
		return fmt.Errorf("failed to reactivate email: %v", err)
	}

	return nil
}
