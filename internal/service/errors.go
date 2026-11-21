package service

import "errors"

var (
	ErrNotFound         = errors.New("not_found")
	ErrConflict         = errors.New("conflict")
	ErrInvalid          = errors.New("invalid")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrBusy             = errors.New("channel_busy")
	ErrNoChannelDevice  = errors.New("no_channel_device")
	ErrNoPayCode        = errors.New("no_pay_code")
	ErrBoundElsewhere   = errors.New("bound_elsewhere")
	ErrInsufficient     = errors.New("insufficient_balance")
	ErrAlreadyPaid      = errors.New("already_paid")
	ErrRateLimited      = errors.New("rate_limited")
	ErrCodeInvalid      = errors.New("code_invalid")
	ErrCaptchaFailed    = errors.New("captcha_failed")
	ErrRegisterDisabled = errors.New("register_disabled")
)
