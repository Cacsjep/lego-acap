# Lego ACAP

An [AXIS Camera Application Platform (ACAP)](https://developer.axis.com/) application that downloads and invokes the [lego](https://github.com/go-acme/lego) ACME client to obtain and renew SSL/TLS certificates via DNS-01 challenge directly on Axis network cameras.

## Features

- Automatic download of the lego binary on first start (no need to ship it in the .eap)
- DNS-01 challenge with 100+ DNS providers supported by lego
- Web UI for configuration, certificate management, and live log output
- Real-time progress via WebSocket (download progress, lego output)
- Certificate viewing and download (certificate, private key, issuer cert)
- External Account Binding (EAB) support for CAs that require it
- Runs behind the Axis camera reverse proxy (no extra port exposed)
- Supports both `armv7hf` and `aarch64` architectures

## Screenshots

<!-- TODO: Add screenshots -->

## Quick Start

1. Install the `.eap` file on your Axis camera
2. Open the app settings page from the camera web interface
3. The lego binary downloads automatically on first start
4. Configure your email, domain(s), DNS provider, and provider-specific environment variables
5. Click **Obtain** to get your certificate

## Configuration

| Field | Description |
|-------|-------------|
| **Email** | ACME account email for certificate notifications |
| **Domains** | Comma-separated list of domains (e.g. `example.com, *.example.com`) |
| **DNS Provider** | DNS provider name for DNS-01 challenge (e.g. `cloudflare`, `freemyip`, `rfc2136`) |
| **DNS Resolvers** | DNS resolver for propagation check (default: `8.8.8.8:53`) |
| **CA Server** | ACME directory URL (default: Let's Encrypt production) |
| **Key Type** | Certificate key algorithm (`ec256`, `ec384`, `rsa2048`, `rsa4096`) |
| **EAB** | External Account Binding (toggle + Key ID and HMAC) |
| **Environment Variables** | Provider-specific credentials as key-value pairs |

For provider-specific environment variables, refer to the [lego DNS provider documentation](https://go-acme.github.io/lego/dns/).

### Example: FreeMyIP

- DNS Provider: `freemyip`
- Environment Variables: `FREEMYIP_TOKEN` = `your-token`

### Example: RFC2136

- DNS Provider: `rfc2136`
- Environment Variables:
  - `RFC2136_NAMESERVER` = `ns.example.com`
  - `RFC2136_TSIG_KEY` = `keyname`
  - `RFC2136_TSIG_SECRET` = `base64secret`
  - `RFC2136_TSIG_ALGORITHM` = `hmac-sha256.`

### Example: Cloudflare

- DNS Provider: `cloudflare`
- Environment Variables: `CF_DNS_API_TOKEN` = `your-api-token`

## Building

### Prerequisites

- [goxisbuilder](https://github.com/Cacsjep/goxisbuilder) (Docker-based ACAP cross-compiler)
- Node.js and npm (for frontend)
- AXIS Camera SDK 12.5.0+

### Development

```bash
cd app
make dev
```

This builds the frontend, then compiles and deploys to the camera with live reload.

### Production

```bash
cd app
make build
```

### CI/CD

The GitHub Actions workflow builds for both `armv7hf` and `aarch64` on tagged releases. See `.github/workflows/build.yaml`.

## Architecture

```
legoacap/
  app/              # Go backend (ACAP)
    app.go          # Fiber web server, routes, WebSocket
    lego.go         # Binary download, execution, DNS provider extraction
    config.go       # GORM config model + SQLite persistence
    ws.go           # WebSocket broadcast hub
    manifest.json   # ACAP manifest with reverse proxy config
  frontend/         # Vue 3 + Vuetify 3 frontend
    src/App.vue     # Single-page application
```

- **Backend**: Go with [Fiber v3](https://github.com/gofiber/fiber) web framework, [GORM](https://gorm.io/) + SQLite for config persistence, WebSocket for real-time updates
- **Frontend**: [Vue 3](https://vuejs.org/) + [Vuetify 3](https://vuetifyjs.com/) with dark theme, built with [Vite](https://vitejs.dev/)
- **ACAP SDK**: [goxis](https://github.com/Cacsjep/goxis) for Axis camera integration
- **Architecture selection**: Go build tags (`//go:build aarch64` / `//go:build armv7hf`) passed via goxisbuilder

## Acknowledgements

This project relies on the following open source software:

| Project | License | Description |
|---------|---------|-------------|
| [lego](https://github.com/go-acme/lego) | MIT | ACME client and library - the core tool this ACAP wraps |
| [goxis](https://github.com/Cacsjep/goxis) | MIT | Go SDK for Axis Camera Application Platform |
| [Fiber](https://github.com/gofiber/fiber) | MIT | Express-inspired Go web framework |
| [GORM](https://github.com/go-gorm/gorm) | MIT | Go ORM library |
| [go-sqlite3](https://github.com/mattn/go-sqlite3) | MIT | SQLite3 driver for Go |
| [Vue.js](https://github.com/vuejs/core) | MIT | Progressive JavaScript framework |
| [Vuetify](https://github.com/vuetifyjs/vuetify) | MIT | Material Design component framework for Vue |
| [Vite](https://github.com/vitejs/vite) | MIT | Next generation frontend build tool |
| [Material Design Icons](https://github.com/Templarian/MaterialDesign) | Apache 2.0 | Icon set used in the UI |

Special thanks to the [axis-uacme](https://github.com/Cronvs/axis-uacme) project for inspiration on ACME certificate management on Axis cameras.

## License

[MIT](app/LICENSE)
