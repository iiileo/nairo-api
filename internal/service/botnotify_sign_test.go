package service

import (
	"strings"
	"testing"
)

func TestFeishuSign(t *testing.T) {
	sign, err := feishuSign("1596607260", "test_secret")
	if err != nil {
		t.Fatal(err)
	}
	if sign == "" {
		t.Fatal("empty sign")
	}
}

func TestDingtalkSignedURL(t *testing.T) {
	u, err := dingtalkSignedURL("https://oapi.dingtalk.com/robot/send?access_token=abc", "SECxxx")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(u, "timestamp=") || !strings.Contains(u, "sign=") {
		t.Fatalf("missing sign params: %s", u)
	}
	plain, err := dingtalkSignedURL("https://oapi.dingtalk.com/robot/send?access_token=abc", "")
	if err != nil || plain != "https://oapi.dingtalk.com/robot/send?access_token=abc" {
		t.Fatalf("plain url changed: %v %s", err, plain)
	}
}
