package geetest

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const validateURL = "https://gcaptcha4.geetest.com/validate"

// Proof 前端回传的 GT4 校验参数。
type Proof struct {
	LotNumber     string `json:"lot_number"`
	CaptchaOutput string `json:"captcha_output"`
	PassToken     string `json:"pass_token"`
	GenTime       string `json:"gen_time"`
}

type validateResp struct {
	Result string `json:"result"`
	Reason string `json:"reason"`
}

// Validate 服务端二次校验。captchaID/key 为空时返回 error（调用方决定是否跳过）。
func Validate(ctx context.Context, captchaID, captchaKey string, p Proof) error {
	captchaID = strings.TrimSpace(captchaID)
	captchaKey = strings.TrimSpace(captchaKey)
	if captchaID == "" || captchaKey == "" {
		return fmt.Errorf("geetest not configured")
	}
	p.LotNumber = strings.TrimSpace(p.LotNumber)
	p.CaptchaOutput = strings.TrimSpace(p.CaptchaOutput)
	p.PassToken = strings.TrimSpace(p.PassToken)
	p.GenTime = strings.TrimSpace(p.GenTime)
	if p.LotNumber == "" || p.CaptchaOutput == "" || p.PassToken == "" || p.GenTime == "" {
		return fmt.Errorf("missing captcha params")
	}

	mac := hmac.New(sha256.New, []byte(captchaKey))
	_, _ = mac.Write([]byte(p.LotNumber))
	signToken := hex.EncodeToString(mac.Sum(nil))

	form := url.Values{}
	form.Set("lot_number", p.LotNumber)
	form.Set("captcha_output", p.CaptchaOutput)
	form.Set("pass_token", p.PassToken)
	form.Set("gen_time", p.GenTime)
	form.Set("sign_token", signToken)

	reqURL := validateURL + "?captcha_id=" + url.QueryEscape(captchaID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	cli := &http.Client{Timeout: 5 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return fmt.Errorf("geetest request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("geetest http %d", resp.StatusCode)
	}
	var out validateResp
	if err := json.Unmarshal(body, &out); err != nil {
		return fmt.Errorf("geetest json: %w", err)
	}
	if out.Result != "success" {
		if out.Reason != "" {
			return fmt.Errorf("geetest fail: %s", out.Reason)
		}
		return fmt.Errorf("geetest fail")
	}
	return nil
}
