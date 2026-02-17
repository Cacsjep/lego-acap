# Lego ACAP

An [ACAP](https://developer.axis.com/) that downloads and invokes [lego](https://github.com/go-acme/lego) to obtain SSL/TLS certificates via DNS-01 challenge on Axis network cameras.

Downloads the lego binary on first start, provides a web UI for configuration, and supports all 100+ DNS providers that lego offers. Supports `armv7hf` and `aarch64`.

## Quick Start

1. Install the `.eap` on your Axis camera
2. Open the app settings page
3. Wait for the lego binary to download automatically
4. Configure email, domain(s), DNS provider, and environment variables
5. Click **Obtain**

For provider-specific environment variables, see the [lego DNS provider docs](https://go-acme.github.io/lego/dns/).

## Web UI Reference

### Status Bar

The top bar provides connection status and all primary actions.

| Element | Description |
|---------|-------------|
| Connection indicator | Green dot = WebSocket connected, red dot = disconnected. Reconnects automatically every 3 seconds. |
| Certificate chip | Shows the primary domain when a certificate exists. Click to open the Certificate Details dialog. Shows "No certificate" when none is present. |
| **Download** / **Binary ready** | Downloads the lego binary from GitHub. Shows a percentage while downloading. Turns green with "Binary ready" once the binary is available. |
| **Obtain** | Starts a new certificate issuance via `lego run`. Disabled while lego is not downloaded or already running. |
| **Renew** | Renews the existing certificate via `lego renew`. Disabled when no certificate exists or lego is already running. |
| **Install** | Uploads the certificate and private key to the camera via VAPIX/ONVIF and configures it as the HTTPS certificate. Only visible when a certificate exists. |
| **Stop** | Cancels a running lego process. Only visible while lego is running. |

### Configuration Panel

All fields are saved together when clicking **Save** (sticky button, top-right of the panel). The Save button is disabled until all required fields are valid.

#### Core Settings

| Field | Required | Description |
|-------|----------|-------------|
| **Email** | Yes | ACME account email. Used for certificate expiry notifications from the CA. |
| **Domains** | Yes | Comma-separated list of domains. The first domain becomes the certificate common name. Wildcard domains (e.g. `*.example.com`) are supported. |
| **DNS Provider** | Yes | The DNS provider used for DNS-01 challenge verification. Autocomplete field populated from the lego binary. See [lego DNS providers](https://go-acme.github.io/lego/dns/) for the full list. |
| **DNS Resolvers** | No | Custom DNS resolvers for challenge propagation checks. Format: `host:port`. Default: `8.8.8.8:53`. |
| **CA Server** | No | ACME directory URL. Default: Let's Encrypt production (`https://acme-v02.api.letsencrypt.org/directory`). Change to `https://acme-staging-v02.api.letsencrypt.org/directory` for testing. |
| **Key Type** | No | Private key algorithm. Options: `ec256`, `ec384`, `rsa2048`, `rsa4096`. Default: `ec256`. |

#### External Account Binding (EAB)

Toggle to enable EAB, required by some CAs (e.g. ZeroSSL, Google Trust Services, Sectigo).

| Field | Required | Description |
|-------|----------|-------------|
| **EAB Key ID** | Yes (when EAB enabled) | Key identifier provided by the CA. |
| **EAB HMAC** | Yes (when EAB enabled) | Base64 URL-encoded MAC key provided by the CA. Hidden by default; click the eye icon to reveal. |

#### Provider Environment Variables

Key-value pairs passed as environment variables to the lego process. Each DNS provider requires specific credentials (e.g. `CF_DNS_API_TOKEN` for Cloudflare, `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` for Route53).

- Values are masked by default. Click the eye icon on each row to toggle visibility.
- Click the **+** button to add a new variable. Click the delete icon to remove one.
- Refer to your DNS provider's [lego documentation page](https://go-acme.github.io/lego/dns/) for required variables.

#### Automation

| Field | Description |
|-------|-------------|
| **Auto mode** | `Disabled` or `Enabled`. When enabled, the app checks every 24 hours whether the certificate needs renewal. If the certificate expires within the configured threshold, it automatically renews and installs it to the camera. An initial check runs 30 seconds after app startup. |
| **Days before expiry** | Renewal threshold in days. The certificate is renewed when it expires within this many days. Default: `30`. Only editable when auto mode is enabled. |

### Certificate Details Dialog

Click the certificate chip in the status bar to open. Shows:

- **Domain** — Primary domain on the certificate
- **Issuer** — CA that issued the certificate
- **Valid From / Valid Until** — Certificate validity period
- **SAN** — Subject Alternative Names (all domains covered)
- **Serial** — Certificate serial number

Download buttons at the bottom:

| Button | File |
|--------|------|
| **Certificate** | PEM-encoded certificate (`.crt`) |
| **Private Key** | PEM-encoded private key (`.key`) |
| **Issuer Cert** | PEM-encoded issuer/intermediate certificate (`.issuer.crt`) |

### Last Run Panel

Displays the most recent lego operation result. Updates automatically after each obtain, renew, or auto-renew/install.

- **Success/Failed** chip — green for success, red for failure
- **Command** chip — which operation ran (`obtain`, `renew`, `auto-renew`, `auto-install`)
- **Timestamp** — when the operation completed
- **Show log** — expands the full lego output for that run

### Lego Log Output

Real-time streaming output from the running lego process, delivered over WebSocket.

- Lines are color-coded: red for errors, orange for warnings, green for info, blue for markers, purple for the command line
- Auto-scrolls to the bottom as new output arrives
- **Clear** button resets the log panel

## Certificate Installation

When you click **Install** (or auto-mode triggers installation), the app:

1. Generates a unique certificate ID with a timestamp (e.g. `lego-260215143025`)
2. Uploads the certificate and private key to the camera via ONVIF `LoadCertificateWithPrivateKey`
3. Fetches available TLS ciphers from the camera
4. Configures the camera's HTTPS server to use the new certificate via `SetWebServerTlsConfiguration`
5. Cleans up any previous `lego-*` certificates from the camera

VAPIX credentials are obtained automatically via D-Bus at app startup. If credential retrieval fails (e.g. on non-root installs), the Install button and auto-install are unavailable.

## Building

Requires [goxisbuilder](https://github.com/Cacsjep/goxisbuilder), Node.js, and AXIS Camera SDK 12.5.0+.

```bash
cd app
make dev    # build + deploy to camera
make build  # production build
```

## Acknowledgements

| Project | License |
|---------|---------|
| [lego](https://github.com/go-acme/lego) | MIT |
| [goxis](https://github.com/Cacsjep/goxis) | MIT |
| [Fiber](https://github.com/gofiber/fiber) | MIT |
| [GORM](https://github.com/go-gorm/gorm) | MIT |
| [Vue.js](https://github.com/vuejs/core) | MIT |
| [Vuetify](https://github.com/vuetifyjs/vuetify) | MIT |

## License

[MIT](app/LICENSE)
