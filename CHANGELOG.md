# Changelog

## 2.0.0 — 2026-10-22

The first release since 1.5.0. It collects the entries below that were never published separately (1.7.0, 1.6.0); read them for the details. Every Catz Connect SDK (TypeScript, Go, PHP, Rust, Dart/Flutter) is released as 2.0.0 together.

**Highlights**

- Message status (`Messages().Get` / `List`), retry-safe sends (`SendInput.IdempotencyKey`), `Reference`, test keys, hosted OTP (`Otp()`), the management API (`Manage()`) and the push device registry (`PushDevices`).
- WhatsApp approved templates and media; Email topics and tracking; push broadcasts, topics and scheduled pushes (`Push()`); cross-channel opt-outs.
- Security options: https-only base URLs (`WithAllowInsecureHTTP` / `CATZCONNECT_ALLOW_INSECURE_HTTP=1` to opt out), `CheckBaseURL`, redacted `EnvValues` printing.
- User-Agent `catzconnect-go-sdk/2.0.0`.

**Upgrading from 1.5.x**

- **New module path (Go's rule for v2+):** the module is now `github.com/Catalizo/catzconnect-go-sdk/v2`. Change the import path to `github.com/Catalizo/catzconnect-go-sdk/v2` (the package is still `catzconnect`, so `catzconnect.Send(…)` etc. are unchanged), then `go get github.com/Catalizo/catzconnect-go-sdk/v2@v2.0.0` and `go mod tidy`. The old path keeps the 1.x releases.
- **https only** (since the 1.6.0 entry): a plain `http://` base URL to a host other than `localhost`, `127.0.0.1`, `::1` or `10.0.2.2` is refused before a request is sent; opt in to plain http for a test server only.
- The API answers a request whose timestamp is more than 30 s off with HTTP 400 `clock_skew` (was 401): keep the sending host's clock in sync (NTP).
- Everything else is backward compatible: existing sends post the same body.

## 1.7.0 (unreleased, included in 2.0.0)

**Message status, retry-safe sends, test keys, hosted OTP, management API, push device registry, WhatsApp templates.**

- `PushDevices.OpenedWithToken(ctx, publishableKey, messageID, token)`: reports
  an open with the device's FCM token (verified; the first verified report
  fires `push.opened`). `Opened` (no token) still records the open, but no
  webhook fires for it. Answers `{recorded, verified}`.
- `SendInput.IdempotencyKey` (Idempotency-Key header + `idempotency_key`) and
  `SendInput.Reference`; left out when empty.
- `SendPayload.WhatsApp` (`WhatsAppOptions`: `Language`, `Header`, `Body` or
  `NamedBody`, `Buttons`, `Media`) for approved templates (`Template("order_update")`)
  and media messages (`TemplateCustom` + `Media`); `VerifyWhatsAppOptions`,
  `VerifyWhatsAppMedia`. `SendPayload.Tracking` (Email).
- `client.Messages().Get / List`, `client.Otp().Start / Verify`,
  `client.Manage().Call` (+ `ListTemplates`, `SaveTemplate`, `SaveWebhook`,
  `AddSuppression`, `CreateKey`, `RotateKey`, `RevokeKey`, …),
  `client.PushDevices(baseURL).Register / Unregister / Opened`.
- `Code*` constants for the API's codes, `IsRetryableCode`, `Error.Details`.
- `WebhookEvents`, `ParseWebhookEvent[At]` (`Test` is true for test-key events).
- User-Agent `catzconnect-go-sdk/1.7.0`.


**Cross-channel opt-outs, topics and push broadcasts** (servers with per-channel suppressions):

- `SendPayload.Topic` (Email subscription topic key) and `SendPayload.Push`
  (`*PushOptions{Broadcast, Topic, SendAt}`, `PushSendAt(time.Time)`); the
  answer's `data` has `broadcast_id`. New `Push()`: `Broadcast`, `Cancel`,
  `Subscribe`, `Unsubscribe`, `Topics` (server keys only).
- `PushDevices().SubscribeTopics` / `UnsubscribeTopics`.
- `ListMessagesInput.BroadcastID`; `MessageStatus.BroadcastID` / `ResentFrom`.
- `Manage().AddSuppressionWith` / `RemoveSuppressionWith` (`SuppressionInput`:
  every channel, email `Topic`, or `ID`).
- Codes `CodeInvalidTopic`, `CodeTopicEmailOnly`, `CodeRecipientSuppressed`,
  `CodeWhatsAppOptedOut`, `CodeNotCancelable`, `CodeNotResendable`,
  `CodeContentGone`; `WebhookEvents` adds `email.complained`,
  `email.unsubscribed`, `push.opened`, `sms.inbound`, `sms.opt_out`,
  `whatsapp.opt_out`.

## 1.6.0 (unreleased, included in 2.0.0)

**Security options.**

- **https only.** A plain `http://` `CATZCONNECT_BASE_URL` to a host other
  than `localhost`, `127.0.0.1`, `::1` or `10.0.2.2` now fails with an
  `ErrValidation` error before a request is sent (it used to send the API key
  in clear text). Opt in with `CATZCONNECT_ALLOW_INSECURE_HTTP=1` or
  `catzconnect.New(catzconnect.WithAllowInsecureHTTP())`. New:
  `catzconnect.CheckBaseURL`.
- `EnvValues` implements `String`, `GoString` and `fmt.Formatter`: printing
  it with `%v`, `%+v`, `%#v` or `%s` no longer shows the API key or the
  private key.
- User-Agent `catzconnect-go-sdk/1.6.0`.

Earlier versions: see the git history.
