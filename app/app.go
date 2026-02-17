package main

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"strings"

	"github.com/Cacsjep/goxis/pkg/acapapp"
	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/static"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type LegoApplication struct {
	acapp     *acapapp.AcapApplication
	webserver *fiber.App
	db        *gorm.DB
	wsHub     *WSHub
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

	if err := db.AutoMigrate(&Config{}); err != nil {
		app.acapp.Syslog.Critf("Failed to migrate database: %s", err)
		return
	}

	if err := SeedDefaultConfig(db); err != nil {
		app.acapp.Syslog.Critf("Failed to seed config: %s", err)
		return
	}

	app.wsHub = NewWSHub()

	if !IsLegoReady() {
		app.acapp.Syslog.Info("Lego binary not found, downloading...")
		go func() {
			if err := DownloadLego(app.wsHub); err != nil {
				app.acapp.Syslog.Critf("Auto-download of lego failed: %s", err)
			} else {
				app.acapp.Syslog.Info("Lego binary downloaded successfully")
			}
		}()
	}

	app.webserver = fiber.New()
	app.webserver.Use(cors.New())
	app.setupRoutes()

	app.acapp.OnCloseCleaners = append(app.acapp.OnCloseCleaners, func() {
		app.webserver.Shutdown()
	})

	go app.acapp.RunInBackground()

	app.acapp.Syslog.Infof("Starting web server on %s", ListenAddr)
	if err := app.webserver.Listen(ListenAddr); err != nil {
		app.acapp.Syslog.Critf("Web server error: %s", err)
	}
}

func (app *LegoApplication) setupRoutes() {
	app.webserver.Get("/ws", websocket.New(func(c *websocket.Conn) {
		app.wsHub.Register(c)
		defer app.wsHub.Unregister(c)

		for {
			_, _, err := c.ReadMessage()
			if err != nil {
				break
			}
		}
	}))

	api := app.webserver.Group("/api")

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
			// Try extracting on-demand if binary exists but providers.json is missing/empty
			if IsLegoReady() {
				if extractErr := extractDNSProviders(); extractErr != nil {
					app.acapp.Syslog.Infof("Failed to extract DNS providers: %s", extractErr)
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
				app.acapp.Syslog.Critf("Lego download failed: %s", err)
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
			if err := RunLego(config, app.wsHub, "obtain", app.acapp.Syslog.Infof); err != nil {
				app.acapp.Syslog.Critf("Lego obtain failed: %s", err)
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
			if err := RunLego(config, app.wsHub, "renew", app.acapp.Syslog.Infof); err != nil {
				app.acapp.Syslog.Critf("Lego renew failed: %s", err)
			}
		}()
		return c.JSON(fiber.Map{"message": "Certificate renewal started"})
	})

	api.Get("/cert", func(c fiber.Ctx) error {
		config, err := GetConfig(app.db)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "No config found"})
		}
		domain := config.Domains
		if idx := len(domain); idx > 0 {
			parts := splitDomains(domain)
			if len(parts) > 0 {
				domain = parts[0]
			}
		}
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
		domain := config.Domains
		parts := splitDomains(domain)
		if len(parts) > 0 {
			domain = parts[0]
		}

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

	app.webserver.Use("/", static.New("./html"))
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

func parseCertInfo(certPath string) (map[string]interface{}, error) {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"issuer":     cert.Issuer.CommonName,
		"not_before": cert.NotBefore.Format("2006-01-02 15:04:05 UTC"),
		"not_after":  cert.NotAfter.Format("2006-01-02 15:04:05 UTC"),
		"san":        cert.DNSNames,
		"serial":     cert.SerialNumber.String(),
	}, nil
}
