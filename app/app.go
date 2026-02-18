package main

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Cacsjep/goxis/pkg/acapapp"
	"github.com/Cacsjep/goxis/pkg/dbus"
	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/static"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type LegoApplication struct {
	acapp           *acapapp.AcapApplication
	webserver       *fiber.App
	db              *gorm.DB
	wsHub           *WSHub
	vapixUser       string
	vapixPass       string
	vapixReady      bool
	autoRenewTicker *time.Ticker
}

func NewLegoApplication() *LegoApplication {
	return &LegoApplication{}
}

func (app *LegoApplication) Start() {
	app.acapp = acapapp.NewAcapApplication()
	app.acapp.Syslog.Infof("Starting Lego ACAP for arch: %s", LegoArch)

	if err := os.MkdirAll("./localdata", 0755); err != nil {
		app.acapp.Syslog.Critf("Failed to create localdata dir: %s", err)
		return
	}

	db, err := gorm.Open(sqlite.Open("./localdata/db.sqlite"), &gorm.Config{})
	if err != nil {
		app.acapp.Syslog.Critf("Failed to open database: %s", err)
		return
	}
	app.db = db

	if err := db.AutoMigrate(&Config{}, &RunHistory{}); err != nil {
		app.acapp.Syslog.Critf("Failed to migrate database: %s", err)
		return
	}

	if err := SeedDefaultConfig(db); err != nil {
		app.acapp.Syslog.Critf("Failed to seed config: %s", err)
		return
	}

	var httpBase, wsBase string
	if UseBasePath {
		httpBase, err = app.acapp.AcapWebBaseUri()
		if err != nil {
			app.acapp.Syslog.Critf("Failed to get ACAP web base URI: %s", err)
			return
		}
		pkgcfg := app.acapp.Manifest.ACAPPackageConf
		wsBase = fmt.Sprintf("/local/%s/%s", pkgcfg.Setup.AppName, pkgcfg.Configuration.ReverseProxy[1].ApiPath)
		app.acapp.Syslog.Infof("Reverse proxy base paths - HTTP: %s, WS: %s", httpBase, wsBase)
	}

	app.wsHub = NewWSHub()

	// Retrieve VAPIX credentials via D-Bus for cert installation
	vapixUser, vapixPass, err := dbus.RetrieveVapixCredentials("root")
	if err != nil {
		app.acapp.Syslog.Infof("Could not retrieve VAPIX credentials: %s (cert install will be unavailable)", err)
	} else {
		app.vapixUser = vapixUser
		app.vapixPass = vapixPass
		app.vapixReady = true
		app.acapp.Syslog.Info("VAPIX credentials retrieved successfully")
	}

	if !IsLegoReady() {
		app.acapp.Syslog.Info("Lego binary not found, downloading...")
		go func() {
			if err := DownloadLego(app.wsHub); err != nil {
				app.acapp.Syslog.Errorf("Auto-download of lego failed: %s", err)
			} else {
				app.acapp.Syslog.Info("Lego binary downloaded successfully")
			}
		}()
	}

	app.webserver = fiber.New()
	app.webserver.Use(cors.New())
	app.setupRoutes(httpBase, wsBase)

	app.startAutoRenew()

	app.acapp.OnCloseCleaners = append(app.acapp.OnCloseCleaners, func() {
		if app.autoRenewTicker != nil {
			app.autoRenewTicker.Stop()
		}
		app.webserver.Shutdown()
	})

	go app.acapp.RunInBackground()

	app.acapp.Syslog.Infof("Starting web server on %s", ListenAddr)
	if err := app.webserver.Listen(ListenAddr); err != nil {
		app.acapp.Syslog.Critf("Web server error: %s", err)
	}
}

func (app *LegoApplication) startAutoRenew() {
	// Initial check after 30s delay (give time for lego download on first boot)
	go func() {
		time.Sleep(30 * time.Second)
		app.checkAndAutoRenew()
	}()

	// Daily check
	app.autoRenewTicker = time.NewTicker(24 * time.Hour)
	go func() {
		for range app.autoRenewTicker.C {
			app.checkAndAutoRenew()
		}
	}()
}

func (app *LegoApplication) checkAndAutoRenew() {
	config, err := GetConfig(app.db)
	if err != nil || !config.AutoMode {
		return
	}
	if !IsLegoReady() {
		return
	}

	domain := primaryDomain(config)
	if domain == "" {
		return
	}

	certFile := legoCertsPath + "/certificates/" + domain + ".crt"
	days, err := getCertDaysRemaining(certFile)
	if err != nil {
		return // no cert yet or can't parse
	}

	app.acapp.Syslog.Infof("Certificate expires in %d days (threshold: %d)", days, config.AutoDays)

	if days > config.AutoDays {
		return
	}

	// Auto-renew
	app.acapp.Syslog.Infof("Auto-renewing certificate (expires in %d days)", days)
	output, err := RunLego(config, app.wsHub, "renew", app.acapp.Syslog.Infof)
	SaveRunHistory(app.db, "auto-renew", err == nil, output)

	if err != nil {
		app.acapp.Syslog.Errorf("Auto-renew failed: %s", err)
		return
	}

	// Auto-install
	if !app.vapixReady {
		app.acapp.Syslog.Infof("Skipping auto-install: VAPIX credentials not available")
		return
	}
	app.acapp.Syslog.Infof("Auto-installing certificate for %s", domain)
	if err := InstallCertToCamera(app.vapixUser, app.vapixPass, domain); err != nil {
		app.acapp.Syslog.Errorf("Auto-install failed: %s", err)
		SaveRunHistory(app.db, "auto-install", false, err.Error())
		app.wsHub.Broadcast(MsgLegoError, map[string]string{"error": "Auto-install failed: " + err.Error()})
	} else {
		app.acapp.Syslog.Infof("Auto-install successful for %s", domain)
		SaveRunHistory(app.db, "auto-install", true, "Certificate installed successfully")
		app.wsHub.Broadcast(MsgLegoComplete, map[string]string{"message": "Certificate auto-installed to camera"})
	}
}

func (app *LegoApplication) setupRoutes(httpBase, wsBase string) {
	app.webserver.Get(wsBase+"/ws", websocket.New(func(c *websocket.Conn) {
		app.wsHub.Register(c)
		defer app.wsHub.Unregister(c)

		for {
			_, _, err := c.ReadMessage()
			if err != nil {
				break
			}
		}
	}))

	api := app.webserver.Group(httpBase + "/api")

	api.Get("/status", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"lego_ready":   IsLegoReady(),
			"lego_running": IsLegoRunning(),
			"arch":         LegoArch,
		})
	})

	api.Post("/stop", func(c fiber.Ctx) error {
		if err := StopLego(app.wsHub); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"message": "Lego process stopped"})
	})

	api.Get("/providers", func(c fiber.Ctx) error {
		providers, err := GetDNSProviders()
		if err != nil || len(providers) == 0 {
			if IsLegoReady() {
				if extractErr := extractDNSProviders(); extractErr != nil {
					app.acapp.Syslog.Errorf("Failed to extract DNS providers: %s", extractErr)
					return c.JSON([]string{})
				}
				providers, err = GetDNSProviders()
				if err != nil {
					return c.JSON([]string{})
				}
			} else {
				return c.JSON([]string{})
			}
		}
		return c.JSON(providers)
	})

	api.Get("/config", func(c fiber.Ctx) error {
		config, err := GetConfig(app.db)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "No config found"})
		}
		return c.JSON(config)
	})

	api.Put("/config", func(c fiber.Ctx) error {
		var config Config
		if err := c.Bind().JSON(&config); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		existing, _ := GetConfig(app.db)
		if existing != nil {
			config.ID = existing.ID
		}
		if err := SaveConfig(app.db, &config); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(config)
	})

	api.Post("/download", func(c fiber.Ctx) error {
		go func() {
			if err := DownloadLego(app.wsHub); err != nil {
				app.acapp.Syslog.Errorf("Lego download failed: %s", err)
			}
		}()
		return c.JSON(fiber.Map{"message": "Download started"})
	})

	api.Post("/obtain", func(c fiber.Ctx) error {
		config, err := GetConfig(app.db)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "No config found"})
		}
		go func() {
			output, err := RunLego(config, app.wsHub, "obtain", app.acapp.Syslog.Infof)
			SaveRunHistory(app.db, "obtain", err == nil, output)
			if err != nil {
				app.acapp.Syslog.Errorf("Lego obtain failed: %s", err)
			}
		}()
		return c.JSON(fiber.Map{"message": "Certificate obtain started"})
	})

	api.Post("/renew", func(c fiber.Ctx) error {
		config, err := GetConfig(app.db)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "No config found"})
		}
		go func() {
			output, err := RunLego(config, app.wsHub, "renew", app.acapp.Syslog.Infof)
			SaveRunHistory(app.db, "renew", err == nil, output)
			if err != nil {
				app.acapp.Syslog.Errorf("Lego renew failed: %s", err)
			}
		}()
		return c.JSON(fiber.Map{"message": "Certificate renewal started"})
	})

	api.Get("/runs/last", func(c fiber.Ctx) error {
		run, err := GetLastRun(app.db)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "No runs yet"})
		}
		return c.JSON(run)
	})

	api.Get("/cert", func(c fiber.Ctx) error {
		config, err := GetConfig(app.db)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "No config found"})
		}
		domain := primaryDomain(config)
		certFile := legoCertsPath + "/certificates/" + domain + ".crt"
		keyFile := legoCertsPath + "/certificates/" + domain + ".key"

		certExists := fileExists(certFile)
		keyExists := fileExists(keyFile)

		result := fiber.Map{
			"has_cert":  certExists && keyExists,
			"cert_path": certFile,
			"key_path":  keyFile,
			"domain":    domain,
		}

		if certExists {
			if info, err := parseCertInfo(certFile); err == nil {
				result["issuer"] = info["issuer"]
				result["not_before"] = info["not_before"]
				result["not_after"] = info["not_after"]
				result["san"] = info["san"]
				result["serial"] = info["serial"]
			}
		}

		return c.JSON(result)
	})

	api.Get("/cert/download/:type", func(c fiber.Ctx) error {
		config, err := GetConfig(app.db)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "No config found"})
		}
		domain := primaryDomain(config)

		fileType := c.Params("type")
		var filePath, fileName string
		switch fileType {
		case "crt":
			filePath = legoCertsPath + "/certificates/" + domain + ".crt"
			fileName = domain + ".crt"
		case "key":
			filePath = legoCertsPath + "/certificates/" + domain + ".key"
			fileName = domain + ".key"
		case "issuer":
			filePath = legoCertsPath + "/certificates/" + domain + ".issuer.crt"
			fileName = domain + ".issuer.crt"
		default:
			return c.Status(400).JSON(fiber.Map{"error": "Invalid type, use: crt, key, issuer"})
		}

		if !fileExists(filePath) {
			return c.Status(404).JSON(fiber.Map{"error": "File not found"})
		}

		c.Set("Content-Disposition", "attachment; filename=\""+fileName+"\"")
		return c.SendFile(filePath)
	})

	api.Post("/cert/install", func(c fiber.Ctx) error {
		if !app.vapixReady {
			return c.Status(500).JSON(fiber.Map{"error": "VAPIX credentials not available"})
		}
		config, err := GetConfig(app.db)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "No config found"})
		}
		domain := primaryDomain(config)

		app.acapp.Syslog.Infof("Installing certificate for %s to camera", domain)
		if err := InstallCertToCamera(app.vapixUser, app.vapixPass, domain); err != nil {
			app.acapp.Syslog.Errorf("Failed to install certificate: %s", err)
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		app.acapp.Syslog.Infof("Certificate for %s installed successfully", domain)
		return c.JSON(fiber.Map{"message": "Certificate installed to camera"})
	})

	app.webserver.Use(httpBase+"/", static.New("./html"))
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func splitDomains(domains string) []string {
	var result []string
	for _, d := range strings.Split(domains, ",") {
		d = strings.TrimSpace(d)
		if d != "" {
			result = append(result, d)
		}
	}
	return result
}

// primaryDomain returns the first domain from a comma-separated domain list.
func primaryDomain(config *Config) string {
	parts := splitDomains(config.Domains)
	if len(parts) > 0 {
		return parts[0]
	}
	return config.Domains
}

func getCertDaysRemaining(certPath string) (int, error) {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return 0, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return 0, fmt.Errorf("failed to decode certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return 0, err
	}
	return int(time.Until(cert.NotAfter).Hours() / 24), nil
}

func parseCertInfo(certPath string) (map[string]any, error) {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"issuer":     cert.Issuer.CommonName,
		"not_before": cert.NotBefore.Format("2006-01-02 15:04:05 UTC"),
		"not_after":  cert.NotAfter.Format("2006-01-02 15:04:05 UTC"),
		"san":        cert.DNSNames,
		"serial":     cert.SerialNumber.String(),
	}, nil
}
