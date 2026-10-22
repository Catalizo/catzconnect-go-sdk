# CatzConnect SDK (Go)

A secure, minimal SDK for sending encrypted communication requests (e.g., email OTP) to the CatzConnect API. This is a Go port of the CatzConnect TypeScript SDK.

---

## ✨ Features

* 🔐 End-to-end payload encryption (X25519 + BLAKE2b + ChaCha20-Poly1305)
* ⚡ Stateless design (no `init()` required)
* 🧠 Automatic payload validation (based on type + channel + template)
* 🔑 API key via environment (Bearer token)
* 🧱 Clean, extensible architecture

---

## 📦 Installation

```bash
go get github.com/Catalizo/catzconnect-go-sdk/v2@v2.0.0
```

> **Upgrading from 1.x:** from 2.0.0 the module path ends in `/v2` (Go's rule
> for major versions 2 and up). Change the import path to
> `github.com/Catalizo/catzconnect-go-sdk/v2` (the package is still named
> `catzconnect`), then run `go get github.com/Catalizo/catzconnect-go-sdk/v2@v2.0.0`
> and `go mod tidy`. Nothing else in your code has to change.

---

## ⚙️ Environment Setup

Set the following environment variables (for example, from a `.env` loaded by your process manager):

```env
CATZCONNECT_API_KEY=your_api_key
CATZCONNECT_PRIVATE_KEY=your_base64_private_key
CATZCONNECT_SERVER_PUBLIC_KEY=server_base64_public_key
```

> ⚠️ Never expose these values in frontend/public environments.

You can also pass them explicitly via `*EnvValues` instead of the environment (see below).

---

## 🚀 Usage

```go
package main

import (
	"log"

	catzconnect "github.com/Catalizo/catzconnect-go-sdk/v2"
)

func main() {
	_, err := catzconnect.Send(catzconnect.SendInput{
		Type:     "Verification",
		Channel:  "Email",
		Template: "Otp",
		Identity: "user@domain.com",
		Payload: catzconnect.SendPayload{
			To:  "user@example.com",
			Otp: "123456",
		},
	}, nil)
	if err != nil {
		log.Fatal(err)
	}
}
```

Passing credentials explicitly (instead of reading them from the environment):

```go
res, err := catzconnect.Send(input, &catzconnect.EnvValues{
	APIKey:          "your_api_key",
	PrivateKey:      "your_base64_private_key",
	ServerPublicKey: "server_base64_public_key",
})
```

Typed constants are provided if you prefer them over string literals:

```go
catzconnect.SendInput{
	Type:     catzconnect.MessageTypeVerification,
	Channel:  catzconnect.ChannelEmail,
	Template: catzconnect.TemplateOtp,
	// ...
}
```

---

## 🌐 Language of the default OTP templates

`SendInput.Language` (optional, top level of the request next to channel and
template) picks the language of the **built-in** Otp/Verification templates
for Email, SMS, Voice and WhatsApp: `en`, `de`, `ja`, `ar` or `hi`. A BCP-47
tag such as `"de-DE"` is accepted and sent as `"de"`; anything else fails
before the request with a validation `*Error` (`Unsupported 'language' "fr":
use one of en, de, ja, ar, hi …`). Empty leaves the field out, so the server
uses English as before. Templates you created in the panel are not affected.

```go
catzconnect.Send(catzconnect.SendInput{
	Type: catzconnect.MessageTypeVerification, Channel: catzconnect.ChannelEmail, Template: catzconnect.TemplateOtp,
	Identity: "noreply@yourdomain.com",
	Language: "de-DE", // sent as "language": "de"
	Payload:  catzconnect.SendPayload{To: "user@example.com", Otp: "123456"},
}, nil)

// A client whose sends default to Japanese when Language is empty.
client := catzconnect.New(catzconnect.WithDefaultLanguage("ja"))
```

`NormalizeLanguage("de-DE")` returns `"de"`; `SupportedLanguages` lists the codes.

---

## 📬 Supported Operations

### Email Verification OTP

```go
catzconnect.SendInput{
	Type:     "Verification",
	Channel:  "Email",
	Template: "Otp",
	Identity: "user@domain.com",
	Language: "de",              // optional: localized default template (en|de|ja|ar|hi, or "de-DE")
	Payload: catzconnect.SendPayload{
		To:  "user@example.com", // required, valid email
		Otp: "123456",           // required, exactly 6 digits
	},
}
```

### Email Transactional

```go
catzconnect.SendInput{
	Type:     "Transactional",
	Channel:  "Email",
	Template: "Custom",
	Identity: "user@domain.com",
	Payload: catzconnect.SendPayload{
		To:      "user@example.com", // required, valid email
		Subject: "Welcome",          // required
		Body:    "Hello there",      // required
	},
}
```

---

## 🔐 How It Works

1. **Validate Input**

   * Ensures required fields are present
   * Validates email format

2. **Encrypt Payload**

   * Uses X25519 (ECDH) + ChaCha20-Poly1305
   * Derived symmetric key via BLAKE2b

3. **Send Request**

   * `POST /sdk/send`
   * Authorization via Bearer token

4. **Server Processes Securely**

---

## ❌ Error Handling

`Send` returns an `error` for:

* Missing environment variables / keys
* Invalid payload (e.g., missing `to`, invalid email)
* Encryption failure
* HTTP errors (non-2xx response)

Example:

```go
res, err := catzconnect.Send(input, nil)
if err != nil {
	log.Println(err)
	return
}
_ = res
```

---

## 🧩 Example Response

`Send` returns the decoded JSON response as a `map[string]any`:

```json
{
  "status": "success"
}
```

---

## 🛠 Development

```bash
go mod tidy
go build ./...
go vet ./...
go test ./...
```

---

## WhatsApp

```go
catzconnect.Send(catzconnect.SendInput{
	Type:     catzconnect.MessageTypeVerification,
	Channel:  catzconnect.ChannelWhatsApp,
	Template: catzconnect.TemplateOtp,
	Identity: "919578456444", // your connected WhatsApp number
	Language: "hi",           // optional: Meta template language code to use
	Payload: catzconnect.SendPayload{
		To:  "+91 98765 43210", // phone number with country code
		Otp: "123456",
	},
}, nil)
```

`Transactional` / `Custom` takes `To`, `Body`, and an optional `Subject` sent as a
bold first line. With `Language` the server asks Meta for the Authentication
template in that language — it must be approved in that language on your
WhatsApp Business Account, or Meta rejects the message. OTPs need an approved Authentication template; custom messages
only reach people who messaged your number in the last 24 hours.

## Push notifications (end-to-end encrypted)

```go
catzconnect.Send(catzconnect.SendInput{
	Type:     catzconnect.MessageTypeNotification,
	Channel:  catzconnect.ChannelPush,
	Template: catzconnect.TemplateNotification,
	Identity: "your-firebase-project-id",
	Payload: catzconnect.SendPayload{
		To:        fcmToken,
		DeviceKey: devicePublicKey, // from the app; seals the content to that device
		Title:     "Order shipped",
		Body:      "Arriving Friday",
		Data:      map[string]string{"order": "42"},
	},
}, nil)
```

The device generates its keys and decrypts — see `PUSH.md` for web, Android and iOS.

## Errors

Every error `Send` returns is a `*catzconnect.Error`. Branch on the kind with
`errors.Is`, or read the server's response with `errors.As`:

```go
_, err := catzconnect.Send(input, nil)
switch {
case errors.Is(err, catzconnect.ErrValidation):
	// the payload was refused before sending
case errors.Is(err, catzconnect.ErrMissingEnv):
	// a CATZCONNECT_* variable is unset
case errors.Is(err, catzconnect.ErrAPI):
	var ce *catzconnect.Error
	errors.As(err, &ce)
	log.Println(ce.Status, ce.Body) // what the server said
}
```

## Keys and configuration

Keys are read leniently: surrounding whitespace, a trailing newline, URL-safe
base64 and missing padding are all accepted, as `.env` files and secret managers
commonly introduce them. The API key and base URL are trimmed, and a trailing
slash on `CATZCONNECT_BASE_URL` is removed. Requests time out after 30 seconds.

`CATZCONNECT_BASE_URL` must be `https://`. Plain `http://` is accepted only for
`localhost`, `127.0.0.1`, `[::1]` and `10.0.2.2` (local development); any other
`http://` URL fails with an `ErrValidation` error before anything is sent, since
the API key would travel unencrypted. To allow it anyway set
`CATZCONNECT_ALLOW_INSECURE_HTTP=1` or use
`catzconnect.New(catzconnect.WithAllowInsecureHTTP())`. `catzconnect.CheckBaseURL`
applies the same rule.

Printing an `EnvValues` (`%v`, `%+v`, `%#v`, `String()`) never shows the API key
or the private key.

## Email templates from the panel

```go
_, err := catzconnect.Send(catzconnect.SendInput{
	Channel:  catzconnect.ChannelEmail,
	Type:     catzconnect.MessageTypeTransactional,
	Template: catzconnect.Template("Order shipped"), // the template's name in the panel
	Identity: "noreply@yourdomain.com",
	Payload: catzconnect.SendPayload{
		To:   "user@example.com",
		Data: map[string]string{"name": "Ann", "order_id": "A-1042"},
	},
}, nil)
```

## Push to a user's registered devices

Set `ExternalUserID` instead of `To` to send to every device your app
registered for that user with `POST /push/register`. `DeviceKey` must be
empty; each device's registered key is used.

```go
Payload: catzconnect.SendPayload{ExternalUserID: "user-42", Title: "Order shipped", Body: "It's on the way"},
```

## Verifying webhooks

```go
body, _ := io.ReadAll(r.Body)
if !catzconnect.VerifyWebhookSignature(body, r.Header.Get("Catz-Signature"), os.Getenv("CATZCONNECT_WEBHOOK_SECRET")) {
	http.Error(w, "bad signature", http.StatusBadRequest)
	return
}
```

`VerifyWebhookSignatureAt` takes an explicit clock and tolerance.

## SMS

`To` is an E.164 number (`+` and the country code; spaces, dashes, dots and
brackets are removed). `Identity` is ignored. `SendSMS` returns the typed result.

```go
// One-time code: Verification + Otp (up to 15 letters or digits).
res, err := catzconnect.SendSMS(catzconnect.SendInput{
	Type:     catzconnect.MessageTypeVerification,
	Template: catzconnect.TemplateOtp,
	Payload:  catzconnect.SendPayload{To: "+14155550100", Otp: "123456"},
}, nil)
if err != nil {
	log.Fatal(err)
}
log.Println(res.MessageID, res.Segments, res.Encoding) // "…", 1, "gsm7"

// The default OTP text in German ("de-DE" is sent as "de").
_, err = catzconnect.SendSMS(catzconnect.SendInput{
	Type: catzconnect.MessageTypeVerification, Template: catzconnect.TemplateOtp, Language: "de-DE",
	Payload: catzconnect.SendPayload{To: "+4915112345678", Otp: "123456"},
}, nil)

// Free text: Transactional or Promotional + Custom. India needs a DLT id.
_, err = catzconnect.SendSMS(catzconnect.SendInput{
	Type:     catzconnect.MessageTypePromotional,
	Template: catzconnect.TemplateCustom,
	Payload: catzconnect.SendPayload{
		To:            "+91 98765 43210",
		Body:          "Sale ends tonight",
		DLTTemplateID: "1207161234567890123",
		Gateway:       "MSG91 India", // optional: one of the key's gateways (id or label)
		Sender:        "CATZIN",      // optional: only with Gateway, when it allows overrides
	},
}, nil)

// An SMS template from the panel, filled from Data.
_, err = catzconnect.SendSMS(catzconnect.SendInput{
	Type:     catzconnect.MessageTypeTransactional,
	Template: catzconnect.Template("Order shipped"),
	Payload:  catzconnect.SendPayload{To: "+14155550100", Data: map[string]string{"name": "Ann"}},
}, nil)
```

`Send` with `Channel: catzconnect.ChannelSMS` works too; read its result with
`ParseSMSResult`. Validation matches the server: E.164 `To`; `Otp` only with
`Verification`, `Custom` only with `Transactional`/`Promotional`; `Sender` only
with `Gateway`; numeric `DLTTemplateID`; Custom bodies at most 10 segments
(1,530 GSM-7 / 670 UCS-2 characters) and a DLT id for +91. `AnalyzeSMS(text)`
reports the encoding and segments; `NormalizeE164` normalises a number.

## Phone calls (Voice)

`ChannelVoice` (`"Voice"`; `"voice"` is accepted) places a phone call that
speaks the message. `To` is E.164; `Identity` is ignored.

```go
// A one-time code, read out digit by digit.
res, err := catzconnect.SendVoice(catzconnect.SendInput{
	Channel: catzconnect.ChannelVoice, Type: catzconnect.MessageTypeVerification, Template: catzconnect.TemplateOtp,
	Payload: catzconnect.SendPayload{
		To: "+14155550100", Otp: "482913",
		Data: map[string]string{"language": "en-US", "voice": "female", "repeat": "2"},
	},
}, nil)
fmt.Println(res.MessageID) // voice.* webhook events carry it

// The default OTP script in Japanese; TTS language and voice follow it.
// Data["language"], when set, still overrides the spoken language.
_, err = catzconnect.SendVoice(catzconnect.SendInput{
	Type: catzconnect.MessageTypeVerification, Template: catzconnect.TemplateOtp, Language: "ja",
	Payload: catzconnect.SendPayload{To: "+819012345678", Otp: "482913"},
}, nil)

// Free text (≤ 1,000 characters) through one of the key's phone providers.
_, err = catzconnect.Send(catzconnect.SendInput{
	Channel: catzconnect.ChannelVoice, Type: catzconnect.MessageTypeNotification, Template: catzconnect.TemplateCustom,
	Payload: catzconnect.SendPayload{To: "+919876543210", Body: "Your table is ready.", Provider: "Exotel India"},
}, nil)
```

Checked before sending: E.164 `To`; `Otp` needs `Otp` (≤ 15 letters/digits);
`Custom` needs `Body` (1–1,000 characters, no control characters);
`Data["language"]` like `en-US`, `Data["voice"]` `female`/`male`/a provider
voice, `Data["repeat"]` `1`–`3`. Any other template name is a Voice template
from the panel, filled from `Data`.

## Voice & Video calls

Calls between your app's users. Your backend creates the call and a
short-lived user token; the browser (or app) connects with the token. All
requests use the same key, encryption and headers as `Send`.

```go
ctx := context.Background()
calls := catzconnect.Default.Calls() // .WithEnv(&catzconnect.EnvValues{...}) for explicit keys

call, err := calls.Create(ctx, catzconnect.CreateCallInput{
	Mode: catzconnect.CallModeVideo, // or CallModeAudio
	From: "alice",                   // your user ids, ≤ 128 characters
	To:   []string{"bob"},           // 1–49, unique, not including From
	// Engine: "auto" | "catz" | "<provider id or label>",
	// Ring: catzconnect.Bool(true), Metadata: map[string]any{"order": "A-1042"}, MaxParticipants: 4,
})

tok, err := calls.Token(ctx, "alice", 3600) // 60–86400 s; 0 = server default (3600)
// send tok.Token and tok.WSURL to alice's browser

detail, err := calls.Get(ctx, call.ID) // participants, DurationSecs, BilledMinutes, EndReason
_, err = calls.End(ctx, call.ID, "support_closed")
```

Refusals are `*catzconnect.Error` with `Kind == KindAPI`, `Status`, and
`Code` (`plan_required`, `minutes_cap_reached`, `engine_unavailable`,
`too_many_participants`, `provider_not_allowed`):

```go
var ce *catzconnect.Error
if errors.As(err, &ce) && ce.Code == "minutes_cap_reached" { /* … */ }
```

Inputs are checked first, as the server does: mode `audio`/`video`; user
ids non-empty, ≤ 128 characters, no control characters; `To` 1–49 unique
ids without `From`; metadata ≤ 4 KB of JSON; TTL 60–86400.

### In the browser

Hand the token (never the API key) to the user's page and use the
TypeScript SDK's browser entry, `catzconnect/call` (no dependencies; native
WebRTC):

```ts
import { CatzCallClient } from "catzconnect/call";

const client = new CatzCallClient({ getToken: () => fetch("/call-token").then((r) => r.text()), url: wsUrl });
client.on("incoming", ({ callId }) => client.accept(callId));
client.on("track", ({ pid, stream }) => (videoFor(pid).srcObject = stream));
client.on("local-stream", ({ stream }) => (selfView.srcObject = stream));
await client.connect();
await client.join(callId, { audio: true, video: true });
```

It keeps the call up by itself: direct path first, relay fallback (UDP →
TCP → TLS), ICE restart on failure or network change, WebSocket reconnect
with resume, token renewal through `getToken`. See the TypeScript README for
every event and method, and a React example.

**Provider hand-off.** When the call runs on a provider engine (Agora,
Twilio Video, LiveKit, …), the browser client emits `provider` with
`{ name, room, token, app_id?, url?, extra? }` instead of opening peer
connections; join with that provider's own SDK.

## Message status, retry-safe sends, test keys, hosted OTP (2.0.0)

```go
c := catzconnect.New()
res, err := c.Send(catzconnect.SendInput{Channel: catzconnect.ChannelSMS, Type: catzconnect.MessageTypeTransactional,
    Template: catzconnect.TemplateCustom, Payload: catzconnect.SendPayload{To: "+919876543210", Body: "Shipped", DLTTemplateID: "1107…"},
    IdempotencyKey: "ship-42", Reference: "order-42"}, nil)

msg, err := c.Messages().Get(ctx, res["data"].(map[string]any)["message_id"].(string))
start, err := c.Otp().Start(ctx, catzconnect.OtpStartInput{Channel: catzconnect.ChannelSMS, To: "+919876543210"})
_, err = c.Otp().Verify(ctx, catzconnect.OtpVerifyInput{VerificationID: start.VerificationID, Code: typed})
var ce *catzconnect.Error
if errors.As(err, &ce) && ce.Code == catzconnect.CodeOtpInvalid { /* ce.Details["attempts_remaining"] */ }
```

WhatsApp approved templates: `Template("order_update")` with `Payload.WhatsApp =
&catzconnect.WhatsAppOptions{Language: "en_US", Body: []string{"Ravi"}}`; media:
`TemplateCustom` with `WhatsApp.Media`. Test keys (panel → Test key) are
validated but never delivered or billed (`data.test`, webhook `Test`).
Management API: `c.Manage().Call(ctx, "templates", "save", map[string]any{...})`
with a key that has the management scope. Push devices:
`c.PushDevices("").Register(ctx, catzconnect.RegisterDeviceInput{...})`.

## Email topics, push broadcasts and topics (2.0.0)

```go
// Email under a subscription topic: its opt-outs apply.
c.Send(catzconnect.SendInput{Channel: catzconnect.ChannelEmail, Type: catzconnect.MessageTypeTransactional,
	Template: catzconnect.TemplateCustom, Identity: "news",
	Payload: catzconnect.SendPayload{To: "ana@example.com", Subject: "October", Body: "<p>…</p>", Topic: "newsletter"}}, nil)

// Push to a topic, later (server key); data.broadcast_id comes back.
res, _ := c.Send(catzconnect.SendInput{Channel: catzconnect.ChannelPush, Type: catzconnect.MessageTypeNotification,
	Template: catzconnect.TemplateNotification, Identity: "my-firebase",
	Payload: catzconnect.SendPayload{Body: "Today only", Push: &catzconnect.PushOptions{Topic: "deals",
		SendAt: catzconnect.PushSendAt(time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC))}}}, nil)
id := res["data"].(map[string]any)["broadcast_id"].(string)
b, _ := c.Push().Broadcast(ctx, id) // b.Status, b.Queued, b.OverQuota
c.Push().Subscribe(ctx, catzconnect.PushTopicChange{Identity: "my-firebase", Topic: "deals", ExternalUserID: "u1"})
c.PushDevices("").SubscribeTopics(ctx, publishableKey, fcmToken, []string{"deals"}) // in the app
c.Manage().AddSuppressionWith(ctx, catzconnect.SuppressionInput{Channel: "whatsapp", Address: "+919876543210"})
```

App and test keys are refused for broadcasts / topics (`CodeNotAllowedForAppKeys`,
`CodeNotAllowedForTestKeys`). `WebhookEvents` includes `email.complained`,
`email.unsubscribed`, `push.opened`, `sms.inbound`, `sms.opt_out`, `whatsapp.opt_out`.
