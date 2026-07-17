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
go get github.com/Catalizo/catzconnect-go
```

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

	catzconnect "github.com/Catalizo/catzconnect-go"
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

## 📬 Supported Operations

### Email Verification OTP

```go
catzconnect.SendInput{
	Type:     "Verification",
	Channel:  "Email",
	Template: "Otp",
	Identity: "user@domain.com",
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
