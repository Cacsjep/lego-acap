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
