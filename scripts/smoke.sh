#!/usr/bin/env bash
# 冒烟：console 双凭证 + admin + app 回调（push / heartbeat）
set -euo pipefail

BASE="${BASE:-http://127.0.0.1:8088}"
PASS='Passw0rd!'
EMAIL="smoke_$(date +%s)@example.com"

need() { command -v "$1" >/dev/null || { echo "need $1"; exit 1; }; }
need curl
need python3

json_get() {
  python3 -c 'import sys,json; d=json.load(sys.stdin); print(d'"$1"')'
}

md5_hex() {
  python3 -c 'import hashlib,sys; print(hashlib.md5(sys.argv[1].encode()).hexdigest())' "$1"
}

echo "== health =="
curl -sf "$BASE/healthz" | python3 -m json.tool >/dev/null
echo ok

echo "== send-code =="
GT_ENABLED=$(curl -sf "$BASE/api/v1/public/geetest" | python3 -c 'import sys,json; print(json.load(sys.stdin).get("enabled", False))' 2>/dev/null || echo False)
LOG="${NAIRO_LOG:-/tmp/nairo-api.log}"
CODE=""
if [[ "$GT_ENABLED" == "True" ]]; then
  echo "skip (geetest enabled; set SMOKE_REGISTER_CODE to continue)"
  CODE="${SMOKE_REGISTER_CODE:-}"
else
  curl -sf -X POST "$BASE/api/v1/console/register/send-code" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"$EMAIL\"}" >/dev/null
  # 开发态验证码打在服务日志；测试环境也可用固定注入。此处从最近 API 日志解析。
  if [[ -f "$LOG" ]]; then
    CODE=$(python3 - <<PY
import re
text=open("$LOG").read()
ms=re.findall(r"验证码是 (\d{6})", text)
print(ms[-1] if ms else "")
PY
)
  fi
fi
if [[ -z "$CODE" ]]; then
  echo "cannot parse verification code from $LOG; set NAIRO_LOG / SMOKE_REGISTER_CODE or ensure SMTP_DEV_PRINT logs"
  exit 1
fi
echo "email=$EMAIL code=$CODE"

echo "== register =="
REG=$(curl -sf -X POST "$BASE/api/v1/console/register" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"code\":\"$CODE\",\"password\":\"$PASS\",\"confirm_password\":\"$PASS\"}")
TOKEN=$(echo "$REG" | json_get '["token"]')
RENEW=$(echo "$REG" | json_get '["renew"]')
test -n "$TOKEN" && test -n "$RENEW"
echo register_ok

echo "== me =="
curl -sf "$BASE/api/v1/console/me" -H "Authorization: Bearer $TOKEN" | json_get '["user"]["email"]'

echo "== renew =="
REF=$(curl -sf -X POST "$BASE/api/v1/console/refresh" \
  -H 'Content-Type: application/json' \
  -d "{\"renew\":\"$RENEW\"}")
TOKEN=$(echo "$REF" | json_get '["token"]')
RENEW2=$(echo "$REF" | json_get '["renew"]')
test "$RENEW" != "$RENEW2"
OLD_HTTP=$(curl -s -o /tmp/old_renew.json -w '%{http_code}' -X POST "$BASE/api/v1/console/refresh" \
  -H 'Content-Type: application/json' \
  -d "{\"renew\":\"$RENEW\"}")
test "$OLD_HTTP" = "401"
RENEW="$RENEW2"
echo renew_ok

echo "== login =="
LOGIN=$(curl -sf -X POST "$BASE/api/v1/console/login" \
  -H 'Content-Type: application/json' \
  -d "{\"login\":\"$EMAIL\",\"password\":\"$PASS\"}")
TOKEN=$(echo "$LOGIN" | json_get '["token"]')
RENEW=$(echo "$LOGIN" | json_get '["renew"]')
echo login_ok

echo "== create account (wechat+alipay) =="
ACC=$(curl -sf -X POST "$BASE/api/v1/console/accounts" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"channels":["wechat","alipay"],"name":"smoke-device"}')
CB=$(echo "$ACC" | json_get '["bind"]["callback_id"]')
SECRET=$(echo "$ACC" | json_get '["bind"]["secret"]')
CB_URL=$(echo "$ACC" | json_get '["bind"]["push_url"]')
HB_URL=$(echo "$ACC" | json_get '["bind"]["heartbeat_url"]')
CHS=$(echo "$ACC" | python3 -c 'import sys,json; print(",".join(json.load(sys.stdin)["bind"]["channels"]))')
test "$CHS" = "alipay,wechat" -o "$CHS" = "wechat,alipay" || test -n "$CHS"
echo "callback_id=$CB channels=$CHS"
echo "push_url=$CB_URL"
echo "heartbeat_url=$HB_URL"
echo "$ACC" | python3 -c 'import sys,json; b=json.load(sys.stdin)["bind"]; assert b["secret"] and b["push_url"] and b["heartbeat_url"] and set(b["channels"])=={"wechat","alipay"}'

echo "== app heartbeat =="
T=$(date +%s)
NONCE="hb_$RANDOM$RANDOM"
SIGN=$(md5_hex "${CB}${T}${NONCE}${SECRET}")
curl -sf -X POST "$BASE/app/heartbeat" \
  -H 'Content-Type: application/json' \
  -d "{\"callback_id\":\"$CB\",\"t\":$T,\"nonce\":\"$NONCE\",\"sign\":\"$SIGN\",\"conn_type\":\"http\"}" \
  | python3 -c 'import sys,json; assert json.load(sys.stdin).get("ok") is True'
echo heartbeat_ok

echo "== app push wechat =="
T=$(date +%s)
NONCE="push_$RANDOM$RANDOM"
PRICE="0.01"
CHANNEL="wechat"
SIGN=$(md5_hex "${CB}${T}${CHANNEL}${PRICE}${NONCE}${SECRET}")
curl -sf -X POST "$BASE/app/push/$CB" \
  -H 'Content-Type: application/json' \
  -d "{\"t\":$T,\"channel\":\"$CHANNEL\",\"price\":\"$PRICE\",\"nonce\":\"$NONCE\",\"sign\":\"$SIGN\"}" \
  | python3 -c 'import sys,json; assert json.load(sys.stdin).get("code")==0'
echo push_wechat_ok

echo "== app push alipay =="
T=$(date +%s)
NONCE="push_ali_$RANDOM$RANDOM"
PRICE="0.02"
CHANNEL="alipay"
SIGN=$(md5_hex "${CB}${T}${CHANNEL}${PRICE}${NONCE}${SECRET}")
curl -sf -X POST "$BASE/app/push/$CB" \
  -H 'Content-Type: application/json' \
  -d "{\"t\":$T,\"channel\":\"$CHANNEL\",\"price\":\"$PRICE\",\"nonce\":\"$NONCE\",\"sign\":\"$SIGN\"}" \
  | python3 -c 'import sys,json; assert json.load(sys.stdin).get("code")==0'
echo push_alipay_ok

echo "== bad sign rejected =="
BAD_HTTP=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/app/push/$CB" \
  -H 'Content-Type: application/json' \
  -d "{\"t\":$T,\"channel\":\"$CHANNEL\",\"price\":\"$PRICE\",\"nonce\":\"$NONCE\",\"sign\":\"deadbeef\"}")
test "$BAD_HTTP" = "401"
echo bad_sign_ok

echo "== orphans visible =="
curl -sf "$BASE/api/v1/console/orphans" -H "Authorization: Bearer $TOKEN" >/tmp/nairo_orphans.json
python3 - <<'PY'
import json
d=json.load(open("/tmp/nairo_orphans.json"))
assert any(i.get("pay_amount_cents")==1 for i in d.get("items",[])), d
print("orphans_ok", len(d["items"]))
PY

echo "== ignore orphan =="
ORPHAN_ID=$(python3 -c 'import json; d=json.load(open("/tmp/nairo_orphans.json")); print(next(i["id"] for i in d["items"] if i.get("status")=="open"))')
curl -sf -X POST "$BASE/api/v1/console/orphans/$ORPHAN_ID/ignore" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"remark":"smoke ignore"}' \
  | python3 -c 'import sys,json; assert json.load(sys.stdin).get("ok") is True'
echo ignore_orphan_ok

echo "== delete monitoring device =="
ACC2=$(curl -sf -X POST "$BASE/api/v1/console/accounts" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"channels":["wechat"],"name":"to-delete"}')
AID2=$(echo "$ACC2" | json_get '["account"]["id"]')
curl -sf -X DELETE "$BASE/api/v1/console/accounts/$AID2" \
  -H "Authorization: Bearer $TOKEN" >/dev/null
curl -sf "$BASE/api/v1/console/accounts" -H "Authorization: Bearer $TOKEN" \
  | python3 -c 'import sys,json; ids=[i["id"] for i in json.load(sys.stdin)["items"]]; assert "'"$AID2"'" not in ids'
echo delete_account_ok

echo "== admin =="
ADM=$(curl -sf -X POST "$BASE/api/v1/admin/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}')
AT=$(echo "$ADM" | json_get '["token"]')
curl -sf "$BASE/api/v1/admin/users" -H "Authorization: Bearer $AT" \
  | python3 -c 'import sys,json; assert len(json.load(sys.stdin)["items"])>=1'
echo admin_ok

echo "== order + manual set status =="
AID=$(echo "$ACC" | json_get '["account"]["id"]')
curl -sf -X POST "$BASE/api/v1/console/pay-codes" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"account_id\":\"$AID\",\"code_type\":\"float\",\"image_url\":\"https://example.com/qr.png\"}" >/dev/null
USER_ID=$(curl -sf "$BASE/api/v1/console/me" -H "Authorization: Bearer $TOKEN" | json_get '["user"]["id"]')
curl -sf -X POST "$BASE/api/v1/admin/users/$USER_ID/recharge" \
  -H "Authorization: Bearer $AT" \
  -H 'Content-Type: application/json' \
  -d '{"amount_cents":10000,"remark":"smoke"}' >/dev/null
CRED=$(curl -sf -X POST "$BASE/api/v1/console/credential/reset" -H "Authorization: Bearer $TOKEN")
AK=$(echo "$CRED" | json_get '["access_key"]')
AS=$(echo "$CRED" | json_get '["api_secret"]')
OTN="smoke_$(date +%s)_$RANDOM"
BODY=$(python3 -c 'import json; print(json.dumps({"out_trade_no":"'"$OTN"'","amount_cents":100,"channel":"wechat"}))')
TS=$(date +%s)
SIGN=$(python3 - <<PY
import hashlib,hmac,json
body='''$BODY'''.encode()
canon="POST\n/api/v1/open/orders\n$TS\n"+hashlib.sha256(body).hexdigest()
print(hmac.new(b'''$AS''', canon.encode(), hashlib.sha256).hexdigest())
PY
)
ORDER=$(curl -sf -X POST "$BASE/api/v1/open/orders" \
  -H "Content-Type: application/json" \
  -H "X-Nairo-Key: $AK" \
  -H "X-Nairo-Timestamp: $TS" \
  -H "X-Nairo-Sign: $SIGN" \
  -d "$BODY")
OID=$(echo "$ORDER" | json_get '["order_id"]')
test "$(echo "$ORDER" | json_get '["status"]')" = "pending"
curl -sf -X POST "$BASE/api/v1/console/orders/$OID/status" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"status":"paid","remark":"用户已付款但 App 未上报，人工确认"}' \
  | python3 -c 'import sys,json; assert json.load(sys.stdin)["status"]=="paid"'
echo set_order_status_ok

echo "== spa =="
test "$(curl -s -o /dev/null -w '%{http_code}' "$BASE/console/")" = "200"
test "$(curl -s -o /dev/null -w '%{http_code}' "$BASE/admin_nairo/")" = "200"

echo ALL_SMOKE_OK
